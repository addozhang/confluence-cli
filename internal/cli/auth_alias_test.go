package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/addozhang/cfl/internal/auth"
)

func Test_cmd_auth_add_with_alias(t *testing.T) {
	orig := promptHiddenToken
	promptHiddenToken = func(string) (string, error) { return "PAT-1", nil }
	defer func() { promptHiddenToken = orig }()

	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")

	out, _, err := runCmd(t, creds, "", "auth", "add", "https://wiki.example.com", "--alias", "prod")
	if err != nil {
		t.Fatalf("auth add --alias error: %v", err)
	}
	if !strings.Contains(out, "prod") {
		t.Errorf("confirmation should name the alias, got: %q", out)
	}
	if strings.Contains(out, "PAT-1") {
		t.Errorf("auth add leaked the token: %q", out)
	}

	store, _ := auth.Load(creds)
	if key, ok := store.ResolveAlias("prod"); !ok || key != "https://wiki.example.com" {
		t.Errorf("alias not stored: (%q, %v)", key, ok)
	}
}

func Test_cmd_auth_add_duplicate_alias_rejected(t *testing.T) {
	orig := promptHiddenToken
	promptHiddenToken = func(string) (string, error) { return "PAT", nil }
	defer func() { promptHiddenToken = orig }()

	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")
	// Seed an instance already holding alias 'prod'.
	s := auth.NewStore(nil)
	_ = s.AddWithAlias("https://wiki.example.com", "tok", "prod")
	if err := s.Save(creds); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, _, err := runCmd(t, creds, "", "auth", "add", "https://other.example.com", "--alias", "prod")
	if err == nil {
		t.Fatalf("adding a duplicate alias should fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "prod") {
		t.Errorf("error %q should name the conflicting alias", err.Error())
	}
}

func Test_cmd_auth_list_shows_alias(t *testing.T) {
	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")
	s := auth.NewStore(nil)
	_ = s.AddWithAlias("https://wiki.example.com", "SECRET", "prod")
	s.Add("https://other.example.com", "SECRET2")
	if err := s.Save(creds); err != nil {
		t.Fatalf("seed: %v", err)
	}

	out, _, err := runCmd(t, creds, "", "auth", "list", "-o", "json")
	if err != nil {
		t.Fatalf("auth list error: %v", err)
	}
	if strings.Contains(out, "SECRET") {
		t.Errorf("auth list leaked a token: %q", out)
	}

	var decoded struct {
		Instances []struct {
			Key   string  `json:"key"`
			Alias *string `json:"alias"`
		} `json:"instances"`
	}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output not JSON in the expected shape: %v (%q)", err, out)
	}
	if len(decoded.Instances) != 2 {
		t.Fatalf("want 2 instances, got %d: %q", len(decoded.Instances), out)
	}
	// Find the wiki instance and check its alias.
	var wikiAlias *string
	var otherAlias *string
	for _, inst := range decoded.Instances {
		switch inst.Key {
		case "https://wiki.example.com":
			wikiAlias = inst.Alias
		case "https://other.example.com":
			otherAlias = inst.Alias
		}
	}
	if wikiAlias == nil || *wikiAlias != "prod" {
		t.Errorf("wiki instance alias = %v, want prod", wikiAlias)
	}
	if otherAlias != nil {
		t.Errorf("other instance alias = %v, want null", *otherAlias)
	}
}
