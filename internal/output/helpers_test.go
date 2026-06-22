package output

import (
	"encoding/json"
	"testing"

	"sigs.k8s.io/yaml"
)

func decodeJSONToMap(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return m
}

func decodeYAMLToMap(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := yaml.Unmarshal(b, &m); err != nil {
		t.Fatalf("decode yaml: %v", err)
	}
	return m
}
