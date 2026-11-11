# Windows and Linux parity

How the Windows and Linux app (Go: `desktop/`, `cmd/smolcorp/`) compares with
the Mac app (Swift: `Sources/AgentDeck/`), feature by feature. Both serve the
same web front end, `Resources/web/`, so a gap here is almost always on the
server side: a route, a field of the setup state or the snapshot, or what an
action does on the computer.

Effort: **S** an afternoon, **M** a day or two, **L** several days, **XL** a
subsystem of its own. "–" where there is nothing to port.

## The audit

### Routes

| Feature | Mac | Go | Gap | Effort |
|---|---|---|---|---|
| `/`, `/office`, `/pair`, `/office.js`, `/sounds/*`, `/icon.png`, favicons | ✓ | ✓ | none | – |
| `/events` (SSE, pings, hang-up on unpair) | ✓ | ✓ | none | – |
| `/focus` | ✓ | ✓ | launcher tabs can't be brought forward (no window to find) | – |
| `/act` accept, send, reject, esc, up, down, clear, button, effort, scroll, mic, swap, style, music, volume | ✓ | ✓ | none; scroll only where the terminal can (tmux, kitty) | – |
| `/act?action=fire` | ✓ | ✓ | none | – |
| `/act?action=hire` (Anyone, or a role, at a desk) | ✓ | ✓ | on Linux, a role's Claude hire in tmux at a desk that isn't held or pinned goes through the hire engine (its own tmux server, worktree and ports); every other hire opens as before. Under the Mac app, a role's Claude hire always goes through the engine; one the Mac would manage that can't start fenced (no tmux, a pinned desk or floor, a role on an agent Smolcorp doesn't know) is refused (409), and so is an Anyone hire of Claude with the manager on. A hire of the gm role here is refused (409) while a GM is working, as the Mac does: "A General Manager is already working. Fire it first, then hire another."; with none working it hires as before | – |
| `/act?action=language` (dictation's language) | ✓ | ✓ | a language it takes is 204, any other 400 "Unknown language", as the Mac answers. Under the Mac app (engine=go) it is kept as dictation's language, saved and sent to every phone, as the Mac's Deck does, checked against the languages that Mac has. On Linux and Windows (fr_FR, en_US) nothing is stored and nothing dictates: dictation is the Mac's | – |
| The office's mic dictating (`/act?action=mic`, `language`, `cleanup`) under the Mac app (engine=go) | ✓ | – | not done: the mic presses Claude Code's own voice key, as with dictation off, and a desk's `dictates` and the snapshot's `dictation.on` stay false. A known gap, M3's through the bridge (`Tests/fixtures/contract/INVENTORY.md`), so M7 sees it | M |
| `/dictate` (the phone's text, typed or heard: one line, or several as one paste) | ✓ | ✓ | **done** (I5), with the Mac's rules: a paired phone's key, over the Wi-Fi or Tailscale, never the office's window; the phone-text switch; "!" first, control characters, 2,000 characters; not to a desk asking you something (checked again right before the words; under the Mac app, the app asks the core the desk's state from the terminal's own queue, as the Swift engine looks there); Return on its own after, never part way. Where: tmux (several lines only where the pane's agent asked for bracketed paste: tmux reads its flag and pastes in one command, and says which it did); a launcher's window on Windows and Linux (the launcher pastes into the agent's console input as it pastes mail, several lines only inside a bracketed paste the agent turned on, and says whether it did); WezTerm and kitty one line only; under the Mac app, Terminal, iTerm2 and Ghostty through the bridge (`phone-text`), by the Swift engine's own code (`Deck.phoneWordsRefusal`, `Deck.typePhoneWords`, IGL), so with its every gate (several lines through the clipboard for Claude Code only, only while the desk waits at its prompt with nothing else reading its tab, the Mac unlocked and not in use) and its sentences, as 409s; ⌘V the terminal hadn't read in time, or any failure that may have typed, is a 502 (`Tests/fixtures/bridge/phone-text.json`). The setting has no switch in Setup here yet (config.json's `phoneMicEnabled`, on by default as on the Mac): Setup's phone-mic section explains `tailscale serve`, which this server admits only on a Mac (its `ts.net` name, as the Mac app) | – |
| `/peek`, `GET /choice` (reading a terminal) | ✓ | ✓ | **done** (I5): over Tailscale only (to this computer's Tailscale address from a tailnet one, or handed on by `tailscale serve` from this computer: with a proxy's headers, and on a Mac to its `ts.net` name, as the Mac app; on a Mac Tailscale's `utunN` is told by its addresses, 100.64/10 with fd7a:115c:a1e0::/48), once a second. A proxied request is never this computer's own (as the Mac's `isProxied`): no window key, no setup. The peek reads tmux, WezTerm and kitty, and Terminal, iTerm2 and Ghostty through the bridge (`peek`); a launcher's window can't show its text. No menu is followed here (the Mac's PendingChoice: the iPhone app's), so `/choice` says it isn't asking anything | M (menus) |
| `/choose` (a typed answer to a menu) | ✓ | ✓ | the rules (I5): option=other, one line, the same text rules; with no menu followed here (above), a typed answer that passes them is never typed: 409 "It's asking something else now, or nothing" | M (menus) |
| `/summary` | ✓ | ✓ | none | – |
| `/hook` with agent, tab, project, pid, parent headers | ✓ | ✓ | none; Go also has `/launcher` | – |
| `/setup`, `/setup/session`, `/setup/state`, `/setup/preview` | ✓ | ✓ | Go adds a one-time tray token; `POST /setup/session` without a menu link's key answers 401 with the Mac's text, "Only the menu's link opens a setup page." | – |
| `/setup/layout`, `/setup/theme`, `/setup/logo`, `/setup/logo/*` | ✓ | ✓ | none | – |
| `/setup/profiles` (switch, save, rename, delete, import, check) | ✓ | ✓ | none | – |
| `/setup/pair`, `/setup/unpair`, `/setup/qr.png` | ✓ | ✓ | none | – |
| `/setup/pin`, `/setup/unpin` | ✓ | ✓ | none | – |
| `/setup/scroll`, `/setup/music`, `/setup/login`, `/setup/hooks` | ✓ | ✓ | none | – |
| `/setup/webhook`, `/setup/webhook/test` | ✓ | ✓ | none | – |
| `/setup/summaries`, `/setup/summary/key` | ✓ | ✓ | none | – |
| `/setup/roles` | ✓ | ✓ | the roles themselves differ (below) | – |
| `/setup/hiring` enabled, folder, agent | ✓ | ✓ | none | – |
| `/setup/hiring` terminal, tmux | ✓ | ✓ | **done**, with this system's terminals | – |
| `/setup/hiring/choose` (the system's folder picker) | ✓ | ✓ | **done** | – |
| `/setup/briefs`, `/setup/briefs/<id>`, `/setup/briefs/preview`, `/setup/briefs/restore`, `/setup/hire/folders` | ✓ | ✓ | none on the routes; Go hires get the Mac's BRIEF files, byte for byte (`Tests/fixtures/briefs`), but for the differences under Briefs, below. The setup state's `roles.limits` has no `answer`, `briefFile` or `briefFileName` yet (known gaps in the route contract). Checked by the route contract (`setup.json`), `GET /setup/briefs` on the app's own library on both | – |
| `GET /setup/guard`, `POST /setup/guard` | ✓ | ✓ | none: setup's switches and your own patterns are saved as the Mac saves them, the file byte for byte (`Tests/fixtures/frontdesk/hire-guard.json`), and the guard refuses by them. Checked by the route contract (`setup.json`) | – |
| `/setup/choices` (T3 Code on or off, agent windows), `/setup/hire/window`; the setup state's `choices` and `t3CodeEnabled` | ✓ | ✓ | none (`choices.go`). Checked by the route contract (`Tests/fixtures/contract/setup.json`) | – |
| `/hire/window` (the office's Open a window on a hire's desk) | ✓ | – | not served: 404 (the office polls it every 30 s when hires run in the background: a console error on Linux/Windows until served); Setup's `/setup/hire/window` is. Owner: M4b (Linux/Windows by `openHireViewer`; the Mac via the bridge) | S |
| `/t3code`, `/t3code/open`, `/setup/t3code`, `/setup/t3code/open` (Open in T3 Code) | ✓ | ✓ | none: `lane` included, and a hire in T3 Code opens on its own thread; all four answer 404 while T3 Code is off, as on the Mac | – |
| `/setup/t3code/letgo`, and hiring into T3 Code (`engine: "t3"`, experimental) | ✓ | ✓ | the lane is ported (`t3desk.go`, `t3deskdeck.go`): the same order, fence (byte for byte, `Tests/fixtures/frontdesk/t3-fence.json`), Follow, mail and Let go. Reached from Setup's hire sheet (`POST /setup/hire` with `engine: "t3"`, which calls `Deck.hireIntoT3`). A hire in T3 Code has a desk in the office (the row below), and Let go is also on its desk's card. Let go's "no process left" check reads processes on Linux and Windows; the Go core on a Mac can't read a process's folder without cgo, so there it keeps the fence and says why. The follow rests while T3 Code is off | S |
| A hire in T3 Code's desk in the office (T3D-a, experimental): the snapshot's `t3` desk from the registry alone (its worktree in `<repo>-hires` or the hires' worktree folder), `/focus` opening its thread, `/act?action=letgo`. A terminal you open in a T3 hire's worktree has no desk until the hire is let go | ✓ | ✓ | none (T3D-b, `t3desks.go`): the same desk, from the registry alone, with the same refusals; checked by the contract's given "a T3 hire" and its value check (contract.json's `t3Desk`). The office's phone mic (`/dictate`) isn't served on Go (M6), so there's no dictation to refuse there. On Go, the folder for hires' worktrees is `<repo>-hires` (Go has no setting for another) | – |
| `/setup/dictation` | ✓ | – | Mac only (Apple's speech recognition) | – |
| `/setup/accessibility` | ✓ | – | not needed off the Mac | – |
| `/setup/viewer*` (portable office) | ✓ | ✓ | **done** | – |
| `GET /setup/gm/version`, `GET /setup/gm/events`, `GET /setup/gm/team`, `GET /setup/gm/approvals` | ✓ | ✓ | none (the Go app lists only the features `events` and `team`); all but `version` answer 409 while the manager is off; approvals only list, nothing answers a prompt | – |
| `POST /setup/gm/manager` (the manager switch) | ✓ | ✓ | the page only; no GM caller exists here yet | – |
| `GET` and `POST /setup/claude-accounts`, `.claudeAccounts` in the setup state | ✓ | ✓ | the Mac's semantics: ~/.claude is "default", the others are the folders you confirmed (`claudeProfiles` in config.json, up to 8, names of letters, digits, `-` and `_`); found are `~/.claude*` folders with a settings.json or .claude.json; the list is checked whole and refused with the Mac's 400 texts. **Stricter than the Mac:** a folder must be the home folder or inside it (symlinks resolved), and a network or device path is refused. One the Mac saved outside the home folder isn't listed, so under a test copy with engine=go, saving the page's list drops it from config.json (every hire is still denied it while it's there). Only names and folders are looked at, never a token or key. **Limit:** adding or removing an account only saves the list: Smolcorp's hooks are not put into an added account nor taken out of a removed one (the Mac does), and `connected` is true only where Go's own hooks check finds the hooks in that account's settings.json | M |
| `.manager` in the setup state | ✓ | ✓ | the Mac's shape: `enabled` is the manager switch; `gmByHand` and `gmStartedByHand` are false and `gmTokenFile` and `gmTokenFileShown` empty, as this app has no GM started by hand and keeps the GM's token in memory only. The page shows its Manager panel whenever `manager` is there, so it now shows on Linux and Windows; its by-hand and token controls are hidden here (the setup state names no token file); its fire buttons (the GM's and the team's) call `POST /setup/gm/fire`, served here, and its screens choice asks `GET /setup/gm/screens`, which isn't (404, so the choice stays hidden) until M4. Under the Mac app with engine=go (test copies), `gmByHand` is always false although the Mac's config.json keeps the real key: engine=go has no by-hand GM yet | – |
| An unknown route, `GET /setup/qr.png` without a pairing code, `GET /setup/logo/<unknown>` | ✓ | ✓ | each answers an empty 404, as the Mac's does (it was Go's "404 page not found" in text) | – |
| `POST /setup/gm/registry` | ✓ | ✓ | the page only, for the same reason | – |
| `/setup/hire/gm` (Start here's Hire your General Manager) | ✓ | ✓ | the Go app hires the plain GM role (through the hire engine when the agent is Claude: on Linux when tmux is the chosen or only way, and always under the Mac app, where with no tmux it's refused (409); otherwise as before); the Mac's also gives it its token, rules and tools. It refuses (409) while a GM is working, as the Mac does: "A General Manager is already working. Fire it first, then hire another." Nothing is fired for it | L |
| `POST /setup/hire` (Setup's own hire sheet: folder, answers, task, model, nickname, worktree, account) | ✓ | ✓ | the Mac's checks and words, and refused while hiring is off (403), for the page and the GM alike. A role's Claude hire in tmux goes through the hire engine with its answers, task and fence; under the Mac app it always does, and one the Mac would manage (the manager on, a worktree asked for, or the GM) that can't start fenced (no tmux, a desk or floor pinned to a folder) is refused (409). Any other opens through the launcher, which asks the role's questions itself: the answer's `note` (an extra the Mac doesn't send) says the task and answers weren't passed on, and a Claude hire that way on a named account, or in a test copy, is refused (409). Into T3 Code (`engine: "t3"`, experimental): hired through the T3 Code lane (`Deck.hireIntoT3`), after hiring off, the role, Claude Code's hooks and the cooldown, with the Mac's answer; an account or the API-key account named for it gets T3 Code's own sentence (409). On the API-key account otherwise: refused in a sentence (409). The accounts a hire may name are setup's own (`state.claudeAccounts`, the same checker, `claudeAccountList`): one setup leaves out, a folder outside the home folder say, no hire takes either, and with two or more accounts a hire that names none gets "Say which Claude account to hire on". Each saved account's settings and transcripts are denied to every hire, one outside the home folder too (as on the Mac), on every path: in tmux, through the launcher (Windows' managed hires: the office, this sheet and the GM's route) and in T3 Code's fence (`t3FenceSettings`, as the Mac's `HireSettings.forHire`; with nothing saved it is the default account alone, the `t3-fence.json` fixture's): that list is every saved folder given as a full path, checked by its text alone (no stat, no symlink followed, no share touched; a network, device or relative path gives no rule) | – |
| `POST /setup/gm/send`, `GET /setup/gm/outbox` (mail) | ✓ | ✓ | none on the routes (`mail_routes.go`); a hire's mail is typed into its tmux or launcher pane, a bot's goes to its mailbox folder. Checked by the route contract (`Tests/fixtures/contract/gm.json`) | – |
| `POST /setup/gm/contacts` | ✓ | ✓ | none (`frontdeskmail.go`). Checked by the route contract (`gm.json`) | – |
| `GET /setup/gm/watches`, `POST /setup/gm/watch`, `POST`/`DELETE /setup/gm/unwatch` | ✓ | ✓ | none on the routes (`watchers.go`); a firing is a line in the event log, as on the Mac. Checked by the route contract (`gm.json`) | – |
| `POST /setup/gm/nickname` | ✓ | ✓ | none (`gmnickname.go`). Checked by the route contract (`gm.json`) | – |
| `GET /setup/gm/screens` | ✓ | – | not served: 404 | M |
| `POST /setup/gm/hire` | ✓ | ✓ | for the GM and the page, through the same hire path as the office, and only into a hire Smolcorp fences (the tmux engine, or the launcher's behind its beta switch): anything that would open a plain terminal is refused with 409, never a fallback. `engine` as the Mac's: `"t3"` hires into T3 Code through Setup's own lane (as `POST /setup/hire` with `engine: "t3"`), `"terminal"` or left out is the hire above, any other word is 400 in the Mac's words. Refused with 400, naming the field: `account`, `model`, `nickname`, `task`, `answers`, `apiKey: true`, `worktree: false`. `floor` (0 to 5) is honoured, on the floor's first free desk; a floor with a folder of its own, or a pinned desk, is refused (409). A `folder` outside the home folder, or the home folder itself, is 400. Under the Mac app (this engine), it goes through the hire engine or is refused: a missing tmux in M4's words, a floor with its own folder in the engine's words for the route (another floor), and a field it can't honour as "this engine"'s. **On Windows** the hire is a launcher window (`smolcorp run`), only with managed windows on in Setup; with it off the route says so in one sentence (409): never a plain terminal, and there is no tmux hire | M |
| `POST /setup/gm/fire`, `POST /setup/gm/brief` | ✓ | ✓ | fire is served: the office's fire for a hire in the registry (every refusal of `fire.json`, nothing typed, not even with force); no `live:<session>` ids (404); the GM's own token can't fire the GM (409); a hire in T3 Code is 409 in the Mac's words (let go from Setup's team list or from its desk), with nothing run. The worktree is kept unless `removeWorktree: true` is asked: a plain fire is 204 with no registry note, and 200 `{ok, kept, worktree, branch}` only when removal was asked and the worktree had to stay (`fire.json`). Brief is served: the dated part goes to the end of the hire's BRIEF file (the Mac's bytes, `briefmore.go`) and a mail tells the hire, nothing typed; a hire with no BRIEF file (a plain hire, not the engine's or the launcher's) is refused with 409 ("has no BRIEF file to add to"). **On Windows** fire ends only the hire's own launcher, found by registry id, and refuses a hire that is not a launcher window (409); a window the launcher can't read is refused even with `force` | S |
| `POST /setup/gm/tile`, `screen` | ✓ | – | not served: 404; tiling is the Mac's | XL |
| `POST /setup/gm/token/rotate`, `POST /setup/gm/by-hand` | ✓ | – | no GM token here | – |
| `smolcorp gm`, `guard`, `mcp` | ✓ | ✓ | the same commands on all three; on Go they can ask only what is served above | – |

### Config keys (`config.json`)

| Key | Mac | Go | Gap | Effort |
|---|---|---|---|---|
| pins, colours, layout, profiles, activeProfile, logo | ✓ | ✓ | none | – |
| scrollEnabled, hiringEnabled, hireFolder, hireAgent, musicEnabled | ✓ | ✓ | none | – |
| webhook, summaryOptIn, hasCompletedSetup | ✓ | ✓ | none | – |
| roles | ✓ | ✓ | **done**: outfits and every badge kept | – |
| rolesOffered | ✓ | ✓ | **done** | – |
| hireTerminal, hireInTmux | ✓ | ✓ | **done**; under the Mac app (`smolcorp core`), Terminal, iTerm2 and Ghostty open through the app's bridge, and each window's record is kept for its hire | – |
| dictationEnabled, dictationLanguage, dictationForClaude, dictationCleanup, dictationGlossary, keystrokesAllowed, keystrokeApps, showInDock | ✓ | read under the Mac app (engine=go), which does what they say through the bridge; kept, not read, on Linux and Windows | Mac only | – |
| viewer | ✓ | ✓ | **done** | – |
| phoneMicEnabled, phoneMicSkipsConfirm | ✓ | ✓ | read (I5): the phone's text on or off (on by default, as on the Mac), and an older page's text sent from three words; no Setup switch here yet | S |
| unknown keys kept on save | ✓ | ✓ | none | – |

### Roles and the office

| Feature | Mac | Go | Gap | Effort |
|---|---|---|---|---|
| Shipped roles | 18 | 18 | **done**, checked word for word against Roles.swift | – |
| Badges | 26 | 26 | **done** | – |
| `Role.outfit` (what its agents wear) | ✓ | ✓ | **done**, and sent to the phone | – |
| Outfit given to the shipped roles kept from before outfits | ✓ | ✓ | **done** | – |
| A role on its desk (`role` in the snapshot) | by hire, kept across `/clear` and restarts | by hire or `smolcorp run --role` | **done**: the launcher keeps it, across `/clear` too | – |
| `hiring.roles` / `hiring.hire` in the snapshot | ✓ | ✓ | **done** | – |
| Hire by role from the office | ✓ | ✓ | **done** | – |
| `smolcorp run --role` | – | ✓ | Go only | – |
| Layout and room fields (style, pets, wall, plan, hung, added, team, dress code, calm) | ✓ | ✓ | none: the choices are the same lists | – |
| Calm office | ✓ | ✓ | a room choice in both; the drawing is office.js | – |
| "Lite" | – | – | no such setting in either app; office.js's lighter drawing is shared | – |

### Briefs: what differs

The BRIEF files, the setup routes above and the guard file are the Mac's,
checked against a fixture the Mac's tests write. These differences are known
and kept:

- **Accented folder names.** The Mac builds a brief's paths with `URL`, which
  decomposes accents (NFD: "e" then the accent), while it keeps the project
  folder as given (usually precomposed, NFC); so a Mac brief for
  `~/Projets/équipe` mixes the two. Go keeps every path as given, which is
  right on Linux, where an NFD name is a different file. The fixture's
  accented hire is compared with the Mac's brief made NFC.
- **Limits count code points, not characters.** The Mac counts and cuts an
  answer (2,000), a task (8,000), a picked file's name (100) and a guard
  pattern (200) in characters as a reader sees them (grapheme clusters); Go
  counts code points. They agree but for text made of combining marks or
  emoji built from several code points, over the limit: there Go keeps less,
  and refuses a guard pattern sooner. French typed on a keyboard is
  precomposed, so it counts the same.
- **Text compared as is.** Swift's strings are equal when only their accents'
  encoding differs; Go's compare byte for byte. It matters only for two of
  your guard patterns, or two guard groups, that differ that way: the Mac
  takes them for one.
- **Windows line ends.** A library brief you edited and saved with CRLF reads
  as one with plain line ends on Go. The Mac doesn't, and loses its front
  matter and slots.
- **The guard file's layout.** Go writes `"name": "push"`, the Mac
  `"name" : "push"`: the same JSON, with the same groups in the same order.

### Sessions

| Feature | Mac | Go | Gap | Effort |
|---|---|---|---|---|
| States, tools, subagents, models, compaction, errors, effort | ✓ | ✓ | none | – |
| Hand-on after `/clear` and `/resume` | ✓ | ✓ | none | – |
| Messages between sessions (SendMessage) | ✓ | ✓ | none | – |
| Floor 2: headless runs, hired-by card, pid and quiet pruning | ✓ | ✓ | none | – |
| Sessions saved and restored across a restart | ✓ | ✓ | **done** | – |
| A dropped session heard from again comes back, with its desk | ✓ | ✓ | **done** | – |

### The computer's side

| Feature | Mac | Go | Gap | Effort |
|---|---|---|---|---|
| Jukebox play/pause, previous, next | ✓ | ✓ | Linux through playerctl, Windows through media keys | – |
| Jukebox lit while music plays | ✓ | Linux only | Windows can't tell | M |
| Volume bar | ✓ | ✓ | Linux wpctl/pactl, Windows Core Audio | – |
| Summaries with Claude | ✓ | ✓ | none | – |
| Tell other tools (webhook) | ✓ | ✓ | none | – |
| Open at login | ✓ | ✓ | none | – |
| Menu: Open Setup, Pair, Unpair All, Open at Login, Quit | ✓ | ✓ | none | – |
| Menu: Show Office (the office in a window) | native window | in the browser | **done** | – |
| Portable office (beta) | ✓ | ✓ | **done**, but for Copy viewer link, and stop-when-back where the system can't say | – |
| Screen saver | ✓ | – | a macOS `.saver` | – |
| Dictation for agents without a voice mode | ✓ | – | Apple's on-device speech recognition | – |
| Accessibility step | ✓ | – | not needed | – |

### Setup page

The page is shared and reads what each app's `/setup/state` holds. On Go it
shows the launcher step, hides Accessibility and Dictation, shows each role's
`smolcorp run --role` command, and its Viewer link section says "secret
store" and "tray" where the Mac's says Keychain and menu bar. Its role editor
offers every badge and outfit the state lists. Before the portable office was
ported, the Viewer link section showed on Windows and Linux with nothing
working in it: its routes were 404s.

## Ranked: what to port

All six are done: see Progress, below.


1. **Roles, whole.** The 13 shipped roles, their badges and `Role.outfit`,
   the outfit migration, and `rolesOffered`. Small, and the most visible: the
   shared setup page and office already draw them, and today a config the Mac
   wrote loses its outfits when Go saves it.
2. **Hiring from the office.** Open a terminal window running `smolcorp run`,
   as Anyone or as a role, at the held desk, in the hire folder or the pinned
   project's; with the cooldown, the role's first message as the Mac words
   it, the terminals installed to choose from, and tmux.
3. **Sessions across a restart, and dropped sessions coming back** with their
   desk.
4. **Show Office** from the tray, in the browser.
5. **The folder picker** for new hires.
6. **The portable office**: a subsystem of its own.

Not ported, and why:

- **Dictation**: it is Apple's on-device speech recognition. There's no
  equivalent on Windows and Linux without shipping a speech model.
- **The screen saver**: a macOS `.saver` bundle. A spare screen showing
  `/office` does the same job anywhere.
- **Accessibility**: nothing off the Mac needs it.
- **The office window's options** (always on top, on all Spaces,
  click-through, sideways or upright): they're a native window's. Show Office
  opens the browser instead, where Chrome and Edge can pop the office out on
  top.

Left:

- **The jukebox lit while music plays, on Windows** (M). Windows only says
  what's playing through its media session API, a WinRT interface that Go
  can't reach without a WinRT binding or a PowerShell run every three
  seconds; neither is worth it for a light on the jukebox. Play, pause, skip
  and the volume all work.
- **Windows has been built and vetted, never run**, as before this work:
  hiring's Windows Terminal and console windows, the folder dialog and
  GetLastInputInfo are untested on a real PC.

## Progress

Every commit passes `go vet ./...` and `go test ./...`, builds for Windows and
Linux on amd64 and arm64, and passes `scripts/check-public.sh`. The Swift side
is unchanged. The shared setup page changed in four places, each only where
the state says it's the Windows and Linux app (`state.launcher`), so the Mac
sees the page as before: the roles' hint names both ways to start a role,
tmux's hint says what it's for here, and the viewer's steps say "secret
store" and "tray".

Done:

1. **Roles, whole**: the eighteen shipped roles, word for word as the Mac
   has them (a test reads `Roles.swift` to check), all 26 badges,
   `Role.outfit` kept, checked and sent to the phone, the outfit given to a
   shipped role kept from before outfits, and `rolesOffered`, so a role that
   comes with an update is added once and one you deleted stays deleted.
2. **Hiring from the office**, as Anyone or by role, at the held desk, in the
   folder for new hires or a pinned project's, with the Mac's cooldown, checks
   and words. The window runs `smolcorp run --hire <token>`: the token, made
   by the deck for that hire, ties the launcher to the desk it was hired at,
   and only ids and folders are on its command line, never a role's
   instructions, which the launcher reads from setup. A role's first message
   is the Mac's, for a general folder or a pinned desk. Where it opens:
   - Linux: GNOME Terminal, Konsole, Xfce Terminal, kitty, WezTerm,
     Alacritty, foot or XTerm, the first installed until you choose, through
     your login shell, as the Mac's hires, so it finds what your PATH finds;
     or a tmux session of its own, shown in that terminal. With no desktop,
     as over SSH, always the tmux session alone (`tmux attach` shows it):
     the Mac has nothing like that.
   - Windows: Windows Terminal where it's installed, else a console window.
   - A hire whose agent can't start says why and waits for Enter, where the
     Mac's tab would stay open showing the shell's error.

   Tested end to end on Linux: a phone's hire through `smolcorp serve`
   opened tmux, the launcher started the Tester with the Mac's wording, and
   its desk showed the role. Not run on Windows: built only.
3. **A dropped session comes back.** One dropped as its pane seemed to
   close, or its agent's process to go, is remembered for an hour, as on the
   Mac: heard from again, it takes back its desk and what it knew, rather
   than coming in as new; a new agent in its tab ends it for good.
4. **Sessions across a restart.** As the Mac: `sessions.json`, written a
   second after a change, readable by you alone, in the Mac's field names;
   at start, the sessions still running are back on their keys, as they were
   if saved in the last two minutes, at rest if older. Still running is,
   for a tmux, WezTerm or kitty pane, its agent still in front there; for a
   headless run, its process still there; and for a launcher, which the Mac
   doesn't have, the launcher connecting again, as it does by itself a few
   seconds after the app is back. One that doesn't within 30 seconds ended
   while the app was away.
5. **Show Office** in the tray opens the office in the browser, as the Mac's
   menu opens its office window. That window reads the office with a key of
   its own, new at each start, taken from this computer only, and listed as
   no phone; so does the browser's page here. The browser is started with a
   token good once, for two minutes, as setup's links are, so no key is on a
   command line other people signed in here could read; the deck swaps it
   for the key with a redirect. The key then sits in the browser's history
   until the app restarts, readable by you alone. Chrome and Edge can POP OUT
   the page to stay on top: the Mac's window options (always on top, on all
   Spaces, click-through) have no counterpart.
6. **The folder picker** for new hires, from the setup page's Choose…:
   zenity or kdialog on Linux, the Windows folder dialog through PowerShell,
   started in the folder chosen before, or your home. As the Mac, asked
   again while it's open it answers at once rather than opening another; the
   start folder and title are the picker's own arguments, or on Windows its
   environment, never in a script. With no picker, or no desktop, the button
   isn't shown and the path is typed, as before.
7. **The portable office** (beta), as `Viewer.swift`: the viewer's settings
   in config.json; the id, key and write secret in Credential Manager or the
   Secret Service; AES-256-GCM bound to the id (the spec's test vector
   matches the Mac's hash byte for byte); the snapshot built field by field
   with the Mac's redaction (its self-test is ported as a Go test, with a
   secret in every field that must never leave, and checked to fail when
   one leaks); the uploads with the Mac's debounce, heartbeat, backoff,
   hours, stand-in ids and DELETE again after a late PUT, no redirect
   followed; every setup route; and the tray's launch and stop. Tested end to
   end against the viewer's own relay handlers
   (`viewer/scripts/dev-relay.mjs`): the page's own `viewer-crypto.js` opened
   what Go sealed, and after Stop the relay held nothing.

   Different from the Mac, and why:
   - **Stop when I'm back** can't watch for a wake or an unlock as macOS
     announces them. It looks every five seconds instead: on Linux at
     logind's LockedHint and IdleHint, which GNOME and KDE set; on Windows at
     GetLastInputInfo. Back is the screen unlocked after it was locked, or
     input after ten minutes without, with the Mac's two-minute grace. Where
     neither can be told, it stops after its hours, or by hand.
   - **No Copy viewer link** in the tray: the Mac copies it for that Mac
     only, hidden from clipboard managers, which no clipboard here can
     promise. The setup page shows and copies it.
   - **No dot on the tray icon** while it's on: the tray menu's first line
     says so instead.
   - A Linux computer with no secret store can't keep the viewer's keys,
     and says so as the write secret is made.
