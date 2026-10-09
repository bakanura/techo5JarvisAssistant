# Jarvis Show OTA channels

Jarvis Show has three compiled-in release streams under the same signed release trust root:

- `stable`: `https://github.com/bakanura/techo5JarvisAssistant/releases/latest/download/manifest.json`
- `staging`: `https://github.com/bakanura/techo5JarvisAssistant/releases/download/channel-staging/manifest.json`
- `dev`: `https://github.com/bakanura/techo5JarvisAssistant/releases/download/channel-dev/manifest.json`

Stable follows GitHub's latest non-prerelease release. Staging and dev follow the rolling
`channel-staging` and `channel-dev` releases, which hold only the signed manifest of the release they
point at. Changing the update channel (Home Assistant select or the Show's own settings) stores only
`stable`, `staging` or `dev`; it cannot configure an arbitrary URL.

## Where releases come from

Each channel has a branch of the same name. A push to it runs `.github/workflows/release.yml`, which
tests, builds the rootfs, signs and publishes a release on that channel:

| Branch    | Version            | Moves channels         |
|-----------|--------------------|------------------------|
| `dev`     | `vX.Y.Z-dev.N`     | dev                    |
| `staging` | `vX.Y.Z-rc.N`      | staging, dev           |
| `stable`  | `vX.Y.Z`           | stable, staging, dev   |

`X.Y.Z` is the `VERSION` file, the next stable version. `N` is the workflow run number. For one VERSION
that ranks dev < rc < stable, and a release moves the less stable channels too, so dev never offers
something older than what stable already has. Channels only ever move forward.

Usual flow: work lands on `dev`; when it is good, fast-forward `staging` to it; when staging has run fine
on a unit, fast-forward `stable` to staging. After a stable release, bump `VERSION` on `dev` before the
next push. The workflow stops if VERSION is already released as stable, since every build of it would
rank below the stable release and reach no device.

Pushes that only touch `docs/`, `audit/` or Markdown files don't cut a release. A `v*.*.*` tag push or a
manual run of the workflow still works and picks the channel from the version (no suffix: stable,
`-rc.N`: staging, anything else: dev).

## On the device

A manifest cache is channel-bound. Changing channel clears the previous offer immediately. If a fetch
fails during installation, only a cache produced by the currently selected channel may be reused. If
the selection changes while a network request is in flight, that result is discarded even when its
signature is valid.

Version ordering is fail-closed. A release older than the running version is never installed, so moving
a unit from staging to dev shows nothing until dev passes it. A running build whose version cannot be
ranked is also barred from network OTA rather than guessing whether a signed release is newer. Proper
release/dev builds must use a rankable `vX.Y.Z` style stamp.

All channels use the same Ed25519 verification, size/hash checks, inactive-slot install, trial boot,
health commit and automatic rollback contract.
