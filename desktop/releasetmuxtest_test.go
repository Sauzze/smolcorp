//go:build unix

package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// The guarded tmux exits unless the core gives it SMOLCORP_TMUX_SERVER, so release-desktop.sh builds nothing until
// HEAD's tree holds the test that says the core always does, by its exact name, in desktop/ for the Mac's and Linux's
// builds alike, and that test passes on the machine the release is built on, then fails its control run with the
// fixed line (TMUXGATE1). M7/L4 write the real test; here it is a stand-in in each throwaway repo, run by the real go.
// These rows are heavy (heavyReleaseRows): the script runs them before every release, and go test skips them elsewhere.

const (
	tmuxTestName      = "TestNoExecOfTheBundledTmuxWithoutSmolcorpTmuxServer"
	tmuxTestFile      = "desktop/tmux_test.go"
	tmuxControl       = "SMOLCORP_TMUXGATE_CONTROL=1"
	tmuxControlLine   = "tmuxgate control: a start without SMOLCORP_TMUX_SERVER was let through"
	otherToolchainRan = "stand-in toolchain ran"

	stopNoTmuxTest      = "HEAD's tree has no " + tmuxTestName + " in desktop/"
	stopNotListed       = tmuxTestName + " isn't a test in ./desktop's "
	stopTmuxTestFailed  = tmuxTestName + " didn't pass in ./desktop"
	stopTmuxControl     = tmuxTestName + " didn't fail its control run in ./desktop"
	stopTmuxTestChanged = "the files changed while " + tmuxTestName + " ran"
)

// realGo is the go the stand-in hands go version, env, list and test to, and the build cache it already has, so each
// repo's test builds in a moment.
var realGo = func() func(t *testing.T) (string, string) {
	once := sync.OnceValues(func() (string, error) {
		path, err := exec.LookPath("go")
		if err != nil {
			return "", err
		}
		out, err := exec.Command(path, "env", "GOCACHE").Output()
		if err != nil {
			return "", err
		}
		return path + "\n" + strings.TrimSpace(string(out)), nil
	})
	return func(t *testing.T) (string, string) {
		t.Helper()
		both, err := once()
		if err != nil {
			t.Fatalf("the release tests need go on PATH and its build cache: %v", err)
		}
		path, cache, _ := strings.Cut(both, "\n")
		return path, cache
	}
}()

// standInTmuxTest is a stand-in for M7/L4's test. As written with no fields set it keeps their contract: the stand-in
// core starts a stand-in tmux (which exits 98 without SMOLCORP_TMUX_SERVER, as the guard does) with that variable
// and nothing else, a recorder reports each start the guard stopped with the control line, and with
// SMOLCORP_TMUXGATE_CONTROL=1 the test also makes one start without the variable through that recorder.
type standInTmuxTest struct {
	inherits  bool   // the core passes its own environment on instead
	guard     string // the variable the stand-in tmux needs, if not SMOLCORP_TMUX_SERVER
	noControl bool   // the test ignores the control variable
	extra     string // more of the test's body
	pkg       string // its package, if not desktop
}

func (s standInTmuxTest) source() string {
	core := `cmd.Env = []string{"SMOLCORP_TMUX_SERVER=stand-in"}`
	if s.inherits {
		core = "// It passes its own environment on."
	}
	guard, pkg := s.guard, s.pkg
	if guard == "" {
		guard = "SMOLCORP_TMUX_SERVER"
	}
	if pkg == "" {
		pkg = "desktop"
	}
	control := `if os.Getenv("SMOLCORP_TMUXGATE_CONTROL") == "1" {
		broken := exec.Command(tmux)
		broken.Env = []string{}
		record(broken.Run())
	}`
	if s.noControl {
		control = ""
	}
	return "package " + pkg + `

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func startStandInTmux(tmux string) error {
	cmd := exec.Command(tmux)
	` + core + `
	return cmd.Run()
}

func ` + tmuxTestName + `(t *testing.T) {
	tmux := filepath.Join(t.TempDir(), "tmux")
	if err := os.WriteFile(tmux, []byte("#!/bin/sh\n[ -n \"${` + guard + `:-}\" ] || exit 98\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	record := func(err error) {
		if err != nil {
			t.Error("` + tmuxControlLine + `")
		}
	}
	record(startStandInTmux(tmux))
	` + control + `
	` + s.extra + `
}
`
}

