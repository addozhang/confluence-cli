package schema

import (
	"encoding/json"
	"testing"
)

// pageFixture is a representative Confluence Server/DC content response with
// body.storage, version, space, and ancestors expanded, plus the noisy
// internal fields (_links, _expandable, extensions) the schema must drop.
const pageFixture = `{
  "id": "12345",
  "type": "page",
  "status": "current",
  "title": "Runbook",
  "space": { "id": 99, "key": "ENG", "name": "Engineering", "type": "global", "_expandable": {} },
  "version": { "number": 7, "minorEdit": false, "when": "2026-06-01T10:00:00.000Z" },
  "ancestors": [
    { "id": "100", "title": "Home", "_links": { "self": "x" } },
    { "id": "200", "title": "Ops", "_links": { "self": "y" } }
  ],
  "body": {
    "storage": { "value": "<p>hello</p>", "representation": "storage" },
    "_expandable": { "view": "" }
  },
  "_links": { "webui": "/display/ENG/Runbook", "base": "https://wiki.example.com", "self": "https://wiki.example.com/rest/api/content/12345" },
  "_expandable": { "children": "/rest/api/content/12345/child" },
  "extensions": { "position": 1 }
}`

func Test_MapPage_extracts_documented_fields(t *testing.T) {
	page, err := MapPage([]byte(pageFixture))
	if err != nil {
		t.Fatalf("MapPage error: %v", err)
	}

	if page.ID != "12345" {
		t.Errorf("ID = %q, want 12345", page.ID)
	}
	if page.Title != "Runbook" {
		t.Errorf("Title = %q, want Runbook", page.Title)
	}
	if page.SpaceKey != "ENG" {
		t.Errorf("SpaceKey = %q, want ENG", page.SpaceKey)
	}
	if page.Version != 7 {
		t.Errorf("Version = %d, want 7", page.Version)
	}
	if page.Body == nil || *page.Body != "<p>hello</p>" {
		t.Errorf("Body = %v, want <p>hello</p>", page.Body)
	}
	if len(page.Ancestors) != 2 {
		t.Fatalf("Ancestors len = %d, want 2", len(page.Ancestors))
	}
	if page.Ancestors[0].ID != "100" || page.Ancestors[0].Title != "Home" {
		t.Errorf("Ancestors[0] = %+v, want {100 Home}", page.Ancestors[0])
	}
	// parentId is the immediate (last) ancestor.
	if page.ParentID == nil || *page.ParentID != "200" {
		t.Errorf("ParentID = %v, want 200", page.ParentID)
	}
	if page.URL == nil || *page.URL == "" {
		t.Errorf("URL should be populated from _links, got %v", page.URL)
	}
}

func Test_MapPage_drops_undocumented_fields(t *testing.T) {
	page, err := MapPage([]byte(pageFixture))
	if err != nil {
		t.Fatalf("MapPage error: %v", err)
	}

	// Re-encode the schema value and assert none of the Confluence internals
	// survive the mapping.
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{"_links", "_expandable", "extensions", "minorEdit", "representation"} {
		if containsKey(encoded, forbidden) {
			t.Errorf("schema output leaked undocumented field %q: %s", forbidden, encoded)
		}
	}
}

func Test_MapPage_top_level_page_has_null_parent(t *testing.T) {
	const noParent = `{
      "id": "1",
      "title": "Top",
      "space": { "key": "ENG" },
      "version": { "number": 1 },
      "ancestors": [],
      "body": { "storage": { "value": "<p/>", "representation": "storage" } }
    }`

	page, err := MapPage([]byte(noParent))
	if err != nil {
		t.Fatalf("MapPage error: %v", err)
	}
	if page.ParentID != nil {
		t.Errorf("ParentID = %v, want nil for a top-level page", *page.ParentID)
	}

	// In JSON the field must be present and explicitly null (not omitted).
	encoded, _ := json.Marshal(page)
	if !containsKey(encoded, "parentId") {
		t.Errorf("parentId must be present (as null), got: %s", encoded)
	}
}

