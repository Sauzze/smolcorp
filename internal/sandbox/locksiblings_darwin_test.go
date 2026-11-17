package sandbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// FXLOCKSIBLINGS on this Mac, with stand-ins only: under the real profile (PrepareProfile, sandbox-exec -f), a stand-in
// hire (python, sbProbe) opens none of the names a denied file's writers write first and then rename onto it, whether
// the owner's program has it there already (the race RT126REPLACEREFS showed: the desk keeps a handle the rename then
// puts in the file's place) or the desk would make it: git's lock beside the shared config, index, HEAD, a base branch,
// packed-refs (and its new copy), shallow, a base branch's reflog and a worktree's config; the owner's programs' names
// beside ~/.gitconfig, a plant, git's other config and what it names; the writers' names beside a fence and its own
// Claude folder's fence; a sign-in's copy, not even read. Its own branch's lock, its worktree's index and HEAD locks,
// a tag's lock, packed-refs' lock (held, never renamed), a temporary pack, its Claude folder's .claude.json copy and
// its status are still its work. Each check
// is only "ok" or the error's name, and the files are plain text stand-ins nobody's git reads.
func TestOnAMacTheProfileKeepsADeniedFilesWritersNamesFromADesk(t *testing.T) {
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
	etc := filepath.Join(base, "etc", "gitconfig")
	const old = "1111111111111111111111111111111111111111\n"
	// What the owner's programs have there at the moment the desk opens it: git's locks and the other writers'
	// copies, each about to be renamed onto its file.
	held := []string{
		git + "/config.lock", git + "/index.lock", git + "/HEAD.lock", git + "/refs/heads/main.lock",
		git + "/refs/heads/develop.lock", git + "/shallow.lock", git + "/logs/refs/heads/main.lock",
		git + "/worktrees/h1/config.worktree.lock", home + "/.gitconfig.lock", etc + ".lock", home + "/.gitconfig-work.lock",
		home + "/.claude/.credentials.json.tmp.4242.1763400000000", home + "/.codex/auth.json.tmp",
		root + "/.claude/settings.local.json.tmp", own + "/claude/settings.json.tmp.4242.1763400000000",
		own + "/claude/.settings.json.fresh-12345",
	}
	files := map[string]string{
		git + "/config": "[core]\n", git + "/HEAD": "ref: refs/heads/main\n", git + "/refs/heads/main": old,
		git + "/logs/refs/heads/main": old, git + "/packed-refs": old, git + "/objects/pack/pack-a.pack": "standin",
		git + "/worktrees/h1/HEAD": old, git + "/refs/tags/v1": old, git + "/refs/heads/hire/h1/work": old,
		git + "/logs/refs/heads/hire/h1/work": old, root + "/main.go": "package main", root + "/.claude/settings.local.json": "{}",
		own + "/BRIEF.md": "brief", own + "/claude/settings.json": "{}", home + "/.gitconfig": "[user]\n", home + "/.ssh/known_hosts": "",
		home + "/.claude/CLAUDE.md": "the owner's", etc: "[core]\n",
	}
	for _, at := range held {
		files[at] = "the owner's\n"
	}
	for at, text := range files {
		mustWrite(t, at, text)
	}
	p := mustNew(t, Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", Root: root, Checkout: repo, GitDir: git,
		OwnWorktree: "h1", OtherWorktrees: []string{"h1"}, DefaultBranch: "develop", HiresFolder: hires,
		GitConfigs: []string{etc}, GitNamed: []string{home + "/.gitconfig-work"}})
	file := filepath.Join(own, "sandbox.sb")
	if err := PrepareProfile(p, file); err != nil {
		t.Fatal(err)
	}

	type step struct{ what, at, want string }
	const ok, no = "ok", "PermissionError"
	var steps []step
	for _, at := range held {
		steps = append(steps, step{"write", at, no})
	}
	// …made by the desk itself, before the owner's program would.
	for _, at := range []string{git + "/packed-refs.new", git + "/config.worktree.lock", git + "/gitdir.lock",
		git + "/logs/refs/heads/develop.lock", home + "/.zshrc.tmp.4242.1763400000000", home + "/.zshenv___jb_tmp___",
		home + "/.ssh/config.lock", root + "/.claude/settings.json.tmp.4242.1763400000000", own + "/claude/settings.local.json.lock",
		home + "/.gemini/oauth_creds.json.tmp"} {
		steps = append(steps, step{"write", at, no})
	}
	// A sign-in's copy isn't read either.
	steps = append(steps, step{"read", home + "/.claude/.credentials.json.tmp.4242.1763400000000", no},
		step{"read", home + "/.codex/auth.json.tmp", no})
	// …and what a desk's own git and Claude Code write beside them.
	// packed-refs' lock among them: git only holds it, and renames packed-refs.new onto packed-refs.
	for _, at := range []string{git + "/refs/heads/hire/h1/work.lock", git + "/logs/refs/heads/hire/h1/work.lock", git + "/packed-refs.lock",
		git + "/worktrees/h1/index.lock", git + "/worktrees/h1/HEAD.lock", git + "/refs/tags/v2.lock",
		git + "/objects/pack/tmp_pack_x", git + "/refs/heads/mainline.lock", root + "/.claude/notes.md", root + "/main.go",
		own + "/claude/.claude.json.tmp.4242.1763400000000", own + "/STATUS.md", home + "/.zsh_history", home + "/.zshrc-notes",
		home + "/.gitconfig-work.bak"} {
		steps = append(steps, step{"write", at, ok})
	}
	steps = append(steps, step{"read", home + "/.claude/CLAUDE.md", ok})
	var args [][]string
	for _, s := range steps {
		args = append(args, []string{s.what, s.at})
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
			t.Errorf("%s %s: %s, want %s", s.what, strings.TrimPrefix(s.at, base), said[i], s.want)
		}
	}
	for _, at := range held {
		if data, _ := os.ReadFile(at); string(data) != "the owner's\n" {
			t.Errorf("%s changed: %q", strings.TrimPrefix(at, base), data)
		}
	}
}

