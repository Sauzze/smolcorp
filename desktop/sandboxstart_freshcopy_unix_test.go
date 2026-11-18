//go:build !windows

package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// FXFRESHCOPYCAP: freshCopy copies a file desks may read but not write, one with a second name in a place a desk may
// write. A desk could grow it through that name, sparse, to any size; the copy writes real bytes into the support
// folder. Each row here watches the copy (freshCopyWatch): a row that fails writes about freshCopyWatchMost, never more.

// freshCopyWatchMost is how many bytes a copy may reach in a row before the watch ends it.
const freshCopyWatchMost = 64 << 20

// onDisk is how many bytes the file takes on disk, holes left out.
func onDisk(info os.FileInfo) int64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return st.Blocks * 512
	}
	return info.Size()
}

// freshCopyWatch watches a folder for freshCopy's temp files while a row runs: the most bytes one held, in size or on
// disk, and whether it ended a copy. Past freshCopyWatchMost it empties the file the copy reads, which ends it.
type freshCopyWatch struct {
	most     int64
	fired    bool
	done     chan struct{}
	finished chan struct{}
	once     sync.Once
}

func watchFreshCopies(t *testing.T, folder, source string) *freshCopyWatch {
	t.Helper()
	w := &freshCopyWatch{done: make(chan struct{}), finished: make(chan struct{})}
	go func() {
		defer close(w.finished)
		for {
			select {
			case <-w.done:
				return
			case <-time.After(time.Millisecond):
			}
			entries, _ := os.ReadDir(folder)
			for _, e := range entries {
				if !strings.Contains(e.Name(), ".fresh-") {
					continue
				}
				info, err := os.Lstat(filepath.Join(folder, e.Name()))
				if err != nil {
					continue
				}
				w.most = max(w.most, info.Size(), onDisk(info))
				if w.most > freshCopyWatchMost && !w.fired {
					w.fired = true
					_ = os.Truncate(source, 0)
				}
			}
		}
	}()
	t.Cleanup(w.stop)
	return w
}

func (w *freshCopyWatch) stop() {
	w.once.Do(func() { close(w.done) })
	<-w.finished
}

// growHuge makes the file at name, a desk's own name for it, a 1 TiB sparse file, as the desk could.
func growHuge(t *testing.T, name string) {
	t.Helper()
	if err := os.Truncate(name, 1<<40); err != nil {
		t.Skipf("this volume can't hold a 1 TiB sparse file: %v", err)
	}
}

func freshLeftovers(folder string) []string {
	var left []string
	entries, _ := os.ReadDir(folder)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".fresh-") {
			left = append(left, e.Name())
		}
	}
	return left
}

// A desk with a second name of the guard file makes it 1 TiB, sparse, through that name. The start is refused at once,
// in words, and nothing is copied: no temp file, no byte on disk.
func TestAHugeSparseSecondNameRefusesTheStartWithNothingWritten(t *testing.T) {
	tr := newLinkTree(t)
	guard := filepath.Join(tr.support, "hire-guard.json")
	other := tr.linkMet(t, guard)
	growHuge(t, other)
	before, _ := os.Lstat(guard)
	folderBefore := onDiskUnder(t, tr.support)
	w := watchFreshCopies(t, tr.support, guard)
	start := time.Now()
	replaced, err := checkLinks(tr.policy(t), tr.places())
	took := time.Since(start)
	w.stop()
	after, statErr := os.Lstat(guard)
	if statErr != nil {
		t.Fatal(statErr)
	}
	folderAfter := onDiskUnder(t, tr.support)
	t.Logf("guard file %d bytes, %d on disk; the copy reached %d bytes; the support folder %d bytes on disk before, %d after; %v; %v",
		before.Size(), onDisk(before), w.most, folderBefore, folderAfter, took, err)
	if w.fired {
		t.Fatalf("the copy went past %d bytes and was ended by the row: nothing capped it", freshCopyWatchMost)
	}
	var words *linkCheckError
	if !errors.As(err, &words) || !strings.Contains(err.Error(), guard) || !strings.Contains(err.Error(), "bigger than Smolcorp ever makes it") || len(replaced) != 0 {
		t.Fatalf("checkLinks: %v, replaced %v; want the start refused for its size, naming %s", err, replaced, guard)
	}
	if w.most != 0 || folderAfter != folderBefore {
		t.Errorf("something was written: a copy of %d bytes, the folder %d → %d bytes on disk", w.most, folderBefore, folderAfter)
	}
	if left := freshLeftovers(tr.support); len(left) != 0 {
		t.Errorf("a temp file was left: %v", left)
	}
	if !os.SameFile(after, before) || after.Size() != 1<<40 {
		t.Error("the file refused for was changed")
	}
	if took > 5*time.Second {
		t.Errorf("the refusal took %v", took)
	}
}

