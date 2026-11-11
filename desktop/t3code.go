package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Open in T3 Code (T792), as the Mac app's T3Code.swift: a hire's folder (its
// worktree, else the folder it was hired into) opened in T3 Code, a separate
// app on this computer with its own agent. Smolcorp only runs what's
// installed: never npx, an install or a download, and never the hire's
// terminal or agent. `t3` only ever runs with `--version`, `app <folder>` or
// `project add <folder> --title <title>`, and `thread list --project <folder>
// --json`, before `t3 app` and again after one that says it failed, to see
// whether it did; its arguments always as a list. On Windows
// `t3` is a .cmd, which only cmd.exe starts: its command line is escaped as
// OpenCode's is (cmdLine), so a folder's name is never read as a command.

const (
	t3VersionTimeout = 3 * time.Second
	t3OpenWithin     = 10 * time.Second
	// How long a check is kept: the page asks again at most this often, or when the button's menu opens. Even
	// asked fresh, a check newer than t3FreshFor is the answer.
	t3KeptFor  = 30 * time.Second
	t3FreshFor = 2 * time.Second
	// How new a thread of the folder's must be to count as the one `t3 app` opened: one that wasn't listed before it.
	t3ThreadWithin = 15 * time.Second
	// What a run may print that's kept: a thread list's JSON fits.
	t3KeptOutput     = 1 << 20
	t3AlreadyOpening = "Already opening T3 Code for this hire."
	t3AppOnly        = "In T3 Code, open Settings → About and install the t3 command."
	t3NoneHint       = "Install T3 Code to open this hire's folder there (t3.codes)."
	t3Found          = "found"
	t3AppOnlyIs      = "appOnly"
	t3CliOnly        = "cliOnly"
	t3None           = "none"
)

// t3Status is what's installed: found (the app and t3), appOnly, cliOnly or none, and what `t3 --version` said.
type t3Status struct{ State, Version string }

// t3Ran is one run of t3: whether it ended in time, its exit code, and what it printed.
type t3Ran struct {
	Ended         bool
	Code          int
	Output, Error string
}

// t3Machine is what the checks and the open use on this computer. Tests give their own: a home folder with a
// fake t3, and no app.
type t3Machine struct {
	Home, GOOS, Path string
	Getenv           func(string) string
	// App is the desktop app's launcher ("" without), and OpenApp starts it.
	App     func() string
	OpenApp func(launcher string)
	Run     func(program string, args []string, timeout time.Duration) t3Ran
	// Feed runs program as Run does, with input on its standard input (a hire in T3 Code's mail, t3desk.go).
	Feed func(program string, args []string, input []byte, timeout time.Duration) t3Ran
	// OpenURL hands a link to this computer's opener (a hire's thread in T3 Code, t3desk.go): never a shell.
	OpenURL func(link string) error
	Pause   func(time.Duration)
	Now     func() time.Time
}

// newT3Machine is the machine the routes use: this computer's. The tests' own has neither t3 nor the app, so
// no test ever runs the real ones.
var newT3Machine = defaultT3Machine

func defaultT3Machine() t3Machine {
	home, _ := os.UserHomeDir()
	m := t3Machine{Home: home, GOOS: runtime.GOOS, Path: os.Getenv("PATH"), Getenv: os.Getenv, Run: runT3, Feed: feedT3,
		OpenURL: func(link string) error { return openURL(link) }, Pause: time.Sleep, Now: time.Now}
	m.App = func() string { return t3AppLauncher(m) }
	m.OpenApp = func(launcher string) {
		cmd := exec.Command(launcher)
		detach(cmd)
		if cmd.Start() == nil {
			go func() { _ = cmd.Wait() }()
		}
	}
	return m
}

// t3Folders is where t3 is looked for: where T3 Code links it (~/.local/bin, ~/bin; on Windows, the folder it
// adds to your PATH, which this app's own PATH may predate: bin in T3 Code's home, %T3CODE_HOME% where it's
// set, else ~/.t3), then the PATH.
func t3Folders(m t3Machine) []string {
	first := []string{filepath.Join(m.Home, ".local", "bin"), filepath.Join(m.Home, "bin")}
	if m.GOOS == "windows" {
		first = []string{filepath.Join(m.Home, ".t3", "bin")}
		if m.Getenv != nil && m.Getenv("T3CODE_HOME") != "" {
			first = append([]string{filepath.Join(m.Getenv("T3CODE_HOME"), "bin")}, first...)
		}
	}
	return append(first, filepath.SplitList(m.Path)...)
}

// t3Program is the t3 command, or "".
func t3Program(m t3Machine) string {
	names := []string{"t3"}
	if m.GOOS == "windows" {
		names = []string{"t3.cmd", "t3.exe"}
	}
	for _, folder := range t3Folders(m) {
		for _, name := range names {
			candidate := filepath.Join(folder, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() && (m.GOOS == "windows" || info.Mode()&0o111 != 0) {
				return candidate
			}
		}
	}
	return ""
}

// t3AppLauncher is T3 Code's desktop app, as its builds install it: on Linux the .deb's t3code (/usr/bin, or
// /opt/<product>), Nix's t3code-desktop, or an AppImage (T3-Code-<version>-<arch>.AppImage) in ~/Applications
// or ~/.local/bin; on Windows, the installer's T3 Code (Beta).exe, for you or for everyone. "" without.
func t3AppLauncher(m t3Machine) string {
	var candidates []string
	switch m.GOOS {
	case "linux":
		for _, folder := range filepath.SplitList(m.Path) {
			candidates = append(candidates, filepath.Join(folder, "t3code-desktop"), filepath.Join(folder, "t3code"))
		}
		candidates = append(candidates, "/usr/bin/t3code", "/opt/T3 Code (Beta)/t3code", "/opt/T3 Code/t3code")
		for _, folder := range []string{filepath.Join(m.Home, "Applications"), filepath.Join(m.Home, ".local", "bin")} {
			images, _ := filepath.Glob(filepath.Join(folder, "T3-Code-*.AppImage"))
			candidates = append(candidates, images...)
		}
	case "windows":
		for _, base := range []string{m.Getenv("LOCALAPPDATA") + `\Programs`, m.Getenv("ProgramFiles")} {
			if base == `\Programs` || base == "" {
				continue
			}
			for _, product := range []string{"T3 Code (Beta)", "T3 Code"} {
				candidates = append(candidates, filepath.Join(base, product, product+".exe"))
			}
		}
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// t3Detect is what's installed now, and t3's version. It runs t3.
func t3Detect(m t3Machine) t3Status {
	app := m.App() != ""
	program := t3Program(m)
	if program == "" {
		if app {
			return t3Status{State: t3AppOnlyIs}
		}
		return t3Status{State: t3None}
	}
	version := ""
	if ran := m.Run(program, []string{"--version"}, t3VersionTimeout); ran.Ended && ran.Code == 0 {
		// It says "t3 v0.1.8": the version is what follows "t3 ".
		version = clip(strings.TrimPrefix(firstLine(ran.Output), "t3 "), 40)
	}
	if app {
		return t3Status{State: t3Found, Version: version}
	}
	return t3Status{State: t3CliOnly, Version: version}
}

var t3Kept struct {
	sync.Mutex
	status t3Status
	at     time.Time
}

// t3Checking is held while t3 --version runs: one check at a time.
var t3Checking sync.Mutex

// t3StatusNow is the last check if it's newer than t3KeptFor (t3FreshFor, asked fresh); otherwise a new one, one
// at a time: a check that ends while this one waits for it is the answer.
func t3StatusNow(fresh bool, m t3Machine) t3Status {
	newEnough := func() (t3Status, bool) {
		t3Kept.Lock()
		defer t3Kept.Unlock()
		limit := t3KeptFor
		if fresh {
			limit = t3FreshFor
		}
		age := m.Now().Sub(t3Kept.at)
		return t3Kept.status, !t3Kept.at.IsZero() && age >= 0 && age < limit
	}
	if status, ok := newEnough(); ok {
		return status
	}
	t3Checking.Lock()
	defer t3Checking.Unlock()
	if status, ok := newEnough(); ok {
		return status
	}
	status := t3Detect(m)
	t3Kept.Lock()
	t3Kept.status, t3Kept.at = status, m.Now()
	t3Kept.Unlock()
	return status
}

// t3JSON is GET /setup/t3code's answer, and the office's: the state, and the line a greyed button shows.
func t3JSON(s t3Status) map[string]any {
	hint := ""
	switch s.State {
	case t3AppOnlyIs:
		hint = t3AppOnly
	case t3None:
		hint = t3NoneHint
	}
	return map[string]any{"state": s.State, "version": s.Version, "hint": hint}
}

// t3Folder is the folder to open for a hire: its worktree, else its folder. Only a folder that's there, inside
// the home folder, not the home folder itself and not hidden (nor inside a hidden one). Else a status and why.
func t3Folder(h Hire, home string) (string, int, string) {
	given := cmpOr(h.Worktree, h.Folder)
	if given == "" {
		return "", http.StatusBadRequest, "This hire has no folder to open."
	}
	if info, err := os.Stat(given); err != nil || !info.IsDir() {
		return "", http.StatusConflict, "This hire's folder isn't there any more."
	}
	real := func(p string) string {
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			p = resolved
		}
		abs, _ := filepath.Abs(p)
		return filepath.Clean(abs)
	}
	folder, base := real(given), real(home)
	rel, err := filepath.Rel(base, folder)
	switch {
	case err == nil && rel == ".":
		return "", http.StatusConflict, "That's your whole home folder: T3 Code opens a hire's own folder only."
	case err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)):
		return "", http.StatusConflict, "Only a folder inside your home folder opens in T3 Code."
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") {
			return "", http.StatusConflict, "A hidden folder doesn't open in T3 Code."
		}
	}
	return folder, 0, ""
}

