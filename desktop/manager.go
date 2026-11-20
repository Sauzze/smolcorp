package desktop

import (
	"net/http"
	"slices"
	"strings"
)

// features is what the GM's tools can count on in this build, for GET
// /setup/gm/version. The Mac app lists events, team, registry, token, mail,
// screens and tile; this build lists only what it serves, so a GM never relies
// on a missing feature. Each phase 6 task that adds a route adds its word here.
// registry waits until a GM caller can edit it (on the Mac it means the GM's edit).
var features = []string{"events", "team"}

// macFeatures are what this build serves besides, under the Mac app only: a new token (gmbyhand.go), and the
// screens and tiling (gmscreens.go), as the Mac's own words name them.
var macFeatures = []string{"token", "screens", "tile"}

// managerOffAnswer is what a /setup/gm route is told while the manager is off.
const managerOffAnswer = "The manager is off. Turn it on in Smolcorp's setup."

// managerStaysOpen is the routes that work with the manager off: the version (a
// tool asks it first) and the switch itself, as turning it on has to be
// possible; and POST /setup/gm/by-hand, as on the Mac, served under the Mac app (gmbyhand.go).
func managerStaysOpen(method, path string) bool {
	return method == http.MethodGet && path == "/setup/gm/version" || method == http.MethodPost && (path == "/setup/gm/manager" || path == "/setup/gm/by-hand")
}

// managerRefuses is whether a request is answered 409: any method on a /setup/gm/
// path while the manager is off, but for the routes that stay open. It runs
// after the route table's gate, so only an allowed caller sees it.
func managerRefuses(method, path string, enabled bool) bool {
	return !enabled && strings.HasPrefix(path, "/setup/gm/") && !managerStaysOpen(method, path)
}

// versionAnswer is GET /setup/gm/version, in the shape of the shared fixture
// Tests/fixtures/frontdesk/version.json.
func versionAnswer(version string, managerEnabled, underMac bool) map[string]any {
	served := features
	if underMac {
		served = append(slices.Clone(features), macFeatures...)
	}
	return map[string]any{"version": version, "managerEnabled": managerEnabled, "features": served}
}
