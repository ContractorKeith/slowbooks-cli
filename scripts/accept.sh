#!/usr/bin/env bash
# Fast offline acceptance; Bash 3.2 compatible.
set -eu
set -o pipefail
cd "$(dirname "$0")/.."
export TMPDIR="${TMPDIR:-$PWD/.tmp}"
mkdir -p "$TMPDIR"
scratch="$(mktemp -d "$TMPDIR/accept.XXXXXX")"
fake_pid=''
cleanup() {
  if test -n "$fake_pid"; then kill "$fake_pid" 2>/dev/null || true; wait "$fake_pid" 2>/dev/null || true; fi
  rm -r "$scratch"
}
trap cleanup EXIT
# Isolate the gate too; Go cache locations must survive changing HOME.
export GOCACHE="$(go env GOCACHE)" GOMODCACHE="$(go env GOMODCACHE)"
export HOME="$scratch/home" SLOWBOOKS_CONFIG_DIR="$scratch/config" SLOWBOOKS_CACHE_DIR="$scratch/cache"
mkdir -p "$HOME" "$SLOWBOOKS_CONFIG_DIR" "$SLOWBOOKS_CACHE_DIR"
unset SLOWBOOKS_URL SLOWBOOKS_TOKEN_FILE SLOWBOOKS_COMPANY SLOWBOOKS_PROFILE SLOWBOOKS_TIMEOUT_SECONDS SBP_E2E_URL

echo '== gate'
go test -race ./...
test -z "$(gofmt -l $(git ls-files '*.go'))"
go vet ./...
staticcheck ./...
echo '== build'
go build -trimpath -ldflags '-X main.version=v0.0.0-accept -X main.commit=accept' -o "$scratch/sbp" ./cmd/sbp
go build -trimpath -o "$scratch/sbp-fake" ./cmd/sbp-fake
"$scratch/sbp-fake" > "$scratch/fake.url" &
fake_pid=$!
i=0
while test ! -s "$scratch/fake.url"; do
  kill -0 "$fake_pid"
  i=$((i+1)); test "$i" -lt 100
  sleep 0.05
done
fake_url="$(cat "$scratch/fake.url")"
export SLOWBOOKS_URL="$fake_url"
# Run the fake/client seam against this separately built process.
SBP_ACCEPT_URL="$SLOWBOOKS_URL" go test ./internal/client -run '^TestAcceptFake$' -count=1

