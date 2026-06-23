package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/addozhang/cfl/internal/auth"
)

func Test_cmd_auth_add_stores_token_via_prompt(t *testing.T) {
	// Substitute the hidden prompt with a deterministic token.
	orig := promptHiddenToken
	promptHiddenToken = func(string) (string, error) { return "PAT-xyz", nil }
	defer func() { promptHiddenToken = orig }()

	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")

	out, _, err := runCmd(t, creds, "", "auth", "add", "https://wiki.example.com")
	if err != nil {
		t.Fatalf("auth add error: %v", err)
	}
	if !strings.Contains(out, "Stored token for https://wiki.example.com") {
		t.Errorf("output = %q, want a stored-token confirmation", out)
	}

	// The token must be persisted and resolvable, and never printed.
	if strings.Contains(out, "PAT-xyz") {
		t.Errorf("auth add leaked the token to stdout: %q", out)
	}
	store, lerr := auth.Load(creds)
	if lerr != nil {
		t.Fatalf("load creds: %v", lerr)
	}
	if tok, ok, _ := store.Resolve("https://wiki.example.com/x"); !ok || tok != "PAT-xyz" {
		t.Errorf("stored token = (%q, %v), want (PAT-xyz, true)", tok, ok)
	}
}

func Test_cmd_auth_list_prints_keys_not_tokens(t *testing.T) {
	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")
	if err := seedCredential(creds, "https://wiki.example.com", "SECRET"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	out, _, err := runCmd(t, creds, "", "auth", "list", "-o", "json")
	if err != nil {
		t.Fatalf("auth list error: %v", err)
	}
	if strings.Contains(out, "SECRET") {
		t.Errorf("auth list leaked a token: %q", out)
	}
	var decoded struct {
		Instances []struct {
			Key   string  `json:"key"`
			Alias *string `json:"alias"`
		} `json:"instances"`
	}
	if jerr := json.Unmarshal([]byte(out), &decoded); jerr != nil {
		t.Fatalf("output not JSON: %v", jerr)
	}
	if len(decoded.Instances) != 1 || decoded.Instances[0].Key != "https://wiki.example.com" {
		t.Errorf("instances = %v, want one key", decoded.Instances)
	}
}

func Test_cmd_auth_remove_idempotent(t *testing.T) {
	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")
	if err := seedCredential(creds, "https://wiki.example.com", "tok"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// First removal succeeds.
	if _, _, err := runCmd(t, creds, "", "auth", "remove", "https://wiki.example.com"); err != nil {
		t.Fatalf("first remove error: %v", err)
	}
	// Second removal is idempotent (still exit 0).
	out, _, err := runCmd(t, creds, "", "auth", "remove", "https://wiki.example.com")
	if err != nil {
		t.Fatalf("second remove should be idempotent, got: %v", err)
	}
	if !strings.Contains(out, "nothing to remove") {
		t.Errorf("output = %q, want an idempotent no-op message", out)
	}
}

func Test_cmd_page_get_by_display_url_resolves_title(t *testing.T) {
	var sawLookup bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/content" && r.URL.Query().Get("title") == "Runbook":
			sawLookup = true
			_, _ = w.Write([]byte(`{"results":[{"id":"555"}]}`))
		case strings.HasPrefix(r.URL.Path, "/rest/api/content/555"):
			_, _ = w.Write([]byte(`{"id":"555","title":"Runbook","space":{"key":"ENG"},"version":{"number":2},"ancestors":[],"body":{"storage":{"value":"<p/>","representation":"storage"}}}`))
		default:
			t.Errorf("unexpected path %s?%s", r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)
	url := srv.URL + "/display/ENG/Runbook"

	out, _, err := runCmd(t, creds, "", "page", "get", url, "-o", "json")
	if err != nil {
		t.Fatalf("page get by display url error: %v", err)
	}
	if !sawLookup {
		t.Errorf("display URL should trigger a title lookup first")
	}
	if !strings.Contains(out, `"id":"555"`) {
		t.Errorf("output should carry the resolved id 555, got: %s", out)
	}
}

func Test_cmd_page_get_display_url_no_match(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)
	url := srv.URL + "/display/ENG/Missing"

	_, _, err := runCmd(t, creds, "", "page", "get", url)
	if err == nil {
		t.Fatalf("a display URL with no matching page should error")
	}
	if !strings.Contains(err.Error(), "Missing") || !strings.Contains(err.Error(), "ENG") {
		t.Errorf("error = %q, should name the title and space", err.Error())
	}
}

func Test_cmd_page_children_empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)
	url := srv.URL + "/pages/viewpage.action?pageId=12345"

	out, _, err := runCmd(t, creds, "", "page", "children", url, "-o", "json")
	if err != nil {
		t.Fatalf("page children error: %v", err)
	}
	if !strings.Contains(out, `"children":[]`) {
		t.Errorf("empty children must encode as [], got: %s", out)
	}
}

func Test_cmd_page_create_top_level(t *testing.T) {
	var sentBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			sentBody, _ = readAll(r)
		}
		_, _ = w.Write([]byte(`{"id":"900","title":"Release Notes","space":{"key":"ENG"},"version":{"number":1},"_links":{"base":"https://wiki.example.com","webui":"/display/ENG/Release+Notes"}}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	out, _, err := runCmd(t, creds, "", "page", "create",
		"--instance", srv.URL,
		"--space", "ENG",
		"--title", "Release Notes",
		"--body", "<p>hi</p>",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("page create error: %v", err)
	}

	var sent map[string]any
	_ = json.Unmarshal(sentBody, &sent)
	if _, hasAncestors := sent["ancestors"]; hasAncestors {
		t.Errorf("top-level create must not include ancestors")
	}
	if !strings.Contains(out, `"id":"900"`) {
		t.Errorf("output should carry created id 900, got: %s", out)
	}
}

func Test_cmd_page_create_body_from_file(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"901","title":"FromFile","space":{"key":"ENG"},"version":{"number":1}}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)
	bodyFile := filepath.Join(dir, "body.xhtml")
	if err := writeFile(bodyFile, "<p>from file</p>"); err != nil {
		t.Fatalf("write body file: %v", err)
	}

	_, _, err := runCmd(t, creds, "", "page", "create",
		"--instance", srv.URL,
		"--space", "ENG",
		"--title", "FromFile",
		"--body", "@"+bodyFile,
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("page create from file error: %v", err)
	}
}
