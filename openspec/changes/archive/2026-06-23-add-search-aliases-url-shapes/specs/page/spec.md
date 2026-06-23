# page Specification

## ADDED Requirements

### Requirement: Select the target instance for page commands

Every page command (`get`, `update`, `delete`, `children`) SHALL provide an `--instance <url-or-alias>` flag that selects the Confluence instance the command operates against. The flag value MAY be a full instance URL or a configured alias; an alias is expanded to its instance base URL and context path before the request. Instance selection follows these rules:

- **When the argument is a full URL** (a `/spaces/...`, `/display/...`, `pages/viewpage.action?pageId=`, or `rest/api/content/{id}` URL), the host is taken from the URL and `--instance` is ignored — the URL's instance always wins.
- **When the argument is an `<alias>:<id>` form**, the instance comes from the alias and `--instance` is ignored.
- **When the argument is a bare numeric page ID**, the instance is taken from `--instance` when supplied; otherwise it falls back to the single configured instance. A bare numeric ID with no `--instance` and zero or multiple configured instances MUST fail with an actionable error telling the user to pass `--instance <url-or-alias>` (or to use the `<alias>:<id>` form).

The `--instance` flag MUST NOT change how a credential is matched to a request: once the instance is resolved to a base URL, the existing host + context-path credential lookup applies.

#### Scenario: Bare ID with multiple instances requires an instance
- **WHEN** two instances are configured and the user runs `cfl page get 12345` with no `--instance`
- **THEN** the command exits with a non-zero code and prints an error instructing the user to pass `--instance <url-or-alias>` (or use `<alias>:<id>`), because the target instance is ambiguous

#### Scenario: Bare ID with an explicit instance alias
- **WHEN** an instance is configured with alias `prod` (among several) and the user runs `cfl page get 12345 --instance prod`
- **THEN** the command resolves `prod` to its instance and reads page `12345` from it, unambiguously

#### Scenario: Bare ID with an explicit instance URL
- **WHEN** several instances are configured and the user runs `cfl page get 12345 --instance https://wiki.example.com`
- **THEN** the command reads page `12345` from `https://wiki.example.com`

#### Scenario: Bare ID with a single configured instance needs no flag
- **WHEN** exactly one instance is configured and the user runs `cfl page get 12345` with no `--instance`
- **THEN** the command reads page `12345` from that single instance

#### Scenario: A full URL ignores --instance
- **WHEN** the user runs `cfl page get https://wiki.example.com/spaces/ENG/pages/12345 --instance other` where `other` is a different configured instance
- **THEN** the command reads the page from `https://wiki.example.com` (the URL's host), ignoring `--instance other`

#### Scenario: Alias-qualified ID selects its instance
- **WHEN** an instance is configured with alias `prod` (among several) and the user runs `cfl page get prod:12345`
- **THEN** the command reads page `12345` from the `prod` instance, valid even with multiple instances configured
