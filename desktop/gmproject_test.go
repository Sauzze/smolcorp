//go:build !windows

package desktop

import (
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// FXMAKEPROJECT: POST /setup/gm/project (gmproject.go), on a test deck whose GM was hired into a projects folder in
// the stand-in home. Clones come from a stand-in smart-http server over https on 127.0.0.1 (git http-backend behind
// net/http/cgi), never the internet.

// projectDeck is a test deck with the manager and hiring on, a GM hired into ~/Projects (a folder that isn't a repo,
// holding a repo), a hire with a send key, and the GM's token.
func projectDeck(t *testing.T) (d *Deck, projects, gm, hireKey string) {
	t.Helper()
	d = newTestDeck(t)
	managerOn(d)
	d.mu.Lock()
	d.config.HiringEnabled = true
	d.mu.Unlock()
	projects = filepath.Join(physical(homeDir), "Projects")
	if err := os.MkdirAll(filepath.Join(projects, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRegistryHires(t, Hire{ID: "gm1", Name: "General Manager", Role: gmRole, Folder: keptPath(projects), State: "working", Started: 1},
		Hire{ID: "h1", Name: "Janitor", Role: "janitor", Folder: keptPath(filepath.Join(projects, "app")), State: "working", Started: 2})
	key, err := newSendKey(registryPath(), "h1")
	if err != nil || key == "" {
		t.Fatalf("newSendKey: %q, %v", key, err)
	}
	return d, projects, d.newGMToken(), key
}

// askProject is POST /setup/gm/project as who, with body.
func askProject(t *testing.T, d *Deck, who caller, gm, hireKey string, body any) (int, string) {
	t.Helper()
	data, _ := json.Marshal(body)
	c := asCaller(who, gm, hireKey)
	c.method, c.target, c.host, c.from, c.body = "POST", gmProjectPath, "localhost:7710", thisComputer, string(data)
	c.headers = mergeHeaders(c.headers, map[string]string{setupHeader: "1"})
	w := serve(t, d, c)
	return w.Code, strings.TrimSpace(w.Body.String())
}

// projectsHold fails unless the projects folder holds exactly names.
func projectsHold(t *testing.T, projects string, names ...string) {
	t.Helper()
	entries, _ := os.ReadDir(projects)
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if strings.Join(got, " ") != strings.Join(names, " ") {
		t.Errorf("the projects folder holds %q, want %q", got, names)
	}
}

// projectLines are the office's lines about project folders made.
func projectLines(d *Deck) []eventEntry {
	var lines []eventEntry
	for _, e := range d.events.snapshot() {
		if e.Kind == eventKindGMProject {
			lines = append(lines, e)
		}
	}
	return lines
}

// standInGit runs git for the stand-in's own repos, outside the core, with no config of the owner's.
func standInGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := disclaim.Cmd(exec.Command("git", append([]string{"-c", "user.name=Ada", "-c", "user.email=ada@example.invalid",
		"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...))
	cmd.Dir = dir
	cmd.Env = []string{"HOME=" + homeDir, "PATH=/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "LANG=C"}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %q: %v\n%s", args, err, out)
	}
}

// httpsGitStandIn serves one bare repo, site.git (one commit with a README), over smart http on https://127.0.0.1,
// and has git trust its certificate (GIT_SSL_CAINFO, which the core's git keeps). Its base address, with no slash.
func httpsGitStandIn(t *testing.T) string {
	t.Helper()
	base, _, _ := heldGitStandIn(t, false)
	return base
}

// heldGitStandIn is httpsGitStandIn which, with hold, keeps the first clone waiting at its first request: held is
// closed once git has asked (its folder and .git made by then), and release lets it go on (by itself after 60 s).
func heldGitStandIn(t *testing.T, hold bool) (base string, held <-chan struct{}, release func()) {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git here")
	}
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	standInGit(t, work, "init", "-q")
	if err := os.WriteFile(filepath.Join(work, "README"), []byte("a stand-in project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	standInGit(t, work, "add", "README")
	standInGit(t, work, "commit", "-q", "-m", "first")
	standInGit(t, root, "clone", "-q", "--bare", work, filepath.Join(root, "repos", "site.git"))
	backend := &cgi.Handler{Path: gitPath, Args: []string{"http-backend"}, Root: "/",
		Env: []string{"GIT_PROJECT_ROOT=" + filepath.Join(root, "repos"), "GIT_HTTP_EXPORT_ALL=1", "HOME=" + homeDir,
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}}
	asked, let := make(chan struct{}), make(chan struct{})
	var once, letOnce sync.Once
	release = func() { letOnce.Do(func() { close(let) }) }
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hold {
			once.Do(func() {
				close(asked)
				select {
				case <-let:
				case <-time.After(60 * time.Second):
				}
			})
		}
		backend.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(release)
	ca := filepath.Join(root, "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_SSL_CAINFO", ca)
	// The test's git reads nothing of the owner's real home (curl's .netrc, say). In the app it does: git's curl
	// reads the owner's ~/.netrc, which every desk can read anyway.
	t.Setenv("HOME", homeDir)
	return server.URL, asked, release
}

// Each name the route refuses, with its words, and nothing made for any of them; the name's limits at their edge.
func TestANewProjectsNameIsOnePlainFolderName(t *testing.T) {
	d, projects, gm, _ := projectDeck(t)
	const plain = "A project's folder name is plain letters, digits, \"-\", \"_\" and \".\" only: no spaces, slashes or other signs."
	const dots = "A project's folder name can't start or end with a dot."
	for _, c := range []struct {
		body any
		want string
	}{
		{map[string]any{}, "Say the new project's folder name, as text."},
		{map[string]any{"name": 7}, "Say the new project's folder name, as text."},
		{map[string]any{"name": ""}, "Say the new project's folder name."},
		{map[string]any{"name": strings.Repeat("a", 65)}, "A project's folder name is at most 64 characters."},
		{map[string]any{"name": "a/b"}, plain}, {map[string]any{"name": "../up"}, plain}, {map[string]any{"name": "/abs"}, plain},
		{map[string]any{"name": `a\b`}, plain}, {map[string]any{"name": "two words"}, plain}, {map[string]any{"name": "tab\there"}, plain},
		{map[string]any{"name": "new\nline"}, plain}, {map[string]any{"name": "café"}, plain}, {map[string]any{"name": "a:b"}, plain},
		{map[string]any{"name": "~home"}, plain}, {map[string]any{"name": "nul\x00"}, plain},
		{map[string]any{"name": "."}, dots}, {map[string]any{"name": ".."}, dots}, {map[string]any{"name": ".hidden"}, dots},
		{map[string]any{"name": "trailing."}, dots},
		{map[string]any{"name": "app-hires"}, "A name ending in \"-hires\" is kept for a project's hires' worktrees: pick another."},
		{map[string]any{"name": "App-HIRES"}, "A name ending in \"-hires\" is kept for a project's hires' worktrees: pick another."},
		{map[string]any{"name": "ok", "folder": "/elsewhere"}, "A new project takes a name and, for a clone, an address: nothing else."},
	} {
		if code, text := askProject(t, d, callerGM, gm, "", c.body); code != http.StatusBadRequest || text != c.want {
			t.Errorf("%v: %d %q, want 400 %q", c.body, code, text, c.want)
		}
	}
	projectsHold(t, projects, "app")
	// At the edge: 64 plain characters, digits, "-", "_" and an inner dot are made.
	long := strings.Repeat("b", 64)
	for _, name := range []string{long, "v2.site_new-1"} {
		if code, text := askProject(t, d, callerGM, gm, "", map[string]any{"name": name}); code != http.StatusOK {
			t.Errorf("%q: %d %q", name, code, text)
		}
	}
	projectsHold(t, projects, "app", long, "v2.site_new-1")
	if lines := projectLines(d); len(lines) != 2 || lines[0].Detail != "The General Manager made the project folder "+long+"." ||
		lines[0].Folder != filepath.Join(projects, long) {
		t.Errorf("the office's lines: %+v", lines)
	}
}

// A name already there, as a folder, a file, or a link (to a folder or to nothing), is refused with a plain reason,
// in any case on a disk that matches names in any case, and nothing is followed, made or replaced.
func TestANewProjectNeverReplacesOrFollowsWhatIsAtItsName(t *testing.T) {
	d, projects, gm, _ := projectDeck(t)
	elsewhere := filepath.Join(physical(homeDir), "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projects, "notes"), []byte("the owner's\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, to := range map[string]string{"linked": elsewhere, "dangling": filepath.Join(physical(homeDir), "missing")} {
		if err := os.Symlink(to, filepath.Join(projects, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"app", "notes", "linked", "dangling"} {
		want := "There's already something called " + name + " in " + projects + ": pick another name."
		if code, text := askProject(t, d, callerGM, gm, "", map[string]any{"name": name}); code != http.StatusConflict || text != want {
			t.Errorf("%s: %d %q, want 409 %q", name, code, text, want)
		}
	}
	// A clone at a link's name is refused before anything is cloned through it.
	if code, _ := askProject(t, d, callerGM, gm, "", map[string]any{"name": "linked", "clone": "https://127.0.0.1:9/x.git"}); code != http.StatusConflict {
		t.Errorf("a clone at the link's name: %d", code)
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Errorf("something was made through the link: %v", entries)
	}
	if _, err := os.Lstat(filepath.Join(physical(homeDir), "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Error("something was made where the dangling link points")
	}
	if data, _ := os.ReadFile(filepath.Join(projects, "notes")); string(data) != "the owner's\n" {
		t.Errorf("the owner's file is now %q", data)
	}
	if target, _ := os.Readlink(filepath.Join(projects, "linked")); target != elsewhere {
		t.Errorf("the link now points at %q", target)
	}
	// On a disk that matches names in any case (a Mac's), "APP" is "app": refused too, never a second folder.
	if probe := filepath.Join(projects, "APP"); func() bool { _, err := os.Stat(probe); return err == nil }() {
		if code, _ := askProject(t, d, callerGM, gm, "", map[string]any{"name": "APP"}); code != http.StatusConflict {
			t.Errorf("APP beside app: %d", code)
		}
	}
	projectsHold(t, projects, "app", "dangling", "linked", "notes")
	if lines := projectLines(d); len(lines) != 0 {
		t.Errorf("a refusal wrote the office's line: %+v", lines)
	}
}

// A clone of the https stand-in lands in the new folder, its own repo, and the office says so.
func TestANewProjectCanBeACloneOfAnHTTPSAddress(t *testing.T) {
	d, projects, gm, _ := projectDeck(t)
	address := httpsGitStandIn(t) + "/site.git"
	code, text := askProject(t, d, callerGM, gm, "", map[string]any{"name": "site", "clone": address})
	folder := filepath.Join(projects, "site")
	var answer map[string]any
	_ = json.Unmarshal([]byte(text), &answer)
	if code != http.StatusOK || answer["folder"] != folder || answer["cloned"] != address || answer["ok"] != true {
		t.Fatalf("the clone: %d %s", code, text)
	}
	if data, err := os.ReadFile(filepath.Join(folder, "README")); err != nil || string(data) != "a stand-in project\n" {
		t.Errorf("the clone's README: %q %v", data, err)
	}
	if info, err := os.Lstat(filepath.Join(folder, ".git")); err != nil || !info.IsDir() {
		t.Errorf("the clone is no repo of its own: %v", err)
	}
	if lines := projectLines(d); len(lines) != 1 || lines[0].Detail != "The General Manager made the project folder site, a clone of "+address+"." ||
		lines[0].Folder != folder {
		t.Errorf("the office's lines: %+v", lines)
	}
}

// The clone runs no hook, whatever the core's own environment says: not one from a global config, git's config
// parameters or a template folder, and the new repo gets no hooks to run later either.
func TestANewProjectsCloneRunsNoHooks(t *testing.T) {
	d, projects, gm, _ := projectDeck(t)
	address := httpsGitStandIn(t) + "/site.git"
	planted := filepath.Join(t.TempDir(), "ran")
	hooks, templates := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(templates, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho ran >> '" + planted + "'\n"
	for _, dir := range []string{hooks, filepath.Join(templates, "hooks")} {
		for _, name := range []string{"post-checkout", "reference-transaction", "post-index-change"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[core]\n\thooksPath = "+hooks+"\n[init]\n\ttemplateDir = "+templates+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_TEMPLATE_DIR", templates)
	t.Setenv("GIT_CONFIG_PARAMETERS", "'core.hooksPath'='"+hooks+"'")
	if code, text := askProject(t, d, callerGM, gm, "", map[string]any{"name": "site", "clone": address}); code != http.StatusOK {
		t.Fatalf("the clone: %d %s", code, text)
	}
	if _, err := os.Lstat(planted); !errors.Is(err, os.ErrNotExist) {
		data, _ := os.ReadFile(planted)
		t.Errorf("the clone ran a hook: %q", data)
	}
	if entries, _ := os.ReadDir(filepath.Join(projects, "site", ".git", "hooks")); len(entries) != 0 {
		t.Errorf("the new repo has hooks: %v", entries)
	}
	if data, _ := os.ReadFile(filepath.Join(projects, "site", "README")); string(data) != "a stand-in project\n" {
		t.Errorf("the clone's README: %q", data)
	}
}

// Only an https address is taken; a file:// one, which the route refuses, is refused by git's allow-list too when
// handed to the clone itself. A clone that fails leaves nothing behind, and the office writes no line.
func TestANewProjectClonesOnlyOverHTTPSAndTakesAFailedOneAway(t *testing.T) {
	d, projects, gm, _ := projectDeck(t)
	base := httpsGitStandIn(t)
	local := t.TempDir()
	standInGit(t, local, "init", "-q")
	const only = "Smolcorp clones only an https:// address, such as https://github.com/owner/project.git."
	for address, want := range map[string]string{
		"file://" + local: only, local: only, "http://127.0.0.1:9/x.git": only, "ssh://git@example.invalid/x.git": only,
		"git@example.invalid:x.git": only, "ext::sh -c true": only, "https://": only, "https:///x.git": only,
		"HTTPS://example.invalid/x.git?token=1": only, "https://example.invalid/x.git#main": only, "https://example.invalid/x .git": only,
		" https://example.invalid/x.git": only, "https://ada:secret@example.invalid/x.git": "The address has a name or a password in it, which the new repo would keep: give it without them.",
	} {
		if code, text := askProject(t, d, callerGM, gm, "", map[string]any{"name": "site", "clone": address}); code != http.StatusBadRequest || text != want {
			t.Errorf("%q: %d %q, want 400 %q", address, code, text, want)
		}
	}
	if code, text := askProject(t, d, callerGM, gm, "", map[string]any{"name": "site", "clone": 7}); code != http.StatusBadRequest || text != "The address to clone is text." {
		t.Errorf("a number to clone: %d %q", code, text)
	}
	// git's own allow-list: https alone, whatever the address.
	into := filepath.Join(t.TempDir(), "into")
	if err := os.Mkdir(into, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := gmProjectClone(t.Context(), into, "file://"+local); err == nil {
		t.Error("git cloned a file:// address")
	}
	if _, err := os.Lstat(filepath.Join(into, ".git")); err == nil {
		t.Error("a file:// clone left a repo")
	}
	// A clone that fails (no such repo on the stand-in) leaves nothing in the projects folder, nor in staging.
	want := "Git couldn't clone " + base + "/missing.git, so Smolcorp made nothing in " + projects + ". Check the address, and that it needs no sign-in."
	if code, text := askProject(t, d, callerGM, gm, "", map[string]any{"name": "site", "clone": base + "/missing.git"}); code != http.StatusBadGateway || text != want {
		t.Errorf("a failed clone: %d %q", code, text)
	}
	// …and one stopped at its time limit too.
	saved := gmProjectCloneMost
	gmProjectCloneMost = time.Nanosecond
	t.Cleanup(func() { gmProjectCloneMost = saved })
	if code, _ := askProject(t, d, callerGM, gm, "", map[string]any{"name": "site", "clone": base + "/site.git"}); code != http.StatusBadGateway {
		t.Errorf("a clone past its time: %d", code)
	}
	projectsHold(t, projects, "app")
	if entries, _ := os.ReadDir(filepath.Join(supportDir, gmProjectStagingName)); len(entries) != 0 {
		t.Errorf("a failed clone left %v in staging", entries)
	}
	if lines := projectLines(d); len(lines) != 0 {
		t.Errorf("a failed clone wrote the office's line: %+v", lines)
	}
}

// Who may ask: the GM and the page. A hire's key, a tool's key, no key and a paired phone (on the Wi-Fi or here) are
// refused at the route table; an MCP key and a night job, which have the GM's column, at the gate; a GM held to one
// project by its scope. Off while hiring is off, and while the manager is off. Nothing is made by any refusal.
func TestOnlyTheGMAndThePageStartAProject(t *testing.T) {
	d, projects, gm, hireKey := projectDeck(t)
	keepNightJob(t, d, NightJob{ID: "nightly-tests", Status: nightApproved, ProposedBy: "page"})
	for who, want := range map[caller]int{callerHire: http.StatusForbidden, callerTool: http.StatusForbidden, callerNobody: http.StatusUnauthorized} {
		if routeAllows(who, "POST", gmProjectPath) {
			t.Errorf("the table lets in %s", who.name())
		}
		if code, text := askProject(t, d, who, gm, hireKey, map[string]any{"name": "by-" + who.name()}); code != want {
			t.Errorf("as %s: %d %q, want %d", who.name(), code, text, want)
		}
	}
	d.mu.Lock()
	phone := d.access.Phones.Pair("Phone", true)
	d.mu.Unlock()
	for _, c := range []call{{host: deckOnWiFi, from: aPhone}, {host: "localhost:7710", from: thisComputer}} {
		c.method, c.target, c.body, c.withoutSetupKey = "POST", gmProjectPath+"?t="+phone, `{"name":"by-phone"}`, true
		c.headers = map[string]string{setupHeader: "1", hookKeyHeader: phone}
		if got := serve(t, d, c); got.Code == http.StatusOK || got.Code < 400 {
			t.Errorf("a phone from %s: %d", c.from, got.Code)
		}
	}
	for _, a := range []actor{mcpActor("claude-desktop"), nightActor("nightly-tests")} {
		if v := d.decide(a, action{route: &routeAsk{method: "POST", path: gmProjectPath}}); v.allowed() || v.status != http.StatusForbidden || v.why != gmProjectRefusal {
			t.Errorf("%s: %+v", a, v)
		}
	}
	projectsHold(t, projects, "app")
	// The page and the GM.
	if code, text := askProject(t, d, callerPage, gm, "", map[string]any{"name": "by-page"}); code != http.StatusOK {
		t.Errorf("the page: %d %q", code, text)
	}
	if code, text := askProject(t, d, callerGM, gm, "", map[string]any{"name": "by-gm"}); code != http.StatusOK {
		t.Errorf("the GM: %d %q", code, text)
	}
	if lines := projectLines(d); len(lines) != 2 || lines[0].Detail != "Setup made the project folder by-page." ||
		lines[1].Detail != "The General Manager made the project folder by-gm." {
		t.Errorf("the office's lines: %+v", lines)
	}
	// Hiring off, then the manager off.
	d.mu.Lock()
	d.config.HiringEnabled = false
	d.mu.Unlock()
	if code, text := askProject(t, d, callerGM, gm, "", map[string]any{"name": "hiring-off"}); code != http.StatusForbidden || text != hiringOff {
		t.Errorf("hiring off: %d %q", code, text)
	}
	d.mu.Lock()
	d.config.HiringEnabled, d.config.ManagerEnabled = true, false
	d.mu.Unlock()
	if code, text := askProject(t, d, callerGM, gm, "", map[string]any{"name": "manager-off"}); code != http.StatusConflict || text != managerOffAnswer {
		t.Errorf("the manager off: %d %q", code, text)
	}
	managerOn(d)
	// A GM held to one project starts none.
	writeRegistryHires(t, Hire{ID: "gm1", Name: "General Manager", Role: gmRole, Folder: keptPath(projects), State: "working", Started: 1,
		Scope: filepath.Join(projects, "app")})
	v := d.decide(actorGM, action{route: &routeAsk{method: "POST", path: gmProjectPath, body: strings.NewReader(`{"name":"held"}`)}})
	if want := "This General Manager works only in " + filepath.Join(projects, "app") + ": it doesn't start new project folders."; v.allowed() || v.status != http.StatusForbidden || v.why != want {
		t.Errorf("a GM held to one project: %+v", v)
	}
	projectsHold(t, projects, "app", "by-gm", "by-page")
}

// A GM in a repo (its own worktree or a checkout) has no projects folder, and neither has an office with no GM.
func TestANewProjectNeedsAGMOverAProjectsFolder(t *testing.T) {
	d, projects, gm, _ := projectDeck(t)
	app := filepath.Join(projects, "app")
	writeRegistryHires(t, Hire{ID: "gm1", Name: "General Manager", Role: gmRole, Folder: keptPath(app), Repo: app, State: "working", Started: 1})
	want := "The General Manager works in a repository, not over a projects folder, so it can't start a project here."
	if code, text := askProject(t, d, callerGM, gm, "", map[string]any{"name": "x"}); code != http.StatusConflict || text != want {
		t.Errorf("a GM in a repo: %d %q", code, text)
	}
	writeRegistryHires(t, Hire{ID: "gm1", Name: "General Manager", Role: gmRole, Folder: keptPath(projects), State: "fired", Started: 1})
	want = "There's no General Manager now, so there's no projects folder to start a project in."
	if code, text := askProject(t, d, callerPage, gm, "", map[string]any{"name": "x"}); code != http.StatusConflict || text != want {
		t.Errorf("no GM: %d %q", code, text)
	}
	projectsHold(t, projects, "app")
}

// The GM's start words say how to start a project, once.
func TestTheGMIsToldHowToStartAProject(t *testing.T) {
	intro := gmIntroduction("/Users/ada/Projects", "/s/hires/gm1/work", "/Users/ada/Projects/.smolcorp/plans/gm1", false)
	if strings.Count(intro, "To start a new project, ask Smolcorp: `smolcorp gm project new <name>`") != 1 {
		t.Errorf("the start words: %q", intro)
	}
	if strings.Contains(gmIntroduction("/Users/ada/Projects", "", "", false), "project new") {
		t.Error("a GM in a repo is told to start projects")
	}
}

// FXMAKEPROJECTR1: the link check passes over a support folder's project-staging folder (a clone there can be a big
// tree) and nothing else: not the support folder, a folder in staging, nor a folder of that name elsewhere.
func TestTheLinkCheckPassesOverTheProjectStagingFolderAlone(t *testing.T) {
	base := t.TempDir()
	support, other := filepath.Join(base, "support"), filepath.Join(base, "test-copy")
	for folder, want := range map[string]bool{
		filepath.Join(support, gmProjectStagingName): true, filepath.Join(other, gmProjectStagingName) + "/": true,
		support: false, filepath.Join(support, gmProjectStagingName, "clone-1"): false,
		filepath.Join(support, "hires", gmProjectStagingName): false, filepath.Join(base, gmProjectStagingName): false,
	} {
		if got := gmProjectStagingSkipped([]string{support, other, ""}, folder); got != want {
			t.Errorf("%s: %v, want %v", folder, got, want)
		}
	}
}

// RT126FIXREVIEW-MAKEPROJECTR1 O2: what a clone left in <support>/project-staging when Smolcorp stopped in the middle
// of it is taken away at the next start: each clone-* folder, nothing else there. Each start (the app's, smolcorp
// serve's and the Mac app's engine) does it once it holds the instance lock, before the deck is made.
func TestAStartTakesAwayWhatAStoppedCloneLeft(t *testing.T) {
	root := filepath.Join(supportDir, gmProjectStagingName)
	left := filepath.Join(root, "clone-1234", "site", ".git")
	if err := os.MkdirAll(left, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(left, "config"), []byte("[core]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "not-a-clone")
	if err := os.WriteFile(other, []byte("kept"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	clearProjectStaging()
	if _, err := os.Lstat(filepath.Join(root, "clone-1234")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the stopped clone is still there: %v", err)
	}
	if _, err := os.Lstat(other); err != nil {
		t.Errorf("something that isn't a clone went too: %v", err)
	}
	for file, entries := range map[string][]string{
		"app.go":         {"func RunApp() int {", "func RunServer() int {"},
		"core_darwin.go": {"func RunCore(args []string, in io.Reader, out, errs io.Writer) int {"},
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			body, _, _ := strings.Cut(string(data)[max(strings.Index(string(data), entry), 0):], "\n}\n")
			lock := max(strings.Index(body, "acquire()"), strings.Index(body, "appHoldsInstance()"))
			cleared, made := strings.Index(body, "\tclearProjectStaging()\n"), strings.Index(body, "NewDeck()")
			if !strings.HasPrefix(body, entry) || lock < 0 || cleared < lock || made < cleared {
				t.Errorf("%s: %s doesn't clear the staging folder after the instance lock and before the deck", file, entry)
			}
		}
	}
}

// RT126FIXREVIEW-MAKEPROJECTR1 O3: a projects folder on another disk than Smolcorp's own folder is refused before the
// clone starts, in the words the move in would give after it: git is never asked, and nothing is left behind. The
// look itself compares the disks: one folder's with another's on the same disk, and (not on Windows, which leaves it
// to the move) with /dev's, another file system.
func TestACloneOntoAnotherDiskIsRefusedBeforeItRuns(t *testing.T) {
	d, projects, gm, _ := projectDeck(t)
	base, held, _ := heldGitStandIn(t, true)
	saved := gmProjectOneDisk
	gmProjectOneDisk = func(*projectsPlace, string) (bool, error) { return false, nil }
	t.Cleanup(func() { gmProjectOneDisk = saved })
	code, text := askProject(t, d, callerGM, gm, "", map[string]any{"name": "site", "clone": base + "/site.git"})
	want := "Smolcorp clones in its own folder first, then moves the clone in, and " + projects +
		" is on another disk, so it can't: ask the owner to clone it there."
	if code != http.StatusConflict || text != want {
		t.Errorf("a clone onto another disk: %d %q\nwant 409 %q", code, text, want)
	}
	select {
	case <-held:
		t.Error("git was asked for the clone all the same")
	default:
	}
	projectsHold(t, projects, "app")
	if entries, _ := os.ReadDir(filepath.Join(supportDir, gmProjectStagingName)); len(entries) != 0 {
		t.Errorf("the refused clone left %v in staging", entries)
	}
	if lines := projectLines(d); len(lines) != 0 {
		t.Errorf("the refused clone wrote the office's line: %+v", lines)
	}

	place, err := openProjectsPlace(projects)
	if err != nil {
		t.Fatal(err)
	}
	defer place.close()
	if same, err := saved(place, filepath.Join(projects, "app")); !same || err != nil {
		t.Errorf("a folder inside the projects folder: same disk %v, %v", same, err)
	}
	if runtime.GOOS != "windows" {
		if same, err := saved(place, "/dev"); same || err != nil {
			t.Errorf("/dev: same disk %v, %v", same, err)
		}
	}
}
