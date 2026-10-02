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
