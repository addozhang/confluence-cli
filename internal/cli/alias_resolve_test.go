package cli

import (
	"strings"
	"testing"

	"github.com/addozhang/cfl/internal/auth"
)

func Test_resolveInstance_alias_expands_to_url(t *testing.T) {
	store := auth.NewStore(nil)
	_ = store.AddWithAlias("https://wiki.example.com/confluence", "tok", "prod")

	got, err := resolveInstance("prod", store)
	if err != nil {
		t.Fatalf("resolveInstance(prod) error: %v", err)
	}
	if got != "https://wiki.example.com/confluence" {
		t.Errorf("resolveInstance(prod) = %q, want the aliased instance key", got)
	}
}

func Test_resolveInstance_url_stays_url(t *testing.T) {
	store := auth.NewStore(nil)
	got, err := resolveInstance("https://wiki.example.com", store)
	if err != nil {
		t.Fatalf("resolveInstance(url) error: %v", err)
	}
	if got != "https://wiki.example.com" {
		t.Errorf("a URL value should be returned unchanged, got %q", got)
	}
}

func Test_resolveInstance_value_with_dot_or_slash_is_url(t *testing.T) {
	store := auth.NewStore(nil)
	// Even if no scheme, a value containing '.' or '/' is treated as a URL
	// (host-like), never as an alias.
	for _, v := range []string{"wiki.example.com", "host/path"} {
		got, err := resolveInstance(v, store)
		if err != nil {
			t.Fatalf("resolveInstance(%q) error: %v", v, err)
		}
		if got != v {
			t.Errorf("resolveInstance(%q) = %q, want it treated as a URL (unchanged)", v, got)
		}
	}
}

func Test_resolveInstance_unknown_alias_like_value_errors(t *testing.T) {
	store := auth.NewStore(nil)
	// A bare token-like value (no dot/slash/scheme) that is not a configured
	// alias is an error suggesting auth list.
	_, err := resolveInstance("nope", store)
	if err == nil {
		t.Fatalf("an unknown alias-like value should error")
	}
	if !strings.Contains(err.Error(), "cfl auth list") {
		t.Errorf("error %q should suggest `cfl auth list`", err.Error())
	}
}

func Test_resolveRef_alias_qualified_bare_id(t *testing.T) {
	store := auth.NewStore(nil)
	_ = store.AddWithAlias("https://wiki.example.com/confluence", "tok", "prod")
	// Add a second instance to prove the alias-qualified form is unambiguous
	// even with multiple instances configured.
	store.Add("https://other.example.com", "tok2")

	ref, err := resolveRef("prod:12345", store)
	if err != nil {
		t.Fatalf("resolveRef(prod:12345) error: %v", err)
	}
	if ref.BaseURL != "https://wiki.example.com" || ref.ContextPath != "/confluence" {
		t.Errorf("ref = %+v, want base+context from the aliased instance", ref)
	}
	if ref.PageID != "12345" {
		t.Errorf("PageID = %q, want 12345", ref.PageID)
	}
}

func Test_resolveRef_unknown_alias_prefix_errors(t *testing.T) {
	store := auth.NewStore(nil)
	store.Add("https://wiki.example.com", "tok")

	_, err := resolveRef("staging:12345", store)
	if err == nil {
		t.Fatalf("an unknown alias prefix should error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "staging") || !strings.Contains(err.Error(), "cfl auth list") {
		t.Errorf("error %q should name the alias and suggest `cfl auth list`", err.Error())
	}
}

func Test_resolveRef_url_not_mistaken_for_alias_id(t *testing.T) {
	store := auth.NewStore(nil)
	store.Add("https://wiki.example.com", "tok")

	// A URL contains '://' so it must be parsed as a URL, not <alias>:<id>.
	ref, err := resolveRef("https://wiki.example.com/spaces/ENG/pages/12345", store)
	if err != nil {
		t.Fatalf("resolveRef(url) error: %v", err)
	}
	if ref.PageID != "12345" || ref.SpaceKey != "ENG" {
		t.Errorf("ref = %+v, want parsed from the spaces/pages URL", ref)
	}
}

func Test_resolveRef_alias_colon_nonnumeric_not_alias_form(t *testing.T) {
	store := auth.NewStore(nil)
	_ = store.AddWithAlias("https://wiki.example.com", "tok", "prod")

	// "prod:abc" — alias known but the suffix is not numeric, so it is NOT the
	// alias-qualified bare-id form; it falls through to normal parsing and
	// errors as an unparseable argument.
	if _, err := resolveRef("prod:abc", store); err == nil {
		t.Errorf("prod:abc (non-numeric suffix) should not resolve as an alias-qualified id")
	}
}
