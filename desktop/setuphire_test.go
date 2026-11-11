//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// homeRepo is a git repo inside the test's home folder, where setup may hire.
func homeRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(homeDir, "Projects", "app")
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	repo = physical(repo)
	os.WriteFile(filepath.Join(repo, "sub", "a.txt"), []byte("a"), 0o644)
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "one"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		if args[0] == "init" {
			stopSigning(t, repo)
		}
	}
	return repo
}

// Setup's hire sheet hires through the engine with everything it said: the
// answers and the task reach the brief, the nickname and model the registry,
// the folder the recent list; it's answered with the desk it sits at.
func TestSetupsHireSheetHiresWithWhatItSaid(t *testing.T) {
	d, _, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	repo := homeRepo(t)
	body, _ := json.Marshal(map[string]any{"role": "janitor", "folder": repo, "answers": []string{"the login page"},
		"task": "Fix the login page's tab order.", "nickname": "Ana", "model": "claude-sonnet-5-5", "worktree": true, "floor": 0})
	w := serve(t, d, call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer, body: string(body),
		headers: map[string]string{setupHeader: "1"}})
	if w.Code != http.StatusOK {
		t.Fatalf("hire: %d %s", w.Code, w.Body)
	}
	var answer map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil || answer["ok"] != true || answer["desk"] != float64(0) || answer["session"] != nil {
		t.Fatalf("answer %s", w.Body)
	}
	d.watching.Wait()
	got := registered(t)
	if len(got) != 1 || got[0].Nickname != "Ana" || got[0].Model != "claude-sonnet-5-5" || got[0].Worktree == "" || got[0].Brief == "" {
		t.Fatalf("registry %+v", got)
	}
	if !strings.Contains(got[0].Branch, "ana-") {
		t.Errorf("the worktree is named for the nickname: %q", got[0].Branch)
	}
	brief, _ := os.ReadFile(got[0].Brief)
	if !strings.Contains(string(brief), "Fix the login page's tab order.") {
		t.Errorf("the task isn't in the brief:\n%s", brief)
	}
	settings, _ := os.ReadFile(filepath.Join(hireFolder(supportDir, got[0].ID), hireSettingsFile))
	if strings.Contains(string(settings), `"defaultMode" : "auto"`) != (runtime.GOOS == "darwin") {
		t.Errorf("auto mode on a Mac only, as the Mac's HireSettings: %s", settings)
	}
	if !strings.Contains(string(settings), absoluteRulePath(filepath.Join(homeDir, ".claude"))+"/settings*.json") {
		t.Error("the default account's settings aren't denied by name")
	}
	d.mu.Lock()
	recent := d.config.RecentHireFolders
	d.mu.Unlock()
	if len(recent) == 0 || recent[0] != repo {
		t.Errorf("recent folders %v", recent)
	}
}

