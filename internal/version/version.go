package version

import "fmt"

// Build-time variables set via ldflags.
var (
	Version   = "0.1.0"
	GitCommit = "unknown"
	BuildDate = "unknown"
	GoVersion = "unknown"
)

// Info returns a formatted version string.
func Info() string {
	return fmt.Sprintf("kronveil %s (commit: %s, built: %s, go: %s)",
		Version, GitCommit, BuildDate, GoVersion)
}
