package desktop

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// SA-X8 under the real profile, SANDBOX-PLAN's T14 rows, with stand-ins only (owner Q5 (a)): a desk folder made by
// makeDeskClaude in a stand-in home and support folder, the Policy's profile made by PrepareProfile, and a stand-in
// desk (python) run by /usr/bin/sandbox-exec. Through the links, the account's notes, plugins and skills are read but
// never written; the owner's shell snapshots and IDE lock, ~/.claude.json and another desk's folder are out of reach;
// its own folder (a snapshot, its .claude.json) is its to write, but not its settings, nor the links the core put
// there; its plans, in its place, are its to write and read. Each step says only "ok" or the error's name.

// deskProbe does each step of its JSON argument, [what, path], and prints a JSON list of "ok" or the error's name.
const deskProbe = `import json, os, sys
said = []
for what, at in json.loads(sys.argv[1]):
    try:
        if what == "read":
            with open(at) as f:
                f.read()
        elif what == "write":
            with open(at, "w") as f:
                f.write("standin")
        elif what == "mkdir":
            os.mkdir(at)
        elif what == "unlink":
            os.unlink(at)
        said.append("ok")
    except OSError as e:
        said.append(type(e).__name__)
print(json.dumps(said))`

// deskPython is a python to run the stand-in with, or "". /usr/bin/python3 is Apple's shim: with no developer tools
// it asks to install them, a system dialog, so it is used only when xcode-select names a developer folder.
func deskPython(t *testing.T) string {
	found, err := exec.LookPath("python3")
	if err != nil {
		return ""
	}
	if found == "/usr/bin/python3" && disclaim.Cmd(exec.Command("/usr/bin/xcode-select", "-p")).Run() != nil {
		return ""
	}
	return found
}

func TestOnAMacADeskReadsItsAccountThroughItsFolderButWritesOnlyItsOwn(t *testing.T) {
	sandboxExec, python := "/usr/bin/sandbox-exec", deskPython(t)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v) or python3 (%q) missing", err, python)
		}
		t.Skip("no sandbox-exec or python3 here")
	}
	isolate(t)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// The profile names resolved places, as the kernel sees them.
	homeDir, supportDir = filepath.Join(base, "home"), filepath.Join(base, "support")
	account, _ := deskAccount(t, "")
	hires := filepath.Join(base, "P", "app-hires")
	root := filepath.Join(hires, "h1")
	for path, text := range map[string]string{
		filepath.Join(account, "shell-snapshots", "snapshot-owner.sh"): "owner", filepath.Join(account, "ide", "1.lock"): "{}",
		filepath.Join(supportDir, "hires", "h2", "claude", "todos.json"): "[]", filepath.Join(root, "main.go"): "package main",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := makeDeskPlans(root, "h1"); err != nil {
		t.Fatal(err)
	}
	own := deskClaudeFolder(supportDir, "h1")
	if err := makeDeskClaude(own, deskClaude{Trusted: root}); err != nil {
		t.Fatal(err)
	}
	p, err := sandbox.New(sandbox.Places{GOOS: "darwin", Home: homeDir, Support: supportDir, OwnID: "h1", Root: root, HiresFolder: hires})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(hireFolder(supportDir, "h1"), "sandbox.sb")
	if err := sandbox.PrepareProfile(p, file); err != nil {
		t.Fatal(err)
	}

	const ok, no = "ok", "PermissionError"
	steps := []struct{ what, at, want string }{
		// Read through its links: the owner's notes and skills.
		{"read", own + "/CLAUDE.md", ok}, {"read", own + "/skills/s/SKILL.md", ok}, {"read", own + "/settings.json", ok},
		// Never written through them: the notes, a plugin's hook, a new skill.
		{"write", own + "/CLAUDE.md", no}, {"write", own + "/plugins/hook.sh", no}, {"mkdir", own + "/skills/new", no},
		{"write", own + "/skills/s/SKILL.md", no},
		// The owner's shell snapshot and IDE lock, ~/.claude.json, another desk's folder.
		{"write", account + "/shell-snapshots/snapshot-owner.sh", no}, {"read", account + "/ide/1.lock", no},
		{"write", account + "/ide/1.lock", no}, {"read", homeDir + "/.claude.json", no},
		{"write", supportDir + "/hires/h2/claude/todos.json", no},
		// Its own: a snapshot, its .claude.json; not its settings, nor the links the core put there.
		{"mkdir", own + "/shell-snapshots", ok}, {"write", own + "/shell-snapshots/snapshot-desk.sh", ok},
		{"write", own + "/.claude.json", ok}, {"write", own + "/settings.json", no}, {"unlink", own + "/CLAUDE.md", no},
		{"write", root + "/main.go", ok},
		// Its plans, pinned in its place (deskPlansFor): written and read again (RT126SAX8 F1).
		{"write", root + "/" + deskPlansFor("h1") + "/calm-river.md", ok}, {"read", root + "/" + deskPlansFor("h1") + "/calm-river.md", ok},
	}
	var args [][]string
	for _, s := range steps {
		args = append(args, []string{s.what, s.at})
	}
	encoded, _ := json.Marshal(args)
	cmd := disclaim.Cmd(exec.Command(sandboxExec, "-f", file, python, "-I", "-c", deskProbe, string(encoded)))
	cmd.Dir, cmd.Env = base, []string{"HOME=" + homeDir, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("the stand-in didn't run under the profile: %v %s", err, stderr.Bytes())
	}
	var said []string
	if err := json.Unmarshal(out.Bytes(), &said); err != nil || len(said) != len(steps) {
		t.Fatalf("the stand-in said %q (%v)", out.Bytes(), err)
	}
	for i, s := range steps {
		if said[i] != s.want {
			t.Errorf("%s %s: %s, want %s", s.what, strings.TrimPrefix(s.at, base), said[i], s.want)
		}
	}
	for path, want := range map[string]string{filepath.Join(account, "CLAUDE.md"): "the owner's notes",
		filepath.Join(account, "skills", "s", "SKILL.md"): "a skill", filepath.Join(account, "shell-snapshots", "snapshot-owner.sh"): "owner"} {
		if data, _ := os.ReadFile(path); string(data) != want {
			t.Errorf("%s is now %q", strings.TrimPrefix(path, base), data)
		}
	}
}
