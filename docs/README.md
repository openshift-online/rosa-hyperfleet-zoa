# ZOA Documentation

**Zero Operator Access (ZOA)** is the audited operator access model for ROSA HyperFleet. No standing cluster admin, no laptop-side `kubectl` — every action flows through an audited, reason-tagged path.

Platform Terraform and accounts: [ZOA architecture (rosa-hyperfleet)](https://github.com/openshift-online/rosa-hyperfleet/blob/main/docs/design/zoa-architecture.md). Behavior and CLI/API contracts: **this repo**.

## Start here

| You are… | Read |
|----------|------|
| **Operator** (session) | [Operator workflow](guides/operator-workflow.md) → `zoa session start` → work inside boundary |
| **TA author** | [Trusted Actions](trusted-actions.md) — writing, testing, conformance gates |
| **Contributor** | [Development](development.md) · [E2E testing](e2e-testing.md) |

## Boundary

The boundary is the shell. Everything else happens inside it.

| Document | Description |
|----------|-------------|
| [Boundary architecture](boundary/architecture.md) | Access Lambda vs API Lambda, SSM, session lifecycle |
| [Container image](boundary/container-image.md) | `Containerfile.boundary`, Konflux pipeline |
| [Session logging](design/boundary-session-logging.md) | CloudWatch: container logs vs ECS Exec transcript |
| [Session reaper](design/boundary-session-reaper.md) | Hard deadline + inactivity termination |
| [Identity and storage](design/boundary-identity-and-storage.md) | Identity bridge (task ARN → session → operator), DynamoDB schema |
| [ZOA deployment discovery](design/deployments-discovery.md) | Central SSM `/zoa/deployments`, failover, `--deployments-region`, direct Access |

## Trusted Actions

TAs run from inside the boundary via `zoa run`. They are the steady-state operational model.

| Document | Description |
|----------|-------------|
| [TA authoring guide](trusted-actions.md) | Writing a new TA, conformance gates, RBAC patterns |
| [API reference](api-reference.md) | TA dispatch + Access session APIs |
| [CLI reference](cli-reference.md) | Commands, sessions, discovery, offline catalog |

## Break-glass

Not implemented. See [break-glass/README.md](break-glass/README.md) for the planned stub.

**Policy:** break-glass is a one-time escape hatch. Every break-glass use must produce a follow-up TA that automates the same operation — the TA catalog grows monotonically so the same emergency never requires raw access twice.

## Architecture

Cross-cutting concerns that span all three pillars.

| Document | Description |
|----------|-------------|
| [Lambda model](architecture/lambda-model.md) | API, Worker, Access — three modes from one image |
| [Implementation](architecture/implementation.md) | Package layout, execution flows, env by `HANDLER_MODE` |
| [Storage](architecture/storage.md) | DynamoDB tables, S3, TTL, GSIs |
| [Timeout tuning](architecture/timeout-tuning.md) | Lambda ceiling → code deadline → per-TA timeout |

## Engineering

| Document | Description |
|----------|-------------|
| [Development](development.md) | Build, test, lint, images |
| [E2E testing](e2e-testing.md) | Functional + monitoring suites, Makefile targets |
| [Observability](observability.md) | TA metrics; session metrics TBD |
| [Konflux](konflux.md) | `zoa-lambda`, `zoa-runner`, `zoa-boundary` images |

## Doc conventions

- **Guides** — end-to-end tasks (operator workflow, TA authoring).
- **Reference** — CLI/HTTP contracts. Keep in sync with code.
- **Architecture / design** — how it works and decisions that survive refactors.

Platform ADRs live in **rosa-hyperfleet**; implementation detail lives here with the code.
