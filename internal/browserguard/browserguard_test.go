package browserguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// FXCHROMEPOPUP: no test runs the owner's installed browser. Two finders fell back to it when SMOLCORP_TEST_CHROME was
// unset: the sandbox's normal work ran /Applications/Google Chrome.app in a desk's profile, where it aborted, and each
// crash put a "Google Chrome quit unexpectedly" dialog on the owner's screen (seven of them, all from sandbox.test);
// the setup page's scroll test looked on the PATH, in /Applications and in Program Files. Now a test's browser is
// chrome-headless-shell, only where SMOLCORP_TEST_CHROME names one (an opt-in, on the runner and CI): with no flag the
// test skips, here and everywhere, and nothing looks for a browser.
//
// This guard reads every Go file of the module, tests and the code beside them, every system's (go/ast, as far as
// constants, sums, Sprintf and Join go): a test can call a helper of its package's own code, so a finder's fallback
// moved there is as much a test's (RT126CHROMEPOPUP B1, its MC). No file spells an installed browser's program (a
// browser's .app, its Windows .exe, or its command as on the PATH, alone or as a path's last part), but in a
// declaration on installedBrowserWords, where it's words only, never resolved or run; none looks up a browser on the
// PATH (exec.LookPath of a browser's name, chrome-headless-shell's and "chrome" included); and none reads a variable
// that names a browser (CHROME_BIN, PUPPETEER_EXECUTABLE_PATH and the like) but SMOLCORP_TEST_CHROME: a test's
// browser comes from the opt-in only. Since FXCHROMEPOPUPR1 the desktop's helper refuses the rest at run time too
// (browserRefusal, desktop/headlesschrome_test.go): this guard is the earlier of the two.

// guardRoot is the module's folder, from this one; guardSelf is this package's, the one folder not read: its tables
// name every browser.
var (
	guardRoot = filepath.Join("..", "..")
	guardSelf = "internal/browserguard"
)

// guardMustRead are folders the guard fails without: the packages whose tests start a browser, with at least 20 Go
// files each, so a guard that looks in the wrong place can't pass by reading nothing.
var guardMustRead = []string{"desktop", "internal/sandbox"}

// allowance is why a declaration may spell an installed browser's program, and, when under is set, the only places it
// may: every such string in it starts with one of them (RT126CHROMEPOPUP N2: a real path added later to a declaration
// on the list isn't let through by it).
type allowance struct {
	why   string
	under []string
}

// installedBrowserWords are the declarations that may spell an installed browser's program. A key is
// "<folder>/<file> <declaration>". One no longer used fails the guard, so the list stays true.
var installedBrowserWords = map[string]allowance{
	"internal/sandbox/escape3_darwin_test.go var deskFormRows":                              {why: "startRefusal's rows for its desk forms: paths given to it and to headlessChrome as words only, never stat'd or run; the stand-in Chrome's is in a temp folder of the row's, with nothing made there"},
	"internal/sandbox/escape3_darwin_test.go var refusalProbes":                             {why: "missing paths in a folder that isn't there, given to the start helpers, which refuse each before any start", under: []string{"<gone>/"}},
	"internal/sandbox/apparmor_test.go ownComputer":                                         {why: "a made-up Ubuntu's AppArmor files, in memory (snap Firefox's profile attaches to /snap/firefox/*/firefox): words read back by the AppArmor rows, never resolved or run", under: []string{"/snap/firefox/*/"}},
	"desktop/headlesschrome_guard_test.go TestTheBrowserCheckCatchesAStartOutsideTheHelper": {why: "the browser check's self-test: sources it only parses, never compiled or run"},
}

