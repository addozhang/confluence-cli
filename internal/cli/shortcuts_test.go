package cli

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/addozhang/cfl/internal/auth"
)

// Test_flag_shortcuts verifies the short forms of common flags work the same as
// their long forms, and that no command's flag registration panics on a
// shorthand collision (cobra panics at construction if two flags share a
// letter, so simply building the root command exercises that).
func Test_flag_shortcuts_root_builds(t *testing.T) {
	// Building the full command tree would panic on any shorthand collision.
	_ = NewRootCmd()
}

func Test_flag_shortcut_instance_on_page_get(t *testing.T) {
	srv := pageByIDServer(t)
	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")
	s := auth.NewStore(nil)
	_ = s.AddWithAlias(srv.URL, "tok", "prod")
	s.Add("https://other.example.com", "tok2")
	if err := s.Save(creds); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// -i is the short form of --instance.
	out, _, err := runCmd(t, creds, "", "page", "get", "12345", "-i", "prod", "-o", "json")
	if err != nil {
		t.Fatalf("page get -i prod error: %v", err)
	}
	if !strings.Contains(out, `"id":"12345"`) {
		t.Errorf("output should carry id 12345, got: %s", out)
	}
}

func Test_flag_shortcuts_page_create(t *testing.T) {
	var sentBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			sentBody, _ = readAll(r)
		}
		_, _ = w.Write([]byte(`{"id":"900","title":"Short","space":{"key":"ENG"},"version":{"number":1}}`))
	}))
	defer srv.Close()
	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	// -s space, -t title, -b body, -i instance, -p parent.
	_, _, err := runCmd(t, creds, "", "page", "create",
		"-i", srv.URL, "-s", "ENG", "-t", "Short", "-b", "<p>x</p>", "-p", "42",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("page create with shortcuts error: %v", err)
	}
	if !strings.Contains(string(sentBody), `"key":"ENG"`) {
		t.Errorf("-s space shortcut did not set space.key: %s", sentBody)
	}
	if !strings.Contains(string(sentBody), `"id":"42"`) {
		t.Errorf("-p parent shortcut not applied: %s", sentBody)
	}
	// The body is JSON-encoded, so the XHTML is unicode-escaped; assert on the
	// representation marker rather than the raw angle brackets.
	if !strings.Contains(string(sentBody), `"representation":"storage"`) {
		t.Errorf("-b body shortcut did not set the storage body: %s", sentBody)
	}
}

func Test_flag_shortcuts_search(t *testing.T) {
	var cql string
	srv := searchServer(t, &cql)
	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	// -s space, -l limit, -i instance.
	_, _, err := runCmd(t, creds, "", "search", "runbook", "-s", "ENG", "-l", "5", "-i", srv.URL, "-o", "json")
	if err != nil {
		t.Fatalf("search with shortcuts error: %v", err)
	}
	if !strings.Contains(cql, `space = "ENG"`) {
		t.Errorf("-s space shortcut not applied to CQL: %q", cql)
	}
}

func Test_flag_shortcut_delete_yes(t *testing.T) {
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

	// -y is the short form of --yes.
	_, _, err := runCmd(t, creds, "", "page", "delete", url, "-y", "-o", "json")
	if err != nil {
		t.Fatalf("page delete -y error: %v", err)
	}
	if !deleteCalled {
		t.Errorf("-y should confirm deletion")
	}
}

func Test_flag_shortcut_space_list_limit(t *testing.T) {
	var gotLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		_, _ = w.Write([]byte(`{"results":[],"start":0,"limit":7,"size":0}`))
	}))
	defer srv.Close()
	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	_, _, err := runCmd(t, creds, "", "space", "list", "-i", srv.URL, "-l", "7", "-o", "json")
	if err != nil {
		t.Fatalf("space list -l error: %v", err)
	}
	if gotLimit != "7" {
		t.Errorf("-l limit shortcut not applied, got limit=%q", gotLimit)
	}
}
