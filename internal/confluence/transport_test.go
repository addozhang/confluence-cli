package confluence

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/addozhang/cfl/internal/auth"
)

// roundTrip drives a RoundTripper against a test server and returns the
// captured server-side request plus the response body.
func doGet(t *testing.T, rt http.RoundTripper, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	return resp
}

func Test_BearerTransport_sets_authorization_header(t *testing.T) {
	var gotAuth string
	var gotBasic bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _, gotBasic = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := auth.NewStore(map[string]string{srv.URL: "PAT-abc123"})
	rt, err := NewTransport(TransportConfig{Timeout: 5 * time.Second}, store)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}

	resp := doGet(t, rt, srv.URL+"/rest/api/space")
	_ = resp.Body.Close()

	if gotAuth != "Bearer PAT-abc123" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer PAT-abc123")
	}
	if gotBasic {
		t.Errorf("request carried HTTP Basic auth; want Bearer only, no username")
	}
}

func Test_BearerTransport_no_credential_leaves_header_unset(t *testing.T) {
	var hadAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadAuth = r.Header["Authorization"]
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Empty store: no token resolves, so no Authorization header is sent. The
	// server-side 401/403 handling is the caller's concern, not the transport's.
	store := auth.NewStore(nil)
	rt, err := NewTransport(TransportConfig{Timeout: 5 * time.Second}, store)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}

	resp := doGet(t, rt, srv.URL+"/rest/api/space")
	_ = resp.Body.Close()

	if hadAuth {
		t.Errorf("Authorization header was set despite no stored credential")
	}
}

func Test_DebugTransport_redacts_bearer_token(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	var debugLog bytes.Buffer
	store := auth.NewStore(map[string]string{srv.URL: "SUPER-SECRET-TOKEN"})
	rt, err := NewTransport(TransportConfig{
		Timeout:  5 * time.Second,
		Debug:    true,
		DebugLog: &debugLog,
	}, store)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}

	resp := doGet(t, rt, srv.URL+"/rest/api/content/1")
	_ = resp.Body.Close()

	logged := debugLog.String()
	if strings.Contains(logged, "SUPER-SECRET-TOKEN") {
		t.Errorf("debug log leaked the token: %q", logged)
	}
	if !strings.Contains(logged, "Bearer ****") {
		t.Errorf("debug log should show a redacted Bearer header, got: %q", logged)
	}
	// Sanity: the debug log should record method and URL.
	if !strings.Contains(logged, "GET") || !strings.Contains(logged, "/rest/api/content/1") {
		t.Errorf("debug log should include method and URL, got: %q", logged)
	}
}

func Test_InsecureTransport_writes_warning_to_stderr(t *testing.T) {
	var warn bytes.Buffer
	store := auth.NewStore(nil)
	_, err := NewTransport(TransportConfig{
		Timeout:  5 * time.Second,
		Insecure: true,
		Warn:     &warn,
	}, store)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}

	if !strings.Contains(strings.ToLower(warn.String()), "verification") {
		t.Errorf("insecure mode should warn about disabled verification, got: %q", warn.String())
	}
}
