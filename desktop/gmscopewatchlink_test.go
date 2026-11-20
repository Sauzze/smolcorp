//go:build !windows

package desktop

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RT126POLISHBR2 N1: a held GM's scope checks a file watch's path with its links followed, and the route follows
// them again. A link on the way, swapped between the two, must not land the watch on a file outside the project.

// watchFiles makes proj/real/log.txt and elsewhere/real/log.txt (elsewhere is Bea's folder: open, outside the
// project), and the link proj/L to proj/real.
func watchFiles(t *testing.T, proj, elsewhere string) (link string) {
	t.Helper()
	for _, dir := range []string{filepath.Join(proj, "real"), filepath.Join(elsewhere, "real")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "log.txt"), []byte("old\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	link = filepath.Join(proj, "L")
	if err := os.Symlink(filepath.Join(proj, "real"), link); err != nil {
		t.Fatal(err)
	}
	return link
}

// pointAt makes link lead to target from now on.
func pointAt(t *testing.T, link, target string) {
	t.Helper()
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

// watchRoute is POST /setup/gm/watch as the router serves it, given body (as the gate handed it on) for who, held
// to held (nil: not held).
func watchRoute(d *Deck, body map[string]any, who caller, held *gmScope) (int, string) {
	data, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/setup/gm/watch", bytes.NewReader(data))
	(&router{deck: d}).watchRoutes(w, &request{Request: req, caller: who, heldTo: held}, "/setup/gm/watch")
	return w.Code, w.Body.String()
}

// The reviewer's pW: checked with the link inside the project, the link swapped to Bea's folder, then the route,
// held to the scope, refuses with the gate's words and keeps nothing. With the link back inside, it keeps the watch.
func TestAHeldGMsFileWatchThroughALinkSwappedAfterItsCheckIsRefused(t *testing.T) {
	d, gm, _, proj, elsewhere := heldNames(t)
	link := watchFiles(t, proj, elsewhere)
	held := gmScopeOf(gm)
	asked := heldChecked(t, d, gm, "/setup/gm/watch", `{"kind":"file","target":"`+filepath.Join(link, "log.txt")+`","pattern":"."}`)
	pointAt(t, link, filepath.Join(elsewhere, "real"))
	words := "This General Manager watches only its own hires and the files in " + proj + "."
	if status, text := watchRoute(d, asked, callerGM, &held); status != http.StatusForbidden || text != words {
		t.Errorf("the route after the swap: %d %q, want 403 %q", status, text, words)
	}
	if kept := d.watchers.list(); len(kept) != 0 {
		t.Errorf("after the refusal, the watches are %v, want none", kept)
	}
	pointAt(t, link, filepath.Join(proj, "real"))
	status, text := watchRoute(d, asked, callerGM, &held)
	if want := `"target":"` + filepath.Join(proj, "real", "log.txt") + `"`; status != http.StatusOK || !strings.Contains(text, want) {
		t.Errorf("the route with the link inside: %d %q, want 200 with %s", status, text, want)
	}
}

// The owner's watch and an unheld GM's (no heldTo) through the same link, out to Bea's folder, are kept as before:
// a hire's folder is theirs to watch.
func TestTheOwnersAndAnUnheldGMsFileWatchThroughTheSameLinkStillWork(t *testing.T) {
	d, _, _, proj, elsewhere := heldNames(t)
	link := watchFiles(t, proj, elsewhere)
	pointAt(t, link, filepath.Join(elsewhere, "real"))
	body := map[string]any{"kind": "file", "target": filepath.Join(link, "log.txt"), "pattern": "."}
	want := `"target":"` + filepath.Join(elsewhere, "real", "log.txt") + `"`
	for _, who := range []caller{callerPage, callerGM} {
		if status, text := watchRoute(d, body, who, nil); status != http.StatusOK || !strings.Contains(text, want) {
			t.Errorf("caller %v: %d %q, want 200 with %s", who, status, text, want)
		}
	}
	if kept := d.watchers.list(); len(kept) != 2 {
		t.Errorf("the watches are %v, want the two", kept)
	}
}

// A folder on the way, a plain folder at the check, made a link to Bea's folder after it: also refused, nothing kept.
func TestAHeldGMsFileWatchWhoseFolderBecomesALinkAfterItsCheckIsRefused(t *testing.T) {
	d, gm, _, proj, elsewhere := heldNames(t)
	watchFiles(t, proj, elsewhere)
	held := gmScopeOf(gm)
	sub := filepath.Join(proj, "real")
	asked := heldChecked(t, d, gm, "/setup/gm/watch", `{"kind":"file","target":"`+filepath.Join(sub, "log.txt")+`","pattern":"."}`)
	if err := os.Rename(sub, sub+"-aside"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(elsewhere, "real"), sub); err != nil {
		t.Fatal(err)
	}
	if status, text := watchRoute(d, asked, callerGM, &held); !scopeRefused(status, text) {
		t.Errorf("the route after the folder became a link: %d %q, want the scope's 403", status, text)
	}
	if kept := d.watchers.list(); len(kept) != 0 {
		t.Errorf("after the refusal, the watches are %v, want none", kept)
	}
}

// The nickname route does the same with the folder its gate checked: still held there, and never followed again.
// The project made a link to Bea's folder after the check doesn't name Bea's desk.
func TestAHeldGMsNicknameAfterItsFolderBecomesALinkNamesNoOneOutside(t *testing.T) {
	d, gm, _, proj, elsewhere := heldNames(t)
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "session_id": "s-bea", "cwd": elsewhere}, Claude, "tab-bea", "", 1761000540000)
	d.mu.Unlock()
	held := gmScopeOf(gm)
	asked := heldChecked(t, d, gm, "/setup/gm/nickname", `{"folder":"`+proj+`","name":"Eve"}`)
	if err := os.Rename(proj, proj+"-aside"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, proj); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(asked)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/setup/gm/nickname", bytes.NewReader(data))
	(&router{deck: d}).setup(w, &request{Request: req, caller: callerGM, heldTo: &held})
	if !scopeRefused(w.Code, w.Body.String()) {
		t.Errorf("the nickname after the swap: %d %q, want the scope's 403", w.Code, w.Body.String())
	}
	d.mu.Lock()
	named := d.config.Layout.Room.Team[lastPart(elsewhere)].Name
	d.mu.Unlock()
	if named != "" {
		t.Errorf("Bea's desk is named %q, want no name", named)
	}
}
