#!/bin/bash
# The release gate: every check a release must pass before anything that
# ships is built, signed or sent, in this one file (RELGATE1). No other
# script keeps a copy of any of it. Each path that makes or sends a download
# runs it first and stops if it fails:
#
#   scripts/release.sh            the Mac app, its DMG and zip (--dmg-only too)
#   scripts/release-desktop.sh    the Windows and Linux downloads
#   scripts/make-dmg.sh           run on its own (release.sh runs it after its
#                                 own gate, in a subshell: see below)
#   scripts/build-app.sh          when it would sign with a Developer ID (release.sh
#                                 runs it after its own gate, in a subshell too)
#   scripts/publish-release.sh    before it checks or sends anything
#
# A CI job that built or sent a download would run it too. None does: CI's
# builds go nowhere, and the Linux tmux it keeps is checked again by
# release-desktop.sh after this gate. desktop/releasegate_test.go holds each
# of these lines.
#
# Each of them sources it (. scripts/release-gate.sh), so a check that fails
# stops the script that sourced it, before anything is built. Run on its own,
# scripts/release-gate.sh runs every gate and says whether they passed.
#
# The gates, in order (RELEASE_GATES below names them):
#
#   1. pins: the go first on PATH and the C compiler, found once at the start
#      by their real paths and logged with their SHA-256 (SA-X11). Every go
#      and C compiler this gate runs, and every one a script that sourced it
#      runs (release-desktop.sh's builds, build-app.sh's when it builds a
#      release), is those, with /usr/bin/env -i and a fixed list of settings
#      (go_env, through pinned_env and pinned_go): GOFLAGS
#      =-mod=readonly over any go env file or the toolchain's go.env, GOENV
#      and GOWORK off, GOTOOLCHAIN=local, no CGO_* of the caller's, go's
#      folders made fresh for the run and removed when it ends. That go is
#      go.mod's release line: the same major.minor as its go line, at or past
#      its patch (FXCITRIAGE), checked once the files here are HEAD's tree
#      (gate 3), so it reads the go.mod that goes out, and before
#      check-public.sh and any build.
#   2. one-commit: a release is one squashed commit of the final files,
#      signed (the -S below; this gate doesn't check the signature), made
#      straight onto main as GitHub has it (origin/main). The branches the
#      work was done on (a feature branch, release/<version>) never go out,
#      and neither does their history. HEAD is that commit: it has one
#      parent, and that parent is origin/main. Anything else (a branch with
#      its own commits, a merge, a commit on an older main) stops it. HEAD
#      is read as a push sends it, so replace refs and a grafts file can't
#      make it look like another commit. There is no way around it.
#   3. heads-files: scripts/check-public.sh checks exactly the files that go
#      out: HEAD's, with nothing changed or added beside them, before and
#      after it runs. A checkout with files git is told not to look at
#      (assume-unchanged, skip-worktree, a sparse checkout, untracked files
#      status is set not to list) or with Go code git ignores beside the
#      code stops it too.
#   4. tmux-test: the guarded tmux (the Linux downloads' and, since X4, the
#      Mac's) exits unless the core gives it SMOLCORP_TMUX_SERVER. HEAD's
#      tree must hold the test that says the core never runs it without that
#      variable, TestNoExecOfTheBundledTmuxWithoutSmolcorpTmuxServer by that
#      exact name, in desktop/, as a test of the package's Mac and Linux
#      builds alike. It must pass here, with none of this machine's Smolcorp
#      or tmux settings (SMOLCORP_*, TMUX*): not failed, not skipped. And run
#      again with SMOLCORP_TMUXGATE_CONTROL=1, it must fail with one fixed
#      line, the one its recorder gives a start without the variable, so it
#      is known to look (TMUXGATE1).
#   5. own-tests: the release scripts' and this gate's own tests run here
#      too: go test -run '^TestRelease' ./desktop, with
#      SMOLCORP_RELEASE_GATE_ROWS=1. Only that variable runs the heavy rows,
#      dry runs of the release in throwaway repos (each pins go and cc, and
#      the bundled tmux's rows build and run its test twice); an ordinary go
#      test of ./desktop skips them, as they would take it past go's 10
#      minutes. So they run on every release, and they must all pass: none
#      failed or skipped, each heavy test passed by name.
#   6. forged-events: a desk can't send an event as another desk (SA-X10,
#      SA-X11): the forged-event table is in each system's build of the
#      desktop package and passes here.
#
# To make that commit from the branch you finished on, and release from it:
#
#   git fetch origin main
#   git checkout --detach "$(git commit-tree -S -p origin/main -m 'Smolcorp <version>' 'HEAD^{tree}')"
#   scripts/release.sh, then scripts/release-desktop.sh
#
# release.sh runs build-app.sh and make-dmg.sh in subshells after its gate
# passed: there the gate only checks the files are still the tree it passed.
# It knows it passed by a mark it sets readonly and never exports
# (release_gate_passed), so a variable of the caller's can't stand for it.
# Like the pins, that holds against the caller's settings, not against a
# caller who controls what bash runs before this starts (PATH, BASH_ENV):
# whoever does chooses the gate.

