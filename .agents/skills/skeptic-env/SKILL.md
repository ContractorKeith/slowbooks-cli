---
name: skeptic-env
description: "How a skeptical reviewer builds, seeds and attacks slowbooks-cli (slowbooks). Read by skeptic-review; not a workflow on its own."
---

# skeptic-env — slowbooks-cli

Go CLI over the SlowBooks Pro token API. The server is the source of truth; the
catalog cache is disposable; profile and token files are the only local state.

## Stack and commands

```bash
go build -o "$PWD/.tmp/slowbooks" ./cmd/slowbooks
bash scripts/accept.sh
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o .tmp/slowbooks.exe ./cmd/slowbooks
# Build first, then isolate (HOME isolation also moves Go's caches).
scratch=$(mktemp -d)
export HOME="$scratch/home" SLOWBOOKS_CONFIG_DIR="$scratch/config" SLOWBOOKS_CACHE_DIR="$scratch/cache"
unset SLOWBOOKS_URL SLOWBOOKS_TOKEN_FILE SLOWBOOKS_COMPANY SLOWBOOKS_PROFILE
mkdir -p "$HOME"
# Real server: scripts/e2e.sh --keep prints URL and token files (Linux: via newgrp docker).
```

Never use the reviewer's legacy `~/.config/sbp` (real Switchboard tokens) or books.kodade.com
except through `scripts/smoke-live.sh`, which voids what it creates.

## Platforms

| Platform | Ships | Review on |
|---|---|---|
| Linux amd64/arm64 | yes | Omarchy |
| macOS amd64/arm64 | yes | Mac Studio |
| Windows amd64/arm64 | yes | CI only |

## What to attack

- Network rules: http to a public host, redirects (301 to another host), paths outside `/api/`, IPv6 loopback, DNS names resolving to private ranges.
- Tokens: wrong mode, trailing newline/whitespace, CRLF, missing file, token in argv or shell history, token echoed in errors or `--dry-run` output.
- Payment key: same key twice (one payment), same key different body (exit 6), missing key (exit 2), concurrent runs with one key.
- Writes after a timeout: no retry, exit 8, hint present. Reads retry once only.
- Envelope and exit codes for every error class; `--quiet`, `--jq` errors, piped vs TTY.
- Catalog cache: server upgrade (version change refetch), corrupt cache file, two profiles on one server.
- `download`: overwrite without `--force`, path traversal in Content-Disposition filenames, `-o -` to stdout, a JSON route given to download.
- `--limit/--all` counts and summary; company header switching between two companies.
- Role and closing date refusals surface the server's sentence unchanged.

## Traps
- On Omarchy Docker needs `newgrp docker`; `/tmp` is a shared 6.8 GB tmpfs, use `$PWD/.tmp`.
- macOS `mktemp -d` paths are symlinks under `/private/var`.
