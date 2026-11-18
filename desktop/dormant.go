package desktop

import (
	"net/http"
	"strings"
)

// What this version leaves off. Each feature here is built and tested, but nothing in this version uses it yet, so
// it's switched off as a whole and named nowhere an owner or a hire reads: its routes answer 404 with plain words
// (dormantRoute), it starts no timer, and Setup, the office, the phone, the GM's text and the docs don't mention it.
// The code stays for a later version, and its tests run with its switch on (dormant_test.go), so it keeps working.
// Each switch is a var only so those tests can turn it on: nothing else ever sets one.
var (
	// A spend command asks the owner instead of being refused (gates.go's decideShell, grants.go's answers). Off,
	// the guard and the gate service refuse it like any other guarded command, and no ask is ever kept.
	spendAsksOn = false
	// Night jobs (nightshift.go, nightschedule.go, nightstart.go): their routes and the scheduler.
	nightShiftOn = false
	// Company templates and the hire plan made from one (hireplan.go, roles.go's companyTemplates).
	templatesOn = false
	// Floors from other Macs (peers_mac*.go): another Mac's /peer/ routes and Setup's /setup/peers.
	peersOn = false
	// Sending a photo to a desk (photo.go): from the phone and from the office window.
	photoOn = false
)

// notInThisVersion is what a route of a feature that's off answers, with a 404, as T3 Code's off routes do.
const notInThisVersion = "This isn't in this version of Smolcorp."

// dormantRoute is whether a request's path belongs to a feature that's off. The hire queue runs both a template's
// plan and a night job's hire, so its routes are off only while both are.
func dormantRoute(path string) bool {
	under := func(prefix string) bool { return path == prefix || strings.HasPrefix(path, prefix+"/") }
	switch {
	case !spendAsksOn && (path == "/grants/answer" || path == "/setup/grants/answer"):
	case !nightShiftOn && isNightRoute(path):
	case !templatesOn && under("/setup/hire/plan"):
	case !templatesOn && !nightShiftOn && under("/setup/hire/queue"):
	case !peersOn && (under("/peer") || under("/setup/peers")):
	case !photoOn && (path == "/photo" || path == "/talk/photo"):
	default:
		return false
	}
	return true
}

// answerDormant answers a request for a feature that's off.
func answerDormant(w http.ResponseWriter) {
	writeStatus(w)(http.StatusNotFound, notInThisVersion)
}
