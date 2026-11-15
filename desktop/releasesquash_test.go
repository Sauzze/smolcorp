//go:build unix

package desktop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A release goes out as one squashed commit straight onto main, never as the branches it was made on with their
// history, and scripts/release-desktop.sh builds only from that commit: HEAD has one parent, origin/main. Each case
// runs the script in a throwaway repo of its own, as a subtest beside the others. Each run takes a second or more
// (the go and cc the script pins at its start), so these rows are heavy (heavyReleaseRows): the script runs them
// before every release, and go test skips them elsewhere. Its check-public.sh is a stand-in that says it ran, and a stand-in
// go comes first on PATH and fails, so nothing is ever built: a commit that passes every check stops at the first go
// that would build. Only go version and go env go to the real go, for the go the script names at its start. The
// stand-in answers go list and go test of the bundled tmux's test as a test keeping M7/L4's contract would (its rows
// in releasetmuxtest_test.go hand them to the real go), and the script's run of its own tests as their passing, so a
// release commit gets past those checks too.

const (
	checkPublicRan = "stand-in check-public ran"
	goReached      = "stand-in go reached"

	// The first words of each stop, so a case shows which check stopped it.
	stopParents   = "HEAD isn't one"
	stopNoMain    = "origin/main isn't here"
	stopClean     = "Commit or set aside these first"
	stopTree      = "aren't HEAD's tree"
	stopSyso      = "Remove these .syso files first"
	stopIgnoredGo = "Go compiles them"
	stopWorktree  = "git here looks at another folder"
)

// releaseRepo is a throwaway repo holding release-desktop.sh, the stand-in check-public.sh and a little Go code
// (go.mod and web.go at the top, cmd/smolcorp/main.go, the stand-in tmux test in desktop/): main and origin/main at
// one commit, and a branch "work" two commits past it.
type releaseRepo struct {
	t              *testing.T
	dir, home, bin string
	env            []string
}

