# Updating to 1.26

What happens when you install 1.26 over 1.24 (there was no public 1.25), on a
Mac, Windows or Linux. Back to the [README](../README.md).

Install 1.26 as you installed 1.24 (see [Install](install.md)): quit
Smolcorp first, then put the new one in its place. Your support folder stays
where it is.

## What carries over

- **Your team:** every hire, its worktree and branch, the Claude account it
  works on, and who may write to whom.
- **Your Claude accounts** and **your office:** floors, styles, pins, the
  layout, the looks and your roles.
- **Your settings**, the security ones included (who can reach Smolcorp,
  hiring, the General Manager, the webhook), and **your guard**, with any group
  you switched off and any pattern you added.

The update never loosens anything. Settings that are new in 1.26 start as
they do for a new setup, so anything that would let more in stays off. Two of
them touch your phone:

- **Typing over Wi-Fi** is off after the update, on every system. A paired
  phone types to your agents over Tailscale only; its keys still work over the
  Wi-Fi. On a Mac this is a change: 1.24 typed over the Wi-Fi too. Setup
  offers to turn it on, under **Paired phones**. Anyone on that Wi-Fi could
  then read what you type.
- **Typing from your phone** is new on Linux and Windows: 1.24 couldn't do it
  there at all. 1.26 lets a paired phone type to your agents over Tailscale,
  as a new setup does, and over the Wi-Fi only once you turn on Typing over
  Wi-Fi.

## What resets, and why

1.26 keeps Smolcorp's keys out of your hires' reach: Setup's key lives in
memory only, and on a Mac the records that let things in are sealed. In 1.24
those keys sat in files any program of yours could read, so at its first
start 1.26 replaces every one of them, once. Links, copies and keys from
before the update stop working.

- **Pair each phone again.**
- **Re-hire your hires from 1.24.** They keep running as they started, but
  their keys no longer work (they can't send messages) and they lack 1.26's
  protections: a key of their own for each desk, settings that come with
  their start, and on a Mac the sandbox. Fire each one and hire it again.
  Smolcorp never stops a desk by itself, so nothing is lost mid-task. On a
  Mac with the sandbox on, hire a Claude Code role in place of a Codex, Gemini
  CLI, OpenCode or Grok hire: 1.26.0 refuses those four there, and they come
  back in 1.26.1.
- **A General Manager you start by hand** (Mac) needs a new token: run
  `smolcorp gm login` again.
- **HTTPS through `tailscale serve`**, if you used it, has to be turned on
  again, once: see [HTTPS through tailscale serve](phone.md#https-through-tailscale-serve-mac-and-linux).

On a Mac, Smolcorp seals what 1.24 left as it finds it. If it can't (say a
test build of 1.26 already made its key in your login keychain), the records
it can't vouch for fall back to their safe defaults: the front desk and
hiring off, no webhook, the built-in guard plus what your file adds. That is
always stricter, never looser. Setup names each one, and you set them again
there.

## On a Mac, two things to do once

- **Restart your Mac** if a hire ran a server or any other background program
  before you updated. From 1.26, firing a hire, Take over, restart and Hand
  back end what it started in its sandbox, but a program started before the
  update ran outside any sandbox, so Smolcorp can't end it.
- **Remove `.git/rr-cache`** in each repository where a hire worked, unless
  you use `git rerere` yourself: a conflict resolution a hire recorded there
  before 1.26 would still be applied to your merges.

## Where Setup says it

**Start here** has a line, **Updated to Smolcorp 1.26**, with what was kept,
what reset, and your hires from 1.24 by name. It stays a step to do until each
of them is hired again, and goes once they are and two weeks have passed since
the update. Beside it, while Typing over Wi-Fi is off, a second line,
**Typing over Wi-Fi is off**, says where to turn it on. `smolcorp setup`
prints the same lines in a terminal.

More on the keys: [Keys, records and the guard](keys-and-guard.md#updating-to-126-every-key-changes-once).
