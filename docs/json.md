# JSON output

Piped stdout, or `--json`, selects JSON. Each command prints one newline-terminated
object on stdout (except jq streams). A terminal without `--json` gets a table for
arrays of objects and a key/value view for objects; human errors
go to stderr, with a non-empty hint on a second line. All human diagnostics,
including warnings, escape controls with the table escaper. JSON encoders do not
HTML-escape `<`, `>` or `&`. `--quiet` prints only the
successful `data` value as JSON. Errors keep their full envelope even with `--quiet`. `--no-color` is accepted; M0 emits
no color. `--jq <expr>` filters the full success envelope by default (for example,
`--jq '.data[].id'`). With `--quiet`, jq instead filters the data value
(`--quiet --jq '.[].id'`). Each result is JSON on its own line, including quoted
strings; `empty` emits nothing. jq parse, compile and evaluation errors exit 2.
Parse and compile errors are checked before requests or mutations. Errors always
retain their full envelope, bypassing jq; a streaming evaluation error may follow
already emitted results. Evaluation uses the command context and the configured
`--timeout` (including the timeout environment variable). Output is capped at
100,000 results and 64 MiB including newlines. During evaluation, a watchdog
samples process heap objects every 50 ms and cancels when growth from the
pre-evaluation baseline exceeds 512 MiB
(exit 2, `--jq used more than 512 MiB`). The already-loaded response is excluded from this sampled growth budget;
so allocation bursts may briefly exceed it. If evaluation has not stopped within
250 ms of cancellation, the CLI emits the error envelope and exits 2.
Cancellation, timeout, and cap failures exit 2 with a clear jq error. Shared call decoding preserves JSON numbers through named/generic reads, quiet
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
is a string, an object with `message`, or an array whose first item has
`message` or `msg` (in that order). Writes are never automatically retried. Network failures and HTTP 502/503/504 on unkeyed
writes have
`retryable:false` and a hint to check the books before running the write again.
Network failures and HTTP 502/503/504 on writes carrying `--key` have `retryable:true` and the hint
`rerun with the same --key`. The known exceptions are `POST /api/batch-payments`
and payment `/replace` routes: the server ignores keys on them, so they retain
`retryable:false` and check-the-books advice even when `--key` is supplied.
Never re-run these operations after an uncertain result. Reads retry once on
network failure or HTTP 502/503/504. Only network errors and HTTP 502/503/504 are marked
retryable for reads; HTTP 500/501 are not. Timeout is 120 seconds by default, range
1–3600, per attempt, and also bounds DNS lookup. Reads may take two attempts. Responses larger than 64 MiB fail with exit 8
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
| 7 | `invalid` | HTTP 400 business refusal or HTTP 422 validation failure |
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

Profile names are unique ignoring ASCII case; a conflicting login exits 2 and
names the existing profile. New managed token paths and catalog profile directories
use lowercase names; legacy managed token paths remain usable. Relative profile
`token_file` paths resolve against the config directory. Login refuses to replace
an existing token not managed by the selected profile (exit 2, naming the file),
and logout preserves external tokens.

UTF-8 BOMs are accepted in profiles and token files. UTF-16 token files are refused
with a UTF-8/ASCII hint. Login moves malformed or empty profiles aside to
`profiles.json.corrupt-<UTC timestamp>` and reports the backup path on stderr
before starting a fresh store; other
commands name the file and the login recovery command. On Windows, config readers
share deletion, and replacements retry sharing/access-denied errors for at most
280 ms of backoff. Doctor reports inherited ACLs instead of Unix mode bits there.

Tokens are at most 4096 bytes, excluding an optional UTF-8 BOM and LF or CRLF.
Non-regular files are refused before reading; Unix opens are nonblocking to guard against
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
with an explicitly declared non-JSON response retain raw text as a JSON string.
If a write omits or leaves Content-Type blank, valid JSON is decoded first.
If an operation declares JSON and instead returns an undeclared non-JSON body,
the command exits 1 with `retryable:false`, message `the server answered with a
page, not JSON; the write may have happened`, and a check-the-books hint. The
write is never replayed. Unknown operations retain permissive text handling. Malformed JSON
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
exit 2. Request data accepts inline JSON, `@file`, or `-` for stdin. UTF-8 BOMs are
accepted; UTF-16 input exits 2 with a save-as-UTF-8 hint. Repeated
`--param name=value` fills path placeholders and otherwise adds query values.

Keys must match `[A-Za-z0-9._:-]{1,128}`; an explicitly empty `--key` exits 2. Writes with trailing slashes
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
`N of M shown; --all for everything`; otherwise the summary is `M <noun>`, using a singular noun for one row.
On a TTY, trimmed-list summaries also appear on stderr.
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

