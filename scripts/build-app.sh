#!/bin/bash
# Compiles Smolcorp and assembles build/Smolcorp.app.
#
#   scripts/build-app.sh                  for this Mac's processor
#   scripts/build-app.sh universal        for Apple silicon and Intel, as released
#   scripts/build-app.sh --check-signing  says how it would sign, and signs and builds nothing
#
# Used by install.sh and scripts/release.sh. Needs the Xcode Command Line Tools,
# and Go 1.26 or later: the app's engine is the `smolcorp` command inside it.
#
# Every build has the hardened runtime, ad hoc or not: the app, the smolcorp
# command, tmux and smolcorp-spawn. Without it, any program of yours could attach a debugger to
# them, or slip a library in (DYLD_INSERT_LIBRARIES), and read the keys they
# hold in memory or change the guard's answer. scripts/check-runtime.sh checks
# each one after signing, and a build without it stops there.

set -euo pipefail

cd "$(dirname "$0")/.."
MODE="${1:-}"

# With a code-signing identity of your own in your keychain, the app is signed
# with it: the Keychain then knows each rebuild as the same app, and an "Always
# Allow" lasts. Which one: $SMOLCORP_SIGN_IDENTITY, else the name in
# ~/.config/smolcorp/sign-identity, else "Smolcorp Local". Without one, or with
# SMOLCORP_SIGN_IDENTITY=-, an ad-hoc signature, which the Keychain sees as a new
# app every time. Test copies are never signed with yours (scripts/test-copy.sh).
SAVED_IDENTITY="$(head -1 "$HOME/.config/smolcorp/sign-identity" 2>/dev/null || true)"
IDENTITY="${SMOLCORP_SIGN_IDENTITY:-${SAVED_IDENTITY:-Smolcorp Local}}"
ENTITLEMENTS=packaging/macos/Smolcorp.entitlements
# While your signing identity is unlocked, any program of yours can sign a
# changed Smolcorp with it, which then reads Smolcorp's Keychain items. So it
# can live in a keychain of its own (docs/RELEASING.md), named here by its file:
# unlocked only to sign, its password asked in this terminal, and locked again
# when this script ends, however it ends. Never the login keychain, which this
# would lock.
SIGN_KEYCHAIN="${SMOLCORP_SIGN_KEYCHAIN:-}"
if [ -n "$SIGN_KEYCHAIN" ]; then
  case "$SIGN_KEYCHAIN" in
    login.keychain | login.keychain-db | */login.keychain | */login.keychain-db) LOGIN=1 ;;
    *) LOGIN=0 ;;
  esac
  [ "$SIGN_KEYCHAIN" -ef "$HOME/Library/Keychains/login.keychain-db" ] && LOGIN=1
  if [ "$LOGIN" = 1 ]; then
    echo "SMOLCORP_SIGN_KEYCHAIN names your login keychain, which this script would lock as it ends. Give the signing identity a keychain of its own (docs/RELEASING.md)."
    exit 1
  fi
  [ -f "$SIGN_KEYCHAIN" ] || { echo "SMOLCORP_SIGN_KEYCHAIN: there's no keychain at $SIGN_KEYCHAIN."; exit 1; }
fi

# A dry run: how it would sign. Nothing is built, signed, unlocked or locked.
if [ "$MODE" = "--check-signing" ]; then
  [ -f "$ENTITLEMENTS" ] || { echo "$ENTITLEMENTS is missing."; exit 1; }
  if [ "$IDENTITY" = "-" ]; then
    echo "Signing: ad hoc (SMOLCORP_SIGN_IDENTITY is -)."
  elif [ -n "$SIGN_KEYCHAIN" ]; then
    echo "Signing: $IDENTITY, from $SIGN_KEYCHAIN, unlocked only to sign (its password is asked here) and locked again when the build ends."
  elif grep -qF "\"$IDENTITY\"" <<<"$(security find-identity -v -p codesigning 2>/dev/null || true)"; then
    echo "Signing: $IDENTITY, found in your keychains. SMOLCORP_SIGN_KEYCHAIN isn't set, so it stays unlocked for any program of yours (docs/RELEASING.md)."
  else
    echo "Signing: ad hoc ($IDENTITY isn't in your keychains)."
  fi
  echo "Hardened runtime: on, for the app, bin/smolcorp, bin/tmux and bin/smolcorp-spawn, checked by scripts/check-runtime.sh after signing."
  echo "Entitlements: $ENTITLEMENTS, for the app only."
  exit 0
