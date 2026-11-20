package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXRC3FIX (RT126RC3's F1): a hire's place is checked by its identity too, not its path alone. A real folder of the
// owner's renamed into a hire's place (no link, which a GM may do for a folder outside the projects folder) fails the
// place's check: nothing in it is listed or removed, and the hire doesn't start there. Stand-ins only, in the test's
// own home folder.

// renameInto renames place aside and owner into its place, as a GM could, and gives what puts both back.
func renameInto(t *testing.T, place, owner string) func() {
	t.Helper()
	if err := os.Rename(place, place+"-aside"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(owner, place); err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		if err := os.Rename(place, owner); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(place+"-aside", place); err != nil {
			t.Fatal(err)
		}
	}
}

// The review's TestZZRC3RemoveThroughAnOwnersFolderMovedIn: checkedFolder refuses; the listing holds the place whole
// and lists nothing in it; Remove of what's listed, and of a plant on record from before the move, takes nothing; the
// owner's .claude is as it was once the GM moves it back.
func TestARealFolderRenamedIntoAHiresPlaceIsMovedAndNothingThereIsListedOrRemoved(t *testing.T) {
	h, place, owner, settings := movedPlace(t, "abcdef012348")
	if h.FolderID == "" {
		t.Fatal("no identity kept at the hire")
	}
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, h) }); err != nil {
		t.Fatal(err)
	}
	if got, err := h.checkedFolder(); err != nil || got != place {
		t.Fatalf("the baseline: %q %v", got, err)
	}
	if got := listClaudePlantsOf(h); len(got) != 0 {
		t.Fatalf("the baseline listed %v", got)
	}
	back := renameInto(t, place, owner)
	_, err := h.checkedFolder()
	var moved *hireMoved
	if !errors.As(err, &moved) {
		t.Errorf("checkedFolder with the owner's folder renamed in: %v", err)
	}
	listed := listClaudePlantsOf(h)
	for _, p := range listed {
		t.Logf("listed: %s (%s)", p.File, p.Why)
		if p.File != place {
			t.Errorf("listed in the folder renamed in: %s", p.File)
		}
	}
	if len(listed) != 1 {
		t.Errorf("listed %d, want the moved place alone", len(listed))
	}
	if why := claudeHeldRefusal(filepath.Join(place, "sub")); !strings.Contains(why, "the hire's folder "+place+movedPlaceWords) || strings.Contains(why, "a hire left") {
		t.Errorf("the gate in the place: %q", why)
	}
	// A plant on record under the place, as a listing before the move would have kept it.
	p := claudeEntry(filepath.Join(place, ".claude"), whyClaudeFolder, place, place)
	p.Desk = h.ID
	sum := sha256.Sum256([]byte(p.Desk + "\x00" + p.File))
	p.ID = hex.EncodeToString(sum[:8])
	claudeHeld.Lock()
	claudeHeld.plants[p.ID] = p
	claudeHeld.Unlock()
	ids := []string{p.ID}
	for _, l := range listed {
		ids = append(ids, l.ID)
	}
	for _, id := range ids {
		if err := removeClaudePlant(id, false); err == nil {
			t.Errorf("Remove of %s went ahead", id)
		}
	}
	back()
	if data, err := os.ReadFile(settings); err != nil || string(data) != `{"standin":"the owner's own"}` {
		t.Errorf("the owner's .claude after Remove and the move back: %q %v", data, err)
	}
	if got, err := h.checkedFolder(); err != nil || got != place {
		t.Errorf("put back: %q %v", got, err)
	}
}

// A start: a row kept with its identity refuses a start in a folder renamed into its place; a row kept resolved
// without one (made before FXRC3FIX) gets it at its first start, as D21 keeps the resolution, and is then held to it.
func TestAStartInAFolderRenamedIntoItsPlaceIsRefusedAndAnOlderRowKeepsItsIdentityAtItsFirstStart(t *testing.T) {
	_, _, _, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	notes := physical(filepath.Join(homeDir, "code", "notes"))
	other := physical(filepath.Join(homeDir, "code", "other"))
	for _, f := range []string{notes, filepath.Join(other, ".claude")} {
		if err := os.MkdirAll(f, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	row := Hire{ID: "a00000000011", Name: "Ada", Role: "janitor", Folder: keptPath(notes), FolderResolved: notes, State: "working"}
	if err := os.MkdirAll(hireFolder(supportDir, row.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, row) }); err != nil {
		t.Fatal(err)
	}
	kept := func() string {
		t.Helper()
		for _, h := range registered(t) {
			if h.ID == row.ID {
				return h.FolderID
			}
		}
		t.Fatalf("no row %s", row.ID)
		return ""
	}
	if pl, err, made := r4Start(row); err != nil || made != nil || pl.Root != notes {
		t.Fatalf("its first start: Root %s, %v, %v", pl.Root, err, made)
	}
	if row.FolderID = kept(); row.FolderID == "" || row.FolderID != folderIdentity(notes) {
		t.Fatalf("its identity at its first start: %q, want %q", row.FolderID, folderIdentity(notes))
	}
	back := renameInto(t, notes, other)
	r4Refused(t, "the owner's folder renamed into its place", row, "This hire's folder, "+shownPath(notes)+
		", was moved or replaced since it was hired, so it didn't start. Fire it, then hire it again.")
	if _, err := row.checkedFolder(); err == nil {
		t.Error("checkedFolder with the owner's folder renamed in")
	}
	back()
	if pl, err, made := r4Start(row); err != nil || made != nil || pl.Root != notes {
		t.Errorf("put back: Root %s, %v, %v", pl.Root, err, made)
	}
}