// An account saved in config.json is one a hire can work on, as setup lists it: loaded from the file, the
// sheet's hire names it (no "There's no Claude account called work"), the registry keeps its name, and the
// hire's settings deny it the account's settings and transcripts as they do the default's.
func TestAHireWorksOnAnAccountSavedInConfigJSON(t *testing.T) {
	d, _, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	repo := homeRepo(t)
	// Not a ~/.claude* folder: only the account's own rules cover it.
	work := filepath.Join(homeDir, "accounts", "work")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"settings.json", ".claude.json"} { // Claude Code started once on it
		os.WriteFile(filepath.Join(work, name), []byte("{}"), 0o600)
	}
	// config.json as the Mac (or setup) leaves it, then read back the way the app starts.
	d.mu.Lock()
	d.config.save()
	data, err := os.ReadFile(configFile)
	var fields map[string]any
	if err == nil {
		err = json.Unmarshal(data, &fields)
	}
	if err != nil {
		d.mu.Unlock()
		t.Fatalf("config.json: %v", err)
	}
	fields["claudeProfiles"] = []map[string]string{{"name": "work", "dir": "~/accounts/work"}}
	data, _ = json.Marshal(fields)
	os.WriteFile(configFile, data, 0o600)
	d.config = loadConfig()
	d.mu.Unlock()

	body, _ := json.Marshal(map[string]any{"role": "janitor", "folder": repo, "account": "work"})
	w := serve(t, d, call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer, body: string(body),
		headers: map[string]string{setupHeader: "1"}})
	if w.Code != http.StatusOK {
		t.Fatalf("hire on the saved account: %d %s", w.Code, w.Body)
	}
	d.watching.Wait()
	got := registered(t)
	if len(got) != 1 || got[0].Account != "work" {
		t.Fatalf("registry %+v", got)
	}
	settings, _ := os.ReadFile(filepath.Join(hireFolder(supportDir, got[0].ID), hireSettingsFile))
	for _, rule := range []string{"Edit(" + absoluteRulePath(work) + "/settings*.json)", "Read(" + absoluteRulePath(work) + "/projects/**)"} {
		if !strings.Contains(string(settings), rule) {
			t.Errorf("the saved account's %s isn't denied:\n%s", rule, settings)
		}
	}
	// Among two accounts, a hire must still say which.
	body, _ = json.Marshal(map[string]any{"role": "janitor", "folder": repo})
	w = serve(t, d, call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer, body: string(body),
		headers: map[string]string{setupHeader: "1"}})
	if w.Code != http.StatusBadRequest || w.Body.String() != "Say which Claude account to hire on: default, work.\n" {
		t.Errorf("no account named: %d %q", w.Code, w.Body)
	}
}

// A saved account outside the home folder (one the Mac kept, say) is no account a hire works on, but every hire is
// still denied its settings and transcripts, as on the Mac and before setup and the hires shared one checker. A
// network, relative or NUL path gives no rule at all.
func TestEverySavedAccountsFolderIsDeniedToAHire(t *testing.T) {
	d, _, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	repo := homeRepo(t)
	outside := claudeFolder(t, filepath.Join(physical(t.TempDir()), "volume", ".claude-ext"), "settings.json")
	d.mu.Lock()
	d.config.ClaudeProfiles = []claudeProfile{{Name: "ext", Dir: outside}, {Name: "share", Dir: "//server/share/.claude-s"},
		{Name: "rel", Dir: "rel/.claude-r"}, {Name: "nul", Dir: homeDir + "/.claude-n\x00"}}
	d.mu.Unlock()

	body, _ := json.Marshal(map[string]any{"role": "janitor", "folder": repo, "account": "ext"})
	w := serve(t, d, call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer, body: string(body),
		headers: map[string]string{setupHeader: "1"}})
	if w.Code != http.StatusNotFound || w.Body.String() != "There's no Claude account called ext. Pick one of: default.\n" {
		t.Errorf("a hire on the outside account: %d %q", w.Code, w.Body)
	}

	body, _ = json.Marshal(map[string]any{"role": "janitor", "folder": repo})
	w = serve(t, d, call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer, body: string(body),
		headers: map[string]string{setupHeader: "1"}})
	if w.Code != http.StatusOK {
		t.Fatalf("a hire on the default account: %d %s", w.Code, w.Body)
	}
	d.watching.Wait()
	got := registered(t)
	if len(got) != 1 || got[0].Account != "" && got[0].Account != claudeDefaultName {
		t.Fatalf("registry %+v", got)
	}
	settings, _ := os.ReadFile(filepath.Join(hireFolder(supportDir, got[0].ID), hireSettingsFile))
	for _, rule := range []string{"Edit(" + absoluteRulePath(outside) + "/settings*.json)", "Read(" + absoluteRulePath(outside) + "/projects/**)"} {
		if !strings.Contains(string(settings), rule) {
			t.Errorf("the outside account's %s isn't denied:\n%s", rule, settings)
		}
	}
	for _, part := range []string{"server/share", ".claude-s", ".claude-r", ".claude-n"} {
		if strings.Contains(string(settings), part) {
			t.Errorf("a rule for a badly formed folder (%s):\n%s", part, settings)
		}
	}
}

