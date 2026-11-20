//go:build darwin

package desktop

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXGMHIRENOGIT126 round 3: the reviewer's B1 (the engine's place check judged one spelling of the folder and ran git
// on another), B2 (a hire's tree swapped for a link moved its sandbox's place), N1 (a link at the folder for hires,
// removed through) and N2 (the T3 hook look ran git in a GM's folder). Every GM is a row in the test's own registry;
// links, renames and "victim" folders are the test's own, in its temp folders.

// r3Check is the engine's own check of a hire's place asked into folder, as runManagedHire hands it to prepareHire,
// its sandbox on (hirePlaceCheck).
func r3Check(folder string) func(hirePlace, string) error {
	return hirePlaceCheck(true, "abcdef123456", "janitor", engineOptions{Git: runGit})
}

// r3Turning turns x between plain and lib2, as a GM's loop would, until the returned func stops it and says how many
// times it turned.
func r3Turning(t *testing.T, x, plain, lib2 string) func() int {
	t.Helper()
	tmp := x + ".tmp"
	if err := os.Symlink(plain, x); err != nil {
		t.Fatal(err)
	}
	var stop atomic.Bool
	done := make(chan int)
	go func() {
		turns := 0
		for i := 0; !stop.Load(); i++ {
			to := plain
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
	return func() int {
		stop.Store(true)
		return <-done
	}
}

// B1: the engine's own check (hirePlaceCheck, as runManagedHire hands it to prepareHire, its sandbox on) and the plain
// route's look (hirePlaceWords) see the folder the rule judged, so no git ever starts in the GM's repo (lib2, in its
// plans, with FIFOs at its HEAD and config) however a link on the way turns:
//   - x in its plans → a plain folder at the rule's look, turned to lib2 right after it;
//   - x, then ~/code/x, not resolvable at the look, then turned to lib2;
//   - a loop of the test's own turning x between the plain folder and lib2 over 40 hires (the reviewer's race), and
//     over 40 of the plain route's looks.
func TestTheEnginesPlaceCheckSeesTheFolderTheRuleJudged(t *testing.T) {
	for _, mode := range []string{"x → a plain folder at the look", "x not resolvable at the look", "~/code/x not resolvable at the look"} {
		t.Run(mode, func(t *testing.T) {
			_, l, lib2, fifos := gmFolderDeck(t)
			x := filepath.Join(gmPlansOf(l.gm), "x")
			if strings.HasPrefix(mode, "~/code") {
				x = filepath.Join(homeDir, "code", "x")
			}
			plain := filepath.Join(homeDir, "code", "plain")
			if err := os.MkdirAll(plain, 0o700); err != nil {
				t.Fatal(err)
			}
			first := plain
			if strings.Contains(mode, "not resolvable") {
				first = filepath.Join(t.TempDir(), "not-there")
			}
			if err := os.Symlink(first, x); err != nil {
				t.Fatal(err)
			}
			saved := gmOwnsFolder
			var turned atomic.Bool
			gmOwnsFolder = func(f string) bool {
				v := saved(f)
				if turned.CompareAndSwap(false, true) {
					_ = os.Remove(x)
					_ = os.Symlink(lib2, x)
				}
				return v
			}
			t.Cleanup(func() { gmOwnsFolder = saved })
			calls := gitCalls(t)
			place, err, took := hireWithin(t, 20*time.Second, hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: x, Worktree: true, Check: r3Check(x)},
				hirePlaceOptions{PortFree: func(int) bool { return true }}, fifos...)
			if !turned.Load() {
				t.Fatal("x was never turned: the row proves nothing")
			}
			if got := calls(lib2); len(got) > 0 {
				t.Errorf("git ran in the GM's repo: %q", got)
			}
			if opened := fifosOpened(fifos); len(opened) > 0 {
				t.Errorf("FIFOs opened: %q", opened)
			}
			if took > 5*time.Second {
				t.Errorf("the hire took %v", took)
			}
			t.Logf("took %v; place %+v; err %v", took.Round(time.Millisecond), place, err)
		})
	}
	t.Run("a loop turning x over 40 hires, and over 40 of the plain route's looks", func(t *testing.T) {
		_, l, lib2, fifos := gmFolderDeck(t)
		for _, f := range fifos {
			f.release()
		}
		x := filepath.Join(gmPlansOf(l.gm), "x")
		plain := filepath.Join(homeDir, "code", "plain")
		if err := os.MkdirAll(plain, 0o700); err != nil {
			t.Fatal(err)
		}
		calls := gitCalls(t)
		stop := r3Turning(t, x, plain, lib2)
		tries, hits, looks := 40, 0, 0
		outcomes := map[string]int{}
		for range tries {
			before := len(calls(lib2))
			place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: x, Worktree: true, Check: r3Check(x)},
				hirePlaceOptions{PortFree: func(int) bool { return true }})
			if len(calls(lib2)) > before {
				hits++
			}
			outcomes[firstRunes(place.Note, 30)+" / "+firstRunes(fmt.Sprint(err), 40)]++
			before = len(calls(lib2))
			_ = hirePlaceWords(x, runGit)
			if len(calls(lib2)) > before {
				looks++
			}
		}
		turns := stop()
		t.Logf("git started in lib2 in %d of %d hires and %d of %d looks (x turned %d times); outcomes %v", hits, tries, looks, tries, turns, outcomes)
		if hits > 0 || looks > 0 {
			t.Errorf("git started in the GM's repo: %q", calls(lib2))
		}
		if turns < 2*tries {
			t.Errorf("x turned only %d times: the row proves little", turns)
		}
	})
}

