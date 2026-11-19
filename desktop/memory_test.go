package desktop

import (
	"bufio"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// T126WINMEM (RT126SD1R2 N-B1): RunApp and RunServer are the two ways this program becomes the core (cmd/smolcorp),
// and their first statement is protectMemory's, failing closed. Read from app.go itself, so no later step can slip
// in front of it unseen.
func TestTheAppAndTheServerProtectTheirMemoryAsTheirFirstStep(t *testing.T) {
	requireProtectedFirst(t)
}

// The ordering seam: protectMemory is stood in for, and fails. RunApp and RunServer each call it while the support
// folder is still empty (no key, no pid, no instance lock), make nothing there after it, and tell the owner why
// nothing started.
func TestTheAppAndTheServerStartNothingWhenTheirMemoryCantBeKept(t *testing.T) {
	// Run with a protectMemory that slipped down the list, this test would start a real app: it isn't.
	requireProtectedFirst(t)
	for name, run := range map[string]func() int{"RunApp": RunApp, "RunServer": RunServer} {
		t.Run(name, func(t *testing.T) {
			isolate(t)
			protect, warn := protectMemory, warnOwner
			t.Cleanup(func() { protectMemory, warnOwner = protect, warn })
			calls := 0
			protectMemory = func() error {
				calls++
				if entries, _ := os.ReadDir(supportDir); len(entries) != 0 {
					t.Errorf("the support folder had %d entries before the memory was protected", len(entries))
				}
				return errors.New("fake: the access list was refused")
			}
			var told []string
			warnOwner = func(message string) { told = append(told, message) }

			if code := run(); code != 1 {
				t.Errorf("it went on, or ended as if all was well: %d", code)
			}
			if calls != 1 {
				t.Errorf("protectMemory ran %d times", calls)
			}
			if entries, _ := os.ReadDir(supportDir); len(entries) != 0 {
				t.Errorf("the support folder has %d entries: something ran past the failed step", len(entries))
			}
			if len(told) != 1 || !strings.HasPrefix(told[0], "Smolcorp didn't start: ") ||
				!strings.Contains(told[0], "(fake: the access list was refused)") || !strings.Contains(told[0], "No key was made or read.") {
				t.Errorf("the owner wasn't told plainly why: %q", told)
			}
		})
	}
}

// The first-step check itself (RT126WINMEM N2): it takes only the exact protect-or-refuse form, not a near miss.
func TestTheFirstStepCheckTakesOnlyTheProtectOrRefuseForm(t *testing.T) {
	for body, want := range map[string]bool{
		"if err := protectMemory(); err != nil { return refuseUnprotected(err) }":              true,
		"if err := protectMemory(); err == nil { return refuseUnprotected(err) }":              false,
		"if err := protectMemory(); other != nil { return refuseUnprotected(err) }":            false,
		"if err := protectMemory(); err != nil { return refuseUnprotected(nil) }":              false,
		"if err = protectMemory(); err != nil { return refuseUnprotected(err) }":               false,
		"if err := protectMemory(); err != nil { return 1 }":                                   false,
		"if err := protectMemory(); err != nil { return refuseUnprotected(err) } else { }":     false,
		"if err := other.protectMemory(); err != nil { return refuseUnprotected(err) }":        false,
		"recordPid(); if err := protectMemory(); err != nil { return refuseUnprotected(err) }": false,
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "row.go", "package p\nfunc f() int { "+body+"\nreturn 0 }", 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := protectsFirst(file.Decls[0].(*ast.FuncDecl).Body.List); got != want {
			t.Errorf("%s: taken %v, want %v", body, got, want)
		}
	}
}

// requireProtectedFirst stops the test unless RunApp's and RunServer's first statement is
// `if err := protectMemory(); err != nil { return refuseUnprotected(err) }`.
func requireProtectedFirst(t *testing.T) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "app.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	first := map[string]bool{"RunApp": false, "RunServer": false}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil {
			if _, wanted := first[fn.Name.Name]; wanted {
				// Its first statement can't be preceded by a local of the same name: only a parameter or a named
				// result could shadow the package's protectMemory or refuseUnprotected, and neither function has one.
				first[fn.Name.Name] = fn.Type.Params.NumFields() == 0 && fn.Type.Results.NumFields() == 1 &&
					len(fn.Type.Results.List[0].Names) == 0 && protectsFirst(fn.Body.List)
			}
		}
	}
	for name, protects := range first {
		if !protects {
			t.Fatalf("%s's first statement no longer protects its memory and fails closed (app.go)", name)
		}
	}
}

