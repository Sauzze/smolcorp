#!/bin/bash
# Builds the downloads for a GitHub release from one universal app for Apple
# silicon and Intel Macs, and prints their SHA-256s for the release notes:
#
#   build/Smolcorp.dmg   open it and drag Smolcorp to Applications
#   build/Smolcorp.zip   the same app, for the Homebrew cask and older links
#
#   scripts/release.sh
#   scripts/release.sh --check-signing   says how it would sign, and signs and builds nothing
#   scripts/release.sh --dmg-only        keeps build/Smolcorp.app, already signed, notarised
#                                        and stapled, and makes, signs, notarises and
#                                        staples the DMG around it, then the zip
#
# By default the app is signed ad hoc, not notarised, so macOS asks people to
# approve it once in System Settings → Privacy & Security. The README explains
# how. With a Developer ID set up (docs/RELEASING.md), two variables change that:
#
#   SMOLCORP_DEVELOPER_ID     "Developer ID Application: Your Name (TEAMID)":
#                             signs the app and the DMG with it, hardened runtime on.
#                             Unset, the one Developer ID Application identity in
#                             the keychain is used, if there's exactly one.
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

# What a command says is read whole before it's searched: with pipefail, a
# grep -q that stops at its first match would fail the command it reads from,
# and set -e would end the run there, saying nothing.
IDENTITIES="$(security find-identity -v -p codesigning 2>/dev/null || true)"
has_identity() { grep -qF "\"$1\"" <<<"$IDENTITIES"; }

# `check "why" command…` runs the command, and if it fails, says why the
# release stopped there.
check() {
  local why="$1"
  shift
  "$@" || { echo "Stopped: $why"; exit 1; }
}

# With none named, the keychain's own Developer ID, where it has one and only
# one; with none there, ad hoc, as before.
if [ -z "$DEVELOPER_ID" ]; then
  FOUND="$(sed -n 's/^ *[0-9]*) [0-9A-F]* "\(Developer ID Application: .*\)"$/\1/p' <<<"$IDENTITIES" | sort -u)"
  case "$(printf '%s' "$FOUND" | grep -c .)" in
    0) ;;
    1) DEVELOPER_ID="$FOUND" ;;
    *) echo "Several Developer ID Application identities are in the keychain: name one with SMOLCORP_DEVELOPER_ID."
       printf '%s\n' "$FOUND"
       exit 1 ;;
  esac
fi

if [ -n "$NOTARY_PROFILE" ] && [ -z "$DEVELOPER_ID" ]; then
  echo "SMOLCORP_NOTARY_PROFILE is set but SMOLCORP_DEVELOPER_ID isn't: Apple only notarises a Developer ID app."
  exit 1
fi

# A dry run, before the real one: what's set, and whether this Mac has it.
# Nothing is signed, built or sent to Apple.
if [ "${1:-}" = "--check-signing" ]; then
  ready=1
  if [ -z "$DEVELOPER_ID" ]; then
    echo "Signing: ad hoc (SMOLCORP_DEVELOPER_ID isn't set). macOS will ask people to approve the app once."
  elif has_identity "$DEVELOPER_ID"; then
    echo "Signing: $DEVELOPER_ID, found in the keychain, with the hardened runtime and packaging/macos/Smolcorp.entitlements."
  else
    echo "Signing: $DEVELOPER_ID is NOT in the keychain. security find-identity -v -p codesigning lists what is."
    ready=0
  fi
  if [ -z "$NOTARY_PROFILE" ]; then
    echo "Notarising: no (SMOLCORP_NOTARY_PROFILE isn't set)."
  elif xcrun --find notarytool >/dev/null 2>&1 && xcrun --find stapler >/dev/null 2>&1; then
    echo "Notarising: with the keychain profile \"$NOTARY_PROFILE\" (docs/RELEASING.md says how it's made; it's checked when used)."
  else
    echo "Notarising: notarytool or stapler isn't installed (Xcode's command line tools have them)."
    ready=0
  fi
  [ -f packaging/macos/Smolcorp.entitlements ] || { echo "packaging/macos/Smolcorp.entitlements is missing."; ready=0; }
  [ "$ready" = 1 ] && echo "Ready." || { echo "Not ready."; exit 1; }
  exit 0
fi

# --dmg-only: the app from the last run is kept as it is. It must be this
# Developer ID's, with the hardened runtime and its ticket stapled.
DMG_ONLY=0
if [ "${1:-}" = "--dmg-only" ]; then
  DMG_ONLY=1
  [ -n "$DEVELOPER_ID" ] || { echo "--dmg-only needs a Developer ID: none is set or in the keychain."; exit 1; }
  [ -n "$NOTARY_PROFILE" ] || { echo "--dmg-only needs SMOLCORP_NOTARY_PROFILE: the DMG is made to be notarised."; exit 1; }
  [ -d "$APP" ] || { echo "--dmg-only needs $APP from a full run: run scripts/release.sh."; exit 1; }
  SIGNATURE="$(codesign --display --verbose=2 "$APP" 2>&1 || true)"
  grep -qF "Authority=$DEVELOPER_ID" <<<"$SIGNATURE" || { echo "Stopped: $APP isn't signed with ${DEVELOPER_ID}: run scripts/release.sh."; exit 1; }
  grep -qF 'flags=0x10000(runtime)' <<<"$SIGNATURE" || { echo "Stopped: $APP is signed without the hardened runtime: run scripts/release.sh."; exit 1; }
  check "$APP's signature doesn't verify: run scripts/release.sh." codesign --verify --deep --strict "$APP"
  check "$APP has no valid ticket stapled: run scripts/release.sh." xcrun stapler validate "$APP"
  echo "Keeping $APP ($(/usr/libexec/PlistBuddy -c "Print :CFBundleShortVersionString" "$APP/Contents/Info.plist")), notarised and stapled."
