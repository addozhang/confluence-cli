## Context

This is the v0.2 change for `cfl`. It extends three already-shipped capabilities (`url-resolution`, `auth`) and adds one (`search`), staying within the constitution (`SPEC.md`) and the architecture set by the initial change: `cmd/cfl` wiring only, `internal/confluence` returns raw bytes, `internal/schema` maps as pure functions, errors translated at the top. The decisions below record how the three features fit that shape without re-litigating the established patterns (D1–D9 of the initial `design.md` still hold).

## Goals / Non-Goals

**Goals:**

- Parse the modern `/spaces/KEY/pages/ID/Title` page URL into the same `confluenceurl.Ref` every command already consumes.
- A `cfl search` that is friendly by default (keyword + flags) and compiles to CQL internally, with a raw `--cql` escape hatch — bounded to a single page like `space list`.
- Instance aliases as a thin naming layer over the existing credential store, usable wherever an instance is named, plus an `<alias>:<id>` qualifier for bare IDs.

**Non-Goals:**

- Client-side CQL validation, raw-CQL as the primary interface, alias for non-instances, auto-pagination, new content types beyond page/blogpost, Cloud support, any `stable` schema break.

## Decisions

### D10. The `/spaces/KEY/pages/ID/Title` shape is a new parse branch, not a new marker

`confluenceurl.Parse` already splits the path at the first marker segment (`rest`/`display`/`spaces`/`pages`) to separate context path from the Confluence path. `spaces` is already a marker. The new shape reuses that split and adds a branch inside the existing `case "spaces"`: if the segments after the space key are `pages/{numericID}[/{Title}]`, produce a page `Ref` (space key + page ID + optional title) instead of a space-only `Ref`.

- **Why**: no change to context-path detection; the shape is just a richer `spaces` path. A bare `/spaces/KEY` still yields a space ref; `/spaces/KEY/pages/ID/Title` yields a page ref. Title is decoded the same way as display URLs (`+`→space).
- **Page ID is authoritative**: when both a page ID and a title are present, the ID is used directly (no title lookup needed), unlike a `display/KEY/Title` URL which must look the title up. The title is retained on the `Ref` for display only.
- **Alternative considered**: a separate top-level marker. Rejected — `spaces` is the marker; the page-ness is a sub-path, and treating it otherwise would duplicate context-path logic.

### D11. `search` compiles friendly flags to CQL; `--cql` is highest precedence

`cfl search <text>` builds a CQL string from the positional text plus flags: `text` → `text ~ "<escaped>"`, `--space KEY` → `space = "KEY"`, `--type page|blogpost` → `type = "<t>"`, joined with `AND`. The request is `GET /rest/api/search?cql=<built>&limit=&start=`. **`--cql "<raw>"` has the highest precedence**: when present, it is the complete query and the builder inputs (`text`, `--space`, `--type`) are ignored, not rejected — `--cql` overrides them. A stderr note SHOULD record that `--cql` took precedence so a silently-overridden friendly flag is visible, while stdout stays limited to the payload.

- **Why override, not mutual-exclusion**: `--cql` is the power interface and should always win when given, so a user can drop a full CQL query in without first removing other flags (e.g. from shell history or a script). Erroring on the combination would be friction for the exact users who reach for `--cql`. Precedence is also trivial to reason about: "if `--cql` is set, that is the query."
- **Escaping (friendly path only)**: the positional text is CQL-quoted (wrap in double quotes, backslash-escape embedded quotes/backslashes) so a search term with spaces or quotes cannot break the query or inject CQL. `--space`/`--type` values are constrained (space key charset; type enum) and quoted. `--cql` is the user's own responsibility and is passed through verbatim (documented), like raw SQL.
- **Result shape**: the search endpoint returns `results[]` of search records; `MapSearch` projects each into `{id, title, type, spaceKey, url}` and carries `start`/`limit`/`size`. Records without a content id (e.g. a space hit) are mapped with the fields that exist and `null` for the rest, never dropped silently — but `--type` defaults to `page` so the common case is clean.
- **Empty results**: an empty `results` is an empty array and exit `0`, mirroring `space list` / `page children`.
- **Pagination always applies**: `--limit`/`--start` map onto the request regardless of whether the CQL came from the builder or `--cql`.

### D12. Aliases live in the credentials file as an optional field; resolution is a pre-pass

