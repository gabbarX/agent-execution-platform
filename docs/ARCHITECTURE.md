# Architecture (Current State)

> This document describes what **exists right now**. It grows with each
> milestone. The vision lives in the design doc
> (`docs/superpowers/specs/2026-11-21-agent-execution-platform-design.md`);
> the reference architecture lives in `docs/REFERENCE.md`.

**As of:** M1 (walking skeleton)

## Module Layout

```
agent-execution-platform/
├── cmd/
│   └── controlplane/    the one binary: API + scheduler + worker pool
├── internal/
│   ├── run/             Run record, status state machine, in-memory Store
│   ├── registry/        agent name@version -> spec (seeded: demo@1)
│   ├── sched/           weighted fair scheduler (per-team FIFOs, WRR)
│   ├── worker/          goroutine pool + injected AgentFunc (FakeAgent)
│   ├── api/             HTTP handlers (stdlib pattern routing)
│   └── version/         platform version string
├── scripts/
│   └── check.ps1        the quality gate: fmt → vet → lint → test → build
├── docs/                all documentation (see README.md for the map)
├── demos/               per-milestone demo scripts
│
│   — planned, not yet created —
├── sdk/                 public Go SDK agents are written against (M3)
├── agents/              example agents (M3+)
└── deploy/              docker-compose for Postgres etc. (M2)
```

## Request Flow (M1)

```
POST /runs ──> api: validate team/agent, Store.Create (idempotency key)
                    └─> sched.Enqueue(team, runID)        [status QUEUED]
worker goroutine ──> sched.Next (weighted round-robin across teams)
                    └─> STARTING -> RUNNING -> agent steps -> SUCCEEDED
                        (Cancel: per-run context -> CANCELLED;
                         panic -> FAILED, worker survives)
```

All state is in memory; a process restart loses every run. That is M2's
problem, by design.

## The Quality Gate

`pwsh -File scripts/check.ps1` runs gofmt (fail on any unformatted file),
`go vet`, `golangci-lint`, `go test ./...`, `go build ./...` and exits
non-zero at the first failure. Every commit must pass it.

## Decisions

Recorded as ADRs in `docs/adr/`. Current: 0001 (Go), 0002 (vertical-slice
milestones), 0003 (Docker containers as the sandbox boundary).
