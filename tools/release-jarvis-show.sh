#!/usr/bin/env bash
# Publish one Jarvis Show release from already-built, already-verified artifacts.
# This script NEVER builds a rootfs or boot image. CI/local build jobs produce those separately.
set -euo pipefail

REPO=${JARVIS_SHOW_RELEASE_REPO:-bakanura/techo5JarvisAssistant}
ROOT=$(cd "$(dirname "$0")/.." && pwd)
VERSION=
NOTES=
BINARY=
ROOTFS=
SIGN_KEY=${JARVIS_SHOW_SIGN_KEY:-}
CROWN_BOOT=
CHECKERS_BOOT=
PUBLISH=
OUT=${JARVIS_SHOW_RELEASE_OUT:-$ROOT/bin/release}

usage() {
  cat <<USAGE
usage: $0 --version vX.Y.Z[...prerelease] --notes TEXT --binary FILE --rootfs FILE \\
          --sign-key FILE [--crown-boot FILE] [--checkers-boot FILE] [--publish]

Without --publish the script only validates artifacts and writes signed manifest/SHA256SUMS into --out.
It does not build the rootfs or boot images.
USAGE
}

while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION=$2; shift 2;;
    --notes) NOTES=$2; shift 2;;
    --binary) BINARY=$2; shift 2;;
    --rootfs) ROOTFS=$2; shift 2;;
    --sign-key) SIGN_KEY=$2; shift 2;;
    --crown-boot) CROWN_BOOT=$2; shift 2;;
    --checkers-boot) CHECKERS_BOOT=$2; shift 2;;
    --out) OUT=$2; shift 2;;
    --publish) PUBLISH=1; shift;;
    -h|--help) usage; exit 0;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2;;
  esac
done

