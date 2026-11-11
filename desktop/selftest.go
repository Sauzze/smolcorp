package desktop

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// The self-test: setup's "Start here" checklist, and what `smolcorp setup`
// prints. Each check only looks; what puts one right is a fix, asked for by
// its name (see fix), so a check can be run as often as the page likes.

// check is one line of the checklist.
type check struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	// Fix is the fix's name for POST /setup/fix, and Label its button's words,
	// where there's one to press.
	Fix   string `json:"fix,omitempty"`
	Label string `json:"label,omitempty"`
}

// A check's status.
const (
	checkOK   = "ok"   // done
	checkFix  = "fix"  // wrong, and in the way
	checkTodo = "todo" // a step still to take, such as pairing a phone
	checkInfo = "info" // worth knowing, and nothing to do
)

// The fixes, by name.
const (
	fixRepair    = "repair"    // every agent's hooks, to this copy of Smolcorp
	fixInstall   = "install"   // this copy into its own folder, started from there
	fixFirewall  = "firewall"  // Windows Firewall's rules for the port, with one OK
	fixHomeWiFi  = "homewifi"  // phones on the home network too, not only Tailscale
	fixTailscale = "tailscale" // Tailscale's download page
	fixLogin     = "login"     // Smolcorp opens at login
	// Under the Mac app only (hostshell.go).
	fixAccessibility = "accessibility" // macOS asked to let Smolcorp use the keyboard
)

// selfTest is the checklist, in the order it's done in. Without the lock.
func (d *Deck) selfTest() []check {
	checks := []check{}
	if restart := restartCheck(); restart != nil {
		checks = append(checks, *restart)
	}
	if h := d.macHost(); h != nil {
		// Under the Mac app, where the app is, as the Mac's own self-test says it (hostshell.go). Without
		// the app's answer there's no line, rather than a made-up one.
		if install, err := hostInstallCheck(h); err == nil {
			checks = append(checks, install)
		}
	} else if install := installCheck(); install != nil {
		checks = append(checks, *install)
	}
	if leavesAgentsAlone() {
		checks = append(checks, testCopyHooksCheck())
	} else {
		checks = append(checks, hooksCheck(foundAgents(), isConnected, hooksPointHere))
	}
	d.mu.Lock()
	scope := effectiveScope(d.config)
	phones := phonesOnly(d.access.Phones)
	d.mu.Unlock()
	// Under the Mac app, the rest is the Mac's (hostshell.go).
	if d.macHost() != nil {
		return append(checks, d.macChecks(phones)...)
	}
	checks = append(checks, networkCheck(scope, tailscaleAddresses()))
	if scope != listenThisComputer {
		if firewall := firewallCheck(scope); firewall != nil {
			checks = append(checks, *firewall)
		}
	}
	checks = append(checks, phoneCheck(phones), loginCheck(isOpenAtLogin(), hasTray()))
	return checks
}

// ready is whether nothing in checks is in the way: a step still to take
// isn't.
func ready(checks []check) bool {
	return !slices.ContainsFunc(checks, func(c check) bool { return c.Status == checkFix })
}

// testCopyHooksCheck is a test copy's hooks line, as the Mac's (SelfTest.hooksCheck): ticked, with no Connect, as
// a test copy never writes your agents' settings (errTestCopyAgents) and its hires report through a hook of their own.
//
// A copy that is on a port of its own but isn't marked a test copy (no SMOLCORP_TEST_SUPPORT_DIR beside the port:
// a real install on SMOLCORP_PORT, say) leaves your agents alone just the same, but says so: not connected, and why,
// where it used to tick a line that was never true (RFXC1a N1).
func testCopyHooksCheck() check {
	if !testCopyMarked() {
		return check{ID: "hooks", Status: checkTodo, Title: "Your agents report to Smolcorp", Detail: notConnectedOnPort()}
	}
	return check{ID: "hooks", Status: checkOK, Title: "Your agents report to Smolcorp", Detail: testCopyHires + " Sessions you start yourself report to the Smolcorp you use."}
}

// testCopyHires is the Mac's SelfTest.testCopyHires, word for word.
const testCopyHires = "Your hires report to this test copy through its own hook."

// notConnectedOnPort is what a copy on a port of its own, not marked a test copy, says of your agents: they aren't
// connected, because only a copy on the office's own port connects them.
func notConnectedOnPort() string {
	return fmt.Sprintf("Not connected: Smolcorp is on port %d, not %d, so it leaves your agents' settings alone. "+
		"To connect them, run Smolcorp on port %d.", Port, portFromEnvironment(""), portFromEnvironment(""))
}

