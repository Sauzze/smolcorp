package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// The release gate's row for the Mac app's Swift (SA-M9): the app never runs tmux but through TmuxClient, which tells it
// SMOLCORP_TMUX_SERVER, a server from the core or "unchecked", with nothing of the app's own environment. It reads the
// app's Swift files (Sources/AgentDeck), as tmuxExecProblems reads the core's Go, and holds closed lists, by file, type
// and function: every place a tmux program is named or picked, every start of a program (Tool.run, Disclaimed.process)
// with what it runs, and every TmuxClient call with the server it tells. A new site, or one that changed, fails until it
// is listed here with what it does; so does a TmuxClient or a Tool.run that stops setting the guard or the environment.
// HostBridgeTests and TmuxTests (Swift, on the Mac Studio) run the same code on stand-ins.

// swiftSource is one Swift file: its text with comments blanked (strings kept), the same with string contents blanked
// too (for braces and declarations), and its declarations' bodies.
type swiftSource struct {
	name, code, bare string
	decls            []swiftDecl
}

// swiftDecl is a type's or a function's body: from its "{" to its "}", as offsets in the file.
type swiftDecl struct {
	kind, name string
	from, to   int
}

// swiftStringEnd is the offset just past the Swift string literal starting at i ("…", """…""", #"…"#, with \( … )
// interpolations, which may hold strings of their own), and the offsets of its contents.
func swiftStringEnd(src string, i int) (end int, contents [][2]int) {
	hashes := 0
	for i+hashes < len(src) && src[i+hashes] == '#' {
		hashes++
	}
	j := i + hashes
	if j >= len(src) || src[j] != '"' {
		return i + 1, nil
	}
	quote := `"`
	if strings.HasPrefix(src[j:], `"""`) {
		quote = `"""`
	}
	closing := quote + strings.Repeat("#", hashes)
	start := j + len(quote)
	for k := start; k < len(src); {
		switch {
		case strings.HasPrefix(src[k:], closing):
			return k + len(closing), append(contents, [2]int{start, k})
		case src[k] == '\\' && hashes == 0 && k+1 < len(src) && src[k+1] == '(':
			contents = append(contents, [2]int{start, k})
			depth, m := 1, k+2
			for m < len(src) && depth > 0 {
				switch src[m] {
				case '"', '#':
					if src[m] == '#' && (m+1 >= len(src) || src[m+1] != '"' && src[m+1] != '#') {
						m++
						continue
					}
					var inner [][2]int
					m, inner = swiftStringEnd(src, m)
					contents = append(contents, inner...)
					continue
				case '(':
					depth++
				case ')':
					depth--
				}
				m++
			}
			k, start = m, m
		case src[k] == '\\' && hashes == 0:
			k += 2
		default:
			k++
		}
	}
	return len(src), append(contents, [2]int{start, len(src)})
}

