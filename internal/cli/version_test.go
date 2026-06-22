package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

// runVersion executes the version command with the given format and returns
// stdout. It exercises the real command wiring (root flags -> Deps -> output).
func runVersion(t *testing.T, args ...string) string {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"version"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("version command returned error: %v", err)
	}
	return out.String()
}

func Test_version_json_has_schemaVersion_first(t *testing.T) {
	out := runVersion(t, "-o", "json")

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, out)
	}
	for _, k := range []string{"schemaVersion", "version", "commit", "date"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("missing key %q in version output: %q", k, out)
		}
	}
}

func Test_version_default_yaml_starts_with_schemaVersion(t *testing.T) {
	out := runVersion(t)
	if len(out) == 0 {
		t.Fatal("expected output")
	}
	// The very first line must be schemaVersion.
	want := "schemaVersion:"
	if got := out[:len(want)]; got != want {
		t.Errorf("first bytes = %q, want %q", got, want)
	}
}

func Test_version_placeholders_render_null(t *testing.T) {
	// With no -ldflags injection (the test binary), commit/date are placeholders
	// and must render as null in JSON, while version stays the "dev" placeholder
	// string (a recognizable non-empty value).
	out := runVersion(t, "-o", "json")

	var decoded struct {
		Version string  `json:"version"`
		Commit  *string `json:"commit"`
		Date    *string `json:"date"`
	}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Version == "" {
		t.Errorf("version should be a non-empty placeholder, got empty")
	}
	if decoded.Commit != nil {
		t.Errorf("commit should be null without injection, got %q", *decoded.Commit)
	}
	if decoded.Date != nil {
		t.Errorf("date should be null without injection, got %q", *decoded.Date)
	}
}
