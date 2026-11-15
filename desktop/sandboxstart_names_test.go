package desktop

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// X12LINK1 (SIERRA's ruling on RT126SAX12R2's F1): a file the link check would replace is replaced only when the walk
// met every one of its names. A name it didn't meet may be a file desks can't read in a place the walk passes over (a
// transcript, another desk's worktree), and a fresh copy would hand desks its bytes.

// linkSecondName makes the file at name a second name of the file at of, as a process outside any sandbox could have
// before the desk's start. A file at of that isn't there is made, holding a stand-in.
func linkSecondName(t *testing.T, name, of string) {
	t.Helper()
	if _, err := os.Lstat(of); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(of), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(of, []byte("standin"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(of, name); err != nil {
		t.Fatal(err)
	}
}

func oneFile(t *testing.T, a, b string) bool {
	t.Helper()
	ai, err := os.Lstat(a)
	if err != nil {
		t.Fatal(err)
	}
	bi, err := os.Lstat(b)
	if err != nil {
		t.Fatal(err)
	}
	return os.SameFile(ai, bi)
}

func TestAFileToReplaceWithANameTheCheckDoesntMeetRefusesTheStart(t *testing.T) {
	const transcript = "home/.claude/projects/p/x.jsonl"
	for _, c := range []struct {
		replace, other string
		denied         bool // desks can't read other
		// met: the desk's status file, which the check looks at, is a third name; metFirst: the walk meets it first.
		met, metFirst bool
	}{
		// The reviewer's probe: the guard file a second name of a transcript, which desks can't read and the walk
		// passes over (accountFoldersSkipped). The night shift's jobs and the desk's brief, the same.
		{replace: "home/L/AD/hire-guard.json", other: transcript, denied: true},
		{replace: "home/L/AD/nightshift.json", other: transcript, denied: true},
		{replace: "home/L/AD/hires/h1/BRIEF.md", other: transcript, denied: true},
		// Another desk's worktree and the owner's checkout (linkCheckSkipped).
		{replace: "home/L/AD/hire-guard.json", other: "home/P/app-hires/h2/notes.txt"},
		{replace: "home/L/AD/hire-guard.json", other: "home/P/app/notes.txt"},
		// A third name the check does meet doesn't make up for the one it doesn't, in both walk orders.
		{replace: "home/L/AD/hire-guard.json", other: transcript, denied: true, met: true},
		{replace: "home/L/AD/nightshift.json", other: transcript, denied: true, met: true, metFirst: true},
	} {
		t.Run(c.replace+" as "+c.other, func(t *testing.T) {
			tr := newLinkTree(t)
			at := func(rel string) string { return filepath.Join(tr.base, filepath.FromSlash(rel)) }
			replace, other := at(c.replace), at(c.other)
			pol := tr.policy(t)
			if v, _ := pol.Decide(replace, sandbox.Read); v == sandbox.Deny {
				t.Fatalf("%s: desks can't read it, so it isn't one the check replaces", replace)
			}
			if v, _ := pol.Decide(other, sandbox.Read); (v == sandbox.Deny) != c.denied {
				t.Fatalf("%s: read %v, the row says denied=%v", other, v, c.denied)
			}
			linkSecondName(t, replace, other)
			if c.met {
				status := tr.linkMet(t, replace)
				if (status < replace) != c.metFirst {
					t.Fatalf("the walk meets %s first, not as the row says", map[bool]string{true: status, false: replace}[status < replace])
				}
			}
			// A file that alone would be replaced (its other name is in the desk's Claude folder, which the check
			// looks at) shows nothing is replaced in a refused start.
			alone := filepath.Join(tr.support, "nightshift.json")
			if c.replace == "home/L/AD/nightshift.json" {
				alone = filepath.Join(tr.support, "hire-guard.json")
			}
			aloneOther := filepath.Join(tr.own, "claude", "notes.md")
			linkSecondName(t, aloneOther, alone)
			sums := treeSums(t, tr.base)
			got, err := checkLinks(pol, tr.places())
			var words *linkCheckError
			if !errors.As(err, &words) || !strings.Contains(err.Error(), replace) || !strings.Contains(err.Error(), "doesn't look in") || len(got) != 0 {
				t.Fatalf("checkLinks: %v, replaced %v; want the start refused for a name it didn't meet, naming %s", err, got, replace)
			}
			if now := treeSums(t, tr.base); !slices.Equal(now, sums) {
				t.Error("a refused start changed a file")
			}
			if !oneFile(t, replace, other) {
				t.Errorf("%s was replaced: a fresh copy would hold %s's bytes", replace, other)
			}
			if !oneFile(t, alone, aloneOther) {
				t.Errorf("%s was replaced in a refused start", alone)
			}
		})
	}
}

// TestFilesToReplaceThatShareANameAreAllReplaced: the guard file and the night shift's jobs as one file, every name
// met: each is replaced, and the second's link count, one fewer once the first is, is no reason to refuse.
func TestFilesToReplaceThatShareANameAreAllReplaced(t *testing.T) {
	for _, withStatus := range []bool{false, true} {
		t.Run(map[bool]string{false: "two names", true: "and the status file"}[withStatus], func(t *testing.T) {
			tr := newLinkTree(t)
			guard, night := filepath.Join(tr.support, "hire-guard.json"), filepath.Join(tr.support, "nightshift.json")
			linkSecondName(t, night, guard)
			names := []string{guard, night}
			if withStatus {
				names = append(names, tr.linkMet(t, guard))
			}
			got, err := checkLinks(tr.policy(t), tr.places())
			if err != nil || !slices.Equal(got, []string{guard, night}) {
				t.Fatalf("checkLinks: %v, replaced %v; want both replaced", err, got)
			}
			for i, a := range names {
				for _, b := range names[i+1:] {
					if oneFile(t, a, b) {
						t.Errorf("%s and %s are still one file", a, b)
					}
				}
				if data, _ := os.ReadFile(a); string(data) != `{"fence":1}` {
					t.Errorf("%s holds %q", a, data)
				}
			}
		})
	}
}

// TestAFreshCopyIsntPutInPlaceWhenTheNamesChanged: a name made or removed between the check's walk and the rename
// refuses the start, with the file left as it was.
func TestAFreshCopyIsntPutInPlaceWhenTheNamesChanged(t *testing.T) {
	for _, change := range []string{"made", "removed", "none"} {
		t.Run(change, func(t *testing.T) {
			dir := t.TempDir()
			at, second := filepath.Join(dir, "hire-guard.json"), filepath.Join(dir, "second")
			if err := os.WriteFile(at, []byte("standin"), 0o600); err != nil {
				t.Fatal(err)
			}
			linkSecondName(t, second, at)
			info, err := os.Lstat(at)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "made":
				linkSecondName(t, filepath.Join(dir, "third"), at)
			case "removed":
				if err := os.Remove(second); err != nil {
					t.Fatal(err)
				}
			}
			err = freshCopy(at, info, 2)
			now, _ := os.Lstat(at)
			if change == "none" {
				if err != nil || os.SameFile(now, info) {
					t.Fatalf("freshCopy: %v, still the same file %v", err, os.SameFile(now, info))
				}
				return
			}
			if err == nil || !os.SameFile(now, info) {
				t.Fatalf("freshCopy: %v, still the same file %v; want it refused and the file left", err, os.SameFile(now, info))
			}
			if names, _ := os.ReadDir(dir); slices.ContainsFunc(names, func(e fs.DirEntry) bool { return strings.Contains(e.Name(), ".fresh-") }) {
				t.Errorf("a temp file was left beside it: %v", names)
			}
		})
	}
}

// symlinkOrSkip makes a symbolic link, skipping only where Windows lacks the privilege to.
func symlinkOrSkip(t *testing.T, target, at string) {
	t.Helper()
	if err := os.Symlink(target, at); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("no symbolic links here: %v", err)
		}
		t.Fatal(err)
	}
}

