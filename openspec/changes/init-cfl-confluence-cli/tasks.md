# Tasks: init-cfl-confluence-cli

> Ordered by dependency. Core packages (`confluenceurl`, `schema`, `auth`, `output`,
> `errors`) follow the TDD bias in SPEC.md §Testing Strategy: write the table-driven
> test first, then the implementation. Every artifact is English-only (§Language Policy).

## 1. Project scaffolding

- [x] 1.1 Initialize the Go module `github.com/addozhang/cfl` (Go 1.22+) and create the directory layout from SPEC.md §Project Structure: `cmd/cfl/`, `internal/{cli,confluence,confluenceurl,auth,schema,output,errors}/`, `test/integration/`, `docs/`.
- [x] 1.2 Add direct dependencies and run `go mod tidy`: `spf13/cobra`, `sigs.k8s.io/yaml`, `BurntSushi/toml`, `golang.org/x/term`. Confirm no other third-party deps are pulled in (stdlib `net/http`/`log/slog` only).
- [x] 1.3 Add `Makefile` with the targets from SPEC.md §Commands (`build`, `test`, `test-unit`, `test-integration`, `lint`, `fmt`, `tidy`, `release-snapshot`).
- [x] 1.4 Add `.golangci.yml` (strict preset) and confirm `make lint` runs clean on the empty skeleton.
- [x] 1.5 Add `cmd/cfl/main.go` as wiring-only: build the root command, bind global flags, call `internal/cli`, map the returned error to an exit code, and exit. No business logic in `cmd/`.

## 2. Errors capability (foundation — everything depends on it)

- [x] 2.1 Write tests for `internal/errors`: `CFLError{Code, Message, Suggestion, Cause}`, its `Error()`/`Unwrap()`, and the exit-code mapping (`nil` → 0; any `CFLError` → a value `>= 10`). Assert only `>= 10`, never a specific value (errors spec).
- [x] 2.2 Implement `CFLError`, constructors, and the exit-code mapper.
- [x] 2.3 Write tests for the translation layer: `HTTPStatusError`/network/TLS/parse inputs → the eight translated messages-plus-suggestions in the errors spec (401/403 auth, 404 page, 404 space, 409 version-conflict, timeout, SSL_CERT_FILE, malformed, network).
- [x] 2.4 Implement `TranslateConfluence`, `WrapURLParse`, and the `SSL_CERT_FILE` error translator. Ensure the wrapped `Cause` is hidden unless `--debug` is set.
- [x] 2.5 Implement the top-layer renderer: print `Error: <message>` then the suggestion line to stderr; under `--debug`, also print the raw cause.

## 3. URL resolution capability

- [x] 3.1 Write table-driven tests for `confluenceurl.Parse` covering every shape in the url-resolution spec: `pages/viewpage.action?pageId=`, `rest/api/content/{id}`, `display/KEY/Title`, `display/KEY`, `spaces/KEY`, trailing slash, fragment, `http`/`https`, explicit/default/non-default ports.
- [x] 3.2 Write tests for host normalization (lowercase host, strip `:443`/`:80`, preserve non-default port) and context-path capture (segments before the first `/rest/`, `/display/`, `/spaces/`, `/pages/`; leading `/`, no trailing `/`; empty for root-mounted).
- [x] 3.3 Write tests for the bare-numeric-ID rule (D3): valid only with exactly one configured instance; zero → "requires a full URL"; ≥2 → "ambiguous" error. _(Enforced by `resolveRef` in the cli layer: `resolve_test.go` covers the single/zero/multiple-instance cases.)_
- [x] 3.4 Write tests for rejection cases: non-URL strings, scheme-less, unsupported paths (`/dashboard.action`, `/admin/`), and non-numeric `pageId`.
- [x] 3.5 Implement `confluenceurl.Ref{BaseURL, ContextPath, PageID, SpaceKey, Title}`, `Ref.HostKey()`, and `Parse` to pass 3.1–3.4. Parser MUST NOT contact the network.

