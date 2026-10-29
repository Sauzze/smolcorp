# Smolcorp 1.22.1 (beta)

## Fixed in 1.22.1

- **Tailscale is found on the Mac again.** Setup said "Get Tailscale" while
  Tailscale was running, so the phone's address through Tailscale, and the
  phone mic over HTTPS, weren't offered. Smolcorp now asks Tailscale's Mac app
  the way a terminal does, and gets its answer.

## New in 1.22

Smolcorp is one app, open source, on the Mac, Windows and Linux: your coding
agents as little people in a pixel office, on your computer and on your
phone, with roles to hire, a General Manager, and every art pack.

## Set up without doing it yourself

- **Start here.** Setup opens on a checklist your computer runs: installed,
  your agents reporting, how your phone gets here, Windows Firewall, a phone
  paired, open at login. Each line has the button that puts it right, and
  **Fix everything** does them all.
- **Or let your coding agent do it.** Download `FIRST-SETUP.md` from this
  release (the Windows and Linux downloads have it inside too): paste it into
  your coding agent (Claude Code,
  Codex, Gemini CLI, OpenCode or Grok), opened in the folder you downloaded to,
  and it installs, connects, checks and tells you what only you can do.
- **`smolcorp setup`** in a terminal (`smolcorp-cli setup` on Windows,
  `Smolcorp.app/Contents/MacOS/AgentDeck --setup` on a Mac) does the same, prints
  the checklist and ends with 0 when nothing is in the way.
- **Hire your General Manager** in one click from Start here: it opens in its
  own terminal and asks you what the goal is.

## Windows

**Alpha, still in testing.** The Mac app is the beta; the Windows build is
an early one, for trying out and reporting back.

- **Installs itself.** Open `Smolcorp.exe` from the zip: it moves into
  `%LOCALAPPDATA%\Smolcorp`, puts `smolcorp-cli` on your PATH, and starts from
  there, so nothing ever points at your Downloads folder again.
- **The office in its own window** (WebView2), with **Always on Top**, from the
  tray's **Show Office**. Without WebView2 it opens in your browser, with no
  code to type.
- **One Smolcorp at a time**: opening it again brings the office forward.
- **Only your devices can reach it.** Smolcorp listens on this PC and your
  Tailscale address only. Windows Firewall's rules are set with one OK from
  you, and until then Tailscale's address isn't opened, so Windows never makes
  rules that let anyone in. **Also on my home Wi-Fi** lets your home network
  in too, on private networks only.
- **Hire from the office**, as Anyone or by role, in Windows Terminal, with the
  roles, badges and outfits as the Mac. Sessions come back to
  their desks after a restart.
- The portable office (beta), the same as the Mac's.

## Linux

**Alpha, still in testing**, as on Windows.

- `install.sh` installs for you alone and runs `smolcorp setup`.
- Hiring from the office, in a terminal the setup page lists or tmux.
- **Show Office** opens the office in your browser, already paired.

## Everywhere

- **Six floors**, for when you run dozens of agents: twelve desks to a floor,
  filled in order by cloud bots, runs without a terminal, then anyone with no
  desk free downstairs. The elevator's call button opens a list of the floors
  with how many are on each; pick one to ride there. In setup, each floor gets a
  name and a style (the office's own, the server room, or any style). The
  iPhone app needs a rebuild to show more than two floors; phones in the
  browser and the portable office have it now.
- **Grok Bots in the office.** Cloud bots get desks of their own, upstairs or on
  the ground floor, green when they've reported, with a card of their last
  action. On the Mac, a Grok Bot is followed by its id in the Grok Bot app, and
  its desk shows its current name. On Windows and Linux they report through
  `POST /bots/<id>/event`.
- **Summaries with Claude**, with your own Anthropic API key, off until you turn
  it on, on all three systems. Dictation clean-up (Mac) uses the same key, and
  is off until you turn it on too.
- **Every art pack**, branded ones included, and the round-3 packs.
- **The iPhone app's Mac side** (conversation, info, notifications) is in the
  source. The iPhone app is build-it-yourself for now: see `ios/project.yml`.

## Updating

- **Mac:** drag the new Smolcorp into Applications. If this release is signed
  with a Developer ID and notarised (the release page says), macOS opens it
  with no warning; otherwise System Settings → Privacy & Security → **Open
  Anyway**. Either way, its signature is new, so macOS asks once more for
  **Accessibility** (and, the first time a key is pressed, for Terminal):
  turn Smolcorp back on there.
- **Windows:** quit Smolcorp from its tray icon, then open the new
  `Smolcorp.exe`. Then on Start here, **Set the rules** once for the new
  firewall rules. An older "Smolcorp (fenêtre)" window device is still
  recognised.
- **Linux:** quit Smolcorp, then run the new `install.sh`.
- Settings this version doesn't read are kept, on every system.

## Known limits

- The Mac app is signed with a Developer ID and notarised by Apple, so it opens
  with no warning. Windows builds aren't signed yet: SmartScreen warns you the
  first time.
- The Windows build is cross-compiled and its tests compile, but this release
  hasn't been run on a Windows PC by us yet: the firewall prompt, the install in
  place and the window are tested in code, not on a real PC. Please report how
  it goes in the issues.
- Grok Bot's app source is Mac-only.

## Downloads

Each file's SHA-256 is beside it on the release page.

- **Mac (beta):** **Smolcorp.dmg**: open it and drag Smolcorp to Applications.
  **Smolcorp.zip** is the same app. Signed with a Developer ID and notarised.
- **Windows (alpha, in testing):** **Smolcorp-Windows-x86_64.zip**, or
  **Smolcorp-Windows-arm64.zip** for an ARM PC. Extract it and open
  `Smolcorp.exe`: it isn't signed yet, so SmartScreen warns you first (More
  info, then Run anyway), then it installs itself.
- **Linux (alpha, in testing):** **Smolcorp-Linux-x86_64.tar.gz**, or
  **Smolcorp-Linux-arm64.tar.gz**. Extract it and run `./install.sh`.

`FIRST-SETUP.md` is attached on its own too, for any system, and inside the
Windows and Linux downloads: it hands the setup to your coding agent (Claude
Code, Codex, Gemini CLI, OpenCode or Grok).
