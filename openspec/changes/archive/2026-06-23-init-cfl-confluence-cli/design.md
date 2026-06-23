## Context

`cfl` is the initial build of a URL-native Confluence Server/Data Center CLI. The project constitution (`SPEC.md`) fixes the toolchain (Go 1.22+, cobra, stdlib `net/http`), the layout (`cmd/cfl` wiring + `internal/*` business code), and the boundaries. This document records the cross-cutting technical decisions that the seven MVP capabilities (`auth`, `page`, `space`, `url-resolution`, `output`, `errors`, `tls-and-transport`) share, so that implementation can proceed without re-litigating architecture per command.

The design borrows the proven shape of a sibling project (`jk`, a Jenkins CLI by the same author): a mapping-free HTTP client returning raw bytes, a self-owned schema layer, kubectl-style output, and a translated error layer. The Confluence-specific deltas — Bearer auth, version-aware writes, storage-format bodies, and Confluence URL shapes — are the substance of the decisions below.

## Goals / Non-Goals

**Goals:**

- A single, testable architecture where `internal/confluence` issues HTTP and returns raw `[]byte`, and `internal/schema` maps those bytes into the self-owned output types as pure functions.
- URL-as-identity: one `confluenceurl.Ref` type that every command resolves its argument into, carrying base URL, context path, page ID, and space key.
- Version-safe page updates that read-then-write, never guessing the version number, and surface a stale-version conflict as an actionable error.
- Credential resolution by longest context-path prefix so one host can serve several reverse-proxy-mounted Confluence instances.
- Deterministic, fixture-driven tests (`httptest.Server`) with no real-network dependency.

**Non-Goals:**

- Confluence Cloud (different auth + v2 API).
- CQL search, attachments, comments, labels, user management (post-MVP). `auth whoami` is read-only identity verification, not user management.
- Markdown/wiki ↔ storage-format conversion (bodies pass through verbatim).
- Credential encryption / OS keychain (plaintext `0600`, deferred).
- Auto-pagination, bulk, or recursive operations — every command acts on one page/space and one bounded result window.
- Emitting any output field not pinned in `docs/schema.md` through the schema layer; Confluence internals are reachable only via `-o raw`.
- Any concurrency beyond what `net/http` provides; commands are sequential.

## Decisions

### D1. HTTP client returns raw bytes; mapping is a separate pure layer

`internal/confluence.Client` methods return `([]byte, error)` (or `error` for writes), never schema types. `internal/schema` owns JSON→type mapping as pure functions of the input bytes.

- **Why**: keeps fixtures trivial to record and replay; mapping logic is unit-testable without HTTP; the schema layer is the single place that isolates users from Confluence field drift.
- **Alternative considered**: a client that returns typed structs directly. Rejected — it couples wire decoding to HTTP, makes the schema contract harder to test in isolation, and mirrors the exact coupling `jk` deliberately avoided.

### D2. Bearer authentication via a custom `http.RoundTripper`

Auth is injected at the transport layer: a `RoundTripper` wrapper resolves the request URL to a stored PAT (via `auth.Store.Resolve`) and sets `Authorization: Bearer <token>`. No username is involved.

- **Why**: Confluence Server/DC PATs carry user identity; Bearer is the documented scheme. Putting it in the transport means every client method is auth-agnostic and the token is set exactly once, in one place that also handles redaction under `--debug`.
- **Alternative considered**: set the header per-method in the client. Rejected — error-prone (easy to forget on a new endpoint) and duplicates the credential lookup.
- **Contrast with `jk`**: Jenkins uses Basic auth + a CSRF crumb RoundTripper. `cfl` drops crumb handling entirely (PAT REST calls do not require XSRF), so the transport stack is simpler: TLS config → Bearer → base `http.Transport`.

### D3. One `Ref` type, produced by `confluenceurl.Parse`

Every page/space command turns its first argument into a `confluenceurl.Ref{BaseURL, ContextPath, PageID, SpaceKey, Title}`. `Parse` accepts the documented URL shapes (`pages/viewpage.action?pageId=`, `display/KEY/Title`, `spaces/KEY`, `rest/api/content/{id}`) and a bare numeric ID.

