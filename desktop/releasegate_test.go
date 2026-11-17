//go:build unix

package desktop

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"
)

// RELGATE1: every check a release must pass is in one script, scripts/release-gate.sh, and every path that makes or
// sends a download sources it first and builds nothing if it fails: release.sh (the Mac's, --dmg-only too),
// release-desktop.sh (Windows and Linux), make-dmg.sh run on its own, build-app.sh with a Developer ID, and
// publish-release.sh. No other script keeps a copy of any of it, and no CI job ships a download around it. These rows
// are fast: a stand-in gate in a throwaway folder, the real script, and stand-ins for every program it could run after
// the gate, which note that they ran and do nothing; the real gate's own rows are release-desktop.sh's heavy ones.

const (
	releaseGatePath = "scripts/release-gate.sh"
	releaseGateLine = ". " + releaseGatePath

	gateStandinRan    = "stand-in gate ran"
	gateStandinPassed = "stand-in gate passed"
	gatePassedAlready = "The release gate passed in this run already"
	stopGateChanged   = "Stopped: the files changed after the release gate passed"
	standinIdentity   = "Developer ID Application: Stand In (STANDIN000)"
)

// shippingPath is one way to make or send a download: a script, its arguments and settings, and what it does next
// once the gate passed (a line it says; any stand-in program noting it ran counts too).
type shippingPath struct {
	name, script string
	args, env    []string
	// setup prepares the stand-in home, when the path needs something there; files are written beside the script.
	setup func(t *testing.T, home string)
	files map[string]string
	after string
}

var shippingPaths = []shippingPath{
	{name: "release.sh", script: "scripts/release.sh"},
	{name: "release.sh --dmg-only", script: "scripts/release.sh", args: []string{"--dmg-only"},
		env: []string{"SMOLCORP_DEVELOPER_ID=" + standinIdentity, "SMOLCORP_NOTARY_PROFILE=stand-in"}},
	{name: "release-desktop.sh", script: "scripts/release-desktop.sh"},
	{name: "make-dmg.sh run on its own", script: "scripts/make-dmg.sh",
		env: []string{"SMOLCORP_DEVELOPER_ID=" + standinIdentity}, after: "No build/Smolcorp.app to put in the DMG"},
	{name: "build-app.sh universal, a Developer ID named", script: "scripts/build-app.sh", args: []string{"universal"},
		env: []string{"SMOLCORP_SIGN_IDENTITY=" + standinIdentity}, after: "Building…"},
	{name: "build-app.sh, a Developer ID saved as its identity", script: "scripts/build-app.sh",
		setup: func(t *testing.T, home string) {
			writeReleaseFile(t, filepath.Join(home, ".config", "smolcorp", "sign-identity"), standinIdentity+"\n", 0o644)
		}, after: "Building…"},
	{name: "publish-release.sh", script: "scripts/publish-release.sh"},
	{name: "publish-release.sh --check", script: "scripts/publish-release.sh", args: []string{"--check"}},
	// A gate mark of the caller's makes even an ad hoc build run the whole gate: only the gate tells its own mark.
	{name: "build-app.sh ad hoc, a gate mark in its environment", script: "scripts/build-app.sh",
		args: []string{"universal"}, env: []string{"SMOLCORP_SIGN_IDENTITY=-", "release_gate_passed=t1"}, after: "Building…"},
}

// standinPrograms are the programs a release script could run after the gate. Each is a stand-in that notes its run
// and exits 97; nothing else is on PATH but dirname and head, which the scripts use before their gate.
var standinPrograms = []string{"awk", "basename", "cat", "cc", "chmod", "clang", "codesign", "cp", "cut", "ditto", "du",
	"env", "find", "gh", "git", "go", "grep", "hdiutil", "lipo", "ln", "ls", "mkdir", "mktemp", "mv", "perl", "pip",
	"python3", "realpath", "rm", "security", "sed", "shasum", "sort", "spctl", "strip", "swift", "tar", "touch", "tr",
	"unzip", "xattr", "xcrun", "zip"}

// pathRun is one run of a shipping path's script in a folder of its own.
type pathRun struct {
	status  int
	out     string
	reached []string
	left    []string
}

// runShippingPath runs p's real script in a fresh folder whose scripts/release-gate.sh is gate ("" for none), with
// only the stand-in programs on PATH, and gives what happened.
func runShippingPath(t *testing.T, p shippingPath, gate string) pathRun {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("..", p.script))
	if err != nil {
		t.Fatal(err)
	}
	dir, home, bin := t.TempDir(), t.TempDir(), t.TempDir()
	log := filepath.Join(t.TempDir(), "ran")
	writeReleaseFile(t, filepath.Join(dir, p.script), string(script), 0o755)
	if gate != "" {
		gate = strings.ReplaceAll(gate, "LOG", log)
		writeReleaseFile(t, filepath.Join(dir, releaseGatePath), gate, 0o644)
	}
	for _, name := range standinPrograms {
		writeReleaseFile(t, filepath.Join(bin, name), "#!/bin/sh\necho \"${0##*/} $*\" >> '"+log+"'\nexit 97\n", 0o755)
	}
	linkRealTools(t, bin, "dirname", "head")
	if p.setup != nil {
		p.setup(t, home)
	}
	for name, text := range p.files {
		writeReleaseFile(t, filepath.Join(dir, name), text, 0o644)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/bash", append([]string{p.script}, p.args...)...)
	cmd.Dir = dir
	cmd.Env = append([]string{"PATH=" + bin, "HOME=" + home, "LANG=C", "TMPDIR=" + t.TempDir()}, p.env...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("%s didn't end within 30s:\n%s", p.script, out)
	}
	run := pathRun{out: string(out)}
	if exitErr, ok := err.(*exec.ExitError); ok {
		run.status = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(log); err == nil {
		run.reached = strings.Split(strings.TrimSpace(string(data)), "\n")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "scripts" && (len(p.files) == 0 || entry.Name() != "packaging") {
			run.left = append(run.left, entry.Name())
		}
	}
	return run
}

// linkRealTools puts the system's own programs of these names in bin, or skips the test if it has none.
func linkRealTools(t *testing.T, bin string, names ...string) {
	t.Helper()
	for _, name := range names {
		real := ""
		for _, folder := range []string{"/usr/bin", "/bin"} {
			if _, err := os.Stat(filepath.Join(folder, name)); err == nil {
				real = filepath.Join(folder, name)
				break
			}
		}
		if real == "" {
			t.Skipf("no %s in /usr/bin or /bin", name)
		}
		if err := os.Symlink(real, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
}

// The stand-in gates: one that fails as the real one does (exit 1), one that fails by returning 1 to the script that
// sourced it, and one that passes and gives release-desktop.sh and build-app.sh what they build with.
const (
	gateExits   = "echo '" + gateStandinRan + "'\nexit 1\n"
	gateReturns = "echo '" + gateStandinRan + "'\nreturn 1\n"
	gatePasses  = "echo '" + gateStandinPassed + "'\n" +
		"pinned_go() { echo \"pinned_go $*\" >> 'LOG'; exit 97; }\npinned_env() { echo \"pinned_env $*\" >> 'LOG'; exit 97; }\n" +
		"GO=/stand-in/go\nCC_PROGRAM=/stand-in/cc\nrelease_gate_go_done() { :; }\n"
)

// Each path, with the gate failing (exiting, returning) or missing: it stops with the gate's status, and no program
// after it ran and nothing was written. With the gate passing, it goes on (a stand-in ran, or it says its next line),
// so the rows above stopped at the gate, not on the way to it.
func TestReleaseGateEveryShippingPathRefusesWhenTheGateFails(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("no /bin/bash")
	}
	for _, p := range shippingPaths {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			for _, c := range []struct{ name, gate string }{
				{"the gate failing", gateExits},
				{"the gate returning a failure", gateReturns},
				{"the gate missing", ""},
			} {
				run := runShippingPath(t, p, c.gate)
				ranGate := c.gate == "" || strings.Contains(run.out, gateStandinRan)
				if run.status == 0 || !ranGate || len(run.reached) > 0 || len(run.left) > 0 ||
					(p.after != "" && strings.Contains(run.out, p.after)) {
					t.Errorf("%s: want a stop at the gate, nothing run or written after it; got status %d, ran %q, left %q:\n%s",
						c.name, run.status, run.reached, run.left, run.out)
				}
			}
			run := runShippingPath(t, p, gatePasses)
			if !strings.Contains(run.out, gateStandinPassed) ||
				(len(run.reached) == 0 && (p.after == "" || !strings.Contains(run.out, p.after))) {
				t.Errorf("the gate passing: want the script on past it, got status %d, ran %q:\n%s", run.status, run.reached, run.out)
			}
		})
	}
}

// What isn't a release doesn't run the gate: a dry run of the signing, and a copy of your own (ad hoc, or with your
// own identity). Each goes on with the gate failing.
func TestReleaseGateLeavesWhatIsntAReleaseAlone(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("no /bin/bash")
	}
	for _, p := range []shippingPath{
		{name: "release.sh --check-signing", script: "scripts/release.sh", args: []string{"--check-signing"}},
		{name: "build-app.sh --check-signing, a Developer ID named", script: "scripts/build-app.sh",
			args: []string{"--check-signing"}, env: []string{"SMOLCORP_SIGN_IDENTITY=" + standinIdentity},
			files: map[string]string{"packaging/macos/Smolcorp.entitlements": "<plist/>\n"}, after: "Signing: "},
		{name: "build-app.sh universal, ad hoc", script: "scripts/build-app.sh", args: []string{"universal"},
			env: []string{"SMOLCORP_SIGN_IDENTITY=-"}, after: "Building…"},
		{name: "build-app.sh, an identity of your own", script: "scripts/build-app.sh",
			env: []string{"SMOLCORP_SIGN_IDENTITY=Smolcorp Local"}, after: "Building…"},
	} {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			run := runShippingPath(t, p, gateExits)
			if strings.Contains(run.out, gateStandinRan) ||
				(len(run.reached) == 0 && (p.after == "" || !strings.Contains(run.out, p.after))) {
				t.Errorf("want it on without the gate, got status %d, ran %q:\n%s", run.status, run.reached, run.out)
			}
		})
	}
}

// gateRun runs shell in a fresh folder holding the real gate (and the real make-dmg.sh), with the real go first on
// PATH, and gives its status and output.
func gateRun(t *testing.T, shell string, env ...string) (int, string) {
	t.Helper()
	goPath, _ := realGo(t)
	dir := t.TempDir()
	for _, name := range []string{releaseGatePath, "scripts/make-dmg.sh"} {
		data, err := os.ReadFile(filepath.Join("..", name))
		if err != nil {
			t.Fatal(err)
		}
		writeReleaseFile(t, filepath.Join(dir, name), string(data), 0o755)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", shell)
	cmd.Dir = dir
	cmd.Env = append([]string{"PATH=" + filepath.Dir(goPath) + ":/usr/bin:/bin", "HOME=" + t.TempDir(), "LANG=C",
		"TMPDIR=" + t.TempDir(), "GIT_CEILING_DIRECTORIES=" + filepath.Dir(dir)}, env...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("didn't end within a minute:\n%s", out)
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), string(out)
	} else if err != nil {
		t.Fatal(err)
	}
	return 0, string(out)
}

