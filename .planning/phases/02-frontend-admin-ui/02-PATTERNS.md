# Phase 2: Frontend & Admin UI - Pattern Map

**Mapped:** 2026-08-02 (updated with 02-UI-SPEC.md)
**Files analyzed:** 8
**Analogs found:** 7 / 8

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `web/queue/index.html` | component | request-response | DESIGN.md §7 + UI-SPEC | canonical |
| `web/queue/queue.js` | component | event-driven | DESIGN.md §7 queue.js | canonical |
| `web/admin/index.html` | component | request-response | `web/queue/index.html` (sibling) | role-match |
| `web/admin/admin.js` | component | request-response | `web/queue/queue.js` (sibling) | role-match |
| `internal/api/handler_events.go` | handler | CRUD | `internal/api/handler_admin.go` | exact |
| `internal/api/router.go` | route | request-response | `internal/api/router.go` (self) | self-modify |
| `docker-compose.yml` | config | — | `docker-compose.yml` (self) | self-modify |
| `cmd/stuborigin/main.go` | handler | request-response | `cmd/stuborigin/main.go` (self) | self-modify |

> **Note:** UI-SPEC uses `index.html` (not `queue.html` / `admin.html`) as filenames. Use `index.html` for both pages.

---

## Design System (from 02-UI-SPEC.md)

Vanilla HTML/CSS/JS — no build step, no component library, no icon library.

**Colors:**
| Token | Value | Usage |
|-------|-------|-------|
| Background | `#ffffff` | Page, card, form backgrounds |
| Secondary surface | `#f3f4f6` | Top bar (admin), table row alt |
| Accent | `#2563eb` | Submit button, active-event highlight, SSE indicator dot |
| Destructive | `#dc2626` | Session-expired error banner only |
| Warning surface | `#fef3c7` | Constrained banner (queue page) |
| Secondary text | `#6b7280` | Sub-labels, estimates, metadata |
| Border | `#e5e7eb` | Input borders, card borders, table dividers |

**Typography:**
| Role | Size | Weight | Usage |
|------|------|--------|-------|
| Body | 16px | 400 | Default text, status messages, labels |
| Label | 14px | 400 | Secondary info, stat sub-labels |
| Heading | 20px | 600 | Section headings, card titles |
| Display | 28px | 600 | Queue position number, primary stat values |

**Spacing scale** (multiples of 4): xs=4px, sm=8px, md=16px, lg=24px, xl=32px, 2xl=48px, 3xl=64px.

**Font:** `system-ui, -apple-system, sans-serif`

---

## Pattern Assignments

### `web/queue/queue.js` (component, event-driven)

**Analog:** DESIGN.md §7 — canonical implementation, adapt API base URL and add UI-SPEC state/copy.

