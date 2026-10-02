# Wi-Fi provisioning and recovery

Jarvis Show must always remain recoverable when it has no usable network. Home Assistant discovery,
mDNS and an existing LAN connection are conveniences, never prerequisites for provisioning.

## Fresh install

The installer may receive Wi-Fi explicitly with `--wifi`, but it does **not** require LineageOS to
have a saved network. A device installed with no network is a supported state.

At boot the Wi-Fi driver and `wpa_supplicant` are brought up even when the configuration contains no
`network={...}` block. `echod` starts without waiting ninety seconds for an address. If the Show still
has no IPv4 address after 45 seconds, it opens the native Wi-Fi picker automatically.

The same picker is always reachable manually at:

`Settings -> Connections -> Wi-Fi`

## Wrong password / moved house

The native Wi-Fi page remains on-screen after a failed join and displays the error. Entering the same
SSID again replaces its old passphrase. When there was a previously-working configuration, a failed
join restores it rather than leaving the device stranded on the attempted network.

When no usable network is saved, the network keeper does not reboot the device in a loop: it keeps
`wpa_supplicant` alive for the local screen to add one.

## USB recovery

If the screen path is unavailable or the device has moved to a completely different network, use the
same installer front-end over the persistent USB serial console:

```sh
python3 tools/jarvis-show.py wifi --wifi 'Network Name'
```

With more than one Show attached, specify its TECHO5 USB serial:

```sh
python3 tools/jarvis-show.py wifi --wifi 'Network Name' --serial DEVICE_SERIAL
```

The passphrase is prompted locally and converted to WPA's PSK on the computer; only the derived key is
written to `/data/techo5-linux/wpa_supplicant.conf`. `--wifi-passphrase-file` is available for scripted
provisioning. The tool restarts the unit, waits for USB serial to return, and verifies that an IPv4
address was obtained. A failed association reports the likely wrong-passphrase state and can simply be
run again.

This path requires neither Home Assistant nor zeroconf nor SSH.

## Recovery invariants

- A missing Wi-Fi configuration never prevents `echod` from starting and painting the local picker.
- No-network first boot is accepted by the installer.
- The Settings Wi-Fi picker remains available after HA is lost.
- USB serial provisioning remains available while the rootfs is otherwise healthy.
- Credentials remain owner-only (`0600` file under an owner-only runtime directory).
- Wi-Fi failure is not a reason to touch boot/kernel/slot metadata.