// The gate runs whole unless it passed in this very shell (release.sh, then build-app.sh and make-dmg.sh in its
// subshells): its mark is readonly and never exported, so a setting of the caller's can't stand for it. Run whole here,
// it pins go and the C compiler (the log's "go: " line) and then stops, as this folder isn't a repo. Likewise go's
// folders stay past the gate only when the script that sources it asks in its own shell, unexported (RT126RELGATE1's
// nit): a release_gate_builds_go of the caller's would leave ~650 MB behind a script whose exit trap doesn't run.
func TestReleaseGateRunsWholeUnlessItPassedInThisShell(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("no /bin/bash")
	}
	const (
		passed   = "TREE=t1; heads_tree() { echo 'heads checked'; }; readonly release_gate_passed=t1; "
		after    = "; echo 'the shell went on'"
		wholeRun = "go: "
		stopRepo = "Stopped: git here looks at another folder"
		goKept   = "go's folders: kept for the builds after the gate"
		goGone   = "go's folders: removed as soon as the gate passes"
	)
	for _, c := range []struct {
		name, shell string
		env         []string
		// want are lines the output must have, absent lines it mustn't; status the exit status.
		want, absent []string
		status       int
	}{
		{"passed in this shell, the files still its tree", passed + releaseGateLine + after,
			nil, []string{gatePassedAlready, "heads checked", "the shell went on"}, nil, 0},
		{"passed in this shell, the files changed since", strings.Replace(passed, "echo 'heads checked'", "return 1", 1) +
			releaseGateLine + after, nil, []string{stopGateChanged}, nil, 1},
		{"passed in this shell for another tree", strings.Replace(passed, "TREE=t1", "TREE=t2", 1) + releaseGateLine + after,
			nil, []string{stopGateChanged}, nil, 1},
		{"the mark in the caller's environment", "TREE=t1; heads_tree() { echo 'heads checked'; }; " + releaseGateLine + after,
			[]string{"release_gate_passed=t1"}, []string{wholeRun, stopRepo}, nil, 1},
		{"the mark readonly and exported", "TREE=t1; heads_tree() { :; }; export release_gate_passed=t1; " +
			"readonly release_gate_passed; " + releaseGateLine + after, nil,
			[]string{"Stopped: release_gate_passed is set readonly from elsewhere"}, nil, 1},
		{"make-dmg.sh run on its own, the mark in its environment", "/bin/bash scripts/make-dmg.sh",
			[]string{"release_gate_passed=t1"}, []string{wholeRun, stopRepo}, nil, 1},
		{"make-dmg.sh in release.sh's subshell after the gate passed", passed + "( SMOLCORP_DEVELOPER_ID=; . scripts/make-dmg.sh )",
			nil, []string{gatePassedAlready, "heads checked", "No build/Smolcorp.app to put in the DMG"}, nil, 1},
		{"go's folders: kept when the sourcing script asks in its own shell", "declare +x release_gate_builds_go=1; " +
			releaseGateLine + after, []string{"release_gate_builds_go=1"}, []string{wholeRun, stopRepo, goKept},
			[]string{goGone}, 1},
		{"go's folders: the caller's setting doesn't keep them", releaseGateLine + after,
			[]string{"release_gate_builds_go=1"}, []string{wholeRun, stopRepo, goGone}, []string{goKept}, 1},
		{"go's folders: the caller's setting, set again but still exported", "release_gate_builds_go=1; " +
			releaseGateLine + after, []string{"release_gate_builds_go=1"}, []string{wholeRun, stopRepo, goGone},
			[]string{goKept}, 1},
		{"go's folders: asked readonly", "readonly release_gate_builds_go=1; " + releaseGateLine + after, nil,
			[]string{"Stopped: release_gate_builds_go is set readonly from elsewhere"}, []string{goKept, goGone}, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			status, out := gateRun(t, c.shell, c.env...)
			whole := slices.Contains(c.want, wholeRun)
			if status != c.status || strings.Contains(out, "the shell went on") != (c.status == 0) ||
				strings.HasPrefix(out, wholeRun) != whole {
				t.Errorf("want status %d, the whole gate run: %v; got status %d:\n%s", c.status, whole, status, out)
			}
			for _, line := range c.want {
				if !strings.Contains(out, line) {
					t.Errorf("want %q in:\n%s", line, out)
				}
			}
			for _, line := range c.absent {
				if strings.Contains(out, line) {
					t.Errorf("want no %q in:\n%s", line, out)
				}
			}
		})
	}
}

// callerBuildSettings are the caller's settings that would change what go or the C compiler builds (RT126RELGATE1 B1):
// each value holds forgedMark, so a build that got one shows it.
var callerBuildSettings = []string{"GOFLAGS=-toolexec=/" + forgedMark + "/toolexec", "GOTOOLCHAIN=go1.99.0-" + forgedMark,
	"GOENV=/" + forgedMark + "/go.env", "GOWORK=/" + forgedMark + "/go.work", "GOCACHE=/" + forgedMark + "/cache",
	"GOROOT=/" + forgedMark + "/goroot", "CC=/" + forgedMark + "/cc", "CGO_CFLAGS=-I/" + forgedMark + "/include",
	"CGO_LDFLAGS=-L/" + forgedMark + "/lib", "LIBRARY_PATH=/" + forgedMark + "/lib", "CPATH=/" + forgedMark + "/include",
	"SDKROOT=/" + forgedMark + "/sdk", "CCC_OVERRIDE_OPTIONS=#^-I/" + forgedMark,
	// RT126RELGATE1R2's probe: 11 more, and those that reach clang in tmux's build (DEVELOPER_DIR, TOOLCHAINS,
	// COMPILER_PATH, CFLAGS, CPPFLAGS, LDFLAGS).
	"DEVELOPER_DIR=/" + forgedMark + "/xcode", "GOEXPERIMENT=" + forgedMark, "COMPILER_PATH=/" + forgedMark + "/bin",
	"GOAMD64=" + forgedMark, "CGO_CPPFLAGS=-I/" + forgedMark, "GODEBUG=" + forgedMark, "TOOLCHAINS=" + forgedMark,
	"MACOSX_DEPLOYMENT_TARGET=10.9-" + forgedMark, "GOPROXY=https://" + forgedMark, "GONOSUMCHECK=" + forgedMark,
	"release_gate_builds_go=1", "CFLAGS=-I/" + forgedMark, "CPPFLAGS=-I/" + forgedMark, "LDFLAGS=-L/" + forgedMark}

// macBuildCall is one run of a go or a C compiler by a Mac build: whose it is (the gate's, or the one on PATH), its
// arguments, and its settings.
type macBuildCall struct {
	who, args string
	env       map[string]string
}

// macBuild is one run of a Mac build's script: its status and output, each go and C compiler it ran (and tmux's
// build), in order, how many had run when it said it was done with go's folders (-1 if it never did), the gate's
// go_env, what it removed (with how many had run by then), and each codesign's arguments.
type macBuild struct {
	status  int
	out     string
	calls   []macBuildCall
	done    int
	goEnv   map[string]string
	removed map[string]int
	signed  []string
}

// runMacBuild runs script (build-app.sh, or release.sh, which sources it) in a fresh folder, with the caller's build
// settings and env. The gate is a stand-in made of the real gate's pinned_env and pinned_go around a go and a C
// compiler of its own, which note their arguments and settings. On PATH, a go, cc and clang that note theirs too, an
// rm and a codesign that note their arguments and do nothing, and for every other program but grep a stand-in that
// does nothing and succeeds (or the one identities gives for security), so the build runs through to its end and
// writes nothing: no app, nothing signed (this Mac never builds an .app). scripts/build-tmux.sh notes its settings
// and fails, as with no network.
func runMacBuild(t *testing.T, script string, args []string, identities string, env ...string) macBuild {
	t.Helper()
	gate, err := os.ReadFile(filepath.Join("..", releaseGatePath))
	if err != nil {
		t.Fatal(err)
	}
	var pins []string
	for _, name := range []string{"pinned_env", "pinned_go"} {
		found := regexp.MustCompile(`(?m)^`+name+`\(\) \{.*\}$`).FindAllString(string(gate), -1)
		if len(found) != 1 {
			t.Fatalf("the gate defines %s on one line %d times, want once", name, len(found))
		}
		pins = append(pins, found[0])
	}
	dir, home, bin, gateBin, tmp := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	log := filepath.Join(t.TempDir(), "ran")
	for _, name := range []string{script, "scripts/build-app.sh"} {
		data, err := os.ReadFile(filepath.Join("..", name))
		if err != nil {
			t.Fatal(err)
		}
		writeReleaseFile(t, filepath.Join(dir, name), string(data), 0o755)
	}
	recorder := func(who string) string {
		return "#!/bin/sh\n{ echo \"RUN " + who + " $*\"; /usr/bin/env | while IFS= read -r line; do echo \"  $line\"; done; } >> '" +
			log + "'\nexit 0\n"
	}
	for name, status := range map[string]int{"build-saver.sh": 0, "check-runtime.sh": 0} {
		writeReleaseFile(t, filepath.Join(dir, "scripts", name), fmt.Sprintf("#!/bin/sh\nexit %d\n", status), 0o755)
	}
	writeReleaseFile(t, filepath.Join(dir, "scripts", "build-tmux.sh"), strings.Replace(recorder("tmux-build"), "exit 0", "exit 1", 1), 0o755)
	if err := os.Mkdir(filepath.Join(dir, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeReleaseFile(t, filepath.Join(gateBin, "go"), recorder("gate-go"), 0o755)
	writeReleaseFile(t, filepath.Join(gateBin, "clang"), recorder("gate-cc"), 0o755)
	for _, name := range append(standinPrograms, "bash", "cmp", "tail") {
		text := "#!/bin/sh\nexit 0\n"
		switch name {
		case "go", "cc", "clang":
			text = recorder("path-" + name)
		case "rm", "codesign":
			text = "#!/bin/sh\necho \"" + strings.ToUpper(name) + " $*\" >> '" + log + "'\n"
		case "security":
			text = "#!/bin/sh\nprintf '%s' '" + identities + "'\n"
		case "env", "grep":
			continue
		}
		writeReleaseFile(t, filepath.Join(bin, name), text, 0o755)
	}
	linkRealTools(t, bin, "dirname", "head", "env", "grep")
	goEnv := map[string]string{"HOME": home, "PATH": "/usr/bin:/bin", "TMPDIR": tmp, "GOCACHE": tmp + "/cache",
		"GOFLAGS": "-mod=readonly", "GOENV": "off", "GOWORK": "off", "GOTOOLCHAIN": "local", "LANG": "C",
		"CC": filepath.Join(gateBin, "clang")}
	var list []string
	for _, name := range slices.Sorted(maps.Keys(goEnv)) {
		list = append(list, name+"='"+goEnv[name]+"'")
	}
	writeReleaseFile(t, filepath.Join(dir, releaseGatePath), "echo '"+gateStandinPassed+"'\n"+
		"GO='"+filepath.Join(gateBin, "go")+"'\nCC_PROGRAM='"+filepath.Join(gateBin, "clang")+"'\n"+
		"host_os=darwin\nhost_arch=arm64\ngo_env=("+strings.Join(list, " ")+")\n"+strings.Join(pins, "\n")+"\n"+
		"release_gate_go_done() { echo DONE >> '"+log+"'; }\n"+
		"[ -n \"${release_gate_passed+set}\" ] || readonly release_gate_passed=stand-in\n", 0o644)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/bash", append([]string{script}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(append([]string{"PATH=" + bin, "HOME=" + home, "LANG=C", "TMPDIR=" + tmp}, callerBuildSettings...), env...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("%s didn't end within 30s:\n%s", script, out)
	}
	run := macBuild{out: string(out), done: -1, goEnv: goEnv, removed: map[string]int{}}
	if exitErr, ok := err.(*exec.ExitError); ok {
		run.status = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "RUN "); ok {
			who, args, _ := strings.Cut(rest, " ")
			run.calls = append(run.calls, macBuildCall{who: who, args: args, env: map[string]string{}})
		} else if kv, ok := strings.CutPrefix(line, "  "); ok && len(run.calls) > 0 {
			name, value, _ := strings.Cut(kv, "=")
			run.calls[len(run.calls)-1].env[name] = value
		} else if what, ok := strings.CutPrefix(line, "RM "); ok {
			run.removed[what] = len(run.calls)
		} else if what, ok := strings.CutPrefix(line, "CODESIGN "); ok {
			run.signed = append(run.signed, what)
		} else if line == "DONE" {
			if run.done != -1 {
				t.Errorf("it said it was done with go's folders twice")
			}
			run.done = len(run.calls)
		}
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "build")); err != nil || slices.ContainsFunc(entries, func(e fs.DirEntry) bool {
		return strings.HasSuffix(e.Name(), ".app")
	}) {
		t.Errorf("the build left an app behind (or build/ can't be read: %v)", err)
	}
	return run
}

// macBuildShellSettings are the settings sh itself gives a program it runs, beside those it was given.
var macBuildShellSettings = []string{"PWD", "OLDPWD", "SHLVL", "_"}

