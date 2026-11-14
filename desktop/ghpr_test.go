package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// The pull request badge (ghpr.go): the remote's owner and name, what gh is given and told, what it answers, the
// cache, and the routes. No test runs the real gh or touches the network: gh is a stand-in script that prints canned
// text, or a fake runner, and the app's own `prs` is a refusing one unless a test puts one of its own.

func init() {
	prs = newPRBadges(func(context.Context, string, string) ([]byte, []byte, int, error) {
		return nil, nil, 0, errors.New("a test ran the real gh")
	})
}

// withPRs puts run in for the app's own `prs` and gives it a clock the test can move.
func withPRs(t *testing.T, run ghRunner) (*prBadges, *time.Time) {
	t.Helper()
	was := prs
	p := newPRBadges(run)
	clock := time.Unix(2_000_000, 0)
	var mu sync.Mutex
	p.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return clock }
	prs = p
	t.Cleanup(func() { p.wg.Wait(); prs = was })
	return p, &clock
}

func goodPR(repo string, number int) []byte {
	b, _ := json.Marshal(map[string]any{"number": number, "state": "OPEN", "reviewDecision": "APPROVED",
		"url": "https://github.com/" + repo + "/pull/" + strconv.Itoa(number)})
	return b
}

func aGitHubState(branch string) GitState {
	return GitState{Repo: true, Branch: branch, remote: "https://github.com/acme/widgets.git"}
}

func TestOnlyGitHubRemotesGiveARepo(t *testing.T) {
	for remote, want := range map[string]string{
		"https://github.com/acme/widgets.git":      "acme/widgets",
		"https://github.com/acme/widgets":          "acme/widgets",
		"https://github.com/acme/widgets/":         "acme/widgets",
		"https://user:pw@github.com/acme/w.x_y-z":  "acme/w.x_y-z",
		"HTTPS://GitHub.com/Acme/Widgets.git":      "Acme/Widgets",
		"git@github.com:acme/widgets.git":          "acme/widgets",
		"ssh://git@github.com/acme/widgets.git":    "acme/widgets",
		"git://github.com/acme/widgets.git":        "acme/widgets",
		"https://gitlab.com/acme/widgets.git":      "",
		"https://github.example.com/acme/w.git":    "",
		"https://github.com.evil.test/acme/w.git":  "",
		"https://evil.test/github.com/acme/w.git":  "",
		"https://github.com:8443/acme/widgets.git": "",
		"http://github.com/acme/widgets.git":       "",
		"git@gitlab.com:acme/widgets.git":          "",
		"git@github.com:acme/widgets/extra.git":    "",
		"https://github.com/acme":                  "",
		"https://github.com/acme/widgets/tree/x":   "",
		"https://github.com/acme/widgets?x=1":      "",
		"https://github.com/-acme/widgets":         "",
		"https://github.com/acme/..":               "",
		"https://github.com/acme/wid gets":         "",
		"file:///srv/git/widgets.git":              "",
		"/srv/git/widgets.git":                     "",
		`C:\repos\widgets`:                         "",
		"../widgets":                               "",
		"":                                         "",
	} {
		if got := githubRepoOf(remote); got != want {
			t.Errorf("%q: %q, want %q", remote, got, want)
		}
	}
}

func TestABranchGhCouldMisreadIsNotGivenToIt(t *testing.T) {
	for branch, want := range map[string]bool{"feat/pr-badge": true, "main": true, "hire/bo.2": true, "v1.26": true,
		"": false, "-x": false, "--repo": false, "42": false, "owner:branch": false, "https://github.com/a/b/pull/1": false,
		"a b": false, "a\nb": false, `a\b`: false, strings.Repeat("x", 300): false} {
		if prBranchOK(branch) != want {
			t.Errorf("%q: want %v", branch, want)
		}
	}
}