**Canonical source** (DESIGN.md lines 366–419, with adaptations):
```javascript
// queue.js — identical static file served to every queued user from S3/CDN
const params   = new URLSearchParams(location.search);
const ticketId = params.get('ticket');
const target   = params.get('target');
const SSE_THRESHOLD = 200; // crossover point — matches server-side cfg.SSEThreshold

if (target) sessionStorage.setItem('q_target', target);

let pollTimer = null;
let es = null;

// UI-SPEC: initial loading state before first poll returns
document.getElementById('pos').textContent = 'Checking your position…';

function handleAdmitted(token) {
    if (es) es.close();
    if (pollTimer) clearInterval(pollTimer);
    document.cookie = `q_admission=${token}; path=/; max-age=1800; SameSite=Strict; Secure`;
    window.location.href = sessionStorage.getItem('q_target') || '/';
}

function startSSE() {
    es = new EventSource(`${window.QUEUE_CONFIG.apiBase}/queue/status/${ticketId}?mode=sse`);
    es.addEventListener('update', (e) => {
        const data = JSON.parse(e.data);
        if (data.type === 'position') renderPosition(data.value);
        if (data.type === 'admitted') handleAdmitted(data.token);
        // UI-SPEC: constrained state
        if (data.constrained) showConstrained(true);
        else showConstrained(false);
    });
    // UI-SPEC copy: "Reconnecting…"
    es.onerror = () => { document.getElementById('status').textContent = 'Reconnecting…'; };
}

async function pollOnce() {
    try {
        const res  = await fetch(`${window.QUEUE_CONFIG.apiBase}/queue/status/${ticketId}?mode=poll`);
        const data = await res.json();

        if (data.type === 'admitted') { handleAdmitted(data.token); return; }
        if (data.type === 'pending') return;

        renderPosition(data.value);
        showConstrained(!!data.constrained);

        if (data.upgrade_to_sse && !es) {
            clearInterval(pollTimer);
            startSSE();
        }
    } catch (_) {
        // UI-SPEC copy: poll error state
        document.getElementById('status').textContent = 'Connection lost. Retrying…';
    }
}

function renderPosition(rank) {
    // UI-SPEC copy contract
    document.getElementById('pos').textContent = `${rank} people ahead`;
    const mins = Math.ceil(rank / admitRatePerMin);
    document.getElementById('wait').textContent =
        rank < 1 ? 'Less than a minute' : `~${mins} min`;
}

function showConstrained(on) {
    // UI-SPEC: constrained banner — hidden by default, shown when server returns constrained:true
    document.getElementById('constrained').style.display = on ? 'block' : 'none';
    document.getElementById('wait').style.display = on ? 'none' : '';
}

pollTimer = setInterval(pollOnce, 5000);
pollOnce(); // immediate first read
```

---

### `web/queue/index.html` (component, request-response)

**Analog:** DESIGN.md §7 DOM contract + UI-SPEC layout.

**Required DOM ids** (consumed by queue.js): `pos`, `wait`, `status`, `constrained`.

**Layout** (UI-SPEC): single-column, `<main>` max-width 480px, auto horizontal margins, top padding 64px (3xl). Position display (28px) above wait estimate (16px), 8px gap. Constrained banner: full width, background `#fef3c7`, 16px padding, 8px border-radius.

**Structure pattern:**
```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Virtual Queue</title>
  <!-- D-02: config injected first so queue.js reads it on load -->
  <script>window.QUEUE_CONFIG = { apiBase: 'http://localhost:8080' };</script>
  <link rel="stylesheet" href="queue.css">
  <script src="queue.js" defer></script>
</head>
<body>
  <main>
    <!-- UI-SPEC copy: loading state set by JS on init -->
    <p id="pos" style="font-size:28px;font-weight:600"></p>
    <p id="wait" style="font-size:16px"></p>
    <p id="status" style="font-size:16px"></p>
    <!-- UI-SPEC: constrained banner, hidden by default -->
    <div id="constrained" style="display:none;background:#fef3c7;padding:16px;border-radius:8px">
      Queue paused &mdash; waiting for capacity to free up.
    </div>
  </main>
</body>
</html>
```

---

### `web/admin/admin.js` (component, request-response)

**Analog:** queue.js polling pattern — same `setInterval` + `fetch` structure, 2s interval.

