# cfl — URL-native Confluence CLI

`cfl` is a command-line tool for Confluence **Server / Data Center** that makes
the Confluence URL the unit of identity. Paste the URL you would open in a
browser; `cfl` resolves the credential, fetches the page in a stable schema, and
lets you edit it with version-safe updates.

> **Server / Data Center only.** Confluence Cloud uses a different auth model and
> API surface and is out of scope.

## Features

- **URL as identity** — every page/space command accepts a Confluence URL
  (display URL, `pageId` URL, REST URL) or a bare numeric page ID.
- **PAT Bearer auth** — credentials are Personal Access Tokens sent as
  `Authorization: Bearer <token>`; no username is stored or transmitted.
- **Stable self-owned schema** — `-o yaml|json|raw`, YAML by default, every
  structured response begins with `schemaVersion: "1"`. See
  [`docs/schema.md`](docs/schema.md).
- **Version-safe updates** — `cfl page update` reads the current version and
  submits `current + 1`, never guessing; stale-version conflicts are a clear
  error.
- **Actionable errors** — every failure renders a one-sentence message plus a
  suggested next step. `--debug` shows the redacted raw exchange.

## Install

### `go install`

```sh
go install github.com/addozhang/cfl/cmd/cfl@latest
```

### Homebrew

```sh
brew install addozhang/tap/cfl
```

### From source

```sh
git clone https://github.com/addozhang/confluence-cli
cd confluence-cli
make build      # produces ./bin/cfl
```

## Quick start

```sh
# 1. Store a Personal Access Token for your instance (hidden prompt).
#    Create the PAT in Confluence: profile -> Settings -> Personal Access Tokens.
cfl auth add https://wiki.example.com

# 2. Verify the token works.
cfl auth whoami https://wiki.example.com

# 3. Read a page (any Confluence URL shape, or a bare page ID).
cfl page get https://wiki.example.com/pages/viewpage.action?pageId=12345
cfl page get https://wiki.example.com/display/ENG/Runbook
cfl page get 12345          # works when exactly one instance is configured
```

## Commands

### auth

```sh
cfl auth add <url>          # store a PAT (hidden prompt; overwrite is confirmed)
cfl auth list               # list configured instance keys (never tokens)
cfl auth remove <url>       # remove a stored credential (idempotent)
cfl auth whoami <url>       # verify a stored token against its instance
```

### page

```sh
cfl page get <url-or-id>
cfl page create --space KEY --title T --body <input> [--parent ID] [--instance URL]
cfl page update <url-or-id> --body <input> [--title T]
cfl page delete <url-or-id> [--yes]
cfl page children <url-or-id>
```

`--body` accepts three forms:

- `--body @path` — read the body from a file
- `--body -` — read the body from stdin
- `--body '<p>literal</p>'` — use the string verbatim

Bodies are **Confluence storage format (XHTML)**, passed through unchanged — no
Markdown/wiki conversion.

`cfl page delete` requires explicit intent: pass `--yes`, or confirm the
interactive prompt. In a non-interactive session it refuses without `--yes`.

### space

```sh
cfl space list [--limit N] [--start N] [--instance URL]
cfl space get <key> [--instance URL]
```

`space list` returns a single bounded page; `--limit`/`--start` map directly onto
the REST pagination parameters.

### version

```sh
cfl version                 # offline; prints version/commit/date
```

## Global flags

| Flag | Default | Description |
|---|---|---|
| `-o, --output` | `yaml` | Output format: `yaml`, `json`, or `raw`. |
| `--timeout` | `30s` | Per-request timeout (Go duration, e.g. `30s`, `2m`). |
| `--insecure` | off | Disable TLS verification (prints a stderr warning). |
| `--debug` | off | Log the raw HTTP exchange to stderr (`Authorization` redacted). |

### Custom CAs / self-signed Confluence

`cfl` honors the `SSL_CERT_FILE` environment variable with no flag. Point it at a
PEM CA bundle and `cfl` trusts a self-signed instance without `--insecure`:

```sh
export SSL_CERT_FILE=/path/to/corporate-ca.pem
cfl page get https://wiki.internal/pages/viewpage.action?pageId=12345
```

An invalid `SSL_CERT_FILE` path produces a clear error.

## Output and `schemaVersion`

Default output is YAML; `-o json` is the structurally-identical compact form;
`-o raw` prints the verbatim Confluence response. Every yaml/json response begins
with `schemaVersion: "1"`.

**Pin the schema version** in scripts: read `schemaVersion` and fail fast if it
is not the value you tested against. Field stability tiers are documented in
[`docs/schema.md`](docs/schema.md) — only `stable` fields carry the
compatibility promise.

```sh
ver=$(cfl page get "$URL" -o json | jq -r .schemaVersion)
[ "$ver" = "1" ] || { echo "unexpected cfl schemaVersion: $ver" >&2; exit 1; }
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success. |
| `>= 10` | Any cfl-level failure (bad URL, auth, network, parse, configuration). |

There are no intermediate command-result codes. The exact value within the
`>= 10` range is not stable; assert only on `>= 10`.

## Credentials

Tokens are stored in plaintext TOML at `~/.config/cfl/credentials` (file mode
`0600`), the same posture as `~/.aws/credentials` and `~/.kube/config`. The
lookup key is `scheme://host[:port]` plus an optional context-path prefix, so one
host can front several reverse-proxy-mounted Confluence instances.

## Development

```sh
make build              # build ./bin/cfl
make test               # unit + integration tests, race + coverage
make lint               # golangci-lint
make fmt                # gofmt + goimports
```

End-to-end tests against a real Confluence run under `test/e2e/` behind the
`e2e` build tag; see [`test/e2e/README.md`](test/e2e/README.md).

This project uses [OpenSpec](https://openspec.dev) for change management
(`openspec/`) and `SPEC.md` as the engineering constitution.

## License

MIT