// tmuxTestSource is a test of the name in package desktop that does only what body does, with the imports it needs.
func tmuxTestSource(body string, imports ...string) string {
	imports = append([]string{"testing"}, imports...)
	return "package desktop\n\nimport (\n\t\"" + strings.Join(imports, "\"\n\t\"") + "\"\n)\n\n" +
		"func " + tmuxTestName + "(t *testing.T) {\n\t" + body + "\n}\n"
}

// tmuxContractAnswer is shell that prints what go test of a test keeping M7/L4's contract prints, and exits as it does: a
// pass, or with the control variable the control line and a fail.
const tmuxContractAnswer = "echo '=== RUN   " + tmuxTestName + "'\n" +
	"if [ -n \"${SMOLCORP_TMUXGATE_CONTROL:-}\" ]; then\n" +
	"  echo '    tmux_test.go:1: " + tmuxControlLine + "'\n  echo '--- FAIL: " + tmuxTestName + " (0.00s)'\n  echo FAIL\n  exit 1\nfi\n" +
	"echo '--- PASS: " + tmuxTestName + " (0.00s)'\necho PASS\nexit 0\n"

// writeOtherGo writes a go that isn't the one the script pinned: go test only prints what a test keeping the contract
// would and says it ran; anything else goes to the real go.
func (r *releaseRepo) writeOtherGo(path string) {
	r.t.Helper()
	goPath, _ := realGo(r.t)
	writeReleaseFile(r.t, path, "#!/bin/sh\n"+
		"case \"$1\" in test) ;; *) GOTOOLCHAIN=local exec '"+goPath+"' \"$@\";; esac\n"+
		"echo '"+otherToolchainRan+"'\n"+tmuxContractAnswer, 0o755)
}

// squashedAfter makes edit's changes on the branch, as one more commit, and checks out the release commit made of it.
func (r *releaseRepo) squashedAfter(edit func()) {
	r.t.Helper()
	edit()
	r.git("add", "-A")
	r.git("commit", "-q", "--allow-empty", "-m", "edit")
	r.squashed()
}

// remove deletes a file in the repo, at a path relative to it.
func (r *releaseRepo) remove(path string) {
	r.t.Helper()
	if err := os.Remove(filepath.Join(r.dir, path)); err != nil {
		r.t.Fatal(err)
	}
}

// fakePass writes a program that says the bundled tmux's test passed without running anything, as go test's -exec
// would run it, and gives its path.
func (r *releaseRepo) fakePass() string {
	r.t.Helper()
	path := filepath.Join(r.home, "fake-pass")
	writeReleaseFile(r.t, path, "#!/bin/sh\necho '=== RUN   "+tmuxTestName+"'\necho '--- PASS: "+tmuxTestName+" (0.00s)'\necho PASS\n", 0o755)
	return path
}

