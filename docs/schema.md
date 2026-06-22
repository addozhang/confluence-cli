# cfl Output Schema

This document is the **authoritative external contract** for `cfl`'s structured
output (`-o yaml` / `-o json`). Scripts may depend on every field tagged
`stable`. Fields tagged `experimental` may change without notice within a major
schema version.

## Schema versioning

Every structured response begins with a top-level `schemaVersion` string.

| Field | Type | Tier | Description |
|---|---|---|---|
| `schemaVersion` | string | stable | Schema major version. Current value: `"1"`. The first key of every yaml/json response. Absent under `-o raw`. |

**Compatibility promise.** A `stable` field will not be removed, renamed, or
have its type changed without bumping `schemaVersion`. Adding a new optional
field is not a breaking change. `experimental` fields carry no such promise.

**Missing values.** Any field whose value is unavailable is emitted as `null`,
never omitted. A field documented for a command is always present on that
command's output.

**`-o raw`.** Bypasses this schema entirely and prints the verbatim Confluence
REST response body; `schemaVersion` is not injected. Use it to reach fields this
schema does not expose.

---

## `cfl version`

Offline; reports build metadata.

| Field | Type | Tier | Description |
|---|---|---|---|
| `version` | string | stable | Build version (e.g. `v0.1.0`, or `dev` when not injected). |
| `commit` | string \| null | stable | Build commit revision, or `null` when not injected. |
| `date` | string \| null | stable | Build date (ISO 8601 UTC), or `null` when not injected. |

## `cfl auth list`

| Field | Type | Tier | Description |
|---|---|---|---|
| `instances` | string[] | stable | Configured instance keys (`scheme://host[:port][/contextpath]`). Empty array when none. Never includes any token. |

## `cfl auth whoami <url>`

| Field | Type | Tier | Description |
|---|---|---|---|
| `host` | string | stable | The resolved instance key the token was verified against. |
| `username` | string | stable | The authenticated user's username. |
| `displayName` | string \| null | stable | The user's display name, or `null` when the instance does not provide one. |

## `cfl page get <url-or-id>`

| Field | Type | Tier | Description |
|---|---|---|---|
| `id` | string | stable | Numeric page ID (as a string). |
| `title` | string | stable | Page title. |
| `spaceKey` | string | stable | Key of the space containing the page. |
| `version` | number | stable | Current version number. |
| `parentId` | string \| null | stable | Immediate parent page ID, or `null` for a top-level page. |
| `body` | string \| null | stable | Body in Confluence storage format (XHTML), unconverted, or `null` when absent. |
| `ancestors` | object[] | stable | Ancestor chain, root first. Each entry: `{ id: string, title: string }`. |
| `url` | string \| null | stable | Absolute page URL, or `null` when the instance does not provide one. |

## `cfl page create` / `cfl page update`

| Field | Type | Tier | Description |
|---|---|---|---|
| `id` | string | stable | Created/updated page ID. |
| `title` | string | stable | Page title. |
| `spaceKey` | string | stable | Space key. |
| `version` | number | stable | New version number (incremented for updates). |
| `url` | string \| null | stable | Absolute page URL, or `null`. |

> `body` and `ancestors` are intentionally **not returned** by create/update;
> run `cfl page get` to read them.

## `cfl page children <url-or-id>`

| Field | Type | Tier | Description |
|---|---|---|---|
| `children` | object[] | stable | Direct child pages. Empty array when none. Each entry: `{ id: string, title: string, version: number }`. |

## `cfl page delete <url-or-id>`

| Field | Type | Tier | Description |
|---|---|---|---|
| `id` | string | stable | The page ID that was moved to trash. |
| `status` | string | stable | Confirmation status; currently `"trashed"`. |

## `cfl space list`

| Field | Type | Tier | Description |
|---|---|---|---|
| `spaces` | object[] | stable | Spaces in the requested window. Empty array when none. Each entry: `{ key: string, name: string, type: string }`. |
| `start` | number | stable | Pagination start offset of this page. |
| `limit` | number | stable | Pagination limit of this page. |
| `size` | number | stable | Number of spaces returned in this page. |

The `type` enum is an uppercase-or-lowercase string as returned by Confluence
(e.g. `global`, `personal`); `cfl` passes it through unchanged.

## `cfl space get <key>`

| Field | Type | Tier | Description |
|---|---|---|---|
| `key` | string | stable | Space key. |
| `name` | string | stable | Space name. |
| `type` | string | stable | Space type (e.g. `global`, `personal`). |
| `description` | string \| null | stable | Plain-text description, or `null` when the space has none. |

---

## Fields cfl deliberately drops

The schema layer never surfaces these Confluence internals in yaml/json output;
reach them with `-o raw`:

`_links`, `_expandable`, `extensions`, `minorEdit`, `representation`, and any
field not listed above.
