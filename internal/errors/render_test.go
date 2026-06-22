package errors

import (
	stderrors "errors"
	"strings"
	"testing"
)

func Test_Render_prints_message_then_suggestion(t *testing.T) {
	err := &CFLError{
		Code:       CodeNotFound,
		Message:    "Page not found: https://wiki.example.com/x",
		Suggestion: "Check the URL or page ID and try again.",
	}

	var buf strings.Builder
	Render(&buf, err, false)
	out := buf.String()

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), out)
	}
	if lines[0] != "Error: Page not found: https://wiki.example.com/x" {
		t.Errorf("line 1 = %q, want 'Error: <message>'", lines[0])
	}
	if !strings.Contains(lines[1], "Check the URL") {
		t.Errorf("line 2 = %q, want the suggestion", lines[1])
	}
}

func Test_Render_hides_cause_without_debug(t *testing.T) {
	err := &CFLError{
		Code:       CodeNetwork,
		Message:    "Network error contacting https://wiki.example.com",
		Suggestion: "Check that the host is reachable.",
		Cause:      stderrors.New("dial tcp 10.0.0.1:443: connect: connection refused"),
	}

	var buf strings.Builder
	Render(&buf, err, false)
	out := buf.String()

	if strings.Contains(out, "connection refused") {
		t.Errorf("non-debug output leaked the cause: %q", out)
	}
}

func Test_Render_shows_cause_with_debug(t *testing.T) {
	err := &CFLError{
		Code:       CodeNetwork,
		Message:    "Network error contacting https://wiki.example.com",
		Suggestion: "Check that the host is reachable.",
		Cause:      stderrors.New("dial tcp 10.0.0.1:443: connect: connection refused"),
	}

	var buf strings.Builder
	Render(&buf, err, true)
	out := buf.String()

	if !strings.Contains(out, "connection refused") {
		t.Errorf("debug output should include the cause, got: %q", out)
	}
}

func Test_Render_plain_error(t *testing.T) {
	// A non-CFLError still renders an Error: line so the user is never left
	// without feedback.
	var buf strings.Builder
	Render(&buf, stderrors.New("unexpected boom"), false)
	out := buf.String()

	if !strings.HasPrefix(out, "Error: ") {
		t.Errorf("plain error should render an 'Error: ' line, got: %q", out)
	}
}
