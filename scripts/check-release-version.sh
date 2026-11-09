#!/usr/bin/env bash
# Fail before tests, builds or publishing for non-stable tags. Bash 3.2.
set -eu
version="${1:-}"
stable='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
if ! [[ "$version" =~ $stable ]]; then
  echo "release requires a stable vX.Y.Z tag; got: $version" >&2
  exit 2
fi
