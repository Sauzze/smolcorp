//go:build linux || darwin

package sandbox

// RT126KEPTFD, the cold review's swaps, as rows (stand-ins only: every place is a t.TempDir(), the owner's place too).
// Each swap lands between the core's check and what it does next, at the seam (keptBetween) or racing in a goroutine
// with the desk's own primitive (swapNames). The stand-in owner's place is never moved, removed, written nor shown, and
// the core always comes back: a pipe, a socket, a device or a link where a folder was is refused at once, never opened
// or waited on (B1).

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const ownText = "OWNERTEXT the owner's own\n"

// ownersPlace is a stand-in owner's place: home/.bashrc, home/c/f, home/c/other.
func ownersPlace(t *testing.T) string {
	home, _ := filepath.EvalSymlinks(t.TempDir())
	writeKeptFile(t, home+"/.bashrc", ownText)
	writeKeptFile(t, home+"/c/f", ownText)
	writeKeptFile(t, home+"/c/other", ownText)
	return home
}

// placeState is everything in home: each path, its kind, its device and inode, its content.
func placeState(home string) string {
	var b strings.Builder
	filepath.Walk(home, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(&b, "%s err %v\n", p, err)
			return nil
		}
		dev, ino, _ := fileID(info)
		fmt.Fprintf(&b, "%s %v %d/%d", p, info.Mode(), dev, ino)
		if info.Mode().IsRegular() {
			data, _ := os.ReadFile(p)
			fmt.Fprintf(&b, " %q", data)
		}
		b.WriteString("\n")
		return nil
	})
	return b.String()
}

// ownersUntouched fails the row if home isn't as it was, or the words show any of it.
func ownersUntouched(t *testing.T, home, before string, changes []KeptChange) {
	t.Helper()
	if now := placeState(home); now != before {
		t.Errorf("the stand-in owner's place changed:\nbefore:\n%s\nafter:\n%s", before, now)
	}
	if w := KeptWords(changes); strings.Contains(w, "OWNERTEXT") || strings.Contains(w, "c/other") {
		t.Errorf("the stand-in owner's place shown: %s", w)
	}
}

// atSeam does do once, when the core reaches step (keptBetween).
func atSeam(t *testing.T, step string, do func()) {
	done := false
	keptBetween = func(s, _ string) {
		if s == step && !done {
			done = true
			do()
		}
	}
	t.Cleanup(func() { keptBetween = func(string, string) {} })
}

func mustDo(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// letGo opens each pipe at (or that a link at leads to) to write, without waiting: whatever waits to read it goes on.
func letGo(at ...string) {
	for _, p := range at {
		if w, err := os.OpenFile(p, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			w.Close()
		}
	}
}

// restoreNoWait is RestoreKept, failing the row if it doesn't come back within 20 s (a pipe waited on: its stack is
// quoted); unblock then lets it go.
func restoreNoWait(t *testing.T, held string, unblock func()) ([]KeptChange, error) {
	t.Helper()
	type res struct {
		ch  []KeptChange
		err error
	}
	got := make(chan res, 1)
	go func() { ch, err := RestoreKept(held); got <- res{ch, err} }()
	select {
	case r := <-got:
		return r.ch, r.err
	case <-time.After(20 * time.Second):
		buf := make([]byte, 1<<16)
		t.Errorf("RestoreKept waited past 20s:\n%s", waitingIn(string(buf[:runtime.Stack(buf, true)])))
		if unblock != nil {
			unblock()
		}
		r := <-got
		return r.ch, r.err
	}
}

// waitingIn is the goroutine under RestoreKept waiting in the kernel, its frames in os and this package only.
func waitingIn(all string) string {
	for _, g := range strings.Split(all, "\n\n") {
		if strings.Contains(g, "RestoreKept") && strings.Contains(g, "[syscall") {
			var keep []string
			for _, line := range strings.Split(g, "\n") {
				if strings.HasPrefix(line, "goroutine") || strings.HasPrefix(line, "syscall.") || strings.HasPrefix(line, "os.") ||
					strings.Contains(line, "sandbox.") && !strings.Contains(line, "_test") {
					keep = append(keep, line)
				}
			}
			return strings.Join(keep, "\n")
		}
	}
	return "(no goroutine found in a syscall)"
}

// swapOverAndOver swaps a and b, as the desk can, over and over until stop is called.
func swapOverAndOver(a, b string) (stop func()) {
	quit, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-quit:
				return
			default:
				swapNames(a, b)
			}
		}
	}()
	once := false
	return func() {
		if !once {
			once = true
			close(quit)
			<-done
		}
	}
}

