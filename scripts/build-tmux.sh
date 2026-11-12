#!/bin/bash
# Builds the tmux that ships inside Smolcorp's Mac app: a universal (Apple silicon and Intel) tmux for
# macOS 13.0 and later, with libevent, ncurses, utf8proc and jemalloc linked in as static libraries, so the
# only libraries it loads are the Mac's own libSystem and libresolv.
#
#   scripts/build-tmux.sh              builds (or finds the cached build) and prints the path of the tmux
#                                      as its last line
#   scripts/build-tmux.sh check FILE   starts a server of FILE on a private socket, sends it a key, reads the
#                                      pane back and kills the server by its PID (the build runs this too)
#
# Sources are five pinned tarballs, downloaded into build/third-party/ and checked against
# third_party/tmux/SHA256SUMS: a mismatch stops the build. The result is cached in build/tmux/<key>/, where
# <key> hashes this script, SHA256SUMS, the Xcode SDK and the compiler, so it's rebuilt only when one changes:
#
#   build/tmux/<key>/tmux                     the universal file, stripped and signed ad hoc
#   build/tmux/<key>/THIRD-PARTY-NOTICES.txt  the licences of what's inside, from the tarballs built
#
# Signing for a release (Developer ID) is done by the caller, after this. Needs the Xcode Command Line Tools
# and the network for the first run only. Nothing is installed and nothing from Homebrew is used.

set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$(pwd -P)"

# Nothing from Homebrew or the home folder: only the Xcode tools and the system.
export PATH="/usr/bin:/bin:/usr/sbin:/sbin"
unset PKG_CONFIG PKG_CONFIG_PATH CPATH LIBRARY_PATH
export MACOSX_DEPLOYMENT_TARGET=13.0

TMUX_V=3.8 LIBEVENT_V=2.1.13-stable NCURSES_V=6.6 UTF8PROC_V=2.12.0 JEMALLOC_V=5.4.0
# Where each tarball comes from. The file names are the ones in SHA256SUMS. utf8proc's is one GitHub makes
# from a tag, the likeliest to change: the checksum stops the build if it does.
URL_tmux="https://github.com/tmux/tmux/releases/download/$TMUX_V/tmux-$TMUX_V.tar.gz"
URL_libevent="https://github.com/libevent/libevent/releases/download/release-$LIBEVENT_V/libevent-$LIBEVENT_V.tar.gz"
URL_ncurses="https://invisible-mirror.net/archives/ncurses/ncurses-$NCURSES_V.tar.gz https://ftpmirror.gnu.org/ncurses/ncurses-$NCURSES_V.tar.gz"
URL_utf8proc="https://github.com/JuliaStrings/utf8proc/archive/refs/tags/v$UTF8PROC_V.tar.gz"
URL_jemalloc="https://github.com/jemalloc/jemalloc/releases/download/$JEMALLOC_V/jemalloc-$JEMALLOC_V.tar.bz2"
TARBALLS="tmux-$TMUX_V.tar.gz libevent-$LIBEVENT_V.tar.gz ncurses-$NCURSES_V.tar.gz utf8proc-$UTF8PROC_V.tar.gz jemalloc-$JEMALLOC_V.tar.bz2"
SUMS="$REPO/third_party/tmux/SHA256SUMS"
# Entries built into ncurses, for when no terminfo folder has the terminal tmux is shown in.
FALLBACKS="tmux-256color,tmux,screen-256color,screen,xterm-256color,xterm"

