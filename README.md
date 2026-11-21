# Agent Execution Platform

A platform that runs AI agents built by other teams. Teams hand over agent
code; the platform owns everything around it: fair scheduling across teams,
isolated sandboxes for untrusted (often model-generated) code, durable
progress that survives worker crashes without repeating side effects,
gated access to models and tools with budgets and permissions, human
approval for risky actions, and full tracing with cost attribution.

Built in Go as a learning + portfolio project, implementing a published
system-design reference architecture at laptop scale. The *mechanisms*
are real — event-sourced replay, leases and fencing epochs, idempotency
keys, atomic budget reservation, action-hash-bound approvals — while the
*infrastructure* is scaled down (Docker containers instead of microVMs,
one machine instead of a fleet).

## Status

| Milestone | Scope | Status |
|-----------|-------|--------|
| M0 | Repo foundation: quality gate, docs skeleton, agent tooling | ✅ done |
| M1 | Walking skeleton: run API, fair scheduler, worker pool, state machine | ✅ done |
| M2 | Durability: Postgres event log, leases, fencing, checkpoint resume | planned |
| M3 | Agent SDK: deterministic orchestration, event-sourced replay | planned |
| M4 | Sandboxes: one Docker container per run, warm pools, egress lockdown | planned |
| M5 | Gateways: LLM + tool gateways, budgets, idempotency keys | planned |
| M6 | Approvals: park/resume, action-hash binding | planned |
| M7 | Observability: SSE streaming, run trees, cost roll-up, dashboard | planned |

## Quickstart

```powershell
pwsh -File scripts/check.ps1   # fmt, vet, lint, test, build — the quality gate
```

## Tooling

Agentic-coding support: [codegraph](https://github.com/colbymchenry/codegraph)
builds a semantic code graph of this repo (MCP server registered with
Claude Code globally; index in the git-ignored `.codegraph/`, auto-syncs on
file changes). Rebuild any time with `codegraph init`.

## Documentation

- [`docs/superpowers/specs/2026-11-21-agent-execution-platform-design.md`](docs/superpowers/specs/2026-11-21-agent-execution-platform-design.md) — the approved design (vision)
- `docs/ARCHITECTURE.md` — what exists right now
- `docs/REFERENCE.md` — summary of the reference architecture
- `docs/STATE.md` — where work stands; read this first when resuming
- `docs/adr/` — decision records
- `docs/milestones/` — per-milestone write-ups (what was built, concepts taught)
- `demos/` — copy-pasteable demo scripts per milestone
