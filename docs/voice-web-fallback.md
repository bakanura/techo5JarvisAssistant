# General questions, web search and backend ownership

Jarvis Show has two voice backends: Home Assistant Assist and Direct Brain. In `Automatic` mode the
backend is chosen once at the beginning of a turn. A live HA subscription wins; Direct Brain is used
only when HA is unavailable and the Direct STT/TTS/LLM services are configured. A turn never changes
backend halfway through.

## Direct Brain information policy

Direct Brain must distinguish **information** from **device actions**. A word that happens to resemble
an entity, command or device name is not sufficient reason to call a device tool.

Questions about food, recipes, definitions, facts, people, places and the web are information requests
unless the user explicitly asks to control something. Example:

- `Was ist ein Taco?` -> answer the question; do not call a switch/light/media tool.
- `Suche im Web nach einem Taco-Rezept` -> use SearXNG, then read a useful result.
- `Mach das Küchenlicht aus` -> device-control tooling is appropriate.

When SearXNG is configured, explicit requests to search/look up/find online, find a source, or find a
recipe use `web_search` even if the LLM believes it already knows an answer. `read_page` is available
for grounding the spoken result in a useful search result.

## Home Assistant / Klar boundary

The historical `livingRoomEcho8` failure where `Was ist ein Taco?` became `Was soll ich einschalten?`
and a Taco-recipe query was interpreted as camera/screen control occurred **inside the HA/Klar
conversation path while HA was online**. Jarvis Show cannot safely infer that HA's completed intent was
wrong and silently execute a different assistant behind HA's back.

Therefore release acceptance has two separate gates:

1. Direct Brain firmware tests verify information/search behavior and HA-unavailable fallback.
2. The deployed HA/Klar pipeline must independently pass the same Taco/general-question regression
   set. A failure there remains an HA/Klar configuration/integration bug, not a firmware success.
