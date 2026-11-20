package sandbox

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// FXREPLACEREFS on this Mac, with stand-ins only: under the real profile (PrepareProfile, sandbox-exec -f), a stand-in
// hire (python, sbProbe) writes none of what makes the owner's git read a base branch as another tree or history: a
// replace ref (made as git makes one, through its lock file), the shallow list (written, or a file renamed onto it), a
// multi-pack index (or its chain's folder), a base branch's reflog, and none of the folders above them moves. A pack,
// its index, an object, its own branch's reflog and a tag are still its work, so the stand-in really runs. Each check
// is only "ok" or the error's name, and the files are plain text stand-ins nobody's git reads.
func TestOnAMacTheProfileKeepsGitsSubstitutionsFromADesk(t *testing.T) {
	sandboxExec, python := "/usr/bin/sandbox-exec", python3(t)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v) or python3 (%q) missing", err, python)
		}
		t.Skip("no sandbox-exec or python3 here")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "home")
	support := filepath.Join(home, "L", "AD")
	own := filepath.Join(support, "hires", "h1")
	hires := filepath.Join(home, "P", "app-hires")
	root := filepath.Join(hires, "h1")
	repo := filepath.Join(home, "P", "app")
	git := filepath.Join(repo, ".git")
	const old = "1111111111111111111111111111111111111111\n"
	for at, text := range map[string]string{
		git + "/config": "[core]\n", git + "/HEAD": "ref: refs/heads/main\n", git + "/refs/heads/main": old,
		git + "/logs/refs/heads/main": old, git + "/objects/pack/pack-a.pack": "standin", git + "/objects/pack/pack-a.idx": "standin",
		git + "/worktrees/h1/HEAD": old, git + "/refs/tags/v1": old, git + "/logs/refs/heads/hire/h1/work": old,
		root + "/main.go": "package main", own + "/BRIEF.md": "brief", base + "/elsewhere/shallow": old,
	} {
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// The owner's own replace refs' folder, there already (`git replace` run once): a desk's write inside it is refused,
	// not just missing a folder.
	if err := os.MkdirAll(git+"/refs/replace", 0o700); err != nil {
		t.Fatal(err)
	}
	p := mustNew(t, Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", Root: root, Checkout: repo, GitDir: git,
		OwnWorktree: "h1", OtherWorktrees: []string{"h1"}, DefaultBranch: "develop", HiresFolder: hires})
	file := filepath.Join(own, "sandbox.sb")
	if err := PrepareProfile(p, file); err != nil {
		t.Fatal(err)
	}

	type step struct{ what, at, to, want string }
	const ok, no = "ok", "PermissionError"
	replace := git + "/refs/replace/0123456789abcdef0123456789abcdef01234567"
	steps := []step{
		{"mkdir", git + "/refs/replace/sub", "", no}, {"write", replace + ".lock", "", no}, {"write", replace, "", no},
		{"rename", git + "/refs/replace", git + "/refs/replace-moved", no},
		{"write", git + "/shallow", "", no}, {"rename", base + "/elsewhere/shallow", git + "/shallow", no},
		{"write", git + "/objects/pack/multi-pack-index", "", no}, {"mkdir", git + "/objects/pack/multi-pack-index.d", "", no},
		{"write", git + "/logs/refs/heads/main", "", no}, {"write", git + "/logs/refs/heads/develop", "", no},
		{"rename", git + "/objects/pack", git + "/objects/pack-moved", no}, {"rename", git + "/logs", git + "/logs-moved", no},
		// …and what a desk's commits and gc write.
		{"write", git + "/objects/pack/pack-b.pack", "", ok}, {"write", git + "/objects/pack/pack-b.idx", "", ok},
		{"write", git + "/objects/pack/pack-b.rev", "", ok}, {"write", git + "/objects/pack/pack-b.bitmap", "", ok},
		{"unlink", git + "/objects/pack/pack-a.idx", "", ok}, {"mkdir", git + "/objects/ab", "", ok},
		{"write", git + "/objects/ab/cdef", "", ok}, {"write", git + "/logs/refs/heads/hire/h1/work", "", ok},
		// A ref of its own beside refs/replace: its branch, since a desk writes no tag from FXGITROWS2R4 (this stand-in
		// moved in FXGITROWS2R6).
		{"write", git + "/refs/heads/hire/h1", "", ok}, {"write", git + "/worktrees/h1/HEAD", "", ok}, {"write", root + "/main.go", "", ok},
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
	for _, at := range []string{git + "/shallow", replace, git + "/objects/pack/multi-pack-index"} {
		if _, err := os.Lstat(at); err == nil {
			t.Errorf("%s was made", strings.TrimPrefix(at, base))
		}
	}
}
