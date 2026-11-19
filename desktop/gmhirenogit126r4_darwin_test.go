//go:build darwin

package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXGMHIRENOGIT126 round 4: R10 (a hire with no worktree had no start check of its folder, the class of B2) and R12
// (the undo after a later step's refusal removed the tree by its path). Every GM is a row in the test's own registry;
// links, renames and "victim" folders are the test's own, in its temp folders.

// r4Targets are where a GM would point a hire's folder: the owner's project the GM reads, the GM's own repo in its
// plans, another desk's worktree, ~/.ssh.
func r4Targets(t *testing.T, l gmRootLayout, lib2 string) map[string]string {
	t.Helper()
	ssh := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(ssh, 0o700); err != nil {
		t.Fatal(err)
	}
	return map[string]string{"the owner's project the GM reads": physical(l.lib), "the GM's own repo in its plans": physical(lib2),
		"another desk's worktree": physical(l.hireTree), "~/.ssh": physical(ssh)}
}

// r4Start is desk row's start as its row makes it (deskAtOf): its places, or why not, and its sandbox's words.
func r4Start(row Hire) (sandbox.Places, error, error) {
	at := func() deskAt {
		return deskAtOf(row, deskSocketPath(row.ID, testCopyPort()), nil, engineOptions{Git: runGit})
	}
	pl, err := deskPlaces(at())
	_, made := makeDeskSandbox(at())
	return pl, err, made
}

// r4Refused checks row's start is refused in want's words, by its places and by its sandbox alike.
func r4Refused(t *testing.T, what string, row Hire, want string) {
	t.Helper()
	_, err, made := r4Start(row)
	if err == nil || err.Error() != want {
		t.Errorf("%s: its places: %v, want %q", what, err, want)
	}
	var words *errSandboxNotMade
	if !errors.As(made, &words) || words.words != want {
		t.Errorf("%s: its start: %v, want %q", what, made, want)
	}
}

// r4Row is the newest row in the registry, the hire just made.
func r4Row(t *testing.T) Hire {
	t.Helper()
	hires := registered(t)
	return hires[len(hires)-1]
}

// R10, at every start: a hire with no worktree, its row made by the engine's own hire, keeps its folder resolved;
// that folder, or a folder on its way, turned into a link to each of the reviewer's targets refuses every start, in
// words that name the folder, before any Policy is made:
//   - Setup's hire into ~/code/notes (a folder in no repo), with a GM not held to one project open;
//   - the GM's own hire, by its route, into x in its plans (placed under the rule: no git there).
//
// Control: unchanged, before and after, each starts with its folder as its place.
func TestAHireWithNoWorktreeWhoseFolderTurnedNeverStarts(t *testing.T) {
	for _, mode := range []string{"Setup's hire into ~/code/notes", "the GM's hire into x in its plans"} {
		t.Run(mode, func(t *testing.T) {
			d, l, lib2, fifos := gmFolderDeck(t)
			for _, f := range fifos {
				f.release()
			}
			targets := r4Targets(t, l, lib2)
			var folder string
			if strings.HasPrefix(mode, "Setup") {
				folder = filepath.Join(homeDir, "code", "notes")
				if err := os.MkdirAll(folder, 0o700); err != nil {
					t.Fatal(err)
				}
				spend(d)
				if status, answer, why := d.hireFromSetupSheet(setupHire{Role: "janitor", Folder: folder, Answers: []string{}, Worktree: true}); status != http.StatusOK {
					t.Fatalf("Setup's hire: %d %v %q", status, answer, why)
				}
			} else {
				folder = filepath.Join(gmPlansOf(l.gm), "x")
				if err := os.MkdirAll(folder, 0o700); err != nil {
					t.Fatal(err)
				}
				a := <-gmRouteHire(d, folder)
				var said map[string]any
				_ = json.Unmarshal([]byte(a.body), &said)
				if a.code != http.StatusOK || said["note"] != gmFolderLine {
					t.Fatalf("the GM's hire: %d %s", a.code, a.body)
				}
			}
			d.watching.Wait()
			row := r4Row(t)
			if !row.Worktree.none() || row.Folder.storedOnly() != folder || row.FolderResolved != physical(folder) {
				t.Fatalf("its row: folder %q, worktree %q, resolved %q; want %q kept resolved", row.Folder, row.Worktree, row.FolderResolved, physical(folder))
			}
			starts := func(when string) {
				t.Helper()
				pl, err, made := r4Start(row)
				if err != nil || made != nil || pl.Root != physical(folder) {
					t.Fatalf("%s: Root %s, places %v, start %v", when, pl.Root, err, made)
				}
			}
			starts("before any turn")
			want := "This hire's folder, " + shownPath(folder) + ", was moved or replaced since it was hired, so it didn't start. Fire it, then hire it again."
			for name, target := range targets {
				back := r3Swap(t, folder, target)
				r4Refused(t, "the folder turned into a link to "+name, row, want)
				back()
			}
			// A folder on its way: its parent turned into a link to another folder that holds one of that name.
			elsewhere := filepath.Join(physical(t.TempDir()), "elsewhere")
			if err := os.MkdirAll(filepath.Join(elsewhere, filepath.Base(folder)), 0o700); err != nil {
				t.Fatal(err)
			}
			back := r3Swap(t, filepath.Dir(folder), elsewhere)
			r4Refused(t, "its parent turned into a link", row, want)
			back()
			starts("after the turns, put back")
		})
	}
}

