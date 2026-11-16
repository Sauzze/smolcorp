#!/bin/bash
# Packs the Linux test binaries for a run on a Linux desktop (a VM, in its
# graphical session), each with what it reads beside it, in the repo's own
# layout:
#
#   scripts/vmpack.sh <tree-or-HEAD> <id> [out folder]   build/vm-<id> if not given
#
# <tree-or-HEAD> is any tree git knows: HEAD, a commit, or a tree's hash (one
# written from what is staged too). It is exported with git archive, so what is
# packed is that tree, never the working folder. Each package's test binary is
# built from it with go test -c, CGO off, for linux/arm64; VMPACK_GOOS,
# VMPACK_GOARCH and VMPACK_CGO change that (the darwin twin of a pack, which
# proves its layout on a Mac, needs VMPACK_CGO=1 for the tray's library).
# The packages: internal/sandbox and internal/sandbox/tmuxfake, every package
# with a test file only Linux builds (*_linux_test.go), and any more named in
# VMPACK_PACKAGES (folders, space-separated).
#
# What a test reads is put where it reads it from: its package's testdata/,
# and the repo's files the tests reach through ../ (the tmux guard's build
# script and its pins, for tmuxfake). Each binary runs from its own package
# folder, as go test runs it, never from the top of the pack.
#
# Out, in the out folder:
#   smolcorp-vm-<id>.tar   a folder smolcorp-vm-<id>/ with the binaries, what
#                          they read, SHA256SUMS and the RUN note
#   RUN-vm-<id>.txt        the RUN note to fill in before it is sent: what the
#                          run proves, and which tests; the commands are written
#
# Nothing is run but git and go; nothing is written outside the out folder
# (and Go's own build cache).

set -euo pipefail

[ $# -ge 2 ] && [ $# -le 3 ] || { echo "Usage: scripts/vmpack.sh <tree-or-HEAD> <id> [out folder]" >&2; exit 2; }
case "$2" in
  *[!A-Za-z0-9._-]* | "" | .*) echo "vmpack: the id is letters, digits, dots, dashes and underscores, not first a dot: $2" >&2; exit 2 ;;
esac

cd "$(dirname "$0")/.."
REPO="$(pwd -P)"
ID="$2"
TREE="$(git rev-parse --verify --quiet "$1^{tree}")" || { echo "vmpack: git knows no tree $1" >&2; exit 2; }
GOOS_="${VMPACK_GOOS:-linux}"
GOARCH_="${VMPACK_GOARCH:-arm64}"
CGO_="${VMPACK_CGO:-0}"
OUT="${3:-build/vm-$ID}"
mkdir -p "$OUT"
OUT="$(cd "$OUT" && pwd -P)"
NAME="smolcorp-vm-$ID"
PACK="$OUT/$NAME"
SRC="$OUT/.src-$ID"
rm -rf "$PACK" "$SRC" "$OUT/$NAME.tar"
mkdir -p "$PACK" "$SRC"
trap 'rm -rf "$SRC"' EXIT

git -C "$REPO" archive "$TREE" | tar -x -C "$SRC"

# The repo's files a package's tests read through ../, by package.
reads() {
  case "$1" in
    internal/sandbox/tmuxfake) echo scripts/build-tmux.sh scripts/check-runtime.sh third_party/tmux ;;
  esac
}

# The packages: the two always, then each with a Linux-only test file, then VMPACK_PACKAGES.
PACKAGES="$( (
  echo internal/sandbox
  echo internal/sandbox/tmuxfake
  cd "$SRC" && find . -name '*_linux_test.go' -not -path './node_modules/*' -exec dirname {} \; | sed 's,^\./,,'
  for p in ${VMPACK_PACKAGES:-}; do echo "${p%/}"; done
) | awk '!seen[$0]++')"

