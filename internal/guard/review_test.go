package guard

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// The attempts the first review got past the guard, one table per idea.
// Each must be refused now; the "still runs" tables are the look-alikes
// that must not be.

func mustRefuse(t *testing.T, idea string, lines []string) {
	t.Helper()
	for _, line := range lines {
		if Check(line, DefaultGroups()) == nil {
			t.Errorf("%s: let through: %s", idea, line)
		}
	}
}

func mustRun(t *testing.T, idea string, lines []string) {
	t.Helper()
	for _, line := range lines {
		if r := Check(line, DefaultGroups()); r != nil {
			t.Errorf("%s: refused %q: %v", idea, line, r)
		}
	}
}

// 1: a tool that isn't Bash but has a command (Monitor runs a shell command).
func TestAnyToolWithACommandIsChecked(t *testing.T) {
	var out bytes.Buffer
	for _, call := range []string{
		`{"tool_name":"Monitor","tool_input":{"command":"git push"}}`,
		`{"tool_name":"SomethingNew","tool_input":{"command":"sh -c 'vercel'"}}`,
		`{"tool_name":"Monitor","tool_input":{"command":5}}`,
		`{"tool_name":"Monitor","tool_input":{"command":null}}`,
	} {
		out.Reset()
		code := Hook(strings.NewReader(call), &out, DefaultGroups(), "")
		if ok, _ := denied(t, out.String()); code != exitOK || !ok {
			t.Errorf("%s: exit %d, %q, want a deny", call, code, out.String())
		}
	}
	for _, call := range []string{
		`{"tool_name":"Monitor","tool_input":{"command":"tail -f log"}}`,
		`{"tool_name":"Grep","tool_input":{"pattern":"git push"}}`,
		`{"tool_name":"Read","tool_input":"git push"}`,
	} {
		out.Reset()
		if code := Hook(strings.NewReader(call), &out, DefaultGroups(), ""); code != exitOK || out.Len() != 0 {
			t.Errorf("%s: exit %d, %q, want an allow", call, code, out.String())
		}
	}
}

// 2: tmux takes the start of a command's name, and other ways to type.
func TestTmuxCommandsSpelledShort(t *testing.T) {
	mustRefuse(t, "tmux", []string{
		"tmux send-k -t other x Enter", "tmux send-ke -t other x", "tmux kill-ses -t other", "tmux kill-serv",
		"tmux paste-buffer -t other", "tmux pasteb -t other", `tmux set-buffer x \; paste-buffer -t other`,
		"tmux source-file x", "tmux source x", "tmux send-p", "tmux -L a kill-wi", "tmux pipe-p -t x 'cat > f'",
		"tmux if-sh true 'send-keys x'", "tmux command-prompt",
	})
	mustRun(t, "tmux", []string{
		"tmux ls", "tmux list-s", "tmux capture-p -p -t x", "tmux new -d -s box", "tmux attach -t box", "tmux set-buffer x",
	})
}

// 3: quoting the lexer didn't understand, braces and patterns.
func TestQuotingBracesAndPatterns(t *testing.T) {
	mustRefuse(t, "quoting", []string{
		`$'\x67it' push`, `$'g\151t' push`, `$'\u0067it' push`, `$"git" push`, `$'git' push`, `$'\147\151\164' push`,
		"{git,push}", "git {push,x}", "git p{ush,ull}", "{git,push,x}", "git {push,}", "git {push,}{,}",
		"/usr/bin/g?t push", "/usr/bin/g[i]t push", "/usr/bin/gi* push", "/usr/*/git push",
		"sh -c '/usr/bin/g?t push'", "{a..z}{a..z}{a..z} x", `FOO=1 /usr/bin/g?t push`,
	})
	mustRun(t, "quoting", []string{
		`echo $'a\tb'`, `echo $"hello"`, "echo {a,b}", "echo {1..5}", "ls *.go", "ls src/*.ts", "cat <<EOF\n* one\n** two\nEOF",
		"[ -f x ] && echo y", "[[ -f x ]] && echo y", "git commit -m '{git,push}'", `git commit -m "g?t push"`, "echo '/usr/bin/g?t'",
		"for f in *.go; do echo $f; done", "git add -- '*.go'",
		"g*t push", "env g?t push", // a name with no / is looked up on the PATH, never globbed
		"git commit -m \"$(cat <<'EOF'\nfix: the thing\n\n* one bullet\n- another (here)\nEOF\n)\"",
	})
}

// 4: elif runs its words like then and else.
func TestElifRunsItsWords(t *testing.T) {
	mustRefuse(t, "elif", []string{"if false; then :; elif git push; then :; fi", "if x; then y; elif vercel; then z; fi"})
	mustRun(t, "elif", []string{"if false; then :; elif true; then ls; fi"})
}

