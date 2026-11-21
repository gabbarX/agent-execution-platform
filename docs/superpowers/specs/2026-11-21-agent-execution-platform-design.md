# Agent Execution Platform — Design

**Date:** 2026-11-21
**Status:** Draft, awaiting owner approval
**Owner:** Ankit (learning + portfolio project)

## Intent and Why

Build a working, single-machine implementation of an *agent execution
platform*: a system that runs AI agents written by other teams. The
platform — not the agents — owns scheduling, sandboxed execution, durable
progress that survives crashes, gated access to models and tools, human
approval of risky actions, and tracing/cost attribution.

Two reasons this exists, in priority order:

1. **Learning.** Ankit starts from zero on these concepts. Every milestone
   is also a lesson: concepts are explained before they're coded, and the
   explanation is captured in the docs. A milestone where the code works
   but the ideas weren't understood is a failure.
2. **Portfolio.** This repo goes on a resume. Every claim it supports must
   be defensible in an interview ("show me the fencing logic", "kill the
   worker and prove the run resumes"). Documentation quality is a
   first-class deliverable, not an afterthought.

The reference design is the "Design an Agent Execution Platform" system
design article. The owner supplied its full text in conversation; it has
no public URL we can cite, so M0 captures a faithful summary as
`docs/REFERENCE.md` and that file plus this doc are jointly authoritative
from then on. Portfolio wording: "based on a published system-design
reference architecture", not a named citation. We implement its
architecture at laptop scale, preserving the *mechanisms* (event-sourced
replay, leases + fencing epochs, idempotency keys, budget reservation,
action-hash-bound approvals) while scaling down the *infrastructure*
(no fleet, no Firecracker, no 30k concurrent runs).

## Goals

- A platform binary (control plane) and worker(s) that accept runs over
  HTTP, schedule them fairly across teams, and execute them to completion.
- **Durable execution:** runs survive a killed worker. Recovery is
  event-sourced replay from a Postgres event log, protected by leases and
  fencing epochs so a stale worker cannot corrupt state. This is the
  centerpiece — the demo is literally `kill -9` mid-run.
- **Agent SDK (Go):** teams write agents as ordinary Go programs against a
  small SDK; durable operations (model calls, tool calls, sleeps) route
  through the platform and are recorded/replayed.
- **Sandboxing:** agent processes run in Docker containers with no network
  egress except the platform's gateways.
- **Gateways:** an LLM gateway (mock provider by default, real Anthropic
  provider behind an env var) and a tool gateway enforcing per-agent
  allowlists, per-run budgets with atomic reservation, and idempotency
  keys on side-effecting tools.
- **Approvals:** runs park in WAITING (releasing their sandbox), notify an
  approver, and resume on decision; decisions are bound to an action hash
  so the approved arguments can't drift.
- **Observability:** SSE event streaming to callers, parent/child run
  trees with roll-up cost attribution, and a minimal web dashboard.
- **Documentation:** `docs/ARCHITECTURE.md` kept current; one
  `docs/milestones/M<N>-<topic>.md` per milestone (what was built, the
  concepts behind it, the demo script); ADRs in `docs/adr/` for decisions
  that had real alternatives.
- Every milestone ends in a scripted, repeatable demo.

## Non-Goals

- Production scale: no multi-host fleet, no 30k concurrency, no sharding.
  Load numbers from the article inform the design but are not targets.
- Firecracker/gVisor: Docker containers are the isolation boundary.
  The trade-off is documented, not hidden.
- Real credential brokering (OIDC token exchange, per-user downstream
  auth): modeled with signed per-run tokens and a stub broker, honestly
  labeled as a simplification.
- Kubernetes, Temporal, or any workflow engine: the durable-execution
  machinery is built by hand — that's the point.
- Agent *quality*: prompts, evals, retrieval. The platform runs agents;
  it doesn't make them smart.
- Multi-language SDKs: Go only.

## Anti-Goals

Things that would make this a failure even if the code runs:

- A simulation that only *looks* durable (e.g. state survives because the
  process never actually dies in the demo). Crash recovery must be real.
- Resume claims the owner can't explain under questioning.
- A single giant build session that produces 10k lines nobody understood.
  The process is small milestones, explained as they're built.

## Constraints

- Windows 11 host; PowerShell + Git Bash available. Platform code must
  build and run on Windows; sandbox milestones require Docker Desktop
  (installed; must be started manually per session).
- Go 1.27 (installed). Docker 29.x CLI + Docker Desktop (installed).
- Postgres runs in Docker via `docker compose` from M2 onward. Milestone
  M1 is deliberately storage-free (in-memory) so there's a demo before
  any infrastructure exists.
- Model calls cost money only when the owner opts in: the mock provider
  is the default everywhere, including demos and tests.
- Work happens across many sessions over months. The repo must always be
  in a state a fresh session can pick up: passing tests, current docs,
  and a `docs/STATE.md` noting what's done and what's next.

## Approaches Considered

1. **Vertical slices (chosen).** A thin end-to-end walking skeleton
   first, then one deep concern per milestone. Demoable at every step;
   each milestone teaches one big idea. Cost: some early code gets
   rewritten as later milestones deepen it (accepted — rewriting is
   teaching).
2. **Component-by-component** in article order. Cleaner mapping to the
   reference text, but nothing runs end-to-end until most components
   exist. Rejected: too long without a demo, bad for motivation and
   for incremental learning.
3. **Build on Temporal.** Realistic industry choice, but it outsources
   the most instructive subsystem (durable replay) and weakens the
   "I built this" story. Rejected for this project's goals.

## Milestones

Each milestone ships: working code, tests, a demo script
(`demos/m<N>.md` with exact commands), a milestone doc with the concepts,
and updated ARCHITECTURE/STATE docs. Sizing: roughly 2–5 working
sessions each.

- **M0 — Repo foundation.** Go module, repo layout, `CLAUDE.md` coding
  guidelines for agentic work, codegraph indexing (exact tool verified at
  setup; see "Left to the builder"), lint/test tooling (`go vet`,
  `golangci-lint`, `go test`), docs skeleton (ARCHITECTURE, STATE,
  REFERENCE, adr/, milestones/), git init + first commit.
  *Demo:* `make check` (or PowerShell equivalent) passes on a hello-world
  package.
- **M1 — Walking skeleton.** Control-plane HTTP API (`POST /runs`,
  `GET /runs/{id}`, `POST /runs/{id}/cancel`), in-memory per-team queues
  with weighted fair scheduling, in-process worker pool executing a
  built-in fake agent (sleeps + fake steps), run state machine
  (QUEUED→STARTING→RUNNING→terminal) with validated transitions,
  idempotency keys on run creation. Run requests name an agent + version
  from day one (`{"agent": "demo", "version": 1, ...}`) against a
  built-in in-memory registry, so the API surface never breaks when the
  real registry arrives.
  *Demo:* curl a run and watch states; submit a burst from team A and show
  team B's runs aren't starved.
- **M2 — Durability.** Postgres (docker compose): `runs`, `run_events`
  (append-only history with per-run sequence numbers), transactional
  outbox for enqueue; worker leases with heartbeat + expiry; fencing
  epochs checked atomically on every history append; scheduler reassigns
  expired-lease runs. Agent registry moves to Postgres with a
  `POST /agents` registration API (per the reference article's
  register-agent-version API), carrying per-version allowlists/budget
  defaults that later milestones enforce.
  Recovery in M2 is honest *checkpoint-style resume*: the new worker
  reads the event log and continues the built-in agent from the last
  recorded step. True event-sourced *replay* (re-executing orchestration
  code while feeding back recorded results) requires the SDK and arrives
  in M3 — the M2 milestone doc teaches the difference explicitly, since
  it's a classic interview distinction.
  *Demo:* kill the worker process mid-run; a second worker picks the run
  up from the event log and finishes it without redoing completed steps;
  a resurrected stale worker's writes are rejected (fencing), shown in
  logs.
- **M3 — Agent SDK + replay.** `sdk/` package: deterministic orchestration
  rules, `sdk.Step`, recorded operations (model call, tool call, sleep,
  now/random via recorded values). Agents become separate Go programs
  launched as subprocesses (not yet containerized), talking to the worker
  over a local socket/HTTP. Event-sourced replay: on resume, recorded
  results are fed back instead of re-executing.
  *Demo:* a 10-step agent is killed at step 6; on resume, steps 1–5 replay
  from history (visibly instant, no side effects re-fire), 6–10 run live.
- **M4 — Sandboxes.** Agents packaged as Docker images; worker becomes a
  sandbox host that starts one container per run with no network except
  the gateway endpoint; a small warm pool for the demo agent image;
  sandboxes destroyed after each run, released when a run parks.
  *Demo:* agent code tries to `curl` the open internet and fails; tries
  the gateway and succeeds; warm-pool start visibly faster than cold.
- **M5 — Gateways.** LLM gateway: per-agent-version model allowlist,
  budget reservation/reconciliation (atomic, in Postgres), mock provider +
  Anthropic provider, request/response recording for replay. Tool
  gateway: tool registry, permission checks, `tool.intent`/`tool.result`
  events with idempotency keys, risk levels. Per-run signed identity
  token attached by the worker (host side), never visible inside the
  sandbox.
  *Demo:* an agent exceeds its token budget and is stopped mid-run with a
  clean FAILED(budget) state; a crashed run retries a side-effecting tool
  with the same idempotency key and the (demo) tool dedupes it.
- **M6 — Approvals.** High-risk tools trigger an approval request:
  `approval.requested` event, run → WAITING, sandbox released. Approval
  API + action hash binding (changed arguments invalidate the decision).
  Decision signals the run; it requeues through the normal resume path.
  *Demo:* agent requests a "refund"; owner approves via curl (or the M7
  dashboard); run resumes in a fresh sandbox and completes. A tampered
  re-request with different arguments is blocked.
- **M7 — Observability.** `GET /runs/{id}/events` as SSE with
  Last-Event-ID resume; parent/child runs (`parent_run_id`,
  `root_run_id`) with shared root budgets; cost roll-up per run tree;
  minimal dashboard (single static page served by the control plane)
  showing run list, run tree, event timeline, token/cost totals.
  *Demo:* a multi-agent run fans out to children; dashboard shows the tree
  and which branch spent the money.

Possible post-M7 stretch (not committed): structured OTel export,
history compaction/checkpoints, a second example agent using the real
Anthropic provider end-to-end.

## Decided Hows

- **Language:** Go 1.27 (owner's pick).
- **Storage:** Postgres 17 in Docker, accessed with `pgx`. No ORM —
  hand-written SQL, because the schema (event log, fencing) *is* the
  lesson. (Builder's call, stated here for visibility.)
- **HTTP:** Go stdlib `net/http` with 1.22+ pattern routing; no web
  framework. JSON APIs; SSE for streaming. (Builder's call.)
- **Sandbox tech:** Docker via the official Docker Go SDK. Documented as
  the laptop stand-in for microVMs.
- **Model providers:** `ModelProvider` interface; `mock` default,
  `anthropic` optional via `ANTHROPIC_API_KEY`. The mock provider reports
  deterministic token usage so budget demos and tests are repeatable.
- **Budget model (builder's call, per the article's mechanics):** budgets
  are denominated in USD cents, computed from token usage via a static
  per-model price table (mock models included). A budget may be supplied
  by the caller on `POST /runs`, defaulting to the agent version's
  configured budget. Child runs draw from the *root* run's single shared
  pool — the gateway atomically reserves against it before each model
  call and reconciles after, exactly the concurrency mechanism the
  article warns about ("several children could read the same value and
  all spend it").
- **Repo shape:** single Go module monorepo —
  `cmd/` (controlplane, worker, agent binaries), `internal/` (platform
  packages), `sdk/` (public agent SDK), `agents/` (example agents),
  `docs/`, `demos/`, `deploy/` (docker-compose). (Builder's call.)
- **Process:** each milestone goes brainstorm-refresher → written plan
  (superpowers:writing-plans) → TDD implementation → demo → docs → commit.
  Git history itself is part of the portfolio (clean, explained commits).
- **Teaching contract:** before implementing a concept (e.g. fencing), it
  gets a plain-language explanation in the session and a distilled version
  in the milestone doc. Owner should be able to re-derive the idea from
  the docs alone.

## Left to the Builder

- Exact package layout inside `internal/`, table DDL details, index
  choices.
- Library picks beyond those stated (e.g. test assertion lib or stdlib
  only; golangci-lint config).
- "Codegraph" tooling: owner asked to "install codegraph" for better
  agentic coding. Builder verifies what the current best codegraph/code-
  indexing tool for Claude Code on Windows is at M0 time and installs it,
  or documents why it was skipped, in M0's milestone doc.
- Worker↔agent transport details in M3 (local HTTP vs stdio), revisited
  at M4 when the boundary becomes a container wall.
- Demo scripting format (PowerShell vs bash vs Make targets) — must work
  on this Windows machine either way.
