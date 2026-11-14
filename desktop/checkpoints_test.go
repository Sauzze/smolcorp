package desktop

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The checkpoint store (checkpoints.go) on real throwaway repos: signing off in their local config only, and every
// temp file in a folder of the test's own.

func cpDesk(dir string) checkpointDesk { return checkpointDesk{Session: "s1", Folder: dir} }

// cpScratch points the temp folder at a folder of this test, and says what is left in it and in the store's own
// folder (in the tests' support folder): nothing, after a snapshot, in either.
func cpScratch(t *testing.T) func() []string {
	t.Helper()
	dir := physical(t.TempDir())
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	return func() []string {
		var names []string
		for _, folder := range []string{dir, cpFolder()} {
			entries, _ := os.ReadDir(folder)
			for _, e := range entries {
				names = append(names, filepath.Join(folder, e.Name()))
			}
		}
		return names
	}
}

func newSnap(t *testing.T, s *checkpointStore, desk checkpointDesk, label string) Checkpoint {
	t.Helper()
	cp, note := s.snapshot(desk, label)
	if note != "" || cp.ID == "" {
		t.Fatalf("the snapshot was skipped: %q", note)
	}
	return cp
}

// cpFiles is what a checkpoint holds: path → content.
func cpFiles(t *testing.T, repo, id string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, path := range strings.Split(gitRepoCmd(t, repo, "ls-tree", "-r", "--name-only", id), "\n") {
		if path != "" {
			files[path] = gitRepoCmd(t, repo, "show", id+":"+path)
		}
	}
	return files
}

// ownerState is everything of the owner's that a snapshot must leave as it is: the index (its bytes and its
// time), HEAD, the config, every ref but the checkpoints', the stash, the reflogs, the status, the files of the
// working folder, and what else sits in the git folder. Read with no optional locks, so reading it writes nothing.
func ownerState(t *testing.T, dir string) map[string]string {
	t.Helper()
	state := map[string]string{}
	digest := func(path string) string {
		data, err := os.ReadFile(path)
		if err != nil {
			return "missing"
		}
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
	git := filepath.Join(dir, ".git")
	for _, name := range []string{"index", "HEAD", "config", "logs/HEAD", "refs/stash", "logs/refs/stash", "packed-refs", "info/exclude"} {
		state[name] = digest(filepath.Join(git, name))
	}
	if info, err := os.Stat(filepath.Join(git, "index")); err == nil {
		state["index mtime"] = info.ModTime().String() + " " + strconv.FormatInt(info.Size(), 10)
	}
	refs := gitRepoCmd(t, dir, "for-each-ref", "--format=%(refname) %(objectname)")
	var kept []string
	for _, line := range strings.Split(refs, "\n") {
		if !strings.HasPrefix(line, "refs/smolcorp/") {
			kept = append(kept, line)
		}
	}
	state["refs"] = strings.Join(kept, "\n")
	state["stash"] = gitRepoCmd(t, dir, "stash", "list")
	state["reflog"] = gitRepoCmd(t, dir, "reflog", "show", "--format=%H %gs", "HEAD")
	state["status"] = gitRepoCmd(t, dir, "--no-optional-locks", "status", "--porcelain=v2", "--branch", "--untracked-files=all")
	top := []string{}
	entries, _ := os.ReadDir(git)
	for _, e := range entries {
		top = append(top, e.Name())
	}
	state["git folder"] = strings.Join(top, " ")
	var tree []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if info, err := d.Info(); err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			tree = append(tree, rel+" "+info.ModTime().String()+" "+strconv.FormatInt(info.Size(), 10)+" "+digest(path))
		}
		return nil
	})
	state["files"] = strings.Join(tree, "\n")
	return state
}

func sameState(t *testing.T, before, after map[string]string) {
	t.Helper()
	for key, was := range before {
		if after[key] != was {
			t.Errorf("%s changed:\n was %q\n now %q", key, was, after[key])
		}
	}
}

// aBusyRepo is a repo with a commit, a stash, a staged change, an unstaged one, an untracked file and an ignored one.
func aBusyRepo(t *testing.T) string {
	t.Helper()
	dir := aGitRepo(t)
	writeRepoFile(t, dir, ".gitignore", "*.log\nbuild/\n")
	gitRepoCmd(t, dir, "add", ".gitignore")
	gitRepoCmd(t, dir, "commit", "-q", "-m", "ignore")
	writeRepoFile(t, dir, "stashed.txt", "to the stash\n")
	gitRepoCmd(t, dir, "add", "stashed.txt")
	gitRepoCmd(t, dir, "stash", "push", "-q", "-m", "owner's stash")
	gitRepoCmd(t, dir, "branch", "other")
	gitRepoCmd(t, dir, "tag", "v1")
	writeRepoFile(t, dir, "staged.txt", "staged\n")
	gitRepoCmd(t, dir, "add", "staged.txt")
	writeRepoFile(t, dir, "a.txt", "one\nTWO\nthree\n")
	writeRepoFile(t, dir, "new/untracked.txt", "untracked\n")
	writeRepoFile(t, dir, "debug.log", "ignored\n")
	writeRepoFile(t, dir, "build/out.bin", "ignored too\n")
	return dir
}

func TestASnapshotHoldsTheWorkingTreeAsItIs(t *testing.T) {
	left := cpScratch(t)
	dir := aBusyRepo(t)
	// A file the repo tracks but its own .gitignore names stays in; the owner's ignore file hides what it names.
	writeRepoFile(t, dir, "forced.log", "tracked though ignored\n")
	gitRepoCmd(t, dir, "add", "-f", "forced.log")
	gitRepoCmd(t, dir, "commit", "-q", "-m", "force")
	writeRepoFile(t, dir, "forced.log", "tracked though ignored, changed\n")
	writeRepoFile(t, dir, "notes.secret", "the owner's own ignore file\n")
	writeRepoFile(t, dir, "crlf.txt", "a\r\nb\r\n")
	if err := os.Remove(filepath.Join(dir, "src", "b.txt")); err != nil {
		t.Fatal(err)
	}
	gitRepoCmd(t, dir, "config", "core.autocrlf", "true") // a conversion the snapshot must not apply
	ignore := filepath.Join(physical(t.TempDir()), "ignore")
	writeRepoFile(t, filepath.Dir(ignore), filepath.Base(ignore), "*.secret\n")
	withGitHere(t, gitHereInfo{version: "git version 2.55.0", owner: [][2]string{{"core.excludesfile", ignore}}})

	store := newCheckpoints()
	cp := newSnap(t, store, cpDesk(dir), "before the turn")
	if cp.N != 1 || cp.Label != "before the turn" || cp.Head != gitRepoCmd(t, dir, "rev-parse", "HEAD") || time.Since(time.UnixMilli(cp.At)) > time.Minute {
		t.Errorf("%+v", cp)
	}
	if gitRepoCmd(t, dir, "rev-parse", checkpointRefs+"s1/1") != cp.ID {
		t.Error("the ref doesn't name the commit")
	}
	got := cpFiles(t, dir, cp.ID)
	want := map[string]string{".gitignore": "*.log\nbuild/", "a.txt": "one\nTWO\nthree", "staged.txt": "staged", "new/untracked.txt": "untracked",
		"forced.log": "tracked though ignored, changed", "crlf.txt": "a\r\nb"}
	// (git show output is trimmed by gitRepoCmd; the CRLF inside is whole)
	want["a.txt"] = "one\nTWO\nthree"
	for _, name := range []string{"stashed.txt", "debug.log", "build/out.bin", "notes.secret", "src/b.txt"} {
		if _, in := got[name]; in {
			t.Errorf("%s is in the snapshot", name)
		}
	}
	for name, text := range want {
		if got[name] != text {
			t.Errorf("%s = %q, want %q", name, got[name], text)
		}
	}
	if gitRepoCmd(t, dir, "cat-file", "-p", cp.ID) == "" || !strings.Contains(gitRepoCmd(t, dir, "log", "-1", "--format=%an <%ae> %P|%s", cp.ID), "Smolcorp checkpoint <checkpoint@smolcorp.invalid> |smolcorp-checkpoint v1") {
		t.Error("the commit has a parent or another author")
	}
	if names := left(); len(names) != 0 {
		t.Errorf("a private index was left behind: %v", names)
	}
}

func TestEveryLineEndingIsKeptWhateverTheConfigAndAttributesSay(t *testing.T) {
	cpScratch(t)
	dir := aGitRepo(t)
	gitRepoCmd(t, dir, "config", "core.autocrlf", "true")
	gitRepoCmd(t, dir, "config", "core.safecrlf", "true")
	writeRepoFile(t, dir, "win.txt", "a\r\nb\r\n")
	withAttrSource(t, true)
	cp := newSnap(t, newCheckpoints(), cpDesk(dir), "x")
	if size := gitRepoCmd(t, dir, "cat-file", "-s", cp.ID+":win.txt"); size != "6" {
		t.Errorf("win.txt is %s bytes, not the 6 on disk", size)
	}
}

func TestTheOwnersIndexHeadStashReflogAndConfigAreLeftAsTheyWere(t *testing.T) {
	// The second run has a split index: a write of any index of it would leave a sharedindex.* file in the git folder.
	for _, split := range []bool{false, true} {
		t.Run("split index "+strconv.FormatBool(split), func(t *testing.T) {
			cpScratch(t)
			dir := aBusyRepo(t)
			gitRepoCmd(t, dir, "config", "core.logAllRefUpdates", "always") // a checkpoint ref still gets no reflog
			if split {
				gitRepoCmd(t, dir, "config", "core.splitIndex", "true")
				gitRepoCmd(t, dir, "update-index", "--split-index")
				if shared, _ := filepath.Glob(filepath.Join(dir, ".git", "sharedindex.*")); len(shared) != 1 {
					t.Fatalf("the repo has no split index: %v", shared)
				}
			}
			before := ownerState(t, dir)
			if before["stash"] == "" || before["index"] == "missing" || before["refs"] == "" {
				t.Fatalf("the repo is not as busy as the test needs: %v", before)
			}
			store := newCheckpoints()
			first := newSnap(t, store, cpDesk(dir), "one")
			newSnap(t, store, cpDesk(dir), "two")
			sameState(t, before, ownerState(t, dir))
			if _, err := os.Stat(filepath.Join(dir, ".git", "logs", "refs", "smolcorp")); err == nil {
				t.Error("a checkpoint ref has a reflog")
			}
			// The same through every other call.
			store.list(cpDesk(dir))
			store.between(cpDesk(dir), first.ID, first.ID)
			store.keepOnly(cpDesk(dir), 1)
			store.forget(cpDesk(dir))
			sameState(t, before, ownerState(t, dir))
			if shared, _ := filepath.Glob(filepath.Join(dir, ".git", "sharedindex.*")); split && len(shared) != 1 {
				t.Errorf("a shared index file was added: %v", shared)
			}
			if refs := gitRepoCmd(t, dir, "for-each-ref", "refs/smolcorp"); refs != "" {
				t.Errorf("checkpoint refs are left after forget: %s", refs)
			}
		})
	}
}

func TestTheOwnersIndexIsNotEvenRefreshedWhenAFileChangesUnderIt(t *testing.T) {
	cpScratch(t)
	dir := aBusyRepo(t)
	// Touching a tracked file makes its index entry stale: a plain `git add` or `status` would rewrite the index.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "a.txt"), future, future); err != nil {
		t.Fatal(err)
	}
	before := ownerState(t, dir)
	newSnap(t, newCheckpoints(), cpDesk(dir), "x")
	sameState(t, before, ownerState(t, dir))
}

// The control and the proof in one: a hostile repo (aHostileGitRepo) whose filters, hooks, fsmonitor and the like all
// write a marker. The same commands of a plain git run them; the store runs none.
func TestASnapshotRunsNothingTheRepoNames(t *testing.T) {
	for _, mode := range []struct {
		name string
		on   bool
	}{{"with an attribute source", true}, {"without one", false}} {
		t.Run(mode.name, func(t *testing.T) {
			cpScratch(t)
			h := aHostileGitRepo(t)
			if out, err := exec.Command("git", "version").Output(); err != nil || (mode.on && !gitVersionAtLeast(string(out), 2, 40)) {
				t.Skip("this git has no --attr-source")
			}
			// The control: plain git, the same steps, on the same repo.
			index := filepath.Join(t.TempDir(), "index")
			for _, args := range [][]string{{"add", "-A"}, {"update-ref", "refs/control", "HEAD"}, {"status", "--porcelain"}} {
				cmd := exec.Command("git", append([]string{"-C", h.dir}, args...)...)
				cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
				if args[0] == "status" {
					cmd.Env = os.Environ()
				}
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("%v: %v %s", args, err, out)
				}
			}
			gitRepoCmd(t, h.dir, "update-ref", "-d", "refs/control")
			ran := h.ran()
			for _, name := range []string{"process", "included", "infoclean", "equalsclean", "global", "env", "hook", "includedmonitor"} {
				if !slices.Contains(ran, name) {
					t.Fatalf("the control repo didn't run %s (%v), so this test proves less than it says", name, ran)
				}
			}
			withAttrSource(t, mode.on)
			before := ownerState(t, h.dir)
			h.ran()
			store := newCheckpoints()
			cp, note := store.snapshot(cpDesk(h.dir), "hostile")
			if note != "" {
				t.Fatalf("the hostile repo was refused: %s", note)
			}
			store.list(cpDesk(h.dir))
			store.snapshot(cpDesk(h.dir), "hostile again")
			store.keepOnly(cpDesk(h.dir), 1)
			store.forget(cpDesk(h.dir))
			if ran := h.ran(); len(ran) != 0 {
				t.Errorf("the store ran: %v", ran)
			}
			if cp.ID == "" {
				t.Error("no checkpoint")
			}
			sameState(t, before, ownerState(t, h.dir))
		})
	}
}

