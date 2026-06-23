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

// uniqueTitle returns a collision-resistant page title for a test run.
func uniqueTitle(prefix string) string {
	return "cfl-e2e-" + prefix + "-" + time.Now().UTC().Format("20060102-150405.000")
}

// createPage creates a page and registers cleanup; returns its id and version.
func (e *e2eEnv) createPage(t *testing.T, title, body string, extra ...string) (string, int) {
	t.Helper()
	args := append([]string{
		"page", "create",
		"--instance", e.baseURL,
		"--space", e.spaceKey,
		"--title", title,
		"--body", body,
		"-o", "json",
	}, extra...)
	out := e.run(t, false, nil, args...)
	var created struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("create output not JSON: %v\n%s", err, out)
	}
	if created.ID == "" {
		t.Fatalf("create did not return an id: %s", out)
	}
	t.Cleanup(func() { _ = e.run(t, false, nil, "page", "delete", created.ID, "--yes") })
	return created.ID, created.Version
}

// TestE2E_Page_create_with_parent verifies the parent/child relationship is
// real: the child reports the parent as its ancestor, and the parent's children
// listing includes the child. Neither can be faked by a mock.
func TestE2E_Page_create_with_parent(t *testing.T) {
	env := loadEnv(t)
	env.build(t)

	parentID, _ := env.createPage(t, uniqueTitle("parent"), "<p>parent</p>")
	childTitle := uniqueTitle("child")
	childID, _ := env.createPage(t, childTitle, "<p>child</p>", "--parent", parentID)

	// The child's ancestors must include the parent.
	childOut := env.run(t, false, nil, "page", "get", childID, "-o", "json")
	var child struct {
		ParentID  *string `json:"parentId"`
		Ancestors []struct {
			ID string `json:"id"`
		} `json:"ancestors"`
	}
	if err := json.Unmarshal([]byte(childOut), &child); err != nil {
		t.Fatalf("child get not JSON: %v\n%s", err, childOut)
	}
	if child.ParentID == nil || *child.ParentID != parentID {
		t.Errorf("child parentId = %v, want %s", child.ParentID, parentID)
	}
	foundAncestor := false
	for _, a := range child.Ancestors {
		if a.ID == parentID {
			foundAncestor = true
		}
	}
	if !foundAncestor {
		t.Errorf("child ancestors %v should include parent %s", child.Ancestors, parentID)
	}

	// The parent's children listing must include the child.
	childrenOut := env.run(t, false, nil, "page", "children", parentID, "-o", "json")
	if !strings.Contains(childrenOut, `"id":"`+childID+`"`) {
		t.Errorf("parent children should include child %s: %s", childID, childrenOut)
	}
}

// TestE2E_Page_get_by_display_url verifies the display-URL title-lookup round
// trip against a real server: cfl resolves space+title to an id, then reads it.
func TestE2E_Page_get_by_display_url(t *testing.T) {
	env := loadEnv(t)
	env.build(t)

	title := uniqueTitle("display")
	id, _ := env.createPage(t, title, "<p>display url target</p>")

	// Build a display URL with the '+'-encoded title.
	displayURL := env.baseURL + "/display/" + env.spaceKey + "/" + strings.ReplaceAll(title, " ", "+")
	out := env.run(t, false, nil, "page", "get", displayURL, "-o", "json")
	if !strings.Contains(out, `"id":"`+id+`"`) {
		t.Errorf("display-url get should resolve to id %s: %s", id, out)
	}
}