# The runtime check: a server on a private socket, a key sent, the pane read back, the server killed by its
# PID (never by name). The socket is relative to a folder of its own: a socket path over 104 bytes fails.
runtime_check() {
  local tmux="$1" arch_cmd="${2:-}" label="${3:-this Mac}" dir out pid
  mkdir -p "$REPO/build"
  dir="$(mktemp -d "$REPO/build/tmux-check.XXXXXX")"
  cd "$dir"
  # Every command names the socket and no config file, so nothing of the user's is read or touched.
  # shellcheck disable=SC2086
  $arch_cmd "$tmux" -f /dev/null -S p.sock new-session -d -s proof 'cat'
  # shellcheck disable=SC2086
  pid="$($arch_cmd "$tmux" -S p.sock display-message -p '#{pid}')"
  # shellcheck disable=SC2086
  $arch_cmd "$tmux" -S p.sock send-keys -t proof 'hello from bundled tmux' Enter
  sleep 1
  # shellcheck disable=SC2086
  out="$($arch_cmd "$tmux" -S p.sock capture-pane -p -t proof)"
  # shellcheck disable=SC2086
  echo "  $label: server $pid, version $($arch_cmd "$tmux" -S p.sock display-message -p '#{version}')," \
    "default-terminal $($arch_cmd "$tmux" -S p.sock show-options -gv default-terminal)"
  kill "$pid"
  sleep 0.5
  if kill -0 "$pid" 2>/dev/null; then kill -9 "$pid"; fi
  cd "$REPO"
  rm -rf "$dir"
  case "$out" in
    *"hello from bundled tmux"*) echo "  $label: the pane says what was sent; server $pid ended (killed by PID)" ;;
    *) echo "The server on $label didn't echo what was sent to it." >&2; return 1 ;;
  esac
}

# Runs the slices that can run here: this Mac's own, and Intel's under Rosetta when it has it.
check_file() {
  local tmux="$1"
  case "$(uname -m)" in
    arm64) runtime_check "$tmux" "arch -arm64" "arm64" ;;
    *) runtime_check "$tmux" "arch -x86_64" "x86_64" ;;
  esac
  if [ "$(uname -m)" = arm64 ]; then
    if arch -x86_64 /usr/bin/true 2>/dev/null; then
      runtime_check "$tmux" "arch -x86_64" "x86_64 under Rosetta"
    else
      echo "  x86_64: skipped, this Mac has no Rosetta"
    fi
  fi
}

