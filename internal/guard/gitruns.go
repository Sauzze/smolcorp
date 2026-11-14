package guard

import (
	"regexp"
	"slices"
	"strings"
)

// N4 (RT126NS0aR2): git runs text it is given in the line, as sh -c does: config set in the line (core.fsmonitor,
// core.editor, core.sshCommand, an alias…), variables (GIT_EDITOR, GIT_SSH_COMMAND, PAGER…), and options that take
// a command (grep -O, --upload-pack, difftool -x, bisect run…). The guard core reads them for every group, for every
// hire and the GM, failing closed:
//
//   - in-line config (-c, --config-env, GIT_CONFIG_KEY_n, a clone's -c) only for keys that run nothing and move no
//     branch (gitKeyKind: user.*, color.*, advice.*, core.quotepath, commit.gpgsign, init.defaultBranch), or for the
//     keys git runs as a command with the value true, false, : or cat; GIT_CONFIG_PARAMETERS is refused, and
//     GIT_CONFIG_GLOBAL, GIT_CONFIG_SYSTEM and GIT_CONFIG name /dev/null or nothing;
//   - the variables git (or the pager it starts) runs as a command (gitRunVars, any *ASKPASS) only as true, : or cat,
//     in every form a variable is set (plain.go's judge, setsVarByName, ${NAME:=…}, PowerShell's $env:NAME=…);
//   - a git config that writes a key git runs (gitWriteRuns) with anything else, or any value that starts with ! or
//     holds a shell sign (RT126NS0aR3 B2); git config is read as git reads it (readConfig: its legacy form takes
//     options only before the key), taken as a write unless it only reads, and refused when the guard can't read it,
//     or when it renames or removes a section other than user, color, advice and branch (RT126NS0aR4 N1, N2);
//   - an option's command: rebase -x, bisect run, submodule foreach (and submodule--helper's, its internal name),
//     instaweb -d and filter-branch's filters are lines, read as sh -c's is (wrapped; several words with each but
//     the first kept whole, lineOf, RT126NS0aR4 N4); grep -O, --upload-pack, -u, --receive-pack, --exec (ls-remote's
//     hidden one too), difftool and mergetool -x and send-email's commands, by any start of their names git takes
//     (RT126NS0aR4 N3), must be one program's plain name, which is read as a command too (git runs it in a shell with
//     its own words after it), and clone --template is refused;
//   - a subcommand the guard doesn't know (gitKnown: an alias, an extension, git's internal *--helper and other --
//     commands) is refused while the push, merge or deploy group is on, as the guard can't tell what it runs
//     (RT126NS0aR3 B1).

const (
	gitTextRule    = "a command git runs from the line"
	gitUnknownRule = "a git subcommand the guard doesn't know"
	gitUnknownWhy  = "The guard can't tell what this git subcommand runs or moves (an alias, an extension, or one of git's internal commands), so it takes it as a push, a merge or a deploy. Use git's own command for it, or leave it to the owner."
	gitTextWhy     = "git would run text given here (in-line config, an editor, pager or ssh variable, or an option that takes a command) out of the guard's sight. Run it without, or with true, : or cat where git asks for an editor or a pager; in-line config may set only user.*, color.*, advice.*, core.quotepath, commit.gpgsign and init.defaultBranch."
	plainGitVar    = "git runs the value of GIT_EDITOR, GIT_SEQUENCE_EDITOR, EDITOR, VISUAL, GIT_SSH, GIT_SSH_COMMAND, GIT_EXTERNAL_DIFF, GIT_PAGER, PAGER, *ASKPASS and the like as a command, and GIT_CONFIG_* set its config, out of the guard's sight. Set an editor or a pager only to true, : or cat, and config only for user.*, color.*, advice.*, core.quotepath, commit.gpgsign and init.defaultBranch."
)

// The kinds of a git config key.
const (
	keyOther = iota // anything else: refused in the line
	keyInert        // runs nothing and moves no branch
	keyRuns         // its value is a command git runs: only a trivial one
)

// gitKeyKind is the kind of a config key (folded, as git compares sections and names).
func gitKeyKind(key string) int {
	k := Fold(key)
	section := k[:strings.IndexByte(k+".", '.')]
	switch {
	case strings.ContainsAny(k, "$`*?[\\\x01\x02\x04") || !strings.Contains(k, "."):
		return keyOther
	case oneOf("user|color|advice", section), oneOf("core.quotepath|commit.gpgsign|init.defaultbranch", k):
		return keyInert
	case section == "pager", oneOf("core.editor|sequence.editor|core.pager|core.fsmonitor|core.sshcommand|core.askpass|"+
		"diff.external|interactive.difffilter|credential.helper|gpg.program", k):
		return keyRuns
	}
	return keyOther
}

