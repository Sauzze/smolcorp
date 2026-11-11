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
	"time"
)

// A General Manager held to one project (gmscope.go), through the real routes with the GM's own token, on the
// engine tests' stand-ins: what every probe of RX1, RX1b and RX1c that reaches a route gets.

// scopedGM hires the GM from setup's own sheet, as the owner (the page), held to the engine's repository (moved
// into home), and gives back its token and that project.
func scopedGM(t *testing.T) (d *Deck, r *engineRig, h *hiring, token, project string) {
	t.Helper()
	return scopedGMOn(t, "", nil)
}

// scopedGMOn is scopedGM on a Claude account by name ("" for none named), first set up by prepare when given.
func scopedGMOn(t *testing.T, account string, prepare func(*Deck)) (d *Deck, r *engineRig, h *hiring, token, project string) {
	t.Helper()
	d, r, h, _ = gmHireDeck(t)
	if prepare != nil {
		prepare(d)
	}
	project = physical(r.repo) // as the app keeps it: links followed (on a Mac, /var is /private/var)
	ask := map[string]any{"role": gmRole, "folder": project, "scope": project}
	if account != "" {
		ask["account"] = account
	}
	body, _ := json.Marshal(ask)
	w := serve(t, d, call{method: "POST", target: "/setup/hire", host: "localhost", from: thisComputer, body: string(body),
		headers: map[string]string{setupHeader: "1"}})
	d.watching.Wait()
	if w.Code != http.StatusOK {
		t.Fatalf("the owner's hire of the GM: %d %s", w.Code, w.Body)
	}
	if hires := registered(t); len(hires) != 1 || hires[0].Role != gmRole || hires[0].Scope != project || hires[0].Folder != project {
		t.Fatalf("registry: %+v", hires)
	}
	d.mu.Lock()
	token = d.gmToken
	d.lastHire = time.Time{}
	d.mu.Unlock()
	if token == "" {
		t.Fatal("the GM has no token")
	}
	return d, r, h, token, project
}

// RX1d S2: a scoped GM's hires work on the Claude account it was hired on (the one the demo's card picked): named,
// or filled in by the app when left out; another account is refused. So a test copy's GM, on its test account,
// hires, where the default account would be refused (T721).
func TestAScopedGMHiresOnTheAccountItWasHiredOn(t *testing.T) {
	var work string
	withWork := func(d *Deck) {
		work = claudeFolder(t, filepath.Join(homeDir, "accounts", "work"), "settings.json", ".claude.json")
		d.config.ClaudeProfiles = []claudeProfile{{Name: "work", Dir: work}}
	}
	d, r, _, token, project := scopedGMOn(t, "work", withWork)
	if gm := registered(t)[0]; gm.Account != "work" {
		t.Fatalf("the GM's row: %+v", gm)
	}
	hire := func(fields map[string]any) (int, string) {
		fields["role"], fields["folder"] = "janitor", project
		body, _ := json.Marshal(fields)
		return scopedCall(t, d, token, "POST", "/setup/gm/hire", string(body))
	}
	for name, account := range map[string]any{"the default one": "default", "an account that isn't one": "home", "not a name": 5} {
		if code, text := hire(map[string]any{"account": account}); !scopeRefused(code, text) ||
			text != "This General Manager hires only on the Claude account it was hired on, work." {
			t.Errorf("%s: %d %q", name, code, text)
		}
	}
	for name, fields := range map[string]map[string]any{"left out": {}, "named": {"account": "work"}, "empty": {"account": ""}} {
		if code, text := hire(fields); code != http.StatusOK {
			t.Fatalf("%s: %d %q", name, code, text)
		}
		hires := registered(t)
		last := hires[len(hires)-1]
		if script := r.script(last); last.Role != "janitor" || last.Account != "work" || !strings.Contains(script, work) {
			t.Errorf("%s: the hire %+v works elsewhere:\n%s", name, last, script)
		}
	}
	// A GM on the default account: its hires are on the default one, and another is refused, as before.
	d, _, _, token, project = scopedGM(t)
	body, _ := json.Marshal(map[string]any{"role": "janitor", "folder": project, "account": "work"})
	if code, text := scopedCall(t, d, token, "POST", "/setup/gm/hire", string(body)); !scopeRefused(code, text) ||
		text != "This General Manager hires only on your default Claude account, the one it was hired on." {
		t.Errorf("a default GM naming another: %d %q", code, text)
	}
	// With two accounts saved, the route asks which of a hire that names none (C3 A2); a GM on the default one
	// has it filled in by name, so its hire as the demo's task writes it (no account) isn't asked.
	d, r, _, token, project = scopedGMOn(t, claudeDefaultName, withWork)
	body, _ = json.Marshal(map[string]any{"role": "janitor", "folder": project})
	if code, text := scopedCall(t, d, token, "POST", "/setup/gm/hire", string(body)); code != http.StatusOK {
		t.Fatalf("a default GM with two accounts saved, naming none: %d %q", code, text)
	}
	hires := registered(t)
	if last := hires[len(hires)-1]; last.Role != "janitor" || strings.Contains(r.script(last), work) {
		t.Errorf("a default GM's hire works elsewhere: %+v\n%s", last, r.script(last))
	}
	// A test copy on a Mac (never the default account there): its GM on the test account hires on it.
	if runtime.GOOS == "darwin" {
		onPort(t, portFromEnvironment("")+1)
		d, _, _, token, project = scopedGMOn(t, "work", withWork)
		body, _ := json.Marshal(map[string]any{"role": "janitor", "folder": project})
		if code, text := scopedCall(t, d, token, "POST", "/setup/gm/hire", string(body)); code != http.StatusOK {
			t.Errorf("a test copy's GM on its test account: %d %q", code, text)
		}
	}
}

