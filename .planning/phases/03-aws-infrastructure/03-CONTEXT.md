# Phase 3: AWS Infrastructure - Context

**Gathered:** 2026-08-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Deploy the full system to AWS and add a CloudFront edge enforcement layer (replacing the deferred Akamai EdgeWorker). Phase 3 naturally splits into two sub-phases:

**Phase 3a — Core compute + data infrastructure:**
ECS Fargate (queueserver, scheduler, stuborigin), two ElastiCache Redis clusters (redis-queue, redis-origin), ECR, DynamoDB audit tables, SQS FIFO admission queue, SSM Parameter Store for secrets, VPC/networking/ALBs — all defined in Terraform with a production module structure. GitHub Actions deploys on push to master.

**Phase 3b — Edge + content delivery + event customization:**
S3 + CloudFront for the static queue page; CloudFront distribution in front of the stub origin with a CloudFront Function for JWT signature verification at edge (the CloudFront replacement for Akamai EdgeWorker); per-event custom HTML stored in S3, uploadable via admin dashboard; `queue.js` SameSite cookie fix.

Phase 2 delivered the browser layer. Phase 3 puts it on AWS. No new queue mechanics.

</domain>

<decisions>
## Implementation Decisions

### Phase Split

- **D-01:** Phase 3 is split into **3a** (core infrastructure) and **3b** (edge + content delivery). 3a must complete before 3b (CloudFront distributions reference ALB outputs from 3a). — **Reversibility:** reversible

### IaC Toolchain

- **D-02:** **Terraform** is the IaC tool. HCL, AWS provider. Region: **ap-south-1 (Mumbai)** — closest to India, matches the IPL use case. — **Reversibility:** one-way — changing IaC tool requires rewriting all infra definitions.
- **D-03:** Terraform state stored in **S3 backend only** (no DynamoDB lock table — solo project). State bucket: one S3 bucket bootstrapped manually before first `terraform init`. — **Reversibility:** reversible
- **D-04:** **Production module structure** for learning:
  ```
  infra/
    modules/
      ecs/          # task definitions, services, ALBs
      redis/        # ElastiCache clusters
      cloudfront/   # CF distributions, Function, KeyValueStore
      s3/           # static page bucket + event HTML bucket
      dynamodb/     # audit + events tables
      sqs/          # FIFO admission queue
      networking/   # VPC, subnets, security groups
    environments/
      dev/
        main.tf     # calls modules
        variables.tf
        terraform.tfvars
        backend.tf
  ```
  One environment (`dev`) for Phase 3. — **Reversibility:** reversible

### Deployment Pipeline

- **D-05:** **GitHub Actions** workflow on push to master: build Go binaries → `docker build` → `docker push` to ECR → `aws ecs update-service --force-new-deployment` for each service. AWS credentials stored in GitHub Secrets. — **Reversibility:** reversible
- **D-06:** Three separate ECS services: `queueserver`, `scheduler`, `stuborigin`. Each has its own ECR repository and task definition. — **Reversibility:** reversible

### Secrets Management

- **D-07:** `ADMISSION_SECRET` and `SESSION_SECRET` stored in **SSM Parameter Store** (SecureString). ECS task definitions reference them via `secrets` block → injected as environment variables. `config.go` already reads `os.Getenv()` — no code change needed. — **Reversibility:** reversible

### CloudFront Edge Layer (Phase 3b)

- **D-08:** **CloudFront Functions** (not Lambda@Edge) for the edge enforcement layer. Lightweight viewer-request function: verifies HMAC-SHA256 JWT signature on `q_session` or `q_admission` cookie. No VPC access needed — SETNX one-time enforcement stays in Go `queue_guard.go` at the origin. — **Reversibility:** costly — switching to Lambda@Edge requires VPC wiring and rewrite.
- **D-09:** On invalid/missing token: **302 redirect** to `/queue/join?eventId=...&target=<original URL>`. No HTML served from edge. — **Reversibility:** reversible
- **D-10:** Secrets at edge: **CloudFront KeyValueStore**. `ADMISSION_SECRET` and `SESSION_SECRET` stored in the KVS and read by the CloudFront Function at request time. Rotatable without redeploying the function. — **Reversibility:** reversible
- **D-11:** CloudFront Function runs on the **stub origin distribution only** (the protected resource). Static queue page and admin dashboard distributions have no edge function — they are publicly readable. — **Reversibility:** reversible
- **D-12:** **Both layers run** (defence-in-depth): CloudFront Function does fast JWT signature check at edge; Go `queue_guard.go` still runs at origin for SETNX + q_session issuance. A token that passes edge but was already used gets 403 at origin. `queue_guard.go` is unchanged. — **Reversibility:** reversible
- **D-13:** Stub origin ALB **security group restricted to CloudFront managed prefix list** (com.amazonaws.global.cloudfront.origin-facing). ALB not directly reachable from public internet — CloudFront is the only entry point. — **Reversibility:** reversible

