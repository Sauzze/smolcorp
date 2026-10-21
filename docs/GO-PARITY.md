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
| `/act?action=hire` (Anyone, or a role, at a desk) | ✓ | answers 501 | **hiring from the office** | L |
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
| `/setup/hiring` terminal, tmux | ✓ | – | where hires open | with hiring |
| `/setup/hiring/choose` (the system's folder picker) | ✓ | – | no folder picker | S |
| `/setup/dictation` | ✓ | – | Mac only (Apple's speech recognition) | – |
| `/setup/accessibility` | ✓ | – | not needed off the Mac | – |
| `/setup/viewer*` (portable office) | ✓ | – | **portable office** | XL |

### Config keys (`config.json`)

| Key | Mac | Go | Gap | Effort |
|---|---|---|---|---|
| pins, colours, layout, profiles, activeProfile, logo | ✓ | ✓ | none | – |
| scrollEnabled, hiringEnabled, hireFolder, hireAgent, musicEnabled | ✓ | ✓ | none | – |
| webhook, summaryOptIn, hasCompletedSetup | ✓ | ✓ | none | – |
| roles | ✓ | ✓ | Go loses `outfit` and the 13 newer roles' badges | S |
| rolesOffered | ✓ | – | Go never adds a role that comes with an update | S |
| hireTerminal, hireInTmux | ✓ | – | with hiring | with hiring |
| dictationEnabled, dictationLanguage, dictationForClaude | ✓ | kept, not read | Mac only | – |
| viewer | ✓ | kept, not read | with the portable office | XL |
| unknown keys kept on save | ✓ | ✓ | none | – |

### Roles and the office

| Feature | Mac | Go | Gap | Effort |
|---|---|---|---|---|
| Shipped roles | 18 | 5 | Art Director to Chef missing | S |
| Badges | 26 | 13 | 13 badges refused as roles are checked | S |
| `Role.outfit` (what its agents wear) | ✓ | – | dropped as roles load, so a Mac config saved by Go loses it | S |
| Outfit given to the shipped roles kept from before outfits | ✓ | – | migration | S |
| A role on its desk (`role` in the snapshot) | by hire, kept across `/clear` and restarts | by `smolcorp run --role` | Go's comes only from the launcher | with hiring |
| `hiring.roles` / `hiring.hire` in the snapshot | ✓ | always empty / false | with hiring | with hiring |
| Hire by role from the office | ✓ | – | **hiring** | L |
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
| Sessions saved and restored across a restart | ✓ | – | after a restart the desks are empty until each agent next reports | M |
| A dropped session heard from again comes back, with its desk | ✓ | – | it comes back as new, and loses its desk | S |

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
| Menu: Show Office (the office in a window) | native window | – | could open the office in the browser | S |
| Portable office (beta) | ✓ | – | AES-GCM viewer on your own Vercel | XL |
| Screen saver | ✓ | – | a macOS `.saver` | – |
| Dictation for agents without a voice mode | ✓ | – | Apple's on-device speech recognition | – |
| Accessibility step | ✓ | – | not needed | – |

### Setup page

The page is shared and reads what each app's `/setup/state` holds. On Go it
shows the launcher step, hides Accessibility and Dictation, says hiring "needs
the Mac app", shows each role's `smolcorp run --role` command, and hides the
viewer link (GET `/setup/viewer` is a 404). Its role editor already offers
every badge and outfit the state lists; Go lists only 13 badges.

## Ranked: what to port

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

## Progress

Nothing ported yet.