The credentials TOML entry gains an optional `alias`. Today an entry is `key → token`; it becomes `key → { token, alias }`, encoded so that entries written by v0.1 (bare token strings) still load. `cfl auth add <url> --alias <name>` sets it. `auth.Store` gains `ResolveAlias(name) → (instanceKey, ok)`.

Alias resolution happens **before** URL parsing, in the cli layer, as a textual pre-pass:

- `--instance <value>`: if `<value>` matches a stored alias, substitute the instance's base URL (+context path) before building the client; otherwise treat `<value>` as a URL.
- A target argument of the form `<alias>:<id>` where `<id>` is numeric and `<alias>` is a known alias resolves to that instance's base URL + the page ID — this is the alias-qualified bare-ID form, and it is unambiguous even with many instances configured (it sidesteps the "bare numeric needs a single instance" rule from D3).

- **Why a pre-pass, not a new `Ref` field**: keeps `confluenceurl` network-free and alias-unaware (aliases require the store, which `confluenceurl` must not import). The cli layer already owns the bare-ID instance decision (`resolveRef`), so alias expansion is the natural sibling of that logic.
- **Alias namespace and validation**: an alias is `[a-zA-Z0-9_-]+`, case-sensitive, unique across instances. `auth add --alias` rejects a name already bound to a different instance (with an actionable error) and rejects a malformed name. Re-adding the same instance with the same alias is idempotent.
- **Collision with `host:port`**: the `<alias>:<id>` form only triggers when the part before `:` is a known alias **and** the part after is purely numeric; `host:port/...` URLs are still parsed as URLs because they contain `/` and `.`/scheme. A bare `name:123` with an unknown `name` is an error suggesting `cfl auth list`.
- **Backward compatibility**: loading a v0.1 credentials file (no aliases) works unchanged; saving re-writes in the extended format. No `schemaVersion` impact (that governs command output, not the credentials file).

### D13. Endpoint mapping addition

Extends the D9 table from the initial change:

| Command | Method & path | Query | Notes |
|---|---|---|---|
| `search` | `GET /rest/api/search` | `cql=<built-or-raw>`, `limit`, `start` (omitted → server defaults) | CQL built from flags or passed via `--cql` |

`page get` on a `/spaces/KEY/pages/ID/Title` URL uses the **existing** `GET /rest/api/content/{id}` path (the ID is in the URL); no new endpoint, no title lookup.

## Risks / Trade-offs

- **CQL injection via the positional term** → Mitigation: the term is always CQL-quoted and escaped (D11); it is never concatenated raw. `--cql` is explicitly the user's own responsibility (documented), like passing raw SQL.
- **Alias vs. URL ambiguity for `--instance`** → A value that is both a valid alias and a plausible hostname is theoretically ambiguous. Mitigation: alias lookup is tried first and only matches exact stored alias names (`[a-zA-Z0-9_-]+` with no dots/scheme/slash); anything containing `://`, `.`, or `/` is treated as a URL. Documented.
- **Credentials format migration** → Adding `alias` changes the TOML shape. Mitigation: the loader accepts both the old (bare string token) and new (table with token+alias) forms; a round-trip test pins backward compatibility.
- **Search result heterogeneity** → `/rest/api/search` can return spaces and other entities, not just pages. Mitigation: `--type` defaults to `page`; the mapper tolerates missing fields with `null` rather than failing, and `docs/schema.md` documents which fields may be null per result.
- **`/spaces/.../pages/...` without a title** → Some links omit the trailing title (`/spaces/KEY/pages/ID`). Mitigation: title is optional in the parse branch; the ID alone fully identifies the page.

## Migration Plan

Additive feature change on top of v0.1. The only on-disk change is the credentials file gaining an optional `alias` field, written in a backward-compatible way (old files load; newly-saved files use the extended form). No output `schemaVersion` bump (all additions are new commands/fields; no `stable` field changes). Rollback is "do not ship the tag"; a v0.1 binary still reads a credentials file written by v0.2 as long as it ignores the unknown `alias` key (the v0.1 loader decodes `key → token`; see the backward-compat task to confirm the encoding tolerates this in both directions).

## Open Questions

_None blocking._ Resolved during proposal:

- Search interface — friendly flags primary, `--cql` escape hatch (chosen).
- Alias scope — instances only, used for `--instance` and `<alias>:<id>` (chosen).
- Alias storage — in the credentials file via `auth add --alias` (chosen).