// A signature would run gpg.program; and commit.gpgsign=true is what a repo says to ask for one.
func TestASnapshotIsNeverSignedAndRunsNoProgramForIt(t *testing.T) {
	cpScratch(t)
	h := aHostileGitRepo(t)
	gitRepoCmd(t, h.dir, "config", "commit.gpgsign", "true")
	gitRepoCmd(t, h.dir, "config", "gpg.program", h.program(t, "gpg"))
	withAttrSource(t, true)
	cp, note := newCheckpoints().snapshot(cpDesk(h.dir), "unsigned")
	if note != "" || cp.ID == "" {
		t.Fatalf("%q", note)
	}
	if body := gitRepoCmd(t, h.dir, "cat-file", "-p", cp.ID); strings.Contains(body, "gpgsig") {
		t.Error("the commit is signed")
	}
	if ran := h.ran(); slices.Contains(ran, "gpg") {
		t.Errorf("gpg ran: %v", ran)
	}
}

func TestARepoWithAKeyTheStoreDoesNotKnowIsLeftAlone(t *testing.T) {
	left := cpScratch(t)
	dir := aGitRepo(t)
	gitRepoCmd(t, dir, "config", "core.alternateRefsCommand", "something")
	cp, note := newCheckpoints().snapshot(cpDesk(dir), "x")
	if cp.ID != "" || !strings.Contains(note, "core.alternaterefscommand") {
		t.Errorf("%+v %q", cp, note)
	}
	if refs := gitRepoCmd(t, dir, "for-each-ref", "refs/smolcorp"); refs != "" || len(left()) != 0 {
		t.Errorf("%q %v", refs, left())
	}
}

func TestAFolderOutsideARepoAnOldGitAndNoGitAreSaidNotHidden(t *testing.T) {
	cpScratch(t)
	dir := aGitRepo(t)
	store := newCheckpoints()
	if _, note := store.snapshot(cpDesk(physical(t.TempDir())), "x"); !strings.Contains(note, "isn't in a git repo") {
		t.Errorf("%q", note)
	}
	if _, note := store.snapshot(checkpointDesk{Session: "s1"}, "x"); !strings.Contains(note, "no folder") {
		t.Errorf("%q", note)
	}
	if _, note := store.snapshot(cpDesk(filepath.Join(dir, "nowhere")), "x"); !strings.Contains(note, "folder is gone") {
		t.Errorf("%q", note)
	}
	withGitHere(t, gitHereInfo{version: "git version 2.31.0"})
	if _, note := store.snapshot(cpDesk(dir), "x"); !strings.Contains(note, "older than 2.32") {
		t.Errorf("%q", note)
	}
	withGitHere(t, gitHereInfo{})
	if _, note := store.snapshot(cpDesk(dir), "x"); !strings.Contains(note, "isn't installed") {
		t.Errorf("%q", note)
	}
	if refs := gitRepoCmd(t, dir, "for-each-ref", "refs/smolcorp"); refs != "" {
		t.Errorf("a skipped snapshot left a ref: %s", refs)
	}
}

func TestARepoBeforeItsFirstCommitAndAFolderInsideARepoAreSnapshotted(t *testing.T) {
	cpScratch(t)
	dir := physical(t.TempDir())
	gitRepoCmd(t, dir, "init", "-q", "-b", "main")
	writeRepoFile(t, dir, "first.txt", "hello\n")
	store := newCheckpoints()
	cp := newSnap(t, store, cpDesk(dir), "unborn")
	if cp.Head != "" || cpFiles(t, dir, cp.ID)["first.txt"] != "hello" {
		t.Errorf("%+v", cp)
	}
	// The desk works in a subfolder of the repo: the whole repo is still what is snapshotted.
	repo := aGitRepo(t)
	writeRepoFile(t, repo, "src/deep/new.txt", "x\n")
	sub := newSnap(t, store, checkpointDesk{Session: "s2", Folder: filepath.Join(repo, "src")}, "sub")
	if files := cpFiles(t, repo, sub.ID); files["a.txt"] == "" || files["src/deep/new.txt"] != "x" {
		t.Errorf("%v", files)
	}
}

func TestASha256RepoIsSnapshotted(t *testing.T) {
	cpScratch(t)
	dir := physical(t.TempDir())
	if out, err := exec.Command("git", "-C", dir, "init", "-q", "--object-format=sha256", "-b", "main").CombinedOutput(); err != nil {
		t.Skipf("no sha256 repos here: %s", out)
	}
	gitRepoCmd(t, dir, "config", "commit.gpgsign", "false")
	writeRepoFile(t, dir, "a.txt", "a\n")
	gitRepoCmd(t, dir, "add", ".")
	gitRepoCmd(t, dir, "commit", "-q", "-m", "first")
	writeRepoFile(t, dir, "b.txt", "b\n")
	withAttrSource(t, true)
	store := newCheckpoints()
	one := newSnap(t, store, cpDesk(dir), "one")
	if len(one.ID) != 64 || one.Head == "" || cpFiles(t, dir, one.ID)["b.txt"] != "b" {
		t.Errorf("%+v", one)
	}
	if list, note := store.list(cpDesk(dir)); note != "" || len(list) != 1 || list[0] != one {
		t.Errorf("%v %q", list, note)
	}
}

func TestListAndBetweenNameWhatChanged(t *testing.T) {
	cpScratch(t)
	dir := aBusyRepo(t)
	store := newCheckpoints()
	one := newSnap(t, store, cpDesk(dir), "start")
	writeRepoFile(t, dir, "a.txt", "changed\n")        // modified
	writeRepoFile(t, dir, "fresh/c.txt", "c\n")        // added
	writeRepoFile(t, dir, "debug.log", "still hidden") // ignored: not a change
	if err := os.Remove(filepath.Join(dir, "src", "b.txt")); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Remove(filepath.Join(dir, "staged.txt")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("a.txt", filepath.Join(dir, "staged.txt")); err != nil { // a file that became a link
			t.Fatal(err)
		}
	}
	two := newSnap(t, store, cpDesk(dir), "end")
	list, note := store.list(cpDesk(dir))
	if note != "" || len(list) != 2 || list[0] != one || list[1] != two || two.N != 2 {
		t.Fatalf("%v %q", list, note)
	}
	changes, note := store.between(cpDesk(dir), one.ID, two.ID)
	if note != "" {
		t.Fatal(note)
	}
	var got []string
	for _, c := range changes {
		got = append(got, c.Status+" "+filepath.ToSlash(c.Path))
	}
	want := []string{"M a.txt", "A fresh/c.txt", "D src/b.txt"}
	if runtime.GOOS != "windows" {
		want = []string{"M a.txt", "A fresh/c.txt", "D src/b.txt", "T staged.txt"}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("%v, want %v", got, want)
	}
	if same, note := store.between(cpDesk(dir), one.ID, one.ID); note != "" || len(same) != 0 {
		t.Errorf("a checkpoint against itself: %v %q", same, note)
	}
	// A path is carried exactly, spaces and non-ASCII included.
	writeRepoFile(t, dir, "dir with space/é — ü.txt", "x\n")
	three := newSnap(t, store, cpDesk(dir), "end")
	if changes, _ := store.between(cpDesk(dir), two.ID, three.ID); len(changes) != 1 || changes[0].Path != filepath.FromSlash("dir with space/é — ü.txt") || changes[0].Status != "A" {
		t.Errorf("%v", changes)
	}
}

func TestBetweenOnlyAnswersForTheDesksOwnCheckpoints(t *testing.T) {
	cpScratch(t)
	dir := aGitRepo(t)
	store := newCheckpoints()
	mine := newSnap(t, store, cpDesk(dir), "mine")
	theirs := newSnap(t, store, checkpointDesk{Session: "other", Folder: dir}, "theirs")
	head := gitRepoCmd(t, dir, "rev-parse", "HEAD")
	for _, id := range []string{"--output=/x", "HEAD", "", "ZZ", strings.ToUpper(mine.ID), mine.ID[:20]} {
		if changes, note := store.between(cpDesk(dir), id, mine.ID); changes != nil || note == "" {
			t.Errorf("%q was taken: %v", id, changes)
		}
	}
	for _, id := range []string{theirs.ID, head} { // another desk's, and an ordinary commit of the repo
		if changes, note := store.between(cpDesk(dir), mine.ID, id); changes != nil || !strings.Contains(note, "no longer kept") {
			t.Errorf("%s: %v %q", id, changes, note)
		}
	}
}

func TestALabelIsOneShortLineAndASessionIdMakesNoOddRef(t *testing.T) {
	cpScratch(t)
	dir := aGitRepo(t)
	store := newCheckpoints()
	long := strings.Repeat("é", 400)
	cases := map[string]string{"line one\nline two\x00\ttab": "line one line two tab", "": "checkpoint", "   ": "checkpoint", "  spaced   out ": "spaced out"}
	for label, want := range cases {
		if got := newSnap(t, store, cpDesk(dir), label).Label; got != want {
			t.Errorf("%q became %q, want %q", label, got, want)
		}
	}
	if got := newSnap(t, store, cpDesk(dir), long).Label; len(got) > checkpointLabelMax || strings.ToValidUTF8(got, "?") != got || !strings.HasPrefix(long, got) {
		t.Errorf("a long label became %d bytes: %q", len(got), got)
	}
	list, _ := store.list(cpDesk(dir))
	if len(list) != 5 || list[4].N != 5 {
		t.Errorf("%v", list)
	}
	for _, session := range []string{"../../refs/heads/main", "a b", "x..y", "x.lock", "-rf", "", "é", "a/b", strings.Repeat("a", 200), "_abc"} {
		name := sessionRefName(session)
		if out, err := exec.Command("git", "check-ref-format", checkpointRefs+name+"/1").CombinedOutput(); err != nil || strings.ContainsAny(name, "/ ") || name == "" {
			t.Errorf("%q gave %q: %v %s", session, name, err, out)
		}
		if got := newSnap(t, store, checkpointDesk{Session: session, Folder: dir}, "x"); got.N != 1 {
			t.Errorf("%q: %+v", session, got)
		}
		if l, _ := store.list(checkpointDesk{Session: session, Folder: dir}); len(l) != 1 {
			t.Errorf("%q lists %v", session, l)
		}
	}
	if sessionRefName("abc-1.2_3") != "abc-1.2_3" || sessionRefName("a b") == sessionRefName("a  b") || !strings.HasPrefix(sessionRefName("a b"), "_") {
		t.Error("plain ids are kept as they are, any other is a hash behind an underscore")
	}
}

func TestHeadIsRecordedSoACommitBetweenTwoCheckpointsShows(t *testing.T) {
	cpScratch(t)
	dir := aGitRepo(t)
	store := newCheckpoints()
	one := newSnap(t, store, cpDesk(dir), "start")
	writeRepoFile(t, dir, "made.txt", "m\n")
	gitRepoCmd(t, dir, "add", "made.txt")
	gitRepoCmd(t, dir, "commit", "-q", "-m", "the agent committed")
	two := newSnap(t, store, cpDesk(dir), "end")
	if one.Head == "" || two.Head == one.Head || two.Head != gitRepoCmd(t, dir, "rev-parse", "HEAD") {
		t.Errorf("%+v %+v", one, two)
	}
	list, _ := store.list(cpDesk(dir))
	if list[0].Head != one.Head || list[1].Head != two.Head {
		t.Errorf("the heads aren't read back: %v", list)
	}
}

func TestOldCheckpointsArePrunedToTheLastNAndNumbersKeepCounting(t *testing.T) {
	cpScratch(t)
	dir := aGitRepo(t)
	other := checkpointDesk{Session: "other", Folder: dir}
	store := newCheckpoints()
	newSnap(t, store, other, "not mine")
	for i := 1; i <= 5; i++ {
		newSnap(t, store, cpDesk(dir), "n"+strconv.Itoa(i))
	}
	if note := store.keepOnly(cpDesk(dir), 2); note != "" {
		t.Fatal(note)
	}
	list, _ := store.list(cpDesk(dir))
	if len(list) != 2 || list[0].N != 4 || list[1].N != 5 || list[1].Label != "n5" {
		t.Errorf("%v", list)
	}
	if next := newSnap(t, store, cpDesk(dir), "after"); next.N != 6 {
		t.Errorf("the next number is %d", next.N)
	}
	if note := store.keepOnly(cpDesk(dir), 5); note != "" { // more than there are: nothing is dropped
		t.Fatal(note)
	}
	if list, _ := store.list(cpDesk(dir)); len(list) != 3 {
		t.Errorf("%v", list)
	}
	if list, _ := store.list(other); len(list) != 1 {
		t.Errorf("another desk lost one: %v", list)
	}
	// And by itself after each snapshot, at the default size.
	defer func(n int) { checkpointsKept = n }(checkpointsKept)
	checkpointsKept = 3
	for i := 0; i < 4; i++ {
		newSnap(t, store, cpDesk(dir), "more")
	}
	list, _ = store.list(cpDesk(dir))
	if len(list) != 3 || list[2].N != 10 {
		t.Errorf("%v", list)
	}
}

