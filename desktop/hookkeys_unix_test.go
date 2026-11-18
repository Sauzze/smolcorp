//go:build !windows

package desktop

import (
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
)

// A managed hire's start ticket holds its own hook key, the registry its hash alone, the pane's command neither; its
// hook then reports from its desk with that key, and the owner's sessions' key no longer reaches that desk.
func TestAManagedDeskReportsWithTheHookKeyItsTicketHolds(t *testing.T) {
	d, r, hire := hiredForFire(t, "")
	plan, ok := issuedPlan(hire.ID)
	if !ok {
		t.Fatal("no ticket")
	}
	i := slices.IndexFunc(plan.Changes, func(c envChange) bool { return c.Name == hookKeyEnvironment })
	if i < 0 || plan.Changes[i].Value == "" {
		t.Fatalf("its changes: %+v", plan.Changes)
	}
	key := plan.Changes[i].Value
	if got := readRegistry(registryPath(), false).hookKeys[hire.ID]; got != hashKey(key) {
		t.Errorf("the registry keeps %q", got)
	}
	if data, _ := os.ReadFile(registryPath()); strings.Contains(string(data), key) {
		t.Error("the registry holds the key itself")
	}
	for _, asked := range r.tmux.asked {
		if strings.Contains(strings.Join(asked, " "), key) {
			t.Errorf("tmux was given the key: %q", asked)
		}
	}
	tab := tmuxTabOn(hire.TmuxSocket)
	if got := hookAs(t, d, d.access.HookKey, tab, "SessionStart", "s-desk", hire.Worktree); got != http.StatusForbidden {
		t.Errorf("the owner's key on the desk: %d", got)
	}
	if got := hookAs(t, d, key, tab, "SessionStart", "s-desk", hire.Worktree); got != http.StatusNoContent {
		t.Errorf("its own key: %d", got)
	}
	if got := teamSession(d, hire.ID); got != "s-desk" {
		t.Errorf("its row: %q", got)
	}
}
