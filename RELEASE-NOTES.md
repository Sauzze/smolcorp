# Smolcorp 1.26.0 (beta)

## The Pixel Terminal: talk to your agents in the office
Click the bubble over an agent's head, and read it, answer it and tell it what to do right there, without going to its terminal. [The office](docs/office.md#the-pixel-terminal) has the details.
- **The bubble opens into a small conversation:** the agent's last few messages, a run of tools folded into one line, and a line to type on. **Send** types your words into that agent, with Return. Rest the mouse on a bubble and its last line shows above it, or its question while it asks. Esc, a click on the floor or a second click on the bubble closes it.
- **A desk that asks shows its question** in a small bubble over its head, with its buttons where they fit. The open bubble has a button per choice: a question's options, then **No**, or **Yes** and **No** for a permission.
- **Nothing is typed into a question.** While a desk asks, the line to type on is greyed: answer with the buttons. To tell it to do something else, tap **No**, then type.
- **Needs you.** Where small bubbles would cover each other, the desk that has waited longest keeps its own, and the others show the plain "?". The **Needs you** key in the room's corner counts the desks waiting on you and opens each one's bubble in turn, the longest waiting first.
- **Expand opens the Pixel Terminal:** the desk's whole conversation on a large chalkboard screen over the office, with **Older messages** to scroll back. Its question comes with the choices and the four keys ▲ ▼ ✓ ✕, and its box takes several lines: Enter sends, Shift+Enter starts a new line. Beside it are a small picture of the office and your other desks, the ones waiting first: click one to switch to it. **Next question** goes to the next desk waiting on you. **Shrink to bubble** goes back to the office with that desk's bubble open, and **Back to office**, Esc or the picture closes it. In the Mac app, the ▤ conversation key opens it too.
- **Open in Terminal (Mac).** With hires running in the background, the Pixel Terminal's **Menu** has **Open in Terminal**, which opens a window on that hire's own tmux session, as the ⧉ window key does.
- **Only in the office's own window on your computer,** the one **Show Office** opens (on Linux, in your browser). A phone, a spare screen, the screen saver and Setup's preview keep the ✓ ✕ keys, as before.
- **The conversation is Claude Code's.** Other agents' bubbles say "No conversation for this agent", and otherwise work as Claude Code's do, but for the buttons.
- **Buttons only for Claude Code, where Smolcorp can read its screen:** in tmux, WezTerm or kitty, and on a Mac in Terminal or iTerm2. A question gets its options if it's a single question with one answer and nine options at most; a permission gets Yes and No if it's to run a command or to write or edit a file. Everything else gets the four keys: several questions at once, a question with several answers, any other permission, leaving plan mode, a desk in Ghostty or in a `smolcorp run` window, and other agents.
- **Yes is the menu's first choice.** For its other choices, use ▲ ▼ ✓ in the Pixel Terminal.
- **A button presses nothing that isn't on screen,** a question's **No** included. Right before the key, Smolcorp reads the desk's screen, and presses only while that question's menu is still there. If it isn't (you answered it or put it away in its own terminal), nothing is pressed, and you're told why. The ▲ ▼ ✓ ✕ keys press as they are.
- **Typing follows the phone's rules,** without needing the phone's switch: 2,000 characters at most, no "!" first, and never into a desk that's asking. WezTerm and kitty take one line at a time, and the box says so before you type.
- **Not into a Terminal, iTerm2 or Ghostty tab (Mac).** Typing into one would bring its window forward, so the bubble never types into a desk there: it says so in one line, and you type in that tab yourself. It does type into desks in tmux, WezTerm and kitty, and in a `smolcorp run` window.
- **No bubble** over the office staff, a hire in T3 Code or an idle agent: open an idle desk's bubble with Enter on its desk, or from the desk list. A headless run shows its conversation and its question, with no keys and nothing to type into.

## HTTPS through tailscale serve: on the Mac, Linux and Windows
- **On the Mac, Linux and Windows,** setup's **Use from anywhere** now says where Tailscale stands, and **Turn it on…** runs the `tailscale serve` line for you, after showing you exactly what it runs and what it opens to your tailnet.
- **Through a private socket now.** `tailscale serve` now hands the deck on through a private socket in Smolcorp's folder, not its port. Smolcorp checks that what connects there really is Tailscale, and anything that comes over the port with Tailscale's headers is just another program on your computer.
- **Turn it on again, once.** If HTTPS through `tailscale serve` was on before this update, phones can't reach the deck that way until you turn it on again. Setup says so, and **Turn it on…** does it in one tap.
- **If your phone shows 502 Bad Gateway,** Tailscale reached your computer but Smolcorp didn't take what it handed on. Setup and [the docs](docs/phone.md#if-the-phone-shows-502-bad-gateway) name the three causes and the fix for each: Smolcorp couldn't open its socket (quit and open it again), it refused who connected (Tailscale from the App Store on a Mac; on Linux, a tailscaled not running as root), or Tailscale can't reach the socket.
- **Not with Tailscale from the Mac App Store.** Use Tailscale's own download from tailscale.com instead. The App Store version runs as you, so Smolcorp can't tell it apart from any other program on your Mac, and refuses what it hands on.
- **On Linux, what connects as root counts.** tailscaled runs as root, and Linux has no signature to check, so Smolcorp takes what connects to its socket as root. Any program running as root on your computer could do the same, as it can do anything else there. A tailscaled run as you, not as root, is refused.
- **On Linux, the one line, with sudo.** Changing what Tailscale serves takes root there, so setup shows the exact `sudo tailscale serve …` line Smolcorp needs, in full: if **Turn it on…** says Tailscale didn't do it, run that line in a terminal. It does that one thing and nothing else. Setup also mentions making yourself Tailscale's operator (`sudo tailscale set --operator=$USER`) so **Turn it on…** works without sudo, but only as an option, and says what it means: every program you run could then change Tailscale's settings (serve, funnel and the rest), not only Smolcorp.
- **On Windows, through a port of its own.** Tailscale on Windows can't hand on to a socket, so it hands the deck on to the port after Smolcorp's (7711 for 7710), on this computer only. Smolcorp takes a connection there only from the Tailscale service itself, as Windows names it: no program you run can pass for it, but one running as administrator could, as it can do anything else on your computer. Anything else that connects there is closed unanswered, and no other program can listen on that port while Smolcorp does.

