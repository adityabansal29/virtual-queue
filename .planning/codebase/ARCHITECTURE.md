<!-- refreshed: 2026-08-29 -->
# Architecture

**Analysis Date:** 2026-08-29

## System Overview

```text
┌─────────────────────────────────────────────────────────────┐
│                    Browser / static UI                       │
│  `web/queue/`       `web/admin/`       `web/queue/index.html` │
└──────────────┬───────────────┬──────────────────────────────┘
               │ HTTP/CORS     │ HTTP/CORS
               ▼               ▼
┌─────────────────────────────────────────────────────────────┐
│                     Queue HTTP API                           │
│ `cmd/queueserver/main.go` → `internal/api/`                  │
│ Join, poll/SSE status, exit, admin configuration, health     │
└──────────────────────┬──────────────────────────────────────┘
                       │ Redis queue namespace
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                         Redis                                │
│ `internal/store/keys.go`: sorted sets, hashes, counters,     │
│ pub/sub channels, and scheduler locks                       │
└──────────────┬──────────────────────────┬───────────────────┘
               │                          │
               ▼                          ▼
┌──────────────────────────┐  ┌───────────────────────────────┐
│ Admission scheduler       │  │ Protected/stub origin          │
│ `cmd/scheduler/main.go`   │  │ `cmd/stuborigin/main.go`        │
│ `internal/scheduler/`     │  │ `pkg/middleware/queue_guard.go`│
└──────────────┬───────────┘  └───────────────┬───────────────┘
               │ signs q_admission             │ consumes once,
               │ publishes events              │ issues q_session
               └───────────────┬────────────────┘
                               ▼
                    Browser redirects to origin
```

The current implementation is a small multi-process Go system. `queueserver`,
`scheduler`, and `stuborigin` are independently built binaries that coordinate
through Redis. Static assets are mounted into an nginx container by
`docker-compose.yml`; there is no Go template or server-side rendering layer.

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| Queue API bootstrap | Loads queue config, creates Redis client, builds router, starts Gin HTTP server | `cmd/queueserver/main.go` |
| HTTP routing | Gin middleware, CORS allowlist, endpoint dispatch | `internal/api/router.go` |
| Queue handlers | Ticket creation, position reads, SSE, exit, health, and admin operations | `internal/api/handler_join.go`, `internal/api/handler_status.go`, `internal/api/handler_exit.go`, `internal/api/handler_admin.go`, `internal/api/handler_events.go` |
| Redis access conventions | Centralizes key names, queue rank reads, ticket lookup, and event discovery | `internal/store/keys.go`, `internal/store/redis.go` |
| Admission scheduler | Scans events, acquires per-event lock, applies rate/capacity limits, pops FIFO tickets, signs and publishes admission tokens | `internal/scheduler/admission.go`, `internal/scheduler/leader_lock.go` |
| Token model | Issues and validates admission and session JWTs with independent secrets | `internal/token/jwt.go`, `internal/token/session.go` |
| Origin guard | Validates session fast path or one-time admission JWT, sets session cookie, and protects the stub checkout route | `pkg/middleware/queue_guard.go`, `cmd/stuborigin/main.go` |
| Queue waiting UI | Polls, crosses over to SSE near the front, renders position/constraint state, and redirects after admission | `web/queue/queue.js` |
| Admin UI | Polls event configuration and submits rate/capacity changes | `web/admin/admin.js` |
| Logging | Thin context-aware wrappers around `log/slog` | `pkg/log/log.go` |

## Pattern Overview

**Overall:** Small layered service with dependency injection at process boundaries.

**Key Characteristics:**

- Executables compose concrete dependencies; handlers and schedulers receive a Redis client and configuration rather than constructing them internally.
- Redis is the source of truth for queue membership and admission coordination; sorted-set rank is the user-facing position.
- FIFO admission is implemented by `ZPOPMIN`; scheduler-to-client updates use Redis pub/sub and ticket hashes.
- The queue API is stateless between requests except for Redis state. The browser owns transport state (`EventSource`, polling timer, and session storage target).
- Authentication is a two-stage cookie flow: `q_admission` is a one-time JWT and `q_session` is the ongoing session JWT.