// hooksCheck is whether every agent found reports to this Smolcorp: hooked
// up, and naming this copy, not one in Downloads or one since removed.
func hooksCheck(found []Agent, connected func(Agent) bool, here func(Agent) (bool, string)) check {
	c := check{ID: "hooks", Title: "Your agents report to Smolcorp"}
	if len(found) == 0 {
		c.Status, c.Detail = checkFix, noAgentsFound().Error()
		return c
	}
	var good, wrong []string
	for _, agent := range found {
		switch ok, elsewhere := here(agent); {
		case !connected(agent):
			wrong = append(wrong, agent.Name()+" isn't connected")
		case !ok:
			wrong = append(wrong, fmt.Sprintf("%s reports to another copy of Smolcorp (%s)", agent.Name(), elsewhere))
		default:
			good = append(good, agent.Name())
		}
	}
	if len(wrong) > 0 {
		c.Status, c.Fix, c.Label = checkFix, fixRepair, "Connect"
		c.Detail = strings.Join(wrong, "; ") + "."
		return c
	}
	c.Status, c.Detail = checkOK, strings.Join(good, ", ")+": each new session shows up at a desk."
	return c
}

// hooksPointHere is whether every one of the deck's hooks in agent's
// settings runs this copy, and the command of one that doesn't. An agent
// with a plugin rather than settings (OpenCode) is as its plugin file is.
func hooksPointHere(agent Agent) (bool, string) {
	entries := entriesFor(agent)
	if entries == nil {
		return true, ""
	}
	settings, err := readSettings(entries.file)
	if err != nil {
		return false, entries.file
	}
	want, _ := entries.handler["command"].(string)
	hooks, _ := settings["hooks"].(map[string]any)
	for _, event := range entries.events {
		groups, _ := hooks[event].([]any)
		for _, group := range groups {
			g, _ := group.(map[string]any)
			handlers, _ := g["hooks"].([]any)
			for _, handler := range handlers {
				h, _ := handler.(map[string]any)
				if command, _ := h["command"].(string); isOurs(handler) && command != want {
					return false, command
				}
			}
		}
	}
	return true, ""
}

// networkCheck is how a phone reaches this computer, by the listening
// scope and Tailscale's addresses here.
func networkCheck(scope string, tailscale []string) check {
	c := check{ID: "network", Title: "Your phone can reach this computer"}
	switch {
	case scope == listenThisComputer:
		// A test copy: nothing on the network reaches it, and nothing here offers to change that (RTCH N2).
		c.Status = checkInfo
		c.Detail = "This test copy listens on this computer only, so no phone reaches it. Open it from a browser on this computer."
	case len(tailscale) > 0 && scope == listenTailscale:
		c.Status = checkOK
		c.Detail = fmt.Sprintf("Through Tailscale, at %s, from anywhere your phone is signed in to it. Not on the Wi-Fi itself: "+
			"only this computer and your Tailscale devices can open Smolcorp's port.", tailscale[0])
		c.Fix, c.Label = fixHomeWiFi, "Also on my home Wi-Fi"
	case len(tailscale) > 0:
		c.Status = checkOK
		c.Detail = fmt.Sprintf("On this network, and through Tailscale at %s from anywhere.", tailscale[0])
	case scope == listenTailscale:
		// No phone can reach this computer yet: the home Wi-Fi is the button, as it works at once.
		c.Status, c.Fix, c.Label = checkTodo, fixHomeWiFi, "Let in my home Wi-Fi"
		c.Detail = "No phone can reach this computer yet. Let phones on your home Wi-Fi in, or get Tailscale " +
			"(tailscale.com/download), a free private network between your own devices: installed here and on your phone, " +
			"signed in to the same account, your phone reaches this computer from anywhere, and the Wi-Fi needn't be let in."
	default:
		c.Status, c.Fix, c.Label = checkInfo, fixTailscale, "Get Tailscale"
		c.Detail = "On this network only. With Tailscale, free, on this computer and your phone, it reaches you from anywhere."
	}
	return c
}

// phoneCheck is whether a phone is paired, the office window not counted.
func phoneCheck(phones int) check {
	c := check{ID: "phone", Title: "A phone is paired"}
	if phones > 0 {
		c.Status, c.Detail = checkOK, fmt.Sprintf("%d paired. Pair another from Phone and voice.", phones)
		return c
	}
	c.Status, c.Detail = checkTodo, "Open Pair a phone, then scan its QR code with your phone's camera, or type its 6-digit code on your phone."
	return c
}

