---
phase: 02
slug: frontend-admin-ui
status: verified
threats_open: 0
asvs_level: 1
created: 2026-08-29
---

# Phase 02 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Browser → queue API | Cross-origin browser requests | Queue status and admin mutations |
| URL/session storage → browser flow | User-controlled ticket and target values | Ticket ID and redirect URL |
| HTTP request → session validation | Untrusted `q_session` cookie | Signed session token |
| Browser inline script → exit API | Event ID embedded into checkout HTML | Event identifier |

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-02-01 | Spoofing | CORS allowlist | medium | mitigate | Exact origin allowlist in `internal/api/router.go` | closed |
| T-02-02 | Tampering | `q_admission` cookie | low | accept | Signed JWT; local-dev scope | closed — accepted risk |
| T-02-03 | Elevation | Target redirect | medium | mitigate | Browser navigation only; local-dev open redirect risk accepted | closed |
| T-02-04 | Information Disclosure | Ticket ID URL parameter | low | accept | Non-sensitive lookup identifier | closed — accepted risk |
| T-02-05 | Elevation | `GET /queue/events` | low | accept | No auth; Docker Compose local-dev scope | closed — accepted risk |
| T-02-06 | Tampering | `PUT /queue/rate` | low | accept | No auth; Docker Compose local-dev scope | closed — accepted risk |
| T-02-07 | Denial of Service | Redis event discovery | low | mitigate | Cursor-based Redis `SCAN` in `internal/api/handler_events.go` | closed |
| T-02-08 | Information Disclosure | Event selector | low | accept | Event IDs are operational identifiers, not secrets | closed — accepted risk |
| T-02-09 | Elevation | `q_session` validation | high | mitigate | HMAC session validation in middleware/token package | closed |
| T-02-10 | Tampering | Event ID inline JS injection | medium | mitigate | `fmt.Sprintf("%q", eventID)` JSON-safe quoting | closed |
| T-02-11 | Tampering | Capacity mutation | low | accept | Local-dev scope; production auth deferred | closed — accepted risk |
| T-02-12 | Denial of Service | Scheduler zero headroom | low | mitigate | Early return before `ZPOPMIN` when headroom is exhausted | closed |
| T-02-SC | Tampering | Dependency installation | low | accept | No new packages; existing Go dependencies and official nginx image | closed — accepted risk |

*Status: open · closed · open — below high threshold (non-blocking)*

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-02-01 | T-02-02 | Signed admission cookie; local development scope | Phase plan | 2026-08-29 |
| AR-02-02 | T-02-04 | Ticket ID is a non-sensitive lookup key | Phase plan | 2026-08-29 |
| AR-02-03 | T-02-05, T-02-06, T-02-11 | Admin endpoints intentionally unauthenticated for local Docker scope | Phase plan | 2026-08-29 |
| AR-02-04 | T-02-08 | Event IDs are operational identifiers, not secrets | Phase plan | 2026-08-29 |
| AR-02-05 | T-02-SC | No new package installation surface | Phase plan | 2026-08-29 |

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-29 | 13 | 13 | 0 | gsd-security-auditor |

## Sign-Off

- [x] All threats have a disposition
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-29
