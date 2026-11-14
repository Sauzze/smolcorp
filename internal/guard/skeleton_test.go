package guard

import (
	"strings"
	"testing"
	"time"
)

// WIN125j (RWIN8): every row of RWIN8's four tables is refused, for every managed hire and the GM. The guard reads
// a pattern as its skeleton (skeleton.go) instead of modelling the shell's patterns.
func TestRWIN8sRowsAreRefused(t *testing.T) {
	const dir = `~/Library/Application\ Support/AgentDeck/`
	rows := map[string][]string{
		// B1: a [ inside a bracket (the end of a range, x-[), and zsh's [!].
		"B1": {
			"cat " + dir + "setup-k[!x-[:x:]y", "cat " + dir + "setup-k[!x-[:alpha:]y", "cat <" + dir + "setup-k[!x-[:x:]y",
			"echo '{}' > " + dir + "hire-gu[!x-[:x:]rd.json", "rm " + dir + "hire-gu[!x-[:x:]rd.json",
			"rm .claude/s[!x-[:x:]ttings.local.json", "echo '{}' > .claude/s[!x-[:x:]ttings.json", "rm -rf .c[!x-[:x:]aude",
			"cat " + dir + "gm-t[!x-[:x:]ken", "cat " + dir + "setup-k[ee-[:x:]y", "rm -rf .c[ll-[:x:]aude",
			"cat " + dir + "setup-k[!]y", "rm -rf .c[!]aude", "cat " + dir + "setup-k[![:x:]y",
			// Readings no single close finds: a class then a letter in one bracket, and two brackets.
			"cat [[:alpha:]x]etup-ke[y]", "cat [s]etup-ke[y]", "cat s[!x-[:x:]tup-k[e]y", "cat *setup-key*",
		},
		// B2: zsh's x^y is x and then anything.
		"B2": {
			"rm -rf .c^1", "rm -rf .c^", "rm -rf .cl^{tree}", "mv .c^1 /tmp/x", "setopt extendedglob; rm -rf .c^1",
			"setopt extendedglob; rm -rf .c^", "setopt extendedglob; mv .c^1 /tmp/x", "cat s^1", "cat se^1", "cat hire-g^1",
			"cat gm^1", "rm -rf .claude^1", "cat .c^1/settings.json", "echo '{}' > .c^1/settings.json",
			"cat " + dir + "s^1", "rm -rf .c~1",
		},
		// B3: a pattern earlier in the path than the last part.
		"B3": {
			"rm .c?aude/settings.json", "echo '{}' > .c?aude/settings.json", "rm .c[!x]aude/settings.json",
			"rm .c[[:alpha:]]aude/settings.json", "rm .cl*/settings.json", "rm .c?aude/settings.local.json",
			"rm .*/settings.local.json", "cp /dev/null .c*/settings.local.json", "cat ~/Lib*/App*/A*/s*",
		},
		// B4: text the shell runs without eval or -c.
		"B4": {
			"PS4='$(git push)'; set -x; :", "PS4='$(ssh host)'; set -x; :", "export PS4='`git push`'; set -x; :",
			"setopt promptsubst xtrace; PS4='$(git push)'; :", "setopt promptsubst; print -P '$(git push)'",
			"setopt promptsubst; print -P '$(cat setup-key)'", "zstyle -e ':a' b 'git push'; zstyle -s ':a' b x",
			"zmodload zsh/zpty; zpty w 'git push'", "functions[f]='git push'; f", "functions[f]='cat setup-key'; f",
			"aliases[P]='git push'; eval P", "alias -g P=push; eval 'git P'", "autoload -Uz zargs; zargs -- origin -- git push",
			"autoload -U zmv; zmv '(*)' '$(git push)'", "eval -- 'git push'", "eval -- 'ssh host'",
			`zmodload zsh/mapfile; echo "${mapfile[/Users/ada/Library/Application Support/AgentDeck/setup-key]}"`,
			`zmodload zsh/mapfile; echo "${mapfile[$HOME/Library/Application Support/AgentDeck/setup-key]}"`,
			"echo ${mapfile[setup-key]}",
			// For comparison, refused before too.
			"eval 'git push'", "builtin eval 'git push'", "trap 'git push' EXIT", "eval -- 'cat setup-key'",
			"mapfile[.claude/settings.json]='{}'",
		},
		// Found on the way (WIN125j): hidden names by dot globbing, a program by another name, bash's arithmetic.
		"more": {
			"setopt globdots; rm -rf ?claude", "setopt GLOB_DOTS; rm -rf *", "set -o globdots; rm -rf *", "set -4; rm -rf ?claude",
			"options[globdots]=on; rm -rf *", "unsetopt noglobdots; rm -rf *", "shopt -s dotglob; rm -rf *",
			"GLOBIGNORE=x; rm -rf ?claude", "zsh -4 -c 'rm -rf ?claude'", "hash g=/usr/bin/git; g push",
			"hash -p /usr/bin/git g; g push", "commands[g]=/usr/bin/git; g push", "x='a[$(git push)]'; (( x ))",
			"x='a[`git push`]'; [[ $x -eq 1 ]]", "read PS4 <<< x", "printf -v PS4 x", "typeset -n r=PS4",
			"PROMPT_COMMAND='git push'", "RPROMPT='$(git push)'", "sched +0 git push", "fc -e - git", "zcp a b",
			// The office window's key on Windows, among the names no pattern may fill in.
			"cat window-key", "cat WINDOW~1", "cat window-k*", "cat " + dir + "w*",
		},
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for table, lines := range rows {
			for _, line := range lines {
				if Check(line, groups) == nil {
					t.Errorf("%s: let through %q", table, line)
				}
			}
		}
	}
	for line, rule := range map[string]string{
		"setopt promptsubst; print -P '$(git push)'": textRule, "zargs -- origin -- git push": textRule,
		"PS4='$(git push)'; set -x; :": notPlainRule, "setopt globdots": notPlainRule, "rm -rf .c^1": fenceRule,
		"cat ~/Lib*/App*/A*/s*": "gm-token",
	} {
		if r := Check(line, DefaultGroups()); r == nil || r.Rule != rule {
			t.Errorf("%q: %v, want %s", line, r, rule)
		}
	}
}

