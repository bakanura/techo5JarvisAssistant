import importlib.util
import pathlib
import sys
import tempfile
import unittest
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE = ROOT / "tools" / "jarvis_crown" / "preflight.py"
spec = importlib.util.spec_from_file_location("jarvis_crown_preflight", MODULE)
preflight = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = preflight
spec.loader.exec_module(preflight)


class PreflightTests(unittest.TestCase):
    def base_paths(self, td):
        td = pathlib.Path(td)
        repo = td / "repo"
        amonet = td / "amonet-crown-v2.0.1"
        repo.mkdir()
        (amonet / "amonet").mkdir(parents=True)
        (amonet / "amonet" / "bootrom-step.sh").write_text("#!/bin/sh\n", encoding="utf-8")
        return repo, amonet, td / "work", td / "backups"

    def test_missing_fastboot_fails_without_device_io(self):
        with tempfile.TemporaryDirectory() as td:
            repo, amonet, work, backups = self.base_paths(td)
            def which(name):
                if name == "adb":
                    return "/usr/bin/adb"
                if name == "fastboot":
                    return None
                return None
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
            missing = pathlib.Path(td) / "missing-amonet"
            with mock.patch.object(preflight.sys, "platform", "linux"), \
                 mock.patch.object(preflight.sys, "version_info", (3, 13, 0)), \
                 mock.patch.object(preflight.os, "geteuid", return_value=0), \
                 mock.patch.object(preflight.shutil, "which", return_value="/usr/bin/tool"), \
                 mock.patch.object(preflight.shutil, "disk_usage", return_value=mock.Mock(free=20 * 1024**3)):
                checks = preflight.run_preflight(repo_root=repo, amonet_dir=missing, work_dir=work, backup_dir=backups)
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
            self.assertTrue(any(c.name == "disk-space" and c.level == "FAIL" for c in checks))


if __name__ == "__main__":
    unittest.main()
