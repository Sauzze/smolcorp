#!/usr/bin/env python3
"""Draws the DMG window's background: background.png and background@2x.png.

A corner of the office in its own pixel art and palette (Resources/web/office.js):
the slate wall, a whiteboard with the app on the left, Applications on the
right and an arrow drawn between them, a plant and a water cooler on the
planks. The art is 160 x 100 pixels, scaled by 4 for the 640 x 400 window and
by 8 for Retina screens.

The PNGs are checked in, so a release doesn't need this. Run it again after a
change, with Pillow (scripts/make-dmg.sh sets up a venv that has it):

    build/dmg-venv/bin/python packaging/macos/dmg/make-background.py

Where the icons go is in settings.py; ICON_X and ICON_Y below must match it.
"""

import os
import sys

from PIL import Image

W, H = 160, 100
SCALE = 4  # art pixels to window points
# The icons' centres in window points, as in settings.py.
ICON_X = (160, 480)
ICON_Y = 196

# The office's palette (office.js PALETTE), and Claude's shirt for the arrow.
WALL, WALL_TOP, WAINSCOT, RAIL = 0x2F3446, 0x252A3A, 0x272B3B, 0x3D4359
BASEBOARD = FRAME = 0x1B1E2A
SILL = 0x8A90A6
WOOD, PLANK_SEAM = 0x6F5440, 0x624A38
WHITEBOARD, MARKER = 0xE9EDF2, 0x3D4359
LEAF, LEAF_DARK, POT = 0x4F9A5A, 0x3B7A46, 0xA0603E
WATER, COOLER = 0x8FC6F0, 0xE2E2E2
ARROW = 0xD97757
PEN_RED, PEN_BLUE, PEN_GREEN = 0xD4140B, 0x3F6FC4, 0x2FA04A
SHADOW = 0x000000

# The office's pixel font (office.js GLYPHS), plus an M it never needed and an N
# a pixel wider, so the two don't read alike.
GLYPHS = {
    "A": ["010", "101", "111", "101", "101"], "C": ["011", "100", "100", "100", "011"],
    "D": ["110", "101", "101", "101", "110"], "G": ["011", "100", "101", "101", "011"],
    "I": ["111", "010", "010", "010", "111"], "L": ["100", "100", "100", "100", "111"],
    "M": ["10001", "11011", "10101", "10001", "10001"],
    "N": ["1001", "1101", "1011", "1001", "1001"], "O": ["010", "101", "101", "101", "010"],
    "P": ["110", "101", "110", "100", "100"], "R": ["110", "101", "110", "101", "101"],
    "S": ["011", "100", "010", "001", "110"], "T": ["111", "010", "010", "010", "010"],
    " ": ["00", "00", "00", "00", "00"],
}


def rgb(c):
    return (c >> 16 & 255, c >> 8 & 255, c & 255)


class Art:
    def __init__(self):
        self.img = Image.new("RGB", (W, H))
        self.px = self.img.load()

    def dot(self, x, y, c, alpha=1.0):
        if not (0 <= x < W and 0 <= y < H):
            return
        if alpha >= 1:
            self.px[x, y] = rgb(c)
        else:
            old, new = self.px[x, y], rgb(c)
            self.px[x, y] = tuple(round(o + (n - o) * alpha) for o, n in zip(old, new))

    def fill(self, x, y, w, h, c, alpha=1.0):
        for yy in range(y, y + h):
            for xx in range(x, x + w):
                self.dot(xx, yy, c, alpha)

    def text(self, s, x, y, c):
        for ch in s:
            glyph = GLYPHS[ch]
            for r, row in enumerate(glyph):
                for k, on in enumerate(row):
                    if on == "1":
                        self.dot(x + k, y + r, c)
            x += len(glyph[0]) + 1

    @staticmethod
    def text_width(s):
        return sum(len(GLYPHS[ch][0]) + 1 for ch in s) - 1


