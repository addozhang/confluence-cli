package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/addozhang/cfl/internal/auth"
	cflerrors "github.com/addozhang/cfl/internal/errors"
)

func Test_translateKeyringError_missing_points_at_secure_readd(t *testing.T) {
	err := translateKeyringError(&auth.KeyringError{Key: "https://wiki.example.com", Missing: true})
	if err == nil {
		t.Fatal("expected the keyring error to translate")
	}
	cflok, ok := cflerrors.AsCFLError(err)
	if !ok {
		t.Fatalf("error %v is not a *CFLError", err)
	}
	if !strings.Contains(cflok.Suggestion, "--secure-storage") {
		t.Errorf("suggestion should name `--secure-storage`: %q", cflok.Suggestion)
	}
	if !strings.Contains(cflok.Suggestion, "https://wiki.example.com") {
		t.Errorf("suggestion should name the instance key: %q", cflok.Suggestion)
	}
}

func Test_translateKeyringError_failure_points_at_plain_storage(t *testing.T) {
	err := translateKeyringError(&auth.KeyringError{Key: "https://wiki.example.com", Err: errors.New("locked")})
	if err == nil {
		t.Fatal("expected the keyring error to translate")
	}
	cflok, ok := cflerrors.AsCFLError(err)
	if !ok {
		t.Fatalf("error %v is not a *CFLError", err)
	}
	if !strings.Contains(cflok.Suggestion, "without --secure-storage") {
		t.Errorf("suggestion should offer plain file storage: %q", cflok.Suggestion)
	}
}

func Test_translateKeyringError_passes_other_errors_through(t *testing.T) {
	plain := errors.New("alias conflict")
	if err := translateKeyringError(plain); err != nil {
		t.Errorf("non-keyring errors must pass through untranslated, got %v", err)
	}
}