func TestTheRemoteIsTheOneTheBranchTracks(t *testing.T) {
	list := "remote.origin.url\nhttps://github.com/acme/widgets.git\x00remote.fork.url\nhttps://github.com/me/widgets.git\x00" +
		"branch.feat/x.remote\nfork\x00branch.feat/x.merge\nrefs/heads/feat/x\x00branch.local.remote\n.\x00"
	for branch, want := range map[string]string{"feat/x": "https://github.com/me/widgets.git", "other": "https://github.com/acme/widgets.git",
		"local": "", "": ""} {
		if got := gitRemoteOf([]byte(list), branch); got != want {
			t.Errorf("%q: %q, want %q", branch, got, want)
		}
	}
	// the last url stands, as git's own `config --get`
	if got := gitRemoteOf([]byte("remote.origin.url\nhttps://github.com/a/b\x00remote.origin.url\nhttps://github.com/c/d\x00"), "m"); got != "https://github.com/c/d" {
		t.Errorf("last url: %q", got)
	}
	// a rewrite of addresses means the one written isn't the one git uses
	if got := gitRemoteOf([]byte("remote.origin.url\nhttps://github.com/a/b\x00url.https://evil.test/.insteadof\nhttps://github.com/\x00"), "m"); got != "" {
		t.Errorf("insteadOf: %q", got)
	}
	if got := gitRemoteOf([]byte("remote.origin.url\nhttps://github.com/a/b\x00url.https://evil.test/.pushinsteadof\nhttps://github.com/\x00"), "m"); got != "" {
		t.Errorf("pushInsteadOf: %q", got)
	}
}

func TestOnlyTheAskedPullRequestMakesABadge(t *testing.T) {
	repo := "acme/widgets"
	ok := func(s string) []byte { return []byte(s) }
	for name, tc := range map[string]struct {
		out  []byte
		want bool
	}{
		"good":               {goodPR(repo, 7), true},
		"no review yet":      {ok(`{"number":7,"state":"OPEN","reviewDecision":"","url":"https://github.com/acme/widgets/pull/7"}`), true},
		"merged":             {ok(`{"number":7,"state":"MERGED","reviewDecision":"APPROVED","url":"https://github.com/acme/widgets/pull/7"}`), true},
		"other case":         {ok(`{"number":7,"state":"OPEN","reviewDecision":"","url":"https://github.com/Acme/Widgets/pull/7"}`), true},
		"another repo":       {goodPR("acme/other", 7), false},
		"another owner":      {goodPR("evil/widgets", 7), false},
		"another host":       {ok(`{"number":7,"state":"OPEN","reviewDecision":"","url":"https://evil.test/acme/widgets/pull/7"}`), false},
		"http":               {ok(`{"number":7,"state":"OPEN","reviewDecision":"","url":"http://github.com/acme/widgets/pull/7"}`), false},
		"another number":     {ok(`{"number":7,"state":"OPEN","reviewDecision":"","url":"https://github.com/acme/widgets/pull/8"}`), false},
		"a tail":             {ok(`{"number":7,"state":"OPEN","reviewDecision":"","url":"https://github.com/acme/widgets/pull/7/files"}`), false},
		"a script":           {ok(`{"number":7,"state":"OPEN","reviewDecision":"","url":"javascript:alert(1)"}`), false},
		"unknown state":      {ok(`{"number":7,"state":"DRAFT","reviewDecision":"","url":"https://github.com/acme/widgets/pull/7"}`), false},
		"lower case state":   {ok(`{"number":7,"state":"open","reviewDecision":"","url":"https://github.com/acme/widgets/pull/7"}`), false},
		"unknown decision":   {ok(`{"number":7,"state":"OPEN","reviewDecision":"LOVED","url":"https://github.com/acme/widgets/pull/7"}`), false},
		"zero":               {ok(`{"number":0,"state":"OPEN","reviewDecision":"","url":"https://github.com/acme/widgets/pull/0"}`), false},
		"huge":               {ok(`{"number":999999999999,"state":"OPEN","reviewDecision":"","url":"https://github.com/acme/widgets/pull/999999999999"}`), false},
		"an extra field":     {ok(`{"number":7,"state":"OPEN","reviewDecision":"","url":"https://github.com/acme/widgets/pull/7","title":"x"}`), false},
		"two objects":        {append(goodPR(repo, 7), goodPR(repo, 8)...), false},
		"a list":             {append(append([]byte("["), goodPR(repo, 7)...), ']'), false},
		"not json":           {ok("no pull requests found"), false},
		"empty":              {nil, false},
		"number as text":     {ok(`{"number":"7","state":"OPEN","reviewDecision":"","url":"https://github.com/acme/widgets/pull/7"}`), false},
		"a newline in a url": {ok(`{"number":7,"state":"OPEN","reviewDecision":"","url":"https://github.com/acme/widgets/pull/7\n"}`), false},
	} {
		got := prFromGH(tc.out, repo)
		if (got != nil) != tc.want {
			t.Errorf("%s: %+v, want a badge: %v", name, got, tc.want)
		}
		if got != nil && got.URL != "https://github.com/acme/widgets/pull/7" && name != "other case" {
			t.Errorf("%s: %q", name, got.URL)
		}
	}
	// what is shown is the asked repo's own spelling of the address
	if got := prFromGH(ok(`{"number":7,"state":"OPEN","reviewDecision":"","url":"https://github.com/Acme/Widgets/pull/7"}`), repo); got == nil || got.URL != "https://github.com/acme/widgets/pull/7" {
		t.Errorf("%+v", got)
	}
}

