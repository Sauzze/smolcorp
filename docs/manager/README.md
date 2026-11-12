# The manager

The manager lets a **General Manager** (the GM) run a small team for you. The GM is a Claude Code session that Smolcorp starts. It plans the work, hires one employee per task, checks their proof, and tells you what needs you. You stay the owner: the GM can't push, publish, deploy or use cloud tools, and it never answers a permission prompt.

## Turning it on

It is off until you hire a General Manager (or switch it on by hand under **Manager**), and while it is off Smolcorp works as it did in 1.23. Your team's events are still noted, so switching it on shows what happened lately.

1. Open Smolcorp's setup page from the menu bar.
2. Press **Hire your General Manager** in Start here. Hiring the GM turns the manager on, so you don't need to switch anything on first. (If the hire fails, the manager goes back to how it was.)
3. Switching it off later fires no one.

## Hiring your GM

On the setup page, under **Start here**, press **Hire your General Manager**. Smolcorp starts a Claude Code session with the GM's rules and tools already loaded (a bundled plugin). The GM never asks you for a token or a key, and you never paste one.

Tell it the goal. It answers with a plan and waits for your OK. After that it chooses who to hire, which role and in what order, and asks you only about what is yours. See the [hiring guide](HIRING-GUIDE.md) for how a hire works.

## What the GM can do

- See the team: who is working, where, and what each one last reported.
- Hire an employee (its own worktree and branch), send it a brief, fire it.
- Send a message to a hire, and read the replies in its inbox.
- Ask Smolcorp to watch a file, a time, or a hire going idle, and be told when it happens.
- Tile the team's windows on a screen (on the Mac).
- Keep notes on each employee.

## What the GM can't do

The GM can't push, publish, deploy or use cloud tools: the same guard as hires (with its own `smolcorp gm` actions allowed). It never answers a permission prompt (no route does).

**Never, by the app:**
- **Answer a permission prompt.** No tool, command or route does this, for the GM or for anyone it hires. Only you, in the window that asks. If a hire is stuck on a prompt, the GM tells you.
- Turn on spending, pair or unpair a phone, change which hooks are connected, or edit the summaries' key. These are only the setup page's, and a caller with just the setup-key file is refused.

**Also by its instructions:**
- Merge to your main branch, release, spend or delete. The GM is told not to and asks you.
- Type keystrokes into another window. It talks with messages and the status files.
- Pass your approval to a hire as if it were the hire's own.

There is one GM per office, and only you hire or replace it, from Setup: the GM can't hire another GM, and a hire is never the GM. It has its own key, which lets it write to the GM and to the hires you have allowed it to write to, and nothing else.

## The phone's waiting list

Everything that waits for your answer shows on your phone: a hire's question or permission prompt. The list is read-only: you answer in the hire's own window, never through the GM. When a hire writes `NEEDS-REVIEW:`, your phone gets a "Ready for review" card and a notice. Messages sent to a hire that is at a question, a menu, a half-typed draft or at work are not typed in; they wait, and the GM sees in its outbox any that were given up on.

## More

- [Hiring guide](HIRING-GUIDE.md): a hire's life, what the guard stops and what it doesn't, the optional separate macOS account.
- [Example briefs](examples/): a fix, a review, a docs task.
- [Front desk API](API.md): the routes tools and the GM use.
