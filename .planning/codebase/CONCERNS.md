# Codebase Concerns

**Analysis Date:** 2026-08-29

## Tech Debt

**Unauthenticated operator API:**
- Issue: `PUT /queue/rate/:eventId`, `GET /queue/config/:eventId`, and `GET /queue/events` are exposed without authentication. The admin UI is a client, not an access control boundary.
- Files: `internal/api/handler_admin.go`, `internal/api/handler_events.go`, `internal/api/router.go`, `web/admin/admin.js`
- Impact: Any network client that can reach the queue API can change admission throughput/capacity or enumerate event IDs, enabling queue disruption and operational-data disclosure.
- Fix approach: Require an operator credential at the router or handler boundary, validate it server-side, and keep CORS as a browser policy only. The existing `ponytail:` comments explicitly defer this before production.

**Untrusted exit accounting:**
- Issue: `POST /queue/exit` accepts an arbitrary JSON `eventId`, has no session/authentication check, and blindly decrements Redis.
- Files: `internal/api/handler_exit.go`, `internal/api/router.go`, `pkg/middleware/queue_guard.go`
- Impact: Repeated or forged exits can drive `active:{eventId}` below zero, causing the scheduler to over-admit beyond the configured capacity.
- Fix approach: Authenticate the session, derive the event from its claims, make exit idempotent per session/ticket, reject mismatched events, and clamp/account atomically in Redis.

**Admission accounting and reclamation are split across components:**
- Issue: The scheduler increments `active:{eventId}` when it pops a ticket, while `QueueGuard` performs no increment when the origin admits a request. There is no TTL-based decrement for abandoned sessions.
- Files: `internal/scheduler/admission.go`, `pkg/middleware/queue_guard.go`, `internal/config/config.go`, `.planning/STATE.md`, `DESIGN.md`
- Impact: A token that is never used consumes capacity permanently; a failed origin admission can still consume capacity. The configured capacity therefore stops reflecting live origin sessions.
- Fix approach: Define one ownership point for increment/decrement, record an expiring per-session admission marker, and reconcile expiration atomically. Implement the deferred TTL sweep before relying on capacity for production traffic.

**Ignored Redis and signing errors:**
- Issue: Several writes and counter updates discard errors; admission removes a ticket before token issuance and persistence are known to have succeeded.
- Files: `internal/api/handler_join.go`, `internal/api/handler_admin.go`, `internal/api/handler_exit.go`, `internal/api/handler_status.go`, `internal/scheduler/admission.go`, `pkg/middleware/queue_guard.go`
- Impact: Redis outages can silently lose tickets, return success for unapplied admin changes, leak capacity, or leave an admitted user without a usable token. `ZPOPMIN` makes the loss irreversible without a recovery path.
- Fix approach: Check and surface critical errors, and use an atomic Lua/scripted state transition or a retry/recovery stream around pop, token persistence, accounting, and publish.

## Known Bugs

**Cross-host admission cookie cannot reach the origin in the local topology:**
- Symptoms: The queue page sets `q_admission` with JavaScript on `localhost:8082`, then redirects to the stub origin on `localhost:8081`; the host-only cookie is not sent to the origin.
- Files: `web/queue/queue.js`, `web/queue/index.html`, `cmd/stuborigin/main.go`, `internal/config/config.go`
- Trigger: Complete an admission flow through the Docker Compose static page and stub origin.
- Workaround: Keep queue page and protected origin on a cookie-compatible host, or transfer the token through a deliberate same-site handoff endpoint. Configure cookie domain/SameSite/Secure behavior for the actual deployment topology.

**Concurrent polling can deliver one token multiple times:**
- Symptoms: `QueueStatusPoll` performs `HGET` and then `HDEL` as separate commands, so concurrent pollers can both read the same admission token before either delete completes.
- Files: `internal/api/handler_status.go`
- Trigger: Poll the same admitted ticket concurrently from multiple tabs or retrying clients.
- Workaround: The origin SETNX rejects the second use, but the queue API still duplicates a sensitive token and clients receive inconsistent results.
- Fix approach: Replace the pair with an atomic `HGETDEL` operation or a Lua script and test concurrent callers.

**Scheduler lock release is not ownership-safe:**
- Symptoms: A tick that runs longer than `SchedulerLockTTL` can lose its lock; a second scheduler can acquire it, then the first scheduler's deferred `DEL` deletes the newer owner's lock.
- Files: `internal/scheduler/leader_lock.go`, `internal/scheduler/admission.go`, `internal/config/config.go`
- Trigger: Redis latency, a large admission batch, or a paused scheduler lasting beyond 10 seconds.
- Workaround: Run one scheduler instance and keep batches small enough to finish within the TTL.
- Fix approach: Store a unique lock value and release only when the value matches; renew the lease for long ticks.

## Security Considerations

**Open redirect and token-flow abuse:**
- Risk: The `target` query parameter is carried into `sessionStorage` and later assigned to `window.location.href` without an origin allowlist.
- Files: `internal/api/handler_join.go`, `pkg/middleware/queue_guard.go`, `web/queue/queue.js`
- Current mitigation: Query escaping protects URL syntax, and `queueErrorPage` escapes the generated link parameter.
- Recommendations: Allow only configured protected-origin paths/hosts, normalize and validate before storing, and avoid treating arbitrary user input as a navigation destination.

**Weak deployment defaults and plaintext transport:**
- Risk: The example secrets are literal `change-me-*` values, `SECURE` defaults false, local HTTP URLs are embedded in clients, and Redis has no configured authentication or TLS.
- Files: `.env.example`, `internal/config/config.go`, `web/queue/index.html`, `web/admin/index.html`, `docker-compose.yml`, `internal/store/redis.go`
- Current mitigation: Production secrets are required to be non-empty and the two JWT secrets must differ.
- Recommendations: Fail deployment validation on placeholder/weak secrets, require HTTPS and secure cookies outside local development, and configure private/authenticated/TLS Redis for production.

