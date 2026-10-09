#!/usr/bin/env python3
"""The OpenJade lockup as the bootloader shows it: the white jade mark, then "open" and "Jade" in Fira
Code Bold, laid out like the website's hero (text 80 px, mark 112 px, 16 px between them, letter
spacing -0.04 em), scaled to the given width and centred on black. The text is outlined, so the SVG
renders the same anywhere without the font.

    make-lockup.py FiraCode-VF.woff2 openjade-white-jade.svg lockup.svg [--size 315x170] [--width 290]

Needs fontTools with brotli (for the woff2).
"""
import argparse
import re

from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont

TEXT_PX, MARK_PX, GAP_PX, TRACKING = 80.0, 112.0, 16.0, -0.04
OPEN_COLOR, JADE_COLOR = "#f6fbf8", "#aab1ba"  # as on the site's dark ground


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("font")
    ap.add_argument("mark")
    ap.add_argument("out")
    ap.add_argument("--size", default="315x170", help="canvas, the wordmark slot's size by default")
    ap.add_argument("--width", type=float, default=290, help="how wide the lockup is drawn")
    a = ap.parse_args()
    w, h = (int(v) for v in a.size.split("x"))

    font = instantiateVariableFont(TTFont(a.font), {"wght": 700})
    glyphs, cmap, hmtx = font.getGlyphSet(), font.getBestCmap(), font["hmtx"]
    scale = TEXT_PX / font["head"].unitsPerEm

    def outline(text, x):
        pen = SVGPathPen(glyphs)
        for ch in text:
            g = cmap[ord(ch)]
            glyphs[g].draw(TransformPen(pen, (scale, 0, 0, -scale, x, 0)))
            x += hmtx[g][0] * scale + TRACKING * TEXT_PX
        return pen.getCommands(), x

    start = MARK_PX + GAP_PX
    open_d, x = outline("open", start)
    jade_d, x = outline("Jade", x)
    total = x - TRACKING * TEXT_PX  # no tracking after the last letter
    # The capitals sit centred on the mark, as the hero's flex row centres them.
    baseline = (MARK_PX + font["OS/2"].sCapHeight * scale) / 2

    k = a.width / total
    ox, oy = (w - total * k) / 2, (h - MARK_PX * k) / 2
    mark = re.search(r"<svg[^>]*>(.*)</svg>", open(a.mark).read(), re.S).group(1)
    with open(a.out, "w") as f:
        f.write(f'''<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}">
<rect width="{w}" height="{h}" fill="#000"/>
<g transform="translate({ox:.3f} {oy:.3f}) scale({k:.5f})">
<svg width="{MARK_PX:g}" height="{MARK_PX:g}" viewBox="0 0 128 128">{mark}</svg>
<g transform="translate(0 {baseline:.3f})">
<path fill="{OPEN_COLOR}" d="{open_d}"/>
<path fill="{JADE_COLOR}" d="{jade_d}"/>
</g>
</g>
</svg>
''')
    print(f"{a.out}: lockup {total * k:.0f}x{MARK_PX * k:.0f} on {w}x{h}")


if __name__ == "__main__":
    main()