for pkg in $PACKAGES; do
  [ -d "$SRC/$pkg" ] || { echo "vmpack: no package $pkg in tree $TREE" >&2; exit 1; }
  bin="$PACK/$pkg/$(basename "$pkg").test"
  mkdir -p "$PACK/$pkg"
  (cd "$SRC" && CGO_ENABLED="$CGO_" GOOS="$GOOS_" GOARCH="$GOARCH_" go test -c -o "$bin" "./$pkg")
  [ -f "$bin" ] || { echo "vmpack: $pkg has no tests for $GOOS_" >&2; exit 1; }
  if [ -d "$SRC/$pkg/testdata" ]; then
    cp -R "$SRC/$pkg/testdata" "$PACK/$pkg/testdata"
  fi
  for f in $(reads "$pkg"); do
    [ -e "$SRC/$f" ] || { echo "vmpack: $pkg reads $f, which tree $TREE doesn't have" >&2; exit 1; }
    mkdir -p "$PACK/$(dirname "$f")"
    cp -R "$SRC/$f" "$PACK/$f"
  done
done

sum() { if command -v sha256sum >/dev/null; then sha256sum "$@"; else shasum -a 256 "$@"; fi; }
(cd "$PACK" && find . -name '*.test' -type f | sed 's,^\./,,' | sort | while read -r f; do sum "$f"; done) > "$PACK/SHA256SUMS"

# The RUN note: the commands for each binary, from its own folder. A package beside the two runs only its
# Linux-only tests: the rest of it may read the Mac's sources or build with Go, which the VM doesn't have.
{
  echo "RUN-vm-$ID: <what this run proves, in a line>. <who>, $(date '+%Y-%m-%d %H:%M')."
  echo "Built from tree $TREE ($1) by scripts/vmpack.sh: $GOOS_/$GOARCH_, CGO_ENABLED=$CGO_, go test -c."
  echo "Binaries (SHA256SUMS, also in the pack; check them on the VM before running):"
  sed 's/^/  /' "$PACK/SHA256SUMS"
  echo
  echo "RULES: run each session type once, INSIDE the graphical session (a terminal in it, never ssh). Revert to the"
  echo "clean snapshot before each run. Stand-ins only: the tests use their own temp folders and fake keys. No real"
  echo "keys, no Tailscale, no repos, no real agent start. Change no system setting (no sysctl). Send back every line"
  echo "verbatim, FAIL and SKIP lines included."
  echo
  echo "SETUP, once per session:"
  echo "  tar -xf $NAME.tar && cd $NAME && sha256sum -c SHA256SUMS"
  echo "  echo \"session=\$XDG_SESSION_TYPE kernel=\$(uname -r) bwrap=\$(bwrap --version) git=\$(git --version) tmux=\$(tmux -V) xauth=\$XAUTHORITY ssh_sock=\$SSH_AUTH_SOCK\""
  echo
  echo "SESSIONS: (1) GNOME Wayland, (2) \"Ubuntu on Xorg\". In each, from $NAME/ (each binary runs from its own folder):"
  for pkg in $PACKAGES; do
    base="$(basename "$pkg")"
    case "$pkg" in
      internal/sandbox | internal/sandbox/tmuxfake) only="" ;;
      *)
        names="$(cd "$SRC/$pkg" && sed -n 's/^func \(Test[A-Za-z0-9_]*\)(.*/\1/p' ./*_linux_test.go 2>/dev/null | paste -sd '|' -)"
        only=" -test.run '^($names)\$'"
        ;;
    esac
    # The sandbox's rows fail, not skip, where the sandbox can't start; tmuxfake's would fail on what isn't packed.
    required=""
    [ "$pkg" = internal/sandbox ] && required="SMOLCORP_SANDBOX_REQUIRED=1 "
    echo "  (cd $pkg && $required./$base.test -test.v -test.count=1$only) 2>&1 | tee $base-\$XDG_SESSION_TYPE.log"
  done
  echo
  echo "WHAT IT PROVES: <fill in>"
  echo "NOT IN THIS RUN: <fill in>"
} > "$OUT/RUN-vm-$ID.txt"
cp "$OUT/RUN-vm-$ID.txt" "$PACK/RUN-vm-$ID.txt"

# No ._ files of the Mac's for a file's extended attributes.
COPYFILE_DISABLE=1 tar -cf "$OUT/$NAME.tar" -C "$OUT" "$NAME"
echo "vmpack: $OUT/$NAME.tar and $OUT/RUN-vm-$ID.txt (tree $TREE, $GOOS_/$GOARCH_): fill in the RUN note's <...> before sending."
