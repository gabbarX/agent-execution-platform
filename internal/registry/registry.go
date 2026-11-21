// Package registry maps agent name@version to its execution spec. In
// M1 it's an in-memory seed; M2 moves it to Postgres behind a
// POST /agents registration API.
package registry

import (
	"fmt"
	"sync"
	"time"
)

// AgentSpec describes one registered agent version. Steps/StepDelay
// parameterize the M1 fake agent; later milestones replace them with a
// container image reference.
type AgentSpec struct {
	Name      string
	Version   int
	Steps     int
	StepDelay time.Duration
}

type Registry struct {
	mu     sync.Mutex
	agents map[string]AgentSpec
}

// New returns a registry pre-seeded with the demo agent.
func New() *Registry {
	g := &Registry{agents: make(map[string]AgentSpec)}
	g.Register(AgentSpec{Name: "demo", Version: 1, Steps: 8, StepDelay: 300 * time.Millisecond})
	return g
}

func (g *Registry) Register(spec AgentSpec) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.agents[key(spec.Name, spec.Version)] = spec
}

func (g *Registry) Lookup(name string, version int) (AgentSpec, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	spec, ok := g.agents[key(name, version)]
	return spec, ok
}

func key(name string, version int) string {
	return fmt.Sprintf("%s@%d", name, version)
}
