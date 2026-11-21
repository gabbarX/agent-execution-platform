// Package version exposes the platform's version string.
// Bumped manually at each milestone close-out.
package version

const current = "0.1.0-m1"

// String returns the platform version.
func String() string {
	return current
}
