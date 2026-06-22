package schema

import "bytes"

// containsKey reports whether the encoded JSON contains the given object key
// anywhere (a quoted key followed by a colon). Used to assert that undocumented
// Confluence fields do not survive mapping, and that null-able fields are
// present.
func containsKey(encoded []byte, key string) bool {
	needle := []byte(`"` + key + `":`)
	return bytes.Contains(encoded, needle)
}

// containsValue reports whether the encoded JSON contains the given literal
// substring.
func containsValue(encoded []byte, sub string) bool {
	return bytes.Contains(encoded, []byte(sub))
}
