package cli

import "golang.org/x/term"

// termIsTerminal reports whether the given file descriptor is a terminal. It is
// a thin wrapper over x/term so the rest of the package does not import term
// directly and tests can reason about interactivity via *os.File detection.
func termIsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}
