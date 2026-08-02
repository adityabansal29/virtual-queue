# Phase 1: Queue Core & Local Stack - Context

**Gathered:** 2026-08-02
**Status:** Ready for planning

<domain>
## Phase Boundary

Full queue mechanics — join, position tracking, admission, and one-time token enforcement — runnable locally via Docker Compose and verifiable end-to-end without a browser. Success criteria are met via curl commands and a verification shell script; no frontend required.

</domain>

<decisions>
## Implementation Decisions

### Service Architecture

- **D-01:** Stub ticket origin is a separate binary (`cmd/stuborigin/`) — NOT embedded handlers in the queue API. It runs as its own Docker Compose service on a different port. — **Reversibility:** reversible
- **D-02:** Stub origin does NOT access queue Redis directly. The two services (queue API, stub origin) have separate Redis instances. — **Reversibility:** costly — changing this later means refactoring connection config in both services and Docker Compose.
- **D-03:** Stub origin owns `token:{jti}` SETNX (one-time use enforcement) and q_session issuance. q_session issuance is the origin's responsibility, not the queue service's.
- **D-04:** Docker Compose runs two Redis containers: `redis-queue` (queue service) and `redis-origin` (stub origin / QueueGuard). — **Reversibility:** reversible
- **D-05:** Stub origin returns a simple HTML page on successful admission ("Seat selection — Event: {eventId}, Ticket: {ticketId}").

### Capacity & Active Count

- **D-06:** Active count capping (`active:{eventId}`) is skipped in Phase 1. Scheduler admits at the configured rate with no headroom check (`headroom = ∞`). The capacity ceiling is deferred to a future phase when a real origin is wired up. — **Reversibility:** reversible — add `active:{eventId}` tracking + scheduler headroom check later without breaking existing keys.

### Go Project Layout

- **D-07:** Go module lives at the repo root (`go.mod` at `/`). Module name: `github.com/adityabansal29/virtual-queue`. — **Reversibility:** one-way — changing the module name after Phase 1 requires updating all import paths across every Go file.
- **D-08:** Standard Go layout: `cmd/queueserver/` and `cmd/stuborigin/` as entry points; `internal/` for private packages (api, scheduler, token, store, config); `pkg/middleware/` for `queue_guard.go` (shared between both binaries). — **Reversibility:** reversible

### Testing

- **D-09:** Phase 1 includes Go unit tests in `internal/token/` covering: JWT sign with ADMISSION_SECRET, verify with correct secret, rejection with wrong secret, expired token detection. No other unit tests in Phase 1.
- **D-10:** No integration tests using testcontainers. Docker Compose (`docker compose up`) serves as the integration test environment.
- **D-11:** `scripts/verify.sh` — executable shell script that runs all 5 success criteria in sequence against a running docker compose. Includes both HTTP assertions (status codes, response bodies) AND `redis-cli` introspection of internal Redis state (e.g., ZRANK after join, `token:{jti}` existence after admission, SETNX key presence after 403). Run via `make verify` or `./scripts/verify.sh`.

### Claude's Discretion

- Config injection approach (os.Getenv vs dotenv) — not discussed; standard Go practice
- Logging library (slog/zap/log) — not discussed; open to standard Go slog or zap
- Docker Compose health checks and service dependency ordering
- Specific Docker image base (distroless vs alpine)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Architecture & Design
- `DESIGN.md` — Complete HLD v3. All core architectural decisions are here: two-cookie token model, hybrid SSE/poll, two independent secrets, SETNX placement, Redis data model, Go service structure (Section 10), EdgeWorker design (as Go middleware locally). **Read before planning any component.**

### Requirements
- `.planning/REQUIREMENTS.md` — Phase 1 requirements: QUEUE-01 through QUEUE-09, TOKEN-01 through TOKEN-07, INFRA-01. Success criteria and traceability table.
- `.planning/PROJECT.md` — Active requirements list, key decisions table, constraints (Go + Redis + AWS stack), out-of-scope items.

### Phase Roadmap
- `.planning/ROADMAP.md` — Phase 1 goal, success criteria (5 items), and dependency on nothing (first phase).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- None — fresh repo. Only `DESIGN.md`, `README.md`, and `.gitignore` exist.

### Established Patterns
- DESIGN.md Section 7 contains exact Go code for the SSE handler, poll handler, and hybrid crossover client JS — these are the reference implementation patterns.
- DESIGN.md Section 8 contains QueueGuard middleware code (Go) and EdgeWorker code (JS) — use as the implementation reference for `pkg/middleware/queue_guard.go`.
- DESIGN.md Section 6.5 contains the scheduler tick function — use as the reference for `internal/scheduler/admission.go`.

### Integration Points
- `pkg/middleware/queue_guard.go` — imported by both `cmd/stuborigin/` and (later) any real origin. Must accept Redis client and config as parameters, not globals.
- Docker Compose: queue API on one port, stub origin on another, two Redis containers.

</code_context>

<specifics>
## Specific Ideas

- The DESIGN.md Go directory structure (Section 10) maps directly to the chosen layout without the `internal-queue/` wrapper.
- `scripts/verify.sh` should exercise each of the 5 success criteria in sequence, printing PASS/FAIL, and can be used in CI as a smoke test.
- Two separate Redis containers (`redis-queue`, `redis-origin`) cleanly enforce the service boundary: queue service code can never accidentally read origin Redis and vice versa.

</specifics>

<deferred>
## Deferred Ideas

- Active count capping (`capacity:{eventId}` ceiling enforcement) — deferred to a later phase when a real origin is wired up. All scaffolding (Redis key, scheduler headroom calculation) is designed in DESIGN.md; just not enforced in Phase 1.
- Admin dashboard UI (UI-06, UI-07) — Phase 2. Queue API admin endpoints (`PUT /queue/rate/:eventId`, `GET /queue/config/:eventId`) are in Phase 1 scope but the browser UI is Phase 2.
- Static queue waiting page (HTML/JS, UI-01 through UI-05) — Phase 2.
- TTL sweep for abandoned sessions (OPS-03) — v2 backlog.

</deferred>

---

*Phase: 1-Queue Core & Local Stack*
*Context gathered: 2026-08-02*
