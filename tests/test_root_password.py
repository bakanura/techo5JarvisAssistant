import pathlib
import sys
import unittest
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "tools"))

import techo5lib  # noqa: E402
from techo5lib import ConsoleLocked, console_exchange, sha512_crypt  # noqa: E402


class Sha512CryptTests(unittest.TestCase):
    # The test vectors from Ulrich Drepper's SHA-crypt specification.
    def test_spec_vectors(self):
        self.assertEqual(
            sha512_crypt("Hello world!", "saltstring"),
            "$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1")
        self.assertEqual(
            sha512_crypt("Hello world!", "saltstringsaltstring", rounds=10000),
            "$6$rounds=10000$saltstringsaltst$OW1/O6BYHV6BcXZu8QVeXbDWra3Oeqh0sbHbbMCVNSnCM/UrjmM0Dp8vOuZeHBy/YTBmSK6H9qs/y3RnOaw5v.")

    def test_random_salt_and_shape(self):
        a, b = sha512_crypt("correct horse"), sha512_crypt("correct horse")
        self.assertNotEqual(a, b)
        self.assertRegex(a, r"^\$6\$[A-Za-z0-9./]{16}\$[A-Za-z0-9./]{86}$")


class FakePort:
    """A serial console that answers like busybox login, then like a shell."""

    def __init__(self, password):
        self.password, self.state, self.out = password, "login", []

    def write(self, data):
        text = data.decode()
        if self.state == "login":
            if text == "\n":
                self.out.append(b"\r\ntecho5 login: ")
            elif text == "root\n":
                self.state = "password"
                self.out.append(b"root\r\nPassword: ")
        elif self.state == "password":
            if text == self.password + "\n":
                self.state = "shell"
                self.out.append(b"\r\ntecho5:~ # ")
            else:
                self.state = "login"
                self.out.append(b"\r\nLogin incorrect\r\ntecho5 login: ")
        elif text.startswith("b="):
            b = text.split(";")[0][2:]
            m = text.split(";")[1].strip()[2:]
            self.out.append(("%s\r\n%s\r\nHELLO\r\n%s\r\ntecho5:~ # " % (text, b, m)).encode())
        else:
            self.out.append(b"\r\ntecho5:~ # ")

    def read(self):
        return self.out.pop(0) if self.out else b""

    def close(self):
        pass


class ConsoleLoginTests(unittest.TestCase):
    def run_with(self, port, password):
        with mock.patch.object(techo5lib, "SerialPort", lambda _: port), mock.patch.object(techo5lib.time, "sleep"):
            return console_exchange("/dev/fake", "echo HELLO", 2, password)

    def test_logs_in_and_runs(self):
        port = FakePort("correct horse")
        self.assertEqual(self.run_with(port, "correct horse"), "HELLO")
        self.assertEqual(port.state, "shell")

    def test_no_or_wrong_password_is_locked(self):
        with self.assertRaises(ConsoleLocked):
            self.run_with(FakePort("correct horse"), None)
        with self.assertRaises(ConsoleLocked):
            self.run_with(FakePort("correct horse"), "wrong one")


class DeviceSideTests(unittest.TestCase):
    def test_hash_is_laid_over_shadow_and_console_asks(self):
        lib = (ROOT / "tools/linux/techo5-lib.sh").read_text()
        self.assertIn("T5_ROOT_PW=/data/misc/techo5/root_pw", lib)
        self.assertIn("mount --bind /run/techo5/shadow /etc/shadow", lib)
        console = (ROOT / "tools/linux/rootfs/usr/local/sbin/techo5-console").read_text()
        self.assertIn("exec /bin/login", console)
        self.assertIn("usb_install", console)
        installer = (ROOT / "tools/install-show.py").read_text()
        self.assertIn("/data/misc/techo5/root_pw", installer)


if __name__ == "__main__":
    unittest.main()
