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
//     revision written as git does (HEAD~1, HEAD^, abc1234^2, HEAD~3..HEAD, HEAD^:go.mod) is one too, and so is a
//     branch's with a ^ (main^, origin/main^2) unless its skeleton could fill in a name the guard keeps (zsh's
//     .c^1 is .claude, s^1 the setup key: RWIN8 B2). main~1 stays refused: MAIN~1 is a Windows short name;
//   - a bracket that starts with ! or ^ or holds another [ ([!x], [[:alpha:]], [x-[]), outside quotes, in a
//     pattern that could fill in a name the guard keeps (RWIN7 B1, oddBracket);
//   - zsh's short forms (RWIN7 B2): a word right after ]] or a (( )) with no ; between (if [[ -n x ]] git push),
//     which bash refuses anyway;
//   - zsh's parameter flags, ${( outside single quotes (${(e)…} runs the text it's given), and bash 5's @P in a
//     ${ } (RWIN7 B3), which bash and zsh refuse in the other's reading anyway.
//
// A redirection's target, glued to it or not (>file, 2>file, <file, &>file), is a word of its own, judged as a
// path like any other.

// Two more kinds of word are refused the same way, quoted or not (WIN125j):
//
//   - a word that names zsh's GLOB_DOTS (any case, _ or - anywhere: setopt GLOB_DOTS, options[globdots]=on,
//     zsh --glob-dots), bash's dotglob or GLOBIGNORE: each makes * and ? fill in hidden names (.claude), which the
//     guard's reading of patterns (skeleton.go) takes as never filled in by them. zsh's set -4 is checked with the
//     command (runsText). WIN125k: zsh's GLOB_SUBST and GLOB_ASSIGN too, and ${~x} (RWIN9 B3), which fill in a
//     variable's text as a pattern; and a word that sets HOME, PWD, OLDPWD, CDPATH or DIRSTACK (dirVars, RWIN9 B1);
//   - text the shell runs as a command without eval (RWIN8 B4): a prompt variable (promptVar: PS0–PS4,
//     PROMPT_COMMAND, zsh's PROMPT, RPROMPT and the rest, by their exact names) as a word, assigned or
//     named (read PS4, typeset -n r=PS4); an entry of zsh's tables of functions, aliases, commands, named folders
//     and options (functions[f]='git push'); and a [ with a $( or a backtick after it in the word's own text
//     (x='a[$(git push)]': bash runs it when it does arithmetic with x). A real $( ) leaves no text in the word;
//   - WIN125l (RWIN10 B2, B5): a word that sets a variable a shell reads as it starts (startVars, any BASH_FUNC_…),
//     and a ${NAME=…}, ${NAME:=…} or ${NAME::=…} that sets HOME and the rest, a start variable or a prompt one
//     (expansionAssigns); and a variable's value written in the line that reaches a kept name or folder (lexer.value).

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
	plainBracket = "A bracket that starts with ! or holds a class ([!x], [[:alpha:]]) could fill in a file the guard keeps (the setup key, the guard file, Claude Code's settings). Write the command plainly: spell the name out."
	plainShort   = "zsh runs a word right after ]] or (( )) as a command (if [[ -n x ]] git push). Write the command plainly: put a ; after the test (if [[ -n x ]]; then …; fi)."
	plainDots    = "zsh's GLOB_DOTS, bash's dotglob and GLOBIGNORE make * and ? fill in hidden names (.claude). Write the command without them: name a hidden file outright."
	plainCode    = "The shell runs this text as a command without eval: a prompt variable (PS1–PS4, PROMPT…, RPROMPT…, PROMPT_COMMAND) expands it, zsh's functions[…], aliases[…] and commands[…] make it one, and bash runs a $( ) in a variable's text when it counts with it (x='a[$(…)]'). Write the command plainly, as a command."
	plainParam   = "zsh's parameter flags (${(e)…}, ${(P)…}) and bash's ${x@P} run the text they're given as commands. Write the command plainly, without them."
	plainSubst   = "zsh's ${~x}, GLOB_SUBST and GLOB_ASSIGN fill in a variable's text as a pattern where it's used, out of the guard's sight. Write the command without them: the pattern itself, or the names."
	plainDirVar  = "HOME, PWD, OLDPWD, CDPATH and DIRSTACK (zsh's cdpath and dirstack) move where ~, cd -, cd name and ~1 lead, which could be .claude or the app's own folder. Write the command without setting them: cd to the folder by its path."
	plainBudget  = "The line has more patterns than the guard reads in one go. Write it as shorter commands, or spell the names out."
	plainStart   = "BASH_ENV, ENV, ZDOTDIR, BASH_FUNC_…, SHELLOPTS, BASHOPTS, NULLCMD, READNULLCMD and FPATH make a shell run a file or a command when it starts (or when a line has no command), out of the guard's sight. Write the command without setting them."
	plainValue   = "A variable would hold a path in .claude or the app's own folder, a file the guard keeps, or a pattern that fills one in (x=.claude/agents, read x <<< …, for x in …, a=(…)): where $x is used, the shell reaches it out of the guard's sight. Write the path where it's used instead."
	plainOption  = "zsh's setopt, unsetopt, emulate and set -4 change how the shell reads what follows (patterns, cd, names). The guard allows only NULL_GLOB, CSH_NULL_GLOB, NOMATCH, EXTENDED_GLOB, ERR_EXIT, NO_UNSET, PIPE_FAIL, LOCAL_OPTIONS and INTERACTIVE_COMMENTS. Write the command without the others."
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

