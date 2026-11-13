package guard

import (
	"strings"
	"unicode/utf8"
)

// WIN125j (VICTOR): eight rounds each found a spelling of a pattern that the guard's matcher read narrower than the
// shell ([!x-[:x:]], zsh's [!], x^y, a pattern earlier in the path). The guard stops modelling the shell's patterns.
// A word that could name a path is read as its skeleton: its letters, folded (Fold: NFC, then full case folding),
// and runs that may be anything. Outside quotes, every character bash or zsh can give a pattern or expansion
// meaning (skeletonSpecial) is a run that may be anything of any length, a / included, so a pattern earlier in the
// path (.c?aude/settings.json) and one across parts (Lib*/…) are read too. A bracket is a run from its [ to a ]
// after it, any ] after it, or its [ is a letter: the skeleton is read every way at once, so where bash 3.2, bash 5 or zsh close a
// bracket ([!x-[:x:]], [!], [[:alpha:]x]) is one of its readings, and so is every other. A ( or { group is a run
// up to any ) or } after it, the same way. zsh's ^ (anything but) makes the rest of the path part a run, its ~
// (except) the rest of the word, and its # (any number of) makes the letter before it a run too. The word is
// refused where any reading could fill in a name the guard keeps (skeletonNames). This only widens what the shell
// fills in: the guard (protectedByPattern, gitRevisionWord) and the demo's fence (PatternCouldFill) read
// patterns this one way.
//
// Two things a run can't be, as bash and zsh read patterns by default: a dot that starts a name. * ? [!a]
// [[:punct:]] [.] and zsh's ^x never start a hidden name in bash 3.2, bash 5.3 or zsh 5.9 (WIN125j's report); a dot
// written out at the start of its part does (.c*, not *.c), and so may a group (zsh's (.|x)claude, bash's
// @(.claude)) and a dot a # makes optional (.#claude). A line that changes that (zsh's GLOB_DOTS, bash's dotglob and GLOBIGNORE) is refused (plainDots).
// And the setup key, the token and the guard file are taken as filled in by a run only when at least three of
// their letters are written out (s* in any folder is just a pattern), unless the path before could be the app's
// support folder (three letters of its name written out: ~/Lib*/App*/A*/s*).

// skeletonSpecial says whether word[i], outside quotes, is a character bash or zsh may give a pattern or
// expansion meaning: * ? [ ] ^ # ! ( ) { } | < > , = and a ~ that isn't home (home is a ~ that starts the word,
// or follows = or :), and the @ or + of ksh's @( ) and +( ). A \ outside quotes never reaches here (the lexer
// takes it as an escape); one inside quotes, and a / anywhere, divide the path.
func skeletonSpecial(word string, unq []bool, i int) bool {
	if unq != nil && (i >= len(unq) || !unq[i]) {
		return false
	}
	switch c := word[i]; c {
	case '*', '?', '[', ']', '^', '#', '!', '(', ')', '{', '}', '|', '<', '>', ',', '=':
		return true
	case '~':
		return i > 0 && word[i-1] != '=' && word[i-1] != ':'
	case '@', '+':
		return i+1 < len(word) && word[i+1] == '(' && (unq == nil || unq[i+1])
	}
	return false
}

// runOpen and runClose hold, in a word the lexer hands the skeleton, an expansion the shell fills in when it runs
// (WIN125k, RWIN9 B2): a $name, a ${…}, a $((…)), a $( ) or a backtick, outside single quotes, in double quotes too.
// Such a run may be anything, empty included, and a letter after it starts its part if the run does (an empty $x
// before .c?aude leaves .c?aude). It may start a hidden name only when its own text holds a dot (${x:-.claude}/s*)
// and the word holds something written besides runs: a word that is runs alone (${f%.txt}, $(ls *.go)) is a word
// built at run time, the package comment's open class, where ${x:-.claude}'s written word is handed on by itself
// (runAlternatives). Between the two marks is the expansion's text (nothing for a $( ) or a backtick, whose output
// is that open class). finishWord takes the marks out of the words it hands on: a line can't hold them (controlIn
// refuses control characters).
const (
	runOpen  = '\x02'
	runClose = '\x03'
)