**Missing abuse controls at the queue boundary:**
- Risk: `/queue/join` can mint unlimited tickets and `/queue/status/:ticketId` can be polled or held as SSE without rate or connection limits.
- Files: `internal/api/handler_join.go`, `internal/api/handler_status.go`, `internal/api/router.go`
- Current mitigation: The design explicitly scopes CAPTCHA/WAF to v2; no application-level limiter is present.
- Recommendations: Add deployment-layer WAF/rate limiting and per-client join/SSE limits before exposing the service to untrusted traffic.

## Performance Bottlenecks

**Full Redis keyspace scan on every scheduler tick:**
- Problem: Each scheduler tick scans all `queue:*` keys, then processes every discovered event.
- Files: `internal/scheduler/admission.go`, `internal/store/keys.go`
- Cause: Event discovery is coupled to the one-second scheduler loop.
- Improvement path: Maintain an event registry or queue event index and refresh it separately. The existing `ponytail:` note sets the known ceiling at roughly 100 events.

**SSE fan-out amplifies Redis and scheduler work:**
- Problem: Every tick is published to each event channel, and every connected SSE handler performs a separate `ZRANK` and JSON write.
- Files: `internal/scheduler/admission.go`, `internal/api/handler_status.go`
- Cause: Position updates are recomputed per connection rather than batched or filtered.
- Improvement path: Measure connection counts and Redis command latency, then publish only when positions change materially or use a shared event-position fan-out.

## Fragile Areas

**SSE lifecycle and pub/sub handling:**
- Files: `internal/api/handler_status.go`, `internal/store/redis.go`
- Why fragile: The SSE loop dereferences a received pub/sub message without checking whether the channel closed, and it has no explicit subscription error handling or write deadline.
- Safe modification: Handle closed channels and subscription errors, bound stream lifetime, and test disconnects, Redis restarts, and slow clients.
- Test coverage: No SSE handler or Redis integration tests are present.

**Ticket state transitions:**
- Files: `internal/api/handler_join.go`, `internal/api/handler_status.go`, `internal/scheduler/admission.go`
- Why fragile: Ticket sorted-set membership, ticket hash metadata, admission-token deletion, and scheduler pop are separate operations with transient windows and no transactional recovery.
- Safe modification: Preserve a ticket state machine and cover join/admit/poll/retry/expiry races with an in-memory Redis-compatible integration test.
- Test coverage: Only JWT behavior is unit-tested in `internal/token/jwt_test.go`.

**Shutdown and process lifecycle:**
- Files: `cmd/queueserver/main.go`, `cmd/scheduler/main.go`, `cmd/stuborigin/main.go`, `internal/store/redis.go`
- Why fragile: The queue server creates a signal context but does not pass it to an HTTP server, and all binaries use `router.Run` without graceful server shutdown or explicit Redis client close.
- Safe modification: Own an `http.Server`, call `Shutdown` on context cancellation, and close Redis clients after serving stops.
- Test coverage: No lifecycle or signal-handling tests are present.

## Scaling Limits

**Queue and event retention:**
- Current capacity: Ticket hashes expire after 2 hours; queue sorted-set members and active counters have no matching expiration in `internal/config/config.go`.
- Limit: Abandoned or admitted members can accumulate indefinitely, and active counts can remain saturated indefinitely without a successful exit.
- Scaling path: Add explicit queue cleanup and session-expiry reconciliation, then load-test Redis memory, ZRANK, SCAN, and SSE fan-out at the intended 10k-user target.

## Dependencies at Risk

**Indirect dependency declarations:**
- Risk: Runtime-critical packages such as Gin, JWT, and go-redis are listed as `// indirect`, obscuring the direct dependency contract and making maintenance tooling less reliable.
- Files: `go.mod`, `internal/api/router.go`, `internal/token/jwt.go`, `internal/store/redis.go`
- Impact: Dependency upgrades and vulnerability review can miss which packages are intentionally part of the application surface.
- Migration plan: Declare directly imported modules as direct requirements and run normal module tidy/vulnerability checks in CI.

## Missing Critical Features

**Production AWS integration remains absent:**
- Problem: The roadmap requires ECS, ElastiCache, S3/CloudFront, DynamoDB audit records, and SQS FIFO admission events, but no infrastructure or AWS implementation files are present in the repository.
- Blocks: Production deployment, durable auditability, and the stated admission-event delivery guarantee.
- Files: `ROADMAP.md` (via `.planning/ROADMAP.md`), `go.mod`, `cmd/scheduler/main.go`

## Test Coverage Gaps

**Queue correctness and security paths:**
- What's not tested: Admin authorization, exit idempotency, event isolation, cookie handoff, open redirects, Redis failures, atomic token delivery, scheduler locking, admission/accounting races, SSE disconnects, and handler input limits.
- Files: `internal/api/*.go`, `internal/scheduler/*.go`, `pkg/middleware/queue_guard.go`, `web/queue/queue.js`, `web/admin/admin.js`
- Risk: The core value—accurate position and exactly-once admission—can regress while `go test ./...` remains green.
- Priority: High

**Current verification shape:**
- What's not tested: The repository has one token unit-test file and a shell smoke test that requires a running Docker Compose stack; it has no handler-level or scheduler-level automated tests.
- Files: `internal/token/jwt_test.go`, `scripts/verify.sh`, `Makefile`
- Risk: End-to-end checks can miss deterministic race conditions and are environment-dependent.
- Priority: Medium

---

*Concerns audit: 2026-08-29*
