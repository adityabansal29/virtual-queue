# In-House Virtual Queue System — HLD v3

*System Design Document*

*High-Level Design*

Architecture · Akamai EdgeWorkers · Go SSE/Polling Hybrid · Two-Cookie Token Model · Build-vs-Buy

Version **3.0** | Stack: **Go + AWS** | Edge: **Akamai EdgeWorkers** | Status: **UPDATED — INTERNAL**

**V3 — Changes from Discussion**

- Split edge secret into **two independent keys** — `ADMISSION_SECRET` and `SESSION_SECRET` — matching the two distinct secrets already used at origin. v2's EdgeWorker snippet incorrectly reused one `QUEUE_SECRET` for both token types.
- Added **active-user / capacity accounting**: new `active:{eventId}` and `capacity:{eventId}` Redis keys. Scheduler now admits `min(configured_rate, capacity - active)` per tick, not rate alone. `QueueGuard` increments `active` on admission; `/queue/exit` and session-TTL sweep decrement it.
- Added **hybrid SSE/polling transport**: SSE reserved for the last ~200 positions (or last ~2 minutes of estimated wait); everyone further back polls every 5s. Reduces persistent-connection count by orders of magnitude at 500k+ scale without materially hurting perceived UX.
- Clarified capacity-blocked vs rate-blocked wait-time estimation — frontend must distinguish these, since a ceiling-blocked queue does not move even though rate looks unchanged.

## Table of Contents

