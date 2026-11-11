//go:build !windows

package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// C3's findings (the two-engine comparison of hiring): each answered as the Mac's engine answers it.

// twoAccounts saves a second Claude account, work, that a hire can start on (Claude Code has run there once).
func twoAccounts(t *testing.T, d *Deck) string {
	t.Helper()
	work := claudeFolder(t, filepath.Join(homeDir, ".claude-work"), ".claude.json")
	d.mu.Lock()
	d.config.ClaudeProfiles = []claudeProfile{{Name: "work", Dir: work}}
	d.mu.Unlock()
	return work
}

const askWhichAccount = "Say which Claude account to hire on: default, work."

// A1: Start here's GM hire reads account= and worktree=1 as the Mac's DeckRouter does, and passes them on.
func TestStartHeresGMHireTakesTheAccountAndTheWorktreeAsTheMacs(t *testing.T) {
	for _, worktree := range []bool{false, true} {
		d, r, _ := engineDeck(t)
		d.engine.HeardFrom = func(Hire) bool { return true }
		work := twoAccounts(t, d)
		// None named, with two: the Mac's question, nothing made, the switches back as they were.
		if code, text := pageCall(t, d, "POST", "/setup/hire/gm"); code != http.StatusBadRequest || text != askWhichAccount {
			t.Fatalf("no account named: %d %q", code, text)
		}
		if len(registered(t)) != 0 || r.tmux.ran("new-session") || d.config.ManagerEnabled {
			t.Fatalf("a refused GM hire made something: %+v", registered(t))
		}
		target := "/setup/hire/gm?account=work"
		if worktree {
			target += "&worktree=1"
		}
		if code, text := pageCall(t, d, "POST", target); code != http.StatusNoContent {
			t.Fatalf("%s: %d %q", target, code, text)
		}
		d.watching.Wait()
		got := registered(t)
		if len(got) != 1 || got[0].Role != gmRole || got[0].Account != "work" || (got[0].Worktree != "") != worktree {
			t.Fatalf("%s: registry %+v", target, got)
		}
		if script := r.script(got[0]); !strings.Contains(script, "export CLAUDE_CONFIG_DIR="+shellQuoted(work)) {
			t.Errorf("%s: the GM doesn't start on the work account:\n%s", target, script)
		}
	}
}

// A2: with more than one account, the office's hire, Anyone's and the GM route's ask which, in the Mac's words;
// the GM route takes the account it names.
func TestEveryWayInAsksWhichClaudeAccountWithMoreThanOne(t *testing.T) {
	d, r, h, token := gmHireDeck(t)
	twoAccounts(t, d)
	for name, hire := range map[string]func() (int, string){
		"the office's hire": func() (int, string) { return d.Perform("hire", "", url.Values{"role": {"janitor"}}) },
		"Anyone's hire":     func() (int, string) { return d.Perform("hire", "", url.Values{}) },
		"upstairs": func() (int, string) {
			return d.Perform("hire", "", url.Values{"role": {"janitor"}, "floor": {"1"}, "desk": {"0"}})
		},
		"the GM's route": func() (int, string) { return postGMHire(t, d, asGMCaller, token, `{"role":"janitor"}`) },
		"Start here's":   func() (int, string) { return pageCall(t, d, "POST", "/setup/hire/gm") },
	} {
		if code, text := hire(); code != http.StatusBadRequest || text != askWhichAccount {
			t.Errorf("%s: %d %q, want 400 %q", name, code, text, askWhichAccount)
		}
	}
	nothingMade(t, r, h)
	if code, text := postGMHire(t, d, asGMCaller, token, `{"role":"janitor","account":"nobody"}`); code != http.StatusNotFound ||
		text != "There's no Claude account called nobody. Pick one of: default, work." {
		t.Errorf("an account that isn't there: %d %q", code, text)
	}
	if code, text := postGMHire(t, d, asGMCaller, token, `{"role":"janitor","account":"work"}`); code != http.StatusOK {
		t.Fatalf("the GM's hire on work: %d %q", code, text)
	}
	if got := registered(t); len(got) != 1 || got[0].Account != "work" {
		t.Errorf("registry %+v", got)
	}
}

// With one account (the default), nothing is asked, as before.
func TestOneAccountIsNeverAskedAbout(t *testing.T) {
	d, _, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	status, text := d.Perform("hire", "", url.Values{"role": {"janitor"}})
	d.watching.Wait() // its watcher done before the next test's folders
	if status != http.StatusNoContent {
		t.Fatalf("%d %q", status, text)
	}
}