// R10, a row from before its folder was kept resolved: its first start resolves the folder and keeps that on its
// row, refused only where the resolution lands in a GM's place, another desk's place or a project a GM reads only,
// as the ruling chose. Rows: already resolved, and spelled through a link above it (each starts, and keeps the
// resolved folder, which the next start then holds it to); turned into a link to the owner's project the GM reads,
// to the GM's own repo, to another desk's worktree (each refused, nothing kept); a GM-placed x in its plans turned to
// the owner's project (refused).
func TestARowFromBeforeKeepsItsFolderAtItsFirstStart(t *testing.T) {
	_, l, lib2, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	notes := physical(filepath.Join(homeDir, "code", "notes"))
	if err := os.MkdirAll(notes, 0o700); err != nil {
		t.Fatal(err)
	}
	add := func(row Hire) Hire {
		t.Helper()
		if err := os.MkdirAll(hireFolder(supportDir, row.ID), 0o700); err != nil { // its own folder, as its hire made it
			t.Fatal(err)
		}
		if err := updateRegistry(func(f *registryFile) {
			f.hires = slices.DeleteFunc(f.hires, func(h Hire) bool { return h.ID == row.ID })
			f.hires = append(f.hires, row)
		}); err != nil {
			t.Fatal(err)
		}
		return row
	}
	kept := func(id string) string {
		t.Helper()
		for _, h := range registered(t) {
			if h.ID == id {
				return h.FolderResolved
			}
		}
		t.Fatalf("no row %s", id)
		return ""
	}
	want := func(folder string) string {
		return "This hire's folder, " + shownPath(folder) + ", was moved or replaced since it was hired, so it didn't start. Fire it, then hire it again."
	}

	row := add(Hire{ID: "a00000000001", Name: "Ada", Role: "janitor", Folder: keptPath(notes), State: "working"})
	if pl, err, made := r4Start(row); err != nil || made != nil || pl.Root != notes || kept(row.ID) != notes {
		t.Errorf("already resolved: Root %s, %v, %v; kept %q", pl.Root, err, made, kept(row.ID))
	}
	back := r3Swap(t, notes, physical(l.lib))
	row.FolderResolved = kept(row.ID)
	r4Refused(t, "already resolved, kept, then turned", row, want(notes))
	back()

	above := filepath.Join(homeDir, "code-link")
	if err := os.Symlink(filepath.Dir(notes), above); err != nil {
		t.Fatal(err)
	}
	row = add(Hire{ID: "a00000000002", Name: "Ada", Role: "janitor", Folder: keptPath(filepath.Join(above, "notes")), State: "working"})
	if pl, err, made := r4Start(row); err != nil || made != nil || pl.Root != notes || kept(row.ID) != notes {
		t.Errorf("spelled through a link above it: Root %s, %v, %v; kept %q", pl.Root, err, made, kept(row.ID))
	}

	for name, target := range map[string]string{"the owner's project the GM reads": physical(l.lib), "the GM's own repo": physical(lib2),
		"another desk's worktree": physical(l.hireTree)} {
		row := add(Hire{ID: "a00000000003", Name: "Ada", Role: "janitor", Folder: keptPath(notes), State: "working"})
		back := r3Swap(t, notes, target)
		r4Refused(t, "turned into a link to "+name, row, want(notes))
		if got := kept(row.ID); got != "" {
			t.Errorf("turned into a link to %s: kept %q", name, got)
		}
		back()
	}

	x := filepath.Join(gmPlansOf(l.gm), "x")
	if err := os.MkdirAll(x, 0o700); err != nil {
		t.Fatal(err)
	}
	row = add(Hire{ID: "a00000000004", Name: "Ada", Role: "janitor", Folder: keptPath(x), Notes: gmFolderLine, State: "working"})
	back = r3Swap(t, x, physical(l.lib))
	r4Refused(t, "x in the GM's plans turned into a link to the owner's project", row, want(x))
	back()
	if pl, err, made := r4Start(row); err != nil || made != nil || pl.Root != physical(x) {
		t.Errorf("x put back: Root %s, %v, %v", pl.Root, err, made)
	}
}

