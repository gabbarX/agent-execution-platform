package run

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestStoreCreateAssignsIDAndQueued(t *testing.T) {
	s := NewStore()
	r, existed := s.Create(&Run{Team: "a", Agent: "demo", Version: 1})
	if existed {
		t.Fatal("fresh create reported existed=true")
	}
	if !strings.HasPrefix(r.ID, "run_") || len(r.ID) != len("run_")+12 {
		t.Errorf("ID = %q, want run_ + 12 hex chars", r.ID)
	}
	if r.Status != StatusQueued {
		t.Errorf("Status = %s, want QUEUED", r.Status)
	}
	if r.CreatedAt.IsZero() {
		t.Error("CreatedAt not set")
	}
}

func TestStoreCreateIdempotentReturnsSameRun(t *testing.T) {
	s := NewStore()
	first, _ := s.Create(&Run{Team: "a", Agent: "demo", Version: 1, IdempotencyKey: "k1"})
	second, existed := s.Create(&Run{Team: "a", Agent: "demo", Version: 1, IdempotencyKey: "k1"})
	if !existed {
		t.Fatal("second create with same key reported existed=false")
	}
	if second.ID != first.ID {
		t.Errorf("IDs differ: %s vs %s", first.ID, second.ID)
	}
}

func TestStoreGetReturnsCopy(t *testing.T) {
	s := NewStore()
	created, _ := s.Create(&Run{Team: "a", Agent: "demo", Version: 1})
	got, ok := s.Get(created.ID)
	if !ok {
		t.Fatal("run not found")
	}
	got.Status = StatusFailed // mutate the copy
	again, _ := s.Get(created.ID)
	if again.Status != StatusQueued {
		t.Errorf("mutating a Get result leaked into the store: %s", again.Status)
	}
}

func TestStoreUpdateTransitions(t *testing.T) {
	s := NewStore()
	created, _ := s.Create(&Run{Team: "a", Agent: "demo", Version: 1})

	updated, err := s.Update(created.ID, func(r *Run) error {
		return r.Transition(StatusStarting)
	})
	if err != nil {
		t.Fatalf("legal transition errored: %v", err)
	}
	if updated.Status != StatusStarting {
		t.Errorf("Status = %s, want STARTING", updated.Status)
	}

	_, err = s.Update(created.ID, func(r *Run) error {
		return r.Transition(StatusSucceeded) // STARTING -> SUCCEEDED is illegal
	})
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("got %v, want ErrInvalidTransition", err)
	}
	after, _ := s.Get(created.ID)
	if after.Status != StatusStarting {
		t.Errorf("state changed despite failed update: %s", after.Status)
	}
}

func TestStoreListFiltersByTeam(t *testing.T) {
	s := NewStore()
	s.Create(&Run{Team: "a", Agent: "demo", Version: 1})
	s.Create(&Run{Team: "b", Agent: "demo", Version: 1})
	s.Create(&Run{Team: "a", Agent: "demo", Version: 1})

	if got := len(s.List("")); got != 3 {
		t.Errorf("List(\"\") = %d runs, want 3", got)
	}
	if got := len(s.List("a")); got != 2 {
		t.Errorf("List(\"a\") = %d runs, want 2", got)
	}
	if got := len(s.List("zzz")); got != 0 {
		t.Errorf("List(\"zzz\") = %d runs, want 0", got)
	}
}

func TestStoreCreateIdempotentConcurrent(t *testing.T) {
	s := NewStore()
	const n = 50
	ids := make([]string, n)
	existedCount := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, existed := s.Create(&Run{Team: "a", Agent: "demo", Version: 1, IdempotencyKey: "k"})
			mu.Lock()
			ids[i] = r.ID
			if existed {
				existedCount++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	unique := map[string]bool{}
	for _, id := range ids {
		unique[id] = true
	}
	if len(unique) != 1 {
		t.Errorf("got %d distinct runs for one idempotency key, want 1", len(unique))
	}
	if existedCount != n-1 {
		t.Errorf("existed=true count = %d, want %d", existedCount, n-1)
	}
}
