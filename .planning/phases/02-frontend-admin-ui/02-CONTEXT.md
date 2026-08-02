# Phase 2: Frontend & Admin UI - Context

**Gathered:** 2026-08-02
**Status:** Ready for planning

<domain>
## Phase Boundary

A browser-accessible waiting room experience: static queue page with hybrid poll→SSE crossover and admission redirect; event-specific admin dashboard with auto-discovered events and live rate/capacity controls; enhanced stub checkout that validates `q_session` and displays ticket info with a Done button.

Phase 1 delivered all queue API endpoints. Phase 2 delivers the browser layer on top of them — no new queue mechanics, no AWS infrastructure (Phase 3).

</domain>

<decisions>
## Implementation Decisions

### Static File Serving

- **D-01:** Add a `static-pages` nginx service to Docker Compose serving a `web/` directory. Queue page at `web/queue/`, admin dashboard at `web/admin/`. Port TBD (e.g., 8082). This matches the S3/CDN prod model and keeps static serving separate from the queue API. — **Reversibility:** reversible
- **D-02:** Queue page and admin dashboard use `window.QUEUE_CONFIG = { apiBase: '...' }` injected as an inline `<script>` in the HTML file. Client JS reads `QUEUE_CONFIG.apiBase` instead of hardcoded URLs. Local HTML uses `http://localhost:8080`; S3 build uses the prod URL. Zero build tooling required. — **Reversibility:** reversible
- **D-03:** CORS on the queue API allows localhost origins only (not wildcard `*`). The queue API adds `Access-Control-Allow-Origin` for `http://localhost:8082` (or the configured nginx port). Also applies to the stub origin endpoints called from the browser. — **Reversibility:** reversible

### Admin Dashboard

- **D-04:** Admin dashboard is vanilla HTML + CSS + JS — no build step, no npm. Single HTML file with linked CSS and JS. Consistent with the queue page approach and the "simple web admin dashboard" intent in DESIGN.md. — **Reversibility:** reversible
- **D-05:** Dashboard polls `GET /queue/config/:eventId` every 2 seconds for live stats. No new server-side streaming endpoint needed. Acceptable staleness for an ops dashboard. — **Reversibility:** reversible
- **D-06:** No authentication on the admin dashboard (carries forward Phase 1 D-06 intent documented in `handler_admin.go` ponytail comment). Local Docker Compose only. — **Reversibility:** reversible
- **D-07:** Dashboard is event-specific. Admin selects an event from a dropdown auto-populated by a new `GET /queue/events` endpoint that scans Redis for active `queue:*` keys. Selected event context drives all stats display and update controls. — **Reversibility:** reversible
- **D-08:** Dashboard shows all config stats: queue depth, active users, admit rate, capacity, headroom (`capacity - active`), estimated drain time. Edit controls: rate input + capacity input + Submit → `PUT /queue/rate/:eventId`. Default config (from `cfg.DefaultAdmitRate`) displayed when no event-specific config exists. — **Reversibility:** reversible

### New API Endpoint

- **D-09:** Add `GET /queue/events` to the queue server router. Returns a list of eventIds discovered by scanning Redis keys matching `queue:*`. Required for the admin dashboard event selector. — **Reversibility:** reversible

### Stub Checkout (UI-08)

- **D-10:** Stub checkout handler calls `token.ValidateSession()` on the `q_session` cookie to extract `EventID` and `Subject` (ticketId) from `SessionClaims`. No Gin context change to QueueGuard needed — the handler decodes the cookie directly. — **Reversibility:** reversible
- **D-11:** Stub checkout displays event ID, ticket ID, and a "Complete Purchase" button. Clicking "Complete" calls `POST /queue/exit` (queue API, `http://localhost:8080/queue/exit`) with `{ "eventId": "..." }` from the browser JS to decrement `active:{eventId}` on redis-queue and free the slot. — **Reversibility:** reversible
- **D-12:** Missing or expired `q_session` at stub origin → error page ("Your session has expired. Return to the queue.") with a link back to the queue page. QueueGuard already handles the redirect for the admission path; the checkout handler adds an explicit check for the fast-path session validation failure. — **Reversibility:** reversible

### Active Counter Architecture (clarification for planners)

The `active:{eventId}` counter exists on **both** Redis instances with different purposes:
- **redis-queue**: incremented by scheduler at ZPOPMIN admit; decremented by `POST /queue/exit`. This is what the admin dashboard reads and what the scheduler will use for headroom (Phase 2 capacity enforcement, D-06 from Phase 1).
- **redis-origin**: incremented by QueueGuard on first admission through origin. For future origin-side capacity enforcement (not admin dashboard).

No cross-Redis writes needed. The two counters serve different purposes and are not synchronized.

### Claude's Discretion

- nginx config details (port assignment, MIME types, try_files behavior)
- `web/` directory structure within the repo
- CSS styling approach for queue page and admin dashboard
- Error page visual design
- Whether `GET /queue/events` uses `KEYS queue:*` or `SCAN` (should use SCAN for safety)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Architecture & Design

