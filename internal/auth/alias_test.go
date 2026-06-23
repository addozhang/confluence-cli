package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_Add_with_alias_and_ResolveAlias(t *testing.T) {
	s := NewStore(nil)
	if err := s.AddWithAlias("https://wiki.example.com", "tok", "prod"); err != nil {
		t.Fatalf("AddWithAlias error: %v", err)
	}

	key, ok := s.ResolveAlias("prod")
	if !ok || key != "https://wiki.example.com" {
		t.Errorf("ResolveAlias(prod) = (%q, %v), want (https://wiki.example.com, true)", key, ok)
	}
	// The token still resolves normally.
	if tok, ok, _ := s.Resolve("https://wiki.example.com/x"); !ok || tok != "tok" {
		t.Errorf("Resolve = (%q, %v), want (tok, true)", tok, ok)
	}
}

func Test_ResolveAlias_unknown_is_miss(t *testing.T) {
	s := NewStore(nil)
	if _, ok := s.ResolveAlias("nope"); ok {
		t.Errorf("ResolveAlias of an unknown alias should be a miss")
	}
}

func Test_AddWithAlias_rejects_malformed_alias(t *testing.T) {
	s := NewStore(nil)
	for _, bad := range []string{"has space", "with/slash", "with:colon", "with.dot", ""} {
		if err := s.AddWithAlias("https://wiki.example.com", "tok", bad); err == nil {
			t.Errorf("AddWithAlias should reject malformed alias %q", bad)
		}
	}
}

func Test_AddWithAlias_rejects_duplicate_on_different_instance(t *testing.T) {
	s := NewStore(nil)
	if err := s.AddWithAlias("https://a.example.com", "t1", "prod"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	err := s.AddWithAlias("https://b.example.com", "t2", "prod")
	if err == nil {
		t.Fatalf("adding alias 'prod' to a second instance should be rejected")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "prod") {
		t.Errorf("error %q should name the conflicting alias", err.Error())
	}
	// The original entry must be untouched.
	if key, ok := s.ResolveAlias("prod"); !ok || key != "https://a.example.com" {
		t.Errorf("original alias binding changed: (%q, %v)", key, ok)
	}
}

func Test_AddWithAlias_idempotent_same_instance(t *testing.T) {
	s := NewStore(nil)
	if err := s.AddWithAlias("https://wiki.example.com", "t1", "prod"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	// Re-adding the same instance+alias (new token) is allowed.
	if err := s.AddWithAlias("https://wiki.example.com", "t2", "prod"); err != nil {
		t.Errorf("re-adding same instance+alias should be idempotent, got: %v", err)
	}
	if tok, _, _ := s.Resolve("https://wiki.example.com/x"); tok != "t2" {
		t.Errorf("token should be updated to t2, got %q", tok)
	}
}

func Test_AliasOf(t *testing.T) {
	s := NewStore(nil)
	_ = s.AddWithAlias("https://wiki.example.com", "tok", "prod")
	s.Add("https://other.example.com", "tok2") // no alias

	if a, ok := s.AliasOf("https://wiki.example.com"); !ok || a != "prod" {
		t.Errorf("AliasOf(wiki) = (%q, %v), want (prod, true)", a, ok)
	}
	if _, ok := s.AliasOf("https://other.example.com"); ok {
		t.Errorf("AliasOf(other) should report no alias")
	}
}

func Test_Save_Load_round_trip_with_alias(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials")
	s := NewStore(nil)
	_ = s.AddWithAlias("https://wiki.example.com", "tok", "prod")
	s.Add("https://other.example.com", "tok2")
	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if key, ok := loaded.ResolveAlias("prod"); !ok || key != "https://wiki.example.com" {
		t.Errorf("alias did not round-trip: (%q, %v)", key, ok)
	}
	if tok, _, _ := loaded.Resolve("https://other.example.com/x"); tok != "tok2" {
		t.Errorf("alias-less entry did not round-trip: %q", tok)
	}
}

func Test_Load_v01_file_without_aliases(t *testing.T) {
	// A credentials file written by v0.1 (only a [tokens] table) must load.
	path := filepath.Join(t.TempDir(), "credentials")
	v01 := `[tokens]
  "https://wiki.example.com" = "legacy-token"
`
	if err := os.WriteFile(path, []byte(v01), 0o600); err != nil {
		t.Fatalf("write v0.1 file: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load of a v0.1 file should succeed: %v", err)
	}
	if tok, ok, _ := s.Resolve("https://wiki.example.com/x"); !ok || tok != "legacy-token" {
		t.Errorf("v0.1 token = (%q, %v), want (legacy-token, true)", tok, ok)
	}
	if _, ok := s.AliasOf("https://wiki.example.com"); ok {
		t.Errorf("a v0.1 entry should have no alias")
	}
}

func Test_v02_file_readable_as_v01_tokens(t *testing.T) {
	// A file written by v0.2 must keep [tokens] as plain key->token so a v0.1
	// binary can still read the tokens (forward compatibility).
	path := filepath.Join(t.TempDir(), "credentials")
	s := NewStore(nil)
	_ = s.AddWithAlias("https://wiki.example.com", "tok", "prod")
	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "[tokens]") {
		t.Errorf("v0.2 file must retain a [tokens] table, got:\n%s", content)
	}
	if !strings.Contains(content, `"https://wiki.example.com" = "tok"`) {
		t.Errorf("v0.2 [tokens] must map key->token as a plain string, got:\n%s", content)
	}
}
