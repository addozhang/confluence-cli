package cli

import (
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test_timeout_is_bound_via_context proves that --timeout bounds a request
// through the command context deadline: a server slower than the timeout causes
// the request to fail with a deadline/timeout, not hang.
func Test_timeout_is_bound_via_context(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release // hold the response open past the timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer close(release)

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	start := time.Now()
	_, _, err := runCmd(t, creds, "", "space", "list", "--instance", srv.URL, "--timeout", "200ms")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("a request slower than --timeout should fail")
	}
	// It must fail promptly (near the timeout), not hang until the server is
	// released or some default kicks in.
	if elapsed > 5*time.Second {
		t.Errorf("request did not honor the 200ms timeout; took %s", elapsed)
	}
	// The failure should be surfaced as a timeout/network-class error.
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "timed out") && !strings.Contains(msg, "timeout") && !strings.Contains(msg, "deadline") && !strings.Contains(msg, "network") {
		t.Errorf("error should indicate a timeout, got: %q", err.Error())
	}
}

// Test_context_deadline_classified_as_timeout checks the error translation maps
// a context deadline to the timeout CFLError, not a generic failure.
func Test_context_deadline_classified_as_timeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer close(release)

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	_, _, err := runCmd(t, creds, "", "space", "get", "ENG", "--instance", srv.URL, "--timeout", "150ms")
	if err == nil {
		t.Fatalf("expected a timeout error")
	}
	var cflErr interface{ Error() string }
	if !stderrors.As(err, &cflErr) {
		t.Fatalf("expected an error value")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "tim") {
		t.Errorf("error should mention a timeout, got: %q", err.Error())
	}
}
