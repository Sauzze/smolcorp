package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// FXRC3FIXR1 (RT126RC3FIX's F-c): a plain hire into a folder in no repo has its launcher told the folder's identity at
// the hire beside its path (--place-id). A real folder of the owner's renamed into the place before the hire's one
// start resolves to the same path; its identity tells it apart, and the start is refused in the moved words. Put back,
// it starts there.
func TestAPlainHiresFolderRenamedIntoItsPlaceBeforeItsStartIsRefused(t *testing.T) {
	d, l, h := r7wDeck(t, true)
	notes := filepath.Join(l.projects, "notes")
	if err := os.Mkdir(notes, 0o700); err != nil {
		t.Fatal(err)
	}
	if status, said := d.Perform("hire", "", url.Values{"project": {"notes"}}); status != http.StatusNoContent {
		t.Fatalf("the plain hire into notes: %d %s", status, said)
	}
	folder, desk := r7wPlainHire(t, h)
	after := func(flag string) string {
		if i := slices.Index(h.tmux[0], flag); i >= 0 && i+1 < len(h.tmux[0]) {
			return h.tmux[0][i+1]
		}
		return ""
	}
	place, id := after("--place"), after("--place-id")
	if place != physical(notes) || id == "" || id != folderIdentity(physical(notes)) {
		t.Fatalf("its launcher: --place %q --place-id %q, want %q %q", place, id, physical(notes), folderIdentity(physical(notes)))
	}
	run, err := parseRun(h.tmux[0][slices.Index(h.tmux[0], "--hire"):])
	if err != nil || run.place != place || run.placeID != id {
		t.Fatalf("its launcher's arguments: %+v %v", run, err)
	}

	program := filepath.Join(l.base, "claude")
	writeFile(t, program, "#!/bin/sh\n")
	// The owner's own folder, holding their Claude settings, renamed into the place.
	owners := filepath.Join(l.base, "owners")
	writeFile(t, filepath.Join(owners, ".claude", "settings.json"), `{"the":"owner's"}`)
	if err := os.Rename(notes, notes+".hired"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(owners, notes); err != nil {
		t.Fatal(err)
	}
	want := "This hire's folder, " + shownPath(folder) + ", was moved or replaced since it was hired, so it didn't start. Fire it, then hire it again."
	if s, err := newLauncherSandboxPlaced(desk, run.place, run.placeID, folder, false, "L1", Claude, program); s != nil || err == nil || err.Error() != want {
		t.Errorf("its start with the owner's folder renamed in: %v, want %q", err, want)
		if s != nil {
			s.close()
		}
	}
	if err := os.Rename(notes, owners); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(notes+".hired", notes); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(owners, ".claude", "settings.json")); err != nil || string(got) != `{"the":"owner's"}` {
		t.Errorf("the owner's settings: %q %v", got, err)
	}

	// Put back, it starts there.
	s, err := newLauncherSandboxPlaced(desk, run.place, run.placeID, folder, false, "L1", Claude, program)
	if err != nil {
		t.Fatalf("its start, put back: %v", err)
	}
	t.Cleanup(s.close)
	if s.desk.places.Root != physical(notes) {
		t.Errorf("its place: %s, want %s", s.desk.places.Root, physical(notes))
	}
}

// The launcher takes --place-id only right after --place, in folderIdentity's shape.
func TestTheLauncherTakesAPlacesIdentityOnlyAfterItsPlace(t *testing.T) {
	const token = "0123456789ABCDEF0123456789ABCDEF"
	run, err := parseRun([]string{"--hire", token, "--desk", "plain-0a1b2c3d4e5f", "--place", "/x/notes", "--place-id", "16777232:4242", "claude"})
	if err != nil || run.place != "/x/notes" || run.placeID != "16777232:4242" {
		t.Errorf("a place and its identity: %+v %v", run, err)
	}
	for _, args := range [][]string{
		{"--hire", token, "--desk", "plain-0a1b2c3d4e5f", "--place-id", "1:2", "claude"},
		{"--hire", token, "--desk", "plain-0a1b2c3d4e5f", "--place", "/x/notes", "--place-id", "1:2:3", "claude"},
		{"--hire", token, "--desk", "plain-0a1b2c3d4e5f", "--place", "/x/notes", "--place-id", "", "claude"},
		{"--hire", token, "--desk", "plain-0a1b2c3d4e5f", "--place", "/x/notes", "--place-id", "a:1", "claude"},
	} {
		if run, err := parseRun(args); err == nil && run.placeID != "" {
			t.Errorf("%q: %+v", args, run)
		}
	}
}