func Test_MapChildren(t *testing.T) {
	const fixture = `{
      "results": [
        { "id": "11", "title": "Child A", "version": { "number": 3 }, "_links": {} },
        { "id": "12", "title": "Child B", "version": { "number": 1 }, "_links": {} }
      ],
      "size": 2,
      "_links": { "self": "x" }
    }`

	children, err := MapChildren([]byte(fixture))
	if err != nil {
		t.Fatalf("MapChildren error: %v", err)
	}
	if len(children.Children) != 2 {
		t.Fatalf("Children len = %d, want 2", len(children.Children))
	}
	if children.Children[0].ID != "11" || children.Children[0].Version != 3 {
		t.Errorf("Children[0] = %+v, want id=11 version=3", children.Children[0])
	}
}

func Test_MapChildren_empty_is_empty_slice_not_nil(t *testing.T) {
	const fixture = `{ "results": [], "size": 0 }`
	children, err := MapChildren([]byte(fixture))
	if err != nil {
		t.Fatalf("MapChildren error: %v", err)
	}
	if children.Children == nil {
		t.Errorf("Children must be an empty slice, not nil")
	}
	if len(children.Children) != 0 {
		t.Errorf("Children len = %d, want 0", len(children.Children))
	}

	// Must serialize as [] not null.
	encoded, _ := json.Marshal(children)
	if !containsValue(encoded, `"children":[]`) {
		t.Errorf("empty children must encode as [], got: %s", encoded)
	}
}

func Test_MapSpaceList(t *testing.T) {
	const fixture = `{
      "results": [
        { "key": "ENG", "name": "Engineering", "type": "global", "_links": {} },
        { "key": "~user", "name": "Personal", "type": "personal", "_links": {} }
      ],
      "start": 50,
      "limit": 25,
      "size": 2
    }`

	list, err := MapSpaceList([]byte(fixture))
	if err != nil {
		t.Fatalf("MapSpaceList error: %v", err)
	}
	if list.Start != 50 || list.Limit != 25 || list.Size != 2 {
		t.Errorf("pagination = {start:%d limit:%d size:%d}, want {50 25 2}", list.Start, list.Limit, list.Size)
	}
	if len(list.Spaces) != 2 {
		t.Fatalf("Spaces len = %d, want 2", len(list.Spaces))
	}
	if list.Spaces[0].Key != "ENG" || list.Spaces[0].Type != "global" {
		t.Errorf("Spaces[0] = %+v, want key=ENG type=global", list.Spaces[0])
	}
}

func Test_MapSpace_with_description(t *testing.T) {
	const fixture = `{
      "key": "ENG",
      "name": "Engineering",
      "type": "global",
      "description": { "plain": { "value": "All things eng", "representation": "plain" } },
      "_links": {}
    }`

	sp, err := MapSpace([]byte(fixture))
	if err != nil {
		t.Fatalf("MapSpace error: %v", err)
	}
	if sp.Key != "ENG" || sp.Name != "Engineering" || sp.Type != "global" {
		t.Errorf("space = %+v", sp)
	}
	if sp.Description == nil || *sp.Description != "All things eng" {
		t.Errorf("Description = %v, want 'All things eng'", sp.Description)
	}
}

func Test_MapSpace_without_description_is_null(t *testing.T) {
	const fixture = `{ "key": "OPS", "name": "Operations", "type": "global", "_links": {} }`
	sp, err := MapSpace([]byte(fixture))
	if err != nil {
		t.Fatalf("MapSpace error: %v", err)
	}
	if sp.Description != nil {
		t.Errorf("Description = %v, want nil", *sp.Description)
	}
	encoded, _ := json.Marshal(sp)
	if !containsKey(encoded, "description") {
		t.Errorf("description must be present (as null), got: %s", encoded)
	}
}

func Test_MapPage_malformed_errors(t *testing.T) {
	if _, err := MapPage([]byte(`{not json`)); err == nil {
		t.Errorf("MapPage on malformed JSON should error")
	}
}
