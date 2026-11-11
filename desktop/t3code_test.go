package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
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
	home, calls, mode, threads, next string
	opened                           int
	clock                            time.Time
	app                              bool
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
	// Each test starts with no check kept: an earlier test's, at the same fake time, would be taken as new.
	t3Kept.Lock()
	t3Kept.status, t3Kept.at = t3Status{}, time.Time{}
	t3Kept.Unlock()
	return &t3Fake{home: home, calls: filepath.Join(home, "calls"), mode: filepath.Join(home, "mode"), threads: filepath.Join(home, "threads"),
		next: filepath.Join(home, "next"), clock: time.Unix(1_000_000, 0)}
}

// install puts the fake t3 in folder, under the home folder, behaving as mode says: "ok", "fail" (always, on two
// lines), "exists" (project add says it's there), "late" (t3 app fails twice), "renderer" (t3 app fails twice as
// T3 Code does while its window isn't up), "slow" (its version takes a second) or "unsure" (t3 app says
// thread-open-failed, as T3 Code 0.1.8 does even when it opened one). thread list prints the threads file, and
// fails as T3 Code does when it isn't running without one; a next file is what t3 app makes the threads. It
// says its version as the real one does: "t3 v0.1.8".
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
  [ "$mode" = "slow" ] && sleep 1
  echo "t3 v0.1.8"; exit 0
fi
if [ "$1" = "thread" ]; then
  [ -f '` + f.threads + `' ] || { echo "No running T3 Code server uses ~/.t3. Start T3 Code, then retry." >&2; exit 1; }
  cat '` + f.threads + `'; exit 0
fi
[ -f '` + f.next + `' ] && mv '` + f.next + `' '` + f.threads + `'
apps=$(grep -c '^app$' '` + f.calls + `')
case "$mode" in
  unsure) echo "[01:33:17.503] ERROR (#2): DesktopAppRequestFailedError: T3 Code could not open $2 (thread-open-failed)." >&2
    echo "    at Object.<anonymous> (file:///binCli.mjs:8963:34)" >&2; exit 1 ;;
  fail) printf '  Could not reach the T3 Code desktop app.  \nmore detail\n' >&2; exit 1 ;;
  exists) echo "Project already exists: $3" >&2; exit 1 ;;
  late) [ "$apps" -le 2 ] && { echo "not yet" >&2; exit 1; }; exit 0 ;;
  renderer) [ "$apps" -le 2 ] && { echo "T3 Code could not open $2 (renderer-unavailable)." >&2; exit 1; }; exit 0 ;;
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

// t3List is the run that lists folder's threads.
func t3List(folder string) []string { return []string{"thread", "list", "--project", folder, "--json"} }

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
	if got := t3Detect(f.machine()); got != (t3Status{State: t3Found, Version: "v0.1.8"}) {
		t.Errorf("both: %+v", got)
	}
	f.app = false
	if got := t3Detect(f.machine()); got != (t3Status{State: t3CliOnly, Version: "v0.1.8"}) {
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
	// The folders are joined and the PATH split by this computer's own rules (its separator, its list
	// separator), as on the real machine, whose GOOS is always this one's: a Linux machine can't be played on
	// Windows, so there the PATH is Windows'.
	m := t3Machine{Home: "/home/ada", GOOS: "linux", Path: "/usr/bin:/bin"}
	if runtime.GOOS == "windows" {
		m = t3Machine{Home: `C:\Users\ada`, GOOS: "windows", Path: `C:\Windows;C:\bin`, Getenv: func(string) string { return "" }}
		if got, want := t3Folders(m), []string{`C:\Users\ada\.t3\bin`, `C:\Windows`, `C:\bin`}; !reflect.DeepEqual(got, want) {
			t.Errorf("Windows, then its PATH: %q", got)
		}
		m.Home = "/home/ada"
	} else if got, want := t3Folders(m), []string{"/home/ada/.local/bin", "/home/ada/bin", "/usr/bin", "/bin"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Linux: %q", got)
	}
	m.GOOS = "windows"
	m.Getenv = func(string) string { return "" }
	if got := t3Folders(m)[0]; got != filepath.Join("/home/ada", ".t3", "bin") {
		t.Errorf("Windows looks in the folder T3 Code adds to the PATH first: %q", got)
	}
	m.Getenv = func(name string) string {
		if name == "T3CODE_HOME" {
			return `D:\t3home`
		}
		return ""
	}
	if got := t3Folders(m)[:2]; !reflect.DeepEqual(got, []string{filepath.Join(`D:\t3home`, "bin"), filepath.Join("/home/ada", ".t3", "bin")}) {
		t.Errorf("with T3CODE_HOME set, its bin first: %q", got)
	}
}

