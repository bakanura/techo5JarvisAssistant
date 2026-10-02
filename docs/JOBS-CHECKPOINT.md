# Jarvis Crown v1 — Job Ledger

Rule: complete and validate one job before starting the next. Do not build a release/rootfs tarball until the user explicitly asks.

## Phase A — Baseline and safety

- [x] **J01 — Reconstruct archived TECHO5 baseline**
  - Restore the actual Git repository from `Archive.zip` into this project.
  - Verify exact HEAD commit and worktree state.
  - Inventory bundled Crown boot/rootfs and Amonet assets.
  - Do not modify source behavior yet.

- [x] **J02 — Freeze Crown-v1 product scope and supported hardware contract**
  - CROWN / Echo Show 8 1st gen only.
  - Refuse cronos/checkers/other products in installer.
  - Add machine-readable product profile and docs.

- [x] **J03 — Third-party asset and redistribution audit**
  - Inventory Amonet/TWRP/boot/Lineage dependencies and licenses.
  - Keep private/local-only assets out of public-release paths where redistribution is unclear.

## Phase B — Voice appliance behavior

- [x] **J04 — Jarvis Crown wake-word profile**
  - `Hey Jarvis` primary maintained microWakeWord model.
  - `Okay Nabu` fallback.
  - Optional Alexa recovery model only if already shipped upstream.
  - One logical wake selector; second slot disabled by policy.
  - Implemented in fork commit `8b62921`.
  - Fresh-device fallback order: `hey_jarvis` → `okay_nabu` → `alexa` → any installed model.
  - Exactly one wake model active at a time; explicit `No wake word` still survives restart.
  - Static/parser validation passed; Go test execution deferred because this environment has Go 1.23.2 and the repo requires Go 1.26.0.

- [x] **J05 — Alexa-like voice state machine**
  - Idle → wake → listening → processing → responding → follow-up → idle.
  - Silent 6 s follow-up, max 2 automatic turns.
  - HA-requested continuation still takes precedence.
  - Wake detector remains local; no HA wake re-arm automation.
  - Implemented in fork commit `82cefa1`.
  - Static/parser validation passed; Go tests remain blocked by local Go 1.23.2 vs repo-required Go 1.26.0.

- [x] **J06 — Local stop/barge-in/audio defaults**
  - Preserve AEC and wake-over-playback.
  - Default stop threshold 0.55; playback floor remains 0.50.
  - Local stop ladder audited: ring/alarm → turn/TTS → announcement → media pause → reminder.
  - Active-call far-end wake protection retained.
  - Implemented in fork commit `7a3898a`.
  - Static/parser validation passed; Go tests remain blocked by local Go 1.23.2 vs repo-required Go 1.26.0.

- [x] **J07 — Automatic brain mode**
  - HA Assist first when subscribed/healthy at turn start.
  - Existing Direct Brain fallback when HA is unavailable between turns.
  - No mid-turn backend switch: the selected backend is pinned on the open conversation.
  - Visible conversation state carries the selected backend so display behavior follows the actual turn.
  - Setup UI exposes Automatic / Home Assistant / Direct and refuses incomplete Automatic/Direct fallback configuration.
  - Existing direct timers/alarms/reminders/intercom/radio/calendar/weather/screen/search tools remain unchanged.
  - Implemented in fork commit `2940514`.
  - Static/parser validation passed; Go tests remain blocked by local Go 1.23.2 vs repo-required Go 1.26.0.

## Phase C — Display appliance behavior

- [x] **J08 — Jarvis Display product defaults**
  - Fresh devices default to streamed `jarvis-display`, idle ON, native kiosk/headerless request ON.
  - Persisted user settings override product defaults and survive restart/OTA.
  - Browser-side sidebar/header enforcement remains owned by J09; stale-page lifecycle remains J10.
  - Static/format validation passed; Go tests remain blocked by local Go 1.23.2 vs repo-required Go 1.26.0.

- [x] **J09 — Dashcast Crown appliance mode**
  - `JARVIS_CROWN_MODE=1` forces 1280×800, `/jarvis-display`, and kiosk ON server-side.
  - Device-supplied viewport/path/kiosk values cannot override the Crown appliance contract.
  - Dashcast owns Home Assistant header + sidebar suppression; HA Kiosk Mode / edge CSS hacks are unsupported and unnecessary.
  - Generic Dashcast behavior is unchanged when appliance mode is disabled.
  - Regression tests added for viewport/path/kiosk ownership and browser-chrome ownership.
  - Go/gofmt + injected JavaScript syntax validation passed; Go tests remain blocked by local Go 1.23.2 vs dashcast-required Go 1.26.8.

