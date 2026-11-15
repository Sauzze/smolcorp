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
#   4. The guarded tmux (the Linux downloads' and, since X4, the Mac's) exits
#      unless the core gives it SMOLCORP_TMUX_SERVER. HEAD's tree must hold
#      the test that says the core never runs it without that variable,
#      TestNoExecOfTheBundledTmuxWithoutSmolcorpTmuxServer by that exact
#      name, in desktop/, as a test of the package's Mac and Linux builds
#      alike. It must pass here, with none of this machine's Smolcorp or
#      tmux settings (SMOLCORP_*, TMUX*): not failed, not skipped. And run
#      again with SMOLCORP_TMUXGATE_CONTROL=1, it must fail with one fixed
#      line, the one its recorder gives a start without the variable, so it
#      is known to look. Until it does all that, nothing is built.
#   5. This script's own tests run here too, before anything is built:
#      go test -run '^TestReleaseDesktop' ./desktop, with
#      SMOLCORP_RELEASE_GATE_ROWS=1. Only that variable runs the heavy rows,
#      dry runs of this script in throwaway repos (each pins go and cc, and
#      the bundled tmux's rows build and run its test twice); an ordinary go
#      test of ./desktop skips them, as they would take it past go's 10
#      minutes. So they run on every release, and they must all pass: none
#      failed or skipped, each heavy test passed by name.
#
# To make that commit from the branch you finished on, and build from it:
#
#   git fetch origin main
#   git checkout --detach "$(git commit-tree -S -p origin/main -m 'Smolcorp <version>' 'HEAD^{tree}')"
#   scripts/release-desktop.sh

set -euo pipefail

cd "$(dirname "$0")/.."
# The go this release runs, every time (SA-X11): the one first on PATH now, at
# the start, by its real path, so a go put earlier on PATH while this runs (by
# a test, say) isn't the one that checks or builds. It goes in the log first,
# with its version and SHA-256.
go_found="$(type -P go || true)"
if [ -z "$go_found" ] || ! GO="$(realpath "$go_found")"; then
  echo "Stopped: no go on PATH."
  exit 1
fi
if [ -z "${HOME:-}" ]; then
  echo "Stopped: HOME isn't set."
  exit 1
fi
# go's own folders for this run (SA-X11): its build cache, module cache and
# GOPATH, in a folder made fresh here, never the caller's or go's usual ones,
# so no cache prepared beforehand can hand a check a test binary built from
# other code. Removed when this ends, done or failed, after its size is
# logged: a cache grows large. go's temp files go in the temp folder named
# now (TMPDIR, or /tmp), where go makes a fresh folder for each build: not in
# this one, whose longer path would leave the tests' sockets too long a name.
system_path=/usr/bin:/bin:/usr/sbin:/sbin
temp_dir="${TMPDIR:-/tmp}"
go_run="$(mktemp -d "$temp_dir/smolcorp-go.XXXXXX")"
remove_syso=
on_exit() {
  if [ -n "$remove_syso" ]; then
    rm -f cmd/smolcorp/rsrc_windows_*.syso
  fi
  echo "go's folders for this run ($go_run): $(du -sk "$go_run" 2>/dev/null | cut -f 1) KB, removed."
  # go makes the modules it unpacks read-only.
  chmod -R u+w "$go_run" 2>/dev/null || true
  rm -rf "$go_run"
}
trap on_exit EXIT
# The module downloads go reads, offline: go's own download folder for this
# account (as go itself names it, with none of the caller's settings), read
# as a proxy (GOPROXY=file://), so go unpacks each module into the run's
# module cache only after its zip matches go.sum (GOFLAGS=-mod=readonly: no
# module that go.sum doesn't name; GOSUMDB=off: nothing asked of the network).
go_downloads="$(/usr/bin/env -i HOME="$HOME" PATH="$system_path" GOENV=off GOTOOLCHAIN=local LANG=C "$GO" env GOMODCACHE)/cache/download"
# Every go this script runs, and every go the checks it runs start, gets only
# these settings (SA-X11, RT126SAX11R3 B4): /usr/bin/env -i, then this list,
# each set here. Nothing else of the caller's crosses, a setting nobody has
# thought of included: no GOFLAGS -exec, -toolexec or -overlay from the caller,
# a go env file or the toolchain's own go.env (a set GOFLAGS replaces theirs),
# no go.work, no other toolchain than this go's own whatever go.mod's
# toolchain line asks, no GOROOT but its own, no folders of libraries or
# headers the C compiler searches first (LIBRARY_PATH, CPATH and the like), no
# linker it finds through COMPILER_PATH or CCC_OVERRIDE_OPTIONS, and on PATH
# the system's own folders only: go and the C compiler are named by their
# real paths, found before it. GOOS and GOARCH are set at each call: this
# machine's for a check (a program built for another system would run through
# a go_<system>_<arch>_exec), the one asked for a build.
# Nor this machine's Smolcorp or tmux settings (TMUXGATE1): every pane of a
# Smolcorp tmux has SMOLCORP_TMUX_SERVER, TMUX and TMUX_PANE, and the bundled
# tmux's test would pass because the core passed them on, not because it gives
# the tmux its own. A step adds only the one it means to give (the tmux test's
# control run, this script's own tests).
go_env=(HOME="$HOME" PATH="$system_path" TMPDIR="$temp_dir" GOCACHE="$go_run/cache" GOMODCACHE="$go_run/modules"
  GOPATH="$go_run/gopath" GOPROXY="file://$go_downloads" GOSUMDB=off GOFLAGS=-mod=readonly GOENV=off GOWORK=off
  GOTOOLCHAIN=local LANG=C)
