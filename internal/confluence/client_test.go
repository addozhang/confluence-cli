package confluence

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/addozhang/cfl/internal/auth"
	cflerrors "github.com/addozhang/cfl/internal/errors"
)

// recordingServer captures the last request it received so tests can assert the
// exact endpoint, query, and body the client produced (the D9 wire contract).
type recordingServer struct {
	srv      *httptest.Server
	method   string
	path     string
	rawQuery string
	query    url.Values
	body     []byte
}

func newRecordingServer(t *testing.T, status int, respBody string) *recordingServer {
	t.Helper()
	rs := &recordingServer{}
	rs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.method = r.Method
		rs.path = r.URL.Path
		rs.rawQuery = r.URL.RawQuery
		rs.query = r.URL.Query()
		rs.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(rs.srv.Close)
	return rs
}

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	store := auth.NewStore(map[string]string{baseURL: "tok"})
	rt, err := NewTransport(TransportConfig{Timeout: 5 * time.Second}, store)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}
	return NewClient(&http.Client{Transport: rt}, baseURL, "")
}

func ctx() context.Context { return context.Background() }

func Test_GetPage_endpoint_and_expand(t *testing.T) {
	rs := newRecordingServer(t, http.StatusOK, `{"id":"12345","title":"Runbook"}`)
	c := newTestClient(t, rs.srv.URL)

	raw, err := c.GetPage(ctx(), "12345")
	if err != nil {
		t.Fatalf("GetPage error: %v", err)
	}
	if rs.method != http.MethodGet {
		t.Errorf("method = %s, want GET", rs.method)
	}
	if rs.path != "/rest/api/content/12345" {
		t.Errorf("path = %s, want /rest/api/content/12345", rs.path)
	}
	if exp := rs.query.Get("expand"); exp != "body.storage,version,space,ancestors" {
		t.Errorf("expand = %q, want body.storage,version,space,ancestors", exp)
	}
	if !strings.Contains(string(raw), `"id":"12345"`) {
		t.Errorf("raw body should be passed through verbatim, got: %s", raw)
	}
}

func Test_LookupPageByTitle_endpoint_and_query(t *testing.T) {
	rs := newRecordingServer(t, http.StatusOK, `{"results":[{"id":"99"}]}`)
	c := newTestClient(t, rs.srv.URL)

	_, err := c.LookupPageByTitle(ctx(), "ENG", "Release Notes")
	if err != nil {
		t.Fatalf("LookupPageByTitle error: %v", err)
	}
	if rs.path != "/rest/api/content" {
		t.Errorf("path = %s, want /rest/api/content", rs.path)
	}
	if got := rs.query.Get("spaceKey"); got != "ENG" {
		t.Errorf("spaceKey = %q, want ENG", got)
	}
	if got := rs.query.Get("title"); got != "Release Notes" {
		t.Errorf("title = %q, want 'Release Notes'", got)
	}
	if got := rs.query.Get("expand"); got != "body.storage,version" {
		t.Errorf("expand = %q, want body.storage,version", got)
	}
}