func newReleaseRepo(t *testing.T) *releaseRepo {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("no /bin/bash")
	}
	script, err := os.ReadFile(filepath.Join("..", "scripts", "release-desktop.sh"))
	if err != nil {
		t.Fatal(err)
	}
	_, goCache := realGo(t)
	r := &releaseRepo{t: t, dir: t.TempDir(), home: t.TempDir(), bin: t.TempDir()}
	// Its own home, so no one's own git or go settings apply, and its own temp folder; a stand-in go before the real
	// one. The real go builds with the cache it already has, never a newer toolchain or a module from the network.
	r.env = []string{
		"PATH=" + r.bin + ":" + filepath.Dir(gitPath) + ":/usr/bin:/bin",
		"HOME=" + r.home, "XDG_CONFIG_HOME=" + r.home, "GIT_CONFIG_NOSYSTEM=1", "LANG=C", "TMPDIR=" + t.TempDir(),
		"GOCACHE=" + goCache, "GOTOOLCHAIN=local", "GOPROXY=off",
	}
	r.writeGo(false)
	r.writeOtherGo(filepath.Join(r.bin, "other-go"))

	r.git("init", "-q")
	r.git("symbolic-ref", "HEAD", "refs/heads/main")
	r.git("config", "user.name", "Release Test")
	r.git("config", "user.email", "release-test@example.invalid")
	r.git("config", "commit.gpgsign", "false")
	writeReleaseFile(t, filepath.Join(r.dir, "scripts", "release-desktop.sh"), string(script), 0o755)
	// With TOUCH set it changes a committed file while it runs, as if something did behind its back; with ADD set it
	// adds that file.
	r.write("scripts/check-public.sh", "#!/bin/sh\necho '"+checkPublicRan+"'\n"+
		"if [ -n \"${TOUCH:-}\" ]; then echo changed >> README; fi\nif [ -n \"${ADD:-}\" ]; then echo added > \"$ADD\"; fi\n")
	if err := os.Chmod(filepath.Join(r.dir, "scripts", "check-public.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	r.write("README", "main\n")
	r.write("go.mod", "module example.invalid/release\n\ngo 1.21\n")
	r.write("web.go", "package smolcorp\n")
	r.write("cmd/smolcorp/main.go", "package main\n")
	r.write(tmuxTestFile, standInTmuxTest{}.source())
	r.git("add", ".")
	r.git("commit", "-q", "-m", "main")
	r.git("update-ref", "refs/remotes/origin/main", "HEAD")

	r.git("checkout", "-q", "-b", "work")
	for _, line := range []string{"one", "two"} {
		r.write("README", "main\n"+line+"\n")
		r.git("commit", "-q", "-am", line)
	}
	return r
}

// writeGo writes the stand-in go first on PATH. go version and go env go to the real go. go test with the release
// gate's variable is the script's run of its own tests: it prints what bin/rows-out holds and exits with
// bin/rows-status, or else says every heavy test passed (with bin/rows-stray there, it first leaves a file beside the
// code). go list and go test of the bundled tmux's test go to the real go when real is set, and are otherwise
// answered as a test keeping M7/L4's contract would answer them. Each go test it is given is logged in
// bin/go-test.log, with the variable and the first folder on PATH. Anything else would build: it says so and exits 97.
// With DROP_GO set, its first run also writes another go there, as something might while the script runs.
func (r *releaseRepo) writeGo(real bool) {
	r.t.Helper()
	goPath, _ := realGo(r.t)
	bin := func(name string) string { return "'" + filepath.Join(r.bin, name) + "'" }
	rows := ""
	for _, name := range heavyReleaseTests(r.t) {
		rows += "  echo '=== RUN   " + name + "'\n  echo '--- PASS: " + name + " (0.00s)'\n"
	}
	tmux := "list|test) exec '" + goPath + "' \"$@\";;\n"
	if !real {
		writeReleaseFile(r.t, filepath.Join(r.bin, "_testmain.go"), "\t{\""+tmuxTestName+"\", _test."+tmuxTestName+"},\n", 0o644)
		tmux = "list) printf 'example.invalid/release/desktop.test\\t%s\\n' " + bin("_testmain.go") + "; exit 0;;\n" +
			"test)\n" + tmuxContractAnswer + ";;\n"
	}
	writeReleaseFile(r.t, filepath.Join(r.bin, "go"), "#!/bin/sh\n"+
		"if [ -n \"${DROP_GO:-}\" ] && [ ! -e \"$DROP_GO\" ]; then cp "+bin("other-go")+" \"$DROP_GO\"; fi\n"+
		"case \"$1\" in version|env) exec '"+goPath+"' \"$@\";; esac\n"+
		"if [ \"$1\" = test ]; then\n"+
		"  echo \"rows=${"+releaseGateRows+"-unset} path=${PATH%%:*} $*\" >> "+bin("go-test.log")+"\n"+
		"fi\n"+
		"if [ \"$1\" = test ] && [ \"${"+releaseGateRows+":-}\" = 1 ]; then\n"+
		"  [ ! -e "+bin("rows-stray")+" ] || : > stray.txt\n"+
		"  if [ -e "+bin("rows-out")+" ]; then cat "+bin("rows-out")+"; exit \"$(cat "+bin("rows-status")+")\"; fi\n"+
		rows+"  echo PASS\n  printf 'ok  \\texample.invalid/release/desktop\\t0.01s\\n'\n  exit 0\n"+
		"fi\n"+
		"case \"$1\" in\n"+tmux+"esac\n"+
		"echo '"+goReached+"'\nexit 97\n", 0o755)
}

func writeReleaseFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

// write writes a file in the repo, at a path relative to it.
func (r *releaseRepo) write(path, body string) {
	r.t.Helper()
	writeReleaseFile(r.t, filepath.Join(r.dir, path), body, 0o644)
}

func (r *releaseRepo) git(args ...string) string {
	r.t.Helper()
	return r.gitIn("", args...)
}

// gitIn runs git with stdin as its input.
func (r *releaseRepo) gitIn(stdin string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env, cmd.Stdin = r.dir, r.env, strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// at checks out commit, detached, as the release is built.
func (r *releaseRepo) at(commit string) {
	r.t.Helper()
	r.git("checkout", "-q", "--detach", commit)
}

// squashed makes the release commit, the branch's files as one commit onto origin/main, and checks it out.
func (r *releaseRepo) squashed() string {
	r.t.Helper()
	commit := r.git("commit-tree", "-p", "origin/main", "-m", "Smolcorp 0.0.0", "work^{tree}")
	r.at(commit)
	return commit
}

// blind checks that git status really doesn't see what a case did, so only the script's other checks can.
func (r *releaseRepo) blind(what string) {
	r.t.Helper()
	if out := r.git("status", "--porcelain"); out != "" {
		r.t.Fatalf("git status sees %s: %s", what, out)
	}
}

// release runs release-desktop.sh with the extra environment given, and gives its exit status and output.
func (r *releaseRepo) release(extra ...string) (int, string) {
	r.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/bash", "scripts/release-desktop.sh")
	cmd.Dir, cmd.Env = r.dir, append(append([]string{}, r.env...), extra...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		r.t.Fatalf("release-desktop.sh didn't end within 30 seconds:\n%s", out)
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), string(out)
	} else if err != nil {
		r.t.Fatal(err)
	}
	return 0, string(out)
}

// refused checks that the script stopped with want, before check-public.sh and any build.
func (r *releaseRepo) refused(name, want string, extra ...string) {
	r.t.Helper()
	status, out := r.release(extra...)
	if status != 1 || !strings.Contains(out, want) || strings.Contains(out, checkPublicRan) || strings.Contains(out, goReached) {
		r.t.Errorf("%s: want a stop (%q) before check-public.sh and any build, got status %d:\n%s", name, want, status, out)
	}
}

// passes checks that the script got past every check, check-public.sh ran, and the next step was go's.
func (r *releaseRepo) passes(name string, extra ...string) {
	r.t.Helper()
	status, out := r.release(extra...)
	if status != 97 || !strings.Contains(out, checkPublicRan) || !strings.Contains(out, goReached) || strings.Contains(out, "Stopped: ") {
		r.t.Errorf("%s: want it past every check up to go, got status %d:\n%s", name, status, out)
	}
}

// releaseCase is one way HEAD or the files here can be, set up in a fresh repo; setup gives any extra environment to
// run the script with. want is the stop it must make before check-public.sh and any build, or "" when it must get
// past every check to go.
type releaseCase struct {
	name, want string
	setup      func(r *releaseRepo) []string
}

// runReleaseCases runs each case as a subtest, side by side.
func runReleaseCases(t *testing.T, cases []releaseCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newReleaseRepo(t)
			extra := c.setup(r)
			if c.want == "" {
				r.passes(c.name, extra...)
			} else {
				r.refused(c.name, c.want, extra...)
			}
		})
	}
}