// With the manager off and no worktree asked for, the hire works in the folder
// itself, as the Mac's; a floor above answers with the floor and no desk.
func TestSetupsHireWithoutAWorktreeWorksInTheFolder(t *testing.T) {
	d, _, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	repo := homeRepo(t)
	status, answer, why := d.hireFromSetupSheet(setupHire{Role: "janitor", Folder: repo, Answers: []string{}, Floor: 2})
	if status != http.StatusOK {
		t.Fatalf("%d %s", status, why)
	}
	if body := answer.(map[string]any); body["floor"] != 2 || body["desk"] != nil {
		t.Errorf("answer %v", body)
	}
	d.watching.Wait()
	if got := registered(t); len(got) != 1 || got[0].Worktree != "" || got[0].Folder != repo {
		t.Fatalf("registry %+v", got)
	}
}

// Each refusal in the Mac's words, before anything is made.
func TestSetupsHireRequestIsCheckedAsTheMacs(t *testing.T) {
	isolate(t)
	home := physical(t.TempDir())
	for _, dir := range []string{"Projects/app", "Library/x", ".ssh/x"} {
		os.MkdirAll(filepath.Join(home, dir), 0o700)
	}
	os.WriteFile(filepath.Join(home, "Projects", "file.txt"), nil, 0o600)
	app := filepath.Join(home, "Projects", "app")
	roles := []Role{{ID: "janitor", Name: "Janitor", Agent: "claude"}, {ID: "writer", Name: "Writer", Agent: "codex"}}
	one := []claudeProfile{{Name: "default", Dir: filepath.Join(home, ".claude")}}
	two := append(one, claudeProfile{Name: "work", Dir: filepath.Join(home, ".claude-work")})
	machine := "computer's"
	if runtime.GOOS == "darwin" {
		machine = "Mac's"
	}
	for _, c := range []struct {
		body     string
		accounts []claudeProfile
		status   int
		why      string
	}{
		{`[]`, one, 400, "Send {role, folder, answers}."},
		{`{"folder":"~/Projects/app"}`, one, 400, "Say which role to hire."},
		{`{"role":"gone"}`, one, 404, "That role isn't there any more. Pick another."},
		{`{"role":"janitor","model":"sonnet 5"}`, one, 400, "A model is a name such as claude-sonnet-5-5."},
		{`{"role":"janitor","nickname":"` + strings.Repeat("a", 41) + `"}`, one, 400, "A nickname is one line of at most 40 characters."},
		{`{"role":"janitor","nickname":"a\nb"}`, one, 400, "A nickname is one line of at most 40 characters."},
		{`{"role":"janitor","engine":"t3"}`, two, 400, "Say which folder to hire into."},
		{`{"role":"janitor","engine":"t3","apiKey":true,"folder":"~/Projects/none"}`, two, 400, "There's no folder at ~/Projects/none."},
		{`{"role":"janitor","engine":"vim"}`, one, 400, `Where it works is "t3" (T3 Code), or left out for a terminal.`},
		{`{"role":"janitor","folder":"~/Projects/app"}`, two, 400, "Say which Claude account to hire on: default, work."},
		{`{"role":"janitor","account":"home","folder":"~/Projects/app"}`, two, 404, "There's no Claude account called home. Pick one of: default, work."},
		{`{"role":"janitor","apiKey":true,"folder":"~/Projects/app"}`, two, 409, setupHireNotAPIKey},
		{`{"role":"janitor"}`, one, 400, "Say which folder to hire into."},
		{`{"role":"janitor","folder":"Projects/app"}`, one, 400, "Give the folder's full path, such as ~/Projects/app."},
		{`{"role":"janitor","folder":"~/Projects/none"}`, one, 400, "There's no folder at ~/Projects/none."},
		{`{"role":"janitor","folder":"~/Projects/file.txt"}`, one, 400, "~/Projects/file.txt is a file, not a folder."},
		{`{"role":"janitor","folder":"/"}`, one, 400, "Hire into a folder inside your home folder."},
		{`{"role":"janitor","folder":"~"}`, one, 400, "Hire into a folder inside your home folder."},
		{`{"role":"janitor","folder":"~/Library/x"}`, one, 400, "That's a folder for your " + machine + " or your apps' own files. Hire into a project folder."},
		{`{"role":"janitor","folder":"~/.ssh/x"}`, one, 400, "That's a folder for your " + machine + " or your apps' own files. Hire into a project folder."},
		{`{"role":"janitor","folder":"~/Projects/app","task":"` + strings.Repeat("t", 8001) + `"}`, one, 400, "The task is at most 8000 characters."},
		{`{"role":"janitor","folder":"~/Projects/app","task":3}`, one, 400, "The task is text."},
		{`{"role":"janitor","folder":"~/Projects/app","answers":"yes"}`, one, 400, "Answers are a list of at most 8 texts."},
		{`{"role":"janitor","folder":"~/Projects/app","answers":[1]}`, one, 400, "Answers are a list of at most 8 texts."},
		{`{"role":"janitor","folder":"~/Projects/app","answers":["1","2","3","4","5","6","7","8","9"]}`, one, 400, "Answers are a list of at most 8 texts."},
	} {
		_, status, why := setupHireRequest([]byte(c.body), roles, c.accounts, home)
		if status != c.status || why != c.why {
			t.Errorf("%s: %d %q, want %d %q", c.body, status, why, c.status, c.why)
		}
	}
	// What's kept: the folder resolved, the task cleaned, the account's folder (none for the default), no answers as none.
	hire, status, why := setupHireRequest([]byte(`{"role":"janitor","folder":" ~/Projects/app ","account":"work","task":"  Do it.\u0007 ","nickname":" Ana ","floor":2}`),
		roles, two, home)
	if status != 0 || hire.Folder != app || hire.Task != "Do it." || hire.Account != "work" || hire.configDir != two[1].Dir ||
		hire.Nickname != "Ana" || hire.Floor != 2 || hire.Answers == nil || len(hire.Answers) != 0 {
		t.Errorf("%d %q %+v", status, why, hire)
	}
	// Into T3 Code: no account asked for among several; one named, or the API-key account, kept for T3 Code's own refusal.
	if hire, status, why := setupHireRequest([]byte(`{"role":"janitor","engine":"t3","folder":"~/Projects/app"}`), roles, two, home); status != 0 || !hire.T3 ||
		hire.Account != "" || hire.APIKey || hire.Folder != app {
		t.Errorf("into T3 Code: %d %q %+v", status, why, hire)
	}
	if hire, status, why := setupHireRequest([]byte(`{"role":"janitor","engine":"t3","folder":"~/Projects/app","account":" work ","apiKey":true}`), roles, two, home); status != 0 ||
		!hire.T3 || hire.Account != "work" || !hire.APIKey || hire.configDir != "" {
		t.Errorf("into T3 Code, an account: %d %q %+v", status, why, hire)
	}
	// Another agent names no account; a floor that isn't one is the ground floor.
	if hire, status, _ := setupHireRequest([]byte(`{"role":"writer","folder":"~/Projects/app","floor":9}`), roles, two, home); status != 0 || hire.Account != "" || hire.Floor != 0 {
		t.Errorf("another agent: %d %+v", status, hire)
	}
}

