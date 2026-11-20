# The front desk API

The routes the GM and your own tools use to talk to the running app. The GM normally doesn't call them: its tools (`smolcorp gm <command> --json`, and the same as MCP tools) do. This page is for a tool of yours, or a port of the app.

The table that decides who may call what is `desktop/routes.go`. Its copy for tests is `Tests/fixtures/frontdesk/routes.json`, and the other files in that folder are real examples of each request and answer. A test (`ManagerDocsTests`) fails if a `/setup/gm/` route in the table isn't mentioned here.

## Who is calling

The app tells callers apart by what they send. Every request is local, and has the header `x-agentdeck-setup: 1`: Smolcorp's setup header, which kept the app's first name.

| Caller | Sends | Notes |
|---|---|---|
| **page** | the setup page's session cookie | Opened from the menu's one-time link. |
| **GM** | `x-smolcorp-gm-token: <token>` | The token lives in the GM's environment only. A hire can't use it. Smolcorp keeps it in the Mac's Keychain (on Linux and Windows, the computer's secret store, else a file in Smolcorp's folder: on Linux only you can read it, and on Windows it is as private as that folder), so a hired GM keeps working after Smolcorp restarts or updates. |
| **tool** | `x-smolcorp-setup-key: <a tool's key>` | For your own scripts and apps. No file holds it: a tool asks Smolcorp for one (`POST /terminal/ask?kind=tool`), and gets it once you click Allow in Smolcorp's own Settings window after checking its code, on a Mac or on Windows (see [Getting a tool's key](#getting-a-tools-key)). A tool can do less than the GM. The `selftest-key` file opens `GET /setup/selftest` alone, sent as `x-smolcorp-selftest-key`. |
| nobody | none of these | Answered `401`. |

A caller who isn't allowed gets `403`. A route not in the table keeps 1.23's rule: the page or a tool. A `/setup/gm/` route not in the table is the GM's or the page's, so a new one starts closed.

While the manager is off (see the [README](README.md)), every `/setup/gm/` route answers `409` ("The manager is off") except the version and the switch.

## Getting a tool's key

For an app of yours (a script, an editor's plugin, a studio app) that reads what Smolcorp knows. It works on a Mac, and on Windows with Smolcorp's own Settings window, which needs the Microsoft Edge WebView2 Runtime. On Linux, on Windows without WebView2, and under `smolcorp serve`, the ask is refused at once with `403` and the reason: there's no window of Smolcorp's own to say yes in.

Up to 1.25, Smolcorp kept a `setup-key` file in its folder for tools to read. Any program could read it too, so 1.26 has none: a copy of the old key opens nothing (`401`). A tool asks for its key instead, and you say yes in Smolcorp's window.

1. **Ask.** From the same computer, to `127.0.0.1` on Smolcorp's port (7710 unless `$SMOLCORP_PORT` says otherwise):
   ```
   POST http://127.0.0.1:7710/terminal/ask?kind=tool&folder=<the folder your app runs in, URL-encoded>
   x-agentdeck-setup: 1
   ```
   The answer is `{"ticket": "…", "code": "4-7-2-9", "expiresIn": 180, "where": "…"}`. Keep the ticket to your app: the key comes back on it, and on nothing else. Show the code to whoever is at the computer, with `where`: "Smolcorp's Settings window (the tray icon, then Open Setup…)" on Windows, "Smolcorp's Settings (menu bar icon, then Settings…)" on a Mac.
2. **The owner checks the code and clicks Allow.** Smolcorp's Settings window shows the ask at the top: "A command in a terminal, or a tool of yours, asks for a key to Setup, until Smolcorp quits", the folder your app said, and the same code. On Windows it also shows which program asked, as Windows itself names it (your app's `.exe`, by its full path, and the programs that started it), since nothing there keeps a hire from asking too. The owner ticks "My terminal shows 4-7-2-9" once the codes match (the window says terminal for an app too), then clicks **Allow**, or **Don't allow**. Only that window answers: not Setup in a browser, not the phone, not another tool.
3. **Wait for the answer.**
   ```
   GET http://127.0.0.1:7710/terminal/ask?ticket=<the ticket>
   ```
   Each call waits up to 20 seconds, then answers `{"state": "waiting"}`, `{"state": "denied"}`, or, once, `{"state": "allowed", "key": "…"}`. Call again while it says `waiting`. `404` means the ask ran out (after 3 minutes) or its answer was already taken: ask again. `429` on the ask means three asks are already waiting.
4. **Send the key.** On every request, with the setup header:
   ```
   x-agentdeck-setup: 1
   x-smolcorp-setup-key: <the key>
   ```
   Keep it in your app's memory only: never in a file, a log or a URL.
5. **How long it lasts.** Until Smolcorp quits (a restart or an update). At most four tools hold a key at once: a fifth Allow stops the oldest. Each run of `smolcorp status` or `smolcorp desks` is an Allow too, and takes one of the four, so four of them in a row stop your app's key. Your app knows its key stopped when it gets `401`: ask again, from step 1.
6. **Taking it back.** Quit Smolcorp and open it again: every tool's key stops working at once, and each tool has to ask again.

A tool's key opens what the **tool** column of the route table allows: Setup's state (`GET /setup/state`), the front desk's reads (`/setup/gm/team`, `events`, `version`…), the repair and the plain look-and-feel switches. It never pairs a phone, hires, fires, answers an ask or opens a page-only route (`403`).

```
ask=$(curl -s -X POST -H 'x-agentdeck-setup: 1' 'http://127.0.0.1:7710/terminal/ask?kind=tool&folder=%2Fwork')
# {"ticket":"…","code":"4-7-2-9","expiresIn":180,"where":"…"}: show the code, then wait for the owner's Allow
curl -s "http://127.0.0.1:7710/terminal/ask?ticket=$(printf '%s' "$ask" | jq -r .ticket)"
# {"state":"allowed","key":"…"}
curl -s -H 'x-agentdeck-setup: 1' -H "x-smolcorp-setup-key: $KEY" http://127.0.0.1:7710/setup/gm/team
```

A General Manager you start yourself gets its token the same way, with `smolcorp gm login` (`kind=gm`), once **Allow a General Manager started by hand** is on in Settings.

## The front desk, `/setup/gm/`

Reads: GM, page or tool. Actions: GM or page. Fixtures are in `Tests/fixtures/frontdesk/`.

| Route | Who | What it does | Example |
|---|---|---|---|
| `GET /setup/gm/team` | GM, page, tool | The team: the registry merged with live sessions, and the contacts. A hire's `nickname` is the registry's, whole (edit it with `POST /setup/gm/registry`); a session nobody hired, or a hire with no nickname, gets the one the office shows for it (set with `POST /setup/gm/nickname` or setup's outfit maker). A hire with no nickname of its own is also found by that name, the office's, in mail, contacts, watches and a desk's preview, once its id, nickname and name have found nobody (in mail, only when no bot has that name); `smolcorp attach` finds it by the office's name for the folder it started in. | `team.json` |
| `GET /setup/gm/events` | GM, page, tool | The event log since a time. Long-polls up to 30 seconds. | `events.json`, `event-line.json`, `hook-events.json` |
| `GET /setup/gm/version` | GM, page, tool | The app's version, `managerEnabled`, and the features it has. Works with the manager off. | `version.json` |
| `GET /setup/gm/outbox` | GM, page, tool | The messages sent and their states. The text of a message goes to the GM and the page, not a tool. | `outbox.json` |
| `GET /setup/gm/watches` | GM, page, tool | What was asked to be watched. | `watches.json` |
| `GET /setup/gm/approvals` | GM, page, tool | What waits for you. Read-only. | none yet |
| `GET /setup/gm/screens` | GM, page, tool | The screens the team can be tiled on. The Mac only: 404 on Windows and Linux. | none yet |
| `POST /setup/gm/hire` | GM, page | Hire an employee: `role` (any role but `gm`: one GM per office, so the GM's token gets a 409: "This office already has a General Manager. The owner can replace it from Setup." when one is working, "Only the owner hires the General Manager, from Setup." when none is; only the page hires or replaces the GM), and optionally `folder`, `account`, `model`, `nickname`, `floor`, `worktree`, `apiKey`, `task`. The hire works in a worktree of its own (on a Mac `~/.smolcorp/worktrees/<repo>-<short id>/<name>`, which the GM reads and never writes; elsewhere `<folder>-hires/<name>`) on branch `hire/<id>/<name>` (in a folder of branches of its own, named by the hire's ID), not in the folder given: the Go core's answer says where, in `worksIn` (and `branch`), and `smolcorp gm hire` prints "Works in <path>". `model` (at most 60 letters, digits and `- _ . : [ ]`, as Setup's hire sheet takes it) and `nickname` (one line of 40 characters at most) work on every system; any other value is a `400`. | `hire.json` |
| `POST /setup/gm/project` | GM, page | Start a new project folder in the GM's projects folder (the folder it was hired into, when that isn't a repository; its sandbox only reads there): `name` (letters, digits, `-`, `_` and `.`, at most 64, no dot first or last, not ending in `-hires`), and `clone`, an `https://` address, if it's a clone. Smolcorp makes the folder itself, never over or through anything already at that name. A clone runs over https alone, with no hooks and no templates, in a folder of Smolcorp's own in its support folder, which no desk can write on a Mac; once it's done, Smolcorp moves it into the projects folder in one step that never replaces anything. A clone that fails leaves nothing in the projects folder (`502`). Answers `{ok, folder, cloned}`, and the event log gets a line, which `smolcorp gm inbox` shows (no page shows it yet). `403` with hiring off, and for a GM held to one project; `409` for a name already there, for one that appeared there during the clone, and for a projects folder on another disk than Smolcorp's own folder, said at once, before git runs (on Windows, once the clone is done). Go only. | none yet |
| `POST /setup/gm/fire` | GM, page | Fire one: `id`, `force`, `removeWorktree`. Answers `204`, and writes the recap; `200` with `{ok, kept, worktree, branch}` when `removeWorktree` was asked and the worktree stayed (it holds changes), saying why. A `live:<session>` id from the team is a session the owner started, not a hire: `404`, saying so and how to stop it. | `fire.json` |
| `POST /setup/gm/brief` | GM, page | Add a new, dated part to the end of a working hire's BRIEF file (`id`, `text`), then mail the hire to read it. Refuses a fired or failed hire, an empty text, or one over the mail size setting. | `brief.json` |
| `POST /setup/gm/tile` | GM, page | Tile the team's windows. The Mac only. | none yet |
| `POST /setup/gm/screen` | GM, page | Choose the screen that holds the team. The Mac only. | none yet |
| `POST /setup/gm/send` | GM, page | Send a message: `to`, `text`, and an `id` of yours (the same `id` twice is delivered once). | `send.json` |
| `GET /setup/gm/registry` | page | Not served yet: the row only keeps a tool's refusal truthful. The GM reads `team` instead. | none |
| `POST /setup/gm/registry` | GM, page | Edit a registry entry's `nickname`, `notes` and `state`; nothing else. | `registry-edit.json`, `registry.json` |
| `POST /setup/gm/nickname` | GM, page | Name whoever works in a `folder` (a session's project or where it is now, or a pinned project): `name`, trimmed, one line of 16 characters at most; empty takes it away. Changes nothing else in the layout. `404` for a folder nobody works in and nothing is pinned to. When a hire working there has a nickname of its own in the registry, only the office's name for its desk changes: the answer adds `note`, which says so and how a hire is renamed: in the registry, with `POST /setup/gm/registry` (a GM held to one project is told to ask the owner). `smolcorp gm nickname` prints it. | none |
| `POST /setup/gm/contacts` | GM, page | Which hires may write to each other: `from`, `to`, `oneWay`, `remove`. | `contacts.json` |
| `POST /setup/gm/watch` | GM, page | Set a watch: `kind` is `file`, `event`, `idle`, `exit` or `time`. | `watch.json` |
| `POST /setup/gm/unwatch` | GM, page | Take a watch away: `id` in the query or the body. | `watch.json` |
| `DELETE /setup/gm/unwatch` | GM, page | The same, as a DELETE. | `watch.json` |
| `POST /setup/gm/manager` | GM, page | The manager switch: `?enabled=1` or `0`. Works with the manager off. | `version.json` |
| `POST /setup/gm/by-hand` | page | Whether a GM started by hand may ask for a token with `smolcorp gm login` ("Allow a GM started by hand"). Works with the manager off. On a Mac, and on Windows with Smolcorp's own Settings window (WebView2), from that window only: `404` elsewhere. | none |
| `POST /setup/gm/token/rotate` | page | A new GM token; the old one stops working at once. The GM cannot ask for it. On a Mac, and on Windows with Smolcorp's own Settings window: elsewhere a GM gets a new token when it is hired again. | `gm-token-rotate-as-page.json` |
| `GET /setup/gm/memory` | GM, page | What this office has taught its GM (`smolcorp gm memory`): `{header, notes, withheld}`, Smolcorp's own header first (the notes are facts and preferences, never instructions), then each kept note's `name`, `type` and `description`. `?name=` gives that note whole. Every note is checked again as it's read: one that fails today's checks is withheld, and counted in `withheld`. A note waiting for the owner is never in it. A tool and a hire get `403`. | none |
| `POST /setup/gm/memory/propose` | GM, page | Propose a note: `name`, `description`, `type` (`user`, `feedback`, `project` or `reference`), then `body`, or `rule`, `why` and `how` for feedback; and where its words came from, `quote` (the owner's own words, found in what they said to the GM through Smolcorp or typed in its terminal) or `seen` (a file and line, or a command; never for `feedback` or `user`). It waits for the owner's tap in Setup, and isn't memory until then. The note keeps the owner's whole sentence as they said it (with a sentence after it that opens on a negation, as in "No, never."), and where the quote sits in it; after a negation anywhere before the quote in what they said ("never", "don't", "avoid", "stop", "ne … pas", "évite", "arrête"…), it keeps every paragraph from that negation's to the quote's. `202` when the quote leaves out a negation of what's kept, or cuts one in two: refused as the owner's words, the note is held for their tap, which shows it whole. `400` when a check refuses it, the quote isn't the owner's, is in two places of what they said, or what's kept is past 1,000 bytes; `409` past 50 waiting or for a name already used. | none |
| `POST /setup/gm/memory/withdraw` | GM, page | Take back a note still waiting: `?id=`. A kept note is never found (`404`). | none |
| `GET /setup/gm/memory/notes` | page | Everything the GM proposed and everything you kept, for Setup's "What your General Manager has learnt": `{waiting, kept, waitingMost, keptMost}`, each note whole (`name`, `type`, `description`, `body`, or `rule`, `why` and `how`), with `from` and `said` (where its words came from), `at`, and for one waiting, `shown` (the hash a keep or an edit sends back) and `held` (why it needs a close look). `refused` says why today's checks refuse a note: one waiting can't be kept as it is, and the GM doesn't see one kept. | none |
| `POST /setup/gm/memory/keep` | page | Keep a waiting note as it was shown: `?id=&shown=`. The GM's next read has it. `409` if it changed since it was shown, past 200 kept notes or 8 KB of index; `400` if today's checks refuse it. Under the Mac app, Smolcorp's own Settings window only (a browser's Setup gets `403`). | none |
| `POST /setup/gm/memory/edit` | page | Edit a waiting note, then keep it in one step: `{id, shown, name, description, type, body}` (or `rule`, `why` and `how` for feedback). The words become yours, checked again as a new note is (a key, a password with its value, an e-mail, a phone number, a card or IBAN number, a home-folder path, markup or hidden characters are refused, `400`), and kept under a new id; the GM's words are gone. An edit that changes nothing is refused (`400`: use keep). `409` as for keep, or for a name another note has. | none |
| `POST /setup/gm/memory/drop` | page | Drop a waiting note: `?id=`. A kept note is never found here (`404`). | none |
| `POST /setup/gm/memory/forget` | page | Forget a kept note: `?id=`. The GM's next read doesn't have it. A waiting note is never found here (`404`). | none |
| `* /setup/gm/memory/*` | page | Any other memory route: no GM route keeps, changes or deletes a note; only you do, in Setup (the five rows above). | none |
| `* /setup/gm/*` | GM, page | Any `/setup/gm/` route not listed above. | none |

No route lets the GM answer a permission prompt, and none will.

A few things worth knowing:

- **Hire** answers `409` when no folder is set up for new hires and none was given, when every desk is taken, or when the hooks aren't connected. A `folder` outside your home folder is refused with `400`.
- **Fire** answers `409` when the worktree has changes nobody committed (`force` overrides this), when the hire is busy (`force` overrides this too), and always when its window shows a question or a menu: nothing is sent, even with `force`. A hire that runs in a window of its own, not in tmux (on a Mac, one made by Smolcorp 1.24 in a Terminal tab), never has its tab closed from here: Smolcorp could only find it by its folder, and the one session in the hire's worktree may be yours, or the GM's. The answer is `409` and nothing changes. Fired again with `force`, it is marked fired and the answer is `200` with a `note` saying its window wasn't closed; its worktree is kept even when `removeWorktree` was asked, with `kept` saying why. Its tab is yours to close, or the office's Fire on its desk closes it, only while its agent is in front there.
- **Contacts** (`/setup/gm/contacts`, GM or page): `{from, to, oneWay?, remove?}` names two hires by id or name. By default they may write to each other; with `oneWay`, only `from` to `to`; with `remove`, the pair is taken away. A hire may always write to the GM and the GM to any hire, so the GM isn't part of a pair. Answers the pairs as they are now.
- **Send** states: `queued`, `delivered`, `left` (given up on after several tries, not sent again) or `failed`. Mail is never typed at a question, a menu, a draft or a busy window, nor into the GM's own window. It reaches a hire through its own tmux session; a hire not in tmux gets it only through Claude Code's inbox socket, which is off unless you set `"mailInboxSocket": true` in `config.json` (Claude Code 2.1.309 or later); otherwise its mail fails and says why. A hire's message to the GM is kept in the outbox and also makes one `mail` event, from that hire, in `GET /setup/gm/events`, so `gm inbox --wait` wakes for it; the same `id` sent again makes no second event.
- **Pinning** a project to a desk is `POST /setup/pin?key=<desk from 0>&folder=<folder>&floor=<floor from 0>` and `POST /setup/unpin?key=&floor=`, both open to the GM and the page, on any floor: `floor` is 0 for the ground floor (1F) up to 5 for 6F, and the ground floor when it's left out, as before. A folder has one desk in the building, so pinning it moves its pin from any other floor. `smolcorp gm pin`, `unpin` and `nickname` (and the MCP tools of the same names) use them, with desks counted from 1 as the office shows and `--floor 2` for 2F (the ground floor unless given; `gm pin` with no `--desk` pins the desk its session sits at now, on whichever floor). Every `smolcorp gm` command counts desks from 1 like that (`gm team`'s FLOOR/DESK column, `gm hire`'s "At desk N", `--desk N`); the JSON the app answers (`desk` in `/setup/gm/team` and the hire answer, `key` in `/setup/pin`) and the `team` MCP tool's JSON stay counted from 0. A nickname is kept under the project's last folder name, so two projects with the same last folder name share it, and it finds the project by any floor's pin too. A General Manager held to one project pins only its own folders, over no other desk's pin, and only on the ground floor or a floor where one of its sessions sits or it already has a pin.
- Use your GM token (`x-smolcorp-gm-token`) for the layout and pin routes, not the setup key.
- **Registry** edits can't change a hire's folder, branch, session or tmux socket.

### An example

```
curl -s -H 'x-agentdeck-setup: 1' -H "x-smolcorp-gm-token: $SMOLCORP_GM_TOKEN" \
  -H 'content-type: application/json' \
  -d '{"to":"Rev","text":"Please look at the login change first.","id":"gm-001"}' \
  http://127.0.0.1:<port>/setup/gm/send
# {"id":"gm-001","state":"delivered","to":"abcdef123456","size":38}
```

The port is the one the app is on; tests never use 7710, 7720 or 7730.

## Other routes the GM can reach

These are 1.23's routes, now closed to the setup-key file because a hire could misuse them. Each is for the GM or the page: `/setup/hire`, `/setup/hire/gm`, `/setup/hiring`, `/setup/roles`, `/setup/hooks`, `/setup/unpair`, `/setup/notify` (it was for the iPhone app, which is gone: 404), `/setup/webhook`, `/setup/webhook/test`, `/setup/bots`, `/setup/bots/override`, `/setup/bots/resolve`, `/setup/maxconfirm`, `/setup/phonemic`, `/setup/dictation`, `/setup/login`, `/setup/layout`, `/setup/profiles`, `/setup/pin`, `/setup/unpin`, `/setup/floors`. On `/setup/hiring` the GM can turn hiring off but not back on, and can't set the folder for hires or confirm an unsigned Claude Code: those are the owner's, in Setup. `POST /setup/hire/gm` starts the GM and is the page's alone in practice: the GM's token gets the same 409 as on `/setup/gm/hire` (one of the two texts above, by whether a GM is working). `GET /setup/claude-accounts` is open to everyone.

The setup page alone may spend or send elsewhere (summaries, the viewer) and handle phone codes and keys (`/setup/pair`, the QR pictures, the viewer's key), and open the folder panel on your screen (`/setup/hiring/choose`: the folder for hires is the owner's, set in Setup, and a GM only chooses where one hire works, with `hire --folder`), and choose the keys that show and hide the office from anywhere (`/setup/officekey`). The routes under `/bots/` are for the page and your own tools.
