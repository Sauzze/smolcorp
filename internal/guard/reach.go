package guard

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// WIN125k (RWIN9 B1, B6): ways to reach Claude Code's settings, or the app's support folder, that name neither and use
// no shell pattern. A command that moves into a folder before it acts (cd, pushd, git -C, make -C, env -C,
// --work-tree, GIT_DIR=…) then reaches the settings by a name of one part (cd .claude/agents && rm ../settings.json):
// such a folder is refused when it is or could be in a .claude folder, or the support folder (dirRefusal). And find
// and git match names themselves, with patterns the shell never sees (find . -name 'settings.loc*' -delete,
// git rm '*settings.json'): their patterns are read the same skeleton way (keptByMatch).

// keptDirRule names a refusal of a move into a kept folder.
const (
	keptDirRule   = "a move into .claude or the app's own folder"
	keptDirWhy    = "From there a short name reaches Claude Code's settings or the app's own files. Work from the project's folder and name the file outright."
	twoArgCd      = "zsh's cd old new"
	twoArgCdWhy   = "zsh's cd with two words puts the second in place of the first in the current folder's path, which the guard can't follow. cd to the folder by its path."
	builtDirWhy   = "A folder built at run time ($x, $( ), a backtick) could be .claude or the app's own folder, out of the guard's sight. cd to the folder by its path, written out."
	dirsRule      = "zsh's dirs with folders"
	dirsWhy       = "zsh's dirs with words loads them as the folder stack, where popd and cd +1 then lead, out of the guard's sight. cd to the folder by its path."
	keptMatchRule = "a pattern that could match Claude Code's settings or a file the guard keeps"
	keptMatchWhy  = "find and git match names themselves: this pattern (or a find with no name test, or one negated) could reach Claude Code's settings or a file the guard keeps. Name the files outright, or narrow the pattern."
)

// dirOptions are the options, of any command, whose value is a folder it moves into before it acts: git's -C,
// --git-dir and --work-tree, make's and tar's -C and --directory, env's -C and --chdir, and the package tools' --cwd,
// --dir, --cd, --wd and --workdir. A value that isn't a folder (grep -C 3) holds no .claude and passes, and a command
// that only reads (guardReaders, git log) may look from anywhere.
var dirOptions = map[string]bool{
	"-C": true, "--chdir": true, "--directory": true, "--dir": true, "--cwd": true, "--cd": true, "--work-tree": true,
	"--git-dir": true, "--wd": true, "--workdir": true,
}

// cdFamily are the commands that move the shell itself into a folder: cd, pushd and zsh's chdir, and PowerShell's
// Set-Location (sl) and Push-Location (RWIN10 B2), whose cd, chdir and pushd are the same.
var cdFamily = map[string]bool{"cd": true, "pushd": true, "chdir": true, "set-location": true, "sl": true, "push-location": true}

// dirTargets are the folders a command moves into: the cd family's words (PowerShell's -Path:x and -LiteralPath:x
// too: every word that isn't an option is taken, in PowerShell's any-prefix reading of its options), each dirOptions
// value (attached, -Cdir and --dir=x, or the next word), sudo's -D and --chdir, and the value of a GIT_DIR= or
// GIT_WORK_TREE= among all the command's words (words; args are those after its name). two says it's a cd or pushd
// with two words, which zsh reads as old and new.
func dirTargets(name string, words simple, args []string) (targets []string, two bool) {
	for _, w := range unmark(words) {
		for _, v := range []string{"GIT_DIR=", "GIT_WORK_TREE="} {
			if strings.HasPrefix(w, v) {
				targets = append(targets, w[len(v):])
			}
		}
	}
	args = unmark(args)
	if cdFamily[name] {
		var pos []string
		for i, a := range args {
			if a == "--" {
				pos = append(pos, args[i+1:]...)
				break
			}
			if _, value, ok := strings.Cut(a, ":"); ok && strings.HasPrefix(a, "-") {
				targets = append(targets, value) // PowerShell's -Path:x
			}
			if strings.HasPrefix(a, "-") && a != "-" || strings.HasPrefix(a, "+") {
				continue // -L, -P, -q, PowerShell's -Path, and zsh's -2 and +2 (the folder stack, where only checked folders go)
			}
			pos = append(pos, a)
		}
		two = len(pos) >= 2 && (name == "cd" || name == "pushd" || name == "chdir")
		for _, p := range pos {
			if p != "-" {
				targets = append(targets, p)
			}
		}
	}
	for i, a := range args {
		opt, value, glued := a, "", false
		if eq := strings.IndexByte(a, '='); strings.HasPrefix(a, "--") && eq > 0 {
			opt, value, glued = a[:eq], a[eq+1:], true
		} else if strings.HasPrefix(a, "-C") && len(a) > 2 {
			opt, value, glued = "-C", a[2:], true
		}
		if !dirOptions[opt] && !(name == "sudo" && (opt == "-D" || opt == "--chdir")) || readsGuardFile(name, args) {
			continue // a command that only reads may look from there (ls -C .claude is ls in columns, git -C x log)
		}
		if glued {
			targets = append(targets, value)
		} else if i+1 < len(args) {
			targets = append(targets, args[i+1])
		}
	}
	return targets, two
}

