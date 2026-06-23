# auth Specification

## ADDED Requirements

### Requirement: Store an instance alias on add

The system SHALL accept an optional `--alias <name>` flag on `cfl auth add <url>` that stores a short alias for the instance alongside its Personal Access Token. The alias MUST match `[a-zA-Z0-9_-]+` and is case-sensitive. An alias MUST be unique across configured instances: adding an alias already bound to a different instance MUST fail with a non-zero exit code and an actionable error, without modifying any entry. Re-adding the same instance with the same alias MUST be idempotent. The alias MUST be persisted in the credentials file in a backward-compatible way, so that a credentials file written by a version without alias support still loads, and an entry without an alias remains valid. The confirmation message MUST name the stored alias when one was set, and MUST NOT print the token.

#### Scenario: Add an instance with an alias
- **WHEN** the user runs `cfl auth add https://wiki.example.com --alias prod` and enters a valid token
- **THEN** the system stores the token under the instance key with alias `prod`, prints a confirmation naming both the key and the alias `prod`, and does not print the token

#### Scenario: Duplicate alias on a different instance is rejected
- **WHEN** alias `prod` is already bound to `https://wiki.example.com` and the user runs `cfl auth add https://other.example.com --alias prod`
- **THEN** the system exits with a non-zero code, reports that the alias `prod` is already in use, and leaves all existing entries unchanged

#### Scenario: Malformed alias is rejected
- **WHEN** the user runs `cfl auth add https://wiki.example.com --alias "has spaces"`
- **THEN** the system exits with a non-zero code and reports that the alias must match `[a-zA-Z0-9_-]+`, without storing anything

#### Scenario: Alias is optional and backward compatible
- **WHEN** the user runs `cfl auth add https://wiki.example.com` without `--alias` against a credentials file that contains entries written without aliases
- **THEN** the system stores the new entry with no alias, leaves the existing alias-less entries intact, and all entries remain loadable

### Requirement: Resolve an alias to its instance

The system SHALL provide an internal alias resolution that maps a configured alias name to its instance base URL and context path. The resolution is used by every instance-targeting command so that an `--instance <name>` matching a stored alias is expanded to the instance, and an `<alias>:<id>` target selects the instance for a bare page ID. An alias that is not configured MUST resolve to a miss that the caller surfaces as an actionable error suggesting `cfl auth list`.

#### Scenario: Alias resolves to the instance base URL
- **WHEN** alias `prod` is bound to `https://wiki.example.com/confluence` and a command resolves `--instance prod`
- **THEN** the system yields base URL `https://wiki.example.com` and context path `/confluence` for the request

#### Scenario: Unknown alias resolves to a miss
- **WHEN** a command resolves `--instance nope` and no alias `nope` is configured and `nope` is not a valid URL
- **THEN** the system reports an actionable error that the alias is unknown, suggesting `cfl auth list`

### Requirement: Show aliases in the instance listing

The system SHALL include the alias (when set) for each instance in the `cfl auth list` output. The listing MUST continue to never print any token, and an instance without an alias MUST still be listed with its key (its alias rendered as absent/null in structured output).

#### Scenario: Listing shows aliases
- **WHEN** the user runs `cfl auth list` after adding `https://wiki.example.com` with alias `prod` and `https://other.example.com` without an alias
- **THEN** the output lists both instances, shows alias `prod` for the first and an absent/null alias for the second, exits with code `0`, and prints no token
