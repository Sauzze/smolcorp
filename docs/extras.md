# Portable office, summaries and other tools

Three extras, each off until you turn it on. Back to the [README](../README.md).

## Portable office (beta)

See the office on your phone from anywhere, over 4G, while you're away. It's
view only: nothing on the phone can press a key or type into a session. It's
off until you launch it, and off again each time Smolcorp starts.

While it's on, Smolcorp locks a trimmed-down copy of the office on your Mac
(AES-256-GCM) and puts it on a small viewer you run on your own Vercel. The
key is only in your Mac's Keychain and in your link, after the `#`, which
browsers never send to any server, so the viewer only ever holds locked data.

### Set it up on Vercel

1. On Vercel, make a project from your fork of this repository. Set its Root
   Directory to `viewer/`, its Framework to "Other", and leave the build
   command empty.
2. Add **Upstash Redis** to it from the Vercel Marketplace (the free plan is
   enough). It adds `KV_REST_API_URL` and `KV_REST_API_TOKEN` for you.
3. On Smolcorp's setup page, under **Viewer link**, press **Make a write
   secret** and copy it. It's shown once. In the Vercel project, add it as
   the environment variable `VIEWER_WRITE_SECRET` (Production).
4. Deploy to production. Paste the production address
   (`https://office-viewer-xxxx.vercel.app`) into **Viewer address**, press
   **Save**, then **Check the viewer**. It says **Viewer ready ✓** or what's
   wrong.
5. Scan the QR code with your phone and add the page to your Home Screen.

A custom domain works too. Vercel Authentication doesn't, since the phone has
no Vercel login. To keep strangers from using up your free Upstash allowance,
add the firewall rule in [viewer/README.md](../viewer/README.md), which also has
the relay's protocol, if you'd rather host the viewer elsewhere.

### Start and stop it

- **Start:** menu bar → **Going away: launch portable office**. The phone shows
  the office within a few seconds. While it's on, the menu bar icon has a small
  dot, and the menu says **Portable office: ON** with when it last sent.
- **Stop:** menu bar → **Stop portable office**, or quit Smolcorp. The viewer
  forgets the office straight away, and the phone says "Not active". It also
  stops by itself when you're back at your Mac (you unlock it, or wake it and
  use it, but not in the first two minutes, so you can launch it and then
  lock), and after 8 hours. Both can be changed on the setup page.
- **Lost your phone, or shared the link by mistake:** setup → **Rotate key…**.
  Every old link stops working for good; scan the new QR code. The link holds
  the key, so keep it to yourself: **Copy viewer link** in the menu copies it
  for this Mac only, never to your other devices.
- **Wipe the viewer** empties it of everything it holds.

### What's sent, and what never is

Sent, locked: each desk's state (working, waiting, done, failed), its agent and
effort, whether it's compacting, its subagents' kinds, its role's name, badge
and outfit, the theme, and how the room is furnished. Project names too,
unless you turn on **Hide project names**, which shows "Desk 1", "Desk 2"…
instead, upstairs too, where a run's hirer becomes the hiring desk's number.

Never: transcripts, prompts, what an agent or you wrote, dictation,
file names, folders and paths, commands, models, your buttons, session ids,
your Mac's name, addresses, phone ids, and any key or token. Your write
secret, the viewer's id and its key are kept in the Keychain, never in
`config.json`.

The viewer sees when the office is on and roughly how big it is, nothing
else.

### On Windows and Linux

The portable office works the same way, from the tray: **Going away: launch
portable office** and **Stop portable office**. The write secret, the viewer's
id and its key are kept in Credential Manager on Windows, or in the Secret
Service (GNOME Keyring or KWallet) on Linux; a Linux computer with no secret
store can't keep them, so it can't use the portable office. There's no **Copy
viewer link** in the tray: copy it from the setup page.

**Stop when I'm back** works where the system says you're back: on Linux, the
screen unlocking after it was locked, or the keyboard or mouse used after a
while away, where your desktop tells logind (GNOME and KDE do); on Windows,
the keyboard or mouse used after ten minutes without. Elsewhere,
it stops after the hours you set, or when you stop it.

## Summaries with Claude

Away from the computer, a desk on the phone shows *what* an agent is doing,
not why. Hold a desk and tap **✨ summary**: Claude Haiku 5.5 reads the end of
that agent's Claude Code transcript and answers in a few lines what you asked,
what it's doing, whether it's waiting on you, and its last result.

It's off until you turn it on. It uses **your own Anthropic API key**, billed to
you by Anthropic: Haiku is Anthropic's least expensive model, and a summary is
one short request. You can use your own plan instead: Claude Code or OpenCode
can write the summary. Setup's Summaries card lists the others too, and says
which can on this computer and why one can't. The rest of this section is
about the API key.

### Turn it on

