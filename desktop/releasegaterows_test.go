//go:build unix

package desktop

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
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
)

// The release gate (scripts/release-gate.sh, which every release script sources) runs this package's TestRelease
// tests before every release, with SMOLCORP_RELEASE_GATE_ROWS=1 (its gate 5). Only then do the heavy ones run: each of
// their rows is a dry run of the release, a second or more for the go and cc it pins at its start, and the bundled
// tmux's rows build and run its test twice, which together would take an ordinary go test of ./desktop past go's 10
// minutes. Everywhere else they skip, and the fast rows here check that the variable still does both.

const (
	releaseGateRows    = "SMOLCORP_RELEASE_GATE_ROWS"
	releaseGateRowsRun = "^TestRelease"

	stopRows        = "the release's own tests didn't all pass in ./desktop"
	stopRowsChanged = "the files changed while the release's own tests ran"
)

// heavyReleaseRows skips a test of heavy rows unless the release gate runs it. It is that test's first statement, and
// each of the test's rows is a subtest, so a fast row can run the test with none of them.
func heavyReleaseRows(t *testing.T) {
	t.Helper()
	if os.Getenv(releaseGateRows) != "1" {
		t.Skip("heavy rows: the release gate (scripts/release-gate.sh) runs them before every release, with " + releaseGateRows + "=1")
	}
}

// heavyTests is what this package's test files say about heavyReleaseRows: the tests that call it, and those whose
// first statement it isn't.
type heavyTests struct {
	names, misplaced []string
}

var readHeavyTests = sync.OnceValues(func() (heavyTests, error) {
	var found heavyTests
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		return found, err
	}
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
		if err != nil {
			return found, err
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv != nil {
				continue
			}
			calls := 0
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "heavyReleaseRows" {
						calls++
					}
				}
				return true
			})
			if calls == 0 {
				continue
			}
			found.names = append(found.names, fn.Name.Name)
			first := false
			if len(fn.Body.List) > 0 {
				if stmt, ok := fn.Body.List[0].(*ast.ExprStmt); ok {
					if call, ok := stmt.X.(*ast.CallExpr); ok {
						id, ok := call.Fun.(*ast.Ident)
						first = ok && id.Name == "heavyReleaseRows"
					}
				}
			}
			if calls != 1 || !first || !strings.HasPrefix(fn.Name.Name, "Test") {
				found.misplaced = append(found.misplaced, fn.Name.Name)
			}
		}
	}
	slices.Sort(found.names)
	return found, nil
})

func heavyReleaseTests(t *testing.T) []string {
	t.Helper()
	found, err := readHeavyTests()
	if err != nil {
		t.Fatalf("reading this package's tests for heavyReleaseRows: %v", err)
	}
	return found.names
}

// The heavy tests skip without the variable and run with it, the script names the variable, the tests it runs and the
// heavy ones as this package has them, and says so at its top. A fast row: a heavy test is run here as this test
// program again, with none of its rows.
func TestReleaseDesktopRunsItsHeavyRowsOnlyForTheGate(t *testing.T) {
	found, err := readHeavyTests()
	if err != nil {
		t.Fatal(err)
	}
	if len(found.misplaced) > 0 {
		t.Errorf("heavyReleaseRows must be called once, as the first statement of a test: %v", found.misplaced)
	}
	if !slices.Contains(found.names, "TestReleaseDesktopNeedsTheBundledTmuxTestPassing") {
		t.Errorf("the bundled tmux's rows aren't heavy rows: heavy tests %v", found.names)
	}

	script, err := os.ReadFile(filepath.Join("..", "scripts", "release-gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"RELEASE_ROWS=" + releaseGateRows + "=1\n",
		"RELEASE_ROWS_RUN=" + releaseGateRowsRun + "\n",
	} {
		if n := strings.Count(string(script), line); n != 1 {
			t.Errorf("release-gate.sh has %q %d times, want once", line, n)
		}
	}
	heavy := regexp.MustCompile(`(?m)^RELEASE_ROWS_HEAVY=\(([^)]*)\)$`).FindAllStringSubmatch(string(script), -1)
	if len(heavy) != 1 {
		t.Fatalf("release-gate.sh has RELEASE_ROWS_HEAVY=(…) %d times, want once", len(heavy))
	}
	named := strings.Fields(heavy[0][1])
	slices.Sort(named)
	if !slices.Equal(named, found.names) {
		t.Errorf("release-gate.sh's heavy tests are %v, this package's are %v", named, found.names)
	}
	top, _, _ := strings.Cut(string(script), "\nset -euo pipefail\n")
	if !strings.Contains(top, "#      "+releaseGateRows+"=1.") {
		t.Errorf("release-gate.sh's header doesn't name %s=1", releaseGateRows)
	}
	run := regexp.MustCompile(releaseGateRowsRun)
	for _, name := range found.names {
		if !run.MatchString(name) {
			t.Errorf("the script's run of its own tests (-run %s) leaves out %s", releaseGateRowsRun, name)
		}
	}

	// The heavy tests, run as this test program again: skipped without the variable or with another value, run
	// (with none of their rows) with it.
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, releaseGateRows+"=") {
			env = append(env, kv)
		}
	}
	for _, c := range []struct{ setting, want string }{
		{"", "SKIP"},
		{releaseGateRows + "=0", "SKIP"},
		{releaseGateRows + "=true", "SKIP"},
		{releaseGateRows + "=1", "PASS"},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^("+strings.Join(found.names, "|")+")$/^$", "-test.v", "-test.count=1")
		cmd.Env = env
		if c.setting != "" {
			cmd.Env = append(slices.Clip(env), c.setting)
		}
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil || strings.Contains(string(out), "--- FAIL") || strings.Count(string(out), "\n--- ") != len(found.names) {
			t.Errorf("with %q: want each heavy test to %s, got %v:\n%s", c.setting, c.want, err, out)
			continue
		}
		var missed []string
		for _, name := range found.names {
			if !strings.Contains(string(out), "\n--- "+c.want+": "+name+" (") {
				missed = append(missed, name)
			}
		}
		if len(missed) > 0 {
			t.Errorf("with %q: want each heavy test to %s, these didn't: %v\n%s", c.setting, c.want, missed, out)
		}
	}
}

