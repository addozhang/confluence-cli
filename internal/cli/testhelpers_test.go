package cli

import (
	"io"
	"net/http"
	"os"

	"github.com/addozhang/cfl/internal/auth"
)

// seedCredential writes a credentials file with a single instance->token entry,
// used by command tests to set up an authenticated instance without going
// through the interactive `auth add` prompt.
func seedCredential(path, instanceKey, token string) error {
	store := auth.NewStore(map[string]string{instanceKey: token})
	return store.Save(path)
}

// readAll reads and returns a request body, ignoring close errors (test only).
func readAll(r *http.Request) ([]byte, error) {
	defer func() { _ = r.Body.Close() }()
	return io.ReadAll(r.Body)
}

// writeFile writes content to path with 0600, for body-from-file tests.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
