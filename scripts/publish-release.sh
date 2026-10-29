#!/bin/bash
# Publishes a release on GitHub, as the latest, from what scripts/release.sh
# and scripts/release-desktop.sh built:
#
#   scripts/publish-release.sh --check   the checks and what it would do; nothing is sent
#   scripts/publish-release.sh           the same, then, once you type the version, publishes
#
# The branch's own history stays on this computer: what goes to GitHub is one
# new commit on origin/main whose files are the branch's as they are now, on
# release/<version>, merged into main by a pull request, tagged v<version>.
# The release is titled from RELEASE-NOTES.md's first line, the rest is its
# text, and the downloads are attached under their own names, so
# releases/latest/download/Smolcorp.dmg is always the newest Mac app, and
# releases/latest/download/FIRST-SETUP.md the prompt that sets it up.
#
# Needs: gh, signed in to the repository's account (gh auth status), and the
# builds in build/: the Mac app signed with a Developer ID and notarised.

set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="1.22.0"
BRANCH="release-next"
TAG="v$VERSION"
RELEASE_BRANCH="release/$VERSION"
APP="build/Smolcorp.app"
DMG="build/Smolcorp.dmg"
ASSETS=(
  build/Smolcorp.dmg
  build/Smolcorp.zip
  build/Smolcorp-Windows-x86_64.zip
  build/Smolcorp-Windows-arm64.zip
  build/Smolcorp-Linux-x86_64.tar.gz
  build/Smolcorp-Linux-arm64.tar.gz
  packaging/FIRST-SETUP.md
)
CHECK_ONLY=0
[ "${1:-}" = "--check" ] && CHECK_ONLY=1

# `check "why" command…` runs the command, and if it fails, says why it stopped there.
check() {
  local why="$1"
  shift
  "$@" >/dev/null 2>&1 || { echo "Stopped: $why"; exit 1; }
}

# A command's answer is read whole before it's searched: under pipefail, a
# grep -q that stops at its first match would fail the command it reads from.
says() { grep -qF -- "$2" <<<"$1"; }

echo "Smolcorp $VERSION, from $BRANCH"

# --- What's published: the branch's files, as committed, and nothing personal

check "the files have changes that aren't committed: a release is published from what's committed." \
  test -z "$(git status --porcelain --untracked-files=no)"
check "$BRANCH isn't here." git rev-parse --verify --quiet "$BRANCH^{commit}"
TREE="$(git rev-parse "$BRANCH^{tree}")"
check "$BRANCH isn't what's checked out, so the builds may not be its." test "$(git rev-parse HEAD^{tree})" = "$TREE"
# The names that must never be published are in check-public.sh, as hashes only.
check "the public check found something personal (scripts/check-public.sh says what)." scripts/check-public.sh
check "the release notes' title doesn't say $VERSION." grep -q "^# Smolcorp $VERSION" RELEASE-NOTES.md
TITLE="$(head -1 RELEASE-NOTES.md | sed 's/^# //')"

# --- The builds: there, this version, and the Mac app notarised

for FILE in "${ASSETS[@]}"; do
  check "$FILE is missing: scripts/release.sh builds the Mac's, scripts/release-desktop.sh the others." test -s "$FILE"
done
# The setup prompt is attached as it's committed, not as it may have been edited since.
git show "$TREE:packaging/FIRST-SETUP.md" | cmp -s - packaging/FIRST-SETUP.md \
  || { echo "Stopped: packaging/FIRST-SETUP.md isn't as $BRANCH has it: commit it, or put it back."; exit 1; }
check "$APP is missing." test -d "$APP"
check "$APP is version $(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$APP/Contents/Info.plist" 2>/dev/null), not $VERSION." \
  test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$APP/Contents/Info.plist")" = "$VERSION"
SIGNATURE="$(codesign --display --verbose=2 "$APP" 2>&1 || true)"
says "$SIGNATURE" "Authority=Developer ID Application:" || { echo "Stopped: $APP isn't signed with a Developer ID."; exit 1; }
says "$SIGNATURE" "flags=0x10000(runtime)" || { echo "Stopped: $APP is signed without the hardened runtime."; exit 1; }
check "$APP has no valid notarisation ticket stapled (xcrun stapler validate)." xcrun stapler validate "$APP"
check "$DMG has no valid notarisation ticket stapled (xcrun stapler validate)." xcrun stapler validate "$DMG"
check "Gatekeeper refuses $DMG." spctl --assess --type open --context context:primary-signature "$DMG"

# The app inside the DMG, which is what people download, is this version too.
MOUNT="$(mktemp -d)"
check "$DMG couldn't be opened to read its version." hdiutil attach -nobrowse -readonly -mountpoint "$MOUNT" "$DMG"
INSIDE="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$MOUNT/Smolcorp.app/Contents/Info.plist" 2>/dev/null || true)"
hdiutil detach -quiet "$MOUNT" || true
rmdir "$MOUNT" 2>/dev/null || true
[ "$INSIDE" = "$VERSION" ] || { echo "Stopped: the app in $DMG is version ${INSIDE:-unknown}, not $VERSION."; exit 1; }

