//go:build unix

package sandbox

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// L1GUARD1: the places no mount holds in Linux's view are kept by the start and compared on both sides of every tool
// call by the guard, which only stops the desk and notes why. Only the core puts back, and only what the desk was
// stopped for, as it was found then: at a fire, a Take over or a Hand back (L1GUARD1FIRE), and at the next start. What it finds is
// set aside whole, never removed, and the repo's shared objects/info is never put back (SIERRA's rulings, 15 Nov).
// Each test works in its own temp folder, a stand-in for a desk's worktree, its registration and its shared .git.

// keptDesk is the desk h1's places in a temp folder, made as a start leaves them: its place with a .claude and a .git
// link file, its registration, the shared objects/info and its branch folders.
func keptDesk(t *testing.T) Places {
	t.Helper()
	base := t.TempDir()
	if real, err := filepath.EvalSymlinks(base); err == nil {
		base = real
	}
	return keptDeskIn(t, base, "h1")
}

// keptDeskIn is the desk id's places on the repo base/app, its worktree base/app-hires/id, kept by its first start.
func keptDeskIn(t *testing.T, base, id string) Places {
	t.Helper()
	pl := Places{GOOS: "linux", Root: base + "/app-hires/" + id, GitDir: base + "/app/.git", OwnWorktree: id, OwnID: id,
		Support: base + "/support"}
	reg := pl.GitDir + "/worktrees/" + id
	for at, text := range map[string]string{
		pl.Root + "/.claude/settings.json":               `{"hooks":{}}` + "\n",
		pl.Root + "/.claude/settings.local.json":         `{"permissions":{}}` + "\n",
		pl.Root + "/.claude/commands/review.md":          "Review it.\n",
		pl.Root + "/.git":                                "gitdir: " + reg + "\n",
		pl.Root + "/src/main.go":                         "package main\n",
		reg + "/commondir":                               "../..\n",
		reg + "/gitdir":                                  pl.Root + "/.git\n",
		reg + "/config.worktree":                         "",
		reg + "/HEAD":                                    "ref: refs/heads/hire/" + id + "/work\n",
		pl.GitDir + "/refs/heads/hire/" + id + "/work":   "0123\n",
		pl.GitDir + "/logs/refs/heads/hire/" + id + "/x": "",
	} {
		writeKeptFile(t, at, text)
	}
	if _, err := os.Lstat(pl.GitDir + "/objects/info/packs"); err != nil {
		writeKeptFile(t, pl.GitDir+"/objects/info/packs", "P pack-1.pack\n")
	}
	for _, folder := range []string{reg + "/branches", reg + "/remotes", pl.Support + "/hires/" + id} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := SaveKept(pl); err != nil {
		t.Fatalf("the start's copy: %v", err)
	}
	return pl
}

