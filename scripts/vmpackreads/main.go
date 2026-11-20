// Command vmpackreads lists the repo's files that a package's tests read through ../, outside their own package,
// for scripts/vmpack.sh to put at the same place in a VM pack (FXVMPACK: the shims rows read
// ../../Resources/shims, which no pack had, so they failed on the VM).
//
//	go run ./scripts/vmpackreads -root DIR [-goos linux] [-goarch arm64] [-cgo] [-listed PKG]… PKG[=RUN]…
//
// DIR is a tree's top folder (vmpack's export of it), each PKG a package folder in it, and RUN the -test.run
// pattern the pack runs it with (every test when not given). Only the files that build for -goos/-goarch are
// read, as Go source, and in them only what the tests that run can reach: the matching Test functions, TestMain,
// the init functions and package-level values, and each function, method, var or const they name, by name. In
// that code, what is named through ../ is found from:
//
//   - a string that starts with ../ ("../Resources/web/setup.html");
//   - filepath.Join or path.Join whose first words are constants (strings, or the package's string consts):
//     filepath.Join("..", "Tests", "fixtures", "briefs", "briefs.json"); when a word known only at run time
//     follows, the folder before it is what is read (filepath.Join("..", "..", "Resources", "shims", name)).
//
// A path the tree has, outside the package and inside the repo, is printed: "PKG PATH", one a line. A path that
// climbs to the repo's top and goes into one of its folders, but the tree hasn't, stops it (a test that would fail
// on the VM for it). Anything else is a stand-in's word, not a read: a link's target in a temp folder, a path
// above the repo, a top folder the repo hasn't (dotfiles, outside), a build output (build/ isn't in a tree).
//
// What it can't see stops it too: a test that holds the repo's top folder itself (const repo = "../../..",
// filepath.Join("..", "..") kept or handed on) or a folder above its package followed by a word known only at run
// time names what it reads there at run time. Such a package must have its reads listed by hand in vmpack.sh and be
// named with -listed; its sites are then left to that list.
//
// What it doesn't see at all: a Join from a word known only at run time, as filepath.Join(wd, "..", "..", "docs")
// or filepath.Join(wd, "../../go.sum") with wd from os.Getwd. Its constant words go below that word, wherever it is,
// so they are neither read nor stopped on; such a read must be listed by hand in vmpack.sh.
//
// It reads only DIR's Go files, and writes only to stdout and stderr. Exit 1 on what stops a pack, 2 on usage.
package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, " ") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

func main() {
	root := flag.String("root", "", "the tree's top folder")
	goos := flag.String("goos", "linux", "the test binaries' GOOS")
	goarch := flag.String("goarch", "arm64", "the test binaries' GOARCH")
	cgo := flag.Bool("cgo", false, "the test binaries are built with cgo")
	var listed listFlag
	flag.Var(&listed, "listed", "a package whose run-time reads vmpack.sh lists by hand (repeatable)")
	flag.Parse()
	if *root == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Usage: go run ./scripts/vmpackreads -root DIR [-goos linux] [-goarch arm64] [-cgo] [-listed PKG]… PKG[=RUN]…")
		os.Exit(2)
	}
	ctx := build.Default
	ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = *goos, *goarch, *cgo
	os.Exit(run(ctx, *root, listed, flag.Args(), os.Stdout, os.Stderr))
}