// RX1d N1: the owner setting the GM's row to failed or fired from the page takes its token away, as a fire does:
// the scope follows the row, so a token left working would be unscoped.
func TestTheOwnersRegistryEditTakesAGMsTokenAway(t *testing.T) {
	for _, state := range []string{"failed", "fired"} {
		d, _, _, token, project := scopedGM(t)
		gm := registered(t)[0]
		body, _ := json.Marshal(map[string]any{"id": gm.ID, "state": state})
		w := serve(t, d, call{method: "POST", target: "/setup/gm/registry", host: "localhost", from: thisComputer, body: string(body),
			headers: map[string]string{setupHeader: "1"}})
		if w.Code != http.StatusOK || gmScopeNow() != "" {
			t.Fatalf("%s: %d %s", state, w.Code, w.Body)
		}
		d.mu.Lock()
		left := d.gmToken
		d.mu.Unlock()
		hire, _ := json.Marshal(map[string]any{"role": "janitor", "folder": filepath.Join(homeDir, "elsewhere")})
		os.MkdirAll(filepath.Join(homeDir, "elsewhere"), 0o755)
		if code, text := scopedCall(t, d, token, "POST", "/setup/gm/hire", string(hire)); left != "" || code < 400 {
			t.Errorf("%s: the token is left (%v), and an unscoped hire outside %s is %d %q", state, left != "", project, code, text)
		}
	}
	// A nickname or notes edit keeps the token.
	d, _, _, token, _ := scopedGM(t)
	body, _ := json.Marshal(map[string]any{"id": registered(t)[0].ID, "notes": "planning"})
	serve(t, d, call{method: "POST", target: "/setup/gm/registry", host: "localhost", from: thisComputer, body: string(body),
		headers: map[string]string{setupHeader: "1"}})
	d.mu.Lock()
	kept := d.gmToken == token
	d.mu.Unlock()
	if !kept {
		t.Error("a notes edit took the GM's token away")
	}
}

// scopedCall is a request with the GM's token alone.
func scopedCall(t *testing.T, d *Deck, token, method, target, body string) (int, string) {
	t.Helper()
	w := serve(t, d, call{method: method, target: target, host: "localhost", from: thisComputer, body: body, withoutSetupKey: true,
		headers: map[string]string{setupHeader: "1", gmTokenHeader: token}})
	d.watching.Wait()
	d.mu.Lock()
	d.lastHire = time.Time{} // the next hire needn't wait out the cooldown
	d.mu.Unlock()
	return w.Code, strings.TrimSpace(w.Body.String())
}

// scopeRefused is the scope's answer: 403, one sentence that says whose it is.
func scopeRefused(code int, text string) bool {
	return code == http.StatusForbidden && strings.HasPrefix(text, "This General Manager ") && !strings.Contains(text, "\n") &&
		strings.Count(text, ". ") == 0
}

