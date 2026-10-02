# Jarvis Show v1 security threat model

Jarvis Show runs on an IoT LAN and must assume that another device on that LAN can be compromised.
The security boundary is therefore the device/service credential, not merely "same subnet".

## Trust zones

1. **Device-local / physical-presence** — screen, hardware buttons, local storage and the setup-session gesture.
2. **Authenticated Home Assistant control plane** — ESPHome/native API on TCP 6053 protected by the provisioned device encryption key.
3. **Authenticated peer plane** — signed house announcements, encrypted/authenticated intercom, SIP over TLS/SRTP, and PSK-authenticated Dashcast.
4. **Restricted media plane** — Sendspin must be limited to the configured Music Assistant server; LAN membership alone is not authorization.
5. **Untrusted IoT LAN** — arbitrary peers must not gain camera/screen reads, root SSH, media injection, configuration writes or update authority.
6. **Internet** — outbound update/model/web/LLM requests only; no router port forwards are part of the supported design.

## Security invariants

- No unauthenticated LAN peer may obtain camera frames or screenshots.
- No unauthenticated LAN peer may inject audio into Sendspin.
- Home Assistant actions carrying credentials/private data require the encrypted device API.
- SSH is off by default, key-only, and may start only with a real API encryption key and at least one valid authorized key.
- Setup writes require physical-presence session authorization.
- House announcements are HMAC signed and replay protected; plaintext legacy house credentials are rejected.
- Intercom/Drop In uses authenticated peer identity; Drop In is opt-in and has audible + visible privacy cues.
- SIP never downgrades below TLS signalling + SRTP media.
- TLS certificate verification cannot be persistently disabled in the Jarvis Show production image.
- Production Linux exposes no ADB daemon over TCP 5555.
- Update/install authority is the Jarvis Show Ed25519 release key plus product/board/hash/version checks and A/B rollback.
- Debug/management listeners are disabled unless explicitly enabled and must still authenticate/restrict the caller.
- Ordinary OTA is rootfs A/B only; boot/kernel/partition writes are never implied by an OTA.

## Exposed-surface inventory

| Surface | Default | Authentication / restriction | Risk / J32 action |
|---|---|---|---|
| ESPHome/native API TCP 6053 | on | provisioned encryption key | Keep; private actions fail closed without encryption. |
| Dashcast TCP 9555 (server-side) | external service | device PSK / authenticated protocol | Keep LAN-only; never port-forward. |
| Device web TCP 8181 | closed unless feature opens it | setup writes use physical-presence session; camera/screen reads were unauthenticated | **High:** require session for camera/screen reads. |
| Sendspin TCP 8928 | upstream default on | upstream accepted any LAN client | **Critical:** restrict to explicitly configured Music Assistant server and no listener without that pairing. |
| SSH TCP 22 | off | key-only; requires encrypted HA link + authorized key | Keep opt-in; audit binding/logging. |
| Remote ADB TCP 5555 | Android diagnostic only | Android firewall toggle | Production Linux has no adbd; ensure saved setting cannot revive an exposure in Jarvis Show. |
| SIP | opt-in | TLS certificate validation + SRTP | Keep secure-only; credentials stay private. |
| House announcements/intercom | opt-in/peer discovery | HMAC/replay protection and encrypted peer protocol | Keep; no plaintext fallback. |
| Camera ESPHome entity | HA-controlled | encrypted API | Preferred camera path. |
| Bluetooth | opt-in/configured | Bluetooth protocol/pairing | Treat pairing as physical/user action; no hidden discoverability. |
| OTA | scheduled/manual | signed manifest + hashes + version/channel/product checks | J34 audits supply chain. |
| Direct Brain / SearXNG / LLM / STT / TTS | outbound | endpoint-specific configuration | Never disable TLS verification; secrets handled in J33. |

## Privilege observation

`echod` currently runs as root because it directly owns hardware, firewall, audio, framebuffer and several device nodes. This raises the impact of a network parser bug. J32 therefore minimizes listeners and caller reachability; J33 separately reviews whether helpers/components can be privilege-separated without breaking Crown/Checkers hardware access.