// RT126REPLACEREFS' race, replayed against the owner's real git in a throwaway repo: a stand-in desk in its sandbox
// waits for the lock (or packed-refs' new copy) the owner's `git commit`, `git config`, `git add` or `git pack-refs`
// writes, and tries to open it for writing the moment it is there. It only says whether the open worked: it writes
// nothing. The profile must refuse it at the open. Without the deny, the open works on the owner's first or second
// run, and the handle would write main, the shared config, the index or packed-refs once git renames the file.
func TestOnAMacADeskCantOpenTheOwnersGitLocks(t *testing.T) {
	sandboxExec := sandboxExecOrSkip(t)
	git := gitOrSkip(t)
	python := python3(t)
	if python == "" {
		t.Skip("no python3 here")
	}
	d, outside := newGitDesk(t, git)
	// The stand-in: it says "ready", then spins on opening at for writing until it is there or time is up.
	const spin = `import os, sys, time
at, until = sys.argv[1], time.time() + float(sys.argv[2])
print("ready", flush=True)
while time.time() < until:
    try:
        fd = os.open(at, os.O_WRONLY)
    except FileNotFoundError:
        continue
    except PermissionError:
        print("PermissionError", flush=True)
        sys.exit(0)
    os.close(fd)
    print("opened", flush=True)
    sys.exit(0)
print("never there", flush=True)`
	for _, c := range []struct {
		name, at string
		owner    func(i int) []string
	}{
		{"git commit on main", "refs/heads/main.lock", func(i int) []string {
			return []string{"commit", "-q", "--allow-empty", "-m", fmt.Sprintf("the owner's %d", i)}
		}},
		{"git config", "config.lock", func(i int) []string { return []string{"config", "owner.note", fmt.Sprintf("n%d", i)} }},
		{"git add", "index.lock", func(i int) []string {
			mustWrite(t, filepath.Join(d.checkout, "README"), fmt.Sprintf("the owner's checkout, %d", i))
			return []string{"add", "README"}
		}},
		{"git pack-refs", "packed-refs.new", func(i int) []string {
			outside(d.checkout, "tag", fmt.Sprintf("t%d", i))
			return []string{"pack-refs", "--all"}
		}},
	} {
		func() {
			cmd := disclaim.Cmd(exec.Command(sandboxExec, "-f", d.profile, python, "-I", "-c", spin, filepath.Join(d.git, c.at), "20"))
			cmd.Dir, cmd.Env = d.root, []string{"HOME=" + d.home, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
			lines := bufio.NewScanner(stdout)
			if !lines.Scan() || lines.Text() != "ready" {
				t.Fatalf("%s: the stand-in didn't start: %q %s", c.name, lines.Text(), stderr.Bytes())
			}
			said := make(chan string, 1)
			go func() {
				lines.Scan()
				said <- lines.Text()
			}()
			var got string
			runs := 0
			for deadline := time.Now().Add(15 * time.Second); got == "" && time.Now().Before(deadline); runs++ {
				outside(d.checkout, c.owner(runs)...)
				select {
				case got = <-said:
				default:
				}
			}
			if got == "" {
				select {
				case got = <-said:
				case <-time.After(10 * time.Second):
				}
			}
			if got != "PermissionError" {
				t.Errorf("the desk's open of %s while the owner's %s ran (%d runs): %q, want PermissionError at the open", c.at, c.name, runs, got)
			} else {
				t.Logf("%s: refused at the open, after %d of the owner's runs", c.name, runs)
			}
		}()
	}
	if got := outside(d.checkout, "config", "--get", "owner.note"); !strings.HasPrefix(got, "n") {
		t.Errorf("the owner's config: %q", got)
	}
}

// A stand-in desk's real git round in its linked worktree, under the real profile, beside the denied locks: its own
// commits, a rebase onto the owner's new main, a branch of its own made, merged and deleted, a tag, a fetch into a
// remote-tracking ref, a repack and its status all still work, with nothing refused on the way (git 2.55 takes
// packed-refs' lock to drop a commit's and a rebase's own refs), and main stays where the owner put it.
func TestOnAMacADesksGitRoundStillWorksBesideTheDeniedLocks(t *testing.T) {
	sandboxExecOrSkip(t)
	git := gitOrSkip(t)
	d, outside := newGitDesk(t, git)
	hireGit := func(args ...string) {
		t.Helper()
		r := d.inside(t, gitTestEnv(git), git, args...)
		if r.status != 0 {
			t.Errorf("the desk's git %q: %+v", args, r)
		}
		// Two of the shared git places, refused before this piece too: a repack's lists for the dumb http transport
		// (`info`, `objects/info`), and the shared config a branch's deletion drops its section from (on the base, git
		// took config.lock and its rename failed: "could not write config file"). Nothing else may be said.
		for _, line := range strings.Split(strings.TrimSpace(r.stderr), "\n") {
			switch {
			case line == "":
			case args[0] == "repack" && (strings.HasSuffix(line, "/.git/info/refs: Operation not permitted") ||
				strings.HasSuffix(line, "/.git/objects/info/packs: Operation not permitted")):
			case args[0] == "branch" && (line == "error: could not lock config file "+filepath.Join(d.git, "config") ||
				line == "warning: update of config-file failed"):
			default:
				t.Errorf("the desk's git %q: %s", args, line)
			}
		}
	}
	mustWrite(t, filepath.Join(d.root, "work.go"), "package work")
	hireGit("add", "work.go")
	hireGit("commit", "-q", "-m", "the desk's work")
	outside(d.checkout, "commit", "-q", "--allow-empty", "-m", "the owner's next")
	mainBefore := outside(d.checkout, "rev-parse", "main")
	for _, args := range [][]string{
		{"rebase", "-q", "main"},
		{"branch", "h1-side"}, {"switch", "-q", "h1-side"},
		{"commit", "-q", "--allow-empty", "-m", "on the side"}, {"switch", "-q", "h1"},
		{"merge", "-q", "--no-ff", "--no-edit", "h1-side"}, {"branch", "-D", "h1-side"},
		{"tag", "h1-v1"}, {"fetch", "-q", ".", "main:refs/remotes/o/main"}, {"repack", "-q", "-d"},
		{"status", "--short"}, {"log", "--oneline", "-3"},
	} {
		hireGit(args...)
	}
	if got := outside(d.checkout, "rev-parse", "main"); got != mainBefore {
		t.Errorf("main moved: %s, was %s", got, mainBefore)
	}
	if got := outside(d.checkout, "log", "-1", "--format=%s", "h1^2"); got != "on the side\n" {
		t.Errorf("the desk's merge: %q", got)
	}
	if got := outside(d.checkout, "branch", "--list", "h1-side"); got != "" {
		t.Errorf("the desk's branch -D: h1-side is still there: %q", got)
	}
	if got := outside(d.checkout, "rev-parse", "refs/remotes/o/main"); got != mainBefore {
		t.Errorf("the desk's fetch: %s, want %s", got, mainBefore)
	}
}

func gitTestEnv(git string) []string {
	return []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Stand In",
		"GIT_AUTHOR_EMAIL=standin@example.invalid", "GIT_COMMITTER_NAME=Stand In", "GIT_COMMITTER_EMAIL=standin@example.invalid",
		"PATH=" + filepath.Dir(git) + ":/usr/bin:/bin"}
}

