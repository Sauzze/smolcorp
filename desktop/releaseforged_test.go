//go:build unix

package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// scripts/release-desktop.sh stops before it builds anything unless the forged-event table is whole and passes
// (scripts/forgedevents, SA-X11), run by the go it named at its start with that go's settings pinned. Each case runs
// the script in a throwaway repo holding the real gate and a stand-in desktop package with the table's tests. A
// stand-in go comes first on PATH: the office's check passes, the Linux tmux's check (the next step) says it was
// reached and stops the script there, and anything else (go version, go env, go run of the gate, go list, go test) goes
// to the real go. With TO_BUILDS set, the Linux tmux's check passes too, mkwinres passes and the first build stops the
// script. Every call notes its arguments and the whole environment it was given (forgedCalls), which must be the list
// of go's settings the log starts with, and nothing else (RT126SAX11R3 B4). The bundled tmux's checks (TMUXGATE1),
// before the gate, go to the real go too, with the stand-in tmux test every release repo holds; the script's run of
// its own tests is answered as every heavy test passing (heavyRowsPassing). Each case builds cold in the run's own
// cache, so these rows are heavy (heavyReleaseRows): the script runs them before every release, and go test skips them
// elsewhere; TestReleaseDesktopGivesEveryGoOnlyItsOwnList checks the script's side of them in every run.

const (
	forgedPassed   = "The forged-event table is in every system's build, and passed here."
	officeRan      = "stand-in office check ran"
	stopForged     = "Stopped: the forged-event table didn't pass here"
	stopForgedHole = "Stopped: the forged-event table isn't whole"
	stopForgedTree = "the files changed while the forged-event table ran"
)

// forgedTable is scripts/forgedevents' table: the tests on every system, then the ones on unix only.
var forgedTable = [2][]string{
	{"TestADeskCantSendAnEventAsAnotherDesk", "TestALaunchersDeskReportsWithItsOwnKeyOnly",
		"TestALauncherFoundByFolderIsCheckedAsItsDesks", "TestADesksDroppedSessionComesBackForItsOwnKeyOnly",
		"TestADeskCantSendAnEventAsAnotherDeskThroughItsHook"},
	{"TestAManagedDeskReportsWithTheHookKeyItsTicketHolds", "TestTheAppsReportShSendsItsDesksKeyAndNoOthers"},
}

// forgedGoSettings is the line of the release log after which it lists go's settings, one per line.
const forgedGoSettings = "go's settings, the only ones it gets (plus GOOS, GOARCH and CGO_ENABLED where a step sets them):"

// forgedPerCall are the settings a step of the release sets itself, beside the list: the bundled tmux's test's control
// run (TMUXGATE1) and the script's run of its own tests (which also puts the pinned go's folder first on PATH) set
// theirs.
var forgedPerCall = []string{"CGO_ENABLED", "GOARCH", "GOOS", releaseGateRows, "SMOLCORP_TMUXGATE_CONTROL"}

// forgedMark is in the name of each folder of stand-in files the caller names (libraries, headers) and in the files:
// the release's output holds it only if the build read one of them.
const forgedMark = "named-by-the-caller"

// forgedCallerFolder writes files (name → text) in a fresh folder whose name holds forgedMark, and gives the folder.
func forgedCallerFolder(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), forgedMark)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range files {
		writeReleaseFile(t, filepath.Join(dir, name), text, 0o644)
	}
	return dir
}

// forgedSettings reads the list of go's settings from the release log, name → value.
func forgedSettings(out string) map[string]string {
	settings := map[string]string{}
	_, list, ok := strings.Cut(out, forgedGoSettings+"\n")
	if !ok {
		return settings
	}
	for _, line := range strings.Split(list, "\n") {
		kv, ok := strings.CutPrefix(line, "  ")
		name, value, isSetting := strings.Cut(kv, "=")
		if !ok || !isSetting {
			break
		}
		settings[name] = value
	}
	return settings
}

// forgedCall is one run of the stand-in go: its arguments, and each setting it got, name → value.
type forgedCall struct {
	args string
	env  map[string]string
}

// forgedCalls are the stand-in go's runs in r, in order.
func forgedCalls(t *testing.T, r *releaseRepo) []forgedCall {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.SplitList(pathOf(r))[0], "calls"))
	if err != nil {
		t.Fatal(err)
	}
	var calls []forgedCall
	for _, line := range strings.Split(string(data), "\n") {
		if args, ok := strings.CutPrefix(line, "call: "); ok {
			calls = append(calls, forgedCall{args, map[string]string{}})
		} else if kv, ok := strings.CutPrefix(line, "  "); ok && len(calls) > 0 {
			name, value, _ := strings.Cut(kv, "=")
			calls[len(calls)-1].env[name] = value
		}
	}
	return calls
}

// forgedCgo is a file of the stand-in desktop package that uses cgo, as the real one does on a Mac: its test binary is
// built with the C compiler and linked by it.
var forgedCgo = map[string]string{
	"desktop/forged_cgo.go": "package desktop\n\n// static int two(void) { return 2; }\nimport \"C\"\n\nfunc two() int { return int(C.two()) }\n",
}

