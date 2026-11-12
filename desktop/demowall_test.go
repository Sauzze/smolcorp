//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// WIN125c: the wall behind the demo's fence. A demo's GM, through the real routes with its own token, is held to
// the demo's roles and to worktrees inside the demo folder, whatever its fence reads (RWIN2 B1, B2).
func TestTheDemosGMIsHeldToItsRolesAndItsFolder(t *testing.T) {
	d, f := demoPageDeck(t)
	token := hireTheDemosGM(t, d, f)
	if gm := gmNow(); gm.Scope != f.repo || gm.ScopeRoles != "reviewer,tester" {
		t.Fatalf("the demo's GM keeps %q and %q", gm.Scope, gm.ScopeRoles)
	}
	hire := func(role, folder string) (int, string) {
		body, _ := json.Marshal(map[string]any{"role": role, "folder": folder})
		return scopedCall(t, d, token, "POST", "/setup/gm/hire", string(body))
	}
	// B1: a role the office has but the demo doesn't offer (the fence read a comment's --role).
	if code, text := hire("janitor", f.repo); !scopeRefused(code, text) || text != "This General Manager hires only the demo's roles, reviewer, tester." {
		t.Errorf("another role: %d %q", code, text)
	}
	if code, text := hire("tester", f.repo); code != http.StatusOK {
		t.Fatalf("a demo's role into the project: %d %q", code, text)
	}
	// B2: a worktree of the project whose top is outside the demo folder, as `cd elsewhere && git --git-dir=…
	// worktree add wt` makes it. Its git folder is the project's; the wall holds it all the same.
	outside := filepath.Join(homeDir, "rwin2-nowhere")
	os.MkdirAll(outside, 0o755)
	t.Cleanup(func() { os.RemoveAll(outside) })
	wt := physical(outside) + "/wt"
	gitOut(t, outside, "--git-dir="+filepath.Join(f.repo, ".git"), "worktree", "add", "-q", "wt")
	if !(gmScope{root: f.repo, git: runGit}).holds(wt) {
		t.Fatal("the stand-in isn't a worktree of the project: the test proves nothing")
	}
	if gmScopeOf(gmNow()).holds(wt) {
		t.Error("the demo's scope holds a worktree outside the demo folder")
	}
	if code, text := hire("tester", wt); !scopeRefused(code, text) ||
		text != "This General Manager hires only into "+f.repo+" or the top of one of its worktrees in "+f.root+", a folder that is there." {
		t.Errorf("a worktree outside the demo folder: %d %q", code, text)
	}
	// Its own hire's worktree, in app-hires, is held.
	hires := registered(t)
	if last := hires[len(hires)-1]; last.Role != "tester" || !gmScopeOf(gmNow()).hasHire(last) {
		t.Errorf("the demo's own hire isn't held: %+v", last)
	}
	// A demo's GM whose row kept no roles hires no one: the shape alone makes it a demo's.
	if err := updateRegistry(func(r *registryFile) {
		for i := range r.hires {
			if r.hires[i].Role == gmRole {
				r.hires[i].ScopeRoles = ""
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	if code, text := hire("tester", f.repo); !scopeRefused(code, text) {
		t.Errorf("no roles kept: %d %q", code, text)
	}
}

// The owner's hire of a GM held to a demo's project keeps that demo's roles, and is refused for a demo the app
// wasn't told about: its roles would be anyone's.
func TestAGMHeldToADemoKeepsTheDemosRoles(t *testing.T) {
	_, f := demoPageDeck(t)
	roles := []Role{{ID: gmRole, Name: "General Manager", Agent: "claude"}, {ID: "tester", Name: "Tester", Agent: "claude"}}
	body, _ := json.Marshal(map[string]any{"role": gmRole, "folder": f.repo, "scope": f.repo})
	if hire, status, why := setupHireRequest(body, roles, nil, homeDir); status != 0 || hire.ScopeRoles != "reviewer,tester" {
		t.Errorf("a registered demo: %+v %d %q", hire, status, why)
	}
	demoPages.Lock()
	delete(demoPages.at, f.root)
	demoPages.Unlock()
	if _, status, why := setupHireRequest(body, roles, nil, homeDir); status != http.StatusConflict || why == "" {
		t.Errorf("a demo the app wasn't told about: %d %q", status, why)
	}
	// Another project keeps no roles: a GM held to it is as before.
	other := filepath.Join(homeDir, "proj-"+t.Name())
	os.MkdirAll(other, 0o755)
	t.Cleanup(func() { os.RemoveAll(other) })
	body, _ = json.Marshal(map[string]any{"role": gmRole, "folder": other, "scope": other})
	if hire, status, why := setupHireRequest(body, roles, nil, homeDir); status != 0 || hire.ScopeRoles != "" || gmScopeOf(Hire{Scope: hire.Scope}).demo != "" {
		t.Errorf("another project: %+v %d %q", hire, status, why)
	}
}
