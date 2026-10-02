import tempfile
import unittest
import zipfile
from pathlib import Path
import sys
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import jarvis_crown  # noqa: E402


class AmonetBundleTests(unittest.TestCase):
    def make_bundle(self, device="crown"):
        root = Path(self.temp.name) / "amonet"
        for name in jarvis_crown.AMONET_REQUIRED:
            path = root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(b"x")
        (root / "device.prop").write_text(f"DEVICE={device}\n", encoding="utf-8")
        return root

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp.cleanup()

    def test_crown_bundle_passes(self):
        result = jarvis_crown.check_amonet_bundle(self.make_bundle())
        self.assertFalse(any(item.level == "FAIL" for item in result))

    def test_wrong_board_bundle_fails_closed(self):
        result = jarvis_crown.check_amonet_bundle(self.make_bundle("cronos"))
        self.assertTrue(any(item.level == "FAIL" for item in result))

    def test_missing_payload_fails(self):
        root = self.make_bundle()
        (root / "bin" / "fastbrick.img").unlink()
        result = jarvis_crown.check_amonet_bundle(root)
        self.assertTrue(any(item.level == "FAIL" for item in result))


class LineageTests(unittest.TestCase):
    def make_zip(self, board):
        path = Path(self.temp.name) / f"lineage-{board}.zip"
        with zipfile.ZipFile(path, "w") as archive:
            archive.writestr("META-INF/com/android/metadata", f"pre-device={board}\n")
        return path

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.temp.cleanup()

    def test_crown_lineage_passes(self):
        result = jarvis_crown.check_lineage_zip(self.make_zip("crown"))
        self.assertFalse(any(item.level == "FAIL" for item in result))

    def test_wrong_lineage_board_fails_closed(self):
        result = jarvis_crown.check_lineage_zip(self.make_zip("checkers"))
        self.assertTrue(any(item.level == "FAIL" for item in result))


class HostToolTests(unittest.TestCase):
    def test_missing_fastboot_fails(self):
        def fake_which(name):
            return None if name == "fastboot" else f"/usr/bin/{name}"

        with mock.patch.object(jarvis_crown.shutil, "which", side_effect=fake_which):
            result = jarvis_crown.check_tools()
        self.assertTrue(any(item.level == "FAIL" and "fastboot" in item.message for item in result))

    def test_low_disk_space_fails(self):
        usage = shutil_usage = mock.Mock(free=jarvis_crown.MIN_FREE_BYTES - 1)
        # disk_usage also exposes total/used in real life; the preflight reads only free.
        with tempfile.TemporaryDirectory() as directory, \
                mock.patch.object(jarvis_crown.shutil, "disk_usage", return_value=usage):
            result = jarvis_crown.check_disk((Path(directory),))
        self.assertTrue(any(item.level == "FAIL" and "8 GiB" in item.message for item in result))


if __name__ == "__main__":
    unittest.main()
