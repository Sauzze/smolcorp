//go:build !windows

package desktop

import (
	"slices"
	"testing"
)

// The team grid's tmux calls: one session, a pane to a hire, each a read-only attach to that hire's own server.
func TestTheTeamGridIsAReadOnlyPaneToAHire(t *testing.T) {
	calls := teamGridCommands("/tmp/tmux-501/smolcorp-team", []string{"one", "two"})
	if len(calls) != 5 || !slices.Equal(calls[0], []string{"-S", "/tmp/tmux-501/smolcorp-team", "start-server", ";", "set-environment", "-gu", "SMOLCORP_TMUX_SERVER", ";",
		"new-session", "-d", "-P", "-F", "#{pid}", "-s", "team", "-x", "240", "-y", "60", "one"}) ||
		!slices.Equal(calls[2], []string{"-S", "/tmp/tmux-501/smolcorp-team", "split-window", "-t", "=team:", "two"}) {
		t.Errorf("calls %q", calls)
	}
	// A pane runs `smolcorp attach`, read-only, with exec (SA-M8); a test copy's names its port and folder.
	if pane := gridPaneLine("/App/smolcorp", "ab12", 0, "/s"); pane != `exec '/App/smolcorp' attach --read-only 'ab12'` {
		t.Errorf("pane %q", pane)
	}
	if pane := gridPaneLine("/App/smolcorp", "ab12", 7771, "/s up"); pane != `SMOLCORP_PORT='7771' SMOLCORP_TEST_SUPPORT_DIR='/s up' exec '/App/smolcorp' attach --read-only 'ab12'` {
		t.Errorf("a test copy's pane %q", pane)
	}
	if teamGridSocket(7771) != hireSocketFolder()+"/smolcorp-7771-team" {
		t.Errorf("a test copy's grid: %q", teamGridSocket(7771))
	}
}
