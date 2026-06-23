package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/addozhang/cfl/internal/auth"
	"github.com/addozhang/cfl/internal/confluenceurl"
	cflerrors "github.com/addozhang/cfl/internal/errors"
)

// aliasQualifiedIDRe matches the <alias>:<id> form: an alias name, a colon, and
// a purely numeric page id. Values containing a scheme, dot, or slash are URLs
// and never match here.
var aliasQualifiedIDRe = regexp.MustCompile(`^([a-zA-Z0-9_-]+):([0-9]+)$`)

// resolveRef parses a command argument into a confluenceurl.Ref, applying the
// bare-numeric-ID rule (D3): a bare id has no host, so the base URL and context
// path are taken from the credential store, and the form is valid only when
// exactly one instance is configured. It also resolves the <alias>:<id> form,
// which selects the instance unambiguously even with multiple configured.
func resolveRef(arg string, store *auth.Store) (confluenceurl.Ref, error) {
	// <alias>:<id> — only when there is no URL punctuation, so a URL like
	// https://host/... is never mistaken for it.
	if !strings.Contains(arg, "://") && !strings.Contains(arg, "/") && !strings.Contains(arg, ".") {
		if m := aliasQualifiedIDRe.FindStringSubmatch(arg); m != nil {
			alias, id := m[1], m[2]
			key, ok := store.ResolveAlias(alias)
			if !ok {
				return confluenceurl.Ref{}, &cflerrors.CFLError{
					Code:       cflerrors.CodeConfig,
					Message:    fmt.Sprintf("Unknown instance alias %q in %q. Run `cfl auth list` to see configured aliases.", alias, arg),
					Suggestion: "Add it with `cfl auth add <url> --alias " + alias + "`.",
				}
			}
			base, ctx := splitInstanceKey(key)
			return confluenceurl.Ref{BaseURL: base, ContextPath: ctx, PageID: id}, nil
		}
	}

	ref, err := confluenceurl.Parse(arg)
	if err != nil {
		return confluenceurl.Ref{}, cflerrors.WrapURLParse(arg, err)
	}
	if !ref.IsBareID {
		return ref, nil
	}

	// Bare numeric id: fill the base URL + context path from the single
	// configured instance, or reject when zero or multiple are configured.
	keys := store.List()
	switch len(keys) {
	case 0:
		return confluenceurl.Ref{}, &cflerrors.CFLError{
			Code:       cflerrors.CodeConfig,
			Message:    fmt.Sprintf("A bare page ID (%q) needs a full Confluence URL because no instance is configured.", arg),
			Suggestion: "Pass a full Confluence URL, or run `cfl auth add <url>` to configure a single instance.",
		}
	case 1:
		base, ctx := splitInstanceKey(keys[0])
		ref.BaseURL = base
		ref.ContextPath = ctx
		return ref, nil
	default:
		return confluenceurl.Ref{}, &cflerrors.CFLError{
			Code:       cflerrors.CodeConfig,
			Message:    fmt.Sprintf("A bare page ID (%q) is ambiguous because multiple instances are configured.", arg),
			Suggestion: "Pass a full Confluence URL, or use `<alias>:<id>` to pick the instance.",
		}
	}
}

// resolveInstance turns an --instance value into a Confluence instance URL: a
// value matching a configured alias is expanded to that instance's key;
// otherwise the value is treated as a URL. A value with no URL punctuation
// (no scheme, dot, or slash) that is not a known alias is rejected, because it
// is neither a usable URL nor a configured alias.
func resolveInstance(value string, store *auth.Store) (string, error) {
	looksLikeURL := strings.Contains(value, "://") || strings.Contains(value, ".") || strings.Contains(value, "/")
	if !looksLikeURL {
		if key, ok := store.ResolveAlias(value); ok {
			return key, nil
		}
		return "", &cflerrors.CFLError{
			Code:       cflerrors.CodeConfig,
			Message:    fmt.Sprintf("Unknown instance alias %q. Run `cfl auth list` to see configured aliases, or pass a full Confluence URL.", value),
			Suggestion: "Add an alias with `cfl auth add <url> --alias <name>`.",
		}
	}
	return value, nil
}

// requireCredential verifies a credential resolves for the given request URL,
// returning first-run onboarding guidance (naming the exact `cfl auth add`
// command) when none is configured.
func requireCredential(rawURL string, store *auth.Store) error {
	_, ok, err := store.Resolve(rawURL)
	if err != nil {
		return cflerrors.WrapURLParse(rawURL, err)
	}
	if !ok {
		key, kerr := auth.KeyFromURL(rawURL)
		if kerr != nil {
			key = rawURL
		}
		return &cflerrors.CFLError{
			Code:       cflerrors.CodeConfig,
			Message:    fmt.Sprintf("No credential configured for %s. Run `cfl auth add %s` to store a Personal Access Token.", key, key),
			Suggestion: "Create a Personal Access Token in your Confluence profile's Personal Access Tokens settings.",
		}
	}
	return nil
}

// refForInstance builds a confluenceurl.Ref addressing an instance base URL
// (scheme://host[:port][/contextpath]) with no page or space. Unlike
// resolveRef, it accepts a bare instance URL because space/list and page/create
// target the instance itself, not a specific page.
func refForInstance(instanceURL string) (confluenceurl.Ref, error) {
	key, err := auth.KeyFromURL(instanceURL)
	if err != nil {
		return confluenceurl.Ref{}, cflerrors.WrapURLParse(instanceURL, err)
	}
	base, ctx := splitInstanceKey(key)
	return confluenceurl.Ref{BaseURL: base, ContextPath: ctx}, nil
}

// splitInstanceKey decomposes a stored instance key
// (scheme://host[:port][/contextpath]) into its base URL (scheme://host[:port])
// and context path ("" when root-mounted).
func splitInstanceKey(key string) (baseURL, contextPath string) {
	idx := strings.Index(key, "://")
	if idx < 0 {
		return key, ""
	}
	rest := key[idx+3:]
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		return key[:idx+3] + rest[:slash], strings.TrimRight(rest[slash:], "/")
	}
	return key, ""
}