func TestT3ACheckIsKeptThirtySecondsUnlessAskedFresh(t *testing.T) {
	f := newT3Fake(t)
	f.install(t, "bin", "ok")
	m := f.machine()
	t3StatusNow(true, m)
	t3StatusNow(false, m)
	f.clock = f.clock.Add(time.Second)
	t3StatusNow(true, m)
	if n := len(f.runs()); n != 1 {
		t.Errorf("asked fresh within 2 s: %d runs, want 1", n)
	}
	f.clock = f.clock.Add(28 * time.Second)
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
	if got := f.runs(); !reflect.DeepEqual(got, [][]string{t3List(tricky), {"app", tricky}}) || f.opened != 1 {
		t.Errorf("its threads before, then one argument for the folder, as it is: %q, opened %d", got, f.opened)
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
	want := [][]string{t3List(project), {"app", project}, {"app", project}, {"app", project}}
	if status, _ := t3Open(t3Hire(project, ""), t3Status{State: t3Found}, f.machine()); status != http.StatusOK || !reflect.DeepEqual(f.runs(), want) {
		t.Errorf("waited for the app: %d after %q", status, f.runs())
	}
	if err := os.Remove(f.calls); err != nil {
		t.Fatal(err)
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
	for i, run := range f.runs() {
		if i == 0 && !reflect.DeepEqual(run, t3List(project)) || i > 0 && !reflect.DeepEqual(run, []string{"app", project}) {
			t.Errorf("its threads, then only t3 app <folder>: %q", run)
		}
	}
}

// T3 Code says renderer-unavailable while its window isn't up: tried again, as any failure but thread-open-failed,
// which is matched with its parentheses so a folder's name can't pass for it.
func TestT3RendererUnavailableIsTriedAgain(t *testing.T) {
	f := newT3Fake(t)
	f.install(t, ".local/bin", "renderer")
	f.app = true
	project := f.folder(t, "Projects/thread-open-failed")
	status, answer := t3Open(t3Hire(project, ""), t3Status{State: t3Found}, f.machine())
	if status != http.StatusOK || !reflect.DeepEqual(answer, map[string]any{"ok": true, "opened": "thread"}) {
		t.Errorf("%d %v", status, answer)
	}
	if got, want := f.runs(), [][]string{t3List(project), {"app", project}, {"app", project}, {"app", project}}; !reflect.DeepEqual(got, want) {
		t.Errorf("three t3 app, no list after: %q", got)
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

func TestT3OneCheckAtATime(t *testing.T) {
	f := newT3Fake(t)
	f.install(t, ".local/bin", "slow")
	m := f.machine()
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() { defer wg.Done(); t3StatusNow(true, m) }()
	}
	wg.Wait()
	if n := len(f.runs()); n != 1 {
		t.Errorf("three fresh asks at once: %d runs of t3 --version, want 1", n)
	}
}

// T3 Code 0.1.8's t3 app says thread-open-failed even when it opened one, and that thread is a draft, which isn't
// listed. The folder's threads are listed before t3 app and once after, and nothing is tried again: a thread that's
// new since, made in the last 15 s, is "thread"; a list without one, "unsure"; a list that fails, T3 Code's error
// without its log prefix.
func TestT3AFalseFailureIsCheckedInTheThreadList(t *testing.T) {
	f := newT3Fake(t)
	f.install(t, ".local/bin", "unsure")
	f.app = true
	project := f.folder(t, "Projects/My App; touch pwned")
	// listed is a thread list: each thread's id, made that many seconds ago.
	listed := func(ids ...any) string {
		items := []string{}
		for i := 0; i < len(ids); i += 2 {
			at := f.clock.Add(-time.Duration(ids[i+1].(int)) * time.Second).UTC().Format("2006-01-02T15:04:05.000Z")
			items = append(items, `{"threadId":"`+ids[i].(string)+`","createdAt":"`+at+`"}`)
		}
		return "[" + strings.Join(items, ",") + "]"
	}
	open := func(before, after string) (int, any) {
		_ = os.Remove(f.calls)
		for file, list := range map[string]string{f.threads: before, f.next: after} {
			_ = os.Remove(file)
			if list != "" {
				if err := os.WriteFile(file, []byte(list), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
		return t3Open(t3Hire(project, ""), t3Status{State: t3Found}, f.machine())
	}
	want := [][]string{t3List(project), {"app", project}, t3List(project)}
	for _, c := range []struct{ before, after, opened, why string }{
		{listed("t-old", 3600), listed("t-old", 3600, "t1", 4), "thread", "a new thread from 4 s ago"},
		{"", listed("t-old", 3600, "t1", 4), "thread", "no list before: one from 4 s ago"},
		{"[]", "[]", "unsure", "the draft, not listed"},
		{listed("t1", 4), listed("t1", 4), "unsure", "a thread that was there before"},
		{listed("t-old", 3600), listed("t-old", 3600, "t2", 20), "unsure", "a new one, but from 20 s ago"},
	} {
		if status, answer := open(c.before, c.after); status != http.StatusOK || !reflect.DeepEqual(answer, map[string]any{"ok": true, "opened": c.opened}) {
			t.Errorf("%s: %d %v", c.why, status, answer)
		}
		if got := f.runs(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: the list before, t3 app once, the list after once: %q", c.why, got)
		}
	}
	status, answer := open("", "")
	if line := "T3 Code could not open " + project + " (thread-open-failed)."; status != http.StatusBadGateway || answer != clip(line, 200) {
		t.Errorf("the list exits 1: %d %q, want the line without T3 Code's log prefix", status, answer)
	}
	if got := f.runs(); !reflect.DeepEqual(got, want) {
		t.Errorf("not tried again: %q", got)
	}
	if status, _ := open("not json", ""); status != http.StatusBadGateway {
		t.Errorf("a list that isn't a list: %d", status)
	}
	if _, err := os.Stat(filepath.Join(f.home, "Projects", "pwned")); err == nil {
		t.Error("the folder's name was run as a command")
	}
}

// Every t3 error is said without T3 Code's log prefix; an error without one, as it is.
func TestT3CodesLogPrefixIsLeftOut(t *testing.T) {
	logged := t3Ran{Ended: true, Code: 1, Error: "[01:33:17.503] ERROR (#12): DesktopAppRequestFailedError: T3 Code could not open /a (renderer-unavailable).\n  at x"}
	if _, answer := t3Failed(logged); answer != "T3 Code could not open /a (renderer-unavailable)." {
		t.Errorf("logged: %q", answer)
	}
	if _, answer := t3Failed(t3Ran{Ended: true, Code: 1, Error: "[note] Could not reach the T3 Code desktop app."}); answer != "[note] Could not reach the T3 Code desktop app." {
		t.Errorf("plain: %q", answer)
	}
}

// A second open for a hire while one runs is a 409, and runs nothing; another hire's isn't held up.
func TestT3OneOpenAtATimeForAHire(t *testing.T) {
	f := newT3Fake(t)
	f.install(t, ".local/bin", "ok")
	f.app = true
	project := f.folder(t, "Projects/app")
	started, release := make(chan struct{}), make(chan struct{})
	m := f.machine()
	var mu sync.Mutex
	apps := 0
	m.Run = func(program string, args []string, timeout time.Duration) t3Ran {
		if args[0] == "app" {
			mu.Lock()
			apps++
			first := apps == 1
			mu.Unlock()
			if first {
				close(started)
				<-release
			}
		}
		return runT3(program, args, timeout)
	}
	m.OpenApp = func(string) {}
	done := make(chan int)
	go func() { status, _ := t3OpenOnce(t3Hire(project, ""), m); done <- status }()
	<-started
	if status, answer := t3OpenOnce(t3Hire(project, ""), m); status != http.StatusConflict || answer != "Already opening T3 Code for this hire." {
		t.Errorf("a second open while one runs: %d %q", status, answer)
	}
	other := t3Hire(project, "")
	other.ID = "h2"
	if status, _ := t3OpenOnce(other, m); status != http.StatusOK {
		t.Errorf("another hire's: %d", status)
	}
	close(release)
	if status := <-done; status != http.StatusOK {
		t.Errorf("the first: %d", status)
	}
	if status, _ := t3OpenOnce(t3Hire(project, ""), m); status != http.StatusOK {
		t.Errorf("once it's done, it opens again: %d", status)
	}
}

// t3 runs with its four argument lists only, and never through a shell: Windows' .cmd goes through cmdLine alone.
func TestT3RunsWithItsFourArgumentListsNeverAShell(t *testing.T) {
	read := func(name string) string {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		code := []string{}
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "//") {
				code = append(code, line)
			}
		}
		return strings.Join(code, "\n")
	}
	source := read("t3code.go")
	got := regexp.MustCompile(`m\.Run\(program, (\[\]string\{[^}]*\})`).FindAllStringSubmatch(source, -1)
	lists := []string{}
	for _, call := range got {
		lists = append(lists, call[1])
	}
	want := []string{`[]string{"--version"}`, `[]string{"project", "add", folder, "--title", t3Title(h)}`, `[]string{"app", folder}`,
		`[]string{"thread", "list", "--project", folder, "--json"}`}
	if !reflect.DeepEqual(lists, want) || strings.Count(source, "m.Run(") != len(want) {
		t.Errorf("t3's argument lists: %q", lists)
	}
	for _, name := range []string{"t3code.go", "opencode_other.go", "opencode_windows.go"} {
		code := read(name)
		for _, never := range []string{`"sh"`, `/bin/sh`, `"-c"`, `"/c"`, "bash", "zsh", "-lc", "cmd /c", "powershell", "npx", "npm "} {
			if strings.Contains(strings.ToLower(code), never) {
				t.Errorf("%s has %s", name, never)
			}
		}
	}
	if windows := read("opencode_windows.go"); !strings.Contains(windows, "CmdLine: cmdLine(shell, program, args)") || strings.Count(windows, "CmdLine:") != 1 {
		t.Error("a .cmd's command line is cmdLine's, and only that")
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
	// T3 Code on (T796): off, every route is a 404; see TestT3RoutesAnswerNothingWhileT3CodeIsOff.
	d.mu.Lock()
	d.config.T3CodeEnabled = true
	d.mu.Unlock()
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

// The T3 Code gate (T796): off, which it is until chosen, none of the four routes answers, and nothing of T3 Code's
// is looked for or run; on, they answer as above. The page's and the office's, both ways.
func TestT3RoutesAnswerNothingWhileT3CodeIsOff(t *testing.T) {
	d := newTestDeck(t)
	looked := 0
	kept := newT3Machine
	newT3Machine = func() t3Machine {
		looked++
		return kept()
	}
	t.Cleanup(func() { newT3Machine = kept })
	calls := []call{
		{method: "GET", target: "/t3code?session=s1&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer},
		{method: "POST", target: "/t3code/open?session=s1&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer},
		{method: "GET", target: "/setup/t3code?fresh=1", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}},
		{method: "POST", target: "/setup/t3code/open", host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}, body: `{"id":"nobody"}`},
	}
	for _, c := range calls {
		if got := serve(t, d, c); got.Code != http.StatusNotFound || !strings.Contains(got.Body.String(), t3Off) {
			t.Errorf("off: %s %s: %d %q", c.method, c.target, got.Code, got.Body)
		}
	}
	if looked != 0 {
		t.Fatalf("off, T3 Code was looked for %d times", looked)
	}
	d.mu.Lock()
	d.config.T3CodeEnabled = true
	d.mu.Unlock()
	for _, c := range calls {
		if got := serve(t, d, c); strings.Contains(got.Body.String(), t3Off) {
			t.Errorf("on: %s %s: %d %q", c.method, c.target, got.Code, got.Body)
		}
	}
	if got := serve(t, d, calls[2]); got.Code != http.StatusOK || looked == 0 {
		t.Errorf("on, the page's look: %d, looked %d", got.Code, looked)
	}
}
