//go:build darwin

package desktop

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// FXGMHIRENOGIT126 round 5: R13 (the engine's first start ran no tree check), R14 (the engine's and T3's undos after
// prepareHire went by the tree's path), D21's limit (a row from before turned toward a place its own Policy holds or
// denies started there), and the other instances of the class found: the launcher's git before its check, the trust
// and the plans written by the folder's path after its check, a restart's plans before its check. Links, renames and
// "victim" folders are the test's own, in its temp folders.

// r5Victim is a repo of the owner's that a turned tree leads to: a marker, a clean status.
func r5Victim(t *testing.T) string {
	t.Helper()
	victim := filepath.Join(physical(t.TempDir()), "victim")
	ownersApp(t, victim)
	return victim
}

// r5Untouched checks victim is as r5Victim made it: no git ran there, its status is clean, its marker is there, and
// nothing of a hire's (its plans, a fence) was put in it.
func r5Untouched(t *testing.T, what, victim string, calls func(...string) []string) {
	t.Helper()
	if got := calls(victim); len(got) > 0 {
		t.Errorf("%s: git ran in the victim: %q", what, got)
	}
	if status := standInOwnersGit(t, victim, "status", "--porcelain", "--ignored"); strings.TrimSpace(status) != "" {
		t.Errorf("%s: the victim changed: %q", what, status)
	}
	if readOr(filepath.Join(victim, "marker.txt")) != "harmless\n" {
		t.Errorf("%s: the victim's marker changed", what)
	}
	for _, made := range []string{".smolcorp", ".claude"} {
		if _, err := os.Lstat(filepath.Join(victim, made)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: %s was made in the victim: %v", what, made, err)
		}
	}
}

// r5Gone checks nothing of the hire's tree is left in repo: no registration but the checkout's, no hire branch.
func r5Gone(t *testing.T, what, repo string) {
	t.Helper()
	if list := standInOwnersGit(t, repo, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Errorf("%s: a registration is left: %q", what, list)
	}
	if branches := standInOwnersGit(t, repo, "branch", "--list", "hire/*"); strings.TrimSpace(branches) != "" {
		t.Errorf("%s: the hire's branch is left: %q", what, branches)
	}
}

// r5Turn turns tree into a link to victim, its real folder kept beside it, and says where that is.
func r5Turn(t *testing.T, tree, victim string) string {
	t.Helper()
	if err := os.Rename(tree, tree+".real"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, tree); err != nil {
		t.Fatal(err)
	}
	return tree + ".real"
}

// R13 and R14, the engine: the tree turned into a link to a victim repo between prepareHire's check and the first
// start (when its start message is typed, before its plans, its trust and its sandbox). The first start is refused in
// the tree's words, before any Policy is made; the undo goes through the carry's roots: no git in the victim, nothing
// put there (its plans), the real tree emptied, no registration or branch left, the link left as it is. Control: the
// same hire with no turn starts, its place the tree.
func TestATreeTurnedBeforeTheFirstStartIsRefusedThereAndTakenBackThroughItsRoots(t *testing.T) {
	r := newSandboxRig(t)
	victim := r5Victim(t)
	calls := gitCalls(t)
	request := r.request("janitor")
	var tree, real string
	request.StartTyped = func(string) {
		tree = registered(t)[0].Worktree.storedOnly()
		real = r5Turn(t, tree, victim)
	}
	_, err := runManagedHire(request, r.o)
	want := "This hire's worktree, " + shownPath(tree) + ", was moved or replaced since it was hired, so it didn't start. Fire it, then hire it again."
	var refused *hireRefusal
	if !errors.As(err, &refused) || refused.Status != http.StatusConflict || refused.Message != want {
		t.Errorf("the first start: %v, want %q", err, want)
	}
	if len(r.desks) != 0 || len(r.starts) != 0 {
		t.Errorf("a sandbox was made (%d) or started (%d): Root %q", len(r.desks), len(r.starts), r.firstRoot())
	}
	r5Untouched(t, "the first start", victim, calls)
	if info, err := os.Lstat(tree); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link was taken away or followed: %v", err)
	}
	if entries, err := os.ReadDir(real); err != nil || len(entries) > 0 {
		t.Errorf("the real tree wasn't emptied: %d entries, %v", len(entries), err)
	}
	r5Gone(t, "the first start", r.repo)
	if hires := registered(t); len(hires) != 0 {
		t.Errorf("a row is left: %+v", hires)
	}

	// Control: no turn.
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil || len(r.desks) != 1 || r.desks[0].places.Root != hire.Worktree.storedOnly() {
		t.Fatalf("with no turn: %v, %d sandboxes, Root %q", err, len(r.desks), r.firstRoot())
	}
	if _, err := os.Stat(filepath.Join(hire.Worktree.storedOnly(), filepath.FromSlash(deskPlansFor(hire.ID)))); err != nil {
		t.Errorf("with no turn, its plans: %v", err)
	}
}

