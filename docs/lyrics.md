# Lyrics on the now-playing page

When Music Assistant plays a song from the house's own music server, the now-playing page gets a
button with lines of text on it, left of the clock. Tapping it swaps the song's details for its words;
tapping again brings them back. Words with times in them follow the song: the line being sung is lit,
the one before stays above it while it is short, and the page moves on the moment the next line starts.
Words without times move down through the song as it plays.

Nothing here goes outside the house. The words come from the music files themselves, as tagged in the
library (a `LYRICS`/`USLT` tag, with or without LRC times in it), and the music server hands them out
over the OpenSubsonic `getLyricsBySongId` call, which current Navidrome versions answer.

## Why through Home Assistant

The Show sits on the IoT network and reaches Home Assistant, not the music server. Home Assistant
reaches both, so a `rest_command` there asks the server, and the Show calls that command with the
song's id and reads the answer (`return_response`). The music server's login stays in Home Assistant's
`secrets.yaml`; the Show never has it.

Without the command the Show simply has no lyrics button. It tries again every ten minutes, so adding
the command later needs nothing on the Show.

Only songs Music Assistant took from an OpenSubsonic provider get words: that is where the song's id on
the server comes from. Radio, Spotify and the like show the page as before.

## Setting it up

1. On the music server, make a user for this, e.g. `jarvis-lyrics`, with no admin rights and access to
   the library. Store its password where you keep such things:

       secret-tool store --label="Navidrome jarvis-lyrics" service navidrome user jarvis-lyrics

2. Make the login line and add it to Home Assistant's `secrets.yaml` (back the file up first):

       secret-tool lookup service navidrome user jarvis-lyrics | python3 tools/lyrics-auth.py jarvis-lyrics

   It prints `jarvis_lyrics_auth: "u=jarvis-lyrics&t=…&s=…&v=1.16.1&c=jarvis-show&f=json"`. The token
   logs in as that user, so it goes in `secrets.yaml` and nowhere else.

3. Add the command to `configuration.yaml`, with the server's own address:

   ```yaml
   rest_command:
     jarvis_lyrics:
       url: "https://music.example.org/rest/getLyricsBySongId?id={{ id }}"
       method: POST
       payload: !secret jarvis_lyrics_auth
       content_type: application/x-www-form-urlencoded
       timeout: 10
   ```

   If `rest_command:` is already there, add `jarvis_lyrics` under it.

4. Restart Home Assistant. `rest_command` is read from YAML when Home Assistant starts.

5. Check it in Developer tools → Actions: `rest_command.jarvis_lyrics` with `id:` set to a song's id
   and "Return response" on. A good answer has `status: 200` and `subsonic-response.status: ok`. An
   answer of `status: failed` with code 40 is a wrong user or password.

## Checking on the Show

Play a song from the library with words in its tags. Within a few seconds of the song starting, the
button appears; echod logs `lyrics song=… lines=… synced=true`. A failed fetch logs the reason once and
rests ten minutes.
