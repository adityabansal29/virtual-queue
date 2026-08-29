# Requirements: Virtual Queue System

**Defined:** 2026-08-02
**Core Value:** A waiting user must always see an accurate position, and an admitted user must always be let through exactly once — no skips, no duplicates, no queue bypass.

## v1 Requirements

### Queue Core

- [x] **QUEUE-01**: User can join the queue and receive a unique ticketId (POST /queue/join → ZADD with join-timestamp score)
- [x] **QUEUE-02**: User can poll their queue position (GET /queue/status/:id?mode=poll — stateless ZRANK read, connection closes immediately)
- [x] **QUEUE-03**: Poll response includes `upgrade_to_sse: true` hint when rank < SSE_THRESHOLD (default 200)
- [x] **QUEUE-04**: User can connect via SSE to receive real-time position updates when near the front (GET /queue/status/:id?mode=sse)
- [x] **QUEUE-05**: SSE handler subscribes to `queue:tick:{eventId}` and `ticket:updates:{ticketId}` pub/sub channels before reading initial rank (avoids race)
- [x] **QUEUE-06**: Admission scheduler ticks every second and admits min(rate, headroom) users via atomic ZPOPMIN
- [x] **QUEUE-07**: Scheduler holds a distributed leader lock (Redis SETNX NX+TTL) so only one instance admits per tick
- [x] **QUEUE-08**: Admitted token is written to `ticket:{ticketId}` hash field `admission_token` for poll-tier pickup (in addition to pub/sub publish)
- [x] **QUEUE-09**: User can call POST /queue/exit to proactively decrement `active:{eventId}` and free their slot

### Token & Security

- [x] **TOKEN-01**: Scheduler issues HMAC-signed JWT (q_admission) with claims: JTI, eventId, ticketId, iat, exp (30min), signed with ADMISSION_SECRET
- [x] **TOKEN-02**: ADMISSION_SECRET and SESSION_SECRET are two independent keys — compromise of one does not expose the other
- [x] **TOKEN-03**: Go middleware (EdgeWorker-equivalent) verifies q_session with SESSION_SECRET or q_admission with ADMISSION_SECRET inline (no Redis call), redirects if missing/invalid/expired
- [x] **TOKEN-04**: Origin QueueGuard performs SETNX on `token:{jti}` (TTL 30min) after valid JWT check — SETNX fail → 403 (token already used)
- [x] **TOKEN-05**: QueueGuard increments `active:{eventId}` after successful SETNX and issues q_session cookie signed with SESSION_SECRET
- [x] **TOKEN-06**: QueueGuard clears q_admission cookie and passes request to business handler on success
- [x] **TOKEN-07**: `active:{eventId}` counter tracks live concurrency; scheduler reads it every tick and computes headroom = capacity - active; zero headroom → skip tick

### Frontend

- [x] **UI-01**: Queue waiting page is a static HTML/JS shell (identical for every user) served from S3 via CDN
- [x] **UI-02**: Queue page starts in polling mode (every 5s) and upgrades to SSE when rank < SSE_THRESHOLD without page reload
- [x] **UI-03**: Queue page displays position ("X people ahead") and estimated wait time (~rank/admitRatePerMin minutes)
- [x] **UI-04**: Queue page shows "waiting for capacity" state when server returns `constrained: true` (ceiling-blocked queue does not show a decrementing countdown)
- [x] **UI-05**: On admission event, queue page sets q_admission cookie, reads target URL from sessionStorage, redirects to protected origin
- [x] **UI-06**: Admin dashboard shows live queue depth, active users, admit rate, capacity, and capacity headroom
- [x] **UI-07**: Admin dashboard allows live adjustment of admit rate and capacity ceiling via PUT /queue/rate/:eventId
- [x] **UI-08**: Stub ticket checkout validates q_session cookie and displays simulated seat selection page

### Infrastructure

