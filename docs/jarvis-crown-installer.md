# Jarvis Crown v1 installer

The Jarvis Crown installer is deliberately built in fail-closed stages.  At J17 only the host-only
preflight exists.  It **does not invoke adb or fastboot at all**, so running it cannot change an Echo.

```sh
python3 tools/jarvis_crown.py preflight \
  --amonet /path/to/amonet-crown-v2.0.1/amonet \
  --lineage-zip /path/to/lineage-18.1-...-UNOFFICIAL-crown.zip
```

The preflight requires:

- a Linux host;
- Python 3.10 or newer;
- `adb`, `fastboot`, `git`, `bash`, and GNU `timeout` in `PATH`;
- usable USB-serial permissions (`dialout`/`uucp`, unless running as root);
- ModemManager not running;
- at least 8 GiB free on each distinct filesystem used for installer work/backups, with a writable parent path;
- a complete local Amonet package whose `device.prop` says `DEVICE=crown`;
- when a LineageOS ZIP is supplied, metadata whose `pre-device` is exactly `crown`.

The Amonet directory is deliberately a local input rather than a redistributable Jarvis Crown asset;
see the third-party asset policy.  Later installer stages must not weaken or bypass this preflight.

J18 adds the independent live-device `CROWN` gate.  Passing this host preflight therefore never means
that a connected device is accepted for flashing.
