# Jarvis Show CROWN installation runbook

This runbook installs the audited Jarvis Show candidate on an Echo Show 8
first generation (`CROWN`). It uses the supported installer only. Do not use
manual `fastboot` writes, `dd`, or raw partition commands.

The candidate documented here is `v0.0.0-dev.32`, built from commit
`0f78d1d55feda5b7121b2d20bfcbc0e3bf8fd1f9`. Its CI run, artifact audit, and
provenance checks passed. This is an installation candidate, not a published
release.

## Before connecting the device

The installer must be run on Linux with Python 3.10+, `adb`, `fastboot`,
`git`, `bash`, and GNU `timeout` available. The active login must belong to
either `uucp` or `dialout` so recovery serial access remains available.

On NixOS, add the appropriate group to the user's `extraGroups` in the system
configuration, rebuild/switch the configuration, then log out and back in.
Confirm the new login has the group:

```sh
id -nG
```

Keep the device on mains power, use a known-good USB data cable, and connect
only this Echo Show during installation. The installer refuses zero or multiple
fastboot devices.

## Inputs

These inputs were validated for CROWN:

- Amonet: `/home/baka/Documents/sidequests/amonet-crown-v2.0.1`
- Lineage vendor ZIP: `/home/baka/Documents/lineage-18.1-20260904-UNOFFICIAL-crown.zip`
- Pinned CROWN boot image: `build/release-techo5-v0.8.0/techo5-boot-crown-v0.8.0.img`
- Audited rootfs: `build/jarvis-e2e-0f78d1d/artifacts/jarvis-show-rootfs-v0.0.0-dev.32/jarvis-show-rootfs-v0.0.0-dev.32.tar.gz`

Run all commands from the repository root:

```sh
cd /home/baka/Documents/techo5JarvisAssistant-build/src
```

## Preflight and identity

Preflight only validates the host and files; it does not contact or modify the
device:

```sh
python3 tools/jarvis-show.py preflight --board crown \
  --amonet-dir /home/baka/Documents/sidequests/amonet-crown-v2.0.1 \
  --lineage-zip /home/baka/Documents/lineage-18.1-20260904-UNOFFICIAL-crown.zip
```

Put the Echo Show into fastboot mode, then run the read-only identity gate:

```sh
python3 tools/jarvis-show.py identify --board crown
```

Continue only when it reports exactly one device and `product=CROWN`. Stop if
the product, serial, or board is unexpected. If a prior install attempt stopped
while the same device is already in verified TWRP recovery, `install --board crown`
can resume from that recovery session; it verifies one root TWRP device reports
`crown` and does not require a fastboot reboot first.

## Install

The following invokes the supported installer with the audited rootfs hash:

```sh
python3 tools/jarvis-show.py install --board crown \
  --name livingRoomEcho8 \
  --amonet-dir /home/baka/Documents/sidequests/amonet-crown-v2.0.1 \
  --lineage-zip /home/baka/Documents/lineage-18.1-20260904-UNOFFICIAL-crown.zip \
  --boot-image "$PWD/build/release-techo5-v0.8.0/techo5-boot-crown-v0.8.0.img" \
  --rootfs "$PWD/build/jarvis-e2e-0f78d1d/artifacts/jarvis-show-rootfs-v0.0.0-dev.32/jarvis-show-rootfs-v0.0.0-dev.32.tar.gz" \
  --rootfs-sha256 eb8c9aaa8d03bf08c47747f9f156cf64970ffeeebecb666049cf3f0a50af48bf
```

The installer first repeats its host and CROWN identity checks. It preserves
the detected fastboot serial across reconnects, enters/reuses recovery as
needed, and creates a verified off-device recovery backup before destructive
work. A locked device separately requires the exact `UNLOCK CROWN` phrase.
The final destructive stage requires the exact phrase shown by the installer.

Do not disconnect power or USB while the installer is running. If it stops,
do not retry blindly: rerun the `identify` command first and use the designed
recovery/rollback path.

## After first boot

Check each layer before changing another one: boot and active slot, network,
Home Assistant connection, display/touch, audio, and microphone/mute. Then
perform the installed-device acceptance work: CROWN's 1280x800 display checks
(J38), HA/Klar routing (J39), music, privacy, reboot durability, OTA, and
rollback. See [final-acceptance.md](final-acceptance.md).

CHECKERS remains fail-closed for destructive installation. Do not use this
runbook for CHECKERS or CRONOS.
