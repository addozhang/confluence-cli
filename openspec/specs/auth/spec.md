# auth Specification

## Purpose
TBD - created by archiving change init-cfl-confluence-cli. Update Purpose after archive.
## Requirements
### Requirement: Add a Personal Access Token for a Confluence instance

The system SHALL provide a `cfl auth add <url>` command that stores a Confluence Personal Access Token (PAT) for an instance. Confluence Server/DC authenticates with a PAT carrying the user identity, so the command MUST prompt for the token alone using hidden (non-echoing) terminal input, and MUST NOT store or prompt for a username. The lookup key MUST be the URL scheme + hostname (with optional non-default port) plus an optional context-path prefix. The context-path prefix is the portion of the path preceding the first `/rest/`, `/display/`, `/spaces/`, or `/pages/` segment, or the entire path when the URL contains none of those segments; it MUST be normalized to a leading `/` with no trailing slash, and an empty result MUST yield a host-only key. Credentials MUST be persisted as TOML to `~/.config/cfl/credentials` (or the platform-equivalent config directory) with file mode `0600` on POSIX systems. The confirmation message MUST name the stored key verbatim so the user can see whether a context path was retained, and MUST NOT print the token.

#### Scenario: First-time credential addition
- **WHEN** the user runs `cfl auth add https://wiki.example.com` and enters a valid PAT at the hidden prompt
- **THEN** the system writes the credential under the key `https://wiki.example.com` to `~/.config/cfl/credentials` with mode `0600` and prints a confirmation message naming that key, without echoing or printing the token

#### Scenario: Overwriting an existing credential
- **WHEN** the user runs `cfl auth add https://wiki.example.com` for a key that already has a stored token
- **THEN** the system prompts for confirmation before overwriting and replaces the existing token on confirmation, leaving all other entries untouched

#### Scenario: Host-only normalization (no context path)
- **WHEN** the user runs `cfl auth add https://wiki.example.com/` (trailing slash) or `cfl auth add https://wiki.example.com/display/DEV/Home` (display URL, no context path)
- **THEN** the system stores the host-only key `https://wiki.example.com`, discarding the trailing slash and any `/display/...` path

#### Scenario: Context-path instance credential addition
- **WHEN** the user runs `cfl auth add https://wiki.example.com/confluence` (or `https://wiki.example.com/confluence/display/DEV/Home` with a display path) for a host that already has a separate `https://wiki.example.com/wiki` entry
- **THEN** the system stores the key `https://wiki.example.com/confluence`, names that full key in the confirmation message, and leaves the `https://wiki.example.com/wiki` entry untouched

### Requirement: List configured instances

The system SHALL provide a `cfl auth list` command that prints each configured instance key, one per line, in the configured output format (YAML by default). The command MUST NOT print any stored token.

#### Scenario: Listing configured instances
- **WHEN** the user runs `cfl auth list` after adding credentials for two instances
- **THEN** the system prints both instance keys to stdout, one per line, in the configured output format, exits with code `0`, and prints no token values

#### Scenario: No instances configured
- **WHEN** the user runs `cfl auth list` with no credentials file or an empty credentials file
- **THEN** the system prints an empty list (or an empty YAML/JSON array) and exits with code `0`

### Requirement: Verify a stored token against its instance

The system SHALL provide a `cfl auth whoami <url>` command that verifies the stored Personal Access Token for the instance the URL resolves to, by making an authenticated request that returns the current user's identity. The command exists so a user can confirm, before relying on `cfl`, that their token is present, valid, and unexpired. The URL argument MUST be resolved to a stored credential using the same most-specific-key lookup as every other contacting command. The exact REST endpoint is defined in `design.md` (D9 endpoint mapping). On success the structured response MUST begin with `schemaVersion: "1"` and expose the resolved `host` plus the identity fields the instance returns (e.g. `username`, `displayName`); it MUST NOT print the token. When the token is rejected (authentication failure), the command MUST exit with a non-zero code and print a translated error indicating the token is invalid or expired, suggesting the user run `cfl auth add <url>` to refresh it. When no credential is configured for the URL, the command MUST exit with a non-zero code instructing the user to run `cfl auth add <url>` first.

#### Scenario: Valid token resolves to the current user
- **WHEN** the user runs `cfl auth whoami https://wiki.example.com` and the stored token is valid
- **THEN** the command makes an authenticated identity request, prints a document with `schemaVersion: "1"` carrying the resolved `host` and the current user's identity (e.g. `username`), exits with code `0`, and never prints the token

#### Scenario: Expired or invalid token
- **WHEN** the user runs `cfl auth whoami https://wiki.example.com` and the instance rejects the token as unauthorized
- **THEN** the command exits with a non-zero code and prints a translated error stating the token is invalid or expired, suggesting the user run `cfl auth add https://wiki.example.com` to refresh it

#### Scenario: No credential configured for the URL
- **WHEN** the user runs `cfl auth whoami https://wiki.absent.com` for a host with no stored credential
- **THEN** the command exits with a non-zero code and prints an actionable error instructing the user to run `cfl auth add https://wiki.absent.com` first

### Requirement: Remove a stored instance credential