var (
	// browserBundle is a browser's .app, any case: Chrome's, Chromium's, Edge's, Firefox's, Safari's and the like,
	// Chrome for Testing's or a channel's too.
	browserBundle = regexp.MustCompile(`(?i)^(google chrome|chrome|chromium|microsoft edge|firefox|safari|brave browser|opera|vivaldi|arc)\b.*[.]app$`)
	// browserExe is a browser's program on Windows.
	browserExe = regexp.MustCompile(`(?i)^(chrome|msedge|firefox|brave|opera|vivaldi)[.]exe$`)
	// browserCommand is a browser's command, as on the PATH, alone or as a path's last part (/usr/bin/google-chrome).
	// "chrome" alone is left out: it's a profile folder's name in the tests, and a finder's list of commands names
	// google-chrome as well. So is "safari": no system has it as a command (Safari's .app is browserBundle's), and
	// it's an outfit's name in the app (desktop/layout.go).
	browserCommand = regexp.MustCompile(`(?i)^(google-chrome(-stable|-beta|-unstable|-canary)?|chromium(-browser)?|msedge|` +
		`microsoft-edge(-stable|-beta|-dev)?|firefox|brave-browser)$`)
	// browserLookedUp is a browser's name given to exec.LookPath: any of the above, "chrome", and
	// chrome-headless-shell, which comes from the opt-in only.
	browserLookedUp = regexp.MustCompile(`(?i)^(google-chrome.*|chrome|chromium.*|chrome-headless-shell|headless[_-]shell|` +
		`chromedriver|msedge|microsoft-edge.*|firefox|brave.*|safari|opera|vivaldi)([.]exe)?$`)
	// browserVariable is an environment variable that names a browser to run (RT126CHROMEPOPUP B1): CHROMEKILL1's
	// (desktop/headlesschrome_guard_test.go), Puppeteer's and Playwright's, any *_EXECUTABLE_PATH or *_BIN, and one
	// that names another browser. SMOLCORP_TEST_CHROME, the opt-in, is the one a test may read.
	browserVariable = regexp.MustCompile(`(?i)chrom|(^|_)browser$|puppeteer|playwright|_executable_path$|(^|_)bin$|` +
		`firefox|msedge|(^|_)edge(_|$)|safari|brave|opera|vivaldi`)
)

// optInVariable is the one variable a test may read for its browser.
const optInVariable = "SMOLCORP_TEST_CHROME"

// unread stands for a part of a string the guard can't read: a variable's, a call's.
const unread = "\x00"

// pathParts are a string's parts between slashes, backslashes and what can't be read.
func pathParts(words string) []string {
	return strings.FieldsFunc(words, func(r rune) bool { return r == '/' || r == '\\' || r == 0 })
}

// installedBrowser is what words name of an installed browser's program, or "" when they name none.
// chrome-headless-shell is none: the opt-in names it.
func installedBrowser(words string) string {
	for _, part := range pathParts(words) {
		switch part = strings.TrimSpace(part); {
		case browserBundle.MatchString(part):
			return "an installed browser's bundle, " + part
		case browserExe.MatchString(part):
			return "an installed browser's Windows program, " + part
		}
	}
	if parts := pathParts(words); len(parts) > 0 && browserCommand.MatchString(strings.TrimSpace(parts[len(parts)-1])) {
		return "an installed browser's command, " + words
	}
	return ""
}

// reader reads one package's files: their string constants, then what each declaration's strings say.
type reader struct {
	files  map[string]*ast.File
	consts map[string]string
}

