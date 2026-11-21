# Project State

> Read this first when resuming work. Keep it current at every session end.

**Last updated:** 2026-11-21

## Done

- Design doc approved: `docs/superpowers/specs/2026-11-21-agent-execution-platform-design.md`
- M0 plan: `docs/superpowers/plans/2026-11-21-m0-repo-foundation.md`
- Go module + `internal/version` + quality gate (`scripts/check.ps1`)
- Docs skeleton (this file, ARCHITECTURE, REFERENCE, ADRs 0001–0003)

## In Progress

- M0 — remaining: CLAUDE.md, codegraph tooling, milestone doc + demo

## Next Up

- M1 — walking skeleton: run API, in-memory per-team queues with weighted
  fair scheduling, worker pool, run state machine (design doc § Milestones)

## How to Resume a Session

1. Read this file, then `docs/ARCHITECTURE.md`, then the current
   milestone's doc in `docs/milestones/`.
2. Run `pwsh -File scripts/check.ps1` — it must be green before you start.
3. Each milestone follows: brainstorm-refresher → plan → TDD → demo →
   docs → commit. Never claim a milestone done without its demo passing.
