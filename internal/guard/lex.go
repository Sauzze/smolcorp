package guard

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Sauzze/smolcorp/internal/appdirs"
)

// unreadable starts a word the guard can't tell the meaning of: a command
// name with a pattern in a path (/usr/bin/g?t, /usr/bin/gi*) or with too many
// or too long braces to list. As the command word it is refused; as an argument it is just a word.
const unreadable = "\x01"

// A simple command: its words, as the shell would run them, quotes taken
// away and escapes undone. Words with no command word of their own (an
// operator, a redirection's target) are left out, except that the target of
// an output redirection (> >> &>) comes out as its own two-word command,
// redirMark and the file, so that the guard can see what is written, and the
// target of an input one (<) as redirInMark and the file, so that it can see
// what is read.
type simple []string

// redirMark starts the two-word command that stands for an output redirection.
const redirMark = "\x00redir"

// redirInMark starts the one that stands for an input redirection: only the setup key is looked for in it.
const redirInMark = "\x00redirin"

// commands splits a command line into the simple commands the shell could
// run, wherever they are: after ; && || | & or a newline, inside ( ) { },
// and inside every $( ), backtick and <( ) >( ), in quotes or not. A here
// document's body is read as commands too (with no quotes and no comments,
// as the shell reads it, so an apostrophe in it hides nothing), which may
// refuse a harmless one but never lets one through. depth stops a line
// nested beyond reason.
//
// ps reads the line as PowerShell (or cmd) does: a backslash is just a
// character (C:\x\git.exe), the backtick is the escape, and { } start and
// end commands even with no space around them (%{git push}).
func commands(line string, depth int, ps bool) []simple {
	return commandsAs(line, depth, ps, false)
}

// A lexer is one reading of a line. The usual reading knows bash's here
// documents and its arithmetic and expansion units ((( )), $(( )), $[ ],
// ${ }, name[ ]), where << is no here document. The pessimist reading knows
// neither: every << is a plain redirection and every line after it is read as
// commands, so that a unit the usual reading mistook cannot hide a command
// (see needsPessimist). budget is how many bytes the unit scans may still
// read; spent, the line is refused, so that no input makes the guard slow.
type lexer struct {
	pessimist bool
	budget    int
	over      bool
	// plain is why the line isn't written plainly, and the word (WIN125g, plain.go): "" when it is.
	plain, plainWord string
	// skBudget is what's left of the line's skeletonBudget (skeleton.go).
	skBudget int
	// funsubs counts the ${ …; } read so far: past maxFunsubs, the line is refused (over).
	funsubs int
}

// maxFunsubs is the most of bash 5.3's ${ …; } one line may hold (one: each has the rest of the line read as commands).
const maxFunsubs = 1

// commandsAs is commands, as the usual reading or as the pessimist.
func commandsAs(line string, depth int, ps, pessimist bool) []simple {
	l := &lexer{pessimist: pessimist, budget: 32*len(line) + 4096, skBudget: skeletonBudget}
	out := l.lexLine(line, depth, ps, false)
	if l.over {
		out = append(out, simple{"\x00long", "an arithmetic or expansion the guard can't find the end of"})
	}
	if l.plain != "" {
		out = append(out, simple{plainMark, l.plain, l.plainWord})
	}
	return out
}

// needsPessimist is whether line has a << after something that may open an
// arithmetic or expansion unit on the same line ((( $[ ${ [ $' $"), where a
// reading mistake could hide a command: the pessimist reading is run too.
func needsPessimist(line string) bool {
	marked := false
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case c == '\n':
			marked = false
		case c == '[' || c == '$' && i+1 < len(line) && strings.IndexByte("[{'\"", line[i+1]) >= 0:
			marked = true
		case c == '(' && i+1 < len(line) && line[i+1] == '(':
			marked = true
		case c == '<' && strings.HasPrefix(line[i:], "<<<"):
			i += 2 // a here-string
		case c == '<' && marked && strings.HasPrefix(line[i:], "<<"):
			return true
		}
	}
	return false
}