## Nothing lands on main without you, and nothing is bought
- **Merges onto a base branch are refused** for hires and the General Manager: a `git merge`, `pull` or `rebase` on `main`, `master`, `release/*` or the repository's default branch, a rebase that moves one, and a base branch made, moved or deleted by name (`branch -f`, `branch -D`, `checkout -b`, `update-ref`, a mirror remote and the like), or by writing its file under `.git`. On a base branch, a git command the guard doesn't know is refused too. Merging `main` into a feature branch still runs, and so does rebasing onto `main` or `origin/main`. Your own terminal is untouched.
- **A branch the guard can't place is taken as a base branch.** Where it can't read the repository's default branch (after `-C`, `cd`, `GIT_DIR` or a wrapper), it treats a branch a command makes, moves or deletes by name as the default, and refuses the command. The refusal says how to run it: alone, from the repository's own folder.
- **`gh release list`, `view` and `download` run.** Of `gh release`, only what writes is refused: `create` (and its alias `new`), `upload`, `edit`, `delete` and `delete-asset`.
- **The guard reads what git would run from a command,** for every hire and the GM: config set in the command (`git -c core.fsmonitor=…`), an editor or pager variable other than `true`, `:` or `cat`, and options that take a command (`git grep -O`, `--upload-pack`, `difftool -x`). `git -c core.editor=true` and `GIT_EDITOR=true` still run.
- **Commands that spend money are refused** (Stripe in live mode, creating a server at DigitalOcean, Hetzner, Linode, Vultr or Scaleway, `openai api`).
- **A guard file from before 1.26 gets both groups.** To switch one off, name it in the file with `"on": false`.

## Smolcorp's keys stay out of your agents' reach
Your hires, and every other program on your computer, run as you. From 1.26 they can't loosen Smolcorp's guard while it runs, and on a Mac or Linux they can't read Setup's key, which lives only in Smolcorp's memory. On Windows, only `smolcorp serve` keeps them out of that memory: the tray app leaves its own readable by every program of yours, Setup's key included. On a Mac they also can't read your phones' keys, loosen the guard while Smolcorp is stopped, or forge what lets a phone or a hire in. Windows and Linux can seal less: see below. [Keys, records and the guard](docs/keys-and-guard.md) has the details, and says plainly what's still open.
- **Pair your phones again, once.** At the first start after the update, every key changes: each phone pairs again, hires get new keys when they next start, and the General Manager gets one when you hire it again. A desk started before the update keeps its settings until it's restarted.
- **Setup's key stays in memory.** It lives in Smolcorp's memory only, as the office window's does on Windows. Hires' and the General Manager's keys are kept only as fingerprints, and on a Mac, phones' keys too. Some keys are still in files, readable by you alone: the hook key your agents report with, the check-only `selftest-key`, and off a Mac, the secret each phone signs with, in `phones.json`. The setup link works once: for five minutes on a Mac, and one minute on Windows and Linux. If another program opens it before you do, Smolcorp ends that program's session at once, and Setup tells you what it changed.
- **Records nobody else can write (Mac).** Paired phones, hires' keys, the General Manager, your security settings and your guard are sealed with a key in your login keychain that only Smolcorp reads. If another program changes them, none of them counts until you reset them under **Paired phones**. If the keychain is locked, unlock it and restart Smolcorp. A Smolcorp you build yourself, or rebuild ad hoc, can't read the installed app's Keychain items: it asks for a reset, and for the API key again.
- **The guard holds.** A desk gets its settings as it starts, and can't make Claude Code read them again mid-session: a change to a hire's settings applies at its next start, while a guard rule you add applies at once. A desk needs Claude Code 2.1.314 or later, which stops a command when its guard can't answer. A change to `hire-guard.json` by hand can only add rules, and your next save in Setup drops it.
- **Desks start cleaner.** The General Manager and the Claude Code hires it manages, in tmux on a Mac or Linux, start with a one-time ticket, not through a login shell, so your shell's start files and `~/.tmux.conf` no longer run as they start. Other hires in tmux start on a tmux server of their own, which reads no `~/.tmux.conf` either; a desk in a terminal window still runs your shell's start files, and on Windows a desk gets the launcher's one-time token instead.
- **Claude Code is checked before every start (Mac).** If it isn't signed by Anthropic, nothing starts, and Smolcorp says to reinstall it with Anthropic's installer. Claude Code installed with npm, Codex, Gemini CLI, OpenCode and Grok aren't checked, and Setup says so where you hire. If Claude Code changes from a signed copy to an unsigned one, no desk starts on it until you click **Use this copy** in Setup.
- **Smolcorp's permissions stay with Smolcorp (Mac).** Hires, git, tmux and everything else Smolcorp starts no longer inherit its Accessibility or microphone. macOS may ask you once to let "tmux" or "git" use your Documents, Desktop, Downloads or iCloud Drive folder, or find devices on your Wi-Fi. A project elsewhere in your home folder, like `~/Projects`, asks nothing.
- **Your tools ask first (Mac).** `smolcorp demo`, `status` and `desks` ask you to **Allow** them in Smolcorp's Settings window at each run, with a code to check against the terminal's. The demo's page then opens in that Settings window too, not in your browser, so `smolcorp demo --no-open` is refused before it makes anything. `smolcorp setup` now only checks; the fixes are in Setup. A General Manager you start by hand gets its token through `smolcorp gm login` the same way, and, with no General Manager hired, asks again after each restart of Smolcorp.
- **Smolcorp's own memory.** On a Mac every build runs with the hardened runtime, the `smolcorp` and `tmux` inside it included. On Linux, Smolcorp and its guard keep other programs out of their memory. On Windows, `smolcorp serve` does, but the tray app doesn't. This covers Smolcorp alone: your agents and the programs they run, such as MCP servers, get no such protection.
- **On Windows and Linux, less can be sealed.** There's nowhere to keep a key only Smolcorp can read, so a program of yours can change Smolcorp's files while it's stopped, adding a phone or loosening the guard for later hires, and the change counts at the next start. Any program of yours can also read the API key from the system's secret store, and a desk's keys from its environment. Start here says so.
- **On Windows,** a desk whose settings don't fit on its command line is refused, with a note to turn some guard rules off in Settings. So is Claude Code installed with npm, whose script can't take a desk's settings there: install it with Anthropic's own installer.
- **If you build Smolcorp yourself (Mac),** Start here shows a step while the key you sign it with is unlocked, since any program of yours could then sign a fake Smolcorp. Keep that key in a keychain of its own: see [Releasing](docs/RELEASING.md).

