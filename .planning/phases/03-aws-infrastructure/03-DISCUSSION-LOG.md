# Phase 3: AWS Infrastructure - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-29
**Phase:** 3-aws-infrastructure
**Areas discussed:** CloudFront edge layer, IaC toolchain, Custom HTML per event, Deploy scope

---

## Scope Additions (pre-discussion)

Before area selection, the user added two scope changes:
1. **CloudFront replaces Akamai** — move edge enforcement from v2 backlog into Phase 3, using CloudFront instead of Akamai EdgeWorker.
2. **Custom HTML per event via admin dashboard** — stored in S3, new capability not in original roadmap.
3. **Phase split allowed** — user explicitly allowed Phase 3 to be split into 3a/3b if scope warranted it.

---

## CloudFront Edge Layer

| Option | Description | Selected |
|--------|-------------|----------|
| JWT verify only (CloudFront Functions) | No VPC, JWT signature check at edge, SETNX stays at origin | ✓ |
| Full QueueGuard at edge (Lambda@Edge) | VPC access to ElastiCache, SETNX + q_session at edge | |

**User's choice:** JWT verify only — CloudFront Functions

| Option | Description | Selected |
|--------|-------------|----------|
| Redirect to queue join URL | 302 → /queue/join?eventId=...&target=... | ✓ |
| Return 401 with error page | HTML error page from edge | |

**User's choice:** Redirect to queue join URL

| Option | Description | Selected |
|--------|-------------|----------|
| CloudFront KeyValueStore | First-party secret injection, rotatable without redeploy | ✓ |
| Hardcoded in function at deploy time | Simpler but secrets visible in IaC state | |

**User's choice:** CloudFront KeyValueStore

| Option | Description | Selected |
|--------|-------------|----------|
| Origin paths only | Edge function on stub origin distribution only | ✓ |
| All distributions | Edge function on every CloudFront distribution | |

**User's choice:** Origin paths only

| Option | Description | Selected |
|--------|-------------|----------|
| Both CF edge + Go QueueGuard run | Defence-in-depth | ✓ |
| Remove Go QueueGuard | Edge handles everything — not viable with CF Functions | |

**User's choice:** Both layers run

**Notes:**
- User asked about custom domains: no domains available (learning project). Decision: AWS-provided CloudFront and ALB URLs throughout, no ACM certs needed.
- User flagged SameSite=Strict cookie issue: `q_admission` cookie set with SameSite=Strict on queue page domain (`d1.cloudfront.net`) won't be sent on redirect to stub origin (`d2.cloudfront.net`) because `cloudfront.net` is on the Public Suffix List. Fix: change to SameSite=Lax in queue.js.

---

## IaC Toolchain

| Option | Description | Selected |
|--------|-------------|----------|
| Terraform | HCL, AWS provider, terraform plan for dry-run | ✓ |
| AWS CDK (TypeScript) | TypeScript, compiles to CloudFormation | |

**User's choice:** Terraform

| Option | Description | Selected |
|--------|-------------|----------|
| S3 backend, no DynamoDB lock | Sufficient for solo use | ✓ |
| S3 + DynamoDB lock table | Concurrency protection, overkill for solo | |

**User's choice:** S3 backend only

| Option | Description | Selected |
|--------|-------------|----------|
| Flat root module | All .tf files at infra/ root, simplest | |
| Modules per service (production structure) | infra/modules/ + infra/environments/ | ✓ |

**User's choice:** Production module structure (explicit learning goal)

| Option | Description | Selected |
|--------|-------------|----------|
| ap-south-1 (Mumbai) | Closest to India, matches IPL use case | ✓ |
| us-east-1 (N. Virginia) | Cheapest, default for tutorials | |

**User's choice:** ap-south-1 (Mumbai)

**Notes:**
- User asked to explain Terraform S3 + DynamoDB state storage. Explanation provided. User chose S3 only (no DynamoDB lock) as solo project.

---

## Custom HTML Per Event

| Option | Description | Selected |
|--------|-------------|----------|
| Full page replace | Admin uploads complete HTML, stored at events/{eventId}/page.html | ✓ |
| Template slots only | Logo, headline, color stored as JSON in DynamoDB | |

**User's choice:** Full page replace

| Option | Description | Selected |
|--------|-------------|----------|
| Queue API proxy | Dashboard calls PUT /queue/events/:id/page, server writes to S3 | |
| Presigned S3 URL | Dashboard calls GET upload-url, uploads directly to S3 | ✓ |

**User's choice:** Presigned S3 URL

| Option | Description | Selected |
|--------|-------------|----------|
| Queue API redirects | /queue/join checks S3, redirects to event-specific page if exists | ✓ |
| Default page fetches inline | Default page JS fetches and replaces body | |

**User's choice:** Queue API redirects to event-specific S3/CF URL

---

## Deploy Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Actually deploy to AWS | Run terraform apply, verify against live endpoints (~₹400-800/day) | ✓ |
| IaC-complete, no live deploy | terraform plan only, no AWS account required | |

**User's choice:** Actually deploy to AWS

| Option | Description | Selected |
|--------|-------------|----------|
| Makefile targets | make docker-build, make ecr-push, make deploy | |
| GitHub Actions workflow | Push to master triggers build + ECR push + ECS redeploy | ✓ |

**User's choice:** GitHub Actions workflow

| Option | Description | Selected |
|--------|-------------|----------|
| CF only, ALB blocked to public | ALB SG restricted to CloudFront managed prefix list | ✓ |
| CF optional, ALB also accessible | CloudFront for demo but ALB publicly reachable | |

**User's choice:** CF only, ALB blocked to public

**Notes:**
- User asked whether CloudFront domain is necessary for stub origin access. Answer: not technically required (Go QueueGuard still works), but CloudFront in front of stub origin IS the Akamai replacement — the core Phase 3b deliverable.

---

## Claude's Discretion

- VPC CIDR ranges and subnet layout
- NAT gateway vs NAT instance
- ElastiCache node type (cache.t3.micro for dev)
- ECS task CPU/memory sizing (512MB / 0.25 vCPU for dev)
- CloudFront cache behaviors and TTLs
- S3 bucket naming convention
- GitHub Actions runner and workflow trigger details

## Deferred Ideas

- Custom domain / TLS cert — when a domain becomes available
- Admin dashboard auth — Phase 3 prod hardening (noted in Phase 2 also)
- OPS-03: TTL-sweep background job — v2 backlog
- OPS-02: CloudWatch alarms — v2 backlog
- Multi-region / failover — out of scope for learning project