// aStandInGH is a gh that is a shell script, in a usual place and nowhere else: the PATH and the login shell's are
// empty, so the real gh can't be found. It writes where it ran, what it was given and the environment it saw to a
// folder, and then runs body. It needs a shell: not on Windows.
func aStandInGH(t *testing.T, body string) (seen string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in gh is a shell script")
	}
	home := t.TempDir()
	seen = filepath.Join(t.TempDir(), "seen")
	if err := os.Mkdir(seen, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\npwd > '" + seen + "/pwd'\nprintf '%s\\n' \"$@\" > '" + seen + "/args'\nenv > '" + seen + "/env'\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(home, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	savedPlaces, savedLogin := ghPlaces, tmuxLoginPath
	t.Cleanup(func() { ghPlaces, tmuxLoginPath = savedPlaces, savedLogin })
	ghPlaces, tmuxLoginPath = []string{home}, func() []string { return nil }
	t.Setenv("PATH", t.TempDir())
	return seen
}

func runStandIn(t *testing.T, repo, branch string, wait time.Duration) prAnswer {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	return newPRBadges(runGH).ask(ctx, repo, branch)
}

func TestAStandInGhGivesABadgeFromTheCommandAskedFor(t *testing.T) {
	seen := aStandInGH(t, `cat <<'EOF'
{"number":42,"state":"OPEN","reviewDecision":"CHANGES_REQUESTED","url":"https://github.com/acme/widgets/pull/42"}
EOF`)
	got := runStandIn(t, "acme/widgets", "feat/x", 5*time.Second)
	if got.badge == nil || *got.badge != (PRBadge{42, "OPEN", "CHANGES_REQUESTED", "https://github.com/acme/widgets/pull/42"}) || got.hint != "" {
		t.Fatalf("%+v %+v", got, got.badge)
	}
	args, _ := os.ReadFile(filepath.Join(seen, "args"))
	if want := "pr\nview\nfeat/x\n--repo\nacme/widgets\n--json\nnumber,state,reviewDecision,url\n"; string(args) != want {
		t.Errorf("gh was run with %q, want %q", args, want)
	}
}