## Hires in their sandbox (Mac)
On a Mac, each desk starts in a sandbox of its own, which keeps it from changing another desk's work or your own checkout. Setup's **Start desks in their sandbox**, under Sandbox, turns it on or off, and lists every desk that runs without one, with why.
- **What it holds.** In your projects folder, a desk changes files only in its own worktree, or in its project folder when that holds no repository: your other projects there are read-only to it. So are its repository's git settings and hooks, your shell's and git's start files, your terminals' and agents' own settings, `~/.claude` and your keychains. Elsewhere in your home folder, it can change what you can. [Keys, records and the guard](docs/keys-and-guard.md#how-a-desk-starts) has the details.
- **The switch covers every hire.** While it's on, every hire starts in its sandbox, the General Manager included, and so does a plain hire you make yourself, at a pinned desk or with no role. Turned off, the desks you start from then on run without one, and Setup lists each as **No sandbox**. Desks already running stay as they are. A hire in T3 Code never runs in it: one hired into T3 Code (experimental), or taken over there, runs outside the sandbox, and Setup lists it as **No sandbox** too.
- **Each hire works in one project of its own.** With the sandbox on, hiring from the office, whatever the agent, first asks which project in your projects folder: each repository there, each one inside a plain folder (such as `clients/acme`), and each folder that holds no repository. Every hire then gets its own worktree of the repository you pick, on a branch of its own, a plain hire too (one with no role, or a Claude Code role with the manager off). A role's hire is told its project, worktree and branch, so its first reply asks only the role's own questions. No hire works in the projects folder as a whole, the General Manager aside (it gets a workspace of its own, and only reads your projects): a hire into it, or into a folder that isn't a repository but holds one, isn't made, and a desk made that way before doesn't start, so fire it and hire it again into the project. With the switch off, and on Windows and Linux, hiring is as before.
- **After a local `git clone`.** A local clone shares its git files with the repository it came from, so a desk working in it doesn't start until you run `git repack -a -d && git prune` in it.
- **Hires run Claude Code, for now.** With the sandbox on, a Codex, Gemini CLI, OpenCode or Grok hire is refused before anything opens, and Smolcorp suggests a Claude Code role instead. They come back in 1.26.1. With the switch off, they start as before, outside any sandbox. These agents still work with the office as before when you start them yourself, in your own terminal.
- **A hire commits only to its own branches:** its branch, and any new one it makes under its own `hire/<id>/`. Other branches, tags and `git stash` aren't open to it, so work in progress goes in a commit on its branch. For another branch or a tag, it asks the General Manager or you, and the General Manager asks you.
- **Firing ends what a hire left running.** Firing a hire, Take over, restart and Hand back now end the background programs it started in its sandbox, a dev server included. A fire ends them once the hire's session has ended, before it checks the repository's objects or removes anything. If one can't be ended, the fire is refused: the objects aren't checked and nothing is removed.
- **Programs started before the update.** What a hire started before you updated ran outside any sandbox, so Smolcorp can't end it. If a hire ran a server or any other background program before you updated, restart your Mac once afterwards to be sure it's gone.
- **Remove `.git/rr-cache` once.** After updating, remove it in each repository where a hire worked, unless you use `git rerere` yourself: a conflict resolution a hire recorded there before this version would still be applied to your merges.
- **Restart in the sandbox.** A Claude Code hire that runs without its sandbox, because it started before you updated or while the switch was off, has **Restart in the sandbox** on its line in Setup, and **Restart all in the sandbox** does every such hire. A restart ends the desk's conversation and starts it afresh in its sandbox, in the same worktree and on the same branch: its files and commits stay. Only an idle desk with nothing typed is restarted, and Smolcorp never restarts a desk by itself. Where a desk can't be restarted that way, its line says why and what to do instead: fire and hire again the General Manager, a hire in T3 Code, one on the API key or one started outside Smolcorp's own tmux; a hire you took over in T3 Code comes back in its sandbox when you hand it back.
- **Hooks and settings your git runs.** A desk's start lists in Setup the git hooks and settings it finds in a repository's `.git`: git runs them at your own commands, outside any sandbox, and a desk that ran without its sandbox could have left one there. Remove what you didn't put there; Smolcorp removes nothing by itself. Setup also lists what makes git read a branch as something else (a replace ref, a grafts file, or a shallow list or multi-pack index that doesn't match the repository), and no desk starts in that repository until you remove or keep each of those.
- **Remote Login.** While it's on (System Settings → General → Sharing), desks can't use your SSH keys or their agents, so commits you sign and pushes over SSH fail in a desk. Turn it off if you don't use it. Smolcorp checks at each desk start, and a desk started while it was off says **Restart to apply** once it's on.
- **Fire your hires before you edit your dotfiles.** While hires are working, don't edit your shell or git dotfiles in your home folder, or a project's git settings and hook scripts, with any tool: one that saves through a temporary file gives a hire a moment to change that file. Setup says so too.

## Hires' worktrees in a folder of Smolcorp's own (Mac)
- **Where a hire's work is now.** On a Mac, every new hire gets its worktree in `~/.smolcorp/worktrees/<repository>-<short id>/<hire>`, a folder Smolcorp makes for itself, readable by you alone. Two repositories with the same name get folders of their own. No hire's worktree is put beside your repository any more (the old `<repository>-hires`), except the General Manager's own, when you hire it into a repository.
- **The hire tells you.** A hire from Setup or from your General Manager names its worktree's folder and its branch in its first message, and how to look at its work: open that folder, or run `git diff main...<its branch>` in your own checkout (with your default branch's name in place of `main`, if it has another). A hire from the office may not say all of that, but its worktree is in the same folder.
- **No General Manager can change it.** Its sandbox reads that folder and writes nothing there, and can't move it or the folder above it. Each hire changes only its own worktree there. So a General Manager can't swap a hire's worktree for a folder of its own while Smolcorp works in it.
- **Hires made before keep their worktrees** beside the repository until you fire them, and each start still checks that the worktree is where it was made.
- **A folder you set yourself** (`hireWorktrees` in `config.json`) is still used if no General Manager could change it. If it's reached through a link in your home folder, holds your home folder or your projects folder, or is in a General Manager's own folder, the hire is refused and says why: take the setting out, or set another folder.
- **A worktree moved or replaced since its hire is left alone.** Wherever a hire's worktree or folder no longer leads where it did at its hire, Smolcorp doesn't run git there, or open a window or T3 Code there: the answer says it was moved or replaced, and to fire the hire and hire it again. Nothing there is listed through it or removed: Smolcorp holds the whole place for you instead, and your own Claude Code doesn't start in or around it until you've checked it yourself. Its recap says **(kept)** beside the worktree, and how to remove it yourself.
- **The demo keeps its hires in the demo folder,** beside its project, as before.
- **On Windows and Linux,** hires' worktrees are beside the repository, as before: there's no sandbox there yet, so no folder is kept from a General Manager.

## Your hires and the General Manager's don't wait on each other
- **A hire the General Manager asks for has a turn of its own.** While it's being made, your own hires from the office and from Setup go ahead, and its own git gives up after a set time, so a General Manager's hire can hold up only itself.

## tmux comes with Smolcorp
- **Open-source notices.** The Mac app now includes tmux 3.8 (ISC licence), with libevent, ncurses, utf8proc and jemalloc built into it. Their licence notices are in the app, and setup links to them (**Open-source notices**).

## The Mac app's program is called Smolcorp
- **Inside `Smolcorp.app`, the program is now `Smolcorp`,** not `AgentDeck`, and Activity Monitor shows it by that name. For the checklist from a terminal, run `/Applications/Smolcorp.app/Contents/MacOS/Smolcorp --setup`. The app's identity for macOS is unchanged, so what you allowed it, and its Keychain items, stay as they were.

