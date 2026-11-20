//go:build darwin

package desktop

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// FXGMHIRENOGIT126 round 7: Take over never asked whether the hire's worktree was moved. After a GM swapped the owner's
// hire's tree for a link to a folder of its own whose .git names the hire's registration, Take over's own tree check
// passed and the core's git ran in the GM's folder (RT126GMHIRENOGIT126R3, B1). Take over, Hand back (and so Take
// over's undo) now check the tree as a start does, first; a T3 desk's let go keeps the fence of a moved tree. The
// GM-made folder, the link and the rename are the test's own, in its temp folders. Nothing reaches a real T3 Code: t3
// is a stand-in file that never runs, and every call goes to the test's own ways.

// r7OwnersHire is the owner's hire into ~/code/app with a worktree of its own, in the full flow with a GM open
// (gmFolderDeck): its row, and the owner's checkout.
func r7OwnersHire(t *testing.T) (Hire, string) {
	t.Helper()
	app := filepath.Join(homeDir, "code", "app")
	ownersApp(t, app)
	place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: physical(app), Worktree: true},
		hirePlaceOptions{PortFree: func(int) bool { return true }})
	if err != nil || place.Worktree == "" {
		t.Fatalf("the owner's hire: %+v %v", place, err)
	}
	return Hire{ID: "abcdef123456", Name: "Ada", Role: "janitor", Folder: keptPath(place.Folder), Repo: place.Repo, Worktree: keptPath(place.Worktree),
		Branch: place.Branch, State: "working"}, physical(app)
}

// r7SwapTree swaps hire h's tree for a link to fake, a folder in GM gm's plans whose .git names the hire's own
// registration in app, holding the GM's a.txt, and with fence a file of the GM's at the fence's name. The real tree is
// put back at the end.
func r7SwapTree(t *testing.T, h Hire, app string, gm Hire, fence bool) (fake string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(app, ".git", "worktrees"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("registrations: %v %v", entries, err)
	}
	fake = filepath.Join(gmPlansOf(gm), "fake")
	writeFile(t, filepath.Join(fake, ".git"), "gitdir: "+filepath.Join(app, ".git", "worktrees", entries[0].Name())+"\n")
	writeFile(t, filepath.Join(fake, "a.txt"), "the GM's\n")
	if fence {
		writeFile(t, filepath.Join(fake, filepath.FromSlash(t3FenceName)), `{"the GM's":true}`)
	}
	aside := h.Worktree.storedOnly() + ".real"
	if err := os.Rename(h.Worktree.storedOnly(), aside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(fake, h.Worktree.storedOnly()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(h.Worktree.storedOnly()); _ = os.Rename(aside, h.Worktree.storedOnly()) })
	return fake
}

// r7Ways are Take over's and Hand back's ways with every step logged, nothing real behind them: t3 a stand-in file
// that never runs (Run logs it), T3 Code's calls refused. With looks, the process look finds nothing and the stop
// goes ahead, so Take over would go on as far as it can; without, the look can't be made and the stop is refused.
func r7Ways(t *testing.T, log *[]string, looks bool) t3Handing {
	t.Helper()
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".local", "bin", "t3"), "#!/bin/sh\nexit 1\n")
	if err := os.Chmod(filepath.Join(home, ".local", "bin", "t3"), 0o755); err != nil {
		t.Fatal(err)
	}
	note := func(what string) { *log = append(*log, what) }
	return t3Handing{
		Machine: t3Machine{Home: home, GOOS: "darwin", Getenv: func(string) string { return "" }, App: func() string { return "" },
			OpenApp: func(string) { note("open app") },
			Run: func(program string, args []string, _ time.Duration) t3Ran {
				note("t3 " + strings.Join(args, " "))
				return t3Ran{}
			},
			Feed:    func(string, []string, []byte, time.Duration) t3Ran { note("t3 feed"); return t3Ran{} },
			OpenURL: func(string) error { note("open url"); return nil }, Pause: func(time.Duration) {}, Now: time.Now},
		Call: func(_, method string, _ map[string]any) (map[string]any, error) {
			note("call " + method)
			return nil, errors.New("no T3 Code here")
		},
		Holding: func(folder, session string) ([]int, bool) { note("look"); return nil, looks },
		Name:    func(int) string { return "" },
		Stat:    os.Stat, Pause: func(time.Duration) {}, Now: time.Now,
		Fence: func() (map[string]any, error) { note("fence"); return map[string]any{}, nil },
		Stop: func(Hire, string) error {
			note("stop")
			if looks {
				return nil
			}
			return errors.New("not stopped here")
		},
		Resume: func(Hire, string) (t3Resumed, error) {
			note("resume")
			return t3Resumed{}, errors.New("not resumed here")
		},
		Save: func(string, func(*Hire)) error { note("save"); return nil },
	}
}

