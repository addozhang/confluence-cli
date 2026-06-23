---
name: cfl-confluence-cli
description: Operate Confluence Server/Data Center from AI coding agents using the `cfl` CLI. Use this skill whenever the user mentions Confluence, a Confluence page or space, a wiki page, reading or editing Confluence content from the terminal, searching Confluence, or asks an agent to fetch, create, update, delete, or find Confluence pages by URL.
---

# cfl Confluence CLI

Use `cfl` to read and operate Confluence **Server / Data Center** pages and spaces from the terminal.

## Core Model

`cfl` is URL-native:

- **Confluence URLs are the primary identity.** Use the exact URL the user would paste into a browser. The host (plus optional reverse-proxy context path) selects the stored credential.
- **Server / Data Center only.** Confluence **Cloud** is out of scope (different auth + API). Do not use `cfl` against `*.atlassian.net`.
- **Output is stable and self-owned.** Prefer `-o json` for agent parsing, YAML (default) for humans, and `-o raw` only when you need a field the schema does not expose. Every structured response begins with `schemaVersion: "1"`.
- **Bodies are Confluence storage format (XHTML), passed through unchanged.** `cfl` does not convert Markdown or wiki markup. When creating/updating, supply valid storage-format XHTML.
- **Version-safe updates.** `cfl page update` reads the current version and submits `current + 1`; a concurrent edit surfaces as a clear version-conflict error.
- It can read pages, create/update/delete pages, list a page's children, list/read spaces, search content (CQL), and manage per-instance credentials.

## First Move

When Confluence work is requested:

1. Check whether `cfl` is available with `cfl version` or `cfl --help`.
2. If the command shape is unclear, run `cfl <group> --help` (e.g. `cfl page --help`) rather than guessing.
3. Ask for the Confluence URL if the user has not given enough to identify the page, space, or instance.
4. Prefer `-o json` for any output you will parse or summarize.
5. Confirm a credential exists for the target host. If a command reports no credential, run the `cfl auth add <url>` it suggests (interactively — never paste tokens into chat).

## Command Reference

```sh
cfl auth add <url> [--alias <name>]      # store a PAT (hidden prompt); optional alias
cfl auth list -o json                    # configured instances + aliases (never tokens)
cfl auth whoami <url> -o json            # verify a stored token
cfl auth remove <url>                    # remove a credential (idempotent)

cfl page get <url-or-id> [-i <url|alias>] -o json
cfl page create -s KEY -t TITLE -b <body> [-p PARENT] [-i <url|alias>] -o json
cfl page update <url-or-id> -b <body> [-t TITLE] [-i <url|alias>] -o json
cfl page delete <url-or-id> [-y] [-i <url|alias>] -o json
cfl page children <url-or-id> [-i <url|alias>] -o json

cfl space list [-l N] [--start N] [-i <url|alias>] -o json
cfl space get <key> [-i <url|alias>] -o json

cfl search <text> [-s KEY] [--type page|blogpost] [-l N] [--start N] [-i <url|alias>] -o json
cfl search --cql '<raw CQL>' [-l N] [-i <url|alias>] -o json

cfl version -o json                      # offline; build version/commit/date
```

Flag shortcuts: `-o`=`--output`, `-i`=`--instance`, `-s`=`--space`, `-t`=`--title`, `-b`=`--body`, `-l`=`--limit`, `-p`=`--parent`, `-y`=`--yes`.

## URL Handling

Use Confluence URLs instead of inventing page IDs. Accepted shapes:

```text
https://host/spaces/KEY/pages/12345/Title        # modern page URL (ID is authoritative)
https://host/spaces/KEY/pages/12345              # modern page URL, no title
https://host/pages/viewpage.action?pageId=12345  # classic pageId URL
https://host/display/KEY/Page+Title              # display URL (resolved by title lookup)
https://host/rest/api/content/12345              # REST content URL
https://host/spaces/KEY                          # space (for space get/list context)
12345                                            # bare page ID (see instance rules)
prod:12345                                        # <alias>:<id> (picks the instance)
```

Context paths (reverse-proxy mounts like `/confluence`) before the first
`/spaces|display|pages|rest` segment are preserved automatically.

### Instance selection (important for bare IDs)

- A **full URL** carries its own host; `-i/--instance` is ignored.
- An **`<alias>:<id>`** form carries its instance via the alias; `-i` is ignored.
- A **bare numeric ID** has no host: pass `-i <url|alias>` to choose the instance. With a single configured instance it is optional; with several it is required (or use `<alias>:<id>`).

Prefer pasting the user's full URL whenever available — it removes all ambiguity.

## `--body` Input

`-b/--body` accepts three forms:

```sh
cfl page create -s ENG -t "Notes" -b @./notes.xhtml   # @path : read from file
cat notes.xhtml | cfl page create -s ENG -t N -b -     # -    : read from stdin
cfl page update <url> -b '<p>literal storage XHTML</p>' # literal string verbatim
```

The body must be valid Confluence **storage format** (XHTML). Do not pass
Markdown — it will be stored literally or rejected by the server.

## Common Workflows

