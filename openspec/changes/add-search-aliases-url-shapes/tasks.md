# Tasks: add-search-aliases-url-shapes

> Ordered by dependency, TDD throughout (table-driven test first, then code).
> Builds on the v0.1 packages; touches `confluenceurl`, `auth`, `schema`,
> `confluence`, `cli`. English-only per SPEC.md.

## 1. URL shape: spaces/pages

- [x] 1.1 Add table-driven tests to `internal/confluenceurl` for `/spaces/KEY/pages/ID[/Title]`: with title, without title, with context path, trailing slash, fragment, and the non-numeric-id rejection (per the url-resolution spec scenarios).
- [x] 1.2 Add a test asserting a bare `/spaces/KEY` still yields a space-only Ref (no regression).
- [x] 1.3 Extend `Parse`'s `case "spaces"` branch: if the segments after the space key are `pages/{numericID}[/{Title}]`, populate `PageID` (and decoded `Title`); otherwise keep the space-only behavior. Keep the parser network-free.

## 2. Aliases: storage & resolution (auth)

- [x] 2.1 Write tests for the credentials TOML backward-compatible encoding: a v0.1 file (bare `key = "token"`) loads; a v0.2 file (`[tokens.<key>] token=.. alias=..`) round-trips; entries without an alias remain valid.
- [x] 2.2 Write tests for alias rules: `--alias` charset `[a-zA-Z0-9_-]+`; duplicate alias on a different instance rejected; idempotent re-add of the same instance+alias; alias uniqueness across instances.
- [x] 2.3 Write tests for `Store.ResolveAlias(name) → (instanceKey, ok)` and alias surfacing in `List`.
- [x] 2.4 Implement the extended credentials model in `internal/auth` (token + optional alias), the backward-compatible TOML load/save, `Add` with alias validation, `ResolveAlias`, and alias-aware listing. Preserve atomic write + `0600`.

## 3. Alias resolution in the CLI

- [x] 3.1 Write tests in `internal/cli` for instance resolution: `--instance <alias>` expands to the instance base URL+context; `--instance <url>` stays a URL; a value with `://`/`.`/`/` is never treated as an alias.
- [x] 3.2 Write tests for the `<alias>:<id>` target form: known alias + numeric id resolves the instance (valid with multiple instances configured); unknown alias prefix errors with a `cfl auth list` suggestion; a URL is not mistaken for the form.
- [x] 3.3 Implement an alias pre-pass: resolve `--instance` values and `<alias>:<id>` targets before URL parsing (`resolveRef`/`refForInstance` siblings), using the auth store. Keep `confluenceurl` alias-unaware.
- [x] 3.4 Write tests for the page instance-selection rules (page spec "Select the target instance"): bare ID + multiple instances + no `--instance` errors; bare ID + `--instance <url|alias>` resolves; bare ID + single instance needs no flag; a full URL ignores `--instance`; `<alias>:<id>` selects its instance.
- [x] 3.5 Implement `resolveTarget(arg, instance, store)` encoding those rules and refactor page `prepare` to resolve the Ref first (then check the credential against the resolved host), fixing the `<alias>:<id>` credential-lookup path. Add the `--instance` flag to `page get/update/delete/children`.

## 4. auth add/list wiring for aliases

- [x] 4.1 Add the `--alias` flag to `cfl auth add`; on success name the alias in the confirmation (never the token); reject duplicate/malformed aliases before writing.
- [x] 4.2 Extend the `auth list` output (schema + command) to include each instance's alias (null when absent); keep tokens out of the output.
- [x] 4.3 Write cli integration tests (httptest where needed) for `auth add --alias`, the duplicate-alias error, and `auth list` showing aliases.

## 5. Search: schema + client

- [x] 5.1 Define `schema.SearchResults` (`results[]{id,title,type,spaceKey,url}` + `start`/`limit`/`size`) with camelCase json tags and null-able fields where the search record may omit data.
- [x] 5.2 Write table-driven `MapSearch` tests from recorded `/rest/api/search` JSON fixtures: page hits, mixed/absent fields mapped to null (not dropped), empty results → empty slice (not null), and that Confluence internals are not leaked.
- [x] 5.3 Implement `MapSearch` as a pure function in `internal/schema`.
- [x] 5.4 Write integration tests for `confluence.Client.Search(ctx, cql, limit, start)` asserting the endpoint, `cql`/`limit`/`start` query params (omit zero limit/start), and raw-bytes return (per D13).
- [x] 5.5 Implement `Client.Search` returning raw `[]byte`.

## 6. Search: CQL builder + command

- [x] 6.1 Write tests for the CQL builder: positional text → quoted/escaped `text ~ "..."`; `--space` → `space = "KEY"`; `--type` → `type = page|blogpost` (default page); AND-joined; and that a term containing quotes/operators is escaped, not injected (per the search spec injection scenario).
- [x] 6.2 Write tests for `--cql` precedence: when `--cql` is set it overrides text/`--space`/`--type` (those are ignored, no error), a stderr note records the override, and `--limit`/`--start` still apply with `--cql`.
- [x] 6.3 Implement `internal/cli/search.go`: build CQL or take `--cql`, resolve the instance (URL or alias, single-instance default, onboarding on missing credential), call `Client.Search`, map via `MapSearch`, render; support `-o raw`.
- [x] 6.4 Write cli integration tests for `cfl search` happy path (within a space), default type, pagination params, empty results, `--cql` passthrough, and `--cql` overriding friendly flags (text/`--space`/`--type` ignored).

## 7. Wire `cfl search` + register

- [x] 7.1 Register `newSearchCmd(deps)` on the root command; add `--space`, `--type`, `--limit`, `--start`, `--cql`, `--instance` flags.
- [x] 7.2 Verify `page get` on a `/spaces/KEY/pages/ID/Title` URL reads by the ID in the URL (no title lookup), end to end.

## 8. Docs

- [x] 8.1 Update `docs/schema.md`: add the `search` output field set (with `stable`/`experimental` tags and per-field null notes) and the `alias` field on `auth list`.
- [x] 8.2 Update `README.md`: add a Search section (friendly flags + `--cql`), an alias example (`auth add --alias`, `--instance prod`, `prod:12345`), and the `/spaces/KEY/pages/ID/Title` row in the URL-conventions table.

## 9. Verify

- [x] 9.1 `make test` green with `-race`; the 5 core packages stay ≥ 70% (add tests if the new code dips any below).
- [x] 9.2 `make lint` zero warnings; `gofmt` clean.
- [x] 9.3 Add e2e coverage (behind `-tags=e2e`): `TestE2E_Search` (keyword + `--space`), `TestE2E_Page_get_spaces_pages_url`, and `TestE2E_Alias_roundtrip` (add with `--alias`, use `--instance <alias>` and `<alias>:<id>`). Skip without `CFL_E2E_*`. _(Tests written and compiling under `-tags=e2e`; awaiting a manual run against a licensed Confluence. The same flows are covered automatically by the httptest-based cli integration tests.)_
- [x] 9.4 `openspec validate add-search-aliases-url-shapes --strict` passes.
