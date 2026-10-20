#!/bin/bash
# Builds the downloads for a GitHub release from one universal app for Apple
# silicon and Intel Macs, and prints their SHA-256s for the release notes:
#
#   build/Smolcorp.dmg   open it and drag Smolcorp to Applications
#   build/Smolcorp.zip   the same app, for the Homebrew cask and older links
#
#   scripts/release.sh
#
# By default the app is signed ad hoc, not notarised, so macOS asks people to
# approve it once in System Settings → Privacy & Security. The README explains
# how. With a Developer ID set up (docs/RELEASING.md), two variables change that:
#
#   SMOLCORP_DEVELOPER_ID     "Developer ID Application: Your Name (TEAMID)":
#                             signs the app and the DMG with it, hardened runtime on
#   SMOLCORP_NOTARY_PROFILE   a notarytool keychain profile: has Apple notarise the
#                             DMG, then staples the ticket to the DMG and the app
#
# The DMG's one-time setup (a venv in build/) is in scripts/make-dmg.sh.

set -euo pipefail

cd "$(dirname "$0")/.."
APP="build/Smolcorp.app"
DMG="build/Smolcorp.dmg"
ZIP="build/Smolcorp.zip"
DEVELOPER_ID="${SMOLCORP_DEVELOPER_ID:-}"
NOTARY_PROFILE="${SMOLCORP_NOTARY_PROFILE:-}"

if [ -n "$NOTARY_PROFILE" ] && [ -z "$DEVELOPER_ID" ]; then
  echo "SMOLCORP_NOTARY_PROFILE is set but SMOLCORP_DEVELOPER_ID isn't: Apple only notarises a Developer ID app."
  exit 1
fi

# A release is built from what's committed, and nothing else: no changes
# waiting, and no stray files, ignored or not, beside the pages it seals in.
if [ -n "$(git status --porcelain)" ] || [ -n "$(git ls-files --others --ignored --exclude-standard Resources/web)" ]; then
  echo "Commit or set aside these first: a release is built from what's committed."
  git status --short
  git ls-files --others --ignored --exclude-standard Resources/web
  exit 1
fi
scripts/build-app.sh universal

# Developer ID: the screen saver inside first, then the app around it, both
# with the hardened runtime and a secure timestamp, as notarisation needs.
if [ -n "$DEVELOPER_ID" ]; then
  echo "Signing with $DEVELOPER_ID…"
  codesign --force --options runtime --timestamp --sign "$DEVELOPER_ID" "$APP/Contents/Resources/Smolcorp Office.saver"
  codesign --force --options runtime --timestamp --sign "$DEVELOPER_ID" \
    --entitlements packaging/macos/Smolcorp.entitlements "$APP"
  codesign --verify --deep --strict "$APP"
fi

scripts/make-dmg.sh

# Notarising the DMG covers the app inside it too, so the app gets the same
# ticket, stapled before it's zipped.
if [ -n "$NOTARY_PROFILE" ]; then
  echo "Notarising (this takes a few minutes)…"
  xcrun notarytool submit "$DMG" --keychain-profile "$NOTARY_PROFILE" --wait
  xcrun stapler staple "$DMG"
  xcrun stapler staple "$APP"
  spctl --assess --type open --context context:primary-signature "$DMG"
  spctl --assess --type execute "$APP"
fi

rm -f "$ZIP"
ditto -c -k --keepParent "$APP" "$ZIP"

echo
SIGNED="signed ad hoc"
[ -n "$DEVELOPER_ID" ] && SIGNED="Developer ID, not notarised"
[ -n "$NOTARY_PROFILE" ] && SIGNED="Developer ID, notarised"
echo "Smolcorp $(/usr/libexec/PlistBuddy -c "Print :CFBundleShortVersionString" "$APP/Contents/Info.plist"): $(lipo -archs "$APP/Contents/MacOS/AgentDeck"), $SIGNED"
for FILE in "$DMG" "$ZIP"; do
  echo "$FILE ($(du -h "$FILE" | cut -f 1 | tr -d ' '))"
  echo "SHA-256: $(shasum -a 256 "$FILE" | cut -d ' ' -f 1)"
done
