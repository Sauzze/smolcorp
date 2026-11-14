package guard

import (
	"strings"
	"testing"
)

// WIN125l (RWIN10): every row of RWIN10's seven findings, for every managed hire and the GM: cmd's switches as Git
// Bash and cmd.exe spell them (B1), the moves into .claude by a variable, zsh's dirs, ${CDPATH:=} and PowerShell
// (B2), an assignment's value in every form (B3), zsh's $=x, quoted find and git patterns and --opt= values (B4),
// the variables that make a shell run text when it starts (B5), and find's -regex escapes (B6).
func TestRWIN10sRowsAreRefused(t *testing.T) {
	const dir = `~/Library/Application\ Support/AgentDeck/`
	rows := map[string][]string{
		"B1": {
			`cmd //c "git push"`, `cmd //c git push`, `cmd.exe //c "git push"`, `cmd //C "git push"`, `cmd //c "vercel deploy"`,
			`cmd //c ssh host`, `cmd //k "git push"`, `cmd //r "git push"`, `cmd //s //c "git push"`,
			`cmd //c "cd .claude\agents && del ..\settings.json"`, `cmd /cgit push`, `cmd /c"git push"`, `cmd /q/c git push`,
			`cmd //q//c git push`, `cmd /D /S /C"git push"`, `CMD.EXE //C git push`, `cmd /e:on /c git push`, `cmd x /c git push`,
			// For comparison, refused before.
			`cmd /c "git push"`, `cmd /s /c "git push"`, `cmd /d /c git push`, `wsl git push`, `wsl -e git push`,
		},
		"B2": {
			"x=.claude/agents; cd $x && rm ../settings.json", "x=.claude/agents; pushd $x && rm ../settings.json",
			"read x <<< .claude/agents; cd $x && rm ../settings.json", "dirs .claude/agents; popd; rm ../settings.json",
			"dirs .claude/agents; cd +1; rm ../settings.json", "dirs .claude/agents; cd -1; rm ../settings.json",
			"dirs $PWD/.claude/agents; popd; rm ../settings.json", "dirs $PWD/.claude/agents && cd +1 && rm ../settings.json",
			": ${CDPATH:=.claude/agents}; cd x && rm ../../settings.json", ": ${CDPATH::=$PWD/.claude/agents}; cd x",
			": ${CDPATH=.claude/agents}; cd x", `echo "${HOME:=x}"`, ": ${cdpath::=x}", ": ${OLDPWD:=.claude}; cd -",
			`powershell -c "Set-Location .claude/agents; Remove-Item ../settings.json"`, `powershell -c "sl .claude/agents; ri ../settings.json"`,
			`pwsh -c "Push-Location .claude/agents; Remove-Item ../settings.json"`, `pwsh -c "Set-Location -Path .claude/agents; ri ../settings.json"`,
			`pwsh -c "Set-Location -LiteralPath .claude/agents"`, `pwsh -c "Set-Location -lit .claude/agents"`, `pwsh -c "Set-Location -Path:.claude/agents"`,
			"Set-Location .claude/agents; Remove-Item ../settings.json", `pwsh -c "chdir .claude/agents"`, `cmd //c "cd /d .claude\agents && del ..\settings.json"`,
			// Fail closed: a cd-family target holding a $ or a backtick.
			"cd $x && rm ../settings.json", `cd "$x"`, "cd `printf .claude`/agents", "cd $(printf .claude)/agents", "pushd ${x}",
			`bash -c "cd $(printf .claude)/agents && rm ../settings.json"`, `pwsh -c "Set-Location $env:X"`, "cd ~/$x",
			"x=.claude/agents; git -C $x rm ../settings.json", "for d in .claude/agents; do git -C $d rm ../settings.json; done",
			// For comparison, refused before.
			"cd .cla${x}ude/agents", `cd .cla$=x""ude/agents`, `pwsh -c "cd .claude/agents; Remove-Item ../settings.json"`,
			`cmd /c "cd /d .claude\agents && del ..\settings.json"`, "declare -n r=CDPATH", "export -- HOME=.claude", "cdpath+=(.claude)",
		},
		"B3": {
			"a=('.c?aude/settings.json'); rm ${a[@]}", "a+=('.c?aude/settings.json'); rm ${a[@]}",
			"declare -a a=('.c?aude/settings.json'); rm ${a[@]}", "x+='.c?aude/settings.json'; rm $x",
			"read x <<< '.c?aude/settings.json'; rm $x", "IFS= read -r x <<< '.c?aude/settings.json'; rm $x",
			"mapfile -t a <<< '.c?aude/settings.json'; rm ${a[@]}", "printf -v x '.c?aude/settings.json'; rm $x",
			"for x in '.c?aude/settings.json'; do rm $x; done", "set -- '.c?aude/settings.json'; rm $1",
			"select x in '.c?aude/settings.json'; do rm $x; done", "set -A a '.c?aude/settings.json'; rm $a",
			"readarray -t a <<< '.c?aude/settings.json'; rm ${a[@]}", "a=([0]='.c?aude/settings.json'); rm ${a[@]}",
			"getopts a: o -a '.c?aude/settings.json'; rm $OPTARG", "print -v x '.c?aude/settings.json'; rm $x",
			"read x <<EOF\n.claude/agents\nEOF\ngit -C $x rm ../settings.json", "x=.claude/agents/reviewer.md; cat $x",
			"a=(.claude/agents); git -C ${a[1]} rm ../settings.json", "x+=.claude/agents; git -C $x rm ../settings.json",
			// For comparison, refused before.
			"x='.c?aude/settings.json'; rm $x", "declare x='.c?aude/settings.json'", "typeset x='.c?aude/settings.json'",
			"local x='.c?aude/settings.json'", "readonly x='.c?aude/settings.json'", "export x='.c?aude/settings.json'",
			"a[0]='.c?aude/settings.json'", "x+=.c?aude/settings.json", ": ${x:='.c?aude/settings.json'}",
		},
		"B4": {
			`rm .clau$=x""de/settings.json`, `rm .claude/s$=x""ettings.local.json`, `cat ` + dir + `setup-k$=x""ey`,
			`echo '{}' > ` + dir + `hire-gu$=x""ard.json`, `cat ` + dir + `gm-t$=x""oken`, `rm .clau$+x""de/settings.json`,
			`rm .clau$#x""de/settings.json`, `rm .clau"$=x"de/settings.json`,
			`find . -name "sett${x}ings.json" -delete`, `find . -name "s${x}*" -delete`, `find . -name "$(printf s)ettings.json" -delete`,
			`find . -name "set${x}tings.json" -exec cp /dev/null {} \;`, `git rm "*sett${x}ings.json"`, `git checkout -- "*set${x}tings.json"`,
			"find . -name \"sett`printf i`ngs.json\" -delete", `find . -path "*/.cl${x}aude/*" -delete`, `find . -name "sett$=x""ings.json" -delete`,
			"cp settings.json --target-directory=.clau${x}de", "cp settings.json --target-directory=.clau$(printf '')de",
			"cp settings.json --target-directory=.c?aude",
			// For comparison, refused before.
			"rm .clau${x}de/settings.json", `rm .clau$"d"e/settings.json`, "rm .clau$!de/settings.json", "rm .clau${:-}de/settings.json",
			"rm .clau$^xde/settings.json", "rm .clau$~xde/settings.json", "find . -name sett${x}ings.json -delete", "cp -t .clau${x}de settings.json",
		},
		"B5": {
			"BASH_ENV=<(echo git push) bash -c :", "env BASH_ENV=<(echo git push) bash -c :", "BASH_ENV=<(echo vercel deploy) bash -c :",
			"env 'BASH_FUNC_ls%%=() { git push; }' bash -c ls", "ENV=<(echo git push) sh -i", "ZDOTDIR=. zsh -c :", "export BASH_ENV=x.sh",
			"declare -x BASH_ENV=x", "export ENV=$(mktemp)", "env -i BASH_ENV=x bash -c :", "SHELLOPTS=xtrace bash -c :", "BASHOPTS=expand_aliases bash -c :",
			"NULLCMD=sh; <<< 'git push'", "READNULLCMD=sh; < x", "FPATH=. zsh -c :", "export 'BASH_FUNC_f()=() { git push; }'",
			": ${BASH_ENV:=x}", "read BASH_ENV <<< x", "printf -v ZDOTDIR .", "for ENV in x; do sh -i; done", "BASH_ENV+=x bash -c :",
			// For comparison, refused before.
			"bash --rcfile <(echo git push) -i", "compgen -C 'git push' x", "coproc git push", "env -S 'git push'", "repeat 1 git push",
			"for x (a) git push", "- git push", "() git push",
		},
		"B6": {
			`find . -regex '.*/\.\S\S\S\S\S\S/\S\S\S\S\S\S\S\S\.\S\S\S\S' -delete`, `find . -regex '.*/\.\w\w\w\w\w\w/\w\w\w\w\w\w\w\w\.\w\w\w\w' -delete`,
			`find . -regex '.*/\S\S\S\S\S\S\S\S\.\S\S\S\S\S\.\S\S\S\S' -delete`, `find . -iregex '.*/\.\W\W\W\W\W\W/.*' -delete`,
			`find . -regex '.*/\.[xc]laude/settings\.json' -delete`, `find . -regextype posix-extended -regex '.*/(\.claude|q)/settings\.json' -delete`,
			`find . -regex '.*/\(\.claude\|q\)/settings\.json' -delete`, `find . -regex '.*/\.claudeq?/settings\.json' -delete`,
			`find . -regex '.*/\.claudeq\{0\}/settings\.json' -delete`, `find . -regex '.*/\.claudeq+/settings\.json' -delete`,
			// For comparison, refused before.
			`find . -regex '.*/\.[a-z]laude/settings\.json' -delete`, `find . -regex '.*settingsq*\.json' -delete`,
			`find . -regex '.*/\.\w+/\w+\.local\.json' -delete`, `find . -name '[[:alpha:]]ettings.json' -delete`,
		},
		"B7-kept": {
			// B7 lets a negated test stand beside a name test that can't reach; these still reach.
			"find . -name '*.json' -not -path './node_modules/*' -delete", "find . -not -name '*.js' -delete",
			"find . -name '*.js' -o -not -name '*.js' -delete", "find . ! -path './node_modules/*' -delete",
			`find . \( -name '*.js' -o ! -name x \) -delete`, "find . -name 's*' ! -path './node_modules/*' -delete",
			"find . -type f ! -name '*.md' -exec rm {} +", `find . ! \( -name x -o -name y \) -delete`,
			// RWIN10 N2: git rm --cached touches only the index, but not a --cached after --, which is a path.
			"git rm -- --cached '*.json'",
		},
	}
	for name, lines := range rows {
		for _, line := range lines {
			for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
				if r := Check(line, groups); r == nil {
					t.Errorf("%s: let through %q", name, line)
				}
			}
		}
	}
}

