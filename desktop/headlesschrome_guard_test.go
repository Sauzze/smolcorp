package desktop

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// T126CHROMEKILL1, FXCHROMEKILL1R1 B2 and B3: the guards read the desktop tests' code (go/ast), not their lines, so a
// natural spelling can't dodge them: a start written across lines, a browser's path held in a variable, a start in a
// helper of another file, an `open` held in a variable, a bundle's path built with Sprintf, Join or +.
//   - A browser's path (what headlessChrome gives, SMOLCORP_TEST_CHROME, or a string naming a browser's program) goes
//     only to startHeadlessChrome or startBrowserUntil: handed to anything else, returned, stored or put in a value, it
//     fails. A variable it's put in is followed through the function, a package's variable through the package.
//   - A start (exec.Command, CommandContext, os.StartProcess, syscall.Exec, ForkExec, StartProcess, an exec.Cmd's
//     Path, a starter held in a variable, a function of the package that hands one of its words to a start as the
//     program, a program behind env or the like, a word of an `sh -c` script) whose program is or can be a browser,
//     `open` or xdg-open, or a program in a .app bundle, fails.
//   - LSOpen… or NSWorkspace named, a browser driver imported, or a browser's own flag in a string, fails.
//   - In the browser tests' own files, any string that is `open`, or a path in a .app, fails, outside headlessChrome
//     (which reads where Chrome is installed).
// Its words are read as far as constants go: "chro"+"me", fmt.Sprintf with constants, filepath.Join. What a program
// computes at run time (a path read from a file, say) can't be read: that's left to review.

// browserHelperFiles are the files of the helper: only their helper functions may hand a browser on.
var browserHelperFiles = []string{"headlesschrome_test.go", "headlesschrome_unix_test.go", "headlesschrome_windows_test.go"}

// browserTestFiles are the files in which no string may be `open` or a path in a .app: the helper's, and the one test
// that starts a real browser.
var browserTestFiles = append(slices.Clone(browserHelperFiles), "setup_scroll_refresh_test.go")

// browserGuardFile is this file: its tables and the code it's tried on name every flag and driver.
const browserGuardFile = "headlesschrome_guard_test.go"

// browserHelpers are the functions that may hand a browser's path on (startHeadlessChrome and startBrowserUntil, its
// way in; start, which starts it; headlessChrome, which finds it), in the helper's files only.
var browserHelpers = []string{"headlessChrome", "startHeadlessChrome", "startBrowserUntil", "(*headlessBrowser).start"}

// browserWaysIn are what a browser's path may be handed to.
var browserWaysIn = []string{"startHeadlessChrome", "startBrowserUntil"}

var (
	// browserProgram is a browser's program, or its driver, as the last part of a path, any case.
	browserProgram = regexp.MustCompile(`(?i)^(google[ _-]?chrome([ _-](stable|beta|dev|unstable|canary))?|chrome|chromium([ _-]browser)?|` +
		`(chrome[_-])?headless[_-]shell|chromedriver|msedge|microsoft[ _-]edge.*|firefox|geckodriver|safari|safaridriver|` +
		`webkitwebdriver|minibrowser|brave([ _-]browser)?|opera|vivaldi|epiphany)([.]exe|[.]app)?$`)
	// opener is a program that opens a page or an app through the system: LaunchServices on a Mac.
	opener = regexp.MustCompile(`^(xdg-)?open$`)
	// bundlePath is a path into or to a .app bundle.
	bundlePath = regexp.MustCompile(`(?i)[.]app($|[/\\\x00])`)
	// browserVariable is an environment variable that names a browser to run.
	browserVariable = regexp.MustCompile(`(?i)chrom|(^|_)browser$`)
	// browserDriver is a browser driver's package.
	browserDriver = regexp.MustCompile(`(?i)chromedp|playwright|puppeteer|go-rod|selenium|webdriver|agouti|cdproto`)
	// browserFlags are a browser's own flags, and `open -a`.
	browserFlags = []string{"--headless", "--remote-debugging", "--dump-dom", "--user-data-dir", "open -a "}
	// scriptLaunchers are programs that start the program among their words.
	scriptLaunchers = []string{"env", "nohup", "nice", "xargs", "arch", "caffeinate", "timeout", "gtimeout", "setsid", "sudo", "stdbuf", "time", "exec", "command"}
	// scriptShells run the script after their -c.
	scriptShells = []string{"sh", "bash", "zsh", "dash", "ksh"}
)

// unread stands for a part of a string the guard can't read: a variable's, a call's.
const unread = "\x00"

// pathParts are a string's parts between slashes, backslashes and what can't be read.
func pathParts(words string) []string {
	return strings.FieldsFunc(words, func(r rune) bool { return r == '/' || r == '\\' || r == 0 })
}

