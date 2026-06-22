package schema

import (
	"encoding/json"
	"testing"
)

func Test_MapPageSummary(t *testing.T) {
	const fixture = `{
      "id": "777",
      "title": "Created",
      "space": { "key": "ENG" },
      "version": { "number": 1 },
      "_links": { "base": "https://wiki.example.com", "webui": "/display/ENG/Created" }
    }`

	s, err := MapPageSummary([]byte(fixture))
	if err != nil {
		t.Fatalf("MapPageSummary error: %v", err)
	}
	if s.ID != "777" || s.Title != "Created" || s.SpaceKey != "ENG" || s.Version != 1 {
		t.Errorf("summary = %+v", s)
	}
	if s.URL == nil || *s.URL != "https://wiki.example.com/display/ENG/Created" {
		t.Errorf("URL = %v, want absolute webui URL", s.URL)
	}

	// create/update output must NOT carry body or ancestors keys.
	encoded, _ := json.Marshal(s)
	if containsKey(encoded, "body") || containsKey(encoded, "ancestors") {
		t.Errorf("PageSummary must not include body/ancestors: %s", encoded)
	}
}

func Test_MapPageSummary_url_falls_back_to_self(t *testing.T) {
	// When base+webui are absent, the absolute self link is used.
	const fixture = `{
      "id": "1", "title": "T", "space": { "key": "E" }, "version": { "number": 1 },
      "_links": { "self": "https://wiki.example.com/rest/api/content/1" }
    }`
	s, err := MapPageSummary([]byte(fixture))
	if err != nil {
		t.Fatalf("MapPageSummary error: %v", err)
	}
	if s.URL == nil || *s.URL != "https://wiki.example.com/rest/api/content/1" {
		t.Errorf("URL = %v, want the self link", s.URL)
	}
}

func Test_MapPageSummary_no_links_yields_null_url(t *testing.T) {
	const fixture = `{ "id": "1", "title": "T", "space": { "key": "E" }, "version": { "number": 1 } }`
	s, err := MapPageSummary([]byte(fixture))
	if err != nil {
		t.Fatalf("MapPageSummary error: %v", err)
	}
	if s.URL != nil {
		t.Errorf("URL = %v, want nil when no links are present", *s.URL)
	}
}

func Test_MapWhoAmI(t *testing.T) {
	const fixture = `{ "type": "known", "username": "jdoe", "displayName": "Jane Doe", "_links": {} }`
	w, err := MapWhoAmI("https://wiki.example.com", []byte(fixture))
	if err != nil {
		t.Fatalf("MapWhoAmI error: %v", err)
	}
	if w.Host != "https://wiki.example.com" || w.Username != "jdoe" {
		t.Errorf("whoami = %+v", w)
	}
	if w.DisplayName == nil || *w.DisplayName != "Jane Doe" {
		t.Errorf("DisplayName = %v, want 'Jane Doe'", w.DisplayName)
	}

	// The token must never appear in whoami output.
	encoded, _ := json.Marshal(w)
	if containsKey(encoded, "token") {
		t.Errorf("whoami leaked a token field: %s", encoded)
	}
}

func Test_MapWhoAmI_no_display_name_is_null(t *testing.T) {
	const fixture = `{ "username": "svc" }`
	w, err := MapWhoAmI("https://wiki.example.com", []byte(fixture))
	if err != nil {
		t.Fatalf("MapWhoAmI error: %v", err)
	}
	if w.DisplayName != nil {
		t.Errorf("DisplayName = %v, want nil", *w.DisplayName)
	}
}

func Test_NewAuthList_empty_is_non_nil(t *testing.T) {
	list := NewAuthList(nil)
	if list.Instances == nil {
		t.Errorf("Instances must be non-nil")
	}
	encoded, _ := json.Marshal(list)
	if !containsValue(encoded, `"instances":[]`) {
		t.Errorf("empty auth list must encode instances as [], got: %s", encoded)
	}
}

func Test_VersionInfo_round_trip(t *testing.T) {
	commit := "abc1234"
	v := VersionInfo{Version: "v0.1.0", Commit: &commit, Date: nil}
	encoded, _ := json.Marshal(v)
	if !containsKey(encoded, "version") || !containsKey(encoded, "commit") || !containsKey(encoded, "date") {
		t.Errorf("VersionInfo must carry version/commit/date: %s", encoded)
	}
	if !containsValue(encoded, `"date":null`) {
		t.Errorf("nil date must encode as null: %s", encoded)
	}
}
