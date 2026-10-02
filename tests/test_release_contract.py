import pathlib
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]


class ReleaseContractTests(unittest.TestCase):
    def test_official_release_namespace_is_consistent(self):
        expected = "bakanura/techo5JarvisAssistant"
        for rel in (
            "tools/release-jarvis-show.sh",
            "tools/install-show.py",
            "tools/linux/deploy-rootfs.sh",
        ):
            self.assertIn(expected, (ROOT / rel).read_text(encoding="utf-8"), rel)
        self.assertIn(
            "https://github.com/bakanura/techo5JarvisAssistant/releases",
            (ROOT / "echod/internal/update/releases_cronos.go").read_text(encoding="utf-8"),
        )

    def test_rootfs_builder_emits_shared_product_marker(self):
        source = (ROOT / "tools/linux/mkrootfs.sh").read_text(encoding="utf-8")
        self.assertIn('jarvis-show-release.json', source)
        self.assertIn('"product": "jarvis-show-v1"', source)
        self.assertIn('"boards": ["crown", "checkers"]', source)
        self.assertIn('"version": "$VERSION"', source)

    def test_release_workflow_uses_secret_and_attests_payloads(self):
        source = (ROOT / ".github/workflows/release.yml").read_text(encoding="utf-8")
        self.assertIn("secrets.JARVIS_SHOW_SIGN_KEY", source)
        self.assertIn("actions/attest-build-provenance", source)
        self.assertIn("jarvis-show-rootfs-${{ steps.meta.outputs.version }}.tar.gz", source)
        self.assertIn("tools/release-jarvis-show.sh", source)

    def test_inherited_generic_publisher_is_disabled(self):
        source = (ROOT / "tools/release.ps1").read_text(encoding="utf-8")
        self.assertIn("inherited TECHO5 publisher is disabled in Jarvis Show", source)

    def test_release_tool_does_not_build_rootfs(self):
        source = (ROOT / "tools/release-jarvis-show.sh").read_text(encoding="utf-8")
        self.assertIn("This script NEVER builds a rootfs or boot image", source)
        self.assertNotIn("deploy-rootfs.sh", source)
        self.assertIn("mkmanifest", source)


if __name__ == "__main__":
    unittest.main()
