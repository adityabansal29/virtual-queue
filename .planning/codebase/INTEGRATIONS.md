# External Integrations

**Analysis Date:** 2026-08-29

## APIs & External Services

**Queue HTTP API:**
- Queue server - exposes join, status, SSE, exit, admin configuration, events, and health endpoints from `internal/api/router.go`.
  - SDK/Client: Gin v1.12.0 (`github.com/gin-gonic/gin`) and Go `net/http`.
  - Auth: no queue-server authentication layer; token signing/validation is handled by scheduler/origin paths.

**Browser transport:**
- Native browser `fetch` - calls queue and admin HTTP endpoints in `web/queue/queue.js`, `web/admin/admin.js`, and generated checkout JavaScript in `cmd/stuborigin/main.go`.
- Native browser `EventSource` - consumes `/queue/status/:ticketId?mode=sse` in `web/queue/queue.js`.
- No third-party API client, outbound HTTP integration, webhook client, CAPTCHA, or payment provider is implemented.

## Data Storage

**Databases:**
- Redis 7 - primary queue and token state store.
  - Connection: `REDIS_ADDR`, loaded in `internal/config/config.go` and supplied by `docker-compose.yml`.
  - Client: `github.com/redis/go-redis/v9` v9.21.0, constructed in `internal/store/redis.go`.
  - Queue instance: `redis-queue` stores `queue:{eventId}` sorted sets, ticket hashes, rate/capacity counters, pub/sub channels, and scheduler locks for `queueserver` and `scheduler`.
  - Origin instance: `redis-origin` stores one-time admission markers used by `pkg/middleware/queue_guard.go` and is used by `stuborigin`.
  - Operations include ZADD/ZRANK/ZPOPMIN, hashes, TTLs, GET/INCR/DECR, SCAN, SETNX, and pub/sub across `internal/store/`, `internal/api/`, `internal/scheduler/`, and `pkg/middleware/`.
- MongoDB driver `go.mongodb.org/mongo-driver/v2` v2.5.0 is present in the module graph but no MongoDB client or usage is detected.

**File Storage:**
- Local read-only filesystem volume only - `./web` is mounted into Nginx at `/usr/share/nginx/html` by `docker-compose.yml`.
- S3 is described as a production design target in `DESIGN.md`, but no S3 SDK, bucket configuration, or upload path is implemented.

**Caching:**
- None separate from Redis data structures and Redis TTLs defined in `internal/config/config.go`.

## Authentication & Identity

**Auth Provider:**
- Custom HMAC JWT - `internal/token/jwt.go` issues and validates `q_admission` tokens using `ADMISSION_SECRET`; `internal/token/session.go` handles `q_session` tokens using `SESSION_SECRET`.
  - Admission tokens are issued by the scheduler and delivered through polling or Redis-backed SSE in `internal/scheduler/admission.go` and `internal/api/handler_status.go`.
  - `pkg/middleware/queue_guard.go` validates tokens, enforces one-time use with Redis SETNX, then replaces the admission cookie with a session cookie.
  - No OAuth, OIDC, Auth0, Cognito, external identity provider, user account system, or login flow is implemented.

## Monitoring & Observability

**Error Tracking:**
- None detected; no Sentry, Datadog, OpenTelemetry, or equivalent integration is configured.

**Logs:**
- Go standard-library `log/slog`, exposed through context-aware wrappers in `pkg/log/log.go` and used by service entrypoints and scheduler/store code.
- Docker captures process stdout/stderr; no log shipper or metrics exporter is configured.
- `/health` in `internal/api/router.go` is a lightweight HTTP liveness response and does not probe Redis.

## CI/CD & Deployment

**Hosting:**
- Local Docker Compose is the only executable deployment topology in `docker-compose.yml`.
- `DESIGN.md` specifies AWS ECS Fargate, ElastiCache Redis, S3, Akamai CDN/EdgeWorkers, and CloudWatch/SNS/PagerDuty as intended production architecture; no Terraform, ECS, Kubernetes, AWS SDK, or Akamai EdgeWorker source is present.

**CI Pipeline:**
- None detected. `Makefile` and `scripts/verify.sh` provide local build, unit-test, and running-stack verification commands.

## Environment Configuration

**Required env vars:**
- `ADMISSION_SECRET` - required by `internal/config/config.go` for scheduler and stub origin; signs admission JWTs.
- `SESSION_SECRET` - required by `internal/config/config.go` for stub origin; signs session JWTs and must differ from the admission secret.
- `REDIS_ADDR` - connection address; Compose overrides it to the appropriate Redis service for each backend.
- Optional runtime controls: `PORT`, `SSE_THRESHOLD`, `DEFAULT_ADMIT_RATE`, `QUEUE_PAGE_URL`, `SCHEDULER_TICK_SECS`, `EVENT_ID`, `QUEUE_JOIN_URL`, and `SECURE`.

**Secrets location:**
- `.env` is present locally and gitignored; its contents are intentionally not documented.
- `.env.example` documents variable names and non-production example values only.
- No cloud secret-manager integration is implemented.

## Webhooks & Callbacks

**Incoming:**
- None. Redis pub/sub is an internal event channel, not an external webhook; scheduler events use `ticket:updates:{ticketId}` and `queue:tick:{eventId}` in `internal/scheduler/admission.go` and `internal/api/handler_status.go`.

**Outgoing:**
- None. The browser makes direct HTTP requests to the local queue API; no third-party callback or webhook delivery exists.

---

*Integration audit: 2026-08-29*