// R10, the launcher's start: the office's plain hire into a project in no repo (no worktree, no record) tells its
// launcher its folder as the rule judged it (--place); that folder turned into a link to the owner's project, or no
// --place said, its launcher's sandbox is refused in words naming the folder. Put back, it starts there.
func TestAPlainHiresFolderTurnedNeverStarts(t *testing.T) {
	d, l, h := r7wDeck(t, true)
	notes := filepath.Join(l.projects, "notes")
	if err := os.Mkdir(notes, 0o700); err != nil {
		t.Fatal(err)
	}
	if status, said := d.Perform("hire", "", url.Values{"project": {"notes"}}); status != http.StatusNoContent {
		t.Fatalf("the plain hire into notes: %d %s", status, said)
	}
	folder, desk := r7wPlainHire(t, h)
	place := ""
	if i := slices.Index(h.tmux[0], "--place"); i >= 0 && i+1 < len(h.tmux[0]) {
		place = h.tmux[0][i+1]
	}
	if _, found := plainHireRecord(desk); found || place != physical(notes) || physical(folder) != physical(notes) {
		t.Fatalf("its launcher: in %q for desk %q, --place %q (a record: %v), want %q", folder, desk, place, found, physical(notes))
	}
	program := filepath.Join(l.base, "claude")
	writeFile(t, program, "#!/bin/sh\n")
	want := "This hire's folder, " + shownPath(folder) + ", was moved or replaced since it was hired, so it didn't start. Fire it, then hire it again."
	back := r3Swap(t, notes, physical(l.lib))
	if s, err := newLauncherSandboxAt(desk, place, folder, false, "L1", Claude, program); s != nil || err == nil || err.Error() != want {
		t.Errorf("its sandbox through the link: %v, want %q", err, want)
		if s != nil {
			s.close()
		}
	}
	back()
	if s, err := newLauncherSandboxAt(desk, "", folder, false, "L1", Claude, program); s != nil || err == nil || err.Error() != want {
		t.Errorf("its sandbox with no --place: %v, want %q", err, want)
		if s != nil {
			s.close()
		}
	}
	s, err := newLauncherSandboxAt(desk, place, folder, false, "L1", Claude, program)
	if err != nil {
		t.Fatalf("its sandbox, put back: %v", err)
	}
	t.Cleanup(s.close)
	if s.desk.places.Root != physical(notes) {
		t.Errorf("its place: %s, want %s", s.desk.places.Root, physical(notes))
	}
}

