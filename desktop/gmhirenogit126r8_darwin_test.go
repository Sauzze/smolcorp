//go:build darwin

package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// FXGMHIRENOGIT126 round 8: round 7's "found, not fixed" 1–3. The git panel read a moved tree where it led (the GM's
// folder, for a tree swapped for a link); a hire taken over whose tree turned could be neither handed back nor fired;
// and Hand back's resume checked a tree made in config.json's hireWorktrees against the folder beside the repo. The
// links, renames and the GM-made folders are the test's own, in its temp folders. Nothing reaches a real T3 Code.

// The git panel: a desk's own /desk/git and the GM's /setup/gm/git?hire= read a hire's worktree as hired, and run no
// git at all once it was swapped for a link to a folder of the GM's; each says plainly why. The control is the same
// desk and hire before the swap.
func TestTheGitPanelRunsNoGitInAMovedTree(t *testing.T) {
	d, keys, sockets := keyedOffice(t)
	managerOn(d)
	worktree := filepath.Join(filepath.Dir(sockets["aaa111"]), "wt-aaa111")
	gitRepoCmd(t, filepath.Dir(worktree), "init", "-q", "-b", "main", worktree)
	fake := filepath.Join(physical(t.TempDir()), "gm-made")
	gitRepoCmd(t, filepath.Dir(fake), "init", "-q", "-b", "main", fake)
	writeFile(t, filepath.Join(fake, "a.txt"), "the GM's\n")

	var mu sync.Mutex
	var ran []string // the folder each git read ran in, resolved
	read := func(ctx context.Context, dir string, call gitCall, args ...string) ([]byte, int, error) {
		mu.Lock()
		ran = append(ran, physical(dir)+" "+args[0])
		mu.Unlock()
		return readGitOut(ctx, dir, call, args...)
	}
	saved := gits
	t.Cleanup(func() { gits = saved })
	fresh := func() { // no state kept from the read before
		gits = newGitStates()
		gits.read = read
		mu.Lock()
		ran = nil
		mu.Unlock()
	}
	runs := func(in string) (n int) {
		mu.Lock()
		defer mu.Unlock()
		for _, r := range ran {
			if strings.HasPrefix(r, in+"/") || strings.HasPrefix(r, in+" ") {
				n++
			}
		}
		return n
	}
	gmRead := func(t *testing.T) GitState {
		t.Helper()
		got := serve(t, d, call{method: "GET", target: "/setup/gm/git?hire=aaa111", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
		var state GitState
		if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &state) != nil {
			t.Fatalf("the GM's read: %d %s", got.Code, got.Body)
		}
		return state
	}
	if code := hookAs(t, d, keys["aaa111"], tmuxTabOn(sockets["aaa111"]), "UserPromptSubmit", "s-top", worktree); code != http.StatusNoContent {
		t.Fatalf("the hook: %d", code)
	}

	t.Run("its tree as hired is read", func(t *testing.T) {
		fresh()
		if code, answer := d.deskGit("s-top"); code != http.StatusOK || !gitAnswer(t, answer).Repo || gitAnswer(t, answer).Skipped {
			t.Errorf("the desk's window: %d %+v", code, answer)
		}
		if state := gmRead(t); !state.Repo || state.Skipped {
			t.Errorf("the GM's read: %+v", state)
		}
		if runs(worktree) == 0 {
			t.Error("no git ran in the tree: the control proves nothing")
		}
	})

	t.Run("its tree swapped for a link", func(t *testing.T) {
		if err := os.Rename(worktree, worktree+".real"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(fake, worktree); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(worktree); _ = os.Rename(worktree+".real", worktree) })
		fresh()
		code, answer := d.deskGit("s-top")
		if state := gitAnswer(t, answer); code != http.StatusOK || state.Repo || !state.Skipped || state.Note != deskMovedNote {
			t.Errorf("the desk's window: %d %+v", code, answer)
		}
		if state := gmRead(t); state.Repo || !state.Skipped || state.Note != deskMovedNote {
			t.Errorf("the GM's read: %+v", state)
		}
		mu.Lock()
		all := ran
		mu.Unlock()
		if runs(fake) != 0 || len(all) != 0 {
			t.Errorf("git ran: %q", all)
		}
		if deskMovedNote != "This hire's worktree was moved or replaced since it was hired, so its git state isn't read. Fire it, then hire it again." {
			t.Errorf("the words: %q", deskMovedNote)
		}
	})
}

