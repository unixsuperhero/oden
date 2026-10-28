#!/usr/bin/env python3
"""Regenerate oden's logo set: assets/icon.svg, assets/logo-light.svg and
assets/logo-dark.svg.

    python3 assets/build.py

The mark is the Kushi (concept oden-1 in ~/hub/o-plans/icons/build.py, whose
drawing code this copies): konnyaku, daikon and chikuwa on one bamboo skewer,
as subcommands sit on one bin. The files follow ochazuke's and matomeru's
assets: a 128x128 icon on a rounded tile (rx 28) in a dark tint of the tool's
hue, and a 96-high logo of the untiled mark beside the name in the family's
monoline lettering (round-capped strokes, width 11, x-height 44 to 84),
drawn as paths so no font is needed. Standard library only.
"""
import pathlib

OUT = pathlib.Path(__file__).resolve().parent
NAME = "oden"

# Dashi amber. TILE is the icon's ground and the light logo's lettering;
# PALE is the dark logo's lettering.
TILE, PALE = "#241a12", "#fff1d6"

# Ingredients (cartoon style: round shapes, a dark gap outline, light detail).
OUTLINE = "#3b2615"
KON, KON_DK, KON_LT = "#9d93ab", "#5f566b", "#c4bccf"
DAI_TOP, DAI_RIM, DAI_SIDE, DAI_LINE = "#fff4dc", "#f0d6a0", "#e7ad5e", "#e2bb7c"
CHK_SKIN, CHK_TOAST, CHK_END, CHK_HOLE = "#c9803f", "#8f4f22", "#fbe6c2", "#3b2615"
# The stick: pale bamboo on dark grounds, a deeper bamboo on white so it reads.
STICK = {"dark": "#dcbc7e", "light": "#c49a52"}


def g(x, y, body, s=1.0, rot=0.0):
    return f'<g transform="translate({x} {y}) rotate({rot}) scale({s})">{body}</g>'


def konnyaku(x, y, s=1.0, rot=0.0):
    body = (f'<path d="M-4.5 -14.5Q0 -20 4.5 -14.5Q13 -3 17 8Q20 16 11 16.5Q0 18 -11 16.5'
            f'Q-20 16 -17 8Q-13 -3 -4.5 -14.5Z" fill="{KON}" stroke="{OUTLINE}" stroke-width="3" stroke-linejoin="round"/>'
            f'<path d="M-7 -1L5 11M-1 -9L11 3M7 -3L-5 9M1 -10L-11 2" stroke="{KON_LT}" stroke-width="1.4" stroke-linecap="round" opacity=".45"/>'
            f'<g fill="{KON_DK}"><circle cx="-9" cy="10" r="1.4"/><circle cx="-2" cy="4" r="1.2"/>'
            f'<circle cx="8" cy="11" r="1.4"/><circle cx="3" cy="-5" r="1.1"/><circle cx="-5" cy="-3" r="1"/>'
            f'<circle cx="12" cy="6" r="1.1"/><circle cx="1" cy="12" r="1.2"/></g>'
            f'<path d="M-8 -3Q-4 -10 -1 -13" fill="none" stroke="#ddd6e5" stroke-width="2.2" stroke-linecap="round"/>')
    return g(x, y, body, s, rot)


def daikon(x, y, s=1.0, rot=0.0, rx=18, ry=7.5, h=12, stick="#dcbc7e"):
    """A thick round slice seen from slightly above: creamy top, chamfered rim, broth-tinted side."""
    top = -h / 2
    side = (f'<path d="M{-rx} {top}V{top+h}A{rx} {ry} 0 0 0 {rx} {top+h}V{top}Z" fill="{DAI_SIDE}" '
            f'stroke="{OUTLINE}" stroke-width="3" stroke-linejoin="round"/>')
    face = (f'<ellipse cx="0" cy="{top}" rx="{rx}" ry="{ry}" fill="{DAI_TOP}" stroke="{OUTLINE}" stroke-width="3"/>'
            f'<ellipse cx="0" cy="{top}" rx="{rx*0.72:.1f}" ry="{ry*0.68:.1f}" fill="none" stroke="{DAI_RIM}" stroke-width="2"/>'
            f'<path d="M{-rx*0.42:.1f} {top-ry*0.2:.1f}L{-rx*0.15:.1f} {top-ry*0.07:.1f}M{rx*0.42:.1f} {top+ry*0.2:.1f}L{rx*0.15:.1f} {top+ry*0.07:.1f}'
            f'M{rx*0.1:.1f} {top-ry*0.45:.1f}L{rx*0.04:.1f} {top-ry*0.18:.1f}M{-rx*0.1:.1f} {top+ry*0.45:.1f}L{-rx*0.04:.1f} {top+ry*0.18:.1f}" '
            f'stroke="{DAI_LINE}" stroke-width="1.5" stroke-linecap="round"/>'
            f'<path d="M{-rx+3} {top+3}Q{-rx+4} {top+h-1} {-rx+9} {top+h+ry*0.55:.1f}" fill="none" stroke="#f6c98a" stroke-width="2" stroke-linecap="round"/>')
    stub = f'<ellipse cx="0" cy="{top}" rx="2.6" ry="1.6" fill="{stick}"/>'
    return g(x, y, side + face + stub, s, rot)


