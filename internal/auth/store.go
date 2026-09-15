// Package auth stores per-instance Personal Access Tokens and resolves a
// request URL to the most specific stored credential. The lookup key is the URL
// scheme + host (+ non-default port) plus an optional context-path prefix; the
// token is sent as an HTTP Bearer header by the transport layer.
package auth

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/zalando/go-keyring"
)

// markerSegments delimit the context-path prefix from the Confluence-specific
// path. They mirror confluenceurl's markers but are applied here in a looser
// context: KeyFromURL also accepts a bare instance base URL (no page/space
// shape), which confluenceurl.Parse deliberately rejects.
var markerSegments = map[string]bool{
	"rest":    true,
	"display": true,
	"spaces":  true,
	"pages":   true,
}

// Store holds instance keys mapped to PAT tokens, plus optional per-instance
// aliases. It is safe for sequential command use; cfl issues no concurrent
// requests.
type Store struct {
	tokens map[string]string
	// aliases maps an instance key to its short alias. Stored separately from
	// tokens so the on-disk [tokens] table stays a plain key->token map,
	// readable by versions without alias support.
	aliases map[string]string
	// secure records the instance keys whose real token lives in the OS
	// keyring. For those keys tokens[key] is "", so the on-disk file never
	// holds the secret and stays readable by versions without keyring support.
	secure map[string]bool
}

// NewStore builds a Store from an existing key→token map. A nil map yields an
// empty store.
func NewStore(tokens map[string]string) *Store {
	if tokens == nil {
		tokens = map[string]string{}
	}
	return &Store{tokens: tokens, aliases: map[string]string{}, secure: map[string]bool{}}
}

// newStoreWithAliases builds a Store from all three on-disk maps (used by
// Load). Keys marked secure have their file token forced to "": the real token
// lives only in the keyring, and a stray plaintext value must not linger.
func newStoreWithAliases(tokens map[string]string, aliases map[string]string, secure map[string]bool) *Store {
	s := NewStore(tokens)
	if aliases != nil {
		s.aliases = aliases
	}
	if secure != nil {
		s.secure = secure
	}
	for key := range s.secure {
		s.tokens[key] = ""
	}
	return s
}

var aliasRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// AddWithAlias stores or overwrites the token for key and binds the given alias
// to it. The alias must match [a-zA-Z0-9_-]+ and must not already be bound to a
// different instance. Re-binding the same alias to the same instance (with a new
// token) is idempotent. With secure set, the token is written to the OS keyring
// instead of the store; the write happens only after alias validation passes.
func (s *Store) AddWithAlias(key, token, alias string, secure bool) error {
	if !aliasRe.MatchString(alias) {
		return fmt.Errorf("alias %q must match [a-zA-Z0-9_-]+", alias)
	}
	if existing, ok := s.aliases[key]; ok && existing == alias {
		// same instance already holds this alias; just update the token below
	} else if owner, taken := s.aliasOwner(alias); taken && owner != key {
		return fmt.Errorf("alias %q is already in use by %s", alias, owner)
	}
	if err := s.Add(key, token, secure); err != nil {
		return err
	}
	s.aliases[key] = alias
	return nil
}

// ResolveAlias maps an alias name to its instance key.
func (s *Store) ResolveAlias(alias string) (string, bool) {
	key, ok := s.aliasOwner(alias)
	return key, ok
}

// AliasOf returns the alias bound to an instance key, if any.
func (s *Store) AliasOf(key string) (string, bool) {
	a, ok := s.aliases[key]
	return a, ok
}

// Aliases returns a copy of the key→alias map.
func (s *Store) Aliases() map[string]string {
	out := make(map[string]string, len(s.aliases))
	for k, v := range s.aliases {
		out[k] = v
	}
	return out
}

// aliasOwner finds the instance key bound to an alias.
func (s *Store) aliasOwner(alias string) (string, bool) {
	for key, a := range s.aliases {
		if a == alias {
			return key, true
		}
	}
	return "", false
}

// KeyFromURL derives the credential-lookup key for a URL: scheme + lowercased
// host (+ non-default port) plus the context-path prefix (the path segments
// before the first Confluence marker, or the whole path when none is present).
// Unlike confluenceurl.Parse, it accepts a bare instance base URL with no
// page/space shape, because `cfl auth add` keys on the instance, not a page.
func KeyFromURL(rawURL string) (string, error) {
	scheme, host, path, err := splitRequest(rawURL)
	if err != nil {
		return "", err
	}
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("URL must use http or https scheme")
	}
	if host == "" {
		return "", fmt.Errorf("URL has no host")
	}
	ctx := contextPathOf(path)
	return scheme + "://" + host + ctx, nil
}

// contextPathOf extracts the context-path prefix from a request path: the
// segments before the first marker segment, normalized to a leading "/" with no
// trailing slash, or "" when the path is empty or begins with a marker.
func contextPathOf(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	for i, p := range parts {
		if markerSegments[p] {
			if i == 0 {
				return ""
			}
			return "/" + strings.Join(parts[:i], "/")
		}
	}
	// No marker: the whole path is the context path.
	return "/" + strings.Join(parts, "/")
}

