package desktop

import (
	"slices"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// A desk's profile opens the local databases only when the opt-in passed SA-M12's check at that desk's start
// (SA-X14 wires it in): every other desk, and one whose check found a login that gets in, keeps them denied.
func TestADesksProfileOpensTheDatabasesOnlyWhenItsStartsCheckPassed(t *testing.T) {
	freshChecks(t)
	pl := sandbox.Places{OwnID: "db0000000001", Home: homeDir}
	if ports := deskServices(pl, "/s").DeniedPorts; !slices.Contains(ports, 5432) || !slices.Contains(ports, 3306) {
		t.Errorf("no check, the databases open: %v", ports)
	}
	deskStartChecks(pl.OwnID, true) // no database place to try: it passes
	if ports := deskServices(pl, "/s").DeniedPorts; slices.Contains(ports, 5432) || slices.Contains(ports, 3306) || !slices.Contains(ports, 22) {
		t.Errorf("the check passed, still denied: %v", ports)
	}
	if other := deskServices(sandbox.Places{OwnID: "db0000000002", Home: homeDir}, "/s").DeniedPorts; !slices.Contains(other, 5432) {
		t.Errorf("another desk got the databases: %v", other)
	}
	deskChecksForgotten(pl.OwnID)
	if ports := deskServices(pl, "/s").DeniedPorts; !slices.Contains(ports, 5432) {
		t.Errorf("forgotten, still open: %v", ports)
	}
}
