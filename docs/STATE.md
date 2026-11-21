# Project State

> Read this first when resuming work. Keep it current at every session end.

**Last updated:** 2026-11-21

## Done

- Design doc approved: `docs/superpowers/specs/2026-11-21-agent-execution-platform-design.md`
- **M0 — repo foundation** ([doc](milestones/M0-foundation.md),
  [demo](../demos/m0.md)): Go module, quality gate (`scripts/check.ps1`),
  docs skeleton, ADRs 0001–0003, CLAUDE.md, codegraph tooling

## In Progress

- Nothing — between milestones.

## Next Up

- **M1 — walking skeleton**: `POST /runs` / `GET /runs/{id}` /
  `POST /runs/{id}/cancel`, in-memory per-team queues with weighted fair
  scheduling, in-process worker pool running a built-in fake agent, run
  state machine with validated transitions, idempotency keys on run
  creation, built-in agent registry (`{agent, version}` in the request).
  Spec: design doc § Milestones → M1. Start with a brainstorm-refresher,
  then a plan via superpowers:writing-plans.

## How to Resume a Session

1. Read this file, then `docs/ARCHITECTURE.md`, then the current
   milestone's doc in `docs/milestones/`.
2. Run `pwsh -File scripts/check.ps1` — it must be green before you start.
3. Each milestone follows: brainstorm-refresher → plan → TDD → demo →
   docs → commit. Never claim a milestone done without its demo passing.
