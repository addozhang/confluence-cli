## ADDED Requirements

### Requirement: Parse Confluence URLs into a structured reference

The system SHALL provide a URL parser that converts any supported Confluence Server/DC URL into a structured `Ref` value carrying the base URL (scheme + host, default port stripped), the optional context-path prefix, an optional numeric page ID, and an optional space key. The parser MUST NOT contact Confluence; resolving a space-key-plus-title reference to a concrete page ID requires an API call and is not the parser's responsibility.

The parser MUST accept URLs with or without trailing slashes, with `http://` or `https://` schemes, and with or without explicit ports. The parser MUST strip trailing slashes from the path before extraction and MUST ignore the fragment. The parser MUST read the `pageId` query parameter where the shape defines it but otherwise MUST NOT let unrelated query parameters change the extracted identity.

#### Scenario: Parse a viewpage pageId URL
- **WHEN** the parser receives `https://wiki.example.com/pages/viewpage.action?pageId=12345`
- **THEN** the parser returns a Ref with base URL `https://wiki.example.com`, empty context path, page ID `12345`, and an empty space key

#### Scenario: Parse a REST content URL
- **WHEN** the parser receives `https://wiki.example.com/rest/api/content/12345`
- **THEN** the parser returns a Ref with base URL `https://wiki.example.com`, empty context path, page ID `12345`, and an empty space key

#### Scenario: Parse a display URL with space key and title
- **WHEN** the parser receives `https://wiki.example.com/display/DEV/Release+Notes`
- **THEN** the parser returns a Ref with base URL `https://wiki.example.com`, empty context path, space key `DEV`, and no page ID (the page is identified by title within the space and resolved later via the API)

#### Scenario: Parse a space home URL
- **WHEN** the parser receives `https://wiki.example.com/spaces/DEV` or `https://wiki.example.com/display/DEV`
- **THEN** the parser returns a Ref with base URL `https://wiki.example.com`, space key `DEV`, and no page ID

#### Scenario: Trailing slash and fragment ignored
- **WHEN** the parser receives `https://wiki.example.com/pages/viewpage.action?pageId=12345#comments` or `https://wiki.example.com/spaces/DEV/`
- **THEN** the parser returns the same Ref as the equivalent URL without the trailing slash or fragment

### Requirement: Parse a bare numeric page ID against a single configured instance

The system SHALL accept a bare numeric argument (e.g. `12345`) as a page ID. Because a bare numeric ID carries no host, the parser cannot determine the base URL from the argument alone; the system SHALL therefore resolve the base URL and context path from the credentials store, and this form is valid ONLY when exactly one instance is configured. When zero or more than one instance is configured, the system MUST reject the bare numeric form with an error instructing the user to provide a full Confluence URL.

#### Scenario: Bare numeric ID with a single configured instance
- **WHEN** exactly one instance `https://wiki.example.com` is configured and the user runs a command with the argument `12345`
- **THEN** the system resolves a Ref with base URL `https://wiki.example.com`, page ID `12345`, and the configured instance's context path

#### Scenario: Bare numeric ID with no configured instance
- **WHEN** no instance is configured and the user runs a command with the argument `12345`
- **THEN** the system returns an error stating that a bare page ID requires a full Confluence URL (or a configured single instance) and exits with a non-zero code

#### Scenario: Bare numeric ID with multiple configured instances
- **WHEN** two or more instances are configured and the user runs a command with the argument `12345`
- **THEN** the system returns an error stating that the target instance is ambiguous and instructing the user to pass a full Confluence URL, and exits with a non-zero code

### Requirement: Preserve the context path for instances behind a reverse proxy

The system SHALL capture the context-path prefix of a Confluence URL — the path segments between the host and the first `/rest/`, `/display/`, `/spaces/`, or `/pages/` segment — onto the `Ref` so that requests address the instance at its actual mount point. The context path MUST be normalized to a leading `/` with no trailing slash, and MUST be the empty string for instances mounted at the host root. The presence of a context path MUST NOT change the scheme+host portion of the base URL.

#### Scenario: Context path preserved on a display URL
- **WHEN** the parser receives `https://wiki.example.com/confluence/display/DEV/Release+Notes`
- **THEN** the parser returns a Ref with base URL `https://wiki.example.com`, context path `/confluence`, and space key `DEV`

#### Scenario: Context path preserved on a pageId URL
- **WHEN** the parser receives `https://wiki.example.com/confluence/pages/viewpage.action?pageId=12345`
- **THEN** the parser returns a Ref with base URL `https://wiki.example.com`, context path `/confluence`, and page ID `12345`

#### Scenario: Root-mounted URL yields an empty context path
- **WHEN** the parser receives `https://wiki.example.com/pages/viewpage.action?pageId=12345`
- **THEN** the parser returns a Ref with context path `""`, identical to the behavior for an instance mounted at the host root

### Requirement: Normalize host for credential lookup

The system SHALL derive the credential-lookup host key from a parsed URL as scheme + lowercased hostname + non-default port, stripping the default port for the scheme (`:443` for `https`, `:80` for `http`). The normalized host key, combined with the captured context path, is the input to the `auth` capability's lookup.

#### Scenario: Host lowercased for lookup
- **WHEN** a command is invoked with `https://Wiki.Example.COM/display/DEV/Home`
- **THEN** the system derives the host key `https://wiki.example.com` (lowercased) for credential lookup

#### Scenario: Non-default port preserved
- **WHEN** a command is invoked with `http://wiki.local:8090/display/DEV/Home`
- **THEN** the system derives the host key `http://wiki.local:8090`, preserving the explicit non-default port

#### Scenario: Default port stripped
- **WHEN** a command is invoked with `https://wiki.example.com:443/display/DEV/Home` (or `http://wiki.example.com:80/...`)
- **THEN** the system derives the host key `https://wiki.example.com` (or `http://wiki.example.com`), stripping the default port to normalize the lookup key

### Requirement: Reject malformed or non-Confluence URLs

The system SHALL reject any argument that is neither a recognized Confluence URL shape nor a bare numeric page ID, returning an actionable error rather than a partial or guessed `Ref`. A string that is not a valid URL, a URL with no scheme, and a URL whose path matches none of the supported Confluence shapes MUST all be rejected.

#### Scenario: Malformed argument rejected
- **WHEN** the parser receives `not a url`, an empty string, or a scheme-less `wiki.example.com/display/DEV` 
- **THEN** the parser returns an error describing the parse failure and the command exits with a non-zero code

#### Scenario: Non-Confluence path rejected
- **WHEN** the parser receives a URL whose path matches no supported shape (e.g. `https://wiki.example.com/dashboard.action` or `https://wiki.example.com/admin/`)
- **THEN** the parser returns an error indicating the URL does not point at a Confluence page or space

#### Scenario: pageId query present but non-numeric
- **WHEN** the parser receives `https://wiki.example.com/pages/viewpage.action?pageId=abc`
- **THEN** the parser returns an error indicating the `pageId` value is not a valid numeric page ID