- [x] **J10 — Dashcast warm-tab generation contract**
  - Warm key includes authenticated device, viewport, path, kiosk/header state, and `JARVIS_CROWN_UI_GENERATION`.
  - Crown mode defaults UI generation to `1`; changing generation can never match an older parked tab.
  - Added one-shot ESPHome `dashboard_reload` action: the device reconnects with authenticated `cold=true`, Dashcast discards the matching parked page, and opens a fresh Chrome tab.
  - Cold reload state is not persisted and is consumed only after the hello is successfully sent.
  - Added regression tests for generation-aware keys, cold flag preservation, warm-pool discard, and one-shot state.
  - Implemented in fork commit `dfeb98b`.
  - Static/gofmt/profile validation passed; Go tests remain blocked by local Go 1.23.2 vs Dashcast-required Go 1.26.8 / echod-required Go 1.26.0.

## Phase D — Alexa-like feature integration

- [x] **J11 — Timers, alarms, reminders experience audit/polish**
  - Audited existing local timer persistence, alarm/snooze persistence, missed-timer restart handling and Crown full-screen ringing/touch-stop behavior.
  - Added strict local German ring Stop commands alongside English without consuming unrelated smart-home/media requests.
  - Added strict local German Snooze commands and common German minute lengths; a snooze still requires an explicit snooze word.
  - Reminder/alarm labels now prefer live HA speech, fall back to configured Direct Wyoming TTS when HA is unavailable, then degrade to local chime + screen only.
  - Implemented in fork commit `6f6fb8b`.
  - Pure local stop/snooze parser tests passed on the available Go toolchain; full package tests remain blocked by local Go 1.23.2 vs repo-required Go 1.26.0.

- [x] **J12 — Announcements / intercom / Drop In productization**
  - Existing HA Assist Satellite announcement + `StartConversation` path retained; DND now acknowledges/suppresses proactive HA audio without opening the microphone.
  - Native peer announcements remain local/mDNS-based and now require signed HMAC requests only; Jarvis Crown rejects the legacy plaintext `X-Techo5-House` credential path.
  - DND rejects incoming intercom and mutes incoming house announcements while keeping their screen card; alarms/timers/local wake interactions remain available.
  - Drop In remains explicit opt-in and known-peer-name+address only, requires the existing audible chime to complete before auto-answer, falls back to ringing when the chime is interrupted, and now uses a persistent distinct orange privacy light in addition to the `Drop In` screen.
  - Implemented in fork commit `8d9f030`.
  - gofmt/diff/invariant validation passed; full Go tests remain blocked by local Go 1.23.2 vs repo-required Go 1.26.0.

- [x] **J13 — SIP calling and contacts polish**
  - SIP signaling is TLS-only and media SRTP-only; the legacy TECHO5 `plain` account bit is decoded for compatibility but cannot downgrade transport.
  - Provider certificate validation remains enabled with the SIP host as `ServerName`; TLS 1.2 is the minimum.
  - SIP account files remain owner-only (`0600`).
  - `phone_account`, `phone_contacts` and HA-triggered `phone_call` refuse to carry password/address-book/dialed-number data until the ESPHome API has a real encryption key.
  - Incoming caller-ID/contact labels are sanitized and length-bounded before entering call state; matching local contacts override provider display names.
  - Routine phone logs no longer include peer/number fields; HA call events retain peer identity for automations.
  - Existing Crown incoming/outgoing/connected call UI and wake-word suppression during calls remain intact.
  - Implemented in fork commit `0c34e39`.
  - gofmt/diff/invariant validation passed; full Go tests remain blocked by local Go 1.23.2 vs repo-required Go 1.26.0.

