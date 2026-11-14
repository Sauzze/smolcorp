package desktop

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// SANDBOX-PLAN §3.5 (ST1): every program the core starts on a Mac holds none of Smolcorp's grants. It goes through
// disclaim.Cmd, which has smolcorp-spawn start it with responsibility disclaimed. This reads the core's code as a Mac
// build compiles it, and fails on any exec.Command, exec.CommandContext, os.StartProcess or syscall exec that isn't
// handed straight to disclaim.Cmd, an exec.Cmd literal with a Path, and a pseudo-terminal's Command or
// CommandContext (go-pty, creack/pty) whose program and words aren't disclaim.Words's, unless it is on the lists
// below. A dot import of a starter's package, and a call to disclaim.StandIn, fail it too.
//
// Starts inside the libraries a Mac build compiles aren't read here: go-pty's are caught at its Command, above.
// zalando/go-keyring runs /usr/bin/security, but only from files built off a Mac. godbus/dbus runs launchctl on a
// session bus connect, which only trayhost_linux.go makes.

// spawnExemptPrograms are Apple's programs the core may start as Smolcorp, by their full path: protected by the
// system, and running no code they're pointed at. Music, Terminal and Tiler need Automation and Accessibility as
// Smolcorp; ps and defaults only read. /usr/bin/git is NOT one of them: it runs the developer tools' git, which runs
// whatever a repo's config names. Nor is osascript, here: only the app runs it, with its own fixed scripts (ST1s).
var spawnExemptPrograms = map[string]string{
	"/bin/ps":           "reads a process's parent and terminal (hook_mac.go)",
	"/usr/bin/open":     "opens a page in the browser, or Smolcorp.app by its bundle id (network.go)",
	"/usr/bin/defaults": "reads the Mac's clock settings (macclock.go)",
}

// spawnExemptFunctions are the starts that are Smolcorp itself, or run only where nothing is charged to Smolcorp.app,
// by file and function. Each covers its whole function: a start added inside one passes unseen, so each says so where
// it starts.
var spawnExemptFunctions = map[string]string{
	"desktop/restart.go startProgramWith":    "Smolcorp itself (a restart, as this program) or /usr/bin/open -b for its own app (openMacApp)",
	"desktop/restart_unix.go replaceProcess": "Smolcorp becoming itself again, in place, for a restart",
	"desktop/deskstart.go RunDeskStart":      "a desk's helper becoming its agent in place, inside the desk's tmux, itself started through disclaim.Cmd",
	"desktop/launcher.go launch": "smolcorp run starting its agent in a pseudo-terminal: it runs only in a hire's terminal window (Terminal, " +
		"iTerm or Ghostty) or a desk's tmux, itself started through disclaim.Cmd, so the agent is charged to that terminal or tmux, never to the core",
	"internal/gm/login_unix.go execGM": "smolcorp gm login putting the General Manager in its own place (SK2, B3): an exec in place starts no " +
		"process, so the GM inherits the charge of whatever ran the command (the owner's terminal, or a hire's), and it's never the core's: " +
		"only the command line runs it",
}

// spawnSite is one start found.
type spawnSite struct {
	where    string // file:line
	function string // the enclosing function, or the variable a function literal is
	call     string // e.g. exec.CommandContext
	pkg      string // the starter's package path
	program  string // the program, when it's a string literal
	wrapped  bool   // handed straight to disclaim.Cmd, or a pseudo-terminal's given disclaim.Words's program and words
	never    string // why it fails whatever the lists say: a dot import, a call to disclaim.StandIn
}

// starters are what start a program, by package path.
var starters = map[string][]string{
	"os/exec": {"Command", "CommandContext"},
	"os":      {"StartProcess"},
	"syscall": {"Exec", "ForkExec", "StartProcess"},
}

// ptyPackages are the pseudo-terminal libraries whose Command and CommandContext start a program.
var ptyPackages = []string{"github.com/aymanbagabas/go-pty", "github.com/creack/pty"}