// r3Swap turns tree into a link to target, the folder renamed aside first, as a GM writing the folder for hires can;
// the returned func puts it back.
func r3Swap(t *testing.T, tree, target string) func() {
	t.Helper()
	if err := os.Rename(tree, tree+".real"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, tree); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Remove(tree); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(tree+".real", tree); err != nil {
			t.Fatal(err)
		}
	}
}

// B2, at every start: the owner's hire into ~/code/app (the full flow), its tree in ~/code/app-hires, which a GM may
// write (held to one project, or not), is swapped for a link to each of the reviewer's targets: the owner's project
// the GM reads, the GM's own hire's tree, the GM's read-only project, ~/.ssh. Each start, as its row makes it, is
// refused in words that name the tree, before any Policy is made; so is one whose folder for hires (the tree's
// parent) is swapped for a link to itself. Control: with no swap, before and after, it starts with its tree as its
// place, and the hire writes there.
func TestAHiresTreeSwappedForALinkNeverStarts(t *testing.T) {
	for _, mode := range []string{"a GM held to one project", "a GM not held to one project"} {
		t.Run(mode, func(t *testing.T) {
			targets := map[string]string{}
			if strings.Contains(mode, "not held") {
				_, l, _, fifos := gmFolderDeck(t)
				for _, f := range fifos {
					f.release()
				}
				targets["the owner's project the GM reads"] = physical(l.lib)
			} else {
				d, _, _, token, project := scopedGM(t)
				if code, text := scopedCall(t, d, token, "POST", "/setup/gm/hire", `{"role":"janitor","folder":`+jsonText(project)+`}`); code != http.StatusOK {
					t.Fatalf("the GM's hire: %d %q", code, text)
				}
				hires := registered(t)
				targets["the GM's read-only project"] = physical(project)
				targets["the GM's own hire's tree"] = hires[len(hires)-1].Worktree.storedOnly()
			}
			ssh := filepath.Join(homeDir, ".ssh")
			if err := os.MkdirAll(ssh, 0o700); err != nil {
				t.Fatal(err)
			}
			targets["~/.ssh"] = physical(ssh)
			app := filepath.Join(homeDir, "code", "app")
			ownersApp(t, app)
			place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: physical(app), Worktree: true},
				hirePlaceOptions{PortFree: func(int) bool { return true }})
			if err != nil || place.Worktree == "" {
				t.Fatalf("the owner's hire: %+v %v", place, err)
			}
			row := Hire{ID: "abcdef123456", Name: "Ada", Role: "janitor", Folder: keptPath(place.Folder), Repo: place.Repo, Worktree: keptPath(place.Worktree), Branch: place.Branch, State: "working"}
			at := func() deskAt {
				return deskAtOf(row, deskSocketPath(row.ID, testCopyPort()), nil, engineOptions{Git: runGit})
			}
			starts := func(when string) {
				t.Helper()
				pl, err := deskPlaces(at())
				if err != nil {
					t.Fatalf("%s: its start was refused: %v", when, err)
				}
				pol, err := sandbox.New(pl)
				if err != nil {
					t.Fatalf("%s: its Policy: %v", when, err)
				}
				if v, rule := pol.Decide(filepath.Join(place.Worktree, "a.txt"), sandbox.Write); pl.Root != place.Worktree || v != sandbox.Allow {
					t.Errorf("%s: Root %s, its write in its tree %v (%s)", when, pl.Root, v, rule.Row)
				}
			}
			starts("before any swap")
			want := "This hire's worktree, " + shownPath(place.Worktree) + ", was moved or replaced since it was hired, so it didn't start. Fire it, then hire it again."
			refused := func(what string) {
				t.Helper()
				if _, err := deskPlaces(at()); err == nil || err.Error() != want {
					t.Errorf("%s: its places: %v, want %q", what, err, want)
				}
				var words *errSandboxNotMade
				if _, err := makeDeskSandbox(at()); !errors.As(err, &words) || words.words != want {
					t.Errorf("%s: its start: %v, want %q", what, err, want)
				}
			}
			for name, target := range targets {
				back := r3Swap(t, place.Worktree, target)
				refused("the tree swapped for a link to " + name)
				back()
			}
			back := r3Swap(t, filepath.Dir(place.Worktree), filepath.Dir(place.Worktree)+".real")
			refused("the folder for hires swapped for a link to itself")
			back()
			starts("after the swaps, put back")
		})
	}
}