// 5: env -S, attached, combined or long.
func TestEnvSplitString(t *testing.T) {
	mustRefuse(t, "env -S", []string{
		"env -S'git push'", "env -iS 'git push'", "env --split-string='git push'", "env --split-string 'git push'",
		"env -S 'git' push", "env -i -S'vercel deploy'", "env -uSOME git push",
	})
	mustRun(t, "env -S", []string{"env -S 'echo hi'", "env -i ls", "env --split-string='ls -la'"})
}

// 6: an alias set on the line.
func TestAliasSetOnTheLine(t *testing.T) {
	mustRefuse(t, "alias", []string{"alias gp='git push'\ngp", "alias gp='git push'", "alias git='vercel'", "alias a=b c='eas build'"})
	mustRun(t, "alias", []string{"alias ll='ls -la'", "alias"})
}

// 7: PowerShell options shortened, and a value attached.
func TestPowerShellShortOptions(t *testing.T) {
	mustRefuse(t, "powershell", []string{
		`powershell -com "git push"`, `powershell -Comm "git push"`, `pwsh -co 'git push'`, `powershell -c:"git push"`,
		`powershell -Command:"git push"`, "powershell -en abc", "powershell -EncodedCom abc", "pwsh -ec abc",
		"powershell -NoProfile -encodedcomm AAAA", "powershell -e abc",
	})
	mustRun(t, "powershell", []string{
		`powershell -com "Get-Date"`, "powershell -NoProfile -NoLogo -Command Get-Date", "powershell -ExecutionPolicy Bypass -Command dir",
	})
}

// 8: PowerShell script blocks with no space inside the braces, and iex on a pipe.
func TestPowerShellScriptBlocks(t *testing.T) {
	mustRefuse(t, "script blocks", []string{
		"pwsh -Command {git push}", `powershell -Command "& {git push}"`, "Invoke-Command {git push}", "icm {git push}",
		"1|%{git push}", "1|ForEach-Object{vercel deploy}", "& {git push}", "if ($true) {git push}", "echo hi | iex", "Get-Content x | Invoke-Expression",
		"cmd /c {git push}",
	})
	mustRun(t, "script blocks", []string{"pwsh -Command {Get-Date}", "1|%{echo $_}", "Invoke-Command {dir}", "iex 'dir'"})
}

// 9: Start-Process arguments in one string or a comma list.
func TestStartProcessArguments(t *testing.T) {
	mustRefuse(t, "Start-Process", []string{
		"Start-Process -FilePath git -ArgumentList 'push origin'", "Start-Process git -ArgumentList push,origin",
		`Start-Process git -ArgumentList "push","origin"`, "Start-Process -FilePath npm -ArgumentList 'publish'", "saps git -ArgumentList push",
		"start git 'push origin'",
	})
	mustRun(t, "Start-Process", []string{"Start-Process git -ArgumentList 'status'", "Start-Process notepad -ArgumentList a,b"})
}

// 10: Windows paths with backslashes, quoted or not. An unquoted path in a
// Bash line is a known bypass (see TestKnownBypassesAreDocumentedNotFixed).
func TestWindowsPaths(t *testing.T) {
	mustRefuse(t, "Windows paths", []string{
		`& 'C:\Program Files\Git\bin\git.exe' push`, `& "C:\Program Files\Git\bin\git.exe" push`,
		`& "C:\Program Files\nodejs\npm.cmd" publish`, `"C:\Program Files\Git\bin\git.exe" push`, `'C:\tools\vercel.exe' deploy`,
		`C:\tools\vercel.exe deploy`, `.\git push`, `.\node_modules\.bin\vercel.cmd`,
		"powershell -Command \"C:\\tools\\vercel.exe deploy\"", `cmd /c C:\tools\vercel.exe deploy`, "git`.exe push",
		`$env:Path; C:\x\git.exe push`, `"C:\x\az.cmd" login`,
	})
	mustRun(t, "Windows paths", []string{
		`& 'C:\Program Files\Git\bin\git.exe' status`, `type C:\tools\vercel.json`, `dir C:\Users\x`, `echo "C:\Users\a b"`,
	})
}

// 11: options with a value before the subcommand.
func TestOptionsWithAValueBeforeTheSubcommand(t *testing.T) {
	mustRefuse(t, "value options", []string{
		"npm --otp 123456 publish", "npm --tag beta publish", "npm --access public publish", "npm --loglevel silent publish",
		"npm --cache /tmp/c publish", "pnpm --reporter silent publish", "kubectl --token t apply -f x", "kubectl --as bob delete pod x",
		"kubectl --username u apply -f x", "docker --tlscacert ca push img", "git --super-prefix x push", "git --attr-source main push",
		"cargo +nightly publish", "cargo +stable publish --dry-run", "npm --scope @a publish", "kubectl --request-timeout 5 apply -f x",
	})
	mustRun(t, "value options", []string{"npm --loglevel silent install", "cargo +nightly build", "git --attr-source main status", "kubectl --as bob get pods"})
}

