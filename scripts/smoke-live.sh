#!/usr/bin/env bash
# Explicitly manual: creates one invoice and voids it. Bash 3.2 compatible.
set -eu
set -o pipefail
if test -n "${CI:-}"; then echo 'smoke-live refuses to run in CI' >&2; exit 2; fi
if test -z "${SLOWBOOKS_URL:-}" || test -z "${SLOWBOOKS_TOKEN_FILE:-}"; then
  echo 'smoke-live requires SLOWBOOKS_URL and SLOWBOOKS_TOKEN_FILE from the caller' >&2
  exit 2
fi
sbp="${SBP_BIN:-sbp}"
"$sbp" doctor --json
customer="$("$sbp" customers list --limit 1 --json --jq '.data[0].id // empty')"
case "$customer" in ''|*[!0-9]*) echo 'no customer found; no invoice created' >&2; exit 2;; esac
"$sbp" invoices list --limit 3 --json
"$sbp" accounts list --limit 3 --json
day="$(date +%Y-%m-%d)"
memo="sbp smoke $(date -u +%Y-%m-%dT%H:%M:%SZ)"
# An invoice with one descriptive line; no item or account lookup required.
data='{"lines":[{"description":"CLI smoke test","quantity":1,"rate":1,"amount":1}]}'
"$sbp" invoices create --customer "$customer" --date "$day" --memo "$memo" --data "$data" --dry-run --json
invoice="$("$sbp" invoices create --customer "$customer" --date "$day" --memo "$memo" --data "$data" --json --jq '.data.id')"
case "$invoice" in ''|*[!0-9]*) echo 'create returned no invoice id; inspect invoices before another write' >&2; exit 1;; esac
printf 'created invoice id=%s customer id=%s\n' "$invoice" "$customer"
"$sbp" invoices void "$invoice" --json
printf 'voided invoice id=%s\n' "$invoice"
