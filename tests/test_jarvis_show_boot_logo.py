import hashlib
import os
import pathlib
import re
import sys
import tempfile
import unittest
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "tools"))

from jarvis_crown import boot_logo as bl  # noqa: E402

OFFSET, LENGTH = 1000, 9000
AMAZON = os.urandom(bl.SLOT_SIZE)  # stands in for Amazon's picture, which is not in the repo
LOGO = bl.LOGO_FILE.read_bytes()


def sha(b):
    return hashlib.sha256(b).hexdigest()


def kaeru(slot):
    base = bytearray(hashlib.sha256(b"kaeru").digest() * (LENGTH // 32 + 1))[:LENGTH]
    base[OFFSET:OFFSET + bl.SLOT_SIZE] = slot
    return bytes(base)


class FakeShow:
    """expdb as bytes, answering the commands boot_logo sends over SSH."""

    def __init__(self, slot):
        self.expdb = bytearray(kaeru(slot) + bytes(4096))
        self.writes = []

    def shell(self, command, data=None):
        if "PARTNAME=expdb" in command:
            return "/sys/class/block/mmcblk0p7/uevent\n"
        if "drop_caches" in command:
            return ""
        m = re.fullmatch(r"busybox head -c (\d+) /dev/mmcblk0p7 \| busybox sha256sum", command)
        if m:
            return sha(bytes(self.expdb[:int(m[1])])) + "  -\n"
        m = re.fullmatch(r"busybox dd of=/dev/mmcblk0p7 bs=1 seek=(\d+) count=(\d+) conv=notrunc 2>/dev/null && busybox sync",
                         command)
        if m:
            at, n = int(m[1]), int(m[2])
            self.writes.append(at)
            self.expdb[at:at + n] = data[:n]
            return ""
        raise AssertionError(f"unexpected command {command!r}")


class BootLogoTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.saved = pathlib.Path(tmp.name) / "p7-expdb.img"
        self.saved.write_bytes(kaeru(AMAZON))
        board = bl.Kaeru(OFFSET, LENGTH, sha(kaeru(AMAZON)), sha(kaeru(LOGO)))
        for patch in (mock.patch.dict(bl.KAERU, {"test": board}),
                      mock.patch.object(bl, "AMAZON_SLOT_SHA256", sha(AMAZON))):
            patch.start()
            self.addCleanup(patch.stop)

    def put(self, show, **kw):
        kw.setdefault("confirm", lambda phrase: phrase)
        return bl.put_logo(show, "test", saved_expdb=self.saved, **kw)

    def test_the_logo_file_is_the_pinned_one(self):
        self.assertEqual(len(LOGO), bl.SLOT_SIZE)
        self.assertEqual(sha(LOGO), bl.LOGO_SLOT_SHA256)

    def test_the_logo_goes_into_the_slot_and_nowhere_else(self):
        show = FakeShow(AMAZON)
        self.assertIn("written", self.put(show))
        self.assertEqual(bytes(show.expdb[:LENGTH]), kaeru(LOGO))
        self.assertEqual(show.writes, [OFFSET])

    def test_amazons_logo_goes_back(self):
        show = FakeShow(LOGO)
        self.put(show, amazon=True)
        self.assertEqual(bytes(show.expdb[:LENGTH]), kaeru(AMAZON))

    def test_nothing_is_written_twice(self):
        show = FakeShow(LOGO)
        self.assertIn("already", self.put(show))
        self.assertEqual(show.writes, [])

    def test_an_unknown_kaeru_is_left_alone(self):
        show = FakeShow(os.urandom(bl.SLOT_SIZE))
        with self.assertRaisesRegex(bl.BootLogoError, "nothing written"):
            self.put(show)
        self.assertEqual(show.writes, [])

    def test_without_the_phrase_nothing_is_written(self):
        show = FakeShow(AMAZON)
        with self.assertRaisesRegex(bl.BootLogoError, "not confirmed"):
            self.put(show, confirm=lambda phrase: "")
        self.assertEqual(show.writes, [])

    def test_a_bad_read_back_puts_the_old_slot_back(self):
        show = FakeShow(AMAZON)
        orig = show.shell

        def corrupting(command, data=None):
            if "dd of=" in command and not show.writes:
                data = bytes(len(data))  # the first write lands as zeros
            return orig(command, data)
        show.shell = corrupting
        with self.assertRaisesRegex(bl.BootLogoError, "went back"):
            self.put(show)
        self.assertEqual(bytes(show.expdb[:LENGTH]), kaeru(AMAZON))
        self.assertEqual(show.writes, [OFFSET, OFFSET])

    def test_a_saved_expdb_without_amazons_logo_stops_it(self):
        self.saved.write_bytes(kaeru(LOGO))
        with self.assertRaisesRegex(bl.BootLogoError, "does not hold Amazon"):
            self.put(FakeShow(AMAZON))

    def test_the_real_boards_are_pinned(self):
        self.assertEqual(bl.KAERU["checkers"].offset, 335308)
        self.assertEqual(bl.KAERU["crown"].offset, 288716)
        for k in (bl.KAERU["checkers"], bl.KAERU["crown"]):
            self.assertRegex(k.amazon, r"^[0-9a-f]{64}$")
            self.assertRegex(k.openjade, r"^[0-9a-f]{64}$")
            self.assertGreater(k.length, k.offset + bl.SLOT_SIZE)


if __name__ == "__main__":
    unittest.main()
