# url-resolution Specification

## ADDED Requirements

### Requirement: Parse the modern spaces/pages page URL

The system SHALL parse the modern Confluence page URL shape `/spaces/{KEY}/pages/{ID}[/{Title}]` into a page `Ref` carrying the base URL (scheme + host, default port stripped), the optional context-path prefix, the space key, the numeric page ID, and the optional title. The page ID in this shape is authoritative: when present, the page is identified directly by ID with no title lookup required, and the title (if present) is decoded for display the same way as a display URL (`+` decoded to space). The parser MUST accept this shape with or without a trailing title segment, with or without a trailing slash, with a context-path prefix before `/spaces/`, and MUST ignore the fragment. A non-numeric ID in the `pages/{ID}` position MUST be rejected with an error. The parser MUST NOT contact Confluence.

#### Scenario: Parse a spaces/pages URL with title
- **WHEN** the parser receives `https://test-confluence.example.com/spaces/test/pages/5789518257/Test+Page`
- **THEN** the parser returns a Ref with base URL `https://test-confluence.example.com`, empty context path, space key `test`, page ID `5789518257`, and title `Test Page`

#### Scenario: Parse a spaces/pages URL without a title
- **WHEN** the parser receives `https://wiki.example.com/spaces/ENG/pages/12345`
- **THEN** the parser returns a Ref with base URL `https://wiki.example.com`, space key `ENG`, page ID `12345`, and an empty title

#### Scenario: Context path preserved on a spaces/pages URL
- **WHEN** the parser receives `https://wiki.example.com/confluence/spaces/ENG/pages/12345/Runbook`
- **THEN** the parser returns a Ref with base URL `https://wiki.example.com`, context path `/confluence`, space key `ENG`, page ID `12345`, and title `Runbook`

#### Scenario: Trailing slash and fragment ignored on a spaces/pages URL
- **WHEN** the parser receives `https://wiki.example.com/spaces/ENG/pages/12345/Runbook/` or `https://wiki.example.com/spaces/ENG/pages/12345#section`
- **THEN** the parser returns the same Ref (space key `ENG`, page ID `12345`) as the equivalent URL without the trailing slash or fragment

#### Scenario: A bare spaces URL is still a space reference
- **WHEN** the parser receives `https://wiki.example.com/spaces/ENG`
- **THEN** the parser returns a space Ref (space key `ENG`, no page ID), unchanged from the existing behavior, because only the `/pages/{ID}` continuation makes it a page reference

#### Scenario: Non-numeric page id in the spaces/pages shape is rejected
- **WHEN** the parser receives `https://wiki.example.com/spaces/ENG/pages/not-a-number/Title`
- **THEN** the parser returns an error indicating the page ID is not a valid numeric page ID

### Requirement: Resolve an alias-qualified bare page ID

The system SHALL accept a target of the form `<alias>:<id>`, where `<alias>` is a configured instance alias and `<id>` is a bare numeric page ID, and resolve it to a page `Ref` whose base URL and context path come from the aliased instance and whose page ID is `<id>`. This form selects the instance unambiguously and is therefore valid even when multiple instances are configured, unlike a plain bare numeric ID. The form MUST only be recognized when the segment before the first `:` is a known alias and the segment after it is entirely numeric; a value containing `://`, `.`, or `/` MUST be treated as a URL instead, and an unknown alias prefix MUST produce an actionable error. Because alias resolution requires the credential store, this resolution occurs in the CLI layer (not the network-free URL parser).

#### Scenario: Alias-qualified bare id resolves the instance
- **WHEN** an instance `https://wiki.example.com` is configured with alias `prod` and the user runs a command with the argument `prod:12345`
- **THEN** the system resolves a Ref with base URL `https://wiki.example.com`, page ID `12345`, and the aliased instance's context path, regardless of how many other instances are configured

#### Scenario: Unknown alias prefix is rejected
- **WHEN** no alias `staging` is configured and the user runs a command with the argument `staging:12345`
- **THEN** the system exits with a non-zero code and prints an error that the alias `staging` is unknown, suggesting `cfl auth list` to see configured aliases

#### Scenario: A URL is not mistaken for an alias-qualified id
- **WHEN** the user runs a command with `https://wiki.example.com/spaces/ENG/pages/12345` (which contains `://`)
- **THEN** the system parses it as a URL, not as an `<alias>:<id>` form