// newGitDesk is an escape desk whose repo git made itself, a first commit on main and the hire's linked worktree on
// its own branch, with its real profile; the function it gives runs the owner's git, outside any sandbox, through
// outside (escape2_darwin_test.go, startRefusal first), and gives its output.
func newGitDesk(t *testing.T, git string) (*escapeDesk, func(dir string, args ...string) string) {
	t.Helper()
	d := newEscapeDesk(t)
	for _, gone := range []string{d.checkout, d.root, filepath.Join(d.home, "app-hires", "h2")} {
		if err := os.RemoveAll(gone); err != nil {
			t.Fatal(err)
		}
	}
	env := append([]string{"HOME=" + d.home}, gitTestEnv(git)...)
	owner := func(dir string, args ...string) string {
		t.Helper()
		return outside(t, dir, env, git, args...)
	}
	if err := os.MkdirAll(d.checkout, 0o700); err != nil {
		t.Fatal(err)
	}
	owner(d.checkout, "init", "-q", "-b", "main")
	owner(d.checkout, "config", "commit.gpgsign", "false")
	owner(d.checkout, "config", "tag.gpgsign", "false")
	mustWrite(t, filepath.Join(d.checkout, "README"), "the owner's checkout")
	owner(d.checkout, "add", "README")
	owner(d.checkout, "commit", "-q", "-m", "first")
	owner(d.checkout, "worktree", "add", "-q", "-b", "h1", d.root)
	owner(d.checkout, "worktree", "add", "-q", "-b", "h2", filepath.Join(d.home, "app-hires", "h2"))
	mustWrite(t, filepath.Join(d.root, ".claude", "settings.json"), "{}")
	d.prepare(t)
	return d, owner
}

// ownersToolsDesk is an escape desk whose Places name git's other config (etc/gitconfig, outside the home folder) and
// a file the owner's git config names (~/.gitconfig-work), with stand-ins of the plants the rows below use.
func ownersToolsDesk(t *testing.T) (d *escapeDesk, etc string) {
	t.Helper()
	d = newEscapeDesk(t)
	etc = filepath.Join(d.base, "etc", "gitconfig")
	for at, text := range map[string]string{etc: "[core]\n\tn = 0\n", filepath.Join(d.home, ".zshrc"): "alias a=0\n",
		filepath.Join(d.home, ".gitconfig"): "[user]\n\tn = 0\n", filepath.Join(d.home, ".gitconfig-work"): "[user]\n",
		filepath.Join(d.home, ".zlogin"): "# the owner's\n", filepath.Join(d.home, ".bashrc"): "alias b=0\n",
		filepath.Join(d.home, ".ssh", "config"): "Host a\n  Port 1\n"} {
		mustWrite(t, at, text)
	}
	d.pl.GitConfigs, d.pl.GitNamed = []string{etc}, []string{filepath.Join(d.home, ".gitconfig-work")}
	d.prepare(t)
	return d, etc
}

