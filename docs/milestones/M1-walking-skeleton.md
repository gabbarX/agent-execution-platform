# M1 — Walking Skeleton

**Completed:** 2026-11-21 · **Demo:** [`demos/m1.md`](../../demos/m1.md)

## What Was Built

One process (`cmd/controlplane`) covering a run's whole life:

- `internal/run` — the `Run` record, a **validated state machine**
  (QUEUED→STARTING→RUNNING→{WAITING}→SUCCEEDED/FAILED/CANCELLED), and an
  in-memory `Store` with idempotent create.
- `internal/registry` — agent `name@version` → spec, seeded with
  `demo@1` (8 steps × 300 ms).
- `internal/sched` — weighted fair scheduler: per-team FIFOs, sorted
  round-robin serving up to `weight` runs per visit.
- `internal/worker` — a pool of N goroutines executing an injected
  `AgentFunc`; per-run cancellation; panics fail the run, not the worker.
- `internal/api` — `POST /runs`, `GET /runs`, `GET /runs/{id}`,
  `POST /runs/{id}/cancel` on stdlib pattern routing.

Measured in the demo: a 12-run `bulk` burst against 2 `interactive`
runs on one worker — both interactive runs finished while only 3 bulk
runs had (a global FIFO would have made them wait behind all 12).

## Concepts Taught

**A status is a state machine, not a string.** Every status change goes
through `Run.Transition`, which consults an explicit edge table. Illegal
moves (SUCCEEDED→RUNNING) return `ErrInvalidTransition` instead of
happening. This looks like ceremony in M1; it becomes load-bearing in M2,
when a crashed-and-resurrected worker will *try* exactly those illegal
writes and must bounce off.

**Weighted fair queuing.** One global FIFO lets whoever submits most run
first — a burst starves everyone. Instead: one FIFO per team, and a
round-robin cursor over teams (sorted order), serving up to `weight` runs
per visit. Fairness becomes a structural property of the scheduler, not a
policy bolted on. The cursor persists across calls — that's what makes
`a,a,b,a,a,b` come out for weights 2:1.

**Worker pools and context cancellation.** N goroutines loop
`Next → execute`. Capacity is the pool size; the queue absorbs overflow.
Cancellation is Go-idiomatic: each run gets a child `context.Context`;
`Cancel(runID)` cancels it; the agent's `select` on `ctx.Done()` stops
work promptly; the pool maps `context.Canceled` to CANCELLED. The pool's
own context (from `signal.NotifyContext`) shuts everything down on Ctrl-C.

**Idempotency keys.** Clients retry on timeouts; without a key, a retry
double-submits. `Create` checks the key under the same lock that inserts
— the 50-goroutine test proves one run per key. In M2 this becomes a
unique index; the API behavior stays identical.

**Stores hand out copies.** `Get`/`List`/`Update` never leak internal
pointers; mutation happens only inside `Update` under the lock. Without
this, an API handler reading `run.Status` while a worker writes
`StepsDone` is a data race. (Caveat: we couldn't run `go test -race`
locally — no C toolchain; see backlog.)

**Injected dependencies as seams.** The pool takes an `AgentFunc`, not a
hardcoded agent. Tests inject a panicking agent; M3 injects SDK-driven
agents; M4 injects "start a container". The seam was designed now, cheap.

## What We'd Do Differently

- `-race` needs cgo+gcc on Windows; backlog: mingw or Linux CI.
- Cancel has a benign race window (queued→dequeued during cancel); the
  API falls through to pool-cancel, but a run can slip through if it's
  between `sched.Next` and `STARTING`. Honest M1 limitation; M2's
  lease/epoch machinery is the systematic fix.

## Open Threads for M2

- Everything dies with the process — **the** M2 motivation. Postgres
  `runs` + `run_events`, transactional outbox, leases + fencing epochs.
- Registry gains `POST /agents`; store's idempotency key becomes a
  unique index.
