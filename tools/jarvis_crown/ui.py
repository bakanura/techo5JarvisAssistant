"""How the installer looks at the terminal: a heading per part, one line per thing, questions.

    ── Home Assistant ──────────────────────────────
      ✓ room dashboard dashboard-livingroom made
      · the Show is already in Living Room
      ! the Show has no esphome.x_y action yet

✓ is something done, · something that was already so, ! something to look at, ✗ a stop. Colour and
the symbols only where the terminal shows them; NO_COLOR, TERM=dumb and a pipe get plain text with
the same layout, so a saved log still reads.

The steps underneath report with a plain progress(text). A text starting "# " is a heading and one
starting "WARN: " a warning; the rest are sorted into done and already-so by how they read.
"""
from __future__ import annotations

import os
import re
import shutil
import sys
import textwrap
from typing import TextIO

WIDTH = 78

_CODES = re.compile(r"\033\[[0-9;]*m|[\001\002]")

# What a line says when nothing had to change.
_SAME = re.compile(r"\balready\b|\bleft (alone|as it is|at)\b|\bwas kept\b|\bwere kept\b|\bwas there\b|"
                   r"\bkept\b|\bunchanged\b|\bleft in\b|\bleft out\b|^no |, so no ")


def visible(text: str) -> str:
    """text without its colour codes: what takes room on the screen."""
    return _CODES.sub("", text)


class Out:
    def __init__(self, stream: TextIO | None = None, err: TextIO | None = None, *,
                 color: bool | None = None, fancy: bool | None = None, width: int | None = None) -> None:
        self._stream = stream
        self._err = err
        self._color = color
        self._fancy = fancy
        self._width = width
        self.warnings: list[str] = []

    # The streams are looked up when used, so a test that swaps sys.stdout sees the lines.
    @property
    def stream(self) -> TextIO:
        return self._stream or sys.stdout

    @property
    def err(self) -> TextIO:
        return self._err or sys.stderr

    @property
    def color(self) -> bool:
        if self._color is not None:
            return self._color
        return (self.stream.isatty() and not os.environ.get("NO_COLOR")
                and os.environ.get("TERM", "") != "dumb")

    @property
    def fancy(self) -> bool:
        if self._fancy is not None:
            return self._fancy
        return "utf" in (getattr(self.stream, "encoding", "") or "").lower()

    @property
    def width(self) -> int:
        if self._width:
            return self._width
        return max(40, min(WIDTH, shutil.get_terminal_size((WIDTH, 24)).columns))

    def paint(self, code: str, text: str, *, prompt: bool = False) -> str:
        """text in colour. prompt: for input(), whose line editing has to be told the codes take no room."""
        if not (self.color and text):
            return text
        start, end = f"\033[{code}m", "\033[0m"
        if prompt and "readline" in sys.modules:
            start, end = f"\001{start}\002", f"\001{end}\002"
        return start + text + end

    def bold(self, text: str, *, prompt: bool = False) -> str:
        return self.paint("1", text, prompt=prompt)

    def dim(self, text: str, *, prompt: bool = False) -> str:
        return self.paint("2", text, prompt=prompt)

    def mark(self, kind: str, *, prompt: bool = False) -> str:
        fancy, plain, code = {"done": ("✓", "+", "32"), "same": ("·", "-", "2"), "warn": ("!", "!", "33"),
                              "fail": ("✗", "x", "31"), "ask": ("?", "?", "36"), "prompt": ("›", ">", "36"),
                              "step": ("▸", ">", "36"), "caution": ("▲", "!", "33")}[kind]
        return self.paint("1;" + code if kind != "same" else code, fancy if self.fancy else plain, prompt=prompt)

    def _wrap(self, text: str, first: str, rest: str, width: int) -> list[str]:
        return textwrap.wrap(text, width=width, initial_indent=first, subsequent_indent=rest,
                             break_long_words=False, break_on_hyphens=False) or [first.rstrip()]

    def _line(self, kind: str, text: str, *, stream: TextIO | None = None, paint: str | None = None) -> None:
        for line in self.lines(kind, text, paint=paint):
            print(line, file=stream or self.stream, flush=True)

    def lines(self, kind: str, text: str, *, paint: str | None = None) -> list[str]:
        """text after its mark, wrapped under itself. The mark is one column wide whatever it looks like."""
        wrapped = self._wrap(text, "", "", self.width - 4)
        return [("  " + self.mark(kind) + " " if i == 0 else "    ") + (self.paint(paint, w) if paint else w)
                for i, w in enumerate(wrapped)]

    def heading_lines(self, text: str) -> list[str]:
        rule = "─" if self.fancy else "-"
        head = f"{rule * 2} {text} "
        return ["", self.paint("1;36", head) + self.dim(rule * max(3, self.width - len(head)))]

    def note_lines(self, text: str, *, indent: int = 4) -> list[str]:
        pad = " " * indent
        return [pad + self.dim(w) for w in self._wrap(text, "", "", self.width - indent)]

    # ------------------------------------------------------------------------------------ the lines

    def title(self, text: str, detail: str = "") -> None:
        print(file=self.stream)
        print(self.bold(text) + (self.dim("  " + detail) if detail else ""), file=self.stream, flush=True)

    def heading(self, text: str) -> None:
        for line in self.heading_lines(text):
            print(line, file=self.stream, flush=True)

    def done(self, text: str) -> None:
        self._line("done", text)

    def same(self, text: str) -> None:
        self._line("same", text, paint="2")

    def warn(self, text: str) -> None:
        self.warnings.append(text)
        self._line("warn", text, paint="33")

    def fail(self, text: str) -> None:
        self.stream.flush()
        self._line("fail", text, stream=self.err, paint="31")

    def step(self, text: str) -> None:
        """A longer stage starting, like the bootloader being unlocked."""
        self._line("step", text, paint="1")

    def caution(self, text: str) -> None:
        """Something to know before saying yes: not a problem, so not counted with the warnings."""
        self._line("caution", text, paint="33")

    def note(self, text: str, *, indent: int = 4) -> None:
        """A quieter line under the one before: a hint, a path, what to run next."""
        for line in self.note_lines(text, indent=indent):
            print(line, file=self.stream, flush=True)

    def command(self, text: str) -> None:
        print("      " + self.paint("1", text), file=self.stream, flush=True)

    def field(self, label: str, value: str, *, pad: int = 10) -> None:
        print("  " + self.dim(label.ljust(pad)) + " " + value, file=self.stream, flush=True)

    def progress(self, text: str) -> None:
        """One line from a step underneath, sorted by what it says."""
        if text.startswith("# "):
            self.heading(text[2:])
        elif text.startswith("WARN: "):
            self.warn(text[6:])
        elif _SAME.search(text):
            self.same(text)
        else:
            self.done(text)

    def finish(self, ok_text: str) -> None:
        """The last line of a run: done, or done with the warnings counted."""
        print(file=self.stream)
        if self.warnings:
            n = len(self.warnings)
            print("  " + self.mark("warn") + " " + self.bold(ok_text) +
                  self.paint("33", f", with {n} thing{'s' if n != 1 else ''} to look at (marked ! above)"),
                  file=self.stream, flush=True)
        else:
            print("  " + self.mark("done") + " " + self.bold(ok_text), file=self.stream, flush=True)


out = Out()
