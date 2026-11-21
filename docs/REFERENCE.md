# Reference Architecture: Agent Execution Platform

> Summary of the system-design article this project implements. The article
> was supplied privately during project planning and has no public citation;
> this summary plus the design doc are the project's authoritative spec.

## The Problem

An agent execution platform runs AI agents built by other teams. Teams
package agent code (container images, via a platform SDK); the platform
owns the infrastructure: scheduling runs, isolated sandboxes, preserving
progress across failures, brokering access to models and tools, human
approvals, and recording what happened. Without it, every team rebuilds
retry logic, runs model-generated code with unrestricted network access,
loses in-memory progress on restarts, and nobody can see total spend.

**Boundary:** the platform owns reliability, isolation, and visibility.
Teams own prompts, answer quality, and evaluations.

## Requirements

Functional: register versioned agents (image pinned by digest, allowed
tools/models/budgets); start/cancel/stream runs (API, schedule, or event);
isolated sandbox per run; model calls through a shared LLM gateway and
tool calls through a registry with permission checks; pause for human
approval and resume on decision; full observability including sub-agents.

Non-functional: isolation (no cross-run reads, no host access, no
out-of-permission tool calls); durability (survive crashes/deploys without
repeating completed side effects); interactive start < 1 s p95; 1 M
runs/day ≈ 30 k concurrently active at peak; fairness across teams; every
token and sandbox-second attributed to a team/agent.

## Estimation Highlights

- ~36 starts/s average; concurrency via Little's law (arrival rate ×
  ~15 min active time) ≈ tens of thousands of active runs.
- ~15 steps/run, ~6 k input + 400 output tokens per model call → model
  spend (~$360 k/day pre-caching) dwarfs infrastructure spend. Prompt
  caching on stable prefixes is the big lever.
- Sandboxes at 2 GiB each → memory, not CPU, sizes the fleet (agents
  mostly wait on model responses, ~0.2 vCPU avg). Two optimizations
  follow: smaller sandboxes and releasing them during long waits.
- History ~10 KB/step (large payloads in object storage) → write rate
  modest; the hard requirement is consistency, not throughput.

## Core APIs

- `POST /agents` — register an agent version: image **by digest** (a
  resumed run must use the code it started with), allowed models/tools,
  budgets, egress policy.
- `POST /runs` — idempotency key (safe client retries); returns
  immediately with run id + QUEUED; caller authorization checked against
  the acting user.
- `GET /runs/{id}/events` — SSE stream; events carry sequence numbers;
  reconnect with `Last-Event-ID`.
- `POST /runs/{id}/signals` — approvals, follow-up messages, cancellation;
  authenticated and authorized (approver eligibility checked).
- `POST /tools` — register a tool with a **risk level** (drives approval
  requirements) and an **idempotency declaration** (whether retrying with
  the same key is safe — must reflect actual tool behavior).

## High-Level Design

1. **Accept & schedule.** Run API validates, writes run (QUEUED) + a
   durable enqueue instruction (transactional outbox). Per-team queues +
   weighted fair queuing so one team's 50 k batch runs can't starve
   another's interactive traffic; interactive prioritized within a team.
   One sandbox per run.
2. **Model & tool access.** The sandbox is the least-trusted component
   (prompt injection). Provider keys and tool credentials never enter it.
   All egress passes a host-level proxy: model calls → LLM gateway
   (allowlists, usage limits), tool calls → tool gateway (permissions,
   credential broker), everything else blocked unless the agent version's
   egress policy allows it.
3. **Durable progress.** SDK routes durable operations through the
   platform; results land in the run's history before execution advances.
   Hosts hold a lease (renew ~10 s, expire ~30 s); on expiry the scheduler
   reassigns, and the new sandbox reconstructs progress from history.
4. **Human approvals.** Tool gateway holds the call, records
   `approval.requested`, moves the run to WAITING, and **releases the
   sandbox** (a decision may take days; 2 GiB idle is waste). The decision
   signals the run, which requeues through the same resume path as crash
   recovery — exercising recovery in normal operation.
5. **Observability.** Spans from SDK (steps), gateways (model/tool calls),
   scheduler (queueing/startup). Budget enforcement uses authoritative
   usage and reservations, never delayed/sampled telemetry.

State machine: QUEUED → STARTING → RUNNING → {WAITING ⇄ QUEUED} →
SUCCEEDED | FAILED | CANCELLED. Both host failures and approval waits
return through QUEUED.

## Data Design

- **Run state:** strong per-run consistency, conditional updates; sharded
  Postgres by `run_id` (or a KV store with conditional writes).
  `run_events` PK `(run_id, seq)` prevents two events at one sequence.
- **Fencing:** scheduler increments `lease_epoch` on every ownership
  change; each history append atomically checks epoch + lease, rejecting
  stale workers. Gateways also validate ownership before admitting calls —
  fencing only DB writes would still let a stale worker fire a tool.
