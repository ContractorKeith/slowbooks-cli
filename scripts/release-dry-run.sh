#!/usr/bin/env bash
# Build exactly the release artifacts locally; never tags or publishes. Bash 3.2.
set -eu
set -o pipefail
cd "$(dirname "$0")/.."
export TMPDIR="${TMPDIR:-$PWD/.tmp}" CGO_ENABLED=0
mkdir -p "$TMPDIR" .tmp/dist
version="${SBP_RELEASE_VERSION:-v0.0.0}"
bash scripts/check-release-version.sh "$version"
commit="$(git rev-parse HEAD)"
scratch="$(mktemp -d "$TMPDIR/release.XXXXXX")"
trap 'rm -r "$scratch"' EXIT
# A clean output directory prevents old archives entering checksums or uploads.
python3 - <<'PY'
from pathlib import Path
for file in Path('.tmp/dist').iterdir():
    if file.is_file():
        file.unlink()
PY
for os in linux darwin windows; do
  label="$os"; test "$os" != darwin || label=macos
  for arch in amd64 arm64; do
    target="$arch"; test "$arch" != amd64 || target=x64
    binary=slowbooks; ext=tar.gz
    if test "$os" = windows; then binary=slowbooks.exe; ext=zip; fi
    GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "-s -w -X main.version=$version -X main.commit=$commit" -o "$scratch/$binary" ./cmd/slowbooks
    python3 - "$scratch/$binary" ".tmp/dist/slowbooks_${version#v}_${label}-${target}.$ext" <<'PY'
import sys, tarfile, zipfile
from pathlib import Path
binary, dest = map(Path, sys.argv[1:])
if dest.suffix == '.zip':
    with zipfile.ZipFile(dest, 'w', zipfile.ZIP_DEFLATED) as archive:
        archive.write(binary, binary.name)
        for doc in ("LICENSE", "README.md"):
            archive.write(doc, doc)
else:
    with tarfile.open(dest, 'w:gz') as archive:
        archive.add(binary, arcname=binary.name)
        for doc in ("LICENSE", "README.md"):
            archive.add(doc, arcname=doc)
PY
  done
done
python3 - <<'PY'
import hashlib
from pathlib import Path
dist = Path('.tmp/dist')
files = sorted(dist.glob('*.tar.gz')) + sorted(dist.glob('*.zip'))
(dist / 'SHA256SUMS').write_text(''.join(hashlib.sha256(f.read_bytes()).hexdigest() + '  ' + f.name + '\n' for f in files))
PY
go build -trimpath -ldflags "-s -w -X main.version=$version -X main.commit=$commit" -o "$scratch/slowbooks-host" ./cmd/slowbooks
"$scratch/slowbooks-host" version --json > "$scratch/version.json"
python3 - "$scratch/version.json" <<'PY'
import json, sys
from pathlib import Path
versions = json.loads(Path(sys.argv[1]).read_text())['data']['verified_sbp']
Path('.tmp/dist/NOTES.md').write_text('Verified against SlowBooks Pro ' + ', '.join(versions) + '\n')
PY
echo 'release dry run: ok (six archives, SHA256SUMS, NOTES.md)'
