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
- **`smolcorp setup` only checks.** It, and `AgentDeck --setup` on a Mac, read
  the checklist with the check-only key in `selftest-key`, which opens nothing
  else. The fixes are on the setup page.
- **A tool of yours asks, with a code** (on a Mac). `smolcorp demo`, `status`
  and `desks` each ask at every run: Smolcorp's Settings window shows the ask
  with a four-digit code, and you click **Allow** once you've checked that the
  terminal shows the same code. On Windows and Linux they stop and say so: see
  your team and set things up in Setup instead.
- **A General Manager you start by hand** (a Mac only) gets its token the same
  way: turn on **Allow a General Manager started by hand**, run
  `smolcorp gm login`, check the code and click **Allow**. Smolcorp keeps no
  such token across a restart while no General Manager is hired, so after
  Smolcorp starts again (an update, say), run `smolcorp gm login` again. A
  General Manager you hire from Setup keeps its token.

## Records nobody else can write (Mac)

What lets something in (paired phones and Macs, hires' keys, the General
Manager, who may write to whom, your security settings and your guard) is kept
in records that Smolcorp seals with a key in your login keychain. Only
Smolcorp's own app reads that key, and only while it holds the lock on
Smolcorp's folder. Smolcorp reads each record once, as it starts, and holds it
in memory.

- **A record another program changed doesn't count.** Setup's **Paired
  phones** says so, and none of the records is honoured until you **Reset**
  them: every phone and paired Mac pairs again, and what the check dropped is
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

- **Pair each phone again, and each paired Mac.** The keys they held open
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
- **What still runs for other desks.** A plain hire in tmux (Codex, Gemini
  CLI, OpenCode, Grok, or Claude Code with the manager off) starts on tmux's
  usual server, which runs your `~/.tmux.conf`. A desk opened in a terminal
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
  included.** It can type into a desk's tmux, read its scrollback, or restart
  its pane with a program of its own, which then runs without the guard. It can
  debug a desk's own programs that lack the hardened runtime, such as Node MCP
  servers, or plant code that a desk runs later, in `~/.claude` or your shell's
  start files. On Linux it can also trace a desk's programs, unless your system
  forbids it. On a Mac, it can't open Setup, read your phones' keys, change your
  guard, or forge a record.
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
