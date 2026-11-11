//go:build !windows

package desktop

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// The launcher engine's tests (T734): a fake launcher stream through httptest
// stands in for `smolcorp run` in a window; nothing real is started. Git, the
// guard and the opener are stand-ins, as the tmux engine's tests have.

// winLauncher is one hire's launcher as the app sees it: it asked for its
// environment, connected with the hire's token, answers looks from its screen
// and, when told to stop, goes (or doesn't, stopEnds false).
type winLauncher struct {
	id     string
	tab    *launcherTab
	screen *launcherScreen
	env    hireStartEnv

	mu     sync.Mutex
	writes []string
	stops  int
}

func (w *winLauncher) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writes = append(w.writes, string(p))
	return len(p), nil
}

func (w *winLauncher) typed() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.writes)
}
func (w *winLauncher) stopped() int { w.mu.Lock(); defer w.mu.Unlock(); return w.stops }

type lrig struct {
	t      *testing.T
	d      *Deck
	repo   string
	srv    *httptest.Server
	o      launcherEngineOptions
	opened [][]string
	// What the fake window does when it is opened: start a launcher showing this, or nothing.
	launches    bool
	shows       string
	stopEnds    bool
	window      map[string]*winLauncher // by hire id
	trace       []string
	afterAnswer func(n int)
	answers     int
	answerMu    sync.Mutex
}

func newLRig(t *testing.T) *lrig {
	t.Helper()
	d := newTestDeck(t)
	home := homeDir
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "no-gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(home, "no-gitconfig"))
	writeConfig(t, home, "{}")
	r := &lrig{t: t, d: d, repo: placeRepo(t), launches: true, shows: "idle-empty", stopEnds: true, window: map[string]*winLauncher{}}
	router := &router{deck: d}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if q.URL.Path == screenAnswerPath {
			// The answer's body is read already: a change made here is for the next look.
			r.answerMu.Lock()
			r.answers++
			n := r.answers
			r.answerMu.Unlock()
			if r.afterAnswer != nil {
				r.afterAnswer(n)
			}
		}
		router.ServeHTTP(w, q)
	}))
	t.Cleanup(r.srv.Close)
	r.o = launcherEngineOptions{
		Worktrees: t.TempDir(), PortFree: func(int) bool { return true }, Key: func() string { return placeTestKey },
		Guard:  func(string, []string) (int, string, bool) { return 0, engineGuardPrinted, true },
		IsExec: func(string) bool { return true }, CLI: filepath.Join(home, "bin", "smolcorp"),
		Expect: func(token, hireID string, env hireStartEnv) {
			r.trace = append(r.trace, "expect")
			d.expectLauncher(token, hireID, env)
		},
		Forget:   func(token string) { r.trace = append(r.trace, "forget"); d.forgetLauncher(token) },
		Open:     r.open,
		StopWait: 300 * time.Millisecond, LookWait: 2 * time.Second,
	}
	d.config.Roles = []Role{{ID: "janitor", Name: "Janitor", Agent: "claude", Model: "sonnet"}, {ID: "gm", Name: "General Manager", Agent: "claude", Model: "sonnet"}}
	d.config.HireManagedWindows = true
	return r
}

func (r *lrig) request(roleID string) engineRequest {
	i := slices.IndexFunc(r.d.config.Roles, func(x Role) bool { return x.ID == roleID })
	return engineRequest{Role: r.d.config.Roles[i], How: hiredIntoGeneralFolder, Folder: r.repo, DefaultAccount: true,
		GMToken: r.d.newGMToken}
}

// open is the fake window: it records the command and, as the launcher does, asks for the environment, then connects.
func (r *lrig) open(command []string, folder string) error {
	r.trace = append(r.trace, "open")
	r.opened = append(r.opened, slices.Clone(command))
	if !r.launches {
		return nil
	}
	r.startWindow(command, folder)
	return nil
}

// commandWords are the words of a launcher command, also when the terminal's login shell
// (on a Mac or Linux) holds them as one quoted line.
func commandWords(command []string) []string {
	var words []string
	for _, arg := range command {
		if strings.HasPrefix(arg, "exec '") {
			for _, w := range strings.Split(strings.TrimPrefix(arg, "exec "), "' '") {
				words = append(words, strings.Trim(w, "'"))
			}
		} else {
			words = append(words, arg)
		}
	}
	return words
}

// flagAfter is the word after flag in a launcher command.
func flagAfter(command []string, flag string) string {
	command = commandWords(command)
	if i := slices.Index(command, flag); i >= 0 && i+1 < len(command) {
		return command[i+1]
	}
	return ""
}

func (r *lrig) startWindow(command []string, folder string) *winLauncher {
	r.t.Helper()
	token, id := flagAfter(command, "--hire"), flagAfter(command, "--hire-id")
	env, err := fetchHireStartEnv(r.srv.Client(), r.srv.URL, r.d.access.HookKey, token, id)
	if err != nil {
		r.t.Fatalf("the launcher's environment: %v", err)
	}
	return r.connect("L-"+id, folder, flagAfter(command, "--role"), token, id, env)
}

func (r *lrig) connect(launcherID, folder, role, token, hireID string, env hireStartEnv) *winLauncher {
	r.t.Helper()
	d := r.d
	tab := d.launcherConnected(launcherID, Claude, folder, role, token)
	if tab == nil {
		r.t.Fatal("launcher not connected")
	}
	w := &winLauncher{id: launcherID, tab: tab, screen: newLauncherScreen(60, 200, true), env: env}
	w.screen.Write([]byte("\x1b[?2004h\x1b[2J\x1b[H" + ptyOutput(pane(r.t, r.shows))))
	pipe, feed := io.Pipe()
	r.t.Cleanup(func() { feed.Close() })
	go func() {
		encoder := json.NewEncoder(feed)
		encoder.Encode(launcherCommand{Secret: tab.screenSecret})
		for {
			select {
			case command := <-tab.commands:
				if encoder.Encode(command) != nil {
					return
				}
			case <-tab.gone:
				return
			}
		}
	}()
	cursor := &cursorMode{bracketed: w.screen.Bracketed}
	go follow(pipe, w, cursor, func(proof, number string) {
		postScreenAnswer(r.srv.Client(), r.srv.URL, d.access.HookKey, proof, launcherID, number, w.screen)
	}, func() {
		w.mu.Lock()
		w.stops++
		w.mu.Unlock()
		if r.stopEnds {
			d.launcherGone(launcherID, tab)
		}
	})
	if hireID != "" {
		r.window[hireID] = w
	}
	return w
}