// R1: a folder two deep (.claude/a/b) becomes a link to the owner's place between the check and the put back.
func TestAFolderTwoDeepSwappedAtThePutBackNeverReachesTheOwnersPlace(t *testing.T) {
	for _, disk := range []string{"one disk", "another disk"} {
		t.Run(disk, func(t *testing.T) {
			pl, home := keptDesk(t), ownersPlace(t)
			b := pl.Root + "/.claude/a/b"
			writeKeptFile(t, b+"/c/f", "the desk's, kept\n")
			mustDo(t, SaveKept(pl))
			writeKeptFile(t, b+"/c/f", "the desk's, changed\n")
			guardStops(t, pl)
			if disk == "another disk" {
				onAnotherDisk(t)
			}
			before := placeState(home)
			atSeam(t, "put back", func() { mustDo(t, os.Rename(b, b+".away")); mustDo(t, os.Symlink(home, b)) })
			changes, _ := restoreNoWait(t, KeptFolder(pl.Support, "h1"), nil)
			ownersUntouched(t, home, before, changes)
			if got := readKeptFile(t, b+".away/c/f"); got != "the desk's, kept\n" { // the folder checked and held
				t.Errorf("not put back in the folder checked: %q", got)
			}
		})
	}
}

// R2: the place itself, a file, becomes a link to the owner's file at the put back, and before the words.
func TestAFileSwappedForALinkNeverReachesTheOwnersFile(t *testing.T) {
	for _, step := range []string{"put back", "show"} {
		for _, disk := range []string{"one disk", "another disk"} {
			t.Run(step+", "+disk, func(t *testing.T) {
				pl, home := keptDesk(t), ownersPlace(t)
				f := pl.Root + "/.claude/x/f"
				writeKeptFile(t, f, "the desk's, kept\n")
				mustDo(t, SaveKept(pl))
				writeKeptFile(t, f, "the desk's, changed\n")
				guardStops(t, pl)
				if disk == "another disk" {
					onAnotherDisk(t)
				}
				before := placeState(home)
				atSeam(t, step, func() { mustDo(t, os.Remove(f)); mustDo(t, os.Symlink(home+"/.bashrc", f)) })
				changes, _ := restoreNoWait(t, KeptFolder(pl.Support, "h1"), nil)
				ownersUntouched(t, home, before, changes)
			})
		}
	}
}

// R3: the desk's own primitive, racing: a folder in the middle (.claude/a/b) and an added folder (.claude/added), each
// swapped with a link to the owner's place over and over while the guard stops the desk and the core puts back.
func TestAMiddleAndAnAddedFolderSwappedRacingNeverReachTheOwnersPlace(t *testing.T) {
	for _, disk := range []string{"one disk", "another disk"} {
		t.Run(disk, func(t *testing.T) {
			pl, home := keptDesk(t), ownersPlace(t)
			b, added := pl.Root+"/.claude/a/b", pl.Root+"/.claude/added"
			lb, la := pl.Root+"/.claude/a/lnk", pl.Root+"/.claude/lnk"
			writeKeptFile(t, b+"/c/f", "the desk's, kept\n")
			mustDo(t, os.Symlink(home, lb))
			mustDo(t, os.Symlink(home, la))
			mustDo(t, SaveKept(pl))
			if disk == "another disk" {
				onAnotherDisk(t)
			}
			before := placeState(home)
			for try := range 120 {
				writeKeptFile(t, b+"/c/f", fmt.Sprintf("the desk's, changed %d\n", try))
				os.RemoveAll(added)
				writeKeptFile(t, added+"/the-desks.txt", "the desk's\n")
				guardStops(t, pl)
				stop1, stop2 := swapOverAndOver(b, lb), swapOverAndOver(added, la)
				changes, _ := restoreNoWait(t, KeptFolder(pl.Support, "h1"), nil)
				stop1()
				stop2()
				for _, p := range [][2]string{{b, lb}, {added, la}} {
					if st, err := os.Lstat(p[0]); err == nil && st.Mode()&fs.ModeSymlink != 0 {
						mustDo(t, swapNames(p[0], p[1]))
					}
				}
				if ownersUntouched(t, home, before, changes); t.Failed() {
					t.Fatalf("at try %d", try)
				}
			}
		})
	}
}

