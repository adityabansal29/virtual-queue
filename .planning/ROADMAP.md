# Roadmap: Virtual Queue System

## Overview

Three phases deliver a complete, curl-testable queue API (with token model and local Docker stack), then a static frontend with admin UI and stub checkout, then the full AWS infrastructure deployment. Each phase is independently runnable and verifiable before the next begins.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Queue Core & Local Stack** - Go queue API, Redis sorted-set queue, admission scheduler, two-cookie token model, and Docker Compose local dev environment (completed 2026-08-02)
- [ ] **Phase 2: Frontend & Admin UI** - Static queue waiting page with hybrid poll/SSE crossover, admin dashboard, and stub ticket checkout
- [ ] **Phase 3: AWS Infrastructure** - ECS Fargate, ElastiCache Redis, S3/CloudFront, DynamoDB, and SQS FIFO deployment

## Phase Details

### Phase 1: Queue Core & Local Stack

**Goal**: The full queue mechanics — join, position tracking, admission, and one-time token enforcement — are runnable locally via Docker Compose and verifiable end-to-end without a browser.
**Mode:** mvp
**Depends on**: Nothing (first phase)
**Requirements**: QUEUE-01, QUEUE-02, QUEUE-03, QUEUE-04, QUEUE-05, QUEUE-06, QUEUE-07, QUEUE-08, QUEUE-09, TOKEN-01, TOKEN-02, TOKEN-03, TOKEN-04, TOKEN-05, TOKEN-06, TOKEN-07, INFRA-01
**Success Criteria** (what must be TRUE):

  1. `docker compose up` starts Redis and the queue API with no manual steps; `curl POST /queue/join` returns a ticketId
  2. `curl GET /queue/status/:id?mode=poll` returns position (rank) and `upgrade_to_sse: true` hint when rank < 200
  3. `curl GET /queue/status/:id?mode=sse` opens an SSE stream that delivers a position event and later an admitted event when the scheduler pops that ticket
  4. Sending a valid q_admission JWT to the QueueGuard middleware succeeds once (SETNX succeeds, q_session issued) and returns 403 on the identical second request
  5. `docker compose up` starts without missing env vars; ADMISSION_SECRET and SESSION_SECRET are two distinct values and the system rejects tokens signed with the wrong secret

**Plans**: 4/4 plans executed

Plans:

- [x] 01-01-PLAN.md — [Wave 1] Walking skeleton: module scaffold, Docker Compose, POST /queue/join end-to-end tracer
- [x] 01-02-PLAN.md — [Wave 2, parallel with 01-03] Queue mechanics: poll handler, SSE handler, admission scheduler, leader lock, queue exit
- [x] 01-03-PLAN.md — [Wave 2, parallel with 01-02] Token model: JWT issue/verify, QueueGuard middleware, stub origin, unit tests
- [x] 01-04-PLAN.md — [Wave 3, after 01-02 + 01-03] Admin endpoints, verification script (scripts/verify.sh)

### Phase 2: Frontend & Admin UI

**Goal**: A browser-accessible waiting room experience — the static queue page polls, upgrades to SSE near the front, redirects on admission, and the admin dashboard lets an operator adjust rate and capacity live.
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: UI-01, UI-02, UI-03, UI-04, UI-05, UI-06, UI-07, UI-08
**Success Criteria** (what must be TRUE):

  1. Opening the queue waiting page in a browser shows "X people ahead" and an estimated wait time, without reloading when position updates arrive
  2. When rank drops below 200 the browser switches from polling to SSE automatically (no page reload); the network tab shows the EventSource connection open
  3. On admission the browser sets the q_admission cookie and redirects to the stub checkout page without manual intervention
  4. The stub checkout page validates the q_session cookie and displays the simulated seat selection; a missing or expired cookie shows an error
  5. The admin dashboard displays live queue depth, active users, admit rate, capacity, and headroom; changing admit rate or capacity via the dashboard takes effect on the next scheduler tick

**Plans**: 3/3 plans executed

Plans:
**Wave 1**

- [x] 02-01-PLAN.md — [Wave 1, tracer] Queue waiting page end-to-end: nginx service, CORS, queue.js poll/SSE/admission

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 02-02-PLAN.md — [Wave 2, parallel] Admin dashboard: GET /queue/events, event selector, live stats, rate/capacity update
- [x] 02-03-PLAN.md — [Wave 2, parallel] Stub checkout + capacity enforcement: constrained flag, session validation, seat grid

**UI hint**: yes

### Phase 3: AWS Infrastructure

**Goal**: The full system runs on AWS — queue API on ECS Fargate, Redis on ElastiCache, static page on S3 + CloudFront, audit data in DynamoDB, and admission events flowing into SQS FIFO.
**Mode:** mvp
**Depends on**: Phase 2
**Requirements**: INFRA-02, INFRA-03, INFRA-04, INFRA-05, INFRA-06
**Success Criteria** (what must be TRUE):

  1. ECS Fargate service runs the queue API container and auto-scales; a `docker push` + task redeploy produces a running service reachable at the queue-api subdomain
  2. Queue API connects to ElastiCache Redis (not the local container); sorted-set operations and pub/sub function identically to the local stack
  3. The static queue waiting page is served from S3 via CloudFront with the correct cache headers; the page loads from the CDN URL without hitting the queue API
  4. DynamoDB tables exist and queue-sessions audit records are written for each admitted ticket; queue-events config table is readable by the admin API
  5. Admitted events appear in the SQS FIFO queue with MessageGroupId = eventId within 2 seconds of admission

**Plans**: 5 plans

Plans:
**Wave 1**

- [ ] 03-01-PLAN.md — [Wave 1, tracer] Terraform skeleton: remote backend, module structure, networking (VPC/subnets/SGs), ECR repos

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 03-02-PLAN.md — [Wave 2, parallel] ECS Fargate + ElastiCache Redis + DynamoDB + SQS modules; SSM secrets wiring
- [ ] 03-03-PLAN.md — [Wave 2, parallel] Go internal/aws package (DynamoWriter + SQSEmitter); scheduler injection; GitHub Actions CI/CD

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 03-04-PLAN.md — [Wave 3, parallel] S3 + CloudFront static queue page; SameSite=Lax fix; per-event custom HTML upload/redirect
- [ ] 03-05-PLAN.md — [Wave 3, parallel] CloudFront Function edge JWT enforcement; CloudFront KVS; stub origin ALB lock-down

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Queue Core & Local Stack | 4/4 | Complete    | 2026-08-02 |
| 2. Frontend & Admin UI | 3/3 | In Progress|  |
| 3. AWS Infrastructure | 0/TBD | Not started | - |