// Add stores or overwrites the token for the given instance key. With secure
// set, the token is written to the OS keyring under the instance key and the
// in-memory token entry is set to "", so a later Save records only that the
// instance uses the keyring. Re-adding a secure key without the flag clears the
// secure marker and removes the now-stale keyring entry (best effort).
func (s *Store) Add(key, token string, secure bool) error {
	if !secure && s.secure[key] {
		_ = keyringDelete(keyringService, key)
	}
	if secure {
		if err := keyringSet(keyringService, key, token); err != nil {
			return &KeyringError{Key: key, Err: err}
		}
		s.tokens[key] = ""
		s.secure[key] = true
		return nil
	}
	delete(s.secure, key)
	s.tokens[key] = token
	return nil
}

// Remove deletes the token for key. It reports whether an entry was present, so
// callers can distinguish a real removal from an idempotent no-op. Removing a
// secure key also deletes its keyring entry (best effort: a keyring miss must
// not block the removal, the entry is unreachable either way).
func (s *Store) Remove(key string) bool {
	if _, ok := s.tokens[key]; !ok {
		return false
	}
	if s.secure[key] {
		_ = keyringDelete(keyringService, key)
	}
	delete(s.secure, key)
	delete(s.tokens, key)
	delete(s.aliases, key)
	return true
}

// List returns the configured instance keys in sorted order. It never returns
// any token value.
func (s *Store) List() []string {
	keys := make([]string, 0, len(s.tokens))
	for k := range s.tokens {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SecureKeys returns the instance keys whose tokens live in the OS keyring, in
// sorted order. `cfl auth list` exposes them through the per-instance secure
// flag; it never returns any token value.
func (s *Store) SecureKeys() []string {
	keys := make([]string, 0, len(s.secure))
	for k := range s.secure {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Resolve selects the most specific stored token for a request URL. A key
// matches when its scheme+host equal the request's and the request path either
// equals the key's context path or continues past it at a "/" segment boundary.
// Among matches, the longest context path wins; a host-only key is the shortest
// prefix. It returns (token, true, nil) on a match, ("", false, nil) when no key
// matches, or an error when the request URL is unparseable or the matched key
// is secure and its keyring entry cannot be read.
func (s *Store) Resolve(rawURL string) (string, bool, error) {
	reqScheme, reqHost, reqPath, err := splitRequest(rawURL)
	if err != nil {
		return "", false, err
	}

	bestKey := ""
	bestLen := -1
	for key := range s.tokens {
		keyScheme, keyHost, keyCtx, ok := splitKey(key)
		if !ok || keyScheme != reqScheme || keyHost != reqHost {
			continue
		}
		if !pathMatches(reqPath, keyCtx) {
			continue
		}
		if len(keyCtx) > bestLen {
			bestKey = key
			bestLen = len(keyCtx)
		}
	}
	if bestLen < 0 {
		return "", false, nil
	}
	if s.secure[bestKey] {
		token, err := keyringGet(keyringService, bestKey)
		if errors.Is(err, keyring.ErrNotFound) {
			return "", false, &KeyringError{Key: bestKey, Missing: true, Err: err}
		}
		if err != nil {
			return "", false, &KeyringError{Key: bestKey, Err: err}
		}
		return token, true, nil
	}
	return s.tokens[bestKey], true, nil
}

// splitRequest normalizes a request URL into (scheme, host-with-port, path).
// The host is lowercased and the default port stripped, matching key form.
func splitRequest(rawURL string) (scheme, host, path string, err error) {
	u, perr := url.Parse(strings.TrimSpace(rawURL))
	if perr != nil {
		return "", "", "", perr
	}
	scheme = u.Scheme
	host = strings.ToLower(u.Hostname())
	if port := u.Port(); port != "" && !isDefaultPort(scheme, port) {
		host += ":" + port
	}
	path = strings.TrimRight(u.Path, "/")
	return scheme, host, path, nil
}

// splitKey decomposes a stored key (scheme://host[:port]<contextpath>) into its
// parts. The context path is "" for a host-only key.
func splitKey(key string) (scheme, host, ctx string, ok bool) {
	idx := strings.Index(key, "://")
	if idx < 0 {
		return "", "", "", false
	}
	scheme = key[:idx]
	rest := key[idx+3:]
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		host = rest[:slash]
		ctx = strings.TrimRight(rest[slash:], "/")
	} else {
		host = rest
		ctx = ""
	}
	return scheme, host, ctx, host != ""
}

// pathMatches reports whether a request path is served by a key context path:
// either an exact match, a host-only key (empty ctx always matches), or the
// request continuing past the ctx at a "/" boundary.
func pathMatches(reqPath, keyCtx string) bool {
	if keyCtx == "" {
		return true
	}
	if reqPath == keyCtx {
		return true
	}
	return strings.HasPrefix(reqPath, keyCtx+"/")
}

func isDefaultPort(scheme, port string) bool {
	return (scheme == "https" && port == "443") || (scheme == "http" && port == "80")
}
