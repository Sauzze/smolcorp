package guard

import (
	"strings"
	"testing"
)

// A nested claude gets none of the --settings a hire was started with, so
// starting one is refused in both profiles, through every wrapper.
func TestStartingClaudeAgainIsRefusedInBothProfiles(t *testing.T) {
	lines := []string{
		"claude", "claude -p 'hi'", "claude --dangerously-skip-permissions", "/usr/local/bin/claude -p x", "./claude", "~/.claude/local/claude",
		"CLAUDE -p x", "claude.exe -p x", `C:\Users\me\AppData\Roaming\npm\claude.cmd`,
		"env claude -p x", "env FOO=1 claude", "sudo claude", "nohup claude -p x &", "timeout 60 claude -p x", "command claude", "exec claude -p x",
		"sh -c 'claude -p x'", "bash -lc \"claude -p x\"", "zsh -c 'cd /tmp && claude'", "xargs -n1 claude < list", "time claude -p x",
		"true && claude -p x", "ls; claude", "echo hi | claude -p", "echo $(claude -p x)", "(cd x && claude)", "x &\nclaude",
		"sh -c 'sh -c \"claude\"'", "cl''aude -p x", "\"claude\" -p x", "cl\\aude",
		"npx claude", "npx @anthropic-ai/claude-code", "npx -y @anthropic-ai/claude-code@latest -p x", "npx --package=@anthropic-ai/claude-code claude",
		"npx -p @anthropic-ai/claude-code claude", "pnpx @anthropic-ai/claude-code", "bunx @anthropic-ai/claude-code", "bunx claude",
		"pnpm dlx @anthropic-ai/claude-code", "yarn dlx @anthropic-ai/claude-code", "npm exec @anthropic-ai/claude-code", "npm exec -- claude",
		"node /usr/lib/node_modules/@anthropic-ai/claude-code/cli.js", "node ./node_modules/@anthropic-ai/claude-code/cli.js -p x",
		"node ./node_modules/.bin/claude", "node /opt/claude-code/cli.mjs", "env node ../claude-code/cli.js",
	}
	for _, profile := range []struct {
		name   string
		groups []Group
	}{{"hire", DefaultGroups()}, {"gm", ForGM(DefaultGroups())}} {
		for _, line := range lines {
			r := Check(line, profile.groups)
			if r == nil || r.Group != "claude" {
				t.Errorf("%s profile let through %q: %v", profile.name, line, r)
			}
		}
	}
}

// A here document fed to a shell is refused as a shell reading a pipe, which also covers claude in it.
func TestClaudeInAHereDocumentIsRefused(t *testing.T) {
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		if Check("bash <<EOF\nclaude -p x\nEOF", groups) == nil {
			t.Error("let through")
		}
	}
}

func TestClaudeAsAnArgumentOrANameStaysAllowed(t *testing.T) {
	for _, profile := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range []string{
			"git log --grep claude", "cat CLAUDE.md", "ls .claude", "ls -la ~/.claude/", "grep -r claude src", "echo claude",
			"which claude", "command -v claude", "type claude", "git commit -m 'ask claude'", "git add .claude/settings.json CLAUDE.md",
			"cat .claude/settings.json", "node build.js claude", "node script.js --name claude", "npm install @anthropic-ai/sdk",
			"npm install -g @anthropic-ai/claude-code", "npm ls claude-code", "cd claude-code && ls", "ls node_modules/@anthropic-ai/claude-code",
			"smolcorp gm team", "sed -i s/claude/x/ notes.txt", "pnpm run build", "brew info claude",
		} {
			if r := Check(line, profile); r != nil {
				t.Errorf("refused %q: %v", line, r)
			}
		}
	}
}

// It is not a group: nothing in the guard file switches it off, and it does not show in the groups.
func TestStartingClaudeStaysRefusedWhateverTheGroupsSay(t *testing.T) {
	if r := Check("claude -p x", nil); r == nil {
		t.Error("no groups: let through")
	}
	groups := DefaultGroups()
	for i := range groups {
		groups[i].On = false
	}
	if r := Check("sh -c 'claude'", groups); r == nil {
		t.Error("every group off: let through")
	}
	for _, g := range DefaultGroups() {
		if strings.Contains(strings.ToLower(g.Name), "claude") {
			t.Errorf("a group named %q", g.Name)
		}
	}
}

func TestNestedClaudeRefusalWords(t *testing.T) {
	hire := Check("claude -p x", DefaultGroups()).Error()
	if !strings.Contains(hire, "A hire doesn't start another Claude Code") || !strings.Contains(hire, "fence") {
		t.Errorf("hire: %q", hire)
	}
	gm := Check("claude -p x", ForGM(DefaultGroups())).Error()
	if !strings.Contains(gm, "hires through Smolcorp") || strings.Contains(gm, "ask the GM") {
		t.Errorf("gm: %q", gm)
	}
}