// run prints each package's reads to out, and what stops the pack to errs; it gives the exit status.
func run(ctx build.Context, root string, listed, pkgs []string, out, errs io.Writer) int {
	status := 0
	for _, arg := range pkgs {
		pkg, pattern, _ := strings.Cut(arg, "=")
		pkg = path.Clean(filepath.ToSlash(pkg))
		runs, err := regexp.Compile(pattern)
		if err != nil {
			fmt.Fprintf(errs, "vmpack: %s: its run pattern: %v\n", pkg, err)
			status = 1
			continue
		}
		found, err := scan(ctx, root, pkg, runs)
		if err != nil {
			fmt.Fprintf(errs, "vmpack: %s: %v\n", pkg, err)
			status = 1
			continue
		}
		for _, m := range found.missing {
			fmt.Fprintf(errs, "vmpack: %s reads %s (%s), which the tree doesn't have\n", pkg, m.path, m.at)
			status = 1
		}
		if !slices.Contains(listed, pkg) {
			for _, at := range found.runtime {
				fmt.Fprintf(errs, "vmpack: %s names at run time what it reads above its folder (%s): list its reads in vmpack.sh's reads()\n", pkg, at)
				status = 1
			}
		}
		for _, p := range found.reads {
			fmt.Fprintf(out, "%s %s\n", pkg, p)
		}
	}
	return status
}

type missingRead struct{ path, at string }

type found struct {
	reads   []string      // repo paths the tree has, sorted, each once
	missing []missingRead // repo paths the tree hasn't
	runtime []string      // file:line of each read named at run time
}

// scan reads pkg's files for ctx and sorts what the tests runs matches reach name through ../.
func scan(ctx build.Context, root, pkg string, runs *regexp.Regexp) (found, error) {
	dir := filepath.Join(root, filepath.FromSlash(pkg))
	bp, err := ctx.ImportDir(dir, 0)
	var none *build.NoGoError
	if err != nil && !errors.As(err, &none) {
		return found{}, err
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range slices.Concat(bp.GoFiles, bp.TestGoFiles, bp.XTestGoFiles) {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return found{}, err
		}
		files = append(files, f)
	}
	consts, funcs := stringConsts(files), testFuncs(fset, files)
	var res found
	seen := map[string]bool{}
	for _, node := range reached(fset, files, runs) {
		for _, c := range candidates(node, consts, funcs) {
			pos := fset.Position(c.pos)
			at := fmt.Sprintf("%s/%s:%d", pkg, filepath.Base(pos.Filename), pos.Line)
			switch kind, rel := classify(root, pkg, c); kind {
			case isRead:
				if !seen[rel] {
					seen[rel] = true
					res.reads = append(res.reads, rel)
				}
			case isMissing:
				res.missing = append(res.missing, missingRead{rel, at})
			case isRuntime:
				res.runtime = append(res.runtime, at)
			}
		}
	}
	slices.Sort(res.reads)
	return res, nil
}

// reached is the test files' code the tests runs matches can run, in the files' order: from those tests, TestMain,
// the init functions and package-level vars, each top-level function, method (by its name, whatever its type), var
// and const of the test files a reached piece names. The package's own code isn't read: it names no repo file
// through ../ (its ".." are a path's checks, filepath.Rel's answers), and a test's cwd is no place of its.
func reached(fset *token.FileSet, files []*ast.File, runs *regexp.Regexp) []ast.Node {
	named := map[string][]ast.Node{}
	var todo []ast.Node
	for _, f := range files {
		if !strings.HasSuffix(fset.Position(f.Package).Filename, "_test.go") {
			continue
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				named[d.Name.Name] = append(named[d.Name.Name], d)
				name := d.Name.Name
				if d.Recv == nil && (name == "TestMain" || name == "init" ||
					strings.HasPrefix(name, "Test") && runs.MatchString(name)) {
					todo = append(todo, d)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, n := range vs.Names {
						named[n.Name] = append(named[n.Name], vs)
					}
					if d.Tok == token.VAR {
						todo = append(todo, vs)
					}
				}
			}
		}
	}
	done := map[ast.Node]bool{}
	for len(todo) > 0 {
		n := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if done[n] {
			continue
		}
		done[n] = true
		ast.Inspect(n, func(m ast.Node) bool {
			if id, ok := m.(*ast.Ident); ok {
				todo = append(todo, named[id.Name]...)
			}
			return true
		})
	}
	var out []ast.Node
	for _, f := range files {
		ast.Inspect(f, func(m ast.Node) bool {
			if done[m] {
				out = append(out, m)
				return false
			}
			return true
		})
	}
	return out
}

