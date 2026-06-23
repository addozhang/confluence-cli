package schema

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SearchResult is one hit from `cfl search`. id/type/spaceKey are pointers so a
// non-content hit (e.g. a space) renders them as null rather than being dropped.
type SearchResult struct {
	ID       *string `json:"id"`
	Title    string  `json:"title"`
	Type     *string `json:"type"`
	SpaceKey *string `json:"spaceKey"`
	URL      *string `json:"url"`
}

// SearchResults is the output of `cfl search`: the page of hits plus the
// pagination window that produced it.
type SearchResults struct {
	Results []SearchResult `json:"results"`
	Start   int            `json:"start"`
	Limit   int            `json:"limit"`
	Size    int            `json:"size"`
}

// MapSearch maps a Confluence /rest/api/search response into SearchResults.
// base is the instance base URL, used to absolutize the result-level webui URL.
// It is a pure function of the input bytes.
func MapSearch(base string, raw []byte) (SearchResults, error) {
	var wire struct {
		Results []struct {
			Content *struct {
				ID    string `json:"id"`
				Type  string `json:"type"`
				Title string `json:"title"`
				Space *struct {
					Key string `json:"key"`
				} `json:"space"`
			} `json:"content"`
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"results"`
		Start int `json:"start"`
		Limit int `json:"limit"`
		Size  int `json:"size"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return SearchResults{}, fmt.Errorf("decode search: %w", err)
	}

	out := SearchResults{
		Results: make([]SearchResult, 0, len(wire.Results)),
		Start:   wire.Start,
		Limit:   wire.Limit,
		Size:    wire.Size,
	}
	for _, r := range wire.Results {
		sr := SearchResult{}
		if r.Content != nil {
			if r.Content.ID != "" {
				id := r.Content.ID
				sr.ID = &id
			}
			if r.Content.Type != "" {
				typ := r.Content.Type
				sr.Type = &typ
			}
			if r.Content.Space != nil && r.Content.Space.Key != "" {
				key := r.Content.Space.Key
				sr.SpaceKey = &key
			}
		}
		// Prefer the clean content title; the result-level title is wrapped in
		// Confluence search highlight markers (@@@hl@@@…@@@endhl@@@). Strip those
		// markers from whichever title we use so they never reach the user.
		if r.Content != nil && r.Content.Title != "" {
			sr.Title = stripHighlight(r.Content.Title)
		} else {
			sr.Title = stripHighlight(r.Title)
		}
		if r.URL != "" {
			u := base + r.URL
			sr.URL = &u
		}
		out.Results = append(out.Results, sr)
	}
	return out, nil
}

// stripHighlight removes Confluence search highlight markers from a string. The
// search endpoint wraps matched terms in @@@hl@@@…@@@endhl@@@ for front-end
// emphasis; those markers are noise in a CLI title.
func stripHighlight(s string) string {
	s = strings.ReplaceAll(s, "@@@hl@@@", "")
	s = strings.ReplaceAll(s, "@@@endhl@@@", "")
	return s
}
