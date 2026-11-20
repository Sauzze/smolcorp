//go:build darwin

package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// FXGMHIRENOGIT126 round 2: the reviewer's B1 (git checked the hire's tree out by its path), B2 (the rule judged one
// spelling of the folder, git ran on another) and O1 (a .git outside a GM's places that leads into one). Every GM is a
// row in the test's own registry; links, renames and the "victim" folders are the test's own, in its temp folders.

// ownersApp makes app a repo of the owner's with one commit: a file, a folder, an executable and a link.
func ownersApp(t *testing.T, app string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(app, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	standInOwnersGit(t, app, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(app, "marker.txt"), "harmless\n")
	writeFile(t, filepath.Join(app, "src", "a.txt"), "a\n")
	writeFile(t, filepath.Join(app, "run.sh"), "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(app, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("marker.txt", filepath.Join(app, "link")); err != nil {
		t.Fatal(err)
	}
	standInOwnersGit(t, app, "add", "marker.txt", "src/a.txt", "run.sh", "link")
	standInOwnersGit(t, app, "commit", "-q", "-m", "first")
}

// swapWhenMade turns tree into a link to victim the moment a folder is there, as a GM's loop would: the folder is
// renamed aside first. The returned func stops the loop and says whether it swapped.
func swapWhenMade(tree, victim string) func() bool {
	var stop atomic.Bool
	done := make(chan bool, 1)
	go func() {
		for !stop.Load() {
			if info, err := os.Lstat(tree); err == nil && info.IsDir() {
				done <- os.Rename(tree, tree+".real") == nil && os.Symlink(victim, tree) == nil
				return
			}
		}
		done <- false
	}()
	return func() bool {
		stop.Store(true)
		return <-done
	}
}

// B1: the owner's hire into a repo outside every GM's own places (~/code/app…, the full flow), with a GM open that may
// write its folder for hires, not held to one project or held to one, has its tree swapped for a link to a temp
// folder the moment the tree's folder is there, over many tries: nothing ever lands in the link's target. Then the
// control: the hire's worktree is checked out whole, as git left it (status and diff-files print nothing), git's two
// links name the real tree and its registration, in either form git writes them, and the hire's own commit works.
func TestAWorktreeAddNeverChecksOutThroughASwappedTree(t *testing.T) {
	for _, gm := range []string{"a GM not held to one project", "a GM held to one project"} {
		t.Run(gm, func(t *testing.T) {
			if strings.Contains(gm, "not held") {
				_, _, _, fifos := gmFolderDeck(t)
				for _, f := range fifos {
					f.release()
				}
			} else {
				scopedGM(t)
			}
			tries, swaps, landed := 50, 0, 0
			for i := range tries {
				app := filepath.Join(homeDir, "code", fmt.Sprintf("app%02d", i))
				ownersApp(t, app)
				if gmOwnsFolder(app) {
					t.Fatal("~/code/app counts as the GM's own")
				}
				victim := filepath.Join(physical(t.TempDir()), "victim")
				if err := os.Mkdir(victim, 0o700); err != nil {
					t.Fatal(err)
				}
				tree := filepath.Join(physical(hiresFolder(physical(app), "")), hireWorktreeName("Ada", "abcdef123456"))
				swapped := swapWhenMade(tree, victim)
				place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: physical(app), Worktree: true},
					hirePlaceOptions{PortFree: func(int) bool { return true }})
				if swapped() {
					swaps++
				}
				if entries, _ := os.ReadDir(victim); len(entries) > 0 {
					landed++
					var names []string
					for _, e := range entries {
						names = append(names, e.Name())
					}
					t.Errorf("try %d: the link's target got %q (place %+v, %v)", i, names, place, err)
				}
			}
			t.Logf("%s: the tree was swapped in %d of %d tries; something landed in the link's target in %d", gm, swaps, tries, landed)
			if swaps == 0 {
				t.Error("the swap never happened: the row proves nothing")
			}
			for _, relative := range []bool{false, true} {
				app := filepath.Join(homeDir, "code", fmt.Sprintf("control-%v", relative))
				ownersApp(t, app)
				if relative {
					standInOwnersGit(t, app, "config", "worktree.useRelativePaths", "true")
				}
				place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: physical(app), Worktree: true},
					hirePlaceOptions{PortFree: func(int) bool { return true }})
				if err != nil || place.Worktree == "" || place.Branch == "" {
					t.Fatalf("the control (relative paths %v): %+v, %v", relative, place, err)
				}
				checkCarriedTree(t, physical(app), place)
			}
			if entries, _ := os.ReadDir(gitReadsFolder()); len(entries) > 0 {
				var names []string
				for _, e := range entries {
					names = append(names, e.Name())
				}
				t.Errorf("the core's folders were left: %q", names)
			}
		})
	}
}