// B2, the launcher's start: an office's plain hire on a Mac works in its own worktree, on record for its desk. Its tree
// swapped for a link to the owner's project, its launcher's sandbox is refused in the same words, whatever git finds
// there now; put back, it starts.
func TestAPlainHiresTreeSwappedForALinkNeverStarts(t *testing.T) {
	d, l, h := r7wDeck(t, true)
	if status, said := d.hireWith(hireSpec{fromSetup: true, extra: &setupHire{Folder: l.lib}}); status != http.StatusNoContent {
		t.Fatalf("Setup's plain hire into lib: %d %s", status, said)
	}
	folder, desk := r7wPlainHire(t, h)
	if record, found := plainHireRecord(desk); !found || record.Worktree.storedOnly() != folder {
		t.Fatalf("its record: %+v (%v), want its worktree %s", record, found, folder)
	}
	program := filepath.Join(l.base, "claude")
	writeFile(t, program, "#!/bin/sh\n")
	back := r3Swap(t, folder, physical(l.app))
	want := "This hire's worktree, " + shownPath(folder) + ", was moved or replaced since it was hired, so it didn't start. Fire it, then hire it again."
	if s, err := newLauncherSandboxAt(desk, "", folder, false, "L1", Claude, program); s != nil || err == nil || err.Error() != want {
		t.Errorf("its sandbox through the link: %v, want %q", err, want)
		if s != nil {
			s.close()
		}
	}
	back()
	s, err := newLauncherSandboxAt(desk, "", folder, false, "L1", Claude, program)
	if err != nil {
		t.Fatalf("its sandbox, put back: %v", err)
	}
	t.Cleanup(s.close)
	if s.desk.places.Root != folder {
		t.Errorf("its place: %s, want %s", s.desk.places.Root, folder)
	}
}

