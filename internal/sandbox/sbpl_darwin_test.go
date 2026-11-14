package sandbox

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// SA-M1 on this Mac, with stand-ins only (owner Q5 (a)): the real profile, made by PrepareProfile for a stand-in hire
// whose places are all in the test's temp folder, run by /usr/bin/sandbox-exec -f from its file at
// hires/<id>/sandbox.sb. A stand-in hire (python) inside tries each row of the profile's: the support folder whole
// (a stand-in key read, a file added, the folder listed, the canary read, serve.sock connected to, another desk's
// brief) and its exceptions (the guard file and its brief read, its status written, its Claude folder, the desk-start
// socket); the rename guards on the support folder, its worktree, the project and the folders above them; no new
// .claude, in any case; the socket folder written only in its own folder, and no link in that either; the plants, the
// ssh keys, the account folders, the shared git places and the other desks' places. Absent names are tried in another
// case and decomposed (NFD), as the Mac's volume takes them for the same file. Each check is only "ok" or the error's
// name; the stand-in key is never printed. The "ok" rows show the stand-in really runs.

// sbProbe does each step of its JSON argument, a list of [what, path, (path)], and prints a JSON list of "ok" or the
// error's name, one per step.
const sbProbe = `import json, os, socket, sys
said = []
for step in json.loads(sys.argv[1]):
    what, at = step[0], step[1]
    try:
        if what == "read":
            with open(at) as f:
                f.read()
        elif what == "write":
            with open(at, "w") as f:
                f.write("standin")
        elif what == "list":
            os.listdir(at)
        elif what == "stat":
            os.stat(at)
        elif what == "mkdir":
            os.mkdir(at)
        elif what == "rmdir":
            os.rmdir(at)
        elif what == "unlink":
            os.unlink(at)
        elif what == "rename":
            os.rename(at, step[2])
        elif what == "link":
            os.link(at, step[2])
        elif what == "symlink":
            os.symlink(step[2], at)
        elif what == "chmod":
            os.chmod(at, 0o700)
        elif what == "connect":
            s = socket.socket(socket.AF_UNIX)
            s.connect(at)
            s.close()
        said.append("ok")
    except OSError as e:
        said.append(type(e).__name__)
print(json.dumps(said))`

// nfd is s with its é decomposed, as HFS+ and many tools write it.
func nfd(s string) string { return strings.ReplaceAll(s, "é", "é") }