func TestFiringADeskDropsEveryCheckpointOfItAndOnlyIt(t *testing.T) {
	cpScratch(t)
	dir := aGitRepo(t)
	other := checkpointDesk{Session: "other", Folder: dir}
	store := newCheckpoints()
	for i := 0; i < 3; i++ {
		newSnap(t, store, cpDesk(dir), "x")
	}
	newSnap(t, store, other, "x")
	if note := store.forget(cpDesk(dir)); note != "" {
		t.Fatal(note)
	}
	if list, _ := store.list(cpDesk(dir)); len(list) != 0 {
		t.Errorf("%v", list)
	}
	if list, _ := store.list(other); len(list) != 1 {
		t.Errorf("another desk lost its checkpoint: %v", list)
	}
	if note := store.forget(cpDesk(dir)); note != "" { // a second time is nothing
		t.Error(note)
	}
	if next := newSnap(t, store, cpDesk(dir), "again"); next.N != 1 {
		t.Errorf("%+v", next)
	}
}

// A hire's own worktree is deleted when it is fired; its checkpoints live in the repo it was hired from.
func TestFiringAHireWhoseWorktreeIsGoneStillDropsItsCheckpoints(t *testing.T) {
	cpScratch(t)
	repo := aGitRepo(t)
	tree := filepath.Join(physical(t.TempDir()), "hire")
	gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/x", tree)
	writeRepoFile(t, tree, "work.txt", "w\n")
	desk := checkpointDesk{Session: "hire-1", Folder: tree}
	store := newCheckpoints()
	newSnap(t, store, desk, "x")
	newSnap(t, store, desk, "y")
	if refs := gitRepoCmd(t, repo, "for-each-ref", "refs/smolcorp"); strings.Count(refs, "\n") != 1 {
		t.Fatalf("the checkpoints of a worktree are kept in the repo: %q", refs)
	}
	gitRepoCmd(t, repo, "worktree", "remove", "--force", tree)
	// A store that has not seen the desk can't know where to look: it says so and leaves them (the app calls forget
	// while the folder is still there; this is the fallback).
	if note := newCheckpoints().forget(desk); !strings.Contains(note, "folder is gone") {
		t.Errorf("%q", note)
	}
	if note := store.forget(desk); note != "" {
		t.Fatal(note)
	}
	if refs := gitRepoCmd(t, repo, "for-each-ref", "refs/smolcorp"); refs != "" {
		t.Errorf("%s", refs)
	}
}

func TestABigFolderIsSkippedWithAWordAndLeavesNothing(t *testing.T) {
	left := cpScratch(t)
	dir := aGitRepo(t)
	writeRepoFile(t, dir, "big.bin", strings.Repeat("x", 3000))
	for i := 0; i < 5; i++ {
		writeRepoFile(t, dir, "many/"+strconv.Itoa(i), "x")
	}
	store := newCheckpoints()
	defer func(f int, b int64) { checkpointMaxFiles, checkpointMaxBytes = f, b }(checkpointMaxFiles, checkpointMaxBytes)
	checkpointMaxFiles = 6
	if cp, note := store.snapshot(cpDesk(dir), "x"); cp.ID != "" || !strings.Contains(note, "more than 6 files") {
		t.Errorf("%+v %q", cp, note)
	}
	checkpointMaxFiles, checkpointMaxBytes = 20000, 2048
	if cp, note := store.snapshot(cpDesk(dir), "x"); cp.ID != "" || !strings.Contains(note, "add up to more than") {
		t.Errorf("%+v %q", cp, note)
	}
	// An ignored folder doesn't count: a build folder never makes a repo too big.
	writeRepoFile(t, dir, ".gitignore", "big.bin\n")
	if cp, note := store.snapshot(cpDesk(dir), "x"); cp.ID == "" || note != "" {
		t.Errorf("%+v %q", cp, note)
	}
	if list, _ := store.list(cpDesk(dir)); len(list) != 1 || list[0].N != 1 {
		t.Errorf("a skipped snapshot used a number: %v", list)
	}
	if names := left(); len(names) != 0 {
		t.Errorf("%v", names)
	}
}

func TestASlowSnapshotIsGivenUpOnAndLeavesNothing(t *testing.T) {
	left := cpScratch(t)
	dir := aGitRepo(t)
	defer func(d time.Duration) { checkpointTimeout = d }(checkpointTimeout)
	checkpointTimeout = time.Millisecond
	store := newCheckpoints()
	started := time.Now()
	cp, note := store.snapshot(cpDesk(dir), "x")
	if cp.ID != "" || !strings.Contains(note, "took too long") || time.Since(started) > 10*time.Second {
		t.Errorf("%+v %q after %s", cp, note, time.Since(started))
	}
	if refs := gitRepoCmd(t, dir, "for-each-ref", "refs/smolcorp"); refs != "" || len(left()) != 0 {
		t.Errorf("%q %v", refs, left())
	}
	// Waiting for a free slot counts against the same time.
	checkpointTimeout = 200 * time.Millisecond
	jammed := newCheckpoints()
	for i := 0; i < cap(jammed.jobs); i++ {
		jammed.jobs <- struct{}{}
	}
	started = time.Now()
	if cp, note := jammed.snapshot(cpDesk(dir), "x"); cp.ID != "" || !strings.Contains(note, "took too long") || time.Since(started) > 5*time.Second {
		t.Errorf("%+v %q", cp, note)
	}
}

func TestSnapshotsOfOneDeskTogetherGetNumbersOfTheirOwn(t *testing.T) {
	cpScratch(t)
	dir := aGitRepo(t)
	store := newCheckpoints()
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[int]bool{}
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cp, note := store.snapshot(cpDesk(dir), "x")
			mu.Lock()
			defer mu.Unlock()
			if note != "" || seen[cp.N] {
				t.Errorf("%+v %q", cp, note)
			}
			seen[cp.N] = true
		}()
	}
	wg.Wait()
	list, _ := store.list(cpDesk(dir))
	if len(list) != 6 || list[5].N != 6 {
		t.Errorf("%v", list)
	}
}

func TestAnAnswerIsAlwaysAListNeverNil(t *testing.T) {
	cpScratch(t)
	dir := aGitRepo(t)
	if list, note := newCheckpoints().list(cpDesk(dir)); list == nil || len(list) != 0 || note != "" {
		t.Errorf("%v %q", list, note)
	}
}

// aHostileSubmodule is a repo with a submodule whose own config and attributes name programs that leave a marker (a
// clean filter, fsmonitor, a refs command), at the commit the repo records, with two same-size edits in its files, so
// whoever asks it whether it is dirty has to look inside them. It gives the repo, the submodule's folder and its commit.
func aHostileSubmodule(t *testing.T) (h *hostileRepo, sub, recorded string) {
	t.Helper()
	return aHostileSubmoduleAt(t, "sub")
}

// aHostileSubmoduleAt is aHostileSubmodule with the submodule at path (slash-separated, below the repo's top).
func aHostileSubmoduleAt(t *testing.T, path string) (h *hostileRepo, sub, recorded string) {
	t.Helper()
	h = &hostileRepo{markers: physical(t.TempDir())}
	// The submodule's own source: a file its own .gitattributes names, and one its info/attributes will.
	source := aGitRepo(t)
	writeRepoFile(t, source, "x.txt", "1\n")
	writeRepoFile(t, source, "y.txt", "1\n")
	writeRepoFile(t, source, ".gitattributes", "y.txt filter=subf\n")
	gitRepoCmd(t, source, "add", ".")
	gitRepoCmd(t, source, "commit", "-q", "-m", "files")
	h.dir = aGitRepo(t)
	gitRepoCmd(t, h.dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", source, path)
	gitRepoCmd(t, h.dir, "commit", "-q", "-m", "a submodule")
	sub = filepath.Join(h.dir, filepath.FromSlash(path))
	gitRepoCmd(t, sub, "config", "filter.subf.clean", h.program(t, "subclean"))
	gitRepoCmd(t, sub, "config", "core.fsmonitor", h.program(t, "submonitor"))
	gitRepoCmd(t, sub, "config", "core.alternateRefsCommand", h.program(t, "subrefs")) // a key the store refuses in a top repo
	gitDir := gitRepoCmd(t, sub, "rev-parse", "--absolute-git-dir")
	writeRepoFile(t, gitDir, "info/attributes", "x.txt filter=subf\n")
	recorded = gitRepoCmd(t, sub, "rev-parse", "HEAD")
	for _, name := range []string{"x.txt", "y.txt"} {
		writeRepoFile(t, sub, name, "2\n")
	}
	return h, sub, recorded
}

// A submodule has a config and attributes of its own, which add reads when it asks whether the submodule is dirty. The
// control runs that plain add, so the filter is really named where a plain git finds it.
func TestASubmodulesOwnConfigRunsNothingDuringASnapshot(t *testing.T) {
	for _, mode := range []struct {
		name string
		on   bool
	}{{"with an attribute source", true}, {"without one", false}} {
		t.Run(mode.name, func(t *testing.T) {
			cpScratch(t)
			if runtime.GOOS == "windows" {
				t.Skip("shell scripts")
			}
			if out, err := exec.Command("git", "version").Output(); err != nil || (mode.on && !gitVersionAtLeast(string(out), 2, 40)) {
				t.Skip("this git has no --attr-source")
			}
			h, sub, recorded := aHostileSubmodule(t)
			// The control: plain git, the same steps. Its add runs the submodule's filter.
			index := filepath.Join(t.TempDir(), "index")
			for _, args := range [][]string{{"read-tree", "HEAD"}, {"add", "-A"}} {
				cmd := exec.Command("git", append([]string{"-C", h.dir}, args...)...)
				cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("%v: %v %s", args, err, out)
				}
			}
			if ran := h.ran(); !slices.Contains(ran, "subclean") {
				t.Fatalf("a plain add didn't run the submodule's filter (%v), so this test proves less than it says", ran)
			}
			// The control's own add refreshed the submodule's index: edit again, so a read has to look inside the files.
			for _, name := range []string{"x.txt", "y.txt"} {
				writeRepoFile(t, sub, name, "3\n")
			}
			withAttrSource(t, mode.on)
			store := newCheckpoints()
			cp, note := store.snapshot(cpDesk(h.dir), "with a submodule")
			if note != "" {
				t.Fatalf("a repo with a submodule was refused: %s", note)
			}
			store.snapshot(cpDesk(h.dir), "again")
			store.list(cpDesk(h.dir))
			store.keepOnly(cpDesk(h.dir), 1)
			if ran := h.ran(); len(ran) != 0 {
				t.Errorf("the store ran: %v", ran)
			}
			// The submodule is a link at the commit its HEAD is at, and the files around it are snapshotted as always.
			if line := gitRepoCmd(t, h.dir, "ls-tree", cp.ID, "sub"); !strings.HasPrefix(line, "160000 commit "+recorded+"\t") {
				t.Errorf("the submodule is recorded as %q, not at %s", line, recorded)
			}
			if names := gitRepoCmd(t, h.dir, "ls-tree", "-r", "--name-only", cp.ID); names != ".gitmodules\na.txt\nsrc/b.txt\nsub" {
				t.Errorf("%q", names)
			}
			// A submodule a commit ahead of what the repo records: the link is at the commit its HEAD is at.
			writeRepoFile(t, sub, "z.txt", "z\n")
			gitRepoCmd(t, sub, "add", "z.txt")
			gitRepoCmd(t, sub, "commit", "-q", "-m", "ahead")
			ahead := gitRepoCmd(t, sub, "rev-parse", "HEAD")
			h.ran() // (the commit above looked inside the edited files itself)
			next := newSnap(t, store, cpDesk(h.dir), "ahead")
			if line := gitRepoCmd(t, h.dir, "ls-tree", next.ID, "sub"); !strings.HasPrefix(line, "160000 commit "+ahead+"\t") || ahead == recorded {
				t.Errorf("the submodule is recorded as %q, not at %s", line, ahead)
			}
			if ran := h.ran(); len(ran) != 0 {
				t.Errorf("the store ran: %v", ran)
			}
		})
	}
}