// A release's Mac app is built by the gate's go and C compiler, each with the gate's settings only (RT126RELGATE1 B1):
// build-app.sh with a Developer ID, and release.sh's build (build-app.sh sourced in a subshell after its gate). Its
// office check, both builds of the engine and smolcorp-spawn's compile each get go_env and the settings that step sets
// (GOOS, GOARCH, CGO_ENABLED, MACOSX_DEPLOYMENT_TARGET), none of the caller's GOFLAGS, GOTOOLCHAIN, CC, CGO_* and the
// like, and the go and cc on PATH never run; then go's folders are let go, after the last of them. tmux's build
// (RT126RELGATE1R2) runs once, fresh (build/tmux removed first), with the gate's settings and the system's PATH only.
// A copy of your own, ad hoc, is built by the go and cc on PATH with the caller's settings, and tmux's build gets them
// and its kept build, as before: the control, which shows these rows see a setting that reaches a build.
func TestReleaseGateBuildsTheMacAppWithItsPins(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("no /bin/bash")
	}
	for _, c := range []struct {
		name, script string
		args, env    []string
		status       int
	}{
		{"build-app.sh universal, a Developer ID named", "scripts/build-app.sh", []string{"universal"},
			[]string{"SMOLCORP_SIGN_IDENTITY=" + standinIdentity}, 0},
		{"release.sh, its build of the app", "scripts/release.sh", nil, nil, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			run := runMacBuild(t, c.script, c.args, "", c.env...)
			if run.status != c.status || !strings.Contains(run.out, gateStandinPassed) {
				t.Errorf("want status %d after the stand-in gate, got %d:\n%s", c.status, run.status, run.out)
			}
			var goArgs []string
			ccRuns, tmuxRuns := 0, 0
			for i, call := range run.calls {
				switch call.who {
				case "gate-go":
					goArgs = append(goArgs, call.args)
					if strings.HasPrefix(call.args, "build ") {
						for name, value := range map[string]string{"GOOS": "darwin", "CGO_ENABLED": "1",
							"MACOSX_DEPLOYMENT_TARGET": "13.0"} {
							if call.env[name] != value {
								t.Errorf("go %s: %s=%q, want %s", call.args, name, call.env[name], value)
							}
						}
						if !strings.HasSuffix(call.args, " -o /smolcorp-"+call.env["GOARCH"]+" ./cmd/smolcorp") {
							t.Errorf("go %s built for GOARCH=%s", call.args, call.env["GOARCH"])
						}
					}
				case "gate-cc":
					ccRuns++
					if !strings.Contains(call.args, "-arch arm64 -arch x86_64 ") ||
						!strings.HasSuffix(call.args, " packaging/macos/smolcorp-spawn.c") {
						t.Errorf("the gate's C compiler ran %s, want smolcorp-spawn's compile", call.args)
					}
				case "tmux-build":
					tmuxRuns++
					if call.env["PATH"] != "/usr/bin:/bin:/usr/sbin:/sbin" {
						t.Errorf("tmux's build got PATH=%s, want the system's", call.env["PATH"])
					}
					if at, ok := run.removed["-rf build/tmux"]; !ok || at > i {
						t.Errorf("tmux's build may reuse a kept build: build/tmux wasn't removed before it (removed %v)", run.removed)
					}
				default:
					t.Errorf("%s ran (%s): a release builds with the gate's go and C compiler only", call.who, call.args)
				}
				for name, value := range call.env {
					if strings.Contains(value, forgedMark) {
						t.Errorf("%s %s got the caller's %s=%s", call.who, call.args, name, value)
					} else if name == "PATH" && call.who == "tmux-build" {
						continue
					} else if want, listed := run.goEnv[name]; listed && value != want {
						t.Errorf("%s %s got %s=%s, the gate's list says %s", call.who, call.args, name, value, want)
					} else if !listed && !slices.Contains([]string{"GOOS", "GOARCH", "CGO_ENABLED", "MACOSX_DEPLOYMENT_TARGET"}, name) &&
						!slices.Contains(macBuildShellSettings, name) {
						t.Errorf("%s %s got %s=%s, which isn't in the gate's list or set by its step", call.who, call.args, name, value)
					}
				}
				for name := range run.goEnv {
					if _, ok := call.env[name]; !ok {
						t.Errorf("%s %s didn't get the gate's %s", call.who, call.args, name)
					}
				}
			}
			if len(goArgs) != 4 || goArgs[0] != "run ./scripts/goversion" || goArgs[1] != "run ./scripts/buildoffice -check" ||
				ccRuns != 1 || tmuxRuns != 1 {
				t.Errorf("want the gate's go to check its release line and the office, then build the engine twice, tmux's build once and the gate's C compiler once; got go %q, tmux %d times, cc %d times",
					goArgs, tmuxRuns, ccRuns)
			}
			if run.done != len(run.calls) {
				t.Errorf("go's folders let go after %d of %d builds, want once, after the last", run.done, len(run.calls))
			}
		})
	}

	t.Run("control: a copy of your own, ad hoc", func(t *testing.T) {
		t.Parallel()
		run := runMacBuild(t, "scripts/build-app.sh", []string{"universal"}, "", "SMOLCORP_SIGN_IDENTITY=-")
		var who []string
		leaked := map[string]bool{}
		for _, call := range run.calls {
			who = append(who, call.who)
			for name, value := range call.env {
				if strings.Contains(value, forgedMark) {
					leaked[call.who+" "+name] = true
				}
			}
		}
		_, removed := run.removed["-rf build/tmux"]
		if run.status != 0 || strings.Contains(run.out, gateStandinPassed) || run.done != -1 || removed ||
			!slices.Equal(who, []string{"path-go", "path-go", "path-go", "path-go", "tmux-build", "path-cc"}) ||
			!leaked["path-go GOFLAGS"] || !leaked["path-go CGO_CFLAGS"] || !leaked["path-cc CPATH"] ||
			!leaked["tmux-build CCC_OVERRIDE_OPTIONS"] || !leaked["tmux-build DEVELOPER_DIR"] {
			t.Errorf("want the go and cc on PATH and tmux's kept build, with the caller's settings, and no gate; got status %d, ran %q, removed %v, settings %v:\n%s",
				run.status, who, run.removed, leaked, run.out)
		}
	})

	// A Developer ID named any other way than by its whole name (by its SHA-1 hash, or part of its name, which codesign
	// takes too) isn't one build-app.sh signs with: it signs only with a name its keychain lists whole, so that build is
	// signed ad hoc, with no gate. Named whole, the gate runs and the Developer ID signs (the control).
	hash := "0123456789ABCDEF0123456789ABCDEF01234567"
	listed := "  1) " + hash + " \"" + standinIdentity + "\"\n     1 valid identities found\n"
	for _, c := range []struct {
		name, identity string
		release        bool
	}{
		{"a Developer ID by its hash", hash, false},
		{"a Developer ID by part of its name", "Stand In (STANDIN000)", false},
		{"control: a Developer ID by its whole name", standinIdentity, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			run := runMacBuild(t, "scripts/build-app.sh", []string{"universal"}, listed, "SMOLCORP_SIGN_IDENTITY="+c.identity)
			developerID := slices.ContainsFunc(run.signed, func(args string) bool { return !strings.Contains(args, " --sign - ") })
			if run.status != 0 || strings.Contains(run.out, gateStandinPassed) != c.release || len(run.signed) < 2 || developerID != c.release {
				t.Errorf("want a release %v (the gate, and signed with the Developer ID); got status %d, signed %q:\n%s",
					c.release, run.status, run.signed, run.out)
			}
		})
	}

	// And nothing in build-app.sh runs a go or a C compiler by name, where the rows above may not reach: each is
	// "$BUILD_GO" or "$BUILD_CC", after "${BUILD_ENV[@]}".
	script, err := os.ReadFile(filepath.Join("..", "scripts", "build-app.sh"))
	if err != nil {
		t.Fatal(err)
	}
	byName := regexp.MustCompile(`(^|[;&|(!]|\bthen|\bdo|\belse|=\S*)\s*(go|cc|clang|gcc|"?\$\{?(GO|CC_PROGRAM)\}?"?)\s+[a-z-]`)
	for _, line := range codeLines(strings.ReplaceAll(string(script), "\\\n", " ")) {
		if byName.MatchString(line) && !strings.Contains(line, "command -v go") {
			t.Errorf("build-app.sh runs a go or C compiler by name, not through BUILD_ENV: %s", strings.TrimSpace(line))
		}
	}
	if n := strings.Count(strings.Join(codeLines(string(script)), "\n"), `"${BUILD_ENV[@]}" `); n != 5 {
		t.Errorf("build-app.sh has %d runs through BUILD_ENV, want 5 (go's release line, the office check, the engine's builds, tmux's build, smolcorp-spawn)", n)
	}
}

// releaseScriptExts are the extensions of files a shell, an interpreter or a runtime runs as they are.
var releaseScriptExts = []string{".sh", ".bash", ".zsh", ".ksh", ".fish", ".command", ".tool", ".py", ".rb", ".pl",
	".ps1", ".psm1", ".bat", ".cmd", ".js", ".mjs", ".cjs", ".ts", ".mts"}

// releaseMakeLike are the files a task runner reads (fastlane's Fastfile among them).
var releaseMakeLike = []string{"Makefile", "GNUmakefile", "makefile", "justfile", "Justfile", ".justfile",
	"Taskfile.yml", "Taskfile.yaml", ".goreleaser.yml", ".goreleaser.yaml", "package.json", "Rakefile", "Fastfile"}

func releaseMakeFile(path string) bool {
	base := filepath.Base(path)
	return slices.Contains(releaseMakeLike, base) || strings.HasSuffix(base, ".mk")
}

// releaseActionFile is a GitHub action of the repo's own (action.yml), which a workflow job runs as its steps by
// uses: ./its/folder (RT126RELGATE1R2 B2.3), wherever it is.
func releaseActionFile(path string) bool {
	base := filepath.Base(path)
	return base == "action.yml" || base == "action.yaml"
}

// releaseWorkflowLike is a workflow, or an action of the repo's own: steps CI runs.
func releaseWorkflowLike(path string) bool {
	return strings.HasPrefix(path, ".github/workflows/") || releaseActionFile(path)
}

// releaseScanned is whether a file in the tree is one a release could be made or checked by, whatever its name
// (RT126RELGATE1 B2): a CI workflow or action, a make-like file, or a script by what it is (a known script extension,
// a #! first line, or the executable bit), but never a binary; and the Go of any program (RT126RELGATE1R2 B2.2: cmd/
// is Go's usual home for one), which go run runs as a script. The packages only such a program uses are read too
// (releaseFilesIn); the app's own Go isn't.
func releaseScanned(path string, mode fs.FileMode, data []byte) bool {
	if releaseBinary(data) {
		return false
	}
	if strings.HasSuffix(path, ".go") {
		return releaseGoProgram(path, data)
	}
	return releaseWorkflowLike(path) || releaseMakeFile(path) ||
		slices.Contains(releaseScriptExts, strings.ToLower(filepath.Ext(path))) || bytes.HasPrefix(data, []byte("#!")) ||
		mode&0o111 != 0
}

// releaseBinary is a file with a NUL in its first 8 KB.
func releaseBinary(data []byte) bool {
	return bytes.IndexByte(data[:min(len(data), 8192)], 0) >= 0
}

// releaseGoProgram is a Go file of a program (package main, not a test), wherever it is.
func releaseGoProgram(path string, data []byte) bool {
	return !strings.HasSuffix(path, "_test.go") && regexp.MustCompile(`(?m)^package main\b`).Match(data)
}

// releaseAppProgram is the app's own program: the Go it and the packages it uses aren't a release's (its engine names
// Apple's tools and gh release in the guard's rules, which isn't shipping anything).
const releaseAppProgram = "cmd/smolcorp"

