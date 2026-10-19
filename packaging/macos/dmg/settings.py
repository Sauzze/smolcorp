# dmgbuild's settings for Smolcorp.dmg: the window people see when they open it.
#
# dmgbuild writes the window's layout (.DS_Store) itself, so building the DMG
# never opens Finder or needs AppleScript. scripts/make-dmg.sh runs it as:
#
#   dmgbuild -s packaging/macos/dmg/settings.py -D app=build/Smolcorp.app "Smolcorp" build/Smolcorp.dmg
#
# The icons' positions must match ICON_X and ICON_Y in make-background.py,
# which draws the arrow between them.

import os.path

app = defines.get("app", "build/Smolcorp.app")  # noqa: F821, defined by dmgbuild
# Paths are from the repository's top, where make-dmg.sh runs this.
here = "packaging/macos/dmg"
app_name = os.path.basename(app)

# The volume: compressed, read-only, and named like the app.
format = "UDZO"
compression_level = 9
filesystem = "HFS+"
size = None  # worked out from the app

files = [app]
symlinks = {"Applications": "/Applications"}

# The volume's icon in Finder's sidebar and on the desktop, from the app's.
icon = os.path.join(app, "Contents/Resources/AppIcon.icns")

# The window: 640 x 400 points, nothing but the icons on the background.
# background.png has a background@2x.png beside it, which dmgbuild joins into
# one image for Retina screens.
background = os.path.join(here, "background.png")
window_rect = ((200, 120), (640, 400))
show_status_bar = False
show_tab_view = False
show_toolbar = False
show_pathbar = False
show_sidebar = False
default_view = "icon-view"
show_icon_preview = False
arrange_by = None
icon_size = 128
text_size = 13
label_pos = "bottom"
icon_locations = {
    app_name: (160, 196),
    "Applications": (480, 196),
}
hide_extensions = [app_name]