// R4: a pipe. (a) The place becomes a pipe at the put back; (b) a changed file becomes a pipe before the words; (c) a
// file and a pipe swap names racing the look and the put back. The core never waits on it.
func TestAPipeAtThePutBackOrTheWordsIsNeverWaitedOn(t *testing.T) {
	for _, disk := range []string{"one disk", "another disk"} {
		t.Run("at the put back, "+disk, func(t *testing.T) {
			pl, home := keptDesk(t), ownersPlace(t)
			f := pl.Root + "/.claude/x/f"
			writeKeptFile(t, f, "the desk's, kept\n")
			mustDo(t, SaveKept(pl))
			writeKeptFile(t, f, "the desk's, changed\n")
			guardStops(t, pl)
			if disk == "another disk" {
				onAnotherDisk(t)
			}
			before := placeState(home)
			atSeam(t, "put back", func() { mustDo(t, os.Remove(f)); mustDo(t, syscall.Mkfifo(f, 0o600)) })
			changes, _ := restoreNoWait(t, KeptFolder(pl.Support, "h1"), func() { letGo(f) })
			ownersUntouched(t, home, before, changes)
		})
	}
	t.Run("before the words", func(t *testing.T) {
		pl, home := keptDesk(t), ownersPlace(t)
		f := pl.Root + "/.claude/x/f"
		writeKeptFile(t, f, "the desk's, kept\n")
		mustDo(t, SaveKept(pl))
		writeKeptFile(t, f, "the desk's, changed\n")
		before := placeState(home)
		atSeam(t, "show", func() { mustDo(t, os.Remove(f)); mustDo(t, syscall.Mkfifo(f, 0o600)) })
		changes, _ := restoreNoWait(t, KeptFolder(pl.Support, "h1"), func() { letGo(f) })
		ownersUntouched(t, home, before, changes)
	})
	t.Run("a file and a pipe racing", func(t *testing.T) {
		pl := keptDesk(t)
		f, p := pl.Root+"/.claude/x/f", pl.Root+"/.claude/x/p"
		writeKeptFile(t, f, "the desk's, kept\n")
		mustDo(t, syscall.Mkfifo(p, 0o600))
		mustDo(t, SaveKept(pl))
		writeKeptFile(t, f, "the desk's, changed\n")
		for try := range 200 {
			guardStops(t, pl)
			stop := swapOverAndOver(f, p)
			_, err := restoreNoWait(t, KeptFolder(pl.Support, "h1"), func() { stop(); letGo(f, p) })
			if stop(); t.Failed() {
				t.Fatalf("try %d: %v", try, err)
			}
		}
	})
}

// aPipeAndAFolderSwapping races a folder of the desk's (.claude<in>/d) and a pipe swapping names against RestoreKept,
// 400 times; changed adds a file beside them first, so their folder is a change, and its seal walks them too.
func aPipeAndAFolderSwapping(t *testing.T, in string, changed bool) {
	pl := keptDesk(t)
	x := pl.Root + "/.claude" + in
	d, p := x+"/d", x+"/p"
	writeKeptFile(t, d+"/in", "the desk's\n")
	mustDo(t, syscall.Mkfifo(p, 0o600))
	mustDo(t, SaveKept(pl))
	if changed {
		writeKeptFile(t, x+"/added", "a change\n")
	}
	for try := range 400 {
		stop := swapOverAndOver(d, p)
		_, err := restoreNoWait(t, KeptFolder(pl.Support, "h1"), func() { stop(); letGo(d, p) })
		if stop(); t.Failed() {
			t.Fatalf("try %d: %v", try, err)
		}
	}
}

