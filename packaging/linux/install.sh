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
# before it was Smolcorp is taken out, so the app menu shows it once.

set -eu
cd "$(dirname "$0")"
BIN="$HOME/.local/bin"
DATA="${XDG_DATA_HOME:-$HOME/.local/share}"

rm -rf "$BIN/sauzzedeck" "$DATA/sauzzedeck-pro" "$DATA/applications/sauzzedeck-pro.desktop"
mkdir -p "$BIN" "$DATA/applications" "$DATA/smolcorp"
install -m 755 smolcorp "$BIN/smolcorp"
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

echo "Installed. Open Smolcorp from your app menu, or run: smolcorp"
case ":$PATH:" in
  *":$BIN:"*) ;;
  *) echo "Add $BIN to your PATH to start agents with: smolcorp run claude" ;;
esac