// B2, at the hire: the tree swapped for a link to a "victim" folder right after git's checkout was carried in. The hire
// is refused in words naming the tree; nothing lands in the victim; what the carry made is taken back through its
// roots (the real tree emptied, the registration and the branch gone). Controls: with no swap the row keeps the
// carry's own name for the tree; with a configured folder for worktrees reached through a link, the tree's name is
// the resolved one, and that hire starts.
func TestATreeSwappedAsItIsMadeIsTakenBack(t *testing.T) {
	_, _, _, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	app := physical(filepath.Join(homeDir, "code", "app"))
	ownersApp(t, app)
	victim := filepath.Join(physical(t.TempDir()), "victim")
	if err := os.Mkdir(victim, 0o700); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(physical(hiresFolder(app, "")), hireWorktreeName("Ada", "abcdef123456"))
	swapped := false
	git := func(dir string, args ...string) (int, string, bool) {
		status, out, ok := runGit(dir, args...)
		if ok && status == 0 && len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
			swapped = os.Rename(tree, tree+".real") == nil && os.Symlink(victim, tree) == nil
		}
		return status, out, ok
	}
	_, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: app, Worktree: true},
		hirePlaceOptions{Git: git, PortFree: func(int) bool { return true }})
	if !swapped {
		t.Fatal("the tree was never swapped: the row proves nothing")
	}
	want := "Smolcorp didn't hire: the hire's worktree, " + shownPath(tree) + ", was moved or replaced while it was being made, so what was made there is taken away again. Hire again."
	var refusal *hireRefusal
	if !errors.As(err, &refusal) || refusal.Message != want {
		t.Errorf("the hire: %v, want %q", err, want)
	}
	if entries, _ := os.ReadDir(victim); len(entries) > 0 {
		t.Errorf("something landed in the link's target: %d entries", len(entries))
	}
	if entries, err := os.ReadDir(tree + ".real"); err != nil || len(entries) > 0 {
		t.Errorf("the real tree wasn't emptied: %d entries, %v", len(entries), err)
	}
	if list := standInOwnersGit(t, app, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Errorf("a registration is left: %q", list)
	}
	if branches := standInOwnersGit(t, app, "branch", "--list", "hire/*"); strings.TrimSpace(branches) != "" {
		t.Errorf("the hire's branch is left: %q", branches)
	}
	_ = os.Remove(tree)
	_ = os.Remove(tree + ".real")

	place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: app, Worktree: true},
		hirePlaceOptions{PortFree: func(int) bool { return true }})
	if err != nil || place.Worktree != tree {
		t.Fatalf("the control: %+v, %v, want the tree %s", place, err, tree)
	}
	checkCarriedTree(t, app, place)

	other := physical(filepath.Join(homeDir, "code", "other"))
	ownersApp(t, other)
	worktrees := filepath.Join(physical(t.TempDir()), "worktrees")
	link := filepath.Join(homeDir, "code", "worktrees-link")
	if err := os.Mkdir(worktrees, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(worktrees, link); err != nil {
		t.Fatal(err)
	}
	// A configured folder reached through a link in the home folder, which a GM could change, isn't honoured since
	// FXCOREWORKTREES126 (item 1): the hire is refused in plain words, and nothing is made. The folder itself is.
	place, err = prepareHire(hireRequest{HireID: "fedcba654321", Name: "Ada", Folder: other, Worktree: true},
		hirePlaceOptions{Worktrees: link, PortFree: func(int) bool { return true }})
	if err == nil || !strings.Contains(err.Error(), "is reached through a link in your home folder") || place.Worktree != "" {
		t.Fatalf("a configured folder for worktrees through a link: %+v, %v, want it refused", place, err)
	}
	if entries, _ := os.ReadDir(worktrees); len(entries) != 0 {
		t.Errorf("made in the refused folder: %v", entries)
	}
	place, err = prepareHire(hireRequest{HireID: "fedcba654321", Name: "Ada", Folder: other, Worktree: true},
		hirePlaceOptions{Worktrees: worktrees, PortFree: func(int) bool { return true }})
	if want := filepath.Join(worktrees, "other", hireWorktreeName("Ada", "fedcba654321")); err != nil || place.Worktree != want {
		t.Fatalf("a configured folder for worktrees: %+v, %v, want the tree %s", place, err, want)
	}
	row := Hire{ID: "fedcba654321", Name: "Ada", Role: "janitor", Folder: keptPath(place.Folder), Repo: place.Repo, Worktree: keptPath(place.Worktree), Branch: place.Branch, State: "working"}
	if pl, err := deskPlaces(deskAtOf(row, deskSocketPath(row.ID, testCopyPort()), nil, engineOptions{Git: runGit, Worktrees: worktrees})); err != nil || pl.Root != place.Worktree {
		t.Errorf("its start: Root %s, %v", pl.Root, err)
	}
}

// N1: the folder for hires itself swapped for a link to a "victim" folder the moment git's checkout is in the core's
// own folder, before the carry opens it: the hire is refused in the carry's own words (never "git didn't answer"),
// nothing lands in the victim, and nothing there is removed (a folder of the hire's tree name planted in it stays).
func TestAFolderForHiresSwappedForALinkLosesNothingThere(t *testing.T) {
	_, _, _, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	tries, swaps := 12, 0
	for i := range tries {
		app := physical(filepath.Join(homeDir, "code", fmt.Sprintf("p%02d", i)))
		ownersApp(t, app)
		victim := filepath.Join(physical(t.TempDir()), "victim")
		name := hireWorktreeName("Ada", "abcdef123456")
		if err := os.MkdirAll(filepath.Join(victim, name), 0o700); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(victim, name, "keep.txt"), "keep\n")
		parent := physical(hiresFolder(app, ""))
		reads := gitReadsFolder()
		var stop atomic.Bool
		done := make(chan bool, 1)
		go func() {
			for !stop.Load() {
				entries, _ := os.ReadDir(reads)
				for _, e := range entries {
					if sub, _ := os.ReadDir(filepath.Join(reads, e.Name())); strings.HasPrefix(e.Name(), "tree-") && len(sub) > 0 {
						if os.Rename(parent, parent+".real") == nil && os.Symlink(victim, parent) == nil {
							done <- true
							return
						}
					}
				}
			}
			done <- false
		}()
		place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: app, Worktree: true},
			hirePlaceOptions{PortFree: func(int) bool { return true }})
		stop.Store(true)
		swapped := <-done
		if entries, _ := os.ReadDir(victim); len(entries) != 1 {
			t.Errorf("try %d: the link's target has %d entries (%+v, %v)", i, len(entries), place, err)
		}
		if data := readOr(filepath.Join(victim, name, "keep.txt")); data != "keep\n" {
			t.Errorf("try %d: the victim's %s/keep.txt was removed", i, name)
		}
		if !swapped {
			if err == nil && place.Worktree != "" {
				removeWorkspace(place.Repo, place.Worktree, place.Branch, runGit)
			}
			continue
		}
		swaps++
		var refusal *hireRefusal
		if !errors.As(err, &refusal) || strings.Contains(refusal.Message, "git didn't answer") ||
			!strings.HasPrefix(refusal.Message, "Couldn't make the hire's worktree: Smolcorp didn't put it at "+shownPath(filepath.Join(parent, name))+": ") {
			t.Errorf("try %d: the hire: %v, want the carry's own refusal", i, err)
		}
	}
	t.Logf("the folder for hires was swapped in %d of %d tries", swaps, tries)
	if swaps == 0 {
		t.Error("the swap never happened: the row proves nothing")
	}
}

