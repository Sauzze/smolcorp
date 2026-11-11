package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The fence of a hire in T3 Code is its worktree's .claude/settings.local.json
// (T797b). Tests/fixtures/frontdesk/fence-commands.json lists what must be
// refused and what must still run; the Mac's GuardFileTests checks it names
// the very file T3Desk writes, and runs the real guard on it. Every hire is
// held to it, the GM too.
func TestNoHireTakesAwayOrRewritesClaudeCodesSettings(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "Tests", "fixtures", "frontdesk", "fence-commands.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Fence   string   `json:"fence"`
		Refused []string `json:"refused"`
		Allowed []string `json:"allowed"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if f.Fence != ".claude/settings.local.json" || len(f.Refused) == 0 || len(f.Allowed) == 0 {
		t.Fatalf("the fixture: %+v", f)
	}
	mustRefuse(t, "fence", f.Refused)
	mustRun(t, "fence look-alikes", f.Allowed)
	for _, line := range f.Refused {
		if Check(line, ForGM(DefaultGroups())) == nil {
			t.Errorf("the GM: let through: %s", line)
		}
	}
	for line, rule := range map[string]string{
		"rm .claude/settings.local.json": fenceRule, "echo '{}' > .claude/settings.local.json": fenceRule,
		"git clean -fdx": "git clean -x", "git stash push -a": "git stash --all",
	} {
		if r := Check(line, DefaultGroups()); r == nil || r.Group != "guard" || r.Rule != rule {
			t.Errorf("%q: %+v, want guard: %s", line, r, rule)
		}
	}
}