host_os="$(/usr/bin/env -i "${go_env[@]}" "$GO" env GOHOSTOS)"
host_arch="$(/usr/bin/env -i "${go_env[@]}" "$GO" env GOHOSTARCH)"
# The C compiler, found once here too, by its real path (SA-X11): it builds the
# cgo parts of what go tests and links the test binary whose results a check
# reads, so a caller's CC, or a cc put earlier on PATH while this runs, would
# write that binary. On a Mac, the active developer folder's clang and the SDK
# it builds with, asked of xcrun by its own path, with no lookup cache and none
# of the caller's settings (DEVELOPER_DIR, SDKROOT, TOOLCHAINS); elsewhere, the
# cc first on PATH now.
if [ "$host_os" = darwin ]; then
  xcrun_here() { /usr/bin/env -i HOME="$HOME" PATH="$system_path" LANG=C /usr/bin/xcrun --no-cache "$@" 2>/dev/null; }
  cc_found="$(xcrun_here --find clang || true)"
  sdk_found="$(xcrun_here --show-sdk-path || true)"
  if [ -z "$sdk_found" ] || ! SDK="$(realpath "$sdk_found")"; then
    echo "Stopped: no Mac SDK found (xcrun --show-sdk-path). Install Xcode's command line tools, then run this again."
    exit 1
  fi
else
  cc_found="$(type -P cc || true)"
fi
if [ -z "$cc_found" ] || ! CC_PROGRAM="$(realpath "$cc_found")"; then
  echo "Stopped: no C compiler found (on a Mac, xcrun --find clang; elsewhere, cc on PATH)."
  exit 1
fi
# CC is this one, and on a Mac SDKROOT is this SDK.
go_env+=(CC="$CC_PROGRAM")
if [ "$host_os" = darwin ]; then
  go_env+=(SDKROOT="$SDK")
fi
# pinned_env runs a program with those settings and no others; pinned_go runs
# this go with them, for this machine.
pinned_env() { /usr/bin/env -i "${go_env[@]}" "$@"; }
pinned_go() { pinned_env GOOS="$host_os" GOARCH="$host_arch" "$GO" "$@"; }
go_version="$(pinned_go version)"
go_sum="$(shasum -a 256 "$GO" | cut -d ' ' -f 1)"
cc_sum="$(shasum -a 256 "$CC_PROGRAM" | cut -d ' ' -f 1)"
echo "go: $GO"
echo "  $go_version"
echo "  sha256 $go_sum"
echo "cc: $CC_PROGRAM"
echo "  sha256 $cc_sum"
if [ "$host_os" = darwin ]; then
  echo "  sdk $SDK"