// readConstants keeps every constant that is a string, read as far as other constants go. A name two constants have,
// with two values, is kept as neither.
func (r *reader) readConstants() {
	for range 8 { // a constant made of constants, eight deep at most
		values, twice := map[string]string{}, map[string]bool{}
		for _, file := range r.files {
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
						words := r.words(spec.Values[i])
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
		if maps.Equal(values, r.consts) {
			return
		}
		r.consts = values
	}
}

// words is what a string expression says, with what can't be read as unread.
func (r *reader) words(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			s, _ := strconv.Unquote(e.Value)
			return s
		}
		return e.Value
	case *ast.ParenExpr:
		return r.words(e.X)
	case *ast.Ident:
		if s, ok := r.consts[e.Name]; ok {
			return s
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return r.words(e.X) + r.words(e.Y)
		}
	case *ast.CallExpr:
		var args []string
		for _, arg := range e.Args {
			args = append(args, r.words(arg))
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
					parts = append(parts, r.words(elt))
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

// wordsNode is the string a node gives that the guard reads: a string, a sum, a Sprintf, a Join.
func wordsNode(node ast.Node) (ast.Expr, bool) {
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

// fileImports are a file's imports, by the name each is used under.
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

// declName is a top-level declaration's name: a function's ("headlessChrome"), a method's ("(*headlessBrowser).start"),
// or a var's, const's or type's with its keyword ("var deskFormRows").
func declName(decl ast.Decl) string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Recv != nil && len(d.Recv.List) == 1 {
			return "(" + typeName(d.Recv.List[0].Type) + ")." + d.Name.Name
		}
		return d.Name.Name
	case *ast.GenDecl:
		var names []string
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.ValueSpec:
				for _, n := range s.Names {
					names = append(names, n.Name)
				}
			case *ast.TypeSpec:
				names = append(names, s.Name.Name)
			}
		}
		return d.Tok.String() + " " + strings.Join(names, ", ")
	}
	return "?"
}

// typeName is a receiver's type as written: T or *T.
func typeName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + typeName(e.X)
	}
	return "?"
}

// sinks are the calls whose string arguments are words for a person, and the strings functions that only test their
// words: nothing given to them directly is resolved or run.
var sinks = map[string]bool{"Error": true, "Errorf": true, "Fatal": true, "Fatalf": true, "Log": true, "Logf": true,
	"Skip": true, "Skipf": true, "Contains": true, "HasPrefix": true, "HasSuffix": true, "EqualFold": true}

// finding is one thing the guard holds against a file: where, in which declaration (its key on
// installedBrowserWords), what, and the words it's about.
type finding struct{ at, key, what, words string }

// findings reads one package's files (by "<folder>/<file>") and gives what the guard holds.
func findings(fset *token.FileSet, files map[string]*ast.File) []finding {
	r := &reader{files: files, consts: map[string]string{}}
	r.readConstants()
	var out []finding
	for _, name := range slices.Sorted(maps.Keys(files)) {
		imports := fileImports(files[name])
		for _, decl := range files[name].Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
				continue
			}
			key := name + " " + declName(decl)
			add := func(n ast.Node, what, words string) {
				f := finding{at: name + ":" + strconv.Itoa(fset.Position(n.Pos()).Line), key: key, what: what, words: words}
				if !slices.Contains(out, f) {
					out = append(out, f)
				}
			}
			message := map[ast.Expr]bool{}
			var markMessage func(ast.Expr)
			markMessage = func(e ast.Expr) {
				switch e := e.(type) {
				case *ast.BasicLit:
					message[e] = true
				case *ast.ParenExpr:
					markMessage(e.X)
				case *ast.BinaryExpr:
					if e.Op == token.ADD {
						message[e] = true
						markMessage(e.X)
						markMessage(e.Y)
					}
				}
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					name := calleeName(call.Fun)
					if sinks[name[strings.LastIndex(name, ".")+1:]] {
						for _, a := range call.Args {
							markMessage(a)
						}
					}
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 1 {
						if x, ok := sel.X.(*ast.Ident); ok {
							from, words := imports[x.Name], r.words(call.Args[0])
							switch {
							case from == "os/exec" && sel.Sel.Name == "LookPath":
								if parts := pathParts(words); len(parts) > 0 && browserLookedUp.MatchString(parts[len(parts)-1]) {
									add(call, "looks up a browser on the PATH, "+parts[len(parts)-1]+": a test's comes from SMOLCORP_TEST_CHROME only", words)
								}
							case from == "os" && (sel.Sel.Name == "Getenv" || sel.Sel.Name == "LookupEnv"), from == "syscall" && sel.Sel.Name == "Getenv":
								// RT126CHROMEPOPUP B1, its MA: a finder that reads another variable for its browser.
								if words != optInVariable && browserVariable.MatchString(words) {
									add(call, "reads "+words+", a variable that names a browser: a test's comes from SMOLCORP_TEST_CHROME only", words)
								}
							}
						}
					}
				}
				e, ok := wordsNode(n)
				if !ok || message[e] {
					return true
				}
				if words := r.words(e); installedBrowser(words) != "" {
					add(n, installedBrowser(words), words)
					return false
				}
				return true
			})
		}
	}
	return out
}

