# Jarvis Show full-screen Music Assistant surface

Music Assistant playback owns a native full-screen surface for the complete active or paused session.
Unlike legacy TECHO5 radio playback, MA playback never times itself down into the music strip and a
swipe cannot dismiss it while the queue still exists. Alarm/timer ringing, calls, settings, camera and
other privacy/attention-critical pages keep their existing higher display priority.

The page combines two sources deliberately:

- Sendspin supplies low-latency title/artist/album and artwork when this Show is an output.
- The resolved HA/Music Assistant player supplies route/group identity, state, best-effort progress and
  queue context. When the preferred room speaker is external, its HA metadata and entity picture keep
  the Show useful even though no Sendspin audio is arriving locally.

The page shows:

- artwork (with the existing bounded image decoder);
- title, artist and album;
- the actual room/output or named group and the live resolved room members;
- elapsed/total progress when the MA player exposes it;
- the next queue item when `music_assistant.get_queue` is available;
- previous, play/pause, next and Stop/Done controls;
- Music Assistant's existing favorite-current-song control.

Transport, Stop and Favorite target the real routed Music Assistant entity rather than blindly
controlling this Show's local player. This is required when J42 selected an external primary speaker.

Home Assistant documents `music_assistant.get_queue` as a response-producing action and its REST API
supports `?return_response`; queue lookup is therefore an optional background enhancement. A failure
there never interrupts playback or removes the page.

Jarvis does not invent an "add this track to album" mutation. Music Assistant exposes library/query
operations and favorites through HA, but not a stable album-mutation action suitable for a blind touch
button. The page keeps the safe Favorite operation and album metadata instead of guessing at a library
write.