## Layers

**Process/bootstrap layer:**
- Purpose: Load environment configuration, create clients, wire dependencies, and run each process.
- Location: `cmd/queueserver/main.go`, `cmd/scheduler/main.go`, `cmd/stuborigin/main.go`
- Contains: `main` functions and process-specific composition.
- Depends on: `internal/config`, `internal/store`, `internal/api`, `internal/scheduler`, `pkg/middleware`.
- Used by: Docker entrypoints in `Dockerfile.queueserver`, `Dockerfile.scheduler`, and `Dockerfile.stuborigin`.

**HTTP/API layer:**
- Purpose: Translate HTTP requests into queue, status, admission-control, and admin operations.
- Location: `internal/api/`
- Contains: `Handler`, router wiring, Gin handlers, JSON/SSE responses.
- Depends on: `internal/store`, `internal/config`, Redis, Gin, and logging.
- Used by: `cmd/queueserver/main.go` and the browser assets under `web/`.

**Coordination/data-access layer:**
- Purpose: Define Redis namespaces and small reusable Redis operations.
- Location: `internal/store/`
- Contains: Key constructors, `SCAN` event discovery, `ZRANK`, ticket metadata lookup, and client creation.
- Depends on: `github.com/redis/go-redis/v9` and `pkg/log`.
- Used by: `internal/api/` and `internal/scheduler/`.

**Admission/application layer:**
- Purpose: Periodically turn queued tickets into signed admission events while enforcing rate, capacity, and single-scheduler ownership.
- Location: `internal/scheduler/`
- Contains: ticker lifecycle, event scan, per-event lock, batch admission, and Redis pub/sub publication.
- Depends on: `internal/store`, `internal/config`, Redis, token callback, and logging.
- Used by: `cmd/scheduler/main.go`.

**Security/token layer:**
- Purpose: Define and verify admission and session credentials.
- Location: `internal/token/`
- Contains: JWT claims, issue functions, and validation functions.
- Depends on: `github.com/golang-jwt/jwt/v5`, UUID generation, and TTL constants in `internal/config/config.go`.
- Used by: `cmd/scheduler/main.go`, `pkg/middleware/queue_guard.go`, and token tests.

**Origin integration layer:**
- Purpose: Enforce queue admission at the protected origin and expose the local checkout simulation.
- Location: `pkg/middleware/queue_guard.go`, `cmd/stuborigin/main.go`
- Contains: session fast path, admission `SETNX`, cookie transitions, and checkout HTML.
- Depends on: `internal/token`, `internal/config`, Redis, Gin.
- Used by: `cmd/stuborigin/main.go`; the middleware is reusable by another origin binary.

**Presentation layer:**
- Purpose: Serve static queue and admin pages and run browser-side transport/control logic.
- Location: `web/queue/`, `web/admin/`
- Contains: HTML shells, plain JavaScript, and CSS. `web/` is mounted read-only into nginx by `docker-compose.yml`.
- Depends on: Queue API endpoints and `window.QUEUE_CONFIG.apiBase`.
- Used by: Browser clients through the `static-pages` service.

## Data Flow

### Primary Queue and Admission Path

