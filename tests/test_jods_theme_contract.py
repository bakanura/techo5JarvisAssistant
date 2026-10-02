import pathlib
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]


class JodsThemeContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.palette = (ROOT / "echod/internal/lib/palette/palette.go").read_text(encoding="utf-8")
        cls.screen = (ROOT / "echod/internal/config/screen.go").read_text(encoding="utf-8")
        cls.setup = (ROOT / "echod/internal/feature/setup/tabs.go").read_text(encoding="utf-8")
        cls.jobs = (ROOT.parent / "JOBS.md").read_text(encoding="utf-8")
        cls.acceptance = (ROOT / "docs/final-acceptance.md").read_text(encoding="utf-8")

    def test_fresh_jarvis_defaults_to_white_jade(self):
        self.assertIn('const DefaultTheme = "White Jade"', self.screen)
        first = self.palette.index('{"White Jade", 0xeeeff2, 0x86909d, 0x20242a, 0x7c8794, 0xd8dade}')
        self.assertLess(first, self.palette.index('{"Walnut"'))

    def test_canonical_jods_palette_family_is_available(self):
        for row in (
            '{"White Jade", 0xeeeff2, 0x86909d, 0x20242a, 0x7c8794, 0xd8dade}',
            '{"Leaf Jade", 0xedf6f1, 0x3d9a61, 0x1f3127, 0x698474, 0xd2e1d8}',
            '{"Sakura Jade", 0xf8edf3, 0xd66f99, 0x302128, 0x8b7480, 0xe5d9df}',
            '{"Ember Jade", 0xf7ece7, 0xea9468, 0x34180f, 0x8d5a48, 0xead9d2}',
        ):
            self.assertIn(row, self.palette)

    def test_setup_surface_uses_jods_shape_and_layering(self):
        self.assertIn("<title>Jarvis Show setup</title>", self.setup)
        for token in (
            '--font-ui:"Segoe UI","Noto Sans",system-ui,sans-serif',
            '--card-radius:18px',
            '--shadow:0 18px 38px',
            'radial-gradient(circle at top right',
            'backdrop-filter:blur(12px)',
            'border-radius:999px',
            'var(--accent-soft)',
        ):
            self.assertIn(token, self.setup)

    def test_live_display_and_klar_checks_are_post_install_e2e(self):
        self.assertIn("- [x] **J47 — JODS visual-system port / White Jade default**", self.jobs)
        phase = self.jobs.index("## Phase K — Final post-install real-device E2E acceptance")
        self.assertGreater(self.jobs.index("J38 — Installed-firmware display E2E acceptance"), phase)
        self.assertGreater(self.jobs.index("J39 — Installed-firmware HA/Klar voice/web E2E acceptance"), phase)
        gate_c = self.acceptance.index("## Gate C — first real device per board")
        self.assertGreater(self.acceptance.index("J38 installed-firmware display E2E"), gate_c)
        self.assertGreater(self.acceptance.index("J39 installed-firmware HA/Klar E2E"), gate_c)
        gate_a = self.acceptance[:gate_c]
        self.assertNotIn("tools/live-acceptance.py verify", gate_a)


if __name__ == "__main__":
    unittest.main()
