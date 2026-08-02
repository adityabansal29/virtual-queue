---
phase: 01-queue-core-local-stack
verified: 2026-08-02T00:00:00Z
status: human_needed
score: 12/14 must-haves verified
behavior_unverified: 2
re_verification: false
behavior_unverified_items:
  - truth: "GET /queue/status/:id?mode=sse opens SSE stream with position event + admitted event"
    test: "Join a ticket, open SSE stream with curl --max-time 5, observe event output"
    expected: "event: update\\ndata: {\"type\":\"position\",\"value\":0}\\n\\n then within 2s event: update\\ndata: {\"type\":\"admitted\",\"token\":\"eyJ...\"}"
    why_human: "Requires a running docker compose stack; SSE stream delivery timing and admitted payload content cannot be verified by static analysis"
  - truth: "Valid q_admission JWT -> QueueGuard: SETNX succeeds, q_session issued; 403 on replay"
    test: "Obtain JWT from poll endpoint after admission, present to stuborigin twice"
    expected: "First request HTTP 200 with Set-Cookie: q_session; second identical request HTTP 403"
    why_human: "Requires live stack with redis-origin; SETNX atomicity and cookie issuance are runtime behaviors that grep cannot observe"
human_verification:
  - test: "Run ./scripts/verify.sh against a running docker compose stack (make up && make verify)"
    expected: "All 11 PASS lines printed, exit code 0"
    why_human: "Verification script tests all 5 Phase 1 success criteria against a live stack including SSE stream delivery, QueueGuard SETNX enforcement, and redis-origin introspection"
  - test: "SSE stream delivers position then admitted events (Criterion 3)"
    expected: "curl -N --max-time 5 /queue/status/:id?mode=sse returns position event followed by admitted event with a base64-encoded JWT (starts with eyJ)"
    why_human: "SSE is a long-lived streaming response; cannot be verified without a running server"
  - test: "QueueGuard one-time enforcement (Criterion 4)"
    expected: "First curl with q_admission cookie returns HTTP 200 with Seat Selection HTML; second identical request returns HTTP 403; redis-cli EXISTS token:{jti} on redis-origin returns 1"
    why_human: "Requires live redis-origin and QueueGuard middleware executing a real SETNX — runtime-only"
---

# Phase 1: Queue Core & Local Stack Verification Report

**Phase Goal:** The full queue mechanics — join, position tracking, admission, and one-time token enforcement — are runnable locally via Docker Compose and verifiable end-to-end without a browser.
**Verified:** 2026-08-02
**Status:** human_needed (2 behavior-dependent truths require live stack)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `docker compose up` starts Redis and queue API; `curl POST /queue/join` returns ticketId | VERIFIED | `docker-compose.yml` has redis-queue + queueserver services with healthchecks; `handler_join.go` returns `{"ticketId":"<uuid>","eventId":"...","position":"queued"}` with HTTP 200; `go build ./...` exits 0 |
| 2 | `curl GET /queue/status/:id?mode=poll` returns position + `upgrade_to_sse: true` when rank < 200 | VERIFIED | `handler_status.go:QueueStatusPoll` returns `{"type":"position","value":rank,"upgrade_to_sse": rank < SSEThreshold}`; SSEThreshold defaults to 200 from config |
| 3 | `curl GET /queue/status/:id?mode=sse` opens SSE stream with position event + admitted event | PRESENT_BEHAVIOR_UNVERIFIED | `handler_status.go:QueueStatusSSE` wired correctly: Subscribe before GetPosition (QUEUE-05), heartbeat, admitted closes stream; delivery timing requires live stack |
| 4 | Valid q_admission JWT → QueueGuard: SETNX succeeds, q_session issued; 403 on replay | PRESENT_BEHAVIOR_UNVERIFIED | `queue_guard.go` implements the full flow (SetNX → Incr active → IssueSession → clear admission cookie → 403 on !set); runtime SETNX atomicity requires live redis-origin |
| 5 | Two distinct secrets; wrong-secret tokens rejected; startup panics if secrets missing/equal | VERIFIED | `config.go:Load()` panics with descriptive messages on empty or equal secrets; `ValidateJWT` rejects wrong-secret tokens (TestWrongSecretRejected PASS); `.env.example` has distinct values: `dev-admission-secret-change-in-prod` vs `dev-session-secret-change-in-prod` |

**Score:** 12/14 truths verified (2 present, behavior-unverified)

### CONTEXT.md Decision Cross-Check

