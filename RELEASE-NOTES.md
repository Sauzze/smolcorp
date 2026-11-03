# Smolcorp 1.24.0 (beta)

If you're coming from 1.22.1, this covers everything since: 1.23 was never
released, so its changes are here too.

## A manager for your team (opt-in; the team itself is the Mac's)

The manager is **off until you hire a General Manager**. Until then Smolcorp
works as it did in 1.22: nothing below changes how you hire or what you see.

- **A General Manager that runs a team.** Press **Hire your General Manager**
  in setup's Start here. It starts with its rules and tools already loaded,
  asks what the goal is, makes a plan, and waits for your OK. Then it hires,
  briefs, messages and fires employees for you, and only comes back to you
  for what's yours.
- **Every hire gets its own branch.** Each employee works in its own git
  worktree, so they don't step on each other or on your main branch, and
  they're seated together, a row at a time.
- **Smolcorp remembers the team.** Who works where, what each is doing and
  what they last reported is kept by the app, so restarting the GM loses
  nothing.
- **Messages between the team**, with a record, and mail to your cloud bots
  through a folder. Employees can only write to the GM and to the people you
  let them write to.
- **Watches**: the GM can ask Smolcorp to tell it when a file changes, a time
  comes, or an employee goes quiet. A hire that writes `NEEDS-REVIEW:` in its
  notes gets a "Ready for review" card on your phone.
- **Your phone shows what's waiting for you**: a hire's question or permission
  prompt, with one notification for each. **Only you answer a permission
  prompt**, in its terminal window or with your own tap on the deck or your
  paired phone: neither the GM nor any hire can answer one for you. The
  waiting list itself is for reading.
- **A fence around employees.** Claude Code hires and the GM are set up to
  refuse `git push`, deploy, publish and cloud commands, to keep out of your
  `.env` files and keys, and to leave Smolcorp's own settings alone. A hire
  can't use the setup page's actions; the GM can run only the ones its job
  needs. You can see and extend the list in setup's Manager pane. This is a
  fence against mistakes, not a wall: a script an employee writes and runs can
  get past it, other agents (not Claude Code) get no fence, and everyone still
  runs as you. The docs say how to add a separate Mac account as a real wall.
- **Several Claude accounts.** If you use more than one, tick the ones
  Smolcorp should connect; nothing is written to an account you didn't tick.
- **Tiling.** The GM can lay the team's terminal windows out on a screen.
- **A General Manager you start yourself is recognised.** With **Allow a
  General Manager started by hand** on, Settings → Manager says "Your General
  Manager was started by hand." and doesn't count it as an employee, and Easy
  setup stops offering to hire another. Its session has the role `gm` in
  `smolcorp gm team`. Smolcorp keeps which session it is (the session id only,
  in a file only you can read), so this is still there after you restart
  Smolcorp. It's forgotten when you rotate the token, turn the switch off or
  fire the GM. It's a label: it gives the GM no new power. Its `smolcorp` has
  to be from this version, so it can say which session it is.
- **The GM can pin a desk and name a person.** `smolcorp gm pin [--desk N]
  [--folder F]`, `smolcorp gm unpin --desk N` and `smolcorp gm nickname <name>
  [--folder F]` (16 characters; `""` takes the nickname away), also as MCP
  tools. Desks count **from 1** in every `smolcorp gm` command, as in the
  office.
- **`smolcorp` in Terminal.** Setup can add the `smolcorp` command to
  `~/.local/bin`, and takes it away again.
- **Keys pressed in other apps are off**, and stay off unless you turn the
  switch on and list the apps they may go to.
- Read more in `docs/manager`.

On Linux and Windows the manager is only started in this version. The
`smolcorp gm`, `smolcorp guard` and `smolcorp mcp` commands ship on all three
computers, but the app there answers only a few of the GM's questions: which
version it is, and the manager switch in setup; once the manager is on, also
the team, the log of what happened and who is waiting for your answer; `gm pin`,
`unpin` and `nickname` work there too. The rest of what the GM does with a team
(hiring and firing for you, briefs, messages, watches, tiling, the fence, a
branch for each hire) is the Mac's for now, and so is recognising a General
Manager you started by hand. You can still hire the **General Manager** role
from Start here on Linux and Windows, as in 1.22, but it doesn't run a team
for you there.

## Safer pairing and setup

- **Setup opens with a one-time link.** The menu, the office's ⚙︎ and the
  Settings window open setup with a link that works once, for five minutes.
  Any agent on your computer used to be able to read the setup key and use it
  to pair a phone of its own; now it can't.
- **For scripts that use the setup key:** it **no longer works as a cookie**,
  and a saved `#k=` link no longer opens setup. On the Mac, sending it as the
  `x-smolcorp-setup-key` header still reads setup state, runs the check and
  repair that `smolcorp setup` does, and the plain look-and-feel switches
  (theme, music, dock, keep on top), but nothing that matters for safety: it
  can't pair or unpair a phone, show a pairing code, hire, connect or
  disconnect agents, change the webhook, bots or layout, or turn on anything
  that spends money or sends elsewhere (Summaries, their API key, where they're
  sent, the viewer). Those are for the setup page (and, for team work, the
  General Manager). On Linux and Windows the same limits apply, with one
  difference: the key can also install this copy and open the firewall for
  how Smolcorp already listens.
- A request that carries both the setup key and the GM token works. Each
  request has one caller, picked in the same order on every computer: the page,
  a hire's key, the setup key, the GM token. A wrong credential changes
  nothing.
- To open setup, use the menu bar's Settings… on a Mac, or the tray's
  Open Setup… on Linux and Windows.
