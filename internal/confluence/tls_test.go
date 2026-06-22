package confluence

import (
	"encoding/pem"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/addozhang/cfl/internal/auth"
	cflerrors "github.com/addozhang/cfl/internal/errors"
)

func Test_SSLCertFile_trusts_self_signed(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Write the server's self-signed cert to a PEM file to act as the CA bundle.
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, certPEM, 0o600); err != nil {
		t.Fatalf("write ca: %v", err)
	}

	store := auth.NewStore(map[string]string{srv.URL: "tok"})
	rt, err := NewTransport(TransportConfig{
		Timeout:     5 * time.Second,
		SSLCertFile: caPath,
	}, store)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/rest/api/space", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("request against self-signed server with SSL_CERT_FILE should succeed, got: %v", err)
	}
	_ = resp.Body.Close()
}

func Test_SSLCertFile_missing_path_errors(t *testing.T) {
	store := auth.NewStore(nil)
	_, err := NewTransport(TransportConfig{
		Timeout:     5 * time.Second,
		SSLCertFile: filepath.Join(t.TempDir(), "nope.pem"),
	}, store)
	if err == nil {
		t.Fatalf("NewTransport with a missing SSL_CERT_FILE should error")
	}
	var cflErr *cflerrors.CFLError
	if !stderrors.As(err, &cflErr) || cflErr.Code != cflerrors.CodeTLS {
		t.Errorf("error = %v, want a CFLError with CodeTLS", err)
	}
}

func Test_SSLCertFile_bad_pem_errors(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(badPath, []byte("this is not a PEM bundle"), 0o600); err != nil {
		t.Fatalf("write bad pem: %v", err)
	}

	store := auth.NewStore(nil)
	_, err := NewTransport(TransportConfig{
		Timeout:     5 * time.Second,
		SSLCertFile: badPath,
	}, store)
	if err == nil {
		t.Fatalf("NewTransport with a non-PEM SSL_CERT_FILE should error")
	}
	var cflErr *cflerrors.CFLError
	if !stderrors.As(err, &cflErr) || cflErr.Code != cflerrors.CodeTLS {
		t.Errorf("error = %v, want a CFLError with CodeTLS", err)
	}
}
