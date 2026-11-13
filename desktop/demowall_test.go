//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
	// RWIN4 N1: ~user is that user's home to bash, not a folder from cwd. N2: a folder named like a brief file
	// is a place to cd from, to ../../../registry.json.
	briefFolder := filepath.Join(testerFolder, "d.md")
	if err := os.Mkdir(briefFolder, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, who := range []struct {
		fence demoFence
		cwd   string
	}{{asGM, f.repo}, {asTester, tester.Worktree}} {
		refused = append(refused,
			row{who.fence, bashCall("cat ~nobody/registry.json", who.cwd)},
			row{who.fence, bashCall("cat ~root/Library/Application\\ Support/AgentDeck/registry.json", who.cwd)},
			row{who.fence, read(who.cwd, "~nobody/registry.json")},
			row{who.fence, write(who.cwd, "~nobody/STATUS.md")},
			row{who.fence, bashCall("mkdir "+shellQuoted(filepath.Join(testerFolder, "e.md")), who.cwd)},
			row{who.fence, bashCall("cd "+shellQuoted(briefFolder)+" && cat ../../../registry.json", who.cwd)},
			row{who.fence, bashCall("ls "+shellQuoted(briefFolder), who.cwd)})
	}
	// RWIN5 B1: a path the shell builds with $ or a backtick is refused, whatever it fills in: $HOME is the fence's
	// home, and the app's folder is below it. B3: a name the Mac's disk reads with ſ (the long s) as s, which
	// strings.ToLower didn't: the setup key, Claude Code's settings (the demo's own fence), smolcorp itself.
	fromHome, err := filepath.Rel(homeDir, supportDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, who := range []struct {
		fence demoFence
		cwd   string
	}{{asGM, f.repo}, {asTester, tester.Worktree}} {
		fromCwd, err := filepath.Rel(who.cwd, supportDir)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range []string{
			`cat "$HOME/` + fromHome + `/registry.json"`, `cat "${HOME}/` + fromHome + `/config.json"`,
			`cat "$TMPDIR/` + filepath.Base(supportDir) + `/registry.json"`, `cat "$PWD/` + fromCwd + `/registry.json"`,
			`grep -r . "$HOME/` + fromHome + `"`, `cp "$HOME/` + fromHome + `/registry.json" leak.txt`,
			`echo x > "$HOME/` + fromHome + `/config.json"`, `cat "$(pwd)/` + fromCwd + `/registry.json"`,
			"cat `pwd`/" + strings.ReplaceAll(fromCwd, " ", `\ `) + "/registry.json", "cat $OLDPWD/registry.json",
			`cat $'\x2f'` + shellQuoted(strings.TrimPrefix(supportDir, "/")) + `/registry.json`,
			"echo '{}' > .claude/\u017fettings.json", "rm .claude/\u017fettings.json", "cp x.json .claude/\u017fettings.local.json",
			"cat " + shellQuoted(filepath.Join(supportDir, "\u017fetup-key")), "cat \u017fetup-key", "\u017fmolcorp mcp",
			"/usr/local/bin/\u017fmolcorp mcp < m.txt",
		} {
			refused = append(refused, row{who.fence, bashCall(line, who.cwd)})
		}
		// The demo's own lines with a $ still run: one with no path in it, and a $ in single quotes, which the
		// shell leaves as it is.
		for _, line := range []string{"echo $?", "git diff main...hire/tester-1 > /dev/null; echo $?", "grep -n '$HOME/x' notes.md"} {
			if why := who.fence.check(bashCall(line, who.cwd)); why != "" {
				t.Errorf("refused %q: %s", line, why)
			}
		}
	}
	// RWIN6 B1: a redirection glued to its target (>file, 2>file, <file) reads or writes that file: its target is a
	// path, judged as any other. B2–B4: a line not written plainly is refused by the guard the fence asks first:
	// ß, ﬂ and ﬁ in a program's name, a NUL in $'…', zsh's (a|b) and =cmd (the demo's wall too). And a tool's
	// path not written plainly. Its own STATUS.md, written with a glued >>, is still its own.
	for _, who := range []struct {
		fence demoFence
		cwd   string
	}{{asGM, f.repo}, {asTester, tester.Worktree}} {
		registry, config := filepath.Join(supportDir, "registry.json"), filepath.Join(supportDir, "config.json")
		relRegistry, err := filepath.Rel(who.cwd, registry)
		if err != nil {
			t.Fatal(err)
		}
		relConfig, _ := filepath.Rel(who.cwd, config)
		for _, line := range []string{
			"cat <" + shellQuoted(registry), "cat 0<" + shellQuoted(registry), "cat <" + tilde(registry), "cat <" + shellQuoted(relRegistry),
			"cat<" + shellQuoted(registry), "cat <>" + shellQuoted(registry), "echo '{}' >" + shellQuoted(config), "echo '{}' >>" + shellQuoted(config),
			"ls 2>" + shellQuoted(config), "ls &>" + shellQuoted(config), "echo x>" + shellQuoted(config), "echo x >" + shellQuoted(relConfig),
			"cat x >" + tilde(config), "echo x 1>" + tilde(config),
			"\u00dfh host", "\ufb02y deploy", "\ufb01rebase deploy", "cat \u00dfetup-key", `cat $'setup-key\0x'`, "cat setup-(key|x)",
			"=git push origin main", "=git -C " + shellQuoted(filepath.Join(homeDir, "elsewhere")) + " status",
			"=git worktree add " + shellQuoted(filepath.Join(homeDir, "elsewhere")), "=ssh host",
			// RWIN7 B1: a bracket Go's matcher read otherwise ([!x], POSIX's classes): the demo's own fence, the setup
			// key, and a part of the app's folder's path.
			"rm .claude/s[!x]ttings.json", "echo '{}' > .claude/s[!x]ttings.json", "rm -rf .c[!x]aude", "rm -rf .c[[:alpha:]]aude",
			"cat " + shellQuoted(supportDir) + "/setup-k[!x]y", "cat " + shellQuoted(supportDir) + "/setup-k[[:alpha:]]y",
			"cat " + shellQuoted(filepath.Dir(supportDir)) + "/[!x]" + shellQuoted(filepath.Base(supportDir)[1:]) + "/registry.json",
			"cat " + shellQuoted(filepath.Dir(supportDir)) + "/[[:alnum:]]" + shellQuoted(filepath.Base(supportDir)[1:]) + "/registry.json",
			// B2: zsh's precommand modifiers, repeat, emulate -c and short forms (the wall's rows too).
			"noglob git push origin main", "nocorrect git push origin main", "repeat 1 git push origin main",
			"if [[ -n x ]] git push origin main", "noglob ssh host", "true; - git push origin main",
			"nocorrect git -C " + shellQuoted(filepath.Join(homeDir, "elsewhere")) + " status",
			"noglob git worktree add " + shellQuoted(filepath.Join(homeDir, "elsewhere")), "emulate zsh -c 'git push origin main'",
			// B3: zsh's ${(e)…}, bash's @P.
			"echo ${(e):-'$(git push)'}", "echo ${(e):-'$(cat setup-key)'}", `x='$(git push)'; echo "${x@P}"`,
			// WIN125j (RWIN8 B1–B4): a [ inside a bracket and zsh's [!], zsh's x^y, a pattern earlier in the path, and
			// text the shell runs without eval; and a run that takes a / to the app's folder.
			"cat " + shellQuoted(supportDir) + "/setup-k[!x-[:x:]y", "cat " + shellQuoted(supportDir) + "/setup-k[!]y",
			"rm .claude/s[!x-[:x:]ttings.json", "rm -rf .c[!x-[:x:]aude", "rm -rf .c[!]aude", "rm -rf .c^1", "rm -rf .c^",
			"setopt extendedglob; mv .c^1 x", "rm .c?aude/settings.json", "echo '{}' > .c?aude/settings.json", "rm .cl*/settings.json",
			"PS4='$(git push origin main)'; set -x; :", "zargs -- origin -- git push", "eval -- 'git push origin main'",
			"setopt promptsubst; print -P '$(git push)'", "functions[f]='git push origin main'; f",
			"cat " + shellQuoted(filepath.Dir(filepath.Dir(supportDir))) + "/" + shellQuoted(filepath.Base(filepath.Dir(supportDir))[:1]) + "*/registry.json",
		} {
			refused = append(refused, row{who.fence, bashCall(line, who.cwd)})
		}
		// WIN125h: a path the disk may read as another name, or in another script, is refused: ß, ﬁ, a fullwidth Ｆ, a
		// Cyrillic а, and an é typed as e and an accent.
		refused = append(refused, row{who.fence, read(who.cwd, filepath.Join(who.cwd, "\u00dfetup.md"))},
			row{who.fence, write(who.cwd, filepath.Join(who.cwd, "\ufb01le.md"))},
			row{who.fence, read(who.cwd, filepath.Join(who.cwd, "\uff26ile.md"))},
			row{who.fence, write(who.cwd, filepath.Join(who.cwd, "d\u0430ta.md"))},
			row{who.fence, read(who.cwd, filepath.Join(who.cwd, "re\u0301sume\u0301.md"))},
			row{who.fence, tool(who.cwd, "Glob", map[string]any{"pattern": "\ufb01*"})},
			row{who.fence, bashCall("cat \ufb01le.md", who.cwd)}, row{who.fence, bashCall("cat \uff26ile.md", who.cwd)},
			row{who.fence, bashCall("cat docs/pr\u0435sentation.md", who.cwd)})
	}
	for _, line := range []string{"echo 'Done.' >>" + shellQuoted(filepath.Join(testerFolder, "STATUS.md")), "ls 2>/dev/null", "git diff HEAD~1 -- bill.py",
		"if [[ -f bill.py ]]; then cat bill.py; fi", "ls [!.]*", "grep -n repeat bill.py"} {
		if why := asTester.check(bashCall(line, tester.Worktree)); why != "" {
			t.Errorf("refused %q: %s", line, why)
		}
	}
	// WIN125h: an accented path in the demo's folder is the project's own (the owner's clients are French): read,
	// written, listed and moved, by a tool and in a line, for the GM and the tester.
	for _, who := range []struct {
		fence demoFence
		cwd   string
	}{{asGM, f.repo}, {asTester, tester.Worktree}} {
		accented := []demoCall{
			read(who.cwd, filepath.Join(who.cwd, "docs", "pr\u00e9sentation.md")),
			write(who.cwd, filepath.Join(who.cwd, "\u00c9quipe", "r\u00e9sum\u00e9.md")),
			tool(who.cwd, "Glob", map[string]any{"pattern": "\u00c9quipe/*.md"}),
			tool(who.cwd, "Grep", map[string]any{"pattern": "fa\u00e7ade", "path": filepath.Join(who.cwd, "docs", "pr\u00e9sentation.md")}),
		}
		for _, line := range []string{
			"cat docs/pr\u00e9sentation.md", `cd "Wonder/\u00c9quipe"`, "ls \u00c9quipe/", "git add docs/pr\u00e9sentation.md",
			"mv pr\u00e9sentation.md pr\u00e9sentation-v2.md", "cat " + shellQuoted(filepath.Join(who.cwd, "docs", "pr\u00e9sentation.md")),
			`git commit -m "Ajoute la pr\u00e9sentation de l'\u00e9quipe"`,
		} {
			accented = append(accented, bashCall(line, who.cwd))
		}
		for _, call := range accented {
			if why := who.fence.check(call); why != "" {
				t.Errorf("refused %s %s: %s", call.ToolName, call.ToolInput, why)
			}
		}
	}
	// RWIN4 B2: the support folder written in another case is the same folder on the Mac's disk (and Windows'):
	// every rule holds for it. The last part upper-cased, the whole path, through ~ and .., and a link to it.
	if runtime.GOOS == "darwin" {
		other := func(path string) string {
			rest, _ := filepath.Rel(supportDir, path)
			return filepath.Join(filepath.Dir(supportDir), strings.ToUpper(filepath.Base(supportDir)), rest)
		}
		upper := other(supportDir)
		if _, err := os.Stat(upper); err != nil {
			t.Fatalf("this Mac's disk reads names by case: %v", err)
		}
		fromHome, err := filepath.Rel(homeDir, upper)
		if err != nil {
			t.Fatal(err)
		}
		linkUpper := filepath.Join(tester.Worktree, "upper.md")
		if err := os.Symlink(filepath.Join(upper, "registry.json"), linkUpper); err != nil {
			t.Fatal(err)
		}
		// Its own brief, in another case, is still its own.
		if why := asTester.check(read(tester.Worktree, other(tester.Brief))); why != "" {
			t.Errorf("refused its brief in another case: %s", why)
		}
		for _, who := range []struct {
			fence demoFence
			cwd   string
		}{{asGM, f.repo}, {asTester, tester.Worktree}} {
			for _, file := range []string{
				filepath.Join(upper, "gm-token"), filepath.Join(upper, "registry.json"), filepath.Join(upper, "config.json"),
				filepath.Join(upper, "setup-key"), strings.ToUpper(filepath.Join(supportDir, "registry.json")),
				strings.ToLower(filepath.Join(supportDir, "registry.json")), other(filepath.Join(testerFolder, "settings.json")),
				filepath.Join(other(testerFolder), "..", "..", "registry.json"), linkUpper, upper,
			} {
				refused = append(refused,
					row{who.fence, read(who.cwd, file)},
					row{who.fence, bashCall("cat "+shellQuoted(file), who.cwd)},
					row{who.fence, bashCall("cp x "+shellQuoted(file), who.cwd)},
					row{who.fence, write(who.cwd, file)})
			}
			refused = append(refused,
				row{who.fence, bashCall("cat ~/"+filepath.ToSlash(fromHome)+"/registry.json", who.cwd)},
				row{who.fence, read(who.cwd, "~/"+filepath.ToSlash(fromHome)+"/gm-token")},
				row{who.fence, tool(who.cwd, "LS", map[string]any{"path": upper})},
				row{who.fence, tool(who.cwd, "Grep", map[string]any{"path": upper, "pattern": ".", "output_mode": "content"})},
				row{who.fence, tool(who.cwd, "Glob", map[string]any{"path": upper, "pattern": "*"})},
				row{who.fence, tool(who.cwd, "Grep", map[string]any{"path": strings.ToUpper(filepath.Dir(supportDir)), "pattern": "."})},
				row{who.fence, tool(who.cwd, "Glob", map[string]any{"path": strings.ToUpper(filepath.Dir(supportDir)), "pattern": "*/gm-t*"})},
				row{who.fence, bashCall("cat "+shellQuoted(upper)+"/*.json", who.cwd)})
			// The Mac's other name for the data volume's folders: the same folder to the disk, whatever its spelling.
			if data := filepath.Join("/System/Volumes/Data", support); fileExists(data) {
				refused = append(refused,
					row{who.fence, read(who.cwd, filepath.Join(data, "registry.json"))},
					row{who.fence, bashCall("cat "+shellQuoted(filepath.Join(data, "gm-token")), who.cwd)},
					row{who.fence, tool(who.cwd, "Grep", map[string]any{"path": data, "pattern": "."})},
					row{who.fence, tool(who.cwd, "Grep", map[string]any{"path": filepath.Dir(data), "pattern": "."})})
			}
		}
	}
	for _, r := range refused {
		if why := r.fence.check(r.call); !strings.HasPrefix(why, demoFenceRefusal) {
			t.Errorf("let through %s %s", r.call.ToolName, r.call.ToolInput)
		}
	}
}