// A submodule's folder that is gone is a deletion, as plain git's add -A makes it; one that holds ordinary files now, or
// is a link or a file, keeps the commit the repo had for it; one behind a link is gone, as plain git has it. Nothing is run either way, and the
// snapshot is made.
func TestASubmoduleThatIsGoneOrNoLongerACheckoutIsSettledByHandAndNothingFails(t *testing.T) {
	cpScratch(t)
	source := aGitRepo(t)
	dir := aGitRepo(t)
	for _, name := range []string{"gone", "plain", "linked", "file", "deep/sub"} {
		gitRepoCmd(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", "--", source, name)
	}
	// A link whose name starts with a dash, for a folder that is gone (git won't add such a submodule itself).
	gitRepoCmd(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+gitRepoCmd(t, source, "rev-parse", "HEAD")+",-gone")
	gitRepoCmd(t, dir, "commit", "-q", "-m", "submodules")
	was := map[string]string{}
	for _, line := range strings.Split(gitRepoCmd(t, dir, "ls-files", "--stage"), "\n") {
		if f := strings.Fields(line); len(f) == 4 && f[0] == "160000" {
			was[f[3]] = f[1]
		}
	}
	for _, name := range []string{"gone", "plain", "linked", "file", "deep"} {
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	writeRepoFile(t, dir, "plain/now.txt", "a folder of files\n")
	if runtime.GOOS != "windows" {
		if err := os.Symlink("src", filepath.Join(dir, "linked")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("src", filepath.Join(dir, "deep")); err != nil { // a link where a parent folder of a submodule was
			t.Fatal(err)
		}
	}
	writeRepoFile(t, dir, "file", "a file where a submodule was\n")
	cp := newSnap(t, newCheckpoints(), cpDesk(dir), "x")
	got := map[string]string{}
	for _, line := range strings.Split(gitRepoCmd(t, dir, "ls-tree", "-r", cp.ID), "\n") {
		if f := strings.Fields(line); len(f) == 4 && f[0] == "160000" {
			got[f[3]] = f[2]
		}
	}
	if _, there := got["gone"]; there || got["-gone"] != "" {
		t.Errorf("a folder that is gone is still a submodule: %v", got)
	}
	if _, there := got["deep/sub"]; there || gitRepoCmd(t, dir, "ls-tree", cp.ID, "deep") == "" {
		t.Errorf("a submodule behind a link should be gone, with the link in the snapshot: %v", got)
	}
	for _, name := range []string{"plain", "linked", "file"} {
		if got[name] != was[name] || got[name] == "" {
			t.Errorf("%s is recorded at %q, not at the commit the repo had (%s)", name, got[name], was[name])
		}
	}
	if len(got) != 3 {
		t.Errorf("%v", got)
	}
}

// What is on disk at the gitlinks must not decide what add is shown. The wrapper stands in for a hire's background loop:
// a submodule's .git is out of the way when the store looks, and back just before add runs (the first add of a command
// line it sees). The control, plain git through the same wrapper, runs the submodule's filter.
func TestADotGitThatAppearsJustBeforeAddNeverLetsASubmodulesFilterRun(t *testing.T) {
	for _, mode := range []struct {
		name string
		on   bool
	}{{"with an attribute source", true}, {"without one", false}} {
		t.Run(mode.name, func(t *testing.T) {
			cpScratch(t)
			if runtime.GOOS == "windows" {
				t.Skip("shell scripts")
			}
			if out, err := exec.Command("git", "version").Output(); err != nil || (mode.on && !gitVersionAtLeast(string(out), 2, 40)) {
				t.Skip("this git has no --attr-source")
			}
			h, sub, recorded := aHostileSubmodule(t)
			real, err := exec.LookPath("git")
			if err != nil {
				t.Skip("no git here")
			}
			off, back := filepath.Join(sub, ".git.off"), filepath.Join(sub, ".git")
			bin := filepath.Join(physical(t.TempDir()), "bin")
			if err := os.MkdirAll(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			wrapper := "#!/bin/sh\nfor a in \"$@\"; do\n  if [ \"$a\" = add ] && [ -e '" + off + "' ]; then mv '" + off + "' '" + back + "'; fi\ndone\nexec '" + real + "' \"$@\"\n"
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			hide := func() {
				t.Helper()
				if err := os.Rename(back, off); err != nil {
					t.Fatal(err)
				}
			}
			// The control: no .git when it looks, and there by the time add looks.
			hide()
			index := filepath.Join(t.TempDir(), "index")
			for _, args := range [][]string{{"read-tree", "HEAD"}, {"add", "-A"}} {
				cmd := exec.Command("git", append([]string{"-C", h.dir}, args...)...)
				cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("%v: %v %s", args, err, out)
				}
			}
			if ran := h.ran(); !slices.Contains(ran, "subclean") {
				t.Fatalf("a plain add didn't run the submodule's filter (%v), so this test proves less than it says", ran)
			}
			for _, name := range []string{"x.txt", "y.txt"} {
				writeRepoFile(t, sub, name, "3\n")
			}
			hide()
			withAttrSource(t, mode.on)
			store := newCheckpoints()
			cp, note := store.snapshot(cpDesk(h.dir), "racy")
			if note != "" {
				t.Fatalf("the snapshot was refused: %s", note)
			}
			if _, err := os.Lstat(back); err != nil {
				t.Fatalf("the wrapper never brought the .git back, so the race wasn't run: %v", err)
			}
			if ran := h.ran(); len(ran) != 0 {
				t.Errorf("the store ran: %v", ran)
			}
			if line := gitRepoCmd(t, h.dir, "ls-tree", cp.ID, "sub"); !strings.HasPrefix(line, "160000 commit "+recorded+"\t") {
				t.Errorf("the submodule is recorded as %q, not at %s", line, recorded)
			}
		})
	}
}

// RT126C1b B1' and RT126C1c, for real, with no wrapper: while snapshots run, a loop flips what add would find at a
// submodule's place: its .git there and gone (0 to 3 ms apart), or the folder above it a link one moment and the real
// folder the next. The store never runs the submodule's filter or fsmonitor; the control (plain add, the .git there)
// does.
func TestASubmoduleThatFlipsDuringSnapshotsNeverRunsItsFilter(t *testing.T) {
	for _, flip := range []string{"its .git", "the folder above it"} {
		t.Run(flip, func(t *testing.T) {
			cpScratch(t)
			if runtime.GOOS == "windows" {
				t.Skip("shell scripts and links")
			}
			path := "sub"
			if flip == "the folder above it" {
				path = "p/sub"
			}
			h, sub, _ := aHostileSubmoduleAt(t, path)
			plainAdd(t, h.dir)
			if ran := h.ran(); !slices.Contains(ran, "subclean") {
				t.Fatalf("a plain add didn't run the submodule's filter (%v)", ran)
			}
			here, away := filepath.Join(sub, ".git"), filepath.Join(sub, ".git.off")
			if flip == "the folder above it" {
				here, away = filepath.Join(h.dir, "p"), filepath.Join(h.dir, "p.real")
			}
			empty := physical(t.TempDir())
			stop, done := make(chan struct{}), make(chan struct{})
			var flipping sync.Mutex // the test's own edits wait for a flip to end; the snapshots never do
			go func() {
				defer close(done)
				for i := 0; ; i++ {
					select {
					case <-stop:
						return
					default:
					}
					flipping.Lock()
					if os.Rename(here, away) == nil {
						if flip == "the folder above it" {
							_ = os.Symlink(empty, here)
						}
						time.Sleep(time.Duration(i%4) * time.Millisecond)
						if flip == "the folder above it" {
							_ = os.Remove(here)
						}
						_ = os.Rename(away, here)
					}
					flipping.Unlock()
					time.Sleep(time.Duration(i%4) * time.Millisecond)
				}
			}()
			store, kept := newCheckpoints(), 0
			for i := 0; i < 60; i++ {
				flipping.Lock()
				for _, name := range []string{"x.txt", "y.txt"} { // same size each time: only a look inside tells
					writeRepoFile(t, sub, name, strconv.Itoa(i%10)+"\n")
				}
				flipping.Unlock()
				if cp, _ := store.snapshot(cpDesk(h.dir), "flip"); cp.ID != "" {
					kept++
				}
			}
			close(stop)
			<-done
			if ran := h.ran(); len(ran) != 0 {
				t.Errorf("the store ran: %v", ran)
			}
			if kept == 0 {
				t.Error("no snapshot was kept, so the race proves nothing")
			}
			t.Logf("%d of 60 kept", kept)
		})
	}
}

// A .git that goes between the store's look and its read of the submodule's HEAD must not make the repo above the
// submodule's: the link keeps the commit the repo had. (HEAD is read with --git-dir, so nothing is searched for.)
func TestASubmodulesHeadIsNeverTakenFromTheRepoAboveIt(t *testing.T) {
	cpScratch(t)
	source := aGitRepo(t)
	dir := aGitRepo(t)
	gitRepoCmd(t, dir, "-c", "protocol.file.allow=always", "submodule", "add", "-q", source, "sub")
	gitRepoCmd(t, dir, "commit", "-q", "-m", "a submodule")
	was := gitRepoCmd(t, dir, "rev-parse", "HEAD:sub")
	repo, err := newCheckpoints().open(context.Background(), cpDesk(dir), true)
	if err != nil {
		t.Fatal(err)
	}
	// The .git of the submodule is gone: the one named doesn't exist.
	out, status, err := repo.gitIn(context.Background(), repo.dir, []string{"--git-dir=" + filepath.Join(dir, "sub", ".git.gone")}, nil, "rev-parse", "--verify", "-q", "HEAD")
	if err != nil || status == 0 || strings.TrimSpace(string(out)) != "" {
		t.Errorf("a .git that is not there answered %q (%d, %v), as if it were the repo above", out, status, err)
	}
	if got := strings.TrimSpace(gitRepoCmd(t, dir, "rev-parse", "HEAD")); got == was {
		t.Fatal("the two commits are the same, so this proves nothing")
	}
}

// slowRefWrites stands in for a slow disk: each write of a ref (a create or a delete) waits as long as slow last said,
// after the look at the folder pinned.
func slowRefWrites(t *testing.T) (slow func(d time.Duration)) {
	t.Helper()
	var wait atomic.Int64
	cpBeforeRefWrite = func() { time.Sleep(time.Duration(wait.Load())) }
	t.Cleanup(func() { cpBeforeRefWrite = nil })
	return func(d time.Duration) { wait.Store(int64(d)) }
}

// refWriteBefore runs action (shell) once, at the next write of a ref, between the look at the folder pinned and the
// write: the worst moment, which no git wrapper reaches now that no git writes a ref. arm arms it; fired says it ran.
func refWriteBefore(t *testing.T, action string) (arm func(), fired func() bool) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	var armed, done atomic.Bool
	cpBeforeRefWrite = func() {
		if armed.CompareAndSwap(true, false) {
			if out, err := exec.Command("sh", "-c", action).CombinedOutput(); err != nil {
				t.Errorf("%v %s", err, out)
			}
			done.Store(true)
		}
	}
	t.Cleanup(func() { cpBeforeRefWrite = nil })
	return func() { done.Store(false); armed.Store(true) }, done.Load
}

// locksIn lists the lock files of a git folder.
func locksIn(t *testing.T, dir string) []string {
	t.Helper()
	var locks []string
	_ = filepath.WalkDir(filepath.Join(dir, ".git"), func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".lock") {
			rel, _ := filepath.Rel(dir, path)
			locks = append(locks, rel)
		}
		return nil
	})
	return locks
}

// The store writes its refs itself (T126CK1), as git does: no lock file is left, the owner's own git works on them as
// on its own, and refs git packed are counted (the next number is past them) and dropped from packed-refs, loose or not.
func TestTheRefsTheStoreWritesAreGitsOwnAndLeaveNoLock(t *testing.T) {
	cpScratch(t)
	dir := aGitRepo(t)
	store := newCheckpoints()
	for i := 0; i < 3; i++ {
		newSnap(t, store, cpDesk(dir), "x")
	}
	ownersGitWorks := func(what string) {
		t.Helper()
		if locks := locksIn(t, dir); len(locks) != 0 {
			t.Errorf("%s left lock files: %v", what, locks)
		}
		gitRepoCmd(t, dir, "branch", "zzz")
		gitRepoCmd(t, dir, "branch", "-D", "zzz") // takes the packed-refs lock too
		gitRepoCmd(t, dir, "fsck", "--no-dangling")
	}
	ownersGitWorks("a create")
	gitRepoCmd(t, dir, "pack-refs", "--all")
	if cp := newSnap(t, store, cpDesk(dir), "after a pack"); cp.N != 4 {
		t.Errorf("%+v", cp)
	}
	if note := store.keepOnly(cpDesk(dir), 1); note != "" {
		t.Error(note)
	}
	list, _ := store.list(cpDesk(dir))
	if len(list) != 1 || list[0].N != 4 {
		t.Fatalf("%v", list)
	}
	packed, err := os.ReadFile(filepath.Join(dir, ".git", "packed-refs"))
	if err != nil || strings.Contains(string(packed), "refs/smolcorp") || !strings.Contains(string(packed), "refs/heads/main") {
		t.Errorf("%v %q", err, packed)
	}
	ownersGitWorks("a prune")
	// Packed only, its folder gone (pack-refs removes it); then packed and loose at once. Each goes whole.
	folder := filepath.Join(dir, ".git", "refs", "smolcorp", "checkpoints", "s1")
	for _, loose := range []bool{false, true} {
		gitRepoCmd(t, dir, "pack-refs", "--all")
		if loose {
			writeRepoFile(t, folder, "1", list[0].ID+"\n") // git's own update-ref writes no loose copy of a packed ref's value
		}
		if note := store.forget(cpDesk(dir)); note != "" {
			t.Error(note)
		}
		if refs := cpRefs(t, dir); refs != "" {
			t.Errorf("%q", refs)
		}
		if _, err := os.Lstat(folder); err == nil {
			t.Error("the desk's empty folder of refs was left")
		}
		ownersGitWorks("a forget")
		list = []Checkpoint{newSnap(t, store, cpDesk(dir), "again")}
	}
}

