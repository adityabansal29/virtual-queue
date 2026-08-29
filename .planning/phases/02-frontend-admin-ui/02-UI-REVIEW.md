# Phase 02 — UI Review

**Audited:** 2026-08-29  
**Baseline:** `02-UI-SPEC.md` (approved contract; verified 2026-08-02)  
**Screenshots:** not captured (no dev server on ports 3000, 5173, or 8080)

## Pillar Scores

| Pillar | Score | Key Finding |
|---|---:|---|
| 1. Copywriting | 2/4 | Most queue/admin strings match, but the required session-expired checkout copy is not implemented. |
| 2. Visuals | 2/4 | Queue focal content exists, but the queue is not vertically centered and the checkout has no visual heading/hierarchy. |
| 3. Color | 3/4 | Declared colors are used consistently; repeated hardcoded tokens make drift likely and the visual split was not verifiable without screenshots. |
| 4. Typography | 3/4 | Core 14/16/28px sizes and 400/600 weights match, but checkout/error typography lacks the contract’s complete line-height treatment. |
| 5. Spacing | 2/4 | Several token values match, but the 768px breakpoint is wrong and queue layout does not implement the specified vertical centering. |
| 6. Experience Design | 2/4 | Loading, polling, SSE, stale, and purchase states exist, but session expiry and constrained SSE behavior fail the state contract. |

**Overall: 14/24**

## Top 3 Priority Fixes

1. **Implement the contract checkout session-expired path** — users without a valid session receive a generic “You need a queue ticket…” page instead of the required recovery copy — add the `errorPage()` response and validate/render the session flow exactly as specified in `cmd/stuborigin/main.go` / middleware.
2. **Fix responsive/layout contract violations** — the queue is not vertically centered and 768px incorrectly switches to two columns — use a centered minimum-height layout and change the admin media query to apply only below 768px.
3. **Preserve constrained state through SSE and selection changes** — SSE position events omit `constrained`, causing the client to hide the banner; selecting the empty event option leaves polling active — include constrained state in SSE or retain the last constrained value, and stop polling/disable updates when no event is selected.

## Detailed Findings

### Pillar 1: Copywriting (2/4)

- **WARNING — `cmd/stuborigin/main.go:39-48`:** The checkout route delegates to `QueueGuard` and only renders `checkoutPage`; there is no `errorPage()` and no implementation of the required “Your session has expired.” / “Return to the queue to rejoin.” / “Return to queue” contract. `pkg/middleware/queue_guard.go:78-93` instead renders “You need a queue ticket to access this page.” with “Join the queue”.
- **WARNING — `web/admin/index.html:14-16`:** “No events active” is correctly present, but it remains an available placeholder even when events are populated; the user can select it and enter an undefined operational state.
- **PASS evidence:** Queue loading, constrained, reconnect, connection-lost, and admin update strings in `web/queue/index.html:14-17`, `web/queue/queue.js:37,66`, and `web/admin/admin.js:113-118` match the contract. Checkout success/error strings exist at `cmd/stuborigin/main.go:82-83`.

### Pillar 2: Visuals (2/4)

- **WARNING — `web/queue/queue.css:10-14`:** `main` has a top padding but no viewport-height/flex centering, so the queue waiting content is top-offset rather than vertically centered as required.
- **WARNING — `cmd/stuborigin/main.go:59-84`:** Checkout contains only event/ticket paragraphs, the seat grid, and a button; there is no “Seat Selection” heading or equivalent page-level focal hierarchy.
- **WARNING — `web/admin/index.html:13-18`:** The admin top bar has an unlabeled select and a bare status dot. The dot has no `aria-label`, `title`, or adjacent text explaining idle vs polling, making the connection state visually ambiguous.
- **WARNING — `web/admin/index.html:20-52`:** The dashboard has no page title or section heading, reducing orientation for an operator landing on the admin page.

### Pillar 3: Color (3/4)

