package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// The T3 Code desk (t3desk.go), as the Mac's T3DeskTests: the gate, the hire in T797a's order with its fence,
// the undo, the follow, the deep link, mail on standard input, letting go, and t3 with its pinned argument
// lists only. A scratch home folder holds a fake t3 that writes down each run (and, at `project add`, what
// was on disk then); the git repo is a real one in the same scratch folder. The real t3 and T3 Code are never
// run here (see t3desk_real_test.go).

const t3TestHireID = "abcdef12-0000-4000-8000-000000000001"

// t3TestPrinted is the guard's groups as a stand-in guard prints them.
const t3TestPrinted = `{"groups":[{"name":"push","on":true,"commands":["git push|send-pack"]}],"file":"x","problem":""}`

type t3Rig struct {
	t                                *testing.T
	home, calls, flags, notes, stdin string
	log, failed, startedWith, opened []string
	atBrief                          func(place hirePlace)
	clock                            time.Time
}

func newT3Rig(t *testing.T) *t3Rig {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake t3 is a shell script; on Windows t3 runs as a .cmd through cmdLine, tested with OpenCode's")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git isn't here")
	}
	isolate(t)
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := &t3Rig{t: t, home: home, calls: filepath.Join(home, "calls"), flags: filepath.Join(home, "flags"), notes: filepath.Join(home, "notes"),
		stdin: filepath.Join(home, "stdin"), atBrief: func(hirePlace) {}, clock: time.Unix(1_000_000, 0)}
	if err := os.MkdirAll(filepath.Join(supportDir, "hires"), 0o700); err != nil {
		t.Fatal(err)
	}
	return r
}