def chikuwa(x, y, s=1.0, rot=0.0):
    """A browned tube lying across the skewer, its pale cut end and hole on the right."""
    body = (f'<path d="M-16 -9H14A6 9 0 0 1 14 9H-16A6 9 0 0 1 -16 -9Z" fill="{CHK_SKIN}" stroke="{OUTLINE}" stroke-width="3" stroke-linejoin="round"/>'
            f'<path d="M-15 -4Q-8 -7 -1 -4Q6 -1 12 -5" fill="none" stroke="{CHK_TOAST}" stroke-width="3.5" stroke-linecap="round"/>'
            f'<ellipse cx="14" cy="0" rx="6" ry="9" fill="{CHK_END}" stroke="{OUTLINE}" stroke-width="2.5"/>'
            f'<ellipse cx="14" cy="0" rx="2.6" ry="4.4" fill="{CHK_HOLE}"/>')
    return g(x, y, body, s, rot)


def mark(theme):
    """The Kushi in the 128x128 icon space."""
    st = STICK[theme]
    return (f'<g transform="rotate(-9 64 64)">'
            f'<path d="M64 8L64 120" stroke="{st}" stroke-width="4.5" stroke-linecap="round"/>'
            f'<circle cx="64" cy="8" r="1.4" fill="#f4dfae"/>'
            + konnyaku(64, 35, 1.25, 4) + daikon(64, 71, 1.28, -5, stick=st) + chikuwa(64, 100, 1.22, 6)
            + '</g>')


# The mark's drawn box in icon space is x 36.7..95.9, y 6.4..121.6 (measured
# from a 1280 px render). In the logo it is scaled to 90 high and set 4 from
# the left, 3 from the top.
LOGO_MARK = "translate(-24.6 -2) scale(0.78)"
MARK_RIGHT = 50.2


# --- The family lettering -------------------------------------------------
# Centre-line strokes, width 11, round caps and joins. x-height 44, baseline
# 84, ascender 18; a bowl is a radius-20 circle centred on y 64. Glyphs sit
# 15 apart, measured between their centre-line boxes, as in ochazuke's and
# matomeru's logos.
GAP = 15


def glyph(ch, x):
    """Return (path data, advance) for one letter whose left edge is x."""
    if ch == "o":
        return f"M{x+20:g} 44a20 20 0 1 0 0.01 0", 40
    if ch == "a":
        return f"M{x+20:g} 44a20 20 0 1 0 0.01 0M{x+40:g} 44V84", 40
    if ch == "d":
        return f"M{x+20:g} 44a20 20 0 1 0 0.01 0M{x+40:g} 18V84", 40
    if ch == "e":
        return f"M{x:g} 64H{x+40:g}A20 20 0 1 0 {x+34.14:g} 78.14", 40
    if ch == "n":
        return f"M{x:g} 44V84M{x:g} 62a18 18 0 0 1 36 0V84", 36
    if ch == "i":
        return f"M{x:g} 44V84M{x:g} 24v0.01", 0
    if ch == "-":
        return f"M{x:g} 64H{x+16:g}", 16
    raise ValueError(f"no glyph for {ch!r}")


def wordmark(text, x):
    """Return (path data, right edge of the last glyph's centre line)."""
    parts = []
    for ch in text:
        d, adv = glyph(ch, x)
        parts.append(d)
        x += adv + GAP
    return "".join(parts), x - GAP


def svg_open(w, h, title):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w:g} {h:g}" width="{w:g}" height="{h:g}" '
            f'role="img" aria-label="{title}">\n<title>{title}</title>\n')


def icon():
    return (svg_open(128, 128, NAME)
            + f'<rect width="128" height="128" rx="28" fill="{TILE}"/>\n'
            + mark("dark") + "\n</svg>\n")


def logo(theme):
    ink = TILE if theme == "light" else PALE
    start = MARK_RIGHT + 24 + 5.5  # the family's mark-to-word gap, plus half a stroke
    d, right = wordmark(NAME, start)
    w = round(right + 9)  # half a stroke and the family's right margin
    return (svg_open(w, 96, NAME)
            + f'<g transform="{LOGO_MARK}">{mark(theme)}</g>\n'
            + f'<path d="{d}" fill="none" stroke="{ink}" stroke-width="11" '
              f'stroke-linecap="round" stroke-linejoin="round"/>\n</svg>\n')


def main():
    files = {"icon.svg": icon(), "logo-light.svg": logo("light"), "logo-dark.svg": logo("dark")}
    for name, text in files.items():
        (OUT / name).write_text(text)
        print(OUT / name)


if __name__ == "__main__":
    main()
