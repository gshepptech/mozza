// Package version provides build metadata injected via ldflags.
package version

import "runtime"

// Build metadata set by ldflags at compile time.
var (
	Version = "dev"     //nolint:gochecknoglobals // set by ldflags
	Commit  = "unknown" //nolint:gochecknoglobals // set by ldflags
	Date    = "unknown" //nolint:gochecknoglobals // set by ldflags
)

// GoVersion returns the Go version used to compile the binary.
func GoVersion() string {
	return runtime.Version()
}