## Windows and Linux
- **On Windows, the office window signs in afresh after the update.** The key an earlier window used opens nothing any more, and there's nothing for you to do.
- **Updating on Windows while Smolcorp runs.** From 1.26 on, opening a newer download installs it, and the Smolcorp that's running makes way for it by itself. A running 1.25 or older can't be asked to: if Smolcorp is open when you update to 1.26, quit it once from its tray icon, then open Smolcorp again to start the new version.
- **`smolcorp status` and `desks` run on a Mac and on Windows.** Each run prints a four-digit code, and Smolcorp's own Settings window (the tray icon, then **Open Setup…**) shows the same code, the folder, and the program that asked, as Windows names it: click **Allow** once the codes match. A General Manager you start by hand gets its token the same way on Windows, with `smolcorp gm login`, once **Allow a General Manager started by hand** is on, and so does an app of yours ([Getting a tool's key](docs/manager/API.md#getting-a-tools-key)). The window needs the Microsoft Edge WebView2 Runtime. On Linux there's no window of Smolcorp's own to say yes in, so these commands stop and say so: see your team and set things up in Setup instead. On Windows, `smolcorp demo` asks the same way, makes its folder and opens its page in that Settings window, so `--no-open` is refused there too, but hiring from that page hasn't been tried there yet: the demo is still made for a Mac.
- **Messages and fires reach a hire on Windows.** The General Manager's briefs and messages now reach a hire working in Claude Code, and firing it works, where before the messages could wait for good and the fire was refused.
- **On Windows and Linux, a hire reads its own brief without asking you first.**

## The General Manager and its team
- **`smolcorp gm hire --model` and `--nickname` work on every system.** The hire starts on the model asked for, checked as Setup's hire sheet checks it, and goes by that nickname.
- **A General Manager hired into a folder that isn't a repository** keeps its brief, `STATUS.md` and `TEAM-BOARD.md` in a folder of its own in Smolcorp's folder, where it can write them.
- **A General Manager held to one project** (as the demo's is) keeps its file watches to that project: a link put on the way later can't take a watch anywhere else, and a watch reads nothing once the General Manager is no longer held to that project.

## Known limits
- **A repository's shared object stores (Mac).** A hire can add or change files in git's objects and in git-lfs's files, which every worktree of the repository shares. So the next time git writes a file into your checkout, it can hold bytes a hire wrote, even if the hire's own work never touched that file, and `git status` then shows it as changed. `git fsck` (for git's objects) and `git lfs fsck` (for git-lfs's files, which it also moves aside, to `.git/lfs/bad`) find every stored file whose bytes don't match its id: run them after a hire has worked in a repository, before you check out or push.
- **Remote-tracking branches (Mac).** A hire can update `refs/remotes/*`, as its own `git fetch` needs to. Fetch before you merge `origin/*` into your own branch, so you merge what the remote really holds.
- **Git tools other than git's own command line (Mac).** If you use Xcode's or Eclipse's source control (or another libgit2 or JGit tool) on a repository while hires work in it, a hire could change its packed refs. Use git's command line there, or fire the hires first.
- **Very large repositories.** In a repository with millions of objects, the fire's check of the stored objects can't finish within its two minutes. The hire's worktree and branch are then kept, and you merge its work by hand.
- **Don't run git in a hire's worktree (Mac).** A hire can make a repository inside its own worktree whose settings your git would run there. Look at its work from your own checkout instead (`git diff main...<its branch>`, `git log <its branch>`), and remove a worktree that stayed with `git worktree remove --force <its path>`, from your checkout too: that also deletes what the hire left uncommitted there, so open or copy those files first, without git.
- **Smolcorp looks at each start, not in between (Mac).** It looks for repositories and links in a desk's folder as the desk starts. A repository moved in there, a link made there, or a submodule a hire adds in its worktree, and used within the same session, is caught only at the desk's next start. So while a desk works, don't run git in a repository that turned up in its folder, or behind a link there.
- **Package managers' own folders (Mac).** Swift's `.build`, Bundler's `vendor/bundle/…/cache/bundler/git` and a virtual environment's `src` stay open to a desk, so a hire's `swift build` can fetch new dependencies. A clone a desk fetches there is its own until its next start, and a repository it puts there isn't looked at before then.
- **Installers and updaters (Mac).** A package manager or an app's updater that stages files in a folder of its own can be watched by a hire while it runs. Install and update with no hire working.
- **Links deep in your agents' and apps' folders (Mac).** At each start, Smolcorp follows the links in your terminals', agents' and keychains' folders and in `~/.claude`, so a settings file you keep as a link into a dotfiles folder is held where it leads. It looks there only two levels deep, and not at all into the folders an agent adds to at every session (its sessions, checkpoints, snapshots, temporary files and downloaded packages; in `~/.claude`, `projects`, `file-history`, `todos`, `shell-snapshots`, `debug` and `session-env`). A link deeper in, or inside one of those, isn't followed, so a hire may write where it leads. A hire can't make such a link itself: only you or an app of yours could have. `~/.claude`'s `skills`, `commands`, `agents` and `hooks` are looked into whole, with wherever their links lead, and iTerm2's `Scripts/AutoLaunch` two levels deep.
- **An agent's folder you moved (Mac).** Smolcorp holds your agents' folders where they're kept by default (`~/.codex`, `~/.gemini`, `~/.grok`, `~/.agents`, and OpenCode's in `~/.opencode`, `~/.config`, `~/.local` and `~/.cache`). If you moved one with `CODEX_HOME`, `GEMINI_CLI_HOME`, `GROK_HOME`, `XDG_CONFIG_HOME` or the like, Smolcorp holds it where it went only if the variable is in launchd's environment: set it with `launchctl setenv`, then quit and reopen Smolcorp. A desk that was already running holds it only from its next start: fire it and hire it again. A variable set only in your shell's start files doesn't reach an app opened from the Finder or at login, so that folder isn't held, and a desk can change what your agent keeps there. On Windows and Linux there's no sandbox yet, so no agent's folder is held there, moved or not.
- **zsh's files in a folder you moved them to (Mac).** Smolcorp holds zsh's start files in your home folder and in `~/.config/zsh`. If you set `ZDOTDIR` to another folder, they're held there only if `ZDOTDIR` is in launchd's environment: set it with `launchctl setenv ZDOTDIR <folder>`, then quit and reopen Smolcorp. Set only in `~/.zshenv`, it doesn't reach Smolcorp, and a hire can change the files there.
- **What your shell's start files load (Mac).** Smolcorp holds zsh's and bash's own start files, oh-my-zsh in `~/.oh-my-zsh` or your `ZDOTDIR`, and `~/.p10k.zsh`, not the other files and programs they load from your home folder: a plugin manager's plugins, Powerlevel10k's cache in `~/.cache`, nvm's `~/.nvm`, `~/.cargo/env`, conda. A hire can change those, so your next terminal window runs what it put there.
- **Program folders outside your home folder (Mac).** A hire can add or replace programs in Homebrew's folders and the other folders on your PATH outside your home folder, like `/usr/local/bin`, and in `~/.local/bin`. So a command you run later, such as `ls` or the `ssh` behind `git push`, can be one it put there, and so can a zsh function your next Terminal window runs at its start, from Homebrew's `share/zsh/site-functions`. Only the programs Smolcorp itself runs are held, wherever they're found.
- **Local servers that run commands (Mac).** A hire in its sandbox can still connect to most servers on your Mac's own addresses, since the sandbox can't tell yours from the ones a hire starts for its work. One that runs commands or reads files for whoever connects does that for a hire too: a virtual machine's SSH port, Node's inspector, Jupyter, a browser's debugging port. Smolcorp holds the SSH keys of Lima, Colima, Rancher Desktop and OrbStack machines, and of Podman's default machine (`~/.lima/_config`, `~/.colima/_lima/_config`, `~/Library/Application Support/rancher-desktop/lima/_config`, `~/.orbstack/ssh` and Podman's default machine keys), so a hire can't read them. A `LIMA_HOME` or `COLIMA_HOME` moved elsewhere isn't followed, so keys kept there stay readable. Stop such a server while hires work if you don't need it.
- **A hire's window in your terminal app.** While one is open, what the hire prints reaches Terminal, iTerm2, Ghostty or whichever terminal app shows it, including sequences some terminals act on, such as one that sets your clipboard (OSC 52). Don't paste what a hire's window put on your clipboard without reading it first.
- **A network or FUSE-T volume holding git's hooks (Mac).** If a network share or a FUSE-T volume (a vault, an rclone mount) is mounted in your home folder and holds git's hooks, or a file git's config names, at its top, no desk starts or stops while it's mounted. Smolcorp says which one it is: unmount it to go on.
- **What your General Manager can change (Mac).** A General Manager can change files across your home folder outside a few protected places, so a folder it prepared on purpose could affect a hire there. Look over a hire's changes before you merge them. Inside a General Manager's own folders, Smolcorp doesn't run git, and a hire there can read the files but not change them. Hires' worktrees are in `~/.smolcorp/worktrees`, which no General Manager can change. Smolcorp says so when it happens: in Setup's answer, on the hire's card, and, for a hire from Setup or from the General Manager, to the hire itself. A General Manager hired into your projects folder keeps its plans in `.smolcorp/plans/<its id>` there. It can't make a git folder in it, but it can move in a folder that holds one, and Smolcorp doesn't check that at its next start: don't run git in its plans folder, read or copy the plans instead.
- **No sandbox on Windows and Linux yet.** Hires there run as you, held by the guard only, and Setup says so. The Windows sandbox comes in 1.26.1, and Linux's in a later release.

# Smolcorp 1.25.0 (beta)

**Smolcorp gives your AI coding agents a General Manager.** It plans the work with you, hires one agent per task on its own branch, checks the proof, gets a second agent's review, and asks you before anything ships. Watch the whole team in a pixel-art office, from any browser.

If you're coming from 1.24.2, this covers everything since: 1.24.3 and 1.24.4 were never released, so their changes are here too. The General Manager now hires its team on Windows and Linux as well, your phone can type to your agents, `smolcorp demo` shows a General Manager at work on a sample project, and the Mac app runs the same core as the Windows and Linux app. Read **Updating** at the end before you update a Mac.

**For the Mac** (beta), **Windows and Linux** (alpha). From 1.25.0, the General Manager hires its team on all three (on Windows, with managed hires turned on).

## One core on every computer
- **The Mac app now runs the same core as the Windows and Linux app.** The Mac's own engine is gone. The Mac app is now the menu bar, the windows, and what only macOS can do (the keys, Terminal, iTerm2 and Ghostty, the Keychain, dictation), around the core all three share. The office, setup and your phone work as before, and a fix now reaches every computer at once.
- **What you had stays:** your office, floors, pins, roles, paired phones and hires. A General Manager you hired keeps its token across the update and every restart after it.

## The General Manager hires on Windows and Linux too
- **It runs the team there, as on the Mac.** On Windows and Linux, the General Manager now hires one agent per task, sends a hire a later brief, and fires it. It also messages its hires, asks Smolcorp to watch for things, names people and pins desks.
- **Only into a hire Smolcorp can fence.** On Linux, its hires run in tmux, each in a session of its own. On Windows, it can hire only with managed hires turned on, a beta that's off by default. Where neither works, the hire is refused with what to install or turn on, and it never falls back to a plain window.
- **A General Manager you hired keeps working after a restart.** Its token is kept in your computer's secret store, or, without one, in a private file in Smolcorp's folder, so it keeps its role when Smolcorp restarts or updates.
- **Still the Mac's:** tiling the team's windows, choosing the screen that holds them, a General Manager started by hand, and getting it a new token without hiring it again. On Windows and Linux, hire the General Manager again from Setup for a new token.

## Type to your agents from your phone
- **⚙︎ → Type to …** types into the picked agent, on the Mac, Windows and Linux, over Tailscale, or your Wi-Fi where Smolcorp lets it in. Over the Wi-Fi, what you type isn't encrypted, and the deck says so: Tailscale is the private option.
- **Several lines go in as one paste**, never with a Return part way through. That works in tmux when the agent asked for it, and in a window of Smolcorp's launcher on Windows and Linux. On a Mac it also works in Terminal, iTerm2 and Ghostty, through the clipboard and ⌘V, for Claude Code only: it has to be waiting at its prompt with nothing else open in its tab, and the Mac mustn't be locked or in use (Terminal and iTerm2 come to the front for it). Anywhere else, such as WezTerm and kitty, it's one line at a time: several lines are refused, with a sentence saying so, and nothing is typed.
- **Your clipboard comes back.** After a paste on the Mac, Smolcorp puts back what you had copied, except a password, which is cleared. A clipboard manager, or iTerm2's paste history, may keep a copy of what you sent.
- **Careful by design.** Nothing is typed to an agent that's asking you something, and that's checked again right before the words go in. Text starting with `!` is refused, since Claude Code would run it as a shell command. Return is pressed on its own, after the text is in.
- **On unless you turn it off:** on the Mac, with the **Phone microphone** switch in setup's Phone and voice; on Windows and Linux, which have no switch in setup yet, with `"phoneMicEnabled": false` in `config.json`.

## Your phone's own mic (Mac, over Tailscale)
- **The phone does the listening.** With **Phone microphone** on, open the deck over HTTPS through Tailscale: setup's **Use from anywhere** gives the `tailscale serve` line to run, and a QR code that pairs your phone there. Then hold the mic: the phone recognises what you say itself, and only the words reach your Mac. Nothing is kept. On an iPhone, Apple does the recognising, and may send your voice to Apple for it.
- **Check it before it goes.** Let go, and the phone shows what it heard: send it, type it without sending, or cancel. Or let it send by itself when it's three words or more. Once it's typed, the phone shows the last lines of that agent's terminal.
- Over plain http on your Wi-Fi, the mic listens on the Mac, as before.

## Watch a General Manager at work: `smolcorp demo`
- **One command, one click.** `smolcorp demo` makes a folder of its own (`~/smolcorp-demo`) with a small sample project, says what it will spend and waits for your yes. Then Setup opens with the hire filled in, in Smolcorp's own Settings window on a Mac, and you click **Hire**. The General Manager plans with you, hires a small team on branches of their own, checks their proof, has each branch reviewed, and stops before anything ships, with a report.
- **Kept in its folder.** Smolcorp holds the demo's General Manager to the sample project, whichever way it asks, and a fence of the demo's own refuses anything outside it. The project has no remote and refuses every push.
- **It runs real Claude Code** on your Claude account, never an API key: the General Manager on Opus, and about four hires for some minutes each. `smolcorp demo --dry-run` goes through the same steps with stand-ins and spends nothing. See [The demo](docs/demo.md).
- **Made for a Mac.** What it does on Windows and Linux is under **Windows and Linux** in 1.26.0, above.

## Choose the shortcut that opens the office (Mac)
- **Shortcut to show and hide the office**, in setup's General → Window, is ⌥⌘O until you change it: click it and press the keys you want, with ⌘, ⌥ or ⌃. It works at once, from any app, and needs no permission. **Reset to default** brings ⌥⌘O back, and **Off** turns it off.
- **Your keyboard's own keys.** The keys are shown as your keyboard prints them: on a French AZERTY Mac, ⌥⌘A is ⌥⌘A.
- **It won't take the Mac's keys.** That means ⌘Tab, ⌘Space, ⌘Q, ⌘W and the editing keys every app has, and keys macOS already uses. Keys with ⌘ and neither ⌥ nor ⌃ get a warning, since an app of yours may use them.
- On Windows and Linux, the setting is kept but does nothing: use **Show Office** in the tray.

## Take over in T3 Code (experimental)
**Experimental**, and only with the T3 Code choice on and T3 Code's `t3` v0.1.8, the version it was tried on.
- **Take over in T3 Code** is on a hire's row in setup's team list. It works only while the hire is idle with nothing typed. It stops the hire's Claude Code in its own tmux session and carries on the same session in T3 Code, in its worktree, with its fence. T3 Code turns its own permission prompts off, but the fence still applies.
- **Hand back** brings it home. T3 Code lets go of the session and removes its thread. Once nothing holds the session, the hire carries on in its own tmux session. If a process still holds it, Hand back names that process and doesn't resume the hire, so two agents never share one session.
- **While it's taken over**, nobody can fire it, neither you nor the General Manager, until it's handed back. A hire on the API-key account can't be taken over, because Hand back would bring it back on your Claude account.
- **On the Mac and Linux.** On Windows, a hire runs in a window of Smolcorp's launcher, not in tmux, so Take over is refused there.

## Two plain choices: T3 Code, and agent windows
Start here asks them once on a new install, and Settings has both, under Hires and roles → Hiring. Each has one line saying what it does.
- **T3 Code: On or Off.** On adds T3 Code's features to the office and setup (below). Off, there's no T3 Code button anywhere and Smolcorp never looks for T3 Code. **It's off by default.** Start here only asks when T3 Code is already on this computer: it looks for the app and its `t3` command, without running either. With T3 Code on but not found, Settings says to install it from t3.codes.
- **Agent windows: Open a window for each hire, or Run hires in the background.** A window for each hire is how Smolcorp has always worked. In the background, each hire runs in a tmux session of its own, with no window. When you want to watch one, open a window from its desk: **Open a window** in setup's team list, or the **⧉ window** key in the office on this computer. The background needs tmux: without it, that answer is greyed and says why. With T3 Code on, Start here recommends the background, so your hires work out of sight while you work in T3 Code.
- **Switching to the background closes windows with care.** Smolcorp detaches whoever is watching each hire's own tmux session, then closes only a window it opened for that hire and kept a note of. It never ends a tmux session and never closes anything by its name. A window it didn't open, a Terminal window with other tabs, or one where something still runs is left open, and setup says which, one line each. Switching back reopens nothing.
- **On Linux**, the background applies to a role's Claude Code hire (not one at a pinned desk); other hires still open a window. Switching detaches each such hire, and the window Smolcorp opened for it closes with it; any other window is left open, and setup says so. **On Windows**, the background answer is greyed, since it needs tmux.
- **What you had stays.** If you're updating, you aren't asked: T3 Code stays off, and each hire still opens a window of its own, until you change them in Settings.

## Open in T3 Code
T3 Code ([t3.codes](https://t3.codes)) is a separate app for working with coding agents. With the T3 Code choice on:
- **Open in T3 Code** on a hire's row in setup's team list, or on the **T3 Code** key in the office on this computer (never on a phone), opens that hire's worktree, or else its folder, in T3 Code, in a new thread. T3 Code opens its own agent there, separate from your hire.
- **With only the `t3` command**, the folder is added to T3 Code's projects instead: open T3 Code to see it. Without the command, or without T3 Code, the button is greyed with what to install: the command from T3 Code's Settings → About, or T3 Code itself from t3.codes.
- **Only a hire's own folder**, inside your home folder and not a hidden one. For an agent Smolcorp didn't hire, the office says only a hire's folder opens there. When T3 Code doesn't confirm a new thread, Smolcorp says so and asks you to look in T3 Code, rather than claim it worked.
- Smolcorp never installs anything for it, never types into your hire's terminal, and never runs `t3` through a shell.

## Hire into T3 Code (experimental)
**Experimental:** a hire in T3 Code says so, "EXPERIMENTAL · runs in T3 Code".
- **A Claude Code role, hired into T3 Code.** With the T3 Code choice on and T3 Code's `t3` command found, setup's hire sheet offers **Where its agent runs: In T3 Code (experimental)**. Only you can hire there: the General Manager's ask is refused. The hire gets its own worktree and branch, as any hire does, and a T3 Code thread of its own whose first message is its brief. Its agent works on T3 Code's own Claude sign-in, so there's no account or API key to pick.
- **Behind the fence.** Before T3 Code hears of the folder, Smolcorp writes the same deny rules and guard as any fenced hire's into the worktree's `.claude/settings.local.json`, keeps that file out of git, and checks it's still there right before the thread starts. It then checks the thread really runs in the hire's worktree. If the fence or the project can't be confirmed, no thread starts and everything made for the hire is taken back. If T3 Code starts the thread anywhere else, the hire is taken back too, and Smolcorp says plainly when that thread may still be running in T3 Code.
- **Its desk.** It has a desk in the office, which shows how its thread is doing (at work, waiting for an answer in T3 Code, done), checked about every ten seconds. Tapping the desk opens its thread in T3 Code. Setup's team list shows its name, branch and status. Your phone doesn't type into it: mail it, or open it in T3 Code.
- **Mail.** The General Manager's messages reach its thread. When the thread is busy, a message is queued for its next turn.
- **Let go, instead of fired.** T3 Code has no stop, so you let a T3 Code hire go, from setup's team list or its desk's card, after a confirm that says what happens: its thread and its history in T3 Code are removed; its worktree and branch stay. Smolcorp waits until nothing is still working in the worktree before it takes the fence away and writes the recap. If something is, it says so and keeps the fence. To stop it without removing anything, open it in T3 Code and stop it there. Firing a T3 Code hire from the office, a phone or the General Manager is refused, with a pointer to Let go.
- **Only with T3 Code's `t3` v0.1.8**, the version Smolcorp's fence was proven on. Any other version is refused, with one sentence naming it.
- **If Smolcorp closes partway through such a hire**, the next launch removes its T3 Code project and its fence, and marks it failed. Its worktree and branch stay.

## Your first General Manager, step by step
- **Arrows that point the way.** On the first run, a pixel arrow points at the next thing to do: choose where your hires work, make two quick choices (see above), check what the first hire needs, hire your General Manager, then see it at its desk. It moves on when you've done each step. Skip it any time; **Show Me Around Again** is in the Help menu, the menu bar icon's menu (the tray icon on Windows and Linux) and Easy setup.
- **The folder for your hires in one click.** Start here suggests one (such as ~/Projects), says what it's for, and has **Use this folder** and **Choose another…**. If a folder can't be used, it says why and what to do instead.
- **Checked before the first hire.** Start here checks that Claude Code is installed, signed in and reporting to Smolcorp, and that your Mac lets Smolcorp open the terminal (tmux on Windows and Linux). Each line that isn't ready says what to do: a command to copy, a button that opens the right setting, or **Check again**.
- **One click to hire your General Manager.** Its role is filled in for you. Hiring it also turns on **Hire and fire from the office**, so it can hire its team. Once it's at its desk, Start here suggests what to say to it first.
- **No more hiring into your home folder.** Without a folder for your hires, the General Manager's hire (and, on Windows and Linux, a hire from setup) is refused, and setup points at the step that sets one.

## Setup is easier to find your way around
- **Fewer, clearer sections:** Easy setup, Office and look, Hires and roles, Floors, and Phone and voice, then General and Advanced. Each choice now has one home.
- **Find a setting** on Easy setup leads to what people change most, and the search finds every setting by its words and says where it is.

## The fence, said plainly
The fence is Claude Code deny rules and a guard that refuse `git push`, deploys, publishing and cloud commands, and keep out of your `.env` files and keys. It's a fence against mistakes, not a wall: a script a hire writes and runs can get past it, and everyone runs as you. The README, the docs, setup and the office now say the same thing about who gets it:
- **On the Mac:** the General Manager, and a role's Claude Code hire whenever tmux is installed, unless it's at a pinned project's desk or on a floor with a folder of its own. With the manager on, a Claude Code hire that couldn't be fenced is refused rather than started without it.
- **On Linux:** a role's Claude Code hire that runs in tmux, the General Manager's included, unless it's hired at a pinned project's desk or on a floor with a folder of its own.
- **On Windows:** a role's Claude Code hire, only with managed hires turned on: a beta, off by default.
- **Not fenced:** on the Mac with the manager off, a Claude Code hire with no role, at a pinned desk, or without tmux; on Linux, a Claude Code hire with no role, at a pinned desk, or outside tmux; on Windows, a hire with no role, and every hire while managed hires are off; and everywhere, every hire of Codex, Gemini CLI, OpenCode and Grok. They run with their own settings. (From 1.26.0, a Mac with the sandbox on refuses these four hires until 1.26.1: see **Hires in their sandbox (Mac)** above.)
- **Said where you hire.** When the hire you pick won't be fenced, setup's hire sheet (on the Mac) and the office say so. Setup's General Manager section says who gets the fence on your system.
- **The guard keeps Claude Code's settings.** It now also refuses commands that change or remove Claude Code's settings files or the `.claude` folder (reading them still works), and `git clean -x` and `git stash --all`, which sweep away ignored files such as a T3 Code hire's fence. `mkdir -p .claude/commands` still runs; `mkdir -p .claude` and `cd .claude` are now refused.
- **Only setup opens a folder window.** The window where you pick a folder now opens only from the setup page: a General Manager or a tool of yours can't make one pop up on your screen. The General Manager gets a folder through its hire or the hiring settings instead.

## A short README
- The README now starts with what Smolcorp does and stays short: how a day with it goes, what keeps it safe, and how to install it. Everything else moved into `docs/`: the office, your team, your phone, setup and privacy, Windows and Linux, the demo, extras, troubleshooting, and how it works.
- Your phone opens the office in its browser, over your Wi-Fi or Tailscale, with no app to install. The README, FIRST-SETUP.md and the Linux app's menu entry now lead with the General Manager.

## Four teams in costume
- **Four new styles that dress the whole office:** the Castle great hall (a royal court: king, queen, jester, knight and more), Mount Olympus (Zeus, Hera and seven more Greek gods), the Zoo (nine animal onesies) and the Ant colony (a queen, workers, soldiers and more). Each has nine outfits that everyone wears in that style, the office's staff included, and you can pick any of them for someone on your team.

## Fixed
- **Text from the phone arrives as you wrote it.** A `;` at the end is no longer dropped, accented letters no longer come apart on a Mac, emoji made of several characters stay whole, and text over 2,000 characters says it's too long instead of "Nothing to type".
- **✕ or Esc from the phone** no longer leaves a Claude Code desk showing it's asking you something.
- **The phone no longer waits behind Terminal.** Keys and text for a desk in iTerm2, Ghostty or tmux go straight through, even while Terminal is slow to answer.
- **The Mac and the Windows and Linux app agree.** Moving the Mac onto the shared core meant checking every request both apps answer. Dozens of places where one of them refused, counted or worded something differently now behave the same, and on a Mac, setup and the office still say "Mac".
- The release's zip holds no hidden `._` files, which some unzip tools turned into files inside the app and broke its signature.

## For anyone building from source
- **Building the Mac app needs [Go](https://go.dev) 1.26 or later** (`brew install go`), as well as Apple's developer tools: the app's core is written in Go. `install.sh` stops and says so if Go is missing.
- **The native iPhone app's code is gone.** It was never part of a release: the phone's web page is the way in from a phone.
- **CI** runs the whole Swift test suite on its Mac runner, and the Go tests on Linux and Windows too, with Go's race detector and a `gofmt` check. Tests that need real tmux panes skip where the runner has no tmux.
- The tests check that every script in the app's web pages parses, so a script a browser would refuse fails the tests first.
- **`Resources/web/office.js` is built** from its sources in `Resources/web/office/` (`go run ./scripts/buildoffice`); the tests fail if the two differ. The web viewer copies it when it's deployed, and `viewer/public/office.js` is no longer in git: `viewer/scripts/sync-office.sh` makes a local copy for a preview.
- **Test copies keep to themselves.** A copy you start beside your own Smolcorp to try a build (`scripts/test-copy.sh` on a Mac, or `SMOLCORP_PORT` with `SMOLCORP_TEST_SUPPORT_DIR` on Windows and Linux) starts hidden and never takes focus. It listens on this computer only and holds no office shortcut. It brings no terminal, T3 Code window or system prompt forward, and types into nothing but its own hires. When it refuses something, it says so in a sentence.

## Known limits
- **On Windows and Linux**, the phone's own mic and dictation are the Mac's. Typing from the phone works on all three.
- **Hiring into T3 Code** is built for Windows and Linux as on the Mac, but hasn't been tried on a real install there yet.
- A hire in T3 Code can't write to the General Manager, because T3 Code gives its thread no way to carry the hire's key. The General Manager can still write to it.
- Open in T3 Code hasn't been tried on a real Windows or Linux install yet: where Smolcorp looks for T3 Code there comes from how T3 Code's builds are made.
- On Windows, running hires in the background isn't built yet, and neither is Take over.

## Updating
Download the new version and replace the app, as with 1.24.2. Your office, floors, pins, paired phones and hires are kept. You won't see the first-run arrows unless you ask for them, and you aren't asked the two new choices: T3 Code stays off, and each hire still opens its own window, until you change them in Settings → Hires and roles.

On a Mac, also expect this:
- **The firewall may ask once.** Your phone now reaches Smolcorp through `smolcorp`, the core inside the app. If the macOS firewall is on and **Automatically allow downloaded signed software to receive incoming connections** is off, macOS asks once whether "smolcorp" may accept incoming network connections. Click **Allow**, or your phone can't connect.
- **The General Manager and the hires it manages run in tmux.** If tmux isn't installed, hiring them says so: install it with `brew install tmux`, then hire again.
- **Hires made by 1.24 in a Terminal tab of their own** keep working, but the General Manager and setup no longer close their tab when they fire one: they say they can't find it, and `gm fire --force` only marks it fired and keeps its worktree. To end one, fire it from its desk in the office, which closes its tab when it's sure which one is the hire's, or close the tab yourself.
- **"Port 7710 is in use"** works as it did: if another copy of Smolcorp, or another app, holds the port, the menu bar says so and Smolcorp tries again every couple of seconds, starting by itself once the port is free.

# Smolcorp 1.24.2 (beta)

A small release for 1.24.1: the Mac app is notarised, there's a new floor style for your bots, and a few fixes.

## The Mac app is notarised
- **No more Open Anyway.** Apple has checked the app and the DMG, and the notarisation ticket is stapled to both, so macOS accepts Smolcorp even offline. The first time you open it, macOS only asks whether to open an app downloaded from the internet: click **Open**.

## The Geode, a floor for your bots
- **A new style, The Geode.** An amethyst geode wall with a vault at its heart (a mine cart brings the gems in as it opens), pools of lantern light, a crystal grotto in the chill area, crystals that glow at night, and a crystal golem. It fits every floor's nine desks and chill area, in the office and on your phone.
- **Gem outfits.** Thirteen of them: Ruby, Opal, Sapphire, Amber, Garnet, Topaz, Pearl, Moonstone, Malachite, Jade, Lapis, Amethyst and Obsidian. In The Geode, everyone without an outfit of their own wears one, and someone named after a gem, such as a bot called Ruby, wears that gem's. You can also pick one for anyone on your team.

## Fixed
- **No more "add a desk".** Every floor has its nine desks, so there's no desk to add. When every floor is full, setup now says to hide or remove a bot, or free a desk. When every desk on the ground floor is taken, the Mac app and the Windows and Linux app say to fire someone there, or free a desk.
- The source's tests pass with Go's race detector, and CI runs them that way.

## Updating
As for 1.24.1: download the new version and replace the app. Your office, floors, pins and hires are kept.

# Smolcorp 1.24.1 (beta)

A fix release for 1.24.0, with two bigger changes: every floor now works like the ground floor, and bots can sit on any floor.

## Every floor works like the ground floor
- **Nine desks and a chill area upstairs.** Every floor above the ground floor has nine desks in three rows, plus a chill area like the ground floor's (couch, rug, plant, coffee and water cooler), with the corner piece of the floor's style.
- **Arrange works on every floor.** Pick a floor in the Arranger and move its desks, furniture and chill area, as on the ground floor. Undo, Redo and Back to standard apply to that floor only. Each floor's arrangement is saved, kept across restarts and shown on your phone.
- **Desks 10 to 12.** Anyone who sat at desks 10–12 upstairs moves once, at the first launch, to a free desk on the same floor, or else on the next floor up with one, or else the nearest one below. Nobody disappears; roles and nicknames go with them.
- A profile saved before 1.24.1 has no arrangement for the floors upstairs. Switching to it puts them back to their standard look.

## Bots
- **Bots on any floor.** A cloud bot, such as a Grok Bot, can take a desk on any floor, the ground floor included. Put it there yourself (the office's ⇅ floor, the elevator, or setup → Bots), or let it be placed. A bot you put somewhere stays there.
- **Where new bots go.** A new bot goes to the floor a folder rule names for it (`/cloud`, or `/cloud/<its name>`), or else to the floor set in setup → Bots → **New bots go to**. When that floor is full, it sits at the first free desk from 2F up. It never takes the desk of someone already sitting there.
- **No more limit of nine bots.** Room is counted floor by floor, people and bots together. When every floor is full, setup says which bots have no desk.
- **A bot keeps its desk across a restart**, at its key on the ground floor or at its desk upstairs. One you put on the ground floor takes no room from the floors above.
- **Moving from 1.24.0:** bots that sat "upstairs" stay on the floor they were on.
- **A Grok Bot group chat never gets a desk.** A conversation between your bots isn't a bot. One that had a desk loses it, and its place goes to a bot that was waiting.
- **Bots show their name from the Grok Bot app**, unless you gave one a name of your own. An old stored name no longer wins.

## The General Manager started by hand
- **Its role survives a restart even when the Keychain can't be read.** If macOS couldn't read the GM token from the Keychain at launch (for example after the app's signature changed), Smolcorp used to make a new token, and the started-by-hand GM lost its role until it ran a `gm` command. Now it uses the token in its private file and changes nothing.
- After an upgrade from a build before 1.24.0, the first launch still needs one `smolcorp gm …` command from the GM before the role comes back, since older builds didn't save which session was the GM.

## Fixed
- **Mail to hires with a tmux before 3.7.** On a tmux before 3.7 (stock Ubuntu 24.04 and Debian 12, or an older Homebrew or MacPorts tmux on a Mac), every message to a hire failed: "Its window couldn't be looked at" on Linux, "Couldn't tell how its window takes a paste" on the Mac. It's fixed on both.
- **Windows: hires' deny rules.** On Windows, the rules that keep a hire away from the GM token, the setup key, other hires' folders and its own settings didn't match, because Claude Code reads Windows paths in another form. They match now. Windows managed hires are still beta and off by default.
- **Windows: a hire asked into a repo's subfolder** started at the top of its worktree. It now starts in that subfolder.
- **A style's outfits.** A style that dresses each person differently, such as the Hall of Armor, left about half of the people in their own clothes. Everyone is dressed now, and nobody who was dressed changes outfit.
- **FIRST-SETUP.md** said the Mac app is notarised. It's signed with a Developer ID and not notarised: if macOS says it can't check it, open System Settings → Privacy & Security and click Open Anyway, once.
- **Building from source** works again with newer Swift compilers.
- The source passes CI on macOS, Linux and Windows again.

## Updating
As for 1.24.0: download the new version and replace the app. Your office, floors, pins and hires are kept.

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