// parseSwift reads a Swift file's comments, strings and declarations.
func parseSwift(name, src string) swiftSource {
	code, bare := []byte(src), []byte(src)
	blank := func(b []byte, from, to int) {
		for k := from; k < to && k < len(b); k++ {
			if b[k] != '\n' {
				b[k] = ' '
			}
		}
	}
	for i := 0; i < len(src); {
		switch {
		case strings.HasPrefix(src[i:], "//"):
			end := len(src)
			if j := strings.IndexByte(src[i:], '\n'); j >= 0 {
				end = i + j
			}
			blank(code, i, end)
			blank(bare, i, end)
			i = end
		case strings.HasPrefix(src[i:], "/*"):
			end := len(src)
			if j := strings.Index(src[i+2:], "*/"); j >= 0 {
				end = i + 2 + j + 2
			}
			blank(code, i, end)
			blank(bare, i, end)
			i = end
		case src[i] == '"' || src[i] == '#' && i+1 < len(src) && (src[i+1] == '"' || src[i+1] == '#'):
			end, contents := swiftStringEnd(src, i)
			for _, c := range contents {
				blank(bare, c[0], c[1])
			}
			i = end
		default:
			i++
		}
	}
	s := swiftSource{name: name, code: string(code), bare: string(bare)}
	for _, m := range swiftDeclPattern.FindAllStringSubmatchIndex(s.bare, -1) {
		kind, name := s.bare[m[2]:m[3]], ""
		if m[4] >= 0 {
			name = s.bare[m[4]:m[5]]
		}
		if strings.HasPrefix(kind, "init") {
			kind, name = "func", "init"
		}
		// The body: the first "{" outside the signature's parentheses and brackets, to its own "}".
		depth, open := 0, -1
	scan:
		for k := m[1]; k < len(s.bare); k++ {
			switch s.bare[k] {
			case '(', '[':
				depth++
			case ')', ']':
				depth--
			case '{':
				if depth == 0 {
					open = k
					break scan
				}
			case '}', ';':
				if depth == 0 {
					break scan // a declaration with no body (a protocol's)
				}
			}
		}
		if open < 0 {
			continue
		}
		level, close := 0, len(s.bare)
		for k := open; k < len(s.bare); k++ {
			if s.bare[k] == '{' {
				level++
			} else if s.bare[k] == '}' {
				if level--; level == 0 {
					close = k
					break
				}
			}
		}
		s.decls = append(s.decls, swiftDecl{kind: kind, name: name, from: open, to: close})
	}
	return s
}

// swiftDeclPattern is a declaration at the start of a line, after its attributes and modifiers.
var swiftDeclPattern = regexp.MustCompile(`(?m)^[ \t]*(?:(?:@\w+|public|private|fileprivate|internal|open|static|final|override|required|` +
	`convenience|mutating|nonisolated|indirect|lazy)\s+)*(func|struct|enum|class|extension|actor|protocol|init[?!]?)\b\s*([A-Za-z_][A-Za-z0-9_.]*)?`)

// where is the type and function holding offset at, as "Type.function" ("" for a part not inside one).
func (s swiftSource) where(at int) string {
	var typ, fn swiftDecl
	for _, d := range s.decls {
		if d.from < at && at < d.to {
			if d.kind == "func" {
				if fn.to == 0 || d.to-d.from < fn.to-fn.from {
					fn = d
				}
			} else if typ.to == 0 || d.to-d.from < typ.to-typ.from {
				typ = d
			}
		}
	}
	return typ.name + "." + fn.name
}

// callArguments is the text of each top-level argument of the call whose "(" is at open, in the comment-blanked code.
func (s swiftSource) callArguments(open int) []string {
	var args []string
	depth, from := 0, open+1
	for k := open; k < len(s.bare); k++ {
		switch s.bare[k] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth--; depth == 0 {
				return append(args, strings.Join(strings.Fields(s.code[from:k]), " "))
			}
		case ',':
			if depth == 1 {
				args = append(args, strings.Join(strings.Fields(s.code[from:k]), " "))
				from = k + 1
			}
		}
	}
	return args
}

// readSwiftApp reads the Mac app's Swift files: none read is a failure, never an empty pass.
func readSwiftApp(t testing.TB) []swiftSource {
	t.Helper()
	dir := filepath.Join("..", "Sources", "AgentDeck")
	files, err := filepath.Glob(filepath.Join(dir, "*.swift"))
	if err != nil || len(files) < 40 {
		t.Fatalf("the Mac app's Swift files in %s: %d, %v", dir, len(files), err)
	}
	var sources []swiftSource
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		sources = append(sources, parseSwift(filepath.Base(file), string(data)))
	}
	return sources
}