// spawnSites reads one Go file and gives every start in it.
func spawnSites(fset *token.FileSet, name string, file *ast.File) []spawnSite {
	local := map[string]string{} // the name a starter's package goes by in this file -> its path
	disclaimName, execName, hasPty := "", "", false
	var sites []spawnSite
	at := func(node ast.Node) string { return name + ":" + strconv.Itoa(fset.Position(node.Pos()).Line) }
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		as := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			as = spec.Name.Name
		}
		if _, ok := starters[path]; ok {
			local[as] = path
			if as == "." {
				// Its starters would be called by their bare names, which this doesn't follow.
				sites = append(sites, spawnSite{where: at(spec), function: "its imports", call: `import . "` + path + `"`, pkg: path,
					never: "a dot import of a package that starts programs: import it by name"})
			}
		}
		switch {
		case path == "github.com/Sauzze/smolcorp/internal/disclaim":
			disclaimName = as
		case path == "os/exec":
			execName = as
		case slices.Contains(ptyPackages, path):
			hasPty = true
		}
	}
	var stack []ast.Node
	enclosing := func(site *spawnSite) *ast.FuncDecl {
		var fn *ast.FuncDecl
		for i := len(stack) - 2; i >= 0; i-- {
			switch outer := stack[i].(type) {
			case *ast.FuncDecl:
				if site.function == "" {
					site.function = outer.Name.Name
				}
				fn = outer
			case *ast.ValueSpec:
				if site.function == "" {
					site.function = outer.Names[0].Name
				}
			}
		}
		return fn
	}
	// handedToCmd is whether the node at stack[i] is disclaim.Cmd's one argument.
	handedToCmd := func(i int) bool {
		if i < 1 || disclaimName == "" {
			return false
		}
		outer, ok := stack[i-1].(*ast.CallExpr)
		if !ok || len(outer.Args) != 1 || outer.Args[0] != stack[i] {
			return false
		}
		fun, ok := outer.Fun.(*ast.SelectorExpr)
		if !ok || fun.Sel.Name != "Cmd" {
			return false
		}
		id, ok := fun.X.(*ast.Ident)
		return ok && id.Name == disclaimName
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, node)
		// An exec.Cmd written out with a Path is there to be started: only as disclaim.Cmd's argument.
		if lit, ok := node.(*ast.CompositeLit); ok && execName != "" && isSelector(lit.Type, execName, "Cmd") && hasKey(lit, "Path") {
			site := spawnSite{where: at(lit), call: execName + ".Cmd{Path}", pkg: "os/exec"}
			enclosing(&site)
			i := len(stack) - 1
			if i >= 1 {
				if amp, ok := stack[i-1].(*ast.UnaryExpr); ok && amp.Op == token.AND {
					i--
				}
			}
			site.wrapped = handedToCmd(i)
			sites = append(sites, site)
			return true
		}
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && disclaimName != "" && id.Name == disclaimName && sel.Sel.Name == "StandIn" {
			site := spawnSite{where: at(sel), call: id.Name + ".StandIn", never: "disclaim.StandIn is for tests: it can turn disclaiming off"}
			enclosing(&site)
			sites = append(sites, site)
			return true
		}
		// A pseudo-terminal's Command or CommandContext, called: its program and words must be disclaim.Words's.
		if hasPty && (sel.Sel.Name == "Command" || sel.Sel.Name == "CommandContext") && len(stack) >= 2 {
			if id, ok := sel.X.(*ast.Ident); !ok || local[id.Name] == "" {
				if call, ok := stack[len(stack)-2].(*ast.CallExpr); ok && call.Fun == sel {
					site := spawnSite{where: at(sel), call: exprName(sel.X) + "." + sel.Sel.Name, pkg: "pty"}
					fn := enclosing(&site)
					site.wrapped = fromWords(fn, call, sel.Sel.Name == "CommandContext", disclaimName)
					sites = append(sites, site)
					return true
				}
			}
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || local[pkg.Name] == "" || !slices.Contains(starters[local[pkg.Name]], sel.Sel.Name) {
			return true
		}
		site := spawnSite{where: at(sel), call: pkg.Name + "." + sel.Sel.Name, pkg: local[pkg.Name]}
		enclosing(&site)
		// Called here (not only named), with its program as a literal, and that call handed to disclaim.Cmd.
		if len(stack) >= 2 {
			if call, ok := stack[len(stack)-2].(*ast.CallExpr); ok && call.Fun == sel {
				programAt := 0
				if sel.Sel.Name == "CommandContext" {
					programAt = 1
				}
				if len(call.Args) > programAt {
					if lit, ok := call.Args[programAt].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						site.program, _ = strconv.Unquote(lit.Value)
					}
				}
				site.wrapped = handedToCmd(len(stack) - 2)
			}
		}
		sites = append(sites, site)
		return true
	})
	return sites
}