// releaseFilesIn reads the files under root that releaseScanned takes, path → text, leaving out git's own folder,
// build output, downloaded tools and the folders a maintainer keeps beside the code and never commits. Then, as
// they're run too: the packages of the module that a program other than the app uses and the app doesn't
// (RT126RELGATE1R2 B2.2), and any file a release script, or a file it runs, hands to a shell or an interpreter, whatever
// its name or mode (RT126RELGATE1R2: bash scripts/lib/release-common).
func releaseFilesIn(t *testing.T, root string) map[string]string {
	t.Helper()
	texts, files := map[string]string{}, map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if slices.Contains([]string{".git", "build", ".build", ".swiftpm", "node_modules", "js-tools"}, entry.Name()) ||
				slices.Contains([]string{"sandbox", "notes-out"}, rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if releaseBinary(data) {
			return nil
		}
		texts[rel] = string(data)
		if releaseScanned(rel, info.Mode(), data) {
			files[rel] = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range releaseProgramPackages(t, texts) {
		files[path] = texts[path]
	}
	for _, path := range releaseRunFiles(texts, files) {
		files[path] = texts[path]
	}
	return files
}

// releaseProgramPackages are the non-test Go files of the module's packages that a program (package main) other than
// the app imports, directly or not, and the app doesn't: the rest of that program.
func releaseProgramPackages(t *testing.T, texts map[string]string) []string {
	t.Helper()
	module := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindStringSubmatch(texts["go.mod"])
	if module == nil {
		return nil
	}
	imports, goFiles, programs := map[string][]string{}, map[string][]string{}, map[string]bool{}
	for path, text := range texts {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, text, parser.ImportsOnly)
		if err != nil {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(path))
		goFiles[dir] = append(goFiles[dir], path)
		if parsed.Name.Name == "main" {
			programs[dir] = true
		}
		for _, spec := range parsed.Imports {
			if local, ok := strings.CutPrefix(strings.Trim(spec.Path.Value, "\"`"), module[1]+"/"); ok {
				imports[dir] = append(imports[dir], local)
			}
		}
	}
	closure := func(dir string) map[string]bool {
		seen := map[string]bool{}
		next := []string{dir}
		for len(next) > 0 {
			d := next[0]
			next = next[1:]
			if !seen[d] {
				seen[d] = true
				next = append(next, imports[d]...)
			}
		}
		return seen
	}
	app := closure(releaseAppProgram)
	var found []string
	for program := range programs {
		if program == releaseAppProgram {
			continue
		}
		for dir := range closure(program) {
			if !app[dir] && !programs[dir] {
				found = append(found, goFiles[dir]...)
			}
		}
	}
	slices.Sort(found)
	return slices.Compact(found)
}

// releaseInterpreterRun is a line that runs a program that runs a file it's handed as code (a shell, an interpreter,
// eval, or . and source) where a command starts, by its name or a path to it: bash x, cat x | sh, eval "$(cat x)",
// subprocess.run(["bash", x]) once flattened. A message that names a file isn't one.
var releaseInterpreterRun = regexp.MustCompile(`(?:^|[;&|(!{]|\$\(|\b(?:then|do|else|elif|if|while|until|exec|command|builtin|` +
	`time|nohup|env|xargs)\b)\s*(?:\w+=\S*\s+)*(?:\S*/)?(?:(?:ba|z|k|da|fi|c|tc)?sh|python[0-9.]*|node|deno|bun|ruby|perl|` +
	`pwsh|osascript|eval|source|\.)(?:\s|$)`)

// releaseRunFiles are the files of the tree, beyond those already read, that a release script (or a file it names, in
// turn) names on a line that runs a shell or an interpreter: bash x, sh < x, . x, eval "$(cat x)". A file with no
// script extension, #! line or executable bit can only be run that way.
func releaseRunFiles(texts, files map[string]string) []string {
	read := map[string]bool{}
	var found []string
	next := slices.Clone(gatedScripts)
	for len(next) > 0 {
		path := next[0]
		next = next[1:]
		text, ok := files[path]
		if !ok {
			text, ok = texts[path]
		}
		if !ok || read[path] {
			continue
		}
		read[path] = true
		for _, line := range releaseLines(path, text) {
			words := strings.Fields(regexp.MustCompile(`[<>|;&()]`).ReplaceAllString(flatArgv(line), " "))
			interpreted := releaseInterpreterRun.MatchString(line) || releaseInterpreterRun.MatchString(flatArgv(line))
			for _, word := range words {
				word = regexp.MustCompile(`^(?:\$\{?\w+\}?/|\./)+`).ReplaceAllString(word, "")
				for _, name := range []string{word, filepath.ToSlash(filepath.Join(filepath.Dir(path), word))} {
					_, exists := texts[name]
					_, already := files[name]
					switch {
					case !exists || read[name]:
					case already:
						// A script it runs: what that one runs is followed too.
						next = append(next, name)
					case interpreted:
						if !slices.Contains(found, name) {
							found = append(found, name)
						}
						next = append(next, name)
					}
				}
			}
		}
	}
	slices.Sort(found)
	return found
}

// releaseFiles reads the tree's files releaseScanned takes.
func releaseFiles(t *testing.T) map[string]string {
	t.Helper()
	return releaseFilesIn(t, "..")
}

// flatArgv is a line with its argv spellings flattened (RT126RELGATE1R2 B2.1): quotes, commas and brackets dropped
// and spaces made single, so a program run by a list of its words, ["gh", "release", "upload", tag] in Python,
// ['gh', ['release', 'create']] in JavaScript or exec.Command("gh", "release", "edit") in Go, reads as the shell
// spelling, gh release upload tag.
func flatArgv(line string) string {
	return strings.Join(strings.FieldsFunc(line, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("\"'`,[]", r)
	}), " ")
}

// codeLines are a file's lines that aren't comments (shell's and YAML's #).
func codeLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			lines = append(lines, line)
		}
	}
	return lines
}

// codeLinesOf are a file's lines that aren't comments in its language (// in Go and JavaScript, # elsewhere), and
// aren't lines releaseLetBe lets be.
func codeLinesOf(path, text string) []string {
	slash := slices.Contains([]string{".go", ".js", ".mjs", ".cjs", ".ts", ".mts"}, filepath.Ext(path))
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || (slash && strings.HasPrefix(trimmed, "//")) || (!slash && strings.HasPrefix(trimmed, "#")) {
			continue
		}
		if _, let := releaseLetBe[path][trimmed]; !let {
			lines = append(lines, line)
		}
	}
	return lines
}

// releaseLines are a file's code lines (codeLinesOf), each joined with the next while it ends in a line's
// continuation (\) or inside a list of words (an open ( or [, or a comma): exec.Command("gh", "release",\n"upload")
// and a shell line ending in \ with "release upload" on the next are one line each.
func releaseLines(path, text string) []string {
	var lines []string
	joining := false
	for _, line := range codeLinesOf(path, text) {
		trimmed := strings.TrimSpace(line)
		if joining {
			lines[len(lines)-1] += " " + strings.TrimSuffix(trimmed, "\\")
		} else {
			lines = append(lines, strings.TrimSuffix(line, "\\"))
		}
		joining = strings.HasSuffix(trimmed, "\\") || strings.HasSuffix(trimmed, ",") || strings.HasSuffix(trimmed, "(") ||
			strings.HasSuffix(trimmed, "[")
	}
	return lines
}

// releaseLetBe are the lines that look like a release's work or the gate's and aren't, file → the line, trimmed → why.
// Each must still be in its file (TestReleaseGateIsTheOnlyCopyOfItsChecks), so none outlives its line.
var releaseLetBe = map[string]map[string]string{
	".github/workflows/ci.yml": {
		`Linux) asset=Smolcorp-Linux-x86_64.tar.gz; program=smolcorp ;;`:          "UPGRADE1's upgrade job: the name of 1.24's own release download, which it fetches to install this build over; it makes, signs and sends nothing",
		`Windows) asset=Smolcorp-Windows-x86_64.zip; program=smolcorp-cli.exe ;;`: "the same",
	},
	"packaging/macos/dmg/settings.py": {
		`app = defines.get("app", "build/Smolcorp.app")  # noqa: F821, defined by dmgbuild`: "dmgbuild's settings, which only make-dmg.sh reads, after the gate: dmgbuild is named in a comment",
		`format = defines.get("format", "UDZO")  # noqa: F821, defined by dmgbuild`:         "the same",
	},
	"scripts/check-runtime.sh": {
		`said="$(env -i "$program" -S /dev/null/smolcorp-guard-check has-session 2>&1)" || status=$?`: "the guarded tmux run with no settings at all, to see it refuse: neither a go nor the gate's pins",
	},
	"scripts/vmpack.sh": {
		`(cd "$SRC" && CGO_ENABLED="$CGO_" GOOS="$GOOS_" GOARCH="$GOARCH_" go test -c -o "$bin" "./$pkg")`:        "SA-L1's pack for the Linux VM: go test -c builds the test binaries of a tree, for the VM to run; no go test runs here, nor the gate's",
		`echo "Built from tree $TREE ($1) by scripts/vmpack.sh: $GOOS_/$GOARCH_, CGO_ENABLED=$CGO_, go test -c."`: "the same, said in the pack's RUN note",
	},
	"scripts/publish-release.sh": {
		`check "origin/main can't be fetched." git fetch --quiet origin main`:                                                 "what it publishes onto, after the gate passed",
		`echo "  one commit on origin/main ($(git rev-parse --short origin/main)) with $BRANCH's files, on $RELEASE_BRANCH"`:  "the same, said",
		`COMMIT="$(git commit-tree "$TREE" -p origin/main -m "Smolcorp $VERSION" -m "See RELEASE-NOTES.md for what's new.")"`: "the commit it publishes, after the gate passed (RT126RELGATE1's nit: not the signed commit the gate checked)",
		`check "main couldn't be fetched after the merge." git fetch --quiet origin main`:                                     "main once that commit's pull request is merged, to tag the merge",
	},
}

// shipsADownloadNeedles are what a line shipsADownload matches must have one of, looked for first. Each is a word
// flatArgv keeps as it is, so the line is flattened only when one is there.
var shipsADownloadNeedles = []string{"notarytool", "stapler", "release", "hdiutil", "dmgbuild", "ditto", "zip", "-c",
	"Smolcorp", "notarize", "upload_to_app_store", "build_mac_app"}

// shipsADownload is a line, its argv spellings flattened, that makes, signs, notarises or sends a download: the DMG,
// the zip and the archives, Apple's notary, a GitHub release (by gh, or by any action with release in its name:
// ncipollo/release-action, softprops/action-gh-release, goreleaser), fastlane's notarize and release actions.
var shipsADownload = regexp.MustCompile(`notarytool|stapler staple|gh release (create|upload|edit)|hdiutil (create|convert)` +
	`|dmgbuild|ditto -c|(^|[^a-z])zip -q|\btar\b[^|]*\s-c[a-z]*z|uses:\s*[^\s@#]*release[^\s@#]*@|upload-release-asset` +
	`|\bnotarize\b|set_github_release|upload_to_app_store|build_mac_app` +
	`|Smolcorp(\.dmg|\.zip|-Windows-[^ ]*\.zip|-Linux-[^ ]*\.tar\.gz)`)

// goShipsADownload is a line of Go that does, beside what shipsADownload finds in it flattened: Apple's tools by
// name, an archive written.
var goShipsADownload = regexp.MustCompile(`"(notarytool|stapler|hdiutil|dmgbuild|ditto|codesign|productsign|altool)"` +
	`|\b(zip|tar|gzip)\.NewWriter\b|Smolcorp(\.dmg|\.zip|-Windows-[^ "]*\.zip|-Linux-[^ "]*\.tar\.gz)`)

// lineShipsADownload is whether a line of the file at path ships a download: as written or flattened.
func lineShipsADownload(path, line string) bool {
	if strings.HasSuffix(path, ".go") && goShipsADownload.MatchString(line) {
		return true
	}
	if releaseMakeFile(path) && strings.Contains(line, "go build") {
		return true
	}
	return slices.ContainsFunc(shipsADownloadNeedles, func(n string) bool { return strings.Contains(line, n) }) &&
		shipsADownload.MatchString(flatArgv(line))
}

// goHasAGitHubRelease is whether a Go file has the words of a GitHub release by gh as string literals of their own,
// wherever they are in it (RT126RELGATE1R3's follow-up): "gh" (or a path ending in /gh), "release", and "create",
// "upload" or "edit". So gh's words gathered in a slice first, args := []string{"release", "upload", tag} and then
// exec.Command("gh", args...), are found, as are words appended later or kept in the file's own variables.
//
// What a text scan can't see, here or in any language (said, not half caught): words that aren't literals in the
// file that runs them. A word built at run time ("g"+"h", strings.Fields("release upload"), base64, a file's
// contents), passed in from another file or package, or, in Python, a program found first (GH = shutil.which("gh"),
// then run([GH, "release", …])) isn't found.
func goHasAGitHubRelease(path, text string) bool {
	parsed, err := parser.ParseFile(token.NewFileSet(), path, text, parser.SkipObjectResolution)
	if err != nil {
		return false
	}
	words := map[string]bool{}
	ast.Inspect(parsed, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if word, err := strconv.Unquote(lit.Value); err == nil {
				words[word] = true
				if strings.HasSuffix(word, "/gh") {
					words["gh"] = true
				}
			}
		}
		return true
	})
	return words["gh"] && words["release"] && (words["create"] || words["upload"] || words["edit"])
}

// gatedScripts are the scripts that source the gate, each once: the shipping paths, and what a CI job may run to ship.
var gatedScripts = []string{"scripts/build-app.sh", "scripts/make-dmg.sh", "scripts/publish-release.sh",
	"scripts/release-desktop.sh", "scripts/release.sh"}