fi
echo "go's settings, the only ones it gets (plus GOOS, GOARCH and CGO_ENABLED where a step sets them):"
printf '  %s\n' "${go_env[@]}"
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
# The bundled tmux's test (point 4 above), in the desktop package only, run
# by the go and with the settings pinned at the top (go_env):
# found in HEAD's tree by its exact name, then a test in go's own list of the
# package's tests in its darwin build and each Linux one (a comment, a method,
# a copy in another package or under testdata, or a stub only one system
# builds doesn't count). It runs twice here: as it is, where it must pass, and
# with SMOLCORP_TMUXGATE_CONTROL=1, where it must start the stand-in tmux once
# without SMOLCORP_TMUX_SERVER, through its recorder, and fail with exactly one
# TMUX_CONTROL_LINE, so a test that checks nothing can't pass. The pins hold
# against the caller's go settings, not against a caller who controls PATH
# before this starts: whoever does chooses the go that is pinned.
TMUX_TEST=TestNoExecOfTheBundledTmuxWithoutSmolcorpTmuxServer
TMUX_TEST_PACKAGE=desktop
TMUX_CONTROL=SMOLCORP_TMUXGATE_CONTROL=1
TMUX_CONTROL_LINE="tmuxgate control: a start without SMOLCORP_TMUX_SERVER was let through"
# git grep says 1 both for "no match" and for a file it couldn't read (an
# object it can't open), so anything it writes to stderr counts as a failure
# to read, whatever its exit.
status=0
grep_errors=$(git grep -q -E "^func $TMUX_TEST\\(" "$TREE" -- ":(glob)$TMUX_TEST_PACKAGE/*_test.go" 2>&1 >/dev/null) || status=$?
if [ -n "$grep_errors" ] || { [ "$status" != 0 ] && [ "$status" != 1 ]; }; then
  echo "Stopped: git grep couldn't read HEAD's tree ($TREE) for $TMUX_TEST."
  [ -z "$grep_errors" ] || printf '%s\n' "$grep_errors" | sed 's/^/  /'
  exit 1
elif [ "$status" = 1 ]; then
  echo "Stopped: HEAD's tree has no $TMUX_TEST in $TMUX_TEST_PACKAGE/."
  echo "The bundled tmux exits unless the core gives it SMOLCORP_TMUX_SERVER, and that test says it always does."
  echo "A release waits until the test is in, under that exact name, and passes."
  exit 1
fi
# go for a system and processor, the pinned way; cgo only on this machine's
# own system, as the other's CI builds it.
tmux_go() { # GOOS GOARCH ARGS...
  local system="$1" arch="$2"
  shift 2
  if [ "$system" = "$host_os" ] && [ "$arch" = "$host_arch" ]; then
    pinned_go "$@"
  else
    pinned_env CGO_ENABLED=0 GOOS="$system" GOARCH="$arch" "$GO" "$@"
  fi
}
# Whether the test is in that build's list of tests: the _testmain.go go list
# writes for the package, which holds what go test would run.
tmux_listed() { # GOOS GOARCH
  local testmain
  testmain="$(tmux_go "$1" "$2" list -test -f '{{.ImportPath}}{{"\t"}}{{join .GoFiles "\t"}}' "./$TMUX_TEST_PACKAGE" |
    awk -F '\t' '$1 ~ /\.test$/ { print $2 }')" || return 1
  [ -n "$testmain" ] && grep -q -x -F -e "$(printf '\t{"%s", _test.%s},' "$TMUX_TEST" "$TMUX_TEST")" \
    -e "$(printf '\t{"%s", _xtest.%s},' "$TMUX_TEST" "$TMUX_TEST")" "$testmain"
}
for build in "darwin $host_arch" "linux amd64" "linux arm64"; do
  if ! tmux_listed $build; then
    echo "Stopped: $TMUX_TEST isn't a test in ./$TMUX_TEST_PACKAGE's ${build/ //} build."
    echo "It must be one in every system's build of the package, the Mac's and Linux's, not a stub one of them has."
    exit 1
  fi
done
tmux_test() { # [SETTING]
  pinned_env "$@" GOOS="$host_os" GOARCH="$host_arch" "$GO" test -count=1 -v -run "^$TMUX_TEST\$" "./$TMUX_TEST_PACKAGE" 2>&1
}
echo "$TMUX_TEST in ./$TMUX_TEST_PACKAGE:"
status=0
out="$(tmux_test)" || status=$?
echo "$out" | sed 's/^/  /'
# Passed, as itself, with nothing in it failed or skipped.
if [ "$status" != 0 ] || ! grep -q -x -E -e "--- PASS: $TMUX_TEST \\([0-9.]+s\\)" <<<"$out" ||
  grep -q -E -e '^ *--- (FAIL|SKIP):' <<<"$out"; then
  echo "Stopped: $TMUX_TEST didn't pass in ./$TMUX_TEST_PACKAGE (it failed, was skipped, or doesn't run on this machine)."
  exit 1