**Full pattern including UI-SPEC state machines:**
```javascript
let pollTimer = null;
let currentEventId = null;
let lastStats = null;

// UI-SPEC: load events on page init; show "No events active" when list is empty
async function loadEvents() {
    const res = await fetch(`${window.QUEUE_CONFIG.apiBase}/queue/events`);
    const { events } = await res.json();
    const sel = document.getElementById('event-select');
    if (!events.length) {
        sel.innerHTML = '<option value="">No events active</option>';
        setStatsBlank();
        return;
    }
    sel.innerHTML = events.map(id => `<option value="${id}">${id}</option>`).join('');
    startPolling(events[0]);
}

function startPolling(eventId) {
    if (pollTimer) clearInterval(pollTimer);
    currentEventId = eventId;
    document.getElementById('update-btn').disabled = false;
    setPollIndicator(true);
    fetchConfig(); // immediate first read
    pollTimer = setInterval(fetchConfig, 2000);
}

async function fetchConfig() {
    try {
        const res = await fetch(`${window.QUEUE_CONFIG.apiBase}/queue/config/${currentEventId}`);
        if (!res.ok) throw new Error('non-200');
        const data = await res.json();
        // data shape: { eventId, queueDepth, activeUsers, admitRate, capacity, estimatedDrainSec }
        lastStats = data;
        renderStats(data, false);
    } catch (_) {
        // UI-SPEC: stats error — retain last value with "(stale)" suffix
        if (lastStats) renderStats(lastStats, true);
    }
}

function renderStats(data, stale) {
    const suffix = stale ? ' (stale)' : '';
    document.getElementById('stat-depth').textContent    = data.queueDepth + suffix;
    document.getElementById('stat-active').textContent   = data.activeUsers + suffix;
    document.getElementById('stat-rate').textContent     = data.admitRate + suffix;
    document.getElementById('stat-capacity').textContent = data.capacity + suffix;
    const headroom = data.capacity - data.activeUsers;
    document.getElementById('stat-headroom').textContent = headroom + suffix;
    // UI-SPEC: accent color on headroom when < 10% of capacity
    document.getElementById('stat-headroom').style.color =
        (data.capacity > 0 && headroom < data.capacity * 0.1) ? '#2563eb' : '';
    const drainMin = data.estimatedDrainSec > 0
        ? Math.ceil(data.estimatedDrainSec / 60) + ' min' : '—';
    document.getElementById('stat-drain').textContent = drainMin + suffix;
}

function setStatsBlank() {
    ['stat-depth','stat-active','stat-rate','stat-capacity','stat-headroom','stat-drain']
        .forEach(id => { document.getElementById(id).textContent = '—'; });
}

// UI-SPEC: poll indicator dot — blue when active, gray when idle
function setPollIndicator(active) {
    document.getElementById('poll-indicator').style.background = active ? '#2563eb' : '#9ca3af';
}

document.getElementById('event-select').addEventListener('change', e => {
    if (e.target.value) startPolling(e.target.value);
});

// UI-SPEC: update success/failure states
document.getElementById('update-form').addEventListener('submit', async e => {
    e.preventDefault();
    const btn = document.getElementById('update-btn');
    try {
        const res = await fetch(`${window.QUEUE_CONFIG.apiBase}/queue/rate/${currentEventId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                rate:     parseInt(document.getElementById('rate-input').value),
                capacity: parseInt(document.getElementById('capacity-input').value),
            }),
        });
        if (!res.ok) throw new Error('non-200');
        // UI-SPEC: "Saved" for 1.5s, then revert
        btn.textContent = 'Saved';
        setTimeout(() => { btn.textContent = 'Update Config'; }, 1500);
        document.getElementById('update-error').textContent = '';
    } catch (_) {
        // UI-SPEC copy: "Update failed. Try again."
        document.getElementById('update-error').textContent = 'Update failed. Try again.';
    }
});

loadEvents();
```

---

### `web/admin/index.html` (component, request-response)

**Required DOM ids** (consumed by admin.js): `event-select`, `poll-indicator`, `stat-depth`, `stat-active`, `stat-rate`, `stat-capacity`, `stat-headroom`, `stat-drain`, `update-form`, `rate-input`, `capacity-input`, `update-btn`, `update-error`.

**Layout** (UI-SPEC): two-region — top bar (full width, `#f3f4f6`, 16px vertical / 24px horizontal padding) + content area. Stat cards: CSS grid 3 columns >= 768px / 2 columns < 768px, 16px gap. Each card: white bg, 1px `#e5e7eb` border, 8px border-radius, 24px padding. Update button: accent fill `#2563eb`, white text, 8px/16px padding, 4px border-radius.