// B1: Take over of a hire whose tree a GM swapped for a link to its own folder is refused first: no git in that folder,
// no fence written there, no T3 call, the desk not stopped. The control, the same hire with its tree as hired, goes on
// as now: past the check, the hooks look runs git in the tree, then the process look.
func TestTakeOverOfAMovedTreeIsRefusedBeforeAnythingAtItsPath(t *testing.T) {
	_, l, _, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	h, app := r7OwnersHire(t)
	session := t3Session{ID: t3TakeOverSession}

	t.Run("its tree as hired goes on", func(t *testing.T) {
		calls := gitCalls(t)
		var log []string
		_, _, err := t3TakeOverSaid(h, session, r7Ways(t, &log, false))
		if refused := refusalOf(t, err); refused.Message != t3CantLook("Ada") || !slices.Equal(log, []string{"look"}) {
			t.Errorf("%+v %q", refused, log)
		}
		if len(calls(h.Worktree.storedOnly())) == 0 {
			t.Error("the hooks look ran no git in the tree: the control didn't get past the check")
		}
	})

	t.Run("its tree swapped for a link", func(t *testing.T) {
		fake := r7SwapTree(t, h, app, l.gm, false)
		calls := gitCalls(t)
		var log []string
		_, _, err := t3TakeOverSaid(h, session, r7Ways(t, &log, true))
		want := "Ada's worktree, " + shownPath(h.Worktree.storedOnly()) + ", was moved or replaced since it was hired, so it wasn't taken over, " +
			"and Take over changed nothing. Fire it, then hire it again."
		if refused := refusalOf(t, err); refused.Status != http.StatusConflict || refused.Message != want {
			t.Errorf("%+v", refused)
		}
		if len(log) != 0 {
			t.Errorf("Take over went on: %q", log)
		}
		if ran := calls(fake, h.Worktree.storedOnly()); len(ran) != 0 {
			t.Errorf("git ran in the GM's folder: %q", ran)
		}
		if exists(filepath.Join(fake, ".claude")) {
			t.Error("a fence went into the GM's folder")
		}
		if data, _ := os.ReadFile(filepath.Join(app, ".git", "info", "exclude")); strings.Contains(string(data), t3FenceName) {
			t.Error("the fence went into the repo's exclude")
		}
	})
}

// Hand back (and so Take over's undo, giveBack) checks the tree first too: a moved tree's fence isn't taken off by its
// path, T3 Code isn't asked, the session isn't resumed. A T3 desk's let go ends T3 Code's project as before, but keeps
// what is at the fence's name of a moved tree.
func TestHandBackAndLetGoOfAMovedTreeLeaveItsPathAlone(t *testing.T) {
	_, l, _, fifos := gmFolderDeck(t)
	for _, f := range fifos {
		f.release()
	}
	h, app := r7OwnersHire(t)
	h.Engine, h.T3Session, h.T3Thread = t3Engine, t3TakeOverSession, t3ImportedThread+t3TakeOverSession
	fenceAt := func(tree string) string { return filepath.Join(tree, filepath.FromSlash(t3FenceName)) }
	leaving := func(log *[]string) t3Leaving {
		ways := r7Ways(t, log, false)
		ways.Machine.Run = func(_ string, args []string, _ time.Duration) t3Ran {
			*log = append(*log, "t3 "+strings.Join(args, " "))
			return t3Ran{Ended: true}
		}
		return t3Leaving{Machine: ways.Machine, Working: func(string) ([]int, bool) { return nil, true }, Name: ways.Name,
			Pause: ways.Pause, Now: ways.Now}
	}

	t.Run("its tree as hired: let go takes the fence off", func(t *testing.T) {
		writeFile(t, fenceAt(h.Worktree.storedOnly()), "{}")
		var log []string
		gone, err := t3Leave(h, leaving(&log))
		if err != nil || !gone || exists(fenceAt(h.Worktree.storedOnly())) || !slices.Equal(log, []string{"t3 project remove " + h.Worktree.storedOnly() + " --force"}) {
			t.Errorf("%v %v %q", gone, err, log)
		}
	})

	fake := r7SwapTree(t, h, app, l.gm, true)
	t.Run("hand back", func(t *testing.T) {
		var log []string
		_, err := t3HandBack(h, r7Ways(t, &log, false))
		want := "Ada's worktree, " + shownPath(h.Worktree.storedOnly()) + ", was moved or replaced since it was hired, so it wasn't handed back, " +
			"and Hand back changed nothing. If T3 Code has a project \"Ada · " + h.Branch + "\" for it, remove it there yourself."
		if refused := refusalOf(t, err); refused.Status != http.StatusConflict || refused.Message != want || len(log) != 0 {
			t.Errorf("%+v %q", refused, log)
		}
		if data, _ := os.ReadFile(fenceAt(fake)); string(data) != `{"the GM's":true}` {
			t.Errorf("the file at the fence's name in the GM's folder: %q", data)
		}
	})
	t.Run("a T3 desk's let go", func(t *testing.T) {
		var log []string
		gone, err := t3Leave(h, leaving(&log))
		if err != nil || gone || !slices.Equal(log, []string{"t3 project remove " + h.Worktree.storedOnly() + " --force"}) {
			t.Errorf("%v %v %q", gone, err, log)
		}
		if data, _ := os.ReadFile(fenceAt(fake)); string(data) != `{"the GM's":true}` {
			t.Errorf("the file at the fence's name in the GM's folder: %q", data)
		}
	})
}