// WIN125j: everyday work still runs, for a hire and the GM, with patterns read as their skeletons.
func TestEverydayPatternsStillRun(t *testing.T) {
	lines := []string{
		"[ -f x ] && cat x", "[[ -n $x ]] && echo \"$x\"", "grep '[0-9]' notes.md", "ls *.[ch]", "git log HEAD^1",
		"git diff main^..HEAD", "rm -rf build/*", "git log main^", "git log HEAD^..main", "git rebase -i main^",
		"git diff main^ -- src/", "git show v1.2.0^{commit}", "git log origin/main^2", "ls src/*.[ch]pp", "ls [!.]*",
		"ls [A-Z]*.md", "ls [[:upper:]]*", "rm -f build/*.[oa]", "find . -name '*.[ch]'", "git add src/[a-m]*.go",
		"ls .claude/[a-r]*", "cat setup-key.md", "cat .claude/settings.json", "cat setup-k[!e]y.md", "ls */", "du -sh */*",
		"ls ~/*", "ls src/*/*.go", "ls **/*.go", "cp -r src/* dist/", "tar czf out.tgz build/*", "for f in *.md; do echo $f; done",
		"grep -rn foo --include=*.go .", "rsync -a --exclude=*.log src/ dst/", "find . -name '*.go' -exec wc -l {} +",
		"echo \"[$(date)]\"", "n=$(( $(wc -l < f) + 1 ))", "x=$(date +%F)", "today=\"$(date +%F)\"", "cut -d, -f1,2 f.csv",
		"ps -o pid,comm", "git log --pretty=format:%h,%an", "make CC=gcc CFLAGS=-O2", "echo $PS1", "ls .claude/agents/*.md",
		"ls -d .*/", "cat Équipe/Réunion\\ du\\ 12.md", "ls Équipe/*.md", "mv présentation.md présentation-v2.md",
		"alias ll='ls -la'", "print -r -- x", "zstyle ':completion:*' menu select", "hash -r", "set -euo pipefail",
		"setopt extendedglob", "jq '.items[0].name' x.json", "echo ${arr[@]}", "echo ]] done", "cat window-keys.md",
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range lines {
			if r := Check(line, groups); r != nil {
				t.Errorf("refused %q: %v", line, r)
			}
		}
	}
}