// The rows' patterns: a start of a program, a tmux named or picked, a TmuxClient call. Each also caught when taken as a
// value (`let run = Tool.run`), which no list allows.
var (
	swiftStartCall  = regexp.MustCompile(`\b(Tool\s*\.\s*run|Disclaimed\s*\.\s*process)\s*\(`)
	swiftStartValue = regexp.MustCompile(`\b(Tool\s*\.\s*run|Disclaimed\s*\.\s*process|TmuxClient\s*\.\s*run)\b\s*[^\s(]`)
	swiftTmuxNamed  = regexp.MustCompile(`\bTmux\s*\.\s*(path|bundled|find|resourcesTmux|places)\b`)
	swiftTmuxPath   = regexp.MustCompile(`bin/tmux`)
	swiftClientCall = regexp.MustCompile(`\bTmuxClient\s*\.\s*run\s*\(`)
)

// swiftTmuxSites is every place the app names or picks a tmux program, by "file Type.function name", with how many
// times: which tmux a site picks is part of the row, so a checked desk's Tmux.bundled turned into Tmux.path, or into a
// path of its own, fails it.
var swiftTmuxSites = map[string]int{
	"TerminalDrivers.swift Tmux.places bin/tmux":        5, // the bundle's, then the usual places
	"TerminalDrivers.swift Tmux.resourcesTmux bin/tmux": 1, // the bundle's, held to Tool.isTrusted
	"TerminalDrivers.swift Tmux.run Tmux.path":          2, // a tmux of yours, or a desk from before the sandbox
	"TerminalDrivers.swift Tmux.run Tmux.bundled":       1, // a desk whose server the core checked: the bundled tmux alone
	"TerminalDriver.swift Terminals.driver Tmux.path":   1, // whether there's a tmux at all: nothing run
	"HireTmux.swift HireTmux.init Tmux.path":            1, // an old desk's -L server, "unchecked", through TmuxClient
}

// swiftStartSites is every start of a program in the app through Tool.run or Disclaimed.process, by "file
// Type.function program". None runs tmux but TmuxClient.run, whose program the TmuxClient rows hold.
var swiftStartSites = map[string]int{
	"HireTmux.swift TmuxClient.run Tool.run(program":                               1,
	"TerminalDrivers.swift Tool.run Disclaimed.process(URL(fileURLWithPath: path)": 1,
	"TerminalDrivers.swift Firing.fire Tool.run(\"/bin/ps\"":                       1,
	"TerminalDrivers.swift Firing.root Tool.run(\"/bin/ps\"":                       1,
	"TerminalDrivers.swift WezTerm.run Tool.run(\"/usr/bin/env\"":                  1,
	"TerminalDrivers.swift Kitty.run Tool.run(path":                                1,
	"TerminalDrivers.swift Kitty.window Tool.run(\"/bin/ps\"":                      1,
	"Terminal.swift Terminal.place Tool.run(\"/bin/ps\"":                           1,
	"OpenCodeInstall.swift OpenCodeInstall.loginShellPath Disclaimed.process(URL(fileURLWithPath: ProcessInfo.processInfo.environment[\"SHELL\"] ?? \"/bin/zsh\")": 1,
	"OpenCodeInstall.swift OpenCodeInstall.major Disclaimed.process(program":                                                                                       1,
}

// swiftClientCalls is every TmuxClient call, by "file Type.function server: <what it tells>".
var swiftClientCalls = map[string]int{
	"TerminalDrivers.swift Tmux.run server: TmuxClient.unchecked": 1,
	"TerminalDrivers.swift Tmux.run server: desk.server":          1,
	"HireTmux.swift HireTmux.init server: TmuxClient.unchecked":   1,
}

// swiftClientBody is what TmuxClient and Tool.run must say, word for word (spaces aside): the guard's name and
// values, refused unless one, and the whole environment set, with stdin from /dev/null.
var swiftClientBody = map[string][]string{
	"HireTmux.swift TmuxClient.": {
		`static let variable = "SMOLCORP_TMUX_SERVER"`, `static let unchecked = "unchecked"`,
		`static let environment = ["PATH": "/usr/bin:/bin:/usr/sbin:/sbin", "TERM": "xterm-256color", "LANG": "en_US.UTF-8"]`,
	},
	"HireTmux.swift TmuxClient.run": {
		`guard !program.isEmpty, isGuard(server) else { return nil }`,
		`return Tool.run(program, arguments, input: input, environment: environment.merging([variable: server]) { $1 })`,
	},
	"HireTmux.swift TmuxClient.isGuard": {`server == unchecked || Int32(server).map { $0 > 1 && String($0) == server } == true`},
	"TerminalDrivers.swift Tool.run": {
		`if let environment { process.environment = environment process.standardInput = FileHandle.nullDevice }`,
	},
}

