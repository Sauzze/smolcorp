<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/logo-dark.svg">
  <img src="docs/logo-light.svg" alt="Smolcorp" width="420">
</picture>

# Smolcorp

[![CI](https://github.com/Sauzze/smolcorp/actions/workflows/ci.yml/badge.svg)](https://github.com/Sauzze/smolcorp/actions/workflows/ci.yml)

**Smolcorp gives your AI coding agents a General Manager.** It plans the work with you, hires one agent per task on its own branch, checks the proof, gets a second agent's review, and asks you before anything ships. Watch the whole team in a pixel-art office, from any browser.

<p align="center">
  <img src="docs/office-reel.gif" alt="The office at work: on a busy ground floor, agents dressed for their roles sit at desks each furnished its own way; the general manager, crown on its badge, plans a release at the middle desk, one agent waits on you with its keys up, and a new hire walks in to take the last desk; the tester's desk turns green when all its tests pass. The elevator's call button opens a list of the floors with how many are on each, and the elevator rides up to a server room of headless runs with robot badges, then a floor of cloud bots whose desks turn green as their reports come in, then a research floor dressed as a space station, and back down" width="800">
</p>

Tell the General Manager the goal. Each agent it hires works in a git
worktree of its own, and nothing is pushed, merged, published, deployed or
paid for until you say so.

The office shows who's working, who's waiting for you and who's done. Answer a
permission prompt from across the room, interrupt an agent, or hold the mic
and talk to it. It opens in any browser, your phone's too, over your Wi-Fi or
[Tailscale](https://tailscale.com): no account, and no app to install.

Works with Claude Code, Codex, Gemini CLI, OpenCode and Grok, in your own
terminal. Free and open source (MIT).

**For the Mac** (beta), **Windows and Linux** (alpha). From 1.25.0, the General Manager hires its team on all three (on Windows, with managed hires turned on).

> Smolcorp is an independent project. It is not made by or affiliated with Anthropic.

## How a day with Smolcorp goes

1. **Set it up once.** Download, open, and follow the arrows: choose where
   your hires work, check what the first hire needs, hire your General
   Manager.
2. **Give it a goal.** It asks what the team may spend and use, and what you
   want to approve yourself, then shows you the plan.
3. **Approve the plan.** It hires the team (a Reviewer, a Tester, a Bug
   hunter, or roles of your own), briefs each one, checks their proof, and has
   each change reviewed by a second agent.
4. **Approve what ships.** Pushes, merges, releases and deploys wait for you.

To watch all of this on a sample project first, in a folder of its own, run `smolcorp demo`: a browser page opens for the one click that hires its General Manager ([The demo](docs/demo.md)).

You can also hire anyone yourself: hold a free desk in the office and pick a
role. [Your team](docs/team.md) has every role and what the General Manager
can do.

## What keeps it safe

- **Only you answer permission prompts.** The General Manager and its hires
  can't.
- **A fence against mistakes, for the General Manager and the Claude Code
  hires it manages.** They start with deny rules and a guard that refuse
  `git push`, deploys, publishing and cloud commands, and keep out of `.env`
  files and keys. **Hires of Codex, Gemini CLI, OpenCode and Grok don't get
  this fence, and nor do some Claude Code hires you make yourself, such as one
  with no role on the Mac while the manager is off:** they run with their own settings.
  Who gets it differs from system to system: see
  [The fence, and who gets it](docs/team.md#the-fence-and-who-gets-it).
  It's a fence, not a wall: everyone runs as you.
- **The General Manager's hires each work on a branch of their own**, so
  nothing lands on main without you.
- **Nothing leaves your network unless you turn it on.** No account, no
  analytics, no cloud service of its own. See
  [Setup, privacy and security](docs/setup.md#privacy-and-security).

## Install

**The quickest way: let your coding agent do it.** Download
[**FIRST-SETUP.md**](https://github.com/Sauzze/smolcorp/releases/latest/download/FIRST-SETUP.md)
from the latest release, open your coding agent in the folder you downloaded
it to, and paste what's in it. It installs Smolcorp, connects your agents,
checks everything, and tells you what only you can do.

Or by hand, from the
[latest release](https://github.com/Sauzze/smolcorp/releases/latest):

- **Mac** (macOS 13 or later): download **Smolcorp.dmg**, drag Smolcorp into
  Applications and open it. Or `brew install --cask sauzze/tap/smolcorp`.
- **Windows** (alpha): download **Smolcorp-Windows-x86_64.zip** (or
  `-arm64`), extract it and open **Smolcorp.exe**. It isn't signed yet, so
  SmartScreen warns you the first time: **More info**, then **Run anyway**.
- **Linux** (alpha): download **Smolcorp-Linux-x86_64.tar.gz** (or `-arm64`),
  then `tar -xzf Smolcorp-Linux-x86_64.tar.gz && ./Smolcorp-Linux-x86_64/install.sh`.

The setup page opens in your browser, on **Start here**. Every way to
install, building it yourself, and what each setup step does are in
[Install and set up](docs/install.md).

## More

- [The office](docs/office.md): floors, styles, outfits, arranging the room
- [Your team](docs/team.md): every role, hiring by hand, the General
  Manager's commands, the fence, T3 Code, the agents and terminals it works
  with
- [Your phone](docs/phone.md): pairing, the keys, typing and voice
- [Setup, privacy and security](docs/setup.md): what's sent and what never
  is, and setup from a script
- [Windows and Linux](docs/windows-linux.md): what differs, who can reach
  Smolcorp, building it yourself
- [Portable office, summaries and other tools](docs/extras.md)
- [Troubleshooting and uninstall](docs/troubleshooting.md)
- [How it works](docs/how-it-works.md)
- [Release notes](RELEASE-NOTES.md)

Smolcorp was called SauzzeDeck Pro up to version 1.6.1, and AgentDeck up to 1.0.1.

## Support the project

Smolcorp is free. If it earns a place on your desk and you'd like to say
thanks, you can send ETH, BNB or any token to this address, on Ethereum, BNB
Smart Chain or another EVM network:

```
0x7a81788cf4cbb7357dfe3d98ab85a6422aab26c7
```

Check the address after pasting it: transfers can't be reversed.

## License

[MIT](LICENSE)