fi
echo "$TMUX_TEST in ./$TMUX_TEST_PACKAGE, its control run ($TMUX_CONTROL):"
status=0
out="$(tmux_test "$TMUX_CONTROL")" || status=$?
echo "$out" | sed 's/^/  /'
# Failed, as itself, with the line its recorder gives the one broken start,
# once.
if [ "$status" = 0 ] || ! grep -q -x -E -e "--- FAIL: $TMUX_TEST \\([0-9.]+s\\)" <<<"$out" ||
  [ "$(grep -c -x -E -e " +[^ ]+\\.go:[0-9]+: $TMUX_CONTROL_LINE" <<<"$out" || true)" != 1 ]; then
  echo "Stopped: $TMUX_TEST didn't fail its control run in ./$TMUX_TEST_PACKAGE."
  echo "With $TMUX_CONTROL it must start the stand-in tmux once without SMOLCORP_TMUX_SERVER, through its recorder,"
  echo "and fail with exactly one line: $TMUX_CONTROL_LINE"
  exit 1
fi
if ! heads_tree; then
  echo "Stopped: the files changed while $TMUX_TEST ran, so they aren't HEAD's tree ($TREE) any more. Run this again."
  exit 1
fi
# This script's own tests (point 5 above), run by the go and with the settings
# pinned at the top, eight rows at a time (each row's dry run has a time limit,
# and the bundled tmux's and the forged-event table's rows build as they go,
# each in its run's own fresh cache). RELEASE_ROWS is set here and only for
# this run (the caller's own never crosses env -i), so the heavy tests
# (RELEASE_ROWS_HEAVY) run on every release and nowhere else by default. The
# pinned go's folder comes first on their PATH, before the system's own
# folders: they hand it their own go list and go test. Passed means go test
# passed, nothing in it failed or was skipped, and each heavy test says it
# passed.
RELEASE_ROWS=SMOLCORP_RELEASE_GATE_ROWS=1
RELEASE_ROWS_RUN=^TestReleaseDesktop
RELEASE_ROWS_HEAVY=(TestReleaseDesktopBuildsOnlyFromOneCommitOntoMain TestReleaseDesktopChecksEveryShapeOfHEAD
  TestReleaseDesktopStopsWhenTheCheckedFilesArentHEADs TestReleaseDesktopStopsOnFilesThatDontGoOut
  TestReleaseDesktopReadsHEADAsAPushSendsIt TestReleaseDesktopStopsOnFilesGitDoesntLookAt
  TestReleaseDesktopNeedsTheBundledTmuxTestPassing TestReleaseDesktopStopsUnlessTheForgedEventTableIsWhole)
echo "This script's own tests in ./$TMUX_TEST_PACKAGE ($RELEASE_ROWS):"
status=0
out="$(pinned_env "$RELEASE_ROWS" GOOS="$host_os" GOARCH="$host_arch" PATH="$(dirname "$GO"):$system_path" \
  "$GO" test -count=1 -parallel 8 -v -run "$RELEASE_ROWS_RUN" "./$TMUX_TEST_PACKAGE" 2>&1)" || status=$?
# What each test and row came to, without go's lines for starting each one.
{ grep -v -E '^ *=== (RUN|PAUSE|CONT|NAME) ' <<<"$out" || true; } | sed 's/^/  /'
rows_passed() {
  local row
  if [ "$status" != 0 ] || grep -q -E -e '^ *--- (FAIL|SKIP):' <<<"$out"; then
    return 1
  fi
  for row in "${RELEASE_ROWS_HEAVY[@]}"; do
    grep -q -x -E -e "--- PASS: $row \\([0-9.]+s\\)" <<<"$out" || return 1
  done
}
if ! rows_passed; then
  echo "Stopped: this script's own tests didn't all pass in ./$TMUX_TEST_PACKAGE (one failed, was skipped, or didn't run)."
  echo "Each of these must pass with $RELEASE_ROWS: ${RELEASE_ROWS_HEAVY[*]}"
  exit 1
fi
if ! heads_tree; then
  echo "Stopped: the files changed while this script's own tests ran, so they aren't HEAD's tree ($TREE) any more. Run this again."
  exit 1
fi
# A desk can't send an event as another desk (SA-X10, SA-X11): the forged-event table is in each system's build of
# the desktop package and passes here, run by this go with its settings pinned, or nothing is built.
pinned_go run ./scripts/forgedevents -go "$GO" -cc "$CC_PROGRAM" -sdk "${SDK:-}" -work "$go_run" -modules "$go_downloads"
# Its tests ran the tree's code: what goes out is still HEAD's tree.
if ! heads_tree; then
  echo "Stopped: the files changed while the forged-event table ran, so they aren't HEAD's tree ($TREE) any more. Run this again."
  exit 1
fi
# The office the programs build in is exactly the build of its sources. Pinned
# as every check here is (see go_env at the top).
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
# on_exit (at the top) deletes it.
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
