from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[1]


class MusicFailoverRegressionContractTests(unittest.TestCase):
    def test_runtime_suite_covers_all_j45_route_transitions(self):
        source = (ROOT / "echod/internal/feature/home/music_failover_test.go").read_text(encoding="utf-8")
        for test_name in (
            "TestMusicPrimaryOfflineAtStartUsesJarvisFallback",
            "TestMusicPrimaryLossMidPlaybackTransfersAndStaysFallback",
            "TestNextExplicitPlayRestoresRecoveredPrimary",
            "TestRoomWithoutPreferredSpeakerUsesJarvisAsPrimary",
            "TestNamedGroupSubstitutesOfflineRoomAndReportsRealMembers",
        ):
            self.assertIn(test_name, source)
        self.assertIn('"music_assistant", "transfer_queue"', source)
        self.assertIn('"media_player", "media_stop"', source)
        self.assertIn('"media_player", "join"', source)

    def test_now_playing_runtime_checks_use_resolved_not_preferred_route(self):
        route_test = (ROOT / "echod/internal/feature/home/music_failover_test.go").read_text(encoding="utf-8")
        display_test = (ROOT / "echod/internal/feature/display/render_nowplaying_test.go").read_text(encoding="utf-8")
        self.assertIn("Now Playing claimed the unavailable preferred speaker", route_test)
        self.assertIn("TestMusicRouteLabelReportsTheResolvedDestination", display_test)
        self.assertIn("Playing on  Living Room Jarvis", display_test)
        self.assertIn("Playing in  Wohnung", display_test)

    def test_playback_cache_tracks_external_picture_without_compile_gap(self):
        source = (ROOT / "echod/internal/feature/home/music_route.go").read_text(encoding="utf-8")
        start = source.index("type musicPlaybackState struct")
        end = source.index("var musicPlayback", start)
        state = source[start:end]
        self.assertIn("picture", state)
        self.assertIn("musicPlayback.picture", source)

    def test_final_acceptance_lists_every_j45_scenario(self):
        doc = (ROOT / "docs/final-acceptance.md").read_text(encoding="utf-8")
        for marker in (
            "preferred speaker offline at Play start",
            "preferred speaker lost mid-playback",
            "does not pull a failed-over session back mid-song",
            "next explicit Play re-resolves",
            "room with no preferred speaker",
            "named/whole-home groups substitute",
            "Now Playing reports the real resolved endpoint/room/group",
        ):
            self.assertIn(marker, doc)


if __name__ == "__main__":
    unittest.main()
