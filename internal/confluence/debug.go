package confluence

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
)

// debugTransport logs the request and response through log/slog, redacting the
// Authorization header so the Bearer token never appears. It buffers the
// request and response bodies and restores them so logging is transparent to
// the rest of the stack.
type debugTransport struct {
	base   http.RoundTripper
	logger *slog.Logger
}

// newDebugTransport builds a debugTransport that emits structured records to
// out via a slog text handler.
func newDebugTransport(base http.RoundTripper, out io.Writer) *debugTransport {
	handler := slog.NewTextHandler(out, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		// Drop the timestamp so debug output is stable and uncluttered; the
		// level attribute remains, marking these as slog records.
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	return &debugTransport{base: base, logger: slog.New(handler)}
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.logRequest(req)

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		t.logger.Debug("http response error", "error", err.Error())
		return resp, err
	}

	t.logResponse(resp)
	return resp, nil
}

func (t *debugTransport) logRequest(req *http.Request) {
	attrs := []any{
		"direction", "request",
		"method", req.Method,
		"url", req.URL.String(),
	}
	attrs = append(attrs, headerAttrs(req.Header)...)
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err == nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
			if len(body) > 0 {
				attrs = append(attrs, "body", string(body))
			}
		}
	}
	t.logger.Debug("http request", attrs...)
}

func (t *debugTransport) logResponse(resp *http.Response) {
	attrs := []any{
		"direction", "response",
		"status", resp.Status,
	}
	attrs = append(attrs, headerAttrs(resp.Header)...)
	if resp.Body != nil {
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewReader(body))
			if len(body) > 0 {
				attrs = append(attrs, "body", string(body))
			}
		}
	}
	t.logger.Debug("http response", attrs...)
}

// headerAttrs renders headers as slog attributes in sorted order, redacting
// Authorization so the Bearer token value is never logged.
func headerAttrs(h http.Header) []any {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	attrs := make([]any, 0, len(keys)*2)
	for _, k := range keys {
		value := strings.Join(h[k], ", ")
		if strings.EqualFold(k, "Authorization") {
			value = "Bearer ****"
		}
		attrs = append(attrs, "header."+k, value)
	}
	return attrs
}
