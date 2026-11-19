//go:build !windows

package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// RT126CRLFRULING's rows (FXLINEENDS): the core's git carries the owner's core.autocrlf, core.eol and core.safecrlf,
// read from their system and global config and the core's environment only, in git's order around the repo's own
// settings, and nothing else of theirs. Stand-ins only: a stand-in home and XDG folder (with the system config off),
// the owner's environment as GIT_CONFIG_COUNT, throwaway repos, and marker programs that only leave a file in the
// test's own folder. Each row looks only at what git did (the checkout's line ends, the core's status), so it runs
// as written on a tree from before the carry, where it fails.

// lineEndOwner is a stand-in owner for the core's git: a home and an XDG folder of the test's, no system config and
// nothing in the environment, unless a row puts it there. It returns the home.
func lineEndOwner(t *testing.T) string {
	t.Helper()
	isolate(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	home := physical(t.TempDir())
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, name := range []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	if err := os.MkdirAll(filepath.Join(home, "xdg", "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return home
}

// ownerFiles writes the stand-in owner's ~/.gitconfig and XDG git config ("" leaves a file out).
func ownerFiles(t *testing.T, home, gitconfig, xdg string) {
	t.Helper()
	for file, text := range map[string]string{filepath.Join(home, ".gitconfig"): gitconfig, filepath.Join(home, "xdg", "git", "config"): xdg} {
		os.Remove(file)
		if text != "" {
			if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// ownerEnvironment is the core's own environment handing git config, as an owner (or WIN125b's row) sets it.
func ownerEnvironment(t *testing.T, kv ...string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_COUNT", fmt.Sprint(len(kv)/2))
	for i := 0; i < len(kv)/2; i++ {
		t.Setenv(fmt.Sprintf("GIT_CONFIG_KEY_%d", i), kv[2*i])
		t.Setenv(fmt.Sprintf("GIT_CONFIG_VALUE_%d", i), kv[2*i+1])
	}
}

// lineEndRepo is a repo with t.txt ("one\ntwo\n", LF) and, if given, a committed .gitattributes; made before the
// stand-in owner's config is written, so the owner's settings never touch it.
func lineEndRepo(t *testing.T, attributes string) string {
	t.Helper()
	repo := placeRepo(t)
	writeRepoFile(t, repo, "t.txt", "one\ntwo\n")
	if attributes != "" {
		writeRepoFile(t, repo, ".gitattributes", attributes)
	}
	gitRepoCmd(t, repo, "add", ".")
	gitRepoCmd(t, repo, "commit", "-q", "-m", "text")
	gitRepoCmd(t, repo, "config", "user.name", "Stand In")
	gitRepoCmd(t, repo, "config", "user.email", "stand-in@example.invalid")
	return repo
}

// lineEndHire makes a hire's worktree from repo as the core makes every one (prepareHire, through runGit), and
// returns it; n numbers the row's hires.
func lineEndHire(t *testing.T, repo string, n *int) string {
	t.Helper()
	*n++
	place, err := prepareHire(hireRequest{HireID: fmt.Sprintf("%06dabcdef", *n), Name: "ends", Folder: repo, Worktree: true}, placeOptions(t))
	if err != nil {
		t.Fatalf("the hire's worktree: %v", err)
	}
	return place.Worktree
}

// lineEndsOf is "CRLF", "LF" or "none": how file's lines end.
func lineEndsOf(t *testing.T, file string) string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case strings.Contains(string(data), "\r\n"):
		return "CRLF"
	case strings.Contains(string(data), "\n"):
		return "LF"
	}
	return "none"
}

// ownersGit is git as the owner (or the hire's agent) runs it in dir: their config, all of it.
func ownersGit(dir string, args ...string) string {
	out, _ := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(out))
}

// laterStamp moves file's time on, so the next status reads its content rather than trusting the index's stamp.
func laterStamp(t *testing.T, file string) {
	t.Helper()
	later := time.Now().Add(3 * time.Second)
	if err := os.Chtimes(file, later, later); err != nil {
		t.Fatal(err)
	}
}

func TestTheOwnersLineEndSettingIsCarriedIntoAHiresWorktree(t *testing.T) {
	home := lineEndOwner(t)
	n := 0
	// The owner's ~/.gitconfig asks for CRLF in the working files.
	plain := lineEndRepo(t, "")
	ownerFiles(t, home, "[core]\n\tautocrlf = true\n", "")
	if got := lineEndsOf(t, filepath.Join(lineEndHire(t, plain, &n), "t.txt")); got != "CRLF" {
		t.Errorf("the owner's ~/.gitconfig says core.autocrlf=true: the hire's worktree is %s, want CRLF as the owner's git checks out", got)
	}
	// The owner's XDG config sets core.eol, with a ~/.gitconfig present too (which a `config --global` read would see
	// alone): it's carried. core.eol applies to files marked text, as git does.
	text := lineEndRepo(t, "*.txt text\n")
	ownerFiles(t, home, "[user]\n\tname = Stand In\n", "[core]\n\teol = crlf\n")
	if got := lineEndsOf(t, filepath.Join(lineEndHire(t, text, &n), "t.txt")); got != "CRLF" {
		t.Errorf("the owner's XDG config says core.eol=crlf (and ~/.gitconfig exists): the hire's worktree is %s, want CRLF", got)
	}
	// The control: with nothing set, LF, as the owner's git would write.
	ownerFiles(t, home, "", "")
	if got := lineEndsOf(t, filepath.Join(lineEndHire(t, text, &n), "t.txt")); got != "LF" {
		t.Errorf("with no line-end setting the hire's worktree is %s, want LF", got)
	}
}

func TestARepoOwnLineEndSettingWinsOverTheOwners(t *testing.T) {
	home := lineEndOwner(t)
	n := 0
	plain := lineEndRepo(t, "")
	local := lineEndRepo(t, "")
	gitRepoCmd(t, local, "config", "core.autocrlf", "false")
	ownerFiles(t, home, "[core]\n\tautocrlf = true\n", "")
	for _, c := range []struct {
		repo, want, why string
	}{
		{plain, "CRLF", "a repo with no setting of its own takes the owner's global core.autocrlf=true"},
		{local, "LF", "the repo's own core.autocrlf=false beats the owner's global true, as in the owner's git"},
	} {
		if got := lineEndsOf(t, filepath.Join(lineEndHire(t, c.repo, &n), "t.txt")); got != c.want {
			t.Errorf("%s: the hire's worktree is %s, want %s", c.why, got, c.want)
		}
		if got := filepath.Join(t.TempDir(), "own"); ownersGit(c.repo, "worktree", "add", "-q", "-b", fmt.Sprintf("own-%d", n), got) == "" {
			if own := lineEndsOf(t, filepath.Join(got, "t.txt")); own != c.want {
				t.Errorf("%s: the owner's own git checks out %s, so this row's premise is wrong", c.why, own)
			}
		}
	}
	// The core's own environment is git's command scope, above the repo's: there the owner's value wins, as git has it.
	ownerFiles(t, home, "", "")
	ownerEnvironment(t, "core.autocrlf", "true")
	if got := lineEndsOf(t, filepath.Join(lineEndHire(t, local, &n), "t.txt")); got != "CRLF" {
		t.Errorf("core.autocrlf=true in the core's environment beats the repo's own false in the owner's git: the hire's worktree is %s, want CRLF", got)
	}
}

func TestALineEndSettingFromADeskWritableSourceIsntCarried(t *testing.T) {
	home := lineEndOwner(t)
	m := newDeskMoves(t)
	n := 0
	plain := lineEndRepo(t, "")
	filtered := lineEndRepo(t, "*.txt filter=owner\n") // made now: the owner's git refuses the bad values below
	// The control: the owner's own ~/.gitconfig is carried.
	ownerFiles(t, home, "[core]\n\tautocrlf = true\n", "")
	if got := lineEndsOf(t, filepath.Join(lineEndHire(t, plain, &n), "t.txt")); got != "CRLF" {
		t.Fatalf("the control: the owner's own core.autocrlf=true gives a %s worktree, want CRLF, so this row proves nothing", got)
	}
	// An include (or an includeIf) names a file a desk wrote: its settings aren't the owner's, and aren't carried.
	desk := filepath.Join(m.base, "desk-wrote.gitconfig")
	if err := os.WriteFile(desk, []byte("[core]\n\tautocrlf = true\n\teol = crlf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ownerFiles(t, home, "[include]\n\tpath = "+desk+"\n[includeIf \"gitdir:/\"]\n\tpath = "+desk+"\n", "[include]\n\tpath = "+desk+"\n")
	if got := lineEndsOf(t, filepath.Join(lineEndHire(t, plain, &n), "t.txt")); got != "LF" {
		t.Errorf("a line-end setting from an included file a desk wrote was carried: the hire's worktree is %s, want LF", got)
	}
	// Values that aren't line ends are dropped: the hire is made, as with nothing set, and nothing runs.
	ownerFiles(t, home, "[core]\n\tautocrlf = \"!touch "+m.marker+"\"\n\teol = "+m.base+"\n\tsafecrlf = $(touch "+m.marker+")\n", "")
	if got := lineEndsOf(t, filepath.Join(lineEndHire(t, plain, &n), "t.txt")); got != "LF" {
		t.Errorf("values that aren't line ends: the hire's worktree is %s, want LF", got)
	}
	if ran := m.ran(t); ran != "" {
		t.Errorf("a value that isn't a line end ran: %s", ran)
	}
	// Other keys of the owner's global config, which name programs, aren't carried: a filter a committed attribute
	// names (git reads it when it checks the files out) and an fsmonitor. Only the line ends are.
	ownerFiles(t, home, "[core]\n\tautocrlf = true\n\tfsmonitor = "+m.program(t, "fsmonitor", false)+"\n[filter \"owner\"]\n\tclean = "+
		m.program(t, "clean", true)+"\n\tsmudge = "+m.program(t, "smudge", true)+"\n", "")
	tree := lineEndHire(t, filtered, &n)
	if got := lineEndsOf(t, filepath.Join(tree, "t.txt")); got != "CRLF" {
		t.Errorf("the owner's core.autocrlf=true beside a filter and an fsmonitor: the hire's worktree is %s, want CRLF", got)
	}
	laterStamp(t, filepath.Join(tree, "t.txt"))
	if status, out, ok := runGit(tree, "status", "--porcelain"); !ok || status != 0 {
		t.Errorf("the core's status: %d %q %v", status, out, ok)
	}
	if ran := m.ran(t); ran != "" {
		t.Errorf("a program of the owner's global config ran in the core's git: %s", ran)
	}
	// The control: the owner's own git runs that filter.
	if ownersGit(filtered, "worktree", "add", "-q", "-b", "own-filtered", filepath.Join(t.TempDir(), "own")); m.ran(t) == "" {
		t.Error("the control: the owner's own git ran no stand-in, so this row proves nothing")
	}
}

func TestLineEndSettingsWithADesksAttributesRunNothing(t *testing.T) {
	lineEndOwner(t)
	m := newDeskMoves(t)
	m.knownOnly = true
	n := 0
	// The desk wrote the repo: attributes naming a filter, a diff driver and a merge driver on every file, and its
	// config naming programs for them, an fsmonitor and a hooks folder.
	repo := lineEndRepo(t, "* text eol=crlf filter=z diff=t merge=mz ident working-tree-encoding=UTF-8\n")
	common := filepath.Join(repo, ".git")
	gitRepoCmd(t, repo, "config", "diff.t.textconv", m.program(t, "textconv", true))
	gitRepoCmd(t, repo, "config", "diff.t.command", m.program(t, "diffcmd", false))
	gitRepoCmd(t, repo, "config", "merge.mz.driver", m.program(t, "mergedriver", false))
	gitRepoCmd(t, repo, "config", "filter.z.process", m.program(t, "process", false))
	m.now(t, common)
	for _, c := range []struct {
		setting []string
		clean   bool // the owner's git, and so the core, calls the committed CRLF file unchanged
	}{
		{[]string{"core.autocrlf", "true", "core.eol", "crlf", "core.safecrlf", "true"}, true},
		{[]string{"core.autocrlf", "input", "core.safecrlf", "warn"}, true},
		{[]string{"core.autocrlf", "false", "core.eol", "native"}, false},
	} {
		// The owner's settings, and a driver of theirs under the attribute's name: only the line ends may be carried.
		ownerEnvironment(t, append(append([]string{}, c.setting...), "filter.z.smudge", m.program(t, "ownersmudge", true))...)
		tree := lineEndHire(t, repo, &n)
		if ran := m.ran(t); ran != "" {
			t.Errorf("%v: making the worktree ran: %s", c.setting, ran)
		}
		file := filepath.Join(tree, "t.txt")
		if err := os.WriteFile(file, []byte("uno\r\ntwo\r\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"status", "--porcelain"}, {"diff", "--numstat", "HEAD"}, {"add", "-A"}, {"commit", "-q", "-m", "x"}} {
			if status, out, ok := runGit(tree, args...); !ok || status != 0 {
				t.Errorf("%v: the core's %s: %d %q %v", c.setting, args[0], status, out, ok)
			}
		}
		laterStamp(t, file)
		status, out, ok := runGit(tree, "status", "--porcelain")
		if ran := m.ran(t); ran != "" {
			t.Errorf("%v: the core's reads and writes ran: %s", c.setting, ran)
		}
		if c.clean && (!ok || status != 0 || out != "") {
			t.Errorf("%v: after the commit the core's status is %d %q %v, want clean, as the owner's git has it", c.setting, status, out, ok)
		}
	}
	// The control: the owner's own git, on the desk's repo, runs the stand-ins.
	if ownersGit(repo, "worktree", "add", "-q", "-b", "control", filepath.Join(t.TempDir(), "control")); m.ran(t) == "" {
		t.Error("the control: a plain worktree add ran nothing, so this row proves nothing")
	}
}

func TestTheCoresStatusAgreesWithTheOwnersGitOnLineEnds(t *testing.T) {
	lineEndOwner(t)
	ownerEnvironment(t, "core.autocrlf", "true") // a Windows install's default, as WIN125b's row sets it
	n := 0
	repo := lineEndRepo(t, "")
	tree := lineEndHire(t, repo, &n)
	file := filepath.Join(tree, "t.txt")
	// The hire's own git (the agent's shell, the owner's config) restores the file, as checkout --, restore and stash
	// do: CRLF.
	os.Remove(file)
	ownersGit(tree, "checkout", "--", "t.txt")
	laterStamp(t, file)
	if got := lineEndsOf(t, file); got != "CRLF" {
		t.Fatalf("the hire's own git restored t.txt as %s, want CRLF: this row proves nothing", got)
	}
	if own := ownersGit(tree, "status", "--porcelain"); own != "" {
		t.Fatalf("the owner's own git calls the tree changed (%q): this row proves nothing", own)
	}
	// The core's status, on a tree with no row (ownerGit), agrees: clean.
	if status, out, ok := runGit(tree, "status", "--porcelain"); !ok || status != 0 || out != "" {
		t.Errorf("the core's status of a tree the owner's git calls clean: %d %q %v, want clean", status, out, ok)
	}
	// So does the fire's look at the hire's own tree (deskGit, no sandbox).
	keepRegistry(t)
	h := Hire{ID: fmt.Sprintf("%06dabcdef", n), Name: "ends", Role: "builder", Folder: keptPath(repo), Repo: repo, Worktree: keptPath(tree),
		Branch: "hire/ends-" + fmt.Sprintf("%06d", n), State: "working"}
	writeRegistry(t, h)
	if message := uncommittedRefusal(h, runGit); message != "" {
		t.Errorf("the fire's look at a tree the owner's git calls clean: %q", message)
	}
}
