package auth

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// useMockKeyring substitutes an in-memory keyring for the injectable package
// vars and returns a restore func. With fail set, every operation errors,
// simulating an unavailable keyring (e.g. no unlocked keychain session).
func useMockKeyring(store map[string]string, fail bool) func() {
	origGet, origSet, origDelete := keyringGet, keyringSet, keyringDelete
	keyringGet = func(service, user string) (string, error) {
		if fail {
			return "", errors.New("keyring unavailable")
		}
		token, ok := store[user]
		if !ok {
			return "", keyring.ErrNotFound
		}
		return token, nil
	}
	keyringSet = func(service, user, token string) error {
		if fail {
			return errors.New("keyring unavailable")
		}
		store[user] = token
		return nil
	}
	keyringDelete = func(service, user string) error {
		if fail {
			return errors.New("keyring unavailable")
		}
		delete(store, user)
		return nil
	}
	return func() { keyringGet, keyringSet, keyringDelete = origGet, origSet, origDelete }
}

func Test_AddSecure_round_trip(t *testing.T) {
	mock := map[string]string{}
	restore := useMockKeyring(mock, false)
	defer restore()

	key := "https://wiki.example.com"
	path := filepath.Join(t.TempDir(), "credentials")
	s := NewStore(nil)
	if err := s.AddWithAlias(key, "secret", "prod", true); err != nil {
		t.Fatalf("secure AddWithAlias: %v", err)
	}
	// The in-memory entry holds only an empty placeholder; the secret went to
	// the keyring.
	if tok := s.tokens[key]; tok != "" {
		t.Errorf("in-memory token for a secure key = %q, want an empty placeholder", tok)
	}
	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret") {
		t.Errorf("token leaked into the credentials file:\n%s", raw)
	}
	if !strings.Contains(string(raw), "[secure]") {
		t.Errorf("credentials file lacks the [secure] table:\n%s", raw)
	}
	if !strings.Contains(string(raw), `"https://wiki.example.com" = ""`) {
		t.Errorf("a secure key must keep an empty [tokens] value for old readers:\n%s", raw)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := loaded.SecureKeys(); len(got) != 1 || got[0] != key {
		t.Errorf("SecureKeys() = %v, want [%s]", got, key)
	}
	tok, ok, err := loaded.Resolve("https://wiki.example.com/display/DEV/Home")
	if err != nil || !ok {
		t.Fatalf("Resolve = (%q, %v, %v), want the keyring token", tok, ok, err)
	}
	if tok != "secret" {
		t.Errorf("Resolve token = %q, want the value fetched from the keyring", tok)
	}

	// Removing the instance must also delete its keyring entry.
	if removed := loaded.Remove(key); !removed {
		t.Fatalf("Remove(%s) = false, want true", key)
	}
	if _, ok := mock[key]; ok {
		t.Errorf("keyring entry for %s survived Remove", key)
	}
	if err := loaded.Save(path); err != nil {
		t.Fatalf("Save after Remove: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := reloaded.List(); len(got) != 0 {
		t.Errorf("List() after removal = %v, want empty", got)
	}
	if got := reloaded.SecureKeys(); len(got) != 0 {
		t.Errorf("SecureKeys() after removal = %v, want empty", got)
	}
}

func Test_AddSecure_keyring_unavailable_keeps_file_clean(t *testing.T) {
	restore := useMockKeyring(map[string]string{}, true)
	defer restore()

	key := "https://wiki.example.com"
	path := filepath.Join(t.TempDir(), "credentials")
	// Seed an existing plain credential so the test can prove a failed secure
	// add leaves the file untouched.
	seed := NewStore(map[string]string{"https://other.example.com": "tok"})
	if err := seed.Save(path); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	err = s.Add(key, "secret", true)
	if err == nil {
		t.Fatal("secure Add should fail when the keyring is unavailable")
	}
	var ke *KeyringError
	if !errors.As(err, &ke) || ke.Missing {
		t.Fatalf("error = %v, want a *KeyringError without Missing", err)
	}
	// The failed add must not have mutated the store...
	if _, ok := s.tokens[key]; ok {
		t.Errorf("failed secure add left a token entry for %s", key)
	}
	if s.secure[key] {
		t.Errorf("failed secure add left a secure marker for %s", key)
	}
	// ...so saving it back leaves the file byte-identical.
	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("credentials file changed after a failed secure add:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func Test_Resolve_secure_missing_keyring_token(t *testing.T) {
	mock := map[string]string{}
	restore := useMockKeyring(mock, false)
	defer restore()

	key := "https://wiki.example.com"
	s := NewStore(nil)
	if err := s.Add(key, "secret", true); err != nil {
		t.Fatalf("secure Add: %v", err)
	}
	// Simulate the token being deleted outside cfl.
	delete(mock, key)

	tok, ok, err := s.Resolve("https://wiki.example.com/x")
	if ok {
		t.Fatalf("Resolve matched although the keyring entry is gone")
	}
	var ke *KeyringError
	if !errors.As(err, &ke) || !ke.Missing {
		t.Fatalf("error = %v, want a *KeyringError with Missing set", err)
	}
	if !strings.Contains(err.Error(), "no token for "+key) {
		t.Errorf("error %q should name the key and the keyring", err.Error())
	}
	if tok != "" {
		t.Errorf("Resolve token = %q, want empty on error", tok)
	}
}

func Test_Add_plain_over_secure_cleans_keyring(t *testing.T) {
	mock := map[string]string{}
	restore := useMockKeyring(mock, false)
	defer restore()

	key := "https://wiki.example.com"
	s := NewStore(nil)
	if err := s.Add(key, "secret", true); err != nil {
		t.Fatalf("secure Add: %v", err)
	}
	if err := s.Add(key, "plain", false); err != nil {
		t.Fatalf("plain re-add: %v", err)
	}
	if len(mock) != 0 {
		t.Errorf("stale keyring entry was not cleaned up: %v", mock)
	}
	if s.secure[key] {
		t.Errorf("secure marker survived the plain re-add")
	}
	tok, ok, _ := s.Resolve("https://wiki.example.com/x")
	if !ok || tok != "plain" {
		t.Errorf("Resolve = (%q, %v), want (plain, true)", tok, ok)
	}
}

func Test_Load_file_without_secure_table(t *testing.T) {
	// A credentials file written before keyring support (only [tokens] and
	// [aliases]) must load with no secure keys.
	path := filepath.Join(t.TempDir(), "credentials")
	content := "[tokens]\n" +
		"  \"https://wiki.example.com\" = \"tok\"\n" +
		"[aliases]\n" +
		"  \"https://wiki.example.com\" = \"prod\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := s.SecureKeys(); len(got) != 0 {
		t.Errorf("SecureKeys() = %v, want empty", got)
	}
	if tok, ok, _ := s.Resolve("https://wiki.example.com/x"); !ok || tok != "tok" {
		t.Errorf("Resolve = (%q, %v), want (tok, true)", tok, ok)
	}
}

func Test_Load_stray_token_for_secure_key_is_cleared(t *testing.T) {
	// If a hand-edited file carries both a [secure] marker and a plaintext
	// token for the same key, the plaintext value must be discarded and the
	// keyring entry wins.
	restore := useMockKeyring(map[string]string{"https://wiki.example.com": "kring"}, false)
	defer restore()

	path := filepath.Join(t.TempDir(), "credentials")
	content := "[tokens]\n" +
		"  \"https://wiki.example.com\" = \"leftover\"\n" +
		"[secure]\n" +
		"  \"https://wiki.example.com\" = true\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tok, ok, err := s.Resolve("https://wiki.example.com/x")
	if err != nil || !ok {
		t.Fatalf("Resolve = (%q, %v, %v), want the keyring token", tok, ok, err)
	}
	if tok != "kring" {
		t.Errorf("Resolve token = %q, want the keyring value %q", tok, "kring")
	}
}
