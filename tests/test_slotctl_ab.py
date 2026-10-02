import io
import os
import pathlib
import subprocess
import tarfile
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
SLOTCTL = ROOT / "tools" / "linux" / "slotctl"
BOOT = ROOT / "tools" / "linux" / "rootfs" / "etc" / "techo5" / "boot.sh"
LAYOUT = ROOT / "echod" / "internal" / "layout" / "device_cronos.go"


def rootfs_tar(path: pathlib.Path, version: str) -> pathlib.Path:
    out = path / f"fixture-{version}.tar.gz"
    members = {
        "sbin/init": b"#!/bin/sh\n",
        "usr/local/bin/techo5": b"fixture\n",
        "etc/techo5/boot.sh": b"#!/bin/sh\n",
        "etc/inittab": b"::sysinit:/etc/techo5/boot.sh\n",
        "etc/techo5-release": (version + "\n").encode(),
    }
    with tarfile.open(out, "w:gz") as tf:
        for name, body in members.items():
            info = tarfile.TarInfo(name)
            info.mode = 0o755 if name in {"sbin/init", "usr/local/bin/techo5", "etc/techo5/boot.sh"} else 0o644
            info.size = len(body)
            tf.addfile(info, io.BytesIO(body))
    return out


class SlotABTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = pathlib.Path(self.tmp.name)
        self.store = self.base / "store"
        self.slot_file = self.base / "booted-slot"
        (self.store / "slots" / "a").mkdir(parents=True)
        (self.store / ".techo5-store").write_text("")
        (self.store / "active").write_text("a\n")
        (self.store / "slots" / "a.state").write_text("good\n")
        (self.store / "slots" / "a" / "etc").mkdir(parents=True)
        (self.store / "slots" / "a" / "etc" / "techo5-release").write_text("v1.0.0\n")
        self.slot_file.write_text("a\n")
        self.sentinel = self.base / "userdata-state.json"
        self.sentinel.write_text('{"keep":"me"}\n')

    def run_slot(self, *args, check=True):
        env = os.environ.copy()
        env["STORE"] = str(self.store)
        env["SLOT_FILE"] = str(self.slot_file)
        return subprocess.run(
            [str(SLOTCTL), *args], env=env, text=True,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=check,
        )

    def test_install_is_inactive_trial_then_commit(self):
        tar = rootfs_tar(self.base, "v1.1.0")
        self.run_slot("install", str(tar))
        self.assertEqual((self.store / "active").read_text().strip(), "b")
        self.assertEqual((self.store / "slots" / "b.state").read_text().strip(), "trial 3")
        self.assertEqual((self.store / "slots" / "b" / "etc" / "techo5-release").read_text().strip(), "v1.1.0")
        self.slot_file.write_text("b\n")
        self.run_slot("commit")
        self.assertEqual((self.store / "slots" / "b.state").read_text().strip(), "good")
        self.assertEqual(self.sentinel.read_text(), '{"keep":"me"}\n')

    def test_failed_trial_consumes_three_boots_then_falls_back(self):
        tar = rootfs_tar(self.base, "v1.1.0")
        self.run_slot("install", str(tar))
        self.assertEqual((self.store / "active").read_text().strip(), "b")
        expected = ["trial 2", "trial 1", "trial 0"]
        for state in expected:
            result = self.run_slot("next")
            self.assertEqual(result.stdout.strip(), "b")
            self.assertEqual((self.store / "slots" / "b.state").read_text().strip(), state)
        result = self.run_slot("next")
        self.assertEqual(result.stdout.strip(), "a")
        self.assertEqual((self.store / "slots" / "b.state").read_text().strip(), "bad")
        self.assertEqual((self.store / "active").read_text().strip(), "a")

    def test_manual_rollback_marks_trial_bad_and_selects_good_peer(self):
        tar = rootfs_tar(self.base, "v1.1.0")
        self.run_slot("install", str(tar))
        self.slot_file.write_text("b\n")
        self.run_slot("rollback")
        self.assertEqual((self.store / "slots" / "b.state").read_text().strip(), "bad")
        self.assertEqual((self.store / "active").read_text().strip(), "a")

    def test_never_overwrites_running_slot(self):
        # Active a means install targets b; claim b is the currently running root and require refusal.
        self.slot_file.write_text("b\n")
        tar = rootfs_tar(self.base, "v1.1.0")
        result = self.run_slot("install", str(tar), check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("running root; refusing to overwrite it", result.stderr)
        self.assertFalse((self.store / "slots" / "b.state").exists())

    def test_boot_health_contract_commits_only_after_settle_window(self):
        source = BOOT.read_text(encoding="utf-8")
        self.assertIn("TRIAL_SETTLE=${TRIAL_SETTLE:-300}", source)
        self.assertIn("TRIAL_TIMEOUT=${TRIAL_TIMEOUT:-900}", source)
        self.assertIn("slotctl commit", source)
        self.assertIn("daemon did not settle", source)

    def test_persistent_product_state_lives_on_userdata_not_a_slot(self):
        source = LAYOUT.read_text(encoding="utf-8")
        self.assertIn('StateDir   = "/data/misc/techo5"', source)
        slot_source = SLOTCTL.read_text(encoding="utf-8")
        self.assertNotIn("/data/misc/techo5", slot_source)


if __name__ == "__main__":
    unittest.main()