// trivialRun is whether a value git would run as a command runs nothing: true, false, : or cat.
func trivialRun(value string) bool {
	return oneOf("true|false|:|cat", value)
}

// inlineEntryOK is whether a key=value set in the line (with no =, the key is true) may run.
func inlineEntryOK(entry string) bool {
	key, value, has := strings.Cut(entry, "=")
	if !has {
		value = "true"
	}
	switch gitKeyKind(key) {
	case keyInert:
		return true
	case keyRuns:
		return trivialRun(value)
	}
	return false
}

// gitWriteRuns is whether git config writing value to key stores a command for git to run later in the line or after:
// a key git runs with anything but a trivial value, or one of the keys that name a program, a driver, a hook folder,
// an alias, an include or a protocol (a list, as config written to the repository can't be held to an allow-list
// without refusing git config pull.rebase true). Those its helpers run too (RT126NS0aR3 B2): imap-send's tunnel,
// git-lfs's and instaweb's sections, and keys ending in tunnel, httpd, clean, smudge, process, path or args, or in
// defaultKeyCommand (gpg.ssh's, in any section, as a rename can move it: RT126NS0aR4 N2). And, failing closed for a
// key the list misses, any value git or a shell would take as a command: one that starts with ! (a submodule's update
// command, submodule.<name>.update !…, among them), or holds ;, &, |, a backquote, $(, > or <.
func gitWriteRuns(key, value string) bool {
	k := Fold(key)
	section, last := k[:strings.IndexByte(k+".", '.')], k[strings.LastIndexByte(k, '.')+1:]
	switch {
	case gitKeyKind(k) == keyRuns:
		return !trivialRun(value)
	case strings.ContainsAny(k, "$`*?[\\\x01\x02\x04"):
		return true
	case oneOf("alias|include|includeif|filter|protocol|credential|pager|gpg|sendemail|man|browser|difftool|mergetool|"+
		"trailer|uploadpack|lfs|instaweb", section):
		return true
	case strings.HasPrefix(strings.TrimSpace(value), "!") || strings.ContainsAny(value, ";&|`><") || strings.Contains(value, "$("):
		return true
	}
	return oneOf("textconv|command|cmd|driver|helper|program|uploadpack|receivepack|sshcommand|hookspath|fsmonitor|editor|"+
		"pager|gitproxy|alternaterefscommand|askpass|external|difffilter|packobjectshook|vcs|templatedir|proxy|"+
		"tunnel|httpd|clean|smudge|process|path|args|defaultkeycommand", last)
}

// gitRunVars are the variables whose value git, or the program it starts, runs as a command (a shell's line): only
// true, : or cat may be set. Any name ending in ASKPASS too.
var gitRunVars = map[string]bool{
	"GIT_EDITOR": true, "GIT_SEQUENCE_EDITOR": true, "EDITOR": true, "VISUAL": true, "GIT_SSH": true, "GIT_SSH_COMMAND": true,
	"GIT_EXTERNAL_DIFF": true, "GIT_PAGER": true, "PAGER": true, "MANPAGER": true, "LESSOPEN": true, "LESSCLOSE": true,
	"GIT_PROXY_COMMAND": true, "GIT_EXEC_PATH": true, "GIT_TEMPLATE_DIR": true,
}

// gitVarName is whether a variable's name (any case: PowerShell's and Windows' names have none) is one gitVarValue holds.
func gitVarName(name string) bool {
	n := strings.ToUpper(name)
	return gitRunVars[n] || strings.HasSuffix(n, "ASKPASS") || strings.HasPrefix(n, "GIT_CONFIG_KEY_") ||
		oneOf("GIT_CONFIG_PARAMETERS|GIT_CONFIG_GLOBAL|GIT_CONFIG_SYSTEM|GIT_CONFIG|GIT_ALLOW_PROTOCOL", n)
}