func TestReleaseDesktopBuildsOnlyFromOneCommitOntoMain(t *testing.T) {
	heavyReleaseRows(t)
	// One repo, its commits one after another.
	t.Run("one repo", func(t *testing.T) {
		r := newReleaseRepo(t)
		work := r.git("rev-parse", "work")
		squashed := r.squashed()

		// The branch's files as one commit onto origin/main pass, check-public.sh runs, and the next step is go's.
		r.passes("one squashed commit on origin/main")

		// The branch itself, two commits past main.
		r.at(work)
		r.refused("the working branch, two commits past main", stopParents)
		if _, out := r.release(); !strings.Contains(out, "commit-tree -S -p origin/main") {
			t.Errorf("a refusal doesn't say how to make the release commit:\n%s", out)
		}

		// A merge of the branch into main: its first parent is origin/main, but it brings the history along.
		r.at(r.git("commit-tree", "-p", "origin/main", "-p", "work", "-m", "Merge work", "work^{tree}"))
		r.refused("a merge commit", stopParents)

		// A commit with no parent at all, the same files.
		r.at(r.git("commit-tree", "-m", "Smolcorp 0.0.0", "work^{tree}"))
		r.refused("a commit with no parent", stopParents)
		if _, out := r.release(); !strings.Contains(out, "HEAD's parents:\n  none\n") {
			t.Errorf("a commit with no parent: the refusal doesn't say it has none:\n%s", out)
		}

		// The squashed commit, after main moved on: it's onto an older main.
		main := r.git("rev-parse", "origin/main")
		r.git("update-ref", "refs/remotes/origin/main", r.git("commit-tree", "-p", main, "-m", "newer main", "main^{tree}"))
		r.at(squashed)
		r.refused("a squashed commit onto an older main", stopParents)

		// No origin/main to check against.
		r.git("update-ref", "-d", "refs/remotes/origin/main")
		r.refused("no origin/main", stopNoMain)
	})
}

