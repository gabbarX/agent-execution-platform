// Package worker runs scheduled runs on a fixed pool of goroutines.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/gabbarX/agent-execution-platform/internal/registry"
	"github.com/gabbarX/agent-execution-platform/internal/run"
	"github.com/gabbarX/agent-execution-platform/internal/sched"
)

type Pool struct {
	n     int
	sched *sched.Scheduler
	store *run.Store
	reg   *registry.Registry
	agent AgentFunc

	mu      sync.Mutex
	cancels map[string]context.CancelFunc // runID -> cancel for in-flight runs
}

func NewPool(n int, s *sched.Scheduler, store *run.Store, reg *registry.Registry, agent AgentFunc) *Pool {
	return &Pool{
		n:       n,
		sched:   s,
		store:   store,
		reg:     reg,
		agent:   agent,
		cancels: make(map[string]context.CancelFunc),
	}
}

// Start launches n worker goroutines. They exit when ctx is done.
func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.n; i++ {
		go p.workerLoop(ctx)
	}
}

// Cancel interrupts an in-flight run. False if the run is not
// currently executing on any worker.
func (p *Pool) Cancel(runID string) bool {
	p.mu.Lock()
	cancel, ok := p.cancels[runID]
	p.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

func (p *Pool) workerLoop(ctx context.Context) {
	for {
		id, err := p.sched.Next(ctx)
		if err != nil {
			return // ctx done
		}
		p.execute(ctx, id)
	}
}

// execute drives one run through its lifecycle. Any panic from the
// agent is recovered: the run fails, the worker survives.
func (p *Pool) execute(ctx context.Context, id string) {
	// finish applies a terminal transition; a failure here means the
	// run was concurrently moved (e.g. cancelled) — log, don't crash.
	finish := func(fn func(r *run.Run) error) {
		if _, err := p.store.Update(id, fn); err != nil {
			log.Printf("worker: finishing run %s: %v", id, err)
		}
	}
	fail := func(msg string) {
		finish(func(r *run.Run) error {
			r.Error = msg
			return r.Transition(run.StatusFailed)
		})
	}

	cur, ok := p.store.Get(id)
	if !ok {
		return
	}
	spec, ok := p.reg.Lookup(cur.Agent, cur.Version)
	if !ok {
		// API validates on create, so this is a registry regression.
		finish(func(r *run.Run) error {
			if err := r.Transition(run.StatusStarting); err != nil {
				return err
			}
			r.Error = fmt.Sprintf("unknown agent %s@%d", cur.Agent, cur.Version)
			return r.Transition(run.StatusFailed)
		})
		return
	}

	// Register the cancel hook before any transition so the window in
	// which a cancel request can find neither a queued run nor a
	// registered one is as small as possible (M2's leases close it
	// completely).
	runCtx, cancel := context.WithCancel(ctx)
	p.mu.Lock()
	p.cancels[id] = cancel
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.cancels, id)
		p.mu.Unlock()
		cancel()
	}()

	if _, err := p.store.Update(id, func(r *run.Run) error {
		r.StepsTotal = spec.Steps
		return r.Transition(run.StatusStarting)
	}); err != nil {
		return // e.g. cancelled between dequeue and start
	}
	if _, err := p.store.Update(id, func(r *run.Run) error {
		return r.Transition(run.StatusRunning)
	}); err != nil {
		return
	}

	err := p.runAgent(runCtx, spec, id)
	switch {
	case err == nil:
		finish(func(r *run.Run) error {
			return r.Transition(run.StatusSucceeded)
		})
	case errors.Is(err, context.Canceled):
		finish(func(r *run.Run) error {
			return r.Transition(run.StatusCancelled)
		})
	default:
		fail(err.Error())
	}
}

// runAgent calls the agent function, converting panics into errors.
func (p *Pool) runAgent(ctx context.Context, spec registry.AgentSpec, id string) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("agent panicked: %v", v)
		}
	}()
	progress := func(done int) {
		if _, uerr := p.store.Update(id, func(r *run.Run) error {
			r.StepsDone = done
			return nil
		}); uerr != nil {
			log.Printf("worker: progress for run %s: %v", id, uerr)
		}
	}
	return p.agent(ctx, spec, progress)
}
