#!/bin/bash
# wsl-build.sh — the WSL half of deploy-rootfs.sh: copy the staged inputs to the
# WSL disk, build the root filesystem there with mkrootfs.sh under QEMU user
# emulation, and print the Windows path of the tarball.
#
#   wsl-build.sh <stage dir, WSL path> <version> <timezone>
#
# Needs qemu-user-static + binfmt-support (apt), Alpine's static apk at
# ~/apk/apk.static, and a user namespace with a subuid range (Ubuntu's default)
# so the build owns files as root without sudo.
set -euo pipefail
STAGE=$1; VERSION=$2; TZNAME=$3
W=$HOME/techo5-build
APK=$HOME/apk/apk.static
[ -e /proc/sys/fs/binfmt_misc/qemu-arm ] || { echo "wsl-build: no qemu-arm binfmt (apt install qemu-user-static binfmt-support)" >&2; exit 1; }
[ -x "$APK" ] || { echo "wsl-build: no $APK" >&2; exit 1; }

rm -rf "$W"
mkdir -p "$W"
cp -r "$STAGE" "$W/in"
find "$W/in" -type f -name '*.sh' -exec sed -i 's/\r$//' {} +
# Root-only mappings cannot preserve Alpine's service users/groups (including shadow's gid 42).
for helper in newuidmap newgidmap; do
	command -v "$helper" >/dev/null || { echo "wsl-build: $helper is required; install uidmap" >&2; exit 1; }
done
"${JARVIS_ROOTFS_UNSHARE:-unshare}" -Ur --map-auto sh -c '
	echo "Rootfs build UID/GID mappings:" >&2
	cat /proc/self/uid_map /proc/self/gid_map >&2
	probe=$(mktemp "$1/ownership-check.XXXXXX") || exit 1
	trap '\''rm -f "$probe"'\'' EXIT
	chown 0:42 "$probe" && chown 65534:65534 "$probe" || {
		echo "wsl-build: namespace cannot preserve Alpine ownership; check subordinate UID/GID ranges and host AppArmor userns policy" >&2
		exit 1
	}
	rm -f "$probe"
	trap - EXIT
	shift
	exec sh "$@"
' sh "$W" "$W/in/tools/mkrootfs.sh" -i "$W/in" -o "$W/rootfs.tar.gz" -w "$W" \
	-V "$VERSION" -z "$TZNAME" -a armv7 -A "$APK"
# The path the caller copies from: a Windows path under WSL, a plain one on Linux.
wslpath -w "$W/rootfs.tar.gz" 2>/dev/null || echo "$W/rootfs.tar.gz"