def draw():
    a = Art()

    # The wall, its rail and panelling, the baseboard, and the planks.
    a.fill(0, 0, W, H, WALL)
    a.fill(0, 0, W, 3, WALL_TOP)
    a.fill(0, 68, W, 2, RAIL)
    a.fill(0, 70, W, 13, WAINSCOT)
    for x in range(0, W, 16):  # the panelling's seams
        a.fill(x, 71, 1, 11, BASEBOARD, 0.35)
    a.fill(0, 83, W, 2, BASEBOARD)
    a.fill(0, 85, W, H - 85, WOOD)
    for i, y in enumerate(range(88, H, 4)):
        a.fill(0, y, W, 1, PLANK_SEAM)
        for x in range((i % 2) * 12, W, 24):
            a.fill(x, y - 3, 1, 3, PLANK_SEAM)

    # The whiteboard: an aluminium frame, a shadow on the wall, a pen tray.
    bx, by, bw, bh = 18, 8, 124, 72
    a.fill(bx + 2, by + 2, bw, bh, SHADOW, 0.25)
    a.fill(bx, by, bw, bh, SILL)
    a.fill(bx + 1, by + 1, bw - 2, bh - 2, WHITEBOARD)
    a.fill(bx + 1, by + bh - 2, bw - 2, 1, SILL, 0.25)  # a smudge of old ink
    a.fill(bx + 20, by + bh, bw - 40, 2, SILL)  # the tray
    a.fill(bx + 20, by + bh + 2, bw - 40, 1, FRAME, 0.5)
    for i, pen in enumerate((PEN_RED, PEN_BLUE, PEN_GREEN)):
        a.fill(bx + 30 + i * 8, by + bh - 1, 6, 1, pen)
        a.dot(bx + 35 + i * 8, by + bh - 1, FRAME)

    # The line across the top, centred on the board, Smolcorp in Claude's colour.
    words = [("DRAG ", MARKER), ("SMOLCORP", ARROW), (" TO APPLICATIONS", MARKER)]
    line = "".join(w for w, _ in words)
    x = bx + (bw - Art.text_width(line)) // 2
    for w, c in words:
        a.text(w, x, by + 6, c)
        x += Art.text_width(w) + 1
    # Underlined in marker, a little short of the line, as drawn by hand.
    ux = bx + (bw - Art.text_width(line)) // 2
    a.fill(ux + 2, by + 13, Art.text_width(line) - 4, 1, MARKER, 0.2)

    # The arrow, from the app to Applications: a marker's stroke and a solid head.
    cy = ICON_Y // SCALE
    x0 = ICON_X[0] // SCALE + 22
    x1 = ICON_X[1] // SCALE - 22
    a.fill(x0, cy - 1, x1 - 7 - x0, 3, ARROW)
    for k in range(8):
        a.fill(x1 - k, cy - k, 1, 2 * k + 1, ARROW)

    # A plant in the left corner.
    px, py = 4, 76
    a.fill(px + 1, py + 10, 8, 1, SHADOW, 0.25)
    a.fill(px + 1, py + 4, 7, 6, POT)
    a.fill(px, py + 3, 9, 2, POT)
    a.fill(px + 1, py + 4, 7, 1, FRAME, 0.3)
    for dx, dy, h in ((4, -12, 15), (1, -7, 9), (7, -8, 10), (2, -10, 6), (6, -13, 7)):
        a.fill(px + dx, py + dy, 1, h, LEAF_DARK)
    for dx, dy in ((3, -11), (5, -12), (0, -6), (8, -7), (1, -9), (7, -10), (3, -5), (6, -4), (2, -2), (5, -1)):
        a.fill(px + dx, py + dy, 2, 2, LEAF)

    # A water cooler in the right corner.
    cx, cy2 = 146, 62
    a.fill(cx + 1, cy2 + 30, 11, 1, SHADOW, 0.25)
    a.fill(cx + 2, cy2, 8, 9, WATER)
    a.fill(cx + 3, cy2 + 1, 2, 6, 0xFFFFFF, 0.45)
    a.fill(cx + 4, cy2 + 9, 4, 1, COOLER)
    a.fill(cx, cy2 + 10, 12, 20, COOLER)
    a.fill(cx, cy2 + 10, 12, 1, SILL)
    a.fill(cx + 4, cy2 + 15, 4, 2, FRAME)
    a.fill(cx + 1, cy2 + 21, 10, 1, SILL, 0.6)

    return a.img


def main():
    here = os.path.dirname(os.path.abspath(__file__))
    out = sys.argv[1] if len(sys.argv) > 1 else here
    art = draw()
    for scale, name in ((SCALE, "background.png"), (SCALE * 2, "background@2x.png")):
        img = art.resize((W * scale, H * scale), Image.NEAREST)
        # 72 and 144 dpi, so Finder knows which is which.
        dpi = 72 * scale // SCALE
        img.save(os.path.join(out, name), dpi=(dpi, dpi), optimize=True)
        print(os.path.join(out, name), img.size)


if __name__ == "__main__":
    main()
