#!/bin/bash
# Fetches pnpm and bun for CI's Mac sandbox job (.github/workflows/ci.yml, mac-sandbox), whose normal-work rows
# install with them. The runner's image has node and npm but neither of these, and the job reaches no package
# registry: each comes from its own project's release on github.com, at a pinned version, and is checked against
# third_party/js-tools/SHA256SUMS, as scripts/build-tmux.sh checks its tarballs. A mismatch stops the job.
#
#   scripts/fetch-js-tools.sh DIR    puts `pnpm` and `bun` for this Mac's processor in DIR and prints DIR as its
#                                    last line. A download already in DIR is checked again, never trusted.
#
# Where SHA256SUMS's lines came from (15 Nov 2026, each the same from all three):
#   - the files themselves, downloaded from the release URLs below and hashed with `shasum -a 256`;
#   - GitHub's own digest of each release file (`gh release view <tag> --json assets`, its `digest`);
#   - for bun, its release's SHASUMS256.txt. pnpm's releases publish none.
# The bun inside bun-darwin-aarch64.zip is byte for byte the bun 1.4.2 the rows were first run with.
#
# Only for a Mac. Nothing is installed: DIR is the caller's (CI puts it on PATH for the next step).

set -euo pipefail

[ $# -eq 1 ] || { echo "usage: scripts/fetch-js-tools.sh DIR" >&2; exit 2; }
SUMS="$(cd "$(dirname "$0")/.." && pwd -P)/third_party/js-tools/SHA256SUMS"
mkdir -p "$1"
DIR="$(cd "$1" && pwd -P)"

PNPM_V=10.34.7 BUN_V=1.4.2
case "$(uname -s)/$(uname -m)" in
  Darwin/arm64) PNPM_FILE=pnpm-macos-arm64 BUN_FILE=bun-darwin-aarch64 ;;
  Darwin/x86_64) PNPM_FILE=pnpm-macos-x64 BUN_FILE=bun-darwin-x64 ;;
  *) echo "scripts/fetch-js-tools.sh is for a Mac, not $(uname -s)/$(uname -m)." >&2; exit 1 ;;
esac

actual_sum() { shasum -a 256 "$1" | cut -d' ' -f1; }

# fetch NAME URL: DIR/<the file's name>, downloaded unless it's there, and always checked against NAME's line.
fetch() {
  local name="$1" url="$2" want file
  want="$(awk -v f="$name" '$2 == f { print $1 }' "$SUMS")"
  [ -n "$want" ] || { echo "third_party/js-tools/SHA256SUMS has no line for $name." >&2; exit 1; }
  file="$DIR/$(basename "$name")"
  if [ ! -f "$file" ]; then
    echo "Downloading $name from $(echo "$url" | cut -d/ -f3)" >&2
    curl -fsSL --proto '=https' --proto-redir '=https' --retry 3 --connect-timeout 20 -o "$file.part" "$url"
    mv "$file.part" "$file"
  fi
  if [ "$(actual_sum "$file")" != "$want" ]; then
    rm -f "$file"
    echo "$name doesn't match third_party/js-tools/SHA256SUMS: stopped, and the file removed." >&2
    exit 1
  fi
}

fetch "pnpm-$PNPM_V/$PNPM_FILE" "https://github.com/pnpm/pnpm/releases/download/v$PNPM_V/$PNPM_FILE"
fetch "bun-$BUN_V/$BUN_FILE.zip" "https://github.com/oven-sh/bun/releases/download/bun-v$BUN_V/$BUN_FILE.zip"

# The programs, from the checked files only.
install -m 0755 "$DIR/$PNPM_FILE" "$DIR/pnpm"
rm -rf "$DIR/$BUN_FILE"
unzip -q "$DIR/$BUN_FILE.zip" "$BUN_FILE/bun" -d "$DIR"
install -m 0755 "$DIR/$BUN_FILE/bun" "$DIR/bun"
rm -rf "$DIR/$BUN_FILE"
echo "$DIR"
