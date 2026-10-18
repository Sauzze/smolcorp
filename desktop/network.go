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
)

// phoneAddress is what a phone on the same network opens to pair.
func phoneAddress() string { return fmt.Sprintf("http://%s:%d", localAddress(), Port) }

// pairingLink is what the QR code opens: the pairing page with code filled in.
func pairingLink(code string) string { return phoneAddress() + "/?pair=" + code }

// setupLink is the setup page, with its key after #k=, which never leaves
// this computer: the page hands it over once for a cookie, and takes it out of
// the address. then is what the page does on opening: "pair" or "unpair".
func (d *Deck) setupLink(then string) string {
	return setupAddress("k="+d.access.SetupKey, then)
}

// openSetup opens the setup page in the browser. The browser is started
// with the link on its command line, which other people signed in to this
// computer can read, so the link carries a token that works once, for a
// couple of minutes, instead of the key: the page swaps it for the cookie.
func (d *Deck) openSetup(then string) {
	d.mu.Lock()
	token := d.access.NewSetupToken()
	d.mu.Unlock()
	openURL(setupAddress("o="+token, then))
}

func setupAddress(fragment, then string) string {
	link := fmt.Sprintf("http://localhost:%d/setup#%s", Port, fragment)
	if then != "" {
		link += "&" + then
	}
	return link
}

// Adapters a phone can't reach: containers, virtual machines, VPN tunnels.
var virtualAdapter = regexp.MustCompile(`(?i)^(docker|br-|veth|virbr|vethernet|vmnet|vbox|tun|tap|utun|wg|tailscale|zt)`)

// localAddress is the address a phone on the same network can reach this
// computer at: a private IPv4 address on an adapter that's up, preferring Wi-Fi
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

// openURL opens a page in the default browser.
func openURL(url string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}

// trimmed keeps a path short for display, with the home folder as ~.
func trimmed(path string) string { return strings.Replace(path, homeDir, "~", 1) }
