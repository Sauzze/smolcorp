#!/bin/bash
# Builds the Windows and Linux downloads for a GitHub release, next to the
# Mac's, and prints their SHA-256s for the release notes:
#
#   build/Smolcorp-Windows-x86_64.zip    build/Smolcorp-Windows-arm64.zip
#   build/Smolcorp-Linux-x86_64.tar.gz   build/Smolcorp-Linux-arm64.tar.gz
#
#   scripts/release-desktop.sh
#
# Needs Go. The version is the Mac app's, from Resources/Info.plist, so the
# three platforms are released together.
#
# Each Linux download carries Smolcorp's own tmux, with its guard, which its
# install.sh puts in ~/.local/lib/smolcorp. It is built on Linux, for each
# processor, by CI's tmux-linux job (scripts/build-tmux.sh), not here. Put
# what that job kept in build/tmux-linux/<amd64|arm64>/ first:
#
#   gh run download <run> -n tmux-linux-amd64 -D build/tmux-linux/amd64
#   gh run download <run> -n tmux-linux-arm64 -D build/tmux-linux/arm64
#
# scripts/linuxtmux checks each before anything is built: an ELF program for
# that processor that loads only glibc, with the guard in it, and a BUILDINFO
# naming its SHA-256 and the recipe of HEAD's own build-tmux.sh, SHA256SUMS
# and guard.patch, so a tmux built from other sources or another guard
# stops the script. After each archive is made, the tmux inside it is read
# back and must be that file, byte for byte.
#
# The Windows programs get their version info, icon and manifest from a .syso
# that scripts/mkwinres writes beside the code just before each build. It is
# never committed (it is ignored), and it is deleted when the script ends,
# done or failed.
#
# Before anything is built, scripts/release-gate.sh runs every check a
# release must pass (its top lists them, and how to make the release commit):
# this script builds only from one squashed commit straight onto origin/main,
# its files exactly HEAD's, once the bundled tmux's test, the release's own
# tests and the forged-event table have passed. If a check fails, nothing is
# built. The builds below use the go, the C compiler and the settings the gate
# pinned (pinned_env, pinned_go), and go's folders it made for the run.

set -euo pipefail

cd "$(dirname "$0")/.."
# go's folders stay for the builds below: asked in this shell, unexported, as
# the gate wants it (a setting of the caller's doesn't count).
declare +x release_gate_builds_go=1
. scripts/release-gate.sh
# The office the programs build in is exactly the build of its sources. Pinned
# as every check of the gate is (go_env, in scripts/release-gate.sh).
pinned_go run ./scripts/buildoffice -check
# The tmux each Linux download carries, built by CI (see the top). Pinned too.
for arch in amd64 arm64; do
  if ! pinned_go run ./scripts/linuxtmux -arch "$arch" "build/tmux-linux/$arch"; then
    echo "Stopped: build/tmux-linux/$arch doesn't hold the tmux CI built from HEAD's recipe for linux/$arch."
    echo "Download it from a tmux-linux run of these files (see the top of this script), then run this again."
    exit 1
  fi
  # Its SHA-256 as checked: what goes into the archive must be this file.
  printf -v "tmux_sum_$arch" '%s' "$(shasum -a 256 "build/tmux-linux/$arch/tmux" | cut -d ' ' -f 1)"
done
VERSION="$(sed -n '/CFBundleShortVersionString/{n;s/.*<string>\(.*\)<\/string>.*/\1/p;}' Resources/Info.plist)"
BUILD="$(sed -n '/<key>CFBundleVersion<\/key>/{n;s/.*<string>\(.*\)<\/string>.*/\1/p;}' Resources/Info.plist)"
STAGE="build/desktop"
# Only after the checks above: from here on a .syso may sit in cmd/smolcorp, and
# the gate's on_exit deletes it when this ends.
remove_syso=1
rm -rf "$STAGE"
mkdir -p "$STAGE"

# -trimpath keeps the paths of this machine out of the programs, and
# -buildvcs=false the commit they were built from, which may be one of this
# machine's own that the published repository doesn't have. Built by the go
# named at the start, with its settings pinned as the checks' are (no caller's
# GOFLAGS -tags, -ldflags or -toolexec), for the system asked.
build() { # GOOS GOARCH OUTPUT [extra linker flags]
  pinned_env CGO_ENABLED=0 GOOS="$1" GOARCH="$2" "$GO" build -trimpath -buildvcs=false \
    -ldflags "-s -w -X main.version=$VERSION ${4:-}" -o "$3" ./cmd/smolcorp
}