**Structure pattern:**
```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Queue Admin</title>
  <script>window.QUEUE_CONFIG = { apiBase: 'http://localhost:8080' };</script>
  <link rel="stylesheet" href="admin.css">
  <script src="admin.js" defer></script>
</head>
<body>
  <header><!-- top bar -->
    <select id="event-select"></select>
    <span id="poll-indicator" style="display:inline-block;width:10px;height:10px;border-radius:50%;background:#9ca3af"></span>
  </header>
  <main>
    <div class="stat-grid">
      <div class="stat-card"><p class="stat-value" id="stat-depth">&mdash;</p><p class="stat-label">Queue Depth</p></div>
      <div class="stat-card"><p class="stat-value" id="stat-active">&mdash;</p><p class="stat-label">Active Users</p></div>
      <div class="stat-card"><p class="stat-value" id="stat-rate">&mdash;</p><p class="stat-label">Admit Rate</p></div>
      <div class="stat-card"><p class="stat-value" id="stat-capacity">&mdash;</p><p class="stat-label">Capacity</p></div>
      <div class="stat-card"><p class="stat-value" id="stat-headroom">&mdash;</p><p class="stat-label">Headroom</p></div>
      <div class="stat-card"><p class="stat-value" id="stat-drain">&mdash;</p><p class="stat-label">Est. Drain</p></div>
    </div>
    <form id="update-form">
      <label>Rate <input type="number" id="rate-input" min="1"></label>
      <label>Capacity <input type="number" id="capacity-input" min="1"></label>
      <button type="submit" id="update-btn" disabled>Update Config</button>
      <span id="update-error" style="color:#dc2626"></span>
    </form>
  </main>
</body>
</html>
```

---

### `internal/api/handler_events.go` (handler, CRUD)

**Analog:** `internal/api/handler_admin.go` — same `Handler` receiver, same Redis ctx pattern.

**Imports pattern** (handler_admin.go lines 1–9):
```go
package api

import (
    "context"
    "net/http"

    "github.com/gin-gonic/gin"
)
```

**Core handler pattern:**
```go
// GetEvents handles GET /queue/events.
// Scans redis-queue for active queue:* keys and returns eventIds.
// ponytail: SCAN over KEYS — safe on large keyspaces (Claude's Discretion item).
func (h *Handler) GetEvents(c *gin.Context) {
    ctx := context.Background()
    var cursor uint64
    var eventIDs []string
    for {
        keys, next, err := h.rdb.Scan(ctx, cursor, "queue:*", 100).Result()
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "redis scan failed"})
            return
        }
        for _, k := range keys {
            eventIDs = append(eventIDs, k[len("queue:"):]) // strip "queue:" prefix
        }
        cursor = next
        if cursor == 0 {
            break
        }
    }
    if eventIDs == nil {
        eventIDs = []string{} // never return null — admin JS expects array
    }
    c.JSON(http.StatusOK, gin.H{"events": eventIDs})
}
```

---

### `internal/api/router.go` (route — self-modify)

**Analog:** `internal/api/router.go` (self).

**Route to add** (same pattern as existing routes, router.go lines 15–33):
```go
r.GET("/queue/events", h.GetEvents)
```

**CORS middleware to add** (before existing routes, D-03 — localhost only):
```go
r.Use(func(c *gin.Context) {
    origin := c.GetHeader("Origin")
    // ponytail: explicit allowlist; extend slice if static-pages port changes (D-03).
    allowed := []string{"http://localhost:8082", "http://localhost:8081"}
    for _, o := range allowed {
        if origin == o {
            c.Header("Access-Control-Allow-Origin", o)
            break
        }
    }
    c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
    c.Header("Access-Control-Allow-Headers", "Content-Type")
    if c.Request.Method == "OPTIONS" {
        c.AbortWithStatus(http.StatusNoContent)
        return
    }
    c.Next()
})
```

