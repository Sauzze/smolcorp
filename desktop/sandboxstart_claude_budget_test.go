//go:build !windows

package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// FXCLAUDESUMOPENR1's rows (RT126CLAUDESUMOPEN B1, B2): the plant walk runs in the registry's save, under
// frontDeskMu, and a desk decides what it finds. B1: it read a file once for every folder above it in its .claude,
// with no bound on the whole. B2: a folder it had listed, swapped for a link, was followed out of the place. Stand-ins
// only: big files are empty sparse ones, and what's outside the place is a few small files in the test's own folder.

// claudeOpens counts, while the test runs, each file the plant walk opens (claudeOpening), and the bytes it was
// looked at with.
type claudeOpens struct {
	sync.Mutex
	names map[string]int
	bytes int64
}

func countClaudeOpens(t *testing.T) *claudeOpens {
	t.Helper()
	c := &claudeOpens{names: map[string]int{}}
	was := claudeOpening
	claudeOpening = func(name string) {
		c.Lock()
		defer c.Unlock()
		c.names[name]++
		if info, err := os.Lstat(name); err == nil {
			c.bytes += info.Size()
		}
	}
	t.Cleanup(func() { claudeOpening = was })
	return c
}

// opensUnder is how many opens there were of files under folder.
func (c *claudeOpens) opensUnder(folder string) int {
	c.Lock()
	defer c.Unlock()
	n := 0
	for name, times := range c.names {
		if strings.HasPrefix(name, folder+string(filepath.Separator)) {
			n += times
		}
	}
	return n
}

// sparse makes an empty sparse file at at, size bytes long.
func sparse(t *testing.T, at string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(at, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(at, size); err != nil {
		t.Fatalf("this volume can't make a sparse file: %v", err)
	}
}

// fireHire fires a working hire at place, through the registry's save, and gives what's held for you after it: the
// save must come back within claudeSwapLimit.
func fireHire(t *testing.T, place string) map[string]claudePlant {
	t.Helper()
	h := Hire{ID: "abcdef000002", Name: "budget", State: "working", Folder: keptPath(place)}
	claudeRegistry(t, h)
	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- markFired(h.ID, "") }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(claudeSwapLimit):
		t.Fatalf("the fire's save was still held by the plant walk after %v", claudeSwapLimit)
	}
	t.Logf("the fire's save took %v", time.Since(start))
	claudeHeld.Lock()
	defer claudeHeld.Unlock()
	held := map[string]claudePlant{}
	for _, p := range claudeHeld.plants {
		held[p.File] = p
	}
	return held
}

// The reviewer's first measurement (B1): a .claude ten folders deep, two files of claudeSumMost (32 MiB) in each.
// The walk read each once for every folder above it: 150 reads, 4 GiB, under frontDeskMu. Now the fire's listing
// reads no more than it may sum, and what it didn't finish is held as too much to check.
func TestAFireReadsNoMoreThanItsListingMaySum(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	place := filepath.Join(claudeTempDir(t), "place")
	folder := filepath.Join(place, ".claude")
	for depth := 0; depth < 10; depth++ {
		folder = filepath.Join(folder, "d")
		sparse(t, filepath.Join(folder, "a"), claudeSumMost)
		sparse(t, filepath.Join(folder, "b"), claudeSumMost)
	}
	opens := countClaudeOpens(t)
	held := fireHire(t, place)
	t.Logf("%d bytes opened to read", opens.bytes)
	if opens.bytes > claudeSumBytesMost {
		t.Errorf("the fire's listing opened %d bytes to read, more than the %d it may sum", opens.bytes, claudeSumBytesMost)
	}
	if p, ok := held[place]; !ok || p.Why != whyClaudeTooMuch || p.Sum != "" {
		t.Errorf("the place isn't held as too much to check: %+v", p)
	}
	if why := claudeHeldRefusal(place); why == "" {
		t.Error("your Claude Code would start in the place")
	}
}

