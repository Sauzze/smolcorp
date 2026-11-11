package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// No test runs the real t3 or the real app: the routes' machine here has neither.
func init() {
	newT3Machine = func() t3Machine {
		return t3Machine{Home: filepath.Join(os.TempDir(), "smolcorp-t3-none"), GOOS: runtime.GOOS, Getenv: func(string) string { return "" },
			App: func() string { return "" }, OpenApp: func(string) {}, Run: func(string, []string, time.Duration) t3Ran { return t3Ran{} },
			Pause: func(time.Duration) {}, Now: time.Now}
	}
}

// t3Fake is a home folder with a fake t3 in it that writes down each run's arguments, a clock that moves only
// when the open pauses, and the app as the test says.
type t3Fake struct {
	home, calls, mode string
	opened            int
	clock             time.Time
	app               bool
}

func newT3Fake(t *testing.T) *t3Fake {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake t3 is a shell script; on Windows t3 runs as a .cmd through cmdLine, tested with OpenCode's")
	}
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &t3Fake{home: home, calls: filepath.Join(home, "calls"), mode: filepath.Join(home, "mode"), clock: time.Unix(1_000_000, 0)}
}

// install puts the fake t3 in folder, under the home folder, behaving as mode says: "ok", "fail" (always, on two
// lines), "exists" (project add says it's there) or "late" (fails twice).
func (f *t3Fake) install(t *testing.T, folder, mode string) {
	t.Helper()
	bin := filepath.Join(f.home, folder)
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	f.setMode(t, mode)
	script := `#!/bin/sh
for a in "$@"; do printf '%s\n' "$a" >> '` + f.calls + `'; done
printf -- '--\n' >> '` + f.calls + `'
mode=$(cat '` + f.mode + `')
if [ "$1" = "--version" ]; then
  [ "$mode" = "fail" ] && { echo "  broken install  " >&2; exit 1; }
  echo "0.1.8"; exit 0
fi
case "$mode" in
  fail) printf '  Could not reach the T3 Code desktop app.  \nmore detail\n' >&2; exit 1 ;;
  exists) echo "Project already exists: $3" >&2; exit 1 ;;
  late) n=$(grep -c '^--$' '` + f.calls + `'); [ "$n" -le 2 ] && { echo "not yet" >&2; exit 1; }; exit 0 ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "t3"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func (f *t3Fake) setMode(t *testing.T, mode string) {
	t.Helper()
	if err := os.WriteFile(f.mode, []byte(mode), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *t3Fake) machine() t3Machine {
	return t3Machine{Home: f.home, GOOS: runtime.GOOS, Getenv: func(string) string { return "" },
		App: func() string {
			if f.app {
				return "/fake/t3code"
			}
			return ""
		},
		OpenApp: func(string) { f.opened++ }, Run: runT3,
		Pause: func(d time.Duration) { f.clock = f.clock.Add(d) }, Now: func() time.Time { return f.clock }}
}

// runs is each run's arguments, as the fake wrote them down.
func (f *t3Fake) runs() [][]string {
	data, _ := os.ReadFile(f.calls)
	runs := [][]string{}
	for _, run := range strings.Split(string(data), "--\n") {
		if run == "" {
			continue
		}
		runs = append(runs, strings.Split(strings.TrimSuffix(run, "\n"), "\n"))
	}
	return runs
}

func (f *t3Fake) folder(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(f.home, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func t3Hire(folder, worktree string) Hire {
	return Hire{ID: "h1", Name: "Ada", Folder: folder, Worktree: worktree, Branch: "hire/ada-1a2b3c", State: "working"}
}

func TestT3EachStateIsFoundAndTheVersionIsAsked(t *testing.T) {
	f := newT3Fake(t)
	if got := t3Detect(f.machine()); got != (t3Status{State: t3None}) {
		t.Errorf("nothing: %+v", got)
	}
	f.app = true
	if got := t3Detect(f.machine()); got != (t3Status{State: t3AppOnlyIs}) {
		t.Errorf("the app alone: %+v", got)
	}
	f.install(t, ".local/bin", "ok")
	if got := t3Detect(f.machine()); got != (t3Status{State: t3Found, Version: "0.1.8"}) {
		t.Errorf("both: %+v", got)
	}
	f.app = false
	if got := t3Detect(f.machine()); got != (t3Status{State: t3CliOnly, Version: "0.1.8"}) {
		t.Errorf("t3 alone: %+v", got)
	}
	if got := f.runs(); !reflect.DeepEqual(got, [][]string{{"--version"}, {"--version"}}) {
		t.Errorf("only --version to find it: %q", got)
	}
	f.setMode(t, "fail")
	if got := t3Detect(f.machine()); got != (t3Status{State: t3CliOnly}) {
		t.Errorf("a t3 that fails is still there: %+v", got)
	}
	want := map[string]any{"state": "appOnly", "version": "", "hint": "In T3 Code, open Settings → About and install the t3 command."}
	if got := t3JSON(t3Status{State: t3AppOnlyIs}); !reflect.DeepEqual(got, want) {
		t.Errorf("appOnly's answer: %v", got)
	}
	if got := t3JSON(t3Status{State: t3None})["hint"]; got != "Install T3 Code to open this hire's folder there (t3.codes)." {
		t.Errorf("none's hint: %v", got)
	}
}

func TestT3IsLookedForWhereT3CodeLinksItThenThePath(t *testing.T) {
	m := t3Machine{Home: "/home/ada", GOOS: "linux", Path: "/usr/bin:/bin"}
	if got, want := t3Folders(m), []string{"/home/ada/.local/bin", "/home/ada/bin", "/usr/bin", "/bin"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Linux: %q", got)
	}
	m.GOOS = "windows"
	if got := t3Folders(m)[0]; got != filepath.Join("/home/ada", ".t3", "bin") {
		t.Errorf("Windows looks in the folder T3 Code adds to the PATH first: %q", got)
	}
}

func TestT3ACheckIsKeptThirtySecondsUnlessAskedFresh(t *testing.T) {
	f := newT3Fake(t)
	f.install(t, "bin", "ok")
	m := f.machine()
	t3StatusNow(true, m)
	t3StatusNow(false, m)
	f.clock = f.clock.Add(29 * time.Second)
	t3StatusNow(false, m)
	if n := len(f.runs()); n != 1 {
		t.Errorf("kept: %d runs", n)
	}
	t3StatusNow(true, m)
	f.clock = f.clock.Add(31 * time.Second)
	t3StatusNow(false, m)
	if n := len(f.runs()); n != 3 {
		t.Errorf("fresh, then older than 30 s: %d runs, want 3", n)
	}
}

func TestT3TheFolderComesFromTheRegistryAndEachRefusal(t *testing.T) {
	f := newT3Fake(t)
	project, worktree := f.folder(t, "Projects/app"), f.folder(t, "Projects/app-hires/ada-1a2b3c")
	if got, _, _ := t3Folder(t3Hire(project, worktree), f.home); got != worktree {
		t.Errorf("the worktree first: %q", got)
	}
	if got, _, _ := t3Folder(t3Hire(project, ""), f.home); got != project {
		t.Errorf("else the folder: %q", got)
	}
	if got := t3Title(t3Hire(project, "")); got != "Ada · hire/ada-1a2b3c" {
		t.Errorf("title: %q", got)
	}
	outside := t.TempDir()
	link := filepath.Join(f.home, "Projects", "away")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		folder string
		status int
		why    string
	}{
		{"", http.StatusBadRequest, "This hire has no folder to open."},
		{filepath.Join(f.home, "gone"), http.StatusConflict, "This hire's folder isn't there any more."},
		{f.home, http.StatusConflict, "That's your whole home folder: T3 Code opens a hire's own folder only."},
		{f.home + "/", http.StatusConflict, "That's your whole home folder: T3 Code opens a hire's own folder only."},
		{f.folder(t, ".hidden"), http.StatusConflict, "A hidden folder doesn't open in T3 Code."},
		{f.folder(t, "Projects/.cache/app"), http.StatusConflict, "A hidden folder doesn't open in T3 Code."},
		{outside, http.StatusConflict, "Only a folder inside your home folder opens in T3 Code."},
		{link, http.StatusConflict, "Only a folder inside your home folder opens in T3 Code."},
	} {
		if _, status, why := t3Folder(t3Hire(c.folder, ""), f.home); status != c.status || why != c.why {
			t.Errorf("%q: %d %q, want %d %q", c.folder, status, why, c.status, c.why)
		}
	}
	// A refused folder runs nothing, whatever is installed.
	f.install(t, ".local/bin", "ok")
	f.app = true
	if status, _ := t3Open(t3Hire(f.home, ""), t3Status{State: t3Found}, f.machine()); status != http.StatusConflict || len(f.runs()) != 0 || f.opened != 0 {
		t.Errorf("the home folder: %d, %d runs, opened %d", status, len(f.runs()), f.opened)
	}
}

func TestT3FoundOpensTheAppThenAThreadNeverThroughAShell(t *testing.T) {
	f := newT3Fake(t)
	f.install(t, ".local/bin", "ok")
	f.app = true
	tricky := f.folder(t, "Projects/My App; touch pwned")
	status, answer := t3Open(t3Hire(tricky, ""), t3Status{State: t3Found}, f.machine())
	if status != http.StatusOK || !reflect.DeepEqual(answer, map[string]any{"ok": true, "opened": "thread"}) {
		t.Fatalf("%d %v", status, answer)
	}
	if got := f.runs(); !reflect.DeepEqual(got, [][]string{{"app", tricky}}) || f.opened != 1 {
		t.Errorf("one argument for the folder, as it is: %q, opened %d", got, f.opened)
	}
	if _, err := os.Stat(filepath.Join(f.home, "Projects", "pwned")); err == nil {
		t.Error("the folder's name was run as a command")
	}
}

func TestT3FoundWaitsForTheAppThenSaysItsFirstLine(t *testing.T) {
	f := newT3Fake(t)
	f.install(t, ".local/bin", "late")
	f.app = true
	project := f.folder(t, "Projects/app")
	if status, _ := t3Open(t3Hire(project, ""), t3Status{State: t3Found}, f.machine()); status != http.StatusOK || len(f.runs()) != 3 {
		t.Errorf("waited for the app: %d after %d runs", status, len(f.runs()))
	}
	f.setMode(t, "fail")
	start := f.clock
	status, answer := t3Open(t3Hire(project, ""), t3Status{State: t3Found}, f.machine())
	if status != http.StatusBadGateway || answer != "Could not reach the T3 Code desktop app." {
		t.Errorf("a t3 error: %d %q", status, answer)
	}
	if waited := f.clock.Sub(start); waited > t3OpenWithin+time.Second {
		t.Errorf("waited %v", waited)
	}
	for _, run := range f.runs() {
		if !reflect.DeepEqual(run, []string{"app", project}) {
			t.Errorf("only t3 app <folder>: %q", run)
		}
	}
}

func TestT3CliOnlyAddsAProjectAndAppOnlyOrNoneSayWhatToInstall(t *testing.T) {
	f := newT3Fake(t)
	f.install(t, ".local/bin", "ok")
	worktree := f.folder(t, "Projects/app-hires/ada 1a2b3c")
	status, answer := t3Open(t3Hire(f.folder(t, "Projects/app"), worktree), t3Status{State: t3CliOnly}, f.machine())
	if status != http.StatusOK || !reflect.DeepEqual(answer, map[string]any{"ok": true, "opened": "project"}) {
		t.Errorf("%d %v", status, answer)
	}
	if got := f.runs(); !reflect.DeepEqual(got, [][]string{{"project", "add", worktree, "--title", "Ada · hire/ada-1a2b3c"}}) || f.opened != 0 {
		t.Errorf("project add's arguments: %q", got)
	}
	f.setMode(t, "exists")
	if status, _ := t3Open(t3Hire(worktree, ""), t3Status{State: t3CliOnly}, f.machine()); status != http.StatusOK {
		t.Errorf("already a project: %d", status)
	}
	f.setMode(t, "fail")
	if status, answer := t3Open(t3Hire(worktree, ""), t3Status{State: t3CliOnly}, f.machine()); status != http.StatusBadGateway || answer != "Could not reach the T3 Code desktop app." {
		t.Errorf("an error: %d %q", status, answer)
	}
	runs := len(f.runs())
	for state, want := range map[string]string{t3AppOnlyIs: t3AppOnly, t3None: t3NoneHint} {
		if status, answer := t3Open(t3Hire(worktree, ""), t3Status{State: state}, f.machine()); status != http.StatusConflict || answer != want {
			t.Errorf("%s: %d %q", state, status, answer)
		}
	}
	if len(f.runs()) != runs || f.opened != 0 {
		t.Error("appOnly and none run nothing")
	}
}

// The routes: the page on this computer only; the GM, a tool and a hire are refused (routes.json, and every row
// through the gate in routes_test.go); the office's, only to its own window, never a phone.
func TestT3RoutesArePageOnlyAndTheOfficesItsWindowOnly(t *testing.T) {
	for _, path := range []string{"/setup/t3code", "/setup/t3code/open"} {
		method := http.MethodGet
		if strings.HasSuffix(path, "/open") {
			method = http.MethodPost
		}
		if row := routeFor(method, path); row == nil || !reflect.DeepEqual(row.callers, []string{"page"}) {
			t.Errorf("%s %s: %+v", method, path, row)
		}
	}
	d := newTestDeck(t)
	phone := pair(t, d)
	for _, c := range []struct {
		method, target string
	}{{"GET", "/t3code"}, {"POST", "/t3code/open"}} {
		if got := serve(t, d, call{method: c.method, target: c.target + "?session=s1&t=" + phone, host: deckOnWiFi, from: aPhone}); got.Code != http.StatusForbidden {
			t.Errorf("%s %s from a phone: %d, want 403", c.method, c.target, got.Code)
		}
		window := serve(t, d, call{method: c.method, target: c.target + "?session=s1&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
		if window.Code == http.StatusForbidden || window.Code == http.StatusUnauthorized {
			t.Errorf("%s %s from the office's window: %d", c.method, c.target, window.Code)
		}
		if c.method == "GET" {
			var answer map[string]any
			if json.Unmarshal(window.Body.Bytes(), &answer) != nil || answer["state"] != "none" || answer["hire"] != nil {
				t.Errorf("the window's answer, with no T3 Code and no hire in s1: %s", window.Body)
			}
		} else if window.Code != http.StatusNotFound {
			t.Errorf("no hire in that session: %d, want 404", window.Code)
		}
	}
	// The page's open takes an id, and a hire not on the team is a 404 before anything runs.
	if got := serve(t, d, call{method: "POST", target: "/setup/t3code/open", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}, body: `{"id":"nobody","folder":"/etc"}`}); got.Code != http.StatusNotFound {
		t.Errorf("an unknown id: %d", got.Code)
	}
}
