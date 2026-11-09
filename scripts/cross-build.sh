#!/usr/bin/env bash
# Keep all release targets in one CI job so each run verifies the complete set.
set -eu
cd "$(dirname "$0")/.."
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  target_os="${target%/*}"
  target_arch="${target#*/}"
  echo "cross-build: $target"
  GOOS="$target_os" GOARCH="$target_arch" CGO_ENABLED=0 go build ./...
done
echo 'cross-build: all six targets passed'
