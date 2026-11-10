---
name: slowbooks-cli
description: Use the slowbooks CLI to read and update SlowBooks Pro from a shell, inspect server operations, run reports, or download invoices and exports. Use when the user wants bookkeeping through the CLI rather than an MCP connector.
---

# SlowBooks Pro from a shell

Use the user's chosen server and company. Tokens come from SlowBooks Pro
Settings → API tokens. Never print a token or put one in a command argument.

## Connect

Log in once; the token prompt is hidden (stdin also works):

```bash
slowbooks auth login --url https://your-server.example
slowbooks doctor --json
```

For env-only use, set `SLOWBOOKS_URL` and `SLOWBOOKS_TOKEN_FILE` to the server
and existing token file. No profile is needed. The token file must have mode
0600 on Linux/macOS. `SLOWBOOKS_COMPANY` selects a company; `--company` overrides
it. Use `--profile` for a saved connection.

## Discover before constructing a request

```bash
slowbooks commands --json
slowbooks --help --agent
slowbooks ops find deposit
slowbooks ops describe POST /api/deposits
```

The reference includes arguments, flags, required body fields, operation,
read/write status, payment key policy, file output, examples and exit codes.
For operations without a named command, use `ops call` and the server schema.

## Read

```bash
slowbooks invoices list --open --json
slowbooks customers list --limit 10 --json --jq '.data[].id'
slowbooks customers list --all --json
slowbooks invoices show 31 --json
slowbooks reports run profit-loss --from 2026-01-01 --to 2026-12-31 --json
```

Piped output or `--json` returns `{ok,data,summary,next}`. Errors return
`{ok:false,error:{code,message,retryable,hint,detail}}`. `--jq` filters the
whole envelope; `--quiet` emits only data. Lists default to 50 rows; use
`--limit` or `--all`, and read the summary for trimming. `ops find` defaults to
20 concise matches with the same flags; `ops describe` gives full schemas.

## Write

Confirm the intended record and amounts from the user's request. Preview a
write with `--dry-run`, inspect it, then run the same command without that flag.

```bash
slowbooks invoices create --customer 7 --date 2026-11-09 --memo Service --data "@invoice.json" --dry-run --json
slowbooks payments record --key chk-1042 --customer 7 --amount 500 --date 2026-11-09 --apply 31=500 --dry-run --json
slowbooks invoices void 31 --dry-run --json
slowbooks ops call POST /api/deposits --data "@deposit.json" --dry-run --json
```

Invoice data is an object containing `lines`, not a bare array. Explicit flags
override data fields. `--data` also accepts inline JSON or `-` for stdin.
Payment record commands require `--key`: use a stable check number or bank
reference, never a fresh random key. For named records, the key defaults
`payments record` body `reference`, or `bill-payments record` body `check_number`.
A field present in `--data` wins, including null; `--dry-run` shows the default.
Generic keyed payment operations also
require it. Amounts are positive plain decimals with at most two decimal places.

Never automatically retry a write, even with a key. After a failure inspect the
record/list and the error hint before considering another write. The CLI never
auto-retries writes. Read `next` after success; it points to the record to inspect.

## Files

```bash
slowbooks invoices pdf 31 -o invoice-31.pdf --json
slowbooks download /api/csv/export/customers -o customers.csv --json
```

Omit `-o` to use the server filename. Existing files require `--force`.
`-o -` streams raw bytes to stdout; with `--json` the envelope goes to stderr.
Discard partial streamed output if the command fails.

## Exit codes

| Exit | Meaning |
|---|---|
| 0 | Success |
| 1 | Other error |
| 2 | Usage, missing payment key, or network-rule refusal |
| 3 | Authentication or token file problem |
| 4 | Role or closing-date refusal |
| 5 | Not found |
| 6 | Conflict, including key reused with a different body |
| 7 | Business refusal (400) or invalid data (422) |
| 8 | Network, timeout, or server failure |

Use `--open` for open invoices/bills. `--status` accepts invoices
`draft|sent|partial|paid|void`, bills `draft|unpaid|partial|paid|void`.
Keys must match `[A-Za-z0-9._:-]{1,128}`.

On Windows PowerShell 5.1, prefer `--data "@file"` to inline JSON. Save JSON as
UTF-8; UTF-8 BOMs are accepted, UTF-16 is unsupported. For downloads use `-o file`
or `cmd /c "slowbooks.exe ... -o - > file"`, because PowerShell's `>` re-encodes bytes.

`POST /api/batch-payments` and the payment `/replace` routes do not accept an
`Idempotency-Key` on the server. Supplying `--key` cannot make these operations
retry-safe: never re-run them after an uncertain result. Inspect the books and
resolve what happened before making another payment change.

When the catalog omits a status enum, verified 2.31.10 bills use
`draft|unpaid|partial|paid|void`; other operations warn and pass the value through. Generic calls such as `slowbooks api GET /api/invoices --param status=sent`
bypass named status validation.

In PowerShell, quote file arguments as `"@file"` to avoid splatting. For example:

```powershell
slowbooks invoices create --customer 7 --date 2026-11-09 --data "@invoice.json" --dry-run
```

Company selection accepts only the exact served-company `file` from
`/api/companies`, never its display name, id or database name. Online reads and
writes refuse unknown files. Login validates the same selection before saving
its profile. If the server lists no files (including PostgreSQL),
selection is unsupported: use `--company=` for its default books. Failed company
lookups warn and permit the operation. For persistent recovery, unset
`SLOWBOOKS_COMPANY` or clear a saved selection with
`slowbooks auth login --profile <profile> --company=` and enter its token.

A supplied `--key` only makes a keyed operation safe to replay. On any unkeyed write
operation it warns that the server ignores the key; never follow a same-key replay
strategy for invoices, customers, deposits, journal entries or payment voids.
After a successful write, a jq runtime failure emits the unfiltered success
envelope, warns on stderr and exits 0. Treat that write as completed.
