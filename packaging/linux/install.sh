#!/bin/sh
# Installs Smolcorp for you alone:
#
#   ~/.local/bin/smolcorp                         the app and its commands
#   ~/.local/lib/smolcorp/tmux                    the tmux it drives your desks with
#   ~/.local/share/applications/smolcorp.desktop  its entry in your app menu
#
# and, the one step that asks for your password (sudo), once, the sandbox your
# hires start in:
#
#   /usr/local/lib/smolcorp/bwrap                 Smolcorp's copy of your bubblewrap, which only
#                                                 root and your account's group can run
#   /etc/apparmor.d/smolcorp-bwrap                where AppArmor needs it: that copy may make user
#                                                 namespaces, and what it runs gets no capabilities
#                                                 and can make none of its own; a program of yours
#                                                 that loads its own code into the copy (LD_PRELOAD)
#                                                 can use the copy's capabilities in them
#
#   ./install.sh
#
# Running it again updates an existing install; your paired phones and
# settings are kept. A copy of yours that is running is stopped first, by the
# pid it recorded (smolcorp quit); agents you started with `smolcorp run`
# keep running either way. An install from
# before it was Smolcorp is taken out, so the app menu shows it once; its
# command stays as a link to the new one, as your agents' hooks and the login
# item still name it until Smolcorp first runs and moves them over.

set -eu
cd "$(dirname "$0")"
BIN="$HOME/.local/bin"
DATA="${XDG_DATA_HOME:-$HOME/.local/share}"

# The old copy would carry on, on the program this replaces. Its own quit stops
# it by its recorded pid, and nothing else; one from before quit existed can't.
if [ -x "$BIN/smolcorp" ]; then
  "$BIN/smolcorp" quit 2>/dev/null || echo "The old Smolcorp wasn't stopped: if it still runs, quit it from its tray menu (with no tray, press Ctrl+C in its terminal) and open it again after this."
fi

rm -rf "$DATA/sauzzedeck-pro" "$DATA/applications/sauzzedeck-pro.desktop"
mkdir -p "$BIN" "$DATA/applications" "$DATA/smolcorp"
install -m 755 smolcorp "$BIN/smolcorp"
if [ -e "$BIN/sauzzedeck" ] || [ -L "$BIN/sauzzedeck" ]; then
  ln -sf smolcorp "$BIN/sauzzedeck"
fi
install -m 644 smolcorp.png "$DATA/smolcorp/icon.png"

# Smolcorp's own tmux, with its guard: Smolcorp looks here before any other
# tmux. Put in place by a rename, so a desk's tmux still running keeps its
# file. Tried once first: on a computer whose system is older than it needs
# (glibc 2.35, as in Ubuntu 22.04), it's taken out again, and Smolcorp uses
# your own tmux, as before.
LIB="$HOME/.local/lib/smolcorp"
mkdir -p "$LIB"
chmod 755 "$LIB"
install -m 755 tmux "$LIB/tmux.new"
mv -f "$LIB/tmux.new" "$LIB/tmux"
install -m 644 THIRD-PARTY-NOTICES-tmux.txt "$LIB/THIRD-PARTY-NOTICES-tmux.txt"
if ! "$LIB/tmux" -V 2>/dev/null | grep -q '^tmux '; then
  rm -f "$LIB/tmux"
  echo "Smolcorp's own tmux doesn't run on this computer (most likely its glibc is older than 2.35), so Smolcorp uses your own tmux instead."
fi
cat > "$DATA/applications/smolcorp.desktop" <<ENTRY
[Desktop Entry]
Type=Application
Name=Smolcorp
Comment=A General Manager for your AI coding agents, in a pixel-art office
Exec="$BIN/smolcorp"
Icon=$DATA/smolcorp/icon.png
Categories=Development;
ENTRY

echo "Installed as $BIN/smolcorp, and in your app menu."
case ":$PATH:" in
  *":$BIN:"*) ;;
  *)
    # Only printed: nothing in your shell files is changed for you. RC is
    # shown as written (the ~ is for the reader, not expanded here).
    case "$(basename "${SHELL:-}")" in
      zsh) RC="~/.zshrc" ;;
      bash) RC="~/.bashrc" ;;
      *) RC="" ;;
    esac
    echo "$BIN isn't on your PATH yet, so Terminal won't find smolcorp (to start agents with: smolcorp run claude)."
    if [ -n "$RC" ]; then
      echo "Run this, then open a new Terminal window:"
      echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> $RC"
    else
      echo "Add this line to your shell's startup file, then open a new Terminal window:"
      echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    fi ;;
esac

# Hires start in a sandbox made by Smolcorp's own copy of bubblewrap: the one
# step that needs root, done once, with your password. Smolcorp's own check
# says whether it's needed and why, with exactly what it grants (who can run
# the copy, that what the copy runs gets no capabilities and can make no user
# namespaces, and that a program of yours can load its own code into the
# copy, with LD_PRELOAD, and use the copy's own); the command it runs is
# Smolcorp's own (smolcorp sandbox command), the same one Setup shows, printed
# before it runs. Declined or failed, hires don't start until it's done;
# nothing else here needs it.
echo
SANDBOX_OK=0
WHY=$("$BIN/smolcorp" sandbox check 2>/dev/null) || SANDBOX_OK=$?
if [ "$SANDBOX_OK" -eq 3 ]; then
  echo "$WHY Then run ./install.sh again."
elif [ "$SANDBOX_OK" -ne 0 ]; then
  SANDBOX=$("$BIN/smolcorp" sandbox command)
  echo "$WHY"
  echo "The installer runs it for you now."
  if ! sh -c "$SANDBOX" || ! "$BIN/smolcorp" sandbox check >/dev/null; then
    echo "It didn't finish, so hires won't start until it does: run the line above in a terminal, or see Setup."
  fi
fi

# Started from where it now lives, it connects your agents and says what's
# left: pairing your phone, Tailscale. Run it again any time: smolcorp setup
# With no desktop to open a browser on (over SSH), --open also says where the
# office is and how to get a setup link.
echo
if [ -z "${DISPLAY:-}" ] && [ -z "${WAYLAND_DISPLAY:-}" ]; then
  "$BIN/smolcorp" setup --open || true
else
  "$BIN/smolcorp" setup || true
fi