- [Overview](#overview)
- [Queue-Fair](#queuefair)
- [Core Flow](#flow)
- [Token Model](#tokens)
- [Architecture](#hld)
- [Components](#components)
- [Capacity & Rate](#capacity)
- [Transport: SSE + Polling](#sse)
- [Akamai Edge](#edge)
- [Data Model](#datamodel)
- [Go Design](#godesign)
- [AWS Infra](#infra)
- [NFRs](#nfr)
- [Cost](#cost)
- [Verdict](#verdict)

> Section 01

## Executive Overview

This document presents the High-Level Design for an **in-house Virtual Queue / Waiting Room system** — a direct alternative to Queue-Fair. The system controls user access to a protected resource (limited-seat event, flash sale, card application) by holding excess traffic in a virtual queue and admitting users in FIFO order at a configurable rate matching backend capacity.

**Core Principle:** Queued users generate zero load on the protected origin. The queue waiting page is a static CDN-served shell. Dynamic position data flows via a hybrid SSE/polling transport from a separate purpose-built queue API. Only admitted users — bounded by both configured rate and remaining concurrent capacity — ever reach the business origin.

Stack: **Go** (queue server), **ElastiCache Redis** (sorted set queue + pub/sub), **ECS Fargate** (containerized), **S3 + Akamai CDN** (static queue page), **Akamai EdgeWorkers** (edge token check), **HTTP SSE + polling hybrid** (position streaming), **HMAC-signed JWTs** (admission tokens).

> Section 02

## Understanding Queue-Fair (Reference Product)

Queue-Fair (patented 2004) intercepts excess web traffic, holds users on a CDN-hosted branded waiting page, and redirects them back in FIFO order at the operator-configured rate. It is the reference product this in-house design replicates.

- 🚦 **Rate-Based Admission**: Operator sets visitors/min outflow. Queue activates automatically when inflow exceeds SafeGuard threshold.
- 🔐 **Signed Tokens**: HMAC-signed cookies prevent queue bypass. Every admitted user carries a cryptographically valid pass.
- 🌍 **External CDN Queue Page**: Queue page served from Queue-Fair's Google CDN. Origin sees zero load from queued users.
- 📊 **Real-time Portal**: Live dashboard: queue depth, admit rate, estimated wait, session health.
- 🤖 **Bot Controls**: CAPTCHA, IP filters, one-order-per-visitor, pre-sale randomiser (lottery mode).
- ⚡ **Zero-Code Integration**: One JS snippet or server-side adapter. Go adapter available.

| Tier | Outflow | Cost |
| --- | --- | --- |
| Free | ≤10 visitors/min, 1 queue | FREE |
| Unlimited | Unlimited | Custom — est. £1,000–£3,000/mo |

**⚡ Tactical:** Queue-Fair publicly commits to "25% off any competitor quote." Get a quote from queue-it.com or Cloudflare Waiting Room first, then use it as leverage before signing an annual contract.

> Section 03

## Core System Flow

1. **Intercept — User Arrives at app.yourdomain.com**

   Akamai EdgeWorker fires on every request. Checks for `q_session` cookie (ongoing session) or `q_admission` cookie (first-time JWT). Both absent → 302 redirect to `queue.yourdomain.com/join?target=<encoded_url>`.
2. **Enqueue — Assign Position**

   Queue join handler issues ticketId, stores in Redis Sorted Set with score = join timestamp ms (ZADD), redirects browser to static queue waiting page: `queue.yourdomain.com/wait?ticket=<id>&target=<url>`.
3. **Hold — Queue Waiting Page (Static Shell + Hybrid Transport)**

   HTML/JS shell served from S3 via Akamai CDN — **same static files for every user**, zero per-request rendering. Client JS estimates distance from the front (via initial ZRANK) and chooses **polling** (far back) or **SSE** (near front) — see Section 07. Protected origin receives zero load from queued users either way.
4. **Admit — Scheduler Pops + Delivers Token**

   Admission Scheduler ticks every second. Computes `headroom = capacity - active`, admits `min(configured_rate, headroom)` tickets via ZPOPMIN from Redis Sorted Set front. For each: issues HMAC-signed JWT, stores it on `ticket:{ticketId}` as `admission_token`, publishes `{"type":"admitted","token":"<jwt>"}` to `ticket:updates:{ticketId}` Redis pub/sub channel (delivered via SSE if connected, or picked up on next poll otherwise).
5. **Redirect — Browser Sets Cookie and Navigates**

   Queue page JS receives admitted event (via SSE push or poll response). Sets `q_admission=<jwt>` as cookie. Reads original target URL from sessionStorage (saved at step 3). Closes any open SSE connection. Does `window.location.href = targetUrl`.
6. **Edge Check — Akamai Verifies and Forwards**

   Request hits Akamai EdgeWorker with `q_admission` cookie. EdgeWorker verifies HMAC signature inline using `ADMISSION_SECRET` (SubtleCrypto, ~1ms, no Redis call). Valid → forwards to origin. EdgeWorker does NOT do SETNX or capacity accounting — no VPC/Redis access at edge.
7. **Origin — SETNX One-Time Check + Active Increment + Session Upgrade**

   Origin QueueGuard middleware validates JWT, extracts JTI. Calls `SETNX token:{jti} "used" EX 1800`. SETNX fails → token already used (sharing attempt) → 403. SETNX succeeds → increments `active:{eventId}`, issues `q_session` signed cookie (using `SESSION_SECRET`), clears `q_admission` cookie, passes request to business handler.
8. **Subsequent Requests — Session Cookie Only**

   All further requests in this browser session carry `q_session` only. EdgeWorker verifies its HMAC signature inline using `SESSION_SECRET` — no Redis call. Expired session → clean 302 to queue with reason=expired. No more JWT handling in normal flow.
9. **Reclaim — Decrement Active Count, Proactive or TTL**

   Origin calls `POST /queue/exit` on checkout complete or session end — decrements `active:{eventId}`, immediately freeing headroom for the next scheduler tick. Otherwise the active count is reclaimed by a TTL-driven sweep when the session slot expires (30-minute TTL).

> Section 04

## Two-Cookie Token Model

Two distinct cookies serve different purposes — and, critically, are **signed and verified with two independent secrets** end to end (join, edge, origin). Understanding the distinction explains the edge routing logic, the SETNX placement, and why key separation matters.

**Correction from v2:** The v2 EdgeWorker code sample verified both `q_admission` and `q_session` with a single shared `QUEUE_SECRET`, while the origin code already used two separate secrets (`cfg.QueueSecret` for the JWT, `cfg.SessionSecret` for the session cookie). This was an inconsistency, not an intentional simplification. v3 corrects the EdgeWorker to hold two independent secrets — `ADMISSION_SECRET` and `SESSION_SECRET` — mirroring origin exactly. See Section 08 for the corrected implementation and the reasoning for key separation.

#### q_admission — One-Time JWT

- Issued by Admission Scheduler
- Signed with `ADMISSION_SECRET`
- Delivered to browser via SSE or poll response
- Set as cookie by queue page JS on admission
- Claims: JTI (unique per token), eventId, ticketId, iat, exp (30min)
- HMAC-signed — EdgeWorker verifies without Redis
- One-time use: SETNX at origin on first request consumes it
- Purpose: carry admission proof from queue page to origin, once
- Prevents: sharing the admission redirect URL with another person

#### q_session — Ongoing Signed Cookie

- Issued by origin QueueGuard middleware after successful SETNX
- Signed with `SESSION_SECRET` — a different key from the admission token
- Simpler signed payload (not full JWT): sessionId, eventId, iat, exp
- HMAC-signed — EdgeWorker verifies inline
- No Redis call at edge for session verification
- Expired session detected at edge → clean redirect to queue
- Purpose: allow all subsequent page requests within session without re-queuing
- Prevents: user being sent back to queue on every navigation

**⚡ Why Two Secrets, Not One:** The admission token and session cookie protect different things and have different blast radii if compromised. `ADMISSION_SECRET` only needs to be valid for the ~30 minutes between admission and the one-time SETNX at origin — if it ever leaked, the damage window is small and self-limiting (SETNX still caps each JTI to one use). `SESSION_SECRET` signs cookies that stay valid for the entire session and gate every subsequent request — a leak here is a much bigger problem. Keeping them independent means rotating or revoking one does not touch the other, and a forged admission token (even if someone obtained `ADMISSION_SECRET`) still cannot forge a valid ongoing session, and vice versa.

### EdgeWorker Routing Table — Every Request to app.yourdomain.com

| Request Type | Cookie State | EdgeWorker Action | Origin Hit? |
| --- | --- | --- | --- |
| Static assets / health checks | Any | Skip (path rule) | Yes or Akamai cache |
| queue-api.yourdomain.com | Any | Skip (different subdomain) | Yes — Queue API |
| queue.yourdomain.com | Any | Skip (different subdomain) | No — S3/CDN |
| First visit, no tokens | None | 302 → queue join | No |
| Post-admission redirect | q_admission (valid HMAC via ADMISSION_SECRET) | Forward to origin | Yes — SETNX + active++ happens here |
| Subsequent in-session requests | q_session (valid HMAC via SESSION_SECRET, not expired) | Forward to origin | Yes |
| Expired session | q_session (expired) | 302 → queue, reason=expired | No |
| Tampered/forged cookie | Invalid HMAC (either secret) | 302 → queue join | No |
| Shared admission token (2nd use) | q_admission (valid sig) | Forward to origin | Yes — SETNX fails → 403 |

**⚡ Why SETNX at Origin, Not Edge:** Akamai EdgeWorkers run in Akamai's global edge runtime — they have no VPC access and cannot reach ElastiCache Redis. Exposing Redis publicly is a security anti-pattern. The HMAC signature check at edge gives cryptographic validity (no Redis needed). The SETNX + active-count increment at origin gives one-time-use enforcement and capacity accounting (needs Redis). Both layers are necessary; each lives in the right place.

> Section 05

## High-Level Architecture

*(Architecture diagram unchanged from v2 — Akamai edge, Go queue service on ECS Fargate, Redis + DynamoDB data layer, protected origin. See Section 08 for the corrected EdgeWorker secret handling and Section 09 for the new capacity keys.)*

**⚡ Subdomain Separation is Mandatory:** Host Queue API on `queue-api.yourdomain.com` and the static waiting page on `queue.yourdomain.com`. Apply EdgeWorker queue protection only to `app.yourdomain.com`. This prevents circular dependency — users in the queue must reach the Queue API and queue page without triggering queue protection themselves.

> Section 06

## Component Breakdown

### Queue API Endpoints

| Endpoint | Caller | Description | Latency Target |
| --- | --- | --- | --- |
| `POST /queue/join` | Browser (redirected by edge) | ZADD to sorted set. Issues ticketId. Idempotent on sessionId. Redirects to static waiting page. | <15ms |
| `GET /queue/status/:ticketId` | Browser JS — SSE (near front) or single-shot poll (far back) | SSE: opens stream, sends initial ZRANK, subscribes to pub/sub. Poll: single ZRANK read, returns immediately, connection closes. | SSE initial <20ms then streaming; poll <20ms per call |
| `POST /queue/exit` | Origin business service | Proactively frees slot on checkout complete or session end. Decrements `active:{eventId}`. | <10ms |
| `PUT /queue/rate/:eventId` | Admin dashboard | Update admit rate and/or capacity in Redis. Scheduler picks up on next tick. | <10ms |
| `GET /queue/config/:eventId` | Admin / internal | Queue depth, admit rate, capacity, active count, estimated drain time. | <5ms |

### Position Display — ZRANK Is the Only Counter Needed

**Correction from v1:** The v1 design introduced a separate admitted counter (INCRBY admitted:{eventId}) for computing people-ahead. This is unnecessary. When the Admission Scheduler calls ZPOPMIN N, those N entries are removed from the sorted set. Every remaining member's ZRANK automatically decreases. ZRANK(ticketId) directly equals people-ahead-of-you with no further arithmetic. The admitted counter was removed from the hot path entirely.

```lua
-- Lua script: atomic position read
-- Returns -1 if ticket not in set (already admitted or expired)
-- Returns 0 if next in line, N if N people ahead
local rank = redis.call('ZRANK', KEYS[1], ARGV[1])
if rank == false then return -1 end
return rank
```

`rank == false` (key not found) means the scheduler already popped this ticket. Both the SSE handler and the poll handler use this as a secondary confirmation that admission is imminent — the primary trigger is the pub/sub admitted event (SSE) or the poll response directly carrying the token (polling).

### INCRBY vs HINCRBY

Use `INCRBY` on a plain string key for any standalone counter (e.g., `INCRBY active:{eventId} 1` on admission, `INCRBY active:{eventId} -1` on exit). `HINCRBY` increments a field inside a Redis hash — only use it if consolidating multiple stats for one event into a single key (`HINCRBY stats:{eventId} admitted 50`). For the simple case, `INCRBY` on a dedicated key is cleaner and slightly faster, and — as of v3 — `active:{eventId}` is no longer just a dashboard vanity metric; the scheduler reads it every tick to compute headroom (Section 09).

> Section 06.5

## Active-User Accounting & Admission Rate

**New in v3:** v1/v2 treated `rate:{eventId}` as the only admission control — the scheduler simply admitted N per tick regardless of how many previously-admitted users were still active. That's correct only when the protected origin's constraint is *inflow speed*. Many use cases (fixed inventory flash sales, checkout systems with a hard concurrency ceiling) also need a *concurrency ceiling* — that requires knowing how many admitted users are still active.

### The two constraints, and why they don't collide

- **Admission rate** (`rate:{eventId}`) — how fast new users are let in per minute. A flow-control valve, protecting against burst load on newly-admitted requests.
- **Capacity ceiling** (`capacity:{eventId}`, tracked against `active:{eventId}`) — how many admitted users may be inside *at the same time*, total. Protects against sustained overload from users who haven't left yet.

They gate the scheduler at two independent checkpoints in the same tick — whichever is more restrictive wins:

```go
func (s *Scheduler) tick(ctx context.Context, eventID string) {
    active, _   := s.redis.Get(ctx, "active:"+eventID).Int64()
    capacity, _ := s.redis.Get(ctx, "capacity:"+eventID).Int64()
    rate, _     := s.redis.Get(ctx, "rate:"+eventID).Int64()

    headroom := capacity - active
    if headroom <= 0 {
        return // ceiling reached — admit nobody this tick, don't publish a tick event
    }

    n := min(rate, headroom) // effective_rate = min(configured_rate, capacity - active)
    if n <= 0 { return }
    s.admitBatch(ctx, eventID, n)
}
```

Effective throughput therefore has two regimes: **below saturation** (active well under capacity) the configured rate is the binding constraint — this is the steady-state case the NFR table's "±2% of target rate/min" describes. **Near/at saturation** the ceiling binds instead, and effective admission can drop to zero regardless of configured rate, until active-count drops via exits or TTL expiry.

### QueueGuard — increment on admission

```
// inside QueueGuard, after successful SETNX (Section 08):
rdb.Incr(c.Request.Context(), "active:"+claims.EventID)
```

### /queue/exit — decrement on completion

```go
func (h *Handler) QueueExit(c *gin.Context) {
    var req struct{ EventID string `json:"eventId"` }
    if err := c.BindJSON(&req); err != nil {
        c.AbortWithStatus(http.StatusBadRequest); return
    }
    h.redis.Decr(c.Request.Context(), "active:"+req.EventID)
    c.Status(http.StatusNoContent)
}
```

If a client never calls `/queue/exit`, `active` is not simply left stale forever — a background TTL sweep (or a Redis keyspace-notification listener on `token:{jti}` expiry) decrements `active:{eventId}` when a session's underlying token TTL lapses, so abandoned sessions still free their slot within the 30-minute bound.

**⚡ Frontend Must Distinguish Rate-Blocked vs Capacity-Blocked:** The naive wait-time estimate (`rank / admitRatePerMin`) assumes steady admission at the configured rate. When the queue is ceiling-blocked, effective admission can be at or near zero even though `rate` looks unchanged — position stops moving. Expose a `constrained: true` flag (or similar) from `/queue/status` so the client can show "waiting for capacity" rather than a countdown that silently isn't decrementing — a shrinking-then-frozen ETA reads as a bug to users, not as correct behavior.

**⚡ Race Window Worth Flagging:** The scheduler's leader lock (`scheduler:lock:{eventId}`) guarantees only one scheduler decides `n` per tick, but the origin-side `active` increment happens independently, slightly after that decision, inside each individual request's `QueueGuard` pass. Between "scheduler decided N was safe" and "all N have actually completed SETNX + increment," true concurrent `active` can transiently exceed the ceiling by up to one tick's admitted batch. For strict hard-capacity use cases (fixed inventory), treat `capacity` with a small safety margin rather than an exact ceiling.

> Section 07

## Transport Design — Hybrid SSE + Polling

**Changed from v2:** v2 used SSE unconditionally for every queued user. At very high concurrency (500k+), holding one persistent connection + one Go goroutine + one Redis pub/sub subscription per waiting user becomes the bottleneck itself — not CPU, but connection count, file descriptors, and the operational cost of draining hundreds of thousands of long-lived connections during a deploy. v3 uses SSE only where its low-latency advantage actually matters, and polling everywhere else.

### Why Hybrid, and Where the Line Sits

- **Far from the front (poll, every 5s):** stateless request/response — no persistent connection, no idle goroutine, no Redis subscription held open, nothing to drain on deploy. Standard HTTP infra (ALB, Fargate autoscaling) handles this shape at scale without special casing. A 3–5s lag on a position update is imperceptible when the user's estimated wait is measured in minutes.
- **Near the front (SSE, last ~200 positions or ~2 min estimated wait):** the one moment where latency is genuinely felt — "you're about to be let in" needs to land in under a second, not up to 5s late. SSE's push model plus the existing `ticket:updates:{ticketId}` pub/sub channel delivers the admission token essentially instantly. The connection count at this tier is small by construction (~200 users per event, not the full queue), so the per-connection cost that was prohibitive at full scale is a non-issue here.
- **Crossover trigger:** client JS re-evaluates its own ZRANK on every poll response. When rank crosses the threshold (configurable, default 200), it closes the poll loop and opens an `EventSource` connection instead — a one-way upgrade, never downgraded, since rank only decreases while waiting.

### Redis Pub/Sub Fan-Out Pattern (SSE tier only)

Two channels per event drive SSE streams for the near-front tier:

- `queue:tick:{eventId}` — one publish by scheduler after each ZPOPMIN batch. Connected SSE handlers (small set — only near-front users) independently re-read their own ZRANK. One pub/sub message triggers a handful of parallel ZRANK reads, not hundreds of thousands.
- `ticket:updates:{ticketId}` — one publish per admitted ticket, containing the JWT. Delivered to that user's SSE goroutine if connected; if the user is still in the polling tier when admitted, the token is instead picked up on their next poll response (see poll handler below).

### Go SSE Handler (near-front tier)

```go
func (h *Handler) QueueStatusSSE(c *gin.Context) {
    ticketID := c.Param("ticketId")
    eventID  := h.eventIDFromTicket(c, ticketID)

    c.Header("Content-Type",   "text/event-stream")
    c.Header("Cache-Control",  "no-cache")
    c.Header("Connection",     "keep-alive")
    c.Header("X-Accel-Buffering", "no") // disable Nginx/proxy buffering

    flusher := c.Writer.(http.Flusher)

    // Subscribe BEFORE reading initial rank to avoid race condition:
    // admission between rank-read and subscribe would be missed
    pubsub := h.redis.Subscribe(c.Request.Context(),
        "queue:tick:"+eventID,
        "ticket:updates:"+ticketID,
    )
    defer pubsub.Close()

    if rank, err := h.getPosition(c, ticketID, eventID); err == nil && rank >= 0 {
        fmt.Fprintf(c.Writer, "event: update\ndata: {\"type\":\"position\",\"value\":%d}\n\n", rank)
        flusher.Flush()
    }

    heartbeat := time.NewTicker(15 * time.Second)
    defer heartbeat.Stop()

    for {
        select {
        case msg := <-pubsub.Channel():
            var ev map[string]string
            json.Unmarshal([]byte(msg.Payload), &ev)

            switch ev["type"] {
            case "tick":
                rank, err := h.getPosition(c.Request.Context(), ticketID, eventID)
                if err != nil || rank < 0 { continue } // popped — await admitted event
                data, _ := json.Marshal(map[string]any{"type":"position","value":rank})
                fmt.Fprintf(c.Writer, "event: update\ndata: %s\n\n", data)
                flusher.Flush()

            case "admitted":
                data, _ := json.Marshal(map[string]string{
                    "type":"admitted", "token":ev["token"],
                })
                fmt.Fprintf(c.Writer, "event: update\ndata: %s\n\n", data)
                flusher.Flush()
                return
            }

        case <-heartbeat.C:
            fmt.Fprintf(c.Writer, ": heartbeat\n\n")
            flusher.Flush()

        case <-c.Request.Context().Done():
            return
        }
    }
}
```

### Go Poll Handler (far-from-front tier)

```go
// Single-shot, stateless. Client re-calls this every 5s on a timer.
// If the ticket was admitted since the last poll, the token is
// picked up here directly — no pub/sub needed for this tier.
func (h *Handler) QueueStatusPoll(c *gin.Context) {
    ticketID := c.Param("ticketId")
    eventID  := h.eventIDFromTicket(c, ticketID)

    // Check if this ticket was already admitted (token cached against ticketId
    // by the scheduler at issuance time, short TTL, read-once semantics)
    if token, err := h.redis.HGet(c.Request.Context(), "ticket:"+ticketID, "admission_token").Result(); err == nil {
        h.redis.HDel(c.Request.Context(), "ticket:"+ticketID, "admission_token")
        c.JSON(200, gin.H{"type": "admitted", "token": token})
        return
    }

    rank, err := h.getPosition(c.Request.Context(), ticketID, eventID)
    if err != nil || rank < 0 {
        // popped but token not yet cached — transient, client retries in ~1s
        c.JSON(200, gin.H{"type": "pending"})
        return
    }

    c.JSON(200, gin.H{
        "type": "position", "value": rank,
        "upgrade_to_sse": rank < h.cfg.SSEThreshold, // e.g. 200
    })
}
```

The scheduler's `admitBatch` now writes the issued token into the existing `ticket:{ticketId}` hash as `admission_token` (in addition to publishing to `ticket:updates:{ticketId}`) so the polling tier — which has no pub/sub subscription — can pick it up on its next scheduled poll rather than needing a push channel.

### Queue Waiting Page — Client JS (Hybrid Crossover)

```javascript
// queue.js — identical static file served to every queued user from S3/CDN
const params   = new URLSearchParams(location.search);
const ticketId = params.get('ticket');
const target   = params.get('target');
const SSE_THRESHOLD = 200; // crossover point — matches server-side cfg.SSEThreshold

if (target) sessionStorage.setItem('q_target', target);

let pollTimer = null;
let es = null;

function handleAdmitted(token) {
    if (es) es.close();
    if (pollTimer) clearInterval(pollTimer);
    document.cookie = `q_admission=${token}; path=/; max-age=1800; SameSite=Strict; Secure`;
    window.location.href = sessionStorage.getItem('q_target') || '/';
}

function startSSE() {
    es = new EventSource(`https://queue-api.yourdomain.com/queue/status/${ticketId}?mode=sse`);
    es.addEventListener('update', (e) => {
        const data = JSON.parse(e.data);
        if (data.type === 'position') renderPosition(data.value);
        if (data.type === 'admitted') handleAdmitted(data.token);
    });
    es.onerror = () => { document.getElementById('status').textContent = 'Reconnecting...'; };
}

async function pollOnce() {
    const res  = await fetch(`https://queue-api.yourdomain.com/queue/status/${ticketId}?mode=poll`);
    const data = await res.json();

    if (data.type === 'admitted') { handleAdmitted(data.token); return; }
    if (data.type === 'pending') return; // try again next tick

    renderPosition(data.value);
    // Crossover: close polling, switch to SSE for the low-latency final stretch
    if (data.upgrade_to_sse && !es) {
        clearInterval(pollTimer);
        startSSE();
    }
}

function renderPosition(rank) {
    document.getElementById('pos').textContent = `${rank} people ahead`;
    const mins = Math.ceil(rank / admitRatePerMin);
    document.getElementById('wait').textContent =
        rank < 1 ? 'Less than a minute' : `~${mins} min`;
}

// Start in polling mode; crossover to SSE happens automatically near the front
pollTimer = setInterval(pollOnce, 5000);
pollOnce(); // immediate first read, don't wait 5s for it
```

**⚡ Connection Count at Scale, Before vs After:** At 500k concurrent queued users, v2's all-SSE model needs ~500k persistent connections and Redis subscriptions. With a 200-position SSE threshold, the same event needs roughly (number of active events) × 200 SSE connections at any moment — the rest are stateless 5s polls. This is the change that actually makes the 500k+ tier operationally viable rather than just theoretically described.

> Section 08

## Akamai EdgeWorker Design

**Correction from v2 — Two Independent Secrets:** v2's EdgeWorker used one `QUEUE_SECRET` to verify both `q_admission` and `q_session`, while origin already signed them with two different secrets. v3 provisions two Akamai Property Manager variables — `ADMISSION_SECRET` and `SESSION_SECRET` — each mirroring the corresponding origin secret exactly, and routes verification accordingly.

### EdgeWorker Implementation (corrected)

```javascript
// Akamai EdgeWorker — onClientRequest
// ADMISSION_SECRET and SESSION_SECRET loaded from Akamai Property Manager
// variables — two independent keys, mirroring origin's cfg.QueueSecret /
// cfg.SessionSecret respectively. Compromise of one does not expose the other.
import { Cookies } from 'http-cookies';

const QUEUE_JOIN  = 'https://queue.yourdomain.com/join';
const SKIP_PATHS  = ['/static/', '/assets/', '/health', '/favicon', '/public/'];

export async function onClientRequest(request) {
    if (SKIP_PATHS.some(p => request.path.startsWith(p))) return;

    const cookies = new Cookies(request.getHeader('Cookie') || '');
    const sessionToken   = cookies.get('q_session');
    const admissionToken = cookies.get('q_admission');

    // Fast path: active session — verified with SESSION_SECRET, inline, no external call
    if (sessionToken) {
        const r = await verifySignedCookie(sessionToken, SESSION_SECRET);
        if (r.valid && !r.expired) return;
        if (r.expired) {
            const t = encodeURIComponent(request.url);
            request.respondWith(302, {'Location':`${QUEUE_JOIN}?target=${t}&reason=expired`}, '');
            return;
        }
        // invalid signature (tampered/forged) falls through to admission check, then to re-queue
    }

    // Admission token: verified with ADMISSION_SECRET — a different key from SESSION_SECRET.
    // Signature check only here; SETNX one-time enforcement + active++ happen at origin.
    if (admissionToken) {
        const r = await verifyJWT(admissionToken, ADMISSION_SECRET);
        if (r.valid && !r.expired) return; // forward; origin does SETNX + active++
    }

    // No valid token under either secret — redirect to queue
    const t = encodeURIComponent(request.url);
    request.respondWith(302, {'Location':`${QUEUE_JOIN}?target=${t}`}, '');
}

// Shared verification routine — takes the secret as a parameter so the
// same HMAC logic serves both token types without duplicating code.
async function verifyJWT(token, secret) {
    try {
        const [hdr, pay, sig] = token.split('.');
        const key = await crypto.subtle.importKey(
            'raw', new TextEncoder().encode(secret),
            {name:'HMAC',hash:'SHA-256'}, false, ['verify']
        );
        const valid = await crypto.subtle.verify('HMAC', key,
            base64UrlDecode(sig), new TextEncoder().encode(`${hdr}.${pay}`));
        if (!valid) return {valid:false};
        const p = JSON.parse(atob(pay.replace(/-/g,'+').replace(/_/g,'/')));
        return {valid:true, expired: Date.now()/1000 > p.exp};
    } catch { return {valid:false}; }
}

// q_session uses the same HMAC-verify mechanics as the JWT path, just against
// a non-JWT-structured payload (sessionId, eventId, iat, exp) — same function
// signature works for both since both are '.'-shaped at the wire level.
async function verifySignedCookie(token, secret) {
    return verifyJWT(token, secret); // structurally identical HMAC check, different secret
}
```

### Origin QueueGuard Middleware — SETNX + Active Increment + Session Upgrade

```go
func QueueGuard(cfg Config, rdb *redis.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. Session cookie — fast path, verified against cfg.SessionSecret
        if sc, err := c.Cookie("q_session"); err == nil {
            if token.ValidateSession(sc, cfg.SessionSecret) == nil {
                c.Next(); return
            }
        }
        // 2. Admission token — verified against cfg.QueueSecret (== edge's ADMISSION_SECRET)
        ac, err := c.Cookie("q_admission")
        if err != nil {
            c.Redirect(http.StatusFound, cfg.QueueURL+"?target="+
                url.QueryEscape(c.Request.URL.String()))
            c.Abort(); return
        }
        claims, err := token.ValidateJWT(ac, cfg.QueueSecret)
        if err != nil {
            c.Redirect(http.StatusFound, cfg.QueueURL+"?target="+
                url.QueryEscape(c.Request.URL.String()))
            c.Abort(); return
        }
        // 3. SETNX — one-time use enforcement (prevents admission URL sharing)
        set, err := rdb.SetNX(c.Request.Context(),
            "token:"+claims.ID, "used", 30*time.Minute).Result()
        if err != nil || !set {
            c.AbortWithStatus(http.StatusForbidden); return
        }
        // 4. NEW — bump active count now that admission is confirmed one-time-valid
        rdb.Incr(c.Request.Context(), "active:"+claims.EventID)
        // 5. Issue session cookie signed with cfg.SessionSecret — a different key from cfg.QueueSecret
        sc, _ := token.IssueSession(claims.Subject, claims.EventID, cfg.SessionSecret)
        c.SetCookie("q_session", sc, 1800, "/", "", true, true)
        c.SetCookie("q_admission", "", -1, "/", "", true, true) // clear
        c.Next()
    }
}
```

> Section 09

## Data Model

### Redis Keys

| Key Pattern | Type | Purpose | TTL |
| --- | --- | --- | --- |
| `queue:{eventId}` | Sorted Set | Waiting users. Score = join timestamp ms. ZRANK = people ahead. ZPOPMIN = admit batch. | Event duration + 1hr |
| `ticket:{ticketId}` | Hash | sessionId, eventId, join time, user fingerprint, admission_token. Read on connect/poll. Token field is cleared after poll delivery. | 4 hours |
| `token:{jti}` | String | One-time use flag. SETNX at origin on first admission. Presence = token consumed. | 30 min (matches JWT TTL) |
| `rate:{eventId}` | String | Configured admit rate (visitors/min). Written by admin API. One of two inputs to the scheduler's per-tick admit count. | No TTL |
| `capacity:{eventId}` | String | **New in v3.** Max concurrent admitted users allowed for this event. Second input to the scheduler's per-tick admit count. | No TTL |
| `active:{eventId}` | String (counter) | **New in v3.** Current count of admitted-and-not-yet-exited users. Incremented in QueueGuard on admission, decremented on `/queue/exit` or TTL sweep. Read by scheduler every tick to compute headroom. | No TTL (counter, not a record) |
| `scheduler:lock:{eventId}` | String NX+TTL | Distributed leader lock. One scheduler instance admits at a time. | 10 seconds |

**Correction from v1/v2:** The `admitted:{eventId}` total-admitted counter (v1) was removed from the hot path — ZRANK is sufficient for user-facing position. The new `active:{eventId}` counter introduced in v3 is a different thing entirely: it is not a running total, it is a live concurrency gauge that goes up and down, and it directly feeds the scheduler's admission decision rather than being purely a dashboard stat.

### DynamoDB

| Table | PK / SK | Purpose |
| --- | --- | --- |
| queue-events | eventId | Event config: name, protected URL, admit rate, capacity, start/end time. |
| queue-sessions | ticketId / eventId | Audit record per queued session. TTL 30 days. |
| queue-audit-log | eventId / timestamp#action | Immutable log: rate changes, capacity changes, overrides, scheduler events. |

> Section 10

## Go Service Design

```
internal-queue/
├── cmd/queueserver/main.go
├── internal/
│   ├── api/
│   │   ├── handler_join.go      # POST /queue/join — ZADD + redirect
│   │   ├── handler_status.go    # GET  /queue/status/:id — SSE and poll modes
│   │   ├── handler_exit.go      # POST /queue/exit — decrements active:{eventId}
│   │   └── handler_admin.go     # PUT  /queue/rate — live rate + capacity adjustment
│   ├── scheduler/
│   │   ├── admission.go         # ticker · headroom calc · ZPOPMIN · pub/sub publish
│   │   └── leader_lock.go       # Redis SETNX leader election
│   ├── token/
│   │   ├── jwt.go               # HMAC JWT issue + validate (q_admission, ADMISSION_SECRET)
│   │   └── session.go           # signed session cookie issue + validate (q_session, SESSION_SECRET)
│   ├── store/
│   │   ├── redis.go             # client wrapper + Lua scripts
│   │   └── dynamo.go
│   └── config/config.go
├── pkg/middleware/
│   └── queue_guard.go           # drop-in: SETNX + active++ + session upgrade
└── deploy/
    ├── task-definition.json
    └── terraform/
```

**⚡ Proactive Slot Release:** The protected service (checkout, seat lock) should call `POST /queue/exit` on transaction complete or session timeout. In a 30-minute ticket sale window, abandoned slots sitting on 30-minute TTL waste real capacity — and with v3's active-count-gated admission, an un-reclaimed abandoned session now directly blocks a real waiting user once the event is at its capacity ceiling, not just a minor inefficiency.

> Section 11

## AWS Infrastructure

- **🌐 Akamai EdgeWorkers**: Queue protection on `app.yourdomain.com`. HMAC verify inline against two independent secrets. Queue API and queue page subdomains fully bypass EdgeWorker queue logic. WAF handles bot/DDoS upstream.
- **📦 ECS Fargate (Queue API)**: Containerized Go service. Scale on `active_sse_connections` (now bounded by SSE threshold, not total queue size) + CPU. 2–8 tasks at idle, burst to 20+. 512MB / 0.5 vCPU per task.
- **⚡ ElastiCache Redis Cluster**: Sorted set queue + pub/sub + active/capacity counters. cache.r7g.large, 2 shards for <500k users. AOF persistence. Pub/sub channel count now scales with SSE-tier size only, not total queue.
- **🗂️ S3 + Akamai CDN**: Static HTML/JS/CSS queue waiting page with hybrid poll/SSE client logic. Same files for all users. Works even if origin is down.
- **📬 SQS FIFO**: Admission audit events. MessageGroupId = eventId. Consumed asynchronously by analytics/notification services. Does not affect queue hot path.
- **📊 CloudWatch + Alarms**: Key metrics: `active_sse_connections`, `queue_depth`, `admit_rate_actual`, `active_users`, `capacity_headroom`, `setnx_failures`. Alarm → SNS → PagerDuty.

### Scaling

| Dimension | Bottleneck | Action |
| --- | --- | --- |
| Concurrent queued users (poll tier) | Stateless request rate | Standard Fargate autoscale on request count/CPU — no persistent-connection ceiling. |
| Concurrent queued users (SSE tier) | SSE connections per ECS task | Bounded by SSE threshold (~200/event) rather than total queue size — scale on `active_sse_connections`. |
| Enqueue request rate | Redis ZADD O(log N) | Fargate auto-scale; Redis cluster handles millions of ops/sec. |
| Admission throughput | Scheduler ZPOPMIN + pub, now gated by min(rate, headroom) | Single leader by design; increase batch size per tick for higher rates, subject to capacity ceiling. |
| Token validation at origin | QueueGuard middleware | Stateless HMAC check. SETNX + active++ is one Redis roundtrip per new admission only. |
| Queue page serving | S3 + Akamai CDN | Unlimited — static files, Akamai absorbs all load, origin not involved. |

**⚡ Pre-warm Before Events:** Pre-warm ECS tasks 10 minutes before known go-live via ECS Service minimum task count. Pre-allocate Redis memory. Set `capacity:{eventId}` deliberately below true hard capacity for the first admission wave to absorb the active-count race window noted in Section 06.5. The first 30 seconds of a ticket sale are the highest-risk window.

> Section 12

## Non-Functional Requirements

| NFR | Target | How Achieved |
| --- | --- | --- |
| Enqueue latency p99 | <50ms | Redis ZADD ~1ms; Go handler ~5ms; network budget ~40ms. |
| Queue capacity | Millions of users | Redis Sorted Set + cluster mode. No hard limit. |
| Admission accuracy | ±2% of `min(configured_rate, headroom)` | Go ticker + atomic ZPOPMIN; absolute tick anchoring, no drift. |
| Queue page availability | 99.99% | Static S3 + Akamai CDN. Independent of all backend services. |
| Edge token check latency | <2ms added | HMAC-SHA256 in EdgeWorker SubtleCrypto against ADMISSION_SECRET or SESSION_SECRET — no external call. |
| Origin SETNX + active++ added latency | <5ms | Two Redis calls on first admission only. Session cookie path: no Redis at all. |
| Admission → redirect latency (SSE tier) | <500ms | Redis pub/sub ~1ms; SSE flush immediate; JS redirect instant. |
| Admission → redirect latency (poll tier) | <5s | Bounded by poll interval; acceptable since this tier is far from admission by definition. |
| Concurrent persistent connections | ~200 per active event, not total queue size | SSE reserved for near-front tier via crossover threshold. |
| Data durability | Positions survive restart | Redis AOF. SQS durable audit record. |
| RTO Queue Service | <2 minutes | ECS auto-restart; ElastiCache replica auto-promotion. |
| Bot resistance | Block automated enqueue | CAPTCHA on join; Akamai WAF; signed ticket prevents position spoofing. |

**Known Gap vs Queue-Fair:** Queue-Fair's patented one-order-per-visitor and pre-sale randomiser (lottery mode) are not covered by this design. This covers pure FIFO traffic protection plus concurrency-ceiling enforcement. Lottery-style access requires a separate pre-sale ballot service.

> Section 13

## Cost Analysis: In-House vs Queue-Fair

Assumes: ~20 high-demand events/year, peak 50k concurrent queued users, 500 admit/min, 30–60 min events. Hybrid transport and capacity accounting change infra shape slightly (fewer persistent connections, two extra Redis keys/reads per tick) but not materially the cost envelope below.

#### 🏗️ In-House (AWS)

- ECS Fargate (4 tasks avg): ~$60/mo
- ElastiCache r7g.large (2x): ~$220/mo
- S3 + data transfer: ~$5/mo
- DynamoDB (on-demand): ~$15/mo
- SQS + CloudWatch: ~$10/mo
- Akamai EdgeWorkers (incremental): ~$20/mo
- **AWS Total**: ~$330/mo

#### 📦 Queue-Fair (Unlimited)

- License (estimated): £1,000–3,000/mo
- Integration effort: 1–3 dev days
- Maintenance: Zero
- Scaling: Their problem
- Additional AWS infra: $0
- **Total/mo (USD approx)**: ~$1,250–$3,750

### 3-Year TCO

| Year | In-House | Queue-Fair (mid) | Delta |
| --- | --- | --- | --- |
| Year 1 | ~$110k build + $4k AWS = $114k | ~$30k | QF cheaper ~$84k |
| Year 2 | ~$64k (eng + AWS) | ~$30k | QF cheaper ~$34k |
| Year 3 | ~$64k | ~$30k | QF cheaper ~$34k |
| **3-Year Total** | **~$242k** | **~$90k** | QF saves ~$152k |

**Cost verdict:** At medium event frequency Queue-Fair is cheaper within 3 years. In-house wins only at high event frequency, high QF pricing, or compliance triggers.

| In-House Wins When | Trigger Point |
| --- | --- |
| High event frequency | >100 events/year — QF annual cost exceeds $100k; in-house marginal AWS cost ~$4k/yr |
| QF pricing escalates | At £3k+/mo, in-house payback drops to ~18 months |
| Data residency / compliance | Fintech — user PII flows through QF's Google Cloud infra. Regulatory blocker. |
| Custom integration required | QF charges bespoke; in-house is fully extensible, including hard concurrency ceilings QF doesn't natively expose |

> Section 14

## Build vs Buy — Verdict

> **Recommendation: Start with Queue-Fair; build in-house at scale or compliance trigger**
> 
> Queue-Fair solves a genuinely hard problem with 18+ years of iteration — accurate rate-based admission, bot resistance, CDN-hosted static queue page, zero-code integration. The in-house design is architecturally sound but costs significantly more in Year 1 and 2. The calculus flips at high event frequency, escalating QF pricing, or when fintech data residency requirements surface.

| Factor | Queue-Fair | In-House |
| --- | --- | --- |
| Time to production | Days | 2–3 months |
| Year 1 cost (medium use) | Lower (~$30k) | Higher (~$114k) |
| Year 3+ (high use) | Ongoing license | ~$4k/yr AWS only |
| Operational burden | Zero | Full ownership |
| Extensibility | Limited to their API | Unlimited — incl. hard concurrency ceilings |
| Data residency / PII | Third-party Google Cloud | Fully in-house |
| Bot/abuse maturity | 18yr battle-tested | Needs investment |
| Akamai native integration | Adapter / JS snippet | Native EdgeWorker control |
| Vendor lock-in risk | Single vendor | None |
| Connection scaling at 500k+ | Their infra | Hybrid SSE/poll makes this tractable in-house too |

### Tactical Playbook

**⚡ Phase 1 (0–12mo):** Use Queue-Fair Free Tier for low-traffic events; Queue-Fair Unlimited for critical events. Instrument: event frequency, peak queue depth, cost per event.

**⚡ Phase 2 (12–18mo):** If >50 events/year or data residency requirements surface, begin in-house build. Start with Redis queue store + scheduler, including capacity/active accounting from day one — retrofitting a concurrency ceiling later is harder than building it in. Keep Queue-Fair for branded wait page initially (hybrid), cut over when in-house page is ready.

**⚡ Phase 3 (18mo+):** Full in-house. Queue-Fair cancelled. Breakeven at ~18mo if QF >£2k/month. Team owns full roadmap: lottery mode, pre-sale ballots, per-product capacity controls, native Akamai EdgeWorker integration.

**⚡ Negotiation:** Queue-Fair publicly commits to "25% off any competitor quote." Get a quote from queue-it.com or Cloudflare Waiting Room first, use it before signing any annual contract.