func TestTheOwnersGMHireKeepsItsScope(t *testing.T) {
	isolate(t)
	project := filepath.Join(homeDir, "proj")
	os.MkdirAll(project, 0o755)
	os.MkdirAll(filepath.Join(homeDir, "other"), 0o755)
	roles := []Role{{ID: gmRole, Name: "General Manager", Agent: "claude"}, {ID: "tester", Name: "Tester", Agent: "claude"}}
	ask := func(fields map[string]any) (setupHire, int, string) {
		body, _ := json.Marshal(fields)
		return setupHireRequest(body, roles, nil, homeDir)
	}
	hire, status, why := ask(map[string]any{"role": gmRole, "folder": project, "scope": "~/proj"})
	if status != 0 || hire.Scope != physical(project) {
		t.Errorf("a GM held to its folder: %+v %d %q", hire, status, why)
	}
	if hire, status, _ := ask(map[string]any{"role": gmRole, "folder": project}); status != 0 || hire.Scope != "" {
		t.Errorf("no scope asked: %+v %d", hire, status)
	}
	for name, c := range map[string]struct {
		fields map[string]any
		status int
	}{
		"a hire that isn't the GM":  {map[string]any{"role": "tester", "folder": project, "scope": project}, 400},
		"another folder":            {map[string]any{"role": gmRole, "folder": project, "scope": filepath.Join(homeDir, "other")}, 400},
		"not text":                  {map[string]any{"role": gmRole, "folder": project, "scope": true}, 400},
		"empty":                     {map[string]any{"role": gmRole, "folder": project, "scope": " "}, 400},
		"a folder that isn't there": {map[string]any{"role": gmRole, "folder": project, "scope": filepath.Join(project, "gone")}, 400},
		"into T3 Code":              {map[string]any{"role": gmRole, "folder": project, "scope": project, "engine": "t3"}, 409},
	} {
		if _, status, why := ask(c.fields); status != c.status || why == "" {
			t.Errorf("%s: %d %q, want %d", name, status, why, c.status)
		}
	}
	// Kept in its registry row, and only when set: every other row is written as it was.
	shortSupport(t)
	writeRegistry(t, Hire{ID: "g1", Name: "General Manager", Role: gmRole, Folder: project, State: "working", Scope: project},
		Hire{ID: "t1", Name: "Tester", Role: "tester", Folder: project, State: "working"})
	if err := updateRegistry(func(*registryFile) {}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(registryPath())
	if hires := registered(t); hires[0].Scope != project || hires[1].Scope != "" || strings.Count(string(data), `"scope"`) != 1 {
		t.Errorf("registry: %+v\n%s", hires, data)
	}
}

// The scope is the open GM hired last: each GM's hire makes a new token in the older one's place.
func TestTheScopeIsTheNewestOpenGMs(t *testing.T) {
	shortSupport(t)
	for _, c := range []struct {
		hires []Hire
		want  string
	}{
		{nil, ""},
		{[]Hire{{ID: "g1", Name: "GM", Role: gmRole, Folder: "/p", State: "working", Started: 1, Scope: "/p"}}, "/p"},
		{[]Hire{{ID: "g1", Name: "GM", Role: gmRole, Folder: "/p", State: "working", Started: 1, Scope: "/p"},
			{ID: "g2", Name: "GM", Role: gmRole, Folder: "/q", State: "working", Started: 2}}, ""},
		{[]Hire{{ID: "g1", Name: "GM", Role: gmRole, Folder: "/q", State: "working", Started: 1},
			{ID: "g2", Name: "GM", Role: gmRole, Folder: "/p", State: "starting", Started: 2, Scope: "/p"}}, "/p"},
		{[]Hire{{ID: "g1", Name: "GM", Role: gmRole, Folder: "/p", State: "fired", Started: 1, Scope: "/p"}}, ""},
		{[]Hire{{ID: "t1", Name: "Tester", Role: "tester", Folder: "/p", State: "working", Started: 1, Scope: "/p"}}, ""},
	} {
		writeRegistry(t, c.hires...)
		if got := gmScopeNow(); got != c.want {
			t.Errorf("%+v: scope %q, want %q", c.hires, got, c.want)
		}
	}
}

// Item 1, the hire: only into the project or the top of one of its own worktrees, a folder that is there, in a
// terminal, on the default account. RX1 B1/B2, RX1b B1 (smolcorp mcp: the same route), RX1c B1 and S2 (a link).
func TestAScopedGMHiresOnlyIntoItsProject(t *testing.T) {
	d, r, h, token, project := scopedGM(t)
	elsewhere := filepath.Join(homeDir, "elsewhere")
	other := filepath.Join(homeDir, "other-repo")
	for _, dir := range []string{elsewhere, filepath.Join(project, "sub2"), other} {
		os.MkdirAll(dir, 0o755)
	}
	gitOut(t, other, "init", "-q")
	os.Symlink(elsewhere, filepath.Join(homeDir, "p"))
	os.Symlink(elsewhere, filepath.Join(project, "p"))
	hire := func(fields map[string]any) (int, string) {
		body, _ := json.Marshal(fields)
		return scopedCall(t, d, token, "POST", "/setup/gm/hire", string(body))
	}
	trees, _ := filepath.Glob(filepath.Join(filepath.Dir(project), "*-hires", "*"))
	for name, fields := range map[string]map[string]any{
		"outside the project (RX1 B1, RX1b B1)":      {"role": "janitor", "folder": elsewhere},
		"the home folder":                            {"role": "janitor", "folder": "~"},
		"the folder the project is in (RX1c B1)":     {"role": "janitor", "folder": filepath.Dir(project)},
		"a folder in the project (RX1c B1)":          {"role": "janitor", "folder": filepath.Join(project, "sub2")},
		"a folder that isn't there (RX1c S2)":        {"role": "janitor", "folder": filepath.Join(project, "not-yet")},
		"a link out, beside it (RX1c S2)":            {"role": "janitor", "folder": filepath.Join(homeDir, "p")},
		"a link out, in it (RX1c S2)":                {"role": "janitor", "folder": filepath.Join(project, "p")},
		"another repository":                         {"role": "janitor", "folder": other},
		"no folder: the one for new hires":           {"role": "janitor"},
		"the API key (RX1 B2)":                       {"role": "janitor", "folder": project, "apiKey": true},
		"another account":                            {"role": "janitor", "folder": project, "account": "work"},
		"T3 Code":                                    {"role": "janitor", "folder": project, "engine": "t3"},
		"the project, spelt with .. out and back in": {"role": "janitor", "folder": project + "/../" + filepath.Base(project) + "/sub2/.."},
	} {
		code, text := hire(fields)
		if name == "the project, spelt with .. out and back in" {
			if code != http.StatusOK {
				t.Errorf("%s: %d %q, want 200", name, code, text)
			}
			continue
		}
		if !scopeRefused(code, text) {
			t.Errorf("%s: %d %q, want the scope's 403", name, code, text)
		}
	}
	if now, _ := filepath.Glob(filepath.Join(filepath.Dir(project), "*-hires", "*")); len(now) != len(trees)+1 || len(h.windows) != 2 {
		t.Errorf("a refused hire made something: worktrees %q, windows %q", now, h.windows)
	}
	// Into the project, and into the top of a worktree of it: the office hires as it always does.
	if code, text := hire(map[string]any{"role": "janitor", "folder": project}); code != http.StatusOK {
		t.Fatalf("into the project: %d %q", code, text)
	}
	hires := registered(t)
	wt := hires[len(hires)-1].Worktree
	if wt == "" || !(gmScope{root: project, git: runGit}).holds(wt) {
		t.Fatalf("the hire's worktree %q isn't the project's", wt)
	}
	if code, text := hire(map[string]any{"role": "janitor", "folder": wt}); code != http.StatusOK {
		t.Errorf("into a worktree of the project: %d %q", code, text)
	}
	if !r.tmux.ran("new-session") {
		t.Error("no hire was made")
	}
}

// RX1b S2 / RX1c S3: the hire routes spelt another way reach a route the scope holds; RX1b's /setup/hire (the
// setup sheet takes any folder) and the switches (RX1b S2's note) are the office's, not this GM's.
func TestAScopedGMCantHireOrChangeTheOfficeAnotherWay(t *testing.T) {
	d, _, _, token, _ := scopedGM(t)
	outside := `{"role":"janitor","folder":"` + filepath.Join(homeDir, "elsewhere") + `"}`
	os.MkdirAll(filepath.Join(homeDir, "elsewhere"), 0o755)
	for _, c := range []struct{ method, target, body string }{
		{"POST", "/setup/gm/%68ire", outside},
		{"POST", "/setup/gm/hir%65", outside},
		{"POST", "/setup/hire", outside},
		{"POST", "/setup/hire/gm", ""},
		{"POST", "/setup/gm/manager?enabled=0", ""},
		{"POST", "/setup/hiring?enabled=0", ""},
		{"POST", "/setup/hire-folder?folder=" + homeDir, ""},
		{"POST", "/setup/roles", `{}`},
		{"POST", "/setup/hooks", ""},
		{"POST", "/setup/layout", `{}`},
		{"POST", "/setup/gm/registry", `{"id":"x","state":"fired"}`},
		{"PUT", "/setup/gm/hire", outside},
		{"DELETE", "/setup/gm/fire", `{"id":"x"}`},
	} {
		if code, text := scopedCall(t, d, token, c.method, c.target, c.body); !scopeRefused(code, text) {
			t.Errorf("%s %s: %d %q, want the scope's 403", c.method, c.target, code, text)
		}
	}
	// Not a route at all: the route table refuses it first, as for any caller but the page and a tool.
	if code, _ := scopedCall(t, d, token, "POST", "/setup//hire", outside); code != http.StatusForbidden || len(registered(t)) != 1 {
		t.Errorf("/setup//hire: %d, registry %+v", code, registered(t))
	}
	// What it reads is as before.
	if code, text := scopedCall(t, d, token, "GET", "/setup/gm/team", ""); code != http.StatusOK {
		t.Errorf("team: %d %q", code, text)
	}
}

// RX1b B3, RX1c S4: send, brief, fire, contacts, nickname, pin, unpin, tile, screen, watch and unwatch reach
// only the GM's own hires and folders, after a restart too: the scope is the registry's, not the token's.
func TestAScopedGMReachesOnlyItsOwnHiresAfterARestartToo(t *testing.T) {
	d, _, _, _, project := scopedGM(t)
	elsewhere := filepath.Join(homeDir, "elsewhere")
	os.MkdirAll(elsewhere, 0o755)
	wt := filepath.Join(filepath.Dir(project), "proj-hires", "tester-1")
	gitOut(t, project, "worktree", "add", "-q", "-b", "hire/tester-1", wt)
	gm := registered(t)[0]
	writeRegistry(t, gm,
		Hire{ID: "tester-1", Name: "Tester", Nickname: "Ada", Role: "tester", Folder: wt, Worktree: wt, Repo: project, State: "working", Started: 2},
		Hire{ID: "rev-9", Name: "Reviewer", Nickname: "Rex", Role: "reviewer", Folder: elsewhere, State: "working", Started: 3},
		Hire{ID: "tester-2", Name: "Tester", Nickname: "Bo", Role: "tester", Folder: project, Worktree: filepath.Join(elsewhere, "wt"), State: "working", Started: 4})
	// The app starts again: a new deck on the same support folder, the registry as it was, a new token.
	d.stopWatchers()
	d = startDeck(t)
	d.config.ManagerEnabled, d.config.HiringEnabled, d.config.HireInTmux = true, true, true
	d.config.Pins[1] = &elsewhere // the owner's pin
	owners, refused := d.watchers.add(map[string]any{"kind": "time", "target": "tea", "minutes": 5.0}, "owner")
	if refused != nil {
		t.Fatal(refused.message)
	}
	token := d.newGMToken()
	refusedCalls := []struct{ method, target, body string }{
		{"POST", "/setup/gm/send", `{"to":"rev-9","text":"hi"}`},
		{"POST", "/setup/gm/send", `{"to":"Rex","text":"hi"}`},
		{"POST", "/setup/gm/send", `{"to":"tester-2","text":"hi"}`},
		{"POST", "/setup/gm/send", `{"to":"Tester","text":"hi"}`}, // two are called that: an error, not a hire of its own
		{"POST", "/setup/gm/send", `{"to":"bot:x","text":"hi"}`},
		{"POST", "/setup/gm/send", `{"to":"Ada","text":"hi","to":"Rex"}`},
		{"POST", "/setup/gm/brief", `{"id":"rev-9","text":"review my branch"}`},
		{"POST", "/setup/gm/fire", `{"id":"rev-9"}`},
		{"POST", "/setup/gm/fire", `{"id":"tester-2","force":true}`},
		{"POST", "/setup/gm/contacts", `{"from":"Ada","to":"Rex"}`},
		{"POST", "/setup/gm/nickname", `{"folder":"` + elsewhere + `","name":"Boss"}`},
		{"POST", "/setup/gm/nickname", `{"folder":"` + project + `","name":"x","Folder":"` + elsewhere + `"}`},
		{"POST", "/setup/pin?key=2&folder=" + elsewhere, ""},
		{"POST", "/setup/pin?key=1&folder=" + project, ""}, // over the owner's pin
		{"POST", "/setup/unpin?key=1", ""},
		{"POST", "/setup/gm/tile", ""},
		{"POST", "/setup/gm/screen?id=1", ""},
		{"POST", "/setup/gm/watch", `{"kind":"exit","target":"rev-9"}`},
		{"POST", "/setup/gm/watch", `{"kind":"IDLE","target":"Rex","minutes":5}`},
		{"POST", "/setup/gm/watch", `{"kind":"file","target":"` + filepath.Join(elsewhere, "x.log") + `","pattern":"x"}`},
		{"POST", "/setup/gm/watch", `{"kind":"file","target":"../../elsewhere/x.log","hire":"Ada","pattern":"x"}`},
		{"POST", "/setup/gm/unwatch?id=" + owners.ID, ""},
		{"DELETE", "/setup/gm/unwatch", `{"id":"` + owners.ID + `"}`},
	}
	for _, c := range refusedCalls {
		if code, text := scopedCall(t, d, token, c.method, c.target, c.body); !scopeRefused(code, text) {
			t.Errorf("%s %s %s: %d %q, want the scope's 403", c.method, c.target, c.body, code, text)
		}
	}
	// Its own: the routes answer as they always do (whatever that is), never the scope.
	for _, c := range []struct{ method, target, body string }{
		{"POST", "/setup/gm/send", `{"to":"Ada","text":"hi"}`},
		{"POST", "/setup/gm/send", `{"to":"gm","text":"hi"}`},
		{"POST", "/setup/gm/brief", `{"id":"tester-1","text":"and the docs"}`},
		{"POST", "/setup/gm/nickname", `{"folder":"` + wt + `","name":"Ada"}`},
		{"POST", "/setup/pin?key=3&folder=" + wt, ""},
		{"POST", "/setup/unpin?key=3", ""},
		{"POST", "/setup/gm/watch", `{"kind":"exit","target":"tester-1"}`},
		{"POST", "/setup/gm/watch", `{"kind":"file","target":"` + filepath.Join(wt, "x.log") + `","pattern":"x"}`},
		{"POST", "/setup/gm/watch", `{"kind":"time","target":"look again","minutes":5}`},
		{"POST", "/setup/gm/watch", `{"kind":"event","target":"mail"}`},
	} {
		if code, text := scopedCall(t, d, token, c.method, c.target, c.body); scopeRefused(code, text) {
			t.Errorf("%s %s %s: %d %q, refused", c.method, c.target, c.body, code, text)
		}
	}
	if pins := d.config.Pins; len(pins) < 2 || pins[1] == nil || *pins[1] != elsewhere {
		t.Errorf("the owner's pin moved: %v", pins)
	}
}

// A GM with no scope, every other, is as it always was: the same calls go through to their routes.
func TestAGMWithNoScopeIsAsBefore(t *testing.T) {
	d, _, _, token := gmHireDeck(t)
	elsewhere := filepath.Join(homeDir, "elsewhere")
	os.MkdirAll(elsewhere, 0o755)
	writeRegistry(t, Hire{ID: "g1", Name: "General Manager", Role: gmRole, Folder: d.config.HireFolder, State: "working", Started: 1},
		Hire{ID: "rev-9", Name: "Reviewer", Nickname: "Rex", Role: "reviewer", Folder: elsewhere, State: "working", Started: 3})
	for _, c := range []struct {
		method, target, body string
		status               int
	}{
		{"POST", "/setup/gm/hire", `{"role":"janitor","folder":"` + elsewhere + `"}`, http.StatusOK},
		{"POST", "/setup/gm/send", `{"to":"Rex","text":"hi"}`, http.StatusOK},
		{"POST", "/setup/pin?key=2&folder=" + elsewhere, "", http.StatusNoContent},
		{"POST", "/setup/gm/tile", "", http.StatusNotFound},
		{"POST", "/setup/hiring?enabled=1", "", http.StatusOK},
	} {
		if code, text := scopedCall(t, d, token, c.method, c.target, c.body); code != c.status {
			t.Errorf("%s %s: %d %q, want %d", c.method, c.target, code, text, c.status)
		}
	}
}
