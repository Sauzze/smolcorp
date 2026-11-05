# Set up Smolcorp with your coding agent

Paste everything below the line into your coding agent (Claude Code, Codex,
Gemini CLI, OpenCode or Grok), opened in the folder where you unzipped or
downloaded Smolcorp. It installs Smolcorp, connects your agents, checks everything,
and tells you the two or three things only you can do.

---

You're setting up Smolcorp on this computer for me. Smolcorp shows my coding
agents as little people in a pixel office, on this computer and on my
phone. Do the whole setup, check it, and report. Work only with what's
described here.

**Rules**
- Don't read, print or copy any key: not Smolcorp's `setup-key` or
  `hook-key`, not a pairing code's link, not my `.env` files.
- Don't change firewall rules, PATH or agent settings by hand: Smolcorp's
  own setup does that, with my OK where Windows or macOS asks.
- Ask me before anything that needs my password, a permission dialog, or my
  phone, and wait for me.

**1. Find Smolcorp and run its setup.** Look in this folder.
- **Windows** (`Smolcorp.exe` and `smolcorp-cli.exe`): run
  `.\smolcorp-cli.exe setup`. The first run installs Smolcorp in
  `%LOCALAPPDATA%\Smolcorp` and starts it from there. Windows then asks once
  to change the firewall: tell me to choose **Yes**. If WebView2 is missing,
  Smolcorp says so; the office then opens in the browser, which works too.
- **Linux** (`smolcorp` and `install.sh`): run `./install.sh`, which
  installs to `~/.local/bin` and runs `smolcorp setup` itself.
- **Mac** (`Smolcorp.app`, or `Smolcorp.dmg`): if it's a DMG, tell me to
  open it and drag Smolcorp to Applications, then open it once from there.
  It's signed with a Developer ID and notarised by Apple: the first time,
  macOS asks whether to open an app downloaded from the internet; tell me
  to click **Open**. Then run `/Applications/Smolcorp.app/Contents/MacOS/AgentDeck --setup`. If its
  checklist says macOS is opening a temporary copy of an app that is in
  Applications, tell me to quit Smolcorp, run
  `xattr -d com.apple.quarantine /Applications/Smolcorp.app` myself in
  Terminal, and open it again; don't run it for me.
- On **Windows** and **Linux**, Smolcorp is an alpha, still in testing: say so
  if something doesn't work as described, and tell me what you saw.

**2. Read the checklist it prints.** `✓` is done, `✗` is in the way, `•` is
a step still to take, `i` is worth knowing. It ends with exit code 0 when
nothing is in the way.
- For each `✗` line, do what it says, then run the setup command again
  (with `--check` to only look). If a line says no coding agent is
  installed, tell me: installing one is my choice.
- If agents don't show up in the office, or a line says their hooks point
  at another copy of Smolcorp (an old download, say), use the checklist's
  **Connect them** fix, then tell me to restart any agent sessions that were
  already open: they only pick up the change when they start.
- Run only one Smolcorp: if an older copy is still running from a
  download folder, tell me to quit it before going on.
- For each `•` line, tell me what to do: pair my phone (scan the QR code in
  Smolcorp's setup page, then type the 6 digits), allow keyboard access on a
  Mac (System Settings → Privacy & Security → Accessibility), open at login.
- If it mentions Tailscale, explain it: a free private network between my
  own devices, so my phone reaches this computer from anywhere, not only on
  my Wi-Fi. Install it on this computer and on my phone, signed in to the
  same account. On Windows and Linux, Smolcorp only listens to my own
  Tailscale devices until I choose **Also on my home Wi-Fi** in setup.

**3. Open the setup page**, so I can pair my phone and, if I want, click
**Hire your General Manager**. No command makes a setup link, so don't run
`setup --open` for one. Tell me to choose **Open Setup…** from Smolcorp's tray
icon, or to use the link Smolcorp printed as it started. With no tray (a Linux
desktop with none), a new link comes only from stopping Smolcorp and starting
it again: run `smolcorp quit` (or press Ctrl+C in its terminal), then start
Smolcorp again.

**4. Report** in a few lines: what was installed and where, each checklist
line and whether it's done, and what's left for me. Then run the setup
command with `--check` once more and paste its last line.
