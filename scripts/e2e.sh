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
    echo "bookkeeper token: $SBP_E2E_TOKEN"
    echo "readonly token: $SBP_E2E_READONLY_TOKEN"
    echo "teardown: docker compose --env-file '$run/compose.env' -f '$checkout/docker-compose.yml' -p '$project' down -v"
  else
    if test "$status" != 0; then compose logs --tail 60; fi
    compose down -v --remove-orphans
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
# M1–M5: append milestone blocks here, using the exported URL and token paths.
if test "${SBP_E2E_CAPTURE_SNAPSHOT:-0}" = 1; then
  cp "$SBP_E2E_SNAPSHOT" testdata/openapi-2.31.10.json
fi
python3 -m json.tool --sort-keys testdata/openapi-2.31.10.json > "$run/expected.json"
diff -u "$run/expected.json" "$SBP_E2E_SNAPSHOT"
echo 'e2e: snapshot diff clean'
passed=1
