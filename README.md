# slowbooks-cli

`slowbooks` puts SlowBooks Pro in your shell: bookkeeping reads and writes, reports,
PDFs and exports, with JSON for scripts and agents. One Go binary, no runtime.
Connect it to a running SlowBooks Pro server with an API token.

Download the archive for your OS and architecture from
[Releases](https://github.com/ContractorKeith/slowbooks-cli/releases), verify it
against `SHA256SUMS`, extract it, and put `slowbooks` (`slowbooks.exe` on Windows) on your
PATH. Archives cover Linux, macOS and Windows on x64 and arm64.

On macOS, a browser download may be quarantined. After extracting, run
`xattr -d com.apple.quarantine slowbooks` before starting the binary. Downloads made
with `gh release download` or `curl` are unaffected.

On Windows PowerShell 5.1, save downloads with `-o file`, or preserve raw stdout
with `cmd /c "slowbooks.exe ... -o - > file"`; PowerShell's `>` re-encodes bytes. Prefer
`--data "@file"` over inline JSON, since PowerShell 5.1 changes inline quoting.
Save JSON files as UTF-8 (a UTF-8 BOM is accepted; UTF-16 is unsupported).

While this repository is private, release downloads and `go install ...@latest`
require repository access (`go install` also needs Git credentials). Building
from a clone you already have works now:

```bash
go build -o slowbooks ./cmd/slowbooks
```

With repository access, you can also install with Go:

```bash
go install github.com/ContractorKeith/slowbooks-cli/cmd/slowbooks@latest
```

Create a token in SlowBooks Pro Settings → API tokens, then log in and paste
it at the hidden prompt:

```bash
slowbooks auth login --url https://your-server.example
slowbooks doctor
```

An existing Switchboard connection also works: set `SLOWBOOKS_URL` and
`SLOWBOOKS_TOKEN_FILE` (0600 on Linux/macOS). No profile required.
Use `--company` or `SLOWBOOKS_COMPANY` with the exact served-company `file` from
`/api/companies`. Display names and database names cannot select books. Servers
without served-company files (including PostgreSQL) require the default selection;
use `--company=` to clear a stored or environment selection.

```bash
slowbooks invoices list --open --json --jq '.data[].id'
slowbooks payments record --key chk-1042 --customer 7 --amount 500 --date 2026-11-09 --apply 31=500 --dry-run
slowbooks invoices pdf 31 -o invoice-31.pdf
```

Preview writes with `--dry-run` before sending them. Payment keys must be stable
check numbers or bank references. Writes never auto-retry; inspect the books
and the error hint after an uncertain result.

Piped output and `--json` use `{ok,data,summary,next}`; errors use
`{ok:false,error}`. See [the envelope and exit codes](docs/json.md).
Use `slowbooks commands --json` or `slowbooks --help --agent` for the generated command
reference, and `slowbooks ops find deposit` for server operations beyond named commands.

Install the agent skill by copying [skills/slowbooks-cli](skills/slowbooks-cli/SKILL.md) to your
agent's skills directory (for Codex: `~/.agents/skills/slowbooks-cli/SKILL.md`). It teaches
connection setup, discovery, safe writes and file handling.

Requires SlowBooks Pro 2.31 or later; verified against SlowBooks Pro 2.31.10.
Other versions produce an advisory warning and remain usable. `slowbooks version --json`
lists this build's verified versions. V1 plus two three-platform skeptic rounds complete.
MIT licensed.

For contributors: `bash scripts/accept.sh` runs the offline gate;
`bash scripts/e2e.sh` tests the pinned server in Docker.
`bash scripts/release-dry-run.sh` builds six archives and checksums in `.tmp/dist` without publishing. `scripts/smoke-live.sh` is manual:
it requires explicit server/token environment variables and creates then voids
one invoice. It refuses to run in CI.

Invoice and bill lists accept `--open` for `open_only=true`, or `--status` with
one status: invoices `draft|sent|partial|paid|void`; bills
`draft|unpaid|partial|paid|void`. Values outside a declared enum exit 2 before the list request; absent enums
produce a warning and pass the value through, except verified 2.31.10 bills use
the known status list above. Generic `--param status=value`
bypasses named status validation.
Payment keys use 1–128 letters, digits, or `._:-`. Selected companies are checked
at login and before online reads/writes; unknown files are refused, while a failed
lookup warns and proceeds. A supplied key is retry-safe only on keyed operations;
unkeyed writes warn that the server ignores it. After a successful write, a jq
runtime failure prints the unfiltered success and warns, with exit 0.
`slowbooks version` reports release versions without the leading `v`.

In PowerShell, quote file arguments as `"@file"` to avoid splatting. For example:

```powershell
slowbooks invoices create --customer 7 --date 2026-11-09 --data "@invoice.json" --dry-run
```
