# The demo

Watch a General Manager run a small team on a sample project, from a plan to
reviewed branches, without pointing it at a project of yours. Back to the
[README](../README.md).

## Run it

```sh
smolcorp demo             # Linux
smolcorp-cli demo         # Windows
/Applications/Smolcorp.app/Contents/Resources/bin/smolcorp demo   # Mac
```

It makes a new folder, `~/smolcorp-demo` (then `smolcorp-demo-2`, and so on;
`--in FOLDER` makes it somewhere else inside your home folder), copies a small
sample project into it, and starts Smolcorp if it isn't running. Then it says
what it's about to spend and waits for your yes.

**A browser page opens for the one click.** After your yes, Smolcorp opens
Setup in your browser at the demo, signed in, as Open Setup… does. The page
shows what the demo is about to hire: the project, the folder the General
Manager is held to, the roles it may hire, its task word for word, what it
spends, and which switches Hire turns on. You click Hire there; the demo's
terminal waits for that, then follows the team. The demo itself never hires:
the setup key it holds can't, on purpose. Close the page without hiring and
the demo says so and stops, nothing spent; the demo folder stays for you to
delete. It waits 15 minutes at most, and Ctrl-C stops the wait.

With `--no-open`, nothing is opened: the demo prints the page's link instead.
Open it in a browser on this computer where Setup is already open (signed in);
if it isn't, open Setup from Smolcorp's menu first, then the link.

The page shows a demo only for a folder `smolcorp demo` told Smolcorp about
while Smolcorp was running, with roles the office has. A folder that only looks
like a demo's (a cloned repository, an unpacked download) is refused, and the
General Manager's task always comes from Smolcorp itself, never from the
folder's files. After Smolcorp restarts, run the demo again for a new page.

The sample is `bill`: one Python file that splits a restaurant bill, with its
tests, MIT licensed. Its `TASKS.md` lists three tasks: a lost cent to fix,
tests to add, and a README to finish. Python 3 is all the proof needs.

## What happens

1. **You hire the General Manager** on the demo's page, into the sample
   project, with the demo's goal and its questions answered. If Smolcorp's
   manager or hiring is off, the question and the page say so: Hire turns it
   on, and it stays on until you turn it off in Setup. If the hire is
   refused, the page turns back off what it turned on and says why. At the
   end (the report, Ctrl-C, or Smolcorp no longer answering), the demo's
   terminal says what the page turned on and where to turn it off.
2. **It plans with you.** It shows you the plan in its terminal and waits for
   your OK there.
3. **It hires one agent per task**, each in a worktree of its own on a branch of
   its own: a Bug hunter, a Tester, a Docs writer, or whichever Claude Code
   roles it picks.
4. **It checks the proof** (the tests pass, the diff does what was asked) and
   **has a Reviewer review each branch.**
5. **It stops before anything ships.** Nothing is merged, pushed or published.
   It writes `DEMO-REPORT.md` in the demo folder, and the demo prints it, then
   checks that nothing shipped and lists the branches waiting for you.

The demo's terminal follows the team as it works. The office shows it too, on
your phone or from Smolcorp's menu. Ctrl-C stops following, not the team.

## What it spends

It runs real Claude Code on your default Claude account, or on the one you
pick on the demo's page: the General Manager on Opus, and about four hires for
some minutes each. Its hires work on the General Manager's own account, which
Smolcorp fills in for them. That uses your plan's usage, as any Claude Code
session does. Never an API key: Smolcorp refuses a hire on the API-key account,
or on any account but the General Manager's. A test copy of Smolcorp never
hires on the default account, so there the page picks the test account, and
the hires follow it. Nothing
else is paid for, and the demo asks before it starts anything.

## What keeps it in its folder

- **Everything is in the demo folder**: the project in `app`, the hires'
  worktrees beside it in `app-hires`.
- **The project has no remote**, and a pre-push hook that refuses every push.
- **Smolcorp holds the General Manager to the project.** The demo's page hires
  its GM with a scope, the project, and Smolcorp keeps it with that GM (in its
  registry, so a restart doesn't drop it). Every request made with the GM's
  token, whichever way it comes (`smolcorp gm`, its MCP tools, or a request
  of its own), is held to it: a hire only into the project or the top of one
  of its worktrees, a folder that is there, in a terminal, on your default
  Claude account and never the API key; send, brief, fire, contacts,
  nicknames, pins and watches only for its own hires and folders; nothing
  else that changes your office (its switches, roles, layout or windows).
  Each refusal is one sentence. A General Manager you hire yourself has no
  scope, and works as it always has. The scope needs Smolcorp's hire engine
  in tmux (a Mac, or Linux): where a GM's hire would open any other way
  (Windows' launcher), Smolcorp refuses to hire it with a scope rather than
  drop it.
- **Its own fence, for every session in it.** The project's committed
  `.claude/settings.json` runs `smolcorp demo fence` before each tool call of
  the General Manager and of each hire. It refuses a file changed outside the
  demo folder, or in a `.claude` or `.git` folder inside it (where the fence
  and the hooks are); a hire into any folder but the project or the top of
  one of its worktrees (the folders that hold this fence), one that isn't
  there yet, or one the shell would expand (`$HOME`, `~someone`, `{a,b}`,
  `*`); a hire of a role that isn't
  on Claude Code, into T3 Code, or on the API-key account or another Claude
  account; a `smolcorp` command built with `$` or a backtick, a command whose
  name holds `$`, and a word that holds `smolcorp` without being the program
  or a path in the demo (`X=smolcorp`, `alias h=smolcorp`); any `smolcorp`
  command but `smolcorp gm …`, `version` and `help` (its MCP server on standard
  input, or `smolcorp run`, would hire or start an agent where the fence can't
  see); a send, brief or fire for anyone but the demo's own hires; anything
  that names the setup key; `git config` writes, `git checkout`,
  `git switch`, and `git worktree add` outside the demo folder; and what the
  guard refuses (pushes, deploys, publishing, cloud
  commands). That's on top of the fence the General
  Manager and its hires get anyway ([The fence, and who gets it](team.md#the-fence-and-who-gets-it)).
- Like the guard, it reads one command line: a script a hire writes and runs
  isn't read. It's a fence, not a wall: the wall is Smolcorp's hold on the
  General Manager, above.
- It reads files changed with Claude Code's own tools and shell commands. A
  tool of another MCP server that writes files isn't read.
- It's a Claude Code hook: if `smolcorp` is gone, or doesn't answer within
  10 seconds, Claude Code goes ahead without it. The project's pre-push hook
  and its settings' deny rules for `git push` stay; `git push --no-verify`
  skips the hook, and the fence and the deny rules refuse it anyway.

## A dry run first

```sh
smolcorp demo --dry-run
```

The same steps with stand-ins for the office and for Claude Code: a plan, a
branch per task with a scripted change, a review, the fence refusing a hire
outside the folder, a push and a file written outside it, and the report.
Nothing is started or spent.

## When you're done

Fire the demo's team in the office, then delete the demo folder. The demo
runs its own team: if a General Manager or any other hire is already working,
it asks you to fire or finish them first.
