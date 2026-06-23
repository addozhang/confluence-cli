# space Specification

## Purpose
TBD - created by archiving change init-cfl-confluence-cli. Update Purpose after archive.
## Requirements
### Requirement: List spaces with pagination

The system SHALL provide a `cfl space list` command that lists Confluence spaces. The command MUST expose `--limit` and `--start` flags that map directly onto the REST pagination parameters, returning a single page of results rather than auto-paginating the entire instance; this keeps each invocation bounded and predictable. When `--limit` and `--start` are omitted, the command MUST rely on the Confluence server defaults. The exact REST endpoint and pagination parameters are defined in `design.md` (D9 endpoint mapping). The structured response MUST begin with `schemaVersion: "1"` and use camelCase fields, exposing a `spaces` array in which each entry carries at least `key`, `name`, and `type` (e.g. `global` or `personal`), plus pagination metadata reflecting the `start`, `limit`, and `size` of the returned page. When the instance returns no spaces for the requested window, the command MUST return an empty `spaces` array (not null and not an error) and exit with code `0`.

#### Scenario: List spaces with defaults
- **WHEN** the user runs `cfl space list` against an instance that has spaces
- **THEN** the command lists spaces using the server's default pagination and returns a document with `schemaVersion: "1"` and a `spaces` array whose entries each carry `key`, `name`, and `type`

#### Scenario: List spaces with explicit pagination flags
- **WHEN** the user runs `cfl space list --limit 25 --start 50`
- **THEN** the command requests that single window (limit 25, start 50) and returns only that page of results, with pagination metadata reflecting `start: 50` and `limit: 25`

#### Scenario: Empty result
- **WHEN** the user runs `cfl space list --start 1000` against an instance that has no spaces in that window
- **THEN** the command returns a document with `schemaVersion: "1"` and an empty `spaces` array, exiting with code `0`

### Requirement: Read a single space by key

The system SHALL provide a `cfl space get <key>` command that reads one Confluence space by its key. The read MUST request the space's expanded plain-text description and homepage. The exact REST endpoint and `expand` values are defined in `design.md` (D9 endpoint mapping). The structured response MUST begin with `schemaVersion: "1"` and use camelCase fields, exposing at least `key`, `name`, `type`, and `description` (the expanded plain-text description value, or `null` when the space has no description). When the space key does not exist, the command MUST exit with a non-zero code and print a translated `Space not found` error suggesting the user list spaces with `cfl space list`.

#### Scenario: Get an existing space
- **WHEN** the user runs `cfl space get ENG` against an instance that has a space with key `ENG`
- **THEN** the command reads space `ENG` (with its description and homepage expanded) and returns a document with `schemaVersion: "1"`, `key: "ENG"`, the space `name`, `type`, and `description`

#### Scenario: Space without a description
- **WHEN** the user runs `cfl space get OPS` against a space that has no description set
- **THEN** the response carries `key: "OPS"`, `name`, `type`, and `description: null`

#### Scenario: Space not found
- **WHEN** the user runs `cfl space get NOPE` and the space does not exist
- **THEN** the command exits with a non-zero code and prints a translated `Space not found` error suggesting the user run `cfl space list` to see available space keys