// A ref write runs on no deadline of the snapshot's: a snapshot whose own time is spent by the time it writes (a slow
// disk) still ends by writing its ref.
func TestAWriteDoesNotRunOnWhatIsLeftOfTheSnapshotsTime(t *testing.T) {
	cpScratch(t)
	slow := slowRefWrites(t)
	dir := aGitRepo(t)
	store := newCheckpoints()
	defer func(d time.Duration) { checkpointTimeout = d }(checkpointTimeout)
	checkpointTimeout = 3 * time.Second // the steps before the write are well inside it; the write (4 s) is not
	slow(4 * time.Second)
	cp, note := store.snapshot(cpDesk(dir), "slow write")
	if note != "" || cp.N != 1 {
		t.Fatalf("%+v %q", cp, note)
	}
	if locks := locksIn(t, dir); len(locks) != 0 {
		t.Errorf("%v", locks)
	}
}

// A lock file already there, left by something else, is said in words, and doesn't wedge the desk.
func TestALockFileLeftInTheRepoIsSaidInWordsAndNeverWedgesTheDesk(t *testing.T) {
	cpScratch(t)
	dir := aGitRepo(t)
	store := newCheckpoints()
	one := newSnap(t, store, cpDesk(dir), "one")
	refs := filepath.Join(dir, ".git", "refs", "smolcorp", "checkpoints", "s1")
	// A lock at the next number: that number is skipped, and every later snapshot works.
	writeRepoFile(t, refs, "2.lock", "")
	for i := 0; i < 3; i++ {
		if cp, note := store.snapshot(cpDesk(dir), "x"); note != "" || cp.N != 3+i {
			t.Fatalf("%+v %q", cp, note)
		}
	}
	// A lock on an old ref: it is told, and the others still go (one stuck ref doesn't make a desk grow for good).
	writeRepoFile(t, refs, "1.lock", "")
	note := store.keepOnly(cpDesk(dir), 1)
	if !strings.Contains(note, "1.lock") || !strings.Contains(note, "lock file") {
		t.Errorf("%q", note)
	}
	if list, _ := store.list(cpDesk(dir)); len(list) != 2 || list[0] != one || list[1].N != 5 {
		t.Errorf("only the stuck one and the newest should be left: %v", list)
	}
	// The lock of packed-refs, as an owner's git left it: told once, without a try at every ref.
	if err := os.Remove(filepath.Join(refs, "1.lock")); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, filepath.Join(dir, ".git"), "packed-refs.lock", "")
	started := time.Now()
	if note := store.forget(cpDesk(dir)); !strings.Contains(note, "packed-refs.lock") || time.Since(started) > 5*time.Second {
		t.Errorf("%q after %s", note, time.Since(started))
	}
	// Once the locks are gone, everything works again.
	for _, lock := range []string{filepath.Join(dir, ".git", "packed-refs.lock"), filepath.Join(refs, "2.lock")} {
		_ = os.Remove(lock)
	}
	if note := store.forget(cpDesk(dir)); note != "" {
		t.Error(note)
	}
	if refs := gitRepoCmd(t, dir, "for-each-ref", "refs/smolcorp"); refs != "" {
		t.Errorf("%s", refs)
	}
}

// Fire and prune wait for a snapshot of the desk that is running, as long as a snapshot can run, so the ref it adds is
// not left behind; past that they give up in words.
func TestFiringADeskWaitsForASnapshotThatIsRunning(t *testing.T) {
	cpScratch(t)
	dir := aGitRepo(t)
	store := newCheckpoints()
	newSnap(t, store, cpDesk(dir), "x")
	defer func(d time.Duration) { checkpointDeskWait = d }(checkpointDeskWait)
	checkpointDeskWait = 5 * time.Second
	leave, ok := store.enter(context.Background(), "s1") // a snapshot, running
	if !ok {
		t.Fatal("no turn")
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		leave()
	}()
	started := time.Now()
	if note := store.forget(cpDesk(dir)); note != "" || time.Since(started) < 250*time.Millisecond {
		t.Errorf("%q after %s", note, time.Since(started))
	}
	if list, _ := store.list(cpDesk(dir)); len(list) != 0 {
		t.Errorf("%v", list)
	}
	checkpointDeskWait = 200 * time.Millisecond
	hold, _ := store.enter(context.Background(), "s1")
	defer hold()
	started = time.Now()
	if note := store.forget(cpDesk(dir)); !strings.Contains(note, "took too long") || time.Since(started) > 3*time.Second {
		t.Errorf("%q after %s", note, time.Since(started))
	}
	if note := store.keepOnly(cpDesk(dir), 1); !strings.Contains(note, "took too long") {
		t.Errorf("%q", note)
	}
}

// B4: core.worktree can name another repo's folder. The store must refuse, and write nothing in either repo.
func TestACoreWorktreeThatNamesAnotherRepoIsRefusedAndNothingIsWrittenAnywhere(t *testing.T) {
	cpScratch(t)
	desk := aGitRepo(t)
	other := aGitRepo(t)
	writeRepoFile(t, other, "secret.txt", "not the desk's\n")
	gitRepoCmd(t, desk, "config", "core.worktree", other)
	store := newCheckpoints()
	cp, note := store.snapshot(cpDesk(desk), "x")
	if cp.ID != "" || !strings.Contains(note, "core.worktree") {
		t.Errorf("%+v %q", cp, note)
	}
	if list, note := store.list(cpDesk(desk)); len(list) != 0 || !strings.Contains(note, "core.worktree") {
		t.Errorf("%v %q", list, note)
	}
	if note := store.keepOnly(cpDesk(desk), 1); !strings.Contains(note, "core.worktree") {
		t.Errorf("%q", note)
	}
	if note := store.forget(cpDesk(desk)); !strings.Contains(note, "core.worktree") {
		t.Errorf("%q", note)
	}
	for name, repo := range map[string]string{"the desk's": desk, "the other": other} {
		// (the desk's own git is asked with its work tree named, as its config would send it elsewhere)
		args := []string{"for-each-ref", "refs/smolcorp"}
		if name == "the desk's" {
			args = append([]string{"--git-dir=" + filepath.Join(desk, ".git"), "--work-tree=" + desk}, args...)
		}
		if refs := gitRepoCmd(t, repo, args...); refs != "" {
			t.Errorf("%s repo got %q", name, refs)
		}
	}
	if left := gitRepoCmd(t, other, "status", "--porcelain"); left != "?? secret.txt" {
		t.Errorf("the other repo's state changed: %q", left)
	}
}

// A repo whose core.worktree is its own top (a separate git folder) is the plain case and works; so does a linked
// worktree, whose files are its own and whose refs go to the common git folder.
func TestASeparateGitFolderAndALinkedWorktreeAreSnapshottedAsTheirOwn(t *testing.T) {
	cpScratch(t)
	store := newCheckpoints()
	// A separate git folder: .git is a file, core.worktree names the top.
	top, git := physical(t.TempDir()), filepath.Join(physical(t.TempDir()), "gitdir")
	gitRepoCmd(t, top, "init", "-q", "-b", "main", "--separate-git-dir", git)
	writeRepoFile(t, top, "a.txt", "a\n")
	gitRepoCmd(t, top, "add", ".")
	gitRepoCmd(t, top, "commit", "-q", "-m", "first")
	writeRepoFile(t, top, "new.txt", "n\n")
	cp := newSnap(t, store, cpDesk(top), "separate")
	if names := gitRepoCmd(t, top, "ls-tree", "-r", "--name-only", cp.ID); names != "a.txt\nnew.txt" {
		t.Errorf("%q", names)
	}
	if refs := gitRepoCmd(t, top, "for-each-ref", "--format=%(refname)", "refs/smolcorp"); refs != "refs/smolcorp/checkpoints/s1/1" {
		t.Errorf("%q", refs)
	}
	// A linked worktree.
	main := aGitRepo(t)
	linked := filepath.Join(physical(t.TempDir()), "wt")
	gitRepoCmd(t, main, "worktree", "add", "-q", "-b", "side", linked)
	writeRepoFile(t, linked, "only-here.txt", "w\n")
	cp = newSnap(t, store, checkpointDesk{Session: "w1", Folder: linked}, "linked")
	if names := gitRepoCmd(t, main, "ls-tree", "-r", "--name-only", cp.ID); names != "a.txt\nonly-here.txt\nsrc/b.txt" {
		t.Errorf("%q", names)
	}
	if refs := gitRepoCmd(t, main, "for-each-ref", "--format=%(refname)", "refs/smolcorp"); refs != "refs/smolcorp/checkpoints/w1/1" {
		t.Errorf("%q", refs)
	}
	if list, note := store.list(checkpointDesk{Session: "w1", Folder: linked}); note != "" || len(list) != 1 || list[0].ID != cp.ID {
		t.Errorf("%v %q", list, note)
	}
}

// N3: a drop that runs out of time says so in its own words, and stops between deletes when its time is spent.
func TestADropIsBoundedAsAWholeAndSaysItInItsOwnWords(t *testing.T) {
	cpScratch(t)
	slow := slowRefWrites(t)
	dir := aGitRepo(t)
	store := newCheckpoints()
	for i := 0; i < 8; i++ {
		newSnap(t, store, cpDesk(dir), "x")
	}
	slow(time.Second) // each delete takes a second and goes through
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	started := time.Now()
	note := store.prune(ctx, cpDesk(dir), 1)
	slow(0)
	if note != dropTooLong || time.Since(started) > 8*time.Second {
		t.Errorf("%q after %s", note, time.Since(started))
	}
	if list, _ := store.list(cpDesk(dir)); len(list) <= 1 || len(list) >= 8 {
		t.Errorf("%d checkpoints are left: some were dropped, and the drop stopped", len(list))
	}
}

// A reftable repo, whose refs the store can't write by hand through the folder pinned (T126CK1), is refused in words
// before any work, and nothing is written in it. Its refs can still be read.
func TestAReftableRepoIsRefusedInWords(t *testing.T) {
	cpScratch(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	dir := physical(t.TempDir())
	if out, err := exec.Command("git", "-C", dir, "init", "-q", "-b", "main", "--ref-format=reftable").CombinedOutput(); err != nil {
		t.Skipf("this git has no reftable: %s", out)
	}
	writeRepoFile(t, dir, "a.txt", "a\n")
	gitRepoCmd(t, dir, "add", ".")
	gitRepoCmd(t, dir, "commit", "-q", "-m", "first")
	store := newCheckpoints()
	if cp, note := store.snapshot(cpDesk(dir), "one"); cp.ID != "" || note != errReftable.Error() {
		t.Errorf("%+v %q", cp, note)
	}
	if refs := cpRefs(t, dir); refs != "" {
		t.Errorf("%q", refs)
	}
	if list, note := store.list(cpDesk(dir)); len(list) != 0 || note != "" {
		t.Errorf("%v %q", list, note)
	}
	if note := store.forget(cpDesk(dir)); note != "" {
		t.Errorf("%q", note)
	}
}

// N6: ids that differ only in their letter case never share a folder of refs, on a disk that ignores case or not.
func TestSessionIdsThatDifferOnlyInCaseGetFoldersOfTheirOwn(t *testing.T) {
	if a, b := sessionRefName("Abc"), sessionRefName("abc"); a == b || strings.EqualFold(a, b) || b != "abc" {
		t.Errorf("%q %q", a, b)
	}
}

// gitBefore puts a git first in the PATH that, once armed, runs action (shell, "$@" being git's arguments) just before
// the real git, the first time a command line holds the word when: a stand-in for a hire's background loop, at the
// worst moment. arm arms it once; fired says it ran.
func gitBefore(t *testing.T, when, action string) (arm func(), fired func() bool) {
	t.Helper()
	return gitAround(t, when, action, "")
}

// gitAround is gitBefore with a second action, run as soon as that git has ended (before is a swap, after puts it
// back): the swap no look before or after the command can see.
func gitAround(t *testing.T, when, before, after string) (arm func(), fired func() bool) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git here")
	}
	root := physical(t.TempDir())
	armed, done, bin := filepath.Join(root, "armed"), filepath.Join(root, "fired"), filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nif [ -e '" + armed + "' ]; then\n  for a in \"$@\"; do\n    if [ \"$a\" = '" + when + "' ]; then\n" +
		"      rm -f '" + armed + "'\n      " + before + "\n      touch '" + done + "'\n"
	if after != "" {
		script += "      '" + real + "' \"$@\"\n      status=$?\n      " + after + "\n      exit $status\n"
	}
	script += "      break\n    fi\n  done\nfi\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() {
			_ = os.Remove(done)
			if err := os.WriteFile(armed, nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}, func() bool {
			_, err := os.Stat(done)
			return err == nil
		}
}

