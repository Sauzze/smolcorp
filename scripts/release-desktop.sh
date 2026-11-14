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
# Since 1.26, a release goes out as one commit, and this script builds only
# from that commit:
#
#   1. It is one squashed commit of the final files, signed (the -S below;
#      this script doesn't check the signature), made straight onto main as
#      GitHub has it (origin/main). The branches the work was done on (a
#      feature branch, release/<version>) never go out, and neither does
#      their history.
#   2. HEAD is that commit: it has one parent, and that parent is origin/main.
#      Anything else (a branch with its own commits, a merge, a commit on an
#      older main) stops the script before it builds anything. HEAD is read
#      as a push sends it, so replace refs and a grafts file can't make it
#      look like another commit. There is no way around it: this script
#      always builds the downloads for real.
#   3. scripts/check-public.sh checks exactly the files that go out: HEAD's,
#      with nothing changed or added beside them, before and after it runs.
#      A checkout with files git is told not to look at (assume-unchanged,
#      skip-worktree, a sparse checkout, untracked files status is set not
#      to list) or with Go code git ignores beside the code stops the script
#      too.
#
# To make that commit from the branch you finished on, and build from it:
#
#   git fetch origin main
#   git checkout --detach "$(git commit-tree -S -p origin/main -m 'Smolcorp <version>' 'HEAD^{tree}')"
#   scripts/release-desktop.sh

set -euo pipefail

cd "$(dirname "$0")/.."
# git looks at this folder, the one check-public.sh and the build read, and
# at its own index: not at another one a caller's environment or the repo's
# config (core.worktree) names.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE
top="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [ "$top" != "$(pwd -P)" ]; then
  echo "Stopped: git here looks at another folder (${top:-none}), not this one ($(pwd -P))."
  echo "Unset core.worktree (git config --unset core.worktree), then run this again."
  exit 1
fi

# HEAD as a push sends it (point 2 above): the parent and tree lines of its
# own commit object, which a grafts file (info/grafts) doesn't change, read
# with no replace refs (refs/replace). Every git command below sees the real
# objects too.
export GIT_NO_REPLACE_OBJECTS=1
head_header() { git cat-file commit HEAD 2>/dev/null | sed '/^$/,$d'; }
head_parents() { head_header | sed -n 's/^parent //p'; }
head_tree() { head_header | sed -n 's/^tree //p'; }

# One commit straight onto origin/main, and nothing else (point 2 above).
# HEAD's parents, one per line: exactly origin/main means one parent, and the
# right one.
BASE="$(git rev-parse --verify --quiet 'refs/remotes/origin/main^{commit}' || true)"
if [ -z "$BASE" ]; then
  echo "Stopped: origin/main isn't here, so the release commit can't be checked."
  echo "Run git fetch origin main, make the release commit (see the top of this script), then this again."
  exit 1
fi
parents="$(head_parents || true)"
if [ "$parents" != "$BASE" ]; then
  echo "Stopped: a release is built from one commit straight onto origin/main, and HEAD isn't one."
  echo "HEAD's parents:"
  echo "${parents:-none}" | sed 's/^/  /'
  echo "origin/main:"
  echo "  $BASE"
  echo "Make the release commit from your branch's files, then run this again:"
  echo "  git fetch origin main"
  echo "  git checkout --detach \"\$(git commit-tree -S -p origin/main -m 'Smolcorp <version>' 'HEAD^{tree}')\""
  exit 1
fi
TREE="$(head_tree || true)"
if [ -z "$TREE" ]; then
  echo "Stopped: HEAD's files can't be read here."
  exit 1
fi

# Nothing differs from HEAD, untracked files included even when
# status.showUntrackedFiles says not to list them, and git could tell: a git
# status that fails says nothing either.
clean() {
  local changes
  changes="$(git status --porcelain --untracked-files=all)" && [ -z "$changes" ]
}
# A release is built from what's committed, and nothing else: no changes
# waiting, and no stray files, ignored or not, beside the pages it seals in.
if ! clean || [ -n "$(git ls-files --others --ignored --exclude-standard Resources/web)" ]; then
  echo "Commit or set aside these first: a release is built from what's committed."
  git status --short --untracked-files=all || true
  git ls-files --others --ignored --exclude-standard Resources/web
  exit 1
fi
# Files git is told not to look at: git status and git write-tree pass over
# their changes on disk, and check-public.sh reads the disk. git ls-files -v
# tags every ordinary file H; assume-unchanged ones are lowercase, and
# skip-worktree ones (which a sparse checkout makes) S.
unseen() { git ls-files -v | grep -v '^H ' || true; }
sparse() { [ "$(git config --bool core.sparseCheckout || true)" = true ]; }
# So the files here are HEAD's tree, exactly (point 3 above): the index makes
# that tree, HEAD still has it, nothing differs from it, and git looks at
# every file.
heads_tree() {
  [ "$(git write-tree)" = "$TREE" ] && [ "$(head_tree)" = "$TREE" ] && clean &&
    [ -z "$(unseen)" ] && ! sparse
}
if ! heads_tree; then
  echo "Stopped: the files here aren't HEAD's tree ($TREE), the one that goes out."
  if sparse; then
    echo "This is a sparse checkout. Run git sparse-checkout disable, then this again."
  elif [ -n "$(unseen)" ]; then
    echo "git is told not to look at these files (assume-unchanged or skip-worktree):"
    unseen | sed 's/^/  /'
    echo "Run git update-index --no-assume-unchanged --no-skip-worktree on each, then git status, then this again."
  fi
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
# Go code git ignores: go build compiles every .go file in each package
# folder it builds, ignored or not, so one there would be built in unseen.
# Ignored .go files elsewhere (a folder of notes, sandbox/) aren't built.
go_dirs() { git -c core.quotePath=false ls-files -- "$@" | sed -e 's,/[^/]*$,,' -e t -e 's,.*,.,' | sort -u; }
ignored_go="$(awk 'FILENAME == ARGV[1] { pkg[$0]; next } { d = $0; if (!sub(/\/[^\/]*$/, "", d)) d = "." } d in pkg' \
  <(go_dirs '*.go') <(git -c core.quotePath=false ls-files --others --ignored --exclude-standard -- '*.go' '*.s'))"
if [ -n "$ignored_go" ]; then
  echo "Remove these first: git ignores them, but Go compiles them into the programs."
  echo "$ignored_go"
  exit 1
fi
# Nothing of the maintainer's own goes out in a download.
scripts/check-public.sh
# What it checked is still HEAD's tree: nothing changed while it ran.
if ! heads_tree; then
  echo "Stopped: the files changed while scripts/check-public.sh ran, so it didn't check HEAD's tree ($TREE). Run this again."
  exit 1
fi
# The office the programs build in is exactly the build of its sources.
go run ./scripts/buildoffice -check
# The tmux each Linux download carries, built by CI (see the top).
for arch in amd64 arm64; do
  if ! go run ./scripts/linuxtmux -arch "$arch" "build/tmux-linux/$arch"; then
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
