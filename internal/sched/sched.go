// Package sched implements weighted fair scheduling over per-team
// FIFO queues: round-robin across team names in sorted order, serving
// up to a team's weight per visit. One team's burst therefore cannot
// starve another team — the fairness property M1's demo proves.
package sched

import (
	"context"
	"sort"
	"sync"
)

type Scheduler struct {
	mu      sync.Mutex
	cond    *sync.Cond
	weights map[string]int
	queues  map[string][]string // team -> FIFO of run IDs

	// Round-robin cursor, persisted across Next calls: the team being
	// served and how many items it has received this visit.
	curTeam string
	served  int
}

// New creates a scheduler. A nil map is fine; any team missing from
// weights (or with weight < 1) gets weight 1.
func New(weights map[string]int) *Scheduler {
	s := &Scheduler{
		weights: weights,
		queues:  make(map[string][]string),
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Enqueue adds a run to its team's queue and wakes blocked Next calls.
func (s *Scheduler) Enqueue(team, runID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queues[team] = append(s.queues[team], runID)
	s.cond.Broadcast()
}

// Next blocks until a run is available (returning its ID) or ctx is
// done (returning ctx.Err()).
func (s *Scheduler) Next(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Wake our cond.Wait when the context ends, so blocked callers
	// don't sleep forever.
	stop := context.AfterFunc(ctx, func() {
		s.mu.Lock()
		s.cond.Broadcast()
		s.mu.Unlock()
	})
	defer stop()

	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		if id, ok := s.pickLocked(); ok {
			return id, nil
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		s.cond.Wait()
	}
}

// Remove pulls a still-queued run out of its queue (the cancel path).
// Returns false if the run is not queued (already dispatched or never
// enqueued).
func (s *Scheduler) Remove(runID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for team, q := range s.queues {
		for i, id := range q {
			if id == runID {
				s.queues[team] = append(q[:i:i], q[i+1:]...)
				return true
			}
		}
	}
	return false
}

// pickLocked advances the weighted round-robin by one item. Caller
// holds s.mu.
func (s *Scheduler) pickLocked() (string, bool) {
	teams := make([]string, 0, len(s.queues))
	for t, q := range s.queues {
		if len(q) > 0 {
			teams = append(teams, t)
		}
	}
	if len(teams) == 0 {
		return "", false
	}
	sort.Strings(teams)

	cur, served := s.curTeam, s.served
	if cur == "" || len(s.queues[cur]) == 0 || served >= s.weight(cur) {
		cur = nextAfter(teams, cur)
		served = 0
	}
	q := s.queues[cur]
	id := q[0]
	s.queues[cur] = q[1:]
	s.curTeam, s.served = cur, served+1
	return id, true
}

func (s *Scheduler) weight(team string) int {
	if w, ok := s.weights[team]; ok && w >= 1 {
		return w
	}
	return 1
}

// nextAfter returns the first team strictly after cur in sorted order,
// wrapping to the first team.
func nextAfter(teams []string, cur string) string {
	for _, t := range teams {
		if t > cur {
			return t
		}
	}
	return teams[0]
}