// exitingPane makes the stand-in's pane take a paste (in a mode or not: mode is pane_in_mode, "1" for one),
// and its session end by itself when ends is set and "/exit" is pasted, then Return pressed. What was pasted is kept.
func exitingPane(r *engineRig, mode string, ends bool) *[]string {
	var pasted []string
	inner := r.o.Tmux
	r.o.Tmux = func(args []string) (string, int, bool) {
		switch args[2] {
		case "display-message":
			r.tmux.asked = append(r.tmux.asked, args)
			return mode + " 1", 0, true
		case "set-buffer":
			r.tmux.asked = append(r.tmux.asked, args)
			pasted = append(pasted, args[len(args)-1])
			return "", 0, true
		case "send-keys":
			r.tmux.asked = append(r.tmux.asked, args)
			if ends && len(pasted) > 0 && pasted[len(pasted)-1] == "/exit" && args[len(args)-1] == "Enter" {
				r.tmux.up = false
			}
			return "", 0, true
		}
		return inner(args)
	}
	return &pasted
}

func verbs(r *engineRig) (out []string) {
	for _, a := range r.tmux.asked {
		out = append(out, a[2])
	}
	return out
}

// A3: an idle hire with an empty box is ended as the Mac ends it: "/exit" and Return pasted into its own tmux
// session (on its own socket), a bounded wait for the session to end, then its server ended if it is still up.
func TestAFireTypesExitIntoAnIdleHireAndWaitsForItsSessionToEnd(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	windowShows(r, pane(t, "idle-empty"), true)
	pasted := exitingPane(r, "0", true)
	if _, err := fireEngineHire(hire, 0, r.o, false); err != nil {
		t.Fatal(err)
	}
	got := verbs(r)
	at := func(verb string) int { return slices.Index(got, verb) }
	if !slices.Equal(*pasted, []string{"/exit"}) || at("set-buffer") < 0 || at("paste-buffer") < at("set-buffer") || at("send-keys") < at("paste-buffer") {
		t.Fatalf("pasted %q, tmux was asked %q", *pasted, got)
	}
	// It ended by itself: its server isn't killed, and nothing went to any other socket.
	if r.tmux.ran("kill-server") || r.tmux.ran("kill-session") || registered(t)[0].State != "fired" {
		t.Errorf("tmux was asked %q, registry %+v", got, registered(t))
	}
	for _, a := range r.tmux.asked[1:] {
		if a[0] != "-S" || a[1] != hire.TmuxSocket {
			t.Errorf("a call not on the hire's own socket: %q", a)
		}
	}
	if a := r.tmux.asked[slices.Index(got, "send-keys")]; !slices.Equal(a[3:], []string{"-t", hirePane, "Enter"}) {
		t.Errorf("Return went to %q", a)
	}
}

// A session that doesn't end within the wait is ended with its server; one where nothing may be typed (in a
// mode of tmux's own, or forced while at work) is ended straight away, with nothing typed.
func TestAFireEndsTheServerWhenTheSessionDoesntEndAndTypesNothingWhereItMayNot(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	windowShows(r, pane(t, "idle-empty"), true)
	pasted := exitingPane(r, "0", false)
	clock, waited := time.Unix(1_790_000_000, 0), time.Duration(0)
	r.o.Now = func() time.Time { return clock }
	r.o.Pause = func(d time.Duration) { clock, waited = clock.Add(d), waited+d }
	if _, err := fireEngineHire(hire, 0, r.o, false); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(*pasted, []string{"/exit"}) || !r.tmux.ran("kill-server") || waited != fireExitWithin {
		t.Errorf("pasted %q, waited %v, tmux %q", *pasted, waited, verbs(r))
	}
	for name, c := range map[string]struct {
		capture, mode string
		force         bool
	}{
		"scrolled back": {pane(t, "idle-empty"), "1", false},
		"busy, forced":  {pane(t, "busy-1"), "0", true},
	} {
		_, r, hire := hiredForFire(t, "")
		windowShows(r, c.capture, true)
		pasted := exitingPane(r, c.mode, true)
		if _, err := fireEngineHire(hire, 0, r.o, c.force); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(*pasted) != 0 || r.tmux.ran("send-keys") || !r.tmux.ran("kill-server") || registered(t)[0].State != "fired" {
			t.Errorf("%s: pasted %q, tmux %q", name, *pasted, verbs(r))
		}
	}
}

