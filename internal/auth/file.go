package auth

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// credentialsFile is the on-disk TOML shape. Tokens are stored under a single
// table keyed by the instance key (a plain key->token map, so versions without
// alias support can still read it). Aliases live in a separate optional table
// keyed by the same instance key, so a v0.1 reader simply ignores them.
type credentialsFile struct {
	Tokens  map[string]string `toml:"tokens"`
	Aliases map[string]string `toml:"aliases,omitempty"`
}

// Load reads the credentials file at path into a Store. A missing file is not
// an error: it yields an empty store, so first-run commands work before any
// credential is added.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewStore(nil), nil
		}
		return nil, fmt.Errorf("read credentials %s: %w", path, err)
	}

	var cf credentialsFile
	if err := toml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse credentials %s: %w", path, err)
	}
	return newStoreWithAliases(cf.Tokens, cf.Aliases), nil
}

// Save writes the store to path as TOML with file mode 0600, creating parent
// directories (mode 0700) as needed. The write is atomic: it encodes to a temp
// file in the same directory, fsyncs it, and renames it over the target, so a
// crash never leaves a half-written credentials file.
func (s *Store) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}

	var buf bytes.Buffer
	cf := credentialsFile{Tokens: s.tokens}
	if len(s.aliases) > 0 {
		cf.Aliases = s.aliases
	}
	if err := toml.NewEncoder(&buf).Encode(cf); err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp credentials: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we fail before the rename.
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp credentials: %w", err)
	}
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp credentials: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fsync temp credentials: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp credentials: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename credentials into place: %w", err)
	}
	return nil
}

// DefaultPath returns the platform credentials path (~/.config/cfl/credentials
// or the OS-equivalent config directory).
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(dir, "cfl", "credentials"), nil
}