// forgedRepo is a throwaway release repo whose release commit holds the real gate and the stand-in table, each test's
// body as bodies gives it ("" passes), and go.mod with toolchain as its toolchain line when there is one. target is
// the go the stand-in hands what it doesn't answer to, and extra any file to add, name → text.
func forgedRepo(t *testing.T, target, toolchain string, bodies map[string]string, extra map[string]string) *releaseRepo {
	t.Helper()
	r := newReleaseRepo(t)
	// go builds the gate and the table's tests with the run's own build cache, made fresh, so cold.
	r.wait = 3 * time.Minute
	gate, err := os.ReadFile(filepath.Join("..", "scripts", "forgedevents", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	mod := "module example.invalid/release\n\ngo 1.21\n"
	if toolchain != "" {
		mod += "\ntoolchain " + toolchain + "\n"
	}
	r.write("go.mod", mod)
	r.write("scripts/forgedevents/main.go", string(gate))
	r.write("desktop/forged.go", "package desktop\n")
	for i, file := range []string{"desktop/forged_test.go", "desktop/forged_unix_test.go"} {
		text := "package desktop\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nvar _ = os.Getpid\n"
		if i == 1 {
			text = "//go:build !windows\n\n" + text
		}
		for _, name := range forgedTable[i] {
			text += "\nfunc " + name + "(t *testing.T) {\n\t" + bodies[name] + "\n}\n"
		}
		r.write(file, text)
	}
	for name, text := range extra {
		r.write(name, text)
	}
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "the table")
	r.squashed()

	// The stand-in go, first on PATH. It takes DROP, DROPCC and TO_BUILDS from standin.conf beside it (standinSettings),
	// and notes each run, its arguments and its whole environment but the shell's own,
	// in calls beside it. It answers the script's run of its own tests as every heavy test passing. With DROP set, its first run puts another go and another env in that folder, which is first
	// on PATH: something doing so while the script runs. With DROPCC set, its go version (the script's first go call
	// after it found its C compiler) puts a cc, a clang and a gcc there in the same way.
	bin := filepath.SplitList(pathOf(r))[0]
	writeReleaseFile(t, filepath.Join(bin, "go"), "#!/bin/sh\n"+
		"here=\"${0%/*}\"\n[ ! -f \"$here/standin.conf\" ] || . \"$here/standin.conf\"\n"+
		"{ echo \"call: $*\"; env | grep -v -E '^(PWD|SHLVL|_|OLDPWD)=' | sed 's/^/  /'; } >> \"$here/calls\"\n"+
		"if [ -n \"${DROP:-}\" ] && [ ! -e \"$DROP/go\" ]; then\n"+
		"  for name in go env; do\n"+
		"    printf '#!/bin/sh\\necho \"$0 $*\" >> %s\\necho another $0 ran\\nexit 97\\n' \"$DROP/ran\" > \"$DROP/$name\"\n"+
		"    chmod 755 \"$DROP/$name\"\n  done\nfi\n"+
		"if [ -n \"${DROPCC:-}\" ] && [ \"$1\" = version ] && [ ! -e \"$DROPCC/cc\" ]; then\n"+
		"  for name in cc clang gcc; do\n"+
		"    printf '#!/bin/sh\\necho \"$0 $*\" >> %s\\nexit 1\\n' \"$DROPCC/ran\" > \"$DROPCC/$name\"\n"+
		"    chmod 755 \"$DROPCC/$name\"\n  done\nfi\n"+
		"if [ \"$1\" = test ] && [ \"${"+releaseGateRows+":-}\" = 1 ]; then\n"+heavyRowsPassing(t)+"fi\n"+
		"case \"$1 $2\" in\n"+
		"'run ./scripts/buildoffice') echo '"+officeRan+"'; exit 0 ;;\n"+
		"'run ./scripts/linuxtmux') [ -z \"${TO_BUILDS:-}\" ] || exit 0; echo '"+goReached+"'; exit 97 ;;\n"+
		"'run ./scripts/mkwinres') exit 0 ;;\n"+
		"'build -trimpath') echo '"+goReached+"'; exit 97 ;;\n"+
		"esac\nexec '"+target+"' \"$@\"\n", 0o755)
	return r
}

// forgedStandin writes a program that notes its run in ran and fails, as name in a folder of its own, and gives its
// path: a C compiler or linker the caller names, which must never build or link the tests.
func forgedStandin(t *testing.T, ran, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	writeReleaseFile(t, path, "#!/bin/sh\necho \"$0 $*\" >> '"+ran+"'\nexit 1\n", 0o755)
	return path
}

// releaseCC is the C compiler, by its real path, and on a Mac the SDK, that release-desktop.sh finds at its start in
// r: on a Mac xcrun's clang and SDK, elsewhere the cc first on r's PATH.
func releaseCC(t *testing.T, r *releaseRepo) (string, string) {
	t.Helper()
	var cc, sdk string
	if runtime.GOOS == "darwin" {
		xcrun := func(args ...string) string {
			cmd := exec.Command("/usr/bin/xcrun", append([]string{"--no-cache"}, args...)...)
			cmd.Env = r.env
			out, err := cmd.Output()
			if err != nil {
				t.Skipf("no xcrun %v: %v", args, err)
			}
			path, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
			if err != nil {
				t.Fatal(err)
			}
			return path
		}
		cc, sdk = xcrun("--find", "clang"), xcrun("--show-sdk-path")
	} else {
		for _, dir := range filepath.SplitList(pathOf(r)) {
			if info, err := os.Stat(filepath.Join(dir, "cc")); err == nil && !info.IsDir() {
				cc = filepath.Join(dir, "cc")
				break
			}
		}
		if cc == "" {
			t.Skip("no cc")
		}
		var err error
		if cc, err = filepath.EvalSymlinks(cc); err != nil {
			t.Fatal(err)
		}
	}
	return cc, sdk
}

// forgedFakePass writes a program that says each test of the table and the gate passed, running nothing, as go's
// -exec or a toolchain put in go's place would run it, and gives its path. Each run is noted in ran.
func forgedFakePass(t *testing.T, ran string) string {
	t.Helper()
	text := "#!/bin/sh\necho \"$0 $*\" >> '" + ran + "'\n"
	for _, names := range forgedTable {
		for _, name := range names {
			text += "echo '--- PASS: " + name + " (0.00s)'\n"
		}
	}
	text += "echo PASS\necho '" + forgedPassed + "'\n"
	path := filepath.Join(t.TempDir(), "fake-pass")
	writeReleaseFile(t, path, text, 0o755)
	return path
}

// forgedGoroot is a copy of the real go's toolchain, its go program copied and the rest linked, whose own go.env is
// goEnv. It gives the copy's folder and its go.
func forgedGoroot(t *testing.T, real, goEnv string) (string, string) {
	t.Helper()
	cmd := exec.Command(real, "env", "GOROOT")
	cmd.Env = append(os.Environ(), "GOROOT=")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	from := strings.TrimSpace(string(out))
	root := t.TempDir()
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if name := entry.Name(); name != "bin" && name != "go.env" {
			if err := os.Symlink(filepath.Join(from, name), filepath.Join(root, name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	program := filepath.Join(root, "bin", "go")
	src, err := os.Open(filepath.Join(from, "bin", "go"))
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	writeReleaseFile(t, program, "", 0o755)
	dst, err := os.OpenFile(program, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}
	writeReleaseFile(t, filepath.Join(root, "go.env"), goEnv, 0o644)
	return root, program
}

func TestReleaseDesktopStopsUnlessTheForgedEventTableIsWhole(t *testing.T) {
	heavyReleaseRows(t)
	real, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go")
	}
	const failing = `t.Fatal("stand-in: a forged event let in")`
	failOne := map[string]string{forgedTable[0][4]: failing}
	other := "linux"
	if runtime.GOOS == "linux" {
		other = "darwin"
	}

	type forgedCase struct {
		name string
		// bodies are the table tests' bodies, toolchain go.mod's toolchain line; target, when set, gives the go the
		// stand-in hands on to (the real one otherwise); env gives the caller's settings. Any stand-in they set up
		// notes its runs in ran.
		bodies    map[string]string
		toolchain string
		extra     map[string]string
		target    func(t *testing.T, ran string) string
		env       func(t *testing.T, r *releaseRepo, ran string) []string
		// want is the stop, or "" when the release must go on to the next step; then, when set, checks a run that
		// goes on to the builds (TO_BUILDS) instead.
		want string
		then func(t *testing.T, r *releaseRepo, status int, out string)
		// script, when set, is run in place of release-desktop.sh.
		script string
	}
	for _, c := range []forgedCase{
		{name: "the table whole", want: ""},
		// The gate run on its own (RELGATE1), as release.sh, make-dmg.sh, build-app.sh and publish-release.sh run it:
		// it passes, says its list, and removes go's folders for the run before it says so, as a script that sourced
		// it and doesn't build with them may set an exit trap of its own after it.
		{name: "the gate on its own, the table whole", script: releaseGatePath,
			then: func(t *testing.T, r *releaseRepo, status int, out string) {
				removed, passed := strings.Index(out, " KB, removed.\n"), strings.Index(out, "\nThe release gate passed for HEAD's tree (")
				if status != 0 || !strings.Contains(out, forgedPassed) || removed < 0 || passed < removed ||
					!strings.Contains(out, "): pins one-commit heads-files tmux-test own-tests forged-events.\n") ||
					strings.Contains(out, officeRan) || strings.Contains(out, goReached) || strings.Contains(out, "Stopped: ") {
					t.Errorf("want the gate passed, go's folders removed first, and nothing after it, got status %d:\n%s", status, out)
				}
			}},
		{name: "a test of the table missing", extra: map[string]string{
			"desktop/forged_test.go": "package desktop\n\nimport \"testing\"\n\nfunc TestSomethingElse(t *testing.T) {}\n"},
			want: stopForgedHole},
		{name: "a test of the table failing", bodies: failOne, want: stopForged},
		{name: "failing, the caller's GOFLAGS running a stand-in that says it passed", bodies: failOne,
			env: func(t *testing.T, r *releaseRepo, ran string) []string {
				return []string{"GOFLAGS=-exec=" + forgedFakePass(t, ran)}
			}, want: stopForged},
		{name: "failing, a go env file's GOFLAGS running a stand-in that says it passed", bodies: failOne,
			env: func(t *testing.T, r *releaseRepo, ran string) []string {
				env := filepath.Join(r.home, "go-env")
				writeReleaseFile(t, env, "GOFLAGS=-exec="+forgedFakePass(t, ran)+"\n", 0o644)
				return []string{"GOENV=" + env}
			}, want: stopForged},
		{name: "failing, go.mod's toolchain line and the caller's GOTOOLCHAIN switching to a stand-in toolchain on PATH",
			bodies: failOne, toolchain: "go1.99.0",
			env: func(t *testing.T, r *releaseRepo, ran string) []string {
				bin := t.TempDir()
				if err := os.Rename(forgedFakePass(t, ran), filepath.Join(bin, "go1.99.0")); err != nil {
					t.Fatal(err)
				}
				return []string{"GOTOOLCHAIN=path", "PATH=" + strings.Replace(pathOf(r), ":", ":"+bin+":", 1)}
			}, want: stopForged},
		{name: "whole, go.mod's toolchain line asking for a toolchain to download: none downloaded, none switched to",
			toolchain: "go1.99.0", env: func(t *testing.T, r *releaseRepo, ran string) []string {
				return []string{"GOTOOLCHAIN=auto"}
			}, want: ""},
		{name: "failing, the pinned go's own go.env (its toolchain's) running a stand-in that says it passed", bodies: failOne,
			target: func(t *testing.T, ran string) string {
				_, program := forgedGoroot(t, real, "GOFLAGS=-exec="+forgedFakePass(t, ran)+"\n")
				return program
			}, want: stopForged},
		{name: "failing, the caller's GOROOT naming a toolchain whose go.env runs a stand-in that says it passed", bodies: failOne,
			env: func(t *testing.T, r *releaseRepo, ran string) []string {
				root, _ := forgedGoroot(t, real, "GOFLAGS=-exec="+forgedFakePass(t, ran)+"\n")
				return []string{"GOROOT=" + root}
			}, want: stopForged},
		{name: "failing, the caller's GOOS building for another system, run by a stand-in go_<system>_<arch>_exec on PATH",
			bodies: failOne, env: func(t *testing.T, r *releaseRepo, ran string) []string {
				bin := t.TempDir()
				if err := os.Rename(forgedFakePass(t, ran), filepath.Join(bin, "go_"+other+"_"+runtime.GOARCH+"_exec")); err != nil {
					t.Fatal(err)
				}
				return []string{"GOOS=" + other, "CGO_ENABLED=0", "PATH=" + strings.Replace(pathOf(r), ":", ":"+bin+":", 1)}
			}, want: stopForged},
		{name: "whole, another go and env put first on PATH after the start", env: func(t *testing.T, r *releaseRepo, ran string) []string {
			drop := filepath.Join(r.home, "early")
			if err := os.Mkdir(drop, 0o755); err != nil {
				t.Fatal(err)
			}
			return []string{"DROP=" + drop, "PATH=" + drop + ":" + pathOf(r)}
		}, want: ""},
		// The C compiler links the test binary whose results the gate reads (RT126SAX11R2 B3).
		{name: "failing, the caller's CC building and linking the test binary", bodies: failOne, extra: forgedCgo,
			env: func(t *testing.T, r *releaseRepo, ran string) []string {
				return []string{"CC=" + forgedStandin(t, ran, "cc")}
			}, want: stopForged},
		{name: "failing, the caller's CGO_LDFLAGS naming a stand-in linker with -fuse-ld", bodies: failOne, extra: forgedCgo,
			env: func(t *testing.T, r *releaseRepo, ran string) []string {
				return []string{"CGO_LDFLAGS=-fuse-ld=" + forgedStandin(t, ran, "ld")}
			}, want: stopForged},
		{name: "failing, a cc, clang and gcc put first on PATH after the start", bodies: failOne, extra: forgedCgo,
			env: func(t *testing.T, r *releaseRepo, ran string) []string {
				drop := filepath.Join(r.home, "early-cc")
				if err := os.Mkdir(drop, 0o755); err != nil {
					t.Fatal(err)
				}
				return []string{"DROPCC=" + drop, "PATH=" + drop + ":" + pathOf(r)}
			}, want: stopForged},
		// Every go and C setting the release pins, set otherwise by the caller: the gate still runs a whole table
		// with cgo, and the office's and the tmux's checks, mkwinres and the builds themselves get the pinned ones.
		{name: "whole, the caller's go and C settings: every step after the gate, the builds too, gets the pinned ones",
			extra: map[string]string{"desktop/forged_cgo.go": forgedCgo["desktop/forged_cgo.go"],
				"Resources/Info.plist": "<key>CFBundleShortVersionString</key>\n<string>0.0.0</string>\n" +
					"<key>CFBundleVersion</key>\n<string>1</string>\n"},
			env: func(t *testing.T, r *releaseRepo, ran string) []string {
				return []string{"TO_BUILDS=1", "GOFLAGS=-tags=standin", "GOENV=" + filepath.Join(r.home, "go-env"),
					"GOWORK=" + filepath.Join(r.home, "go.work"), "GOTOOLCHAIN=path", "GOROOT=" + t.TempDir(),
					"CC=" + forgedStandin(t, ran, "cc"), "CXX=" + forgedStandin(t, ran, "c++"),
					"CGO_CFLAGS=-DSTANDIN", "CGO_CPPFLAGS=-DSTANDIN", "CGO_CXXFLAGS=-DSTANDIN", "CGO_FFLAGS=-DSTANDIN",
					"CGO_LDFLAGS=-fuse-ld=" + forgedStandin(t, ran, "ld"), "PKG_CONFIG=" + forgedStandin(t, ran, "pkg-config"),
					"DEVELOPER_DIR=" + t.TempDir(), "SDKROOT=" + t.TempDir(), "TOOLCHAINS=example.invalid.none",
					"COMPILER_PATH=" + filepath.Dir(forgedStandin(t, ran, "ld")), "GCC_EXEC_PREFIX=" + t.TempDir() + "/",
					"CCC_OVERRIDE_OPTIONS=# +-fuse-ld=" + forgedStandin(t, ran, "ld")}
			}, then: func(t *testing.T, r *releaseRepo, status int, out string) {
				// Each step got the list of go's settings (checked for every case below) and, of its own, this
				// machine's system for a check, the one asked for a build.
				other := map[string]string{"darwin": "CGO_ENABLED=0", "linux": "CGO_ENABLED=0", "windows": "CGO_ENABLED=0"}
				other[runtime.GOOS] = ""
				here := "GOARCH=" + runtime.GOARCH + " GOOS=" + runtime.GOOS
				// The bundled tmux's checks: its test in go's list for this Mac's arch and each Linux one, with cgo
				// only for this machine's own, then run twice here, the second time as its control run.
				tmuxList := func(system, arch string) string {
					if system == runtime.GOOS && arch == runtime.GOARCH {
						return "GOARCH=" + arch + " GOOS=" + system
					}
					return "CGO_ENABLED=0 GOARCH=" + arch + " GOOS=" + system
				}
				want := map[string][]string{
					"run ./scripts/buildoffice": {here}, "run ./scripts/forgedevents": {here},
					"run ./scripts/linuxtmux": {here, here}, "run ./scripts/mkwinres": {here},
					"test -count=1 -v -run ^(": {here}, "build -trimpath": {"CGO_ENABLED=0 GOARCH=amd64 GOOS=windows"},
					"list -f": {strings.TrimSpace(other["darwin"] + " GOARCH=" + runtime.GOARCH + " GOOS=darwin"),
						strings.TrimSpace(other["linux"] + " GOARCH=" + runtime.GOARCH + " GOOS=linux"),
						"CGO_ENABLED=0 GOARCH=" + runtime.GOARCH + " GOOS=windows"},
				}
				want["list -test"] = []string{tmuxList("darwin", runtime.GOARCH), tmuxList("linux", "amd64"), tmuxList("linux", "arm64")}
				want["test -count=1 -v -run ^"+tmuxTestName+"$"] = []string{here, here + " SMOLCORP_TMUXGATE_CONTROL=1"}
				// And the script's run of its own tests, with the variable.
				want["test -count=1 -parallel 8 "] = []string{here + " " + releaseGateRows + "=1"}
				got := map[string][]string{}
				for _, call := range forgedCalls(t, r) {
					for step := range want {
						if strings.HasPrefix(call.args, step) {
							var own []string
							for _, name := range forgedPerCall {
								if value, ok := call.env[name]; ok {
									own = append(own, name+"="+value)
								}
							}
							got[step] = append(got[step], strings.Join(own, " "))
						}
					}
				}
				for step := range want {
					if !slices.Equal(got[step], want[step]) {
						t.Errorf("go %s ran with these settings of its own:\n  %s\nwant:\n  %s", step,
							strings.Join(got[step], "\n  "), strings.Join(want[step], "\n  "))
					}
				}
				if status != 97 || !strings.Contains(out, forgedPassed) || !strings.Contains(out, goReached) ||
					strings.Contains(out, "Stopped: ") {
					t.Errorf("want the table passed and the release on to its first build, got status %d:\n%s", status, out)
				}
			}},
		// The caller's library and header folders, searched before the SDK's (RT126SAX11R3 B4); go's build cache is
		// the run's own, fresh, so the headers would be read.
		{name: "failing, the caller's LIBRARY_PATH naming a folder of stand-in libraries", bodies: failOne, extra: forgedCgo,
			env: func(t *testing.T, r *releaseRepo, ran string) []string {
				libs := map[string]string{}
				for _, name := range []string{"libSystem", "libresolv", "libpthread", "libc", "libobjc", "libdl", "libm"} {
					for _, kind := range []string{".tbd", ".dylib", ".so", ".a"} {
						libs[name+kind] = "stand-in library " + forgedMark + "\n"
					}
				}
				return []string{"LIBRARY_PATH=" + forgedCallerFolder(t, libs)}
			}, want: stopForged},
		{name: "failing, the caller's CPATH and the other include paths naming a folder with a stand-in stdlib.h, the cache cold",
			bodies: failOne, extra: forgedCgo, env: func(t *testing.T, r *releaseRepo, ran string) []string {
				headers := map[string]string{}
				for _, name := range []string{"stdlib.h", "stdio.h", "string.h", "stdint.h", "stddef.h", "errno.h", "pthread.h",
					"unistd.h", "signal.h"} {
					headers[name] = "#error \"stand-in header " + forgedMark + "\"\n"
				}
				dir := forgedCallerFolder(t, headers)
				return []string{"CPATH=" + dir, "C_INCLUDE_PATH=" + dir, "CPLUS_INCLUDE_PATH=" + dir, "OBJC_INCLUDE_PATH=" + dir,
					"OBJCPLUS_INCLUDE_PATH=" + dir}
			}, want: stopForged},
		// The allow-list holds for settings nobody has thought of: every go call is checked below, this one included.
		{name: "failing, a setting nobody has thought of, made up here, never reaching go", bodies: failOne, extra: forgedCgo,
			env: func(t *testing.T, r *releaseRepo, ran string) []string {
				return []string{"SMOLCORP_MADE_UP_SETTING=the caller's"}
			}, want: stopForged},
		// go's caches are the run's own, fresh (SIERRA, RT126SAX11R3): folders the caller prepared are left alone.
		{name: "whole, the caller's GOCACHE, GOMODCACHE and GOPATH naming folders prepared beforehand", extra: forgedCgo,
			env: func(t *testing.T, r *releaseRepo, ran string) []string {
				var env []string
				for _, name := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
					prepared := forgedCallerFolder(t, map[string]string{"prepared": name + "\n"})
					env = append(env, name+"="+prepared)
					t.Cleanup(func() {
						if entries, _ := os.ReadDir(prepared); len(entries) != 1 {
							t.Errorf("go used the caller's %s (%s): it holds %d files", name, prepared, len(entries))
						}
					})
				}
				return env
			}, want: ""},
		{name: "whole, a test of the table leaving a file beside the code", bodies: map[string]string{
			forgedTable[0][0]: `if err := os.WriteFile("stray.txt", nil, 0o644); err != nil { t.Fatal(err) }`},
			want: stopForgedTree},
	} {
		t.Run(c.name, func(t *testing.T) {
			ran := filepath.Join(t.TempDir(), "ran")
			target := real
			if c.target != nil {
				target = c.target(t, ran)
			}
			r := forgedRepo(t, target, c.toolchain, c.bodies, c.extra)
			r.script = c.script
			var env []string
			if c.env != nil {
				env = c.env(t, r, ran)
			}
			// The DROP and DROPCC cases look at their own folders.
			for _, kv := range env {
				name, drop, _ := strings.Cut(kv, "=")
				if name == "DROP" || name == "DROPCC" {
					defer func() {
						if stood, _ := os.ReadFile(filepath.Join(drop, "ran")); len(stood) > 0 {
							t.Errorf("a go, env or C compiler put first on PATH after the start ran:\n%s", stood)
						}
					}()
				}
			}
			status, out := r.release(env...)
			forgedEnvChecks(t, r, out)

			// The log starts with the pinned go: its real path, this go's own version (no other toolchain's), and
			// its SHA-256.
			pinned, err := filepath.EvalSymlinks(filepath.Join(filepath.SplitList(pathOf(r))[0], "go"))
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(pinned)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(data)
			head := "go: " + pinned + "\n  go version go"
			if !strings.HasPrefix(out, head) || !strings.Contains(out, "\n  sha256 "+hex.EncodeToString(sum[:])+"\n") ||
				strings.Contains(out, "go1.99.0") {
				t.Errorf("the log doesn't start with the pinned go (%s), its own version and its SHA-256:\n%s", pinned, out)
			}
			// Then the C compiler found at the start, by its real path, with its SHA-256, and on a Mac its SDK.
			cc, sdk := releaseCC(t, r)
			ccData, err := os.ReadFile(cc)
			if err != nil {
				t.Fatal(err)
			}
			ccSum := sha256.Sum256(ccData)
			ccLines := "\ncc: " + cc + "\n  sha256 " + hex.EncodeToString(ccSum[:]) + "\n"
			if sdk != "" {
				ccLines += "  sdk " + sdk + "\n"
			}
			if !strings.Contains(out, ccLines) {
				t.Errorf("the log doesn't name the C compiler found at the start (%s), its SHA-256 and SDK:\n%s", cc, out)
			}
			if stood, _ := os.ReadFile(ran); len(stood) > 0 {
				t.Errorf("a stand-in ran in place of go, the gate or the tests:\n%s", stood)
			}
			after := strings.Index(out, checkPublicRan)
			if c.then != nil {
				c.then(t, r, status, out)
				return
			}
			if c.want == "" {
				// On to the next steps after the gate passed: the office's check, then the Linux tmux's, whose stand-in
				// stops the script there, and nothing else.
				passed, office, next := strings.Index(out, forgedPassed), strings.Index(out, officeRan), strings.Index(out, goReached)
				if status != 1 || after < 0 || passed < after || office < passed || next < office ||
					strings.Count(out, "Stopped: ") != 1 || !strings.Contains(out, "Stopped: build/tmux-linux/amd64 ") {
					t.Errorf("want the table passed after check-public.sh and the release on to the next steps, got status %d:\n%s", status, out)
				}
				return
			}
			if status != 1 || !strings.Contains(out, c.want) || after < 0 || strings.Index(out, c.want) < after ||
				strings.Contains(out, officeRan) || strings.Contains(out, goReached) {
				t.Errorf("want a stop (%q) after check-public.sh and before anything else, got status %d:\n%s", c.want, status, out)
			}
		})
	}
}

// forgedEnvChecks checks, for every case, that each go the release ran got the list of go's settings the log starts
// with and nothing else (RT126SAX11R3 B4): HOME, the system's PATH, TMPDIR, the C compiler found at the start, go's
// own folders for the run, made fresh in TMPDIR and gone when it ended after their size was logged (SIERRA), go's own
// download folder for this account read offline, and the fixed go settings. No file the caller named was read.
func forgedEnvChecks(t *testing.T, r *releaseRepo, out string) {
	t.Helper()
	settings := forgedSettings(out)
	cc, sdk := releaseCC(t, r)
	var tmp string
	for _, kv := range r.env {
		if value, ok := strings.CutPrefix(kv, "TMPDIR="); ok {
			tmp = value
		}
	}
	run := filepath.Dir(settings["GOCACHE"])
	want := forgedWant(r.home, tmp, run, cc, sdk)
	pinned, err := filepath.EvalSymlinks(filepath.Join(filepath.SplitList(pathOf(r))[0], "go"))
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(settings, want) ||
		filepath.Dir(run) != tmp || !strings.HasPrefix(filepath.Base(run), "smolcorp-go.") {
		t.Errorf("the log's list of go's settings:\n  %v\nwant:\n  %v (go's folders in %s)", settings, want, tmp)
	}
	if _, err := os.Stat(run); err == nil || !strings.Contains(out, "go's folders for this run ("+run+"): ") ||
		!strings.Contains(out, " KB, removed.") {
		t.Errorf("go's folders for the run (%s) weren't removed after their size was logged:\n%s", run, out)
	}
	for _, call := range forgedCalls(t, r) {
		listed := settings
		// The script's run of its own tests: the pinned go's folder first on PATH, then the list's.
		if _, rows := call.env[releaseGateRows]; rows {
			listed = maps.Clone(settings)
			listed["PATH"] = filepath.Dir(pinned) + ":" + settings["PATH"]
		}
		for name, value := range call.env {
			if v, ok := listed[name]; !slices.Contains(forgedPerCall, name) && (!ok || v != value) {
				t.Errorf("go %s got %s=%s, which isn't in the log's list of go's settings", call.args, name, value)
			}
		}
		// Every call after the list is made has all of it; before (go env for this machine's system and go's own
		// download folder) only some.
		if !strings.HasPrefix(call.args, "env ") {
			for name, value := range listed {
				if call.env[name] != value {
					t.Errorf("go %s ran without the log's %s=%s", call.args, name, value)
				}
			}
		}
	}
	if strings.Contains(out, forgedMark) {
		t.Errorf("the build read a file the caller named:\n%s", out)
	}
}

// pathOf is the PATH r's script runs with.
func pathOf(r *releaseRepo) string {
	for _, kv := range r.env {
		if value, ok := strings.CutPrefix(kv, "PATH="); ok {
			return value
		}
	}
	return ""
}

// forgedWant is the list of go's settings every go of a release gets, name → value, and nothing else: HOME, the
// system's PATH, TMPDIR, go's own folders for the run (in run), go's own download folder for the account read offline,
// the fixed go settings, and the C compiler found at the start with, on a Mac, its SDK.
func forgedWant(home, tmp, run, cc, sdk string) map[string]string {
	want := map[string]string{"HOME": home, "PATH": "/usr/bin:/bin:/usr/sbin:/sbin", "TMPDIR": tmp,
		"GOCACHE": run + "/cache", "GOMODCACHE": run + "/modules", "GOPATH": run + "/gopath",
		"GOPROXY": "file://" + home + "/go/pkg/mod/cache/download", "GOSUMDB": "off", "GOFLAGS": "-mod=readonly",
		"GOENV": "off", "GOWORK": "off", "GOTOOLCHAIN": "local", "LANG": "C", "CC": cc}
	if sdk != "" {
		want["SDKROOT"] = sdk
	}
	return want
}

// A fast row, in every go test of this package (no go build, no run of the script): release-desktop.sh's side, with the
// gate it sources (scripts/release-gate.sh), of what
// the forged-event table's heavy rows check. Its list of go's settings is forgedWant's, set from the run's own values;
// every go it runs gets that list through /usr/bin/env -i, its own tests' run with the pinned go's folder first on
// PATH; go's folders for the run are made fresh and removed by a trap set before its first go, their size logged; the
// list is logged; the forged-event table runs after the bundled tmux's checks and this script's own tests, before
// the office's check, with HEAD's tree checked again between; and its rows are heavy rows, here and in the script.
func TestReleaseDesktopGivesEveryGoOnlyItsOwnList(t *testing.T) {
	// The script as it runs: the gate it sources, in its place (RELGATE1).
	script := releaseDesktopAsRun(t)

	// The forged-event table's rows take minutes: never in a plain go test, always in the script's run of its own.
	const forgedRows = "TestReleaseDesktopStopsUnlessTheForgedEventTableIsWhole"
	if !slices.Contains(heavyReleaseTests(t), forgedRows) {
		t.Errorf("%s isn't a heavy row (heavyReleaseRows(t) first)", forgedRows)
	}
	heavy := regexp.MustCompile(`(?m)^RELEASE_ROWS_HEAVY=\(([^)]*)\)$`).FindStringSubmatch(script)
	if heavy == nil || !slices.Contains(strings.Fields(heavy[1]), forgedRows) {
		t.Errorf("release-desktop.sh's RELEASE_ROWS_HEAVY doesn't name %s", forgedRows)
	}
	once := func(what string) int {
		t.Helper()
		if n := strings.Count(script, what); n != 1 {
			t.Errorf("release-desktop.sh has %q %d times, want once", what, n)
		}
		return strings.Index(script, what)
	}

	// The list, as the script sets it: each name forgedWant has, from what the script found at its start.
	listed := map[string]string{}
	lists := regexp.MustCompile(`(?ms)^ *go_env\+?=\(([^)]*)\)$`).FindAllStringSubmatch(script, -1)
	for _, list := range lists {
		for _, kv := range strings.Fields(list[1]) {
			name, value, _ := strings.Cut(kv, "=")
			if _, twice := listed[name]; twice {
				t.Errorf("release-desktop.sh sets %s twice in go_env", name)
			}
			listed[name] = value
		}
	}
	from := map[string]string{"HOME": `"$HOME"`, "PATH": `"$system_path"`, "TMPDIR": `"$temp_dir"`,
		"GOCACHE": `"$go_run/cache"`, "GOMODCACHE": `"$go_run/modules"`, "GOPATH": `"$go_run/gopath"`,
		"GOPROXY": `"file://$go_downloads"`, "CC": `"$CC_PROGRAM"`, "SDKROOT": `"$SDK"`}
	want := forgedWant("home", "tmp", "run", "cc", "sdk")
	for name, value := range want {
		if found, ok := from[name]; ok {
			value = found
		}
		if listed[name] != value {
			t.Errorf("release-desktop.sh's go_env has %s=%s, want %s", name, listed[name], value)
		}
	}
	for name := range listed {
		if _, ok := want[name]; !ok {
			t.Errorf("release-desktop.sh's go_env has %s, which forgedWant doesn't", name)
		}
	}
	once("\nsystem_path=" + want["PATH"] + "\n")
	pinnedEnv := once("\npinned_env() { /usr/bin/env -i \"${go_env[@]}\" \"$@\"; }\n")
	once("\necho \"" + forgedGoSettings + "\"\nprintf '  %s\\n' \"${go_env[@]}\"\n")

	// go's folders for the run: made fresh, then the trap that logs their size and removes them, before any go runs.
	made := once("\ngo_run=\"$(mktemp -d \"$temp_dir/smolcorp-go.XXXXXX\")\"\n")
	trap := once("\ntrap on_exit EXIT\n")
	_, onExit, _ := strings.Cut(script, "\non_exit() {\n")
	onExit, _, _ = strings.Cut(onExit, "\n}\n")
	onExit = "\n" + onExit + "\n"
	for _, line := range []string{`echo "go's folders for this run ($go_run): $(du -sk "$go_run" 2>/dev/null | cut -f 1) KB, removed."`,
		`rm -rf "$go_run"`} {
		if !strings.Contains(onExit, "\n  "+line+"\n") {
			t.Errorf("release-desktop.sh's on_exit doesn't have %s:\n%s", line, onExit)
		}
	}

	// Every go it runs, through /usr/bin/env -i or pinned_env, on one line (a backslash goes on into the next); none
	// by name.
	goCall := regexp.MustCompile(`"\$GO" ("\$@"|[a-z]+)`)
	pinnedEnvWord := regexp.MustCompile(`\bpinned_env `)
	byName := regexp.MustCompile(`(^|[;&|(!]|\bthen|\bdo|\belse)\s*go\s+[a-z]`)
	// Offsets stay the script's: a backslash and its newline are two characters, as their stand-in is.
	calls, first, offset := 0, len(script), 0
	for _, line := range strings.Split(strings.ReplaceAll(script, "\\\n", "  "), "\n") {
		at := offset
		offset += len(line) + 1
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if byName.MatchString(line) {
			t.Errorf("release-desktop.sh runs go by name, not pinned: %s", line)
		}
		for _, m := range goCall.FindAllStringIndex(line, -1) {
			calls++
			first = min(first, at)
			before := line[:m[0]]
			if !strings.Contains(before, "/usr/bin/env -i ") && !pinnedEnvWord.MatchString(before) {
				t.Errorf("release-desktop.sh runs a go without its list (/usr/bin/env -i or pinned_env): %s", line)
			}
		}
	}
	if calls < 8 {
		t.Errorf("release-desktop.sh has %d go calls by \"$GO\", want every check's and build's", calls)
	}
	if !(made < trap && trap < first && pinnedEnv > trap) {
		t.Errorf("release-desktop.sh must make go's folders, then set its trap, before any go runs")
	}
	once(`PATH="$(dirname "$GO"):$system_path"`)

	// The forged-event table, after the bundled tmux's checks and this script's own tests, before the office's check,
	// HEAD's tree checked between.
	rows := once("\nif ! rows_passed; then\n")
	forged := once("\npinned_go run ./scripts/forgedevents -go \"$GO\" -cc \"$CC_PROGRAM\" -sdk \"${SDK:-}\" -work \"$go_run\" -modules \"$go_downloads\"\n")
	office := once("\npinned_go run ./scripts/buildoffice -check\n")
	if !(rows < forged && forged < office) || !strings.Contains(script[forged:office], "\nif ! heads_tree; then\n") {
		t.Errorf("release-desktop.sh must run the forged-event table after its own tests and before the office's check, HEAD's tree checked again after it")
	}
}
