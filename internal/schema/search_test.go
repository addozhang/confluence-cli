package schema

import (
	"encoding/json"
	"testing"
)

// searchFixture mirrors a Confluence /rest/api/search response: results wrap a
// `content` object (id/type/title/space) and carry a result-level `url` plus
// `_links` for absolute URL construction. It also includes a non-content result
// (a space hit) to exercise null-tolerance, and the usual internals to drop.
const searchFixture = `{
  "results": [
    {
      "content": {
        "id": "12345",
        "type": "page",
        "status": "current",
        "title": "Runbook",
        "space": { "key": "ENG", "name": "Engineering", "type": "global" },
        "_expandable": {}
      },
      "title": "Runbook",
      "url": "/spaces/ENG/pages/12345/Runbook",
      "_links": { "self": "https://wiki.example.com/rest/api/content/12345" }
    },
    {
      "content": {
        "id": "777",
        "type": "blogpost",
        "title": "Release",
        "space": { "key": "ENG" }
      },
      "url": "/spaces/ENG/blog/777",
      "_links": {}
    },
    {
      "title": "A Space Result",
      "url": "/spaces/OPS",
      "_links": {}
    }
  ],
  "start": 0,
  "limit": 25,
  "size": 3,
  "_links": { "base": "https://wiki.example.com", "self": "x" }
}`

func Test_MapSearch_projects_results(t *testing.T) {
	res, err := MapSearch("https://wiki.example.com", []byte(searchFixture))
	if err != nil {
		t.Fatalf("MapSearch error: %v", err)
	}
	if res.Start != 0 || res.Limit != 25 || res.Size != 3 {
		t.Errorf("pagination = {start:%d limit:%d size:%d}, want {0 25 3}", res.Start, res.Limit, res.Size)
	}
	if len(res.Results) != 3 {
		t.Fatalf("Results len = %d, want 3", len(res.Results))
	}

	first := res.Results[0]
	if first.ID == nil || *first.ID != "12345" {
		t.Errorf("Results[0].ID = %v, want 12345", first.ID)
	}
	if first.Title != "Runbook" {
		t.Errorf("Results[0].Title = %q, want Runbook", first.Title)
	}
	if first.Type == nil || *first.Type != "page" {
		t.Errorf("Results[0].Type = %v, want page", first.Type)
	}
	if first.SpaceKey == nil || *first.SpaceKey != "ENG" {
		t.Errorf("Results[0].SpaceKey = %v, want ENG", first.SpaceKey)
	}
	if first.URL == nil || *first.URL == "" {
		t.Errorf("Results[0].URL should be populated, got %v", first.URL)
	}

	second := res.Results[1]
	if second.Type == nil || *second.Type != "blogpost" {
		t.Errorf("Results[1].Type = %v, want blogpost", second.Type)
	}
}

func Test_MapSearch_tolerates_non_content_results(t *testing.T) {
	res, err := MapSearch("https://wiki.example.com", []byte(searchFixture))
	if err != nil {
		t.Fatalf("MapSearch error: %v", err)
	}
	// The third result is a space hit with no content: id/type/spaceKey null,
	// but it is NOT dropped, and title/url survive.
	third := res.Results[2]
	if third.ID != nil {
		t.Errorf("space-result ID should be null, got %v", *third.ID)
	}
	if third.Type != nil {
		t.Errorf("space-result Type should be null, got %v", *third.Type)
	}
	if third.Title != "A Space Result" {
		t.Errorf("space-result Title = %q, want 'A Space Result'", third.Title)
	}
}

func Test_MapSearch_drops_internals(t *testing.T) {
	res, _ := MapSearch("https://wiki.example.com", []byte(searchFixture))
	encoded, _ := json.Marshal(res)
	for _, forbidden := range []string{"_links", "_expandable", "status"} {
		if containsKey(encoded, forbidden) {
			t.Errorf("search output leaked %q: %s", forbidden, encoded)
		}
	}
}

func Test_MapSearch_empty_is_empty_slice(t *testing.T) {
	const empty = `{"results":[],"start":0,"limit":25,"size":0}`
	res, err := MapSearch("https://wiki.example.com", []byte(empty))
	if err != nil {
		t.Fatalf("MapSearch error: %v", err)
	}
	if res.Results == nil {
		t.Errorf("Results must be an empty slice, not nil")
	}
	encoded, _ := json.Marshal(res)
	if !containsValue(encoded, `"results":[]`) {
		t.Errorf("empty search must encode as [], got: %s", encoded)
	}
}

func Test_MapSearch_malformed_errors(t *testing.T) {
	if _, err := MapSearch("https://wiki.example.com", []byte(`{bad`)); err == nil {
		t.Errorf("MapSearch on malformed JSON should error")
	}
}
