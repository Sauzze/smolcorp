# Smolcorp on Windows and Linux

Back to the [README](../README.md).

## Windows and Linux

Smolcorp runs on Windows and Linux too, with the same phone deck, the same
pairing, the same agents, roles and hiring. What's different from the Mac:

- **Start agents through Smolcorp**, in any terminal:
  `smolcorp run claude` (on Windows, `smolcorp-cli run claude`), or `codex`,
  `gemini`, `opencode` or `grok`, or `smolcorp run --role tester` for a role.
  The agent runs exactly as usual, and the deck can press its keys, with no
  permission to grant. Windows and Linux have no equivalent of Terminal's
  scripting, so this is how the deck reaches a session. A hire from the office
  opens this way, in Windows Terminal or a terminal the setup page lists.
- **In tmux, WezTerm or kitty**, agents started the usual way work too: the
  deck sends keys through the terminal's own commands. kitty needs its remote
  control turned on (`allow_remote_control yes` and `listen_on` in
  `kitty.conf`).
- **Tapping a desk** brings its pane forward in tmux, WezTerm and kitty. In the
  launcher it only selects the session, because the launcher can't tell which
  window it's in.
- **Two-finger scrolling** works in tmux and kitty.
- **Show Office**, in the tray menu: on Windows, the office in a window of its
  own; on Linux, in your browser, where Chrome and Edge can **POP OUT** it on
  top of your other windows.
- **No Accessibility step.** Nothing here needs that permission.
- **Hook commands name the office's port**, so two offices on one computer
  don't mix. Agents set up by an older Smolcorp are moved over the first time
  1.24 starts.
- **Grok Bots** sit at their desks too, but their news comes only through
  `POST /bots/<id>/event`: the Grok Bot app that the Mac reads is the Mac's
  only.

### Windows: the office in its own window

**Show Office**, in the tray menu, opens the office in a window titled
Smolcorp, with its icon. It uses WebView2, the Edge engine that comes with
Windows 10 and 11. If it's missing, a message says so and the office opens in
your browser instead.

- A second click on **Show Office** brings the open window to the front, and
  restores it if minimised. There is never a second window.
- **Always on Top** keeps the window above the others.
- Size and position are remembered (`window` in `config.json`); if the screen
  changed and the place is out of reach, it opens centred.
- The window pairs itself, as a device named "Smolcorp (window)", its key in
  `window-key` beside `hook-key`. Phones still pair with the six-digit code,
  and **Unpair All Phones** leaves the window's device alone.

### Who can reach Smolcorp (Windows and Linux)

On Windows and Linux the server listens on this computer (`127.0.0.1` and `::1`: the
setup page, the hooks, the launcher and the office window are all local) **and
on the address Tailscale gave this computer** (`100.64.0.0/10`, on the adapter
named *Tailscale*). The home network, a Wi-Fi hotspot or anything else cannot open
port 7710 at all. If Tailscale starts after Smolcorp, Smolcorp joins its address
within about 15 seconds, with no restart; if it stops, the address is let go.

- The phone's pairing address (the QR code and the address on the setup page) is
  the Tailscale one. With no Tailscale on this computer there's none, and no QR code:
  Start here's button lets in your home Wi-Fi, or you get Tailscale.
- On Linux, which listened on every interface before 1.24.0, an install with a phone
  paired already keeps doing so: its first start of 1.24.0 writes `"listen": "all"`
  in `config.json`, as the phone most likely reaches it on the home Wi-Fi. Otherwise
  it writes `"listen": "tailscale"`, so pairing a phone later changes nothing.
- Every other protection is unchanged: the 6-digit pairing code, the check of the
  host name, the connection limits, and the setup page, hooks and launcher answering
  only to this computer.
- **Windows Firewall** matches it: until setup's **Set the rules** has run,
  with your OK, Smolcorp doesn't open Tailscale's address at all, so Windows
  never asks and never makes rules of its own. The rules let TCP in from
  `100.64.0.0/10` only, and remove any older rule for a copy of Smolcorp, as
  one in `Downloads`, or one Windows made that let anyone in on a public
  network.