// dirTargetsFolded are the skeleton targets of a folder that is, or is below, a .claude folder, or could be the app's
// support folder: three of the letters of application support or sauzzedeck written out (cd ~/Lib*/App*/A*).
var dirTargetsFolded = func() []skTarget {
	return []skTarget{
		target(0, piece{any: true}, piece{text: "/"}, piece{start: true}, piece{text: ".claude"}),
		target(0, piece{any: true}, piece{text: "/"}, piece{start: true}, piece{text: ".claude/"}, piece{any: true}),
		target(3, piece{any: true}, piece{text: "application support", counted: true}, piece{any: true}),
		target(3, piece{any: true}, piece{text: "sauzzedeck", counted: true}, piece{any: true}),
	}
}()

// keptDir says whether a folder a command moves into is, or could be, in a .claude folder or the app's support
// folder: after cleaning it (cleanWord: .claude/., x/..), one of its parts is .claude (as Windows reads it too:
// .claude. is .claude), Application Support, or sauzzedeck-pro, SauzzeDeck Pro or ~/.config (Linux's folder); or,
// when it holds a pattern or a $ (markDollars), some reading of it could fill in one (dirTargetsFolded). A .. after
// a part may be anywhere: it is a run that may start a hidden name, so x/.. is refused.
func keptDir(dir string) bool {
	return keptDirMarked(markDollars(strings.ReplaceAll(dir, `\`, "/")), nil)
}

// keptDirMarked is keptDir for a folder whose runs are marked already, unq saying which bytes the shell may read as a
// pattern (nil for all of them): a variable's value (lexer.value).
func keptDirMarked(marked string, unq []bool) bool {
	for _, alt := range runAlternatives(marked) {
		if keptDir(alt) {
			return true // cd ${x:-.claude}
		}
	}
	word, unq := cleanWord(marked, unq)
	if plain, _ := stripRuns(word, nil); keptFolderName(withoutSubs(plain)) {
		return true
	}
	if !isWild(word, unq) {
		return false
	}
	sk := skeletonOf(word, unq)
	for i := range dirTargetsFolded {
		if couldFill(&sk, &dirTargetsFolded[i]) {
			return true
		}
	}
	return false
}

// keptFolderName says whether a folder's path, as written (/ between its parts), has a part that is .claude (as
// Windows reads it too: .claude. is .claude), Application Support, or sauzzedeck-pro or SauzzeDeck Pro (Linux's and
// Windows' support folders), or ends in .config (Linux's folder above it), case aside.
func keptFolderName(dir string) bool {
	parts := strings.Split(Fold(strings.ReplaceAll(dir, `\`, "/")), "/")
	for i, part := range parts {
		switch strings.TrimRight(part, ". ") {
		case ".claude", "application support", "sauzzedeck-pro", "sauzzedeck pro":
			return true
		case ".config":
			if i == len(parts)-1 {
				return true
			}
		}
	}
	return false
}

// dirRefusal is a refusal for a command that moves into a kept folder (keptDir), or a two-word cd (in bash's
// reading: PowerShell's reads cd Société\ Générale as two words); nil for none.
//
// Fail closed (RWIN10 B2): a cd family's folder that holds a $ or a backtick, or a $( ) (subMark), is refused, as it's
// built at run time (x=.claude/agents; cd $x); and zsh's dirs with a word that isn't an option (+n and -n are), which
// loads the folder stack popd and cd +1 lead to.
func dirRefusal(name string, words simple, args []string, ps bool) *Refusal {
	command := strings.ReplaceAll(strings.Join(unmark(words), " "), string(subMark), "$(…)")
	targets, two := dirTargets(name, words, args)
	for _, t := range targets {
		if cdFamily[name] && strings.ContainsAny(t, "$`"+string(subMark)) {
			return &Refusal{Group: "guard", Rule: keptDirRule, Command: command, Why: builtDirWhy}
		}
		if keptDir(t) {
			return &Refusal{Group: "guard", Rule: keptDirRule, Command: command, Why: keptDirWhy}
		}
	}
	if name == "dirs" {
		for i, a := range unmark(args) {
			if a == "--" && i+1 < len(args) || !strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "+") {
				return &Refusal{Group: "guard", Rule: dirsRule, Command: command, Why: dirsWhy}
			}
		}
	}
	if two && !ps {
		return &Refusal{Group: "guard", Rule: twoArgCd, Command: command, Why: twoArgCdWhy}
	}
	return nil
}