// textAsCode is why a word is refused as text the shell would run, or as a switch to hidden names (see above), or "".
func textAsCode(s string) string {
	// zsh ignores _ in an option's name, and reads a - as a _ in one given on its command line (zsh --glob-dots).
	low := optionName.Replace(Fold(s))
	if strings.Contains(low, "globdots") || strings.Contains(low, "dotglob") || strings.Contains(low, "globignore") {
		return plainDots
	}
	if strings.Contains(low, "globsubst") || strings.Contains(low, "globassign") {
		return plainSubst
	}
	if strings.Contains(s, "$mapfile") || strings.Contains(s, "${mapfile") {
		return plainCode // zsh/mapfile's table reads any file as text: ${mapfile[setup-key]}
	}
	if strings.Contains(s, "BASH_FUNC_") {
		return plainStart // env 'BASH_FUNC_ls%%=() { git push; }' bash -c ls (RWIN10 B5)
	}
	if why := expansionAssigns(s); why != "" {
		return why
	}
	n := 0
	for n < len(s) && (s[n] == '_' || isAlnum(rune(s[n])) && (n > 0 || s[n] > '9')) {
		n++
	}
	name, rest := s[:n], s[n:]
	if n > 0 && (rest == "" || rest[0] == '=' || rest[0] == '[' || strings.HasPrefix(rest, "+=")) {
		// A prompt variable's bare name (read PS4) is checkCommand's (setsVarByName): grep -rn PROMPT src/ runs.
		if promptVar(name) && rest != "" || zshTables[name] && rest != "" {
			return plainCode
		}
		if dirVars[name] && rest != "" {
			return plainDirVar
		}
		if startVars[name] && rest != "" && (name != "ENV" || !plainEnvValue(rest)) {
			return plainStart
		}
	}
	if eq := strings.IndexByte(s, '='); eq > 0 && promptVar(s[eq+1:]) {
		return plainCode
	}
	if eq := strings.IndexByte(s, '='); eq > 0 && dirVars[s[eq+1:]] {
		return plainDirVar // typeset -n r=HOME: r is HOME
	}
	if eq := strings.IndexByte(s, '='); eq > 0 && startVars[s[eq+1:]] {
		return plainStart
	}
	if open := strings.IndexByte(s, '['); open >= 0 {
		for i := open + 1; i < len(s); i++ {
			if s[i] == '`' || s[i] == '$' && strings.HasPrefix(s[i:], "$(") && !strings.HasPrefix(s[i:], "$((") {
				return plainCode
			}
		}
	}
	return ""
}

// optionName takes out the _ and - zsh ignores in an option's name.
var optionName = strings.NewReplacer("_", "", "-", "")

// promptVar says whether name is a variable bash or zsh expands as a prompt, $( ) and backticks run (bash always,
// zsh with PROMPT_SUBST): bash's PS0 to PS4 and PROMPT_COMMAND; zsh's PS1 to PS4, PROMPT, PROMPT2 to PROMPT4,
// RPS1, RPS2, RPROMPT, RPROMPT2, SPROMPT and PROMPT_EOL_MARK. Those names exactly (RWIN9 N1): PROMPT_TEMPLATE is a
// constant in a project's code, not a prompt.
func promptVar(name string) bool {
	switch name {
	case "PS0", "PS1", "PS2", "PS3", "PS4", "PROMPT_COMMAND", "PROMPT", "PROMPT2", "PROMPT3", "PROMPT4", "RPS1", "RPS2",
		"RPROMPT", "RPROMPT2", "SPROMPT", "PROMPT_EOL_MARK":
		return true
	}
	return false
}

