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
# approve it once in System Settings → Privacy & Security. With a Developer ID
# set up (docs/RELEASING.md), two variables change that:
#
#   SMOLCORP_DEVELOPER_ID     "Developer ID Application: Your Name (TEAMID)":
#                             signs the app and the DMG with it, hardened runtime on.
#                             Unset, the one Developer ID Application identity in
#                             the keychain is used, if there's exactly one.
#   SMOLCORP_NOTARY_PROFILE   a notarytool keychain profile: has Apple notarise the
#                             DMG, staples the ticket to the app, then makes the DMG
#                             again around it and has that notarised and stapled too
#
# The app carries tmux (Resources/bin/tmux, built by scripts/build-tmux.sh) and its licence notices; a release
# stops without them, and signs tmux with the Developer ID as it signs the smolcorp command, with no entitlements.
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

# tmux ships in the app (Resources/bin/tmux, for Apple silicon and Intel) with its notices. build-app.sh only warns
# when it can't build tmux, so a copy of your own still gets built; a release stops here instead.
TMUX_FILE="$APP/Contents/Resources/bin/tmux"
[ -f "$TMUX_FILE" ] || { echo "Stopped: $APP has no tmux (Contents/Resources/bin/tmux): scripts/build-tmux.sh couldn't build it, and a release without it isn't made."; exit 1; }
[ "$(lipo -archs "$TMUX_FILE" | tr ' ' '\n' | sort | tr '\n' ' ')" = "arm64 x86_64 " ] || { echo "Stopped: tmux isn't a file for Apple silicon and Intel in one (lipo -archs says: $(lipo -archs "$TMUX_FILE"))."; exit 1; }
[ -s "$APP/Contents/Resources/licenses/THIRD-PARTY-NOTICES.txt" ] || { echo "Stopped: $APP has no Contents/Resources/licenses/THIRD-PARTY-NOTICES.txt, which tmux's licences require."; exit 1; }
# smolcorp-spawn starts every program the engine runs on its own account (internal/disclaim): without it, the engine
# starts nothing.
SPAWN_FILE="$APP/Contents/Resources/bin/smolcorp-spawn"
[ -f "$SPAWN_FILE" ] || { echo "Stopped: $APP has no smolcorp-spawn (Contents/Resources/bin/smolcorp-spawn)."; exit 1; }
[ "$(lipo -archs "$SPAWN_FILE" | tr ' ' '\n' | sort | tr '\n' ' ')" = "arm64 x86_64 " ] || { echo "Stopped: smolcorp-spawn isn't a file for Apple silicon and Intel in one (lipo -archs says: $(lipo -archs "$SPAWN_FILE"))."; exit 1; }
# The engine is linked for the app (build-app.sh's FOR_APP), so it never runs a program charged to Smolcorp, whatever
# its path says.
grep -qaF 'smolcorp-spawn-required' "$APP/Contents/Resources/bin/smolcorp" || { echo "Stopped: $APP's engine (Contents/Resources/bin/smolcorp) isn't linked for the app: build it with scripts/build-app.sh."; exit 1; }

