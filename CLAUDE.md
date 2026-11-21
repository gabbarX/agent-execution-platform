# CLAUDE.md — working agreement for this repo

Agent execution platform in Go: runs other teams' AI agents with fair
scheduling, Docker sandboxes, durable event-sourced execution, gated
model/tool access, approvals, and tracing. Vision:
`docs/superpowers/specs/2026-11-21-agent-execution-platform-design.md`.
Current state: `docs/STATE.md` — **read it first, every session.**

## Workflow

- Every milestone: brainstorm-refresher → written plan → TDD → demo →
  docs → commit. No skipping stages.
- A milestone is done only when its `demos/m<N>.md` runs green from the
  file alone, start to finish.
- Spikes are allowed to explore, but are thrown away; kept code goes back
  through plan + TDD.

## Quality Gate

- `pwsh -File scripts/check.ps1` (fmt, vet, lint, test, build) must pass
  before **every** commit. No exceptions, including docs-only commits.

## Code Style

- Stdlib-first: `net/http` with 1.22+ pattern routing, no web framework.
- No ORM — hand-written SQL via pgx; the schema is part of the lesson.
- Table-driven tests; test names say the behavior, not the function.
- Wrap errors with `fmt.Errorf("context: %w", err)`; never discard them.
- Prefer clarity over cleverness everywhere — the owner is learning Go.

## Docs Contract

- Update `docs/STATE.md` at the end of every session.
- Update `docs/ARCHITECTURE.md` whenever structure changes (it describes
  what exists, never what's planned beyond a "planned" marker).
- Any decision with a real alternative gets an ADR in `docs/adr/`.
- Each milestone gets `docs/milestones/M<N>-<topic>.md` + `demos/m<N>.md`.

## Teaching Contract

- Explain a concept (fencing, replay, outbox, …) in plain language
  **before** writing its code, and distill the explanation into the
  milestone doc. The owner must be able to re-derive the idea from the
  docs alone.

## Commit Style

- Conventional commits: `feat:` / `fix:` / `docs:` / `chore:` / `test:`.
- Body: one or two lines on *why*, not what.
- **Never add AI co-author trailers** (no `Co-Authored-By: Claude`).
- Git history is portfolio material; keep it clean and linear.
