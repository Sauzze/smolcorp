//go:build !windows

package desktop

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bundleBeside makes a stand-in Smolcorp program in a folder of its own, with a tmux of mode beside it, and
// has executable name it, with the real lookup for the bundle (a test binary has no bundle of its own).
func bundleBeside(t *testing.T, mode os.FileMode) (folder, program, tmux string) {
	t.Helper()
	folder, tmux = standInTmux(t, mode)
	// The lookup resolves links (/private/var for /var on a Mac), so the stand-in is named that way.
	folder = physical(folder)
	tmux = filepath.Join(folder, "tmux")
	program = filepath.Join(folder, "smolcorp")
	if err := os.WriteFile(program, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	savedExecutable, savedBundled := executable, bundledTmux
	t.Cleanup(func() { executable, bundledTmux = savedExecutable, savedBundled })
	executable = func() (string, error) { return program, nil }
	bundledTmux = bundledTmuxBesideThisProgram
	return folder, program, tmux
}

// tmuxOnlyOnPATH puts a stand-in tmux on the PATH, with no usual place and no login shell to ask.
func tmuxOnlyOnPATH(t *testing.T) string {
	t.Helper()
	folder, path := standInTmux(t, 0o755)
	t.Setenv("PATH", folder)
	savedPlaces, savedLogin := tmuxPlaces, tmuxLoginPath
	t.Cleanup(func() { tmuxPlaces, tmuxLoginPath = savedPlaces, savedLogin })
	tmuxPlaces, tmuxLoginPath = nil, func() []string { return nil }
	return path
}

// The tmux that came with Smolcorp is the one used, before any on the PATH (1.26, bundled tmux).
func TestTheBundledTmuxBesideTheProgramIsPickedOverOneOnThePATH(t *testing.T) {
	onPath := tmuxOnlyOnPATH(t)
	_, _, bundled := bundleBeside(t, 0o755)
	if got := findTmux(); got != bundled {
		t.Errorf("findTmux: %q, want the bundled %q (the PATH has %q)", got, bundled, onPath)
	}
	if got := tmuxProgram(); got != bundled {
		t.Errorf("tmuxProgram: %q, want the bundled %q", got, bundled)
	}
	if got := tmuxPath(); got != bundled {
		t.Errorf("tmuxPath: %q, want the bundled %q", got, bundled)
	}
}

// Setup links smolcorp into Terminal's PATH: the bundle is still found beside the real file.
func TestTheBundledTmuxIsFoundThroughALinkToTheProgram(t *testing.T) {
	_, program, bundled := bundleBeside(t, 0o755)
	linkFolder := t.TempDir()
	link := filepath.Join(linkFolder, "smolcorp")
	if err := os.Symlink(program, link); err != nil {
		t.Fatal(err)
	}
	executable = func() (string, error) { return link, nil }
	// EvalSymlinks may spell the folder differently from t.TempDir (/private/var for /var).
	got := bundledTmux()
	if physical(got) != physical(bundled) {
		t.Errorf("through a link: %q, want %q", got, bundled)
	}
}

// A bundled file that can't be run is skipped: the PATH's tmux is used.
func TestABundledTmuxThatCantBeRunIsSkipped(t *testing.T) {
	onPath := tmuxOnlyOnPATH(t)
	bundleBeside(t, 0o644)
	if got := bundledTmux(); got != "" {
		t.Errorf("a file you can't run was picked: %q", got)
	}
	if got := findTmux(); got != onPath {
		t.Errorf("findTmux: %q, want the PATH's %q", got, onPath)
	}
}

// A bundled tmux that is a link, or that others can write, or that sits in a folder others can write, could have
// been swapped by someone else: it is skipped, and the PATH's tmux is used (FXTM3).
func TestABundledTmuxThatIsALinkOrWritableByOthersIsSkipped(t *testing.T) {
	onPath := tmuxOnlyOnPATH(t)
	folder, program, tmux := bundleBeside(t, 0o755)
	if got := bundledTmux(); got != tmux {
		t.Fatalf("a good one: %q, want %q", got, tmux)
	}
	skipped := func(why string) {
		t.Helper()
		if got := bundledTmux(); got != "" {
			t.Errorf("%s: picked %q", why, got)
		}
		if got := findTmux(); got != onPath {
			t.Errorf("%s: findTmux %q, want the PATH's %q", why, got, onPath)
		}
	}
	// World-writable file, and group-writable.
	for _, mode := range []os.FileMode{0o777, 0o775, 0o757} {
		if err := os.Chmod(tmux, mode); err != nil {
			t.Fatal(err)
		}
		skipped("mode " + mode.String())
	}
	if err := os.Chmod(tmux, 0o755); err != nil {
		t.Fatal(err)
	}
	// A link to a file elsewhere that is fine in itself.
	elsewhere := filepath.Join(t.TempDir(), "real-tmux")
	if err := os.WriteFile(elsewhere, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(tmux); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, tmux); err != nil {
		t.Fatal(err)
	}
	skipped("a link")
	// A good file in a world-writable folder.
	if err := os.Remove(tmux); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmux, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(folder, 0o777); err != nil {
		t.Fatal(err)
	}
	skipped("a world-writable folder")
	// Back as it was: picked again, so none of the above was a lookup that always says no.
	if err := os.Chmod(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := bundledTmux(); got != tmux {
		t.Errorf("a good one again: %q, want %q (program %s)", got, tmux, program)
	}
	// The same rule in Linux's library folder.
	home := t.TempDir()
	lib := filepath.Join(home, ".local", "lib", "smolcorp")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(lib, "tmux")); err != nil {
		t.Fatal(err)
	}
	if got := bundledTmuxFor(filepath.Join(t.TempDir(), "smolcorp"), home, "linux"); got != "" {
		t.Errorf("a link in the library folder: %q", got)
	}
}

// A folder named tmux is no program either.
func TestABundledTmuxThatIsAFolderIsSkipped(t *testing.T) {
	folder := t.TempDir()
	if err := os.Mkdir(filepath.Join(folder, "tmux"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := bundledTmuxFor(filepath.Join(folder, "smolcorp"), t.TempDir(), "linux"); got != "" {
		t.Errorf("a folder was picked: %q", got)
	}
}

// On Linux the bundle is in ~/.local/lib/smolcorp, when there's none beside the program.
func TestOnLinuxTheBundledTmuxIsInTheLibraryFolder(t *testing.T) {
	home := t.TempDir()
	lib := filepath.Join(home, ".local", "lib", "smolcorp")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	tmux := filepath.Join(lib, "tmux")
	if err := os.WriteFile(tmux, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "smolcorp") // nothing beside it
	if got := bundledTmuxFor(program, home, "linux"); got != tmux {
		t.Errorf("linux: %q, want %q", got, tmux)
	}
	if got := bundledTmuxFor(program, home, "darwin"); got != "" {
		t.Errorf("the Mac has no library folder, got %q", got)
	}
	if got := bundledTmuxFor(program, "", "linux"); got != "" {
		t.Errorf("no home folder, got %q", got)
	}
	// Beside the program wins over the library folder.
	_, beside := standInTmux(t, 0o755)
	if got := bundledTmuxFor(filepath.Join(filepath.Dir(beside), "smolcorp"), home, "linux"); got != beside {
		t.Errorf("beside the program: %q, want %q", got, beside)
	}
}

// A hire's recorded tmux drives its server, whatever tmux is picked now.
func TestAHiresRecordedTmuxWinsOverTheOneNowPicked(t *testing.T) {
	shortSupport(t)
	_, picked := standInTmux(t, 0o755)
	_, recorded := standInTmux(t, 0o755)
	bundledTmux = func() string { return picked }
	t.Cleanup(func() { bundledTmux = func() string { return "" } })
	socket := filepath.Join(supportDir, "smolcorp-ab12-cd34")
	writeRegistry(t, Hire{ID: "ab12-cd34", Name: "Dev", Folder: "/p", TmuxSocket: socket, TmuxProgram: recorded})

	if got := recordedTmux(socket); got != recorded {
		t.Errorf("recordedTmux: %q, want %q", got, recorded)
	}
	if got := tmuxDriving(socket); got != recorded {
		t.Errorf("tmuxDriving: %q, want the recorded %q", got, recorded)
	}
	if got := tmuxProgramFor(socket); got != recorded {
		t.Errorf("a tab's tmux: %q, want the recorded %q", got, recorded)
	}
	server := hireTmuxAt(socket, 0, nil)
	if server == nil || server.program != physical(recorded) {
		t.Fatalf("the hire's server: %+v, want one run by %q", server, recorded)
	}
	// Another socket has no such row: the one picked now.
	other := filepath.Join(supportDir, "smolcorp-ef56-0000")
	if got := tmuxDriving(other); got != picked {
		t.Errorf("another socket: %q, want the picked %q", got, picked)
	}
	// A server that is to be started takes the one picked now, not an old row's.
	fresh := newHireTmux("ab12-cd34", 0, nil)
	if fresh == nil || fresh.program != physical(picked) {
		t.Errorf("a new server for the same id: %+v, want one run by the picked %q", fresh, picked)
	}
}

// A row with no path (a 1.25 hire), or one whose tmux has gone, uses today's order.
func TestAnOldRowWithoutAPathFallsBackAndSoDoesAGoneOne(t *testing.T) {
	shortSupport(t)
	_, picked := standInTmux(t, 0o755)
	bundledTmux = func() string { return picked }
	t.Cleanup(func() { bundledTmux = func() string { return "" } })
	gone := filepath.Join(t.TempDir(), "tmux") // never made
	old := filepath.Join(supportDir, "smolcorp-ab12-cd34")
	goneSocket := filepath.Join(supportDir, "smolcorp-ef56-0000")
	writeRegistry(t,
		Hire{ID: "ab12-cd34", Name: "Dev", Folder: "/p", TmuxSocket: old},
		Hire{ID: "ef56-0000", Name: "Dev", Folder: "/p", TmuxSocket: goneSocket, TmuxProgram: gone},
	)
	for _, socket := range []string{old, goneSocket, ""} {
		if got := recordedTmux(socket); got != "" {
			t.Errorf("recordedTmux(%q): %q, want none", socket, got)
		}
		if got := tmuxDriving(socket); got != picked {
			t.Errorf("tmuxDriving(%q): %q, want today's pick %q", socket, got, picked)
		}
	}
	if server := hireTmuxAt(old, 0, nil); server == nil || server.program != physical(picked) {
		t.Errorf("an old row's server: %+v, want one run by %q", server, picked)
	}
	// A recorded path that isn't a program you can run is no better than a gone one.
	_, notRunnable := standInTmux(t, 0o644)
	writeRegistry(t, Hire{ID: "ab12-cd34", Name: "Dev", Folder: "/p", TmuxSocket: old, TmuxProgram: notRunnable})
	if got := tmuxDriving(old); got != picked {
		t.Errorf("a recorded file that can't be run: %q, want %q", got, picked)
	}
}

// A recorded tmux that is a link, or others can write, or sits in a folder others can write, is refused as the
// bundled one is: the hire's tmux is then the one picked now (FXTM3).
func TestARecordedTmuxThatIsALinkOrWritableByOthersFallsBack(t *testing.T) {
	shortSupport(t)
	_, picked := standInTmux(t, 0o755)
	bundledTmux = func() string { return picked }
	t.Cleanup(func() { bundledTmux = func() string { return "" } })
	socket := filepath.Join(supportDir, "smolcorp-ab12-cd34")
	record := func(program string) {
		t.Helper()
		writeRegistry(t, Hire{ID: "ab12-cd34", Name: "Dev", Folder: "/p", TmuxSocket: socket, TmuxProgram: program})
	}
	check := func(why, program string, want string) {
		t.Helper()
		record(program)
		if got := tmuxDriving(socket); got != want {
			t.Errorf("%s: tmuxDriving %q, want %q", why, got, want)
		}
		if got := tmuxProgramFor(socket); got != want && want == picked {
			t.Errorf("%s: a tab's tmux %q, want %q", why, got, want)
		}
	}
	folder, good := standInTmux(t, 0o755)
	check("a good recorded one", good, good)

	if err := os.Chmod(good, 0o777); err != nil {
		t.Fatal(err)
	}
	check("mode 0777", good, picked)
	if err := os.Chmod(good, 0o755); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(t.TempDir(), "tmux")
	if err := os.Symlink(good, link); err != nil {
		t.Fatal(err)
	}
	check("a link", link, picked)

	if err := os.Chmod(folder, 0o777); err != nil {
		t.Fatal(err)
	}
	check("a world-writable folder", good, picked)
	if err := os.Chmod(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	check("the folder fixed", good, good)
}

// A recorded tmux that isn't used is said once per hire: one plain line with why and the tmux used instead, however
// many times a tab polls (RT126TM3b).
func TestARecordedTmuxThatIsNotUsedIsSaidOnceWithWhyAndTheOneUsed(t *testing.T) {
	shortSupport(t)
	_, picked := standInTmux(t, 0o755)
	bundledTmux = func() string { return picked }
	t.Cleanup(func() { bundledTmux = func() string { return "" } })
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	skippedTmuxMu.Lock()
	skippedTmuxLogged = map[string]bool{}
	skippedTmuxMu.Unlock()
	socket := filepath.Join(supportDir, "smolcorp-ab12-cd34")
	record := func(program string) {
		t.Helper()
		writeRegistry(t, Hire{ID: "ab12-cd34", Name: "Dev", Folder: "/p", TmuxSocket: socket, TmuxProgram: program})
	}
	lines := func() []string { return strings.Split(strings.TrimSpace(logged.String()), "\n") }

	// A row with no tmux recorded, and one whose tmux is used: nothing to say.
	_, good := standInTmux(t, 0o755)
	record("")
	_ = tmuxDriving(socket)
	record(good)
	_ = tmuxDriving(socket)
	if logged.Len() != 0 {
		t.Fatalf("nothing was skipped, yet: %q", logged.String())
	}

	// Gone: polled many times through both callers, one line, naming the path, the reason and the tmux used instead.
	gone := filepath.Join(t.TempDir(), "tmux")
	record(gone)
	for range 5 {
		if tmuxDriving(socket) != picked || tmuxProgramFor(socket) != picked {
			t.Fatal("not the picked tmux")
		}
	}
	got := lines()
	if len(got) != 1 || !strings.Contains(got[0], gone) || !strings.Contains(got[0], "it's gone") || !strings.Contains(got[0], picked) || !strings.Contains(got[0], "smolcorp-ab12-cd34") {
		t.Fatalf("a gone tmux: %q", got)
	}

	// Each other reason is its own path, so its own line, once.
	_, writable := standInTmux(t, 0o755)
	if err := os.Chmod(writable, 0o775); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "tmux")
	if err := os.Symlink(good, link); err != nil {
		t.Fatal(err)
	}
	open, openFile := standInTmux(t, 0o755)
	if err := os.Chmod(open, 0o777); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ program, why string }{
		{writable, "it's writable by others"}, {link, "it isn't a regular file"}, {openFile, "its folder is writable by others"}, {"tmux", "it isn't a full path"},
	} {
		before := len(lines())
		record(c.program)
		for range 3 {
			_ = tmuxDriving(socket)
		}
		got := lines()
		if len(got) != before+1 || !strings.Contains(got[len(got)-1], c.why) || !strings.Contains(got[len(got)-1], c.program) {
			t.Errorf("%s: %q", c.why, got)
		}
	}
	// And a tab's poll after the first line says nothing new for a path already said.
	n := len(lines())
	_ = tmuxProgramFor(socket)
	if len(lines()) != n {
		t.Errorf("said again: %q", lines())
	}
}

// A tmux that is a link by design (Homebrew's) is recorded as the file it points to, which a later check accepts;
// a link put in its place afterwards is not.
func TestALinkedTmuxIsRecordedAsTheFileItPointsTo(t *testing.T) {
	_, real := standInTmux(t, 0o755)
	link := filepath.Join(t.TempDir(), "tmux")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if got := recordableTmux(link); got != physical(real) {
		t.Errorf("recordableTmux(link): %q, want %q", got, physical(real))
	}
	if got := trustedExecutableIn(filepath.Dir(recordableTmux(link)), "tmux"); got == "" {
		t.Error("the recorded file isn't accepted")
	}
	if got := recordableTmux("/gone/tmux"); got != "/gone/tmux" {
		t.Errorf("a path that can't be resolved is kept: %q", got)
	}
}

// The path is written only when known: a 1.25 row, and one made with a stand-in tmux, are as they were.
func TestTheRecordedTmuxIsWrittenOnlyWhenKnown(t *testing.T) {
	plain, err := json.Marshal(Hire{ID: "a", Name: "Dev", Folder: "/p"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "tmuxProgram") {
		t.Errorf("a row with no path has one: %s", plain)
	}
	kept, _ := json.Marshal(Hire{ID: "a", Name: "Dev", Folder: "/p", TmuxProgram: "/x/tmux"})
	if !strings.Contains(string(kept), `"tmuxProgram":"/x/tmux"`) {
		t.Errorf("the path isn't written: %s", kept)
	}
	var back Hire
	if err := json.Unmarshal(kept, &back); err != nil || back.TmuxProgram != "/x/tmux" {
		t.Errorf("read back: %+v, %v", back, err)
	}
}

// A new hire's server records the tmux it is run by, for its row.
func TestANewHiresServerKnowsItsTmuxForItsRow(t *testing.T) {
	shortSupport(t)
	_, picked := standInTmux(t, 0o755)
	bundledTmux = func() string { return picked }
	t.Cleanup(func() { bundledTmux = func() string { return "" } })
	if server := newHireTmux("ab12-cd34", 0, nil); server == nil || server.program != physical(picked) {
		t.Errorf("a new server: %+v, want one run by %q", server, picked)
	}
	// A stand-in runner has no real tmux to record.
	if server := newHireTmux("ab12-cd34", 0, newRecorder().run); server == nil || server.program != "" {
		t.Errorf("a stand-in server: %+v, want no recorded tmux", server)
	}
}
