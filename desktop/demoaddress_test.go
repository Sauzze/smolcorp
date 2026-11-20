//go:build !windows

package desktop

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// `smolcorp demo` asks Setup where Setup is (FXDEMO126). Setup lives on 127.0.0.1: a GET at localhost is sent on
// there, and Go's client follows with a Referer of localhost, which Setup refuses as another page's ask. The demo's
// team, as the command makes it (newOfficeTeam), aimed at localhost, so on 1.26.0 (40) every demo stopped right after
// Allow at its first GET, GET /setup/state, with "Smolcorp said: Setup answers its own page alone…", on the Mac
// (whose app runs the same core) as on Windows and Linux. The row runs the command's own team against a real Deck on
// an ephemeral port, holding the key Allow gives it.
func TestTheDemoAsksSetupAtSetupsOwnAddress(t *testing.T) {
	d, _, _, _ := gmHireDeck(t)
	server := httptest.NewServer(&router{deck: d})
	t.Cleanup(server.Close)
	swap(t, &Port, server.Listener.Addr().(*net.TCPAddr).Port)
	allowed := func() ([]byte, error) { return []byte(d.access.SetupKey), nil }

	team := newOfficeTeam()
	team.key, team.http = allowed, &http.Client{}
	if team.base != setupBase() {
		t.Errorf("the demo asks Setup at %s, not at Setup's own address %s", team.base, setupBase())
	}
	var state demoState
	if err := team.call(http.MethodGet, "/setup/state", nil, &state); err != nil {
		t.Fatalf("the demo's first GET after Allow was refused: %v", err)
	}
	if !state.Manager.Enabled {
		t.Errorf("the demo read Setup's state without the manager on: %+v", state)
	}

	// The guard isn't widened for it: the same GET at localhost is still refused once it's sent on to 127.0.0.1.
	atLocalhost := &officeTeam{base: localBase(), key: allowed, http: &http.Client{}}
	var refused demoRefusal
	err := atLocalhost.call(http.MethodGet, "/setup/state", nil, &state)
	if !errors.As(err, &refused) || refused.status != http.StatusForbidden || !strings.Contains(refused.said, setupFromAnotherPage) {
		t.Errorf("a GET at localhost, sent on to 127.0.0.1 with a Referer of localhost, got %v, not Setup's 403", err)
	}
}
