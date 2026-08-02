---
phase: 02-frontend-admin-ui
verified: 2026-08-03T00:00:00Z
status: human_needed
score: 5/5 must-haves verified
behavior_unverified: 2
overrides_applied: 0
behavior_unverified_items:
  - truth: "When rank drops below 200 the browser switches from polling to SSE automatically (no page reload)"
    test: "Open queue page with a ticket whose rank is >= 200, watch it drop below 200 via scheduler admissions, observe network tab — should close the poll interval and open an EventSource connection without reload"
    expected: "Network tab shows EventSource connection established; polling requests stop"
    why_human: "The poll→SSE crossover is a runtime state transition (clearInterval + new EventSource). Code is wired correctly — pollOnce() calls clearInterval + startSSE() when upgrade_to_sse is true — but no test exercises the actual transition with a live browser."
  - truth: "On admission the browser sets the q_admission cookie and redirects to the stub checkout page without manual intervention"
    test: "Join queue, wait for scheduler to admit the ticket, observe: (a) q_admission cookie appears in DevTools, (b) page redirects to stuborigin without any user action, (c) QueueGuard validates the token, issues q_session, and renders the seat grid"
    expected: "Seamless redirect to checkout with seat grid displayed; no 403 or session-expired page"
    why_human: "The full admission flow involves: scheduler writing admission_token to Redis, poll/SSE delivering it to the browser, queue.js setting a cookie and redirecting, QueueGuard validating and issuing q_session. All code paths are wired correctly but the end-to-end state machine requires a running stack and browser observation."
human_verification:
  - test: "Poll → SSE crossover in browser"
    expected: "Network tab shows EventSource opening after rank drops below 200; no page reload"
    why_human: "Runtime state transition — clearInterval + new EventSource cannot be verified by grep or static analysis"
  - test: "Full admission flow end-to-end"
    expected: "Scheduler admits ticket → poll/SSE delivers token → queue.js sets q_admission cookie + redirects → QueueGuard issues q_session → seat grid renders"
    why_human: "Multi-component state machine across scheduler, Redis pub/sub, browser, and middleware. All pieces are wired correctly but only observable in a running stack."
---

# Phase 02: Frontend & Admin UI — Verification Report

**Phase Goal:** A browser-accessible waiting room experience — the static queue page polls, upgrades to SSE near the front, redirects on admission, and the admin dashboard lets an operator adjust rate and capacity live.
**Verified:** 2026-08-03
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC1 | Opening queue page shows "X people ahead" and estimated wait time, without reloading when position updates arrive | VERIFIED | `queue.js:72` renders `rank + ' people ahead'`; `~N min` ETA computed from admitRatePerMin; SSE path uses `es.addEventListener('update')` — DOM updated in-place, no reload |
| SC2 | Rank < 200 triggers browser switch from polling to SSE automatically (no page reload) | PRESENT_BEHAVIOR_UNVERIFIED | Code path exists: `pollOnce()` checks `data.upgrade_to_sse && !es`, calls `clearInterval(pollTimer)` then `startSSE()`. Server emits `upgrade_to_sse: rank < SSEThreshold` (`handler_status.go:54`). Wiring is complete; transition is a runtime state change — no test exercises it. |
| SC3 | On admission browser sets q_admission cookie and redirects to stub checkout without manual intervention | PRESENT_BEHAVIOR_UNVERIFIED | Cookie set at `queue.js:23` with SameSite=Strict; redirect at `queue.js:24`; `navigating` guard prevents double-fire. Full flow requires running stack. |
| SC4 | Stub checkout validates q_session cookie; missing/expired cookie shows error | VERIFIED | `stuborigin/main.go:42-55`: reads `q_session` cookie, calls `token.ValidateSession()`, returns `errorPage()` (401) on failure or `checkoutPage()` (200) on success. Both paths fully implemented. |
| SC5 | Admin dashboard displays live queue depth, active users, admit rate, capacity, headroom; changing rate/capacity takes effect on next scheduler tick | VERIFIED | `GetConfig` returns all 5 fields; `admin.js:55-74` renders them; headroom computed client-side `(capacity - activeUsers)`; `PUT /queue/rate` wired in `router.go:50` and consumed by `admin.js:107-110`; scheduler reads `rate:` on every tick. |

**Score:** 3/5 truths presence-verified + 2/5 present-but-behavior-unverified (runtime transitions)

### Requirement-by-Requirement Verdict

