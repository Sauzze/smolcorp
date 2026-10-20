#!/bin/sh
# Copies the office's drawing code into the viewer, unchanged.
# Run after any change to Resources/web/office.js, and commit both.
#   viewer/scripts/sync-office.sh [--check]   (--check: exit 1 if the copy is out of date)
set -eu
here=$(cd "$(dirname "$0")" && pwd)
from="$here/../../Resources/web/office.js"
to="$here/../public/office.js"
if [ "${1:-}" = "--check" ]; then
  if cmp -s "$from" "$to"; then echo "viewer/public/office.js is in sync"; exit 0; fi
  echo "viewer/public/office.js is out of date: run viewer/scripts/sync-office.sh" >&2; exit 1
fi
cp "$from" "$to"
echo "copied Resources/web/office.js -> viewer/public/office.js ($(wc -c < "$to" | tr -d ' ') bytes)"