// hire makes one engine hire and, unless the window doesn't launch, connects its launcher.
func (r *lrig) hire(roleID string) (Hire, *winLauncher) {
	r.t.Helper()
	hire, _, err := runLauncherHire(r.request(roleID), r.o)
	if err != nil {
		r.t.Fatal(err)
	}
	return hire, r.window[hire.ID]
}

func (r *lrig) worktrees() string {
	out, _ := exec.Command("git", "-C", r.repo, "worktree", "list").Output()
	return strings.TrimSpace(string(out))
}

// nothingLeft is that no hire, key, worktree, pending token or settings folder was left behind.
func (r *lrig) nothingLeft() {
	r.t.Helper()
	file := readRegistry(registryPath(), false)
	if len(file.hires) != 0 || len(file.keys) != 0 {
		r.t.Errorf("registry left: %+v %v", file.hires, file.keys)
	}
	if lines := strings.Split(r.worktrees(), "\n"); len(lines) != 1 {
		r.t.Errorf("worktrees left: %v", lines)
	}
	if left, _ := os.ReadDir(filepath.Join(supportDir, "hires")); len(left) != 0 {
		r.t.Errorf("hire folders left: %v", left)
	}
	r.d.mu.Lock()
	pending := len(r.d.hires)
	r.d.mu.Unlock()
	if pending != 0 {
		r.t.Errorf("%d tokens left", pending)
	}
}

func TestTheSwitchOffOrNotWindowsOpensAHireAsItDid(t *testing.T) {
	saved := launcherHires
	t.Cleanup(func() { launcherHires = saved })
	for name, c := range map[string]struct{ launcher, on bool }{"switch off": {true, false}, "not windows": {false, true}} {
		t.Run(name, func(t *testing.T) {
			launcherHires = c.launcher
			r := newLRig(t)
			h := fakeHiring(t, "")
			r.d.config.HiringEnabled, r.d.config.HireFolder = true, r.repo
			r.d.config.HireManagedWindows = c.on
			r.d.launcherEngine = r.o
			if status, message := r.d.hireFromSetup("janitor"); status != http.StatusNoContent {
				t.Fatalf("hire: %d %s", status, message)
			}
			if words := commandWords(h.windows[0]); len(h.windows) != 1 || !slices.Contains(words, "--hire") || slices.Contains(words, "--hire-id") {
				t.Errorf("window: %q", h.windows)
			}
			if len(readRegistry(registryPath(), false).hires) != 0 || len(r.opened) != 0 || len(r.trace) != 0 {
				t.Error("the engine made a hire with its switch off")
			}
		})
	}
}

func TestTheSwitchOnMakesTheHireWithTheEngineAndNoSecretIsInTheOpenersArgumentsOrEnvironmentOrAFile(t *testing.T) {
	saved := launcherHires
	t.Cleanup(func() { launcherHires = saved })
	launcherHires = true
	r := newLRig(t)
	h := fakeHiring(t, "")
	var env [][]string
	startHire = func(cmd *exec.Cmd) error {
		h.windows, h.dirs = append(h.windows, cmd.Args), append(h.dirs, cmd.Dir)
		env = append(env, cmd.Env)
		return nil
	}
	t.Setenv("SMOLCORP_GM_TOKEN", "inherited-gm-token")
	r.d.config.HiringEnabled, r.d.config.HireFolder = true, r.repo
	o := r.o
	o.Open = nil // the real opener, through the terminal's own arguments
	// No launcher connects here, so the hire is heard from only once the test is done: its watch ends then,
	// in this test, and never writes the registry in a later test's folder.
	done := make(chan struct{})
	o.HeardFrom = func(Hire) bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}
	o.Every = 5 * time.Millisecond
	t.Cleanup(func() { close(done); r.d.watching.Wait() })
	r.d.launcherEngine = o
	if status, message := r.d.hireFromSetup("janitor"); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	hires := readRegistry(registryPath(), false).hires
	if len(hires) != 1 || !hires[0].ViaLauncher || hires[0].TmuxSocket != "" || hires[0].State != "starting" {
		t.Fatalf("registry: %+v", hires)
	}
	if len(h.windows) != 1 {
		t.Fatalf("windows: %q", h.windows)
	}
	args := commandWords(h.windows[0])
	token := flagAfter(args, "--hire")
	if !hireToken.MatchString(token) || flagAfter(args, "--hire-id") != hires[0].ID || flagAfter(args, "--role") != "janitor" ||
		!slices.Contains(args, "--settings="+filepath.Join(hireFolder(supportDir, hires[0].ID), hireSettingsFile)) {
		t.Errorf("command: %q", args)
	}
	// The launcher asks for its environment: the secrets are there, and only there.
	got, err := fetchHireStartEnv(r.srv.Client(), r.srv.URL, r.d.access.HookKey, token, hires[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	key := got.Set[hireKeyEnvironment]
	if key == "" || got.Set["SMOLCORP_HIRE_PORTS"] != hires[0].Ports || got.Set["SMOLCORP_GM_TOKEN"] != "" ||
		!slices.Contains(got.Unset, "SMOLCORP_GM_TOKEN") || !slices.Contains(got.Unset, "CLAUDE_CONFIG_DIR") {
		t.Errorf("environment: %+v", got)
	}
	if found := readRegistry(registryPath(), false).hireForSendKey(key); found == nil || found.ID != hires[0].ID {
		t.Errorf("the key isn't the hire's: %+v", found)
	}
	secrets := []string{key, hires[0].Ports, "inherited-gm-token"}
	for _, secret := range secrets {
		for _, arg := range h.windows[0] {
			if strings.Contains(arg, secret) {
				t.Errorf("%q is on the opener's command line: %q", secret, arg)
			}
		}
		for _, e := range env[0] {
			if strings.Contains(e, secret) {
				t.Errorf("%q is in the window's environment: %q", secret, e)
			}
		}
	}
	_ = filepath.WalkDir(supportDir, func(path string, entry os.DirEntry, _ error) error {
		if data, err := os.ReadFile(path); err == nil && !entry.IsDir() {
			for _, secret := range secrets[:1] {
				if strings.Contains(string(data), secret) {
					t.Errorf("the send key is in a file: %s", path)
				}
			}
		}
		return nil
	})
	if files, _ := filepath.Glob(filepath.Join(hireFolder(supportDir, hires[0].ID), "*.sh")); len(files) != 0 {
		t.Errorf("a start script was written: %v", files)
	}
	if h.dirs[0] != hires[0].Worktree {
		t.Errorf("the window opened in %q, not the worktree %q", h.dirs[0], hires[0].Worktree)
	}
}

func TestAStartIsMadeInOrderEachReadyBeforeTheNext(t *testing.T) {
	r := newLRig(t)
	r.launches = false
	r.o.Open = func(command []string, folder string) error {
		r.trace = append(r.trace, "open")
		hires := readRegistry(registryPath(), false)
		if len(hires.hires) != 1 || hires.hires[0].State != "starting" || hires.hires[0].TmuxSocket != "" || hires.keys[hires.hires[0].ID] == "" {
			t.Errorf("at the window's opening the row or its key wasn't ready: %+v %v", hires.hires, hires.keys)
		}
		if _, err := os.Stat(filepath.Join(hireFolder(supportDir, hires.hires[0].ID), hireSettingsFile)); err != nil {
			t.Errorf("no settings when the window opened: %v", err)
		}
		if len(strings.Split(r.worktrees(), "\n")) != 2 {
			t.Errorf("no worktree when the window opened: %s", r.worktrees())
		}
		return nil
	}
	if _, _, err := runLauncherHire(r.request("janitor"), r.o); err != nil {
		t.Fatal(err)
	}
	if strings.Join(r.trace, ",") != "expect,open" {
		t.Errorf("order: %v", r.trace)
	}
}

func TestEachRefusalOfAStartLeavesNothingBehind(t *testing.T) {
	cases := map[string]func(r *lrig, req *engineRequest){
		"test copy": func(r *lrig, req *engineRequest) { req.TestPort = 7999 },
		"no ports":  func(r *lrig, req *engineRequest) { r.o.PortFree = func(int) bool { return false } },
		"guard": func(r *lrig, req *engineRequest) {
			r.o.Guard = func(string, []string) (int, string, bool) { return 1, "", false }
		},
		"no GM token": func(r *lrig, req *engineRequest) {
			req.Role.ID, req.GMToken = gmRole, func() string { return "" }
		},
		"api key": func(r *lrig, req *engineRequest) { req.APIKey = true },
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			r := newLRig(t)
			req := r.request("janitor")
			spoil(r, &req)
			if _, _, err := runLauncherHire(req, r.o); err == nil {
				t.Fatal("started")
			}
			if slices.Contains(r.trace, "open") {
				t.Errorf("a window was opened: %v", r.trace)
			}
			r.nothingLeft()
		})
	}
	t.Run("window won't open", func(t *testing.T) {
		r := newLRig(t)
		r.o.Open = func([]string, string) error { r.trace = append(r.trace, "open"); return os.ErrPermission }
		if _, _, err := runLauncherHire(r.request("janitor"), r.o); err == nil {
			t.Fatal("started")
		}
		// One failed row stays, whose key works no more; everything else is gone.
		file := readRegistry(registryPath(), false)
		if len(file.hires) != 1 || file.hires[0].State != "failed" || len(strings.Split(r.worktrees(), "\n")) != 1 {
			t.Errorf("left: %+v / %s", file.hires, r.worktrees())
		}
		if _, ok := file.keys[file.hires[0].ID]; !ok || file.hireForSendKey("x") != nil {
			t.Log("the key's hash stays against a failed row")
		}
		if r.d.hires == nil || len(r.d.hires) != 0 {
			t.Errorf("token left: %v", r.d.hires)
		}
	})
}