// checkCarriedTree checks a hire's worktree the core carried: its files whole (an executable kept so, a link kept a
// link), git finding them as it left them, git's two links naming the real tree and its registration, and a commit.
func checkCarriedTree(t *testing.T, app string, place hirePlace) {
	t.Helper()
	tree := place.Worktree
	if data := readOr(filepath.Join(tree, "src", "a.txt")); data != "a\n" {
		t.Errorf("src/a.txt in the tree: %q", data)
	}
	if info, err := os.Stat(filepath.Join(tree, "run.sh")); err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("run.sh in the tree isn't executable: %v, %v", info, err)
	}
	if to, err := os.Readlink(filepath.Join(tree, "link")); err != nil || to != "marker.txt" {
		t.Errorf("link in the tree: %q, %v", to, err)
	}
	if out := gitRepoCmd(t, tree, "diff-files", "--name-only"); out != "" {
		t.Errorf("the tree's files aren't as its index has them (diff-files): %q", out)
	}
	if out := gitRepoCmd(t, tree, "status", "--porcelain"); out != "" {
		t.Errorf("the tree isn't clean: %q", out)
	}
	name := filepath.Base(tree)
	registration := filepath.Join(app, ".git", "worktrees", name)
	gitdir := strings.TrimSpace(readOr(filepath.Join(registration, "gitdir")))
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(registration, gitdir)
	}
	if !oneFolderOnDisk(filepath.Dir(gitdir), tree) || filepath.Base(gitdir) != ".git" {
		t.Errorf("the registration's gitdir names %s, not the tree's .git", gitdir)
	}
	if got := gitRepoCmd(t, tree, "rev-parse", "--path-format=absolute", "--git-dir"); !oneFolderOnDisk(got, registration) {
		t.Errorf("the tree's .git names %s, not its registration", got)
	}
	if strings.Contains(readOr(filepath.Join(registration, "gitdir"))+readOr(filepath.Join(tree, ".git")), "git-reads") {
		t.Error("a link names the core's own folder")
	}
	writeFile(t, filepath.Join(tree, "work.txt"), "the hire's\n")
	gitRepoCmd(t, tree, "add", "work.txt")
	gitRepoCmd(t, tree, "commit", "-q", "-m", "the hire's work")
	if got := gitRepoCmd(t, app, "log", "-1", "--format=%s", place.Branch); got != "the hire's work" {
		t.Errorf("the hire's commit on %s: %q", place.Branch, got)
	}
}

