#!/bin/bash
# Compiles Smolcorp and assembles build/Smolcorp.app.
#
#   scripts/build-app.sh             for this Mac's processor
#   scripts/build-app.sh universal   for Apple silicon and Intel, as released
#
# Used by install.sh and scripts/release.sh. Needs the Xcode Command Line Tools,
# and Go 1.26 or later: the app's engine is the `smolcorp` command inside it.

set -euo pipefail

cd "$(dirname "$0")/.."
# Without Go the app would have no engine to run, so nothing is built.
if ! command -v go >/dev/null 2>&1; then
  echo "Smolcorp needs Go 1.26 or later to build: its engine is written in Go. Install it (brew install go, or go.dev/dl), then run this again."
  exit 1
fi
APP_NAME="Smolcorp"
EXECUTABLE="AgentDeck" # the Swift package's product, named in Info.plist
APP="build/$APP_NAME.app"

ARCHS=()
[ "${1:-}" = "universal" ] && ARCHS=(--arch arm64 --arch x86_64)

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
# office.js is built from Resources/web/office (scripts/buildoffice), and
# committed: checked here, never rebuilt, so a change made in office.js itself
# isn't lost. The pages, office.js and the briefs are inside the `smolcorp`
# command below (go:embed, web.go), so the app carries no copy of Resources/web
# or Resources/briefs: nothing reads one. The GM's plugin stays a folder, as the
# command finds it beside itself (desktop/hirestart.go).
go run ./scripts/buildoffice -check
cp -R Resources/gm "$APP/Contents/Resources/"
# Signed already, so the app's signature seals it as it is.
cp -R "build/Smolcorp Office.saver" "$APP/Contents/Resources/"
# The `smolcorp` command, for Apple silicon and Intel in one file: the app's
# engine (`smolcorp core`, which the app starts and is the shell of), and the
# GM's hands and the guard. Setup can link it into Terminal. Signed with the app
# below. cgo is on because the tray the other computers use needs it to build;
# on the Mac only the app starts the engine, so the command refuses to start an
# app of its own (cmd/smolcorp/start_darwin.go).
mkdir -p "$APP/Contents/Resources/bin"
CLI_VERSION="$(sed -n '/CFBundleShortVersionString/{n;s/.*<string>\(.*\)<\/string>.*/\1/p;}' Resources/Info.plist)"
CLI_TMP="$(mktemp -d)"
for goarch in arm64 amd64; do
  CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=13.0 GOOS=darwin GOARCH="$goarch" go build -trimpath -buildvcs=false \
    -ldflags "-s -w -X main.version=$CLI_VERSION" -o "$CLI_TMP/smolcorp-$goarch" ./cmd/smolcorp
done
lipo -create "$CLI_TMP/smolcorp-arm64" "$CLI_TMP/smolcorp-amd64" -output "$APP/Contents/Resources/bin/smolcorp"
chmod 755 "$APP/Contents/Resources/bin/smolcorp"
rm -rf "$CLI_TMP"
# With a code-signing identity of your own in your keychain, the app is signed
# with it: the Keychain then knows each rebuild as the same app, and an "Always
# Allow" lasts. Which one: $SMOLCORP_SIGN_IDENTITY, else the name in
# ~/.config/smolcorp/sign-identity, else "Smolcorp Local". Without one, an
# ad-hoc signature, which the Keychain sees as a new app every time.
SAVED_IDENTITY="$(head -1 "$HOME/.config/smolcorp/sign-identity" 2>/dev/null || true)"
IDENTITY="${SMOLCORP_SIGN_IDENTITY:-${SAVED_IDENTITY:-Smolcorp Local}}"
# Read whole, then searched: a grep -q stopping at its first match would fail the pipe under pipefail.
IDENTITIES="$(security find-identity -v -p codesigning 2>/dev/null || true)"
# The command is code of its own that macOS runs from the app's Resources, so it
# is signed first, with the same identity, and the app's signature then seals it.
if grep -qF "\"$IDENTITY\"" <<<"$IDENTITIES"; then
  [ -f "$APP/Contents/Resources/bin/smolcorp" ] && codesign --force --sign "$IDENTITY" --identifier io.github.sauzze.smolcorp-cli "$APP/Contents/Resources/bin/smolcorp"
  codesign --force --sign "$IDENTITY" --identifier io.github.sauzze.agentdeck "$APP"
else
  [ -f "$APP/Contents/Resources/bin/smolcorp" ] && codesign --force --sign - "$APP/Contents/Resources/bin/smolcorp"
  codesign --force --sign - "$APP"
fi
