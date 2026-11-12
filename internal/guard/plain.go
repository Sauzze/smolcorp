package guard

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// WIN125g: six rounds found spellings the shell or the disk reads as a name and a text guard doesn't (ſ, ß, ﬁ,
// a NUL in $'…', zsh's =cmd and (a|b)). The guard stops chasing them: a line that holds one of these forms is
// refused, for every managed hire and the GM (and so in the demo, whose fence asks the guard first), and the
// refusal asks for the command written plainly. Each form below is refused wherever it is:
//
//   - a control character (a NUL, an escape, a lone carriage return), or text that isn't UTF-8: anywhere;
//   - $'…' quoting, outside other quotes: anywhere (a NUL in it ends a name, its escapes build any byte);
//   - a letter outside ASCII outside quotes: anywhere. Inside quotes, only in data, and data is exactly:
//     every argument of echo and printf (and PowerShell's write-output, write-host), and the value of git's
//     message option for commit, tag, merge, stash, notes and revert (-m text, -mtext, --message text,
//     --message=text, a cluster ending in m: -am text). A
//     program's name, a path, a redirection's target, a NAME=value and any other argument stay ASCII;
//   - a word that starts with = (zsh's =git is git's full path): anywhere outside quotes, and as a command's
//     name in any case. A word of = alone (test's = and ==) is no program;
//   - zsh's glob forms outside quotes (zsh reads them with EXTENDED_GLOB, and a hire's shell may set it): a ( glued
//     to a word or opening a word after a command's name ((a|b), x(N), bash's @(…) too), a ^, a ~ inside a word
//     (not after = or :, where it is home), a # inside a word (not $#). NAME=( … ) is an array, and a git
//     revision written as git does (HEAD~1, HEAD^, abc1234^2, HEAD~3..HEAD, HEAD^:go.mod) is one too.
//
// A redirection's target, glued to it or not (>file, 2>file, <file, &>file), is a word of its own, judged as a
// path like any other.

// notPlainRule names these refusals.
const notPlainRule = "a command not written plainly"

// plainMark starts the three-word command that stands for a line not written plainly: the mark, why, the word.
const plainMark = "\x00plain"

const (
	plainControl = "It holds a control character (a NUL, an escape, a lone carriage return) or bytes that aren't UTF-8. Write the command plainly."
	plainANSI    = "$'…' quoting builds bytes the guard won't decode: a NUL in it ends a name. Write the command plainly, with plain quotes."
	plainLetter  = "It holds a letter outside ASCII in a program's name, a path or an argument, which the disk may read as another name (ß as ss, ﬁ as fi). Write the command plainly: names and paths in ASCII; other letters only inside quotes, in a git commit message (-m) or in echo or printf text."
	plainEquals  = "zsh reads a word that starts with = as a program's full path (=git is /usr/bin/git). Write the command plainly: the program's name alone."
	plainGlob    = "zsh reads ( ), ^, # or ~ in a word as a pattern that fills in names from the disk. Write the command plainly: put that word in quotes, or spell the name out."
)

// controlIn is the line refused when it holds a control character or isn't UTF-8: a tab and a line end
// (\n, \r\n) are its only ones. The lexer's own marks (\x00, \x01) can't come from a line.
func controlIn(line string) *Refusal {
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRuneInString(line[i:])
		switch {
		case r == utf8.RuneError && size <= 1,
			r < 0x20 && r != '\t' && r != '\n' && !(r == '\r' && strings.HasPrefix(line[i+1:], "\n")),
			r >= 0x7f && r <= 0x9f:
			return &Refusal{Group: "guard", Rule: notPlainRule, Command: "a control character", Why: plainControl}
		}
		i += size
	}
	return nil
}

// flag keeps the first reason the line isn't written plainly; commandsAs hands it on as a plainMark command.
func (l *lexer) flag(why, word string) {
	if l.plain == "" {
		l.plain, l.plainWord = why, word
	}
}

// gitRevision is a word git reads as a revision, with ~ and ^ as git means them: a base that is no name a file
// could be confused with (HEAD and its kin, @, a hash), its ~n, ^n and ^{type}, a range of two, and a :path.
var gitRevision = func() *regexp.Regexp {
	rev := `(?:HEAD|FETCH_HEAD|ORIG_HEAD|MERGE_HEAD|@|[0-9a-f]{7,40})(?:~[0-9]*|\^[0-9]*|\^\{[a-z]*\})+`
	side := `(?:` + rev + `|[A-Za-z0-9._/@-]+)`
	return regexp.MustCompile(`^` + side + `(?:\.\.\.?` + side + `)?(?::[A-Za-z0-9._/-]*)?$`)
}()