- [x] **J14 — Music / now-playing / Sendspin integration contract**
  - Music Assistant remains the server-side owner of library/search/queue/grouping; Crown remains a Sendspin room endpoint.
  - Fresh installs show full native Now Playing for 30 seconds, then return to Jarvis Display with the native music strip over the streamed dashboard.
  - Existing configs with omitted `music_strip` keep historical Full page behavior, so OTA does not reinterpret an explicit zero-value choice.
  - The strip is rendered over streamed Dashcast frames and owns only visible taps inside its rectangle; every other browser touch/edge gesture continues to Dashcast.
  - Camera/settings/Wi-Fi/drawer/etc. retain higher input priority and cannot be tapped through an invisible strip.
  - Existing Sendspin metadata/artwork/controller/held-pause semantics retained; Music Assistant transport stays server-owned.
  - Implemented in fork commit `c49ac3e`.
  - Config regression tests pass in a temporary lowered-go-directive copy; display target tests are committed/static-clean but full compilation remains blocked by the local Go 1.23.2 / repo Go 1.26 dependency environment.

- [x] **J15 — Camera / doorbell / proactive conversation UX**
  - Added encrypted-API-only HA camera/list/show actions and a first-class `home_doorbell` action.
  - Doorbell popup is visual even under DND/quiet hours; local cue and camera audio are suppressed there.
  - Doorbell/camera actions never open the microphone themselves; HA Assist Satellite `StartConversation` remains the explicit proactive-reply path and J12 DND rules can veto it.
  - Camera entity IDs are constrained and camera names are printable/length-bounded before reaching the screen.
  - Proactive HA announcement/question text is no longer written to routine logs.
  - Explicit regression test proves camera/doorbell temporarily covers a requested Jarvis dashboard and the same dashboard returns afterward.
  - Full Go test execution remains blocked by local Go 1.23.2 vs repo-required Go 1.26.0.

- [x] **J16 — Night mode / DND / bedside behavior**
  - Night and DND remain separate controls: screen policy vs proactive-communication/privacy policy.
  - A ringing alarm/timer now wakes an otherwise dark night screen and is capped to gentler night brightness; sunrise alarms keep their own ramp.
  - Reminder-only events do not light a sleeping room.
  - DND now also turns away incoming SIP calls, in addition to the J12 intercom/Drop In/announcement privacy rules and J15 visual-only doorbell behavior.
  - Alarms, timers, explicit local interactions and outgoing calls remain available under DND.
  - Added regression tests for dark-screen ring wake and SIP DND gating.
  - Full Go test execution remains blocked by local Go 1.23.2 vs repo-required Go 1.26.0.

## Phase E — One installer

- [x] **J17 — Crown installer host preflight**
  - Linux + Python 3.10+, adb/fastboot/git/bash/timeout, USB-serial permissions, ModemManager, writable work/backup storage, and >=8 GiB free space.
  - Implemented as a host-only gate: it never invokes adb/fastboot, enumerates USB devices, or writes to an Echo.
  - Local Amonet input must be structurally complete and declare `DEVICE=crown`; an optional Lineage ZIP must declare `pre-device=crown`.
  - Seven pure-Python regression tests cover wrong/incomplete Amonet, wrong Crown ZIP metadata, missing fastboot, and low disk space.
  - Container rehearsal deliberately failed closed with adb/fastboot absent and confirmed no device command was executed.

- [x] **J18 — Crown identification and fail-closed device gate**
  - Requires exactly one connected fastboot device and queries it by serial.
  - Requires `product=CROWN` and a recognizable boolean `unlock_status`; explicitly rejects every other product/state.
  - `lk_build_desc` is diagnostic only and cannot broaden supported hardware.
  - Device-gate implementation is read-only: regression tests forbid write-capable fastboot verbs and cover zero/multiple devices, timeout, malformed status, wrong product, locked Crown and already-unlocked Crown.
  - 16 combined J17/J18 pure-Python tests pass; no real device I/O or build occurred.

- [x] **J19 — Amonet Crown unlock wrapper**
  - Uses the known-good local Crown Amonet tooling only after J17/J18 pass; critical script/profile/payload/fastboot bytes are SHA-256 pinned.
  - Already-unlocked Crown skips Amonet entirely; locked Crown requires exact `UNLOCK CROWN` confirmation.
  - Amonet success is never trusted alone: the same serial must return as `CROWN` with `unlock_status=true` through the read-only identity gate.
  - Hash mismatch, exploit timeout/error, serial change or unconfirmed unlock fails closed and requires re-identification before retry.
  - 24 combined J17-J19 pure-Python tests pass; no real device I/O occurred.