func protectsFirst(body []ast.Stmt) bool {
	if len(body) == 0 {
		return false
	}
	step, ok := body[0].(*ast.IfStmt)
	if !ok || len(step.Body.List) != 1 || step.Else != nil {
		return false
	}
	init, ok := step.Init.(*ast.AssignStmt)
	if !ok || init.Tok != token.DEFINE || len(init.Lhs) != 1 || len(init.Rhs) != 1 || !callsNamed(init.Rhs[0], "protectMemory") {
		return false
	}
	failed, ok := init.Lhs[0].(*ast.Ident)
	if !ok {
		return false
	}
	// The condition is exactly `err != nil`, on the error the step returned.
	condition, ok := step.Cond.(*ast.BinaryExpr)
	if !ok || condition.Op != token.NEQ || !isIdent(condition.X, failed.Name) || !isIdent(condition.Y, "nil") {
		return false
	}
	refuse, ok := step.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(refuse.Results) != 1 || !callsNamed(refuse.Results[0], "refuseUnprotected") {
		return false
	}
	told := refuse.Results[0].(*ast.CallExpr).Args
	return len(told) == 1 && isIdent(told[0], failed.Name)
}

func callsNamed(expr ast.Expr, name string) bool {
	call, ok := expr.(*ast.CallExpr)
	return ok && isIdent(call.Fun, name)
}

func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

// memoryStandInVariable makes this test program a stand-in for Smolcorp (TestMain): which part it plays.
const memoryStandInVariable = "SMOLCORP_MEMORY_STANDIN"

// standInWindow opens a window like the office's, WebView2's, and returns once its page has run: Windows' only
// (memory_windows_test.go).
var standInWindow = func() error { return errors.New("no office window here") }

// memoryStandIn is this test program run as Smolcorp, as far as the end of RunApp's first step (RunServer's for
// "serve"): "app" and "serve" with the real protectMemory, "open" with none, as the rows' control. "app-window" and
// "open-window" also open a window like the office's after it. It says "ready" (or why not), and waits for its input
// to end. Then the step fails on purpose, so nothing past it runs: no key, no lock, no server.
func memoryStandIn(part string) int {
	protect := protectMemory
	if strings.HasPrefix(part, "open") {
		protect = func() error { return nil }
	}
	warnOwner = func(string) {}
	var failed error
	protectMemory = func() error {
		if failed = protect(); failed == nil && strings.HasSuffix(part, "-window") {
			failed = standInWindow()
		}
		if failed != nil {
			os.Stdout.WriteString("failed: " + failed.Error() + "\n")
			return failed
		}
		os.Stdout.WriteString("ready\n")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		return errors.New("the stand-in's run is over")
	}
	run := RunApp
	if part == "serve" {
		run = RunServer
	}
	run()
	if failed != nil {
		return 1
	}
	return 0
}

// memoryStandInAnswer bounds a stand-in's first answer: a WebView2 that never opens must not hold the test until
// go test's own timeout, which would fail the whole package.
var memoryStandInAnswer = 90 * time.Second

// RT126WINMEM S1: a stand-in that never answers, as with a WebView2 that never opens, is stopped when its time is
// up, and the answer says so, so neither the row nor go test waits for it.
func TestAStandInThatNeverAnswersIsStoppedInTime(t *testing.T) {
	limit := memoryStandInAnswer
	t.Cleanup(func() { memoryStandInAnswer = limit })
	memoryStandInAnswer = 2 * time.Second
	began := time.Now()
	var pid int
	var answer string
	t.Run("silent", func(t *testing.T) { pid, answer = startMemoryStandIn(t, "silent") }) // its cleanup ends it
	if took := time.Since(began); took > 20*time.Second {
		t.Errorf("the stand-in held the row %v", took)
	}
	if answer != "no answer within 2s: stopped" {
		t.Errorf("the answer doesn't say it timed out: %q", answer)
	}
	if process, err := os.FindProcess(pid); err == nil && process.Signal(syscall.Signal(0)) == nil {
		t.Errorf("the stand-in (pid %d) still runs", pid)
	}
}

// startMemoryStandIn starts a stand-in playing part, and waits for its first answer: its pid, and "ready" or why
// not. A stand-in that doesn't answer in time is stopped, and the answer says so. It ends with the test.
func startMemoryStandIn(t *testing.T, part string) (int, string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), memoryStandInVariable+"="+part)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	ended := make(chan struct{})
	t.Cleanup(func() {
		input.Close()
		select {
		case <-ended:
		case <-time.After(30 * time.Second):
			_ = cmd.Process.Kill()
			select {
			case <-ended:
			case <-time.After(10 * time.Second):
				t.Logf("the %s stand-in (pid %d) was stopped but hasn't ended", part, cmd.Process.Pid)
			}
		}
	})
	answers := make(chan string, 1)
	go func() {
		line, err := bufio.NewReader(output).ReadString('\n')
		if err != nil && line == "" {
			line = "no answer: " + err.Error()
		}
		answers <- strings.TrimSpace(line)
		_ = cmd.Wait()
		close(ended)
	}()
	select {
	case answer := <-answers:
		return cmd.Process.Pid, answer
	case <-time.After(memoryStandInAnswer):
		_ = cmd.Process.Kill()
		return cmd.Process.Pid, fmt.Sprintf("no answer within %v: stopped", memoryStandInAnswer)
	}
}
