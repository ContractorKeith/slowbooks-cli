# JSON output

Piped stdout, or `--json`, selects JSON. Each command prints one newline-terminated
object on stdout (except jq streams). A terminal without `--json` gets a table for
arrays of objects and a key/value view for objects; human errors
go to stderr, with a non-empty hint on a second line. `--quiet` prints only the
successful `data` value as JSON. Errors keep their full envelope even with `--quiet`. `--no-color` is accepted; M0 emits
no color. `--jq <expr>` filters the full success envelope by default (for example,
`--jq '.data[].id'`). With `--quiet`, jq instead filters the data value
(`--quiet --jq '.[].id'`). Each result is JSON on its own line, including quoted
strings; `empty` emits nothing. jq parse, compile and evaluation errors exit 2.
Parse and compile errors are checked before requests or mutations. Errors always
retain their full envelope, bypassing jq; a streaming evaluation error may follow
already emitted results. Evaluation uses the command context and the configured
`--timeout` (including the timeout environment variable). Output is capped at
100,000 results and 64 MiB including newlines; cancellation, timeout, and cap
failures exit 2 with a clear jq error. Shared call decoding preserves JSON numbers through named/generic reads, quiet
output, and tables. jq receives exact integers, including IDs beyond 2^53;
jq arithmetic on non-integer numbers uses gojq's floating-point semantics.

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
a depth cap of 8, a 600-node budget, and a 16,000-character budget; alias chains
have a 60-reference cap and cycle detection. Cuts are marked. Calls return the
server JSON (or null for an empty response). Successful writes
with a non-JSON media type return raw text as a JSON string. Malformed JSON
responses and local cache errors use `error` (exit 1), not `server`. Catalog write
failures warn on stderr and commands continue using the fetched catalog.

Dry-run returns `{method,url,headers,body}` with Authorization redacted. It uses
only the existing catalog cache and sends no HTTP requests, including discovery.
With no cached version marker it exits 2 and hints to run `sbp ops find <words>`
first. An unreadable or corrupt offline cache exits 1.
Unknown `ops call` operations exit 2; `api` permits them with a stderr warning.
File routes are GET/HEAD operations identified by their path or declared non-JSON
success media types; they exit 2 with a `sbp download <path>` hint. Writes on
paths containing file-related words remain callable. Keyed writes without `--key`
exit 2. Request data accepts inline JSON, `@file`, or `-` for stdin. Repeated
`--param name=value` fills path placeholders and otherwise adds query values.

Keys are 1–200 bytes and must contain no control characters or leading/trailing
whitespace; an explicitly empty `--key` exits 2. Writes with trailing slashes
are refused with exit 2 before lookup or sending. Malformed query strings exit
2 instead of silently dropping filters. Path parameter values must be non-empty
single segments and cannot contain slashes, traversal, controls, or ambiguous
encodings. Request bodies are capped at 64 MiB; file read errors name the `@path`.

Read attempts use fresh connections so Go's automatic replay on reused connections
cannot exceed the two-attempt wire budget. Catalog filenames sanitize unusual
server versions with a hash suffix; reported versions retain the server's value.

M3 named reads use the same request path as `ops call`. Lists default to 50 rows;
`--limit N` requires a positive N, and `--all` cannot be combined with `--limit`.
For operations declaring `limit`, the client requests one extra row to detect
truncation. The summary says `N of N+ shown; --all for everything` when this
probe finds more rows. Lists without server limits trim locally and report
`N of M shown; --all for everything`; otherwise the summary is `M <noun>`.
Non-array responses pass through unchanged.

`--all` follows declared `skip` or `offset` paging until a short page. Page size
uses the declared maximum, otherwise the declared default, otherwise 500. If a
limited operation has no paging parameter, `--all` requests that size and warns
on stderr when the response fills it, since results may be incomplete. Ordinary
list requests also honor a declared maximum and page when necessary to satisfy
`--limit`. Operations without a limit parameter receive none.

