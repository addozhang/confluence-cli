# tls-and-transport Specification

## Purpose
TBD - created by archiving change init-cfl-confluence-cli. Update Purpose after archive.
## Requirements
### Requirement: Trust additional CAs via `SSL_CERT_FILE`

The system SHALL, on every HTTPS request to a Confluence host, build the TLS root CA pool from the system trust store augmented by any certificates loaded from the file path stored in the `SSL_CERT_FILE` environment variable. The environment variable MUST be honored without any additional flag, so that a Confluence instance presenting a self-signed certificate signed by that CA is trusted without `--insecure`.

#### Scenario: Self-signed Confluence via `SSL_CERT_FILE`
- **WHEN** the user runs any `cfl` command against a Confluence instance whose certificate is signed by a CA contained in the file referenced by `SSL_CERT_FILE`
- **THEN** the request succeeds without certificate verification errors and without requiring `--insecure`

#### Scenario: `SSL_CERT_FILE` set but file is missing
- **WHEN** the user runs any `cfl` command with `SSL_CERT_FILE` pointing at a path that does not exist
- **THEN** the command exits with a code `>= 10` and prints an error indicating that `SSL_CERT_FILE` references a path that could not be loaded, with the resolved path included in the message

#### Scenario: `SSL_CERT_FILE` set but file is not a valid PEM bundle
- **WHEN** the user runs any `cfl` command with `SSL_CERT_FILE` pointing at a file that is not a valid PEM-encoded certificate bundle
- **THEN** the command exits with a code `>= 10` and prints an error indicating that the file could not be loaded as a PEM certificate bundle

### Requirement: Allow insecure TLS via `--insecure` flag

The system SHALL provide a global `--insecure` flag that disables TLS certificate verification for the current invocation. When `--insecure` is in effect, the system MUST print a warning to stderr.

#### Scenario: Using `--insecure` against a self-signed instance
- **WHEN** the user runs `cfl page get https://wiki.local/display/DEV/Home --insecure`
- **THEN** TLS verification is skipped, a warning is printed to stderr indicating that certificate verification was disabled, and the request proceeds

#### Scenario: Warning is written to stderr, not stdout
- **WHEN** the user runs any `cfl` command with `--insecure` and pipes stdout to another tool
- **THEN** the insecure-mode warning appears on stderr only and does not contaminate the stdout payload

### Requirement: Apply a global request timeout

The system SHALL apply a per-request timeout configurable by a global `--timeout` flag accepting a Go-style duration string (e.g. `30s`, `2m`). The default value MUST be `30s`, and the timeout MUST bound every outbound request.

#### Scenario: Default timeout applied
- **WHEN** the user runs any `cfl` command without `--timeout`
- **THEN** each outbound HTTP request to Confluence is bounded by a 30-second timeout

#### Scenario: Custom timeout applied
- **WHEN** the user runs `cfl space list https://wiki.example.com --timeout 5m`
- **THEN** each outbound HTTP request is bounded by a 5-minute timeout

#### Scenario: Timeout exceeded
- **WHEN** a request to Confluence does not return within the configured timeout
- **THEN** the command exits with a code `>= 10` and prints an error indicating the timeout duration that was exceeded and suggesting `--timeout <duration>` to increase it

### Requirement: Log HTTP exchanges with `--debug` and redact the Authorization header

The system SHALL provide a global `--debug` flag that logs every outbound HTTP request (method, URL, headers, body) and inbound HTTP response (status, headers, body) to stderr. The `Authorization` header MUST always be redacted: the Bearer token value MUST NEVER appear anywhere in the debug output.

#### Scenario: Debug logging redacts the Bearer token
- **WHEN** the user runs any `cfl` command with `--debug` against an authenticated Confluence host
- **THEN** the debug log includes request method, URL, body, and all headers, but the `Authorization` header is rendered as a redacted placeholder and the Personal Access Token value never appears in the output

#### Scenario: Debug logging does not affect stdout
- **WHEN** the user runs `cfl page get <url> --debug -o json`
- **THEN** debug output is written exclusively to stderr, and stdout contains only the normal JSON response, allowing the user to pipe stdout to other tools while watching the debug stream