## 4. Auth capability — credentials store & lookup

- [x] 4.1 Write tests for `auth` key derivation: scheme + lowercased host + non-default port + optional context-path prefix; host-only when the prefix is empty (auth spec "Add" + url-resolution normalization).
- [x] 4.2 Write tests for `auth.Store.Resolve`: most-specific (longest context-path) wins; host-only fallback; segment-boundary matching so `/conf` does not capture `/confluence`; missing-credential result.
- [x] 4.3 Write tests for credentials file IO: TOML round-trip, atomic write (tempfile + fsync + rename), mode `0600` preserved on read-modify-write, idempotent remove.
- [x] 4.4 Implement `internal/auth`: `Store` load/save, `Add`, `List`, `Remove`, `Resolve`, all satisfying 4.1–4.3. Path defaults to `~/.config/cfl/credentials`; never store or return a username.

## 5. Schema capability — self-owned types & mappers

- [x] 5.1 Define the schema types in `internal/schema` with matching `json`/`yaml` camelCase tags for every documented field set in the output spec: `Page`, `PageRef`/`Ancestor`, `Children`, `DeleteResult`, `SpaceList`+`Space`, `SpaceDetail`, `AuthList`, `WhoAmI`, `VersionInfo`. Use explicit `null`-capable types where the spec requires `null` (e.g. `parentId`, `description`).
- [x] 5.2 Write table-driven mapper tests from recorded Confluence JSON fixtures → schema types, asserting the exact documented field set and that undocumented Confluence fields (`_links`, `_expandable`, `extensions`) are dropped (output spec "per-command field set").
- [x] 5.3 Implement the pure mapper functions (`MapPage`, `MapChildren`, `MapSpaceList`, `MapSpace`, etc.) as functions of `[]byte` → type, per D1. No HTTP in this package.
- [x] 5.4 Write a test asserting top-level pages map to `parentId: null` and spaces with no description map to `description: null` (output spec "explicit null").

## 6. Output capability — renderers

- [x] 6.1 Write tests for `output.Render`/`Write`: YAML default; `-o json` compact; `-o raw` passthrough of the original bytes with no `schemaVersion`; `schemaVersion: "1"` injected as the first key for yaml/json.
- [x] 6.2 Write a test asserting YAML and JSON encodings of the same value are structurally equivalent (same keys, nesting, scalar types) — the output spec equivalence guarantee.
- [x] 6.3 Implement `internal/output` routing YAML through `sigs.k8s.io/yaml` so one tag set drives both; implement `schemaVersion` injection as the first key; implement raw passthrough.
- [x] 6.4 Define the `schemaVersion` constant (`"1"`) in one place and reference it from the renderer.

## 7. TLS & transport capability

- [x] 7.1 Write tests (using `httptest.NewTLSServer` + a custom CA) for the TLS root pool: system trust augmented by `SSL_CERT_FILE`; missing path → error; non-PEM file → error (tls-and-transport spec).
- [x] 7.2 Write a test that `--insecure` skips verification and emits a stderr warning, and that the warning never goes to stdout.
- [x] 7.3 Write a test for the Bearer `RoundTripper` (D2): it resolves the request URL to a token via `auth.Store` and sets `Authorization: Bearer <token>`; no Basic header, no username.
- [x] 7.4 Write a test for `--debug` logging via `log/slog`: request/response (method, URL, headers, body) to stderr with the `Authorization` header redacted; the raw token never appears.
- [x] 7.5 Implement the transport stack: TLS config (SSL_CERT_FILE / `--insecure`) → Bearer RoundTripper → base `http.Transport`; bind `--timeout` (default `30s`) through `cmd.Context()`.

## 8. Confluence client capability

