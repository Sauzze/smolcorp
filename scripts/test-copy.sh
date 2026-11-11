#!/bin/bash
# Builds a test copy of Smolcorp and opens it beside the one in use:
#
#   scripts/test-copy.sh [--show] [port]     7790 if not given
#
# It opens hidden, in the background (open -g -j): it never comes in front of
# what you're doing or takes the keyboard, and puts up no window or alert by
# itself (see TestCopy.mayComeForward); everything goes through its HTTP
# routes. --show opens it as an app normally opens, for trying it by hand. It
# says its pid as it starts; quit it with kill <pid>.
#
# It listens on this Mac only (127.0.0.1 and ::1), so nothing on the network
# reaches it and the Mac's firewall never asks about it, on either engine. For
# the phone matrix only, SMOLCORP_TEST_LISTEN=lan lets it onto the local network:
#
#   SMOLCORP_TEST_LISTEN=lan scripts/test-copy.sh 7791
#
# The copy is "Smolcorp Test" (io.github.sauzze.agentdeck.test), so macOS
# keeps its permissions apart from the real app's. It listens on its own
# port, keeps its settings and keys in build/test-support, and changes
# nothing outside it: no hooks in your agents' settings, no login item (see
# TestCopy in Sources/AgentDeck/Paths.swift).
#
# Every hire in it, the General Manager's too, names a Claude account of its own:
# the copy refuses the default one, whose trust would go in your real ~/.claude.json.
#
# Its secrets are files in that folder (mode 0600, the folder 0700), never
# the login Keychain, which would ask you about every rebuilt copy:
#   test-gm-token   the General Manager's token, made by the copy itself
#   test-api-key    the ✨ Summary API key. Pasting one under Summaries writes
#                   it; to give a fake one by hand:
#                     printf '%s\n' "sk-""ant-fake-$(printf 'T3st%.0s' 1 2 3 4 5 6)" > build/test-support/test-api-key
#                     chmod 600 build/test-support/test-api-key
#                   A running copy sees the file at once (no restart), and a
#                   removed file counts as no key. A hire on the API-key
#                   account reads that file too.
#   test-viewer-key-id, -key, -write-secret
#                   the portable office's id, key and write secret
#   (the push key is never read in a test copy: it has none)
#
# For ✨ Summary without a key or a bill, point it at a stand-in for
# Anthropic's API on this computer, which only a test copy ever uses:
#
#   SMOLCORP_TEST_ANTHROPIC_URL=http://127.0.0.1:7781/v1/messages scripts/test-copy.sh
#
# To run it while Terminal.app is open without asking Terminal anything (no
# Automation prompt, no wait on one), for desks in tmux only:
#
#   SMOLCORP_TEST_NO_TERMINAL_APP=1 scripts/test-copy.sh
#
# Sessions report to it only when started with its own hook, and without your
# own settings, whose hooks report to the Smolcorp you use:
#
#   claude --setting-sources project,local --settings build/test-support/claude-settings.json
#
# Quit it by its pid, or from its menu bar icon. Running this again replaces
# the copy on the same port, and only that one.

set -euo pipefail

cd "$(dirname "$0")/.."
# Hidden and in the background unless --show asks for it as an app opens.
HIDDEN=(-g -j)
if [[ "${1:-}" == "--show" ]]; then
  HIDDEN=()
  shift