printf 'sbp_bookkeeper\n' > "$scratch/input.token"
chmod 600 "$scratch/input.token"
"$scratch/sbp" auth login --url "$SLOWBOOKS_URL" --company acme < "$scratch/input.token" > "$scratch/login.json"
# Normalize only volatile URL and scratch paths; assertions remain in goldens.
normalize() {
  python3 -c 'import sys,os; value=sys.stdin.read().replace(sys.argv[1],"<server>").replace(os.path.normpath(os.environ["SLOWBOOKS_CONFIG_DIR"]),"<config>").replace(os.path.normpath(os.environ["SLOWBOOKS_CACHE_DIR"]),"<cache>"); sys.stdout.write(value)' "$fake_url"
}
unset SLOWBOOKS_URL
echo '== goldens'
"$scratch/sbp" version --json | diff - testdata/golden/version.json
"$scratch/sbp" auth status --json | normalize | diff - testdata/golden/auth-status.json
"$scratch/sbp" profile list --json | normalize | diff - testdata/golden/profile-list.json
"$scratch/sbp" doctor --json | normalize | diff - testdata/golden/doctor.json
"$scratch/sbp" ops find customer --json | diff - testdata/golden/ops-find.json
"$scratch/sbp" ops describe POST /api/payments --json | diff - testdata/golden/ops-describe.json
"$scratch/sbp" api POST /api/payments --key check-1 --data '{"amount":1}' --dry-run --json | normalize | diff - testdata/golden/dry-run.json
# M3 named reads and output contracts.
"$scratch/sbp" customers list | diff - testdata/golden/customers-list.json
"$scratch/sbp" invoices show 1 | diff - testdata/golden/invoices-show.json
"$scratch/sbp" reports run profit-loss | diff - testdata/golden/reports-run.json
"$scratch/sbp" customers list --jq '.data[].id' | diff - testdata/golden/customers-jq.json
"$scratch/sbp" customers list --quiet | diff - testdata/golden/customers-quiet.json
"$scratch/sbp" expenses list | diff - testdata/golden/expenses-trimmed.json
echo '== live exit-code checks'
check_exit() {
  expected="$1"; shift
  status=0
  "$scratch/sbp" "$@" > "$scratch/result.json" || status=$?
  test "$status" -eq "$expected"
}
check_exit 0 version --json
check_exit 2 ops call POST /api/payments
# The in-process fake recorder verifies zero hits, including catalog discovery.
go test ./internal/cli -run '^TestGenericCallsAndSafety$' -count=1
check_exit 2 nosuch
check_exit 2 version --timeout 0
check_exit 2 version --jq '['
printf 'sbp_wrong\n' > "$scratch/input.token"
chmod 600 "$scratch/input.token"
SLOWBOOKS_TOKEN_FILE="$scratch/input.token" check_exit 3 auth status --json
printf 'sbp_bookkeeper\n' > "$scratch/input.token"
chmod 644 "$scratch/input.token"
SLOWBOOKS_TOKEN_FILE="$scratch/input.token" check_exit 3 auth status --json
# All error codes are also exercised in process at the root seam by the gate.
# M4 download envelopes, overwrite protection and raw bytes.
echo '== M4 downloads'
(cd "$scratch" && ./sbp invoices pdf 1 --json) | diff - testdata/golden/download.json
status=0
(cd "$scratch" && ./sbp invoices pdf 1 --json) > "$scratch/exists.json" || status=$?
test "$status" -eq 2
(cd "$scratch" && ./sbp invoices pdf 1 --force --json) > "$scratch/force.json"
"$scratch/sbp" download /api/invoices/1/pdf -o - > "$scratch/raw.pdf"
cmp testdata/fake/invoice.pdf "$scratch/raw.pdf"
(cd "$scratch" && ./sbp download /api/invoices/traversal/pdf --json) > "$scratch/traversal.json"
cmp testdata/fake/invoice.pdf "$scratch/evil.pdf"
"$scratch/sbp" download /api/csv/export/customers -o - | cat > "$scratch/customers.csv"
cmp testdata/fake/customers.csv "$scratch/customers.csv"
check_exit 5 download /api/invoices/missing/pdf
echo '== M5 named writes'
# M5 each named write has a success and a no-request preview golden.
"$scratch/sbp" invoices create --customer 1 --date 2026-11-09 --due 2026-12-09 --memo Service --data '{"lines":[]}' --dry-run | normalize | diff - testdata/golden/invoices-create-dry.json
"$scratch/sbp" invoices create --customer 1 --date 2026-11-09 --due 2026-12-09 --memo Service --data '{"lines":[]}' | diff - testdata/golden/invoices-create.json
for kind in payments bill-payments; do
  party=customer; test "$kind" != bill-payments || party=vendor
  "$scratch/sbp" "$kind" record --key accept-1 --"$party" 1 --date 2026-11-09 --amount 1234.50 --apply 1=1234.50 --dry-run | normalize | diff - "testdata/golden/$kind-record-dry.json"
  "$scratch/sbp" "$kind" record --key accept-1 --"$party" 1 --date 2026-11-09 --amount 1234.50 --apply 1=1234.50 | diff - "testdata/golden/$kind-record.json"
done
for kind in invoices bills expenses journal payments bill-payments; do
  "$scratch/sbp" "$kind" void 1 --dry-run | normalize | diff - "testdata/golden/$kind-void-dry.json"
  "$scratch/sbp" "$kind" void 1 | diff - "testdata/golden/$kind-void.json"
done
"$scratch/sbp" payments void 1 --fee-refunded --dry-run | normalize | diff - testdata/golden/payments-void-fee-dry.json
check_exit 2 payments record
check_exit 2 bill-payments record
check_exit 6 payments record --key accept-1 --customer 1 --date 2026-11-09 --amount 1
check_exit 6 bill-payments record --key accept-1 --vendor 1 --date 2026-11-09 --amount 1
printf 'sbp_readonly\n' > "$scratch/input.token"
chmod 600 "$scratch/input.token"
SLOWBOOKS_TOKEN_FILE="$scratch/input.token" check_exit 4 invoices create --customer 1 --date 2026-11-09
go test ./internal/cli -run '^TestNamedWrites$' -count=1
echo 'accept: ok'
