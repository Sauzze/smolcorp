//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	// A literal null body is an empty request: refused or answered, never a panic.
	if code, _ := scopedCall(t, d, token, "POST", "/setup/gm/hire", "null"); code == http.StatusOK || code >= 500 {
		t.Errorf("a null body: %d", code)
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

// WIN125d (RWIN3 B1): on a real deck, the demo's GM reads its brief (kept in the app's briefs folder) and a hire
// it made reads its own (in hires/<id>), each by Read and by cat, and writes its STATUS.md beside it, with the
// fence built as runDemoFence builds it. The rest of the app's folder stays refused by every route: the setup
// key, the GM's token, the app's state, a hire's settings, another project's hire, a pattern or a link into it.
func TestTheDemosFenceLetsItsHiresReadTheirBriefs(t *testing.T) {
	d, f := demoPageDeck(t)
	token := hireTheDemosGM(t, d, f)
	body, _ := json.Marshal(map[string]any{"role": "tester", "folder": f.repo})
	if code, text := scopedCall(t, d, token, "POST", "/setup/gm/hire", string(body)); code != http.StatusOK {
		t.Fatalf("the GM's hire: %d %q", code, text)
	}
	support := resolveExisting(supportDir)
	gm, tester := gmNow(), Hire{}
	for _, h := range registered(t) {
		if h.Role == "tester" {
			tester = h
		}
	}
	if !strings.HasPrefix(gm.Brief, filepath.Join(supportDir, "briefs")+string(filepath.Separator)) ||
		tester.Worktree == "" || tester.Brief != filepath.Join(supportDir, "hires", tester.ID, "BRIEF.md") {
		t.Fatalf("not the briefs RWIN3 probed: the GM's %q, the tester's %q in %q", gm.Brief, tester.Brief, tester.Worktree)
	}
	for _, brief := range []string{gm.Brief, tester.Brief} {
		if _, err := os.Stat(brief); err != nil {
			t.Fatal(err)
		}
	}
	gmStatus := strings.TrimSuffix(gm.Brief, ".md") + "-STATUS.md"
	testerFolder := filepath.Dir(tester.Brief)
	// Another project's hire, its brief in the app's folder too.
	other := Hire{ID: "outsider-1", Name: "Outsider", Role: "tester", Folder: filepath.Join(homeDir, "elsewhere"), Brief: filepath.Join(supportDir, "hires", "outsider-1", "BRIEF.md"), State: "working"}
	if err := updateRegistry(func(r *registryFile) { r.hires = append(r.hires, other) }); err != nil {
		t.Fatal(err)
	}
	// Links into the app's folder: one in the hire's worktree, one beside its brief, named as a brief's file.
	os.WriteFile(filepath.Join(supportDir, "gm-token"), []byte("x"), 0o600)
	t.Cleanup(func() { os.Remove(filepath.Join(supportDir, "gm-token")) })
	linkInWorktree, linkBeside := filepath.Join(tester.Worktree, "notes.md"), filepath.Join(testerFolder, "notes.md")
	for _, link := range []string{linkInWorktree, linkBeside} {
		if err := os.Symlink(filepath.Join(supportDir, "gm-token"), link); err != nil {
			t.Fatal(err)
		}
	}

	fence := demoFenceHere()
	fence.root, fence.roles, fence.team = f.root, []string{"reviewer", "tester"}, nil
	asGM, asTester := fence, fence
	asTester.hireSession = true
	tool := func(cwd, name string, input map[string]any) demoCall {
		data, _ := json.Marshal(input)
		return demoCall{ToolName: name, ToolInput: data, Cwd: cwd}
	}
	read := func(cwd, file string) demoCall { return tool(cwd, "Read", map[string]any{"file_path": file}) }
	write := func(cwd, file string) demoCall {
		return tool(cwd, "Write", map[string]any{"file_path": file, "content": "Done: the lost cent is fixed."})
	}
	type row struct {
		fence demoFence
		call  demoCall
	}
	allowed := []row{
		{asGM, read(f.repo, gm.Brief)},
		{asGM, read(f.repo, tildePath(gm.Brief))}, // as its first message names it
		{asGM, bashCall("cat "+shellQuoted(gm.Brief), f.repo)},
		{asGM, write(f.repo, gmStatus)},
		{asGM, read(f.repo, gmStatus)},
		{asGM, read(f.repo, filepath.Join(testerFolder, "STATUS.md"))}, // its hire's status
		{asTester, read(tester.Worktree, tester.Brief)},
		{asTester, read(tester.Worktree, tildePath(tester.Brief))},
		{asTester, bashCall("cat "+shellQuoted(tester.Brief), tester.Worktree)},
		{asTester, write(tester.Worktree, filepath.Join(testerFolder, "STATUS.md"))},
		{asTester, tool(tester.Worktree, "Edit", map[string]any{"file_path": filepath.Join(testerFolder, "STATUS.md"), "old_string": "a", "new_string": "b"})},
		{asTester, bashCall("tail -5 "+shellQuoted(filepath.Join(testerFolder, "STATUS.md")), tester.Worktree)},
		// RWIN3 N1: a * in the project is the project's files, not the setup key.
		{asTester, tool(tester.Worktree, "Glob", map[string]any{"pattern": "*"})},
		{asGM, tool(f.repo, "Glob", map[string]any{"pattern": "bill/*"})},
		{asGM, tool(f.repo, "Grep", map[string]any{"pattern": "set*"})},
		{asGM, tool(f.repo, "Grep", map[string]any{"pattern": ".", "glob": "setup-*"})},
	}
	for _, r := range allowed {
		if why := r.fence.check(r.call); why != "" {
			t.Errorf("refused %s %s: %s", r.call.ToolName, r.call.ToolInput, why)
		}
	}
	tilde := func(path string) string { return strings.ReplaceAll(tildePath(path), " ", `\ `) }
	var refused []row
	for _, who := range []struct {
		fence demoFence
		cwd   string
	}{{asGM, f.repo}, {asTester, tester.Worktree}} {
		for _, file := range []string{
			filepath.Join(supportDir, "setup-key"), filepath.Join(supportDir, "gm-token"), filepath.Join(supportDir, "gm-token-kept"),
			filepath.Join(supportDir, "registry.json"), filepath.Join(supportDir, "config.json"), filepath.Join(supportDir, "hook-key"),
			filepath.Join(testerFolder, "settings.json"), filepath.Join(testerFolder, "..", "..", "gm-token"), other.Brief,
			filepath.Join(supportDir, "briefs", "templates", "tester.md"), filepath.Join(supportDir, "notes.md"),
			linkInWorktree, linkBeside, supportDir,
		} {
			refused = append(refused,
				row{who.fence, read(who.cwd, file)},
				row{who.fence, bashCall("cat "+shellQuoted(file), who.cwd)},
				row{who.fence, bashCall("cat "+tilde(file), who.cwd)},
				row{who.fence, write(who.cwd, file)})
		}
		refused = append(refused,
			// A brief is the GM's to write, and the other files there aren't a hire's.
			row{who.fence, write(who.cwd, gm.Brief)}, row{who.fence, write(who.cwd, tester.Brief)},
			row{who.fence, write(who.cwd, filepath.Join(filepath.Dir(other.Brief), "STATUS.md"))},
			row{who.fence, write(who.cwd, filepath.Join(supportDir, "hires", "STATUS.md"))},
			// The folder by LS, Grep and Glob; and patterns that fill in a name in it (RWIN3 N7).
			row{who.fence, tool(who.cwd, "LS", map[string]any{"path": supportDir})},
			row{who.fence, tool(who.cwd, "Grep", map[string]any{"path": supportDir, "pattern": "."})},
			row{who.fence, tool(who.cwd, "Grep", map[string]any{"path": filepath.Join(supportDir, "briefs"), "pattern": "."})},
			row{who.fence, tool(who.cwd, "Glob", map[string]any{"path": supportDir, "pattern": "*"})},
			row{who.fence, tool(who.cwd, "Glob", map[string]any{"path": filepath.Join(supportDir, "briefs"), "pattern": "../s*"})},
			row{who.fence, tool(who.cwd, "Glob", map[string]any{"pattern": filepath.Join(supportDir, "*")})},
			row{who.fence, tool(who.cwd, "Glob", map[string]any{"path": filepath.Dir(supportDir), "pattern": "**/s*"})},
			row{who.fence, tool(who.cwd, "Glob", map[string]any{"path": filepath.Dir(supportDir), "pattern": "*/gm-t*"})},
			row{who.fence, tool(who.cwd, "Grep", map[string]any{"path": filepath.Dir(supportDir), "pattern": "."})},
			row{who.fence, tool(who.cwd, "Grep", map[string]any{"path": filepath.Dir(supportDir), "pattern": ".", "glob": "gm-*"})},
			row{who.fence, bashCall("cat "+filepath.Dir(support)+"/"+filepath.Base(support)[:2]+"*/s*", who.cwd)},
			row{who.fence, bashCall("cat "+filepath.Dir(support)+"/*/g?-token", who.cwd)},
			row{who.fence, bashCall("cat "+filepath.Dir(support)+"/{x,"+filepath.Base(support)+"}/gm-token-kept", who.cwd)},
			row{who.fence, bashCall("cat "+filepath.Join(testerFolder, "..", "..")+"/s*", who.cwd)},
			row{who.fence, bashCall("cat "+filepath.Join(support, "hires")+"/*/settings.json", who.cwd)},
			row{who.fence, bashCall("cat "+filepath.Join(support, "briefs")+"/*.md", who.cwd)},
			row{who.fence, bashCall("cat "+support+"/**/BRIEF.md", who.cwd)})
	}
	for _, r := range refused {
		if why := r.fence.check(r.call); !strings.HasPrefix(why, demoFenceRefusal) {
			t.Errorf("let through %s %s", r.call.ToolName, r.call.ToolInput)
		}
	}
}