| Req | Description | Status | Evidence |
|-----|-------------|--------|----------|
| UI-01 | Static file serving via nginx on :8082 | PASS | `docker-compose.yml:48-54`: `static-pages` service, `nginx:alpine`, port `127.0.0.1:8082:80`, `./web:/usr/share/nginx/html:ro`, `nginx.conf` mounted. `nginx.conf` has `root /usr/share/nginx/html`, `include /etc/nginx/mime.types`. |
| UI-02 | Poll→SSE upgrade at rank < 200 | PASS (wiring) | `handler_status.go:54` emits `upgrade_to_sse: rank < int64(h.cfg.SSEThreshold)`; `config.go:28` defaults `SSEThreshold=200`; `queue.js:57-60` acts on the flag. Full runtime transition is human-verifiable (see below). |
| UI-03 | Position display with "X people ahead" + ETA | PASS | `queue.js:72-76`: `rank + ' people ahead'`; ETA = `Math.ceil(rank / admitRatePerMin) + ' min'`; `admitRatePerMin` seeded from `data.admitRate * 60` on each poll response. |
| UI-04 | Constrained banner shown/hidden correctly | PASS | `handler_status.go:45-55`: emits `constrained: capacity > 0 && (capacity-active) <= 0`; `queue.js:79-89`: `classList.add/remove('visible')` on `#constrained`; `queue.css:37-48`: `#constrained { display:none }` / `.visible { display:block }`. Full round-trip wired. |
| UI-05 | Admission sets cookie + redirects | PASS (wiring) | `queue.js:23-24`: `document.cookie = 'q_admission=...; SameSite=Strict'` then `window.location.href`. `navigating` guard at line 19. Runtime observation needed (see Human Verification). |
| UI-06 | Admin dashboard with event selector + 6 stat cards | PASS | `web/admin/index.html`: `#event-select` dropdown, 6 stat cards (`stat-depth`, `stat-active`, `stat-rate`, `stat-capacity`, `stat-headroom`, `stat-drain`). `admin.js:6-28`: `loadEvents()` populates selector from `/queue/events`. `handler_events.go`: Redis SCAN on `queue:*` keys. |
| UI-07 | Rate/capacity update form | PASS | `admin.js:96-121`: form submit → `PUT /queue/rate/{eventId}` with `{rate, capacity}` body; "Saved" feedback at 1.5s; error display on failure. `handler_admin.go:14-43`: writes to `rate:` and `capacity:` Redis keys. |
| UI-08 | Stub checkout: session validation, seat grid, exit call | PASS | `stuborigin/main.go`: `QueueGuard` middleware + explicit `token.ValidateSession()`; `checkoutPage()`: 3x4 seat grid (12 buttons), Seat 1 pre-selected, Complete Purchase → `POST /queue/exit`; double-submit guard (`btn.disabled = true`). `errorPage()`: session-expired with return link. |

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `web/queue/index.html` | VERIFIED | Exists, substantive, served by nginx |
| `web/queue/queue.js` | VERIFIED | Exists, full poll/SSE/admission implementation |
| `web/queue/queue.css` | VERIFIED | Exists, design tokens per UI-SPEC |
| `nginx.conf` | VERIFIED | Exists, correct root and mime.types |
| `web/admin/index.html` | VERIFIED | Exists, 6 stat cards + event selector + rate form |
| `web/admin/admin.js` | VERIFIED | Exists, loadEvents/fetchConfig/renderStats/update fully wired |
| `web/admin/admin.css` | VERIFIED | Exists |
| `internal/api/handler_events.go` | VERIFIED | Exists, Redis SCAN loop, nil-guarded response |
| `internal/api/handler_status.go` | VERIFIED | constrained field added (lines 43-55) |
| `internal/scheduler/admission.go` | VERIFIED | min(rate, headroom) capacity enforcement at lines 74-83 |
| `cmd/stuborigin/main.go` | VERIFIED | Full checkout: session validation, seat grid, exit call |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `queue.js` | `/queue/status/:id?mode=poll` | `fetch(QUEUE_CONFIG.apiBase + '/queue/status/' + ticketId + '?mode=poll')` | WIRED |
| `queue.js` | `/queue/status/:id?mode=sse` | `new EventSource(QUEUE_CONFIG.apiBase + '/queue/status/' + ticketId + '?mode=sse')` | WIRED |
| `handler_status.go` | Redis `capacity:/active:` keys | `h.rdb.Get(ctx, "capacity:"+eventID)` / `h.rdb.Get(ctx, "active:"+eventID)` | WIRED |
| `scheduler/admission.go` | Redis `capacity:/active:` keys | `s.rdb.Get(ctx, "active:"+eventID)` / `s.rdb.Get(ctx, "capacity:"+eventID)` | WIRED |
| `admin.js` | `/queue/config/:eventId` | `fetch(QUEUE_CONFIG.apiBase + '/queue/config/' + currentEventId)` every 2s | WIRED |
| `admin.js` | `PUT /queue/rate/:eventId` | `fetch(..., {method:'PUT', body: JSON.stringify({rate, capacity})})` | WIRED |
| `handler_events.go` | Redis `queue:*` keys | Redis SCAN cursor loop | WIRED |
| `router.go` | `handler_events.GetEvents` | `r.GET("/queue/events", h.GetEvents)` | WIRED |
| `stuborigin/main.go` | `token.ValidateSession` | Direct call on `q_session` cookie | WIRED |
| `stuborigin/main.go` | `POST /queue/exit` | Inline JS fetch in `checkoutPage()` | WIRED |
| `docker-compose.yml` | `./web` directory | `./web:/usr/share/nginx/html:ro` volume | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `admin.js renderStats` | `data.queueDepth` | `GET /queue/config` → `h.rdb.ZCard(ctx, "queue:"+eventID)` | Yes — live Redis ZCARD | FLOWING |
| `admin.js renderStats` | `data.activeUsers` | `GET /queue/config` → `h.rdb.Get("active:"+eventID)` | Yes — live Redis counter | FLOWING |
| `queue.js renderPosition` | `rank` | `GET /queue/status` → `store.GetPosition` (ZRANK on sorted set) | Yes — live Redis ZRANK | FLOWING |
| `queue.js showConstrained` | `data.constrained` | `handler_status.go` reads `capacity:/active:` from Redis | Yes — live computation | FLOWING |
| `admin.js loadEvents` | `events[]` | `GET /queue/events` → Redis SCAN on `queue:*` keys | Yes — live SCAN | FLOWING |

