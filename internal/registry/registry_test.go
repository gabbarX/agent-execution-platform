package registry

import (
	"testing"
	"time"
)

func TestRegistrySeedsDemoAgent(t *testing.T) {
	g := New()
	spec, ok := g.Lookup("demo", 1)
	if !ok {
		t.Fatal("demo@1 not seeded")
	}
	if spec.Steps != 8 {
		t.Errorf("Steps = %d, want 8", spec.Steps)
	}
	if spec.StepDelay != 300*time.Millisecond {
		t.Errorf("StepDelay = %v, want 300ms", spec.StepDelay)
	}
}

func TestRegistryLookupUnknown(t *testing.T) {
	g := New()
	cases := []struct {
		name    string
		version int
	}{
		{"demo", 2},
		{"nope", 1},
	}
	for _, c := range cases {
		if _, ok := g.Lookup(c.name, c.version); ok {
			t.Errorf("Lookup(%q, %d) unexpectedly found", c.name, c.version)
		}
	}
}

func TestRegistryRegisterThenLookup(t *testing.T) {
	g := New()
	g.Register(AgentSpec{Name: "fast", Version: 3, Steps: 2, StepDelay: time.Millisecond})
	spec, ok := g.Lookup("fast", 3)
	if !ok || spec.Steps != 2 {
		t.Fatalf("Lookup(fast, 3) = %+v, %v", spec, ok)
	}
}