// stringConsts are the string consts of the package's files, by name; a name given two values is left out.
func stringConsts(files []*ast.File) map[string]string {
	consts, twice := map[string]string{}, map[string]bool{}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			decl, ok := n.(*ast.GenDecl)
			if !ok || decl.Tok != token.CONST {
				return true
			}
			for _, spec := range decl.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					if s, ok := literal(vs.Values[i], nil); ok {
						if old, there := consts[name.Name]; there && old != s {
							twice[name.Name] = true
						}
						consts[name.Name] = s
					}
				}
			}
			return true
		})
	}
	for name := range twice {
		delete(consts, name)
	}
	return consts
}

// literal is e's value when it is a constant string: a string, a const of consts, or their sum.
func literal(e ast.Expr, consts map[string]string) (string, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			s, err := strconv.Unquote(e.Value)
			return s, err == nil
		}
	case *ast.Ident:
		s, ok := consts[e.Name]
		return s, ok
	case *ast.ParenExpr:
		return literal(e.X, consts)
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			x, okX := literal(e.X, consts)
			y, okY := literal(e.Y, consts)
			return x + y, okX && okY
		}
	}
	return "", false
}

// candidate is a path named through ../, slash-separated: whether a word known only at run time follows it, whether
// it is kept or handed on rather than said (held: a var's, const's or assignment's value, as const repo = "../../..",
// a call's argument, as releaseFilesIn(t, ".."), a return), where it is, and how many ../ it starts with.
type candidate struct {
	path       string
	open, held bool
	pos        token.Pos
	climbed    int
}

// pathWord is what a path a test reads looks like; a word with a space, a colon, a line's end or a $ is a stand-in's
// text, not a path to read.
var pathWord = regexp.MustCompile(`^[A-Za-z0-9._/@+*?\[\]-]+$`)

// candidates are the paths through ../ in n: its Join calls' constant first words, then its sums with a constant
// head ("../../" + "x", "../../" + name), then its other strings.
func candidates(n ast.Node, consts map[string]string, funcs map[string][]*ast.FuncDecl) []candidate {
	var out []candidate
	used := map[ast.Node]bool{}
	add := func(p string, open, held bool, pos token.Pos) {
		if !pathWord.MatchString(p) {
			return
		}
		climbed := 0
		for _, seg := range strings.Split(p, "/") {
			if seg != ".." {
				break
			}
			climbed++
		}
		if climbed > 0 {
			out = append(out, candidate{p, open, held, pos, climbed})
		}
	}
	var joinStack []ast.Node
	ast.Inspect(n, func(m ast.Node) bool {
		if m == nil {
			joinStack = joinStack[:len(joinStack)-1]
			return true
		}
		var parent ast.Node
		if len(joinStack) > 0 {
			parent = joinStack[len(joinStack)-1]
		}
		joinStack = append(joinStack, m)
		call, ok := m.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Join" {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); !ok || (x.Name != "filepath" && x.Name != "path") {
			return true
		}
		var words []string
		open := false
		for _, arg := range call.Args {
			s, ok := literal(arg, consts)
			if open || !ok {
				// The constant words after one known at run time go below it, not from the test's folder.
				if open = true; ok {
					markUsed(arg, used)
				}
				continue
			}
			words = append(words, filepath.ToSlash(s))
			markUsed(arg, used)
		}
		if len(words) > 0 {
			// A Join that is only a folder above (filepath.Join("..", "..")) is held or said by its place, as the
			// same folder spelled "../.." is.
			p := path.Join(words...)
			add(p, open, climbOnly(p) && !wordPlace(parent, call, funcs, map[*ast.FuncDecl]bool{}), call.Pos())
		}
		return true
	})
	var stack []ast.Node
	ast.Inspect(n, func(m ast.Node) bool {
		if m == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		var parent ast.Node
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, m)
		if used[m] {
			return true
		}
		switch m := m.(type) {
		case *ast.BinaryExpr:
			// A sum is read whole, from its first operand: "../../" + "x" is ../../x; "../../Resources/" + name reads
			// the folder before the word known at run time, wherever the sum is (a list's row, a field's value), and
			// "../../" + name a folder above held at run time.
			if m.Op != token.ADD {
				return true
			}
			head, whole := sumHead(m, consts)
			if head != ".." && !strings.HasPrefix(head, "../") {
				return true
			}
			markUsed(m, used)
			p := head
			if !whole && !strings.HasSuffix(p, "/") && path.Base(p) != ".." {
				p = path.Dir(p) // "../../Tests/fix" + kind reads in ../../Tests
			}
			p = path.Clean(p)
			said := wordPlace(parent, m, funcs, map[*ast.FuncDecl]bool{})
			if !whole && said && climbOnly(p) {
				return true // strings.HasPrefix(rel, ".."+sep): a word
			}
			add(p, !whole, !said, m.Pos())
		case *ast.BasicLit:
			if m.Kind != token.STRING {
				return true
			}
			if s, err := strconv.Unquote(m.Value); err == nil && (s == ".." || strings.HasPrefix(s, "../")) {
				add(path.Clean(s), false, !wordPlace(parent, m, funcs, map[*ast.FuncDecl]bool{}), m.Pos())
			}
		}
		return true
	})
	return out
}