// The other shapes HEAD and the checkout can have, each in a repo of its own.
func TestReleaseDesktopChecksEveryShapeOfHEAD(t *testing.T) {
	heavyReleaseRows(t)
	runReleaseCases(t, []releaseCase{
		{"a branch with one ordinary commit onto origin/main", "", func(r *releaseRepo) []string {
			r.git("checkout", "-q", "-B", "release", "origin/main")
			r.write("README", "main\nrelease\n")
			r.git("commit", "-q", "-am", "Smolcorp 0.0.0")
			return nil
		}},
		{"detached at origin/main", stopParents, func(r *releaseRepo) []string { r.at("origin/main"); return nil }},
		{"on main, the same commit as origin/main", stopParents, func(r *releaseRepo) []string {
			r.git("checkout", "-q", "main")
			return nil
		}},
		{"a merge with origin/main as its second parent", stopParents, func(r *releaseRepo) []string {
			r.at(r.git("commit-tree", "-p", "work", "-p", "origin/main", "-m", "Merge main", "work^{tree}"))
			return nil
		}},
		{"a local branch and a tag named origin/main, HEAD onto them", stopParents, func(r *releaseRepo) []string {
			r.git("branch", "origin/main", "work")
			r.git("tag", "origin/main", "work")
			r.at(r.git("commit-tree", "-p", "refs/heads/origin/main", "-m", "Smolcorp 0.0.0", "work^{tree}"))
			return nil
		}},
		{"a depth-1 clone, the release commit made in it", "", func(r *releaseRepo) []string {
			clone := r.t.TempDir()
			r.git("clone", "-q", "--depth", "1", "--branch", "main", "file://"+r.dir, clone)
			r.dir = clone
			r.git("config", "user.name", "Release Test")
			r.git("config", "user.email", "release-test@example.invalid")
			r.write("README", "main\nrelease\n")
			r.git("add", "README")
			r.at(r.git("commit-tree", "-p", "origin/main", "-m", "Smolcorp 0.0.0", r.git("write-tree")))
			return nil
		}},
		{"a committed file changed", stopClean, func(r *releaseRepo) []string {
			r.squashed()
			r.write("README", "not what goes out\n")
			return nil
		}},
		{"a stray ignored .syso beside the code", stopSyso, func(r *releaseRepo) []string {
			r.squashed()
			r.write(".git/info/exclude", "*.syso\n")
			r.write("cmd/smolcorp/rsrc_windows_amd64.syso", "not a resource\n")
			return nil
		}},
	})
}

// check-public.sh checks HEAD's tree: files changed or added while it ran stop the script before any build.
func TestReleaseDesktopStopsWhenTheCheckedFilesArentHEADs(t *testing.T) {
	heavyReleaseRows(t)
	for _, c := range []struct {
		name  string
		extra []string
	}{
		{"a committed file changed under check-public.sh", []string{"TOUCH=1"}},
		{"a Go file added under check-public.sh, status set not to list untracked files",
			[]string{"ADD=cmd/smolcorp/late.go", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=status.showUntrackedFiles", "GIT_CONFIG_VALUE_0=no"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newReleaseRepo(t)
			r.squashed()
			status, out := r.release(c.extra...)
			if status != 1 || !strings.Contains(out, checkPublicRan) || !strings.Contains(out, "didn't check HEAD's tree") || strings.Contains(out, goReached) {
				t.Errorf("want a stop before any build, got status %d:\n%s", status, out)
			}
		})
	}
}

