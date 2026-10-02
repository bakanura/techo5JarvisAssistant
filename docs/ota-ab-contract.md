# Jarvis Show A/B OTA contract

Jarvis Show keeps TECHO5's slot-store update design unchanged for Crown and Checkers.

- `/dev/mmcblk0p12` is the slot store; ordinary OTA never writes LK, preloader, recovery, boot,
  userdata, persist or eMMC boot areas.
- The running root is never overwritten. `slotctl install` unpacks only into the inactive slot.
- A new slot starts as `trial 3` and becomes active for the next boot.
- Every attempted trial boot consumes one try. After three failed trial boots, the slot becomes `bad`
  and the other `good` slot is selected automatically.
- If no good slot exists, the initramfs remains in the rescue environment rather than selecting an
  untrusted/bad root.
- A trial slot becomes `good` only after `echod` has remained running continuously for 300 seconds.
  A trial that never settles within 900 seconds reboots itself so a try is actually consumed.
- Manual `slotctl rollback` marks the running slot bad and selects the other good slot.

Product/user state is deliberately outside either rootfs slot in `/data/misc/techo5`: Home Assistant
PSK, name, state.json, wake models, timezone, SSH keys, SIP secrets/contacts and other persistent state
survive A/B changes. The downloaded update tarball is only temporary under `/data/techo5-linux` and
is removed after `slotctl install` returns.

`SLOT_FILE` in `slotctl` is test-overridable only; production defaults to `/run/techo5/slot`.
