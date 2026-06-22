## Why

Confluence Server / Data Center users who live in the terminal have no first-class way to read, create, or update pages without opening a browser or hand-rolling `curl` against the REST API. `cfl` makes the Confluence base URL the unit of identity — paste the URL you'd open in a browser, and the CLI resolves credentials, fetches the page in a stable schema, and lets you edit it with version-safe updates.

## What Changes

- **New `cfl` CLI** built in Go, organized around Confluence Server/DC concepts, distributed via `go install` and Homebrew.
- **URL-as-identity model**: every page/space command accepts a Confluence URL (display URL, `pageId` URL, or REST URL); the host (plus optional context path) selects the stored credential.
- **PAT Bearer authentication**: credentials are Personal Access Tokens sent as `Authorization: Bearer <token>`; no username is stored or transmitted.
- **Token verification + onboarding**: `cfl auth whoami <url>` confirms a stored token is valid and unexpired; any contacting command run with no matching credential responds with uniform first-run guidance naming the exact `cfl auth add <url>` to run.
- **Offline `cfl version`**: reports build version/commit/date, needs no credentials, and contacts no instance.
- **Stable self-owned output schema**: `-o yaml|json|raw`, YAML by default, every structured response begins with `schemaVersion: "1"`, isolating users from Confluence version drift. Each command's exact field set is pinned in `docs/schema.md`; output emits exactly the documented fields, never undocumented Confluence internals.
- **Version-aware page updates**: `cfl page update` reads the current version and submits `version.number = current + 1`, never guessing; stale-version conflicts surface as a clear error.
- **Translated, actionable errors**: every failure (bad URL, auth rejected, not found, version conflict, TLS, timeout, malformed response) is rendered as a one-sentence message plus a suggested next step, with `--debug` for the raw exchange.
- **Behavior-first specs**: capability scenarios assert user-observable behavior; the exact REST endpoint each command calls is an implementation contract recorded in `design.md` (D9) and verified by integration fixtures, keeping product specs stable across Confluence-version endpoint drift.

### Not Doing (non-goals)

- **No Confluence Cloud support** — Cloud uses a different auth model and the v2 REST API; Server/DC only for the MVP.
- **No CQL search** — deferred to v0.2.
- **No attachment, comment, label, or user-management commands** — page + space read/write is the MVP surface. `cfl auth whoami` is the only identity touchpoint, and it is read-only (no user listing or management).
- **No content-format conversion** — bodies are passed through as Confluence storage format (XHTML); no Markdown/wiki conversion.
- **No auto-pagination** — `space list` returns a single bounded `--limit`/`--start` window; walking every page of an instance is the caller's job.
- **No bulk or recursive operations** — every command acts on exactly one page or space; no recursive tree delete, no batch create.
- **No output field beyond the documented schema** — anything not in `docs/schema.md` is reachable only via `-o raw`; the schema layer never leaks Confluence internals (`_links`, `_expandable`, `extensions`).
- **No TUI, AI features, notifications, daemons, or watch loops.**
- **No credential encryption / OS keychain** — plaintext PAT at `~/.config/cfl/credentials` mode `0600`, matching `~/.aws/credentials` posture; deferred to v0.2.

## Capabilities

### New Capabilities

- `auth`: Store, list, and remove per-instance Personal Access Tokens; verify a stored token via `cfl auth whoami`; resolve a request URL to the most specific stored credential (host + optional context-path longest-prefix match); send the token as an HTTP Bearer header; guide first-run users with no configured credential.
- `page`: Read a page (with body.storage + version + ancestors), create a page under a space/parent, update a page version-safely, delete (trash) a page, and list a page's direct children — all addressed by Confluence URL or numeric page ID.
- `space`: List spaces (paginated) and read a single space by key.
- `meta`: Report the build version offline (`cfl version`) with version/commit/date, requiring no credentials and contacting no instance.
- `url-resolution`: Parse the accepted Confluence URL shapes (display URL, `pages/viewpage.action?pageId=`, REST `content/{id}`, space URL) and a bare numeric ID into a `Ref` carrying base URL, context path, page ID, and space key; normalize host (lowercase, default-port strip) for credential lookup.
- `output`: Render schema values as YAML, JSON, or raw bytes; inject `schemaVersion` for yaml/json; pin each command's exact field set to `docs/schema.md`; guarantee yaml and json encodings of the same value are equivalent.
- `errors`: Translate low-level HTTP/network/parse failures into user-facing `CFLError` values with a message + suggestion; define the process exit-code policy (0 success; ≥10 cfl-level failure).
- `tls-and-transport`: Honor `SSL_CERT_FILE` for custom CAs, support `--insecure` (with a stderr warning), bound every request with `--timeout`, and log redacted request/response pairs under `--debug`.

### Modified Capabilities

<!-- None; this is the initial change. openspec/specs/ is empty. -->

## Impact

- **New module** `github.com/addozhang/cfl` with `cmd/cfl` + `internal/{cli,confluence,confluenceurl,auth,schema,output,errors}`.
- **New external contract** `docs/schema.md` defining every output field, its type, and its stability tier, pinned per command (see the `output` capability).
- **New commands beyond page/space**: `cfl version` (offline build info) and `cfl auth whoami <url>` (token verification via the current-user endpoint).
- **Dependencies**: `spf13/cobra`, `sigs.k8s.io/yaml`, `BurntSushi/toml`, `golang.org/x/term` (hidden token prompt); stdlib `net/http`/`log/slog` otherwise.
- **Credentials file** created at `~/.config/cfl/credentials` (TOML, mode `0600`).
- **REST endpoint contract** recorded in `design.md` (D9) as the normative wire mapping; specs assert observable behavior and integration fixtures assert the endpoints.
- **Build/release**: Makefile, `.golangci.yml`, `.goreleaser.yaml`, GitHub Actions CI + release, Homebrew tap `addozhang/homebrew-tap`. Version/commit/date are injected at build time via `-ldflags` for `cfl version`.
