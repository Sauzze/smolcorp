#!/bin/sh
# Installs Smolcorp for you alone, with no root needed:
#
#   ~/.local/bin/smolcorp                         the app and its commands
#   ~/.local/share/applications/smolcorp.desktop  its entry in your app menu
#
#   ./install.sh
#
# Running it again updates an existing install; your paired phones and
# settings are kept. Quit Smolcorp from its tray menu first: agents you
# started with `smolcorp run` keep running either way. An install from
# before it was Smolcorp is taken out, so the app menu shows it once; its
# command stays as a link to the new one, as your agents' hooks and the login
# item still name it until Smolcorp first runs and moves them over.

set -eu
cd "$(dirname "$0")"
BIN="$HOME/.local/bin"
DATA="${XDG_DATA_HOME:-$HOME/.local/share}"

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
Comment=A control pad for your coding agents, on your phone
Exec="$BIN/smolcorp"
Icon=$DATA/smolcorp/icon.png
Categories=Development;
ENTRY

echo "Installed as $BIN/smolcorp, and in your app menu."
case ":$PATH:" in
  *":$BIN:"*) ;;
  *) echo "Add $BIN to your PATH to start agents with: smolcorp run claude" ;;
esac

# Started from where it now lives, it connects your agents and says what's
# left: pairing your phone, Tailscale. Run it again any time: smolcorp setup
echo
"$BIN/smolcorp" setup || true
