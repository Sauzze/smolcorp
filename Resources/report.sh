#!/bin/bash
# Smolcorp's hook, run by every coding agent it follows. Forwards each
# hook event to the app.
#
#   report.sh [agent]   claude (the default), codex, gemini or opencode
#
# The event JSON arrives on stdin and is passed on as it is. The hook's key
# comes from hook-header, which the app writes next to this script: curl
# reads it from the file, so it never appears on a command line, where any
# process on the Mac could see it with `ps`. Five more things are added as
# headers:
#   - the agent, so the app reads the event in that agent's terms.
#   - the time the event was raised. Hooks run in the background, so two events
#     a few milliseconds apart can arrive in either order.
#   - the agent's tty. Hooks have no terminal of their own, so this walks up
#     the process tree to the first ancestor that has one.
#   - the tab, as the app names tabs: tty:/dev/ttys003 for Terminal.app,
#     tmux:%3@<socket>, iterm:<session>, ghostty:<tty>, wezterm:, kitty:, or
#     other:<tty> for a terminal the app can only watch.
#   - the folder the agent was started in, where the agent says. The event's
#     cwd follows the agent into subfolders; the app names the session's
#     project from this instead.
#
# It never blocks or fails the session. If the app isn't running, the event
# is simply dropped.
#
# Sessions in every terminal report in. One the app can't drive yet gets a
# desk that shows what it's doing, without the controls.

# Some agents run hooks with a pared-down environment.
PATH=/usr/bin:/bin:/usr/sbin:/sbin

AGENT="${1:-claude}"
AT="$(perl -MTime::HiRes=time -e 'printf "%d", time * 1000')"
DIR="$(cd "$(dirname "$0")" && pwd)"

tty=""
tab=""
pid=$PPID
while [ -n "$pid" ] && [ "$pid" -gt 1 ]; do
  t="$(ps -o tty= -p "$pid" 2>/dev/null | tr -d ' ')"
  if [[ "$t" =~ ^tty[A-Za-z0-9]+$ ]]; then
    tty="/dev/$t"
    break
  fi
  pid="$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ')"
done

# Whether $1 is free of control characters, which would end a header line.
clean() { [[ "$1" != *[[:cntrl:]]* ]]; }

# A session in another terminal says so in TERM_PROGRAM. Where an agent has
# dropped that variable, Terminal itself is found further up the tree.
in_terminal_app() {
  [ "$TERM_PROGRAM" = "Apple_Terminal" ] && return 0
  [ -n "$TERM_PROGRAM" ] && return 1
  while [ -n "$pid" ] && [ "$pid" -gt 1 ]; do
    case "$(ps -o comm= -p "$pid" 2>/dev/null)" in
      */Terminal.app/*) return 0 ;;
    esac
    pid="$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ')"
  done
  return 1
}

# No tty at all means the hook runs outside the session's process tree, as
# Codex's do, and OpenCode 2's, in a background service of their own. The app
# finds the tab itself, by the session's folder. Any other agent without a tty
# is running headless, as claude -p does, and is left out: found by its folder,
# it would land on the tab of an interactive session there.
if [ -n "$tty" ]; then
  # tmux first: it runs inside any of the others, whose variables it then
  # carries from whichever started it, stale. As the Windows and Linux hook's
  # tabFromEnvironment. Each variable is taken only in its own shape: one
  # carrying anything else, as a line break that would add a header of its
  # own and point the session at another tab, isn't believed.
  if [ -n "$TMUX_PANE" ]; then
    socket="${TMUX%%,*}"
    # Without its socket, a pane number could be another server's pane.
    if [[ "$TMUX_PANE" =~ ^%[0-9]+$ ]] && [ -n "$socket" ] && clean "$socket"; then tab="tmux:$TMUX_PANE@$socket"; else tab="other:$tty"; fi
  elif [ -n "$WEZTERM_PANE" ]; then
    if [[ "$WEZTERM_PANE" =~ ^[0-9]+$ ]] && [ -n "$WEZTERM_UNIX_SOCKET" ] && clean "$WEZTERM_UNIX_SOCKET"; then tab="wezterm:$WEZTERM_PANE@$WEZTERM_UNIX_SOCKET"; else tab="other:$tty"; fi
  elif [ -n "$KITTY_WINDOW_ID" ] && [ -n "$KITTY_LISTEN_ON" ]; then
    if [[ "$KITTY_WINDOW_ID" =~ ^[0-9]+$ ]] && clean "$KITTY_LISTEN_ON"; then tab="kitty:$KITTY_WINDOW_ID@$KITTY_LISTEN_ON"; else tab="other:$tty"; fi
  elif [ -n "$ITERM_SESSION_ID" ]; then
    session="${ITERM_SESSION_ID#*:}" # w0t1p0:<the session's unique ID>
    if [[ "$session" =~ ^[0-9A-Fa-f-]+$ ]]; then tab="iterm:$session"; else tab="other:$tty"; fi
  elif [ "$TERM_PROGRAM" = "ghostty" ]; then
    tab="ghostty:$tty"
  elif in_terminal_app; then
    tab="tty:$tty"
  else
    tab="other:$tty"
  fi
else
  case "$AGENT" in
    codex|opencode) ;;
    *) exit 0 ;;
  esac
fi

# The app takes up to 1 MiB. A bigger event, such as a tool's whole output,
# loses that output, which the app never shows, and has each text longer than
# 4096 characters shortened, rather than being lost: what the office shows,
# as the file a tool reads or the command it runs, is in the first of it.
# Smaller events pass as they are. As the Windows and Linux hook's trimmedEvent.
TRIM='
  local $/; my $event = <STDIN>;
  if (length $event > 900 * 1024) {
    require JSON::PP;
    my $json = JSON::PP->new->utf8;
    my $fields = eval { $json->decode($event) };
    if (ref $fields eq "HASH") {
      delete $fields->{tool_response};
      my $shorten; $shorten = sub {
        my $v = shift;
        if (ref $v eq "HASH") { $v->{$_} = $shorten->($v->{$_}) for keys %$v; return $v }
        if (ref $v eq "ARRAY") { return [map { $shorten->($_) } @$v] }
        return (defined $v && !ref $v && length $v > 4096 && !JSON::PP::is_bool($v)) ? substr($v, 0, 4096) . "..." : $v;
      };
      $event = $json->encode($shorten->($fields));
    }
  }
  print $event;
'

# The folder the agent was started in, under the same rule as the tab.
project="${CLAUDE_PROJECT_DIR:-$GEMINI_PROJECT_DIR}"
clean "$project" || project=""

perl -e "$TRIM" | curl -s -m 2 -o /dev/null \
  -H 'Content-Type: application/json' \
  -H @"$DIR/hook-header" \
  -H "X-AgentDeck-Agent: $AGENT" \
  -H "X-AgentDeck-TTY: $tty" \
  -H "X-AgentDeck-Tab: $tab" \
  -H "X-AgentDeck-Project: $project" \
  -H "X-AgentDeck-At: $AT" \
  --data-binary @- \
  "http://127.0.0.1:{{PORT}}/hook"

exit 0