### No Custom Domains

- **D-14:** No custom domains — AWS-provided URLs throughout:
  - Static queue page: CloudFront distribution URL (`d1xxx.cloudfront.net`)
  - Queue API: ALB URL (`my-alb.ap-south-1.elb.amazonaws.com`)
  - Stub origin: CloudFront distribution URL (`d2yyy.cloudfront.net`)
  No ACM certificates needed. — **Reversibility:** reversible

### SameSite Cookie Fix (Phase 3b — required code change)

- **D-15:** `web/queue/queue.js` must change `q_admission` cookie from `SameSite=Strict` to **`SameSite=Lax`**. `cloudfront.net` is on the browser Public Suffix List — `d1xxx.cloudfront.net` (queue page) and `d2yyy.cloudfront.net` (stub origin) are treated as different sites. `Strict` blocks the cookie on the admission redirect; `Lax` sends it on top-level GET navigations. Safe: token is single-use (SETNX) and short-lived (30min). — **Reversibility:** reversible

### Static Files + Per-Event Custom HTML (Phase 3b)

- **D-16:** Static queue page (`web/queue/`) served from **S3 + CloudFront**. S3 bucket with public read disabled; CloudFront OAC (Origin Access Control) for access. `window.QUEUE_CONFIG.apiBase` set to the queue API's ALB URL. — **Reversibility:** reversible
- **D-17:** Per-event custom HTML: **full page replace**. Admin uploads a complete HTML file for an event. Stored at `s3://events-bucket/events/{eventId}/page.html`. — **Reversibility:** reversible
- **D-18:** Write path: **presigned S3 URL**. Admin dashboard calls `GET /queue/events/:id/page-upload-url` → queue API returns a short-lived presigned PUT URL → dashboard JS uploads HTML directly to S3. No HTML body passes through the queue server. — **Reversibility:** reversible
- **D-19:** Load path: **queue API redirects**. `GET /queue/join?eventId=X` handler checks if `events/{eventId}/page.html` exists in S3. If yes: 302 to the CloudFront URL for that page. If no custom page: falls back to default `queue/index.html`. — **Reversibility:** reversible

### Application Code Additions (Go — Phase 3a)

- **D-20:** **DynamoDB writes** in scheduler `admitBatch()`: after each ZPOPMIN admission, write a `queue-sessions` record (ticketId, eventId, admittedAt, jti) to DynamoDB. New `internal/aws/` package with injected DynamoDB client. Skipped when `AWS_REGION` env var is unset (local Docker Compose). — **Reversibility:** reversible
- **D-21:** **SQS FIFO emit** in scheduler `admitBatch()`: publish one message per admitted ticket to the `admission-events.fifo` SQS queue with `MessageGroupId = eventId`. Same `internal/aws/` package, same skip-if-no-region guard. — **Reversibility:** reversible

### Claude's Discretion

- VPC CIDR ranges, subnet layout, NAT gateway vs NAT instance
- ElastiCache node type (cache.t3.micro for dev is sufficient)
- ECS task CPU/memory sizing (512MB / 0.25 vCPU for dev)
- CloudFront cache behaviors and TTLs for the static page distribution
- S3 bucket naming convention
- GitHub Actions runner and exact workflow trigger conditions

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Architecture & Design

- `DESIGN.md` — Complete HLD v3. Section 8 (QueueGuard / EdgeWorker design) is the reference for the CloudFront Function logic. Section 10 (Go service structure) defines the three binaries. **Read before planning any component.**
- `DESIGN.md §8` — EdgeWorker-equivalent Go middleware. The CloudFront Function replicates the JWT verification portion of this for the edge runtime (no Redis calls, no SETNX).

### Requirements

- `.planning/REQUIREMENTS.md` — Phase 3 requirements: INFRA-02 through INFRA-06. Also EDGE-01 through EDGE-03 (now re-targeted to CloudFront instead of Akamai).
- `.planning/PROJECT.md` — Active requirements, key decisions, constraints, out-of-scope items.

### Phase Roadmap

- `.planning/ROADMAP.md` — Phase 3 goal and success criteria. Note: success criteria reference ECS Fargate, ElastiCache, S3/CloudFront, DynamoDB, SQS. CloudFront edge enforcement is an additional success criterion added in this discussion.

### Prior Phase Context

- `.planning/phases/02-frontend-admin-ui/02-CONTEXT.md` — Phase 2 decisions: D-01 (nginx static serving → replaced by S3+CloudFront in Phase 3b), D-02 (QUEUE_CONFIG.apiBase injection), D-03 (CORS), D-07 (event selector), D-08 (stats + rate/capacity controls). The presigned URL endpoint and custom HTML redirect are new API additions.
- `.planning/phases/01-queue-core-local-stack/01-CONTEXT.md` — Phase 1 decisions: D-02 (two Redis instances — maps to two ElastiCache clusters in Phase 3a), D-07 (Go module path), D-08 (service layout).