// gitVarWhy is why a word that sets one of git's variables (NAME=value, NAME+=…, NAME[…]=…, PowerShell's
// $env:NAME=…), or names one as another's target (typeset -n r=GIT_EDITOR), is refused, or "".
func gitVarWhy(s string) string {
	for _, c := range []string{s, s[strings.LastIndexByte(s, ':')+1:]} {
		n := 0
		for n < len(c) && (c[n] == '_' || isAlnum(rune(c[n])) && (n > 0 || c[n] > '9')) {
			n++
		}
		name, rest := c[:n], c[n:]
		if n == 0 || !gitVarName(name) || !(strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, "+=") || strings.HasPrefix(rest, "[")) {
			continue
		}
		value, set := strings.CutPrefix(rest, "=")
		upper := strings.ToUpper(name)
		switch {
		case !set, upper == "GIT_CONFIG_PARAMETERS":
			return plainGitVar
		case strings.HasPrefix(upper, "GIT_CONFIG_KEY_"):
			if gitKeyKind(value) != keyInert {
				return plainGitVar
			}
		case oneOf("GIT_CONFIG_GLOBAL|GIT_CONFIG_SYSTEM|GIT_CONFIG", upper):
			if value != "" && value != "/dev/null" {
				return plainGitVar
			}
		case upper == "GIT_ALLOW_PROTOCOL":
			if strings.Contains(Fold(value), "ext") {
				return plainGitVar
			}
		case !oneOf("true|:|cat", value):
			return plainGitVar
		}
	}
	if eq := strings.IndexByte(s, '='); eq > 0 && gitVarName(s[eq+1:]) {
		return plainGitVar
	}
	return ""
}

// setsGitVar is why a command that sets one of git's variables by its bare name (read GIT_EDITOR, for EDITOR in …,
// Set-Item env:GIT_EDITOR, [Environment]::SetEnvironmentVariable('GIT_EDITOR', …)) is refused, or "". unset, env -u
// and the commands that only read or print their words may name one.
func setsGitVar(name string, args []string) string {
	if name == "unset" || name == "printenv" || readsGuardFile(name, args) && name != "printf" {
		return ""
	}
	for i, a := range append([]string{name}, args...) {
		if i > 1 && (args[i-2] == "-u" || args[i-2] == "--unset") {
			continue
		}
		a = strings.Trim(a, `'"(),[]`)
		if low := Fold(a); strings.Contains(low, "env:") {
			a = a[strings.Index(low, "env:")+len("env:"):]
		} else if strings.Contains(low, "setenvironmentvariable") {
			for _, b := range args {
				if gitVarName(strings.Trim(b, `'"(),[] `)) {
					return plainGitVar
				}
			}
		}
		if gitVarName(a) {
			return plainGitVar
		}
	}
	return ""
}

// gitRunOpt is an option of a git subcommand that takes a command or config: its letter (0 for none), its long name
// and the fewest letters of it the guard takes as it (min: no more than the fewest git takes, as its --help-all lists
// the subcommand's options, hidden ones too; RT126NS0aR4 N3), whether its value may only be attached (optional), and what
// its value is: 'c' a config entry, 'w' one program's name, 'l' a line, 'x' refused.
type gitRunOpt struct {
	short    byte
	long     string
	min      int
	optional bool
	kind     byte
}

// gitRunSpec is a subcommand's options of that kind, and its other letters that take a value (which end a cluster
// of letters, taking the next word when last) or end one with a value attached at most (ends). perl is a Perl script's
// Getopt::Long reading (send-email): a long option starts with --, - or +, in any case.
type gitRunSpec struct {
	valued, ends string
	opts         []gitRunOpt
	perl         bool
}

