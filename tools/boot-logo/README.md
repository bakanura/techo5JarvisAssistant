# Boot logos

What the bootloader paints before Linux starts lives in kaeru, amonet's bootloader in `expdb`, as a
315×170 picture in a 6105-byte slot (see [Boot logo](../../docs/hardware.md#boot-logo-lk)). Each `.bin`
here is that slot with another picture in it.

- `cronos-wordmark.bin`: the TECHO5 mark. `install-show.py` writes it on a Show 5 2nd gen or a Show 8.
- `openjade-wordmark.bin`: the OpenJade lockup, for Jarvis Shows (Crown and Checkers). The install
  leaves expdb alone; it goes in afterwards, on its own, with
  `tools/jarvis-show.py boot-logo --name "<the Show's name>"` (and back out with `--amazon-logo`).

## How the OpenJade one was made

`openjade/` holds the sources: the white jade mark and the Fira Code licence, both from the website
(`public/logos/openjade-white-jade.svg`, `public/fonts/`), and `lockup.svg`, which
`make-lockup.py` builds from them the way the website's hero lays them out, with the text outlined:

    openjade/make-lockup.py FiraCode-VF.woff2 openjade/openjade-white-jade.svg openjade/lockup.svg
    resvg openjade/lockup.svg lockup.png
    tools/linux/patch-lk-logo.py checkers-kaeru.bin kaeru-openjade.bin lockup.png \
        --bundle 335308 --in-place --colors 64

and the slot is bytes 335308 to 341413 of the result. 64 flat colours keep the mark's shading and
compress to 5124 bytes. `make-lockup.py` needs fontTools with brotli; `checkers-kaeru.bin` is the one in
amonet-checkers v2.0.1. The slot is the same in Crown's kaeru, so one file serves both.

## The other themes

The website has four jade themes, and `openjade/` has each one's mark (`openjade-<theme>-jade.svg`).
`openjade/themes/` holds the lockup in every one of them, 315×170 like the boot slot:

- `lockup-<theme>-dark.svg`: on black, "Jade" in the theme's colour lifted towards white, as the
  boot logo has it. `lockup-white-dark.svg` is `lockup.svg`.
- `lockup-<theme>-light.svg`: on the theme's page colour, in the hero's colours.
- `lockup-<theme>-transparent.svg`: the hero's colours with no ground, to put on something light.

The themes are `white`, `sakura`, `leafgreen` and `ember`. One of them, made again:

    openjade/make-lockup.py FiraCode-VF.woff2 openjade/openjade-sakura-jade.svg out.svg \
        --theme sakura --ground light [--transparent]

`--size` and `--width` give another canvas and width, if a different size is wanted.