set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
RELEASE_GATES=(pins one-commit heads-files tmux-test own-tests forged-events)
if [[ "$(declare -p release_gate_passed 2>/dev/null)" == "declare -r release_gate_passed="* ]]; then
  if [ "$release_gate_passed" != "${TREE:-}" ] || ! heads_tree; then
    echo "Stopped: the files changed after the release gate passed, so they aren't the tree it checked ($release_gate_passed). Run this again."
    exit 1
  fi
  echo "The release gate passed in this run already, for HEAD's tree ($TREE)."
  return 0
fi
if ! unset release_gate_passed 2>/dev/null; then
  echo "Stopped: release_gate_passed is set readonly from elsewhere, not by this gate."
  exit 1
fi
# go's folders stay after the gate only for a script that sourced it and set
# release_gate_builds_go=1 in its own shell, unexported (see the end): never
# for a setting of the caller's.
if [ "$(declare -p release_gate_builds_go 2>/dev/null)" != 'declare -- release_gate_builds_go="1"' ] &&
  ! unset release_gate_builds_go 2>/dev/null; then
  echo "Stopped: release_gate_builds_go is set readonly from elsewhere, not by a release script."
  exit 1
fi
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
# logged: a cache grows large; when the gate passes, at once, unless the
# script that sourced it builds with them (see the end). go's temp files go
# in the temp folder named
# now (TMPDIR, or /tmp), where go makes a fresh folder for each build: not in
# this one, whose longer path would leave the tests' sockets too long a name.
system_path=/usr/bin:/bin:/usr/sbin:/sbin
temp_dir="${TMPDIR:-/tmp}"
go_run="$(mktemp -d "$temp_dir/smolcorp-go.XXXXXX")"
# release-desktop.sh sets it once its builds may leave a .syso beside the code.
remove_syso=
on_exit() {
  if [ -n "$remove_syso" ]; then
    rm -f cmd/smolcorp/rsrc_windows_*.syso
  fi
  # Removed already (release_gate_go_done, below): nothing more to do.
  [ -d "$go_run" ] || return 0
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
# Every go this gate and the script that sourced it run, and every go the
# checks they start, gets only these settings (SA-X11, RT126SAX11R3 B4):
# /usr/bin/env -i, then this list,
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
# control run, the release's own tests).
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
# A script that kept go's folders calls this once its last build is done:
# they go then, and an exit trap it sets after that is its own.
release_gate_go_done() {
  trap - EXIT
  on_exit
}
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
if [ "${release_gate_builds_go:-}" = 1 ]; then
  echo "go's folders: kept for the builds after the gate, removed when they're done."
else
  echo "go's folders: removed as soon as the gate passes."
fi
# Every git this gate runs (git_here), and check-public.sh with its own git and
# python3, run without the caller's choice of Xcode (DEVELOPER_DIR, SDKROOT,
# TOOLCHAINS), as xcrun is asked above (FXGOPINR2): on a Mac, git and python3
# in /usr/bin are xcrun's stand-ins, and a DEVELOPER_DIR naming a folder with
# no Xcode in it makes them fail, so git would say nothing and the gate stop
# for the wrong reason. They keep the rest of the caller's settings. Nothing
# else changes: go and the C compiler get only go_env, and the script that
# sourced this keeps the caller's Xcode for its own steps.
without_xcode() { /usr/bin/env -u DEVELOPER_DIR -u SDKROOT -u TOOLCHAINS "$@"; }
git_here() { without_xcode git "$@"; }
# git looks at this folder, the one check-public.sh and the build read, and
# at its own index: not at another one a caller's environment or the repo's
# config (core.worktree) names.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE
top="$(git_here rev-parse --show-toplevel 2>/dev/null || true)"
if [ "$top" != "$(pwd -P)" ]; then
  echo "Stopped: git here looks at another folder (${top:-none}), not this one ($(pwd -P))."
  echo "Unset core.worktree (git config --unset core.worktree), then run this again."
  exit 1
fi

# HEAD as a push sends it (gate 2 at the top): the parent and tree lines of its
# own commit object, which a grafts file (info/grafts) doesn't change, read
# with no replace refs (refs/replace). Every git command below sees the real
# objects too.
export GIT_NO_REPLACE_OBJECTS=1
head_header() { git_here cat-file commit HEAD 2>/dev/null | sed '/^$/,$d'; }
head_parents() { head_header | sed -n 's/^parent //p'; }
head_tree() { head_header | sed -n 's/^tree //p'; }

# One commit straight onto origin/main, and nothing else (gate 2 at the top).
# HEAD's parents, one per line: exactly origin/main means one parent, and the
# right one.
BASE="$(git_here rev-parse --verify --quiet 'refs/remotes/origin/main^{commit}' || true)"
if [ -z "$BASE" ]; then
  echo "Stopped: origin/main isn't here, so the release commit can't be checked."
  echo "Run git fetch origin main, make the release commit (see the top of scripts/release-gate.sh), then this again."
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
  changes="$(git_here status --porcelain --untracked-files=all)" && [ -z "$changes" ]
}
# A release is built from what's committed, and nothing else: no changes
# waiting, and no stray files, ignored or not, beside the pages it seals in.
if ! clean || [ -n "$(git_here ls-files --others --ignored --exclude-standard Resources/web)" ]; then
  echo "Commit or set aside these first: a release is built from what's committed."
  git_here status --short --untracked-files=all || true
  git_here ls-files --others --ignored --exclude-standard Resources/web
  exit 1