// firstRoot is the first sandbox's place, "" for none.
func (r *sandboxRig) firstRoot() string {
	if len(r.desks) == 0 {
		return ""
	}
	return r.desks[0].places.Root
}

// R14, the engine, after the tree's check at the first start: the tree turned into a link to a victim repo as its
// start is made, and the start then refused. Its agent may have run, so a changed tree would be kept, as before; a
// turned one is kept too, with no git on its path: nothing in the victim changes and no git runs there.
func TestATreeTurnedAsItsStartIsRefusedGetsNoGitOnItsPath(t *testing.T) {
	r := newSandboxRig(t)
	victim := r5Victim(t)
	calls := gitCalls(t)
	r.refusal = errors.New("The sandbox didn't hold, so the hire wasn't started.")
	start := r.o.Sandbox
	r.o.Sandbox = func(s *deskSandbox) tmuxStarter {
		run := start(s)
		return func(args []string) (string, int, error) {
			r5Turn(t, registered(t)[0].Worktree.storedOnly(), victim)
			return run(args)
		}
	}
	if _, err := runManagedHire(r.request("janitor"), r.o); err == nil || !strings.Contains(err.Error(), "The sandbox didn't hold") {
		t.Errorf("the start: %v", err)
	}
	if len(r.starts) != 1 {
		t.Fatalf("%d starts", len(r.starts))
	}
	r5Untouched(t, "a start refused after the turn", victim, calls)
}

// R14, T3: the tree turned into a link to a victim repo right after prepareHire (as its row is written). The hire is
// refused before its fence, git's exclude or T3 Code act on the tree's path, and taken back through the carry's
// roots: nothing in the victim, no git there, no T3 call at all, the real tree emptied, no registration or branch.
func TestAT3HiresTreeTurnedAfterItsCheckIsTakenBackThroughItsRoots(t *testing.T) {
	r := newT3Rig(t)
	r.fake()
	repo := r.repo()
	victim := r5Victim(t)
	calls := gitCalls(t)
	ways := r.ways()
	real := ""
	register := ways.Register
	ways.Register = func(place hirePlace) error {
		real = r5Turn(t, place.Worktree, victim)
		return register(place)
	}
	_, err := t3HireInto(r.hiring(repo), ways)
	want := "Smolcorp didn't hire: the hire's worktree, " + shownPath(r.worktree()) + ", was moved or replaced while it was being made, so what was made there is taken away again. Hire again."
	if err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("the hire: %v, want %q", err, want)
	}
	r5Untouched(t, "T3's hire", victim, calls)
	if runs := r.runs(); len(runs) != 0 {
		t.Errorf("T3 Code was asked: %q", runs)
	}
	if entries, err := os.ReadDir(real); err != nil || len(entries) > 0 {
		t.Errorf("the real tree wasn't emptied: %d entries, %v", len(entries), err)
	}
	r5Gone(t, "T3's hire", repo)
}