1. A browser requests `GET /queue/join?eventId=...&target=...`; `internal/api/handler_join.go:34` validates the event, reuses a `q_ticket` still present in the sorted set, or creates a UUID ticket with `ZADD` and ticket hash metadata.
2. The handler redirects to `QueuePageURL` with the ticket and target. `web/queue/queue.js:8` stores the target in `sessionStorage` and starts an immediate poll plus a five-second interval.
3. `internal/api/handler_status.go:18` reads the ticket's event, checks the scheduler-written `admission_token`, otherwise returns `ZRANK` plus `upgrade_to_sse` and `constrained` state.
4. For ranks below the threshold, `web/queue/queue.js:56` stops polling and opens `GET /queue/status/:ticketId?mode=sse`. `internal/api/handler_status.go:81` subscribes before reading the initial rank, then listens for queue ticks and ticket admission events.
5. `internal/scheduler/admission.go:36` scans `queue:*`, locks each event, calculates `min(rate, capacity-active)` when capacity is configured, and uses `ZPOPMIN` to remove FIFO tickets.
6. For each popped ticket, the scheduler signs an admission JWT through the callback supplied by `cmd/scheduler/main.go:27`, stores it in the ticket hash, increments active users, publishes a ticket-specific admitted payload, and publishes an event tick.
7. The browser's `handleAdmitted` in `web/queue/queue.js:18` writes `q_admission`, closes transport, and navigates to the saved target.
8. `pkg/middleware/queue_guard.go:31` validates `q_session` first. Otherwise it validates `q_admission`, performs `SETNX token:{jti}`, issues `q_session`, clears `q_admission`, stores session claims in Gin context, and calls the protected handler.
9. The stub origin reads those claims in `cmd/stuborigin/main.go:39`, renders the checkout page, and posts `/queue/exit` on completion. `internal/api/handler_exit.go:14` decrements the event active counter.

### Admin Configuration Flow

1. `web/admin/admin.js:6` fetches `GET /queue/events` and selects the first active event.
2. It fetches `GET /queue/config/:eventId` every two seconds and renders queue depth, active users, configured rate, capacity, headroom, and estimated drain time.
3. Submitting the form sends `PUT /queue/rate/:eventId` with rate and optional capacity. `internal/api/handler_admin.go:15` writes the corresponding Redis keys; the scheduler reads them on its next tick.

**State Management:**

- Queue state is Redis state: `queue:{eventId}` sorted sets, `ticket:{ticketId}` hashes, counters, and scheduler locks are named in `internal/store/keys.go`.
- Browser state is limited to cookies, `sessionStorage.q_target`, a polling timer, an `EventSource`, and an in-memory duplicate-navigation guard in `web/queue/queue.js`.
- No application database, process-local queue, global mutable registry, or frontend framework state store is present.

## Key Abstractions

**`api.Handler`:**
- Purpose: Holds shared queue API configuration and Redis dependency for all HTTP handlers.
- Examples: `internal/api/handler_join.go:19`, `internal/api/router.go:10`.
- Pattern: Constructor-based dependency injection through `NewHandler`.

**Redis key constructors:**
- Purpose: Keep queue, ticket, counter, pub/sub, and lock namespaces consistent.
- Examples: `internal/store/keys.go:12-19`.
- Pattern: Small pure functions; callers should use these rather than inline key strings. `pkg/middleware/queue_guard.go:59` is the current exception for the one-time token key.

**Token issue callback:**
- Purpose: Let the scheduler admit tickets without depending directly on token package details.
- Examples: `internal/scheduler/admission.go:18`, `cmd/scheduler/main.go:27`.
- Pattern: A function value is injected into `NewScheduler`; the production composition supplies `token.IssueAdmission` with configured secret.

**`middleware.QueueGuard`:**
- Purpose: Reusable Gin middleware for a protected origin.
- Examples: `pkg/middleware/queue_guard.go:31`, `cmd/stuborigin/main.go:39`.
- Pattern: Configuration struct captures secrets, event, Redis, join URL, and cookie security mode; successful validation places `*token.SessionClaims` under the `session` Gin context key.

## Entry Points

**Queue API:**
- Location: `cmd/queueserver/main.go`
- Triggers: Docker `queueserver` service or direct Go execution.
- Responsibilities: Load `QueueServerConfig`, create queue Redis client, construct `api.Handler` and Gin router, listen on configured port.

**Admission scheduler:**
- Location: `cmd/scheduler/main.go`
- Triggers: Docker `scheduler` service or direct Go execution.
- Responsibilities: Load scheduler config, create Redis client, inject JWT signing, and run until SIGTERM/SIGINT.

**Stub protected origin:**
- Location: `cmd/stuborigin/main.go`
- Triggers: Docker `stuborigin` service or direct Go execution.
- Responsibilities: Load origin secrets/config, build `QueueGuard`, expose protected checkout at `/`, and listen on port 8081.

