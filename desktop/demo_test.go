package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// demoCheckout makes root/app a git project with one commit, and a worktree of it at root/app-hires/<name>
// for each name, as the office makes a hire's. It returns the project.
func demoCheckout(t *testing.T, root string, names ...string) string {
	t.Helper()
	repo := filepath.Join(root, demoRepoName)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitOut(t, repo, "init", "-q", "-b", "main")
	stopSigning(t, repo)
	gitOut(t, repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "one")
	for _, name := range names {
		gitOut(t, repo, "worktree", "add", "-q", "-b", "hire/"+name, filepath.Join(root, "app-hires", name))
	}
	return repo
}

func TestTheDemosFenceRefusesOutsideTheDemoFolder(t *testing.T) {
	in := demoTestFolder(t)
	root := filepath.Join(in, "smolcorp-demo")
	repo := demoCheckout(t, root, "tester-1")
	worktree := filepath.Join(root, "app-hires", "tester-1")
	// RX1c B1: folders in the demo that hold no fence: the demo folder itself, the worktrees' folder, a new
	// folder, a folder in the project, and another project's checkout.
	other := filepath.Join(root, "other")
	for _, dir := range []string{filepath.Join(root, "x"), filepath.Join(repo, "sub"), other} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitOut(t, other, "init", "-q")
	elsewhere := filepath.Join(in, "elsewhere")
	os.MkdirAll(elsewhere, 0o755)
	// A link inside the demo that leads out of it (where the system lets a test make one).
	linked := os.Symlink(elsewhere, filepath.Join(root, "link")) == nil
	f := demoFence{root: root, roles: []string{"tester", "reviewer"}, home: homeDir,
		team: func() ([]demoMember, error) {
			return []demoMember{{ID: "tester-1", Folder: repo, Registered: true}}, nil
		}}
	write := func(path string) demoCall {
		input, _ := json.Marshal(map[string]string{"file_path": path})
		return demoCall{ToolName: "Write", ToolInput: input, Cwd: repo}
	}
	tool := func(name string, input map[string]any) demoCall {
		data, _ := json.Marshal(input)
		return demoCall{ToolName: name, ToolInput: data, Cwd: repo}
	}
	// Claude Code's Bash is bash, Git Bash on Windows, where a \ escapes: a Windows path goes in with /, as the
	// demo writes it for the GM. PowerShell is refused whatever it says (RWIN B3).
	bash := demoShellFolder
	powerShell := func(command string) demoCall { return tool("PowerShell", map[string]any{"command": command}) }
	allowed := []demoCall{
		bashCall("smolcorp gm hire --role tester --folder "+shellPath(repo)+" --worktree --task 'fix it'", repo),
		bashCall("smolcorp gm hire --role=reviewer --folder="+bash(repo), repo),
		bashCall("smolcorp gm hire --role tester --folder .", repo),
		bashCall("smolcorp gm fire tester-1", repo),
		bashCall("python3 -m unittest && git diff main...hire/tester-1", repo),
		write(filepath.Join(repo, "new-file.py")),
		write("notes/plan.md"),
		tool("Read", map[string]any{"file_path": "/etc/hosts"}),
		tool("mcp__smolcorp__hire", map[string]any{"role": "tester", "folder": repo}),
		// On the default account, said or not; a $ in single quotes is text, not the shell's.
		bashCall("smolcorp gm hire --role tester --folder "+bash(repo)+" --account default", repo),
		bashCall("smolcorp gm hire --role tester --folder "+bash(repo)+" --task 'parse_amount reads $12.50'", repo),
		tool("mcp__smolcorp__hire", map[string]any{"role": "tester", "folder": repo, "account": "default", "api-key": false}),
		// A $ in a line that doesn't name smolcorp, though its paths do (smolcorp-demo).
		bashCall("ls "+bash(root)+" && echo $?", repo),
		// git reads its config, and puts files back.
		bashCall("git config user.email", repo),
		bashCall("git config --get core.hooksPath", repo),
		bashCall("git -C "+bash(repo)+" config --list --show-origin", repo),
		bashCall("git restore bill.py && git status", repo),
		bashCall("cat .git/HEAD .claude/settings.json", repo),
		write(filepath.Join(repo, "docs", "claude.md")),
		tool("NotebookEdit", map[string]any{"notebook_path": filepath.Join(repo, "a.ipynb")}),
		// RX1b: smolcorp's own commands the demo uses.
		bashCall("smolcorp version", repo),
		bashCall("smolcorp --version && smolcorp help", repo),
		bashCall("smolcorp gm team", repo),
		// RX1b S3: a hire over several lines, as models write them.
		bashCall("smolcorp gm hire --role tester \\\n  --folder "+bash(repo)+" \\\n  --worktree \\\n  --task 'Fix the rounding'", repo),
		// FXX1c: the top of a worktree of the project, as the office makes them; paths in the demo name it.
		bashCall("smolcorp gm hire --role reviewer --folder "+bash(worktree), repo),
		tool("mcp__smolcorp__hire", map[string]any{"role": "reviewer", "folder": worktree}),
		bashCall("cd "+bash(worktree)+" && python3 -m unittest", repo),
		bashCall("git -C "+bash(repo)+" worktree list && git worktree add ../app-hires/extra -b extra", repo),
		bashCall("smolcorp gm hire --role tester --folder="+bash(repo)+" --worktree", repo),
		// RWIN B5: text in quotes is text: braces, globs and an escaped quote in it.
		bashCall("smolcorp gm hire --role tester --folder "+bash(repo)+` --task "Fix {all} the *bugs*? say \"done\""`, repo),
		bashCall("smolcorp gm send tester-1 --text 'h?re [x] {a,b}'", repo),
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
		// RX1b B1: the MCP server on standard input hires anywhere.
		bashCall(`printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"hire","arguments":{"role":"tester","folder":"`+elsewhere+`"}}}' > m.txt; smolcorp mcp < m.txt`, repo),
		bashCall("smolcorp mcp", repo),
		// RX1b B2: smolcorp run starts an agent where the demo's fence isn't.
		bashCall("smolcorp run claude -p 'fix everything' --dangerously-skip-permissions", repo),
		bashCall("cd "+elsewhere+" && smolcorp run --role tester", repo),
		bashCall("smolcorp run codex", repo),
		// Every other command of smolcorp's, by any path or spelling, and smolcorp handed to another program.
		bashCall("smolcorp setup --check", repo),
		bashCall("smolcorp serve", repo),
		bashCall("smolcorp core", repo),
		bashCall("smolcorp hook claude", repo),
		bashCall("smolcorp quit", repo),
		bashCall("smolcorp guard --profile gm", repo),
		bashCall("smolcorp demo --dry-run", repo),
		bashCall("smolcorp frobnicate", repo),
		bashCall("smolcorp", repo),
		bashCall("/usr/local/bin/smolcorp-cli mcp < calls.jsonl", repo),
		bashCall("SMOLCORP.exe setup", repo),
		bashCall("sm''olcorp mcp", repo),
		bashCall("nohup smolcorp serve &", repo),
		bashCall("xargs smolcorp mcp < calls.jsonl", repo),
		// RX1b S2: a backslash before a line end joins the lines: this is gm hire.
		bashCall("smolcorp gm hi\\\nre --role tester --folder "+elsewhere, repo),
		// RX1b S5: the owner's setup key.
		bashCall("cat '"+filepath.Join(homeDir, "Library", "Application Support", "AgentDeck", "setup-key")+"'", repo),
		bashCall(`curl -H 'X-Smolcorp-Setup-Key: k' 'http://127.0.0.1:7710/setup/hiring?enabled=0'`, repo),
		tool("Read", map[string]any{"file_path": filepath.Join(homeDir, ".config", "Smolcorp", "Setup-Key")}),
		{ToolName: "Grep", ToolInput: json.RawMessage(`{"pattern":"x","path":"/somewhere/setup\u002dKey"}`), Cwd: repo},
		// RX1c B1: a hire into the demo folder but not the project or a worktree of it: no fence there.
		bashCall("smolcorp gm hire --role reviewer --folder "+root+" --task 'Write DEMO-REPORT.md'", repo),
		bashCall("smolcorp gm hire --role reviewer --folder "+filepath.Join(root, "app-hires"), repo),
		bashCall("mkdir -p "+filepath.Join(root, "x")+" && smolcorp gm hire --role tester --folder "+filepath.Join(root, "x"), repo),
		bashCall("smolcorp gm hire --role tester --folder "+filepath.Join(repo, "sub"), repo),
		bashCall("smolcorp gm hire --role tester --folder "+other, repo),
		bashCall("smolcorp gm hire --role tester --folder "+filepath.Join(root, "app-hires", "not-yet"), repo),
		tool("mcp__smolcorp__hire", map[string]any{"role": "tester", "folder": root}),
		// RX1c S2: a link made in the same line isn't there when the fence looks.
		bashCall("ln -s "+elsewhere+" "+filepath.Join(root, "p")+"; smolcorp gm hire --role tester --folder "+filepath.Join(root, "p"), repo),
		// RX1c S1: smolcorp as a part of a word, and a command whose name the shell builds.
		bashCall("X=smolcorp; $X mcp < m.txt", repo),
		bashCall("X=smolcorp\n\"$X\" mcp < m.txt", repo),
		bashCall("alias h='smolcorp mcp'\neval h < m.txt", repo),
		bashCall("alias h=smolcorp\nh mcp < m.txt", repo),
		bashCall("X=smol; Y=corp; $X$Y mcp < m.txt", repo),
		// RX1c S5: a worktree outside the demo folder.
		bashCall("git worktree add ../../../outside-wt", repo),
		bashCall("git -C "+repo+" worktree add ~/Projects/wt", repo),
		bashCall("git -C "+elsewhere+" worktree add wt", repo),
		bashCall("git worktree add -b x "+elsewhere+"/wt main", repo),
	}
	refused = append(refused,
		// RWIN B3: PowerShell builds words the fence can't read, so the demo refuses it, whatever it says.
		powerShell("smolcorp gm hire --role tester --folder "+elsewhere),
		powerShell("smolcorp gm hire --role tester --folder ~someone/Projects"),
		powerShell("smolcorp mcp"),
		powerShell("$S='smolcorp'; & $S mcp"),
		powerShell("smolcorp gm hire --role tester --folder "+repo+" --worktree --task 'fix it'"),
		powerShell("Get-ChildItem "+root),
		powerShell("Set-Location C:/Users; & ('cla'+'ude') -p 'x' --dangerously-skip-permissions"),
		powerShell("& ('smol'+'corp') run claude"),
		powerShell("smolcorp gm ‘hire’ --role tester --worktree --folder "+elsewhere),
		// RWIN B4: cmd's %VAR% and !VAR! in a folder.
		bashCall(`cmd //c "smolcorp gm hire --role tester --worktree --folder %USERPROFILE%"`, repo),
		bashCall("git worktree add %USERPROFILE%", repo),
		bashCall("smolcorp gm hire --role tester --worktree --folder !HOME!", repo),
		// RWIN B1: zsh's extended globs (x#, ^x, a~b) make a folder something else.
		bashCall("smolcorp gm hire --role tester --worktree --folder "+bash(in)+"/outsidee#~/../smolcorp-demo/app", repo),
		bashCall("smolcorp gm hire --role tester --worktree --folder "+bash(in)+"/^demo~/../smolcorp-demo/app", repo),
		bashCall("git worktree add "+bash(in)+"/outsidee#~/../smolcorp-demo/wt2", repo),
		// RWIN B5: an escaped quote inside double quotes, braces and globs: the shell's words, not the fence's.
		bashCall("smolcorp gm hire --role tester --worktree --folder "+elsewhere+` --task "\" --folder `+bash(repo)+`"`, repo),
		bashCall("smolcorp gm {hire,} --role tester --worktree --folder "+elsewhere, repo),
		bashCall("smolcorp gm {send,} someone-else --text hi", repo),
		bashCall("smolcorp gm h?re --role tester --worktree --folder "+elsewhere, repo),
		bashCall("smolcorp gm hi[r]e --role tester --worktree --folder "+elsewhere, repo),
		bashCall("smolcorp gm hi*e --role tester --worktree --folder "+elsewhere, repo),
		// bash reads notes\smolcorp-plan.md as notessmolcorp-plan.md: a word that holds smolcorp and isn't a path.
		bashCall(`cat notes\smolcorp-plan.md`, repo))
	if runtime.GOOS == "windows" { // elsewhere, C:\ is a folder name like any other
		refused = append(refused, bashCall(`smolcorp gm hire --role tester --folder C:\Users\someone\elsewhere`, repo),
			powerShell(`smolcorp gm hire --role tester --folder C:\Users\someone\elsewhere`),
			// A hire's folder as bash would get it: C:Users… with the \ taken away.
			bashCall("smolcorp gm hire --role tester --folder "+repo, repo))
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

// WIN125, RWIN B1: on Windows a ~ is a letter only in a short name's shape (RUNNER~1); elsewhere only ~ and
// ~/… at the start are plain (zsh reads a~b as a glob). cmd's % and !, and zsh's # and ^, never are.
func TestTheDemosFenceReadsAShortNamesTildeAsALetter(t *testing.T) {
	for _, c := range []struct {
		folder           string
		windows, unixish bool
	}{
		{"C:/Users/RUNNER~1/AppData/Local/Temp/SMOLCO~4/app", true, false},
		{`C:\PROGRA~1\smolcorp-demo\app`, true, false},
		{`C:\Users\ada\NOTES~12.TXT`, true, false},
		{"/Users/ada/smolcorp-demo/app", true, true},
		{"~/smolcorp-demo/app", true, true},
		{`~\smolcorp-demo\app`, true, true},
		{"~", true, true},
		{"~someone/Projects", false, false},
		{"app/~x", false, false},
		{`app\~x`, false, false},
		{"app/x~", false, false},
		{"app/x~y", false, false},
		{"app/x~1y", false, false},
		{"a=~x", false, false},
		{"a:~x", false, false},
		{"~/a/~b", false, false},
		{"outside~1/../smolcorp-demo/app", true, false}, // a short name to Git Bash; zsh isn't the shell there
		{"C:/Users/RUNNER~1/$HOME", false, false},
		{"/Users/ada/outsidee#~/../smolcorp-demo/app", false, false},
		{"/Users/ada/^demo/app", false, false},
		{"%USERPROFILE%", false, false},
		{"/Users/ada/!HOME!", false, false},
	} {
		if got := plainFolderOn(true, c.folder); got != c.windows {
			t.Errorf("on Windows, plainFolder(%q) = %v, want %v", c.folder, got, c.windows)
		}
		if got := plainFolderOn(false, c.folder); got != c.unixish {
			t.Errorf("on a Mac or Linux, plainFolder(%q) = %v, want %v", c.folder, got, c.unixish)
		}
	}
}

// WIN125: the folder the demo tells the GM to hire into is written as bash reads it on each system.
// WIN125b: the fence reads another system's calls by that system's path rules (goos), so a Windows call is
// judged on this Mac (or Linux) as on Windows, and the other way round.
func TestTheDemoWritesAWindowsFolderWithSlashes(t *testing.T) {
	for _, c := range []struct{ goos, path, want string }{
		{"windows", `C:\Users\ada\smolcorp-demo\app`, "C:/Users/ada/smolcorp-demo/app"},
		{"windows", `C:\Users\Ada Lovelace\smolcorp-demo\app`, "'C:/Users/Ada Lovelace/smolcorp-demo/app'"},
		{"windows", `C:\Users\a,b\smolcorp-demo\app`, "'C:/Users/a,b/smolcorp-demo/app'"},
		{"windows", `C:\Users\O'Brien Ada\smolcorp-demo\app`, `"C:/Users/O'Brien Ada/smolcorp-demo/app"`}, // RWIN N2
		{"darwin", "/Users/ada/smolcorp-demo/app", "/Users/ada/smolcorp-demo/app"},
		{"linux", "/home/ada/smolcorp-demo/app", "/home/ada/smolcorp-demo/app"},
	} {
		if got := shellFolderOn(c.goos, c.path); got != c.want {
			t.Errorf("on %s, %s is written %s, want %s", c.goos, c.path, got, c.want)
		}
	}
	bash := func(command string) demoCall { return bashCall(command, "") }
	tool := func(name, key, value string) demoCall {
		input, _ := json.Marshal(map[string]string{key: value})
		return demoCall{ToolName: name, ToolInput: input}
	}
	write := func(path string) demoCall { return tool("Write", "file_path", path) }
	for _, system := range []struct {
		goos, root, home string
		allowed, refused []demoCall
	}{
		{"windows", `C:\Users\ada\smolcorp-demo`, `C:\Users\ada`,
			[]demoCall{
				// A relative path with either separator is a path: in quotes bash keeps the \.
				bash(`cat 'notes\smolcorp-plan.md'`),
				bash(`cat "notes\smolcorp-plan.md"`),
				bash("cat notes/smolcorp-plan.md"),
				bash(`cat 'C:\Users\ada\smolcorp-demo\app\notes.md'`),
				bash("cat C:/Users/ada/smolcorp-demo/app/notes.md ../smolcorp-demo/PLAN.md"),
				bash("cat c:/users/ADA/Smolcorp-Demo/app/notes.md"), // Windows' disk: case aside
				bash("ls ~/smolcorp-demo/app"),
				write(`notes\smolcorp-plan.md`),
				write("C:/Users/ada/smolcorp-demo/app/bill.py"),
				write(`..\app-hires\tester-1\bill.py`),
			},
			[]demoCall{
				// bash takes the \ away: notessmolcorp-plan.md, a word, not a path.
				bash(`cat notes\smolcorp-plan.md`),
				bash(`cat C:\Users\ada\smolcorp-demo\app\notes.md`),
				bash("cat C:/Users/ada/smolcorp-x/notes.md"),
				bash("cat C:/Users/ada/smolcorp-demo/../smolcorp-x/notes.md"),
				tool("PowerShell", "command", `Get-Content notes\smolcorp-plan.md`),
				// RWIN B2: a drive with no folder after it is from that drive's own current folder; a : after the
				// drive is an NTFS stream; \x is the top of some drive.
				write(`C:..\..\x.txt`),
				write(`C:.claude\settings.json`),
				write(`C:\Users\ada\smolcorp-demo\app\.git::$INDEX_ALLOCATION\hooks\pre-push`),
				write(`notes\a:b.txt`),
				write(`\Users\ada\smolcorp-demo\app\x.txt`),
				write(`D:\Users\ada\smolcorp-demo\app\x.txt`),
				write(`\\server\share\smolcorp-demo\app\x.txt`),
				bash(`cat 'C:smolcorp-demo\x'`),
				bash("git worktree add 'C:..\\wt'"),
				// Windows drops a name's dots and blanks at its end: .git. is .git.
				write(`.git.\hooks\pre-push`),
				write(`.claude \settings.json`),
				write(`C:\Users\ada\smolcorp-demo\app\.CLAUDE\settings.json`),
				// A hire needs its folder on the disk, and this Mac has no C:\: refused, never let through.
				bash("smolcorp gm hire --role tester --folder C:/Users/ada/smolcorp-demo/app --worktree"),
				bash("smolcorp gm hire --role tester --folder C:/Users/someone/elsewhere --worktree"),
			}},
		{"darwin", "/Users/ada/smolcorp-demo", "/Users/ada",
			[]demoCall{
				bash("cat notes/smolcorp-plan.md"),
				bash(`cat 'notes\smolcorp-plan.md'`), // a \ in a Mac file's name is a letter
				bash("cat /Users/ada/smolcorp-demo/app/notes.md"),
				write("notes/smolcorp-plan.md"),
				write(`C:\x.txt`), // a name like any other on a Mac
				write("notes/a:b.txt"),
			},
			[]demoCall{
				bash("cat /Users/ada/smolcorp-x/notes.md"),
				bash("cat /users/ada/smolcorp-demo/app/notes.md"), // a case variant: refused (fails closed)
				bash(`cat notes\smolcorp-plan.md`),
				tool("PowerShell", "command", "Get-Content notes/smolcorp-plan.md"),
				write("/Users/ada/smolcorp-demo/app/.git/hooks/pre-push"),
				write("../../elsewhere.txt"),
			}},
		{"linux", "/home/ada/smolcorp-demo", "/home/ada",
			[]demoCall{bash("cat notes/smolcorp-plan.md"), write("notes/smolcorp-plan.md")},
			[]demoCall{bash("cat /home/ada/smolcorp-x/notes.md"), write("/home/ada/elsewhere.txt")}},
	} {
		f := demoFence{root: system.root, roles: []string{"tester"}, home: system.home, goos: system.goos}
		cwd := f.paths().join(system.root, "app")
		for _, c := range system.allowed {
			if c.Cwd = cwd; f.check(c) != "" {
				t.Errorf("on %s, refused %s %s: %s", system.goos, c.ToolName, c.ToolInput, f.check(c))
			}
		}
		for _, c := range system.refused {
			if c.Cwd = cwd; !strings.HasPrefix(f.check(c), demoFenceRefusal) {
				t.Errorf("on %s, let through %s %s", system.goos, c.ToolName, c.ToolInput)
			}
		}
		// A refusal names the word as it was read, its \ once (not Go-quoted), and on Windows says how to write a path.
		why := f.command(`cat notes\smolcorp-plan.md`, cwd)
		if !strings.Contains(why, `"notessmolcorp-plan.md" holds smolcorp`) || strings.Contains(why, `\\`) {
			t.Errorf("on %s, the refusal: %q", system.goos, why)
		}
		hint := strings.Contains(why, "In bash, a \\ escapes the letter after it")
		if hint != (system.goos == "windows") { // RWIN N1: a Mac GM is never told to write a Windows path
			t.Errorf("on %s, the bash hint shown: %v: %q", system.goos, hint, why)
		}
		if why := f.command(`cat 'notes\smolcorp-x\plan.md' 'C:\smolcorp-x'`, cwd); system.goos == "windows" && !strings.Contains(why, `"C:\smolcorp-x" holds smolcorp`) {
			t.Errorf("on %s, the refusal doubles a \\: %q", system.goos, why)
		}
	}
}

// WIN125b (CI #49): git on Windows checks a worktree out with \r\n (core.autocrlf), and the stand-ins' edits
// still find their place. The same git setting here makes this computer's checkouts the same.
func TestTheDryRunWorksWhereGitWritesCRLF(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.autocrlf")
	t.Setenv("GIT_CONFIG_VALUE_0", "true")
	in := demoTestFolder(t)
	if code, out := dryRun(t, in); code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	tree := filepath.Join(in, demoFolderName, demoRepoName+"-hires", "bughunter-1")
	if data, err := os.ReadFile(filepath.Join(tree, "test_bill.py")); err != nil || !strings.Contains(string(data), "\r\n") {
		t.Errorf("the worktree wasn't checked out with \\r\\n, so this test proves nothing: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(tree, "bill.py")); !strings.Contains(string(data), "divmod(with_tip, people)\r\n") {
		t.Errorf("the bug hunter's change isn't in bill.py, in its line ends:\n%q", data)
	}
}

// standInOffice is Smolcorp's setup API as the demo uses it, for the real flow's tests. Its page is a stand-in
// too: opened (POST /setup/demo/open), it hires the GM as your click on Hire does, turning on what's off.
type standInOffice struct {
	mu        sync.Mutex
	state     string
	calls     []string
	hire      map[string]any
	teamCalls int
	finish    func()            // the stand-in GM's last step, at the second look at the team
	gone      bool              // Smolcorp stops answering the team
	others    []map[string]any  // the hires open before the demo's GM is hired
	noClick   bool              // the page is opened, and nobody clicks Hire
	page      string            // what the page says: "" (waiting), "open", "closed"
	failOpen  bool              // the browser doesn't open
	noDemo    bool              // a Smolcorp that doesn't serve the demo's page (404)
	onWait    func()            // runs at each look for the GM before it's hired
	told      map[string]string // the demo folders registered, with their roles, as the app keeps them
}

func (s *standInOffice) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.URL.Path != "/pair" && r.Header.Get(setupKeyHeader) != "fake-setup-key" {
		http.Error(w, "no key", http.StatusForbidden)
		return
	}
	s.calls = append(s.calls, r.Method+" "+r.URL.RequestURI())
	root := r.URL.Query().Get("root")
	if s.noDemo && strings.HasPrefix(r.URL.Path, "/setup/demo") {
		http.NotFound(w, r)
		return
	}
	switch r.Method + " " + r.URL.Path {
	case "GET /pair":
	case "GET /setup/state":
		io.WriteString(w, s.state)
	case "GET /setup/demo":
		if root == "" {
			open := []string{}
			for _, m := range s.others {
				if m["registered"] == true && m["state"] != "fired" && m["state"] != "failed" {
					name, _ := m["nickname"].(string)
					if name == "" {
						name, _ = m["name"].(string)
					}
					open = append(open, name)
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"gmHired": false, "open": open})
			return
		}
		if _, told := s.told[root]; !told {
			http.Error(w, root+" isn't a demo folder this Smolcorp was told about: run smolcorp demo for one.", http.StatusBadRequest)
			return
		}
		var gm any
		if s.hire != nil {
			gm = map[string]any{"id": "g1", "state": "starting"}
		} else if s.onWait != nil {
			s.onWait()
		}
		page := s.page
		if page == "" {
			page = "waiting"
		}
		json.NewEncoder(w).Encode(map[string]any{"demo": map[string]any{"page": page, "gm": gm}})
	case "POST /setup/demo/register":
		if s.told == nil {
			s.told = map[string]string{}
		}
		s.told[root] = r.URL.Query().Get("roles")
		w.WriteHeader(http.StatusNoContent)
	case "POST /setup/demo/open":
		if _, told := s.told[root]; !told {
			http.Error(w, "not told", http.StatusBadRequest)
			return
		}
		if s.failOpen {
			http.Error(w, "The browser didn't open (no xdg-open): run smolcorp demo --no-open for the demo's link.", http.StatusBadGateway)
			return
		}
		if !s.noClick {
			// Your click: the page turns on what's off, then hires the GM into the project, held to it.
			s.state = strings.Replace(strings.Replace(s.state, `"manager":{"enabled":false}`, `"manager":{"enabled":true}`, 1),
				`"hiring":{"enabled":false`, `"hiring":{"enabled":true`, 1)
			repo := filepath.Join(root, demoRepoName)
			s.hire = map[string]any{"role": gmRole, "folder": repo, "scope": repo}
		}
		w.WriteHeader(http.StatusNoContent)
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
	default:
		http.NotFound(w, r)
	}
}

// posts is the office's POSTs, in order.
func (s *standInOffice) posts() []string {
	var posts []string
	for _, c := range s.calls {
		if strings.HasPrefix(c, "POST") {
			posts = append(posts, c)
		}
	}
	return posts
}

const demoTestState = `{"gmHired":%s,"manager":{"enabled":false},"hiring":{"enabled":true,"canHire":true},
"roles":{"roles":[{"id":"gm","agent":"claude"},{"id":"reviewer","agent":"claude"},{"id":"tester","agent":""},{"id":"porter","agent":"codex"}]}}`

func realDemo(t *testing.T, office *standInOffice, answer string) (int, string, string) {
	t.Helper()
	return realDemoIn(t, context.Background(), office, answer)
}

func realDemoIn(t *testing.T, ctx context.Context, office *standInOffice, answer string) (int, string, string) {
	t.Helper()
	return realDemoWith(t, ctx, office, answer, nil)
}

// realDemoWith is the real flow against office, with the demo's team changed by change first (nil: as is).
func realDemoWith(t *testing.T, ctx context.Context, office *standInOffice, answer string, change func(*officeTeam)) (int, string, string) {
	t.Helper()
	in := demoTestFolder(t)
	server := httptest.NewServer(office)
	t.Cleanup(server.Close)
	team := &officeTeam{base: server.URL, key: func() ([]byte, error) { return []byte("fake-setup-key\n"), nil }, http: server.Client(),
		start: func() error { t.Error("started the app while it was running"); return nil }, every: 5 * time.Millisecond, wait: 5 * time.Second}
	if change != nil {
		change(team)
	}
	var out bytes.Buffer
	code := runDemo(ctx, demoRun{in: in, team: team, stdin: strings.NewReader(answer), out: &out})
	return code, out.String(), in
}

func TestTheRealDemoAsksBeforeItSpendsAndHasYouHireTheGMOnItsPage(t *testing.T) {
	// No OK: nothing made, nothing opened.
	office := &standInOffice{state: strings.Replace(demoTestState, "%s", "false", 1)}
	code, out, in := realDemo(t, office, "n\n")
	if entries, _ := os.ReadDir(in); code != 1 || len(entries) != 0 || len(office.posts()) != 0 || !strings.Contains(out, "your default Claude account") ||
		!strings.Contains(out, "Setup opens in your browser at\nthe demo: check it there, and click Hire") {
		t.Errorf("without an OK: %d %v %v\n%s", code, entries, office.calls, out)
	}

	// A GM already hired: refused before anything is asked or made.
	office = &standInOffice{state: strings.Replace(demoTestState, "%s", "true", 1)}
	if code, out, in := realDemo(t, office, "y\n"); code != 1 || !strings.Contains(out, "already hired") || strings.Contains(out, "Go on?") {
		entries, _ := os.ReadDir(in)
		t.Errorf("with a GM working: %d %v\n%s", code, entries, out)
	}

	// With an OK: the folder, then the page opened at it, and nothing else asked of the office but reads: the demo
	// never hires nor turns a switch on (its key can't). You click Hire; the demo follows the team until the report.
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
	// It tells the app which folder it made, with the roles its fence allows (the office's Claude roles), first.
	if want := []string{"POST /setup/demo/register?root=" + url.QueryEscape(root) + "&roles=" + url.QueryEscape("reviewer,tester"),
		"POST /setup/demo/open?root=" + url.QueryEscape(root)}; !slices.Equal(office.posts(), want) {
		t.Errorf("the demo's POSTs: %v, want %v", office.posts(), want)
	}
	for _, want := range []string{"Setup is open in your browser at the demo", "Waiting for the General Manager's hire", "You hired the General Manager",
		"General Manager (gm): working", "Ada (tester): working, on hire/tester-ab12", "All three reviewed.", "Nothing shipped",
		"The demo's page turned on Smolcorp's manager, and left it on"} {
		if !strings.Contains(out, want) {
			t.Errorf("doesn't say %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Someone else's") {
		t.Errorf("shows a hire outside the demo:\n%s", out)
	}
	// Only the roles on Claude Code may be hired, through the fence of every session in the project.
	settings, _ := os.ReadFile(filepath.Join(root, demoRepoName, ".claude", "settings.json"))
	if !strings.Contains(string(settings), "reviewer,tester") || strings.Contains(string(settings), "porter") {
		t.Errorf("the settings: %s", settings)
	}
}

func TestTheDemoSaysWhichSwitchesItsPageTurnsOn(t *testing.T) {
	bothOff := strings.Replace(strings.Replace(demoTestState, "%s", "false", 1), `"hiring":{"enabled":true`, `"hiring":{"enabled":false`, 1)
	managerOff := strings.Replace(demoTestState, "%s", "false", 1)
	leftOn := "turn it off in Setup, Hires and roles: Let a General Manager run your team."

	// The question names the switches that are off, that the page's Hire turns them on, and that they stay on.
	office := &standInOffice{state: bothOff}
	code, out, _ := realDemo(t, office, "n\n")
	if code != 1 || !strings.Contains(out, "Smolcorp's manager is off and hiring is off: the demo's page turns them on when you click Hire, and they stay on until you turn them off in Setup.") ||
		len(office.posts()) != 0 {
		t.Errorf("both off, no: %d %v\n%s", code, office.calls, out)
	}
	if _, out, _ := realDemo(t, &standInOffice{state: managerOff}, "n\n"); !strings.Contains(out, "Smolcorp's manager is off: the demo's page turns it on when you click Hire, and it stays on until you turn it off in Setup.") {
		t.Errorf("the manager off:\n%s", out)
	}
	// Both turned on by the page: both named at the end.
	office = &standInOffice{state: bothOff}
	office.finish = func() {
		folder, _ := office.hire["folder"].(string)
		os.WriteFile(filepath.Join(filepath.Dir(folder), demoReportName), []byte("# Report"), 0o644)
	}
	if code, out, _ := realDemo(t, office, "y\n"); code != 0 || !strings.Contains(out, "The demo's page turned on Smolcorp's manager and hiring, and left them on") {
		t.Errorf("both turned on: %d\n%s", code, out)
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

// FXX1d: the wait for your click on the page is bounded, ends when the page is closed without a hire, on Ctrl-C,
// or when Smolcorp stops answering; each says so, and leaves the folder. Nobody is hired by the demo.
func TestTheDemoWaitsForYourHireAndStopsCleanly(t *testing.T) {
	managerOff := strings.Replace(demoTestState, "%s", "false", 1)
	ends := func(name string, office *standInOffice, ctx context.Context, change func(*officeTeam), want string) {
		t.Helper()
		code, out, in := realDemoWith(t, ctx, office, "y\n", change)
		root := filepath.Join(in, demoFolderName)
		if _, err := os.Stat(filepath.Join(root, demoRepoName, ".git")); err != nil {
			t.Errorf("%s: the demo folder didn't stay: %v", name, err)
		}
		if code != 1 || !strings.Contains(out, want) || !strings.Contains(out, "The demo folder stays: "+root) || strings.Contains(out, "hired the General Manager") ||
			strings.Contains(out, "left it on") || slices.ContainsFunc(office.posts(), func(c string) bool {
			return !strings.HasPrefix(c, "POST /setup/demo/open?") && !strings.HasPrefix(c, "POST /setup/demo/register?")
		}) {
			t.Errorf("%s: %d %v\n%s", name, code, office.posts(), out)
		}
	}
	// The page closed without a hire.
	ends("closed", &standInOffice{state: managerOff, noClick: true, page: "closed"}, context.Background(), nil,
		"The demo's page was closed without a hire: nobody was hired, nothing was spent.")
	// Nobody clicks: the wait is bounded.
	ends("timeout", &standInOffice{state: managerOff, noClick: true, page: "open"}, context.Background(),
		func(o *officeTeam) { o.wait = 50 * time.Millisecond }, "No General Manager was hired on the demo's page within 0 seconds: the demo stops waiting")
	// Ctrl-C while waiting.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	office := &standInOffice{state: managerOff, noClick: true, page: "open"}
	office.onWait = cancel
	ends("Ctrl-C", office, ctx, nil, "Stopped waiting: nobody was hired, nothing was spent. If the demo's page is still open, close it.")
	// The browser didn't open: said, with the way to the link.
	ends("no browser", &standInOffice{state: managerOff, failOpen: true}, context.Background(), nil,
		"The demo's page didn't open. Smolcorp said: The browser didn't open (no xdg-open): run smolcorp demo --no-open for the demo's link.")

	if demoWaitWords(15*time.Minute) != "15 minutes" || demoWaitWords(time.Minute) != "1 minute" || demoWaitWords(30*time.Second) != "30 seconds" {
		t.Error("the wait's words")
	}
}

// --no-open: the link is printed, nothing is opened, and the demo waits for the hire all the same.
func TestTheDemoPrintsItsPageWithNoOpen(t *testing.T) {
	office := &standInOffice{state: strings.Replace(demoTestState, "%s", "false", 1), noClick: true, page: "closed"}
	code, out, in := realDemoWith(t, context.Background(), office, "y\n", func(o *officeTeam) { o.noOpen = true })
	link := office.posts()
	root := filepath.Join(in, demoFolderName)
	// Only told, never opened: the folder is registered all the same, so the printed link has a page to show.
	if code != 1 || len(link) != 1 || !strings.HasPrefix(link[0], "POST /setup/demo/register?root="+url.QueryEscape(root)+"&") || !strings.Contains(out, "/setup?demo="+url.QueryEscape(root)+"\n") || strings.Contains(out, "#o=") ||
		!strings.Contains(out, "It opens signed in where Setup is already open.") || !strings.Contains(out, "closed without a hire") {
		t.Errorf("--no-open: %d %v\n%s", code, link, out)
	}
}

// A Smolcorp that doesn't serve the demo's page (the Mac's own engine, an older app): said before the question, nothing made.
func TestTheDemoNeedsASmolcorpThatServesItsPage(t *testing.T) {
	office := &standInOffice{state: strings.Replace(demoTestState, "%s", "false", 1), noDemo: true}
	code, out, in := realDemo(t, office, "y\n")
	if entries, _ := os.ReadDir(in); code != 1 || len(entries) != 0 || strings.Contains(out, "Go on?") || !strings.Contains(out, demoNotServed()) {
		t.Errorf("no demo page: %d %v\n%s", code, entries, out)
	}
}

// RX1b B3: the GM sends, briefs and fires only the hires the demo made, through the CLI or MCP.
func TestTheDemosFenceKeepsTheGMToItsOwnHires(t *testing.T) {
	in := demoTestFolder(t)
	root := filepath.Join(in, "smolcorp-demo")
	repo := demoCheckout(t, root, "tester-1")
	elsewhere := filepath.Join(in, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	team := []demoMember{
		{ID: "g1", Name: "General Manager", Role: "gm", Folder: repo, Registered: true, State: "working"},
		{ID: "tester-1", Name: "Tester", Nickname: "Ada", Role: "tester", Folder: repo, Worktree: filepath.Join(root, "app-hires", "tester-1"), Registered: true, State: "working"},
		{ID: "rev-9", Name: "Reviewer", Role: "reviewer", Folder: elsewhere, Registered: true, State: "working"},
		{ID: "tester-2", Name: "Tester", Nickname: "Bo", Role: "tester", Folder: repo, Worktree: filepath.Join(elsewhere, "wt"), Registered: true, State: "idle"},
		{ID: "live:abc", Name: "app", Folder: repo},
	}
	f := demoFence{root: root, roles: []string{"tester", "reviewer"}, home: homeDir, team: func() ([]demoMember, error) { return team, nil }}
	tool := func(name string, input map[string]any) demoCall {
		data, _ := json.Marshal(input)
		return demoCall{ToolName: name, ToolInput: data, Cwd: repo}
	}
	allowed := []demoCall{
		bashCall("smolcorp gm send tester-1 --text hi", repo),
		bashCall("smolcorp gm send Ada --text 'hi there'", repo),
		bashCall("smolcorp gm send --text hi ada", repo),
		bashCall("smolcorp gm send gm --text done", repo),
		bashCall("smolcorp gm brief tester-1 --text 'and the docs'", repo),
		bashCall("smolcorp gm fire Ada --force", repo),
		bashCall("smolcorp gm fire tester-1 --remove-worktree --json", repo),
		bashCall("smolcorp gm contacts && smolcorp gm contacts allow Ada tester-1 --one-way", repo),
		tool("mcp__smolcorp__send", map[string]any{"to": "Ada", "text": "hi"}),
		tool("mcp__plugin_smolcorp_smolcorp__brief", map[string]any{"hire": "tester-1", "text": "x"}),
		tool("mcp__smolcorp__fire", map[string]any{"hire": "ada", "force": 1}),
		tool("mcp__smolcorp__contacts", map[string]any{}),
	}
	refused := []demoCall{
		bashCall("smolcorp gm send Reviewer --text hi", repo),
		bashCall("smolcorp gm send rev-9 --text hi", repo),
		bashCall("smolcorp gm brief reviewer --text 'review my branch'", repo),
		bashCall("smolcorp gm fire rev-9", repo),
		bashCall("smolcorp gm fire --force rev-9", repo),
		bashCall("smolcorp gm fire tester-2", repo),
		bashCall("smolcorp gm fire Tester", repo), // two hires called that: one of them works outside
		bashCall("smolcorp gm send live:abc --text hi", repo),
		bashCall("smolcorp gm send bot:x --text hi", repo),
		bashCall("smolcorp gm send nobody --text hi", repo),
		bashCall("smolcorp gm send Ada Reviewer --text hi", repo),
		bashCall("smolcorp gm contacts allow Ada Reviewer", repo),
		bashCall("sh -c 'smolcorp gm fire rev-9'", repo),
		tool("mcp__smolcorp__send", map[string]any{"to": "Reviewer", "text": "hi"}),
		tool("mcp__smolcorp__brief", map[string]any{"hire": "rev-9", "text": "x"}),
		tool("mcp__smolcorp__fire", map[string]any{"hire": "rev-9"}),
		tool("mcp__smolcorp__contacts", map[string]any{"action": "allow", "a": "Ada", "b": "rev-9"}),
		tool("mcp__smolcorp__send", map[string]any{"to": 5, "text": "hi"}),
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
	// A team it can't read: refused, never let through.
	for _, unread := range []func() ([]demoMember, error){nil, func() ([]demoMember, error) { return nil, io.ErrUnexpectedEOF }} {
		g := f
		g.team = unread
		if why := g.check(bashCall("smolcorp gm send Ada --text hi", repo)); !strings.Contains(why, "couldn't read the team") {
			t.Errorf("without the team: %q", why)
		}
	}
	// A hire writes only to its GM or a hire the GM allowed, as the app checks: its send isn't the fence's to judge.
	hire := f
	hire.team, hire.hireSession = nil, true
	if why := hire.check(bashCall("smolcorp gm send Reviewer --text found-one", repo)); why != "" {
		t.Errorf("a hire's send: %s", why)
	}
	if why := hire.check(bashCall("smolcorp gm fire rev-9", repo)); why == "" {
		t.Error("a hire's fire let through")
	}
}

// RX1b B3: the demo starts only in an office with no hire working, so its GM finds none of the owner's.
func TestTheDemoWaitsForAnOfficeWithNoHires(t *testing.T) {
	state := strings.Replace(demoTestState, "%s", "false", 1)
	office := &standInOffice{state: state, others: []map[string]any{
		{"id": "rev-9", "name": "Reviewer", "nickname": "Rex", "role": "reviewer", "folder": "/elsewhere", "registered": true, "state": "working"},
		{"id": "doc-1", "name": "Docs writer", "role": "docs", "folder": "/elsewhere", "registered": true, "state": "idle"},
	}}
	code, out, in := realDemo(t, office, "y\n")
	if entries, _ := os.ReadDir(in); code != 1 || len(entries) != 0 || strings.Contains(out, "Go on?") ||
		!strings.Contains(out, "The demo runs its own team: fire or finish your hires first (Rex, Docs writer)") ||
		slices.ContainsFunc(office.calls, func(c string) bool { return strings.HasPrefix(c, "POST") }) {
		t.Errorf("with hires working: %d %v %v\n%s", code, entries, office.calls, out)
	}
	// Hires fired or failed, and sessions that aren't hires, don't count.
	office = &standInOffice{state: state, others: []map[string]any{
		{"id": "rev-9", "name": "Reviewer", "folder": "/elsewhere", "registered": true, "state": "fired"},
		{"id": "doc-1", "name": "Docs writer", "folder": "/elsewhere", "registered": true, "state": "failed"},
		{"id": "live:abc", "name": "notes", "folder": "/elsewhere", "registered": false, "state": ""},
	}}
	if code, out, _ := realDemo(t, office, "n\n"); code != 1 || !strings.Contains(out, "Go on?") || !strings.Contains(out, "Nothing was made or started.") {
		t.Errorf("with no hire working: %d\n%s", code, out)
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
		for _, name := range []string{"Sauzze", "coach" + "pop", "Dav" + "id", "AgentDeck", "/Users/"} {
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