// B2: the rule and git see one spelling of the folder. x, in the GM's own plans, is a link the GM turns between the
// owner's lib and its own lib2 (FIFOs at lib2's .git/HEAD and config); the rule judges x resolved, and git then runs
// on that very spelling, never on x again, and a spelling whose links couldn't be resolved gets no git at all: no git
// ever starts in lib2, in prepareHire (turned at the rule's look, and by a loop over 40 hires), at a desk's start
// (deskPlaces) and at a launcher's (launcherRepo).
func TestTheRuleAndGitSeeOneSpellingOfTheFolder(t *testing.T) {
	turn := func(t *testing.T, x, to string) {
		t.Helper()
		_ = os.Remove(x)
		if err := os.Symlink(to, x); err != nil {
			t.Fatal(err)
		}
	}
	// at turns x to "to" once, around the rule's first look, whatever spelling it is asked: after it (the GM winning the
	// race to git's start), or before it (the GM winning the race to the look).
	at := func(t *testing.T, x, to string, after bool) *atomic.Bool {
		saved := gmOwnsFolder
		var swapped atomic.Bool
		gmOwnsFolder = func(f string) bool {
			if !after && swapped.CompareAndSwap(false, true) {
				turn(t, x, to)
			}
			v := saved(f)
			if after && swapped.CompareAndSwap(false, true) {
				turn(t, x, to)
			}
			return v
		}
		t.Cleanup(func() { gmOwnsFolder = saved })
		return &swapped
	}
	t.Run("prepareHire, turned after the rule's look", func(t *testing.T) {
		_, l, lib2, fifos := gmFolderDeck(t)
		x, lib := filepath.Join(gmPlansOf(l.gm), "x"), physical(l.lib)
		turn(t, x, lib)
		swapped := at(t, x, lib2, true)
		calls := gitCalls(t)
		place, err, took := hireWithin(t, 20*time.Second, hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: x, Worktree: true},
			hirePlaceOptions{PortFree: func(int) bool { return true }}, fifos...)
		t.Logf("swapped %v; %v: %+v, %v", swapped.Load(), took.Round(time.Millisecond), place, err)
		if !swapped.Load() {
			t.Fatal("the stand-in never turned x")
		}
		if got := calls(lib2); len(got) > 0 || len(fifosOpened(fifos)) > 0 {
			t.Errorf("git ran in the GM's own lib2: %q (FIFOs opened %q)", got, fifosOpened(fifos))
		}
		if place.Worktree != "" {
			removeWorkspace(place.Repo, place.Worktree, place.Branch, runGit)
		}
	})
	// x, outside every GM's places where a GM not held to one project may write (~/code), can't be resolved when the
	// folder is looked at (here it leads nowhere; a link turned while it is read can make readlink refuse as well),
	// then leads into lib2 by git's start: no git runs at all.
	unresolved := func(t *testing.T) string {
		t.Helper()
		code := filepath.Join(homeDir, "code")
		if err := os.MkdirAll(code, 0o700); err != nil {
			t.Fatal(err)
		}
		return filepath.Join(code, "x")
	}
	t.Run("prepareHire, x not resolved at the look", func(t *testing.T) {
		_, _, lib2, fifos := gmFolderDeck(t)
		x := unresolved(t)
		turn(t, x, filepath.Join(t.TempDir(), "not-there"))
		swapped := at(t, x, lib2, true)
		calls := gitCalls(t)
		place, err, _ := hireWithin(t, 20*time.Second, hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: x, Worktree: true},
			hirePlaceOptions{PortFree: func(int) bool { return true }}, fifos...)
		t.Logf("swapped %v: %+v, %v", swapped.Load(), place, err)
		if !swapped.Load() {
			t.Fatal("the stand-in never turned x")
		}
		if got := calls(lib2, x); len(got) > 0 || len(fifosOpened(fifos)) > 0 {
			t.Errorf("git ran: %q (FIFOs opened %q)", got, fifosOpened(fifos))
		}
	})
	t.Run("launcherRepo, x not resolved at the look", func(t *testing.T) {
		_, _, lib2, fifos := gmFolderDeck(t)
		x := unresolved(t)
		turn(t, x, filepath.Join(t.TempDir(), "not-there"))
		swapped := at(t, x, lib2, true)
		calls := gitCalls(t)
		checkout, hires := launcherRepo(x, "")
		t.Logf("swapped %v: %q %q", swapped.Load(), checkout, hires)
		if got := calls(lib2, x); len(got) > 0 || len(fifosOpened(fifos)) > 0 {
			t.Errorf("git ran: %q (FIFOs opened %q)", got, fifosOpened(fifos))
		}
	})
	t.Run("prepareHire, a loop turning x over 40 hires", func(t *testing.T) {
		_, l, lib2, fifos := gmFolderDeck(t)
		for _, f := range fifos {
			f.release()
		}
		plans := gmPlansOf(l.gm)
		x, tmp, lib := filepath.Join(plans, "x"), filepath.Join(plans, "x.tmp"), physical(l.lib)
		turn(t, x, lib)
		var stop atomic.Bool
		done := make(chan int)
		go func() {
			turns := 0
			for i := 0; !stop.Load(); i++ {
				to := lib
				if i%2 == 1 {
					to = lib2
				}
				_ = os.Remove(tmp)
				if os.Symlink(to, tmp) == nil && os.Rename(tmp, x) == nil {
					turns++
				}
			}
			done <- turns
		}()
		calls := gitCalls(t)
		tries, inLib2 := 40, 0
		for i := range tries {
			before := len(calls(lib2))
			place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: x, Worktree: true},
				hirePlaceOptions{PortFree: func(int) bool { return true }})
			if got := calls(lib2); len(got) > before {
				inLib2++
				t.Errorf("try %d: git ran in lib2: %q (place %+v, %v)", i, got[before:], place, err)
			}
			if place.Worktree != "" {
				removeWorkspace(place.Repo, place.Worktree, place.Branch, runGit)
			}
		}
		stop.Store(true)
		turns := <-done
		t.Logf("git started in the GM's own lib2 in %d of %d hires (x turned %d times)", inLib2, tries, turns)
	})
	t.Run("deskPlaces, turned before the rule's look", func(t *testing.T) {
		_, l, lib2, fifos := gmFolderDeck(t)
		x := filepath.Join(gmPlansOf(l.gm), "x")
		turn(t, x, lib2)
		// Its row as the hire keeps it since FXGMHIRENOGIT126R4 (R10): its folder resolved at its hire, so its start
		// gets as far as the rule's look; a row with none is refused before it here (deskPlacedWords).
		placed := physical(x)
		swapped := at(t, x, physical(l.lib), false)
		calls := gitCalls(t)
		done := make(chan error, 1)
		go func() {
			_, err := deskPlaces(deskAtOf(Hire{ID: "abcdef123456", Role: "janitor", Folder: keptPath(x), FolderResolved: placed}, deskSocketPath("abcdef123456", testCopyPort()), nil, engineOptions{Git: runGit}))
			done <- err
		}()
		select {
		case err := <-done:
			t.Logf("swapped %v; deskPlaces: %v", swapped.Load(), err)
		case <-time.After(20 * time.Second):
			t.Error("deskPlaces was still going after 20s")
			for _, f := range fifos {
				f.release()
			}
			<-done
		}
		if !swapped.Load() {
			t.Fatal("the stand-in never turned x")
		}
		if got := calls(lib2); len(got) > 0 || len(fifosOpened(fifos)) > 0 {
			t.Errorf("git ran in the GM's own lib2: %q (FIFOs opened %q)", got, fifosOpened(fifos))
		}
	})
	t.Run("launcherRepo, turned after the rule's look", func(t *testing.T) {
		_, l, lib2, fifos := gmFolderDeck(t)
		x, lib := filepath.Join(gmPlansOf(l.gm), "x"), physical(l.lib)
		turn(t, x, lib)
		swapped := at(t, x, lib2, true)
		calls := gitCalls(t)
		done := make(chan string, 1)
		go func() {
			checkout, hires := launcherRepo(x, "")
			done <- checkout + " " + hires
		}()
		select {
		case got := <-done:
			t.Logf("swapped %v; launcherRepo: %q", swapped.Load(), got)
		case <-time.After(20 * time.Second):
			t.Error("launcherRepo was still going after 20s")
			for _, f := range fifos {
				f.release()
			}
			<-done
		}
		if !swapped.Load() {
			t.Fatal("the stand-in never turned x")
		}
		if got := calls(lib2); len(got) > 0 || len(fifosOpened(fifos)) > 0 {
			t.Errorf("git ran in the GM's own lib2: %q (FIFOs opened %q)", got, fifosOpened(fifos))
		}
	})
}