// t3Title is how a hire's project is named in T3 Code: "<name> · <branch>".
func t3Title(h Hire) string {
	name := cmpOr(h.Nickname, h.Name)
	if h.Branch == "" {
		return name
	}
	return name + " · " + h.Branch
}

// t3Open opens a hire's folder in T3 Code as status says it's installed: 200 with {ok, opened}, or a status and
// its sentence. It runs t3 and may wait for the app.
func t3Open(h Hire, status t3Status, m t3Machine) (int, any) {
	// A hire that runs in T3 Code (t3desk.go): its own thread there, by T3 Code's link, not a new one.
	if h.Engine == t3Engine {
		return t3OpenThread(h, status, m)
	}
	folder, refused, why := t3Folder(h, m.Home)
	if refused != 0 {
		return refused, why
	}
	switch status.State {
	case t3None:
		return http.StatusConflict, t3NoneHint
	case t3AppOnlyIs:
		return http.StatusConflict, t3AppOnly
	}
	program := t3Program(m)
	if program == "" {
		if status.State == t3Found {
			return http.StatusConflict, t3AppOnly
		}
		return http.StatusConflict, t3NoneHint
	}
	if status.State == t3CliOnly {
		ran := m.Run(program, []string{"project", "add", folder, "--title", t3Title(h)}, t3OpenWithin)
		// A folder that's already one of its projects: it's there, as asked.
		if ran.Ended && (ran.Code == 0 || strings.Contains(strings.ToLower(ran.Output+ran.Error), "already exists")) {
			return http.StatusOK, map[string]any{"ok": true, "opened": "project"}
		}
		return t3Failed(ran)
	}
	// The folder's threads before: only one that isn't among them can be the one `t3 app` opens. None when T3 Code
	// isn't running yet or the folder isn't one of its projects.
	before := map[string]bool{}
	listed, _ := t3Threads(program, folder, m)
	for _, thread := range listed {
		before[thread.ThreadID] = true
	}
	// The app, started, then `t3 app` once it answers: tried again until it does, for t3OpenWithin at most.
	if launcher := m.App(); launcher != "" {
		m.OpenApp(launcher)
	}
	until := m.Now().Add(t3OpenWithin)
	var ran t3Ran
	for {
		ran = m.Run(program, []string{"app", folder}, max(time.Second, until.Sub(m.Now())))
		if ran.Ended && ran.Code == 0 {
			return http.StatusOK, map[string]any{"ok": true, "opened": "thread"}
		}
		// The app answered, and said it couldn't open a thread: T3 Code 0.1.8 says so even when it did, and its new
		// thread is a draft, which isn't listed until it's used. Not tried again, it would open another. A thread of
		// the folder's that's just appeared says it did; a list without one, that the folder is open as a project
		// and T3 Code got as far as its thread; a list that fails, the error.
		if ran.Ended && strings.Contains(ran.Output+ran.Error, "(thread-open-failed)") {
			after, ok := t3Threads(program, folder, m)
			if !ok {
				return t3Failed(ran)
			}
			opened, since := "unsure", m.Now().Add(-t3ThreadWithin)
			for _, thread := range after {
				if made, err := time.Parse(time.RFC3339Nano, thread.CreatedAt); err == nil && !before[thread.ThreadID] && !made.Before(since) {
					opened = "thread"
				}
			}
			return http.StatusOK, map[string]any{"ok": true, "opened": opened}
		}
		m.Pause(time.Second)
		if !m.Now().Before(until) {
			return t3Failed(ran)
		}
	}
}

