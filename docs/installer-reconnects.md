# Jarvis Show installer reboot/reconnect contract

The installer expects the Show to disappear from USB and enumerate as a different interface several
times. Those transitions are treated as normal state changes, not immediate failures.

Jarvis Show uses polling deadlines rather than fixed sleeps, so fast machines continue immediately
while slow USB hosts still get enough time:

- post-Amonet hacked-fastboot return: 300 seconds
- recovery/TWRP enumeration: 300 seconds
- TWRP reboot after userdata format: 300 seconds
- adb -> fastboot enumeration: 180 seconds
- rescue USB serial console: 420 seconds
- first Jarvis rootfs boot + daemon: 600 seconds

A timeout stops at that stage and never falls through to a later write. Fastboot presence probes have
their own 10-second command timeout so a wedged host-side USB process cannot make the outer deadline
infinite. Amonet verification tolerates the device being absent while it resets, but still requires the
same serial/product to return with `unlock_status=true`.

These waits do not authorize retrying an unknown destructive state. A failed Amonet/flash stage still
requires the read-only identity/recovery checks before another write-capable stage can run.
