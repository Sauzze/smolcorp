#!/bin/bash
# The demo's page, for real (FXX1d): `smolcorp demo --no-open`, not a dry run, against a hidden test copy of
# Smolcorp on its Go engine, with stand-in agents (scripts/input-matrix/fake-agent.mjs, nothing spent):
#
#   scripts/build-app.sh                 first, once (the copy is build/Smolcorp.app's)
#   scripts/demo-page-run.sh [port] [cdp]    7790 and 7791 if not given (7790-7794 only)
#
# It seeds build/dp (a fake home, the copy's own support folder, a test Claude account, a stand-in `claude`),
# opens "Smolcorp Test" HIDDEN and in the BACKGROUND (open -g -j: no window, never activated), runs the demo
# with --no-open, and drives the printed link in headless Chrome on 127.0.0.1 (demo-page-run.mjs): signed in
# with the copy's own one-time link, the demo's card filled in, Hire clicked; then the GM hired with its scope,
# the demo's terminal following it, a scope refusal through the routes with the GM's own token, the GM fired.
# Everything it started is quit by its PID: Chrome, the demo, the stand-ins, the copy. Nothing outside build/dp
# is written, and the real Smolcorp (7710) is never asked anything. Prints PASS or FAIL lines, exits 0 on all PASS.
set -euo pipefail
cd "$(dirname "$0")/.."
WT="$PWD"
PORT="${1:-7790}"
CDP="${2:-7791}"
for p in "$PORT" "$CDP"; do
  case "$p" in 7790|7791|7792|7793|7794) ;; *) echo "Ports 7790-7794 only."; exit 1 ;; esac
done
[ "$PORT" != "$CDP" ] || { echo "Two ports, please."; exit 1; }
for p in "$PORT" "$CDP"; do
  if lsof -t -nP -iTCP:"$p" -sTCP:LISTEN >/dev/null 2>&1; then echo "Port $p is taken."; exit 1; fi
done
[ -d build/Smolcorp.app ] || { echo "Build the app first: scripts/build-app.sh"; exit 1; }
TMUX_BIN="$(command -v tmux)" || { echo "The run needs tmux (the hire engine's)."; exit 1; }
NODE_BIN="$(command -v node)" || { echo "The run needs node."; exit 1; }

# A short path: the copy's bridge socket must fit in 103 bytes.
RUN="$WT/build/dp"
APP="$WT/build/Smolcorp Test.app"
FAKE_HOME="$RUN/home"
SUPPORT="$RUN/support"
rm -rf "$RUN"
mkdir -p "$FAKE_HOME" "$SUPPORT" "$RUN/bin" "$RUN/tmp" "$RUN/agents"
chmod 700 "$SUPPORT"

# The copy, as scripts/test-copy.sh makes it.
rm -rf "$APP"
cp -R build/Smolcorp.app "$APP"
/usr/libexec/PlistBuddy -c "Set :CFBundleIdentifier io.github.sauzze.agentdeck.test" -c "Set :CFBundleName Smolcorp Test" "$APP/Contents/Info.plist"
codesign --force --sign - "$APP" 2>/dev/null
CLI="$APP/Contents/Resources/bin/smolcorp"

# A test Claude account (a test copy never hires on the default one), and the copy's config: the Go engine, setup
# done (no Settings window), hires in the background (no terminal window), the manager and hiring off, as a first demo finds them.
mkdir -p "$FAKE_HOME/.claude-test"
echo '{}' >"$FAKE_HOME/.claude-test/settings.json"
echo '{}' >"$FAKE_HOME/.claude-test/.claude.json"
/usr/bin/python3 -I - "$SUPPORT/config.json" "$FAKE_HOME" <<'PY'
import json, sys
path, home = sys.argv[1], sys.argv[2]
json.dump({"engine": "go", "hasCompletedSetup": True, "guideDone": True, "choicesAsked": True, "hireInTmux": True, "hiresInBackground": True,
           "managerEnabled": False, "hiringEnabled": False, "hireFolder": home,
           "claudeProfiles": [{"name": "test", "dir": home + "/.claude-test"}]}, open(path, "w"), indent=2)
PY

# The stand-in `claude`: fake-agent.mjs at the desk, reporting through the copy's own hook as Claude Code (the
# `#` ends the line before the stand-in's own "codex"). The GM's token, which its start script gives it, is kept
# (0600, in build/dp) so the run can ask as the GM.
cat >"$RUN/bin/claude" <<EOF
#!/bin/bash
echo "\$(date +%T) \$PWD \$*" >>"$RUN/fake-claude.log"
if [ -n "\${SMOLCORP_GM_TOKEN:-}" ]; then (umask 077; printf '%s\n' "\$SMOLCORP_GM_TOKEN" >"$RUN/gm-token"); fi
exec -a claude "$NODE_BIN" "$WT/scripts/input-matrix/fake-agent.mjs" "$RUN/agents/\$\$.log" "'$CLI' hook --port $PORT claude #"
EOF
chmod 755 "$RUN/bin/claude"
ln -s "$TMUX_BIN" "$RUN/bin/tmux"
ln -s "$NODE_BIN" "$RUN/bin/node"
# The demo's command finds the setup key where the app keeps it in its home: the copy's own folder.
mkdir -p "$FAKE_HOME/Library/Application Support"
ln -s "$SUPPORT" "$FAKE_HOME/Library/Application Support/AgentDeck"
ENVS=(HOME="$FAKE_HOME" PATH="$RUN/bin:/usr/bin:/bin:/usr/sbin:/sbin" TMPDIR="$RUN/tmp/" LANG=en_US.UTF-8)

