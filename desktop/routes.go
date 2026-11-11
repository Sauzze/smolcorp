package desktop

import "strings"

// routeRow says which callers may use a setup route: one row of the table
// Tests/fixtures/frontdesk/routes.json, which the Mac app writes from its
// SetupRoutes. The table below is a copy of it kept in step by a test
// (TestTheRouteTableIsTheFixture), since the fixture lives under Tests/ and
// cannot be embedded.
type routeRow struct {
	method string
	// A path, or a prefix ending in /*.
	path    string
	callers []string
}

var routeTable = []routeRow{
	{"GET", "/setup/gm/team", []string{"gm", "page", "tool"}},
	{"GET", "/setup/gm/events", []string{"gm", "page", "tool"}},
	{"GET", "/setup/gm/version", []string{"gm", "page", "tool"}},
	{"GET", "/setup/gm/outbox", []string{"gm", "page", "tool"}},
	{"GET", "/setup/gm/watches", []string{"gm", "page", "tool"}},
	{"GET", "/setup/gm/approvals", []string{"gm", "page", "tool"}},
	{"GET", "/setup/gm/screens", []string{"gm", "page", "tool"}},
	{"POST", "/setup/gm/hire", []string{"gm", "page"}},
	{"POST", "/setup/gm/fire", []string{"gm", "page"}},
	{"POST", "/setup/gm/brief", []string{"gm", "page"}},
	{"POST", "/setup/gm/tile", []string{"gm", "page"}},
	{"POST", "/setup/gm/screen", []string{"gm", "page"}},
	{"POST", "/setup/gm/send", []string{"gm", "page", "hire"}},
	{"POST", "/setup/gm/contacts", []string{"gm", "page"}},
	{"POST", "/setup/gm/registry", []string{"gm", "page"}},
	{"POST", "/setup/gm/nickname", []string{"gm", "page"}},
	{"GET", "/setup/gm/registry", []string{"page"}},
	{"POST", "/setup/gm/watch", []string{"gm", "page"}},
	{"POST", "/setup/gm/unwatch", []string{"gm", "page"}},
	{"DELETE", "/setup/gm/unwatch", []string{"gm", "page"}},
	{"POST", "/setup/gm/manager", []string{"gm", "page"}},
	{"POST", "/setup/gm/token/rotate", []string{"page"}},
	{"POST", "/setup/gm/by-hand", []string{"page"}},
	{"*", "/setup/gm/*", []string{"gm", "page"}},
	{"POST", "/setup/hire", []string{"gm", "page"}},
	{"POST", "/setup/hire/gm", []string{"gm", "page"}},
	{"GET", "/setup/hire/gm/preflight", []string{"page"}},
	{"POST", "/setup/hiring", []string{"gm", "page"}},
	{"POST", "/setup/hiring/choose", []string{"gm", "page"}},
	{"POST", "/setup/hire-folder", []string{"gm", "page"}},
	{"POST", "/setup/roles", []string{"gm", "page"}},
	{"POST", "/setup/hooks", []string{"gm", "page"}},
	{"POST", "/setup/unpair", []string{"gm", "page"}},
	{"POST", "/setup/notify", []string{"gm", "page"}},
	{"POST", "/setup/webhook", []string{"gm", "page"}},
	{"POST", "/setup/webhook/test", []string{"gm", "page"}},
	{"POST", "/setup/bots", []string{"gm", "page"}},
	{"POST", "/setup/bots/override", []string{"gm", "page"}},
	{"POST", "/setup/bots/resolve", []string{"gm", "page"}},
	{"POST", "/setup/bots/mailbox", []string{"page"}},
	{"POST", "/setup/maxconfirm", []string{"gm", "page"}},
	{"POST", "/setup/phonemic", []string{"gm", "page"}},
	{"POST", "/setup/dictation", []string{"gm", "page"}},
	{"POST", "/setup/login", []string{"gm", "page"}},
	{"POST", "/setup/layout", []string{"gm", "page"}},
	{"POST", "/setup/profiles", []string{"gm", "page"}},
	{"GET", "/setup/claude-accounts", []string{"gm", "page", "tool"}},
	{"GET", "/setup/guard", []string{"gm", "page", "tool"}},
	{"POST", "/setup/pin", []string{"gm", "page"}},
	{"POST", "/setup/unpin", []string{"gm", "page"}},
	{"POST", "/setup/floors", []string{"gm", "page"}},
	{"POST", "/setup/summaries", []string{"page"}},
	{"POST", "/setup/summary/provider", []string{"page"}},
	{"POST", "/setup/summary/key", []string{"page"}},
	{"POST", "/setup/viewer", []string{"page"}},
	{"POST", "/setup/viewer/switch", []string{"page"}},
	{"POST", "/setup/viewer/wipe", []string{"page"}},
	{"POST", "/setup/briefs/restore", []string{"page"}},
	{"POST", "/setup/claude-accounts", []string{"page"}},
	{"POST", "/setup/terminal-command", []string{"page"}},
	{"POST", "/setup/guard", []string{"page"}},
	{"POST", "/setup/keystrokes", []string{"page"}},
	{"POST", "/setup/keystrokes/apps", []string{"page"}},
	{"POST", "/setup/choices", []string{"page"}},
	{"POST", "/setup/hire/window", []string{"page"}},
	{"GET", "/setup/t3code", []string{"page"}},
	{"POST", "/setup/t3code/open", []string{"page"}},
	{"POST", "/setup/t3code/letgo", []string{"page"}},
	{"POST", "/setup/pair", []string{"page"}},
	{"GET", "/setup/qr.png", []string{"page"}},
	{"GET", "/setup/viewer/qr.png", []string{"page"}},
	{"POST", "/setup/viewer/secret", []string{"page"}},
	{"POST", "/setup/viewer/rotate", []string{"page"}},
	{"*", "/bots/*", []string{"page", "tool"}},
}

