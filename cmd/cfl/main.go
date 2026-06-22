// Command cfl is a URL-native Confluence Server/Data Center CLI.
//
// This package is wiring only: it runs the root command, renders any error
// through internal/errors, and maps the result to a process exit code. All
// behavior lives under internal/.
package main

import (
	"os"

	"github.com/addozhang/cfl/internal/cli"
	cflerrors "github.com/addozhang/cfl/internal/errors"
)

func main() {
	err := cli.Execute()
	if err != nil {
		// The --debug flag is parsed inside the command tree; by the time an
		// error reaches here we render without the raw cause unless it is a
		// CFLError that already carries a user-facing message. Debug rendering
		// of causes happens closer to the failure in later iterations.
		cflerrors.Render(os.Stderr, err, false)
	}
	os.Exit(cflerrors.ExitCode(err))
}
