//go:build !windows

package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// FXCLAUDESUMOPENR3's rows (SIERRA's ruling on FXCLAUDESUMOPENR2's proposal A+B): the place is listed by name and kind
// only (A), and a hire's listing is no longer made under frontDeskMu or the Deck's lock (B): the save asks for it on
// record and lets the lock go, and one worker lists. While it isn't done, your Claude Code isn't started there; a start
// held so goes ahead by itself once it's done (SIERRA's first condition), and a crash before it's done leaves the place
// refused until it's listed again (her second). Stand-ins only: `{"hooks":"standin"}` and `#!/bin/sh`.

// holdOnOpen holds the plant walk the first time it's about to open the file at at, until the test lets it go (once
// it's let go, or at the end of the test): reached is closed once it's held there.
func holdOnOpen(t *testing.T, at string) (reached <-chan struct{}, letGo func()) {
	t.Helper()
	hold, held := make(chan struct{}), make(chan struct{})
	var once, release sync.Once
	was := claudeOpening
	claudeOpening = func(name string) {
		if name == at {
			once.Do(func() { close(held); <-hold })
		}
	}
	letGo = func() { release.Do(func() { close(hold) }) }
	t.Cleanup(func() { letGo(); claudeWorkers.Wait(); claudeOpening = was })
	return held, letGo
}

// holdListings holds the worker each time it's about to list a place, until the test lets it go (or at the end of
// the test): reached is told each desk held so.
func holdListings(t *testing.T) (reached <-chan string, letGo func()) {
	t.Helper()
	hold, at := make(chan struct{}), make(chan string, 16)
	var release sync.Once
	was := claudeListingStarts
	claudeListingStarts = func(desk string) {
		at <- desk
		<-hold
	}
	letGo = func() { release.Do(func() { close(hold) }) }
	t.Cleanup(func() { letGo(); claudeWorkers.Wait(); claudeListingStarts = was })
	return at, letGo
}

// checkingWithin has a gate, or a look at what's held, wait no more than wait for a listing before it says Smolcorp is
// still checking, for the rest of the test.
func checkingWithin(t *testing.T, wait time.Duration) {
	t.Helper()
	was := claudeCheckWait
	claudeCheckWait = wait
	t.Cleanup(func() { claudeCheckWait = was })
}

// B: a fire's save holds frontDeskMu for its record write only, however long its place takes to list. While the
// listing isn't done, the place is refused as still being checked (in plain words, naming the place, saying it
// clears by itself); the fire goes on once it's done, with what the hire left held for you.
func TestAFiresSaveHoldsTheFrontDeskLockOnlyForItsRecordWrite(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	checkingWithin(t, 100*time.Millisecond)
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), ".claude/settings.json")
	h := Hire{ID: "abcdef000031", Name: "slow", State: "working", Folder: place}
	claudeRegistry(t, h)
	reached, letGo := holdOnOpen(t, filepath.Join(place, ".claude", "settings.json"))
	done := make(chan error, 1)
	go func() { done <- markFired(h.ID, "") }()
	select {
	case <-reached:
	case <-time.After(claudeSwapLimit):
		t.Fatal("the place was never listed")
	}
	// The listing is held: the lock must be free.
	free := false
	for deadline := time.Now().Add(2 * time.Second); !free && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if free = frontDeskMu.TryLock(); free {
			frontDeskMu.Unlock()
		}
	}
	if !free {
		t.Error("the fire's save holds frontDeskMu while its place is listed")
	}
	why := claudeHeldRefusal(filepath.Join(place, "src"))
	if !strings.Contains(why, "still checking what a hire left in "+place) || !strings.Contains(why, "clears by itself") {
		t.Errorf("while the place is listed: %q, want it refused as still being checked, naming the place", why)
	}
	select {
	case <-done:
		t.Error("the fire went on before what its hire left was listed")
	default:
	}
	letGo()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(claudeSwapLimit):
		t.Fatalf("the fire didn't go on within %v of its listing", claudeSwapLimit)
	}
	if why := claudeHeldRefusal(place); !strings.Contains(why, filepath.Join(place, ".claude")) {
		t.Errorf("once listed: %q, want what the hire left held", why)
	}
}