- [x] **J20 — TWRP + recovery backup orchestration**
  - Reuses already-running Crown TWRP; otherwise flashes only the SHA-256-pinned TWRP image to `recovery` + `swdl` and reboots recovery after re-proving the same unlocked Crown identity.
  - Never uses unsupported `fastboot boot` and never writes `lk`, preloader, expdb, tee*, boot, system or userdata in this stage.
  - Requires Crown/root TWRP and atomically saves p1-p11 plus p14/p15 and eMMC boot0/boot1 before later destructive storage work.
  - Every image is checked for exact block size and must match both device-side and host-side SHA-256; incomplete backups cannot be reused.
  - Existing backups are accepted only after full hash + coverage re-verification.
  - 7 J20 regression tests pass; no real device I/O occurred.
  - Documented caveat: this is a post-Amonet safety backup, not a pristine pre-unlock factory image.

- [x] **J21 — Lineage vendor-driver staging**
  - Requires Crown Lineage metadata before destructive work and uses LineageOS only as the proven vendor/system driver source; Android never needs to boot.
  - Fixes the real Crown TWRP handoff failure by using `twrp reboot recovery` after formatting data and waiting for recovery + mounted/writable `/data`.
  - Verifies the uploaded ZIP by SHA-256 before TWRP installation.
  - Mounts installed system read-only and requires both MT7668 Wi-Fi and Bluetooth modules to match kernel ABI `4.9.337-g8d928c5176cc` before later slot-store conversion.
  - 5 J21 regression tests pass; no real device I/O occurred.

- [x] **J22 — Jarvis Crown rootfs install/provision wrapper**
  - Reuses upstream `install-show.py`/`slotctl` destructive storage path instead of cloning partition logic.
  - Re-verifies J20 backup + target serial, pins the known-good Crown boot SHA-256, and requires an exact SHA-256 plus `jarvis-crown-v1`/`crown` marker for the future Jarvis rootfs.
  - J21 pre-staged mode consumes the already-verified Lineage vendor tree without formatting/reinstalling it and bypasses network release lookup.
  - Forces Amazon-logo preservation so J22 never writes `expdb`; destructive targets stay limited to `boot` and documented `system`/mmcblk0p12 slot-store conversion.
  - Provisions name, HA PSK, optional SSH/Wi-Fi, and persistent `jarvis-crown-v1` profile marker without freezing product defaults into state.json.
  - Exact final confirmation is `ERASE LINEAGE INSTALL JARVIS CROWN`; 14 J21/J22 tests pass; no rootfs/release tarball was built.

- [x] **J22B — Shared Jarvis Show platform / Checkers integration**
  - One shared daemon/rootfs now targets both supported 2019 Shows: Jarvis Crown v1 (`crown` / Echo Show 8 1st gen) and Jarvis Checkers v1 (`checkers` / Echo Show 5 1st gen).
  - Destructive installer inputs remain board-specific and cross-board identity, Lineage, recovery, backup and boot-image checks fail closed.
  - Dashcast appliance geometry is board-owned: Crown 1280x800, Checkers 960x480; both force `/jarvis-display` and the same headerless/sidebarless contract.
  - Added shared `jarvis-show-v1` rootfs marker with explicit supported-board list plus a `jarvis-show` installer front-end.
  - Crown destructive asset hashes remain pinned from the supplied archive. Checkers real flashing remains blocked until trusted Checkers Amonet/TWRP/boot bytes are locally supplied and SHA-256 pinned; synthetic tests do not authorize a real flash.
  - 55 Python installer tests pass across the Crown compatibility suite plus new Checkers/shared-rootfs cases. Dashcast Go tests are written but cannot execute here because the archive requires Go >=1.26 and the local toolchain is 1.23.2.
  - No rootfs/release tarball was built.

- [x] **J23 — One-command installer acceptance tests**
  - `tools/jarvis-show.py install --board crown|checkers` now orchestrates preflight → read-only board identity → optional Amonet unlock → TWRP → verified recovery backup → local boot/rootfs/backup validation → exact destructive confirmation → Lineage vendor staging → pre-staged slot-store install.
  - All boot/rootfs/backup inputs are validated before the first userdata format; a bad local install input never reaches Lineage staging.
  - Added a hidden, board-bound `install-show.py --jarvis-show-stage-lineage-only` mode that requires J20's complete backup, installs/verifies only the Lineage vendor tree, and returns before release lookup, boot flash or slot-store conversion.
  - The same exact product-specific erase phrase gates the first userdata write and the final pre-staged install plan.
  - Tests cover wrong-board identity, locked/unlocked Crown, locked Checkers with synthetic pins, failed preflight/backup/rootfs plan/confirmation/Lineage stage, and board-specific stage-only early return.
  - 68 Python Jarvis installer tests pass; all destructive device interactions in J23 acceptance are mocked. No real device I/O occurred.
  - Checkers real flashing remains fail-closed until trusted Amonet/TWRP/boot digests are pinned.
  - No rootfs/release tarball was built.