// dirVars are the variables that move where ~, cd -, cd name, popd and ~1 lead (RWIN9 B1): bash's and zsh's HOME,
// PWD, OLDPWD, CDPATH and DIRSTACK, and zsh's cdpath and dirstack arrays. A word that sets one (HOME=…, CDPATH+=…,
// DIRSTACK[0]=…, typeset -n r=HOME) is refused, quoted or not; a command that sets one by its bare name (read HOME,
// for HOME in …, printf -v HOME) is checkCommand's to refuse (setsVarByName).
var dirVars = map[string]bool{"HOME": true, "PWD": true, "OLDPWD": true, "CDPATH": true, "DIRSTACK": true, "cdpath": true, "dirstack": true}

// startVars are the variables that make a shell run a file or a command by its own reading, when it starts or when a
// line has no command, out of the guard's sight (RWIN10 B5), as one closed list from the manuals: bash's BASH_ENV (the
// file a bash -c runs first), ENV (sh's, dash's, ksh's and an interactive bash's in POSIX mode), SHELLOPTS and
// BASHOPTS (options set as it starts); zsh's ZDOTDIR (where its .zshenv is read, by every zsh), NULLCMD and READNULLCMD
// (the command a line of redirections alone runs: NULLCMD=sh; <<< 'git push'); ksh's and zsh's FPATH and fpath (where
// functions are found); and any word holding BASH_FUNC_ (bash imports a function from BASH_FUNC_name%%). A word that
// sets one (NAME=, NAME+=, ${NAME:=…}, typeset -n r=NAME) is refused; a command that sets one by its bare name (read
// BASH_ENV, for ENV in …, printf -v ZDOTDIR) is checkCommand's to refuse (setsVarByName). ENV is a common name for a
// make or build setting (make ENV=prod), so ENV= is refused only with a value that isn't written plainly (plainEnvValue).
var startVars = map[string]bool{
	"BASH_ENV": true, "ENV": true, "ZDOTDIR": true, "SHELLOPTS": true, "BASHOPTS": true, "NULLCMD": true, "READNULLCMD": true,
	"FPATH": true, "fpath": true,
}

// plainEnvValue says whether ENV='s value (rest, from its = or +=) is a plain word: not empty (ENV=<(…) leaves it so),
// and with no $, backtick, $( ), < or ( in it.
func plainEnvValue(rest string) bool {
	_, value, _ := strings.Cut(rest, "=")
	return value != "" && !strings.ContainsAny(value, "$`<("+string(subMark))
}

// expansionAssigns is why a ${NAME=…}, ${NAME:=…} or zsh's ${NAME::=…} (a subscript after NAME too) in s is refused (RWIN10 B2, B5): it sets NAME
// as an assignment would, so NAME is held to the same lists (dirVars, startVars, promptVar, zshTables); "" for none.
func expansionAssigns(s string) string {
	for i := strings.Index(s, "${"); i >= 0; {
		body := s[i+2:]
		n := 0
		for n < len(body) && (body[n] == '_' || isAlnum(rune(body[n]))) {
			n++
		}
		if name, rest := body[:n], body[n:]; n > 0 {
			if strings.HasPrefix(rest, "[") {
				if end := strings.IndexByte(rest, ']'); end > 0 {
					rest = rest[end+1:] // ${cdpath[1]:=…}
				}
			}
			if strings.HasPrefix(rest, "::=") || strings.HasPrefix(rest, ":=") || strings.HasPrefix(rest, "=") {
				switch {
				case dirVars[name]:
					return plainDirVar
				case startVars[name]:
					return plainStart
				case promptVar(name) || zshTables[name]:
					return plainCode
				case gitVarName(name):
					return plainGitVar
				}
			}
		}
		next := strings.Index(body, "${")
		if next < 0 {
			break
		}
		i += 2 + next
	}
	return ""
}

