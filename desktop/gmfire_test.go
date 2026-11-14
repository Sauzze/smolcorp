//go:build !windows

package desktop

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The GM's fire and brief routes, on a hire made on a stand-in tmux (hirefire_test.go's way: /setup/gm/hire is PR1).
type gmRig struct {
	d     *Deck
	r     *engineRig
	hire  Hire
	token string
	setup int // tmux commands made before the route's (the hire's own start)
}

func newGMRig(t *testing.T, nickname string) *gmRig {
	t.Helper()
	d, r, hire := hiredForFire(t, nickname)
	managerOn(d)
	windowShows(r, pane(t, "idle-empty"), true)
	d.engine = r.o
	return &gmRig{d: d, r: r, hire: hire, token: d.newGMToken(), setup: len(r.tmux.asked)}
}

func (g *gmRig) post(t *testing.T, asGM bool, path string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	data, _ := json.Marshal(body)
	c := call{method: "POST", target: path, host: "localhost:7755", from: thisComputer, body: string(data), headers: map[string]string{setupHeader: "1"}}
	if asGM {
		c.withoutSetupKey, c.headers[gmTokenHeader] = true, g.token
	}
	return serve(t, g.d, c)
}

func (g *gmRig) with(extra map[string]any) map[string]any {
	body := map[string]any{"id": g.hire.ID}
	maps.Copy(body, extra)
	return body
}

func (g *gmRig) fire(t *testing.T, asGM bool, extra map[string]any) *httptest.ResponseRecorder {
	return g.post(t, asGM, "/setup/gm/fire", g.with(extra))
}

func (g *gmRig) asked() [][]string { return g.r.tmux.asked[g.setup:] }

func (g *gmRig) typed() bool {
	return g.r.tmux.ran("send-keys") || g.r.tmux.ran("paste-buffer") || g.r.tmux.ran("load-buffer")
}

func (g *gmRig) showing(t *testing.T, window string) {
	windowShows(g.r, pane(t, window), true)
	g.d.engine = g.r.o
}

func (g *gmRig) changeRow(change func(*Hire)) {
	updateRegistry(func(f *registryFile) { change(&f.hires[0]) })
}

func decodedFixture(t *testing.T, name string) (f map[string]any) {
	json.Unmarshal(frontDeskFixture(t, name), &f)
	return f
}

type gmRow struct {
	setup   func(*gmRig)
	request map[string]any
	status  int
}

// gmRefusals runs the rows of a fixture's refusals, then the extra rows, each on a fresh hire named Rex; check sees every answer.
func gmRefusals(t *testing.T, fixture, path string, rows []gmRow, check func(i int, g *gmRig, w *httptest.ResponseRecorder)) {
	t.Helper()
	refusals, _ := decodedFixture(t, fixture)["refusals"].([]any)
	if len(refusals) > len(rows) {
		t.Fatalf("%s has %d refusals, the test knows %d", fixture, len(refusals), len(rows))
	}
	for i, row := range rows {
		g := newGMRig(t, "Rex")
		request, status := row.request, row.status
		if i < len(refusals) {
			fixed := refusals[i].(map[string]any)
			request, status = fixed["request"].(map[string]any), int(fixed["status"].(float64))
			if request["id"] == "abcdef123456" {
				request["id"] = g.hire.ID
			}
		} else if request["id"] == nil {
			request = g.with(request)
		}
		if row.setup != nil {
			row.setup(g)
		}
		w := g.post(t, true, path, request)
		if w.Code != status || g.typed() {
			t.Errorf("row %d: %d %q, want %d (typed %v)", i, w.Code, w.Body, status, g.typed())
		}
		check(i, g, w)
	}
}

func TestAGMFireRefusesEveryRowOfFireJSONWithItsStatusAndNeverTypes(t *testing.T) {
	dirty := func(g *gmRig) { os.WriteFile(filepath.Join(g.hire.Worktree, "new.txt"), []byte("x"), 0o644) }
	rows := []gmRow{{}, {}, {setup: dirty}, {setup: func(g *gmRig) { g.showing(t, "permission-menu") }}, {setup: func(g *gmRig) { markFired(g.hire.ID, "") }},
		{request: map[string]any{"id": "live:s1"}, status: 404},
		{setup: func(g *gmRig) { g.d.mu.Lock(); g.d.config.HiringEnabled = false; g.d.mu.Unlock() }, status: 403}}
	gmRefusals(t, "fire.json", "/setup/gm/fire", rows, func(i int, g *gmRig, w *httptest.ResponseRecorder) {
		if i >= 2 && i < 5 && (!strings.HasPrefix(w.Body.String(), "Rex ") || strings.Contains(w.Body.String(), "janitor")) ||
			i == 2 && !strings.Contains(w.Body.String(), "new.txt") || (i == 3 || i == 6) && (g.r.tmux.ran("kill-server") || !g.r.tmux.up) {
			t.Errorf("row %d: %q, tmux %q", i, w.Body, g.asked())
		}
	})
}

