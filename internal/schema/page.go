package schema

import (
	"encoding/json"
	"fmt"
)

// Ancestor is a page in the ancestor chain, carrying only id and title.
type Ancestor struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Page is the output of `cfl page get` and the page object embedded in
// create/update responses. body and url are pointers so they can be null;
// parentId is the immediate ancestor's id, or null for a top-level page.
type Page struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	SpaceKey  string     `json:"spaceKey"`
	Version   int        `json:"version"`
	ParentID  *string    `json:"parentId"`
	Body      *string    `json:"body"`
	Ancestors []Ancestor `json:"ancestors"`
	URL       *string    `json:"url"`
}

// PageSummary is the output of `cfl page create` and `cfl page update`: the
// created/updated page without body or ancestors.
type PageSummary struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	SpaceKey string  `json:"spaceKey"`
	Version  int     `json:"version"`
	URL      *string `json:"url"`
}

// ChildPage is one entry in a children listing.
type ChildPage struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Version int    `json:"version"`
}

// Children is the output of `cfl page children`.
type Children struct {
	Children []ChildPage `json:"children"`
}

// DeleteResult is the output of `cfl page delete`.
type DeleteResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// confluenceContent is the wire shape of a Confluence content object. Only the
// fields the schema exposes are decoded; everything else (_links, _expandable,
// extensions, ...) is ignored, which is how undocumented fields are dropped.
type confluenceContent struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Space struct {
		Key string `json:"key"`
	} `json:"space"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
	Ancestors []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"ancestors"`
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
	Links struct {
		Base  string `json:"base"`
		WebUI string `json:"webui"`
		Self  string `json:"self"`
	} `json:"_links"`
}

// MapPage maps a Confluence content response into a Page. It is a pure function
// of the input bytes (no HTTP, no globals).
func MapPage(raw []byte) (Page, error) {
	var c confluenceContent
	if err := json.Unmarshal(raw, &c); err != nil {
		return Page{}, fmt.Errorf("decode page: %w", err)
	}

	page := Page{
		ID:        c.ID,
		Title:     c.Title,
		SpaceKey:  c.Space.Key,
		Version:   c.Version.Number,
		Ancestors: make([]Ancestor, 0, len(c.Ancestors)),
	}

	for _, a := range c.Ancestors {
		page.Ancestors = append(page.Ancestors, Ancestor{ID: a.ID, Title: a.Title})
	}
	// The immediate parent is the last ancestor in the chain.
	if n := len(c.Ancestors); n > 0 {
		parent := c.Ancestors[n-1].ID
		page.ParentID = &parent
	}

	if c.Body.Storage.Value != "" {
		body := c.Body.Storage.Value
		page.Body = &body
	}

	if u := webURL(c.Links.Base, c.Links.WebUI, c.Links.Self); u != "" {
		page.URL = &u
	}

	return page, nil
}

// MapPageSummary maps a Confluence content response (typically the result of a
// create or update) into a PageSummary.
func MapPageSummary(raw []byte) (PageSummary, error) {
	var c confluenceContent
	if err := json.Unmarshal(raw, &c); err != nil {
		return PageSummary{}, fmt.Errorf("decode page: %w", err)
	}
	s := PageSummary{
		ID:       c.ID,
		Title:    c.Title,
		SpaceKey: c.Space.Key,
		Version:  c.Version.Number,
	}
	if u := webURL(c.Links.Base, c.Links.WebUI, c.Links.Self); u != "" {
		s.URL = &u
	}
	return s, nil
}

// MapChildren maps a Confluence child-page listing into Children. An empty
// result yields a non-nil empty slice so it serializes as [] not null.
func MapChildren(raw []byte) (Children, error) {
	var wire struct {
		Results []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Version struct {
				Number int `json:"number"`
			} `json:"version"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Children{}, fmt.Errorf("decode children: %w", err)
	}

	out := Children{Children: make([]ChildPage, 0, len(wire.Results))}
	for _, r := range wire.Results {
		out.Children = append(out.Children, ChildPage{ID: r.ID, Title: r.Title, Version: r.Version.Number})
	}
	return out, nil
}

// webURL chooses the best absolute page URL from the _links fields. Confluence
// returns webui as a path relative to base, and self as an absolute REST URL.
func webURL(base, webui, self string) string {
	switch {
	case base != "" && webui != "":
		return base + webui
	case self != "":
		return self
	default:
		return ""
	}
}

// FirstContentID returns the id of the first entry in a Confluence content
// search/lookup response (results[].id), used to resolve a display URL's
// space-key+title to a concrete page id. found is false when results is empty.
func FirstContentID(raw []byte) (id string, found bool, err error) {
	var wire struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return "", false, fmt.Errorf("decode content lookup: %w", err)
	}
	if len(wire.Results) == 0 {
		return "", false, nil
	}
	return wire.Results[0].ID, true, nil
}
