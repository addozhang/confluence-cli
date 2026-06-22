package schema

import (
	"encoding/json"
	"fmt"
)

// Space is one entry in a space listing.
type Space struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// SpaceList is the output of `cfl space list`, carrying the page of spaces plus
// the pagination window that produced it.
type SpaceList struct {
	Spaces []Space `json:"spaces"`
	Start  int     `json:"start"`
	Limit  int     `json:"limit"`
	Size   int     `json:"size"`
}

// SpaceDetail is the output of `cfl space get`. description is null when the
// space has none.
type SpaceDetail struct {
	Key         string  `json:"key"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description *string `json:"description"`
}

// MapSpaceList maps a Confluence space listing into a SpaceList. An empty
// result yields a non-nil empty slice so it serializes as [] not null.
func MapSpaceList(raw []byte) (SpaceList, error) {
	var wire struct {
		Results []struct {
			Key  string `json:"key"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"results"`
		Start int `json:"start"`
		Limit int `json:"limit"`
		Size  int `json:"size"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return SpaceList{}, fmt.Errorf("decode space list: %w", err)
	}

	out := SpaceList{
		Spaces: make([]Space, 0, len(wire.Results)),
		Start:  wire.Start,
		Limit:  wire.Limit,
		Size:   wire.Size,
	}
	for _, r := range wire.Results {
		out.Spaces = append(out.Spaces, Space{Key: r.Key, Name: r.Name, Type: r.Type})
	}
	return out, nil
}

// MapSpace maps a single Confluence space response into a SpaceDetail.
func MapSpace(raw []byte) (SpaceDetail, error) {
	var wire struct {
		Key         string `json:"key"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description struct {
			Plain struct {
				Value string `json:"value"`
			} `json:"plain"`
		} `json:"description"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return SpaceDetail{}, fmt.Errorf("decode space: %w", err)
	}

	detail := SpaceDetail{Key: wire.Key, Name: wire.Name, Type: wire.Type}
	if wire.Description.Plain.Value != "" {
		desc := wire.Description.Plain.Value
		detail.Description = &desc
	}
	return detail, nil
}