// R5: nothing changed at all; a folder of the desk's and a pipe swap names while the core looks (the look a fire's
// first deskKept, a Take over, a Hand back and the next start all make, the session still up). It always comes back.
func TestAPipeSwappedForAFolderNeverHoldsTheLook(t *testing.T) { aPipeAndAFolderSwapping(t, "", false) }

// R6: the same in a changed folder, whose seal walks the pair too.
func TestAPipeSwappedForAFolderInAChangedOneNeverHoldsTheLookOrItsSeal(t *testing.T) {
	aPipeAndAFolderSwapping(t, "/x", true)
}

// R7: a hard link to an owner's file (one disk) in the desk's place: the place is put back, the owner's file neither
// moved, removed nor written. (Its text is shown: the desk's entry is that file. A Linux desk can only link a file it
// can read already, within one mount.)
func TestAHardLinkToAnOwnersFileIsPutBackWithoutTouchingIt(t *testing.T) {
	for _, disk := range []string{"one disk", "another disk"} {
		t.Run(disk, func(t *testing.T) {
			pl, home := keptDesk(t), ownersPlace(t)
			f := pl.Root + "/.claude/x/f"
			writeKeptFile(t, f, "the desk's, kept\n")
			mustDo(t, SaveKept(pl))
			mustDo(t, os.Remove(f))
			mustDo(t, os.Link(home+"/.bashrc", f))
			guardStops(t, pl)
			if disk == "another disk" {
				onAnotherDisk(t)
			}
			st0, _ := os.Lstat(home + "/.bashrc")
			_, err := restoreNoWait(t, KeptFolder(pl.Support, "h1"), nil)
			st1, err1 := os.Lstat(home + "/.bashrc")
			data, _ := os.ReadFile(home + "/.bashrc")
			if err1 != nil || !os.SameFile(st0, st1) || string(data) != ownText {
				t.Errorf("the owner's file was moved, removed or written: %v %q", err1, data)
			}
			if got := readKeptFile(t, f); got != "the desk's, kept\n" {
				t.Errorf("not put back: %q (%v)", got, err)
			}
		})
	}
}

// socketAt makes a socket at at, bound from its own folder (so a long temp path is no matter), its fd closed.
func socketAt(t *testing.T, at string) {
	t.Chdir(path.Dir(at))
	fd, err := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	mustDo(t, err)
	defer unix.Close(fd)
	mustDo(t, unix.Bind(fd, &unix.SockaddrUnix{Name: path.Base(at)}))
}

// noWait is do's error, failing the row if do waits past 5 s (on a pipe at at, or at.p), which it then lets go.
func noWait(t *testing.T, at string, do func() error) error {
	t.Helper()
	got := make(chan error, 1)
	go func() { got <- do() }()
	select {
	case err := <-got:
		return err
	case <-time.After(5 * time.Second):
		t.Errorf("waited past 5s on what is at %s", at)
		letGo(at, at+".p")
		return <-got
	}
}

