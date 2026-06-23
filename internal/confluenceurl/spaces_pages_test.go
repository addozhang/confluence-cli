package confluenceurl

import "testing"

// Test_Parse_spaces_pages covers the modern /spaces/KEY/pages/ID[/Title] page
// URL shape (url-resolution spec, ADDED requirement).
func Test_Parse_spaces_pages(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantBase    string
		wantContext string
		wantSpace   string
		wantPageID  string
		wantTitle   string
	}{
		{
			name:       "with title",
			in:         "https://test-confluence.example.com/spaces/test/pages/5789518257/Test+Page",
			wantBase:   "https://test-confluence.example.com",
			wantSpace:  "test",
			wantPageID: "5789518257",
			wantTitle:  "Test Page",
		},
		{
			name:       "without title",
			in:         "https://wiki.example.com/spaces/ENG/pages/12345",
			wantBase:   "https://wiki.example.com",
			wantSpace:  "ENG",
			wantPageID: "12345",
		},
		{
			name:        "with context path",
			in:          "https://wiki.example.com/confluence/spaces/ENG/pages/12345/Runbook",
			wantBase:    "https://wiki.example.com",
			wantContext: "/confluence",
			wantSpace:   "ENG",
			wantPageID:  "12345",
			wantTitle:   "Runbook",
		},
		{
			name:       "trailing slash ignored",
			in:         "https://wiki.example.com/spaces/ENG/pages/12345/Runbook/",
			wantBase:   "https://wiki.example.com",
			wantSpace:  "ENG",
			wantPageID: "12345",
			wantTitle:  "Runbook",
		},
		{
			name:       "fragment ignored",
			in:         "https://wiki.example.com/spaces/ENG/pages/12345#section",
			wantBase:   "https://wiki.example.com",
			wantSpace:  "ENG",
			wantPageID: "12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tt.in, err)
			}
			if ref.BaseURL != tt.wantBase {
				t.Errorf("BaseURL = %q, want %q", ref.BaseURL, tt.wantBase)
			}
			if ref.ContextPath != tt.wantContext {
				t.Errorf("ContextPath = %q, want %q", ref.ContextPath, tt.wantContext)
			}
			if ref.SpaceKey != tt.wantSpace {
				t.Errorf("SpaceKey = %q, want %q", ref.SpaceKey, tt.wantSpace)
			}
			if ref.PageID != tt.wantPageID {
				t.Errorf("PageID = %q, want %q", ref.PageID, tt.wantPageID)
			}
			if ref.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", ref.Title, tt.wantTitle)
			}
		})
	}
}

func Test_Parse_bare_spaces_still_space_ref(t *testing.T) {
	// /spaces/KEY without a /pages continuation must remain a space-only Ref.
	ref, err := Parse("https://wiki.example.com/spaces/ENG")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if ref.SpaceKey != "ENG" {
		t.Errorf("SpaceKey = %q, want ENG", ref.SpaceKey)
	}
	if ref.PageID != "" {
		t.Errorf("PageID = %q, want empty for a bare spaces URL", ref.PageID)
	}
}

func Test_Parse_spaces_pages_non_numeric_id_rejected(t *testing.T) {
	if _, err := Parse("https://wiki.example.com/spaces/ENG/pages/not-a-number/Title"); err == nil {
		t.Errorf("a non-numeric page id in the spaces/pages shape should error")
	}
}
