from pathlib import Path
import json
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
TOOL = ROOT / "tools/live-acceptance.py"


class LiveAcceptanceToolTests(unittest.TestCase):
    def run_tool(self, *args, expected=0):
        proc = subprocess.run(
            [sys.executable, str(TOOL), *args],
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            timeout=15,
        )
        self.assertEqual(proc.returncode, expected, proc.stdout)
        return proc.stdout

    def test_verify_fails_closed_without_live_evidence(self):
        with tempfile.TemporaryDirectory() as td:
            state = Path(td) / "state.json"
            out = self.run_tool("verify", "--state", str(state), expected=1)
            self.assertIn("FAIL: J38 crown", out)
            self.assertIn("FAIL: J38 checkers", out)
            self.assertIn("FAIL: J39", out)

    def test_j38_requires_every_explicit_operator_confirmation(self):
        with tempfile.TemporaryDirectory() as td:
            state = Path(td) / "state.json"
            shot = Path(td) / "crown.png"
            shot.write_bytes(b"physical screenshot")
            out = self.run_tool(
                "j38", "--state", str(state), "--board", "crown", "--evidence", str(shot),
                "--no-black-bar",
                expected=1,
            )
            self.assertIn("refusing PASS without explicit confirmations", out)
            self.assertFalse(state.exists())

    def test_j38_and_j39_record_hashes_not_evidence_contents(self):
        with tempfile.TemporaryDirectory() as td:
            state = Path(td) / "state.json"
            crown = Path(td) / "crown.png"
            checkers = Path(td) / "checkers.png"
            general = Path(td) / "general.txt"
            recipe = Path(td) / "recipe.txt"
            crown.write_bytes(b"crown-screen-secret-marker")
            checkers.write_bytes(b"checkers-screen-secret-marker")
            general.write_text("general-response-secret-marker", encoding="utf-8")
            recipe.write_text("recipe-response-secret-marker", encoding="utf-8")

            j38_flags = (
                "--no-black-bar", "--no-header-sidebar", "--edge-touch-ok", "--native-geometry",
                "--card-geometry-unchanged", "--cold-reload-clean",
            )
            self.run_tool("j38", "--state", str(state), "--board", "crown", "--evidence", str(crown), *j38_flags)
            self.run_tool("j38", "--state", str(state), "--board", "checkers", "--evidence", str(checkers), *j38_flags)
            self.run_tool(
                "j39", "--state", str(state),
                "--general-evidence", str(general), "--recipe-evidence", str(recipe),
                "--ha-owned", "--general-answer-ok", "--recipe-web-grounded", "--no-device-misroute",
            )

            raw = state.read_text(encoding="utf-8")
            self.assertNotIn("secret-marker", raw)
            data = json.loads(raw)
            self.assertEqual(len(data["j38"]["crown"]["evidence"]["sha256"]), 64)
            self.assertEqual(len(data["j39"]["general_evidence"]["sha256"]), 64)
            self.assertEqual(state.stat().st_mode & 0o777, 0o600)
            out = self.run_tool("verify", "--state", str(state))
            self.assertIn("PASS: J38 crown", out)
            self.assertIn("PASS: J38 checkers", out)
            self.assertIn("PASS: J39", out)

    def test_missing_evidence_file_is_rejected(self):
        with tempfile.TemporaryDirectory() as td:
            state = Path(td) / "state.json"
            out = self.run_tool(
                "j38", "--state", str(state), "--board", "crown", "--evidence", str(Path(td) / "missing.png"),
                "--no-black-bar", "--no-header-sidebar", "--edge-touch-ok", "--native-geometry",
                "--card-geometry-unchanged", "--cold-reload-clean",
                expected=1,
            )
            self.assertIn("evidence file not found", out)


if __name__ == "__main__":
    unittest.main()
