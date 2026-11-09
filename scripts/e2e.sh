#!/usr/bin/env bash
# Pinned real-server harness. Run on Linux with Docker access.
set -eu
cd "$(dirname "$0")/.."
export TMPDIR="$PWD/.tmp"
mkdir -p "$TMPDIR/e2e"
export GOCACHE="$(go env GOCACHE)" GOMODCACHE="$(go env GOMODCACHE)"
keep=0
case "${1:-}" in --keep) keep=1;; '') ;; *) echo 'usage: scripts/e2e.sh [--keep]' >&2; exit 2;; esac
checkout="$TMPDIR/e2e/SlowBooks-Pro-2026"
if test ! -d "$checkout/.git"; then
  git clone --depth 1 --branch v2.31.10 https://github.com/VonHoltenCodes/SlowBooks-Pro-2026.git "$checkout"
fi
test "$(git -C "$checkout" describe --tags --exact-match)" = v2.31.10
run="$(mktemp -d "$TMPDIR/e2e/run.XXXXXX")"
project="sbp-e2e-$(basename "$run" | tr '[:upper:].' '[:lower:]-')-$$"
export APP_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
export POSTGRES_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
export PAYROLL_ENCRYPTION_SECRET="$(openssl rand -hex 32)"
# Compose needs its secret during teardown too. The run file supports --keep cleanup.
umask 077
printf 'APP_PORT=%s\nPOSTGRES_PORT=%s\nPAYROLL_ENCRYPTION_SECRET=%s\n' "$APP_PORT" "$POSTGRES_PORT" "$PAYROLL_ENCRYPTION_SECRET" > "$run/compose.env"
compose() { docker compose --env-file "$run/compose.env" -f "$checkout/docker-compose.yml" -p "$project" "$@"; }
passed=0
cleanup() {
  status=$?
  if test -f "$run/closing.json"; then
    SBP_E2E_CLOSING_ACTION=restore go test ./internal/client -run '^TestE2EClosingDate$' -count=1 || status=1
  fi
  if test "$keep" = 1 && test "$passed" = 1; then
    echo "kept: $SBP_E2E_URL"
    echo "bookkeeper token file: $SBP_E2E_TOKEN"
    echo "readonly token file: $SBP_E2E_READONLY_TOKEN"
    echo "teardown: docker compose --env-file '$run/compose.env' -f '$checkout/docker-compose.yml' -p '$project' down -v --rmi local"
  else
    if test "$status" != 0; then compose logs --tail 60; fi
    compose down -v --rmi local --remove-orphans
    rm -r "$run"
    echo 'e2e: stack torn down'
  fi
  exit "$status"
}
trap cleanup EXIT
compose up -d --build --wait --wait-timeout 300
export SBP_E2E_URL="http://127.0.0.1:$APP_PORT"
export SBP_E2E_TOKEN="$run/bookkeeper.token"
export SBP_E2E_READONLY_TOKEN="$run/readonly.token"
export SBP_E2E_SNAPSHOT="$run/openapi.json"
export SBP_E2E_CLOSING_STATE="$run/closing.json"
# HTTP bootstrap and version checks live at the same client seam as unit tests.
go test ./internal/client -run '^TestE2E$' -count=1 -v
# M1: use only this stack's tokens and isolated CLI state.
export SLOWBOOKS_CONFIG_DIR="$run/config" SLOWBOOKS_CACHE_DIR="$run/cache"
export GOCACHE="$(go env GOCACHE)" GOMODCACHE="$(go env GOMODCACHE)"
export HOME="$run/home"
mkdir -p "$HOME"
unset SLOWBOOKS_URL SLOWBOOKS_TOKEN_FILE SLOWBOOKS_PROFILE SLOWBOOKS_COMPANY SLOWBOOKS_TIMEOUT_SECONDS
go build -o "$run/sbp" ./cmd/sbp
"$run/sbp" auth login --url "$SBP_E2E_URL" < "$SBP_E2E_TOKEN" > "$run/login.json"
"$run/sbp" auth status --json > "$run/status.json"
python3 -c 'import json,sys; assert json.load(open(sys.argv[1]))["data"]["role"]=="bookkeeper"' "$run/status.json"
"$run/sbp" auth login --profile readonly --url "$SBP_E2E_URL" < "$SBP_E2E_READONLY_TOKEN" > "$run/login-readonly.json"
"$run/sbp" auth status --profile readonly --json > "$run/status-readonly.json"
python3 -c 'import json,sys; assert json.load(open(sys.argv[1]))["data"]["role"]=="readonly"' "$run/status-readonly.json"
printf 'sbp_wrong\n' > "$run/wrong.token"
status=0
SLOWBOOKS_TOKEN_FILE="$run/wrong.token" "$run/sbp" auth status --json > "$run/wrong.json" || status=$?
test "$status" -eq 3
"$run/sbp" doctor --json > "$run/doctor.json"
python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["ok"] and all(c["ok"] for c in d["data"]["checks"])' "$run/doctor.json"
export SLOWBOOKS_CONFIG_DIR="$run/env-config" SLOWBOOKS_URL="$SBP_E2E_URL" SLOWBOOKS_TOKEN_FILE="$SBP_E2E_TOKEN"
"$run/sbp" version --json > "$run/env-version.json"
"$run/sbp" doctor --json > "$run/env-doctor.json"
python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["ok"] and all(c["ok"] for c in d["data"]["checks"])' "$run/env-doctor.json"
echo 'e2e: M1 auth, roles, wrong token, profiles and env-only doctor passed'
# M2 generic catalog, writes, key guard, and no-write dry-run.
"$run/sbp" ops find customer --json > "$run/find.json"
python3 -c 'import json,sys; assert any(o["method"]=="POST" and o["path"]=="/api/customers" for o in json.load(open(sys.argv[1]))["data"])' "$run/find.json"
"$run/sbp" ops call POST /api/customers --data '{"name":"M2 Generic Customer"}' > "$run/customer.json"
"$run/sbp" api GET /api/customers > "$run/before.json"
python3 -c 'import json,sys; assert any(o["name"]=="M2 Generic Customer" for o in json.load(open(sys.argv[1]))["data"])' "$run/before.json"
status=0
"$run/sbp" ops call POST /api/payments > "$run/key.json" || status=$?
test "$status" -eq 2
"$run/sbp" api POST /api/customers --data '{"name":"Must not be created"}' --dry-run > "$run/dry.json"
"$run/sbp" api GET /api/customers > "$run/after.json"
python3 -c 'import json,sys; assert len(json.load(open(sys.argv[1]))["data"])==len(json.load(open(sys.argv[2]))["data"])' "$run/before.json" "$run/after.json"
echo 'e2e: M2 catalog, generic customer create/list, key guard and dry-run passed'
# M3 named reads; setup writes use the generic layer.
today="$(date +%Y-%m-%d)"
year_start="$(date +%Y)-01-01"
"$run/sbp" ops call POST /api/customers --data '{"name":"M3 Named Customer"}' > "$run/m3-customer.json"
customer_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["data"]["id"])' "$run/m3-customer.json")"
invoice_body="$(python3 -c 'import json,sys; print(json.dumps({"customer_id":int(sys.argv[1]),"date":sys.argv[2],"lines":[{"description":"M3 service","quantity":"1","rate":"100","amount":"100"}]}))' "$customer_id" "$today")"
"$run/sbp" ops call POST /api/invoices --data "$invoice_body" > "$run/m3-invoice.json"
invoice_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["data"]["id"])' "$run/m3-invoice.json")"
"$run/sbp" customers list --all > "$run/m3-customers.json"
"$run/sbp" customers show "$customer_id" > "$run/m3-customer-show.json"
"$run/sbp" invoices list --all > "$run/m3-invoices.json"
"$run/sbp" invoices show "$invoice_id" > "$run/m3-invoice-show.json"
python3 -c 'import json,sys; rows=json.load(open(sys.argv[1]))["data"]; shown=json.load(open(sys.argv[2]))["data"]; assert shown["id"]==int(sys.argv[3]) and any(r["id"]==shown["id"] for r in rows)' "$run/m3-customers.json" "$run/m3-customer-show.json" "$customer_id"
python3 -c 'import json,sys; rows=json.load(open(sys.argv[1]))["data"]; shown=json.load(open(sys.argv[2]))["data"]; assert shown["id"]==int(sys.argv[3]) and any(r["id"]==shown["id"] for r in rows)' "$run/m3-invoices.json" "$run/m3-invoice-show.json" "$invoice_id"
"$run/sbp" reports list > "$run/m3-reports.json"
python3 -c 'import json,sys; assert any(r["name"]=="profit-loss" for r in json.load(open(sys.argv[1]))["data"])' "$run/m3-reports.json"
"$run/sbp" reports run profit-loss --from "$year_start" --to "$today" > "$run/m3-report.json"
python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["ok"] and isinstance(d["data"],dict) and d["data"]' "$run/m3-report.json"
"$run/sbp" search 'M3 Named Customer' > "$run/m3-search.json"
python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["ok"] and "M3 Named Customer" in json.dumps(d["data"])' "$run/m3-search.json"
"$run/sbp" customers list --all --jq '.data[].id' > "$run/m3-jq.jsonl"
python3 -c 'import json,sys; assert int(sys.argv[2]) in [json.loads(line) for line in open(sys.argv[1])]' "$run/m3-jq.jsonl" "$customer_id"
# Skeptic regressions: open filtering, duplicate detail and company refusal.
"$run/sbp" invoices list --open --all > "$run/open-invoices.json"
"$run/sbp" bills list --open --all > "$run/open-bills.json"
python3 -c 'import json,sys; rows=json.load(open(sys.argv[1]))["data"]; assert any(r["id"]==int(sys.argv[2]) for r in rows); assert all(r["status"] not in ("paid","void") for r in rows)' "$run/open-invoices.json" "$invoice_id"
status=0
"$run/sbp" ops call POST /api/customers --data '{"name":"M3 Named Customer"}' > "$run/duplicate.json" || status=$?
test "$status" -eq 6
python3 -c 'import json,sys; e=json.load(open(sys.argv[1]))["error"]; assert e["message"]==e["detail"]["message"] and "similarly-named" in e["message"]' "$run/duplicate.json"
status=0
"$run/sbp" doctor --company bogus > "$run/bogus-doctor.json" || status=$?
test "$status" -eq 2
python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert any(c["name"]=="companies" and not c["ok"] for c in d["data"]["checks"])' "$run/bogus-doctor.json"
for verb in named generic; do
  status=0
  if test "$verb" = named; then
    "$run/sbp" invoices create --company bogus --customer "$customer_id" --date "$today" --data '{"lines":[]}' > "$run/bogus-write.json" || status=$?
  else
    "$run/sbp" api POST /api/invoices --company bogus --data "$invoice_body" > "$run/bogus-write.json" || status=$?
  fi
  test "$status" -eq 2
  python3 -c 'import json,sys; e=json.load(open(sys.argv[1]))["error"]; assert e["code"]=="usage" and "available companies" in e["hint"]' "$run/bogus-write.json"