// Idle at the first look but not at the second: nothing is sent, nothing ended, in the Mac's words.
func TestAWindowThatChangedBeforeTheExitIsLeftAlone(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	looks := 0
	inner := r.tmux.run
	r.o.Tmux = func(args []string) (string, int, bool) {
		if args[2] == "capture-pane" {
			r.tmux.asked = append(r.tmux.asked, args)
			if looks++; looks == 1 {
				return pane(t, "idle-empty"), 0, true
			}
			return pane(t, "busy-1"), 0, true
		}
		return inner(args)
	}
	_, err := fireEngineHire(hire, 0, r.o, true)
	if err == nil || err.Error() != "Its window changed while it was being looked at, so nothing was sent. Try again." {
		t.Fatalf("err = %v", err)
	}
	if r.tmux.ran("set-buffer") || r.tmux.ran("kill-server") || !r.tmux.up || registered(t)[0].State != "starting" {
		t.Errorf("tmux %q, registry %+v", verbs(r), registered(t))
	}
}

// A4: PermissionDenied is among Claude Code's hook events, as the Mac's Hooks.claudeEvents, so a hire's settings
// and the owner's Connect both report it.
func TestClaudesHookEventsAreTheMacsWithPermissionDenied(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "Sources", "AgentDeck", "Hooks.swift"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "static let claudeEvents = [")
	end := strings.Index(text[start:], "]")
	var mac []string
	for _, part := range strings.Split(text[start+len("static let claudeEvents = ["):start+end], ",") {
		if name := strings.Trim(strings.TrimSpace(part), `"`); name != "" {
			mac = append(mac, name)
		}
	}
	if got := entriesFor(Claude).events; !slices.Contains(mac, "PermissionDenied") || !slices.Equal(got, mac) {
		t.Errorf("Go's Claude events %q, the Mac's %q", got, mac)
	}
	// A test copy's hire reports through its own settings file: PermissionDenied is there too.
	if _, ok := reportingHooks("/x/smolcorp", 7760)["PermissionDenied"]; !ok {
		t.Error("a test copy's hire's settings don't report PermissionDenied")
	}
}

