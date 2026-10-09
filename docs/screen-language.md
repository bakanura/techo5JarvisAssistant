# Screen language

Settings → Display → Screen language picks the language the screen is in. The menu shows each
language by its own name (Deutsch, Español, Français, Italiano, Nederlands), so someone who
doesn't read English can still find theirs. A new pick shows on the next frame, within about a second.

The first choice, **Match Assistant**, is what a new device starts in. The screen then uses the
language your voice assistant is set to in Home Assistant (Settings → Voice assistants), for the
pipeline picked on the device's Assistant select. The Show checks it every ten minutes. With the
Direct Brain it uses the Brain's language instead. A language the screen has no words for yet
shows English. Under Match Assistant the screen still reacts to "go home" or "weather" in every
language, since people often mix them.

Only the screen and the few things the Show says on its own are translated. What the assistant
answers is up to the assistant.

## How it works

The code lives in `echod/internal/i18n`, one file per language (`de.go`, `es.go`, …).

- **The English text is the key.** `i18n.T("Check now")` is "Jetzt suchen" on a German screen.
  A string nobody has translated yet stays English, so it still reads as something.
- **Most text is translated when it's drawn.** The display's text, width, wrap and clip helpers
  call `T` themselves, so a plain label in a row or a button doesn't need wrapping in code.
  Text made from pieces does need it, because "4:59 left" is not a key.
- **Values go in with templates.** Use `i18n.F("{time} left", "time", t)`. The translation puts
  `{time}` wherever its grammar wants it, as in "noch {time}". Older code uses
  `i18n.Sprintf("Missed: %s at %s", …)`. A translation that needs the values in another order
  uses indexes: `"%[2]s um %[1]s"`.
- **Dates go through `i18n.Date(t, layout)`.** It takes the usual Go layouts ("Monday, January 2",
  "Mon, Jan 2", "Jan 2", "January 2006"). Each language can reorder them in its `dates` map and
  has its own day and month names. `i18n.Names(s)` swaps the names inside other text, like an
  alarm's "Mon Wed Fri".
- **After changing the language setting, call `i18n.Changed()`.** Otherwise the old language
  stays cached for up to a second.

`TestTranslationsKeepPlaceholders` fails if a translation drops or adds a `{name}` or a `%d`.

## Wording

Write the way people at home talk to a kitchen gadget, not like a manual.

- Address people the familiar way: du, tú, tu, je. French uses vous, because that's how French
  devices talk.
- Keep it short. Rows on the Show 5 are narrow, and a long German compound gets cut off.
  Check the preview images (below) before calling a string done.
- Use the language's own quote marks: „…“ (de), «…» (es, it), « … » (fr), ‘…’ (nl).
- Don't translate names: Home Assistant, Drop In, Show, Spotify, Bluetooth. A task or alarm name
  someone typed in stays exactly as they typed it.
- Use the word people already use, even if it's English. "WLAN" and "Updates" in German, "wifi"
  in Spanish and Dutch.

| English | Deutsch | Español | Français | Italiano | Nederlands |
|---|---|---|---|---|---|
| Settings | Einstellungen | Configuración | Paramètres | Impostazioni | Instellingen |
| Alarm | Wecker | Alarma | Alarme | Sveglia | Wekker |
| Wake word | Aktivierungswort | Palabra de activación | Mot d'activation | Parola di attivazione | Activeringswoord |
| Wi-Fi | WLAN | Wifi | Wi-Fi | Wi-Fi | Wifi |
| Updates | Updates | Actualizaciones | Mises à jour | Aggiornamenti | Updates |

## Adding a language

1. Copy `de.go` to `<code>.go` and translate the values. Leave out any entry whose
   translation is the same as the English.
2. Fill in `dayMonth(...)`: the weekdays Monday first, the short weekdays, the months, and the
   short months. Add `dates` entries for layouts the language orders differently.
3. Add the code to `langs` in `i18n.go`, and add the language under its own name to `langOptions`
   in `echod/internal/feature/display/triggers.go`.
4. Run `go test ./internal/i18n ./internal/feature/display`.

To look at the screens in a language, render the display previews with the screen set to it:
`SHOW_PREVIEW=<dir> go test ./internal/feature/display -run TestShowScenesDraw`, with a test
that sets `config.Set().Screen().Language("<code>")` and calls `i18n.Changed()` first.
