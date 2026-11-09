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
is a string. Writes are never automatically retried. Network failures on unkeyed
writes have
`retryable:false` and a hint to check the books before running the write again.
Network failures on writes carrying `--key` have `retryable:true` and the hint
`rerun with the same --key`. Reads retry once on network failure or HTTP
502/503/504. Only network errors and HTTP 502/503/504 are marked
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
need no config file. The catalog check reports cached server version, operation
count and cache path. Version mismatches are warnings in stderr and in the version
row, and do not
fail commands. Check the process exit and individual row `ok` values for health.

Role probes omit the company header because roles belong to tokens. A non-403
probe failure leaves login/status successful with role `unknown` and a stderr
warning; `/api/system` must still accept the token. Doctor reports probe failures
in its role check. Login's system request retains the selected company header.

Config mutations share an OS-backed lock with a two-second bounded wait (busy
config exits 3); process death releases the lock. Login stages the token, saves
profiles, then replaces the token. Logout saves profiles before removing its
managed token. Failed final token operations restore the previous profile data.
Deleting the default promotes the alphabetically first remaining profile and
names that default in the logout summary.

Tokens are at most 4096 bytes, excluding an optional LF or CRLF. Non-regular
files are refused before reading; Unix opens are nonblocking to guard against
replacement with a FIFO. File-access errors exit 3 and identify the path.
An environment URL without a token file names `SLOWBOOKS_TOKEN_FILE` in its hint;
a missing server URL is a usage error (exit 2).

M2 adds `ops find`, `ops describe`, `ops call`, and `api`. Find returns a ranked
array of operations (default limit 20). Describe returns operation metadata,
including params, body schema, keyed and file-route flags, role notes when the
server provides them, and `truncated`. Local schema references are resolved with
a depth cap of 8, a 600-node budget, and a 16,000-character budget; cuts are
marked. Calls return the server JSON (or null for an empty response).

Dry-run returns `{method,url,headers,body}` with Authorization redacted. It uses
only the existing catalog cache and sends no HTTP requests, including discovery.
With no valid cache it exits 2 and hints to run `sbp ops find <words>` first.
Unknown `ops call` operations exit 2; `api` permits them with a stderr warning.
File routes exit 2 with a `sbp download <path>` hint. Keyed writes without `--key`
exit 2. Request data accepts inline JSON, `@file`, or `-` for stdin. Repeated
`--param name=value` fills path placeholders and otherwise adds query values.
