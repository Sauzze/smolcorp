// Package guard is the check every hire's Bash command goes through
// before it runs: Claude Code's PreToolUse hook runs `smolcorp guard`,
// which refuses a command that would push, deploy, publish, type into
// another session or read a secret, wherever it sits in the line: plain, in
// `sh -c '…'`, after `env`, `npx` or `git -C`, after ; && || | or inside
// $( ) and backticks. Deny rules match only how a command starts; this sees
// through the wrappers. A word is refused only as a command, never as a file
// name or an argument: `cat vercel.json` runs, `sh -c 'vercel deploy'` doesn't.
//
// A line is read twice, once as bash and once as PowerShell/cmd (where a
// backslash is part of a path and { } start commands), and refused if either
// reading finds a refused command. When a << comes after something that
// may open an arithmetic unit, it is read a third time with no here
// documents at all. The body of a here document is read as commands only
// when its command may run it (a shell, ssh, a command we don't know).
// Every word of a command is also checked for the token and the guard file,
// patterns included (gm-t*). Anything it can't read as a command
// name (a pattern in a path like /usr/bin/g?t, too many braces, -EncodedCommand, a shell
// reading a pipe) is refused too. The guard fails closed: a call, a file or
// a tool it can't make sense of is denied.
//
// KNOWN BYPASSES, which this guard does not and cannot catch (the deny
// rules, auto mode and a separate macOS account for the hires stay the
// other layers):
//
//   - a script the hire writes and then runs (sh go.sh, . ./x.sh, source
//     x.sh, powershell -File x.ps1, ./ship), because it reads one line,
//     never the files;
//   - a command spelled through a variable or a substitution ($v deploy,
//     $(echo git) push, git ${EMPTY}push, "$(which git)" push, iex 'g'+'it
//     push');
//   - another language running it (python -c, node -e, perl -e, ruby -e,
//     awk's system()), and xargs -I{} {} or bash -c "$(printf …)", where
//     the command comes from input;
//   - a git or gh alias set in config (git -c alias.p=push p, git config
//     alias.p push && git p, gh alias set p 'pr create' && gh p);
//   - a here-string given to a sourcing dot (. /dev/stdin <<< 'git push');
//   - aws-vault exec p -- aws …, and any wrapper program not listed in rules.go
//     (screen -dm, fakeroot, strace, ltrace, gdb --args, launchctl submit,
//     runuser, proxychains4, reattach-to-user-namespace, asdf exec, volta run,
//     fnm exec, pyenv exec, uvx, pipx run, expect -c, script -c, limactl
//     shell, zellij run, entr, at); there are too many to chase;
//   - a file in the app's support folder reached by a glob after a cd, or
//     by a glob earlier in the path (cd <folder>; cat gm*, cat ~/Lib*/App*/A*/g*,
//     cat A*/Ag*/s*), or by a command that reads the whole folder (grep -r .
//     <folder>): the guard tests each word alone. The setup key too. This is not chased with more patterns,
//     because the GM token file is gone by default in 1.24 (the GM gets the
//     token in its environment only, T310) and the deny rules (T306) guard
//     the folder for Edit, Write and Read;
//   - text the lexer can't place: an arithmetic or expansion unit it
//     mistakes, followed by a real here document with a quote in it.
//
// tests in guard_test.go name each of these.
package guard

import (
	"path"
	"strings"
)

// A Rule refuses one command: its name, then the words that must come
// first among its arguments (options skipped), each with | between
// alternatives. A word starting with - is an option, any one of which must
// be given. "..." before the words means they may come anywhere among the
// arguments; "@remote" means any argument is a remote host (host:path).
//
//	"vercel"                      any vercel
//	"eas build|submit|update"     eas build, eas submit, eas update
//	"gh api -X|--method|-f"       gh api with one of those options
//	"tmux ... send-keys|kill-server"
//	"rsync @remote"
type Rule string

