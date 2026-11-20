//go:build !windows

package desktop

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// FXCLAUDESUMOPEN's rows (RT126RESOLVESWEEP S6): a desk still at work once its hire is gone changes a file or folder
// the plant walk looked at, just before the walk opens it: a pipe or a link to an endless file in its place, or the
// file itself made endless. The walk runs in the registry's save, under frontDeskMu, so a walk that waits on the pipe
// or reads without end would hold every hook key, fire and start. Stand-ins only: the endless file is an empty sparse
// one, made 1 TiB long, and nothing ever writes to a pipe.

// claudeSwapLimit is how long a fire's save may take with a swap in its walk. It takes milliseconds.
const claudeSwapLimit = 10 * time.Second

// endless makes the file at at a sparse stand-in, 1 TiB long and empty: a read of it doesn't end within the test.
func endless(t *testing.T, at string) {
	if err := os.Truncate(at, 1<<40); err != nil {
		t.Errorf("this volume can't make a 1 TiB sparse file: %v", err)
	}
}

// pipeAt puts a pipe at at, in place of what was there.
func pipeAt(t *testing.T, at string) {
	if err := os.RemoveAll(at); err != nil {
		t.Error(err)
	}
	if err := syscall.Mkfifo(at, 0o600); err != nil {
		t.Error(err)
	}
}

// unpipe lets a walk waiting to open the pipe at at go: it opens it to write and closes it, so the walk reads its end.
func unpipe(at string) {
	if f, err := os.OpenFile(at, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
		f.Close()
	}
}

// fireWithASwap fires a working hire whose place has a .claude, after swap has changed what's in it, given the
// .claude's path, the moment the walk is about to open its settings.json for the first time. The fire's save must come
// back within claudeSwapLimit, with what the hire left held for you. unstick lets a walk that got stuck go, so the test
// can end.
func fireWithASwap(t *testing.T, swap func(claude string), unstick func(claude string)) {
	t.Helper()
	newTestDeck(t)
	freshClaudeHeld(t)
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), ".claude/settings.json", ".claude/skills/s/SKILL.md")
	h := Hire{ID: "abcdef000001", Name: "swapped", State: "working", Folder: keptPath(place)}
	claudeRegistry(t, h)
	claude := filepath.Join(place, ".claude")
	var swapped atomic.Bool
	was := claudeOpening
	claudeOpening = func(name string) {
		if name == filepath.Join(claude, "settings.json") && swapped.CompareAndSwap(false, true) {
			swap(claude)
		}
	}
	done := make(chan error, 1)
	go func() { done <- markFired(h.ID, "") }()
	select {
	case err := <-done:
		claudeOpening = was
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(claudeSwapLimit):
		unstick(claude)
		select {
		case <-done:
			claudeOpening = was
		case <-time.After(claudeSwapLimit): // still stuck: the hook stays, and the test binary ends with it
		}
		t.Fatalf("the fire's save was still held by the plant walk after %v", claudeSwapLimit)
	}
	if !swapped.Load() {
		t.Fatal("the walk never came to open settings.json")
	}
	if why := claudeHeldRefusal(place); !strings.Contains(why, claude) {
		t.Errorf("what the hire left isn't held: %q", why)
	}
}

// A file the walk looked at, made a pipe before the walk opens it: the walk doesn't wait for a writer.
func TestAFileSwappedForAPipeDoesntStallThePlantWalk(t *testing.T) {
	fireWithASwap(t, func(claude string) { pipeAt(t, filepath.Join(claude, "settings.json")) },
		func(claude string) { unpipe(filepath.Join(claude, "settings.json")) })
}

// A file the walk looked at, made a link to an endless file (as one to /dev/zero would be): the walk doesn't follow
// it.
func TestAFileSwappedForALinkToAnEndlessFileDoesntStallThePlantWalk(t *testing.T) {
	big := filepath.Join(claudeTempDir(t), "big")
	if err := os.WriteFile(big, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	endless(t, big)
	fireWithASwap(t, func(claude string) {
		at := filepath.Join(claude, "settings.json")
		if err := os.Remove(at); err != nil {
			t.Error(err)
		}
		if err := os.Symlink(big, at); err != nil {
			t.Error(err)
		}
	}, func(string) { os.Truncate(big, 0) })
}

// The file the walk looked at, itself made endless before the walk opens it: the walk reads no more than its size
// when it was looked at.
func TestAFileMadeEndlessDoesntStallThePlantWalk(t *testing.T) {
	fireWithASwap(t, func(claude string) { endless(t, filepath.Join(claude, "settings.json")) },
		func(claude string) { os.Truncate(filepath.Join(claude, "settings.json"), 0) })
}

// A folder the walk looked at (it comes after settings.json), made a pipe before the walk reads it: the walk doesn't
// wait for a writer.
func TestAFolderSwappedForAPipeDoesntStallThePlantWalk(t *testing.T) {
	fireWithASwap(t, func(claude string) { pipeAt(t, filepath.Join(claude, "skills")) },
		func(claude string) { unpipe(filepath.Join(claude, "skills")) })
}

// The folder above the file the walk is about to open, made a pipe: the open doesn't wait on it (an open through
// os.OpenRoot of that folder would).
func TestTheFolderAboveAFileSwappedForAPipeDoesntStallThePlantWalk(t *testing.T) {
	fireWithASwap(t, func(claude string) { pipeAt(t, claude) }, unpipe)
}

// A file already bigger than claudeSumMost when the walk looks at it isn't read: it can't be summed, so it never
// matches.
func TestAFileTooBigIsntReadByThePlantWalk(t *testing.T) {
	claude := claudeTree(t, filepath.Join(claudeTempDir(t), ".claude"), "settings.json")
	endless(t, filepath.Join(claude, "settings.json"))
	done := make(chan error, 1)
	go func() { done <- sumClaudeTree(io.Discard, claude) }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("a file of 1 TiB was summed")
		}
	case <-time.After(claudeSwapLimit):
		os.Truncate(filepath.Join(claude, "settings.json"), 0)
		<-done
		t.Fatalf("the walk was still reading after %v", claudeSwapLimit)
	}
}

// What the walk sums is what it summed before it opened files this way (filepath.WalkDir, a plain read of each file),
// byte for byte: a baseline on record from before still matches.
func TestThePlantSumIsAsBefore(t *testing.T) {
	claude := claudeTree(t, filepath.Join(claudeTempDir(t), ".claude"), "settings.json", "hooks/pre.sh", "skills/s/SKILL.md",
		"agents/", "notes.txt")
	claudeLink(t, "settings.json", filepath.Join(claude, "linked.json"))
	before := sha256.New()
	err := filepath.WalkDir(claude, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(claude, name)
		fmt.Fprintf(before, "%s\x00%v\x00", rel, d.Type())
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(name)
			fmt.Fprintf(before, "%s\x00", target)
			return err
		case d.Type().IsRegular():
			data, err := os.ReadFile(name)
			before.Write(data)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	now := sha256.New()
	if err := sumClaudeTree(now, claude); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(now.Sum(nil), before.Sum(nil)) {
		t.Error("the sum isn't what it was")
	}
}