1. On the setup page, under *Getting started*, paste your API key into
   **Summaries with Claude** and click **Save key**. Make a key in the
   [Anthropic Console](https://console.anthropic.com/settings/keys).
2. Turn the switch on. A dialog says what's sent and where; it turns on only
   when you click **Turn on**.

Turn it off with the same switch, at any time. **Remove** forgets the key.

### What's sent, and where your key is kept

- **Sent, to Anthropic's API, only when you tap ✨ summary:** the end of that
  agent's transcript, at most 16,000 characters. That's:
  - your recent requests and its replies, each cut short;
  - each tool it ran, with the one file path, command or search that says
    what it did;
  - each tool's output, cut to its first line. For a file it read, that's the
    file's first line.

  Lines that look like they hold a secret (keys, tokens, passwords, private
  keys, passwords in URLs) are taken out first. The project's folder name is
  sent with it.
- **Your key** is kept by your system:
  - in the Keychain on a Mac;
  - in Credential Manager on Windows;
  - in the Secret Service (GNOME Keyring or KWallet) on Linux.

  It's never in `config.json`, the phone, the portable office or a log, and the
  setup page doesn't show any of it again.
- **Limits:** a summary is kept for 30 seconds, so a second tap doesn't pay
  twice. At most 20 summaries go to Anthropic in any 10 minutes. A failed one
  is never retried by itself.

### Or, as a fallback, `ANTHROPIC_API_KEY`

**Paste your key on the setup page if you can:** it's then kept in your
system's secret store, and only Smolcorp reads it. With no key saved there,
Smolcorp uses `ANTHROPIC_API_KEY` from its own environment instead. On a Linux
computer with no secret store, this is the only way.

The ways below give the key to **every app you start** after setting it, not
just Smolcorp. On Windows and Linux they also keep it in a plain text setting
or file, so use them only where that's acceptable. An app you open from the
Dock, the Start menu or at login doesn't see what you `export` in a terminal,
so:

- **Mac:** run `launchctl setenv ANTHROPIC_API_KEY sk-ant-…`, then quit and
  reopen Smolcorp. It lasts until you restart the Mac.
- **Windows:** run `setx ANTHROPIC_API_KEY sk-ant-…` in a terminal, then
  restart Smolcorp. It's kept in your user's environment settings.
- **Linux:** add `ANTHROPIC_API_KEY=sk-ant-…` to
  `~/.config/environment.d/smolcorp.conf`, then log out and back in. It's
  kept in that file.

Smolcorp never reads a project's `.env` file, or any other file, for a key.

## Tell other tools

Smolcorp can tell another tool what your agents are doing, so a lamp
turns orange when one waits on you, or your phone buzzes when a long task is
done. Turn on **Tell other tools** under **More** on the setup page, give it an address, and
pick what to send:

| Event | When |
|---|---|
| `waiting` | An agent is waiting on you: a permission prompt or a question |
| `done` | An agent finished, and you haven't looked yet |
| `error` | A turn ended on an error, such as a rate limit |
| `working` | An agent got to work |
| `started` | A session started |
| `ended` | A session ended |

Waiting on you, done and failed are on to begin with. Each event is a `POST`
with a JSON body:

```json
{
  "event": "waiting",
  "at": "2026-10-17T20:41:07Z",
  "session": {
    "project": "api-server",
    "agent": "claude",
    "agentName": "Claude Code",
    "state": "asking",
    "previous": "thinking",
    "key": 1
  }
}
```

`key` is the number of the desk the session is at, as the setup page numbers
them from 1, or `null` if it has none. An
`error` also has an `errorKind`, as the agent named it, such as `rate_limit`
or `overloaded`. **Send a test** sends `"event": "test"` with no session. If
you set a secret, every message carries it in an `X-Smolcorp-Secret` header.

Only addresses on your own network are allowed: your computer, a `.local`
name, a name without dots or a private IP address. Tick **Allow addresses
outside your network** to send elsewhere. Messages name the project, never its
folder, and never carry prompts or anything an agent wrote.

### Home Assistant

Add an automation with a webhook trigger, then give Smolcorp the
webhook's address, such as
`http://homeassistant.local:8123/api/webhook/smolcorp-lamp`. Home Assistant
doesn't read headers, so keep the webhook id hard to guess and leave the
secret empty.

```yaml
automation:
  - alias: Desk lamp follows the agents
    triggers:
      - trigger: webhook
        webhook_id: smolcorp-lamp
        allowed_methods: [POST]
        local_only: true
    actions:
      - choose:
          - conditions: "{{ trigger.json.event == 'waiting' }}"
            sequence:
              - action: light.turn_on
                target: { entity_id: light.desk_lamp }
                data: { color_name: orange }
          - conditions: "{{ trigger.json.event == 'done' }}"
            sequence:
              - action: light.turn_on
                target: { entity_id: light.desk_lamp }
                data: { color_name: green }
          - conditions: "{{ trigger.json.event == 'error' }}"
            sequence:
              - action: light.turn_on
                target: { entity_id: light.desk_lamp }
                data: { color_name: red }
```
