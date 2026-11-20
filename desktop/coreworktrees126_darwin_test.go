package desktop

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXCOREWORKTREES126: every new hire's worktree in the core's own folder, ~/.smolcorp/worktrees (the test's stand-in
// home), which no GM's Policy lets it write; the start check and the accessors refuse a tree that turned anyway; a GM's
// hire stalls only itself. Every link, FIFO and repo here is in the tests' own temp folders, removed by Go.

// gmPlacesNow is the GM's Places for l.gm, as its every start makes them.
func gmPlacesNow(t *testing.T, l gmRootLayout) (sandbox.Places, *sandbox.Policy) {
	t.Helper()
	pl, err := deskPlaces(deskAtOf(l.gm, deskSocketPath(l.gm.ID, testCopyPort()), nil, engineOptions{Git: runGit}))
	if err != nil {
		t.Fatalf("the GM's Places: %v", err)
	}
	pol, err := sandbox.New(pl)
	if err != nil {
		t.Fatalf("the GM's Policy: %v", err)
	}
	return pl, pol
}

// The owner's office hire into ~/Projects/app with a GM open: its tree is made in the core's own folder (0700, the
// folder above it too), on its branch, with a clean first status and a commit that lands; its first message says where
// its work is and how to look at it. The GM's Policy reads all of the core's folder and writes none of it, nor the
// folder above: the end states of every tree-side probe of the F-8 reviews (a tree swapped as git's worktree add made
// it, after the carry, a submodule's tree, the repo's folder made a link for discardHalfMade) are writes there. And a
// tree that turned anyway (as only the owner could) is refused at its start, by its accessor and by the fire's look,
// with no git run where the link leads.
func TestEveryNewHiresTreeIsInTheCoresFolderWhichNoGMWrites(t *testing.T) {
	d, l, lib2, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	_ = lib2
	hire := pickedHire(t, d, "app")
	d.watching.Wait()
	tree := hire.Worktree.storedOnly()
	core := coreWorktreesFolder()
	if filepath.Dir(tree) != hiresFolder(l.app, "") || filepath.Dir(filepath.Dir(tree)) != core || hireTreeMoved(hire) {
		t.Fatalf("its tree: %s, want it in %s", tree, hiresFolder(l.app, ""))
	}
	for _, folder := range []string{filepath.Dir(core), core, filepath.Dir(tree)} {
		if info, err := os.Lstat(folder); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
			t.Errorf("%s: %v %v, want a folder of the owner's alone", folder, info, err)
		}
	}
	// Its first message, as the engine makes it for that place (engineBrief).
	d.mu.Lock()
	role := d.config.Roles[0]
	d.mu.Unlock()
	message, _, err := engineBrief(engineRequest{Role: role, Folder: l.app}, hirePlace{Folder: tree, Worktree: tree, Branch: hire.Branch, Repo: l.app},
		"fedcba654321", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "Your work is in your own worktree, "+shownPath(tree)+", on the branch "+hire.Branch+".") ||
		!strings.Contains(message, "git diff main..."+hire.Branch+" from their own checkout") {
		t.Errorf("its first message says nothing of where its work is:\n%s", message)
	}
	if out := gitRepoCmd(t, tree, "status", "--porcelain"); strings.TrimSpace(out) != "" {
		t.Errorf("its first status isn't clean: %q", out)
	}
	writeFile(t, filepath.Join(tree, "work.txt"), "done\n")
	gitRepoCmd(t, tree, "add", "work.txt")
	gitRepoCmd(t, tree, "commit", "-q", "-m", "work")
	if log := gitRepoCmd(t, l.app, "log", "--oneline", hire.Branch); !strings.Contains(log, "work") {
		t.Errorf("its commit isn't on its branch: %q", log)
	}

	pl, pol := gmPlacesNow(t, l)
	if len(pl.Worktrees) == 0 || pl.Worktrees[0] != core {
		t.Fatalf("the GM's Places hold no core folder: %v", pl.Worktrees)
	}
	for _, at := range []string{
		filepath.Join(tree, "work.txt"), // its work
		tree,                            // the tree swapped for a link, after git's add or the carry
		filepath.Join(tree, "mod"),      // a submodule's tree swapped
		filepath.Dir(tree),              // the repo's folder made a link (discardHalfMade's parent)
		filepath.Join(filepath.Dir(tree), "x-1234"), // a tree made ahead of the core's
		core,               // the core's folder moved aside
		filepath.Dir(core), // the folder above it moved, and made again
	} {
		if v, by := pol.Decide(at, sandbox.Write); v != sandbox.Deny {
			t.Errorf("the GM may write %s (%q)", at, by.Row)
		}
	}
	if v, by := pol.Decide(filepath.Join(tree, "work.txt"), sandbox.Read); v != sandbox.Allow {
		t.Errorf("the GM can't read its hire's work (%q)", by.Row)
	}

	// Turned anyway (the owner moves it, as the GM can't): refused everywhere, and no git where the link leads.
	elsewhere := filepath.Join(physical(t.TempDir()), "elsewhere")
	gitRepoCmd(t, filepath.Dir(elsewhere), "init", "-q", "-b", "main", elsewhere)
	if err := os.Rename(tree, tree+".real"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, tree); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(tree); _ = os.Rename(tree+".real", tree) })
	calls := gitCalls(t)
	if _, err := deskPlaces(deskAtOf(hire, hire.TmuxSocket, nil, engineOptions{Git: runGit})); err == nil || !strings.Contains(err.Error(), "was moved or replaced since it was hired") {
		t.Errorf("its start: %v", err)
	}
	if _, err := hire.checkedTree(); err == nil {
		t.Error("its accessor gave the turned tree")
	}
	if _, _, own := hiresOwnWorktree(runGit, "", hire); own {
		t.Error("the fire's look took the turned tree as its own")
	}
	if got := calls(elsewhere); len(got) > 0 {
		t.Errorf("git ran where the link leads: %q", got)
	}
}

