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
CHANNEL=
TARGET=
OUT=${JARVIS_SHOW_RELEASE_OUT:-$ROOT/bin/release}

usage() {
  cat <<USAGE
usage: $0 --version vX.Y.Z[...prerelease] --notes TEXT --binary FILE --rootfs FILE \\
          --sign-key FILE [--crown-boot FILE] [--checkers-boot FILE]
          [--channel stable|staging|dev] [--target COMMIT] [--publish]

Without --publish the script only validates artifacts and writes signed manifest/SHA256SUMS into --out.
It does not build the rootfs or boot images.

--channel says which update channel the release is for. Without it, a version without a suffix is
stable, -rc.N is staging and anything else is dev. A release moves its own channel and every less
stable one forward (stable: stable, staging, dev; staging: staging, dev), never back.
--target is the commit a new release tag is made on; without it GitHub uses the default branch.
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
    --channel) CHANNEL=$2; shift 2;;
    --target) TARGET=$2; shift 2;;
    --publish) PUBLISH=1; shift;;
    -h|--help) usage; exit 0;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2;;
  esac
done

[[ $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]] || { echo "invalid --version: $VERSION" >&2; exit 2; }
[ -n "$NOTES" ] || { echo "--notes is required" >&2; exit 2; }
if [ -z "$CHANNEL" ]; then
  case $VERSION in
    *-rc.*) CHANNEL=staging;;
    *-*) CHANNEL=dev;;
    *) CHANNEL=stable;;
  esac
fi
case $CHANNEL in
  stable) [[ $VERSION != *-* ]] || { echo "a stable release cannot be a prerelease: $VERSION" >&2; exit 2; };;
  staging|dev) [[ $VERSION == *-* ]] || { echo "a $CHANNEL release must be a prerelease (vX.Y.Z-...): $VERSION" >&2; exit 2; };;
  *) echo "invalid --channel: $CHANNEL" >&2; exit 2;;
esac
for pair in "binary:$BINARY" "rootfs:$ROOTFS" "signing key:$SIGN_KEY"; do
  label=${pair%%:*}; path=${pair#*:}
  [ -f "$path" ] || { echo "$label not found: $path" >&2; exit 2; }
done
python3 - "$SIGN_KEY" <<'PYKEY'
import os, stat, sys
p = sys.argv[1]
if os.path.islink(p):
    raise SystemExit("signing key must not be a symlink")
st = os.stat(p)
if not stat.S_ISREG(st.st_mode):
    raise SystemExit("signing key is not a regular file")
if st.st_mode & 0o077:
    raise SystemExit("signing key permissions are too open; require owner-only access (0600 or stricter)")
PYKEY
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
PYTHONPATH="$ROOT/tools" python3 - "$ROOTFS" "$rootfs_sha" "$VERSION" <<'PY'
import pathlib, sys
from jarvis_crown.install import verify_jarvis_rootfs
path = pathlib.Path(sys.argv[1])
sha = sys.argv[2]
for board in ("crown", "checkers"):
    ident = verify_jarvis_rootfs(path, sha, board=board)
    if ident.product != "jarvis-show-v1":
        raise SystemExit("rootfs is not a shared jarvis-show-v1 image")
    if ident.version != sys.argv[3]:
        raise SystemExit(f"rootfs marker version {ident.version!r} does not match release filename")
print("PASS: shared Jarvis Show rootfs marker/version")
PY

from="https://github.com/$REPO/releases/download/$VERSION"
mk=(run ./cmd/mkmanifest
  -version "$VERSION"
  -product "jarvis-show-v1"
  -board crown
  -board checkers
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
[ -z "$TARGET" ] || release_args+=(--target "$TARGET")
# Stable is GitHub's "latest" release; everything else is a prerelease, so a stable device never sees it.
if [ "$CHANNEL" = stable ]; then release_args+=(--latest); else release_args+=(--prerelease --latest=false); fi
release_args+=("$OUT/echod-arm" "$OUT/jarvis-show-rootfs-$VERSION.tar.gz" "$OUT/manifest.json" "$OUT/manifest.json.sig" "$OUT/SHA256SUMS")
release_args+=("${assets[@]}")
gh "${release_args[@]}"

# Staging and dev are rolling releases (channel-staging, channel-dev) holding only the signed manifest of
# the release they point at; the manifest names that release's own immutable assets. A release moves its
# own channel and the less stable ones, so dev never offers something older than staging or stable: a
# unit would refuse it and its update card would never clear (techo5 issue #42). A channel never moves
# back. The tags are not the branch names because a tag and a branch both called dev confuse git.
case $CHANNEL in
  stable|staging) rolling=(staging dev);;
  dev) rolling=(dev);;
esac

newer() {
  python3 - "$1" "$2" <<'PY'
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
}

for name in "${rolling[@]}"; do
  tag="channel-$name"
  current=
  if gh release view "$tag" --repo "$REPO" >/dev/null 2>&1; then
    current=$(gh release download "$tag" --repo "$REPO" -p manifest.json -O - 2>/dev/null | python3 -c 'import json,sys; print(json.load(sys.stdin).get("version", ""))' || true)
  else
    create=(release create "$tag" --repo "$REPO" --prerelease --latest=false --title "Jarvis Show $name channel"
      --notes "Rolling Jarvis Show $name channel. It holds only the signed manifest of the release it points at.")
    [ -z "$TARGET" ] || create+=(--target "$TARGET")
    gh "${create[@]}"
  fi
  set +e
  newer "$VERSION" "$current"
  compare_status=$?
  set -e
  case $compare_status in
    0)
      gh release upload "$tag" "$OUT/manifest.json" "$OUT/manifest.json.sig" --repo "$REPO" --clobber
      echo "PASS: $name channel advanced to $VERSION"
      ;;
    10) echo "PASS: $name channel left on newer/equal $current";;
    *) exit "$compare_status";;
  esac
done

echo "PASS: published https://github.com/$REPO/releases/tag/$VERSION ($CHANNEL)"