- `DESIGN.md` — Complete HLD v3. Section 7 has the reference `queue.js` client code (poll/SSE crossover, `handleAdmitted`, `renderPosition`). Section 8 has QueueGuard middleware. **Read before planning any component.** The client JS is the reference implementation — do not rewrite it from scratch.
- `DESIGN.md §7` — Queue Waiting Page client JS (lines ~363–421). This is the canonical queue.js implementation. Phase 2 adapts it to use `window.QUEUE_CONFIG.apiBase` instead of hardcoded URLs.

### Requirements

- `.planning/REQUIREMENTS.md` — Phase 2 requirements: UI-01 through UI-08. Full acceptance criteria.
- `.planning/PROJECT.md` — Active requirements, constraints (Go + Redis + Docker Compose stack), out-of-scope items.

### Phase Roadmap

- `.planning/ROADMAP.md` — Phase 2 goal, 5 success criteria, and dependency on Phase 1.

### Phase 1 Context (carried-forward decisions)

- `.planning/phases/01-queue-core-local-stack/01-CONTEXT.md` — Phase 1 decisions: D-01 (stub origin as separate binary), D-02 (separate Redis instances), D-03 (stub origin owns SETNX + q_session), D-05 (stub origin returns HTML), D-07 (Go module layout), D-08 (standard Go layout).

### Existing Code

- `pkg/middleware/queue_guard.go` — QueueGuard middleware: SETNX enforcement, active INCR on redis-origin, q_session issuance. Checkout handler uses `token.ValidateSession()` to read claims.
- `internal/token/session.go` — `SessionClaims` struct (EventID + Subject/ticketId). `IssueSession` / `ValidateSession` functions.
- `internal/api/handler_admin.go` — `GetConfig` returns queueDepth, activeUsers, admitRate, capacity, estimatedDrainSec. `UpdateRate` sets rate and capacity.
- `internal/api/handler_exit.go` — `POST /queue/exit` decrements `active:{eventId}` on redis-queue. Browser JS in stub checkout calls this.
- `internal/api/router.go` — Current routes. Phase 2 adds `GET /queue/events`.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- `internal/token/session.go` — `ValidateSession(tokenString, secret)` returns `*SessionClaims{EventID, Subject(=ticketId)}`. Stub checkout handler calls this directly on the `q_session` cookie.
- `internal/api/handler_admin.go` — `GetConfig` already returns all fields the admin dashboard needs: `queueDepth`, `activeUsers`, `admitRate`, `capacity`, `estimatedDrainSec`.
- `DESIGN.md §7` queue.js — Complete client implementation for the queue page. Adapt by replacing hardcoded `https://queue-api.yourdomain.com` with `window.QUEUE_CONFIG.apiBase`.

### Established Patterns

- Separate Docker Compose services per concern (redis-queue, redis-origin, queueserver, stuborigin) — nginx follows the same pattern.
- Two Redis containers: queue service never touches redis-origin and vice versa. Admin dashboard and scheduler both use redis-queue.
- Gin router pattern: add `GET /queue/events` to `router.go` using the same `h.Handler` receiver pattern.

### Integration Points

- `docker-compose.yml` — Add `static-pages` (nginx) service. Set QUEUE API base URL via nginx config or HTML injection.
- `internal/api/router.go` — Add `GET /queue/events` route.
- `cmd/stuborigin/` — Checkout handler enhancement: call `ValidateSession()`, render ticket info page, handle error case.
- CORS middleware in queue server: add before existing routes in `router.go`.

</code_context>

<specifics>
## Specific Ideas

- Admin dashboard event selector is a `<select>` dropdown populated on page load from `GET /queue/events`. Selecting an event triggers an immediate stats fetch and starts the 2s polling loop for that event.
- `GET /queue/events` should use Redis `SCAN` (not `KEYS`) to find `queue:*` keys safely — `KEYS` blocks on large keyspaces.
- The queue page `window.QUEUE_CONFIG` injection: place it as the first `<script>` tag in `<head>` before loading `queue.js` so the config is always available when queue.js executes.
- "Complete Purchase" button in stub checkout fires `fetch('http://localhost:8080/queue/exit', { method: 'POST', body: JSON.stringify({ eventId }) })` then shows a "Thank you" state in-page (no redirect needed).

</specifics>

<deferred>
## Deferred Ideas

- Capacity ceiling enforcement in the scheduler (`min(rate, headroom)` check) — Phase 1 D-06, still deferred. The `active:{eventId}` counter on redis-queue is now accurate, so Phase 2 is the right time to wire this up IF it falls within scope. But it's a scheduler change (Go), not a frontend change — could be Phase 2 plan wave 1 or Phase 3 setup task. Planner to decide.
- Admin dashboard auth (bearer token or IP allowlist) — Phase 3 / prod hardening. Intentionally deferred.
- S3/CloudFront deployment of static files — Phase 3 (INFRA-04).
- Real Akamai EdgeWorker deployment — v2 backlog.

</deferred>

---

*Phase: 2-Frontend & Admin UI*
*Context gathered: 2026-08-02*
