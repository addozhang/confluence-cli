package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type sample struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func Test_Write_yaml_is_default_and_injects_schemaVersion(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, sample{ID: "1", Title: "Hi"}, FormatYAML); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	out := buf.String()

	// schemaVersion must be the first non-empty line.
	firstLine := strings.SplitN(strings.TrimLeft(out, "\n"), "\n", 2)[0]
	if !strings.HasPrefix(firstLine, "schemaVersion:") {
		t.Errorf("first line = %q, want it to start with schemaVersion:", firstLine)
	}
	if !strings.Contains(out, `"1"`) && !strings.Contains(out, "'1'") && !strings.Contains(out, "schemaVersion: \"1\"") {
		t.Errorf("output should carry schemaVersion value \"1\": %q", out)
	}
	if !strings.Contains(out, "id: \"1\"") && !strings.Contains(out, "id: 1") {
		t.Errorf("output should contain the id field: %q", out)
	}
}

func Test_Write_json_is_compact_with_schemaVersion_first(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, sample{ID: "1", Title: "Hi"}, FormatJSON); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	out := strings.TrimSpace(buf.String())

	if !strings.HasPrefix(out, `{"schemaVersion":"1"`) {
		t.Errorf("JSON should begin with schemaVersion as the first key, got: %q", out)
	}

	// Must be valid JSON and decode to the expected keys.
	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, out)
	}
	if decoded["schemaVersion"] != "1" {
		t.Errorf("schemaVersion = %v, want \"1\"", decoded["schemaVersion"])
	}
	if decoded["id"] != "1" || decoded["title"] != "Hi" {
		t.Errorf("decoded payload missing fields: %v", decoded)
	}
}

func Test_Write_raw_passthrough_no_schemaVersion(t *testing.T) {
	raw := []byte(`{"results":[],"_links":{"self":"x"}}`)
	var buf bytes.Buffer
	if err := Write(&buf, Raw(raw), FormatRaw); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "schemaVersion") {
		t.Errorf("raw output must not inject schemaVersion: %q", out)
	}
	if !strings.Contains(out, `"_links"`) {
		t.Errorf("raw output should pass bytes through verbatim: %q", out)
	}
}

// Test_yaml_and_json_equivalent asserts the spec guarantee: YAML and JSON of
// the same value decode to the same logical structure.
func Test_yaml_and_json_equivalent(t *testing.T) {
	v := sample{ID: "42", Title: "Runbook"}

	var yb, jb bytes.Buffer
	if err := Write(&yb, v, FormatYAML); err != nil {
		t.Fatalf("yaml write: %v", err)
	}
	if err := Write(&jb, v, FormatJSON); err != nil {
		t.Fatalf("json write: %v", err)
	}

	yDecoded := decodeYAMLToMap(t, yb.Bytes())
	jDecoded := decodeJSONToMap(t, jb.Bytes())

	if len(yDecoded) != len(jDecoded) {
		t.Fatalf("key count differs: yaml=%v json=%v", yDecoded, jDecoded)
	}
	for k, jv := range jDecoded {
		if yv, ok := yDecoded[k]; !ok || yv != jv {
			t.Errorf("key %q: yaml=%v json=%v", k, yDecoded[k], jv)
		}
	}
}

func Test_ParseFormat(t *testing.T) {
	tests := map[string]Format{
		"yaml": FormatYAML,
		"json": FormatJSON,
		"raw":  FormatRaw,
	}
	for in, want := range tests {
		got, err := ParseFormat(in)
		if err != nil {
			t.Errorf("ParseFormat(%q) error: %v", in, err)
		}
		if got != want {
			t.Errorf("ParseFormat(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseFormat("xml"); err == nil {
		t.Errorf("ParseFormat(\"xml\") should error")
	}
}

func Test_Write_raw_requires_RawBytes(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, sample{ID: "1"}, FormatRaw); err == nil {
		t.Errorf("FormatRaw with a non-RawBytes value should error")
	}
}

func Test_Write_non_object_value_errors(t *testing.T) {
	var buf bytes.Buffer
	// A bare slice does not encode to a JSON object, so schemaVersion injection
	// is not meaningful and must error rather than emit malformed output.
	if err := Write(&buf, []string{"a", "b"}, FormatJSON); err == nil {
		t.Errorf("non-object value should error under FormatJSON")
	}
	if err := Write(&buf, []string{"a", "b"}, FormatYAML); err == nil {
		t.Errorf("non-object value should error under FormatYAML")
	}
}

func Test_Write_empty_object(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, struct{}{}, FormatJSON); err != nil {
		t.Fatalf("empty struct should encode: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out != `{"schemaVersion":"1"}` {
		t.Errorf("empty object json = %q, want just schemaVersion", out)
	}
}