// goFolders are the module's folders that hold Go files, below root, each by its name in the module ("desktop",
// "internal/sandbox", "" for the module's own): all of them (RT126CHROMEPOPUP N3), but this package's and those Go
// leaves out of a build (testdata, vendor, a name starting with . or _), and node_modules.
func goFolders(root string) ([]string, error) {
	var folders []string
	err := filepath.WalkDir(root, func(at string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, at)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == guardSelf || rel != "." && (strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_") ||
			slices.Contains([]string{"testdata", "vendor", "node_modules"}, d.Name())) {
			return filepath.SkipDir
		}
		if names, _ := filepath.Glob(filepath.Join(at, "*.go")); len(names) > 0 {
			folders = append(folders, strings.TrimPrefix(rel, "."))
		}
		return nil
	})
	return folders, err
}

// readFolder parses a folder's Go files, tests and code, every system's, each by "<as>/<file>".
func readFolder(fset *token.FileSet, dir, as string) (map[string]*ast.File, error) {
	names, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	files := map[string]*ast.File{}
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		key := filepath.Base(name)
		if as != "" {
			key = as + "/" + key
		}
		files[key] = f
	}
	return files, nil
}

// held are the findings no allowance lets through, and the allowances that let one through.
func held(found []finding, allowed map[string]allowance) (out []finding, used map[string]bool) {
	used = map[string]bool{}
	for _, f := range found {
		a, ok := allowed[f.key]
		if ok && (len(a.under) == 0 || slices.ContainsFunc(a.under, func(place string) bool { return strings.HasPrefix(f.words, place) })) {
			used[f.key] = true
			continue
		}
		if ok {
			f.what += ", outside " + strings.Join(a.under, " and ") + ", the only places installedBrowserWords lets this declaration name one"
		}
		out = append(out, f)
	}
	return out, used
}

func TestNoTestRunsAnInstalledBrowserOrLooksForOne(t *testing.T) {
	folders, err := goFolders(guardRoot)
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, folder := range folders {
		fset := token.NewFileSet()
		files, err := readFolder(fset, filepath.Join(guardRoot, filepath.FromSlash(folder)), folder)
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(guardMustRead, folder) && len(files) < 20 {
			t.Fatalf("the Go files in %s: only %d found: the guard is looking in the wrong place", folder, len(files))
		}
		found, allowed := held(findings(fset, files), installedBrowserWords)
		maps.Copy(used, allowed)
		for _, f := range found {
			t.Errorf("%s, in %s: %s. A test's browser is chrome-headless-shell, only where SMOLCORP_TEST_CHROME names one "+
				"(FXCHROMEPOPUP); words that are never resolved or run go on installedBrowserWords, with why", f.at, f.key, f.what)
		}
	}
	for _, folder := range guardMustRead {
		if !slices.Contains(folders, folder) {
			t.Errorf("the guard never read %s: it's looking in the wrong place", folder)
		}
	}
	for _, key := range slices.Sorted(maps.Keys(installedBrowserWords)) {
		if !used[key] || installedBrowserWords[key].why == "" {
			t.Errorf("%q is on installedBrowserWords with nothing of its own to allow, or no reason: take it off", key)
		}
	}
}