// The reviewer's second measurement (B1): one file 200 folders down a .claude was read 202 times a listing. Now a file
// is read once a listing, into every sum it's part of; and one too big to sum into all of them isn't read at all.
func TestAFileDeepInAClaudeIsReadOnceAListing(t *testing.T) {
	freshClaudeHeld(t)
	for _, size := range []int64{1 << 10, claudeSumMost} {
		place := filepath.Join(claudeTempDir(t), "place")
		file := filepath.Join(append([]string{place, ".claude"}, slicesRepeat("a", 200)...)...)
		file = filepath.Join(file, "f")
		sparse(t, file, size)
		opens := countClaudeOpens(t)
		start := time.Now()
		listed := holdClaudePlants(fmt.Sprintf("h%d", size), placeLook(place), "")
		t.Logf("%d bytes 200 folders down: listed in %v", size, time.Since(start))
		if n := opens.names[file]; n > 1 {
			t.Errorf("a file of %d bytes 200 folders down was read %d times in one listing", size, n)
		}
		byFile := map[string]claudePlant{}
		for _, p := range listed {
			byFile[p.File] = p
		}
		if size == 1<<10 && (opens.names[file] != 1 || byFile[file].Sum == "") {
			t.Errorf("the small file wasn't read and summed: %d opens, %+v", opens.names[file], byFile[file])
		}
		if size == claudeSumMost && byFile[place].Why != whyClaudeTooMuch {
			t.Errorf("the big file 200 folders down didn't make the place too much to check: %+v", byFile[place])
		}
	}
}

// A file just past claudeSumMost isn't opened, though the listing could still sum that much: the cap is per file,
// not only the listing's.
func TestAFileJustPastTheMostOneFileMayBeIsntRead(t *testing.T) {
	claude := claudeTree(t, filepath.Join(claudeTempDir(t), ".claude"), "settings.json")
	sparse(t, filepath.Join(claude, "settings.json"), claudeSumMost+1)
	opens := countClaudeOpens(t)
	if err := sumClaudeTree(io.Discard, claude); err == nil {
		t.Error("a file past claudeSumMost was summed")
	}
	if n := opens.names[filepath.Join(claude, "settings.json")]; n != 0 {
		t.Errorf("a file past claudeSumMost was opened %d times", n)
	}
}

func slicesRepeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

// Past claudeLookMost entries in the place, or claudeTreeMost in its .claude, the listing stops, and the place is held
// as too much to check: what's past the bound isn't known.
func TestTooManyEntriesInAPlaceAreHeldAsTooMuch(t *testing.T) {
	look, tree := claudeLookMost, claudeTreeMost
	t.Cleanup(func() { claudeLookMost, claudeTreeMost = look, tree })
	claudeLookMost, claudeTreeMost = 1000, 100
	for _, c := range []struct {
		name  string
		files []string
	}{
		{"in the place", manyFiles("a", 1200)},
		{"in its .claude", manyFiles(".claude/commands", 150)},
	} {
		t.Run(c.name, func(t *testing.T) {
			freshClaudeHeld(t)
			place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), append(c.files, "z/.claude/settings.json")...)
			listed := listedAs(t, place, holdClaudePlants("h1", placeLook(place), ""))
			if listed["."] != whyClaudeTooMuch {
				t.Errorf("the place isn't held as too much to check: %v", listed)
			}
			if _, ok := listed["z/.claude/settings.json"]; ok {
				t.Error("the listing went on past its bound")
			}
			if why := claudeHeldRefusal(place); why == "" {
				t.Error("your Claude Code would start in the place")
			}
		})
	}
}

func manyFiles(folder string, n int) []string {
	files := make([]string, n)
	for i := range files {
		files[i] = fmt.Sprintf("%s/f%04d.md", folder, i)
	}
	return files
}

// outsideTree makes, outside any place, 6 folders one in another with 5 files in each, as the reviewer's probe did.
func outsideTree(t *testing.T) string {
	t.Helper()
	outside := filepath.Join(claudeTempDir(t), "outside")
	var files []string
	folder := ""
	for depth := 0; depth < 6; depth++ {
		folder = filepath.Join(folder, fmt.Sprintf("d%d", depth))
		for i := 0; i < 5; i++ {
			files = append(files, filepath.ToSlash(filepath.Join(folder, fmt.Sprintf("f%d.md", i))))
		}
	}
	return claudeTree(t, outside, files...)
}

// swapOnOpen calls swap the first time the walk is about to open the file at at.
func swapOnOpen(t *testing.T, at string, swap func()) *claudeOpens {
	t.Helper()
	c := &claudeOpens{names: map[string]int{}}
	was := claudeOpening
	var once sync.Once
	claudeOpening = func(name string) {
		if name == at {
			once.Do(swap)
		}
		c.Lock()
		c.names[name]++
		c.Unlock()
	}
	t.Cleanup(func() { claudeOpening = was })
	return c
}