// namesBrowser is whether words are a browser's program, or a path through a browser's bundle.
func namesBrowser(words string) bool {
	parts := pathParts(words)
	if len(parts) == 0 {
		return false
	}
	if browserProgram.MatchString(strings.TrimSpace(parts[len(parts)-1])) {
		return true
	}
	return slices.ContainsFunc(parts, func(part string) bool {
		return strings.HasSuffix(strings.ToLower(part), ".app") && browserProgram.MatchString(part)
	})
}

// namesOpener is whether words are `open` or xdg-open: by name, in a system folder, or in a folder that can't be read.
func namesOpener(words string) bool {
	parts := pathParts(words)
	if len(parts) == 0 || !opener.MatchString(parts[len(parts)-1]) {
		return false
	}
	folder := strings.TrimSuffix(words, parts[len(parts)-1])
	return strings.HasPrefix(parts[len(parts)-1], "xdg") || slices.Contains([]string{"", "/bin/", "/usr/bin/", "/usr/local/bin/"}, folder) ||
		strings.HasPrefix(words, unread)
}

// guardFinding is one way a test file starts a browser outside the helper, or opens through LaunchServices or runs or
// makes a bundle (launch).
type guardFinding struct {
	where, what string
	launch      bool
}

func (f guardFinding) String() string { return f.where + ": " + f.what }

// guardFunction is a function of the package: a declared one, or a function literal given a name.
type guardFunction struct {
	name    string
	params  []string
	body    ast.Node
	imports map[string]string // its file's
}

// browserGuardReader reads one package's files.
type browserGuardReader struct {
	fset     *token.FileSet
	files    map[string]*ast.File
	consts   map[string]string // constants that are strings, by name
	wrappers map[string][]int  // functions that start the word at these places as a program, by name
	aliases  map[string]int    // names a starter is held in, with its program's place
	funcs    []guardFunction
	found    []guardFinding

	browsers, openers, bundles map[string]bool // the package's variables that hold a browser's path, `open`, a bundle's path
}

// browserGuard reads a package's Go files (by name) and gives what fails.
func browserGuard(fset *token.FileSet, files map[string]*ast.File) []guardFinding {
	g := &browserGuardReader{fset: fset, files: files, consts: map[string]string{}, wrappers: map[string][]int{}, aliases: map[string]int{}}
	g.readConstants()
	g.readFunctions()
	g.readWrappers()
	g.browsers, g.openers, g.bundles = map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
				g.spread(gen, g.browsers, g.browserSource)
				g.spread(gen, g.openers, g.openerSource)
				g.spread(gen, g.bundles, g.bundleSource)
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		g.readFile(name, files[name])
	}
	return g.found
}

func (g *browserGuardReader) at(node ast.Node, name, what string, launch bool) {
	f := guardFinding{where: name + ":" + strconv.Itoa(g.fset.Position(node.Pos()).Line), what: what, launch: launch}
	if !slices.Contains(g.found, f) {
		g.found = append(g.found, f)
	}
}

// readConstants keeps every constant that is a string, read as far as other constants go. A name two constants have,
// with two values, is kept as neither.
func (g *browserGuardReader) readConstants() {
	for range 8 { // a constant made of constants, eight deep at most
		values, twice := map[string]string{}, map[string]bool{}
		for _, file := range g.files {
			ast.Inspect(file, func(node ast.Node) bool {
				decl, ok := node.(*ast.GenDecl)
				if !ok || decl.Tok != token.CONST {
					return true
				}
				for _, spec := range decl.Specs {
					spec := spec.(*ast.ValueSpec)
					for i, name := range spec.Names {
						if i >= len(spec.Values) {
							continue
						}
						words := g.words(spec.Values[i])
						if known, ok := values[name.Name]; ok && known != words || strings.Contains(words, unread) {
							twice[name.Name] = true
						}
						values[name.Name] = words
					}
				}
				return true
			})
		}
		for name := range twice {
			delete(values, name)
		}
		if maps.Equal(values, g.consts) {
			return
		}
		g.consts = values
	}
}

// words is what a string expression says, with what can't be read as unread.
func (g *browserGuardReader) words(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			s, _ := strconv.Unquote(e.Value)
			return s
		}
		return e.Value
	case *ast.ParenExpr:
		return g.words(e.X)
	case *ast.Ident:
		if s, ok := g.consts[e.Name]; ok {
			return s
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return g.words(e.X) + g.words(e.Y)
		}
	case *ast.CallExpr:
		var args []string
		for _, arg := range e.Args {
			args = append(args, g.words(arg))
		}
		if e.Ellipsis.IsValid() {
			return unread
		}
		switch calleeName(e.Fun) {
		case "fmt.Sprintf":
			if len(args) > 0 {
				rest := make([]any, len(args)-1)
				for i, arg := range args[1:] {
					rest[i] = arg
				}
				return fmt.Sprintf(args[0], rest...)
			}
		case "fmt.Sprint", "string":
			return strings.Join(args, "")
		case "filepath.Join", "path.Join":
			return strings.Join(args, "/")
		case "strings.Join":
			if list, ok := e.Args[0].(*ast.CompositeLit); ok && len(args) == 2 {
				var parts []string
				for _, elt := range list.Elts {
					parts = append(parts, g.words(elt))
				}
				return strings.Join(parts, args[1])
			}
		case "strings.ToLower", "strings.ToUpper", "strings.TrimSpace", "filepath.Clean", "path.Clean", "filepath.FromSlash", "filepath.ToSlash":
			if len(args) == 1 {
				return args[0]
			}
		}
	}
	return unread
}

