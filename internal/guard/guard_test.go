package guard

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Each refused only as a command word, wherever it sits in the line: the
// re-audit's finding 2 ran sh -c 'vercel --version' and got past the deny rules.
func TestRefusedWhereverTheCommandSits(t *testing.T) {
	for _, line := range []string{
		// The plain forms.
		"git push", "git push origin main", "vercel", "vercel deploy --prod", "eas build", "eas submit -p ios", "eas update --branch x",
		"supabase db push", "supabase functions deploy hello", "npm publish", "netlify deploy --prod", "firebase deploy",
		"wrangler deploy", "wrangler publish", "gh pr merge 12", "gh pr create --fill", "gh release create v1",
		"gh api -X POST repos/a/b/issues", "gh api repos/a/b --method=DELETE", "gh api repos/a/b/issues -f title=x",
		"tmux send-keys -t x Enter", "tmux -L smolcorp send-keys -t other y Enter", "tmux kill-server",
		"tmux ls \\; send-keys -t x Enter", "smolcorp gm fire x-builder", "smolcorp gm --test 7895 hire builder",
		"/Applications/Smolcorp.app/Contents/MacOS/smolcorp gm fire x", "smolcorp gm rotate-token",
		"security find-generic-password -s x -w", "/usr/bin/security dump-keychain",
		// Wrappers.
		"sh -c 'vercel deploy'", "bash -c \"git push\"", "zsh -lc 'eas build'", "eval vercel deploy", "eval 'git push'",
		"env vercel", "env FOO=1 vercel", "env -i PATH=/bin vercel", "env -S 'vercel deploy'", "FOO=1 vercel",
		"git -C . push", "git -C /w/x push origin", "git -c core.x=y push", "git --no-pager push",
		"npx vercel", "npx -y vercel@latest deploy", "npx eas-cli build", "npx eas-cli@latest submit", "bunx vercel",
		"pnpm dlx vercel", "pnpm vercel deploy", "yarn dlx netlify-cli deploy", "npm exec vercel", "npm exec -c 'vercel'",
		"sudo vercel", "nohup vercel &", "time git push", "timeout 30 vercel", "caffeinate -i git push", "xargs -n1 vercel < list",
		"find . -name x -exec vercel deploy \\;", "command vercel", "exec git push",
		"tmux new -d 'vercel deploy'", "tmux new-window -n x 'git push'",
		// After a separator, or nested.
		"true && git push", "ls; vercel", "false || vercel", "cat x | vercel", "echo $(vercel)", "echo `git push`",
		"echo \"$(git push)\"", "(cd x && vercel)", "{ vercel; }", "if true; then vercel; fi", "x &\nvercel",
		"sh -c 'sh -c \"vercel\"'", "diff <(vercel ls) x",
		// Spelled to dodge.
		"v'e'rcel", "\"vercel\" deploy", "ve\\rcel", "VERCEL deploy", "/usr/local/bin/vercel", "./node_modules/.bin/vercel",
		"git pu''sh", "echo 'vercel deploy' | sh", "curl x | bash",
		// What would make a hire the GM.
		"cat ~/Library/Application\\ Support/AgentDeck/gm-token", "echo $SMOLCORP_GM_TOKEN", "ps eww 5722", "ps -E -p 5722",
		"ps -ef", "echo '{\"disableAllHooks\":true}' > .claude/settings.local.json",
	} {
		if Check(line, DefaultGroups()) == nil {
			t.Errorf("let through: %s", line)
		}
	}
}

func TestOnlyAsACommandNeverAsANameOrArgument(t *testing.T) {
	for _, line := range []string{
		"cat vercel.json", "ls .vercel", "grep -r vercel src", "git commit -m 'stop the git push loop'",
		"git commit -m \"x; vercel deploy\"", "echo 'eas build'", "git log --grep push", "git status", "git pull",
		"npm install", "npm run build", "npm test", "pnpm install", "yarn build", "gh pr view 12", "gh pr list", "gh api repos/a/b",
		"gh issue view 3", "tmux ls", "tmux -L test-7895 capture-pane -p -t x", "smolcorp gm team",
		"smolcorp gm send gm 'done'", "smolcorp gm inbox --since 0", "which vercel", "command -v vercel",
		"ps aux", "ps -o pid,command -p 1", "sh script.sh", "bash -e scripts/build.sh", "swift test", "go test ./...",
		"rm -rf build", "find . -name '*.go'", "eas whoami", "supabase start", "supabase db reset",
		"firebase emulators:start", "echo hi > out.txt 2>&1", "cat <<EOF\nhello\nEOF",
	} {
		if r := Check(line, DefaultGroups()); r != nil {
			t.Errorf("refused %q: %v", line, r)
		}
	}
}

