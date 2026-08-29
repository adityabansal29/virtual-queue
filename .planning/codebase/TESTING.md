# Testing Patterns

**Analysis Date:** 2026-08-29

## Test Framework

**Runner:**
- Go's standard `testing` package, with tests in `internal/token/jwt_test.go`.
- Config: No `go test` configuration file; module and Go version are declared in `go.mod`.

**Assertion Library:**
- Standard `testing.T` methods only: `Fatalf`, `Fatal`, and `Error`.
- No third-party assertion or mocking library is present.

**Run Commands:**
```bash
go test ./...          # Run all Go tests
go test ./internal/token # Run the existing token tests
go build ./...        # Compile all packages
./scripts/verify.sh   # Run Docker-backed end-to-end verification
```

No watch-mode or coverage command is defined in `Makefile`; convenience targets are `make test`, `make build`, and `make verify`.

## Test File Organization

**Location:**
- Tests are co-located with the package under test. The only test file is `internal/token/jwt_test.go`, beside `internal/token/jwt.go` and `internal/token/session.go`.
- No `web/` JavaScript tests, API handler tests, scheduler tests, storage tests, or middleware tests are present.

**Naming:**
- Test files use the standard `_test.go` suffix.
- Test functions use `Test` plus a PascalCase behavior name: `TestIssueAndVerify`, `TestWrongSecretRejected`, `TestExpiredToken`, `TestJTIUniqueness`, and `TestSessionSecretIsolation` in `internal/token/jwt_test.go`.

**Structure:**
```text
internal/token/
├── jwt.go
├── jwt_test.go
└── session.go
```

## Test Structure

**Suite Organization:**
```go
func TestWrongSecretRejected(t *testing.T) {
	tok, err := IssueAdmission("ticket-abc", "evt-001", "secret-A")
	if err != nil {
		t.Fatalf("IssueAdmission error: %v", err)
	}
	_, err = ValidateJWT(tok, "secret-B")
	if err == nil {
		t.Fatal("expected error when verifying with wrong secret, got nil")
	}
}
```

**Patterns:**
- Each test exercises one token behavior with direct function calls and local literal inputs in `internal/token/jwt_test.go`.
- Fail setup errors immediately with `t.Fatalf`; use `t.Error`/`t.Errorf` for non-fatal assertion failures.
- Tests cover matching secret, wrong secret, expiry, JTI uniqueness, and admission/session secret isolation.
- Tests do not use `t.Run`, table-driven cases, package fixtures, setup hooks, teardown hooks, or parallel execution.

## Mocking

**Framework:**
- No mocking framework or fake Redis implementation is present.

**Patterns:**
- Token tests avoid mocks because `IssueAdmission`, `ValidateJWT`, and `ValidateSession` operate over in-memory strings and claims.
- The scheduler has an injectable `issueToken` function in `internal/scheduler/admission.go`; use this seam for scheduler unit tests.
- HTTP and Redis-dependent code has no test doubles or handler test harness; if added, use `httptest` and a narrowly scoped Redis test dependency.

**What to Mock:**
- Mock or inject external Redis behavior when testing `internal/api`, `internal/store`, `internal/scheduler`, or `pkg/middleware` without a live service.
- Keep token signing and validation real in token-focused tests; existing tests treat the JWT implementation as the unit under test.

**What NOT to Mock:**
- Do not mock simple local calculations, claim construction, or standard-library parsing in `internal/token/jwt_test.go`.
- Do not replace the end-to-end queue flow with only mocks; preserve `scripts/verify.sh` for Docker/Redis, HTTP, SSE, cookie, and replay behavior.

## Fixtures and Factories

**Test Data:**
```go
claims := &AdmissionClaims{
	EventID:  "evt-001",
	TicketID: "ticket-abc",
	RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "ticket-abc",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Second)),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
		ID:        "test-jti",
	},
}
```

**Location:**
- Fixtures are declared inline inside tests in `internal/token/jwt_test.go`.
- No shared fixture directory, factory helpers, golden files, or generated test data are present.

## Coverage

**Requirements:**
- No coverage target or enforcement is configured.
- Existing automated unit coverage is limited to token issuance/validation in `internal/token/jwt_test.go`; other Go packages and browser code have no repository test files.

**View Coverage:**
```bash
go test ./... -cover
go test ./internal/token -coverprofile=coverage.out
go tool cover -func=coverage.out
```

These are standard Go tooling commands; no coverage target is wired into `Makefile` or detected CI files.

## Test Types

**Unit Tests:**
- Present for JWT/session token behavior in `internal/token/jwt_test.go`, using direct calls and no external services.
- Add focused tests beside configuration parsing (`internal/config/config.go`), key construction (`internal/store/keys.go`), scheduler decisions (`internal/scheduler/admission.go`), and middleware branches (`pkg/middleware/queue_guard.go`) using existing dependency seams.

**Integration Tests:**
- No Go integration test files are present.
- `scripts/verify.sh` is the integration/system check: it calls live queue and origin HTTP endpoints, validates polling and SSE, checks one-time token replay, and introspects Redis. It assumes `docker compose` is already running.

**E2E Tests:**
- No browser automation framework is used.
- Screenshots under `test-screenshots/` are manual/UI verification artifacts, not executable tests.

## Common Patterns

**Async Testing:**
```go
// No asynchronous Go test helper is currently used.
// Service lifecycle and SSE timing are exercised by scripts/verify.sh.
```

For future asynchronous tests, use context cancellation and bounded timeouts, matching production lifecycle patterns in `internal/scheduler/admission.go` and `internal/api/handler_status.go`; avoid unbounded waits.

**Error Testing:**
```go
_, err = ValidateJWT(tok, "secret-B")
if err == nil {
	t.Fatal("expected error when verifying with wrong secret, got nil")
}
```

Assert error conditions directly and include the expected behavior in the failure message. Preserve security-negative cases alongside happy paths, as in `internal/token/jwt_test.go` and the wrong-secret/replay checks in `scripts/verify.sh`.

---

*Testing analysis: 2026-08-29*
