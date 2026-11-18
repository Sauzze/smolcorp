package guard

import "testing"

// smolcorp run starts the agent it's given as that agent would start (RX1b B2): `smolcorp run claude`
// is starting Claude Code again, and `run --role` may be. Both profiles, as for claude itself.
func TestSmolcorpRunIsJudgedAsTheAgentItStarts(t *testing.T) {
	for _, groups := range [][]Group{DefaultGroups(), ForGM(DefaultGroups())} {
		for _, line := range []string{
			"smolcorp run claude -p 'fix everything' --dangerously-skip-permissions",
			"cd /elsewhere && smolcorp run --role tester",
			"smolcorp run --role reviewer", "/usr/local/bin/smolcorp run claude", `C:\Tools\smolcorp-cli.exe run claude`, "SMOLCORP RUN claude",
			"smolcorp run npx @anthropic-ai/claude-code", "smolcorp run env FOO=1 claude", "nohup smolcorp run claude &", "sh -c 'smolcorp run claude'",
			"smolcorp run --hire abc123 --hire-id h1 --role x", "smolcorp run --hire abc123 --pinned claude",
		} {
			if r := Check(line, groups); r == nil || r.Group != "claude" || r.Rule != "starting Claude Code again" {
				t.Errorf("let through %q: %v", line, r)
			}
		}
		// What it starts meets every other rule too.
		if r := Check("smolcorp run vercel deploy --prod", groups); r == nil || r.Group != "deploy" {
			t.Errorf("smolcorp run vercel: %v", r)
		}
		for _, line := range []string{"smolcorp run codex", "smolcorp run", "smolcorp version", "echo smolcorp run claude", "smolcorp run --hire"} {
			if r := Check(line, groups); r != nil {
				t.Errorf("refused %q: %v", line, r)
			}
		}
	}
}
