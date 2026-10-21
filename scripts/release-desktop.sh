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
# Nothing of the maintainer's own goes out in a download.
scripts/check-public.sh
VERSION="$(sed -n '/CFBundleShortVersionString/{n;s/.*<string>\(.*\)<\/string>.*/\1/p;}' Resources/Info.plist)"
STAGE="build/desktop"
rm -rf "$STAGE"
mkdir -p "$STAGE"

# -trimpath keeps the paths of this machine out of the programs.
build() { # GOOS GOARCH OUTPUT [extra linker flags]
  CGO_ENABLED=0 GOOS="$1" GOARCH="$2" go build -trimpath \
    -ldflags "-s -w -X main.version=$VERSION ${4:-}" -o "$3" ./cmd/smolcorp
}

for arch in amd64 arm64; do
  name="${arch/amd64/x86_64}"

  # Windows: the app, which has no console window, and the command line
  # that agents and terminals run. Windows ignores case, so the two can't be
  # Smolcorp.exe and smolcorp.exe.
  dir="$STAGE/Smolcorp-Windows-$name"
  mkdir -p "$dir"
  build windows "$arch" "$dir/Smolcorp.exe" "-H windowsgui"
  build windows "$arch" "$dir/smolcorp-cli.exe"
  cp LICENSE "$dir/LICENSE.txt"
  rm -f "build/Smolcorp-Windows-$name.zip"
  # -X leaves out this machine's user and group ids.
  (cd "$STAGE" && zip -qrX "../Smolcorp-Windows-$name.zip" "Smolcorp-Windows-$name")

  # Linux: one program, and an installer for your own account.
  dir="$STAGE/Smolcorp-Linux-$name"
  mkdir -p "$dir"
  build linux "$arch" "$dir/smolcorp"
  cp packaging/linux/install.sh LICENSE "$dir/"
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
