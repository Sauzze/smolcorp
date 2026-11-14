package desktop

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// phoneAddress is what a phone opens to pair: this computer on the local network,
// or on Tailscale when that is the only way in (see phoneHost).
func phoneAddress() string {
	host := phoneHost()
	if host == "" {
		return ""
	}
	return fmt.Sprintf("http://%s:%d", host, Port)
}

// pairingLink is what the QR code opens: the pairing page, with the key it hands over after # (SECWIFI),
// which a browser never sends, so it never crosses the network; the phone signs with it to claim it.
func pairingLink(key string) string { return phoneAddress() + "/#pair=" + key }

// setupLink is the setup page, with a token after #o= that works once, for a
// minute off a Mac (setupTokenLifetime): the key is never in a link. A token
// used twice ends the session it opened first (setuplink.go). The page hands the token over for
// a session cookie and takes it out of the address. then is what the page does
// on opening: "pair" or "unpair".
func (d *Deck) setupLink(then string) string {
	d.mu.Lock()
	token := d.access.NewSetupToken()
	d.mu.Unlock()
	return setupAddress("o="+token, then)
}

// openSetup opens the setup page in the browser. The browser is started
// with the link on its command line, which any program of yours can read,
// so the link carries a token that works once, for a minute, instead of the
// key: the page swaps it for the cookie, and a token another program used
// first is reported and its session ended (setuplink.go).
// openSetupPage opens setup for the office's ⚙︎: a variable, for the tests.
var openSetupPage = func(d *Deck) error { return d.openSetup("") }

// showGuideAgain is the tray's "Show Me Around Again": the first-run guide is
// on again, saved, and setup opens on it.
func (d *Deck) showGuideAgain() error {
	d.guideAgain()
	return openGuidePage(d)
}

// guideAgain turns the first-run guide on again, saved, so setup starts it
// however it opens: the Mac's menus then open their Settings window on it.
func (d *Deck) guideAgain() {
	d.mu.Lock()
	d.config.GuideDone = false
	d.config.save()
	d.mu.Unlock()
}

// openGuidePage opens setup on the guide: a variable, for the tests.
var openGuidePage = func(d *Deck) error { return d.openSetup("guide") }

// It says if the browser could not be started; the tray and the first run have no one to tell.
func (d *Deck) openSetup(then string) error {
	d.mu.Lock()
	token := d.access.NewSetupToken()
	d.mu.Unlock()
	return openURL(setupAddress("o="+token, then))
}

func setupAddress(fragment, then string) string {
	link := setupBase() + "/setup#" + fragment
	if then != "" {
		link += "&" + then
	}
	return link
}

// Adapters a phone can't reach: containers, virtual machines, VPN tunnels.
var virtualAdapter = regexp.MustCompile(`(?i)^(docker|br-|veth|virbr|vethernet|vmnet|vbox|tun|tap|utun|wg|tailscale|zt)`)

// localAddress is the address a phone on the same network can reach this
// computer at (Tailscale's own is left to phoneHost, as it isn't this network): a private IPv4 address on an adapter that's up, preferring Wi-Fi
// and Ethernet, which have hardware addresses, over anything virtual.
func localAddress() string {
	adapters, _ := net.Interfaces()
	fallback := ""
	for _, adapter := range adapters {
		if adapter.Flags&net.FlagUp == 0 || adapter.Flags&net.FlagLoopback != 0 || virtualAdapter.MatchString(adapter.Name) {
			continue
		}
		addresses, _ := adapter.Addrs()
		for _, address := range addresses {
			network, ok := address.(*net.IPNet)
			if !ok || network.IP.To4() == nil || !network.IP.IsPrivate() {
				continue
			}
			if len(adapter.HardwareAddr) > 0 {
				return network.IP.String()
			}
			if fallback == "" {
				fallback = network.IP.String()
			}
		}
	}
	if fallback == "" {
		return "localhost"
	}
	return fallback
}

// launchCommand is how to start the launcher from a terminal: by name when
// the program is on PATH, by its full path when it isn't.
func launchCommand() string {
	program := commandLineProgram()
	// As Windows names it, and by the name it had before, while that's the one on PATH
	for _, name := range []string{"smolcorp", "smolcorp-cli", "sauzzedeck"} {
		if found, err := exec.LookPath(name); err == nil {
			if resolved, err := filepath.EvalSymlinks(found); err == nil && sameFile(resolved, program) {
				return name + " run"
			}
		}
	}
	return shellPath(program) + " run"
}

func sameFile(a, b string) bool {
	first, err1 := os.Stat(a)
	second, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(first, second)
}

// openURL opens a page in the default browser: a variable, for the tests.
// It says if the browser could not be started.
var openURL = func(url string) error {
	switch runtime.GOOS {
	case "windows":
		return startAndWait(disclaim.Cmd(exec.Command("rundll32", "url.dll,FileProtocolHandler", url)))
	case "darwin":
		return startAndWait(exec.Command("/usr/bin/open", url))
	default:
		return startAndCheck(disclaim.Cmd(exec.Command("xdg-open", url)), xdgOpenWait)
	}
}

// xdgOpenWait is how long xdg-open has to say it found no browser: with
// none, it exits at once with a status of its own (3 or 4), where a browser
// it starts runs on. A variable, for the tests.
var xdgOpenWait = time.Second

// startAndCheck starts cmd as startAndWait does, and says it failed if it
// exits with an error within wait: a browser that wouldn't start is then a
// 502 with the way into setup that works, not a ⚙︎ that does nothing
// (SIERRA 1.23.0 gear). Still running by then, it's taken to have worked.
func startAndCheck(cmd *exec.Cmd, wait time.Duration) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	pid := cmd.Process.Pid
	children.Lock()
	children.pids[pid] = true
	children.Unlock()
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
		children.Lock()
		delete(children.pids, pid)
		children.Unlock()
	}()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(cmd.Path), err)
		}
	case <-time.After(wait):
	}
	return nil
}

// setupAdvice is the way into setup that works today when the browser would not open, for the computer it runs on.
// On a Mac it names the menu bar icon's Settings… item and the app binary's --setup --open.
func setupAdvice(goos string) string {
	switch goos {
	case "darwin":
		return "choose Settings… from Smolcorp's menu bar icon, or run /Applications/Smolcorp.app/Contents/MacOS/AgentDeck --setup --open"
	case "windows": // the tray is always there; no command makes a setup link
		return "choose Open Setup… from Smolcorp's tray icon once your browser opens links, or quit Smolcorp from its tray icon and start it again for a new link"
	default: // Linux: the same tray item, and with no tray the only new link comes from quitting and starting again
		return "choose Open Setup… from Smolcorp's tray icon once your browser opens links; with no tray, for a new link " + noTrayNewLink
	}
}

// trimmed keeps a path short for display, with the home folder as ~.
func trimmed(path string) string { return strings.Replace(path, homeDir, "~", 1) }
