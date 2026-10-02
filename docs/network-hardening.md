# Jarvis Show network hardening

Jarvis Show assumes another IoT device on the same LAN can be compromised. Same-subnet membership is
therefore not authorization.

## Secure defaults

- **ESPHome/native API 6053:** normal control plane; provision a real device encryption key.
- **SSH 22:** off by default, key-only, and will not start without both an encrypted HA link and a
  valid authorized key.
- **Sendspin 8928:** off and unpaired by default. Pair one literal Music Assistant server IP with the
  encrypted `sendspin_server` HA action, then enable Sendspin. Connections from every other source IP
  are rejected before WebSocket/session parsing.
- **Web diagnostics 8181:** closed unless Setup, Camera diagnostics, or Screen diagnostics is enabled.
  Camera/screen reads require the same short-lived physical-presence setup session as sensitive setup
  operations. Ordinary HA camera access should use the encrypted ESPHome camera entity.
- **Certificate verification:** always on. Older saved `insecure_tls=true` state is migrated to false;
  Jarvis Show exposes no control that can turn it back on.
- **Remote ADB 5555:** disabled as a Jarvis Show feature on every platform. Older `remote_adb=true`
  state is one-way cleared and there is no entity/action that can turn it back on. USB serial/TWRP is
  the supported recovery/debug path.

## Sendspin pairing

Pairing is deliberately an IP allowlist rather than trusting mDNS discovery. The configured value is a
single IPv4 or IPv6 literal. Changing or clearing it closes the current Sendspin listener/session and
requires the service to settle again. Clearing the address also disables Sendspin.

This protects the parser/audio path even though the upstream Sendspin WebSocket protocol itself has no
application authentication.

## Web diagnostic authorization

The camera and screenshot HTTP endpoints are diagnostic surfaces, not the primary HA integration.
Enabling their switches merely makes the paths exist; a browser must also be authorized by the setup
page's local action-button challenge. The cookie is HttpOnly and SameSite=Strict and is forgotten when
the setup page closes.

## Router policy

J35 will publish the final OpenWrt rules. Until then, do not port-forward any Jarvis Show service and
keep the devices on the IoT network with only explicitly required flows to HA, Music Assistant,
Dashcast, DNS/NTP and configured local backends.

## Reviewed listeners and peer surfaces

- **Dashcast 9555:** device↔Dashcast traffic is Noise-encrypted/authenticated with the configured
  `DASHCAST_KEY`; the server refuses to start without its HA URL, HA token and device key. Keep it
  LAN-only and do not port-forward it.
- **ESPHome/native API 6053:** remains the normal always-on control plane. Installer-provisioned
  devices use a real PSK; deliberate re-adoption can temporarily open the upstream zero-key window
  only from the physically authorized setup page, and it auto-closes with a random key if unused.
- **SIP:** remains opt-in and secure-only (TLS signalling, SRTP media); J13 removed the plaintext
  downgrade path.
- **House announcements/intercom:** remain closed until a house secret exists. Announcements are
  HMAC-signed with nonce/replay protection; intercom is authenticated/encrypted with the derived
  house key; plaintext house-word compatibility is rejected.
- **Bluetooth audio:** BlueZ is forced out of pairing/discoverable mode at start and pairing mode is
  an explicit user action with a three-minute timeout.
- **SSH 22:** remains opt-in and key-only. Starting it requires a real encrypted HA API key and at
  least one valid public key; J33 separately audits permissions, key persistence and root privilege.

## HTTP hardening

The shared device web listener now applies `Cache-Control: no-store`, `X-Content-Type-Options:
nosniff`, `X-Frame-Options: DENY`, and `Referrer-Policy: no-referrer`, limits request headers, and
keeps a bounded header/idle timeout. Streaming camera responses remain possible because there is no
global write timeout.