// TestE2E_Page_sequential_updates verifies repeated version-safe updates against
// a real server: each `cfl page update` re-reads the current version and submits
// current+1, so two sequential updates take the page v1 -> v2 -> v3 without the
// client ever guessing the number. (A deterministic stale-version 409 cannot be
// forced through the CLI alone because each update re-reads first; the 409
// translation path is covered by the Test_cmd_page_update_version_conflict unit
// test against an httptest server.)
func TestE2E_Page_sequential_updates(t *testing.T) {
	env := loadEnv(t)
	env.build(t)

	title := uniqueTitle("sequpdate")
	id, _ := env.createPage(t, title, "<p>v1</p>")

	env.run(t, false, nil, "page", "update", id, "--body", "<p>v2</p>")
	out := env.run(t, false, nil, "page", "update", id, "--body", "<p>v3</p>", "-o", "json")
	if !strings.Contains(out, `"version":3`) {
		t.Errorf("two sequential updates should reach version 3: %s", out)
	}
}

// TestE2E_Output_raw_and_yaml verifies the output formats against real data:
// -o raw returns the verbatim Confluence JSON (with internals like _links),
// while -o yaml is the schema view with schemaVersion on the first line.
func TestE2E_Output_raw_and_yaml(t *testing.T) {
	env := loadEnv(t)
	env.build(t)

	title := uniqueTitle("output")
	id, _ := env.createPage(t, title, "<p>output formats</p>")

	// -o raw: the verbatim Confluence response exposes internals the schema drops.
	rawOut := env.run(t, false, nil, "page", "get", id, "-o", "raw")
	if !strings.Contains(rawOut, "_links") && !strings.Contains(rawOut, "_expandable") {
		t.Errorf("-o raw should expose Confluence internals (_links/_expandable): %s", rawOut)
	}
	if strings.Contains(rawOut, "schemaVersion") {
		t.Errorf("-o raw must NOT inject schemaVersion: %s", rawOut)
	}

	// -o yaml: schema view, schemaVersion first line, no Confluence internals.
	yamlOut := env.run(t, false, nil, "page", "get", id, "-o", "yaml")
	if !strings.HasPrefix(strings.TrimLeft(yamlOut, "\n"), "schemaVersion:") {
		t.Errorf("-o yaml first line should be schemaVersion: %s", yamlOut)
	}
	if strings.Contains(yamlOut, "_expandable") {
		t.Errorf("-o yaml must not leak Confluence internals: %s", yamlOut)
	}
}

// TestE2E_Page_storage_format_roundtrip verifies a non-trivial storage-format
// body survives create -> read unchanged, proving cfl passes XHTML through and
// the server accepts it.
func TestE2E_Page_storage_format_roundtrip(t *testing.T) {
	env := loadEnv(t)
	env.build(t)

	title := uniqueTitle("storage")
	body := "<h2>Heading</h2><ul><li>one</li><li>two</li></ul><p><strong>bold</strong> and <em>italic</em></p>"
	id, _ := env.createPage(t, title, body)

	out := env.run(t, false, nil, "page", "get", id, "-o", "json")
	var page struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal([]byte(out), &page); err != nil {
		t.Fatalf("get not JSON: %v\n%s", err, out)
	}
	for _, fragment := range []string{"<h2>Heading</h2>", "<li>one</li>", "<strong>bold</strong>", "<em>italic</em>"} {
		if !strings.Contains(page.Body, fragment) {
			t.Errorf("stored body lost fragment %q; got: %s", fragment, page.Body)
		}
	}
}

// TestE2E_Bare_id_resolution verifies that a bare numeric page id resolves
// against the single configured instance and reads the right page.
func TestE2E_Bare_id_resolution(t *testing.T) {
	env := loadEnv(t)
	// This relies on exactly one instance being configured. The seeded creds
	// file may contain the TLS host too; skip if so to keep the bare-id rule
	// unambiguous.
	if env.tlsURL != "" {
		t.Skip("bare-id resolution requires a single configured instance; TLS tier also configured, skipping")
	}
	env.build(t)

	title := uniqueTitle("bareid")
	id, _ := env.createPage(t, title, "<p>bare id</p>")

	out := env.run(t, false, nil, "page", "get", id, "-o", "json")
	if !strings.Contains(out, `"id":"`+id+`"`) {
		t.Errorf("bare-id get should resolve to id %s: %s", id, out)
	}
}