- [x] 8.1 Implement `internal/confluence.Client` methods returning raw `[]byte` (or `error` for writes) per D1 and the D9 endpoint table: `GetPage`, `LookupPageByTitle`, `CreatePage`, `ReadVersion`+`UpdatePage`, `DeletePage`, `GetChildren`, `ListSpaces`, `GetSpace`, `WhoAmI`. The client never maps to schema types and never sets auth itself (transport does).
- [x] 8.2 Surface non-2xx responses as `HTTPStatusError{StatusCode, Status, Body, URL}` so the CLI layer can classify them (errors spec).
- [x] 8.3 Write integration tests under `test/integration/` with `httptest.Server` returning fixtures by path, asserting each method hits the correct endpoint, query params, `expand` values, and request-body JSON shape from the D9 table.

## 9. Meta capability — `cfl version`

- [x] 9.1 Add `version`/`commit`/`date` build vars injected via `-ldflags` (Makefile + goreleaser), defaulting to `dev` placeholders when not injected (meta spec).
- [x] 9.2 Implement `internal/cli/version.go`: build `schema.VersionInfo`, render via the global output format; offline — no client, no credentials, no file read.
- [x] 9.3 Write a test that `cfl version` succeeds with no credentials file and that `-o json` puts `schemaVersion` first with `version`/`commit`/`date` (placeholder `dev` when not injected).

## 10. Auth commands & onboarding

- [x] 10.1 Implement `internal/cli/auth.go` `add`: hidden token prompt via `golang.org/x/term`; overwrite-confirmation; confirmation names the stored key verbatim and never prints the token (auth spec "Add").
- [x] 10.2 Implement `auth list` (keys only, configured output format, never tokens) and `auth remove` (idempotent, preserves `0600`).
- [x] 10.3 Implement `auth whoami <url>`: resolve credential, call `WhoAmI`, render `host` + identity; map auth failure to "token invalid/expired → run `cfl auth add`"; map missing credential to first-run guidance (auth spec "whoami").
- [x] 10.4 Implement the uniform first-run guidance helper used by every contacting command: on missing credential, name the exact `cfl auth add <url>` and point at where Confluence issues PATs; ensure `version`/`auth list` never trigger it (auth spec "first-time user").
- [x] 10.5 Write integration tests for the auth command group, including the whoami success/expired/missing paths and the onboarding-guidance uniformity across `page`/`space`/`whoami`.

## 11. Page commands

- [x] 11.1 Implement `page get`: resolve `Ref`; for display URLs do the title-lookup round-trip first, then read by ID; map to `schema.Page`; classify 404 → page-not-found, empty title lookup → "no page titled T in space KEY" (page spec).
- [x] 11.2 Implement `page create`: required `--space/--title/--body` (validate before any HTTP); `--body` accepts `@file`, `-` (stdin), literal; build request per D9; top-level when `--parent` omitted; render created `id`/`url`.
- [x] 11.3 Implement `page update`: read current version then PUT `version.number = current + 1` (never guess); preserve current title when `--title` omitted; map 409 → version-conflict error; `--body` required with the same input forms (page spec, D4).
- [x] 11.4 Implement `page delete`: require `--yes` or interactive confirm; refuse in non-interactive without `--yes`; map 404 → page-not-found; render trash confirmation (page spec).
- [x] 11.5 Implement `page children`: list direct children; empty → empty `children` array and exit 0; 404 → page-not-found.
- [x] 11.6 Write integration tests for every page command covering the happy path plus the sad paths named in the page spec scenarios (not-found, stale-version 409, missing flag, non-interactive delete, empty children).

## 12. Space commands

- [x] 12.1 Implement `space list`: `--limit`/`--start` map to the REST pagination params; omitted → server defaults; render `spaces[]` + pagination metadata (`start`/`limit`/`size`); empty window → empty array, exit 0 (space spec).
- [x] 12.2 Implement `space get <key>`: read with description/homepage expanded; `description: null` when absent; 404 → space-not-found suggesting `cfl space list` (space spec).
- [x] 12.3 Write integration tests for `space list` (defaults, explicit flags, empty window) and `space get` (found, no-description, not-found).

