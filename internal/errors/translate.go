package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"net"
	"time"
)

// Resource describes what kind of Confluence entity a request targeted, so a
// shared HTTP status (notably 404) can be translated into the right message.
// Use ResourcePage / ResourceSpace for the general cases, or ResourcePageID to
// carry the page id into version-conflict messages.
type Resource struct {
	kind   resourceKind
	pageID string
}

type resourceKind int

const (
	resourceUnknown resourceKind = iota
	resourcePage
	resourceSpace
)

// ResourcePage marks a request as targeting a page (without a known id).
var ResourcePage = Resource{kind: resourcePage}

// ResourceSpace marks a request as targeting a space.
var ResourceSpace = Resource{kind: resourceSpace}

// ResourcePageID marks a request as targeting a specific page id; the id is
// used in version-conflict messages.
func ResourcePageID(id string) Resource {
	return Resource{kind: resourcePage, pageID: id}
}

// TranslateConfluence classifies an HTTPStatusError (or any error) returned by
// the Confluence client into a CFLError. host is the normalized instance host
// key used in messages. res says whether a page or space was targeted.
func TranslateConfluence(host string, res Resource, err error) error {
	if err == nil {
		return nil
	}

	var httpErr *HTTPStatusError
	if !stderrors.As(err, &httpErr) {
		// Not an HTTP status failure: classify transport-level failures
		// (timeout, then other network errors) before falling back to a
		// malformed/unexpected-response message.
		if isTimeout(err) {
			return &CFLError{
				Code:       CodeTimeout,
				Message:    fmt.Sprintf("Timed out contacting %s.", host),
				Suggestion: "Increase the limit with `--timeout <duration>` or check VPN connectivity.",
				Cause:      err,
			}
		}
		if isNetworkError(err) {
			return TranslateNetwork(host, err)
		}
		return TranslateMalformed(host, err)
	}

	switch httpErr.StatusCode {
	case 401, 403:
		return &CFLError{
			Code:       CodeAuth,
			Message:    fmt.Sprintf("API token rejected by %s.", host),
			Suggestion: fmt.Sprintf("Run `cfl auth add %s` to refresh.", host),
			Cause:      httpErr,
		}
	case 404:
		if res.kind == resourceSpace {
			return &CFLError{
				Code:       CodeSpaceNotFound,
				Message:    fmt.Sprintf("Space not found: %s.", httpErr.URL),
				Suggestion: "List available spaces with `cfl space list`.",
				Cause:      httpErr,
			}
		}
		return &CFLError{
			Code:       CodeNotFound,
			Message:    fmt.Sprintf("Page not found: %s.", httpErr.URL),
			Suggestion: "Check the URL or page ID and try again.",
			Cause:      httpErr,
		}
	case 409:
		id := res.pageID
		if id == "" {
			id = httpErr.URL
		}
		return &CFLError{
			Code:       CodeVersionConflict,
			Message:    fmt.Sprintf("Update rejected: page %s changed since you last read it (version conflict).", id),
			Suggestion: "Re-run `cfl page get` and retry the update.",
			Cause:      httpErr,
		}
	default:
		return TranslateMalformed(host, httpErr)
	}
}

// TranslateTimeout renders a request timeout as a CFLError naming the duration
// that was exceeded.
func TranslateTimeout(host string, d time.Duration, cause error) error {
	return &CFLError{
		Code:       CodeTimeout,
		Message:    fmt.Sprintf("Timed out after %s contacting %s.", d, host),
		Suggestion: "Increase with `--timeout <duration>` or check VPN connectivity.",
		Cause:      cause,
	}
}

// TranslateSSLCertFile renders a failure to load the SSL_CERT_FILE CA bundle.
func TranslateSSLCertFile(path string, cause error) error {
	return &CFLError{
		Code:       CodeTLS,
		Message:    fmt.Sprintf("SSL_CERT_FILE points to %s which could not be loaded.", path),
		Suggestion: "Verify the path and that the file is a valid PEM certificate bundle.",
		Cause:      cause,
	}
}

// TranslateMalformed renders an unexpected/unparseable response.
func TranslateMalformed(host string, cause error) error {
	return &CFLError{
		Code:       CodeMalformed,
		Message:    fmt.Sprintf("Received an unexpected response from %s.", host),
		Suggestion: "Re-run with `--debug` to inspect the exchange, or file an issue.",
		Cause:      cause,
	}
}

// TranslateNetwork renders a connection-level failure that is not a timeout
// (connection refused, DNS failure, unreachable host).
func TranslateNetwork(host string, cause error) error {
	msg := fmt.Sprintf("Network error contacting %s", host)
	if cause != nil {
		msg = fmt.Sprintf("%s: %v.", msg, cause)
	} else {
		msg += "."
	}
	return &CFLError{
		Code:       CodeNetwork,
		Message:    msg,
		Suggestion: "Check that the host is reachable and that any required VPN is connected.",
		Cause:      cause,
	}
}

// WrapURLParse renders an argument that could not be parsed into a Confluence
// reference.
func WrapURLParse(arg string, cause error) error {
	return &CFLError{
		Code:       CodeBadURL,
		Message:    fmt.Sprintf("Could not parse %q as a Confluence URL or page ID.", arg),
		Suggestion: "Pass a Confluence page/space URL or a numeric page ID.",
		Cause:      cause,
	}
}

// isTimeout reports whether err is a deadline/timeout: a context deadline, or a
// net.Error whose Timeout() is true.
func isTimeout(err error) bool {
	if stderrors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if stderrors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

// isNetworkError reports whether err is a connection-level failure (DNS,
// connection refused, unreachable host) that is not a timeout.
func isNetworkError(err error) bool {
	if stderrors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if stderrors.As(err, &netErr) {
		return true
	}
	var dnsErr *net.DNSError
	if stderrors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	return stderrors.As(err, &opErr)
}