// D21's limit: a row from before (no resolved folder kept), its folder turned into a link toward a place its own
// Policy holds or denies, is refused at its first start, and nothing is kept: ~/.ssh, an agent's folder (~/.codex),
// a folder the owner's programs read (~/Library/LaunchAgents), a folder in the support folder. Controls: turned toward
// an ordinary folder no row holds, it starts there and keeps it, as D21 ruled; spelled through a link above it, it
// starts and keeps its folder resolved.
func TestARowFromBeforeTurnedTowardAHeldPlaceNeverStarts(t *testing.T) {
	_, _, _, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	notes := physical(filepath.Join(homeDir, "code", "notes"))
	held := map[string]string{
		"~/.ssh":                 filepath.Join(homeDir, ".ssh"),
		"~/.codex":               filepath.Join(homeDir, ".codex"),
		"~/Library/LaunchAgents": filepath.Join(homeDir, "Library", "LaunchAgents"),
		"the support folder's":   filepath.Join(supportDir, "hires", "elsewhere"),
	}
	for _, folder := range append([]string{notes}, mapValues(held)...) {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	add := func(id, folder string) Hire {
		t.Helper()
		row := Hire{ID: id, Name: "Ada", Role: "janitor", Folder: keptPath(folder), State: "working"}
		if err := os.MkdirAll(hireFolder(supportDir, id), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := updateRegistry(func(f *registryFile) {
			f.hires = slices.DeleteFunc(f.hires, func(h Hire) bool { return h.ID == id })
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
		return ""
	}
	want := "This hire's folder, " + shownPath(notes) + ", was moved or replaced since it was hired, so it didn't start. Fire it, then hire it again."
	for name, target := range held {
		row := add("a00000000005", notes)
		back := r3Swap(t, notes, physical(target))
		r4Refused(t, "turned toward "+name, row, want)
		if got := kept(row.ID); got != "" {
			t.Errorf("turned toward %s: kept %q", name, got)
		}
		if at, checked, words := deskStartFolder(row, deskAtOf(row, deskSocketPath(row.ID, testCopyPort()), nil, engineOptions{Git: runGit})); words != want || checked || at != "" {
			t.Errorf("turned toward %s, a restart's first look: %q %v %q", name, at, checked, words)
		}
		back()
	}
	// Controls.
	plain := filepath.Join(physical(t.TempDir()), "plain")
	if err := os.Mkdir(plain, 0o700); err != nil {
		t.Fatal(err)
	}
	row := add("a00000000006", notes)
	back := r3Swap(t, notes, plain)
	if pl, err, made := r4Start(row); err != nil || made != nil || pl.Root != plain || kept(row.ID) != plain {
		t.Errorf("turned toward a folder no row holds: Root %s, %v, %v; kept %q", pl.Root, err, made, kept(row.ID))
	}
	back()
	above := filepath.Join(homeDir, "code-link")
	if err := os.Symlink(filepath.Dir(notes), above); err != nil {
		t.Fatal(err)
	}
	row = add("a00000000007", filepath.Join(above, "notes"))
	if pl, err, made := r4Start(row); err != nil || made != nil || pl.Root != notes || kept(row.ID) != notes {
		t.Errorf("spelled through a link above it: Root %s, %v, %v; kept %q", pl.Root, err, made, kept(row.ID))
	}
}

// mapValues are m's values, in no order.
func mapValues(m map[string]string) []string {
	var values []string
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// The launcher's start runs no git at its folder before that folder's check: a plain hire's tree (a record), and a
// plain hire's folder in no repo (--place), each turned into a link to one of the owner's projects, are refused with no
// git run there.
func TestTheLaunchersStartRunsNoGitAtATurnedFolder(t *testing.T) {
	d, l, h := r7wDeck(t, true)
	calls := gitCalls(t)
	program := filepath.Join(l.base, "claude")
	writeFile(t, program, "#!/bin/sh\n")
	if status, said := d.hireWith(hireSpec{fromSetup: true, extra: &setupHire{Folder: l.lib}}); status != http.StatusNoContent {
		t.Fatalf("Setup's plain hire into lib: %d %s", status, said)
	}
	tree, desk := r7wPlainHire(t, h)
	back := r3Swap(t, tree, physical(l.app))
	if s, err := newLauncherSandboxAt(desk, "", tree, false, "L1", Claude, program); s != nil || err == nil || !strings.Contains(err.Error(), "was moved or replaced") {
		t.Errorf("its tree turned: %v", err)
		if s != nil {
			s.close()
		}
	}
	back()
	if got := calls(l.app); len(got) > 0 {
		t.Errorf("its tree turned: git ran in the owner's app: %q", got)
	}

	h.tmux = nil
	notes := filepath.Join(l.projects, "notes")
	if err := os.Mkdir(notes, 0o700); err != nil {
		t.Fatal(err)
	}
	spend(d)
	if status, said := d.Perform("hire", "", url.Values{"project": {"notes"}}); status != http.StatusNoContent {
		t.Fatalf("the plain hire into notes: %d %s", status, said)
	}
	folder, desk := r7wPlainHire(t, h)
	place := physical(notes)
	back = r3Swap(t, notes, physical(l.app))
	if s, err := newLauncherSandboxAt(desk, place, folder, false, "L1", Claude, program); s != nil || err == nil || !strings.Contains(err.Error(), "was moved or replaced") {
		t.Errorf("its folder turned: %v", err)
		if s != nil {
			s.close()
		}
	}
	back()
	if got := calls(l.app); len(got) > 0 {
		t.Errorf("its folder turned: git ran in the owner's app: %q", got)
	}
}

// The trust prepareHire writes in the account's file is the tree as checked: the tree turned into a link to a victim
// repo right before (the hire's check), the victim's path is never trusted; the tree's own is.
func TestAHiresTrustIsTheTreeItsCheckSaw(t *testing.T) {
	r := newEngineRig(t)
	victim := r5Victim(t)
	tree := ""
	check := func(p hirePlace, _ string) error {
		tree = p.Worktree
		r5Turn(t, tree, victim)
		return nil
	}
	place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: r.repo, Claude: true, Worktree: true, Check: check},
		hirePlaceOptions{Worktrees: r.o.Worktrees, PortFree: func(int) bool { return true }})
	if err != nil || place.Worktree != tree {
		t.Fatalf("the hire: %+v %v", place, err)
	}
	data := readOr(claudeJSON(""))
	if strings.Contains(data, victim) {
		t.Errorf("the victim was trusted: %s", data)
	}
	if !strings.Contains(data, `"`+tree+`"`) {
		t.Errorf("the tree as checked wasn't trusted: %s", data)
	}
}

// A restart's first look (deskStartFolder): a desk whose tree is turned gets its tree's words, before its plans or its
// trust are made; put back, it gets its tree, checked, and its plans go there through a root on it.
func TestARestartChecksItsPlaceBeforeItMakesAnythingThere(t *testing.T) {
	r := newSandboxRig(t)
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	row := registered(t)[0]
	victim := r5Victim(t)
	at := func() deskAt { return deskAtOf(row, "", nil, engineOptions{Git: runGit, Worktrees: r.o.Worktrees}) }
	_ = os.RemoveAll(filepath.Join(hire.Worktree.storedOnly(), ".smolcorp"))
	back := r3Swap(t, hire.Worktree.storedOnly(), victim)
	want := "This hire's worktree, " + shownPath(hire.Worktree.storedOnly()) + ", was moved or replaced since it was hired, so it didn't start. Fire it, then hire it again."
	if folder, checked, words := deskStartFolder(row, at()); words != want || checked || folder != "" {
		t.Errorf("turned: %q %v %q", folder, checked, words)
	}
	back()
	folder, checked, words := deskStartFolder(row, at())
	if words != "" || !checked || folder != hire.Worktree.storedOnly() {
		t.Fatalf("put back: %q %v %q", folder, checked, words)
	}
	if err := makeStartPlans(folder, checked, row.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(hire.Worktree.storedOnly(), filepath.FromSlash(deskPlansFor(row.ID)))); err != nil {
		t.Errorf("its plans: %v", err)
	}
	r5Untouched(t, "a restart's first look", victim, func(...string) []string { return nil })
}
