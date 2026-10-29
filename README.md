<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/logo-dark.svg">
  <img src="docs/logo-light.svg" alt="Smolcorp" width="420">
</picture>

# Smolcorp

[![CI](https://github.com/Sauzze/smolcorp/actions/workflows/ci.yml/badge.svg)](https://github.com/Sauzze/smolcorp/actions/workflows/ci.yml)

<p align="center">
  <img src="docs/office-reel.gif" alt="The office at work: on a busy ground floor, agents dressed for their roles sit at desks each furnished its own way; the general manager, crown on its badge, plans a release at the middle desk, one agent waits on you with its keys up, and a new hire walks in to take the last desk; the tester's desk turns green when all its tests pass. The elevator's call button opens a list of the floors with how many are on each, and the elevator rides up to a server room of headless runs with robot badges, then a floor of cloud bots whose desks turn green as their reports come in, then a research floor dressed as a space station, and back down" width="800">
</p>

<p align="center">
  <img src="docs/demo.gif" alt="Smolcorp on a phone held sideways: the office fills the screen, and the new hire, in an orange tie, works at their desk while coffee mugs drop in as their effort rises; a finger held on their desk brings up their keys along the foot of the room; they wait for an answer, the phone's edge glows orange and ✓ is tapped; then the jukebox plays and heads bob to the beat" width="800">
</p>

A control pad on your phone for the AI coding agents in your terminal: Claude
Code, Codex, Gemini CLI, OpenCode and Grok.

Your agent sessions are people in [a little pixel-art office](#the-office) on
your phone, each at a desk, acting out what it's doing. From the office you can
approve a permission request from across the room, interrupt an agent, change
how hard Claude Code thinks, press buttons of your own, or hold the mic and talk to
it. Smolcorp runs in
your Mac's menu bar, or in the tray on [Windows and Linux](#windows-and-linux).
Your phone connects to it over your Wi-Fi, so there's nothing to install from
an app store.

And it's not only one agent at a time: it's [a small company](#run-a-team).
Hold an empty desk to hire a Reviewer, a Tester, a Bug hunter or a role of your
own, and put a manager at the middle desk to brief the others and check on
their work, while you watch the whole team from your phone.

Smolcorp was called SauzzeDeck Pro up to version 1.6.1, and AgentDeck up to 1.0.1.

<p align="center">
  <img src="docs/deck-portrait.png" alt="The deck on a phone held upright: the office, an agent picked, and its keys along the foot of the room" width="300">
</p>

> Smolcorp is an independent project. It is not made by or affiliated with Anthropic.

## On your phone

The phone opens straight into the office, full screen, upright or sideways as
you hold it. Each desk is one of your agent sessions, named after its project:
the folder the agent started in, or the top of its git repository.

| In the office | What it does |
|---|---|
| **A desk, or its person** | Picks that session and brings its Terminal window to the front. |
| **▲ ▼ ✓ ✕ over the picked agent, when it's waiting on you** | Move through its options, accept or reject. They come up on their own. |
| **A desk, held** | Brings out its keys along the foot of the room: **esc** interrupts it, **⏎ send** presses Return, **/clear** starts a fresh conversation (tap twice), **↑ ↓** move through its menus, then [your own buttons](#make-it-yours). On a Claude Code desk, also paint chips for its thinking effort, from low to max; if Claude is mid-task, the change waits until it finishes. The coffee by the monitor shows the effort: one mug for low, up to four, and a steaming pot at max. A tap elsewhere puts them away. |
| **The red button** | ☢ Max effort for the picked session, in one press. |
| **The mic, held** | Talk to Claude Code. Let go, and Claude sends what you said once it's three words or more. |
| **Two fingers, dragged** | Scroll the picked session's Terminal window; tap with two to jump back to the bottom. Off until you turn on **Let two fingers scroll Terminal** on the setup page. |

How the people show what their sessions are doing is under
[The office](#the-office). The room's edge glows while an agent is waiting on
you.

Sessions keep their order at the desks; with a fixed number of desks, each
keeps its own until it ends. You can pin projects to specific desks and
furnish the office your way.

## Run a team

The office has nine desks on the ground floor and five floors above it, with
twelve desks to a floor, so you can run it like a small company of dozens of
agents.

**Hire by role.** Hold the desk a new hire would take, and pick who to hire:
a **Reviewer**, a **Tester**, a **Bug hunter**, a **Docs writer**, a
**Refactorer**, fourteen more from an Art Director to a Thinker, or any role
of your own, as many as you like. Each role has its own instructions, agent, model and
effort, and comes in wearing its badge, like the Reviewer's glasses and red pen,
and many dress for the job. A role never guesses what to work on: its first
reply asks you which project, and its own questions, such as which branch to
review. On Windows and Linux, `smolcorp run --role reviewer` also starts one
from any terminal.
[More about roles](#the-office).

**Hire a General Manager.** On the setup page, **Hire your General Manager**
on Start here (or hire the role from a free desk) starts one in its own
terminal. It asks you the goal, what the team may spend and use, and what you
want to approve yourself. Then it:

- **plans** the work as tasks one agent can finish and prove in one sitting,
  and waits for your OK on the plan;
- **hires one agent per task**, each with a brief of its own, in its own folder
  or worktree, branch and port range;
- **checks** that each hire really started, and checks its proof itself;
- has **a second agent review** any change before it's merged;
- makes the team's day-to-day calls itself, and **asks you** before anything is
  pushed, merged to main, published, deployed, released, deleted or paid for,
  or reaches a real person;
- keeps its board of what's done and what's left next to its brief.

Smolcorp puts the whole team in one room, so you see at a glance who's
working, who's waiting for you and who's done, and you answer any of them from
your phone.

**Or a manager of your own:** start any session in your projects folder and
tell it its job, for example: *"You're the manager of this team. For each task
I give you, open a new session in the right folder, brief it, and check on
everyone every ten minutes. Have a second agent review any change before it's
merged. Ask me before anything is pushed, released or deleted."*

**Agents without a terminal work upstairs.** A manager, or a script of your
own, can run Claude Code without a terminal, as `claude -p` does. Started with
`SMOLCORP_HEADLESS=1` in its environment, such a run gets a desk upstairs,
away from your keys. There's nothing to press there, as there's no terminal to
type into. Set
`SMOLCORP_PARENT_FOLDER` to the folder of whoever started it, and holding its
desk says who hired it, what it's doing and for how long; tap the hirer to go
down to their desk. A run that ends, or stops being heard from, leaves.

```sh
SMOLCORP_HEADLESS=1 SMOLCORP_PARENT_FOLDER="$PWD" claude -p "Write the changelog for this release"
```

**Six floors.** Over the ground floor there are five more, twelve desks to a
floor, filled in order: cloud bots such as Grok Bots, then runs without a
terminal, then anyone with no desk free downstairs (their keys are still on
your phone's list). Press the call button beside the elevator, by the entrance,
for a list of the floors and how many are on each; pick one and the elevator
takes you there. In setup, under Office → Floors, give each floor a name and a
style: the office's own, the server room, or any of the styles.

## Supported agents

Each agent tells Smolcorp what it's doing through its own hooks, and the
deck presses that agent's own keys:

| Agent | Interrupt | Fresh conversation | Effort and mic |
|---|---|---|---|
| [Claude Code](https://claude.com/claude-code) | Esc | `/clear` | ✓ |
| [Codex](https://github.com/openai/codex) | Esc | `/new` | – |
| [Gemini CLI](https://github.com/google-gemini/gemini-cli) | Esc | `/clear` | – |
| [OpenCode](https://opencode.ai) | Esc, twice | `/new` | – |
| [Grok](https://github.com/xai-org/grok-build) | Ctrl+C | `/new` | – |

The effort chips, the red button and the mic drive Claude Code's own `/effort`
and `/voice`, so they do nothing when another agent is picked. In Grok, ✕ declines with
Ctrl+C, the key Grok uses to cancel a request.

**Codex** asks you to approve new hooks before it runs them: after connecting,
run `/hooks` in Codex once and trust Smolcorp's. Codex runs its hooks in
a background service rather than in your tab, so the deck finds each Codex
session's tab by its project folder.

**OpenCode** 1 and 2 both work. OpenCode 2 runs its plugins in a background
service of its own, so the deck finds each OpenCode session's tab by its
project folder, as it does for Codex; with `opencode --standalone`, they run
in your tab and it's found at once. Either way, run one OpenCode per project
folder for the deck to tell them apart.

## Terminals

Smolcorp follows sessions in any terminal on your Mac, and drives these:

| Terminal | Keys and mic | Needs |
|---|---|---|
| **Terminal** | ✓, also behind other windows and while the Mac is locked | Accessibility |
| **iTerm2** | ✓ | Allow Smolcorp to control iTerm when macOS asks |
| **Ghostty** 1.4 or later | ✓ | Allow Smolcorp to control Ghostty when macOS asks |
| **tmux**, in any terminal | ✓ | nothing |
| **WezTerm** | ✓ | nothing |
| **kitty** | ✓ | `allow_remote_control socket-only` and `listen_on unix:/tmp/kitty` in kitty.conf |

Keys, the mic and typed commands reach the agent without bringing its window
forward, so the phone works while you're in another app, under the screen
saver and with the Mac locked. In iTerm2, Ghostty, tmux, WezTerm and kitty
that needs no Accessibility at all.

- **Ghostty 1.3** can't tell Smolcorp which window a session is in, so its
  desks show what the agent is doing, without the controls. Update to 1.4, or
  run the agent inside tmux.
- **Other terminals** (VS Code, Cursor, Alacritty, Warp): their sessions get a
  desk that shows what they're doing, with no controls. To control one of
  these agents, run it inside tmux.
- **Hiring** opens new hires in Terminal, iTerm2 or Ghostty, as you choose on
  the setup page, and with **Start inside tmux**, in a tmux session of their own.

## What you need

- A Mac running macOS 13 or later, a Windows 10 or 11 PC, or a Linux
  computer. See [Windows and Linux](#windows-and-linux) for what differs.
- One or more of the [supported agents](#supported-agents), running in
  Terminal, iTerm2, Ghostty, WezTerm or kitty, or in tmux inside any terminal
  (see [Terminals](#terminals)). Sessions in other terminals (VS Code,
  Alacritty, Warp…) show on the deck, but can't be controlled from it.
- A phone, on the same Wi-Fi as your computer, or anywhere with
  [Tailscale](https://tailscale.com) on both.

## Install

On the Mac, Smolcorp is in **beta**: signed with a Developer ID and notarised
by Apple, so it opens with no warning. On Windows and Linux it's an **alpha**,
still in testing: the Windows build isn't signed yet, so SmartScreen warns you
the first time.

**The quickest way: let your coding agent do it.** Download
[**FIRST-SETUP.md**](https://github.com/Sauzze/smolcorp/releases/latest/download/FIRST-SETUP.md) from the latest release (the Windows and Linux
downloads have it inside too). Open your coding agent (Claude Code, Codex,
Gemini CLI, OpenCode or Grok) in the folder you downloaded to, and paste what's
in that file. It installs Smolcorp,
connects your agents, checks everything, and tells you what only you can do:
an OK for Windows Firewall or macOS, and pairing your phone.

**Or from a terminal:** `smolcorp setup` on Windows (`smolcorp-cli setup`) and
Linux, `/Applications/Smolcorp.app/Contents/MacOS/AgentDeck --setup` on a Mac.
It does the same, prints a checklist, and ends with 0 when nothing is in the
way.

**On Windows (alpha, in testing):** download **Smolcorp-Windows-x86_64.zip** (or `-arm64`) from the
[latest release](https://github.com/Sauzze/smolcorp/releases/latest), extract
it and open **Smolcorp.exe**. SmartScreen warns about an unsigned app: click
**More info**, then **Run anyway**. Smolcorp installs itself in
`%LOCALAPPDATA%\Smolcorp`, puts `smolcorp-cli` on your PATH, and starts from
there: the folder you extracted can go. Its setup opens on **Start here**.

**On Linux (alpha, in testing):** download **Smolcorp-Linux-x86_64.tar.gz** (or `-arm64`), then
`tar -xzf Smolcorp-Linux-x86_64.tar.gz && ./Smolcorp-Linux-x86_64/install.sh`.

**On a Mac**, three ways, all giving the same app:

### Download the app

1. Download **Smolcorp.dmg** from the
   [latest release](https://github.com/Sauzze/smolcorp/releases/latest) and open it.
2. In the window that opens, drag **Smolcorp** onto the **Applications**
   folder beside it. Then open Smolcorp from your Applications folder, and
   eject the **Smolcorp** disk in Finder's sidebar.

   macOS asks once whether to open an app downloaded from the internet: click
   **Open**. Releases before 1.22.0 weren't notarised and needed **Open
   Anyway** in System Settings → Privacy & Security instead.

The release also has **Smolcorp.zip**, the same app in a zip: open it and
drag **Smolcorp** into your Applications folder.

To check that your download is the one that was published, run
`shasum -a 256 Smolcorp.dmg` (or `Smolcorp.zip`) in Terminal and compare the
result with the SHA-256 in the release notes.

Updating from SauzzeDeck Pro? Quit it, delete **SauzzeDeck Pro** from
Applications and install as above. Your settings and paired phones carry over;
turn the app back on under Accessibility (see [Set up](#set-up)), and add the
deck to your phone's Home Screen again for the new name and icon.

Updating from AgentDeck? Quit it, delete **AgentDeck** from Applications and
install as above. Your settings carry over, but pair your phone again, add it
to your Home Screen again, and turn the app back on under Accessibility.

### Or with Homebrew

```sh
brew install --cask sauzze/tap/smolcorp
```

Update with `brew upgrade --cask smolcorp`.

### Or build it yourself

You need Apple's free developer tools. The installer offers to install them if
you don't have them. A copy you build yourself opens without the warning.

```bash
git clone https://github.com/Sauzze/smolcorp.git
cd smolcorp
./install.sh
```

This builds Smolcorp, puts it in your Applications folder and opens it.
It also replaces an older copy named SauzzeDeck Pro or AgentDeck.

Either way, you'll see a little person in a suit in the menu bar, and the setup page
opens in your browser.

## Set up

Setup opens by itself the first time, on **Start here**: a checklist your
computer runs, each line with the button that puts it right.

<p align="center">
  <img src="docs/setup.png" alt="The setup page: getting started, the desks, and a pairing code with its QR code for your phone" width="720">
</p>

- **Smolcorp is installed**: in Applications on a Mac (opened from
  Downloads or a disk image, **Move to Applications** puts it there), in
  `%LOCALAPPDATA%\Smolcorp` on Windows (it does that itself), in `~/.local`
  on Linux.
- **Your agents report to Smolcorp**: every supported agent on your
  computer tells Smolcorp what it's doing. **Connect them** sets it up, and
  puts right hooks left by another copy of Smolcorp. Restart any sessions
  that were already open.
- **Your phone's keys reach your agents** (Mac): System Settings opens;
  turn on **Smolcorp**. The buttons, the mic and scrolling need this.
- **Your phone can reach this computer**: on your Wi-Fi, and with
  [Tailscale](https://tailscale.com), free, from anywhere.
- **Windows Firewall lets in only your devices** (Windows): **Set the
  rules**, and Windows asks once for your OK.
- **A phone is paired** (see below), and **Smolcorp opens at login**.

**Fix everything** does every line it can at once. Beside them, **Open the
office**, and **Hire your General Manager**: one click starts the GM role in
its own terminal, where it asks you what the goal is.

Further down: the [roles](#run-a-team) you can hire, the office's style,
pets and mic, and under Advanced everything else, from the desks to
[Tell other tools](#tell-other-tools).

The first time you press a button on a Mac, macOS asks whether Smolcorp may
control Terminal. Click **Allow**.

## Pair your phone

1. On your computer, click the Smolcorp icon in the menu bar or tray and
   choose **Pair a Phone…**. The setup page opens with a six-digit code and a
   QR code.
2. Scan the QR code with your phone's camera. Or open your phone's browser, go
   to the address shown with the code (for example `192.168.1.20:7710`) and
   type the code.
3. Add the deck to your Home Screen: on an iPhone, tap **Share → Add to Home
   Screen**; on Android, open the browser's menu and tap **Add to Home
   screen**. Smolcorp now opens full screen and stays paired.

While the deck is open, your phone's screen stays on, from your first tap. If
an agent has been waiting on you for five minutes, the deck reminds you once,
with a chime, and a buzz on phones that allow it.

Each phone you pair appears under **Paired phones** on the setup page, with
when it was last used. **Revoke** cuts one phone off straight away;
**Unpair All Phones** does it for every phone, and is also in Smolcorp's
menu. A phone you've cut off shows the pairing page again. To use it once
more, pair it with a new code, then remove the old Home Screen icon and add
the deck again: the icon keeps the old pairing.

<p align="center">
  <img src="docs/deck-landscape.png" alt="The deck sideways: the office, with the picked agent's keys along the foot of the room" width="640">
</p>

## Make it yours

The setup page also has:

- **Desks.** How many desks for sessions: Auto, one per running session, or
  always 3, 6 or 9. Drag a running session onto a desk to keep that project
  there; desks you don't pin go to the next session that starts.
- **Profiles.** Save your office and your buttons under a name, and switch
  between them in a click: one for the couch, one for the desk. Default is
  always the standard office. Export a profile to back it up or share it.
  Before an imported profile is added, the setup page lists everything its
  buttons would type or press.
- **Office.** Its style, [arranging the room](#the-office), its look and its
  life, your team, the dress code, and **Your buttons**: each types a command,
  such as `/compact`, or presses a key, such as Shift+Tab to switch Claude
  Code's mode, from the keys a held desk brings out. A button can wait until
  the agent isn't busy, ask for a second tap, or be for some agents only. A
  live preview of your phone sits beside it.
- **Logo.** The Smolcorp hire or the original six keys, in the menu bar, on the
  setup page and on your phone's Home Screen. A phone picks up a new logo the
  next time you add the deck to its Home Screen.
- **More:** [Tell other tools](#tell-other-tools), for sending what your agents
  do to Home Assistant or another tool of yours.

<p align="center">
  <img src="docs/layout.png" alt="Profiles and the Office on the setup page: its styles, Arrange the room, and its look, beside a live preview of the phone" width="720">
</p>

Open the setup page any time from the menu bar (**Open Setup…**), or by opening
Smolcorp again from Applications. A bookmark of the page won't open it: its
link carries a key that changes each time Smolcorp starts.

## The office

<p align="center">
  <img src="docs/office.png" alt="The office: nine agents at their desks, dressed for their models from a T-shirt to a suit, one waving for attention as the room's edge glows, interns round a desk, a dog lying by the web-app desk, an arcade machine in the lounge, and the big red button in the corridor" width="800">
</p>

Your sessions are people in a little pixel-art office. Each session is a
desk, numbered as on the setup page, and the person at it acts out what their
session is doing:

| Session | In the office |
|---|---|
| Working | Typing, the screen lit, dots rising from it |
| Waiting on you | Standing up and waving, with a blinking **!**. Kept waiting more than five minutes, they get agitated: hopping, a red **!**, steam from the ears |
| Done, unread | Leaning back with a coffee and a ✓ |
| Idle | Off to the coffee machine, the window, the water cooler, the couch or a game of ping-pong |
| Error | Head in hands, smoke from the screen. A rate limit has them waiting under the clock instead. |
| A new session, an ended one | Someone walks in through the door, or out of it |

There's more to see while they work:

- **Under each desk**, the project's name and what its agent is doing, such as
  *Editing Settings.swift* or *Waiting for you: Bash*.
- **On the monitor**, how long the turn has run. After ten minutes, the agent
  starts to sweat.
- **What they're working with.** Reading files, they leaf through papers; a
  command turns the screen terminal-green; the web spins a globe on the desk;
  a to-do list sends them to the whiteboard. Compacting the conversation, they
  feed it into a shredder.
- **Interns**, one for each of Claude Code's subagents, gather round the desk
  while they work: up to four, then a "+n".
- **What they wear** is the size of their model, the bigger the smarter: small
  models such as Haiku, GPT mini or Gemini Flash-Lite dress casual, in a
  T-shirt and jeans; medium ones such as Sonnet wear a shirt; large ones such
  as Opus or Gemini Pro a jacket; and the top ones, such as Fable, a suit and
  tie. Interns dress for their own model. While an agent's model isn't known,
  as for most agents but Claude Code, it wears its agent's colour.
- **Coffee at the desk** is Claude Code's effort: a stack of mugs by the
  monitor, one for low up to four, swaying, for extra high, and at max a
  steaming moka pot, with steam coming off the agent. Press ☢ Max, and the
  agent runs to the big red button in the corridor and slams it.
- When few sessions are running, the office's own staff hang around the lounge.

**Furnish it your way** under *Office* on the setup page:

- **A style** furnishes the whole office in one go: Classic, Campsite, Royal
  court, Space station, Pirate cove, Arctic base, Dragon's lair, Gamer den,
  Startup garage, Fitness floor, Jungle, Neon city, Submarine, Lo-fi café, Wild
  West saloon, Zen garden or Treehouse. Most bring a pet of their own: a
  fox at the Campsite, a corgi at the Royal court, a robot dog on the Space
  station, a parrot in the Pirate cove, a penguin at the Arctic base, an owl in
  the Dragon's lair, a hamster in a ball in the Gamer den, a monkey in the
  Jungle, a hover drone in Neon city, a cat in the Submarine and the Lo-fi café,
  a tumbleweed in the saloon, a snail in the Zen garden and a red panda in the
  Treehouse. Switch it from your phone too: tap the key in the room's top-left
  corner and pick one. **Yours** is the office as you furnish it below, and
  changing anything there but the pets takes you back to it. Your team, the
  daylight, the seasons and the sound stay as you set them, and so do the pets
  you pick, in place of the style's own.
- **The floor** in wood, tiles, concrete or carpet, and more, and **the walls**
  in slate, sage, clay or cream, and more.
- **In the room editor** (below): the mic you talk into, from a microphone to a
  megaphone, what hangs on the back wall, a window, a whiteboard, a pinboard,
  a poster and dozens more, with the to-do lists going up on the first
  whiteboard, the lounge's own floor and wall, and its corner piece, one of
  the lounge's pieces, each picked from its picture: an arcade
  machine, a vending machine, a foosball table, a fish tank, a bookcase and a
  beanbag, a dragon asleep on its gold, pinball, a jukebox, a pool table, a
  piano, a drum kit, a telescope, jellyfish, a bonsai, a jungle, a coffee bar,
  servers, a 3D printer, a punching bag, a treadmill, a hot tub, a ball pit or a
  campfire. Idle agents and staff go and play, grab a snack, or sit by the fire,
  and the pets join in.
- **Where everyone works:** at a desk, or a standing desk, a gaming desk, a
  tent, a cardboard box, a beanbag, an igloo, a camper van, a spaceship, a
  pirate ship, a throne or a greenhouse. Each project can have its own on its
  team card, and it goes with them when they swap desks.
- **Daylight** that follows the clock, with night falling at 8 pm, or always day.
- **Seasons in the windows:** snow in winter, with lights strung along the wall
  through December, falling leaves in autumn, blossom in spring, birds in summer.
- **Up to three pets:** a cat that naps on the couch, a dog that follows
  people about, a lizard that basks on the wall, a monkey that climbs on things
  and steals cups, or a llama, a penguin, a parrot, an owl, a capybara, a red
  panda, a corgi, a hamster in a ball, a robot dog, a bat, a snail, a fox, an
  axolotl, a black cat, a goat, a raven, a little ghost, a spider on a thread, a
  reindeer, a little snowman, a snow hare, a little cupid, lovebirds, a hover
  drone or a tumbleweed. Pick them in any style, and they take the place of its
  own.
- **In the air:** nothing, snow, floating hearts, a dragon dance or confetti,
  and **in the windows**, fireworks at night.
- **The seasons' pieces**, each its own choice in the room editor: Halloween,
  Christmas, winter, Saint Valentine, Lunar New Year, New Year's Eve and a
  sport season, a basketball court and a broadcast set, with outfits to match.
- **Sound:** off, quiet or normal, in the style of an old games console, and
  mostly silence: a burst of typing as an agent gets to work, the door, the
  coffee machine, a ping-pong rally, the red button's siren, a hire, a firing,
  the pets, the wind in winter, and now and then the lounge's piece while
  someone's at it. The one sound that asks for you is the reminder after five
  minutes of waiting. It starts with your first tap, as browsers require.
  Under it, softly, real keyboards while agents work, each at its own pace, and
  a muffled chat at a desk while a message is taken there; turn that off under
  *Typing and chatter*. The recordings are public domain (CC0), from Freesound.
- **The desks** in wood, walnut, white or black, and **the chairs** in teal,
  black, red, grey or green.
- **Office staff:** a few, none, or lots of them in the lounge.
- **A calm office**, under *Life*: nobody wanders off, the staff and pets stay
  put and nothing drifts by the windows, so the office moves only for the work,
  and draws less often, for less power. Agents still work, type, come and go.
- **A lite office on your phone**, on by default: it goes as a calm office
  does, at 30 frames a second, so the phone stays cool. It's that phone's own
  choice, and changes nothing on your other screens. To turn it off, tap the
  paint roller in the room's corner, then the switch at the foot of *Office
  style*.
- **A jukebox that plays your music**, once you turn it on under *Getting
  started*: tap any jukebox in the office to play or pause whatever this
  computer is playing, Spotify, Apple Music or a video in your browser. It
  lights up while Spotify or Apple Music plays; macOS asks once whether
  Smolcorp may ask them. Hold a jukebox for its controls: back, play or
  pause, next, and a bar for this computer's volume.

**Arrange the room** puts everything where you want it. Click **Arrange the
room…** under *Office*, and the office fills the window: click a piece to pick
it and drag it anywhere, or nudge it with the arrow keys. The lounge's pieces,
the couch, the ping-pong table, the plant, the coffee counter, the red button
and the mic grow by the handle on their corner: small, medium or large. Pick a
desk to make it what you like, a tent, a beanbag or a cardboard box, whoever
sits at it; a project's own choice on its team card still comes first.

**The drawer** beside the room holds whatever isn't in it: desks and furniture
put away, any piece the lounge can hold, up to twelve and the same one more
than once, and everything the back wall can hang, up to twelve, wherever you
slide them along it. Drag a piece into the room, or click it to put it where
it fits. To put one away, drag it onto the trash that shows below the room
while you carry it, or back to the drawer. A session whose
desk is put away waits in line along the back wall. **Start empty** leaves the
floor, the walls, the door, the mic and the red button; **Back to standard**
puts everything back; **Undo** and **Redo** (⌘Z, ⇧⌘Z) step through your
changes. The room on its side and upright are arranged on their own, and
**Copy from upright** or **Copy from sideways** starts one from the other.
People walk round whatever is in their way, and a desk nobody can get to from
the door is outlined in red. Your phone follows as you go.

**Your team**, under the furnishing, has a card for each of your projects:
choose its agent's skin, hair and hairstyle, something extra (glasses,
sunglasses, headphones, a cap, a beanie, a beard, a moustache or a bow), an
outfit and its colour, what's on their desk, where they work, and a nickname
for their desk's label. Anything left on Auto stays as the office picks it, and clothes on Auto
follow the model.

**The dress code**, under the team, chooses the outfit and colour for each size
of model, and for each agent while its model isn't known. Add your own rules
for models the office doesn't know, such as "a model whose name contains
`qwen-max` is large". Thirteen outfits to choose from, there and on the team
cards, from a T-shirt to a lab coat, overalls or a Hawaiian shirt.

It's all kept with the layout, so each profile can have an office of its own.

**On your phone**, the office is the deck: see [On your phone](#on-your-phone).
Two more things work there:

| In the office | What it does |
|---|---|
| An agent's desk, held, then dragged | Picks the agent up, chair and all. Let go over another desk and the two swap. They stay that way: each project is pinned to its new desk, as on the setup page's *Desks*, where you can undo it. |
| A desk, held | With hiring and firing on, also **YOU'RE FIRED**, or the roles to hire on a desk a new hire would take (see below). |

**Hire and fire**, once you turn on **Let the office hire and fire** on the
setup page (it's off until you do):

- **Hold an agent's desk**, and a red **YOU'RE FIRED** comes out above its
  chips. Tap it, confirm, and its work stops and its Terminal tab closes. The
  agent packs a box and walks out. Claude Code keeps the conversation, so
  `claude --resume` can bring it back.
- **Hold the desk a new hire would take**, the next free one, and the roles
  come out along the foot of the room: **Anyone**, then each role with its
  badge. Tap one, and a new Terminal window opens in the folder you chose on
  the setup page. Anyone is a regular hire, with the agent you chose; a role
  starts its own agent, at its model and effort, on its instructions. The new
  hire walks in through the door with a coffee or a box. On a pinned project's
  empty desk, the hire starts in that project instead.
- **Comings and goings**, on the setup page: as above, with the coffee, the box
  and the packing up, or **Plain**, where people just walk in and out.

**Hire by role.** A role is a name, an agent, a model, an effort, a badge and
instructions, set under *Roles* on the setup page. Nineteen come with
Smolcorp, each reporting what it finds and asking before it changes anything:

- **Reviewer** (glasses and a red pen): reviews the changes you point it to,
  most serious problems first.
- **Tester** (a clipboard): runs the tests you pick and explains each failure.
- **Bug hunter** (a magnifying glass, dressed as a detective): looks for bugs
  where you point it, with how to reproduce each one.
- **Docs writer** (a quill and notebook, dressed as a librarian): checks the
  docs you pick against the code.
- **Refactorer** (a wrench, in a hard hat): proposes one refactor at a time,
  within the limits you give.
- And fourteen more, each dressed for the job: an **Art Director**, a
  **Lawyer** (who says it isn't legal advice), a **Therapist** for when you're
  stuck, a **Plumber** for pipelines, a **General Manager** to run a team of
  agents (see [Run a team](#run-a-team)), a **Janitor** for clean-ups, a
  **Firefighter** for outages, a **Pilot** for releases, a **Postman** for
  notifications and webhooks, an **Accountant**, a **Marketer**, a **Security
  guard**, a **Chef** for scaffolds and prototypes, and a **Thinker**, who
  thinks a question through and writes it up.

Setup's brief library also has briefs to start a role of your own from: a
**Builder**, a **Designer**, a **Pixel artist**, a **Planner**, a **Fixer**, a
**Study**, and **Security** and **Performance auditors**. On the Mac, each role
starts from its brief, written into its folder; on Windows and Linux, from its
instructions.

New roles that come with an update are added once, at the end of yours; one
you've deleted isn't brought back. Change any of them, put them in your order,
or add as many of your own as you like.
A role never assumes what to work on. Its first reply only asks you, all
together, which project to work on (at a pinned project's desk, whether it's
that one) and its own questions, such as the Reviewer's branch, base and what
matters most. It's told to read nothing and run nothing until you've
answered, then it gets to work. Each role's questions are yours to change, one per line. Its desk shows the role,
as in *Reviewer · api-server*, and its badge, and it keeps the role after
`/clear`. Roles stay on your computer, out of profiles: the phone only knows
their names and badges.

On Windows and Linux, firing closes the agent's pane in tmux, kitty and
WezTerm, and stops an agent started with `smolcorp run`. A hire opens a window
of the terminal you choose on the setup page (GNOME Terminal, Konsole, kitty,
WezTerm and others on Linux; Windows Terminal or a console window on Windows),
running the agent through `smolcorp run`, so its keys work from the phone. On
Linux it can run in a tmux session of its own instead; on a computer with no
desktop, as over SSH, it always does, for `tmux attach` to show it.
`smolcorp run --role tester`, in the folder to work in, also starts a role from
any terminal, and its desk shows the role.

**Held upright,** the room stands upright too: the lounge moves below the
desks, through a doorway at the end of the corridor.

**On a spare screen**, such as an old tablet, a TV or a second monitor, open the
address your phone uses with `/office` at the end. It pairs like a phone, then
shows the office alone, full screen, and can be tapped too.

**On top of your other windows:** on the computer running Smolcorp, open
`http://localhost:7710/office` in Chrome or Edge and click **POP OUT** in its
corner. The office moves into a small window that stays above everything else,
and can still be clicked. Other browsers, and the address your phone uses,
don't offer it.

**As your Mac's screen saver:** click the Smolcorp icon in the menu bar
and choose **Install Screen Saver…**. System Settings opens at Screen Saver:
choose **Smolcorp Office**, click **Options…**, and enter a code from
**Pair a Phone…**. It's listed under Paired phones as **Screen saver**. While
your Mac is idle, each display shows the office, just to watch: a key or a click
ends the screen saver as usual. When Smolcorp isn't running, or the screen
saver was unpaired, it says what to do instead. After updating Smolcorp,
choose **Install Screen Saver…** again.

On your phone, on a spare screen and in the pop-out window, the office fills a
rounded window; the screen saver fills each display edge to edge. Either way,
its edge glows while an agent is waiting on you.

## Voice

The mic button uses Claude Code's own voice input (`/voice`), in *tap* mode.
Connecting Claude Code sets that up, unless you had already set voice up
yourself. In that case, make sure `~/.claude/settings.json` contains:

```json
"voice": { "enabled": true, "mode": "tap" }
```

Voice input needs Claude Code signed in with a Claude.ai account. Claude Code
sends your audio to Anthropic to be transcribed. Smolcorp itself never
hears it.

**Clean up dictation with Claude** (Mac, setup → Phone and voice; off until
you turn it on): with the Mac's own dictation on, the dictated *text*, never
the audio, goes to Claude Haiku before it's typed, with
[your own API key](#summaries-with-claude): fillers and stutters go, and
misheard names come back as the glossary beside it spells them. If that
fails, there's no key, or it takes over 2.5 s, the text goes in as heard.

## Portable office (beta)

See the office on your phone from anywhere, over 4G, while you're away. It's
view only: nothing on the phone can press a key or type into a session. It's
off until you launch it, and off again each time Smolcorp starts.

While it's on, Smolcorp locks a trimmed-down copy of the office on your Mac
(AES-256-GCM) and puts it on a small viewer you run on your own Vercel. The
key is only in your Mac's Keychain and in your link, after the `#`, which
browsers never send to any server, so the viewer only ever holds locked data.

### Set it up on Vercel

1. On Vercel, make a project from your fork of this repository. Set its Root
   Directory to `viewer/`, its Framework to "Other", and leave the build
   command empty.
2. Add **Upstash Redis** to it from the Vercel Marketplace (the free plan is
   enough). It adds `KV_REST_API_URL` and `KV_REST_API_TOKEN` for you.
3. On Smolcorp's setup page, under **Viewer link**, press **Make a write
   secret** and copy it. It's shown once. In the Vercel project, add it as
   the environment variable `VIEWER_WRITE_SECRET` (Production).
4. Deploy to production. Paste the production address
   (`https://office-viewer-xxxx.vercel.app`) into **Viewer address**, press
   **Save**, then **Check the viewer**. It says **Viewer ready ✓** or what's
   wrong.
5. Scan the QR code with your phone and add the page to your Home Screen.

A custom domain works too. Vercel Authentication doesn't, since the phone has
no Vercel login. To keep strangers from using up your free Upstash allowance,
add the firewall rule in [viewer/README.md](viewer/README.md), which also has
the relay's protocol, if you'd rather host the viewer elsewhere.

### Start and stop it

- **Start:** menu bar → **Going away: launch portable office**. The phone shows
  the office within a few seconds. While it's on, the menu bar icon has a small
  dot, and the menu says **Portable office: ON** with when it last sent.
- **Stop:** menu bar → **Stop portable office**, or quit Smolcorp. The viewer
  forgets the office straight away, and the phone says "Not active". It also
  stops by itself when you're back at your Mac (you unlock it, or wake it and
  use it, but not in the first two minutes, so you can launch it and then
  lock), and after 8 hours. Both can be changed on the setup page.
- **Lost your phone, or shared the link by mistake:** setup → **Rotate key…**.
  Every old link stops working for good; scan the new QR code. The link holds
  the key, so keep it to yourself: **Copy viewer link** in the menu copies it
  for this Mac only, never to your other devices.
- **Wipe the viewer** empties it of everything it holds.

### What's sent, and what never is

Sent, locked: each desk's state (working, waiting, done, failed), its agent and
effort, whether it's compacting, its subagents' kinds, its role's name, badge
and outfit, the theme, and how the room is furnished. Project names too,
unless you turn on **Hide project names**, which shows "Desk 1", "Desk 2"…
instead, upstairs too, where a run's hirer becomes the hiring desk's number.

Never: transcripts, prompts, what an agent or you wrote, dictation,
file names, folders and paths, commands, models, your buttons, session ids,
your Mac's name, addresses, phone ids, and any key or token. Your write
secret, the viewer's id and its key are kept in the Keychain, never in
`config.json`.

The viewer sees when the office is on and roughly how big it is, nothing
else.

### On Windows and Linux

The portable office works the same way, from the tray: **Going away: launch
portable office** and **Stop portable office**. The write secret, the viewer's
id and its key are kept in Credential Manager on Windows, or in the Secret
Service (GNOME Keyring or KWallet) on Linux; a Linux computer with no secret
store can't keep them, so it can't use the portable office. There's no **Copy
viewer link** in the tray: copy it from the setup page.

**Stop when I'm back** works where the system says you're back: on Linux, the
screen unlocking after it was locked, or the keyboard or mouse used after a
while away, where your desktop tells logind (GNOME and KDE do); on Windows,
the keyboard or mouse used after ten minutes without. Elsewhere,
it stops after the hours you set, or when you stop it.

## Summaries with Claude

Away from the computer, a desk on the phone shows *what* an agent is doing,
not why. Hold a desk and tap **✨ summary**: Claude Haiku 5.5 reads the end of
that agent's Claude Code transcript and answers in a few lines what you asked,
what it's doing, whether it's waiting on you, and its last result.

It's off until you turn it on, and it uses **your own Anthropic API key**,
billed to you by Anthropic. Haiku is Anthropic's least expensive model, and a
summary is one short request.

### Turn it on

1. On the setup page, under *Getting started*, paste your API key into
   **Summaries with Claude** and click **Save key**. Make a key in the
   [Anthropic Console](https://console.anthropic.com/settings/keys).
2. Turn the switch on. A dialog says what's sent and where; it turns on only
   when you click **Turn on**.

Turn it off with the same switch, at any time. **Remove** forgets the key.

### What's sent, and where your key is kept

- **Sent, to Anthropic's API, only when you tap ✨ summary:** the end of that
  agent's transcript, at most 16,000 characters. That's:
  - your recent requests and its replies, each cut short;
  - each tool it ran, with the one file path, command or search that says
    what it did;
  - each tool's output, cut to its first line. For a file it read, that's the
    file's first line.

  Lines that look like they hold a secret (keys, tokens, passwords, private
  keys, passwords in URLs) are taken out first. The project's folder name is
  sent with it.
- **Your key** is kept by your system:
  - in the Keychain on a Mac;
  - in Credential Manager on Windows;
  - in the Secret Service (GNOME Keyring or KWallet) on Linux.

  It's never in `config.json`, the phone, the portable office or a log, and the
  setup page shows only its last four characters.
- **Limits:** a summary is kept for 30 seconds, so a second tap doesn't pay
  twice. At most 20 summaries go to Anthropic in any 10 minutes. A failed one
  is never retried by itself.

### Or, as a fallback, `ANTHROPIC_API_KEY`

**Paste your key on the setup page if you can:** it's then kept in your
system's secret store, and only Smolcorp reads it. With no key saved there,
Smolcorp uses `ANTHROPIC_API_KEY` from its own environment instead. On a Linux
computer with no secret store, this is the only way.

The ways below give the key to **every app you start** after setting it, not
just Smolcorp. On Windows and Linux they also keep it in a plain text setting
or file, so use them only where that's acceptable. An app you open from the
Dock, the Start menu or at login doesn't see what you `export` in a terminal,
so:

- **Mac:** run `launchctl setenv ANTHROPIC_API_KEY sk-ant-…`, then quit and
  reopen Smolcorp. It lasts until you restart the Mac.
- **Windows:** run `setx ANTHROPIC_API_KEY sk-ant-…` in a terminal, then
  restart Smolcorp. It's kept in your user's environment settings.
- **Linux:** add `ANTHROPIC_API_KEY=sk-ant-…` to
  `~/.config/environment.d/smolcorp.conf`, then log out and back in. It's
  kept in that file.

Smolcorp never reads a project's `.env` file, or any other file, for a key.

## Tell other tools

Smolcorp can tell another tool what your agents are doing, so a lamp
turns orange when one waits on you, or your phone buzzes when a long task is
done. Turn on **Tell other tools** under **More** on the setup page, give it an address, and
pick what to send:

| Event | When |
|---|---|
| `waiting` | An agent is waiting on you: a permission prompt or a question |
| `done` | An agent finished, and you haven't looked yet |
| `error` | A turn ended on an error, such as a rate limit |
| `working` | An agent got to work |
| `started` | A session started |
| `ended` | A session ended |

Waiting on you, done and failed are on to begin with. Each event is a `POST`
with a JSON body:

```json
{
  "event": "waiting",
  "at": "2026-10-17T20:41:07Z",
  "session": {
    "project": "api-server",
    "agent": "claude",
    "agentName": "Claude Code",
    "state": "asking",
    "previous": "thinking",
    "key": 1
  }
}
```

`key` is the number of the desk the session is at, as the setup page numbers
them from 1, or `null` if it has none. An
`error` also has an `errorKind`, as the agent named it, such as `rate_limit`
or `overloaded`. **Send a test** sends `"event": "test"` with no session. If
you set a secret, every message carries it in an `X-Smolcorp-Secret` header.

Only addresses on your own network are allowed: your computer, a `.local`
name, a name without dots or a private IP address. Tick **Allow addresses
outside your network** to send elsewhere. Messages name the project, never its
folder, and never carry prompts or anything an agent wrote.

### Home Assistant

Add an automation with a webhook trigger, then give Smolcorp the
webhook's address, such as
`http://homeassistant.local:8123/api/webhook/smolcorp-lamp`. Home Assistant
doesn't read headers, so keep the webhook id hard to guess and leave the
secret empty.

```yaml
automation:
  - alias: Desk lamp follows the agents
    triggers:
      - trigger: webhook
        webhook_id: smolcorp-lamp
        allowed_methods: [POST]
        local_only: true
    actions:
      - choose:
          - conditions: "{{ trigger.json.event == 'waiting' }}"
            sequence:
              - action: light.turn_on
                target: { entity_id: light.desk_lamp }
                data: { color_name: orange }
          - conditions: "{{ trigger.json.event == 'done' }}"
            sequence:
              - action: light.turn_on
                target: { entity_id: light.desk_lamp }
                data: { color_name: green }
          - conditions: "{{ trigger.json.event == 'error' }}"
            sequence:
              - action: light.turn_on
                target: { entity_id: light.desk_lamp }
                data: { color_name: red }
```

## Privacy and security

- **Smolcorp sends nothing off your network unless you turn it on.** It talks
  to your phone and to the coding agents on your computer. Only these, each off
  until you turn it on, go further:
  - the address you gave [Tell other tools](#tell-other-tools), which is on
    your network unless you allow otherwise;
  - the [portable office](#portable-office-beta)'s viewer, on your own Vercel,
    which only ever holds locked data;
  - Anthropic's API, with your own key, when you ask for a
    [summary](#summaries-with-claude).

  It has no accounts, no analytics and no cloud service of its own.
- **Each phone has its own key.** Every request carries a long random key that
  only that phone has. The key is never typed: pairing hands it over. Your
  computer keeps only a fingerprint of each key (a SHA-256 hash), so you can revoke one
  phone without touching the others.
- **Buttons you make type only what you set up.** A phone names a button by
  its id; the command it types, or the key it presses, comes from your
  computer. A paired phone can't send text of its own.
- **Pairing codes are short-lived.** A code exists only after you ask for one.
  It works once and expires after five minutes. Five wrong tries from one
  device lock that device out until you get a new code, and twenty wrong tries
  in all withdraw the code. The QR code holds the same one-time code, never a
  key, and disappears once the code is used.
- **The agents have a key of their own.** Their hook uses a separate
  key, read from a file only you can read, so it never shows up in the list of
  running processes. It works only from your computer, and only for reporting what
  an agent is doing. A phone's key can't pose as an agent, and the agents' key
  can't drive the deck.
- **Scrolling, hiring and firing are off until you turn them on.** Out of the
  box, a paired phone can only use the office's keys, effort chips and
  choices, and move desks around, which pins projects to desks as the setup
  page does. Even turned on, firing asks first, and only stops the agent
  itself; hiring only starts the agent you chose, or a role's, in the folder you
  chose or in a pinned project's. A role's instructions are the agent's first
  message, there to read in its Terminal window, never a command, and a role
  changes nothing about what its agent may do without asking.
- **The screen saver pairs like a phone,** and keeps its key in its own
  settings, as a phone's browser does.
- **The office says what your agents are doing in a few words.** They're made
  from a file's name, a web page's site or the description the agent gave a
  step, never from a command or a file's path.
- **Smolcorp listens on port 7710 on your network,** so your phone can
  reach it. Without a key, all another device gets is the pairing page. Its
  pages can't be shown inside other websites, and it turns away connections
  beyond a fixed limit, so a misbehaving device can't swamp it.
- **The setup page only opens on your computer, from Smolcorp's menu.** Other
  devices can't reach it, and other websites can't press its buttons. The menu
  opens it with a key that's new each time Smolcorp starts, so other programs
  on your computer, and other people signed in to it, can't use it either.
  Your own scripts can: the key is in a file only you can read, sent as the
  `X-Smolcorp-Setup-Key` header (see [Use setup from a script](#use-setup-from-a-script)).
- **Use a network you trust.** The connection is plain HTTP on your local
  network. Use Smolcorp on home or office Wi-Fi, not public hotspots.

### Use setup from a script

Everything the setup page does is an HTTP call to Smolcorp on this computer,
and your own scripts can make the same calls. Each needs the setup key, which
is new each time Smolcorp starts and kept in a file only you can read:

| | The setup key |
|---|---|
| Mac | `~/Library/Application Support/AgentDeck/setup-key` |
| Linux | `~/.config/sauzzedeck-pro/setup-key` |
| Windows | `%APPDATA%\SauzzeDeck Pro\setup-key` |

Send it as `X-Smolcorp-Setup-Key`. Calls that change something also need
`X-AgentDeck-Setup: 1`. For example, on a Mac:

```bash
key=$(cat ~/Library/Application\ Support/AgentDeck/setup-key)
curl -s -H "X-Smolcorp-Setup-Key: $key" http://localhost:7710/setup/state
curl -s -X POST -H "X-Smolcorp-Setup-Key: $key" -H "X-AgentDeck-Setup: 1" \
  http://localhost:7710/setup/pair
```

Read the key again after Smolcorp restarts.

## Troubleshooting

**The desks stay empty.** Click **Connect** on the setup page, then restart your
agent sessions. On Windows and Linux, start them with `smolcorp run`, or in
tmux, WezTerm or kitty.

**A desk has no controls.** Its session runs in a terminal Smolcorp can only
watch (see [Terminals](#terminals)), or in Ghostty 1.3. In kitty, check the
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
**Open Settings** on the setup page again afterwards, and turn Smolcorp
back on.

**Two fingers don't scroll.** Turn on **Let two fingers scroll Terminal** on
the setup page.

**The phone shows the pairing page instead of the deck.** It was revoked, or
it was paired with AgentDeck 1.0. Pair it again, then add it to your Home
Screen again: see [Pair your phone](#pair-your-phone).

**The mic types a space instead of listening.** Claude's input box has to be
empty, and voice has to be in tap mode: see [Voice](#voice).

**The phone can't connect.** Check that both devices are on the same Wi-Fi.
If macOS or Windows Firewall asked whether Smolcorp may accept incoming
connections, allow it. On Windows the server answers only on this PC and on
Tailscale's address, so the phone must be on Tailscale too: see
[Who can reach Smolcorp](#who-can-reach-smolcorp-windows).

**The roles, or HIRE, don't come out.** Turn on **Let the office hire and fire**
on the setup page, and choose a folder for new hires. They show only on a desk
a new hire would take: the next free one, or a pinned project's. With no roles
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
quit Smolcorp from its tray menu, then delete:

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

Connecting left a backup of each settings file it changed next to it, ending
in `.agentdeck.bak`: Claude Code's `~/.claude/settings.json`, Codex's
`~/.codex/hooks.json` and Gemini CLI's `~/.gemini/settings.json`. Delete them
once you no longer need them. Connecting Claude Code also turned on its voice
input, if it was off. Disconnecting leaves voice as it is: remove the `voice`
entry from `~/.claude/settings.json` if you don't want it.

## How it works

Each agent runs a small script ([`Resources/report.sh`](Resources/report.sh))
at each step of a session, through its hooks: Claude Code's
[hooks](https://code.claude.com/docs/en/hooks), and the matching settings in
Codex, Gemini CLI and Grok. OpenCode has no command hooks, so a small plugin
([`Resources/opencode-plugin.js`](Resources/opencode-plugin.js)) runs the same
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
  effort and first message, each quoted for the shell. **Firing** stops the agent's
  process in its tab, only once it's sure that's the agent, then closes the
  tab.

The Mac app is plain Swift with no dependencies. The Windows and Linux app is
Go, with a handful: a tray icon, pseudo-terminals, QR codes. The phone side is
a single web page, shared by both.

```
Sources/AgentDeck/   the Mac app
ScreenSaver/         the Mac's screen saver, which shows the office
desktop/             the Windows and Linux app, file for file where they match
cmd/smolcorp/        its command: the app, the launcher and the hook
Resources/web/       the phone page, the office, the pairing page and the setup page, shared
Resources/           the Mac's hook script, and the OpenCode plugin
install.sh           build and install on a Mac
packaging/linux/     the Linux installer
scripts/             the builds, and the release downloads
docs/                screenshots for this page
```

## Windows and Linux

Smolcorp runs on Windows and Linux too, with the same phone deck, the same
pairing, the same agents, roles and hiring. What's different from the Mac:

- **Start agents through Smolcorp**, in any terminal:
  `smolcorp run claude` (on Windows, `smolcorp-cli run claude`), or `codex`,
  `gemini`, `opencode` or `grok`, or `smolcorp run --role tester` for a role.
  The agent runs exactly as usual, and the deck can press its keys, with no
  permission to grant. Windows and Linux have no equivalent of Terminal's
  scripting, so this is how the deck reaches a session. A hire from the office
  opens this way, in Windows Terminal or a terminal the setup page lists.
- **In tmux, WezTerm or kitty**, agents started the usual way work too: the
  deck sends keys through the terminal's own commands. kitty needs its remote
  control turned on (`allow_remote_control yes` and `listen_on` in
  `kitty.conf`).
- **Tapping a desk** brings its pane forward in tmux, WezTerm and kitty. In the
  launcher it only selects the session, because the launcher can't tell which
  window it's in.
- **Two-finger scrolling** works in tmux and kitty.
- **Show Office**, in the tray menu: on Windows, the office in a window of its
  own; on Linux, in your browser, where Chrome and Edge can **POP OUT** it on
  top of your other windows.
- **No Accessibility step.** Nothing here needs that permission.
- **Grok Bots** sit at their desks too, but their news comes only through
  `POST /bots/<id>/event`: the Grok Bot app that the Mac reads is the Mac's
  only.

### Windows: the office in its own window

**Show Office**, in the tray menu, opens the office in a window titled
Smolcorp, with its icon. It uses WebView2, the Edge engine that comes with
Windows 10 and 11. If it's missing, a message says so and the office opens in
your browser instead.

- A second click on **Show Office** brings the open window to the front, and
  restores it if minimised. There is never a second window.
- **Always on Top** keeps the window above the others.
- Size and position are remembered (`window` in `config.json`); if the screen
  changed and the place is out of reach, it opens centred.
- The window pairs itself, as a device named "Smolcorp (window)", its key in
  `window-key` beside `hook-key`. Phones still pair with the six-digit code,
  and **Unpair All Phones** leaves the window's device alone.

### Who can reach Smolcorp (Windows)

On Windows the server listens on this computer (`127.0.0.1` and `::1`: the
setup page, the hooks, the launcher and the office window are all local) **and
on the address Tailscale gave this computer** (`100.64.0.0/10`, on the adapter
named *Tailscale*). The home network, a Wi-Fi hotspot or anything else cannot open
port 7710 at all. If Tailscale starts after Smolcorp, Smolcorp joins its address
within about 15 seconds, with no restart; if it stops, the address is let go.

- The phone's pairing address (the QR code and the address on the setup page) is
  the Tailscale one. With no Tailscale on this computer it reads `localhost`, which a
  phone cannot open: Start here says to get Tailscale, or to let in your home Wi-Fi.
- Every other protection is unchanged: the 6-digit pairing code, the check of the
  host name, the connection limits, and the setup page, hooks and launcher answering
  only to this computer.
- **Windows Firewall** matches it: until setup's **Set the rules** has run,
  with your OK, Smolcorp doesn't open Tailscale's address at all, so Windows
  never asks and never makes rules of its own. The rules let TCP in from
  `100.64.0.0/10` only, and remove any older rule for a copy of Smolcorp, as
  one in `Downloads`, or one Windows made that let anyone in on a public
  network.
- **Also on my home Wi-Fi**, on Start here, lets phones on your home network
  in too: it sets `"listen": "all"` in `config.json`, adds a firewall rule for
  your local network on private networks only, and starts Smolcorp again.
  By hand, `"listen": "all"` in `config.json` (in `%APPDATA%\SauzzeDeck Pro`)
  does the same once Smolcorp starts again, without the firewall's rule;
  `"listen": "tailscale"` asks for the limited behaviour on Linux too, where
  the default is every interface, as on the Mac.
- `SMOLCORP_PORT` (1024-65535) replaces port 7710, to try a build next to the one
  you use. It is for tests: the hooks and the launcher read it too, so set it for
  everything that should talk to that copy.

### One Smolcorp at a time

Starting `Smolcorp.exe` while Smolcorp is already running does not start another
tray icon. The second launch asks the running one to open the office window (or
bring it to the front) and exits at once with code 0. If the running one is an
older Smolcorp that cannot be asked, a message says that Smolcorp is already
running. On Windows the lock is a named mutex of the session
(`Local\Smolcorp-single-instance`); on Linux, a lock on `instance.lock` next to
the keys. If the port is taken by something else, Smolcorp says so and exits,
rather than stay in the tray without a server.

### Linux

Download **Smolcorp-Linux-x86_64.tar.gz** (or `-arm64`) from the
[latest release](https://github.com/Sauzze/smolcorp/releases/latest), then:

```bash
tar -xzf Smolcorp-Linux-x86_64.tar.gz
./Smolcorp-Linux-x86_64/install.sh
```

This installs it for you alone, in `~/.local`, with an entry in your app
menu. Open **Smolcorp** from the menu, or run `smolcorp`. The setup
page opens in your browser.

- On GNOME, the tray icon needs the AppIndicator extension, which Ubuntu
  includes. Without it, Smolcorp still runs: open
  `http://localhost:7710/setup#k=` followed by the key in
  `~/.config/sauzzedeck-pro/setup-key`.
- On a machine without a desktop, such as one you reach over SSH, run
  `smolcorp serve`. It prints the setup page's link as it starts. Open it
  through a tunnel: `ssh -L 7710:localhost:7710` to the machine, then the
  link in your own browser.

### Windows

Download **Smolcorp-Windows-x86_64.zip** (or `-arm64` for an ARM PC), extract
it and open **Smolcorp.exe** (see [Install](#install)). It installs itself in
`%LOCALAPPDATA%\Smolcorp` and starts from there, so your agents' hooks and the
firewall's rules never name the folder you downloaded to. Opening a newer
download later updates it the same way: quit Smolcorp from its tray icon
first.

### Build it yourself

You need [Go](https://go.dev) 1.26 or later:

```bash
go build ./cmd/smolcorp
```

On Windows, that one program runs everything, in a console window of its
own. To build it as the downloads have it, with no console window, build
both:

```bash
go build -ldflags "-H windowsgui" -o Smolcorp.exe ./cmd/smolcorp
go build -o smolcorp-cli.exe ./cmd/smolcorp
```

`go test ./...` runs the tests, and `scripts/release-desktop.sh` builds the
downloads for both systems.

The version shown by `smolcorp-cli version` is `dev` unless the build sets it:

```bash
go build -ldflags "-X main.version=1.2.3" -o smolcorp-cli ./cmd/smolcorp
```

(on Windows, add `-s -w -H windowsgui` for `Smolcorp.exe`, as above;
`scripts/release-desktop.sh` passes its own version.)

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