// fake installs a fake t3 in ~/.local/bin: each run's arguments one to a line, then "--". flags make a step
// fail: addfail, elsewhere (add names another folder), eatfence (add deletes the fence), startfail, ownworktree
// (show has a T3 worktree), otherproject (show has another project), slow (show takes a second), removefail,
// gone (remove finds no project), busy (send queues), sendfail, authfail (Take over's sign-in, t3takeover_test.go),
// authgarbled (the sign-in answers what Smolcorp can't read; `auth session list` has one of Smolcorp's and one
// other). At `project add` it notes whether the
// worktree, the fence, git's ignoring it and the brief were there; `thread send` keeps its standard input.
func (r *t3Rig) fake(flags ...string) {
	r.t.Helper()
	bin := filepath.Join(r.home, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(r.flags, []byte(strings.Join(flags, " ")), 0o644); err != nil {
		r.t.Fatal(err)
	}
	q := func(path string) string { return "'" + path + "'" }
	script := `#!/bin/sh
for a in "$@"; do printf '%s\n' "$a" >> ` + q(r.calls) + `; done
printf -- '--\n' >> ` + q(r.calls) + `
has() { grep -qw "$1" ` + q(r.flags) + `; }
link='https://app.t3.codes/threads/env-1/mcp%3Aabc-1'
[ "$1" = "--version" ] && { echo "t3 v0.1.8"; exit 0; }
case "$1 $2" in
  "project add")
    n=''
    [ -d "$3" ] && n="$n worktree"
    [ -f "$3/.claude/settings.local.json" ] && n="$n fence"
    git -C "$3" check-ignore -q .claude/settings.local.json && n="$n ignored"
    [ -f ` + q(filepath.Join(r.home, "brief.md")) + ` ] && n="$n brief"
    echo "$n" >> ` + q(r.notes) + `
    has eatfence && rm -f "$3/.claude/settings.local.json"
    has addfail && { echo "Project already exists: $3" >&2; exit 1; }
    has elsewhere && { echo "Added project p-1 ($5) at /somewhere/else."; exit 0; }
    echo "Added project p-1 ($5) at $3."; exit 0 ;;
  "thread start")
    has startfail && { printf '  Could not reach the T3 Code desktop app.  \nmore\n' >&2; exit 1; }
    echo "{\"threadId\":\"mcp:abc-1\",\"runId\":\"run:1\",\"status\":\"starting\",\"link\":\"$link\"}"; exit 0 ;;
  "thread show")
    has slow && sleep 1
    wt=null; has ownworktree && wt='"/elsewhere/wt"'
    p=p-1; has otherproject && p=p-2
    echo "{\"thread\":{\"threadId\":\"$3\",\"projectId\":\"$p\",\"status\":\"running\",\"pendingRequestCount\":0,\"worktreePath\":$wt,\"branch\":null,\"link\":\"$link\"}}"; exit 0 ;;
  "thread send")
    cat > ` + q(r.stdin) + `
    has sendfail && { echo "No thread matches." >&2; exit 1; }
    has busy && { echo "{\"threadId\":\"$3\",\"status\":\"running\",\"delivery\":\"queued\"}"; exit 0; }
    echo "{\"threadId\":\"$3\",\"status\":\"starting\",\"delivery\":\"started\"}"; exit 0 ;;
  "auth session")
    has authfail && [ "$3" = issue ] && { echo '{"sessionId":"s-1","token":"fake-tok-leak"}'; echo "Could not open T3 Code's auth store." >&2; exit 1; }
    has authgarbled && [ "$3" = issue ] && { echo "Issued bearer access token s-9."; exit 0; }
    [ "$3" = list ] && { echo '[{"sessionId":"s-9","client":{"label":"` + t3CallLabel + `","deviceType":"bot"}},{"sessionId":"s-8","client":{"label":"Someone else","deviceType":"bot"}}]'; exit 0; }
    [ "$3" = issue ] && { echo '{"sessionId":"s-1","token":"fake-tok","method":"bearer","scopes":["orchestration:operate"]}'; exit 0; }
    echo "Revoked session $4."; exit 0 ;;
  "project remove")
    has removefail && { echo "Could not reach the T3 Code desktop app." >&2; exit 1; }
    has gone && { echo "No T3 Code project matches $3." >&2; exit 1; }
    echo "Removed project p-1 (x)."; exit 0 ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "t3"), []byte(script), 0o755); err != nil {
		r.t.Fatal(err)
	}
}

// runs are the fake t3's runs, each its arguments.
func (r *t3Rig) runs() [][]string {
	data, _ := os.ReadFile(r.calls)
	var out [][]string
	for _, run := range strings.Split(string(data), "--\n") {
		if run == "" {
			continue
		}
		out = append(out, slices.DeleteFunc(strings.Split(run, "\n"), func(s string) bool { return s == "" }))
	}
	return out
}

// firsts are each run's first two words.
func (r *t3Rig) firsts() [][]string {
	var out [][]string
	for _, run := range r.runs() {
		out = append(out, run[:min(2, len(run))])
	}
	return out
}

func (r *t3Rig) machine() t3Machine {
	return t3Machine{Home: r.home, GOOS: runtime.GOOS, Getenv: func(string) string { return "" },
		App: func() string { return "/fake/t3code" }, OpenApp: func(string) {}, Run: runT3, Feed: feedT3,
		OpenURL: func(link string) error { r.opened = append(r.opened, link); return nil },
		Pause:   func(time.Duration) {}, Now: time.Now}
}

func (r *t3Rig) git(dir string, args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v %s", args, err, out)
	}
}

// repo is a real repo, one commit, no remote.
func (r *t3Rig) repo() string {
	r.t.Helper()
	repo := filepath.Join(r.home, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		r.t.Fatal(err)
	}
	r.git(repo, "init", "-q", "-b", "main")
	stopSigning(r.t, repo)
	r.git(repo, "commit", "-q", "--allow-empty", "-m", "init")
	return repo
}

// settings are what hireSettingsObject makes for this hire, with a stand-in guard that prints the groups.
func (r *t3Rig) settings() map[string]any {
	r.t.Helper()
	object, err := hireSettingsObject(hireSettingsOptions{HireID: t3TestHireID, CLI: "/opt/smolcorp/bin/smolcorp", Support: supportDir,
		Run:    func(string, []string) (int, string, bool) { return 0, t3TestPrinted, true },
		IsExec: func(string) bool { return true }})
	if err != nil {
		r.t.Fatal(err)
	}
	return object
}

func (r *t3Rig) hiring(folder string) t3Hiring {
	return t3Hiring{HireID: t3TestHireID, Name: "Ada", Called: "Ada", Folder: folder, Model: "claude-haiku-5-5"}
}

func (r *t3Rig) ways() t3Ways {
	brief := filepath.Join(r.home, "brief.md")
	return t3Ways{Machine: r.machine(), PortFree: func(int) bool { return true },
		Settings: func() (map[string]any, error) { r.log = append(r.log, "settings"); return r.settings(), nil },
		Brief: func(place hirePlace) (t3Brief, error) {
			r.log = append(r.log, "brief")
			r.atBrief(place)
			if err := os.WriteFile(brief, []byte("# Brief"), 0o644); err != nil {
				return t3Brief{}, err
			}
			return t3Brief{Message: "Read your brief at " + brief + ", then start.", Path: brief}, nil
		},
		DiscardBrief: func() { r.log = append(r.log, "discard"); _ = os.Remove(brief) },
		Register:     func(place hirePlace) error { r.log = append(r.log, "register "+place.Branch); return nil },
		Started: func(thread, link, _ string) error {
			r.log = append(r.log, "started")
			r.startedWith = []string{thread, link}
			return nil
		},
		Fail: func(why string) error { r.log = append(r.log, "fail"); r.failed = append(r.failed, why); return nil },
	}
}

func (r *t3Rig) worktree() string {
	return filepath.Join(r.home, "repo-hires", "ada-abcdef")
}

func (r *t3Rig) branchExists(repo string) bool {
	return exec.Command("git", "-C", repo, "rev-parse", "--verify", "-q", "refs/heads/hire/ada-abcdef").Run() == nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func refusalOf(t *testing.T, err error) *hireRefusal {
	t.Helper()
	var refused *hireRefusal
	if !errors.As(err, &refused) {
		t.Fatalf("not refused: %v", err)
	}
	return refused
}

// The gate

func TestT3GateRefusesInOneSentenceEach(t *testing.T) {
	ok := t3Status{State: t3Found, Version: "v0.1.8"}
	cases := []struct {
		on     bool
		status t3Status
		want   string
	}{
		{true, ok, ""},
		{true, t3Status{State: t3CliOnly, Version: "v0.1.8"}, ""},
		{false, ok, "T3 Code is off in Settings: turn it on to hire into T3 Code."},
		{true, t3Status{State: t3None}, "Install T3 Code and its t3 command to hire into T3 Code (t3.codes)."},
		{true, t3Status{State: t3AppOnlyIs}, "Hiring into T3 Code needs its t3 command: in T3 Code, open Settings → About and install it."},
		{true, t3Status{State: t3Found, Version: "v0.2.0"}, "t3 v0.2.0 isn't a version Smolcorp's fence was proven on (v0.1.8), so it won't hire into T3 Code."},
		{true, t3Status{State: t3Found}, "Smolcorp couldn't read t3's version, so it won't hire into T3 Code."},
	}
	for _, c := range cases {
		if got := t3LaneRefusal(c.on, c.status); got != c.want {
			t.Errorf("%v %+v: %q", c.on, c.status, got)
		}
	}
}

func TestT3OnlyAClaudeCodeRoleOnT3sOwnAccountNeverTheGMNorATestCopy(t *testing.T) {
	role := Role{ID: "reviewer", Name: "Reviewer", Agent: "claude"}
	gm := Role{ID: "gm", Name: "GM", Agent: "claude"}
	codex := Role{ID: "reviewer", Name: "Reviewer", Agent: "codex"}
	cases := []struct {
		role     *Role
		account  string
		apiKey   bool
		testCopy bool
		want     string
	}{
		{&role, "", false, false, ""},
		{&role, "", false, true, "A test copy doesn't hire into T3 Code: that's your real T3 Code."},
		{nil, "", false, false, "Pick a role: a hire in T3 Code starts on its brief."},
		{&gm, "", false, false, "The General Manager isn't hired into T3 Code."},
		{&codex, "", false, false, "Only a Claude Code role is hired into T3 Code: the fence there is Claude Code's."},
		{&role, "work", false, false, "A hire in T3 Code works on T3 Code's own Claude account, so no account is picked for it."},
		{&role, "", true, false, "A hire in T3 Code works on T3 Code's own Claude account, so no account is picked for it."},
	}
	for _, c := range cases {
		if got := t3HireRefusal(c.role, c.account, c.apiKey, c.testCopy); got != c.want {
			t.Errorf("%+v: %q", c, got)
		}
	}
}

// Hiring

func TestT3TheHireGoesWorktreeFenceExcludeBriefProjectThreadInThatOrder(t *testing.T) {
	r := newT3Rig(t)
	r.fake()
	repo := r.repo()
	started, err := t3HireInto(r.hiring(repo), r.ways())
	if err != nil {
		t.Fatalf("refused: %v %v", err, r.failed)
	}
	wt := r.worktree()
	if started.Place.Worktree != wt || started.Place.Branch != "hire/ada-abcdef" {
		t.Errorf("place: %+v", started.Place)
	}
	link := "https://app.t3.codes/threads/env-1/mcp%3Aabc-1"
	if started.Thread != "mcp:abc-1" || started.Link != link {
		t.Errorf("started: %+v", started)
	}
	brief := filepath.Join(r.home, "brief.md")
	want := [][]string{
		{"project", "add", wt, "--title", "Ada · hire/ada-abcdef"},
		{"thread", "start", wt, "Read your brief at " + brief + ", then start.", "--provider", "claudeAgent", "--model", "claude-haiku-5-5", "--json"},
		{"thread", "show", "mcp:abc-1", "--json"},
	}
	if got := r.runs(); !reflect.DeepEqual(got, want) {
		t.Errorf("no --worktree, no --wait; the thread checked once started:\n%q\nwant\n%q", got, want)
	}
	// When T3 Code first heard of the folder, the worktree, the fence, git's ignoring it and the brief were all there.
	if notes, _ := os.ReadFile(r.notes); strings.TrimSpace(string(notes)) != "worktree fence ignored brief" {
		t.Errorf("at project add: %q", notes)
	}
	if !reflect.DeepEqual(r.log, []string{"register hire/ada-abcdef", "settings", "brief", "started"}) {
		t.Errorf("log: %q", r.log)
	}
	if !reflect.DeepEqual(r.startedWith, []string{"mcp:abc-1", link}) {
		t.Errorf("started with %q", r.startedWith)
	}
	// The fence is exactly what hireSettingsObject makes for this hire, written as the Mac writes it.
	fence := filepath.Join(wt, ".claude", "settings.local.json")
	written, err := os.ReadFile(fence)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written, swiftJSON(r.settings())) {
		t.Errorf("the fence isn't the hire's settings in the Mac's encoding:\n%s", written)
	}
	var object map[string]any
	if err := json.Unmarshal(written, &object); err != nil {
		t.Fatal(err)
	}
	deny := object["permissions"].(map[string]any)["deny"].([]any)
	for _, rule := range []string{"Bash(git push:*)", "Edit(**/.claude/settings*.json)"} {
		if !slices.Contains(deny, any(rule)) {
			t.Errorf("%s isn't denied", rule)
		}
	}
	hook := object["hooks"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)
	if command := hook["hooks"].([]any)[0].(map[string]any)["command"].(string); hook["matcher"] != "*" || !strings.Contains(command, " guard --rules ") {
		t.Errorf("the guard before every tool: %v", hook)
	}
	if object["disableAllHooks"] != false {
		t.Error("hooks stay on")
	}
	if info, _ := os.Stat(fence); info.Mode().Perm() != 0o600 {
		t.Errorf("the fence's mode is %v", info.Mode().Perm())
	}
	// Never committed: in the repo's info/exclude, once, and the worktree shows nothing new.
	exclude, _ := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if strings.Count("\n"+string(exclude), "\n.claude/settings.local.json\n") != 1 {
		t.Errorf("exclude: %q", exclude)
	}
	if !t3Exclude(wt, runGit) {
		t.Error("asked again, it isn't ignored")
	}
	if again, _ := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude")); !bytes.Equal(again, exclude) {
		t.Error("written twice")
	}
	if _, out, _ := runGit(wt, "status", "--porcelain", "--untracked-files=all"); out != "" {
		t.Errorf("the worktree shows %q", out)
	}
}

func TestT3AProjectAddThatFailsTakesEverythingBack(t *testing.T) {
	r := newT3Rig(t)
	r.fake("addfail")
	repo := r.repo()
	_, err := t3HireInto(r.hiring(repo), r.ways())
	refused := refusalOf(t, err)
	if refused.Message != "T3 Code didn't add the hire's worktree as a project: Project already exists: "+r.worktree() {
		t.Errorf("%q", refused.Message)
	}
	if got := r.firsts(); !reflect.DeepEqual(got, [][]string{{"project", "add"}}) {
		t.Errorf("no thread, and someone else's project is left alone: %q", got)
	}
	if exists(r.worktree()) || r.branchExists(repo) {
		t.Error("the worktree or its branch is left")
	}
	if !reflect.DeepEqual(r.log, []string{"register hire/ada-abcdef", "settings", "brief", "discard", "fail"}) || !reflect.DeepEqual(r.failed, []string{refused.Message}) {
		t.Errorf("%q %q", r.log, r.failed)
	}
}

func TestT3AThreadThatDoesntStartRemovesTheProjectThenTheRest(t *testing.T) {
	r := newT3Rig(t)
	r.fake("startfail")
	repo := r.repo()
	_, err := t3HireInto(r.hiring(repo), r.ways())
	if refused := refusalOf(t, err); refused.Message != "T3 Code didn't start the hire's thread: Could not reach the T3 Code desktop app." {
		t.Errorf("%q", refused.Message)
	}
	runs := r.runs()
	if got := r.firsts(); !reflect.DeepEqual(got, [][]string{{"project", "add"}, {"thread", "start"}, {"project", "remove"}}) ||
		!reflect.DeepEqual(runs[len(runs)-1], []string{"project", "remove", r.worktree(), "--force"}) {
		t.Errorf("%q", runs)
	}
	if exists(r.worktree()) || r.branchExists(repo) {
		t.Error("the worktree or its branch is left")
	}
	if got := r.log[len(r.log)-2:]; !reflect.DeepEqual(got, []string{"discard", "fail"}) {
		t.Errorf("%q", r.log)
	}
}

func TestT3AThreadThatCantBeConfirmedInTheWorktreeIsTakenBack(t *testing.T) {
	for _, flag := range []string{"elsewhere", "ownworktree"} {
		t.Run(flag, func(t *testing.T) {
			r := newT3Rig(t)
			r.fake(flag)
			repo := r.repo()
			_, err := t3HireInto(r.hiring(repo), r.ways())
			refused := refusalOf(t, err)
			runs := r.runs()
			if !reflect.DeepEqual(runs[len(runs)-1], []string{"project", "remove", r.worktree(), "--force"}) {
				t.Errorf("%q", runs)
			}
			want := "T3 Code didn't say it added the hire's worktree"
			if flag == "ownworktree" {
				want = "T3 Code put the hire's thread in a worktree of its own (/elsewhere/wt), not the hire's, so it was taken back with its project."
			}
			if !strings.HasPrefix(refused.Message, want) {
				t.Errorf("%q", refused.Message)
			}
			if exists(r.worktree()) || slices.Contains(r.log, "started") {
				t.Errorf("left: %q", r.log)
			}
		})
	}
}

func TestT3AProjectT3CodeKeepsKeepsItsFenceAndSaysSo(t *testing.T) {
	r := newT3Rig(t)
	r.fake("startfail", "removefail")
	_, err := t3HireInto(r.hiring(r.repo()), r.ways())
	refused := refusalOf(t, err)
	if !strings.HasSuffix(refused.Message, ` T3 Code didn't remove the project it was given (Could not reach the T3 Code desktop app.), so the hire's worktree and fence were kept: remove "Ada · hire/ada-abcdef" in T3 Code.`) {
		t.Errorf("%q", refused.Message)
	}
	if !exists(filepath.Join(r.worktree(), ".claude", "settings.local.json")) {
		t.Error("whatever T3 still runs there stays fenced")
	}
}

func TestT3AFenceFileTheRepoAlreadyHasIsNeverWrittenOver(t *testing.T) {
	r := newT3Rig(t)
	r.fake()
	repo := r.repo()
	// A repo that commits its own settings.local.json: ours would be committed with it.
	if err := os.MkdirAll(filepath.Join(repo, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".claude", "settings.local.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.git(repo, "add", "-f", ".claude/settings.local.json")
	r.git(repo, "commit", "-q", "-m", "settings")
	_, err := t3HireInto(r.hiring(repo), r.ways())
	if refused := refusalOf(t, err); refused.Message != "This repository keeps its own .claude/settings.local.json, so Smolcorp can't put the hire's fence there." {
		t.Errorf("%q", refused.Message)
	}
	if len(r.runs()) != 0 || exists(r.worktree()) {
		t.Errorf("T3 Code heard of it, or the worktree is left: %q", r.runs())
	}
	if data, _ := os.ReadFile(filepath.Join(repo, ".claude", "settings.local.json")); string(data) != "{}" {
		t.Error("the repo's own file was changed")
	}
}

func TestT3AFolderThatIsntARepoIsRefusedWithNothingMade(t *testing.T) {
	r := newT3Rig(t)
	r.fake()
	plain := filepath.Join(r.home, "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := t3HireInto(r.hiring(plain), r.ways())
	if refused := refusalOf(t, err); refused.Message != "A hire in T3 Code works in a worktree of its own: hire into a folder in a git repository." {
		t.Errorf("%q", refused.Message)
	}
	if len(r.runs()) != 0 || len(r.log) != 0 {
		t.Errorf("%q %q", r.runs(), r.log)
	}
}

func TestT3AProjectAddAnswerIsReadForThatFolderOnly(t *testing.T) {
	// T797a's real answer.
	if got := t3AddedProject("Added project e1715360-4223 (t797a-fence (x)) at /Users/a/wt-own.\n", "/Users/a/wt-own"); got != "e1715360-4223" {
		t.Errorf("%q", got)
	}
	if got := t3AddedProject("Added project e1 (Ada at work · hire/ada) at /Users/a/my at home/wt.", "/Users/a/my at home/wt"); got != "e1" {
		t.Errorf("%q", got)
	}
	if got := t3AddedProject("Added project e1 (Ada) at /Users/a/other.", "/Users/a/my at home/wt"); got != "" {
		t.Errorf("%q", got)
	}
	if got := t3AddedProject("Project already exists", "/x"); got != "" {
		t.Errorf("%q", got)
	}
}

// Following

func TestT3EachThreadStatusIsADeskStatus(t *testing.T) {
	for _, raw := range []string{"preparing", "queued", "starting", "running"} {
		if got := t3DeskStatus(map[string]any{"status": raw}); got != "working" {
			t.Errorf("%s: %s", raw, got)
		}
	}
	for _, raw := range []string{"completed", "failed", "interrupted", "cancelled", "rolled_back", "idle"} {
		if got := t3DeskStatus(map[string]any{"status": raw}); got != "done" {
			t.Errorf("%s: %s", raw, got)
		}
	}
	if t3DeskStatus(map[string]any{"status": "waiting"}) != "waiting" || t3DeskStatus(map[string]any{"status": "running", "pendingRequestCount": 1.0}) != "waiting" {
		t.Error("a question pending is waiting")
	}
	if t3DeskStatus(map[string]any{"status": "something new"}) != "unknown" {
		t.Error("something new")
	}
	if t3State("working") != "working" || t3State("waiting") != "working" || t3State("done") != "idle" || t3State("unknown") != "" {
		t.Error("states")
	}
}

func TestT3FollowingLooksAtEachOpenT3HireOnceAndKeepsItsLink(t *testing.T) {
	r := newT3Rig(t)
	r.fake()
	inT3 := Hire{ID: "h1", Name: "Ada", Folder: "/x", State: "working", Engine: t3Engine, T3Thread: "mcp:abc-1"}
	gone := Hire{ID: "h2", Name: "Bo", Folder: "/x", State: "fired", Engine: t3Engine, T3Thread: "mcp:old"}
	plain := Hire{ID: "h3", Name: "Cy", Folder: "/x", State: "working"}
	f := newT3Follower()
	seen := f.round([]Hire{inT3, gone, plain}, r.machine(), time.Now)
	if got := r.runs(); !reflect.DeepEqual(got, [][]string{{"thread", "show", "mcp:abc-1", "--json"}}) {
		t.Errorf("only the open T3 hire: %q", got)
	}
	want := []t3Seen{{"h1", t3Glance{Status: "working", Raw: "running", Link: "https://app.t3.codes/threads/env-1/mcp%3Aabc-1"}}}
	if !reflect.DeepEqual(seen, want) {
		t.Errorf("%+v", seen)
	}
	desk := t3DeskJSON(inT3, f)
	if desk["status"] != "working" || desk["tag"] != "EXPERIMENTAL · runs in T3 Code" {
		t.Errorf("%v", desk)
	}
	rows := t3Decorate([]map[string]any{{"id": "h1", "engine": "t3"}, {"id": "h3"}}, []Hire{inT3, plain}, f)
	if rows[0]["t3"] == nil || rows[1]["t3"] != nil {
		t.Errorf("nobody else's row changes: %v", rows)
	}
}

// B2: a tick while the last round is still out starts nothing, and a hire is looked at no more than every 10 s.
func TestT3FollowIsBoundedOneRoundAtATimeAndEachHireEveryTenSeconds(t *testing.T) {
	r := newT3Rig(t)
	r.fake("slow")
	hire := Hire{ID: "follow-1", Name: "Ada", Folder: "/x", State: "working", Engine: t3Engine, T3Thread: "mcp:abc-1"}
	f := newT3Follower()
	back := make(chan []t3Seen, 4)
	apply := func(seen []t3Seen, _ bool) { back <- seen }
	spawn := func(job func()) { go job() }
	if !f.tick([]Hire{hire}, r.machine(), time.Now, spawn, apply) {
		t.Fatal("the first tick began no round")
	}
	if f.tick([]Hire{hire}, r.machine(), time.Now, spawn, func([]t3Seen, bool) { t.Error("a second round while the first is out") }) {
		t.Error("a second round began")
	}
	if seen := <-back; len(seen) != 1 || seen[0].ID != "follow-1" {
		t.Errorf("%+v", seen)
	}
	shows := func() int {
		return len(slices.DeleteFunc(r.firsts(), func(run []string) bool { return !reflect.DeepEqual(run, []string{"thread", "show"}) }))
	}
	if shows() != 1 {
		t.Errorf("%d shows", shows())
	}
	// Right after: a round runs, but the hire isn't due again yet.
	if !f.tick([]Hire{hire}, r.machine(), time.Now, spawn, apply) {
		t.Fatal("no round")
	}
	if seen := <-back; len(seen) != 0 || shows() != 1 {
		t.Errorf("no second show within 10 s: %+v, %d", seen, shows())
	}
	// Ten seconds on, it is.
	if seen := f.round([]Hire{hire}, r.machine(), func() time.Time { return time.Now().Add(t3FollowEvery) }); len(seen) != 1 || shows() != 2 {
		t.Errorf("%+v, %d", seen, shows())
	}
}

// The desk's open

// t3RealLink is T797a's real link, from `t3 thread show --json`.
const t3RealLink = "https://app.t3.codes/threads/4e467189-667c-4dec-b427-6a0819b039cb/mcp%3A32872c1f-1f9c-4bbd-a31c-0a28a6248867"

func TestT3TheDeepLinkIsBuiltFromARealLink(t *testing.T) {
	cases := []struct{ thread, link, want string }{
		{"mcp:32872c1f-1f9c-4bbd-a31c-0a28a6248867", t3RealLink, "t3code://threads/4e467189-667c-4dec-b427-6a0819b039cb/mcp%3A32872c1f-1f9c-4bbd-a31c-0a28a6248867"},
		{"", t3RealLink, ""},
		{"mcp:x", "", ""},
		{"mcp:x", "https://app.t3.codes/projects/x", ""},
		{"mcp:x", "https://app.t3.codes/threads/a%2Fb/mcp%3Ax", ""},                                        // an env that isn't plain
		{"mcp:x", "https://app.t3.codes/threads/a&b;c/mcp%3Ax", ""},                                        // nor this
		{"mcp:a/b?c", t3RealLink, "t3code://threads/4e467189-667c-4dec-b427-6a0819b039cb/mcp%3Aa%2Fb%3Fc"}, // the id encoded whole
		{"é \"x\"", t3RealLink, "t3code://threads/4e467189-667c-4dec-b427-6a0819b039cb/%C3%A9%20%22x%22"},
	}
	for _, c := range cases {
		if got := t3DeepLink(c.thread, c.link); got != c.want {
			t.Errorf("%q %q: %q", c.thread, c.link, got)
		}
	}
}

func TestT3OpenInT3CodeOpensAT3HiresOwnThreadByItsLinkAndRunsNothing(t *testing.T) {
	r := newT3Rig(t)
	r.fake()
	hire := Hire{ID: "h9", Name: "Ada", Folder: "/nowhere", Worktree: "/nowhere/wt", State: "working", Engine: t3Engine,
		T3Thread: "mcp:32872c1f-1f9c-4bbd-a31c-0a28a6248867", T3Link: t3RealLink}
	status, _ := t3Open(hire, t3Status{State: t3Found, Version: "v0.1.8"}, r.machine())
	if status != http.StatusOK || !reflect.DeepEqual(r.opened, []string{"t3code://threads/4e467189-667c-4dec-b427-6a0819b039cb/mcp%3A32872c1f-1f9c-4bbd-a31c-0a28a6248867"}) {
		t.Errorf("%d %q", status, r.opened)
	}
	if len(r.runs()) != 0 {
		t.Errorf("no t3 app, no new thread: %q", r.runs())
	}
	if status, _ := t3Open(hire, t3Status{State: t3AppOnlyIs}, r.machine()); status != http.StatusOK {
		t.Error("the app alone is enough")
	}
	if status, why := t3Open(hire, t3Status{State: t3CliOnly, Version: "v0.1.8"}, r.machine()); status != http.StatusConflict ||
		why != "T3 Code's app isn't on this computer, so the hire's thread can't be shown there." {
		t.Errorf("%d %v", status, why)
	}
	hire.T3Link = ""
	if _, why := t3Open(hire, t3Status{State: t3Found}, r.machine()); why != "T3 Code hasn't given this hire's thread a link yet." {
		t.Errorf("%v", why)
	}
}

// Mail

// Several lines reach a T3 hire's thread as they are (I34): on t3's standard input, byte for byte, line
// feeds, indentation, a tab, accents and emoji kept, never typed anywhere, so no line break can press Return.
func TestT3MailOfSeveralLinesArrivesByteForByte(t *testing.T) {
	r := newT3Rig(t)
	hire := Hire{ID: "h1", Name: "Ada", Folder: "/x", State: "working", Engine: t3Engine, T3Thread: "mcp:abc-1"}
	text := "Deux lignes :\n    « ça marche » 👩‍💻 $(touch nope) `y` fin;\n\tet un tab\r\nfin"
	r.fake()
	if got := t3Mail(hire, text, func() bool { return true }, r.machine()); got != (mailResult{Outcome: mailSent}) {
		t.Errorf("%+v", got)
	}
	if got, _ := os.ReadFile(r.stdin); !bytes.Equal(got, []byte(text)) {
		t.Errorf("stdin: %q", got)
	}
	if exists(filepath.Join(r.home, "nope")) {
		t.Error("the text was run")
	}
}

func TestT3MailGoesOnStandardInputAndABusyThreadQueuesIt(t *testing.T) {
	r := newT3Rig(t)
	hire := Hire{ID: "h1", Name: "Ada", Folder: "/x", State: "working", Engine: t3Engine, T3Thread: "mcp:abc-1"}
	text := "— from the GM (Smolcorp), message m1\n\nPlease review; don't `rm -rf` anything. $(touch nope)"
	yes := func() bool { return true }
	r.fake()
	if got := t3Mail(hire, text, yes, r.machine()); got != (mailResult{Outcome: mailSent}) {
		t.Errorf("%+v", got)
	}
	if got := r.runs(); !reflect.DeepEqual(got, [][]string{{"thread", "send", "mcp:abc-1", "-", "--json"}}) {
		t.Errorf("the text never on the command line: %q", got)
	}
	if got, _ := os.ReadFile(r.stdin); string(got) != text {
		t.Errorf("stdin: %q", got)
	}
	if exists(filepath.Join(r.home, "nope")) {
		t.Error("the text was run")
	}
	r.fake("busy")
	if got := t3Mail(hire, text, yes, r.machine()); got != (mailResult{Outcome: mailSent, Note: "queued: Ada's thread in T3 Code was busy, so it's its next turn."}) {
		t.Errorf("%+v", got)
	}
	r.fake("sendfail")
	if got := t3Mail(hire, text, yes, r.machine()); got != (mailResult{Outcome: mailTrouble, Note: "T3 Code didn't take it: No thread matches."}) {
		t.Errorf("%+v", got)
	}
	none := hire
	none.T3Thread = ""
	if got := t3Mail(none, text, yes, r.machine()); got != (mailResult{Outcome: mailCannot, Note: "Ada has no T3 Code thread, so mail can't reach it."}) {
		t.Errorf("%+v", got)
	}
	_ = os.Remove(r.calls)
	if got := t3Mail(hire, text, func() bool { return false }, r.machine()); got != (mailResult{Outcome: mailTrouble, Note: "The outbox couldn't be written, so nothing was sent."}) || len(r.runs()) != 0 {
		t.Errorf("not counted, not sent: %+v %q", got, r.runs())
	}
}

func TestT3TheGMsMailToAT3HireTakesT3sWayAfterItIsCounted(t *testing.T) {
	r := newPumpRig(t)
	r.hires = append(r.hires, Hire{ID: "cccccc333333", Name: "Ada", State: "working", Engine: t3Engine, T3Thread: "mcp:abc-1"})
	var sent []string
	counted := true
	r.pump.t3 = func(h Hire, text string, commit func() bool) mailResult {
		if counted = commit(); !counted {
			return mailResult{Outcome: mailTrouble, Note: "The outbox couldn't be written, so nothing was sent."}
		}
		sent = append(sent, text)
		return mailResult{Outcome: mailSent, Note: "queued: Ada's thread in T3 Code was busy, so it's its next turn."}
	}
	mustAdd(t, r.o, toHire("cccccc333333", "Ada", "hello", "m1"))
	r.pump.round()
	if len(r.typed) != 0 || len(sent) != 1 || !strings.HasSuffix(sent[0], "hello") {
		t.Errorf("typed %q, sent %q", r.typed, sent)
	}
	if m := r.msg("m1"); m.State != mailDelivered || m.Attempts != 1 || m.Note != "queued: Ada's thread in T3 Code was busy, so it's its next turn." {
		t.Errorf("%+v", m)
	}
}

// Letting go

func (r *t3Rig) leaving(working ...[]int) (t3Leaving, func() int) {
	if len(working) == 0 {
		working = [][]int{{}}
	}
	looks := 0
	return t3Leaving{Machine: r.machine(),
		Working: func(string) ([]int, bool) {
			defer func() { looks++ }()
			return working[min(looks, len(working)-1)], true
		},
		Name:  func(int) string { return "zsh" },
		Pause: func(d time.Duration) { r.clock = r.clock.Add(d) },
		Now:   func() time.Time { return r.clock }}, func() int { return looks }
}

func (r *t3Rig) t3Hire(worktree string) Hire {
	return Hire{ID: "abcdef12", Name: "Ada", Role: "reviewer", Folder: r.home, Branch: "hire/ada-abcdef", Worktree: worktree,
		State: "working", Engine: t3Engine, T3Thread: "mcp:abc-1"}
}

func (r *t3Rig) fencedWorktree() string {
	r.t.Helper()
	wt := filepath.Join(r.home, "wt")
	if err := os.MkdirAll(filepath.Join(wt, ".claude"), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".claude", "settings.local.json"), []byte("{}"), 0o600); err != nil {
		r.t.Fatal(err)
	}
	return wt
}

func TestT3LettingGoRemovesTheProjectThenChecksNothingWorksThereThenTheFenceRecapAndRegistry(t *testing.T) {
	r := newT3Rig(t)
	r.fake()
	wt := r.fencedWorktree()
	leave, looks := r.leaving([]int{4242}, []int{})
	var gone []t3Gone
	left, err := t3LetGo(r.t3Hire(wt), leave, func(g t3Gone) { gone = append(gone, g) })
	if err != nil {
		t.Fatal(err)
	}
	if left.Note != "" || !reflect.DeepEqual(gone, []t3Gone{left}) {
		t.Errorf("%+v %+v", left, gone)
	}
	if got := r.runs(); !reflect.DeepEqual(got, [][]string{{"project", "remove", wt, "--force"}}) {
		t.Errorf("%q", got)
	}
	if looks() != 2 {
		t.Errorf("looked %d times: again once the process had left", looks())
	}
	if exists(filepath.Join(wt, ".claude", "settings.local.json")) || exists(filepath.Join(wt, ".claude")) || !exists(wt) {
		t.Error("the fence is taken away last, its folder with it; the worktree stays")
	}
	recap, _ := os.ReadFile(left.Recap)
	if !strings.Contains(string(recap), "- Worktree: "+wt+" (kept)") || !strings.Contains(string(recap), "- Branch: hire/ada-abcdef") {
		t.Errorf("recap:\n%s", recap)
	}
}

func TestT3LettingGoSaysPlainlyWhereItStoppedAndClaimsNothing(t *testing.T) {
	r := newT3Rig(t)
	wt := r.fencedWorktree()
	letGone := 0
	count := func(t3Gone) { letGone++ }
	r.fake("removefail")
	leave, _ := r.leaving()
	if _, err := t3LetGo(r.t3Hire(wt), leave, count); refusalOf(t, err).Message != "T3 Code didn't remove Ada's project (Could not reach the T3 Code desktop app.), so nothing else was changed: it may still be running there." {
		t.Errorf("%v", err)
	}
	// The process never leaves: the fence stays, nothing is marked.
	r.fake()
	leave, _ = r.leaving([]int{4242})
	if _, err := t3LetGo(r.t3Hire(wt), leave, count); refusalOf(t, err).Message != "T3 Code removed the project, but a process still works in Ada's worktree (zsh, PID 4242), so its fence was kept and it isn't let go yet: if that's yours, leave that folder and let go again." {
		t.Errorf("%v", err)
	}
	if !exists(filepath.Join(wt, ".claude", "settings.local.json")) || letGone != 0 {
		t.Error("the fence went, or it was let go")
	}
	// Processes that can't be looked at: the fence stays, said.
	leave.Working = func(string) ([]int, bool) { return nil, false }
	if _, err := t3LetGo(r.t3Hire(wt), leave, count); refusalOf(t, err).Message != "T3 Code removed the project, but Smolcorp can't see which processes work in Ada's worktree on this computer, so its fence was kept and it isn't let go yet." {
		t.Errorf("%v", err)
	}
	// A project already removed in T3 Code is as good as removed now.
	r.fake("gone")
	leave, _ = r.leaving()
	if _, err := t3LetGo(r.t3Hire(wt), leave, count); err != nil || letGone != 1 {
		t.Errorf("%v %d", err, letGone)
	}
	// Not a T3 hire, or gone already: nothing run.
	_ = os.Remove(r.calls)
	plain := r.t3Hire(wt)
	plain.Engine = ""
	if _, err := t3LetGo(plain, leave, count); refusalOf(t, err).Message != "Ada doesn't run in T3 Code." {
		t.Errorf("%v", err)
	}
	fired := r.t3Hire(wt)
	fired.State = "fired"
	if _, err := t3LetGo(fired, leave, count); refusalOf(t, err).Message != "Ada is already gone." {
		t.Errorf("%v", err)
	}
	if len(r.runs()) != 0 {
		t.Errorf("%q", r.runs())
	}
	if t3LetGoConfirm != "T3 Code's thread for this hire, and its history in T3 Code, are removed. Its worktree and branch stay." {
		t.Error("the confirm")
	}
}

// N5: a fence that can't be taken away is said, not passed over.
func TestT3LettingGoSaysSoWhenTheFenceStays(t *testing.T) {
	r := newT3Rig(t)
	if os.Geteuid() == 0 {
		t.Skip("root removes a file from a read-only folder")
	}
	r.fake()
	wt := r.fencedWorktree()
	claude := filepath.Join(wt, ".claude")
	if err := os.Chmod(claude, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(claude, 0o755) })
	var notes []string
	leave, _ := r.leaving()
	gone, err := t3LetGo(r.t3Hire(wt), leave, func(g t3Gone) { notes = append(notes, g.Note) })
	if err != nil {
		t.Fatal(err)
	}
	want := "Ada's fence couldn't be taken out of its worktree, so that folder is still fenced: " + filepath.Join(wt, ".claude", "settings.local.json") + "."
	if gone.Note != want || !reflect.DeepEqual(notes, []string{want}) || !exists(filepath.Join(claude, "settings.local.json")) {
		t.Errorf("%q %q", gone.Note, notes)
	}
}

// N11: a hire left starting with no thread at launch is let go as Let go does, then marked failed, saying what was done.
func TestT3AHireLeftPartWayAtLaunchIsLetGoAndMarkedFailedHonestly(t *testing.T) {
	r := newT3Rig(t)
	wt := r.fencedWorktree()
	left := r.t3Hire(wt)
	left.State, left.T3Thread = "starting", ""
	started := r.t3Hire(wt)
	started.ID = "other123"
	r.fake("gone")
	leave, _ := r.leaving()
	reasons := t3Reconcile([]Hire{left, started}, leave)
	if len(reasons) != 1 || reasons[0].ID != "abcdef12" ||
		reasons[0].Reason != "Smolcorp closed while hiring it into T3 Code, so at the next launch its T3 Code project (if it had one) was removed and its fence taken away. Its worktree and branch stay." {
		t.Errorf("only the one with no thread: %+v", reasons)
	}
	if got := r.runs(); !reflect.DeepEqual(got, [][]string{{"project", "remove", wt, "--force"}}) || exists(filepath.Join(wt, ".claude", "settings.local.json")) {
		t.Errorf("%q", got)
	}
	// T3 Code won't remove it: said, and the fence stays.
	r.fencedWorktree()
	r.fake("removefail")
	if got := t3Reconcile([]Hire{left}, leave); len(got) != 1 || got[0].Reason != "Smolcorp closed while hiring it into T3 Code, and at the next launch it couldn't be let go: T3 Code didn't remove Ada's project (Could not reach the T3 Code desktop app.), so nothing else was changed: it may still be running there." {
		t.Errorf("%+v", got)
	}
	if !exists(filepath.Join(wt, ".claude", "settings.local.json")) {
		t.Error("the fence went")
	}
}

// The registry

func TestT3ARecordSaysT3OnlyWhenItIs(t *testing.T) {
	plain := Hire{ID: "h1", Name: "Ada", Folder: "/x"}
	data, _ := json.Marshal(plain)
	var keys map[string]any
	_ = json.Unmarshal(data, &keys)
	for _, key := range []string{"engine", "t3Thread", "t3Link"} {
		if _, found := keys[key]; found {
			t.Errorf("%s written for a plain hire", key)
		}
	}
	inT3 := Hire{ID: "h2", Name: "Bo", Folder: "/x", Engine: t3Engine, T3Thread: "mcp:abc-1", T3Link: t3RealLink}
	data, _ = json.Marshal(inT3)
	var back Hire
	if json.Unmarshal(data, &back) != nil || back != inT3 {
		t.Errorf("%+v", back)
	}
	rows := buildTeam([]Hire{plain, inT3}, nil, 0, func(string) string { return "" })
	if _, found := rows[0]["engine"]; found || rows[1]["engine"] != "t3" || rows[1]["t3Thread"] != "mcp:abc-1" {
		t.Errorf("%v", rows)
	}
}

// Fix round 1 (FX797b)

// B1: the fence is checked again right before project add: gone or changed there, nothing reaches T3 Code.
func TestT3AFenceGoneBeforeProjectAddStartsNothing(t *testing.T) {
	for _, how := range []string{"gone", "changed"} {
		t.Run(how, func(t *testing.T) {
			r := newT3Rig(t)
			r.fake()
			repo := r.repo()
			r.atBrief = func(place hirePlace) {
				fence := filepath.Join(place.Worktree, ".claude", "settings.local.json")
				if how == "gone" {
					_ = os.Remove(fence)
				} else {
					_ = os.WriteFile(fence, []byte(`{"disableAllHooks":false}`), 0o600)
				}
			}
			_, err := t3HireInto(r.hiring(repo), r.ways())
			if refusalOf(t, err).Message != t3FenceLost {
				t.Errorf("%v", err)
			}
			if len(r.runs()) != 0 || exists(r.worktree()) || slices.Contains(r.log, "started") {
				t.Errorf("T3 Code heard of it: %q %q", r.runs(), r.log)
			}
		})
	}
}

// B1: and again right before thread start: a fence gone after project add means no thread, and the project removed.
func TestT3AFenceGoneBeforeThreadStartStartsNoThread(t *testing.T) {
	r := newT3Rig(t)
	r.fake("eatfence")
	repo := r.repo()
	_, err := t3HireInto(r.hiring(repo), r.ways())
	if refusalOf(t, err).Message != t3FenceLost {
		t.Errorf("%v", err)
	}
	want := [][]string{{"project", "add", r.worktree(), "--title", "Ada · hire/ada-abcdef"}, {"project", "remove", r.worktree(), "--force"}}
	if got := r.runs(); !reflect.DeepEqual(got, want) {
		t.Errorf("no thread start: %q", got)
	}
	if exists(r.worktree()) || r.branchExists(repo) {
		t.Error("left")
	}
}

// B1: Let go while the hire is still being hired is refused in one sentence, and changes nothing; once hired, it can be.
func TestT3LetGoDuringAHireIsRefused(t *testing.T) {
	r := newT3Rig(t)
	r.fake()
	repo := r.repo()
	var during error
	r.atBrief = func(place hirePlace) {
		hire := Hire{ID: t3TestHireID, Name: "Ada", Role: "reviewer", Folder: repo, Branch: place.Branch, Worktree: place.Worktree, State: "starting", Engine: t3Engine}
		if !t3IsHiring(t3TestHireID) {
			t.Error("not in flight")
		}
		leave, _ := r.leaving()
		_, during = t3LetGo(hire, leave, func(t3Gone) { t.Error("let go during its hire") })
	}
	if _, err := t3HireInto(r.hiring(repo), r.ways()); err != nil {
		t.Fatalf("refused: %v", err)
	}
	if refused := refusalOf(t, during); refused.Status != http.StatusConflict || refused.Message != "Ada is still being hired, try again in a moment." {
		t.Errorf("%+v", refused)
	}
	if slices.ContainsFunc(r.firsts(), func(run []string) bool { return reflect.DeepEqual(run, []string{"project", "remove"}) }) ||
		!exists(filepath.Join(r.worktree(), ".claude", "settings.local.json")) {
		t.Error("its project or fence was touched")
	}
	if t3IsHiring(t3TestHireID) {
		t.Error("hired: still in flight")
	}
}

// N4: a main checkout whose own local settings switch hooks off is refused, with nothing made.
func TestT3AMainCheckoutWithHooksSwitchedOffIsRefused(t *testing.T) {
	r := newT3Rig(t)
	r.fake()
	repo := r.repo()
	local := filepath.Join(repo, ".claude", "settings.local.json")
	_ = os.MkdirAll(filepath.Dir(local), 0o755)
	_ = os.WriteFile(local, []byte(`{"disableAllHooks": true, "permissions": {"allow": ["Bash(ls:*)"]}}`), 0o644)
	if !t3HooksSwitchedOff(repo, runGit) {
		t.Error("not seen")
	}
	_, err := t3HireInto(r.hiring(repo), r.ways())
	if refusalOf(t, err).Message != t3HooksOff || len(r.runs()) != 0 || len(r.log) != 0 || exists(r.worktree()) {
		t.Errorf("%v %q %q", err, r.runs(), r.log)
	}
	// Off is false (or not said): the hire goes on.
	_ = os.WriteFile(local, []byte(`{"disableAllHooks": false}`), 0o644)
	if t3HooksSwitchedOff(repo, runGit) {
		t.Error("false is on")
	}
	if _, err := t3HireInto(r.hiring(repo), r.ways()); err != nil {
		t.Errorf("refused: %v", err)
	}
}

// N1: a thread T3 Code put in another project is said plainly: removing the hire's own project doesn't stop it.
func TestT3AThreadInAnotherProjectIsSaidPlainly(t *testing.T) {
	r := newT3Rig(t)
	r.fake("otherproject")
	_, err := t3HireInto(r.hiring(r.repo()), r.ways())
	message := refusalOf(t, err).Message
	if message != "T3 Code started the hire's thread in another project (p-2), not in its worktree; Smolcorp removed only the hire's own project, and T3 Code has no stop, so that thread may still be running there, outside the fence: stop it in T3 Code." {
		t.Errorf("%q", message)
	}
	runs := r.runs()
	if !reflect.DeepEqual(runs[len(runs)-1], []string{"project", "remove", r.worktree(), "--force"}) || strings.Contains(message, "taken back") {
		t.Errorf("%q", runs)
	}
}

// N7: project add with no answer may have added it all the same: it's removed too ("no such project" is fine).
func TestT3AProjectAddWithNoAnswerIsRemovedAnyway(t *testing.T) {
	for _, flag := range []string{"", "gone"} {
		t.Run("flag "+flag, func(t *testing.T) {
			r := newT3Rig(t)
			r.fake(flag)
			repo := r.repo()
			ways := r.ways()
			real := ways.Machine.Run
			ways.Machine.Run = func(program string, args []string, within time.Duration) t3Ran {
				ran := real(program, args, within)
				if len(args) >= 2 && args[0] == "project" && args[1] == "add" {
					return t3Ran{}
				}
				return ran
			}
			_, err := t3HireInto(r.hiring(repo), ways)
			if refusalOf(t, err).Message != "T3 Code didn't add the hire's worktree as a project: it didn't answer" {
				t.Errorf("%v", err)
			}
			if got := r.firsts(); !reflect.DeepEqual(got, [][]string{{"project", "add"}, {"project", "remove"}}) || exists(r.worktree()) {
				t.Errorf("%q", got)
			}
		})
	}
}

// Processes, read from the system

func TestT3AProcessInTheWorktreeIsFoundAndNamed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("covered by t3WorksIn's own test; no sleep here")
	}
	isolate(t)
	home, _ := filepath.EvalSymlinks(t.TempDir())
	wt := filepath.Join(home, "busy wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	// Its folder the worktree, and the worktree given with --add-dir, as T3 Code's agent has them.
	sleeper := exec.Command("/bin/sh", "-c", "exec sleep 30", "sh", "--add-dir", wt)
	sleeper.Dir = wt
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sleeper.Process.Kill(); _ = sleeper.Wait() })
	pid := sleeper.Process.Pid
	deadline := time.Now().Add(5 * time.Second)
	for processName(pid) != "sleep" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := processName(pid); got != "sleep" {
		t.Errorf("named %q", got)
	}
	if processName(-1) != "" {
		t.Error("no such process is named")
	}
	found, looked := processesWorkingIn(wt)
	switch runtime.GOOS {
	case "darwin":
		// Without the Mac app (none here), this engine can't read a process's folder: the answer is "can't look",
		// so Let go keeps the fence. Under the app it asks it (t3process_darwin_test.go).
		if looked {
			t.Error("said it looked")
		}
	default:
		if !looked || !slices.Contains(found, pid) {
			t.Errorf("%v %v", found, looked)
		}
		if found, _ := processesWorkingIn(filepath.Join(home, "elsewhere")); slices.Contains(found, pid) {
			t.Error("found elsewhere")
		}
	}
}

func TestT3WorksInByFolderOrAddDir(t *testing.T) {
	// The cases in this system's paths, as its callers give them (t3process_*.go): only a path with a drive is
	// full on Windows, so there /home/a/wt is C:\home\a\wt.
	sep, at := "/", func(path string) string { return path }
	if runtime.GOOS == "windows" {
		sep, at = `\`, func(path string) string { return `C:` + strings.ReplaceAll(path, "/", `\`) }
	}
	folders := []string{at("/home/a/wt")}
	cases := []struct {
		cwd  string
		args []string
		want bool
	}{
		{at("/home/a/wt"), nil, true},
		{at("/home/a/wt/src"), nil, true},
		{at("/home/a/wt-2"), nil, false},
		{at("/home/a"), []string{"claude", "--add-dir", at("/home/a/wt")}, true},
		{at("/home/a"), []string{"claude", "--add-dir=" + at("/home/a/wt/")}, true},
		{at("/home/a"), []string{"claude", "--add-dir"}, false},
		{"", []string{at("/home/a/wt")}, false},
	}
	for _, c := range cases {
		if got := t3WorksIn(folders, c.cwd, c.args, sep, false); got != c.want {
			t.Errorf("%+v: %v", c, got)
		}
	}
	// Windows: case ignored, its separator, a trailing one (its paths are only absolute there).
	if runtime.GOOS == "windows" && (!t3WorksIn([]string{`C:\Users\a\wt`}, `c:\users\A\WT\`, nil, `\`, true) ||
		t3WorksIn([]string{`C:\Users\a\wt`}, `C:\Users\a\wt2\`, nil, `\`, true)) {
		t.Error("Windows paths")
	}
}

// Pinned, and the shared fixture

// t3RunsIn are the argument lists a file runs t3 with (.Run and .Feed), its comments left out.
func t3RunsIn(t *testing.T, file, call string) []string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var code []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "//") {
			code = append(code, line)
		}
	}
	var lists []string
	for _, part := range strings.Split(strings.Join(code, "\n"), call)[1:] {
		end := strings.Index(part, "}")
		lists = append(lists, part[:end+1])
	}
	return lists
}

// t3 runs with these argument lists only, as lists, never through a shell (the Mac's five, and its one feed).
func TestT3RunsWithItsPinnedArgumentListsOnly(t *testing.T) {
	runs := t3RunsIn(t, "t3desk.go", ".Run(program, ")
	want := []string{
		`[]string{"project", "remove", worktree, "--force"}`,
		`[]string{"project", "add", worktree, "--title", title}`,
		`[]string{"thread", "start", worktree, brief.Message, "--provider", t3Provider, "--model", h.Model, "--json"}`,
		`[]string{"thread", "show", thread, "--json"}`,
		`[]string{"project", "remove", h.Worktree, "--force"}`,
	}
	if !reflect.DeepEqual(runs, want) {
		t.Errorf("runs:\n%q", runs)
	}
	if feeds := t3RunsIn(t, "t3desk.go", ".Feed(program, "); !reflect.DeepEqual(feeds, []string{`[]string{"thread", "send", h.T3Thread, "-", "--json"}`}) {
		t.Errorf("feeds: %q", feeds)
	}
	for _, file := range []string{"t3desk.go", "t3deskdeck.go", "t3process_linux.go", "t3process_darwin.go", "t3process_windows.go", "t3process_other.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		code := regexp.MustCompile(`(?m)^\s*//.*$`).ReplaceAllString(string(data), "")
		if file == "t3desk.go" {
			if n := strings.Count(code, ".Run("); n != len(want) {
				t.Errorf("%d runs: no other", n)
			}
			if n := strings.Count(code, ".Feed("); n != 1 {
				t.Errorf("%d feeds", n)
			}
		} else if strings.Contains(code, ".Run(") || strings.Contains(code, ".Feed(") {
			t.Errorf("%s runs t3", file)
		}
		for _, never := range []string{"--worktree", "os/exec", "exec.Command", "npx", "/bin/sh", "bash", `"-c"`, "brew ", "npm", "curl", `"ps"`, "lsof", "wmic", "powershell"} {
			if strings.Contains(code, never) {
				t.Errorf("%s: %s", file, never)
			}
		}
	}
	// The worktree always comes from the registry or from what Smolcorp made, never from a request.
	server, _ := os.ReadFile("server.go")
	at := strings.Index(string(server), `path == "/setup/t3code/letgo":`)
	body := string(server)[at : at+300]
	if !strings.Contains(body, `id, _ := decodeJSON(r.Body)["id"].(string)`) || !strings.Contains(body, "d.letGoOfT3(id)") ||
		strings.Contains(body, "folder") || strings.Contains(body, `"worktree"`) || strings.Contains(body, "path)") {
		t.Errorf("the let go route:\n%s", body)
	}
}