fi

# Without Go the app would have no engine to run, so nothing is built.
if ! command -v go >/dev/null 2>&1; then
  echo "Smolcorp needs Go 1.26 or later to build: its engine is written in Go. Install it (brew install go, or go.dev/dl), then run this again."
  exit 1
fi
APP_NAME="Smolcorp"
EXECUTABLE="AgentDeck" # the Swift package's product, named in Info.plist
APP="build/$APP_NAME.app"

ARCHS=()
[ "$MODE" = "universal" ] && ARCHS=(--arch arm64 --arch x86_64)

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
# Linked for the app: the engine then counts as in Smolcorp.app wherever it runs from, so it never runs a program
# charged to Smolcorp, even with no smolcorp-spawn beside it (internal/disclaim's builtForApp). scripts/release.sh
# stops on an engine without this mark.
FOR_APP="-X github.com/Sauzze/smolcorp/internal/disclaim.builtForApp=smolcorp-spawn-required"
for goarch in arm64 amd64; do
  CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=13.0 GOOS=darwin GOARCH="$goarch" go build -trimpath -buildvcs=false \
    -ldflags "-s -w -X main.version=$CLI_VERSION $FOR_APP" -o "$CLI_TMP/smolcorp-$goarch" ./cmd/smolcorp
done
lipo -create "$CLI_TMP/smolcorp-arm64" "$CLI_TMP/smolcorp-amd64" -output "$APP/Contents/Resources/bin/smolcorp"
chmod 755 "$APP/Contents/Resources/bin/smolcorp"
rm -rf "$CLI_TMP"
# tmux, beside the command, for the same two processors, and the notices of the libraries inside it. scripts/build-tmux.sh
# builds it once and keeps it (build/tmux/), so a rebuild of the app only copies it. It is the same file with or without
# `universal`. If it can't be built (no network the first time, a broken toolchain) the app is built without it and says so:
# it then uses the tmux the Mac has, so a copy of your own never fails over tmux. scripts/release.sh doesn't allow that.
HAS_TMUX=0
echo "tmux (kept after the first build, which takes a few minutes)…"
if scripts/build-tmux.sh >build/tmux-build.log 2>&1; then
  TMUX_BUILT="$(tail -n 1 build/tmux-build.log)"
  if [ -x "$TMUX_BUILT" ] && [ -s "$(dirname "$TMUX_BUILT")/THIRD-PARTY-NOTICES.txt" ]; then
    cp "$TMUX_BUILT" "$APP/Contents/Resources/bin/tmux"
    chmod 755 "$APP/Contents/Resources/bin/tmux"
    mkdir -p "$APP/Contents/Resources/licenses"
    cp "$(dirname "$TMUX_BUILT")/THIRD-PARTY-NOTICES.txt" "$APP/Contents/Resources/licenses/THIRD-PARTY-NOTICES.txt"
    HAS_TMUX=1
  fi
fi
if [ "$HAS_TMUX" = 0 ]; then
  tail -n 15 build/tmux-build.log
  echo "Warning: tmux couldn't be built (build/tmux-build.log has the whole log). Smolcorp is built without it and will use the tmux this Mac has, if any."
fi
# smolcorp-spawn, beside the command, for the same two processors (packaging/macos/smolcorp-spawn.c). Every program the
# engine starts goes through it, so that program holds none of Smolcorp's permissions (Accessibility, the microphone,
# the screen): macOS charges it to itself (internal/disclaim). The engine starts nothing from an app without it, so a
# build without it stops here.
echo "smolcorp-spawn…"
cc -Wall -Wextra -Werror -O2 -arch arm64 -arch x86_64 -mmacosx-version-min=13.0 \
  -o "$APP/Contents/Resources/bin/smolcorp-spawn" packaging/macos/smolcorp-spawn.c
