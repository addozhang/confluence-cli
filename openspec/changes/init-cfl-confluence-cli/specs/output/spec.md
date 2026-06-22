# output

## ADDED Requirements

### Requirement: Render output as YAML by default

The system SHALL render all command output as YAML by default. The YAML output MUST follow the `cfl` self-owned schema documented in `docs/schema.md`, not the raw Confluence REST API field names.

#### Scenario: Default output is YAML
- **WHEN** the user runs any read command without specifying `-o`
- **THEN** the response is printed to stdout as a YAML document

#### Scenario: Field names follow schema conventions
- **WHEN** any command renders YAML output
- **THEN** field names use camelCase, timestamps use the `Utc` suffix on ISO 8601 UTC strings, and enums are uppercase string constants

### Requirement: Support JSON output via `-o json`

The system SHALL support a `-o json` / `--output json` flag that renders the same self-owned schema as compact JSON to stdout. The JSON encoding and the YAML encoding of the same value MUST be equivalent in structure, field names, and types.

#### Scenario: JSON output for a page command
- **WHEN** the user runs `cfl page get <url> -o json`
- **THEN** the response is printed as compact JSON whose structure matches the YAML output exactly, with the same field names, types, and `schemaVersion`

#### Scenario: JSON and YAML encodings are equivalent
- **WHEN** the user runs the same command once with `-o yaml` and once with `-o json`
- **THEN** both outputs decode to the same logical value, with identical keys, nesting, and scalar types

### Requirement: Support raw API passthrough via `-o raw`

The system SHALL support a `-o raw` / `--output raw` flag that bypasses the self-owned schema layer and prints the underlying Confluence REST API response body verbatim. The `-o raw` flag is the escape hatch for fields that `cfl`'s schema does not expose.

#### Scenario: Raw output for a page command
- **WHEN** the user runs `cfl page get <url> -o raw`
- **THEN** the response printed to stdout is the original Confluence REST API response body (typically JSON), byte-for-byte, without re-rendering and without `schemaVersion` injection

#### Scenario: Raw output is not reformatted
- **WHEN** the user runs any command with `-o raw`
- **THEN** the system does not parse, reorder, or re-encode the Confluence response body, and emits exactly the bytes received from the host

### Requirement: Include `schemaVersion` in every schema-rendered response

The system SHALL include a top-level `schemaVersion` field as the first key in every YAML or JSON response. The field value MUST be a string identifying the current schema major version (initial value: `"1"`). The field MUST NOT be present when `-o raw` is used.

#### Scenario: Schema version in YAML output
- **WHEN** the user runs any command with default YAML output
- **THEN** the response begins with `schemaVersion: "1"` on the first non-comment line

#### Scenario: Schema version is the first key in JSON output
- **WHEN** the user runs any command with `-o json`
- **THEN** `schemaVersion` is the first key of the top-level JSON object and its value is the string `"1"`

#### Scenario: Schema version omitted from raw output
- **WHEN** the user runs any command with `-o raw`
- **THEN** the response does not contain a `schemaVersion` field injected by `cfl`

### Requirement: Use explicit `null` for missing values

The system SHALL emit `null` (not omit the field, not emit an empty string) for any schema field whose value is unavailable for the current response. Consumers MUST be able to assume that a field defined in `docs/schema.md` is always present on every response of the relevant command.

#### Scenario: Null ancestor on a top-level page
- **WHEN** `cfl page get <url>` returns a page that has no parent page
- **THEN** the rendered output contains the literal field `parentId: null` rather than omitting `parentId` or using an empty string

#### Scenario: Defined fields are always present
- **WHEN** a command renders a value for which some schema-defined fields have no underlying data
- **THEN** every field documented for that command in `docs/schema.md` appears in the output, with `null` standing in for any unavailable value

### Requirement: Define an authoritative per-command output field set

The system SHALL define, for every command that produces structured output, an exact field set in `docs/schema.md`, and the rendered output of that command MUST contain exactly that field set (no undocumented fields, no missing documented fields). `docs/schema.md` is the authoritative contract; a command MUST NOT emit a structured field that is not documented there, and MUST emit every field documented there (using `null` for unavailable values, per the explicit-null requirement). Each documented field MUST carry its type and a `stable`/`experimental` tag. The minimum field set per command is:

- **`page get`** (and the `page` object returned by create/update): `schemaVersion`, `id`, `title`, `spaceKey`, `version`, `parentId`, `body`, `ancestors[]` (each `{id, title}`), `url`.
- **`page create` / `page update`**: `schemaVersion`, `id`, `title`, `spaceKey`, `version`, `url` (the `body` and `ancestors` fields MAY be omitted from create/update output and, when omitted, MUST be documented as not-returned for those commands rather than emitted as `null`).
- **`page children`**: `schemaVersion`, `children[]` (each `{id, title, version}`).
- **`page delete`**: `schemaVersion`, `id`, `status` (confirming the page was moved to trash).
- **`space list`**: `schemaVersion`, `spaces[]` (each `{key, name, type}`), `start`, `limit`, `size`.
- **`space get`**: `schemaVersion`, `key`, `name`, `type`, `description`.
- **`auth list`**: `schemaVersion`, `instances[]` (each a configured key string); never any token.
- **`auth whoami`**: `schemaVersion`, `host`, plus the identity fields the instance returns (e.g. `username`, `displayName`); never the token.
- **`version`**: `schemaVersion`, `version`, `commit`, `date`.

The presence of `-o raw` does not satisfy this requirement: the field-set contract governs the schema-rendered (`yaml`/`json`) output, which is what scripts depend on. Documenting a field in `docs/schema.md` is a prerequisite to emitting it; the documentation and the implementation land in the same change.

#### Scenario: Output contains exactly the documented field set
- **WHEN** the user runs `cfl page get <url> -o json` for a page that has no parent
- **THEN** the JSON object contains every field documented for `page get` in `docs/schema.md` — including `parentId` (as `null`), `body`, `ancestors`, and `url` — and contains no field that is absent from `docs/schema.md`

#### Scenario: Undocumented Confluence fields are not leaked
- **WHEN** a Confluence response includes fields that `cfl`'s schema does not expose (e.g. `_links`, `_expandable`, `extensions`)
- **THEN** the schema-rendered (`yaml`/`json`) output omits those fields entirely, surfacing them only via `-o raw`

#### Scenario: Every command's fields are documented before they ship
- **WHEN** a maintainer reviews `docs/schema.md` for any command listed above
- **THEN** every field that command emits in `yaml`/`json` output appears in `docs/schema.md` with its type and a `stable`/`experimental` tag, and no emitted field is missing from the document



The system SHALL maintain `docs/schema.md` as the authoritative output contract. Every field defined by the schema MUST be tagged either `stable` or `experimental`. Only `stable` fields carry the version-compatibility promise: they will not be removed, renamed, or have their type changed without bumping `schemaVersion`. `experimental` fields MAY change without notice within a major schema version.

#### Scenario: Schema document exists and tags every field
- **WHEN** a maintainer inspects `docs/schema.md`
- **THEN** every field referenced by any command's output appears in `docs/schema.md` with an explicit `stable` or `experimental` tag and a one-line description