func TestGhRunsOutsideTheRepoWithACleanedEnvironment(t *testing.T) {
	seen := aStandInGH(t, `echo '{"number":1,"state":"OPEN","reviewDecision":"","url":"https://github.com/acme/widgets/pull/1"}'`)
	repo := aGitRepo(t)
	t.Chdir(repo) // the app itself happens to be in a repo
	for name, value := range map[string]string{"GH_TOKEN": "fake-t1", "GITHUB_TOKEN": "fake-t2", "GH_ENTERPRISE_TOKEN": "fake-t3",
		"GH_HOST": "evil.test", "GH_REPO": "evil/repo", "GH_CONFIG_DIR": repo, "GH_BROWSER": "/bin/evil", "GH_EDITOR": "/bin/evil",
		"GITHUB_ENTERPRISE_TOKEN": "fake-t4", "GH_PAGER": "evil", "BROWSER": "evil", "EDITOR": "evil", "GIT_DIR": repo + "/.git",
		"GIT_SSH_COMMAND": "evil", "LD_PRELOAD": "/evil.so", "DYLD_INSERT_LIBRARIES": "/evil.dylib", "SECRET_THING": "fake-s",
		"NODE_OPTIONS": "--evil", "HTTPS_PROXY": "http://proxy.test:3128", "LC_ALL": "C"} {
		t.Setenv(name, value)
	}
	if got := runStandIn(t, "acme/widgets", "feat/x", 5*time.Second); got.badge == nil {
		t.Fatalf("%+v", got)
	}
	where, _ := os.ReadFile(filepath.Join(seen, "pwd"))
	tmp, _ := filepath.EvalSymlinks(os.TempDir())
	here, _ := filepath.EvalSymlinks(strings.TrimSpace(string(where)))
	if here != tmp {
		t.Errorf("gh ran in %q, want the temp folder %q", here, tmp)
	}
	if strings.HasPrefix(here, repo) || here == "" {
		t.Errorf("gh ran in the repo: %q", here)
	}
	text, _ := os.ReadFile(filepath.Join(seen, "env"))
	env := map[string]string{}
	for _, line := range strings.Split(string(text), "\n") {
		if name, value, ok := strings.Cut(line, "="); ok {
			env[name] = value
		}
	}
	for name, value := range env {
		switch {
		case ghEnvKept[strings.ToUpper(name)] || strings.HasPrefix(name, "LC_") || ghSetByUs[name] || name == "PATH":
		case name == "PWD" || name == "OLDPWD" || name == "SHLVL" || name == "_": // the shell the stand-in runs in adds these
		default:
			t.Errorf("gh saw %s=%q, which is neither kept nor set by the app", name, value)
		}
		if strings.Contains(value, "fake-") || strings.Contains(value, "evil") {
			t.Errorf("gh saw %s=%q", name, value)
		}
	}
	if env["PATH"] != "/usr/bin:/bin" {
		t.Errorf("PATH=%q", env["PATH"])
	}
	if env["HOME"] == "" || env["GH_PROMPT_DISABLED"] != "1" || env["GH_PAGER"] != "cat" || env["NO_COLOR"] != "1" {
		t.Errorf("what gh should be told is missing: %v", env)
	}
	if env["HTTPS_PROXY"] != "http://proxy.test:3128" || env["LC_ALL"] != "C" {
		t.Errorf("a kept variable was dropped: proxy %q, locale %q", env["HTTPS_PROXY"], env["LC_ALL"])
	}
}

// ghSetByUs are the variables ghRunEnv adds itself.
var ghSetByUs = map[string]bool{"GH_PROMPT_DISABLED": true, "GH_NO_UPDATE_NOTIFIER": true, "GH_NO_EXTENSION_UPDATE_NOTIFIER": true,
	"GH_SPINNER_DISABLED": true, "GH_PAGER": true, "PAGER": true, "NO_COLOR": true, "CLICOLOR": true, "GH_TELEMETRY": true, "DO_NOT_TRACK": true}

