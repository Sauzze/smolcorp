//go:build !windows

package sandbox

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// SA-T2, the dev-tool shims (Resources/shims; SANDBOX-PLAN §A4.3, AQ9): what each one hands to the real tool, run on
// stand-ins of swift and xcodebuild that only print their words, and that no desk can change them. Their folder is
// first on every Mac desk's PATH (desktop/deskstart.go); their real-profile rows are shims_darwin_test.go's.

// shimNames are the shims the app ships, as build-app.sh copies them.
var shimNames = []string{"swift", "xcodebuild"}

// shimFolder is a copy of the repo's shims in a plain folder of the test's own, as the app has them.
func shimFolder(t *testing.T, base string) string {
	t.Helper()
	folder := filepath.Join(base, "shims")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range shimNames {
		data, err := os.ReadFile(filepath.Join("..", "..", "Resources", "shims", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, name), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return folder
}

// printsItsWords is a stand-in tool: its path, then each of its words, one per line.
const printsItsWords = "#!/bin/sh\nprintf '%s\\n' \"$0\" \"$@\"\n"

// shimSays runs the shim name from folder with path as PATH, in dir, and gives its status and its words.
func shimSays(t *testing.T, folder, path, dir, name string, args ...string) (int, []string, string) {
	t.Helper()
	// A shim that hands on to itself never ends: it is stopped here, and fails.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := disclaim.Cmd(exec.CommandContext(ctx, filepath.Join(folder, name), args...))
	cmd.Env, cmd.Dir, cmd.WaitDelay = []string{"PATH=" + path, "HOME=" + dir, "LANG=C"}, dir, 5*time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	status := 0
	var exitErr *exec.ExitError
	if err := cmd.Run(); ctx.Err() != nil {
		t.Fatalf("%s %q didn't end within 20s: does it hand on to itself? (%s)", name, args, stderr.String())
	} else if errors.As(err, &exitErr) {
		status = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("%s %q: %v", name, args, err)
	}
	return status, strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n"), stderr.String()
}

func TestTheSwiftShimTurnsOffSwiftPMsSandboxAndNothingElse(t *testing.T) {
	s := newStandins(t)
	folder := shimFolder(t, s.base)
	real := s.program("tools/swift", printsItsWords)
	path := folder + ":" + filepath.Dir(real) + ":/usr/bin:/bin"
	for _, c := range []struct{ args, want []string }{
		{[]string{"build"}, []string{"build", "--disable-sandbox"}},
		{[]string{"test", "--filter", "MTests"}, []string{"test", "--disable-sandbox", "--filter", "MTests"}},
		{[]string{"run", "tool", "an argument"}, []string{"run", "--disable-sandbox", "tool", "an argument"}},
		{[]string{"package", "resolve"}, []string{"package", "--disable-sandbox", "resolve"}},
		{[]string{"build", "-Xswiftc", "-D A"}, []string{"build", "--disable-sandbox", "-Xswiftc", "-D A"}},
		// Already there: as typed. After a `--`, it is the program's, so it doesn't count.
		{[]string{"build", "-c", "release", "--disable-sandbox"}, []string{"build", "-c", "release", "--disable-sandbox"}},
		{[]string{"run", "tool", "--", "--disable-sandbox"}, []string{"run", "--disable-sandbox", "tool", "--", "--disable-sandbox"}},
		// Anything else: as typed.
		{nil, nil},
		{[]string{"--version"}, []string{"--version"}},
		{[]string{"script.swift", "build"}, []string{"script.swift", "build"}},
		{[]string{"sdk", "list"}, []string{"sdk", "list"}},
		{[]string{"-v", "build"}, []string{"-v", "build"}},
		{[]string{"Build"}, []string{"Build"}},
	} {
		status, said, stderr := shimSays(t, folder, path, s.base, "swift", c.args...)
		if want := append([]string{real}, c.want...); status != 0 || !slices.Equal(said, want) {
			t.Errorf("swift %q: status %d, %q (%s), want %q", c.args, status, said, stderr, want)
		}
	}
}

func TestTheXcodebuildShimTurnsOffScriptSandboxingForABuild(t *testing.T) {
	s := newStandins(t)
	folder := shimFolder(t, s.base)
	real := s.program("tools/xcodebuild", printsItsWords)
	path := folder + ":" + filepath.Dir(real) + ":/usr/bin:/bin"
	const off = "ENABLE_USER_SCRIPT_SANDBOXING=NO"
	for _, c := range []struct{ args, want []string }{
		{nil, []string{off}},
		{[]string{"-scheme", "M", "build"}, []string{"-scheme", "M", "build", off}},
		{[]string{"-project", "P.xcodeproj", "-target", "T", "test", "SYMROOT=/x y"}, []string{"-project", "P.xcodeproj", "-target", "T", "test", "SYMROOT=/x y", off}},
		// The owner's own value, typed: as typed, whatever it is.
		{[]string{"build", "ENABLE_USER_SCRIPT_SANDBOXING=YES"}, []string{"build", "ENABLE_USER_SCRIPT_SANDBOXING=YES"}},
		{[]string{"ENABLE_USER_SCRIPT_SANDBOXING=NO", "build"}, []string{"ENABLE_USER_SCRIPT_SANDBOXING=NO", "build"}},
		// What builds nothing: as typed.
		{[]string{"-version"}, []string{"-version"}},
		{[]string{"-sdk", "macosx", "-version"}, []string{"-sdk", "macosx", "-version"}},
		{[]string{"-showsdks"}, []string{"-showsdks"}},
		{[]string{"-find-executable", "clang"}, []string{"-find-executable", "clang"}},
		{[]string{"-list"}, []string{"-list"}},
		{[]string{"-project", "P.xcodeproj", "-list"}, []string{"-project", "P.xcodeproj", "-list"}},
		{[]string{"-scheme", "M", "-showdestinations"}, []string{"-scheme", "M", "-showdestinations"}},
		{[]string{"-scheme", "M", "-showTestPlans"}, []string{"-scheme", "M", "-showTestPlans"}},
		{[]string{"-exportArchive", "-archivePath", "M.xcarchive", "-exportPath", "out"}, []string{"-exportArchive", "-archivePath", "M.xcarchive", "-exportPath", "out"}},
		// Settings shown are the ones a build through the shim uses.
		{[]string{"-scheme", "M", "-showBuildSettings"}, []string{"-scheme", "M", "-showBuildSettings", off}},
	} {
		status, said, stderr := shimSays(t, folder, path, s.base, "xcodebuild", c.args...)
		if want := append([]string{real}, c.want...); status != 0 || !slices.Equal(said, want) {
			t.Errorf("xcodebuild %q: status %d, %q (%s), want %q", c.args, status, said, stderr, want)
		}
	}
}

// Each shim hands on to the first tool on PATH that isn't itself: its own folder again, or through a link, is passed
// over, and so are another Smolcorp's shims and a relative folder (a decoy in the project's folder); with none, it says
// so and runs nothing. It runs no program of its own on the way, so a PATH with no /usr/bin or /bin works the same.
func TestAShimNeverRunsItselfOrAToolFromARelativeFolder(t *testing.T) {
	s := newStandins(t)
	folder := shimFolder(t, s.base)
	s.link("again", "shims")
	project := s.dir("project")
	second := shimFolder(t, s.at("second"))
	for _, name := range shimNames {
		real := s.program("tools/"+name, printsItsWords)
		s.program("project/"+name, "#!/bin/sh\necho decoy\n")
		path := strings.Join([]string{folder, s.at("again"), second, ".", "project", "", filepath.Dir(real)}, ":")
		if status, said, stderr := shimSays(t, folder, path, project, name, "-version"); status != 0 || len(said) == 0 || said[0] != real {
			t.Errorf("%s: status %d, %q (%s), want %s first", name, status, said, stderr, real)
		}
		// Through the link: the same folder, passed over.
		if status, said, stderr := shimSays(t, s.at("again"), path, project, name, "-version"); status != 0 || len(said) == 0 || said[0] != real {
			t.Errorf("%s through a link to its folder: status %d, %q (%s)", name, status, said, stderr)
		}
		// Two Smolcorps' shims and nothing else: each passes the other over, rather than hand to it forever.
		if status, said, stderr := shimSays(t, second, second+":"+folder, project, name, "build"); status != 127 || len(said) != 1 || said[0] != "" {
			t.Errorf("%s with another Smolcorp's shims after it: status %d, %q, %q", name, status, said, stderr)
		}
		status, said, stderr := shimSays(t, folder, folder+":"+s.at("again")+":.:"+s.at("empty"), project, name, "build")
		if status != 127 || !strings.Contains(stderr, "there's no "+name+" on PATH besides Smolcorp's own") || slices.Contains(said, "decoy") {
			t.Errorf("%s with no other on PATH: status %d, %q, %q", name, status, said, stderr)
		}
	}
}

// The shims are in Smolcorp's install, which ResolvePrograms takes from the core's own program and the Policy denies
// whole (SA-X2): every desk reads and runs them, and none changes, replaces, moves or links them, nor adds a tool to
// their folder that its PATH would take first.
func TestTheAppsShimsAreReadOnlyToEveryDesk(t *testing.T) {
	s := newStandins(t)
	onlyPrograms(t)
	core := s.program("Applications/Smolcorp.app/Contents/Resources/bin/smolcorp", "#!/bin/sh\n")
	folder := s.at("Applications/Smolcorp.app/Contents/Resources/shims")
	ps := mustResolve(t, Finding{GOOS: "darwin", Home: s.home, Executable: core, DeveloperDir: s.at("no-developer-tools")})
	for _, c := range setups() {
		pl := c.pl
		pl.InstallRoots = append(ps.InstallRoots(), pl.InstallRoots...)
		p := mustNew(t, pl)
		places := []string{folder, filepath.Join(folder, "git"), filepath.Join(folder, "node")}
		for _, name := range shimNames {
			places = append(places, filepath.Join(folder, name))
		}
		for _, at := range places {
			for op, want := range map[Op]Verdict{Read: Allow, Write: Deny, Move: Deny, Link: Deny} {
				if got, rule := p.Decide(at, op); got != want {
					t.Errorf("%s: %s, %s: %v (%+v), want %v", c.name, at, op, got, rule, want)
				}
			}
		}
	}
}

// shimTools are the program list's entries for the tools the shims hand on to, as a Mac's list has them: none, should
// they leave the list, and then the rows that need them held fail on their own.
func shimTools() []ProgramName {
	var held []ProgramName
	for _, n := range ProgramNames("darwin") {
		if slices.Contains(shimNames, n.Name) {
			held = append(held, n)
		}
	}
	return held
}

// RT126SAT2 B1: a shim hands on to the first swift or xcodebuild on the search after its own folder, in whichever desk
// calls it. So both names are held in every search folder as the core's own programs are: a desk can't put one in a
// folder before the real one (the home folder's ~/.local/bin, a Homebrew bin outside it) for the GM's next build, nor
// change the real one. The folders stay the desk's to write: brew install of another program still works.
func TestNoDeskPlantsASwiftOrXcodebuildWhereAShimLooks(t *testing.T) {
	s := newStandins(t)
	onlyPrograms(t, shimTools()...)
	local, brew := filepath.Join(s.home, ".local", "bin"), s.dir("brew/bin")
	for _, name := range shimNames {
		s.program("tools/"+name, printsItsWords)
	}
	ps := mustResolve(t, Finding{GOOS: "darwin", Home: s.home, Search: []string{local, brew, s.at("tools")}, DeveloperDir: s.at("no-developer-tools")})
	for _, name := range shimNames {
		if got := ps.Path(name); got != s.at("tools/"+name) {
			t.Errorf("%s is %q, want the one the search finds", name, got)
		}
	}
	for column, subject := range map[string]Subject{"hire": Hire, "GM": GM} {
		p := mustNew(t, Places{GOOS: "darwin", Subject: subject, Home: s.home, Support: filepath.Join(s.home, "support"), OwnID: "h1",
			Root: filepath.Join(s.home, "P", "app"), InstallRoots: ps.InstallRoots()})
		for _, name := range shimNames {
			for _, at := range []string{filepath.Join(local, name), filepath.Join(brew, name), s.at("tools/" + name)} {
				if v, r := p.Decide(at, Write); v != Deny || r.Row != "programs that run outside" {
					t.Errorf("%s: a desk writing %s: %v by %q", column, at, v, r.Row)
				}
				for _, op := range []Op{Move, Link} {
					if v, r := p.Decide(at, op); v != Deny {
						t.Errorf("%s: a desk's %s at %s: %v by %q", column, op, at, v, r.Row)
					}
				}
			}
		}
		for _, other := range []string{filepath.Join(local, "jq"), filepath.Join(brew, "jq"), s.at("tools/jq")} {
			if v, _ := p.Decide(other, Write); v != Allow {
				t.Errorf("%s: a desk installing another program at %s: %v", column, other, v)
			}
		}
	}
}