// The GM's token never hires the GM, as the Mac's gmHireRefusal; the page may.
func TestTheGMNeverHiresTheGMFromSetupsHire(t *testing.T) {
	if why := gmSetupHireRefusal(gmRole, callerGM, nil); why != onlyOwnerHiresGMRefusal {
		t.Errorf("no GM yet: %q", why)
	}
	if why := gmSetupHireRefusal(gmRole, callerGM, []Hire{{Role: gmRole, State: "working"}}); why != secondGMRefusal {
		t.Errorf("a GM there: %q", why)
	}
	if gmSetupHireRefusal(gmRole, callerPage, nil) != "" || gmSetupHireRefusal("janitor", callerGM, nil) != "" {
		t.Error("the page hires the GM, and the GM hires others")
	}
}

// A test copy never hires on the default account; on a Mac it hires on a named
// one, as the Mac's Swift engine does.
func TestATestCopyHiresOnANamedAccountOnAMac(t *testing.T) {
	if testCopyRefusal(7779, "claude", "") == nil {
		t.Error("a test copy hired on the default account")
	}
	if err := testCopyRefusal(7779, "claude", "/Users/me/.claude-test"); (err == nil) != (runtime.GOOS == "darwin") {
		t.Errorf("a named account on %s: %v", runtime.GOOS, err)
	}
}

