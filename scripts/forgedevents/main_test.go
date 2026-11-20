package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// standinModule is a throwaway module of its own, with a package "desk" holding files (name → text), for check to
// read and run as a release would the desktop package. It never reads this repo's packages.
func standinModule(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go")
	}
	dir := t.TempDir()
	write := func(name, text string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.invalid/standin\n\ngo 1.22\n")
	write("desk/desk.go", "package desk\n")
	for name, text := range files {
		write(filepath.Join("desk", name), text)
	}
	// Its own module alone: no workspace of this repo's, no download.
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOPROXY", "off")
	return dir
}

// realGo is the go on PATH, for check to run.
func realGo(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go")
	}
	return path
}

// realTools are the go on PATH and the C compiler (on a Mac, with its SDK) the gate finds as the release does, for
// check to run, with go's own download folder for this account and a fresh folder for go's caches (goWork).
func realTools(t *testing.T) tools {
	t.Helper()
	cc, sdk, err := cCompiler()
	if err != nil {
		t.Skip(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	use := tools{goProgram: realGo(t), cc: cc, sdk: sdk, home: home, tmp: os.TempDir(), work: goWork(t)}
	if use.modules, err = goDownloads(use); err != nil {
		t.Fatal(err)
	}
	return use
}

// goWork is a fresh folder for go's build cache, module cache and GOPATH, as the release makes one at its start,
// removed after the test (go makes the modules it unpacks read-only).
func goWork(t *testing.T) string {
	t.Helper()
	work := t.TempDir()
	t.Cleanup(func() { removeFolders(work, io.Discard) })
	return work
}

func testFile(tag, pkg string, bodies map[string]string) string {
	var text strings.Builder
	if tag != "" {
		text.WriteString("//go:build " + tag + "\n\n")
	}
	text.WriteString("package " + pkg + "\n\nimport \"testing\"\n\n")
	for name, body := range bodies {
		text.WriteString("func " + name + "(t *testing.T) {\n\t" + body + "\n}\n\n")
	}
	return text.String()
}

var standinTable = []test{{"TestEverywhere", everywhere}, {"TestOnUnix", unix}, {"TestOutside", everywhere}}

// whole is a package that holds the stand-in table as it should be: one test in an untagged file, one in a file
// Windows doesn't build, one in the package's _test package.
func whole() map[string]string {
	return map[string]string{
		"a_test.go":      testFile("", "desk", map[string]string{"TestEverywhere": "", "TestNotInTheTable": `t.Fatal("run")`}),
		"b_unix_test.go": testFile("!windows", "desk", map[string]string{"TestOnUnix": ""}),
		"c_test.go":      testFile("", "desk_test", map[string]string{"TestOutside": ""}),
	}
}

func TestAWholeTableThatPassesLetsTheReleaseOn(t *testing.T) {
	dir := standinModule(t, whole())
	var out bytes.Buffer
	if err := check(dir, "./desk", realTools(t), standinTable, &out); err != nil {
		t.Fatalf("a whole table that passes: %v\n%s", err, out.String())
	}
}

// Each way the table can be broken stops the release: a test missing, renamed, left out of one system's build,
// failing, skipped, one whose name is only in a comment, or tests that pass in a go test that fails all the same.
func TestABrokenTableStopsTheRelease(t *testing.T) {
	for _, c := range []struct {
		name   string
		change func(files map[string]string)
		want   string
	}{
		{"a test removed", func(f map[string]string) {
			f["a_test.go"] = testFile("", "desk", map[string]string{"TestNotInTheTable": ""})
		}, "no TestEverywhere (darwin), no TestEverywhere (linux), no TestEverywhere (windows)"},
		{"a test renamed", func(f map[string]string) {
			f["a_test.go"] = testFile("", "desk", map[string]string{"TestEverywhereRenamed": ""})
		}, "no TestEverywhere (darwin)"},
		{"a test only in a comment", func(f map[string]string) {
			f["a_test.go"] = "package desk\n\n// func TestEverywhere(t *testing.T) {\n"
		}, "no TestEverywhere (darwin)"},
		{"a test in a /* */ block", func(f map[string]string) {
			f["a_test.go"] = "package desk\n\nimport \"testing\"\n\n/*\nfunc TestEverywhere(t *testing.T) {\n}\n*/\n\n" +
				"func TestNotInTheTable(t *testing.T) {}\n"
		}, "no TestEverywhere (darwin), no TestEverywhere (linux), no TestEverywhere (windows)"},
		{"a test in a string", func(f map[string]string) {
			f["a_test.go"] = "package desk\n\nvar notATest = `\nfunc TestEverywhere(t *testing.T) {\n}\n`\n"
		}, "no TestEverywhere (darwin)"},
		{"a method of the name", func(f map[string]string) {
			f["a_test.go"] = "package desk\n\nimport \"testing\"\n\ntype s struct{}\n\nfunc (s) TestEverywhere(t *testing.T) {}\n"
		}, "no TestEverywhere (darwin)"},
		{"a test Windows doesn't build", func(f map[string]string) {
			f["a_test.go"] = testFile("!windows", "desk", map[string]string{"TestEverywhere": ""})
		}, "no TestEverywhere (windows)"},
		{"a test Linux doesn't build", func(f map[string]string) {
			f["b_unix_test.go"] = testFile("darwin", "desk", map[string]string{"TestOnUnix": ""})
		}, "no TestOnUnix (linux)"},
		{"a test that fails", func(f map[string]string) {
			f["a_test.go"] = testFile("", "desk", map[string]string{"TestEverywhere": `t.Fatal("forged event let in")`})
		}, "TestEverywhere failed or was skipped"},
		{"a test that skips", func(f map[string]string) {
			f["c_test.go"] = testFile("", "desk_test", map[string]string{"TestOutside": `t.Skip("no curl")`})
		}, "TestOutside failed or was skipped"},
		{"a package that doesn't build", func(f map[string]string) {
			f["d_test.go"] = "package desk\n\nfunc broken( {\n"
		}, ""},
		// Every test of the table passes, and go test fails after them: a TestMain whose teardown fails.
		{"a go test that fails after the table passed", func(f map[string]string) {
			f["e_test.go"] = "package desk\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\n" +
				"func TestMain(m *testing.M) {\n\tm.Run()\n\tos.Exit(3)\n}\n"
		}, "go test of the forged-event table failed"},
	} {
		files := whole()
		c.change(files)
		dir := standinModule(t, files)
		var out bytes.Buffer
		err := check(dir, "./desk", realTools(t), standinTable, &out)
		if err == nil || c.want != "" && !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want a stop saying %q, got %v\n%s", c.name, c.want, err, out.String())
		}
	}
}

// The table names X10's and X11's tests as they are in the desktop package, each in every system's build it is for:
// checked here against this repo's own files, without running them.
func TestTheTableNamesTheDesktopPackagesTests(t *testing.T) {
	use := realTools(t)
	for _, system := range everywhere {
		found, err := testsIn(filepath.Join("..", ".."), "./desktop", use, system)
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range table {
			for _, s := range test.systems {
				if s == system && !found[test.name] {
					t.Errorf("%s isn't in the desktop package as %s builds it", test.name, system)
				}
			}
		}
	}
}