// RWIN10's splice fuzz: each of 20 forms that expand to nothing, spliced at every place of the names the guard keeps
// (the fence, a hire's own settings.local.json, the setup key, the guard file, the GM token) and into cd .claude/agents.
// Every line is refused, for a hire and the GM.
func TestRWIN10sSpliceFuzzIsRefused(t *testing.T) {
	const dir = `~/Library/Application\ Support/AgentDeck/`
	forms := []string{`$x""`, "${x}", "$(:)", "`:`", "${x:-}", "$@", "$*", `"$x"`, `"${x}"`, "${x#y}", "${!x}", "${x[0]}", "$''",
		`$=x""`, `$+x""`, `$#x""`, "${x%y}", "${x/y/}", "$((0))", `"$(:)"`}
	targets := []struct{ before, name, after string }{
		{"rm ", ".claude/settings.json", ""},
		{"rm ", ".claude/settings.local.json", ""},
		{"cat " + dir, "setup-key", ""},
		{"echo '{}' > " + dir, "hire-guard.json", ""},
		{"cat " + dir, "gm-token", ""},
		{"cd ", ".claude/agents", " && rm ../settings.json"},
	}
	n := 0
	for _, form := range forms {
		for _, tg := range targets {
			for i := 1; i < len(tg.name); i++ {
				line := tg.before + tg.name[:i] + form + tg.name[i:] + tg.after
				n++
				for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
					if Check(line, groups) == nil {
						t.Errorf("let through: %s", line)
					}
				}
			}
		}
	}
	if n < 1500 {
		t.Errorf("only %d lines", n)
	}
}

