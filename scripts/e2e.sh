#!/usr/bin/env bash
# Pinned real-server harness. Run on Linux with Docker access.
set -eu
cd "$(dirname "$0")/.."
export TMPDIR="$PWD/.tmp"
mkdir -p "$TMPDIR/e2e"
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
  if test "$keep" = 1 && test "$passed" = 1; then
    echo "kept: $SBP_E2E_URL"
    echo "bookkeeper token file: $SBP_E2E_TOKEN"
    echo "readonly token file: $SBP_E2E_READONLY_TOKEN"
    echo "teardown: docker compose --env-file '$run/compose.env' -f '$checkout/docker-compose.yml' -p '$project' down -v"
  else
    if test "$status" != 0; then compose logs --tail 60; fi
    compose down -v --remove-orphans
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
if test "${SBP_E2E_CAPTURE_SNAPSHOT:-0}" = 1; then
  cp "$SBP_E2E_SNAPSHOT" testdata/openapi-2.31.10.json
fi
python3 -m json.tool --sort-keys testdata/openapi-2.31.10.json > "$run/expected.json"
diff -u "$run/expected.json" "$SBP_E2E_SNAPSHOT"
echo 'e2e: snapshot diff clean'
passed=1