// setsVarByName is why a command that names HOME, PWD, OLDPWD, CDPATH or DIRSTACK (dirVars), a variable a shell
// reads as it starts (startVars: read BASH_ENV, for ENV in …), or a prompt variable (promptVar), by itself as a word
// is refused: read, for, select, getopts, printf -v and the rest set it. A command
// that only reads or prints its words may (grep -rn PROMPT src/, echo PWD, printenv HOME: RWIN9 N1). "" for none.
func setsVarByName(name string, args []string) string {
	if name == "printenv" || readsGuardFile(name, args) && name != "printf" {
		return ""
	}
	for _, a := range unmark(args) {
		if dirVars[a] {
			return plainDirVar
		}
		if startVars[a] {
			return plainStart
		}
		if promptVar(a) {
			return plainCode
		}
	}
	return ""
}

// matchSkeleton is a pattern as find's -name and -path and git's pathspecs read it (fnmatch, with neither
// FNM_PATHNAME nor FNM_PERIOD): * and ? may be anything, a / and a leading dot included, a bracket may be any letter,
// a dot one too, and a \ makes the letter after it plain. A run the lexer marked (a $x, a $( ): RWIN10 B4) may be
// anything too, a pattern itself, so it reads as a *. Case aside (-iname's), as the skeleton always reads.
//
// With regex (find's -regex and -iregex, in any of GNU find's syntaxes: RWIN10 B6), a letter is written only when
// nothing gives it a meaning: a \ and the character after it are one run (\w, \S, \. and \( alike), a bracket is
// one run up to its close, every other character but a letter, a digit, a / and a - is a run, and a quantifier (* ?
// + {n} and their \ forms) makes the letter before it a run too. A regex with a | anywhere (an alternation, whose
// branches may leave any letter out) is one run: it may match anything.
func matchSkeleton(pattern string, regex bool) skeleton {
	sk := skeleton{dot: true}
	lit := func(r rune) {
		if r < utf8.RuneSelf {
			if 'A' <= r && r <= 'Z' {
				r += 'a' - 'A'
			}
			sk.letters[r]++
			sk.atoms = append(sk.atoms, atom{kind: aLit, r: r, start: true})
			return
		}
		for _, f := range Fold(string(r)) {
			sk.atoms = append(sk.atoms, atom{kind: aLit, r: f, start: true})
		}
	}
	run := func() { sk.atoms = append(sk.atoms, atom{kind: aRun, dot: true}) }
	// optional makes the letter before a regex's quantifier a run: x* and x? may leave it out.
	optional := func() {
		if n := len(sk.atoms); n > 0 && sk.atoms[n-1].kind == aLit {
			if r := sk.atoms[n-1].r; r < utf8.RuneSelf {
				sk.letters[r]--
			}
			sk.atoms[n-1] = atom{kind: aRun, dot: true}
		}
		run()
	}
	if len(pattern) > maxSkeleton {
		sk.long = true
	}
	if regex && strings.Contains(pattern, "|") {
		sk.atoms = []atom{{kind: aRun, dot: true}}
		return sk
	}
	for i := 0; i < len(pattern); {
		r, size := utf8.DecodeRuneInString(pattern[i:])
		c := pattern[i]
		switch {
		case c == runOpen:
			_, end := runText(pattern, i)
			run()
			i = end + 1
			continue
		case r >= utf8.RuneSelf:
			lit(r)
		case c == '/':
			sk.atoms = append(sk.atoms, atom{kind: aSlash})
		case regex && c == '\\':
			// \w, \S, \. : one run. \? \+ \{ are quantifiers (an interval to its \} is read whole).
			if i+1 < len(pattern) {
				_, n := utf8.DecodeRuneInString(pattern[i+1:])
				next := pattern[i+1]
				i += 1 + n
				switch next {
				case '?', '+', '*':
					optional()
				case '{':
					optional()
					if end := strings.Index(pattern[i:], `\}`); end >= 0 {
						i += end + 2
					} else {
						i = len(pattern)
					}
				default:
					run()
				}
				continue
			}
			run()
		case regex && c == '[':
			// A bracket is one character, whatever it holds: [^…], []…], [[:alpha:]].
			j := i + 1
			if j < len(pattern) && pattern[j] == '^' {
				j++
			}
			if j < len(pattern) && pattern[j] == ']' {
				j++
			}
			for j < len(pattern) && pattern[j] != ']' {
				if pattern[j] == '[' && j+1 < len(pattern) && strings.IndexByte(":.=", pattern[j+1]) >= 0 {
					if end := strings.Index(pattern[j+2:], string(pattern[j+1])+"]"); end >= 0 {
						j += end + 4
						continue
					}
				}
				j++
			}
			run()
			i = j + 1
			continue
		case regex && (c == '*' || c == '?' || c == '+'):
			optional()
		case regex && c == '{':
			optional()
			if end := strings.IndexByte(pattern[i:], '}'); end >= 0 {
				i += end + 1
			} else {
				i = len(pattern)
			}
			continue
		case regex && !isAlnum(r) && c != '-':
			run()
		case regex:
			lit(r)
		case c == '\\' && i+1 < len(pattern):
			// The letter after it is plain to find and git; read as it is, it may only be read wider.
		case c == '*' || c == '?':
			run()
		case c == '[':
			sk.atoms = append(sk.atoms, atom{kind: aOpen, dot: true})
		case c == ']':
			sk.atoms = append(sk.atoms, atom{kind: aClose, dot: true})
		default:
			lit(r)
		}
		i += size
	}
	return sk
}

