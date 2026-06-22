package confluence

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	cflerrors "github.com/addozhang/cfl/internal/errors"
)

// Client issues HTTP requests to a Confluence Server/DC instance and returns
// raw response bytes. It performs no JSON-to-schema mapping (that is the schema
// package's job) and sets no auth headers (the transport does). The exact
// endpoints follow the D9 mapping in the change's design.md.
type Client struct {
	http        *http.Client
	baseURL     string
	contextPath string
}

// NewClient builds a Client. baseURL is scheme+host (no trailing slash);
// contextPath is the reverse-proxy mount prefix ("" for root-mounted).
func NewClient(httpClient *http.Client, baseURL, contextPath string) *Client {
	return &Client{http: httpClient, baseURL: baseURL, contextPath: contextPath}
}

// restURL builds an absolute REST URL: base + contextPath + /rest/api + path,
// with the given query values.
func (c *Client) restURL(path string, q url.Values) string {
	u := c.baseURL + c.contextPath + "/rest/api" + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

// GetPage reads a page by id with body, version, space, and ancestors expanded.
func (c *Client) GetPage(ctx context.Context, id string) ([]byte, error) {
	q := url.Values{"expand": {"body.storage,version,space,ancestors"}}
	return c.do(ctx, http.MethodGet, c.restURL("/content/"+id, q), nil)
}

// ReadVersion reads only the current version of a page (used before an update).
func (c *Client) ReadVersion(ctx context.Context, id string) ([]byte, error) {
	q := url.Values{"expand": {"version"}}
	return c.do(ctx, http.MethodGet, c.restURL("/content/"+id, q), nil)
}

// LookupPageByTitle resolves a page by space key and title (for display URLs).
func (c *Client) LookupPageByTitle(ctx context.Context, spaceKey, title string) ([]byte, error) {
	q := url.Values{
		"spaceKey": {spaceKey},
		"title":    {title},
		"expand":   {"body.storage,version"},
	}
	return c.do(ctx, http.MethodGet, c.restURL("/content", q), nil)
}

// CreatePageInput is the input to CreatePage. ParentID is optional; when empty,
// the page is created at the top level (no ancestors).
type CreatePageInput struct {
	SpaceKey string
	Title    string
	Body     string
	ParentID string
}

// CreatePage creates a new page and returns the raw create response.
func (c *Client) CreatePage(ctx context.Context, in CreatePageInput) ([]byte, error) {
	payload := map[string]any{
		"type":  "page",
		"title": in.Title,
		"space": map[string]any{"key": in.SpaceKey},
		"body": map[string]any{
			"storage": map[string]any{
				"value":          in.Body,
				"representation": "storage",
			},
		},
	}
	if in.ParentID != "" {
		payload["ancestors"] = []map[string]any{{"id": in.ParentID}}
	}
	return c.doJSON(ctx, http.MethodPost, c.restURL("/content", nil), payload)
}

// UpdatePageInput is the input to UpdatePage. NewVersion must be the current
// version plus one; the caller (the page-update command) reads the current
// version first and computes this — the client never guesses it.
type UpdatePageInput struct {
	ID         string
	Title      string
	Body       string
	NewVersion int
}

// UpdatePage submits a version-aware update and returns the raw response.
func (c *Client) UpdatePage(ctx context.Context, in UpdatePageInput) ([]byte, error) {
	payload := map[string]any{
		"type":    "page",
		"title":   in.Title,
		"version": map[string]any{"number": in.NewVersion},
		"body": map[string]any{
			"storage": map[string]any{
				"value":          in.Body,
				"representation": "storage",
			},
		},
	}
	return c.doJSON(ctx, http.MethodPut, c.restURL("/content/"+in.ID, nil), payload)
}

// DeletePage moves a page to the Confluence trash.
func (c *Client) DeletePage(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, c.restURL("/content/"+id, nil), nil)
	return err
}

// GetChildren lists the direct child pages of a page.
func (c *Client) GetChildren(ctx context.Context, id string) ([]byte, error) {
	q := url.Values{"expand": {"version"}}
	return c.do(ctx, http.MethodGet, c.restURL("/content/"+id+"/child/page", q), nil)
}

// ListSpaces lists spaces for the given pagination window. A zero limit or
// start is omitted, relying on the Confluence server defaults.
func (c *Client) ListSpaces(ctx context.Context, limit, start int) ([]byte, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if start > 0 {
		q.Set("start", strconv.Itoa(start))
	}
	return c.do(ctx, http.MethodGet, c.restURL("/space", q), nil)
}

// GetSpace reads a single space by key with its description and homepage
// expanded.
func (c *Client) GetSpace(ctx context.Context, key string) ([]byte, error) {
	q := url.Values{"expand": {"description.plain,homepage"}}
	return c.do(ctx, http.MethodGet, c.restURL("/space/"+key, q), nil)
}

// WhoAmI reads the current user's identity (used to verify a stored token).
func (c *Client) WhoAmI(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, c.restURL("/user/current", nil), nil)
}

// doJSON marshals payload as the request body with a JSON content type.
func (c *Client) doJSON(ctx context.Context, method, fullURL string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode request body: %w", err)
	}
	return c.do(ctx, method, fullURL, body)
}

// do issues a request and returns the response body. Non-2xx responses are
// returned as *cflerrors.HTTPStatusError so the CLI layer can classify them.
func (c *Client) do(ctx context.Context, method, fullURL string, body []byte) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, rdr)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, fullURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &cflerrors.HTTPStatusError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       respBody,
			URL:        fullURL,
		}
	}
	return respBody, nil
}

// asHTTPStatusError reports whether err is, or wraps, an *HTTPStatusError.
func asHTTPStatusError(err error, target **cflerrors.HTTPStatusError) bool {
	return stderrors.As(err, target)
}