// shippingFiles are the files, of those given, that ship a download in a line of their own; for a workflow, its jobs
// that do so without running the gate or a script that sources it (as "file: job"), with the steps of any action of
// the repo's own a job uses. An action is judged in the jobs that use it.
func shippingFiles(files map[string]string) []string {
	var found []string
	for path, text := range files {
		if strings.HasPrefix(path, ".github/workflows/") {
			found = append(found, ungatedJobs(path, text, files)...)
			continue
		}
		if path == releaseGatePath || releaseActionFile(path) {
			continue
		}
		if strings.HasSuffix(path, ".go") && goHasAGitHubRelease(path, text) ||
			slices.ContainsFunc(releaseLines(path, text), func(line string) bool { return lineShipsADownload(path, line) }) {
			found = append(found, path)
		}
	}
	slices.Sort(found)
	return found
}

// localActions is the text of each action of the repo's own that a text uses (uses: ./folder), and of those they use
// in turn, each once.
func localActions(text string, files map[string]string, seen map[string]bool) string {
	var steps []string
	for _, m := range regexp.MustCompile(`(?m)uses:\s*["']?\./([^\s"'@#]*)`).FindAllStringSubmatch(text, -1) {
		// uses: ./ is the action at the repo's root (RT126RELGATE1R3's follow-up): its folder is ".", its file action.yml.
		dir := filepath.Clean(m[1])
		for _, name := range []string{filepath.ToSlash(filepath.Join(dir, "action.yml")), filepath.ToSlash(filepath.Join(dir, "action.yaml"))} {
			if action, ok := files[name]; ok && !seen[name] {
				seen[name] = true
				steps = append(steps, strings.Join(codeLines(action), "\n"), localActions(action, files, seen))
			}
		}
	}
	return strings.Join(steps, "\n")
}

// ungatedJobs are a workflow's jobs that ship a download (or, in a workflow started by a tag or a release, any job)
// without running the gate or a script that sources it. A job's steps include those of the repo's own actions it uses.
func ungatedJobs(path, text string, files map[string]string) []string {
	var found []string
	jobs := regexp.MustCompile(`(?m)^jobs:\s*$`).FindStringIndex(text)
	if jobs == nil {
		return nil
	}
	released := regexp.MustCompile(`(?m)^\s+(tags|release|release_published):`).MatchString(text[:jobs[0]]) ||
		regexp.MustCompile(`(?m)^on:.*\b(release)\b`).MatchString(text[:jobs[0]])
	names := regexp.MustCompile(`(?m)^  ([A-Za-z0-9_-]+):\s*$`).FindAllStringSubmatchIndex(text[jobs[1]:], -1)
	for i, m := range names {
		end := len(text) - jobs[1]
		if i+1 < len(names) {
			end = names[i+1][0]
		}
		// A line releaseLetBe lets be in the workflow is left out of its job (codeLinesOf), as it is everywhere else.
		body := strings.Join(codeLinesOf(path, text[jobs[1]+m[1]:jobs[1]+end]), "\n")
		body += "\n" + localActions(body, files, map[string]bool{})
		ships := released || shipsADownload.MatchString(body) || slices.ContainsFunc(strings.Split(body, "\n"), func(line string) bool {
			return lineShipsADownload(path, line)
		})
		// build-app.sh runs the gate only for a Developer ID, so it doesn't count.
		gated := false
		for _, script := range append([]string{releaseGatePath}, gatedScripts...) {
			if script != "scripts/build-app.sh" && strings.Contains(body, script) {
				gated = true
			}
		}
		if ships && !gated {
			found = append(found, path+": "+text[jobs[1]+m[2]:jobs[1]+m[3]])
		}
	}
	return found
}

// sourceCommand is a . or source where a command starts: the file, and what follows it.
var sourceCommand = regexp.MustCompile(`(?:^|[;&|({!]|\b(?:then|do|else|elif|if|while|until)\b)\s*(?:builtin\s+|command\s+)?` +
	`(?:\.|source)\s+([^\s;&|)]+)((?:[ \t]+[^\s;&|)]+)*)`)

// releaseSourced are what a release script may source: the gate, and in release.sh the app's build and the DMG, each
// in its subshell after the gate (as "file args").
var releaseSourced = map[string][]string{"scripts/release.sh": {"scripts/build-app.sh universal", "scripts/make-dmg.sh"}}

// wrongSources are the lines, of the files given, where a release script sources a file other than those it may: a
// copy of the gate, or of any of it, kept in a file of its own (RT126RELGATE1 B2), as "file: what it sources".
func wrongSources(files map[string]string) []string {
	var found []string
	for _, path := range gatedScripts {
		for _, line := range codeLinesOf(path, files[path]) {
			for _, m := range sourceCommand.FindAllStringSubmatch(line, -1) {
				what := strings.TrimSpace(m[1] + m[2])
				if what != releaseGatePath && !slices.Contains(releaseSourced[path], what) {
					found = append(found, path+": "+what)
				}
			}
		}
	}
	slices.Sort(found)
	return found
}

// The sweep: every file that makes, signs, notarises or sends a download is a script that sources the gate, once (and
// its rows above show it stops there), and sources nothing else; release.sh runs build-app.sh and make-dmg.sh only in
// subshells of their own, after its gate; no CI job ships around it; and the gate's top names each path.
func TestReleaseGateEveryShippingPathGoesThroughIt(t *testing.T) {
	files := releaseFiles(t)
	gate, ok := files[releaseGatePath]
	if !ok {
		t.Fatalf("no %s", releaseGatePath)
	}
	for _, path := range shippingFiles(files) {
		if !slices.Contains(gatedScripts, path) {
			t.Errorf("%s makes, signs or sends a download, and isn't one of the paths that run the gate first", path)
		}
	}
	for _, path := range gatedScripts {
		n := 0
		for _, line := range codeLines(files[path]) {
			if strings.HasSuffix(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ";;")), releaseGateLine) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s sources the gate %d times, want once", path, n)
		}
		if !strings.Contains(gate, "\n#   "+path+" ") {
			t.Errorf("the gate's top doesn't name %s", path)
		}
	}
	for _, wrong := range wrongSources(files) {
		t.Errorf("a release script sources what isn't the gate: %s", wrong)
	}
	release := regexp.MustCompile(`"[^"\n]*"`).ReplaceAllString(strings.Join(codeLines(files["scripts/release.sh"]), "\n"), `""`)
	dmg := regexp.MustCompile(`(?m)^\(\n  SMOLCORP_DEVELOPER_ID="\$DEVELOPER_ID"\n  \. scripts/make-dmg\.sh\n\)$`)
	if strings.Count(release, "make-dmg.sh") != 1 || !dmg.MatchString(files["scripts/release.sh"]) {
		t.Errorf("release.sh must run make-dmg.sh once, sourced in a subshell of its own after its gate")
	}
	app := regexp.MustCompile(`(?m)^if \[ "\$DMG_ONLY" = 0 \]; then\n  \(\n    export SMOLCORP_SIGN_IDENTITY=-\n` +
		`    \. scripts/build-app\.sh universal\n  \)\nfi$`)
	if strings.Count(release, "build-app.sh") != 1 || !app.MatchString(files["scripts/release.sh"]) {
		t.Errorf("release.sh must run build-app.sh once, ad hoc, sourced in a subshell of its own after its gate")
	}

	// The sweep's own rows: what it must find, and what it must let be.
	for _, c := range []struct {
		name, path, text string
		want             bool
	}{
		{"a script that notarises", "scripts/ship.sh", "xcrun notarytool submit build/x.dmg\n", true},
		{"a script that makes a release archive", "scripts/ship.sh", "tar --uid 0 -czf build/Smolcorp-Linux-x.tar.gz x\n", true},
		{"a script that zips", "scripts/ship.sh", "(cd build && zip -qrX ../x.zip x)\n", true},
		{"a script that only unzips", "scripts/fetch.sh", "unzip -q x.zip\n", false},
		{"a script that names a release in a comment", "scripts/fetch.sh", "# gh release create v1\n", false},
		{"a Makefile that builds", "Makefile", "release:\n\tgo build -o build/smolcorp ./cmd/smolcorp\n", true},
		{"a workflow job that publishes", ".github/workflows/r.yml",
			"on: push\njobs:\n  ship:\n    steps:\n      - run: gh release create v1 build/x.zip\n", true},
		{"a workflow job that publishes after the gate", ".github/workflows/r.yml",
			"on: push\njobs:\n  ship:\n    steps:\n      - run: scripts/release-gate.sh\n      - run: gh release upload v1 x\n", false},
		{"a workflow job that runs release-desktop.sh", ".github/workflows/r.yml",
			"on: push\njobs:\n  ship:\n    steps:\n      - run: scripts/release-desktop.sh\n      - run: gh release upload v1 x\n", false},
		{"a workflow started by a tag, building", ".github/workflows/r.yml",
			"on:\n  push:\n    tags: ['v*']\njobs:\n  build:\n    steps:\n      - run: go build ./cmd/smolcorp\n", true},
		{"a workflow job with a release action", ".github/workflows/r.yml",
			"on: push\njobs:\n  ship:\n    steps:\n      - uses: softprops/action-gh-release@v2\n", true},
		{"a workflow job keeping a release archive", ".github/workflows/r.yml",
			"on: push\njobs:\n  ship:\n    steps:\n      - uses: actions/upload-artifact@v4\n        with:\n          path: build/Smolcorp.dmg\n", true},
		{"a Go program that notarises", "tools/ship/main.go",
			"package main\n\nfunc main() { exec.Command(\"xcrun\", \"notarytool\", \"submit\", \"x.dmg\").Run() }\n", true},
		{"a Go program that writes a zip", "scripts/pack/main.go", "package main\n\nfunc main() { zip.NewWriter(f) }\n", true},
		{"a Go program that says it in a comment", "scripts/pack/main.go", "package main\n\n// zip.NewWriter(f)\n", false},
		{"dmgbuild's settings, its comment let be", "packaging/macos/dmg/settings.py",
			`app = defines.get("app", "build/Smolcorp.app")  # noqa: F821, defined by dmgbuild` + "\n", false},
	} {
		if got := len(shippingFiles(map[string]string{c.path: c.text})) > 0; got != c.want {
			t.Errorf("%s: found shipping %v, want %v", c.name, got, c.want)
		}
	}
}

// gateMark is a piece of the gate's code, or of what it does, by any of its usual spellings: a script that has one
// keeps a copy of the gate (RT126RELGATE1 B2: the marks are the gate's actions, not its exact text, so a renamed
// function or another spelling of the same check is found too). Its regexp can't match a line without one of its
// needles, which are looked for first: the tree's scripts include megabytes of the office's JavaScript. ci marks CI's
// own checks of every push may have.
type gateMark struct {
	name    string
	needles []string
	re      *regexp.Regexp
	ci      bool
}

// in is whether line has the mark, as written or with its argv spellings flattened (flatArgv).
func (m gateMark) in(line string) bool {
	return m.inEither(line, func() string { return flatArgv(line) })
}

// inEither is in, with the flattened line given by flat, which is asked at most once a line by its caller.
func (m gateMark) inEither(line string, flat func() string) bool {
	return slices.ContainsFunc(m.needles, func(n string) bool { return strings.Contains(line, n) }) &&
		(m.re.MatchString(line) || m.re.MatchString(flat()))
}

// releaseGitOptions are git's own options before its command: -C dir, -c name=value, --flag.
const releaseGitOptions = `(\s+(-[Cc]\s+\S+|--\S+))*`

