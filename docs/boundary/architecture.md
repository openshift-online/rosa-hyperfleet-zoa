# ZOA Boundary — Architecture

## Role

ZOA Boundary is an **ephemeral jump server** in the **same VPC** as the target EKS cluster, with purpose-built tooling (`zoa` CLI, `kubectl`, `aws` CLI v2, Claude Code, `jq`). Everything that happens inside is audited via CloudWatch logs (container stdout + ECS Exec transcripts, FedRAMP AU-09). It provides:

- Network path to the **private EKS API** (for TAs and future break-glass).
- **`zoa` CLI** wired to the **per-VPC API Lambda** (`ZOA_API_URL`).

The design is deliberately stateless:

1. **Ephemeral by design.** Sessions are disposable containers with no persistent state. TA outputs live in **S3** (DynamoDB-indexed); session metadata lives in **DynamoDB**. Nothing survives the task stop — the container disk is discarded. This eliminates lifecycle management (orphaned storage, stale state, cleanup failures) and simplifies the operator experience to `session start` → work → `session stop`.
2. **Shareable by ID.** Every TA execution produces an ID that any authorized operator can use to retrieve output (`zoa output <id>`), inspect metadata (`zoa get <id>`), or audit the full history (`zoa runs`). DynamoDB records who ran what, against which target, with which parameters — no need to share a session or a filesystem. This also enables third-party integrations: an automation (PagerDuty, EventBridge) can execute TAs and pass execution IDs to the paged SRE, who picks up pre-gathered diagnostics from S3 without re-running anything.
3. **Single CLI.** `zoa` is the only SRE CLI — sessions, TAs, break-glass (future), discovery, audit. No separate tooling to maintain.
4. **Pipeline integration.** Boundary image builds, ECS task definitions, SSM target registration, and boundary IAM all live in the same Terraform modules as the Lambda TA plane (`modules/zoa-lambda`). A single `terraform apply` deploys both — no parallel stack per region.
5. **Reason-scoped.** Access is scoped around a **reason** (Jira/PD ticket) that is a field on each session — the session IS the task, 1:1. An SRE might open 3 sessions across 2 days for the same reason; the reason links them, not a container lifecycle.

## Control plane split

| Component         | Where                  | Responsibility                                                                  |
| ----------------- | ---------------------- | ------------------------------------------------------------------------------- |
| **Access Lambda** | RC account, **no VPC** | Session start/stop/list, `ecs:RunTask`, target discovery, future approve/reject |
| **API Lambda**    | **Each** target VPC    | `zoa run`, TA execution, audit for API calls                                    |
| **Worker Lambda** | Each target VPC        | Reconciler, GC, **boundary reaper** (session **deadline** + **idle**)           |
| **Boundary ECS**  | Each target VPC        | Interactive shell + Claude assist                                               |

**From your laptop** you talk to **Access** only for sessions (`zoa session *`, `zoa deployments`, `zoa targets`).

**Inside the boundary** you talk to the **API Lambda** for TAs (`zoa run`, `zoa actions`, …).

A compromised **invoker** role can manage sessions but cannot replace the boundary task role for arbitrary TA calls. A **boundary task role** can invoke only **that VPC’s** API Function URL.

## Identity (summary)

Three separate credential layers: **invoker + SigV4** to Access on the laptop, **vended Exec credentials** scoped to one ECS task on join, **ECS task role + SigV4** to the API inside the container. Session **ownership** and TA **operator** attribution are enforced server-side (DynamoDB session rows + **identity bridge**: ECS task id → session → human operator). Env vars such as `ZOA_OPERATOR` are for the shell UX only.

**Authoritative walkthrough for SREs and reviewers:** [Operator workflow — Identity and SigV4](../guides/operator-workflow.md#identity-and-sigv4).

## Session lifecycle

1. **Start** — Access creates a DynamoDB session row (`creating`), returns `deployment/target/session-uuid`.
2. **Join** — Access starts the Fargate task (if needed), vends ECS Exec credentials, returns `exec_command` (default `runuser -u sre -- /bin/bash -l`). After `ExecuteCommand`, the CLI registers the SSM exec session id with Access (`exec-attached`; best-effort if Access is down).
3. **Work** — SRE uses ECS Exec; shell prompt shows `session:<id>` and `operator@zoa:deployment/target`. Entrypoint writes `ZOA_SESSION.md` and RC/MC **`ZOA_ACTIONS.md`** from the image catalog (`ZOA_TARGET_TYPE`).
4. **Stop** — From the laptop: `zoa session stop <deployment>/<session-id>` (use the session line from the prompt or `zoa session list`).
5. **Reaper** — Worker stops tasks when **`deadline`** passes (`SESSION_MAX_DURATION_HOURS` on Access at start) or when **idle** exceeds **`SESSION_IDLE_TIMEOUT_SECONDS`** (SSM + CloudWatch exec logs; unused tasks with no join use `createdAt`). See [session reaper](../design/boundary-session-reaper.md).

Session stop is an **Access** API operation today. It does not belong on the API Lambda (TA plane).

## Discovery (SSM)

Two SSM layouts, one **Central account** store:

| Path                                     | Written by                                           | Read by                        | Purpose                                                    |
| ---------------------------------------- | ---------------------------------------------------- | ------------------------------ | ---------------------------------------------------------- |
| `/zoa/deployments/<deployment_name>`     | RC Terraform (central provider)                      | **Laptop** (`zoa deployments`) | Access Function URL + invoker role ARN                     |
| `/zoa/targets/<deployment>/<cluster_id>` | `zoa-lambda` module per target (RC writes locally; MC writes cross-account via `zoa-data-access` role) | **Access Lambda** | ECS cluster, subnets, task definition, **per-VPC API URL** |

**Why SSM for both?** Single discovery mechanism for “where is this environment?” without a custom registry DB. Deployments answer “how do I reach Access?”; targets answer “where do I run the boundary task and what `ZOA_API_URL` do I inject?”

Ephemeral regions use deployment names like `us-east-1-eph-<id>`; production uses region-style names (e.g. `us-east-1`).

## RC vs MC

- **RC boundary** — Task in RC VPC; `ZOA_API_URL` → RC API Lambda.
- **MC boundary** — Access **assumes role** into MC and `RunTask` there; same session table in RC; cross-account exec KMS and IAM documented in hyperfleet `zoa-boundary` module.

HyperFleet does **not** connect RC and MC networks; each VPC gets its own Lambda pair + boundary stack.

## Network (summary)

Cluster VPCs use **gateway** endpoints (S3, DynamoDB) and **interface** endpoints (STS, logs, ECR, KMS, Bedrock, SSM/Exec trio). **Lambda Function URLs** (laptop → Access, boundary → API) still use **NAT** until private ingress is designed.

## Future (same epic)

- Break-glass kube/AWS via scoped roles.
- Approval workflow on Access (no boundary required to approve).
- Audited **presigned download** for TA artifacts to laptop (`zoa download` extension).
