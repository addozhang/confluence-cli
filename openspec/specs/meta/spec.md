# meta Specification

## Purpose
TBD - created by archiving change init-cfl-confluence-cli. Update Purpose after archive.
## Requirements
### Requirement: Report the build version

The system SHALL provide a `cfl version` command that prints the tool's build version information to stdout and exits with code `0`. The command MUST NOT contact any Confluence instance, MUST NOT require a configured credential, and MUST NOT read or write the credentials file. The reported information MUST include at least the semantic version string; it SHOULD also include the build's commit revision and build date when those were injected at build time, and MUST fall back to a recognizable placeholder (e.g. `dev`) when the binary was built without version injection.

When rendered through the structured output formats (`-o yaml` / `-o json`), the response MUST begin with `schemaVersion: "1"` and expose at least a `version` field, with `commit` and `date` fields present (using `null` when unavailable). Because this command is informational and offline, it MUST succeed identically whether or not `~/.config/cfl/credentials` exists.

#### Scenario: Print the version of a release build

- **WHEN** the user runs `cfl version` on a binary built with an injected version `v0.1.0`
- **THEN** the command prints version information including `v0.1.0` to stdout and exits with code `0`, without contacting any Confluence instance or reading the credentials file

#### Scenario: Version of a build without injected metadata

- **WHEN** the user runs `cfl version` on a binary built without version injection (e.g. a plain `go build`)
- **THEN** the command prints a recognizable placeholder version (e.g. `dev`) and exits with code `0`, rather than failing or printing an empty value

#### Scenario: Structured version output

- **WHEN** the user runs `cfl version -o json`
- **THEN** the output is a JSON object whose first key is `schemaVersion` with value `"1"`, carrying a `version` field plus `commit` and `date` fields (each either a string or `null`), and the command exits with code `0`

#### Scenario: Version command needs no credentials

- **WHEN** the user runs `cfl version` with no credentials file present at `~/.config/cfl/credentials`
- **THEN** the command succeeds and prints version information, never prompting for or requiring a Personal Access Token

