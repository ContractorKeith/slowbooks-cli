# slowbooks-cli

`sbp` — SlowBooks Pro from the terminal, from scripts and from agents. One binary,
no runtime. It talks to your running SlowBooks Pro over its API with a token you
give it once.

```
sbp auth login                              # paste the sbp_ token from Settings → API tokens
sbp invoices list --status open --json
sbp payments record --key chk-1042 --customer 7 --amount 500 --date 2026-11-09 --apply 31=500
sbp invoices pdf 31 -o invoice-31.pdf
sbp ops find deposit                        # anything without a named command
```

Status: M0 harness implemented: `sbp version`, output contract, HTTP client,
offline acceptance, and Docker e2e at v2.31.10. The commands above are the v1
target; auth and bookkeeping commands arrive in later milestones. See
`docs/PLAN.md`.

Requires SlowBooks Pro 2.31 or later. MIT licensed.