// plainAdd is the control: plain git's read-tree HEAD and add -A in dir, into an index of its own.
func plainAdd(t *testing.T, dir string) {
	t.Helper()
	index := filepath.Join(t.TempDir(), "index")
	for _, args := range [][]string{{"read-tree", "HEAD"}, {"add", "-A"}} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v %s", args, err, out)
		}
	}
}

func cpRefs(t *testing.T, dir string) string {
	t.Helper()
	return gitRepoCmd(t, dir, "for-each-ref", "--format=%(refname)", "refs/smolcorp")
}

var cpAttrModes = []struct {
	name string
	on   bool
}{{"with an attribute source", true}, {"without one", false}}

// B5: what add reads is the private git folder the store wrote, so a filter written into the repo's config and its
// info/attributes after the store's look, just before add, is never read. The control (plain add, with that config)
// runs it.
func TestAFilterWrittenInJustBeforeAddNeverRuns(t *testing.T) {
	for _, mode := range cpAttrModes {
		t.Run(mode.name, func(t *testing.T) {
			cpScratch(t)
			h := &hostileRepo{dir: aGitRepo(t), markers: physical(t.TempDir())}
			git := filepath.Join(h.dir, ".git")
			late := filepath.Join(physical(t.TempDir()), "late.cfg")
			writeRepoFile(t, filepath.Dir(late), "late.cfg", "[filter \"late\"]\n\tclean = "+h.program(t, "lateclean")+"\n")
			writeRepoFile(t, filepath.Dir(late), "attributes", "* filter=late\n")
			writeRepoFile(t, h.dir, "a.txt", "changed\n")
			plainConfig, err := os.ReadFile(filepath.Join(git, "config"))
			if err != nil {
				t.Fatal(err)
			}
			hostile := func() {
				t.Helper()
				latest, _ := os.ReadFile(late)
				writeRepoFile(t, git, "config", string(plainConfig)+string(latest))
				writeRepoFile(t, git, "info/attributes", "* filter=late\n")
			}
			hostile()
			plainAdd(t, h.dir)
			if ran := h.ran(); !slices.Contains(ran, "lateclean") {
				t.Fatalf("a plain add didn't run the filter (%v), so this test proves less than it says", ran)
			}
			writeRepoFile(t, git, "config", string(plainConfig))
			if err := os.Remove(filepath.Join(git, "info/attributes")); err != nil {
				t.Fatal(err)
			}
			arm, fired := gitBefore(t, "add", "cat '"+late+"' >> '"+filepath.Join(git, "config")+"'; cp '"+
				filepath.Join(filepath.Dir(late), "attributes")+"' '"+filepath.Join(git, "info", "attributes")+"'")
			withAttrSource(t, mode.on)
			arm()
			cp, note := newCheckpoints().snapshot(cpDesk(h.dir), "late")
			if !fired() {
				t.Fatal("the wrapper never wrote the filter in, so the race wasn't run")
			}
			if note != "" || cp.ID == "" {
				t.Errorf("the snapshot was skipped: %q", note)
			}
			if ran := h.ran(); len(ran) != 0 {
				t.Errorf("the store ran: %v", ran)
			}
			if got := gitRepoCmd(t, h.dir, "show", cp.ID+":a.txt"); got != "changed" {
				t.Errorf("%q", got)
			}
		})
	}
}

// B5 for real, with no wrapper: the repo's config flips between a plain one and one with a filter (an atomic rename each
// time), its info/attributes naming the filter, while snapshots run.
func TestAConfigThatFlipsDuringSnapshotsNeverHasItsFilterRun(t *testing.T) {
	cpScratch(t)
	h := &hostileRepo{dir: aGitRepo(t), markers: physical(t.TempDir())}
	git := filepath.Join(h.dir, ".git")
	plain, err := os.ReadFile(filepath.Join(git, "config"))
	if err != nil {
		t.Fatal(err)
	}
	hostile := string(plain) + "[filter \"late\"]\n\tclean = " + h.program(t, "lateclean") + "\n"
	writeRepoFile(t, git, "info/attributes", "* filter=late\n")
	writeRepoFile(t, git, "config", hostile)
	writeRepoFile(t, h.dir, "a.txt", "changed\n")
	plainAdd(t, h.dir)
	if ran := h.ran(); !slices.Contains(ran, "lateclean") {
		t.Fatalf("a plain add didn't run the filter (%v)", ran)
	}
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			text := string(plain)
			if i%2 == 0 {
				text = hostile
			}
			tmp := filepath.Join(git, "config.flip")
			if os.WriteFile(tmp, []byte(text), 0o644) == nil {
				_ = os.Rename(tmp, filepath.Join(git, "config"))
			}
			time.Sleep(time.Duration(i%4) * time.Millisecond)
		}
	}()
	store, kept := newCheckpoints(), 0
	for i := 0; i < 60; i++ {
		writeRepoFile(t, h.dir, "a.txt", "changed "+strconv.Itoa(i)+"\n")
		if cp, _ := store.snapshot(cpDesk(h.dir), "flip"); cp.ID != "" {
			kept++
		}
	}
	close(stop)
	<-done
	if ran := h.ran(); len(ran) != 0 {
		t.Errorf("the store ran the filter: %v", ran)
	}
	if kept == 0 {
		t.Error("no snapshot was kept, so the race proves nothing")
	}
}

// anEvilRepo is a repo a hire made, whose own config and info/attributes name a clean filter, with a changed file.
func anEvilRepo(t *testing.T, h *hostileRepo) string {
	t.Helper()
	evil := aGitRepo(t)
	gitRepoCmd(t, evil, "config", "filter.ev.clean", h.program(t, "evilclean"))
	writeRepoFile(t, evil, ".git/info/attributes", "* filter=ev\n")
	writeRepoFile(t, evil, "a.txt", "evil's own change\n")
	return evil
}

// B6: the desk's .git is swapped for a gitfile or a link to another repo at the worst moments: just before a command
// (and left so, which the look before the write sees), or for the command's run only, put back as soon as it ends,
// which no look before or after can see. The ref write (no git now) is swapped at its worst moment, after the look at
// the folder pinned. Nothing of that repo runs, it gets no ref and no object (every write goes through the folder
// pinned), and a swap that is still there is said in words.
func TestADotGitSwappedForAnotherRepoAtAnyMomentSendsNothingThere(t *testing.T) {
	for _, moment := range []string{"read-tree", "add", "commit-tree", "the ref write"} {
		for _, kind := range []string{"gitfile", "link"} {
			for _, around := range []bool{false, true} {
				for _, mode := range cpAttrModes {
					name := moment + " " + kind + " " + mode.name
					if around {
						name += " put back after"
					}
					t.Run(name, func(t *testing.T) {
						cpScratch(t)
						h := &hostileRepo{markers: physical(t.TempDir())}
						desk := aGitRepo(t)
						writeRepoFile(t, desk, "a.txt", "the desk's change\n")
						evil := anEvilRepo(t, h)
						dotGit, aside := filepath.Join(desk, ".git"), filepath.Join(desk, ".git.aside")
						swap := "mv '" + dotGit + "' '" + aside + "'; printf 'gitdir: %s\\n' '" + filepath.Join(evil, ".git") + "' > '" + dotGit + "'"
						if kind == "link" {
							swap = "mv '" + dotGit + "' '" + aside + "'; ln -s '" + filepath.Join(evil, ".git") + "' '" + dotGit + "'"
						}
						putBack := "rm -f '" + dotGit + "'; mv '" + aside + "' '" + dotGit + "'"
						// The control: plain git in the desk, once swapped, runs the other repo's filter.
						if out, err := exec.Command("sh", "-c", swap).CombinedOutput(); err != nil {
							t.Fatalf("%v %s", err, out)
						}
						plainAdd(t, desk)
						if ran := h.ran(); !slices.Contains(ran, "evilclean") {
							t.Fatalf("plain git in the swapped desk didn't run the filter (%v)", ran)
						}
						if out, err := exec.Command("sh", "-c", putBack).CombinedOutput(); err != nil {
							t.Fatalf("%v %s", err, out)
						}
						evilObjects := gitRepoCmd(t, evil, "count-objects") // after the control's own
						var arm func()
						var fired func() bool
						switch {
						case moment == "the ref write" && around: // swapped and put back between the look and the write
							arm, fired = refWriteBefore(t, swap+"; "+putBack)
						case moment == "the ref write":
							arm, fired = refWriteBefore(t, swap)
						case around:
							arm, fired = gitAround(t, moment, swap, putBack)
						default:
							arm, fired = gitBefore(t, moment, swap)
						}
						withAttrSource(t, mode.on)
						arm()
						cp, note := newCheckpoints().snapshot(cpDesk(desk), "swapped")
						if !fired() {
							t.Fatal("the wrapper never swapped the .git, so the race wasn't run")
						}
						if !around {
							if out, err := exec.Command("sh", "-c", putBack).CombinedOutput(); err != nil {
								t.Fatalf("%v %s", err, out)
							}
							if cp.ID != "" || note != errDotGitMoved.Error() && moment != "read-tree" {
								t.Errorf("not said in words: %+v %q", cp, note)
							}
						}
						if ran := h.ran(); len(ran) != 0 {
							t.Errorf("the store ran: %v", ran)
						}
						if refs := cpRefs(t, evil); refs != "" {
							t.Errorf("the other repo got a ref: %q", refs)
						}
						if got := gitRepoCmd(t, evil, "count-objects"); got != evilObjects {
							t.Errorf("the other repo got objects: %s, was %s", got, evilObjects)
						}
						// The desk has a ref only if it was said, and only to its own snapshot.
						refs := cpRefs(t, desk)
						switch {
						case cp.ID == "" && refs != "":
							t.Errorf("the desk got a ref that wasn't said: %q (%q)", refs, note)
						case cp.ID != "" && gitRepoCmd(t, desk, "show", cp.ID+":a.txt") != "the desk's change":
							t.Errorf("the snapshot isn't the desk's")
						}
					})
				}
			}
		}
	}
}

// B6 for real, with no wrapper: the desk's .git flips between itself and a link to another repo's while snapshots run.
// The other repo's filter never runs, and it gets no ref and no object; every ref the desk gets is to a commit it has,
// of its own files.
func TestADotGitThatFlipsToAnotherRepoDuringSnapshotsSendsNothingThere(t *testing.T) {
	cpScratch(t)
	if runtime.GOOS == "windows" {
		t.Skip("links")
	}
	h := &hostileRepo{markers: physical(t.TempDir())}
	desk := aGitRepo(t)
	evil := anEvilRepo(t, h)
	evilObjects := gitRepoCmd(t, evil, "count-objects")
	dotGit, aside := filepath.Join(desk, ".git"), filepath.Join(desk, ".git.aside")
	stop, done := make(chan struct{}), make(chan struct{})
	strays := 0
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			if os.Rename(dotGit, aside) == nil {
				_ = os.Symlink(filepath.Join(evil, ".git"), dotGit)
				time.Sleep(time.Duration(i%4) * time.Millisecond)
				// A git of the store's that started on the desk's path as it went can have made folders there for a
				// ref of its own (git makes a ref's folders as it needs them): they go, as the loop's own mess.
				if info, err := os.Lstat(dotGit); err == nil && info.IsDir() {
					strays++
				}
				_ = os.RemoveAll(dotGit)
				_ = os.Rename(aside, dotGit)
			}
			time.Sleep(time.Duration(i%30) * time.Millisecond) // the desk as it is, long enough for a snapshot now and then
		}
	}()
	store, kept, notes := newCheckpoints(), 0, map[string]int{}
	var ids []string
	for i := 0; i < 80; i++ {
		writeRepoFile(t, desk, "a.txt", "the desk's change "+strconv.Itoa(i)+"\n")
		if cp, note := store.snapshot(cpDesk(desk), "flip"); cp.ID != "" {
			kept++
			ids = append(ids, cp.ID)
		} else {
			notes[note]++
		}
	}
	close(stop)
	<-done
	if info, err := os.Lstat(dotGit); err != nil || !info.IsDir() {
		t.Fatalf("the loop didn't put the desk's .git back: %v", err)
	}
	if ran := h.ran(); len(ran) != 0 {
		t.Errorf("the store ran: %v", ran)
	}
	if refs := cpRefs(t, evil); refs != "" {
		t.Errorf("the other repo got refs: %q", refs)
	}
	if got := gitRepoCmd(t, evil, "count-objects"); got != evilObjects {
		t.Errorf("the other repo got objects: %s, was %s", got, evilObjects)
	}
	for _, ref := range strings.Fields(cpRefs(t, desk)) {
		if text := gitRepoCmd(t, desk, "show", ref+":a.txt"); !strings.HasPrefix(text, "the desk's change") {
			t.Errorf("%s isn't of the desk's files: %q", ref, text)
		}
	}
	for _, id := range ids {
		gitRepoCmd(t, desk, "cat-file", "-e", id+"^{commit}")
	}
	if kept == 0 {
		t.Error("no snapshot was kept, so the race proves nothing")
	}
	t.Logf("%d of 80 kept, %d stray folders cleared; the others: %v", kept, strays, notes)
}

