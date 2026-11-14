#!/bin/bash
# Checks that each file given is signed with the hardened runtime, for every processor it holds, and carries none of
# the entitlements that would open it again: a debugger let in (get-task-allow), DYLD_* variables read, other
# signers' libraries loaded, or memory made writable and executable. Without the runtime, any program of yours can
# attach a debugger to Smolcorp or slip a library into it (DYLD_INSERT_LIBRARIES), and read the keys it holds in
# memory. scripts/build-app.sh runs it on the app, the smolcorp command, tmux and smolcorp-spawn before it says the build is done,
# so a build without the runtime fails there; scripts/test-copy.sh and CI run it on what they build.
#
#   scripts/check-runtime.sh FILE...      an app (checked as its main program) or a program
#
# A program named tmux must also be the guarded tmux scripts/build-tmux.sh builds (third_party/tmux/guard.patch):
# run with an empty environment and a socket that can't exist, it stops before it connects and says why. A stock
# tmux in the app would let a hire's own tmux server read and write files, and run programs, as you.
#
# Says each file that fails and why, and ends with 1 if any did. It signs nothing, reads no keychain, and runs
# nothing but a tmux it's given, that way.

set -euo pipefail

[ "$#" -gt 0 ] || { echo "Usage: scripts/check-runtime.sh FILE..."; exit 2; }

# Entitlements that undo what the runtime is for. Smolcorp needs none of them.
OPENERS=(
  com.apple.security.get-task-allow
  com.apple.security.cs.debugger
  com.apple.security.cs.allow-dyld-environment-variables
  com.apple.security.cs.disable-library-validation
  com.apple.security.cs.allow-unsigned-executable-memory
  com.apple.security.cs.disable-executable-page-protection
)

failed=0
for file in "$@"; do
  if [ ! -e "$file" ]; then
    echo "$file: missing."
    failed=1
    continue
  fi
  # An app's processors are its main program's.
  program="$file"
  if [ -d "$file" ]; then
    name="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$file/Contents/Info.plist" 2>/dev/null || true)"
    program="$file/Contents/MacOS/$name"
  fi
  # Each processor's slice has a signature of its own, and entitlements of its own: all of them must have the
  # runtime, and none an opener. A file lipo can't read is checked once, as codesign sees it.
  archs="$(lipo -archs "$program" 2>/dev/null || true)"
  for arch in ${archs:-native}; do
    which=(--arch "$arch")
    [ "$arch" = native ] && which=()
    # Read whole, then searched: a grep -q stopping at its first match would fail the pipe under pipefail.
    signature="$(codesign --display --verbose=2 "${which[@]}" "$file" 2>&1 || true)"
    # flags=0x10000(runtime) with a certificate, flags=0x10002(adhoc,runtime) ad hoc.
    if ! grep -Eq '^CodeDirectory .*flags=0x[0-9a-f]+\(([a-z-]+,)*runtime(,[a-z-]+)*\)' <<<"$signature"; then
      echo "$file ($arch): signed without the hardened runtime."
      failed=1
    fi
    # Entitlements that can't be read can't be said to be safe: that fails too.
    if ! entitlements="$(codesign --display --entitlements - --xml "${which[@]}" "$file" 2>/dev/null)"; then
      echo "$file ($arch): its entitlements can't be read."
      failed=1
      continue
    fi
    for key in "${OPENERS[@]}"; do
      if grep -qF "<key>$key</key>" <<<"$entitlements"; then
        echo "$file ($arch): carries $key, which opens it to other programs again."
        failed=1
      fi
    done
  done
  if [ "$(basename "$program")" = tmux ]; then
    status=0
    said="$(env -i "$program" -S /dev/null/smolcorp-guard-check has-session 2>&1)" || status=$?
    if [ "$status" != 98 ] || ! grep -q '^smolcorp guard: no server pid given' <<<"$said"; then
      echo "$file: isn't the guarded tmux (scripts/build-tmux.sh applies third_party/tmux/guard.patch); it said: $said"
      failed=1
    fi
  fi
done
exit "$failed"