// B1, by kind: between the core's Lstat of a folder and its open, the desk puts a pipe, a socket or a link to a pipe
// in its place. Each folder open of the kept code refuses it at once with the kernel's ENOTDIR, never opening it and
// never waiting on it: in a folder held open (keptSub: the look, the words, the seal, the put back, the set aside), and
// by its path (openKeptRoot: the folder above a kept place, the found folder). A link to a folder is refused by its
// fd. A device stands in from /dev: only root can make one.
func TestNoFolderOpenOpensOrWaitsOnWhatWasSwappedForAFolder(t *testing.T) {
	inFolder := func(dir, name string, st fs.FileInfo) error {
		root, err := os.OpenRoot(dir)
		mustDo(t, err)
		defer root.Close()
		sub, err := keptSub(root, name, st)
		if err == nil {
			sub.Close()
		}
		return err
	}
	byPath := func(at string) error {
		root, err := openKeptRoot(at)
		if err == nil {
			root.Close()
		}
		return err
	}
	kinds := []struct {
		kind string
		make func(t *testing.T, at string)
	}{
		{"a pipe", func(t *testing.T, at string) { mustDo(t, syscall.Mkfifo(at, 0o600)) }},
		{"a socket", socketAt},
		{"a link to a pipe", func(t *testing.T, at string) {
			mustDo(t, syscall.Mkfifo(at+".p", 0o600))
			mustDo(t, os.Symlink(path.Base(at)+".p", at))
		}},
	}
	for _, k := range kinds {
		for _, by := range []string{"in its folder", "by its path"} {
			t.Run(k.kind+", "+by, func(t *testing.T) {
				base, _ := filepath.EvalSymlinks(t.TempDir())
				at := base + "/d"
				mustDo(t, os.Mkdir(at, 0o700))
				st, err := os.Lstat(at)
				mustDo(t, err)
				mustDo(t, os.Remove(at))
				k.make(t, at)
				err = noWait(t, at, func() error {
					if by == "by its path" {
						return byPath(at)
					}
					return inFolder(base, "d", st)
				})
				if !errors.Is(err, syscall.ENOTDIR) {
					t.Errorf("%s was opened, or not refused as no folder: %v", k.kind, err)
				}
			})
		}
	}
	t.Run("a device", func(t *testing.T) {
		st, err := os.Lstat(t.TempDir())
		mustDo(t, err)
		for what, err := range map[string]error{"in its folder": inFolder("/dev", "null", st), "by its path": byPath("/dev/null")} {
			if !errors.Is(err, syscall.ENOTDIR) {
				t.Errorf("%s: /dev/null was opened, or not refused as no folder: %v", what, err)
			}
		}
	})
	t.Run("a link to a folder, in its folder", func(t *testing.T) {
		base, _ := filepath.EvalSymlinks(t.TempDir())
		mustDo(t, os.Mkdir(base+"/d", 0o700))
		mustDo(t, os.Mkdir(base+"/e", 0o700))
		st, err := os.Lstat(base + "/d")
		mustDo(t, err)
		mustDo(t, os.Remove(base+"/d"))
		mustDo(t, os.Symlink("e", base+"/d"))
		if err := inFolder(base, "d", st); err == nil || !strings.Contains(err.Error(), "was replaced") {
			t.Errorf("a link to another folder was opened as the folder: %v", err)
		}
	})
}

// N1: a change below a folder the desk swapped for a link to the owner's place, between the look and the seal, is
// sealed as not there: the seal walks from the folders the copy saved, by their fds, and never reads the owner's place.
func TestASealNeverWalksThroughAFolderSwappedInTheMiddle(t *testing.T) {
	pl, home := keptDesk(t), ownersPlace(t)
	a := pl.Root + "/.claude/a"
	writeKeptFile(t, a+"/c/f", "the desk's, kept\n")
	mustDo(t, SaveKept(pl))
	writeKeptFile(t, a+"/c/f", "the desk's, changed\n")
	atSeam(t, "show", func() { mustDo(t, os.Rename(a, a+".away")); mustDo(t, os.Symlink(home, a)) })
	changes, err := CheckKept(KeptFolder(pl.Support, "h1"))
	mustDo(t, err)
	if len(changes) != 1 || changes[0].At != a+"/c/f" {
		t.Fatalf("found %+v", changes)
	}
	if notThere := keptSealIn(nil, fs.ErrNotExist, a+"/c/f"); changes[0].Seal != notThere {
		t.Errorf("the seal walked through the link into the owner's place: %s, not %s", changes[0].Seal, notThere)
	}
}