### Read a Page

```sh
cfl page get https://wiki.example.com/spaces/ENG/pages/12345/Runbook -o json
```

Use the resolved `id`, `version`, `spaceKey`, `body` (storage XHTML), and
`ancestors` from the JSON. For a display URL (`/display/KEY/Title`) `cfl` does a
title lookup first; a non-unique or missing title errors clearly.

### Update a Page Safely

Read first if you need the current body, then update — `cfl` handles the version
increment itself:

```sh
cfl page get <url> -o json                 # inspect current body/version if needed
cfl page update <url> -b @./new.xhtml -o json
```

On a `version conflict` error, re-read with `cfl page get` and retry; do not
guess a version number. Preserve the title by omitting `-t` (it keeps the
current title).

### Create a Page

```sh
cfl page create -s ENG -t "Release Notes" -b @./notes.xhtml -o json          # top-level
cfl page create -s ENG -t "Sub" -p 12345 -b '<p>hi</p>' -o json              # child of 12345
```

### Search

Friendly flags compile to CQL (the search term is always escaped, never
injected):

```sh
cfl search "release notes" -s ENG -o json            # within a space; default type=page
cfl search "runbook" --type blogpost -o json
```

`--cql` is the **highest-precedence** input: when given it is the complete query
and the term/`-s`/`--type` are ignored. CQL is not validated client-side; a
malformed query surfaces the server's error.

```sh
cfl search --cql 'space = ENG AND title ~ "runbook" AND created > now("-7d")' -o json
```

### List a Page's Children / a Space's Pages

```sh
cfl page children <url-or-id> -o json
cfl space list -i prod -o json
cfl space get ENG -i prod -o json
```

### Delete a Page

Deletion needs explicit intent:

```sh
cfl page delete <url-or-id> -y -o json     # -y confirms; required in non-interactive use
```

Without `-y` in a non-interactive session the command refuses. Confirm with the
user before deleting unless they explicitly requested that exact page deletion.

## Credentials and TLS

Use `cfl auth add <url>` for first-time setup. It prompts for a Personal Access
Token (hidden), stores it under `~/.config/cfl/credentials` (mode `0600`), and
never prints tokens. Add `--alias prod` to give an instance a short name.

Security rules:

- Do not ask the user to paste PATs into chat if an interactive terminal path is available.
- Do not print credential file contents.
- Do not include tokens in command lines, logs, issue comments, or summaries.
- Use `SSL_CERT_FILE=/path/to/ca.pem` for private CAs (honored with no flag) when the instance presents a self-signed certificate.
- Use `--insecure` only as a last resort, and mention that it disables certificate verification.

## Output Handling

Prefer `-o json` for any command whose output you will parse or summarize.
Structured output begins with `schemaVersion: "1"` — check it before relying on
fields in automation. Use `-o raw` only to reach a Confluence field the schema
does not expose (it prints the verbatim REST response). Aliases and tokens are
never both shown; `auth list` shows keys + aliases, never tokens.

## Exit Codes

```text
0   success
>=10 cfl-level failure (bad URL, auth, not found, version conflict, TLS,
     timeout, network, malformed response)
```

Assert only on `>= 10` for failure; the exact value is not stable.

## Safety

Read-only inspection is safe by default:

```sh
cfl page get ...
cfl page children ...
cfl space list ...
cfl space get ...
cfl search ...
cfl auth list ...
cfl auth whoami ...
cfl version
```

State-changing commands need clear user intent:

```sh
cfl page create ...
cfl page update ...
cfl page delete ... -y
cfl auth add ...
cfl auth remove ...
```

Confirm once before creating, overwriting, or deleting Confluence content, or
before modifying credentials, unless the user explicitly requested that exact
action.

## Error Triage

`cfl` user-facing errors include a message and a suggestion. Follow the
suggestion first. Common cases:

- **No credential configured**: run the suggested `cfl auth add <url>` (or `--alias`); check `cfl auth list -o json` for configured instances.
- **Page not found**: verify the URL or page ID; a display URL may have a non-unique or missing title — prefer a `pages/.../{id}` URL.
- **Bare ID ambiguous / requires instance**: pass `-i <url|alias>` or use `<alias>:<id>`, or paste the full page URL.
- **Version conflict**: re-run `cfl page get`, then retry `cfl page update`.
- **TLS failure**: set `SSL_CERT_FILE` to the CA bundle; avoid `--insecure` unless explicitly acceptable.
- **Timeout**: increase with `--timeout <duration>` or check VPN connectivity.
- Re-run any failing command with `--debug` to inspect the raw HTTP exchange (the `Authorization` header is redacted).

## Agent Response Pattern

When reporting Confluence findings back to the user:

1. State the object inspected: the page URL/ID or space key.
2. State the key facts from the JSON (title, version, spaceKey; or the search hit count).
3. For an edit, state what changed and the new version.
4. Suggest the smallest next action: read a related page, fix the body, retry after a conflict, or add a credential.
5. Mention if work was limited by missing auth, an ambiguous bare ID, an unavailable `cfl`, or a Cloud URL (unsupported).