// pinnedGo is what the script must say at its start about the go it runs: the stand-in's real path and its SHA-256.
func (r *releaseRepo) pinnedGo() []string {
	r.t.Helper()
	path, err := filepath.EvalSymlinks(filepath.Join(r.bin, "go"))
	if err != nil {
		r.t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		r.t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return []string{"go: " + path + "\n", "\n  sha256 " + hex.EncodeToString(sum[:]) + "\n", "\ncc: "}
}

func TestReleaseDesktopNeedsTheBundledTmuxTestPassing(t *testing.T) {
	heavyReleaseRows(t)
	failing := tmuxTestSource(`t.Fatal("stand-in: the bundled tmux ran without SMOLCORP_TMUX_SERVER")`)
	// What the control run must fail with, as the test's own line.
	controlFail := `if os.Getenv("SMOLCORP_TMUXGATE_CONTROL") == "1" { t.Error("` + tmuxControlLine + `") }`
	// A darwin stub that only logs, beside the real test for Linux, failing here: on a Mac the stub runs (and fails
	// its control run), on Linux the failing test does.
	p4 := stopTmuxControl
	if runtime.GOOS == "linux" {
		p4 = stopTmuxTestFailed
	}
	// tmuxCase is one release commit, in a repo of its own: edit is what it changes on the branch, extra any
	// environment the script runs with, and want the stop it must make after check-public.sh and before any build,
	// or "" when it must get past every check up to go.
	type tmuxCase struct {
		name, want string
		edit       func(r *releaseRepo)
		extra      func(r *releaseRepo) []string
	}
	stopUnreadable := "git grep couldn't read HEAD's tree"
	keep := func(r *releaseRepo) {}
	leaks := func(setting string) func(r *releaseRepo) []string {
		return func(r *releaseRepo) []string { return []string{setting} }
	}
	inherits := func(guard string) func(r *releaseRepo) {
		return func(r *releaseRepo) { r.write(tmuxTestFile, standInTmuxTest{inherits: true, guard: guard}.source()) }
	}
	for _, c := range []tmuxCase{
		{"a test keeping the contract, in desktop/", "", keep, nil},
		{"the test keeping the contract, GOOS for another system from the caller", "", keep,
			func(r *releaseRepo) []string { return []string{"GOOS=plan9", "GOARCH=amd64"} }},
		{"the test keeping the contract, with this machine's Smolcorp and tmux settings", "", keep,
			func(r *releaseRepo) []string {
				return []string{"SMOLCORP_TMUX_SERVER=off", "TMUX=/stand-in/tmux,1,0", "TMUX_PANE=%1", "SMOLCORP_SUPPORT_DIR=x"}
			}},
		{"the test keeping the contract, the caller's control variable already set", "", keep, leaks(tmuxControl)},
		{"OK2: the test keeping the contract as package desktop_test", "", func(r *releaseRepo) {
			r.write(tmuxTestFile, standInTmuxTest{pkg: "desktop_test"}.source())
		}, nil},
		{"a failing copy of the test in another package (only desktop/'s counts)", "", func(r *releaseRepo) {
			r.write("gate/gate_test.go", strings.Replace(failing, "package desktop", "package gate", 1))
		}, nil},

		// Not in desktop/, or not under the name.
		{"no such test", stopNoTmuxTest, func(r *releaseRepo) { r.remove(tmuxTestFile) }, nil},
		{"the test only in the top folder's package", stopNoTmuxTest, func(r *releaseRepo) {
			r.remove(tmuxTestFile)
			r.write("tmux_test.go", standInTmuxTest{pkg: "smolcorp"}.source())
		}, nil},
		{"P3: the test gone from desktop/, a copy keeping the contract in another package", stopNoTmuxTest, func(r *releaseRepo) {
			r.remove(tmuxTestFile)
			r.write("stub/stub_test.go", standInTmuxTest{pkg: "stub"}.source())
		}, nil},
		{"P3b: the test gone from desktop/, an empty copy in another package", stopNoTmuxTest, func(r *releaseRepo) {
			r.remove(tmuxTestFile)
			r.write("stub/stub_test.go", strings.Replace(tmuxTestSource(""), "package desktop", "package stub", 1))
		}, nil},
		{"P6c: the test gone from desktop/, a copy keeping the contract in desktop/sub", stopNoTmuxTest, func(r *releaseRepo) {
			r.remove(tmuxTestFile)
			r.write("desktop/sub/tmux_test.go", standInTmuxTest{pkg: "sub"}.source())
		}, nil},
		{"P6: the test gone from desktop/, a copy keeping the contract under desktop/testdata", stopNoTmuxTest, func(r *releaseRepo) {
			r.remove(tmuxTestFile)
			r.write("desktop/testdata/tmux_test.go", standInTmuxTest{}.source())
		}, nil},
		{"the test renamed, a word added", stopNoTmuxTest, func(r *releaseRepo) {
			r.write(tmuxTestFile, strings.Replace(standInTmuxTest{}.source(), tmuxTestName, tmuxTestName+"OnLinux", 1))
		}, nil},
		{"the test renamed, another case", stopNoTmuxTest, func(r *releaseRepo) {
			r.write(tmuxTestFile, strings.Replace(standInTmuxTest{}.source(), "SmolcorpTmuxServer(", "SMOLCORP_TMUX_SERVER(", 1))
		}, nil},
		{"the name in a comment only", stopNoTmuxTest, func(r *releaseRepo) {
			r.write(tmuxTestFile, "package desktop\n\n// func "+tmuxTestName+"(t *testing.T) is still to write.\n")
		}, nil},
		{"the test in a file that isn't a test file", stopNoTmuxTest, func(r *releaseRepo) {
			r.remove(tmuxTestFile)
			r.write("desktop/tmux.go", tmuxTestSource(""))
		}, nil},
		{"the test only in an ignored file beside HEAD's tree", stopNoTmuxTest, func(r *releaseRepo) {
			r.remove(tmuxTestFile)
			r.write(".git/info/exclude", "/notes/\n")
			r.write("notes/tmux_test.go", standInTmuxTest{}.source())
		}, nil},

		// Its name at a line's start, but not a test go would run, or not in every system's build.
		{"the name at a line's start in a /* */ comment", stopNotListed + "darwin/", func(r *releaseRepo) {
			r.write(tmuxTestFile, "package desktop\n\n/*\nfunc "+tmuxTestName+"(t *testing.T) {}\n*/\n")
		}, nil},
		{"the name at a line's start in a raw string", stopNotListed + "darwin/", func(r *releaseRepo) {
			r.write(tmuxTestFile, "package desktop\n\nvar notATest = `\nfunc "+tmuxTestName+"(t *testing.T) {}\n`\n")
		}, nil},
		{"the test built only for another system", stopNotListed + "darwin/", func(r *releaseRepo) {
			r.write(tmuxTestFile, "//go:build plan9\n\n"+standInTmuxTest{}.source())
		}, nil},
		{"the test built only for another system, its package building here", stopNotListed + "darwin/", func(r *releaseRepo) {
			r.write(tmuxTestFile, "//go:build plan9\n\n"+standInTmuxTest{}.source())
			r.write("desktop/desktop.go", "package desktop\n")
		}, nil},
		{"P4: the test only in a darwin file (no Linux build has it)", stopNotListed, func(r *releaseRepo) {
			r.remove(tmuxTestFile)
			r.write("desktop/tmux_darwin_test.go", standInTmuxTest{}.source())
			r.write("desktop/desktop.go", "package desktop\n")
		}, nil},
		{"the test only in a linux file (no darwin build has it)", stopNotListed, func(r *releaseRepo) {
			r.remove(tmuxTestFile)
			r.write("desktop/tmux_linux_test.go", standInTmuxTest{}.source())
			r.write("desktop/desktop.go", "package desktop\n")
		}, nil},
		{"the test built for darwin and linux/amd64 only", stopNotListed + "linux/arm64", func(r *releaseRepo) {
			r.write(tmuxTestFile, "//go:build darwin || (linux && amd64)\n\n"+standInTmuxTest{}.source())
			r.write("desktop/desktop.go", "package desktop\n")
		}, nil},
		{"P4: a darwin file that only logs, beside a failing Linux file", p4, func(r *releaseRepo) {
			r.remove(tmuxTestFile)
			r.write("desktop/tmux_darwin_test.go", tmuxTestSource(`t.Log("checked on Linux CI")`))
			r.write("desktop/tmux_linux_test.go", failing)
		}, nil},

		// Run as it is: failed or skipped.
		{"the test failing", stopTmuxTestFailed, func(r *releaseRepo) { r.write(tmuxTestFile, failing) }, nil},
		{"the core starting the tmux without the variable", stopTmuxTestFailed, func(r *releaseRepo) {
			r.write(tmuxTestFile, strings.Replace(standInTmuxTest{}.source(), `"SMOLCORP_TMUX_SERVER=stand-in"`, "", 1))
		}, nil},
		{"the test skipped", stopTmuxTestFailed, func(r *releaseRepo) {
			r.write(tmuxTestFile, tmuxTestSource(`t.Skip("stand-in: no bundled tmux here")`))
		}, nil},
		{"a part of the test skipped", stopTmuxTestFailed, func(r *releaseRepo) {
			r.write(tmuxTestFile, standInTmuxTest{extra: `t.Run("linux", func(t *testing.T) { t.Skip("stand-in: not Linux") })`}.source())
		}, nil},
		{"the test beside another test it calls, that one failing", stopTmuxTestFailed, func(r *releaseRepo) {
			r.write(tmuxTestFile, standInTmuxTest{extra: `t.Run("hires", TestBundledTmuxHires)`}.source()+
				"\nfunc TestBundledTmuxHires(t *testing.T) { t.Error(\"stand-in: a hire's tmux had no variable\") }\n")
		}, nil},

		// B1: a test that passes only because this machine's settings reach the core and the tmux it starts.
		{"P1a: passing only through the caller's SMOLCORP_TMUX_SERVER", stopTmuxTestFailed,
			inherits("SMOLCORP_TMUX_SERVER"), leaks("SMOLCORP_TMUX_SERVER=off")},
		{"P1c: passing only through the caller's SMOLCORP_TMUX_SERVER, the caller's control variable set too", stopTmuxTestFailed,
			inherits("SMOLCORP_TMUX_SERVER"), func(r *releaseRepo) []string { return []string{"SMOLCORP_TMUX_SERVER=off", tmuxControl} }},
		{"P1n: passing only through the caller's SMOLCORP_TMUX_SERVER, its value holding a newline and TMUX=b", stopTmuxTestFailed,
			inherits("SMOLCORP_TMUX_SERVER"), leaks("SMOLCORP_TMUX_SERVER=off\nTMUX=b")},
		{"passing only through the caller's TMUX", stopTmuxTestFailed, inherits("TMUX"), leaks("TMUX=/stand-in/tmux,1,0")},
		{"passing only through the caller's TMUX_PANE", stopTmuxTestFailed, inherits("TMUX_PANE"), leaks("TMUX_PANE=%1")},
		{"passing only through the caller's TMUX_TMPDIR", stopTmuxTestFailed, inherits("TMUX_TMPDIR"), leaks("TMUX_TMPDIR=/stand-in")},
		{"passing only through another SMOLCORP_ variable of the caller's", stopTmuxTestFailed,
			inherits("SMOLCORP_SUPPORT_DIR"), leaks("SMOLCORP_SUPPORT_DIR=/stand-in")},

		// The control run (SIERRA's B2 option 3): passing as it is, it must fail with the line, once.
		{"P2: an empty body", stopTmuxControl, func(r *releaseRepo) { r.write(tmuxTestFile, tmuxTestSource("")) }, nil},
		{"a stub that only logs", stopTmuxControl, func(r *releaseRepo) {
			r.write(tmuxTestFile, tmuxTestSource(`t.Log("covered on Linux CI")`))
		}, nil},
		{"a test that ignores the control variable", stopTmuxControl, func(r *releaseRepo) {
			r.write(tmuxTestFile, standInTmuxTest{noControl: true}.source())
		}, nil},
		{"P5: a TestMain that prints the pass and exits 0 without m.Run", stopTmuxControl, func(r *releaseRepo) {
			r.write(tmuxTestFile, tmuxTestSource("", "fmt", "os")+"\nfunc TestMain(m *testing.M) {\n"+
				"\tfmt.Println(\"=== RUN   "+tmuxTestName+"\\n--- PASS: "+tmuxTestName+" (0.00s)\\nPASS\")\n\tos.Exit(0)\n}\n")
		}, nil},
		{"the control run failing with another line", stopTmuxControl, func(r *releaseRepo) {
			r.write(tmuxTestFile, tmuxTestSource(strings.Replace(controlFail, "tmuxgate control:", "tmuxgate check:", 1), "os"))
		}, nil},
		{"the control run failing with the line twice", stopTmuxControl, func(r *releaseRepo) {
			r.write(tmuxTestFile, tmuxTestSource(controlFail+"\n\t"+controlFail, "os"))
		}, nil},
		{"V2: the control run printing the line with fmt and passing", stopTmuxControl, func(r *releaseRepo) {
			r.write(tmuxTestFile, tmuxTestSource(`if os.Getenv("SMOLCORP_TMUXGATE_CONTROL") == "1" { fmt.Println("    tmux_test.go:1: `+
				tmuxControlLine+`") }`, "fmt", "os"))
		}, nil},
		{"V3b: the control run failing with the line in two subtests", stopTmuxControl, func(r *releaseRepo) {
			r.write(tmuxTestFile, tmuxTestSource(`for _, name := range []string{"a", "b"} {
		t.Run(name, func(t *testing.T) { `+controlFail+` })
	}`, "os"))
		}, nil},
		{"the control run logging the line and passing", stopTmuxControl, func(r *releaseRepo) {
			r.write(tmuxTestFile, tmuxTestSource(strings.Replace(controlFail, "t.Error(", "t.Log(", 1), "os"))
		}, nil},
		{"the control run logging the line and exiting 1, no test failed", stopTmuxControl, func(r *releaseRepo) {
			r.write(tmuxTestFile, tmuxTestSource(strings.Replace(controlFail, `t.Error("`+tmuxControlLine+`")`,
				`t.Log("`+tmuxControlLine+`"); os.Exit(1)`, 1), "os"))
		}, nil},
		{"the control run failing with the line in a message of its own", stopTmuxControl, func(r *releaseRepo) {
			r.write(tmuxTestFile, tmuxTestSource(strings.Replace(controlFail, `t.Error("`, `t.Error("stand-in: `, 1), "os"))
		}, nil},
		// What the control run can't tell: a test written to answer it and nothing else gets through. Only M7/L4's
		// review can say the test checks the core's starts.
		{"still gets through: a test that only fails when told to", "", func(r *releaseRepo) {
			r.write(tmuxTestFile, tmuxTestSource(controlFail, "os"))
		}, nil},

		// The caller's go settings: a pass forged by a stand-in, or another toolchain.
		{"the test failing, the caller's GOFLAGS running a stand-in that says it passed", stopTmuxTestFailed,
			func(r *releaseRepo) { r.write(tmuxTestFile, failing) },
			func(r *releaseRepo) []string { return []string{"GOFLAGS=-exec=" + r.fakePass()} }},
		{"the test failing, go env's GOFLAGS running a stand-in that says it passed", stopTmuxTestFailed,
			func(r *releaseRepo) { r.write(tmuxTestFile, failing) },
			func(r *releaseRepo) []string {
				env := filepath.Join(r.home, "go-env")
				writeReleaseFile(r.t, env, "GOFLAGS=-exec="+r.fakePass()+"\n", 0o644)
				return []string{"GOENV=" + env}
			}},
		{"G7: the test failing, the caller's GOTOOLCHAIN naming a stand-in on PATH", stopTmuxTestFailed,
			func(r *releaseRepo) { r.write(tmuxTestFile, failing) },
			func(r *releaseRepo) []string {
				r.writeOtherGo(filepath.Join(r.bin, "go1.99.0"))
				return []string{"GOTOOLCHAIN=go1.99.0"}
			}},
		{"G7b: the test failing, go env's GOTOOLCHAIN naming a stand-in on PATH", stopTmuxTestFailed,
			func(r *releaseRepo) { r.write(tmuxTestFile, failing) },
			func(r *releaseRepo) []string {
				r.writeOtherGo(filepath.Join(r.bin, "go1.99.0"))
				env := filepath.Join(r.home, "go-env")
				writeReleaseFile(r.t, env, "GOTOOLCHAIN=go1.99.0\n", 0o644)
				return []string{"GOENV=" + env, "GOTOOLCHAIN="}
			}},
		{"G7c: the test failing, go.mod's toolchain line naming a stand-in on PATH", stopTmuxTestFailed,
			func(r *releaseRepo) {
				r.write(tmuxTestFile, failing)
				r.write("go.mod", "module example.invalid/release\n\ngo 1.21\n\ntoolchain go1.99.0\n")
			},
			func(r *releaseRepo) []string {
				r.writeOtherGo(filepath.Join(r.bin, "go1.99.0"))
				return []string{"GOTOOLCHAIN="}
			}},
		{"G7d: go.mod asking for a newer go than the one pinned", stopNotListed + "darwin/", func(r *releaseRepo) {
			r.write("go.mod", "module example.invalid/release\n\ngo 1.99.0\n")
		}, func(r *releaseRepo) []string {
			r.writeOtherGo(filepath.Join(r.bin, "go1.99.0"))
			return []string{"GOTOOLCHAIN="}
		}},
		{"G7e: the test failing, the caller's GOTOOLCHAIN naming a stand-in on PATH, +path", stopTmuxTestFailed,
			func(r *releaseRepo) { r.write(tmuxTestFile, failing) },
			func(r *releaseRepo) []string {
				r.writeOtherGo(filepath.Join(r.bin, "go1.99.0"))
				return []string{"GOTOOLCHAIN=go1.99.0+path"}
			}},
		{"the test failing, another go put first on PATH after the start", stopTmuxTestFailed,
			func(r *releaseRepo) { r.write(tmuxTestFile, failing) },
			func(r *releaseRepo) []string {
				early := r.t.TempDir()
				return []string{"PATH=" + early + ":" + strings.TrimPrefix(r.env[0], "PATH="), "DROP_GO=" + filepath.Join(early, "go")}
			}},

		// git can't read the test's file in HEAD's tree (its object unreadable, the file on disk as committed): git grep
		// fails, which isn't "found".
		{"git grep failing on HEAD's tree", stopUnreadable, keep, func(r *releaseRepo) []string {
			blob := r.git("rev-parse", "HEAD:"+tmuxTestFile)
			if err := os.Chmod(filepath.Join(r.dir, ".git", "objects", blob[:2], blob[2:]), 0); err != nil {
				r.t.Fatal(err)
			}
			return nil
		}},
		{"V6a: the test keeping the contract, a file left beside the code in its control run only", stopTmuxTestChanged, func(r *releaseRepo) {
			r.write(tmuxTestFile, standInTmuxTest{extra: `if os.Getenv("SMOLCORP_TMUXGATE_CONTROL") == "1" {
		if err := os.WriteFile("stray.txt", nil, 0o644); err != nil { t.Fatal(err) }
	}`}.source())
		}, nil},
		{"the test keeping the contract, a file left beside the code", stopTmuxTestChanged, func(r *releaseRepo) {
			r.write(tmuxTestFile, standInTmuxTest{extra: `if err := os.WriteFile("stray.txt", nil, 0o644); err != nil { t.Fatal(err) }`}.source())
		}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newReleaseRepo(t)
			r.writeGo(true)
			r.squashedAfter(func() { c.edit(r) })
			var extra []string
			if c.extra != nil {
				extra = c.extra(r)
			}
			status, out := r.release(extra...)
			if strings.Contains(out, otherToolchainRan) {
				t.Errorf("a go other than the one pinned at the start ran the test:\n%s", out)
			}
			for _, line := range r.pinnedGo() {
				if !strings.Contains(out, line) {
					t.Errorf("the start doesn't name the pinned go and cc (%q):\n%s", line, out)
				}
			}
			if c.want == "" {
				// Past this check and every other: the test's own pass and its control run's fail are quoted.
				if status != 97 || !strings.Contains(out, goReached) || strings.Contains(out, "Stopped: ") ||
					!strings.Contains(out, "\n  --- PASS: "+tmuxTestName+" (") || !strings.Contains(out, ": "+tmuxControlLine+"\n") ||
					!strings.Contains(out, "\n  --- FAIL: "+tmuxTestName+" (") {
					t.Errorf("want the test's pass and its control run's fail quoted, and the script past every check up to go, got status %d:\n%s", status, out)
				}
				return
			}
			if status != 1 || !strings.Contains(out, checkPublicRan) || !strings.Contains(out, c.want) || strings.Contains(out, goReached) {
				t.Errorf("want a stop (%q) after check-public.sh and before any build, got status %d:\n%s", c.want, status, out)
			}
		})
	}
}

// The name, the control variable and its line the release gate needs are the ones M7/L4 are told to use, and each is said
// once, so a rename in one place shows.
func TestReleaseDesktopNamesTheBundledTmuxTestOnce(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "scripts", "release-gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"TMUX_TEST=" + tmuxTestName + "\n",
		"TMUX_TEST_PACKAGE=desktop\n",
		"TMUX_CONTROL=" + tmuxControl + "\n",
		`TMUX_CONTROL_LINE="` + tmuxControlLine + `"` + "\n",
	} {
		if n := strings.Count(string(script), line); n != 1 {
			t.Errorf("release-gate.sh has %q %d times, want once", line, n)
		}
	}
}
