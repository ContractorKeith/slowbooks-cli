# JSON output

Piped stdout, or `--json`, selects JSON. Each command prints one newline-terminated
object on stdout. A terminal without `--json` gets a key/value view; human errors
go to stderr, with a non-empty hint on a second line. `--quiet` prints only the
successful `data` value as JSON. Errors keep their full envelope even with `--quiet`. `--no-color` is accepted; M0 emits
no color. `--jq` is reserved and returns a usage error until M3.

Success (all fields are always present; `next` is an array, including when empty):

```json
{"ok":true,"data":{"version":"dev","commit":"unknown","verified_sbp":["2.31.10"]},"summary":"sbp dev","next":[]}
```

Error (all error fields are always present):

```json
{"ok":false,"error":{"code":"refused","message":"Books are closed","retryable":false,"hint":"","detail":"Books are closed"}}
```

`detail` preserves the server's JSON value, including strings and validation-error
arrays; it is `null` when absent. `message` uses the server's sentence when detail
is a string. `retryable` is false for writes. Network failures on writes include
a hint to check the books before running the write again. M0 makes one attempt;
read retries arrive in M2. Only network errors and HTTP 502/503/504 are marked
retryable for reads; HTTP 500/501 are not. Timeout is 120 seconds by default, range
1–3600, and also bounds DNS lookup. Responses larger than 64 MiB fail with exit 8
and message `response too large`; these size failures are not retryable.

| Exit | code | Meaning |
|---|---|---|
| 0 | — | Success |
| 1 | `error` | Unexpected failure or an otherwise unmapped HTTP status |
| 2 | `usage` | Bad arguments/flags or a network-rule refusal (including redirect) |
| 3 | `auth` | HTTP 401; missing, invalid, or unsafe token file (M1) |
| 4 | `refused` | HTTP 403, including role or closing-date refusal |
| 5 | `not_found` | HTTP 404 |
| 6 | `conflict` | HTTP 409, including mismatched payment-key reuse |
| 7 | `invalid` | HTTP 422 |
| 8 | `network` / `server` | Connection failure, timeout, response read failure, or HTTP 5xx |

Builds inject `version` and `commit` with `-ldflags '-X main.version=... -X
main.commit=...'`. `verified_sbp` comes from the small manifest in
`internal/commands`. A unit test checks it against every `testdata/openapi-<version>.json` filename. The production
binary includes neither the snapshots nor fake fixtures.

M1 adds `auth login/status/logout`, `profile list/use`, and `doctor`. Auth status
and login return `url`, `company`, `role`, `server_version`, and `token_file`;
token contents are never printed. Profile list returns the stored `default` and
`profiles` map, including the role recorded at login. Status and doctor probe the
current role. `version` remains a local build query and needs no connection.

Doctor returns `data.checks`, an array of `{name, ok, detail}` rows, even when a
check fails. Its success envelope means the diagnostic ran; the process exits
with the first failing check's class (zero only when every row passes). Missing
prerequisites mark dependent checks unavailable. Environment-only connections
need no config file. The catalog check is informational (`not built yet`) until
M2. Version mismatches are warnings in stderr and in the version row, and do not
fail commands. Check the process exit and individual row `ok` values for health.
