# Setup page security and interaction contract

The device-local Setup page is the management surface for settings that are awkward to enter on the
small touchscreen. It is deliberately simple HTML served from the device itself: no framework, no
external JavaScript and no internet dependency.

## Exposure

The page lives on the device web listener and is **off by default**. It can be opened from the device
settings or Home Assistant. A Dot may additionally open it by holding its real action button. The
listener closes again after inactivity and sessions are forgotten when Setup closes or the daemon
restarts.

The page is plain HTTP on the local network. That is why opening the page is not enough to manage the
device: every browser session requires a physical-presence approval on the device, and the IoT
firewall policy must keep the management listener off untrusted networks.

## Physical-presence approval

The original generic page assumed every device had an action button. Echo Show 5/8 do not. Telling a
Show owner to press that nonexistent button made Setup unusable even though the display already had an
approval screen.

Jarvis Show uses the hardware-appropriate approval path:

- **Crown / Checkers / other screen devices:** the device displays `Allow` and `Not now`. Tapping
  `Allow` authorizes only the browser that is already waiting.
- **Dot:** press the real action button while the browser is waiting.

One request waits at a time. It expires after sixty seconds. Repeated unanswered requests trigger a
cool-down. Approval cannot be saved up for a future browser.

The browser receives an `HttpOnly`, `SameSite=Strict` session cookie scoped to the device web port.
Every state-changing form also carries the live session token and posts to `/setup/save`; mismatched
or missing tokens are refused.

## HTTP contract

Read-only endpoints accept only GET/HEAD:

- `/setup`
- `/setup/state`
- `/setup/diagnostics.txt`

State-changing endpoints accept POST only:

- `/setup/wait`
- `/setup/save`
- `/setup/photo`

Request bodies are bounded. Photo upload has its own bounded multipart limit. Diagnostics, photos and
all settings remain behind the same physical-presence session.

## Settings round-trip contract

The Setup page must be able to render, save and re-render the major management areas without losing
state or rendering secrets back into HTML:

- alarms, timers and reminders;
- Wi-Fi and Home Assistant/adoption state;
- Direct Brain / STT / TTS / LLM / SearXNG settings;
- follow-up listening settings;
- Dashcast/Jarvis dashboard address and key state;
- update channel/automatic update settings;
- weather/location and calendars;
- SIP/contacts, house/intercom settings and radio stations;
- photos, diagnostics and general settings.

Secrets may be accepted but are never rendered back. The UI shows only that a key/password is saved.
A save returns to the tab it came from and malformed values are reported on that same tab.

## What the page must never do

The Setup page is not a firmware-upload or shell interface. It must never:

- execute arbitrary commands;
- accept SSH private keys;
- expose the Home Assistant API key or ESPHome PSK;
- bypass signed OTA manifests;
- write boot/kernel/partition images;
- make camera/screen diagnostics public without the local authorization gate.

## Regression requirements

Release validation must cover:

1. Show builds never claim an action button exists.
2. The on-screen `Allow` path really authorizes a waiting browser.
3. Every rendered settings form has a corresponding save handler.
4. Every settings POST uses `/setup/save` and the session token is checked before dispatch.
5. Read-only routes reject state-changing methods.
6. Major Brain, listening, dashboard, update, alarm/timer and tab state round-trip through HTTP.
7. Diagnostics remain inaccessible before physical approval.
8. The session cookie remains `HttpOnly` and `SameSite=Strict`.

These tests are intentionally browserless: they exercise the same HTTP handlers and persisted state as
a browser, while making regressions deterministic in CI.