- **Why**: a single resolution point means commands never string-munge URLs; the credential lookup, context-path preservation, and host normalization all live behind `Ref`.
- **Bare-numeric-ID rule**: a bare `12345` has no host, so the base URL must come from the credential store. Decision: a bare ID is valid **only when exactly one instance is configured** (zero → "requires a full URL"; two or more → "ambiguous, pass a full URL"). This is deterministic and testable without inventing an `--instance` flag.
- **Display-URL pages**: `display/KEY/Title` identifies a page by space + title, not by ID. `page get` on such a URL resolves the ID via `GET /rest/api/content?spaceKey=KEY&title=Title` before fetching the body.

### D4. Version-aware updates are read-then-write inside the command

`page update` performs two calls: `GET /content/{id}?expand=version` to read the current `version.number`, then `PUT /content/{id}` with `version.number = current + 1`. The client never defaults or guesses the version.

- **Why**: Confluence rejects a `PUT` whose version is not exactly current+1 with HTTP 409; reading first is the only correct way. Doing it in the command (not hidden in the client) keeps the client a thin HTTP surface and makes the two-step flow explicit and testable.
- **Conflict handling**: a 409 on the PUT maps to a `version_conflict` `CFLError` — "page changed since you last read it" — suggesting the user re-run `cfl page get` and retry. This is the expected outcome of a concurrent edit, not a bug.
- **Trade-off**: a tiny TOCTOU window exists between the GET and the PUT; if someone edits in that window, the PUT 409s and the user retries. Acceptable for a CLI; locking is out of scope.

### D5. Output layer injects `schemaVersion`; `sigs.k8s.io/yaml` unifies encodings

`internal/output.Render(v, format)` marshals via `encoding/json` (YAML routed through JSON by `sigs.k8s.io/yaml`), so a single set of `json:` struct tags drives both YAML and JSON. `schemaVersion: "1"` is injected as the first key for yaml/json and never for raw.

- **Why**: guarantees YAML and JSON of the same value are structurally identical (a spec requirement); one tag set, one code path.
- **`-o raw`**: prints the verbatim Confluence REST response body (the `[]byte` from the client) with no schemaVersion — the escape hatch for fields the schema does not expose.

### D6. Errors are translated at the top, wrapped at the bottom

Lower layers wrap with `fmt.Errorf("doing X: %w", err)`. The CLI layer translates recognized failures into `*errors.CFLError{Code, Message, Suggestion, Cause}`. `main` renders `Error: <message>` then the suggestion line and maps the error to an exit code.

- **Exit-code policy**: `0` success; `>= 10` cfl-level failure. Unlike `jk` (which has build-result codes 0–4 for `--watch`), `cfl` has no command-result codes — there is no long-running watch. Tests assert `>= 10`, never a specific value.
- **HTTP status → error**: the client surfaces non-2xx as `HTTPStatusError{StatusCode, Status, Body, URL}`; the CLI classifies 401/403→auth, 404→not-found (page vs space by context), 409→version-conflict, plus timeout/network/TLS/malformed.

### D7. `SSL_CERT_FILE` + `--insecure` for TLS; `--debug` redacts the token

The transport honors `SSL_CERT_FILE` (custom CA PEM bundle), supports `--insecure` (disables verification, prints a stderr warning), bounds every request with `--timeout` (default 30s, via `cmd.Context()`), and under `--debug` logs request/response to stderr with the `Authorization` header redacted.

- **Why**: matches the `~/.kube`/`~/.aws` operational posture; the Bearer token must never leak into debug logs or error output.

### D8. Credentials file: TOML, one entry per instance key, longest-prefix resolution

`~/.config/cfl/credentials` (mode `0600`) stores `key → token` where the key is `scheme://host[:non-default-port]` plus an optional context-path prefix (segments before the first `/rest/`, `/display/`, `/spaces/`, or `/pages/`). `auth.Store.Resolve(reqURL)` picks the longest key whose path is a segment-boundary prefix of the request, falling back to a host-only key.

- **Why**: one host can front several Confluence instances behind a reverse proxy; segment-boundary matching prevents `/team-a` from capturing `/team-amber`.
- **Atomic writes**: tempfile + `fsync` + `rename`, so a crash never leaves a half-written credentials file.

### D9. Specs assert observable behavior; this document owns the REST endpoint contract

The capability specs (`specs/**/*.md`) describe **user-observable behavior** — what command the user runs, what output schema they get back, what state changed, and what error/exit code surfaces. The **exact Confluence REST endpoint each command calls** (path, query parameters, `expand` values, request-body JSON shape) is an implementation contract owned by this `design.md` table below, not by the scenarios.