The system SHALL provide a `cfl auth remove <host>` command that deletes the stored token for the given instance key from the credentials file. The operation MUST be idempotent: removing a key that is not present MUST succeed without error rather than failing. The command MUST preserve file mode `0600` after rewriting the credentials file.

#### Scenario: Removing an existing credential
- **WHEN** the user runs `cfl auth remove https://wiki.example.com` for a key that currently has a stored token
- **THEN** the system deletes that entry, rewrites `~/.config/cfl/credentials` preserving mode `0600`, prints a confirmation naming the removed key, and exits with code `0`

#### Scenario: Removing a non-existent credential is idempotent
- **WHEN** the user runs `cfl auth remove https://wiki.absent.com` for a key that has no stored token
- **THEN** the system exits with code `0` without modifying any other entry and reports that no matching credential was present

### Requirement: Look up the token for a request URL

The system SHALL provide an internal credential lookup that resolves a Confluence request URL to a stored PAT by selecting the most specific configured key. A configured key matches a request when its scheme and host equal the request's and the request path either equals the key's context-path prefix or continues past it at a `/` segment boundary. Among all matching keys, the system SHALL select the one with the longest context-path prefix. A host-only key (no context path) matches any same-host request as the shortest prefix. The lookup is used by every command that contacts a Confluence instance.

#### Scenario: Host-only lookup
- **WHEN** a command is invoked with a URL whose host matches a host-only credential entry and no more specific entry exists
- **THEN** the system retrieves that entry's token and uses it to authenticate subsequent requests

#### Scenario: Most-specific context-path entry wins
- **WHEN** the store holds both `https://wiki.example.com` and `https://wiki.example.com/confluence`, and a command runs against `https://wiki.example.com/confluence/pages/viewpage.action?pageId=12345`
- **THEN** the system selects the `https://wiki.example.com/confluence` token, not the host-only one

#### Scenario: Fallback to a host-only entry
- **WHEN** the store holds `https://wiki.example.com` (host-only) and a command runs against `https://wiki.example.com/wiki/display/DEV/Home` for which no `/wiki` entry exists
- **THEN** the system uses the host-only `https://wiki.example.com` token

#### Scenario: Segment boundary prevents partial matches
- **WHEN** the store holds `https://wiki.example.com/conf` (and no host-only entry) and a command runs against `https://wiki.example.com/confluence/display/DEV/Home`
- **THEN** the `https://wiki.example.com/conf` entry does NOT match, because the request path does not continue at a `/` boundary, and the lookup reports a missing credential

#### Scenario: Missing credential for the request
- **WHEN** a command is invoked with a URL for which no configured key matches at any prefix
- **THEN** the system exits with a non-zero code and prints an actionable error message instructing the user to run `cfl auth add <url>`

### Requirement: Send the token as an HTTP Bearer header

The system SHALL attach the resolved PAT to every Confluence request as the HTTP header `Authorization: Bearer <token>`. The system MUST NOT send HTTP Basic authentication and MUST NOT transmit a username. The token MUST NOT appear in any user-facing output or in `--debug` logs, where it MUST be redacted.

#### Scenario: Bearer header construction
- **WHEN** a request is issued to an instance whose resolved credential token is `PAT-abc123`
- **THEN** the outgoing request carries the header `Authorization: Bearer PAT-abc123` and carries no Basic `Authorization` header and no username

#### Scenario: Token redacted in debug output
- **WHEN** the user runs any contacting command with `--debug` and a stored token is used to authenticate
- **THEN** the logged request/response pair shows the `Authorization` header value redacted (e.g. `Bearer ****`), never the raw token

### Requirement: Guide a first-time user with no configured credentials

The system SHALL detect the first-run condition — a contacting command invoked when no credential matches the target instance — and respond with onboarding guidance rather than a bare failure. The error message MUST name the exact `cfl auth add <url>` command to run for the specific instance the user targeted, and SHOULD point the user at where Confluence Server/DC issues Personal Access Tokens (the user profile's Personal Access Tokens settings). This guidance MUST be uniform across every contacting command (`page`, `space`, and `auth whoami`), so that whichever command the user tries first, the path to a working setup is the same. The guidance MUST NOT block offline commands: `cfl version` and `cfl auth list` MUST work with no credentials configured and MUST NOT emit this guidance.

#### Scenario: First contacting command names the add command for the target instance
- **WHEN** a user with no credentials file runs `cfl page get https://wiki.example.com/pages/viewpage.action?pageId=12345`
- **THEN** the command exits with a non-zero code and prints guidance naming `cfl auth add https://wiki.example.com` as the next step, and pointing the user at where to create a Personal Access Token

#### Scenario: Guidance is uniform across commands
- **WHEN** a user with no credentials file runs `cfl space list https://wiki.example.com` and, separately, `cfl auth whoami https://wiki.example.com`
- **THEN** both commands surface the same onboarding guidance to run `cfl auth add https://wiki.example.com`, rather than divergent or generic errors

#### Scenario: Offline commands do not demand credentials
- **WHEN** a user with no credentials file runs `cfl version` or `cfl auth list`
- **THEN** the command succeeds, exits with code `0`, and does not print first-run credential guidance

