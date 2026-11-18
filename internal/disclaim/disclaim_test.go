package disclaim

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// standIn has Cmd see a Mac (or not) and smolcorp-spawn's place as given, for one test.
func standIn(t *testing.T, mac bool, where Place) {
	t.Helper()
	t.Cleanup(StandIn(mac, where))
}

// fakeSpawn is an executable file named smolcorp-spawn in a folder of its own: looked at, never run.
func fakeSpawn(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), SpawnName)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// standInProgram is a program by its full path that exists: this test binary itself, never started.
func standInProgram(t *testing.T) string {
	t.Helper()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return program
}

func TestOffAMacACommandIsLeftAsItIs(t *testing.T) {
	standIn(t, false, Place{Spawn: fakeSpawn(t), InApp: true})
	program := standInProgram(t)
	cmd := Cmd(exec.Command(program, "a", "b"))
	if cmd.Path != program || !slices.Equal(cmd.Args, []string{program, "a", "b"}) || cmd.Err != nil {
		t.Fatalf("changed off a Mac: %q %q %v", cmd.Path, cmd.Args, cmd.Err)
	}
}

func TestOnAMacTheProgramStartsThroughSmolcorpSpawnWithItsWordsAsTheyAre(t *testing.T) {
	spawn := fakeSpawn(t)
	program := standInProgram(t)
	words := []string{"two words", "", "$(touch x)", "*", "'quoted'", "-l"}
	for _, inApp := range []bool{true, false} {
		standIn(t, true, Place{Spawn: spawn, InApp: inApp})
		cmd := Cmd(exec.Command(program, words...))
		want := append([]string{spawn, program, program}, words...)
		if cmd.Err != nil || cmd.Path != spawn || !slices.Equal(cmd.Args, want) {
			t.Fatalf("in an app %v: %q %q %v, want %q", inApp, cmd.Path, cmd.Args, cmd.Err, want)
		}
	}
}

// A release never runs a program charged to Smolcorp: in an app with no smolcorp-spawn, nothing starts.
func TestInAnAppWithoutSmolcorpSpawnNothingStarts(t *testing.T) {
	folder := t.TempDir()
	standIn(t, true, Place{Spawn: filepath.Join(folder, "Smolcorp.app", "Contents", "Resources", "bin", SpawnName), InApp: true})
	marker := filepath.Join(folder, "ran")
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh here")
	}
	cmd := Cmd(exec.Command(sh, "-c", `touch "$0"`, marker))
	if err := cmd.Run(); err == nil || !strings.Contains(err.Error(), "Smolcorp didn't start sh: its smolcorp-spawn is missing") {
		t.Fatalf("started without smolcorp-spawn: %v", err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the program ran: %v", err)
	}
	// Nor with something that isn't a program in its place.
	if err := os.MkdirAll(filepath.Join(folder, SpawnName), 0o755); err != nil {
		t.Fatal(err)
	}
	standIn(t, true, Place{Spawn: filepath.Join(folder, SpawnName), InApp: true})
	if cmd := Cmd(exec.Command(sh, "-c", "true")); cmd.Err == nil {
		t.Fatal("a folder passed for smolcorp-spawn")
	}
	// Nor when where Smolcorp runs from can't be told.
	standIn(t, true, Place{InApp: true, Err: errors.New("no path")})
	if cmd := Cmd(exec.Command(sh, "-c", "true")); cmd.Err == nil || !strings.Contains(cmd.Err.Error(), "couldn't be told") {
		t.Fatalf("started with no place: %v", cmd.Err)
	}
}

// go test, go run and a build of the command on its own run from no app: no Smolcorp.app to be charged to, so
// with no smolcorp-spawn beside them their programs run as before.
func TestOutsideAnAppWithoutSmolcorpSpawnTheProgramRunsAsBefore(t *testing.T) {
	standIn(t, true, Place{Spawn: filepath.Join(t.TempDir(), SpawnName), InApp: false})
	program := standInProgram(t)
	cmd := Cmd(exec.Command(program, "x"))
	if cmd.Path != program || !slices.Equal(cmd.Args, []string{program, "x"}) || cmd.Err != nil {
		t.Fatalf("changed outside an app: %q %q %v", cmd.Path, cmd.Args, cmd.Err)
	}
}

func TestACommandThatCantStartStaysThatWay(t *testing.T) {
	standIn(t, true, Place{Spawn: fakeSpawn(t), InApp: true})
	cmd := exec.Command("smolcorp-no-such-program-here")
	before := cmd.Err
	if before == nil {
		t.Skip("a program by that name is on this PATH")
	}
	if Cmd(cmd).Err != before || cmd.Path != "smolcorp-no-such-program-here" {
		t.Fatalf("changed: %q %v", cmd.Path, cmd.Err)
	}
	missing := filepath.Join(t.TempDir(), "gone")
	if err := Cmd(exec.Command(missing)).Start(); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a missing program by its path: %v", err)
	}
}

func TestSmolcorpSpawnIsBesideTheCommandInTheAppsBinFolder(t *testing.T) {
	for _, c := range []struct {
		program string
		inApp   bool
	}{
		{"/Applications/Smolcorp.app/Contents/Resources/bin/smolcorp", true},
		{"/Users/x/build/Smolcorp Test.app/Contents/Resources/bin/smolcorp", true},
		{"/Users/x/go/bin/smolcorp", false},
		{"/tmp/go-build1/b001/disclaim.test", false},
		{"/Applications/Smolcorp.app/Contents/MacOS/smolcorp", false},
		{"/Applications/Smolcorp/Contents/Resources/bin/smolcorp", false},
	} {
		got := PlaceBeside(c.program)
		if got.InApp != c.inApp || got.Spawn != filepath.Join(filepath.Dir(c.program), SpawnName) {
			t.Errorf("%s: %+v", c.program, got)
		}
	}
}