// B3 (RM4): with hiring off, setup's hire is refused for the page and the GM
// alike, as the Mac's Deck.hire refuses first for every caller; Start here's
// own GM hire (hireFromSetup) keeps its rule and isn't refused for it.
func TestSetupsHireIsOffWhenHiringIsOff(t *testing.T) {
	d, _, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	managerOn(d)
	repo := homeRepo(t)
	d.mu.Lock()
	d.config.HiringEnabled = false
	d.mu.Unlock()
	gm := d.newGMToken()
	body := `{"role":"janitor","folder":"` + repo + `","answers":[]}`
	for _, c := range []struct {
		name string
		call call
	}{
		{"the page", call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer, body: body,
			headers: map[string]string{setupHeader: "1"}}},
		{"the GM", call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer, body: body,
			withoutSetupKey: true, headers: map[string]string{setupHeader: "1", gmTokenHeader: gm}}},
	} {
		w := serve(t, d, c.call)
		if w.Code != http.StatusForbidden || strings.TrimSpace(w.Body.String()) != hiringOff {
			t.Errorf("%s: %d %s", c.name, w.Code, w.Body)
		}
	}
	if got := registered(t); len(got) != 0 {
		t.Fatalf("hired with hiring off: %+v", got)
	}
	if status, message := d.hireFromSetup(gmRole); status == http.StatusForbidden || message == hiringOff {
		t.Errorf("Start here's GM hire: %d %s", status, message)
	}
	d.watching.Wait()
}

// N3 (RM4): a setup hire that goes the old way (here, a role on another agent)
// says its task and answers weren't passed on; the engine's says nothing of it.
func TestSetupsHireSaysWhenItsTaskWasntPassedOn(t *testing.T) {
	d, _, h := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	repo := homeRepo(t)
	status, answer, why := d.hireFromSetupSheet(setupHire{Role: "janitor", Folder: repo, Answers: []string{}, Task: "Sweep."})
	if status != http.StatusOK || answer.(map[string]any)["note"] != nil {
		t.Fatalf("the engine's hire: %d %v %s", status, answer, why)
	}
	d.watching.Wait()
	i := slices.IndexFunc(d.config.Roles, func(r Role) bool { return r.ID == "accountant" })
	d.config.Roles[i].Agent = "codex"
	d.lastHire = time.Time{}
	status, answer, why = d.hireFromSetupSheet(setupHire{Role: "accountant", Folder: repo, Answers: []string{"x"}, Task: "Count."})
	if status != http.StatusOK || answer.(map[string]any)["note"] != setupHireNotPassedOn || len(h.windows)+len(h.tmux) == 0 {
		t.Fatalf("the old way: %d %v %s, opened %q %q", status, answer, why, h.windows, h.tmux)
	}
}