---

### `cmd/stuborigin/main.go` (handler — self-modify)

**Analog:** `cmd/stuborigin/main.go` (self) + `internal/token/session.go`.

**Import to add:**
```go
"github.com/adityabansal29/virtual-queue/internal/token"
```

**Checkout handler pattern** (D-10, D-11, D-12 + UI-SPEC stub checkout states):
```go
r.GET("/", middleware.QueueGuard(mwCfg), func(c *gin.Context) {
    cookie, err := c.Cookie("q_session")
    if err != nil {
        renderSessionExpired(c)
        return
    }
    claims, err := token.ValidateSession(cookie, cfg.SessionSecret)
    if err != nil {
        renderSessionExpired(c)
        return
    }
    // D-11: "Complete Purchase" button fires POST /queue/exit from browser JS
    c.Header("Content-Type", "text/html")
    c.String(http.StatusOK, checkoutPage(claims.EventID, claims.Subject))
})
```

**Session-expired page** (UI-SPEC copy contract — "Your session has expired." / "Return to queue"):
```go
func renderSessionExpired(c *gin.Context) {
    c.Header("Content-Type", "text/html")
    c.String(http.StatusUnauthorized, `<!DOCTYPE html><html lang="en"><body>`+
        `<h1 style="color:#dc2626">Your session has expired.</h1>`+
        `<p>Return to the queue to rejoin.</p>`+
        `<a href="http://localhost:8082/queue/">Return to queue</a>`+
        `</body></html>`)
}
```

**Checkout page** (UI-SPEC: event ID, ticket ID, 3×4 seat grid, Complete Purchase button, success state, exit-failed state):
```go
func checkoutPage(eventID, ticketID string) string {
    // UI-SPEC: seat grid 3 columns, 48×48px seats, accent fill on pre-selected
    // UI-SPEC: Complete Purchase full-width, accent fill; success/error states inline
    // D-11: POST /queue/exit called from JS, then inline success — no redirect
    return `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Checkout</title></head><body>` +
        `<main style="max-width:560px;margin:0 auto;padding-top:48px">` +
        `<p>Event: ` + eventID + `</p>` +
        `<p>Ticket: ` + ticketID + `</p>` +
        `<div class="seat-grid" style="display:grid;grid-template-columns:repeat(3,48px);gap:8px">` +
        seatButtons() +
        `</div>` +
        `<button id="complete-btn" onclick="completePurchase('` + eventID + `')" ` +
        `style="width:100%;padding:16px;background:#2563eb;color:#fff;border:none;border-radius:4px;cursor:pointer;margin-top:16px">` +
        `Complete Purchase</button>` +
        `<div id="success" style="display:none">Thank you &mdash; your ticket is confirmed.</div>` +
        `<div id="exit-error" style="display:none;color:#dc2626">Could not free slot. Please close this tab.</div>` +
        `<script>` +
        `async function completePurchase(eventId) {` +
        `  const btn = document.getElementById('complete-btn');` +
        `  btn.disabled = true;` +
        `  try {` +
        `    const res = await fetch('http://localhost:8080/queue/exit',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({eventId})});` +
        `    if (!res.ok) throw new Error();` +
        `    btn.style.display='none';` +
        `    document.getElementById('success').style.display='block';` +
        `  } catch(_) {` +
        `    btn.disabled=false;` +
        `    document.getElementById('exit-error').style.display='block';` +
        `  }` +
        `}` +
        `</script></main></body></html>`
}

func seatButtons() string {
    // UI-SPEC: 3×4 grid = 12 seats; seat 1 pre-selected (accent fill)
    out := ""
    for i := 1; i <= 12; i++ {
        selected := ""
        if i == 1 {
            selected = `style="background:#2563eb;color:#fff;border:none;border-radius:4px;width:48px;height:48px;cursor:pointer"`
        } else {
            selected = `style="background:#fff;border:1px solid #e5e7eb;border-radius:4px;width:48px;height:48px;cursor:pointer"`
        }
        out += `<button ` + selected + `>Seat ` + fmt.Sprintf("%d", i) + `</button>`
    }
    return out
}
```