// name is how the table spells a caller.
func (c caller) name() string {
	switch c {
	case callerPage:
		return "page"
	case callerTool:
		return "tool"
	case callerGM:
		return "gm"
	case callerHire:
		return "hire"
	}
	return "nobody"
}

// routeFor is the row for a request: an exact path first, then the longest prefix.
func routeFor(method, path string) *routeRow {
	fits := func(row *routeRow) bool { return row.method == "*" || row.method == method }
	var best *routeRow
	for i := range routeTable {
		row := &routeTable[i]
		if !fits(row) {
			continue
		}
		if row.path == path {
			return row
		}
		if prefix, ok := strings.CutSuffix(row.path, "*"); ok && strings.HasSuffix(prefix, "/") && strings.HasPrefix(path, prefix) &&
			(best == nil || len(row.path) > len(best.path)) {
			best = row
		}
	}
	return best
}

// routeAllows is whether a caller may use a route. A route not in the table
// keeps 1.23's rule: the page, or a tool with the setup-key file.
func routeAllows(who caller, method, path string) bool {
	row := routeFor(method, path)
	if row == nil {
		return who == callerPage || who == callerTool
	}
	for _, name := range row.callers {
		if name == who.name() {
			return true
		}
	}
	return false
}

// routeRefusal is what a refused caller is told.
func routeRefusal(method, path string) string {
	if row := routeFor(method, path); row != nil {
		for _, name := range row.callers {
			if name == "gm" {
				return "Only the setup page or the GM can do this."
			}
		}
	}
	return "This is for the setup page the menu opens."
}

// fixAllowed is whether a caller may run a fix. A tool (smolcorp setup) may
// repair, with auto=1, which leaves agents you disconnected alone; install this
// copy (it copies the program that is running, nothing it's handed); and open
// the firewall for how Smolcorp already listens, which the system asks you to
// OK. Listening on the Wi-Fi, opening at login and Tailscale are the page's.
func fixAllowed(who caller, fix, auto string) bool {
	return who != callerTool || fix == fixRepair && auto == "1" || fix == fixInstall || fix == fixFirewall
}

// webhookSecretFor is the webhook's secret for a caller: the setup page's alone,
// since with it anyone could sign as the owner.
func webhookSecretFor(who caller, secret string) string {
	if who == callerPage {
		return secret
	}
	return ""
}
