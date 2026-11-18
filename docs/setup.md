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
  computer.
- **Text from a paired phone goes only where you send it.** What you type or
  dictate on the phone goes to the agent you picked, as text: never to an
  agent that's asking you something, never starting with `!` (Claude Code
  would run it as a shell command), 2,000 characters at most, with Return
  pressed on its own afterwards. Over your Wi-Fi it isn't encrypted; over
  Tailscale it is. It's on unless you turn it off: on the Mac with setup's
  **Phone microphone** switch, on Windows and Linux with `"phoneMicEnabled":
  false` in `config.json`.
  See [Typing from the phone](phone.md#typing-from-the-phone).
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
  opens it with a link that works once: for five minutes on a Mac, and for one
  minute on Windows and Linux. If another program on your computer opens it
  first, Smolcorp ends that program's session at once and your page says so.
  The setup key lives in Smolcorp's memory only, never in a file, and a saved
  link no longer opens setup. Your own scripts can get a key of their own, for
  what's listed under [Use setup from a script](#use-setup-from-a-script).
- **Smolcorp's keys stay out of reach of the agents it starts.** How, what you'll
  notice, and what it can't stop are in
  [Keys, records and the guard](keys-and-guard.md).
- **On Windows, "a file only you can read" means your profile's folder.**
  Smolcorp sets no permissions of its own on its files there (the agents' key,
  the guard's list): they're as private as `%APPDATA%`, which
  Windows lets only you, administrators and the system read. Fenced hires
  also have deny rules that keep them away from these files. On the Mac and
  Linux, Smolcorp writes them so only you can read them.
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

The setup page is a set of HTTP calls to Smolcorp on this computer. No file
holds a key to them: the setup key lives in Smolcorp's memory only, new each
time it starts, so nothing that can read your files can use it.

- **The checklist.** `smolcorp setup` (on a Mac, `AgentDeck --setup`) reads it
  with the check-only key, in the `selftest-key` file in Smolcorp's folder. That
  key opens `GET /setup/selftest` and nothing else, sent as
  `X-Smolcorp-Selftest-Key`.
- **Anything more.** A script asks Smolcorp for a key, as `smolcorp demo`,
  `status` and `desks` do: `POST /terminal/ask?kind=tool` (with
  `X-AgentDeck-Setup: 1`) answers a ticket and a four-digit code to show. Setup
  then shows the ask with the same code, and once you tick that the codes match
  and click **Allow**, `GET /terminal/ask?ticket=…` answers the key, once. Send
  it as `X-Smolcorp-Setup-Key`; it works until Smolcorp quits. Only
  Smolcorp's own Settings window answers an ask, never Setup in a browser, so
  this works on a Mac alone for now. On Windows and Linux the ask is refused
  at once, with why: only your paired phone (and on Windows, Smolcorp's own
  window) may say yes to a terminal, and neither can yet.

A tool's key reads setup state, runs the repair, and flips the plain
look-and-feel switches (theme, music, dock, keep on top). It can't pair or
unpair a phone, show a pairing code, hire, connect or disconnect agents, change
the webhook, bots or layout, change the office's shortcut, answer an ask, or
turn on anything that spends money or sends data elsewhere. Those are for the
setup page.