func TestTheGMGetsTheTokenAndNoKeyOthersAKeyAndNoToken(t *testing.T) {
	r := newLRig(t)
	gm, gmWindow := r.hire("gm")
	other, otherWindow := r.hire("janitor")
	if gmWindow.env.Set["SMOLCORP_GM_TOKEN"] == "" || !r.d.isGMToken(gmWindow.env.Set["SMOLCORP_GM_TOKEN"]) ||
		gmWindow.env.Set[hireKeyEnvironment] != "" || slices.Contains(gmWindow.env.Unset, "SMOLCORP_GM_TOKEN") {
		t.Errorf("the GM's environment: %+v", gmWindow.env)
	}
	if otherWindow.env.Set[hireKeyEnvironment] == "" || otherWindow.env.Set["SMOLCORP_GM_TOKEN"] != "" ||
		!slices.Contains(otherWindow.env.Unset, "SMOLCORP_GM_TOKEN") {
		t.Errorf("the other hire's environment: %+v", otherWindow.env)
	}
	if file := readRegistry(registryPath(), false); file.keys[gm.ID] != "" || file.keys[other.ID] == "" {
		t.Errorf("keys: %v", file.keys)
	}
	// What the launcher makes of it: set on the agent's process, the inherited one taken out.
	base := []string{"PATH=/usr/bin", "SMOLCORP_GM_TOKEN=inherited", "CLAUDE_CONFIG_DIR=/other", "KEEP=1"}
	got := startEnvironment(base, otherWindow.env, false)
	if slices.ContainsFunc(got, func(e string) bool {
		return strings.HasPrefix(e, "SMOLCORP_GM_TOKEN=") || strings.HasPrefix(e, "CLAUDE_CONFIG_DIR=")
	}) ||
		!slices.Contains(got, "KEEP=1") || !slices.Contains(got, hireKeyEnvironment+"="+otherWindow.env.Set[hireKeyEnvironment]) ||
		!strings.HasPrefix(got[0], "PATH="+filepath.Dir(r.o.CLI)+string(os.PathListSeparator)) {
		t.Errorf("environment: %v", got)
	}
	if slices.Contains(base, hireKeyEnvironment) || base[1] != "SMOLCORP_GM_TOKEN=inherited" {
		t.Error("the inherited environment was changed")
	}
}

func TestTheStartEnvironmentRouteRefusesAWrongUsedOrExpiredToken(t *testing.T) {
	r := newLRig(t)
	r.launches = false
	hire, token, err := runLauncherHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	ask := func(token, id, key string) int {
		request, _ := http.NewRequest(http.MethodGet, r.srv.URL+hireStartPath+"?hire="+token+"&id="+id, nil)
		if key != "" {
			request.Header.Set(hookKeyHeader, key)
		}
		response, err := r.srv.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}
	key := r.d.access.HookKey
	other := strings.Repeat("A", 32)
	for name, got := range map[string]int{
		"no key":                ask(token, hire.ID, ""),
		"another token":         ask(other, hire.ID, key),
		"another hire":          ask(token, "abcdef123456", key),
		"not a token":           ask("nope", hire.ID, key),
		"a plain hire's":        ask(r.plainPending(), hire.ID, key),
		"a plain hire's, no id": ask(r.plainPending(), "", key),
	} {
		want := http.StatusForbidden
		if name == "no key" {
			want = http.StatusUnauthorized
		}
		if got != want {
			t.Errorf("%s: %d, want %d", name, got, want)
		}
	}
	if got := ask(token, hire.ID, key); got != http.StatusOK {
		t.Fatalf("the right token: %d", got)
	}
	if got := ask(token, hire.ID, key); got != http.StatusGone {
		t.Errorf("a second ask: %d, want 410", got)
	}
	// Expired: the token is older than a hire may wait.
	_, again, _ := runLauncherHire(r.request("janitor"), r.o)
	r.d.mu.Lock()
	r.d.hires[again].at = time.Now().Add(-hiredTabWait - time.Second)
	r.d.mu.Unlock()
	var second Hire
	for _, h := range readRegistry(registryPath(), false).hires {
		if h.ID != hire.ID {
			second = h
		}
	}
	if got := ask(again, second.ID, key); got != http.StatusGone {
		t.Errorf("an expired token: %d, want 410", got)
	}
	// And the launcher refuses to start anything without it.
	if _, err := fetchHireStartEnv(r.srv.Client(), r.srv.URL, key, token, hire.ID); err == nil || strings.Contains(err.Error(), token) {
		t.Errorf("a used token's error: %v", err)
	}
	if _, err := fetchHireStartEnv(r.srv.Client(), "http://127.0.0.1:1", key, token, hire.ID); err == nil || strings.Contains(err.Error(), token) {
		t.Errorf("an unreachable app's error: %v", err)
	}
}