func TestNoBadgeAndTheRightHintWhenGhCannotAnswer(t *testing.T) {
	good := `echo '{"number":1,"state":"OPEN","reviewDecision":"","url":"https://github.com/acme/widgets/pull/1"}'`
	for name, tc := range map[string]struct {
		body string
		wait time.Duration
		hint string
	}{
		"signed out (exit 4)":      {"echo 'To get started with GitHub CLI, please run:  gh auth login' >&2; exit 4", 5 * time.Second, ghHintOut},
		"signed out (its words)":   {"echo 'Try authenticating with:  gh auth login' >&2; exit 1", 5 * time.Second, ghHintOut},
		"no pull request":          {"echo 'no pull requests found for branch \"feat/x\"' >&2; exit 1", 5 * time.Second, ""},
		"a bad url":                {`echo '{"number":1,"state":"OPEN","reviewDecision":"","url":"https://evil.test/acme/widgets/pull/1"}'`, 5 * time.Second, ""},
		"another repo":             {`echo '{"number":1,"state":"OPEN","reviewDecision":"","url":"https://github.com/acme/other/pull/1"}'`, 5 * time.Second, ""},
		"an unknown state":         {`echo '{"number":1,"state":"WEIRD","reviewDecision":"","url":"https://github.com/acme/widgets/pull/1"}'`, 5 * time.Second, ""},
		"a timeout":                {"exec sleep 30", 300 * time.Millisecond, ""},
		"huge output":              {"yes '{\"number\":1}' | head -c 400000", 5 * time.Second, ""},
		"huge output, over an end": {"head -c 100000 /dev/zero | tr '\\0' ' '; " + good, 5 * time.Second, ""},
		"a crash":                  {"kill -9 $$", 5 * time.Second, ""},
		"nothing":                  {"exit 0", 5 * time.Second, ""},
	} {
		aStandInGH(t, tc.body)
		started := time.Now()
		got := runStandIn(t, "acme/widgets", "feat/x", tc.wait)
		if got.badge != nil || got.hint != tc.hint {
			t.Errorf("%s: badge %+v, hint %q, want none and %q", name, got.badge, got.hint, tc.hint)
		}
		if time.Since(started) > 4*time.Second {
			t.Errorf("%s: took %v", name, time.Since(started))
		}
	}
}

func TestAMissingGhIsNoBadgeAndAHint(t *testing.T) {
	aStandInGH(t, "exit 0")
	empty := t.TempDir()
	ghPlaces = []string{empty, "relative/folder", ""}
	if got := runStandIn(t, "acme/widgets", "feat/x", 5*time.Second); got.badge != nil || got.hint != ghHintAbsent {
		t.Errorf("%+v", got)
	}
	// a file that can't run is no gh
	if runtime.GOOS != "windows" {
		if err := os.WriteFile(filepath.Join(empty, "gh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := findGH(); got != "" {
			t.Errorf("a gh that can't run: %q", got)
		}
	}
}

func TestTheUsualPlacesComeBeforeThePathAndARelativeOneIsNeverUsed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	usual, onPath, relative := t.TempDir(), t.TempDir(), t.TempDir()
	for _, dir := range []string{usual, onPath, relative} {
		if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	savedPlaces, savedLogin := ghPlaces, tmuxLoginPath
	t.Cleanup(func() { ghPlaces, tmuxLoginPath = savedPlaces, savedLogin })
	tmuxLoginPath = func() []string { return nil }
	t.Setenv("PATH", onPath)
	ghPlaces = []string{usual}
	if got := findGH(); got != filepath.Join(usual, "gh") {
		t.Errorf("a usual place: %q", got)
	}
	ghPlaces = nil
	if got := findGH(); got != filepath.Join(onPath, "gh") {
		t.Errorf("the PATH: %q", got)
	}
	t.Chdir(relative)
	t.Setenv("PATH", ".")
	if got := findGH(); got != "" {
		t.Errorf("a PATH folder relative to where this runs: %q", got)
	}
	tmuxLoginPath = func() []string { return []string{"", "bin", usual} }
	if got := findGH(); got != filepath.Join(usual, "gh") {
		t.Errorf("the login shell's PATH: %q", got)
	}
}

// fakeGH answers like gh would, counting its runs and how many are under way at once, and may block.
type fakeGH struct {
	mu        sync.Mutex
	runs      map[string]int
	now, peak int
	gate      chan struct{}
	out       []byte
	status    int
	err       error
	errText   []byte
}

func (f *fakeGH) run(_ context.Context, repo, branch string) ([]byte, []byte, int, error) {
	f.mu.Lock()
	f.runs[repo+"@"+branch]++
	f.now++
	f.peak = max(f.peak, f.now)
	gate := f.gate
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}
	f.mu.Lock()
	f.now--
	f.mu.Unlock()
	out := f.out
	if out == nil && f.err == nil && f.status == 0 {
		out = goodPR(repo, 42)
	}
	return out, f.errText, f.status, f.err
}

