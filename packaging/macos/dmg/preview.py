#!/usr/bin/env python3
"""Draws what the DMG's window should look like, without opening Finder.

Lays the icons over background@2x.png where settings.py puts them, with their
labels, at Retina size (2 pixels a point), and saves it as a PNG:

    build/dmg-venv/bin/python packaging/macos/dmg/preview.py build/dmg-preview.png

Needs Pillow in the venv (build/dmg-venv/bin/pip install pillow) and macOS's
sips, which turns the .icns icons into PNGs. A preview, not a screenshot: Finder
draws the labels in its own font, and the Applications arrow badge comes from
the alias itself.
"""

import os
import subprocess
import sys
import tempfile

from PIL import Image, ImageDraw, ImageFont

SCALE = 2
HERE = "packaging/macos/dmg"
APPLICATIONS_ICON = "/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/ApplicationsFolderIcon.icns"
ALIAS_BADGE = "/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/AliasBadgeIcon.icns"


def settings():
    values = {"defines": {"app": "build/Smolcorp.app"}}
    with open(os.path.join(HERE, "settings.py")) as f:
        exec(compile(f.read(), "settings.py", "exec"), values)
    return values


def icon(path, size, work):
    out = os.path.join(work, os.path.basename(path) + ".png")
    subprocess.run(["sips", "-s", "format", "png", "-Z", str(size), path, "--out", out],
                   check=True, stdout=subprocess.DEVNULL)
    return Image.open(out).convert("RGBA").resize((size, size), Image.LANCZOS)


def font(size):
    for path in ("/System/Library/Fonts/SFNS.ttf", "/System/Library/Fonts/Helvetica.ttc"):
        if os.path.exists(path):
            return ImageFont.truetype(path, size)
    return ImageFont.load_default()


def main():
    out = sys.argv[1] if len(sys.argv) > 1 else "build/dmg-preview.png"
    s = settings()
    (_, (w, h)) = s["window_rect"]
    size = int(s["icon_size"] * SCALE)
    canvas = Image.open(os.path.join(HERE, "background@2x.png")).convert("RGBA").resize((w * SCALE, h * SCALE))
    draw = ImageDraw.Draw(canvas)
    label_font = font(int(s["text_size"] * SCALE))

    with tempfile.TemporaryDirectory() as work:
        app_icon = icon("Resources/AppIcon.icns", size, work)
        apps_icon = icon(APPLICATIONS_ICON, size, work)
        apps_icon.alpha_composite(icon(ALIAS_BADGE, size, work))
        app_name = os.path.basename(s["app"])
        shown = {app_name: (app_icon, os.path.splitext(app_name)[0]), "Applications": (apps_icon, "Applications")}
        for name, (x, y) in s["icon_locations"].items():
            img, label = shown[name]
            cx, cy = x * SCALE, y * SCALE
            canvas.alpha_composite(img, (cx - size // 2, cy - size // 2))
            tw = draw.textlength(label, font=label_font)
            draw.text((cx - tw / 2, cy + size // 2 + 4 * SCALE), label, font=label_font, fill=(0, 0, 0, 255))

    canvas.save(out)
    print(out, canvas.size)


if __name__ == "__main__":
    main()
