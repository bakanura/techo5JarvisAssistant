# Jarvis Show OTA channels

Jarvis Show has two compiled-in release streams under the same signed release trust root:

- `stable`: `https://github.com/bakanura/techo5JarvisAssistant/releases/latest/download/manifest.json`
- `dev`: `https://github.com/bakanura/techo5JarvisAssistant/releases/download/dev/manifest.json`

Stable follows GitHub's latest non-prerelease release. Dev follows the rolling `dev` release tag.
Changing the Home Assistant update-channel select stores only `stable` or `dev`; it cannot configure
an arbitrary URL.

A manifest cache is channel-bound. Changing channel clears the previous offer immediately. If a fetch
fails during installation, only a cache produced by the currently selected channel may be reused. If
the selection changes while a network request is in flight, that result is discarded even when its
signature is valid.

Version ordering is fail-closed. A release older than the running version is never installed. A
running build whose version cannot be ranked is also barred from network OTA rather than guessing
whether a signed release is newer. Proper release/dev builds must use a rankable `vX.Y.Z` style stamp.

Both channels use the same Ed25519 verification, size/hash checks, inactive-slot install, trial boot,
health commit and automatic rollback contract.
