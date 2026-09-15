package auth

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

// keyringService namespaces cfl's entries inside the OS keyring. The account
// of each entry is the instance key (e.g. "https://wiki.example.com"), so on
// macOS `security find-generic-password -s cfl -a <key>` locates one.
const keyringService = "cfl"

// keyringGet/keyringSet/keyringDelete are package vars so tests can substitute
// an in-memory mock instead of touching the real OS keyring. Production use
// always goes through the go-keyring implementations.
var (
	keyringGet    = keyring.Get
	keyringSet    = keyring.Set
	keyringDelete = keyring.Delete
)

// KeyringError reports a failure to read or write an OS-keyring entry for an
// instance key. Missing distinguishes an absent entry (the token was deleted
// outside cfl, or never written) from an unreadable or unwritable keyring,
// because the two call for different remediation: re-run `cfl auth add <url>
// --secure-storage` versus fall back to plain credentials-file storage.
type KeyringError struct {
	Key     string
	Missing bool
	Err     error
}

// Error renders the failure in the package's fmt style. The CLI layer wraps
// KeyringError in a CFLError carrying the actionable suggestion.
func (e *KeyringError) Error() string {
	if e.Missing {
		return fmt.Sprintf("no token for %s in the OS keyring", e.Key)
	}
	return fmt.Sprintf("keyring operation for %s failed: %v", e.Key, e.Err)
}

// Unwrap exposes the underlying keyring failure for errors.Is/errors.As.
func (e *KeyringError) Unwrap() error {
	return e.Err
}
