package confluence

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// debugTransport logs the request and response to out, redacting the
// Authorization header so the Bearer token never appears. It buffers the
// request and response bodies and restores them so logging is transparent to
// the rest of the stack.
type debugTransport struct {
	base http.RoundTripper
	out  io.Writer
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.logRequest(req)

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		_, _ = fmt.Fprintf(t.out, "< error: %v\n", err)
		return resp, err
	}

	t.logResponse(resp)
	return resp, nil
}

func (t *debugTransport) logRequest(req *http.Request) {
	_, _ = fmt.Fprintf(t.out, "> %s %s\n", req.Method, req.URL.String())
	writeHeaders(t.out, ">", req.Header)

	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err == nil {
			// Restore the body for the actual request.
			req.Body = io.NopCloser(bytes.NewReader(body))
			if len(body) > 0 {
				_, _ = fmt.Fprintf(t.out, "> body: %s\n", body)
			}
		}
	}
}

func (t *debugTransport) logResponse(resp *http.Response) {
	_, _ = fmt.Fprintf(t.out, "< %s\n", resp.Status)
	writeHeaders(t.out, "<", resp.Header)

	if resp.Body != nil {
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewReader(body))
			if len(body) > 0 {
				_, _ = fmt.Fprintf(t.out, "< body: %s\n", body)
			}
		}
	}
}

// writeHeaders prints headers in sorted order, redacting Authorization so the
// Bearer token value is never logged.
func writeHeaders(w io.Writer, prefix string, h http.Header) {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		value := strings.Join(h[k], ", ")
		if strings.EqualFold(k, "Authorization") {
			value = "Bearer ****"
		}
		_, _ = fmt.Fprintf(w, "%s %s: %s\n", prefix, k, value)
	}
}
