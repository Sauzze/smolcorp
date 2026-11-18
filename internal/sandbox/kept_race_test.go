//go:build linux || darwin

package sandbox

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXKEPTFD, the desk's own way: a folder in the middle and a link to the owner's place swap names atomically, over and
// over (swapNames: renameat2's RENAME_EXCHANGE on Linux, renamex_np's RENAME_SWAP on a Mac), while the core looks and
// puts back. Whatever moment each swap lands on, nothing of the owner's is moved, removed, written or shown.
func TestASwapRacingTheLookAndThePutBackNeverReachesTheOwnersPlace(t *testing.T) {
	for _, disk := range []string{"one disk", "another disk"} {
		t.Run(disk, func(t *testing.T) {
			pl := keptDesk(t)
			held := KeptFolder(pl.Support, "h1")
			home, _ := filepath.EvalSymlinks(t.TempDir())
			writeKeptFile(t, home+"/.bashrc", "the owner's own\n")
			writeKeptFile(t, home+"/owner-only.txt", "the owner's own\n")
			x, lnk := pl.Root+"/.claude/x", pl.Root+"/.claude/lnk"
			writeKeptFile(t, x+"/.bashrc", "the desk's, kept\n")
			if err := os.Symlink(home, lnk); err != nil {
				t.Fatal(err)
			}
			if err := SaveKept(pl); err != nil {
				t.Fatal(err)
			}
			if disk == "another disk" {
				onAnotherDisk(t)
			}
			for try := range 150 {
				writeKeptFile(t, x+"/.bashrc", fmt.Sprintf("the desk's, changed %d\n", try))
				guardStops(t, pl)
				stop, done := make(chan struct{}), make(chan struct{})
				go func() {
					defer close(done)
					for {
						select {
						case <-stop:
							return
						default:
							swapNames(x, lnk)
						}
					}
				}()
				changes, _ := RestoreKept(held)
				close(stop)
				<-done
				if st, err := os.Lstat(x); err == nil && st.Mode()&fs.ModeSymlink != 0 {
					if err := swapNames(x, lnk); err != nil {
						t.Fatal(err)
					}
				}
				if words := KeptWords(changes); strings.Contains(words, "the owner's own") || strings.Contains(words, "owner-only") {
					t.Fatalf("try %d: the owner's place shown: %s", try, words)
				}
				got := snapshot(t, home+"/.bashrc", home+"/owner-only.txt")
				if names, _ := os.ReadDir(home); len(names) != 2 || got[home+"/.bashrc"] != "the owner's own\n" || got[home+"/owner-only.txt"] != "the owner's own\n" {
					t.Fatalf("try %d: the owner's place was changed: %d names, %v", try, len(names), got)
				}
			}
			filepath.Walk(foundFolder(pl), func(p string, info os.FileInfo, _ error) error {
				if info != nil && info.Mode().IsRegular() && readKeptFile(t, p) == "the owner's own\n" {
					t.Errorf("the owner's file was set aside in %s", p)
				}
				return nil
			})
		})
	}
}