// B7: the usual ways to leave node_modules out of a find that changes files run again, and the everyday lines of
// RWIN10's corpus with them.
func TestEverydayWorkStillRunsAfterRWIN10(t *testing.T) {
	runs := []string{
		"find . -name '*.js' -not -path './node_modules/*' -delete",
		"find . -name '*.ts' -not -path './node_modules/*' -exec prettier --write {} +",
		"find . -type f -name '*.py' ! -path './.venv/*' -exec black {} +",
		"find . -name '*.js' ! -path '*/node_modules/*' -delete", "find . -path ./node_modules -prune -o -name '*.js' -delete",
		"find . -path ./node_modules -prune -o -name '*.js' -print", "find . -name '*.log' -not -name 'keep.log' -delete",
		"find . -type f -name '*.pyc' -not -path './.git/*' -delete", "find . -name '*.orig' ! -path './vendor/*' -exec rm {} +",
		"find . -regex '.*\\.pyc' -delete", "find src -regex '.*/[a-z_]+\\.tmp' -delete",
		"pushd src && ls && popd", "make -C build test", "git -C ../other status", "export PATH=\"$HOME/.local/bin:$PATH\"",
		"find . -type d -name __pycache__ -exec rm -rf {} +", `find . \( -name '*.orig' -o -name '*.rej' \) -delete`,
		"mkdir -p .claude/agents/x", "cp templates/agent.md .claude/agents/new.md", "export PATH=$PATH:./bin", "export GOPATH=$HOME/go",
		"cmd /c dir", "cmd //c dir", "cmd /c npm test", "cmd //c \"npm run build\"", `powershell -c "Set-Location src; ls"`,
		`pwsh -c "Push-Location src; npm test; Pop-Location"`, "dirs", "dirs -v", "popd", "cd -", "cd ..", "cd ~", "cd src/app",
		"read -r line <<< \"$text\"", "IFS=, read -r a b <<< 'x,y'", "for f in src/*.go; do gofmt -l \"$f\"; done",
		"printf -v now '%s' \"$(date +%s)\"", "set -- a b c", "set -euo pipefail", "x+=1", "a=(one two three)", "a+=(four)",
		"mapfile -t lines < f.txt", "getopts ab: opt", "echo $#", "echo $#x", "echo ${#x}",
		"git rm \"$f\"", "git checkout -- \"$f\"", "find . -name \"$pattern\" -print", "cp a.txt --target-directory=out",
		"grep -rn BASH_ENV_X docs/ || true", "echo ENV", "env | grep PATH", "env FOO=1 npm test", "NODE_ENV=production npm run build",
		"git rm --cached '*.json'", "git rm -r --cached -- '.c*'", "ls -C .claude", "cat .claude/settings.json",
	}
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range runs {
			if r := Check(line, groups); r != nil {
				t.Errorf("refused %q: %v", line, r)
			}
		}
	}
}

