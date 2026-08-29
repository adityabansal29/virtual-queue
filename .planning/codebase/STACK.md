# Technology Stack

**Analysis Date:** 2026-08-29

## Languages

**Primary:**
- Go 1.25.0 - queue API, admission scheduler, stub checkout origin, Redis access, JWT handling, and HTTP middleware in `cmd/`, `internal/`, and `pkg/`.

**Secondary:**
- JavaScript (browser, no transpilation) - queue waiting page and admin dashboard in `web/queue/queue.js` and `web/admin/admin.js`.
- HTML/CSS - static queue/admin pages in `web/queue/` and `web/admin/`, plus the generated checkout page in `cmd/stuborigin/main.go`.
- POSIX/Bash shell - local verification in `scripts/verify.sh` and build orchestration in `Makefile`.

## Runtime

**Environment:**
- Go toolchain 1.25.0 - declared by `go.mod` and used by all Go binaries.
- Linux containers - multi-stage builds compile static binaries with `CGO_ENABLED=0` in `Dockerfile.queueserver`, `Dockerfile.scheduler`, and `Dockerfile.stuborigin`.
- Browser runtime - static pages use native `fetch`, `EventSource`, `URLSearchParams`, `sessionStorage`, and cookies; no frontend bundler is present.

**Package Manager:**
- Go modules - dependency resolution via `go.mod` and `go.sum`.
- Lockfile: `go.sum` present; no npm, pnpm, yarn, Python, or Cargo lockfile detected.

## Frameworks

**Core:**
- Gin v1.12.0 - HTTP routing, middleware, JSON responses, redirects, and request binding in `internal/api/router.go` and `cmd/stuborigin/main.go`.
- Go standard library - HTTP server runtime, context cancellation, JSON, URL handling, signals, timers, and `log/slog`.

**Testing:**
- Go `testing` package - tests such as `internal/token/jwt_test.go`.
- `github.com/stretchr/testify` v1.11.1 and related test modules are in the module graph, but no broad test harness or test configuration is detected.

**Build/Dev:**
- Docker Compose - local multi-service stack in `docker-compose.yml`.
- Docker multi-stage builds - service images in `Dockerfile.queueserver`, `Dockerfile.scheduler`, and `Dockerfile.stuborigin`.
- Nginx Alpine - static asset server in the `static-pages` Compose service, configured by `nginx.conf`.
- Make - common commands in `Makefile` (`up`, `down`, `logs`, `verify`, `test`, `build`).

## Key Dependencies

**Critical:**
- `github.com/redis/go-redis/v9` v9.21.0 - Redis client used for sorted-set queues, ticket hashes, counters, pub/sub, and scheduler locks in `internal/store/`, `internal/api/`, `internal/scheduler/`, and `pkg/middleware/`.
- `github.com/gin-gonic/gin` v1.12.0 - HTTP application framework used by the queue server and stub origin.
- `github.com/golang-jwt/jwt/v5` v5.3.1 - HMAC-SHA256 admission and session token issuance/validation in `internal/token/`.
- `github.com/google/uuid` v1.6.0 - ticket ID and JWT identifier generation in `internal/api/handler_join.go` and `internal/token/`.

**Infrastructure:**
- `redis:7-alpine` - two local Redis services, `redis-queue` and `redis-origin`, in `docker-compose.yml`.
- `nginx:alpine` - serves the read-only `web/` volume on port 8082 in `docker-compose.yml`.
- `golang:1.25-alpine` - build stage for each Go service Dockerfile.
- `gcr.io/distroless/static-debian12` - minimal runtime stage for each Go service image.

## Configuration

**Environment:**
- Environment variables are read directly with `os.Getenv` in `internal/config/config.go`.
- `ADMISSION_SECRET` and `SESSION_SECRET` are required by scheduler/origin startup and must differ; `.env.example` documents local values, while `.env` exists locally and is gitignored.
- Service settings include `REDIS_ADDR`, `PORT`, `SSE_THRESHOLD`, `DEFAULT_ADMIT_RATE`, `QUEUE_PAGE_URL`, `SCHEDULER_TICK_SECS`, `EVENT_ID`, `QUEUE_JOIN_URL`, and `SECURE`; defaults are defined in `internal/config/config.go`.
- Docker Compose injects `.env` with `env_file` and overrides `REDIS_ADDR` per service in `docker-compose.yml`.

**Build:**
- `go.mod` and `go.sum` - module and dependency metadata.
- `Dockerfile.queueserver`, `Dockerfile.scheduler`, and `Dockerfile.stuborigin` - reproducible container builds.
- `docker-compose.yml` - local topology, ports, healthchecks, dependencies, and volume mounts.
- `nginx.conf` - static-page routing and MIME configuration.
- `Makefile` - developer-facing build/test/verification entry points.

## Platform Requirements

**Development:**
- Go 1.25.0 for direct builds/tests, or Docker with Compose for the full stack; `scripts/verify.sh` assumes the Compose stack is running.
- Local ports 6379, 6380, 8080, 8081, and 8082 are used by the Compose services in `docker-compose.yml`.

**Production:**
- Container runtime capable of running the three statically compiled Go services and Redis; production AWS/Akamai architecture is documented in `DESIGN.md` but no deployment manifests or provider SDK wiring are present.

---

*Stack analysis: 2026-08-29*
