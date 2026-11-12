//go:build !windows

package desktop

import (
	"slices"
	"testing"
)

// The team grid's tmux calls: one session, a pane to a hire, each a read-only attach to that hire's own server.
func TestTheTeamGridIsAReadOnlyPaneToAHire(t *testing.T) {
	calls := teamGridCommands("/tmp/tmux-501/smolcorp-team", []string{"one", "two"})
	if len(calls) != 5 || !slices.Equal(calls[0], []string{"-S", "/tmp/tmux-501/smolcorp-team", "new-session", "-d", "-s", "team", "-x", "240", "-y", "60", "one"}) ||
		!slices.Equal(calls[2], []string{"-S", "/tmp/tmux-501/smolcorp-team", "split-window", "-t", "=team:", "two"}) {
		t.Errorf("calls %q", calls)
	}
	if pane := teamGridPane("/opt/tmux", "/tmp/tmux-501/smolcorp-a b"); pane != `exec '/opt/tmux' -S '/tmp/tmux-501/smolcorp-a b' attach -r -t '=hire'` {
		t.Errorf("pane %q", pane)
	}
	if teamGridSocket(7771) != hireSocketFolder()+"/smolcorp-7771-team" {
		t.Errorf("a test copy's grid: %q", teamGridSocket(7771))
	}
}
