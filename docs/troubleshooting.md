# Troubleshooting and uninstall

Back to the [README](../README.md).

## Troubleshooting

**The desks stay empty.** Click **Connect** on the setup page, then restart your
agent sessions. On Windows and Linux, start them with `smolcorp run`, or in
tmux, WezTerm or kitty.

**A desk has no controls.** Its session runs in a terminal Smolcorp can only
watch (see [Terminals](team.md#terminals)), or in Ghostty 1.3. In kitty, check the
two lines in kitty.conf.

**Codex sessions don't appear.** Run `/hooks` in Codex and trust Smolcorp's
hook. Codex skips new hooks until you do.

**OpenCode sessions don't appear.** Smolcorp before 1.8.0 had a plugin OpenCode 2
can't load: open the new Smolcorp once, and it brings the plugin up to date,
which OpenCode picks up by itself. Still nothing? Two OpenCode sessions in one
project folder can't be told apart, so keep one per folder, or start them with
`opencode --standalone`, which runs the plugin in the session's own tab.

**Buttons, the mic or scrolling do nothing, or the phone says keyboard access
is off.** Turn Smolcorp on in System
Settings → Privacy & Security → Accessibility. Updating Smolcorp clears
that permission, because macOS treats each version as a new app. Click
**Allow** on the setup page again afterwards, and turn Smolcorp
back on.

**Two fingers don't scroll.** Turn on **Let two fingers scroll Terminal** on
the setup page.

**The phone shows the pairing page instead of the deck.** It was revoked, or
it was paired with AgentDeck 1.0. Pair it again, then add it to your Home
Screen again: see [Pair your phone](phone.md#pair-your-phone).

**The mic types a space instead of listening.** Claude's input box has to be
empty, and voice has to be in tap mode: see [Voice](phone.md#voice).

**The phone can't connect.** Check that both devices are on the same Wi-Fi.
If macOS or Windows Firewall asked whether Smolcorp may accept incoming
connections, allow it. On Windows the server answers only on this PC and on
Tailscale's address, so the phone must be on Tailscale too: see
[Who can reach Smolcorp](windows-linux.md#who-can-reach-smolcorp-windows-and-linux).

**The roles, or HIRE, don't come out.** Turn on **Let the office hire and fire**
on the setup page, and choose a folder for new hires. They show only on a free
desk, or a pinned project's. With no roles
set up, it's a single **HIRE**. Hiring works on the Mac, on Windows (a new
Windows Terminal tab) and on Linux, where a hire needs a terminal the setup
page lists, or tmux.

**The screen saver says to open Smolcorp, or to pair again.** Make sure
Smolcorp is running. If it was unpaired, open System Settings → Screen
Saver, click **Options…** and enter a new code. After updating Smolcorp,
choose **Install Screen Saver…** again.

**"Port 7710 is in use."** Another copy of Smolcorp, or another app, is
using that port. Quit it and open Smolcorp again.

## Uninstall

1. On the setup page, click **Disconnect** next to your coding agents, and turn
   off **Open at login**.
2. Quit Smolcorp from the menu bar.
3. Delete **Smolcorp** from Applications, and the folder
   `~/Library/Application Support/AgentDeck`. It kept the app's original name.
4. If you installed the screen saver, choose another one in System Settings →
   Screen Saver, then delete **Smolcorp Office.saver** from
   `~/Library/Screen Savers`, and the files starting with
   `io.github.sauzze.agentdeck.screensaver` in
   `~/Library/Containers/com.apple.ScreenSaver.Engine.legacyScreenSaver/Data/Library/Preferences/ByHost`.

On Windows and Linux, disconnect and turn off **Open at login** the same way,
quit Smolcorp (from its tray menu; on Linux also `smolcorp quit`, or Ctrl+C in
its terminal if your desktop has no tray), then delete:

- **Linux:** `~/.local/bin/smolcorp` (and `~/.local/bin/sauzzedeck`, if you
  updated from SauzzeDeck Pro), `~/.local/share/applications/smolcorp.desktop`,
  and the folders
  `~/.local/share/smolcorp` and `~/.config/sauzzedeck-pro`. The settings folder
  kept the name the app had before.
- **Windows:** `%LOCALAPPDATA%\Smolcorp`, where it installed itself (and the
  folder you extracted, if it's still there), and `%APPDATA%\SauzzeDeck Pro`,
  which kept the name the app had before. Then take
  `%LOCALAPPDATA%\Smolcorp` out of your PATH (Settings → System → About →
  Advanced system settings → Environment Variables), and, in **Windows
  Defender Firewall with Advanced Security**, delete the inbound rules in the
  group **Smolcorp**.

Connecting left two backups of each settings file it changed next to it:
Claude Code's `~/.claude/settings.json`, Codex's `~/.codex/hooks.json` and
Gemini CLI's `~/.gemini/settings.json`. The one ending in `.agentdeck.bak` is
the file as it was before Smolcorp first changed it, and is never replaced;
the one ending in `.agentdeck-last.bak` is the file as it was before the latest
change. Delete them once you no longer need them. Connecting Claude Code also turned on its voice
input, if it was off. Disconnecting leaves voice as it is: remove the `voice`
entry from `~/.claude/settings.json` if you don't want it.
