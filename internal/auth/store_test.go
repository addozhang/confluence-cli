package auth

import (
	"sort"
	"testing"
)

func Test_KeyFromURL(t *testing.T) {
	tests := []struct {
		in      string
		wantKey string
	}{
		{"https://wiki.example.com", "https://wiki.example.com"},
		{"https://wiki.example.com/", "https://wiki.example.com"},
		{"https://wiki.example.com/display/DEV/Home", "https://wiki.example.com"},
		{"https://wiki.example.com/confluence", "https://wiki.example.com/confluence"},
		{"https://wiki.example.com/confluence/display/DEV/Home", "https://wiki.example.com/confluence"},
		{"https://Wiki.Example.COM/display/DEV/Home", "https://wiki.example.com"},
		{"http://wiki.local:8090/spaces/DEV", "http://wiki.local:8090"},
		{"https://wiki.example.com:443/display/DEV/Home", "https://wiki.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := KeyFromURL(tt.in)
			if err != nil {
				t.Fatalf("KeyFromURL(%q) error: %v", tt.in, err)
			}
			if got != tt.wantKey {
				t.Errorf("KeyFromURL(%q) = %q, want %q", tt.in, got, tt.wantKey)
			}
		})
	}
}

func Test_KeyFromURL_rejects_invalid(t *testing.T) {
	bad := []string{
		"",
		"wiki.example.com/display/DEV", // scheme-less
		"ftp://wiki.example.com",       // unsupported scheme
		"https://",                     // no host
	}
	for _, in := range bad {
		t.Run(in, func(t *testing.T) {
			if _, err := KeyFromURL(in); err == nil {
				t.Errorf("KeyFromURL(%q) = nil error, want an error", in)
			}
		})
	}
}

func Test_Store_Resolve_host_only(t *testing.T) {
	s := NewStore(map[string]string{
		"https://wiki.example.com": "host-token",
	})
	tok, ok, err := s.Resolve("https://wiki.example.com/display/DEV/Home")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if !ok || tok != "host-token" {
		t.Errorf("Resolve = (%q, %v), want (host-token, true)", tok, ok)
	}
}

func Test_Store_Resolve_most_specific_wins(t *testing.T) {
	s := NewStore(map[string]string{
		"https://wiki.example.com":            "host-token",
		"https://wiki.example.com/confluence": "ctx-token",
	})
	tok, ok, err := s.Resolve("https://wiki.example.com/confluence/pages/viewpage.action?pageId=12345")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if !ok || tok != "ctx-token" {
		t.Errorf("Resolve = (%q, %v), want (ctx-token, true)", tok, ok)
	}
}

func Test_Store_Resolve_fallback_to_host_only(t *testing.T) {
	s := NewStore(map[string]string{
		"https://wiki.example.com": "host-token",
	})
	// No /wiki entry exists; the host-only key must serve the request.
	tok, ok, err := s.Resolve("https://wiki.example.com/wiki/display/DEV/Home")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if !ok || tok != "host-token" {
		t.Errorf("Resolve = (%q, %v), want (host-token, true)", tok, ok)
	}
}

func Test_Store_Resolve_segment_boundary_prevents_partial_match(t *testing.T) {
	s := NewStore(map[string]string{
		"https://wiki.example.com/conf": "conf-token",
	})
	// The request path /confluence does not continue /conf at a / boundary.
	_, ok, err := s.Resolve("https://wiki.example.com/confluence/display/DEV/Home")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if ok {
		t.Errorf("Resolve matched /conf for /confluence; want missing credential")
	}
}

func Test_Store_Resolve_missing_credential(t *testing.T) {
	s := NewStore(map[string]string{
		"https://wiki.example.com": "host-token",
	})
	_, ok, err := s.Resolve("https://other.example.com/display/DEV/Home")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if ok {
		t.Errorf("Resolve matched a different host; want missing credential")
	}
}

func Test_Store_Resolve_exact_context_path_match(t *testing.T) {
	s := NewStore(map[string]string{
		"https://wiki.example.com/confluence": "ctx-token",
	})
	// The request path equals the context-path key exactly (no trailing path).
	tok, ok, err := s.Resolve("https://wiki.example.com/confluence")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if !ok || tok != "ctx-token" {
		t.Errorf("Resolve = (%q, %v), want (ctx-token, true)", tok, ok)
	}
}

func Test_Store_List_sorted(t *testing.T) {
	s := NewStore(map[string]string{
		"https://b.example.com": "t1",
		"https://a.example.com": "t2",
		"https://c.example.com": "t3",
	})
	got := s.List()
	want := []string{"https://a.example.com", "https://b.example.com", "https://c.example.com"}
	if len(got) != len(want) {
		t.Fatalf("List() len = %d, want %d", len(got), len(want))
	}
	sorted := append([]string(nil), got...)
	sort.Strings(sorted)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("List()[%d] = %q, want %q (List must be sorted)", i, got[i], want[i])
		}
	}
}

func Test_Store_Add_and_Remove(t *testing.T) {
	s := NewStore(nil)
	s.Add("https://wiki.example.com", "tok")
	if tok, ok, _ := s.Resolve("https://wiki.example.com/x"); !ok || tok != "tok" {
		t.Fatalf("after Add, Resolve = (%q, %v), want (tok, true)", tok, ok)
	}

	// Remove is idempotent: removing twice is not an error and the second
	// removal reports it was already absent.
	if removed := s.Remove("https://wiki.example.com"); !removed {
		t.Errorf("first Remove should report removed=true")
	}
	if removed := s.Remove("https://wiki.example.com"); removed {
		t.Errorf("second Remove should report removed=false (idempotent)")
	}
}
