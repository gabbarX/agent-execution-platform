package worker

import (
	"context"
	"time"

	"github.com/gabbarX/agent-execution-platform/internal/registry"
)

// AgentFunc is the unit of executable agent work. Injecting it keeps
// the pool testable (a test can supply a panicking or instant agent)
// and is the seam where M3's SDK-driven agents and M4's containerized
// agents plug in.
type AgentFunc func(ctx context.Context, spec registry.AgentSpec, progress func(done int)) error

// FakeAgent simulates agent work: spec.Steps steps of spec.StepDelay
// each, reporting progress after every step, stopping promptly when
// ctx is cancelled.
func FakeAgent(ctx context.Context, spec registry.AgentSpec, progress func(done int)) error {
	for i := 0; i < spec.Steps; i++ {
		if spec.StepDelay > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(spec.StepDelay):
			}
		} else if err := ctx.Err(); err != nil {
			return err
		}
		progress(i + 1)
	}
	return nil
}
