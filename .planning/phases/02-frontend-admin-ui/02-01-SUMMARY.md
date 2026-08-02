---
phase: 02-frontend-admin-ui
plan: "01"
subsystem: frontend-queue-page
status: complete
tags: [frontend, nginx, cors, sse, polling, queue-page]
completed: "2026-08-03"

dependency_graph:
  requires: []
  provides:
    - web/queue/index.html
    - web/queue/queue.js
    - web/queue/queue.css
    - nginx.conf
    - static-pages docker service on port 8082
    - CORS middleware in queue API router
  affects:
    - internal/api/router.go
    - docker-compose.yml

tech_stack:
  added: []
  patterns:
    - "QUEUE_CONFIG injection (D-02): inline script in HTML head, no build tooling"
    - "CORS allowlist middleware: exact-match origin, not wildcard"
    - "Poll/SSE hybrid crossover: SSE_THRESHOLD=200, upgrade_to_sse flag from API"
    - "navigating guard: idempotent handleAdmitted on race"

key_files:
  created:
    - web/queue/index.html
    - web/queue/queue.js
    - web/queue/queue.css
    - nginx.conf
  modified:
    - docker-compose.yml
    - internal/api/router.go

decisions:
  - "D-02 config injection: first inline script in HTML head so queue.js always has apiBase"
  - "Constrained toggle uses classList.add/remove('visible') not inline style — CSS controls show/hide"
  - "navigating flag guards duplicate handleAdmitted calls from SSE+poll race"
  - "nginx.conf includes /etc/nginx/mime.types for correct JS/CSS MIME types"

metrics:
  duration_minutes: 8
  completed: "2026-08-03"
  tasks_completed: 2
  tasks_total: 2
  commits: 1

actuals:
  tokens: 5200
  tasks: 2
  commits: 1
---

# Phase 02 Plan 01: Queue Waiting Page — End-to-End Tracer Summary

**One-liner:** nginx static-pages service on port 8082, CORS allowlist middleware, and full poll/SSE crossover queue.js adapted from DESIGN.md §7 with all 7 UI-SPEC states.

## What Was Built

The complete browser-to-API path for the queue waiting room:

1. **docker-compose.yml** — `static-pages` service (nginx:alpine) on port 127.0.0.1:8082 serving `./web` read-only with the custom `nginx.conf`.

2. **nginx.conf** — Standard static file server: `listen 80`, `try_files $uri $uri/ =404`, `include /etc/nginx/mime.types` for correct JS/CSS serving.

3. **internal/api/router.go** — CORS middleware added as first `r.Use()` after gin.New(). Explicit allowlist `["http://localhost:8082", "http://localhost:8081"]` — no wildcard. OPTIONS preflight returns 204.

4. **web/queue/index.html** — Queue waiting page with `window.QUEUE_CONFIG = { apiBase: 'http://localhost:8080' }` as first script in head. DOM elements `#pos`, `#wait`, `#status` (aria-live="polite"), `#constrained` all present.

5. **web/queue/queue.js** — Full poll/SSE hybrid client adapted from DESIGN.md §7:
   - Reads `ticketId` and `target` from URL params; saves target to sessionStorage
   - `handleAdmitted(token)`: `navigating` guard, closes es/pollTimer, sets `q_admission` cookie (SameSite=Strict), redirects to sessionStorage target
   - `startSSE()`: EventSource on QUEUE_CONFIG.apiBase, `onerror` sets "Reconnecting…" without closing es
   - `pollOnce()`: fetch with error handling, "Connection lost. Retrying…" on failure, `upgrade_to_sse` crossover
   - `renderPosition(rank, data)`: "{N} people ahead", "~N min" / "Less than a minute"
   - `showConstrained(on)`: `classList.add/remove('visible')` on `#constrained`; hides `#wait` when constrained
   - null ticketId guard: shows "Checking your position…" without polling

6. **web/queue/queue.css** — Per UI-SPEC tokens: 28px/600 `#pos`, `#fef3c7` constrained banner, `overflow-wrap: break-word` backstop for 500k+ values, `#constrained.visible { display: block }`.

## Deviations from Plan

### Auto-built in Task 1 (no separate Task 2 diff)

Task 2 was a review-and-complete pass over Task 1's implementation. All 7 UI-SPEC states, the `navigating` guard, `aria-live="polite"`, and exact copywriting strings were built directly in Task 1. No net-new code was needed for Task 2. Both tasks share the single commit `c6842ed`.

None — plan executed as written. All acceptance criteria met in a single production-quality commit.

## Verification Results

| Check | Result |
|-------|--------|
| `go build ./...` | PASS |
| `docker compose config --services \| grep static-pages` | PASS |
| No hardcoded `localhost:8080` in queue.js | PASS (0 occurrences) |
| CORS header set in router.go | PASS |
| CORS wildcard `*` absent | PASS |
| All 7 queue page states implemented | PASS |
| Exact copywriting strings match contract | PASS |
| `q_admission` cookie set SameSite=Strict | PASS |
| `aria-live="polite"` on `#status` | PASS |
| `navigating` guard in handleAdmitted | PASS |

## Known Stubs

None. All behavior is implemented. The `constrained` field is handled conditionally (shows banner if present and true, hides otherwise) per planner assumption — Plan 02-03 will add the field to the scheduler+handler.

## Threat Surface Scan

No new security surface beyond plan scope. CORS allowlist (T-02-01) mitigated as planned. Open-redirect risk (T-02-03) accepted per plan — local dev scope only.

## Self-Check: PASSED

- [x] web/queue/index.html exists
- [x] web/queue/queue.js exists
- [x] web/queue/queue.css exists
- [x] nginx.conf exists
- [x] docker-compose.yml has static-pages service
- [x] internal/api/router.go has CORS middleware
- [x] Commit c6842ed exists
