# Jarvis Show music routing

Jarvis Show does not implement a second music/grouping engine. Music Assistant remains the owner of queues, synchronization and groups. Jarvis Show only decides which Music Assistant player represents this room at the start of a new request and provides one automatic failure direction.

## Room policy

Each Show can store one preferred Music Assistant `media_player` entity with the ESPHome action `music_primary_player`. Empty means the Show itself is the room's primary output.

For a new explicit Play request:

1. If the configured preferred Music Assistant player is online, use it.
2. Otherwise use this Show's own Music Assistant player.
3. HA/Klar can invoke the device action `music_play`; Direct Brain uses its `play_music` tool. Both call the same resolver.

`unavailable`, `unknown`, an unreadable player state, or an entity that is not a Music Assistant player are not considered a usable preferred output.

## Failure during active playback

The daemon watches only for the preferred room player disappearing while it was actively playing. In that case it asks Home Assistant/Music Assistant to run `music_assistant.transfer_queue` from the preferred player to the local Jarvis Show player with autoplay.

That failover is sticky. If the preferred speaker returns five seconds later, Jarvis does not yank the currently playing queue back mid-song.

On the next explicit Play request the room is resolved again. If the preferred speaker is healthy again, Jarvis stops the old local fallback first and starts the new request on the preferred player. If the preferred speaker is still unavailable, the new request stays local.

This intentionally mirrors the desired appliance behavior: keep existing music stable, but make every new user command choose the best currently available room output.

## Grouping boundary

Named groups and whole-home routing are J43. Group synchronization remains Music Assistant/Sendspin; Jarvis Show will resolve which player each room contributes to the group rather than inventing its own synchronized playback protocol.
