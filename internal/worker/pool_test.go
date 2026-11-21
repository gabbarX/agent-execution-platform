package worker

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gabbarX/agent-execution-platform/internal/registry"
	"github.com/gabbarX/agent-execution-platform/internal/run"
	"github.com/gabbarX/agent-execution-platform/internal/sched"
)

func newFixture(agent AgentFunc, workers int) (*run.Store, *registry.Registry, *sched.Scheduler, *Pool) {
	store := run.NewStore()
	reg := registry.New()
	reg.Register(registry.AgentSpec{Name: "fast", Version: 1, Steps: 3, StepDelay: time.Millisecond})
	reg.Register(registry.AgentSpec{Name: "slow", Version: 1, Steps: 1000, StepDelay: time.Millisecond})
	reg.Register(registry.AgentSpec{Name: "boom", Version: 1, Steps: 3, StepDelay: time.Millisecond})
	s := sched.New(nil)
	p := NewPool(workers, s, store, reg, agent)
	return store, reg, s, p
}

func createAndEnqueue(store *run.Store, s *sched.Scheduler, agent string) *run.Run {
	r, _ := store.Create(&run.Run{Team: "t", Agent: agent, Version: 1})
	s.Enqueue(r.Team, r.ID)
	return r
}

func waitTerminal(t *testing.T, store *run.Store, id string) *run.Run {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r, ok := store.Get(id)
		if !ok {
			t.Fatalf("run %s vanished", id)
		}
		if r.Status.Terminal() {
			return r
		}
		time.Sleep(5 * time.Millisecond)
	}
	r, _ := store.Get(id)
	t.Fatalf("run %s never reached terminal state (status %s)", id, r.Status)
	return nil
}

func TestPoolRunsToSuccess(t *testing.T) {
	store, _, s, p := newFixture(FakeAgent, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	r := createAndEnqueue(store, s, "fast")
	final := waitTerminal(t, store, r.ID)

	if final.Status != run.StatusSucceeded {
		t.Errorf("Status = %s, want SUCCEEDED (error: %s)", final.Status, final.Error)
	}
	if final.StepsDone != 3 || final.StepsTotal != 3 {
		t.Errorf("steps = %d/%d, want 3/3", final.StepsDone, final.StepsTotal)
	}
	if final.StartedAt.IsZero() || final.FinishedAt.IsZero() {
		t.Error("StartedAt/FinishedAt not stamped")
	}
}

func TestPoolCancelMidRun(t *testing.T) {
	store, _, s, p := newFixture(FakeAgent, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	r := createAndEnqueue(store, s, "slow")

	deadline := time.Now().Add(2 * time.Second)
	for {
		cur, _ := store.Get(r.ID)
		if cur.StepsDone >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("run never made progress")
		}
		time.Sleep(2 * time.Millisecond)
	}

	if !p.Cancel(r.ID) {
		t.Fatal("Cancel returned false for an executing run")
	}
	final := waitTerminal(t, store, r.ID)
	if final.Status != run.StatusCancelled {
		t.Errorf("Status = %s, want CANCELLED", final.Status)
	}
	if final.StepsDone >= 1000 {
		t.Errorf("StepsDone = %d, run was not actually interrupted", final.StepsDone)
	}
}

func TestPoolRecoversFromAgentPanic(t *testing.T) {
	panicky := func(ctx context.Context, spec registry.AgentSpec, progress func(int)) error {
		if spec.Name == "boom" {
			panic("kaboom")
		}
		return FakeAgent(ctx, spec, progress)
	}
	store, _, s, p := newFixture(panicky, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	boom := createAndEnqueue(store, s, "boom")
	normal := createAndEnqueue(store, s, "fast")

	boomFinal := waitTerminal(t, store, boom.ID)
	if boomFinal.Status != run.StatusFailed {
		t.Errorf("boom run Status = %s, want FAILED", boomFinal.Status)
	}
	if !strings.Contains(boomFinal.Error, "panicked") {
		t.Errorf("boom run Error = %q, want it to mention panicked", boomFinal.Error)
	}

	normalFinal := waitTerminal(t, store, normal.ID)
	if normalFinal.Status != run.StatusSucceeded {
		t.Errorf("run after panic Status = %s, want SUCCEEDED (worker died?)", normalFinal.Status)
	}
}

func TestPoolStopsOnContextCancel(t *testing.T) {
	store, _, s, p := newFixture(FakeAgent, 2)
	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)
	cancel()
	time.Sleep(20 * time.Millisecond) // let workers observe cancellation

	r := createAndEnqueue(store, s, "fast")
	time.Sleep(50 * time.Millisecond)
	cur, _ := store.Get(r.ID)
	if cur.Status != run.StatusQueued {
		t.Errorf("run picked up by a stopped pool: %s", cur.Status)
	}
}

func TestFakeAgentReportsProgress(t *testing.T) {
	var got []int
	spec := registry.AgentSpec{Steps: 3, StepDelay: 0}
	err := FakeAgent(context.Background(), spec, func(done int) { got = append(got, done) })
	if err != nil {
		t.Fatalf("FakeAgent: %v", err)
	}
	want := fmt.Sprint([]int{1, 2, 3})
	if fmt.Sprint(got) != want {
		t.Errorf("progress = %v, want %s", got, want)
	}
}
