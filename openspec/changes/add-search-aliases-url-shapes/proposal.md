## Why

Three gaps surfaced once `cfl` met real Confluence instances and multi-instance workflows:

1. **Modern page URLs don't parse.** Newer Confluence builds render pages as `/spaces/KEY/pages/ID/Title` (the URL you get from the browser address bar today), but `cfl`'s parser only understands the legacy `pages/viewpage.action?pageId=` and `display/KEY/Title` shapes. Pasting the URL you actually see fails.
2. **No way to find a page you can't already address.** Every command needs a URL or ID up front; there is no way to discover content. This was deferred as "CQL search → v0.2", and this is v0.2.
3. **Multi-instance ergonomics are poor.** Targeting an instance means pasting its full base URL on every `--instance` flag (and bare numeric IDs are rejected outright when more than one instance is configured). Operators who work across `prod`/`staging` want a short name.

## What Changes

- **New page URL shape**: `cfl` parses `https://host/spaces/KEY/pages/ID/Title` (and its context-path and trailing-slash variants) into a page `Ref` carrying space key, numeric page ID, and title.
- **New `cfl search` command**: a keyword search with friendly flags (`--space`, `--type`, `--limit`, `--start`) that `cfl` compiles into Confluence CQL and runs against the search REST endpoint, returning a stable `results` schema. A `--cql` flag is the **highest-precedence** input: when supplied it is used as the complete query and overrides the friendly inputs, so power users can drop a full CQL string in without removing other flags.
- **Instance aliases**: `cfl auth add <url> --alias <name>` stores a short alias for an instance. Anywhere a command accepts an instance (`--instance <alias>`) or a target, the alias resolves to the stored base URL + context path. A bare numeric page ID may be qualified as `<alias>:<id>` to pick the instance unambiguously even when several are configured. `cfl auth list` shows aliases.
- **Instance selection for page commands**: `page get/update/delete/children` gain an `--instance <url-or-alias>` flag. A full URL or an `<alias>:<id>` argument carries its own instance and ignores `--instance`; a bare numeric ID uses `--instance` when given, falls back to the single configured instance, and requires `--instance` (or `<alias>:<id>`) when several instances are configured.

### Not Doing (non-goals)

- **No client-side CQL validation** — an invalid `--cql` is surfaced as the server's translated error, not pre-validated by `cfl`. `--cql` is the highest-precedence search input (it overrides the friendly flags), not a fenced-off escape hatch.
- **No alias for anything but instances** — aliases name instances, not spaces, pages, or saved searches.
- **No search result pagination beyond a single bounded window** — like `space list`, `search` returns one `--limit`/`--start` page, not an auto-paginated stream.
- **No new content types in search** — `--type` covers `page` and `blogpost` (Confluence's content types); attachments/comments remain out of scope.
- **No change to the credential resolution model** — aliases are a naming convenience layered on top of the existing host + context-path longest-prefix lookup; they do not change how tokens are matched to requests.
- **No Confluence Cloud support** — still Server/DC only.

## Capabilities

### New Capabilities

- `search`: Search Confluence content by keyword with `--space`/`--type`/`--limit`/`--start` flags compiled to CQL, or a raw `--cql` string that takes highest precedence and overrides the friendly flags; return a `results` array (each with id, title, type, spaceKey, url) plus pagination metadata, addressed at a resolved instance.

### Modified Capabilities

- `url-resolution`: Additionally parse the `/spaces/KEY/pages/ID/Title` page shape (with context-path and trailing-slash/fragment variants) into a page `Ref`; accept an `<alias>:<id>` qualified bare-ID form.
- `auth`: Store an optional per-instance alias on `cfl auth add --alias <name>`; resolve an alias to its instance base URL + context path; surface aliases in `cfl auth list`; reject duplicate or malformed aliases.
- `page`: Add an `--instance <url-or-alias>` flag to `get`/`update`/`delete`/`children` for selecting the target instance of a bare page ID; a full URL or `<alias>:<id>` argument carries its own instance and ignores the flag; a bare ID requires an instance when several are configured.

## Impact

- **Modified** `internal/confluenceurl`: a new parse branch for `/spaces/.../pages/...`; alias-qualified bare-ID handling lives in `internal/cli` (it needs the store), consistent with the existing bare-ID rule.
- **New** `internal/cli/search.go` + `cfl search` wiring; **new** client method `Search` in `internal/confluence` and a `MapSearch` mapper + `SearchResults` type in `internal/schema`.
- **Modified** `internal/auth`: the credentials TOML gains an optional `alias` per instance entry; new `Store.ResolveAlias` and alias-aware `List`. The on-disk format stays backward compatible (entries without an alias still load).
- **New external contract** rows in `docs/schema.md` for the `search` output and the alias field in `auth list`.
- **New REST endpoint** added to the design endpoint mapping: `GET /rest/api/search?cql=&limit=&start=`.
- **Docs**: README gains a Search section, an alias example, and the new URL shape in the URL-conventions table.
- This is a **v0.2** change; it is additive and does not alter any `stable` schema field already shipped, so `schemaVersion` stays `"1"`.
