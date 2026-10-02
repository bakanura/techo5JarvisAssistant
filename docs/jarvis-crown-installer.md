# Jarvis Crown v1 installer

The installer is intentionally split into gates.  A later phase is not allowed
to run unless every earlier phase completed successfully.

## Host preflight

`python3 tools/jarvis-crown.py preflight`

The preflight is host-only.  It does **not** enumerate, query, reboot, flash, or
otherwise touch an Echo Show.  It verifies:

- Linux host and Python 3.10 or newer;
- `adb` and `fastboot` availability;
- mandatory USB-serial access (`dialout`/`uucp` for an unprivileged Linux user);
- ModemManager is not active;
- the Jarvis Crown source tree is present;
- the locally supplied Amonet Crown bundle is present;
- work and backup directories are writable;
- at least 8 GiB free in the work filesystem.

Any FAIL stops the installer before device discovery.  Running as root is
permitted for development but reported as WARN; the intended installation path
is an unprivileged user with serial-device access.

Device identification begins only in J18 and must remain fail-closed to CROWN.