// subMark is a $( ) or a backtick's place, as the run's text and in the word the lexer hands on (WIN125l, RWIN10 B2
// B4): checkCommand takes it out of the words it reads, but the folder moves and find's and git's patterns read it as
// a run, and a line a wrapper runs (bash -c "cd $(…)") keeps it.
const subMark = '\x04'

// withoutSubs is a word without its subMarks: as the shell's word is when the substitution prints nothing.
func withoutSubs(word string) string {
	if strings.IndexByte(word, subMark) < 0 {
		return word
	}
	return strings.ReplaceAll(word, string(subMark), "")
}

// runText is a run's text between its marks, and where its close mark is (len(word) when it never closes).
func runText(word string, i int) (string, int) {
	end := strings.IndexByte(word[i+1:], runClose)
	if end < 0 {
		return word[i+1:], len(word)
	}
	return word[i+1 : i+1+end], i + 1 + end
}

// onlyRuns says whether a word is runs the lexer marked, and nothing written besides (a subMark is a run too).
func onlyRuns(word string) bool {
	for i := 0; i < len(word); i++ {
		switch word[i] {
		case runOpen:
			_, i = runText(word, i)
		case subMark:
		default:
			return false
		}
	}
	return true
}

// stripRuns is a word without its run marks, as the shell's word is written, with unq to match (nil stays nil).
func stripRuns(word string, unq []bool) (string, []bool) {
	if strings.IndexByte(word, runOpen) < 0 && strings.IndexByte(word, runClose) < 0 {
		return word, unq
	}
	var b []byte
	var q []bool
	for i := 0; i < len(word); i++ {
		if word[i] == runOpen || word[i] == runClose {
			continue
		}
		b = append(b, word[i])
		if unq != nil {
			q = append(q, unq[i])
		}
	}
	return string(b), q
}

// markDollars puts run marks around the $name and ${…} of a word the lexer has already handed on (a command's
// argument, its marks gone): the checks that read a command's arguments as paths (dirRefusal, keptByMatch) read
// them as runs too. A $( ) or a backtick left no text in the word.
func markDollars(word string) string {
	if !strings.ContainsAny(word, "$"+string(subMark)) {
		return word
	}
	var b strings.Builder
	for i := 0; i < len(word); i++ {
		if word[i] == subMark {
			b.WriteByte(runOpen)
			b.WriteByte(runClose)
			continue
		}
		if word[i] != '$' {
			b.WriteByte(word[i])
			continue
		}
		n := 1 + dollarName(word, i+1, false)
		if i+1 < len(word) && word[i+1] == '{' {
			depth := 0
			for n = 1; i+n < len(word); n++ {
				if word[i+n] == '{' {
					depth++
				} else if word[i+n] == '}' {
					if depth--; depth == 0 {
						n++
						break
					}
				}
			}
		}
		b.WriteByte(runOpen)
		b.WriteString(word[i : i+n])
		b.WriteByte(runClose)
		i += n - 1
	}
	return b.String()
}

// dollarName is how long the name after a $ at word[i:] is: a variable's name, digits, or one of bash's and zsh's
// special parameters (@ * # ? $ ! -); 0 for none (a $ alone is a letter). PowerShell's $env:NAME takes its scope.
// zsh's $=name, $+name and $#name (split words, is it set, its length: RWIN10 B4) take the name after the flag, or the
// { } after it; its $~ and $^ are left to the lexer, which refuses them.
func dollarName(word string, i int, ps bool) int {
	if i >= len(word) {
		return 0
	}
	if c := word[i]; !ps && (c == '=' || c == '+' || c == '#') && i+1 < len(word) {
		switch n := word[i+1]; {
		case n == '_' || isAlnum(rune(n)) && !(n >= '0' && n <= '9'):
			return 1 + dollarName(word, i+1, ps)
		case n == '{':
			if end := strings.IndexByte(word[i+1:], '}'); end >= 0 {
				return 2 + end
			}
			return len(word) - i
		}
	}
	switch c := word[i]; {
	case c == '_' || isAlnum(rune(c)) && !(c >= '0' && c <= '9'):
		n := 1
		for i+n < len(word) && (word[i+n] == '_' || isAlnum(rune(word[i+n])) || ps && word[i+n] == ':') {
			n++
		}
		return n
	case c >= '0' && c <= '9':
		n := 1
		for i+n < len(word) && word[i+n] >= '0' && word[i+n] <= '9' {
			n++
		}
		return n
	case strings.IndexByte("@*#?$!-", c) >= 0 && !ps:
		return 1
	}
	return 0
}