// TestANameMetTwiceCountsOnce: the same folder met under two spellings (through a symbolic link above it) is one
// folder, so a name met twice can't stand in for a name not met.
func TestANameMetTwiceCountsOnce(t *testing.T) {
	base := t.TempDir()
	real, alias, away := filepath.Join(base, "real"), filepath.Join(base, "alias"), filepath.Join(base, "away")
	for _, d := range []string{real, away} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	symlinkOrSkip(t, real, alias)
	file := filepath.Join(real, "hire-guard.json")
	if err := os.WriteFile(file, []byte("standin"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkSecondName(t, filepath.Join(real, "second"), file)
	linkSecondName(t, filepath.Join(away, "not-met"), file)
	var met []linked
	for _, at := range []string{file, filepath.Join(alias, "hire-guard.json"), filepath.Join(real, "second"), filepath.Join(alias, "second")} {
		info, err := os.Lstat(at)
		if err != nil {
			t.Fatal(err)
		}
		met = append(met, linked{at: at, info: info})
	}
	if n, err := namesMet(met[0], met); err != nil || n != 2 {
		t.Fatalf("namesMet: %d, %v; want 2 (of 3), each met once under two spellings", n, err)
	}
}

// TestTheLinkCheckCountsNamesThroughALinkedParent: places spelled through a symbolic link above them are walked as
// any other: the reviewer's probe still refuses the start, and a file whose every name was met is still replaced.
func TestTheLinkCheckCountsNamesThroughALinkedParent(t *testing.T) {
	for _, probe := range []bool{true, false} {
		t.Run(map[bool]string{true: "the guard file as a transcript", false: "the guard file as the status file"}[probe], func(t *testing.T) {
			tr := newLinkTree(t)
			hl := filepath.Join(tr.base, "hl")
			symlinkOrSkip(t, tr.home, hl)
			via := func(at string) string { return hl + strings.TrimPrefix(at, tr.home) }
			lt := linkTree{base: tr.base, home: hl, support: via(tr.support), own: via(tr.own), hires: via(tr.hires),
				root: via(tr.root), repo: via(tr.repo), git: via(tr.git)}
			guard := filepath.Join(lt.support, "hire-guard.json")
			other := filepath.Join(lt.home, ".claude", "projects", "p", "x.jsonl")
			if probe {
				linkSecondName(t, guard, other)
			} else {
				other = lt.linkMet(t, guard)
			}
			got, err := checkLinks(lt.policy(t), lt.places())
			if probe {
				if err == nil || !strings.Contains(err.Error(), "doesn't look in") || len(got) != 0 || !oneFile(t, guard, other) {
					t.Fatalf("checkLinks: %v, replaced %v; want the start refused and the guard file left", err, got)
				}
				return
			}
			if err != nil || !slices.Equal(got, []string{guard}) || oneFile(t, guard, other) {
				t.Fatalf("checkLinks: %v, replaced %v; want the guard file replaced", err, got)
			}
		})
	}
}