// B4 (P3): a desk whose .git is a one-line gitfile to another repo's submodule git folder, whose core.worktree names
// that submodule's checkout. Git would snapshot the owner's checkout and write the ref in its repo: refused in words.
func TestADotGitFileThatSendsGitToAWorkingFolderElsewhereIsRefused(t *testing.T) {
	cpScratch(t)
	source := aGitRepo(t)
	owner := aGitRepo(t)
	gitRepoCmd(t, owner, "-c", "protocol.file.allow=always", "submodule", "add", "-q", source, "sub")
	gitRepoCmd(t, owner, "commit", "-q", "-m", "a submodule")
	writeRepoFile(t, owner, "sub/owner-secret.txt", "the owner's\n")
	desk := physical(t.TempDir())
	writeRepoFile(t, desk, "desk.txt", "the desk's\n")
	writeRepoFile(t, desk, ".git", "gitdir: "+filepath.Join(owner, ".git", "modules", "sub")+"\n")
	store := newCheckpoints()
	words := "sends git to a working folder somewhere else"
	if cp, note := store.snapshot(cpDesk(desk), "x"); cp.ID != "" || !strings.Contains(note, words) {
		t.Errorf("%+v %q", cp, note)
	}
	if _, note := store.list(cpDesk(desk)); !strings.Contains(note, words) {
		t.Errorf("%q", note)
	}
	if note := store.forget(cpDesk(desk)); !strings.Contains(note, words) {
		t.Errorf("%q", note)
	}
	if refs := cpRefs(t, filepath.Join(owner, "sub")); refs != "" {
		t.Errorf("the owner's submodule repo got %q", refs)
	}
}

// RT126C1c N1 (P3c): a .git (a gitfile, or a link) naming another folder's own .git is refused in words; a linked
// worktree's git folder has to point back at the desk's .git.
func TestADotGitNamingAnotherFoldersGitFolderIsRefused(t *testing.T) {
	cpScratch(t)
	other := aGitRepo(t)
	store := newCheckpoints()
	words := "isn't this folder's own"
	for _, kind := range []string{"gitfile", "link"} {
		desk := physical(t.TempDir())
		writeRepoFile(t, desk, "desk.txt", "the desk's\n")
		if kind == "gitfile" {
			writeRepoFile(t, desk, ".git", "gitdir: "+filepath.Join(other, ".git")+"\n")
		} else if err := os.Symlink(filepath.Join(other, ".git"), filepath.Join(desk, ".git")); err != nil {
			t.Skip("no links here")
		}
		if cp, note := store.snapshot(cpDesk(desk), kind); cp.ID != "" || !strings.Contains(note, words) {
			t.Errorf("%s: %+v %q", kind, cp, note)
		}
	}
	// A gitfile to another worktree's git folder (whose gitdir names that worktree's .git, not this one).
	linked := filepath.Join(physical(t.TempDir()), "wt")
	gitRepoCmd(t, other, "worktree", "add", "-q", "-b", "side", linked)
	desk := physical(t.TempDir())
	writeRepoFile(t, desk, ".git", "gitdir: "+filepath.Join(other, ".git", "worktrees", "wt")+"\n")
	if cp, note := store.snapshot(cpDesk(desk), "worktree"); cp.ID != "" || !strings.Contains(note, words) {
		t.Errorf("worktree: %+v %q", cp, note)
	}
	if refs := cpRefs(t, other); refs != "" {
		t.Errorf("the other repo got %q", refs)
	}
	// The worktree itself is fine, even with a core.worktree in the shared config (P3b).
	gitRepoCmd(t, other, "config", "core.worktree", other)
	writeRepoFile(t, linked, "hire.txt", "the hire's\n")
	cp := newSnap(t, store, checkpointDesk{Session: "w1", Folder: linked}, "linked")
	if names := gitRepoCmd(t, other, "ls-tree", "-r", "--name-only", cp.ID); names != "a.txt\nhire.txt\nsrc/b.txt" {
		t.Errorf("%q", names)
	}
}

// The private git folder add runs in holds what the store wrote and nothing of the repo's config or attributes: the
// wrapper copies it as add starts, in the hostile repo (filters in its config, an include, info/attributes, hooks).
func TestThePrivateGitFolderHoldsNothingOfTheReposConfigOrAttributes(t *testing.T) {
	cpScratch(t)
	h := aHostileGitRepo(t)
	writeRepoFile(t, h.dir, ".git/info/exclude", "*.mine\n")
	writeRepoFile(t, h.dir, "x.mine", "ignored by info/exclude\n")
	capture := filepath.Join(physical(t.TempDir()), "private")
	arm, fired := gitBefore(t, "add", "for b in \"$@\"; do case \"$b\" in --git-dir='"+cpFolder()+"'/*) cp -R \"${b#--git-dir=}\" '"+capture+"';; esac; done")
	withAttrSource(t, true)
	arm()
	cp := newSnap(t, newCheckpoints(), cpDesk(h.dir), "private")
	if !fired() {
		t.Fatal("the wrapper never ran")
	}
	var names []string
	_ = filepath.WalkDir(capture, func(path string, d fs.DirEntry, err error) error {
		if err == nil && path != capture {
			rel, _ := filepath.Rel(capture, path)
			names = append(names, filepath.ToSlash(rel))
			if data, err := os.ReadFile(path); err == nil && rel != "index" {
				for _, word := range []string{"filter", "fsmonitor", "hooks", "include", h.markers, ".sh"} {
					if strings.Contains(string(data), word) {
						t.Errorf("%s holds %q:\n%s", rel, word, data)
					}
				}
			}
		}
		return nil
	})
	if got := strings.Join(names, " "); got != "HEAD config index info info/attributes info/exclude objects objects/info objects/info/alternates refs" {
		t.Errorf("the private git folder holds %q", got)
	}
	// Its one line of the repo's own: where the repo's objects are, to be read (an object is named by its hash).
	store, _ := filepath.EvalSymlinks(filepath.Join(h.dir, ".git", "objects"))
	if data, _ := os.ReadFile(filepath.Join(capture, "objects", "info", "alternates")); string(data) != store+"\n" {
		t.Errorf("%q", data)
	}
	if keys := gitRepoCmd(t, h.dir, "config", "--file", filepath.Join(capture, "config"), "--name-only", "--list"); keys != "core.repositoryformatversion\ncore.bare\nextensions.objectformat" {
		t.Errorf("%q", keys)
	}
	if data, _ := os.ReadFile(filepath.Join(capture, "info", "attributes")); len(data) != 0 {
		t.Errorf("%q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(capture, "info", "exclude")); string(data) != "*.mine\n" {
		t.Errorf("the repo's info/exclude wasn't carried: %q", data)
	}
	if ran := h.ran(); len(ran) != 0 {
		t.Errorf("the store ran: %v", ran)
	}
	if names := gitRepoCmd(t, h.dir, "ls-tree", "-r", "--name-only", cp.ID); strings.Contains(names, "x.mine") {
		t.Errorf("a file info/exclude ignores is in the snapshot: %q", names)
	}
}

// What the repo says that shapes a snapshot is carried into the private git folder: its own ignore file, its sparse
// checkout (a file out of the cone stays as in HEAD, not deleted), the letter case of a disk that ignores it. The tree
// is the one plain git makes.
func TestTheReposOwnIgnoreFileSparseCheckoutAndLetterCaseAreKept(t *testing.T) {
	cpScratch(t)
	withAttrSource(t, true) // and no owner's ignore file, which would win over the repo's
	dir := aGitRepo(t)
	writeRepoFile(t, dir, "other/c.txt", "c\n")
	gitRepoCmd(t, dir, "add", ".")
	gitRepoCmd(t, dir, "commit", "-q", "-m", "other")
	ignore := filepath.Join(physical(t.TempDir()), "ignore")
	writeRepoFile(t, filepath.Dir(ignore), "ignore", "*.secret\n")
	gitRepoCmd(t, dir, "config", "core.excludesFile", ignore)
	writeRepoFile(t, dir, "x.secret", "kept out\n")
	writeRepoFile(t, dir, "src/new.txt", "new\n")
	gitRepoCmd(t, dir, "sparse-checkout", "set", "src")
	if _, err := os.Stat(filepath.Join(dir, "other", "c.txt")); err == nil {
		t.Fatal("the sparse checkout left other/ on disk, so this proves less")
	}
	if caseInsensitive(dir) {
		if err := os.Rename(filepath.Join(dir, "a.txt"), filepath.Join(dir, "A.txt")); err != nil {
			t.Fatal(err)
		}
	}
	cp := newSnap(t, newCheckpoints(), cpDesk(dir), "kept")
	names := gitRepoCmd(t, dir, "ls-tree", "-r", "--name-only", cp.ID)
	if names != "a.txt\nother/c.txt\nsrc/b.txt\nsrc/new.txt" {
		t.Errorf("%q", names)
	}
	index := filepath.Join(t.TempDir(), "index")
	var plain string
	for _, args := range [][]string{{"read-tree", "HEAD"}, {"add", "-A"}, {"write-tree"}} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, out)
		}
		plain = strings.TrimSpace(string(out))
	}
	if tree := gitRepoCmd(t, dir, "rev-parse", cp.ID+"^{tree}"); tree != plain {
		t.Errorf("the snapshot's tree %s isn't plain git's %s", tree, plain)
	}
}

// caseInsensitive says the disk of dir ignores letter case.
func caseInsensitive(dir string) bool {
	probe := filepath.Join(dir, ".case-probe")
	if os.WriteFile(probe, nil, 0o644) != nil {
		return false
	}
	defer os.Remove(probe)
	_, err := os.Stat(filepath.Join(dir, ".CASE-PROBE"))
	return err == nil
}

// The objects go to the desk's own store, named by its resolved path, whatever else says otherwise: a clone --shared
// reads its alternate and writes nothing in it; the app's environment naming another repo is dropped; a linked
// worktree's commondir rewritten to another repo just before the ref is written doesn't move it (the refs are named by
// the common git folder itself); and a repo shared with its group gets objects its group can read.
func TestTheObjectsAndTheRefGoToTheDesksOwnRepoWhateverElseSays(t *testing.T) {
	cpScratch(t)
	other := aGitRepo(t)
	origin := aGitRepo(t)
	shared := filepath.Join(physical(t.TempDir()), "shared")
	gitRepoCmd(t, origin, "clone", "-q", "--shared", origin, shared)
	gitRepoCmd(t, shared, "config", "commit.gpgsign", "false")
	writeRepoFile(t, shared, "a.txt", "the clone's change\n")
	count := func(dir string) string { return gitRepoCmd(t, dir, "count-objects") }
	originObjects := count(origin)
	for _, kv := range [][2]string{{"GIT_DIR", filepath.Join(other, ".git")}, {"GIT_WORK_TREE", other}, {"GIT_INDEX_FILE", filepath.Join(other, ".git", "index")},
		{"GIT_OBJECT_DIRECTORY", filepath.Join(other, ".git", "objects")}, {"GIT_COMMON_DIR", filepath.Join(other, ".git")}} {
		t.Setenv(kv[0], kv[1])
	}
	store := newCheckpoints()
	cp, note := store.snapshot(cpDesk(shared), "shared")
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_COMMON_DIR"} {
		os.Unsetenv(name)
	}
	if note != "" || cp.ID == "" {
		t.Fatalf("%q", note)
	}
	if got := gitRepoCmd(t, shared, "show", cp.ID+":a.txt"); got != "the clone's change" {
		t.Errorf("%q", got)
	}
	if count(origin) != originObjects {
		t.Errorf("the alternate got objects: %s, was %s", count(origin), originObjects)
	}
	if refs := cpRefs(t, other); refs != "" || cpRefs(t, origin) != "" {
		t.Errorf("another repo got a ref: %q", refs)
	}
	// A linked worktree whose commondir is rewritten to another repo's just before the write.
	main := aGitRepo(t)
	linked := filepath.Join(physical(t.TempDir()), "wt")
	gitRepoCmd(t, main, "worktree", "add", "-q", "-b", "side", linked)
	commondir := filepath.Join(main, ".git", "worktrees", "wt", "commondir")
	was, err := os.ReadFile(commondir)
	if err != nil {
		t.Fatal(err)
	}
	arm, fired := refWriteBefore(t, "printf '%s\\n' '"+filepath.Join(other, ".git")+"' > '"+commondir+"'")
	arm()
	cp = newSnap(t, store, checkpointDesk{Session: "w1", Folder: linked}, "commondir")
	if !fired() {
		t.Fatal("the rewrite never ran")
	}
	if err := os.WriteFile(commondir, was, 0o644); err != nil {
		t.Fatal(err)
	}
	if refs := cpRefs(t, main); refs != "refs/smolcorp/checkpoints/w1/1" {
		t.Errorf("the ref isn't in the worktree's repo: %q", refs)
	}
	if refs := cpRefs(t, other); refs != "" {
		t.Errorf("the other repo got %q", refs)
	}
	// Shared with the group: the snapshot's objects are as the repo's own would be.
	if runtime.GOOS != "windows" {
		group := aGitRepo(t)
		gitRepoCmd(t, group, "config", "core.sharedRepository", "group")
		writeRepoFile(t, group, "g.txt", "for the group\n")
		cp = newSnap(t, store, checkpointDesk{Session: "g1", Folder: group}, "group")
		object := filepath.Join(group, ".git", "objects", cp.ID[:2], cp.ID[2:])
		if info, err := os.Stat(object); err != nil || info.Mode().Perm()&0o040 == 0 {
			t.Errorf("the commit's object isn't readable by the group: %v %v", info.Mode(), err)
		}
	}
}

