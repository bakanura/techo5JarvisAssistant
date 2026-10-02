# Jarvis Show secret storage and privilege model

## Secret-bearing persistent state

Jarvis Show treats the following as owner-only material:

- `/data/misc/techo5/state.json` — house HMAC secret, Direct Brain key, private iCal URLs, camera/NVR credentials and other household settings.
- `/data/misc/techo5/psk` — ESPHome/native API encryption key.
- `/data/misc/techo5/hass.json` — Home Assistant REST URL/token.
- `/data/misc/techo5/phone.json` — SIP credentials.
- `/data/misc/techo5/phone-contacts.json` — private address book.
- `/data/misc/techo5/ssh/*` and root's managed `authorized_keys` — SSH identities.
- `/data/techo5-linux/wpa_supplicant.conf` — derived Wi-Fi PSKs.
- `/data/techo5-linux/techo5.log` and rotated log — logs that can contain private household context.

Fresh writes use mode `0600`. Loading an older state/API/HA/SIP/contact/Wi-Fi file repairs its mode to `0600` before it is used, and the containing credential directories converge to `0700`. The boot path also repairs persistent daemon/boot logs and kernel crash records from older installations. Persistent runtime directories are `0700`, launchers use `umask 077`, and process core dumps are disabled.

## Logs and diagnostics

The daemon does not log API tokens, the house word, Direct Brain key, SIP password or Wi-Fi passphrases. Bluetooth pairing no longer logs the peer address/object path or numeric passkey. SIP library text is never copied verbatim into the persistent log, because provider warnings can contain authentication headers or caller/account data. Signed Home Assistant media/TTS URLs and private iCal URLs are likewise removed at the logging/error source instead of relying on a later redaction pass.

The diagnostics bundle runs all output through the redactor and explicitly registers the current house secret, Direct Brain key, Reolink password, private calendar URLs, Dashcast key, ESPHome device PSK and Home Assistant token as known secrets in addition to generic token/password/URL/IP/MAC/phone patterns.

Device recovery backups are not source artifacts. Any locally retained backup carrying pairing/API material is kept outside the Git checkout under the project-private directory with owner-only permissions, and is excluded from source checkpoint archives and releases.

## Release signing key

The Jarvis Show Ed25519 **public** verification key is part of the image. The matching private release seed is never installed on a Show and never belongs in Git; it is held outside the repository / in the release CI secret as documented by the release workflow.

## Privilege boundary

`echod` remains root in v1. It directly owns framebuffer, ALSA/audio, microphone/camera nodes, Bluetooth/radio integration, network/firewall transitions, Wi-Fi management, SSH lifecycle and update/slot operations. Splitting those safely would require a privileged broker with a new authenticated IPC/security boundary. A partial UID drop while retaining broad capabilities/device ownership would add complexity without creating a trustworthy boundary, so v1 does not pretend otherwise.

The v1 mitigation is therefore:

1. minimize/authenticate network listeners (J32),
2. owner-only state/logs and no core dumps (J33),
3. signed and product-bound updates (J34), and
4. IoT segmentation / regression tests (J35).

Privilege separation remains a future architectural hardening item rather than a release-blocking half-measure.
