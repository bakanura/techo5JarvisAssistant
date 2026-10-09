from contextlib import redirect_stdout
import hashlib
import importlib.util
import io
import pathlib
import sys
import tempfile
import unittest
import zipfile
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


def manual_asset(sha256="", size=0):
    return assets.Asset("amonet", "amonet-test.zip", "https://xdaforums.com/attachments/x.1/", sha256, size,
                        "test", manual=True)


def write_zip(path, files):
    with zipfile.ZipFile(path, "w") as z:
        for name, data in files.items():
            z.writestr(name, data)
    return path


def no_blobs(repo, branches):
    return {}


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
            self.assertTrue({"lineage", "boot"} <= set(kinds) <= {"lineage", "boot", "amonet"})
            self.assertEqual(kinds["boot"].sha256, BOOT_SHA256_BY_BOARD[board])
            for a in (kinds["lineage"], kinds["boot"]):
                self.assertFalse(a.manual)
                self.assertRegex(a.sha256, r"^[0-9a-f]{64}$")
                self.assertTrue(a.url.startswith("https://github.com/"), a.url)
                self.assertGreater(a.size, 0)
            self.assertRegex(kinds["lineage"].name, r"^lineage-18\.1-2026090[4-9]-UNOFFICIAL-%s\.zip$" % board)

    def test_amonet_packages_are_manual_and_either_unpinned_or_fully_pinned(self):
        amonet = [a for board in ("crown", "checkers") for a in assets.assets_for(board) if a.kind == "amonet"]
        self.assertTrue(amonet)
        for a in amonet:
            self.assertTrue(a.manual)
            self.assertTrue(a.url.startswith("https://xdaforums.com/attachments/"), a.url)
            if a.sha256:
                self.assertRegex(a.sha256, r"^[0-9a-f]{64}$")
                self.assertGreater(a.size, 0)
            else:
                self.assertEqual(a.size, 0)

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


class ManualTests(unittest.TestCase):
    def run_fetch(self, td, table, **kw):
        kw.setdefault("blobs_for", no_blobs)
        with mock.patch.object(fetcher, "assets_for", lambda board: table):
            return fetcher.fetch_board("checkers", pathlib.Path(td) / "cache",
                                       downloads=pathlib.Path(td) / "dl", **kw)

    def test_missing_manual_asset_is_reported_not_fatal(self):
        with tempfile.TemporaryDirectory() as td:
            out = io.StringIO()
            with redirect_stdout(out):
                self.assertEqual(self.run_fetch(td, (manual_asset(),)), {})
            self.assertIn("https://xdaforums.com/attachments/x.1/", out.getvalue())

    def test_unpinned_package_is_inspected_and_refused(self):
        with tempfile.TemporaryDirectory() as td:
            dl = pathlib.Path(td) / "dl"
            dl.mkdir()
            script, binary, changed = b"echo hi\n", b"\x00lk", b"echo changed\n"
            write_zip(dl / "amonet-test.zip", {
                "amonet/fastbrick.sh": script,
                "amonet/bin/lk.bin": binary,
                "amonet/modules/main.py": changed,
            })
            upstream = {
                fetcher.git_blob_sha(script): [("mt8163-echo-show", "fastbrick.sh")],
                fetcher.git_blob_sha(b"original\n"): [("mt8163-echo-show", "modules/main.py")],
            }
            out = io.StringIO()
            with redirect_stdout(out), self.assertRaisesRegex(techo5lib.Fail, "no pinned sha256"):
                self.run_fetch(td, (manual_asset(),), blobs_for=lambda repo, branches: upstream)
            text = out.getvalue()
            self.assertIn("1 files match", text)
            self.assertIn("DIFFERS from every upstream copy: amonet/modules/main.py", text)
            self.assertIn("only in the zip: amonet/bin/lk.bin", text)
            self.assertFalse((pathlib.Path(td) / "cache").exists())

    def test_pinned_package_is_taken_from_downloads_and_then_reused(self):
        with tempfile.TemporaryDirectory() as td:
            dl = pathlib.Path(td) / "dl"
            dl.mkdir()
            data = write_zip(dl / "amonet-test.zip", {"amonet/x": b"x"}).read_bytes()
            table = (manual_asset(hashlib.sha256(data).hexdigest(), len(data)),)
            with redirect_stdout(io.StringIO()):
                paths = self.run_fetch(td, table)
                self.assertEqual(paths["amonet"].read_bytes(), data)
                (dl / "amonet-test.zip").unlink()
                self.assertEqual(self.run_fetch(td, table, check_only=True)["amonet"], paths["amonet"])

    def test_pinned_package_with_other_bytes_is_refused(self):
        with tempfile.TemporaryDirectory() as td:
            dl = pathlib.Path(td) / "dl"
            dl.mkdir()
            data = write_zip(dl / "amonet-test.zip", {"amonet/x": b"x"}).read_bytes()
            table = (manual_asset("0" * 64, len(data)),)
            with redirect_stdout(io.StringIO()), self.assertRaisesRegex(techo5lib.Fail, "does not match"):
                self.run_fetch(td, table)
            self.assertFalse((pathlib.Path(td) / "cache" / "checkers" / "amonet-test.zip").exists())

    def test_upstream_path_strips_package_folders(self):
        self.assertEqual(fetcher.upstream_path("amonet/modules/main.py"), "modules/main.py")
        self.assertEqual(fetcher.upstream_path("unlock/amonet/fastbrick.sh"), "fastbrick.sh")
        self.assertEqual(fetcher.upstream_path("META-INF/com/google/android/update-binary"),
                         "META-INF/com/google/android/update-binary")


if __name__ == "__main__":
    unittest.main()