- [x] **INFRA-01**: Docker Compose runs full local stack: Redis, queue API, stub ticket checkout, queue waiting page, admin dashboard
- [ ] **INFRA-02**: ECS Fargate task definition for queue service (512MB / 0.5 vCPU, auto-scale on active_sse_connections + CPU)
- [ ] **INFRA-03**: ElastiCache Redis cluster configuration (cache.r7g.large, 2 shards, AOF persistence enabled)
- [ ] **INFRA-04**: S3 bucket + CloudFront distribution configuration for static queue waiting page
- [ ] **INFRA-05**: DynamoDB tables: queue-events (event config), queue-sessions (audit per ticket), queue-audit-log (immutable event log)
- [ ] **INFRA-06**: SQS FIFO queue for admission audit events with MessageGroupId = eventId

## v2 Requirements

### Edge (Real Akamai)

- **EDGE-01**: Deploy EdgeWorker to real Akamai property on app.yourdomain.com
- **EDGE-02**: Configure two independent Akamai Property Manager variables (ADMISSION_SECRET, SESSION_SECRET)
- **EDGE-03**: Validate EdgeWorker token verification matches local middleware behavior end-to-end

### Resilience & Operations

- **OPS-01**: Load test: 10k concurrent queued users, steady-state admission at configured rate
- **OPS-02**: CloudWatch alarms: queue_depth, active_users, capacity_headroom, setnx_failures → SNS
- **OPS-03**: TTL-sweep background job: decrement active:{eventId} on token:{jti} keyspace expiry notification (abandoned sessions)

### Bot Resistance

- **BOT-01**: CAPTCHA on /queue/join (hCaptcha or equivalent)
- **BOT-02**: Akamai WAF upstream of queue protection

## Out of Scope

| Feature | Reason |
|---------|--------|
| Real payment processing | Stub origin is sufficient for demonstrating queue mechanics |
| Lottery / pre-sale ballot | Separate problem — not FIFO, requires ballot service |
| CAPTCHA / bot resistance | Deferred to v2; not core to queue mechanics learning goal |
| 500k+ load testing | Design validated on paper; functional correctness is v1 target |
| Multi-tenant / SaaS mode | Single-operator use case for learning; generalization deferred |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| QUEUE-01 | Phase 1 | Complete |
| QUEUE-02 | Phase 1 | Complete |
| QUEUE-03 | Phase 1 | Complete |
| QUEUE-04 | Phase 1 | Complete |
| QUEUE-05 | Phase 1 | Complete |
| QUEUE-06 | Phase 1 | Complete |
| QUEUE-07 | Phase 1 | Complete |
| QUEUE-08 | Phase 1 | Complete |
| QUEUE-09 | Phase 1 | Complete |
| TOKEN-01 | Phase 1 | Complete |
| TOKEN-02 | Phase 1 | Complete |
| TOKEN-03 | Phase 1 | Complete |
| TOKEN-04 | Phase 1 | Complete |
| TOKEN-05 | Phase 1 | Complete |
| TOKEN-06 | Phase 1 | Complete |
| TOKEN-07 | Phase 1 | Complete |
| UI-01 | Phase 2 | Complete |
| UI-02 | Phase 2 | Complete |
| UI-03 | Phase 2 | Complete |
| UI-04 | Phase 2 | Complete |
| UI-05 | Phase 2 | Complete |
| UI-06 | Phase 2 | Complete |
| UI-07 | Phase 2 | Complete |
| UI-08 | Phase 2 | Complete |
| INFRA-01 | Phase 1 | Complete |
| INFRA-02 | Phase 3 | Pending |
| INFRA-03 | Phase 3 | Complete |
| INFRA-04 | Phase 3 | Pending |
| INFRA-05 | Phase 3 | Complete |
| INFRA-06 | Phase 3 | Complete |

**Coverage:**

- v1 requirements: 30 total
- Mapped to phases: 30 ✓
- Unmapped: 0

---
*Requirements defined: 2026-08-02*
*Last updated: 2026-08-02 after roadmap creation*
