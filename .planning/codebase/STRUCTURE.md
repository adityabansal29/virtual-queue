# Codebase Structure

**Analysis Date:** 2026-08-29

## Directory Layout

```text
virtual-queue/
├── cmd/
│   ├── queueserver/       # Queue HTTP API executable
│   ├── scheduler/         # Admission scheduler executable
│   └── stuborigin/        # Protected checkout origin executable
├── internal/
│   ├── api/               # Gin router and queue/admin handlers
│   ├── config/            # Environment loading and shared TTL constants
│   ├── scheduler/         # Admission loop and Redis event locks
│   ├── store/             # Redis client, key namespace, and reads
│   └── token/             # Admission/session JWTs and token tests
├── pkg/
│   ├── log/               # Context-aware slog wrappers
│   └── middleware/         # Reusable protected-origin QueueGuard
├── web/
│   ├── queue/              # Static waiting-room page
│   └── admin/              # Static operator dashboard
├── scripts/                # Shell verification workflow
├── test-screenshots/       # Browser verification evidence
├── .planning/              # GSD project, phase, and codebase artifacts
├── Dockerfile.*            # Per-process multi-stage container builds
├── docker-compose.yml       # Local Redis, Go services, and nginx stack
├── Makefile                 # Local lifecycle, test, build, and verify commands
├── nginx.conf               # Static-page nginx server config
├── go.mod / go.sum          # Go module and dependency checksums
├── DESIGN.md                # High-level system design
└── README.md                # Short project description
```

## Directory Purposes

**`cmd/`:**
- Purpose: Executable composition roots; keep process startup and dependency wiring here.
- Contains: One `main.go` per independently deployed local service.
- Key files: `cmd/queueserver/main.go`, `cmd/scheduler/main.go`, `cmd/stuborigin/main.go`.

**`internal/api/`:**
- Purpose: Queue HTTP API implementation.
- Contains: `Handler`, router registration, queue join/status/exit handlers, and admin handlers.
- Key files: `internal/api/router.go`, `internal/api/handler_join.go`, `internal/api/handler_status.go`.

**`internal/config/`:**
- Purpose: Service-specific environment configuration and shared lifetime constants.
- Contains: `QueueServerConfig`, `SchedulerConfig`, `StubOriginConfig`, environment parsing helpers, Redis/JWT/cookie TTLs.
- Key file: `internal/config/config.go`.

**`internal/scheduler/`:**
- Purpose: Admission control orchestration.
- Contains: `Scheduler`, event scan/tick logic, batch admission, and distributed lock helpers.
- Key files: `internal/scheduler/admission.go`, `internal/scheduler/leader_lock.go`.

**`internal/store/`:**
- Purpose: Redis naming and low-level queue reads/client construction.
- Contains: Key constructor functions, event `SCAN`, `ZRANK`, ticket event lookup, and Redis client setup.
- Key files: `internal/store/keys.go`, `internal/store/redis.go`.

**`internal/token/`:**
- Purpose: Credential types and HMAC JWT operations.
- Contains: Admission/session claims and issue/validate functions.
- Key files: `internal/token/jwt.go`, `internal/token/session.go`, `internal/token/jwt_test.go`.

**`pkg/middleware/`:**
- Purpose: Origin-facing middleware that is importable by a protected service.
- Contains: `QueueGuard` and its HTML error-page helper.
- Key file: `pkg/middleware/queue_guard.go`.

**`pkg/log/`:**
- Purpose: Central logging seam.
- Contains: Context-aware wrappers around `log/slog`.
- Key file: `pkg/log/log.go`.

**`web/queue/`:**
- Purpose: Static user waiting room.
- Contains: `index.html`, `queue.js`, and `queue.css`; no build step or frontend framework is present.
- Key files: `web/queue/index.html`, `web/queue/queue.js`.

**`web/admin/`:**
- Purpose: Static operator dashboard.
- Contains: `index.html`, `admin.js`, and `admin.css`; it calls the queue API directly from the browser.
- Key files: `web/admin/index.html`, `web/admin/admin.js`.

**`.planning/`:**
- Purpose: GSD project context, requirements, roadmap, phase artifacts, and generated codebase maps.
- Contains: `PROJECT.md`, `REQUIREMENTS.md`, `ROADMAP.md`, `STATE.md`, `config.json`, `phases/`, and `codebase/`.
- Generated analysis files: `.planning/codebase/ARCHITECTURE.md` and `.planning/codebase/STRUCTURE.md`.

**`test-screenshots/`:**
- Purpose: Captured browser verification evidence for queue states.
- Contains: PNG screenshots such as `test-screenshots/03--sse-crossover.png` and `test-screenshots/05--admitted-checkout.png`.

## Key File Locations

**Entry Points:**
- `cmd/queueserver/main.go`: Starts the Gin queue API on the configured port.
- `cmd/scheduler/main.go`: Starts the Redis-backed admission loop.
- `cmd/stuborigin/main.go`: Starts the protected checkout origin on port 8081.
- `web/queue/index.html`: Browser waiting-room entry document.
- `web/admin/index.html`: Browser admin-dashboard entry document.