- **Why**: D1 makes the schema layer the single isolation point against Confluence field/endpoint drift. If a scenario hard-codes `GET /rest/api/content/{id}?expand=body.storage,version,space,ancestors`, then a server-version change to the `expand` grammar forces a *spec* edit, defeating that isolation and conflating "what the product promises" with "how it talks to Confluence today". Scenarios that assert `the page is read with its body, version, and ancestors` survive such churn; the endpoint table absorbs it.
- **What stays in scenarios**: the resolved page **identity path** (e.g. "by `pageId` URL" vs "by display URL") remains observable, because it changes the user-facing flow (a display URL triggers a title-lookup round-trip first). Scenarios may name the *kind* of request ("a title lookup", "a version read", "the update PUT") without pinning its literal URL.
- **What moves here**: the literal endpoints, query parameters, `expand` sets, and request-body field names. The implementation MUST satisfy this table; integration tests (`httptest.Server` fixtures) assert the wire contract, which is the correct layer for endpoint-shape verification.

#### Normative endpoint mapping (MVP)

| Command | Method & path | Query / expand | Request body (writes) |
|---|---|---|---|
| `page get` (by id) | `GET /rest/api/content/{id}` | `expand=body.storage,version,space,ancestors` | — |
| `page get` (by display URL) | `GET /rest/api/content` then `GET …/{id}` | `spaceKey={KEY}&title={T}&expand=body.storage,version` | — |
| `page create` | `POST /rest/api/content` | — | `type=page`, `title`, `space.key`, `body.storage={value,representation:"storage"}`, optional `ancestors:[{id}]` |
| `page update` | `GET /rest/api/content/{id}?expand=version` then `PUT /rest/api/content/{id}` | version read via `expand=version` | `type=page`, `title`, `version.number=current+1`, `body.storage={value,representation:"storage"}` |
| `page delete` | `DELETE /rest/api/content/{id}` | — | — |
| `page children` | `GET /rest/api/content/{id}/child/page` | `expand=version` | — |
| `space list` | `GET /rest/api/space` | `limit`, `start` (omitted → server defaults) | — |
| `space get` | `GET /rest/api/space/{key}` | `expand=description.plain,homepage` | — |
| `auth whoami` | `GET /rest/api/user/current` | — | — |
| `version` | _(offline; no request)_ | — | — |

- **Why a table, not prose**: one normative place to read the full wire surface; a new endpoint or `expand` tweak is a one-row diff here plus its integration fixture, never a scenario rewrite.

## Risks / Trade-offs

- **TOCTOU on version-aware update (D4)** → The GET→PUT window can 409 on a concurrent edit. Mitigation: surface a clear `version_conflict` error telling the user to re-read and retry; no locking.
- **Bare-numeric-ID ambiguity (D3)** → Convenient but only safe with one configured instance. Mitigation: hard error (not a silent guess) when zero or multiple instances exist.
- **Display-URL title lookup (D3)** → Title-based resolution can match zero or multiple pages if titles are non-unique within a space. Mitigation: treat "no match" as page-not-found and "multiple" as an explicit error suggesting a `pageId` URL.
- **Storage-format passthrough (Non-Goal)** → Users must supply valid Confluence XHTML; malformed bodies are rejected by the server. Mitigation: surface the server's 400 as a translated error; document that `--body` is storage format, not Markdown.
- **Schema drift across Confluence versions** → Field shapes may vary by server version. Mitigation: the schema layer is the single isolation point; `-o raw` is always available as an escape hatch; fields are tagged `stable`/`experimental` in `docs/schema.md`.
- **Plaintext token storage** → A reader of the file gets the PAT. Mitigation: `0600` + parent `0700`; same posture as kube/aws configs; encryption deferred with an explicit revisit point.

## Migration Plan

Not applicable — this is the initial implementation; there is no prior version, data, or on-disk format to migrate. The first release is `v0.1.0`. Rollback is "do not ship the tag."

## Open Questions

_None blocking._ Resolved during proposal/spec:

- Cloud support — deferred indefinitely (different auth + API).
- CQL search — deferred to v0.2.
- Credential encryption — deferred to v0.2.
- Bare-numeric-ID semantics — resolved in D3 (single-instance-only).
- Delete confirmation — resolved in the `page` spec (`--yes`, or interactive prompt; refuse in non-interactive without `--yes`).
- Space-list pagination — resolved in the `space` spec (`--limit`/`--start`, single bounded page).