// matchTargets are what find's and git's patterns are compared with: by a name alone (-name, -iname), Claude Code's
// folder and settings and each kept file (three of its letters written out); by a whole path (-path, -regex, a git
// pathspec), a .claude folder anywhere, the settings.json in one, a settings.local.json anywhere, and each kept file.
var matchTargets = func() (t struct{ name, path []skTarget }) {
	for _, n := range []string{".claude", "settings.json", "settings.local.json"} {
		t.name = append(t.name, target(0, piece{text: n}))
	}
	for _, name := range keptFiles() {
		t.name = append(t.name, target(3, piece{text: name, counted: true}))
	}
	t.path = []skTarget{
		target(0, piece{any: true}, piece{text: "/"}, piece{start: true}, piece{text: ".claude"}),
		target(0, piece{any: true}, piece{text: "/"}, piece{start: true}, piece{text: ".claude/settings.json"}),
	}
	for _, kept := range keptTargets.files[3] {
		t.path = append(t.path, kept.tg)
	}
	return t
}()

// matchCouldReach says whether a find or git pattern could match a kept name: by name alone, or as a whole path. Its
// $ runs are marked first (markDollars), and the words written in a ${x:-word} are read too; a pattern that is runs
// alone ("$f") is a word built at run time, the package comment's open class, as rm "$f" is.
func matchCouldReach(pattern string, whole, regex bool) bool {
	marked := markDollars(pattern)
	for _, alt := range runAlternatives(marked) {
		if matchCouldReach(alt, whole, regex) {
			return true
		}
	}
	if onlyRuns(marked) {
		return false
	}
	sk := matchSkeleton(marked, regex)
	list := matchTargets.name
	if whole {
		list = matchTargets.path
	}
	for i := range list {
		if couldFill(&sk, &list[i]) {
			return true
		}
	}
	return false
}

// keptByMatch is a refusal for find or git when their own patterns could reach a kept file (RWIN9 B6); nil for none.
// Its words keep their $ text and subMarks (RWIN10 B4: find . -name "sett${x}ings.json").
//
//   - find, with an action that changes or runs (-delete, -fprint, -fprint0, -fprintf, -fls, or -exec, -execdir, -ok
//     or -okdir with a command that doesn't only read): refused when a -name, -iname, -path, -ipath, -wholename,
//     -iwholename, -regex or -iregex that isn't negated could match a kept name (matchCouldReach), or when some
//     branch of its -o or , has no name test that isn't negated (or -empty: a fence is never an empty file) before
//     its action (find . -delete, find . -name x -o -delete, find . ( -type f -o -name x ) -delete, find . ! -name x
//     -delete reach every file). A negated test beside a name test that can't reach is fine (RWIN10 B7: find .
//     -name '*.js' -not -path './node_modules/*' -delete): the branch reaches only what that name test matches. A
//     test is negated after ! or -not, or inside a negated ( ), where -o joins no branch: !(a -o b) is !a and !b.
//     Reading through find (find ~ -name 'setup-k*' -exec cat {} +) is the open class of programs that match names.
//   - git rm, mv, restore, checkout, reset, clean and stash: refused when a path holds a pattern (* ? [) or a magic
//     prefix (:(glob), :/), or a $ run, read as a run, that could match a kept path; and for an exclusion (:!x, :^x,
//     :(exclude)x), which reaches everything else, and --pathspec-from-file, whose paths it can't see. git rm
//     --cached touches only the index (RWIN10 N2).
func keptByMatch(name string, args []string) string {
	args = unmark(args)
	switch name {
	case "find":
		return findReaches(args)
	case "git":
		sub, rest := gitSubcommand(args)
		switch sub {
		case "rm", "mv", "restore", "checkout", "reset", "clean", "stash":
			return gitPathspecReaches(sub, rest)
		}
	}
	return ""
}