// The guard reads every package of the module, not only the two whose tests start a browser (RT126CHROMEPOPUP N3):
// the command's, the sandbox's stand-in tmux, the scripts', the module's own folder.
func TestTheInstalledBrowserGuardReadsEveryPackage(t *testing.T) {
	folders, err := goFolders(guardRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"", "cmd/smolcorp", "desktop", "internal/gm", "internal/sandbox", "internal/sandbox/tmuxfake", "scripts/upgrade"} {
		if !slices.Contains(folders, want) {
			t.Errorf("the guard doesn't read %q (it reads %q)", want, folders)
		}
	}
	if slices.Contains(folders, guardSelf) {
		t.Errorf("the guard reads its own package, whose tables name every browser")
	}
}

// dodges are sources the guard must hold (caught) or pass, each a test file of its own: parsed, never compiled or
// run. The first two are the finders as they were before FXCHROMEPOPUP; the rows marked RT126CHROMEPOPUP are the
// review's dodges (D…) and mutants (M…) of the real tree, with what each now does; "left to" rows are ones this guard
// isn't the one to hold, and says so.
var dodges = []struct {
	what   string
	src    string
	caught bool
}{
	{"the sandbox's headlessChrome, falling back to the installed Google Chrome", "package x\n\nfunc headlessChrome() string {\n\tif chrome := os.Getenv(\"SMOLCORP_TEST_CHROME\"); chrome != \"\" {\n\t\treturn chrome\n\t}\n\treturn \"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome\"\n}\n", true},
	{"the desktop's headlessChrome, looking on the PATH", "package x\n\nimport \"os/exec\"\n\nfunc headlessChrome() string {\n\tfor _, name := range []string{\"google-chrome\", \"chromium\"} {\n\t\tif found, err := exec.LookPath(name); err == nil {\n\t\t\treturn found\n\t\t}\n\t}\n\treturn \"\"\n}\n", true},
	{"Program Files' Chrome, by Join", "package x\n\nfunc f() string { return filepath.Join(os.Getenv(\"ProgramFiles\"), \"Google\", \"Chrome\", \"Application\", \"chrome.exe\") }\n", true},
	{"Chromium's bundle, by Join and +", "package x\n\nfunc f() { os.Stat(filepath.Join(\"/Applications\", \"Chrom\"+\"ium.app\", \"Contents\", \"MacOS\", \"Chromium\")) }\n", true},
	{"a constant of the path, used elsewhere", "package x\n\nconst at = \"/Applications/\" + \"Google Chrome.app\"\n\nfunc f() { os.Stat(at) }\n", true},
	{"Safari's, by Sprintf", "package x\n\nfunc f() { exec.Command(fmt.Sprintf(\"/Applications/%s.app/Contents/MacOS/%s\", \"Safari\", \"Safari\")).Run() }\n", true},
	{"Edge's bundle in the home's Applications", "package x\n\nfunc f(home string) string { return home + \"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge\" }\n", true},
	{"Chrome for Testing's bundle", "package x\n\nfunc f(dir string) string { return filepath.Join(dir, \"Google Chrome for Testing.app\") }\n", true},
	{"chrome-headless-shell looked up on the PATH", "package x\n\nimport \"os/exec\"\n\nfunc f() (string, error) { return exec.LookPath(\"chrome-headless-shell\") }\n", true},
	{"chrome looked up on the PATH, under another name", "package x\n\nimport run \"os/exec\"\n\nfunc f() (string, error) { return run.LookPath(\"chrome\") }\n", true},
	{"a browser's command in a list", "package x\n\nvar browsers = []string{\"google-chrome-stable\"}\n", true},
	{"a start beside a message", "package x\n\nfunc f(t *testing.T) { exec.Command(\"/Applications/Firefox.app/Contents/MacOS/firefox\").Run(); t.Log(\"x\") }\n", true},

	{"RT126CHROMEPOPUP MA: the desktop's headlessChrome reading CHROME_BIN first", "package x\n\nimport \"os\"\n\nfunc headlessChrome() string {\n\tif chrome := os.Getenv(\"CHROME_BIN\"); chrome != \"\" {\n\t\treturn chrome\n\t}\n\tnamed := os.Getenv(\"SMOLCORP_TEST_CHROME\")\n\tif strings.TrimSuffix(filepath.Base(named), \".exe\") != \"chrome-headless-shell\" || bundlePath.MatchString(named) {\n\t\treturn \"\"\n\t}\n\treturn named\n}\n", true},
	{"RT126CHROMEPOPUP MB: the desktop's headlessChrome giving /usr/bin/google-chrome when it's there", "package x\n\nimport \"os\"\n\nfunc headlessChrome() string {\n\tif _, err := os.Stat(\"/usr/bin/google-chrome\"); err == nil {\n\t\treturn \"/usr/bin/google-chrome\"\n\t}\n\treturn \"\"\n}\n", true},
	{"RT126CHROMEPOPUP MD: the sandbox's normal work running CHROME_BIN in a desk", "package x\n\nimport \"os\"\n\nfunc f(t *testing.T, w *escapeDesk, folder string) {\n\tw.work(t, \"\", folder, 5*time.Second, nil, os.Getenv(\"CHROME_BIN\"), \"--headless\", \"--dump-dom\")\n}\n", true},
	{"RT126CHROMEPOPUP D1: the bundle's folder by Join", "package x\n\nfunc f() { os.Stat(filepath.Join(\"/Applications\", \"Google Chrome.app\")) }\n", true},
	{"RT126CHROMEPOPUP D3: google-chrome looked up on the PATH", "package x\n\nimport \"os/exec\"\n\nfunc f() (string, error) { return exec.LookPath(\"google-chrome\") }\n", true},
	{"RT126CHROMEPOPUP D4: exec.Command of CHROME_BIN", "package x\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n)\n\nfunc f() { exec.Command(os.Getenv(\"CHROME_BIN\"), \"--headless\").Run() }\n", true},
	{"RT126CHROMEPOPUP D4b: a stat of PUPPETEER_EXECUTABLE_PATH", "package x\n\nimport \"os\"\n\nfunc f() { os.Stat(os.Getenv(\"PUPPETEER_EXECUTABLE_PATH\")) }\n", true},
	{"Playwright's variable, by LookupEnv", "package x\n\nimport \"os\"\n\nfunc f() (string, bool) { return os.LookupEnv(\"PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH\") }\n", true},
	{"another browser's variable, under another import name", "package x\n\nimport env \"os\"\n\nfunc f() string { return env.Getenv(\"FIREFOX_BIN\") }\n", true},
	{"a program's variable, *_BIN", "package x\n\nimport \"os\"\n\nfunc f() string { return os.Getenv(\"HEADLESS_SHELL_BIN\") }\n", true},
	{"a browser's variable, by syscall and a constant", "package x\n\nimport \"syscall\"\n\nconst v = \"BROWSER\"\n\nfunc f() (string, bool) { return syscall.Getenv(v) }\n", true},
	{"RT126CHROMEPOPUP D9: Chrome Canary's bundle", "package x\n\nvar at = \"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary\"\n", true},
	{"RT126CHROMEPOPUP D10: snap Chromium's command, as a path", "package x\n\nfunc f() { os.Stat(\"/snap/bin/chromium\") }\n", true},
	{"RT126CHROMEPOPUP D12: a raw string", "package x\n\nvar at = `/Applications/Google Chrome.app`\n", true},

	{"left to CHROMEKILL1's and escape3's guards (RT126CHROMEPOPUP D5, MF): open -a with no .app", "package x\n\nfunc f() { exec.Command(\"/usr/bin/open\", \"-a\", \"Google Chrome\", \"--args\", \"--headless\").Run() }\n", false},
	{"left to CHROMEKILL1's and escape3's guards (RT126CHROMEPOPUP D5b): open -b by bundle id", "package x\n\nfunc f() { exec.Command(\"/usr/bin/open\", \"-b\", \"com.google.Chrome\").Run() }\n", false},
	{"left to review (RT126CHROMEPOPUP D6): a name split across a variable", "package x\n\nfunc f() { name := \"Google Chrome\"; os.Stat(\"/Applications/\" + name + \".app\") }\n", false},
	{"left to review (RT126CHROMEPOPUP D8): a helper named like a message", "package x\n\nfunc Log(p string) { exec.Command(p).Run() }\n\nfunc f() { Log(\"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome\") }\n", false},
	{"left to review (RT126CHROMEPOPUP D11): Chrome's own folder on Linux, its program named chrome", "package x\n\nfunc f() { os.Stat(\"/opt/google/chrome/chrome\") }\n", false},

	{"control: the desktop's headlessChrome now", "package x\n\nimport \"os\"\n\nfunc headlessChrome() string {\n\tnamed := os.Getenv(\"SMOLCORP_TEST_CHROME\")\n\tif strings.TrimSuffix(filepath.Base(named), \".exe\") != \"chrome-headless-shell\" || bundlePath.MatchString(named) {\n\t\treturn \"\"\n\t}\n\treturn named\n}\n", false},
	{"control: the sandbox's headlessChrome now", "package x\n\nimport \"os\"\n\nfunc headlessChrome() string { return os.Getenv(\"SMOLCORP_TEST_CHROME\") }\n", false},
	{"control: a skip's message", "package x\n\nfunc f(t *testing.T) { t.Skip(\"no Google Chrome.app here\") }\n", false},
	{"control: a log line checked for a word", "package x\n\nfunc f(out string) bool { return strings.Contains(out, \"Chromium.app\") }\n", false},
	{"control: a profile folder named chrome", "package x\n\nfunc f(dir string) string { return \"--user-data-dir=\" + filepath.Join(dir, \"chrome\") }\n", false},
	{"control: a LookPath of another package's", "package x\n\nfunc f() (string, error) { return mine.LookPath(\"chrome\") }\n", false},
	{"control: git looked up on the PATH", "package x\n\nimport \"os/exec\"\n\nfunc f(name string) (string, error) { exec.LookPath(name); return exec.LookPath(\"git\") }\n", false},
	{"control: another app's bundle", "package x\n\nvar app = \"/System/Applications/Calculator.app\"\n", false},
	{"control: the stand-in browser's variable, by a constant", "package x\n\nimport \"os\"\n\nconst holder = \"SMOLCORP_TEST_BROWSER_HOLDER\"\n\nfunc f() string { return os.Getenv(holder) + os.Getenv(\"SHELL\") }\n", false},
	{"control: a Getenv of another package's", "package x\n\nfunc f() string { return mine.Getenv(\"CHROME_BIN\") }\n", false},
	{"control: an outfit named safari", "package x\n\nvar teamChoices = []string{\"gymkit\", \"safari\", \"flannel\"}\n", false},
	{"control: a folder named firefox inside a path", "package x\n\nvar at = \"/snap/firefox/common/profile\"\n", false},
}

