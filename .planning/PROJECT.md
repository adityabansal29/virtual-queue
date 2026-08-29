# Virtual Queue System

## What This Is

A production-replica in-house virtual queue system for high-throughput event ticket sales (IPL-scale). It holds excess traffic in a virtual waiting room and admits users in FIFO order at a configurable rate, bounded by both admission rate and a hard concurrency ceiling. Queued users generate zero load on the protected origin.

## Core Value

A waiting user must always see an accurate position, and an admitted user must always be let through exactly once — no skips, no duplicates, no queue bypass.

## Requirements

### Validated

- ✓ Static queue waiting page with poll→SSE crossover and admission redirect — Phase 2
- ✓ Stub checkout with session validation and simulated seat selection — Phase 2
- ✓ Admin dashboard with live queue stats and rate/capacity controls — Phase 2

### Active

- [ ] Go queue API handles join, status (SSE + poll), exit, and admin endpoints
- [ ] Redis sorted-set queue with FIFO admission via ZPOPMIN
- [ ] Admission scheduler: gates on min(rate, capacity - active) per tick
- [ ] Hybrid SSE/polling transport with automatic crossover at configurable rank threshold
- [ ] Two-cookie token model: q_admission (one-time JWT) + q_session (ongoing signed cookie)
- [ ] SETNX one-time use enforcement + active count increment at origin
- [ ] Akamai EdgeWorker logic implemented as Go middleware (real Akamai deployment deferred)
- [ ] Full AWS infrastructure: ECS Fargate, ElastiCache Redis, S3, DynamoDB, SQS FIFO
- [ ] Local dev environment via Docker Compose (Redis, queue API, stub origin, admin UI)

### Out of Scope

- Real Akamai account / production EdgeWorker deployment — deferred until account available
- Lottery / pre-sale ballot mode — not FIFO, separate problem
- Bot resistance / CAPTCHA — referenced in design but not core to queue mechanics learning
- Load testing at 500k+ scale — functional correctness is the target, not stress validation
- Payment processing — stub origin simulates checkout without real payment

## Context

Design is fully specified in `DESIGN.md` (v3) at repo root. Key resolved decisions:
- Two independent secrets: `ADMISSION_SECRET` (admission JWT) and `SESSION_SECRET` (session cookie) — different blast radii, independent rotation
- SETNX at origin, not edge — ElastiCache not reachable from Akamai edge runtime
- SSE reserved for last ~200 positions; everyone further back polls every 5s — caps persistent connections at ~200/event regardless of total queue size
- `active:{eventId}` counter is a live concurrency gauge (up/down), not a running total — scheduler reads it every tick for headroom calculation
- ZRANK directly equals people-ahead — no separate admitted counter needed

## Constraints

- **Tech stack**: Go (queue service), Redis (ElastiCache), AWS (ECS Fargate, S3, DynamoDB, SQS), Akamai (edge, deferred)
- **EdgeWorker runtime**: No Akamai account yet — EdgeWorker JS written but tested via equivalent Go middleware locally
- **Scale target**: Functional correctness over load validation — design is already validated on paper for 500k+

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Akamai EdgeWorker as Go middleware locally | No account yet; same HMAC logic, different runtime | Validated in Phase 1 |
| Hybrid SSE/poll (threshold 200) | Caps persistent connections regardless of queue depth | Validated in Phase 2 |
| Two independent secrets | Different compromise blast radii; independent rotation | Validated in Phase 1 |
| SETNX at origin, not edge | Edge has no VPC/Redis access | Validated in Phase 1 |
| Docker Compose for local dev | Full stack runnable without AWS account | Validated in Phase 1/2 |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-08-29 after Phase 2*