**Static pages:**
- Location: `web/queue/index.html`, `web/admin/index.html`
- Triggers: nginx `static-pages` service in `docker-compose.yml`.
- Responsibilities: Load browser scripts/styles and inject the local API base URL.

## Architectural Constraints

- **Threading:** Gin handles concurrent HTTP requests; the scheduler uses one ticker goroutine and synchronous Redis calls per event. SSE handlers block per connection while waiting on pub/sub, heartbeat, or request cancellation.
- **Global state:** Browser scripts use module-level timers and connection variables in `web/queue/queue.js` and `web/admin/admin.js`; server packages do not maintain a process-local queue registry.
- **Circular imports:** No circular dependency chain is detected. `internal/config` supplies constants to token and middleware; `internal/store` is shared by API and scheduler without importing either.
- **Redis separation:** Docker uses `redis-queue` for API/scheduler queue coordination and `redis-origin` for origin one-time token consumption, configured by `docker-compose.yml`.
- **Transport threshold:** The server default is `SSE_THRESHOLD=200` in `internal/config/config.go:70`; the browser also has a literal `SSE_THRESHOLD=200` in `web/queue/queue.js:6`, so changing the server threshold requires keeping the client crossover behavior aligned.
- **Context shutdown:** `cmd/queueserver/main.go:22` creates a signal context but reserves it rather than passing it into `router.Run`; scheduler shutdown is wired through `Start(ctx)`.

## Anti-Patterns

### Inline token key construction

**What happens:** `pkg/middleware/queue_guard.go:59` builds `"token:"+claims.ID` directly instead of calling a constructor in `internal/store/keys.go`.

**Why it's wrong:** Redis namespaces can drift when key formats change, and the single-source-of-truth rule in `internal/store/keys.go` is bypassed.

**Do this instead:** Add/use a token JTI key constructor in `internal/store/keys.go` when another caller needs the same namespace.

### Unauthenticated admin endpoints

**What happens:** `internal/api/handler_admin.go:14` and `internal/api/handler_events.go:14` expose event discovery and rate/capacity writes without authentication.

**Why it's wrong:** Any network client that can reach the queue API can change admission behavior or inspect event state.

**Do this instead:** Put authentication or a network-level allowlist at the router boundary before exposing these routes outside the local stack.

## Error Handling

**Strategy:** Handlers return HTTP errors for request/Redis failures; scheduler failures are logged and skip the affected operation; the origin returns a minimal HTML error page for missing/invalid credentials.

**Patterns:**

- Validate required query/body values and return `400` in `internal/api/handler_join.go:37` and `internal/api/handler_exit.go:18`.
- Return `404` when a ticket hash cannot provide an event ID in `internal/api/handler_status.go:22`.
- Treat missing/invalid admission or session credentials as `401`, and replay or Redis `SETNX` failure as `403`, in `pkg/middleware/queue_guard.go:43-62`.
- Log scheduler Redis and token-publication failures through `pkg/log/log.go`; most non-critical Redis writes in handlers are intentionally best-effort.

## Cross-Cutting Concerns

**Logging:** `pkg/log/log.go` wraps `log/slog` with context-aware Info, Error, Warn, and Debug functions. API requests also use Gin's logger/recovery middleware in `internal/api/router.go:12`.

**Validation:** Request-level validation is inline in Gin handlers. JWT validation is centralized in `internal/token/jwt.go` and `internal/token/session.go`; Redis rank and ticket lookups are centralized in `internal/store/redis.go`.

**Authentication:** Admission and session JWTs use HMAC-SHA256 with separate `ADMISSION_SECRET` and `SESSION_SECRET`. One-time admission use is enforced at the origin with Redis `SETNX` in `pkg/middleware/queue_guard.go`.

**CORS:** `internal/api/router.go:15` allows only the local queue and origin page origins and handles `OPTIONS` requests globally.

**Observability:** There is structured application logging and a `/health` JSON endpoint; metrics, tracing, and external error tracking are not detected.

---

*Architecture analysis: 2026-08-29*