func (f *fakeGH) count(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.runs[key]
}

func (f *fakeGH) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.runs {
		n += c
	}
	return n
}

func TestTheBadgeIsKeptASecondMinuteAndNotAskedForAgainBeforeThen(t *testing.T) {
	f := &fakeGH{runs: map[string]int{}}
	p, clock := withPRs(t, f.run)
	state := aGitHubState("feat/x")
	if badge, hint := p.For(state); badge != nil || hint != "" { // nothing kept yet: no wait, and a run starts
		t.Fatalf("first: %+v %q", badge, hint)
	}
	p.wg.Wait()
	badge, _ := p.For(state)
	if badge == nil || badge.Number != 42 || badge.State != "OPEN" || badge.URL != "https://github.com/acme/widgets/pull/42" {
		t.Fatalf("kept: %+v", badge)
	}
	*clock = clock.Add(59 * time.Second)
	if badge, _ := p.For(state); badge == nil {
		t.Error("gone inside the minute")
	}
	p.wg.Wait()
	if n := f.count("acme/widgets@feat/x"); n != 1 {
		t.Fatalf("%d runs inside the minute, want 1", n)
	}
	*clock = clock.Add(2 * time.Second)
	if badge, _ := p.For(state); badge == nil { // the kept one is shown while the next is asked for
		t.Error("the answer was dropped at the minute")
	}
	p.wg.Wait()
	if n := f.count("acme/widgets@feat/x"); n != 2 {
		t.Errorf("%d runs after the minute, want 2", n)
	}
	// a repo's name in another case is the same repo
	state.remote = "git@github.com:Acme/Widgets.git"
	p.For(state)
	p.wg.Wait()
	if f.total() != 2 {
		t.Errorf("the same repo in another case ran again: %v", f.runs)
	}
}

func TestNoBadgeIsAlsoKeptAMinuteAndAnOldAnswerIsNotShown(t *testing.T) {
	f := &fakeGH{runs: map[string]int{}, status: 1}
	p, clock := withPRs(t, f.run)
	state := aGitHubState("feat/x")
	for i := 0; i < 5; i++ {
		p.For(state)
		p.wg.Wait()
		*clock = clock.Add(10 * time.Second)
	}
	if n := f.total(); n != 1 {
		t.Errorf("no answer was asked for %d times in 50 s, want once", n)
	}
	f.mu.Lock()
	f.status, f.out = 0, goodPR("acme/widgets", 9)
	f.mu.Unlock()
	*clock = clock.Add(11 * time.Second)
	p.For(state)
	p.wg.Wait()
	if badge, _ := p.For(state); badge == nil || badge.Number != 9 {
		t.Errorf("%+v", badge)
	}
	// a refresh that hangs for long shows nothing in the end rather than an old answer
	f.mu.Lock()
	f.gate = make(chan struct{})
	f.mu.Unlock()
	*clock = clock.Add(prStaleFor + time.Second)
	if badge, _ := p.For(state); badge != nil {
		t.Errorf("an answer older than %v was shown: %+v", prStaleFor, badge)
	}
	close(f.gate)
	p.wg.Wait()
}

func TestDesksOnOneBranchShareOneRunAndManyBranchesDontStormGh(t *testing.T) {
	f := &fakeGH{runs: map[string]int{}, gate: make(chan struct{})}
	p, _ := withPRs(t, f.run)
	var asking sync.WaitGroup
	for i := 0; i < 40; i++ { // forty desks, one branch
		asking.Add(1)
		go func() { defer asking.Done(); p.For(aGitHubState("feat/same")) }()
	}
	asking.Wait()
	for i := 0; i < 12; i++ { // and twelve branches of the same repo
		p.For(aGitHubState("feat/b" + strconv.Itoa(i)))
	}
	time.Sleep(100 * time.Millisecond)
	f.mu.Lock()
	under := f.now
	f.mu.Unlock()
	if under > prAtOnce {
		t.Errorf("%d gh runs at once, want at most %d", under, prAtOnce)
	}
	close(f.gate)
	p.wg.Wait()
	if n := f.count("acme/widgets@feat/same"); n != 1 {
		t.Errorf("one branch ran %d times, want 1", n)
	}
	if f.total() != 13 || f.peak > prAtOnce {
		t.Errorf("%d runs, %d at once at most: %v", f.total(), f.peak, f.runs)
	}
}