### Existing Code

- `pkg/middleware/queue_guard.go` — The CloudFront Function replicates only the JWT verify portion. SETNX + q_session issuance stay here unchanged.
- `internal/config/config.go` — `LoadQueueServer()`, `LoadScheduler()`, `LoadStubOrigin()`. All read from `os.Getenv()` — ECS task definitions inject values from SSM via `secrets` block. The `SECURE` env var in `StubOriginConfig` must be `true` in ECS (CloudFront delivers HTTPS).
- `internal/api/handler_join.go` — Phase 3b adds S3 existence check + conditional redirect to event-specific page before the existing QueuePageURL redirect.
- `internal/api/router.go` — Phase 3b adds `GET /queue/events/:id/page-upload-url` route.
- `web/queue/queue.js` — Phase 3b changes `SameSite=Strict` → `SameSite=Lax` on the `q_admission` cookie (line ~23).
- `docker-compose.yml` — Local stack remains unchanged. AWS infra is additive.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- `internal/config/config.go` — `LoadStubOrigin()` already has `Secure bool` field driven by `SECURE` env var. Set `SECURE=true` in ECS task definition → cookies are marked Secure automatically.
- `pkg/middleware/queue_guard.go` — CloudFront Function is a JS port of the JWT verify path in this file (lines ~43–56). The HMAC-SHA256 verify logic maps directly to `crypto.subtle.verify()` in CloudFront Functions runtime.
- `internal/scheduler/admission.go` — `admitBatch()` is the correct injection point for DynamoDB writes and SQS emits (D-20, D-21). Inject AWS clients via the `Scheduler` struct alongside the existing `issueToken` func.

### Established Patterns

- Three Docker Compose services → three ECS Fargate services. Same separation, same env var contract.
- Two Redis instances (redis-queue, redis-origin) → two ElastiCache clusters. Same naming, same service boundaries.
- Config via `os.Getenv()` → SSM `secrets` block in task definition. No code change needed for secret injection.
- CORS currently allows `http://localhost:8082`. In Phase 3b this must be updated to allow the CloudFront static page URL.

### Integration Points

- `cmd/*/main.go` — ECS task definitions point to ECR image URIs; Dockerfiles are already in repo.
- `docker-compose.yml` — Remains the local dev environment; not modified in Phase 3.
- `web/queue/queue.js` — One-line SameSite fix (D-15) + QUEUE_CONFIG.apiBase must point to ALB URL in the S3-deployed version.
- `internal/api/handler_join.go` — New S3 check before QueuePageURL redirect (D-19). Requires AWS SDK S3 client injected into Handler (or HEAD request in the join handler).

</code_context>

<specifics>
## Specific Ideas

- CloudFront Function runtime supports `crypto.subtle` for HMAC verification — no external JWT library needed. The function reads secrets from CloudFront KeyValueStore via `cloudfront.kvs()` API.
- `SECURE` env var already exists in `StubOriginConfig.LoadStubOrigin()` — set it to `"true"` in the ECS task definition (and `terraform.tfvars`) to flip cookies to Secure mode automatically.
- The `internal/aws/` package pattern (D-20, D-21): exported `DynamoWriter` and `SQSEmitter` types with `Write(ctx, record)` and `Emit(ctx, event)` methods. Both return `nil` when client is nil (skip guard for local dev). Injected into `Scheduler` via constructor.
- For the presigned URL flow (D-18): `s3.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: ..., Key: "events/{eventId}/page.html"}, s3.WithPresignExpires(15*time.Minute))`. Dashboard JS uses `fetch(presignedURL, { method: 'PUT', body: htmlContent })`.
- For the join redirect check (D-19): `s3.HeadObject(ctx, &s3.HeadObjectInput{Bucket: ..., Key: fmt.Sprintf("events/%s/page.html", eventID)})` — if no error, redirect to `https://{events-cf-domain}/events/{eventID}/page.html`.

</specifics>

<deferred>
## Deferred Ideas

- **Custom domain / TLS cert** — Add ACM certificate + Route53 hosted zone when a domain is available. No code changes needed beyond Terraform additions.
- **Admin dashboard auth** (bearer token or IP allowlist) — Phase 3 / prod hardening, noted in Phase 2 CONTEXT.md. Still deferred.
- **OPS-03: TTL-sweep background job** — Decrement `active:{eventId}` on `token:{jti}` keyspace expiry (abandoned sessions). v2 backlog.
- **OPS-02: CloudWatch alarms** — `queue_depth`, `active_users`, `capacity_headroom`, `setnx_failures` → SNS. v2 backlog.
- **Multi-region / failover** — Out of scope for learning project.

</deferred>

---

*Phase: 3-AWS Infrastructure*
*Context gathered: 2026-08-29*
