#!/bin/bash
# Fetches chrome-headless-shell, Google's headless-only Chrome, for the Mac desk rows that look at a page in a
# sandboxed desk (internal/sandbox/headlessshell_darwin_test.go) and for CI's Mac sandbox job. It is a plain program
# in a plain folder, no .app bundle: it's run by its path, never through LaunchServices (T126HEADLESSSHELL1).
# It comes from Chrome for Testing, at a pinned version, and is checked against
# third_party/chrome-headless-shell/SHA256SUMS, as scripts/fetch-js-tools.sh checks pnpm and bun. A mismatch stops.
#
#   scripts/fetch-headless-shell.sh DIR    puts chrome-headless-shell for this Mac's processor in DIR and prints
#                                          the program's full path as its last line. A download already in DIR is
#                                          checked again, never trusted. SMOLCORP_TEST_CHROME=<that path> runs the
#                                          rows on it.
#
# Where SHA256SUMS's lines came from (15 Nov 2026, the Stable channel of Chrome for Testing that day):
#   - the files themselves, downloaded from the URL below and hashed with `shasum -a 256`;
#   - their MD5s matched Google's own for each file, read from the storage's object record
#     (https://storage.googleapis.com/storage/v1/b/chrome-for-testing-public/o/<name>, its md5Hash);
#   - the version and the URLs are the ones chrome-for-testing's last-known-good-versions-with-downloads.json gave.
#
# Only for a Mac. Nothing is installed: DIR is the caller's.

set -euo pipefail

[ $# -eq 1 ] || { echo "usage: scripts/fetch-headless-shell.sh DIR" >&2; exit 2; }
SUMS="$(cd "$(dirname "$0")/.." && pwd -P)/third_party/chrome-headless-shell/SHA256SUMS"
mkdir -p "$1"
DIR="$(cd "$1" && pwd -P)"

VERSION=157.0.8103.78
case "$(uname -s)/$(uname -m)" in
  Darwin/arm64) PLATFORM=mac-arm64 ;;
  Darwin/x86_64) PLATFORM=mac-x64 ;;
  *) echo "scripts/fetch-headless-shell.sh is for a Mac, not $(uname -s)/$(uname -m)." >&2; exit 1 ;;
esac
NAME="$VERSION/chrome-headless-shell-$PLATFORM.zip"
URL="https://storage.googleapis.com/chrome-for-testing-public/$VERSION/$PLATFORM/chrome-headless-shell-$PLATFORM.zip"
FOLDER="chrome-headless-shell-$PLATFORM"

want="$(awk -v f="$NAME" '$2 == f { print $1 }' "$SUMS")"
[ -n "$want" ] || { echo "third_party/chrome-headless-shell/SHA256SUMS has no line for $NAME." >&2; exit 1; }
zip="$DIR/$(basename "$NAME")"
if [ ! -f "$zip" ]; then
  echo "Downloading $NAME from $(echo "$URL" | cut -d/ -f3)" >&2
  curl -fsSL --proto '=https' --proto-redir '=https' --retry 3 --connect-timeout 20 -o "$zip.part" "$URL"
  mv "$zip.part" "$zip"
fi
if [ "$(shasum -a 256 "$zip" | cut -d' ' -f1)" != "$want" ]; then
  rm -f "$zip"
  echo "$NAME doesn't match third_party/chrome-headless-shell/SHA256SUMS: stopped, and the file removed." >&2
  exit 1
fi

# The program, from the checked file only, and never from a bundle: a .app anywhere in it stops here.
if unzip -Z1 "$zip" | grep -qE '\.app(/|$)'; then
  echo "$NAME holds an .app bundle: stopped (a plain program only, run by its path)." >&2
  exit 1
fi
rm -rf "${DIR:?}/$FOLDER"
unzip -q "$zip" "$FOLDER/*" -d "$DIR"
program="$DIR/$FOLDER/chrome-headless-shell"
[ -f "$program" ] && [ -x "$program" ] || { echo "$NAME has no $FOLDER/chrome-headless-shell." >&2; exit 1; }
echo "$program"
