# cfl end-to-end tests

These tests run `cfl` against a **real Confluence Server/DC instance** and a
**self-signed TLS proxy**, brought up with Docker Compose. They are gated behind
the `e2e` build tag and are **not** part of `make test`.

## Architecture

```
                       ┌──────────────────────────┐
   cfl --insecure /    │ nginx (self-signed TLS)   │
   SSL_CERT_FILE  ───▶ │ https://localhost:8443    │
                       └────────────┬─────────────┘
                                    │ proxy_pass
   cfl (plain HTTP)   ┌─────────────▼─────────────┐     ┌──────────────┐
   ───────────────▶   │ confluence                │────▶│ postgres     │
                      │ http://localhost:8090     │     │              │
                      └───────────────────────────┘     └──────────────┘
```

- **confluence + postgres** — the real server, exercised by every page / space /
  auth / version command.
- **nginx** — a self-signed HTTPS front door on `:8443`, used to exercise
  `SSL_CERT_FILE` (trust the CA, no flag) and `--insecure`.

## Why this is semi-automated

Confluence Data Center requires a **license** and a **Personal Access Token**
created through the web UI; neither can be fully scripted. So the flow is:

1. Bring the stack up (automated).
2. Complete the Confluence setup wizard and create a PAT (manual, one-time).
3. Run the tests (automated).

## Steps

```sh
cd test/e2e

# 1. Start the stack and generate the self-signed cert. First boot ~3-5 min.
make up

# 2. Watch until Confluence reports RUNNING.
make status        # repeat until /status shows RUNNING

# 3. In a browser, open https://localhost:8443 (accept the self-signed cert)
#    or http://localhost:8090. Complete the setup wizard
#    (trial license, choose the embedded/postgres DB, create an admin user),
#    create at least one Space, then create a Personal Access Token:
#       profile avatar -> Settings -> Personal Access Tokens -> Create token

# 4. Export the test configuration.
export CFL_E2E_BASE_URL=http://localhost:8090
export CFL_E2E_TLS_URL=https://localhost:8443
export CFL_E2E_CA_FILE=$(pwd)/certs/wiki.local.crt
export CFL_E2E_TOKEN=<the PAT you created>
export CFL_E2E_SPACE_KEY=<an existing space key>

# 5. Run the e2e tests.
make test          # go test ./test/e2e/... -tags=e2e -v

# 6. Tear down (use `make clean` to also drop volumes + certs).
make down
```

## What the tests cover

| Test | Verifies |
|---|---|
| `TestE2E_Version_offline` | `cfl version` works offline with `schemaVersion`. |
| `TestE2E_Whoami` | PAT verification; token never leaked. |
| `TestE2E_SpaceList_and_Get` | `space list` + `space get`. |
| `TestE2E_Page_full_lifecycle` | create → get → update (version increments) → children → delete. |
| `TestE2E_Page_not_found` | 404 → translated error, non-zero exit. |
| `TestE2E_SelfSigned_TLS` | `SSL_CERT_FILE` trusts the self-signed CA (no flag); `--insecure` bypasses verification. |

If the `CFL_E2E_*` variables are not set, the suite **skips** rather than fails,
so `go test -tags=e2e ./...` is safe on a machine without the stack.

## Notes

- The self-signed cert and key live in `certs/` and are git-ignored; regenerate
  with `make certs`.
- `atlassian/confluence:8.5` is a large image; the first `make up` pulls several
  hundred MB and the JVM needs ~2 GB. Adjust `JVM_*` in `docker-compose.yml` for
  constrained machines.