// cleanWord is a path word cleaned lexically, as the disk would walk it (WIN125k, RWIN9 B1): a // or a /./ is a
// /, a ./ at the start and a /. at the end go, and a trailing / goes. A .. after a part that isn't one may be
// anywhere (x may be a link: x/.. isn't the folder that holds x), so the path up to it is a run (its text is ..)
// that may start a hidden name. A \ divides the path too (Windows). unq follows the word (nil stays nil); a run's text is kept whole.
func cleanWord(word string, unq []bool) (string, []bool) {
	if !strings.ContainsAny(word, `/\`) {
		return word, unq
	}
	type part struct {
		s string
		q []bool
	}
	var parts []part
	abs := word[0] == '/' || word[0] == '\\'
	start := 0
	for i := 0; i <= len(word); i++ {
		if i < len(word) && word[i] == runOpen {
			_, i = runText(word, i)
			continue
		}
		if i < len(word) && word[i] != '/' && word[i] != '\\' {
			continue
		}
		p := part{s: word[start:i]}
		if unq != nil {
			p.q = unq[start:i]
		}
		start = i + 1
		switch {
		case p.s == "" || p.s == ".":
			continue
		case p.s == ".." && len(parts) > 0 && parts[len(parts)-1].s != "..":
			abs = false
			parts = []part{{string(runOpen) + ".." + string(runClose), []bool{false, false, false, false}}}
			continue
		}
		parts = append(parts, p)
	}
	var b strings.Builder
	var q []bool
	if abs {
		b.WriteByte('/')
		q = append(q, true)
	}
	for k, p := range parts {
		if k > 0 {
			b.WriteByte('/')
			q = append(q, true)
		}
		b.WriteString(p.s)
		q = append(q, p.q...)
	}
	if b.Len() == 0 {
		b.WriteByte('.')
		q = append(q, true)
	}
	if unq == nil {
		q = nil
	}
	return b.String(), q
}

// isWild says whether a word has a character skeletonSpecial reads as a run, or a run the lexer marked.
func isWild(word string, unq []bool) bool {
	if strings.IndexByte(word, runOpen) >= 0 {
		return true
	}
	for i := 0; i < len(word); i++ {
		if skeletonSpecial(word, unq, i) {
			return true
		}
	}
	return false
}

// An atom is one step of a skeleton.
type atom struct {
	kind  atomKind
	r     rune // aLit's letter, folded
	dot   bool // aRun, aGroup: it may start a name with a dot
	start bool // aLit: it starts its path part (nothing before it there but runs a # made), so its dot may start a name
	hash  bool // aRun: a # made it
}

type atomKind uint8

const (
	aLit    atomKind = iota // a letter
	aSlash                  // a / (or a \ in quotes, or Windows')
	aRun                    // a run that may be anything (* ? and the other characters)
	aOpen                   // [: a run up to any ] after it, or a letter
	aClose                  // ]: closes an open bracket, or not; alone, a run
	aGroup                  // ( or {: a run up to any ) or } after it, or a letter
	aEnd                    // ) or }: closes a group, or not; alone, a run
	aNot                    // zsh's ^: a run to the end of the path part
	aExcept                 // zsh's ~: a run to the end of the word
)

// maxSkeleton is the longest word read atom by atom. A longer word with a pattern in it is judged by its letters
// alone (could): it fills in each name whose letters and dot it holds, which is never less than a reading.
const maxSkeleton = 4096

// skeleton is a word read as atoms (see the top of this file). letters counts its ASCII letters (folded), dot says
// some atom may start a name with a dot, and long that the word was too long to read atom by atom.
type skeleton struct {
	atoms   []atom
	letters [utf8.RuneSelf]int
	dot     bool
	long    bool
}

// skeletonOf is a word's skeleton. unq says, for each byte, whether it was outside quotes; nil for all of them.
func skeletonOf(word string, unq []bool) skeleton {
	var sk skeleton
	lower := func(c byte) byte {
		if 'A' <= c && c <= 'Z' {
			return c + 'a' - 'A'
		}
		return c
	}
	if len(word) > maxSkeleton {
		sk.long = true
		for i := 0; i < len(word); i++ {
			if word[i] == runOpen {
				_, i = runText(word, i)
				continue
			}
			if c := word[i]; c < utf8.RuneSelf && !skeletonSpecial(word, unq, i) {
				sk.letters[lower(c)]++
			}
		}
		sk.dot = strings.ContainsAny(word, ".(")
		return sk
	}
	// dots[i] is how many dots word[:i] holds, closes[i] where the first } at or after i is (a { group's dot).
	dots, closes := make([]int, len(word)+1), make([]int, len(word)+1)
	closes[len(word)] = len(word)
	for i := len(word) - 1; i >= 0; i-- {
		closes[i] = closes[i+1]
		if word[i] == '}' && skeletonSpecial(word, unq, i) {
			closes[i] = i
		}
	}
	for i := 0; i < len(word); i++ {
		dots[i+1] = dots[i]
		if word[i] == '.' {
			dots[i+1]++
		}
	}
	add := func(a atom) {
		sk.atoms = append(sk.atoms, a)
		sk.dot = sk.dot || a.dot || a.kind == aLit && a.r == '.' && a.start
	}
	fresh := true // nothing in this path part yet but runs a # made
	// written says the word holds something besides runs the lexer marked: only then may such a run start a hidden name.
	written := false
	for i := 0; i < len(word); i++ {
		if word[i] == runOpen {
			_, i = runText(word, i)
			continue
		}
		written = true
		break
	}
	for i := 0; i < len(word); {
		r, size := utf8.DecodeRuneInString(word[i:])
		switch {
		case word[i] == runOpen:
			// An expansion: a run, which keeps its part fresh (an empty $x before .c?aude leaves .c?aude).
			text, end := runText(word, i)
			add(atom{kind: aRun, dot: text == ".." || strings.Contains(text, ".") && written})
			i = end + 1
			continue
		case skeletonSpecial(word, unq, i):
			switch word[i] {
			case '[':
				add(atom{kind: aOpen})
			case ']':
				add(atom{kind: aClose})
			case '(':
				add(atom{kind: aGroup, dot: true})
			case '{':
				// A brace makes a dot only if it holds one ({.c,x}laude); {} and {Get-Date} are no hidden name.
				add(atom{kind: aGroup, dot: dots[closes[i]] > dots[i]})
			case ')', '}':
				add(atom{kind: aEnd})
			case '^':
				add(atom{kind: aNot})
			case '~':
				add(atom{kind: aExcept})
			case '#':
				// x# is any number of x, none too: the letter before is a run (a dot's may start a name).
				if n := len(sk.atoms); n > 0 && sk.atoms[n-1].kind == aLit {
					prev := sk.atoms[n-1]
					sk.atoms = sk.atoms[:n-1]
					if prev.r < utf8.RuneSelf {
						sk.letters[prev.r]--
					}
					fresh = prev.start
					add(atom{kind: aRun, dot: prev.r == '.', hash: true})
				}
				add(atom{kind: aRun, hash: true})
				i += size
				continue
			default:
				add(atom{kind: aRun})
			}
			fresh = false
		case word[i] == '/' || word[i] == '\\':
			add(atom{kind: aSlash})
			fresh = true
		case r < utf8.RuneSelf:
			c := lower(word[i])
			sk.letters[c]++
			add(atom{kind: aLit, r: rune(c), start: fresh})
			fresh = false
		default:
			for _, f := range Fold(word[i : i+size]) {
				add(atom{kind: aLit, r: f})
			}
			fresh = false
		}
		i += size
	}
	return sk
}

// anyRun stands for "anything" in a target.
const anyRun = -1

// skTarget is what a skeleton is compared with: letters (folded) and anyRun, where a reading may start (starts),
// and how many of the skeleton's own letters must land on the counted ones (count) for it to be taken as naming
// it (min). letters counts its counted letters, and hidden says it has a dot that starts a name.
type skTarget struct {
	t       []rune
	count   []bool
	starts  []int
	min     int
	letters [utf8.RuneSelf]int
	hidden  bool
}

// piece is a part of a target: letters (counted or not), anyRun, or a place a reading may start.
type piece struct {
	text    string
	any     bool
	counted bool
	start   bool
}

// target builds an skTarget from its pieces; a reading starts at its beginning, and at each start piece.
func target(min int, pieces ...piece) skTarget {
	t := skTarget{min: min, starts: []int{0}}
	for _, p := range pieces {
		switch {
		case p.start:
			t.starts = append(t.starts, len(t.t))
		case p.any:
			t.t, t.count = append(t.t, anyRun), append(t.count, false)
		default:
			for _, r := range Fold(p.text) {
				if r == '.' && (len(t.t) == 0 || t.t[len(t.t)-1] == '/' || len(t.starts) > 0 && t.starts[len(t.starts)-1] == len(t.t)) {
					t.hidden = true
				}
				if p.counted && r < utf8.RuneSelf {
					t.letters[r]++
				}
				t.t, t.count = append(t.t, r), append(t.count, p.counted)
			}
		}
	}
	return t
}

// could is whether a skeleton could fill in a target, as far as its letters tell: enough of them for the target's
// min, and a dot that may start a name for a hidden one. fills needs it; a word too long for fills goes by it alone.
func (sk *skeleton) could(tg *skTarget) bool {
	if tg.hidden && !sk.dot {
		return false
	}
	have := 0
	for r, n := range tg.letters {
		have += min(n, sk.letters[r])
	}
	return have >= tg.min
}

// The ways a skeleton is being read at a step: outside any bracket or group, in a bracket, in a group, past a ^
// (to the end of the part), past a ~ (to the end of the word).
const (
	mOut = iota
	mBracket
	mGroup
	mPart
	mWord
	modes
)

// fills says whether some reading of a skeleton could fill in something the target matches: both are read as
// patterns, and the question is whether one string matches both, with at least the target's min of the
// skeleton's letters on its counted letters. A run takes any of the target's letters but a dot that starts a name
// (unless the run may), and the target's anyRun takes any of the skeleton's letters. A bracket or a group that
// never closes is a letter.
func fills(sk *skeleton, tg *skTarget) bool {
	n, m := len(tg.t), tg.min
	size := (n + 1) * (m + 1)
	at := func(j, c int) int { return j*(m+1) + min(c, m) }
	var cur, next [modes][]bool
	var live, nextLive [modes]bool
	for k := range cur {
		cur[k], next[k] = make([]bool, size), make([]bool, size)
	}
	hiddenDot := func(j int) bool { return tg.t[j] == '.' && (j == 0 || tg.t[j-1] == '/') }
	// closure lets the target's anyRun take nothing.
	closure := func(s []bool) {
		for j := 0; j < n; j++ {
			if tg.t[j] == anyRun {
				for c := 0; c <= m; c++ {
					if s[at(j, c)] {
						s[at(j+1, c)] = true
					}
				}
			}
		}
	}
	// run takes, from each place in src, any number of the target's letters into dst.
	run := func(src, dst []bool, dot bool) {
		for c := 0; c <= m; c++ {
			reached := false
			for k := 0; k <= n; k++ {
				reached = reached || src[at(k, c)]
				if reached {
					dst[at(k, c)] = true
				}
				if k < n && !dot && hiddenDot(k) {
					reached = false
				}
			}
		}
	}
	// letter takes one of the skeleton's letters from each place in src into dst. A dot that starts a name in the
	// target is taken only by a dot that starts its part in the skeleton (*.c is no .c, to bash or zsh).
	letter := func(src, dst []bool, r rune, start bool) {
		for j := 0; j < n; j++ {
			for c := 0; c <= m; c++ {
				if !src[at(j, c)] {
					continue
				}
				if tg.t[j] == anyRun {
					dst[at(j, c)] = true
				} else if tg.t[j] == r && (start || !hiddenDot(j)) {
					add := 0
					if tg.count[j] {
						add = 1
					}
					dst[at(j+1, c+add)] = true
				}
			}
		}
	}
	or := func(src, dst []bool) {
		for i, on := range src {
			dst[i] = dst[i] || on
		}
	}
	for _, j := range tg.starts {
		cur[mOut][at(j, 0)] = true
	}
	closure(cur[mOut])
	live[mOut] = true
	for _, a := range sk.atoms {
		for k := range next {
			if nextLive[k] {
				clear(next[k])
			}
		}
		if live[mOut] {
			switch a.kind {
			case aLit:
				letter(cur[mOut], next[mOut], a.r, a.start)
			case aSlash:
				letter(cur[mOut], next[mOut], '/', true)
			case aRun, aClose, aEnd:
				run(cur[mOut], next[mOut], a.dot)
			case aOpen:
				run(cur[mOut], next[mBracket], a.dot)
				letter(cur[mOut], next[mOut], '[', false)
			case aGroup:
				run(cur[mOut], next[mGroup], a.dot)
				letter(cur[mOut], next[mOut], '(', false)
			case aNot:
				run(cur[mOut], next[mPart], false)
			case aExcept:
				run(cur[mOut], next[mWord], false)
			}
		}
		// In a bracket: any ] after its [ may close it. In a group: any ) or } may close it.
		if live[mBracket] {
			or(cur[mBracket], next[mBracket])
			if a.kind == aClose {
				or(cur[mBracket], next[mOut])
			}
		}
		if live[mGroup] {
			or(cur[mGroup], next[mGroup])
			if a.kind == aEnd {
				or(cur[mGroup], next[mOut])
			}
		}
		// Past a ^: to the end of the part. Past a ~: to the end of the word.
		if live[mPart] {
			if a.kind == aSlash {
				letter(cur[mPart], next[mOut], '/', true)
			} else {
				or(cur[mPart], next[mPart])
			}
		}
		if live[mWord] {
			or(cur[mWord], next[mWord])
		}
		alive := false
		for k := range next {
			closure(next[k])
			nextLive[k] = false
			for _, on := range next[k] {
				if on {
					nextLive[k] = true
					break
				}
			}
			alive = alive || nextLive[k]
		}
		if !alive {
			return false
		}
		cur, next = next, cur
		live, nextLive = nextLive, live
	}
	// A bracket or a group that never closed isn't one: its reading as a letter is the one that counts.
	return cur[mOut][at(n, m)] || cur[mPart][at(n, m)] || cur[mWord][at(n, m)]
}

// keptFiles are the files the guard keeps that a pattern may not fill in (protectedNames, the Windows office's
// window key, and Claude Code's settings.local.json anywhere): taken as filled in when three of their letters are
// written out (the .json at the end aside), or when the path before could be the support folder.
func keptFiles() []string {
	return append(append([]string{}, protectedNames...), windowKeyFile, "settings.local.json")
}

// keptFolder are Claude Code's settings and their folder: filled in by any run, as a dot that starts a name is
// never one.
var keptFolder = []string{".claude", ".claude/settings.json", ".claude/settings.local.json"}

// namedTarget is a name the guard keeps and a target that stands for it.
type namedTarget struct {
	name string
	tg   skTarget
}

// keptTargets are skeletonNames' targets: files[n] each kept file with n of its letters written out, at the start
// of the word or after a /; support, a path that could hold the support folder's name, three of its letters
// written out, with a part after it; folder, Claude Code's settings and their folder.
var keptTargets = func() (k struct {
	files   map[int][]namedTarget
	support []skTarget
	folder  []namedTarget
}) {
	k.files = map[int][]namedTarget{}
	for _, least := range []int{0, 1, 3} {
		for _, name := range keptFiles() {
			stem, ext := strings.CutSuffix(name, ".json")
			suffix := ""
			if ext {
				suffix = ".json"
			}
			k.files[least] = append(k.files[least], namedTarget{name, target(least, piece{any: true}, piece{text: "/"}, piece{start: true},
				piece{text: stem, counted: true}, piece{text: suffix})})
		}
	}
	for _, word := range supportWords {
		k.support = append(k.support, target(3, piece{any: true}, piece{text: word, counted: true}, piece{any: true}, piece{text: "/"}, piece{any: true}))
	}
	for _, name := range keptFolder {
		k.folder = append(k.folder, namedTarget{name, target(0, piece{any: true}, piece{text: "/"}, piece{start: true}, piece{text: name})})
	}
	return k
}()

// skeletonBudget is how much comparing one line's words may take, in steps of a reading: a line past it has each
// word after that taken as filling in every name the guard keeps, so no line makes the guard slow.
const skeletonBudget = 1 << 23

// skeletonNames is the names the guard keeps that some reading of a skeleton could fill in, as protectedByPattern
// hands them on: each file of keptFiles, .claude, .claude/settings.json and .claude/settings.local.json. least is
// how many of a file's letters must be written out for a run to be taken as filling it in, where the path before
// can't be the support folder (3 for a pattern; 1 for a git revision, whose x^y fills in names that start with
// x). budget is taken from (skeletonBudget), nil for none.
func skeletonNames(sk *skeleton, least int, budget *int) []string {
	// No dot that may start a name, and fewer letters than a kept file or the support folder needs: none.
	total := 0
	for _, n := range sk.letters {
		total += n
	}
	if !sk.dot && total < min(least, 3) {
		return nil
	}
	fill := func(tg *skTarget) bool {
		if !sk.could(tg) {
			return false
		}
		if sk.long {
			return true
		}
		if budget != nil {
			if *budget -= len(sk.atoms) * len(tg.t) * (tg.min + 1); *budget < 0 {
				return true
			}
		}
		return fills(sk, tg)
	}
	files := keptTargets.files[least]
	for i := range keptTargets.support {
		if fill(&keptTargets.support[i]) {
			files = keptTargets.files[0]
			break
		}
	}
	var out []string
	for _, list := range [][]namedTarget{files, keptTargets.folder} {
		for i := range list {
			if fill(&list[i].tg) {
				out = append(out, list[i].name)
			}
		}
	}
	return out
}

// PatternCouldFill says whether a path pattern, read as its skeleton with every character outside quotes, could
// fill in name (one path part or more, a / between), case aside; or, with below, a path below name. The demo's
// fence matches the parts of a pattern against the app's support folder with it, so the fence and the guard read
// a pattern the same way.
func PatternCouldFill(pattern, name string, below bool) bool {
	sk := skeletonOf(pattern, nil)
	tg := target(0, piece{text: name})
	if below {
		tg = target(0, piece{text: name + "/"}, piece{any: true})
	}
	return couldFill(&sk, &tg)
}

// couldFill says whether some reading of a skeleton could fill in a target (could, then fills; a word too long
// to read atom by atom goes by its letters alone).
func couldFill(sk *skeleton, tg *skTarget) bool {
	if !sk.could(tg) {
		return false
	}
	return sk.long || fills(sk, tg)
}

// oddBracket says whether a word has a bracket, outside quotes, that starts with ! or ^ or holds another [ (a
// class, [[:alpha:]], or a [ as a letter, [x-[]): the guard refuses such a word as not written plainly where it
// could fill in a name it keeps, read or not (plainBracket).
func oddBracket(word string, unq []bool) bool {
	open := -1
	for i := 0; i < len(word); i++ {
		if unq != nil && !unq[i] {
			continue
		}
		switch word[i] {
		case '[':
			if open >= 0 {
				return true
			}
			open = i
			if i+1 < len(word) && (word[i+1] == '!' || word[i+1] == '^') {
				return true
			}
		case ']':
			if open >= 0 && i > open+1 {
				open = -1
			}
		case '/':
			open = -1
		}
	}
	return false
}