// B8, the verdict's P1a: a wrapper git, just before add, writes a filter into every private git folder it finds in
// $TMPDIR (its config and its info/attributes), as any program of the owner's account can. The private folder isn't
// there: it is in the support folder, 0700, so the wrapper finds nothing and nothing runs.
func TestThePrivateGitFolderIsInTheSupportFolderNotInTheTempFolder(t *testing.T) {
	left := cpScratch(t)
	h := &hostileRepo{dir: aGitRepo(t), markers: physical(t.TempDir())}
	program := h.program(t, "tmpclean")
	writeRepoFile(t, h.dir, "a.txt", "changed\n")
	found, gitDirs := filepath.Join(h.markers, "found"), filepath.Join(h.markers, "gitdirs")
	arm, fired := gitBefore(t, "add", "for c in \"$TMPDIR\"/*/git/config \"$TMPDIR\"/*/config; do if [ -f \"$c\" ]; then "+
		"printf '[filter \"tmp\"]\\n\\tclean = %s\\n' '"+program+"' >> \"$c\"; mkdir -p \"$(dirname \"$c\")/info\"; "+
		"echo '* filter=tmp' > \"$(dirname \"$c\")/info/attributes\"; echo \"$c\" >> '"+found+"'; fi; done; "+
		"for b in \"$@\"; do case \"$b\" in --git-dir=*) d=\"${b#--git-dir=}\"; echo \"$d $(ls -ld \"$d\" | cut -c1-10) $(ls -ld \"$(dirname \"$d\")\" | cut -c1-10)\" >> '"+gitDirs+"';; esac; done")
	for _, mode := range cpAttrModes {
		withAttrSource(t, mode.on)
		arm()
		cp, note := newCheckpoints().snapshot(cpDesk(h.dir), "tmp")
		if !fired() {
			t.Fatal("the wrapper never ran")
		}
		if note != "" || cp.ID == "" {
			t.Errorf("%s: the snapshot was skipped: %q", mode.name, note)
		}
		if data, _ := os.ReadFile(found); len(data) != 0 {
			t.Errorf("%s: a private git folder was found in the temp folder: %s", mode.name, data)
		}
		if ran := h.ran(); len(ran) != 0 {
			t.Errorf("%s: the store ran: %v", mode.name, ran)
		}
		data, _ := os.ReadFile(gitDirs)
		_ = os.Remove(gitDirs)
		if fields := strings.Fields(string(data)); len(fields) != 3 || filepath.Dir(filepath.Dir(fields[0])) != cpFolder() ||
			fields[1] != "drwx------" || fields[2] != "drwx------" {
			t.Errorf("%s: add ran in %q, not a folder of %s only the app can open", mode.name, data, cpFolder())
		}
	}
	if info, err := os.Lstat(cpFolder()); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Errorf("the store's folder: %v %v", info, err)
	}
	if names := left(); len(names) != 0 {
		t.Errorf("%v", names)
	}
}

// B8, the verdict's P1b, for real with no wrapper: something watches $TMPDIR while snapshots run and writes a filter into
// any git folder it sees. It never finds one, and nothing runs.
func TestATempFolderWatcherFindsNoPrivateGitFolderToWriteIn(t *testing.T) {
	cpScratch(t)
	h := &hostileRepo{dir: aGitRepo(t), markers: physical(t.TempDir())}
	filter := "[filter \"tmp\"]\n\tclean = " + h.program(t, "tmpclean") + "\n"
	stop, done := make(chan struct{}), make(chan int)
	go func() {
		wrote := 0
		for {
			select {
			case <-stop:
				done <- wrote
				return
			default:
			}
			configs, _ := filepath.Glob(filepath.Join(os.TempDir(), "*", "git", "config"))
			for _, config := range configs {
				if file, err := os.OpenFile(config, os.O_APPEND|os.O_WRONLY, 0); err == nil {
					_, _ = file.WriteString(filter)
					_ = file.Close()
					_ = os.WriteFile(filepath.Join(filepath.Dir(config), "info", "attributes"), []byte("* filter=tmp\n"), 0o600)
					wrote++
				}
			}
		}
	}()
	store := newCheckpoints()
	var notes []string
	for i := 0; i < 20; i++ {
		writeRepoFile(t, h.dir, "a.txt", strconv.Itoa(i)+"\n")
		if _, note := store.snapshot(cpDesk(h.dir), "watched"); note != "" {
			notes = append(notes, note)
		}
	}
	close(stop)
	if len(notes) != 0 {
		t.Errorf("snapshots were skipped: %q", notes)
	}
	if wrote := <-done; wrote != 0 {
		t.Errorf("the watcher wrote a filter into %d private git folders", wrote)
	}
	if ran := h.ran(); len(ran) != 0 {
		t.Errorf("the store ran: %v", ran)
	}
}

// B8: a snapshot's private folder is one it made itself. A folder already there under the name it would take (planted,
// with a filter in it) is refused in words, used for nothing and left as it was; and what a crash left in the store's
// folder is swept before a store's first snapshot.
func TestAPrivateFolderAlreadyThereIsRefusedAndLeftoversAreSwept(t *testing.T) {
	left := cpScratch(t)
	h := &hostileRepo{dir: aGitRepo(t), markers: physical(t.TempDir())}
	store := newCheckpoints()
	newSnap(t, store, cpDesk(h.dir), "first")
	planted := filepath.Join(cpFolder(), "PLANTED")
	config := "[filter \"p\"]\n\tclean = " + h.program(t, "plantedclean") + "\n"
	writeRepoFile(t, planted, "git/config", config)
	writeRepoFile(t, planted, "git/info/attributes", "* filter=p\n")
	defer func(name func() string) { cpFolderName = name }(cpFolderName)
	cpFolderName = func() string { return "PLANTED" }
	writeRepoFile(t, h.dir, "a.txt", "changed\n")
	if cp, note := store.snapshot(cpDesk(h.dir), "planted"); cp.ID != "" || !strings.Contains(note, "private folder couldn't be made") {
		t.Errorf("%+v %q", cp, note)
	}
	if data, err := os.ReadFile(filepath.Join(planted, "git", "config")); err != nil || string(data) != config {
		t.Errorf("the folder that was there was changed or removed: %q %v", data, err)
	}
	if ran := h.ran(); len(ran) != 0 {
		t.Errorf("the store ran: %v", ran)
	}
	if refs := cpRefs(t, h.dir); refs != "refs/smolcorp/checkpoints/s1/1" {
		t.Errorf("%q", refs)
	}
	// A crash's leftovers (this one included) go before the next store's first snapshot, and only then.
	cpFolderName = rand.Text
	newSnap(t, newCheckpoints(), cpDesk(h.dir), "swept")
	if names := left(); len(names) != 0 {
		t.Errorf("left behind: %v", names)
	}
	if ran := h.ran(); len(ran) != 0 {
		t.Errorf("the store ran: %v", ran)
	}
}

// RT126C1R4 N2: a repo whose .git/objects is a link (moved elsewhere and linked back) is said in words of its own.
func TestARepoWhoseObjectsFolderIsALinkIsSaidInWords(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links")
	}
	cpScratch(t)
	dir := aGitRepo(t)
	moved := filepath.Join(physical(t.TempDir()), "objects")
	if err := os.Rename(filepath.Join(dir, ".git", "objects"), moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, filepath.Join(dir, ".git", "objects")); err != nil {
		t.Fatal(err)
	}
	gitRepoCmd(t, dir, "status")
	if cp, note := newCheckpoints().snapshot(cpDesk(dir), "x"); cp.ID != "" || !strings.Contains(note, ".git/objects folder is a link") {
		t.Errorf("%+v %q", cp, note)
	}
	if refs := cpRefs(t, dir); refs != "" {
		t.Errorf("%q", refs)
	}
}

// RT126C1R4 N3: GIT_LITERAL_PATHSPECS (and the other pathspec switches) in the app's environment change nothing: a repo
// with a submodule is snapshotted, with the submodule left out of add as ever.
func TestThePathspecSwitchesInTheAppsEnvironmentChangeNothing(t *testing.T) {
	cpScratch(t)
	h, _, _ := aHostileSubmodule(t)
	for _, name := range []string{"GIT_LITERAL_PATHSPECS", "GIT_GLOB_PATHSPECS", "GIT_ICASE_PATHSPECS"} {
		t.Setenv(name, "1")
	}
	cp := newSnap(t, newCheckpoints(), cpDesk(h.dir), "literal")
	if names := gitRepoCmd(t, h.dir, "ls-tree", "-r", "--name-only", cp.ID); !strings.Contains(names, "sub") {
		t.Errorf("%q", names)
	}
	if ran := h.ran(); len(ran) != 0 {
		t.Errorf("the store ran: %v", ran)
	}
}

// RT126CK1 B1: core.sharedRepository read as git reads it (git_config_perm), for the values the store follows; any
// other is refused.
func TestCoreSharedRepositoryIsReadAsGitReadsIt(t *testing.T) {
	if perm, ok := cpSharedPerm("", false); perm != 0 || !ok {
		t.Errorf("unset: %o %v", perm, ok)
	}
	for value, want := range map[string]int{
		"": 0, "umask": 0, "false": 0, "FALSE": 0, "no": 0, "off": 0, "0": 0, "000": 0,
		"group": 0o660, "true": 0o660, "Yes": 0o660, "on": 0o660, "1": 0o660, "01": 0o660,
		"all": 0o664, "world": 0o664, "everybody": 0o664, "2": 0o664,
		"0600": -0o600, "0640": -0o640, "0660": -0o660, "0644": -0o644, "0700": -0o600, "0777": -0o666, "640": -0o640,
	} {
		if perm, ok := cpSharedPerm(value, true); perm != want || !ok {
			t.Errorf("%q: %o %v, want %o", value, perm, ok, want)
		}
	}
	// Git reads "1k" and "08" as true and "-1" as a mode; it refuses "Group", "UMASK", "0440" and "10" (the octal 8).
	for _, value := range []string{"Group", "UMASK", "1k", "08", "-1", " 0640", "0640 ", "0440", "10", "0o640", "0x1b0", "123456789"} {
		if perm, ok := cpSharedPerm(value, true); ok {
			t.Errorf("%q was taken, as %o", value, perm)
		}
	}
}

// RT126CK1 B1: the modes git's calc_shared_perm and adjust_shared_perm give a file or a folder just made (with umask
// 022 or 077), worked out by hand from git's source.
func TestTheSharedModesAreGits(t *testing.T) {
	for _, c := range []struct {
		shared int
		made   fs.FileMode
		folder bool
		want   fs.FileMode
	}{
		{0o660, 0o644, false, 0o664},
		{0o660, 0o600, false, 0o660},
		{0o660, 0o755, true, 0o775 | fs.ModeSetgid},
		{0o660, 0o700, true, 0o770 | fs.ModeSetgid},
		{0o664, 0o644, false, 0o664},
		{0o664, 0o755, true, 0o775 | fs.ModeSetgid},
		{-0o600, 0o644, false, 0o600},
		{-0o600, 0o755, true, 0o700},
		{-0o640, 0o644, false, 0o640},
		{-0o640, 0o755, true, 0o750 | fs.ModeSetgid},
		{-0o640, 0o700, true, 0o750 | fs.ModeSetgid},
		{-0o600, 0o755 | fs.ModeSetgid, true, 0o700 | fs.ModeSetgid}, // a setgid folder's new folder keeps it, as in git
		{0o660, 0o444, false, 0o444},                                 // no write for the owner: none is given
	} {
		made := c.made
		if c.folder {
			made |= fs.ModeDir
		}
		if got := cpSharedMode(c.shared, made, c.folder); got != c.want {
			t.Errorf("shared %o, made %v: %v, want %v", c.shared, c.made, got, c.want)
		}
	}
}

// RT126CK1 B1: a core.sharedRepository that git takes but the store can't follow exactly ("1k", which git reads as
// true) is refused in words, and nothing is written.
func TestACoreSharedRepositoryTheStoreCantFollowIsRefused(t *testing.T) {
	cpScratch(t)
	desk := aGitRepo(t)
	gitRepoCmd(t, desk, "config", "core.sharedRepository", "1k")
	gitRepoCmd(t, desk, "rev-parse", "HEAD") // git takes it
	cp, note := newCheckpoints().snapshot(cpDesk(desk), "refused")
	if cp.ID != "" || !strings.Contains(note, "core.sharedRepository") {
		t.Errorf("the snapshot: %+v %q", cp, note)
	}
	if _, err := os.Lstat(filepath.Join(desk, ".git", "refs", "smolcorp")); err == nil {
		t.Error("a folder of checkpoint refs was made")
	}
}