// judge flags a lexed word that isn't written plainly: a letter outside ASCII outside quotes (both readings),
// and in bash's a leading = or a zsh glob form. unq says, for each byte, whether it was outside quotes.
func (l *lexer) judge(word []byte, unq []bool, ps bool) {
	s := string(word)
	for i := 0; i < len(s); i++ {
		if unq[i] && s[i] >= 0x80 {
			l.flag(plainLetter, s)
			return
		}
	}
	if ps {
		return
	}
	if len(s) > 1 && unq[0] && s[0] == '=' && strings.Trim(s, "=") != "" {
		l.flag(plainEquals, s)
		return
	}
	if gitRevision.MatchString(s) {
		return
	}
	for i := 0; i < len(s); i++ {
		if !unq[i] {
			continue
		}
		switch s[i] {
		case '^':
			l.flag(plainGlob, s)
		case '~':
			if i > 0 && s[i-1] != '=' && s[i-1] != ':' {
				l.flag(plainGlob, s)
			}
		case '#':
			if i > 0 && s[i-1] != '$' {
				l.flag(plainGlob, s)
			}
		}
	}
}

// paren flags a ( that bash would take for a subshell and zsh for a glob: one glued to a word (x(N), a(b|c),
// @(…)), or one that opens a word after a command's name (cat (a|b)). NAME=( and NAME+=( start an array, and a (
// after only reserved words (if, then, !, …) is a subshell.
func (l *lexer) paren(word []byte, inWord bool, cur simple) {
	if inWord {
		s := string(word)
		if name := strings.TrimSuffix(strings.TrimSuffix(s, "="), "+"); !strings.HasSuffix(s, "=") || !isAssignment(name+"=") {
			l.flag(plainGlob, s+"(")
		}
		return
	}
	for _, w := range cur {
		if !shellReserved[w] {
			l.flag(plainGlob, "(")
			return
		}
	}
}

var shellReserved = map[string]bool{"!": true, "if": true, "then": true, "else": true, "elif": true, "do": true, "while": true, "until": true, "time": true, "{": true}

// notPlainWords is a simple command refused for a letter outside ASCII where it isn't data (see the top of this
// file), or for a command name that starts with =; nil when it's plain. The lexer has refused such a letter
// outside quotes already: this is the one inside quotes.
func notPlainWords(words simple) *Refusal {
	refuse := func(why, word string) *Refusal {
		return &Refusal{Group: "guard", Rule: notPlainRule, Command: strings.TrimPrefix(word, unreadable), Why: why}
	}
	if len(words) == 0 {
		return nil
	}
	if words[0] == redirMark || words[0] == redirInMark {
		for _, w := range words[1:] {
			if !isASCII(w) {
				return refuse(plainLetter, w)
			}
		}
		return nil
	}
	if strings.HasPrefix(words[0], "\x00") {
		return nil
	}
	rest := words
	for len(rest) > 0 && isAssignment(rest[0]) {
		if !isASCII(rest[0]) {
			return refuse(plainLetter, rest[0])
		}
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return nil
	}
	command := strings.TrimPrefix(rest[0], unreadable)
	if !isASCII(command) {
		return refuse(plainLetter, command)
	}
	if strings.HasPrefix(command, "=") && strings.Trim(command, "=") != "" {
		return refuse(plainEquals, command)
	}
	data := dataArgs(commandName(command), unmark(rest[1:]))
	for i, w := range rest[1:] {
		if !isASCII(w) && !data[i] {
			return refuse(plainLetter, w)
		}
	}
	return nil
}

// dataArgs says which of a command's arguments are data, text it prints or stores and never looks up by name:
// every argument of echo and printf, and the value of git's message option for the subcommands that take one.
func dataArgs(name string, args []string) []bool {
	data := make([]bool, len(args))
	switch name {
	case "echo", "printf", "write-output", "write-host":
		for i := range data {
			data[i] = true
		}
	case "git":
		sub, rest := gitSubcommand(args)
		if !gitMessages[sub] {
			return data
		}
		for i := len(args) - len(rest); i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--":
				return data
			case a == "--message":
				if i+1 < len(args) {
					data[i+1] = true
					i++
				}
			case strings.HasPrefix(a, "--message="):
				data[i] = true
			case strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--"):
				// -m text, -am text, -mtext: the m takes the rest of the word, or the next word.
				if m := strings.IndexByte(a, 'm'); m > 0 {
					if m < len(a)-1 {
						data[i] = true
					} else if i+1 < len(args) {
						data[i+1] = true
						i++
					}
				}
			}
		}
	}
	return data
}

var gitMessages = map[string]bool{"commit": true, "tag": true, "merge": true, "stash": true, "notes": true, "revert": true}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
