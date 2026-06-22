package cli

import (
	"github.com/spf13/cobra"

	"github.com/addozhang/cfl/internal/output"
	"github.com/addozhang/cfl/internal/schema"
)

// newVersionCmd builds `cfl version`. It is offline: no client, no credentials,
// no file access. It reports the build metadata injected via -ldflags, falling
// back to placeholders when none was injected.
func newVersionCmd(deps *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the cfl build version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, c, d := BuildInfo()
			info := schema.VersionInfo{
				Version: v,
				Commit:  nilIfEmpty(c, "none"),
				Date:    nilIfEmpty(d, "unknown"),
			}
			return output.Write(cmd.OutOrStdout(), info, deps.OutputFormat)
		},
	}
}

// nilIfEmpty returns nil when s is empty or equal to the placeholder, so that
// un-injected metadata renders as null rather than a meaningless placeholder in
// structured output.
func nilIfEmpty(s, placeholder string) *string {
	if s == "" || s == placeholder {
		return nil
	}
	return &s
}