// Files that aren't HEAD's but would be built: untracked ones git status is set not to list, Go code git ignores
// beside the code, and a folder git is pointed at instead of this one. Each case is one squashed commit onto
// origin/main, in a repo of its own.
func TestReleaseDesktopStopsOnFilesThatDontGoOut(t *testing.T) {
	heavyReleaseRows(t)
	untracked := func(config ...string) func(r *releaseRepo) []string {
		return func(r *releaseRepo) []string {
			r.squashed()
			if len(config) > 0 {
				r.git(append([]string{"config"}, config...)...)
			}
			r.write("cmd/smolcorp/extra.go", "package main\n")
			if len(config) > 0 {
				r.blind("the untracked file")
			}
			return nil
		}
	}
	ignored := func(path string) func(r *releaseRepo) []string {
		return func(r *releaseRepo) []string {
			r.squashed()
			r.write(".git/info/exclude", "*.go\n*.s\n/notes/\n")
			r.write(path, "package main\n")
			r.blind(path)
			return nil
		}
	}
	runReleaseCases(t, []releaseCase{
		{"an untracked Go file", stopClean, untracked()},
		{"an untracked Go file, status.showUntrackedFiles no", stopClean, untracked("status.showUntrackedFiles", "no")},
		{"an untracked Go file, status.showUntrackedFiles no for the account", stopClean,
			untracked("--global", "status.showUntrackedFiles", "no")},
		{"an untracked Go file, status.showUntrackedFiles no from the environment", stopClean, func(r *releaseRepo) []string {
			untracked()(r)
			return []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=status.showUntrackedFiles", "GIT_CONFIG_VALUE_0=no"}
		}},
		{"status.showUntrackedFiles no, nothing extra", "", func(r *releaseRepo) []string {
			r.squashed()
			r.git("config", "status.showUntrackedFiles", "no")
			return nil
		}},
		{"HEAD a commit git can't read, so git status fails", stopClean, func(r *releaseRepo) []string {
			r.squashed()
			bogus := r.gitIn("parent "+r.git("rev-parse", "origin/main")+"\ntree "+r.git("rev-parse", "work^{tree}")+
				"\nauthor A <a@example.invalid> 1 +0000\ncommitter A <a@example.invalid> 1 +0000\n\nSmolcorp 0.0.0\n",
				"hash-object", "-t", "commit", "--literally", "-w", "--stdin")
			// git won't point HEAD at it itself ("bogus commit object"), so the file is written by hand.
			r.write(".git/HEAD", bogus+"\n")
			return nil
		}},
		{"an ignored Go file in a package folder", stopIgnoredGo, ignored("cmd/smolcorp/extra.go")},
		{"an ignored Go file in the top folder's package", stopIgnoredGo, ignored("extra.go")},
		{"an ignored assembly file in a package folder", stopIgnoredGo, ignored("cmd/smolcorp/extra_amd64.s")},
		{"a Go file ignored for the account in a package folder", stopIgnoredGo, func(r *releaseRepo) []string {
			r.squashed()
			writeReleaseFile(r.t, filepath.Join(r.home, "excludes"), "*.go\n", 0o644)
			r.git("config", "--global", "core.excludesFile", filepath.Join(r.home, "excludes"))
			r.write("cmd/smolcorp/extra.go", "package main\n")
			r.blind("the ignored file")
			return nil
		}},
		{"an ignored Go file in a folder of notes, not a package", "", ignored("notes/scratch.go")},
		{"GIT_WORK_TREE naming a clean worktree, a file changed here", stopClean, func(r *releaseRepo) []string {
			other := filepath.Join(r.t.TempDir(), "other")
			r.git("worktree", "add", "-q", "--detach", other, r.squashed())
			r.write("README", "not what goes out\n")
			return []string{"GIT_WORK_TREE=" + other}
		}},
		{"core.worktree naming a clean copy, a file changed here", stopWorktree, func(r *releaseRepo) []string {
			other := r.t.TempDir()
			r.git("--work-tree="+other, "checkout", "-q", r.squashed(), "--", ".")
			r.write("README", "not what goes out\n")
			r.git("config", "core.worktree", other)
			r.blind("the changed file")
			return nil
		}},
	})
}