# The version info, icon and manifest of a Windows program, as a .syso the
# next go build links in: ARCH FILE-DESCRIPTION ORIGINAL-FILE-NAME INTERNAL-NAME.
# The icon is made at 16, 32, 48 and 256 pixels from the 256-pixel Linux one.
# Pinned, as every go run here is.
windows_resources() {
  pinned_go run ./scripts/mkwinres -arch "$1" -version "$VERSION" -build "$BUILD" \
    -description "$2" -original "$3" -internal "$4" -icon packaging/linux/smolcorp.png \
    -license LICENSE -o "cmd/smolcorp/rsrc_windows_$1.syso"
}

for arch in amd64 arm64; do
  name="${arch/amd64/x86_64}"

  # Windows: the app, which has no console window, and the command line
  # that agents and terminals run. Windows ignores case, so the two can't be
  # Smolcorp.exe and smolcorp.exe.
  dir="$STAGE/Smolcorp-Windows-$name"
  mkdir -p "$dir"
  windows_resources "$arch" "Smolcorp" "Smolcorp.exe" "smolcorp"
  build windows "$arch" "$dir/Smolcorp.exe" "-H windowsgui"
  windows_resources "$arch" "Smolcorp command line" "smolcorp-cli.exe" "smolcorp-cli"
  build windows "$arch" "$dir/smolcorp-cli.exe"
  rm -f "cmd/smolcorp/rsrc_windows_$arch.syso"
  cp LICENSE "$dir/LICENSE.txt"
  # One prompt to paste into a coding agent, which then sets it all up.
  cp packaging/FIRST-SETUP.md "$dir/"
  rm -f "build/Smolcorp-Windows-$name.zip"
  # -X leaves out this machine's user and group ids.
  (cd "$STAGE" && zip -qrX "../Smolcorp-Windows-$name.zip" "Smolcorp-Windows-$name")

  # Linux: one program, and an installer for your own account.
  dir="$STAGE/Smolcorp-Linux-$name"
  mkdir -p "$dir"
  build linux "$arch" "$dir/smolcorp"
  cp packaging/linux/install.sh LICENSE packaging/FIRST-SETUP.md "$dir/"
  cp packaging/linux/smolcorp.png "$dir/smolcorp.png"
  # The tmux checked above (a downloaded artifact loses its mode).
  cp "build/tmux-linux/$arch/tmux" "$dir/tmux"
  chmod 755 "$dir/tmux"
  cp "build/tmux-linux/$arch/THIRD-PARTY-NOTICES.txt" "$dir/THIRD-PARTY-NOTICES-tmux.txt"
  # Owned by root, without the Mac's extended attributes: an archive made
  # here would otherwise carry this machine's user name and metadata.
  COPYFILE_DISABLE=1 tar --uid 0 --gid 0 --uname root --gname root --no-xattrs --no-mac-metadata \
    -czf "build/Smolcorp-Linux-$name.tar.gz" -C "$STAGE" "Smolcorp-Linux-$name"
  # What went out is the tmux that was checked.
  want="tmux_sum_$arch"
  want="${!want}"
  got="$(tar -xOzf "build/Smolcorp-Linux-$name.tar.gz" "Smolcorp-Linux-$name/tmux" | shasum -a 256 | cut -d ' ' -f 1)"
  if [ "$got" != "$want" ]; then
    echo "Stopped: the tmux in build/Smolcorp-Linux-$name.tar.gz ($got) isn't the one checked ($want)."
    exit 1
  fi
done

echo "Smolcorp $VERSION for Windows and Linux"
for file in build/Smolcorp-Windows-*.zip build/Smolcorp-Linux-*.tar.gz; do
  echo "SHA-256: $(shasum -a 256 "$file" | cut -d ' ' -f 1)  ${file#build/}"
done
echo "SHA-256: $tmux_sum_amd64  tmux in Smolcorp-Linux-x86_64"
echo "SHA-256: $tmux_sum_arm64  tmux in Smolcorp-Linux-arm64"