func TestOnAMacTheProfileHoldsForAStandInHire(t *testing.T) {
	sandboxExec, python := "/usr/bin/sandbox-exec", python3(t)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v) or python3 (%q) missing", err, python)
		}
		t.Skip("no sandbox-exec or python3 here")
	}
	// A short folder: a unix socket's path must fit in 104 bytes.
	made, err := os.MkdirTemp("", "sbx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(made) })
	base, err := filepath.EvalSymlinks(made)
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "hé")
	support := filepath.Join(home, "L", "AD")
	own := filepath.Join(support, "hires", "h1")
	hires := filepath.Join(home, "P", "app-hires")
	root, other := filepath.Join(hires, "h1"), filepath.Join(hires, "h2")
	repo := filepath.Join(home, "P", "app")
	git := filepath.Join(repo, ".git")
	sockets := filepath.Join(base, "k")
	files := map[string]string{
		filepath.Join(support, "setup-key"): "FAKEKEY", filepath.Join(support, "sandbox-canary-1"): "standin",
		filepath.Join(support, "hire-guard.json"): `{"fence":1}`, filepath.Join(own, "BRIEF.md"): "brief",
		filepath.Join(support, "hires", "h2", "BRIEF.md"): "another brief", filepath.Join(own, "claude", "todos.json"): "[]",
		filepath.Join(root, ".claude", "commands", "x.md"): "x", filepath.Join(other, "main.go"): "package main",
		filepath.Join(repo, "main.go"): "package main", filepath.Join(git, "config"): "[core]\n", filepath.Join(git, "HEAD"): "ref: refs/heads/main\n",
		filepath.Join(git, "refs", "heads", "main"): "1111\n", filepath.Join(git, "objects", "info", "packs"): "",
		filepath.Join(git, "worktrees", "h1", "HEAD"): "1111\n", filepath.Join(home, ".ssh", "id_x"): "FAKEKEY",
		filepath.Join(home, ".ssh", "id_x.pub"): "ssh-ed25519 standin", filepath.Join(home, ".claude", "CLAUDE.md"): "notes",
		filepath.Join(home, ".claude", ".credentials.json"): "FAKEKEY", filepath.Join(home, ".claude.json"): "{}",
		filepath.Join(sockets, "h2", "s"): "", filepath.Join(sockets, "h1", "f"): "", filepath.Join(base, "elsewhere", "codex", "auth.json"): "standin",
	}
	for at, text := range files {
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{filepath.Join(root, "sub"), filepath.Join(root, "pkg")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, at := range []string{filepath.Join(support, "desk-start"), filepath.Join(support, "serve.sock")} {
		l, err := net.Listen("unix", at)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { l.Close() })
	}
	p := mustNew(t, Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", DeskStart: filepath.Join(support, "desk-start"),
		Root: root, Checkout: repo, GitDir: git, OwnWorktree: "h1", OtherWorktrees: []string{"h1", "h2"}, DefaultBranch: "dé",
		HiresFolder: hires, SocketParent: sockets, OwnSockets: filepath.Join(sockets, "h1"), RemoteLogin: true})

	// PrepareProfile makes the Policy's Hold and writes the profile, owner-only, through a rename.
	file := filepath.Join(own, "sandbox.sb")
	if err := os.Symlink(filepath.Join(base, "elsewhere", "followed.sb"), file); err != nil {
		t.Fatal(err)
	}
	if err := PrepareProfile(p, file); err != nil {
		t.Fatal(err)
	}
	want, _ := p.Profile()
	info, err := os.Lstat(file)
	if data, _ := os.ReadFile(file); err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || string(data) != want {
		t.Fatalf("the profile's file: %v %v, its text the profile's: %v", info.Mode(), err, string(data) == want)
	}
	if _, err := os.Lstat(filepath.Join(base, "elsewhere", "followed.sb")); err == nil {
		t.Error("PrepareProfile followed a link at the profile's place")
	}
	if names, _ := os.ReadDir(own); len(names) != 3 {
		t.Errorf("a temporary file was left beside the profile: %v", names)
	}
	gemini := filepath.Join(home, ".gemini")
	if info, err := os.Lstat(gemini); err != nil || !info.IsDir() {
		t.Fatalf("the Hold folder %s wasn't made: %v", gemini, err)
	}

	type step struct {
		what, at, to, want string
	}
	const ok, no = "ok", "PermissionError"
	steps := []step{
		// The support folder, whole: a key, a file added, the folder listed, the canary (EPERM, not ENOENT), the owner's
		// tap, another desk's brief, a hard link out of it. A file's metadata stays readable.
		{"read", support + "/setup-key", "", no}, {"write", support + "/added", "", no}, {"list", support, "", no},
		{"read", support + "/sandbox-canary-1", "", no}, {"connect", support + "/serve.sock", "", no},
		{"read", support + "/hires/h2/BRIEF.md", "", no}, {"link", support + "/setup-key", root + "/key", no},
		{"stat", support + "/setup-key", "", ok}, {"read", nfd(support) + "/SETUP-KEY", "", no},
		// …its exceptions: the guard file read, not written; its brief and fence read, not written; its status written;
		// its Claude folder, not its settings; the desk-start socket.
		{"read", support + "/hire-guard.json", "", ok}, {"write", support + "/hire-guard.json", "", no},
		{"read", own + "/BRIEF.md", "", ok}, {"write", own + "/BRIEF.md", "", no}, {"read", own + "/sandbox.sb", "", ok},
		{"write", own + "/sandbox.sb", "", no}, {"write", own + "/STATUS.md", "", ok}, {"write", own + "/claude/todos.json", "", ok},
		{"write", own + "/claude/settings.json", "", no}, {"connect", support + "/desk-start", "", ok},
		// The rename guards: the support folder, its worktree, the project, the folders above them, a Hold folder.
		{"rename", support, base + "/elsewhere/AD", no}, {"rename", root, hires + "/h1-moved", no}, {"rename", hires, base + "/elsewhere/h", no},
		{"rename", repo, base + "/elsewhere/app", no}, {"rename", home + "/L", base + "/elsewhere/L", no}, {"rmdir", gemini, "", no},
		{"rename", nfd(home) + "/P", base + "/elsewhere/P", no}, {"rename", root + "/pkg", root + "/pkg2", ok},
		// No new .claude, in any case; its fences.
		{"mkdir", root + "/sub/.claude", "", no}, {"mkdir", root + "/sub/.CLAUDE", "", no}, {"mkdir", root + "/sub/.claudex", "", ok},
		{"write", root + "/.claude/settings.json", "", no}, {"write", root + "/.claude/Settings.Local.json", "", no},
		{"write", root + "/.claude/commands/y.md", "", ok}, {"rename", root + "/.claude", root + "/claude-moved", no},
		// The socket folder: only its own folder, and no link in that.
		{"mkdir", sockets + "/h9", "", no}, {"unlink", sockets + "/h2/s", "", no}, {"write", sockets + "/h1/g", "", ok},
		{"symlink", sockets + "/h1/l", base + "/elsewhere", no}, {"link", sockets + "/h1/f", sockets + "/h1/f2", no},
		// The plants, absent ones in another case and decomposed; the ssh keys while Remote Login answers; the account
		// folders; another agent's sign-in, which the Hold folder above it can't bring in.
		{"write", home + "/.zshrc", "", no}, {"write", nfd(home) + "/.ZSHRC", "", no}, {"write", home + "/.zshrc-notes", "", ok},
		{"read", home + "/.ssh/id_x", "", no}, {"read", home + "/.ssh/id_x.pub", "", ok}, {"write", home + "/.ssh/authorized_keys", "", no},
		{"read", home + "/.claude/CLAUDE.md", "", ok}, {"write", home + "/.claude/CLAUDE.md", "", no},
		{"read", home + "/.claude/.credentials.json", "", no}, {"read", home + "/.claude.json", "", no},
		{"write", gemini + "/oauth_creds.json", "", no}, {"rename", base + "/elsewhere/codex", home + "/.codex", no},
		// The shared git places, the base branches (the default one spelled in another case and decomposed), and the
		// rest of the shared .git; other desks' places.
		{"write", git + "/config", "", no}, {"write", git + "/refs/heads/main", "", no}, {"write", git + "/refs/heads/" + nfd("dé"), "", no},
		{"write", git + "/refs/heads/DÉ", "", no}, {"chmod", git + "/refs", "", no}, {"write", git + "/objects/info/alternates", "", no},
		{"write", git + "/refs/heads/feature", "", ok}, {"write", git + "/worktrees/h1/HEAD", "", ok},
		{"write", other + "/main.go", "", no}, {"write", repo + "/main.go", "", no}, {"write", root + "/main.go", "", ok},
	}
	var args [][]string
	for _, s := range steps {
		arg := []string{s.what, s.at}
		if s.to != "" {
			arg = append(arg, s.to)
		}
		args = append(args, arg)
	}
	encoded, _ := json.Marshal(args)
	cmd := disclaim.Cmd(exec.Command(sandboxExec, "-f", file, python, "-I", "-c", sbProbe, string(encoded)))
	cmd.Dir, cmd.Env = base, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
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
			t.Errorf("%s %s %s: %s, want %s", s.what, strings.TrimPrefix(s.at, base), strings.TrimPrefix(s.to, base), said[i], s.want)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(support, "hire-guard.json")); string(data) != `{"fence":1}` {
		t.Errorf("the guard file is %q", data)
	}
}
