//go:build !windows

package desktop

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const briefBefore = "# Task\nfirst\n"

// gmBriefFile makes the hire's BRIEF file where a managed hire's is kept.
func gmBriefFile(t *testing.T, g *gmRig) string {
	t.Helper()
	file := filepath.Join(hireFolder(supportDir, g.hire.ID), "BRIEF.md")
	os.MkdirAll(filepath.Dir(file), 0o755)
	if err := os.WriteFile(file, []byte(briefBefore), 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestAGMBriefRefusesEveryRowOfBriefJSONAndTheGMAndABriefThatIsNoPlainFileAndWritesNothing(t *testing.T) {
	var file string
	brief := func(setup func(*gmRig)) gmRow {
		return gmRow{status: 409, request: map[string]any{"text": "x"}, setup: func(g *gmRig) { file = gmBriefFile(t, g); setup(g) }}
	}
	rows := []gmRow{{}, {}, {}, brief(func(g *gmRig) { markFired(g.hire.ID, "") }),
		{request: map[string]any{"text": strings.Repeat("x", 11)}, setup: func(g *gmRig) { g.d.mu.Lock(); g.d.config.MailMaxBytes = 10; g.d.mu.Unlock() }},
		brief(func(g *gmRig) { g.changeRow(func(h *Hire) { h.Role = gmRole }) }),
		brief(func(*gmRig) { os.Remove(file) }),
		brief(func(g *gmRig) { os.Remove(file); os.Symlink(g.r.repo, file) })}
	gmRefusals(t, "brief.json", "/setup/gm/brief", rows, func(i int, g *gmRig, w *httptest.ResponseRecorder) {
		if data, _ := os.ReadFile(file); len(data) > len(briefBefore) && i != 7 || i == 3 && w.Body.String() != "Rex is already gone." {
			t.Errorf("row %d: %q, file %q", i, w.Body, data)
		}
	})
}

// End to end: a brief, then a fire, then a fire again.
func TestAGMBriefAddsADatedPartAndMailsTheHireThenItIsFiredWithoutAnythingTyped(t *testing.T) {
	fixture := decodedFixture(t, "brief.json")
	for _, asGM := range []bool{true, false} {
		g := newGMRig(t, "Rex")
		file := gmBriefFile(t, g)
		w := g.post(t, asGM, "/setup/gm/brief", g.with(map[string]any{"text": "  Also check the exporter. "}))
		got := decoded(t, w)
		mail, _ := got["mail"].(map[string]any)
		data, _ := os.ReadFile(file)
		id, _ := mail["id"].(string)
		sent, _ := g.d.mail.get(id)
		// The note names the file by its whole path (it is outside the hire's worktree), as the Mac's does.
		sender := map[bool]mailSender{true: mailFromGM, false: mailFromOwner}[asGM]
		if w.Code != http.StatusOK || got["ok"] != true || got["file"] != file || mail["state"] != "queued" || mail["to"] != g.hire.ID || len(id) != 18 || !strings.HasPrefix(id, "brief-") ||
			!strings.HasPrefix(string(data), briefBefore+"\n\n## New part, 20") || !strings.HasSuffix(string(data), "\n\nAlso check the exporter.\n") ||
			sent.From != sender || sent.Text != strings.TrimSuffix(fixture["mail-the-hire-gets"].(string), "BRIEF.md")+file || g.typed() {
			t.Fatalf("%d %s, file %q, mail %+v, typed %v", w.Code, w.Body, data, sent, g.typed())
		}
		w = g.fire(t, asGM, map[string]any{"removeWorktree": true})
		if _, err := os.Stat(g.hire.Worktree); w.Code != http.StatusNoContent || g.r.tmux.up || !g.r.tmux.ran("kill-server") || g.typed() || registered(t)[0].State != "fired" || err == nil {
			t.Fatalf("fire: %d %q (worktree %v)", w.Code, w.Body, err)
		}
		recap, _ := os.ReadFile(filepath.Join(supportDir, "recaps", g.hire.ID+".md"))
		for _, line := range append([]any{"# Recap: Rex"}, decodedFixture(t, "fire.json")["recap"].(map[string]any)["lines"].([]any)[1:]...) {
			if !strings.Contains(string(recap), line.(string)) {
				t.Errorf("the recap has no %q", line)
			}
		}
		if w := g.fire(t, asGM, nil); w.Code != http.StatusConflict || w.Body.String() != "Rex is already gone." {
			t.Errorf("again: %d %q", w.Code, w.Body)
		}
	}
}
