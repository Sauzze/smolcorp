# The front desk API

The routes the GM and your own tools use to talk to the running app. The GM normally doesn't call them: its tools (`smolcorp gm <command> --json`, and the same as MCP tools) do. This page is for a tool of yours, or a port of the app.

The table that decides who may call what is `desktop/routes.go`. Its copy for tests is `Tests/fixtures/frontdesk/routes.json`, and the other files in that folder are real examples of each request and answer. A test (`ManagerDocsTests`) fails if a `/setup/gm/` route in the table isn't mentioned here.

## Who is calling

The app tells callers apart by what they send. Every request is local, and has the header `x-agentdeck-setup: 1`.

| Caller | Sends | Notes |
|---|---|---|
| **page** | the setup page's session cookie | Opened from the menu's one-time link. |
| **GM** | `x-smolcorp-gm-token: <token>` | The token lives in the GM's environment only. A hire can't use it. Smolcorp keeps it in the Mac's Keychain (on Linux and Windows, the computer's secret store, else a file in Smolcorp's folder: on Linux only you can read it, and on Windows it is as private as that folder), so a hired GM keeps working after Smolcorp restarts or updates. |
| **tool** | `x-smolcorp-setup-key: <a tool's key>` | For your own scripts. No file holds it: a script asks Smolcorp for one (`POST /terminal/ask?kind=tool`), and gets it once you click Allow in Smolcorp's window after checking its code (see the setup docs). A tool can do less than the GM. The `selftest-key` file opens `GET /setup/selftest` alone, sent as `x-smolcorp-selftest-key`. |
| nobody | none of these | Answered `401`. |

A caller who isn't allowed gets `403`. A route not in the table keeps 1.23's rule: the page or a tool. A `/setup/gm/` route not in the table is the GM's or the page's, so a new one starts closed.

While the manager is off (see the [README](README.md)), every `/setup/gm/` route answers `409` ("The manager is off") except the version and the switch.

## The front desk, `/setup/gm/`

Reads: GM, page or tool. Actions: GM or page. Fixtures are in `Tests/fixtures/frontdesk/`.

| Route | Who | What it does | Example |
|---|---|---|---|
| `GET /setup/gm/team` | GM, page, tool | The team: the registry merged with live sessions, and the contacts. | `team.json` |
| `GET /setup/gm/events` | GM, page, tool | The event log since a time. Long-polls up to 30 seconds. | `events.json`, `event-line.json`, `hook-events.json` |
| `GET /setup/gm/version` | GM, page, tool | The app's version, `managerEnabled`, and the features it has. Works with the manager off. | `version.json` |
| `GET /setup/gm/outbox` | GM, page, tool | The messages sent and their states. The text of a message goes to the GM and the page, not a tool. | `outbox.json` |
| `GET /setup/gm/watches` | GM, page, tool | What was asked to be watched. | `watches.json` |
| `GET /setup/gm/approvals` | GM, page, tool | What waits for you. Read-only. | none yet |
| `GET /setup/gm/screens` | GM, page, tool | The screens the team can be tiled on. The Mac only: 404 on Windows and Linux. | none yet |
| `POST /setup/gm/hire` | GM, page | Hire an employee: `role` (any role but `gm`: one GM per office, so the GM's token gets a 409: "This office already has a General Manager. The owner can replace it from Setup." when one is working, "Only the owner hires the General Manager, from Setup." when none is; only the page hires or replaces the GM), and optionally `folder`, `account`, `model`, `nickname`, `floor`, `worktree`, `apiKey`, `task`. The hire works in a worktree of its own, `<folder>-hires/<name>` on branch `hire/<name>`, not in the folder given: the Go core's answer says where, in `worksIn` (and `branch`), and `smolcorp gm hire` prints "Works in <path>". | `hire.json` |
| `POST /setup/gm/fire` | GM, page | Fire one: `id`, `force`, `removeWorktree`. Answers `204`, and writes the recap; `200` with `{ok, kept, worktree, branch}` when `removeWorktree` was asked and the worktree stayed (it holds changes), saying why. | `fire.json` |
| `POST /setup/gm/brief` | GM, page | Add a new, dated part to the end of a working hire's BRIEF file (`id`, `text`), then mail the hire to read it. Refuses a fired or failed hire, an empty text, or one over the mail size setting. | `brief.json` |
| `POST /setup/gm/tile` | GM, page | Tile the team's windows. The Mac only. | none yet |
| `POST /setup/gm/screen` | GM, page | Choose the screen that holds the team. The Mac only. | none yet |
| `POST /setup/gm/send` | GM, page | Send a message: `to`, `text`, and an `id` of yours (the same `id` twice is delivered once). | `send.json` |
| `GET /setup/gm/registry` | page | Not served yet: the row only keeps a tool's refusal truthful. The GM reads `team` instead. | none |
| `POST /setup/gm/registry` | GM, page | Edit a registry entry's `nickname`, `notes` and `state`; nothing else. | `registry-edit.json`, `registry.json` |
| `POST /setup/gm/nickname` | GM, page | Name whoever works in a `folder` (a session's project or where it is now, or a pinned project): `name`, trimmed, one line of 16 characters at most; empty takes it away. Changes nothing else in the layout. `404` for a folder nobody works in and nothing is pinned to. | none |
| `POST /setup/gm/contacts` | GM, page | Which hires may write to each other: `from`, `to`, `oneWay`, `remove`. | `contacts.json` |
| `POST /setup/gm/watch` | GM, page | Set a watch: `kind` is `file`, `event`, `idle`, `exit` or `time`. | `watch.json` |
| `POST /setup/gm/unwatch` | GM, page | Take a watch away: `id` in the query or the body. | `watch.json` |
| `DELETE /setup/gm/unwatch` | GM, page | The same, as a DELETE. | `watch.json` |
| `POST /setup/gm/manager` | GM, page | The manager switch: `?enabled=1` or `0`. Works with the manager off. | `version.json` |
| `POST /setup/gm/by-hand` | page | Whether the GM's token is also kept in a file, for a GM started by hand ("Allow a GM started by hand"). Works with the manager off. The Mac only. | none |
| `POST /setup/gm/token/rotate` | page | A new GM token; the old one stops working at once. The GM cannot ask for it. The Mac only: elsewhere a GM gets a new token when it is hired again. | `gm-token-rotate-as-page.json` |
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

These are 1.23's routes, now closed to the setup-key file because a hire could misuse them. Each is for the GM or the page: `/setup/hire`, `/setup/hire/gm`, `/setup/hiring`, `/setup/roles`, `/setup/hooks`, `/setup/unpair`, `/setup/notify` (it was for the iPhone app, which is gone: 404), `/setup/webhook`, `/setup/webhook/test`, `/setup/bots`, `/setup/bots/override`, `/setup/bots/resolve`, `/setup/maxconfirm`, `/setup/phonemic`, `/setup/dictation`, `/setup/login`, `/setup/layout`, `/setup/profiles`, `/setup/pin`, `/setup/unpin`, `/setup/floors`. `POST /setup/hire/gm` starts the GM and is the page's alone in practice: the GM's token gets the same 409 as on `/setup/gm/hire` (one of the two texts above, by whether a GM is working). `GET /setup/claude-accounts` is open to everyone.

The setup page alone may spend or send elsewhere (summaries, the viewer) and handle phone codes and keys (`/setup/pair`, the QR pictures, the viewer's key), and open the folder panel on your screen (`/setup/hiring/choose`: a GM is given a folder through `hire --folder` or the hiring settings instead), and choose the keys that show and hide the office from anywhere (`/setup/officekey`). The routes under `/bots/` are for the page and your own tools.