// FXLOCKSIBLINGSR1 (RT126LOCKSIBLINGS' B1, widened by SIERRA) under the real profile, with stand-ins: the names the
// owner's tools write beside a plant or git's other config, then rename onto it, are neither opened nor made by a desk,
// and its own file isn't renamed onto them: macOS's sed -i (`.!<pid>!<name>`), mise's dotfiles (`.<name>.tmp.<pid>.<n>`),
// install(1) (`INS@` and six) and perl -i (`XX` and six) in the plant's folder, and oh-my-zsh's `.zshrc-omztemp` and
// `.zshrc.pre-oh-my-zsh`. A neighbour's name, a short or long name like theirs, and the same names in its own
// worktree stay its own, and so does perl -i's directly in ~ (SIERRA's ruling on RT126LOCKSIBLINGSR2's B2: the report
// lists its race, R1). Each check is only "ok" or the error's name.
func TestOnAMacTheProfileKeepsTheOwnersToolsNamesBesideAPlantFromADesk(t *testing.T) {
	python := python3(t)
	if python == "" {
		t.Skip("no python3 here")
	}
	d, etc := ownersToolsDesk(t)
	h, conf := d.home, filepath.Dir(etc)
	held := []string{h + "/.!4242!.zshrc", h + "/.!4242!.gitconfig", conf + "/.!4242!gitconfig", h + "/.!4242!.gitconfig-work",
		h + "/..zshrc.tmp.4242.0", conf + "/.gitconfig.tmp.4242.0", h + "/INS@q4Ontc", conf + "/INS@q4Ontc",
		conf + "/XXNu6Xnm", h + "/.zshrc.pre-oh-my-zsh"}
	for _, at := range held {
		mustWrite(t, at, "the owner's\n")
	}
	mustWrite(t, h+"/desk-own", "the desk's\n")
	type step struct{ what, at, to, want string }
	const ok, no = "ok", "PermissionError"
	var steps []step
	for _, at := range held {
		steps = append(steps, step{"write", at, "", no})
	}
	// …made by the desk, before the owner's tool would: RT126LOCKSIBLINGS' omz create printed "made".
	for _, at := range []string{h + "/.zshrc-omztemp", h + "/.!1!.bashrc", h + "/..zprofile.tmp.1.0", conf + "/XXabcdef",
		h + "/INS@zzzzzz", conf + "/.!1!gitconfig", h + "/.!1!.gitconfig-work"} {
		steps = append(steps, step{"write", at, "", no})
	}
	// …or its own file renamed onto the one the owner's tool has there, or onto oh-my-zsh's name before its installer
	// runs (RT126LOCKSIBLINGSR2's probe: the shell's `>` then writes the desk's file).
	for _, at := range []string{h + "/..zshrc.tmp.4242.0", h + "/.!4242!.zshrc", h + "/INS@q4Ontc", h + "/.zshrc.pre-oh-my-zsh",
		h + "/.zshrc-omztemp"} {
		steps = append(steps, step{"rename", h + "/desk-own", at, no})
	}
	for _, at := range []string{h + "/.zshrc-notes", h + "/XXshort", h + "/.!x.zshrc", h + "/.bashrc-omztemp",
		d.root + "/INS@q4Ontc", d.root + "/XXNu6Xnm", d.root + "/.!1!main.go", h + "/desk-own", h + "/XXNu6Xnm", h + "/XXabcdef"} {
		steps = append(steps, step{"write", at, "", ok})
	}
	var args [][]string
	for _, s := range steps {
		args = append(args, []string{s.what, s.at, s.to})
	}
	encoded, _ := json.Marshal(args)
	r := d.inside(t, []string{"PYTHONDONTWRITEBYTECODE=1"}, python, "-I", "-c", sbProbe, string(encoded))
	var said []string
	if err := json.Unmarshal([]byte(r.stdout), &said); err != nil || len(said) != len(steps) {
		t.Fatalf("the stand-in said %q (%v) %s", r.stdout, err, r.stderr)
	}
	for i, s := range steps {
		if said[i] != s.want {
			t.Errorf("%s %s %s: %s, want %s", s.what, strings.TrimPrefix(s.at, d.base), strings.TrimPrefix(s.to, d.base), said[i], s.want)
		}
	}
	for _, at := range held {
		if data, _ := os.ReadFile(at); string(data) != "the owner's\n" {
			t.Errorf("%s changed: %q", strings.TrimPrefix(at, d.base), data)
		}
	}
}