// swiftTmuxProblems is what's wrong with the app's tmux, one line each.
func swiftTmuxProblems(sources []swiftSource) []string {
	var problems []string
	// Found in the code with string contents blanked (a string naming a call isn't one), but a program's path, which is a
	// string: in the code as it is.
	count := func(pattern *regexp.Regexp, inStrings bool, key func(s swiftSource, m []int) string) map[string]int {
		found := map[string]int{}
		for _, s := range sources {
			text := s.bare
			if inStrings {
				text = s.code
			}
			for _, m := range pattern.FindAllStringIndex(text, -1) {
				found[key(s, m)]++
			}
		}
		return found
	}
	compare := func(what string, found, listed map[string]int) {
		for _, key := range slices.Sorted(func(yield func(string) bool) {
			for k := range found {
				if !yield(k) {
					return
				}
			}
		}) {
			if listed[key] != found[key] {
				problems = append(problems, fmt.Sprintf("%s: %s, %d times (listed %d): run tmux only through TmuxClient, told a server, "+
					"or list it in tmuxgate_swift_test.go with why", what, key, found[key], listed[key]))
			}
		}
		for key, n := range listed {
			if found[key] == 0 {
				problems = append(problems, fmt.Sprintf("%s: %s is listed (%d) but not in the app any more: take it off", what, key, n))
			}
		}
	}
	site := func(s swiftSource, m []int) string { return s.name + " " + s.where(m[0]) }
	name := func(s swiftSource, m []int) string {
		return site(s, m) + " " + strings.Join(strings.Fields(s.code[m[0]:m[1]]), "")
	}
	named := count(swiftTmuxNamed, false, name)
	for key, n := range count(swiftTmuxPath, true, name) {
		named[key] += n
	}
	compare("a tmux named or picked", named, swiftTmuxSites)
	compare("a program started", count(swiftStartCall, false, func(s swiftSource, m []int) string {
		args := s.callArguments(m[1] - 1)
		first := ""
		if len(args) > 0 {
			first = args[0]
		}
		return site(s, m) + " " + strings.Join(strings.Fields(s.code[m[0]:m[1]]), "") + first
	}), swiftStartSites)
	compare("a TmuxClient call", count(swiftClientCall, false, func(s swiftSource, m []int) string {
		server := "server: (none)"
		for _, arg := range s.callArguments(m[1] - 1) {
			if strings.HasPrefix(arg, "server:") {
				server = arg
			}
		}
		return site(s, m) + " " + server
	}), swiftClientCalls)
	for _, s := range sources {
		for _, m := range swiftStartValue.FindAllStringIndex(s.bare, -1) {
			problems = append(problems, fmt.Sprintf("%s: %s takes %q as a value: call it where it's listed", s.name, s.where(m[0]), strings.TrimSpace(s.code[m[0]:m[1]])))
		}
	}
	bodies := map[string]string{}
	for _, s := range sources {
		for _, d := range s.decls {
			key := s.name + " " + s.where(d.from+1)
			if d.kind != "func" {
				key = s.name + " " + d.name + "."
			}
			if _, seen := bodies[key]; !seen {
				bodies[key] = strings.Join(strings.Fields(s.code[d.from:min(d.to+1, len(s.code))]), " ")
			}
		}
	}
	keys := slices.Collect(func(yield func(string) bool) {
		for k := range swiftClientBody {
			if !yield(k) {
				return
			}
		}
	})
	sort.Strings(keys)
	for _, key := range keys {
		for _, line := range swiftClientBody[key] {
			if !strings.Contains(bodies[key], strings.Join(strings.Fields(line), " ")) {
				problems = append(problems, fmt.Sprintf("%s no longer says %q: the guard and the clean environment are TmuxClient's to give", key, line))
			}
		}
	}
	return problems
}

