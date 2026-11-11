# The route contract: inventory

Every route the Mac app serves (`Sources/AgentDeck/DeckRouter.swift`, `BriefRoutes.swift`; `HTTPServer.swift`
has none of its own), who may call it, what it takes and answers, which fixture covers it, and whether the
Linux/Windows app (`desktop/`) serves it: **served**, **404**, or **differs** (served, with a known gap).

- The fixtures beside this file are written from the Mac's answers by `Tests/AgentDeckTests/ContractTests.swift`
  (run once with `SMOLCORP_WRITE_FIXTURES=1` to write them; a normal run checks the Mac still answers so), and
  asked of the Go app by `desktop/contract_test.go`. Shapes, not values: status, kind of body (refusals too),
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
- Callers: `page` (the setup page's cookie), `tool` (the setup-key file), `gm` (the GM's token), `hire` (a
  hire's send key), as `frontdesk/routes.json`; `phone` (a paired phone's key, or the office's own window),
  `window` (the office's window on this computer only), `hook` (the hook's key, from this computer), `anyone`.
  Each route outside setup is also asked by one who isn't its caller ("not one of its callers").


## The office, the phone and the hook

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| GET | `/` | anyone | `?t=` a phone's key | HTML: the pairing page without a key, the deck with one | 200, 403 (not addressed directly) | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/office` | anyone | `?t=` | as `/` | 200, 403 | none: a file of the app's web folder, which the test bundle doesn't have | served (Go also swaps a tray token `?o=` for the window key, 303) |
| GET | `/icon.png` | anyone | — | PNG: the logo's 180 px icon | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/favicon-16.png` | anyone | — | PNG | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/favicon-32.png` | anyone | — | PNG | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/office.js` | anyone | — | JS | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/guide.js` | anyone | — | JS | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/sounds/*` | anyone | keys/talk .m4a/.webm | audio | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| POST | `/pair` | anyone | `{code}` | `{key}` | 200, 403 `{error}` | office.json | served |
| POST | `/hook` | hook | hook event; headers `x-agentdeck-agent`, `-tab`, `-tty`, `-project`, `-pid`, `-parent`, `-at`, `-billing` | empty | 204, 400 (a header twice), 401, 404 (not local, not POST) | office.json, frontdesk/hook-events.json | served |
| GET | `/events` | phone | `?t=` | event stream: `data:` the snapshot at once and at each change, `event: ping` | 200, 401 | state.json | differs |
| POST | `/focus` | phone | `?session=`; a hire in T3 Code's desk opens its own thread in T3 Code (T3D-a) | empty, or text | 204, 404, 409 | office.json | served |
| POST | `/act` | phone | `?action=` accept, send, reject, esc, up, down, clear, button, effort, scroll, mic, swap, style, music, fire, hire, move, seen, language, cleanup, settings, choose, letgo (a hire in T3 Code's desk, T3D-a; any other action there is 409)…; `&session=` | empty, or text | 204, 400, 403, 404, 409 | office.json | differs |
| POST | `/summary` | phone | `?session=` | `{summary}` or `{error}` | 200, 400–503 `{error}` | office.json | served |
| GET | `/glossary` | phone | — | `{api, glossary, language}` for the iPhone app's speech | 200 | office.json | 404 |
| GET | `/approvals` | phone | — | `{approvals, at}` | 200, 403 (not Tailscale) | office.json | 404 |
| GET | `/reviews` | phone | — | the NEEDS-REVIEW list | 200, 403 (not Tailscale) | office.json | 404 |
| GET | `/conversation` | phone | `?session=&before=&size=` | a page of the conversation | 200, 400, 403 | office.json | 404 |
| GET | `/session/info` | phone | `?id=` or `?session=` | account, model, time, tokens | 200, 403 | office.json | 404 |
| GET | `/notifications` | phone | — | `{key, registered, finished, waiting, gm}` | 200 | office.json | 404 |
| POST | `/notifications` | phone | `{token, sandbox, finished, waiting, gm, office, officeID}` | as GET | 200, 403 | office.json | 404 |
| POST | `/dictate` | phone | `{text, send}` | empty or text | 204, 403 | office.json | 404 |
| POST | `/peek` | phone | `?session=` | the input line | 200, 403 | office.json | 404 |
| POST | `/choose` | phone | `?option=other&choice=`, `{text}` | empty | 204, 400, 403 | office.json | 404 |
| GET | `/choice` | phone | `?session=` | a prompt's command or file | 200, 403 | office.json | 404 |
| GET | `/t3code` | window | `?session=&fresh=1` | `{installed, …, hire}`, and `lane` for a hire in T3 Code | 200, 403 (a phone), 404 (T3 Code off) | office.json | served |
| POST | `/t3code/open` | window | `?session=` | empty or text | 204, 403, 404 (T3 Code off, not a hire), 409 | office.json | served |
| GET | `/hire/window` | window | — | `{background}`: whether hires run in the background | 200, 403 (a phone) | office.json | 404 |
| POST | `/hire/window` | window | `?session=` | empty or text: a window attached to the hire's tmux session | 204, 403 (a phone), 404 (not a hire), 409 | office.json | 404 |
| POST | `/bots/<id>/event` | page, tool | `{type: report or seen or start or stop, text, count, at}` | empty | 204, 400, 401, 404 | office.json | served |

## Setup: the page, the state and its switches

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| GET | `/setup` | page, tool | — | HTML: setup.html | 200, 404 (not local) | none: a file of the app's web folder, which the test bundle doesn't have | served |
| POST | `/setup/session` | page, tool | the menu link's key in `x-smolcorp-setup-key` | empty, `Set-Cookie` | 204, 401 | setup.json | served |
| GET | `/setup/state` | page, tool | — | the setup state (a tool: no code, no webhook secret) | 200, 401 (`X-Smolcorp-App: mac`) | state.json | differs |
| GET | `/setup/preview` | page, tool | — | HTML: deck.html, framed by this deck only | 200 | none: a file of the app's web folder, which the test bundle doesn't have | served |
| GET | `/setup/selftest` | page, tool | — | `{checks: […], …}` | 200 | none: runs the self-test, which looks at the hooks, the login item and the agents' own commands | served |
| POST | `/setup/fix` | page, tool | `?fix=&auto=1` | the self-test | 200, 400, 403 (a tool: repair only), 409 | setup.json | served |
| POST | `/setup/scroll` | page, tool | `?enabled=` | the setup state | 200 | setup.json | differs |
| POST | `/setup/maxconfirm` | gm, page | `?enabled=` | the setup state | 200 | setup.json | differs |
| POST | `/setup/guide` | page, tool | `?done=` | the setup state | 200 | setup.json | differs |
| POST | `/setup/music` | page, tool | `?enabled=` | the setup state | 200 | none: asks the music app what's playing | served |
| POST | `/setup/phonemic` | gm, page | `?enabled=&autosend=` | the setup state | 200 | setup.json | 404 |
| POST | `/setup/dock` | page, tool | `?enabled=` | the setup state | 200 | none: shows or hides the app in the Dock | served (under the Mac app, through the bridge; 404 on Linux and Windows) |
| POST | `/setup/ontop` | page, tool | `?enabled=` | the setup state | 200 | none: puts the office's window on top, on Windows only | Windows only (404 elsewhere: no window) |
| POST | `/setup/office` | page, tool | — | empty | 204 | none: opens the office's window, or the browser | served (opens the browser) |
| POST | `/setup/login` | gm, page | `?enabled=` | the setup state | 200 | none: writes the login item in your home folder | served |
| POST | `/setup/accessibility` | page, tool | — | empty | 204 | none: asks macOS for Accessibility, a system prompt | served (under the Mac app, through the bridge; 404 on Linux and Windows) |
| POST | `/setup/hooks` | gm, page | `?remove=1` to disconnect | the setup state (`notice`) | 200, 409 (test copy), 500 | none: writes the agents' hooks in your home folder | served |
| POST | `/setup/logo` | page, tool | `?logo=smolcorp or keys` | the setup state | 200, 400 | setup.json | differs |
| GET | `/setup/logo/<logo>` | page, tool | — | PNG | 200, 404 | setup.json | served |
| POST | `/setup/theme` | page, tool | colours | empty | 204 | setup.json | served |
| POST | `/setup/layout` | gm, page | a layout | empty | 204, 400 | setup.json | served |
| POST | `/setup/pin` | gm, page | `?key=&folder=` | empty | 204, 400 | setup.json | served |
| POST | `/setup/unpin` | gm, page | `?key=` | empty | 204, 400 | setup.json | served |
| POST | `/setup/floors` | gm, page | `{floors?, rules?}` | the setup state | 200, 400 | setup.json | differs |
| POST | `/setup/profiles` | gm, page | `?action=switch or save or rename or delete or import or check&name=&to=` | the setup state; check: the profile | 200, 400 | setup.json, frontdesk/profiles.json | differs |
| POST | `/setup/webhook` | gm, page | `{enabled, url, secret, outside, events}` | the setup state | 200, 400 | setup.json | differs |
| POST | `/setup/webhook/test` | gm, page | — | the setup state | 200 | none: sends a webhook to its address | served |
| POST | `/setup/pair` | page | — | the setup state with the code | 200 | setup.json | differs |
| POST | `/setup/unpair` | gm, page | `?phone=` or `?all=1` | the setup state | 200, 400 | setup.json | differs |
| GET | `/setup/qr.png` | page | `?via=tailnet` | PNG | 200, 404 (no code) | setup.json | served |
| POST | `/setup/summaries` | page | `?enabled=&confirmed=1&provider=` | the setup state | 200, 400 | setup.json | differs |
| POST | `/setup/summary/provider` | page | `?id=&confirmed=1` | the setup state | 200, 400 | setup.json | served |
| POST | `/setup/summary/check` | page, tool | — | the setup state | 200 | none: runs each coding agent's own command | served |
| POST | `/setup/summary/key` | page | `{key}` or `?remove=1` | the setup state | 200, 400 | setup.json | served |
| POST | `/setup/dictation` | gm, page | `?enabled=&language=&claude=&cleanup=` | the setup state | 200, 400 | setup.json | served |
| POST | `/setup/dictation/glossary` | page, tool | `[names]` | the setup state | 200, 400 | setup.json | served |
| POST | `/setup/keystrokes` | page | `?enabled=` | the setup state | 200, 400 | setup.json | served |
| POST | `/setup/keystrokes/apps` | page | `?add=` or `?remove=` | the setup state | 200, 400, 409 | setup.json | served |
| POST | `/setup/terminal-command` | page | `?remove=1` | the setup state | 200, 409 | none: writes the smolcorp link and your shell's PATH line | served (under the Mac app, through the bridge; 404 on Linux and Windows) |
| GET | `/setup/claude-accounts` | gm, page, tool | — | `{found, most, profiles}` | 200 | setup.json | served |
| POST | `/setup/claude-accounts` | page | `{profiles: [{name, dir}]}` | as GET | 200, 400 | setup.json | served |
| GET | `/setup/guard` | gm, page, tool | — | `{extra, groups, limits}` | 200 | setup.json | served |
| POST | `/setup/guard` | page | the guard's groups and patterns | as GET | 200, 400, 500 | setup.json | served |
| POST | `/setup/choices` | page | `?t3=&background=&asked=1`, each 0 or 1 | the setup state, and `windowsSaid` when it switched hires to the background | 200, 400 (background without tmux) | setup.json | differs |
| GET | `/setup/t3code` | page | `?fresh=1` | `{installed, …, lane}` | 200, 404 (T3 Code off) | setup.json (T3 Code off; with it on, it looks for the T3 Code app and runs its command: not asked) | served |
| POST | `/setup/t3code/open` | page | `{id}` | empty or text | 204, 404 (T3 Code off, no such hire), 409 | setup.json | served |
| POST | `/setup/t3code/letgo` | page | `{id}` | `{ok, recap, note, worktree, branch}` | 200, 404, 409 | setup.json | served |

## Setup: hiring, roles, briefs and bots

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| POST | `/setup/hiring` | gm, page | `?enabled=&folder=&agent=&terminal=&tmux=` | the setup state | 200, 400 | setup.json | differs |
| POST | `/setup/hiring/choose` | gm, page | `?pick=1&for=floor` | the setup state; pick: `{picked}` | 200 | none: opens the system's folder picker | served (the system's picker) |
| GET | `/setup/hire-folder` | page, tool | `?check=` | `{exists, fix, folder, home, problem, suggested}` | 200 | setup.json | served |
| POST | `/setup/hire-folder` | gm, page | `?folder=&create=1` | as GET | 200, 400 | setup.json | served |
| GET | `/setup/hire/folders` | page, tool | — | `{default, recent}` | 200 | setup.json | served |
| POST | `/setup/hire` | gm, page | `{role, folder, account, model, floor, worktree, apiKey, task, answers, engine}` (`engine: "t3"`: into T3 Code, experimental (Go: `Deck.hireIntoT3`)) | empty or `{ok, desk, session}` | 200, 204, 400, 404, 409 | setup.json, frontdesk/hire-guard.json | served |
| POST | `/setup/hire/gm` | gm, page | `?worktree=1&account=` | empty | 204, 409, and the hire's | setup.json | served |
| POST | `/setup/hire/window` | page | `{id}` | empty or text: a window attached to the hire's own tmux session | 204, 409 | setup.json | served |
| GET | `/setup/hire/gm/preflight` | page | `?account=` | `{ready, items}` | 200, 400 | setup.json | differs |
| POST | `/setup/roles` | gm, page | the roles, whole | the setup state | 200, 400 | setup.json | differs |
| GET | `/setup/briefs` | page, tool | — | `{briefs}` | 200 | setup.json | served |
| GET | `/setup/briefs/<name>` | page, tool | — | the brief's markdown | 200, 404 | none: reads one of the app's briefs, which the test bundle doesn't have (404 there) | 404, owner: M4a: briefs |
| POST | `/setup/briefs/preview` | page, tool | a role and folder | the brief as a hire would get it | 200, 400 | setup.json | served |
| POST | `/setup/briefs/restore` | page | — | the briefs | 200 | none: puts back the app's own briefs, which the test bundle doesn't have | served |
| GET | `/setup/bots` | page, tool | — | `{bots}` | 200 | setup.json | served |
| POST | `/setup/bots` | gm, page | `{bots: [...]}` | `{bots}` | 200, 400 | setup.json | served |
| POST | `/setup/bots/override` | gm, page | `{id, pinned, floor, hidden, show, autoDetect, botFloor}` | the setup state | 200, 400, 404 | setup.json | differs |
| POST | `/setup/bots/mailbox` | page | `{id, folder}` | `{bots}` | 200, 400 | setup.json | served |
| POST | `/setup/bots/resolve` | gm, page | — | `{bots, filled, unresolved}` | 200 | setup.json | served |
| POST | `/setup/notify` | gm, page | `{title, body}` | `{sent}` | 200, 400, 503 | setup.json | 404 |

## Setup: the portable office's viewer

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| GET | `/setup/viewer` | page, tool | — | the viewer's state (the link for the page only) | 200 | none: asks the Keychain for the viewer's link | served |
| POST | `/setup/viewer` | page | `{baseURL, hideNames, stopOnReturn, stopAfterHours}` | the viewer's state | 200, 400 | setup.json | served |
| POST | `/setup/viewer/secret` | page | — | the viewer's state with `secret` | 200, 500 | none: makes a secret and keeps it in the Keychain | served |
| POST | `/setup/viewer/check` | page, tool | — | the viewer's state | 200 | none: asks the viewer's server | served |
| POST | `/setup/viewer/switch` | page | `?on=1` | the viewer's state | 200 | none: starts or stops sending the office to the viewer's server | served |
| POST | `/setup/viewer/rotate` | page | — | the viewer's state | 200 | none: makes a new link with the viewer's server | served |
| POST | `/setup/viewer/wipe` | page | — | the viewer's state | 200 | none: wipes the office from the viewer's server | served |
| GET | `/setup/viewer/qr.png` | page | — | PNG | 200, 404 | none: draws the viewer's link, from the Keychain | served |

## Setup: the General Manager (/setup/gm/, 409 while the manager is off but for version and the switch)

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| GET | `/setup/gm/version` | gm, page, tool | — | `{features, managerEnabled, version}` | 200 | gm.json, frontdesk/version.json | served |
| POST | `/setup/gm/manager` | gm, page | `?enabled=1 or 0` | `{managerEnabled}` | 200, 400 | gm.json | served |
| GET | `/setup/gm/team` | gm, page, tool | — | `{at, contacts, team}` (a hire in T3 Code's row: `engine`, `t3Thread`, and `t3` for the page) | 200, 409 | gm.json, frontdesk/team.json | served |
| GET | `/setup/gm/events` | gm, page, tool | `?since=&wait=&limit=` | `{events, next, oldest}` | 200, 409 | gm.json, frontdesk/events.json | served |
| GET | `/setup/gm/approvals` | gm, page, tool | — | `{approvals, at}` | 200, 409 | gm.json, frontdesk/approvals.json | served |
| GET | `/setup/gm/outbox` | gm, page, tool | `?limit=` | `{messages}` (a tool: no texts) | 200, 409 | gm.json, frontdesk/outbox.json | served |
| GET | `/setup/gm/watches` | gm, page, tool | — | `{watches}` | 200, 409 | gm.json, frontdesk/watches.json | served |
| POST | `/setup/gm/watch` | gm, page | `{kind, target, pattern, minutes, repeat}` | the watch | 200, 400, 409 | gm.json, frontdesk/watch.json | served |
| POST | `/setup/gm/unwatch` | gm, page | `?id=` or `{id}` | `{id, removed}` | 200, 400, 404, 409 | gm.json | served |
| DELETE | `/setup/gm/unwatch` | gm, page | as POST | as POST | as POST | gm.json | served |
| GET | `/setup/gm/screens` | gm, page, tool | — | `{ask, chosen, screens}` | 200, 409 | gm.json | 404 |
| POST | `/setup/gm/screen` | gm, page | `?id=` | as GET | 200, 400, 409 | gm.json | 404 |
| POST | `/setup/gm/tile` | gm, page | — | the tiling's report | 200, 409 | none: tiles the team's windows on a screen, which moves them; with the manager off, both apps answer 409 first | 404, owner: M4 via the bridge (M1) |
| POST | `/setup/gm/hire` | gm, page | as `/setup/hire`, folder optional | `{ok, desk, session}` | 200, 400, 403, 404, 409 | gm.json, frontdesk/hire.json | served |
| POST | `/setup/gm/fire` | gm, page | `{id, force, removeWorktree}` | empty, or `{ok, kept, worktree, branch}` when removal was asked and the worktree stayed | 200, 204, 400, 403, 404, 409 | gm.json, frontdesk/fire.json | served |
| POST | `/setup/gm/brief` | gm, page | `{id, text, file}` | `{ok, …}` | 200, 400, 404, 409 | gm.json, frontdesk/brief.json | served |
| POST | `/setup/gm/send` | gm, page, hire | `{to, text, …}` | the message | 200, 400, 403, 404, 409 | gm.json, frontdesk/send.json | served |
| POST | `/setup/gm/contacts` | gm, page | `{pairs, remove}` | `{contacts}` | 200, 400, 409 | gm.json, frontdesk/contacts.json | served |
| POST | `/setup/gm/registry` | gm, page | `{id, nickname, state, notes}` | the hire | 200, 400, 404, 409 | gm.json, frontdesk/registry-edit.json | served |
| GET | `/setup/gm/registry` | page | — | nothing: listed for the page, not answered | 404 | gm.json | served |
| POST | `/setup/gm/nickname` | gm, page | `{project or folder, nickname}` | `{project, nickname}` | 200, 400, 404, 409 | gm.json | served |
| POST | `/setup/gm/by-hand` | page | `?enabled=1 or 0` | `{gmByHand}` | 200, 400 | gm.json | 404 |
| POST | `/setup/gm/token/rotate` | page | — | `{rotated, message}`, never the token | 200, 500 | gm.json, frontdesk/gm-token-rotate-as-page.json | 404 |
| * | `/setup/gm/*` | gm, page | a route not listed above | — | 403 for a tool, 404 | gm.json | served |

## Served by the Go app only

| Method | Path | What | Mac |
|---|---|---|---|
| GET | `/launcher` | a launcher's connection (`smolcorp run`), the hook key: the deck's keys as NDJSON | none: the Mac has no launcher |
| GET | `/launcher/start` | an engine hire's start environment, once | none |
| POST | `/launcher/screen` | a launcher's answer to a look at its window | none |
| POST | `/raise` | a second Smolcorp asking this one to show the office | none |
| GET | `/office?o=` | the tray's one-time token swapped for the window key (303) | the Mac's window has its key |


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
| `.hiring.apiKey` | object | missing: a hire on the API-key account is the Mac's (M4) |
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
| `.phoneMic` | object | missing: the phone mic is the Mac's personal build |
| `.phoneMic.autoSend` | bool | missing: the phone mic is the Mac's personal build |
| `.phoneMic.enabled` | bool | missing: the phone mic is the Mac's personal build |
| `.phoneMic.serving` | string or null | missing: the phone mic is the Mac's personal build |
| `.phoneMic.tailnet` | string or null | missing: the phone mic is the Mac's personal build |
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
| `.keys[].session.conversation` | bool | missing: the conversation view: the iPhone app's (personal build) |
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
| `.keys[].session.phoneMic` | bool | missing: the phone mic is the Mac's personal build |
| `.keys[].session.project` | string | same |
| `.keys[].session.role` | object or null | same |
| `.keys[].session.startedAt` | number | same |
| `.keys[].session.state` | one of 'idle', 'thinking', 'asking', 'unread', 'error' | same |
| `.keys[].session.subagents` | array or [] | same |
| `.keys[].session.summary` | bool | same |
| `.keys[].session.t3` | object | same |
| `.keys[].session.tool` | string or null | same |
| `.keys[].session.turnStartedAt` | number or null | same |
| `.keys[].session.voiceNote` | object or null | missing: voice notes: the Mac's personal build |
| `.layout` | object | same |
| `.maxConfirm` | bool | same |
| `.music` | object or null | same |
| `.now` | number | same |
| `.overflow` | number | same |
| `.phoneMic` | object | missing: the phone mic is the Mac's personal build |
| `.phoneMic.autoSend` | bool | missing: the phone mic is the Mac's personal build |
| `.phoneMic.on` | bool | missing: the phone mic is the Mac's personal build |
| `.scroll` | bool | same |
| `.theme` | object | same |
| `.upstairs` | object | same |
| `.upstairs.desks` | array | same |
| `.upstairs.overflow` | number | same |

## Not compared

Where a shape doesn't compare all the way down: a list this computer fills (its agents, terminals, accounts,
screens), a map whose keys are data, something the given can't set up (`object or null`), or a list empty in
every given (`[]`: the Go app's must be empty too). ContractTests.swift checks this table is every such spot,
with its why. **In** is a shape's name or a route; a setup-state row is the tool's too. The nulls allowed are
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
| `snapshot` | `.floors[].desks[].voiceNote` | `object or null` | how the last dictation went, for a moment: none in the given (personal build) |
| `snapshot` | `.hiring.roles` | `[]` | no folder for hires in the given, so no role to hire as |
| `snapshot` | `.keys[].session.choice` | `object or null` | a menu's options, read from the terminal: none in the given |
| `snapshot` | `.keys[].session.voiceNote` | `object or null` | how the last dictation went, for a moment: none in the given (personal build) |
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
| `snapshot` | `.upstairs.desks[].voiceNote` | `object or null` | how the last dictation went, for a moment: none in the given (personal build) |

## Null allowed

Where a shape allows a null although the Mac's value alone can't say so: every element in the given leaves it
empty, so none beside it says its type. A null the given does fill somewhere is typed by that element instead
(a model at the pinned key, none at the free one: `string` there, `string or null` here), and isn't in this table.
The Go app may answer null here, or the type. ContractTests.swift checks this table is every such mark, with why.

| In | Where | Shape | Why |
|---|---|---|---|
| `GET /setup/gm/team` | `.team[].started` | `number or null` | when its hire was started: null for a session no hire matches, as both in the given |
| `setup-state` | `.dictation.cleanupProblem` | `string or null` | what stops Claude cleaning dictation up: null when nothing does, as in the given |
| `setup-state` | `.dictation.problem` | `string or null` | what stops dictation: null when nothing does, as this Mac's speech recognition says |
| `setup-state` | `.phoneMic.serving` | `string or null` | the HTTPS address the phone mic is served on: none without Tailscale (personal build) |
| `setup-state` | `.phoneMic.tailnet` | `string or null` | this Mac's Tailscale name: none, as Tailscale isn't asked (personal build) |
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
out of both tables; M7 deletes no Swift twin with a row left. `desktop/contract_test.go` checks each row has one.

| In | Where | Go | Why | Owner |
|---|---|---|---|---|
| `setup-state` | `.hiring.apiKey` | missing | a hire on the API-key account is the Mac's (M4) | M4 |
| `setup-state` | `.hiring.managedBeta` | extra | the launcher's hire beta (Windows) | stays: Go-only, the launcher's hire beta; the page doesn't read it |
| `setup-state` | `.hiring.managedWindows` | extra | the launcher's hire beta (Windows) | stays: Go-only, the launcher's hire beta; the page doesn't read it |
| `setup-state` | `.launcher` | extra | the command that starts an agent through the launcher, which the Mac has no use for | M2: the page shows the launcher's step when it's sent, so a Mac sends it only once it has the launcher |
| `setup-state` | `.mailBeta` | extra | mail to a hire is beta on Windows | stays: Go-only, the page doesn't read it |
| `setup-state` | `.phoneMic` | missing | the phone mic is the Mac's personal build | M6: personal shell |
| `setup-state` | `.roles.limits.answer` | missing | briefs aren't served here (M4) | M4a: briefs |
| `setup-state` | `.roles.limits.briefFile` | missing | briefs aren't served here (M4) | M4a: briefs |
| `setup-state` | `.roles.limits.briefFileName` | missing | briefs aren't served here (M4) | M4a: briefs |
| `setup-state` | `.summary.noStore` | extra | says there's no secret store for the key, which a Mac always has | stays: Go-only; the page hides the key's setup only when it's true, and a Mac's Keychain (M1) makes it false |
| `setup-state-tool` | `.hiring.apiKey` | missing | a hire on the API-key account is the Mac's (M4) | M4 |
| `setup-state-tool` | `.hiring.managedBeta` | extra | the launcher's hire beta (Windows) | stays: Go-only, the launcher's hire beta; the page doesn't read it |
| `setup-state-tool` | `.hiring.managedWindows` | extra | the launcher's hire beta (Windows) | stays: Go-only, the launcher's hire beta; the page doesn't read it |
| `setup-state-tool` | `.launcher` | extra | the command that starts an agent through the launcher, which the Mac has no use for | M2: the page shows the launcher's step when it's sent, so a Mac sends it only once it has the launcher |
| `setup-state-tool` | `.mailBeta` | extra | mail to a hire is beta on Windows | stays: Go-only, the page doesn't read it |
| `setup-state-tool` | `.pairing.codeExpiresIn` | extra | a tool is told how long the code lasts; the Mac tells only the page | stays: Go-only, a tool's; the page has it in its own state on both apps |
| `setup-state-tool` | `.phoneMic` | missing | the phone mic is the Mac's personal build | M6: personal shell |
| `setup-state-tool` | `.roles.limits.answer` | missing | briefs aren't served here (M4) | M4a: briefs |
| `setup-state-tool` | `.roles.limits.briefFile` | missing | briefs aren't served here (M4) | M4a: briefs |
| `setup-state-tool` | `.roles.limits.briefFileName` | missing | briefs aren't served here (M4) | M4a: briefs |
| `setup-state-tool` | `.summary.noStore` | extra | says there's no secret store for the key, which a Mac always has | stays: Go-only; the page hides the key's setup only when it's true, and a Mac's Keychain (M1) makes it false |
| `snapshot` | `.floors[].desks[].conversation` | missing | the conversation view: the iPhone app's (personal build) | M6: personal shell |
| `snapshot` | `.floors[].desks[].phoneMic` | missing | the phone mic is the Mac's personal build | M6: personal shell |
| `snapshot` | `.floors[].desks[].voiceNote` | missing | voice notes: the Mac's personal build | M6: personal shell |
| `snapshot` | `.keys[].session.conversation` | missing | the conversation view: the iPhone app's (personal build) | M6: personal shell |
| `snapshot` | `.keys[].session.phoneMic` | missing | the phone mic is the Mac's personal build | M6: personal shell |
| `snapshot` | `.keys[].session.voiceNote` | missing | voice notes: the Mac's personal build | M6: personal shell |
| `snapshot` | `.phoneMic` | missing | the phone mic is the Mac's personal build | M6: personal shell |
| `snapshot` | `.upstairs.desks[].conversation` | missing | the conversation view: the iPhone app's (personal build) | M6: personal shell |
| `snapshot` | `.upstairs.desks[].phoneMic` | missing | the phone mic is the Mac's personal build | M6: personal shell |
| `snapshot` | `.upstairs.desks[].voiceNote` | missing | voice notes: the Mac's personal build | M6: personal shell |
| `POST /setup/phonemic` | `route` | 404 | the phone mic is the Mac's personal build | M6: personal shell |
| `GET /setup/hire/gm/preflight` | `refusal: no such Claude account` | status 200 | the preflight takes no account yet: Go has the Claude accounts since M2, but the GM's tools are checked for any account | M4 |
| `POST /setup/notify` | `route` | 404 | a push to the iPhone app (personal build) | M6: personal shell |
| `GET /setup/gm/screens` | `route` | 404 | tiling the team is the Mac's (M4) | M4 via the bridge (M1) |
| `POST /setup/gm/screen` | `route` | 404 | tiling the team is the Mac's (M4) | M4 via the bridge (M1) |
| `POST /setup/gm/by-hand` | `route` | 404 | no GM token here (M4) | M4 |
| `POST /setup/gm/token/rotate` | `route` | 404 | no GM token here (M4) | M4 |
| `POST /act` | `mic: dictation on, under the Mac app` | not dictated | the office's mic doesn't dictate on the Go engine: nothing asks the Mac app's speech recognition to listen (action=mic, language, cleanup), so a desk's dictates and the snapshot's dictation.on stay false | M3 via the bridge (M1) |
| `GET /glossary` | `route` | 404 | the iPhone app's speech names (personal build) | M6: personal shell |
| `GET /approvals` | `route` | 404 | over Tailscale only (personal build) | M6: personal shell |
| `GET /reviews` | `route` | 404 | over Tailscale only (personal build) | M6: personal shell |
| `GET /conversation` | `route` | 404 | the iPhone app's conversation (personal build) | M6: personal shell |
| `GET /session/info` | `route` | 404 | ⓘ Info (personal build) | M6: personal shell |
| `GET /notifications` | `route` | 404 | the iPhone app's push notifications (personal build) | M6: personal shell |
| `POST /notifications` | `route` | 404 | the iPhone app's push notifications (personal build) | M6: personal shell |
| `POST /dictate` | `route` | 404 | the phone mic (personal build) | M6: personal shell |
| `POST /peek` | `route` | 404 | the phone mic (personal build) | M6: personal shell |
| `POST /choose` | `route` | 404 | the iPhone app's typed answers (personal build) | M6: personal shell |
| `GET /choice` | `route` | 404 | the iPhone app's prompt details (personal build) | M6: personal shell |
| `GET /hire/window` | `route` | 404 | the office's Open a window on a hire's desk, a terminal attached to its tmux session: Go serves only Setup's /setup/hire/window | M4b: Linux/Windows by openHireViewer, as Setup's /setup/hire/window; the Mac via the bridge (M1) |
| `POST /hire/window` | `route` | 404 | the office's Open a window on a hire's desk, a terminal attached to its tmux session: Go serves only Setup's /setup/hire/window | M4b: Linux/Windows by openHireViewer, as Setup's /setup/hire/window; the Mac via the bridge (M1) |
