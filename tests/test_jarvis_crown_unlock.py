import hashlib
import importlib.util
import pathlib
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[1]
TOOLS = ROOT / "tools"
sys.path.insert(0, str(TOOLS))

from jarvis_crown.device_gate import DeviceGateError, DeviceIdentity  # noqa: E402
from jarvis_crown import unlock  # noqa: E402


class UnlockTests(unittest.TestCase):
    def identity(self, unlocked=False, serial="ABC123", product="CROWN"):
        return DeviceIdentity(serial=serial, product=product, unlocked=unlocked, lk_build_desc="lk")

    def bundle(self, td):
        root = pathlib.Path(td) / "amonet-crown-v2.0.1" / "amonet"
        hashes = {}
        for rel in (
            "fastbrick.sh",
            "profile.sh",
            "device.prop",
            "bin/fastbrick.img",
            "bin/fastboot",
            "bin/fastboot32",
        ):
            path = root / rel
            path.parent.mkdir(parents=True, exist_ok=True)
            body = f"fixture:{rel}\n".encode()
            path.write_bytes(body)
            hashes[rel] = hashlib.sha256(body).hexdigest()
        return root.parent, hashes

    def test_already_unlocked_skips_amonet_without_confirmation(self):
        with tempfile.TemporaryDirectory() as td:
            amonet, hashes = self.bundle(td)
            run = mock.Mock()
            result = unlock.unlock_crown(
                self.identity(unlocked=True), amonet,
                confirmation=None, run=run, expected_hashes=hashes,
            )
            self.assertFalse(result.amonet_invoked)
            run.assert_not_called()

    def test_wrong_product_never_invokes_amonet(self):
        with tempfile.TemporaryDirectory() as td:
            amonet, hashes = self.bundle(td)
            run = mock.Mock()
            with self.assertRaisesRegex(unlock.UnlockError, "unsupported product"):
                unlock.unlock_crown(
                    self.identity(product="CHECKERS"), amonet,
                    confirmation=unlock.CONFIRM_PHRASE, run=run, expected_hashes=hashes,
                )
            run.assert_not_called()

    def test_locked_device_without_lk_build_is_refused_before_amonet(self):
        with tempfile.TemporaryDirectory() as td:
            amonet, hashes = self.bundle(td)
            run = mock.Mock()
            identity = DeviceIdentity(serial="ABC123", product="CROWN", unlocked=False, lk_build_desc=None)
            with self.assertRaisesRegex(unlock.UnlockError, "no readable lk_build_desc"):
                unlock.unlock_crown(
                    identity, amonet,
                    confirmation=unlock.CONFIRM_PHRASE, run=run, expected_hashes=hashes,
                )
            run.assert_not_called()

    def test_hash_mismatch_blocks_execution(self):
        with tempfile.TemporaryDirectory() as td:
            amonet, hashes = self.bundle(td)
            (amonet / "amonet" / "bin" / "fastbrick.img").write_bytes(b"tampered")
            run = mock.Mock()
            with self.assertRaisesRegex(unlock.UnlockError, "hash mismatch"):
                unlock.unlock_crown(
                    self.identity(), amonet,
                    confirmation=unlock.CONFIRM_PHRASE, run=run, expected_hashes=hashes,
                )
            run.assert_not_called()

    def test_locked_crown_requires_exact_confirmation(self):
        with tempfile.TemporaryDirectory() as td:
            amonet, hashes = self.bundle(td)
            run = mock.Mock()
            with self.assertRaisesRegex(unlock.UnlockError, "exact confirmation"):
                unlock.unlock_crown(
                    self.identity(), amonet,
                    confirmation="yes", run=run, expected_hashes=hashes,
                )
            run.assert_not_called()

    def test_success_runs_known_script_and_reverifies_unlock(self):
        with tempfile.TemporaryDirectory() as td:
            amonet, hashes = self.bundle(td)
            run = mock.Mock(return_value=subprocess.CompletedProcess(["bash", "fastbrick.sh"], 0))
            states = iter([
                DeviceGateError("temporarily disconnected"),
                self.identity(unlocked=False),
                self.identity(unlocked=True),
            ])
            def identify():
                value = next(states)
                if isinstance(value, Exception):
                    raise value
                return value
            result = unlock.unlock_crown(
                self.identity(), amonet,
                confirmation=unlock.CONFIRM_PHRASE,
                run=run,
                identify=identify,
                sleep=lambda _: None,
                expected_hashes=hashes,
            )
            self.assertTrue(result.amonet_invoked)
            self.assertTrue(result.identity.unlocked)
            args, kwargs = run.call_args
            self.assertEqual(args[0], ["bash", "fastbrick.sh"])
            self.assertEqual(kwargs["cwd"], amonet / "amonet")
            self.assertEqual(kwargs["input"], "YES\nx\n")

    def test_success_exit_without_unlock_confirmation_fails(self):
        with tempfile.TemporaryDirectory() as td:
            amonet, hashes = self.bundle(td)
            run = mock.Mock(return_value=subprocess.CompletedProcess(["bash", "fastbrick.sh"], 0))
            with self.assertRaisesRegex(unlock.UnlockError, "could not be confirmed"):
                unlock.unlock_crown(
                    self.identity(), amonet,
                    confirmation=unlock.CONFIRM_PHRASE,
                    run=run,
                    identify=lambda: self.identity(unlocked=False),
                    sleep=lambda _: None,
                    expected_hashes=hashes,
                )

    def test_serial_change_after_exploit_fails_closed(self):
        with tempfile.TemporaryDirectory() as td:
            amonet, hashes = self.bundle(td)
            run = mock.Mock(return_value=subprocess.CompletedProcess(["bash", "fastbrick.sh"], 0))
            with self.assertRaisesRegex(unlock.UnlockError, "serial changed"):
                unlock.unlock_crown(
                    self.identity(), amonet,
                    confirmation=unlock.CONFIRM_PHRASE,
                    run=run,
                    identify=lambda: self.identity(unlocked=True, serial="OTHER"),
                    sleep=lambda _: None,
                    expected_hashes=hashes,
                )

    def test_amonet_failure_requires_reidentify_before_retry(self):
        with tempfile.TemporaryDirectory() as td:
            amonet, hashes = self.bundle(td)
            run = mock.Mock(return_value=subprocess.CompletedProcess(["bash", "fastbrick.sh"], 3))
            with self.assertRaisesRegex(unlock.UnlockError, "re-identified"):
                unlock.unlock_crown(
                    self.identity(), amonet,
                    confirmation=unlock.CONFIRM_PHRASE,
                    run=run,
                    expected_hashes=hashes,
                )


if __name__ == "__main__":
    unittest.main()