// calleeName is a call's function as written: name, pkg.name, or x.name for a method.
func calleeName(fun ast.Expr) string {
	switch fun := fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		if x, ok := fun.X.(*ast.Ident); ok {
			return x.Name + "." + fun.Sel.Name
		}
		return "." + fun.Sel.Name
	case *ast.ParenExpr:
		return calleeName(fun.X)
	}
	return ""
}

// lastName is a call's function's own name, without its package or receiver.
func lastName(fun ast.Expr) string {
	name := calleeName(fun)
	return name[strings.LastIndex(name, ".")+1:]
}

// isWordsNode is whether a node gives a string the guard reads: a string, a sum, a Sprintf, a Join.
func (g *browserGuardReader) isWordsNode(node ast.Node) (ast.Expr, bool) {
	switch node := node.(type) {
	case *ast.BasicLit:
		return node, node.Kind == token.STRING
	case *ast.BinaryExpr:
		return node, node.Op == token.ADD
	case *ast.CallExpr:
		return node, slices.Contains([]string{"fmt.Sprintf", "fmt.Sprint", "filepath.Join", "path.Join", "strings.Join"}, calleeName(node.Fun))
	}
	return nil, false
}

// readFunctions lists the package's functions, and the function literals given a name.
func (g *browserGuardReader) readFunctions() {
	params := func(fields *ast.FieldList) []string {
		var names []string
		if fields == nil {
			return nil
		}
		for _, field := range fields.List {
			if len(field.Names) == 0 {
				names = append(names, "_")
			}
			for _, name := range field.Names {
				names = append(names, name.Name)
			}
		}
		return names
	}
	for _, file := range g.files {
		imports := fileImports(file)
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.FuncDecl:
				if node.Body != nil {
					g.funcs = append(g.funcs, guardFunction{node.Name.Name, params(node.Type.Params), node.Body, imports})
				}
			case *ast.AssignStmt:
				for i, right := range node.Rhs {
					if lit, ok := right.(*ast.FuncLit); ok && i < len(node.Lhs) {
						g.funcs = append(g.funcs, guardFunction{lastName(node.Lhs[i]), params(lit.Type.Params), lit.Body, imports})
					}
				}
			case *ast.ValueSpec:
				for i, right := range node.Values {
					if lit, ok := right.(*ast.FuncLit); ok && i < len(node.Names) {
						g.funcs = append(g.funcs, guardFunction{node.Names[i].Name, params(lit.Type.Params), lit.Body, imports})
					}
				}
			}
			return true
		})
	}
}

// starterPlace is where a starter of a file takes its program: its place, or -1 if fun isn't one.
func starterPlace(fun ast.Expr, imports map[string]string) int {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return -1
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || !slices.Contains(starters[imports[pkg.Name]], sel.Sel.Name) {
		return -1
	}
	if sel.Sel.Name == "CommandContext" {
		return 1
	}
	return 0
}

// fileImports are a file's imports, by the name each goes by in it.
func fileImports(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		as := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			as = spec.Name.Name
		}
		imports[as] = path
	}
	return imports
}

// programPlace is one place a program is given: the expression, and what it says (a word of a script, for an `sh -c`).
type programPlace struct {
	expr   ast.Expr
	words  string
	wayIn  bool // given to the helper's way in, which may be handed a browser
	script bool // a word of a script: only what it says is read
}

// programs are the places a call gives a program: a starter's, a starter held in a name's, a wrapper's; then the
// program behind a launcher, and the words of an `sh -c` script.
func (g *browserGuardReader) programs(call *ast.CallExpr, imports map[string]string) []programPlace {
	var places []int
	if at := starterPlace(call.Fun, imports); at >= 0 {
		places = append(places, at)
	}
	if at, ok := g.aliases[lastName(call.Fun)]; ok {
		places = append(places, at)
	}
	places = append(places, g.wrappers[lastName(call.Fun)]...)
	wayIn := slices.Contains(browserWaysIn, lastName(call.Fun))
	var found []programPlace
	for _, at := range places {
		for at < len(call.Args) && !(call.Ellipsis.IsValid() && at == len(call.Args)-1) {
			words := g.words(call.Args[at])
			found = append(found, programPlace{expr: call.Args[at], words: words, wayIn: wayIn})
			program := ""
			if parts := pathParts(words); len(parts) > 0 {
				program = parts[len(parts)-1]
			}
			if slices.Contains(scriptShells, program) {
				for i := at + 1; i+1 < len(call.Args); i++ {
					if g.words(call.Args[i]) == "-c" {
						script := g.words(call.Args[i+1])
						for _, word := range strings.FieldsFunc(script, func(r rune) bool {
							return unicode.IsSpace(r) || strings.ContainsRune(";&|()`'\"<>{}", r)
						}) {
							found = append(found, programPlace{expr: call.Args[i+1], words: word, script: true})
						}
						break
					}
				}
			}
			if !slices.Contains(scriptLaunchers, program) {
				break
			}
			// The next word that isn't a flag, a setting or a number is the program the launcher starts.
			for at++; at < len(call.Args); at++ {
				next := g.words(call.Args[at])
				if next == "" || strings.HasPrefix(next, "-") || strings.Contains(next, "=") || unicode.IsDigit(rune(next[0])) {
					continue
				}
				break
			}
		}
	}
	return found
}

