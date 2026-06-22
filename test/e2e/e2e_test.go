//go:build e2e

// Package e2e runs cfl against a real Confluence Server/DC instance brought up
// by test/e2e/docker-compose.yml. It is gated by the `e2e` build tag and is not
// part of `make test`.
//
// Prerequisites (see test/e2e/README.md):
//
//   - the docker-compose stack is up and Confluence is RUNNING
//
//   - a Personal Access Token has been created in the Confluence web UI
//
//   - the following environment variables are set:
//
//     CFL_E2E_BASE_URL   http://localhost:8090            (the direct HTTP instance)
//     CFL_E2E_TLS_URL    https://localhost:8443           (the self-signed nginx proxy)
//     CFL_E2E_CA_FILE    test/e2e/certs/wiki.local.crt    (the self-signed CA bundle)
//     CFL_E2E_TOKEN      <a valid PAT>
//     CFL_E2E_SPACE_KEY  <an existing space key, e.g. DS>
//
// Any missing variable skips the suite, so a plain `go test -tags=e2e` on a
// machine without the stack is a no-op rather than a failure.
package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type e2eEnv struct {
	baseURL  string
	tlsURL   string
	caFile   string
	token    string
	spaceKey string
	binary   string
	credsDir string
}

func loadEnv(t *testing.T) e2eEnv {
	t.Helper()
	get := func(k string) string { return strings.TrimSpace(os.Getenv(k)) }

	env := e2eEnv{
		baseURL:  get("CFL_E2E_BASE_URL"),
		tlsURL:   get("CFL_E2E_TLS_URL"),
		caFile:   get("CFL_E2E_CA_FILE"),
		token:    get("CFL_E2E_TOKEN"),
		spaceKey: get("CFL_E2E_SPACE_KEY"),
	}
	if env.baseURL == "" || env.token == "" || env.spaceKey == "" {
		t.Skip("e2e env not configured (set CFL_E2E_BASE_URL, CFL_E2E_TOKEN, CFL_E2E_SPACE_KEY); skipping")
	}
	return env
}

// build compiles the cfl binary once and seeds a credentials file for the
// instance(s) under test.
func (e *e2eEnv) build(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	e.binary = filepath.Join(dir, "cfl")
	e.credsDir = dir

	build := exec.Command("go", "build", "-o", e.binary, "../../cmd/cfl")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cfl: %v\n%s", err, out)
	}

	// Seed credentials for both the HTTP and TLS hosts using `auth add` would
	// require an interactive prompt; write the TOML directly instead.
	creds := filepath.Join(dir, "credentials")
	var b strings.Builder
	b.WriteString("[tokens]\n")
	b.WriteString("  \"" + hostKey(e.baseURL) + "\" = \"" + e.token + "\"\n")
	if e.tlsURL != "" {
		b.WriteString("  \"" + hostKey(e.tlsURL) + "\" = \"" + e.token + "\"\n")
	}
	if err := os.WriteFile(creds, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("seed credentials: %v", err)
	}
}

