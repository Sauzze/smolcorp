# Setup, privacy and security

What Smolcorp sends and never sends, how it keeps strangers out, and using setup from a script. Back to the [README](../README.md).

## Privacy and security

- **Smolcorp sends nothing off your network unless you turn it on.** It talks
  to your phone and to the coding agents on your computer. Only these, each off
  until you turn it on, go further:
  - the address you gave [Tell other tools](extras.md#tell-other-tools), which is on
    your network unless you allow otherwise;
  - the [portable office](extras.md#portable-office-beta)'s viewer, on your own Vercel,
    which only ever holds locked data;
  - Anthropic's API, with your own key, when you ask for a
    [summary](extras.md#summaries-with-claude).

  It has no accounts, no analytics and no cloud service of its own.
- **Each phone has its own key.** Every request carries a long random key that
  only that phone has. The key is never typed: pairing hands it over. Your
  computer keeps only a fingerprint of each key (a SHA-256 hash), so you can revoke one
  phone without touching the others.
- **Buttons you make type only what you set up.** A phone names a button by
  its id; the command it types, or the key it presses, comes from your
  computer. A paired phone can't send text of its own.
- **Pairing codes are short-lived.** A code exists only after you ask for one.
  It works once and expires after five minutes. Five wrong tries from one
  device lock that device out until you get a new code, and twenty wrong tries
  in all withdraw the code. The QR code holds the same one-time code, never a
  key, and disappears once the code is used.
- **The agents have a key of their own.** Their hook uses a separate
  key, read from a file only you can read, so it never shows up in the list of
  running processes. It works only from your computer, and only for reporting what
  an agent is doing. A phone's key can't pose as an agent, and the agents' key
  can't drive the deck.
- **Scrolling, hiring and firing are off until you turn them on.** Out of the
  box, a paired phone can only use the office's keys, effort chips and
  choices, and move desks around, which pins projects to desks as the setup
  page does. Even turned on, firing asks first, and only stops the agent
  itself; hiring only starts the agent you chose, or a role's, in the folder you
  chose or in a pinned project's. A role's instructions are the agent's first
  message, there to read in its Terminal window, never a command, and a role
  changes nothing about what its agent may do without asking.
- **The screen saver pairs like a phone,** and keeps its key in its own
  settings, as a phone's browser does.
- **The office says what your agents are doing in a few words.** They're made
  from a file's name, a web page's site or the description the agent gave a
  step, never from a command or a file's path.
- **Smolcorp listens on port 7710 on your network,** so your phone can
  reach it. Without a key, all another device gets is the pairing page. Its
  pages can't be shown inside other websites, and it turns away connections
  beyond a fixed limit, so a misbehaving device can't swamp it.
- **The setup page only opens on your computer, from Smolcorp's menu.** Other
  devices can't reach it, and other websites can't press its buttons. The menu
  opens it with a link that works once, for five minutes, so other programs on
  your computer, and other people signed in to it, can't use it either. The
  setup key, which is in a file only you can read, is no longer a cookie, and a
  saved link with the key in it no longer opens setup. Your own scripts can
  still send the key as the `X-Smolcorp-Setup-Key` header, for what's listed
  under [Use setup from a script](#use-setup-from-a-script).
- **Employees are fenced, not walled, and not all of them.** The General
  Manager and the Claude Code hires it manages refuse `git push`, deploy,
  publish and cloud commands, stay out of your `.env` files and keys, and leave
  Smolcorp's own settings alone. A Claude Code hire made with the manager off,
  and every hire of Codex, Gemini CLI, OpenCode or Grok, gets no fence: see
  [who gets it, platform by platform](team.md#the-fence-and-who-gets-it). Only
  you answer a permission prompt. A script an employee writes and runs can get
  past the fence, and everyone still runs as you.
- **Use a network you trust.** The connection is plain HTTP on your local
  network. Use Smolcorp on home or office Wi-Fi, not public hotspots.

### Use setup from a script

The setup page is a set of HTTP calls to Smolcorp on this computer, and your
own scripts can make some of them. Each needs the setup key, which is new each
time Smolcorp starts and kept in a file only you can read:

| | The setup key |
|---|---|
| Mac | `~/Library/Application Support/AgentDeck/setup-key` |
| Linux | `~/.config/sauzzedeck-pro/setup-key` |
| Windows | `%APPDATA%\SauzzeDeck Pro\setup-key` |

Send it as `X-Smolcorp-Setup-Key`. Calls that change something also need
`X-AgentDeck-Setup: 1`. For example, on a Mac:

```bash
key=$(cat ~/Library/Application\ Support/AgentDeck/setup-key)
curl -s -H "X-Smolcorp-Setup-Key: $key" http://localhost:7710/setup/state
curl -s -X POST -H "X-Smolcorp-Setup-Key: $key" -H "X-AgentDeck-Setup: 1" \
  http://localhost:7710/setup/pair
```

Read the key again after Smolcorp restarts.

On a Mac, the key reads setup state, runs the check and repair that `smolcorp
setup` does, and flips the plain look-and-feel switches (theme, music, dock,
keep on top). It can't pair or unpair a phone, show a pairing code, hire,
connect or disconnect agents, change the webhook, bots or layout, or turn on
anything that spends money or sends data elsewhere. Those are for the setup
page. On Windows and Linux the same limits apply, except that a tool of yours
can also install this copy and open the firewall for how Smolcorp already
listens.