// t3Thread is one of the threads T3 Code lists: its id and when it was made.
type t3Thread struct {
	ThreadID  string `json:"threadId"`
	CreatedAt string `json:"createdAt"`
}

// t3Threads is the threads T3 Code lists for folder; false if the list fails or isn't a list.
func t3Threads(program, folder string, m t3Machine) ([]t3Thread, bool) {
	ran := m.Run(program, []string{"thread", "list", "--project", folder, "--json"}, t3VersionTimeout)
	var threads []t3Thread
	if !ran.Ended || ran.Code != 0 || json.Unmarshal([]byte(ran.Output), &threads) != nil {
		return nil, false
	}
	return threads, true
}

// t3LogPrefix is what T3 Code's log puts before an error: "[01:33:17.503] ERROR (#2): DesktopAppRequestFailedError: ".
var t3LogPrefix = regexp.MustCompile(`^\[[^\]]*\] ERROR \(#\d+\): \w+Error: `)

// t3Failed is a t3 that failed: a 502 with the first line it said, without T3 Code's log prefix, and nothing else.
func t3Failed(ran t3Ran) (int, any) {
	line := t3LogPrefix.ReplaceAllString(firstLine(cmpOr(ran.Error, ran.Output)), "")
	switch {
	case line == "" && !ran.Ended:
		line = "T3 Code didn't answer."
	case line == "":
		line = "T3 Code couldn't open it."
	default:
		line = clip(line, 200)
	}
	return http.StatusBadGateway, line
}