# The Windows and Linux programs say this version, in the archives as attached.
UNPACKED="$(mktemp -d)"
trap 'rm -rf "$UNPACKED"' EXIT
for ARCHIVE in build/Smolcorp-Windows-x86_64.zip build/Smolcorp-Windows-arm64.zip; do
  check "$ARCHIVE can't be read." unzip -q -o "$ARCHIVE" -d "$UNPACKED"
done
for ARCHIVE in build/Smolcorp-Linux-x86_64.tar.gz build/Smolcorp-Linux-arm64.tar.gz; do
  check "$ARCHIVE can't be read." tar -xzf "$ARCHIVE" -C "$UNPACKED"
done
for PROGRAM in "$UNPACKED"/Smolcorp-Windows-*/smolcorp-cli.exe "$UNPACKED"/Smolcorp-Linux-*/smolcorp; do
  LC_ALL=C grep -a -q -F "$VERSION" "$PROGRAM" \
    || { echo "Stopped: ${PROGRAM#"$UNPACKED"/} isn't version $VERSION: run scripts/release-desktop.sh again."; exit 1; }
done

# --- GitHub: signed in, and this version not there yet

check "gh isn't installed (https://cli.github.com)." command -v gh
check "gh isn't signed in: gh auth login." gh auth status
REPO="$(gh repo view --json nameWithOwner -q .nameWithOwner)"
check "origin/main can't be fetched." git fetch --quiet origin main
if git ls-remote --exit-code --heads origin "$RELEASE_BRANCH" >/dev/null 2>&1; then
  echo "Stopped: $RELEASE_BRANCH is on GitHub already."; exit 1
fi
if git ls-remote --exit-code --tags origin "$TAG" >/dev/null 2>&1; then
  echo "Stopped: $TAG is on GitHub already."; exit 1
fi
if gh release view "$TAG" >/dev/null 2>&1; then
  echo "Stopped: the release $TAG is on GitHub already."; exit 1
fi

echo
echo "Ready to publish to $REPO:"
echo "  one commit on origin/main ($(git rev-parse --short origin/main)) with $BRANCH's files, on $RELEASE_BRANCH"
echo "  a pull request into main, merged, and $TAG on the merge"
echo "  the release \"$TITLE\", the latest, with:"
for FILE in "${ASSETS[@]}"; do echo "    $(basename "$FILE")"; done
echo "    SHA256SUMS.txt"
if [ "$CHECK_ONLY" = 1 ]; then
  echo
  echo "Checked: nothing was sent."
  exit 0
fi

echo
read -r -p "Type $VERSION to publish it: " ANSWER
[ "$ANSWER" = "$VERSION" ] || { echo "Stopped: nothing was sent."; exit 1; }

# --- Publish

# One commit, the branch's files, on main as GitHub has it: its own history isn't sent.
COMMIT="$(git commit-tree "$TREE" -p origin/main -m "Smolcorp $VERSION" -m "See RELEASE-NOTES.md for what's new.")"
check "$RELEASE_BRANCH couldn't be pushed." git push origin "$COMMIT:refs/heads/$RELEASE_BRANCH"

BODY="$(mktemp)"
trap 'rm -rf "$UNPACKED" "$BODY"' EXIT
tail -n +2 RELEASE-NOTES.md | sed '1{/^$/d;}' > "$BODY"
PR="$(gh pr create --base main --head "$RELEASE_BRANCH" --title "Smolcorp $VERSION" --body-file "$BODY")"
echo "Pull request: $PR"
check "the pull request couldn't be merged: $PR says why." gh pr merge "$PR" --merge
MERGED="$(gh pr view "$PR" --json mergeCommit -q .mergeCommit.oid)"
[ -n "$MERGED" ] || { echo "Stopped: the pull request's merge commit isn't known yet: $PR"; exit 1; }
check "main couldn't be fetched after the merge." git fetch --quiet origin main
check "$TAG couldn't be made." git tag -a "$TAG" -m "Smolcorp $VERSION" "$MERGED"
check "$TAG couldn't be pushed." git push origin "$TAG"

# Each download's SHA-256, beside it on the release page and in a file of their own, kept out of build/.
SUMS="$UNPACKED/SHA256SUMS.txt"
for FILE in "${ASSETS[@]}"; do
  echo "$(shasum -a 256 "$FILE" | cut -d ' ' -f 1)  $(basename "$FILE")"
done > "$SUMS"
{
  echo
  echo "### SHA-256"
  echo
  echo '```'
  cat "$SUMS"
  echo '```'
} >> "$BODY"
check "the release couldn't be made: gh release view $TAG says how far it got." \
  gh release create "$TAG" "${ASSETS[@]}" "$SUMS" --title "$TITLE" --notes-file "$BODY" --latest --verify-tag

echo
echo "Published: $(gh release view "$TAG" --json url -q .url)"
echo "The Mac download: https://github.com/$REPO/releases/latest/download/Smolcorp.dmg"
