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
)

// selfTest is the checklist, in the order it's done in. Without the lock.
func (d *Deck) selfTest() []check {
	checks := []check{}
	if install := installCheck(); install != nil {
		checks = append(checks, *install)
	}
	checks = append(checks, hooksCheck(foundAgents(), isConnected, hooksPointHere))
	d.mu.Lock()
	scope := scopeOf(d.config)
	phones := phonesOnly(d.access.Phones)
	d.mu.Unlock()
	checks = append(checks, networkCheck(scope, tailscaleAddresses()))
	if firewall := firewallCheck(scope); firewall != nil {
		checks = append(checks, *firewall)
	}
	checks = append(checks, phoneCheck(phones), loginCheck(isOpenAtLogin()))
	return checks
}

// ready is whether nothing in checks is in the way: a step still to take
// isn't.
func ready(checks []check) bool {
	return !slices.ContainsFunc(checks, func(c check) bool { return c.Status == checkFix })
}

// hooksCheck is whether every agent found reports to this Smolcorp: hooked
// up, and naming this copy, not one in Downloads or one since removed.
func hooksCheck(found []Agent, connected func(Agent) bool, here func(Agent) (bool, string)) check {
	c := check{ID: "hooks", Title: "Your agents report to Smolcorp"}
	if len(found) == 0 {
		c.Status, c.Detail = checkFix, errNoAgents.Error()
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
		c.Status, c.Fix, c.Label = checkFix, fixRepair, "Connect them"
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
	case len(tailscale) > 0 && scope == listenTailscale:
		c.Status = checkOK
		c.Detail = fmt.Sprintf("Through Tailscale, at %s, from anywhere your phone is signed in to it. Not on the Wi-Fi itself: "+
			"only this computer and your Tailscale devices can open Smolcorp's port.", tailscale[0])
		c.Fix, c.Label = fixHomeWiFi, "Also on my home Wi-Fi"
	case len(tailscale) > 0:
		c.Status = checkOK
		c.Detail = fmt.Sprintf("On this network, and through Tailscale at %s from anywhere.", tailscale[0])
	case scope == listenTailscale:
		c.Status, c.Fix, c.Label = checkTodo, fixTailscale, "Get Tailscale"
		c.Detail = "Smolcorp waits for Tailscale, a free private network between your own devices: install it here and on " +
			"your phone, signed in to the same account, and your phone reaches this computer from anywhere. " +
			"Or let phones on your home Wi-Fi in."
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
	c.Status, c.Detail = checkTodo, "Scan the code under Pair a phone with your phone's camera, then type the 6 digits it shows."
	return c
}

// loginCheck is whether Smolcorp opens at login, so desks are there from
// the first session of the day.
func loginCheck(on bool) check {
	c := check{ID: "login", Title: "Smolcorp opens at login"}
	if on {
		c.Status, c.Detail = checkOK, "It's in the tray from the moment you sign in."
		return c
	}
	c.Status, c.Fix, c.Label = checkTodo, fixLogin, "Open at login"
	c.Detail = "Otherwise, sessions started before you open Smolcorp aren't at a desk until their next turn."
	return c
}

var errUnknownFix = errors.New("There's no fix by that name.")

// fix runs the fix named name, and says how it went: a message for the page
// where it couldn't. Without the lock.
func (d *Deck) fix(name string) (int, string) {
	switch name {
	case fixRepair:
		if err := connectAgents(); err != nil {
			return http.StatusConflict, err.Error()
		}
	case fixInstall:
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