// HEAD is read as a push sends it: a replace ref or a grafts file that makes the branch look like one commit onto
// origin/main doesn't get it past the check, however git is told to use them, and a commit object is read as git
// reads it. Each case is in a repo of its own, on the branch "work" (two commits past main) unless it says otherwise.
func TestReleaseDesktopReadsHEADAsAPushSendsIt(t *testing.T) {
	heavyReleaseRows(t)
	useReplaceRefs := []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.useReplaceRefs", "GIT_CONFIG_VALUE_0=true"}
	graft := func(config ...string) func(r *releaseRepo) []string {
		return func(r *releaseRepo) []string {
			if len(config) > 0 {
				r.git(append([]string{"config"}, config...)...)
			}
			r.git("replace", "--graft", "HEAD", "origin/main")
			return nil
		}
	}
	fullReplace := func(r *releaseRepo) []string {
		r.git("config", "core.useReplaceRefs", "true")
		r.git("replace", "work", r.git("commit-tree", "-p", "origin/main", "-m", "Smolcorp 0.0.0", "main^{tree}"))
		r.git("reset", "-q", "--hard")
		return nil
	}
	// altBase makes a graft under refs/alt/ and runs the script with that as the replace refs' home.
	altBase := func(config ...string) func(r *releaseRepo) []string {
		return func(r *releaseRepo) []string {
			if len(config) > 0 {
				r.git(append([]string{"config"}, config...)...)
			}
			r.git("update-ref", "refs/alt/"+r.git("rev-parse", "work"),
				r.git("commit-tree", "-p", "origin/main", "-m", "Smolcorp 0.0.0", "work^{tree}"))
			return []string{"GIT_REPLACE_REF_BASE=refs/alt/"}
		}
	}
	graftsLine := func(r *releaseRepo) string {
		return r.git("rev-parse", "work") + " " + r.git("rev-parse", "origin/main") + "\n"
	}
	// elsewhere is a bare clone holding a graft of HEAD onto origin/main, to borrow objects from.
	elsewhere := func(r *releaseRepo) string {
		bare := filepath.Join(r.t.TempDir(), "bare.git")
		r.git("clone", "-q", "--bare", r.dir, bare)
		r.git("--git-dir="+bare, "replace", "--graft", r.git("rev-parse", "work"), r.git("rev-parse", "origin/main"))
		return filepath.Join(bare, "objects")
	}
	// object makes a commit object by hand, header lines as given, and checks it out.
	object := func(header ...string) func(r *releaseRepo) []string {
		return func(r *releaseRepo) []string {
			body := strings.Join(header, "\n")
			for _, w := range [][2]string{{"TREE", r.git("rev-parse", "work^{tree}")}, {"BASE", r.git("rev-parse", "origin/main")}, {"OTHER", r.git("rev-parse", "work~1")}} {
				body = strings.ReplaceAll(body, w[0], w[1])
			}
			r.at(r.gitIn(body, "hash-object", "-t", "commit", "--literally", "-w", "--stdin"))
			return nil
		}
	}
	const who = "Release Test <release-test@example.invalid> 1 +0000"
	runReleaseCases(t, []releaseCase{
		{"a replace ref grafting HEAD onto origin/main", stopParents, graft()},
		{"a replace ref putting a squashed commit in HEAD's place, checked out", stopParents, fullReplace},
		{"a grafts file giving HEAD origin/main as its parent", stopParents, func(r *releaseRepo) []string {
			r.write(r.git("rev-parse", "--git-path", "info/grafts"), graftsLine(r))
			return nil
		}},
		{"a graft, core.useReplaceRefs true", stopParents, graft("core.useReplaceRefs", "true")},
		{"a graft, core.useReplaceRefs false", stopParents, graft("core.useReplaceRefs", "false")},
		{"a graft, core.useReplaceRefs true from the environment", stopParents, func(r *releaseRepo) []string {
			graft()(r)
			return useReplaceRefs
		}},
		{"a graft, GIT_NO_REPLACE_OBJECTS=0 from the caller", stopParents, func(r *releaseRepo) []string {
			graft()(r)
			return []string{"GIT_NO_REPLACE_OBJECTS=0"}
		}},
		{"a graft under GIT_REPLACE_REF_BASE", stopParents, altBase()},
		{"a graft under GIT_REPLACE_REF_BASE, core.useReplaceRefs true", stopParents, altBase("core.useReplaceRefs", "true")},
		{"GIT_GRAFT_FILE giving HEAD origin/main as its parent", stopParents, func(r *releaseRepo) []string {
			writeReleaseFile(r.t, filepath.Join(r.home, "grafts"), graftsLine(r), 0o644)
			return []string{"GIT_GRAFT_FILE=" + filepath.Join(r.home, "grafts")}
		}},
		{"objects borrowed from a repo with a graft (info/alternates)", stopParents, func(r *releaseRepo) []string {
			r.write(".git/objects/info/alternates", elsewhere(r)+"\n")
			return nil
		}},
		{"objects borrowed from a repo with a graft (GIT_ALTERNATE_OBJECT_DIRECTORIES)", stopParents, func(r *releaseRepo) []string {
			return []string{"GIT_ALTERNATE_OBJECT_DIRECTORIES=" + elsewhere(r)}
		}},
		{"the release commit, its tree replaced by main's and checked out", stopClean, func(r *releaseRepo) []string {
			squashed := r.squashed()
			r.git("replace", squashed+"^{tree}", r.git("rev-parse", "main^{tree}"))
			r.git("reset", "-q", "--hard")
			return nil
		}},
		{"the release commit replaced by one with main's tree, checked out", stopClean, func(r *releaseRepo) []string {
			squashed := r.squashed()
			r.git("replace", squashed, r.git("commit-tree", "-p", "origin/main", "-m", "Smolcorp 0.0.0", "main^{tree}"))
			r.git("reset", "-q", "--hard")
			return nil
		}},
		{"a commit object with a signature", "", object("tree TREE", "parent BASE", "author "+who, "committer "+who,
			"gpgsig -----BEGIN SSH SIGNATURE-----", " U1NIU0lH", " ", " -----END SSH SIGNATURE-----", "", "Smolcorp 0.0.0", "")},
		{"a commit object with a second parent line after committer", stopParents,
			object("tree TREE", "parent BASE", "author "+who, "committer "+who, "parent OTHER", "", "Smolcorp 0.0.0", "")},
		{"a commit object with its parent twice", stopParents,
			object("tree TREE", "parent BASE", "parent BASE", "author "+who, "committer "+who, "", "Smolcorp 0.0.0", "")},
		{"a commit object with a second parent line after an unknown one", stopParents,
			object("tree TREE", "parent BASE", "author "+who, "committer "+who, "x-note y", "parent OTHER", "", "Smolcorp 0.0.0", "")},
		{"a commit object with a parent line in its message only", "",
			object("tree TREE", "parent BASE", "author "+who, "committer "+who, "", "Smolcorp 0.0.0", "", "parent OTHER", "")},
		{"a commit object with no message", "", object("tree TREE", "parent BASE", "author "+who, "committer "+who, "")},
		{"a commit object with two tree lines", stopTree,
			object("tree TREE", "parent BASE", "author "+who, "committer "+who, "tree "+"4b825dc642cb6eb9a060e54bf8d69288fbee4904", "", "Smolcorp 0.0.0", "")},
	})

	// A really signed release commit passes: the signature doesn't hide its parent.
	t.Run("a release commit signed with an SSH key", func(t *testing.T) {
		t.Parallel()
		keygen, err := exec.LookPath("ssh-keygen")
		if err != nil {
			t.Log("no ssh-keygen: the signed commit isn't tried")
			return
		}
		r := newReleaseRepo(t)
		key := filepath.Join(r.home, "release-test-key")
		if out, err := exec.Command(keygen, "-q", "-t", "ed25519", "-N", "", "-C", "release-test", "-f", key).CombinedOutput(); err != nil {
			t.Fatalf("ssh-keygen: %v\n%s", err, out)
		}
		r.git("config", "gpg.format", "ssh")
		r.git("config", "user.signingkey", key)
		signed := r.git("commit-tree", "-S", "-p", "origin/main", "-m", "Smolcorp 0.0.0", "work^{tree}")
		if !strings.Contains(r.git("cat-file", "commit", signed), "gpgsig ") {
			t.Fatal("commit-tree -S made no signature")
		}
		r.at(signed)
		r.passes("a release commit signed with an SSH key")
	})
}