// SIERRA's first condition: a start of your Claude Code refused while its folder is being checked says so plainly,
// and goes ahead by itself once the listing is done, with no second request. One the listing then finds a plant
// around is refused, still by itself.
func TestAStartHeldWhileItsFolderIsCheckedGoesAheadByItself(t *testing.T) {
	for _, c := range []struct {
		name    string
		planted bool
	}{{"clean", false}, {"planted", true}} {
		t.Run(c.name, func(t *testing.T) {
			d, h, folder := hiringDeck(t, "")
			freshClaudeHeld(t)
			checkingWithin(t, 200*time.Millisecond)
			d.config.HireAgent = "claude"
			// A desk at work in a worktree in the folder: the gate lists it. What's in it is as in its checkout.
			checkout := claudeTree(t, filepath.Join(claudeTempDir(t), "checkout"), ".claude/settings.json")
			wt := claudeTree(t, filepath.Join(folder, "wt"), ".claude/settings.json")
			if c.planted {
				claudeTree(t, wt, ".claude/hooks/x.sh")
			}
			claudeRegistry(t, Hire{ID: "abcdef000032", Name: "Ada", Folder: checkout, Repo: checkout, Worktree: wt, State: "working"})
			reached, letGo := holdOnOpen(t, filepath.Join(wt, ".claude", "settings.json"))
			type answer struct {
				status  int
				message string
			}
			answered := make(chan answer, 1)
			go func() {
				status, message := d.Perform("hire", "", url.Values{})
				answered <- answer{status, message}
			}()
			select {
			case <-reached:
			case <-time.After(claudeSwapLimit):
				t.Fatal("the gate never listed the desk's worktree")
			}
			var first answer
			select {
			case first = <-answered:
			case <-time.After(2 * time.Second):
				letGo()
				first = <-answered
				t.Errorf("the start waited for the listing: no answer within 2s (then %d %q)", first.status, first.message)
			}
			if first.status != http.StatusConflict || !strings.Contains(first.message, "still checking what a hire left in "+wt) ||
				!strings.Contains(first.message, "your Claude Code in "+folder) || !strings.Contains(first.message, "starts there by itself") {
				t.Errorf("while the listing runs: %d %q, want it held, naming the place, and saying it starts by itself", first.status, first.message)
			}
			windows := func() int {
				d.mu.Lock()
				defer d.mu.Unlock()
				return len(h.windows)
			}
			if n := windows(); n != 0 {
				t.Errorf("%d windows opened while the listing ran", n)
			}
			letGo()
			if !c.planted {
				for deadline := time.Now().Add(claudeSwapLimit); windows() == 0 && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
				}
			}
			claudeStartsHeld.Wait()
			d.mu.Lock()
			opened, dirs := len(h.windows), h.dirs
			d.mu.Unlock()
			if c.planted {
				if opened != 0 {
					t.Errorf("a plant around the folder, and your Claude Code started there: %d windows", opened)
				}
				if why := claudeHeldRefusal(folder); !strings.Contains(why, filepath.Join(wt, ".claude", "hooks")) {
					t.Errorf("once listed: %q, want the hook held", why)
				}
				return
			}
			if opened != 1 || dirs[0] != folder {
				t.Errorf("once the listing was done: %d windows in %q, want your Claude Code started in %s by itself", opened, dirs, folder)
			}
		})
	}
}

// SIERRA's second condition: a crash between a listing's "pending" and its "done" leaves the place refused, never
// trusted, until it's listed again: the mark is on record. The next start lists it, and only then is it allowed, if
// it's clean.
func TestAListingCutShortByACrashLeavesThePlaceRefusedUntilItsListedAgain(t *testing.T) {
	for _, c := range []struct {
		name    string
		planted bool
	}{{"clean", false}, {"planted", true}} {
		t.Run(c.name, func(t *testing.T) {
			newTestDeck(t)
			freshClaudeHeld(t)
			checkingWithin(t, 100*time.Millisecond)
			files := []string{"src/"}
			if c.planted {
				files = append(files, "src/.claude/hooks/x.sh")
			}
			h := Hire{ID: "abcdef000033", Name: "Ada", Folder: claudeTree(t, filepath.Join(claudeTempDir(t), "project"), files...), State: "working"}
			claudeRegistry(t, h)
			reached, letGo := holdListings(t)
			// The GM's edit makes it gone: its save asks for the listing, on record, and doesn't wait for it.
			if _, refused := editHire(registryPath(), []byte(`{"id":"`+h.ID+`","state":"fired"}`), noDeskSession); refused != nil {
				t.Fatal(refused.message)
			}
			select {
			case <-reached:
			case <-time.After(2 * time.Second):
				t.Error("the save didn't leave the listing to the worker")
			}
			// The core stops there, before it's done, and starts again: it reads its record afresh.
			claudeRestart()
			startDeck(t)
			select {
			case <-reached: // the new start's worker lists it again (held here too)
			case <-time.After(2 * time.Second):
				t.Error("the new start didn't list the place again")
			}
			if why := claudeHeldRefusal(filepath.Join(h.Folder, "src")); !strings.Contains(why, "still checking what a hire left in "+h.Folder) {
				t.Errorf("after the crash: %q, want the place refused as still being checked", why)
			}
			if claudeListedNow(h.ID) {
				t.Error("after the crash, the hire is on record as listed")
			}
			letGo()
			why := claudeHeldRefusal(filepath.Join(h.Folder, "src"))
			switch {
			case c.planted && !strings.Contains(why, filepath.Join("src", ".claude")):
				t.Errorf("listed again: %q, want the hook held", why)
			case !c.planted && why != "":
				t.Errorf("listed again, clean: %q, want your Claude Code allowed", why)
			}
		})
	}
}

// A: the place is listed by name and kind, without a look at each name: a folder whose names can be read but not
// looked at (no search permission) is passed over like any other with no plant in it, where a look at each name
// held it as unread. What can't be looked at still holds: a .claude in such a folder, or a folder in it, is held as
// unread.
func TestThePlaceIsListedByNameAndKindWithoutALookAtEachName(t *testing.T) {
	freshClaudeHeld(t)
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), "shut/notes.txt", "shut/more.md", "plant/.claude/settings.json",
		"deeper/sub/notes.txt")
	for _, folder := range []string{"shut", "plant", "deeper"} {
		at := filepath.Join(place, folder)
		if err := os.Chmod(at, 0o400); err != nil { // read, no search
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(at, 0o700) })
	}
	if _, err := os.Lstat(filepath.Join(place, "shut", "notes.txt")); err == nil {
		t.Skip("this account looks into a folder without search permission (root?)")
	}
	listed := listedAs(t, place, holdClaudePlants("h1", placeLook(place), ""))
	sameListing(t, "place", listed, map[string]string{"plant": whyUnread, "deeper/sub": whyUnread})
}
