---
phase: 01-queue-core-local-stack
plan: 02
subsystem: queue-core
status: complete
tags: [go, redis, sse, polling, scheduler, pub-sub]
completed_date: "2026-08-02"
duration_seconds: 645

dependency_graph:
  requires:
    - 01-01 (go module, Handler struct, Redis client, router scaffold)
  provides:
    - store.GetPosition (Lua atomic ZRANK)
    - store.EventIDFromTicket (HGet ticket hash)
    - api.QueueStatusPoll (GET /queue/status/:ticketId?mode=poll)
    - api.QueueStatusSSE (GET /queue/status/:ticketId?mode=sse)
    - api.QueueExit (POST /queue/exit)
    - scheduler.Scheduler (1s ticker, ZPOPMIN, pub/sub publish)
    - scheduler.AcquireLock / ReleaseLock (Redis SETNX NX+TTL 10s)
  affects:
    - 01-03 (must wire scheduler.NewScheduler in main.go with issueToken)

tech_stack:
  added: []
  patterns:
    - Redis Lua eval for atomic ZRANK (GetPosition)
    - Hybrid SSE/poll dispatch via ?mode= query param
    - pub/sub subscribe-before-read pattern (QUEUE-05 race prevention)
    - SSE heartbeat ticker every 15s (keep-alive for proxies)
    - Distributed leader lock via Redis SETNX NX+TTL 10s (QUEUE-07)
    - Injected issueToken function on Scheduler for Plan 03 wire-in

key_files:
  created:
    - internal/api/handler_status.go
    - internal/api/handler_exit.go
    - internal/scheduler/admission.go
    - internal/scheduler/leader_lock.go
  modified:
    - internal/store/redis.go (GetPosition + EventIDFromTicket added)
    - internal/api/router.go (status dispatcher + exit handler wired)

decisions:
  - "QUEUE-05: pub/sub Subscribe call is placed before GetPosition call in QueueStatusSSE — code order enforces the invariant"
  - "D-06 enforced: n := rate with comment marking Phase 2 upgrade point for headroom = min(rate, capacity-active)"
  - "issueToken injected via NewScheduler constructor — Plan 03 replaces stub without touching scheduler package"
  - "main.go not modified — per parallel execution coordination note; Plan 01-03 owns main.go wiring"
  - "map[string]string used for pub/sub unmarshal (all admitted payload values are strings — matches DESIGN.md Section 7 exactly)"

metrics:
  duration_seconds: 645
  completed_date: "2026-08-02"
  tasks: 3
  commits: 2
  files_changed: 6

actuals:
  tokens: 25000
  tasks: 3
  commits: 2
---

# Phase 01 Plan 02: Queue Mechanics Summary

Poll handler, SSE handler with QUEUE-05 race-prevention subscribe order, admission scheduler with leader lock, and queue exit — complete queue mechanics on top of the Plan 01 skeleton.

## What Was Built

Two commits implement the full queue mechanics layer:

1. **Status handlers** (0ab5803): store.GetPosition (Lua), store.EventIDFromTicket, QueueStatusPoll, QueueStatusSSE, router dispatch
2. **Scheduler + exit** (6228816): scheduler.AcquireLock/ReleaseLock, scheduler.Scheduler (tick + admitBatch), api.QueueExit, router wiring

### Key Artifacts

| Artifact | Description |
|----------|-------------|
| `internal/store/redis.go` | +GetPosition (Lua atomic ZRANK, returns -1 if absent), +EventIDFromTicket (HGet ticket hash) |
| `internal/api/handler_status.go` | QueueStatusPoll (check admission_token, read-once delete, rank/pending), QueueStatusSSE (subscribe before read, heartbeat 15s, admitted closes stream) |
| `internal/api/handler_exit.go` | QueueExit: parse eventId, Decr active:{eventId}, return 204 |
| `internal/api/router.go` | GET /queue/status dispatch on ?mode=sse vs poll; POST /queue/exit wired |
| `internal/scheduler/leader_lock.go` | AcquireLock (SETNX NX+TTL 10s), ReleaseLock (DEL) |
| `internal/scheduler/admission.go` | Scheduler struct with injected issueToken, Start (1s ticker), tick (lock acquire, rate read, admitBatch), admitBatch (ZPOPMIN, HSet admission_token, Incr active, Publish ticket:updates + queue:tick) |