// swapForLink puts a link to to in place of the folder at at, moving it aside.
func swapForLink(t *testing.T, at, to string) {
	if err := os.Rename(at, at+".aside"); err != nil {
		t.Error(err)
	}
	if err := os.Symlink(to, at); err != nil {
		t.Error(err)
	}
}

// heldUnder is what's held under folder, by its path from folder.
func heldUnder(held map[string]claudePlant, folder string) []string {
	var under []string
	for at := range held {
		if strings.HasPrefix(at, folder+string(filepath.Separator)) {
			under = append(under, at[len(folder)+1:])
		}
	}
	return under
}

// The reviewer's probe (B2): a folder in a .claude the walk has listed, swapped for a link to a folder outside the
// place before the walk goes into it, isn't followed. It can't be summed, so the .claude and it are held.
func TestAFolderInAClaudeSwappedForALinkIsntFollowed(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	outside := outsideTree(t)
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), ".claude/settings.json", ".claude/skills/s/SKILL.md")
	claude, skills := filepath.Join(place, ".claude"), filepath.Join(place, ".claude", "skills")
	opens := swapOnOpen(t, filepath.Join(claude, "settings.json"), func() { swapForLink(t, skills, outside) })
	held := fireHire(t, place)
	if n := opens.opensUnder(skills); n > 0 {
		t.Errorf("the walk opened %d files through the link, outside the place", n)
	}
	if under := heldUnder(held, skills); len(under) > 0 {
		t.Errorf("what's outside the place was listed as in it: %d plants, %v", len(under), under)
	}
	if p := held[skills]; p.Why != whyUnread {
		t.Errorf("the folder swapped for a link isn't held as unread: %+v", p)
	}
	if p, ok := held[claude]; !ok || p.Sum != "" {
		t.Errorf("the .claude isn't held unsummed: %+v", p)
	}
	if why := claudeHeldRefusal(place); why == "" {
		t.Error("your Claude Code would start in the place")
	}
}

// The same in the place itself: a folder the walk has listed, swapped for a link to a folder outside the place
// with a .claude in it, isn't followed. The .claude out there isn't listed as the desk's, and the folder is held as
// unread.
func TestAFolderInThePlaceSwappedForALinkIsntFollowed(t *testing.T) {
	newTestDeck(t)
	freshClaudeHeld(t)
	outside := claudeTree(t, filepath.Join(claudeTempDir(t), "outside"), ".claude/settings.json", ".claude/hooks/pre.sh")
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), ".mcp.json", "a/notes.txt")
	a := filepath.Join(place, "a")
	opens := swapOnOpen(t, filepath.Join(place, ".mcp.json"), func() { swapForLink(t, a, outside) })
	held := fireHire(t, place)
	if n := opens.opensUnder(a); n > 0 {
		t.Errorf("the walk opened %d files through the link, outside the place", n)
	}
	if under := heldUnder(held, a); len(under) > 0 {
		t.Errorf("what's outside the place was listed as in it: %v", under)
	}
	if p := held[a]; p.Why != whyUnread {
		t.Errorf("the folder swapped for a link isn't held as unread: %+v", p)
	}
	if under := heldUnder(held, outside); len(under) > 0 {
		t.Errorf("the .claude outside the place is held as the desk's: %v", under)
	}
}

// A folder in a .claude swapped for another real folder (renamed into its place) isn't summed as it: it's no longer
// the folder the walk listed (os.SameFile), so it's held as unread.
func TestAFolderInAClaudeSwappedForAnotherFolderIsntSummedAsIt(t *testing.T) {
	freshClaudeHeld(t)
	base := claudeTempDir(t)
	other := claudeTree(t, filepath.Join(base, "other"), "s/SKILL.md")
	place := claudeTree(t, filepath.Join(base, "place"), ".claude/settings.json", ".claude/skills/s/SKILL.md")
	skills := filepath.Join(place, ".claude", "skills")
	swapOnOpen(t, filepath.Join(place, ".claude", "settings.json"), func() {
		if err := os.Rename(skills, skills+".aside"); err != nil {
			t.Error(err)
		}
		if err := os.Rename(other, skills); err != nil {
			t.Error(err)
		}
	})
	listed := listedAs(t, place, holdClaudePlants("h1", placeLook(place), ""))
	if listed[".claude/skills"] != whyUnread {
		t.Errorf("the folder swapped in was summed as the one listed: %v", listed)
	}
}