// dodgesAcrossFiles are dodges a package's files make together, each set of files a folder of its own: read as the
// guard reads a package, never compiled or run.
var dodgesAcrossFiles = []struct {
	what   string
	files  map[string]string
	caught bool
}{
	{"RT126CHROMEPOPUP MC: the desktop's headlessChrome falling back to a helper in the package's own code", map[string]string{
		"headlesschrome_test.go": "package x\n\nfunc headlessChrome() string { return rtInstalledChrome() }\n",
		"zz_rtbrowser.go":        "package x\n\nimport \"os\"\n\nfunc rtInstalledChrome() string {\n\tconst chrome = \"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome\"\n\tif _, err := os.Stat(chrome); err == nil {\n\t\treturn chrome\n\t}\n\treturn \"\"\n}\n",
	}, true},
	{"RT126CHROMEPOPUP ME: the sandbox's normal work running a constant of the package's own code", map[string]string{
		"escape2_darwin_test.go": "package x\n\nfunc f(t *testing.T, w *escapeDesk, folder string) { w.work(t, \"\", folder, 5*time.Second, nil, rtChrome, \"--headless\") }\n",
		"zz_rt_darwin.go":        "package x\n\nconst rtChrome = \"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome\"\n",
	}, true},
	{"RT126CHROMEPOPUP D2: a constant in one file, its bundle in another", map[string]string{
		"a_test.go": "package x\n\nconst chromeName = \"Google Chrome\"\n",
		"b_test.go": "package x\n\nfunc f() { os.Stat(filepath.Join(\"/Applications\", chromeName+\".app\")) }\n",
	}, true},
	{"RT126CHROMEPOPUP D3b: a LookPath of a constant from another file", map[string]string{
		"a_test.go": "package x\n\nconst g = \"google-\"\n",
		"b_test.go": "package x\n\nimport \"os/exec\"\n\nfunc f() { exec.LookPath(g + \"chrome\") }\n",
	}, true},
	{"control: MC's test file alone", map[string]string{
		"headlesschrome_test.go": "package x\n\nfunc headlessChrome() string { return rtInstalledChrome() }\n",
	}, false},
	{"control: words in a file that isn't Go", map[string]string{
		"x_test.go": "package x\n",
		"notes.txt": "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome\n",
	}, false},
}

