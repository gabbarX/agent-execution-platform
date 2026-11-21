# M1 — Walking Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One binary (`cmd/controlplane`) that accepts runs over HTTP, schedules them fairly across teams from in-memory queues, and executes a built-in fake agent through a validated run state machine.

**Architecture:** Five small packages with one responsibility each: `internal/run` (domain: Run, Status, transitions, in-memory Store), `internal/registry` (agent specs), `internal/sched` (weighted-fair scheduler), `internal/worker` (pool + fake agent, injected as a function for testability), `internal/api` (HTTP handlers). `main.go` only wires them. Everything in memory; M2 replaces the Store and adds durability.

**Tech Stack:** Go 1.27 stdlib only — `net/http` 1.22+ pattern routing, `sync` primitives, `crypto/rand` IDs. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-11-21-agent-execution-platform-design.md` (§ Milestones → M1, § Decided Hows) + approved M1 calls: team as a body field, weights via flag, weighted round-robin only (no priority classes), extra `GET /runs` list endpoint, fake agent 8 steps × 300 ms.

## Global Constraints

- Go 1.27, Windows-compatible; stdlib-first — no web framework, no new deps.
- `pwsh -File scripts/check.ps1` green before **every** commit (CLAUDE.md).
- Conventional commits, why in body, **no AI trailers**.
- Table-driven tests; wrap errors with `%w`; clarity over cleverness.
- Status strings exactly: `QUEUED, STARTING, RUNNING, WAITING, SUCCEEDED, FAILED, CANCELLED` — WAITING is defined now (spec state machine) but unreachable until M6.
- JSON fields snake_case. Error responses: `{"error":"<message>"}`.

## Review Focus

- Two concurrent `POST /runs` with the same idempotency key must produce exactly one run — check-then-create race. (Pinned: Task 2 Step 1 `TestStoreCreateIdempotentConcurrent`.)
- Cancel on a terminal run must return 409 and leave state untouched. (Pinned: Task 6 Step 1 `TestCancelTerminalRunConflicts`.)
- Unknown agent or version must 400 with a clear message and enqueue nothing. (Pinned: Task 6 Step 1 `TestCreateRunUnknownAgent`.)
- A team absent from the weights flag must still be scheduled (default weight 1), never silently dropped. (Pinned: Task 4 Step 1 `TestSchedulerUnknownTeamDefaultWeight`.)
- A panicking agent must mark its run FAILED (error recorded) and the worker must survive to serve the next run. (Pinned: Task 5 Step 1 `TestPoolRecoversFromAgentPanic`.)

---

### Task 1: Run domain — Status, transitions, Run struct

**Files:**
- Create: `internal/run/run.go`
- Test: `internal/run/run_test.go`

**Interfaces:**
- Produces:
  - `type Status string`; consts `StatusQueued/StatusStarting/StatusRunning/StatusWaiting/StatusSucceeded/StatusFailed/StatusCancelled` with the exact strings above.
  - `func (s Status) Terminal() bool`
  - `func (s Status) CanTransitionTo(to Status) bool`
  - `var ErrInvalidTransition = errors.New("invalid transition")`
  - `type Run struct { ID, Team, Agent string; Version int; Input string; Status Status; IdempotencyKey string; StepsTotal, StepsDone int; Error string; CreatedAt, StartedAt, FinishedAt time.Time }` — JSON tags snake_case; `StartedAt`/`FinishedAt`/`Error`/`Input` tagged `omitempty`; `IdempotencyKey` tagged `json:"-"`.
  - `func (r *Run) Transition(to Status) error` — rejects illegal edges wrapping `ErrInvalidTransition` with from/to in the message; sets `StartedAt` on →RUNNING and `FinishedAt` on →terminal.

- [ ] **Step 1: Write the failing tests**

Table-driven `TestStatusCanTransitionTo` covering every pair. Legal edges only: QUEUED→{STARTING, CANCELLED}; STARTING→{RUNNING, FAILED, CANCELLED}; RUNNING→{WAITING, SUCCEEDED, FAILED, CANCELLED}; WAITING→{QUEUED, FAILED, CANCELLED}; terminals→nothing. Plus `TestTransitionSetsTimestamps` (→RUNNING sets StartedAt; →SUCCEEDED sets FinishedAt) and `TestTransitionRejectsIllegal`:

```go
err := (&Run{Status: StatusSucceeded}).Transition(StatusRunning)
if !errors.Is(err, ErrInvalidTransition) { t.Fatalf(...) }
```

- [ ] **Step 2: Run to verify failure** — `go test ./internal/run/` → FAIL (package missing).
- [ ] **Step 3: Implement** — transition table as `map[Status][]Status` (or map-of-sets); `Transition` validates then mutates.
- [ ] **Step 4: Verify green** — `go test ./internal/run/ -v` → PASS.
- [ ] **Step 5: Commit** — `feat: add run domain with validated state machine`

### Task 2: In-memory Store with idempotency

**Files:**
- Create: `internal/run/store.go`
- Test: `internal/run/store_test.go`

**Interfaces:**
- Consumes: Task 1 `Run`, `Status`.
- Produces:
  - `func NewStore() *Store`
  - `func (s *Store) Create(r *Run) (*Run, bool)` — assigns `ID` (`"run_"+12 hex chars` via crypto/rand), `CreatedAt`, `Status=StatusQueued`; if `r.IdempotencyKey != ""` and already seen, returns the existing run and `true` (existed). Returns a copy.
  - `func (s *Store) Get(id string) (*Run, bool)` — copy, never the stored pointer.
  - `func (s *Store) List(team string) []*Run` — copies; `team==""` means all; sorted by CreatedAt then ID.
  - `func (s *Store) Update(id string, fn func(*Run) error) (*Run, error)` — applies fn to the stored run under the lock; fn's error aborts and propagates; returns a copy. All post-create mutation goes through Update.

- [ ] **Step 1: Write the failing tests**

`TestStoreCreateAssignsIDAndQueued`; `TestStoreCreateIdempotentReturnsSameRun` (two Creates, same key → same ID, second existed=true); `TestStoreGetReturnsCopy` (mutating the returned Run then Get again → unchanged); `TestStoreUpdateTransitions` (Update with `fn = r.Transition(StatusStarting)` → status changed; illegal transition error propagates, state unchanged); `TestStoreListFiltersByTeam`; and the Review Focus race:

```go
func TestStoreCreateIdempotentConcurrent(t *testing.T) {
    // 50 goroutines Create with key "k"; expect exactly 1 unique ID
    // and 49 existed=true. Run with -race.
}
```

- [ ] **Step 2: Verify failure** — `go test ./internal/run/ -race` → FAIL.
- [ ] **Step 3: Implement** — single `sync.Mutex`; maps `runs map[string]*Run`, `byKey map[string]string`.
- [ ] **Step 4: Verify green** — `go test ./internal/run/ -race -v` → PASS.
- [ ] **Step 5: Commit** — `feat: add in-memory run store with idempotent create`

### Task 3: Agent registry

**Files:**
- Create: `internal/registry/registry.go`
- Test: `internal/registry/registry_test.go`

**Interfaces:**
- Produces:
  - `type AgentSpec struct { Name string; Version int; Steps int; StepDelay time.Duration }`
  - `func New() *Registry` — pre-seeded with `AgentSpec{Name: "demo", Version: 1, Steps: 8, StepDelay: 300 * time.Millisecond}`.
  - `func (g *Registry) Register(spec AgentSpec)`
  - `func (g *Registry) Lookup(name string, version int) (AgentSpec, bool)`

- [ ] **Step 1: Write the failing tests** — `TestRegistrySeedsDemoAgent` (Lookup("demo",1) → Steps 8, StepDelay 300ms); `TestRegistryLookupUnknown` (("demo",2) and ("nope",1) → false); `TestRegistryRegisterThenLookup`.
- [ ] **Step 2: Verify failure** — `go test ./internal/registry/` → FAIL.
- [ ] **Step 3: Implement** — map keyed `name@version` string or struct key; mutex for safety.
- [ ] **Step 4: Verify green** — PASS.
- [ ] **Step 5: Commit** — `feat: add in-memory agent registry seeded with demo agent`

### Task 4: Weighted-fair scheduler

**Files:**
- Create: `internal/sched/sched.go`
- Test: `internal/sched/sched_test.go`

**Interfaces:**
- Produces:
  - `func New(weights map[string]int) *Scheduler` — nil map fine; weight < 1 treated as 1.
  - `func (s *Scheduler) Enqueue(team, runID string)`
  - `func (s *Scheduler) Next(ctx context.Context) (string, error)` — blocks until an item or ctx done (then `("", ctx.Err())`).
  - `func (s *Scheduler) Remove(runID string) bool` — pulls a still-queued run out (cancel path).

Algorithm (pinned so tests are deterministic): round-robin over team names in **sorted order**, serving up to `weight` items from a team's FIFO per visit, skipping empty queues. Blocking: `sync.Cond` broadcast on Enqueue, plus a goroutine per Next call (or ctx watcher) to wake on cancellation — implementer's choice as long as Next honors ctx.

- [ ] **Step 1: Write the failing tests**

All drain synchronously (enqueue first, then repeated `Next` with a background ctx) so no timing:

```go
// TestSchedulerInterleavesEqualWeights: weights a=1,b=1.
// Enqueue a1..a10 then b1,b2. Expect b1 at position 1 and b2 at
// position 3 of the dequeue sequence (a,b,a,b,a,a,...).
// TestSchedulerHonorsWeights: weights a=2,b=1. Enqueue a1..a6, b1..b3.
// Expect exact sequence a,a,b,a,a,b,a,a,b.
// TestSchedulerUnknownTeamDefaultWeight: weights only {"a":5}.
// Enqueue a1..a5 and z1 (team "z" not configured). z1 must appear
// within the first 6 dequeues.
// TestSchedulerRemove: enqueue a1,a2; Remove("a1") → true;
// Next returns a2; Remove("a1") again → false.
// TestSchedulerNextHonorsContext: cancelled ctx → ("", context.Canceled).
// TestSchedulerNextBlocksUntilEnqueue: Next in a goroutine, Enqueue
// after 10ms, assert it returns that run (with 2s safety timeout).
```

- [ ] **Step 2: Verify failure** — `go test ./internal/sched/ -race` → FAIL.
- [ ] **Step 3: Implement** per the pinned algorithm.
- [ ] **Step 4: Verify green** — `go test ./internal/sched/ -race -v` → PASS.
- [ ] **Step 5: Commit** — `feat: add weighted fair scheduler over per-team queues`

### Task 5: Fake agent + worker pool

**Files:**
- Create: `internal/worker/pool.go`, `internal/worker/fakeagent.go`
- Test: `internal/worker/pool_test.go`

**Interfaces:**
- Consumes: Task 2 `*run.Store` (Update/Get), Task 3 `*registry.Registry` (Lookup), Task 4 `*sched.Scheduler` (Next).
- Produces:
  - `type AgentFunc func(ctx context.Context, spec registry.AgentSpec, progress func(done int)) error`
  - `func FakeAgent(ctx context.Context, spec registry.AgentSpec, progress func(done int)) error` — spec.Steps iterations: select on ctx.Done() → return ctx.Err(); else sleep StepDelay, call progress(i+1).
  - `func NewPool(n int, s *sched.Scheduler, store *run.Store, reg *registry.Registry, agent AgentFunc) *Pool`
  - `func (p *Pool) Start(ctx context.Context)` — n goroutines, each loops Next→execute; all exit on ctx done.
  - `func (p *Pool) Cancel(runID string) bool` — cancels the in-flight run's context; false if not currently executing.

Per-run execution (pinned): Update→STARTING, set StepsTotal from spec, Update→RUNNING, call agent with a per-run cancellable ctx (registered for Cancel), progress callback does `Update` bumping StepsDone; agent returns nil → SUCCEEDED; returns context.Canceled → CANCELLED; other error → FAILED with Error set; **panic recovered** → FAILED with Error `"agent panicked: <v>"`. Registry miss → FAILED. The run's ctx derives from Start's ctx.

- [ ] **Step 1: Write the failing tests**

Register a test agent (`Steps: 3, StepDelay: time.Millisecond`). Helper `waitTerminal(t, store, id)` polls Get every 5 ms up to 2 s.

```go
// TestPoolRunsToSuccess: create+enqueue one run → terminal SUCCEEDED,
// StepsDone==3, StepsTotal==3, StartedAt/FinishedAt nonzero.
// TestPoolCancelMidRun: agent Steps=1000; wait until StepsDone>=1,
// Pool.Cancel(id)==true → terminal CANCELLED, StepsDone<1000.
// TestPoolRecoversFromAgentPanic: inject an AgentFunc that panics
// when spec.Name=="boom" and behaves normally otherwise. On a
// 1-worker pool enqueue a boom-run then a normal run → boom-run
// FAILED with Error containing "panicked", normal run SUCCEEDED
// (the worker survived the panic).
// TestPoolStopsOnContextCancel: cancel Start's ctx; Next unblocks;
// no goroutine leak assertion needed — just that enqueued-after work
// stays QUEUED.
```

- [ ] **Step 2: Verify failure** — `go test ./internal/worker/ -race` → FAIL.
- [ ] **Step 3: Implement** `fakeagent.go` then `pool.go` per the pinned execution path.
- [ ] **Step 4: Verify green** — `go test ./internal/worker/ -race -v` → PASS.
- [ ] **Step 5: Commit** — `feat: add worker pool executing injected agent functions`

### Task 6: HTTP API + controlplane binary

**Files:**
- Create: `internal/api/server.go`, `cmd/controlplane/main.go`
- Test: `internal/api/server_test.go`, `cmd/controlplane/main_test.go`

**Interfaces:**
- Consumes: Tasks 2–5 exactly as produced (`store.Create/Get/List/Update`, `reg.Lookup`, `sched.Enqueue/Remove`, `pool.Cancel`).
- Produces:
  - `func NewServer(store *run.Store, reg *registry.Registry, s *sched.Scheduler, pool *worker.Pool) http.Handler`
  - Routes (1.22 patterns): `POST /runs`, `GET /runs`, `GET /runs/{id}`, `POST /runs/{id}/cancel`.
  - `func parseWeights(s string) (map[string]int, error)` in main.go — `"a=2,b=1"`; empty string → empty map; malformed → error.

API contract (pinned):
- `POST /runs` body `{"team":"a","agent":"demo","version":1,"input":"...","idempotency_key":"..."}` (input/key optional). Missing/empty team or agent → 400. Unknown agent@version → 400 `{"error":"unknown agent demo@2"}`, nothing created or enqueued. New run → 201 + run JSON, enqueued. Same idempotency key → 200 + the existing run, **not** re-enqueued.
- `GET /runs?team=a` → 200 `{"runs":[...]}`; no filter → all.
- `GET /runs/{id}` → 200 run JSON or 404.
- `POST /runs/{id}/cancel` → 404 unknown; terminal → 409 `{"error":"run already SUCCEEDED"}`; QUEUED and `sched.Remove` succeeded → transition to CANCELLED, 200 + run JSON; otherwise (executing) → `pool.Cancel`, 202 + current run JSON (cancellation is async).
- `main.go` flags: `-addr :8080`, `-workers 4`, `-weights ""`. Wiring: registry → store → scheduler → pool → server; `signal.NotifyContext` for Ctrl-C; `http.Server.Shutdown` then ctx cancel stops workers. Startup log: addr, workers, weights.

- [ ] **Step 1: Write the failing tests**

`httptest` against `NewServer` with a **non-started** pool (so runs stay QUEUED unless a test starts it): `TestCreateRunReturns201Queued`; `TestCreateRunIdempotentReturns200SameID`; `TestCreateRunUnknownAgent` (400, body contains "unknown agent", GET /runs shows zero runs); `TestCreateRunMissingTeam` (400); `TestGetRunNotFound` (404); `TestListRunsFiltersByTeam`; `TestCancelQueuedRun` (200, status CANCELLED); `TestCancelTerminalRunConflicts` (drive a run to SUCCEEDED via store.Update, cancel → 409, status still SUCCEEDED); `TestCancelUnknownRun` (404). In main_test.go: table-driven `TestParseWeights` (valid, empty, malformed "a=", "a=x", negative→error).

- [ ] **Step 2: Verify failure** — `go test ./internal/api/ ./cmd/controlplane/` → FAIL.
- [ ] **Step 3: Implement** `server.go` handlers, then `main.go`.
- [ ] **Step 4: Verify green** — `go test ./... -race` → PASS (whole suite).
- [ ] **Step 5: Manual smoke** — start `go run ./cmd/controlplane`, `curl -s -X POST localhost:8080/runs -d '{"team":"a","agent":"demo","version":1}'` → 201 JSON; watch `GET /runs/{id}` progress to SUCCEEDED (~2.4 s); Ctrl-C exits cleanly.
- [ ] **Step 6: Commit** — `feat: add run API and controlplane binary`

### Task 7: Demo + milestone close-out

**Files:**
- Create: `demos/m1.md`, `docs/milestones/M1-walking-skeleton.md`
- Modify: `internal/version/version.go` + test (`0.1.0-m1`), `docs/STATE.md`, `docs/ARCHITECTURE.md` (real package layout now), `README.md` (M1 ✅)

**Interfaces:**
- Consumes: the running binary from Task 6.

- [ ] **Step 1: Bump version via TDD** — change test to expect `"0.1.0-m1"`, watch fail, bump const, green.
- [ ] **Step 2: Write `demos/m1.md`** — (a) lifecycle: start server (`-workers 2`), create a run, poll `GET /runs/{id}` showing QUEUED→RUNNING (steps_done climbing)→SUCCEEDED; (b) cancel: create, cancel while RUNNING → CANCELLED; idempotent create shown returning same id; (c) **fairness**: restart with `-workers 1 -weights "bulk=1,interactive=1"`, PowerShell loop POSTs 12 `bulk` runs then 2 `interactive` runs; `GET /runs?team=interactive` shows both finish long before the bulk backlog drains (expected: interactive runs are the 2nd and 4th to complete). Exact commands + expected outputs.
- [ ] **Step 3: Run the demo start to finish** — every command, outputs must match.
- [ ] **Step 4: Write `docs/milestones/M1-walking-skeleton.md`** — what was built; concepts taught: state machines as legal-edge graphs (why SUCCEEDED→RUNNING must not compile-or-run), weighted fair queuing vs global FIFO (the starvation demo), worker pools and context cancellation, idempotency keys, returning copies from a store to prevent data races; demo link; open threads for M2 (Store interface extraction, everything-in-memory loses runs on restart — the M2 hook).
- [ ] **Step 5: Update STATE.md / ARCHITECTURE.md / README.md** — M1 done, M2 next.
- [ ] **Step 6: Final gate + commit** — `pwsh -File scripts/check.ps1` → ALL CHECKS PASSED; `docs: close out M1 — walking skeleton complete`.
