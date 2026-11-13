# How Smolcorp works

Back to the [README](../README.md).

## How it works

Each agent runs a small script ([`Resources/report.sh`](../Resources/report.sh))
at each step of a session, through its hooks: Claude Code's
[hooks](https://code.claude.com/docs/en/hooks), and the matching settings in
Codex, Gemini CLI and Grok. OpenCode has no command hooks, so a small plugin
([`Resources/opencode-plugin.js`](../Resources/opencode-plugin.js)) runs the same
script. The script tells Smolcorp what happened and which terminal tab it
happened in: a Terminal tab, an iTerm2 session, a Ghostty window, or a tmux,
WezTerm or kitty pane. Smolcorp reads each agent's events in that agent's terms,
works out each session's state and streams it to your phone. On Windows and
Linux, `smolcorp hook` does the script's job, and names the launcher, tmux
pane or terminal window instead of a Terminal tab.

On the Mac, each terminal is driven its own way: Terminal, iTerm2 and
Ghostty through their scripting, tmux, WezTerm and kitty through their own
command line tools. In Terminal:

- **Typed commands** such as `/effort` and `/clear` are typed into the tab
  through Terminal's scripting.
- **Keys** are sent to Terminal itself, after its tab is put first among
  Terminal's windows without bringing Terminal forward. **Scrolling** uses
  Terminal's own View → Line Up and Line Down, so it moves the window's view
  and the agent receives nothing; it brings the window forward, and doesn't
  work while the Mac is locked.
- **Hiring** opens a new window of the terminal you chose, in the
  folder you chose, with the agent's own command. A role's hire runs a short
  script that removes itself, then starts the role's agent with its model,
  effort and first message, each quoted for the shell. A role's Claude Code
  hire runs in a tmux session of its own instead, when tmux is installed.
  **Firing** a hire in tmux sends `/exit` to it in its own session. In a
  Terminal tab, Smolcorp stops the agent's process only once it's sure that's
  the agent, then closes the tab.

One app, in Go, runs the office on every computer, with a handful of
dependencies: a tray icon, pseudo-terminals, QR codes. On a Mac it runs inside
a plain Swift app, with no dependencies, that is its menu bar item and windows
and does what only macOS can: the keys, Terminal, iTerm2 and Ghostty, the
Keychain, dictation. The phone side is a single web page, shared by all.

```
Sources/AgentDeck/   the Mac app: the menu bar, the windows and what only macOS does, around the Go app
ScreenSaver/         the Mac's screen saver, which shows the office
desktop/             the app itself, on every computer
cmd/smolcorp/        its command: the app, the launcher and the hook
Resources/web/       the phone page, the office, the pairing page and the setup page, shared
Resources/           the Mac's hook script, and the OpenCode plugin
install.sh           build and install on a Mac
packaging/linux/     the Linux installer
scripts/             the builds, and the release downloads
docs/                these guides, and the screenshots and reel in them
```