// fromWords is whether a pseudo-terminal's call is given, as its program and words, the first two of what
// disclaim.Words gave in the same function (started, words, err := disclaim.Words(…); ….Command(started, words...)),
// with neither name set anywhere else in it.
func fromWords(fn *ast.FuncDecl, call *ast.CallExpr, withContext bool, disclaimName string) bool {
	programAt := 0
	if withContext {
		programAt = 1
	}
	if fn == nil || disclaimName == "" || len(call.Args) != programAt+2 || !call.Ellipsis.IsValid() {
		return false
	}
	program, ok1 := call.Args[programAt].(*ast.Ident)
	words, ok2 := call.Args[programAt+1].(*ast.Ident)
	if !ok1 || !ok2 {
		return false
	}
	fromHelper, setElsewhere := false, false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		names := map[string]bool{}
		for _, left := range assign.Lhs {
			if id, ok := left.(*ast.Ident); ok {
				names[id.Name] = true
			}
		}
		if !names[program.Name] && !names[words.Name] {
			return true
		}
		if len(assign.Lhs) == 3 && len(assign.Rhs) == 1 && exprName(assign.Lhs[0]) == program.Name && exprName(assign.Lhs[1]) == words.Name {
			if helper, ok := assign.Rhs[0].(*ast.CallExpr); ok && isSelector(helper.Fun, disclaimName, "Words") && !fromHelper {
				fromHelper = true
				return true
			}
		}
		setElsewhere = true
		return true
	})
	return fromHelper && !setElsewhere
}

// isSelector is whether expr is pkg.name.
func isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

// hasKey is whether a composite literal sets the field named.
func hasKey(lit *ast.CompositeLit, field string) bool {
	for _, element := range lit.Elts {
		if kv, ok := element.(*ast.KeyValueExpr); ok && exprName(kv.Key) == field {
			return true
		}
	}
	return false
}

// exprName is an identifier's name, or "…" for anything else.
func exprName(expr ast.Expr) string {
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return "…"
}

// spawnViolation is why site breaks the rule, or "".
func spawnViolation(name string, site spawnSite) string {
	switch {
	case site.never != "":
		return site.where + ": " + site.call + " in " + site.function + ": " + site.never
	case site.wrapped:
		return ""
	case site.program != "" && spawnExemptPrograms[site.program] != "" && site.pkg == "os/exec":
		return ""
	case spawnExemptFunctions[name+" "+site.function] != "":
		return ""
	}
	return site.where + ": " + site.call + " in " + site.function + " starts a program charged to Smolcorp: hand it to disclaim.Cmd (a pseudo-terminal's, disclaim.Words's program and words), or add it to the exempt list with why"
}

