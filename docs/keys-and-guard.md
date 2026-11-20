# Keys, records and the guard

How Smolcorp keeps its keys away from the agents it starts and the other
programs on your computer, what you'll notice, and what it can't stop. Back to
the [README](../README.md).

Everything here runs as you, so the question is always the same: what can a
hire, or any other program of yours, do that only you should? From 1.26,
Setup's key stays in Smolcorp's memory, the records that let things in can't be
forged on a Mac, and the programs Smolcorp starts don't inherit what you
allowed Smolcorp to do. A hire can't loosen the guard while Smolcorp runs, and
on a Mac not even while it's stopped. Windows and Linux can seal less, and the
last section says plainly what's still open.

## Where the keys are

- **Setup's key is in memory only.** It's new at each start, and so is the
  office window's on Windows. Smolcorp keeps only a fingerprint of each hire's
  key and the General Manager's, and on a Mac, of each phone's.
- **A few keys are still in files,** readable by you alone:
  - `hook-key` (and on a Mac, `hook-header`): your agents' hooks report what
    they're doing with it, and a second Smolcorp uses it to bring the office
    forward. On Windows and Linux, a desk's launcher connects with it too.
  - `selftest-key`: it reads Start here's checklist and opens nothing else
    (see below).
  - On Windows and Linux, `phones.json`: the secret each phone signs its
    requests with, so it opens what the phone opens (see below).
- **The setup link works once.** It's good for five minutes from the Mac's
  menu, which opens Settings with no command line, and for one minute on
  Windows and Linux, where the browser is started with the link on its command
  line and any program of yours could read it. If another program opens it
  before you, Smolcorp ends that program's session at once, and your page says
  so: "Another program on this computer opened Setup with this link first." It
  also lists what that session changed. Open Setup again from Smolcorp's menu.
- **`smolcorp setup` only checks.** It, and Smolcorp's `--setup` on a Mac
  (`/Applications/Smolcorp.app/Contents/MacOS/Smolcorp --setup`), read the
  checklist with the check-only key in `selftest-key`, which opens nothing
  else. The fixes are on the setup page.
- **A tool of yours asks, with a code** (on a Mac, and on Windows with
  WebView2). `smolcorp demo`, `status` and `desks` each ask at every run:
  Smolcorp's Settings window shows the ask with a four-digit code, and you click
  **Allow** once you've checked that the terminal shows the same code. A hire
  can ask too: on a Mac it still reaches Smolcorp from its sandbox, so an ask
  from a program in a sandbox is refused, and so is one no program is still
  waiting on; when macOS can't tell whether the program that asked is in a
  sandbox, the window says it may be a hire. On Windows, where hires
  have no sandbox, the window names the program that asked, as Windows says
  it. On Linux, and on Windows without
  WebView2, they stop and say so: see your team and set things up in Setup
  instead. On Windows, `smolcorp demo` asks the same way, makes its folder and
  opens its page in your browser, but hiring from that page hasn't been tried
  there yet: the demo is still made for a Mac.
- **A General Manager you start by hand** (on a Mac, and on Windows with
  WebView2) gets its token the same way: turn on **Allow a General Manager started by hand**, run
  `smolcorp gm login`, check the code and click **Allow**. Smolcorp keeps no
  such token across a restart while no General Manager is hired, so after
  Smolcorp starts again (an update, say), run `smolcorp gm login` again. A
  General Manager you hire from Setup keeps its token.

## Records nobody else can write (Mac)

What lets something in (paired phones, hires' keys, the General
Manager, who may write to whom, your security settings and your guard) is kept
in records that Smolcorp seals with a key in your login keychain. Only
Smolcorp's own app reads that key, and only while it holds the lock on
Smolcorp's folder. Smolcorp reads each record once, as it starts, and holds it
in memory.

- **A record another program changed doesn't count.** Setup's **Paired
  phones** says so, and none of the records is honoured until you **Reset**
  them: every phone pairs again, and what the check dropped is
  gone for good. Setup says exactly what a reset keeps beside the button.
- **A locked keychain.** If Smolcorp can't read its key without asking you,
  nothing it saved counts, and it saves nothing until it can. Until then, it
  refuses a change that removes something, such as unpairing a phone, since it
  couldn't be saved. Unlock the keychain and restart Smolcorp: that loses
  nothing you saved before.
