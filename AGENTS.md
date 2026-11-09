# slowbooks-cli

`sbp` is a single Go binary that talks to a running SlowBooks Pro server over its
token API: everyday bookkeeping reads and writes with a stable JSON envelope and exit
codes, plus a generic layer (`api`, `ops`) driven by the server's own `/openapi.json`.
Built for people scripting their books and for agents that have a shell. Keith's
project, private, listed in the SBP plugin directory as `sbp-cli`. "Done" for v1 is
every v1 row in `docs/PLAN.md` §3 working on macOS, Linux and Windows.

## Status
planned — 2026-11-09: plan, glossary and ADRs written; milestones M0–M6 are issues #1–#7.

## Commands
```bash
bash scripts/accept.sh     # the acceptance: gate, build, goldens vs sbp-fake, live checks; CI runs this
bash scripts/e2e.sh        # real SlowBooks Pro v2.31.10 in Docker (Linux; on Omarchy: echo "bash scripts/e2e.sh" | newgrp docker)
go build -o sbp ./cmd/sbp && ./sbp version
go test -race ./...
test -z "$(gofmt -l .)" && go vet ./... && staticcheck ./...
```

## Pointers
- `docs/PLAN.md` — goal, feature matrix, architecture, connection model, CLI contract, milestones, testing.
- `CONTEXT.md` — glossary. `docs/adr/` — payment key (0001), token-API client only (0002).
- `docs/json.md` — envelope and exit codes (keep in step with `internal/output`).
- Session logs: `~/Projects/worklog/projects/slowbooks-cli.md` (not in this repo).

## Conventions & Gotchas
- Only `internal/client` makes HTTP requests; only `internal/output` writes stdout/stderr and picks exit codes.
- Named commands are rows in `internal/commands` and use the same call path as `ops call`.
- Never import, vendor or copy SlowBooks Pro or Switchboard code; HTTP only.
- Payment writes require `--key`; writes never auto-retry.
- Env vars keep Switchboard's names (`SLOWBOOKS_URL`, `SLOWBOOKS_TOKEN_FILE`, `SLOWBOOKS_TIMEOUT_SECONDS`).
- Tests isolate `SLOWBOOKS_CONFIG_DIR`, `SLOWBOOKS_CACHE_DIR` and `HOME`. Never touch `~/.config/sbp` (real Switchboard tokens live there).
- Pure Go, `CGO_ENABLED=0`. Tests pin facts, not status prose.
- One milestone = one issue = one branch = one draft PR = one T3 thread in its own worktree.
- Commits: `<type>: <description>`, author @ContractorKeith only.

## Tracking
GitHub issues (label `milestone`) are the truth. Basecamp `github` board card `slowbooks-cli`, one step per milestone. A tag means "verified against these SlowBooks Pro versions"; no other release process.

## Out of Scope
- An MCP server (Switchboard owns MCP), OS keyring, `--insecure`, following redirects, auto-retrying writes.
- Changes to the SlowBooks Pro product repo.
- Construction verbs until after v1.
