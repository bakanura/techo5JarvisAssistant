import importlib.util
import pathlib
import sys
import tempfile
import unittest
from unittest import mock
import zipfile

ROOT = pathlib.Path(__file__).resolve().parents[1]
TOOLS = ROOT / "tools"
sys.path.insert(0, str(TOOLS))
SPEC = importlib.util.spec_from_file_location("jarvis_install_show", TOOLS / "install-show.py")
install = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(install)


class FakeAdb:
    def __init__(self):
        self.commands = []
        self.pushed = []
        self.state_value = "recovery"
        self.data_writable = True
        self.remote_hash = None
        self.install_output = "Install succeeded"
        self.driver_output = (
            f"wifi=vermagic={install.KERNEL_RELEASE}\n"
            f"bt=vermagic={install.KERNEL_RELEASE}"
        )

    def state(self):
        return self.state_value

    def sh(self, command):
        self.commands.append(command)
        if command == "twrp format data":
            return "Done."
        if command == "mount":
            return "/dev/block/foo on /data type ext4 (rw)" if self.data_writable else ""
        if command.startswith("touch /data/.jarvis-crown-write-test"):
            return "OK" if self.data_writable else ""
        if command == "twrp reboot recovery":
            return ""
        if command.startswith("sha256sum /data/lineage.zip"):
            return (self.remote_hash or "") + "  /data/lineage.zip"
        if command == "twrp install /data/lineage.zip":
            return self.install_output
        if command == "rm -f /data/lineage.zip":
            return ""
        if command.startswith("mkdir -p /tmp/t5sys"):
            return self.driver_output
        raise AssertionError(command)

    def push(self, local, remote):
        self.pushed.append((local, remote))
        import hashlib
        self.remote_hash = hashlib.sha256(pathlib.Path(local).read_bytes()).hexdigest()


class LineageStagingTests(unittest.TestCase):
    def make_zip(self, td, board="crown"):
        path = pathlib.Path(td) / "lineage.zip"
        with zipfile.ZipFile(path, "w") as archive:
            archive.writestr("META-INF/com/android/metadata", f"pre-device={board}\n")
        return path

    def test_crown_zip_metadata_is_detected(self):
        with tempfile.TemporaryDirectory() as td:
            self.assertEqual(install.lineage_board(self.make_zip(td)), "crown")

    def test_wrong_board_is_visible_before_any_device_write(self):
        with tempfile.TemporaryDirectory() as td:
            self.assertEqual(install.lineage_board(self.make_zip(td, "checkers")), "checkers")

    def test_format_uses_twrp_reboot_and_waits_for_writable_data(self):
        with tempfile.TemporaryDirectory() as td:
            adb = FakeAdb()
            zip_path = self.make_zip(td)
            original_wait = install.wait_for
            install.wait_for = lambda label, timeout, predicate, interval: self.assertTrue(predicate())
            try:
                install.install_lineage(adb, str(zip_path))
            finally:
                install.wait_for = original_wait
            self.assertIn("twrp reboot recovery", adb.commands)
            self.assertFalse(any(c.startswith("adb reboot") for c in adb.commands))
            self.assertIn("touch /data/.jarvis-crown-write-test", "\n".join(adb.commands))
            self.assertEqual(adb.pushed[0][1], "/data/lineage.zip")

    def test_data_ready_requires_recovery_mount_and_write(self):
        adb = FakeAdb()
        self.assertTrue(install.twrp_data_ready(adb))
        adb.state_value = "device"
        self.assertFalse(install.twrp_data_ready(adb))
        adb.state_value = "recovery"
        adb.data_writable = False
        self.assertFalse(install.twrp_data_ready(adb))

    def test_prestaged_vendor_path_never_formats_or_reinstalls_lineage(self):
        adb = FakeAdb()
        install.prepare_lineage_vendor(adb, prestaged=True)
        joined = "\n".join(adb.commands)
        self.assertNotIn("twrp format data", joined)
        self.assertNotIn("twrp install /data/lineage.zip", joined)
        self.assertIn("mount -o ro /dev/block/mmcblk0p12", joined)

    def test_driver_gate_requires_wifi_and_bluetooth_exact_kernel_abi(self):
        adb = FakeAdb()
        original_fail = install.fail
        install.fail = lambda message: (_ for _ in ()).throw(RuntimeError(message))
        try:
            install.check_lineage_driver(adb)
            adb.driver_output = f"wifi=vermagic={install.KERNEL_RELEASE}\nbt=vermagic=wrong"
            with self.assertRaisesRegex(RuntimeError, "Bluetooth"):
                install.check_lineage_driver(adb)
        finally:
            install.fail = original_fail


if __name__ == "__main__":
    unittest.main()
