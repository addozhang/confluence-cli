package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func Test_Load_missing_file_is_empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist", "credentials")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load of a missing file should succeed, got: %v", err)
	}
	if len(s.List()) != 0 {
		t.Errorf("missing file should yield an empty store, got %v", s.List())
	}
}

func Test_Save_then_Load_round_trip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials")
	s := NewStore(map[string]string{
		"https://wiki.example.com":            "tok-1",
		"https://wiki.example.com/confluence": "tok-2",
	})
	if err := s.Save(path); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if tok, ok, _ := loaded.Resolve("https://wiki.example.com/x"); !ok || tok != "tok-1" {
		t.Errorf("round-trip host token = (%q, %v), want (tok-1, true)", tok, ok)
	}
	if tok, ok, _ := loaded.Resolve("https://wiki.example.com/confluence/x"); !ok || tok != "tok-2" {
		t.Errorf("round-trip ctx token = (%q, %v), want (tok-2, true)", tok, ok)
	}
}

func Test_Save_uses_mode_0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file mode not enforced on Windows")
	}
	path := filepath.Join(t.TempDir(), "credentials")
	s := NewStore(map[string]string{"https://wiki.example.com": "tok"})
	if err := s.Save(path); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials file mode = %o, want 0600", perm)
	}
}

func Test_Save_preserves_mode_0600_on_rewrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file mode not enforced on Windows")
	}
	path := filepath.Join(t.TempDir(), "credentials")
	s := NewStore(map[string]string{"https://wiki.example.com": "tok"})
	if err := s.Save(path); err != nil {
		t.Fatalf("first Save error: %v", err)
	}

	// Read-modify-write: remove one, add another, save again.
	s.Remove("https://wiki.example.com")
	s.Add("https://other.example.com", "tok2")
	if err := s.Save(path); err != nil {
		t.Fatalf("second Save error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("after rewrite, mode = %o, want 0600", perm)
	}
}

func Test_Save_creates_parent_dir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "cfl", "credentials")
	s := NewStore(map[string]string{"https://wiki.example.com": "tok"})
	if err := s.Save(path); err != nil {
		t.Fatalf("Save should create parent dirs, got: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("credentials file not created: %v", err)
	}
}
