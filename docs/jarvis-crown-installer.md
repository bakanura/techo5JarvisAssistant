# Jarvis Crown v1 installer

The Jarvis Crown installer is deliberately split into fail-closed gates.  J17 implements only the
host/input preflight.  It **never invokes adb or fastboot**, so it cannot query, reboot or modify an
Echo Show.

```sh
python3 tools/jarvis-crown.py preflight \
  --amonet-dir /path/to/amonet-crown-v2.0.1 \
  --lineage-zip /path/to/lineage-18.1-...-UNOFFICIAL-crown.zip
```

The preflight requires:

- Linux and Python 3.10+;
- `adb`, `fastboot`, `git`, `bash`, and GNU `timeout` in `PATH`;
- USB serial access through `dialout`/`uucp` for an unprivileged install;
- ModemManager not running;
- writable work and backup directories;
- at least 8 GiB free on every distinct filesystem used for work/backups;
- a complete local Amonet package whose `amonet/device.prop` says `DEVICE=crown`;
- when supplied, a LineageOS ZIP whose metadata says `pre-device=crown`.

The LineageOS zip and the board boot image are fetched rather than hunted for by hand:

```sh
python3 tools/fetch-show-assets.py --board checkers     # or crown
```

It downloads both from GitHub (amazon-oss/releases for LineageOS, the upstream TECHO5 release for the
boot image) into `~/.cache/jarvis-show/<board>/` (override with `--cache` or `JARVIS_SHOW_ASSETS`),
keeps a file only when its size and SHA-256 match the pin in `tools/jarvis_crown/assets.py`, and prints
the paths. It refuses a folder inside the repository: these files are hundreds of MB and never belong in
the checkout. `--check` verifies what is cached without downloading.

Amonet zips are only attached to the XDA threads, and XDA's CDN answers anything that isn't a real
browser with a 403 challenge page, so a script can't download them. The tool opens the attachment link
in your default browser instead (log in to XDA if it asks) and waits up to ten minutes for the finished
zip to show up in `~/Downloads` (or `--from DIR`). Then it checks the zip against its pin and copies it
into the cache and unpacks its `amonet/` folder to `~/.cache/jarvis-show/<board>/amonet-<board>-v2.0.1/`,
which `jarvis-show.py` uses when `--amonet-dir` isn't given and there's no bundle in `../third_party/`.
With `--no-browser`, or when no browser can be opened, it prints the link and you run it
again after downloading. A package with no pin yet is not used. The tool
compares every file in it with the public source (R0rt1z2/amonet) and sorts them into:

