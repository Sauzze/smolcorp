#!/bin/bash
# Builds a test copy of Smolcorp and opens it beside the one in use:
#
#   scripts/test-copy.sh [port]     7790 if not given
#
# The copy is "Smolcorp Test" (io.github.sauzze.agentdeck.test), so macOS
# keeps its permissions apart from the real app's. It listens on its own
# port, keeps its settings and keys in build/test-support, and changes
# nothing outside it: no hooks in your agents' settings, no login item (see
# TestCopy in Sources/AgentDeck/Paths.swift).
#
# For ✨ Summary without a key or a bill, point it at a stand-in for
# Anthropic's API on this computer, which only a test copy ever uses:
#
#   SMOLCORP_TEST_ANTHROPIC_URL=http://127.0.0.1:7781/v1/messages scripts/test-copy.sh
#
# Sessions report to it only when started with its own hook, as:
#
#   claude --settings build/test-support/claude-settings.json
#
# Quit it from its menu bar icon. Running this again replaces the copy on
# the same port, and only that one.

set -euo pipefail

cd "$(dirname "$0")/.."
PORT="${1:-7790}"
if [ "$PORT" = 7710 ]; then
  echo "7710 is the real Smolcorp's port. Give the test copy another."
  exit 1
fi
APP="build/Smolcorp Test.app"
SUPPORT="$PWD/build/test-support"

scripts/build-app.sh
rm -rf "$APP"
cp -R build/Smolcorp.app "$APP"
/usr/libexec/PlistBuddy -c "Set :CFBundleIdentifier io.github.sauzze.agentdeck.test" \
  -c "Set :CFBundleName Smolcorp Test" "$APP/Contents/Info.plist"
codesign --force --sign - "$APP"

# Its hook: the real one, sending to this copy's port.
mkdir -p "$SUPPORT"
sed "s/{{PORT}}/$PORT/" Resources/report.sh > "$SUPPORT/report.sh"
chmod 755 "$SUPPORT/report.sh"
# Claude Code's hooks for a session that reports here, given with --settings.
python3 - "$SUPPORT" <<'PY'
import json, sys
support = sys.argv[1]
events = ["SessionStart", "UserPromptSubmit", "PreToolUse", "PermissionRequest", "PostToolUse", "PostToolUseFailure",
          "Notification", "Stop", "StopFailure", "SessionEnd", "SubagentStart", "SubagentStop", "PreCompact", "PostCompact"]
handler = {"type": "command", "command": f'"{support}/report.sh"', "async": True}
settings = {"hooks": {event: [{"hooks": [handler]}] for event in events}}
json.dump(settings, open(f"{support}/claude-settings.json", "w"), indent=2)
PY

# Only the test copy on this port: another session's copy, on its own port,
# is left alone.
for pid in $(lsof -t -nP -iTCP:"$PORT" -sTCP:LISTEN 2>/dev/null); do
  case "$(ps -o command= -p "$pid")" in
    *"Smolcorp Test.app/Contents/MacOS/"*) kill "$pid" ;;
  esac
done
open -n --env "SMOLCORP_TEST_PORT=$PORT" --env "SMOLCORP_TEST_SUPPORT_DIR=$SUPPORT" \
  ${SMOLCORP_TEST_ANTHROPIC_URL:+--env "SMOLCORP_TEST_ANTHROPIC_URL=$SMOLCORP_TEST_ANTHROPIC_URL"} "$APP"
echo "Smolcorp Test is opening on port $PORT, with its settings in $SUPPORT."
echo "A Claude session that reports to it: claude --settings $SUPPORT/claude-settings.json"