// lexLine is commands; body is set when line is the body of a here
// document, where ' and " and # mean nothing and $( ) and backticks still run.
func (l *lexer) lexLine(line string, depth int, ps, body bool) []simple {
	if depth > 8 {
		return []simple{{"\x00too-deep"}}
	}
	var (
		out      []simple
		cur      simple
		word     []byte // the word so far
		unq      []bool // for each of its bytes: was it outside any quote?
		inWord   bool
		skipOne  bool      // the next word is a redirection's target
		redirOut bool      // ... of an output redirection
		hereStr  bool      // ... of a here-string (<<<)
		array    bool      // the words are a NAME=( … ) array's (RWIN10 B3)
		pending  []heredoc // here documents whose bodies start at the next newline
		closed   bool      // the last word of the command was a ]], or it was a (( )): see plainShort
	)
	lit := func(c byte) { word = append(word, c); unq = append(unq, false) }
	raw := func(c byte) { word = append(word, c); unq = append(unq, true) }
	// mark puts an expansion's text in the word as a run (runOpen, skeleton.go): the skeleton reads it as anything.
	mark := func(text string) {
		lit(runOpen)
		for _, b := range []byte(text) {
			lit(b)
		}
		lit(runClose)
	}

	reset := func() { word, unq, inWord = word[:0], unq[:0], false }
	unitOK := !ps && !l.pessimist
	// unitHere is the text of a unit read whole: its bytes go into the word, and
	// the commands inside its $( ) and backticks are found.
	unitHere := func(end, from int) {
		if line[from] == '$' {
			mark(line[from:end])
		} else {
			for _, b := range []byte(line[from:end]) {
				lit(b)
			}
		}
		out = append(out, l.unitCommands(line[from:end], depth+1, ps)...)
	}
	endWord := func() {
		if !inWord {
			return
		}
		plainText, plainUnq := stripRuns(string(word), unq)
		l.judge([]byte(plainText), plainUnq, ps)
		if skipOne {
			if redirOut {
				out = append(out, append(simple{redirMark}, l.finishWord(word, unq, ps)...))
			} else {
				if hereStr && !ps && valueInput(cur) {
					// read x <<< …, mapfile -t a <<< …: the here-string is a variable's value (RWIN10 B3).
					l.value(string(word), unq)
				}
				out = append(out, append(simple{redirInMark}, l.finishWord(word, unq, ps)...))
			}
			skipOne, redirOut, hereStr = false, false, false
		} else {
			// zsh's short forms (RWIN7 B2): a word right after ]] or )) with no ; between runs as a command
			// (if [[ -n x ]] git push). A { or } there starts or ends a list, which is split off below.
			s := plainText
			if closed && s != "{" && s != "}" {
				l.flag(plainShort, s)
			}
			// Only a ]] that closes this command's [[ (RWIN8 N1: echo ]] done is no test).
			closed = s == "]]" && !slices.Contains(plainUnq, false) && slices.Contains(cur, "[[")
			// The words the shell stores in a variable as they're written are read as its value (RWIN10 B2 B3).
			if !ps {
				switch {
				case array:
					v, q := string(word), unq
					if k := strings.Index(v, "]="); strings.HasPrefix(v, "[") && k > 0 {
						v, q = v[k+2:], q[k+2:] // a=([0]=x)
					}
					l.value(v, q)
				case valueWord(cur):
					l.value(string(word), unq)
				case isAssignment(s) && assignPosition(cur):
					eq := strings.IndexByte(string(word), '=')
					l.value(string(word[eq+1:]), unq[eq+1:])
				}
			}
			cur = append(cur, l.finishWord(word, unq, ps)...)
		}
		reset()
	}
	endCommand := func() {
		endWord()
		if len(cur) > 0 {
			out = append(out, cur)
		}
		cur, closed = nil, false
	}
	// nested reads a $( ), <( ) or >( ) from i (just past its opening
	// parenthesis), parses what's inside, and says where it ends.
	nested := func(i int) int {
		inner, end := l.balanced(line, i, ps)
		out = append(out, l.lexLine(inner, depth+1, ps, false)...)
		mark(string(subMark)) // what it prints is a word built at run time (the package comment's known bypasses)
		return end
	}
	backtick := func(i int) int {
		end := strings.IndexByte(line[i:], '`')
		if end < 0 {
			end = len(line) - i
		}
		out = append(out, l.lexLine(line[i:i+end], depth+1, ps, false)...)
		mark(string(subMark))
		return i + end + 1
	}
	// dollar reads a $ at line[i] that no other case took: $name (and $1, $@, …) is a run, and a $ before a { or
	// [ that isn't read as a unit (the pessimist reading, PowerShell's) is one too; a $ alone is a letter.
	dollar := func(i int, put func(byte)) int {
		if n := dollarName(line, i+1, ps); n > 0 {
			mark(line[i : i+1+n])
			return i + 1 + n
		}
		if i+1 < len(line) && (line[i+1] == '{' || line[i+1] == '[') {
			mark("$")
			return i + 1
		}
		put('$')
		return i + 1
	}

	for i := 0; i < len(line); {
		c := line[i]
		if !ps && c == '$' && strings.HasPrefix(line[i:], "$'") {
			// $'…' outside quotes, or in a body a shell runs (plain.go).
			l.flag(plainANSI, "$'")
		}
		if !ps && strings.HasPrefix(line[i:], "${(") {
			// zsh's parameter flags, outside quotes or in a body a shell runs (plain.go).
			l.flag(plainParam, "${(")
		}
		if !ps && strings.HasPrefix(line[i:], "${~") {
			// zsh's ${~x} globs x's text (RWIN9 B3).
			l.flag(plainSubst, "${~")
		}
		if !ps && l.funsub(line, i) {
			// bash 5.3's ${ …; } and ${| …; } run commands, in quotes too (RWIN9 B5): the rest of the line is read
			// as commands, past the } that ends them, which may refuse a harmless line but lets none through.
			out = append(out, l.lexLine(line[i+2:], depth+1, ps, false)...)
		}
		if unitOK {
			// $(( )), $[ ], ${ }, a (( )) command and a name[ ] subscript are read whole: a << in them is no here document.
			end, cmdForm := -1, false
			switch {
			case c == '$' && i+1 < len(line) && strings.IndexByte("[{(", line[i+1]) >= 0:
				end = l.unitEnd(line, i)
			case c == '(' && !inWord && i+1 < len(line) && line[i+1] == '(':
				end, cmdForm = l.unitEnd(line, i), true
			case c == '[' && inWord && subscriptStart(line, i):
				end = l.unitEnd(line, i)
			}
			if end > i {
				if cmdForm {
					endCommand()
					out = append(out, l.unitCommands(line[i:end], depth+1, ps)...)
					closed = true
				} else {
					inWord = true
					unitHere(end, i)
				}
				i = end
				continue
			}
		}
		switch {
		case c == '\\' && !ps && !body:
			inWord = true
			if i+1 < len(line) {
				if line[i+1] != '\n' {
					lit(line[i+1])
				}
				i += 2
			} else {
				i++
			}
		case c == '`' && ps:
			// PowerShell's escape character.
			inWord = true
			if i+1 < len(line) {
				if line[i+1] != '\n' {
					lit(line[i+1])
				}
				i += 2
			} else {
				i++
			}
		case c == '\'' && !body:
			inWord = true
			end := strings.IndexByte(line[i+1:], '\'')
			if end < 0 {
				end = len(line) - i - 1
			}
			for _, b := range []byte(line[i+1 : i+1+end]) {
				lit(b)
			}
			i += end + 2
		case c == '$' && !ps && !body && i+1 < len(line) && line[i+1] == '\'':
			// $'…': C-style escapes (\x67, \151, \u0067) decoded into the word.
			inWord = true
			i = ansiC(line, i+2, lit)
		case c == '$' && !ps && !body && i+1 < len(line) && line[i+1] == '"':
			i++ // $"…" is "…"
		case c == '"' && !body:
			inWord = true
			i++
			for i < len(line) && line[i] != '"' {
				if !ps && strings.HasPrefix(line[i:], "${(") {
					l.flag(plainParam, "${(")
				}
				if !ps && strings.HasPrefix(line[i:], "${~") {
					l.flag(plainSubst, "${~")
				}
				if !ps && l.funsub(line, i) {
					out = append(out, l.lexLine(line[i+2:], depth+1, ps, false)...)
				}
				if unitOK && line[i] == '$' && i+1 < len(line) && strings.IndexByte("[{(", line[i+1]) >= 0 {
					if end := l.unitEnd(line, i); end > i {
						unitHere(end, i)
						i = end
						continue
					}
				}
				switch {
				case line[i] == '\\' && !ps && i+1 < len(line):
					// Inside double quotes only these are escaped; any other backslash stays.
					switch n := line[i+1]; {
					case n == '\n':
					case strings.IndexByte("$`\"\\", n) >= 0:
						lit(n)
					default:
						lit('\\')
						lit(n)
					}
					i += 2
				case line[i] == '`' && ps && i+1 < len(line):
					lit(line[i+1])
					i += 2
				case line[i] == '$' && i+1 < len(line) && line[i+1] == '(':
					i = nested(i + 2)
				case line[i] == '`' && !ps:
					i = backtick(i + 1)
				case line[i] == '$':
					i = dollar(i, lit)
				default:
					lit(line[i])
					i++
				}
			}
			i++
		case c == '$' && i+1 < len(line) && line[i+1] == '(':
			inWord = true
			i = nested(i + 2)
		case (c == '<' || c == '>') && i+1 < len(line) && line[i+1] == '(':
			// A word of its own (the file's name), which ends at the next blank as any word does.
			endWord()
			inWord = true
			i = nested(i + 2)
		case c == '`':
			inWord = true
			i = backtick(i + 1)
		case c == ' ' || c == '\t':
			endWord()
			i++
		case c == ';' || c == '&' || c == '|' || c == '\n' || c == '(' || c == ')' || ps && (c == '{' || c == '}'):
			startArray := false
			if c == '(' && !ps {
				plain, _ := stripRuns(string(word), nil)
				if !l.pessimist {
					l.paren([]byte(plain), inWord, cur)
				}
				startArray = inWord && strings.HasSuffix(plain, "=") && isAssignment(plain)
			}
			endCommand()
			array = startArray || array && c != ')'
			i++
			if c == '\n' {
				// The bodies of the here documents opened on the line just ended.
				for _, h := range pending {
					var text string
					text, i = heredocBody(line, i, h)
					if h.value {
						// read x <<EOF: each line, and each word of it, is a variable's value (RWIN10 B3).
						for _, text := range strings.Split(text, "\n") {
							for _, v := range append(strings.Fields(text), text) {
								v = markDollars(v)
								l.value(v, slices.Repeat([]bool{true}, len(v)))
							}
						}
					}
					switch {
					case !h.data:
						// A shell, or a command we don't know, may run the body.
						out = append(out, l.lexLine(text, depth+1, ps, true)...)
					case !h.quoted:
						// Text for a command that only reads it: but the shell expands $( ) and backticks in it.
						out = append(out, l.hereExpansions(text, depth+1)...)
					}
				}
				pending = nil
			}
		case c == '<' || c == '>':
			// A redirection: its operator, then the file it names, which is no command.
			if digits, _ := stripRuns(string(word), nil); inWord && isDigits(digits) {
				reset()
			}
			endWord()
			if h, next, ok := heredocAt(line, i); ok && !l.pessimist {
				// << or <<-: the delimiter is no command either; the body comes after the line.
				// PowerShell and cmd have no <<, so there it is only taken for a command that just reads
				// its text (a bash line is read as PowerShell too, and a commit message is no command).
				h.data = readsOnly(cur)
				h.value = !ps && valueInput(cur)
				if !ps || h.data {
					pending = append(pending, h)
					i = next
					break
				}
			}
			j := i
			for j < len(line) && (line[j] == '<' || line[j] == '>' || line[j] == '&') {
				j++
			}
			redirOut = strings.Contains(line[i:j], ">")
			hereStr = line[i:j] == "<<<"
			i = j
			skipOne = true
		case c == '#' && !inWord && !body:
			// A comment, to the end of the line.
			end := strings.IndexByte(line[i:], '\n')
			if end < 0 {
				end = len(line) - i
			}
			i += end
		case c == '$':
			inWord = true
			i = dollar(i, raw)
		default:
			inWord = true
			raw(c)
			i++
		}
	}
	endCommand()
	// { and } are words, but start and end commands.
	var split []simple
	for _, s := range out {
		var part simple
		for _, w := range s {
			// A ! starts a command only where a command may start (! git push, if ! x): find . ! -name x keeps it.
			if w == "{" || w == "}" || w == "!" && (len(part) == 0 || shellReserved[part[len(part)-1]]) {
				if len(part) > 0 {
					split = append(split, part)
				}
				part = nil
				continue
			}
			part = append(part, w)
		}
		if len(part) > 0 {
			split = append(split, part)
		}
	}
	return split
}