Invoices and bills expose `--open`, mapping to declared `open_only=true`, and
`--status`, validated locally against catalog enums. The pinned invoice response
declares `draft|sent|partial|paid|void`; its query omits that enum. Bill schemas
omit it entirely, so values pass through with a stderr note when no enum is
declared. A value outside a declared enum exits 2 before the list request.
Generic `api`/`ops call --param status=value` bypass named status validation. The pinned server has no `since` parameter.
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
device names (including CONIN$, CONOUT$, COM/LPT0–9 and superscript ¹²³) gain
an `_` prefix, trailing dots/spaces are stripped, and `-`/`~` gain an `_` prefix.
Unicode format characters (category Cf) are removed. Names are capped at 255
UTF-8 bytes, retaining the extension where it fits. Directory separators, traversal, leading dots, and controls are removed.
Without `-o`, the file is saved in the current directory. Existing destinations
exit 2 unless `--force` is set. Files are staged in the destination directory
and installed atomically after a successful transfer; failures leave the existing
file intact. No-clobber installation uses an atomic hard link to avoid a race
between checking and renaming. Any hard-link failure other than an existing
target uses exclusive creation and copying instead; failed copies remove the new target. That fallback
can expose the target during copying. `--force` uses rename. Completed files use
0644 permissions on Unix (fallback creation also respects the process umask).
Local destination failures exit 2 with the destination named and no staging paths.
Directory destinations report that they are directories; a forced replacement of
a file held open without delete sharing reports that the file is in use.

`-o -` reserves stdout for raw bytes. With `--json`, the success envelope goes to
stderr (even with `--quiet`); without it there is no success envelope. Errors and
diagnostics also go to stderr. Downloads bypass the 64 MiB success-body cap;
error bodies retain it. Reads retry once on network failure or 502/503/504.
Staged files rewind before retrying an interrupted body. Raw stdout cannot be
rewound: after any bytes have been emitted, an interrupted stream exits 8 without
replaying bytes and is marked non-retryable. Consumers must discard that partial
stream when the command fails.

Before staging a download, sbp removes `.sbp-download-*` regular files in that
directory that are older than one hour and owned by the current user. Cleanup
skips symlinks and leaves fresh files, other names, and other owners untouched.
Files whose ownership cannot be checked or which cannot be removed are skipped.

SIGINT/Ctrl-C and SIGTERM cancel active downloads, remove staged files, and exit
8 (`network`, non-retryable). Broken stdout pipes also exit 8 with
`retryable:false`; stdout remains reserved for raw bytes.

M5 named writes include invoice create, customer/vendor payment record, and void
for invoices, bills, expenses, journal, payments and bill-payments. Success hints
name the returned record's show command. Unkeyed network-failure hints name the
corresponding list command with `--limit 5` only after write headers were sent.
Lookup, connection, and discovery failures retain connection hints. Record
commands require a stable caller key, such as a check number or bank reference.

Write `--data` accepts an object from inline JSON, `@file`, or stdin (`-`);
explicit flags override its fields. Invoice `--memo` maps to the server's `notes`,
`--due` to `due_date`. Repeated `--apply ID=AMOUNT` replaces `allocations` using
`invoice_id` for payments or `bill_id` for bill payments. Named payment amounts
must be positive plain decimals with at most 13 integer and two fractional
digits, with no sign or exponent, whether supplied by flags or as JSON numbers
or strings in `--data`. Invalid amounts and repeated
allocation IDs exit 2 before HTTP. Accepted flag amounts are JSON numbers
preserving the supplied decimal spelling; data amounts retain their string or
number representation. IDs also retain integer precision. Explicit string
flags must not be blank. Required body fields are declared in the command table.
`--fee-refunded` is available only for payment voids and sends the query parameter
`fee_refunded=true` when enabled. Every named write supports an offline dry-run
with the same redacted preview as the generic layer.

`commands --json` and root `--help --agent` return the same success envelope.
`--agent` requires `--help`; on a subcommand it describes only that subtree. Its
`data` has `commands` (the generated Cobra tree, including inherited flags) and
`exit_codes`. Command entries include `name`, positional `args`, `flags` with
name/type/default/required, `operation` where applicable, `access`
(read/write/dynamic), `keyed`, `file_output`, examples and next hints.
`required_fields` describes required write body fields: they can come from
flags or `--data`, so the individual flags are not unconditionally required.
`reports run` describes a path template; the manifest expands it into the
verified concrete JSON report operations. `reports list` reads the catalog.
Generic `api` and `ops call` have dynamic access and key requirements based on
the selected operation; consult `ops describe` before calling them.

`commands --manifest` emits a raw marketplace `plugin.json`, without an envelope
or jq/quiet transformation. `--checksums SHA256SUMS` requires a valid checksum
for all six release archives; without it, download hashes are placeholder zeros.
Do not use placeholder hashes to verify a download. Tagged build versions have
the leading `v` removed. Only stable `x.y.z` build versions are accepted;
prerelease, build-metadata and unversioned development builds exit 2. The local
release dry run injects `v0.0.0`. Report paths can be regenerated with
`go test ./internal/commands -run '^TestReportsEmbedded$' -update`.

Release `version` output omits the leading `v`, matching the plugin manifest.
Named PDF and write hints use the actual record ID. Unknown record placeholders
are omitted. Environment-only catalog cache
directories include a short hash of the server URL and company. If an environment
URL differs from the selected profile and no environment token file is supplied,
commands exit 3 with a `SLOWBOOKS_TOKEN_FILE` hint before sending its token.
Company and key header values are validated locally (exit 2), including dry-run.
Login and online writes validate non-empty company selections against
`/api/companies`. A successful lookup refuses unknown selections; failed lookups
warn `could not verify company X: <reason>` and allow login/writes to proceed.
Doctor always checks the list and fails its companies row for lookup errors or
unknown selections. One company lookup is cached in memory per invocation. Offline
dry-run makes no HTTP requests and cannot verify company membership.