// A5: the registry's folder is the project, its worktree the worktree, as the contract's registry.json; a row the
// Go core wrote the old way (the worktree, or a folder in it, as the folder) is read as the project, and so saved.
func TestTheRegistrysFolderIsTheProjectAndOldRowsAreReadSo(t *testing.T) {
	r := newEngineRig(t)
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	if hire.Folder != r.repo || registered(t)[0].Folder != r.repo || hire.Worktree == "" {
		t.Fatalf("hire %+v", hire)
	}
	old := []Hire{
		{ID: "a1", Name: "A", Folder: "/w/app-hires/a-1", Worktree: "/w/app-hires/a-1", Repo: "/w/app", State: "working"},
		{ID: "b2", Name: "B", Folder: "/w/app-hires/b-2/sub/x", Worktree: "/w/app-hires/b-2", Repo: "/w/app", State: "working"},
		{ID: "c3", Name: "C", Folder: "/w/plain", State: "idle"},
		{ID: "d4", Name: "D", Folder: "/w/app", Worktree: "/w/app-hires/d-4", Repo: "/w/app", State: "working"},
	}
	data, _ := json.Marshal(map[string]any{"version": 1, "hires": old})
	if err := os.WriteFile(registryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	want := []string{"/w/app", filepath.Join("/w/app", "sub", "x"), "/w/plain", "/w/app"}
	for i, h := range registered(t) {
		if h.Folder != want[i] {
			t.Errorf("%s: folder %q, want %q", h.ID, h.Folder, want[i])
		}
	}
	// Saved so at the next write.
	if err := updateRegistry(func(*registryFile) {}); err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(registryPath())
	if strings.Contains(string(saved), `"folder": "/w/app-hires`) {
		t.Errorf("an old row was saved as it was:\n%s", saved)
	}
}

// A8: a hire's socket is in tmux's own folder (as the Mac's `tmux -L`), so a support folder too long for a socket
// no longer refuses every hire; a hire made before, with its socket in the support folder, is still its own.
func TestALongSupportFolderNoLongerRefusesHires(t *testing.T) {
	r := newEngineRig(t)
	long := filepath.Join(t.TempDir(), strings.Repeat("a-long-user-name-", 6))
	if err := os.MkdirAll(long, 0o700); err != nil {
		t.Fatal(err)
	}
	supportDir = long
	sockets, err := os.MkdirTemp("", "tx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockets) })
	saved := hireSocketFolder
	t.Cleanup(func() { hireSocketFolder = saved })
	hireSocketFolder = func() string { return sockets }
	if len(filepath.Join(long, "smolcorp-7760-0123456789ab")) <= maxSocketPath {
		t.Fatal("the support folder isn't long enough to refuse a socket in it")
	}
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatalf("a hire with a long support folder: %v", err)
	}
	if filepath.Dir(hire.TmuxSocket) != sockets || !isHireSocket(hire, 0) || hireOwnTmux(hire, 0, r.o.Tmux) == nil {
		t.Errorf("socket %q, want it in %q and the hire's own", hire.TmuxSocket, sockets)
	}
	// One made before 1.25, in a (short) support folder, is still the hire's own: it can be fired and cleaned up.
	shortSupport(t)
	before := Hire{ID: "0a1b2c3d4e5f", TmuxSocket: filepath.Join(supportDir, "smolcorp-0a1b2c3d4e5f")}
	listenAt(t, before.TmuxSocket)
	if !isHireSocket(before, 0) || hireOwnTmux(before, 0, r.o.Tmux) == nil {
		t.Error("a hire made before, with its socket in the support folder, isn't its own any more")
	}
	// Elsewhere it isn't: not in another folder, nor the user's own default server in tmux's folder.
	for _, socket := range []string{filepath.Join(t.TempDir(), "smolcorp-0a1b2c3d4e5f"), filepath.Join(sockets, "default")} {
		if hireTmuxAt(socket, 0, r.o.Tmux) != nil {
			t.Errorf("%s counted as a hire's socket", socket)
		}
	}
}

// tmux's own folder is $TMUX_TMPDIR (or /tmp), then tmux-<uid>; one anyone else could open, or a link, is refused.
func TestTheSocketFolderIsTmuxsOwnAndMustBePrivate(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", "/some/where")
	if got, want := tmuxSocketFolder(), filepath.Join("/some/where", "tmux-"+strconv.Itoa(os.Getuid())); got != want {
		t.Errorf("tmux's folder %q, want %q", got, want)
	}
	t.Setenv("TMUX_TMPDIR", "")
	if got := tmuxSocketFolder(); got != "/tmp/tmux-"+strconv.Itoa(os.Getuid()) {
		t.Errorf("tmux's folder %q", got)
	}
	t.Setenv("TMUX_TMPDIR", "relative/folder") // taken against the core's own folder: ignored
	if got := tmuxSocketFolder(); got != "/tmp/tmux-"+strconv.Itoa(os.Getuid()) {
		t.Errorf("a relative TMUX_TMPDIR gave %q", got)
	}
	base := t.TempDir()
	made := filepath.Join(base, "made")
	if !ownSocketFolder(made) {
		t.Error("a folder it made isn't safe")
	}
	if info, err := os.Stat(made); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("made %v %v, want 0700", info, err)
	}
	open := filepath.Join(base, "open")
	os.Mkdir(open, 0o755)
	os.Chmod(open, 0o755)
	link := filepath.Join(base, "link")
	os.Symlink(made, link)
	for _, folder := range []string{open, link} {
		if ownSocketFolder(folder) {
			t.Errorf("%s counted as safe for a socket", folder)
		}
	}
	saved := hireSocketFolder
	t.Cleanup(func() { hireSocketFolder = saved })
	hireSocketFolder = func() string { return open }
	if newHireTmux("0a1b2c3d4e5f", 0, func([]string) (string, int, bool) { return "", 0, true }) != nil {
		t.Error("a hire's server was set up in a folder others can open")
	}
}

