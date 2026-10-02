import importlib.util
import pathlib
import sys
import tempfile
import unittest
from unittest import mock
import zipfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE = ROOT / "tools" / "jarvis_crown" / "preflight.py"
spec = importlib.util.spec_from_file_location("jarvis_crown_preflight", MODULE)
preflight = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = preflight
spec.loader.exec_module(preflight)


class PreflightTests(unittest.TestCase):
    def base_paths(self, td, device="crown"):
        td = pathlib.Path(td)
        repo = td / "repo"
        amonet = td / "amonet-crown-v2.0.1"
        repo.mkdir()
        for name in preflight.AMONET_REQUIRED:
            path = amonet / "amonet" / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(b"x")
        (amonet / "amonet" / "device.prop").write_text(f"DEVICE={device}\n", encoding="utf-8")
        return repo, amonet, td / "work", td / "backups"

    def run_ready(self, repo, amonet, work, backups, **kwargs):
        def which(name):
            return f"/usr/bin/{name}"
        with mock.patch.object(preflight.sys, "platform", "linux"), \
             mock.patch.object(preflight.sys, "version_info", (3, 13, 0)), \
             mock.patch.object(preflight.os, "geteuid", return_value=0), \
             mock.patch.object(preflight.shutil, "which", side_effect=which), \
             mock.patch.object(preflight, "_modemmanager_active", return_value=False), \
             mock.patch.object(preflight.shutil, "disk_usage", return_value=mock.Mock(free=20 * 1024**3)):
            return preflight.run_preflight(
                repo_root=repo,
                amonet_dir=amonet,
                work_dir=work,
                backup_dir=backups,
                **kwargs,
            )

    def make_lineage(self, td, board):
        path = pathlib.Path(td) / f"lineage-{board}.zip"
        with zipfile.ZipFile(path, "w") as archive:
            archive.writestr("META-INF/com/android/metadata", f"pre-device={board}\n")
        return path

    def test_missing_fastboot_fails_without_device_io(self):
        with tempfile.TemporaryDirectory() as td:
            repo, amonet, work, backups = self.base_paths(td)
            def which(name):
                return None if name == "fastboot" else f"/usr/bin/{name}"
            with mock.patch.object(preflight.sys, "platform", "linux"), \
                 mock.patch.object(preflight.sys, "version_info", (3, 13, 0)), \
                 mock.patch.object(preflight.os, "geteuid", return_value=0), \
                 mock.patch.object(preflight.shutil, "which", side_effect=which), \
                 mock.patch.object(preflight.shutil, "disk_usage", return_value=mock.Mock(free=20 * 1024**3)):
                checks = preflight.run_preflight(repo_root=repo, amonet_dir=amonet, work_dir=work, backup_dir=backups)
            self.assertFalse(preflight.preflight_ok(checks))
            self.assertTrue(any(c.name == "tool-fastboot" and c.level == "FAIL" for c in checks))

    def test_missing_amonet_fails_closed(self):
        with tempfile.TemporaryDirectory() as td:
            repo, _, work, backups = self.base_paths(td)
            checks = self.run_ready(repo, pathlib.Path(td) / "missing-amonet", work, backups)
            self.assertFalse(preflight.preflight_ok(checks))
            self.assertTrue(any(c.name == "amonet" and c.level == "FAIL" for c in checks))

    def test_wrong_amonet_board_fails_closed(self):
        with tempfile.TemporaryDirectory() as td:
            repo, amonet, work, backups = self.base_paths(td, device="cronos")
            checks = self.run_ready(repo, amonet, work, backups)
            self.assertFalse(preflight.preflight_ok(checks))
            self.assertTrue(any(c.name == "amonet" and c.level == "FAIL" for c in checks))

    def test_low_disk_space_fails(self):
        with tempfile.TemporaryDirectory() as td:
            repo, amonet, work, backups = self.base_paths(td)
            with mock.patch.object(preflight.sys, "platform", "linux"), \
                 mock.patch.object(preflight.sys, "version_info", (3, 13, 0)), \
                 mock.patch.object(preflight.os, "geteuid", return_value=0), \
                 mock.patch.object(preflight.shutil, "which", return_value="/usr/bin/tool"), \
                 mock.patch.object(preflight.shutil, "disk_usage", return_value=mock.Mock(free=2 * 1024**3)):
                checks = preflight.run_preflight(repo_root=repo, amonet_dir=amonet, work_dir=work, backup_dir=backups)
            self.assertFalse(preflight.preflight_ok(checks))
            self.assertTrue(any(c.name.startswith("disk-space") and c.level == "FAIL" for c in checks))

    def test_crown_lineage_metadata_passes(self):
        with tempfile.TemporaryDirectory() as td:
            repo, amonet, work, backups = self.base_paths(td)
            lineage = self.make_lineage(td, "crown")
            checks = self.run_ready(repo, amonet, work, backups, lineage_zip=lineage)
            self.assertTrue(any(c.name == "lineage" and c.level == "PASS" for c in checks))

    def test_wrong_lineage_board_fails_closed(self):
        with tempfile.TemporaryDirectory() as td:
            repo, amonet, work, backups = self.base_paths(td)
            lineage = self.make_lineage(td, "checkers")
            checks = self.run_ready(repo, amonet, work, backups, lineage_zip=lineage)
            self.assertFalse(preflight.preflight_ok(checks))
            self.assertTrue(any(c.name == "lineage" and c.level == "FAIL" for c in checks))

    def test_complete_crown_host_inputs_can_pass(self):
        with tempfile.TemporaryDirectory() as td:
            repo, amonet, work, backups = self.base_paths(td)
            lineage = self.make_lineage(td, "crown")
            checks = self.run_ready(repo, amonet, work, backups, lineage_zip=lineage)
            self.assertTrue(preflight.preflight_ok(checks))


if __name__ == "__main__":
    unittest.main()
