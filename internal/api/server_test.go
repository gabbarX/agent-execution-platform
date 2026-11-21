package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gabbarX/agent-execution-platform/internal/registry"
	"github.com/gabbarX/agent-execution-platform/internal/run"
	"github.com/gabbarX/agent-execution-platform/internal/sched"
	"github.com/gabbarX/agent-execution-platform/internal/worker"
)

type fixture struct {
	store  *run.Store
	srv    *httptest.Server
	schedr *sched.Scheduler
}

// newFixture builds a server whose pool is NOT started: created runs
// stay QUEUED, which keeps the HTTP tests deterministic.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	store := run.NewStore()
	reg := registry.New()
	s := sched.New(nil)
	pool := worker.NewPool(1, s, store, reg, worker.FakeAgent)
	srv := httptest.NewServer(NewServer(store, reg, s, pool))
	t.Cleanup(srv.Close)
	return &fixture{store: store, srv: srv, schedr: s}
}

func (f *fixture) do(t *testing.T, method, path, body string) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, f.srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decoding %s %s response: %v", method, path, err)
	}
	return resp, decoded
}

const validCreate = `{"team":"a","agent":"demo","version":1}`

func TestCreateRunReturns201Queued(t *testing.T) {
	f := newFixture(t)
	resp, body := f.do(t, "POST", "/runs", validCreate)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %v)", resp.StatusCode, body)
	}
	if body["status"] != "QUEUED" {
		t.Errorf("run status = %v, want QUEUED", body["status"])
	}
	if id, _ := body["id"].(string); !strings.HasPrefix(id, "run_") {
		t.Errorf("id = %v, want run_ prefix", body["id"])
	}
}

func TestCreateRunIdempotentReturns200SameID(t *testing.T) {
	f := newFixture(t)
	withKey := `{"team":"a","agent":"demo","version":1,"idempotency_key":"k1"}`
	resp1, body1 := f.do(t, "POST", "/runs", withKey)
	resp2, body2 := f.do(t, "POST", "/runs", withKey)
	if resp1.StatusCode != 201 || resp2.StatusCode != 200 {
		t.Fatalf("statuses = %d, %d; want 201, 200", resp1.StatusCode, resp2.StatusCode)
	}
	if body1["id"] != body2["id"] {
		t.Errorf("ids differ: %v vs %v", body1["id"], body2["id"])
	}
}

func TestCreateRunUnknownAgent(t *testing.T) {
	f := newFixture(t)
	resp, body := f.do(t, "POST", "/runs", `{"team":"a","agent":"demo","version":2}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "unknown agent") {
		t.Errorf("error = %v, want mention of unknown agent", body["error"])
	}
	if got := len(f.store.List("")); got != 0 {
		t.Errorf("store has %d runs after rejected create, want 0", got)
	}
}

func TestCreateRunMissingTeam(t *testing.T) {
	f := newFixture(t)
	resp, _ := f.do(t, "POST", "/runs", `{"agent":"demo","version":1}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestGetRunNotFound(t *testing.T) {
	f := newFixture(t)
	resp, _ := f.do(t, "GET", "/runs/run_doesnotexist", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestListRunsFiltersByTeam(t *testing.T) {
	f := newFixture(t)
	f.do(t, "POST", "/runs", `{"team":"a","agent":"demo","version":1}`)
	f.do(t, "POST", "/runs", `{"team":"b","agent":"demo","version":1}`)

	_, all := f.do(t, "GET", "/runs", "")
	if n := len(all["runs"].([]any)); n != 2 {
		t.Errorf("GET /runs = %d runs, want 2", n)
	}
	_, onlyA := f.do(t, "GET", "/runs?team=a", "")
	if n := len(onlyA["runs"].([]any)); n != 1 {
		t.Errorf("GET /runs?team=a = %d runs, want 1", n)
	}
}

func TestCancelQueuedRun(t *testing.T) {
	f := newFixture(t)
	_, created := f.do(t, "POST", "/runs", validCreate)
	id := created["id"].(string)

	resp, body := f.do(t, "POST", "/runs/"+id+"/cancel", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %v)", resp.StatusCode, body)
	}
	if body["status"] != "CANCELLED" {
		t.Errorf("status = %v, want CANCELLED", body["status"])
	}
}

func TestCancelTerminalRunConflicts(t *testing.T) {
	f := newFixture(t)
	_, created := f.do(t, "POST", "/runs", validCreate)
	id := created["id"].(string)

	_, err := f.store.Update(id, func(r *run.Run) error {
		for _, s := range []run.Status{run.StatusStarting, run.StatusRunning, run.StatusSucceeded} {
			if err := r.Transition(s); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, body := f.do(t, "POST", "/runs/"+id+"/cancel", "")
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "SUCCEEDED") {
		t.Errorf("error = %v, want mention of SUCCEEDED", body["error"])
	}
	after, _ := f.store.Get(id)
	if after.Status != run.StatusSucceeded {
		t.Errorf("cancel mutated a terminal run: %s", after.Status)
	}
}

func TestCancelUnknownRun(t *testing.T) {
	f := newFixture(t)
	resp, _ := f.do(t, "POST", "/runs/run_nope/cancel", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// Regression for the review's lost-cancel finding: with a live pool, a
// cancel against an executing run must actually end it CANCELLED.
func TestCancelRunningRunEndsCancelled(t *testing.T) {
	store := run.NewStore()
	reg := registry.New()
	reg.Register(registry.AgentSpec{Name: "slow", Version: 1, Steps: 1000, StepDelay: time.Millisecond})
	s := sched.New(nil)
	pool := worker.NewPool(1, s, store, reg, worker.FakeAgent)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx)
	srv := httptest.NewServer(NewServer(store, reg, s, pool))
	defer srv.Close()
	f := &fixture{store: store, srv: srv, schedr: s}

	_, created := f.do(t, "POST", "/runs", `{"team":"a","agent":"slow","version":1}`)
	id := created["id"].(string)

	deadline := time.Now().Add(2 * time.Second)
	for {
		cur, _ := store.Get(id)
		if cur.StepsDone >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("run never started")
		}
		time.Sleep(2 * time.Millisecond)
	}

	resp, _ := f.do(t, "POST", "/runs/"+id+"/cancel", "")
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel status = %d, want 202 or 200", resp.StatusCode)
	}

	for {
		cur, _ := store.Get(id)
		if cur.Status.Terminal() {
			if cur.Status != run.StatusCancelled {
				t.Fatalf("final status = %s, want CANCELLED", cur.Status)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("run never reached terminal state")
		}
		time.Sleep(2 * time.Millisecond)
	}
}
