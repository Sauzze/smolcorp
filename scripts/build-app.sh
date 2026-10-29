#!/bin/bash
# Compiles Smolcorp and assembles build/Smolcorp.app.
#
#   scripts/build-app.sh             for this Mac's processor
#   scripts/build-app.sh universal   for Apple silicon and Intel, as released
#
# Used by install.sh and scripts/release.sh. Needs the Xcode Command Line Tools.

set -euo pipefail

cd "$(dirname "$0")/.."
APP_NAME="Smolcorp"
EXECUTABLE="AgentDeck" # the Swift package's product, named in Info.plist
APP="build/$APP_NAME.app"

ARCHS=()
[ "${1:-}" = "universal" ] && ARCHS=(--arch arm64 --arch x86_64)

# The phone's viewer draws with a copy of the office: an app whose office has
# moved on without it would show the phone another room.
viewer/scripts/sync-office.sh --check

echo "Building…"
mkdir -p build
if ! swift build -c release ${ARCHS[@]+"${ARCHS[@]}"} >build/build.log 2>&1; then
  cat build/build.log
  echo
  echo "The build failed. The messages above say why."
  exit 1
fi
BIN="$(swift build -c release ${ARCHS[@]+"${ARCHS[@]}"} --show-bin-path)/$EXECUTABLE"
scripts/build-saver.sh "${1:-}"

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$BIN" "$APP/Contents/MacOS/$EXECUTABLE"
# Debug symbols name the folder the app was built in, which a download has no
# business carrying. Stripping changes the binary, so it comes before signing.
strip -S -no_code_signature_warning "$APP/Contents/MacOS/$EXECUTABLE"
cp Resources/Info.plist "$APP/Contents/"
cp Resources/AppIcon.icns Resources/report.sh Resources/opencode-plugin.js Resources/opencode-plugin-v1.js "$APP/Contents/Resources/"
cp -R Resources/web "$APP/Contents/Resources/"
cp -R Resources/briefs "$APP/Contents/Resources/"
# Signed already, so the app's signature seals it as it is.
cp -R "build/Smolcorp Office.saver" "$APP/Contents/Resources/"
# With a code-signing identity of your own in your keychain, the app is signed
# with it: the Keychain then knows each rebuild as the same app, and an "Always
# Allow" lasts. Which one: $SMOLCORP_SIGN_IDENTITY, else the name in
# ~/.config/smolcorp/sign-identity, else "Smolcorp Local". Without one, an
# ad-hoc signature, which the Keychain sees as a new app every time.
SAVED_IDENTITY="$(head -1 "$HOME/.config/smolcorp/sign-identity" 2>/dev/null || true)"
IDENTITY="${SMOLCORP_SIGN_IDENTITY:-${SAVED_IDENTITY:-Smolcorp Local}}"
# Read whole, then searched: a grep -q stopping at its first match would fail the pipe under pipefail.
IDENTITIES="$(security find-identity -v -p codesigning 2>/dev/null || true)"
if grep -qF "\"$IDENTITY\"" <<<"$IDENTITIES"; then
  codesign --force --sign "$IDENTITY" --identifier io.github.sauzze.agentdeck "$APP"
else
  codesign --force --sign - "$APP"
fi