// A Group is rules refused together, by name: push, deploy and so on.
type Group struct {
	Name     string `json:"name"`
	On       bool   `json:"on"`
	Commands []Rule `json:"commands"`

	// gm is set by ForGM on the groups of the General Manager's profile. It is
	// not in the guard file or in --print: it only picks the words of a refusal.
	gm bool
}

// Refusal says why a command is refused, or is nil.
type Refusal struct {
	Group   string
	Rule    string
	Command string
	// Why, when set, replaces the usual closing words of the message.
	Why string

	// gm is set by Check when the groups were the General Manager's profile,
	// so the usual words speak to the GM and not to a hire.
	gm bool
}

func (r *Refusal) Error() string {
	if r.Why != "" {
		return "Refused by Smolcorp's guard (" + r.Group + ": " + r.Rule + "): `" + r.Command + "`. " + r.Why
	}
	if r.gm {
		return "Refused by Smolcorp's guard (" + r.Group + ": " + r.Rule + "): `" + r.Command +
			"`. The General Manager doesn't do this; ask the owner."
	}
	return "Refused by Smolcorp's hire guard (" + r.Group + ": " + r.Rule + "): `" + r.Command +
		"`. A hire doesn't do this; ask the GM, who asks the owner."
}

// Always refused, whatever the groups say, wherever the text is in the
// line and in any word of it (quotes, escapes and braces taken away, and
// compared in lower case: the Mac's disk doesn't mind case): what would make
// a hire the GM (its token, from its file or from the GM's own environment,
// as ps shows it) or switch the hooks off. A pattern that fills in the
// token file's name (gm-t*) comes out of the lexer as the name itself.
var always = []string{"gm-token", "SMOLCORP_GM_TOKEN", "disableAllHooks"}

func namesAlways(text string) string {
	low := strings.ToLower(text)
	for _, name := range always {
		if strings.Contains(low, strings.ToLower(name)) {
			return name
		}
	}
	return ""
}

// setupKeyFile is the app's setup key, which opens every setup route of the
// owner's office that a tool may use. A word that is that file (its last
// path part, quotes and escapes taken away, any case: echo setup-key too) or a pattern that fills
// it in (setup-k?y, setup-*) is refused, whatever the command: reading it is
// the harm. Only as a word, not anywhere in the line as the token is, so a
// commit message or a grep for the word in Smolcorp's own code still runs;
// and not as the pattern of a search of other files (searchesForSetupKey).
// Nothing in the app can stop a process of the owner's reading a file of the
// owner's; the per-hire deny rules cover Read, and this covers Bash.
const setupKeyFile = "setup-key"

