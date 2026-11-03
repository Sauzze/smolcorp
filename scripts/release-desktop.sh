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
# The Windows programs get their version info, icon and manifest from a .syso
# that scripts/mkwinres writes beside the code just before each build. It is
# never committed (it is ignored), and it is deleted when the script ends,
# done or failed.

set -euo pipefail

cd "$(dirname "$0")/.."

# A release is built from what's committed, and nothing else: no changes
# waiting, and no stray files, ignored or not, beside the pages it seals in.
if [ -n "$(git status --porcelain)" ] || [ -n "$(git ls-files --others --ignored --exclude-standard Resources/web)" ]; then
  echo "Commit or set aside these first: a release is built from what's committed."
  git status --short
  git ls-files --others --ignored --exclude-standard Resources/web
  exit 1
fi
# A .syso beside the code is linked into the Windows programs, and git ignores
# them now (scripts/mkwinres writes one for each Windows build below, and this
# script deletes it), so one left from elsewhere would slip in unseen.
stray="$( (git ls-files --others --ignored --exclude-standard -- '*.syso'; git ls-files -- '*.syso') | grep -v -e '^build/' -e '^\.build/' || true)"
if [ -n "$stray" ]; then
  echo "Remove these .syso files first: they would be linked into the Windows programs."
  echo "$stray"
  exit 1
fi
# Nothing of the maintainer's own goes out in a download.
scripts/check-public.sh
VERSION="$(sed -n '/CFBundleShortVersionString/{n;s/.*<string>\(.*\)<\/string>.*/\1/p;}' Resources/Info.plist)"
BUILD="$(sed -n '/<key>CFBundleVersion<\/key>/{n;s/.*<string>\(.*\)<\/string>.*/\1/p;}' Resources/Info.plist)"
STAGE="build/desktop"
# Only after the checks above: from here on a .syso may sit in cmd/smolcorp.
trap 'rm -f cmd/smolcorp/rsrc_windows_*.syso' EXIT
rm -rf "$STAGE"
mkdir -p "$STAGE"

# -trimpath keeps the paths of this machine out of the programs, and
# -buildvcs=false the commit they were built from, which may be one of this
# machine's own that the published repository doesn't have.
build() { # GOOS GOARCH OUTPUT [extra linker flags]
  CGO_ENABLED=0 GOOS="$1" GOARCH="$2" go build -trimpath -buildvcs=false \
    -ldflags "-s -w -X main.version=$VERSION ${4:-}" -o "$3" ./cmd/smolcorp
}

# The version info, icon and manifest of a Windows program, as a .syso the
# next go build links in: ARCH FILE-DESCRIPTION ORIGINAL-FILE-NAME INTERNAL-NAME.
# The icon is made at 16, 32, 48 and 256 pixels from the 256-pixel Linux one.
windows_resources() {
  go run ./scripts/mkwinres -arch "$1" -version "$VERSION" -build "$BUILD" \
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
  # Owned by root, without the Mac's extended attributes: an archive made
  # here would otherwise carry this machine's user name and metadata.
  COPYFILE_DISABLE=1 tar --uid 0 --gid 0 --uname root --gname root --no-xattrs --no-mac-metadata \
    -czf "build/Smolcorp-Linux-$name.tar.gz" -C "$STAGE" "Smolcorp-Linux-$name"
done

echo "Smolcorp $VERSION for Windows and Linux"
for file in build/Smolcorp-Windows-*.zip build/Smolcorp-Linux-*.tar.gz; do
  echo "SHA-256: $(shasum -a 256 "$file" | cut -d ' ' -f 1)  ${file#build/}"
done
