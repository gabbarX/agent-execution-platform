// Package run holds the run domain: the Run record, its status state
// machine, and (in store.go) the in-memory store.
package run

import (
	"errors"
	"fmt"
	"time"
)

// Status is a run's position in its lifecycle. Transitions are only
// legal along the edges in transitions below; everything else must be
// rejected, because recovery logic (M2+) depends on illegal writes —
// e.g. a stale worker moving SUCCEEDED back to RUNNING — being
// impossible.
type Status string

const (
	StatusQueued    Status = "QUEUED"
	StatusStarting  Status = "STARTING"
	StatusRunning   Status = "RUNNING"
	StatusWaiting   Status = "WAITING"
	StatusSucceeded Status = "SUCCEEDED"
	StatusFailed    Status = "FAILED"
	StatusCancelled Status = "CANCELLED"
)

var transitions = map[Status]map[Status]bool{
	StatusQueued:   {StatusStarting: true, StatusCancelled: true},
	StatusStarting: {StatusRunning: true, StatusFailed: true, StatusCancelled: true},
	StatusRunning:  {StatusWaiting: true, StatusSucceeded: true, StatusFailed: true, StatusCancelled: true},
	StatusWaiting:  {StatusQueued: true, StatusFailed: true, StatusCancelled: true},
}

// Terminal reports whether a run in this status is finished for good.
func (s Status) Terminal() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusCancelled:
		return true
	}
	return false
}

// CanTransitionTo reports whether the edge s -> to is legal.
func (s Status) CanTransitionTo(to Status) bool {
	return transitions[s][to]
}

// ErrInvalidTransition is returned (wrapped) by Run.Transition for any
// edge not in the state machine.
var ErrInvalidTransition = errors.New("invalid transition")

// Run is one execution of an agent on behalf of a team.
type Run struct {
	ID             string    `json:"id"`
	Team           string    `json:"team"`
	Agent          string    `json:"agent"`
	Version        int       `json:"version"`
	Input          string    `json:"input,omitempty"`
	Status         Status    `json:"status"`
	IdempotencyKey string    `json:"-"`
	StepsTotal     int       `json:"steps_total"`
	StepsDone      int       `json:"steps_done"`
	Error          string    `json:"error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	StartedAt      time.Time `json:"started_at,omitempty"`
	FinishedAt     time.Time `json:"finished_at,omitempty"`
}

// Transition moves the run to a new status, enforcing the state
// machine and stamping StartedAt / FinishedAt at the right edges.
func (r *Run) Transition(to Status) error {
	if !r.Status.CanTransitionTo(to) {
		return fmt.Errorf("%s -> %s: %w", r.Status, to, ErrInvalidTransition)
	}
	r.Status = to
	now := time.Now()
	if to == StatusRunning && r.StartedAt.IsZero() {
		r.StartedAt = now
	}
	if to.Terminal() {
		r.FinishedAt = now
	}
	return nil
}
