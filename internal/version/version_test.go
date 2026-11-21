package version_test

import (
	"testing"

	"github.com/gabbarX/agent-execution-platform/internal/version"
)

func TestVersionString(t *testing.T) {
	if got := version.String(); got != "0.0.1-m0" {
		t.Errorf("version.String() = %q, want %q", got, "0.0.1-m0")
	}
}
