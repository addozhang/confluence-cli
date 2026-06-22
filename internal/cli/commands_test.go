package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// runCmd drives the root command with an isolated credentials file, returning
// stdout, stderr, and the error. A non-terminal stdin (bytes.Reader) makes
// delete refuse without --yes, matching the non-interactive contract.
func runCmd(t *testing.T, credsPath string, stdin string, args ...string) (string, string, error) {
	t.Helper()
	root := NewRootCmd()

	// Inject the credentials path into every command's Deps by setting the
	// persistent flag hook: we recreate the tree with a known path via env.
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)

	// The credentials path is threaded through a test-only env var read by
	// Deps in tests (see deps_test_hook.go).
	t.Setenv("CFL_TEST_CREDENTIALS", credsPath)

	err := root.Execute()
	return out.String(), errBuf.String(), err
}

func writeCreds(t *testing.T, dir, instanceURL string) string {
	t.Helper()
	path := filepath.Join(dir, "credentials")
	// Use the auth package indirectly by running `auth add` would prompt; here
	// we write the file directly via the store for setup speed.
	if err := seedCredential(path, instanceURL, "tok"); err != nil {
		t.Fatalf("seed credential: %v", err)
	}
	return path
}

func Test_cmd_space_list_happy_path(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/space" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"results":[{"key":"ENG","name":"Engineering","type":"global"}],"start":0,"limit":25,"size":1}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	out, _, err := runCmd(t, creds, "", "space", "list", "--instance", srv.URL, "-o", "json")
	if err != nil {
		t.Fatalf("space list error: %v", err)
	}

	var decoded struct {
		SchemaVersion string `json:"schemaVersion"`
		Spaces        []struct {
			Key  string `json:"key"`
			Type string `json:"type"`
		} `json:"spaces"`
	}
	if jerr := json.Unmarshal([]byte(out), &decoded); jerr != nil {
		t.Fatalf("output not JSON: %v (%q)", jerr, out)
	}
	if decoded.SchemaVersion != "1" {
		t.Errorf("schemaVersion = %q, want 1", decoded.SchemaVersion)
	}
	if len(decoded.Spaces) != 1 || decoded.Spaces[0].Key != "ENG" {
		t.Errorf("spaces = %+v, want one ENG", decoded.Spaces)
	}
}

func Test_cmd_space_get_not_found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"No space"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	_, _, err := runCmd(t, creds, "", "space", "get", "NOPE", "--instance", srv.URL)
	if err == nil {
		t.Fatalf("expected an error for a 404 space")
	}
	if !strings.Contains(err.Error(), "Space not found") {
		t.Errorf("error = %q, want 'Space not found'", err.Error())
	}
}

func Test_cmd_page_get_by_pageid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/api/content/12345") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
            "id":"12345","title":"Runbook",
            "space":{"key":"ENG"},
            "version":{"number":7},
            "ancestors":[{"id":"100","title":"Home"}],
            "body":{"storage":{"value":"<p>hi</p>","representation":"storage"}},
            "_links":{"base":"https://wiki.example.com","webui":"/display/ENG/Runbook"}
        }`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)
	url := srv.URL + "/pages/viewpage.action?pageId=12345"

	out, _, err := runCmd(t, creds, "", "page", "get", url, "-o", "json")
	if err != nil {
		t.Fatalf("page get error: %v", err)
	}
	var page struct {
		ID       string `json:"id"`
		Version  int    `json:"version"`
		ParentID string `json:"parentId"`
	}
	if jerr := json.Unmarshal([]byte(out), &page); jerr != nil {
		t.Fatalf("output not JSON: %v (%q)", jerr, out)
	}
	if page.ID != "12345" || page.Version != 7 {
		t.Errorf("page = %+v, want id=12345 version=7", page)
	}
}

func Test_cmd_page_update_increments_version(t *testing.T) {
	var putVersion float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// version read
			_, _ = w.Write([]byte(`{"id":"12345","title":"Runbook","version":{"number":7}}`))
		case http.MethodPut:
			body, _ := readAll(r)
			var sent map[string]any
			_ = json.Unmarshal(body, &sent)
			if v, ok := sent["version"].(map[string]any); ok {
				putVersion, _ = v["number"].(float64)
			}
			_, _ = w.Write([]byte(`{"id":"12345","title":"Runbook","space":{"key":"ENG"},"version":{"number":8}}`))
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)
	url := srv.URL + "/pages/viewpage.action?pageId=12345"

	out, _, err := runCmd(t, creds, "", "page", "update", url, "--body", "<p>new</p>", "-o", "json")
	if err != nil {
		t.Fatalf("page update error: %v", err)
	}
	if putVersion != 8 {
		t.Errorf("PUT version.number = %v, want 8 (current 7 + 1)", putVersion)
	}
	if !strings.Contains(out, `"version":8`) {
		t.Errorf("output should report version 8, got: %s", out)
	}
}

func Test_cmd_page_update_version_conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"12345","title":"Runbook","version":{"number":7}}`))
			return
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"version conflict"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)
	url := srv.URL + "/pages/viewpage.action?pageId=12345"

	_, _, err := runCmd(t, creds, "", "page", "update", url, "--body", "<p>x</p>")
	if err == nil {
		t.Fatalf("expected a version-conflict error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "version conflict") {
		t.Errorf("error = %q, want a version conflict message", err.Error())
	}
}

func Test_cmd_page_delete_non_interactive_without_yes_refuses(t *testing.T) {
	var deleteCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)
	url := srv.URL + "/pages/viewpage.action?pageId=12345"

	// stdin is a strings.Reader (not a terminal) -> non-interactive.
	_, _, err := runCmd(t, creds, "", "page", "delete", url)
	if err == nil {
		t.Fatalf("non-interactive delete without --yes should error")
	}
	if deleteCalled {
		t.Errorf("DELETE must NOT be issued without confirmation")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error = %q, should instruct to pass --yes", err.Error())
	}
}

func Test_cmd_page_delete_with_yes(t *testing.T) {
	var deleteCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)
	url := srv.URL + "/pages/viewpage.action?pageId=12345"

	out, _, err := runCmd(t, creds, "", "page", "delete", url, "--yes", "-o", "json")
	if err != nil {
		t.Fatalf("page delete --yes error: %v", err)
	}
	if !deleteCalled {
		t.Errorf("DELETE should be issued with --yes")
	}
	if !strings.Contains(out, `"status":"trashed"`) {
		t.Errorf("output should confirm trashed, got: %s", out)
	}
}

func Test_cmd_no_credential_gives_onboarding(t *testing.T) {
	dir := t.TempDir()
	// Empty credentials file (no instances).
	creds := filepath.Join(dir, "credentials")

	_, _, err := runCmd(t, creds, "", "space", "list", "--instance", "https://wiki.example.com")
	if err == nil {
		t.Fatalf("expected onboarding error with no credentials")
	}
	if !strings.Contains(err.Error(), "cfl auth add https://wiki.example.com") {
		t.Errorf("error = %q, should name the exact auth add command", err.Error())
	}
}
