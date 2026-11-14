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
// reading finds a refused command. cmd's line is read after its /c, /k or /r
// however cmd.exe and Git Bash spell it (cmd //c, /c"git push", /q/c: cmdLines). When a << comes after something that
// may open an arithmetic unit, it is read a third time with no here
// documents at all. The body of a here document is read as commands only
// when its command may run it (a shell, ssh, a command we don't know).
// Every word of a command is also checked for the token and the guard file,
// patterns included (gm-t*), each read as its skeleton: every way bash or zsh
// could read it, each character they may give a meaning taken as anything
// (skeleton.go). Anything it can't read as a command
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
//     $(echo git) push, "$(which git)" push, iex 'g'+'it push'). git's
//     subcommand spelled so (git ${EMPTY}push, git $cmd) is refused while
//     the push, merge or deploy group is on, as one it doesn't know;
//   - another language running it (python -c, node -e, perl -e, ruby -e,
//     awk's system()), and xargs -I{} {} or bash -c "$(printf …)", where
//     the command comes from input;
//   - a command git runs, stored in config before the line (a
//     core.fsmonitor or core.pager already set, then git status), and a gh
//     alias (gh alias set p 'pr create' && gh p). A git alias, an extension
//     and any subcommand the guard doesn't know (git p after an earlier git
//     config alias.p push) is refused while the push, merge or deploy group
//     is on (gitKnown). What the line itself gives git is read
//     (gitruns.go): -c, --config-env and GIT_CONFIG_… only for keys that run
//     nothing, git's editor, pager and ssh variables only as true, : or cat,
//     and the options that take a command; a git config that writes a key
//     git or its helpers run is refused by a list of such keys, not an
//     allow-list, and so is any value starting with ! or holding a shell
//     sign;
//   - a here-string or a <( ) given to a sourcing dot (. /dev/stdin <<< 'git push',
//     source <(echo git push));
//   - aws-vault exec p -- aws …, and any wrapper program not listed in rules.go
//     (screen -dm, fakeroot, strace, ltrace, gdb --args, launchctl submit,
//     runuser, proxychains4, reattach-to-user-namespace, asdf exec, volta run,
//     fnm exec, pyenv exec, uvx, pipx run, expect -c, limactl
//     shell, zellij run, entr, at); there are too many to chase;
//   - a file in the app's support folder reached by a glob with fewer than
//     three letters of the folder's name written out, earlier in the path or
//     in a cd before it (cat A*/Ag*/s*, cat ~/*/*/*/*, cd ~/*/*/*; cat gm*):
//     WIN125k refuses a cd, pushd or -C into the folder by its name or a
//     pattern with three of its letters, and a call whose cwd is in it, but
//     not the three-letter rule's other side. Or by a command that reads the
//     whole folder (grep -r . <folder>): the guard tests each word alone. The
//     setup key too. This is not chased with more patterns, because the GM
//     token file is gone by default in 1.24 (the GM gets the token in its
//     environment only, T310) and the deny rules (T306) guard the folder for
//     Edit, Write and Read;
//   - text the lexer can't place: an arithmetic or expansion unit it
//     mistakes, followed by a real here document with a quote in it.
//   - a merge onto a base branch raced against a checkout in another call
//     run at the same moment (merge.go reads the branch just before each
//     call), or done by git config already stored that moves a branch (a
//     remote's fetch refspec into refs/heads, an alias, autocorrect, a
//     sequence editor), or by a rebase's todo list written by hand;
//   - a file written under .git/ (refs, packed-refs, HEAD, config, hooks)
//     by the Write and Edit tools (the deny rules' layer) or by a script the
//     hire writes and runs: this hook reads Bash lines only. With the merge
//     group on, a line that names a path in a git folder is refused unless
//     it only reads (merge.go, N3), but a hire's own script and Write/Edit
//     can still write there. Design A's sandbox closes it (a planned row in
//     SA-X1's Policy);
//   - a path into .git/ the line doesn't spell out (RT126NS0aR3 B3): one
//     reached through xargs (echo .git/refs/heads/x | xargs cp), find
//     -exec, a substitution ($(git rev-parse --git-dir)/refs/heads/main), a
//     variable (d=.gi; cp ${d}t/…) or an archive's rename (tar -s) isn't
//     read by this hook, the same as a hire's own script. Design A's
//     sandbox closes it too.
//
// THE CLOSED LISTS (RWIN9, WIN125k; RWIN10, WIN125l). The guard closes these
// lists, each in full:
//
//   - the ways bash and zsh fill in a pattern or an expansion in a word that
//     holds something written (the skeleton, skeleton.go): * ? [ ] and zsh's
//     forms, and $name, ${…}, $( ), backticks, $(( )) and zsh's $=name,
//     $+name and $#name as runs (its $~ and $^ are refused), in quotes or
//     not; in a --opt=value's value as in any word; and in find's and git's
//     own patterns, where a run is a * (reach.go);
//   - the variables and options that change how they do or where ~ and cd
//     lead (HOME, PWD, OLDPWD, CDPATH, DIRSTACK, cdpath, dirstack; GLOB_DOTS,
//     dotglob, GLOBIGNORE, GLOB_SUBST, GLOB_ASSIGN; setopt and unsetopt but
//     for plainOptions), set by NAME=, NAME+=, ${NAME=…}, ${NAME:=…} and
//     zsh's ${NAME::=…}, typeset -n, or a builtin given the bare name (read,
//     for, printf -v and the rest: setsVarByName);
//   - the builtins of both shells that run text (textRunners, runsText,
//     wrapped), and the variables that make a shell run a file or a command
//     when it starts (BASH_ENV, ENV, ZDOTDIR, BASH_FUNC_…, SHELLOPTS,
//     BASHOPTS, NULLCMD, READNULLCMD, FPATH: startVars);
//   - a variable's value written in the line, in every form (lexer.value):
//     NAME= and NAME+= (leading, or after declare, typeset, local, export,
//     readonly, env), the words of =( ) and +=( ), for's, select's and
//     foreach's words after in, set's, getopts', zstyle's and zparseopts'
//     words, printf -v's and print -v's, a here-string or a here document
//     given to read, mapfile or readarray, and ${NAME:=word}: read as a
//     pattern bash fills in and as a folder (x=.claude/agents is refused);
//   - the moves into a .claude folder or the app's own folder (reach.go): cd,
//     pushd, chdir, PowerShell's Set-Location, sl and Push-Location (and
//     cmd's cd /d), git -C and the other dirOptions, GIT_DIR=; a cd-family
//     folder holding a $ or a backtick is refused whatever it holds (cd
//     "$dir" too), and so is zsh's dirs with a folder;
//   - cmd's switches, as cmd.exe and Git Bash spell them (cmdLines).
//
// Three classes are open, and are named here rather than chased:
//
//   - programs that match names themselves: find and git are read (find's
//     -name, -path and -regex when its action changes something, git's
//     pathspecs for rm, mv, restore, checkout, reset, clean and stash:
//     reach.go), but not find reading through a file (find ~ -name
//     'setup-k*' -exec cat {} +), fd (fd -H settings.json -x rm), git
//     ls-files piped to xargs, git stash -u, rsync's filters, tar
//     --wildcards, cp -r or mv over a folder, a pathspec of the whole tree
//     (git rm -r .), git clean without -x on a settings.json git doesn't
//     ignore, or removing a folder above the project (rm -rf ../app);
//   - words built at run time: a name split across assignments (a=.cla;
//     b=ude; rm $a$b/settings.json), one printed whole by a $( ) (cat
//     $(printf 'setup-%s' key)), a value read from a file or a pipe (read x
//     < f, zsh's echo x | read y), and a word that is expansions alone
//     (${f%.txt}, "$f" as find's or git's pattern too): each is any name
//     when it runs, and refusing them would refuse rm "$f" and mv "$a" "$b".
//     A $ splice in a written name (.clau${x}de) is read: the expansion is a
//     run of the skeleton;
//   - the cd class's three-letter rule (above): once a cd into the folder by
//     its name is refused, a cd through patterns with fewer letters, then a
//     short pattern, still reaches it.
//
// The lasting fix for the setup key and the guard file is the app keeping
// them out of a hire's reach: where a hire's account can neither read nor
// write them (a separate macOS account, or a store only the app's process
// opens), not a guard that reads lines.
//
// tests in guard_test.go name each of these.
package guard