// Every plant's sum is what it was before this walk (claudeEntry over filepath.WalkDir, each entry's subtree read
// again, and a link within the place summed with what it leads to), byte for byte: baselines and kept plants on
// record still match.
func TestEveryPlantSumIsAsBefore(t *testing.T) {
	freshClaudeHeld(t)
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), ".claude/settings.json", ".claude/hooks/pre.sh",
		".claude/skills/s/SKILL.md", ".claude/skills/s/deep/er/run.sh", ".claude/agents/", ".claude/CLAUDE.md",
		".claude/output-styles/x.md", ".mcp.json", "shared/s.md", "shared/in/t.md", "b/.claude/commands/c.md")
	if err := os.WriteFile(filepath.Join(place, ".claude", "skills", "s", "blob"), []byte{0, 1, 2, 255, 0}, 0o600); err != nil {
		t.Fatal(err)
	}
	claudeLink(t, "../shared", filepath.Join(place, ".claude", "commands"))
	claudeLink(t, "settings.json", filepath.Join(place, ".claude", "linked.json"))
	claudeLink(t, "/nowhere/at/all", filepath.Join(place, ".claude", "dangling"))
	claudeLink(t, claudeTempDir(t), filepath.Join(place, ".claude", "skills", "out"))
	listed := holdClaudePlants("h1", placeLook(place), "")
	if len(listed) < 15 {
		t.Fatalf("only %d plants listed", len(listed))
	}
	for _, p := range listed {
		if was := oldClaudeSum(t, p.File, place); p.Sum != was {
			t.Errorf("%s: the sum is %q, was %q", p.File, p.Sum, was)
		}
	}
}

// oldClaudeSum is the sum claudeEntry gave the plant at at, looked for within, before this walk.
func oldClaudeSum(t *testing.T, at, within string) string {
	t.Helper()
	info, err := os.Lstat(at)
	if err != nil {
		return ""
	}
	h := sha256.New()
	walk := func(top string) error {
		return filepath.WalkDir(top, func(name string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(top, name)
			fmt.Fprintf(h, "%s\x00%v\x00", rel, d.Type())
			switch {
			case d.Type()&fs.ModeSymlink != 0:
				target, err := os.Readlink(name)
				fmt.Fprintf(h, "%s\x00", target)
				return err
			case d.Type().IsRegular():
				data, err := os.ReadFile(name)
				h.Write(data)
				return err
			}
			return nil
		})
	}
	err = walk(at)
	if err == nil && info.Mode()&fs.ModeSymlink != 0 {
		if to, linkErr := filepath.EvalSymlinks(at); linkErr == nil && inside(physical(within), to) {
			err = walk(to)
		}
	}
	if err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// FXCLAUDESUMOPENR2's rows (RT126CLAUDESUMOPENR2 B1's N1 and N2): two shapes a desk can make in its place, with no
// race, that kept the save under frontDeskMu far past the budget, which counted neither. N1: a chain of folders deeper
// than a path may be (made through each folder's fd, as a desk can), walked to its end with two fds a level and the
// whole path made at each. N2: links resolved by filepath.EvalSymlinks, 255 links and every name on the way, none of
// it spent from the walk.

// openFds is how many fds this process has open.
func openFds(t *testing.T) int {
	t.Helper()
	fds, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Fatal(err)
	}
	return len(fds)
}