## Phase F — OTA and releases

- [ ] **J24 — Fork-owned OTA identity**
  - New Ed25519 public key in daemon/installer.
  - Private seed kept outside repo.
  - Configurable release repository/base URL.

- [ ] **J25 — Preserve A/B rootfs update/rollback contract**
  - Trial slot, health commit, automatic rollback.
  - Settings/provisioning persist across OTA.

- [ ] **J26 — Stable/dev channel behavior**
  - Stable releases and rolling prerelease/dev channel without downgrades.

- [ ] **J27 — Release tooling and CI (NO BUILD YET)**
  - Test/build/sign/publish scripts/workflows.
  - Do not execute rootfs/release tar creation until explicitly requested.

## Phase G — Validation and docs

- [ ] **J28 — Go/Python/shell test suite**
  - Unit tests for profile/wake/backend/dashcast/installer/release behavior.
  - Static/syntax validation available in this environment.

- [ ] **J29 — Upgrade/recovery documentation**
  - Upstream sync workflow, rollback, USB recovery, key rotation.

- [ ] **J30 — Final install + OTA acceptance checklist**
  - Fresh supported Show (Crown or Checkers) → its Jarvis product via one installer.
  - Subsequent repo release → signed A/B OTA → health commit/rollback validation.

## Phase H — Security hardening

- [ ] **J31 — Threat model and exposed-surface inventory**
  - Enumerate listeners, protocols, trust boundaries, credentials, privileged processes and update paths.
  - Classify LAN-only, device-local and internet-facing surfaces.
  - Record explicit security invariants for an IoT-network deployment.

- [ ] **J32 — Network/service hardening**
  - Bind management/debug services to the narrowest interfaces possible.
  - Disable or authenticate unused web/SSH/debug endpoints by default.
  - Review Dashcast, ESPHome, SIP, intercom, setup UI, camera and Bluetooth exposure.
  - Fail closed on missing credentials or malformed peer identity.

- [ ] **J33 — Secret, identity and privilege hardening**
  - Audit file permissions and persistence for API keys, Wi-Fi credentials, house secrets and update keys.
  - Ensure logs/support bundles never expose secrets.
  - Reduce daemon/helper privileges where feasible without breaking Crown hardware access.

- [ ] **J34 — OTA / supply-chain hardening**
  - Verify signed manifest, hash, channel and rollback logic fail closed.
  - Pin third-party inputs by digest/commit and audit release provenance.
  - Reject unsigned/downgrade/cross-product updates.

- [ ] **J35 — Security regression suite and deployment policy**
  - Add tests for unauthorized callers, bad tokens, wrong house peers, malformed update metadata and recovery paths.
  - Publish recommended IoT firewall policy and secure-default checklist.
  - No release acceptance until security tests pass.
## Phase I — Known broken behavior / regression repairs

- [ ] **J36 — Web Setup page repair and regression suite**
  - Reproduce the broken setup-page paths from the archived build instead of assuming the UI works.
  - Audit GET/POST routing, tabs, CSRF/token handling, redirects, saved-state reload, and malformed/missing values.
  - Verify Brain, Wi-Fi/Home Assistant access, alarms/timers, dashboard, security, update and diagnostics forms actually round-trip.
  - Add browserless HTTP regression tests for every repaired page/action.
  - Treat setup UI as a management surface and carry its auth/exposure findings into J31/J32.

- [ ] **J37 — Wi-Fi / first-boot provisioning recovery**
  - Ensure a freshly installed Crown always has an explicit, recoverable Wi-Fi provisioning path.
  - Cover no-network, wrong-password, lost-HA, and USB/serial recovery states.
  - Never depend on zeroconf discovery as the only way to recover a device.

