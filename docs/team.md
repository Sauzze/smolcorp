# Your team

Hiring by role, the General Manager, the fence, the agents Smolcorp works with, and the terminals it drives. Back to the [README](../README.md).

## Run a team

It's not only one agent at a time: it's a small company.
Hold an empty desk to hire a Reviewer, a Tester, a Bug hunter or a role of your
own, and put a manager at the middle desk to brief the others and check on
their work, while you watch the whole team from your phone.

The office has nine desks on the ground floor and five floors above it, each
with nine desks and a chill area of its own, so you can run it like a small
company of dozens of agents.

**Hire by role.** Hold any free desk, and pick who to hire:
a **Reviewer**, a **Tester**, a **Bug hunter**, a **Docs writer**, a
**Refactorer**, fourteen more from an Art Director to a Thinker, or any role
of your own, as many as you like. Each role has its own instructions, agent, model and
effort, and comes in wearing its badge, like the Reviewer's glasses and red pen,
and many dress for the job. A role never guesses what to work on: its first
reply asks you which project, and its own questions, such as which branch to
review. On Windows and Linux, `smolcorp run --role reviewer` also starts one
from any terminal.
[More about roles](office.md#the-office).

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

Each hire works on a branch of its own, in its own git worktree. The GM can
pin a project to a desk and give a person a nickname: `smolcorp gm pin`,
`smolcorp gm unpin --desk N` and `smolcorp gm nickname <name>`, with desks
counted from 1, as in the office.

**A General Manager you start yourself** is recognised too. Turn on **Allow a
General Manager started by hand** in setup's Manager pane and start one with
its token: Settings then says "Your General Manager was started by hand.", and
it isn't counted as an employee. Smolcorp remembers which session it is, the
session id only, so this survives a restart. It's a label: the GM gets no new
power. Its `smolcorp` has to be from 1.24 or later.

Only you answer a permission prompt: neither the GM nor any hire can. The GM
and the Claude Code hires it manages also start inside a fence against
mistakes: see [The fence, and who gets it](#the-fence-and-who-gets-it).

That is the Mac. On Windows and Linux, `smolcorp gm`, `smolcorp guard` and
`smolcorp mcp` are installed, but the app answers the GM only for its version and
the manager switch in setup, and, once the manager is on, for the team, the log
of events and who is waiting for your answer; `gm pin`, `unpin` and `nickname`
work there too. Hiring and firing for you, briefs, messages, watches, tiling
and a General Manager started by hand are the Mac's for now (the fence is
[below](#the-fence-and-who-gets-it), platform by platform), and the
General Manager role there is the plain one from 1.22: you hire it, but it
doesn't run a team for you.

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

**Six floors.** Over the ground floor there are five more, each laid out like
it: nine desks in three rows and a chill area with a couch, a plant and a
coffee counter, in the floor's own style. They're filled in order: cloud bots
such as Grok Bots first, each on its floor (see **Bots on any floor**
below), then runs without a terminal, then anyone with no desk free downstairs
(their keys are still on your phone's list). You can put anyone on
any floor: tap the **⇅ floor** key on an agent, or drop them on the elevator.
In setup's Office pane, you can also give a folder a floor, and its agents sit
there from then on, each keeping their desk. Press the call button beside the
elevator, by the entrance, for a list of the floors and how many are on each;
pick one and the elevator takes you there. In setup, under Office → Floors,
give each floor a name and a style: the office's own, the server room, or any
of the styles. Each floor can be arranged its own way, too: see **Arrange the
room** below.

**Bots on any floor.** A cloud bot, such as a Grok Bot, takes a desk like
anyone else, on any floor, the ground floor too. Put it on one, from the office
or from setup's Bots card, and it stays there. Otherwise it sits where a
floor's rule for its folder says (`/cloud/<its name>`, or `/cloud` for every
bot), or on the floor new bots go to, which you choose on the Bots card; until
you do, it's the floor most of your bots are on. When that floor is full, the
bot sits where there's room, and setup names any bot left with no desk on any
floor. There's no set number of bots: the floors' desks are the room.

## The fence, and who gets it

The General Manager and the Claude Code hires it manages start with a fence:
Claude Code deny rules and a guard that refuse `git push`, deploy, publish and
cloud commands, and keep out of your `.env` files and keys. That is a fence
against mistakes, not a wall: a script a hire writes and runs can get past it,
and everyone still runs as you. On the Mac, you can see and extend the list in
setup, under Advanced → **Guard**; [docs/manager](manager/README.md) says more.

Who gets the fence:

- **On the Mac:** the General Manager, and every Claude Code hire made while
  the manager is on (setup's **Let a General Manager run your team**), or
  made into a worktree of its own.
- **On Linux:** a role's Claude Code hire started inside tmux (setup's
  **Start inside tmux**, or when there's no terminal window to open), the
  General Manager's included, unless it's hired at a pinned project's desk or
  on a floor with a folder of its own.
- **On Windows:** a role's Claude Code hire, only with managed hires turned
  on: a beta, off by default (`"hireManagedWindows": true` in `config.json`).

**Not fenced:** a Claude Code session you hire at a desk with the manager off
on the Mac, or outside tmux on Linux; a hire with no role on Windows and
Linux; and every hire of Codex, Gemini CLI, OpenCode and Grok. They run with
their own settings, as they would if you'd started them yourself. The hire
sheet in setup and the office say so when you pick one.

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
