# Phase 1: Queue Core & Local Stack - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-02
**Phase:** 1-Queue Core & Local Stack
**Areas discussed:** Stub Ticket Origin, Go Module Root, Test Scope

---

## Stub Ticket Origin

| Option | Description | Selected |
|--------|-------------|----------|
| Separate cmd/stuborigin binary | Own main.go, runs as second Docker Compose service, imports pkg/middleware/queue_guard.go | ✓ |
| Embedded handlers in queue API | /stub/* route group inside queue API server | |
| Minimal curl-target only | Tiny HTTP server, no QueueGuard, SETNX tested in unit tests | |

**User's choice:** Separate binary, but stub should NOT access queue Redis directly. Suggested either a queue service endpoint for active count or skipping it.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Add /queue/validate-admission endpoint | Queue service does SETNX + active++, returns session token | |
| Defer active count to v2 | Active count and capacity ceiling skipped | |

**User's choice (freeform):** q_session issuance is the origin's responsibility (not queue service). Both services should have separate Redis. Either stub calls queue endpoint for inc/dec OR skip active count capping.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Stub calls POST /queue/active/inc & /queue/active/dec | Queue service owns active:{eventId} | |
| Skip active count capping in Phase 1 | Scheduler admits at configured rate, no headroom check | ✓ |

**User's choice:** Skip active count capping in Phase 1.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Two Redis containers (queue-redis + origin-redis) | Clean separation, each service connects to its own Redis | ✓ |
| One shared Redis, different key namespaces | Single container, key prefix separation | |

**User's choice:** Two Redis containers.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Simple HTML page | Returns HTML with seat selection confirmation | ✓ |
| JSON response | Returns JSON for curl-based verification | |

**User's choice:** Simple HTML page.

---

## Go Module Root

| Option | Description | Selected |
|--------|-------------|----------|
| Repo root | go.mod at repo root, standard Go project | ✓ |
| queue-service/ subdirectory | go.mod in subdir, self-contained module | |

**User's choice:** Repo root.

---

| Option | Description | Selected |
|--------|-------------|----------|
| github.com/zomato/virtual-queue | Matches org | |
| github.com/adityabansal29/virtual-queue | Personal GitHub username | ✓ |
| virtual-queue | Simple local name | |

**User's choice:** github.com/adityabansal29/virtual-queue.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Standard layout: cmd/ + internal/ + pkg/ | cmd/queueserver/, cmd/stuborigin/, internal/, pkg/middleware/ | ✓ |
| Flat layout: server/ + origin/ at root | Non-idiomatic | |

**User's choice:** Standard layout.

---

## Test Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Unit tests for token package only | Go tests in internal/token/ | ✓ (with follow-up) |
| Unit tests for token + scheduler logic | Adds scheduler tests with Redis | |
| Curl scripts only, no Go tests | Tests deferred | |

**User's choice:** Unit tests for token package. Follow-up: what about integration tests?

---

| Option | Description | Selected |
|--------|-------------|----------|
| No — Docker Compose is the integration test | docker compose up + curl script serves as integration test | ✓ |
| Yes — Go integration tests with testcontainers | Adds testcontainers-go dependency | |

**User's choice:** Docker Compose is the integration test. No testcontainers.

---

| Option | Description | Selected |
|--------|-------------|----------|
| scripts/verify.sh — executable test script | Runs all 5 success criteria, PASS/FAIL output | ✓ |
| Makefile targets | Individual targets per success criterion | |
| Manual curl commands in README | No automation | |

**User's choice:** scripts/verify.sh. Follow-up: how to verify internal module behavior?

---

| Option | Description | Selected |
|--------|-------------|----------|
| redis-cli checks alongside HTTP assertions | Script checks both HTTP responses AND Redis internal state | ✓ |
| HTTP assertions only | Only status codes and response body | |

**User's choice:** redis-cli introspection included in verify.sh.

---

## Claude's Discretion

- Config injection approach (os.Getenv vs dotenv) — not discussed
- Logging library choice — not discussed
- Docker Compose health checks and service startup ordering
- Docker image base (distroless vs alpine)

## Deferred Ideas

- Active count capping (`active:{eventId}` ceiling enforcement) — deferred, too complex for Phase 1
- Admin dashboard HTML UI (UI-06, UI-07) — Phase 2
- Static queue waiting page (UI-01 through UI-05) — Phase 2
- TTL sweep for abandoned sessions (OPS-03) — v2 backlog