var releaseGateMarks = []gateMark{
	{"the bundled tmux's test", []string{tmuxTestName}, regexp.MustCompile(regexp.QuoteMeta(tmuxTestName)), false},
	{"its control run", []string{"TMUXGATE", "tmuxgate"}, regexp.MustCompile(`SMOLCORP_TMUXGATE_CONTROL|tmuxgate control:`), false},
	{"the heavy rows' setting", []string{releaseGateRows}, regexp.MustCompile(regexp.QuoteMeta(releaseGateRows)), false},
	{"the release's own tests", []string{"TestRelease"}, regexp.MustCompile(`TestRelease`), false},
	{"the forged-event table", []string{"forgedevents"}, regexp.MustCompile(`forgedevents`), false},
	{"a go test run", []string{"test"}, regexp.MustCompile(`\bgo\W{1,4}test\b|\$\{?GO\}?"?\s+test\b`), true},
	{"go's pinned settings", []string{"GOTOOLCHAIN", "GOFLAGS", "GOENV", "GOWORK"},
		regexp.MustCompile(`\b(GOTOOLCHAIN|GOFLAGS|GOENV|GOWORK)\s*[=:]`), false},
	{"an empty environment", []string{"env", "exec"},
		regexp.MustCompile(`\benv\W{1,4}(-[A-Za-z]*i[A-Za-z]*|--ignore-environment)\b|\bexec\s+-[a-z]*c\b|\benv\s+-(\s|$)` +
			`|\benv\s*[=:]\s*(\{\s*[^.*\s}]|dict\(\s*[^*)\s])`), false},
	{"the pins' list and runners", []string{"go_env", "pinned_"}, regexp.MustCompile(`go_env=\(|pinned_(env|go)\s*\(\)`), false},
	{"HEAD's tree", []string{"heads_tree", "write-tree"}, regexp.MustCompile(`heads_tree\s*\(\)|write-tree`), false},
	{"origin/main", []string{"origin", "@{u", "refs/remotes"},
		regexp.MustCompile(`origin[/ ]main\b|\borigin/|refs/remotes/|@\{u(pstream)?\}`), false},
	{"a release commit", []string{"commit-tree"}, regexp.MustCompile(`commit-tree`), false},
	{"HEAD as a push sends it", []string{"GIT_NO_REPLACE_OBJECTS", "cat-file", "info/grafts"},
		regexp.MustCompile(`GIT_NO_REPLACE_OBJECTS|cat-file\s+commit|info/grafts`), false},
	{"files git doesn't look at", []string{"ls-files", "sparseCheckout", "skip-worktree", "assume-unchanged"},
		regexp.MustCompile(`ls-files\b[^;|&]*\s-[A-Za-z]*v\b|core\.sparseCheckout|skip-worktree|assume-unchanged`), false},
	{"a clean check", []string{"porcelain", "git", "ls-files"},
		regexp.MustCompile(`--porcelain|\bgit` + releaseGitOptions + `\s+status\b|\bgit` + releaseGitOptions + `\s+diff-(index|files)\b` +
			`|\bgit` + releaseGitOptions + `\s+diff\b[^;|&]*\s--(quiet|exit-code|name-only|name-status|stat|shortstat|numstat)\b` +
			`|ls-files\b[^;|&]*\s(-[A-Za-z]*[mdoku][A-Za-z]*|--(modified|deleted|others|killed|unmerged))\b` +
			// git diff's output taken to judge the tree by (RT126RELGATE1R3's follow-up), whatever its options:
			// $(git diff HEAD), `git diff`, git diff | wc -l, and a captured run of it: Python's check_output, getoutput,
			// getstatusoutput, os.popen, and run or Popen with capture_output=True or stdout=; JavaScript's execSync,
			// execFileSync and spawnSync.
			`|(\$\(|` + "`" + `|\b(check_output|getoutput|getstatusoutput|popen|execSync|execFileSync|spawnSync)\(\s*)\s*git` +
			releaseGitOptions + `\s+diff\b|\bgit` + releaseGitOptions + `\s+diff\b[^;|&]*\|([^|]|$)` +
			`|\b(run|Popen)\(\s*git` + releaseGitOptions + `\s+diff\b[^;]*\b(capture_output\s*=\s*True|stdout\s*=)`), false},
	{"the public check", []string{"check-public"}, regexp.MustCompile(`check-public`), true},
}

// gateCopies are the lines, of the files given, that copy the gate's code, as "file: line". The Go of a program isn't
// read for them (scripts/forgedevents is the gate's own); CI's own check of every push may run go test and
// check-public.sh.
func gateCopies(files map[string]string) []string {
	var found []string
	for path, text := range files {
		if path == releaseGatePath || path == "scripts/check-public.sh" || strings.HasSuffix(path, ".go") {
			continue
		}
		ci := releaseWorkflowLike(path)
		for _, line := range releaseLines(path, text) {
			flat := sync.OnceValue(func() string { return flatArgv(line) })
			for _, mark := range releaseGateMarks {
				if mark.inEither(line, flat) && !(ci && mark.ci) {
					found = append(found, path+": "+strings.TrimSpace(line))
					break
				}
			}
		}
	}
	slices.Sort(found)
	return found
}

// No script keeps its own copy of the gate: none of its code, under any name, is anywhere but scripts/release-gate.sh.
func TestReleaseGateIsTheOnlyCopyOfItsChecks(t *testing.T) {
	files := releaseFiles(t)
	if found := gateCopies(files); len(found) > 0 {
		t.Errorf("gate code outside %s:\n  %s", releaseGatePath, strings.Join(found, "\n  "))
	}
	gate, err := os.ReadFile(filepath.Join("..", releaseGatePath))
	if err != nil {
		t.Fatal(err)
	}
	for _, mark := range releaseGateMarks {
		if !mark.in(string(gate)) {
			t.Errorf("the gate has no %s (%s): a mark of its code that isn't in it finds no copy", mark.name, mark.re)
		}
	}
	for path, lines := range releaseLetBe {
		var trimmed []string
		for _, line := range strings.Split(files[path], "\n") {
			trimmed = append(trimmed, strings.TrimSpace(line))
		}
		for line := range lines {
			if !slices.Contains(trimmed, line) {
				t.Errorf("%s has no line %q any more: take it out of releaseLetBe", path, line)
			}
		}
	}
	for _, c := range []struct{ name, path, text string }{
		{"the tmux test's name", "scripts/release.sh", "go test -run '^" + tmuxTestName + "$' ./desktop\n"},
		{"the forged-event table", "scripts/release.sh", "go run ./scripts/forgedevents\n"},
		{"the pins", "scripts/build-app.sh", "/usr/bin/env -i HOME=\"$HOME\" GOTOOLCHAIN=local go build ./cmd/smolcorp\n"},
		{"the squash check", "scripts/make-dmg.sh", "BASE=\"$(git rev-parse refs/remotes/origin/main)\"\n"},
		{"its own check-public", "scripts/publish-release.sh", "scripts/check-public.sh\n"},
		{"a CI job's own pins", ".github/workflows/r.yml", "      - run: GOFLAGS=-mod=readonly go test ./desktop\n"},
		{"a CI job running the release's own tests", ".github/workflows/r.yml", "      - run: go test -run '^TestRelease' ./desktop\n"},
		{"a clean check by git diff", "scripts/release.sh", "git diff --quiet HEAD || exit 1\n"},
		{"files git doesn't look at", "scripts/release.sh", "[ -z \"$(git ls-files -v | grep -v '^H ')\" ]\n"},
		{"a release commit, unsigned", "scripts/release.sh", "git commit-tree -p origin/main HEAD^{tree}\n"},
		{"replace refs off", "scripts/release.sh", "export GIT_NO_REPLACE_OBJECTS=1\n"},
		{"the pins in Python", "scripts/ship.py", "subprocess.run(['env', '-i', 'go', 'build'])\n"},
		{"the squash check in JavaScript", "scripts/ship.mjs", "execSync('git rev-parse origin/main')\n"},
	} {
		if found := gateCopies(map[string]string{c.path: c.text}); len(found) != 1 {
			t.Errorf("%s in %s: found %q, want it found", c.name, c.path, found)
		}
	}
	if found := gateCopies(map[string]string{".github/workflows/ci.yml": "      - run: scripts/check-public.sh\n      - run: go test ./...\n",
		"scripts/x.sh": "# go run ./scripts/forgedevents, said in a comment\n", "scripts/x.mjs": "// git rev-parse origin/main\n",
		"scripts/forgedevents/main.go": "package main\n\nvar settings = []string{\"GOFLAGS=-mod=readonly\"}\n",
		"scripts/check-runtime.sh":     `said="$(env -i "$program" -S /dev/null/smolcorp-guard-check has-session 2>&1)" || status=$?` + "\n",
	}); len(found) > 0 {
		t.Errorf("CI's own checks, comments, the gate's own Go and a line let be: found %q, want none", found)
	}
}

// RT126RELGATE1 B2's probes, each a row: a copy of the gate in a sourced file that isn't a .sh, a release helper with
// no extension (and its kin in Python, PowerShell, zsh, a .command, a program by its executable bit only, JavaScript and
// Go) that notarises and publishes around the gate, and release.sh's own copies of the gate's checks under other names
// and spellings. Written as files in a tree of their own, they're each found: by what they are, not by their names.
func TestReleaseGateSeesItsCopiesUnderAnyName(t *testing.T) {
	gate, err := os.ReadFile(filepath.Join("..", releaseGatePath))
	if err != nil {
		t.Fatal(err)
	}
	release, err := os.ReadFile(filepath.Join("..", "scripts", "release.sh"))
	if err != nil {
		t.Fatal(err)
	}
	renamed := []string{
		`same_tree() { [ "$(git write-tree)" = "$(git rev-parse 'HEAD^{tree}')" ] && [ -z "$(git status --porcelain --untracked-files=all)" ]; }`,
		`if [ "$(git rev-parse HEAD^ 2>/dev/null)" != "$(git rev-parse origin/main 2>/dev/null)" ]; then echo "Stopped: not one commit onto origin/main."; exit 1; fi`,
		`own_go() { env -i HOME="$HOME" PATH=/usr/bin:/bin go "$@"; }`,
	}
	gateIf := "\n  " + releaseGateLine + "\nfi\n"
	withCopies := strings.Replace(string(release), gateIf, gateIf+
		"# Release.sh's own look at the release commit.\n"+strings.Join(renamed, "\n")+"\n. scripts/lib/release-checks.bash\n", 1)
	if withCopies == string(release) {
		t.Fatal("release.sh has no gate line to put the copies after")
	}
	ship := "#!/bin/bash\nset -euo pipefail\n. scripts/lib/release-checks.bash\n" +
		"xcrun notarytool submit build/Smolcorp.dmg --keychain-profile p --wait\ngh release upload v1 build/Smolcorp.dmg\n"
	dir := t.TempDir()
	probes := []struct {
		path, text string
		mode       os.FileMode
		// copies and ships are what the scan must find in it.
		copies, ships bool
	}{
		{"scripts/lib/release-checks.bash", string(gate), 0o644, true, false},
		{"scripts/ship-mac", ship, 0o755, false, true},
		{"scripts/package-dmg", "hdiutil create -srcfolder build/Smolcorp.app build/Smolcorp.dmg\n", 0o755, false, true},
		{"scripts/ship.py", "subprocess.run(['xcrun', 'notarytool', 'submit', 'build/Smolcorp.dmg'])\n", 0o644, false, true},
		{"scripts/ship.ps1", "gh release upload v1 build/Smolcorp-Windows-x86_64.zip\n", 0o644, false, true},
		{"scripts/ship.zsh", "xcrun notarytool submit build/x.dmg --wait\n", 0o644, false, true},
		{"scripts/Ship.command", "gh release create v1 build/x.zip\n", 0o644, false, true},
		{"scripts/ship.mjs", "execSync('gh release upload v1 build/Smolcorp.zip')\n", 0o644, false, true},
		{"tools/ship/main.go", "package main\n\nimport \"os/exec\"\n\nfunc main() { exec.Command(\"xcrun\", \"notarytool\", \"submit\", \"build/Smolcorp.dmg\").Run() }\n",
			0o644, false, true},
		{"scripts/release.sh", withCopies, 0o755, true, true},
	}
	for _, p := range probes {
		writeReleaseFile(t, filepath.Join(dir, p.path), p.text, p.mode)
	}
	// What isn't a script is let be: a binary with the executable bit, a note, and the app's own Go.
	writeReleaseFile(t, filepath.Join(dir, "scripts", "a.out"), "\x7fELF\x00xcrun notarytool submit build/Smolcorp.dmg\n", 0o755)
	writeReleaseFile(t, filepath.Join(dir, "docs", "notes.md"), "gh release upload v1 build/Smolcorp.dmg\n", 0o644)
	writeReleaseFile(t, filepath.Join(dir, "desktop", "ship.go"), "package desktop\n\nvar _ = \"notarytool\"\n", 0o644)

	files := releaseFilesIn(t, dir)
	copies, ships, sources := gateCopies(files), shippingFiles(files), wrongSources(files)
	for _, p := range probes {
		if _, ok := files[p.path]; !ok {
			t.Errorf("%s: not read as a script", p.path)
		}
		copied := slices.ContainsFunc(copies, func(f string) bool { return strings.HasPrefix(f, p.path+": ") })
		if p.copies && !copied {
			t.Errorf("%s: no copy of the gate found in it", p.path)
		}
		if p.ships != slices.Contains(ships, p.path) {
			t.Errorf("%s: found shipping %v, want %v", p.path, !p.ships, p.ships)
		}
	}
	for _, line := range renamed {
		if !slices.Contains(copies, "scripts/release.sh: "+line) {
			t.Errorf("release.sh's copy %q isn't found", line)
		}
	}
	if n := len(slices.DeleteFunc(slices.Clone(copies), func(f string) bool { return !strings.HasPrefix(f, "scripts/release.sh: ") })); n != len(renamed) {
		t.Errorf("found %d copies in release.sh, want its %d (and nothing it has of its own):\n  %s", n, len(renamed), strings.Join(copies, "\n  "))
	}
	if !slices.Equal(sources, []string{"scripts/release.sh: scripts/lib/release-checks.bash"}) {
		t.Errorf("release.sh sources a copy of the gate: found %q", sources)
	}
	for _, path := range []string{"scripts/a.out", "docs/notes.md", "desktop/ship.go"} {
		if _, ok := files[path]; ok {
			t.Errorf("%s isn't a script, and was read as one", path)
		}
	}
	// Sourced, in any of its spellings, a file that isn't the gate is found.
	for _, line := range []string{". scripts/lib/x.bash", "source scripts/lib/x", "if true; then . ./x; fi",
		"[ -f x ] && source \"$HOME/x\"", "builtin source x", "( . scripts/make-dmg.sh extra )"} {
		if found := wrongSources(map[string]string{"scripts/make-dmg.sh": line + "\n"}); len(found) != 1 {
			t.Errorf("%q: found %q, want it found", line, found)
		}
	}
}