// spread is the names a function's body puts what's tainted in, from names, until no more: assigned, declared,
// ranged over.
func (g *browserGuardReader) spread(body ast.Node, names map[string]bool, source func(ast.Node) bool) {
	add := func(left ast.Expr, right ast.Expr) bool {
		id, ok := left.(*ast.Ident)
		if !ok || id.Name == "_" || names[id.Name] || right == nil || !g.tainted(right, names, source) {
			return false
		}
		names[id.Name] = true
		return true
	}
	for changed := true; changed; {
		changed = false
		ast.Inspect(body, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.AssignStmt:
				for i, left := range node.Lhs {
					var right ast.Expr
					if len(node.Rhs) == len(node.Lhs) {
						right = node.Rhs[i]
					} else if len(node.Rhs) == 1 {
						right = node.Rhs[0]
					}
					changed = add(left, right) || changed
				}
			case *ast.ValueSpec:
				for i, name := range node.Names {
					var right ast.Expr
					if len(node.Values) == len(node.Names) {
						right = node.Values[i]
					} else if len(node.Values) == 1 {
						right = node.Values[0]
					}
					changed = add(name, right) || changed
				}
			case *ast.RangeStmt:
				if node.Key != nil {
					changed = add(node.Key, node.X) || changed
				}
				if node.Value != nil {
					changed = add(node.Value, node.X) || changed
				}
			}
			return true
		})
	}
}

// tainted is whether e holds a name in names, or a source: not inside a function literal, a comparison (a yes or no),
// a field's name, or what the helper's way in gives.
func (g *browserGuardReader) tainted(e ast.Node, names map[string]bool, source func(ast.Node) bool) bool {
	found := false
	ast.Inspect(e, func(node ast.Node) bool {
		if found || node == nil {
			return false
		}
		switch node := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BinaryExpr:
			if node.Op == token.EQL || node.Op == token.NEQ || node.Op == token.LSS || node.Op == token.GTR ||
				node.Op == token.LEQ || node.Op == token.GEQ || node.Op == token.LAND || node.Op == token.LOR {
				return false
			}
		case *ast.CallExpr:
			if slices.Contains(browserWaysIn, lastName(node.Fun)) || calleeName(node.Fun) == "len" {
				return false
			}
		case *ast.SelectorExpr:
			found = g.tainted(node.X, names, source)
			return false
		case *ast.KeyValueExpr:
			found = g.tainted(node.Value, names, source)
			return false
		case *ast.Ident:
			found = names[node.Name]
		}
		if !found && source != nil {
			found = source(node)
		}
		return !found
	})
	return found
}

// readWrappers finds, until no more, the functions that start one of their words as a program, and the names a
// starter is held in.
func (g *browserGuardReader) readWrappers() {
	for _, file := range g.files {
		imports := fileImports(file)
		hold := func(left, right ast.Expr) {
			if at := starterPlace(right, imports); at >= 0 {
				g.aliases[lastName(left)] = at
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.AssignStmt:
				for i := range min(len(node.Lhs), len(node.Rhs)) {
					hold(node.Lhs[i], node.Rhs[i])
				}
			case *ast.ValueSpec:
				for i := range min(len(node.Names), len(node.Values)) {
					hold(node.Names[i], node.Values[i])
				}
			}
			return true
		})
	}
	for changed := true; changed; {
		changed = false
		for _, fn := range g.funcs {
			if !g.startsTainted(fn.body, fn.imports, nil) {
				continue // no start in it at all
			}
			for at, param := range fn.params {
				if param == "_" || slices.Contains(g.wrappers[fn.name], at) {
					continue
				}
				names := map[string]bool{param: true}
				g.spread(fn.body, names, nil)
				if g.startsTainted(fn.body, fn.imports, names) {
					g.wrappers[fn.name] = append(g.wrappers[fn.name], at)
					changed = true
				}
			}
		}
	}
}