// plainPending is the token of a hire the old way: no id, no environment.
func (r *lrig) plainPending() string {
	token := newID()
	r.d.mu.Lock()
	r.d.hires[token] = &pendingHire{desk: -1, at: time.Now()}
	r.d.mu.Unlock()
	return token
}

func TestALauncherIsKnownByTheIdOfItsTokenNeverByWhatItSays(t *testing.T) {
	r := newLRig(t)
	a, wa := r.hire("janitor")
	b, wb := r.hire("janitor")
	if wa.tab.hireID != a.ID || wb.tab.hireID != b.ID {
		t.Fatalf("ids: %q %q", wa.tab.hireID, wb.tab.hireID)
	}
	// Whatever a launcher claims in its query, only a pending token gives an id.
	if tab := r.d.launcherConnected("L-plain", Claude, "/x", "", newID()); tab.hireID != "" {
		t.Errorf("a made-up token gave id %q", tab.hireID)
	}
	if tab := r.d.launcherConnected("L-used", Claude, "/x", "", flagAfter(r.opened[0], "--hire")); tab.hireID != "" {
		t.Errorf("a used token gave id %q", tab.hireID)
	}
}

func TestMailFindsAnEngineHiresLauncherByIdIncludingTwoHiresInOneFolder(t *testing.T) {
	d := newTestDeck(t)
	same := "/work/api"
	connect := func(id, role, hireID string) {
		token := newID()
		if hireID != "" {
			d.expectLauncher(token, hireID, hireStartEnv{})
		} else {
			token = "office-hire-token"
		}
		if d.launcherConnected(id, Claude, same, role, token) == nil {
			t.Fatal("not connected")
		}
	}
	a := Hire{ID: "aaaaaaaaaaaa", Name: "A", Role: "janitor", Folder: same, Worktree: "/work/api-hires/a", ViaLauncher: true, State: "working"}
	b := Hire{ID: "bbbbbbbbbbbb", Name: "B", Role: "janitor", Folder: same, Worktree: "/work/api-hires/b", ViaLauncher: true, State: "working"}
	writeRegistry(t, a, b)
	connect("LA", "janitor", a.ID)
	connect("LB", "janitor", b.ID)
	if id, ok := d.hireLauncherID(a); !ok || id != "LA" {
		t.Errorf("A: %q %v", id, ok)
	}
	if id, ok := d.hireLauncherID(b); !ok || id != "LB" {
		t.Errorf("B: %q %v", id, ok)
	}
	// An engine hire without its launcher has none: the folder is not a way in, nor the owner's plain office window.
	c := Hire{ID: "cccccccccccc", Name: "C", Role: "janitor", Folder: same, ViaLauncher: true, State: "working"}
	connect("LP", "", "")
	writeRegistry(t, c)
	if id, ok := d.hireLauncherID(c); ok {
		t.Errorf("C was matched by folder: %q", id)
	}
	// A hire of the old way still is, by folder, and never to an id-carrying launcher.
	old := Hire{ID: "dddddddddddd", Name: "D", Role: "reviewer", Folder: same, State: "working"}
	writeRegistry(t, old)
	if id, ok := d.hireLauncherID(old); !ok || id != "LP" {
		t.Errorf("the old way: %q %v", id, ok)
	}
	// Never the GM's, by id or otherwise.
	gm := Hire{ID: "eeeeeeeeeeee", Name: "GM", Role: gmRole, ViaLauncher: true, State: "working"}
	connect("LG", "janitor", gm.ID) // whatever role the launcher says it has
	if id, ok := d.hireLauncherID(gm); ok {
		t.Errorf("the GM's window was matched: %q", id)
	}
	// Two launchers claiming one hire's id is doubt.
	d.mu.Lock()
	d.launchers["LB"].hireID = a.ID
	d.mu.Unlock()
	if id, ok := d.hireLauncherID(a); ok {
		t.Errorf("two windows for one hire: %q", id)
	}
}

