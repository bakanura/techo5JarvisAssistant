#!/bin/bash
# Allow the rootless rootfs builder on Ubuntu's AppArmor-restricted CI runner.
# Only the dedicated unshare copy receives the userns exception; no global
# kernel policy is disabled and the build still runs as the runner user.
set -euo pipefail
[[ ${GITHUB_ACTIONS:-} == true && -n ${GITHUB_ENV:-} ]] || {
    echo 'ci-rootfs-userns: intended for GitHub Actions runners only' >&2
    exit 1
}
if [[ ! -e /proc/sys/kernel/apparmor_restrict_unprivileged_userns ]] ||
   [[ $(cat /proc/sys/kernel/apparmor_restrict_unprivileged_userns) == 0 ]]; then
    exit 0
fi
builder=/usr/local/libexec/jarvis-rootfs-unshare
sudo install -D -o root -g root -m 0755 "$(command -v unshare)" "$builder"
sudo tee /etc/apparmor.d/jarvis-rootfs-unshare >/dev/null <<'PROFILE'
abi <abi/4.0>,
include <tunables/global>
/usr/local/libexec/jarvis-rootfs-unshare flags=(unconfined) {
    userns,
}
PROFILE
sudo apparmor_parser -r /etc/apparmor.d/jarvis-rootfs-unshare
echo "JARVIS_ROOTFS_UNSHARE=$builder" >> "$GITHUB_ENV"