[[ $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]] || { echo "invalid --version: $VERSION" >&2; exit 2; }
[ -n "$NOTES" ] || { echo "--notes is required" >&2; exit 2; }
for pair in "binary:$BINARY" "rootfs:$ROOTFS" "signing key:$SIGN_KEY"; do
  label=${pair%%:*}; path=${pair#*:}
  [ -f "$path" ] || { echo "$label not found: $path" >&2; exit 2; }
done
command -v go >/dev/null || { echo "go not found" >&2; exit 2; }
if [ -n "$PUBLISH" ]; then
  command -v gh >/dev/null || { echo "gh not found" >&2; exit 2; }
fi

mkdir -p "$OUT"
chmod 700 "$OUT"
cp "$BINARY" "$OUT/echod-arm"
cp "$ROOTFS" "$OUT/jarvis-show-rootfs-$VERSION.tar.gz"
assets=()
if [ -n "$CROWN_BOOT" ]; then
  [ -f "$CROWN_BOOT" ] || { echo "Crown boot image not found: $CROWN_BOOT" >&2; exit 2; }
  cp "$CROWN_BOOT" "$OUT/techo5-boot-crown-$VERSION.img"
  assets+=("$OUT/techo5-boot-crown-$VERSION.img")
fi
if [ -n "$CHECKERS_BOOT" ]; then
  [ -f "$CHECKERS_BOOT" ] || { echo "Checkers boot image not found: $CHECKERS_BOOT" >&2; exit 2; }
  cp "$CHECKERS_BOOT" "$OUT/techo5-boot-checkers-$VERSION.img"
  assets+=("$OUT/techo5-boot-checkers-$VERSION.img")
fi

# Rootfs has one shared product marker and must explicitly support both first-generation boards.
rootfs_sha=$(sha256sum "$ROOTFS" | awk '{print $1}')
PYTHONPATH="$ROOT/tools" python3 - "$ROOTFS" "$rootfs_sha" <<'PY'
import pathlib, sys
from jarvis_crown.install import verify_jarvis_rootfs
path = pathlib.Path(sys.argv[1])
sha = sys.argv[2]
for board in ("crown", "checkers"):
    ident = verify_jarvis_rootfs(path, sha, board=board)
    if ident.product != "jarvis-show-v1":
        raise SystemExit("rootfs is not a shared jarvis-show-v1 image")
print("PASS: shared Jarvis Show rootfs marker")
PY

from="https://github.com/$REPO/releases/download/$VERSION"
mk=(run ./cmd/mkmanifest
  -version "$VERSION"
  -title "Jarvis Show $VERSION"
  -notes "$NOTES"
  -release-url "https://github.com/$REPO/releases/tag/$VERSION"
  -from "$from"
  -arm "$OUT/echod-arm"
  -rootfs-arm "$OUT/jarvis-show-rootfs-$VERSION.tar.gz"
  -out "$OUT/manifest.json"
  -sign-key "$SIGN_KEY")
for asset in "${assets[@]}"; do mk+=(-asset "$asset"); done
(cd "$ROOT/echod" && go "${mk[@]}")

(
  cd "$OUT"
  files=(echod-arm "jarvis-show-rootfs-$VERSION.tar.gz" manifest.json manifest.json.sig)
  [ -z "$CROWN_BOOT" ] || files+=("techo5-boot-crown-$VERSION.img")
  [ -z "$CHECKERS_BOOT" ] || files+=("techo5-boot-checkers-$VERSION.img")
  sha256sum "${files[@]}" > SHA256SUMS
)

echo "PASS: signed release payload prepared in $OUT"
[ -n "$PUBLISH" ] || exit 0

release_args=(release create "$VERSION" --repo "$REPO" --title "Jarvis Show $VERSION" --notes "$NOTES")
[[ $VERSION == *-* ]] && release_args+=(--prerelease)
release_args+=("$OUT/echod-arm" "$OUT/jarvis-show-rootfs-$VERSION.tar.gz" "$OUT/manifest.json" "$OUT/manifest.json.sig" "$OUT/SHA256SUMS")
release_args+=("${assets[@]}")
gh "${release_args[@]}"

# Rolling dev only moves forward. The manifest continues to point at immutable versioned release assets.
current=
if gh release view dev --repo "$REPO" >/dev/null 2>&1; then
  current=$(gh release download dev --repo "$REPO" -p manifest.json -O - 2>/dev/null | python3 -c 'import json,sys; print(json.load(sys.stdin).get("version", ""))' || true)
else
  gh release create dev --repo "$REPO" --prerelease --title "Jarvis Show dev" --notes "Rolling Jarvis Show development channel."
fi
set +e
python3 - "$VERSION" "$current" <<'PY'
import re, sys

def parse(v):
    m = re.fullmatch(r"v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.]+))?", v or "")
    if not m: return None
    core = tuple(map(int, m.group(1,2,3)))
    pre = m.group(4)
    fields = [] if pre is None else [int(x) if x.isdigit() else x for x in pre.split('.')]
    return core, pre is None, fields

def newer(a,b):
    if not b: return True
    pa,pb=parse(a),parse(b)
    if not pa or not pb: return False
    if pa[0] != pb[0]: return pa[0] > pb[0]
    if pa[1] != pb[1]: return pa[1] and not pb[1]
    for x,y in zip(pa[2],pb[2]):
        if x == y: continue
        if isinstance(x,int) and isinstance(y,int): return x > y
        return str(x) > str(y)
    return len(pa[2]) > len(pb[2])

sys.exit(0 if newer(sys.argv[1], sys.argv[2]) else 10)
PY
compare_status=$?
set -e
case $compare_status in
  0)
    gh release upload dev "$OUT/manifest.json" "$OUT/manifest.json.sig" --repo "$REPO" --clobber
    echo "PASS: dev channel advanced to $VERSION"
    ;;
  10) echo "PASS: dev channel left on newer/equal $current";;
  *) exit $?;;
esac

echo "PASS: published https://github.com/$REPO/releases/tag/$VERSION"
