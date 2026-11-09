---
name: sbp
description: Use the sbp CLI to read and update SlowBooks Pro from a shell, inspect server operations, run reports, or download invoices and exports. Use when the user wants bookkeeping through the CLI rather than the Switchboard MCP connector.
---

# SlowBooks Pro from a shell

Use the user's chosen server and company. Tokens come from SlowBooks Pro
Settings → API tokens. Never print a token or put one in a command argument.

## Connect

Log in once; the token prompt is hidden (stdin also works):

```bash
sbp auth login --url https://your-server.example
sbp doctor --json
```

For env-only use, set `SLOWBOOKS_URL` and `SLOWBOOKS_TOKEN_FILE` to the server
and existing token file. No profile is needed. The token file must have mode
0600 on Linux/macOS. `SLOWBOOKS_COMPANY` selects a company; `--company` overrides
it. Use `--profile` for a saved connection.

## Discover before constructing a request

```bash
sbp commands --json
sbp --help --agent
sbp ops find deposit
sbp ops describe POST /api/deposits
```

The reference includes arguments, flags, required body fields, operation,
read/write status, payment key policy, file output, examples and exit codes.
For operations without a named command, use `ops call` and the server schema.

## Read

```bash
sbp invoices list --status open --json
sbp customers list --limit 10 --json --jq '.data[].id'
sbp customers list --all --json
sbp invoices show 31 --json
sbp reports run profit-loss --from 2026-01-01 --to 2026-12-31 --json
```

Piped output or `--json` returns `{ok,data,summary,next}`. Errors return
`{ok:false,error:{code,message,retryable,hint,detail}}`. `--jq` filters the
whole envelope; `--quiet` emits only data. Lists default to 50 rows; use
`--limit` or `--all`, and read the summary for trimming.

## Write

Confirm the intended record and amounts from the user's request. Preview a
write with `--dry-run`, inspect it, then run the same command without that flag.

```bash
sbp invoices create --customer 7 --date 2026-11-09 --memo Service --data @invoice.json --dry-run --json
sbp payments record --key chk-1042 --customer 7 --amount 500 --date 2026-11-09 --apply 31=500 --dry-run --json
sbp invoices void 31 --dry-run --json
sbp ops call POST /api/deposits --data @deposit.json --dry-run --json
```

Invoice data is an object containing `lines`, not a bare array. Explicit flags
override data fields. `--data` also accepts inline JSON or `-` for stdin.
Payment record commands require `--key`: use a stable check number or bank
reference, never a fresh random key. Generic keyed payment operations also
require it. Amounts are positive plain decimals with at most two decimal places.

Never automatically retry a write, even with a key. After a failure inspect the
record/list and the error hint before considering another write. The CLI never
auto-retries writes. Read `next` after success; it points to the record to inspect.

## Files

```bash
sbp invoices pdf 31 -o invoice-31.pdf --json
sbp download /api/csv/export/customers -o customers.csv --json
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
| 7 | Invalid data (422) |
| 8 | Network, timeout, or server failure |