// findTests are find's tests that match a name: by name alone, or a whole path (whole), or a regex.
var findTests = map[string]struct{ whole, regex bool }{
	"-name": {}, "-iname": {}, "-path": {whole: true}, "-ipath": {whole: true}, "-wholename": {whole: true},
	"-iwholename": {whole: true}, "-regex": {whole: true, regex: true}, "-iregex": {whole: true, regex: true},
}

func findReaches(args []string) string {
	changing := false
	type branch struct{ named, actFirst bool }
	branches := []branch{{}}
	var reach bool
	var groups []bool // for each ( open: whether it's negated
	not := false      // a ! or -not before the next test
	negated := func() bool { return not || len(groups) > 0 && groups[len(groups)-1] }
	for i := 0; i < len(args); i++ {
		a := args[i]
		b := &branches[len(branches)-1]
		act := func() {
			changing = true
			if !b.named {
				b.actFirst = true
			}
		}
		switch {
		case a == "-o" || a == "-or" || a == ",":
			if !negated() {
				branches = append(branches, branch{})
			}
		case a == "!" || a == "-not":
			not = true
			continue
		case a == "(":
			groups = append(groups, negated())
		case a == ")":
			if len(groups) > 0 {
				groups = groups[:len(groups)-1]
			}
		case a == "-empty":
			b.named = b.named || !negated()
		case a == "-delete" || a == "-fls" || strings.HasPrefix(a, "-fprint"):
			act()
			if a != "-delete" {
				i++ // its file (and -fprintf's format after it: read as a plain word)
			}
		case a == "-exec" || a == "-execdir" || a == "-ok" || a == "-okdir":
			end := i + 1
			for end < len(args) && args[end] != ";" && args[end] != "+" {
				end++
			}
			if cmd := args[i+1 : end]; len(cmd) == 0 || !readsGuardFile(commandName(withoutSubs(cmd[0])), stripSubs(cmd[1:])) {
				act()
			}
			i = end
		default:
			if t, ok := findTests[a]; ok && i+1 < len(args) {
				if !negated() {
					b.named = true
					reach = reach || matchCouldReach(args[i+1], t.whole, t.regex)
				}
				i++
			}
		}
		not = false
	}
	if !changing {
		return ""
	}
	if reach {
		return "find"
	}
	for _, b := range branches {
		if !b.named || b.actFirst {
			return "find"
		}
	}
	return ""
}

func gitPathspecReaches(sub string, args []string) string {
	options := args
	if dash := slices.Index(args, "--"); dash >= 0 {
		options = args[:dash] // after --, a --cached is a path
	}
	if sub == "rm" && slices.Contains(options, "--cached") {
		return ""
	}
	paths := false
	for _, a := range args {
		if a == "--" {
			paths = true
			continue
		}
		if strings.HasPrefix(a, "--pathspec-from-file") {
			return "git " + sub + " --pathspec-from-file"
		}
		if !paths && strings.HasPrefix(a, "-") {
			continue
		}
		spec := a
		magic := false
		if strings.HasPrefix(spec, ":") {
			magic = true
			var words string
			if strings.HasPrefix(spec, ":(") {
				end := strings.IndexByte(spec, ')')
				if end < 0 {
					return "git " + sub + " " + a
				}
				words, spec = spec[2:end], spec[end+1:]
			} else {
				k := 1
				for k < len(spec) && strings.IndexByte("/!^", spec[k]) >= 0 {
					k++
				}
				words, spec = spec[1:k], strings.TrimPrefix(spec[k:], ":")
			}
			if strings.ContainsAny(words, "!^") || strings.Contains(Fold(words), "exclude") {
				return "git " + sub + " " + a
			}
		}
		if !magic && !strings.ContainsAny(spec, "*?[$"+string(subMark)) {
			continue
		}
		pattern := spec
		if magic {
			pattern = "*" + spec // the magic prefix, as a run: :(top) and :/ start from the top, anywhere below cwd
		}
		if matchCouldReach(pattern, true, false) {
			return "git " + sub + " " + a
		}
	}
	return ""
}