// balanced is what's inside the parentheses that open just before i, and
// where they close; unclosed, to the end. A here document inside is skipped
// over (bash, not PowerShell), so that an apostrophe or a parenthesis in
// its body is no quote and no close.
func (l *lexer) balanced(line string, i int, ps bool) (string, int) {
	depth := 1
	var quote byte
	var pending []heredoc
	for j := i; j < len(line); j++ {
		c := line[j]
		if quote == 0 && !ps && !l.pessimist {
			end := -1
			switch {
			case c == '$' && j+1 < len(line) && strings.IndexByte("[{(", line[j+1]) >= 0:
				end = l.unitEnd(line, j)
			case c == '(' && j > i && j+1 < len(line) && line[j+1] == '(' && strings.IndexByte(" \t\n;&|(", line[j-1]) >= 0:
				end = l.unitEnd(line, j)
			case c == '[' && subscriptStart(line, j):
				end = l.unitEnd(line, j)
			}
			if end > j {
				j = end - 1
				continue
			}
		}
		switch {
		case quote != 0:
			if c == '\\' && quote == '"' {
				j++
			} else if c == quote {
				quote = 0
			}
		case c == '\\':
			j++
		case c == '\'' || c == '"':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return line[i:j], j + 1
			}
		case c == '<' && !ps && !l.pessimist:
			if strings.HasPrefix(line[j:], "<<<") {
				j += 2
			} else if h, next, ok := heredocAt(line, j); ok {
				pending = append(pending, h)
				j = next - 1
			}
		case c == '\n' && len(pending) > 0:
			k := j + 1
			for _, h := range pending {
				_, k = heredocBody(line, k, h)
			}
			pending = nil
			j = k - 1
		}
	}
	return line[i:], len(line)
}

// A heredoc is a here document the shell is waiting to read: its body is
// the lines after the one with the << on it, up to a line that is only delim.
type heredoc struct {
	delim  string
	strip  bool // <<-: tabs may come before the closing line
	quoted bool // the delimiter has a quote or a backslash: the body is not expanded
	data   bool // the command it feeds only reads it (cat, git commit -F -): see readsOnly
	value  bool // the command it feeds stores it in a variable (read, mapfile): see valueInput
}

