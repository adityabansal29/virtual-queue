---
phase: 02-frontend-admin-ui
plan: "03"
subsystem: scheduler, api, stuborigin
status: complete
tags: [capacity-enforcement, constrained-flag, stub-checkout, session-validation, xss-guard]

dependency_graph:
  requires:
    - 02-01  # queue page and constrained banner (client-side consumer of constrained field)
  provides:
    - capacity enforcement in scheduler (D-06 resolved)
    - constrained:bool in GET /queue/status/:id?mode=poll position response
    - stub checkout: session validation, seat grid, Complete Purchase with POST /queue/exit
  affects:
    - internal/scheduler/admission.go
    - internal/api/handler_status.go
    - cmd/stuborigin/main.go

tech_stack:
  added: []
  patterns:
    - "Go builtin min() for min(rate, headroom) — no helper needed (Go 1.21+)"
    - "fmt.Sprintf(\"%q\") for XSS-safe eventId injection into inline JS (T-02-10)"
    - "token.ValidateSession called directly in handler — no middleware change (D-10)"

key_files:
  modified:
    - internal/scheduler/admission.go
    - internal/api/handler_status.go
    - cmd/stuborigin/main.go

decisions:
  - "Used Go builtin min() rather than writing a helper — available since Go 1.21, codebase is on 1.25"
  - "fmt.Sprintf(\"%q\", eventID) for eventId JS injection: Go's %q produces a double-quoted, Go-escaped string that is valid JSON — prevents XSS from crafted eventId values (T-02-10)"
  - "errorPage() and checkoutPage() as package-level functions (not methods) — no state needed, consistent with seatButtons() helper"
  - "PATTERNS.md checkout snippet used onclick attribute injection (vulnerable); plan action mandated __eventId variable via %q — plan action took precedence (security correctness over pattern proximity)"

metrics:
  duration_minutes: 17
  completed_date: "2026-08-03"
  tasks_completed: 2
  tasks_total: 2
  commits: 2

estimate:
  tokens: 50000

actuals:
  tokens: 9000
  tasks: 2
  commits: 2
---

# Phase 02 Plan 03: Capacity Enforcement, Constrained Flag, and Stub Checkout Summary

Wired the Phase 1 deferred D-06 capacity enforcement into the scheduler tick, added the `constrained` boolean to the poll status handler, and fully implemented the stub checkout with session validation, seat grid, and inline Complete Purchase flow.

## What Was Built

### Task 1: Scheduler capacity enforcement and constrained flag (c065bcd)

**internal/scheduler/admission.go — tick():**
- Added `strconv` import
- Replaced the D-06 deferred comment (`ponytail: add min(rate, headroom) here`) with the actual enforcement: reads `active:{eventId}` and `capacity:{eventId}` from Redis, computes headroom, returns immediately (no ZPOPMIN) when headroom <= 0 (T-02-12), otherwise admits `min(rate, headroom)` tickets
- When `capacity == 0` (unconfigured), falls back to full `rate` — preserves the existing unlimited behavior

**internal/api/handler_status.go — QueueStatusPoll():**
- Added `strconv` import
- After computing rank, reads `capacity:{eventId}` and `active:{eventId}` from Redis
- Sets `constrained = capacity > 0 && (capacity-active) <= 0`
- Added `"constrained": constrained` to the position response `gin.H` map
- Field only appears on `type=position` responses — not on admitted or pending

### Task 2: Stub checkout with session validation and seat grid (bb073e4)

**cmd/stuborigin/main.go:**
- Added imports: `"fmt"`, `"github.com/adityabansal29/virtual-queue/internal/token"`
- Replaced the static `<h1>Seat Selection</h1>` handler body with full checkout logic:
  - `c.Cookie("q_session")` — missing cookie → render `errorPage()`, 401
  - `token.ValidateSession(cookie, cfg.SessionSecret)` — invalid/expired → render `errorPage()`, 401
  - Valid session → render `checkoutPage(claims.EventID, claims.Subject)`, 200
- `errorPage()`: "Your session has expired." heading, "Return to the queue to rejoin.", "Return to queue" link to `localhost:8082/queue/`
- `checkoutPage(eventID, ticketID string)`: event/ticket IDs, 3x4 seat grid (12 buttons), Seat 1 pre-selected, Complete Purchase button, hidden success/error divs, inline JS
- `seatButtons()`: generates 12 `<button class="seat [selected]">Seat N</button>` elements
- Inline JS: seat click handler toggles `.selected`; complete-btn disabled immediately on click (prevents double POST); `fetch('http://localhost:8080/queue/exit', ...)` on 200 hides button and shows `#success`; on non-200 or throw re-enables button and shows `#exit-error`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Security] PATTERNS.md checkout snippet used XSS-vulnerable onclick injection**
- **Found during:** Task 2
- **Issue:** The PATTERNS.md checkout pattern used `onclick="completePurchase('` + eventID + `')"` which would allow XSS if eventID contains quotes or JS. The plan *action* (not patterns) mandated `fmt.Sprintf("%q", eventID)` for safe injection.
- **Fix:** Used `var __eventId = %s;` with `fmt.Sprintf("%q", eventID)` — produces a JSON-compatible double-quoted string. The inline script accesses `__eventId` variable, never interpolates eventID directly into script syntax.
- **Files modified:** cmd/stuborigin/main.go
- **Commit:** bb073e4

No other deviations — plan executed as written.

## Known Stubs

None. All acceptance criteria are wired:
- Scheduler enforces real capacity ceiling
- Poll handler emits real constrained flag from Redis state
- Stub checkout calls real `token.ValidateSession()` and real `POST /queue/exit`

## Threat Surface

No new threat surface beyond what the plan's `<threat_model>` already covers. T-02-09 (session validation bypass) mitigated by `ValidateSession`. T-02-10 (eventId XSS) mitigated by `%q` injection. T-02-12 (scheduler tight-loop at zero headroom) mitigated by early return in tick().

## Self-Check: PASSED

All files present. Both task commits verified in git log.
