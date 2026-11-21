# M0 — Repo Foundation

**Completed:** 2026-11-21 · **Demo:** [`demos/m0.md`](../../demos/m0.md)

## What Was Built

- Go module `github.com/gabbarX/agent-execution-platform` with
  `internal/version` as the toolchain proof (TDD'd, trivially).
- `scripts/check.ps1` — the quality gate: gofmt → go vet → golangci-lint →
  go test → go build; exits non-zero at the first failure and names it.
  Verified to fail loudly on a formatting break and on a red test.
- Docs skeleton: `REFERENCE.md` (the article, distilled),
  `ARCHITECTURE.md` (current state only), `STATE.md` (session resume
  point), ADRs 0001–0003, milestone/demo conventions.
- `CLAUDE.md` working agreement for agent sessions.
- codegraph MCP (code-intelligence index) registered with Claude Code.

## Concepts Taught

**Why a quality gate, and why agents especially need one.** A gate is one
command that answers "is the repo healthy?" with an exit code. Humans
forget to run four tools in order; coding agents are worse — they
confidently commit whatever compiled last. Making the gate a single
script turns discipline into muscle memory (`check.ps1` before every
commit) and makes CI trivial later: CI just runs the same script.
Detail that bit us immediately: PowerShell doesn't propagate native exit
codes automatically — each step must check `$LASTEXITCODE` explicitly, or
the gate becomes a green rubber stamp.

**Why a Go module path looks like a URL.** Go has no central package
registry; the import path *is* the download location. Naming the module
`github.com/gabbarX/agent-execution-platform` before the repo is even
pushed means imports never churn when it goes public — renaming a module
later touches every import line in the codebase.

**What an ADR is.** A half-page record of a decision *that had a real
alternative*: context, decision, consequences. The value is interview-shaped:
"why Docker and not Firecracker?" has a written answer with the trade-off
stated (shared host kernel vs. per-run guest kernel) instead of a shrug.
ADRs are immutable; a changed mind writes a superseding ADR.

**What codegraph does.** It parses the repo into a graph (functions,
types, call edges) an agent can query — "who calls X?" — instead of
grepping and reading whole files. Cheap now; pays off when `internal/`
has thirty packages.

## What We'd Do Differently

- codegraph's installer is interactive-first; non-interactive mode forces
  a global (user-scope) install. Fine, but repo-local config would have
  been more reproducible for a portfolio clone.

## Open Threads for M1

- `cmd/`, `sdk/`, `agents/` directories don't exist yet — M1 creates
  `cmd/controlplane`.
- M1 needs the run state machine: design doc § Milestones M1 lists the
  states and the fairness demo (team A burst must not starve team B).
