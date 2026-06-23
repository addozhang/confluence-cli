package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func asCFL(t *testing.T, err error) *CFLError {
	t.Helper()
	c, ok := AsCFLError(err)
	if !ok {
		t.Fatalf("expected *CFLError, got %T (%v)", err, err)
	}
	return c
}

func Test_TranslateConfluence_auth_rejected(t *testing.T) {
	for _, status := range []int{401, 403} {
		httpErr := &HTTPStatusError{StatusCode: status, URL: "https://wiki.example.com/rest/api/content/1"}
		got := asCFL(t, TranslateConfluence("https://wiki.example.com", ResourcePage, httpErr))

		if got.Code != CodeAuth {
			t.Errorf("status %d: Code = %v, want %v", status, got.Code, CodeAuth)
		}
		if !strings.Contains(got.Message, "wiki.example.com") {
			t.Errorf("status %d: Message %q should name the host", status, got.Message)
		}
		if !strings.Contains(got.Suggestion, "cfl auth add") {
			t.Errorf("status %d: Suggestion %q should mention `cfl auth add`", status, got.Suggestion)
		}
	}
}

func Test_TranslateConfluence_page_not_found(t *testing.T) {
	httpErr := &HTTPStatusError{StatusCode: 404, URL: "https://wiki.example.com/pages/viewpage.action?pageId=99999"}
	got := asCFL(t, TranslateConfluence("https://wiki.example.com", ResourcePage, httpErr))

	if got.Code != CodeNotFound {
		t.Errorf("Code = %v, want %v", got.Code, CodeNotFound)
	}
	if !strings.Contains(got.Message, "Page not found") {
		t.Errorf("Message %q should say 'Page not found'", got.Message)
	}
}

func Test_TranslateConfluence_space_not_found(t *testing.T) {
	httpErr := &HTTPStatusError{StatusCode: 404, URL: "https://wiki.example.com/rest/api/space/NOPE"}
	got := asCFL(t, TranslateConfluence("https://wiki.example.com", ResourceSpace, httpErr))

	if got.Code != CodeSpaceNotFound {
		t.Errorf("Code = %v, want %v", got.Code, CodeSpaceNotFound)
	}
	if !strings.Contains(got.Message, "Space not found") {
		t.Errorf("Message %q should say 'Space not found'", got.Message)
	}
	if !strings.Contains(got.Suggestion, "cfl space list") {
		t.Errorf("Suggestion %q should mention `cfl space list`", got.Suggestion)
	}
}

func Test_TranslateConfluence_version_conflict(t *testing.T) {
	httpErr := &HTTPStatusError{StatusCode: 409, URL: "https://wiki.example.com/rest/api/content/12345"}
	got := asCFL(t, TranslateConfluence("https://wiki.example.com", ResourcePageID("12345"), httpErr))

	if got.Code != CodeVersionConflict {
		t.Errorf("Code = %v, want %v", got.Code, CodeVersionConflict)
	}
	if !strings.Contains(got.Message, "12345") {
		t.Errorf("Message %q should name the page id", got.Message)
	}
	if !strings.Contains(got.Suggestion, "cfl page get") {
		t.Errorf("Suggestion %q should suggest re-running `cfl page get`", got.Suggestion)
	}
}

func Test_TranslateConfluence_timeout(t *testing.T) {
	got := asCFL(t, TranslateTimeout("https://wiki.example.com", 30*time.Second, stderrors.New("context deadline exceeded")))

	if got.Code != CodeTimeout {
		t.Errorf("Code = %v, want %v", got.Code, CodeTimeout)
	}
	if !strings.Contains(got.Message, "30s") {
		t.Errorf("Message %q should include the duration", got.Message)
	}
	if !strings.Contains(got.Suggestion, "--timeout") {
		t.Errorf("Suggestion %q should mention --timeout", got.Suggestion)
	}
}