// The script's run of its own tests, each case in a repo of its own whose stand-in go answers it (bin/rows-out and
// bin/rows-status, or by default every heavy test passing) and logs each go test it is given.
func TestReleaseDesktopRunsItsOwnTestsWithTheVariable(t *testing.T) {
	heavy := heavyReleaseTests(t)
	if len(heavy) == 0 {
		t.Fatal("no heavy tests in this package")
	}
	passes := func(lines ...string) string {
		out := ""
		for _, name := range heavy {
			out += "=== RUN   " + name + "\n--- PASS: " + name + " (0.00s)\n"
		}
		return out + strings.Join(lines, "\n") + "\n"
	}
	answer := func(out string, status int) func(r *releaseRepo) []string {
		return func(r *releaseRepo) []string {
			writeReleaseFile(r.t, filepath.Join(r.bin, "rows-out"), out, 0o644)
			writeReleaseFile(r.t, filepath.Join(r.bin, "rows-status"), strconv.Itoa(status)+"\n", 0o644)
			return nil
		}
	}
	ok := "PASS\nok  \texample.invalid/release/desktop\t0.01s"
	caller := func(setting string) func(r *releaseRepo) []string {
		return func(r *releaseRepo) []string { return []string{setting} }
	}
	for _, c := range []releaseCase{
		{"its own tests passing", "", nil},
		{"its own tests passing, the caller's " + releaseGateRows + "=0", "", caller(releaseGateRows + "=0")},
		{"its own tests passing, the caller's " + releaseGateRows + "=1", "", caller(releaseGateRows + "=1")},
		{"its own tests passing, a folder with no go first on the caller's PATH", "", func(r *releaseRepo) []string {
			return []string{"PATH=" + r.t.TempDir() + ":" + strings.TrimPrefix(r.env[0], "PATH=")}
		}},
		{"a heavy test skipped", stopRows, answer(strings.ReplaceAll(passes(ok), "--- PASS: ", "--- SKIP: "), 0)},
		{"another of its tests skipped", stopRows, answer(passes("--- SKIP: TestReleaseDesktopOther (0.00s)", ok), 0)},
		{"no test run", stopRows, answer("testing: warning: no tests to run\n"+ok+"\n", 0)},
		{"another of its tests failing", stopRows, answer(passes("--- FAIL: TestReleaseDesktopOther (0.00s)",
			"FAIL\nFAIL\texample.invalid/release/desktop\t0.01s\nFAIL"), 1)},
		{"a row of a heavy test failing, go test exiting 0", stopRows,
			answer(passes("    --- FAIL: "+heavy[0]+"/a_row (0.00s)", ok), 0)},
		{"every heavy test passing, go test exiting 1", stopRows, answer(passes(ok), 1)},
		{"a heavy test's pass only inside a longer line", stopRows,
			answer(strings.ReplaceAll(passes(ok), "\n--- PASS: ", "\n    --- PASS: "), 0)},
		{"a file left beside the code while they ran", stopRowsChanged, func(r *releaseRepo) []string {
			writeReleaseFile(r.t, filepath.Join(r.bin, "rows-stray"), "", 0o644)
			return nil
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newReleaseRepo(t)
			r.squashed()
			var extra []string
			if c.setup != nil {
				extra = c.setup(r)
			}
			status, out := r.release(extra...)
			if c.want != "" {
				if status != 1 || !strings.Contains(out, checkPublicRan) || !strings.Contains(out, c.want) || strings.Contains(out, goReached) {
					t.Errorf("want a stop (%q) after check-public.sh and before any build, got status %d:\n%s", c.want, status, out)
				}
				return
			}
			if status != 97 || !strings.Contains(out, goReached) || strings.Contains(out, "Stopped: ") {
				t.Errorf("want it past every check up to go, got status %d:\n%s", status, out)
			}
			// One run of its own tests, with the variable, the pinned go first on PATH; the bundled tmux's test run
			// twice without it.
			log, err := os.ReadFile(filepath.Join(r.bin, "go-test.log"))
			if err != nil {
				t.Fatal(err)
			}
			bin, err := filepath.EvalSymlinks(r.bin)
			if err != nil {
				t.Fatal(err)
			}
			var rows, tmux []string
			for _, line := range strings.Split(strings.TrimSpace(string(log)), "\n") {
				if strings.HasPrefix(line, "rows=1 ") {
					rows = append(rows, line)
				} else if strings.HasPrefix(line, "rows=unset ") && strings.Contains(line, " -run ^"+tmuxTestName+"$ ") {
					tmux = append(tmux, line)
				} else {
					t.Errorf("a go test it shouldn't have run: %s", line)
				}
			}
			wantRows := "rows=1 path=" + bin + " test -count=1 -parallel 8 -v -run " + releaseGateRowsRun + " ./desktop"
			if len(rows) != 1 || rows[0] != wantRows || len(tmux) != 2 {
				t.Errorf("want one run of its own tests (%q) and two of the bundled tmux's test, got:\n%s", wantRows, log)
			}
		})
	}
}
