package confluenceurl

import "testing"

// Test_Parse_shapes covers every accepted URL shape from the url-resolution
// spec, asserting the resulting Ref fields.
func Test_Parse_shapes(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantBase    string
		wantContext string
		wantPageID  string
		wantSpace   string
		wantTitle   string
		wantBareID  bool
	}{
		{
			name:       "viewpage pageId URL",
			in:         "https://wiki.example.com/pages/viewpage.action?pageId=12345",
			wantBase:   "https://wiki.example.com",
			wantPageID: "12345",
		},
		{
			name:       "REST content URL",
			in:         "https://wiki.example.com/rest/api/content/12345",
			wantBase:   "https://wiki.example.com",
			wantPageID: "12345",
		},
		{
			name:      "display URL with space and title",
			in:        "https://wiki.example.com/display/DEV/Release+Notes",
			wantBase:  "https://wiki.example.com",
			wantSpace: "DEV",
			wantTitle: "Release Notes",
		},
		{
			name:      "display URL with percent-encoded title",
			in:        "https://wiki.example.com/display/DEV/Release%20Notes",
			wantBase:  "https://wiki.example.com",
			wantSpace: "DEV",
			wantTitle: "Release Notes",
		},
		{
			name:      "spaces home URL",
			in:        "https://wiki.example.com/spaces/DEV",
			wantBase:  "https://wiki.example.com",
			wantSpace: "DEV",
		},
		{
			name:      "display space-only URL",
			in:        "https://wiki.example.com/display/DEV",
			wantBase:  "https://wiki.example.com",
			wantSpace: "DEV",
		},
		{
			name:       "trailing slash and fragment ignored on pageId",
			in:         "https://wiki.example.com/pages/viewpage.action?pageId=12345#comments",
			wantBase:   "https://wiki.example.com",
			wantPageID: "12345",
		},
		{
			name:      "trailing slash ignored on spaces",
			in:        "https://wiki.example.com/spaces/DEV/",
			wantBase:  "https://wiki.example.com",
			wantSpace: "DEV",
		},
		{
			name:        "context path preserved on display URL",
			in:          "https://wiki.example.com/confluence/display/DEV/Release+Notes",
			wantBase:    "https://wiki.example.com",
			wantContext: "/confluence",
			wantSpace:   "DEV",
			wantTitle:   "Release Notes",
		},
		{
			name:        "context path preserved on pageId URL",
			in:          "https://wiki.example.com/confluence/pages/viewpage.action?pageId=12345",
			wantBase:    "https://wiki.example.com",
			wantContext: "/confluence",
			wantPageID:  "12345",
		},
		{
			name:       "bare numeric id",
			in:         "12345",
			wantPageID: "12345",
			wantBareID: true,
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
			if ref.PageID != tt.wantPageID {
				t.Errorf("PageID = %q, want %q", ref.PageID, tt.wantPageID)
			}
			if ref.SpaceKey != tt.wantSpace {
				t.Errorf("SpaceKey = %q, want %q", ref.SpaceKey, tt.wantSpace)
			}
			if ref.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", ref.Title, tt.wantTitle)
			}
			if ref.IsBareID != tt.wantBareID {
				t.Errorf("IsBareID = %v, want %v", ref.IsBareID, tt.wantBareID)
			}
		})
	}
}

func Test_Parse_host_normalization(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantBase string
	}{
		{"host lowercased", "https://Wiki.Example.COM/display/DEV/Home", "https://wiki.example.com"},
		{"https default port stripped", "https://wiki.example.com:443/display/DEV/Home", "https://wiki.example.com"},
		{"http default port stripped", "http://wiki.example.com:80/display/DEV/Home", "http://wiki.example.com"},
		{"non-default port preserved", "http://wiki.local:8090/display/DEV/Home", "http://wiki.local:8090"},
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
		})
	}
}

func Test_Parse_context_path_root_is_empty(t *testing.T) {
	ref, err := Parse("https://wiki.example.com/pages/viewpage.action?pageId=12345")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if ref.ContextPath != "" {
		t.Errorf("ContextPath = %q, want empty for root-mounted instance", ref.ContextPath)
	}
}

func Test_Parse_rejects_malformed(t *testing.T) {
	bad := []string{
		"not a url",
		"",
		"wiki.example.com/display/DEV", // scheme-less
		"https://wiki.example.com/dashboard.action",
		"https://wiki.example.com/admin/",
		"https://wiki.example.com/pages/viewpage.action?pageId=abc", // non-numeric
	}
	for _, in := range bad {
		t.Run(in, func(t *testing.T) {
			if _, err := Parse(in); err == nil {
				t.Errorf("Parse(%q) = nil error, want a parse error", in)
			}
		})
	}
}

// Test_HostKey verifies the credential-lookup key derivation: scheme + host
// (+ non-default port) combined with the context path.
func Test_HostKey(t *testing.T) {
	tests := []struct {
		in      string
		wantKey string
	}{
		{"https://wiki.example.com/display/DEV/Home", "https://wiki.example.com"},
		{"https://wiki.example.com/confluence/display/DEV/Home", "https://wiki.example.com/confluence"},
		{"http://wiki.local:8090/spaces/DEV", "http://wiki.local:8090"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			ref, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			if got := ref.HostKey(); got != tt.wantKey {
				t.Errorf("HostKey() = %q, want %q", got, tt.wantKey)
			}
		})
	}
}