- **Approvals:** `action_hash` binds a decision to the exact tool +
  arguments shown to the approver ($420 approved ≠ $4,200 requested).
- **Large payloads** (> 64 KB outputs, snapshots) → object storage,
  referenced by `payload_ref`. **Traces/usage** → columnar store
  (ClickHouse-class), bodies in object storage.

## Deep Dive 1: Durable Execution

Four recovery approaches:

| Approach | Agent code changes | Resume cost | Long runs | Main weakness |
|---|---|---|---|---|
| Retry whole run | none | repeats all work/tokens | poor | may repeat side effects |
| Checkpoint state each step | explicit serialization | low | yes | teams omit required state |
| Event-sourced replay | SDK + deterministic orchestration | low for short histories | yes | replay discipline, version compatibility |
| VM snapshot | few | snapshot storage/restore | yes | size, host compatibility, in-flight connections |

**Event-sourced replay is the recommended default** (Temporal's model):
record results of non-deterministic operations (model calls, tool calls,
time/randomness); on recovery, restart orchestration from the beginning,
returning recorded results until reaching unfinished work, then go live.
Constraints: orchestration must be deterministic given recorded results;
runs keep their original agent image; workspace files need durable
snapshots tied to history boundaries (replayed tool results don't recreate
files).

**In-flight side effects:** a crash between a tool executing and its
result being recorded leaves an unknown outcome. Record `tool.intent`
(with stable idempotency key, e.g. `run_7Hk2:step-18`) before dispatch,
`tool.result` after. Intent without result → retry with same key if the
tool is idempotent, else query its status endpoint, else pause for
reconciliation. Never guess whether a refund happened.

## Deep Dive 2: Sandbox Lifecycle

Isolation options: runc containers (shared kernel — for trusted code),
gVisor (user-space kernel, syscall overhead), **microVMs**
(Firecracker/Kata — own guest kernel, the default for untrusted code;
~125 ms VMM boot but image pull + runtime init dominate real startup).

**Warm pools:** pre-initialized sandboxes; assignment = identity + input.
One run per sandbox; destroyed after use, never returned to the pool (no
state leakage). Pool sizing: arrival rate × preparation time, plus burst
headroom, measured per image. Rare images: lazy-loading snapshotters
(stargz/SOCI) or restored initialized snapshots instead of dedicated
pools; cold starts get an explicit service-tier exception.

**Release during waits:** park runs waiting > ~30 s (approvals, long
exports); memory determines fleet size, so parked runs must not hold
sandboxes.

## Deep Dive 3: Secrets & Per-Run Credentials

Problems with a broad service account in an env var: model-generated code
can read it (prompt injection exfiltrates it), and it creates a confused
deputy (agent exceeds the requesting user's authority).

Design: per-run signed identity token (run id, agent version, team, acting
user, permitted tools) bound to the current lease, short expiry, refresh
only while the lease is valid — but gateways also reject cancelled runs
and stale epochs (expiry is not the cancellation mechanism). The sandbox
never sees the identity token or downstream credentials: the host proxy
attaches identity; the tool gateway exchanges it (credential broker, token
exchange) for a short-TTL, user-scoped downstream credential. The
downstream API applies its normal access checks — if the user can't see
the data, their agent can't either. Legacy shared-key tools: key stays
outside the sandbox; gateway enforces user permissions + argument checks.
Everything audited: run, user, tool, scope, resources.

## Deep Dive 4: Tracing Multi-Agent Runs

One trace per root run: children carry `parent_run_id` + `root_run_id`,
context propagates via W3C traceparent. Cost views roll up by subtree
(the $14-instead-of-$2 run: two document-reader grandchildren account for
$9.30). Budgets enforced **across the tree** against the root's allowance
with atomic reserve-then-reconcile (children reading the same remaining
balance would all spend it). Resumed runs appear as separate execution
spans inside one trace; replay must not look like (or bill like) live
calls. Retention: compact metadata for all runs; debugging bodies kept for
failures/over-budget/negative-feedback + a sample of successes; replay
data undeletable while a run can still resume; redaction must not alter
authoritative replay data.

## Evaluation, Scale, Cost

Platform metrics: run outcomes split platform-fault vs agent-fault; resume
success rate (fault-injection tested); start latency warm vs cold;
approval turnaround; per-step overhead vs model latency. Safety: layered
controls (microVMs, egress policy, tool permissions, user-scoped
credentials, approvals, budgets) — no single boundary suffices.

At 10×: partition run state by `run_id` (not team — hot-team shards),
replicated schedulers partitioned by team queues, 1 GiB sandbox class,
scheduler slows admission when provider quotas are the constraint. Cost
levers: cacheable prompt prefixes, budgets + loop detection, visible cost
breakdowns; infra: park long waits, right-size sandboxes, snapshot rare
images, overcommit CPU with burst headroom.