var gitRunSpecs = map[string]gitRunSpec{
	// git 2.55's --help-all (RT126NS0aR4 N3): clone's --co is --config and --u --upload-pack; ls-remote's --u is
	// --upload-pack, and its hidden --exec an alias of it; --e is push's, send-pack's, archive's and difftool's --exec
	// or --extcmd.
	"clone": {valued: "objc", opts: []gitRunOpt{{short: 'c', long: "config", min: 2, kind: 'c'},
		{short: 'u', long: "upload-pack", min: 1, kind: 'w'}, {long: "template", min: 2, kind: 'x'}}},
	"fetch": {valued: "jo", opts: []gitRunOpt{{long: "upload-pack", min: 2, kind: 'w'}}},
	"pull":  {valued: "sXjo", ends: "S", opts: []gitRunOpt{{long: "upload-pack", min: 2, kind: 'w'}}},
	"ls-remote": {valued: "o", opts: []gitRunOpt{{short: 'u', long: "upload-pack", min: 1, kind: 'w'},
		{long: "exec", min: 2, kind: 'w'}}},
	"fetch-pack": {opts: []gitRunOpt{{long: "upload-pack", min: 2, kind: 'w'}, {long: "exec", min: 2, kind: 'w'}}},
	"push":       {valued: "o", opts: []gitRunOpt{{long: "receive-pack", min: 3, kind: 'w'}, {long: "exec", min: 1, kind: 'w'}}},
	"send-pack":  {opts: []gitRunOpt{{long: "receive-pack", min: 3, kind: 'w'}, {long: "exec", min: 1, kind: 'w'}}},
	"archive":    {valued: "o", opts: []gitRunOpt{{long: "exec", min: 1, kind: 'w'}}},
	"grep":       {valued: "efABCm", opts: []gitRunOpt{{short: 'O', long: "open-files-in-pager", min: 2, optional: true, kind: 'w'}}},
	"difftool":   {valued: "t", opts: []gitRunOpt{{short: 'x', long: "extcmd", min: 1, kind: 'w'}}},
	"mergetool":  {valued: "t", opts: []gitRunOpt{{short: 'x', long: "extcmd", min: 1, kind: 'w'}}},
	"rebase":     {valued: "sXC", ends: "S", opts: []gitRunOpt{{short: 'x', long: "exec", min: 2, kind: 'l'}}},
	// instaweb starts the web server -d names (git-instaweb.sh: $full_httpd "$conf"): read as a line (B1).
	"instaweb": {valued: "mpb", opts: []gitRunOpt{{short: 'd', long: "httpd", min: 1, kind: 'l'}}},
	"send-email": {perl: true, opts: []gitRunOpt{{long: "sendmail-cmd", min: 2, kind: 'w'}, {long: "to-cmd", min: 3, kind: 'w'},
		{long: "cc-cmd", min: 3, kind: 'w'}, {long: "header-cmd", min: 1, kind: 'w'}}},
	"filter-branch": {opts: []gitRunOpt{{long: "setup", min: 2, kind: 'l'}, {long: "env-filter", min: 2, kind: 'l'},
		{long: "tree-filter", min: 2, kind: 'l'}, {long: "index-filter", min: 2, kind: 'l'}, {long: "parent-filter", min: 2, kind: 'l'},
		{long: "msg-filter", min: 2, kind: 'l'}, {long: "commit-filter", min: 2, kind: 'l'}, {long: "tag-name-filter", min: 2, kind: 'l'}}},
}

// plainProgram is a program's name written plainly: no blank, quote, $, backtick or shell sign.
var plainProgram = regexp.MustCompile(`^[A-Za-z0-9._/+-]+$`)

