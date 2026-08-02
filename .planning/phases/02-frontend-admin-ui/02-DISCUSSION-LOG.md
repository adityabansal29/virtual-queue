# Phase 2: Frontend & Admin UI - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-02
**Phase:** 2-Frontend & Admin UI
**Areas discussed:** Static file serving, Admin dashboard approach, Stub checkout scope

---

## Static File Serving

| Option | Description | Selected |
|--------|-------------|----------|
| nginx container | New `static-pages` Docker Compose service serving `web/` dir. Closest to S3/CDN prod model. Requires CORS on queue API. | ✓ |
| Embedded in queue server | Queue server serves static files via Go embed.FS. No CORS. Conflates concerns. | |
| Open file:// directly | No new service. CORS headaches with fetch(). | |

**User's choice:** nginx container
**Notes:** —

---

| Option | Description | Selected |
|--------|-------------|----------|
| window.QUEUE_CONFIG injected in HTML | Inline `<script>` sets API base URL. queue.js reads it. No build step. | ✓ |
| Env var via nginx sub_filter | nginx substitutes placeholder at serve time. More ops complexity. | |
| Same URL everywhere | Hardcode localhost:8080. Simple but not prod-portable. | |

**User's choice:** window.QUEUE_CONFIG injected in HTML
**Notes:** —

---

| Option | Description | Selected |
|--------|-------------|----------|
| Allow localhost origins only | Access-Control-Allow-Origin for http://localhost:8082. | ✓ |
| Allow * (wildcard) | Simplest. Fine for local dev system. | |
| You decide | Standard Go practice. | |

**User's choice:** Allow localhost origins only
**Notes:** —

---

## Admin Dashboard Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Vanilla HTML + CSS + JS | No build step. No npm. Single HTML file. | ✓ |
| Vanilla JS + CDN CSS framework | Same no-build but pulls Bulma/Tailwind via CDN. | |
| React/Vue SPA | Full component framework. Build pipeline. Overkill for ops dashboard. | |

**User's choice:** Vanilla HTML + CSS + JS
**Notes:** —

---

| Option | Description | Selected |
|--------|-------------|----------|
| Poll GET /queue/config every 2s | setInterval on fetch. No new endpoint. Consistent with queue page. | ✓ |
| SSE from new /queue/admin/stream endpoint | Push updates. More real-time. New endpoint + more Go work. | |

**User's choice:** Poll every 2s
**Notes:** —

---

| Option | Description | Selected |
|--------|-------------|----------|
| No auth | Carries forward Phase 1 ponytail deferral. Local dev only. | ✓ |
| HTTP Basic auth via nginx | htpasswd. No Go changes. Adds login prompt. | |

**User's choice:** No auth
**Notes:** —

---

| Option | Description | Selected |
|--------|-------------|----------|
| Rate + capacity inputs only | Two inputs + Submit. Matches existing API. Minimal. | |
| Rate + capacity + per-event selector | Text input for eventId. Scoped stats and controls. | |
| Rate + capacity + manual admit button | Force admit N users. New endpoint needed. | |

**User's choice:** Freeform — "I want it to be event specific. admin selects event, even page will show all config and there you can do the required update. initially, there can be default config if needed."
**Notes:** User wants event-specific dashboard, not a global view.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Text input (type eventId) | Admin types or pastes eventId. No API changes. | |
| Auto-discover from Redis | New GET /queue/events endpoint scans active queue: keys. | ✓ |

**User's choice:** Auto-discover from Redis
**Notes:** Requires new GET /queue/events endpoint in queue server router.

---

## Stub Checkout Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Validation + ticket info + Done button | Show event/ticket details. "Complete Purchase" calls POST /queue/exit. Error page for invalid session. | ✓ |
| Fake seat grid | Interactive seat selection UI. Extra JS, no functional benefit. | |
| Bare minimum | Just validate cookie, show static message. No interactivity. | |

**User's choice:** Validation + ticket info + Done button
**Notes:** —

---

| Option | Description | Selected |
|--------|-------------|----------|
| Decode q_session JWT claims | ValidateSession() returns EventID + Subject (ticketId). Zero extra storage. | ✓ |
| Lookup ticket in Redis | ticket:{jti} in redis-origin only has admission_token — nothing richer. | |

**User's choice:** Decode q_session JWT claims
**Notes:** User asked for clarification on where eventId/ticketId come from. Confirmed: middleware decodes from q_admission JWT, re-embeds in q_session. Checkout handler calls ValidateSession() directly.

**Mid-discussion clarification:** User asked whether `active:{eventId}` INCR had been coded. Confirmed: scheduler increments on redis-queue (admission.go line 103), QueueGuard increments on redis-origin (separate counter, different purpose). Admin dashboard reads redis-queue counter — no bug. Two parallel counters are intentional by design.

---

## Claude's Discretion

- nginx config details (port assignment, MIME types, try_files behavior)
- `web/` directory structure within the repo
- CSS styling for queue page and admin dashboard
- Error page visual design
- Whether `GET /queue/events` uses `KEYS` or `SCAN` (should use SCAN)

## Deferred Ideas

- Capacity ceiling enforcement in scheduler — Phase 1 D-06 still deferred. Counter is now accurate; could be wired up in Phase 2 wave 1 or Phase 3. Planner to decide.
- Admin dashboard auth — Phase 3 / prod hardening.
- S3/CloudFront deployment — Phase 3 (INFRA-04).