- files identical to the current upstream branch;
- files identical to an older upstream commit (the report names branch and commit);
- files that differ from every upstream copy it checked;
- binaries the installer already pins elsewhere (unlock payloads, the board's TWRP);
- binaries only the zip carries (preloader, LK, TZ, kaeru, and so on).

Then it stops. The hash goes into `tools/jarvis_crown/assets.py` only after someone has read that
list. The crown and checkers 2.0.1 zips went through this on 2026-10-09 and are pinned.

### Amonet 1.x units

A Show 5 first unlocked with Amonet 1.x has `microloader by xyz` at the start of its boot partition, and
the TWRP gate refuses it. The upgrade the XDA thread gives for that is flashing the 2.x zip in TWRP, and
the installer has a separate command for it:

```sh
adb reboot recovery
python3 tools/jarvis-show.py amonet-upgrade --board checkers
```

It takes the pinned Amonet zip and the pinned LineageOS zip from the cache (`--amonet-zip` and
`--lineage-zip` override that) and refuses either one if it doesn't match its pin. With exactly one Show
in TWRP, it goes through these steps:

1. Checks `ro.product.device` and root, then reads the boot partition. If there's no microloader there,
   it stops with PASS and changes nothing.
2. Resolves every partition the zip's update-binary will write, the same way the update-binary does,
   and stops unless each one is the expected block on this board. On checkers that's `lk_real` → p3,
   `tee1_real` → p4, `tee2_real` → p6, `expdb` → p7, `MISC` → p8, `recovery` → p10, `swdl` → p11, plus
   `mmcblk0boot0` for the preloader. It also checks that p9 is named `boot`, and that every image fits
   its partition (the update-binary's `dd` doesn't check that).
3. Backs up the same partition set as J20 to `backups/<adb serial>/before-amonet2/partitions` and
   verifies it.
4. Asks for `UPGRADE AMONET CHECKERS`. Anything else stops here with nothing written.
5. Pushes the zip, checks its SHA-256 on the device and runs `twrp install`. The update-binary writes
   the preloader, LK, both TEE images, kaeru, TWRP to recovery and swdl, wipes misc and reboots to
   recovery. If TWRP comes back with the same boot id, the update-binary stopped early and the command
   says so.
6. Reads back every written partition and compares it with the bytes in the zip.
7. Writes the `boot.img` from the LineageOS zip straight to p9. The Lineage updater-script writes to
   `by-name/boot`, which doesn't exist while the 1.x renames are in place. The command then checks
   that p9 holds those bytes and that the microloader is gone.

If anything fails after the backup, the error names the backup folder. After a PASS, the normal
`install` flow runs as usual.

The Amonet package stays a local/user-supplied dependency rather than a Jarvis Crown release asset.
Later installer stages must not weaken or bypass this gate.

J18 adds an independent live-device CROWN identity gate. Passing J17 therefore never means a connected
device is accepted for flashing.

## J18 — live Crown identity gate

`identify` runs the J17 host/input preflight first and only then performs read-only fastboot queries:

```sh
python3 tools/jarvis-crown.py identify \
  --amonet-dir /path/to/amonet-crown-v2.0.1 \
  --lineage-zip /path/to/lineage-18.1-...-UNOFFICIAL-crown.zip
```

The live gate requires:

- exactly one device in `fastboot devices`;
- `fastboot -s <serial> getvar product` to return `CROWN`;
- `fastboot -s <serial> getvar unlock_status` to return exactly `true` or `false`;
- all fastboot queries to complete within the installer timeout.

`lk_build_desc` is collected only as diagnostic/payload-selection metadata. Missing LK description is
not enough to reject an otherwise proven Crown because the supplied Amonet Crown bundle has a default
Crown payload.

The J18 implementation cannot issue `flash`, `erase`, `boot`, `reboot`, `oem`, or `flashing`
commands. Wrong product, zero/multiple devices, malformed identity data, command failures, or timeouts
all stop the installer before J19 can invoke Amonet.

## J19 — pinned Amonet Crown unlock wrapper

`unlock` first executes J17 and J18. If the proven Crown already reports
`unlock_status=true`, Amonet is skipped entirely. A locked Crown requires the exact interactive phrase
`UNLOCK CROWN` before any exploit is executed.

Before execution, Jarvis Crown SHA-256 verifies the exact known-good archived bytes for:

- `fastbrick.sh`;
- `profile.sh`;
- `device.prop`;
- `bin/fastbrick.img`;
- `bin/fastboot` and `bin/fastboot32`.

A hash mismatch is fatal. The wrapper runs only the pinned `fastbrick.sh`; it does not duplicate or
rewrite the exploit. When Amonet exits, Jarvis Crown treats the state as unproven until the read-only
J18 gate sees the same fastboot serial, `product=CROWN`, and `unlock_status=true`. A timeout, Amonet
error, serial change, or persistent locked state stops the workflow and requires re-identification
before any retry.

J19 does not flash TWRP, write TECHO5 partitions, or start the rootfs installer. Those operations are
separate later gates.

## J20 — TWRP handoff and recovery backup gate

Jarvis Crown reuses an already-running Crown TWRP whenever possible. This matters because a successful
Amonet fastbrick run normally lands in TWRP already, and reflashing recovery for no reason only adds
risk.

If an already-unlocked Crown is still in fastboot, J20 follows the archived Amonet Crown recovery
sequence exactly, using the SHA-256 pinned `twrp.img`:

1. `fastboot -s <serial> flash recovery twrp.img`
2. `fastboot -s <serial> flash swdl twrp.img`
3. `fastboot -s <serial> reboot recovery`

`fastboot boot` is intentionally not used: TECHO5 documents that Amonet's Crown LK does not implement
it. Before these writes, the live fastboot identity must still be the same unlocked `CROWN` serial.
J20 never writes `lk`, `preloader`, `expdb`, `tee*`, `boot`, `system`, or userdata.

Once TWRP is present, Jarvis Crown requires `ro.product.device=crown`, root ADB, and the expected Crown
boot block before backup. It then saves the same small-partition set TECHO5 protects before install
(p1-p11 except none skipped there, plus p14/p15) and both eMMC boot areas (`boot0`, `boot1`). Every file
must match:

- the block size reported by the device;
- a device-side SHA-256 calculated from the block device;
- a host-side SHA-256 calculated from the received file.

The backup is written to `partitions.partial/`, receives `SHA256SUMS` plus a manifest, is fully
re-verified, and is only then atomically renamed to `partitions/`. A failed/incomplete backup is removed
and can never be mistaken for a reusable completed backup. A pre-existing completed backup is reused
only after all hashes and partition coverage verify again.

This is a **post-Amonet recovery backup**, not a pristine factory/pre-unlock dump. Amonet may already
have replaced recovery/swdl with TWRP by this point. Its purpose is to preserve the known unlocked Crown
state before Jarvis Crown/TECHO5 later formats userdata or converts `system` into the A/B rootfs store.

## J21 — Crown Lineage vendor-driver staging

The LineageOS ZIP remains a vendor-driver source only; Jarvis Crown never requires a first Android boot.
Before any destructive action, the installer verifies the ZIP metadata names `crown` and the live TWRP
unit is the same board.

The known Crown recovery handoff bug is fixed in the fork: after `twrp format data`, the installer uses
TWRP's own `twrp reboot recovery` command rather than generic `adb reboot recovery`. It then waits for
all three conditions before pushing the ZIP:

- ADB reports recovery state;
- `/data` is mounted;
- `/data` passes a real create/remove write probe.

The ZIP is SHA-256 checked after transfer before TWRP installs it. After installation, `system` is
mounted read-only and both MT7668 vendor modules must report the exact kernel ABI Jarvis Crown uses:

- `mt76x8_wlan.ko` -> `4.9.337-g8d928c5176cc`
- `mt76x8_bt.ko` -> `4.9.337-g8d928c5176cc`

If either module is absent or has a different vermagic, the workflow stops in TWRP before the later
slot-store conversion. LineageOS itself is never booted by this path.

## J22 — Jarvis Crown rootfs install/provision wrapper

J22 deliberately reuses TECHO5's proven rescue/slot-store implementation instead of cloning the
partition conversion code.  Jarvis Crown adds a fail-closed wrapper around the only destructive part
of the install.

The wrapper will not construct an install plan until all of these are true:

- J20's atomic `partitions/` backup passes its SHA-256/coverage verification again;
- the backup manifest belongs to the same ADB recovery serial now being installed;
- the boot image is byte-for-byte the known-good Crown v0.8.0 image
  (`cf5a492f7ee7ec16305905c58bf7f0b2e3f3e75521668ca905b9ae1ffb6d0baa`);
- the rootfs matches the exact SHA-256 supplied by the future signed Jarvis Crown release metadata;
- the rootfs contains a regular `etc/jarvis-crown-release.json` marker naming
  `product=jarvis-crown-v1`, `board=crown`, and a non-empty version;
- the operator types exactly `ERASE LINEAGE INSTALL JARVIS CROWN`.

The archived upstream v0.9.21 rootfs intentionally fails the Jarvis product-marker gate.  J22 therefore
cannot accidentally install an old generic TECHO5 userspace while presenting it as Jarvis Crown.  The
future release/build job will create the marked rootfs; J22 does not build one.

J21 has already installed and checked LineageOS's Crown vendor tree.  The internal
`--jarvis-crown-prestaged` path therefore **does not format userdata or install Lineage again**.  It
mounts/checks the vendor modules once more and then enters the same upstream flow that:

1. copies the wrapper-verified rootfs to userdata and verifies the transfer;
2. saves the current Lineage boot image as the USB recovery path;
3. flashes only the pinned Crown TECHO5/Jarvis boot image to the `boot` partition;
4. boots the rescue environment;
5. refuses to continue if a slot store already exists;
6. saves the unit-specific Lineage vendor tree before `system` is erased;
7. converts only `mmcblk0p12` (`system`) into the A/B slot store;
8. installs rootfs slot A and copies the vendor tree into that slot;
9. provisions the device name, encrypted Home Assistant API key, optional SSH public key and optional
   Wi-Fi credentials;
10. writes `/data/misc/techo5/profile` as `jarvis-crown-v1` and boots slot A.

Jarvis Crown forces `--amazon-logo` on this internal handoff.  That is intentionally named after the
upstream option: it means **do not patch the boot logo**, which keeps `expdb`/kaeru unchanged.  The J22
wrapper never requests a Lineage ZIP, never writes `lk`, preloader, `expdb`, `tee1`, `tee2`, persist,
metadata, recovery, swdl or either eMMC boot area.  J20's earlier TWRP handoff is the only recovery/swdl
write in the one-installer flow.

The pre-staged path also skips the upstream GitHub release lookup completely.  Boot and rootfs are
local wrapper-verified inputs, so the destructive stage cannot substitute an asset fetched from the
network.

Fresh Jarvis behavior (Hey Jarvis, follow-up turns, stop threshold, streamed `/jarvis-display`, and the
other product defaults) is compiled into the Jarvis Crown rootfs.  J22 intentionally does not create a
large `state.json` that would freeze today's defaults forever; it writes only persistent identity/
credentials and the profile marker.  Future OTA updates therefore retain user settings while still
allowing new defaults to apply correctly to genuinely fresh devices.