// namesSetupKey is whether any word of the command is the setup key's file.
func namesSetupKey(words simple) bool {
	for _, w := range words {
		low := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(w, unreadable), `\`, "/"))
		// After a /, and after an @ or = (curl's @file, dd's if=file, --file=…); and as a short option's
		// value in the same word (grep -fsetup-key, -rf~/…/setup-key).
		last := low[strings.LastIndexAny(low, "/@=")+1:]
		if last == setupKeyFile || strings.HasPrefix(low, "-") && !strings.HasPrefix(low, "--") && strings.HasSuffix(last, setupKeyFile) {
			return true
		}
	}
	return false
}

// searchesForSetupKey is whether every word of a command's args that names the setup key is where only a
// search's pattern is (searchPatterns): grep -rn setup-key desktop/, git log -S setup-key. Those search other
// files for the word and never read the key (RWIN3). The key's own path among their files, a --glob or -f
// naming it, or a pattern that fills it in (grep x set*, whose filled-in name follows it as a word of its own)
// is still refused.
func searchesForSetupKey(name string, args []string) bool {
	at := searchPatterns(name, unmark(args))
	for i, a := range args {
		if namesSetupKey(simple{a}) && !at[i] {
			return false
		}
	}
	return len(at) > 0
}

// searchPatterns are the places in args (a command's words after its name) of a search's pattern alone, for
// the commands that search other files for one: grep (egrep, fgrep), rg and git grep, whose pattern is -e's
// (--regexp's) or else the first plain word, and git log's -S, -G and --grep. Every other word is a file, a
// revision or an option's value: -f's file of patterns, rg's --glob, grep's --include.
func searchPatterns(name string, args []string) map[int]bool {
	at := map[int]bool{}
	switch name {
	case "grep", "egrep", "fgrep", "rg":
		grepPatterns(args, 0, grepShortValued[name], at)
	case "git":
		sub, rest := gitSubcommand(args)
		from := len(args) - len(rest)
		switch sub {
		case "grep":
			grepPatterns(rest, from, grepShortValued["git"], at)
		case "log":
			for i := 0; i < len(rest) && rest[i] != "--"; i++ {
				switch a := rest[i]; {
				case a == "-S" || a == "-G" || a == "--grep":
					if i+1 < len(rest) {
						i++
						at[from+i] = true
					}
				case strings.HasPrefix(a, "-S") || strings.HasPrefix(a, "-G") || strings.HasPrefix(a, "--grep="):
					at[from+i] = true
				}
			}
		}
	}
	return at
}

// grepShortValued are, by command, the short options that take a value: the next word, or the rest of the word
// they end (grep's -r is a flag, rg's takes its replacement). An option missing here reads its value as the
// pattern, so the word after it is a file: the safe side.
var grepShortValued = map[string]string{"grep": "efmABCdD", "egrep": "efmABCdD", "fgrep": "efmABCdD", "rg": "efgmtTABCjMrEd", "git": "efmABC"}

// grepLongValued are their long options that take the next word as their value (without an =). grep's
// --color takes one only after an =, so it isn't here: rg's --color never then reads never as the pattern.
var grepLongValued = map[string]bool{
	"--regexp": true, "--file": true, "--max-count": true, "--after-context": true, "--before-context": true, "--context": true,
	"--label": true, "--include": true, "--exclude": true, "--exclude-from": true, "--exclude-dir": true, "--binary-files": true,
	"--devices": true, "--directories": true, "--group-separator": true, "--glob": true, "--iglob": true, "--type": true,
	"--type-not": true, "--type-add": true, "--type-clear": true, "--threads": true, "--max-columns": true, "--replace": true,
	"--encoding": true, "--max-depth": true, "--maxdepth": true, "--max-filesize": true, "--ignore-file": true, "--pre": true,
	"--pre-glob": true, "--sort": true, "--sortr": true, "--path-separator": true, "--colors": true,
	"--engine": true, "--context-separator": true, "--field-match-separator": true, "--field-context-separator": true,
	"--dfa-size-limit": true, "--regex-size-limit": true, "--hostname-bin": true, "--hyperlink-format": true,
}

// grepPatterns marks in at (from on) where a grep's pattern is in args: -e's values, else its first plain word.
// With -f (--file), rg's --files or --type-list, no plain word is a pattern.
func grepPatterns(args []string, from int, valued string, at map[int]bool) {
	given, first := false, -1
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			if first < 0 && i+1 < len(args) {
				first = i + 1
			}
			i = len(args)
		case strings.HasPrefix(a, "--"):
			flag, _, joined := strings.Cut(a, "=")
			if flag == "--regexp" || flag == "--file" || flag == "--files" || flag == "--type-list" {
				given = true
			}
			if !joined && grepLongValued[flag] {
				i++
			}
			if flag == "--regexp" && i < len(args) {
				at[from+i] = true
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			for k := 1; k < len(a); k++ {
				c := a[k]
				if c == 'e' || c == 'f' {
					given = true
				}
				if strings.IndexByte(valued, c) < 0 {
					continue
				}
				if k == len(a)-1 {
					i++
				}
				if c == 'e' && i < len(args) {
					at[from+i] = true
				}
				break
			}
		case first < 0:
			first = i
		}
	}
	if !given && first >= 0 {
		at[from+first] = true
	}
}

// guardFile is the owner's guard file: one edit from a hire would switch
// every group off, as the guard reads it at each command. A command that
// names it is refused unless it is one that only reads (see readsGuardFile),
// so cat, grep, ls and git commit -m are fine. T306's per-hire deny rules
// must also cover Edit and Write of that file and of the hire's own
// .claude/settings.json.
const guardFile = "hire-guard"

// Check is the first refusal for line under groups, or nil.
func Check(line string, groups []Group) *Refusal {
	r := check(line, groups)
	if r != nil {
		for _, g := range groups {
			if g.gm {
				r.gm = true
				break
			}
		}
	}
	return r
}

func check(line string, groups []Group) *Refusal {
	if name := namesAlways(line); name != "" {
		return &Refusal{Group: "guard", Rule: name, Command: name}
	}
	// The usual reading of the line, and when a << comes after something
	// that may open an arithmetic unit, the pessimist one too.
	pessimist := []bool{false}
	if needsPessimist(line) {
		pessimist = append(pessimist, true)
	}
	for _, pess := range pessimist {
		for _, ps := range []bool{false, true} {
			for _, cmd := range commandsAs(line, 0, ps, pess) {
				if r := checkCommand(cmd, groups, 0, pess); r != nil {
					return r
				}
			}
		}
	}
	return nil
}

// checkCommand looks at one simple command: the command itself, and what
// any wrapper it is runs.
func checkCommand(words simple, groups []Group, depth int, pess bool) *Refusal {
	if depth > 8 {
		return &Refusal{Group: "guard", Rule: "nesting", Command: strings.Join(words, " ")}
	}
	for _, w := range words {
		if name := namesAlways(strings.TrimPrefix(w, unreadable)); name != "" {
			return &Refusal{Group: "guard", Rule: name, Command: name}
		}
	}
	// Leading assignments: FOO=1 vercel.
	for len(words) > 0 && isAssignment(words[0]) {
		words = words[1:]
	}
	if len(words) == 0 {
		return nil
	}
	if words[0] == "\x00too-deep" {
		return &Refusal{Group: "guard", Rule: "nesting", Command: "a command nested too deep"}
	}
	if words[0] == "\x00opaque" {
		return &Refusal{Group: "guard", Rule: "a command the guard can't read (encoded)", Command: strings.Join(words[1:], " ")}
	}
	if words[0] == "\x00long" {
		return &Refusal{Group: "guard", Rule: "a command with more words than the guard reads", Command: strings.Join(words[1:], " ")}
	}
	if words[0] == redirInMark {
		if namesSetupKey(words[1:]) {
			return &Refusal{Group: "guard", Rule: setupKeyFile, Command: "< " + strings.TrimPrefix(words[len(words)-1], unreadable)}
		}
		return nil
	}
	if words[0] == redirMark {
		for _, w := range words[1:] {
			if strings.Contains(strings.ToLower(w), guardFile) {
				return &Refusal{Group: "guard", Rule: guardFile, Command: "> " + strings.TrimPrefix(w, unreadable)}
			}
			if namesFence(simple{w}) {
				return &Refusal{Group: "guard", Rule: fenceRule, Command: "> " + strings.TrimPrefix(w, unreadable)}
			}
			if namesSetupKey(simple{w}) {
				return &Refusal{Group: "guard", Rule: setupKeyFile, Command: "> " + strings.TrimPrefix(w, unreadable)}
			}
		}
		return nil
	}
	if words[0] == "\x00stdin" {
		return &Refusal{Group: "guard", Rule: "a shell reading its commands from a pipe", Command: "| sh"}
	}
	if strings.HasPrefix(words[0], unreadable) {
		return &Refusal{Group: "guard", Rule: "a command name the guard can't read", Command: strings.TrimPrefix(strings.Join(words, " "), unreadable)}
	}
	name := commandName(words[0])
	args := words[1:]
	if strings.HasPrefix(name, "git-") && len(name) > len("git-") {
		// git's own programs by name ($(git --exec-path)/git-push) are git push.
		args = append([]string{name[len("git-"):]}, args...)
		name = "git"
	}
	if namesSetupKey(words) && (namesSetupKey(words[:1]) || !searchesForSetupKey(name, args)) {
		return &Refusal{Group: "guard", Rule: setupKeyFile, Command: strings.Join(unmark(words), " ")}
	}
	if namesGuardFile(words) && !readsGuardFile(name, args) {
		return &Refusal{Group: "guard", Rule: guardFile, Command: strings.Join(words, " ")}
	}
	// Claude Code's settings, the same way: the fence of a hire in T3 Code is its worktree's own.
	if namesFence(words) && !readsGuardFile(name, args) {
		return &Refusal{Group: "guard", Rule: fenceRule, Command: strings.Join(words, " ")}
	}
	if rule := clearsIgnored(name, args); rule != "" {
		return &Refusal{Group: "guard", Rule: rule, Command: strings.Join(words, " ")}
	}
	// Always on, like the token and the guard file, and not a group: the owner
	// can't switch it off in the guard editor. A nested Claude Code gets none of
	// the --settings (deny rules, this guard) the hire was started with.
	if startsClaude(name, unmark(args)) {
		return &Refusal{Group: "claude", Rule: "starting Claude Code again", Command: strings.Join(words, " "), Why: nestedClaudeWhy(groups)}
	}
	// smolcorp run starts its agent as the agent itself would start: judged as that command. A role's
	// agent may be Claude Code, and nothing here says which, so `run --role` is starting it again.
	if agent, role := smolcorpRun(name, args); role {
		return &Refusal{Group: "claude", Rule: "starting Claude Code again", Command: strings.Join(words, " "), Why: nestedClaudeWhy(groups)}
	} else if r := checkCommand(agent, groups, depth+1, pess); len(agent) > 0 && r != nil {
		return r
	}
	if name == "tmux" {
		args = tmuxFull(args)
	}
	plain := unmark(args)
	if name == "ps" && showsEnvironment(plain) {
		return &Refusal{Group: "guard", Rule: "ps with the environment", Command: strings.Join(words, " ")}
	}
	for _, g := range groups {
		if !g.On {
			continue
		}
		for _, rule := range g.Commands {
			if matches(rule, name, plain) {
				return &Refusal{Group: g.Name, Rule: string(rule), Command: strings.Join(words, " ")}
			}
		}
	}
	// What a wrapper runs: a line to parse again, or the words after it.
	for _, inner := range wrapped(name, args) {
		if inner.line != "" {
			for _, cmd := range commandsAs(inner.line, depth+1, inner.ps, pess) {
				if r := checkCommand(cmd, groups, depth+1, pess); r != nil {
					return r
				}
			}
		} else if r := checkCommand(inner.words, groups, depth+1, pess); r != nil {
			return r
		}
	}
	return nil
}

// commandName is how a word names a command: its last path part, lower
// case (the Mac's disk doesn't mind case), any @version taken off (npx
// vercel@latest), and a package's name for its command (eas-cli is eas).
func commandName(word string) string {
	// A Windows path: C:\Program Files\Git\bin\git.exe.
	name := strings.ToLower(strings.ReplaceAll(word, "\\", "/"))
	if at := strings.LastIndexByte(name, '@'); at > 0 {
		name = name[:at]
	}
	if alias, ok := aliases[name]; ok {
		return alias
	}
	name = path.Base(name)
	// Windows spells a program with its extension, and PowerShell's .\vercel
	// reaches the lexer as .vercel.
	for _, ext := range []string{".exe", ".cmd", ".bat", ".ps1"} {
		name = strings.TrimSuffix(name, ext)
	}
	name = strings.TrimLeft(name, ".")
	if alias, ok := aliases[name]; ok {
		return alias
	}
	return name
}

var aliases = map[string]string{
	"eas-cli": "eas", "netlify-cli": "netlify", "firebase-tools": "firebase", "@railway/cli": "railway",
	"heroku-cli": "heroku", "supabase-cli": "supabase", "flyctl": "fly", "@cloudflare/wrangler": "wrangler",
	"vercel-cli": "vercel", "aws-cdk": "cdk", "vc": "vercel", "ntl": "netlify", "tofu": "terraform",
}

// namesGuardFile is whether any word of the command names the guard file
// (in lower case: the Mac's disk doesn't mind case).
func namesGuardFile(words simple) bool {
	for _, w := range words {
		if strings.Contains(strings.ToLower(w), guardFile) {
			return true
		}
	}
	return false
}

// fenceRule is how a refusal names Claude Code's settings. A hire in T3 Code
// works inside its fence: its worktree's .claude/settings.local.json, the
// deny rules and this guard as a hook (T797b), ignored by git. A command that
// names a .claude/settings file, a settings.local.json or the .claude folder
// itself is refused unless it only reads (as for the guard file), for every
// hire: no hire rewrites or removes Claude Code's settings. The per-hire
// deny rules already cover Edit and Write of them.
const fenceRule = ".claude/settings"

func namesFence(words simple) bool {
	for _, w := range words {
		low := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(w, unreadable), `\`, "/"))
		if strings.Contains(low, ".claude/settings") || strings.Contains(low, "settings.local.json") {
			return true
		}
		// The folder itself: rm -rf .claude, mv .claude x.
		if folder := strings.TrimRight(low, "/"); folder == ".claude" || strings.HasSuffix(folder, "/.claude") {
			return true
		}
	}
	return false
}

// clearsIgnored is the rule when git would take away every ignored file,
// the fence among them, without naming it: git clean with -x or -X, git
// stash with --all (-a). "" otherwise.
func clearsIgnored(name string, args []string) string {
	if name != "git" {
		return ""
	}
	sub, rest := gitSubcommand(unmark(args))
	var wanted, valued string
	switch sub {
	case "clean":
		wanted, valued = "xX", "e"
	case "stash":
		wanted, valued = "a", "m"
	default:
		return ""
	}
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case a == "--":
			return ""
		case sub == "stash" && a == "--all":
			return "git stash --all"
		case strings.HasPrefix(a, "--") || !strings.HasPrefix(a, "-"):
			continue
		}
		// A cluster of short options (-fdx); the one that takes a value
		// (-e, -m) ends it, with the next word as its value when it's last.
		for k := 1; k < len(a); k++ {
			if strings.IndexByte(wanted, a[k]) >= 0 {
				if sub == "clean" {
					return "git clean -x"
				}
				return "git stash --all"
			}
			if strings.IndexByte(valued, a[k]) >= 0 {
				if k == len(a)-1 {
					i++
				}
				break
			}
		}
	}
	return ""
}

// gitSubcommand is git's subcommand, in lower case, and the words after it.
func gitSubcommand(args []string) (string, []string) {
	takes := valueOptions["git"]
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return "", nil
		case strings.HasPrefix(a, "-") && len(a) > 1:
			if !strings.Contains(a, "=") && takesNext(a, takes) {
				i++
			}
		default:
			return strings.ToLower(a), args[i+1:]
		}
	}
	return "", nil
}

// guardReaders are the commands that may name the guard file, because they
// only read it or print a word. Any other command that names it is refused,
// whatever it does with it: a list of writers can never be complete. An
// output redirection to it is seen apart (redirMark). git is a reader only
// for the subcommands in gitReaders.
var guardReaders = map[string]bool{
	"cat": true, "head": true, "tail": true, "less": true, "more": true, "grep": true, "egrep": true, "fgrep": true, "rg": true,
	"ls": true, "stat": true, "file": true, "wc": true, "diff": true, "cmp": true, "jq": true, "echo": true, "printf": true, "git": true,
	// PowerShell and cmd
	"get-content": true, "gc": true, "type": true, "select-string": true, "sls": true, "get-item": true, "gi": true,
	"get-childitem": true, "gci": true, "dir": true, "write-output": true, "write-host": true,
}

var gitReaders = map[string]bool{"commit": true, "log": true, "diff": true, "show": true, "status": true, "add": true, "blame": true, "grep": true}

func readsGuardFile(name string, args []string) bool {
	switch name {
	case "git":
		pos := positional("git", unmark(args))
		return len(pos) > 0 && gitReaders[strings.ToLower(pos[0])]
	case "smolcorp":
		pos := positional("smolcorp", unmark(args))
		return len(pos) > 0 && pos[0] == "guard"
	}
	return guardReaders[name]
}

// unmark takes the unreadable mark off arguments, which are only words.
func unmark(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = strings.TrimPrefix(a, unreadable)
	}
	return out
}

func isAssignment(word string) bool {
	word = strings.TrimPrefix(word, unreadable)
	eq := strings.IndexByte(word, '=')
	if eq <= 0 {
		return false
	}
	for i, r := range word[:eq] {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// showsEnvironment is whether ps's options show other processes'
// environments (BSD's e, or -E), where the GM's token is.
func showsEnvironment(args []string) bool {
	for i, a := range args {
		letters := strings.TrimPrefix(a, "-")
		if (strings.HasPrefix(a, "-") || i == 0) && !strings.HasPrefix(a, "--") && strings.ContainsAny(letters, "eE") &&
			strings.Trim(letters, "abcdeEfghjklmMOopqrSTtUuvwxXZ0123456789,") == "" {
			return true
		}
	}
	return false
}

// startsClaude is whether the command runs Claude Code: claude by name or
// by path, the package's own name (npx @anthropic-ai/claude-code; the package
// runners hand their command here through wrapped), or node running its
// cli.js or the claude link in a bin folder. `claude` as an argument is not a
// command and never gets here.
func startsClaude(name string, args []string) bool {
	if name == "claude" || name == "claude-code" {
		return true
	}
	if name != "node" && name != "nodejs" && name != "bun" && name != "deno" {
		return false
	}
	for _, a := range args {
		low := strings.ToLower(strings.ReplaceAll(a, "\\", "/"))
		if strings.Contains(low, "claude-code/cli") || strings.Contains(low, "@anthropic-ai/claude-code") {
			return true
		}
		// node ./node_modules/.bin/claude: a path, not the word claude as an argument.
		if strings.Contains(low, "/") && !strings.HasPrefix(low, "-") && commandName(a) == "claude" {
			return true
		}
	}
	return false
}

// smolcorpRun reads `smolcorp run`: the agent's command and its arguments, after the launcher's own
// options (--hire, --hire-id, --pinned, --agent, which the app gives a hire it opens), or role when it
// starts a role's agent (run --role). Neither when the command isn't smolcorp run.
func smolcorpRun(name string, args []string) (agent simple, role bool) {
	if name != "smolcorp" && name != "smolcorp-cli" || len(args) == 0 || !strings.EqualFold(strings.TrimPrefix(args[0], unreadable), "run") {
		return nil, false
	}
	rest := args[1:]
	for len(rest) > 0 {
		switch strings.ToLower(strings.TrimPrefix(rest[0], unreadable)) {
		case "--hire", "--hire-id", "--agent":
			rest = rest[min(2, len(rest)):]
		case "--pinned":
			rest = rest[1:]
		case "--role":
			return nil, true
		default:
			return simple(rest), false
		}
	}
	return nil, false
}

func nestedClaudeWhy(groups []Group) string {
	for _, g := range groups {
		if g.gm {
			return "The GM doesn't start another Claude Code; it hires through Smolcorp (smolcorp gm hire) so the hire gets its fence."
		}
	}
	return "A hire doesn't start another Claude Code: its fence (deny rules and this guard) wouldn't come with it."
}
