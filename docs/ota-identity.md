# Jarvis Show OTA identity

Official GitHub repository: `https://github.com/bakanura/techo5JarvisAssistant`

Official release base: `https://github.com/bakanura/techo5JarvisAssistant/releases`

Channels:

- stable: `https://github.com/bakanura/techo5JarvisAssistant/releases/latest/download/manifest.json`
- staging: `https://github.com/bakanura/techo5JarvisAssistant/releases/download/channel-staging/manifest.json`
- dev: `https://github.com/bakanura/techo5JarvisAssistant/releases/download/channel-dev/manifest.json`

Both manifests require a detached Ed25519 signature at `manifest.json.sig`. The updater and installer
trust only the Jarvis Show v1 release key below:

```text
hGZ6WbvLSUjOM09c79iktfAl5StTTFH3Dtps2iyy1nY=
```

The private 32-byte Ed25519 seed (base64) is generated outside the Git repository and must never be
committed, printed in CI logs, included in an artifact, or copied into a rootfs. It lives in two places:

- the maintainer's keyring, entry "Jarvis Show release signing key"
  (`secret-tool lookup application jarvis-show secret release-signing-key`)
- the repository Actions secret `JARVIS_SHOW_SIGN_KEY`, which the release workflow signs with

This key replaced the first v1 key in October 2026, after that key's seed was lost. Releases signed with
the old key are no longer trusted by new firmware, and devices still running firmware that trusts the old
key need one install of a build that trusts this key (the installer, not OTA).

The device release base is compiled in. A development/mirror build may override the Go string variable
`github.com/HuskerMinion/techo5/echod/internal/update.releases` with `-ldflags -X`; there is deliberately
no Home Assistant or Setup-page text field that can redirect a root updater at runtime. The host
installer can use `JARVIS_SHOW_RELEASE_REPO` for a mirror, but every manifest and asset still has to be
covered by this Ed25519 trust root.

Changing the repository URL does not rotate trust. Rotating this key is a separate migration and must
be handled as an explicit signed transition/recovery procedure.
