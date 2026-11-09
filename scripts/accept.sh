#!/usr/bin/env bash
# Fast offline acceptance; Bash 3.2 compatible.
set -eu
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
export SLOWBOOKS_URL="$(cat "$scratch/fake.url")"
# Run the fake/client seam against this separately built process.
SBP_ACCEPT_URL="$SLOWBOOKS_URL" go test ./internal/client -run '^TestAcceptFake$' -count=1

echo '== goldens'
"$scratch/sbp" version --json | diff - testdata/golden/version.json
echo '== live exit-code checks'
check_exit() {
  expected="$1"; shift
  status=0
  "$scratch/sbp" "$@" > "$scratch/result.json" || status=$?
  test "$status" -eq "$expected"
}
check_exit 0 version --json
check_exit 2 nosuch
check_exit 2 version --timeout 0
check_exit 2 version --jq .data
# All error codes are also exercised in process at the root seam by the gate.
echo 'accept: ok'
