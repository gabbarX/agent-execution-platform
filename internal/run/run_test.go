package run

import (
	"errors"
	"testing"
)

var allStatuses = []Status{
	StatusQueued, StatusStarting, StatusRunning, StatusWaiting,
	StatusSucceeded, StatusFailed, StatusCancelled,
}

// The complete set of legal edges, spelled out independently of the
// implementation so the test can't silently agree with a wrong table.
var legalEdges = map[Status][]Status{
	StatusQueued:   {StatusStarting, StatusCancelled},
	StatusStarting: {StatusRunning, StatusFailed, StatusCancelled},
	StatusRunning:  {StatusWaiting, StatusSucceeded, StatusFailed, StatusCancelled},
	StatusWaiting:  {StatusQueued, StatusFailed, StatusCancelled},
}

func isLegal(from, to Status) bool {
	for _, t := range legalEdges[from] {
		if t == to {
			return true
		}
	}
	return false
}

func TestStatusCanTransitionTo(t *testing.T) {
	for _, from := range allStatuses {
		for _, to := range allStatuses {
			want := isLegal(from, to)
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("%s -> %s: got %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestStatusTerminal(t *testing.T) {
	terminals := map[Status]bool{
		StatusSucceeded: true, StatusFailed: true, StatusCancelled: true,
	}
	for _, s := range allStatuses {
		if got := s.Terminal(); got != terminals[s] {
			t.Errorf("%s.Terminal() = %v, want %v", s, got, terminals[s])
		}
	}
}

func TestTransitionSetsTimestamps(t *testing.T) {
	r := &Run{Status: StatusStarting}
	if err := r.Transition(StatusRunning); err != nil {
		t.Fatalf("to RUNNING: %v", err)
	}
	if r.StartedAt.IsZero() {
		t.Error("StartedAt not set on transition to RUNNING")
	}
	if err := r.Transition(StatusSucceeded); err != nil {
		t.Fatalf("to SUCCEEDED: %v", err)
	}
	if r.FinishedAt.IsZero() {
		t.Error("FinishedAt not set on transition to terminal status")
	}
}

func TestTransitionRejectsIllegal(t *testing.T) {
	r := &Run{Status: StatusSucceeded}
	err := r.Transition(StatusRunning)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("got %v, want ErrInvalidTransition", err)
	}
	if r.Status != StatusSucceeded {
		t.Errorf("status mutated on illegal transition: %s", r.Status)
	}
}