// chain makes, in folder, depth folders one in another, each named name, through the fd of the one above (as a desk
// can, past the longest path), and calls each with the fd of each and its path, from the top down.
func chain(t *testing.T, folder, name string, depth int, each func(fd int, at string)) {
	t.Helper()
	fd, err := unix.Open(folder, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	at := folder
	for level := 1; level <= depth; level++ {
		if err := unix.Mkdirat(fd, name, 0o700); err != nil {
			t.Fatal(err)
		}
		next, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if err != nil {
			t.Fatal(err)
		}
		fd, at = next, at+string(filepath.Separator)+name
		each(fd, at)
	}
	unix.Close(fd)
}

// writeAt writes a short stand-in file name in the open folder fd.
func writeAt(t *testing.T, fd int, name string) {
	t.Helper()
	f, err := unix.Openat(fd, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	unix.Write(f, []byte(`{"hooks":"standin"}`))
	unix.Close(f)
}

// N1: a chain 2000 folders deep in the place. The walk stops where the walk by path stopped (claudePathMost): the
// first folder there or past it is held as unread, as before; it's listed in a fixed time, with no more than one fd a
// level open; and every plant near that stop has the sum it had before.
func TestADeepChainOfFoldersInAPlaceIsHeldWhereThePathStops(t *testing.T) {
	freshClaudeHeld(t)
	place := filepath.Join(claudeTempDir(t), "place")
	if err := os.Mkdir(place, 0o700); err != nil {
		t.Fatal(err)
	}
	stop, near := "", 3*len("/aa/.claude/hooks/x.sh") // the first folder past the stop; plants made near it
	chain(t, place, "aa", 2000, func(fd int, at string) {
		if len(at) >= claudePathMost && stop == "" {
			stop = at
		}
		if strings.Count(at, "/aa")%100 == 0 { // a file the walk opens, to count its fds then
			writeAt(t, fd, ".mcp.json")
		}
		if len(at) < claudePathMost && len(at)+near >= claudePathMost {
			unix.Mkdirat(fd, ".claude", 0o700)
			claude, _ := unix.Openat(fd, ".claude", unix.O_RDONLY|unix.O_DIRECTORY, 0)
			writeAt(t, claude, "settings.json")
			unix.Mkdirat(claude, "hooks", 0o700)
			hooks, _ := unix.Openat(claude, "hooks", unix.O_RDONLY|unix.O_DIRECTORY, 0)
			writeAt(t, hooks, "x.sh")
			unix.Close(hooks)
			unix.Close(claude)
		}
	})
	before, most := openFds(t), 0
	was := claudeOpening
	claudeOpening = func(string) { most = max(most, openFds(t)) }
	t.Cleanup(func() { claudeOpening = was })
	start := time.Now()
	listed := holdClaudePlants("h1", placeLook(place), "")
	took := time.Since(start)
	claudeOpening = was
	t.Logf("listed in %v, %d plants, at most %d fds open (%d before)", took, len(listed), most, before)
	if took > 3*time.Second {
		t.Errorf("a chain 2000 deep took %v to list", took)
	}
	if most-before >= claudePathMost/2 {
		t.Errorf("the walk had %d fds open, %d more than before it", most, most-before)
	}
	held := map[string]claudePlant{}
	for _, p := range listed {
		held[p.File] = p
	}
	if p, ok := held[stop]; !ok || p.Why != whyUnread || p.Sum != "" {
		t.Errorf("the first folder at the longest path isn't held as unread: %+v", p)
	}
	if why := claudeHeldRefusal(place); why == "" {
		t.Error("your Claude Code would start in the place")
	}
	nearby := 0
	for _, p := range listed {
		if p.Why == whyUnread {
			continue
		}
		if len(p.File) >= claudePathMost && p.Sum != "" {
			t.Errorf("a plant past the longest path was summed: %d bytes", len(p.File))
		}
		if len(p.File)+near >= claudePathMost {
			nearby++
		}
		if was := oldClaudeSum(t, p.File, place); p.Sum != was {
			t.Errorf("%d bytes down: the sum is %q, was %q", len(p.File), p.Sum, was)
		}
	}
	if nearby == 0 {
		t.Error("no plant near the longest path was listed")
	}
}

// loopLink makes, in folder, a link that leads to itself at the end of 300 folders (the reviewer's 704 bytes from a
// short temp folder), so that filepath.EvalSymlinks looks up every name on the way, then goes through 255 links.
func loopLink(t *testing.T, folder string) string {
	t.Helper()
	deep := filepath.Join(append([]string{folder}, slicesRepeat("a", 300)...)...)
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	loop := filepath.Join(deep, "x")
	claudeLink(t, "x", loop)
	return loop
}

// N2: links to such a loop, 20,000 .mcp.json ones in the place (40,000 entries, a fifth of the old bound), and 500 in a
// .claude. Each was resolved without a bound: 1 min 20 s, and 1.9 s. Now each name looked up is spent from the walk,
// and no more than claudeLinkHops links are gone through: past either, the link can't be summed, and it's held.
func TestLinksToALoopAreHeldWithinTheBound(t *testing.T) {
	for _, c := range []struct {
		name  string
		links func(place string) []string
	}{
		{"20,000 .mcp.json links in the place", func(place string) []string {
			var links []string
			for i := 0; i < 20_000; i++ {
				links = append(links, filepath.Join(place, fmt.Sprintf("f%05d", i), ".mcp.json"))
			}
			return links
		}},
		{"500 links in a .claude", func(place string) []string {
			var links []string
			for i := 0; i < 500; i++ {
				links = append(links, filepath.Join(place, ".claude", "commands", fmt.Sprintf("c%03d.md", i)))
			}
			return links
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			freshClaudeHeld(t)
			base := claudeTempDir(t)
			loop, place := loopLink(t, filepath.Join(base, "z")), filepath.Join(base, "p")
			for _, at := range c.links(place) {
				if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
					t.Fatal(err)
				}
				claudeLink(t, loop, at)
			}
			start := time.Now()
			listed := holdClaudePlants("h1", placeLook(place), "")
			took := time.Since(start)
			t.Logf("listed in %v, %d plants", took, len(listed))
			if took > 5*time.Second {
				t.Errorf("the links took %v to list", took)
			}
			for _, p := range listed {
				if p.Sum != "" && !p.Dir {
					t.Errorf("a link to the loop was summed as %s: %s", p.Sum, p.File)
					break
				}
			}
			if why := claudeHeldRefusal(place); why == "" {
				t.Error("your Claude Code would start in the place")
			}
		})
	}
}

// RT126CLAUDESUMOPENR2's follow-up: each .mcp.json is opened, summed and noted on record like an entry in a .claude,
// so it's spent from claudeTreeMost too: 20,000 of them was 0.8 s, about 4 s at the entry bound.
func TestManyMCPFilesInAPlaceAreHeldAsTooMuch(t *testing.T) {
	freshClaudeHeld(t)
	files := make([]string, claudeTreeMost+500)
	for i := range files {
		files[i] = fmt.Sprintf("f%05d/.mcp.json", i)
	}
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), files...)
	listed := listedAs(t, place, holdClaudePlants("h1", placeLook(place), ""))
	if listed["."] != whyClaudeTooMuch {
		t.Errorf("the place isn't held as too much to check (%d listed)", len(listed))
	}
	if len(listed) > claudeTreeMost+1 {
		t.Errorf("%d listed, past the %d a listing may sum", len(listed), claudeTreeMost)
	}
}

