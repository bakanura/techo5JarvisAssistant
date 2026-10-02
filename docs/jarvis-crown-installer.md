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