// gitRuns reads git's args (the words after git, the lexer's marks kept): why the guard core refuses them (""
// for none), and what git runs from them, to be checked as a wrapper's (a line, or one program's name).
func gitRuns(args []string) (why string, out []inner) {
	raw := args
	args = unmark(args)
	takes := valueOptions["git"]
	i := 0
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return "", nil
		}
		if !strings.HasPrefix(a, "-") || len(a) < 2 {
			break
		}
		entry, config, env := "", false, false
		switch {
		case a == "-c" || a == "--config-env":
			if i+1 >= len(args) {
				return "", nil
			}
			i++
			entry, config, env = args[i], true, a == "--config-env"
		case strings.HasPrefix(a, "--config-env="):
			entry, config, env = strings.TrimPrefix(a, "--config-env="), true, true
		case strings.HasPrefix(a, "-c"):
			entry, config = a[2:], true
		case !strings.Contains(a, "=") && takesNext(a, takes):
			i++
		}
		if config && (env && gitKeyKind(entry[:strings.IndexByte(entry+"=", '=')]) != keyInert || !env && !inlineEntryOK(entry)) {
			return gitTextWhy, nil
		}
	}
	if i >= len(args) {
		return "", nil
	}
	sub, rest, rawRest := strings.ToLower(args[i]), args[i+1:], raw[i+1:]
	switch sub {
	case "bisect":
		// git runs bisect run's and foreach's words through the shell as `first "$@"`: the first word is a line, and
		// the others its words, each kept whole (lineOf, RT126NS0aR4 N4: sh -c "git push origin x" is three words).
		if len(rest) > 1 && rest[0] == "run" {
			return "", []inner{{line: lineOf(rest[1], rawRest[2:])}}
		}
		return "", nil
	case "submodule", "submodule--helper":
		// git submodule--helper is git's internal name for it, and runs foreach's line the same way (B1).
		for j, w := range rest {
			if w == "foreach" {
				k := j + 1
				for k < len(rest) && oneOf("--recursive|-q|--quiet|--", rest[k]) {
					k++
				}
				if k >= len(rest) {
					return "", []inner{{line: ""}}
				}
				return "", []inner{{line: lineOf(rest[k], rawRest[k+1:])}}
			}
		}
		return "", nil
	case "config":
		// Read as git reads it (RT126NS0aR4 N1): what the guard can't read is refused, as is a write of a key git runs
		// and a section renamed or removed outside the plain ones (N2).
		c, ok := readConfig(rest)
		if !ok {
			return gitTextWhy, nil
		}
		for _, kv := range c.writes {
			if gitWriteRuns(kv[0], kv[1]) {
				return gitTextWhy, nil
			}
		}
		for _, name := range c.sections {
			if !plainSection(name) {
				return gitTextWhy, nil
			}
		}
		return "", nil
	}
	spec, ok := gitRunSpecs[sub]
	if !ok {
		return "", nil
	}
	judge := func(o gitRunOpt, value string) bool {
		switch o.kind {
		case 'c':
			return inlineEntryOK(value)
		case 'l':
			out = append(out, inner{line: value})
		case 'w':
			if value != "" && !trivialRun(value) {
				if !plainProgram.MatchString(value) {
					return false
				}
				out = append(out, inner{words: simple{value}})
			}
		case 'x':
			return false
		}
		return true
	}
	for j := 0; j < len(rest); j++ {
		a := rest[j]
		switch {
		case a == "--":
			return "", out
		case strings.HasPrefix(a, "--") || spec.perl && len(a) > 1 && (a[0] == '-' || a[0] == '+'):
			given, value, has := strings.Cut(strings.TrimPrefix(a[1:], "-"), "=")
			if spec.perl {
				given = strings.ToLower(given)
			}
			for _, o := range spec.opts {
				if len(given) < o.min || !strings.HasPrefix(o.long, given) {
					continue
				}
				if !has && !o.optional && j+1 < len(rest) {
					j++
					value = rest[j]
				}
				if !judge(o, value) {
					return gitTextWhy, nil
				}
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
		letters:
			for k := 1; k < len(a); k++ {
				c := a[k]
				for _, o := range spec.opts {
					if o.short != c {
						continue
					}
					value := a[k+1:]
					if value == "" && !o.optional && j+1 < len(rest) {
						j++
						value = rest[j]
					}
					if !judge(o, value) {
						return gitTextWhy, nil
					}
					break letters
				}
				switch {
				case strings.IndexByte(spec.ends, c) >= 0:
					break letters
				case strings.IndexByte(spec.valued, c) >= 0:
					if k == len(a)-1 {
						j++
					}
					break letters
				}
			}
		}
	}
	return "", out
}

// configCall is what a git config line does, read as git reads it: whether it opens the editor, the key and value
// pairs it writes, the keys it unsets, and the section names it renames or removes. None of them is a pure read.
type configCall struct {
	edit     bool
	writes   [][2]string
	unsets   []string
	sections []string
}

// configSubs are git config's subcommands (git 2.46 on).
const configSubs = "list|get|set|unset|rename-section|remove-section|edit"

