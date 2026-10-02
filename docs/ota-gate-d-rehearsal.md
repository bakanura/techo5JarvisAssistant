# OTA Gate D rehearsal — 2026-10-02

Commit under test before this report: `c845f8d` (`jarvis show: record installer gate b rehearsal`)

Result: **PASS for the locally executable/static Gate D rehearsal at this commit, with the Go updater runtime suite still environment-blocked.**

This is not real-device OTA acceptance. No Jarvis firmware/rootfs artifact was built, published or
installed, and no real Echo was contacted.

## Executable/static result

Focused Python/shell/security/release-contract rehearsal:

- 45/45 tests PASS across `test_slotctl_ab`, `test_release_contract`, `test_supply_chain`,
  `test_security_contract`, and `test_antibrick_contract`.
- The A/B slot implementation was actually exercised against temporary stores, including inactive-slot
  installation, trial state, health commit, failed-trial fallback, manual rollback, and running-slot
  overwrite refusal.
- Persistent product-state location remains `/data`, outside either rootfs slot.
- Supply-chain contracts continue to reject untrusted/tampered Alpine inputs and unsafe signing-key
  handling.
- Ordinary OTA remains constrained away from boot/raw partition writes by the anti-brick contract.

## Gate D requirement mapping

| Gate D requirement | Current rehearsal evidence |
|---|---|
| Stable channel fetches only stable manifest | Go source/runtime regression exists in `echod/internal/update/channel_show_test.go`; runtime execution awaits Go 1.26.x. Release namespace/channel source contract remains statically gated. |
| Dev channel fetches only rolling dev manifest | Same channel regression source as above; runtime execution awaits Go 1.26.x. |
| Same/older/unrankable releases are not installed as downgrades | Go tests exist in `install_test.go` / `manifest_test.go`; security/release contracts remain green, but real Go runtime execution awaits Go 1.26.x. |
| Update installs only into inactive rootfs slot | `test_install_is_inactive_trial_then_commit`, `test_never_overwrites_running_slot` PASS. |
| Persistent `/data` state survives update | `test_install_is_inactive_trial_then_commit`, `test_persistent_product_state_lives_on_userdata_not_a_slot` PASS. |
| Healthy update becomes `good` after settle window | `test_install_is_inactive_trial_then_commit`, `test_boot_health_contract_commits_only_after_settle_window` PASS. |
| Broken trial falls back to previous good slot | `test_failed_trial_consumes_three_boots_then_falls_back` PASS. |
| Manual `slotctl rollback` works | `test_manual_rollback_marks_trial_bad_and_selects_good_peer` PASS. |
| Rescue remains reachable when no slot can boot | Boot/rescue source contract remains part of the existing static/security gate; real-device rescue acceptance belongs to Gate C/D hardware validation. |
| Ordinary OTA never rewrites boot/bootloader-chain partitions | `test_ordinary_ota_cannot_write_boot_or_raw_partitions` PASS. |

## Go runtime blocker

The local toolchain is Go 1.23.2. The repository declares Go 1.26.0 for the root/`echod` modules and
Go 1.26.8 for Dashcast. Running `GOTOOLCHAIN=local go test ./internal/update` therefore fails before
compilation with the expected minimum-version refusal. Allowing automatic toolchain download also fails
because this sandbox cannot resolve/reach `proxy.golang.org`.

The Go updater tests are present and cover channel isolation, signed manifest identity, rootfs identity,
hash/size verification, downgrade refusal, and update behavior. They must be executed under Go 1.26.x
on the final release commit before Gate D can be called fully accepted.

## Residual release gates

- J38 physical Crown + Checkers display acceptance remains open.
- J39 deployed HA/Klar voice/web acceptance remains open.
- Full Go 1.26.x runtime validation remains open.
- Gate D must be rerun on the final release commit and then exercised on real devices from an accepted
  previous release before stable release acceptance.
