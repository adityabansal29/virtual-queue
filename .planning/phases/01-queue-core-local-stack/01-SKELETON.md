# Walking Skeleton — Virtual Queue System

**Phase:** 1
**Generated:** 2026-08-02

## Capability Proven End-to-End

A curl client can join the virtual queue (`POST /queue/join`) and receive a unique ticketId — the request flows through the Go queue API, performs a Redis ZADD, persists the ticket hash, and returns a JSON response — with the full Docker Compose stack (`redis-queue`, `queueserver`) running from a single `docker compose up`.

## Architectural Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Language / runtime | Go 1.22, module at repo root | Stack constraint (DESIGN.md Section 10). Go's goroutine model is required for SSE handlers and the admission scheduler goroutine. |
| HTTP framework | Gin (`github.com/gin-gonic/gin`) | Idiomatic Go HTTP with middleware support; `c.Writer.(http.Flusher)` SSE pattern in DESIGN.md Section 7 is Gin-native. |
| Queue store | Redis Sorted Set via `github.com/redis/go-redis/v9` | ZADD/ZPOPMIN/ZRANK are the atomic primitives the entire queue mechanic rests on (DESIGN.md Section 9 data model). |
| Token format | HMAC-SHA256 JWT via `github.com/golang-jwt/jwt/v5` | Two-cookie model needs JTI + claims; golang-jwt is the canonical Go JWT library. |
| Module name | `github.com/adityabansal29/virtual-queue` | D-07: one-way decision recorded here as the canonical reference for all import paths. |
| Directory layout | `cmd/queueserver/`, `cmd/stuborigin/`, `internal/{api,scheduler,token,store,config}/`, `pkg/middleware/` | D-08: standard Go layout separating two binaries and shared middleware. |
| Local dev runtime | Docker Compose with two Redis containers (`redis-queue`, `redis-origin`) | D-04: two separate Redis instances enforce the service boundary — queue API and stub origin cannot accidentally share state. |
| Config injection | `os.Getenv` with `.env` loaded by Docker Compose `env_file` | No dotenv library needed — Docker Compose handles env var injection; Go reads them with stdlib `os`. |
| Logging | `log/slog` (stdlib, Go 1.21+) | No external dependency; structured JSON output on a single `slog.Logger` instance. |

## Stack Touched in Phase 1

- [x] Project scaffold — `go.mod`, `go.sum`, Dockerfile(s), Docker Compose, Makefile
- [x] Routing — `POST /queue/join`, `GET /queue/status/:ticketId`, `POST /queue/exit`, `PUT /queue/rate/:eventId`, `GET /queue/config/:eventId`
- [x] Queue store — Redis ZADD (join), ZRANK (poll position), ZPOPMIN (admit), HSET (ticket hash), SETNX (one-time token)
- [x] Scheduler — admission ticker, leader lock (Redis SETNX NX+TTL), pub/sub publish on admission
- [x] Token model — HMAC JWT issue (q_admission), session signed cookie (q_session), unit tests
- [x] Stub origin binary — QueueGuard middleware: SETNX enforcement, active++ increment, q_session issuance
- [x] Deployment — `docker compose up` starts full local stack; `scripts/verify.sh` exercises all 5 success criteria

## Out of Scope (Deferred to Later Slices)

- Active count capping (`active:{eventId}` ceiling enforcement) — D-06: scaffolded but NOT enforced in Phase 1. Scheduler admits at configured rate with `headroom = ∞`.
- Admin dashboard browser UI (UI-06, UI-07) — Phase 2
- Static queue waiting page HTML/JS (UI-01 through UI-05) — Phase 2
- Real Akamai EdgeWorker deployment — deferred until account available
- TTL-sweep background job for abandoned sessions (OPS-03) — v2 backlog
- DynamoDB audit tables, SQS FIFO, ECS Fargate, ElastiCache — Phase 3

## Subsequent Slice Plan

Each later phase adds one vertical slice on top of this skeleton without altering its architectural decisions:

- Phase 2: Browser-accessible waiting room — static HTML/JS queue page (poll → SSE crossover → admission redirect) + admin dashboard + stub ticket checkout
- Phase 3: Full AWS deployment — ECS Fargate, ElastiCache Redis, S3/CloudFront, DynamoDB, SQS FIFO
