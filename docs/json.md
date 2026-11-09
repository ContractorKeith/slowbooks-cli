# JSON output

Piped stdout, or `--json`, selects JSON. Each command prints one newline-terminated
object on stdout. A terminal without `--json` gets a key/value view; human errors
go to stderr. `--quiet` prints only the successful `data` value as JSON. Errors
keep their full envelope even with `--quiet`. `--no-color` is accepted; M0 emits
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
read retries arrive in M2. Timeout is 120 seconds by default, range 1–3600.

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
main.commit=...'`. `verified_sbp` derives from the embedded
`testdata/openapi-<version>.json` filenames.
