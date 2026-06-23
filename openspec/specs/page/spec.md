# page Specification

## Purpose
TBD - created by archiving change init-cfl-confluence-cli. Update Purpose after archive.
## Requirements
### Requirement: Read a page by URL or numeric ID

The system SHALL provide a `cfl page get <url-or-id>` command that resolves the argument into a Confluence page reference and reads the page from the Server/DC REST API. When the argument carries a numeric page ID (a bare ID, a `pages/viewpage.action?pageId=` URL, or a `rest/api/content/{id}` URL), the command MUST read that page directly by ID. When the argument is a display URL carrying a space key and page title (`/display/KEY/Title`), the command MUST first resolve the page to its numeric ID via a title lookup within the space, then read it. The read MUST request the page's body (in storage format), version, space, and ancestors. The exact REST endpoints, query parameters, and `expand` values are defined in `design.md` (D9 endpoint mapping). The structured response MUST begin with `schemaVersion: "1"` and use camelCase fields, exposing at least `id`, `title`, `spaceKey`, `version` (the numeric version number), `body` (the storage-format XHTML, unconverted), and `ancestors` (each ancestor's `id` and `title`).

#### Scenario: Get a page by pageId URL
- **WHEN** the user runs `cfl page get https://wiki.example.com/pages/viewpage.action?pageId=12345`
- **THEN** the command reads page `12345` directly by ID (no title lookup) and renders a document with `schemaVersion: "1"`, `id: "12345"`, the page `title`, `spaceKey`, the numeric `version`, the `body` storage XHTML, and an `ancestors` array

#### Scenario: Get a page by display URL
- **WHEN** the user runs `cfl page get https://wiki.example.com/display/ENG/Runbook` against a space `ENG` that contains a page titled `Runbook`
- **THEN** the command first performs a title lookup for `Runbook` in space `ENG` to resolve the page ID, then reads the page and renders the same schema shape with the resolved `id` and `spaceKey: "ENG"`

#### Scenario: Page not found
- **WHEN** the user runs `cfl page get https://wiki.example.com/pages/viewpage.action?pageId=99999` and Confluence responds that the page does not exist
- **THEN** the command exits with a non-zero code and prints a translated `Page not found` error plus a suggestion to verify the page ID or URL

#### Scenario: Display URL resolves to no page
- **WHEN** the user runs `cfl page get https://wiki.example.com/display/ENG/Missing` and the title lookup returns no matching page
- **THEN** the command exits with a non-zero code and prints an error stating no page titled `Missing` exists in space `ENG`

### Requirement: Create a page under a space or parent

The system SHALL provide a `cfl page create --space KEY --title T --body <input> [--parent ID]` command that creates a new Confluence page. The request MUST create a page of type `page` with the supplied title under the supplied space key, sending the body in Confluence storage-format representation. When `--parent ID` is supplied, the new page MUST be created as a child of that parent; when it is omitted, the new page MUST be created at the top level of the space (no parent). The exact REST endpoint and request-body shape are defined in `design.md` (D9 endpoint mapping). The `--space`, `--title`, and `--body` flags are required; a missing required flag MUST fail with a non-zero exit code and an actionable error before any HTTP request is made. The `--body` value MUST accept three input forms: `@path` reads the body from the named file, `-` reads the body from stdin, and any other literal string is used verbatim as the storage-format body. The body is passed through unchanged; the command MUST NOT convert Markdown or wiki markup. On success the structured response MUST begin with `schemaVersion: "1"` and expose the created page's `id`, `title`, `spaceKey`, `version`, and a `url` pointing at the new page.

#### Scenario: Create a top-level page from a file body
- **WHEN** the user runs `cfl page create --space ENG --title "Release Notes" --body @./notes.xhtml`
- **THEN** the command reads `./notes.xhtml` and creates a top-level page (no parent) titled `Release Notes` in space `ENG` with the file contents as its storage-format body, then renders a document with `schemaVersion: "1"` carrying the new `id` and `url`

#### Scenario: Create a child page with a parent
- **WHEN** the user runs `cfl page create --space ENG --title "Sub Page" --parent 12345 --body "<p>hi</p>"`
- **THEN** the command creates a page titled `Sub Page` as a child of page `12345`, with the literal `<p>hi</p>` as its storage-format body, then returns the created page's `id` and `url`

#### Scenario: Body read from stdin
- **WHEN** the user runs `cfl page create --space ENG --title "Piped" --body -` and pipes XHTML into stdin
- **THEN** the command reads the entire stdin stream as the storage-format body and submits it unchanged in the create request

#### Scenario: Missing required flag
- **WHEN** the user runs `cfl page create --space ENG --title "No Body"` without a `--body` flag
- **THEN** the command exits with a non-zero code and prints an error naming the missing `--body` flag, without issuing any HTTP request

### Requirement: Update a page version-safely