// 12: the short names of the same programs.
func TestShortNamesOfTheSameCLIs(t *testing.T) {
	mustRefuse(t, "aliases", []string{"vc deploy", "vc", "ntl deploy", "npx vc", "/usr/local/bin/vc --prod", "vc.cmd", "ntl.exe deploy"})
}

// 13: programs that run their arguments.
func TestMoreWrappers(t *testing.T) {
	mustRefuse(t, "wrappers", []string{
		"setsid git push", "ionice git push", "ionice -c 3 git push", "taskset 1 git push", "taskset -c 0-3 git push", "flock /tmp/l git push",
		"flock -n /tmp/l git push", "flock /tmp/l -c 'git push'", "chroot / git push", "unshare git push", "parallel git push",
		"parallel -j 4 git push", "parallel echo ::: 'git push'", "busybox git push", `busybox sh -c "git push"`, "wsl git push",
		"wsl.exe -e git push", "wsl --exec git push", "wsl -d Ubuntu git push", "wsl bash -c 'git push'", `trap "git push" EXIT`,
		"trap 'vercel deploy' EXIT INT", "coproc git push", "chronic git push",
	})
	mustRun(t, "wrappers", []string{
		"setsid sleep 1", "ionice -c 3 nice ls", "taskset 1 ls", "flock /tmp/l ls", "chroot / ls", "parallel echo ::: a b", "wsl ls",
		"trap 'rm -f x' EXIT", "busybox ls",
	})
}

// 14: switching the guard off from a command.
func TestTheGuardFileCantBeEdited(t *testing.T) {
	mustRefuse(t, "guard file", []string{
		`echo '{}' > ~/Library/Application\ Support/AgentDeck/hire-guard.json`,
		`sed -i s/true/false/ ~/.config/sauzzedeck-pro/hire-guard.json`, "rm hire-guard.json",
		`sh -c 'echo x >> "$HOME/.config/sauzzedeck-pro/hire-guard.json"'`,
	})
	mustRun(t, "guard file", []string{"cat hire-guard.json", "grep -n hire-guard README.md"})
}

// 15: a group without "on" is a problem; a rule's capital letters count.
func TestGroupWithoutOnIsAProblemAndRulesIgnoreCase(t *testing.T) {
	for _, text := range []string{
		`{"groups":[{"name":"x","commands":["git push"]}]}`,
		`{"groups":[{"name":"x","on":true,"commands":["git push"]},{"name":"y","commands":["ls"]}]}`,
		`{"groups":[{"name":"x","on":true,"commands":["git push"]}]} garbage`,
		`{"groups":[{"name":"x","on":true,"commands":["git push"]}]}{"groups":[]}`,
	} {
		if groups, problem := Parse([]byte(text)); problem == "" {
			t.Errorf("accepted %s as %v", text, groups)
		}
	}
	if _, problem := Parse([]byte(`{"groups":[{"name":"x","on":false,"commands":["git push"]}]}` + "\n")); problem != "" {
		t.Errorf("an explicit false is fine: %s", problem)
	}
	_, problem := Parse([]byte(`{"groups":[{"name":"x","commands":["git push"]}]}`))
	if !strings.Contains(problem, `"on"`) {
		t.Errorf("the problem should name on: %q", problem)
	}
	groups := []Group{{Name: "mine", On: true, Commands: []Rule{"Git Push", "Make Deploy", "GH api -X"}}}
	for _, line := range []string{"git push", "GIT PUSH origin", "make deploy", "gh api -X POST x"} {
		if Check(line, groups) == nil {
			t.Errorf("a capitalised rule missed %q", line)
		}
	}
}

// 16: a deny that can't be written is not an allow.
type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestADenyThatCantBeWrittenBlocks(t *testing.T) {
	call := `{"tool_name":"Bash","tool_input":{"command":"git push"}}`
	if code := Hook(strings.NewReader(call), brokenWriter{}, DefaultGroups(), ""); code != exitUsage {
		t.Errorf("exit %d, want %d (block)", code, exitUsage)
	}
	if code := Hook(strings.NewReader("garbage"), brokenWriter{}, DefaultGroups(), ""); code != exitUsage {
		t.Errorf("unreadable call: exit %d, want %d", code, exitUsage)
	}
}

// 17: more programs in the groups.
func TestMoreRulesInTheDefaultGroups(t *testing.T) {
	mustRefuse(t, "rules", []string{
		"bun publish", "python -m twine upload dist/*", "python3 -m twine upload x", "py -m twine upload x", "python3.11 -m twine upload x",
		"gsutil cp x gs://b", "bq query x", "cdk deploy", "git send-pack host x", "git subtree push --prefix=a b c", "git svn dcommit",
		"git lfs push origin main", "git -C x lfs push o",
	})
	mustRun(t, "rules", []string{
		"bun install", "python -m pytest", "python3 -m http.server", "git subtree add x", "git lfs pull", "git lfs ls-files",
	})
}
