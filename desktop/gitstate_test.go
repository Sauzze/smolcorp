package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Git state in the core (gitstate.go): the parsers on git's own words, real throwaway repos (signing off in their
// local config only), the cache on a fake clock and a fake git, and the routes.

// gitRepoCmd runs git in dir for a test, with no signing and an identity given on the command line only.
func gitRepoCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	out, err := exec.Command("git", append([]string{"-C", dir, "-c", "commit.gpgsign=false", "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeRepoFile(t *testing.T, dir, name, text string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// aRepo is a repo with one commit, in a temp folder with its links resolved.
func aGitRepo(t *testing.T) string {
	t.Helper()
	dir := physical(t.TempDir())
	gitRepoCmd(t, dir, "init", "-q", "-b", "main")
	gitRepoCmd(t, dir, "config", "commit.gpgsign", "false")
	writeRepoFile(t, dir, "a.txt", "one\ntwo\nthree\n")
	writeRepoFile(t, dir, "src/b.txt", "b\n")
	gitRepoCmd(t, dir, "add", ".")
	gitRepoCmd(t, dir, "commit", "-q", "-m", "first")
	return dir
}

func realGitState(dir string) GitState {
	state, _ := readGitState(dir, readGitOut, time.Now())
	return state
}

func TestStatusOfGitIsReadFromItsPorcelainV2(t *testing.T) {
	out := "# branch.oid 0123456789abcdef\x00# branch.head feat/x\x00# branch.upstream origin/feat/x\x00# branch.ab +2 -1\x00" +
		"1 .M N... 100644 100644 100644 aaa bbb dir/a b.txt\x00" +
		"2 R. N... 100644 100644 100644 aaa bbb R100 new name.txt\x00old name.txt\x00" +
		"u UU N... 100644 100644 100644 100644 a b c conflict.txt\x00" +
		"? new/\x00"
	var state GitState
	state.Files = []GitFile{}
	if unborn := parseGitStatus([]byte(out), &state); unborn {
		t.Error("a repo with a commit isn't unborn")
	}
	if state.Branch != "feat/x" || state.Head != "01234567" || state.Upstream != "origin/feat/x" || state.Ahead != 2 || state.Behind != 1 || state.Changed != 4 {
		t.Fatalf("%+v", state)
	}
	want := []GitFile{{Path: filepath.FromSlash("dir/a b.txt"), State: ".M"}, {Path: "new name.txt", State: "R."}, {Path: "conflict.txt", State: "UU"}, {Path: "new" + string(filepath.Separator), State: "??"}}
	for i, f := range want {
		if state.Files[i] != f {
			t.Errorf("file %d: %+v, want %+v", i, state.Files[i], f)
		}
	}
	var fresh, detached GitState
	if unborn := parseGitStatus([]byte("# branch.oid (initial)\x00# branch.head main\x00"), &fresh); !unborn || fresh.Branch != "main" {
		t.Errorf("before the first commit: %v %+v", unborn, fresh)
	}
	parseGitStatus([]byte("# branch.oid abc\x00# branch.head (detached)\x00"), &detached)
	if !detached.Detached || detached.Branch != "" {
		t.Errorf("detached: %+v", detached)
	}
}

func TestNumstatCountsEveryFileAndNamesTheListedOnes(t *testing.T) {
	state := GitState{Files: []GitFile{{Path: "a.txt"}, {Path: filepath.FromSlash("src/b.txt")}}}
	parseGitNumstat([]byte("3\t1\ta.txt\x00-\t-\tpic.png\x002\t0\tsrc/b.txt\x002\t2\tnot listed.txt\x00"), &state)
	if state.Added != 7 || state.Removed != 3 || state.Files[0].Added != 3 || state.Files[0].Removed != 1 || state.Files[1].Added != 2 {
		t.Errorf("%+v", state)
	}
}

func TestARealRepoIsReadWithItsChangesAndIsLeftAsItWas(t *testing.T) {
	dir := aGitRepo(t)
	writeRepoFile(t, dir, "a.txt", "one\nTWO\nthree\nfour\n") // +2 −1
	writeRepoFile(t, dir, "src/b.txt", "")                    // −1, staged below
	gitRepoCmd(t, dir, "add", "src/b.txt")
	writeRepoFile(t, dir, "fresh.txt", "x\n") // untracked: listed, no lines counted
	before := map[string]string{}
	for _, name := range []string{"index", "HEAD"} {
		data, _ := os.ReadFile(filepath.Join(dir, ".git", name))
		before[name] = string(data)
	}
	state := realGitState(filepath.Join(dir, "src"))
	if !state.Repo || state.Top != dir || state.Branch != "main" || state.Detached || state.Upstream != "" || state.Head == "" {
		t.Fatalf("%+v", state)
	}
	if state.Changed != 3 || state.Added != 2 || state.Removed != 2 || state.Skipped || len(state.Files) != 3 {
		t.Fatalf("%+v", state)
	}
	byPath := map[string]GitFile{}
	for _, f := range state.Files {
		byPath[filepath.ToSlash(f.Path)] = f
	}
	if f := byPath["a.txt"]; f.State != ".M" || f.Added != 2 || f.Removed != 1 {
		t.Errorf("a.txt %+v", f)
	}
	if f := byPath["src/b.txt"]; f.State != "M." || f.Removed != 1 {
		t.Errorf("b.txt %+v", f)
	}
	if f := byPath["fresh.txt"]; f.State != "??" || f.Added != 0 {
		t.Errorf("fresh.txt %+v", f)
	}
	for name, was := range before {
		if data, _ := os.ReadFile(filepath.Join(dir, ".git", name)); string(data) != was {
			t.Errorf("%s was written to", name)
		}
	}
}

func TestAFolderInNoRepoAndAnUnbornRepoAreAnswered(t *testing.T) {
	if state := realGitState(physical(t.TempDir())); state.Repo || state.Skipped || state.Files == nil {
		t.Errorf("no repo: %+v", state)
	}
	dir := physical(t.TempDir())
	gitRepoCmd(t, dir, "init", "-q", "-b", "trunk")
	writeRepoFile(t, dir, "x.txt", "x\n")
	if state := realGitState(dir); !state.Repo || state.Branch != "trunk" || state.Changed != 1 || state.Skipped {
		t.Errorf("unborn: %+v", state)
	}
	if state := realGitState(filepath.Join(dir, "gone")); state.Repo {
		t.Errorf("a folder that isn't there: %+v", state)
	}
}

// A hire works in a worktree of its own: the state is the worktree's branch and changes (startedIn's place), not the repo's.
func TestAHireIsReadInItsWorktreeNotTheRepo(t *testing.T) {
	repo := aGitRepo(t)
	worktree := filepath.Join(physical(t.TempDir()), "hire-wt")
	gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/ana", worktree)
	writeRepoFile(t, worktree, "src/c.txt", "c\n")
	writeRepoFile(t, repo, "only-in-repo.txt", "r\n")
	h := Hire{Folder: filepath.Join(repo, "src"), Repo: repo, Worktree: worktree}
	place := startedIn(h)
	if place != filepath.Join(worktree, "src") {
		t.Fatalf("startedIn: %s", place)
	}
	state := realGitState(place)
	if state.Top != physical(worktree) || state.Branch != "hire/ana" || state.Changed != 1 || state.Files[0].Path != filepath.FromSlash("src/c.txt") {
		t.Errorf("the worktree: %+v", state)
	}
	if main := realGitState(repo); main.Branch != "main" || main.Changed != 1 || main.Files[0].Path != "only-in-repo.txt" {
		t.Errorf("the repo: %+v", main)
	}
}

func TestAnUpstreamGivesAheadAndBehind(t *testing.T) {
	origin := physical(t.TempDir())
	gitRepoCmd(t, origin, "init", "-q", "--bare", "-b", "main")
	repo := aGitRepo(t)
	gitRepoCmd(t, repo, "remote", "add", "origin", origin)
	gitRepoCmd(t, repo, "push", "-q", "-u", "origin", "main")
	writeRepoFile(t, repo, "n.txt", "n\n")
	gitRepoCmd(t, repo, "add", "n.txt")
	gitRepoCmd(t, repo, "commit", "-q", "-m", "second")
	if state := realGitState(repo); state.Upstream != "origin/main" || state.Ahead != 1 || state.Behind != 0 {
		t.Errorf("%+v", state)
	}
}

// A repo's own config may name a program for fsmonitor: reading it must not run it.
func TestReadingARepoRunsNothingItsConfigNames(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	dir := aGitRepo(t)
	marker := filepath.Join(physical(t.TempDir()), "ran")
	hook := filepath.Join(physical(t.TempDir()), "monitor.sh")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitRepoCmd(t, dir, "config", "core.fsmonitor", hook)
	writeRepoFile(t, dir, "a.txt", "changed\n")
	if state := realGitState(dir); state.Changed != 1 {
		t.Fatalf("%+v", state)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("git ran the repo's fsmonitor program")
	}
}

func TestGitIsReadWithNothingToAskAndNoOtherRepoNamed(t *testing.T) {
	env := gitReadEnv([]string{"PATH=/bin", "git_dir=/elsewhere", "GIT_WORK_TREE=/x", "GIT_TERMINAL_PROMPT=1", "GIT_PAGER=less", "HOME=/h"})
	got := strings.Join(env, " ")
	if got != "PATH=/bin HOME=/h GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0 GIT_PAGER=cat" {
		t.Errorf("%s", got)
	}
}

func TestAnAnswerOverTheCapStopsTheCopy(t *testing.T) {
	var out gitOutput
	chunk := make([]byte, gitOutputCap/2+1)
	if _, err := out.Write(chunk); err != nil {
		t.Fatal(err)
	}
	if _, err := out.Write(chunk); !errors.Is(err, errGitTooBig) || !out.over {
		t.Errorf("%v %v", err, out.over)
	}
}

// fakeGit answers like git would for a repo whose git folder is dir, counting its commands, and may block.
type fakeGit struct {
	mu      sync.Mutex
	dir     string
	calls   map[string]int
	changes string // the status's entries
	fail    map[string]error
	gate    chan struct{}
}

func (f *fakeGit) read(_ context.Context, _ string, args ...string) ([]byte, int, error) {
	f.mu.Lock()
	f.calls[args[0]]++
	err, gate := f.fail[args[0]], f.gate
	f.mu.Unlock()
	if gate != nil && args[0] == "status" {
		<-gate
	}
	if err != nil {
		return nil, 0, err
	}
	switch args[0] {
	case "rev-parse":
		return []byte("/repo\n" + f.dir + "\n"), 0, nil
	case "status":
		return []byte("# branch.oid abc\x00# branch.head main\x00" + f.changes), 0, nil
	}
	return []byte("1\t1\ta.txt\x00"), 0, nil
}

func (f *fakeGit) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[name]
}

func cachedWith(t *testing.T) (*gitStates, *fakeGit, *time.Time) {
	t.Helper()
	f := &fakeGit{dir: t.TempDir(), calls: map[string]int{}, fail: map[string]error{}, changes: "1 .M N... 1 1 1 a b a.txt\x00"}
	writeRepoFile(t, f.dir, "index", "i")
	writeRepoFile(t, f.dir, "HEAD", "ref: refs/heads/main\n")
	clock := time.Unix(1_000_000, 0)
	return &gitStates{byDir: map[string]*gitEntry{}, read: f.read, now: func() time.Time { return clock }}, f, &clock
}

func TestTheStateIsKeptFiveSecondsThenOnlyReadWhenSomethingMoved(t *testing.T) {
	g, f, clock := cachedWith(t)
	folder := t.TempDir()
	g.get(folder, 1)
	*clock = clock.Add(4 * time.Second)
	g.get(folder, 99) // inside the 5 seconds: kept, whatever moved
	if f.count("status") != 1 {
		t.Fatalf("inside the window: %d reads", f.count("status"))
	}
	*clock = clock.Add(10 * time.Second)
	g.get(folder, 1)
	if f.count("status") != 1 {
		t.Errorf("nothing moved: %d reads", f.count("status"))
	}
	g.get(folder, 2) // the session's last event moved
	if f.count("status") != 2 {
		t.Errorf("an event moved: %d reads", f.count("status"))
	}
	*clock = clock.Add(10 * time.Second)
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(f.dir, "index"), later, later); err != nil {
		t.Fatal(err)
	}
	g.get(folder, 2) // the index moved
	if f.count("status") != 3 {
		t.Errorf("the index moved: %d reads", f.count("status"))
	}
	*clock = clock.Add(10 * time.Second)
	g.get(folder, 2)
	*clock = clock.Add(gitStateMaxAge)
	g.get(folder, 2) // nothing seems to have moved, but it is a minute old
	if f.count("status") != 4 {
		t.Errorf("a minute old: %d reads", f.count("status"))
	}
}

func TestAskersTogetherShareOneRead(t *testing.T) {
	g, f, _ := cachedWith(t)
	f.gate = make(chan struct{})
	folder := t.TempDir()
	var wg sync.WaitGroup
	answers := make([]GitState, 5)
	for i := range answers {
		wg.Add(1)
		go func() { defer wg.Done(); answers[i] = g.get(folder, 0) }()
	}
	time.Sleep(50 * time.Millisecond)
	close(f.gate)
	wg.Wait()
	if f.count("status") != 1 || f.count("rev-parse") != 1 {
		t.Errorf("reads: %v", f.calls)
	}
	for _, a := range answers {
		if a.Changed != 1 || a.Added != 1 {
			t.Errorf("%+v", a)
		}
	}
}

func TestWhatGitCannotAnswerIsSaidNotHidden(t *testing.T) {
	for _, c := range []struct {
		command string
		err     error
		note    string
	}{
		{"status", errGitTooBig, "too many changes to list"},
		{"rev-parse", exec.ErrNotFound, "isn't installed"},
		{"status", context.DeadlineExceeded, "too long"},
		{"status", errors.New("boom"), "couldn't be read"},
	} {
		g, f, _ := cachedWith(t)
		f.fail[c.command] = c.err
		state := g.get(t.TempDir(), 0)
		if !state.Skipped || !strings.Contains(state.Note, c.note) || state.Files == nil {
			t.Errorf("%v: %+v", c.err, state)
		}
	}
	g, f, _ := cachedWith(t)
	f.fail["diff"] = errGitTooBig
	if state := g.get(t.TempDir(), 0); !state.Skipped || state.Changed != 1 || state.Added != 0 || !strings.Contains(state.Note, "too many changes") {
		t.Errorf("the count skipped: %+v", state)
	}
}

func TestOnlyTheNewestFoldersAreKept(t *testing.T) {
	g, _, clock := cachedWith(t)
	for i := 0; i < gitStatesKept+10; i++ {
		*clock = clock.Add(time.Second)
		g.get(t.TempDir(), 0)
	}
	if len(g.byDir) > gitStatesKept {
		t.Errorf("%d kept", len(g.byDir))
	}
}

func TestTheDeskRouteIsTheWindowsAndTheGMReadsAHireInItsWorktree(t *testing.T) {
	repo := aGitRepo(t)
	worktree := filepath.Join(physical(t.TempDir()), "wt")
	gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/bo", worktree)
	writeRepoFile(t, worktree, "w.txt", "w\n")
	writeRepoFile(t, repo, "r.txt", "r\n")

	m := newMailDeck(t)
	d := m.d
	var file map[string]any
	current, err := os.ReadFile(registryPath()) // with the hires' keys newMailDeck stored in it
	if err != nil || json.Unmarshal(current, &file) != nil {
		t.Fatal("the registry", err)
	}
	file["hires"] = append(file["hires"].([]any),
		map[string]any{"id": "h1", "name": "Bo", "role": "builder", "state": "working", "folder": repo, "repo": repo, "worktree": worktree})
	data, _ := json.Marshal(file)
	if err := os.WriteFile(registryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	phone := pair(t, d)
	hook := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(jsonLine(map[string]any{"hook_event_name": "UserPromptSubmit",
		"session_id": "s1", "cwd": repo})))
	hook.Host, hook.RemoteAddr = "127.0.0.1:7710", thisComputer
	hook.Header.Set(hookKeyHeader, d.access.HookKey)
	hook.Header.Set("X-AgentDeck-Agent", "claude")
	hook.Header.Set("X-AgentDeck-Tab", "tty:/dev/ttys003")
	if w := httptest.NewRecorder(); func() int { (&router{deck: d}).ServeHTTP(w, hook); return w.Code }() != http.StatusNoContent {
		t.Fatal("the hook")
	}
	window := func(target string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: "GET", target: target + "&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
	}
	if got := window("/desk/git?session=s1"); got.Code != http.StatusOK || decoded(t, got)["branch"] != "main" || decoded(t, got)["top"] != repo {
		t.Errorf("window: %d %s", got.Code, got.Body)
	}
	if got := window("/desk/git?session=gone"); got.Code != http.StatusNotFound {
		t.Errorf("gone: %d", got.Code)
	}
	if refused := serve(t, d, call{method: "GET", target: "/desk/git?session=s1&t=" + phone, host: deckOnWiFi, from: aPhone}); refused.Code != http.StatusForbidden ||
		strings.TrimSpace(refused.Body.String()) != "The git state shows only in the office's window on your computer." {
		t.Errorf("phone: %d %s", refused.Code, refused.Body)
	}
	gm := m.call(t, asGM, "GET", "/setup/gm/git?hire=h1", "")
	if gm.Code != http.StatusOK || decoded(t, gm)["branch"] != "hire/bo" || decoded(t, gm)["top"] != worktree {
		t.Errorf("gm: %d %s", gm.Code, gm.Body)
	}
	for target, code := range map[string]int{"/setup/gm/git?hire=nobody": 404, "/setup/gm/git": 400} {
		if got := m.call(t, asGM, "GET", target, ""); got.Code != code {
			t.Errorf("%s: %d", target, got.Code)
		}
	}
	if got := m.call(t, asGM, "POST", "/setup/gm/git?hire=h1", ""); got.Code == http.StatusOK {
		t.Errorf("a POST was answered: %d", got.Code)
	}
	if got := m.call(t, asRev, "GET", "/setup/gm/git?hire=h1", ""); got.Code != http.StatusForbidden {
		t.Errorf("a hire's key: %d", got.Code)
	}
}