// run executes cfl with the seeded credentials and the given extra env, and
// returns combined output. It fails the test on a non-zero exit unless
// wantErr is true.
func (e *e2eEnv) run(t *testing.T, wantErr bool, extraEnv []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(e.binary, args...)
	cmd.Env = append(os.Environ(),
		"CFL_TEST_CREDENTIALS="+filepath.Join(e.credsDir, "credentials"),
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	out, err := cmd.CombinedOutput()
	if wantErr && err == nil {
		t.Fatalf("expected `cfl %s` to fail, but it succeeded:\n%s", strings.Join(args, " "), out)
	}
	if !wantErr && err != nil {
		t.Fatalf("`cfl %s` failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func TestE2E_Version_offline(t *testing.T) {
	env := loadEnv(t)
	env.build(t)
	out := env.run(t, false, nil, "version", "-o", "json")
	if !strings.Contains(out, `"schemaVersion":"1"`) {
		t.Errorf("version output missing schemaVersion: %s", out)
	}
}

func TestE2E_Whoami(t *testing.T) {
	env := loadEnv(t)
	env.build(t)
	out := env.run(t, false, nil, "auth", "whoami", env.baseURL, "-o", "json")
	if !strings.Contains(out, `"username"`) {
		t.Errorf("whoami should report a username: %s", out)
	}
	if strings.Contains(out, env.token) {
		t.Errorf("whoami leaked the token: %s", out)
	}
}

func TestE2E_SpaceList_and_Get(t *testing.T) {
	env := loadEnv(t)
	env.build(t)

	listOut := env.run(t, false, nil, "space", "list", "--instance", env.baseURL, "-o", "json")
	if !strings.Contains(listOut, `"spaces"`) {
		t.Errorf("space list missing spaces array: %s", listOut)
	}

	getOut := env.run(t, false, nil, "space", "get", env.spaceKey, "--instance", env.baseURL, "-o", "json")
	if !strings.Contains(getOut, `"key":"`+env.spaceKey+`"`) {
		t.Errorf("space get should return key %s: %s", env.spaceKey, getOut)
	}
}

// TestE2E_Page_full_lifecycle creates, reads, updates, lists children of, and
// deletes a page, asserting the version increments and the schema is stable.
func TestE2E_Page_full_lifecycle(t *testing.T) {
	env := loadEnv(t)
	env.build(t)

	title := "cfl-e2e-" + time.Now().UTC().Format("20060102-150405")
	createOut := env.run(t, false, nil,
		"page", "create",
		"--instance", env.baseURL,
		"--space", env.spaceKey,
		"--title", title,
		"--body", "<p>created by cfl e2e</p>",
		"-o", "json",
	)
	var created struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("create output not JSON: %v\n%s", err, createOut)
	}
	if created.ID == "" {
		t.Fatalf("create did not return an id: %s", createOut)
	}
	t.Cleanup(func() {
		_ = env.run(t, false, nil, "page", "delete", created.ID, "--yes")
	})

	// Read it back.
	getOut := env.run(t, false, nil, "page", "get", created.ID, "-o", "json")
	if !strings.Contains(getOut, `"id":"`+created.ID+`"`) {
		t.Errorf("page get should return the created id: %s", getOut)
	}

	// Update it; version must increment.
	updateOut := env.run(t, false, nil,
		"page", "update", created.ID,
		"--body", "<p>updated by cfl e2e</p>",
		"-o", "json",
	)
	var updated struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal([]byte(updateOut), &updated); err != nil {
		t.Fatalf("update output not JSON: %v\n%s", err, updateOut)
	}
	if updated.Version != created.Version+1 {
		t.Errorf("update version = %d, want %d (created %d + 1)", updated.Version, created.Version+1, created.Version)
	}

	// Children of the new page (none) must be an empty array.
	childrenOut := env.run(t, false, nil, "page", "children", created.ID, "-o", "json")
	if !strings.Contains(childrenOut, `"children":[]`) {
		t.Errorf("new page should have empty children: %s", childrenOut)
	}
}

func TestE2E_Page_not_found(t *testing.T) {
	env := loadEnv(t)
	env.build(t)
	out := env.run(t, true, nil, "page", "get", env.baseURL+"/pages/viewpage.action?pageId=999999999")
	if !strings.Contains(out, "Page not found") {
		t.Errorf("expected a 'Page not found' error, got: %s", out)
	}
}

// TestE2E_SelfSigned_TLS verifies the SSL_CERT_FILE and --insecure paths against
// the nginx self-signed proxy tier. Skipped unless the TLS env is configured.
func TestE2E_SelfSigned_TLS(t *testing.T) {
	env := loadEnv(t)
	if env.tlsURL == "" || env.caFile == "" {
		t.Skip("TLS tier not configured (set CFL_E2E_TLS_URL and CFL_E2E_CA_FILE); skipping")
	}
	env.build(t)

	// 1. With SSL_CERT_FILE pointing at the self-signed CA, no --insecure needed.
	caAbs, err := filepath.Abs(env.caFile)
	if err != nil {
		t.Fatalf("abs ca file: %v", err)
	}
	out := env.run(t, false, []string{"SSL_CERT_FILE=" + caAbs},
		"space", "list", "--instance", env.tlsURL, "-o", "json")
	if !strings.Contains(out, `"spaces"`) {
		t.Errorf("space list over self-signed TLS with SSL_CERT_FILE should succeed: %s", out)
	}

	// 2. Without SSL_CERT_FILE, verification must fail (proving it was the CA
	//    that made #1 work, not an accident).
	failOut := env.run(t, true, []string{"SSL_CERT_FILE="},
		"space", "list", "--instance", env.tlsURL)
	if !strings.Contains(strings.ToLower(failOut), "network") && !strings.Contains(strings.ToLower(failOut), "unexpected") {
		t.Logf("self-signed without CA produced: %s", failOut)
	}

	// 3. With --insecure, verification is bypassed and it succeeds again.
	insecureOut := env.run(t, false, []string{"SSL_CERT_FILE="},
		"space", "list", "--instance", env.tlsURL, "--insecure", "-o", "json")
	if !strings.Contains(insecureOut, `"spaces"`) {
		t.Errorf("space list with --insecure should succeed over self-signed TLS: %s", insecureOut)
	}
}

// hostKey reduces an instance URL to its credential key (scheme://host[:port]).
// It mirrors auth.KeyFromURL for the simple host-only case used here.
func hostKey(rawURL string) string {
	u := rawURL
	if i := strings.Index(u, "://"); i >= 0 {
		rest := u[i+3:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			rest = rest[:slash]
		}
		return u[:i+3] + rest
	}
	return u
}