func writeRegistry(t *testing.T, hires ...Hire) {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"version": 1, "hires": hires})
	if err := os.WriteFile(registryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (r *lrig) manage(roleID string) (int, string, bool) {
	r.d.launcherEngine = r.o
	r.d.mu.Lock()
	defer r.d.mu.Unlock()
	return r.d.managedLauncherHire(roleID, r.repo, hireOpener{}, true, -1, 0)
}

func TestAHireHeardFromIsWorkingAndOneNeverHeardFromIsCleanedUpWithItsLauncherStopped(t *testing.T) {
	t.Run("heard", func(t *testing.T) {
		r := newLRig(t)
		r.o.Within, r.o.Every = 2*time.Second, time.Millisecond
		heard := 0
		r.o.HeardFrom = func(Hire) bool { heard++; return heard > 3 }
		if status, message, handled := r.manage("janitor"); !handled || status != http.StatusNoContent {
			t.Fatalf("%d %q %v", status, message, handled)
		}
		r.d.watching.Wait()
		if h := registered(t); len(h) != 1 || h[0].State != "working" || r.window[h[0].ID].stopped() != 0 {
			t.Errorf("registry: %+v", h)
		}
	})
	t.Run("never heard", func(t *testing.T) {
		r := newLRig(t)
		r.o.Within, r.o.Every = 40*time.Millisecond, 5*time.Millisecond
		if status, _, handled := r.manage("janitor"); !handled || status != http.StatusNoContent {
			t.Fatal(status, handled)
		}
		r.d.watching.Wait()
		h := registered(t)[0]
		w := r.window[h.ID]
		if h.State != "failed" || !strings.HasPrefix(h.Notes, "Didn't start: no word from it in ") || w.stopped() != 1 {
			t.Errorf("hire: %+v, stops %d", h, w.stopped())
		}
		if id, _ := r.d.launcherOf(h.ID); id != "" {
			t.Error("its launcher is still connected")
		}
		if len(strings.Split(r.worktrees(), "\n")) != 1 {
			t.Errorf("worktree kept: %s", r.worktrees())
		}
		r.d.mu.Lock()
		pending := len(r.d.hires)
		r.d.mu.Unlock()
		if pending != 0 {
			t.Error("its token is still good")
		}
	})
	t.Run("window never opens", func(t *testing.T) {
		r := newLRig(t)
		r.launches = false
		r.o.Within, r.o.Every = 30*time.Millisecond, 5*time.Millisecond
		r.manage("janitor")
		r.d.watching.Wait()
		h := registered(t)[0]
		if h.State != "failed" || len(strings.Split(r.worktrees(), "\n")) != 1 {
			t.Errorf("hire: %+v / %s", h, r.worktrees())
		}
		// A launcher that opens late gets nothing: its token is gone.
		token := flagAfter(r.opened[0], "--hire")
		if _, err := fetchHireStartEnv(r.srv.Client(), r.srv.URL, r.d.access.HookKey, token, h.ID); err == nil {
			t.Error("a late window got its environment")
		}
	})
	t.Run("window closed before it reported", func(t *testing.T) {
		r := newLRig(t)
		r.o.Within, r.o.Every, r.o.Grace = 2*time.Second, 5*time.Millisecond, 30*time.Millisecond
		r.o.HeardFrom = func(h Hire) bool {
			if w := r.window[h.ID]; w != nil && w.stopped() == 0 {
				r.d.launcherGone(w.id, w.tab) // the owner closes the window
				w.mu.Lock()
				w.stops++
				w.mu.Unlock()
			}
			return false
		}
		r.manage("janitor")
		r.d.watching.Wait()
		if h := registered(t)[0]; h.State != "failed" || !strings.Contains(h.Notes, "its window closed before it reported") {
			t.Errorf("hire: %+v", h)
		}
		// Really gone for longer than the grace: cleaned up as before, worktree and all.
		if len(strings.Split(r.worktrees(), "\n")) != 1 {
			t.Errorf("worktree kept: %s", r.worktrees())
		}
	})
	t.Run("launcher that won't stop keeps the worktree", func(t *testing.T) {
		r := newLRig(t)
		r.stopEnds = false
		r.o.Within, r.o.Every = 30*time.Millisecond, 5*time.Millisecond
		r.manage("janitor")
		r.d.watching.Wait()
		h := registered(t)[0]
		if h.State != "failed" || len(strings.Split(r.worktrees(), "\n")) != 2 || !strings.Contains(h.Notes, "worktree was left") {
			t.Errorf("hire: %+v / %s", h, r.worktrees())
		}
	})
}

func TestAGMThatNeverStartsOrWhoseWindowWontOpenTakesItsTokenAway(t *testing.T) {
	r := newLRig(t)
	r.launches = false
	r.o.Within, r.o.Every = 30*time.Millisecond, 5*time.Millisecond
	if status, _, _ := r.manage("gm"); status != http.StatusNoContent {
		t.Fatal(status)
	}
	r.d.watching.Wait()
	if r.d.gmTokenNow() != "" {
		t.Error("a GM that never started kept its token")
	}
	r2 := newLRig(t)
	r2.o.Open = func([]string, string) error { return os.ErrPermission }
	if status, _, _ := r2.manage("gm"); status != http.StatusInternalServerError {
		t.Fatal(status)
	}
	if r2.d.gmTokenNow() != "" {
		t.Error("a GM whose window wouldn't open kept its token")
	}
}

// gmTokenNow is the GM's token as the deck holds it, read under its lock (a watch may be clearing it).
func (d *Deck) gmTokenNow() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.gmToken
}

func (r *lrig) fireRefused(h Hire, force bool, want string) {
	r.t.Helper()
	_, err := r.d.fireLauncherHire(h, r.o, force)
	refused, ok := err.(*hireRefusal)
	if !ok || refused.Status != http.StatusConflict || refused.Message != want {
		r.t.Fatalf("fire (force %v): %v, want %q", force, err, want)
	}
}

// unchanged is that a refused fire changed nothing: hire working, window open and untouched, worktree there, no recap.
func (r *lrig) unchanged(h Hire, w *winLauncher) {
	r.t.Helper()
	if got := registered(r.t); len(got) == 0 || got[len(got)-1].State == "fired" || w.stopped() != 0 || len(w.typed()) != 0 {
		r.t.Errorf("a refused fire changed something: %+v stops %d typed %q", got, w.stopped(), w.typed())
	}
	if id, _ := r.d.launcherOf(h.ID); id == "" {
		r.t.Error("the launcher is gone")
	}
	if _, err := os.Stat(h.Worktree); err != nil {
		r.t.Errorf("worktree: %v", err)
	}
	if recaps, _ := os.ReadDir(filepath.Join(supportDir, "recaps")); len(recaps) != 0 {
		r.t.Error("a recap was written")
	}
}

func TestAFireRefusesInTheEngineTmuxWordsAndChangesNothing(t *testing.T) {
	cases := []struct {
		name   string
		shows  string
		dirty  bool
		force  bool
		reason string
		want   string
	}{
		{"menu", "permission-menu", false, false, "", "Janitor is waiting for an answer in its window. Answer it there first; nothing was ended."},
		{"menu with force", "permission-menu", false, true, "", "Janitor is waiting for an answer in its window. Answer it there first; nothing was ended."},
		{"draft", "draft", false, false, "", "Janitor has something typed in its window that isn't sent yet. Send or clear it, or fire with force."},
		{"busy", "busy-1", false, false, "", "Janitor is working right now. Wait until it is done, or fire with force."},
		{"uncommitted", "idle-empty", true, false, "", "Janitor has changes that aren't committed: ?? new.txt. Commit them, or fire with force."},
		{"unreadable", "idle-empty", false, false, "unreadable", "Couldn't tell what Janitor's window holds, so nothing was ended. Look at it first."},
		{"unreadable with force", "idle-empty", false, true, "unreadable", "Couldn't tell what Janitor's window holds, so nothing was ended. Look at it first."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newLRig(t)
			r.shows = c.shows
			h, w := r.hire("janitor")
			if c.reason == "unreadable" {
				w.screen.mu.Lock()
				w.screen.screen.broken = true
				w.screen.mu.Unlock()
			}
			if c.dirty {
				os.WriteFile(filepath.Join(h.Worktree, "new.txt"), []byte("x"), 0o644)
			}
			r.fireRefused(h, c.force, c.want)
			r.unchanged(h, w)
		})
	}
	t.Run("not reachable", func(t *testing.T) {
		r := newLRig(t)
		r.launches = false
		h, _ := r.hire("janitor")
		r.fireRefused(h, false, "Couldn't reach Janitor's window, so nothing was changed.")
		r.fireRefused(h, true, "Couldn't reach Janitor's window, so nothing was changed.")
	})
	t.Run("already gone", func(t *testing.T) {
		r := newLRig(t)
		h, w := r.hire("janitor")
		markFired(h.ID, "")
		r.fireRefused(h, false, "Janitor is already gone.")
		if w.stopped() != 0 {
			t.Error("a launcher was stopped for a hire already gone")
		}
	})
	t.Run("being fired", func(t *testing.T) {
		r := newLRig(t)
		h, _ := r.hire("janitor")
		firing.Lock()
		firing.ids[h.ID] = true
		firing.Unlock()
		defer func() { firing.Lock(); delete(firing.ids, h.ID); firing.Unlock() }()
		r.fireRefused(h, false, "Janitor is being fired already.")
	})
}