// VICTOR's decision on RT126CLAUDESUMOPENR2's point 4: a folder hire's place is its project folder, which in a big
// checkout (with the team's worktrees in it) holds 233,000 to 464,000 entries. One of 400,000 is listed, not held as
// too much to check.
func TestAFolderPlaceOf400kEntriesIsntHeldAsTooMuch(t *testing.T) {
	freshClaudeHeld(t)
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), "zz/.claude/settings.json")
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for w := 0; w < 4; w++ { // more at once is slower on a Mac
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := w; i < 400; i += 4 {
				folder := filepath.Join(place, fmt.Sprintf("d%03d", i))
				if err := os.Mkdir(folder, 0o700); err != nil {
					errs <- err
					return
				}
				for j := 0; j < 999; j++ { // 999 names and the folder: 1000 entries
					if err := os.Symlink("x", filepath.Join(folder, fmt.Sprintf("n%03d", j))); err != nil {
						errs <- err
						return
					}
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	start := time.Now()
	listed := listedAs(t, place, holdClaudePlants("h1", placeLook(place), ""))
	t.Logf("400,000 entries listed in %v", time.Since(start))
	if listed["."] == whyClaudeTooMuch {
		t.Error("the place is held as too much to check")
	}
	if listed["zz/.claude/settings.json"] != whyClaudeSettings {
		t.Errorf("the walk didn't reach the end of the place: %v", listed)
	}
}

// A link to a loop costs the listing claudeLinkHops links at most, not all it may look at: it can't be summed and is
// held, and the rest of the place is still listed and summed.
func TestALinkToALoopCostsTheListingNoMoreThanItsHops(t *testing.T) {
	freshClaudeHeld(t)
	place := claudeTree(t, filepath.Join(claudeTempDir(t), "place"), ".claude/settings.json")
	claudeLink(t, "loop", filepath.Join(place, ".claude", "loop"))
	listed := holdClaudePlants("h1", placeLook(place), "")
	got := map[string]claudePlant{}
	for _, p := range listed {
		rel, _ := filepath.Rel(place, p.File)
		got[filepath.ToSlash(rel)] = p
	}
	if p, ok := got[".claude/loop"]; !ok || p.Sum != "" {
		t.Errorf("the link to a loop isn't held unsummed: %+v", p)
	}
	if p := got["."]; p.Why == whyClaudeTooMuch {
		t.Error("one link to a loop made the place too much to check")
	}
	if p := got[".claude/settings.json"]; p.Sum == "" || p.Sum != oldClaudeSum(t, p.File, place) {
		t.Errorf("the rest of the place isn't summed as before: %+v", p)
	}
}