// A hire taken over in T3 Code whose folder for hires turned into a link since (so Hand back refuses it): the fire,
// through its route as Setup and the GM call it, goes ahead. What's left of its tmux desk is ended, the hire is fired,
// its tree is kept with no git run in the GM-made folder or the other repo and nothing removed or written there, and
// the words say to remove T3 Code's project and the folder by hand. The control, its tree as hired, is still refused
// with nothing asked of tmux: it's handed back first.
func TestATakenOverHireWhoseTreeTurnedCanBeFired(t *testing.T) {
	takenOver := func(g *gmRig) {
		g.changeRow(func(h *Hire) {
			h.Engine, h.T3Session, h.T3Thread, h.State = t3Engine, t3TakeOverSession, t3ImportedThread+t3TakeOverSession, "idle"
		})
	}

	t.Run("its tree as hired: refused", func(t *testing.T) {
		stopDecksOf(t)
		g := newGMRig(t, "Rex")
		takenOver(g)
		w := g.fire(t, true, map[string]any{"force": true})
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "Rex was taken over in T3 Code: hand it back first, then fire it.") || len(g.asked()) != 0 {
			t.Errorf("%d %q, tmux asked %q", w.Code, w.Body, g.asked())
		}
		if h, _ := registryHire(g.hire.ID); !isOpen(h) || h.T3Session == "" {
			t.Errorf("%+v", h)
		}
	})

	for _, c := range []struct {
		name string
		asGM bool
		up   bool // its tmux desk still runs (Take over leaves it ended)
	}{{"from Setup, its desk ended at Take over", false, false}, {"by the GM, its desk still up", true, true}} {
		t.Run(c.name, func(t *testing.T) {
			stopDecksOf(t)
			g := newGMRig(t, "Rex")
			takenOver(g)
			g.r.tmux.up = c.up
			other, before := r6Other(t, g.hire.Branch)
			calls := gitCalls(t)
			gm, _ := r6Turn(t, g.hire.Worktree.storedOnly(), other)
			if !hireTreeTurned(g.hire) {
				t.Fatal("the turned tree isn't turned: this row proves nothing")
			}
			w := g.fire(t, c.asGM, map[string]any{"removeWorktree": true})
			var answer map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &answer)
			note, _ := answer["note"].(string)
			words := "Rex was taken over in T3 Code, and its worktree, " + g.hire.Worktree.storedOnly() + ", was moved or replaced since it was hired, " +
				"so it couldn't be handed back: its tmux desk is ended, and Smolcorp left the worktree alone, with no git run there. " +
				"Remove its project \"Rex · " + g.hire.Branch + "\" in T3 Code yourself, then delete what's at " + g.hire.Worktree.storedOnly() +
				" (only the link, if it's a link), then run git worktree prune in your own checkout."
			if w.Code != http.StatusOK || !strings.HasPrefix(note, words) {
				t.Errorf("the fire: %d %s\nwant the note to start %q", w.Code, w.Body, words)
			}
			if g.r.tmux.up || c.up && !g.r.tmux.ran("kill-server") {
				t.Errorf("its tmux desk wasn't ended: %q", g.asked())
			}
			r6Untouched(t, "the fire", gm, other, before, calls)
			real := filepath.Join(filepath.Dir(g.hire.Worktree.storedOnly())+".real", filepath.Base(g.hire.Worktree.storedOnly()))
			if _, err := os.Stat(filepath.Join(real, "sub", "a.txt")); err != nil {
				t.Errorf("the real tree was touched: %v", err)
			}
			if info, err := os.Lstat(filepath.Dir(g.hire.Worktree.storedOnly())); err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("the link was taken away: %v", err)
			}
			if branches(t, g.r.repo) == "" {
				t.Error("the hire's branch was deleted")
			}
			if got, _ := registryHire(g.hire.ID); got.State != "fired" || !strings.HasPrefix(got.Notes, words) {
				t.Errorf("its row: %+v", got)
			}
			data, err := os.ReadFile(filepath.Join(supportDir, "recaps", g.hire.ID+".md"))
			if err != nil || !strings.Contains(string(data), "- Worktree: "+g.hire.Worktree.storedOnly()+" (kept). It was moved or replaced, so Smolcorp left it alone. Don't run git in it.") {
				t.Errorf("the recap (%v):\n%s", err, data)
			}
		})
	}
}

// Hand back's resume with config.json's hireWorktrees set: the desk's tree, made in that folder and unmoved, is
// resumed in its sandbox, as the engine's start would start it. The control: that folder for hires turned into a
// link, the resume is refused with the start's words, nothing started.
func TestHandBackResumesATreeMadeInTheConfiguredWorktreesFolder(t *testing.T) {
	d, r, _ := engineDeck(t)
	deskSockets(t)
	folder := physical(t.TempDir())
	d.mu.Lock()
	d.config.HireWorktrees = folder
	d.mu.Unlock()
	starts := 0
	r.o.Sandbox = func(*deskSandbox) tmuxStarter {
		return func(args []string) (string, int, error) {
			starts++
			if err := makeDeskSocketFolder(args[1]); err != nil {
				return "", 0, errors.New(takenPlace)
			}
			staleSocketAt(t, args[1])
			r.tmux.up = true
			return "%1 4242 " + args[1], 5151, nil
		}
	}
	d.engine.Sandbox = r.o.Sandbox
	made := r.o
	made.Worktrees = folder // hired as the deck hires with hireWorktrees set (worktreesFolder)
	hire, err := runManagedHire(r.request("janitor"), made)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(hire.Worktree.storedOnly()) != physical(hiresFolder(hire.Repo, folder)) {
		t.Fatalf("its tree %q isn't in the configured folder %q", hire.Worktree, folder)
	}
	row := registered(t)[0]
	windowShows(r, pane(t, "idle-empty"), true)
	d.engine.Tmux = r.o.Tmux
	if err := d.t3StopInTmux(row); err != nil {
		t.Fatal(err)
	}

	resumed, err := d.t3ResumeInTmux(row, t3TakeOverSession, nil)
	if err != nil || resumed.Socket != deskSocketPath(hire.ID, 0) || starts != 2 {
		t.Fatalf("Hand back's resume: %+v %v, %d starts", resumed, err, starts)
	}

	// The control: stopped again, its folder for hires turned into a link, the resume is refused, nothing started.
	r.tmux.up = false
	if err := d.t3StopInTmux(row); err != nil {
		t.Fatal(err)
	}
	other, _ := r6Other(t, hire.Branch)
	r6Turn(t, hire.Worktree.storedOnly(), other)
	_, err = d.t3ResumeInTmux(row, t3TakeOverSession, nil)
	if err == nil || !strings.Contains(err.Error(), "was moved or replaced since it was hired, so it didn't start.") || starts != 2 {
		t.Errorf("the turned tree's resume: %v, %d starts", err, starts)
	}
}