// Files git is told not to look at: their changes on disk pass git status unseen, so the script stops before
// check-public.sh and any build, and says what to do. Each case is one squashed commit onto origin/main, in a repo of
// its own.
func TestReleaseDesktopStopsOnFilesGitDoesntLookAt(t *testing.T) {
	heavyReleaseRows(t)
	edited := func(flags ...string) func(r *releaseRepo) []string {
		return func(r *releaseRepo) []string {
			r.squashed()
			r.git(append(append([]string{"update-index"}, flags...), "README")...)
			r.write("README", "not what goes out\n")
			r.blind("the edit")
			return nil
		}
	}
	sparse := func(args ...string) func(r *releaseRepo) []string {
		return func(r *releaseRepo) []string {
			r.squashed()
			r.git(append([]string{"sparse-checkout", "set"}, args...)...)
			return nil
		}
	}
	for _, c := range []releaseCase{
		{"an assume-unchanged file, edited", "--no-assume-unchanged", edited("--assume-unchanged")},
		{"a skip-worktree file, edited", "--no-skip-worktree", edited("--skip-worktree")},
		{"a file both assume-unchanged and skip-worktree, edited", "--no-assume-unchanged", edited("--assume-unchanged", "--skip-worktree")},
		{"a skip-worktree file, deleted", "--no-skip-worktree", func(r *releaseRepo) []string {
			r.squashed()
			r.git("update-index", "--skip-worktree", "README")
			if err := os.Remove(filepath.Join(r.dir, "README")); err != nil {
				r.t.Fatal(err)
			}
			r.blind("the deletion")
			return nil
		}},
		{"a sparse checkout leaving README out", "sparse-checkout disable", sparse("--no-cone", "/scripts/")},
		{"a sparse checkout of every file", "sparse-checkout disable", sparse("--no-cone", "/*")},
		{"a cone sparse checkout", "sparse-checkout disable", sparse("--cone", "scripts")},
		{"a cone sparse checkout with a sparse index", "sparse-checkout disable", sparse("--cone", "--sparse-index", "scripts")},
		{"a sparse checkout turned off with its files still skipped", "--no-skip-worktree", func(r *releaseRepo) []string {
			sparse("--no-cone", "/scripts/")(r)
			r.git("config", "--worktree", "core.sparseCheckout", "false")
			r.blind("the skipped README")
			return nil
		}},
		{"core.ignoreStat, a file checked out again then edited", "--no-assume-unchanged", func(r *releaseRepo) []string {
			r.squashed()
			r.git("config", "core.ignoreStat", "true")
			if err := os.Remove(filepath.Join(r.dir, "README")); err != nil {
				r.t.Fatal(err)
			}
			r.git("checkout", "--", "README")
			r.write("README", "not what goes out\n")
			r.blind("the edit")
			return nil
		}},
	} {
		// want is the fix the stop names; the stop itself is the one for HEAD's tree.
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newReleaseRepo(t)
			c.setup(r)
			r.refused(c.name, c.want)
			if _, out := r.release(); !strings.Contains(out, stopTree) {
				t.Errorf("want the stop for HEAD's tree:\n%s", out)
			}
		})
	}
}
