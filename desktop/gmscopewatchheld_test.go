//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// RT126WATCHHELD B1: a held GM's file watch, set legally inside its project, must not read another hire's file
// later, when a folder on its path is made a link. Each look reads it only while its path still resolves to itself.

// heldWatch sets a file watch on file for who, through the route as the gate handed it on (held: the GM's scope,
// nil for none), and gives its id.
func heldWatch(t *testing.T, d *Deck, gm Hire, file string, who caller, held *gmScope) string {
	t.Helper()
	body := map[string]any{"kind": "file", "target": file, "pattern": ".", "repeat": true}
	if held != nil {
		data, _ := json.Marshal(body)
		body = heldChecked(t, d, gm, "/setup/gm/watch", string(data))
	}
	status, text := watchRoute(d, body, who, held)
	var made struct{ ID string }
	if status != http.StatusOK || json.Unmarshal([]byte(text), &made) != nil || made.ID == "" {
		t.Fatalf("the watch on %s: %d %q", file, status, text)
	}
	return made.ID
}

// lookOnce is one look of w's file watches, as tick does it: what fired, and the watches it read further in.
func lookOnce(w *watchers) (fires []watchFire, read []watch) {
	w.mu.Lock()
	watches := slices.Clone(w.all)
	w.mu.Unlock()
	read, fires = checkWatches(watches, w.roots(), w.hires(), w.home(), nil, nil, 0, nil, nil)
	w.mu.Lock()
	w.apply(fires, read)
	w.mu.Unlock()
	return fires, read
}

// linkAside moves folder aside and puts a link to target in its place.
func linkAside(t *testing.T, folder, target string) {
	t.Helper()
	if err := os.Rename(folder, folder+"-aside"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, folder); err != nil {
		t.Fatal(err)
	}
}

func appendLine(t *testing.T, file, line string) {
	t.Helper()
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		t.Fatal(err)
	}
}

// firedFor is the details of the fires of watch id.
func firedFor(fires []watchFire, id string) []string {
	var out []string
	for _, f := range fires {
		if f.id == id {
			out = append(out, f.detail)
		}
	}
	return out
}

// readFor is whether the look read watch id further in.
func readFor(read []watch, id string) bool {
	return slices.ContainsFunc(read, func(x watch) bool { return x.ID == id })
}

// pD1: a held watch on a file that is there. proj/real becomes a link to Bea's elsewhere/real: no look reads or
// fires, not even after Bea's file grows.
func TestAHeldGMsFileWatchReadsNothingOnceAFolderOnItsPathLeadsToAnotherHire(t *testing.T) {
	d, gm, _, proj, elsewhere := heldNames(t)
	watchFiles(t, proj, elsewhere)
	held := gmScopeOf(gm)
	id := heldWatch(t, d, gm, filepath.Join(proj, "real", "log.txt"), callerGM, &held)
	linkAside(t, filepath.Join(proj, "real"), filepath.Join(elsewhere, "real"))
	for look, grow := range []bool{false, true} {
		if grow {
			appendLine(t, filepath.Join(elsewhere, "real", "log.txt"), "bea's line")
		}
		fires, read := lookOnce(d.watchers)
		if got := firedFor(fires, id); len(got) != 0 || readFor(read, id) {
			t.Errorf("look %d after the swap: fired %q, read further %v; want nothing", look+1, got, readFor(read, id))
		}
	}
}

// pD2: the same for a file not there yet. Bea's existing line doesn't fire.
func TestAHeldGMsWatchOnAFileNotThereYetReadsNothingOnceAFolderOnItsPathLeadsOut(t *testing.T) {
	d, gm, _, proj, elsewhere := heldNames(t)
	watchFiles(t, proj, elsewhere)
	appendLine(t, filepath.Join(elsewhere, "real", "later.txt"), "bea's existing line")
	held := gmScopeOf(gm)
	id := heldWatch(t, d, gm, filepath.Join(proj, "real", "later.txt"), callerGM, &held)
	linkAside(t, filepath.Join(proj, "real"), filepath.Join(elsewhere, "real"))
	fires, read := lookOnce(d.watchers)
	if got := firedFor(fires, id); len(got) != 0 || readFor(read, id) {
		t.Errorf("after the swap: fired %q, read further %v; want nothing", got, readFor(read, id))
	}
}