// RWIN10 B2's fail-closed rule refuses these, which RWIN9's corpus ran (WIN125l's report lists them): a cd to a
// folder built at run time. The refusal says why.
func TestRWIN10RefusesACdBuiltAtRunTime(t *testing.T) {
	for _, line := range []string{`cd "$dir"`, "cd $(git rev-parse --show-toplevel)", `cd "$(dirname "$0")"`, "cd $TMPDIR", "pushd \"$OLDPWD\""} {
		r := Check(line, DefaultGroups())
		if r == nil || r.Why != builtDirWhy {
			t.Errorf("%s: %v", line, r)
		}
	}
	if r := Check("cd $(git rev-parse --show-toplevel)", DefaultGroups()); r == nil || !strings.Contains(r.Command, "cd $(…)") {
		t.Errorf("the refusal's command: %v", r)
	}
}

// The pieces: cmd's switches read, a find regex's escapes, and zsh's $= as a run.
func TestRWIN10sPieces(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"//c", "git", "push"}, "git push"}, {[]string{"/cgit push"}, "git push"}, {[]string{"/q/c", "git", "push"}, "git push"},
		{[]string{"/C", "dir"}, "dir"}, {[]string{"/s", "//k", "x"}, "x"}, {[]string{"/d"}, ""},
	} {
		got := ""
		if in := wrapped("cmd", c.args); len(in) > 0 {
			got = in[0].line
		}
		if got != c.want {
			t.Errorf("cmd %v: %q, want %q", c.args, got, c.want)
		}
	}
	for _, c := range []struct {
		regex string
		want  bool
	}{
		{`.*/\.\S\S\S\S\S\S/.*`, true}, {`.*\.pyc`, false}, {`.*/(a|b)\.go`, true}, {`.*/\.[xc]laude/.*`, true}, {`.*/x[a-z]+\.tmp`, false},
	} {
		if got := matchCouldReach(c.regex, true, true); got != c.want {
			t.Errorf("matchCouldReach(%q) = %v", c.regex, got)
		}
	}
	if got := dollarName("=x", 0, false); got != 2 {
		t.Errorf(`dollarName("=x") = %d`, got)
	}
	if got := dollarName("#", 0, false); got != 1 {
		t.Errorf(`dollarName("#") = %d`, got)
	}
	if !strings.Contains(markDollars("a$=x"), string(runOpen)) {
		t.Errorf("markDollars: %q", markDollars("a$=x"))
	}
}
