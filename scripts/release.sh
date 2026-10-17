#!/bin/bash
# Builds the download for a GitHub release: build/Smolcorp.zip, a
# universal app for Apple silicon and Intel Macs, and prints its SHA-256 for
# the release notes.
#
#   scripts/release.sh
#
# The app is signed ad hoc, not notarised, so macOS asks people to approve it
# once in System Settings → Privacy & Security. The README explains how.

set -euo pipefail

cd "$(dirname "$0")/.."
scripts/build-app.sh universal

ZIP="build/Smolcorp.zip"
rm -f "$ZIP"
ditto -c -k --keepParent "build/Smolcorp.app" "$ZIP"
echo "$ZIP ($(lipo -archs "build/Smolcorp.app/Contents/MacOS/AgentDeck"))"
echo "SHA-256: $(shasum -a 256 "$ZIP" | cut -d ' ' -f 1)"
