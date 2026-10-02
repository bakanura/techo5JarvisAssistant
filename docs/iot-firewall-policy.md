# Jarvis Show v1 — IoT firewall policy

Jarvis Show is designed for a **default-deny** IoT VLAN. A Crown/Checkers device should never need a
blanket `IoT -> LAN` allow rule, and none of its listeners should be exposed to the public internet.

This document describes the deployment boundary expected by the J31–J35 security work. Substitute the
actual Music Assistant, Home Assistant, Dashcast, DNS/NTP and administration addresses used by the
installation. Do not broaden a host-specific rule to an entire trusted subnet merely for convenience.

## Baseline flows

| Direction | Protocol / port | Destination | Required | Purpose / rule |
|---|---|---|---|---|
| HA -> Jarvis Show | TCP 6053 | individual Show | yes | ESPHome/native API. Permit only the Home Assistant host. The provisioned encryption key is still mandatory. |
| Music Assistant -> Jarvis Show | TCP 8928 | individual Show | when Music Assistant is used | Sendspin. Permit only the paired Music Assistant server. `echod` independently rejects every other source IP. |
| Jarvis Show -> Dashcast | TCP 9555 | Dashcast host | when streamed dashboard is used | Authenticated/encrypted Dashcast transport. Never port-forward 9555. |
| Jarvis Show -> HA | TCP 8123 or configured HA port | Home Assistant host | yes for HA REST/media/helper paths | Permit only the configured HA endpoint. Prefer HTTPS when HA is exposed that way internally. |
| Jarvis Show -> router | UDP/TCP 53 | IoT gateway/DNS resolver | yes | DNS. Do not allow arbitrary external DNS when local policy intends to enforce the resolver. |
| Jarvis Show -> router | UDP 123 | IoT gateway/NTP | yes | NTP. The existing OpenWrt NTP redirect may enforce this for devices that try another NTP target. |
| Jarvis Show <-> router | UDP 67/68 | IoT gateway | yes | DHCP. |

## Same-IoT-subnet peer traffic

Native house announcements and intercom use TCP **8181** between Jarvis Show peers. They are separately
authenticated (signed announcements, encrypted/authenticated intercom), but if both devices are on the
same IoT VLAN this traffic normally stays at layer 2 and never reaches the OpenWrt forwarding firewall.

**Do not enable Wi-Fi client isolation / AP isolation for the Jarvis Show SSID unless peer intercom and
announcements are intentionally being disabled or moved through a routed design.** Client isolation would
break same-IoT-subnet Crown/Checkers communication even though the router rules are correct.

Cross-VLAN TCP 8181 is not required for normal operation. The setup/camera/screen diagnostic pages on
8181 are physically gated and should be reachable only from a trusted administration host when needed.

## Optional flows

| Direction | Protocol / port | Destination | Default | Notes |
|---|---|---|---|---|
| admin host -> Jarvis Show | TCP 22 | individual Show | deny | SSH is opt-in and key-only. If enabled, permit one administration host, never the whole LAN/Internet. |
| admin host -> Jarvis Show | TCP 8181 | individual Show | deny | Temporary setup/camera/screen diagnostics. The device also requires its physical-presence setup session. |
| Jarvis Show -> SIP provider | TCP 5061 | configured SIP provider | deny unless configured | SIP signaling is TLS-only. Allow established/related return traffic. |
| Jarvis Show <-> SIP provider | UDP provider-negotiated SRTP | configured SIP provider | deny unless configured | Media is SRTP-only. Prefer provider/IP-specific rules where practical; do not expose a static inbound RTP range from the Internet. |
| Jarvis Show -> Ollama / SearXNG / Wyoming | configured TCP ports | explicitly configured Direct Brain hosts | deny unless Direct Brain is configured | Permit only the configured server IPs and ports. Do not grant general IoT -> server-LAN access. |
| Jarvis Show -> Internet | TCP 443 | GitHub release endpoints / explicitly required services | policy-dependent | OTA manifests/assets are signed and hashed, but egress should still be narrowed where the firewall/DNS stack can maintain reliable endpoint policy. |

Bluetooth pairing is not a routed firewall flow. Jarvis Show keeps pairing in its explicit, temporary
pairing window; it should not be treated as a permanently discoverable management path.

## Explicit denies

At minimum, enforce these concepts after the allows above:

1. **IoT -> trusted LAN: deny** by default.
2. **trusted LAN -> IoT: deny** by default except the explicit HA, Music Assistant and optional admin
   initiators above.
3. **WAN -> Jarvis Show: deny all.** No port forwards for 22, 6053, 8181, 8928 or Dashcast 9555.
4. **Jarvis Show -> arbitrary management services: deny.** There is no runtime need for SMB, database,
   Proxmox, Docker API or other homelab administration ports.
5. Do not create a generic `IoT -> 192.168.8.0/24 ACCEPT` rule as a shortcut for Direct Brain.

## OpenWrt rule model

The canonical OpenWrt deployment should express host-specific rules approximately like this (names are
illustrative; use the real zone names and server addresses):

```text
HA host             -> Jarvis Shows       tcp/6053        ACCEPT
Music Assistant     -> Jarvis Shows       tcp/8928        ACCEPT
Admin workstation   -> Jarvis Shows       tcp/22,8181     OPTIONAL ACCEPT
Jarvis Shows        -> Home Assistant     tcp/8123        ACCEPT
Jarvis Shows        -> Dashcast           tcp/9555        ACCEPT
Jarvis Shows        -> router             dns,ntp,dhcp    ACCEPT
Jarvis Shows        -> configured brain   exact ports     OPTIONAL ACCEPT
Jarvis Shows        -> SIP provider       tcp/5061 + EST  OPTIONAL ACCEPT
IoT zone            -> trusted LAN        any             REJECT
WAN                  -> IoT zone          any             REJECT
```

For installations like the current homelab where the Shows are on `192.168.9.0/24` and service hosts are
on `192.168.8.0/24`, write destination-IP-specific exceptions **above** the general IoT-to-LAN reject.
The precise Music Assistant and optional SIP/Direct-Brain endpoints must be filled from the deployment;
do not guess them in firmware or broaden the rule because they are unknown.

## Secure-default acceptance checklist

Before calling a deployment secure:

- [ ] Show devices live on the IoT VLAN/SSID, not the trusted administration LAN.
- [ ] IoT -> trusted LAN is default-deny.
- [ ] No Jarvis/TECHO5 listener is port-forwarded from WAN.
- [ ] HA is the only normal source allowed to device TCP 6053.
- [ ] Music Assistant is the only source allowed to TCP 8928 when Sendspin is enabled.
- [ ] TCP 8181 is not broadly routed; physical-presence setup remains required for private pages.
- [ ] SSH is disabled, or limited to one administration host and key-only authentication.
- [ ] Direct Brain rules name exact hosts/ports rather than the entire server subnet.
- [ ] SIP is disabled unless configured; when configured, signaling is TLS and media is SRTP.
- [ ] Client isolation is **off** when native peer intercom/announcements are expected on the same IoT subnet.
- [ ] `tools/security-regression.sh` passes with the release Go toolchain.
