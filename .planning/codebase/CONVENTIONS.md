# Coding Conventions

**Analysis Date:** 2026-08-29

## Naming Patterns

**Files:**
- Go files use lowercase names such as `internal/api/handler_status.go`, `internal/token/jwt.go`, and `internal/scheduler/leader_lock.go`.
- Tests use the Go `_test.go` suffix, currently `internal/token/jwt_test.go`.
- Browser files are lowercase names grouped by feature: `web/queue/queue.js` and `web/admin/admin.js`.

**Functions:**
- Exported Go functions and methods use PascalCase and domain verbs: `QueueStatusPoll`, `IssueAdmission`, `GetPosition`, and `LoadScheduler`.
- Unexported Go helpers use camelCase: `doesTicketExist`, `createTicket`, `getEnvInt64`, and `queueErrorPage`.
- JavaScript functions use camelCase: `loadEvents`, `startPolling`, `renderStats`, and `handleAdmitted`.

**Variables:**
- Go locals and fields use short camelCase names (`ctx`, `rdb`, `cfg`, `ticketID`, `eventID`); initialisms remain capitalized in exported identifiers (`RDB`, `JWT`, `SSE`, `ID`).
- JavaScript uses camelCase and descriptive DOM-oriented names such as `pollTimer`, `currentEventId`, `lastStats`, and `headroomEl`.
- JSON keys follow the existing API contract, including lower camel case (`eventId`, `ticketId`, `queueDepth`, `estimatedDrainSec`) and explicit snake case for `upgrade_to_sse`.

**Types:**
- Go types use PascalCase nouns: `Handler`, `Scheduler`, `QueueServerConfig`, `AdmissionClaims`, and `SessionClaims`.
- Request payloads used once are anonymous structs with JSON tags, as in `internal/api/handler_admin.go` and `internal/api/handler_exit.go`.
- Configuration is split into service-specific structs in `internal/config/config.go`.

## Code Style

**Formatting:**
- Use `gofmt` for Go files; imports are grouped into standard library, third-party, and local project groups, as shown in `internal/api/handler_join.go`.
- No repository-specific formatter configuration is present. `Makefile` has no format target.
- JavaScript uses four-space indentation, semicolons, single quotes for ordinary strings, and template literals for interpolated URLs, as shown in `web/admin/admin.js`.

**Linting:**
- No ESLint, Prettier, Biome, or Go lint configuration is present.
- Deliberately ignored errors use `//nolint:errcheck`, for example in `internal/api/handler_join.go` and `internal/scheduler/admission.go`.
- Preserve explicit error checks at request, token, Redis, and JSON boundaries; document any additional ignored error.

## Import Organization

**Order:**
1. Standard library imports.
2. External dependencies such as `github.com/gin-gonic/gin` and `github.com/redis/go-redis/v9`.
3. Local module imports under `github.com/adityabansal29/virtual-queue/...`.

**Path Aliases:**
- No path aliases are used. Import the full Go module path from `go.mod`.
- Use `applog` when importing `pkg/log` to distinguish it from the standard logger, as in `internal/api/handler_status.go`.

## Error Handling

**Patterns:**
- HTTP handlers return immediately after writing errors with `c.JSON`, `c.AbortWithStatus`, or `c.Data`; follow `internal/api/handler_join.go` and `pkg/middleware/queue_guard.go`.
- Validate required request/query fields at the boundary and return `400` (`internal/api/handler_join.go`, `internal/api/handler_exit.go`).
- Map missing tickets to `404`, unavailable Redis operations to `500`/`503`, and invalid or replayed admission tokens to `401`/`403` according to existing handlers.
- Lower-level helpers return `(value, error)` and let callers choose HTTP or logging behavior, as in `internal/store/redis.go` and `internal/token/jwt.go`.
- Redis calls that are deliberately non-fatal are documented inline, for example ticket metadata writes in `internal/api/handler_join.go`.
- Startup-only missing secrets fail fast with `panic` in `internal/config/config.go`.

## Logging

**Framework:** `log/slog`, wrapped by `pkg/log/log.go`.

**Patterns:**
- Use `applog.InfoWithContext`, `ErrorWithContext`, `WarnWithContext`, and `DebugWithContext` for request or scheduler work.
- Include a stable operation message and structured fields (`eventId`, `ticketId`, `error`), as in `internal/scheduler/admission.go`.
- Startup entrypoints in `cmd/queueserver/main.go` and `cmd/scheduler/main.go` use `slog` directly.
- Never log secret values; `cmd/scheduler/main.go` records only that `admission_secret` is set.

## Comments

**When to Comment:**
- Comment exported Go functions and types with a sentence beginning with the identifier, as in `internal/store/redis.go` and `internal/token/session.go`.
- Explain protocol decisions, race avoidance, API contracts, and deliberate tradeoffs; see `internal/api/handler_status.go`.
- Preserve `ponytail:` comments when a deliberate simplification has a known ceiling or follow-up trigger, such as the Redis scan in `internal/scheduler/admission.go`.

**JSDoc/TSDoc:**
- No JSDoc or TSDoc is used. JavaScript behavior uses short inline comments tied to UI contracts or design decisions in `web/queue/queue.js` and `web/admin/admin.js`.

## Function Design

**Size:**
- Keep handlers and orchestration functions focused on one request or scheduler operation; current functions are generally below 150 lines, with `internal/api/handler_status.go` containing the largest handler logic.

**Parameters:**
- Pass request-scoped `context.Context` explicitly through storage and scheduler calls.
- Inject external dependencies through constructors or function fields: `NewHandler` in `internal/api/handler_join.go` and `NewScheduler` in `internal/scheduler/admission.go`.
- Use configuration structs when middleware or a service needs several settings (`pkg/middleware/queue_guard.go`).

**Return Values:**
- Return `(value, error)` from operations that can fail, and check the error immediately.
- Return documented sentinel values where useful, such as `GetPosition` returning `-1, nil` for an absent member in `internal/store/redis.go`.
- HTTP handlers write responses directly and do not return response objects.

## Module Design

**Exports:**
- Keep package boundaries domain-oriented: API in `internal/api`, Redis/key access in `internal/store`, tokens in `internal/token`, scheduling in `internal/scheduler`, and middleware in `pkg/middleware`.
- Export only neighboring-package APIs; keep helpers such as `createTicket` and `queueErrorPage` private.

**Barrel Files:**
- Not applicable. Go packages expose declarations directly; no barrel/index modules are used.

---

*Convention analysis: 2026-08-29*
