//go:build !windows

package desktop

import (
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// FXGITSTATECFGR1 (RT126GITSTATECFG B3-1, B3-2, B3-3; SIERRA's fail-closed ruling): every git the core runs as the
// owner on a repo a desk may write runs only on config the core wrote, or isn't run at all. Stand-ins only: throwaway
// repos in the tests' own temp folders, "programs" that only leave their name in a marker file there, and, for the
// check-then-run gap, a stand-in git first on PATH that runs the real one and, right after the core's look at the
// config (`config --list`), has the repo's config made hostile: the desk's move, at exactly that point.

// deskMoves is a stand-in desk: the hostile config it writes into a repo's git folder, and the marker its programs
// leave.
type deskMoves struct {
	base, marker, git string
	knownOnly         bool // hostile writes only keys a read knows (no config-based hook)
}

func newDeskMoves(t *testing.T) *deskMoves {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git here")
	}
	base := physical(t.TempDir())
	return &deskMoves{base: base, marker: filepath.Join(base, "ran"), git: git}
}

// program is a stand-in that leaves its name in the marker; a filter one passes its input through.
func (m *deskMoves) program(t *testing.T, name string, filter bool) string {
	t.Helper()
	script := filepath.Join(m.base, name+".sh")
	body := "#!/bin/sh\necho " + name + " >> '" + m.marker + "'\n"
	if filter {
		body += "cat\n"
	}
	if err := os.WriteFile(script, []byte(body+"exit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

// hostile is the shell script that makes the repo whose git folder is common hostile: a filter for every file (from
// .git/info/attributes, which no attribute source replaces), an fsmonitor, hooks in a hooks folder it names, and
// config-based hooks (hook.<name>.command), which core.hooksPath doesn't switch off, for every event a status, a
// checkout, a commit or a branch's change fires.
func (m *deskMoves) hostile(t *testing.T, common string) string {
	t.Helper()
	hooks := filepath.Join(m.base, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	events := []string{"post-checkout", "reference-transaction", "post-index-change", "pre-commit", "post-commit", "fsmonitor-watchman"}
	for _, hook := range events {
		hook := filepath.Join(hooks, hook)
		if err := os.WriteFile(hook, []byte("#!/bin/sh\necho hookfolder >> '"+m.marker+"'\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	config := filepath.Join(common, "config")
	cfg := func(key, value string) string {
		return "'" + m.git + "' config --file '" + config + "' --add '" + key + "' '" + value + "'\n"
	}
	script := cfg("filter.z.clean", m.program(t, "clean", true)) + cfg("filter.z.smudge", m.program(t, "smudge", true)) +
		cfg("core.fsmonitor", m.program(t, "fsmonitor", false)) + cfg("core.hooksPath", hooks)
	if !m.knownOnly {
		script += cfg("hook.desk.command", m.program(t, "confighook", false))
		for _, event := range events {
			script += cfg("hook.desk.event", event)
		}
	}
	script += "mkdir -p '" + filepath.Join(common, "info") + "'\necho '* filter=z' > '" + filepath.Join(common, "info", "attributes") + "'\n"
	path := filepath.Join(m.base, "hostile-"+filepath.Base(filepath.Dir(common))+".sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// now makes the repo hostile before the core does anything.
func (m *deskMoves) now(t *testing.T, common string) {
	t.Helper()
	if out, err := exec.Command("/bin/sh", m.hostile(t, common)).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

// afterTheCheck puts a stand-in git first on PATH: the real one runs, and right after the first run that lists the
// config (the core's look at it), script runs once.
func (m *deskMoves) afterTheCheck(t *testing.T, script string) {
	t.Helper()
	m.afterARunOf(t, "--list", script)
}

// afterARunOf is afterTheCheck after the first run whose arguments hold words instead.
func (m *deskMoves) afterARunOf(t *testing.T, words, script string) {
	t.Helper()
	bin := filepath.Join(m.base, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	done := filepath.Join(m.base, "flipped")
	wrapper := "#!/bin/sh\n'" + m.git + "' \"$@\"\ncode=$?\ncase \" $* \" in *\" " + words + " \"*)\n  if [ ! -e '" + done + "' ]; then : > '" + done +
		"'; /bin/sh '" + script + "'; fi;;\nesac\nexit $code\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func (m *deskMoves) flipped() bool {
	_, err := os.Stat(filepath.Join(m.base, "flipped"))
	return err == nil
}

// ran is what the stand-in programs left, and clears it.
func (m *deskMoves) ran(t *testing.T) string {
	t.Helper()
	data, _ := os.ReadFile(m.marker)
	os.Remove(m.marker)
	return strings.Join(strings.Fields(string(data)), " ")
}

// realGit runs the real git in dir, with no config of the repo's able to run anything of the stand-ins: the tests'
// own look.
func realGit(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir, "--no-optional-locks", "-c", "core.fsmonitor=", "-c", "core.hooksPath=" + os.DevNull,
		"-c", "filter.z.clean=", "-c", "filter.z.smudge=", "-c", "hook.desk.command=true"}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// B3-1: a desk with no sandbox (the sandbox off, a desk from before the update, every desk off a Mac): the fire's look
// at its tree is a status the core runs outside. The config is clean when the core looks, hostile when status runs.
func TestAnUnsandboxedDesksStatusRunsNothingOfAConfigChangedAfterItsCheck(t *testing.T) {
	r := newDeskRunRig(t)
	m := newDeskMoves(t)
	// A tracked file changed but the same size, so status must read it (through the filter) to know.
	writeRepoFile(t, r.tree, "a.txt", "two\n")
	m.afterTheCheck(t, m.hostile(t, filepath.Join(r.repo, ".git")))
	message := uncommittedRefusal(r.hire, runGit)
	if got := m.ran(t); got != "" {
		t.Errorf("a config changed after the core's check ran as the owner: %s", got)
	}
	if !m.flipped() {
		t.Fatal("the stand-in desk never changed the config: this row proves nothing")
	}
	if !strings.Contains(message, "a.txt") {
		t.Errorf("the fire's look: %q, want a.txt listed", message)
	}
	if plainGit(r.tree, "status", "--porcelain"); m.ran(t) == "" {
		t.Error("the control: a plain status ran nothing of the hostile config, so this row proves nothing")
	}
}

// B3-2, prepareHire: a hire's worktree is made from a repo whose config names a smudge filter for every file, an
// fsmonitor, hooks and config-based hooks for post-checkout and reference-transaction. None runs, and the worktree,
// its branch and its registration are there, as git would have made them.
func TestAHiresWorktreeIsMadeWithNothingOfTheReposConfigRun(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	m := newDeskMoves(t)
	writeRepoFile(t, repo, ".gitattributes", "* filter=z\n")
	gitRepoCmd(t, repo, "add", ".gitattributes")
	gitRepoCmd(t, repo, "commit", "-q", "-m", "attributes")
	m.now(t, filepath.Join(repo, ".git"))
	o := placeOptions(t)
	place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "x", Folder: repo, Worktree: true}, o)
	if got := m.ran(t); got != "" {
		t.Errorf("making the hire's worktree ran as the owner: %s", got)
	}
	if err != nil {
		t.Fatalf("the hire: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(place.Worktree, "sub", "a.txt")); err != nil || string(data) != "a" {
		t.Errorf("the worktree's file: %q %v", data, err)
	}
	if out, err := realGit(repo, "rev-parse", "--verify", "-q", "refs/heads/"+place.Branch); err != nil || out == "" {
		t.Errorf("the hire's branch: %q %v", out, err)
	}
	if out, err := realGit(place.Worktree, "rev-parse", "--abbrev-ref", "HEAD"); err != nil || out != place.Branch {
		t.Errorf("the worktree is on %q (%v), want %s", out, err, place.Branch)
	}
	if out, err := realGit(place.Worktree, "status", "--porcelain"); err != nil || out != "" {
		t.Errorf("the new worktree isn't clean: %q %v", out, err)
	}
	if out, _ := realGit(repo, "worktree", "list", "--porcelain"); !strings.Contains(out, "worktree "+place.Worktree+"\n") {
		t.Errorf("the repo doesn't list the worktree:\n%s", out)
	}
	if plainGit(repo, "worktree", "add", "-q", "-b", "control", filepath.Join(t.TempDir(), "control")); m.ran(t) == "" {
		t.Error("the control: a plain worktree add ran nothing, so this row proves nothing")
	}
}

// B3-2, discardHalfMade: the branch of a worktree add that was stopped is deleted with nothing of the config run.
func TestAHalfMadeHiresBranchIsDeletedWithNothingOfTheReposConfigRun(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	m := newDeskMoves(t)
	gitRepoCmd(t, repo, "branch", "hire/none")
	m.now(t, filepath.Join(repo, ".git"))
	parent := filepath.Join(t.TempDir(), "parent")
	inside := filepath.Join(parent, "x-abc")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	discardHalfMade(repo, parent, inside, "hire/none", runGit)
	if got := m.ran(t); got != "" {
		t.Errorf("clearing a half-made hire ran as the owner: %s", got)
	}
	if out, err := realGit(repo, "rev-parse", "--verify", "-q", "refs/heads/hire/none"); err == nil {
		t.Errorf("the half-made hire's branch is still there: %s", out)
	}
	if _, err := os.Stat(inside); err == nil {
		t.Error("the half-made worktree's folder is still there")
	}
}

// B3-2, removeWorkspace before the registry has the hire's row (the undo of a hire that failed): its look, its
// removal and its branch's deletion run nothing of the config. A file touched but not changed must be read to be
// known clean: a plain status would run the filter on it. With a key a read doesn't know (a config-based hook), the
// look is refused and the worktree kept (fail closed); with keys it knows only, the worktree is removed.
func TestAWorktreeWithNoRowYetIsRemovedWithNothingOfTheReposConfigRun(t *testing.T) {
	for _, knownOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "a key a read doesn't know", true: "keys a read knows"}[knownOnly], func(t *testing.T) {
			isolate(t)
			repo := placeRepo(t)
			m := newDeskMoves(t)
			m.knownOnly = knownOnly
			path := filepath.Join(physical(t.TempDir()), "w-x")
			gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/w-x", path)
			later := time.Now().Add(time.Hour)
			if err := os.Chtimes(filepath.Join(path, "sub", "a.txt"), later, later); err != nil {
				t.Fatal(err)
			}
			keepRegistry(t)
			writeRegistry(t)
			m.now(t, filepath.Join(repo, ".git"))
			removed := removeWorkspace(repo, path, "hire/w-x", runGit)
			if got := m.ran(t); got != "" {
				t.Errorf("removing a worktree with no row ran as the owner: %s", got)
			}
			_, err := os.Stat(path)
			_, branchErr := realGit(repo, "rev-parse", "--verify", "-q", "refs/heads/hire/w-x")
			if gone := removed && err != nil && branchErr != nil; gone != knownOnly || !gone && (removed || err != nil || branchErr != nil) {
				t.Errorf("removed %v, the worktree's stat %v, the branch's look %v: want all gone = %v", removed, err, branchErr, knownOnly)
			}
		})
	}
}

// B3-3: the registry is what says a tree is a desk's; one that can't be read, or has no row for it, says nothing
// either way, so the tree is taken for a desk's: the fire's look still runs nothing of its config (hostile from the
// start, or changed after the core's look) and never goes into a submodule the desk checked out, whose own config
// has an fsmonitor.
func TestARegistryThatCantBeReadOrHasNoRowStillRunsNothingOfTheTreesConfig(t *testing.T) {
	for _, registry := range []string{"no row", "unreadable"} {
		for _, when := range []string{"from the start", "after the check"} {
			t.Run(registry+", "+when, func(t *testing.T) {
				r := newDeskRunRig(t)
				m := newDeskMoves(t)
				sub := filepath.Join(r.tree, "sub2")
				gitRepoCmd(t, r.tree, "init", "-q", "-b", "main", sub)
				gitRepoCmd(t, sub, "config", "commit.gpgsign", "false")
				writeRepoFile(t, sub, "s.txt", "s\n")
				gitRepoCmd(t, sub, "add", "s.txt")
				gitRepoCmd(t, sub, "commit", "-q", "-m", "s")
				gitRepoCmd(t, r.tree, "add", "sub2")
				gitRepoCmd(t, r.tree, "commit", "-q", "-m", "a checked-out repo")
				gitRepoCmd(t, sub, "config", "core.fsmonitor", m.program(t, "submonitor", false))
				writeRepoFile(t, sub, "s.txt", "changed in the submodule\n")
				writeRepoFile(t, r.tree, "a.txt", "changed, and longer\n")
				if when == "from the start" {
					m.now(t, filepath.Join(r.repo, ".git"))
				} else {
					m.afterTheCheck(t, m.hostile(t, filepath.Join(r.repo, ".git")))
				}
				if registry == "no row" {
					writeRegistry(t, Hire{ID: "fedcba654321", Name: "bo", Role: "builder", Folder: keptPath(r.repo), Repo: r.repo,
						Worktree: keptPath(filepath.Join(r.base, "elsewhere")), Branch: "hire/bo", State: "working"})
				} else if err := writeRegistryFile(registryPath(), []byte("{not a registry"), 0o600); err != nil {
					t.Fatal(err)
				}
				message := uncommittedRefusal(r.hire, runGit)
				if got := m.ran(t); got != "" {
					t.Errorf("the fire's look ran as the owner: %s (%q)", got, message)
				}
				if when == "after the check" && !strings.Contains(message, "a.txt") {
					t.Errorf("the fire's look: %q, want a.txt listed", message)
				}
				if message == "" {
					t.Error("a tree with changes was taken for a clean one")
				}
			})
		}
	}
}

// Fail closed (SIERRA): a command the core has no form of on config of its own is refused, never run plain. A
// checkout would run post-checkout and reference-transaction hooks; a prune isn't the core's to run.
func TestTheOwnersGitRefusesWhatItCantRunOnConfigOfItsOwn(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	m := newDeskMoves(t)
	m.now(t, filepath.Join(repo, ".git"))
	for _, args := range [][]string{{"checkout", "-q", "-b", "other"}, {"worktree", "prune"}, {"config", "--local", "user.name", "x"},
		{"symbolic-ref", "HEAD", "refs/heads/other"}} {
		if status, out, ok := runGit(repo, args...); ok {
			t.Errorf("%v was run: %d %q", args, status, out)
		}
	}
	if got := m.ran(t); got != "" {
		t.Errorf("a refused command ran as the owner: %s", got)
	}
	if out, err := realGit(repo, "rev-parse", "--verify", "-q", "refs/heads/other"); err == nil {
		t.Errorf("the refused checkout made its branch: %s", out)
	}
}

// Fail closed, a desk's own tree outside a sandbox: a command with no form on config of the core's (a checkout,
// which fires reference-transaction) is refused there too, before any look at the config.
func TestAnUnsandboxedDesksGitRefusesWhatItCantRunOnConfigOfItsOwn(t *testing.T) {
	r := newDeskRunRig(t)
	m := newDeskMoves(t)
	m.afterTheCheck(t, m.hostile(t, filepath.Join(r.repo, ".git")))
	if status, out, ok := runGit(r.tree, "checkout", "-q", "-b", "other"); ok {
		t.Errorf("a checkout on a desk's tree was run: %d %q", status, out)
	}
	if got := m.ran(t); got != "" {
		t.Errorf("a refused command ran as the owner: %s", got)
	}
}

// Fail closed: the core carries a branch's deletion into the repo as a file it removes. A branch git keeps packed is
// in a file the core doesn't rewrite: refused, and the branch is kept. That holds for one packed only, and for one
// packed at an older commit with its loose file newer: removing the loose file alone would bring the packed one back.
func TestAPackedBranchIsKeptRatherThanDeletedByTheCore(t *testing.T) {
	for _, loose := range []bool{false, true} {
		t.Run(map[bool]string{false: "packed only", true: "packed, and loose at a newer commit"}[loose], func(t *testing.T) {
			isolate(t)
			repo := placeRepo(t)
			gitRepoCmd(t, repo, "branch", "hire/p")
			gitRepoCmd(t, repo, "pack-refs", "--all")
			if loose {
				writeRepoFile(t, repo, "b.txt", "b\n")
				gitRepoCmd(t, repo, "add", "b.txt")
				gitRepoCmd(t, repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "two")
				gitRepoCmd(t, repo, "update-ref", "refs/heads/hire/p", "HEAD")
			}
			want, _ := realGit(repo, "rev-parse", "--verify", "-q", "refs/heads/hire/p")
			if status, out, ok := runGit(repo, "branch", "-d", "hire/p"); ok && status == 0 {
				t.Errorf("a packed branch was deleted: %q", out)
			}
			if out, err := realGit(repo, "rev-parse", "--verify", "-q", "refs/heads/hire/p"); err != nil || out != want {
				t.Errorf("the packed branch went, or back to its packed commit: %q, was %q", out, want)
			}
		})
	}
}

// Fail closed: a branch that moved between the core's git and the core's carrying of its change is left as it is now.
func TestABranchThatMovedMeanwhileIsntDeletedByTheCore(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	m := newDeskMoves(t)
	gitRepoCmd(t, repo, "branch", "hire/m")
	gitRepoCmd(t, repo, "commit", "-q", "--allow-empty", "-m", "later")
	later, _ := realGit(repo, "rev-parse", "HEAD")
	// The stand-in desk moves the branch right after the core's git deleted it, before the core carries the deletion.
	move := filepath.Join(m.base, "move.sh")
	if err := os.WriteFile(move, []byte("unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY\n'"+m.git+"' -C '"+repo+"' update-ref refs/heads/hire/m "+later+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.afterARunOf(t, "branch -d", move)
	if status, out, ok := runGit(repo, "branch", "-d", "hire/m"); ok && status == 0 {
		t.Errorf("a branch that moved meanwhile was deleted: %q", out)
	}
	if !m.flipped() {
		t.Fatal("the branch never moved: this row proves nothing")
	}
	if out, err := realGit(repo, "rev-parse", "--verify", "-q", "refs/heads/hire/m"); err != nil || out != later {
		t.Errorf("the branch is at %q (%v), want where the desk moved it, %s", out, err, later)
	}
}

// Fail closed: a repo whose refs aren't files (reftable) gets no worktree from the core.
func TestAReftableRepoGetsNoWorktreeFromTheCore(t *testing.T) {
	isolate(t)
	if out, err := exec.Command("git", "version").Output(); err != nil || !gitVersionAtLeast(string(out), 2, 45) {
		t.Skip("no reftable in this git")
	}
	// Refused by what the repo's config says, not by the layout git happens to leave: also with the file git keeps
	// at refs/heads (where a files repo has its branches' folder) gone.
	for _, stub := range []bool{true, false} {
		t.Run(map[bool]string{true: "as git made it", false: "with no refs/heads file"}[stub], func(t *testing.T) {
			isolate(t)
			repo := physical(t.TempDir())
			gitRepoCmd(t, repo, "init", "-q", "-b", "main", "--ref-format=reftable")
			stopSigning(t, repo)
			writeRepoFile(t, repo, "a.txt", "a")
			gitRepoCmd(t, repo, "add", "a.txt")
			gitRepoCmd(t, repo, "commit", "-q", "-m", "one")
			if !stub {
				if err := os.Remove(filepath.Join(repo, ".git", "refs", "heads")); err != nil {
					t.Fatal(err)
				}
			}
			o := placeOptions(t)
			if place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "x", Folder: repo, Worktree: true}, o); err == nil {
				t.Errorf("a reftable repo got a worktree: %+v", place)
			}
			if out, _ := realGit(repo, "branch", "--list", "hire/*"); out != "" {
				t.Errorf("a branch was left: %q", out)
			}
			if _, err := os.Lstat(filepath.Join(repo, ".git", "refs", "heads", "hire")); !stub && err == nil {
				t.Errorf("a branch was written as a file into a reftable repo")
			}
		})
	}
}

// Fail closed: with no git folder of its own to read through (the support folder's git-reads can't be made), the core
// reads no working file, rather than read it on the repo's config.
func TestNoGitFolderOfTheCoresOwnMeansNoRead(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	if err := os.WriteFile(gitReadsFolder(), []byte("not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	if status, out, ok := runGit(repo, "status", "--porcelain"); ok {
		t.Errorf("a status was run with no git folder of the core's: %d %q", status, out)
	}
	if _, _, ok := runGit(repo, "rev-parse", "--show-toplevel"); !ok {
		t.Error("rev-parse, which reads no working file, needs no git folder of the core's")
	}
}

// Fail closed: a working file is read only on a config whose every key the read knows (gitRepoConfig), as the git
// state's read: a key it doesn't know refuses the read.
func TestTheOwnersGitReadsNoFileOfARepoWithAKeyItDoesntKnow(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	gitRepoCmd(t, repo, "config", "smolcorp.unknown", "x")
	if status, out, ok := runGit(repo, "status", "--porcelain"); ok {
		t.Errorf("a repo with a key the read doesn't know was read: %d %q", status, out)
	}
}

// The demo's stand-in hires (its dry run): a worktree made, its work added, committed and diffed with nothing of the
// repo's config run, whether it was hostile from the start or turned hostile after the core's look at it. The commit
// is the owner's, by the name and address in their own (stand-in) global config, which the core's folder doesn't
// read: never one git makes up from the computer's name, or none at all ("Author identity unknown").
func TestTheDemosStandInWorkRunsNothingOfTheReposConfig(t *testing.T) {
	for _, when := range []string{"from the start", "after the check"} {
		t.Run(when, func(t *testing.T) {
			isolate(t)
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			writeRepoFile(t, home, ".gitconfig", "[user]\n\tname = \"Stand \\\"In\\\" Owner\"\n\temail = stand-in@example.invalid\n")
			repo := placeRepo(t)
			m := newDeskMoves(t)
			tree := filepath.Join(physical(t.TempDir()), "bughunter-1")
			if _, err := demoGit(repo, "worktree", "add", "-q", "-b", "hire/bughunter-1", tree); err != nil {
				t.Fatal(err)
			}
			writeRepoFile(t, tree, "fix.txt", "fixed\n")
			if when == "from the start" {
				m.now(t, filepath.Join(repo, ".git"))
			}
			for _, step := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "a fix"}} {
				if _, err := demoGit(tree, step...); err != nil {
					t.Fatal(err)
				}
			}
			// After the check: at the diff's look at the config. A config hostile from the start has keys a read
			// doesn't know: the diff is refused rather than read.
			if when == "after the check" {
				m.afterTheCheck(t, m.hostile(t, filepath.Join(repo, ".git")))
			}
			stat, err := demoGit(tree, "diff", "--shortstat", "main...HEAD")
			if got := m.ran(t); got != "" {
				t.Errorf("the demo's work ran as the owner: %s", got)
			}
			if when == "after the check" && (err != nil || !strings.Contains(stat, "1 file changed")) {
				t.Errorf("the stand-in's diff: %q %v", stat, err)
			}
			if out, err := realGit(repo, "log", "-1", "--format=%s|%an <%ae>", "hire/bughunter-1"); err != nil || out != `a fix|Stand "In" Owner <stand-in@example.invalid>` {
				t.Errorf("the stand-in's branch: %q %v", out, err)
			}
		})
	}
}

// B4-1 (RT126GITSTATECFG P4): a keyed desk's headless run reports with the desk's key from a headless tab, which
// no desk's tab names. The session keeps the desk that reported it, and is held to that desk's worktree.
func TestAKeyedDesksHeadlessSessionIsHeldToItsWorktree(t *testing.T) {
	d, keys, sockets := keyedOffice(t)
	outside := aGitRepo(t)
	worktree := filepath.Join(filepath.Dir(sockets["aaa111"]), "wt-aaa111")
	gitRepoCmd(t, filepath.Dir(worktree), "init", "-q", "-b", "main", worktree)
	for _, c := range []struct {
		session, folder string
		read            bool
	}{{"s-headless-out", outside, false}, {"s-headless-in", worktree, true}} {
		if code := hookAs(t, d, keys["aaa111"], "headless:rev1-"+c.session, "UserPromptSubmit", c.session, c.folder); code != http.StatusNoContent {
			t.Fatalf("%s: the hook: %d", c.session, code)
		}
		code, answer := d.deskGit(c.session)
		state := gitAnswer(t, answer)
		if code != http.StatusOK || state.Repo != c.read || state.Skipped == c.read {
			t.Errorf("%s (%s): %d %+v", c.session, c.folder, code, answer)
		}
		if !c.read && state.Note != "This desk names a folder outside its own worktree, so its git state isn't read." {
			t.Errorf("%s: %q", c.session, state.Note)
		}
	}
	// The owner's key on the same headless session afterwards doesn't free it.
	if code := hookAs(t, d, d.access.HookKey, "headless:rev1-s-headless-out", "UserPromptSubmit", "s-headless-out", outside); code != http.StatusNoContent {
		t.Fatalf("the owner's hook: %d", code)
	}
	if _, answer := d.deskGit("s-headless-out"); !gitAnswer(t, answer).Skipped {
		t.Errorf("the desk's headless session was read after an event with the owner's key: %+v", answer)
	}
}

// N4-1: the folder is resolved once, and what git reads is held to it: a desk that swaps its folder for a link out of
// its worktree after the core's look (here, just as git starts) gets nothing read.
func TestADeskSwappingItsFolderForALinkAfterTheLookIsntRead(t *testing.T) {
	d, keys, sockets := keyedOffice(t)
	m := newDeskMoves(t)
	outside := aGitRepo(t)
	worktree := filepath.Join(filepath.Dir(sockets["aaa111"]), "wt-aaa111")
	gitRepoCmd(t, filepath.Dir(worktree), "init", "-q", "-b", "main", worktree)
	nested := filepath.Join(worktree, "x")
	gitRepoCmd(t, worktree, "init", "-q", "-b", "main", nested)
	if code := hookAs(t, d, keys["aaa111"], tmuxTabOn(sockets["aaa111"]), "UserPromptSubmit", "s-swap", nested); code != http.StatusNoContent {
		t.Fatalf("the hook: %d", code)
	}
	// The stand-in git swaps the folder as the first git starts: after the core's look at the path.
	bin := filepath.Join(m.base, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	done := filepath.Join(m.base, "swapped")
	wrapper := "#!/bin/sh\nif [ ! -e '" + done + "' ]; then : > '" + done + "'; mv '" + nested + "' '" + nested + ".was' && ln -s '" + outside + "' '" + nested + "'; fi\nexec '" + m.git + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	code, answer := d.deskGit("s-swap")
	state := gitAnswer(t, answer)
	if _, err := os.Stat(done); err != nil {
		t.Fatal("the folder was never swapped: this row proves nothing")
	}
	if code != http.StatusOK || !state.Skipped || state.Note != "This desk names a folder outside its own worktree, so its git state isn't read." {
		t.Errorf("a folder swapped for a link out of the worktree was read: %d %+v", code, answer)
	}
}

// Nothing loosened: git deletes no branch a worktree is on, and the core, which deletes it in a folder that has no
// worktrees, checks the repo's own HEADs for it.
func TestABranchAWorktreeIsOnIsntDeletedByTheCore(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/on", filepath.Join(physical(t.TempDir()), "on"))
	if status, out, ok := runGit(repo, "branch", "-D", "hire/on"); ok && status == 0 {
		t.Errorf("a branch a worktree is on was deleted: %q", out)
	}
	if out, err := realGit(repo, "rev-parse", "--verify", "-q", "refs/heads/hire/on"); err != nil {
		t.Errorf("the branch went: %s", out)
	}
}

// Fail closed: a git older than 2.32 can't be given the core's config in its environment, nor have the owner's left
// out: it reads no working file and writes nothing in a repo.
func TestAGitTooOldForConfigOfTheCoresOwnRunsNothingButReads(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	gitRepoCmd(t, repo, "branch", "hire/old")
	saved := gitHere
	gitHere = func() gitHereInfo { return gitHereInfo{version: "git version 2.31.0"} }
	t.Cleanup(func() { gitHere = saved })
	for _, args := range [][]string{{"status", "--porcelain"}, {"branch", "-d", "hire/old"},
		{"worktree", "add", "-b", "hire/x", filepath.Join(t.TempDir(), "x")}} {
		if status, out, ok := runGit(repo, args...); ok {
			t.Errorf("%v was run on a git too old: %d %q", args, status, out)
		}
	}
	if out, err := realGit(repo, "rev-parse", "--verify", "-q", "refs/heads/hire/old"); err != nil {
		t.Errorf("the branch went: %s", out)
	}
}

// Fail closed: a worktree is made only in the one form the core carries (`worktree add [-q] -b <branch> <path>`), and
// only under a registration name that is free: one taken (a registration left behind) refuses it, with nothing left.
func TestAWorktreeIsMadeOnlyInTheFormAndNameTheCoreCarries(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	for _, args := range [][]string{{"worktree", "add", filepath.Join(t.TempDir(), "a")}, {"worktree", "add", "--detach", "-b", "hire/y", filepath.Join(t.TempDir(), "b")},
		{"worktree", "add", "-b", "hire/y", "relative"}} {
		if status, out, ok := runGit(repo, args...); ok {
			t.Errorf("%v was run: %d %q", args, status, out)
		}
	}
	o := placeOptions(t)
	left := filepath.Join(repo, ".git", "worktrees", "x-abcdef")
	if err := os.MkdirAll(left, 0o755); err != nil {
		t.Fatal(err)
	}
	if place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "x", Folder: repo, Worktree: true}, o); err == nil {
		t.Errorf("a worktree was made under a registration name already taken: %+v", place)
	}
	if out, _ := realGit(repo, "branch", "--list", "hire/*"); out != "" {
		t.Errorf("a branch was left: %q", out)
	}
	if entries, _ := os.ReadDir(left); len(entries) != 0 {
		t.Errorf("the registration left behind was written into: %v", entries)
	}
}

// RT126GITSTATECFG's note, a split index: git finds the shared index beside the index file the read names, so a repo
// with core.splitIndex reads as any other through the core's folder.
func TestARepoWithASplitIndexIsReadThroughTheCoresFolder(t *testing.T) {
	isolate(t)
	dir := aGitRepo(t)
	gitRepoCmd(t, dir, "config", "core.splitIndex", "true")
	gitRepoCmd(t, dir, "update-index", "--split-index")
	writeRepoFile(t, dir, "new.txt", "new\n")
	gitRepoCmd(t, dir, "add", "new.txt")
	writeRepoFile(t, dir, "a.txt", "changed\n")
	if matches, _ := filepath.Glob(filepath.Join(dir, ".git", "sharedindex.*")); len(matches) == 0 {
		t.Fatal("no shared index: this row proves nothing")
	}
	state := realGitState(dir)
	if !state.Repo || state.Skipped || state.Changed != 2 {
		t.Errorf("a repo with a split index: %+v", state)
	}
}

// RT126GITSTATECFG's note, a crash mid-read: what a read left in git-reads an hour ago or more is cleared at the
// first read of the next run; a folder newer than that (another read's, now) is kept.
func TestWhatACrashedReadLeftIsClearedOnce(t *testing.T) {
	isolate(t)
	old, recent := filepath.Join(gitReadsFolder(), "read-old"), filepath.Join(gitReadsFolder(), "read-recent")
	for _, folder := range []string{old, recent} {
		if err := os.MkdirAll(filepath.Join(folder, "hooks"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	long := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(old, long, long); err != nil {
		t.Fatal(err)
	}
	if state := realGitState(aGitRepo(t)); !state.Repo || state.Skipped {
		t.Fatalf("%+v", state)
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("a read's folder left by a crash is still there")
	}
	if _, err := os.Stat(recent); err != nil {
		t.Errorf("a recent read's folder was removed: %v", err)
	}
	if entries, _ := os.ReadDir(gitReadsFolder()); len(entries) != 1 {
		t.Errorf("the read left its own folder behind: %d entries", len(entries))
	}
}

// SA-X7 (ownergit.go) with stand-ins only: real throwaway repos in the test's own temp folder (signing off in their
// local config only), a bare repo in the same folder as their remote (no network), and stand-in "programs" a repo's
// config names, shell scripts that only leave a marker file. Each plant has its control: the same `worktree add` run
// as the core ran it before SA-X7, which runs the stand-in.

// ownerRig is an owner's repo with a commit, and a folder for its plants.
type ownerRig struct {
	base, repo, marker string
}

func newOwnerRig(t *testing.T) *ownerRig {
	t.Helper()
	base := physical(t.TempDir())
	r := &ownerRig{base: base, repo: filepath.Join(base, "proj"), marker: filepath.Join(base, "ran")}
	if err := os.MkdirAll(r.repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRepoCmd(t, r.repo, "init", "-q", "-b", "main")
	stopSigning(t, r.repo)
	writeRepoFile(t, r.repo, "a.txt", "one\n")
	gitRepoCmd(t, r.repo, "add", "a.txt")
	gitRepoCmd(t, r.repo, "commit", "-q", "-m", "first")
	return r
}

// standIn is a script that leaves the marker with its name: what a planted setting would have run.
func (r *ownerRig) standIn(t *testing.T, name string) string {
	t.Helper()
	script := filepath.Join(r.base, name+".sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho "+name+" >> '"+r.marker+"'\ncat\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func (r *ownerRig) ran(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(r.marker)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(r.marker)
	return strings.TrimSpace(string(data))
}

// hire is prepareHire for a worktree of the rig's repo, with the real git.
func (r *ownerRig) hire(t *testing.T, git gitRunner) (hirePlace, error) {
	t.Helper()
	keepRegistry(t)
	o := hirePlaceOptions{Worktrees: filepath.Join(r.base, "hires"), PortFree: func(int) bool { return true }, Git: git}
	return prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: r.repo, Worktree: true}, o)
}

// withRemote gives the rig's repo a bare remote in the rig's folder, under name.
func (r *ownerRig) withRemote(t *testing.T, name string) string {
	t.Helper()
	bare := filepath.Join(r.base, name+".git")
	gitRepoCmd(t, r.base, "init", "-q", "--bare", bare)
	gitRepoCmd(t, r.repo, "remote", "add", name, bare)
	return bare
}

// What was planted in the owner's repo before its first sandboxed day (a hooks folder with a post-checkout and a
// reference-transaction hook, an fsmonitor, a required filter its .gitattributes names) runs nothing at a hire's
// `worktree add`; the file is checked out as it is kept. The control, the add as the core ran it before, runs them.
func TestTheOwnersRepoRunsNothingPlantedAtAHiresWorktreeAdd(t *testing.T) {
	r := newOwnerRig(t)
	writeRepoFile(t, r.repo, ".gitattributes", "*.txt filter=sneak\n")
	gitRepoCmd(t, r.repo, "add", ".gitattributes")
	gitRepoCmd(t, r.repo, "commit", "-q", "-m", "attributes")
	hooks := filepath.Join(r.base, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, hook := range []string{"post-checkout", "reference-transaction"} {
		if err := os.Rename(r.standIn(t, hook), filepath.Join(hooks, hook)); err != nil {
			t.Fatal(err)
		}
	}
	for _, kv := range [][2]string{{"core.hooksPath", hooks}, {"core.fsmonitor", r.standIn(t, "fsmonitor")},
		{"filter.sneak.smudge", r.standIn(t, "filter")}, {"filter.sneak.required", "true"}} {
		gitRepoCmd(t, r.repo, "config", kv[0], kv[1])
	}

	if out := plainGit(r.repo, "worktree", "add", "-q", "-b", "control", filepath.Join(r.base, "control")); !strings.Contains(r.ran(t), "filter") {
		t.Fatalf("the control: the stand-in filter didn't run for a plain worktree add, so this row proves nothing: %s", out)
	}
	place, err := r.hire(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.ran(t); got != "" {
		t.Errorf("the core's worktree add ran the repo's %s", got)
	}
	if data, err := os.ReadFile(filepath.Join(place.Worktree, "a.txt")); err != nil || string(data) != "one\n" {
		t.Errorf("a.txt in the hire's worktree: %q %v", data, err)
	}
	if got := gitRepoCmd(t, place.Worktree, "symbolic-ref", "--short", "HEAD"); got != place.Branch {
		t.Errorf("the worktree is on %q, want %q", got, place.Branch)
	}
}

// A setting the core's git can't make harmless refuses the hire's worktree, in words that name it, with nothing made:
// an include (the file it names is read after the check), a key no read knows, an address that runs a program.
func TestTheOwnersRepoWithAnIncludeOrAnUnknownKeyMakesNoWorktree(t *testing.T) {
	for _, row := range []struct{ key, value, said string }{
		{"include.path", "elsewhere.cfg", "include.path"},
		{"includeIf.gitdir:/.path", "elsewhere.cfg", "includeif.*.path"},
		{"core.gitProxy", "stand-in", "core.gitproxy"},
		{"remote.origin.url", "ext::sh -c stand-in", "remote.*.url, an address that runs a program"},
	} {
		t.Run(row.key, func(t *testing.T) {
			r := newOwnerRig(t)
			gitRepoCmd(t, r.repo, "config", row.key, row.value)
			_, err := r.hire(t, nil)
			var refusal *hireRefusal
			if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "its git config has "+row.said+",") {
				t.Fatalf("got %v, want a refusal naming %s", err, row.said)
			}
			if entries, _ := os.ReadDir(filepath.Join(r.base, "hires", "proj")); len(entries) != 0 {
				t.Errorf("left %v", entries)
			}
			if got := branches(t, r.repo); got != "" {
				t.Errorf("left the branch %q", got)
			}
		})
	}
}

// A git that can't be told to leave the owner's config out (older than 2.32) makes no worktree.
func TestAnOldGitMakesNoWorktreeInTheOwnersRepo(t *testing.T) {
	r := newOwnerRig(t)
	saved := gitHere
	gitHere = func() gitHereInfo { return gitHereInfo{version: "git version 2.31.8"} }
	t.Cleanup(func() { gitHere = saved })
	_, err := r.hire(t, nil)
	var refusal *hireRefusal
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "older than 2.32") {
		t.Fatalf("got %v", err)
	}
	if got := branches(t, r.repo); got != "" {
		t.Errorf("left the branch %q", got)
	}
}

// The hire's branch is given its upstream, so a plain `git push` from its worktree works (its sandbox denies the
// config write `push -u` makes). The control: a branch made without it can't be pushed that way.
func TestAHiresBranchPushesWithAPlainPush(t *testing.T) {
	r := newOwnerRig(t)
	bare := r.withRemote(t, "origin")
	place, err := r.hire(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := gitRepoCmd(t, r.repo, "config", "--get", "branch."+place.Branch+".remote"); got != "origin" {
		t.Errorf("remote %q", got)
	}
	if got := gitRepoCmd(t, r.repo, "config", "--get", "branch."+place.Branch+".merge"); got != "refs/heads/"+place.Branch {
		t.Errorf("merge %q", got)
	}
	writeRepoFile(t, place.Worktree, "b.txt", "two\n")
	gitRepoCmd(t, place.Worktree, "add", "b.txt")
	gitRepoCmd(t, place.Worktree, "commit", "-q", "-m", "work")
	if out, err := exec.Command("git", "-C", place.Worktree, "push", "-q").CombinedOutput(); err != nil {
		t.Fatalf("a plain push: %v %s", err, out)
	}
	if got := gitRepoCmd(t, bare, "rev-parse", "refs/heads/"+place.Branch); got != gitRepoCmd(t, place.Worktree, "rev-parse", "HEAD") {
		t.Errorf("the remote has %q", got)
	}

	control := filepath.Join(r.base, "control")
	gitRepoCmd(t, r.repo, "worktree", "add", "-q", "-b", "control", control)
	if out, err := exec.Command("git", "-C", control, "push", "-q").CombinedOutput(); err == nil || !strings.Contains(string(out), "no upstream") {
		t.Fatalf("the control: a branch with no upstream was pushed, so this row proves nothing: %v %s", err, out)
	}
}

// The remote a hire's branch pushes to: origin, else the one the checkout's branch tracks, else the only one; else
// none, and nothing is set.
func TestTheRemoteAHiresBranchPushesTo(t *testing.T) {
	for _, row := range []struct {
		name    string
		remotes []string
		tracks  string
		want    string
	}{
		{"origin among others", []string{"up", "origin"}, "up", "origin"},
		{"the tracked one", []string{"up", "fork"}, "fork", "fork"},
		{"the only one", []string{"fork"}, "", "fork"},
		{"two, none tracked", []string{"a", "b"}, "", ""},
		{"none", nil, "", ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			r := newOwnerRig(t)
			for _, name := range row.remotes {
				r.withRemote(t, name)
			}
			if row.tracks != "" {
				gitRepoCmd(t, r.repo, "config", "branch.main.remote", row.tracks)
			}
			if got := hirePushRemote(r.repo, runGit); got != row.want {
				t.Errorf("%q, want %q", got, row.want)
			}
			place, err := r.hire(t, nil)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := exec.Command("git", "-C", r.repo, "config", "--get", "branch."+place.Branch+".remote").Output()
			if strings.TrimSpace(string(got)) != row.want {
				t.Errorf("preset %q, want %q", got, row.want)
			}
		})
	}
}

// Off a Mac, git checks a hire's tree out where it is and the carry keeps no name for it: the undo's emptied folder
// goes by the path the hire made it at, and only while that name is still the folder the carry opened (FXCI53).
func TestACarriedTreeWithNoNameGoesByItsPathOnlyWhileItIsTheSameFolder(t *testing.T) {
	for _, swapped := range []bool{false, true} {
		dir := t.TempDir()
		tree := filepath.Join(dir, "ada-abcdef")
		if err := os.Mkdir(tree, 0o755); err != nil {
			t.Fatal(err)
		}
		root, err := os.OpenRoot(tree)
		if err != nil {
			t.Fatal(err)
		}
		branch := "hire/abcdef123456/ada-abcdef"
		carriedHires.Lock()
		carriedHires.by[branch] = hireCarried{tree: root}
		carriedHires.Unlock()
		if swapped {
			if err := os.Rename(tree, filepath.Join(dir, "moved")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(tree, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		removeCarriedTreeFolder(branch, "")
		if _, err := os.Lstat(tree); err != nil {
			t.Fatalf("swapped %v: no path given, yet the folder went: %v", swapped, err)
		}
		removeCarriedTreeFolder(branch, tree)
		forgetCarried(branch)
		if _, err := os.Lstat(tree); errors.Is(err, os.ErrNotExist) == swapped {
			t.Errorf("swapped %v: the folder at the tree's name: %v", swapped, err)
		}
	}
}

// When the upstream can't be written, the hire's worktree and branch are taken away again, and it says so.
func TestAnUpstreamThatCantBeSetLeavesNothing(t *testing.T) {
	r := newOwnerRig(t)
	r.withRemote(t, "origin")
	git := func(dir string, args ...string) (int, string, bool) {
		if len(args) > 1 && args[0] == "config" && args[1] == "--local" {
			return 255, "error: could not lock config file", true
		}
		return runGit(dir, args...)
	}
	_, err := r.hire(t, git)
	var refusal *hireRefusal
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "pushes to") {
		t.Fatalf("got %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(r.base, "hires", "proj")); len(entries) != 0 {
		t.Errorf("left %v", entries)
	}
	if got := branches(t, r.repo); got != "" {
		t.Errorf("left the branch %q", got)
	}
}

// With an upstream, `git branch -d` also deletes a branch that is pushed but not merged in the checkout. The fire's
// removal keeps such a branch, as it did before the upstream; a merged one still goes. The control: `branch -d` alone
// deletes the pushed one.
func TestTheFireKeepsAPushedBranchTheCheckoutHasntMerged(t *testing.T) {
	r := newOwnerRig(t)
	r.withRemote(t, "origin")
	place, err := r.hire(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, place.Worktree, "b.txt", "two\n")
	gitRepoCmd(t, place.Worktree, "add", "b.txt")
	gitRepoCmd(t, place.Worktree, "commit", "-q", "-m", "work")
	gitRepoCmd(t, place.Worktree, "push", "-q")
	if !removeWorkspace(r.repo, place.Worktree, place.Branch, runGit) {
		t.Fatal("the clean worktree wasn't removed")
	}
	if got := branches(t, r.repo); got != place.Branch {
		t.Errorf("branches %q, want the pushed %q kept", got, place.Branch)
	}
	if out, err := exec.Command("git", "-C", r.repo, "branch", "-d", place.Branch).CombinedOutput(); err != nil {
		t.Fatalf("the control: `branch -d` kept the pushed branch, so this row proves nothing: %v %s", err, out)
	}

	// Pushed, then more of it merged in the checkout: HEAD holds it all, its upstream is behind. It goes, as it went
	// before the upstream; the control: `branch -d` alone keeps it (RT126SAX7's nit 4).
	for _, del := range []string{"", "-d"} {
		r3 := newOwnerRig(t)
		r3.withRemote(t, "origin")
		place, err = r3.hire(t, nil)
		if err != nil {
			t.Fatal(err)
		}
		writeRepoFile(t, place.Worktree, "b.txt", "two\n")
		gitRepoCmd(t, place.Worktree, "add", "b.txt")
		gitRepoCmd(t, place.Worktree, "commit", "-q", "-m", "work")
		gitRepoCmd(t, place.Worktree, "push", "-q")
		writeRepoFile(t, place.Worktree, "c.txt", "three\n")
		gitRepoCmd(t, place.Worktree, "add", "c.txt")
		gitRepoCmd(t, place.Worktree, "commit", "-q", "-m", "more")
		gitRepoCmd(t, r3.repo, "merge", "-q", "--ff-only", place.Branch)
		if del == "-d" {
			gitRepoCmd(t, r3.repo, "worktree", "remove", "--force", place.Worktree)
			if out, err := exec.Command("git", "-C", r3.repo, "branch", "-d", place.Branch).CombinedOutput(); err == nil {
				t.Fatalf("the control: `branch -d` deleted the merged branch its upstream is behind, so this row proves nothing: %s", out)
			}
			continue
		}
		if !removeWorkspace(r3.repo, place.Worktree, place.Branch, runGit) {
			t.Fatal("the clean worktree wasn't removed")
		}
		if got := branches(t, r3.repo); got != "" {
			t.Errorf("a branch the checkout holds, its upstream behind, was kept: %q", got)
		}
	}

	r2 := newOwnerRig(t)
	r2.withRemote(t, "origin")
	place, err = r2.hire(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !removeWorkspace(r2.repo, place.Worktree, place.Branch, runGit) {
		t.Fatal("the clean worktree wasn't removed")
	}
	if got := branches(t, r2.repo); got != "" {
		t.Errorf("a branch the checkout holds was kept: %q", got)
	}
}

// RT126SAX7R2's F2: the fire's merged look names the branch in full, so a tag of its name, at a commit the checkout
// holds, never stands in for an unmerged branch. The control: by its short name, git looks at the tag.
func TestTheFireLooksAtTheBranchNotATagOfItsName(t *testing.T) {
	r := newOwnerRig(t)
	place, err := r.hire(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, place.Worktree, "b.txt", "two\n")
	gitRepoCmd(t, place.Worktree, "add", "b.txt")
	gitRepoCmd(t, place.Worktree, "commit", "-q", "-m", "work")
	gitRepoCmd(t, r.repo, "tag", place.Branch, "main")
	if err := exec.Command("git", "-C", r.repo, "merge-base", "--is-ancestor", place.Branch, "HEAD").Run(); err != nil {
		t.Fatalf("the control: by its short name git looks at the branch (%v), so this row proves nothing", err)
	}
	if !removeWorkspace(r.repo, place.Worktree, place.Branch, runGit) {
		t.Fatal("the clean worktree wasn't removed")
	}
	if got := branches(t, r.repo); got != place.Branch {
		t.Errorf("branches %q, want the unmerged %q kept", got, place.Branch)
	}
}

// The config of a file a desk could have written goes to the owner's own git only when every setting in it is data.
func TestAHandedOverConfigIsDataOnly(t *testing.T) {
	list := func(kvs ...string) []byte { return []byte(strings.Join(kvs, "\x00") + "\x00") }
	for _, row := range []struct {
		list []byte
		want string
	}{
		{list("user.name\nada", "core.autocrlf\ninput", "remote.origin.url\nhttps://example.com/x.git"), ""},
		{list("core.fsmonitor\n/x"), "core.fsmonitor"},
		{list("core.hookspath\n/x"), "core.hookspath"},
		{list("core.editor\n/x"), "core.editor"},
		{list("credential.helper\n/x"), "credential.helper"},
		{list("filter.a.smudge\n/x"), "filter.*.smudge"},
		{list("diff.a.textconv\n/x"), "diff.*.textconv"},
		{list("lfs.customtransfer.x.path\n/x"), "lfs.*.path"}, // a program in a data section
		{list("alias.st\n!x"), "alias.st"},
		{list("include.path\nx.cfg"), "include.path"},
		{list("includeif.gitdir:/.path\nx.cfg"), "includeif.*.path"},
		{list("core.gitproxy\n/x"), "core.gitproxy"},
		{list("remote.o.url\next::x"), "remote.*.url" + gitRefusedAddress},
		{list("nosection\n"), "a key with no section"},
		// RT126SAX7: a program in a data section (B2), where the work tree is (B1), and a merge strategy's program.
		{list("gc.auto\n1", "gc.recentobjectshook\n/stand-in"), "gc.recentobjectshook"},
		{list("core.worktree\n/elsewhere"), "core.worktree"},
		{list("core.bare\nfalse"), "core.bare"},
		{list("pull.twohead\nstand-in"), "pull.twohead"},
		{list("pull.octopus\nstand-in"), "pull.octopus"},
		{list("pull.rebase\ntrue", "gc.auto\n0"), ""},
		// RT126SAX7R2's F1: a merge strategy by branch; the branch's other settings are still data.
		{list("branch.hire/ada-abcdef.mergeOptions\n-s stand-in"), "branch.*.mergeoptions"},
		{list("branch.main.remote\norigin", "branch.main.merge\nrefs/heads/main", "branch.main.rebase\ntrue"), ""},
	} {
		if got := gitHandOverConfig(row.list); got != row.want {
			t.Errorf("%q: %q, want %q", row.list, got, row.want)
		}
	}
}

// The fire's deletion of a hire's branch takes its upstream with it, as git's own `branch -D` does: the core carries
// the deletion as a file, so it drops the branch's section in the repo's config itself. The control: the preset is
// there before the fire.
func TestTheFiresBranchDeletionTakesItsUpstreamWithIt(t *testing.T) {
	r := newOwnerRig(t)
	r.withRemote(t, "origin")
	place, err := r.hire(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := gitRepoCmd(t, r.repo, "config", "--get", "branch."+place.Branch+".remote"); got != "origin" {
		t.Fatalf("the control: the hire's branch has no upstream (%q), so this row proves nothing", got)
	}
	if !removeWorkspace(r.repo, place.Worktree, place.Branch, runGit) {
		t.Fatal("the clean worktree wasn't removed")
	}
	if got := branches(t, r.repo); got != "" {
		t.Fatalf("the merged branch was kept: %q", got)
	}
	if out, err := exec.Command("git", "-C", r.repo, "config", "--get-regexp", `^branch\.`).CombinedOutput(); err == nil {
		t.Errorf("the deleted branch's section is still in the repo's config: %s", out)
	}
}

// Decision 2's forms, exactly (RT126SAX7STACK F1, the reviewer's TestRVSTheAdmittedFormsAndNoWider): what runs on the
// repo's own config is the forms SA-X7's callers send (hireplace.go's merged look, hirePushRemote's remotes,
// presetUpstream's two keys) and nothing wider: another option or count, another config file, another branch, key or
// value, a name that isn't a hire's.
func TestTheOwnersGitAdmitsItsCallersFormsAndNoWider(t *testing.T) {
	b := "hire/abcdef123456/ada-abcdef"
	admitted := func(args ...string) bool { return gitReadsRefsOnly(args) || gitPresetsUpstream(args) }
	for _, c := range []struct {
		args []string
		want bool
	}{
		{[]string{"merge-base", "--is-ancestor", "refs/heads/" + b, "HEAD"}, true},
		{[]string{"config", "-z", "--get-regexp", `^remote\..+\.url$`}, true},
		{[]string{"config", "--local", "branch." + b + ".remote", "origin"}, true},
		{[]string{"config", "--local", "branch." + b + ".merge", "refs/heads/" + b}, true},
		{[]string{"merge-base", "refs/heads/" + b, "HEAD"}, false},
		{[]string{"merge-base", "--is-ancestor", "--fork-point", "HEAD"}, false},
		{[]string{"merge-base", "--is-ancestor", "a", "b", "c"}, false},
		{[]string{"merge-base", "--octopus", "a", "b"}, false},
		{[]string{"merge-base", "--is-ancestor", "a", "--no-replace-objects"}, false},
		{[]string{"config", "-z", "--get-regexp", "x", "y"}, false},
		{[]string{"config", "-z", "--get-regexp", "--file=/x"}, false},
		{[]string{"config", "--get-regexp", "x", "-z"}, false},
		{[]string{"config", "-z", "--file", "x"}, false},
		{[]string{"config", "-z", "--get-regexp"}, false},
		{[]string{"config", "--local", "branch.main.remote", "origin"}, false},
		{[]string{"config", "--local", "branch.main.merge", "refs/heads/main"}, false},
		{[]string{"config", "--local", "branch." + b + ".merge", "refs/heads/main"}, false},
		{[]string{"config", "--local", "branch." + b + ".merge", "refs/heads/" + b + "\n"}, false},
		{[]string{"config", "--local", "branch." + b + ".pushremote", "origin"}, false},
		{[]string{"config", "--local", "branch." + b + ".mergeoptions", "-sx"}, false},
		{[]string{"config", "--local", "branch." + b + ".rebase", "true"}, false},
		{[]string{"config", "--local", "branch." + b + ".remote", "-x"}, false},
		{[]string{"config", "--local", "branch." + b + ".remote", ""}, false},
		{[]string{"config", "--local", "branch." + b + ".remote", "a b"}, false},
		{[]string{"config", "--local", "branch." + b + ".remote", "a\nb"}, false},
		{[]string{"config", "--local", "branch." + b + ".remote", "a\rb"}, false},
		{[]string{"config", "--local", "branch." + b + ".remote", "a\\nb"}, false},
		{[]string{"config", "--local", "branch." + b + ".remote", "a\x00b"}, false},
		{[]string{"config", "--global", "branch." + b + ".remote", "origin"}, false},
		{[]string{"config", "--system", "branch." + b + ".remote", "origin"}, false},
		{[]string{"config", "--file", "branch." + b + ".remote", "origin"}, false},
		{[]string{"config", "--local", "--add", "branch." + b + ".remote", "origin"}, false},
		{[]string{"config", "--local", "branch." + b + ".remote", "origin", "x"}, false},
		{[]string{"config", "--local", "core.hooksPath", "x"}, false},
		{[]string{"config", "--local", "user.name", "x"}, false},
		{[]string{"config", "--local", "hook.hire/x.remote", "origin"}, false},
		{[]string{"config", "--local", "Branch." + b + ".remote", "origin"}, false},
		{[]string{"config", "--local", "branch.hire/../main.remote", "origin"}, false},
		{[]string{"config", "--local", "branch.hire/.remote", "origin"}, false},
		{[]string{"config", "--local", "branch.hire.remote", "origin"}, false},
		{[]string{"config", "--local", "branch.hire/a[core].remote", "origin"}, false},
		{[]string{"config", "--local", "branch.hire/a b.remote", "origin"}, false},
		{[]string{"config", "--local", "branch.hire/a\\b.remote", "origin"}, false},
		{[]string{"config", "--local", "--remove-section", "branch." + b}, false},
		{[]string{"config", "--local", "--unset", "branch." + b + ".remote"}, false},
		{[]string{"config", "--local", "--unset-all", "branch." + b + ".remote"}, false},
		{[]string{"config", "set", "branch." + b + ".remote", "origin"}, false},
	} {
		if got := admitted(c.args...); got != c.want {
			t.Errorf("%q: admitted %v, want %v", c.args, got, c.want)
		}
	}
}

// repoConfigKeys is the repo's own config (`--local`), one "key=value" per entry, read with plain git.
func repoConfigKeys(t *testing.T, repo string) map[string]bool {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "config", "--local", "--list", "-z").Output()
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]bool{}
	for _, record := range strings.Split(string(out), "\x00") {
		if record != "" {
			entries[strings.Replace(record, "\n", "=", 1)] = true
		}
	}
	return entries
}

// Decision 6's removal, exactly (RT126SAX7STACK F1, F2, F4): a branch the core couldn't delete keeps its upstream
// (main, checked out; a hire's, checked out in its worktree); a branch that isn't a hire's keeps it, as no caller
// deletes one; and a hire's deleted branch takes its own two keys and nothing else: not the section of a branch named
// it and a "]" after it, which a section's removal by name (git's own -D too) would take, nor a section of another
// kind by its name.
func TestTheBranchDeletionTakesOnlyThatHiresUpstream(t *testing.T) {
	r := newOwnerRig(t)
	names := []string{"hire/a", "hire/a]", "hire/a]x", "hire/ab", "hire/busy", "other"}
	for _, name := range names {
		gitRepoCmd(t, r.repo, "branch", name)
	}
	gitRepoCmd(t, r.repo, "worktree", "add", "-q", filepath.Join(r.base, "busy"), "hire/busy")
	for _, name := range append([]string{"main"}, names...) {
		gitRepoCmd(t, r.repo, "config", "branch."+name+".remote", "origin")
		gitRepoCmd(t, r.repo, "config", "branch."+name+".merge", "refs/heads/"+name)
	}
	gitRepoCmd(t, r.repo, "config", "core.hire/a.x", "y")
	for _, row := range []struct {
		why, name string
		deleted   bool
		gone      []string
	}{
		{"main, checked out", "main", false, nil},
		{"a hire's, checked out in its worktree", "hire/busy", false, nil},
		{"not a hire's (F4)", "other", true, nil},
		{"a hire's, beside hire-a] and hire-a]x (F2)", "hire/a", true, []string{"branch.hire/a.merge=refs/heads/hire/a", "branch.hire/a.remote=origin"}},
	} {
		t.Run(row.why, func(t *testing.T) { deleteOneBranch(t, r, row.name, row.deleted, row.gone) })
	}
	if got := gitRepoCmd(t, r.repo, "config", "--get", "core.hire/a.x"); got != "y" {
		t.Errorf("core.hire/a.x = %q", got)
	}
}

// deleteOneBranch deletes name through the owner's git, and checks whether it went (deleted) and that it took gone,
// and only that, from the repo's config.
func deleteOneBranch(t *testing.T, r *ownerRig, name string, deleted bool, gone []string) {
	t.Helper()
	before := repoConfigKeys(t, r.repo)
	status, out, ok := ownerGit(r.repo, []string{"branch", "-D", name})
	after := repoConfigKeys(t, r.repo)
	if _, err := realGit(r.repo, "rev-parse", "--verify", "-q", "refs/heads/"+name); (err != nil) != deleted {
		t.Errorf("branch -D %q: %d %v %q; deleted %v, want %v", name, status, ok, out, err != nil, deleted)
	}
	var took []string
	for entry := range before {
		if !after[entry] {
			took = append(took, entry)
		}
	}
	slices.Sort(took)
	if !slices.Equal(took, gone) {
		t.Errorf("branch -D %q took %q from the repo's config, want %q", name, took, gone)
	}
	for entry := range after {
		if !before[entry] {
			t.Errorf("branch -D %q added %q", name, entry)
		}
	}
}