func TestTheInstalledBrowserGuardHoldsEachDodge(t *testing.T) {
	type dodge struct {
		what   string
		files  map[string]string
		caught bool
	}
	var all []dodge
	for _, c := range dodges {
		all = append(all, dodge{c.what, map[string]string{"x_test.go": c.src}, c.caught})
	}
	for _, c := range dodgesAcrossFiles {
		all = append(all, dodge{c.what, c.files, c.caught})
	}
	for _, c := range all {
		dir := t.TempDir()
		for name, src := range c.files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		fset := token.NewFileSet()
		files, err := readFolder(fset, dir, "x")
		if err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		found := findings(fset, files)
		if caught := len(found) > 0; caught != c.caught {
			t.Errorf("the guard on %s: held %v, want held %v (%v)", c.what, caught, c.caught, found)
		}
	}
}

// An allowance with places lets through only what's in them (RT126CHROMEPOPUP N2): a real installed path added
// later to refusalProbes, beside its missing ones, is held.
func TestAnAllowanceLetsThroughOnlyItsOwnPlaces(t *testing.T) {
	src := "package x\n\nvar refusalProbes = []string{\n\t\"<gone>/Google Chrome.app/Contents/MacOS/Google Chrome\",\n\t\"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome\",\n}\n\nvar deskFormRows = []string{\"/Applications/Chromium.app/Contents/MacOS/Chromium\"}\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x_test.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]allowance{
		"x/x_test.go var refusalProbes": {why: "missing paths", under: []string{"<gone>/"}},
		"x/x_test.go var deskFormRows":  {why: "words only"},
	}
	found, used := held(findings(fset, map[string]*ast.File{"x/x_test.go": f}), allowed)
	if len(found) != 1 || found[0].at != "x/x_test.go:5" || !strings.Contains(found[0].what, "outside <gone>/") {
		t.Errorf("held %v, want only the installed path on line 5, as outside <gone>/", found)
	}
	if !used["x/x_test.go var refusalProbes"] || !used["x/x_test.go var deskFormRows"] {
		t.Errorf("the allowances used: %v, want both", used)
	}
}
