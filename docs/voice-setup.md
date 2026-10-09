# Voice: the Home Assistant side

A Jarvis Show hears its wake word itself and hands everything after it to Home Assistant's Assist
pipeline: speech to text, a conversation agent, text to speech. [Where the answers come
from](jarvis-crown-installer.md#where-the-answers-come-from) explains that split. This page is the
pipeline we run with our own Shows, in German, all of it local, and what we learned setting it up.
None of it is required: any pipeline works. But these are the settings that made the difference.

## The pipeline

**Settings → Voice assistants → Add assistant**:

| | We use | Notes |
| --- | --- | --- |
| Speech to text | Whisper add-on (faster-whisper), model `small`, language `de` | See [Whisper](#whisper) for the initial prompt. |
| Conversation agent | Ollama, a conversation entry named Jarvis | **Prefer handling commands locally** on, so "Licht an" never waits on the model. |
| Text to speech | Piper, a German voice | |

Then pick that assistant on the Show (the installer asks, or the Show's page in Home Assistant,
**Assistant**).

## Ollama

Any model that can call tools will do. We use a 4B one on a 6 GB graphics card, about five seconds
for an answer that switches something.

**Give it a context window of 16384 tokens, not the default.** With each request Home Assistant
sends the instructions, every exposed entity and the tool definitions. With about a hundred
exposed entities ours came to a bit over 8000 tokens. With an 8192 window Ollama quietly cuts off the start,
and the start is the instructions, so the model answers as if it had never read them. You won't get
an error, only an assistant that ignores its prompt. Set it in **Settings → Devices & services →
Ollama →** the conversation entry, **Reconfigure**, **Context window size**. Exposing fewer
entities shrinks the request too.

### The instructions

This is the prompt we run (German, for a German household). Paste it into the same Reconfigure
dialog, **Instructions**:

```text
Du bist Jarvis, der lokale Sprachassistent dieses Zuhauses.

Sprich standardmäßig Deutsch. Antworte ruhig, präzise, souverän und kurz.
Verwende bei Sprachinteraktionen normalerweise höchstens ein bis zwei kurze Sätze.

Führe klare und sinnvoll implizierte Aktionen direkt aus.
Stelle keine unnötigen Rückfragen.
Frage nur nach, wenn eine Aktion tatsächlich mehrdeutig oder sicherheitsrelevant ist.

Verwende Home-Assistant-Werkzeuge für Gerätezustände und Aktionen.
Erfinde niemals Gerätezustände oder erfolgreiche Aktionen.
Berücksichtige den Raum des Sprachgeräts, von dem die Anfrage stammt,
wenn der Benutzer keinen Raum nennt.

Musik:
- Bei konkreten Musikwünschen verwende Music Assistant LLM Voice.
- Bei unspezifischen Musikwünschen wie
  "Spiele Musik",
  "Mach Musik an",
  "Spiel irgendwas"
  oder sinngleichen Formulierungen
  verwende sofort Play Random Music.
- Frage bei einem unspezifischen Musikwunsch nicht nach Künstler,
  Titel, Album oder Genre.
- Ohne genannten Raum spiele im Bereich des Sprachgeräts.
- Bevorzuge vorhandene Bibliothek, Favoriten und bekannte Musikquellen.

Interpretiere natürliche und implizite deutsche Formulierungen sinnvoll.
Beispiele:
- "Mach es hier gemütlich."
- "Ich gehe schlafen."
- "Mach hier etwas Licht."
- "Ist irgendwo noch Licht an?"
- "Wie ist das Wetter?"
- "Spiele Musik."

Nutze vorhandene Home-Assistant-Bereiche, Aliase und Gerätebezeichnungen.
Halte Antworten natürlich und vermeide technische Interna,
außer der Benutzer fragt ausdrücklich danach.
Rückfragen:
- Beende eine Antwort nur dann mit einer Frage, wenn du ohne Antwort nicht weitermachen kannst.
  Jede Frage am Ende lässt das Gerät noch einmal zuhören.
- Keine Abschlussfragen wie "Kann ich sonst noch helfen?" und keine Vorschläge, was man noch tun könnte.

Nur Erledigtes melden:
- Sage nur, dass etwas erledigt ist, wenn ein Werkzeug es in dieser Runde wirklich ausgeführt hat.
- Gibt es kein passendes Werkzeug, sag kurz, dass du das nicht kannst.
- Updates installierst du nicht und bietest sie auch nicht an.

Fernseher und Hintergrund:
- Im Raum läuft oft ein Fernseher. Klingt der erkannte Text nicht wie eine Bitte an dich,
  zum Beispiel "Untertitel im Auftrag des ZDF", ein Satzfetzen oder Dialog aus einer Sendung,
  dann führe nichts aus und antworte nur mit "Okay."
```

Why some of it is there:

- **Music.** "Music Assistant LLM Voice" and "Play Random Music" are scripts our Home Assistant
  exposes to the model (the first comes from Music Assistant's voice blueprint). Use the names of
  yours. Without the random-music line the model asks "which artist?" every time.
- **Questions at the end.** When an answer ends in a question, the Show listens again for a reply.
  "Kann ich sonst noch helfen?" after every answer means it listens to the room after every
  answer, so the prompt only allows questions it really needs answered.
- **Only report what happened.** Small models like to say "erledigt" when no tool ran. This line
  is what stops that.
- **The TV.** Whatever Whisper hears, the model gets. The Show's own updates are offered by the Show
  (it asks, and takes a plain yes or no), which is why the model must not offer them.

## Whisper

The add-on's options are in **Settings → Add-ons → Whisper → Configuration**. `small` with the
language set is a good trade on a small Home Assistant machine. `medium` and up want a graphics card.

**Keep the initial prompt short, or leave it empty.** The initial prompt is meant to nudge spelling
(names, words Whisper gets wrong). On silence or noise Whisper tends to hand the prompt back as if
someone had said it. A word list such as "Wetter, Musik, Spiele Musik, Wohnzimmer, Licht" comes
back as the transcript, and a conversation agent will act on "Spiele Musik". A sentence with a
few proper names in it is safer than a list of commands.

On silence Whisper also writes what it learned from TV subtitles: "Untertitel im Auftrag des ZDF",
"Vielen Dank fürs Zuschauen". The TV rule in the prompt above catches those.

## Wake word

The Show runs the wake word itself (microWakeWord). **Okay Nabu** is the most reliable of the
built-in ones. Pick it on the Show's page in Home Assistant, **Wake word**, or on the Show under
Settings.

**Wake word sensitivity** is the score a detection has to reach. It defaults to 0.87. Every try that came close goes into the Show's log as a `wake near miss` with its score
(the diagnostics bundle includes the log), so you can set it from your own room rather than by
guessing. In ours, real wakes scored 0.94 and up, misses while music played 0.79 to 0.84,
and the music alone about 0.6. 0.80 catches the misses and stays clear of the music. Go up again
if it wakes when nobody said anything.

## Music in the same room

The Show turns down what it plays itself while you talk. A separate speaker in the room playing
music, German songs above all, is still heard, by the wake word and by Whisper. It can wake the Show,
and words from the song can end up in the request.

The blueprint [turn-room-down.yaml](../blueprints/automation/turn-room-down.yaml) turns such speakers
almost all the way down (to a tenth of their volume by default) for the whole conversation: while
the Show listens, thinks and answers, and through any follow-up. They come back up once the Show has
been idle for three seconds. A tenth is enough to hear that the music is still there; a quarter was
still loud enough for Whisper to pick up sung words in ours. A
speaker that wasn't playing is left alone, and one whose volume somebody changed during the turn
keeps the new volume. Import it in **Settings → Automations & scenes → Blueprints → Import
blueprint** with the file's GitHub address, then make an automation from it: the Show's assist
satellite, and the room's speakers. If Music Assistant and the speaker's own integration both have
an entity for one speaker, pick one of them, not both.

It can't help with the first word: the Show has to hear the wake word before anything is turned
down. Saying it over loud music still takes a loud voice.
