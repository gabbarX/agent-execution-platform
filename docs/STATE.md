# Project State

> Read this first when resuming work. Keep it current at every session end.

**Last updated:** 2026-11-21

## Done

- Design doc approved: `docs/superpowers/specs/2026-11-21-agent-execution-platform-design.md`
- **M0 — repo foundation** ([doc](milestones/M0-foundation.md), [demo](../demos/m0.md))
- **M1 — walking skeleton** ([doc](milestones/M1-walking-skeleton.md),
  [demo](../demos/m1.md)): run API, weighted fair scheduler, worker pool,
  validated state machine, idempotent create — all in-memory, one binary

## In Progress

- Nothing — between milestones.

## Next Up

- **M2 — durability**: Postgres via docker compose (`runs`, `run_events`
  append-only history, transactional outbox), worker leases with
  heartbeat/expiry, fencing epochs on every history append, checkpoint
  resume for the built-in agent, `POST /agents` registration API.
  Spec: design doc § Milestones → M2. Demo: kill the worker mid-run; a
  second worker finishes the run; a stale worker's writes are rejected.

## Backlog

- `go test -race` locally (needs mingw/gcc) or in CI.
- Pin golangci-lint version; module-scope the gofmt check; comment the
  `$LASTEXITCODE` footgun in check.ps1 (M0 review minors).

## How to Resume a Session

1. Read this file, then `docs/ARCHITECTURE.md`, then the current
   milestone's doc in `docs/milestones/`.
2. Run `pwsh -File scripts/check.ps1` — it must be green before you start.
3. Each milestone follows: brainstorm-refresher → plan → TDD → demo →
   docs → commit. Never claim a milestone done without its demo passing.