// startsTainted is whether a body gives a start, as its program, something that holds a name in names (with names
// nil, anything).
func (g *browserGuardReader) startsTainted(body ast.Node, imports map[string]string, names map[string]bool) bool {
	holds := func(e ast.Expr) bool { return names == nil || g.tainted(e, names, nil) }
	started := false
	ast.Inspect(body, func(node ast.Node) bool {
		if started {
			return false
		}
		switch node := node.(type) {
		case *ast.CallExpr:
			for _, place := range g.programs(node, imports) {
				started = started || !place.script && holds(place.expr)
			}
		case *ast.CompositeLit:
			if path := cmdPath(node, imports); path != nil {
				started = holds(path)
			}
		}
		return !started
	})
	return started
}

// cmdPath is an exec.Cmd literal's Path.
func cmdPath(lit *ast.CompositeLit, imports map[string]string) ast.Expr {
	sel, ok := lit.Type.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Cmd" {
		return nil
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || imports[pkg.Name] != "os/exec" {
		return nil
	}
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok && exprName(kv.Key) == "Path" {
			return kv.Value
		}
	}
	return nil
}

// sources of each kind of taint.
func (g *browserGuardReader) browserSource(node ast.Node) bool {
	if call, ok := node.(*ast.CallExpr); ok {
		switch calleeName(call.Fun) {
		case "headlessChrome":
			return true
		case "os.Getenv", "os.LookupEnv":
			return browserVariable.MatchString(g.words(call.Args[0]))
		}
	}
	e, ok := g.isWordsNode(node)
	return ok && namesBrowser(g.words(e))
}

func (g *browserGuardReader) openerSource(node ast.Node) bool {
	e, ok := g.isWordsNode(node)
	return ok && namesOpener(g.words(e))
}

func (g *browserGuardReader) bundleSource(node ast.Node) bool {
	e, ok := g.isWordsNode(node)
	return ok && bundlePath.MatchString(g.words(e))
}

// isHelper is whether a function, in a file, is one of the helper's own, or this guard's own self-test, whose cases
// are words, never run: they may hand a browser's path on. A start in one is still read.
func isHelper(name string, decl *ast.FuncDecl) bool {
	if decl != nil && name == browserGuardFile && decl.Recv == nil && decl.Name.Name == "TestTheBrowserCheckCatchesAStartOutsideTheHelper" {
		return true
	}
	if decl == nil || !slices.Contains(browserHelperFiles, name) {
		return false
	}
	fn := decl.Name.Name
	if decl.Recv != nil && len(decl.Recv.List) == 1 {
		if star, ok := decl.Recv.List[0].Type.(*ast.StarExpr); ok {
			fn = "(*" + exprName(star.X) + ")." + fn
		}
	}
	return slices.Contains(browserHelpers, fn)
}

// readFile reads one file: its imports, then each declaration.
func (g *browserGuardReader) readFile(name string, file *ast.File) {
	imports := fileImports(file)
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if browserDriver.MatchString(path) && name != browserGuardFile {
			g.at(spec, name, "imports a browser driver, "+path, false)
		}
		if _, starts := starters[path]; starts && spec.Name != nil && spec.Name.Name == "." {
			g.at(spec, name, "imports "+path+" with a dot: its starts can't be read", false)
		}
	}
	for _, decl := range file.Decls {
		var body ast.Node = decl
		fn, _ := decl.(*ast.FuncDecl)
		if gen, ok := decl.(*ast.GenDecl); fn != nil && fn.Body == nil || ok && gen.Tok == token.IMPORT {
			continue
		}
		// A package's variables are followed by name across the package, a function's through the function.
		b, o, a := maps.Clone(g.browsers), maps.Clone(g.openers), maps.Clone(g.bundles)
		g.spread(body, b, g.browserSource)
		g.spread(body, o, g.openerSource)
		g.spread(body, a, g.bundleSource)
		g.readDecl(name, fn, body, imports, b, o, a)
	}
}

