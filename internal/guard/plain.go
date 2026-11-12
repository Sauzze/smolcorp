package guard

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// WIN125g: six rounds found spellings the shell or the disk reads as a name and a text guard doesn't (ſ, ß, ﬁ,
// a NUL in $'…', zsh's =cmd and (a|b)). The guard stops chasing them: a line that holds one of these forms is
// refused, for every managed hire and the GM (and so in the demo, whose fence asks the guard first), and the
// refusal asks for the command written plainly. Each form below is refused wherever it is:
//
//   - a control character (a NUL, an escape, a lone carriage return), or text that isn't UTF-8: anywhere;
//   - $'…' quoting, outside other quotes: anywhere (a NUL in it ends a name, its escapes build any byte);
//   - a program's name with a letter outside ASCII, quoted or not, after any wrapper (env, command, exec, …);
//   - a path or an argument (a redirection's target and a NAME=value too), quoted or not, that isn't written
//     plainly (WIN125h, plainWord): one whose letters outside ASCII aren't all plain Latin letters (é, È, ç, ñ, ø,
//     ő: what the owner's French clients name their folders). That refuses, as Unicode words it: a word that NFKC
//     changes (ﬁ, ﬂ, fullwidth Ｆ, the Kelvin sign, ĳ), one whose full case folding isn't its lower case (ß is ss,
//     ſ is s), any other script (a Cyrillic а that looks like a), a combining accent (é written as e and an
//     accent, which NFC would change), and zero-width or bidi characters. Data takes any letter inside quotes,
//     and data is exactly: every argument of echo and printf (and PowerShell's write-output, write-host), and the
//     value of git's message option for commit, tag, merge, stash, notes and revert (-m text, -mtext, --message
//     text, --message=text, a cluster ending in m: -am text). Outside quotes, data is a word like any other;
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
	plainLetter  = "It holds a letter the disk may read as another name (ß as ss, ﬁ as fi), a letter from another script, an invisible character, or a letter outside ASCII in a program's name. Write the command plainly: program names in ASCII; paths and arguments in ASCII or accented Latin letters (é, è, ç, à), each typed as one letter; anything else only inside quotes, in a git commit message (-m) or in echo or printf text."
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

// judge flags a lexed word that isn't written plainly: outside quotes, a letter outside ASCII that isn't a plain
// Latin letter (both readings, plainLatin), and in bash's a leading = or a zsh glob form. unq says, for each byte,
// whether it was outside quotes. A program's name with any letter outside ASCII is notPlainWords' to refuse.
func (l *lexer) judge(word []byte, unq []bool, ps bool) {
	s := string(word)
	for i, r := range s {
		if unq[i] && r >= 0x80 && !plainLatin(r) {
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

// notPlainWords is a simple command refused for a word not written plainly where it isn't data (see the top of
// this file), or for a command name with a letter outside ASCII or that starts with =; nil when it's plain. The
// lexer has judged the words outside quotes already: this judges them whole, quotes taken away. A wrapper's
// command (env x, command x, sh -c 'x') comes back here as a command of its own, so its name is held to ASCII too.
func notPlainWords(words simple) *Refusal {
	refuse := func(why, word string) *Refusal {
		return &Refusal{Group: "guard", Rule: notPlainRule, Command: strings.TrimPrefix(word, unreadable), Why: why}
	}
	if len(words) == 0 {
		return nil
	}
	if words[0] == redirMark || words[0] == redirInMark {
		for _, w := range words[1:] {
			if !PlainWord(w) {
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
		if !PlainWord(rest[0]) {
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
		if !data[i] && !PlainWord(w) {
			return refuse(plainLetter, w)
		}
	}
	return nil
}

// PlainWord is whether a path or an argument is written plainly (WIN125h): every letter is ASCII or a plain Latin
// letter (plainLatin). Such a word is its own NFC and NFKC, its full case folding is its lower case, and it holds
// no other script, combining mark, zero-width or bidi character, so the disk reads it as written. The demo's fence
// asks it of a tool's path. Control characters are controlIn's.
func PlainWord(word string) bool {
	for _, r := range word {
		if r >= 0x80 && !plainLatin(r) {
			return false
		}
	}
	return true
}

// plainLatin is whether a letter outside ASCII may stand in a path or an argument: an upper or lower case Latin
// letter of Latin-1 Supplement or Latin Extended-A and B (U+00C0 to U+024F), precomposed (é as one letter), but
//   - not one NFKC changes or whose full case folding isn't its lower case (notPlainLatin, from Unicode's tables:
//     TestPlainLatinIsUnicodes pins it), since the disk may read it as other letters;
//   - not one whose other case is ASCII (ı is I upper-cased, İ is i lower-cased), as a disk that ignores case may
//     take it for that letter;
//   - not the clicks and the two with stroke (ǀ ǁ ǂ ǃ ƻ, other letters), which look like | ! 2.
func plainLatin(r rune) bool {
	if r < 0xC0 || r > 0x24F || !(unicode.IsUpper(r) || unicode.IsLower(r)) || strings.ContainsRune(notPlainLatin, r) {
		return false
	}
	for _, c := range []rune{unicode.ToUpper(r), unicode.ToLower(r), unicode.ToTitle(r)} {
		if c < 0x80 {
			return false
		}
	}
	return true
}

// notPlainLatin are the letters of U+00C0 to U+024F that NFKC changes (Ĳ ĳ Ŀ ŀ ŉ ſ Ǆ to ǌ, Ǳ ǲ ǳ) or whose full case
// folding isn't their lower case (ß, ŉ, ſ, ǰ), as Unicode 16's tables give them.
const notPlainLatin = "\u00df\u0132\u0133\u013f\u0140\u0149\u017f\u01c4\u01c5\u01c6\u01c7\u01c8\u01c9\u01ca\u01cb\u01cc\u01f0\u01f1\u01f2\u01f3"

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
