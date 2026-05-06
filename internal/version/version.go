// Package version exposes build-time metadata.
// Values are populated via -ldflags by the Makefile.
package version

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a human-readable build identifier combining Version, Commit
// and Date in the form "Version (Commit, Date)".
func String() string {
	return Version + " (" + Commit + ", " + Date + ")"
}