**Import to add for `fmt`:**
```go
"fmt"
```

---

### `docker-compose.yml` (config — self-modify)

**Analog:** `docker-compose.yml` (self) — add `static-pages` after existing services.

**New service** (follows stuborigin service pattern, docker-compose.yml lines 35–46):
```yaml
static-pages:
  image: nginx:alpine
  ports:
    - "127.0.0.1:8082:80"
  volumes:
    - ./web:/usr/share/nginx/html:ro
    - ./nginx.conf:/etc/nginx/conf.d/default.conf:ro
```

No `depends_on` — static files only.

---

## Shared Patterns

### QUEUE_CONFIG Injection (D-02)
**Apply to:** `web/queue/index.html`, `web/admin/index.html`

First `<script>` in `<head>`, before any other scripts:
```html
<script>window.QUEUE_CONFIG = { apiBase: 'http://localhost:8080' };</script>
```

### Go Handler Error Response
**Source:** `internal/api/handler_admin.go`, `internal/api/handler_exit.go`
**Apply to:** `internal/api/handler_events.go`
```go
c.JSON(http.StatusInternalServerError, gin.H{"error": "descriptive message"})
return
```

### Redis Context Pattern
**Source:** `internal/api/handler_admin.go` lines 49, 53, 57
**Apply to:** `internal/api/handler_events.go`
```go
ctx := context.Background()
h.rdb.SomeCommand(ctx, ...)
```

### fetch + POST JSON (browser)
**Source:** D-11 + handler_exit.go request shape `{ "eventId": string }`
**Apply to:** stub checkout JS (exit call), `web/admin/admin.js` (rate update)
```javascript
await fetch(`${window.QUEUE_CONFIG.apiBase}/queue/exit`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ eventId }),
});
```

### UI-SPEC Copy Strings (authoritative)
| Element | Copy |
|---------|------|
| Queue page loading | "Checking your position…" |
| SSE error | "Reconnecting…" |
| Poll error | "Connection lost. Retrying…" |
| Constrained banner | "Queue paused — waiting for capacity to free up." |
| Position | "{N} people ahead" |
| Wait normal | "~{N} min" |
| Wait < 1 min | "Less than a minute" |
| Admin no events | "No events active" (dropdown option) |
| Admin stale suffix | " (stale)" appended to value |
| Admin update success | "Saved" (1.5s, then revert to "Update Config") |
| Admin update failure | "Update failed. Try again." |
| Checkout success | "Thank you — your ticket is confirmed." |
| Checkout exit failed | "Could not free slot. Please close this tab." |
| Session expired heading | "Your session has expired." |
| Session expired body | "Return to the queue to rejoin." |
| Session expired link text | "Return to queue" |

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `nginx.conf` | config | — | No existing nginx config; use standard `try_files` pattern |

**nginx.conf pattern** (Claude's discretion — standard static serving):
```nginx
server {
    listen 80;
    location / {
        root /usr/share/nginx/html;
        try_files $uri $uri/ =404;
    }
}
```

---

## Backstop Items (from UI-SPEC)

Two items marked for visual verification — planner should note these as manual check tasks:
1. Admin stat cards at extreme values (e.g. queue depth = 500,000) must not overflow card boundary.
2. Checkout page with eventId/ticketId > 20 chars must not break layout.

Both use CSS `overflow: hidden; text-overflow: ellipsis` on the container as a safe default.

---

## Metadata

**Analog search scope:** `internal/api/`, `internal/token/`, `cmd/stuborigin/`, `docker-compose.yml`, `DESIGN.md §7`, `02-UI-SPEC.md`
**Files read:** 11
**Pattern extraction date:** 2026-08-02
