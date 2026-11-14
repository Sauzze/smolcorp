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
	{"POST", "/setup/hiring/choose", []string{"page"}},
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
	{"POST", "/setup/officekey", []string{"page"}},
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

// goOnlyRoutes are setup routes only this app serves (INVENTORY.md's "Served by the Go app only"), so they aren't
// in the Mac's fixture: Take over in T3 Code and Hand back (t3takeover.go), the page alone; a hire's git state (gitstate.go), a read;
// Phone over HTTPS (phonehttps.go), all of it the page's alone, the tap that runs `tailscale serve` included; the
// night shift's jobs (nightshift.go): listed by the page, the GM and a tool, proposed by the GM (or the page), made,
// changed, approved, paused and deleted by the page alone. An MCP key and a night job are refused them all (gates.go);
// and a desk's preview address declared (preview.go): the owner from Setup, the GM for a hire, a hire for its own desk.
// Not under /setup/gm/, so the owner's works with the manager off.
// The sign-in status (signinstatus.go) is the page's alone too.
// Signing in to an agent (signinrun.go) is the page's alone too.
// Floors from other Macs (peers_mac_join.go) are the page's alone too.
var goOnlyRoutes = []routeRow{
	{"GET", "/setup/tailnet/status", []string{"page"}},
	{"GET", "/setup/tailnet/serve", []string{"page"}},
	{"POST", "/setup/tailnet/serve", []string{"page"}},
	{"POST", "/setup/t3code/takeover", []string{"page"}},
	{"POST", "/setup/t3code/handback", []string{"page"}},
	{"GET", "/setup/gm/git", []string{"gm", "page", "tool"}},
	// Typing over Wi-Fi (SECWIFI): the owner's page alone, never an agent.
	{"POST", "/setup/wifityping", []string{"page"}},
	// The hire queue and its plan (hirequeue.go, hireplan.go): the GM proposes a plan, and only the page asks for
	// one, approves it, resumes the queue or cancels it. A tool, an MCP key and a hire do none of these.
	{"GET", "/setup/hire/queue", []string{"gm", "page", "tool"}},
	{"POST", "/setup/hire/plan", []string{"page"}},
	{"POST", "/setup/hire/plan/propose", []string{"gm", "page"}},
	{"POST", "/setup/hire/plan/approve", []string{"page"}},
	{"POST", "/setup/hire/plan/cancel", []string{"page"}},
	{"POST", "/setup/hire/queue/resume", []string{"page"}},
	{"POST", "/setup/hire/queue/cancel", []string{"page"}},
	// Whether each coding agent is signed in (signinstatus.go): the page alone, never the GM, a hire, a tool or a phone.
	{"GET", "/setup/accounts/status", []string{"page"}},
	// Signing in to a coding agent (signinrun.go): the page alone, never the GM, a hire, a tool or a phone.
	{"POST", "/setup/accounts/login", []string{"page"}},
	{"GET", "/setup/accounts/login", []string{"page"}},
	{"POST", "/setup/accounts/login/code", []string{"page"}},
	{"POST", "/setup/accounts/login/cancel", []string{"page"}},
	// Floors from other Macs (peers_mac_join.go): the page alone, as pairing a phone is.
	{"GET", "/setup/peers", []string{"page"}},
	{"POST", "/setup/peers", []string{"page"}},
	{"POST", "/setup/peers/code", []string{"page"}},
	{"POST", "/setup/peers/revoke", []string{"page"}},
	{"POST", "/setup/peers/join", []string{"page"}},
	// The owner's yes or no to an ask (grants.go): the page's cookie alone, never a tool's key nor the GM's token.
	{"POST", "/setup/grants/answer", []string{"page"}},
	// Smolcorp's records started over, once their key failed its check (records.go): the page alone.
	{"POST", "/setup/records/reset", []string{"page"}},
	// The owner's answer to a terminal's ask for a key (terminalask.go): the page alone, under the Mac app its own window.
	{"POST", "/setup/terminal/answer", []string{"page"}},
	// "I've seen it" for a Setup link used first (setuplink.go): the page alone, and only the Mac's own window's.
	{"POST", "/setup/used-first/seen", []string{"page"}},
	{"GET", nightListPath, []string{"gm", "page", "tool"}},
	{"POST", nightProposePath, []string{"gm", "page"}},
	{"POST", nightSavePath, []string{"page"}},
	{"POST", nightApprovePath, []string{"page"}},
	{"POST", nightPausePath, []string{"page"}},
	{"POST", nightDeletePath, []string{"page"}},
	{"POST", "/setup/preview", []string{"gm", "page", "hire"}},
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
	for i := range goOnlyRoutes {
		if row := &goOnlyRoutes[i]; fits(row) && row.path == path {
			return row
		}
	}
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
// keeps 1.23's rule: the page, or a tool (a tool's key you allowed).
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
// Under the Mac app (mac), as the Mac's own (DeckRouter.fixRefusal): a tool
// only repairs, with auto=1, as the Mac app installs and opens itself.
func fixAllowed(who caller, fix, auto string, mac bool) bool {
	if who != callerTool || fix == fixRepair && auto == "1" {
		return true
	}
	return !mac && (fix == fixInstall || fix == fixFirewall)
}

// webhookSecretFor is the webhook's secret for a caller: the setup page's alone,
// since with it anyone could sign as the owner.
func webhookSecretFor(who caller, secret string) string {
	if who == callerPage {
		return secret
	}
	return ""
}
