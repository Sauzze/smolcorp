# Hiring guide

How a hire works, what keeps it in bounds, and where those limits end.

## A hire's life

1. **Hire.** The GM (or you, from the setup page) picks a role and a folder. Smolcorp makes a **git worktree** and a branch for it, so no two hires share a folder, and gives it its own port range.
2. **Start.** The hire is Claude Code running in **tmux** on Smolcorp's own tmux server, so it survives the app closing. Smolcorp checks the session really started (an agent in that folder, its prompt on screen) and undoes the hire if not.
3. **Brief.** The hire starts with a `BRIEF.md`: the job, what "done" means, the proof wanted, what is off-limits. Write it in plain words and give it numbers to check (see the [examples](examples/)). Never put a key, token or password in a brief or a message.
4. **Work and report.** The hire keeps a `STATUS.md`: its plan at the top, then one dated line per step with its proof. Lines that start with a marker mean something:
   - `NEEDS-REVIEW:` the work is ready, or only you can decide something. Carry on with the best default meanwhile.
   - `BLOCKED:` it can't go on; it says what would unblock it.
   - `STOP` the work stops; the GM reads why and decides or asks you.
   - `DONE:` as the last line, with a one-line summary.
5. **Review.** A different hire, who gets the code and not the builder's conclusion, reviews it. Nothing merges without its PASS.
6. **Fire and recap.** The GM checks the proof, asks the hire to stop its servers, then fires it. Smolcorp refuses to fire a hire whose worktree has changes nobody committed, who is busy, or whose window shows a question or a menu, and says why. `force` overrides the uncommitted-changes and busy refusals, never a question or a menu. A fired hire leaves a recap in `recaps/<id>.md`: who, task, folder, branch, when, cost, and its last status.

## What stops a hire

A managed Claude hire starts with its own settings file, merged on top of your account's. It holds two things. Which hires are managed differs by platform, and hires of Codex, Gemini CLI, OpenCode and Grok never are: see [The fence, and who gets it](../team.md#the-fence-and-who-gets-it).

**Deny rules** (Claude Code's own permission rules, which win over any allow):
- push, deploy and publish commands (`git push`, `vercel deploy`, `npm publish` and the like);
- reading or writing `.env*` files anywhere;
- editing any Claude settings file, and the app's own files: the hire guard's rules, the registry and the outbox. Edit and Write are also denied in every hire's folder by file type (`*.json`, `*.sh`, `*.jsonl`, `*.log`, `settings*`), not by name, so this covers hires started later too;
- reading the app's named files, the token files, Claude's session transcripts and `.env*` files (Read is denied only for these).

`STATUS.md` (a `.md` file) stays writable, as the hire must be able to write it.

In setup's **Guard** you can switch a group of commands off (deploy, say). That drops it from the guard **and** from these deny rules: you allowed it, so Smolcorp no longer blocks it for a hire. Off means allowed, for the guard and the deny rules alike: nothing else stands in the way of that group. Setup says so on every group that is off ("Off: hires will be able to deploy…"), and every group is on until you switch it. Switch the group back on and both come back.

**The guard**, a hook that looks at every tool call before it runs (`smolcorp guard`). It reads shell commands, including inside `sh -c`, `env`, `sudo`, `xargs`, brace expansion and `$'…'` quoting, and refuses push, deploy, publish, cloud and remote-login commands, `tmux send-keys`, `kill-*` and `run-shell`, `osascript`, `security`, stored-credential commands, any command naming `disableAllHooks`, any mention of the GM token, and starting Claude Code again (`claude`, `npx @anthropic-ai/claude-code` and the like), because a second Claude Code wouldn't carry the hire's fence. That last one is always on: the guard editor can't switch it off, and the GM's profile has it too. It also refuses the GM's actions in `smolcorp gm` (hire, fire, brief, tile, watch, unwatch, screen), but not the ones a hire needs, such as send and inbox. The GM itself runs under its own guard profile (`smolcorp guard --profile gm`): the same rules, except that its own `smolcorp gm` actions are allowed. If the guard can't read a command, it refuses it. If its rules file is broken, it refuses every shell command. If the guard program is missing from the app, a hire is not started at all.

A hire also can't switch the hook off: the settings that set it are the app's, and editing them is denied.

## What doesn't stop a hire

These are known and **not fixed**. The rules are a fence against mistakes and a clumsy agent, not against one set on getting out.

- A script the hire writes and then runs.
- Commands built from variables, or run through `python -c`, `node -e`, `perl`, `awk` and the like.
- Git aliases, and `xargs` when the command arrives on stdin.
- Wrapped `yarn`, `bun` or `pnpm` with some global options, and `yarn workspace`.
- A glob into the app's folder after a `cd`, such as `cd ~/Library/... && cat *`.
- Reading the GM's environment by any way other than `ps` (a script, another language; the hire runs as you). The guard refuses `ps` that shows it.
- A program that starts Claude Code itself (a Python `subprocess`, a script, `node -e`), or one named through a variable or `$(which claude)`: the guard refuses a nested `claude` typed as a command, but not one started from inside another program, and that one has no fence.
- The guard only covers Claude. Other agents get no guard.
- A refusal from auto mode only shows up when auto mode makes one (`PermissionDenied`).

Everything runs as **you**. A hire that gets past these has your files, your keys and your accounts. If that is more than you want to risk, build the wall below.

## The optional wall: a separate macOS account

For work you don't fully trust (a client's code, a long unattended run), run the hires as a different user. Then a bypass finds little of yours to read. Smolcorp doesn't do this for you; here are the steps.

Smolcorp's own hires still start as you, in a folder inside your home (the front desk refuses a folder outside it). This wall is for sessions you start yourself in the other account, and the GM can't hire into it.

1. **Make the account.** System Settings → Users & Groups → Add User. A **Standard** account, not Administrator, with a name like `hires`.
2. **Give it only what it needs.** Sign in once and set up Claude Code there with its **own** login, ideally a separate plan or an API key with a spending limit. Don't copy your own `~/.claude` over.
3. **Share the project, not your home.** Put the repositories the hires work on in a folder both accounts can reach, such as `/Users/Shared/hires`, and give `hires` write access to only that. You make the worktrees there by hand. Keep your own keys, `.env` files and SSH keys out of it.
4. **No push rights.** Don't give the `hires` account your Git credentials or SSH keys. Pushing stays yours, from your own account, after you've read the diff.
5. **Start the sessions in that account.** Use fast user switching, or start tmux and Claude Code in `hires` over a local SSH session (System Settings → General → Sharing → Remote Login, limited to that user). Keep the GM and the app in your own account.
6. **Close your home folder.** In your own account, run `chmod 700 /Users/<you>`. A Standard user then can't list or read your home.
7. **Check the wall.** As `hires`, try `ls /Users/<you>` and `cat /Users/<you>/Projects/<any file>`. Both should say "Permission denied".

The cost: setting it up takes time and two Claude logins. The gain: the bypasses above matter much less, because there is little of yours to find. Files you made readable to others, and anything outside your home, are still open.
