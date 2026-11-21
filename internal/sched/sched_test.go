package sched

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func drain(t *testing.T, s *Scheduler, n int) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id, err := s.Next(ctx)
		if err != nil {
			t.Fatalf("Next %d: %v", i, err)
		}
		out = append(out, id)
	}
	return out
}

func TestSchedulerInterleavesEqualWeights(t *testing.T) {
	s := New(map[string]int{"a": 1, "b": 1})
	for i := 1; i <= 10; i++ {
		s.Enqueue("a", fmt.Sprintf("a%d", i))
	}
	s.Enqueue("b", "b1")
	s.Enqueue("b", "b2")

	got := drain(t, s, 12)
	if got[1] != "b1" || got[3] != "b2" {
		t.Errorf("b runs not interleaved: positions 1,3 = %s,%s (full: %v)", got[1], got[3], got)
	}
}

func TestSchedulerHonorsWeights(t *testing.T) {
	s := New(map[string]int{"a": 2, "b": 1})
	for i := 1; i <= 6; i++ {
		s.Enqueue("a", fmt.Sprintf("a%d", i))
	}
	for i := 1; i <= 3; i++ {
		s.Enqueue("b", fmt.Sprintf("b%d", i))
	}

	want := []string{"a1", "a2", "b1", "a3", "a4", "b2", "a5", "a6", "b3"}
	got := drain(t, s, 9)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sequence mismatch at %d: got %v, want %v", i, got, want)
		}
	}
}

func TestSchedulerUnknownTeamDefaultWeight(t *testing.T) {
	s := New(map[string]int{"a": 5})
	for i := 1; i <= 5; i++ {
		s.Enqueue("a", fmt.Sprintf("a%d", i))
	}
	s.Enqueue("z", "z1")

	got := drain(t, s, 6)
	found := false
	for _, id := range got {
		if id == "z1" {
			found = true
		}
	}
	if !found {
		t.Errorf("unconfigured team never scheduled: %v", got)
	}
}

func TestSchedulerRemove(t *testing.T) {
	s := New(nil)
	s.Enqueue("a", "a1")
	s.Enqueue("a", "a2")

	if !s.Remove("a1") {
		t.Fatal("Remove(a1) = false, want true")
	}
	got := drain(t, s, 1)
	if got[0] != "a2" {
		t.Errorf("Next = %s, want a2", got[0])
	}
	if s.Remove("a1") {
		t.Error("second Remove(a1) = true, want false")
	}
}

func TestSchedulerNextHonorsContext(t *testing.T) {
	s := New(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	id, err := s.Next(ctx)
	if id != "" || err != context.Canceled {
		t.Errorf("Next on cancelled ctx = (%q, %v), want (\"\", context.Canceled)", id, err)
	}
}

func TestSchedulerNextBlocksUntilEnqueue(t *testing.T) {
	s := New(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan string, 1)
	go func() {
		id, err := s.Next(ctx)
		if err != nil {
			done <- "ERR:" + err.Error()
			return
		}
		done <- id
	}()

	time.Sleep(10 * time.Millisecond)
	s.Enqueue("a", "a1")

	select {
	case id := <-done:
		if id != "a1" {
			t.Errorf("Next = %q, want a1", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Next never returned after Enqueue")
	}
}