// WIN125j: the skeleton's own reading, one word at a time.
func TestAPatternIsReadAsItsSkeleton(t *testing.T) {
	for _, c := range []struct {
		word string
		want string // the names, space-separated
	}{
		{"setup-k[!x-[:x:]y", "setup-key"}, {"setup-k[!]y", "setup-key"}, {"[[:alpha:]x]etup-ke[y]", "setup-key"},
		{"*key*", "setup-key push-key-kept window-key"}, {"s*", ""}, {"*", ""}, {"*.json", ""}, {"set*", "setup-key settings.local.json"},
		{".c*", ".claude .claude/settings.json .claude/settings.local.json"}, {"*", ""}, {"?claude", ""},
		{"[.]claude", ""}, {"(.|x)claude", ".claude"}, {".#claude", ".claude"}, {"build/*.[oa]", ""},
		{".c?aude/settings.json", ".claude/settings.json"}, {"*/settings.json", ""}, {".*/settings.json", ".claude/settings.json"},
		{"~/Lib*/App*/A*/s*", "gm-token gm-token-kept smolcorp_gm_token hire-guard.json hire-guard setup-key grants.jsonl phones-seal claude-install push-key-kept window-key settings.local.json"},
		{"x/AgentDeck/*.swift", ""}, {"setup-key~x", "setup-key"}, {"setup-kx#ey", "setup-key"}, {"{}", ""},
		{"{.c,x}laude", ".claude"}, {"=*.go", ""},
	} {
		sk := skeletonOf(c.word, nil)
		if got := strings.Join(skeletonNames(&sk, 3, nil), " "); got != c.want {
			t.Errorf("%q: %q, want %q", c.word, got, c.want)
		}
	}
	// A git revision: one written letter of a kept name is enough (zsh's x^y fills in names that start with x).
	for word, want := range map[string]bool{"main^": true, "HEAD^1": true, "v1.2.0^{commit}": true, "origin/main^2": true,
		"s^1": false, ".c^1": false, "gm^1": false, "hire-g^1": false, "se^1": false, ".cl^{tree}": false} {
		if got := gitRevisionWord(word, nil); got != want {
			t.Errorf("gitRevisionWord(%q): %v", word, got)
		}
	}
	// No word or line makes it slow: a long word goes by its letters, and a line has a budget.
	for name, line := range map[string]string{
		"long word":      "cat " + strings.Repeat("a*", 1<<19),
		"many patterns":  strings.Repeat("cat setup-k*y ", 1<<15),
		"many brackets":  strings.Repeat("ls [a-z][[:x:]]*x ", 1<<14),
		"long revisions": "git log " + strings.Repeat("main^ ", 1<<16),
	} {
		start := time.Now()
		r := Check(line, DefaultGroups())
		if took := time.Since(start); took > 5*time.Second*raceSlowdown {
			t.Errorf("%s: took %v", name, took)
		}
		if name == "many patterns" && r == nil {
			t.Errorf("%s: let through", name)
		}
	}
	if r := Check("echo "+strings.Repeat("{", 100000), DefaultGroups()); r != nil {
		t.Errorf("a long word of braces as an argument: %v", r)
	}
}
