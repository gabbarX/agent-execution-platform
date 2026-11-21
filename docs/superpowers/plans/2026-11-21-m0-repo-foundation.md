# M0 — Repo Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the empty repo into a documented, lint-and-test-gated Go project that any future session (human or agent) can pick up cold.

**Architecture:** Single Go module monorepo. One `scripts/check.ps1` gate (fmt → vet → lint → test → build) that every milestone must keep green. Docs skeleton carries the project's memory: REFERENCE (the article), ARCHITECTURE, STATE, ADRs, milestone docs, demo scripts. CLAUDE.md tells coding agents how to work here.

**Tech Stack:** Go 1.27, golangci-lint v2 (via `go install`), PowerShell 7 for scripts, git, codegraph MCP (npm) for agent code navigation.

**Spec:** `docs/superpowers/specs/2026-11-21-agent-execution-platform-design.md`

## Global Constraints

- Go 1.27; everything must build/run on Windows 11 (PowerShell scripts, no Makefile — `make` is not installed on this machine).
- Module path: `github.com/<GH_USERNAME>/agent-execution-platform` — `<GH_USERNAME>` is supplied by the owner before Task 1 executes.
- Docs are deliverables: a task isn't done if its doc step is skipped.
- Git history is portfolio material: conventional-commit messages (`feat:`, `docs:`, `chore:`), each commit message body says *why* in one or two lines.
- `docs/REFERENCE.md` content comes from the reference article text, which exists only in the planning conversation — the executor must have that context (reason this plan recommends native execution).

## Review Focus

- A failing `go test` or a formatting diff must make `scripts/check.ps1` exit non-zero — PowerShell silently swallowing native exit codes is the classic bug. (Pinned by Task 2 Step 5.)
- CRLF line endings on Windows corrupt `gofmt` checks and pollute diffs — `.gitattributes` must force LF for `.go` files. (Pinned by Task 1 Step 2 + Task 2 Step 5.)
- `scripts/check.ps1` must run under default execution policy via documented invocation. (Pinned by Task 2 Step 4 using `pwsh -File`.)
- Codegraph install may fail on Windows; the milestone must not block on it — fallback is a documented skip. (Pinned by Task 5 Step 3.)
- A fresh clone with only Go installed must get a green check — no undeclared tool dependencies in the gate script; golangci-lint missing should produce a clear install instruction, not a cryptic error. (Pinned by Task 2 Step 3.)

---

### Task 1: Git repo + Go module + first package

**Files:**
- Create: `.gitignore`, `.gitattributes`, `go.mod`, `internal/version/version.go`, `internal/version/version_test.go`, `README.md`

**Interfaces:**
- Produces: `version.String() string` returning `"0.0.1-m0"`; module path `github.com/<GH_USERNAME>/agent-execution-platform` used by all future imports.

- [ ] **Step 1: Initialize repo and module**

Run: `git init -b main` then `go mod init github.com/<GH_USERNAME>/agent-execution-platform`

- [ ] **Step 2: Write `.gitignore` and `.gitattributes`**

`.gitignore`: `*.exe`, `/bin/`, `.env`, `coverage.out`, `.codegraph/` (verify actual codegraph artifact dir in Task 5 and adjust).
`.gitattributes`:
```
* text=auto
*.go text eol=lf
*.ps1 text eol=crlf
```

- [ ] **Step 3: Write the failing test**