// N2: T3's look for a main checkout's hooks switched off runs no git and reads nothing in a GM's own repo (lib2, its
// HEAD and config FIFOs), asked by its name or through a link in the GM's plans; the owner's repo with hooks switched
// off is still found so.
func TestTheT3HookLookRunsNoGitInAGMsFolder(t *testing.T) {
	_, l, lib2, fifos := gmFolderDeck(t)
	writeFile(t, filepath.Join(lib2, filepath.FromSlash(t3FenceName)), `{"disableAllHooks": true}`)
	x := filepath.Join(gmPlansOf(l.gm), "x")
	if err := os.Symlink(lib2, x); err != nil {
		t.Fatal(err)
	}
	calls := gitCalls(t)
	for _, folder := range []string{lib2, x} {
		if t3HooksSwitchedOff(folder, runGit) {
			t.Errorf("%s: found switched off", folder)
		}
	}
	if got := calls(lib2); len(got) > 0 {
		t.Errorf("git ran in the GM's repo: %q", got)
	}
	if opened := fifosOpened(fifos); len(opened) > 0 {
		t.Errorf("FIFOs opened: %q", opened)
	}
	app := filepath.Join(homeDir, "code", "app")
	ownersApp(t, app)
	writeFile(t, filepath.Join(app, filepath.FromSlash(t3FenceName)), `{"disableAllHooks": true}`)
	if !t3HooksSwitchedOff(app, runGit) {
		t.Error("the owner's repo with hooks switched off isn't found so")
	}
}

// O1's owner row, after a restart: with a GM not held to one project open, the owner's hires into ~/code/app (by
// Setup and by the GM's route) keep the full flow, and each starts again from its row alone, as after Smolcorp's
// restart: its tree as its row keeps it is its place (an ordinary hire is never refused by B2's check).
func TestTheOwnersHireIntoACarriedTreeStartsAgainFromItsRow(t *testing.T) {
	d, _, _, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	app := filepath.Join(homeDir, "code", "app")
	ownersApp(t, app)
	spend(d)
	if status, answer, why := d.hireFromSetupSheet(setupHire{Role: "janitor", Folder: app, Answers: []string{}, Worktree: true}); status != http.StatusOK {
		t.Fatalf("Setup's hire into ~/code/app: %d %v %q", status, answer, why)
	}
	d.watching.Wait()
	spend(d)
	if a := <-gmRouteHire(d, app); a.code != http.StatusOK {
		t.Fatalf("the GM's hire into ~/code/app: %d %s", a.code, a.body)
	}
	started := 0
	for _, row := range registered(t) {
		if row.Repo != physical(app) {
			continue
		}
		if row.Worktree.none() || row.Branch == "" || filepath.Dir(row.Worktree.storedOnly()) != physical(hiresFolder(app, "")) {
			t.Errorf("%s: its row %+v, want its own tree in %s", row.ID, row, hiresFolder(app, ""))
			continue
		}
		if err := os.MkdirAll(hireFolder(supportDir, row.ID), 0o700); err != nil {
			t.Fatal(err)
		}
		s, err := makeDeskSandbox(deskAtOf(row, deskSocketPath(row.ID, testCopyPort()), nil, engineOptions{Git: runGit}))
		if err != nil {
			t.Errorf("%s: its start from its row: %v", row.ID, err)
			continue
		}
		if s.places.Root != row.Worktree.storedOnly() || s.places.Checkout != physical(app) {
			t.Errorf("%s: its place %s in %s, want its tree %s", row.ID, s.places.Root, s.places.Checkout, row.Worktree)
		}
		started++
	}
	if started != 2 {
		t.Errorf("%d of the two hires started again from their rows", started)
	}
}
