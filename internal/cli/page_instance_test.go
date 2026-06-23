package cli

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/addozhang/cfl/internal/auth"
)

// pageByIDServer responds to a content-by-id read.
func pageByIDServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/api/content/12345") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"12345","title":"T","space":{"key":"ENG"},"version":{"number":1},"ancestors":[],"body":{"storage":{"value":"<p/>","representation":"storage"}}}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func Test_page_bare_id_multi_instance_requires_instance(t *testing.T) {
	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")
	s := auth.NewStore(nil)
	s.Add("https://a.example.com", "t1")
	s.Add("https://b.example.com", "t2")
	if err := s.Save(creds); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, _, err := runCmd(t, creds, "", "page", "get", "12345")
	if err == nil {
		t.Fatalf("bare id with multiple instances and no --instance should error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "instance") {
		t.Errorf("error %q should ask for an instance", err.Error())
	}
}

func Test_page_bare_id_with_instance_flag(t *testing.T) {
	srv := pageByIDServer(t)
	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")
	s := auth.NewStore(nil)
	s.Add(srv.URL, "tok")
	s.Add("https://other.example.com", "tok2") // a second instance
	if err := s.Save(creds); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// --instance picks the target unambiguously for a bare id.
	out, _, err := runCmd(t, creds, "", "page", "get", "12345", "--instance", srv.URL, "-o", "json")
	if err != nil {
		t.Fatalf("page get 12345 --instance error: %v", err)
	}
	if !strings.Contains(out, `"id":"12345"`) {
		t.Errorf("output should carry id 12345, got: %s", out)
	}
}

func Test_page_bare_id_with_instance_alias(t *testing.T) {
	srv := pageByIDServer(t)
	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")
	s := auth.NewStore(nil)
	_ = s.AddWithAlias(srv.URL, "tok", "prod")
	s.Add("https://other.example.com", "tok2")
	if err := s.Save(creds); err != nil {
		t.Fatalf("seed: %v", err)
	}

	out, _, err := runCmd(t, creds, "", "page", "get", "12345", "--instance", "prod", "-o", "json")
	if err != nil {
		t.Fatalf("page get 12345 --instance prod error: %v", err)
	}
	if !strings.Contains(out, `"id":"12345"`) {
		t.Errorf("output should carry id 12345, got: %s", out)
	}
}

func Test_page_bare_id_single_instance_no_flag_ok(t *testing.T) {
	srv := pageByIDServer(t)
	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL) // exactly one instance

	// Single instance: bare id works without --instance.
	out, _, err := runCmd(t, creds, "", "page", "get", "12345", "-o", "json")
	if err != nil {
		t.Fatalf("page get 12345 (single instance) error: %v", err)
	}
	if !strings.Contains(out, `"id":"12345"`) {
		t.Errorf("output should carry id 12345, got: %s", out)
	}
}

func Test_page_url_ignores_instance_flag(t *testing.T) {
	// The URL carries its own host; a conflicting --instance must be ignored,
	// and the request must go to the URL's host.
	srv := pageByIDServer(t)
	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")
	s := auth.NewStore(nil)
	s.Add(srv.URL, "tok")
	_ = s.AddWithAlias("https://wrong.example.com", "tok2", "wrong")
	if err := s.Save(creds); err != nil {
		t.Fatalf("seed: %v", err)
	}

	pageURL := srv.URL + "/spaces/ENG/pages/12345/T"
	// --instance points elsewhere, but the URL host must win.
	out, _, err := runCmd(t, creds, "", "page", "get", pageURL, "--instance", "wrong", "-o", "json")
	if err != nil {
		t.Fatalf("page get <url> --instance wrong error: %v", err)
	}
	if !strings.Contains(out, `"id":"12345"`) {
		t.Errorf("URL host should win over --instance, got: %s", out)
	}
}