// The launcher takes --place only after a plain hire's desk, as a whole clean path.
func TestTheLauncherTakesAPlainDesksPlace(t *testing.T) {
	const token = "0123456789ABCDEF0123456789ABCDEF"
	run, err := parseRun([]string{"--hire", token, "--desk", "plain-0a1b2c3d4e5f", "--project", "notes", "--place", "/x/notes", "--agent", "claude", "--role", "janitor"})
	if err != nil || run.desk != "plain-0a1b2c3d4e5f" || run.project != "notes" || run.place != "/x/notes" || run.role != "janitor" {
		t.Errorf("a plain desk's place: %+v %v", run, err)
	}
	for _, args := range [][]string{
		{"--hire", token, "--desk", "plain-0a1b2c3d4e5f", "--place", "x/notes", "claude"},
		{"--hire", token, "--desk", "plain-0a1b2c3d4e5f", "--place", "/x/../notes", "claude"},
	} {
		if run, err := parseRun(args); err == nil {
			t.Errorf("%q: %+v", args, run)
		}
	}
}

// R12: a step after the tree's check refuses the hire (the engine's check), the tree having been turned into a link
// to a "victim" repo in between: what was made is taken back through the carry's roots alone. No git runs in the
// victim, nothing there changes, the real tree is emptied, and no registration or branch is left. Control: refused
// with no turn, nothing of the tree is left at all.
func TestATreeTurnedBeforeALaterRefusalIsTakenBackThroughItsRoots(t *testing.T) {
	_, _, _, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	for _, turn := range []bool{true, false} {
		app := physical(filepath.Join(homeDir, "code", "app"))
		if !turn {
			app = physical(filepath.Join(homeDir, "code", "app2"))
		}
		ownersApp(t, app)
		victim := filepath.Join(physical(t.TempDir()), "victim")
		ownersApp(t, victim)
		tree := filepath.Join(physical(hiresFolder(app, "")), hireWorktreeName("Ada", "abcdef123456"))
		calls := gitCalls(t)
		refusal := errors.New("refused by a later step")
		check := func(p hirePlace, _ string) error {
			if p.Worktree != tree {
				t.Errorf("the check was given %q, want %q", p.Worktree, tree)
			}
			if turn {
				if err := os.Rename(tree, tree+".real"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(victim, tree); err != nil {
					t.Fatal(err)
				}
			}
			return refusal
		}
		_, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: app, Worktree: true, Check: check},
			hirePlaceOptions{PortFree: func(int) bool { return true }})
		if !errors.Is(err, refusal) {
			t.Errorf("turned %v: the hire: %v", turn, err)
		}
		if got := calls(victim); len(got) > 0 {
			t.Errorf("turned %v: git ran in the victim: %q", turn, got)
		}
		if status := standInOwnersGit(t, victim, "status", "--porcelain"); strings.TrimSpace(status) != "" || readOr(filepath.Join(victim, "marker.txt")) != "harmless\n" {
			t.Errorf("turned %v: the victim changed: %q", turn, status)
		}
		if turn {
			if info, err := os.Lstat(tree); err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("the link was taken away or followed: %v", err)
			}
			if entries, err := os.ReadDir(tree + ".real"); err != nil || len(entries) > 0 {
				t.Errorf("the real tree wasn't emptied: %d entries, %v", len(entries), err)
			}
		} else if _, err := os.Lstat(tree); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the control's tree is left: %v", err)
		}
		if list := standInOwnersGit(t, app, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
			t.Errorf("turned %v: a registration is left: %q", turn, list)
		}
		if branches := standInOwnersGit(t, app, "branch", "--list", "hire/*"); strings.TrimSpace(branches) != "" {
			t.Errorf("turned %v: the hire's branch is left: %q", turn, branches)
		}
	}
}
