// Package version exposes the platform's version string.
// Bumped manually at each milestone close-out.
package version

const current = "0.0.1-m0"

// String returns the platform version.
func String() string {
	return current
}