func TestAFireLooksAgainJustBeforeStoppingAndStopsAtAQuestionThatCameUpMeanwhile(t *testing.T) {
	r := newLRig(t)
	h, w := r.hire("janitor")
	r.afterAnswer = func(n int) {
		if n == 1 {
			w.screen.Write([]byte("\x1b[2J\x1b[H" + ptyOutput(pane(t, "permission-menu"))))
		}
	}
	r.fireRefused(h, true, "Janitor is waiting for an answer in its window. Answer it there first; nothing was ended.")
	r.unchanged(h, w)
}

func TestForceSkipsOnlyADraftBusyOrUncommittedFilesAndNothingIsEverTyped(t *testing.T) {
	for _, shows := range []string{"draft", "busy-1"} {
		r := newLRig(t)
		r.shows = shows
		h, w := r.hire("janitor")
		os.WriteFile(filepath.Join(h.Worktree, "new.txt"), []byte("x"), 0o644)
		if kept, err := r.d.fireLauncherHire(h, r.o, true); err != nil || kept == "" {
			t.Fatalf("%s: forced fire: kept %q err %v", shows, kept, err)
		}
		// The uncommitted file kept the worktree, and it said so.
		if got := registered(t)[0]; got.State != "fired" || !strings.Contains(got.Notes, "was kept") {
			t.Errorf("%s: %+v", shows, got)
		}
		if w.stopped() != 1 || len(w.typed()) != 0 {
			t.Errorf("%s: stops %d, typed %q", shows, w.stopped(), w.typed())
		}
	}
}

func TestAFireOfAnIdleHireStopsItsLauncherRemovesItsWorktreeWritesTheRecapAndMarksItFired(t *testing.T) {
	r := newLRig(t)
	h, w := r.hire("janitor")
	if kept, err := r.d.fireLauncherHire(h, r.o, false); err != nil || kept != "" {
		t.Fatalf("fire: %q %v", kept, err)
	}
	if got := registered(t)[0]; got.State != "fired" {
		t.Errorf("state %q", got.State)
	}
	if id, _ := r.d.launcherOf(h.ID); id != "" || w.stopped() != 1 || len(w.typed()) != 0 {
		t.Errorf("launcher %q stops %d typed %q", id, w.stopped(), w.typed())
	}
	if len(strings.Split(r.worktrees(), "\n")) != 1 {
		t.Errorf("worktree kept: %s", r.worktrees())
	}
	info, err := os.Stat(filepath.Join(supportDir, "recaps", h.ID+".md"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("recap: %v %v", info, err)
	}
}

func TestALauncherThatWontStopChangesNothing(t *testing.T) {
	r := newLRig(t)
	r.stopEnds = false
	h, w := r.hire("janitor")
	r.fireRefused(h, true, "Couldn't end Janitor's window, so nothing was changed.")
	if w.stopped() != 1 {
		t.Errorf("stops %d", w.stopped())
	}
	w.stops = 0
	r.unchanged(h, w)
	if got := registered(t)[0]; got.State == "fired" {
		t.Error("marked fired")
	}
}

func TestAFireThroughTheRouteClearsTheGMTokenFailsWaitingMailAndLeavesOtherTabsAlone(t *testing.T) {
	r := newLRig(t)
	gm, gmWindow := r.hire("gm")
	other, otherWindow := r.hire("janitor")
	if r.d.gmTokenNow() == "" {
		t.Fatal("no GM token")
	}
	to := func(id, name, msg string) mailRequest {
		return mailRequest{From: mailFromGM, To: name, Text: "hello", ID: msg, IDGiven: true,
			Resolve: func() (mailDest, *mailError) { return mailDest{ID: id, Name: name}, nil }}
	}
	for _, req := range []mailRequest{to(gm.ID, "General Manager", "m1"), to(other.ID, "Janitor", "m2")} {
		if _, err := r.d.mail.add(req); err != nil {
			t.Fatal(err)
		}
	}
	r.d.launcherEngine = r.o
	// A plain launcher tab, or an unknown one, is not an engine hire's.
	if _, _, handled := r.d.fireLauncherTab("launcher:nobody", false); handled {
		t.Error("an unknown tab was handled")
	}
	if status, message, handled := r.d.fireLauncherTab("launcher:"+gmWindow.id, false); !handled || status != http.StatusNoContent {
		t.Fatalf("fire: %d %q %v", status, message, handled)
	}
	if r.d.gmTokenNow() != "" {
		t.Error("the fired GM's token still works")
	}
	if m, _ := r.d.mail.get("m1"); m.State != mailFailed || m.Note != "General Manager is gone." {
		t.Errorf("the GM's mail: %+v", m)
	}
	if m, _ := r.d.mail.get("m2"); m.State != mailQueued {
		t.Errorf("another hire's mail: %+v", m)
	}
	if id, _ := r.d.launcherOf(other.ID); id == "" || otherWindow.stopped() != 0 {
		t.Error("another hire's window was touched")
	}
	// A refusal through the route says its words and ends nothing.
	otherWindow.screen.Write([]byte("\x1b[2J\x1b[H" + ptyOutput(pane(t, "permission-menu"))))
	status, message, handled := r.d.fireLauncherTab("launcher:"+otherWindow.id, true)
	if !handled || status != http.StatusConflict || !strings.Contains(message, "waiting for an answer") || otherWindow.stopped() != 0 {
		t.Errorf("refusal: %d %q %v", status, message, handled)
	}
}

func TestTheWorktreeGuardOfALauncherHireNeedsItsLauncherGoneAndEveryOtherCheck(t *testing.T) {
	r := newLRig(t)
	h, _ := r.hire("janitor")
	goneYes := func(Hire) bool { return true }
	if removeLauncherHireWorktree(r.o.Git, r.o.Worktrees, func(Hire) bool { return false })(h) {
		t.Fatal("removed while its launcher is connected")
	}
	plain := h
	plain.ViaLauncher = false
	if removeLauncherHireWorktree(r.o.Git, r.o.Worktrees, goneYes)(plain) {
		t.Fatal("removed a hire the engine did not make")
	}
	wrongBranch := h
	wrongBranch.Branch = "main"
	if removeLauncherHireWorktree(r.o.Git, r.o.Worktrees, goneYes)(wrongBranch) {
		t.Fatal("removed a worktree on another branch")
	}
	mainRepo := h
	mainRepo.Worktree = r.repo
	if removeLauncherHireWorktree(r.o.Git, r.o.Worktrees, goneYes)(mainRepo) {
		t.Fatal("removed the main repo")
	}
	if _, err := os.Stat(h.Worktree); err != nil {
		t.Fatal(err)
	}
	if !removeLauncherHireWorktree(r.o.Git, r.o.Worktrees, goneYes)(h) {
		t.Error("its own clean worktree wasn't removed once its launcher was gone")
	}
}

func TestSetupStateShowsTheBetaSwitchAndTheSettingsRouteSetsIt(t *testing.T) {
	d := newTestDeck(t)
	hiring := func() map[string]any {
		var state struct{ Hiring map[string]any }
		body := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer}).Body.Bytes()
		if err := json.Unmarshal(body, &state); err != nil {
			t.Fatal(err)
		}
		return state.Hiring
	}
	if got := hiring(); got["managedWindows"] != false {
		t.Errorf("default: %v", got["managedWindows"])
	}
	if _, ok := hiring()["managedBeta"]; !ok {
		t.Error("no managedBeta field")
	}
	d.config.HireFolder = t.TempDir()
	r := serve(t, d, call{method: "POST", target: "/setup/hiring?managedWindows=1", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}})
	if r.Code != http.StatusOK || !d.config.HireManagedWindows {
		t.Fatalf("set: %d %s %v", r.Code, r.Body, d.config.HireManagedWindows)
	}
	saved := launcherHires
	t.Cleanup(func() { launcherHires = saved })
	launcherHires = true
	if got := hiring(); got["managedWindows"] != true || got["managedBeta"] != true {
		t.Errorf("on windows with the switch on: %v %v", got["managedWindows"], got["managedBeta"])
	}
}