func TestTheMacAppRunsTmuxOnlyThroughItsGuardedClient(t *testing.T) {
	for _, problem := range swiftTmuxProblems(readSwiftApp(t)) {
		t.Error(problem)
	}
}

// The row on stand-ins: each way around TmuxClient is caught, and the app as it is passes (above).
func TestTheSwiftTmuxRowCatchesEachWayAroundTheClient(t *testing.T) {
	app := readSwiftApp(t)
	swap := func(name, old, replacement string) []swiftSource {
		t.Helper()
		out := slices.Clone(app)
		for i, s := range out {
			if s.name == name {
				src := strings.Replace(s.code, old, replacement, 1)
				if src == s.code {
					t.Fatalf("%s has no %q", name, old)
				}
				out[i] = parseSwift(name, src)
				return out
			}
		}
		t.Fatalf("no %s", name)
		return nil
	}
	for name, sources := range map[string][]swiftSource{
		"a tmux run by Tool.run itself": swap("TerminalDrivers.swift", "return TmuxClient.run(path, server: TmuxClient.unchecked, target + arguments, input: input)",
			"return Tool.run(path, target + arguments, input: input)"),
		"Homebrew's tmux for a checked desk": swap("TerminalDrivers.swift", "desk.server == TmuxClient.unchecked ? Tmux.path : Tmux.bundled",
			`desk.server == TmuxClient.unchecked ? Tmux.path : "/opt/homebrew/bin/tmux"`),
		"a guard of off": swap("TerminalDrivers.swift", "TmuxClient.run(program, server: desk.server,", `TmuxClient.run(program, server: "off",`),
		"a new start of the bundled tmux": swap("HireTmux.swift", `    func kill() -> Bool { ran(["kill-server"]) }`,
			"    func again() { _ = Tool.run(Tmux.bundled ?? \"\", [\"kill-server\"]) }\n"+`    func kill() -> Bool { ran(["kill-server"]) }`),
		"a Process of its own for tmux": swap("HireTmux.swift", `    func kill() -> Bool { ran(["kill-server"]) }`,
			"    func again() { let p = try? Disclaimed.process(URL(fileURLWithPath: Tmux.path!), []); try? p?.run() }\n"+`    func kill() -> Bool { ran(["kill-server"]) }`),
		"the client taken as a value": swap("HireTmux.swift", "run = { TmuxClient.run(path, server: TmuxClient.unchecked, $0) }",
			"let client = TmuxClient.run; run = { client(path, TmuxClient.unchecked, $0, nil) }"),
		"the guard not set":               swap("HireTmux.swift", "environment: environment.merging([variable: server]) { $1 })", "environment: environment)"),
		"the app's environment passed on": swap("TerminalDrivers.swift", "process.environment = environment", "process.environment = ProcessInfo.processInfo.environment"),
		"any guard let through":           swap("HireTmux.swift", "guard !program.isEmpty, isGuard(server) else { return nil }", "guard !program.isEmpty else { return nil }"),
	} {
		if problems := swiftTmuxProblems(sources); len(problems) == 0 {
			t.Errorf("%s: not caught", name)
		}
	}
	// A comment or a string naming a start isn't one.
	quiet := swap("HireTmux.swift", `    func kill() -> Bool { ran(["kill-server"]) }`,
		"    // Tool.run(Tmux.path!, [])\n    static let note = \"Tool.run(Tmux.bundled, []) \\(Openers.quoted(\"x\"))\"\n"+`    func kill() -> Bool { ran(["kill-server"]) }`)
	if problems := swiftTmuxProblems(quiet); len(problems) != 0 {
		t.Errorf("a comment and a string were taken for starts: %q", problems)
	}
}