// runT3 runs program with args as they are, stopped after timeout. Only what it says in that time is read.
func runT3(program string, args []string, timeout time.Duration) t3Ran {
	return feedT3(program, args, nil, timeout)
}

// feedT3 is runT3 with input on program's standard input (none when nil).
func feedT3(program string, args []string, input []byte, timeout time.Duration) t3Ran {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := scriptCommand(ctx, program, args)
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	var out, errs strings.Builder
	cmd.Stdout, cmd.Stderr = &limited{&out}, &limited{&errs}
	// An app t3 started may keep its output open: it isn't waited for past the end.
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	ran := t3Ran{Ended: ctx.Err() == nil, Output: out.String(), Error: errs.String()}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		ran.Code = exit.ExitCode()
	case errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil:
		ran.Code = cmd.ProcessState.ExitCode()
	default:
		ran.Ended, ran.Code = false, -1
	}
	return ran
}

// limited keeps the first t3KeptOutput bytes written to it.
type limited struct{ b *strings.Builder }

func (l *limited) Write(p []byte) (int, error) {
	if room := t3KeptOutput - l.b.Len(); room > 0 {
		l.b.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

// hireInSession is the hire working in session id, as the team pairs them; false for a session that's no hire's.
func (d *Deck) hireInSession(id string) (Hire, bool) {
	if id == "" {
		return Hire{}, false
	}
	hires := readRegistry(registryPath(), false).hires
	for _, row := range buildTeam(hires, d.frontDeskLive(), 0, func(string) string { return "" }) {
		if row["sessionId"] == id && row["registered"] == true {
			for _, h := range hires {
				if h.ID == row["id"] {
					return h, true
				}
			}
		}
	}
	return Hire{}, false
}

// t3Opening is the hires whose folder is being opened now.
var t3Opening = struct {
	sync.Mutex
	ids map[string]bool
}{ids: map[string]bool{}}

// t3OpenOnce is t3Open once at a time for a hire: a second open for the same hire while one runs is a 409. What's
// installed is checked once it's this hire's turn.
func t3OpenOnce(h Hire, m t3Machine) (int, any) {
	t3Opening.Lock()
	busy := t3Opening.ids[h.ID]
	t3Opening.ids[h.ID] = true
	t3Opening.Unlock()
	if busy {
		return http.StatusConflict, t3AlreadyOpening
	}
	defer func() {
		t3Opening.Lock()
		delete(t3Opening.ids, h.ID)
		t3Opening.Unlock()
	}()
	return t3Open(h, t3StatusNow(false, m), m)
}

// openHireInT3 is POST /setup/t3code/open's answer, and the office's: a hire still on the team, or a 404; and
// once at a time for a hire, a 409 while it's being opened.
func openHireInT3(w http.ResponseWriter, h Hire, found bool) {
	if !found || h.State == "fired" || h.State == "failed" {
		writeStatus(w)(http.StatusNotFound, "That hire isn't on the team.")
		return
	}
	status, answer := t3OpenOnce(h, newT3Machine())
	if text, ok := answer.(string); ok {
		writeStatus(w)(status, text)
		return
	}
	writeJSON(w, status, answer)
}
