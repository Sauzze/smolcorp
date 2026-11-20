#!/bin/sh
# Copies the office's drawing code into the viewer, unchanged, for a local preview
# (vercel dev, or any static server over public/). The copy is git-ignored: the
# one source is Resources/web/office.js, and a deploy copies it itself
# (vercel.json's buildCommand), as scripts/dev-relay.mjs does when it starts.
#   viewer/scripts/sync-office.sh [--check]   (--check: exit 1 if a local copy is out of date)
set -eu
here=$(cd "$(dirname "$0")" && pwd)
from="$here/../../Resources/web/office.js"
to="$here/../public/office.js"
if [ "${1:-}" = "--check" ]; then
  if [ ! -e "$to" ]; then echo "no local copy of office.js: the viewer takes Resources/web/office.js when it's deployed"; exit 0; fi
  if cmp -s "$from" "$to"; then echo "viewer/public/office.js is in sync"; exit 0; fi
  echo "viewer/public/office.js is out of date: run viewer/scripts/sync-office.sh" >&2; exit 1
fi
cp "$from" "$to"
echo "copied Resources/web/office.js -> viewer/public/office.js ($(wc -c < "$to" | tr -d ' ') bytes, git-ignored)"
