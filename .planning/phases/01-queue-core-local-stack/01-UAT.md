---
status: testing
phase: 01-queue-core-local-stack
source: [01-VERIFICATION.md]
started: 2026-08-02T00:00:00Z
updated: 2026-08-02T00:00:00Z
---

## Current Test

number: 1
name: Run scripts/verify.sh against live stack
expected: |
  make up && make verify exits 0 with all PASS lines
awaiting: user response

## Tests

### 1. Run scripts/verify.sh (canonical gate)

expected: |
  `make up && make verify` — all 11 PASS lines printed, exit code 0.
  This exercises all 5 Phase 1 success criteria against the live stack.
result: [pending]

### 2. SSE stream delivers position then admitted event (Criterion 3)

expected: |
  After joining and waiting for the scheduler tick,
  `curl -N --max-time 5 "http://localhost:8080/queue/status/<ticketId>?mode=sse"`
  returns a position event followed by an admitted event with a JWT payload (starts with eyJ).
result: [pending]

### 3. QueueGuard one-time enforcement (Criterion 4)

expected: |
  With a valid q_admission cookie set:
  - First request to stuborigin (port 8081) returns HTTP 200 + HTML "Seat selection" page
  - Second identical request returns HTTP 403
  - `docker compose exec redis-origin redis-cli EXISTS token:<jti>` returns 1
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