// A GM's own place is never in the core's folder: its start is refused, in words, before anything runs.
func TestAGMsOwnPlaceInTheCoresFolderNeverStarts(t *testing.T) {
	core := coreWorktreesFolder()
	for _, place := range []sandbox.Places{
		{Worktrees: []string{core}, Root: filepath.Join(core, "app-0123abcd", "gm")},
		{Worktrees: []string{core}, Root: "/elsewhere", GMProjects: filepath.Join(core, "app-0123abcd")},
	} {
		if words := gmWorktreesWords(deskAt{GM: true}, place); !strings.Contains(words, "which no General Manager may write, so it didn't start") {
			t.Errorf("a GM at %s: %q", place.Root, words)
		}
		if words := gmWorktreesWords(deskAt{}, place); words != "" {
			t.Errorf("a hire's own tree there is refused: %q", words)
		}
	}
}

// The configured folder for worktrees is honoured only outside every GM's write reach: a plain folder is; one reached
// through a link in the home folder, one holding the home folder or the projects folder, and one in a GM's own place
// are refused, in plain words, and a hire into the owner's repo with it makes nothing.
func TestTheConfiguredWorktreesFolderIsHonouredOnlyOutsideEveryGMsReach(t *testing.T) {
	_, l, _, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	plain := filepath.Join(physical(t.TempDir()), "worktrees")
	if err := os.Mkdir(plain, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(homeDir, "wt-link")
	if err := os.Symlink(plain, link); err != nil {
		t.Fatal(err)
	}
	if words := configuredWorktreesWords(plain); words != "" {
		t.Errorf("a plain folder: %q", words)
	}
	for _, c := range []struct{ folder, words string }{
		{link, "is reached through a link in your home folder"},
		{filepath.Dir(physical(homeDir)), "holds your home folder"},
		{l.projects, "holds the folder for hires"},
		{filepath.Join(gmPlansOf(l.gm), "wt"), "is in a General Manager's own place"},
	} {
		if words := configuredWorktreesWords(c.folder); !strings.Contains(words, c.words) {
			t.Errorf("%s: %q, want %q", c.folder, words, c.words)
		}
	}
	place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: l.app, Worktree: true},
		hirePlaceOptions{Worktrees: link, PortFree: func(int) bool { return true }})
	if err == nil || place.Worktree != "" || !strings.Contains(err.Error(), "Nothing was made.") {
		t.Errorf("a hire with the linked folder: %+v %v", place, err)
	}
	if entries, _ := os.ReadDir(plain); len(entries) != 0 {
		t.Errorf("made in it: %v", entries)
	}
}

// B7's shape: the GM's hire, by its route, into a repo it can write (in the home folder, outside its own places), with
// FIFOs where git reads: it holds only itself. Meanwhile the owner's office hire and Setup's hire go ahead, never
// "Hired a moment ago"; the GM's own next hire is refused until its first ends; and its first ends within its deadline
// and one git timeout.
func TestAGMsHireStallsOnlyItself(t *testing.T) {
	d, l, _, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	saved := gmHireDeadline
	gmHireDeadline = 2 * time.Second
	t.Cleanup(func() { gmHireDeadline = saved })
	x := filepath.Join(homeDir, "code", "x")
	ownersApp(t, x)
	held := []*heldFifo{plantFifo(t, filepath.Join(x, ".git", "HEAD")), plantFifo(t, filepath.Join(x, ".git", "config"))}
	t.Cleanup(func() {
		for _, f := range held {
			f.release()
		}
	})
	gm := gmRouteHire(d, x)
	waitUntil(t, func() bool { return len(fifosOpened(held)) > 0 }, 5*time.Second, "git reading the GM's FIFO")
	again := gmRouteHire(d, l.app)
	if a := <-again; a.code != http.StatusTooManyRequests {
		t.Errorf("the GM's second hire while its first waits: %d %s", a.code, a.body)
	}
	spend(d)
	if status, said := d.Perform("hire", "", url.Values{"role": {"janitor"}, "project": {"app"}}); status != http.StatusNoContent {
		t.Errorf("the owner's office hire meanwhile: %d %s", status, said)
	}
	spend(d)
	if status, answer, why := d.hireFromSetupSheet(setupHire{Role: "janitor", Folder: l.lib, Answers: []string{}, Worktree: true}); status != http.StatusOK {
		t.Errorf("Setup's hire meanwhile: %d %v %q", status, answer, why)
	}
	select {
	case a := <-gm:
		t.Logf("the GM's hire: %d %s in %v", a.code, a.body, a.took.Round(time.Millisecond))
		if limit := gmHireDeadline + worktreeTimeout; a.took > limit {
			t.Errorf("the GM's hire took %v, past its deadline and one git timeout (%v)", a.took, limit)
		}
	case <-time.After(gmHireDeadline + worktreeTimeout + 10*time.Second):
		t.Fatal("the GM's hire never ended")
	}
	d.watching.Wait()
}