// climbOnly says whether the clean path p is only a folder above: "..", "../..", nothing named in it.
func climbOnly(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg != ".." {
			return false
		}
	}
	return true
}

// sumHead is the constant first operands of a sum, added up, and whether that is all of it.
func sumHead(e ast.Expr, consts map[string]string) (string, bool) {
	if s, ok := literal(e, consts); ok {
		return s, true
	}
	switch e := e.(type) {
	case *ast.ParenExpr:
		return sumHead(e.X, consts)
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			x, whole := sumHead(e.X, consts)
			if !whole {
				return x, false
			}
			y, _ := sumHead(e.Y, consts)
			return x + y, false
		}
	}
	return "", false
}

// testFuncs are the test files' functions and methods, by name.
func testFuncs(fset *token.FileSet, files []*ast.File) map[string][]*ast.FuncDecl {
	funcs := map[string][]*ast.FuncDecl{}
	for _, f := range files {
		if !strings.HasSuffix(fset.Position(f.Package).Filename, "_test.go") {
			continue
		}
		for _, decl := range f.Decls {
			if d, ok := decl.(*ast.FuncDecl); ok && d.Body != nil {
				funcs[d.Name.Name] = append(funcs[d.Name.Name], d)
			}
		}
	}
	return funcs
}

// wordPlace says whether e, below parent, is only said, never kept or handed on: a side of a comparison, a sum's tail,
// a case's value, one of a list's or a map's keys or values, a value given to a field (p.OwnID = ".."), a link's
// target, an argument of the strings, bytes, slices, fmt, errors or strconv package or of a test's Error, Fatal, Log
// or Skip, or an argument of a function of the test files that only says it (r.refused(path, "..") puts it in its
// messages). A folder above (".." or "../..") anywhere else is held, so what is read in it is named at run time: a
// var's, const's or assignment's value, an argument of anything else (releaseFilesIn(t, ".."), filepath.Abs("..")),
// a return.
func wordPlace(parent ast.Node, e ast.Expr, funcs map[string][]*ast.FuncDecl, seen map[*ast.FuncDecl]bool) bool {
	switch p := parent.(type) {
	case *ast.BinaryExpr:
		return p.Op != token.ADD || p.X != e // a sum's head is a path's start: root + "/x"
	case *ast.CaseClause:
		return slices.Contains(p.List, e)
	case *ast.CompositeLit, *ast.KeyValueExpr:
		return true
	case *ast.AssignStmt:
		i := slices.Index(p.Rhs, e)
		if len(p.Lhs) != len(p.Rhs) || i < 0 {
			return false
		}
		_, field := p.Lhs[i].(*ast.SelectorExpr)
		return field
	case *ast.CallExpr:
		i := slices.Index(p.Args, e)
		if i < 0 {
			return false
		}
		var name string
		switch fun := p.Fun.(type) {
		case *ast.Ident:
			name = fun.Name
		case *ast.SelectorExpr:
			name = fun.Sel.Name
			if x, ok := fun.X.(*ast.Ident); ok {
				switch x.Name {
				case "strings", "bytes", "slices", "fmt", "errors", "strconv":
					return true
				case "os":
					return name == "Symlink" && i == 0 // a link's target is only written down
				case "filepath", "path", "exec", "fs", "io", "ioutil":
					return false
				}
			}
			for _, verb := range []string{"Error", "Fatal", "Log", "Skip"} {
				if strings.HasPrefix(name, verb) {
					return true
				}
			}
		}
		decls := funcs[name]
		if len(decls) == 0 {
			return false // the package's own code or another package's: what it does with it isn't read
		}
		for _, d := range decls {
			if !paramIsWord(d, i, funcs, seen) {
				return false
			}
		}
		return true
	}
	return false
}

