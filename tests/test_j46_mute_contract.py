from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]


def text(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


class J46MuteContractTests(unittest.TestCase):
    def test_remote_unmute_permission_defaults_off_and_is_persisted_locally(self):
        cfg = text("echod/internal/config/microphone.go")
        setup = text("echod/internal/feature/setup/page.go")

        self.assertIn('AllowRemoteUnmute bool `json:"allow_remote_unmute,omitempty"`', cfg)
        self.assertIn("func (w MicrophoneWriter) AllowRemoteUnmute(v bool) error", cfg)
        self.assertIn('hidden(w, token, "remote-unmute", "privacy")', setup)
        self.assertIn('case "remote-unmute":', setup)
        self.assertIn("Allow trusted Home Assistant to clear its software microphone mute", setup)
        self.assertNotIn('ObjectID: "allow_remote_unmute"', text("echod/internal/feature/mute/mute.go"))

    def test_ha_mute_is_immediate_software_cut_and_unmute_is_fail_closed(self):
        mute = text("echod/internal/feature/mute/mute.go")
        mic = text("echod/internal/hardware/mic/mic.go")

        self.assertIn("func (s *Source) SetSoftwareMuted(muted bool)", mic)
        self.assertRegex(
            mic,
            r"if privacy\.SoftwareCut\(\) \|\| s\.softwareMuted\.Load\(\) \|\| s\.transitionMuted\.Load\(\) \{\s*clear\(raw\)",
        )
        self.assertIn("if !muted && !canRemoteUnmute", mute)
        self.assertIn("security.APIEncrypted()", mute)
        self.assertIn("func canRemoteUnmute(allowed, encrypted bool) bool { return allowed && encrypted }", mute)

        set_body = re.search(r"func \(m \*Mute\) Set\(muted bool\) \{(.*?)\n\}", mute, re.S)
        self.assertIsNotNone(set_body)
        self.assertNotIn("m.line.Set", set_body.group(1))

    def test_physical_latch_always_wins_and_local_mute_cuts_before_latch_settles(self):
        mute = text("echod/internal/feature/mute/mute.go")
        privacy = text("echod/internal/hardware/privacy/platform_cronos.go")

        self.assertIn("return physical || software, nil", mute)
        self.assertIn("physical, err := m.line.Get()", mute)
        self.assertIn("mic.Get().SetTransitionMuted(true)", mute)
        self.assertIn("mic.Get().SetTransitionMuted(false)", mute)
        self.assertIn("ErrButtonOnly", privacy)
        self.assertIn("only the physical button releases them", privacy)

    def test_ha_reports_all_j46_mute_states(self):
        mute = text("echod/internal/feature/mute/mute.go")
        tests = text("echod/internal/feature/mute/remote_test.go")

        self.assertIn('ObjectID: "microphone_mute_status"', mute)
        for state in (
            "muted",
            "unmuted",
            "physical mute active",
            "remote unmute not permitted",
        ):
            self.assertIn(f'"{state}"', mute)
            self.assertIn(f'"{state}"', tests)

    def test_touchscreen_has_local_software_unmute_without_weakening_hardware_latch(self):
        mute = text("echod/internal/feature/mute/mute.go")
        sheet = text("echod/internal/feature/display/sheet.go")
        spot = text("echod/internal/feature/display/display_spot.go")

        self.assertIn("func (m *Mute) LocalToggle()", mute)
        self.assertIn("m.setSoftware(!mic.Get().SoftwareMuted())", mute)
        self.assertIn("if m.physical.Load()", mute)
        self.assertIn("mute.Get().LocalToggle()", sheet)
        self.assertIn("mute.Get().LocalToggle()", spot)

    def test_runtime_security_gate_includes_mute_package(self):
        gate = text("tools/security-regression.sh")
        self.assertIn("./internal/feature/mute", gate)


if __name__ == "__main__":
    unittest.main()