// readDecl reads one declaration.
func (g *browserGuardReader) readDecl(name string, fn *ast.FuncDecl, body ast.Node, imports map[string]string, browsers, openers, bundles map[string]bool) {
	helper := isHelper(name, fn)
	strict := slices.Contains(browserTestFiles, name) && !(helper && fn.Name.Name == "headlessChrome")
	called := map[ast.Node]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.CallExpr:
			called[node.Fun] = true
		case *ast.AssignStmt:
			for _, right := range node.Rhs {
				called[right] = true // a starter held in a name: its calls are read as the starter's
			}
		case *ast.ValueSpec:
			for _, right := range node.Values {
				called[right] = true
			}
		}
		return true
	})
	browser := func(e ast.Node) bool { return g.tainted(e, browsers, g.browserSource) }
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.Ident:
			if strings.HasPrefix(node.Name, "LSOpen") || strings.Contains(node.Name, "NSWorkspace") {
				g.at(node, name, "names "+node.Name+": LaunchServices", true)
			}
		case *ast.SelectorExpr:
			if starterPlace(node, imports) >= 0 && !called[node] {
				g.at(node, name, calleeName(node)+" handed on as a value: its program can't be read", false)
			}
		case *ast.CallExpr:
			for _, place := range g.programs(node, imports) {
				openish := namesOpener(place.words) || !place.script && g.tainted(place.expr, openers, g.openerSource)
				bundled := bundlePath.MatchString(place.words) || !place.script && g.tainted(place.expr, bundles, g.bundleSource)
				browserish := namesBrowser(place.words) || !place.script && browser(place.expr)
				if openish {
					g.at(place.expr, name, calleeName(node.Fun)+" starts `open` or xdg-open: LaunchServices", true)
				}
				if bundled {
					g.at(place.expr, name, calleeName(node.Fun)+" starts a program in a .app bundle", true)
				}
				if browserish && !place.wayIn {
					g.at(place.expr, name, calleeName(node.Fun)+" starts a browser", false)
				}
			}
			if !helper && !slices.Contains(browserWaysIn, lastName(node.Fun)) {
				for _, arg := range node.Args {
					if browser(arg) {
						g.at(arg, name, "hands a browser's path to "+calleeName(node.Fun), false)
					}
				}
			}
		case *ast.CompositeLit:
			if path := cmdPath(node, imports); path != nil {
				if words := g.words(path); namesOpener(words) || bundlePath.MatchString(words) ||
					g.tainted(path, openers, g.openerSource) || g.tainted(path, bundles, g.bundleSource) {
					g.at(path, name, "an exec.Cmd's Path is `open` or in a .app bundle", true)
				}
			}
			for _, elt := range node.Elts {
				if !helper && browser(elt) {
					g.at(elt, name, "puts a browser's path in a value", false)
				}
			}
		case *ast.AssignStmt:
			for i, left := range node.Lhs {
				if _, plain := left.(*ast.Ident); !plain && !helper && i < len(node.Rhs) && browser(node.Rhs[i]) {
					g.at(left, name, "stores a browser's path", false)
				}
				if sel, ok := left.(*ast.SelectorExpr); ok && sel.Sel.Name == "Path" && i < len(node.Rhs) {
					if words := g.words(node.Rhs[i]); namesOpener(words) || bundlePath.MatchString(words) {
						g.at(left, name, "a Path set to `open` or into a .app bundle", true)
					}
				}
			}
		case *ast.ReturnStmt:
			for _, result := range node.Results {
				if !helper && browser(result) {
					g.at(result, name, "returns a browser's path", false)
				}
			}
		case *ast.SendStmt:
			if !helper && browser(node.Value) {
				g.at(node.Value, name, "sends a browser's path", false)
			}
		}
		if e, ok := g.isWordsNode(node); ok {
			words := g.words(e)
			if !helper && name != browserGuardFile {
				for _, flag := range browserFlags {
					if strings.Contains(words, flag) {
						g.at(node, name, "a browser's flag, "+strings.TrimSpace(flag), false)
					}
				}
			}
			if strict && namesOpener(words) {
				g.at(node, name, "`open` in a browser test", true)
			}
			if strict && bundlePath.MatchString(words) {
				g.at(node, name, "a .app path in a browser test", true)
			}
		}
		return true
	})
}

// readBrowserTests parses this package's test files.
func readBrowserTests(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	names, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 100 || !slices.Contains(names, browserGuardFile) {
		t.Fatalf("only %d test files found: the check is looking in the wrong place", len(names))
	}
	fset, files := token.NewFileSet(), map[string]*ast.File{}
	for _, name := range names {
		if files[name], err = parser.ParseFile(fset, name, nil, parser.SkipObjectResolution); err != nil {
			t.Fatal(err)
		}
	}
	return fset, files
}

// TestEveryBrowserATestStartsGoesThroughTheHelper: a test of this package that starts a browser outside
// startHeadlessChrome could leave it running (T126CHROMEKILL1): every test file's code is read for one.
func TestEveryBrowserATestStartsGoesThroughTheHelper(t *testing.T) {
	for _, f := range browserGuard(readBrowserTests(t)) {
		if !f.launch {
			t.Errorf("%s\n\tstarts or hands on a browser outside startHeadlessChrome (headlesschrome_test.go): start it with that", f)
		}
	}
}

// TestNoBrowserTestOpensThroughLaunchServicesOrMakesABundle: no test starts `open`, xdg-open or a program in a .app,
// or calls LSOpen or NSWorkspace; the browser tests make no .app either (VICTOR, 15 Nov: a stand-in .app launched
// through LaunchServices put a Gatekeeper dialog on the owner's screen).
func TestNoBrowserTestOpensThroughLaunchServicesOrMakesABundle(t *testing.T) {
	fset, files := readBrowserTests(t)
	for _, name := range browserTestFiles {
		if files[name] == nil {
			t.Errorf("%s isn't read", name)
		}
	}
	for _, f := range browserGuard(fset, files) {
		if f.launch {
			t.Errorf("%s: start browsers as plain programs, never through LaunchServices or a .app", f)
		}
	}
}

