package cli

import (
	stderrors "errors"
	"strings"
	"testing"

	"github.com/addozhang/cfl/internal/auth"
	cflerrors "github.com/addozhang/cfl/internal/errors"
)

func Test_resolveRef_full_url(t *testing.T) {
	store := auth.NewStore(map[string]string{"https://wiki.example.com": "tok"})
	ref, err := resolveRef("https://wiki.example.com/pages/viewpage.action?pageId=12345", store)
	if err != nil {
		t.Fatalf("resolveRef error: %v", err)
	}
	if ref.BaseURL != "https://wiki.example.com" || ref.PageID != "12345" {
		t.Errorf("ref = %+v, want base=https://wiki.example.com pageID=12345", ref)
	}
}

func Test_resolveRef_bare_id_single_instance(t *testing.T) {
	store := auth.NewStore(map[string]string{"https://wiki.example.com": "tok"})
	ref, err := resolveRef("12345", store)
	if err != nil {
		t.Fatalf("resolveRef error: %v", err)
	}
	if ref.BaseURL != "https://wiki.example.com" {
		t.Errorf("BaseURL = %q, want the single configured instance", ref.BaseURL)
	}
	if ref.PageID != "12345" {
		t.Errorf("PageID = %q, want 12345", ref.PageID)
	}
}

func Test_resolveRef_bare_id_no_instance(t *testing.T) {
	store := auth.NewStore(nil)
	_, err := resolveRef("12345", store)
	if err == nil {
		t.Fatalf("bare id with no configured instance should error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "full") {
		t.Errorf("error %q should tell the user to pass a full URL", err.Error())
	}
}

func Test_resolveRef_bare_id_multiple_instances(t *testing.T) {
	store := auth.NewStore(map[string]string{
		"https://a.example.com": "t1",
		"https://b.example.com": "t2",
	})
	_, err := resolveRef("12345", store)
	if err == nil {
		t.Fatalf("bare id with multiple instances should error as ambiguous")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
		t.Errorf("error %q should mention ambiguity", err.Error())
	}
}

func Test_resolveRef_bare_id_context_path_from_instance(t *testing.T) {
	store := auth.NewStore(map[string]string{"https://wiki.example.com/confluence": "tok"})
	ref, err := resolveRef("12345", store)
	if err != nil {
		t.Fatalf("resolveRef error: %v", err)
	}
	if ref.BaseURL != "https://wiki.example.com" || ref.ContextPath != "/confluence" {
		t.Errorf("ref = %+v, want base+contextpath from the single instance", ref)
	}
}

func Test_resolveRef_malformed(t *testing.T) {
	store := auth.NewStore(nil)
	_, err := resolveRef("not a url", store)
	if err == nil {
		t.Fatalf("malformed arg should error")
	}
	var cflErr *cflerrors.CFLError
	if !stderrors.As(err, &cflErr) || cflErr.Code != cflerrors.CodeBadURL {
		t.Errorf("error = %v, want CFLError with CodeBadURL", err)
	}
}

func Test_requireCredential_missing_gives_onboarding(t *testing.T) {
	store := auth.NewStore(nil)
	err := requireCredential("https://wiki.example.com", store)
	if err == nil {
		t.Fatalf("missing credential should error with onboarding guidance")
	}
	var cflErr *cflerrors.CFLError
	if !stderrors.As(err, &cflErr) {
		t.Fatalf("error = %T, want *CFLError", err)
	}
	// The exact `cfl auth add <url>` command must appear so whichever command
	// the user runs first, the path to a working setup is named.
	if !strings.Contains(cflErr.Message, "cfl auth add https://wiki.example.com") {
		t.Errorf("message %q should name the exact auth add command", cflErr.Message)
	}
}

func Test_requireCredential_present_ok(t *testing.T) {
	store := auth.NewStore(map[string]string{"https://wiki.example.com": "tok"})
	if err := requireCredential("https://wiki.example.com/display/DEV/Home", store); err != nil {
		t.Errorf("present credential should not error, got: %v", err)
	}
}