func Test_TranslateSSLCertFile(t *testing.T) {
	got := asCFL(t, TranslateSSLCertFile("/etc/ca/missing.pem", stderrors.New("no such file")))

	if got.Code != CodeTLS {
		t.Errorf("Code = %v, want %v", got.Code, CodeTLS)
	}
	if !strings.Contains(got.Message, "/etc/ca/missing.pem") {
		t.Errorf("Message %q should include the path", got.Message)
	}
	if !strings.Contains(got.Suggestion, "PEM") {
		t.Errorf("Suggestion %q should mention PEM bundle", got.Suggestion)
	}
}

func Test_TranslateConfluence_malformed(t *testing.T) {
	got := asCFL(t, TranslateMalformed("https://wiki.example.com", stderrors.New("invalid character '<'")))

	if got.Code != CodeMalformed {
		t.Errorf("Code = %v, want %v", got.Code, CodeMalformed)
	}
	if !strings.Contains(got.Suggestion, "--debug") {
		t.Errorf("Suggestion %q should mention --debug", got.Suggestion)
	}
}

func Test_TranslateNetwork(t *testing.T) {
	got := asCFL(t, TranslateNetwork("https://wiki.example.com", stderrors.New("dial tcp: connection refused")))

	if got.Code != CodeNetwork {
		t.Errorf("Code = %v, want %v", got.Code, CodeNetwork)
	}
	if !strings.Contains(got.Message, "wiki.example.com") {
		t.Errorf("Message %q should name the host", got.Message)
	}
}

func Test_WrapURLParse(t *testing.T) {
	got := asCFL(t, WrapURLParse("not a url", stderrors.New("missing scheme")))

	if got.Code != CodeBadURL {
		t.Errorf("Code = %v, want %v", got.Code, CodeBadURL)
	}
	if !strings.Contains(got.Message, "not a url") {
		t.Errorf("Message %q should include the offending argument", got.Message)
	}
}

func Test_TranslateConfluence_context_deadline_is_timeout(t *testing.T) {
	// A wrapped context deadline (as the client surfaces it) must classify as a
	// timeout, not a malformed response.
	wrapped := fmt.Errorf("GET https://wiki.example.com/rest/api/space: %w", context.DeadlineExceeded)
	got := asCFL(t, TranslateConfluence("https://wiki.example.com", ResourceSpace, wrapped))

	if got.Code != CodeTimeout {
		t.Errorf("Code = %v, want %v", got.Code, CodeTimeout)
	}
	if !strings.Contains(got.Suggestion, "--timeout") {
		t.Errorf("Suggestion %q should mention --timeout", got.Suggestion)
	}
}

func Test_TranslateConfluence_net_timeout_is_timeout(t *testing.T) {
	got := asCFL(t, TranslateConfluence("https://wiki.example.com", ResourcePage, timeoutNetErr{}))
	if got.Code != CodeTimeout {
		t.Errorf("Code = %v, want %v", got.Code, CodeTimeout)
	}
}

func Test_TranslateConfluence_dns_error_is_network(t *testing.T) {
	dns := &net.DNSError{Err: "no such host", Name: "wiki.absent.example"}
	wrapped := fmt.Errorf("GET https://wiki.absent.example/rest/api/space: %w", dns)
	got := asCFL(t, TranslateConfluence("https://wiki.absent.example", ResourceSpace, wrapped))

	if got.Code != CodeNetwork {
		t.Errorf("Code = %v, want %v", got.Code, CodeNetwork)
	}
}

func Test_TranslateConfluence_opnerror_is_network(t *testing.T) {
	op := &net.OpError{Op: "dial", Net: "tcp", Err: stderrors.New("connection refused")}
	got := asCFL(t, TranslateConfluence("https://wiki.example.com", ResourcePage, op))
	if got.Code != CodeNetwork {
		t.Errorf("Code = %v, want %v", got.Code, CodeNetwork)
	}
}

// timeoutNetErr is a net.Error whose Timeout() reports true, for classifying
// transport timeouts that are not context deadlines.
type timeoutNetErr struct{}

func (timeoutNetErr) Error() string   { return "i/o timeout" }
func (timeoutNetErr) Timeout() bool   { return true }
func (timeoutNetErr) Temporary() bool { return false }