// The guard itself: each way the reviews found to dodge the line reader (RT126CHROMEKILL1 B2, B3), and others, is
// caught; finding Chrome, starting it through the helper, and words about browsers aren't. Each case is a package of
// one or two files, read as these tests are, never built or run.
func TestTheBrowserCheckCatchesAStartOutsideTheHelper(t *testing.T) {
	wrap := func(body string) string {
		return "package x\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"syscall\"\n)\n\nfunc f(t *testing.T, ctx context.Context, dir, name, page string) error {\n" + body + "\n\treturn nil\n}\n"
	}
	caught := map[string]map[string]string{
		"B2: gofmt's multi-line call, Chrome in a variable": {"x_test.go": wrap("\tbin := headlessChrome()\n\treturn exec.Command(\n\t\tbin,\n\t\t\"--window-size=1100,700\", page,\n\t).Run()")},
		"B2: exec.Command(bin, page)":                       {"x_test.go": wrap("\tbin := headlessChrome()\n\t_ = exec.Command(bin, page)")},
		"B2: CommandContext of SMOLCORP_TEST_CHROME":        {"x_test.go": wrap("\tp := os.Getenv(\"SMOLCORP_TEST_CHROME\")\n\t_ = exec.CommandContext(ctx, p)")},
		"B2: os.StartProcess":                               {"x_test.go": wrap("\tchrome := headlessChrome()\n\t_, _ = os.StartProcess(chrome, []string{chrome, page}, nil)")},
		"B2: an exec.Cmd with Chrome as its Path":           {"x_test.go": wrap("\tchrome := headlessChrome()\n\t_ = (&exec.Cmd{Path: chrome, Args: []string{page}}).Start()")},
		"B2: a helper of another file": {
			"x_test.go": wrap("\tchrome := headlessChrome()\n\treturn runProgram(t, chrome, page)"),
			"y_test.go": "package x\n\nimport \"os/exec\"\n\nfunc runProgram(t *testing.T, name string, args ...string) error {\n\treturn exec.Command(name, args...).Run()\n}\n",
		},
		"a helper of another file, given Chrome's path written out": {
			"x_test.go": wrap("\treturn runProgram(t, \"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome\", page)"),
			"y_test.go": "package x\n\nimport \"os/exec\"\n\nfunc runProgram(t *testing.T, name string, args ...string) error {\n\tprogram := name\n\treturn exec.Command(program, args...).Run()\n}\n",
		},
		"B2: syscall.Exec":                  {"x_test.go": wrap("\tchrome := headlessChrome()\n\t_ = syscall.Exec(chrome, nil, nil)")},
		"Chrome's path written out":         {"x_test.go": wrap("\t_ = exec.Command(\"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome\", page)")},
		"a Windows Chrome":                  {"x_test.go": wrap("\t_ = exec.Command(\"C:\\\\Program Files\\\\Google\\\\Chrome\\\\Application\\\\chrome.exe\")")},
		"chro + me":                         {"x_test.go": wrap("\t_ = exec.Command(\"chro\"+\"me\", page)")},
		"Sprintf's chromium":                {"x_test.go": wrap("\t_ = exec.Command(fmt.Sprintf(\"%s%s\", \"chro\", \"mium\"), page)")},
		"LookPath's google-chrome":          {"x_test.go": wrap("\tp, _ := exec.LookPath(\"google-chrome\")\n\t_ = exec.Command(p, page)")},
		"exec.Command held in a name":       {"x_test.go": wrap("\tstart := exec.Command\n\t_ = start(\"xdg-open\", page)")},
		"exec.Command handed on":            {"x_test.go": wrap("\trun(exec.Command)")},
		"os/exec under another name":        {"x_test.go": "package x\n\nimport run \"os/exec\"\n\nfunc f() { _ = run.Command(headlessChrome()) }\n"},
		"os/exec dot-imported":              {"x_test.go": "package x\n\nimport . \"os/exec\"\n\nfunc f(page string) { _ = Command(page) }\n"},
		"behind env":                        {"x_test.go": wrap("\t_ = exec.Command(\"/usr/bin/env\", \"-i\", \"xdg-open\", page)")},
		"in an sh -c script":                {"x_test.go": wrap("\t_ = exec.Command(\"/bin/sh\", \"-c\", \"exec /usr/bin/open \"+page)")},
		"a Path set to Chrome":              {"x_test.go": wrap("\tcmd := exec.Command(\"true\")\n\tcmd.Path = headlessChrome()")},
		"returned":                          {"x_test.go": "package x\n\nfunc myChrome() string { return headlessChrome() }\n"},
		"a package variable":                {"x_test.go": "package x\n\nimport \"os/exec\"\n\nvar chrome = headlessChrome()\n\nfunc f() { _ = exec.Command(chrome) }\n"},
		"a browser's flags":                 {"x_test.go": wrap("\targs := []string{\"--headless=new\", \"--use-mock-keychain\"}")},
		"a driver":                          {"x_test.go": "package x\n\nimport \"github.com/chromedp/chromedp\"\n\nfunc f() { chromedp.NewContext(nil) }\n"},
		"open -a in a word":                 {"x_test.go": wrap("\t_ = exec.Command(\"/bin/sh\", \"-c\", \"open -a Safari \"+page)")},
		"LSOpen":                            {"x_test.go": wrap("\tLSOpenCFURLRef(nil, nil)")},
		"B3: open held in a variable":       {"x_test.go": wrap("\topener := \"/usr/bin/open\"\n\treturn exec.Command(opener, dir).Run()")},
		"B3: open put together":             {"x_test.go": wrap("\t_ = exec.Command(\"/usr/bin/\"+\"open\", dir)")},
		"B3: open through a helper":         {"x_test.go": wrap("\t_ = runIt(\"open\", dir)"), "y_test.go": "package x\n\nimport \"os/exec\"\n\nfunc runIt(name, arg string) error { return exec.Command(name, arg).Run() }\n"},
		"a program in a bundle, by Sprintf": {"x_test.go": wrap("\tapp := fmt.Sprintf(\"%s/%s.app\", dir, name)\n\t_ = exec.Command(app + \"/Contents/MacOS/\" + name)")},
		"an exec.Cmd whose Path is open":    {"x_test.go": wrap("\t_ = &exec.Cmd{Path: \"/usr/bin/open\"}")},
		"B3: a Sprintf bundle in the helper's file": {
			"headlesschrome_unix_test.go": wrap("\tapp := fmt.Sprintf(\"%s/%s.app\", dir, name)\n\tif err := os.MkdirAll(app, 0o700); err != nil {\n\t\treturn err\n\t}\n\treturn nil"),
		},
		"a joined bundle in the helper's file": {
			"headlesschrome_test.go": wrap("\t_ = os.MkdirAll(filepath.Join(dir, \"StandIn.app\", \"Contents\", \"MacOS\"), 0o700)"),
		},
		"a bundle by + in the scroll test": {
			"setup_scroll_refresh_test.go": wrap("\t_ = os.WriteFile(dir+\"/StandIn\"+\".app/Contents/MacOS/StandIn\", nil, 0o700)"),
		},
		"a start planted inside headlessChrome": {
			"headlesschrome_test.go": "package x\n\nimport \"os/exec\"\n\nfunc headlessChrome() string {\n\t_ = exec.Command(\"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome\").Start()\n\treturn \"\"\n}\n",
		},
		"a start in the helper's file, outside the helper": {
			"headlesschrome_unix_test.go": "package x\n\nimport \"os/exec\"\n\nfunc other() { _ = exec.Command(headlessChrome()).Start() }\n",
		},
	}
	read := func(t *testing.T, sources map[string]string) []guardFinding {
		fset, files := token.NewFileSet(), map[string]*ast.File{}
		for name, source := range sources {
			file, err := parser.ParseFile(fset, name, source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("%s: %v\n%s", name, err, source)
			}
			files[name] = file
		}
		return browserGuard(fset, files)
	}
	for what, sources := range caught {
		t.Run(what, func(t *testing.T) {
			if found := read(t, sources); len(found) == 0 {
				t.Errorf("not caught:\n%v", sources)
			}
		})
	}
	passed := map[string]map[string]string{
		"the scroll test's start": {"setup_scroll_refresh_test.go": wrap("\tchrome := headlessChrome()\n\tif chrome == \"\" {\n\t\tt.Skip(\"no Chrome or Chromium on this computer (SMOLCORP_TEST_CHROME names one)\")\n\t}\n" +
			"\tb := startHeadlessChrome(t, chrome, \"--window-size=1100,700\", page)\n\tt.Fatalf(\"Chrome ended first (%s): %s\", b.why, b.said())")},
		"finding Chrome": {"headlesschrome_test.go": "package x\n\nimport \"os\"\n\nfunc headlessChrome() string {\n\tif _, err := os.Stat(\"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome\"); err == nil {\n" +
			"\t\treturn \"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome\"\n\t}\n\treturn \"\"\n}\n"},
		"a stand-in and a shell": {"headlesschrome_unix_test.go": wrap("\t_ = os.WriteFile(filepath.Join(dir, \"standin-browser\"), nil, 0o700)\n\t_ = exec.Command(\"/bin/sh\", \"-c\", \"exec sleep 30\")")},
		"git":                    {"x_test.go": wrap("\t_ = exec.Command(\"git\", \"status\")")},
		"open as a word":         {"x_test.go": wrap("\tsays(false, \"open\")\n\tr.trace = append(r.trace, \"open\")")},
		"Smolcorp.app as a word": {"x_test.go": wrap("\t_ = filepath.Join(dir, \"Smolcorp.app\", \"Contents\", \"Resources\")")},
		"a phone's browser":      {"x_test.go": wrap("\t_ = map[string]string{\"Mozilla/5.0 (Linux; Android 15; Pixel 9) Chrome/140.0.0\": \"Android phone\"}")},
	}
	for what, sources := range passed {
		t.Run(what, func(t *testing.T) {
			if found := read(t, sources); len(found) != 0 {
				t.Errorf("caught, but starts no browser and opens nothing: %v", found)
			}
		})
	}
}
