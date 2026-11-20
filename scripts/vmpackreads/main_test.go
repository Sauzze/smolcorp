package main

import (
	"bytes"
	"go/build"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// standinTree is a repo of the test's own: a package internal/a whose tests read the repo's files through ../ in each
// way vmpackreads knows, and say words that only look like paths.
func standinTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                       "module example.com/standin\n\ngo 1.26\n",
		"Resources/shims/swift":        "#!/bin/sh\n",
		"Tests/fixtures/x.json":        "{}\n",
		"Tests/fixtures/linux.json":    "{}\n",
		"Tests/fixtures/mac.json":      "{}\n",
		"Sources/App.swift":            "// app\n",
		"scripts/tool.sh":              "#!/bin/sh\n",
		"RELEASE-NOTES.md":             "notes\n",
		"install.sh":                   "#!/bin/sh\n",
		"internal/a/testdata/own.json": "{}\n",
		// The package's own code: its ".." are a path's checks, never a read, and it isn't read.
		"internal/a/a.go": `package a

import "path/filepath"

func Inside(rel string) bool { return rel != ".." && filepath.Join("..", "..", "Resources", "nope") != "" }
`,
		"internal/a/a_test.go": `package a

import (
	"os"
	"path/filepath"
	"testing"
)

const repo = "../.."

func shims(name string) string { return filepath.Join("..", "..", "Resources", "shims", name) }

func TestShims(t *testing.T) { os.ReadFile(shims("swift")) }

func TestFixture(t *testing.T) { os.ReadFile("../../Tests/fixtures/x.json"); os.ReadFile("testdata/own.json") }

func TestWords(t *testing.T) {
	_ = filepath.Join("..", "outside")
	_ = []string{"../../dotfiles/rc", "../../../above", "../../Tests/fixtures/x.json\n", "../own.json"}
	_ = os.Getenv("X") == ".."
}

func TestMissing(t *testing.T) { os.ReadFile("../../Tests/fixtures/gone.json") }

func TestHeld(t *testing.T) {
	os.ReadFile(filepath.Join(repo, "scripts", "tool.sh"))
	name := os.Getenv("NAME")
	os.ReadFile(filepath.Join(repo, name))
}
`,
		// RT126VMPACK item 2: sums, and a folder above handed on, each read or stopped on; and ".." only said.
		"internal/a/sums_test.go":     sumsTest,
		"internal/a/a_linux_test.go":  "package a\n\nimport (\"os\"; \"testing\")\n\nfunc TestLinux(t *testing.T) { os.ReadFile(\"../../Tests/fixtures/linux.json\") }\n",
		"internal/a/a_darwin_test.go": "package a\n\nimport (\"os\"; \"testing\")\n\nfunc TestMac(t *testing.T) { os.ReadFile(\"../../Tests/fixtures/mac.json\") }\n",
	}
	for name, text := range files {
		at := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const sumsTest = `package a

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const notes = "../../" + "RELEASE-NOTES.md"

const up = "../.." // const climb

func TestSumConst(t *testing.T) { os.ReadFile(notes) }

func TestSumInPlace(t *testing.T) { os.ReadFile("../../" + "install.sh") }

func TestSumIntoAFolder(t *testing.T) { os.ReadFile("../../Resources/" + os.Getenv("NAME")) }

func TestSumOfAConstClimb(t *testing.T) { os.ReadFile(up + "/install.sh") }

func TestSumWithARunTimeWord(t *testing.T) { os.ReadFile("../../" + os.Getenv("NAME")) } // run-time word

func TestShortVar(t *testing.T) { root := "../.."; os.ReadFile(root + "/install.sh") } // short var

func TestAbs(t *testing.T) { root, _ := filepath.Abs(".."); os.ReadFile(root) } // abs

func TestAHelperWalks(t *testing.T) { walk(t, "..") } // helper walks

func walk(t *testing.T, root string) { filepath.WalkDir(filepath.Join(root, "Tests"), nil) }

func TestDotDotWords(t *testing.T) {
	said(t, "/x", "..")
	var p struct{ ID string }
	p.ID = ".."
	_ = map[string]bool{"..": true}
	_ = []string{"", "..", "../.."}
	_ = os.Symlink("..", filepath.Join(t.TempDir(), "l"))
	_ = strings.HasPrefix(p.ID, ".."+string(filepath.Separator))
}

func said(t *testing.T, path, why string) { t.Errorf("%s: %s", why, path) }

// RT126VMPACKR2 §2: a sum whose head names a folder below the climb reads that folder, wherever the sum is.

func TestSumInAList(t *testing.T) {
	for _, p := range []string{"../../Resources/" + os.Getenv("NAME")} {
		os.Stat(p)
	}
}

func TestSumInAStructRow(t *testing.T) {
	for _, c := range []struct{ path string }{{path: "../../Tests/" + os.Getenv("NAME")}} {
		os.Stat(c.path)
	}
}

type popts struct{ Dir string }

func popen(o popts) { os.ReadDir(o.Dir) }

func TestSumInAnOption(t *testing.T) { popen(popts{Dir: "../../scripts/" + os.Getenv("NAME")}) }

func TestSumInAField(t *testing.T) {
	var o popts
	o.Dir = "../../Sources/" + os.Getenv("NAME")
	popen(o)
}

// RT126VMPACKR2 §3: filepath.Join("..", "..") kept or handed on is held as "../.." is; only said, a word.

func TestJoinRead(t *testing.T) { os.ReadDir(filepath.Join("..", "..")) } // join read

func TestJoinKept(t *testing.T) {
	root := filepath.Join("..", "..") // join kept
	os.Stat(filepath.Join(root, "CONTRIBUTING.md"))
}

func TestJoinVar(t *testing.T) {
	var r = filepath.Join("..", "..") // join var
	os.Stat(r)
}

func TestJoinToAHelper(t *testing.T) { pwalk(t, filepath.Join("..", "..")) } // join helper

func pwalk(t *testing.T, root string) { filepath.WalkDir(root, nil) }

func TestJoinWalked(t *testing.T) { filepath.WalkDir(filepath.Join("..", ".."), nil) } // join walked

func TestJoinSaid(t *testing.T) {
	id := os.Getenv("ID")
	_ = id == filepath.Join("..", "..")
	_ = []string{"../" + id, filepath.Join("..", "..")}
	t.Logf("above: %s", filepath.Join("..", ".."))
}
`

func scanFor(goos, root string, listed []string, pkgs ...string) (int, string, string) {
	ctx := build.Default
	ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = goos, "arm64", false
	var out, errs bytes.Buffer
	status := run(ctx, root, listed, pkgs, &out, &errs)
	return status, out.String(), errs.String()
}

// Each way a test names a repo file through ../ is found, for the files that build for the pack's system only; a
// folder joined with a word known at run time is read whole; stand-in words and the package's own code aren't reads.
func TestItFindsWhatTheTestsThatRunReadThroughDotDot(t *testing.T) {
	root := standinTree(t)
	status, out, errs := scanFor("linux", root, nil, "internal/a=^(TestShims|TestFixture|TestWords|TestLinux)$")
	if status != 0 || errs != "" {
		t.Fatalf("status %d, errors %q", status, errs)
	}
	want := "internal/a Resources/shims\ninternal/a Tests/fixtures/linux.json\ninternal/a Tests/fixtures/x.json\n"
	if out != want {
		t.Errorf("linux: got\n%s\nwant\n%s", out, want)
	}
	status, out, errs = scanFor("darwin", root, nil, "internal/a=^(TestMac|TestLinux)$")
	if status != 0 || errs != "" || out != "internal/a Tests/fixtures/mac.json\n" {
		t.Errorf("darwin: status %d, %q, errors %q", status, out, errs)
	}
	// A test the pack doesn't run reads nothing it must have.
	if status, out, errs = scanFor("linux", root, nil, "internal/a=^TestFixture$"); status != 0 || out != "internal/a Tests/fixtures/x.json\n" {
		t.Errorf("only TestFixture: status %d, %q, errors %q", status, out, errs)
	}
}

// A test that reads a repo path the tree hasn't stops the pack, and so does one that holds the repo's top folder and
// names what it reads there at run time, unless vmpack.sh lists that package's reads by hand.
func TestItStopsOnAReadThePackCantGive(t *testing.T) {
	root := standinTree(t)
	status, out, errs := scanFor("linux", root, nil, "internal/a=^TestMissing$")
	if status != 1 || out != "" || !strings.Contains(errs, "internal/a reads Tests/fixtures/gone.json (internal/a/a_test.go:") ||
		!strings.Contains(errs, "which the tree doesn't have") {
		t.Errorf("a missing read: status %d, %q, errors %q", status, out, errs)
	}

	status, out, errs = scanFor("linux", root, nil, "internal/a=^TestHeld$")
	if status != 1 || strings.Count(errs, "names at run time what it reads above its folder") != 2 ||
		!strings.Contains(errs, "(internal/a/a_test.go:9)") || out != "internal/a scripts/tool.sh\n" {
		t.Errorf("the repo's top folder held: status %d, %q, errors %q", status, out, errs)
	}
	status, out, errs = scanFor("linux", root, []string{"internal/a"}, "internal/a=^TestHeld$")
	if status != 0 || errs != "" || out != "internal/a scripts/tool.sh\n" {
		t.Errorf("listed by hand: status %d, %q, errors %q", status, out, errs)
	}
}

// FXVMPACK's own case, on this repo: internal/sandbox's tests on Linux read the shims, which the VM pack lacked.
func TestTheSandboxsLinuxTestsReadTheShims(t *testing.T) {
	status, out, errs := scanFor("linux", filepath.Join("..", ".."), nil, "internal/sandbox")
	if status != 0 || !strings.Contains(out, "internal/sandbox Resources/shims\n") {
		t.Errorf("status %d, %q, errors %q", status, out, errs)
	}
}

// A sum is read whole: "../../" + "x", as a const or in place, is packed; "../../Resources/" + name packs the folder.
// A folder above kept or handed on stops it: a const climb, "../../" + name, a short var, filepath.Abs(".."), a test's
// helper that walks it (releaseFilesIn(t, "..")). ".." only said (a message, a field, a list, a map, a link's target,
// a prefix to compare with) is no read (RT126VMPACK item 2).
func TestASumOrAFolderAboveIsReadOrStopsNeverMissed(t *testing.T) {
	root := standinTree(t)
	status, out, errs := scanFor("linux", root, nil, "internal/a=^(TestSumConst|TestSumInPlace|TestSumIntoAFolder|TestDotDotWords)$")
	want := "internal/a RELEASE-NOTES.md\ninternal/a Resources\ninternal/a install.sh\n"
	if status != 0 || errs != "" || out != want {
		t.Errorf("read: status %d, got\n%s\nwant\n%s\nerrors %q", status, out, want, errs)
	}
	lineOf := func(mark string) string { return markedLine(t, mark) }
	for _, c := range []struct{ test, mark, out string }{
		{"TestSumOfAConstClimb", "const climb", "internal/a install.sh\n"},
		{"TestSumWithARunTimeWord", "run-time word", ""},
		{"TestShortVar", "short var", ""},
		{"TestAbs", "abs", ""},
		{"TestAHelperWalks", "helper walks", ""},
	} {
		status, out, errs := scanFor("linux", root, nil, "internal/a=^"+c.test+"$")
		if status != 1 || out != c.out || strings.Count(errs, "\n") != 1 ||
			!strings.Contains(errs, "names at run time what it reads above its folder "+lineOf(c.mark)) {
			t.Errorf("%s: status %d, %q, errors %q", c.test, status, out, errs)
		}
	}
}

// markedLine is where sumsTest's line ending in "// mark" is, as a stop names it.
func markedLine(t *testing.T, mark string) string {
	t.Helper()
	for i, line := range strings.Split(sumsTest, "\n") {
		if strings.HasSuffix(line, "// "+mark) {
			return "(internal/a/sums_test.go:" + strconv.Itoa(i+1) + ")"
		}
	}
	t.Fatalf("no line marked %q", mark)
	return ""
}

// A sum whose head names a folder below the climb ("../../Resources/" + name) packs that folder wherever it is: a
// list's row, a struct literal's field, an option struct handed on, a field's value. Only a climb-only head
// ("../" + id) in such a place is a word (RT126VMPACKR2 §2).
func TestASumIntoAFolderIsPackedWhereverItIs(t *testing.T) {
	root := standinTree(t)
	for _, c := range []struct{ test, out string }{
		{"TestSumInAList", "internal/a Resources\n"},
		{"TestSumInAStructRow", "internal/a Tests\n"},
		{"TestSumInAnOption", "internal/a scripts\n"},
		{"TestSumInAField", "internal/a Sources\n"},
	} {
		status, out, errs := scanFor("linux", root, nil, "internal/a=^"+c.test+"$")
		if status != 0 || errs != "" || out != c.out {
			t.Errorf("%s: status %d, got %q, want %q, errors %q", c.test, status, out, c.out, errs)
		}
	}
}

// filepath.Join("..", "..") kept or handed on stops the pack, as "../.." does: read in place, a short var, a var, a
// test's helper that walks it, filepath.WalkDir. Only said (compared, in a list, in a log) it is a word
// (RT126VMPACKR2 §3).
func TestAJoinedFolderAboveKeptOrHandedOnStops(t *testing.T) {
	root := standinTree(t)
	for _, c := range []struct{ test, mark string }{
		{"TestJoinRead", "join read"},
		{"TestJoinKept", "join kept"},
		{"TestJoinVar", "join var"},
		{"TestJoinToAHelper", "join helper"},
		{"TestJoinWalked", "join walked"},
	} {
		status, out, errs := scanFor("linux", root, nil, "internal/a=^"+c.test+"$")
		if status != 1 || out != "" || strings.Count(errs, "\n") != 1 ||
			!strings.Contains(errs, "names at run time what it reads above its folder "+markedLine(t, c.mark)) {
			t.Errorf("%s: status %d, %q, errors %q", c.test, status, out, errs)
		}
	}
	if status, out, errs := scanFor("linux", root, nil, "internal/a=^TestJoinSaid$"); status != 0 || out != "" || errs != "" {
		t.Errorf("only said: status %d, %q, errors %q", status, out, errs)
	}
}

// The VM's misses on tree f3c0ffda (shims-f3c0ffda, both sessions: "no scripts/release-gate.sh", "a package inside
// another isn't read"): the release gate's sweep and the tmux grep row walk the repo from "..", and stop the pack.
func TestTheRowsThatWalkTheRepoFromDotDotStopThePack(t *testing.T) {
	status, _, errs := scanFor("linux", filepath.Join("..", ".."), nil,
		"desktop=^(TestReleaseGateEveryShippingPathGoesThroughIt|TestTheCoreRunsTmuxOnlyThroughItsHelper)$")
	if status != 1 || !strings.Contains(errs, "(desktop/releasegate_test.go:") || !strings.Contains(errs, "(desktop/tmuxclient_test.go:") {
		t.Errorf("status %d, errors %q", status, errs)
	}
}
