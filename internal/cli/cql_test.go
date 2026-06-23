package cli

import (
	"strings"
	"testing"
)

func Test_buildCQL_text_only(t *testing.T) {
	got := buildCQL("runbook", "", "page")
	// Default type page is always constrained; text is a quoted free-text match.
	if !strings.Contains(got, `type = "page"`) {
		t.Errorf("CQL %q should constrain type=page", got)
	}
	if !strings.Contains(got, `text ~ "runbook"`) {
		t.Errorf("CQL %q should match text ~ \"runbook\"", got)
	}
}

func Test_buildCQL_with_space(t *testing.T) {
	got := buildCQL("notes", "ENG", "page")
	if !strings.Contains(got, `space = "ENG"`) {
		t.Errorf("CQL %q should constrain space=ENG", got)
	}
	if !strings.Contains(got, " AND ") {
		t.Errorf("CQL %q should AND-join clauses", got)
	}
}

func Test_buildCQL_blogpost_type(t *testing.T) {
	got := buildCQL("x", "", "blogpost")
	if !strings.Contains(got, `type = "blogpost"`) {
		t.Errorf("CQL %q should constrain type=blogpost", got)
	}
}

func Test_buildCQL_no_text(t *testing.T) {
	// No positional text: only the constraints, no text clause.
	got := buildCQL("", "ENG", "page")
	if strings.Contains(got, "text ~") {
		t.Errorf("CQL %q should not include a text clause when no text given", got)
	}
	if !strings.Contains(got, `space = "ENG"`) || !strings.Contains(got, `type = "page"`) {
		t.Errorf("CQL %q should still constrain space and type", got)
	}
}

// Test_buildCQL_escapes_injection is the security-critical case: a term with
// quotes and CQL operators must be escaped into a single quoted literal, not
// allowed to alter the query structure.
func Test_buildCQL_escapes_injection(t *testing.T) {
	got := buildCQL(`x" OR space = ALL OR title ~ "y`, "", "page")

	// The dangerous quotes must be backslash-escaped inside the text literal,
	// so the embedded `OR space = ALL` cannot become a real CQL clause.
	if strings.Contains(got, `text ~ "x" OR space = ALL`) {
		t.Errorf("CQL injection not prevented: %q", got)
	}
	// The escaped form keeps the whole term as one quoted literal.
	if !strings.Contains(got, `\"`) {
		t.Errorf("embedded quotes should be backslash-escaped, got: %q", got)
	}
}

func Test_buildCQL_escapes_backslash(t *testing.T) {
	got := buildCQL(`a\b`, "", "page")
	if !strings.Contains(got, `a\\b`) {
		t.Errorf("backslash should be escaped, got: %q", got)
	}
}