- **Also on my home Wi-Fi**, on Start here, lets phones on your home network
  in too: it sets `"listen": "all"` in `config.json`, adds a firewall rule for
  your local network on private networks only, and starts Smolcorp again.
  On Linux there's no firewall rule to add: it sets `"listen": "all"` and starts
  Smolcorp again. Run as `smolcorp serve`, under systemd say, or with no display,
  it starts again in place: the same process, which prints its new setup link
  in its terminal or log, and stops as before. By hand, `"listen": "all"` in `config.json` (in
  `%APPDATA%\SauzzeDeck Pro`, or `~/.config/sauzzedeck-pro` on Linux) does the
  same once Smolcorp starts again, without the firewall's rule. The Mac listens
  on every interface.
- `SMOLCORP_PORT` (1024-65535) replaces port 7710, to try a build next to the one
  you use. It is for tests: the hooks and the launcher read it too, so set it for
  everything that should talk to that copy.

### One Smolcorp at a time

Starting `Smolcorp.exe` while Smolcorp is already running does not start another
tray icon. The second launch asks the running one to open the office window (or
bring it to the front) and exits at once, with code 0, or code 1 if no browser
could open the office (it then prints the office address to open by hand). If
the running one is an older Smolcorp that cannot be asked, a message says that
Smolcorp is already running. On Windows the lock is a named mutex of the session
(`Local\Smolcorp-single-instance`); on Linux, a lock on `instance.lock` next to
the keys. If the port is taken by something else, Smolcorp says so and exits,
rather than stay in the tray without a server.

### Linux

Download **Smolcorp-Linux-x86_64.tar.gz** (or `-arm64`) from the
[latest release](https://github.com/Sauzze/smolcorp/releases/latest), then:

```bash
tar -xzf Smolcorp-Linux-x86_64.tar.gz
./Smolcorp-Linux-x86_64/install.sh
```

This installs it for you alone, in `~/.local`, with an entry in your app
menu. Open **Smolcorp** from the menu, or run `smolcorp`. The setup
page opens in your browser.

- To update, run the new `install.sh`: from 1.24 on it stops the running
  Smolcorp itself first. `smolcorp quit` stops it by hand. (1.22 can't be
  stopped this way: quit it from its tray, or press Ctrl+C in its terminal,
  before you run the new installer.)
- On GNOME, the tray icon needs the AppIndicator extension, which Ubuntu
  includes. Without it, Smolcorp still runs, and prints a setup link as it
  starts. The link works once, for five minutes; for a new one, run
  `smolcorp quit` (or press Ctrl+C in its terminal) and start Smolcorp again.
- On a machine without a desktop, such as one you reach over SSH, run
  `smolcorp serve`. It prints the setup page's link as it starts. Open it
  through a tunnel: `ssh -L 7710:localhost:7710` to the machine, then the
  link in your own browser.

### Windows

Download **Smolcorp-Windows-x86_64.zip** (or `-arm64` for an ARM PC), extract
it and open **Smolcorp.exe** (see [Install](install.md#install)). It installs itself in
`%LOCALAPPDATA%\Smolcorp` and starts from there, so your agents' hooks and the
firewall's rules never name the folder you downloaded to. Opening a newer
download later updates it the same way: quit Smolcorp from its tray icon
first.

### Build it yourself

You need [Go](https://go.dev) 1.26 or later:

```bash
go build ./cmd/smolcorp
```

On Windows, that one program runs everything, in a console window of its
own. To build it as the downloads have it, with no console window, build
both:

```bash
go build -ldflags "-H windowsgui" -o Smolcorp.exe ./cmd/smolcorp
go build -o smolcorp-cli.exe ./cmd/smolcorp
```

`go test ./...` runs the tests, and `scripts/release-desktop.sh` builds the
downloads for both systems.

The version shown by `smolcorp-cli version` is `dev` unless the build sets it:

```bash
go build -ldflags "-X main.version=1.2.3" -o smolcorp-cli ./cmd/smolcorp
```

(on Windows, add `-s -w -H windowsgui` for `Smolcorp.exe`, as above;
`scripts/release-desktop.sh` passes its own version.)