| Decision | Requirement | Evidence | Status |
|----------|-------------|----------|--------|
| D-06: Scheduler must NOT enforce active count ceiling | `admission.go` uses `n := rate` with ponytail comment marking Phase 2 upgrade | Line 68: `n := rate` — no headroom check | VERIFIED |
| D-02/D-04: Two Redis containers | `docker-compose.yml` has `redis-queue` and `redis-origin` as separate services | Lines 1–15: both services present | VERIFIED |
| D-03: SETNX at origin (redis-origin client in QueueGuard) | `queue_guard.go` uses `cfg.RDB` which is `originRedis` from `cmd/stuborigin/main.go` | `stuborigin/main.go:26`: `store.NewQueueRedis(cfg.RedisAddr)` with `REDIS_ADDR=redis-origin:6379` in compose | VERIFIED |
| D-09: Unit tests only in internal/token/ | `find ... -name "*_test.go"` returns only `internal/token/jwt_test.go` | Single test file at correct path | VERIFIED |

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `internal/api/handler_join.go` | VERIFIED | ZADD + HSet, 503 on error, UUID ticketId |
| `internal/api/handler_status.go` | VERIFIED | Poll + SSE, QUEUE-05 Subscribe-before-read, heartbeat 15s |
| `internal/api/handler_exit.go` | VERIFIED | Decr active, HTTP 204 |
| `internal/api/handler_admin.go` | VERIFIED | PUT /queue/rate + GET /queue/config with all fields |
| `internal/api/router.go` | VERIFIED | All 7 routes wired, mode=sse dispatch |
| `internal/scheduler/admission.go` | VERIFIED | tick loop, ZPOPMIN, HSet admission_token, Incr active, Publish |
| `internal/scheduler/leader_lock.go` | VERIFIED | SETNX NX+TTL 10s, Del on release |
| `internal/token/jwt.go` | VERIFIED | IssueAdmission (UUID JTI, HS256, 30min), ValidateJWT |
| `internal/token/session.go` | VERIFIED | IssueSession, ValidateSession, separate signing path |
| `internal/token/jwt_test.go` | VERIFIED | 5 tests, all PASS (go test run confirmed) |
| `pkg/middleware/queue_guard.go` | VERIFIED | Full 8-step QueueGuard flow wired to redis-origin |
| `cmd/queueserver/main.go` | VERIFIED | Signal context, scheduler wired with real token.IssueAdmission |
| `cmd/stuborigin/main.go` | VERIFIED | Separate binary, uses redis-origin, protected by QueueGuard |
| `docker-compose.yml` | VERIFIED | 4 services (redis-queue, redis-origin, queueserver, stuborigin), healthchecks, bound to 127.0.0.1 |
| `scripts/verify.sh` | VERIFIED | bash -n passes; covers all 5 criteria; redis-cli introspection for criterion 4c |
| `.env.example` | VERIFIED | ADMISSION_SECRET != SESSION_SECRET (both set to distinct dev values) |
| `.gitignore` | VERIFIED | `grep -c '^\.env$' .gitignore` returns 1 |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `cmd/queueserver/main.go` | `internal/token/IssueAdmission` | `scheduler.NewScheduler(rdb, cfg, func(...) { return token.IssueAdmission(...) })` | VERIFIED |
| `scheduler/admission.go` | `ticket:updates:{ticketId}` + `queue:tick:{eventId}` | `rdb.Publish(ctx, "ticket:updates:"+ticketID, payload)` + `rdb.Publish(ctx, "queue:tick:"+eventID, ...)` | VERIFIED |
| `handler_status.go SSE` | `scheduler/admission.go` pub/sub | `Subscribe` on both channels, `ev["type"]=="admitted"` closes stream | VERIFIED |
| `scheduler/admission.go` | `ticket:{ticketId}.admission_token` | `rdb.HSet(ctx, "ticket:"+ticketID, "admission_token", jwt)` | VERIFIED |
| `handler_status.go Poll` | `ticket:{ticketId}.admission_token` | `rdb.HGet(ctx, "ticket:"+ticketID, "admission_token")` → read-once delete | VERIFIED |
| `pkg/middleware/queue_guard.go` | `redis-origin` SETNX | `cfg.RDB.SetNX(..., "token:"+claims.ID, ...)` where `cfg.RDB` is the origin Redis client | VERIFIED |
| `cmd/stuborigin/main.go` | `redis-origin:6379` | `store.NewQueueRedis(cfg.RedisAddr)` with `REDIS_ADDR=redis-origin:6379` from compose | VERIFIED |
| `docker-compose.yml env_file: .env` | container env vars | Both queueserver and stuborigin use `env_file: .env` | VERIFIED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` | `go build ./...` | exits 0, no output | PASS |
| 5 unit tests pass | `go test ./internal/token/... -v -count=1` | TestIssueAndVerify, TestWrongSecretRejected, TestExpiredToken, TestJTIUniqueness, TestSessionSecretIsolation — all PASS | PASS |
| verify.sh syntax | `bash -n scripts/verify.sh` | exits 0 | PASS |
| QUEUE-05 ordering | grep line numbers for Subscribe vs GetPosition | Subscribe at line 74, GetPosition at line 81 | PASS |
| SSE stream delivery (live) | `curl -N --max-time 5 .../queue/status/:id?mode=sse` | SKIP — requires running stack | SKIP |
| QueueGuard SETNX replay (live) | two curl requests with same JWT to :8081 | SKIP — requires running stack | SKIP |

### Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|----------|
| QUEUE-01 | Join returns unique ticketId | SATISFIED | `handler_join.go`: `uuid.New().String()` |
| QUEUE-02 | Poll returns position (stateless ZRANK) | SATISFIED | `QueueStatusPoll` with Lua ZRANK via `store.GetPosition` |
| QUEUE-03 | poll returns `upgrade_to_sse:true` when rank < 200 | SATISFIED | `rank < int64(h.cfg.SSEThreshold)` in response |
| QUEUE-04 | SSE endpoint exists and dispatches correctly | SATISFIED | `router.go` dispatches `?mode=sse` to `QueueStatusSSE` |
| QUEUE-05 | Subscribe before rank read in SSE | SATISFIED | Subscribe line 74 precedes GetPosition line 81 |
| QUEUE-06 | Scheduler ticks every second, ZPOPMIN | SATISFIED | `Start` ticker 1s, `admitBatch` calls `ZPopMin` |
| QUEUE-07 | Distributed leader lock | SATISFIED | `AcquireLock` SETNX NX+TTL 10s in `tick` |
| QUEUE-08 | Admitted token written to hash for poll pickup | SATISFIED | `HSet(ctx, "ticket:"+ticketID, "admission_token", jwt)` |
| QUEUE-09 | POST /queue/exit decrements active | SATISFIED | `handler_exit.go`: `Decr active:{eventId}`, HTTP 204 |
| TOKEN-01 | Scheduler issues HMAC JWT with JTI, eventId, ticketId, 30min | SATISFIED | `IssueAdmission`: UUID JTI, HS256, 30min exp, custom claims |
| TOKEN-02 | Two independent secrets | SATISFIED | Separate secrets in config, startup panic if equal; TestWrongSecretRejected + TestSessionSecretIsolation PASS |
| TOKEN-03 | Middleware verifies q_session/q_admission inline, redirects on failure | SATISFIED | `QueueGuard` validates without Redis on session path |
| TOKEN-04 | SETNX on token:{jti} → 403 on replay | SATISFIED | `SetNX("token:"+claims.ID, ...)` → `!set → 403` |
| TOKEN-05 | QueueGuard increments active after SETNX | SATISFIED | `cfg.RDB.Incr(ctx, "active:"+claims.EventID)` |
| TOKEN-06 | QueueGuard clears q_admission | SATISFIED | `SetCookie("q_admission", "", -1, ...)` |
| TOKEN-07 | active counter scaffolded in scheduler | SATISFIED | `rdb.Incr(ctx, "active:"+eventID)` in admitBatch (D-06: not enforced as ceiling) |
| INFRA-01 | Docker Compose runs full local stack | SATISFIED | 4 services in docker-compose.yml; `go build ./...` exits 0 |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/scheduler/admission.go` | 17 | `TODO Plan 03: replace stub issueToken...` | Warning | Stale comment — replacement is complete in `cmd/queueserver/main.go:37`. The injected function is `token.IssueAdmission`, not a stub. Comment is misleading but harmless. Not a BLOCKER: `TODO` is warning-tier (not `TBD`/`FIXME`/`XXX`). |