// paramIsWord says whether d only says its i-th argument (see wordPlace) wherever it uses it.
func paramIsWord(d *ast.FuncDecl, i int, funcs map[string][]*ast.FuncDecl, seen map[*ast.FuncDecl]bool) bool {
	if seen[d] {
		return true // already being looked at, higher up
	}
	seen[d] = true
	defer delete(seen, d)
	var names []string // each parameter's name, "" for one without
	variadic := false
	for _, field := range d.Type.Params.List {
		if len(field.Names) == 0 {
			names = append(names, "")
		}
		for _, id := range field.Names {
			names = append(names, id.Name)
		}
		_, variadic = field.Type.(*ast.Ellipsis)
	}
	var name string
	switch {
	case i < len(names):
		name = names[i]
	case variadic && len(names) > 0:
		name = names[len(names)-1]
	}
	if name == "" || name == "_" {
		return true
	}
	word := true
	var stack []ast.Node
	ast.Inspect(d.Body, func(m ast.Node) bool {
		if m == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if id, ok := m.(*ast.Ident); ok && id.Name == name && len(stack) > 0 {
			if sel, ok := stack[len(stack)-1].(*ast.SelectorExpr); !ok || sel.X == id {
				word = word && wordPlace(stack[len(stack)-1], id, funcs, seen)
			}
		}
		stack = append(stack, m)
		return true
	})
	return word
}

func markUsed(n ast.Node, used map[ast.Node]bool) {
	ast.Inspect(n, func(m ast.Node) bool {
		if m != nil {
			used[m] = true
		}
		return true
	})
}

type kind int

const (
	isWord kind = iota
	isRead
	isMissing
	isRuntime
)

// classify says what c is, from pkg in the tree at root, and the repo path it names.
func classify(root, pkg string, c candidate) (kind, string) {
	p := c.path
	// A glob reads what is under the folder before its first pattern.
	if i := strings.IndexAny(p, "*?["); i >= 0 {
		p, c.open = path.Dir(p[:i]+"x"), true
	}
	rel := path.Join(pkg, p)
	depth := strings.Count(pkg, "/") + 1
	onlyUp := c.climbed == strings.Count(p, "/")+1 // "..", "../..": a folder above, nothing named in it
	switch {
	case rel == ".." || strings.HasPrefix(rel, "../"):
		return isWord, rel // above the repo
	case onlyUp && (c.open || c.held):
		return isRuntime, rel // a folder above held, or joined with a word known at run time: what is read is named then
	case onlyUp:
		return isWord, rel // ".." as a word (rel == "..", "own id .."), not a read
	case rel == pkg || strings.HasPrefix(rel, pkg+"/"):
		return isWord, rel // its own package: vmpack copies its testdata
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
		return isRead, rel
	}
	top := strings.SplitN(rel, "/", 2)[0]
	if _, err := os.Stat(filepath.Join(root, top)); err == nil && c.climbed == depth && top != "build" {
		return isMissing, rel
	}
	return isWord, rel
}