# Developer ID: the screen saver inside first, then the app around it, both
# with the hardened runtime and a secure timestamp, as notarisation needs.
if [ -n "$DEVELOPER_ID" ] && [ "$DMG_ONLY" = 0 ]; then
  echo "Signing with ${DEVELOPER_ID}…"
  check "the screen saver couldn't be signed." \
    codesign --force --options runtime --timestamp --sign "$DEVELOPER_ID" "$APP/Contents/Resources/Smolcorp Office.saver"
  check "the smolcorp command couldn't be signed." \
    codesign --force --options runtime --timestamp --sign "$DEVELOPER_ID" --identifier io.github.sauzze.smolcorp-cli \
    "$APP/Contents/Resources/bin/smolcorp"
  # tmux needs no entitlements (no JIT, no Apple Events, no microphone): none are given.
  check "tmux couldn't be signed." \
    codesign --force --options runtime --timestamp --sign "$DEVELOPER_ID" --identifier io.github.sauzze.smolcorp-tmux \
    "$TMUX_FILE"
  check "smolcorp-spawn couldn't be signed." \
    codesign --force --options runtime --timestamp --sign "$DEVELOPER_ID" --identifier io.github.sauzze.smolcorp-spawn \
    "$SPAWN_FILE"
  check "the app couldn't be signed." codesign --force --options runtime --timestamp --sign "$DEVELOPER_ID" \
    --entitlements packaging/macos/Smolcorp.entitlements "$APP"
  check "tmux's signature doesn't verify." codesign --verify --deep --strict --verbose=2 "$TMUX_FILE"
  check "the app's signature doesn't verify." codesign --verify --deep --strict --verbose=2 "$APP"
  SPAWN_SIGNATURE="$(codesign --display --verbose=2 "$SPAWN_FILE" 2>&1 || true)"
  grep -qF 'flags=0x10000(runtime)' <<<"$SPAWN_SIGNATURE" || { echo "Stopped: smolcorp-spawn is signed without the hardened runtime."; exit 1; }
  grep -qF "Authority=$DEVELOPER_ID" <<<"$SPAWN_SIGNATURE" || { echo "Stopped: smolcorp-spawn isn't signed with ${DEVELOPER_ID}."; exit 1; }
  grep -qF 'Identifier=io.github.sauzze.smolcorp-spawn' <<<"$SPAWN_SIGNATURE" || { echo "Stopped: smolcorp-spawn is signed under another identifier."; exit 1; }
  SPAWN_ENTITLEMENTS="$(codesign --display --entitlements - "$SPAWN_FILE" 2>&1 || true)"
  ! grep -qF '<key>' <<<"$SPAWN_ENTITLEMENTS" || { echo "Stopped: smolcorp-spawn carries entitlements, and needs none."; exit 1; }
  TMUX_SIGNATURE="$(codesign --display --verbose=2 "$TMUX_FILE" 2>&1 || true)"
  grep -qF 'flags=0x10000(runtime)' <<<"$TMUX_SIGNATURE" || { echo "Stopped: tmux is signed without the hardened runtime."; exit 1; }
  grep -qF "Authority=$DEVELOPER_ID" <<<"$TMUX_SIGNATURE" || { echo "Stopped: tmux isn't signed with ${DEVELOPER_ID}."; exit 1; }
  grep -qF 'Identifier=io.github.sauzze.smolcorp-tmux' <<<"$TMUX_SIGNATURE" || { echo "Stopped: tmux is signed under another identifier."; exit 1; }
  TMUX_ENTITLEMENTS="$(codesign --display --entitlements - "$TMUX_FILE" 2>&1 || true)"
  ! grep -qF '<key>' <<<"$TMUX_ENTITLEMENTS" || { echo "Stopped: tmux carries entitlements, and needs none."; exit 1; }
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
  # The DMG, and the app it's made from and the zip will hold, each with its ticket.
  check "the DMG's stapled ticket doesn't validate." xcrun stapler validate "$DMG"
  check "the app's stapled ticket doesn't validate." xcrun stapler validate "$APP"
  # --dmg-only made this DMG around the stapled app: open it read-only and check
  # the app inside, which is the one people drag to Applications.
  if [ "$DMG_ONLY" = 1 ]; then
    MOUNT="$(mktemp -d build/dmg-check.XXXX)"
    check "the DMG can't be opened to check the app in it." \
      hdiutil attach -quiet -readonly -nobrowse -noautoopen -mountpoint "$MOUNT" "$DMG"
    INSIDE_OK=0
    if xcrun stapler validate "$MOUNT/Smolcorp.app" && codesign --verify --deep --strict "$MOUNT/Smolcorp.app"; then
      INSIDE_OK=1
    fi
    hdiutil detach -quiet "$MOUNT" || echo "(couldn't detach $MOUNT: eject it by hand)"
    rmdir "$MOUNT" 2>/dev/null || true
    [ "$INSIDE_OK" = 1 ] || { echo "Stopped: the app inside the DMG has no valid ticket or signature."; exit 1; }
  fi
  check "Gatekeeper refuses the DMG." spctl --assess -vvv --type open --context context:primary-signature "$DMG"
  check "Gatekeeper refuses the app." spctl --assess -vvv --type execute "$APP"
  # How an installer would see it too: said, not a reason to stop, as -t install is for packages more than DMGs.
  spctl --assess -vvv --type install "$DMG" || echo "(spctl -t install on the DMG: see above; -t open is the DMG's own check)"
  # The DMG was made before the app had its ticket, so the app in it has none: make the DMG
  # again around the stapled app, and have that notarised and stapled, as --dmg-only does.
  [ "$DMG_ONLY" = 1 ] || exec env SMOLCORP_DEVELOPER_ID="$DEVELOPER_ID" SMOLCORP_NOTARY_PROFILE="$NOTARY_PROFILE" scripts/release.sh --dmg-only
fi

# No resource forks or extended attributes in the zip: ditto would write them as
# AppleDouble ._ entries, which unzip (Homebrew's, among others) turns into real
# files inside the app, and the seal breaks. The ticket is a file, not an attribute.
rm -f "$ZIP"
ditto -c -k --keepParent --norsrc --noextattr "$APP" "$ZIP"
# The listing is read whole first: grep -q stopping at a match would make unzip fail with SIGPIPE, which
# pipefail turns into "no match".
LISTING="$(unzip -Z1 "$ZIP")"
if grep -E -q '(^|/)\._|^__MACOSX/' <<<"$LISTING"; then
  echo "Stopped: $ZIP holds AppleDouble (._) entries, which break the app's seal when it's unzipped."
  exit 1
fi

echo
SIGNED="signed ad hoc"
[ -n "$DEVELOPER_ID" ] && SIGNED="Developer ID, not notarised"
[ -n "$NOTARY_PROFILE" ] && SIGNED="Developer ID, notarised"
echo "Smolcorp $(/usr/libexec/PlistBuddy -c "Print :CFBundleShortVersionString" "$APP/Contents/Info.plist"): $(lipo -archs "$APP/Contents/MacOS/AgentDeck"), $SIGNED, $("$TMUX_FILE" -V)"
for FILE in "$DMG" "$ZIP"; do
  echo "$FILE ($(du -h "$FILE" | cut -f 1 | tr -d ' '))"
  echo "SHA-256: $(shasum -a 256 "$FILE" | cut -d ' ' -f 1)"
done