- **A Smolcorp you build yourself** is signed differently from the one you
  installed, and so is each ad hoc rebuild. macOS keeps Smolcorp's Keychain
  items for the signature that made them, so such a build can't read them: its
  records read as unreadable and ask for a reset, and the API key has to be
  entered again.

**On Windows and Linux** there's nowhere to keep a key that only Smolcorp can
read, so nothing is sealed. Smolcorp reads its files once as it starts, and
ignores changes made while it runs. A change made while it's stopped counts at
the next start (see [What this doesn't stop](#what-this-doesnt-stop)). The
secret each phone signs its requests with stays in `phones.json`, readable by
you alone: a program of yours that reads it can do whatever that phone can.

## Updating to 1.26: every key changes once

What carries over, what resets and what to do: [Updating to 1.26](upgrade-1.26.md).

- **Pair each phone again.** The keys they held open
  nothing any more, and a phone from before isn't offered back.
- **Hires get new keys** when they next start, and the General Manager when
  you hire it again. Desks started before the update keep the settings they
  started with until they're restarted, and Smolcorp never stops a desk by
  itself.
- **HTTPS through `tailscale serve`, if you used it, has to be turned on again**,
  once: see [HTTPS through tailscale serve](phone.md#https-through-tailscale-serve-mac-and-linux).

## The guard holds

The guard is the hook that refuses the commands you've barred (pushes,
deploys, publishing and the rest): see
[The fence, and who gets it](team.md#the-fence-and-who-gets-it).

- **A desk's settings come with its start,** not from a file in its folder, and
  a hire can't make Claude Code read its settings again in the middle of a
  session. A change to a hire's settings, by the hire or by you, applies when
  it next starts. A rule you add to the guard applies at once.
- **A guard that can't answer stops the call.** A desk needs Claude Code
  2.1.314 or later, which refuses a command when its guard is stopped, killed
  or stalled. With an older one, no desk starts: "Update Claude Code: a desk
  needs 2.1.314 or later, which stops a call when its guard can't answer. No
  desk was started."
- **Smolcorp keeps the guard you set in Setup.** While Smolcorp runs, a change
  to `hire-guard.json`, by hand or by another program, can only add rules, and
  your next save in Setup drops it. A guard file that's a link, a hard link, a
  pipe, a device, or over 64 KB counts as broken. While it is, the guard
  refuses every command a desk runs, and Setup shows every group on until you
  save there.
- **On Windows,** a desk's settings must fit on its command line. If they don't,
  the hire is refused and asks you to turn some guard rules off in Settings.
  Claude Code installed with npm can't take a desk's settings there at all, so
  such a desk is refused: install Claude Code with Anthropic's own installer.

## How a desk starts

- **The General Manager and the Claude Code hires it manages, in tmux (Mac
  and Linux), start with a ticket, not a login shell.** The ticket works once,
  and only for the program Smolcorp started for that desk. Neither your shell's
  start files nor your `~/.tmux.conf` run as such a desk starts, so a variable
  you set there doesn't reach its environment (Claude Code's Bash tool still
  reads your shell's own setup).
- **On a Mac, a desk in its sandbox changes files only in its own worktree or
  project folder in your projects folder** (the folder for hires you chose in
  Setup). Your other projects there, their git settings and hooks, and any
  project you add later are read-only to it. It's told so as it starts: "You
  can change files only in your own worktree here; other projects are
  read-only." Your preferences folder, `~/Library/Preferences`, is read-only
  to it too, and it can't ask macOS's preferences service (cfprefsd) to
  change a setting for it. The settings an app keeps elsewhere, in
  `~/Library/Application Support` or its own container, aren't held, except
  your terminals' (see "Links deep in your agents' and apps' folders" in
  [What this doesn't stop](#what-this-doesnt-stop)). No hire works in the
  projects folder as a whole. When you hire
  from the office, whatever the agent, it first asks you which project (the
  General Manager aside: it gets a workspace of its own). The list shows each
  repository there, each repository inside a plain folder (`clients/acme`, not
  `clients`), and any folder that holds no repository at all. Every hire gets
  its own worktree of the repository you pick and commits there, on a branch
  of its own: a Claude Code hire by role, and a plain hire too (one with no
  role, or a Claude Code role with the manager off). A plain hire's worktree
  is, by default, in the `<repository>-hires` folder beside your checkout, on
  a branch named `hire/plain-…`. When it ends (you fire it, close its window, or its
  agent quits), Smolcorp writes its recap. It removes the worktree once
  nothing in it is left uncommitted, and the branch once your checkout holds
  it; what's left stays. A worktree with a submodule checked out always
  stays. Codex, Gemini CLI, OpenCode and Grok aren't hired on a
  Mac with the sandbox on. Setup and the General Manager have no list: the
  hire goes into the folder you choose or type in Setup, or the one the
  General Manager names. It isn't made if that folder is the projects folder
  itself, or holds it, or isn't a repository but holds one (hire it into that
  repository itself instead), or holds a bare repository. A desk made that
  way before doesn't start.
- **Other repositories inside a desk's folder.** The git folder of a
  repository that's already there when the desk starts (a clone nested in a
  checkout, `clients/acme` when your projects folder or your home is itself a
  repository) is read-only to it, and the folders on the way to it can't be
  moved, so it can't change that repository's settings or hooks, though it
  can change its files. It can't make a git folder there either: `git init`,
  `git clone` and `git submodule update --init` inside its folder are refused.
  Your project's own submodules are checked out in a hire's worktree when
  Smolcorp makes it, each one your checkout has initialised, and their git
  folders are read-only to the hire too. It can change a submodule's files,
  but it can't commit in a submodule, move one to another commit, or
  initialise one. To give a hire a submodule your checkout hasn't
  initialised, initialise it in your checkout and hire again.
  It can still make a repository somewhere else and move it in, or link to
  it. Smolcorp looks at each start: a repository that wasn't there at the
  desk's last start (Smolcorp can't tell whether you or the desk put it
  there), a link to a repository a desk can change or to a folder that holds
  one, a hire's own submodule over a repository your project doesn't have,
  or a bare repository stops the start until you remove it. Between two
  starts it can't look, so while a desk is working, don't run git in a
  repository that turned up in its folder, or behind a link there. Don't run
  git in a hire's worktree at all, while it works or after it ends: a hire
  can make a nested repository there whose settings your git would run.
  Look at its work from your own checkout (`git diff main...<its branch>`,
  `git log <its branch>`), and remove a worktree that stayed with `git
  worktree remove --force <its path>`, from your own checkout too. That
  also deletes whatever the hire left uncommitted there, so open or copy
  those files first, without git. A package manager's
  own folders, such as Swift's `.build`, Bundler's
  `vendor/bundle/…/cache/bundler/git` and a virtual environment's `src`, are
  open to a desk, so a hire's `swift build` can fetch new dependencies. The
  clones already there when it starts are read-only to it: it can't update a
  dependency fetched before (delete `.build` yourself to fetch afresh), and a
  clone it fetches is its own until its next start. To look for all this,
  Smolcorp goes through at most 50,000 folders at each hire and start, and
  as many again where the links in a desk's folder lead; past that, the hire
  isn't made and the desk doesn't start, and the same goes for a folder it
  can't read, until you make it readable or move it out. If a
  project's git files are shared with another repository (a local `git
  clone` links them), a desk working in it doesn't start until you run `git
  repack -a -d && git prune` in it.
- **What still runs for other desks.** A plain hire in tmux (Claude Code with
  the manager off, or Codex, Gemini CLI, OpenCode or Grok, which on a Mac start
  only with the sandbox off in 1.26.0) starts on a tmux server of its own,
  never your default one, and that server reads no start file, so your
  `~/.tmux.conf` doesn't run there. A desk opened in a terminal
  window no longer goes through a login shell of Smolcorp's, but Terminal,
  iTerm2 and Ghostty start your shell for every window, so its start files run
  first. On Windows, a desk gets a one-time token from Smolcorp's launcher
  instead of a ticket tied to the program it starts.
- **Claude Code is checked before every start** (on a Mac). Smolcorp checks
  that Claude Code from Anthropic's installer is signed by Anthropic before it
  starts a desk, a sign-in, a summary or `smolcorp gm login`. If it isn't, it
  starts nothing: "Smolcorp didn't start Claude Code: the program it found
  isn't signed by Anthropic, so another program may have replaced it.
  Reinstall Claude Code with Anthropic's installer, then try again."
- **What it can't check.** Claude Code installed with npm, Codex, Gemini CLI,
  OpenCode and Grok aren't checked, so Smolcorp can't tell whether another
  program changed them, and Setup says so where you hire. If Claude Code
  changes from a signed copy to an unsigned one, no desk starts on it until
  you click **Use this copy** in Setup: do that only if you installed it with
  npm yourself. On Windows and Linux, no agent's program can be checked this way.
- **If Smolcorp's own files change** (on a Mac), it starts no more Claude
  Code desks and says "Smolcorp's own files changed. Reinstall Smolcorp. No
  desk was started."
- **Empty `.smolcorp-keep` folders in a repository's `.git`** (on a Mac).
  A desk's start keeps one in some of the folders git keeps branches in, such
  as `.git/refs/heads/hire` and `.git/refs/remotes/origin`, so git never
  removes a folder a hire could then make again with another branch inside.
  Git doesn't read them as branches, and they're safe to leave.
- **An empty `.git/lfs` folder** (on a Mac). A desk's start makes git-lfs's
  folder in a repository's `.git` when it's missing, so no desk can put a link
  there. git-lfs keeps its files in it as usual, and it's safe to leave.
- **A link in a repository's `.git/refs` or `.git/logs` stops every desk's
  start there** (on a Mac), and so does a file there with a second name, a
  branch Smolcorp keeps that points at one a desk can move, or a checkout's
  HEAD (yours, or another worktree's) that points at a branch a desk can move.
  The message names it and says what to do. Git makes none of these unless you
  ask it to.

## Smolcorp's permissions stay with Smolcorp (Mac)

What you allowed Smolcorp to do, such as Accessibility and the microphone,
isn't passed on to the programs it starts: hires, git, tmux and the rest. So
macOS may ask you, once, to let "tmux" or "git" use your Documents, Desktop,
Downloads or iCloud Drive folder, or find devices on your Wi-Fi. Allow it if
your projects are there. A project anywhere else in your home folder, like
`~/Projects`, asks nothing.

## Smolcorp's own memory

- **On a Mac,** every build of Smolcorp, yours included, runs with the
  hardened runtime, as do the `smolcorp` and `tmux` programs inside it, so no
  other program of yours can read their memory or change what they do.
- **On Linux,** Smolcorp and its guard tell the system that no other program
  may read their memory.
- **On Windows,** `smolcorp serve` refuses other programs of yours access to
  its memory. The tray app doesn't.

This covers Smolcorp alone. **The agents and the programs they run, such as
MCP servers, get no such protection**, and nothing here keeps one desk out of
another's memory.

## If you build Smolcorp yourself: your signing key

Smolcorp's Keychain items trust whatever is signed with the same certificate as
Smolcorp. While the key you sign Smolcorp with is unlocked, any program of
yours can sign a changed Smolcorp that reads those items and forges its
records. So Start here shows **Smolcorp's signing key is locked away** as a
step to take while it finds that key in an unlocked keychain. That's advice:
Smolcorp looks only in the keychains macOS lists for it, which any program of
yours can change, so the line going away proves nothing.

Keep the key in a keychain of its own that locks after each build, as
[Releasing](RELEASING.md) describes, and name the identity in your terminal
with `SMOLCORP_SIGN_IDENTITY` rather than in `~/.config/smolcorp/sign-identity`,
a file any program of yours can write. Smolcorp from the releases page is
signed elsewhere, so this line doesn't show for it unless the key it's signed
with is on your Mac.

## What this doesn't stop

Said plainly, so you can judge for yourself:

- **A hire can still act as any desk it can reach, the General Manager
  included.** Without a sandbox (on Windows and Linux, and on a Mac for a desk
  Setup lists as **No sandbox**), it can type into a desk's tmux, read its
  scrollback, or restart its pane with a program of its own, which then runs
  without the guard, and it can plant code that a desk runs later, in
  `~/.claude` or your shell's start files. In its sandbox on a Mac, it can't
  reach another desk's tmux, and `~/.claude`, your shell's start files and
  your agents' settings are read-only to it. Nor can it debug another desk's
  programs or yours: only the ones it started itself. Without a sandbox, it
  can still debug a desk's programs that lack the hardened runtime, such as
  Node MCP servers, and on Linux it can also trace them, unless your system
  forbids it.
  On a Mac, it can't open Setup, read your phones' keys, change your guard, or
  forge a record.
- **Program folders outside your home folder, on a Mac.** A hire in its
  sandbox can add or replace programs in Homebrew's folders and the other
  folders on your PATH outside your home folder, like `/usr/local/bin`, and in
  `~/.local/bin`. So a command you run later, such as `ls` or the `ssh` behind
  `git push`, can be one it put there, and so can a zsh function your next
  Terminal window runs at its start, from Homebrew's
  `share/zsh/site-functions`. Only the programs Smolcorp itself runs are held,
  wherever they're found.
- **On Windows and Linux, a program of yours can change Smolcorp's files while
  Smolcorp is stopped**: add a phone, or loosen the guard for hires started
  later. Smolcorp takes the change at its next start, and a program can stop
  Smolcorp to make that start come sooner. Start here says this too.
- **On Windows, the tray app's memory.** Any program of yours can read it,
  Setup's key and the office window's included, and so open either.
  `smolcorp serve` refuses that.
- **Secrets outside Smolcorp's memory.** On Windows and Linux, any program of
  yours can read and write the system's secret store (the Secret Service, or
  the Credential Manager), where the API key is kept. It can also read a desk's
  keys, the General Manager's token among them, from that desk's environment. A
  hire on your API key holds that key in its own environment, on every system.
- **On a Mac, a program that can sign as Smolcorp** reads its Keychain items:
  see [your signing key](#if-you-build-smolcorp-yourself-your-signing-key).
  Replacing your whole login keychain also works, but breaks every app's
  sign-ins on the Mac, so you'd notice.
- **On a Mac, a copy of Smolcorp.app started while yours is closed** uses
  Smolcorp's Keychain items as yours does. It can't read them out, only do with
  them what Smolcorp does.
- **Files replaced between a check and a start.** Claude Code is checked, then
  started by its path, so a file swapped in that moment starts unchecked.
  Claude Code installed with npm, Codex, Gemini CLI, OpenCode and Grok aren't
  checked at all, nor is any agent's program on Windows and Linux. They run
  with your rights, but none of Smolcorp's. On a Mac, the first Claude Code
  Smolcorp sees after the update sets which kind you have, so an unsigned one
  put there before then counts as yours.
- **The setup link, within its minute** on Windows and Linux: you're told, and
  that session is ended.
- **Links in your projects, on a Mac.** Each start follows the links in a
  desk's folder to wherever they lead, so a link into a cloud folder or
  `~/Documents` is read at every start. It looks through at most 50,000
  folders where they lead; past that, the desk doesn't start until you
  remove the links it doesn't need.
- **Links deep in your agents' and apps' folders, on a Mac.** At each start,
  Smolcorp follows the links in your terminals', agents' and keychains'
  folders, and in `~/.claude` (and any other Claude Code account folder), so a
  settings file you keep as a link into a dotfiles folder is held where it
  leads. It looks there only two levels deep, and not at all into what an
  agent adds to at every session: its sessions, checkpoints, snapshots,
  temporary files and downloaded packages, and in `~/.claude` the `projects`,
  `file-history`, `todos`, `shell-snapshots`, `debug` and `session-env`
  folders. A link deeper in, or inside one of those, isn't followed, so a hire
  may write where it leads. A hire can't make such a link itself: only you or
  an app of yours could have. Two places are looked into further:
  `~/.claude`'s `skills`, `commands`, `agents` and `hooks`, whole, with
  wherever their links lead, and iTerm2's `Scripts/AutoLaunch` folder, two
  levels deep (a full-environment Python script's main file, three levels in,
  isn't followed if it's a link). What it looks through in `~/.claude` counts
  toward the 200,000 files and folders Smolcorp looks through at most, with
  your other folders that are looked into whole. If a skill brings a lot with
  it (a `node_modules` folder, a Python virtual environment), remove what you
  don't need, or keep it somewhere no link in `~/.claude` leads.
- **An agent's folder you moved, on a Mac.** Smolcorp holds your agents'
  folders where they're kept by default (`~/.codex`, `~/.gemini`, `~/.grok`,
  `~/.agents`, and OpenCode's in `~/.opencode`, `~/.config`, `~/.local` and
  `~/.cache`). If you moved one with `CODEX_HOME`, `GEMINI_CLI_HOME`,
  `GROK_HOME`, `XDG_CONFIG_HOME` or the like, Smolcorp holds it where it went
  only if the variable is in launchd's environment: set it with `launchctl
  setenv`, then quit and reopen Smolcorp. A desk that was already running
  holds it only from its next start: fire it and hire it again. A variable
  set only in your shell's start files doesn't reach an app opened from the
  Finder or at login, so that folder isn't held, and a desk can change what
  your agent keeps there. On Windows and Linux there's no sandbox yet, so no
  agent's folder is held there, moved or not.
- **zsh's files in a folder you moved them to, on a Mac.** Smolcorp holds
  zsh's start files in your home folder and in `~/.config/zsh`. If you set
  `ZDOTDIR` to another folder, they're held there only if `ZDOTDIR` is in
  launchd's environment: set it with `launchctl setenv ZDOTDIR <folder>`,
  then quit and reopen Smolcorp. Set only in `~/.zshenv`, it doesn't reach
  Smolcorp, and a hire can change the files there.
- **What your shell's start files load, on a Mac.** Smolcorp holds zsh's and
  bash's own start files, oh-my-zsh in `~/.oh-my-zsh` or your `ZDOTDIR`, and
  `~/.p10k.zsh`, not the other files and programs they load from your home
  folder: a plugin manager's plugins, Powerlevel10k's cache in `~/.cache`,
  nvm's `~/.nvm`, `~/.cargo/env`, conda. A hire can change those, so your next
  terminal window runs what it put there.
- **Local servers that run commands, on a Mac.** A hire in its sandbox can
  still connect to most servers on your Mac's own addresses, since the sandbox
  can't tell yours from the ones a hire starts for its work. One that runs
  commands or reads files for whoever connects does that for a hire too: a
  virtual machine's SSH port, Node's inspector, Jupyter, a browser's debugging
  port. Smolcorp holds the SSH keys of Lima, Colima, Rancher Desktop and
  OrbStack machines, and of Podman's default machine (`~/.lima/_config`,
  `~/.colima/_lima/_config`,
  `~/Library/Application Support/rancher-desktop/lima/_config`,
  `~/.orbstack/ssh` and Podman's default machine keys), so a hire can't read
  them. A `LIMA_HOME` or `COLIMA_HOME` moved elsewhere isn't followed, so
  keys kept there stay readable. Stop such a server while hires work if you
  don't need it.
- **A hire's window in your terminal app.** While one is open, what the hire
  prints reaches Terminal, iTerm2, Ghostty or whichever terminal app shows it,
  including sequences some terminals act on, such as one that sets your
  clipboard (OSC 52). Don't paste what a hire's window put on your clipboard
  without reading it first.
- **Remote-tracking branches, on a Mac.** A hire can update `refs/remotes/*`,
  as its own `git fetch` needs to. Fetch before you merge `origin/*` into your
  own branch, so you merge what the remote really holds.
- **Git tools other than git's own command line, on a Mac.** If you use
  Xcode's or Eclipse's source control (or another libgit2 or JGit tool) on a
  repository while hires work in it, a hire could change its packed refs. Use
  git's command line there, or fire the hires first.
- **Very large repositories.** In a repository with millions of objects, the
  fire's check of the stored objects can't finish within its two minutes. The
  hire's worktree and branch are then kept, and you merge its work by hand.
- **Installers and updaters, on a Mac.** A package manager or an app's updater
  that stages files in a folder of its own can be watched by a hire while it
  runs. Install and update with no hire working.
- **A network or FUSE-T volume holding git's hooks, on a Mac.** If a network
  share or a FUSE-T volume (a vault, an rclone mount) is mounted in your home
  folder and holds git's hooks, or a file git's config names, at its top, no
  desk starts or stops while it's mounted. Smolcorp says which one it is:
  unmount it to go on.
- **A General Manager you started by hand** runs without the guard.
- **The hook key** stays readable in `hook-key`. A program that reads it can
  report events as if they came from your agents, and connect as a desk's
  launcher.
- **The webhook's secret** stays readable in `config.json`.
- **A server on Smolcorp's port while Smolcorp is closed** receives a phone's
  key when the phone reconnects.
- **Whatever you copy** to the clipboard, and **a hire that can `sudo` without
  a password**, which can do anything.
- **Anything you can do yourself**: read your files and SSH keys, push with your
  credentials, or stop Smolcorp.
