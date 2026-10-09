#!/usr/bin/env python3
"""The OpenJade lockup: a jade mark, then "open" and "Jade" in Fira Code Bold, laid out like the
website's hero (text 80 px, mark 112 px, 16 px between them, letter spacing -0.04 em), scaled to the
given width and centred. The text is outlined, so the SVG renders the same anywhere without the font.

    make-lockup.py FiraCode-VF.woff2 openjade-white-jade.svg lockup.svg [--size 315x170] [--width 290]
        [--theme white|sakura|leafgreen|ember] [--ground dark|light] [--transparent]

The defaults are what the bootloader shows: the white theme on black. On a light ground the colours
are the website's hero: "open" in the theme's text colour, "Jade" in its strong accent, on its page
colour. On a dark one "Jade" is the accent lifted towards white, as the site's dark top bar has it.
Pass the theme's own mark (openjade-<theme>-jade.svg) along with --theme.

Needs fontTools with brotli (for the woff2).
"""
import argparse
import re

from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont

TEXT_PX, MARK_PX, GAP_PX, TRACKING = 80.0, 112.0, 16.0, -0.04

# From the website's jade-themes.css: --text-primary, --accent, --accent-strong and --bg-canvas.
THEMES = {
    "white": ("#20242a", "#86909d", "#58626f", "#eeeff2"),
    "sakura": ("#302128", "#d66f99", "#9f4d74", "#f8edf3"),
    "leafgreen": ("#1f3127", "#3d9a61", "#2a6c44", "#edf6f1"),
    "ember": ("#34180f", "#ea9468", "#a63a16", "#f7ece7"),
}
DARK_OPEN, DARK_GROUND = "#f6fbf8", "#000"


def lifted(color, white=0.3):
    """color-mix(in srgb, color 70%, #fff 30%), the site's top bar "jade"."""
    rgb = [int(color[i:i + 2], 16) for i in (1, 3, 5)]
    return "#" + "".join(f"{round(c + (255 - c) * white):02x}" for c in rgb)


def colours(theme, ground):
    """"open", "Jade" and the ground, for a theme on a dark or a light ground."""
    text, accent, strong, canvas = THEMES[theme]
    if ground == "dark":
        return DARK_OPEN, lifted(accent), DARK_GROUND
    return text, strong, canvas


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("font")
    ap.add_argument("mark")
    ap.add_argument("out")
    ap.add_argument("--size", default="315x170", help="canvas, the wordmark slot's size by default")
    ap.add_argument("--width", type=float, default=290, help="how wide the lockup is drawn")
    ap.add_argument("--theme", choices=THEMES, default="white", help="whose colours")
    ap.add_argument("--ground", choices=("dark", "light"), default="dark", help="what it sits on")
    ap.add_argument("--transparent", action="store_true", help="leave the ground out")
    a = ap.parse_args()
    w, h = (int(v) for v in a.size.split("x"))
    open_color, jade_color, ground = colours(a.theme, a.ground)
    rect = "" if a.transparent else f'<rect width="{w}" height="{h}" fill="{ground}"/>\n'

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
{rect}<g transform="translate({ox:.3f} {oy:.3f}) scale({k:.5f})">
<svg width="{MARK_PX:g}" height="{MARK_PX:g}" viewBox="0 0 128 128">{mark}</svg>
<g transform="translate(0 {baseline:.3f})">
<path fill="{open_color}" d="{open_d}"/>
<path fill="{jade_color}" d="{jade_d}"/>
</g>
</g>
</svg>
''')
    print(f"{a.out}: {a.theme} on {'nothing' if a.transparent else a.ground}, lockup {total * k:.0f}x{MARK_PX * k:.0f} on {w}x{h}")


if __name__ == "__main__":
    main()