// readConfig is git config's words read as git reads them (RT126NS0aR4 N1). A first word that isn't an option, when it
// is a subcommand's name (exactly), makes the subcommand form: options anywhere after it, as parse-options reads them.
// Otherwise it is the legacy form: options only before the first other word (readHead), so in git config x.y v -l the
// -l is a value pattern, and x.y is written. Every call is taken as a write unless the guard is sure it only reads:
// get, get-all, get-regexp, get-urlmatch, get-color or get-colorbool, list with no other word, one word alone, or the
// get and list subcommands. When options come before a subcommand's name, both forms are read. ok is false when a
// word is an option the guard doesn't know, or a start that fits more than one.
func readConfig(rest []string) (c configCall, ok bool) {
	set := gitOptions["config"]
	sub, subAt := "", -1
	for i, w := range rest {
		if w == "--" || w == "--end-of-options" {
			break
		}
		if !strings.HasPrefix(w, "-") || w == "-" {
			if oneOf(configSubs, w) {
				sub, subAt = w, i
			}
			break
		}
	}
	if sub != "" {
		o, ok := set.read(append(slices.Clone(rest[:subAt]), rest[subAt+1:]...))
		if !ok {
			return c, false
		}
		switch sub {
		case "edit":
			c.edit = true
		case "set":
			c.writes = append(c.writes, [2]string{at(o.pos, 0), at(o.pos, 1)})
		case "unset":
			c.unsets = append(c.unsets, at(o.pos, 0))
		case "rename-section", "remove-section":
			c.sections = append(c.sections, sectionNames(o.pos, sub)...)
		}
		if subAt == 0 {
			return c, true
		}
	}
	o, ok := set.readHead(rest)
	if !ok {
		return c, false
	}
	acted := false
	for _, action := range []string{"get", "get-all", "get-regexp", "get-urlmatch", "get-color", "get-colorbool", "list",
		"add", "replace-all", "unset", "unset-all", "rename-section", "remove-section", "edit"} {
		if !o.set[action] {
			continue
		}
		acted = true
		switch action {
		case "list", "add", "replace-all":
			if action != "list" || len(o.pos) > 0 {
				c.writes = append(c.writes, [2]string{at(o.pos, 0), at(o.pos, 1)})
			}
		case "unset", "unset-all":
			c.unsets = append(c.unsets, at(o.pos, 0))
		case "rename-section", "remove-section":
			c.sections = append(c.sections, sectionNames(o.pos, action)...)
		case "edit":
			c.edit = true
		}
	}
	if !acted && len(o.pos) >= 2 {
		c.writes = append(c.writes, [2]string{o.pos[0], o.pos[1]})
	}
	return c, true
}

// at is words[i], or "" past its end.
func at(words []string, i int) string {
	if i < len(words) {
		return words[i]
	}
	return ""
}

// sectionNames are the names a rename-section (old and new) or remove-section is given; a missing one is "", which
// the guard can't tell.
func sectionNames(pos []string, action string) []string {
	want := 1
	if action == "rename-section" {
		want = 2
	}
	names := slices.Clone(pos)
	for len(names) < want {
		names = append(names, "")
	}
	return names
}

// plainSection is whether a section name a rename-section or remove-section is given (section, or section.subsection)
// is one whose keys run nothing (RT126NS0aR4 N2): user, color, advice or branch. Any other may hold, or come to hold,
// a key git runs (gpg.ssh.defaultKeyCommand, protocol.ext.allow): renamed into or out of, or removed, it is refused.
func plainSection(name string) bool {
	k := Fold(name)
	if k == "" || strings.ContainsAny(k, "$`*?[\\\x01\x02\x04\n") {
		return false
	}
	return oneOf("user|color|advice|branch", k[:strings.IndexByte(k+".", '.')])
}

// gitKnown is whether the guard knows what git's subcommand sub runs and moves (RT126NS0aR3 B1): git's own that it
// reads (gitKeeps, gitOptions, gitRunSpecs), the ones it reads apart, and git-lfs. Left out, and so refused while the
// push, merge or deploy group is on: instaweb (it starts a web server and a browser, from config too; its -d line is
// still read for the other groups), an alias or an extension (what it runs is in config or a program the guard never
// sees), git's internal commands (submodule--helper, web--browse, any with -- in its name), and git's own that run a
// command or a server from their words or config (for-each-repo, hook, merge-index, merge-one-file, daemon, shell,
// http-backend, remote-ext and the other remote helpers, svn, p4, cvs*, archimport, quiltimport, gui, citool).
func gitKnown(sub string) bool {
	_, read := gitOptions[sub]
	_, runs := gitRunSpecs[sub]
	return (gitKeeps[sub] || read || runs || gitReadApart[sub]) && sub != "instaweb"
}

// gitReadApart are the subcommands gitRuns, gitMoves and the groups' rules read by name, and git's own that run
// nothing from their words.
var gitReadApart = map[string]bool{}

func init() {
	for _, sub := range strings.Fields(`bisect submodule config remote stash push send-pack subtree lfs replace fast-import
		fast-export filter-branch replay receive-pack imap-send request-pull whatchanged stage init-db fsck-objects
		pack-redundant update-server-info unpack-file`) {
		gitReadApart[sub] = true
	}
}

// guardsGit is whether groups hold git's unknown subcommands (gitKnown): the push, merge or deploy group is on.
func guardsGit(groups []Group) bool {
	for _, g := range groups {
		if g.On && oneOf("push|deploy|"+MergeGroup, g.Name) {
			return true
		}
	}
	return false
}