// onDiskUnder is how many bytes everything under folder takes on disk.
func onDiskUnder(t *testing.T, folder string) int64 {
	t.Helper()
	var total int64
	err := filepath.Walk(folder, func(_ string, info os.FileInfo, err error) error {
		if err == nil {
			total += onDisk(info)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return total
}

// The guard file is looked at small, then grown to 1 TiB through the desk's name before the copy: the copy reads no
// further than the size looked at, and fails.
func TestAFileGrownBetweenTheLookAndTheCopyIsntCopied(t *testing.T) {
	tr := newLinkTree(t)
	guard := filepath.Join(tr.support, "hire-guard.json")
	other := tr.linkMet(t, guard)
	looked, err := os.Lstat(guard)
	if err != nil {
		t.Fatal(err)
	}
	growHuge(t, other)
	w := watchFreshCopies(t, tr.support, guard)
	start := time.Now()
	err = freshCopy(guard, looked, 2)
	took := time.Since(start)
	w.stop()
	t.Logf("looked at %d bytes; the copy reached %d bytes; %v; %v", looked.Size(), w.most, took, err)
	if w.fired {
		t.Fatalf("the copy went past %d bytes and was ended by the row: it read past the size looked at", freshCopyWatchMost)
	}
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("freshCopy: %v; want it refused as changed", err)
	}
	if w.most > looked.Size()+1 {
		t.Errorf("the copy reached %d bytes, more than the %d looked at and one", w.most, looked.Size())
	}
	if left := freshLeftovers(tr.support); len(left) != 0 {
		t.Errorf("a temp file was left: %v", left)
	}
	if now, _ := os.Lstat(guard); !os.SameFile(now, looked) {
		t.Error("the grown file was replaced")
	}
}

// The guard file is looked at, then another file of the same size is put in its place (a stand-in: desks can't write
// the support folder). It isn't read at all: the copy fails before it starts.
func TestAFileSwappedAfterTheLookIsntRead(t *testing.T) {
	tr := newLinkTree(t)
	guard := filepath.Join(tr.support, "hire-guard.json")
	tr.linkMet(t, guard)
	looked, err := os.Lstat(guard)
	if err != nil {
		t.Fatal(err)
	}
	swapped := filepath.Join(tr.support, "swapped")
	if err := os.WriteFile(swapped, []byte(`{"fence":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(swapped, guard); err != nil {
		t.Fatal(err)
	}
	in, _ := os.Lstat(guard)
	err = freshCopy(guard, looked, 2)
	if err == nil || !strings.Contains(err.Error(), "changed while it was looked at") {
		t.Fatalf("freshCopy: %v; want it refused before it reads (changed while it was looked at)", err)
	}
	if now, _ := os.Lstat(guard); !os.SameFile(now, in) {
		t.Error("the file put in its place was replaced")
	}
	if left := freshLeftovers(tr.support); len(left) != 0 {
		t.Errorf("a temp file was left: %v", left)
	}
}

// The cap is the largest a real file can be: 5 MiB is past any guard file Smolcorp writes, so the start is refused;
// a skill's file the core copies into a Linux desk's Claude folder may be that big, and is replaced as before.
func TestTheCapIsTheLargestARealFileCanBe(t *testing.T) {
	const size = 5 << 20
	for _, c := range []struct {
		rel     string
		refused bool
	}{
		{"home/L/AD/hire-guard.json", true},
		{"home/L/AD/hires/h1/claude/skills/s/asset.bin", false},
	} {
		t.Run(c.rel, func(t *testing.T) {
			tr := newLinkTree(t)
			target := filepath.Join(tr.base, filepath.FromSlash(c.rel))
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte("standin"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Truncate(target, size); err != nil {
				t.Fatal(err)
			}
			tr.linkMet(t, target)
			w := watchFreshCopies(t, filepath.Dir(target), target)
			replaced, err := checkLinks(tr.policy(t), tr.places())
			w.stop()
			if c.refused {
				if err == nil || !strings.Contains(err.Error(), "bigger than Smolcorp ever makes it") || len(replaced) != 0 || w.most != 0 {
					t.Fatalf("checkLinks: %v, replaced %v, a copy of %d bytes; want the start refused for its size, nothing copied", err, replaced, w.most)
				}
				return
			}
			if err != nil || !slices.Equal(replaced, []string{target}) {
				t.Fatalf("checkLinks: %v, replaced %v", err, replaced)
			}
			if now, _ := os.Lstat(target); now.Size() != size {
				t.Errorf("the copy is %d bytes, want %d", now.Size(), size)
			}
		})
	}
}