func Test_CreatePage_top_level_body(t *testing.T) {
	rs := newRecordingServer(t, http.StatusOK, `{"id":"1"}`)
	c := newTestClient(t, rs.srv.URL)

	_, err := c.CreatePage(ctx(), CreatePageInput{
		SpaceKey: "ENG",
		Title:    "Release Notes",
		Body:     "<p>hi</p>",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}
	if rs.method != http.MethodPost || rs.path != "/rest/api/content" {
		t.Errorf("got %s %s, want POST /rest/api/content", rs.method, rs.path)
	}

	var sent map[string]any
	if err := json.Unmarshal(rs.body, &sent); err != nil {
		t.Fatalf("request body is not JSON: %v (%s)", err, rs.body)
	}
	if sent["type"] != "page" {
		t.Errorf("type = %v, want page", sent["type"])
	}
	if sent["title"] != "Release Notes" {
		t.Errorf("title = %v, want 'Release Notes'", sent["title"])
	}
	space, _ := sent["space"].(map[string]any)
	if space["key"] != "ENG" {
		t.Errorf("space.key = %v, want ENG", space["key"])
	}
	body, _ := sent["body"].(map[string]any)
	storage, _ := body["storage"].(map[string]any)
	if storage["value"] != "<p>hi</p>" || storage["representation"] != "storage" {
		t.Errorf("body.storage = %v, want value=<p>hi</p> representation=storage", storage)
	}
	if _, hasAncestors := sent["ancestors"]; hasAncestors {
		t.Errorf("top-level create must NOT include ancestors, got: %v", sent["ancestors"])
	}
}

func Test_CreatePage_with_parent_includes_ancestors(t *testing.T) {
	rs := newRecordingServer(t, http.StatusOK, `{"id":"1"}`)
	c := newTestClient(t, rs.srv.URL)

	_, err := c.CreatePage(ctx(), CreatePageInput{
		SpaceKey: "ENG",
		Title:    "Sub",
		Body:     "<p/>",
		ParentID: "12345",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	var sent map[string]any
	_ = json.Unmarshal(rs.body, &sent)
	ancestors, ok := sent["ancestors"].([]any)
	if !ok || len(ancestors) != 1 {
		t.Fatalf("ancestors = %v, want one entry", sent["ancestors"])
	}
	first, _ := ancestors[0].(map[string]any)
	if first["id"] != "12345" {
		t.Errorf("ancestors[0].id = %v, want 12345", first["id"])
	}
}

func Test_UpdatePage_increments_version(t *testing.T) {
	rs := newRecordingServer(t, http.StatusOK, `{"id":"12345","version":{"number":8}}`)
	c := newTestClient(t, rs.srv.URL)

	_, err := c.UpdatePage(ctx(), UpdatePageInput{
		ID:         "12345",
		Title:      "Runbook",
		Body:       "<p>new</p>",
		NewVersion: 8,
	})
	if err != nil {
		t.Fatalf("UpdatePage error: %v", err)
	}
	if rs.method != http.MethodPut || rs.path != "/rest/api/content/12345" {
		t.Errorf("got %s %s, want PUT /rest/api/content/12345", rs.method, rs.path)
	}

	var sent map[string]any
	_ = json.Unmarshal(rs.body, &sent)
	version, _ := sent["version"].(map[string]any)
	if version["number"].(float64) != 8 {
		t.Errorf("version.number = %v, want 8", version["number"])
	}
	if sent["title"] != "Runbook" {
		t.Errorf("title = %v, want Runbook", sent["title"])
	}
}

func Test_ReadVersion_endpoint(t *testing.T) {
	rs := newRecordingServer(t, http.StatusOK, `{"id":"12345","title":"Runbook","version":{"number":7}}`)
	c := newTestClient(t, rs.srv.URL)

	_, err := c.ReadVersion(ctx(), "12345")
	if err != nil {
		t.Fatalf("ReadVersion error: %v", err)
	}
	if rs.path != "/rest/api/content/12345" {
		t.Errorf("path = %s, want /rest/api/content/12345", rs.path)
	}
	if got := rs.query.Get("expand"); got != "version" {
		t.Errorf("expand = %q, want version", got)
	}
}

func Test_DeletePage_endpoint(t *testing.T) {
	rs := newRecordingServer(t, http.StatusNoContent, ``)
	c := newTestClient(t, rs.srv.URL)

	err := c.DeletePage(ctx(), "12345")
	if err != nil {
		t.Fatalf("DeletePage error: %v", err)
	}
	if rs.method != http.MethodDelete || rs.path != "/rest/api/content/12345" {
		t.Errorf("got %s %s, want DELETE /rest/api/content/12345", rs.method, rs.path)
	}
}

func Test_GetChildren_endpoint(t *testing.T) {
	rs := newRecordingServer(t, http.StatusOK, `{"results":[]}`)
	c := newTestClient(t, rs.srv.URL)

	_, err := c.GetChildren(ctx(), "12345")
	if err != nil {
		t.Fatalf("GetChildren error: %v", err)
	}
	if rs.path != "/rest/api/content/12345/child/page" {
		t.Errorf("path = %s, want /rest/api/content/12345/child/page", rs.path)
	}
	if got := rs.query.Get("expand"); got != "version" {
		t.Errorf("expand = %q, want version", got)
	}
}

func Test_ListSpaces_pagination(t *testing.T) {
	rs := newRecordingServer(t, http.StatusOK, `{"results":[],"start":50,"limit":25,"size":0}`)
	c := newTestClient(t, rs.srv.URL)

	_, err := c.ListSpaces(ctx(), 25, 50)
	if err != nil {
		t.Fatalf("ListSpaces error: %v", err)
	}
	if rs.path != "/rest/api/space" {
		t.Errorf("path = %s, want /rest/api/space", rs.path)
	}
	if rs.query.Get("limit") != "25" || rs.query.Get("start") != "50" {
		t.Errorf("query = %s, want limit=25&start=50", rs.rawQuery)
	}
}

func Test_ListSpaces_omits_defaults(t *testing.T) {
	rs := newRecordingServer(t, http.StatusOK, `{"results":[]}`)
	c := newTestClient(t, rs.srv.URL)

	// Zero limit/start means "rely on server defaults": no query params sent.
	_, err := c.ListSpaces(ctx(), 0, 0)
	if err != nil {
		t.Fatalf("ListSpaces error: %v", err)
	}
	if rs.query.Has("limit") || rs.query.Has("start") {
		t.Errorf("with zero values, no pagination params should be sent, got: %s", rs.rawQuery)
	}
}

func Test_GetSpace_endpoint_and_expand(t *testing.T) {
	rs := newRecordingServer(t, http.StatusOK, `{"key":"ENG"}`)
	c := newTestClient(t, rs.srv.URL)

	_, err := c.GetSpace(ctx(), "ENG")
	if err != nil {
		t.Fatalf("GetSpace error: %v", err)
	}
	if rs.path != "/rest/api/space/ENG" {
		t.Errorf("path = %s, want /rest/api/space/ENG", rs.path)
	}
	if got := rs.query.Get("expand"); got != "description.plain,homepage" {
		t.Errorf("expand = %q, want description.plain,homepage", got)
	}
}

func Test_WhoAmI_endpoint(t *testing.T) {
	rs := newRecordingServer(t, http.StatusOK, `{"username":"jdoe"}`)
	c := newTestClient(t, rs.srv.URL)

	_, err := c.WhoAmI(ctx())
	if err != nil {
		t.Fatalf("WhoAmI error: %v", err)
	}
	if rs.path != "/rest/api/user/current" {
		t.Errorf("path = %s, want /rest/api/user/current", rs.path)
	}
}

func Test_non_2xx_returns_HTTPStatusError(t *testing.T) {
	rs := newRecordingServer(t, http.StatusNotFound, `{"message":"No content found"}`)
	c := newTestClient(t, rs.srv.URL)

	_, err := c.GetPage(ctx(), "99999")
	if err == nil {
		t.Fatalf("expected an error for a 404 response")
	}
	var statusErr *cflerrors.HTTPStatusError
	if !asHTTPStatusError(err, &statusErr) {
		t.Fatalf("error = %T (%v), want *HTTPStatusError", err, err)
	}
	if statusErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want 404", statusErr.StatusCode)
	}
	if !strings.Contains(string(statusErr.Body), "No content found") {
		t.Errorf("HTTPStatusError.Body should carry the response, got: %s", statusErr.Body)
	}
}

func Test_context_path_is_prefixed(t *testing.T) {
	rs := newRecordingServer(t, http.StatusOK, `{"id":"1"}`)
	store := auth.NewStore(map[string]string{rs.srv.URL + "/confluence": "tok"})
	rt, _ := NewTransport(TransportConfig{Timeout: 5 * time.Second}, store)
	c := NewClient(&http.Client{Transport: rt}, rs.srv.URL, "/confluence")

	_, err := c.GetPage(ctx(), "1")
	if err != nil {
		t.Fatalf("GetPage error: %v", err)
	}
	if rs.path != "/confluence/rest/api/content/1" {
		t.Errorf("path = %s, want /confluence/rest/api/content/1", rs.path)
	}
}