- Smolcorp's own Keychain reads no longer hold the office up, and the
  portable viewer's key stays in the Keychain until you need it.

## On the Mac

- **Anyone on any floor.** Move an agent with the ⇅ floor key or by dropping
  it on the elevator, or give a folder a floor in setup's Office pane: its
  agents sit there from then on, each keeping their desk. Hire on any floor,
  by role, at any free desk: hold the desk you want and pick who to hire. The
  hire sits there, and stays there after a refresh, `/clear` and a restart. A
  taken desk is refused ("Someone's at that desk already."); with no desk
  chosen, the hire takes the next free one.
- **Summaries on your own plan**, on Linux and Windows too: Claude Code or
  OpenCode can write them, as well as your Anthropic API key. Setup's
  Summaries card says which ones can on this computer, and why one can't
  (Codex, Gemini CLI, Grok Build and Cursor are listed but not supported yet).
- **Keep the office on top** of other windows: a switch in setup's Office
  pane. The office's corner key is **⚙︎ Settings**.
- Setup shows **which version** you're running, and says "Mac" where it
  means the Mac.
- **If macOS opens a temporary copy of Smolcorp**, because the download's
  quarantine mark is still on the app in Applications, the install check says
  so and gives the command (Smolcorp never removes the mark itself):
  `xattr -d com.apple.quarantine '/Applications/Smolcorp.app'`. Then allow
  Accessibility again.
- **Getting `smolcorp` on your PATH.** When `~/.local/bin` isn't on it, setup
  shows the exact line for your shell with a Copy button: zsh `~/.zprofile`,
  bash `~/.bash_profile`. Smolcorp writes no shell file itself.
- **This Mac build is signed with a Developer ID** and the hardened runtime.
  If macOS says it can't check the app the first time, open System Settings →
  Privacy & Security and click **Open Anyway**, once.
- The browser viewer seats people at the same desks as the Mac.
- **Connect** says when it turns on Claude Code's voice input (the phone's mic
  key needs it), and Disconnect turns it off again unless you've changed it
  since. Start here's **Connect them** and the setup command connect without
  touching voice. `smolcorp setup` leaves agents you disconnected as they
  are.
- A hire is refused, with how to fix it, when its agent isn't connected or
  hiring has no folder to use: before, its terminal opened but it never
  reached a desk.

## On Linux and Windows

- **Linux listens on this computer and Tailscale only**, as Windows does. If
  you already paired a phone, it keeps listening on your network, as before.
- **Linux: `smolcorp quit`** stops the running Smolcorp, and from this version
  on, `install.sh` stops the old copy itself before updating.
- **`install.sh` shows the PATH line** when `~/.local/bin` isn't on your PATH:
  zsh `~/.zshrc`, bash `~/.bashrc`. It changes no shell file.
- **Hook commands name the office's port**, so two offices on one computer
  don't mix. Agents are moved over the first time this version starts.
- The setup link Smolcorp prints when it starts, and the tray's **Open Setup…**,
  work once, for five minutes, as on the Mac. `smolcorp setup --open` no longer
  makes a link; it tells you where to get one.
- **New commands.** `smolcorp gm`, `smolcorp guard` and `smolcorp mcp` are on
  Linux and Windows too (`smolcorp-cli` on Windows), as on the Mac.
  `smolcorp quit` (below) is new on Windows too, where it tells you to quit
  from the tray.
- Linux: hook commands work from a folder whose name has quotes, `$` or
  backticks in it; the login entry is quoted properly, uses the installed
  icon, and moves from a portable copy to the installed one; looking for a
  tray no longer starts a D-Bus daemon where there's no session bus; setup's
  wording fits Linux (no tray, install row, scroll switch). The office's ⚙︎
  and Open the office say so when your browser won't open.

## Fixed

- Setup no longer shows the end of your API key, with Summaries on or off.
- Opening the setup page without its key no longer counts as setup done.
- Start here no longer stays hidden when the first check fails: it tries
  again, then shows a Retry. **Hire your General Manager** says it's hired
  only once it's at a desk, and to answer Yes to Claude Code's trust
  question if it asks. A managed hire (manager on, or one with its own
  worktree) on your API key no longer stops at Claude Code's "use this API
  key?": Smolcorp records the answer for that key (only its last 20
  characters, in the account's Claude Code settings file) when it hires.
  **Open the office** on setup says whether it opened.
- Connect and Disconnect show what went wrong on the page, and Disconnect
  carries on past an agent it can't change.
- Escape closes a desk's hold menu.
- **Settings → Office opens faster.** Style previews are drawn as they come
  into view (one when the tab opens, not 102), nothing is redrawn without new
  data, and a refresh never overlaps the one before it. Its pictures are
  hidden from Accessibility.
- Saving a backup as "Default" is still refused, and now says why: "Default" is
  Smolcorp's own profile, so give the backup another name.
- **One old outfit is gone** from the list, on every computer. A saved layout
  or dress code that named it falls back to the default.
- The Tailscale instructions name the port this office runs on, and long
  Tailscale addresses wrap.

## Updating

- **Mac:** drag the new Smolcorp into Applications.
- **Windows:** quit Smolcorp from its tray icon, then open the new
  `Smolcorp.exe`.
- **Linux:** quit Smolcorp first (1.22 can't be stopped by the installer, and
  has no `smolcorp quit`): from its tray, or press Ctrl-C in its terminal if
  your desktop has no tray. Then run the new `install.sh`. Later updates stop
  it for you; `smolcorp quit` does it by hand.
- `FIRST-SETUP.md` is still with the release (and inside the Windows and
  Linux downloads) for letting your coding agent do the setup.

---

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