// heredocAt reads << or <<- at line[i:] and the delimiter word after it
// (quotes and backslashes taken away), and says where that word ends. It is
// not a here document if it is <<<, or has no delimiter.
func heredocAt(line string, i int) (h heredoc, next int, ok bool) {
	if !strings.HasPrefix(line[i:], "<<") || strings.HasPrefix(line[i:], "<<<") {
		return h, i, false
	}
	j := i + 2
	if j < len(line) && line[j] == '-' {
		h.strip = true
		j++
	}
	for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
		j++
	}
	var d []byte
	for j < len(line) && strings.IndexByte(" \t\n;&|()<>", line[j]) < 0 {
		switch c := line[j]; c {
		case '\'':
			h.quoted = true
			end := strings.IndexByte(line[j+1:], '\'')
			if end < 0 {
				end = len(line) - j - 1
			}
			d = append(d, line[j+1:j+1+end]...)
			j += end + 2
		case '"':
			h.quoted = true
			for j++; j < len(line) && line[j] != '"'; j++ {
				if line[j] == '\\' && j+1 < len(line) {
					j++
				}
				d = append(d, line[j])
			}
			j++
		case '\\':
			h.quoted = true
			if j+1 < len(line) {
				d = append(d, line[j+1])
			}
			j += 2
		case '$':
			// $'EOF' and $"EOF" are quoted words, the $ is no part of the delimiter.
			if j+1 < len(line) && (line[j+1] == '\'' || line[j+1] == '"') {
				j++
			} else {
				d = append(d, c)
				j++
			}
		default:
			d = append(d, c)
			j++
		}
	}
	if j > len(line) {
		j = len(line)
	}
	if len(d) == 0 {
		return h, i, false
	}
	h.delim = string(d)
	return h, j, true
}

// heredocBody is the text of h's body, which starts at i (the start of a
// line), and where the line after its closing line begins. A body that is
// never closed runs to the end.
func heredocBody(line string, i int, h heredoc) (body string, next int) {
	start := i
	for i < len(line) {
		text, after := line[i:], len(line)
		if end := strings.IndexByte(text, '\n'); end >= 0 {
			text, after = text[:end], i+end+1
		}
		if h.strip {
			text = strings.TrimLeft(text, "\t")
		}
		if text == h.delim {
			return line[start:i], after
		}
		i = after
	}
	return line[start:], len(line)
}

// readsOnly is whether a here document's command only reads its text and
// never runs it: cat, git (commit -F -), a database client. A shell, eval,
// ssh, a command we don't know, or none (<<EOF cat) may run the body, so
// that is read as commands.
func readsOnly(cur simple) bool {
	for _, w := range cur {
		if isAssignment(w) {
			continue
		}
		return textReaders[commandName(strings.TrimPrefix(w, unreadable))]
	}
	return false
}

var textReaders = map[string]bool{
	"cat": true, "tee": true, "git": true, "mysql": true, "psql": true, "sqlite3": true, "sort": true, "uniq": true, "wc": true,
	"grep": true, "egrep": true, "fgrep": true, "head": true, "tail": true, "tr": true, "cut": true, "diff": true, "patch": true,
	"jq": true, "base64": true, "pbcopy": true, "xclip": true, "less": true, "more": true, "column": true, "fold": true, "fmt": true,
	"nl": true, "paste": true, "comm": true, "join": true, "mail": true, "sendmail": true,
}

// hereExpansions is the commands in $( ) and backticks of a here document's
// body whose delimiter is not quoted: the shell expands those there, and
// nothing else in it runs.
func (l *lexer) hereExpansions(text string, depth int) []simple {
	var out []simple
	for i := 0; i < len(text); {
		switch c := text[i]; {
		case c == '\\':
			i += 2
		case c == '$' && i+1 < len(text) && text[i+1] == '{':
			if l.funsub(text, i) {
				out = append(out, l.lexLine(text[i+2:], depth+1, false, false)...)
			}
			// The shell expands ${ } there too: zsh's flags and bash's @P are refused (paramFlags).
			if end := l.unitEnd(text, i); end > i {
				l.paramFlags(text[i:end], false)
			} else {
				l.paramFlags(text[i:], false)
			}
			i++
		case c == '$' && i+1 < len(text) && text[i+1] == '(':
			inner, end := l.balanced(text, i+2, false)
			out = append(out, l.lexLine(inner, depth+1, false, false)...)
			i = end
		case c == '`':
			end := i + 1
			for end < len(text) && text[end] != '`' {
				if text[end] == '\\' {
					end++
				}
				end++
			}
			if end > len(text) {
				end = len(text)
			}
			out = append(out, l.lexLine(text[i+1:end], depth+1, false, false)...)
			i = end + 1
		default:
			i++
		}
	}
	return out
}

// subscriptStart is whether the [ at line[i] follows a variable's name that
// starts a word (a[1<<2]=3): a subscript, not a test or a pattern.
func subscriptStart(line string, i int) bool {
	k := i
	for k > 0 && (isAlnum(rune(line[k-1])) || line[k-1] == '_') {
		k--
	}
	return k < i && (k == 0 || strings.IndexByte(" \t\n;&|(", line[k-1]) >= 0)
}

// unitEnd is where the arithmetic or expansion unit that starts at line[i]
// ends (just past its closer): $(( )), (( )), $[ ], ${ } or a subscript [ ];
// -1 if none starts there or it never closes (bash reads (( with no )) as
// two subshells, and the here documents in them are real). Quotes and
// backslashes in it are skipped. Time spent is counted against the budget.
func (l *lexer) unitEnd(line string, i int) int {
	var open, shut byte
	j := i
	switch {
	case strings.HasPrefix(line[i:], "$(("):
		j, open, shut = i+3, '(', ')'
	case strings.HasPrefix(line[i:], "(("):
		j, open, shut = i+2, '(', ')'
	case strings.HasPrefix(line[i:], "$["):
		j, open, shut = i+2, '[', ']'
	case strings.HasPrefix(line[i:], "${"):
		j, open, shut = i+2, '{', '}'
	case line[i] == '[':
		j, open, shut = i+1, '[', ']'
	default:
		return -1
	}
	subscript := line[i] == '['
	depth := 0
	for ; j < len(line); j++ {
		if l.budget--; l.budget < 0 {
			l.over = true
			return -1
		}
		switch c := line[j]; {
		case c == '\\':
			j++
		case c == '\'':
			end := strings.IndexByte(line[j+1:], '\'')
			if end < 0 {
				return -1
			}
			l.budget -= end
			j += end + 1
		case c == '"':
			j++
			for j < len(line) && line[j] != '"' {
				if line[j] == '\\' {
					j++
				}
				j++
				l.budget--
			}
			if j >= len(line) {
				return -1
			}
		case subscript && (c == '\n' || c == ';' || c == '&' || c == '|'):
			return -1
		case c == open:
			depth++
		case c == shut:
			if depth > 0 {
				depth--
				continue
			}
			if open == '(' {
				// (( ends at )) and no other way.
				if j+1 < len(line) && line[j+1] == ')' {
					return j + 2
				}
				return -1
			}
			return j + 1
		}
	}
	return -1
}

