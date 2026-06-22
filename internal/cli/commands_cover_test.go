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

func Test_cmd_auth_whoami_happy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/user/current" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"username":"jdoe","displayName":"Jane Doe"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	out, _, err := runCmd(t, creds, "", "auth", "whoami", srv.URL, "-o", "json")
	if err != nil {
		t.Fatalf("whoami error: %v", err)
	}
	if !strings.Contains(out, `"username":"jdoe"`) {
		t.Errorf("whoami output = %s, want username jdoe", out)
	}
}

func Test_cmd_auth_whoami_expired_token(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	_, _, err := runCmd(t, creds, "", "auth", "whoami", srv.URL)
	if err == nil {
		t.Fatalf("whoami with a rejected token should error")
	}
	// An auth rejection should suggest re-running auth add.
	if !strings.Contains(err.Error(), "rejected") {
		t.Errorf("error = %q, want a token-rejected message", err.Error())
	}
}

func Test_cmd_page_create_body_from_stdin(t *testing.T) {
	var sentBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			sentBody, _ = readAll(r)
		}
		_, _ = w.Write([]byte(`{"id":"902","title":"Piped","space":{"key":"ENG"},"version":{"number":1}}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	_, _, err := runCmd(t, creds, "<p>piped body</p>",
		"page", "create",
		"--instance", srv.URL,
		"--space", "ENG",
		"--title", "Piped",
		"--body", "-",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("page create from stdin error: %v", err)
	}
	if !strings.Contains(string(sentBody), "piped body") {
		t.Errorf("request body should carry the piped stdin: %s", sentBody)
	}
}

func Test_cmd_page_update_preserves_title(t *testing.T) {
	var putTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"12345","title":"Runbook","version":{"number":3}}`))
			return
		}
		body, _ := readAll(r)
		var sent map[string]any
		_ = json.Unmarshal(body, &sent)
		if s, ok := sent["title"].(string); ok {
			putTitle = s
		}
		_, _ = w.Write([]byte(`{"id":"12345","title":"Runbook","space":{"key":"ENG"},"version":{"number":4}}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)
	url := srv.URL + "/pages/viewpage.action?pageId=12345"

	// No --title: the current title must be preserved in the PUT.
	_, _, err := runCmd(t, creds, "", "page", "update", url, "--body", "<p>x</p>")
	if err != nil {
		t.Fatalf("page update error: %v", err)
	}
	if putTitle != "Runbook" {
		t.Errorf("PUT title = %q, want preserved 'Runbook'", putTitle)
	}
}

func Test_cmd_space_list_ambiguous_without_instance(t *testing.T) {
	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")
	// Two instances configured, no --instance: ambiguous.
	store := auth.NewStore(map[string]string{
		"https://a.example.com": "t1",
		"https://b.example.com": "t2",
	})
	if err := store.Save(creds); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, _, err := runCmd(t, creds, "", "space", "list")
	if err == nil {
		t.Fatalf("space list with two instances and no --instance should be ambiguous")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
		t.Errorf("error = %q, want ambiguity message", err.Error())
	}
}

func Test_cmd_space_get_no_instance_configured(t *testing.T) {
	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")

	_, _, err := runCmd(t, creds, "", "space", "get", "ENG")
	if err == nil {
		t.Fatalf("space get with no instance configured should error")
	}
	if !strings.Contains(err.Error(), "cfl auth add") && !strings.Contains(strings.ToLower(err.Error()), "no confluence instance") {
		t.Errorf("error = %q, want guidance to configure an instance", err.Error())
	}
}

func Test_cmd_page_get_bare_id_single_instance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/api/content/777") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"777","title":"Bare","space":{"key":"ENG"},"version":{"number":1},"ancestors":[],"body":{"storage":{"value":"<p/>","representation":"storage"}}}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	// A bare numeric id resolves against the single configured instance.
	out, _, err := runCmd(t, creds, "", "page", "get", "777", "-o", "json")
	if err != nil {
		t.Fatalf("page get by bare id error: %v", err)
	}
	if !strings.Contains(out, `"id":"777"`) {
		t.Errorf("output should carry id 777, got: %s", out)
	}
}
