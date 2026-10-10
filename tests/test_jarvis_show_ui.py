import io
import pathlib
import sys
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "tools"))

from jarvis_crown import ui  # noqa: E402


def plain(**kw):
    stream = io.StringIO()
    return ui.Out(stream, stream, color=False, fancy=True, width=60, **kw), stream


class OutTests(unittest.TestCase):
    def test_lines_are_sorted_by_what_they_say(self):
        out, stream = plain()
        out.progress("# Home Assistant")
        out.progress("room dashboard dashboard-livingroom made")
        out.progress("the Show is already in Living Room")
        out.progress("WARN: no esphome action yet")
        lines = stream.getvalue().splitlines()
        self.assertEqual(lines[0], "")
        self.assertTrue(lines[1].startswith("── Home Assistant ──"))
        self.assertEqual(len(lines[1]), 60)
        self.assertEqual(lines[2:], ["  ✓ room dashboard dashboard-livingroom made",
                                     "  · the Show is already in Living Room",
                                     "  ! no esphome action yet"])
        self.assertEqual(out.warnings, ["no esphome action yet"])

    def test_long_lines_wrap_under_their_text(self):
        out, stream = plain()
        out.done("word " * 20)
        lines = stream.getvalue().splitlines()
        self.assertGreater(len(lines), 1)
        self.assertTrue(all(len(line) <= 60 for line in lines))
        self.assertTrue(all(line.startswith("    ") for line in lines[1:]))

    def test_without_utf8_or_colour_the_layout_stays(self):
        stream = io.StringIO()
        out = ui.Out(stream, stream, color=False, fancy=False, width=60)
        out.heading("Checks")
        out.done("ok")
        out.fail("stop")
        self.assertEqual(stream.getvalue().splitlines(), ["", "-- Checks " + "-" * 50, "  + ok", "  x stop"])

    def test_colour_takes_no_room(self):
        out = ui.Out(io.StringIO(), color=True, fancy=True, width=60)
        line = out.lines("warn", "look here", paint="33")[0]
        self.assertIn("\033[", line)
        self.assertEqual(ui.visible(line), "  ! look here")
        prompt = out.paint("36", "›", prompt=True)
        self.assertEqual(ui.visible(prompt), "›")

    def test_finish_counts_the_warnings(self):
        out, stream = plain()
        out.finish("Done")
        out.warn("one")
        out.warn("two")
        out.finish("Done")
        self.assertEqual(stream.getvalue().splitlines(),
                         ["", "  ✓ Done", "  ! one", "  ! two", "", "  ! Done, with 2 things to look at (marked ! above)"])

    def test_cautions_are_not_counted(self):
        out, _ = plain()
        out.caution("it erases the Show")
        self.assertEqual(out.warnings, [])


if __name__ == "__main__":
    unittest.main()
