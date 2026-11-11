# The route contract: inventory

Every route the Mac app serves (`Sources/AgentDeck/DeckRouter.swift`, `BriefRoutes.swift`; `HTTPServer.swift`
has none of its own), who may call it, what it takes and answers, which fixture covers it, and whether the
Linux/Windows app (`desktop/`) serves it: **served**, **404**, or **differs** (served, with a known gap).

- The fixtures beside this file are written from the Mac's answers by `Tests/AgentDeckTests/ContractTests.swift`
  (run once with `SMOLCORP_WRITE_FIXTURES=1` to write them; a normal run checks the Mac still answers so), and
  asked of the Go app by `desktop/contract_test.go`. Shapes, not values: status, kind of body, and for JSON its
  keys and their types. `contract.json` holds the shapes routes share (the setup state, the snapshot) and what
  each fixture's `given` sets up.
- **Go** is checked by `desktop/contract_test.go` for every route with a fixture here: it must be what the
  known gaps (the last table) make it. For a route without one, it is read from `desktop/server.go`, and
  cross-checked with `docs/GO-PARITY.md`.
- Callers: `page` (the setup page's cookie), `tool` (the setup-key file), `gm` (the GM's token), `hire` (a
  hire's send key), as `frontdesk/routes.json`; `phone` (a paired phone's key, or the office's own window),
  `window` (the office's window on this computer only), `hook` (the hook's key, from this computer), `anyone`.
- No fixture asks a route that writes the home folder, asks the Keychain, opens a window, a picker or a
  terminal, or starts another program: those have their refusals in a fixture where one comes first, or none.


## The office, the phone and the hook

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| GET | `/` | anyone | `?t=` a phone's key | HTML: the pairing page without a key, the deck with one | 200, 403 (not addressed directly) | — | served |
| GET | `/office` | anyone | `?t=` | as `/` | 200, 403 | — | served (Go also swaps a tray token `?o=` for the window key, 303) |
| GET | `/icon.png` | anyone | — | PNG: the logo's 180 px icon | 200 | — | served |
| GET | `/favicon-16.png` | anyone | — | PNG | 200 | — | served |
| GET | `/favicon-32.png` | anyone | — | PNG | 200 | — | served |
| GET | `/office.js` | anyone | — | JS | 200 | — | served |
| GET | `/guide.js` | anyone | — | JS | 200 | — | served |
| GET | `/sounds/*` | anyone | keys/talk .m4a/.webm | audio | 200 | — | served |
| POST | `/pair` | anyone | `{code}` | `{key}` | 200, 403 `{error}` | office.json | served |
| POST | `/hook` | hook | hook event; headers `x-agentdeck-agent`, `-tab`, `-tty`, `-project`, `-pid`, `-parent`, `-at`, `-billing` | empty | 204, 400 (a header twice), 401, 404 (not local, not POST) | office.json, frontdesk/hook-events.json | served |
| GET | `/events` | phone | `?t=` | event stream: `data:` the snapshot at once and at each change, `event: ping` | 200, 401 | state.json | differs |
| POST | `/focus` | phone | `?session=` | empty | 204, 404 | office.json | served |
| POST | `/act` | phone | `?action=` accept, send, reject, esc, up, down, clear, button, effort, scroll, mic, swap, style, music, fire, hire, move, seen, language, cleanup, settings, choose…; `&session=` | empty, or text | 204, 400, 403, 404, 409 | office.json | differs |
| POST | `/summary` | phone | `?session=` | `{summary}` or `{error}` | 200, 400–503 `{error}` | office.json | served |
| GET | `/glossary` | phone | — | `{api, glossary, language}` for the iPhone app's speech | 200 | office.json | 404 |
| GET | `/approvals` | phone | — | `{approvals, at}` | 200, 403 (not Tailscale) | office.json | 404 |
| GET | `/reviews` | phone | — | the NEEDS-REVIEW list | 200, 403 (not Tailscale) | office.json | 404 |
| GET | `/conversation` | phone | `?session=&before=&size=` | a page of the conversation | 200, 400, 403 | office.json | 404 |
| GET | `/session/info` | phone | `?id=` or `?session=` | account, model, time, tokens | 200, 403 | office.json | 404 |
| GET | `/notifications` | phone | — | `{key, registered, finished, waiting, gm}` | 200 | — | 404 |
| POST | `/notifications` | phone | `{token, sandbox, finished, waiting, gm, office, officeID}` | as GET | 200, 403 | — | 404 |
| POST | `/dictate` | phone | `{text, send}` | empty or text | 204, 403 | office.json | 404 |
| POST | `/peek` | phone | `?session=` | the input line | 200, 403 | office.json | 404 |
| POST | `/choose` | phone | `?option=other&choice=`, `{text}` | empty | 204, 400, 403 | office.json | 404 |
| GET | `/choice` | phone | `?session=` | a prompt's command or file | 200, 403 | office.json | 404 |
| GET | `/t3code` | window | `?session=&fresh=1` | `{installed, …, hire}` | 200, 403 (a phone) | office.json | served |
| POST | `/t3code/open` | window | `?session=` | empty or text | 204, 403, 404, 409 | office.json | served |
| POST | `/bots/<id>/event` | page, tool | `{type: report or seen or start or stop, text, count, at}` | empty | 204, 400, 401, 404 | office.json | served |

## Setup: the page, the state and its switches

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| GET | `/setup` | page, tool | — | HTML: setup.html | 200, 404 (not local) | — | served |
| POST | `/setup/session` | page, tool | the menu link's key in `x-smolcorp-setup-key` | empty, `Set-Cookie` | 204, 401 | setup.json | served |
| GET | `/setup/state` | page, tool | — | the setup state (a tool: no code, no webhook secret) | 200, 401 (`X-Smolcorp-App: mac`) | state.json | differs |
| GET | `/setup/preview` | page, tool | — | HTML: deck.html, framed by this deck only | 200 | — | served |
| GET | `/setup/selftest` | page, tool | — | `{checks: […], …}` | 200 | — | served |
| POST | `/setup/fix` | page, tool | `?fix=&auto=1` | the self-test | 200, 400, 403 (a tool: repair only), 409 | setup.json | served |
| POST | `/setup/scroll` | page, tool | `?enabled=` | the setup state | 200 | setup.json | differs |
| POST | `/setup/maxconfirm` | gm, page | `?enabled=` | the setup state | 200 | setup.json | differs |
| POST | `/setup/guide` | page, tool | `?done=` | the setup state | 200 | setup.json | differs |
| POST | `/setup/music` | page, tool | `?enabled=` | the setup state | 200 | — | served |
| POST | `/setup/phonemic` | gm, page | `?enabled=&autosend=` | the setup state | 200 | setup.json | 404 |
| POST | `/setup/dock` | page, tool | `?enabled=` | the setup state | 200 | — | 404 |
| POST | `/setup/ontop` | page, tool | `?enabled=` | the setup state | 200 | — | Windows only (404 elsewhere: no window) |
| POST | `/setup/office` | page, tool | — | empty | 204 | — | served (opens the browser) |
| POST | `/setup/login` | gm, page | `?enabled=` | the setup state | 200 | — | served |
| POST | `/setup/accessibility` | page, tool | — | empty | 204 | — | 404 |
| POST | `/setup/hooks` | gm, page | `?remove=1` to disconnect | the setup state (`notice`) | 200, 409 (test copy), 500 | — | served |
| POST | `/setup/logo` | page, tool | `?logo=smolcorp or keys` | the setup state | 200, 400 | setup.json | differs |
| GET | `/setup/logo/<logo>` | page, tool | — | PNG | 200, 404 | setup.json | served |
| POST | `/setup/theme` | page, tool | colours | empty | 204 | setup.json | served |
| POST | `/setup/layout` | gm, page | a layout | empty | 204, 400 | setup.json | served |
| POST | `/setup/pin` | gm, page | `?key=&folder=` | empty | 204, 400 | setup.json | served |
| POST | `/setup/unpin` | gm, page | `?key=` | empty | 204, 400 | setup.json | served |
| POST | `/setup/floors` | gm, page | `{floors?, rules?}` | the setup state | 200, 400 | setup.json | differs |
| POST | `/setup/profiles` | gm, page | `?action=switch or save or rename or delete or import or check&name=&to=` | the setup state; check: the profile | 200, 400 | setup.json, frontdesk/profiles.json | differs |
| POST | `/setup/webhook` | gm, page | `{enabled, url, secret, outside, events}` | the setup state | 200, 400 | setup.json | differs |
| POST | `/setup/webhook/test` | gm, page | — | the setup state | 200 | — | served |
| POST | `/setup/pair` | page | — | the setup state with the code | 200 | setup.json | differs |
| POST | `/setup/unpair` | gm, page | `?phone=` or `?all=1` | the setup state | 200, 400 | setup.json | differs |
| GET | `/setup/qr.png` | page | `?via=tailnet` | PNG | 200, 404 (no code) | setup.json | served |
| POST | `/setup/summaries` | page | `?enabled=&confirmed=1&provider=` | the setup state | 200, 400 | setup.json | differs |
| POST | `/setup/summary/provider` | page | `?id=&confirmed=1` | the setup state | 200, 400 | setup.json | served |
| POST | `/setup/summary/check` | page, tool | — | the setup state | 200 | — | served |
| POST | `/setup/summary/key` | page | `{key}` or `?remove=1` | the setup state | 200, 400 | setup.json | served |
| POST | `/setup/dictation` | gm, page | `?enabled=&language=&claude=&cleanup=` | the setup state | 200, 400 | setup.json | 404 |
| POST | `/setup/dictation/glossary` | page, tool | `[names]` | the setup state | 200, 400 | setup.json | 404 |
| POST | `/setup/keystrokes` | page | `?enabled=` | the setup state | 200, 400 | setup.json | 404 |
| POST | `/setup/keystrokes/apps` | page | `?add=` or `?remove=` | the setup state | 200, 400, 409 | setup.json | 404 |
| POST | `/setup/terminal-command` | page | `?remove=1` | the setup state | 200, 409 | — | 404 |
| GET | `/setup/claude-accounts` | gm, page, tool | — | `{found, most, profiles}` | 200 | setup.json | 404 |
| POST | `/setup/claude-accounts` | page | `{profiles: [{name, dir}]}` | as GET | 200, 400 | setup.json | 404 |
| GET | `/setup/guard` | gm, page, tool | — | `{extra, groups, limits}` | 200 | setup.json | 404 |
| POST | `/setup/guard` | page | the guard's groups and patterns | as GET | 200, 400, 500 | setup.json | 404 |
| GET | `/setup/t3code` | page | `?fresh=1` | `{installed, …}` | 200 | — | served |
| POST | `/setup/t3code/open` | page | `{id}` | empty or text | 204, 404, 409 | setup.json | served |

## Setup: hiring, roles, briefs and bots

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| POST | `/setup/hiring` | gm, page | `?enabled=&folder=&agent=&terminal=&tmux=` | the setup state | 200, 400 | setup.json | differs |
| POST | `/setup/hiring/choose` | gm, page | `?pick=1&for=floor` | the setup state; pick: `{picked}` | 200 | — | served (the system's picker) |
| GET | `/setup/hire-folder` | page, tool | `?check=` | `{exists, fix, folder, home, problem, suggested}` | 200 | setup.json | served |
| POST | `/setup/hire-folder` | gm, page | `?folder=&create=1` | as GET | 200, 400 | — | served |
| GET | `/setup/hire/folders` | page, tool | — | `{default, recent}` | 200 | setup.json | 404 |
| POST | `/setup/hire` | gm, page | `{role, folder, account, model, floor, worktree, apiKey, task, answers}` | empty or `{ok, desk, session}` | 200, 204, 400, 404, 409 | setup.json, frontdesk/hire-guard.json | 404 |
| POST | `/setup/hire/gm` | gm, page | `?worktree=1&account=` | empty | 204, 409, and the hire's | setup.json | served |
| GET | `/setup/hire/gm/preflight` | page | `?account=` | `{ready, items}` | 200, 400 | — | served |
| POST | `/setup/roles` | gm, page | the roles, whole | the setup state | 200, 400 | — | served |
| GET | `/setup/briefs` | page, tool | — | `{briefs}` | 200 | setup.json | 404 |
| GET | `/setup/briefs/<name>` | page, tool | — | the brief's markdown | 200, 404 | — | 404 |
| POST | `/setup/briefs/preview` | page, tool | a role and folder | the brief as a hire would get it | 200, 400 | — | 404 |
| POST | `/setup/briefs/restore` | page | — | the briefs | 200 | — | 404 |
| GET | `/setup/bots` | page, tool | — | `{bots}` | 200 | setup.json | served |
| POST | `/setup/bots` | gm, page | `{bots: [...]}` | `{bots}` | 200, 400 | setup.json | served |
| POST | `/setup/bots/override` | gm, page | `{id, pinned, floor, hidden, show, autoDetect, botFloor}` | the setup state | 200, 400, 404 | setup.json | differs |
| POST | `/setup/bots/mailbox` | page | `{id, folder}` | `{bots}` | 200, 400 | setup.json | served |
| POST | `/setup/bots/resolve` | gm, page | — | `{bots, filled, unresolved}` | 200 | setup.json | 404 |
| POST | `/setup/notify` | gm, page | `{title, body}` | `{sent}` | 200, 400, 503 | setup.json | 404 |

## Setup: the portable office's viewer

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| GET | `/setup/viewer` | page, tool | — | the viewer's state (the link for the page only) | 200 | — | served |
| POST | `/setup/viewer` | page | `{baseURL, hideNames, stopOnReturn, stopAfterHours}` | the viewer's state | 200, 400 | setup.json | served |
| POST | `/setup/viewer/secret` | page | — | the viewer's state with `secret` | 200, 500 | — | served |
| POST | `/setup/viewer/check` | page, tool | — | the viewer's state | 200 | — | served |
| POST | `/setup/viewer/switch` | page | `?on=1` | the viewer's state | 200 | — | served |
| POST | `/setup/viewer/rotate` | page | — | the viewer's state | 200 | — | served |
| POST | `/setup/viewer/wipe` | page | — | the viewer's state | 200 | — | served |
| GET | `/setup/viewer/qr.png` | page | — | PNG | 200, 404 | — | served |

## Setup: the General Manager (/setup/gm/, 409 while the manager is off but for version and the switch)

| Method | Path | Callers | Request | Answer | Statuses | Fixture | Go |
|---|---|---|---|---|---|---|---|
| GET | `/setup/gm/version` | gm, page, tool | — | `{features, managerEnabled, version}` | 200 | gm.json, frontdesk/version.json | served |
| POST | `/setup/gm/manager` | gm, page | `?enabled=1 or 0` | `{managerEnabled}` | 200, 400 | gm.json | served |
| GET | `/setup/gm/team` | gm, page, tool | — | `{at, contacts, team}` | 200, 409 | gm.json, frontdesk/team.json | served |
| GET | `/setup/gm/events` | gm, page, tool | `?since=&wait=&limit=` | `{events, next, oldest}` | 200, 409 | gm.json, frontdesk/events.json | served |
| GET | `/setup/gm/approvals` | gm, page, tool | — | `{approvals, at}` | 200, 409 | gm.json, frontdesk/approvals.json | served |
| GET | `/setup/gm/outbox` | gm, page, tool | `?limit=` | `{messages}` (a tool: no texts) | 200, 409 | gm.json, frontdesk/outbox.json | served |
| GET | `/setup/gm/watches` | gm, page, tool | — | `{watches}` | 200, 409 | gm.json, frontdesk/watches.json | served |
| POST | `/setup/gm/watch` | gm, page | `{kind, target, pattern, minutes, repeat}` | the watch | 200, 400, 409 | gm.json, frontdesk/watch.json | served |
| POST | `/setup/gm/unwatch` | gm, page | `?id=` or `{id}` | `{id, removed}` | 200, 400, 404, 409 | gm.json | served |
| DELETE | `/setup/gm/unwatch` | gm, page | as POST | as POST | as POST | — | served |
| GET | `/setup/gm/screens` | gm, page, tool | — | `{ask, chosen, screens}` | 200, 409 | gm.json | 404 |
| POST | `/setup/gm/screen` | gm, page | `?id=` | as GET | 200, 400, 409 | gm.json | 404 |
| POST | `/setup/gm/tile` | gm, page | — | the tiling's report | 200, 409 | — | 404 |
| POST | `/setup/gm/hire` | gm, page | as `/setup/hire`, folder optional | `{ok, desk, session}` | 200, 400, 404, 409 | gm.json, frontdesk/hire.json | 404 |
| POST | `/setup/gm/fire` | gm, page | `{id, force}` | the fire's report | 200, 400, 404, 409 | gm.json, frontdesk/fire.json | 404 |
| POST | `/setup/gm/brief` | gm, page | `{id, text, file}` | `{ok, …}` | 200, 400, 404, 409 | gm.json, frontdesk/brief.json | 404 |
| POST | `/setup/gm/send` | gm, page, hire | `{to, text, …}` | the message | 200, 400, 403, 404, 409 | gm.json, frontdesk/send.json | served |
| POST | `/setup/gm/contacts` | gm, page | `{pairs, remove}` | `{contacts}` | 200, 400, 409 | gm.json, frontdesk/contacts.json | served |
| POST | `/setup/gm/registry` | gm, page | `{id, nickname, state, notes}` | the hire | 200, 400, 404, 409 | gm.json, frontdesk/registry-edit.json | served |
| GET | `/setup/gm/registry` | page | — | nothing: listed for the page, not answered | 404 | gm.json | served |
| POST | `/setup/gm/nickname` | gm, page | `{project or folder, nickname}` | `{project, nickname}` | 200, 400, 404, 409 | gm.json | served |
| POST | `/setup/gm/by-hand` | page | `?enabled=1 or 0` | `{gmByHand}` | 200, 400 | gm.json | 404 |
| POST | `/setup/gm/token/rotate` | page | — | `{rotated, message}`, never the token | 200, 500 | gm.json, frontdesk/gm-token-rotate-as-page.json | 404 |
| * | `/setup/gm/*` | gm, page | a route not listed above | — | 403 for a tool, 404 | — | 404 |

## Served by the Go app only

| Method | Path | What | Mac |
|---|---|---|---|
| GET | `/launcher` | a launcher's connection (`smolcorp run`), the hook key: the deck's keys as NDJSON | none: the Mac has no launcher |
| GET | `/launcher/start` | an engine hire's start environment, once | none |
| POST | `/launcher/screen` | a launcher's answer to a look at its window | none |
| POST | `/raise` | a second Smolcorp asking this one to show the office | none |
| GET | `/office?o=` | the tray's one-time token swapped for the window key (303) | the Mac's window has its key |


## Fields of `/setup/state` (shape `setup-state`, the page's)

Two levels down; `layout`, `theme` and the roles are the config's own, compared whole by the shape. Fields only the Go app has are the known gaps' "extra" rows.

| Field | Type | Go |
|---|---|---|
| `.activeProfile` | string | same |
| `.agents` | array of objects | same |
| `.agents[].connected` | bool | same |
| `.agents[].id` | string | same |
| `.agents[].name` | string | same |
| `.checklist` | object | same |
| `.checklist.accessibility` | bool | missing: nothing off the Mac needs Accessibility |
| `.checklist.hooks` | bool | same |
| `.checklist.openAtLogin` | bool | same |
| `.checklist.scroll` | bool | same |
| `.claudeAccounts` | object | missing: Claude accounts are the Mac's: /setup/claude-accounts isn't served (M2) |
| `.claudeAccounts.found` | array of objects | missing: Claude accounts are the Mac's: /setup/claude-accounts isn't served (M2) |
| `.claudeAccounts.most` | number | missing: Claude accounts are the Mac's: /setup/claude-accounts isn't served (M2) |
| `.claudeAccounts.profiles` | array of objects | missing: Claude accounts are the Mac's: /setup/claude-accounts isn't served (M2) |
| `.cloudBots` | object | same |
| `.cloudBots.autoDetect` | bool | missing: only the Mac looks in the Grok Bot app |
| `.cloudBots.botFloor` | number | same |
| `.cloudBots.bots` | array | same |
| `.cloudBots.detects` | bool | same |
| `.cloudBots.hidden` | array | same |
| `.cloudBots.noRoom` | array | same |
| `.dictation` | object | missing: dictation is Apple's speech recognition |
| `.dictation.allowed` | bool | missing: dictation is Apple's speech recognition |
| `.dictation.available` | bool | missing: dictation is Apple's speech recognition |
| `.dictation.claude` | bool | missing: dictation is Apple's speech recognition |
| `.dictation.cleanup` | bool | missing: dictation is Apple's speech recognition |
| `.dictation.cleanupProblem` | null | missing: dictation is Apple's speech recognition |
| `.dictation.enabled` | bool | missing: dictation is Apple's speech recognition |
| `.dictation.glossary` | array of string | missing: dictation is Apple's speech recognition |
| `.dictation.language` | string | missing: dictation is Apple's speech recognition |
| `.dictation.problem` | null | missing: dictation is Apple's speech recognition |
| `.dock` | object | missing: In the Dock is a Mac setting |
| `.dock.enabled` | bool | missing: In the Dock is a Mac setting |
| `.floorRules` | array | same |
| `.floors` | array of objects | same |
| `.floors[].name` | string | same |
| `.floors[].preset` | string | same |
| `.gmHired` | bool | same |
| `.guide` | object | same |
| `.guide.done` | bool | same |
| `.hiring` | object | same |
| `.hiring.agent` | string | same |
| `.hiring.apiKey` | object | missing: a hire on the API-key account is the Mac's (M4) |
| `.hiring.canChoose` | bool | same |
| `.hiring.canHire` | bool | same |
| `.hiring.enabled` | bool | same |
| `.hiring.folder` | string | same |
| `.hiring.hasTmux` | bool | same |
| `.hiring.terminal` | string | same |
| `.hiring.terminals` | array of objects | same |
| `.hiring.tmux` | bool | same |
| `.keystrokes` | object | missing: pressing keys in other apps needs Accessibility: the Mac's |
| `.keystrokes.accessibility` | bool | missing: pressing keys in other apps needs Accessibility: the Mac's |
| `.keystrokes.apps` | array | missing: pressing keys in other apps needs Accessibility: the Mac's |
| `.keystrokes.enabled` | bool | missing: pressing keys in other apps needs Accessibility: the Mac's |
| `.keystrokes.running` | array of objects | missing: pressing keys in other apps needs Accessibility: the Mac's |
| `.layout` | object | same |
| `.logo` | string | same |
| `.manager` | object | missing: no manager object here: the switch is in config.json, gmHired is top level (M2) |
| `.manager.enabled` | bool | missing: no manager object here: the switch is in config.json, gmHired is top level (M2) |
| `.manager.gmByHand` | bool | missing: no manager object here: the switch is in config.json, gmHired is top level (M2) |
| `.manager.gmStartedByHand` | bool | missing: no manager object here: the switch is in config.json, gmHired is top level (M2) |
| `.manager.gmTokenFile` | string | missing: no manager object here: the switch is in config.json, gmHired is top level (M2) |
| `.manager.gmTokenFileShown` | string | missing: no manager object here: the switch is in config.json, gmHired is top level (M2) |
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
| `.phoneMic.serving` | null | missing: the phone mic is the Mac's personal build |
| `.phoneMic.tailnet` | null | missing: the phone mic is the Mac's personal build |
| `.phones` | array of objects | same |
| `.phones[].connected` | bool | same |
| `.phones[].id` | string | same |
| `.phones[].lastSeen` | number | same |
| `.phones[].name` | string | same |
| `.phones[].pairedAt` | number | same |
| `.pins` | array of null | same |
| `.platform` | string | same |
| `.presets` | array of string | same |
| `.profileEdited` | bool | same |
| `.profiles` | array | same |
| `.roles` | object | same |
| `.roles.agents` | array of objects | same |
| `.roles.badges` | array of string | same |
| `.roles.efforts` | array of string | same |
| `.roles.limits` | object | same |
| `.roles.shipped` | array of objects | same |
| `.sessions` | array of objects | same |
| `.sessions[].agent` | string | same |
| `.sessions[].agentName` | string | same |
| `.sessions[].folder` | string | same |
| `.sessions[].id` | string | same |
| `.sessions[].key` | number | same |
| `.sessions[].model` | null | same |
| `.sessions[].name` | string | same |
| `.sessions[].role` | null | same |
| `.sessions[].state` | string | same |
| `.summary` | object | same |
| `.summary.checking` | bool | same |
| `.summary.enabled` | bool | same |
| `.summary.fallback` | null | same |
| `.summary.key` | null | same |
| `.summary.model` | string | same |
| `.summary.provider` | null | same |
| `.summary.providers` | array of objects | same |
| `.summary.saved` | string | same |
| `.summary.savedKey` | bool | same |
| `.terminalCommand` | object | missing: Add smolcorp to Terminal is the Mac's |
| `.terminalCommand.available` | bool | missing: Add smolcorp to Terminal is the Mac's |
| `.terminalCommand.link` | string | missing: Add smolcorp to Terminal is the Mac's |
| `.terminalCommand.onPath` | bool | missing: Add smolcorp to Terminal is the Mac's |
| `.terminalCommand.pathFile` | string | missing: Add smolcorp to Terminal is the Mac's |
| `.terminalCommand.pathLine` | string | missing: Add smolcorp to Terminal is the Mac's |
| `.terminalCommand.state` | string | missing: Add smolcorp to Terminal is the Mac's |
| `.terminalCommand.testCopy` | bool | missing: Add smolcorp to Terminal is the Mac's |
| `.theme` | object | same |
| `.version` | string | same |
| `.webhook` | object | same |
| `.webhook.enabled` | bool | same |
| `.webhook.events` | array of string | same |
| `.webhook.last` | null | same |
| `.webhook.outside` | bool | same |
| `.webhook.secret` | string | same |
| `.webhook.url` | string | same |

## Fields of the office snapshot (shape `snapshot`, `/events`' messages)

Two levels down (a desk's session: three); `layout`, `theme` and the roles are the config's own, compared whole by the shape. Fields only the Go app has are the known gaps' "extra" rows.

| Field | Type | Go |
|---|---|---|
| `.dictation` | object | missing: dictation is Apple's speech recognition |
| `.dictation.cleanup` | bool | missing: dictation is Apple's speech recognition |
| `.dictation.language` | string | missing: dictation is Apple's speech recognition |
| `.dictation.on` | bool | missing: dictation is Apple's speech recognition |
| `.floors` | array of objects | same |
| `.floors[].asking` | number | same |
| `.floors[].desks` | array | same |
| `.floors[].n` | number | same |
| `.floors[].name` | string | same |
| `.floors[].preset` | string | same |
| `.floorsOverflow` | number | same |
| `.hiring` | object | same |
| `.hiring.fire` | bool | same |
| `.hiring.hire` | bool | same |
| `.hiring.roles` | array | same |
| `.keys` | array of objects | same |
| `.keys[].pin` | null | same |
| `.keys[].session` | object | same |
| `.keys[].session.activity` | string | same |
| `.keys[].session.agent` | string | same |
| `.keys[].session.agentName` | string | same |
| `.keys[].session.choice` | null | missing: a menu's options, read from the Mac's terminals (M3) |
| `.keys[].session.compacting` | bool | same |
| `.keys[].session.conversation` | bool | missing: the conversation view: the iPhone app's (personal build) |
| `.keys[].session.dial` | bool | same |
| `.keys[].session.dictates` | bool | missing: dictation is Apple's speech recognition |
| `.keys[].session.effort` | string | same |
| `.keys[].session.errorKind` | null | same |
| `.keys[].session.fire` | bool | missing: what the desk's terminal lets the office do: said once here, as hiring.fire (M3) |
| `.keys[].session.id` | string | same |
| `.keys[].session.keys` | bool | missing: what the desk's terminal lets the office do (M3) |
| `.keys[].session.maxPressedAt` | null | same |
| `.keys[].session.messaged` | null | same |
| `.keys[].session.messagedAt` | null | same |
| `.keys[].session.mic` | bool | same |
| `.keys[].session.model` | null | same |
| `.keys[].session.name` | string | same |
| `.keys[].session.pending` | bool | same |
| `.keys[].session.phoneMic` | bool | missing: the phone mic is the Mac's personal build |
| `.keys[].session.project` | string | same |
| `.keys[].session.role` | null | same |
| `.keys[].session.startedAt` | number | same |
| `.keys[].session.state` | string | same |
| `.keys[].session.subagents` | array | same |
| `.keys[].session.summary` | bool | same |
| `.keys[].session.tool` | string | same |
| `.keys[].session.turnStartedAt` | number | same |
| `.keys[].session.voiceNote` | null | missing: voice notes: the Mac's personal build |
| `.layout` | object | same |
| `.maxConfirm` | bool | same |
| `.music` | null | same |
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

## Known gaps

Where the Go app doesn't answer as the Mac does, today: `knownGaps` in `desktop/contract_test.go`, row for row
(the test checks they're the same). **In** is a route or a shape by its name in `contract.json`; **Where** is
`route` (not served: 404), a refusal, or a field; **Go** is what the Go app has instead. Each M piece turns rows
into passes, and takes them out of both tables.

| In | Where | Go | Why |
|---|---|---|---|
| `setup-state` | `.checklist.accessibility` | missing | nothing off the Mac needs Accessibility |
| `setup-state` | `.claudeAccounts` | missing | Claude accounts are the Mac's: /setup/claude-accounts isn't served (M2) |
| `setup-state` | `.cloudBots.autoDetect` | missing | only the Mac looks in the Grok Bot app |
| `setup-state` | `.dictation` | missing | dictation is Apple's speech recognition |
| `setup-state` | `.dock` | missing | In the Dock is a Mac setting |
| `setup-state` | `.hiring.apiKey` | missing | a hire on the API-key account is the Mac's (M4) |
| `setup-state` | `.hiring.managedBeta` | extra | the launcher's hire beta (Windows) |
| `setup-state` | `.hiring.managedWindows` | extra | the launcher's hire beta (Windows) |
| `setup-state` | `.keystrokes` | missing | pressing keys in other apps needs Accessibility: the Mac's |
| `setup-state` | `.launcher` | extra | the command that starts an agent through the launcher, which the Mac has no use for |
| `setup-state` | `.mailBeta` | extra | mail to a hire is beta on Windows |
| `setup-state` | `.manager` | missing | no manager object here: the switch is in config.json, gmHired is top level (M2) |
| `setup-state` | `.phoneMic` | missing | the phone mic is the Mac's personal build |
| `setup-state` | `.roles.limits.answer` | missing | briefs aren't served here (M4) |
| `setup-state` | `.roles.limits.briefFile` | missing | briefs aren't served here (M4) |
| `setup-state` | `.roles.limits.briefFileName` | missing | briefs aren't served here (M4) |
| `setup-state` | `.roles.shipped[].brief` | missing | briefs aren't served here (M4) |
| `setup-state` | `.summary.noStore` | extra | says there's no secret store for the key, which a Mac always has |
| `setup-state` | `.terminalCommand` | missing | Add smolcorp to Terminal is the Mac's |
| `setup-state-tool` | `.checklist.accessibility` | missing | nothing off the Mac needs Accessibility |
| `setup-state-tool` | `.claudeAccounts` | missing | Claude accounts are the Mac's: /setup/claude-accounts isn't served (M2) |
| `setup-state-tool` | `.cloudBots.autoDetect` | missing | only the Mac looks in the Grok Bot app |
| `setup-state-tool` | `.dictation` | missing | dictation is Apple's speech recognition |
| `setup-state-tool` | `.dock` | missing | In the Dock is a Mac setting |
| `setup-state-tool` | `.hiring.apiKey` | missing | a hire on the API-key account is the Mac's (M4) |
| `setup-state-tool` | `.hiring.managedBeta` | extra | the launcher's hire beta (Windows) |
| `setup-state-tool` | `.hiring.managedWindows` | extra | the launcher's hire beta (Windows) |
| `setup-state-tool` | `.keystrokes` | missing | pressing keys in other apps needs Accessibility: the Mac's |
| `setup-state-tool` | `.launcher` | extra | the command that starts an agent through the launcher, which the Mac has no use for |
| `setup-state-tool` | `.mailBeta` | extra | mail to a hire is beta on Windows |
| `setup-state-tool` | `.manager` | missing | no manager object here: the switch is in config.json, gmHired is top level (M2) |
| `setup-state-tool` | `.pairing.codeExpiresIn` | extra | a tool is told how long the code lasts; the Mac tells only the page |
| `setup-state-tool` | `.phoneMic` | missing | the phone mic is the Mac's personal build |
| `setup-state-tool` | `.roles.limits.answer` | missing | briefs aren't served here (M4) |
| `setup-state-tool` | `.roles.limits.briefFile` | missing | briefs aren't served here (M4) |
| `setup-state-tool` | `.roles.limits.briefFileName` | missing | briefs aren't served here (M4) |
| `setup-state-tool` | `.roles.shipped[].brief` | missing | briefs aren't served here (M4) |
| `setup-state-tool` | `.summary.noStore` | extra | says there's no secret store for the key, which a Mac always has |
| `setup-state-tool` | `.terminalCommand` | missing | Add smolcorp to Terminal is the Mac's |
| `snapshot` | `.dictation` | missing | dictation is Apple's speech recognition |
| `snapshot` | `.keys[].session.choice` | missing | a menu's options, read from the Mac's terminals (M3) |
| `snapshot` | `.keys[].session.conversation` | missing | the conversation view: the iPhone app's (personal build) |
| `snapshot` | `.keys[].session.dictates` | missing | dictation is Apple's speech recognition |
| `snapshot` | `.keys[].session.fire` | missing | what the desk's terminal lets the office do: said once here, as hiring.fire (M3) |
| `snapshot` | `.keys[].session.keys` | missing | what the desk's terminal lets the office do (M3) |
| `snapshot` | `.keys[].session.phoneMic` | missing | the phone mic is the Mac's personal build |
| `snapshot` | `.keys[].session.voiceNote` | missing | voice notes: the Mac's personal build |
| `snapshot` | `.phoneMic` | missing | the phone mic is the Mac's personal build |
| `POST /setup/phonemic` | `route` | 404 | the phone mic is the Mac's personal build |
| `POST /setup/dictation` | `route` | 404 | dictation is Apple's speech recognition |
| `POST /setup/dictation/glossary` | `route` | 404 | dictation is Apple's speech recognition |
| `POST /setup/keystrokes` | `route` | 404 | pressing keys in other apps needs Accessibility: the Mac's |
| `POST /setup/keystrokes/apps` | `route` | 404 | pressing keys in other apps needs Accessibility: the Mac's |
| `GET /setup/guard` | `route` | 404 | the hire guard's settings are the Mac's (M4) |
| `POST /setup/guard` | `route` | 404 | the hire guard's settings are the Mac's (M4) |
| `GET /setup/claude-accounts` | `route` | 404 | Claude accounts are the Mac's (M2) |
| `POST /setup/claude-accounts` | `route` | 404 | Claude accounts are the Mac's (M2) |
| `POST /setup/bots/resolve` | `route` | 404 | only the Mac looks in the Grok Bot app |
| `GET /setup/hire/folders` | `route` | 404 | hiring from the setup page by role is the Mac's (M4) |
| `GET /setup/briefs` | `route` | 404 | briefs aren't served here (M4) |
| `POST /setup/hire` | `route` | 404 | hiring from the setup page by role is the Mac's (M4) |
| `POST /setup/notify` | `route` | 404 | a push to the iPhone app (personal build) |
| `GET /setup/gm/screens` | `route` | 404 | tiling the team is the Mac's (M4) |
| `POST /setup/gm/screen` | `route` | 404 | tiling the team is the Mac's (M4) |
| `POST /setup/gm/hire` | `route` | 404 | the GM's hiring is the Mac's (M4) |
| `POST /setup/gm/fire` | `route` | 404 | the GM's firing is the Mac's (M4) |
| `POST /setup/gm/brief` | `route` | 404 | a later brief is the Mac's (M4) |
| `POST /setup/gm/by-hand` | `route` | 404 | no GM token here (M4) |
| `POST /setup/gm/token/rotate` | `route` | 404 | no GM token here (M4) |
| `POST /act` | `refusal: unknown dictation language` | status 404 | no dictation here: action=language isn't known, so its session is looked for |
| `GET /glossary` | `route` | 404 | the iPhone app's speech names (personal build) |
| `GET /approvals` | `route` | 404 | over Tailscale only (personal build) |
| `GET /reviews` | `route` | 404 | over Tailscale only (personal build) |
| `GET /conversation` | `route` | 404 | the iPhone app's conversation (personal build) |
| `GET /session/info` | `route` | 404 | ⓘ Info (personal build) |
| `POST /dictate` | `route` | 404 | the phone mic (personal build) |
| `POST /peek` | `route` | 404 | the phone mic (personal build) |
| `POST /choose` | `route` | 404 | the iPhone app's typed answers (personal build) |
| `GET /choice` | `route` | 404 | the iPhone app's prompt details (personal build) |
