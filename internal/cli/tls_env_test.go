package cli

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test_SSL_CERT_FILE_env_is_honored proves that setting the SSL_CERT_FILE
// environment variable (with no flag) lets cfl trust a self-signed Confluence,
// per the tls-and-transport spec ("honored without any additional flag").
func Test_SSL_CERT_FILE_env_is_honored(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"key":"ENG","name":"Engineering","type":"global"}],"start":0,"limit":25,"size":1}`))
	}))
	defer srv.Close()

	// Write the server's self-signed cert as the CA bundle.
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, certPEM, 0o600); err != nil {
		t.Fatalf("write ca: %v", err)
	}
	t.Setenv("SSL_CERT_FILE", caPath)

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	// No --insecure: success must come from trusting the CA via SSL_CERT_FILE.
	out, _, err := runCmd(t, creds, "", "space", "list", "--instance", srv.URL, "-o", "json")
	if err != nil {
		t.Fatalf("space list over self-signed TLS with SSL_CERT_FILE should succeed, got: %v", err)
	}
	if !strings.Contains(out, `"key":"ENG"`) {
		t.Errorf("output should carry the space, got: %s", out)
	}
}

// Test_SSL_CERT_FILE_invalid_path_errors proves an unreadable SSL_CERT_FILE
// surfaces a clear TLS error rather than a generic failure.
func Test_SSL_CERT_FILE_invalid_path_errors(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("SSL_CERT_FILE", filepath.Join(t.TempDir(), "missing.pem"))

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	_, _, err := runCmd(t, creds, "", "space", "list", "--instance", srv.URL)
	if err == nil {
		t.Fatalf("an invalid SSL_CERT_FILE path should error")
	}
	if !strings.Contains(err.Error(), "SSL_CERT_FILE") {
		t.Errorf("error = %q, should mention SSL_CERT_FILE", err.Error())
	}
}

// Test_insecure_flag_allows_self_signed proves --insecure bypasses verification
// even without SSL_CERT_FILE.
func Test_insecure_flag_allows_self_signed(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[],"start":0,"limit":25,"size":0}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	out, errOut, err := runCmd(t, creds, "", "space", "list", "--instance", srv.URL, "--insecure", "-o", "json")
	if err != nil {
		t.Fatalf("space list with --insecure should succeed against self-signed, got: %v", err)
	}
	if !strings.Contains(out, `"spaces"`) {
		t.Errorf("output should be the space list, got: %s", out)
	}
	// The insecure warning must go to stderr, not stdout.
	if strings.Contains(out, "verification") {
		t.Errorf("insecure warning leaked into stdout: %s", out)
	}
	if !strings.Contains(strings.ToLower(errOut), "verification") {
		t.Errorf("insecure warning should appear on stderr, got: %q", errOut)
	}
}
