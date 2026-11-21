package run

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Store is the in-memory system of record for runs (replaced by
// Postgres in M2). It hands out copies, never internal pointers: all
// mutation goes through Update, under the lock, so concurrent workers
// and API handlers can't race on a shared *Run.
type Store struct {
	mu    sync.Mutex
	runs  map[string]*Run
	byKey map[string]string // idempotency key -> run ID
}

func NewStore() *Store {
	return &Store{
		runs:  make(map[string]*Run),
		byKey: make(map[string]string),
	}
}

// Create registers a new run, assigning its ID, CreatedAt, and QUEUED
// status. If r.IdempotencyKey was seen before, the original run is
// returned with existed=true and nothing new is created.
func (s *Store) Create(r *Run) (*Run, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.IdempotencyKey != "" {
		if id, ok := s.byKey[r.IdempotencyKey]; ok {
			cp := *s.runs[id]
			return &cp, true
		}
	}

	stored := *r
	stored.ID = newRunID()
	stored.CreatedAt = time.Now()
	stored.Status = StatusQueued
	s.runs[stored.ID] = &stored
	if stored.IdempotencyKey != "" {
		s.byKey[stored.IdempotencyKey] = stored.ID
	}
	cp := stored
	return &cp, false
}

// Get returns a copy of the run, or false if unknown.
func (s *Store) Get(id string) (*Run, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return nil, false
	}
	cp := *r
	return &cp, true
}

// List returns copies of runs for a team ("" = all), sorted by
// CreatedAt then ID.
func (s *Store) List(team string) []*Run {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Run
	for _, r := range s.runs {
		if team != "" && r.Team != team {
			continue
		}
		cp := *r
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Update applies fn to the stored run under the lock. If fn errors the
// run is left unchanged and the error is returned.
func (s *Store) Update(id string, fn func(*Run) error) (*Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[id]
	if !ok {
		return nil, fmt.Errorf("update run %s: not found", id)
	}
	scratch := *r
	if err := fn(&scratch); err != nil {
		return nil, fmt.Errorf("update run %s: %w", id, err)
	}
	*r = scratch
	cp := scratch
	return &cp, nil
}

func newRunID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
	}
	return "run_" + hex.EncodeToString(b)
}