// O1: a folder outside every GM's own places whose .git leads into one, by a .git file's gitdir, by a git folder's
// commondir, by a link, or from a folder above, is judged the GM's: a hire there is placed in the folder, told why,
// with no git run and no FIFO in the GM's repo opened. An owner's repo and a worktree of it aren't.
func TestAGitThatLeadsIntoAGMsPlaceGetsNoGit(t *testing.T) {
	_, l, lib2, fifos := gmFolderDeck(t)
	code := filepath.Join(homeDir, "code")
	app := filepath.Join(code, "app")
	ownersApp(t, app)
	worktree := filepath.Join(code, "app-wt")
	standInOwnersGit(t, app, "worktree", "add", "-q", "-b", "wt", worktree)
	f, g, h, i := filepath.Join(code, "f"), filepath.Join(code, "g"), filepath.Join(code, "h"), filepath.Join(code, "i")
	store := filepath.Join(code, "gstore")
	for _, dir := range []string{f, g, h, filepath.Join(i, "sub"), store} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(f, ".git"), "gitdir: "+filepath.Join(lib2, ".git")+"\n")
	writeFile(t, filepath.Join(g, ".git"), "gitdir: ../gstore\n")
	rel, err := filepath.Rel(store, filepath.Join(lib2, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(store, "commondir"), rel+"\n")
	if err := os.Symlink(filepath.Join(lib2, ".git"), filepath.Join(h, ".git")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(i, ".git"), "gitdir: "+filepath.Join(lib2, ".git")+"\n")
	for _, c := range []struct {
		folder string
		owned  bool
	}{{app, false}, {worktree, false}, {f, true}, {g, true}, {h, true}, {filepath.Join(i, "sub"), true}} {
		start := time.Now()
		if got := gmOwnsFolder(c.folder); got != c.owned {
			t.Errorf("gmOwnsFolder(%s) = %v, want %v", c.folder, got, c.owned)
		}
		if took := time.Since(start); took > 2*time.Second {
			t.Errorf("the rule took %v for %s", took, c.folder)
		}
	}
	calls := gitCalls(t)
	for _, folder := range []string{f, g, h, filepath.Join(i, "sub")} {
		place, err, _ := hireWithin(t, 5*time.Second, hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: folder, Worktree: true},
			hirePlaceOptions{PortFree: func(int) bool { return true }}, fifos...)
		inPlace(t, place, err, folder)
	}
	if got := calls(code, gmPlansOf(l.gm)); len(got) > 0 {
		t.Errorf("git ran in, or named, those folders or the GM's plans: %q", got)
	}
	if got := fifosOpened(fifos); len(got) > 0 {
		t.Errorf("FIFOs in the GM's repo were opened: %q", got)
	}
}