done
"$run/sbp" invoices list --all > "$run/company-after.json"
python3 -c 'import json,sys; a,b=[json.load(open(p))["data"] for p in sys.argv[1:]]; assert len(a)==len(b)' "$run/m3-invoices.json" "$run/company-after.json"
echo 'e2e: skeptic --open, duplicate sentence and bogus company guards passed'
echo 'e2e: M3 customer/invoice list/show, reports, search and jq passed'
# M4 real invoice PDF and CSV streaming.
"$run/sbp" ops call POST /api/customers --data '{"name":"M4 Download Customer"}' > "$run/m4-customer.json"
customer_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["data"]["id"])' "$run/m4-customer.json")"
"$run/sbp" ops call POST /api/invoices --data "{\"customer_id\":$customer_id,\"date\":\"2026-11-09\",\"lines\":[{\"description\":\"Download test\",\"quantity\":1,\"rate\":10,\"amount\":10}]}" > "$run/m4-invoice.json"
invoice_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["data"]["id"])' "$run/m4-invoice.json")"
(cd "$run" && ./sbp invoices pdf "$invoice_id" --json > m4-pdf.json)
python3 -c 'import json,sys,pathlib; p=pathlib.Path(sys.argv[1]); d=json.load(open(p/"m4-pdf.json"))["data"]; assert d["filename"].endswith(".pdf") and d["path"]==d["filename"]; assert (p/d["path"]).read_bytes().startswith(b"%PDF")' "$run"
"$run/sbp" download /api/csv/export/customers -o - > "$run/m4-customers.csv"
python3 -c 'import csv,sys; rows=list(csv.reader(open(sys.argv[1]))); assert len(rows)>1 and any("M4 Download Customer" in row for row in rows)' "$run/m4-customers.csv"
echo 'e2e: M4 invoice PDF filename/signature and CSV stdout passed'

