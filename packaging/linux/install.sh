#!/bin/sh
# Installs Smolcorp for you alone, with no root needed:
#
#   ~/.local/bin/smolcorp                         the app and its commands
#   ~/.local/share/applications/smolcorp.desktop  its entry in your app menu
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
