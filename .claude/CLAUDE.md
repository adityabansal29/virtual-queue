<!-- GSD:project-start source:PROJECT.md -->

## Project

**Virtual Queue System**

A production-replica in-house virtual queue system for high-throughput event ticket sales (IPL-scale). It holds excess traffic in a virtual waiting room and admits users in FIFO order at a configurable rate, bounded by both admission rate and a hard concurrency ceiling. Queued users generate zero load on the protected origin.

**Core Value:** A waiting user must always see an accurate position, and an admitted user must always be let through exactly once — no skips, no duplicates, no queue bypass.

### Constraints

- **Tech stack**: Go (queue service), Redis (ElastiCache), AWS (ECS Fargate, S3, DynamoDB, SQS), Akamai (edge, deferred)
- **EdgeWorker runtime**: No Akamai account yet — EdgeWorker JS written but tested via equivalent Go middleware locally
- **Scale target**: Functional correctness over load validation — design is already validated on paper for 500k+

<!-- GSD:project-end -->

<!-- GSD:stack-start source:STACK.md -->

## Technology Stack

Technology stack not yet documented. Will populate after codebase mapping or first phase.
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->

## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->

## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->

## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:

- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->

## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