chmod 755 "$APP/Contents/Resources/bin/smolcorp-spawn"
# Signed with the identity chosen above, with the hardened runtime on each, as a
# release is; the app with its entitlements (Apple Events and the mic), which
# the runtime would otherwise refuse it. The command, tmux and smolcorp-spawn are code of their
# own that macOS runs from the app's Resources, so they're signed first, and the
# app's signature then seals them.
SIGN=(codesign --force --options runtime)
if [ "$IDENTITY" != "-" ] && [ -n "$SIGN_KEYCHAIN" ]; then
  # Locked again when this script ends, however it ends: set before it's unlocked.
  trap 'security lock-keychain "$SIGN_KEYCHAIN"' EXIT
  echo "Unlocking $SIGN_KEYCHAIN to sign…"
  security unlock-keychain "$SIGN_KEYCHAIN"
  IDENTITIES="$(security find-identity -v -p codesigning "$SIGN_KEYCHAIN" 2>/dev/null || true)"
  SIGN+=(--keychain "$SIGN_KEYCHAIN")
else
  # Read whole, then searched: a grep -q stopping at its first match would fail the pipe under pipefail.
  IDENTITIES="$(security find-identity -v -p codesigning 2>/dev/null || true)"
fi
if [ "$IDENTITY" != "-" ] && grep -qF "\"$IDENTITY\"" <<<"$IDENTITIES"; then
  [ -f "$APP/Contents/Resources/bin/smolcorp" ] && "${SIGN[@]}" --sign "$IDENTITY" --identifier io.github.sauzze.smolcorp-cli "$APP/Contents/Resources/bin/smolcorp"
  [ "$HAS_TMUX" = 1 ] && "${SIGN[@]}" --sign "$IDENTITY" --identifier io.github.sauzze.smolcorp-tmux "$APP/Contents/Resources/bin/tmux"
  "${SIGN[@]}" --sign "$IDENTITY" --identifier io.github.sauzze.smolcorp-spawn "$APP/Contents/Resources/bin/smolcorp-spawn"
  "${SIGN[@]}" --sign "$IDENTITY" --identifier io.github.sauzze.agentdeck --entitlements "$ENTITLEMENTS" "$APP"
else
  # Named with its keychain and not found there: said, rather than an ad hoc build that looks like the one asked for.
  [ "$IDENTITY" != "-" ] && [ -n "$SIGN_KEYCHAIN" ] && echo "Warning: $IDENTITY isn't in $SIGN_KEYCHAIN, so this build is signed ad hoc."
  [ -f "$APP/Contents/Resources/bin/smolcorp" ] && "${SIGN[@]}" --sign - "$APP/Contents/Resources/bin/smolcorp"
  [ "$HAS_TMUX" = 1 ] && "${SIGN[@]}" --sign - --identifier io.github.sauzze.smolcorp-tmux "$APP/Contents/Resources/bin/tmux"
  "${SIGN[@]}" --sign - --identifier io.github.sauzze.smolcorp-spawn "$APP/Contents/Resources/bin/smolcorp-spawn"
  "${SIGN[@]}" --sign - --entitlements "$ENTITLEMENTS" "$APP"
fi
# The runtime on each, for every processor, and nothing that opens them again: a build without it isn't one.
SIGNED=("$APP" "$APP/Contents/Resources/bin/smolcorp-spawn")
[ -f "$APP/Contents/Resources/bin/smolcorp" ] && SIGNED+=("$APP/Contents/Resources/bin/smolcorp")
[ "$HAS_TMUX" = 1 ] && SIGNED+=("$APP/Contents/Resources/bin/tmux")
scripts/check-runtime.sh "${SIGNED[@]}" || { echo "Stopped: $APP isn't hardened as every build must be (above)."; exit 1; }
