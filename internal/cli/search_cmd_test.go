package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/addozhang/cfl/internal/auth"
)

// searchServer records the cql it received and returns a canned result set.
func searchServer(t *testing.T, capturedCQL *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/search" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		*capturedCQL = r.URL.Query().Get("cql")
		_, _ = w.Write([]byte(`{"results":[{"content":{"id":"1","type":"page","title":"Hit","space":{"key":"ENG"}},"title":"Hit","url":"/spaces/ENG/pages/1/Hit"}],"start":0,"limit":25,"size":1}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func Test_cmd_search_within_space(t *testing.T) {
	var cql string
	srv := searchServer(t, &cql)
	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	out, _, err := runCmd(t, creds, "", "search", "release notes", "--space", "ENG", "--instance", srv.URL, "-o", "json")
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if !strings.Contains(cql, `space = "ENG"`) || !strings.Contains(cql, `text ~ "release notes"`) {
		t.Errorf("built CQL = %q, want space + text clauses", cql)
	}
	if !strings.Contains(cql, `type = "page"`) {
		t.Errorf("built CQL = %q, want default type=page", cql)
	}

	var decoded struct {
		SchemaVersion string `json:"schemaVersion"`
		Results       []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			SpaceKey string `json:"spaceKey"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output not JSON: %v (%q)", err, out)
	}
	if decoded.SchemaVersion != "1" || len(decoded.Results) != 1 || decoded.Results[0].ID != "1" {
		t.Errorf("unexpected results: %q", out)
	}
}

func Test_cmd_search_pagination(t *testing.T) {
	var captured url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.Query()
		_, _ = w.Write([]byte(`{"results":[],"start":20,"limit":5,"size":0}`))
	}))
	defer srv.Close()
	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	_, _, err := runCmd(t, creds, "", "search", "doc", "--limit", "5", "--start", "20", "--instance", srv.URL, "-o", "json")
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if captured.Get("limit") != "5" || captured.Get("start") != "20" {
		t.Errorf("pagination params = %v, want limit=5 start=20", captured)
	}
}

func Test_cmd_search_empty_results(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[],"start":0,"limit":25,"size":0}`))
	}))
	defer srv.Close()
	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	out, _, err := runCmd(t, creds, "", "search", "nothingxyz", "--instance", srv.URL, "-o", "json")
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if !strings.Contains(out, `"results":[]`) {
		t.Errorf("empty search must encode results as [], got: %s", out)
	}
}

func Test_cmd_search_cql_passthrough(t *testing.T) {
	var cql string
	srv := searchServer(t, &cql)
	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	raw := `space = ENG AND title ~ "runbook" AND created > now("-7d")`
	_, _, err := runCmd(t, creds, "", "search", "--cql", raw, "--instance", srv.URL, "-o", "json")
	if err != nil {
		t.Fatalf("search --cql error: %v", err)
	}
	if cql != raw {
		t.Errorf("--cql query = %q, want it passed through verbatim (%q)", cql, raw)
	}
}

func Test_cmd_search_cql_overrides_friendly(t *testing.T) {
	var cql string
	srv := searchServer(t, &cql)
	dir := t.TempDir()
	creds := writeCreds(t, dir, srv.URL)

	// --cql present together with text/--space/--type: --cql wins, others ignored.
	out, errOut, err := runCmd(t, creds, "",
		"search", "ignored-text", "--space", "OPS", "--type", "blogpost",
		"--cql", "type = page AND space = ENG",
		"--instance", srv.URL, "-o", "json",
	)
	if err != nil {
		t.Fatalf("search override error: %v", err)
	}
	if cql != "type = page AND space = ENG" {
		t.Errorf("expected --cql to win, got cql=%q", cql)
	}
	// stdout stays the payload; the precedence note goes to stderr.
	if strings.Contains(out, "precedence") {
		t.Errorf("precedence note leaked into stdout: %s", out)
	}
	if !strings.Contains(strings.ToLower(errOut), "precedence") {
		t.Errorf("expected a precedence note on stderr, got: %q", errOut)
	}
}

func Test_cmd_search_via_alias(t *testing.T) {
	var cql string
	srv := searchServer(t, &cql)
	dir := t.TempDir()
	creds := dir + "/credentials"
	s := auth.NewStore(nil)
	_ = s.AddWithAlias(srv.URL, "tok", "prod")
	if err := s.Save(creds); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, _, err := runCmd(t, creds, "", "search", "x", "--instance", "prod", "-o", "json")
	if err != nil {
		t.Fatalf("search via alias error: %v", err)
	}
	if cql == "" {
		t.Errorf("search via alias did not reach the server")
	}
}
