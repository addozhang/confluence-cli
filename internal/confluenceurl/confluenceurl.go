// Package confluenceurl parses the Confluence Server/DC URL shapes cfl accepts
// into a structured Ref. It never contacts the network: resolving a
// space-key-plus-title reference to a concrete page ID requires an API call and
// is the caller's responsibility, as is resolving a bare numeric ID against the
// configured instance.
package confluenceurl

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Ref is the structured reference every page/space command resolves its
// argument into. BaseURL is scheme+host with the default port stripped;
// ContextPath is the reverse-proxy mount prefix (leading slash, no trailing
// slash, empty for root). Exactly one of PageID or (SpaceKey[/Title]) is
// populated for a concrete shape. IsBareID marks the "12345" form, whose
// BaseURL/ContextPath must be filled in from the credential store by the caller.
type Ref struct {
	BaseURL     string
	ContextPath string
	PageID      string
	SpaceKey    string
	Title       string
	IsBareID    bool
}

// HostKey returns the credential-lookup key: BaseURL plus ContextPath. It is
// the input to the auth store's Resolve.
func (r Ref) HostKey() string {
	return r.BaseURL + r.ContextPath
}

var (
	numericRe = regexp.MustCompile(`^[0-9]+$`)
	// markerSegments delimit the context path from the Confluence-specific path.
	markerSegments = map[string]bool{
		"rest":    true,
		"display": true,
		"spaces":  true,
		"pages":   true,
	}
)

// Parse converts a Confluence URL or a bare numeric page ID into a Ref. It
// returns an error for any argument that is neither a recognized Confluence URL
// shape nor a bare numeric ID.
func Parse(arg string) (Ref, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return Ref{}, fmt.Errorf("empty argument")
	}

	// Bare numeric page ID: base URL is unknown here; the caller resolves it
	// against the credential store (valid only for a single configured instance).
	if numericRe.MatchString(arg) {
		return Ref{PageID: arg, IsBareID: true}, nil
	}

	u, err := url.Parse(arg)
	if err != nil {
		return Ref{}, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Ref{}, fmt.Errorf("URL must use http or https scheme")
	}
	if u.Hostname() == "" {
		return Ref{}, fmt.Errorf("URL has no host")
	}

	base := u.Scheme + "://" + normalizeHost(u)

	contextPath, segments := splitContextPath(u.Path)

	ref := Ref{BaseURL: base, ContextPath: contextPath}

	if len(segments) == 0 {
		return Ref{}, fmt.Errorf("URL does not point at a Confluence page or space")
	}

	switch segments[0] {
	case "pages":
		// .../pages/viewpage.action?pageId=12345
		id := u.Query().Get("pageId")
		if id == "" {
			return Ref{}, fmt.Errorf("pages URL is missing a pageId query parameter")
		}
		if !numericRe.MatchString(id) {
			return Ref{}, fmt.Errorf("pageId %q is not a valid numeric page ID", id)
		}
		ref.PageID = id
		return ref, nil

	case "rest":
		// .../rest/api/content/12345
		if len(segments) >= 4 && segments[1] == "api" && segments[2] == "content" {
			id := segments[3]
			if !numericRe.MatchString(id) {
				return Ref{}, fmt.Errorf("content id %q is not a valid numeric page ID", id)
			}
			ref.PageID = id
			return ref, nil
		}
		return Ref{}, fmt.Errorf("unsupported REST URL shape")

	case "display":
		// .../display/KEY            -> space
		// .../display/KEY/Title      -> page by title within space
		if len(segments) < 2 {
			return Ref{}, fmt.Errorf("display URL is missing a space key")
		}
		ref.SpaceKey = segments[1]
		if len(segments) >= 3 {
			ref.Title = decodeTitle(segments[2])
		}
		return ref, nil

	case "spaces":
		// .../spaces/KEY                         -> space
		// .../spaces/KEY/pages/ID[/Title]        -> page by id within space
		if len(segments) < 2 {
			return Ref{}, fmt.Errorf("spaces URL is missing a space key")
		}
		ref.SpaceKey = segments[1]
		// A /pages/{ID} continuation makes this a page reference; the ID is
		// authoritative (no title lookup needed). Title, if present, is for
		// display only.
		if len(segments) >= 4 && segments[2] == "pages" {
			id := segments[3]
			if !numericRe.MatchString(id) {
				return Ref{}, fmt.Errorf("page id %q is not a valid numeric page ID", id)
			}
			ref.PageID = id
			if len(segments) >= 5 {
				ref.Title = decodeTitle(segments[4])
			}
		}
		return ref, nil

	default:
		return Ref{}, fmt.Errorf("URL does not point at a Confluence page or space")
	}
}

// normalizeHost lowercases the hostname and strips the default port for the
// scheme, preserving any explicit non-default port.
func normalizeHost(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" {
		return host
	}
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		return host
	}
	return host + ":" + port
}

// splitContextPath separates the reverse-proxy context-path prefix (the
// segments before the first marker segment) from the Confluence-specific path
// segments. The returned context path has a leading slash and no trailing
// slash, or is empty for a root-mounted instance.
func splitContextPath(rawPath string) (contextPath string, segments []string) {
	trimmed := strings.Trim(rawPath, "/")
	if trimmed == "" {
		return "", nil
	}
	parts := strings.Split(trimmed, "/")

	markerIdx := -1
	for i, p := range parts {
		if markerSegments[p] {
			markerIdx = i
			break
		}
	}
	if markerIdx == -1 {
		// No marker: the whole path is a context path with no Confluence shape.
		return "/" + strings.Join(parts, "/"), nil
	}
	if markerIdx > 0 {
		contextPath = "/" + strings.Join(parts[:markerIdx], "/")
	}
	return contextPath, parts[markerIdx:]
}

// decodeTitle turns a URL path title segment into a human title. Confluence
// display URLs encode spaces as '+'; url.Parse already percent-decodes the
// segment, so we only need to translate '+' to space.
func decodeTitle(seg string) string {
	return strings.ReplaceAll(seg, "+", " ")
}