// unitCommands is the commands inside a unit's $( ) and backticks: they run
// when the unit is expanded, and nothing else in it does.
func (l *lexer) unitCommands(text string, depth int, ps bool) []simple {
	if depth > 8 {
		return []simple{{"\x00too-deep"}}
	}
	l.paramFlags(text, ps)
	var out []simple
	for k := 1; k < len(text); {
		switch c := text[k]; {
		case c == '\\':
			k += 2
		case c == '\'':
			end := strings.IndexByte(text[k+1:], '\'')
			if end < 0 {
				return out
			}
			k += end + 2
		case c == '`':
			end := strings.IndexByte(text[k+1:], '`')
			if end < 0 {
				end = len(text) - k - 1
			}
			out = append(out, l.lexLine(text[k+1:k+1+end], depth+1, ps, false)...)
			k += end + 2
		case c == '$' && !ps && k+1 < len(text) && text[k+1] == '\'':
			l.flag(plainANSI, "$'")
			k++
		case c == '$' && k+1 < len(text) && strings.IndexByte("[{(", text[k+1]) >= 0:
			if !ps && l.funsub(text, k) {
				out = append(out, l.lexLine(text[k+2:], depth+1, ps, false)...)
			}
			if end := l.unitEnd(text, k); end > k {
				out = append(out, l.unitCommands(text[k:end], depth+1, ps)...)
				k = end
			} else if text[k+1] == '(' {
				inner, end := l.balanced(text, k+2, ps)
				out = append(out, l.lexLine(inner, depth+1, ps, false)...)
				k = end
			} else {
				k++
			}
		default:
			k++
		}
	}
	return out
}

// funsub says whether text[i:] starts bash 5.3's ${ …; } or ${| …; }: a ${ with a blank, a line end or a | after
// it, which runs the commands inside in the shell itself. Past maxFunsubs, or past the budget (what reading the
// rest of the text takes is counted against it), the line is refused (over).
func (l *lexer) funsub(text string, i int) bool {
	if !strings.HasPrefix(text[i:], "${") || i+2 >= len(text) || strings.IndexByte(" \t\n|", text[i+2]) < 0 {
		return false
	}
	if l.funsubs++; l.funsubs > maxFunsubs {
		l.over = true
		return false
	}
	if l.budget -= len(text) - i; l.budget < 0 {
		l.over = true
		return false
	}
	return true
}