```go
// internal/version/version_test.go
func TestVersionString(t *testing.T) {
    if got := version.String(); got != "0.0.1-m0" {
        t.Errorf("version.String() = %q, want %q", got, "0.0.1-m0")
    }
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/version/`
Expected: FAIL (package doesn't exist yet)

- [ ] **Step 5: Implement `version.String() string` in `internal/version/version.go`**

Const-backed; bumped manually at each milestone.

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 7: Write `README.md`**

Contents: one-paragraph project pitch (platform that runs other teams' AI agents: scheduling, sandboxes, durable execution, gateways, approvals, tracing — built as a learning/portfolio project); milestone status table (M0–M7, M0 "in progress", rest "planned"); quickstart (`pwsh -File scripts/check.ps1` — forward reference, lands in Task 2); link to `docs/` and the design doc.

- [ ] **Step 8: Commit**

`git add -A && git commit` — `chore: initialize Go module with version package`

### Task 2: Check gate script

**Files:**
- Create: `scripts/check.ps1`, `.golangci.yml`

**Interfaces:**
- Produces: `pwsh -File scripts/check.ps1` — the single quality gate every future task runs before committing. Exits 0 only if fmt, vet, lint, test, build all pass.

- [ ] **Step 1: Install golangci-lint**

Run: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`
Verify: `golangci-lint --version` works (it lands in `$(go env GOPATH)\bin`; add to PATH note in README if not resolvable).

- [ ] **Step 2: Write `.golangci.yml`**

Minimal v2 config: `version: "2"`, default linters enabled, nothing exotic. One comment line saying linters get tightened as the codebase grows.

- [ ] **Step 3: Write `scripts/check.ps1`**

Sequence: `gofmt -l .` (fail if output non-empty, print offending files), `go vet ./...`, `golangci-lint run` (if binary missing: print `go install` instruction and fail), `go test ./...`, `go build ./...`. Each step: echo a header, run, check `$LASTEXITCODE`, `exit 1` on failure. Final line prints `ALL CHECKS PASSED`.

- [ ] **Step 4: Run to verify it passes**

Run: `pwsh -File scripts/check.ps1`
Expected: exit code 0, `ALL CHECKS PASSED`

- [ ] **Step 5: Verify it fails loudly**

Temporarily break formatting in `version.go` (add stray spaces), run script → must exit 1 naming the file. Revert. Temporarily make the test assert a wrong value, run → must exit 1. Revert, rerun, green.

- [ ] **Step 6: Commit**

`chore: add check gate (fmt, vet, lint, test, build)`

### Task 3: Docs skeleton

**Files:**
- Create: `docs/REFERENCE.md`, `docs/ARCHITECTURE.md`, `docs/STATE.md`, `docs/adr/README.md`, `docs/adr/0001-go.md`, `docs/adr/0002-vertical-slices.md`, `docs/adr/0003-docker-sandboxes.md`, `docs/milestones/README.md`, `demos/README.md`

**Interfaces:**
- Produces: the doc locations every later milestone appends to; ADR template in `docs/adr/README.md`.

- [ ] **Step 1: Write `docs/REFERENCE.md`**

Faithful ~2-page summary of the reference article from the planning conversation: requirements, estimation highlights, core APIs, high-level design (5 steps), data design, all four deep dives (durable execution incl. the 4 recovery approaches table, sandbox lifecycle, credentials, tracing), scale/cost. Note at top: article supplied privately, no public citation.

- [ ] **Step 2: Write `docs/ARCHITECTURE.md`**

Current-state only (M0): module layout diagram (`cmd/`, `internal/`, `sdk/`, `agents/` — noted as planned), the check gate, and a "grows with each milestone" banner. No aspirational content — ARCHITECTURE describes what exists; the design doc holds the vision.

- [ ] **Step 3: Write `docs/STATE.md`**

Template: "Done" (list), "In progress", "Next up", "How to resume a session" (read STATE → ARCHITECTURE → current milestone doc; run check script). Fill for M0-in-progress.

- [ ] **Step 4: Write ADR template + ADRs 0001–0003**

`docs/adr/README.md`: numbered-file convention, template (Context / Decision / Consequences, ~half page max).
0001: Go over TS/Python (owner pick; infra-domain fit). 0002: vertical slices over component-order and over Temporal (demoable every milestone; learning value). 0003: Docker containers as the sandbox boundary, honest trade-off vs Firecracker (shared kernel, Linux-only constraint of microVMs on this host).

- [ ] **Step 5: Write `docs/milestones/README.md` and `demos/README.md`**

Milestone doc convention: `M<N>-<topic>.md` = what was built, concepts taught (plain language), demo link, what we'd do differently. Demos: `m<N>.md` = copy-pasteable commands + expected output.

- [ ] **Step 6: Commit**

`docs: add reference summary, architecture, state, ADRs 0001-0003`

### Task 4: CLAUDE.md agent guidelines

**Files:**
- Create: `CLAUDE.md`

**Interfaces:**
- Produces: the rules every future coding-agent session loads automatically.

- [ ] **Step 1: Write `CLAUDE.md`**

Sections, each a few lines: project one-liner + pointer to design doc/STATE.md; **workflow** (read STATE.md first; every milestone: brainstorm-refresher → plan → TDD → demo → docs → commit; never claim a milestone done without its demo script passing); **quality gate** (`pwsh -File scripts/check.ps1` before every commit); **code style** (stdlib-first, no ORM, hand-written SQL, table-driven tests, wrap errors with `%w` and context); **docs contract** (update STATE.md every session end; ARCHITECTURE.md when structure changes; ADR for any decision with a real alternative); **teaching contract** (explain concepts before coding them; owner is learning — prefer clarity over cleverness); **commit style** (conventional commits, why in the body).

- [ ] **Step 2: Verify against this plan**

Read back: does CLAUDE.md contradict anything in the design doc or this plan? Fix inline.

- [ ] **Step 3: Commit**

`docs: add CLAUDE.md agent working agreement`

### Task 5: Codegraph install

**Files:**
- Modify: `.gitignore` (codegraph artifacts), `README.md` (tooling section)

**Interfaces:**
- Produces: codegraph MCP registered with Claude Code for this repo, or a documented skip.

- [ ] **Step 1: Install**

Run: `npm install -g @colbymchenry/codegraph`, then `codegraph install` (registers MCP with Claude Code), then `codegraph init` in the repo root to build the index.

- [ ] **Step 2: Verify**

`codegraph` status/doctor command reports a healthy index; confirm the MCP server appears in Claude Code config. Add any generated local artifact dirs to `.gitignore`.

- [ ] **Step 3: Fallback (only if Step 1/2 fail on Windows)**

Try the alternative `code-graph-mcp` plugin (sdsrss). If both fail: document the failure mode in `docs/milestones/M0-foundation.md` under "Tooling", skip without blocking — the milestone's gate is `check.ps1`, not codegraph.

- [ ] **Step 4: Commit**

`chore: add codegraph code-intelligence tooling` (or `docs: record codegraph skip` on fallback)

### Task 6: M0 milestone doc + demo + close-out

**Files:**
- Create: `docs/milestones/M0-foundation.md`, `demos/m0.md`
- Modify: `docs/STATE.md`, `README.md` (status table: M0 done)

**Interfaces:**
- Produces: the pattern every later milestone's close-out follows.

- [ ] **Step 1: Write `demos/m0.md`**

Exact commands: fresh-eyes checkout assumptions, `pwsh -File scripts/check.ps1` → `ALL CHECKS PASSED`, `go test ./... -v` showing the version test, codegraph status (if installed). Each with expected output.

- [ ] **Step 2: Run the demo start to finish**

Execute every command in `demos/m0.md` exactly as written. All must match expected output.

- [ ] **Step 3: Write `docs/milestones/M0-foundation.md`**

What was built; concepts taught (what a quality gate is and why agents need one; why module path = repo URL in Go; what an ADR is; what codegraph does for agent navigation); demo link; open threads for M1.

- [ ] **Step 4: Update `docs/STATE.md` and README status table**

M0 → done; Next up → M1 walking skeleton (link design doc's M1 section).

- [ ] **Step 5: Final commit**

`docs: close out M0 — repo foundation complete`