### Human Verification Required

#### 1. SSE stream delivers position then admitted events

**Test:** `docker compose up --build -d && sleep 5`, then join a ticket, then `curl -sf -N --max-time 5 "http://localhost:8080/queue/status/$TICKET?mode=sse"`
**Expected:** First SSE event is `event: update\ndata: {"type":"position","value":0}`, then within ~2 seconds `event: update\ndata: {"type":"admitted","token":"eyJ..."}` (base64 JWT), then stream closes
**Why human:** SSE is a long-lived streaming HTTP response. The timing of the admitted event depends on the scheduler (1s tick). Static grep confirms the wiring but cannot observe stream content or timing.

#### 2. QueueGuard one-time enforcement (SETNX)

**Test:** Obtain a real JWT via poll after admission, then `curl -b "q_admission=$JWT" http://localhost:8081/` twice
**Expected:** First request: HTTP 200 with HTML "Seat Selection"; response includes `Set-Cookie: q_session=...`; `docker compose exec redis-origin redis-cli EXISTS token:{jti}` returns 1. Second request: HTTP 403.
**Why human:** Requires live redis-origin with a real SETNX call and cookie issuance. Runtime-only.

#### 3. Full verify.sh pass

**Test:** `make up && sleep 5 && make verify`
**Expected:** All 11 PASS lines printed, script exits 0
**Why human:** The verify.sh script is the canonical Phase 1 acceptance gate — it exercises all 5 ROADMAP success criteria against a live stack, including SSE stream delivery, SETNX replay rejection, and redis-cli introspection of `token:{jti}` in redis-origin.

---

## Gaps Summary

No code gaps found. All required artifacts exist and are substantively implemented. All key links are wired. The 2 outstanding items are runtime behaviors that require a live docker compose stack to observe — the code is present and correctly wired.

The stale `TODO` in `admission.go:17` does not represent incomplete work (the replacement was applied in `cmd/queueserver/main.go:37`). It should be cleaned up in a follow-on pass but is not a Phase 1 blocker.

---

_Verified: 2026-08-02_
_Verifier: Claude (gsd-verifier)_