The system SHALL provide a `cfl page update <url-or-id> --body <input> [--title T]` command that performs a version-aware update. The command MUST first read the page's current version number (resolving display URLs to an ID via a title lookup as in page get), then submit an update whose new version number equals the current version number plus one, of type `page`, carrying the body in storage-format representation, and whose title is the supplied `--title` or, when `--title` is omitted, the page's current title (so the title is preserved). The client MUST NOT guess or hard-code the version number; it MUST read the current value first. The exact REST endpoints and request-body shape are defined in `design.md` (D9 endpoint mapping). The `--body` flag is required and accepts the same `@file`, `-` (stdin), and literal forms as page create; a missing `--body` MUST fail with a non-zero exit code before any HTTP request. When the update is rejected because the page was modified concurrently (a stale-version conflict), the command MUST surface a clear error explaining the conflict and suggesting the user re-run the update to retry against the latest version. On success the response MUST begin with `schemaVersion: "1"` and report the new incremented `version`.

#### Scenario: Successful update increments the version
- **WHEN** the user runs `cfl page update https://wiki.example.com/pages/viewpage.action?pageId=12345 --body @./new.xhtml` against a page currently at version `7`
- **THEN** the command reads the current version `7`, submits an update for page `12345` with new version `8`, type `page`, and the file contents as the storage body, then renders a response with `version: 8`

#### Scenario: Stale-version conflict
- **WHEN** the user runs `cfl page update <url> --body @./new.xhtml` and the update is rejected because the page was updated by someone else after the version was read
- **THEN** the command exits with a non-zero code and prints a translated error stating the page changed concurrently, suggesting the user re-run `cfl page update` to retry against the latest version

#### Scenario: Title preserved when not supplied
- **WHEN** the user runs `cfl page update <url> --body @./new.xhtml` without a `--title` flag against a page whose current title is `Runbook`
- **THEN** the submitted update keeps the title `Runbook`, leaving the title unchanged

#### Scenario: Body is required
- **WHEN** the user runs `cfl page update <url>` without a `--body` flag
- **THEN** the command exits with a non-zero code and prints an error naming the missing `--body` flag, without issuing any HTTP request

### Requirement: Delete (trash) a page with explicit intent

The system SHALL provide a `cfl page delete <url-or-id>` command that moves the specified page to the Confluence trash (resolving display URLs to an ID via a title lookup as in page get). The exact REST endpoint is defined in `design.md` (D9 endpoint mapping). Because deletion is a state-changing operation, the command MUST require explicit user intent: it MUST NOT delete unless the user supplies a `--yes` flag OR interactively confirms the prompt. When stdin is an interactive terminal and `--yes` is absent, the command MUST prompt the user to confirm and MUST abort with a non-zero exit code if the confirmation is declined. When stdin is not a terminal (non-interactive) and `--yes` is absent, the command MUST refuse to delete and exit with a non-zero code instructing the user to pass `--yes`. When the page does not exist, the command MUST surface a clear `Page not found` error. On success the response MUST begin with `schemaVersion: "1"` and confirm the page `id` was moved to trash.

#### Scenario: Delete with explicit --yes
- **WHEN** the user runs `cfl page delete https://wiki.example.com/pages/viewpage.action?pageId=12345 --yes`
- **THEN** the command moves page `12345` to trash and returns a document with `schemaVersion: "1"` confirming the page `id: "12345"` was moved to trash

#### Scenario: Interactive confirmation declined
- **WHEN** the user runs `cfl page delete <url>` from an interactive terminal without `--yes` and answers `no` at the confirmation prompt
- **THEN** the command does NOT delete the page and exits with a non-zero code reporting the deletion was cancelled

#### Scenario: Non-interactive without --yes
- **WHEN** the user runs `cfl page delete <url>` with stdin not attached to a terminal (e.g. in a script) and without `--yes`
- **THEN** the command does NOT delete the page and exits with a non-zero code instructing the user to pass `--yes` to confirm deletion

#### Scenario: Delete an already-deleted or missing page
- **WHEN** the user runs `cfl page delete <url> --yes` against a page ID that does not exist
- **THEN** the command exits with a non-zero code and prints a translated `Page not found` error suggesting the page was already deleted or the ID is wrong

### Requirement: List a page's direct children

The system SHALL provide a `cfl page children <url-or-id>` command that lists the direct child pages of the specified page (resolving display URLs to an ID via a title lookup as in page get). The exact REST endpoint and `expand` values are defined in `design.md` (D9 endpoint mapping). The structured response MUST begin with `schemaVersion: "1"` and expose a `children` array in which each entry carries at least the child's `id`, `title`, and numeric `version`. When the page has no children, the command MUST return an empty `children` array (not null and not an error) and exit with code `0`.

#### Scenario: List children of a page
- **WHEN** the user runs `cfl page children https://wiki.example.com/pages/viewpage.action?pageId=12345` against a page with two direct child pages
- **THEN** the command lists the direct children of page `12345` and returns a `children` array with two entries, each carrying `id`, `title`, and `version`

#### Scenario: Page with no children
- **WHEN** the user runs `cfl page children <url>` against a page that has no child pages
- **THEN** the command returns a document with `schemaVersion: "1"` and an empty `children` array, exiting with code `0`

#### Scenario: Children of a missing page
- **WHEN** the user runs `cfl page children <url>` against a page ID that does not exist
- **THEN** the command exits with a non-zero code and prints a translated `Page not found` error

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