func TestNothingIsRunForWhatCannotHaveABadge(t *testing.T) {
	f := &fakeGH{runs: map[string]int{}}
	p, _ := withPRs(t, f.run)
	for name, state := range map[string]GitState{
		"no repo":       {Branch: "x", remote: "https://github.com/a/b"},
		"detached":      {Repo: true, Detached: true, remote: "https://github.com/a/b"},
		"no branch":     {Repo: true, remote: "https://github.com/a/b"},
		"skipped":       {Repo: true, Branch: "x", Skipped: true, remote: "https://github.com/a/b"},
		"no remote":     {Repo: true, Branch: "x"},
		"not github":    {Repo: true, Branch: "x", remote: "https://gitlab.com/a/b.git"},
		"an enterprise": {Repo: true, Branch: "x", remote: "git@github.acme.test:a/b.git"},
		"a bad remote":  {Repo: true, Branch: "x", remote: "nonsense"},
		"a number":      {Repo: true, Branch: "42", remote: "https://github.com/a/b"},
		"an option":     {Repo: true, Branch: "--web", remote: "https://github.com/a/b"},
	} {
		if badge, hint := p.For(state); badge != nil || hint != "" {
			t.Errorf("%s: %+v %q", name, badge, hint)
		}
	}
	p.wg.Wait()
	if f.total() != 0 {
		t.Errorf("gh was run: %v", f.runs)
	}
}

func TestAMissingOrSignedOutGhIsKeptAsAHintForSetup(t *testing.T) {
	for name, tc := range map[string]struct {
		f    *fakeGH
		hint string
	}{
		"missing":    {&fakeGH{runs: map[string]int{}, err: errGHMissing}, ghHintAbsent},
		"signed out": {&fakeGH{runs: map[string]int{}, status: ghAuthStatus}, ghHintOut},
		"a timeout":  {&fakeGH{runs: map[string]int{}, err: context.DeadlineExceeded}, ""},
		"a failure":  {&fakeGH{runs: map[string]int{}, err: errors.New("boom")}, ""},
	} {
		p, _ := withPRs(t, tc.f.run)
		p.For(aGitHubState("x"))
		p.wg.Wait()
		if badge, hint := p.For(aGitHubState("x")); badge != nil || hint != tc.hint {
			t.Errorf("%s: %+v %q, want %q", name, badge, hint, tc.hint)
		}
	}
}

func TestARealRepoOnGitHubGivesItsRemoteAndBranch(t *testing.T) {
	repo := aGitRepo(t)
	gitRepoCmd(t, repo, "remote", "add", "origin", "https://user:pw@github.com/acme/widgets.git")
	gitRepoCmd(t, repo, "remote", "add", "fork", "git@github.com:me/widgets.git")
	state := realGitState(repo)
	if state.remote != "https://user:pw@github.com/acme/widgets.git" || state.Branch != "main" {
		t.Fatalf("%q %q", state.remote, state.Branch)
	}
	if githubRepoOf(state.remote) != "acme/widgets" {
		t.Errorf("%q", githubRepoOf(state.remote))
	}
	gitRepoCmd(t, repo, "config", "branch.main.remote", "fork")
	if got := realGitState(repo).remote; got != "git@github.com:me/widgets.git" {
		t.Errorf("the tracked remote: %q", got)
	}
	// the address is never in what is sent
	if text, _ := json.Marshal(state); strings.Contains(string(text), "pw@") || strings.Contains(string(text), "github.com") {
		t.Errorf("the remote is in the answer: %s", text)
	}
}