### Behavioral Spot-Checks

`go build ./...` — exit 0 (verified)
`go vet ./...` — exit 0 (verified)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go compilation clean | `go build ./...` | exit 0, no output | PASS |
| Go vet clean | `go vet ./...` | exit 0, no output | PASS |
| nginx static-pages service defined | `grep static-pages docker-compose.yml` | Found at port 8082 | PASS |
| GetEvents wired in router | `grep GetEvents router.go` | `r.GET("/queue/events", h.GetEvents)` | PASS |
| constrained field in poll response | `grep constrained handler_status.go` | Computed and returned in `gin.H` | PASS |
| min(rate, headroom) in scheduler | `grep 'min(' admission.go` | `n = min(rate, headroom)` at line 79 | PASS |
| token.ValidateSession in stuborigin | `grep ValidateSession stuborigin/main.go` | Direct call at line 48 | PASS |
| No hardcoded localhost in queue.js | `grep localhost:8080 web/queue/queue.js` | 0 occurrences (all via QUEUE_CONFIG.apiBase) | PASS |
| No hardcoded localhost in admin.js | `grep localhost:8080 web/admin/admin.js` | 0 occurrences (all via QUEUE_CONFIG.apiBase) | PASS |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/scheduler/admission.go` | 18 | `TODO Plan 03: replace stub issueToken...` | WARNING | Stale comment — `queueserver/main.go:37` already injects `token.IssueAdmission`. The TODO was written during Phase 1 and never removed. No functional impact; the real issuer is already wired. |

No TBD, FIXME, or XXX markers found in phase-modified files.

### Human Verification Required

#### 1. Poll → SSE Crossover

**Test:** Join queue with a real ticket. In DevTools Network tab, observe polling requests every 5s. When the scheduler admits enough users to bring rank below 200, verify that polling stops and an EventSource connection (`?mode=sse`) opens — all without a page reload.
**Expected:** Network tab transitions from repeated XHR polls to a persistent SSE connection; `es.onerror` never fires on a stable connection.
**Why human:** Runtime state transition — `clearInterval(pollTimer)` + `new EventSource(...)` executed in a browser. The code path is correctly wired (`queue.js:57-59`) but the state change only manifests in a running browser session.

#### 2. End-to-End Admission Flow

**Test:** With `docker compose up`, join the queue, wait for scheduler to admit the ticket (or reduce rate to 0 then back to 1 to force immediate admission), and observe: (a) the q_admission cookie appears in DevTools; (b) the page redirects to `localhost:8081/` without user action; (c) QueueGuard validates the token (SETNX succeeds), issues q_session, clears q_admission; (d) the seat grid renders with event/ticket IDs visible; (e) clicking Complete Purchase shows the success message.
**Expected:** Seamless browser flow from queue wait → seat selection → confirmation. No 403 or session-expired page.
**Why human:** Multi-component state machine: scheduler → Redis pub/sub → browser EventSource/poll → cookie write → redirect → QueueGuard middleware → session cookie → checkout render. All individual pieces are code-verified; the integrated flow requires a live stack.

---

## Summary

All 8 UI requirements are fully implemented with real code — no stubs, no placeholders. `go build ./... && go vet ./...` pass clean. Every data path flows to live Redis state. The two unverified items are runtime browser state transitions (poll→SSE crossover and the admission redirect chain) that are correct by static analysis but require a running stack to confirm the integrated behavior.

The stale TODO in `admission.go:18` is informational only — the real `token.IssueAdmission` injector is already in place in `queueserver/main.go:37`.

---

_Verified: 2026-08-03_
_Verifier: Claude (gsd-verifier)_
