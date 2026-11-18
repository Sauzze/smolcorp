package sandbox

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// planter is a stand-in desk at the project: in the tree at argv[1], a git folder of its own (stash), a gitfile at
// vendor naming it, a .git folder at tools, each config saying `planted`, and a link out to argv[2].
const planter = `import os, sys
root, outside = sys.argv[1], sys.argv[2]
def write(rel, text):
    at = os.path.join(root, rel)
    os.makedirs(os.path.dirname(at), exist_ok=True)
    with open(at, "w") as f:
        f.write(text)
for g in ("stash", "tools/.git"):
    write(g + "/HEAD", "ref: refs/heads/main\n")
    write(g + "/config", "[core]\n\trepositoryformatversion = 0\n\tbare = false\n[standin]\n\twho = planted\n")
    for d in ("objects", "refs"):
        os.makedirs(os.path.join(root, g, d), exist_ok=True)
write("vendor/.git", "gitdir: ../stash\n")
os.symlink(outside, os.path.join(root, "notes"))
print("planted")`

// GITPLANT1 on this Mac, with stand-ins only (FXSAX1R4 Decision 3): a desk at the project, under its own profile,
// plants a gitfile, a .git folder and a link out in the owner's tree, which the Policy can't deny it. A real git, in a
// stand-in repo in the test's temp folder (a fake home, no system or global config, no hooks), then reads the
// plants' config, a harmless `standin.who`, in vendor and tools: so the check can tell. At the desk's stop the core
// lists exactly those three and holds them, in its support folder, which the desk can't write; then git there reads
// the owner's again.
func TestOnAMacAPlantAtTheProjectIsHeldBeforeTheOwnersGitRuns(t *testing.T) {
	sandboxExec, python, gitPath := "/usr/bin/sandbox-exec", python3(t), gitForStandins(t)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" || gitPath == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v), python3 (%q) or git (%q) missing", err, python, gitPath)
		}
		t.Skip("no sandbox-exec, python3 or git here")
	}
	folder, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(folder, "home")
	repo, outside := filepath.Join(home, "Projects", "app"), filepath.Join(folder, "outside")
	support := filepath.Join(home, "Library", "Application Support", "AgentDeck")
	for _, dir := range []string{support, repo, outside} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	env := []string{"HOME=" + home, "PATH=" + filepath.Dir(gitPath) + ":/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CEILING_DIRECTORIES=" + folder, "PYTHONDONTWRITEBYTECODE=1"}
	run := func(args ...string) (string, error) {
		cmd := disclaim.Cmd(exec.Command(args[0], args[1:]...))
		cmd.Dir, cmd.Env = folder, env
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	gitOK := func(dir string, args ...string) string {
		t.Helper()
		out, err := run(append([]string{gitPath, "-C", dir, "-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false",
			"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main", "-c", "user.name=standin",
			"-c", "user.email=standin@example.invalid"}, args...)...)
		if err != nil {
			t.Fatalf("git %v in %s: %v %s", args, dir, err, out)
		}
		return out
	}
	gitOK(repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "standin.txt"), []byte("standin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitOK(repo, "add", "standin.txt")
	gitOK(repo, "commit", "-q", "-m", "standin")
	gitOK(repo, "config", "standin.who", "owner")
	who := func(rel string) string { return gitOK(filepath.Join(repo, rel), "config", "standin.who") }

	git := filepath.Join(repo, ".git")
	p := mustNew(t, Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", Root: repo, GitDir: git,
		DefaultBranch: DefaultBranch(git), HiresFolder: repo + "-hires"})
	if err := MakeHold(p.Hold()); err != nil {
		t.Fatal(err)
	}
	profile := writeProfile(t, p, folder)
	sandboxed := func(args ...string) (string, error) {
		cmd := disclaim.Cmd(exec.Command(sandboxExec, append([]string{"-p", profile}, args...)...))
		cmd.Dir, cmd.Env = folder, env
		out, err := cmd.CombinedOutput()
		return string(bytes.TrimSpace(out)), err
	}

	start, err := Snap(repo)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := sandboxed(python, "-I", "-c", planter, repo, outside); err != nil || out != "planted" {
		t.Fatalf("the stand-in desk didn't plant: %v %q", err, out)
	}
	for _, rel := range []string{"vendor", "tools"} {
		if got := who(rel); got != "planted" {
			t.Fatalf("git in %s reads %q before the hold, want planted (the check can't tell)", rel, got)
		}
	}

	plants, err := Plants(repo, start)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range plants {
		got = append(got, line(p.Path, p.Kind))
	}
	sameLines(t, "the desk's stop", got, []string{line("notes", LinkOutside), line("tools/.git", GitFolder), line("vendor/.git", GitFile)})
	heldDir := filepath.Join(support, "held", "h1-1")
	held, err := HoldPlants(repo, heldDir, plants)
	if err != nil || len(held) != 3 {
		t.Fatalf("held %v: %v", held, err)
	}
	for _, rel := range []string{"vendor", "tools", "."} {
		if got := who(rel); got != "owner" {
			t.Errorf("after the hold, git in %s reads %q, want owner", rel, got)
		}
	}

	// The held plants are out of every desk's reach: the support folder is denied.
	for _, h := range held {
		if v, by := p.Decide(h.At, Write); v != Deny {
			t.Errorf("write %s: %v by %q", h.At, v, by.Row)
		}
	}
	at := filepath.Join(heldDir, "3", ".git")
	if out, _ := sandboxed(python, "-I", "-c", pythonWrite, at); !strings.Contains(out, "PermissionError") {
		t.Errorf("the desk wrote the held %s: %s", at, out)
	}
	if data, _ := os.ReadFile(at); string(data) != "gitdir: ../stash\n" {
		t.Errorf("the held gitfile is now %q", data)
	}
}