// paramFlags flags a ${ } unit that runs the text it's given (RWIN7 B3): zsh's parameter flags (${(e)…},
// ${(P)…}) at its start, and bash 5's @P (${x@P}) anywhere in it.
func (l *lexer) paramFlags(text string, ps bool) {
	if ps || !strings.HasPrefix(text, "${") {
		return
	}
	if why := expansionAssigns(text); why != "" {
		l.flag(why, text) // ${CDPATH:=…}, ${BASH_ENV:=…} (RWIN10 B2, B5)
	}
	if strings.HasPrefix(text, "${(") {
		l.flag(plainParam, "${(")
	} else if strings.HasPrefix(text, "${~") {
		l.flag(plainSubst, "${~")
	} else if strings.Contains(text, "@P") {
		l.flag(plainParam, "@P")
	}
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// finishWord is the word(s) a lexed word stands for. Brace expansion (bash)
// turns {git,push} into the two words git push; a word with an unquoted
// pattern in a path, or too many braces to list, is marked unreadable. A
// pattern with a bracket Go's matcher reads otherwise ([!x], [[:alpha:]]) that
// fills in a protected name is flagged as not written plainly (RWIN7 B1).
func (l *lexer) finishWord(word []byte, unq []bool, ps bool) []string {
	s := string(word)
	mark := hasPattern(stripRuns(s, unq))
	alts := []alt{{s, unq}}
	if !ps && hasBrace(s, unq) {
		var ok bool
		if alts, ok = braceExpand(s, unq); !ok {
			alts, mark = []alt{{s, unq}}, true
		}
	}
	words := make([]string, 0, len(alts))
	for _, a := range alts {
		plain, _ := stripRuns(a.s, nil)
		if mark {
			words = append(words, unreadable+plain)
		} else {
			words = append(words, plain)
		}
		// A pattern that the GM's token file, the guard file or Claude Code's settings could fill in: the name follows it as a word.
		// A brace sequence too long to list ({1..100}) is read as a run, as in a value (sequenceRuns, WIN125m).
		seq, seqQ := sequenceRuns(a.s, a.q)
		names, odd := protectedByPattern(seq, seqQ, &l.skBudget)
		if odd && len(names) > 0 {
			l.flag(plainBracket, plain)
		}
		// An assignment's value is a path word of its own (RWIN9 B3): the shell globs it where $x is used unquoted
		// (bash always, zsh with ${~x} or GLOB_SUBST), so every character of it counts as outside quotes, and a
		// part starts after the =.
		if isAssignment(plain) {
			eq := strings.IndexByte(seq, '=')
			value := seq[eq+1:]
			more, _ := protectedByPattern(value, valuePattern(value, seqQ[eq+1:]), &l.skBudget)
			names = append(names, more...)
		} else if eq := writtenEquals(seq); strings.HasPrefix(plain, "-") && eq > 0 {
			// A --opt=value's value starts a path of its own, its runs read as in any word (RWIN10 B4):
			// --target-directory=.clau${x}de.
			more, _ := protectedByPattern(seq[eq+1:], seqQ[eq+1:], &l.skBudget)
			names = append(names, more...)
		}
		// A ${x:-word} (and :=, ::=, :+, -, =, +) may be its word, written out: handed on as a word of its own, and read
		// as a pattern (${x:-.claude}, ${x:-.c?aude}). One that assigns is a variable's value too (RWIN10 B2).
		for _, alt := range runAlternatives(a.s) {
			more, _ := protectedByPattern(alt, slices.Repeat([]bool{true}, len(alt)), &l.skBudget)
			names = append(append(names, alt), more...)
		}
		for _, alt := range runAlts(a.s, true) {
			if keptDir(alt) {
				l.flag(plainValue, alt)
			}
		}
		if l.skBudget < 0 {
			// Past the budget, the line is refused for its length, not for a name it may not hold (RWIN9 N4).
			l.flag(plainBudget, plain)
			continue
		}
		words = append(words, names...)
	}
	return words
}

// runAlternatives are the words the ${…} runs of a word may stand for, written in them: the word after :- := :+ - = or
// + (${x:-.claude} is .claude when x is unset), its quotes taken away.
func runAlternatives(word string) []string { return runAlts(word, false) }

// runAlts is runAlternatives, or with assigns only the words of the forms that assign them to the variable (= := and
// zsh's ::=).
func runAlts(word string, assigns bool) []string {
	var out []string
	for i := strings.IndexByte(word, runOpen); i >= 0 && i < len(word); {
		text, end := runText(word, i)
		if strings.HasPrefix(text, "${") && strings.HasSuffix(text, "}") {
			body := text[2 : len(text)-1]
			n := strings.IndexFunc(body, func(r rune) bool { return !(r == '_' || isAlnum(r)) })
			if n > 0 {
				rest := strings.TrimPrefix(body[n:], ":")
				if strings.HasPrefix(rest, ":=") {
					rest = rest[1:] // zsh's ${x::=word}
				}
				if rest != "" && strings.IndexByte("-=+", rest[0]) >= 0 && (!assigns || rest[0] == '=') {
					out = append(out, strings.NewReplacer(`'`, "", `"`, "").Replace(rest[1:]))
				}
			}
		}
		next := strings.IndexByte(word[min(end+1, len(word)):], runOpen)
		if next < 0 {
			break
		}
		i = end + 1 + next
	}
	return out
}

// writtenEquals is where the first = outside a run is in a lexed word, or -1.
func writtenEquals(word string) int {
	for i := 0; i < len(word); i++ {
		switch word[i] {
		case runOpen:
			_, i = runText(word, i)
		case '=':
			return i
		}
	}
	return -1
}

// value reads a word the shell stores in a variable as it's written (WIN125l, RWIN10 B2 B3): an assignment's value
// (NAME=, NAME+=, in a command's leading words or a declare's, typeset's, local's, export's, readonly's, env's), a
// NAME=( … ) or NAME+=( … ) array's words, the words after for's, select's and foreach's in, every word of set (set
// --, zsh's set -A), getopts, zstyle and zparseopts, of printf -v and zsh's print -v, and the here-string or each line
// and word of a here document given to read, mapfile or readarray (valueInput). bash fills the value in as a pattern
// where $x is used unquoted (valuePattern: its pattern characters count in any quotes), and cd $x, git -C $x and the
// rest move into it. The line is refused when the value could fill in a name the guard keeps, is or could be in a
// .claude folder or the app's support folder (keptDir), or names the fence, the guard file or the setup key. s holds
// the lexer's run marks, q says which bytes were outside quotes. A value read from a file or a pipe (read x < f,
// echo x | read y) is the package comment's open class.
//
// A brace sequence in it ({1..5}, {01..12}, {a..e}, {0..100..10}) is read as a run (sequenceRuns, WIN125m, RWIN11
// PART 1): for i in {1..5} and x=({1..3}) run, as echo x{1..3} does, and .{a..z}laude still reaches .claude.
func (l *lexer) value(s string, q []bool) {
	s, q = sequenceRuns(s, q)
	plain, _ := stripRuns(s, nil)
	pattern := valuePattern(s, q)
	names, _ := protectedByPattern(s, pattern, &l.skBudget)
	if l.skBudget < 0 {
		l.flag(plainBudget, plain)
		return
	}
	if len(names) > 0 || keptDirMarked(strings.ReplaceAll(s, `\`, "/"), pattern) ||
		namesFence(simple{plain}) || namesGuardFile(simple{plain}) || namesSetupKey(simple{plain}) {
		l.flag(plainValue, withoutSubs(plain))
	}
}

// sequenceRuns is a value with each brace sequence written outside quotes put as an empty run (runOpen, runClose: a
// run that may be anything but can't start a hidden name), and q to match. A sequence is {x..y} or {x..y..n} with
// x, y and n whole numbers of at most ten digits (a - before them allowed, n not 0), or {x..y} with x and y a single
// letter or digit each. Its words are numbers, or single characters from 0 (0x30) up, in bash 3.2, bash 5 and zsh
// 5.9: never a dot or a /. Any other brace stays as it was, a group that may hold a dot: {+..0} (zsh fills in + , -
// . / 0), {a..e..2} and {1..3..0} (zsh's BRACE_CCL reads them as the characters . 0 1 3), {.c,x}.
func sequenceRuns(s string, q []bool) (string, []bool) {
	if !strings.Contains(s, "..") {
		return s, q
	}
	var b []byte
	var nq []bool
	for i := 0; i < len(s); i++ {
		if s[i] == runOpen {
			_, end := runText(s, i)
			b, nq = append(b, s[i:min(end+1, len(s))]...), append(nq, q[i:min(end+1, len(s))]...)
			i = end
			continue
		}
		if s[i] == '{' && q[i] {
			if end := strings.IndexByte(s[i:], '}'); end > 0 && !slices.Contains(q[i:i+end+1], false) &&
				braceSequence(s[i+1:i+end]) {
				b, nq = append(b, runOpen, runClose), append(nq, false, false)
				i += end
				continue
			}
		}
		b, nq = append(b, s[i]), append(nq, q[i])
	}
	return string(b), nq
}

// braceSequence says whether the inside of a brace is a sequence sequenceRuns reads as a run.
func braceSequence(body string) bool {
	number := func(s string) bool {
		s = strings.TrimPrefix(s, "-")
		return len(s) <= 10 && isDigits(s)
	}
	parts := strings.Split(body, "..")
	switch {
	case len(parts) == 2 && len(parts[0]) == 1 && len(parts[1]) == 1:
		return isAlnum(rune(parts[0][0])) && isAlnum(rune(parts[1][0]))
	case len(parts) == 2:
		return number(parts[0]) && number(parts[1])
	case len(parts) == 3:
		return number(parts[0]) && number(parts[1]) && number(parts[2]) && strings.Trim(parts[2], "-0") != ""
	}
	return false
}

// precommand are the words before a command's name that are no name of their own: the shell's reserved words, and
// the precommand modifiers of bash and zsh.
var precommand = map[string]bool{
	"builtin": true, "command": true, "noglob": true, "nocorrect": true, "exec": true, "-": true, "time": true, "!": true,
	"if": true, "then": true, "else": true, "elif": true, "do": true, "while": true, "until": true, "{": true,
}

// commandWord is the name of the command the words so far start (assignments and precommands skipped), and the words
// after it; "" when there's none yet.
func commandWord(cur simple) (string, simple) {
	for i, w := range cur {
		if isAssignment(w) || precommand[w] {
			continue
		}
		return commandName(withoutSubs(strings.TrimPrefix(w, unreadable))), cur[i+1:]
	}
	return "", nil
}

// assignPosition says whether a NAME=value word after cur assigns: among a command's leading words, or an argument
// of a builtin that declares (declare, typeset, local, export, readonly, integer, float) or of env and sudo.
func assignPosition(cur simple) bool {
	name, _ := commandWord(cur)
	switch name {
	case "", "declare", "typeset", "local", "export", "readonly", "integer", "float", "env", "sudo", "doas":
		return true
	}
	return false
}

// valueWord says whether the word after cur is one the shell stores in a variable as it's written (see value).
func valueWord(cur simple) bool {
	name, rest := commandWord(cur)
	switch name {
	case "for", "select", "foreach":
		return slices.Contains(rest, "in")
	case "set", "getopts", "zstyle", "zparseopts":
		return true
	case "printf", "print":
		for _, w := range rest {
			if w == "--" {
				break
			}
			if strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "--") && strings.Contains(w, "v") {
				return true
			}
		}
	}
	return false
}

// valueInput says whether a here-string or a here document given to the command so far is stored in a variable:
// read, mapfile and readarray, and a loop or a group that may hold one (done, fi, esac, }), or no command yet.
func valueInput(cur simple) bool {
	if len(cur) > 0 {
		switch cur[len(cur)-1] {
		case "done", "fi", "esac", "}":
			return true
		}
	}
	name, _ := commandWord(cur)
	switch name {
	case "", "read", "mapfile", "readarray":
		return true
	}
	return false
}

// valuePattern says, for each byte of an assignment's value, whether bash may read it as a pattern where $x is
// expanded unquoted (RWIN9 B3): * ? [ ] and ! in any quotes, and an extglob group's ( | ) after ? * + @ or !; and what
// the shell would read outside quotes in the assignment itself. zsh globs a value only with ${~x} or GLOB_SUBST, which
// are refused (plain.go). A value's $ runs stay runs.
func valuePattern(value string, unq []bool) []bool {
	out := make([]bool, len(value))
	depth := 0
	for i := 0; i < len(value); i++ {
		switch c := value[i]; {
		case unq[i]:
			out[i] = true
		case strings.IndexByte("*?[]!", c) >= 0:
			out[i] = true
		case c == '(' && i > 0 && strings.IndexByte("?*+@!", value[i-1]) >= 0:
			out[i], depth = true, depth+1
		case depth > 0 && (c == '|' || c == ')'):
			out[i] = true
			if c == ')' {
				depth--
			}
		}
	}
	return out
}

// protectedNames are the files and variable a hire may not name by any
// spelling (the GM token, in its file or where a computer with no secret
// store keeps it, the guard file, the app's setup key, the owner's grant record, the paired phones' seal's
// key, which Claude Code the owner has, and the push key, where a computer with no secret store keeps it). protectedByPattern says which a pattern word (gm-t*, hire-g?ard.json) could fill in, read as its skeleton
// (skeleton.go).
var protectedNames = []string{"gm-token", "gm-token-kept", "smolcorp_gm_token", "hire-guard.json", "hire-guard", setupKeyFile,
	"grants.jsonl", "phones-seal", "claude-install", "push-key-kept"}

// supportWords are the words of the app's support folder on each system (appdirs: AgentDeck, sauzzedeck-pro,
// SauzzeDeck Pro), and Smolcorp's: a path before a pattern that could hold one may be that folder.
var supportWords = []string{appdirs.Mac, appdirs.Linux, appdirs.Windows, "smolcorp", "application support"}

// protectedByPattern is the names the guard keeps that a word with a pattern in it could fill in (skeletonNames:
// the setup key, the token, the guard file, Claude Code's settings and their folder), by a run in any of its path
// parts; none for a word with no pattern. The word is cleaned first (cleanWord: .claude/./s*, x/../s*). budget is the line's for comparing (skeletonBudget). odd says it has a bracket Go's matcher would once have read otherwise
// (oddBracket).
func protectedByPattern(s string, unq []bool, budget *int) (out []string, odd bool) {
	s, unq = cleanWord(s, unq)
	if !isWild(s, unq) {
		return nil, false
	}
	sk := skeletonOf(s, unq)
	return skeletonNames(&sk, 3, budget), oddBracket(s, unq)
}

// hasPattern is whether s, as a command name, could be a pattern the shell
// fills in from the disk: an unquoted * ? or [ in a word that is a path
// (has a / in it, or a \ for PowerShell). A command found on the PATH is
// never a pattern, so g?t is just a word nobody has; /usr/bin/g?t is the
// command. A bare * or **bold** at the start of a line is not one.
func hasPattern(s string, unq []bool) bool {
	glob, slash := false, false
	for i := 0; i < len(s); i++ {
		if unq[i] && strings.IndexByte("*?[", s[i]) >= 0 {
			glob = true
		} else if s[i] == '/' || s[i] == '\\' {
			slash = true
		}
	}
	return glob && slash
}

func hasBrace(s string, unq []bool) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '{' && unq[i] {
			return true
		}
	}
	return false
}

func isAlnum(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// Braces are listed only up to these sizes; a word beyond them is refused
// as unreadable, so that no input makes the guard slow.
const (
	maxBraceWords  = 64   // most words one braced word is listed as
	maxBraceLen    = 4096 // longest word with an unquoted { that is listed
	maxBraceOpens  = 16   // most unquoted { in one such word
	maxSequenceAbs = 1 << 40
)

type alt struct {
	s string
	q []bool // for each byte: outside any quote?
}

// A group is a matched {…}: where it closes, and the commas directly in it.
type group struct {
	end    int
	commas []int
}

// braceExpand lists what the unquoted braces in s stand for: {a,b} as a b,
// x{1..3}y as x1y x2y x3y. False when s is too long or has too many braces,
// or it would be more than maxBraceWords words. The braces are paired in one
// pass, and the words built from the pairs, so the time is in proportion
// to the length times the number of words.
func braceExpand(s string, unq []bool) ([]alt, bool) {
	if len(s) > maxBraceLen {
		return nil, false
	}
	type open struct {
		at     int
		commas []int
	}
	var stack []open
	groups := map[int]group{}
	opens := 0
	for i := 0; i < len(s); i++ {
		if !unq[i] {
			continue
		}
		switch s[i] {
		case '{':
			if opens++; opens > maxBraceOpens {
				return nil, false
			}
			stack = append(stack, open{at: i})
		case '}':
			if n := len(stack); n > 0 {
				groups[stack[n-1].at] = group{end: i, commas: stack[n-1].commas}
				stack = stack[:n-1]
			}
		case ',':
			if n := len(stack); n > 0 {
				stack[n-1].commas = append(stack[n-1].commas, i)
			}
		}
	}
	alts, ok := expandRange(s, unq, groups, 0, len(s))
	if !ok {
		return nil, false
	}
	return alts, true
}

// expandRange lists the words s[lo:hi] stands for. A { that is not a group,
// or is one with neither commas nor a sequence in it, is just a character.
func expandRange(s string, unq []bool, groups map[int]group, lo, hi int) ([]alt, bool) {
	res := []alt{{}}
	litFrom := lo
	flush := func(to int) {
		for k := range res {
			res[k].s += s[litFrom:to]
			res[k].q = append(res[k].q, unq[litFrom:to]...)
		}
	}
	for i := lo; i < hi; i++ {
		g, isGroup := groups[i]
		if s[i] != '{' || !unq[i] || !isGroup || g.end >= hi {
			continue
		}
		var opts []alt
		if len(g.commas) > 0 {
			start := i + 1
			for _, c := range append(append([]int{}, g.commas...), g.end) {
				sub, ok := expandRange(s, unq, groups, start, c)
				if !ok || len(opts)+len(sub) > maxBraceWords {
					return nil, false
				}
				opts = append(opts, sub...)
				start = c + 1
			}
		} else {
			items, valid, tooMany := sequence(s[i+1 : g.end])
			if tooMany {
				return nil, false
			}
			if !valid {
				continue
			}
			for _, it := range items {
				opts = append(opts, alt{it, make([]bool, len(it))})
			}
		}
		flush(i)
		if len(res)*len(opts) > maxBraceWords {
			return nil, false
		}
		next := make([]alt, 0, len(res)*len(opts))
		for _, r := range res {
			for _, o := range opts {
				next = append(next, alt{r.s + o.s, append(append([]bool{}, r.q...), o.q...)})
			}
		}
		res = next
		i = g.end
		litFrom = i + 1
	}
	flush(hi)
	return res, true
}

// sequence reads the inside of {1..5}, {a..e} or {1..9..2}.
func sequence(body string) (items []string, valid, tooMany bool) {
	parts := strings.Split(body, "..")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, false, false
	}
	step := 1
	if len(parts) == 3 {
		n, err := strconv.Atoi(parts[2])
		if err != nil || n == 0 {
			return nil, false, false
		}
		if n < 0 {
			n = -n
		}
		step = n
	}
	from, err1 := strconv.Atoi(parts[0])
	to, err2 := strconv.Atoi(parts[1])
	letters := len(parts[0]) == 1 && len(parts[1]) == 1 && isAlnum(rune(parts[0][0])) && isAlnum(rune(parts[1][0]))
	switch {
	case err1 == nil && err2 == nil:
	case letters:
		from, to = int(parts[0][0]), int(parts[1][0])
	default:
		return nil, false, false
	}
	if from > maxSequenceAbs || from < -maxSequenceAbs || to > maxSequenceAbs || to < -maxSequenceAbs {
		return nil, true, true
	}
	span := to - from
	if span < 0 {
		span = -span
	}
	if span/step+1 > maxBraceWords {
		return nil, true, true
	}
	dir := 1
	if to < from {
		dir = -1
	}
	for v := from; (v-to)*dir <= 0; v += dir * step {
		if err1 == nil && err2 == nil {
			items = append(items, strconv.Itoa(v))
		} else {
			items = append(items, string(rune(v)))
		}
	}
	return items, true, false
}

// ansiC reads the inside of $'…' from i, putting each byte it stands for,
// and says where the closing quote ends.
func ansiC(line string, i int, put func(byte)) int {
	for i < len(line) {
		c := line[i]
		if c == '\'' {
			return i + 1
		}
		if c != '\\' || i+1 >= len(line) {
			put(c)
			i++
			continue
		}
		n := line[i+1]
		i += 2
		switch {
		case n == 'x':
			if v, used := number(line[i:], 16, 2); used > 0 {
				put(byte(v))
				i += used
			} else {
				put('\\')
				put(n)
			}
		case n >= '0' && n <= '7':
			v, used := number(line[i-1:], 8, 3)
			put(byte(v))
			i += used - 1
		case n == 'u' || n == 'U':
			max := 4
			if n == 'U' {
				max = 8
			}
			if v, used := number(line[i:], 16, max); used > 0 {
				for _, b := range []byte(string(rune(v))) {
					put(b)
				}
				i += used
			} else {
				put('\\')
				put(n)
			}
		default:
			if m, ok := ansiEscapes[n]; ok {
				put(m)
			} else if strings.IndexByte("\\'\"?", n) >= 0 {
				put(n)
			} else {
				put('\\')
				put(n)
			}
		}
	}
	return i
}

var ansiEscapes = map[byte]byte{'a': 7, 'b': 8, 'e': 27, 'E': 27, 'f': 12, 'n': 10, 'r': 13, 't': 9, 'v': 11}

// number reads up to max digits of the given base from the start of s.
func number(s string, base, max int) (value, used int) {
	for used < len(s) && used < max {
		d := -1
		switch c := s[used]; {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		}
		if d < 0 || d >= base {
			break
		}
		value = value*base + d
		used++
	}
	return value, used
}