fi
PORT="${1:-7790}"
# A number only, read as decimal: 07710 or +7710 would otherwise slip past the check below.
[[ "$PORT" =~ ^[0-9]+$ ]] || { echo "The port must be a number."; exit 1; }
PORT=$((10#$PORT))
# 7710 is the real Smolcorp, 7720 and 7730 are other copies in use.
case "$PORT" in
  7710|7720|7730)
    echo "$PORT is a port Smolcorp uses (7710, 7720, 7730). Give the test copy another."
    exit 1 ;;
esac
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
chmod 700 "$SUPPORT"
sed "s/{{PORT}}/$PORT/" Resources/report.sh > "$SUPPORT/report.sh"
chmod 755 "$SUPPORT/report.sh"
# Claude Code's hooks for a session that reports here, given with --settings.
python3 - "$SUPPORT" <<'PY'
import json, sys
support = sys.argv[1]
events = ["SessionStart", "UserPromptSubmit", "PreToolUse", "PermissionRequest", "PostToolUse", "PostToolUseFailure",
          "Notification", "Stop", "StopFailure", "SessionEnd", "SubagentStart", "SubagentStop", "PreCompact", "PostCompact", "PermissionDenied"]
handler = {"type": "command", "command": f'"{support}/report.sh"', "async": True}
settings = {"hooks": {event: [{"hooks": [handler]}] for event in events}}
json.dump(settings, open(f"{support}/claude-settings.json", "w"), indent=2)
PY

# Only the test copy on this port: another session's copy, on its own port,
# is left alone. A copy on the Go engine ("engine": "go") listens through its
# core, bin/smolcorp core; the copy is the core's parent, and its core ends
# with it.
for pid in $(lsof -t -nP -iTCP:"$PORT" -sTCP:LISTEN 2>/dev/null); do
  case "$(ps -o command= -p "$pid")" in
    *"Smolcorp Test.app/Contents/MacOS/"*) kill "$pid" ;;
    *"Smolcorp Test.app/Contents/Resources/bin/smolcorp core "*)
      parent="$(ps -o ppid= -p "$pid" | tr -d ' ')"
      case "$(ps -o command= -p "$parent")" in
        *"Smolcorp Test.app/Contents/MacOS/"*) kill "$parent" ;;
      esac ;;
  esac
done
# The port free before the new copy takes it: a core takes a moment to end.
for _ in $(seq 50); do
  lsof -t -nP -iTCP:"$PORT" -sTCP:LISTEN >/dev/null 2>&1 || break
  sleep 0.1
done
open -n ${HIDDEN[@]+"${HIDDEN[@]}"} --env "SMOLCORP_TEST_PORT=$PORT" --env "SMOLCORP_TEST_SUPPORT_DIR=$SUPPORT" \
  ${SMOLCORP_TEST_ANTHROPIC_URL:+--env "SMOLCORP_TEST_ANTHROPIC_URL=$SMOLCORP_TEST_ANTHROPIC_URL"} \
  ${SMOLCORP_TEST_NO_TERMINAL_APP:+--env "SMOLCORP_TEST_NO_TERMINAL_APP=$SMOLCORP_TEST_NO_TERMINAL_APP"} \
  ${SMOLCORP_TEST_LISTEN:+--env "SMOLCORP_TEST_LISTEN=$SMOLCORP_TEST_LISTEN"} "$APP"

# Its pid, once it listens: the app's own, or on the Go engine its core's parent, the app.
COPY=""
for _ in $(seq 200); do
  for pid in $(lsof -t -nP -iTCP:"$PORT" -sTCP:LISTEN 2>/dev/null); do
    case "$(ps -o command= -p "$pid")" in
      *"Smolcorp Test.app/Contents/MacOS/"*) COPY="$pid" ;;
      *"Smolcorp Test.app/Contents/Resources/bin/smolcorp core "*) COPY="$(ps -o ppid= -p "$pid" | tr -d ' ')" ;;
    esac
  done
  [[ -n "$COPY" ]] && break
  sleep 0.1
done
WHERE="this Mac only"
# Read as both engines read it (TestCopy.wantsLAN): spaces and line breaks around it, and the letter case, don't
# matter; one inside it does.
[[ "$(printf %s "${SMOLCORP_TEST_LISTEN:-}" | tr "[:upper:]" "[:lower:]" | tr '[:space:]' ' ' | sed -e 's/^ *//' -e 's/ *$//')" == "lan" ]] && WHERE="the local network too"
if [[ -n "$COPY" ]]; then
  echo "Smolcorp Test is running on port $PORT ($WHERE), pid $COPY, with its settings in $SUPPORT."
  echo "Quit it with: kill $COPY"
else
  echo "Smolcorp Test is opening on port $PORT, with its settings in $SUPPORT, but isn't listening yet."
  echo "Its pid, once it is: lsof -t -nP -iTCP:$PORT -sTCP:LISTEN"
fi
echo "A Claude session that reports to it: claude --setting-sources project,local --settings $SUPPORT/claude-settings.json"