// loginCheck is whether Smolcorp opens at login, so desks are there from
// the first session of the day. tray is whether there's a tray to see it in.
func loginCheck(on, tray bool) check {
	c := check{ID: "login", Title: "Smolcorp opens at login"}
	if on {
		c.Status, c.Detail = checkOK, "It's in the tray from the moment you sign in."
		if !tray {
			c.Detail = "It starts when you sign in."
		}
		return c
	}
	c.Status, c.Fix, c.Label = checkTodo, fixLogin, "Open at login"
	c.Detail = "Otherwise, sessions started before you open Smolcorp aren't at a desk until their next turn."
	return c
}

// errHooksDisconnected is what setup's own repair says when you disconnected
// the agents: it doesn't connect them again behind your back.
var errHooksDisconnected = errors.New("Your agents are disconnected, as you left them, so setup didn't connect them. To connect them again, press Connect on the setup page.")

// hooksDisconnected: you disconnected the agents, and haven't connected them since.
func (d *Deck) hooksDisconnected() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.config.HooksDisconnected
}

func (d *Deck) setHooksDisconnected(off bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.config.HooksDisconnected != off {
		d.config.HooksDisconnected = off
		d.config.save()
	}
}

var errUnknownFix = errors.New("There's no fix by that name.")

// fix runs the fix named name, and says how it went: a message for the page
// where it couldn't. Without the lock.
func (d *Deck) fix(name string) (int, string) {
	// A test copy runs none that opens another app, a download page, System Settings or a system prompt (RTCH N1).
	// Accessibility is the Mac's alone: elsewhere it's no fix at all, test copy or not (RWTC N5).
	switch name {
	case fixAccessibility:
		if why := testCopyKeepsAway(testCopyKeepsAccessibility); why != "" && d.macHost() != nil {
			return http.StatusConflict, why
		}
	case fixTailscale, fixFirewall, fixHomeWiFi:
		if why := testCopyKeepsAway(testCopyKeepsFixes); why != "" {
			return http.StatusConflict, why
		}
	}
	// Under the Mac app, Accessibility and the login item are the app's (hostshell.go).
	if status, message, done := d.macFix(name); done {
		return status, message
	}
	switch name {
	case fixRepair:
		// Repair connects what was connected; it never turns voice on, so there's nothing to tell
		if _, err := connectAgents(false); err != nil {
			return http.StatusConflict, err.Error()
		}
		d.setHooksDisconnected(false)
	case fixInstall:
		// A test copy never installs itself over the Smolcorp in use (RWTC N3), as the Mac's.
		if why := testCopyKeepsAway(testCopyKeepsInstall); why != "" {
			return http.StatusConflict, why
		}
		if err := installNow(); err != nil {
			return http.StatusConflict, err.Error()
		}
	case fixFirewall, fixHomeWiFi:
		d.mu.Lock()
		scope := scopeOf(d.config)
		if name == fixHomeWiFi {
			scope = listenAll
		}
		d.mu.Unlock()
		if err := fixFirewallFor(scope); err != nil {
			return http.StatusConflict, err.Error()
		}
		if name == fixHomeWiFi {
			d.mu.Lock()
			d.config.Listen = listenAll
			d.config.save()
			d.mu.Unlock()
			// Read at start: Smolcorp starts again to listen on the Wi-Fi too.
			restartSoon()
		}
	case fixTailscale:
		openURL("https://tailscale.com/download")
	case fixLogin:
		// Opening at login is the Smolcorp in use's: a test copy's entry would start the copy in its place (RWTC N3).
		// Under the Mac app the app says so itself (macFix).
		if why := testCopyKeepsAway(testCopyKeepsLogin); why != "" {
			return http.StatusConflict, why
		}
		setOpenAtLogin(true)
	default:
		return http.StatusBadRequest, errUnknownFix.Error()
	}
	return http.StatusNoContent, ""
}

// selfTestReport is the checklist as the page and `smolcorp setup` read it.
func selfTestReport(checks []check) map[string]any {
	return map[string]any{"checks": checks, "ready": ready(checks)}
}

// installedFromVariable names the folder a copy installed itself from.
const installedFromVariable = "SMOLCORP_INSTALLED_FROM"
