# Jarvis Show named Music Assistant groups

Jarvis Show does not implement a second multi-room audio engine. Music Assistant remains responsible
for queues, playback and synchronized players.

## Configuration

Home Assistant configures each Show through `music_routing` with a bounded JSON document:

```json
{
  "local_room": "living_room",
  "rooms": [
    {
      "name": "living_room",
      "primary": "media_player.living_room_primary",
      "fallback": "media_player.living_room_jarvis"
    },
    {
      "name": "kitchen",
      "primary": "media_player.kitchen_primary",
      "fallback": "media_player.kitchen_jarvis"
    }
  ],
  "groups": [
    {
      "name": "wohnung",
      "aliases": ["ganze wohnung", "überall"],
      "rooms": ["living_room", "kitchen"]
    }
  ]
}
```

Every remote room must explicitly name its Jarvis Show Music Assistant fallback. The local room may
omit `fallback`; that Show discovers its own Music Assistant player.

## Explicit group Play

For every new explicit group request Jarvis:

1. resolves the requested name or alias;
2. resolves every room independently;
3. uses its preferred Music Assistant player if that player is online;
4. otherwise substitutes the room's Jarvis Show player;
5. skips a room only when neither output is usable;
6. fails rather than silently playing nowhere when the entire group has no live output;
7. asks Home Assistant/Music Assistant to form a temporary synchronized group with
   `media_player.join`;
8. starts the requested media through `music_assistant.play_media` on the selected leader.

Membership is intentionally re-evaluated only for a new explicit Play request. Jarvis does not
continuously reshuffle an already-playing group merely because a preferred speaker reconnects.

Before a later explicit single-room or named-group Play, Jarvis unjoins only the temporary group it
previously created. It does not dismantle arbitrary Music Assistant groups owned by the user.

The resolved group name and real member entities are retained as observational playback state for
the full-screen Now Playing work in J44.