func TestAForcedGMFireKeepsTheWorktreeWithChangesAndSaysWhy(t *testing.T) {
	g := newGMRig(t, "Rex")
	os.WriteFile(filepath.Join(g.hire.Worktree, "new.txt"), []byte("x"), 0o644)
	w := g.fire(t, true, map[string]any{"force": true, "removeWorktree": true})
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if _, err := os.Stat(g.hire.Worktree); w.Code != http.StatusOK || got["ok"] != true || got["kept"] != firedWorktreeKept ||
		got["worktree"] != g.hire.Worktree || got["branch"] != g.hire.Branch || err != nil || g.typed() {
		t.Fatalf("%d %q (worktree %v)", w.Code, w.Body, err)
	}
}

// As the Mac and fire.json: a clean worktree stays unless removeWorktree is true, a plain fire is 204 either way,
// and a worktree kept because removal wasn't asked leaves no note in the registry (HireFire.swift:263).
func TestAGMFireKeepsTheCleanWorktreeUnlessAskedToRemoveIt(t *testing.T) {
	for _, c := range []struct {
		extra map[string]any
		dirty bool
		kept  bool
	}{{nil, false, true}, {map[string]any{"removeWorktree": false}, false, true}, {map[string]any{"removeWorktree": true}, false, false},
		{map[string]any{"force": true}, true, true}} {
		g := newGMRig(t, "Rex")
		if c.dirty {
			os.WriteFile(filepath.Join(g.hire.Worktree, "new.txt"), []byte("x"), 0o644)
		}
		w := g.fire(t, true, c.extra)
		_, err := os.Stat(g.hire.Worktree)
		if w.Code != http.StatusNoContent || w.Body.Len() != 0 || (err == nil) != c.kept || registered(t)[0].State != "fired" || registered(t)[0].Notes != "" {
			t.Errorf("%v: %d %q (worktree %v)", c.extra, w.Code, w.Body, err)
		}
	}
}

func TestAGMFireNeverTypesWithOrWithoutForceAndTouchesOnlyTheHiresOwnSocket(t *testing.T) {
	for _, shown := range []string{"permission-menu", "busy-1", "draft", "idle-empty"} {
		for _, force := range []bool{false, true} {
			g := newGMRig(t, "")
			g.showing(t, shown)
			g.fire(t, true, map[string]any{"force": force})
			for _, a := range g.asked() {
				if a[0] != "-S" || a[1] != g.hire.TmuxSocket || g.typed() {
					t.Errorf("%s, force %v: %q", shown, force, a)
				}
			}
		}
	}
	// Another socket, or a launcher hire with no window: nothing is asked, ended or removed.
	for _, change := range []func(*Hire){func(h *Hire) { h.TmuxSocket = filepath.Join(t.TempDir(), "other") }, func(h *Hire) { h.ViaLauncher = true }} {
		g := newGMRig(t, "Rex")
		g.changeRow(change)
		w := g.fire(t, true, map[string]any{"force": true})
		_, tree := os.Stat(g.hire.Worktree)
		_, repo := os.Stat(g.r.repo) // neither was removed
		if w.Code != http.StatusConflict || !strings.HasPrefix(w.Body.String(), "Couldn't reach Rex's") || len(g.asked()) != 0 || registered(t)[0].State == "fired" || tree != nil || repo != nil {
			t.Errorf("%d %q, tmux %q", w.Code, w.Body, g.asked())
		}
	}
}

func TestTheGMsTokenCantFireTheGMButThePageCan(t *testing.T) {
	g := newGMRig(t, "Boss")
	g.changeRow(func(h *Hire) { h.Role = gmRole })
	if w := g.fire(t, true, map[string]any{"force": true}); w.Code != http.StatusConflict || w.Body.String() != gmFireNotSelf || len(g.asked()) != 0 || registered(t)[0].State == "fired" {
		t.Fatalf("%d %q, tmux %q", w.Code, w.Body, g.asked())
	}
	if w := g.fire(t, false, map[string]any{"removeWorktree": true}); w.Code != http.StatusNoContent || registered(t)[0].State != "fired" || g.d.gmToken != "" {
		t.Fatalf("the page: %d %q, token %q", w.Code, w.Body, g.d.gmToken)
	}
}
