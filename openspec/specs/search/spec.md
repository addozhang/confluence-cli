# search Specification

## Purpose
TBD - created by archiving change add-search-aliases-url-shapes. Update Purpose after archive.
## Requirements
### Requirement: Search content by keyword with friendly flags

The system SHALL provide a `cfl search <text>` command that searches Confluence content at a resolved instance. The command MUST compile its inputs into a Confluence CQL query and issue the search against the Server/DC search REST endpoint (defined in `design.md` D13). This friendly-flag compilation applies only when `--cql` is not supplied; `--cql` has the highest precedence and overrides these inputs (see the `--cql` requirement). The positional `<text>` MUST be matched as a free-text query, and the command MUST expose `--space KEY` (restrict to a space), `--type page|blogpost` (restrict to a content type, defaulting to `page`), and `--limit`/`--start` flags that map onto the REST pagination parameters for a single bounded page. The instance is selected by `--instance <url-or-alias>`, or, when exactly one instance is configured, defaults to it. The positional text MUST be CQL-escaped so that spaces, quotes, or CQL operators in the term cannot alter the query structure. The structured response MUST begin with `schemaVersion: "1"` and use camelCase fields, exposing a `results` array in which each entry carries at least `id`, `title`, `type`, `spaceKey`, and `url`, plus pagination metadata reflecting `start`, `limit`, and `size`. When no content matches, the command MUST return an empty `results` array (not null and not an error) and exit with code `0`.

#### Scenario: Keyword search within a space
- **WHEN** the user runs `cfl search "release notes" --space ENG --instance https://wiki.example.com`
- **THEN** the command builds a CQL query restricting to space `ENG`, type `page`, and text matching `release notes`, issues the search, and returns a document with `schemaVersion: "1"` and a `results` array whose entries each carry `id`, `title`, `type`, `spaceKey`, and `url`

#### Scenario: Default content type is page
- **WHEN** the user runs `cfl search "runbook" --instance https://wiki.example.com` without `--type`
- **THEN** the built CQL restricts the search to `type = page`, and only page results are returned

#### Scenario: Pagination flags map to the request
- **WHEN** the user runs `cfl search "doc" --limit 10 --start 20 --instance https://wiki.example.com`
- **THEN** the search request carries `limit=10` and `start=20`, and the response pagination metadata reflects `start: 20` and `limit: 10`

#### Scenario: Search term is escaped, not injected
- **WHEN** the user runs `cfl search "title~\"x\" OR space=ALL" --instance https://wiki.example.com`
- **THEN** the entire argument is treated as a single quoted free-text term in the built CQL (escaped), not interpreted as CQL operators, so the query cannot be widened or altered by the term's contents

#### Scenario: Empty search result
- **WHEN** the user runs `cfl search "nonexistentxyz" --instance https://wiki.example.com` and nothing matches
- **THEN** the command returns a document with `schemaVersion: "1"` and an empty `results` array, exiting with code `0`

### Requirement: Pass a raw CQL query with `--cql` (highest precedence)

The system SHALL provide a `--cql <query>` flag on `cfl search` that sends the supplied CQL string to the search endpoint verbatim, bypassing the flag-to-CQL builder. `--cql` has the highest precedence: when `--cql` is present, the command MUST use it as the complete query and MUST ignore the positional `<text>`, `--space`, and `--type` inputs rather than failing — `--cql` overrides them. The system MUST NOT validate CQL grammar client-side; a malformed query MUST surface as the server's translated error. The `--limit`/`--start` flags MUST still apply with `--cql`. When friendly inputs are silently overridden, the system SHOULD note on stderr that `--cql` took precedence (so the override is visible), while keeping stdout limited to the result payload.

#### Scenario: Raw CQL passthrough
- **WHEN** the user runs `cfl search --cql 'space = ENG AND title ~ "runbook" AND created > now("-7d")' --instance https://wiki.example.com`
- **THEN** the command sends that CQL string unchanged to the search endpoint and returns the matching `results`

#### Scenario: --cql overrides friendly inputs
- **WHEN** the user runs `cfl search "text" --space OPS --type blogpost --cql 'type = page AND space = ENG' --instance https://wiki.example.com`
- **THEN** the command runs the `--cql` query (`type = page AND space = ENG`) and ignores the positional text, `--space`, and `--type`, optionally noting on stderr that `--cql` took precedence; stdout carries only the result payload

#### Scenario: --cql still honors pagination
- **WHEN** the user runs `cfl search --cql 'type = page' --limit 5 --start 10 --instance https://wiki.example.com`
- **THEN** the search request carries the raw CQL plus `limit=5` and `start=10`

#### Scenario: Malformed CQL surfaces the server error
- **WHEN** the user runs `cfl search --cql 'this is not valid cql (' --instance https://wiki.example.com` and Confluence rejects the query
- **THEN** the command exits with a non-zero code and prints a translated error indicating the search request was rejected, suggesting `--debug` to inspect the exchange

### Requirement: Resolve the search instance by URL or alias

The system SHALL resolve the `--instance` value for `cfl search` to a configured instance using the same alias-or-URL resolution as every other instance-targeting command: an `--instance` value matching a stored alias is expanded to that instance's base URL and context path; otherwise it is treated as a Confluence URL. When no `--instance` is given and exactly one instance is configured, that instance is used; when none or multiple are configured and no `--instance` is given, the command MUST fail with an actionable error. When the resolved instance has no stored credential, the command MUST surface the standard first-run onboarding guidance.

#### Scenario: Search via an instance alias
- **WHEN** an instance is configured with alias `prod` and the user runs `cfl search "runbook" --instance prod`
- **THEN** the command resolves `prod` to the configured instance's base URL and runs the search against it

#### Scenario: Ambiguous instance without --instance
- **WHEN** two instances are configured and the user runs `cfl search "runbook"` without `--instance`
- **THEN** the command exits with a non-zero code instructing the user to pass `--instance <url-or-alias>`