// releaseProbe is a tree of a few files (path → text; a path ending in "|644" or "|755" gives its mode, else 0o644)
// and what the scan must find in it: the files read, the shipping files and jobs (exactly), and the files a copy of
// the gate is found in.
type releaseProbe struct {
	name   string
	files  map[string]string
	read   []string
	ships  []string
	copies []string
}

// RT126RELGATE1R2's probes, each a row, with their kin: a GitHub release by argv in Python, JavaScript and Go; a Go
// release program under cmd/, and the package of its own it ships from; a workflow job whose steps are an action of the
// repo's own; a clean check by git diff --exit-code and its other spellings; and the narrower ones (fastlane, a
// release action, env -, origin/$MAIN, Python's env={…}, git ls-files --modified, a copy of the gate run by bash with no
// name, #! or mode). Each fails on the scan as RT126RELGATE1R2 reviewed it; the let-be rows show what isn't found.
func TestReleaseGateSeesTheSecondRoundsSpellings(t *testing.T) {
	gate, err := os.ReadFile(filepath.Join("..", releaseGatePath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(gate, []byte("#!/bin/bash\n")) {
		t.Fatal("the gate's first line isn't #!/bin/bash, which the copy run by bash leaves out")
	}
	const module = "module github.com/Sauzze/smolcorp\n\ngo 1.26.0\n"
	const local = "github.com/Sauzze/smolcorp/"
	dispatch := "on: workflow_dispatch\njobs:\n  ship:\n    runs-on: macos-latest\n    steps:\n"
	shipAction := "runs:\n  using: composite\n  steps:\n    - run: xcrun notarytool submit build/Smolcorp.dmg --wait\n      shell: bash\n" +
		"    - run: gh release upload v1 build/Smolcorp.dmg\n      shell: bash\n"
	for _, p := range []releaseProbe{
		// B2.1: a GitHub release by argv.
		{name: "Python, gh release by argv", files: map[string]string{"scripts/ship.py": "import subprocess\n" +
			"subprocess.run([\"gh\", \"release\", \"upload\", tag, dmg_path], check=True)\n"},
			read: []string{"scripts/ship.py"}, ships: []string{"scripts/ship.py"}},
		{name: "JavaScript, gh release by argv", files: map[string]string{"scripts/ship.mjs": "import { execFileSync } from 'node:child_process';\n" +
			"execFileSync('gh', ['release', 'create', tag, dmg]);\n"},
			read: []string{"scripts/ship.mjs"}, ships: []string{"scripts/ship.mjs"}},
		{name: "Go, gh release by argv, split over its words", files: map[string]string{"tools/rel/main.go": "package main\n\n" +
			"func main() { exec.Command(\"gh\", \"release\",\n\t\"edit\", tag).Run() }\n",
			"tools/rel/flag.go": "package main\n\nvar args = []string{\"gh\", \"release\", \"edit\"}\n"},
			read: []string{"tools/rel/flag.go", "tools/rel/main.go"}, ships: []string{"tools/rel/flag.go", "tools/rel/main.go"}},
		{name: "shell, gh release over a continued line", files: map[string]string{"scripts/ship.sh": "gh \\\n  release \\\n  upload v1 x\n"},
			read: []string{"scripts/ship.sh"}, ships: []string{"scripts/ship.sh"}},
		// B2.2: a Go release program anywhere, and the packages only it uses.
		{name: "cmd/ship, notarytool and gh by argv", files: map[string]string{"go.mod": module, "cmd/ship/main.go": "package main\n\n" +
			"import \"os/exec\"\n\nfunc main() {\n\texec.Command(\"xcrun\", \"notarytool\", \"submit\", \"build/Smolcorp.dmg\").Run()\n" +
			"\texec.Command(\"gh\", \"release\", \"upload\", \"v1\", \"build/x.dmg\").Run()\n}\n"},
			read: []string{"cmd/ship/main.go"}, ships: []string{"cmd/ship/main.go"}},
		{name: "cmd/release, gh by argv only", files: map[string]string{"go.mod": module, "cmd/release/main.go": "package main\n\n" +
			"func main() { exec.Command(\"gh\", \"release\", \"create\", tag, file).Run() }\n"},
			read: []string{"cmd/release/main.go"}, ships: []string{"cmd/release/main.go"}},
		{name: "cmd/ship shipping from a package of its own; the app's own packages let be", files: map[string]string{"go.mod": module,
			"cmd/ship/main.go":          "package main\n\nimport (\n\t\"" + local + "internal/ship\"\n\t\"" + local + "internal/shared\"\n)\n\nfunc main() { ship.Publish(); shared.X() }\n",
			"internal/ship/ship.go":     "package ship\n\nimport \"" + local + "internal/ship/upload\"\n\nfunc Publish() { upload.Run() }\n",
			"internal/ship/upload/u.go": "package upload\n\nfunc Run() { exec.Command(\"gh\", \"release\", \"upload\", \"v1\", f).Run() }\n",
			"cmd/smolcorp/main.go":      "package main\n\nimport (\n\t\"" + local + "internal/guard\"\n\t\"" + local + "internal/shared\"\n)\n",
			"internal/guard/rules.go":   "package guard\n\nvar rules = []string{\"gh release\", \"xcrun altool|notarytool\"}\n",
			"internal/shared/s.go":      "package shared\n\nvar s = []string{\"gh\", \"release\", \"create\"}\n"},
			read:  []string{"cmd/ship/main.go", "cmd/smolcorp/main.go", "internal/ship/ship.go", "internal/ship/upload/u.go"},
			ships: []string{"internal/ship/upload/u.go"}},
		// B2.3: a job whose steps are an action of the repo's own, wherever it is, and one it uses in turn.
		{name: "a workflow job using a local action that ships", files: map[string]string{".github/workflows/ship.yml": dispatch +
			"      - uses: actions/checkout@v4\n      - uses: ./.github/actions/ship\n", ".github/actions/ship/action.yml": shipAction},
			read:  []string{".github/actions/ship/action.yml", ".github/workflows/ship.yml"},
			ships: []string{".github/workflows/ship.yml: ship"}},
		{name: "a local action elsewhere, using another that ships", files: map[string]string{".github/workflows/ship.yml": dispatch +
			"      - uses: './ci/outer'\n", "ci/outer/action.yaml": "runs:\n  using: composite\n  steps:\n    - uses: ./ci/inner/\n",
			"ci/inner/action.yml": shipAction},
			read:  []string{".github/workflows/ship.yml", "ci/inner/action.yml", "ci/outer/action.yaml"},
			ships: []string{".github/workflows/ship.yml: ship"}},
		{name: "let be: a local action that runs the gate first", files: map[string]string{".github/workflows/ship.yml": dispatch +
			"      - uses: ./.github/actions/ship\n", ".github/actions/ship/action.yml": strings.Replace(shipAction, "    - run: xcrun",
			"    - run: scripts/release-gate.sh\n      shell: bash\n    - run: xcrun", 1)},
			read: []string{".github/actions/ship/action.yml", ".github/workflows/ship.yml"}},
		// B2.4: a clean check by git diff --exit-code, and its other spellings.
		{name: "a clean check, git diff --exit-code", files: map[string]string{"scripts/release.sh": "git diff --exit-code >/dev/null || { echo dirty; exit 1; }\n"},
			read: []string{"scripts/release.sh"}, copies: []string{"scripts/release.sh"}},
		{name: "a clean check, git diff --name-only", files: map[string]string{"scripts/x.sh": "[ -z \"$(git diff --name-only HEAD)\" ] || exit 1\n"},
			read: []string{"scripts/x.sh"}, copies: []string{"scripts/x.sh"}},
		{name: "a clean check, git -C . status -uno", files: map[string]string{"scripts/x.sh": "[ -z \"$(git -C . status -uno)\" ]\n"},
			read: []string{"scripts/x.sh"}, copies: []string{"scripts/x.sh"}},
		{name: "a clean check, git diff-index", files: map[string]string{"scripts/x.py": "subprocess.check_call(['git', 'diff-index', 'HEAD'])\n"},
			read: []string{"scripts/x.py"}, copies: []string{"scripts/x.py"}},
		{name: "a clean check, git ls-files --modified --others", files: map[string]string{"scripts/x.sh": "test -z \"$(git ls-files --modified --others --exclude-standard)\"\n"},
			read: []string{"scripts/x.sh"}, copies: []string{"scripts/x.sh"}},
		// The narrower ones.
		{name: "fastlane's notarize and set_github_release", files: map[string]string{"fastlane/Fastfile": "lane :release do\n" +
			"  notarize(package: \"build/Smolcorp.app\")\n  set_github_release(repository_name: \"Sauzze/smolcorp\", tag_name: \"v1\")\nend\n"},
			read: []string{"fastlane/Fastfile"}, ships: []string{"fastlane/Fastfile"}},
		{name: "ncipollo/release-action on workflow_dispatch", files: map[string]string{".github/workflows/r.yml": dispatch +
			"      - uses: ncipollo/release-action@v1\n        with:\n          artifacts: build/*.dmg\n"},
			read: []string{".github/workflows/r.yml"}, ships: []string{".github/workflows/r.yml: ship"}},
		{name: "an empty environment, env -", files: map[string]string{"scripts/x.sh": "env - PATH=/usr/bin:/bin go build ./cmd/smolcorp\n"},
			read: []string{"scripts/x.sh"}, copies: []string{"scripts/x.sh"}},
		{name: "an empty environment, Python's env={…}", files: map[string]string{"scripts/x.py": "subprocess.run(['go', 'build'], env={\"PATH\": \"/usr/bin\"})\n"},
			read: []string{"scripts/x.py"}, copies: []string{"scripts/x.py"}},
		{name: "an empty environment, JavaScript's env: {…}", files: map[string]string{"scripts/x.mjs": "execFileSync('go', ['build'], { env: { PATH: '/usr/bin' } });\n"},
			read: []string{"scripts/x.mjs"}, copies: []string{"scripts/x.mjs"}},
		{name: "let be: JavaScript's env with the caller's spread in", files: map[string]string{"scripts/x.mjs": "spawn(cmd, { env: { ...process.env, TERM: 'dumb' } });\n"},
			read: []string{"scripts/x.mjs"}},
		{name: "origin's main by a variable", files: map[string]string{"scripts/x.sh": "[ \"$(git rev-parse HEAD^)\" = \"$(git rev-parse \"origin/$MAIN\")\" ]\n"},
			read: []string{"scripts/x.sh"}, copies: []string{"scripts/x.sh"}},
		{name: "a copy of the gate run by bash, with no extension, #! or mode", files: map[string]string{
			"scripts/release.sh":         "#!/bin/bash\n. scripts/release-gate.sh\nbash scripts/lib/release-common\n",
			"scripts/lib/release-common": strings.TrimPrefix(string(gate), "#!/bin/bash\n")},
			read: []string{"scripts/lib/release-common", "scripts/release.sh"}, copies: []string{"scripts/lib/release-common"}},
		{name: "a copy of the gate fed to sh, and one to eval, by the scripts they run", files: map[string]string{
			"scripts/release-desktop.sh": "#!/bin/bash\n\"$ROOT/scripts/lib/step\"\nsh -s < ./lib/one\n",
			"scripts/lib/step|755":       "#!/bin/sh\neval \"$(cat scripts/lib/two)\"\n",
			"scripts/lib/one":            "git status --porcelain\n", "scripts/lib/two": "git write-tree\n"},
			read:   []string{"scripts/lib/one", "scripts/lib/step", "scripts/lib/two", "scripts/release-desktop.sh"},
			copies: []string{"scripts/lib/one", "scripts/lib/two"}},
		// RT126RELGATE1R3's follow-ups: a clean check by git diff's output, the action at the repo's root, and Go's
		// words of a release gathered in a slice first.
		{name: "a clean check, $(git diff HEAD)", files: map[string]string{"scripts/release.sh": "[ -z \"$(git diff HEAD)\" ] || exit 1\n"},
			read: []string{"scripts/release.sh"}, copies: []string{"scripts/release.sh"}},
		{name: "a clean check, `git diff` and git -C dir diff --cached | wc -l", files: map[string]string{
			"scripts/x.sh": "test -z \"`git diff`\"\n", "scripts/y.sh": "[ \"$(git -C \"$ROOT\" diff --cached | wc -l)\" -eq 0 ]\n"},
			read: []string{"scripts/x.sh", "scripts/y.sh"}, copies: []string{"scripts/x.sh", "scripts/y.sh"}},
		{name: "a clean check, git diff's output in Python and JavaScript", files: map[string]string{
			"scripts/x.py":  "if subprocess.check_output([\"git\", \"diff\", \"HEAD\"]):\n    sys.exit(1)\n",
			"scripts/x.mjs": "if (execFileSync('git', ['diff']).length) process.exit(1);\n"},
			read: []string{"scripts/x.mjs", "scripts/x.py"}, copies: []string{"scripts/x.mjs", "scripts/x.py"}},
		// RT126RELGATE1R4's follow-up: Python's run with its output captured, and os.popen.
		{name: "a clean check, git diff's output by Python's run(capture_output=True), Popen(stdout=) and os.popen", files: map[string]string{
			"scripts/x.py": "if subprocess.run([\"git\", \"diff\", \"HEAD\"], capture_output=True).stdout:\n    sys.exit(1)\n",
			"scripts/y.py": "if os.popen(\"git diff\").read():\n    sys.exit(1)\n",
			"scripts/z.py": "out = subprocess.Popen(\n    [\"git\", \"-C\", root, \"diff\", \"--cached\"],\n    stdout=subprocess.PIPE,\n).communicate()[0]\n"},
			read: []string{"scripts/x.py", "scripts/y.py", "scripts/z.py"}, copies: []string{"scripts/x.py", "scripts/y.py", "scripts/z.py"}},
		{name: "let be: Python's run of git diff, shown, not captured", files: map[string]string{
			"scripts/x.py": "subprocess.run([\"git\", \"diff\", \"HEAD~1\"], check=False)\n"},
			read: []string{"scripts/x.py"}},
		{name: "let be: git diff shown, and $(git describe)", files: map[string]string{
			"scripts/x.sh": "git diff HEAD~1 || true\nTAG=\"$(git describe --tags)\"\n"},
			read: []string{"scripts/x.sh"}},
		{name: "a workflow job using the action at the repo's root, uses: ./", files: map[string]string{
			".github/workflows/ship.yml": dispatch + "      - uses: actions/checkout@v4\n      - uses: ./\n", "action.yml": shipAction},
			read: []string{".github/workflows/ship.yml", "action.yml"}, ships: []string{".github/workflows/ship.yml: ship"}},
		{name: "let be: the action at the repo's root, running the gate first", files: map[string]string{
			".github/workflows/ship.yml": dispatch + "      - uses: './'\n", "action.yaml": strings.Replace(shipAction, "    - run: xcrun",
				"    - run: scripts/release-gate.sh\n      shell: bash\n    - run: xcrun", 1)},
			read: []string{".github/workflows/ship.yml", "action.yaml"}},
		// A workflow's line releaseLetBe names (UPGRADE1's download of 1.24) is let be on its own, and nothing else of
		// its job is: the same job sending a release still ships.
		{name: "let be: the upgrade job's download of 1.24, by its named line", files: map[string]string{
			".github/workflows/ci.yml": "on: push\njobs:\n  upgrade:\n    runs-on: ubuntu-latest\n    steps:\n      - run: |\n" +
				"          case \"$RUNNER_OS\" in\n            Linux) asset=Smolcorp-Linux-x86_64.tar.gz; program=smolcorp ;;\n          esac\n"},
			read: []string{".github/workflows/ci.yml"}},
		{name: "the upgrade job's named line, its job also sending a release", files: map[string]string{
			".github/workflows/ci.yml": "on: push\njobs:\n  upgrade:\n    runs-on: ubuntu-latest\n    steps:\n      - run: |\n" +
				"          case \"$RUNNER_OS\" in\n            Linux) asset=Smolcorp-Linux-x86_64.tar.gz; program=smolcorp ;;\n          esac\n" +
				"          gh release upload v1 \"$asset\"\n"},
			read: []string{".github/workflows/ci.yml"}, ships: []string{".github/workflows/ci.yml: upgrade"}},
		{name: "Go, gh release by an args slice", files: map[string]string{"go.mod": module, "cmd/ship/main.go": "package main\n\n" +
			"import \"os/exec\"\n\nfunc main() {\n\targs := []string{\"release\", \"upload\", tag, dmg}\n\texec.Command(\"gh\", args...).Run()\n}\n"},
			read: []string{"cmd/ship/main.go"}, ships: []string{"cmd/ship/main.go"}},
		{name: "Go, gh by its path, its words in the file's variables and appended", files: map[string]string{"go.mod": module,
			"tools/rel/main.go": "package main\n\nimport \"os/exec\"\n\nconst gh = \"/opt/homebrew/bin/gh\"\n\nvar verb = []string{\"release\"}\n\n" +
				"func main() {\n\targs := append(verb, `create`, tag)\n\texec.Command(gh, args...).Run()\n}\n"},
			read: []string{"tools/rel/main.go"}, ships: []string{"tools/rel/main.go"}},
		{name: "let be: Go running gh for a pull request, release in a sentence", files: map[string]string{"go.mod": module,
			"cmd/pr/main.go": "package main\n\nimport \"os/exec\"\n\nfunc main() {\n\texec.Command(\"gh\", \"pr\", \"create\", \"--title\", \"release notes\").Run()\n}\n"},
			read: []string{"cmd/pr/main.go"}},
		{name: "let be: a note a release script names, not run", files: map[string]string{
			"scripts/release.sh": "#!/bin/bash\necho \"See docs/NOTES\"\ncp docs/NOTES build/\n", "docs/NOTES": "gh release upload v1 build/Smolcorp.dmg\n"},
			read: []string{"scripts/release.sh"}},
	} {
		t.Run(p.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, text := range p.files {
				mode := os.FileMode(0o644)
				if path, m, ok := strings.Cut(name, "|"); ok {
					name = path
					if m == "755" {
						mode = 0o755
					}
				}
				writeReleaseFile(t, filepath.Join(dir, name), text, mode)
			}
			files := releaseFilesIn(t, dir)
			read := slices.Sorted(maps.Keys(files))
			ships := shippingFiles(files)
			var copies []string
			for _, found := range gateCopies(files) {
				path, _, _ := strings.Cut(found, ": ")
				copies = append(copies, path)
			}
			copies = slices.Compact(copies)
			if !slices.Equal(read, p.read) {
				t.Errorf("read %q, want %q", read, p.read)
			}
			if !slices.Equal(ships, p.ships) {
				t.Errorf("found shipping %q, want %q", ships, p.ships)
			}
			if !slices.Equal(copies, p.copies) {
				t.Errorf("found copies of the gate in %q, want %q", copies, p.copies)
			}
		})
	}
}

// make-dmg.sh runs Python and pip isolated (RT126RELGATE1R2's follow-up): every python it runs has -I, pip has
// --isolated and PIP_CONFIG_FILE=/dev/null (RT126RELGATE1R3's follow-up: --isolated still reads the file a caller's
// PIP_CONFIG_FILE names, and /dev/null there makes pip read no config file at all), dmgbuild runs through the venv's
// python -I, and a venv not made that way isn't reused.
func TestReleaseGateMakesTheDMGWithPythonIsolated(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "scripts", "make-dmg.sh"))
	if err != nil {
		t.Fatal(err)
	}
	runs := 0
	for _, line := range codeLines(string(script)) {
		for _, m := range regexp.MustCompile(`(?:^|[;&|(!]|\bif|\bthen)\s*(?:!\s*)?((?:\w+=\S*\s+)*)("\$VENV/bin/[a-z0-9]+"|\$VENV/bin/[a-z0-9]+\b|python3?\b|pip3?\b)(\s+\S+)?(.*)`).FindAllStringSubmatch(line, -1) {
			runs++
			if !strings.HasSuffix(strings.Trim(m[2], `"`), "python") && m[2] != "python3" || strings.TrimSpace(m[3]) != "-I" {
				t.Errorf("make-dmg.sh runs %s without python -I: %s", m[2], strings.TrimSpace(line))
			}
			if strings.Contains(m[4], "pip install") && !strings.Contains(m[4], "pip install --isolated ") {
				t.Errorf("make-dmg.sh runs pip install without --isolated: %s", strings.TrimSpace(line))
			}
			if strings.Contains(m[4], "pip install") && !slices.Contains(strings.Fields(m[1]), "PIP_CONFIG_FILE=/dev/null") {
				t.Errorf("make-dmg.sh runs pip install without PIP_CONFIG_FILE=/dev/null: %s", strings.TrimSpace(line))
			}
		}
	}
	text := strings.Join(codeLines(string(script)), "\n")
	if runs != 4 || !strings.Contains(text, `if [ ! -f "$VENV/made-isolated" ] || ! "$VENV/bin/python" -I -c "import dmgbuild"`) ||
		!strings.Contains(text, `"dmgbuild==$DMGBUILD_VERSION"`+"\n  touch \"$VENV/made-isolated\"\nfi") ||
		!strings.Contains(text, `"$VENV/bin/python" -I "$VENV/bin/dmgbuild" -s `) {
		t.Errorf("make-dmg.sh: want 4 python runs, all -I, a venv reused only if made isolated, and dmgbuild run by python -I; found %d runs", runs)
	}
}

