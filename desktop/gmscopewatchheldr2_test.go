//go:build !windows

package desktop

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// RT126WATCHHELDR1 N1, N2 and O2: a held GM's file watch opens its file through a folder the GM's desks can't rename,
// and reads only while the GM is still held to its project; the app's own STATUS.md watches are made only where the
// path below a hire's place is plain.

// editRegistry changes the registry's hires as change says.
func editRegistry(t *testing.T, change func([]Hire) []Hire) {
	t.Helper()
	file := readRegistry(registryPath(), false)
	file.hires = change(file.hires)
	if err := file.save(registryPath()); err != nil {
		t.Fatal(err)
	}
}

// holdGM holds the registry's GM (g00001) to scope from now on; "" for none.
func holdGM(t *testing.T, scope string) {
	t.Helper()
	editRegistry(t, func(hires []Hire) []Hire {
		for i := range hires {
			if hires[i].ID == "g00001" {
				hires[i].Scope = scope
			}
		}
		return hires
	})
}

// The reviewer's pR, with no hook: proj/sub is an open hire's folder, listed before the project's hires, so the look
// finds the watched file in that root. A held watch's file is opened through the project instead, and with proj/sub
// already a link to Bea's elsewhere/sub, the walk from there refuses it.
func TestAHeldGMsFileIsOpenedThroughItsProjectNotAFolderInsideIt(t *testing.T) {
	d, gm, _, proj, elsewhere := heldNames(t)
	watchFiles(t, proj, elsewhere)
	sub := filepath.Join(proj, "sub")
	for _, dir := range []string{sub, filepath.Join(elsewhere, "sub")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	appendLine(t, filepath.Join(elsewhere, "sub", "later.txt"), "bea's existing line")
	editRegistry(t, func(hires []Hire) []Hire {
		return slices.Insert(hires, 0, Hire{ID: "su0001", Name: "Subway", Role: "janitor", Folder: keptPath(sub), State: "idle", Started: 1761000050000})
	})
	held := gmScopeOf(gm)
	id := heldWatch(t, d, gm, filepath.Join(sub, "later.txt"), callerGM, &held)
	x, ok := d.watchers.get(id)
	if !ok || x.Held != proj {
		t.Fatalf("the watch %s: %+v, want it held to %s", id, x, proj)
	}
	path, base, ok := watchInsideRoot(x.Target, d.watchers.roots(), d.watchers.home())
	if !ok || base != sub {
		t.Fatalf("the look finds %s in %q (%v), want the root %s", x.Target, base, ok, sub)
	}
	chosen, ok := watchOpenBase(x, path, base)
	if !ok || chosen != proj {
		t.Errorf("a held watch's file is opened through %q (%v), want the project %s", chosen, ok, proj)
	}
	linkAside(t, sub, filepath.Join(elsewhere, "sub"))
	if f, _ := openPlainFile(chosen, x.Target); f != nil {
		f.Close()
		t.Errorf("openPlainFile through %s with %s a link opened a file", chosen, sub)
	}
	if f, _, err := openHireFile(chosen, x.Target); err == nil {
		f.Close()
		t.Errorf("the walk from %s went through the link %s", chosen, sub)
	}
}

// A held watch on a file in a worktree of the project's is opened through the worktree's top, as the project's git
// registered it, and one in a folder the project never registered through nothing. The owner's and an unheld GM's
// watches keep the root they were found in.
func TestAHeldGMsWorktreeFileIsOpenedThroughTheWorktreesTop(t *testing.T) {
	proj := physical(t.TempDir())
	hiresFolder := filepath.Join(filepath.Dir(proj), filepath.Base(proj)+"-hires")
	top := filepath.Join(hiresFolder, "w1")
	registration := filepath.Join(proj, ".git", "worktrees", "w1")
	for _, dir := range []string{registration, filepath.Join(top, "deep"), filepath.Join(hiresFolder, "w2")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { os.RemoveAll(hiresFolder) })
	if err := os.WriteFile(filepath.Join(registration, "gitdir"), []byte(filepath.Join(top, ".git")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	held := watch{Kind: "file", Held: proj}
	for _, c := range []struct {
		path, want string
		ok         bool
	}{
		{filepath.Join(top, "deep", "log.txt"), top, true},
		{filepath.Join(top, "log.txt"), top, true},
		{filepath.Join(proj, "deep", "log.txt"), proj, true},
		{filepath.Join(hiresFolder, "w2", "log.txt"), "", false},
	} {
		if got, ok := watchOpenBase(held, c.path, "/the/root/found"); got != c.want || ok != c.ok {
			t.Errorf("held, %s: %q %v, want %q %v", c.path, got, ok, c.want, c.ok)
		}
		if got, ok := watchOpenBase(watch{Kind: "file"}, c.path, "/the/root/found"); got != "/the/root/found" || !ok {
			t.Errorf("not held, %s: %q %v, want the root found", c.path, got, ok)
		}
	}
}

// The reviewer's pS: a held watch stops once the owner holds the GM to another project, or to none, and reads again
// while the GM is held to its project or a folder that holds it.
func TestAHeldGMsFileWatchStopsOnceTheGMIsHeldElsewhereOrUnheld(t *testing.T) {
	d, gm, _, proj, elsewhere := heldNames(t)
	watchFiles(t, proj, elsewhere)
	held := gmScopeOf(gm)
	file := filepath.Join(proj, "real", "log.txt")
	id := heldWatch(t, d, gm, file, callerGM, &held)
	for i, c := range []struct {
		scope string
		reads bool
	}{
		{elsewhere, false},
		{"", false},
		{filepath.Dir(proj), true},
		{elsewhere, false},
		{proj, true},
	} {
		holdGM(t, c.scope)
		appendLine(t, file, "line "+string(rune('A'+i)))
		fires, read := lookOnce(d.watchers)
		got := firedFor(fires, id)
		if c.reads != (len(got) == 1 && strings.HasPrefix(got[0], "log.txt: line ")) || c.reads != readFor(read, id) {
			t.Errorf("look %d, the GM held to %q: fired %q, read further %v; want reading %v", i+1, c.scope, got, readFor(read, id), c.reads)
		}
	}
}

// builtinTargets are the targets of the app's own file watches of hire.
func builtinTargets(d *Deck, hire string) []string {
	d.watchers.mu.Lock()
	defer d.watchers.mu.Unlock()
	var out []string
	for _, x := range d.watchers.all {
		if x.Builtin != "" && x.Kind == "file" && x.Hire == hire {
			out = append(out, x.Builtin+" "+x.Target)
		}
	}
	slices.Sort(out)
	return out
}

// builtinsOnFor turns the app's own watches on for d, and catches what the watchers log.
func builtinsOnFor(t *testing.T, d *Deck) *bytes.Buffer {
	t.Helper()
	d.mu.Lock()
	d.config.BuiltinWatches = true
	d.mu.Unlock()
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &logged
}

// The reviewer's pBI: proj/.team is made a link to Bea's elsewhere/notes. No built-in of Ada's (or anyone's) watches
// Bea's STATUS.md there, and a quiet log line says so once.
func TestNoBuiltinWatchesAStatusFileBehindALinkInAHiresPlace(t *testing.T) {
	d, _, _, proj, elsewhere := heldNames(t)
	notes := filepath.Join(elsewhere, "notes")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatal(err)
	}
	appendLine(t, filepath.Join(notes, "STATUS.md"), "DONE: bea's line")
	if err := os.Symlink(notes, filepath.Join(proj, ".team")); err != nil {
		t.Fatal(err)
	}
	logged := builtinsOnFor(t, d)
	d.watchers.syncBuiltins()
	d.watchers.syncBuiltins()
	for _, hire := range []string{"ad0001", "cy0001", "be0001"} {
		for _, x := range builtinTargets(d, hire) {
			if strings.Contains(x, notes) {
				t.Errorf("%s's built-in %s watches Bea's notes", hire, x)
			}
		}
	}
	// And the STATUS.md in Ada's own folder in the support folder (FXSWEEP3P3R1 B2).
	own := plainPath(filepath.Join(hireFolder(supportDir, "ad0001"), "STATUS.md"))
	want := []string{"needs-review " + filepath.Join(proj, "STATUS.md"), "status " + filepath.Join(proj, "STATUS.md"), "needs-review " + own, "status " + own}
	slices.Sort(want)
	if got := builtinTargets(d, "ad0001"); !slices.Equal(got, want) {
		t.Errorf("Ada's built-ins: %q, want only %q", got, want)
	}
	line := "watchers: no built-in watch on " + filepath.Join(proj, ".team", "STATUS.md") + ": a link is on its way"
	if n := strings.Count(logged.String(), line); n != 1 {
		t.Errorf("the log says %q %d times, want once:\n%s", line, n, logged.String())
	}
}

// A plain .team still gets its built-ins, and so does a hire whose place is reached through the owner's own link.
func TestABuiltinStillWatchesAPlainTeamFolderAndAPlaceThroughTheOwnersLink(t *testing.T) {
	d, _, _, proj, _ := heldNames(t)
	if err := os.MkdirAll(filepath.Join(proj, ".team"), 0o755); err != nil {
		t.Fatal(err)
	}
	owners := filepath.Join(filepath.Dir(proj), "proj-link")
	if err := os.Symlink(proj, owners); err != nil {
		t.Fatal(err)
	}
	editRegistry(t, func(hires []Hire) []Hire {
		return append(hires, Hire{ID: "de0001", Name: "Delta", Role: "tester", Folder: keptPath(owners), State: "idle", Started: 1761000400000})
	})
	logged := builtinsOnFor(t, d)
	d.watchers.syncBuiltins()
	var want []string
	for _, kind := range []string{"needs-review", "status"} {
		for _, file := range []string{filepath.Join(proj, ".team", "STATUS.md"), filepath.Join(proj, "STATUS.md")} {
			want = append(want, kind+" "+file)
		}
	}
	for _, hire := range []string{"ad0001", "de0001"} {
		// And the STATUS.md in its own folder in the support folder (FXSWEEP3P3R1 B2).
		own := plainPath(filepath.Join(hireFolder(supportDir, hire), "STATUS.md"))
		want := append(slices.Clone(want), "needs-review "+own, "status "+own)
		slices.Sort(want)
		if got := builtinTargets(d, hire); !slices.Equal(got, want) {
			t.Errorf("%s's built-ins: %q, want %q", hire, got, want)
		}
	}
	if strings.Contains(logged.String(), "no built-in watch") {
		t.Errorf("the log says a plain place got none:\n%s", logged.String())
	}
}
