from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[1]
DOC = ROOT / "docs/known-bug-closure.md"


class KnownBugClosureContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.text = DOC.read_text(encoding="utf-8")

    def test_every_historical_failure_class_is_classified(self):
        for phrase in (
            "Setup browser told the user to press a physical action button",
            "no saved Wi-Fi",
            "zeroconf",
            "TWRP reboot",
            "Wrong board/newer Echo generation",
            "Old generic TECHO5 rootfs",
            "black/reserved strip",
            "warm tabs",
            "jarvis-edge-to-edge.js",
            "WIND separator",
            "Was ist ein Taco?",
            "unwanted `Sir`",
            "NTP",
        ):
            self.assertIn(phrase, self.text)

    def test_open_physical_and_external_gates_are_not_claimed_fixed(self):
        self.assertIn("OPEN GATE — J38", self.text)
        self.assertIn("OPEN GATE — J39", self.text)
        self.assertIn("J38 and J39 remain release gates", self.text)
        self.assertIn("neither may be converted", self.text)

    def test_external_owners_remain_explicit(self):
        for owner in (
            "EXTERNAL HA UI",
            "EXTERNAL HA/Klar configuration",
            "EXTERNAL NETWORK POLICY",
        ):
            self.assertIn(owner, self.text)

    def test_future_music_and_antibrick_work_is_not_hidden_as_a_closed_bug(self):
        self.assertIn("J41: final upstream anti-brick audit", self.text)
        self.assertIn("J42–J45: resilient room/group music routing", self.text)


if __name__ == "__main__":
    unittest.main()