func TestT3LetGoIsThePagesAlone(t *testing.T) {
	i := slices.IndexFunc(routeTable, func(r routeRow) bool { return r.method == "POST" && r.path == "/setup/t3code/letgo" })
	if i < 0 || !reflect.DeepEqual(routeTable[i].callers, []string{"page"}) {
		t.Errorf("not the page's alone: %d", i)
	}
}

// Tests/fixtures/frontdesk/t3-fence.json is the fence as the Mac's T3Desk.writeFence writes it for fixed inputs
// (T3FenceFixtureTests.swift writes it). This engine writes the same bytes from the same inputs.
func TestT3TheFenceIsTheMacsByteForByte(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "frontdesk", "t3-fence.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		CLI       string `json:"cli"`
		FenceName string `json:"fenceName"`
		Cases     []struct {
			Name     string              `json:"name"`
			HireID   string              `json:"hireID"`
			Auto     bool                `json:"auto"`
			Printed  string              `json:"printed"`
			Support  string              `json:"support"`
			Profiles []map[string]string `json:"profiles"`
			Others   []string            `json:"others"`
			Entries  []string            `json:"entries"`
			Fence    string              `json:"fence"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.FenceName != t3FenceName || len(fixture.Cases) < 2 {
		t.Fatalf("%q, %d cases", fixture.FenceName, len(fixture.Cases))
	}
	if runtime.GOOS == "windows" {
		t.Skip("Windows writes its paths as //c/…, so the fixture's aren't its")
	}
	for _, c := range fixture.Cases {
		rules := filepath.Join(c.Support, hireGuardFile)
		groups := guardGroups(fixture.CLI, rules, guardHire, func(string, []string) (int, string, bool) { return 0, c.Printed, true })
		var profiles []claudeProfile
		for _, p := range c.Profiles {
			profiles = append(profiles, claudeProfile{Name: p["name"], Dir: p["dir"]})
		}
		settings := hireSettingsValue(hireDenyRules(c.HireID, groups, c.Support, profiles, c.Others, c.Entries, "darwin"), guardHookCommand(fixture.CLI, rules, guardHire), c.Auto, nil)
		fence := filepath.Join(t.TempDir(), filepath.FromSlash(t3FenceName))
		if !t3WriteFence(settings, fence) {
			t.Fatalf("%s: not written", c.Name)
		}
		if got, _ := os.ReadFile(fence); string(got) != c.Fence {
			gotLines, wantLines := strings.Split(string(got), "\n"), strings.Split(c.Fence, "\n")
			for i := range min(len(gotLines), len(wantLines)) {
				if gotLines[i] != wantLines[i] {
					t.Errorf("%s: line %d is\n  %s\nthe Mac's is\n  %s", c.Name, i+1, gotLines[i], wantLines[i])
					break
				}
			}
			t.Errorf("%s: %d lines, the Mac's %d", c.Name, len(gotLines), len(wantLines))
		}
	}
	// The fence a Go hire gets is that: auto mode as the Mac's, on a Mac only.
	if !t3FenceAuto("darwin") || t3FenceAuto("linux") || t3FenceAuto("windows") {
		t.Error("auto mode")
	}
}

// FXM5c: a thread the registry couldn't keep isn't left only in memory: the hire is taken back, its project
// removed, and the registry told why. When that can't be written either, the answer says so, and that the next
// launch lets it go; t3MarkFailed says when the registry couldn't be written.
func TestT3AThreadTheRegistryCantKeepIsTakenBack(t *testing.T) {
	r := newT3Rig(t)
	r.fake()
	repo := r.repo()
	ways := r.ways()
	started := ways.Started
	ways.Started = func(thread, link, brief string) error {
		_ = started(thread, link, brief)
		return errors.New("disk full")
	}
	_, err := t3HireInto(r.hiring(repo), ways)
	refused := refusalOf(t, err)
	if refused.Status != http.StatusInternalServerError || refused.Message != "The registry couldn't be written, so the hire was taken back." {
		t.Errorf("%+v", refused)
	}
	runs := r.runs()
	if !reflect.DeepEqual(runs[len(runs)-1], []string{"project", "remove", r.worktree(), "--force"}) || exists(r.worktree()) || r.branchExists(repo) {
		t.Errorf("not taken back: %q", runs)
	}
	if !reflect.DeepEqual(r.failed, []string{refused.Message}) {
		t.Errorf("%q", r.failed)
	}

	r = newT3Rig(t)
	r.fake("addfail")
	ways = r.ways()
	ways.Fail = func(string) error { return errors.New("disk full") }
	_, err = t3HireInto(r.hiring(r.repo()), ways)
	if refused := refusalOf(t, err); !strings.HasSuffix(refused.Message, " The registry couldn't be written either, so it shows as starting until Smolcorp's next launch lets it go.") {
		t.Errorf("%q", refused.Message)
	}

	h := r.t3Hire(r.worktree())
	t3Registered(t, h)
	blocker := registryPath() + ".tmp"
	if err := os.Mkdir(blocker, 0o700); err != nil {
		t.Fatal(err)
	}
	if t3MarkFailed(registryPath(), h.ID, "why") == nil {
		t.Error("an unwritten registry said nothing")
	}
	_ = os.Remove(blocker)
	if err := t3MarkFailed(registryPath(), h.ID, "why"); err != nil {
		t.Error(err)
	}
	if after, _ := registryHire(h.ID); after.State != "failed" || after.Notes != "why" {
		t.Errorf("%+v", after)
	}
}