open -g -j -n --stdout "$RUN/app.log" --stderr "$RUN/app.log" \
  --env HOME="$FAKE_HOME" --env CFFIXED_USER_HOME="$FAKE_HOME" --env PATH="$RUN/bin:/usr/bin:/bin:/usr/sbin:/sbin" \
  --env TMPDIR="$RUN/tmp/" --env SHELL=/bin/zsh --env LANG=en_US.UTF-8 \
  --env SMOLCORP_TEST_PORT="$PORT" --env SMOLCORP_TEST_SUPPORT_DIR="$SUPPORT" --env SMOLCORP_TEST_NO_TERMINAL_APP=1 \
  "$APP"

# The copy's PID: the Go core's parent, only ever this worktree's copy.
APP_PID=""
for _ in $(seq 150); do
  for pid in $(lsof -t -nP -iTCP:"$PORT" -sTCP:LISTEN 2>/dev/null); do
    case "$(ps -o command= -p "$pid")" in
      "$APP/Contents/Resources/bin/smolcorp core "*) APP_PID="$(ps -o ppid= -p "$pid" | tr -d ' ')" ;;
    esac
  done
  [ -n "$APP_PID" ] && [ -s "$SUPPORT/setup-link" ] && break
  sleep 0.2
done
DEMO_PID=""
stop() {
  [ -n "$DEMO_PID" ] && kill "$DEMO_PID" 2>/dev/null || true
  # Stand-ins still running (none once the GM is fired): by their PID, found by build/dp in their command line.
  for pid in $(pgrep -f "$RUN/agents/" 2>/dev/null || true); do kill "$pid" 2>/dev/null || true; done
  if [ -n "$APP_PID" ]; then
    kill "$APP_PID" 2>/dev/null || true
    for _ in $(seq 50); do kill -0 "$APP_PID" 2>/dev/null || break; sleep 0.2; done
    kill -0 "$APP_PID" 2>/dev/null && kill -9 "$APP_PID" 2>/dev/null || true
    echo "copy $APP_PID quit"
  fi
  for pid in $(lsof -t -nP -iTCP:"$PORT" -sTCP:LISTEN 2>/dev/null); do
    case "$(ps -o command= -p "$pid")" in
      "$APP/Contents/Resources/bin/smolcorp core --port $PORT "*) echo "core $pid left: stopping it"; kill "$pid" ;;
    esac
  done
  # The copy's keys and the GM's token go; the logs stay for the reviewer.
  rm -f "$SUPPORT/setup-key" "$SUPPORT/hook-key" "$SUPPORT/setup-link" "$SUPPORT/window-key" "$SUPPORT/test-gm-token" "$RUN/gm-token"
}
trap stop EXIT
[ -n "$APP_PID" ] || { echo "FAIL the copy didn't come up (build/dp/app.log)"; exit 1; }
case "$(ps -o command= -p "$APP_PID")" in "$APP/Contents/MacOS/AgentDeck"*) ;; *) echo "PID $APP_PID isn't this worktree's copy"; APP_PID=""; exit 1 ;; esac
echo "copy pid $APP_PID on $PORT (Go engine), hidden"

# The demo itself, as you'd run it, but --no-open: its yes on its input, its words in build/dp/demo.out.
# exec: the PID kept is the demo's own, for Ctrl-C (SIGINT) and the cleanup.
(cd "$FAKE_HOME" && exec env "${ENVS[@]}" SMOLCORP_PORT="$PORT" "$CLI" demo --no-open --in "$FAKE_HOME" <<<y >"$RUN/demo.out" 2>&1) &
DEMO_PID=$!
LINK=""
for _ in $(seq 150); do
  LINK="$(grep -o "http://localhost:$PORT/setup?demo=[^[:space:]]*" "$RUN/demo.out" || true)"
  [ -n "$LINK" ] && break
  kill -0 "$DEMO_PID" 2>/dev/null || break
  sleep 0.2
done
[ -n "$LINK" ] || { echo "FAIL the demo printed no link:"; cat "$RUN/demo.out"; exit 1; }
echo "PASS the demo printed its page's link, with no token in it: $LINK"

set +e
env "${ENVS[@]}" SMOLCORP_PORT="$PORT" node "$WT/scripts/demo-page-run.mjs" "$RUN" "$PORT" "$CDP" "$LINK" "$CLI" "$DEMO_PID"
STATUS=$?
set -e
for _ in $(seq 50); do kill -0 "$DEMO_PID" 2>/dev/null || break; sleep 0.2; done
if kill -0 "$DEMO_PID" 2>/dev/null; then echo "FAIL the demo was still running after Ctrl-C"; STATUS=1; else DEMO_PID=""; fi
echo "--- the demo's terminal (build/dp/demo.out) ---"
cat "$RUN/demo.out"
exit "$STATUS"
