// Package cli implements the cfl subcommands and the root command wiring.
// It exports nothing beyond Execute and the version metadata setters used by
// the main package; all behavior is reached through CLI invocation.
package cli

// Build metadata, injected via -ldflags at build/release time. They default to
// placeholders so a plain `go build` still produces a usable `cfl version`.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// BuildInfo returns the injected version, commit, and date. It is used by the
// version command and is safe to call when no metadata was injected.
func BuildInfo() (v, c, d string) {
	return version, commit, date
}
