package errors

import (
	stderrors "errors"
	"fmt"
	"strings"
	"testing"
)

func Test_CFLError_Error_includes_message(t *testing.T) {
	err := &CFLError{
		Code:       CodeNotFound,
		Message:    "Page not found: https://wiki.example.com/x",
		Suggestion: "Check the URL or page ID and try again.",
	}

	got := err.Error()
	want := "Page not found: https://wiki.example.com/x"
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func Test_CFLError_Unwrap_returns_cause(t *testing.T) {
	cause := stderrors.New("dial tcp: connection refused")
	err := &CFLError{
		Code:    CodeNetwork,
		Message: "Network error",
		Cause:   cause,
	}

	if !stderrors.Is(err, cause) {
		t.Fatalf("errors.Is(err, cause) = false, want true")
	}
}

func Test_CFLError_Unwrap_nil_cause(t *testing.T) {
	err := &CFLError{Code: CodeBadURL, Message: "bad url"}

	if unwrapped := stderrors.Unwrap(err); unwrapped != nil {
		t.Fatalf("Unwrap() = %v, want nil", unwrapped)
	}
}

// ExitCode policy (errors spec "Map exit codes"): nil -> 0; any cfl-level
// failure -> a value >= 10. Tests assert only the >= 10 boundary, never a
// specific value.
func Test_ExitCode_nil_is_zero(t *testing.T) {
	if got := ExitCode(nil); got != 0 {
		t.Fatalf("ExitCode(nil) = %d, want 0", got)
	}
}

func Test_ExitCode_CFLError_is_at_least_ten(t *testing.T) {
	codes := []Code{
		CodeBadURL, CodeAuth, CodeNotFound, CodeSpaceNotFound,
		CodeVersionConflict, CodeTimeout, CodeTLS, CodeMalformed,
		CodeNetwork, CodeConfig,
	}
	for _, c := range codes {
		err := &CFLError{Code: c, Message: "boom"}
		if got := ExitCode(err); got < 10 {
			t.Errorf("ExitCode(%v) = %d, want >= 10", c, got)
		}
	}
}

func Test_ExitCode_plain_error_is_at_least_ten(t *testing.T) {
	// A non-CFLError that still reaches the top layer must map to a failure.
	if got := ExitCode(stderrors.New("unexpected")); got < 10 {
		t.Fatalf("ExitCode(plain error) = %d, want >= 10", got)
	}
}

func Test_ExitCode_wrapped_CFLError_is_at_least_ten(t *testing.T) {
	inner := &CFLError{Code: CodeAuth, Message: "rejected"}
	wrapped := fmt.Errorf("calling whoami: %w", inner)

	// The CFLError must remain discoverable through wrapping.
	if _, ok := AsCFLError(wrapped); !ok {
		t.Fatalf("AsCFLError could not find the wrapped *CFLError")
	}
	if got := ExitCode(wrapped); got < 10 {
		t.Fatalf("ExitCode = %d, want >= 10", got)
	}
}

func Test_Code_String(t *testing.T) {
	tests := []struct {
		code Code
		want string
	}{
		{CodeBadURL, "bad_url"},
		{CodeAuth, "auth"},
		{CodeNotFound, "not_found"},
		{CodeSpaceNotFound, "space_not_found"},
		{CodeVersionConflict, "version_conflict"},
		{CodeTimeout, "timeout"},
		{CodeTLS, "tls"},
		{CodeMalformed, "malformed"},
		{CodeNetwork, "network"},
		{CodeConfig, "config"},
		{CodeUnknown, "unknown"},
	}
	for _, tt := range tests {
		if got := tt.code.String(); got != tt.want {
			t.Errorf("Code(%d).String() = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func Test_HTTPStatusError_Error(t *testing.T) {
	withStatus := &HTTPStatusError{StatusCode: 404, Status: "404 Not Found", URL: "https://wiki.example.com/x"}
	if got := withStatus.Error(); !strings.Contains(got, "404 Not Found") {
		t.Errorf("Error() = %q, should include the status text", got)
	}

	noStatus := &HTTPStatusError{StatusCode: 503, URL: "https://wiki.example.com/y"}
	if got := noStatus.Error(); !strings.Contains(got, "503") {
		t.Errorf("Error() = %q, should include the status code", got)
	}
}

func Test_Render_nil_is_noop(t *testing.T) {
	var buf strings.Builder
	Render(&buf, nil, false)
	if buf.Len() != 0 {
		t.Fatalf("Render(nil) wrote %q, want nothing", buf.String())
	}
}
