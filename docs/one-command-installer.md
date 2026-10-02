# One-command Jarvis Show installer contract

The production entry point is `tools/jarvis-show.py install --board crown|checkers ...`.

Its ordered safety gates are:

1. Host/input preflight; no device I/O.
2. Read-only fastboot identity for the requested board.
3. Amonet unlock only if the proven device is locked, with board-specific exact confirmation and pinned payload hashes.
4. TWRP handoff; already-running matching TWRP is reused, otherwise only pinned `recovery` + `swdl` images may be flashed.
5. Complete board-bound J20 recovery backup with device/host hashes.
6. Validate the final boot image, Jarvis Show rootfs marker/hash and backup **before userdata is erased**.
7. Exact product-specific erase confirmation.
8. Hidden Lineage-vendor-only stage: format userdata, use `twrp reboot recovery`, install matching Lineage ZIP, verify Wi-Fi + Bluetooth module ABI, then return before release/boot/store code.
9. Pre-staged final TECHO5 slot-store handoff: no release download, no second Lineage install, preserve `expdb`, flash only the pinned board boot image and perform the documented `system` -> A/B store conversion.
10. First-boot provisioning and upstream health/trial-slot behavior.

A failure at any step prevents later steps from running. In particular, a bad rootfs/boot/backup is discovered before the first userdata format, and a Lineage staging failure cannot fall through into the slot-store conversion.

## Checkers release gate

The Checkers code path is covered by synthetic regression tests, but a real Checkers flash remains disabled until trusted local bytes are supplied and pinned for:

- Amonet Checkers unlock assets
- Checkers TWRP
- Checkers boot image

Synthetic hashes in tests are test data only and never become production trust anchors.