// waitUntil waits up to within for ok, polling.
func waitUntil(t *testing.T, ok func() bool, within time.Duration, what string) {
	t.Helper()
	until := time.Now().Add(within)
	for !ok() {
		if time.Now().After(until) {
			t.Fatalf("no %s within %v", what, within)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// gitUntil starts no git once its deadline has passed, and is git as it is before it, or with none.
func TestAGMsHireStartsNoGitPastItsDeadline(t *testing.T) {
	ran := 0
	git := func(string, ...string) (int, string, bool) { ran++; return 0, "", true }
	if _, _, ok := gitUntil(git, time.Now().Add(time.Hour))("", "status"); !ok || ran != 1 {
		t.Errorf("before its deadline: ok %v, ran %d", ok, ran)
	}
	if _, _, ok := gitUntil(git, time.Now().Add(-time.Second))("", "status"); ok || ran != 1 {
		t.Errorf("past its deadline: ok %v, ran %d", ok, ran)
	}
	if _, _, ok := gitUntil(git, time.Time{})("", "status"); !ok || ran != 2 {
		t.Errorf("with no deadline: ok %v, ran %d", ok, ran)
	}
}

// The GM reads and reviews its hires' work in the core's folder: /setup/gm/git?hire= reads a hire's tree there.
func TestAGMReadsItsHiresWorkInTheCoresFolder(t *testing.T) {
	d, l, _, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	managerOn(d)
	hire := pickedHire(t, d, "app")
	d.watching.Wait()
	writeFile(t, filepath.Join(hire.Worktree.storedOnly(), "new.txt"), "x\n")
	_ = l
	got := serve(t, d, call{method: "GET", target: "/setup/gm/git?hire=" + hire.ID, host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	var state GitState
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &state) != nil || !state.Repo || state.Skipped {
		t.Fatalf("the GM's read: %d %s", got.Code, got.Body)
	}
	if len(state.Files) != 1 || state.Files[0].Path != "new.txt" {
		t.Errorf("its files: %+v", state.Files)
	}
}

// The accessors (item 10): a hire's tree and its folder are given only while they resolve to themselves; a no-worktree
// hire's folder in the very spelling its hire judged; the moved words say what was done and what to do.
func TestAHiresTreeAndFolderAreGivenOnlyWhileTheyResolveToThemselves(t *testing.T) {
	base := physical(t.TempDir())
	tree, folder, other := filepath.Join(base, "tree"), filepath.Join(base, "folder"), filepath.Join(base, "other")
	for _, f := range []string{tree, folder, other} {
		if err := os.Mkdir(f, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	h := Hire{ID: "abcdef123456", Worktree: keptPath(tree), Repo: base, Folder: keptPath(base)}
	if got, err := h.checkedTree(); err != nil || got != tree {
		t.Errorf("its tree: %q %v", got, err)
	}
	plain := Hire{ID: "abcdef123457", Folder: keptPath(folder), FolderResolved: folder}
	if got, err := plain.checkedFolder(); err != nil || got != folder {
		t.Errorf("its folder: %q %v", got, err)
	}
	for _, p := range []string{tree, folder} {
		if err := os.Rename(p, p+".real"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(other, p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.checkedTree(); err == nil || err.Error() != movedTreeWords(tree) {
		t.Errorf("its turned tree: %v", err)
	}
	if _, err := h.checkedStart(); err == nil {
		t.Error("its start in a turned tree")
	}
	if _, err := plain.checkedFolder(); err == nil || err.Error() != movedFolderWords(folder) {
		t.Errorf("its turned folder: %v", err)
	}
	if !strings.Contains(movedTreeWords(tree), "was moved or replaced since it was hired, so Smolcorp left it alone") {
		t.Errorf("the words: %q", movedTreeWords(tree))
	}
	// Compared only, as stored.
	if h.Worktree.storedOnly() != tree || plain.storedPlace() != folder {
		t.Error("the stored values changed")
	}
	// In the registry file, the plain strings they always were.
	data, err := json.Marshal(h)
	if err != nil || !strings.Contains(string(data), `"worktree":"`+tree+`"`) || !strings.Contains(string(data), `"folder":"`+base+`"`) {
		t.Errorf("the row as JSON: %s %v", data, err)
	}
	var back Hire
	if err := json.Unmarshal(data, &back); err != nil || back.Worktree != h.Worktree || back.Folder != h.Folder {
		t.Errorf("read back: %+v %v", back, err)
	}
}