TTY tables use the named command's columns, or up to five scalar fields in
alphabetical order for generic results. All keys, headers, and cells escape
control characters. Padding uses a small local display-width function covering
common CJK, Hangul, fullwidth and emoji ranges, plus zero-width combining marks;
complex joined emoji sequences and terminal-specific widths may vary.

Invoices and bills expose `--status`; the pinned server has no `since` parameter.
`reports list` discovers direct JSON report operations from the catalog, since
there is no report index API. `reports run <name>` calls `/api/reports/<name>`.
Date flags use the chosen operation's declared parameters: `--from` targets
`start_date`; `--to` targets `end_date`, `as_of_date`, or `as_of`, in that order.
A flag with no declared target exits 2 and names the declared query parameters;
for year-based reports use `--param year=2026`. Named report calls reject
undeclared `--param` names. The generic `api`/`ops call` layer remains permissive.
Search sends the query as `q`, returns its grouped response unchanged, and has
no `--limit` or `--all` flags.

Downloads use `sbp download <path> [-o file|-] [--param k=v]... [--force]`
or `sbp invoices pdf <id> [-o file|-] [--force]`. Only generic `download` accepts
`--param`; invoice IDs are single path segments and cannot include `?` or `#`.
Success data contains `path`, `bytes`, `content_type`, and the sanitized server `filename`.
The server's RFC 6266 filename (including `filename*`) takes precedence; missing
or unusable filenames fall back to the final URL path segment, then `download`.
Names follow Windows-safe rules on every OS: reserved characters become `_`,
device names gain an `_` prefix, trailing dots/spaces are stripped, and `-`
becomes `_-`. Names are capped at 255 UTF-8 bytes, retaining the extension where
it fits. Directory separators, traversal, leading dots, and controls are removed.
Without `-o`, the file is saved in the current directory. Existing destinations
exit 2 unless `--force` is set. Files are staged in the destination directory
and installed atomically after a successful transfer; failures leave the existing
file intact. No-clobber installation uses an atomic hard link to avoid a race
between checking and renaming. Filesystems without hard links use exclusive
creation and copying instead; failed copies remove the new target. That fallback
can expose the target during copying. `--force` uses rename. Completed files use
0644 permissions on Unix (fallback creation also respects the process umask).
Local destination failures exit 2 with the destination named and no staging paths.

`-o -` reserves stdout for raw bytes. With `--json`, the success envelope goes to
stderr (even with `--quiet`); without it there is no success envelope. Errors and
diagnostics also go to stderr. Downloads bypass the 64 MiB success-body cap;
error bodies retain it. Reads retry once on network failure or 502/503/504.
Staged files rewind before retrying an interrupted body. Raw stdout cannot be
rewound: after any bytes have been emitted, an interrupted stream exits 8 without
replaying bytes and is marked non-retryable. Consumers must discard that partial
stream when the command fails.

SIGINT/Ctrl-C and SIGTERM cancel active downloads, remove staged files, and exit
8 (`network`, non-retryable). Broken stdout pipes also exit 8 with
`retryable:false`; stdout remains reserved for raw bytes.

M5 named writes include invoice create, customer/vendor payment record, and void
for invoices, bills, expenses, journal, payments and bill-payments. Success hints
name the returned record's show command. Unkeyed network-failure hints name the
corresponding list command with `--limit 5`. Record commands require a stable
caller key, such as a check number or bank reference.

Write `--data` accepts an object from inline JSON, `@file`, or stdin (`-`);
explicit flags override its fields. Invoice `--memo` maps to the server's `notes`,
`--due` to `due_date`. Repeated `--apply ID=AMOUNT` replaces `allocations` using
`invoice_id` for payments or `bill_id` for bill payments. Flag amounts are JSON
numbers preserving the supplied decimal spelling; data amounts retain their
original string or number representation. IDs also retain integer precision.
`--fee-refunded` is available only for payment voids and sends the query parameter
`fee_refunded=true` when enabled. Every named write supports an offline dry-run
with the same redacted preview as the generic layer.
