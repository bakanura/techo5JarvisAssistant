import hashlib
import importlib.util
import pathlib
import sys
import tempfile
import unittest
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[1]
TOOLS = ROOT / "tools"
sys.path.insert(0, str(TOOLS))

from jarvis_crown import assets  # noqa: E402
from jarvis_crown.install import BOOT_SHA256_BY_BOARD  # noqa: E402
import techo5lib  # noqa: E402

SPEC = importlib.util.spec_from_file_location("jarvis_fetch_show_assets", TOOLS / "fetch-show-assets.py")
fetcher = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(fetcher)


def local_asset(td, name, data, *, sha256=None, size=None):
    src = pathlib.Path(td) / ("src-" + name)
    src.write_bytes(data)
    return assets.Asset(
        "lineage" if name.endswith(".zip") else "boot", name, src.as_uri(),
        sha256 or hashlib.sha256(data).hexdigest(), len(data) if size is None else size, "test",
    )


class PinTests(unittest.TestCase):
    def test_every_board_has_a_pinned_lineage_zip_and_the_installer_boot_image(self):
        for board in ("crown", "checkers"):
            kinds = {a.kind: a for a in assets.assets_for(board)}
            self.assertEqual(set(kinds), {"lineage", "boot"})
            self.assertEqual(kinds["boot"].sha256, BOOT_SHA256_BY_BOARD[board])
            for a in kinds.values():
                self.assertRegex(a.sha256, r"^[0-9a-f]{64}$")
                self.assertTrue(a.url.startswith("https://github.com/"), a.url)
                self.assertGreater(a.size, 0)
            self.assertRegex(kinds["lineage"].name, r"^lineage-18\.1-2026090[4-9]-UNOFFICIAL-%s\.zip$" % board)

    def test_cache_defaults_outside_the_repository(self):
        with mock.patch.dict("os.environ", {"JARVIS_SHOW_ASSETS": "", "XDG_CACHE_HOME": "/var/cache-test"}):
            self.assertEqual(assets.default_cache_dir(), pathlib.Path("/var/cache-test/jarvis-show"))


class FetchTests(unittest.TestCase):
    def run_fetch(self, cache, table, **kw):
        with mock.patch.object(fetcher, "assets_for", lambda board: table):
            return fetcher.fetch_board("checkers", cache, **kw)

    def test_refuses_a_cache_inside_the_repository(self):
        with self.assertRaisesRegex(techo5lib.Fail, "inside the repository"):
            fetcher.fetch_board("checkers", ROOT / "inputs")

    def test_downloads_verifies_and_reuses(self):
        with tempfile.TemporaryDirectory() as td:
            table = (local_asset(td, "l.zip", b"zip"), local_asset(td, "b.img", b"boot"))
            cache = pathlib.Path(td) / "cache"
            paths = self.run_fetch(cache, table)
            self.assertEqual(paths["lineage"].read_bytes(), b"zip")
            self.assertEqual(paths["boot"].read_bytes(), b"boot")
            with mock.patch.object(fetcher, "download_checked", side_effect=AssertionError("re-downloaded")):
                self.run_fetch(cache, table)
                self.run_fetch(cache, table, check_only=True)

    def test_hash_mismatch_keeps_nothing(self):
        with tempfile.TemporaryDirectory() as td:
            table = (local_asset(td, "l.zip", b"zip", sha256="0" * 64),)
            cache = pathlib.Path(td) / "cache"
            with self.assertRaisesRegex(techo5lib.Fail, "does not match its checksum"):
                self.run_fetch(cache, table)
            self.assertEqual([p for p in cache.rglob("*") if p.is_file()], [])

    def test_size_mismatch_is_refused(self):
        with tempfile.TemporaryDirectory() as td:
            table = (local_asset(td, "l.zip", b"zip", size=4),)
            cache = pathlib.Path(td) / "cache"
            with self.assertRaisesRegex(techo5lib.Fail, "pinned size"):
                self.run_fetch(cache, table)
            self.assertFalse((cache / "checkers" / "l.zip").exists())

    def test_check_only_never_downloads(self):
        with tempfile.TemporaryDirectory() as td:
            table = (local_asset(td, "l.zip", b"zip"),)
            with self.assertRaisesRegex(techo5lib.Fail, "missing or does not match"):
                self.run_fetch(pathlib.Path(td) / "cache", table, check_only=True)


if __name__ == "__main__":
    unittest.main()