fi
# Files git is told not to look at: git status and git write-tree pass over
# their changes on disk, and check-public.sh reads the disk. git ls-files -v
# tags every ordinary file H; assume-unchanged ones are lowercase, and
# skip-worktree ones (which a sparse checkout makes) S.
unseen() { git_here ls-files -v | grep -v '^H ' || true; }
sparse() { [ "$(git_here config --bool core.sparseCheckout || true)" = true ]; }
# So the files here are HEAD's tree, exactly (gate 3 at the top): the index makes
# that tree, HEAD still has it, nothing differs from it, and git looks at
# every file.
heads_tree() {
  [ "$(git_here write-tree)" = "$TREE" ] && [ "$(head_tree)" = "$TREE" ] && clean &&
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
# The go pinned at the top is go.mod's release line (gate 1's last check): the
# same major.minor as go.mod's go line, at or past its patch, as CI installs
# and the Mac Studio checks with. A go of another line lays out and reads the
# files otherwise (CI's gofmt failed on a file another go found clean,
# FXCITRIAGE). It's checked here, once the files here are HEAD's tree: go.mod
# on disk is then the one that goes out and the one go reads, even where a
# checkout had left it out (a sparse one stops just above). No go has built or
# run anything yet. internal/goline holds the rule for scripts/build-app.sh and
# the tests; this is the gate's own copy of it, as the gate runs no go program
# before it.
go_mod_line="$(sed -n -E 's/^go[[:space:]]+([0-9]+\.[0-9]+(\.[0-9]+)?)[[:space:]]*$/\1/p' go.mod 2>/dev/null | head -n 1 || true)"
if [ -z "$go_mod_line" ]; then
  echo "Stopped: HEAD's go.mod has no go line (such as go 1.27.2), so there's no release line to check this go ($GO) against."
  echo "Make the release commit with go.mod's go line in it, then run this again."
  exit 1
fi
go_here="$(pinned_go env GOVERSION || true)"
go_is_go_mods() {
  local major minor patch
  [[ $go_mod_line =~ ^([0-9]+)\.([0-9]+)(\.([0-9]+))?$ ]] || return 1
  major="${BASH_REMATCH[1]}" minor="${BASH_REMATCH[2]}" patch="${BASH_REMATCH[4]:-0}"
  [[ $go_here =~ ^go([0-9]+)\.([0-9]+)(\.([0-9]+))? ]] || return 1
  ((10#${BASH_REMATCH[1]} == 10#$major && 10#${BASH_REMATCH[2]} == 10#$minor && 10#${BASH_REMATCH[4]:-0} >= 10#$patch))
}
if ! go_is_go_mods; then
  echo "Stopped: this go ($GO) is ${go_here:-of no version it says}, and go.mod asks for go $go_mod_line or a later patch of its line."
  echo "Put a go of that line first on PATH (CI installs exactly go.mod's), then run this again."
  exit 1
fi
# A .syso beside the code is linked into the Windows programs, and git ignores
# them now (scripts/mkwinres writes one for each Windows build below, and this
# script deletes it), so one left from elsewhere would slip in unseen.
stray="$( (git_here ls-files --others --ignored --exclude-standard -- '*.syso'; git_here ls-files -- '*.syso') | grep -v -e '^build/' -e '^\.build/' || true)"
if [ -n "$stray" ]; then
  echo "Remove these .syso files first: they would be linked into the Windows programs."
  echo "$stray"
  exit 1
fi
# Go code git ignores: go build compiles every .go file in each package
# folder it builds, ignored or not, so one there would be built in unseen.
# Ignored .go files elsewhere (a folder of notes, sandbox/) aren't built.
go_dirs() { git_here -c core.quotePath=false ls-files -- "$@" | sed -e 's,/[^/]*$,,' -e t -e 's,.*,.,' | sort -u; }
ignored_go="$(awk 'FILENAME == ARGV[1] { pkg[$0]; next } { d = $0; if (!sub(/\/[^\/]*$/, "", d)) d = "." } d in pkg' \
  <(go_dirs '*.go') <(git_here -c core.quotePath=false ls-files --others --ignored --exclude-standard -- '*.go' '*.s'))"
if [ -n "$ignored_go" ]; then
  echo "Remove these first: git ignores them, but Go compiles them into the programs."
  echo "$ignored_go"
  exit 1
fi
# Nothing of the maintainer's own goes out in a download.
without_xcode scripts/check-public.sh
# What it checked is still HEAD's tree: nothing changed while it ran.
if ! heads_tree; then
  echo "Stopped: the files changed while scripts/check-public.sh ran, so it didn't check HEAD's tree ($TREE). Run this again."
  exit 1
fi
# The bundled tmux's test (gate 4 at the top), in the desktop package only, run
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
grep_errors=$(git_here grep -q -E "^func $TMUX_TEST\\(" "$TREE" -- ":(glob)$TMUX_TEST_PACKAGE/*_test.go" 2>&1 >/dev/null) || status=$?
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
# The release's own tests (gate 5 at the top), run by the go and with the settings
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
RELEASE_ROWS_RUN=^TestRelease
RELEASE_ROWS_HEAVY=(TestReleaseDesktopBuildsOnlyFromOneCommitOntoMain TestReleaseDesktopChecksEveryShapeOfHEAD
  TestReleaseDesktopStopsWhenTheCheckedFilesArentHEADs TestReleaseDesktopStopsOnFilesThatDontGoOut
  TestReleaseDesktopReadsHEADAsAPushSendsIt TestReleaseDesktopStopsOnFilesGitDoesntLookAt
  TestReleaseDesktopNeedsTheBundledTmuxTestPassing TestReleaseDesktopStopsUnlessTheForgedEventTableIsWhole
  TestReleaseGateRunsGitWithoutTheCallersXcode)
echo "The release's own tests in ./$TMUX_TEST_PACKAGE ($RELEASE_ROWS):"
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
  echo "Stopped: the release's own tests didn't all pass in ./$TMUX_TEST_PACKAGE (one failed, was skipped, or didn't run)."
  echo "Each of these must pass with $RELEASE_ROWS: ${RELEASE_ROWS_HEAVY[*]}"
  exit 1
fi
if ! heads_tree; then
  echo "Stopped: the files changed while the release's own tests ran, so they aren't HEAD's tree ($TREE) any more. Run this again."
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

# Passed. go's folders for the run stay only for a script that builds with
# this go and these settings after it (release-desktop.sh, release.sh and
# build-app.sh set release_gate_builds_go=1 first; the trap, or
# release_gate_go_done, removes them). Otherwise they go now, so a trap of the
# caller's own can't leave them.
if [ "${release_gate_builds_go:-}" != 1 ]; then
  trap - EXIT
  on_exit
fi
readonly release_gate_passed="$TREE"
echo "The release gate passed for HEAD's tree ($TREE): ${RELEASE_GATES[*]}."
