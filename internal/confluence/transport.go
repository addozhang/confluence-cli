// Package confluence issues HTTP requests to a Confluence Server/DC instance
// and returns raw response bytes. The transport stack injects the PAT as a
// Bearer header, configures TLS (SSL_CERT_FILE / --insecure), bounds requests
// with a timeout, and optionally logs the redacted HTTP exchange under --debug.
package confluence

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/addozhang/cfl/internal/auth"
	cflerrors "github.com/addozhang/cfl/internal/errors"
)

// TransportConfig configures the HTTP transport stack. Warn and DebugLog
// default to os.Stderr when nil.
type TransportConfig struct {
	// SSLCertFile is a PEM CA bundle to add to the system trust store. Empty
	// means use the system trust store alone.
	SSLCertFile string
	// Insecure disables TLS certificate verification (prints a warning).
	Insecure bool
	// Debug logs the redacted request/response to DebugLog.
	Debug bool
	// Timeout bounds every request. Applied by the client via context; a
	// non-zero value here is also used to size dial/idle behavior.
	Timeout time.Duration
	// Warn receives the --insecure warning. Defaults to os.Stderr.
	Warn io.Writer
	// DebugLog receives --debug output. Defaults to os.Stderr.
	DebugLog io.Writer
}

// NewTransport builds the RoundTripper stack: TLS config -> Bearer auth ->
// (optional) debug logging -> base http.Transport. It returns a CFLError with
// CodeTLS when SSL_CERT_FILE cannot be loaded.
func NewTransport(cfg TransportConfig, store *auth.Store) (http.RoundTripper, error) {
	tlsCfg, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, err
	}

	if cfg.Insecure {
		warn := cfg.Warn
		if warn == nil {
			warn = os.Stderr
		}
		_, _ = io.WriteString(warn, "warning: TLS certificate verification is disabled (--insecure)\n")
	}

	base := &http.Transport{
		TLSClientConfig:     tlsCfg,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	// Stack inner-to-outer: base transport, then (optional) debug logging, then
	// Bearer injection on top. Bearer must be outermost so the Authorization
	// header is already set when the debug layer logs the request — and the
	// debug layer always renders that header redacted.
	var rt http.RoundTripper = base

	if cfg.Debug {
		debugLog := cfg.DebugLog
		if debugLog == nil {
			debugLog = os.Stderr
		}
		rt = newDebugTransport(rt, debugLog)
	}

	rt = &bearerTransport{base: rt, store: store}

	return rt, nil
}

// buildTLSConfig returns a *tls.Config honoring SSL_CERT_FILE and --insecure.
func buildTLSConfig(cfg TransportConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}

	if cfg.Insecure {
		tlsCfg.InsecureSkipVerify = true
		return tlsCfg, nil
	}

	if cfg.SSLCertFile == "" {
		return tlsCfg, nil
	}

	pem, err := os.ReadFile(cfg.SSLCertFile)
	if err != nil {
		return nil, cflerrors.TranslateSSLCertFile(cfg.SSLCertFile, err)
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, cflerrors.TranslateSSLCertFile(cfg.SSLCertFile,
			fmt.Errorf("file is not a valid PEM certificate bundle"))
	}
	tlsCfg.RootCAs = pool
	return tlsCfg, nil
}

// bearerTransport resolves the request URL to a stored PAT and sets the
// Authorization header. It never sends Basic auth and never transmits a
// username. When no credential resolves, it leaves the header unset and lets
// the server respond (the CLI translates the resulting 401/403).
type bearerTransport struct {
	base  http.RoundTripper
	store *auth.Store
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.store != nil {
		if token, ok, _ := t.store.Resolve(req.URL.String()); ok && token != "" {
			// Clone to avoid mutating the caller's request (RoundTripper contract).
			req = req.Clone(req.Context())
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	return t.base.RoundTrip(req)
}
