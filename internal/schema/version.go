// Package schema defines cfl's self-owned output types and the pure functions
// that map raw Confluence REST responses into them. The types carry json (and,
// via sigs.k8s.io/yaml, yaml) tags using camelCase, matching docs/schema.md.
package schema

// VersionInfo is the output of `cfl version`. commit and date are null-capable
// because a plain build may not inject them.
type VersionInfo struct {
	Version string  `json:"version"`
	Commit  *string `json:"commit"`
	Date    *string `json:"date"`
}
