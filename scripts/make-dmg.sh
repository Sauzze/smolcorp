#!/bin/bash
# Makes build/Smolcorp.dmg from build/Smolcorp.app: open it, and a window shows
# Smolcorp on a whiteboard in the office with an arrow to Applications.
#
#   scripts/make-dmg.sh
#
# Used by scripts/release.sh, after scripts/build-app.sh has built the app.
#
# The window's layout is written by dmgbuild (https://github.com/dmgbuild/dmgbuild),
# not by Finder: nothing opens on screen, no AppleScript, and the disk image is
# attached with -nobrowse while it's made. The layout is in
# packaging/macos/dmg/settings.py, and the background beside it.
#
# One-time setup, done for you the first time this runs: a Python venv at
# build/dmg-venv with dmgbuild in it. It needs Python 3 (the Xcode Command Line
# Tools have it) and the internet, once. Nothing is installed outside build/,
# and deleting build/dmg-venv undoes it.
#
# With SMOLCORP_DEVELOPER_ID set, the DMG is signed with that Developer ID too.
# See docs/RELEASING.md.

set -euo pipefail

cd "$(dirname "$0")/.."
APP="build/Smolcorp.app"
DMG="build/Smolcorp.dmg"
VENV="build/dmg-venv"
DMGBUILD_VERSION="1.6.7"

if [ ! -d "$APP" ]; then
  echo "No $APP to put in the DMG. Run scripts/build-app.sh universal first."
  exit 1
fi

if ! "$VENV/bin/python" -c "import dmgbuild" 2>/dev/null; then
  echo "Setting up dmgbuild in $VENV (once)…"
  rm -rf "$VENV"
  python3 -m venv "$VENV"
  "$VENV/bin/python" -m pip install --quiet --disable-pip-version-check "dmgbuild==$DMGBUILD_VERSION"
fi

echo "Making the DMG…"
rm -f "$DMG"
"$VENV/bin/dmgbuild" -s packaging/macos/dmg/settings.py -D app="$APP" "Smolcorp" "$DMG" >build/dmg.log 2>&1 || {
  cat build/dmg.log
  echo
  echo "The DMG wasn't made. The messages above say why."
  exit 1
}
hdiutil verify -quiet "$DMG"

if [ -n "${SMOLCORP_DEVELOPER_ID:-}" ]; then
  codesign --force --timestamp --sign "$SMOLCORP_DEVELOPER_ID" "$DMG"
  codesign --verify --strict "$DMG"
fi