import (
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"
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
	// place is where the line runs (At), for the merge group's @base rules; nil is a place unknown (merge.go).
	place *Place
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

// Fold is text as the Mac's disk and Windows' compare names, case aside: NFC, then full case folding (WIN125h).
// Each letter goes to the least member of its Unicode simple case folding (ſ, the long s, is s; K, the Kelvin
// sign, is k), then to lower case, and the letters that fold to more than one (ß and ẞ are ss, ﬁ is fi: RWIN6
// B2) are spelled out first. NFC's one-letter forms that are ASCII (the Kelvin sign is K, U+037E is ;, U+1FEF is
// `) are taken too. NFC's other changes put an accent on a letter, and an accented letter is never ASCII: every
// name the guard or the demo's fence holds back (the setup key, the token, the guard file, Claude Code's
// settings, a program's name) is ASCII, so these never make one match or miss. A path or argument that isn't
// written plainly (an accent typed apart, a ligature) is refused before any comparison (plain.go).
func Fold(text string) string {
	return strings.Map(func(r rune) rune {
		least := r
		for other := unicode.SimpleFold(r); other != r; other = unicode.SimpleFold(other) {
			least = min(least, other)
		}
		return unicode.ToLower(least)
	}, fullFolds.Replace(text))
}

// fullFolds are Unicode's full case folds of more than one letter where the letters are ASCII (as RWIN6 found them
// on the Mac's disk), ŉ and ǰ (Latin letters the guard's plain words keep out anyway), and NFC's ASCII singletons.
var fullFolds = strings.NewReplacer(
	"\u00df", "ss", "\u1e9e", "ss", "\ufb00", "ff", "\ufb01", "fi", "\ufb02", "fl", "\ufb03", "ffi", "\ufb04", "ffl",
	"\ufb05", "st", "\ufb06", "st", "\u0149", "\u02bcn", "\u01f0", "j\u030c", "\u037e", ";", "\u1fef", "`",
)

// Always refused, whatever the groups say, wherever the text is in the
// line and in any word of it (quotes, escapes and braces taken away, and
// compared folded, Fold: the Mac's disk doesn't mind case): what would make
// a hire the GM (its token, from its file or from the GM's own environment,
// as ps shows it) or switch the hooks off, or would forge the owner's yes:
// the grant record's file and its key's name (desktop/grants.go), the
// paired phones whose tap answers it and their seal's key (phones.json,
// phones-seal: desktop/access.go; the Windows window's key, whose device
// reads as a phone, is windowKeyFile below), which Claude Code the owner has
// (claude-install: desktop/agentcheck.go), whose "npm" would let an unsigned
// Claude Code start where a signed one ran, and the push key where a computer
// with no secret store keeps it, which signs what rings those phones
// (push-key-kept: desktop/pushkey.go). A pattern that fills in a file's name
// (gm-t*) comes out of the lexer as the name itself.
var always = []string{"gm-token", "SMOLCORP_GM_TOKEN", "disableAllHooks", "grants-key", "grants.jsonl",
	"phones.json", "phones-seal", "claude-install", "push-key-kept"}

func namesAlways(text string) string {
	low := Fold(text)
	for _, name := range always {
		if strings.Contains(low, Fold(name)) {
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
// commit message that mentions it still runs. A search for the word as a
// pattern (grep -rn setup-key desktop/) is refused too (RWIN4 B1): which word
// grep, rg or git reads as the pattern turns on option spellings (--reg is
// --regexp), GREP_OPTIONS, rg's config file and git's: a race the guard won't
// win, so the word is refused wherever it stands.
// Nothing in the app can stop a process of the owner's reading a file of the
// owner's; the per-hire deny rules cover Read, and this covers Bash.
const setupKeyFile = "setup-key"

// windowKeyFile is the key the office's window on Windows opens the office with (desktop/paths.go), held the same
// way as the setup key and under its rule (WIN125j: the brief names it among the files no pattern may fill in).
const windowKeyFile = "window-key"

// namesSetupKey is whether any word of the command is the setup key's file.
func namesSetupKey(words simple) bool {
	for _, w := range words {
		low := Fold(strings.ReplaceAll(strings.TrimPrefix(w, unreadable), `\`, "/"))
		// After a /, and after an @ or = (curl's @file, dd's if=file, --file=…); and as a short option's
		// value in the same word (grep -fsetup-key, -rf~/…/setup-key).
		last := low[strings.LastIndexAny(low, "/@=")+1:]
		if setupKeyName(last, strings.HasPrefix(low, "-") && !strings.HasPrefix(low, "--")) {
			return true
		}
	}
	return false
}

// setupKeyName says whether a word's last path part (folded) is the setup key (or the window key) to some system: as it is, and as
// Windows reads it (RWIN5 B3): with the dots and blanks at its end taken off (Win32 drops them: setup-key. and
// "setup-key " are the key), each side of a : (an NTFS stream, setup-key::$DATA; a drive's own folder,
// C:setup-key), or its 8.3 short name (SETUP-~1, or the hashed SE1A2B~1). With option, the part may end with it
// (grep -fsetup-key).
func setupKeyName(last string, option bool) bool {
	for _, part := range strings.Split(last, ":") {
		part = strings.TrimRight(part, ". ")
		if part == setupKeyFile || part == windowKeyFile || setupKeyShortName.MatchString(part) {
			return true
		}
		if option && (strings.HasSuffix(part, setupKeyFile) || strings.HasSuffix(part, windowKeyFile) || setupKeyShortTail.MatchString(part)) {
			return true
		}
	}
	return false
}

// setupKeyShortName is the shape of the setup key's 8.3 name on Windows: its first letters (setup-) or two and a
// hash, then ~ and digits. Folded first, so in lower case.
var (
	setupKeyShortName = regexp.MustCompile(`^(setup-|se[0-9a-f]{4})~[0-9]+$`)
	setupKeyShortTail = regexp.MustCompile(`(setup-|se[0-9a-f]{4})~[0-9]+$`)
)

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
	if r := controlIn(line); r != nil {
		return r
	}
	if name := namesAlways(line); name != "" {
		return &Refusal{Group: "guard", Rule: name, Command: name}
	}
	// The usual reading of the line, and when a << comes after something
	// that may open an arithmetic unit, the pessimist one too.
	pessimist := []bool{false}
	if needsPessimist(line) {
		pessimist = append(pessimist, true)
	}
	type reading struct {
		pess, ps bool
		cmds     []simple
	}
	var readings []reading
	var all [][]simple
	for _, pess := range pessimist {
		for _, ps := range []bool{false, true} {
			cmds := commandsAs(line, 0, ps, pess)
			readings, all = append(readings, reading{pess, ps, cmds}), append(all, cmds)
		}
	}
	// The merge group's place, for this line: whether it is one command alone, whether it moves, and whether it sets
	// git's config (merge.go). Groups never placed are at a place unknown, for this line all the same.
	if i := slices.IndexFunc(groups, func(g Group) bool { return g.place != nil }); i >= 0 {
		groups = At(groups, placeLine(all, *groups[i].place))
	} else if slices.ContainsFunc(groups, func(g Group) bool {
		return slices.ContainsFunc(g.Commands, func(r Rule) bool { return slices.Contains(ruleWords(r), baseRule) })
	}) {
		groups = At(groups, placeLine(all, Place{}))
	}
	for _, r := range readings {
		for _, cmd := range r.cmds {
			if refusal := checkCommand(cmd, groups, 0, r.pess, r.ps); refusal != nil {
				return refusal
			}
		}
	}
	return nil
}

// checkCommand looks at one simple command: the command itself, and what
// any wrapper it is runs. ps says the line was read as PowerShell's.
func checkCommand(words simple, groups []Group, depth int, pess, ps bool) *Refusal {
	if depth > 8 {
		return &Refusal{Group: "guard", Rule: "nesting", Command: strings.Join(words, " ")}
	}
	if len(words) == 3 && words[0] == plainMark {
		return &Refusal{Group: "guard", Rule: notPlainRule, Command: withoutSubs(words[2]), Why: words[1]}
	}
	// raw keeps the subMarks for the folder moves, find's and git's patterns and what a wrapper runs (RWIN10 B2 B4);
	// every other check reads the words as the shell would when a $( ) prints nothing.
	raw := words
	words = stripSubs(words)
	if r := notPlainWords(words); r != nil {
		return r
	}
	for _, w := range words {
		if name := namesAlways(strings.TrimPrefix(w, unreadable)); name != "" {
			return &Refusal{Group: "guard", Rule: name, Command: name}
		}
	}
	all := raw
	// Leading assignments: FOO=1 vercel. Each value is a path too (RWIN9 B3), whatever runs after it: f=.claude/settings.json
	// alone sets what a later rm $f removes.
	for len(words) > 0 && isAssignment(words[0]) {
		raw = raw[1:]
		if namesSetupKey(words[:1]) {
			return &Refusal{Group: "guard", Rule: setupKeyFile, Command: strings.TrimPrefix(words[0], unreadable)}
		}
		if namesGuardFile(words[:1]) {
			return &Refusal{Group: "guard", Rule: guardFile, Command: strings.TrimPrefix(words[0], unreadable)}
		}
		if namesFence(words[:1]) {
			return &Refusal{Group: "guard", Rule: fenceRule, Command: strings.TrimPrefix(words[0], unreadable)}
		}
		if r := gitFolderRefusal(groups, "=", words[:1]); r != nil {
			return r
		}
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
			if strings.Contains(Fold(w), guardFile) {
				return &Refusal{Group: "guard", Rule: guardFile, Command: "> " + strings.TrimPrefix(w, unreadable)}
			}
			if namesFence(simple{w}) {
				return &Refusal{Group: "guard", Rule: fenceRule, Command: "> " + strings.TrimPrefix(w, unreadable)}
			}
			if namesSetupKey(simple{w}) {
				return &Refusal{Group: "guard", Rule: setupKeyFile, Command: "> " + strings.TrimPrefix(w, unreadable)}
			}
		}
		if r := gitFolderRefusal(groups, "", words[1:]); r != nil {
			r.Command = "> " + r.Command
			return r
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
	args, rawArgs := words[1:], raw[1:]
	if strings.HasPrefix(name, "git-") && len(name) > len("git-") {
		// git's own programs by name ($(git --exec-path)/git-push) are git push.
		args = append([]string{name[len("git-"):]}, args...)
		rawArgs = append([]string{name[len("git-"):]}, rawArgs...)
		name = "git"
	}
	if namesSetupKey(words) {
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
	// A move into .claude or the app's folder, and the variables that move ~ and cd (WIN125k, RWIN9 B1).
	if r := dirRefusal(name, all, rawArgs, ps); r != nil {
		return r
	}
	if why := setsVarByName(name, args); why != "" {
		return &Refusal{Group: "guard", Rule: notPlainRule, Command: strings.Join(unmark(words), " "), Why: why}
	}
	if why := setsGitVar(name, unmark(args)); why != "" {
		return &Refusal{Group: "guard", Rule: notPlainRule, Command: strings.Join(unmark(words), " "), Why: why}
	}
	// What git runs from the line (gitruns.go), for every group.
	if name == "git" {
		if why, _ := gitRuns(rawArgs); why != "" {
			return &Refusal{Group: "guard", Rule: gitTextRule, Command: strings.Join(unmark(words), " "), Why: why}
		}
		// One it can't tell what it runs fails closed for the push, merge and deploy groups (RT126NS0aR3 B1).
		if sub, _ := gitSubcommand(unmark(args)); sub != "" && !gitKnown(sub) && guardsGit(groups) {
			return &Refusal{Group: "guard", Rule: gitUnknownRule, Command: strings.Join(unmark(words), " "), Why: gitUnknownWhy}
		}
	}
	// A path in a git folder, while the merge group is on (merge.go, N3).
	if r := gitFolderRefusal(groups, name, words); r != nil {
		return r
	}
	// find's and git's own patterns (RWIN9 B6).
	if rule := keptByMatch(name, rawArgs); rule != "" {
		return &Refusal{Group: "guard", Rule: keptMatchRule, Command: strings.Join(unmark(words), " "), Why: keptMatchWhy}
	}
	if rule := runsText(name, unmark(args)); rule != "" {
		why := textWhy
		if name == "setopt" || name == "unsetopt" || name == "emulate" || strings.HasSuffix(rule, " -4") {
			why = plainOption // RWIN10 N1: they change how the shell reads, rather than run text
		}
		return &Refusal{Group: "guard", Rule: textRule, Command: strings.Join(words, " "), Why: why}
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
	} else if r := checkCommand(agent, groups, depth+1, pess, ps); len(agent) > 0 && r != nil {
		return r
	}
	if name == "tmux" {
		args, rawArgs = tmuxFull(args), tmuxFull(rawArgs)
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
			if !matches(rule, name, plain) {
				continue
			}
			refusal := &Refusal{Group: g.Name, Rule: string(rule), Command: strings.Join(words, " ")}
			if slices.Contains(ruleWords(rule), baseRule) {
				// Only what lands on a base branch, at the line's place (merge.go).
				if refusal.Why = onBase(g.place, plain, depth); refusal.Why == "" {
					continue
				}
			}
			return refusal
		}
	}
	// What a wrapper runs: a line to parse again, or the words after it.
	for _, inner := range wrapped(name, rawArgs) {
		if inner.line != "" {
			for _, cmd := range commandsAs(inner.line, depth+1, inner.ps, pess) {
				if r := checkCommand(cmd, groups, depth+1, pess, inner.ps); r != nil {
					return r
				}
			}
		} else if r := checkCommand(inner.words, groups, depth+1, pess, ps); r != nil {
			return r
		}
	}
	return nil
}

// commandName is how a word names a command: its last path part, folded
// (Fold: the Mac's disk doesn't mind case, and finds verſel as vercel), any @version taken off (npx
// vercel@latest), and a package's name for its command (eas-cli is eas).
func commandName(word string) string {
	// A Windows path: C:\Program Files\Git\bin\git.exe.
	name := Fold(strings.ReplaceAll(word, "\\", "/"))
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
// (folded, Fold: the Mac's disk doesn't mind case).
func namesGuardFile(words simple) bool {
	for _, w := range words {
		if strings.Contains(Fold(w), guardFile) {
			return true
		}
	}
	return false
}

// textRule names a refusal of a command that runs text it's given as a command by a way the guard doesn't read,
// or that changes which program a name runs (RWIN8 B4). Always on, for every hire and the GM.
const (
	textRule = "a command that runs text as a command"
	textWhy  = "The shell would run text this command is given, out of the guard's sight. Write the command plainly, as a command."
)

// textRunners and runsText are one closed list (WIN125k merges RWIN8 B4's zsh list with RWIN9 B5's bash one): the
// builtins and shipped functions of zsh and bash that take text and run it as a command by a way the guard doesn't
// read, or change which program a name runs or how a pattern fills in names. From zsh's manual (zshbuiltins,
// zshmodules, zshcontrib): zpty (a command in a pseudo-terminal), zargs (a command over a list), zmv, zcp and zln (a
// replacement that can run $( )), autoload (a function's text from a file), zmodload (the modules: mapfile reads
// any file as a variable, zpty, system, net/tcp), sched (a command later), fc (history run again, or text through
// an editor, bash's too). By their options or words (runsText): zsh's print -P, zstyle -e, alias -g and -s, hash
// name=path, -p and -d, set and zsh -4 (GLOB_DOTS by its letter), setopt and unsetopt but for the names in
// plainOptions (RWIN9 B4), and emulate in any mode but zsh's (sh and ksh turn GLOB_SUBST on; its -c line is read,
// wrapped). From bash's manual (SHELL BUILTIN
// COMMANDS): compgen and complete with -C or -F (a command, or a function, run for the words), mapfile and readarray
// with -C (a callback run every -c lines), bind -x (a command run on a key) and enable -f (a builtin loaded from a
// file). bash's jobs -x runs its words as a command (wrapped), eval, trap and coproc too, and its ${ …; } and
// ${| …; } run commands where a $( ) would (the lexer reads them).
var textRunners = map[string]bool{
	"zpty": true, "zargs": true, "zmv": true, "zcp": true, "zln": true, "autoload": true, "zmodload": true, "sched": true, "fc": true,
}

// plainOptions are the zsh options setopt and unsetopt may name (RWIN9 B4: WIN125k takes the closed form): those
// that only change what a pattern that fills in nothing does (NULL_GLOB, CSH_NULL_GLOB, NOMATCH, and their NO_
// forms), which a hire's line sometimes needs in zsh, EXTENDED_GLOB, whose forms (^, #, ~) the guard refuses
// anyway (plain.go), and a script's own hygiene (ERR_EXIT, NO_UNSET, PIPE_FAIL, LOCAL_OPTIONS,
// INTERACTIVE_COMMENTS), which touch neither names nor patterns. Any other name, any letter or number (unsetopt +4 is GLOB_DOTS) and -m (by a pattern) is
// refused: a hire's line has no use for them, and some change how patterns fill in names. A name refused as not
// written plainly (GLOB_DOTS) is left to that refusal.
var plainOptions = map[string]bool{"nullglob": true, "cshnullglob": true, "nomatch": true, "extendedglob": true,
	"errexit": true, "unset": true, "pipefail": true, "localoptions": true, "interactivecomments": true}

// runsText is how a command runs text, or changes which program a name runs or how patterns fill in, by a way the
// guard doesn't read (see textRunners); "" for none.
func runsText(name string, args []string) string {
	if textRunners[name] {
		return name
	}
	cluster := func(letters string) bool {
		for _, a := range args {
			if a == "--" || a == "-" {
				return false
			}
			if len(a) > 1 && (a[0] == '-' || a[0] == '+') && a[1] != '-' && strings.ContainsAny(a[1:], letters) {
				return true
			}
		}
		return false
	}
	switch name {
	case "print":
		if cluster("P") {
			return "print -P"
		}
	case "zstyle":
		if cluster("e") {
			return "zstyle -e"
		}
	case "alias":
		if cluster("gs") {
			return "alias -g"
		}
	case "hash":
		if cluster("pd") || slices.ContainsFunc(args, func(a string) bool { return strings.Contains(a, "=") }) {
			return "hash"
		}
	case "set", "zsh":
		for _, a := range args {
			if len(a) > 1 && (a[0] == '-' || a[0] == '+') && a[1] != '-' && strings.Trim(a[1:], "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") == "" && strings.Contains(a, "4") {
				return name + " -4"
			}
		}
	case "setopt", "unsetopt":
		for _, a := range args {
			option := optionName.Replace(Fold(a))
			if !plainOptions[option] && !plainOptions[strings.TrimPrefix(option, "no")] && textAsCode(a) == "" {
				return name + " " + a
			}
		}
	case "emulate":
		for i, a := range args {
			if a == "-c" || i > 0 && (args[i-1] == "-c" || args[i-1] == "-o" || args[i-1] == "+o") {
				break // the line (read by wrapped), or an option's name (refused by its name if it's a dot switch)
			}
			if !strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "+") && Fold(a) != "zsh" {
				return "emulate " + a
			}
		}
	case "compgen", "complete":
		if cluster("CF") {
			return name + " -C"
		}
	case "mapfile", "readarray":
		if cluster("C") {
			return name + " -C"
		}
	case "bind":
		if cluster("x") {
			return "bind -x"
		}
	case "enable":
		if cluster("f") {
			return "enable -f"
		}
	}
	return ""
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
		low := Fold(strings.ReplaceAll(strings.TrimPrefix(w, unreadable), `\`, "/"))
		// The word, and what follows each = in it (X=.claude, --dir=.claude).
		if fencePath(low) {
			return true
		}
		for i := 0; i < len(low); i++ {
			if low[i] == '=' && fencePath(low[i+1:]) {
				return true
			}
		}
	}
	return false
}

// fencePath says whether a path (folded, / between its parts) names Claude Code's settings or their folder, as it
// is and cleaned lexically (cleanWord, RWIN9 B1: .claude/./settings.json, .claude//settings.json). The path before a
// .. after a part may be anywhere (a link), .claude too: .claude/agents/../settings.json and x/../settings.json name
// the settings, and x/.. the folder.
func fencePath(p string) bool {
	if p == "" {
		return false
	}
	if strings.Contains(p, ".claude/settings") || strings.Contains(p, "settings.local.json") {
		return true
	}
	clean, _ := cleanWord(p, nil)
	// The folder itself: rm -rf .claude, mv .claude x.
	if strings.Contains(clean, ".claude/settings") || clean == ".claude" || strings.HasSuffix(clean, "/.claude") {
		return true
	}
	if clean[0] == runOpen {
		_, end := runText(clean, 0)
		rest := strings.TrimPrefix(clean[end+1:], "/")
		first, _, _ := strings.Cut(rest, "/")
		return first == "" || first == "settings.json"
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

// stripSubs is words without their subMarks (withoutSubs).
func stripSubs(words []string) []string {
	out, cloned := words, false
	for i, w := range words {
		if strings.IndexByte(w, subMark) >= 0 {
			if !cloned {
				out, cloned = slices.Clone(words), true
			}
			out[i] = withoutSubs(w)
		}
	}
	return out
}

// unmark takes the unreadable mark off arguments, which are only words.
func unmark(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = strings.TrimPrefix(a, unreadable)
	}
	return out
}

// isAssignment says whether a word is NAME=value, NAME+=value (RWIN10 B3) or NAME[subscript]=value.
func isAssignment(word string) bool {
	word = strings.TrimPrefix(word, unreadable)
	eq := strings.IndexByte(word, '=')
	if eq <= 0 {
		return false
	}
	name := strings.TrimSuffix(word[:eq], "+")
	if open := strings.IndexByte(name, '['); open > 0 && strings.HasSuffix(name, "]") {
		name = name[:open]
	}
	if name == "" {
		return false
	}
	for i, r := range name {
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
		low := Fold(strings.ReplaceAll(a, "\\", "/"))
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