// Words is Cmd for a starter that isn't an exec.Cmd (go-pty's): smolcorp-spawn, then the program twice (its path
// and its first word) and its words as they are; nothing to start in an app with no smolcorp-spawn; as it is off a
// Mac or outside an app.
func TestWordsGiveAStarterSmolcorpSpawnTheProgramAndItsWords(t *testing.T) {
	spawn := fakeSpawn(t)
	program := standInProgram(t)
	words := []string{"login", "two words", "", "$(touch x)"}
	standIn(t, true, Place{Spawn: spawn, InApp: true})
	got, args, err := Words(program, words)
	if want := append([]string{program, program}, words...); err != nil || got != spawn || !slices.Equal(args, want) {
		t.Fatalf("%q %q %v, want %q %q", got, args, err, spawn, want)
	}
	// By name, found on PATH first, as exec.Command finds it.
	folder := t.TempDir()
	named := filepath.Join(folder, "smolcorp-stand-in-agent")
	if err := os.WriteFile(named, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", folder)
	if got, args, err := Words("smolcorp-stand-in-agent", nil); err != nil || got != spawn || !slices.Equal(args, []string{named, named}) {
		t.Fatalf("by name: %q %q %v", got, args, err)
	}
	if _, _, err := Words("smolcorp-no-such-program-here", nil); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("not on PATH: %v", err)
	}

	standIn(t, true, Place{Spawn: filepath.Join(t.TempDir(), "Smolcorp.app", "Contents", "Resources", "bin", SpawnName), InApp: true})
	if got, args, err := Words(program, words); err == nil || got != "" || args != nil ||
		!strings.Contains(err.Error(), "Smolcorp didn't start "+filepath.Base(program)+": its smolcorp-spawn is missing") {
		t.Fatalf("in an app with no smolcorp-spawn: %q %q %v", got, args, err)
	}
	for _, where := range []struct {
		mac   bool
		place Place
	}{{false, Place{Spawn: spawn, InApp: true}}, {true, Place{Spawn: filepath.Join(t.TempDir(), SpawnName)}}} {
		standIn(t, where.mac, where.place)
		if got, args, err := Words(program, words); err != nil || got != program || !slices.Equal(args, words) {
			t.Fatalf("%+v: %q %q %v", where, got, args, err)
		}
	}
}

// A program by its full path with no execute bit, or a folder, fails at Start as it would have on its own
// (fork/exec … permission denied), not as smolcorp-spawn's exit status 126.
func TestAProgramThatCantRunFailsAtStartWithItsReason(t *testing.T) {
	standIn(t, true, Place{Spawn: fakeSpawn(t), InApp: true})
	folder := t.TempDir()
	plain := filepath.Join(folder, "not-a-program")
	if err := os.WriteFile(plain, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, program := range []string{plain, folder} {
		err := Cmd(exec.Command(program)).Start()
		if !errors.Is(err, fs.ErrPermission) || !strings.Contains(err.Error(), "fork/exec "+program) {
			t.Errorf("%s: %v", program, err)
		}
		if _, _, err := Words(program, nil); !errors.Is(err, fs.ErrPermission) {
			t.Errorf("Words %s: %v", program, err)
		}
	}
}

// A core linked for the app (scripts/build-app.sh's -X) counts as in an app wherever it runs from, so it never runs
// a program charged to Smolcorp; without it, either path in an app is enough.
func TestACoreBuiltForTheAppIsInAnAppWhateverItsPath(t *testing.T) {
	app := "/Applications/Smolcorp.app/Contents/Resources/bin/smolcorp"
	for _, c := range []struct {
		started, resolved string
		built, inApp      bool
	}{
		{"/Users/x/go/bin/smolcorp", "/Users/x/go/bin/smolcorp", true, true},
		{"/Users/x/go/bin/smolcorp", "/Users/x/go/bin/smolcorp", false, false},
		{app, "/Users/x/elsewhere/smolcorp", false, true},
		{"/usr/local/bin/smolcorp", app, false, true},
		{app, app, false, true},
	} {
		got := placeOf(c.started, c.resolved, c.built)
		if got.InApp != c.inApp || got.Spawn != filepath.Join(filepath.Dir(c.resolved), SpawnName) {
			t.Errorf("%+v: %+v", c, got)
		}
	}
	if builtForApp != "" {
		t.Errorf("go test links the core as for the app: %q", builtForApp)
	}
}

// build-app.sh links the engine for the app by this package's variable (a misspelt -X is silently ignored), and
// release.sh stops on an engine without its mark.
func TestTheAppsEngineIsLinkedForTheApp(t *testing.T) {
	const flag = "-X github.com/Sauzze/smolcorp/internal/disclaim.builtForApp=smolcorp-spawn-required"
	build, err := os.ReadFile(filepath.Join("..", "..", "scripts", "build-app.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(build), `FOR_APP="`+flag+`"`) || !strings.Contains(string(build), `-ldflags "-s -w -X main.version=$CLI_VERSION $FOR_APP"`) {
		t.Errorf("build-app.sh doesn't link the engine with %s", flag)
	}
	release, err := os.ReadFile(filepath.Join("..", "..", "scripts", "release.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(release), `grep -qaF 'smolcorp-spawn-required' "$APP/Contents/Resources/bin/smolcorp" || { echo "Stopped:`) {
		t.Error("release.sh doesn't stop on an engine that isn't linked for the app")
	}
}
