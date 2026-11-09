# slowbooks-cli

`sbp` puts SlowBooks Pro in your shell: bookkeeping reads and writes, reports,
PDFs and exports, with JSON for scripts and agents. One Go binary, no runtime.
Connect it to a running SlowBooks Pro server with an API token.

Download the archive for your OS and architecture from
[Releases](https://github.com/ContractorKeith/slowbooks-cli/releases), verify it
against `SHA256SUMS`, extract it, and put `sbp` (`sbp.exe` on Windows) on your
PATH. Archives cover Linux, macOS and Windows on x64 and arm64.

Or install from source with Go:

```bash
go install github.com/ContractorKeith/slowbooks-cli/cmd/sbp@latest
```

Create a token in SlowBooks Pro Settings → API tokens, then log in and paste
it at the hidden prompt:

```bash
sbp auth login --url https://your-server.example
sbp doctor
```

An existing Switchboard connection also works: set `SLOWBOOKS_URL` and
`SLOWBOOKS_TOKEN_FILE` (0600 on Linux/macOS). No profile required.
Use `--company` or `SLOWBOOKS_COMPANY` to select a company.

```bash
sbp invoices list --status open --json --jq '.data[].id'
sbp payments record --key chk-1042 --customer 7 --amount 500 --date 2026-11-09 --apply 31=500 --dry-run
sbp invoices pdf 31 -o invoice-31.pdf
```

Preview writes with `--dry-run` before sending them. Payment keys must be stable
check numbers or bank references. Writes never auto-retry; inspect the books
and the error hint after an uncertain result.

Piped output and `--json` use `{ok,data,summary,next}`; errors use
`{ok:false,error}`. See [the envelope and exit codes](docs/json.md).
Use `sbp commands --json` or `sbp --help --agent` for the generated command
reference, and `sbp ops find deposit` for server operations beyond named commands.

Install the agent skill by copying [skills/sbp](skills/sbp/SKILL.md) to your
agent's skills directory (for Codex: `~/.agents/skills/sbp/SKILL.md`). It teaches
connection setup, discovery, safe writes and file handling.

Requires SlowBooks Pro 2.31 or later; verified against SlowBooks Pro 2.31.10.
Other versions produce an advisory warning and remain usable. `sbp version --json`
lists this build's verified versions. V1 complete, awaiting skeptic review.
MIT licensed.

For contributors: `bash scripts/accept.sh` runs the offline gate;
`bash scripts/e2e.sh` tests the pinned server in Docker.
`bash scripts/release-dry-run.sh` builds six archives, checksums and a marketplace
manifest in `.tmp/dist` without publishing. `scripts/smoke-live.sh` is manual:
it requires explicit server/token environment variables and creates then voids
one invoice. It refuses to run in CI.
