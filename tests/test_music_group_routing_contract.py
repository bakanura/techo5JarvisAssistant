from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[1]


class MusicGroupRoutingContractTests(unittest.TestCase):
    def test_named_group_configuration_is_persisted_and_bounded(self):
        config = (ROOT / "echod/internal/config/home.go").read_text(encoding="utf-8")
        group = (ROOT / "echod/internal/feature/home/music_group.go").read_text(encoding="utf-8")
        self.assertIn('MusicRoom string `json:"music_room,omitempty"`', config)
        self.assertIn('MusicRooms  []MusicRoomRoute `json:"music_rooms,omitempty"`', config)
        self.assertIn('MusicGroups []MusicGroup     `json:"music_groups,omitempty"`', config)
        self.assertIn("maxMusicRooms", group)
        self.assertIn("maxMusicGroups", group)
        self.assertIn("remote room %q needs a Jarvis fallback", group)

    def test_each_group_room_resolves_primary_then_jarvis_fallback(self):
        source = (ROOT / "echod/internal/feature/home/music_group.go").read_text(encoding="utf-8")
        self.assertIn("if musicEntityUsable(r.Primary)", source)
        self.assertIn("fallback := strings.TrimSpace(r.Fallback)", source)
        self.assertIn("fallback = localPlayer", source)
        self.assertIn("if musicEntityUsable(fallback)", source)
        self.assertIn("music group %q has no available outputs", source)

    def test_group_is_music_assistant_sync_not_local_group_engine(self):
        source = (ROOT / "echod/internal/feature/home/music_group.go").read_text(encoding="utf-8")
        self.assertIn('hass.Get().Call("media_player", "join"', source)
        self.assertIn('"group_members": outputs[1:]', source)
        self.assertIn('hass.Get().Call("music_assistant", "play_media"', source)
        self.assertNotIn("sendspin", source.lower())

    def test_membership_is_re_resolved_on_each_explicit_group_play(self):
        source = (ROOT / "echod/internal/feature/home/music_group.go").read_text(encoding="utf-8")
        play = source[source.index("func (f *Feature) PlayMusicGroup"):]
        self.assertIn("resolveMusicGroup(groupName)", play)
        self.assertIn("releaseActiveMusicGroup()", play)
        self.assertIn('hass.Get().Call("media_player", "unjoin"', source)

    def test_ha_and_direct_brain_share_group_resolver(self):
        home = (ROOT / "echod/internal/feature/home/home.go").read_text(encoding="utf-8")
        assistant = (ROOT / "echod/internal/feature/assistant/tools.go").read_text(encoding="utf-8")
        self.assertIn('Name: "music_routing"', home)
        self.assertIn('Name: "music_play_group"', home)
        self.assertIn("f.PlayMusicGroup(c.String(\"group\"), c.String(\"media_id\"))", home)
        self.assertIn('"group":    str(', assistant)
        self.assertIn("home.Get().PlayMusicGroup(group, id)", assistant)

    def test_group_route_is_exposed_for_fullscreen_now_playing(self):
        source = (ROOT / "echod/internal/feature/home/music_route.go").read_text(encoding="utf-8")
        self.assertIn("activeGroup", source)
        self.assertIn("activeMembers", source)
        self.assertIn("func (f *Feature) MusicGroupOutput()", source)


if __name__ == "__main__":
    unittest.main()
