# ZOA Boundary — Architecture

## Role

ZOA Boundary is an **ephemeral jump server** in the **same VPC** as the target EKS cluster, with purpose-built tooling (`zoa` CLI, `kubectl`, `aws` CLI v2, Claude Code, `jq`). Interactive work is recorded to **CloudWatch** (container stdout + ECS Exec PTY transcripts). The container calls the **per-VPC API Lambda** (`ZOA_API_URL`) for Trusted Actions.

Design principles:

1. **Ephemeral** — No persistent disk state. TA artifacts live in **S3**; session metadata in **DynamoDB**. Flow: `session start` → work → **`session terminate`** from the laptop (or reaper).
2. **Shareable execution IDs** — Any authorized operator can inspect a TA run with `zoa get` / `zoa output` / `zoa runs` without sharing a session.
3. **Single CLI** — `zoa` covers sessions, TAs, discovery, and audit.
4. **Co-deployed with TA plane** — Boundary ECS, Lambdas, and IAM ship in the same hyperfleet Terraform modules (`zoa-lambda`, `zoa-access`, `zoa`).
5. **Reason-scoped sessions** — Each session carries a **reason** (Jira / PagerDuty) at start; multiple sessions may share one reason over time.

## Control plane split

| Component         | Where                  | Responsibility                                         |
| ----------------- | ---------------------- | ------------------------------------------------------ |
| **Access Lambda** | RC account, **no VPC** | Sessions, targets, `ecs:RunTask`, deployment discovery |
| **API Lambda**    | **Each** target VPC    | `zoa run`, TA execution, API audit                     |
| **Worker Lambda** | Each target VPC        | Reconciler, GC, **boundary reaper**                    |
| **Boundary ECS**  | Each target VPC        | Interactive shell + Claude assist                      |

**Laptop** → **Access** only (`zoa session *`, `zoa deployments`, `zoa targets`).  
**Inside boundary** → **API** (`zoa run`, `zoa actions`, …).

A compromised **invoker** can manage sessions but cannot impersonate arbitrary TA callers without the **boundary task role** on that VPC’s API URL.

## Identity (summary)

Three credential layers: **invoker + SigV4** to Access, **vended Exec credentials** scoped to one task on join, **ECS task role + SigV4** to the API inside the container.

Human attribution for TAs uses the **[identity bridge](../design/boundary-identity-bridge.md)** (task id → session row → `operator`). See [Operator workflow](../guides/operator-workflow.md#identity-and-sigv4) for the SRE walkthrough.

## Session lifecycle

1. **Start** — Access creates a DynamoDB session (`creating` → `active`), returns `deployment/target/session-uuid`.
2. **Join** — Access `RunTask` if needed, vends Exec credentials, returns `exec_command`. CLI registers SSM exec id via `exec-attached` (best-effort).
3. **Work** — ECS Exec shell; MOTD, prompt, `ZOA_SESSION.md`, offline `ZOA_ACTIONS.md`.
4. **Terminate** — From the laptop: `zoa session terminate <deployment>/<session-id>`. **Exit from Exec does not terminate.**
5. **Reaper** — Worker stops tasks when **deadline** or **idle** policy fires. See [session reaper](../design/boundary-session-reaper.md).

Session lifecycle APIs live on **Access** only, not the API Lambda.

## Discovery (SSM)

| Path                                     | Written by                      | Read by                    | Purpose                                         |
| ---------------------------------------- | ------------------------------- | -------------------------- | ----------------------------------------------- |
| `/zoa/deployments/<deployment_name>`     | RC Terraform (central provider) | Laptop (`zoa deployments`) | Access Function URL + invoker role              |
| `/zoa/targets/<deployment>/<cluster_id>` | `zoa-lambda` per target         | Access Lambda              | ECS cluster, subnets, task def, **ZOA_API_URL** |

Details: [deployments discovery](../design/deployments-discovery.md).

## RC vs MC

- **RC boundary** — Task in RC VPC; `ZOA_API_URL` → RC API Lambda.
- **MC boundary** — Access assumes into MC and `RunTask` there; session table stays in RC; cross-account Exec IAM in hyperfleet modules.

HyperFleet does **not** bridge RC↔MC networks; each VPC has its own Lambda pair and boundary stack.

## Network (summary)

Cluster VPCs use **gateway** and **interface** endpoints (S3, DynamoDB, STS, logs, ECR, KMS, Bedrock, SSM/Exec). **Function URLs** (laptop → Access, boundary → API) use **NAT** until private ingress exists.

## Related

- [Container image](container-image.md)
- [Session logging](../design/boundary-session-logging.md)
- [Break-glass (planned)](../break-glass/README.md)
