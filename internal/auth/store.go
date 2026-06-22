// Package auth stores per-instance Personal Access Tokens and resolves a
// request URL to the most specific stored credential. The lookup key is the URL
// scheme + host (+ non-default port) plus an optional context-path prefix; the
// token is sent as an HTTP Bearer header by the transport layer.
package auth

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
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

// Store holds instance keys mapped to PAT tokens. It is safe for sequential
// command use; cfl issues no concurrent requests.
type Store struct {
	tokens map[string]string
}

// NewStore builds a Store from an existing key→token map. A nil map yields an
// empty store.
func NewStore(tokens map[string]string) *Store {
	if tokens == nil {
		tokens = map[string]string{}
	}
	return &Store{tokens: tokens}
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

// Add stores or overwrites the token for the given instance key.
func (s *Store) Add(key, token string) {
	s.tokens[key] = token
}

// Remove deletes the token for key. It reports whether an entry was present, so
// callers can distinguish a real removal from an idempotent no-op.
func (s *Store) Remove(key string) bool {
	if _, ok := s.tokens[key]; !ok {
		return false
	}
	delete(s.tokens, key)
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

// Resolve selects the most specific stored token for a request URL. A key
// matches when its scheme+host equal the request's and the request path either
// equals the key's context path or continues past it at a "/" segment boundary.
// Among matches, the longest context path wins; a host-only key is the shortest
// prefix. It returns (token, true, nil) on a match, ("", false, nil) when no key
// matches, or an error when the request URL is unparseable.
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