// The owner's watch and an unheld GM's across the same swap behave as before: a hire's folder is theirs to watch. A
// file that's there is taken from its end (a new file), then its new line fires; one not there yet fires from its
// first line.
func TestTheOwnersAndAnUnheldGMsFileWatchesReadAcrossTheSameSwapAsBefore(t *testing.T) {
	d, gm, _, proj, elsewhere := heldNames(t)
	watchFiles(t, proj, elsewhere)
	appendLine(t, filepath.Join(elsewhere, "real", "later.txt"), "bea's existing line")
	var there, later []string
	for _, who := range []caller{callerPage, callerGM} {
		there = append(there, heldWatch(t, d, gm, filepath.Join(proj, "real", "log.txt"), who, nil))
		later = append(later, heldWatch(t, d, gm, filepath.Join(proj, "real", "later.txt"), who, nil))
	}
	linkAside(t, filepath.Join(proj, "real"), filepath.Join(elsewhere, "real"))
	fires, _ := lookOnce(d.watchers)
	for i, id := range later {
		if got := firedFor(fires, id); len(got) != 1 || got[0] != "later.txt: bea's existing line" {
			t.Errorf("watch %d on a file not there yet: fired %q, want Bea's existing line", i, got)
		}
	}
	for i, id := range there {
		if got := firedFor(fires, id); len(got) != 0 {
			t.Errorf("watch %d on a file that's there, at the first look: fired %q, want nothing yet", i, got)
		}
	}
	appendLine(t, filepath.Join(elsewhere, "real", "log.txt"), "bea's line")
	fires, _ = lookOnce(d.watchers)
	for i, id := range there {
		if got := firedFor(fires, id); len(got) != 1 || got[0] != "log.txt: bea's line" {
			t.Errorf("watch %d on a file that's there: fired %q, want Bea's new line", i, got)
		}
	}
}

// A held watch is saved held: a new run of the app reads it back still held, and it still reads nothing after a
// swap.
func TestAHeldGMsFileWatchIsStillHeldAfterARestart(t *testing.T) {
	d, gm, _, proj, elsewhere := heldNames(t)
	watchFiles(t, proj, elsewhere)
	held := gmScopeOf(gm)
	id := heldWatch(t, d, gm, filepath.Join(proj, "real", "log.txt"), callerGM, &held)
	var kept struct{ Watches []map[string]any }
	data, err := os.ReadFile(d.watchers.file)
	if err != nil || json.Unmarshal(data, &kept) != nil {
		t.Fatalf("watchers.json: %v %s", err, data)
	}
	i := slices.IndexFunc(kept.Watches, func(x map[string]any) bool { return x["id"] == id })
	if i < 0 || kept.Watches[i]["held"] != proj {
		t.Errorf("watchers.json keeps %s, want it held to %s", data, proj)
	}
	again := newWatchers(filepath.Dir(d.watchers.file), homeDir, d.watchers.roots, d.watchers.hires, d.events.append, nil)
	if _, ok := again.get(id); !ok {
		t.Fatalf("the new run has no watch %s", id)
	}
	linkAside(t, filepath.Join(proj, "real"), filepath.Join(elsewhere, "real"))
	for look, grow := range []bool{false, true} {
		if grow {
			appendLine(t, filepath.Join(elsewhere, "real", "log.txt"), "bea's line")
		}
		fires, read := lookOnce(again)
		if got := firedFor(fires, id); len(got) != 0 || readFor(read, id) {
			t.Errorf("look %d after the restart and the swap: fired %q, read further %v; want nothing", look+1, got, readFor(read, id))
		}
	}
}

// A held watch whose path is unchanged fires as before: a file that's there, and a file whose folder wasn't there
// when it was set and then comes as a plain folder.
func TestAHeldGMsFileWatchOnAnUnchangedPathStillFires(t *testing.T) {
	d, gm, _, proj, elsewhere := heldNames(t)
	watchFiles(t, proj, elsewhere)
	held := gmScopeOf(gm)
	there := heldWatch(t, d, gm, filepath.Join(proj, "real", "log.txt"), callerGM, &held)
	later := heldWatch(t, d, gm, filepath.Join(proj, "new", "later.txt"), callerGM, &held)
	if err := os.MkdirAll(filepath.Join(proj, "new"), 0o755); err != nil {
		t.Fatal(err)
	}
	appendLine(t, filepath.Join(proj, "new", "later.txt"), "a first line")
	appendLine(t, filepath.Join(proj, "real", "log.txt"), "a new line")
	fires, _ := lookOnce(d.watchers)
	for id, want := range map[string]string{there: "log.txt: a new line", later: "later.txt: a first line"} {
		if got := firedFor(fires, id); len(got) != 1 || !strings.HasSuffix(got[0], want) {
			t.Errorf("watch %s: fired %q, want %q", id, got, want)
		}
	}
}