fi

# A release is built from what's committed, and nothing else: no changes
# waiting, and no stray files, ignored or not, beside the pages it seals in.
if [ -n "$(git status --porcelain)" ] || [ -n "$(git ls-files --others --ignored --exclude-standard Resources/web)" ]; then
  echo "Commit or set aside these first: a release is built from what's committed."
  git status --short
  git ls-files --others --ignored --exclude-standard Resources/web
  exit 1
fi
# Nothing of the maintainer's own goes out in a download.
scripts/check-public.sh
# A Developer ID named must be there, before anything is built.
if [ -n "$DEVELOPER_ID" ] && ! has_identity "$DEVELOPER_ID"; then
  echo "$DEVELOPER_ID is not in the keychain: security find-identity -v -p codesigning lists what is."
  exit 1
fi
# Ad hoc to start with, whatever build-app.sh would pick for a copy of your
# own: a release is signed ad hoc or with the Developer ID, never with a
# development certificate.
[ "$DMG_ONLY" = 1 ] || SMOLCORP_SIGN_IDENTITY=- scripts/build-app.sh universal

# Developer ID: the screen saver inside first, then the app around it, both
# with the hardened runtime and a secure timestamp, as notarisation needs.
if [ -n "$DEVELOPER_ID" ] && [ "$DMG_ONLY" = 0 ]; then
  echo "Signing with ${DEVELOPER_ID}…"
  check "the screen saver couldn't be signed." \
    codesign --force --options runtime --timestamp --sign "$DEVELOPER_ID" "$APP/Contents/Resources/Smolcorp Office.saver"
  check "the app couldn't be signed." codesign --force --options runtime --timestamp --sign "$DEVELOPER_ID" \
    --entitlements packaging/macos/Smolcorp.entitlements "$APP"
  check "the app's signature doesn't verify." codesign --verify --deep --strict --verbose=2 "$APP"
  # The hardened runtime on, and the entitlements it was given
  SIGNATURE="$(codesign --display --verbose=2 "$APP" 2>&1 || true)"
  grep -qF 'flags=0x10000(runtime)' <<<"$SIGNATURE" || { echo "Stopped: the app is signed without the hardened runtime."; exit 1; }
  echo "Hardened runtime: on."
  check "the app's entitlements can't be read." codesign --display --entitlements - "$APP"
fi

# make-dmg.sh signs the DMG with the Developer ID it's given, found here or
# named. The DMG is signed before Apple sees it, so the ticket is for the
# signed DMG and Gatekeeper accepts it.
SMOLCORP_DEVELOPER_ID="$DEVELOPER_ID" scripts/make-dmg.sh
if [ -n "$DEVELOPER_ID" ]; then
  SIGNATURE="$(codesign --display --verbose=2 "$DMG" 2>&1 || true)"
  grep -qF "Authority=$DEVELOPER_ID" <<<"$SIGNATURE" || { echo "Stopped: the DMG isn't signed with ${DEVELOPER_ID}."; exit 1; }
  echo "DMG signed: ${DEVELOPER_ID}."
fi

# Notarising the DMG covers the app inside it too, so the app gets the same
# ticket, stapled before it's zipped.
if [ -n "$DEVELOPER_ID" ] && [ -z "$NOTARY_PROFILE" ]; then
  echo "Not notarised: SMOLCORP_NOTARY_PROFILE isn't set (docs/RELEASING.md says how to make one)."
fi
if [ -n "$NOTARY_PROFILE" ]; then
  echo "Notarising (this takes a few minutes)…"
  check "notarytool couldn't submit the DMG, or Apple didn't accept it (xcrun notarytool log <id> --keychain-profile $NOTARY_PROFILE says why)." \
    xcrun notarytool submit "$DMG" --keychain-profile "$NOTARY_PROFILE" --wait
  check "the DMG's ticket couldn't be stapled: was it accepted?" xcrun stapler staple "$DMG"
  # --dmg-only's app has its ticket already, and is left as it is.
  [ "$DMG_ONLY" = 1 ] || check "the app's ticket couldn't be stapled." xcrun stapler staple "$APP"
  # As a download is checked: the DMG and the app it holds, each with its ticket.
  check "the DMG's stapled ticket doesn't validate." xcrun stapler validate "$DMG"
  check "the app's stapled ticket doesn't validate." xcrun stapler validate "$APP"
  check "Gatekeeper refuses the DMG." spctl --assess -vvv --type open --context context:primary-signature "$DMG"
  check "Gatekeeper refuses the app." spctl --assess -vvv --type execute "$APP"
  # How an installer would see it too: said, not a reason to stop, as -t install is for packages more than DMGs.
  spctl --assess -vvv --type install "$DMG" || echo "(spctl -t install on the DMG: see above; -t open is the DMG's own check)"
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
