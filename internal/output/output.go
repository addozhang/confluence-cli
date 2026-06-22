// Package output renders cfl's self-owned schema values as YAML, JSON, or raw
// bytes. YAML is the default. For yaml and json it injects schemaVersion as the
// first key; raw passes the underlying Confluence bytes through verbatim.
//
// YAML is produced by routing the JSON encoding through sigs.k8s.io/yaml, so a
// single set of json struct tags drives both encodings and they are guaranteed
// structurally equivalent.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"sigs.k8s.io/yaml"
)

// SchemaVersion is the current schema major version, injected as the first key
// of every yaml/json response.
const SchemaVersion = "1"

// Format selects the output encoding.
type Format int

const (
	// FormatYAML is the default human-friendly encoding.
	FormatYAML Format = iota
	// FormatJSON is compact machine-readable JSON.
	FormatJSON
	// FormatRaw passes the underlying Confluence response bytes through verbatim.
	FormatRaw
)

// ParseFormat converts the -o flag value into a Format.
func ParseFormat(s string) (Format, error) {
	switch s {
	case "yaml", "yml":
		return FormatYAML, nil
	case "json":
		return FormatJSON, nil
	case "raw":
		return FormatRaw, nil
	default:
		return FormatYAML, fmt.Errorf("unknown output format %q (want yaml, json, or raw)", s)
	}
}

// RawBytes wraps a verbatim Confluence response body for FormatRaw rendering.
type RawBytes struct {
	b []byte
}

// Raw marks bytes as a verbatim payload to be emitted under FormatRaw.
func Raw(b []byte) RawBytes {
	return RawBytes{b: b}
}

// Write renders v to w in the given format. For FormatRaw, v must be a
// RawBytes; for yaml/json, v is any value with json struct tags.
func Write(w io.Writer, v any, format Format) error {
	switch format {
	case FormatRaw:
		raw, ok := v.(RawBytes)
		if !ok {
			return fmt.Errorf("raw output requires raw bytes, got %T", v)
		}
		_, err := w.Write(raw.b)
		return err

	case FormatJSON:
		payload, err := marshalWithSchemaVersion(v)
		if err != nil {
			return err
		}
		_, err = w.Write(append(payload, '\n'))
		return err

	case FormatYAML:
		// Build YAML with schemaVersion as the first line. JSONToYAML routes
		// through a map and sorts keys, which would bury schemaVersion; so we
		// emit the schemaVersion line ourselves and append the YAML body.
		body, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
		if err := assertJSONObject(body); err != nil {
			return err
		}
		yamlBody, err := yaml.JSONToYAML(body)
		if err != nil {
			return fmt.Errorf("encode yaml: %w", err)
		}
		out := fmt.Sprintf("schemaVersion: %q\n", SchemaVersion)
		if len(bytes.TrimSpace(yamlBody)) > 0 && string(bytes.TrimSpace(yamlBody)) != "{}" {
			out += string(yamlBody)
		}
		_, err = io.WriteString(w, out)
		return err

	default:
		return fmt.Errorf("unsupported output format")
	}
}

// assertJSONObject verifies b is a JSON object (so schemaVersion injection is
// meaningful).
func assertJSONObject(b []byte) error {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return fmt.Errorf("schema value must encode to a JSON object, got %s", trimmed)
	}
	return nil
}

// marshalWithSchemaVersion JSON-encodes v and injects schemaVersion as the
// first key, so the first key is always schemaVersion regardless of struct
// field order.
func marshalWithSchemaVersion(v any) ([]byte, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode json: %w", err)
	}
	if err := assertJSONObject(body); err != nil {
		return nil, err
	}

	trimmed := bytes.TrimSpace(body)
	prefix := fmt.Sprintf(`{"schemaVersion":%q`, SchemaVersion)
	inner := trimmed[1 : len(trimmed)-1] // drop the surrounding braces
	if len(bytes.TrimSpace(inner)) == 0 {
		return []byte(prefix + "}"), nil
	}
	return []byte(prefix + "," + string(inner) + "}"), nil
}