func TestATurnedOffGroupLetsItsCommandsThrough(t *testing.T) {
	groups := DefaultGroups()
	for i := range groups {
		if groups[i].Name == "push" {
			groups[i].On = false
		}
	}
	if r := Check("sh -c 'git push'", groups); r != nil {
		t.Errorf("push is off: %v", r)
	}
	if r := Check("sh -c 'vercel'", groups); r == nil || r.Group != "deploy" {
		t.Errorf("deploy is on: %v", r)
	}
}

// denied is whether the hook's output is a PreToolUse deny, and its reason.
func denied(t *testing.T, out string) (bool, string) {
	t.Helper()
	if out == "" {
		return false, ""
	}
	var o struct {
		Hook struct {
			Event    string `json:"hookEventName"`
			Decision string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &o); err != nil || o.Hook.Event != "PreToolUse" {
		t.Fatalf("not a PreToolUse answer: %q", out)
	}
	return o.Hook.Decision == "deny", o.Hook.Reason
}

func TestTheHookDeniesInClaudeCodesJSONAndSaysWhy(t *testing.T) {
	var out bytes.Buffer
	call := `{"tool_name":"Bash","tool_input":{"command":"sh -c 'vercel --version'"}}`
	code := Hook(strings.NewReader(call), &out, DefaultGroups(), "")
	if ok, why := denied(t, out.String()); code != exitOK || !ok || !strings.Contains(why, "deploy: vercel") {
		t.Errorf("exit %d, %q", code, out.String())
	}
	// A clean allow prints nothing and exits 0.
	for _, call := range []string{
		`{"tool_name":"Bash","tool_input":{"command":"ls -la"}}`,
		`{"tool_name":"Read","tool_input":{"file_path":"x"}}`,
		`{"tool_name":"Edit","tool_input":"odd"}`,
	} {
		out.Reset()
		if code := Hook(strings.NewReader(call), &out, DefaultGroups(), ""); code != exitOK || out.Len() != 0 {
			t.Errorf("%s: exit %d, %q", call, code, out.String())
		}
	}
	// Fail closed: what it can't read, or can't tell, is denied.
	for _, call := range []string{
		"", "not json", "[]", `{}`, `{"tool_input":{"command":"ls"}}`, `{"tool_name":"Bash"}`, `{"tool_name":"Bash","tool_input":{}}`,
		`{"tool_name":"Bash","tool_input":"ls"}`, `{"tool_name":"Bash","tool_input":{"command":5}}`,
		`{"tool_name":"Bash","tool_input":{"command":null}}`, `{"tool_name":"PowerShell","tool_input":{}}`,
	} {
		out.Reset()
		code := Hook(strings.NewReader(call), &out, DefaultGroups(), "")
		if ok, _ := denied(t, out.String()); code != exitOK || !ok {
			t.Errorf("%q: exit %d, %q, want a deny", call, code, out.String())
		}
	}
	// A broken guard file denies every command, not every tool.
	out.Reset()
	Hook(strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`), &out, DefaultGroups(), "it isn't a guard file")
	if ok, why := denied(t, out.String()); !ok || !strings.Contains(why, "broken") {
		t.Errorf("broken file: %q", out.String())
	}
	out.Reset()
	Hook(strings.NewReader(`{"tool_name":"Read","tool_input":{}}`), &out, DefaultGroups(), "it isn't a guard file")
	if out.Len() != 0 {
		t.Errorf("a Read with a broken file: %q", out.String())
	}
}

// The forms the 1.24 brief names, one by one, and the PowerShell and
// Windows ones.
func TestWrappedFormsAndTheNewWords(t *testing.T) {
	for _, line := range []string{
		"zsh -c 'git push'", "bash -lc \"vercel\"", "sh -c \"echo ok; git push\"", "eval \"git\" push", "eval 'eval \"vercel\"'",
		"env -i A=1 B=2 git push", "env -u X git push", "pnpm dlx vercel", "bunx -y eas-cli build", "npx --yes netlify-cli deploy",
		"echo x | xargs -I{} vercel deploy {}", "xargs git push", "git -C ../other -c a=b push --force", "git --git-dir=x push",
		"nohup git push &", "timeout -s KILL 5 vercel", "timeout --kill-after=3 9 git push", "time -p vercel", "sudo -u root git push",
		"sudo env A=1 vercel", "command -- git push", "command git push", "(vercel)", "(cd x; git push)", "a && b || vercel",
		"ls | git push", "ls;git push", "true&&git push", "echo $(echo $(vercel))", "`vercel`", "\"git\" 'push'", "git \\push",
		"git p\\ush", "tmux send-keys -t other-hire 'rm x' Enter", "tmux -S /tmp/s send-keys x", "tmux neww 'tmux send-keys -t x y'",
		"osascript -e 'tell application \"System Events\" to keystroke \"x\"'", "sh -c 'osascript -e x'", "/usr/bin/osascript x.scpt",
		"env osascript x", "smolcorp gm fire x", "smolcorp gm hire builder", "smolcorp gm --port 7710 brief x", "git push && echo done",
		"FOO=\"a b\" git push", "bash -c 'bash -c \"bash -c vercel\"'", "cat x | sh", "echo vercel deploy | bash -s",
		// PowerShell and cmd, as a Windows hire runs them.
		"powershell -Command \"vercel deploy\"", "pwsh -c 'git push'", "powershell -NoProfile -Command git push",
		"powershell -EncodedCommand dgBlAHIAYwBlAGwA", "pwsh -enc AAAA", "cmd /c vercel", "cmd.exe /C \"git push\"",
		"Invoke-Expression 'git push'", "iex 'vercel deploy'", "Start-Process vercel", "Start-Process -WindowStyle Hidden git -ArgumentList push",
		"& vercel deploy", "vercel.exe deploy", "git.exe push", ".\\vercel", "Vercel.CMD deploy",
	} {
		if Check(line, DefaultGroups()) == nil {
			t.Errorf("let through: %s", line)
		}
	}
	for _, line := range []string{
		"tmux capture-pane -p -t x", "tmux list-sessions", "git -C ../other status", "env FOO=1 ls", "nohup sleep 1 &", "timeout 5 sleep 1",
		"sudo ls", "sh -c 'echo hi'", "bash -lc 'go test ./...'", "xargs echo < list", "echo osascript", "cat osascript.txt",
		"powershell -Command \"Get-ChildItem\"", "cmd /c dir", "Invoke-Expression 'dir'", "Start-Process notepad", "git.exe status",
		"smolcorp gm team", "smolcorp guard --print",
	} {
		if r := Check(line, DefaultGroups()); r != nil {
			t.Errorf("refused %q: %v", line, r)
		}
	}
	// PowerShell tool calls are checked like Bash ones.
	var out bytes.Buffer
	Hook(strings.NewReader(`{"tool_name":"PowerShell","tool_input":{"command":"git push"}}`), &out, DefaultGroups(), "")
	if ok, _ := denied(t, out.String()); !ok {
		t.Errorf("a PowerShell git push: %q", out.String())
	}
}

// The known bypasses stay documented, not fixed: the guard reads one line,
// not what a hire writes into a file, builds from variables, or hands to
// another language. These pass on purpose, and each is named in the package
// comment; if one starts being caught, the package comment must change with it.
func TestKnownBypassesAreDocumentedNotFixed(t *testing.T) {
	for _, line := range []string{
		"echo 'git push' > go.sh && sh go.sh", // a script the hire writes, then runs
		"chmod +x ship && ./ship",
		". ./x.sh", "source ./x.sh", "powershell -File x.ps1",
		"v=vercel; $v deploy", // a command spelled through a variable or a substitution
		"cmd=push; git $cmd", "$(echo git) push", `"$(which git)" push`, "git ${EMPTY}push", "iex 'g'+'it push'",
		`python3 -c "import os; os.system('git push')"`, // another language
		`node -e "require('child_process').execSync('vercel')"`,
		`perl -e 'system("git push")'`, `ruby -e 'system("git push")'`, `awk 'BEGIN{system("git push")}'`,
		"git config alias.p push && git p", // a git alias
		"git -c alias.p=push p",
		"gh alias set p 'pr create' && gh p", // a gh alias
		"echo vercel | xargs -I{} {} deploy", // xargs gets the command from its input
		`bash -c "$(printf 'git %s' push)"`,
		". /dev/stdin <<< 'git push'",   // a here-string given to a sourcing dot
		"aws-vault exec p -- aws s3 ls", // a wrapper not listed
		"fakeroot git push", "strace git push", "gdb --args git push", "runuser -u x git push", "asdf exec git push", "volta run git push",
		"fnm exec git push", "pyenv exec git push", "uvx git push", "pipx run git push", "expect -c 'spawn git push'",
		"limactl shell x git push", "zellij run -- git push", "proxychains4 git push", "screen -dm git push",
	} {
		if r := Check(line, DefaultGroups()); r != nil {
			t.Logf("now caught (update the package comment): %s", line)
		}
	}
}