### Endpoints Delivered

| Endpoint | Status |
|----------|--------|
| GET /queue/status/:ticketId?mode=poll | Implemented |
| GET /queue/status/:ticketId?mode=sse | Implemented |
| POST /queue/exit | Implemented |

## Verification Results

End-to-end verified against live docker compose stack (worktree build):

- `GET /queue/status/:id?mode=poll` returns `{"type":"position","value":0,"upgrade_to_sse":true}` — PASS
- `GET /queue/status/:id?mode=sse` returns `Content-Type: text/event-stream` — PASS
- SSE initial event: `event: update\ndata: {"type":"position","value":0}` delivered within 1s — PASS
- SSE admitted event: simulated scheduler publish to `ticket:updates:{id}` delivers `event: update\ndata: {"token":"STUB-JWT-test-token","type":"admitted"}` and closes stream — PASS
- SSE tick handling: `queue:tick:evt-001` publish triggers rank re-read and delivers updated position event — PASS
- `GET /queue/status/nonexistent?mode=poll` returns HTTP 404 — PASS
- `POST /queue/exit {"eventId":"evt-001"}` returns HTTP 204 — PASS

**Note on scheduler integration test:** Scheduler is not started in main.go yet — that is Plan 01-03's responsibility per parallel execution coordination. The full admission flow (scheduler logs tick, active:{eventId} increments automatically) will be verified after Plan 01-03 merges and wires main.go.

## Deviations from Plan

### Structural Deviations

**1. main.go not modified — per parallel execution note**
- **Reason:** Parallel execution coordination note explicitly states "Your plan should NOT touch cmd/queueserver/main.go — Plan 01-03 will add a stub issueToken function there."
- **Impact:** Scheduler package is fully implemented but not started until Plan 01-03 wires `scheduler.NewScheduler(...)` and `go sched.Start(ctx)` in main.go.
- **Resolution:** The `issueToken func(ticketID, eventID string) (string, error)` constructor parameter is the exact interface Plan 01-03 will call.

### Auto-fixed Issues

None — plan executed without bugs or deviations requiring fixes.

### Task 3 Audit Result

No code changes needed. The SSE handler uses `map[string]string` for pub/sub unmarshal (correct — all admitted payload values are strings), switches on `ev["type"] == "admitted"`, and sends `ev["token"]`. The tick case correctly continues the loop when rank < 0 without closing the stream. Wire format matches DESIGN.md Section 7 exactly.

## Known Stubs

| Stub | File | Reason |
|------|------|--------|
| Scheduler not wired in main.go | cmd/queueserver/main.go | Plan 01-02 owns scheduler package; Plan 01-03 owns main.go wiring per parallel execution note |
| issueToken will return stub JWT | Via Plan 01-03 wiring | Plan 01-03 replaces with token.IssueAdmission when internal/token is implemented |

## Threat Surface Scan

All T-02-* mitigations from the plan's threat model are implemented:
- T-02-01: EventIDFromTicket returns 404 for unknown ticketIds — DONE
- T-02-02: SSE restricted to near-front users via upgrade_to_sse hint (rank < SSEThreshold=200) — DONE
- T-02-03: Leader lock has 10s TTL, NX semantics — DONE
- T-02-04: pub/sub channel namespaced per-ticket UUID — DONE
- T-02-05: D-06 active ceiling deferred by design — accepted
- T-02-SC: Scheduler.Start uses ctx from caller; Plan 01-03 will use signal.NotifyContext for SIGTERM/SIGINT — scheduler loop stops cleanly on ctx.Done()

## Self-Check: PASSED

Files created:
- internal/api/handler_status.go: FOUND
- internal/api/handler_exit.go: FOUND
- internal/scheduler/admission.go: FOUND
- internal/scheduler/leader_lock.go: FOUND

Files modified:
- internal/store/redis.go: FOUND
- internal/api/router.go: FOUND

Commits verified:
- 0ab5803 (status handlers): FOUND
- 6228816 (scheduler + exit): FOUND