// A9: the GM's route says an unknown role first, and takes a folder inside a hidden one further down, as the
// Mac's (BriefRoutes.checkedFolder); a hidden folder at the home folder's top stays refused.
func TestTheGMsRouteSaysAnUnknownRoleFirstAndTakesAFolderBelowAHiddenOne(t *testing.T) {
	d, r, _, token := gmHireDeck(t)
	hidden := filepath.Join(homeDir, "Projects", ".team", "app")
	if err := os.MkdirAll(filepath.Dir(hidden), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(r.repo, hidden); err != nil {
		t.Fatal(err)
	}
	body := func(role, folder string) string {
		data, _ := json.Marshal(map[string]any{"role": role, "folder": folder, "model": "opus"})
		return string(data)
	}
	if code, text := postGMHire(t, d, asGMCaller, token, body("nobody", hidden)); code != http.StatusNotFound || text != "That role isn't there any more. Pick another." {
		t.Errorf("an unknown role: %d %q", code, text)
	}
	top := filepath.Join(homeDir, ".config", "app")
	os.MkdirAll(top, 0o755)
	if code, text := postGMHire(t, d, asGMCaller, token, `{"role":"janitor","folder":"`+top+`"}`); code != http.StatusBadRequest ||
		text != "That's a folder for your computer's or your apps' own files. Hire into a project folder." && text != "That's a folder for your Mac's or your apps' own files. Hire into a project folder." {
		t.Errorf("a hidden folder at the top: %d %q", code, text)
	}
	if code, text := postGMHire(t, d, asGMCaller, token, `{"role":"janitor","folder":"`+hidden+`"}`); code != http.StatusOK {
		t.Fatalf("a folder below a hidden one: %d %q", code, text)
	}
	if got := registered(t); len(got) != 1 || got[0].Folder != physical(hidden) {
		t.Errorf("registry %+v", got)
	}
}

// A10: under the Mac app, the hiring route says "this Mac", and the GM's screens, screen and tile are served (M7GAPC)
// from what the app reads; without the Mac app the same three are 404, as on Linux and Windows.
func TestTheMacsWordsUnderTheMacApp(t *testing.T) {
	routes := []struct{ method, path string }{
		{"GET", "/setup/gm/screens"}, {"POST", "/setup/gm/screen"}, {"POST", "/setup/gm/tile"},
	}
	// No Mac app: a version without them, as before.
	d := newTestDeck(t)
	managerOn(d)
	for _, c := range routes {
		if code, _ := pageCall(t, d, c.method, c.path); code != http.StatusNotFound {
			t.Errorf("without the Mac app, %s %s: %d", c.method, c.path, code)
		}
	}
	mac := withMac(t)
	mac.screens = []macScreen{builtIn}
	d = newTestDeck(t)
	managerOn(d)
	if code, text := pageCall(t, d, "POST", "/setup/hiring?terminal=nope"); code != http.StatusBadRequest || text != "That terminal isn't on this Mac" {
		t.Errorf("an unknown terminal: %d %q", code, text)
	}
	// The screens as the stand-in app reads them, a screen chosen asked for by its id, and a tile with nobody to place.
	if code, text := pageCall(t, d, "GET", "/setup/gm/screens"); code != http.StatusOK || !strings.Contains(text, `"name":"Built-in"`) {
		t.Errorf("GET /setup/gm/screens: %d %q", code, text)
	}
	if code, text := pageCall(t, d, "POST", "/setup/gm/screen"); code != http.StatusBadRequest || text != "Say which screen: id=<id> from /setup/gm/screens." {
		t.Errorf("POST /setup/gm/screen: %d %q", code, text)
	}
	if code, text := pageCall(t, d, "POST", "/setup/gm/tile"); code != http.StatusOK || !strings.Contains(text, "nothing to tile") {
		t.Errorf("POST /setup/gm/tile: %d %q", code, text)
	}
}

// FXX1d's engine race: the hire's agent reported (its row marked working) before its window failed to open. The
// answer says it wasn't hired, so the row is failed, and failed is final: a watcher that hears from it afterwards
// never marks it working again.
func TestAHireWhoseWindowDidntOpenStaysFailedEvenIfItReportedFirst(t *testing.T) {
	r := newEngineRig(t)
	r.o.Attach = func(string) error {
		setHire(registered(t)[0].ID, "working", "") // as its watcher would, had the agent reported first
		return errors.New("the window didn't open")
	}
	if _, err := runManagedHire(r.request("gm"), r.o); err == nil || err.Error() != "The terminal didn't open." {
		t.Fatalf("err = %v", err)
	}
	got := registered(t)
	if len(got) != 1 || got[0].State != "failed" || got[0].Notes != "Its window didn't open." {
		t.Fatalf("registry %+v, want the hire failed", got)
	}
	o := r.o
	o.Attach, o.HeardFrom = nil, func(Hire) bool { return true }
	o.Within, o.Every = 20*time.Millisecond, time.Millisecond
	watchManagedHire(got[0], 0, o)
	if after := registered(t); after[0].State != "failed" {
		t.Errorf("a watcher brought the failed hire back: %+v", after[0])
	}
	// A fired hire stays fired too: failHire leaves it as it is.
	markFired(got[0].ID, "")
	failHire(got[0].ID, "late")
	if after := registered(t); after[0].State != "failed" || after[0].Notes != "Its window didn't open." {
		t.Errorf("a closed row was changed: %+v", after[0])
	}
}