# M5 named writes, payment replay and role refusal.
"$run/sbp" ops call POST /api/customers --data '{"name":"Harbor Orchard Receivables"}' > "$run/m5-customer.json"
customer_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["data"]["id"])' "$run/m5-customer.json")"
"$run/sbp" invoices create --customer "$customer_id" --date "$today" --memo 'M5 service' --data '{"lines":[{"description":"M5 service","quantity":"1","rate":"100","amount":"100"}]}' > "$run/m5-invoice.json"
invoice_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["data"]["id"])' "$run/m5-invoice.json")"
"$run/sbp" payments record --key e2e-1 --customer "$customer_id" --amount 100.00 --date "$today" --apply "$invoice_id=100.00" > "$run/m5-payment.json"
"$run/sbp" payments list --all > "$run/m5-before.json"
"$run/sbp" payments record --key e2e-1 --customer "$customer_id" --amount 100.00 --date "$today" --apply "$invoice_id=100.00" > "$run/m5-replay.json"
"$run/sbp" payments list --all > "$run/m5-after.json"
python3 -c 'import json,sys; a,b,c,d=[json.load(open(p))["data"] for p in sys.argv[1:]]; assert a["id"]==b["id"] and len(c)==len(d)' "$run/m5-payment.json" "$run/m5-replay.json" "$run/m5-before.json" "$run/m5-after.json"
status=0
"$run/sbp" payments record --key e2e-1 --customer "$customer_id" --amount 99 --date "$today" --apply "$invoice_id=99" > "$run/m5-conflict.json" || status=$?
test "$status" -eq 6
payment_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["data"]["id"])' "$run/m5-payment.json")"
"$run/sbp" payments void "$payment_id" > "$run/m5-payment-void.json"
"$run/sbp" invoices void "$invoice_id" > "$run/m5-invoice-void.json"
python3 -c 'import json,sys; assert json.load(open(sys.argv[1]))["data"]["is_voided"] is True and json.load(open(sys.argv[2]))["data"]["status"]=="void"' "$run/m5-payment-void.json" "$run/m5-invoice-void.json"
status=0
SLOWBOOKS_TOKEN_FILE="$SBP_E2E_READONLY_TOKEN" "$run/sbp" invoices create --customer "$customer_id" --date "$today" > "$run/m5-readonly.json" || status=$?
test "$status" -eq 4
python3 -c 'import json,sys; e=json.load(open(sys.argv[1]))["error"]; assert e["code"]=="refused" and e["message"]==e["detail"]' "$run/m5-readonly.json"
SBP_E2E_CLOSING_ACTION=set go test ./internal/client -run '^TestE2EClosingDate$' -count=1
status=0
"$run/sbp" invoices create --customer "$customer_id" --date 2026-01-01 --data '{"lines":[{"description":"Closed","quantity":"1","rate":"10","amount":"10"}]}' > "$run/m5-closed.json" || status=$?
test "$status" -eq 4
python3 -c 'import json,sys; e=json.load(open(sys.argv[1]))["error"]; assert e["code"]=="refused" and e["message"]==e["detail"] and "2026-01-31" in e["message"]' "$run/m5-closed.json"
SBP_E2E_CLOSING_ACTION=restore go test ./internal/client -run '^TestE2EClosingDate$' -count=1
echo 'e2e: M5 closing-date CLI refusal preserves the server sentence; setting restored'
echo 'e2e: M5 invoice create, payment replay/count, key conflict, voids and readonly refusal passed'
if test "${SBP_E2E_CAPTURE_SNAPSHOT:-0}" = 1; then
  cp "$SBP_E2E_SNAPSHOT" testdata/openapi-2.31.10.json
fi
python3 -m json.tool --sort-keys testdata/openapi-2.31.10.json > "$run/expected.json"
diff -u "$run/expected.json" "$SBP_E2E_SNAPSHOT"
echo 'e2e: snapshot diff clean'
passed=1
