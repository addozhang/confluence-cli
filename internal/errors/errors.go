// Package errors defines cfl's error model and the translation layer that
// turns low-level HTTP, network, TLS, and parse failures into user-facing
// messages with actionable suggestions.
//
// Lower layers wrap failures with fmt.Errorf("doing X: %w", err); the CLI layer
// translates recognized failures into *CFLError values. The top layer renders
// the message and suggestion and maps the error to a process exit code.
package errors

import (
	stderrors "errors"
	"fmt"
	"io"
)

// Code classifies a cfl-level failure. The exact exit code derived from a Code
// is intentionally unstable (any value >= 10); callers and tests must assert
// only on the >= 10 boundary, never on a specific value.
type Code int

const (
	// CodeUnknown is the zero value; it still maps to a failure exit code.
	CodeUnknown Code = iota
	// CodeBadURL marks an argument that is not a valid Confluence URL or page ID.
	CodeBadURL
	// CodeAuth marks an authentication failure (HTTP 401/403).
	CodeAuth
	// CodeNotFound marks a missing page (HTTP 404 on content).
	CodeNotFound
	// CodeSpaceNotFound marks a missing space (HTTP 404 on a space).
	CodeSpaceNotFound
	// CodeVersionConflict marks a stale-version update rejection (HTTP 409).
	CodeVersionConflict
	// CodeTimeout marks a request that exceeded the configured timeout.
	CodeTimeout
	// CodeTLS marks a TLS/SSL_CERT_FILE configuration failure.
	CodeTLS
	// CodeMalformed marks an unexpected or unparseable response.
	CodeMalformed
	// CodeNetwork marks a connection-level failure that is not a timeout.
	CodeNetwork
	// CodeConfig marks a configuration failure (e.g. missing credential).
	CodeConfig
)

// String renders the Code as a stable identifier for debug logging.
func (c Code) String() string {
	switch c {
	case CodeBadURL:
		return "bad_url"
	case CodeAuth:
		return "auth"
	case CodeNotFound:
		return "not_found"
	case CodeSpaceNotFound:
		return "space_not_found"
	case CodeVersionConflict:
		return "version_conflict"
	case CodeTimeout:
		return "timeout"
	case CodeTLS:
		return "tls"
	case CodeMalformed:
		return "malformed"
	case CodeNetwork:
		return "network"
	case CodeConfig:
		return "config"
	default:
		return "unknown"
	}
}

// CFLError is the single shape for every cfl-level failure. Message is the
// one-sentence description shown to the user; Suggestion is the next step;
// Cause is the wrapped lower-level error, never shown unless --debug is set.
type CFLError struct {
	Code       Code
	Message    string
	Suggestion string
	Cause      error
}

// Error implements the error interface. It returns only the Message so that
// wrapping (fmt.Errorf("...: %w", cflErr)) reads naturally; the Suggestion and
// Cause are surfaced by Render, not by Error.
func (e *CFLError) Error() string {
	return e.Message
}

// Unwrap exposes the wrapped Cause for errors.Is/errors.As traversal.
func (e *CFLError) Unwrap() error {
	return e.Cause
}

// AsCFLError reports whether err is, or wraps, a *CFLError.
func AsCFLError(err error) (*CFLError, bool) {
	var c *CFLError
	if stderrors.As(err, &c) {
		return c, true
	}
	return nil, false
}

// failureExitCode is the exit code for any cfl-level failure. It is >= 10 per
// the errors spec; its exact value is not part of the contract.
const failureExitCode = 10

// ExitCode maps a command result to a process exit code: nil -> 0, any error
// -> a value >= 10.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	return failureExitCode
}

// HTTPStatusError is produced by the Confluence client for any non-2xx
// response. The CLI layer classifies it (via TranslateConfluence) into the
// appropriate CFLError, because the meaning of a 404 depends on whether a page
// or a space was requested.
type HTTPStatusError struct {
	StatusCode int
	Status     string
	Body       []byte
	URL        string
}

func (e *HTTPStatusError) Error() string {
	if e.Status != "" {
		return fmt.Sprintf("unexpected HTTP status %s for %s", e.Status, e.URL)
	}
	return fmt.Sprintf("unexpected HTTP status %d for %s", e.StatusCode, e.URL)
}

// Render writes the user-facing representation of err to w. It always prints
// an "Error: <message>" line; for a CFLError it then prints the suggestion on
// the next line. When debug is true and a Cause is present, the raw cause is
// printed as an additional line; otherwise the cause is hidden.
func Render(w io.Writer, err error, debug bool) {
	if err == nil {
		return
	}

	if c, ok := AsCFLError(err); ok {
		_, _ = fmt.Fprintf(w, "Error: %s\n", c.Message)
		if c.Suggestion != "" {
			_, _ = fmt.Fprintf(w, "%s\n", c.Suggestion)
		}
		if debug && c.Cause != nil {
			_, _ = fmt.Fprintf(w, "cause: %v\n", c.Cause)
		}
		return
	}

	_, _ = fmt.Fprintf(w, "Error: %s\n", err.Error())
}
