package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RT126SWEEP4 part 3's B2, as ruled (Decision 6 reversed for a moved place): a hire's place moved or replaced since its
// hire is held whole, in words that name it, never listed through the link put there; and Remove acts only through the
// place as it was resolved at the hire, refused while that place is moved. Stand-ins only: the test's own home folder,
// where scratch/x is a hire's folder with no worktree (one a General Manager can write) and owner/ a folder of the
// owner's with a .claude of its own.

// movedPlaceWords is what the gate says of a moved place (claudeHeldRefusalWithin).
const movedPlaceWords = " was moved or replaced, so Smolcorp can't tell what it left there; check it yourself"

// movedPlace makes the hire's folder and the owner's, and gives the hire.
func movedPlace(t *testing.T, id string) (h Hire, place, owner, settings string) {
	t.Helper()
	home := physical(isolate(t))
	homeDir = home
	freshClaudeHeld(t)
	place, owner = filepath.Join(home, "scratch", "x"), filepath.Join(home, "owner")
	for _, f := range []string{place, filepath.Join(owner, ".claude")} {
		if err := os.MkdirAll(f, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	settings = filepath.Join(owner, ".claude", "settings.json")
	writeFile(t, settings, `{"standin":"the owner's own"}`)
	return Hire{ID: id, Name: "Ada", Folder: keptPath(place), FolderResolved: place, State: "working"}, place, owner, settings
}

// moveAside renames place aside and puts a link to to in its place, as a GM could.
func moveAside(t *testing.T, place, to string) {
	t.Helper()
	if err := os.Rename(place, place+"-aside"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(to, place); err != nil {
		t.Fatal(err)
	}
}

// The sweep's TestZZSweep4P3RemoveThroughAMovedPlace: once the folder is a link to the owner's, the listing holds the
// folder itself, named as moved, and nothing through the link; the gate in the owner's folder names the moved place,
// not a file a hire "left"; Remove of what's listed takes nothing, and the owner's .claude is as it was.
func TestAMovedPlaceIsHeldWholeAndRemoveNeverReachesThroughIt(t *testing.T) {
	h, place, owner, settings := movedPlace(t, "abcdef012345")
	if got := listClaudePlantsOf(h); len(got) != 0 {
		t.Fatalf("the baseline listed %v", got)
	}
	moveAside(t, place, owner)
	listed := listClaudePlantsOf(h)
	for _, p := range listed {
		t.Logf("listed: %s (%s)", p.File, p.Why)
		if p.File != place || strings.HasPrefix(p.File, owner) {
			t.Errorf("listed through the link: %s", p.File)
		}
	}
	if len(listed) != 1 {
		t.Errorf("listed %d, want the moved place alone", len(listed))
	}
	why := claudeHeldRefusal(filepath.Join(owner, "sub"))
	if !strings.Contains(why, "the hire's folder "+place+movedPlaceWords) || strings.Contains(why, "a hire left") {
		t.Errorf("the gate in the owner's folder: %q", why)
	}
	for _, p := range listed {
		if err := removeClaudePlant(p.ID, false); err == nil {
			t.Errorf("Remove of %s went ahead", p.File)
		}
	}
	if data, err := os.ReadFile(settings); err != nil || string(data) != `{"standin":"the owner's own"}` {
		t.Errorf("the owner's .claude after Remove: %q %v", data, err)
	}
}

// The gate's own listing (the worker's, asked for an open hire near the folder) holds the moved place the same way.
func TestTheGatesListingHoldsAMovedPlaceWhole(t *testing.T) {
	h, place, owner, _ := movedPlace(t, "abcdef012346")
	moveAside(t, place, owner)
	why := claudeGate(filepath.Join(owner, "sub"), []Hire{h})
	if !strings.Contains(why, "the hire's folder "+place+movedPlaceWords) || strings.Contains(why, "a hire left") {
		t.Errorf("the gate in the owner's folder: %q", why)
	}
}

// A plant on record from a listing through the link (as before this fix, or a record kept from then): Remove refuses
// while the hire's place is moved, and takes nothing by the path.
func TestRemoveRefusesAPlantWhoseHiresPlaceIsMoved(t *testing.T) {
	h, place, owner, settings := movedPlace(t, "abcdef012347")
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, h) }); err != nil {
		t.Fatal(err)
	}
	moveAside(t, place, owner)
	p := claudeEntry(filepath.Join(place, ".claude"), whyClaudeFolder, place, place)
	if p.Sum == "" {
		t.Fatalf("the stand-in record can't be summed: %+v", p)
	}
	p.Desk = h.ID
	sum := sha256.Sum256([]byte(p.Desk + "\x00" + p.File))
	p.ID = hex.EncodeToString(sum[:8])
	claudeHeld.Lock()
	claudeHeld.plants[p.ID] = p
	claudeHeld.Unlock()
	err := removeClaudePlant(p.ID, false)
	t.Logf("Remove: %v", err)
	if err == nil {
		t.Error("Remove went ahead through the moved place")
	}
	if data, err := os.ReadFile(settings); err != nil || string(data) != `{"standin":"the owner's own"}` {
		t.Errorf("the owner's .claude after Remove: %q %v", data, err)
	}
}