- [ ] **J38 — Live-display regression acceptance**
  - Re-test the historical black-bar/header/sidebar failures against Jarvis Crown Dashcast appliance mode.
  - Verify WIND/weather/room/clock custom-card geometry is not altered by device/browser chrome handling.
  - Require a fresh-session screenshot/geometry check before declaring display acceptance.

- [ ] **J39 — Voice/web fallback regression acceptance**
  - Verify general questions such as Taco queries cannot be misrouted into arbitrary device-control intents.
  - Verify Direct Brain + SearXNG fallback can answer when HA/Klar rejects or HA is unavailable.
  - Keep HA/Klar-specific server bugs documented separately from firmware behavior.

- [ ] **J40 — Known-bug closure pass**
  - Revisit every defect recorded during the original livingRoomEcho8 deployment and mark fixed, externally-owned, or intentionally unsupported.
  - No v1 release acceptance with an unclassified known regression.


- [ ] **J41 — Final Crown + Checkers anti-brick / upstream safety audit (RELEASE BLOCKER)**
  - Re-read TECHO5's current Crown and Checkers install, recovery, A/B update, boot-image and partition documentation before any release build or device install.
  - Compare every Jarvis Show / board-profile change against upstream safety assumptions; explicitly flag anything that writes bootloader, preloader, LK, recovery, boot, system/slot metadata, userdata, persist, nvram/nvcfg, protect partitions or eMMC boot areas.
  - Verify product detection is fail-closed and board-bound (`CROWN` or `CHECKERS` only, never cross-flashed), backups happen before destructive writes, and installer aborts on unexpected partition/layout/device state.
  - Verify ordinary OTA remains rootfs A/B only, keeps upstream trial-slot health/commit/automatic rollback semantics, and cannot silently become a boot/kernel/partition update.
  - Verify no Jarvis change re-enables deprecated/unsafe TECHO5 paths or bypasses upstream Crown guards.
  - Cross-check known upstream Crown/Checkers brick/recovery warnings and documented failure modes against the final installer and OTA code.
  - Require a written PASS/WARN/FAIL anti-brick report before J30 release acceptance or any real-device install.
  - Do not build or flash anything as part of this audit unless explicitly requested.

## Phase J — Resilient music routing and full-screen playback

- [ ] **J42 — Area-primary speaker fallback routing**
  - Define one preferred Music Assistant/HA player per area, with that area's Jarvis Crown as the automatic fallback endpoint.
  - If the preferred speaker disappears during active playback, keep the session alive by continuing on Crown when technically possible; never stop solely because the primary went unavailable.
  - Do not migrate an already-failed-over session back mid-song merely because the primary comes online again.
  - On the next explicit Play request, re-resolve the area: preferred speaker when online, otherwise Crown.
  - If an area has no configured preferred speaker, Crown is the primary endpoint for that area.

- [ ] **J43 — Dynamic whole-home / named-group playback resolution**
  - Resolve commands such as `Spiele Musik in der ganzen Wohnung` against the configured Music Assistant/HA group (for example `wohnung`).
  - For each member area, select the preferred online room speaker; substitute that room's Jarvis Crown when the preferred speaker is unavailable or absent.
  - Preserve synchronized multi-room playback through Music Assistant/Sendspin; do not invent a competing grouping engine in `echod`.
  - Re-evaluate group membership on every new explicit Play request while leaving an already-playing fallback session stable.

- [ ] **J44 — Full-screen music dashboard and persistent Now Playing**
  - Replace the music-strip-first product UX with a dedicated full-screen music surface while playback is active; the strip remains only a defensive fallback.
  - Always show artwork, title, artist, album, progress, transport, volume, active output/room/group, and enough queue context to understand what is playing and where.
  - Provide touch actions for previous/play-pause/next, queue, favorite/library actions and album navigation where the Music Assistant APIs expose them safely.
  - Automatically return to `/jarvis-display` after playback truly ends; camera/call/alarm/privacy-critical screens retain higher priority.
  - Keep the full-screen Now Playing available for the complete active playback session, not only the first 30 seconds.

- [ ] **J45 — Music failover / group regression suite**
  - Test primary-offline-at-start, primary-loss-mid-playback, primary-recovery-without-forced-migration, next-command-primary-restoration, no-primary-room fallback and named whole-home group substitution.
  - Verify screen reports the real active endpoint/group and never claims music is playing on a speaker that is offline.
  - Include these scenarios in final release acceptance.