func TestTheLauncherTakesOnlyTheArgumentsOfAnEngineHire(t *testing.T) {
	token, id := strings.Repeat("A", 32), "abcdef123456"
	good := []string{"--hire", token, "--hire-id", id, "--agent", "claude", "--role", "janitor", "--settings=" + filepath.Join(t.TempDir(), "s.json")}
	run, err := parseRun(good)
	if err != nil || run.hireID != id || run.role != "janitor" || run.settings != good[8] {
		t.Fatalf("%+v %v", run, err)
	}
	for name, args := range map[string][]string{
		"id without a role":           {"--hire", token, "--hire-id", id, "claude"},
		"bad id":                      {"--hire", token, "--hire-id", "../x", "--role", "janitor"},
		"settings without an id":      {"--hire", token, "--role", "janitor", good[8]},
		"relative settings":           {"--hire", token, "--hire-id", id, "--role", "janitor", "--settings=s.json"},
		"another argument":            {"--hire", token, "--hire-id", id, "--role", "janitor", good[8], "--dangerous"},
		"another argument for a role": {"--hire", token, "--hire-id", id, "--role", "janitor", "--model=x"},
	} {
		if run, err := parseRun(args); err == nil {
			t.Errorf("%s: taken as %+v", name, run)
		}
	}
}

// reconnect is the hire's launcher after a dropped stream: it sends its token again, the
// token is used up, so it has no hire id, and it works in the hire's worktree.
func (r *lrig) reconnect(h Hire, w *winLauncher, token string) *winLauncher {
	r.t.Helper()
	r.d.launcherGone(w.id, w.tab)
	return r.connect("L-again-"+h.ID, hireDir(h), "janitor", token, "", w.env)
}

func TestALauncherThatConnectedAgainIsNeverFiredByAPlainStopAndStillCountsAsTheGM(t *testing.T) {
	r := newLRig(t)
	r.d.launcherEngine = r.o
	h, w := r.hire("janitor")
	token := flagAfter(r.opened[0], "--hire")
	setHire(h.ID, "working", "")
	again := r.reconnect(h, w, token)
	if again.tab.hireID != "" || !again.tab.office {
		t.Fatalf("the reconnected launcher: hire id %q, office %v", again.tab.hireID, again.tab.office)
	}
	status, message, handled := r.d.fireLauncherTab("launcher:"+again.id, true)
	if !handled || status != http.StatusConflict || message != "Couldn't tell which hire's window this is, so nothing was changed. Close it yourself, then hire again." {
		t.Errorf("fire: %d %q %v", status, message, handled)
	}
	if again.stopped() != 0 || len(again.typed()) != 0 {
		t.Error("the reconnected window was sent something")
	}
	if got := registered(t); got[len(got)-1].State != "working" {
		t.Errorf("registry: %+v", got)
	}
	// A plain launcher (no office token) somewhere else is closed the old way.
	other := r.connect("L-plain", t.TempDir(), "", "", "", hireStartEnv{})
	if _, _, handled := r.d.fireLauncherTab("launcher:"+other.id, false); handled {
		t.Error("a plain launcher's fire was taken")
	}

	// The GM: a second one is refused while the reconnected GM window is up.
	r2 := newLRig(t)
	gm, gmWindow := r2.hire("gm")
	gmToken := flagAfter(r2.opened[0], "--hire")
	setHire(gm.ID, "working", "")
	gmAgain := r2.reconnect(gm, gmWindow, gmToken)
	r2.d.mu.Lock()
	err := r2.d.gmRefusal()
	r2.d.mu.Unlock()
	if refused, ok := err.(*hireRefusal); !ok || refused.Status != http.StatusConflict {
		t.Errorf("a second GM with the first's window reconnected: %v", err)
	}
	r2.d.launcherGone(gmAgain.id, gmAgain.tab)
	r2.d.mu.Lock()
	err = r2.d.gmRefusal()
	r2.d.mu.Unlock()
	if err != nil {
		t.Errorf("a second GM with the first's window gone: %v", err)
	}
}