func writeKeptFile(t *testing.T, at, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// replaceByRename puts text at at as a program of the owner's does: a new file renamed over it, a new inode.
func replaceByRename(t *testing.T, at, text string) {
	t.Helper()
	writeKeptFile(t, at+".new", text)
	if err := os.Rename(at+".new", at); err != nil {
		t.Fatal(err)
	}
}

func readKeptFile(t *testing.T, at string) string {
	t.Helper()
	if info, err := os.Lstat(at); err == nil && !info.Mode().IsRegular() {
		return "(not a file: " + info.Mode().String() + ")" // a pipe left in place would block the read
	}
	data, err := os.ReadFile(at)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// onAnotherDisk makes the found folder a stand-in for one on another disk: renames into it, and back, fail.
func onAnotherDisk(t *testing.T) {
	keptRename = func(*os.Root, string, *os.Root, string) error { return syscall.EXDEV }
	t.Cleanup(func() { keptRename = renameKept })
}

// foundFolder is the desk's folder of what the core set aside.
func foundFolder(pl Places) string {
	return filepath.Join(pl.Support, "hires", pl.OwnID, keptFoundName)
}

// guardStops is the guard's look in the desk's view, as hook.go does it: what changed, noted for the next start.
func guardStops(t *testing.T, pl Places) []KeptChange {
	t.Helper()
	held := KeptFolder(pl.Support, pl.OwnID)
	changes, err := CheckKept(held)
	if err != nil {
		t.Fatal(err)
	}
	if err := NoteKeptStop(held, changes); err != nil {
		t.Fatal(err)
	}
	return changes
}

// The start keeps each place no mount holds, and only those: nothing changed is nothing to say. Beside the copy, the
// guard's notes are there, empty.
func TestTheStartKeepsThePlacesNoMountHolds(t *testing.T) {
	pl := keptDesk(t)
	held := KeptFolder(pl.Support, "h1")
	var kept []string
	for _, k := range keptPlaces(pl) {
		kept = append(kept, strings.TrimPrefix(strings.TrimPrefix(k.at, pl.Root), pl.GitDir))
		if k.repo != (k.at == pl.GitDir+"/objects/info") {
			t.Errorf("%s: repo %v", k.at, k.repo)
		}
	}
	want := []string{"/.claude", "/objects/info", "/.git", "/worktrees/h1/commondir", "/worktrees/h1/gitdir",
		"/worktrees/h1/config.worktree", "/worktrees/h1/branches", "/worktrees/h1/remotes", "/refs/heads/hire/h1", "/logs/refs/heads/hire/h1"}
	if !slices.Equal(kept, want) {
		t.Fatalf("kept %q, want %q", kept, want)
	}
	if changes, err := CheckKept(held); err != nil || len(changes) != 0 {
		t.Fatalf("a desk that changed nothing: %v, %+v", err, changes)
	}
	// A file's copy is its content, by its sum; its HEAD and its work tree aren't kept.
	entries, _ := os.ReadDir(held)
	if len(entries) < 4 || readKeptFile(t, filepath.Join(held, keptList)) == "" {
		t.Fatalf("the copy holds %d files", len(entries))
	}
	if list := readKeptFile(t, filepath.Join(held, keptList)); strings.Contains(list, "/HEAD") || strings.Contains(list, "main.go") {
		t.Fatalf("the copy keeps more than the places no mount holds: %s", list)
	}
	if got := readKeptFile(t, keptStopFile(held)); got != "" {
		t.Fatalf("the guard's notes at the start: %q", got)
	}
}

// keptCases are changes to the places the core puts back, whoever made them.
var keptCases = []struct {
	name, at, what string
	change         func(t *testing.T, at string)
}{
	{"config.worktree written in place", "/worktrees/h1/config.worktree", "changed", func(t *testing.T, at string) {
		writeKeptFile(t, at, "[core]\n\tfsmonitor = ./stand-in\n")
	}},
	{"commondir replaced by a rename", "/worktrees/h1/commondir", "replaced", func(t *testing.T, at string) {
		replaceByRename(t, at, "../../stand-in\n")
	}},
	{"gitdir removed", "/worktrees/h1/gitdir", "removed", func(t *testing.T, at string) {
		os.Remove(at)
	}},
	{"the .git link file made a link", "/.git", "replaced", func(t *testing.T, at string) {
		os.Remove(at)
		os.Symlink("/elsewhere/.git", at)
	}},
	{"a hook added to .claude", "/.claude/hooks", "added", func(t *testing.T, at string) {
		writeKeptFile(t, at+"/stand-in.sh", "true\n")
	}},
	{"a setting changed in .claude", "/.claude/settings.json", "changed", func(t *testing.T, at string) {
		writeKeptFile(t, at, `{"hooks":{"stand-in":true}}`)
	}},
	{".claude replaced whole by a rename", "/.claude", "replaced", func(t *testing.T, at string) {
		writeKeptFile(t, at+".new/settings.json", "{}")
		writeKeptFile(t, at+".new/commands/mine.md", "the new one\n")
		os.RemoveAll(at)
		os.Rename(at+".new", at)
	}},
	{"a remote added to the old remote folder", "/worktrees/h1/remotes/origin", "added", func(t *testing.T, at string) {
		writeKeptFile(t, at, "URL: stand-in\n")
	}},
	{"a kept file's mode", "/worktrees/h1/commondir", "changed", func(t *testing.T, at string) {
		os.Chmod(at, 0o755)
	}},
	{"a kept file made a pipe", "/worktrees/h1/gitdir", "replaced", func(t *testing.T, at string) {
		os.Remove(at)
		syscall.Mkfifo(at, 0o600)
	}},
	{"a pipe added", "/.claude/p", "added", func(t *testing.T, at string) {
		syscall.Mkfifo(at, 0o600)
	}},
}

func caseAt(pl Places, at string) string {
	if strings.HasPrefix(at, "/worktrees") || strings.HasPrefix(at, "/objects") {
		return pl.GitDir + at
	}
	return pl.Root + at
}

// snapshot is what is at each of places now: a file's content, else what it is.
func snapshot(t *testing.T, places ...string) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, at := range places {
		if st, err := os.Lstat(at); err != nil {
			got[at] = "(not there)"
		} else if st.Mode().IsRegular() {
			got[at] = readKeptFile(t, at)
		} else {
			got[at] = "(" + st.Mode().String() + ")"
		}
	}
	return got
}

// The guard's look, in the desk's view, finds each change and stops: it touches nothing, whoever made it, and the
// version it found is still there, as found. It notes the place for the next start.
func TestTheGuardsLookTouchesNothingAndNotesWhatStoppedTheDesk(t *testing.T) {
	for _, c := range keptCases {
		t.Run(c.name, func(t *testing.T) {
			pl := keptDesk(t)
			held := KeptFolder(pl.Support, "h1")
			at := caseAt(pl, c.at)
			c.change(t, at)
			watched := []string{at, pl.Root + "/.claude/settings.json", pl.Root + "/.claude/hooks/stand-in.sh", pl.Root + "/.claude/commands/mine.md"}
			before := snapshot(t, watched...)
			done := make(chan struct{})
			var changes []KeptChange
			var err error
			go func() {
				if changes, err = CheckKept(held); err == nil {
					err = NoteKeptStop(held, changes)
				}
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("the check waits on a pipe")
			}
			if err != nil || len(changes) != 1 || changes[0].At != at || changes[0].What != c.what || changes[0].Branch || changes[0].Back || changes[0].Seal == "" {
				t.Fatalf("found %+v; want %s %s", changes, at, c.what)
			}
			if after := snapshot(t, watched...); !mapsEqual(before, after) {
				t.Fatalf("the guard's look changed what it found: %v, then %v", before, after)
			}
			if _, err := os.Lstat(foundFolder(pl)); err == nil {
				t.Fatal("the guard's look set something aside")
			}
			if stops := keptStops(held); !stops[at][changes[0].Seal] {
				t.Fatalf("not noted for the next start: %v", stops)
			}
			words := KeptWords(changes)
			if !strings.Contains(words, at) || !strings.Contains(words, "Nothing is undone here") || strings.Contains(words, "is put back") {
				t.Fatalf("the words: %s", words)
			}
		})
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// The core's put back, at a fire, a Take over or a Hand back, of what the desk's guard stopped it for: what it finds is
// set aside whole in the desk's found folder first, never removed, and the words give where; then the saved content
// comes back, never the desk's.
func TestAChangedKeptPlaceIsSetAsideWholeThenPutBack(t *testing.T) {
	for _, c := range keptCases {
		t.Run(c.name, func(t *testing.T) {
			pl := keptDesk(t)
			held := KeptFolder(pl.Support, "h1")
			at := caseAt(pl, c.at)
			saved := []string{pl.Root + "/.claude/settings.json", pl.Root + "/.claude/settings.local.json", pl.Root + "/.claude/commands/review.md",
				pl.Root + "/.git", pl.GitDir + "/worktrees/h1/commondir", pl.GitDir + "/worktrees/h1/gitdir", pl.GitDir + "/worktrees/h1/config.worktree"}
			before := snapshot(t, saved...)
			c.change(t, at)
			// What the core will find: each name below the place, as it is.
			var wasThere []string
			filepath.Walk(at, func(p string, _ os.FileInfo, _ error) error { wasThere = append(wasThere, p); return nil })
			found := snapshot(t, wasThere...)
			guardStops(t, pl)
			changes, err := RestoreKept(held)
			if err != nil || len(changes) != 1 || changes[0].At != at || !changes[0].Back {
				t.Fatalf("put back: %v, %+v", err, changes)
			}
			if after := snapshot(t, saved...); !mapsEqual(before, after) {
				t.Errorf("after it was put back: %v, want the saved %v", after, before)
			}
			for _, added := range []string{pl.Root + "/.claude/hooks", pl.Root + "/.claude/p", pl.GitDir + "/worktrees/h1/remotes/origin"} {
				if _, err := os.Lstat(added); err == nil {
					t.Errorf("%s, added, is still there", added)
				}
			}
			// What was found is all in the found folder, each name as it was.
			if c.what == "removed" {
				if changes[0].Found != "" {
					t.Errorf("nothing was there, yet set aside in %s", changes[0].Found)
				}
			} else {
				if !strings.HasPrefix(changes[0].Found, foundFolder(pl)+"/") || !strings.HasSuffix(changes[0].Found, at) {
					t.Fatalf("set aside in %q", changes[0].Found)
				}
				for p, was := range found {
					if aside := strings.Replace(p, at, changes[0].Found, 1); snapshot(t, aside)[aside] != was {
						t.Errorf("%s is %q where it was set aside, want %q", aside, snapshot(t, aside)[aside], was)
					}
				}
			}
			// The core's put back takes the new devices and inodes into the copy: the next look finds nothing.
			if again, err := CheckKept(held); err != nil || len(again) != 0 {
				t.Errorf("after it was put back: %v, %+v", err, again)
			}
			words := KeptWords(changes)
			if !strings.Contains(words, at) || !strings.Contains(words, changes[0].When.Format("15:04:05 MST")) ||
				!strings.Contains(words, "put back") || !strings.Contains(words, changes[0].Found) {
				t.Errorf("the words don't say where, when, what and where it was set aside: %s", words)
			}
		})
	}
}

// L1GUARD1FIRE: the core's put back at a fire, a Take over or a Hand back is only what the desk's guard stopped it for,
// still as it was then. The owner's own edit after the stop, and a place changed again after it, are left as they are
// and said; what the desk was stopped for is set aside whole and put back.
func TestTheCoresPutBackIsOnlyWhatTheDeskWasStoppedFor(t *testing.T) {
	pl := keptDesk(t)
	held := KeptFolder(pl.Support, "h1")
	reg := pl.GitDir + "/worktrees/h1"
	replaceByRename(t, reg+"/gitdir", "/elsewhere/.git\n")
	replaceByRename(t, reg+"/commondir", "../../stand-in\n")
	guardStops(t, pl)
	// After the stop: the owner's own edit, and the desk's commondir changed again.
	owners := pl.Root + "/.claude/settings.local.json"
	writeKeptFile(t, owners, `{"permissions":{"allow":["the owner's own"]}}`+"\n")
	replaceByRename(t, reg+"/commondir", "../../changed again\n")
	changes, err := RestoreKept(held)
	if err != nil || len(changes) != 3 {
		t.Fatalf("the core's put back: %v, %+v", err, changes)
	}
	for _, ch := range changes {
		if back := ch.At == reg+"/gitdir"; ch.Back != back || ch.Kept == back || (ch.Found != "") != back {
			t.Errorf("%s: put back %v, kept %v, set aside in %q", ch.At, ch.Back, ch.Kept, ch.Found)
		}
	}
	if got := readKeptFile(t, reg+"/gitdir") + readKeptFile(t, reg+"/commondir") + readKeptFile(t, owners); got !=
		pl.Root+"/.git\n"+"../../changed again\n"+`{"permissions":{"allow":["the owner's own"]}}`+"\n" {
		t.Fatalf("after the core's put back: %q", got)
	}
	if words := KeptWords(changes); strings.Count(words, "left as it is") != 2 || strings.Count(words, "set aside whole in") != 1 {
		t.Fatalf("the words: %s", words)
	}
	// Its note is used up: the next look puts nothing more back.
	if again, err := RestoreKept(held); err != nil || slices.ContainsFunc(again, func(ch KeptChange) bool { return ch.Back }) || len(again) != 2 {
		t.Fatalf("the next look: %v, %+v", err, again)
	}
}

// The repo's shared objects/info, alternates included, is never put back, by the core or at a start: a change stops
// the desk and is shown, and the next start takes it as it is.
func TestTheSharedObjectsInfoIsNeverPutBack(t *testing.T) {
	pl := keptDesk(t)
	held := KeptFolder(pl.Support, "h1")
	info := pl.GitDir + "/objects/info"
	writeKeptFile(t, info+"/alternates", "/elsewhere/objects\n")
	replaceByRename(t, info+"/packs", "P pack-2.pack\n")
	changes := guardStops(t, pl)
	if len(changes) != 2 || !changes[0].Repo || !changes[1].Repo {
		t.Fatalf("found %+v", changes)
	}
	if words := KeptWords(changes); !strings.Contains(words, "never put back") || strings.Contains(words, "Nothing is undone here") {
		t.Fatalf("the words: %s", words)
	}
	if stops := keptStops(held); len(stops) != 0 {
		t.Fatalf("noted, though it is never put back: %v", stops)
	}
	// Even a note that names it, as found, puts nothing back.
	forged := strings.Join([]string{`{"At":"` + info + `/alternates","Seal":"` + keptSeal(info+"/alternates", nil) + `"}`,
		`{"At":"` + info + `/packs","Seal":"` + keptSeal(info+"/packs", nil) + `"}`}, "\n") + "\n"
	writeKeptFile(t, keptStopFile(held), forged)
	if changes, err := RestoreKept(held); err != nil || len(changes) != 2 || changes[0].Back || changes[1].Back {
		t.Fatalf("the core's put back: %v, %+v", err, changes)
	}
	if err := SaveKept(pl); err != nil {
		t.Fatalf("the next start: %v", err)
	}
	if got := readKeptFile(t, info+"/alternates") + readKeptFile(t, info+"/packs"); got != "/elsewhere/objects\nP pack-2.pack\n" {
		t.Fatalf("objects/info is %q", got)
	}
	if _, err := os.Lstat(foundFolder(pl)); err == nil {
		t.Fatal("something was set aside")
	}
	if news := readKeptFile(t, KeptNewsFile(pl.Support, "h1")); !strings.Contains(news, info+"/alternates") || !strings.Contains(news, "kept as the desk's new copy") {
		t.Fatalf("not listed for Setup: %s", news)
	}
}

// P2 (RT126L1GUARD1): two desks on one repo, the owner's gc between their starts: neither undoes the other, nor the gc.
func TestTwoDesksOnOneRepoNeverUndoEachOther(t *testing.T) {
	base, _ := filepath.EvalSymlinks(t.TempDir())
	a, b := keptDeskIn(t, base, "a"), keptDeskIn(t, base, "b")
	info := base + "/app/.git/objects/info"
	writeKeptFile(t, info+"/commit-graph", "stand-in graph\n")
	replaceByRename(t, info+"/packs", "P pack-2.pack\n")
	// Desk a, still running, is stopped and shown; nothing is touched.
	if changes := guardStops(t, a); len(changes) != 2 || !changes[0].Repo {
		t.Fatalf("desk a's guard: %+v", changes)
	}
	for round, pl := range []Places{a, b, a, b} {
		if err := SaveKept(pl); err != nil {
			t.Fatalf("round %d, desk %s starts: %v", round, pl.OwnID, err)
		}
		if got := readKeptFile(t, info+"/commit-graph") + readKeptFile(t, info+"/packs"); got != "stand-in graph\nP pack-2.pack\n" {
			t.Fatalf("round %d, desk %s starts: objects/info is %q", round, pl.OwnID, got)
		}
	}
	for _, pl := range []Places{a, b} {
		if changes, err := CheckKept(KeptFolder(pl.Support, pl.OwnID)); err != nil || len(changes) != 0 {
			t.Fatalf("desk %s after its start: %v, %+v", pl.OwnID, err, changes)
		}
	}
}

// The next start puts back only what the last session was stopped for, still as it was found: here the desk's gitdir.
// The owner's own setting, changed while no desk ran, is kept as it is, the new copy, and listed for Setup.
func TestTheNextStartPutsBackOnlyWhatTheDeskWasStoppedFor(t *testing.T) {
	pl := keptDesk(t)
	held := KeptFolder(pl.Support, "h1")
	at := pl.GitDir + "/worktrees/h1/gitdir"
	saved := readKeptFile(t, at)
	replaceByRename(t, at, "/elsewhere/.git\n")
	if changes := guardStops(t, pl); len(changes) != 1 || changes[0].At != at {
		t.Fatalf("the guard: %+v", changes)
	}
	owners := pl.Root + "/.claude/settings.local.json"
	replaceByRename(t, owners, `{"permissions":{"allow":["Bash(make test)"]}}`+"\n")
	err := SaveKept(pl)
	if !errors.Is(err, ErrKeptChanged) || !strings.Contains(err.Error(), at) || strings.Contains(err.Error(), owners) ||
		!strings.Contains(err.Error(), "set aside whole in") {
		t.Fatalf("the next start: %v", err)
	}
	if got := readKeptFile(t, at); got != saved {
		t.Fatalf("gitdir is %q, want the saved %q", got, saved)
	}
	asides, _ := filepath.Glob(foundFolder(pl) + "/*" + at)
	if len(asides) != 1 || readKeptFile(t, asides[0]) != "/elsewhere/.git\n" {
		t.Fatalf("the desk's gitdir set aside: %v", asides)
	}
	if got := readKeptFile(t, owners); got != `{"permissions":{"allow":["Bash(make test)"]}}`+"\n" {
		t.Fatalf("the owner's setting is %q", got)
	}
	if news := readKeptFile(t, KeptNewsFile(pl.Support, "h1")); !strings.Contains(news, owners) || strings.Contains(news, at) {
		t.Fatalf("listed for Setup: %s", news)
	}
	if got := readKeptFile(t, keptStopFile(held)); got != "" {
		t.Fatalf("the notes after the start: %q", got)
	}
	// The start after it goes ahead, the owner's setting in its copy; a branch folder made again isn't counted.
	branch := pl.GitDir + "/logs/refs/heads/hire/h1"
	os.RemoveAll(branch)
	os.Mkdir(branch, 0o700)
	if err := SaveKept(pl); err != nil {
		t.Fatalf("the start after it: %v", err)
	}
	if got := readKeptFile(t, owners); !strings.Contains(got, "make test") {
		t.Fatalf("the owner's setting is %q", got)
	}
}

// P1 (RT126L1GUARD1): a desk with no worktree of its own, in the owner's project; the owner's own Claude Code rewrites
// settings.local.json and adds a command between two starts. The next start keeps both, whole, and lists them.
func TestTheOwnersOwnClaudeCodeBetweenSessionsIsKept(t *testing.T) {
	base, _ := filepath.EvalSymlinks(t.TempDir())
	pl := Places{GOOS: "linux", Root: base + "/app", GitDir: base + "/app/.git", OwnID: "h1", Support: base + "/support"}
	writeKeptFile(t, pl.Root+"/.claude/settings.local.json", `{"permissions":{}}`+"\n")
	writeKeptFile(t, pl.GitDir+"/objects/info/packs", "")
	os.MkdirAll(pl.Support+"/hires/h1", 0o700)
	if err := SaveKept(pl); err != nil {
		t.Fatal(err)
	}
	big := `{"permissions":{"allow":["` + strings.Repeat("Bash(npm run x)", 200) + `"]}}` + "\n"
	replaceByRename(t, pl.Root+"/.claude/settings.local.json", big)
	writeKeptFile(t, pl.Root+"/.claude/commands/owners-new.md", "The owner's own command.\n")
	if err := SaveKept(pl); err != nil {
		t.Fatalf("the next start: %v", err)
	}
	if got := readKeptFile(t, pl.Root+"/.claude/settings.local.json"); got != big || len(big) < 3000 {
		t.Fatalf("the owner's %d bytes are %d bytes", len(big), len(got))
	}
	if got := readKeptFile(t, pl.Root+"/.claude/commands/owners-new.md"); got != "The owner's own command.\n" {
		t.Fatalf("the owner's command is %q", got)
	}
	news := readKeptFile(t, KeptNewsFile(pl.Support, "h1"))
	if !strings.Contains(news, "settings.local.json") || !strings.Contains(news, "owners-new.md") {
		t.Fatalf("not listed for Setup: %s", news)
	}
	if _, err := os.Lstat(foundFolder(pl)); err == nil {
		t.Fatal("something was set aside")
	}
}

// A place changed again after the desk was stopped for it isn't what it was stopped for: it is kept as it is, and
// listed. A note that isn't one, or a seal that names nothing there, puts nothing back either.
func TestAPlaceChangedAgainAfterTheStopIsKept(t *testing.T) {
	pl := keptDesk(t)
	held := KeptFolder(pl.Support, "h1")
	at := pl.GitDir + "/worktrees/h1/config.worktree"
	replaceByRename(t, at, "[core]\n\tfsmonitor = ./stand-in\n")
	guardStops(t, pl)
	replaceByRename(t, at, "[core]\n\tsparseCheckout = true\n") // the owner's, after the stop
	settings := pl.Root + "/.claude/settings.json"
	replaceByRename(t, settings, "{\"theirs\":true}\n")
	f, _ := os.OpenFile(keptStopFile(held), os.O_WRONLY|os.O_APPEND, 0)
	f.WriteString("not a note\n{\"At\":\"" + settings + "\",\"Seal\":\"0123\"}\n{\"At\":\"\",\"Seal\":\"\"}\n")
	f.Close()
	if err := SaveKept(pl); err != nil {
		t.Fatalf("the next start: %v", err)
	}
	if got := readKeptFile(t, at) + readKeptFile(t, settings); got != "[core]\n\tsparseCheckout = true\n{\"theirs\":true}\n" {
		t.Fatalf("put back: %q", got)
	}
	if news := readKeptFile(t, KeptNewsFile(pl.Support, "h1")); !strings.Contains(news, at) || !strings.Contains(news, settings) {
		t.Fatalf("not listed: %s", news)
	}
}

// A note lost (the desk emptied the file) leaves the change as it is, never silent: it is listed for Setup.
func TestANoteLostLeavesTheChangeListed(t *testing.T) {
	pl := keptDesk(t)
	at := pl.Root + "/.claude/hooks"
	writeKeptFile(t, at+"/stand-in.sh", "true\n")
	guardStops(t, pl)
	os.Truncate(keptStopFile(KeptFolder(pl.Support, "h1")), 0)
	if err := SaveKept(pl); err != nil {
		t.Fatalf("the next start: %v", err)
	}
	if news := readKeptFile(t, KeptNewsFile(pl.Support, "h1")); !strings.Contains(news, at) || !strings.Contains(news, `+ "stand-in.sh"`) {
		t.Fatalf("not listed: %s", news)
	}
}

// The owner sees what changed: the lines taken away and the lines added, the place quoted, the time with its zone.
func TestTheWordsShowTheChangeLineByLine(t *testing.T) {
	pl := keptDesk(t)
	at := pl.GitDir + "/worktrees/h1/config.worktree"
	writeKeptFile(t, at, "[core]\n\tfsmonitor = ./stand-in\n")
	guardStops(t, pl)
	changes, err := RestoreKept(KeptFolder(pl.Support, "h1"))
	if err != nil || len(changes) != 1 {
		t.Fatal(err, changes)
	}
	words := KeptWords(changes)
	if !strings.Contains(words, `+ "\tfsmonitor = ./stand-in"`) || !strings.Contains(words, `+ "[core]"`) {
		t.Fatalf("the lines added aren't shown: %s", words)
	}
	if zone := changes[0].When.Format("MST"); !strings.Contains(words, ":"+changes[0].When.Format("05")+" "+zone+" on ") {
		t.Fatalf("the time has no zone: %s", words)
	}
	writeKeptFile(t, pl.Root+"/.claude/settings.json", "{}\n")
	guardStops(t, pl)
	changes, _ = RestoreKept(KeptFolder(pl.Support, "h1"))
	if words = KeptWords(changes); !strings.Contains(words, `- "{\"hooks\":{}}"`) || !strings.Contains(words, `+ "{}"`) {
		t.Fatalf("the lines taken away aren't shown: %s", words)
	}
	// A name added by whoever wrote there can't break the list: it is quoted.
	writeKeptFile(t, pl.Root+"/.claude/a\n- \"elsewhere\", seen", "x")
	changes, _ = CheckKept(KeptFolder(pl.Support, "h1"))
	if words = KeptWords(changes); strings.Count(words, "\n- ") != 1 || !strings.Contains(words, `a\n- `) {
		t.Fatalf("a name with a line break: %s", words)
	}
}

// F1: the desk's branch folders are watched by what they are, not what is in them: its commits are its own, but a
// folder the owner's git removed and made again stops the desk with words to restart, and nothing is put back.
func TestAReplacedBranchFolderStopsTheDeskWithWordsToRestart(t *testing.T) {
	pl := keptDesk(t)
	held := KeptFolder(pl.Support, "h1")
	branch := pl.GitDir + "/refs/heads/hire/h1"
	writeKeptFile(t, branch+"/second", "4567\n")
	os.Remove(branch + "/work")
	os.Chmod(branch, 0o750)
	if changes, err := CheckKept(held); err != nil || len(changes) != 0 {
		t.Fatalf("the desk's own commits stop it: %v, %+v", err, changes)
	}
	// As pack-refs does: the loose branches packed, the empty folder removed, then made again by the next branch.
	if err := os.Mkdir(branch+".new", 0o700); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(branch)
	os.Rename(branch+".new", branch)
	changes, err := RestoreKept(held)
	if err != nil || len(changes) != 1 || changes[0].At != branch || !changes[0].Branch || changes[0].Back {
		t.Fatalf("found %v, %+v", err, changes)
	}
	if words := KeptWords(changes); !strings.Contains(words, "restart this hire to keep committing") || strings.Contains(words, "put back") {
		t.Fatalf("the words: %s", words)
	}
	if _, err := os.Lstat(branch + "/work"); err == nil {
		t.Fatal("a branch was put back")
	}
	// It stays found until the desk restarts, and isn't noted.
	if again := guardStops(t, pl); len(again) != 1 || len(keptStops(held)) != 0 {
		t.Fatalf("found once only, or noted: %+v", again)
	}
}

// Fails closed: no copy, or one that can't be read, is an error the guard stops on, never "all is well".
func TestNoCopyIsNoAllClear(t *testing.T) {
	if _, err := CheckKept(filepath.Join(t.TempDir(), "held")); err == nil {
		t.Fatal("no copy, no error")
	}
	pl := keptDesk(t)
	held := KeptFolder(pl.Support, "h1")
	os.WriteFile(filepath.Join(held, keptList), []byte("{"), 0o600)
	if _, err := CheckKept(held); err == nil {
		t.Fatal("a broken list, no error")
	}
	if _, err := RestoreKept(held); err == nil {
		t.Fatal("a broken list, no error")
	}
}

// Nothing is put back through a folder above it that isn't the one saved (a link swapped in): it says so instead,
// and nothing is set aside.
func TestNothingIsPutBackThroughAFolderSwappedAbove(t *testing.T) {
	pl := keptDesk(t)
	held := KeptFolder(pl.Support, "h1")
	commands := pl.Root + "/.claude/commands"
	writeKeptFile(t, commands+"/review.md", "changed\n")
	changes, saved, err := compareKept(held, true)
	if err != nil || len(changes) != 1 {
		t.Fatal(err, changes)
	}
	elsewhere := t.TempDir()
	writeKeptFile(t, elsewhere+"/review.md", "the owner's own\n")
	os.RemoveAll(commands)
	os.Symlink(elsewhere, commands)
	if _, _, err := putBack(held, foundFolder(pl)+"/now", changes[0], saved); err == nil {
		t.Fatal("put back through a link")
	}
	if got := readKeptFile(t, elsewhere+"/review.md"); got != "the owner's own\n" {
		t.Fatalf("the file the link leads to was changed: %q", got)
	}
	if _, err := os.Lstat(foundFolder(pl)); err == nil {
		t.Fatal("set aside through a link")
	}
}

// What is set aside is set aside whole: on another disk it is copied, each file, folder and link as it is, and only
// then removed. What can't be copied (a pipe) leaves it all where it is, and nothing is ever set aside over another.
func TestWhatCantBeSetAsideWholeIsLeftAsItIs(t *testing.T) {
	base, _ := filepath.EvalSymlinks(t.TempDir())
	at := base + "/place"
	writeKeptFile(t, at+"/a.json", "{}\n")
	writeKeptFile(t, at+"/sub/b.md", "b\n")
	os.Symlink("/elsewhere", at+"/link")
	dir, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	most := keptMost
	if err := copyAside(dir, "place", dir, "aside", &most); err != nil {
		t.Fatal(err)
	}
	if got := readKeptFile(t, base+"/aside/a.json") + readKeptFile(t, base+"/aside/sub/b.md"); got != "{}\nb\n" {
		t.Fatalf("copied %q", got)
	}
	if to, _ := os.Readlink(base + "/aside/link"); to != "/elsewhere" {
		t.Fatalf("the link copied as %q", to)
	}
	syscall.Mkfifo(at+"/p", 0o600)
	most = keptMost
	if err := copyAside(dir, "place", dir, "aside2", &most); err == nil {
		t.Fatal("a pipe copied")
	}
	if err := setAside(dir, "place", base+"/aside"); err == nil {
		t.Fatal("set aside over what was set aside before")
	}
	if got := readKeptFile(t, at+"/a.json"); got != "{}\n" {
		t.Fatalf("the place is %q", got)
	}
}

// A saved name that can't be made again (a pipe in a kept folder) fails the start with what the owner can do.
func TestAPutBackThatFailsSaysWhatToDo(t *testing.T) {
	pl := keptDesk(t)
	syscall.Mkfifo(pl.Root+"/.claude/p", 0o600)
	if err := SaveKept(pl); err != nil {
		t.Fatal(err)
	}
	claude := pl.Root + "/.claude"
	writeKeptFile(t, claude+".new/settings.json", "{}")
	os.RemoveAll(claude)
	os.Rename(claude+".new", claude)
	guardStops(t, pl)
	err := SaveKept(pl)
	if err == nil || !strings.Contains(err.Error(), "Move that place aside yourself") {
		t.Fatalf("the next start: %v", err)
	}
	// The owner moves it aside: the next start keeps what is there then.
	os.Rename(claude, claude+".mine")
	if err := SaveKept(pl); err != nil {
		t.Fatalf("after the owner moved it: %v", err)
	}
}

// A start that stopped between setting the last copy aside and the new one taking its name still finds the last one.
func TestAStartStoppedBetweenItsRenamesFindsItsLastCopy(t *testing.T) {
	pl := keptDesk(t)
	held := KeptFolder(pl.Support, "h1")
	at := pl.GitDir + "/worktrees/h1/commondir"
	replaceByRename(t, at, "../../stand-in\n")
	guardStops(t, pl)
	if err := os.Rename(held, held+".old"); err != nil {
		t.Fatal(err)
	}
	if err := SaveKept(pl); !errors.Is(err, ErrKeptChanged) {
		t.Fatalf("the next start: %v", err)
	}
	if got := readKeptFile(t, at); got != "../..\n" {
		t.Fatalf("commondir is %q", got)
	}
}

// RT126L1GUARD1FIRE B2 (probe PB): a put back whose saved copy can't be made again once what was there is set aside.
// The stand-in for a full disk at the write is the copy's content gone from held, so the re-make fails at the same
// step. What it made is taken away and the desk's version goes back in its place, as it was found; the words say so,
// and the next look tries again rather than going on: a fire, a Hand back and a start refuse until it can. When the
// desk's version can't go back (its found folder on another disk: renames fail), the words give where it is set aside
// whole, and the next look puts the copy back. The start's SaveKept says the same, the found path in its error.
func TestAPutBackThatFailsAfterTheSetAsideSaysWhereAndTriesAgain(t *testing.T) {
	desk := `{"hooks":{"the desk's":1}}` + "\n"
	stopped := func(t *testing.T) (Places, string, func(), func()) {
		pl := keptDesk(t)
		held := KeptFolder(pl.Support, "h1")
		claude := pl.Root + "/.claude"
		writeKeptFile(t, claude+".desk/settings.json", desk)
		os.RemoveAll(claude)
		os.Rename(claude+".desk", claude)
		guardStops(t, pl)
		content := filepath.Join(held, fmt.Sprintf("%x", sha256.Sum256([]byte("Review it.\n"))))
		if _, err := os.Lstat(content); err != nil {
			t.Fatalf("the copy's content: %v", err)
		}
		full := func() { os.Rename(content, content+".gone") }
		room := func() { os.Rename(content+".gone", content) }
		full()
		return pl, claude, full, room
	}
	aside := func(pl Places, at string) []string {
		found, _ := filepath.Glob(foundFolder(pl) + "/*" + at)
		return found
	}

	t.Run("put back as it was found", func(t *testing.T) {
		pl, claude, _, room := stopped(t)
		held := KeptFolder(pl.Support, "h1")
		for try := 1; try <= 2; try++ {
			changes, err := RestoreKept(held)
			if err == nil || !strings.Contains(err.Error(), "couldn't be made again") || !strings.Contains(err.Error(), "the place is as it was found") ||
				len(changes) != 1 || changes[0].Back || changes[0].Found != "" {
				t.Fatalf("try %d: %v, %+v", try, err, changes)
			}
			if got := readKeptFile(t, claude+"/settings.json"); got != desk || len(aside(pl, claude)) != 0 {
				t.Fatalf("try %d: .claude has %q, set aside %v", try, got, aside(pl, claude))
			}
		}
		room()
		changes, err := RestoreKept(held)
		if err != nil || len(changes) != 1 || !changes[0].Back || readKeptFile(t, claude+"/commands/review.md") != "Review it.\n" {
			t.Fatalf("with room again: %v, %+v", err, changes)
		}
		if found := aside(pl, claude); len(found) != 1 || readKeptFile(t, found[0]+"/settings.json") != desk {
			t.Fatalf("the desk's .claude set aside: %v", found)
		}
	})

	t.Run("set aside elsewhere", func(t *testing.T) {
		pl, claude, _, room := stopped(t)
		held := KeptFolder(pl.Support, "h1")
		onAnotherDisk(t)
		changes, err := RestoreKept(held)
		found := aside(pl, claude)
		if err == nil || len(found) != 1 || readKeptFile(t, found[0]+"/settings.json") != desk || len(changes) != 1 ||
			changes[0].Back || changes[0].Found != found[0] {
			t.Fatalf("%v, %+v, set aside %v", err, changes, found)
		}
		for _, words := range []string{err.Error(), KeptWords(changes)} {
			if !strings.Contains(words, "set aside whole in "+`"`+found[0]+`"`) || !strings.Contains(words, "not there") ||
				strings.Contains(words, "Nothing is undone here") {
				t.Errorf("the words: %s", words)
			}
		}
		// Not silently gone on with: the next look puts back again, and fails again while there is no room.
		if _, err := RestoreKept(held); err == nil {
			t.Fatal("the next look went on")
		}
		keptRename = renameKept
		room()
		changes, err = RestoreKept(held)
		if err != nil || len(changes) != 1 || !changes[0].Back || readKeptFile(t, claude+"/commands/review.md") != "Review it.\n" {
			t.Fatalf("with room again: %v, %+v", err, changes)
		}
	})

	t.Run("at the start", func(t *testing.T) {
		pl, claude, _, room := stopped(t)
		onAnotherDisk(t)
		err := SaveKept(pl)
		found := aside(pl, claude)
		if err == nil || errors.Is(err, ErrKeptChanged) || len(found) != 1 || !strings.Contains(err.Error(), "Move that place aside yourself") ||
			strings.Count(err.Error(), "set aside whole in "+`"`+found[0]+`"`) != 2 {
			t.Fatalf("the start: %v, set aside %v", err, found)
		}
		if err := SaveKept(pl); err == nil {
			t.Fatal("the next start went on")
		}
		keptRename = renameKept
		room()
		if err := SaveKept(pl); !errors.Is(err, ErrKeptChanged) || readKeptFile(t, claude+"/commands/review.md") != "Review it.\n" {
			t.Fatalf("with room again: %v", err)
		}
		if err := SaveKept(pl); err != nil {
			t.Fatalf("the start after it: %v", err)
		}
	})
}

// FXKEPTFD (RT126RESOLVESWEEP S3, S3b): a desk can write and swap anything below its .claude, and the core puts back
// while its session is still up. So a folder in the middle may be swapped for a link to the owner's own place between
// the core's look and what it does next (keptBetween is that moment, deterministic). The core never moves, removes,
// writes or shows anything there: it acts and reads only in the folders it checked, held open.
func TestNothingIsDoneOrShownThroughAFolderSwappedInTheMiddle(t *testing.T) {
	// A stand-in owner's place, and a desk whose .claude/x/.bashrc an earlier start took into its copy as it was.
	setup := func(t *testing.T) (pl Places, home, x string) {
		pl = keptDesk(t)
		home, _ = filepath.EvalSymlinks(t.TempDir())
		writeKeptFile(t, home+"/.bashrc", "the owner's own\n")
		writeKeptFile(t, home+"/owner-only.txt", "the owner's own\n")
		x = pl.Root + "/.claude/x"
		writeKeptFile(t, x+"/.bashrc", "the desk's, kept\n")
		if err := SaveKept(pl); err != nil {
			t.Fatal(err)
		}
		return pl, home, x
	}
	// swapAt is the desk's swap of folder for a link to home, once, at step.
	swapAt := func(t *testing.T, step, folder, home string) {
		done := false
		keptBetween = func(s, _ string) {
			if s == step && !done {
				done = true
				if err := errors.Join(os.Rename(folder, folder+".away"), os.Symlink(home, folder)); err != nil {
					t.Error(err)
				}
			}
		}
		t.Cleanup(func() { keptBetween = func(string, string) {} })
	}
	untouched := func(t *testing.T, pl Places, home string) {
		t.Helper()
		if got := snapshot(t, home+"/.bashrc", home+"/owner-only.txt"); got[home+"/.bashrc"] != "the owner's own\n" || got[home+"/owner-only.txt"] != "the owner's own\n" {
			t.Errorf("the owner's place was changed: %v", got)
		}
		if names, _ := os.ReadDir(home); len(names) != 2 {
			t.Errorf("the owner's place holds %d names", len(names))
		}
		filepath.Walk(foundFolder(pl), func(p string, info os.FileInfo, _ error) error {
			if info != nil && info.Mode().IsRegular() && readKeptFile(t, p) == "the owner's own\n" {
				t.Errorf("the owner's file was set aside in %s", p)
			}
			return nil
		})
	}

	for _, disk := range []string{"one disk", "another disk"} {
		t.Run("put back, "+disk, func(t *testing.T) {
			pl, home, x := setup(t)
			writeKeptFile(t, x+"/.bashrc", "the desk's, changed\n")
			guardStops(t, pl)
			if disk == "another disk" {
				onAnotherDisk(t)
			}
			swapAt(t, "put back", x, home)
			changes, err := RestoreKept(KeptFolder(pl.Support, "h1"))
			untouched(t, pl, home)
			// It was put back in the folder it checked and held, wherever the desk moved it: the desk's own.
			if err != nil || len(changes) != 1 || !changes[0].Back || readKeptFile(t, x+".away/.bashrc") != "the desk's, kept\n" ||
				readKeptFile(t, changes[0].Found) != "the desk's, changed\n" {
				t.Errorf("put back: %v, %+v", err, changes)
			}
		})
	}

	t.Run("a file shown", func(t *testing.T) {
		pl, home, x := setup(t)
		writeKeptFile(t, x+"/.bashrc", "the desk's, changed\n")
		swapAt(t, "show", x, home)
		changes, err := RestoreKept(KeptFolder(pl.Support, "h1"))
		if words := KeptWords(changes); err != nil || strings.Contains(words, "the owner's own") {
			t.Errorf("the owner's file shown: %v, %s", err, words)
		}
		untouched(t, pl, home)
	})

	t.Run("a folder shown", func(t *testing.T) {
		pl, home, _ := setup(t)
		added := pl.Root + "/.claude/added"
		writeKeptFile(t, added+"/the-desks.txt", "the desk's\n")
		swapAt(t, "show", added, home)
		changes, err := RestoreKept(KeptFolder(pl.Support, "h1"))
		if words := KeptWords(changes); err != nil || strings.Contains(words, "owner-only") || !strings.Contains(words, "the-desks.txt") {
			t.Errorf("the owner's names shown, or not the desk's: %v, %s", err, words)
		}
		untouched(t, pl, home)
	})
}
