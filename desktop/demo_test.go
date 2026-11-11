package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// demoTestFolder is a folder for one test's demo inside the test's home folder (a temp one, TestMain's),
// with the demo's repos never signing.
func demoTestFolder(t *testing.T) string {
	t.Helper()
	in := filepath.Join(homeDir, "demo-"+strings.NewReplacer("/", "-", " ", "-").Replace(t.Name()))
	if err := os.MkdirAll(in, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(in) })
	saved := demoAfterInit
	demoAfterInit = func(repo string) { stopSigning(t, repo) }
	t.Cleanup(func() { demoAfterInit = saved })
	return resolveExisting(in)
}

func dryRun(t *testing.T, in string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	code := runDemo(context.Background(), demoRun{in: in, dry: true, team: standInTeam{}, stdin: strings.NewReader(""), out: &out})
	return code, out.String()
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := demoGit(dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestTheDryRunGoesEndToEndWithStandIns(t *testing.T) {
	in := demoTestFolder(t)
	code, out := dryRun(t, in)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	root := filepath.Join(in, demoFolderName)
	repo := filepath.Join(root, demoRepoName)
	for _, want := range []string{"wrote the plan", "hired a Bug hunter", "hired a Tester", "hired a Docs writer", "reviewed hire/docs-3",
		"refused a hire into your home folder", "refused a push", "refused a file written outside the demo", "Nothing shipped",
		"Waiting for your OK: hire/bughunter-1, hire/docs-3, hire/tester-2."} {
		if !strings.Contains(out, want) {
			t.Errorf("the dry run doesn't say %q:\n%s", want, out)
		}
	}
	for _, file := range []string{"PLAN.md", demoReportName, "app/TASKS.md", "app/bill.py", "app/.gitignore", "app/LICENSE", "app/.claude/settings.json"} {
		if _, err := os.Stat(filepath.Join(root, file)); err != nil {
			t.Errorf("%s: %v", file, err)
		}
	}
	// One hire per task, each on its own branch, in a worktree inside the demo folder; main untouched.
	if n := gitOut(t, repo, "rev-list", "--count", "main"); n != "1" {
		t.Errorf("main has %s commits, want the sample's one", n)
	}
	for _, line := range strings.Split(gitOut(t, repo, "worktree", "list", "--porcelain"), "\n") {
		if tree, ok := strings.CutPrefix(line, "worktree "); ok && !within(root, resolveExisting(tree)) {
			t.Errorf("a worktree outside the demo folder: %s", tree)
		}
	}
	for _, branch := range []string{"hire/bughunter-1", "hire/tester-2", "hire/docs-3"} {
		if n := gitOut(t, repo, "rev-list", "--count", "main.."+branch); n != "1" {
			t.Errorf("%s has %s commits of its own, want 1", branch, n)
		}
	}
	if remotes := gitOut(t, repo, "remote"); remotes != "" {
		t.Errorf("remotes: %q", remotes)
	}
	// Nothing outside the demo folder: the folder it was made in holds the demo folder only.
	entries, _ := os.ReadDir(in)
	if len(entries) != 1 || entries[0].Name() != demoFolderName {
		t.Errorf("the folder holds %v", entries)
	}
	// The project's fence is in its committed settings, aimed at this demo folder.
	settings, _ := os.ReadFile(filepath.Join(repo, ".claude", "settings.json"))
	if !strings.Contains(string(settings), "demo fence --root") || !strings.Contains(string(settings), root) || !strings.Contains(string(settings), "bughunter,tester,docs,reviewer") {
		t.Errorf("the settings: %s", settings)
	}
	if tracked := gitOut(t, repo, "ls-files", ".claude/settings.json"); tracked == "" {
		t.Error("the fence isn't committed: a hire's worktree wouldn't have it")
	}
	// A second run makes a new folder: one is never reused.
	if code, out := dryRun(t, in); code != 0 || !strings.Contains(out, filepath.Join(in, demoFolderName+"-2")) {
		t.Errorf("the second run: %d\n%s", code, out)
	}
	// With Python here, the sample's tests pass on main and on each stand-in's branch.
	python, err := exec.LookPath("python3")
	if err != nil || runtime.GOOS == "windows" { // Windows may answer with the Store's stand-in for Python
		t.Log("no python3 to run here: the sample's tests aren't run")
		return
	}
	for _, tree := range []string{repo, filepath.Join(root, "app-hires", "bughunter-1"), filepath.Join(root, "app-hires", "tester-2"), filepath.Join(root, "app-hires", "docs-3")} {
		cmd := exec.Command(python, "-m", "unittest", "-q")
		cmd.Dir, cmd.Env = tree, append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("the tests in %s: %v\n%s", tree, err, out)
		}
	}
}

func TestTheDemoNeverPushes(t *testing.T) {
	in := demoTestFolder(t)
	if code, out := dryRun(t, in); code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	repo := filepath.Join(in, demoFolderName, demoRepoName)
	// Even given a remote (by the test, never the demo), the project's own hook refuses every push.
	bare := filepath.Join(in, "remote.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	gitOut(t, repo, "remote", "add", "origin", bare)
	if status, out, _ := runGit(repo, "push", "origin", "hire/bughunter-1", "main"); status == 0 || !strings.Contains(out, "never pushes") {
		t.Errorf("the push went: %d %s", status, out)
	}
	if refs := gitOut(t, bare, "for-each-ref"); refs != "" {
		t.Errorf("the remote got: %s", refs)
	}
	// And the closing check says so, rather than "nothing shipped".
	var out bytes.Buffer
	start := gitOut(t, repo, "rev-parse", "main")
	if demoShipped(demoFolder{root: filepath.Join(in, demoFolderName), repo: repo, start: start}, &out) != 1 || !strings.Contains(out.String(), "has a remote now") {
		t.Errorf("with a remote: %s", out.String())
	}
}

func TestTheDemosFenceRefusesOutsideTheDemoFolder(t *testing.T) {
	in := demoTestFolder(t)
	root := filepath.Join(in, "smolcorp-demo")
	repo := filepath.Join(root, "app")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(in, "elsewhere")
	os.MkdirAll(elsewhere, 0o755)
	// A link inside the demo that leads out of it (where the system lets a test make one).
	linked := os.Symlink(elsewhere, filepath.Join(root, "link")) == nil
	f := demoFence{root: root, roles: []string{"tester", "reviewer"}, home: homeDir}
	write := func(path string) demoCall {
		input, _ := json.Marshal(map[string]string{"file_path": path})
		return demoCall{ToolName: "Write", ToolInput: input, Cwd: repo}
	}
	tool := func(name string, input map[string]any) demoCall {
		data, _ := json.Marshal(input)
		return demoCall{ToolName: name, ToolInput: data, Cwd: repo}
	}
	allowed := []demoCall{
		bashCall("smolcorp gm hire --role tester --folder "+shellPath(repo)+" --worktree --task 'fix it'", repo),
		bashCall("smolcorp gm hire --role=reviewer --folder="+repo, repo),
		bashCall("smolcorp gm hire --role tester --folder .", repo),
		bashCall("smolcorp gm fire tester-1", repo),
		bashCall("python3 -m unittest && git diff main...hire/tester-1", repo),
		write(filepath.Join(repo, "new-file.py")),
		write("notes/plan.md"),
		tool("Read", map[string]any{"file_path": "/etc/hosts"}),
		tool("mcp__smolcorp__hire", map[string]any{"role": "tester", "folder": repo}),
		// On the default account, said or not; a $ in single quotes is text, not the shell's.
		bashCall("smolcorp gm hire --role tester --folder "+repo+" --account default", repo),
		bashCall("smolcorp gm hire --role tester --folder "+repo+" --task 'parse_amount reads $12.50'", repo),
		tool("mcp__smolcorp__hire", map[string]any{"role": "tester", "folder": repo, "account": "default", "api-key": false}),
		// A $ in a line that doesn't name smolcorp, though its paths do (smolcorp-demo).
		bashCall("ls "+root+" && echo $?", repo),
		// git reads its config, and puts files back.
		bashCall("git config user.email", repo),
		bashCall("git config --get core.hooksPath", repo),
		bashCall("git -C "+repo+" config --list --show-origin", repo),
		bashCall("git restore bill.py && git status", repo),
		bashCall("cat .git/HEAD .claude/settings.json", repo),
		write(filepath.Join(repo, "docs", "claude.md")),
		tool("NotebookEdit", map[string]any{"notebook_path": filepath.Join(repo, "a.ipynb")}),
	}
	refused := []demoCall{
		bashCall("smolcorp gm hire --role tester --folder ~", repo),
		bashCall("smolcorp gm hire --role tester --folder "+elsewhere, repo),
		bashCall("smolcorp gm hire --role tester --folder ../..", repo),
		bashCall("smolcorp gm hire --role tester", repo),
		bashCall("smolcorp gm hire --role codexer --folder "+repo, repo),
		bashCall("smolcorp gm hire --role tester --folder "+repo+" --t3", repo),
		bashCall("cd "+repo+" && smolcorp gm hire --role tester --folder /", repo),
		bashCall("sh -c 'smolcorp gm hire --role tester --folder ~'", repo),
		bashCall(`smolcorp gm hire --role tester --folder `+strings.ReplaceAll(elsewhere, "/", `\`), repo),
		bashCall(`curl -X POST localhost:7710/setup/gm/hire -d '{"role":"tester"}'`, repo),
		bashCall("git push origin hire/tester-1", repo),
		bashCall("gh pr create --fill", repo),
		write(filepath.Join(elsewhere, "x.txt")),
		write("../../outside.txt"),
		tool("Edit", map[string]any{"file_path": filepath.Join(homeDir, ".zshrc")}),
		tool("Edit", map[string]any{}),
		tool("mcp__smolcorp__hire", map[string]any{"role": "tester", "folder": elsewhere}),
		tool("Bash", map[string]any{"command": 5}),
		// RX1's B1: folders the shell makes something else before smolcorp sees them.
		bashCall(`smolcorp gm hire --role tester --folder "$HOME/Projects" --worktree`, repo),
		bashCall(`smolcorp gm hire --role tester --folder ${HOME} --worktree`, repo),
		bashCall(`smolcorp gm hire --role tester --folder ~someone/Projects --worktree`, repo),
		bashCall(`smolcorp gm hire --role tester --folder $PWD/../.. --worktree`, repo),
		bashCall(`smolcorp gm hire --role tester --folder "$(pwd)/../.." --worktree`, repo),
		bashCall(`smolcorp gm hire --role tester --folder {/Users,/tmp}`, repo),
		bashCall("smolcorp gm hire --role tester --folder `pwd`/../..", repo),
		bashCall("smolcorp gm hire --role tester --folder ./*", repo),
		bashCall("smolcorp gm hire --role tester --folder app/~x", repo),
		// B2: the API-key account, any spelling, and another Claude account.
		bashCall("smolcorp gm hire --role tester --folder "+repo+" --api-key", repo),
		bashCall("smolcorp gm hire -api-key --role tester --folder "+repo, repo),
		bashCall("smolcorp gm hire --role tester --folder "+repo+" --api-key=false", repo),
		bashCall("smolcorp gm hire --role tester --folder "+repo+" --account work", repo),
		bashCall("smolcorp gm hire --role tester --account=work --folder "+repo, repo),
		tool("mcp__smolcorp__hire", map[string]any{"role": "tester", "folder": repo, "api-key": true}),
		tool("mcp__smolcorp__hire", map[string]any{"role": "tester", "folder": repo, "api-key": 1}),
		tool("mcp__smolcorp__hire", map[string]any{"role": "tester", "folder": repo, "account": "work"}),
		tool("mcp__smolcorp__hire", map[string]any{"role": "tester", "folder": repo, "account": 5}),
		// A flag that takes a value takes the next word: --folder is the nickname, the hire goes to elsewhere.
		bashCall("smolcorp gm hire --role tester --nickname --folder --folder "+elsewhere, repo),
		// S2: gm built by the shell, and the hire routes spelled another way.
		bashCall("smolcorp $'gm' hire --role tester --folder ~", repo),
		bashCall("G=gm; smolcorp $G hire --role tester --folder ~", repo),
		bashCall("smolcorp `echo gm` hire --role tester --folder ~", repo),
		bashCall("S=smolcorp; $S gm hire --role tester --folder "+repo, repo),
		bashCall(`curl -X POST -H "X: $SMOLCORP_GM_TOKEN" localhost:7710/setup/gm/%68ire -d '{}'`, repo),
		bashCall(`curl -X POST localhost:7710/setup/gm/%68ire -d '{}'`, repo),
		bashCall(`curl -X POST localhost:7710/setup//hire -d '{}'`, repo),
		bashCall(`curl -X POST localhost:7710/setup/./gm/x/../hire -d '{}'`, repo),
		bashCall(`curl -X POST localhost:7710/setup/%2568ire -d '{}'`, repo),
		// S3: the fence and the hooks themselves, in the project and in a hire's worktree.
		write(".claude/settings.json"),
		write(filepath.Join(repo, ".claude", "settings.local.json")),
		write(filepath.Join(repo, ".Claude", "settings.json")),
		write(filepath.Join(repo, ".git", "hooks", "pre-push")),
		write(filepath.Join(root, "app-hires", "tester-1", ".claude", "settings.json")),
		write(filepath.Join(root, "app-hires", "tester-1", ".git")),
		tool("MultiEdit", map[string]any{"file_path": filepath.Join(repo, ".git", "config")}),
		tool("NotebookEdit", map[string]any{"file_path": filepath.Join(repo, "a.ipynb"), "notebook_path": filepath.Join(elsewhere, "n.ipynb")}),
		tool("NotebookEdit", map[string]any{"notebook_path": filepath.Join(repo, ".claude", "n.ipynb")}),
		bashCall("git config core.hooksPath /dev/null", repo),
		bashCall("git config remote.o.url https://example.invalid/x.git", repo),
		bashCall("git -C "+repo+" config --unset core.hooksPath", repo),
		bashCall("git config set core.hooksPath /dev/null", repo),
		bashCall("git checkout -b mine", repo),
		bashCall("cd "+repo+" && /usr/bin/git switch other", repo),
	}
	if runtime.GOOS == "windows" { // elsewhere, C:\ is a folder name like any other
		refused = append(refused, bashCall(`smolcorp gm hire --role tester --folder C:\Users\someone\elsewhere`, repo))
	}
	if linked {
		refused = append(refused, bashCall("smolcorp gm hire --role tester --folder "+filepath.Join(root, "link"), repo),
			write(filepath.Join(root, "link", "x.txt")),
			// The system follows the link before the ..: this is in, beside the demo folder, not in it.
			write(strings.Join([]string{root, "link", "..", "x.txt"}, string(filepath.Separator))))
	}
	for _, c := range allowed {
		if why := f.check(c); why != "" {
			t.Errorf("refused %s %s: %s", c.ToolName, c.ToolInput, why)
		}
	}
	for _, c := range refused {
		if why := f.check(c); !strings.HasPrefix(why, demoFenceRefusal) {
			t.Errorf("let through %s %s", c.ToolName, c.ToolInput)
		}
	}
	// As the hook: a refusal is Claude Code's deny; an allowed call prints nothing; what it can't read is refused.
	args := []string{"--root", root, "--roles", "tester,reviewer"}
	for _, c := range []struct {
		args  []string
		input string
		deny  bool
	}{
		{args, `{"tool_name":"Bash","tool_input":{"command":"smolcorp gm hire --role tester --folder ~"},"cwd":"` + filepath.ToSlash(repo) + `"}`, true},
		{args, `{"tool_name":"Bash","tool_input":{"command":"ls"},"cwd":"` + filepath.ToSlash(repo) + `"}`, false},
		{args, `not json`, true},
		{[]string{"--root"}, `{"tool_name":"Bash","tool_input":{"command":"ls"}}`, true},
		{[]string{"--root", "relative", "--roles", "tester"}, `{"tool_name":"Bash","tool_input":{"command":"ls"}}`, true},
	} {
		var out bytes.Buffer
		if code := runDemoFence(c.args, strings.NewReader(c.input), &out); code != 0 {
			t.Errorf("%s: exit %d", c.input, code)
		}
		if denied := strings.Contains(out.String(), `"permissionDecision":"deny"`); denied != c.deny || !c.deny && out.Len() > 0 {
			t.Errorf("%v %s: %q", c.args, c.input, out.String())
		}
	}
}

// standInOffice is Smolcorp's setup API as the demo uses it, for the real flow's tests.
type standInOffice struct {
	mu        sync.Mutex
	state     string
	calls     []string
	hire      map[string]any
	teamCalls int
	finish    func() // the stand-in GM's last step, at the second look at the team
	failHire  bool   // the GM's hire fails
	gone      bool   // Smolcorp stops answering the team
}

func (s *standInOffice) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.URL.Path != "/pair" && r.Header.Get(setupKeyHeader) != "fake-setup-key" {
		http.Error(w, "no key", http.StatusForbidden)
		return
	}
	s.calls = append(s.calls, r.Method+" "+r.URL.RequestURI())
	switch r.Method + " " + r.URL.Path {
	case "GET /pair":
	case "GET /setup/state":
		io.WriteString(w, s.state)
	case "POST /setup/hire":
		if s.failHire {
			http.Error(w, "No desk is free.", http.StatusConflict)
			return
		}
		json.NewDecoder(r.Body).Decode(&s.hire)
		io.WriteString(w, `{"ok":true,"desk":2,"session":null}`)
	case "GET /setup/gm/team":
		if s.teamCalls++; s.teamCalls == 2 && s.finish != nil {
			s.finish()
		}
		if s.gone {
			http.Error(w, "gone", http.StatusServiceUnavailable)
			return
		}
		folder, _ := s.hire["folder"].(string)
		json.NewEncoder(w).Encode(map[string]any{"team": []map[string]any{
			{"id": "g1", "name": "General Manager", "role": "gm", "folder": folder, "state": "working"},
			{"id": "t1", "name": "Tester", "nickname": "Ada", "role": "tester", "folder": filepath.Join(filepath.Dir(folder), "app-hires", "tester-ab12"), "branch": "hire/tester-ab12", "state": "working"},
			{"id": "x1", "name": "Someone else's", "role": "docs", "folder": "/elsewhere", "state": "idle"},
		}})
	case "POST /setup/gm/manager", "POST /setup/hiring":
	default:
		http.NotFound(w, r)
	}
}

const demoTestState = `{"gmHired":%s,"manager":{"enabled":false},"hiring":{"enabled":true,"canHire":true},
"roles":{"roles":[{"id":"gm","agent":"claude"},{"id":"reviewer","agent":"claude"},{"id":"tester","agent":""},{"id":"porter","agent":"codex"}]}}`

func realDemo(t *testing.T, office *standInOffice, answer string) (int, string, string) {
	t.Helper()
	return realDemoIn(t, context.Background(), office, answer)
}

func realDemoIn(t *testing.T, ctx context.Context, office *standInOffice, answer string) (int, string, string) {
	t.Helper()
	in := demoTestFolder(t)
	server := httptest.NewServer(office)
	t.Cleanup(server.Close)
	team := &officeTeam{base: server.URL, key: func() ([]byte, error) { return []byte("fake-setup-key\n"), nil }, http: server.Client(),
		start: func() error { t.Error("started the app while it was running"); return nil }, every: 5 * time.Millisecond}
	var out bytes.Buffer
	code := runDemo(ctx, demoRun{in: in, team: team, stdin: strings.NewReader(answer), out: &out})
	return code, out.String(), in
}

func TestTheRealDemoAsksBeforeItSpendsAndHiresTheGMInsideItsFolder(t *testing.T) {
	// No OK: nothing made, nobody hired.
	office := &standInOffice{state: strings.Replace(demoTestState, "%s", "false", 1)}
	code, out, in := realDemo(t, office, "n\n")
	if entries, _ := os.ReadDir(in); code != 1 || len(entries) != 0 || slices.Contains(office.calls, "POST /setup/hire") || !strings.Contains(out, "your default Claude account") {
		t.Errorf("without an OK: %d %v %v\n%s", code, entries, office.calls, out)
	}

	// A GM already hired: refused before anything is asked or made.
	office = &standInOffice{state: strings.Replace(demoTestState, "%s", "true", 1)}
	if code, out, in := realDemo(t, office, "y\n"); code != 1 || !strings.Contains(out, "already hired") || strings.Contains(out, "Go on?") {
		entries, _ := os.ReadDir(in)
		t.Errorf("with a GM working: %d %v\n%s", code, entries, out)
	}

	// With an OK: the manager on, the GM hired into the project with the demo's goal, followed until its report.
	office = &standInOffice{state: strings.Replace(demoTestState, "%s", "false", 1)}
	office.finish = func() {
		folder, _ := office.hire["folder"].(string)
		os.WriteFile(filepath.Join(filepath.Dir(folder), demoReportName), []byte("# Report\n\nAll three reviewed."), 0o644)
	}
	code, out, in = realDemo(t, office, "y\n")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	root := filepath.Join(in, demoFolderName)
	repo := filepath.Join(root, demoRepoName)
	if !slices.Contains(office.calls, "POST /setup/gm/manager?enabled=1") || slices.Contains(office.calls, "POST /setup/hiring?enabled=1") {
		t.Errorf("the switches: %v", office.calls)
	}
	task, _ := office.hire["task"].(string)
	answers, _ := office.hire["answers"].([]any)
	if office.hire["role"] != "gm" || office.hire["folder"] != repo || len(answers) != 3 || !strings.Contains(task, "--folder "+repo) ||
		!strings.Contains(task, "Stop before anything ships") || !strings.Contains(task, "reviewer, tester.") {
		t.Errorf("the GM's hire: %v", office.hire)
	}
	for _, want := range []string{"Hired the General Manager at desk 3", "General Manager (gm): working", "Ada (tester): working, on hire/tester-ab12",
		"All three reviewed.", "Nothing shipped"} {
		if !strings.Contains(out, want) {
			t.Errorf("doesn't say %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Someone else's") {
		t.Errorf("shows a hire outside the demo:\n%s", out)
	}
	// Only the roles on Claude Code may be hired, through the fence of every session in the project.
	settings, _ := os.ReadFile(filepath.Join(repo, ".claude", "settings.json"))
	if !strings.Contains(string(settings), "reviewer,tester") || strings.Contains(string(settings), "porter") {
		t.Errorf("the settings: %s", settings)
	}
}

func TestTheDemoSaysWhichSwitchesItTurnsOnAndPutsThemBack(t *testing.T) {
	bothOff := strings.Replace(strings.Replace(demoTestState, "%s", "false", 1), `"hiring":{"enabled":true`, `"hiring":{"enabled":false`, 1)
	managerOff := strings.Replace(demoTestState, "%s", "false", 1)
	leftOn := "turn it off in Setup, Hires and roles: Let a General Manager run your team."

	// The question names the switches that are off, and that they stay on; a no changes nothing.
	office := &standInOffice{state: bothOff}
	code, out, _ := realDemo(t, office, "n\n")
	if code != 1 || !strings.Contains(out, "Smolcorp's manager is off and hiring is off: the demo turns them on, and they stay on until you turn them off in Setup.") ||
		slices.ContainsFunc(office.calls, func(c string) bool { return strings.HasPrefix(c, "POST") }) {
		t.Errorf("both off, no: %d %v\n%s", code, office.calls, out)
	}
	if _, out, _ := realDemo(t, &standInOffice{state: managerOff}, "n\n"); !strings.Contains(out, "Smolcorp's manager is off: the demo turns it on, and it stays on until you turn it off in Setup.") {
		t.Errorf("the manager off:\n%s", out)
	}

	// The GM's hire fails: what the demo turned on goes back off, last first, and it says so.
	office = &standInOffice{state: bothOff, failHire: true}
	code, out, _ = realDemo(t, office, "y\n")
	want := []string{"POST /setup/gm/manager?enabled=1", "POST /setup/hiring?enabled=1", "POST /setup/hire", "POST /setup/hiring?enabled=0", "POST /setup/gm/manager?enabled=0"}
	var posts []string
	for _, c := range office.calls {
		if strings.HasPrefix(c, "POST") {
			posts = append(posts, c)
		}
	}
	if code != 1 || !slices.Equal(posts, want) || !strings.Contains(out, "No desk is free.") || !strings.Contains(out, "Turned hiring back off") ||
		!strings.Contains(out, "Turned Smolcorp's manager back off") || strings.Contains(out, "left") {
		t.Errorf("a failed hire: %d %v\n%s", code, posts, out)
	}

	// The report: the last word says what's still on, and where to turn it off.
	office = &standInOffice{state: managerOff}
	office.finish = func() {
		folder, _ := office.hire["folder"].(string)
		os.WriteFile(filepath.Join(filepath.Dir(folder), demoReportName), []byte("# Report"), 0o644)
	}
	if code, out, _ := realDemo(t, office, "y\n"); code != 0 || !strings.HasSuffix(strings.TrimSpace(out), leftOn) ||
		!strings.Contains(out, "The demo turned on Smolcorp's manager, and left it on") {
		t.Errorf("after the report: %d\n%s", code, out)
	}

	// Ctrl-C while following: the same last word.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	office = &standInOffice{state: managerOff}
	office.finish = cancel
	if code, out, _ := realDemoIn(t, ctx, office, "y\n"); code != 0 || !strings.Contains(out, "Stopped following") || !strings.HasSuffix(strings.TrimSpace(out), leftOn) {
		t.Errorf("after Ctrl-C: %d\n%s", code, out)
	}

	// Smolcorp gone: the same.
	office = &standInOffice{state: managerOff}
	office.finish = func() { office.gone = true }
	if code, out, _ := realDemo(t, office, "y\n"); code != 1 || !strings.Contains(out, "Smolcorp stopped answering") || !strings.HasSuffix(strings.TrimSpace(out), leftOn) {
		t.Errorf("with Smolcorp gone: %d\n%s", code, out)
	}

	// Ctrl-C is the demo's to catch only once the question is answered.
	caught := false
	interrupt := func(ctx context.Context) (context.Context, context.CancelFunc) {
		caught = true
		return context.WithCancel(ctx)
	}
	team := &officeTeam{state: demoState{}}
	team.state.Hiring.Enabled = true
	runDemo(context.Background(), demoRun{in: demoTestFolder(t), team: readyTeam{team}, stdin: strings.NewReader("n\n"), out: io.Discard, interrupt: interrupt})
	if caught {
		t.Error("Ctrl-C was caught before the answer")
	}
}

// readyTeam is an office team whose ready reads nothing.
type readyTeam struct{ *officeTeam }

func (readyTeam) ready(io.Writer) ([]string, error) { return []string{"reviewer"}, nil }

func TestTheSampleHasNoPrivateNames(t *testing.T) {
	err := fs.WalkDir(demoSample, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, _ := demoSample.ReadFile(path)
		for _, name := range []string{"Sauzze", "coachpop", "David", "AgentDeck", "/Users/"} {
			if strings.Contains(string(data), name) {
				t.Errorf("%s names %q", path, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if license, _ := demoSample.ReadFile("demosample/LICENSE"); !strings.HasPrefix(string(license), "MIT License") {
		t.Error("the sample isn't MIT")
	}
}