**Configuration:**
- `internal/config/config.go`: Environment names, defaults, service config structs, and TTL constants.
- `docker-compose.yml`: Local service topology, ports, Redis instances, and environment-file wiring.
- `Makefile`: Supported local commands.
- `nginx.conf`: Static asset serving rules.
- `Dockerfile.queueserver`, `Dockerfile.scheduler`, `Dockerfile.stuborigin`: Per-binary build/deploy definitions.

**Core Logic:**
- `internal/api/handler_join.go`: Ticket creation and queue-page redirect.
- `internal/api/handler_status.go`: Poll/SSE position and admission delivery.
- `internal/scheduler/admission.go`: Rate/capacity-gated FIFO admission.
- `internal/store/keys.go`: Redis namespace contract.
- `pkg/middleware/queue_guard.go`: One-time admission and session enforcement.
- `internal/token/jwt.go`, `internal/token/session.go`: Credential lifecycle.

**Testing:**
- `internal/token/jwt_test.go`: The only Go test file; covers JWT claims, wrong-secret rejection, expiry, JTI uniqueness, and admission/session isolation.
- `scripts/verify.sh`: Shell-based stack verification for join, poll, SSE, one-time origin admission, and secret isolation.
- `test-screenshots/`: Manual/browser verification artifacts.

## Naming Conventions

**Files:**
- Go files use lowercase snake-free names describing the concern, such as `handler_status.go`, `leader_lock.go`, and `config.go`.
- Go tests use the standard `_test.go` suffix, currently `internal/token/jwt_test.go`.
- Static assets use lowercase descriptive names: `index.html`, `queue.js`, `queue.css`, `admin.js`, and `admin.css`.
- Container definitions use `Dockerfile.<service>` matching the executable directory under `cmd/`.

**Directories:**
- Go command directories use the executable name under `cmd/`.
- Reusable implementation is grouped by package responsibility under `internal/`.
- Browser surfaces get one lowercase directory per surface under `web/`.
- Planning documents use uppercase names under `.planning/`; phase directories use numeric prefixes and kebab-case names, such as `.planning/phases/02-frontend-admin-ui`.

**Go symbols and Redis keys:**
- Exported Go types/functions use PascalCase (`QueueServerConfig`, `NewHandler`, `GetPosition`); local variables and fields use camelCase.
- Redis namespaces are lowercase with colon-separated segments, e.g. `queue:{eventId}`, `ticket:{ticketId}`, and `scheduler:lock:{eventId}`; constructors live in `internal/store/keys.go`.
- HTTP JSON fields use lower camel case (`eventId`, `ticketId`, `upgrade_to_sse`, `estimatedDrainSec`) according to the existing endpoint contracts.

## Where to Add New Code

**New API endpoint:**
- Add the handler beside the related concern in `internal/api/handler_*.go`.
- Register it explicitly in `internal/api/router.go`.
- Reuse `api.Handler` dependencies and `internal/store` key/read helpers; add request validation at the handler boundary.

**New queue/admission behavior:**
- Put orchestration in `internal/scheduler/` and keep Redis key construction in `internal/store/keys.go`.
- Wire new dependencies from `cmd/scheduler/main.go`; preserve the injected callback pattern used by `NewScheduler`.

**New credential behavior:**
- Put claims and issue/validate functions in `internal/token/`.
- Use TTL constants from `internal/config/config.go` and extend `internal/token/jwt_test.go` or add a nearby token test file.

**New protected origin integration:**
- Reuse `pkg/middleware.QueueGuard` and provide a process-specific composition root under `cmd/`.
- Put origin-specific handlers/pages in that command package unless the behavior is shared by multiple origins.

**New static UI surface:**
- Add a directory under `web/` containing its HTML, JavaScript, and CSS.
- Mount or expose it through the static-page service in `docker-compose.yml`; use `window.QUEUE_CONFIG.apiBase` for API calls, as in `web/queue/index.html` and `web/admin/index.html`.

**New tests:**
- Co-locate Go tests with the package under test using `_test.go`.
- Extend `scripts/verify.sh` only for stack-level checks that require running Docker services.
- Store browser evidence in `test-screenshots/` when a manual UI verification produces an artifact.

**Utilities:**
- Shared Redis naming/access helpers belong in `internal/store/`.
- Shared token logic belongs in `internal/token/`.
- Shared origin request protection belongs in `pkg/middleware/`.
- Shared logging seams belong in `pkg/log/`.

## Special Directories

**`vendor/` / generated Go code:**
- Purpose: Not detected.
- Generated: Not applicable.
- Committed: Not applicable; dependencies are resolved through `go.mod` and `go.sum`.

**`node_modules/` or frontend build output:**
- Purpose: Not detected.
- Generated: Not applicable.
- Committed: Not applicable; the frontend is plain static HTML/CSS/JavaScript.

**`.planning/codebase/`:**
- Purpose: Generated architecture and codebase reference documents consumed by GSD planning/execution.
- Generated: Yes, by the codebase mapping workflow.
- Committed: Controlled by `.planning/config.json` (`commit_docs` is enabled); no source runtime imports these files.

**`test-screenshots/`:**
- Purpose: Browser verification evidence.
- Generated: Yes, by manual/browser checks.
- Committed: Present in the repository and treated as project artifacts.

---

*Structure analysis: 2026-08-29*