// TestE2E_Auth_list verifies the configured instance key is listed and no token
// is leaked.
func TestE2E_Auth_list(t *testing.T) {
	env := loadEnv(t)
	env.build(t)

	out := env.run(t, false, nil, "auth", "list", "-o", "json")
	if !strings.Contains(out, hostKey(env.baseURL)) {
		t.Errorf("auth list should include the configured instance %s: %s", hostKey(env.baseURL), out)
	}
	if strings.Contains(out, env.token) {
		t.Errorf("auth list leaked the token: %s", out)
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

// TestE2E_SelfSigned_TLS_Mock verifies SSL_CERT_FILE and --insecure against the
// standalone self-signed TLS mock (test/e2e/docker-compose.tls.yml), which
// serves static Confluence-shaped JSON and needs NO Confluence and NO license.
//
// Unlike TestE2E_SelfSigned_TLS, this does not require a PAT or a real space, so
// it can run fully automatically. It is configured independently:
//
//	CFL_E2E_TLS_MOCK_URL  https://localhost:8444
//	CFL_E2E_CA_FILE       test/e2e/certs/wiki.local.crt
//
// It skips when CFL_E2E_TLS_MOCK_URL is unset.
func TestE2E_SelfSigned_TLS_Mock(t *testing.T) {
	mockURL := strings.TrimSpace(os.Getenv("CFL_E2E_TLS_MOCK_URL"))
	caFile := strings.TrimSpace(os.Getenv("CFL_E2E_CA_FILE"))
	if mockURL == "" {
		t.Skip("TLS mock not configured (set CFL_E2E_TLS_MOCK_URL, e.g. https://localhost:8444); skipping")
	}
	if caFile == "" {
		t.Fatal("CFL_E2E_TLS_MOCK_URL is set but CFL_E2E_CA_FILE is missing")
	}
	caAbs, err := filepath.Abs(caFile)
	if err != nil {
		t.Fatalf("abs ca file: %v", err)
	}

	// Build cfl and seed a credential for the mock host (any token works; the
	// mock does not check Authorization).
	dir := t.TempDir()
	binary := filepath.Join(dir, "cfl")
	if out, berr := exec.Command("go", "build", "-o", binary, "../../cmd/cfl").CombinedOutput(); berr != nil {
		t.Fatalf("build cfl: %v\n%s", berr, out)
	}
	creds := filepath.Join(dir, "credentials")
	if werr := os.WriteFile(creds,
		[]byte("[tokens]\n  \""+hostKey(mockURL)+"\" = \"tls-probe-token\"\n"), 0o600); werr != nil {
		t.Fatalf("seed credentials: %v", werr)
	}

	run := func(wantErr bool, extraEnv []string, args ...string) string {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Env = append(os.Environ(), "CFL_TEST_CREDENTIALS="+creds)
		cmd.Env = append(cmd.Env, extraEnv...)
		out, rerr := cmd.CombinedOutput()
		if wantErr && rerr == nil {
			t.Fatalf("expected `cfl %s` to fail, got success:\n%s", strings.Join(args, " "), out)
		}
		if !wantErr && rerr != nil {
			t.Fatalf("`cfl %s` failed: %v\n%s", strings.Join(args, " "), rerr, out)
		}
		return string(out)
	}

	// 1. SSL_CERT_FILE makes cfl trust the self-signed cert with no flag.
	out := run(false, []string{"SSL_CERT_FILE=" + caAbs},
		"space", "list", "--instance", mockURL, "-o", "json")
	if !strings.Contains(out, `"key":"TLS"`) {
		t.Errorf("space list over self-signed TLS with SSL_CERT_FILE should succeed: %s", out)
	}

	// 2. Without the CA, verification must fail — proving #1 succeeded because
	//    of the CA, not by accident.
	run(true, []string{"SSL_CERT_FILE="},
		"space", "list", "--instance", mockURL)

	// 3. --insecure bypasses verification and succeeds again.
	insecureOut := run(false, []string{"SSL_CERT_FILE="},
		"space", "list", "--instance", mockURL, "--insecure", "-o", "json")
	if !strings.Contains(insecureOut, `"key":"TLS"`) {
		t.Errorf("space list with --insecure should succeed over self-signed TLS: %s", insecureOut)
	}
}

// TestE2E_Search runs a keyword search restricted to the configured space.
func TestE2E_Search(t *testing.T) {
	env := loadEnv(t)
	env.build(t)

	// Create a page with a distinctive title, then search for it.
	title := uniqueTitle("searchme")
	id, _ := env.createPage(t, title, "<p>searchable body</p>")
	_ = id

	// Confluence search is eventually consistent; allow a brief indexing delay.
	time.Sleep(3 * time.Second)

	out := env.run(t, false, nil, "search", "searchme", "--space", env.spaceKey, "--instance", env.baseURL, "-o", "json")
	var res struct {
		Results []struct {
			Title string `json:"title"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("search output not JSON: %v\n%s", err, out)
	}
	// Index timing can vary; assert the call succeeded and returned the schema,
	// and log whether the just-created page was found.
	found := false
	for _, r := range res.Results {
		if strings.Contains(r.Title, "searchme") {
			found = true
		}
	}
	if !found {
		t.Logf("search did not yet surface %q (search index may lag); results=%d", title, len(res.Results))
	}
}

// TestE2E_Page_get_spaces_pages_url reads a page via the modern
// /spaces/KEY/pages/ID/Title URL against a real server.
func TestE2E_Page_get_spaces_pages_url(t *testing.T) {
	env := loadEnv(t)
	env.build(t)

	title := uniqueTitle("spacespages")
	id, _ := env.createPage(t, title, "<p>spaces/pages url target</p>")

	pageURL := env.baseURL + "/spaces/" + env.spaceKey + "/pages/" + id + "/" + strings.ReplaceAll(title, " ", "+")
	out := env.run(t, false, nil, "page", "get", pageURL, "-o", "json")
	if !strings.Contains(out, `"id":"`+id+`"`) {
		t.Errorf("spaces/pages URL get should resolve to id %s: %s", id, out)
	}
}

// TestE2E_Alias_roundtrip stores an alias, then uses it for --instance and the
// <alias>:<id> form against a real server.
func TestE2E_Alias_roundtrip(t *testing.T) {
	env := loadEnv(t)
	env.build(t)

	// Re-seed credentials with an alias for the base instance (the default seed
	// writes no alias). Write directly to keep the test non-interactive.
	creds := filepath.Join(env.credsDir, "credentials")
	var b strings.Builder
	b.WriteString("[tokens]\n  \"" + hostKey(env.baseURL) + "\" = \"" + env.token + "\"\n")
	b.WriteString("[aliases]\n  \"" + hostKey(env.baseURL) + "\" = \"e2eprod\"\n")
	if err := os.WriteFile(creds, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("seed alias creds: %v", err)
	}

	// auth list should show the alias.
	listOut := env.run(t, false, nil, "auth", "list", "-o", "json")
	if !strings.Contains(listOut, `"alias":"e2eprod"`) {
		t.Errorf("auth list should show the alias: %s", listOut)
	}

	// --instance <alias> resolves the instance for space list.
	spacesOut := env.run(t, false, nil, "space", "list", "--instance", "e2eprod", "-o", "json")
	if !strings.Contains(spacesOut, `"spaces"`) {
		t.Errorf("space list via alias should succeed: %s", spacesOut)
	}

	// <alias>:<id> reads a page on the aliased instance.
	title := uniqueTitle("aliasid")
	id, _ := env.createPage(t, title, "<p>alias id</p>")
	getOut := env.run(t, false, nil, "page", "get", "e2eprod:"+id, "-o", "json")
	if !strings.Contains(getOut, `"id":"`+id+`"`) {
		t.Errorf("<alias>:<id> get should resolve id %s: %s", id, getOut)
	}
}