// zshTables are zsh's special hashes (zsh/parameter) whose entries, once written, are commands or change how
// names are read: functions and aliases (and their dis_ halves, the global and suffix aliases), commands (a
// name's program), nameddirs (~name) and options (GLOB_DOTS among them); and zsh/mapfile's mapfile, which reads
// and writes any file ($mapfile[…] and ${mapfile[…]} are refused wherever they are).
var zshTables = map[string]bool{
	"functions": true, "dis_functions": true, "aliases": true, "dis_aliases": true, "galiases": true, "dis_galiases": true,
	"saliases": true, "dis_saliases": true, "commands": true, "nameddirs": true, "options": true, "mapfile": true,
}

// flag keeps the first reason the line isn't written plainly; commandsAs hands it on as a plainMark command.
func (l *lexer) flag(why, word string) {
	if l.plain == "" {
		l.plain, l.plainWord = why, word
	}
}

// gitRevision is a word git reads as a revision, with ~ and ^ as git means them: a base that is no name a file
// could be confused with (HEAD and its kin, @, a hash), its ~n, ^n and ^{type}, a range of two, and a :path. A
// branch's name (letters, digits and ._@-, parts between single slashes, each ending in a letter or a digit) takes
// ^n and ^{type} first (main^, origin/main^2, RWIN7 N5): to zsh, x^y fills in x at most, and never what's below x/
// (origin/^ is every name in origin). Not ~n: MAIN~1 is a Windows short name (CLAUDE~1 is .claude). gitRevisionWord
// also keeps out a base that holds a name the guard keeps (setup-key^1 is setup-key to zsh).
var gitRevision = func() *regexp.Regexp {
	steps := `(?:~[0-9]*|\^[0-9]*|\^\{[a-z]*\})`
	branch := `[A-Za-z0-9._@-]*[A-Za-z0-9](?:/[A-Za-z0-9._@-]*[A-Za-z0-9])*`
	rev := `(?:(?:HEAD|FETCH_HEAD|ORIG_HEAD|MERGE_HEAD|@|[0-9a-f]{7,40})` + steps + `+|` + branch + `\^(?:[0-9]*|\{[a-z]*\})` + steps + `*)`
	side := `(?:` + rev + `|[A-Za-z0-9._/@-]+)`
	return regexp.MustCompile(`^` + side + `(?:\.\.\.?` + side + `)?(?::[A-Za-z0-9._/-]*)?$`)
}()

// gitRevisionWord is whether a word is a git revision (gitRevision) whose skeleton (skeleton.go) could fill in none
// of the names the guard keeps (RWIN8 B2): the setup key, the token, the guard file, Claude Code's settings and
// their folder. To zsh, x^y is x and then anything, so .c^1 is .claude and s^1 the setup key: one written letter
// of a kept file is enough here (main^ and HEAD^1 start none). budget is the line's (skeletonBudget).
func gitRevisionWord(s string, budget *int) bool {
	if !gitRevision.MatchString(s) {
		return false
	}
	if !isWild(s, nil) {
		return true
	}
	sk := skeletonOf(s, nil)
	return len(skeletonNames(&sk, 1, budget)) == 0
}

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
	// git's variables (gitruns.go): NAME=… in bash, $env:NAME=… in PowerShell, set NAME=… in cmd.
	if why := gitVarWhy(s); why != "" {
		l.flag(why, s)
		return
	}
	if ps {
		return
	}
	if why := textAsCode(s); why != "" {
		l.flag(why, s)
		return
	}
	if len(s) > 1 && unq[0] && s[0] == '=' && strings.Trim(s, "=") != "" {
		l.flag(plainEquals, s)
		return
	}
	if gitRevisionWord(s, &l.skBudget) {
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
				// -m text, -am text, -mtext: the m takes the rest of the word, or the next word. A letter before
				// it that takes a value of its own ends the cluster (RWIN7 N4): in -Fm, m is -F's file.
				m := strings.IndexAny(a[1:], "m"+gitValued[sub]) + 1
				if m == 0 || a[m] != 'm' {
					continue
				}
				if m < len(a)-1 {
					data[i] = true
				} else if i+1 < len(args) {
					data[i+1] = true
					i++
				}
			}
		}
	}
	return data
}

var gitMessages = map[string]bool{"commit": true, "tag": true, "merge": true, "stash": true, "notes": true, "revert": true}

// gitValued are, by subcommand, the short options other than -m that take a value (attached, or the next word):
// commit -F file, -t template, -C and -c commit, -S key; tag -F, -u key; merge -F, -s strategy, -X option, -S;
// notes -F, -C, -c; revert -S.
var gitValued = map[string]string{"commit": "FtCcS", "tag": "Fu", "merge": "FsXS", "notes": "FCc", "revert": "S"}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