// RT126LOCKSIBLINGS' two replays and their kin, against the owner's real tools, run outside any sandbox on stand-ins
// in the desk's throwaway home: a stand-in desk in its sandbox lists the folder, and the moment a name the owner's
// tool writes there appears, opens it for writing (it writes nothing), or renames its own file onto it (mise's is a
// link, which an open would follow). RT126LOCKSIBLINGSR2's other tactics are cases too: the open by the name's
// upper-case spelling, by /.vol and its inode, through a hard link in its worktree, and a renamex_np swap of its own
// file with it. The profile must refuse each at the open, the link or the rename. Replayed: macOS's
// `sed -i` with an empty suffix on ~/.zshrc and on git's other config, oh-my-zsh's two lines (`> ~/.zshrc-omztemp`,
// then `mv -f ~/.zshrc-omztemp ~/.zshrc`) and its install's and uninstall's moves of ~/.zshrc.pre-oh-my-zsh, `install`
// onto ~/.zshrc and ~/.ssh/config, `install -S` onto ~/.bashrc, the same `sed -i` on ~/.ssh/config and on what git's
// config names, `perl -pi` on git's other config (its `XX` name is held there, not in ~ itself: SIERRA's ruling on
// RT126LOCKSIBLINGSR2's B2) and, where mise is installed, `mise bootstrap dotfiles apply` of ~/.zlogin
// (offline, its folders in the home folder).
func TestOnAMacADeskCantTakeTheOwnersToolsNamesBesideAPlant(t *testing.T) {
	sandboxExec := sandboxExecOrSkip(t)
	python := python3(t)
	if python == "" {
		t.Skip("no python3 here")
	}
	d, etc := ownersToolsDesk(t)
	h := d.home
	env := []string{"HOME=" + h, "PATH=/usr/bin:/bin", "LANG=C"}
	owner := func(program string, args ...string) { outside(t, h, env, program, args...) }
	// oh-my-zsh's name, made first by the desk: RT126LOCKSIBLINGS' create printed "made".
	if r := d.inside(t, nil, python, "-I", "-c", sbProbe, `[["write", "`+h+`/.zshrc-omztemp"]]`); strings.TrimSpace(r.stdout) != `["PermissionError"]` {
		t.Errorf("the desk's own ~/.zshrc-omztemp: %q %s, want PermissionError at the create", r.stdout, r.stderr)
	}
	// The stand-in: it says "ready", then lists folder until a name matching pattern is there, and, once, holds it
	// (opens it for writing, then closes it) by its name, its upper-case name ("case"), /.vol and its inode ("vol") or
	// a hard link it makes in its worktree ("link"), or renames its own file onto it ("rename"), or swaps them
	// (renamex_np's RENAME_SWAP, "swap").
	const spin = `import ctypes, fnmatch, os, sys, time
folder, pattern, how, until = sys.argv[1], sys.argv[2], sys.argv[3], time.time() + float(sys.argv[4])
own = os.path.join(os.environ["HOME"], "desk-spin")
with open(own, "w") as f:
    f.write("the desk's")
libc = ctypes.CDLL(None, use_errno=True)
links = 0
print("ready", flush=True)
while time.time() < until:
    for name in os.listdir(folder):
        if not fnmatch.fnmatchcase(name, pattern):
            continue
        at = os.path.join(folder, name)
        try:
            if how == "hold":
                os.close(os.open(at, os.O_WRONLY))
            elif how == "case":
                os.close(os.open(os.path.join(folder, name.upper()), os.O_WRONLY))
            elif how == "vol":
                st = os.stat(at)
                os.close(os.open("/.vol/%d/%d" % (st.st_dev, st.st_ino), os.O_WRONLY))
            elif how == "link":
                links += 1
                linked = os.path.join(os.getcwd(), "desk-link-%d" % links)
                os.link(at, linked)
                os.close(os.open(linked, os.O_WRONLY))
            elif how == "swap":
                if libc.renamex_np(own.encode(), at.encode(), 2) != 0:
                    e = ctypes.get_errno()
                    raise OSError(e, os.strerror(e))
            else:
                os.rename(own, at)
        except FileNotFoundError:
            continue
        except PermissionError:
            print("PermissionError", flush=True)
            sys.exit(0)
        print({"hold": "held", "rename": "renamed"}.get(how, how + ": got through"), flush=True)
        sys.exit(0)
print("never there", flush=True)`
	src := filepath.Join(d.base, "zshrc.new")
	mustWrite(t, src, "alias a=1\n")
	type race struct {
		name, folder, pattern, how string
		owner                      func(i int)
	}
	races := []race{
		{"sed -i '' on ~/.zshrc", h, ".!*!.zshrc", "hold", func(i int) {
			owner("/usr/bin/sed", "-i", "", fmt.Sprintf("s/a=[0-9]*/a=%d/", i), h+"/.zshrc")
		}},
		{"sed -i '' on ~/.zshrc, renamed onto", h, ".!*!.zshrc", "rename", func(i int) {
			owner("/usr/bin/sed", "-i", "", fmt.Sprintf("s/a=[0-9]*/a=%d/", i), h+"/.zshrc")
		}},
		// RT126LOCKSIBLINGSR2's other tactics on the same replay.
		{"sed -i '' on ~/.zshrc, by its upper-case name", h, ".!*!.zshrc", "case", func(i int) {
			owner("/usr/bin/sed", "-i", "", fmt.Sprintf("s/a=[0-9]*/a=%d/", i), h+"/.zshrc")
		}},
		{"sed -i '' on ~/.zshrc, by /.vol", h, ".!*!.zshrc", "vol", func(i int) {
			owner("/usr/bin/sed", "-i", "", fmt.Sprintf("s/a=[0-9]*/a=%d/", i), h+"/.zshrc")
		}},
		{"sed -i '' on ~/.zshrc, through a hard link", h, ".!*!.zshrc", "link", func(i int) {
			owner("/usr/bin/sed", "-i", "", fmt.Sprintf("s/a=[0-9]*/a=%d/", i), h+"/.zshrc")
		}},
		{"sed -i '' on ~/.zshrc, swapped", h, ".!*!.zshrc", "swap", func(i int) {
			owner("/usr/bin/sed", "-i", "", fmt.Sprintf("s/a=[0-9]*/a=%d/", i), h+"/.zshrc")
		}},
		{"sed -i '' on ~/.ssh/config", h + "/.ssh", ".!*!config", "hold", func(i int) {
			owner("/usr/bin/sed", "-i", "", fmt.Sprintf("s/Port [0-9]*/Port %d/", i), h+"/.ssh/config")
		}},
		{"sed -i '' on what git's config names", h, ".!*!.gitconfig-work", "hold", func(i int) {
			owner("/usr/bin/sed", "-i", "", fmt.Sprintf("s/user[0-9]*/user%d/", i), h+"/.gitconfig-work")
		}},
		{"sed -i '' on git's other config", filepath.Dir(etc), ".!*!gitconfig", "hold", func(i int) {
			owner("/usr/bin/sed", "-i", "", fmt.Sprintf("s/n = [0-9]*/n = %d/", i), etc)
		}},
		{"oh-my-zsh's two lines", h, ".zshrc-omztemp", "hold", func(i int) {
			owner("/bin/sh", "-c", fmt.Sprintf(`printf 'export ZSH=standin%d\n' > "$HOME/.zshrc-omztemp"; mv -f "$HOME/.zshrc-omztemp" "$HOME/.zshrc"`, i))
		}},
		// Its installer keeps the old ~/.zshrc as .zshrc.pre-oh-my-zsh; its uninstaller moves that back onto ~/.zshrc.
		{"oh-my-zsh's install and uninstall moves", h, ".zshrc.pre-oh-my-zsh", "hold", func(i int) {
			owner("/bin/sh", "-c", fmt.Sprintf(`mv -f "$HOME/.zshrc" "$HOME/.zshrc.pre-oh-my-zsh"; printf 'export ZSH=standin%d\n' > "$HOME/.zshrc-omztemp"; mv -f "$HOME/.zshrc-omztemp" "$HOME/.zshrc"; mv "$HOME/.zshrc" "$HOME/.zshrc.omz-uninstalled-%d"; mv "$HOME/.zshrc.pre-oh-my-zsh" "$HOME/.zshrc"`, i, i))
		}},
		{"oh-my-zsh's two lines, by /.vol", h, ".zshrc-omztemp", "vol", func(i int) {
			owner("/bin/sh", "-c", fmt.Sprintf(`printf 'export ZSH=standin%d\n' > "$HOME/.zshrc-omztemp"; mv -f "$HOME/.zshrc-omztemp" "$HOME/.zshrc"`, i))
		}},
		{"install onto ~/.zshrc", h, "INS@??????", "hold", func(int) { owner("/usr/bin/install", "-m", "644", src, h+"/.zshrc") }},
		{"install -S onto ~/.bashrc, by /.vol", h, "INS@??????", "vol", func(int) { owner("/usr/bin/install", "-S", "-m", "644", src, h+"/.bashrc") }},
		{"install onto ~/.ssh/config", h + "/.ssh", "INS@??????", "hold", func(int) { owner("/usr/bin/install", "-m", "600", src, h+"/.ssh/config") }},
		{"perl -pi on git's other config", filepath.Dir(etc), "XX??????", "hold", func(i int) {
			owner("/usr/bin/perl", "-pi", "-e", fmt.Sprintf("s/n = \\d+/n = %d/", i), etc)
		}},
	}
	if mise := "/opt/homebrew/bin/mise"; func() bool { _, err := os.Stat(mise); return err == nil }() {
		menv := append(slices.Clone(env), "XDG_CONFIG_HOME="+h+"/.config", "XDG_DATA_HOME="+h+"/.local/share",
			"XDG_STATE_HOME="+h+"/.local/state", "XDG_CACHE_HOME="+h+"/.cache", "MISE_OFFLINE=1", "MISE_EXPERIMENTAL=1", "MISE_YES=1")
		outside(t, h, menv, mise, "bootstrap", "dotfiles", "add", h+"/.zlogin")
		races = append(races, race{"mise's dotfiles apply of ~/.zlogin", h, "..zlogin.tmp.*", "rename", func(int) {
			if err := os.Remove(h + "/.zlogin"); err != nil {
				t.Fatal(err)
			}
			outside(t, h, menv, mise, "bootstrap", "dotfiles", "apply", "--force")
		}})
	} else {
		t.Logf("no mise at %s: its race isn't run here", mise)
	}
	for _, c := range races {
		func() {
			cmd := disclaim.Cmd(exec.Command(sandboxExec, "-f", d.profile, python, "-I", "-c", spin, c.folder, c.pattern, c.how, "20"))
			cmd.Dir, cmd.Env = d.root, []string{"HOME=" + h, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
			lines := bufio.NewScanner(stdout)
			if !lines.Scan() || lines.Text() != "ready" {
				t.Fatalf("%s: the stand-in didn't start: %q %s", c.name, lines.Text(), stderr.Bytes())
			}
			said := make(chan string, 1)
			go func() {
				lines.Scan()
				said <- lines.Text()
			}()
			var got string
			runs := 0
			for deadline := time.Now().Add(15 * time.Second); got == "" && time.Now().Before(deadline); runs++ {
				c.owner(runs)
				select {
				case got = <-said:
				default:
				}
			}
			if got == "" {
				select {
				case got = <-said:
				case <-time.After(10 * time.Second):
				}
			}
			if got != "PermissionError" {
				t.Errorf("the desk's %s of %s while the owner's %s ran (%d runs): %q, want PermissionError", c.how, c.pattern, c.name, runs, got)
			} else {
				t.Logf("%s: the desk's %s refused, after %d of the owner's runs", c.name, c.how, runs)
			}
		}()
	}
	if data, _ := os.ReadFile(h + "/.zshrc"); !strings.HasPrefix(string(data), "export ZSH=standin") && !strings.HasPrefix(string(data), "alias a=") {
		t.Errorf("the owner's ~/.zshrc: %q", data)
	}
}

// FXLOCKSIBLINGSR2 (RT126LOCKSIBLINGSR2's B2) under the real profile: a desk's perl -pi of its own files changes them
// where the owner's git config names files in its work, the setup the earlier pins missed. The repo's config has
// `include.path ../.gitconfig` and `core.hooksPath .githooks`, whose pre-commit is a link to ../scripts/pre-commit.sh,
// and ReadGitConfig gives what they name, as a desk's start does: for a hire in its worktree, scripts/pre-commit.sh
// there; for a desk at the project, that and the project's .gitconfig. Its own file directly in ~ changes too
// (SIERRA's ruling). perl -i only warns when it can't make its `XX` name, exits 0 and leaves the file as it was, so the
// row reads each file back. sed -i with an empty suffix and install(1) there work too. What git's config names stays
// the owner's: perl -pi of it leaves it as it was.
func TestOnAMacADesksPerlPiChangesItsOwnFilesBesideWhatGitsConfigNames(t *testing.T) {
	git := gitOrSkip(t)
	sandboxExecOrSkip(t)
	for _, atProject := range []bool{false, true} {
		name := "a hire in its worktree"
		if atProject {
			name = "a desk at the project"
		}
		t.Run(name, func(t *testing.T) {
			d, owner := newGitDesk(t, git)
			if atProject {
				d.root = d.checkout
				d.pl.Root, d.pl.Checkout, d.pl.OwnWorktree = d.checkout, "", ""
			}
			owner(d.checkout, "config", "include.path", "../.gitconfig")
			owner(d.checkout, "config", "core.hooksPath", ".githooks")
			mustWrite(t, filepath.Join(d.checkout, ".gitconfig"), "[alias]\n\tst = 1.0.0\n")
			mine := map[string]string{filepath.Join(d.root, "package.json"): `{"version": "1.0.0"}` + "\n",
				filepath.Join(d.root, "scripts", "build.sh"): "echo 1.0.0\n", filepath.Join(d.root, "sub", "notes.txt"): "1.0.0\n",
				filepath.Join(d.home, "desk-notes"): "1.0.0\n"}
			hook := filepath.Join(d.root, "scripts", "pre-commit.sh")
			for at, text := range mine {
				mustWrite(t, at, text)
			}
			mustWrite(t, hook, "#!/bin/sh\n# 1.0.0\n")
			if err := os.MkdirAll(filepath.Join(d.root, ".githooks"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("../scripts/pre-commit.sh", filepath.Join(d.root, ".githooks", "pre-commit")); err != nil {
				t.Fatal(err)
			}
			hooks, named, err := ReadGitConfig(context.Background(), git, append([]string{"HOME=" + d.home}, gitTestEnv(git)...), d.pl)
			if err != nil {
				t.Fatalf("ReadGitConfig: %v", err)
			}
			theirs := []string{hook}
			if atProject {
				theirs = append(theirs, filepath.Join(d.root, ".gitconfig"))
			}
			for _, at := range theirs {
				if !slices.Contains(named, at) {
					t.Fatalf("ReadGitConfig named %q, not %s: the row's setup isn't the one it means", named, at)
				}
			}
			d.pl.GitHooks, d.pl.GitNamed = hooks, named
			d.prepare(t)
			for at, text := range mine {
				r := d.inside(t, nil, "/usr/bin/perl", "-pi", "-e", `s/1\.0\.0/1.0.1/`, at)
				want := strings.Replace(text, "1.0.0", "1.0.1", 1)
				if data, _ := os.ReadFile(at); r.status != 0 || r.stderr != "" || string(data) != want {
					t.Errorf("the desk's perl -pi of %s: exit %d, %q, the file %q, want %q", strings.TrimPrefix(at, d.base), r.status,
						strings.TrimSpace(r.stderr), data, want)
				}
			}
			// RT126LOCKSIBLINGSR2's other desk work there: sed -i '' (`.!<pid>!<name>`) and install(1) (`INS@` and six).
			pkg := filepath.Join(d.root, "package.json")
			if r := d.inside(t, nil, "/usr/bin/sed", "-i", "", `s/1\.0\.1/1.0.2/`, pkg); r.status != 0 {
				t.Errorf("the desk's sed -i '' of package.json: exit %d, %q", r.status, strings.TrimSpace(r.stderr))
			}
			if data, _ := os.ReadFile(pkg); string(data) != `{"version": "1.0.2"}`+"\n" {
				t.Errorf("the desk's sed -i '' of package.json: the file %q", data)
			}
			for _, to := range []string{filepath.Join(d.root, "scripts", "package.copy"), filepath.Join(d.root, "package.copy"),
				filepath.Join(d.home, "desk-tool")} {
				r := d.inside(t, nil, "/usr/bin/install", "-m", "644", pkg, to)
				if data, _ := os.ReadFile(to); r.status != 0 || string(data) != `{"version": "1.0.2"}`+"\n" {
					t.Errorf("the desk's install onto %s: exit %d, %q, the file %q", strings.TrimPrefix(to, d.base), r.status,
						strings.TrimSpace(r.stderr), data)
				}
			}
			for _, at := range theirs {
				before, _ := os.ReadFile(at)
				d.inside(t, nil, "/usr/bin/perl", "-pi", "-e", `s/1\.0\.0/1.0.1/`, at)
				if data, _ := os.ReadFile(at); string(data) != string(before) {
					t.Errorf("the desk's perl -pi of %s, which git's config names, changed it: %q", strings.TrimPrefix(at, d.base), data)
				}
			}
		})
	}
}

// RT126LOCKSIBLINGSR3's F1, the reviewer's setup: `~/Projects` is a link to `../real`, a desk works at the project
// `real/app` (its Root resolved, as deskPlaces gives it), and the repo's config has `include.path =
// ~/Projects/app/.gitconfig.local`, spelled through the link. ReadGitConfig names that file both ways, and followLinks
// held the names perl -i and install(1) write in its folder where the spelling leads, the desk's root, so a desk's perl
// -pi of its own package.json did nothing. Neither name is a row there now, in the Policy nor under the real profile;
// the named file and the names beside it stay denied. The skip goes no further: git's other config spelled through
// `~/dotlink`, a link to a folder no desk works in (an $XDG_CONFIG_HOME given that way), keeps both names where it
// leads, which only followLinks puts there; so does an include through that link; and an include through a link to the
// home folder keeps install(1)'s there but not perl -i's (SIERRA's ruling on RT126LOCKSIBLINGSR2's B2).
func TestOnAMacADesksPerlPiChangesItsOwnFilesWhenGitsConfigNamesOneThroughALink(t *testing.T) {
	git := gitOrSkip(t)
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	app, dots := filepath.Join(d.base, "real", "app"), filepath.Join(d.base, "dots")
	const named, other = "plants the owner's programs run later: what git's config names", "plants the owner's programs run later: git's other config"
	mine := map[string]string{filepath.Join(app, "package.json"): `{"version": "1.0.0"}` + "\n",
		filepath.Join(app, "sub", "notes.txt"): "1.0.0\n"}
	theirs := map[string]string{filepath.Join(app, ".gitconfig.local"): "[alias]\n\tv = 1.0.0\n",
		filepath.Join(dots, "extra.inc"): "[alias]\n\tw = 1.0.0\n", filepath.Join(d.home, ".gitconfig-home"): "[alias]\n\tx = 1.0.0\n",
		filepath.Join(dots, "git", "config"): "[alias]\n\ty = 1.0.0\n"}
	for _, files := range []map[string]string{mine, theirs} {
		for at, text := range files {
			mustWrite(t, at, text)
		}
	}
	for link, to := range map[string]string{filepath.Join(d.home, "Projects"): "../real", filepath.Join(d.home, "dotlink"): "../dots",
		filepath.Join(d.base, "homelink"): "h"} {
		if err := os.Symlink(to, link); err != nil {
			t.Fatal(err)
		}
	}
	env := append([]string{"HOME=" + d.home}, gitTestEnv(git)...)
	outside(t, app, env, git, "init", "-q", "-b", "main")
	for _, include := range []string{"~/Projects/app/.gitconfig.local", "~/dotlink/extra.inc", filepath.Join(d.base, "homelink", ".gitconfig-home")} {
		outside(t, app, env, git, "config", "--add", "include.path", include)
	}
	d.root = app
	d.pl.Root, d.pl.Checkout, d.pl.OwnWorktree, d.pl.GitDir, d.pl.OtherWorktrees = app, "", "", filepath.Join(app, ".git"), nil
	d.pl.GitConfigs = []string{filepath.Join(d.home, "dotlink", "git", "config")}
	hooks, names, err := ReadGitConfig(context.Background(), git, env, d.pl)
	if err != nil {
		t.Fatalf("ReadGitConfig: %v", err)
	}
	for _, at := range []string{filepath.Join(d.home, "Projects", "app", ".gitconfig.local"), filepath.Join(d.home, "dotlink", "extra.inc"),
		filepath.Join(d.base, "homelink", ".gitconfig-home")} {
		if !slices.Contains(names, at) {
			t.Fatalf("ReadGitConfig named %q, not %s: the row's setup isn't the one it means", names, at)
		}
	}
	d.pl.GitHooks, d.pl.GitNamed = hooks, names

	p := mustNew(t, d.pl)
	for _, r := range p.Rules() {
		if slices.Contains(ownersInFolder, filepath.Base(r.Tail)) && under(key("darwin", r.Path), key("darwin", app)) {
			t.Errorf("a row of an owner's tool's name in the desk's root: %+v", r)
		}
	}
	profile := profileOf(p, p.Exception)
	for _, c := range []struct {
		at  string
		v   Verdict
		row string
	}{
		{app + "/XXNu6Xnm", A, ""}, {app + "/INS@q4Ontc", A, ""}, {app + "/sub/XXNu6Xnm", A, ""},
		{app + "/.gitconfig.local", D, named}, {app + "/.!9!.gitconfig.local", D, named}, {app + "/.gitconfig.local.lock", D, named},
		{dots + "/extra.inc", D, named}, {dots + "/XXNu6Xnm", D, named}, {dots + "/INS@q4Ontc", D, named},
		{dots + "/git/config", D, other}, {dots + "/git/XXNu6Xnm", D, other}, {dots + "/git/INS@q4Ontc", D, other},
		{d.home + "/.gitconfig-home", D, named}, {d.home + "/XXNu6Xnm", A, ""}, {d.home + "/INS@q4Ontc", D, ""},
	} {
		if v, r := p.Decide(c.at, Write); v != c.v || c.row != "" && r.Row != c.row {
			t.Errorf("write %s: %v by %q, want %v by %q", strings.TrimPrefix(c.at, d.base), v, r.Row, c.v, c.row)
		}
		if got := profile.says(c.at, Write); got != c.v {
			t.Errorf("write %s: the profile says %v, want %v", strings.TrimPrefix(c.at, d.base), got, c.v)
		}
	}

	d.prepare(t)
	for at, text := range mine {
		r := d.inside(t, nil, "/usr/bin/perl", "-pi", "-e", `s/1\.0\.0/1.0.1/`, at)
		want := strings.Replace(text, "1.0.0", "1.0.1", 1)
		if data, _ := os.ReadFile(at); r.status != 0 || r.stderr != "" || string(data) != want {
			t.Errorf("the desk's perl -pi of %s: exit %d, %q, the file %q, want %q", strings.TrimPrefix(at, d.base), r.status,
				strings.TrimSpace(r.stderr), data, want)
		}
	}
	for at := range theirs {
		before, _ := os.ReadFile(at)
		d.inside(t, nil, "/usr/bin/perl", "-pi", "-e", `s/1\.0\.0/1.0.1/`, at)
		if data, _ := os.ReadFile(at); string(data) != string(before) {
			t.Errorf("the desk's perl -pi of %s, which git's config names, changed it: %q", strings.TrimPrefix(at, d.base), data)
		}
	}
	// perl -i's name where the folders no desk works in are: still the owner's, under the real profile too.
	for _, at := range []string{filepath.Join(dots, "XXabcdef"), filepath.Join(dots, "git", "XXabcdef")} {
		if r := d.inside(t, nil, "/usr/bin/touch", at); r.status == 0 {
			t.Errorf("the desk's touch of %s: exit 0, want refused", strings.TrimPrefix(at, d.base))
		}
		if _, err := os.Lstat(at); !os.IsNotExist(err) {
			t.Errorf("%s was made (%v)", strings.TrimPrefix(at, d.base), err)
		}
	}
}