func TestTheBadgeIsOnTheWindowAndTheGMRoutesOnly(t *testing.T) {
	repo := aGitRepo(t)
	gitRepoCmd(t, repo, "remote", "add", "origin", "https://github.com/acme/widgets.git")
	gitRepoCmd(t, repo, "checkout", "-q", "-b", "feat/badge")
	worktree := filepath.Join(physical(t.TempDir()), "wt")
	gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/bo", worktree)

	f := &fakeGH{runs: map[string]int{}}
	p, _ := withPRs(t, f.run)

	m := newMailDeck(t)
	d := m.d
	var file map[string]any
	current, err := os.ReadFile(registryPath())
	if err != nil || json.Unmarshal(current, &file) != nil {
		t.Fatal("the registry", err)
	}
	file["hires"] = append(file["hires"].([]any),
		map[string]any{"id": "h1", "name": "Bo", "role": "builder", "state": "working", "folder": repo, "repo": repo, "worktree": worktree})
	data, _ := json.Marshal(file)
	if err := writeRegistryFile(registryPath(), data, 0o600); err != nil {
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
	window := func(target string) map[string]any {
		got := serve(t, d, call{method: "GET", target: target + "&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
		if got.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", target, got.Code, got.Body)
		}
		return decoded(t, got)
	}
	// The first answer doesn't wait for gh; the next one has the badge.
	if first := window("/desk/git?session=s1"); first["pr"] != nil || first["gh"] != nil || first["branch"] != "feat/badge" {
		t.Errorf("first: %v", first)
	}
	p.wg.Wait()
	got := window("/desk/git?session=s1")
	pr, _ := got["pr"].(map[string]any)
	if pr["number"] != float64(42) || pr["state"] != "OPEN" || pr["reviewDecision"] != "APPROVED" || pr["url"] != "https://github.com/acme/widgets/pull/42" {
		t.Errorf("window: %v", got)
	}
	if got["remote"] != nil || strings.Contains(serve(t, d, call{method: "GET", target: "/desk/git?session=s1&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer}).Body.String(), "acme/widgets.git") {
		t.Errorf("the remote's address is in the answer: %v", got)
	}
	if f.count("acme/widgets@feat/badge") != 1 {
		t.Errorf("runs: %v", f.runs)
	}
	// The GM reads a hire the same way, by its own branch (another key, so another run).
	m.call(t, asGM, "GET", "/setup/gm/git?hire=h1", "")
	p.wg.Wait()
	gm := m.call(t, asGM, "GET", "/setup/gm/git?hire=h1", "")
	if gm.Code != http.StatusOK || decoded(t, gm)["branch"] != "hire/bo" || decoded(t, gm)["pr"] == nil {
		t.Errorf("gm: %d %s", gm.Code, gm.Body)
	}
	// Nobody else: a phone, a hire's key, and the viewer's and phones' snapshot carry nothing of it.
	refused := serve(t, d, call{method: "GET", target: "/desk/git?session=s1&t=" + phone, host: deckOnWiFi, from: aPhone})
	if refused.Code != http.StatusForbidden || strings.Contains(refused.Body.String(), "pull") || strings.Contains(refused.Body.String(), "42") {
		t.Errorf("phone: %d %s", refused.Code, refused.Body)
	}
	if got := m.call(t, asRev, "GET", "/setup/gm/git?hire=h1", ""); got.Code != http.StatusForbidden || strings.Contains(got.Body.String(), "pull") {
		t.Errorf("a hire's key: %d %s", got.Code, got.Body)
	}
	if got := m.call(t, asGM, "POST", "/setup/gm/git?hire=h1", ""); got.Code == http.StatusOK {
		t.Errorf("a POST was answered: %d", got.Code)
	}
	snapshot := string(d.snapshot())
	viewer, _ := json.Marshal(d.viewerSnapshot(false, func(s string) string { return s }))
	for name, text := range map[string]string{"the snapshot phones read": snapshot, "the viewer's": string(viewer)} {
		if strings.Contains(text, "/pull/") || strings.Contains(text, `"pr"`) || strings.Contains(text, "reviewDecision") {
			t.Errorf("%s carries the pull request: %.300s", name, text)
		}
	}
}