func TestEveryProgramStartsDisclaimedOnAMac(t *testing.T) {
	mac := build.Default
	mac.GOOS, mac.GOARCH, mac.CgoEnabled = "darwin", "arm64", true
	fset := token.NewFileSet()
	var problems []string
	seen := map[string]bool{}
	files := 0
	// The core's code: the command, desktop and internal, and the module's own files. scripts/ holds tools a
	// developer runs, never started by Smolcorp; third_party isn't Smolcorp's.
	for _, root := range []string{"../cmd", "../desktop", "../internal", ".."} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if root == ".." && path != ".." {
					return filepath.SkipDir
				}
				if base := entry.Name(); path != root && (base == "testdata" || strings.HasPrefix(base, ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if match, err := mac.MatchFile(filepath.Dir(path), entry.Name()); err != nil || !match {
				return err
			}
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			files++
			name := strings.TrimPrefix(filepath.ToSlash(path), "../")
			for _, site := range spawnSites(fset, name, file) {
				seen[name+" "+site.function] = true
				if site.wrapped {
					seen["wrapped "+name+" "+site.call] = true
				}
				if site.program != "" {
					seen["program "+site.program] = true
				}
				if problem := spawnViolation(name, site); problem != "" {
					problems = append(problems, problem)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if files < 100 {
		t.Fatalf("only %d files read: the walk is looking in the wrong place", files)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
	// The list holds only what's still there, so it says what Smolcorp starts as itself today.
	for key := range spawnExemptFunctions {
		if !seen[key] {
			t.Errorf("%s is on the exempt list but starts nothing now: take it off", key)
		}
	}
	for program := range spawnExemptPrograms {
		if !seen["program "+program] {
			t.Errorf("%s is on the exempt list but isn't started now: take it off", program)
		}
	}
	// The context form is read too, where the plan names it.
	for _, site := range []string{"desktop/hostphonemic.go exec.CommandContext", "desktop/tabs.go exec.CommandContext",
		"desktop/hiresettings.go exec.CommandContext", "desktop/gitstate.go exec.CommandContext",
		"desktop/opencode.go exec.CommandContext", "internal/acp/client.go exec.Command", "desktop/signinrun.go terminal.Command"} {
		if !seen["wrapped "+site] {
			t.Errorf("%s isn't handed to disclaim.Cmd (or given disclaim.Words's)", site)
		}
	}
}

// The reader itself: each way of starting a program outside disclaim.Cmd is caught, a renamed import included,
// and an exempt program counts only by its full path.
func TestTheSpawnCheckCatchesEachWayToStartAProgram(t *testing.T) {
	source := `package x

import (
	"context"
	"os"
	run "os/exec"
	"syscall"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

var later = run.Command

func f(ctx context.Context) {
	_ = disclaim.Cmd(run.CommandContext(ctx, "git", "status"))
	_ = run.CommandContext(ctx, "git", "status")
	_ = run.Command("git")
	_ = run.Command("/bin/ps", "-p", "1")
	_ = run.Command("ps")
	cmd := run.Command("tmux")
	_ = disclaim.Cmd(cmd)
	_, _ = os.StartProcess("/bin/sh", nil, nil)
	_ = syscall.Exec("/bin/sh", nil, nil)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	var caught []string
	for _, site := range spawnSites(fset, "x.go", file) {
		if spawnViolation("x.go", site) != "" {
			caught = append(caught, site.where)
		}
	}
	want := []string{"x.go:12", "x.go:16", "x.go:17", "x.go:19", "x.go:20", "x.go:22", "x.go:23"}
	if !slices.Equal(caught, want) {
		t.Fatalf("caught %v, want %v", caught, want)
	}
}

// The forms RT126ST1 fed the reader (a written-out exec.Cmd, a dot import, a pseudo-terminal's start), one source
// each, with the lines it must catch: each was missed before.
func TestTheSpawnCheckCatchesStartsOutsideExecCommand(t *testing.T) {
	for _, c := range []struct {
		name, source string
		caught       []string
	}{
		{"exec.Command, the control", `package x

import "os/exec"

func f() { _ = exec.Command("sh") }
`, []string{"x.go:5"}},
		{"an exec.Cmd with a Path, started", `package x

import (
	"os/exec"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

func f() {
	_ = (&exec.Cmd{Path: "/bin/sh", Args: []string{"sh"}}).Start()
	cmd := exec.Cmd{Path: "/bin/sh"}
	_ = cmd.Start()
	_ = disclaim.Cmd(&exec.Cmd{Path: "/bin/sh"}).Start()
	_ = &exec.Cmd{Process: nil}
}
`, []string{"x.go:10", "x.go:11"}},
		{"a dot import", `package x

import . "os/exec"

func f() { _ = Command("sh") }
`, []string{"x.go:3"}},
		{"go-pty's Command, as it is and through disclaim.Words", `package x

import (
	"context"

	"github.com/aymanbagabas/go-pty"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

func plain(terminal pty.Pty) { _ = terminal.Command("codex", "login") }

func withContext(ctx context.Context, terminal pty.Pty) { _ = terminal.CommandContext(ctx, "codex", "login") }

func wrapped(terminal pty.Pty, program string, args []string) {
	started, words, err := disclaim.Words(program, args)
	if err != nil {
		return
	}
	_ = terminal.Command(started, words...)
}

func wrappedThenChanged(terminal pty.Pty, program string, args []string) {
	started, words, _ := disclaim.Words(program, args)
	started = program
	_ = terminal.Command(started, words...)
}

func notAllOfIt(terminal pty.Pty, program string, args []string) {
	started, _, _ := disclaim.Words(program, args)
	_ = terminal.Command(started, args...)
}
`, []string{"x.go:11", "x.go:13", "x.go:26", "x.go:31"}},
		{"creack/pty's", `package x

import (
	"os/exec"

	"github.com/creack/pty"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

func f(p interface{ Command(string, ...string) *exec.Cmd }) {
	_, _ = pty.Start(exec.Command("sh"))
	_, _ = pty.Start(disclaim.Cmd(exec.Command("sh")))
	_ = p.Command("sh")
}
`, []string{"x.go:12", "x.go:14"}},
		{"disclaim.StandIn", `package x

import "github.com/Sauzze/smolcorp/internal/disclaim"

func f() { disclaim.StandIn(false, disclaim.Place{})() }
`, []string{"x.go:5"}},
	} {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "x.go", c.source, 0)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		caught := []string{}
		for _, site := range spawnSites(fset, "x.go", file) {
			if spawnViolation("x.go", site) != "" {
				caught = append(caught, site.where)
			}
		}
		if !slices.Equal(caught, c.caught) {
			t.Errorf("%s: caught %v, want %v", c.name, caught, c.caught)
		}
	}
}

// Setup says what disclaiming costs, on a Mac only (release notes Q2): macOS may ask about tmux or git once.
func TestSetupSaysMacOSMayAskAboutTmuxOrGit(t *testing.T) {
	page, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	text := string(page)
	start := strings.Index(text, `<p class="choice-line" id="hire-prompts" hidden>`)
	if start < 0 {
		t.Fatal("Setup has no hire-prompts line, hidden until it's on a Mac")
	}
	line := text[start : start+strings.Index(text[start:], "</p>")]
	for _, said := range []string{"don't get Smolcorp's own permissions", "“tmux”", "“git”", "Documents", "Desktop", "Downloads",
		"iCloud Drive", "a device on your Wi-Fi", "once", "A project anywhere else in your home folder, like ~/Projects, asks nothing."} {
		if !strings.Contains(line, said) {
			t.Errorf("the line doesn't say %q: %s", said, line)
		}
	}
	if !strings.Contains(text, "$('hire-prompts').hidden = !onMac();") {
		t.Error("the line isn't shown on a Mac only")
	}
}
