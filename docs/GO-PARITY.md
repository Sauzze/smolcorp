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
| `/act?action=hire` (Anyone, or a role, at a desk) | ✓ | ✓ | **done** | – |
| `/act?action=language` (dictation's language) | ✓ | – | dictation is the Mac's (see below) | – |
| `/summary` | ✓ | ✓ | none | – |
| `/hook` with agent, tab, project, pid, parent headers | ✓ | ✓ | none; Go also has `/launcher` | – |
| `/setup`, `/setup/session`, `/setup/state`, `/setup/preview` | ✓ | ✓ | Go adds a one-time tray token | – |
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
| `/setup/dictation` | ✓ | – | Mac only (Apple's speech recognition) | – |
| `/setup/accessibility` | ✓ | – | not needed off the Mac | – |
| `/setup/viewer*` (portable office) | ✓ | ✓ | **done** | – |

### Config keys (`config.json`)

| Key | Mac | Go | Gap | Effort |
|---|---|---|---|---|
| pins, colours, layout, profiles, activeProfile, logo | ✓ | ✓ | none | – |
| scrollEnabled, hiringEnabled, hireFolder, hireAgent, musicEnabled | ✓ | ✓ | none | – |
| webhook, summaryOptIn, hasCompletedSetup | ✓ | ✓ | none | – |
| roles | ✓ | ✓ | **done**: outfits and every badge kept | – |
| rolesOffered | ✓ | ✓ | **done** | – |
| hireTerminal, hireInTmux | ✓ | ✓ | **done**; a Mac terminal's id reads as not installed | – |
| dictationEnabled, dictationLanguage, dictationForClaude | ✓ | kept, not read | Mac only | – |
| viewer | ✓ | ✓ | **done** | – |
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