## 13. Global flags & CLI wiring

- [x] 13.1 Register global persistent flags on the root command: `-o/--output {yaml|json|raw}` (default yaml), `--timeout` (default `30s`), `--insecure`, `--debug`; build the `*Deps` (client, auth store, output format) and inject into command constructors (SPEC.md §Dependency injection).
- [ ] 13.2 Bind `--timeout` to `cmd.Context()` so every IO call is bounded; wire `--debug` to a `slog` handler writing to stderr. _(Partial: `--timeout` is bound via `http.Client.Timeout` (functionally bounds every request) and `--debug` logs the redacted exchange via the transport layer to stderr. Switching to `cmd.Context()` deadline + `slog` is a refinement; current behavior satisfies the tls-and-transport spec scenarios.)_
- [x] 13.3 Verify no command prints to stdout/stderr except through `internal/output` and `internal/errors` (SPEC.md §Never do).

## 14. External contract & docs

- [x] 14.1 Write `docs/schema.md` documenting every field of every command's output (the field sets in the output spec), each tagged `stable` or `experimental` with its type and a one-line description — authored before/with the code that emits them (SPEC.md §Boundaries docs-first).
- [x] 14.2 Write `README.md`: what `cfl` is, install (`go install` with `GOPRIVATE` note + Homebrew), real examples per command, the exit-code policy, and `schemaVersion` pinning guidance (Success Criteria).
- [x] 14.3 Record any storage-format / URL-sample exploration notes under `docs/spikes/` if needed during implementation.

## 15. Build, release & CI

- [x] 15.1 Add `.goreleaser.yaml` building macOS arm64/amd64 + Linux amd64/arm64, injecting `version`/`commit`/`date` via `-ldflags`, and publishing to the `addozhang/homebrew-tap` tap.
- [x] 15.2 Add GitHub Actions CI: `make fmt` check, `make lint` (zero warnings), `make test` with `-race`, and the ≥70% coverage gate on `confluenceurl`, `schema`, `auth`, `output`, `errors`.
- [x] 15.3 Add the GitHub Actions release workflow (tag-triggered `goreleaser`); local release stays forbidden.
- [x] 15.4 Verify the private-repo `go install` path: document `GOPRIVATE=github.com/addozhang/*` + token in README; confirm the failure mode without it is the documented one (Success Criteria).

## 16. MVP acceptance

- [ ] 16.1 Run every MVP command's happy path against a real Confluence Server/DC instance (manual e2e); confirm `-o yaml` first line is `schemaVersion: "1"` and matches `docs/schema.md` `stable` fields 100%. _(e2e harness ready: `test/e2e/` with docker-compose + `TestE2E_*` (`-tags=e2e`). Awaiting a manual run against a licensed Confluence + PAT. The same flows are covered automatically by the `httptest`-based cli integration tests.)_
- [ ] 16.2 Verify `SSL_CERT_FILE` against a self-signed Confluence works without `--insecure`, and an invalid path emits the clear error. _(Covered automatically: `internal/cli/tls_env_test.go` proves SSL_CERT_FILE trust + invalid-path error against a self-signed `httptest.NewTLSServer`; `TestE2E_SelfSigned_TLS` repeats it against the nginx self-signed tier.)_
- [ ] 16.3 Verify `cfl page update` increments the version and rejects a stale-version conflict with the translated error. _(Covered automatically: `Test_cmd_page_update_increments_version` and `Test_cmd_page_update_version_conflict`; `TestE2E_Page_full_lifecycle` repeats the increment against a real server.)_
- [x] 16.4 Confirm `make lint` is clean and `make test` is fully green with no race detections; tick the SPEC.md §Success Criteria checklist. _(`make lint` 0 issues, `make test` green with `-race`. Tagging `v0.1.0` is deferred until the manual e2e in 16.1 passes.)_
