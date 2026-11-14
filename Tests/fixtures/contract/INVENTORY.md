# The route contract: inventory

Every route the Mac app's own engine served until 1.25.0 (its Swift engine, deleted in M7), who may call it,
what it takes and answers, which fixture covers it, and whether the Go app (`desktop/`), which every Mac runs
since 1.25.0, serves it: **served**, **404**, or **differs** (served, with a known gap).

- The fixtures beside this file were written from the Swift engine's answers by its `ContractTests.swift`, gone
  with it in M7: they're now the reference the Go app keeps, asked of it by `desktop/contract_test.go`, and
  changed by hand, with that test, when the contract changes. Shapes, not values: status, kind of body (refusals too),
  and for JSON its keys, their types and a closed set's values (a session's state, an agent…). Nothing empty
  stands for anything: a null, `{}` or `[]` where the Mac has something is a difference. `contract.json` holds
  the shapes routes share (the setup state, the snapshot) and what each fixture's `given` sets up.
- **Go** is checked by `desktop/contract_test.go` for every route with a fixture here: it must be what the
  known gaps (the last table) make it. For a route without one it is read from `desktop/server.go`, and
  cross-checked with `docs/GO-PARITY.md`; a "404" there is asked of the Go app in process, and must be 404, and
  names who serves it (`404, owner: <who>`, as the known gaps' Owner). A "served" one is checked too: a file of
  the web folder is asked for; a setup route must have its callers in Go's route table and its case in the router.
- What only macOS does (Accessibility, keys pressed in other apps, In the Dock, dictation, Add smolcorp to
  Terminal, the Grok Bot app, what a desk's terminal lets the office do) the Go app says and serves only as the
  Mac app's engine, asking the app over the host bridge (`desktop/hostbridge.go`, whose actions are in
  `Tests/fixtures/bridge/actions.json`); on Linux and Windows those fields are left out and those routes are 404.
  `desktop/contract_test.go` asks with a stand-in for the Mac app, so the Mac's answers are checked on any computer.
- **Fixture** names the file, or says `none: ` and why: no fixture asks a route that writes the home folder,
  asks the Keychain, opens a window, a picker or a terminal, starts another program, or reaches another
  computer. Those have their refusals in a fixture where one comes first.
- What only the native iPhone app asked (`/glossary`, `/notifications`, `/reviews`, `/setup/notify`) has no rows:
  the app and its Mac side were dropped (M7DROP), and the phone's web page needs none of it.
- Callers: `page` (the setup page's cookie), `tool` (a tool's key: since 1.26 (SK2) no file holds one; a terminal
  asks for it at `/terminal/ask` and gets it on the owner's Allow; the `selftest-key` file opens `GET /setup/selftest`
  alone), `gm` (the GM's token), `hire` (a
  hire's send key), as `frontdesk/routes.json`; `phone` (a paired phone: its requests signed with its key, or
  the key itself on this computer and over Tailscale only (SECWIFI); or the office's own window),
  `window` (the office's window on this computer only), `hook` (the hook's key, from this computer), `anyone`.
  Each route outside setup is also asked by one who isn't its caller ("not one of its callers").


## The office, the phone and the hook

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| GET | `/` | anyone | `?t=` a phone's key (this computer and Tailscale only), or signed (SECWIFI); `?deck` the deck alone, its key after `#` | HTML: the pairing page without a key, the deck with one or `?deck` | 200, 403 (not addressed directly) | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/office` | anyone | `?t=` | as `/` | 200, 403 | none: a file of the app's web folder, which the test bundle doesn't have | served (Go also swaps a tray token `?o=` for the window key, 303) |
| GET | `/icon.png` | anyone | — | PNG: the logo's 180 px icon | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/favicon-16.png` | anyone | — | PNG | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/favicon-32.png` | anyone | — | PNG | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/office.js` | anyone | — | JS | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/guide.js` | anyone | — | JS | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/sounds/*` | anyone | keys/talk .m4a/.webm | audio | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| POST | `/pair` | anyone | `{code}`, on this computer or over Tailscale only (SECWIFI); or the QR code's claim, signed with its key, `?p=` the phone's own part | `{key}`; the claim: empty | 200, 204, 403 `{error}` (a typed code over the Wi-Fi; a claim also `{why, now}`), 409 `{error}` (Go: a typed code while the records can't take a phone, in Setup's words: 1.26 SK1) | office.json | served |
| POST | `/hook` | hook | hook event; headers `x-agentdeck-agent`, `-tab`, `-tty`, `-project`, `-pid`, `-parent`, `-at`, `-billing` | empty | 204, 400 (a header twice), 401, 404 (not local, not POST) | office.json, frontdesk/hook-events.json | served |
| GET | `/events` | phone | `?t=` | event stream: `data:` the snapshot at once and at each change, `event: ping` | 200, 401 | state.json | served |
| POST | `/focus` | phone | `?session=`; a hire in T3 Code's desk opens its own thread in T3 Code (T3D-a) | empty, or text | 204, 404, 409 | office.json | served |
| POST | `/act` | phone | `?action=` accept, send, reject, esc, up, down, clear, button, effort, scroll, mic, swap (`&from=&to=`, and `&floor=` 0 to 5 for the floor shown, 0 when left out), style, music, fire, hire, move, seen, language, cleanup, settings, choose, letgo (a hire in T3 Code's desk, T3D-a; any other action there is 409; on a desk in a terminal Smolcorp can only watch, every control, the effort dial included, is 409 "Smolcorp can't do that in this terminal yet.": FXC2b)…; `&session=` | empty, or text | 204, 400, 403, 404, 409 | office.json | served |
| POST | `/summary` | phone | `?session=` | `{summary}` or `{error}` | 200, 400–503 `{error}` | office.json | served |
| GET | `/approvals` | phone | — | `{approvals, at}` | 200, 403 (not Tailscale) | office.json | served |
| GET | `/conversation` | phone | `?session=&before=&size=` | a page of the conversation | 200, 204, 400, 403, 404 | office.json | served |
| GET | `/session/info` | phone | `?id=` or `?session=` | account, model, time, tokens | 200, 403, 404 | office.json | served |
| POST | `/dictate` | phone | `{text, send}`, over Tailscale, or the Wi-Fi with Typing over Wi-Fi on (I34, SECWIFI); several lines as one paste (tmux by the pane's flag; Terminal.app, iTerm2 and Ghostty through the clipboard and ⌘V, Claude Code only) | empty or text | 204, 400, 403 (the office's window, the phone mic off, Typing over Wi-Fi off, no Accessibility for ⌘V), 404, 409 (a hire in T3 Code's desk: nothing types into it; asking; several lines where its terminal or agent can't take one paste, a tmux pane in copy mode, the Mac locked for ⌘V; through the clipboard: working, not in front of its tab, something else reading the tab's keys (an editor, a pager, a shell, anything else), the tab not known, not brought or kept in front, the Mac in use, the clipboard can't be put back, another paste still on it), 503 (the terminal stopped answering; ⌘V went but the paste wasn't read in time, no Return) | office.json | served |
| POST | `/peek` | phone | `?session=` | the input line | 200, 403 | office.json | served |
| POST | `/choose` | phone | `?option=other&choice=`, `{text}`, over Tailscale, or the Wi-Fi with Typing over Wi-Fi on (I34, SECWIFI), one line | empty | 204, 400 (several lines too), 403 | office.json | served |
| GET | `/choice` | phone | `?session=` | a prompt's command or file | 200, 403 | office.json | served |
| GET | `/t3code` | window | `?session=&fresh=1` | `{installed, …, hire}`, and `lane` for a hire in T3 Code | 200, 403 (a phone), 404 (T3 Code off) | office.json | served |
| POST | `/t3code/open` | window | `?session=` | empty or text | 204, 403, 404 (T3 Code off, not a hire), 409 | office.json | served |
| GET | `/hire/window` | window | — | `{background}`: whether hires run in the background | 200, 403 (a phone) | office.json | served |
| POST | `/hire/window` | window | `?session=` | empty or text: a window attached to the hire's tmux session | 204, 403 (a phone), 404 (not a hire), 409 | office.json | served |
| POST | `/bots/<id>/event` | page, tool | `{type: report or seen or start or stop, text, count, at}` | empty | 204, 400, 401, 404 | office.json | served |

## Setup: the page, the state and its switches

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| GET | `/setup` | page, tool | — | HTML: setup.html | 200, 404 (not local) | none: a file of the app's web folder, which the test bundle doesn't have | served |
| POST | `/setup/session` | page, tool | the menu link's key in `x-smolcorp-setup-key` | empty, `Set-Cookie`; a link's token used a second time: `{usedFirst, message, changed}` (what it did, in words) and the session it opened ends; every later use of that token, until it is forgotten, gets the same 409 (1.26, SK2). The warning stays in `/setup/state` as `usedFirst` (`at`, `times`, `changed`, `clearIn`, window or tray) and in the menu (`usedFirst` in its state) until the owner clears it | 204, 401, 409 | setup.json | served |
| GET | `/setup/state` | page, tool | — | the setup state (a tool: no code, no webhook secret) | 200, 401 (`X-Smolcorp-App: mac`) | state.json | differs |
| GET | `/setup/preview` | page, tool | — | HTML: deck.html, framed by this deck only | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/setup/selftest` | page, tool | — (or the check-only key in `x-smolcorp-selftest-key`, which opens this route alone) | `{checks: […], …}` | 200 | none: runs the self-test, which looks at the hooks, the login item and the agents' own commands | served |
| POST | `/setup/fix` | page, tool | `?fix=&auto=1` | the self-test | 200, 400, 403 (a tool: repair only), 409 | setup.json | served |
| POST | `/setup/scroll` | page, tool | `?enabled=` | the setup state | 200, 409 (Go: while the records' key can't be read, a change that would come back at the restart, as anything but a plain setting switched off, changed or taken off: 1.26 SK1F) | setup.json | differs |
| POST | `/setup/maxconfirm` | gm, page | `?enabled=` | the setup state | 200 | setup.json | differs |
| POST | `/setup/guide` | page, tool | `?done=` | the setup state | 200 | setup.json | differs |
| POST | `/setup/music` | page, tool | `?enabled=` | the setup state | 200 | none: asks the music app what's playing | served |
| POST | `/setup/phonemic` | gm, page | `?enabled=&autosend=` | the setup state | 200, 409 (Go: while the records' key can't be read, a change that would come back at the restart, as anything but a plain setting switched off, changed or taken off: 1.26 SK1F) | setup.json | differs |
| POST | `/setup/dock` | page, tool | `?enabled=` | the setup state | 200 | none: shows or hides the app in the Dock | served (under the Mac app, through the bridge; 404 on Linux and Windows) |
| POST | `/setup/ontop` | page, tool | `?enabled=` | the setup state | 200 | none: puts the office's window on top, on Windows, and under the Mac app its own window (FXC1b) | Windows only (and under the Mac app, through the bridge; 404 on Linux: no window) |
| POST | `/setup/office` | page, tool | — | empty | 204 | none: opens the office's window, or the browser | served (opens the browser; under the Mac app, its office window through the bridge) |
| POST | `/setup/login` | gm, page | `?enabled=` | the setup state | 200 | none: writes the login item in your home folder | served |
| POST | `/setup/accessibility` | page, tool | — | empty | 204 | none: asks macOS for Accessibility, a system prompt | served (under the Mac app, through the bridge; 404 on Linux and Windows) |
| POST | `/setup/hooks` | gm, page | `?remove=1` to disconnect | the setup state (`notice`) | 200, 409 (test copy), 500, 409 (Go: while the records' key can't be read, a change that would come back at the restart, as anything but a plain setting switched off, changed or taken off: 1.26 SK1F) | none: writes the agents' hooks in your home folder | served |
| POST | `/setup/logo` | page, tool | `?logo=smolcorp or keys` | the setup state | 200, 400 | setup.json | differs |
| GET | `/setup/logo/<logo>` | page, tool | — | PNG | 200, 404 | setup.json | served |
| POST | `/setup/theme` | page, tool | colours | empty | 204 | setup.json | served |
| POST | `/setup/layout` | gm, page | a layout | empty | 204, 400 | setup.json | served |
| POST | `/setup/pin` | gm, page | `?key=&folder=&floor=` (floor 0 to 5, 0 when left out) | empty | 204, 400 | setup.json | served |
| POST | `/setup/unpin` | gm, page | `?key=&floor=` (floor 0 to 5, 0 when left out) | empty | 204, 400 | setup.json | served |
| POST | `/setup/floors` | gm, page | `{floors?, rules?}` | the setup state | 200, 400 | setup.json | differs |
| POST | `/setup/profiles` | gm, page | `?action=switch or save or rename or delete or import or check&name=&to=` | the setup state; check: the profile | 200, 400 | setup.json, frontdesk/profiles.json | differs |
| POST | `/setup/webhook` | gm, page | `{enabled, url, secret, outside, events}` | the setup state | 200, 400, 409 (Go: while the records' key can't be read, a change that doesn't leave it as saved, so nothing taken away is saved until the owner unlocks the keychain and restarts: 1.26 SK1) | setup.json | differs |
| POST | `/setup/webhook/test` | gm, page | — | the setup state | 200 | none: sends a webhook to its address | served |
| POST | `/setup/pair` | page | — | the setup state with the code | 200 | setup.json | differs |
| POST | `/setup/unpair` | gm, page | `?phone=` or `?all=1` | the setup state | 200, 400, 409 (Go: while the records' key can't be read, so nothing taken away is saved until the owner unlocks the keychain and restarts: 1.26 SK1) | setup.json | differs |
| GET | `/setup/qr.png` | page | `?via=tailnet` | PNG | 200, 404 (no code) | setup.json | served |
| POST | `/setup/summaries` | page | `?enabled=&confirmed=1&provider=` | the setup state | 200, 400, 409 (Go: off while the records' key can't be read, so nothing taken away is saved until the owner unlocks the keychain and restarts: 1.26 SK1) | setup.json | differs |
| POST | `/setup/summary/provider` | page | `?id=&confirmed=1` | the setup state | 200, 400, 409 (Go: while the records' key can't be read, a change that would come back at the restart, as anything but a plain setting switched off, changed or taken off: 1.26 SK1F) | setup.json | served |
| POST | `/setup/summary/check` | page, tool | — | the setup state | 200 | none: runs each coding agent's own command | served |
| POST | `/setup/summary/key` | page | `{key}` or `?remove=1` | the setup state | 200, 400 | setup.json | served |
| POST | `/setup/dictation` | gm, page | `?enabled=&language=&claude=&cleanup=` | the setup state | 200, 400, 409 (Go: while the records' key can't be read, a change that would come back at the restart, as anything but a plain setting switched off, changed or taken off: 1.26 SK1F) | setup.json | served |
| POST | `/setup/dictation/glossary` | page, tool | `[names]` | the setup state | 200, 400, 409 (Go: while the records' key can't be read, a change that would come back at the restart, as anything but a plain setting switched off, changed or taken off: 1.26 SK1F) | setup.json | served |
| POST | `/setup/keystrokes` | page | `?enabled=` | the setup state | 200, 400, 409 (Go: off while the records' key can't be read, so nothing taken away is saved until the owner unlocks the keychain and restarts: 1.26 SK1; `/setup/keystrokes/apps?remove=` too) | setup.json | served |
| POST | `/setup/keystrokes/apps` | page | `?add=` or `?remove=` | the setup state | 200, 400, 409 | setup.json | served |
| POST | `/setup/officekey` | page | `?keys=` recorded keys (`alt+cmd+O`), `default` or `off` | the setup state | 200, 400 (no ⌘, ⌥ or ⌃; the Mac's own keys, by what they type on the layout in use), 409 (macOS has them switched on, or won't let them be held) | setup.json | differs |
| POST | `/setup/terminal-command` | page | `?remove=1` | the setup state | 200, 409 | none: writes the smolcorp link and your shell's PATH line | served (under the Mac app, through the bridge; 404 on Linux and Windows) |
| GET | `/setup/claude-accounts` | gm, page, tool | — | `{found, most, profiles}` | 200 | setup.json | served |
| POST | `/setup/claude-accounts` | page | `{profiles: [{name, dir}]}` | as GET | 200, 400, 409 (Go: while the records' key can't be read, a change that would come back at the restart, as anything but a plain setting switched off, changed or taken off: 1.26 SK1F) | setup.json | served |
| GET | `/setup/guard` | gm, page, tool | — | `{extra, groups, limits}` | 200 | setup.json | served |
| POST | `/setup/guard` | page | the guard's groups and patterns | as GET | 200, 400, 409 (Go: the records' key can't be read, so nothing is saved until the owner unlocks the keychain and restarts: 1.26 SK1), 500 | setup.json | served |
| POST | `/setup/choices` | page | `?t3=&background=&asked=1`, each 0 or 1 | the setup state, and `windowsSaid` when it switched hires to the background | 200, 400 (background without tmux), 409 (Go: while the records' key can't be read, a change that would come back at the restart, as anything but a plain setting switched off, changed or taken off: 1.26 SK1F) | setup.json | differs |
| GET | `/setup/t3code` | page | `?fresh=1` | `{installed, …, lane}` | 200, 404 (T3 Code off) | setup.json (T3 Code off; with it on, it looks for the T3 Code app and runs its command: not asked) | served |
| POST | `/setup/t3code/open` | page | `{id}` | empty or text | 204, 404 (T3 Code off, no such hire), 409 | setup.json | served |
| POST | `/setup/t3code/letgo` | page | `{id}` | `{ok, recap, note, worktree, branch}` | 200, 404, 409 | setup.json | served |

## Setup: hiring, roles, briefs and bots

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| POST | `/setup/hiring` | gm, page | `?enabled=&folder=&agent=&terminal=&tmux=`; `?claudeUnsigned=1` the page alone (1.26, RT126SG2 B2: an unsigned Claude Code refused after a signed one ran is the one to start from now on; `hiring.claudeUnsigned` in `/setup/state` names it) | the setup state | 200, 400, 403 (`claudeUnsigned` from anyone but the page), 409 (Go: off while the records' key can't be read, so nothing taken away is saved until the owner unlocks the keychain and restarts: 1.26 SK1) | setup.json | differs |
| POST | `/setup/hiring/choose` | page | `?pick=1&for=floor` | the setup state; pick: `{picked}` | 200 | none: opens the system's folder picker | served (the system's picker; under the Mac app, its folder panel through the bridge) |
| GET | `/setup/hire-folder` | page, tool | `?check=` | `{exists, fix, folder, home, problem, suggested}` | 200 | setup.json | served |
| POST | `/setup/hire-folder` | gm, page | `?folder=&create=1` | as GET | 200, 400 | setup.json | served |
| GET | `/setup/hire/folders` | page, tool | — | `{default, recent}` | 200 | setup.json | served |
| POST | `/setup/hire` | gm, page | `{role, folder, account, model, floor, worktree, apiKey, task, answers, engine, scope}` (`engine: "t3"`: into T3 Code, experimental (Go: `Deck.hireIntoT3`); `scope`, Go only: the GM's own project, see the General Manager's section) | empty or `{ok, desk, session}` | 200, 204, 400, 404, 409 | setup.json, frontdesk/hire-guard.json | served |
| POST | `/setup/hire/gm` | gm, page | `?worktree=1&account=` | empty | 204, 409, and the hire's | setup.json | served |
| POST | `/setup/hire/window` | page | `{id}` | empty or text: a window attached to the hire's own tmux session | 204, 409 | setup.json | served |
| GET | `/setup/hire/gm/preflight` | page | `?account=` | `{ready, items}` | 200, 400 | setup.json | served |
| POST | `/setup/roles` | gm, page | the roles, whole | the setup state | 200, 400, 409 (Go: while the records' key can't be read, a change that would come back at the restart, as anything but a plain setting switched off, changed or taken off: 1.26 SK1F) | setup.json | differs |
| GET | `/setup/briefs` | page, tool | — | `{briefs}` | 200 | setup.json | served |
| GET | `/setup/briefs/<name>` | page, tool | — | the brief's markdown | 200, 404 | none: reads one of the app's briefs, which the test bundle doesn't have (404 there) | 404, owner: M4a: briefs |
| POST | `/setup/briefs/preview` | page, tool | a role and folder | the brief as a hire would get it | 200, 400 | setup.json | served |
| POST | `/setup/briefs/restore` | page | — | the briefs | 200 | none: puts back the app's own briefs, which the test bundle doesn't have | served |
| GET | `/setup/bots` | page, tool | — | `{bots}` | 200 | setup.json | served |
| POST | `/setup/bots` | gm, page | `{bots: [...]}` | `{bots}` | 200, 400, 409 (Go: while the records' key can't be read, a change that would come back at the restart, as anything but a plain setting switched off, changed or taken off: 1.26 SK1F) | setup.json | served |
| POST | `/setup/bots/override` | gm, page | `{id, pinned, floor, hidden, show, autoDetect, botFloor}` | the setup state | 200, 400, 404, 409 (Go: while the records' key can't be read, a change that would come back at the restart, as anything but a plain setting switched off, changed or taken off: 1.26 SK1F) | setup.json | differs |
| POST | `/setup/bots/mailbox` | page | `{id, folder}` | `{bots}` | 200, 400, 409 (Go: while the records' key can't be read, a change that would come back at the restart, as anything but a plain setting switched off, changed or taken off: 1.26 SK1F) | setup.json | served |
| POST | `/setup/bots/resolve` | gm, page | — | `{bots, filled, unresolved}` | 200 | setup.json | served |

## Setup: the portable office's viewer

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| GET | `/setup/viewer` | page, tool | — | the viewer's state (the link for the page only) | 200 | none: asks the Keychain for the viewer's link | served (under the Mac app, its Keychain items through the bridge) |
| POST | `/setup/viewer` | page | `{baseURL, hideNames, stopOnReturn, stopAfterHours}` | the viewer's state | 200, 400, 409 (Go: while the records' key can't be read, a change that would come back at the restart, as anything but a plain setting switched off, changed or taken off: 1.26 SK1F) | setup.json | served |
| POST | `/setup/viewer/secret` | page | — | the viewer's state with `secret` | 200, 500 | none: makes a secret and keeps it in the Keychain | served (under the Mac app, its Keychain items through the bridge) |
| POST | `/setup/viewer/check` | page, tool | — | the viewer's state | 200 | none: asks the viewer's server | served |
| POST | `/setup/viewer/switch` | page | `?on=1` | the viewer's state | 200 | none: starts or stops sending the office to the viewer's server | served (under the Mac app, its Keychain items and when you're back, through the bridge) |
| POST | `/setup/viewer/rotate` | page | — | the viewer's state | 200 | none: makes a new link with the viewer's server | served |
| POST | `/setup/viewer/wipe` | page | — | the viewer's state | 200 | none: wipes the office from the viewer's server | served |
| GET | `/setup/viewer/qr.png` | page | — | PNG | 200, 404 | none: draws the viewer's link, from the Keychain | served (under the Mac app, its Keychain items through the bridge) |

## Setup: the General Manager (/setup/gm/, 409 while the manager is off but for version and the switch)

**A GM's scope (Go only).** The owner's `POST /setup/hire` of the gm role may send `scope`, the folder it hires
the GM into (400 for another role, another folder, or one that isn't there; 409 into T3 Code, or where the hire
wouldn't go through the hire engine in tmux). The Go app keeps it in that GM's registry row (`scope`, written only
when set) and holds every request made with its token to it (`desktop/gmscope.go`): `POST /setup/gm/hire` only
into that folder or the top of one of its worktrees, a folder that is there, never `apiKey` or `engine: "t3"`,
and on the Claude account the GM was hired on (`account` that one or left out, when the app fills it in); `send`, `brief`, `fire`, `contacts`, `nickname`, `/setup/pin`, `/setup/unpin`, `watch` and
`unwatch` only for its own hires, folders and watches; any other route but a GET is refused. Each refusal is
403 with one sentence, before the route. A GM with no scope gets every answer below as before. The Mac's Swift
engine had no scope and ignored the field; since 1.25.0 a Mac runs the Go app, so its GM has it too. `smolcorp demo`
hires its GM so (`desktop/demooffice.go`).

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| GET | `/setup/gm/version` | gm, page, tool | — | `{features, managerEnabled, version}` | 200 | gm.json, frontdesk/version.json | served |
| POST | `/setup/gm/manager` | gm, page | `?enabled=1 or 0` | `{managerEnabled}` | 200, 400, 409 (Go: off while the records' key can't be read, so nothing taken away is saved until the owner unlocks the keychain and restarts: 1.26 SK1) | gm.json | served |
| GET | `/setup/gm/team` | gm, page, tool | — | `{at, contacts, team}` (a hire in T3 Code's row: `engine`, `t3Thread`, and `t3` for the page) | 200, 409 | gm.json, frontdesk/team.json | served |
| GET | `/setup/gm/events` | gm, page, tool | `?since=&wait=&limit=` | `{events, next, oldest}` | 200, 409 | gm.json, frontdesk/events.json | served |
| GET | `/setup/gm/approvals` | gm, page, tool | — | `{approvals, at}` | 200, 409 | gm.json, frontdesk/approvals.json | differs |
| GET | `/setup/gm/outbox` | gm, page, tool | `?limit=` | `{messages}` (a tool: no texts) | 200, 409 | gm.json, frontdesk/outbox.json | served |
| GET | `/setup/gm/watches` | gm, page, tool | — | `{watches}` | 200, 409 | gm.json, frontdesk/watches.json | served |
| POST | `/setup/gm/watch` | gm, page | `{kind, target, pattern, minutes, repeat}` | the watch | 200, 400, 409 | gm.json, frontdesk/watch.json | served |
| POST | `/setup/gm/unwatch` | gm, page | `?id=` or `{id}` | `{id, removed}` | 200, 400, 404, 409 | gm.json | served |
| DELETE | `/setup/gm/unwatch` | gm, page | as POST | as POST | as POST | gm.json | served |
| GET | `/setup/gm/screens` | gm, page, tool | — | `{ask, chosen, screens}` | 200, 409 | gm.json | served (under the Mac app, through the bridge; 404 on Linux and Windows) |
| POST | `/setup/gm/screen` | gm, page | `?id=` | as GET | 200, 400, 409 | gm.json | served (under the Mac app, through the bridge; 404 on Linux and Windows) |
| POST | `/setup/gm/tile` | gm, page | — | the tiling's report | 200, 409 | none: tiles the team's windows on a screen, which moves them; with the manager off, both apps answer 409 first, and a test copy answers 409 too, whatever the screens | served (under the Mac app, through the bridge; 404 on Linux and Windows) |
| POST | `/setup/gm/hire` | gm, page | as `/setup/hire`, folder optional | `{ok, desk, session}` | 200, 400, 403, 404, 409 | gm.json, frontdesk/hire.json | served |
| POST | `/setup/gm/fire` | gm, page | `{id, force, removeWorktree}` | empty, or `{ok, kept, worktree, branch}` when removal was asked and the worktree stayed, or `{ok, note}` (and `kept`…) when a hire in a window of its own was marked fired with no window closed | 200, 204, 400, 403, 404, 409 (Go also: while the records' key can't be read, the hire's session or window is ended and its key stopped for the run, but the fire isn't saved, and the answer says so; one hired in that run is fired in full, `{ok, note}` saying so: 1.26 SK1, SK1F) | gm.json, frontdesk/fire.json | served |
| POST | `/setup/gm/brief` | gm, page | `{id, text, file}` | `{ok, …}` | 200, 400, 404, 409 | gm.json, frontdesk/brief.json | served |
| POST | `/setup/gm/send` | gm, page, hire | `{to, text, …}` | the message | 200, 400, 403, 404, 409 | gm.json, frontdesk/send.json | served |
| POST | `/setup/gm/contacts` | gm, page | `{pairs, remove}` | `{contacts}` | 200, 400, 409 (Go also: a pair removed while the records' key can't be read, so nothing taken away is saved until the owner unlocks the keychain and restarts: 1.26 SK1) | gm.json, frontdesk/contacts.json | served |
| POST | `/setup/gm/registry` | gm, page | `{id, nickname, state, notes}` | the hire | 200, 400, 404, 409 (Go also: fired or failed while the records' key can't be read, so nothing taken away is saved until the owner unlocks the keychain and restarts: 1.26 SK1) | gm.json, frontdesk/registry-edit.json | served |
| GET | `/setup/gm/registry` | page | — | nothing: listed for the page, not answered | 404 | gm.json | served |
| POST | `/setup/gm/nickname` | gm, page | `{project or folder, nickname}` | `{project, nickname}` | 200, 400, 404, 409 | gm.json | served |
| POST | `/setup/gm/by-hand` | page | `?enabled=1 or 0` | `{gmByHand}` | 200, 400, 409 (off while the records' key can't be read and on in the file, so nothing taken away is saved until the owner unlocks the keychain and restarts: 1.26 SK1) | gm.json | served (under the Mac app, in the core; 404 on Linux and Windows) |
| POST | `/setup/gm/token/rotate` | page | — | `{rotated, message}`, never the token | 200, 409 (the records' key can't be read, so no new token is kept, and the old one still works: 1.26 SK1), 500 | gm.json, frontdesk/gm-token-rotate-as-page.json | served (under the Mac app, in the core; 404 on Linux and Windows) |
| * | `/setup/gm/*` | gm, page | a route not listed above | — | 403 for a tool, 404 | gm.json | served |

## Served by the Go app only

| Method | Path | What | Mac |
|---|---|---|---|
| GET | `/launcher` | a launcher's connection (`smolcorp run`), the hook key: the deck's keys as NDJSON | none: the Mac has no launcher |
| GET | `/launcher/start` | an engine hire's start environment, once | none |
| POST | `/launcher/screen` | a launcher's answer to a look at its window | none |
| POST | `/raise` | a second Smolcorp asking this one to show the office | none |
| GET | `/office?o=` | the tray's one-time token swapped for the window key (303) | the Mac's window has its key |
| GET | `/sign.js` | what signs a paired phone's requests (SECWIFI), anyone, like `/office.js` | the Mac's too, as it runs the Go app |
| GET | `/paired` | a phone's signed check that it's still paired, and of its clock (SECWIFI): 204, 401 `{error, why, now}` | the Mac's too, as it runs the Go app |
| POST | `/setup/wifityping` | Typing over Wi-Fi (SECWIFI), the page alone: `?on=`, the setup state (409 for off while the records' key can't be read, so nothing taken away is saved until the owner unlocks the keychain and restarts: 1.26 SK1) | the Mac's too, as it runs the Go app |
| POST | `/setup/t3code/takeover` | Take over in T3 Code (EXPERIMENTAL, M5b), the page alone: `{id}`, a hire in a tmux session of its own goes on, the same session, in T3 Code; `{ok, thread, tag}`, 200, 404, 409, 500, 502 | the Mac's too since 1.25.0, as it runs the Go app (the Swift engine had no Take over) |
| POST | `/setup/t3code/handback` | Hand back (EXPERIMENTAL, M5b), the page alone: `{id}`, T3 Code lets go, the session is checked free, then resumed in its tmux session; `{ok, note}`, 200, 404, 409, 500, 502 | the Mac's too since 1.25.0 (the Swift engine had no Hand back) |
| GET | `/setup/tailnet/status` | Phone over HTTPS (1.26, FR2), on every system, the page alone: where Tailscale stands, `{status (checking, notInstalled, signedOut, off or on), address, port, command, offCommand, busy, notHere, onAgain}`; `command` and `offCommand` are the lines to run in a terminal, which on Linux start with `sudo` (OLD-SERVE1F: the one line as root, never the operator setting), while the tap runs them without; `notHere` is null on a Mac, Linux and Windows (since OLD-SERVE2), and says HTTPS through tailscale serve isn't on this system only where the hand-on isn't let in; `onAgain` (SN1, a Mac and Linux; OLD-SERVE2, Windows, its own sentence) is null, or the sentence asking to turn it on again once while serve still hands the app's own TCP port on, from `tailscale status --json` and `tailscale serve status --json`, read only; 200 | the Mac's too, as it runs the Go app (the Swift engine had no such status) |
| GET | `/setup/tailnet/serve?action=` | the confirmation before the tap, the page alone: `{action, command, exposes: [...]}`, the exact command and what it exposes (this app, through `serve.sock` on a Mac since SN1 and on Linux since OLD-SERVE1, through the serve port, this app's port + 1 on `127.0.0.1`, on Windows since OLD-SERVE2, on the tailnet only, never Funnel); runs nothing; 200, 400, 403 (`action=on` where the hand-on isn't let in) | the Mac's too |
| POST | `/setup/tailnet/serve` | the tap, the page alone (never the GM, a hire, a tool, MCP or a phone): `{action (on or off), command}`, runs exactly `tailscale serve --bg --https=443 unix:<support folder>/serve.sock` (SN1, a Mac, and OLD-SERVE1, Linux; `http://127.0.0.1:<this app's port + 1>` on Windows, the serve port, OLD-SERVE2) or `tailscale serve --https=443 off`, only if `command` is the line the confirmation showed; never `funnel`, never another port; `{status, ...}` 200, 400, 403 (`on` where the hand-on isn't let in), 409 (not the command, not installed or signed in, already on, or not handed on by this app), 428 (no command sent back), 502 (Tailscale refused) | the Mac's too |
| GET | `/setup/night` | 🌙 the night shift's jobs (1.26, N1), for the page, the GM and a tool: `{jobs, skipped, running, most}`, each job with its `actor` (`night:<id>`) and, approved and on a clock, its `next` time; 200 | the Mac's too, as it runs the Go app |
| POST | `/setup/night/propose` | 🌙 a job proposed (1.26, N1), the GM or the page: `{name, kind, role, brief, folder, when, limits}`, nothing else; kept as `proposed`, never due until the page approves it; a GM held to one project only there; 200, 400, 403, 409 | the Mac's too |
| POST | `/setup/night/save` | 🌙 a job made (no `id`: approved, or `status: paused`) or changed (its status kept), the page alone; 200, 400, 404, 409 | the Mac's too |
| POST | `/setup/night/approve`, `/setup/night/pause`, `/setup/night/delete` | 🌙 `?id=`, the page alone; 200, 404 | the Mac's too |
| GET | `/talk` | 💬 the talk bubble's card of one desk (1.26, P1), the office's own window on this computer only (403 from a phone, 401 without a key): `?session=`, `{state, asking, last, size}`; `asking` is null unless it asks, with `since`, `kind`, `tool`, `label`, `questions`, `pickable`, never a tool's input; 200, 404 | the Mac's too, as it runs the Go app (the Swift engine had no talk bubble) |
| GET | `/talk/asking` | 💬 every desk waiting on you, for the small bubbles (1.26, P1), the window only: `{asking: [{session, …asking}]}`, the longest waiting first; 200 | the Mac's too, as it runs the Go app |
| POST | `/talk/pick` | 💬 a choice picked in the bubble (1.26, P3), the window only: `?session=&n=&since=`, a question's option or a permission's Yes as its digit, No as Escape, Claude Code only; pressed once, only while that prompt is asked and its menu is on the desk's screen (RT126P3 B1); 204, 400, 404, 409 (`choiceGone`; not pickable: the four keys; a hire in T3 Code, headless, a test copy's) | the Mac's too, as it runs the Go app |
| POST | `/talk/listen` | 🎙 the mic in a desk's input box (1.26, FD1), the window only: `?session=&phase=start` has the Mac record from its own microphone for that desk's box, `phase=stop` ends it; 204 (a stop with nothing recording is a 204 too), 400 (no phase), 404, 409 (`dictationCouldntStart`; a hire in T3 Code, headless; Linux and Windows: "Dictation isn't available on this computer."). What it hears goes to the window's event stream alone, as `event: dictation` with `{session, phase: listening\|partial\|final, text}`, never in the state; nothing is typed in the tab until Send (`/talk/say`) | the Mac's too, as it runs the Go app (the Swift engine had no input-box dictation) |
| GET | `/grants` | the owner's asks (1.26, FS2): what a GM, an MCP key, a night job or a hire waits to be allowed, `{at, asks: [{id, by, what, amount, since, waitingSeconds, expires}]}`, the longest waiting first, each shown whole (an ask past 2,000 bytes, or with a control, line break, direction mark, invisible or default-ignorable character, a blank that isn't a space or is drawn as nothing, a mark drawn over a letter, more than three marks on one letter, a letter in a styled or wide form, look-alike scripts in one word, or a Cyrillic or Greek word that reads as a Latin one beside Latin text, is a no and never listed); a paired phone over Tailscale or the Mac app's own office window (403 from a phone on the Wi-Fi, the office in a browser, any window off a Mac, the paired device the Windows window holds, this computer's own Tailscale address, and off a Mac the loopback with a proxy's headers; 401 without a key); the same list is `asks` in `GET /approvals` and `GET /setup/gm/approvals`; 200 | the Mac's too, as it runs the Go app |
| POST | `/grants/answer` | the owner's yes or no to one ask (1.26, FS2): `?id=&answer=yes\|no`, a paired phone over Tailscale or the Mac app's office window alone, each with its key; a grant is for that one ask, used once, and an ask nobody answers runs out at 12 h as a no; a restart voids every ask and grant (their key is in memory alone); a no stands 10 minutes; 204, 400, 403, 404 (no such ask), 409 (answered already, or run out), 507 (no room) | the Mac's too, as it runs the Go app |
| POST | `/setup/grants/answer` | the same from the setup page, its cookie alone (403 for a tool's setup key, the GM's token and a hire's key) | the Mac's too, as it runs the Go app |
| POST, GET | `/terminal/ask` | a terminal asks for a key (1.26, SK2), from this computer, no key: POST `?kind=gm\|tool&folder=` with `X-AgentDeck-Setup: 1` answers `{ticket, code, expiresIn, where}` (`gm`: under the Mac app with "Allow a General Manager started by hand" on, else 403; `tool`: under the Mac app only, else 403 with why, as `/setup/state`'s `terminalAsksOff` says it: off a Mac only the paired phone, and on Windows Smolcorp's own window, may answer, and neither can yet; at most 3 asks wait, then 429); GET `?ticket=` waits up to 20 s and answers `{state: waiting\|allowed\|denied}`, with `key` once when allowed (the GM's new token, or a tool's key until Smolcorp quits); 404 for a ticket run out (3 min) or taken. The page's `/setup/state` lists the asks waiting as `terminalAsks` (`id`, `kind`, `folder`, `code`, `askedAt`), never a ticket or a key | the Mac's too, as it runs the Go app |
| POST | `/setup/terminal/answer` | the owner's answer to a terminal's ask, the page alone, and under the Mac app Smolcorp's own window alone (403 for Setup in a browser, and for everyone off a Mac): `?id=&code=&answer=allow\|deny`, the code the page showed; 200, 400, 403, 404 (no such ask waits with that code), 409 (the key couldn't be made) | the Mac's too, as it runs the Go app |
| POST | `/setup/used-first/seen` | "I've seen it" for a Setup link used first (1.26, SK2): Smolcorp's own Settings window alone, under the Mac app; 403 for a browser's Setup and off a Mac, where the tray's "I've seen it" clears it; 200 with the state | the Mac's too, as it runs the Go app |
| POST | `/setup/records/reset` | Smolcorp's records start over once the key that seals them failed its check (1.26, SK1: overwritten, a second one planted, an update interrupted, or unreadable without a prompt), the setup page's cookie alone: the Mac app makes a new key, every phone, paired Mac, hire's send key and the GM's token start over, and the rest is sealed under the new key as the core holds it (a `registry.json` that couldn't be read set aside as `registry.broken.json` first, never written over): the team, its desks and worktrees (RT126SK1 B5), and, after a check that passed, its contacts, the settings and the guard. After a failed check that is what the check left (RT126SK1R2 N2): no contacts, the security settings at their safe defaults, the guard as the built-in one plus what its file adds, and the GM and any hire held to a project let go; the page's state; 200, 403 (anyone else), 409 (no Mac app, or the Keychain made no key); `records` in the page's `/setup/state` says why nothing counts, or that a new key couldn't be made after something was taken away, only while that is so, with `recordsReset`, what the Reset's confirm asks; `recordsDropped` names a record that didn't fit at this start though the check passed (rotated away as the start ended) | the Mac's too, as it runs the Go app |
| POST | `/act?action=takeover`, `/act?action=handback` | the same from a desk's card, the office's own window only (403 from a phone, 404 for a desk with no hire); `GET /t3code` and `GET /setup/t3code` say `takeOver` where it's offered, a T3 desk's `t3.handBack` and a team row's `handBack` only for a hire taken over | the Mac's too since 1.25.0 |
| GET, POST | `/setup/peers` | Floors from other Macs (1.26, M1), the page alone: the setting (`?enabled=1 or 0`, off by default), and `{on, code, expiresIn, peers, joined, storeReadable}`, never a key; 200, 400, 409 (off while the records' key can't be read, so nothing taken away is saved until the owner unlocks the keychain and restarts: 1.26 SK1) | the Mac's too, as it runs the Go app |
| POST | `/setup/peers/code`, `/setup/peers/revoke?peer=`, `/setup/peers/join` | the page alone: a 6-digit code for another Mac (409 while off, or while the records' key failed its check: 1.26 SK1); remove a Mac paired here, its next request refused (404 for none; 409 while the records' key can't be read, so nothing taken away is saved until the owner unlocks the keychain and restarts: 1.26 SK1); join another with `{address, code, name}`, a Tailscale address or ts.net name only (400 otherwise), the key kept in the secret store (503 without one, nothing sent) | the Mac's too: the key kept in the Mac app's own Keychain item `peer-keys`, read only as the app added it and never "none" while it's there but can't be read (503, nothing sent: 1.26 FL0) |
| POST | `/peer/pair` | another Mac's code, `{code, name}` → `{key, name}`: with the setting on, over Tailscale only (403 otherwise, the Wi-Fi too), the phone code's limits (403; 429 for a second try within a second) | the Mac's too |
| GET | `/peer/hello` | the `peer` class (`X-Smolcorp-Peer` key, over Tailscale, setting on): `{name}`; a peer key opens no other route (403), and no other key opens this one | the Mac's too |
| GET | `/push` | 🔔 a phone's notifications (1.26, PUSH1), a paired phone over Tailscale only, as `/approvals` (403 from the Wi-Fi and the office's window; through `tailscale serve` only with the phone's typing on, as every phone route there): `{key, on}`, the app's public VAPID key to subscribe with (made the first time it's asked, and kept: under the Mac app its own Keychain item `push-key`, through the bridge; elsewhere the secret store, else `push-key-kept`) and whether this phone has a subscription kept; never the private key; 200, 503 (nowhere to read or keep the key: a Mac without the app, or a store that won't answer) | the Mac's too, as it runs the Go app (the Swift engine had no notifications) |
| POST | `/push` | 🔔 this phone's subscription, as the browser's `PushSubscription.toJSON()` gives it (`{endpoint, keys: {p256dh, auth}}`), the same class: kept on the phone that posted it, in place of any it had, in the sealed `phones.json`, the one it replaces under a new record key; an address another paired phone holds is refused (409), that phone's left as it was; an `https` address on Apple's, Google's, Mozilla's or Microsoft's push service only (the owner's yes, F4), a P-256 key on the curve and a 16-byte auth secret; dropped when the phone is unpaired, or when the list's seal doesn't fit; 204, 400, 409 (another phone's address, or the records can't take it now) | the Mac's too |
| POST | `/push/off` | 🔔 this phone's subscription forgotten, the same class, under a new record key (a copy from before doesn't fit); another phone's is never touched; 204 | the Mac's too |


## Fields of `/setup/state` (shape `setup-state`, the page's)

Two levels down; `layout`, `theme` and the roles' lists are compared whole by the shape. "one of …" is a closed set;
"x or null" may be null; "array" and "map" are compared no further (see "Not compared"). Fields only the Go app
has are the known gaps' "extra" rows.

| Field | Type | Go |
|---|---|---|
| `.activeProfile` | string | same |
| `.agents` | array | same |
| `.checklist` | object | same |
| `.checklist.accessibility` | bool | same |
| `.checklist.hooks` | bool | same |
| `.checklist.openAtLogin` | bool | same |
| `.checklist.scroll` | bool | same |
| `.choices` | object | same |
| `.choices.asked` | bool | same |
| `.choices.background` | bool | same |
| `.choices.hasTmux` | bool | same |
| `.choices.t3Code` | bool | same |
| `.choices.t3Installed` | bool | same |
| `.claudeAccounts` | object | same |
| `.claudeAccounts.found` | array | same |
| `.claudeAccounts.most` | number | same |
| `.claudeAccounts.profiles` | array | same |
| `.cloudBots` | object | same |
| `.cloudBots.autoDetect` | bool | same |
| `.cloudBots.botFloor` | number | same |
| `.cloudBots.bots` | array | same |
| `.cloudBots.detects` | bool | same |
| `.cloudBots.hidden` | [] | same |
| `.cloudBots.noRoom` | [] | same |
| `.dictation` | object | same |
| `.dictation.allowed` | bool | same |
| `.dictation.available` | bool | same |
| `.dictation.claude` | bool | same |
| `.dictation.cleanup` | bool | same |
| `.dictation.cleanupProblem` | string or null | same |
| `.dictation.enabled` | bool | same |
| `.dictation.glossary` | array | same |
| `.dictation.language` | string | same |
| `.dictation.problem` | string or null | same |
| `.dock` | object | same |
| `.dock.enabled` | bool | same |
| `.floorPins` | array | same |
| `.floorPins[].desk` | number | same |
| `.floorPins[].floor` | number | same |
| `.floorPins[].folder` | string | same |
| `.floorPins[].name` | string | same |
| `.floorRules` | array | same |
| `.floorRules[].exact` | bool | same |
| `.floorRules[].floor` | number | same |
| `.floorRules[].folder` | string | same |
| `.floors` | array | same |
| `.floors[].name` | string | same |
| `.floors[].preset` | string | same |
| `.gmHired` | bool | same |
| `.guide` | object | same |
| `.guide.done` | bool | same |
| `.hiring` | object | same |
| `.hiring.agent` | one of 'claude', 'codex', 'gemini', 'opencode', 'grok' | same |
| `.hiring.apiKey` | object | same |
| `.hiring.canChoose` | bool | same |
| `.hiring.canHire` | bool | same |
| `.hiring.enabled` | bool | same |
| `.hiring.folder` | string | same |
| `.hiring.hasTmux` | bool | same |
| `.hiring.terminal` | string | same |
| `.hiring.terminals` | array | same |
| `.hiring.tmux` | bool | same |
| `.keystrokes` | object | same |
| `.keystrokes.accessibility` | bool | same |
| `.keystrokes.apps` | [] | same |
| `.keystrokes.enabled` | bool | same |
| `.keystrokes.running` | array | same |
| `.layout` | object | same |
| `.logo` | one of 'smolcorp', 'keys' | same |
| `.manager` | object | same |
| `.manager.enabled` | bool | same |
| `.manager.gmByHand` | bool | same |
| `.manager.gmStartedByHand` | bool | same |
| `.manager.gmTokenFile` | string | same |
| `.manager.gmTokenFileShown` | string | same |
| `.maxConfirm` | object | same |
| `.maxConfirm.enabled` | bool | same |
| `.music` | object | same |
| `.music.enabled` | bool | same |
| `.officeKey` | object | same |
| `.officeKey.default` | string | same |
| `.officeKey.held` | bool | same |
| `.officeKey.keys` | string | same |
| `.officeKey.labels` | map | same |
| `.officeWindow` | object | same |
| `.officeWindow.available` | bool | same |
| `.officeWindow.mac` | bool | same |
| `.officeWindow.onTop` | bool | same |
| `.pairing` | object | same |
| `.pairing.address` | string | same |
| `.pairing.code` | string | same |
| `.pairing.codeExpiresIn` | number | same |
| `.pairing.open` | bool | same |
| `.pairing.tailnet` | string | same |
| `.phoneKeys` | number | same |
| `.phoneMic` | object | same |
| `.phoneMic.autoSend` | bool | same |
| `.phoneMic.enabled` | bool | same |
| `.phoneMic.serving` | bool or null | same |
| `.phoneMic.tailnet` | string or null | same |
| `.phones` | array | same |
| `.phones[].connected` | bool | same |
| `.phones[].id` | string | same |
| `.phones[].lastSeen` | number | same |
| `.phones[].name` | string | same |
| `.phones[].pairedAt` | number | same |
| `.pins` | array | same |
| `.pins[].folder` | string | same |
| `.pins[].name` | string | same |
| `.platform` | one of 'darwin', 'linux', 'windows' | same |
| `.presets` | array | same |
| `.profileEdited` | bool | same |
| `.profiles` | array | same |
| `.profiles[].colours` | map | same |
| `.profiles[].layout` | object | same |
| `.profiles[].name` | string | same |
| `.roles` | object | same |
| `.roles.agents` | array | same |
| `.roles.badges` | array | same |
| `.roles.efforts` | array | same |
| `.roles.limits` | object | same |
| `.roles.roles` | array | same |
| `.roles.shipped` | array | same |
| `.sessions` | array | same |
| `.sessions[].agent` | one of 'claude', 'codex', 'gemini', 'opencode', 'grok' | same |
| `.sessions[].agentName` | string | same |
| `.sessions[].folder` | string | same |
| `.sessions[].id` | string | same |
| `.sessions[].key` | number or null | same |
| `.sessions[].model` | string or null | same |
| `.sessions[].name` | string | same |
| `.sessions[].role` | string or null | same |
| `.sessions[].state` | one of 'idle', 'thinking', 'asking', 'unread', 'error' | same |
| `.summary` | object | same |
| `.summary.checking` | bool | same |
| `.summary.enabled` | bool | same |
| `.summary.fallback` | string or null | same |
| `.summary.key` | object or null | same |
| `.summary.model` | string | same |
| `.summary.provider` | object or null | same |
| `.summary.providers` | array | same |
| `.summary.saved` | string | same |
| `.summary.savedKey` | bool | same |
| `.t3CodeEnabled` | bool | same |
| `.terminalCommand` | object | same |
| `.terminalCommand.available` | bool | same |
| `.terminalCommand.link` | string | same |
| `.terminalCommand.onPath` | bool | same |
| `.terminalCommand.pathFile` | string | same |
| `.terminalCommand.pathLine` | string | same |
| `.terminalCommand.state` | string | same |
| `.terminalCommand.testCopy` | bool | same |
| `.theme` | object | same |
| `.version` | string | same |
| `.webhook` | object | same |
| `.webhook.enabled` | bool | same |
| `.webhook.events` | array | same |
| `.webhook.last` | object or null | same |
| `.webhook.outside` | bool | same |
| `.webhook.secret` | string | same |
| `.webhook.url` | string | same |

## Fields of the office snapshot (shape `snapshot`, `/events`' messages)

Two levels down (a desk's session: three), as above. A list's elements are compared each as it is, so a type
here is all of theirs: `.keys[].session.model` is "string or null" as the free key's desk has none, but the
pinned key's desk is "string" alone in the shape, and a null there is a difference. A hire in T3 Code's desk (T3D-a,
T3D-b on Go; the given's "a T3 hire", from the registry alone) is a key's session of its own shape: its `t3` (tag, branch, its
thread's status, whether it can be opened), every control false, and no turn's start (`turnStartedAt` null), as T3
Code doesn't say one. Its values are checked too, beside the shape (contract.json's `t3Desk`, FXT3Da): every `t3:`
desk an app answers has each control false, `choice` null, its `t3` with its tag, `experimental`, status, branch and `canOpen`, and
a `name` and `project` with no `/`.

Not in the table, as no given has them: `.hiring.unfenced` and `.hiring.roles[].unfenced`, the line saying a hire
won't get Smolcorp's fence. Both apps send them only where a hire can be made here, and the given has no folder
for hires (`.hiring.roles` is `[]`).

| Field | Type | Go |
|---|---|---|
| `.dictation` | object | same |
| `.dictation.cleanup` | bool | same |
| `.dictation.language` | string | same |
| `.dictation.on` | bool | same |
| `.floors` | array | same |
| `.floorsOverflow` | number | same |
| `.floors[].asking` | number | same |
| `.floors[].desks` | [] or array | same |
| `.floors[].n` | number | same |
| `.floors[].name` | string | same |
| `.floors[].pins` | array | same |
| `.floors[].preset` | string | same |
| `.hiring` | object | same |
| `.hiring.fire` | bool | same |
| `.hiring.hire` | bool | same |
| `.hiring.roles` | [] | same |
| `.keys` | array | same |
| `.keys[].pin` | string or null | same |
| `.keys[].session` | object | same |
| `.keys[].session.activity` | string | same |
| `.keys[].session.agent` | one of 'claude', 'codex', 'gemini', 'opencode', 'grok' | same |
| `.keys[].session.agentName` | string | same |
| `.keys[].session.choice` | object or null | same |
| `.keys[].session.compacting` | bool | same |
| `.keys[].session.conversation` | bool | same |
| `.keys[].session.dial` | bool | same |
| `.keys[].session.dictates` | bool | same |
| `.keys[].session.effort` | one of 'low', 'medium', 'high', 'xhigh', 'max', '' | same |
| `.keys[].session.errorKind` | string or null | same |
| `.keys[].session.fire` | bool | same |
| `.keys[].session.id` | string | same |
| `.keys[].session.keys` | bool | same |
| `.keys[].session.maxPressedAt` | number or null | same |
| `.keys[].session.messaged` | string or null | same |
| `.keys[].session.messagedAt` | number or null | same |
| `.keys[].session.mic` | bool | same |
| `.keys[].session.model` | string or null | same |
| `.keys[].session.name` | string | same |
| `.keys[].session.pending` | bool | same |
| `.keys[].session.phoneMic` | bool | same |
| `.keys[].session.project` | string | same |
| `.keys[].session.role` | object or null | same |
| `.keys[].session.startedAt` | number | same |
| `.keys[].session.state` | one of 'idle', 'thinking', 'asking', 'unread', 'error' | same |
| `.keys[].session.subagents` | array or [] | same |
| `.keys[].session.summary` | bool | same |
| `.keys[].session.t3` | object | same |
| `.keys[].session.tool` | string or null | same |
| `.keys[].session.turnStartedAt` | number or null | same |
| `.keys[].session.voiceNote` | object or null | same |
| `.layout` | object | same |
| `.maxConfirm` | bool | same |
| `.music` | object or null | same |
| `.now` | number | same |
| `.overflow` | number | same |
| `.phoneMic` | object | same |
| `.phoneMic.autoSend` | bool | same |
| `.phoneMic.on` | bool | same |
| `.scroll` | bool | same |
| `.theme` | object | same |
| `.upstairs` | object | same |
| `.upstairs.desks` | array | same |
| `.upstairs.overflow` | number | same |

## Not compared

Where a shape doesn't compare all the way down: a list this computer fills (its agents, terminals, accounts,
screens), a map whose keys are data, something the given can't set up (`object or null`), or a list empty in
every given (`[]`: the Go app's must be empty too). The Swift engine's ContractTests.swift checked this table was
every such spot, with its why, until M7. **In** is a shape's name or a route; a setup-state row is the tool's too. The nulls allowed are
in the next table.

| In | Where | Shape | Why |
|---|---|---|---|
| `GET /setup/bots` | `.bots[].resolved` | `object or null` | only a bot from the Grok Bot app is resolved: none in the given |
| `GET /setup/claude-accounts` | `.found` | `array` | the Claude accounts found in this Mac's home folder |
| `GET /setup/claude-accounts` | `.profiles` | `array` | the Claude accounts found in this Mac's home folder |
| `GET /setup/gm/screens` | `.screens` | `array` | this Mac's screens |
| `GET /setup/gm/team` | `.contacts` | `[]` | no pair of hires allowed to write to each other in the given |
| `GET /setup/guard` | `.extra` | `[]` | no pattern of your own in the given |
| `GET /setup/hire/folders` | `.recent` | `[]` | no folder hired into in the given |
| `POST /setup/bots/resolve` | `.bots[].resolved` | `object or null` | only a bot from the Grok Bot app is resolved: none in the given |
| `POST /setup/bots/resolve` | `.filled` | `[]` | only a bot from the Grok Bot app is filled in: none in the given |
| `POST /setup/bots/resolve` | `.unresolved` | `[]` | only a bot from the Grok Bot app can be unresolved: none in the given |
| `POST /setup/bots` | `.bots[].resolved` | `object or null` | only a bot from the Grok Bot app is resolved: none in the given |
| `setup-state` | `.agents` | `array` | the coding agents found in this computer's home folder |
| `setup-state` | `.claudeAccounts.found` | `array` | the Claude accounts found in this Mac's home folder |
| `setup-state` | `.claudeAccounts.profiles` | `array` | the Claude accounts found in this Mac's home folder |
| `setup-state` | `.cloudBots.hidden` | `[]` | no bot hidden in the given |
| `setup-state` | `.cloudBots.noRoom` | `[]` | every bot has a desk in the given |
| `setup-state` | `.hiring.terminals` | `array` | the terminals installed on this computer |
| `setup-state` | `.keystrokes.apps` | `[]` | no app allowed keys by default |
| `setup-state` | `.keystrokes.running` | `array` | the apps running on this Mac |
| `setup-state` | `.layout.room.dress.rules` | `[]` | no dress rule in the default layout |
| `setup-state` | `.layout.room.floorPlans` | `map` | a floor's plan by its number: keys are data |
| `setup-state` | `.layout.room.pet` | `[]` | no pet in the default layout |
| `setup-state` | `.layout.room.plan.added.tall` | `[]` | nothing added to the default plan |
| `setup-state` | `.layout.room.plan.added.wide` | `[]` | nothing added to the default plan |
| `setup-state` | `.layout.room.plan.hung` | `map` | what's hung where: keys are data |
| `setup-state` | `.layout.room.plan.tall` | `map` | a tall item's place by its id: keys are data |
| `setup-state` | `.layout.room.plan.wide` | `map` | a wide item's place by its id: keys are data |
| `setup-state` | `.layout.room.team` | `map` | the team's looks by folder name: keys are data |
| `setup-state` | `.officeKey.labels` | `map` | each key's name on the keyboard layout in use, by its US name, where they differ: keys are data |
| `setup-state` | `.profiles[].colours` | `map` | the theme's colours changed in a look, by name: none in the given |
| `setup-state` | `.profiles[].layout.room.dress.rules` | `[]` | no dress rule in the default layout |
| `setup-state` | `.profiles[].layout.room.floorPlans` | `map` | a floor's plan by its number: keys are data |
| `setup-state` | `.profiles[].layout.room.pet` | `[]` | no pet in the default layout |
| `setup-state` | `.profiles[].layout.room.plan.added.tall` | `[]` | nothing added to the default plan |
| `setup-state` | `.profiles[].layout.room.plan.added.wide` | `[]` | nothing added to the default plan |
| `setup-state` | `.profiles[].layout.room.plan.hung` | `map` | what's hung where: keys are data |
| `setup-state` | `.profiles[].layout.room.plan.tall` | `map` | a tall item's place by its id: keys are data |
| `setup-state` | `.profiles[].layout.room.plan.wide` | `map` | a wide item's place by its id: keys are data |
| `setup-state` | `.profiles[].layout.room.team` | `map` | the team's looks by folder name: keys are data |
| `setup-state` | `.summary.key` | `object or null` | where the API key is from: none in the given (no Keychain is asked) |
| `setup-state` | `.summary.provider` | `object or null` | the provider picked: none with summaries off, as in the given |
| `setup-state` | `.webhook.last` | `object or null` | the last webhook sent: none in the given |
| `snapshot` | `.floors[].desks[].choice` | `object or null` | a menu's options, read from the terminal: none in the given |
| `snapshot` | `.floors[].desks[].last` | `object or null` | a cloud bot's last report: none in the given |
| `snapshot` | `.floors[].desks[].subagents` | `[]` | only contract-a, at a key, has a subagent in the given |
| `snapshot` | `.floors[].desks[].voiceNote` | `object or null` | how the last dictation went, for a moment: none in the given |
| `snapshot` | `.hiring.roles` | `[]` | no folder for hires in the given, so no role to hire as |
| `snapshot` | `.keys[].session.choice` | `object or null` | a menu's options, read from the terminal: none in the given |
| `snapshot` | `.keys[].session.voiceNote` | `object or null` | how the last dictation went, for a moment: none in the given |
| `snapshot` | `.layout.room.dress.rules` | `[]` | no dress rule in the default layout |
| `snapshot` | `.layout.room.floorPlans` | `map` | a floor's plan by its number: keys are data |
| `snapshot` | `.layout.room.pet` | `[]` | no pet in the default layout |
| `snapshot` | `.layout.room.plan.added.tall` | `[]` | nothing added to the default plan |
| `snapshot` | `.layout.room.plan.added.wide` | `[]` | nothing added to the default plan |
| `snapshot` | `.layout.room.plan.hung` | `map` | what's hung where: keys are data |
| `snapshot` | `.layout.room.plan.tall` | `map` | a tall item's place by its id: keys are data |
| `snapshot` | `.layout.room.plan.wide` | `map` | a wide item's place by its id: keys are data |
| `snapshot` | `.layout.room.team` | `map` | the team's looks by folder name: keys are data |
| `snapshot` | `.music` | `object or null` | what's playing: null with music off, as in the given |
| `snapshot` | `.upstairs.desks[].choice` | `object or null` | a menu's options, read from the terminal: none in the given |
| `snapshot` | `.upstairs.desks[].last` | `object or null` | a cloud bot's last report: none in the given |
| `snapshot` | `.upstairs.desks[].subagents` | `[]` | only contract-a, at a key, has a subagent in the given |
| `snapshot` | `.upstairs.desks[].voiceNote` | `object or null` | how the last dictation went, for a moment: none in the given |

## Null allowed

Where a shape allows a null although the Mac's value alone can't say so: every element in the given leaves it
empty, so none beside it says its type. A null the given does fill somewhere is typed by that element instead
(a model at the pinned key, none at the free one: `string` there, `string or null` here), and isn't in this table.
The Go app may answer null here, or the type. The Swift engine's ContractTests.swift checked this table was every
such mark, with why, until M7.

| In | Where | Shape | Why |
|---|---|---|---|
| `GET /setup/gm/team` | `.team[].started` | `number or null` | when its hire was started: null for a session no hire matches, as both in the given |
| `setup-state` | `.dictation.cleanupProblem` | `string or null` | what stops Claude cleaning dictation up: null when nothing does, as in the given |
| `setup-state` | `.dictation.problem` | `string or null` | what stops dictation: null when nothing does, as this Mac's speech recognition says |
| `setup-state` | `.phoneMic.serving` | `bool or null` | whether `tailscale serve` hands the deck on: unknown without Tailscale, as Tailscale isn't asked |
| `setup-state` | `.phoneMic.tailnet` | `string or null` | this Mac's Tailscale name: none, as Tailscale isn't asked |
| `setup-state` | `.summary.fallback` | `string or null` | the provider used when the one picked can't answer: none with summaries off, as in the given |
| `snapshot` | `.floors[].desks[].errorKind` | `string or null` | why its last turn failed: no turn fails in the given |
| `snapshot` | `.floors[].desks[].hiredBy.id` | `string or null` | the desk of who started it: a cloud bot's hirer has none |
| `snapshot` | `.floors[].desks[].maxPressedAt` | `number or null` | when max effort was last asked for from the office: never in the given |
| `snapshot` | `.floors[].desks[].messaged` | `string or null` | the last text sent to it from the office: none in the given |
| `snapshot` | `.floors[].desks[].messagedAt` | `number or null` | when text was last sent to it from the office: never in the given |
| `snapshot` | `.floors[].desks[].model` | `string or null` | upstairs, contract-b's agent hasn't said its model and a cloud bot has none |
| `snapshot` | `.floors[].desks[].turnStartedAt` | `number or null` | upstairs, contract-b was asked nothing (it asks you) and a cloud bot has no turns |
| `snapshot` | `.keys[].session.errorKind` | `string or null` | why its last turn failed: no turn fails in the given |
| `snapshot` | `.keys[].session.maxPressedAt` | `number or null` | when max effort was last asked for from the office: never in the given |
| `snapshot` | `.keys[].session.messaged` | `string or null` | the last text sent to it from the office: none in the given |
| `snapshot` | `.keys[].session.messagedAt` | `number or null` | when text was last sent to it from the office: never in the given |
| `snapshot` | `.keys[].session.subagents[].model` | `string or null` | a subagent's model: the given's SubagentStart says none |
| `snapshot` | `.upstairs.desks[].errorKind` | `string or null` | why its last turn failed: no turn fails in the given |
| `snapshot` | `.upstairs.desks[].hiredBy.id` | `string or null` | the desk of who started it: a cloud bot's hirer has none |
| `snapshot` | `.upstairs.desks[].maxPressedAt` | `number or null` | when max effort was last asked for from the office: never in the given |
| `snapshot` | `.upstairs.desks[].messaged` | `string or null` | the last text sent to it from the office: none in the given |
| `snapshot` | `.upstairs.desks[].messagedAt` | `number or null` | when text was last sent to it from the office: never in the given |
| `snapshot` | `.upstairs.desks[].model` | `string or null` | upstairs, contract-b's agent hasn't said its model and a cloud bot has none |
| `snapshot` | `.upstairs.desks[].turnStartedAt` | `number or null` | upstairs, contract-b was asked nothing (it asks you) and a cloud bot has no turns |

## Known gaps

Where the Go app doesn't answer as the Mac does, today: `knownGaps` in `desktop/contract_test.go`, row for row
(the test checks they're the same). **In** is a route or a shape by its name in `contract.json`; **Where** is
`route` (not served: 404), `callers`, a refusal (its status, kind, or a field of its JSON), a field, the
given's sessions in a list (`ids`), or what no fixture can show, asked of the Go app by the test itself as the
Mac's rule says (`mic: …`, office dictation: `macDictationGaps`); **Go** is what the Go app has instead. **Owner** is who closes it: the M
piece (M2 server and setup, M3 office, snapshot and bots, M4 hiring, guard and GM, M4a briefs, M5 T3, M6 the
personal shell), "via the bridge (M1)" for what only macOS can do, served on a Mac through the bridge, or
`stays: <why>` for a Go-only extra that's meant to stay. Each M piece turns rows into passes, and takes them
out of both tables. Since 1.25.0 (M7) every Mac runs the Go app, so a row left is something a Mac no longer
has until the Go app serves it; the Swift engine's twin is in the history before M7 (release/1.25.0 at 1da4e935). `desktop/contract_test.go` checks each row has one.

| In | Where | Go | Why | Owner |
|---|---|---|---|---|
| `setup-state` | `.hiring.agentsChecked` | extra | whether this computer checks agents' programs (a Mac checks Claude Code's signature, RT126SG2 B2), which the Swift engine never did | stays: Go-only, the page's line under Let the office hire and fire; the Mac runs the Go core |
| `setup-state` | `.hiring.claudeUnsigned` | extra | an unsigned Claude Code refused after a signed one ran, for the owner to confirm (RT126SG2 B2), which the Swift engine never had | stays: Go-only, the page's alone to confirm; the Mac runs the Go core |
| `setup-state` | `.hiring.managedBeta` | extra | the launcher's hire beta (Windows) | stays: Go-only, the launcher's hire beta; the page doesn't read it |
| `setup-state` | `.hiring.managedWindows` | extra | the launcher's hire beta (Windows) | stays: Go-only, the launcher's hire beta; the page doesn't read it |
| `setup-state` | `.mailBeta` | extra | mail to a hire is beta on Windows | stays: Go-only, the page doesn't read it |
| `setup-state` | `.wifiTyping` | extra | Typing over Wi-Fi (SECWIFI), which the Swift engine never had | stays: Go-only, the page's switch under Paired phones; the Mac runs the Go core |
| `setup-state` | `.phones[].beforeSigning` | extra | a phone paired before 1.25, whose key crossed the Wi-Fi in clear (FXSEC B1), which the Swift engine never had | stays: Go-only, the page's Paired phones; the Mac runs the Go core |
| `setup-state` | `.phones[].pairedAgain` | extra | a phone of its name paired since, so setup offers to remove the old entry (FXSEC B1) | stays: Go-only, the page's Paired phones; the Mac runs the Go core |
| `setup-state` | `.summary.noStore` | extra | says there's no secret store for the key, which a Mac always has | stays: Go-only; the page hides the key's setup only when it's true, and a Mac's Keychain (M1) makes it false |
| `setup-state-tool` | `.hiring.agentsChecked` | extra | whether this computer checks agents' programs (a Mac checks Claude Code's signature, RT126SG2 B2), which the Swift engine never did | stays: Go-only, the page's line under Let the office hire and fire; the Mac runs the Go core |
| `setup-state-tool` | `.hiring.claudeUnsigned` | extra | an unsigned Claude Code refused after a signed one ran, for the owner to confirm (RT126SG2 B2), which the Swift engine never had | stays: Go-only, the page's alone to confirm; the Mac runs the Go core |
| `setup-state-tool` | `.hiring.managedBeta` | extra | the launcher's hire beta (Windows) | stays: Go-only, the launcher's hire beta; the page doesn't read it |
| `setup-state-tool` | `.hiring.managedWindows` | extra | the launcher's hire beta (Windows) | stays: Go-only, the launcher's hire beta; the page doesn't read it |
| `setup-state-tool` | `.mailBeta` | extra | mail to a hire is beta on Windows | stays: Go-only, the page doesn't read it |
| `setup-state-tool` | `.pairing.codeExpiresIn` | extra | a tool is told how long the code lasts; the Mac tells only the page | stays: Go-only, a tool's; the page has it in its own state on both apps |
| `setup-state-tool` | `.wifiTyping` | extra | Typing over Wi-Fi (SECWIFI), which the Swift engine never had | stays: Go-only, the page's switch under Paired phones; the Mac runs the Go core |
| `setup-state-tool` | `.phones[].beforeSigning` | extra | a phone paired before 1.25, whose key crossed the Wi-Fi in clear (FXSEC B1), which the Swift engine never had | stays: Go-only, the page's Paired phones; the Mac runs the Go core |
| `setup-state-tool` | `.phones[].pairedAgain` | extra | a phone of its name paired since, so setup offers to remove the old entry (FXSEC B1) | stays: Go-only, the page's Paired phones; the Mac runs the Go core |
| `setup-state-tool` | `.summary.noStore` | extra | says there's no secret store for the key, which a Mac always has | stays: Go-only; the page hides the key's setup only when it's true, and a Mac's Keychain (M1) makes it false |
| `GET /setup/gm/approvals` | `.asks` | extra | the asks a GM, an MCP key, a night job or a hire waits on, for the owner to answer (1.26 FS2), beside the sessions asking | stays: Go-only; a reader that doesn't know the key leaves it alone, and the owner's asks are this app's record (grants.go) |