func TestACleanUpAfterADropNeverTakesTheWorktreeOfAHireThatIsStillRunning(t *testing.T) {
	t.Run("dropped and back without its id while the watch looks", func(t *testing.T) {
		r := newLRig(t)
		r.o.Within, r.o.Every, r.o.Grace = 5*time.Second, 2*time.Millisecond, 100*time.Millisecond
		var token string
		var w, again *winLauncher
		calls := 0
		r.o.HeardFrom = func(h Hire) bool {
			calls++
			switch calls {
			case 3:
				token, w = flagAfter(r.opened[0], "--hire"), r.window[h.ID]
				r.d.launcherGone(w.id, w.tab) // the stream drops
			case 12:
				again = r.connect("L-again", h.Worktree, "janitor", token, "", w.env)
			}
			return calls > 30
		}
		r.manage("janitor")
		r.d.watching.Wait()
		h := registered(t)[0]
		if h.State != "working" {
			t.Errorf("hire: %+v", h)
		}
		if _, err := os.Stat(h.Worktree); err != nil || again == nil {
			t.Errorf("worktree: %v, reconnected %v", err, again != nil)
		}
	})
	t.Run("back at once without its id, for longer than the grace", func(t *testing.T) {
		r := newLRig(t)
		r.o.Within, r.o.Every, r.o.Grace = 5*time.Second, 2*time.Millisecond, 10*time.Millisecond
		calls := 0
		r.o.HeardFrom = func(h Hire) bool {
			if calls++; calls == 3 {
				r.reconnect(h, r.window[h.ID], flagAfter(r.opened[0], "--hire"))
			}
			return calls > 40
		}
		r.manage("janitor")
		r.d.watching.Wait()
		if h := registered(t)[0]; h.State != "working" {
			t.Errorf("hire: %+v", h)
		}
	})
	for _, c := range []struct{ role, where string }{{"janitor", "a worktree"}, {"gm", "a worktree"}, {"janitor", "a folder that isn't a repo"}} {
		t.Run("back without its id, the watch times out: "+c.role+" in "+c.where, func(t *testing.T) {
			r := newLRig(t)
			if c.where != "a worktree" {
				r.repo = physical(t.TempDir())
			}
			r.d.launcherEngine = r.o
			r.o.Within, r.o.Every, r.o.Grace = 60*time.Millisecond, 2*time.Millisecond, 20*time.Millisecond
			calls := 0
			var again *winLauncher
			r.o.HeardFrom = func(h Hire) bool {
				if calls++; calls == 2 {
					again = r.reconnect(h, r.window[h.ID], flagAfter(r.opened[0], "--hire"))
				}
				return false
			}
			r.manage(c.role)
			r.d.watching.Wait()
			h := registered(t)[0]
			if h.Worktree != "" {
				if _, err := os.Stat(h.Worktree); err != nil {
					t.Errorf("a running hire's worktree was taken: %v", err)
				}
			}
			// Its window is still open: it stays working, so the fire refuses and a second GM is refused.
			if h.State != "working" || !strings.Contains(h.Notes, "connected again without its id") {
				t.Errorf("hire: %+v", h)
			}
			status, _, handled := r.d.fireLauncherTab("launcher:"+again.id, true)
			if !handled || status != http.StatusConflict || again.stopped() != 0 {
				t.Errorf("fire on the reconnected window: %d %v, %d stops", status, handled, again.stopped())
			}
			if c.role == "gm" {
				r.d.mu.Lock()
				err := r.d.gmRefusal()
				r.d.mu.Unlock()
				if err == nil {
					t.Error("a second GM was allowed while the first's window is open")
				}
			}
		})
	}
}

func TestTheLauncherAsksTheAppWithATimeoutAndGivesUpOnOneThatNeverAnswers(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		select {
		case <-release:
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()
	defer close(release)
	old := hireStartTimeout
	hireStartTimeout = 100 * time.Millisecond
	defer func() { hireStartTimeout = old }()
	began := time.Now()
	_, err := fetchHireStartEnv(hireStartClient(), srv.URL, "fake-hook-key", "tok", "id")
	if err == nil || time.Since(began) > 3*time.Second {
		t.Errorf("an app that never answers: %v after %v", err, time.Since(began))
	}
	if hireStartClient().Timeout != 100*time.Millisecond {
		t.Error("the launcher's client has no timeout")
	}
}

// A hire in a folder that isn't a git repo has no worktree: its launcher that
// connected again is still known by that folder, for the fire and the watch.
func TestAHireWithNoWorktreeKeepsItsProtectionAfterItsLauncherConnectsAgain(t *testing.T) {
	r := newLRig(t)
	r.repo = physical(t.TempDir())
	r.d.launcherEngine = r.o
	h, w := r.hire("janitor")
	if h.Worktree != "" {
		t.Fatalf("a hire in a plain folder got a worktree: %+v", h)
	}
	setHire(h.ID, "working", "")
	again := r.reconnect(h, w, flagAfter(r.opened[0], "--hire"))
	status, _, handled := r.d.fireLauncherTab("launcher:"+again.id, true)
	if !handled || status != http.StatusConflict || again.stopped() != 0 {
		t.Errorf("fire: %d %v, %d stops", status, handled, again.stopped())
	}
	if !r.d.launcherHolds(h) {
		t.Error("the reconnected window isn't counted as the hire's")
	}
}

// The launcher's hire has its brief written as the engine's does; the launcher is handed the message
// that names it with the hire's environment, as it can't make that message itself.
func TestALauncherHireIsHandedTheMessageToReadItsBrief(t *testing.T) {
	r := newLRig(t)
	r.launches = false
	var token, hireID string
	r.o.Expect = func(given, id string, env hireStartEnv) {
		token, hireID = given, id
		r.d.expectLauncher(given, id, env)
	}
	request := r.request("janitor")
	request.Role.Brief = &RoleBrief{Kind: "library", ID: "janitor"}
	hire, _, err := runLauncherHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	brief := filepath.Join(hireFolder(supportDir, hire.ID), "BRIEF.md")
	if data, err := os.ReadFile(brief); err != nil || !strings.Contains(string(data), "# Brief: Janitor") || hire.Brief != brief {
		t.Fatalf("the brief: %v %q", err, hire.Brief)
	}
	env, status := r.d.takeStartEnv(token, hireID)
	if status != http.StatusOK || env.Message != "Read `"+tildePath(brief)+"` and follow it: it's your brief, as my Janitor. As it says, your "+
		"first reply must only ask me its questions, all together; wait for my answers before you read code or run commands." {
		t.Errorf("handed %d %q", status, env.Message)
	}
	// The message isn't on the window's command line.
	if words := strings.Join(commandWords(r.opened[0]), " "); strings.Contains(words, "BRIEF") {
		t.Errorf("the command names the brief: %s", words)
	}
}