if [ "${1:-}" = "check" ]; then
  [ -x "${2:-}" ] || { echo "Usage: scripts/build-tmux.sh check <tmux file>" >&2; exit 2; }
  case "$2" in /*) ;; *) set -- "$1" "$REPO/$2" ;; esac
  echo "Checking $2"
  [ "$("$2" -V)" = "tmux $TMUX_V" ] || { echo "$("$2" -V) isn't tmux $TMUX_V." >&2; exit 1; }
  check_file "$2"
  exit 0
fi

[ "$(uname -s)" = Darwin ] || { echo "build-tmux.sh builds the Mac's tmux and runs only on a Mac." >&2; exit 1; }
for tool in xcrun clang make lipo strip codesign otool nm strings curl shasum tar; do
  command -v "$tool" >/dev/null 2>&1 || { echo "build-tmux.sh needs $tool: install the Xcode Command Line Tools (xcode-select --install)." >&2; exit 1; }
done

KEY="$({ cat "$0" "$SUMS"; xcrun --show-sdk-version; clang --version; } | shasum -a 256 | cut -c1-16)"
OUT="build/tmux/$KEY"
if [ -x "$OUT/tmux" ] && [ -s "$OUT/THIRD-PARTY-NOTICES.txt" ]; then
  echo "tmux $TMUX_V is already built: $OUT/tmux"
  echo "$OUT/tmux"
  exit 0
fi

# ---- 1. the tarballs: fetched once, always checked ----
expected_sum() { awk -v f="$1" '$2 == f { print $1 }' "$SUMS"; }
actual_sum() { shasum -a 256 "$1" | cut -d' ' -f1; }
mkdir -p build/third-party
for name in $TARBALLS; do
  want="$(expected_sum "$name")"
  [ -n "$want" ] || { echo "third_party/tmux/SHA256SUMS has no line for $name." >&2; exit 1; }
  file="build/third-party/$name"
  if [ -f "$file" ]; then
    [ "$(actual_sum "$file")" = "$want" ] || { echo "$file doesn't match third_party/tmux/SHA256SUMS. Delete it to download it again, or stop here if the pin was meant to change." >&2; exit 1; }
    continue
  fi
  case "$name" in
    tmux-*) urls="$URL_tmux" ;; libevent-*) urls="$URL_libevent" ;; ncurses-*) urls="$URL_ncurses" ;;
    utf8proc-*) urls="$URL_utf8proc" ;; jemalloc-*) urls="$URL_jemalloc" ;;
  esac
  fetched=0
  for url in $urls; do
    echo "Downloading $name from $(echo "$url" | cut -d/ -f3)"
    if curl -fsSL --retry 3 --connect-timeout 20 -o "$file.part" "$url" && [ "$(actual_sum "$file.part")" = "$want" ]; then
      mv "$file.part" "$file"
      fetched=1
      break
    fi
    echo "  that didn't give the file pinned in SHA256SUMS."
    rm -f "$file.part"
  done
  [ "$fetched" = 1 ] || { echo "Couldn't get $name with the checksum pinned in third_party/tmux/SHA256SUMS." >&2; exit 1; }
done

# ---- 2. the build, in a folder of its own that becomes $OUT only when everything has passed ----
WORK="$REPO/build/tmux/$KEY.work"
rm -rf "$WORK" "$OUT"
mkdir -p "$WORK/src" "$WORK/out"
trap 'echo "The tmux build failed; its logs are in build/tmux/$KEY.work (config.out and make.out in each folder under build/)." >&2' ERR
for name in $TARBALLS; do tar -xf "build/third-party/$name" -C "$WORK/src"; done
JOBS="$(sysctl -n hw.ncpu)"
SRC="$WORK/src"

# A tic for this Mac, only to compile the entries built into ncurses.
echo "Building tools for this Mac…"
mkdir -p "$WORK/build/host-ncurses" && cd "$WORK/build/host-ncurses"
"$SRC/ncurses-$NCURSES_V/configure" --prefix="$WORK/host" --without-shared --without-cxx \
  --without-cxx-binding --without-ada --without-manpages --without-tests --without-debug \
  --enable-widec --disable-db-install >config.out 2>&1
make -j"$JOBS" >make.out 2>&1
make install.progs >>make.out 2>&1
TIC="$WORK/host/bin/tic"
INFOCMP="$WORK/host/bin/infocmp"

for arch in arm64 x86_64; do
  P="$WORK/prefix/$arch"
  B="$WORK/build/$arch"
  mkdir -p "$P/include" "$P/lib" "$B"
  # -ffile-prefix-map keeps the build folder's path out of the __FILE__ strings libevent carries.
  CC="clang -arch $arch -mmacosx-version-min=13.0"
  CFLAGS="-O2 -g0 -ffile-prefix-map=$SRC/="
  HOST="$arch-apple-darwin"
  [ "$arch" = arm64 ] && HOST="aarch64-apple-darwin"
  echo "Building the $arch slice…"

  # libevent: static, no OpenSSL, no samples. No pipe2: the SDK has it, macOS 13 doesn't (see the weak-import
  # check below).
  mkdir -p "$B/libevent" && cd "$B/libevent"
  "$SRC/libevent-$LIBEVENT_V/configure" --host="$HOST" --prefix="$P" CC="$CC" CFLAGS="$CFLAGS" \
    --disable-shared --enable-static --disable-openssl --disable-mbedtls --disable-samples \
    --disable-libevent-regress --disable-debug-mode \
    ac_cv_func_pipe2=no >config.out 2>&1
  make -j"$JOBS" >make.out 2>&1 && make install >>make.out 2>&1

  # ncurses: the library only, wide characters, terminfo from the Mac's own folder (plus TERMINFO,
  # TERMINFO_DIRS and ~/.terminfo), and a few entries built in for when none of them has the terminal.
  mkdir -p "$B/ncurses" && cd "$B/ncurses"
  "$SRC/ncurses-$NCURSES_V/configure" --host="$HOST" --prefix="$P" CC="$CC" CFLAGS="$CFLAGS" \
    --with-build-cc=clang --without-shared --with-normal --without-debug --without-cxx \
    --without-cxx-binding --without-ada --without-manpages --without-tests --without-progs \
    --enable-widec --disable-db-install --with-default-terminfo-dir=/usr/share/terminfo \
    --with-terminfo-dirs=/usr/share/terminfo --with-fallbacks="$FALLBACKS" \
    --with-tic-path="$TIC" --with-infocmp-path="$INFOCMP" >config.out 2>&1
  make -j"$JOBS" libs >make.out 2>&1
  make install.libs install.includes >>make.out 2>&1

  # utf8proc: its own Makefile, the static library alone.
  rm -rf "$B/utf8proc" && cp -R "$SRC/utf8proc-$UTF8PROC_V" "$B/utf8proc" && cd "$B/utf8proc"
  make CC="$CC" CFLAGS="$CFLAGS" libutf8proc.a >make.out 2>&1
  cp libutf8proc.a "$P/lib/" && cp utf8proc.h "$P/include/"

  # jemalloc: as Homebrew builds it for tmux (no prefix), static, pages of up to 16 KB on both slices (an
  # Intel slice may run under Rosetta on Apple silicon).
  mkdir -p "$B/jemalloc" && cd "$B/jemalloc"
  "$SRC/jemalloc-$JEMALLOC_V/configure" --host="$HOST" --prefix="$P" CC="$CC" CFLAGS="$CFLAGS" \
    --with-jemalloc-prefix= --disable-cxx --disable-debug --disable-stats --disable-prof \
    --disable-libdl --with-lg-page=14 >config.out 2>&1
  make -j"$JOBS" build_lib_static >make.out 2>&1
  make install_include install_lib_static >>make.out 2>&1

  # tmux: every library above by its .a file, so nothing can pick a .dylib. jemalloc whole (-force_load), or
  # its zone registration is left out and free() crashes.
  mkdir -p "$B/tmux" && cd "$B/tmux"
  "$SRC/tmux-$TMUX_V/configure" --host="$HOST" --prefix=/usr/local CC="$CC" CFLAGS="$CFLAGS" \
    CPPFLAGS="-I$P/include -I$P/include/ncursesw" \
    LIBEVENT_CORE_CFLAGS="-I$P/include" LIBEVENT_CORE_LIBS="$P/lib/libevent_core.a" \
    LIBTINFOW_CFLAGS="-I$P/include/ncursesw" LIBTINFOW_LIBS="$P/lib/libncursesw.a" \
    LIBUTF8PROC_CFLAGS="-I$P/include" LIBUTF8PROC_LIBS="$P/lib/libutf8proc.a" \
    JEMALLOC_CFLAGS="-I$P/include" JEMALLOC_LIBS="-Wl,-force_load,$P/lib/libjemalloc.a" \
    PKG_CONFIG=/usr/bin/false \
    --enable-utf8proc --enable-jemalloc --with-TERM=screen-256color >config.out 2>&1
  make -j"$JOBS" >make.out 2>&1
  cp tmux "$WORK/out/tmux-$arch"
done
cd "$REPO"
trap - ERR

# ---- 3. the checks: any one of them stops the build ----
fail() { echo "Check failed: $*" >&2; echo "Its work is left in build/tmux/$KEY.work." >&2; exit 1; }
echo "Checking the slices…"
for arch in arm64 x86_64; do
  f="$WORK/out/tmux-$arch"
  # A function newer than macOS 13 that configure found in the newer SDK is a weak import, NULL on an older
  # Mac, and tmux crashes calling it (libevent's pipe2 did, from the macOS 27 SDK). The only one allowed is
  # jemalloc's own, which it checks for NULL.
  weak="$(nm -m "$f" | grep '(undefined) weak' | grep -v '_malloc_default_purgeable_zone' || true)"
  [ -z "$weak" ] || fail "weak imports in the $arch slice (the Mac's own library may lack them):
$weak"
  # jemalloc's zone registration must be linked in, or free() crashes on memory the system allocated.
  nm "$f" >"$f.syms"
  grep -q 'je_zone_register' "$f.syms" || fail "je_zone_register isn't in the $arch slice (jemalloc wasn't linked whole)"
done
lipo -create "$WORK/out/tmux-arm64" "$WORK/out/tmux-x86_64" -output "$WORK/out/tmux"
strip -S -x "$WORK/out/tmux"
# strip breaks the signature the linker made, and an arm64 Mac runs no unsigned code: sign after it.
codesign --force --sign - "$WORK/out/tmux"
T="$WORK/out/tmux"

[ "$(lipo -archs "$T")" = "x86_64 arm64" ] || fail "lipo says '$(lipo -archs "$T")', not 'x86_64 arm64'"
libs="$(otool -L "$T" | grep -E '^[[:space:]]' | awk '{print $1}' | sort -u | tr '\n' ' ')"
[ "$libs" = "/usr/lib/libresolv.9.dylib /usr/lib/libSystem.B.dylib " ] || fail "it loads $libs, not only libSystem and libresolv"
# Both slices must say macOS 13.0, so the oldest supported Mac can run it.
minos="$(otool -l "$T" | awk '$1 == "minos" { print $2 }' | tr '\n' ' ')"
[ "$minos" = "13.0 13.0 " ] || fail "minos is '$minos', not 13.0 on both slices"
# The build folders' paths would tell a download where it was built.
strings -a "$T" >"$T.strings"
if grep -q -e "$WORK" -e "$REPO" "$T.strings"; then
  fail "the file carries the build folder's path: $(grep -e "$WORK" -e "$REPO" "$T.strings" | head -3)"
fi
codesign --verify "$T" || fail "the ad hoc signature doesn't verify"
[ "$("$T" -V)" = "tmux $TMUX_V" ] || fail "tmux -V says '$("$T" -V)', not 'tmux $TMUX_V'"
echo "  $(lipo -archs "$T"), $(wc -c <"$T" | tr -d ' ') bytes, loads only libSystem and libresolv, minos 13.0, tmux -V = $("$T" -V)"
echo "Running it…"
check_file "$T"

# ---- 4. the notices: each library's licence whole, and the header of each compat file tmux compiled ----
N="$WORK/out/THIRD-PARTY-NOTICES.txt"
{
  echo "Smolcorp's tmux and the libraries linked into it"
  echo "================================================"
  echo
  echo "The tmux that comes with Smolcorp is tmux $TMUX_V, built from its published source with libevent $LIBEVENT_V,"
  echo "ncurses $NCURSES_V, utf8proc $UTF8PROC_V and jemalloc $JEMALLOC_V linked into it. Their licences follow,"
  echo "as they come in the sources built. The copyright of each belongs to its authors."
  echo
  section() { # title, file in the sources
    echo
    echo "################################################################################"
    echo "# $1"
    echo "################################################################################"
    echo
    cat "$2"
  }
  section "tmux $TMUX_V (ISC licence): COPYING" "$SRC/tmux-$TMUX_V/COPYING"
  # tmux's compat/ files with a licence of their own: the header comment of each one that was compiled in.
  H="$WORK/headers"
  mkdir -p "$H"
  for obj in "$WORK"/build/arm64/tmux/compat/*.o; do
    [ -e "$obj" ] || continue
    base="$(basename "$obj" .o)"
    src="$SRC/tmux-$TMUX_V/compat/${base#tmux-}.c"
    [ -f "$src" ] || src="$SRC/tmux-$TMUX_V/compat/$base.c"
    [ -f "$src" ] || continue
    # The comments the file opens with (an $OpenBSD$ line and then the licence, say), up to its first code.
    head="$(awk 'inc { print } !inc && /^[[:space:]]*$/ { next } !inc && !/^\/\*/ { exit } !inc { print; inc = 1 } inc && /\*\// { inc = 0 }' "$src")"
    [ -n "$head" ] || continue
    id="$(printf '%s' "$head" | shasum -a 256 | cut -c1-16)"
    printf '%s\n' "$head" >"$H/h.$id"
    echo "compat/$(basename "$src")" >>"$H/n.$id"
  done
  echo
  echo "################################################################################"
  echo "# tmux $TMUX_V: the notices at the top of the compat/ files compiled into it"
  echo "################################################################################"
  for hf in "$H"/h.*; do
    [ -e "$hf" ] || continue
    id="${hf##*/h.}"
    echo
    echo "Files: $(sort "$H/n.$id" | tr '\n' ' ')"
    echo
    cat "$hf"
  done
  section "libevent $LIBEVENT_V (BSD 3-clause, with the notices it carries): LICENSE" "$SRC/libevent-$LIBEVENT_V/LICENSE"
  section "ncurses $NCURSES_V (MIT/X11 style): COPYING" "$SRC/ncurses-$NCURSES_V/COPYING"
  section "utf8proc $UTF8PROC_V (MIT, and the Unicode data licence): LICENSE.md" "$SRC/utf8proc-$UTF8PROC_V/LICENSE.md"
  section "jemalloc $JEMALLOC_V (BSD 2-clause): COPYING" "$SRC/jemalloc-$JEMALLOC_V/COPYING"
} >"$N"
rm -rf "$H"
[ -s "$N" ] || fail "THIRD-PARTY-NOTICES.txt came out empty"

# ---- 5. keep the result, drop the work ----
mkdir -p "$OUT"
cp "$T" "$OUT/tmux"
cp "$N" "$OUT/THIRD-PARTY-NOTICES.txt"
chmod 755 "$OUT/tmux"
rm -rf "$WORK"
echo "Built tmux $TMUX_V: $OUT/tmux"
echo "$OUT/tmux"
