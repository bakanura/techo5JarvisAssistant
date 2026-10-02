# Jarvis Show OTA identity

Official GitHub repository: `https://github.com/bakanura/techo5JarvisAssistant`

Official release base: `https://github.com/bakanura/techo5JarvisAssistant/releases`

Channels:

- stable: `https://github.com/bakanura/techo5JarvisAssistant/releases/latest/download/manifest.json`
- dev: `https://github.com/bakanura/techo5JarvisAssistant/releases/download/dev/manifest.json`

Both manifests require a detached Ed25519 signature at `manifest.json.sig`. The updater and installer
trust only the Jarvis Show v1 release key below:

```text
MIKd5Pm5qmv1aSWDiO8isqqJXoZzyk/cCH4VEIyzg2I=
```

The private 32-byte Ed25519 seed is generated outside the Git repository and must never be committed,
printed in CI logs, included in an artifact, or copied into a rootfs. In this development checkpoint it
is stored outside `src/` under the sandbox project's private directory with mode 0600. Production CI
should store the same seed as a protected repository secret.

The device release base is compiled in. A development/mirror build may override the Go string variable
`github.com/HuskerMinion/techo5/echod/internal/update.releases` with `-ldflags -X`; there is deliberately
no Home Assistant or Setup-page text field that can redirect a root updater at runtime. The host
installer can use `JARVIS_SHOW_RELEASE_REPO` for a mirror, but every manifest and asset still has to be
covered by this Ed25519 trust root.

Changing the repository URL does not rotate trust. Rotating this key is a separate migration and must
be handled as an explicit signed transition/recovery procedure.
