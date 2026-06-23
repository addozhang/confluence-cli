# errors Specification

## Purpose
TBD - created by archiving change init-cfl-confluence-cli. Update Purpose after archive.
## Requirements
### Requirement: Translate API errors into human-readable messages with next-step suggestions

The system SHALL translate Confluence REST API errors into human-readable error messages printed to stderr. Every translated error MUST include a description of what happened and a suggested next action (a specific `cfl` command, a flag, or a verification step). The system MUST NOT print raw Confluence HTML error pages or Java stack traces to the user when `--debug` is not in effect.

#### Scenario: API token rejected
- **WHEN** any command receives HTTP 401 or 403 from Confluence on a request that should be authenticated
- **THEN** the command exits with a code `>= 10` and prints an error of the form: "API token rejected by `<host>`. Run `cfl auth add <host>` to refresh."

#### Scenario: Page not found
- **WHEN** any command receives HTTP 404 against a Confluence content URL
- **THEN** the command exits with a code `>= 10` and prints an error of the form: "Page not found: `<url>`. Check the URL or page ID and try again."

#### Scenario: Space not found
- **WHEN** any command receives HTTP 404 against a Confluence space URL or space key
- **THEN** the command exits with a code `>= 10` and prints an error of the form: "Space not found: `<key>`. List available spaces with `cfl space list`."

#### Scenario: Version conflict on update
- **WHEN** `cfl page update` receives HTTP 409 because the page changed since it was last read
- **THEN** the command exits with a code `>= 10` and prints an error of the form: "Update rejected: page `<id>` changed since you last read it (version conflict). Re-run `cfl page get` and retry the update."

#### Scenario: Network timeout
- **WHEN** any command's request to Confluence exceeds the configured timeout
- **THEN** the command exits with a code `>= 10` and prints an error of the form: "Timed out after `<duration>` contacting `<host>`. Increase with `--timeout <duration>` or check VPN connectivity."

#### Scenario: `SSL_CERT_FILE` could not be loaded
- **WHEN** the configured `SSL_CERT_FILE` path is missing or unparseable
- **THEN** the command exits with a code `>= 10` and prints an error of the form: "SSL_CERT_FILE points to `<path>` which could not be loaded. Verify the path and that the file is a valid PEM certificate bundle."

#### Scenario: Malformed response
- **WHEN** any command receives a response from Confluence that cannot be parsed as the expected content type or shape
- **THEN** the command exits with a code `>= 10` and prints an error of the form: "Received an unexpected response from `<host>`. Re-run with `--debug` to inspect the exchange, or file an issue."

#### Scenario: Network error
- **WHEN** any command's request to Confluence fails with a connection refused, DNS resolution failure, or unreachable-host error that is not a timeout
- **THEN** the command exits with a code `>= 10` and prints an error of the form: "Network error contacting `<host>`: `<cause>`. Check that the host is reachable and that any required VPN is connected."

### Requirement: Represent every cfl-level failure as a `CFLError`

The system SHALL model every `cfl`-level failure as a `CFLError` value carrying a `Code`, a `Message`, a `Suggestion`, and an optional `Cause`. The top rendering layer MUST print `Error: <message>` followed by the suggestion on the next line, and MUST NOT leak the underlying `Cause` to the user unless `--debug` is in effect.

#### Scenario: Rendered error includes message and suggestion
- **WHEN** a command fails with a `CFLError`
- **THEN** stderr contains a line `Error: <message>` immediately followed by a line containing the suggestion

#### Scenario: Cause is hidden without `--debug`
- **WHEN** a command fails with a `CFLError` that wraps an underlying `Cause` and `--debug` is not set
- **THEN** the user sees the translated message and suggestion but does not see the raw wrapped `Cause`

### Requirement: Bypass error translation with `--debug`

The system SHALL, when `--debug` is set, additionally print the raw HTTP exchange (request and response, including body) to stderr alongside the translated error. The translated message MUST still be printed for consistency.

#### Scenario: Debug shows raw response alongside translation
- **WHEN** the user runs any failing command with `--debug`
- **THEN** stderr contains both the raw HTTP request/response and the translated `cfl` error message

### Requirement: Map exit codes to success and cfl-level failure only

The system SHALL exit with code `0` on success and with a code `>= 10` for any `cfl`-level failure (bad URL, authentication, network, parsing, or configuration). There are no intermediate command-result exit codes. The exact value within the `>= 10` range MAY be unstable across versions but MUST always be non-zero; consumers and tests MUST assert only on `>= 10`, never on a specific value.

#### Scenario: Success exit code
- **WHEN** a command completes successfully and returns no error
- **THEN** the process exits with code `0`

#### Scenario: Failure exit code
- **WHEN** a command fails for any `cfl`-level reason (bad URL, authentication, network, parsing, or configuration)
- **THEN** the process exits with a code `>= 10`

#### Scenario: `nil` error maps to zero
- **WHEN** the top layer receives a `nil` error from a command
- **THEN** it maps the result to exit code `0`; otherwise it maps the error to an exit code `>= 10`