- **PASS evidence:** `web/admin/admin.css:12,31-34,47,52,80-81,98,104,108` and `web/queue/queue.css:6,27,33,39` use the contract’s dominant, secondary, accent, muted, border, warning, and destructive values. Checkout repeats the same contract values at `cmd/stuborigin/main.go:69-74`.
- **WARNING — hardcoded token duplication:** Colors are repeated across two CSS files and inline Go HTML rather than shared custom properties. This is acceptable for the no-build stack but makes future token drift likely.
- **WARNING — no visual capture:** The required 60/30/10 surface distribution and actual contrast could not be verified because no server was available for screenshots.

### Pillar 4: Typography (3/4)

- **PASS evidence:** `web/queue/queue.css:5,17-19,25-28,32-34` and `web/admin/admin.css:6,38-40,51-54` use the declared body/label/display sizes and 400/600 weights.
- **WARNING — `cmd/stuborigin/main.go:66-74`:** Checkout sets body/seat/error font sizes but does not set the contract line heights, and the body has no explicit 16px base size. Browser defaults therefore determine some text metrics.
- **WARNING — missing heading role:** The checkout and admin surfaces do not use the declared 20px/600 page/section heading role, contributing to weak hierarchy.

### Pillar 5: Spacing (2/4)

- **BLOCKER — `web/admin/admin.css:114-117`:** `@media (max-width: 768px)` applies the two-column grid at exactly 768px, while the contract requires three columns on `>= 768px` and two only below it. Use `@media (max-width: 767px)`.
- **WARNING — `web/queue/queue.css:10-14`:** The queue’s `padding: 64px 16px 16px` matches token values but cannot satisfy the specified vertical centering. The `main` also lacks `box-sizing: border-box`, so its 480px max-width plus horizontal padding yields a 512px outer width.
- **PASS evidence:** Admin top bar/content/card/grid spacing and checkout seat grid values match the declared 4px scale at `web/admin/admin.css:13,20,26,27,34,60-67` and `cmd/stuborigin/main.go:66-74`.

### Pillar 6: Experience Design (2/4)

- **BLOCKER — `cmd/stuborigin/main.go:39-48`:** Missing/invalid `q_session` does not render the required dedicated session-expired recovery state. The middleware’s generic queue-ticket page is a different flow and copy, so the contract’s checkout recovery task is not met.
- **WARNING — `internal/api/handler_status.go:88-113` + `web/queue/queue.js:29-34,79-89`:** SSE position events omit `constrained`; the client interprets absent as false and hides the constrained banner. A constrained user can therefore lose the pause message after an SSE update.
- **WARNING — `web/admin/admin.js:92-94`:** Choosing the empty option does nothing. The previous polling timer remains active and the update button stays enabled, so the UI can continue fetching/updating an event that is no longer selected.
- **WARNING — `web/admin/admin.js:6-29`:** Initial event discovery failure only blanks stats; there is no user-visible error or retry affordance. This is weaker than the explicit error handling provided for later stats/update failures.
- **PASS evidence:** Queue loading/poll/SSE error handling exists in `web/queue/queue.js:15-17,35-67`; admin stale values and update failure are implemented in `web/admin/admin.js:47-50,113-120`; checkout double-submit prevention and exit failure recovery exist in `cmd/stuborigin/main.go:94-108`.
- **Verification note:** `go test ./...` reached package compilation but failed overall because the sandbox denied access to the user Go build cache; this is an environment limitation, not counted as a UI pass.

## Files Audited

- `.planning/phases/02-frontend-admin-ui/02-UI-SPEC.md`
- `.planning/phases/02-frontend-admin-ui/02-CONTEXT.md`
- `.planning/phases/02-frontend-admin-ui/02-01-PLAN.md`, `02-01-SUMMARY.md`
- `.planning/phases/02-frontend-admin-ui/02-02-PLAN.md`, `02-02-SUMMARY.md`
- `.planning/phases/02-frontend-admin-ui/02-03-PLAN.md`, `02-03-SUMMARY.md`
- `web/queue/index.html`, `web/queue/queue.css`, `web/queue/queue.js`
- `web/admin/index.html`, `web/admin/admin.css`, `web/admin/admin.js`
- `cmd/stuborigin/main.go`
- `pkg/middleware/queue_guard.go`
- `internal/api/handler_status.go`, `internal/api/router.go`