// The gate's list: RELEASE_GATES names each gate, in the order its top describes them, and it says the list when it
// passes.
func TestReleaseGateListsItsGates(t *testing.T) {
	gate, err := os.ReadFile(filepath.Join("..", releaseGatePath))
	if err != nil {
		t.Fatal(err)
	}
	list := regexp.MustCompile(`(?m)^RELEASE_GATES=\(([^)]*)\)$`).FindAllStringSubmatch(string(gate), -1)
	if len(list) != 1 {
		t.Fatalf("the gate sets RELEASE_GATES=(…) %d times, want once", len(list))
	}
	top, _, _ := strings.Cut(string(gate), "\nset -euo pipefail\n")
	var described []string
	for _, m := range regexp.MustCompile(`(?m)^#   [0-9]+\. ([a-z-]+):`).FindAllStringSubmatch(top, -1) {
		described = append(described, m[1])
	}
	if named := strings.Fields(list[0][1]); len(named) < 6 || !slices.Equal(named, described) {
		t.Errorf("RELEASE_GATES is %v, the gate's top describes %v", named, described)
	}
	if !strings.Contains(string(gate), "\necho \"The release gate passed for HEAD's tree ($TREE): ${RELEASE_GATES[*]}.\"\n") {
		t.Error("the gate doesn't say its list when it passes")
	}
}

// releaseDesktopAsRun is release-desktop.sh as it runs: the gate it sources, in the place of that line.
func releaseDesktopAsRun(t *testing.T) string {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("..", "scripts", "release-desktop.sh"))
	if err != nil {
		t.Fatal(err)
	}
	gate, err := os.ReadFile(filepath.Join("..", releaseGatePath))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(script), "\n"+releaseGateLine+"\n"); n != 1 {
		t.Fatalf("release-desktop.sh sources the gate %d times, want once", n)
	}
	return strings.Replace(string(script), "\n"+releaseGateLine+"\n", "\n"+string(gate), 1)
}
