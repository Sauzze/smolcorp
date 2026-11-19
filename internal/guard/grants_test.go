package guard

import "testing"

// RT126FS2 B1: neither the GM nor a hire may read, write or replace the owner's grant record (desktop/grants.go), the
// file it is written through, or its key's name, by any spelling: a yes it could write is a yes it could forge.
func TestTheGrantRecordIsProtectedForTheGMAndAHire(t *testing.T) {
	lines := []string{
		// The verdict's probes.
		"cat ~/Library/Application\\ Support/AgentDeck/grants-key",
		"printf '%s\\n' x >> ~/Library/Application\\ Support/AgentDeck/grants.jsonl",
		"cp /tmp/forged ~/Library/Application\\ Support/AgentDeck/grants.jsonl",
		// Other spellings: quoted, cased, by a pattern, the temp file, Linux's and Windows' folders, PowerShell.
		"cat \"$HOME/Library/Application Support/AgentDeck/GRANTS.JSONL\"",
		"cat ~/Library/Application\\ Support/AgentDeck/gr'ants'.jsonl",
		"cat ~/Library/Application\\ Support/AgentDeck/grants*",
		"mv x ~/.config/smolcorp/grants.jsonl.tmp",
		"Get-Content $env:APPDATA\\Smolcorp\\grants.jsonl",
		"cp ~/.config/smolcorp/grants.jsonl /tmp/copy && cp /tmp/copy ~/.config/smolcorp/grants.jsonl",
	}
	for name, groups := range map[string][]Group{"a hire": DefaultGroups(), "the GM": ForGM(DefaultGroups())} {
		for _, line := range lines {
			if Check(line, groups) == nil {
				t.Errorf("%s: let through: %s", name, line)
			}
		}
	}
	// Reading or writing grants.go, or the word grants alone, is no grant record.
	for _, line := range []string{"cat desktop/grants.go", "grep -n grants desktop/*.go", "go test -run Grant ./desktop/"} {
		if r := Check(line, DefaultGroups()); r != nil {
			t.Errorf("refused %q: %v", line, r)
		}
	}
}

// RT126FS2b N2: neither the GM nor a hire may name the paired phones a tap is trusted from, their seal's key, or the
// Windows window's key, by any spelling: a phone it could list, or a key it could read, would answer as the owner.
func TestThePairedPhonesAreProtectedForTheGMAndAHire(t *testing.T) {
	lines := []string{
		// The verdict's probe.
		"python3 -c 'import json; json.dump([], open(\"/home/u/.config/smolcorp/phones.json\", \"w\"))'",
		"cp /tmp/planted ~/Library/Application\\ Support/AgentDeck/phones.json",
		"cat ~/Library/Application\\ Support/AgentDeck/phones-seal",
		// Other spellings: quoted, cased, by a pattern, Windows' folder and PowerShell, the window's key.
		"cat \"$HOME/.config/smolcorp/PHONES.JSON\"",
		"cat ~/.config/smolcorp/pho'nes'-seal",
		"cat ~/.config/smolcorp/phones-s*",
		"Get-Content $env:APPDATA\\Smolcorp\\phones.json",
		"type %APPDATA%\\Smolcorp\\window-key",
	}
	for name, groups := range map[string][]Group{"a hire": DefaultGroups(), "the GM": ForGM(DefaultGroups())} {
		for _, line := range lines {
			if Check(line, groups) == nil {
				t.Errorf("%s: let through: %s", name, line)
			}
		}
	}
	// Reading the code that keeps them, or the word phones alone, is neither.
	for _, line := range []string{"cat desktop/access.go", "grep -n phones desktop/*.go", "go test -run Phone ./desktop/", "find . -name '*.json' -newer go.mod"} {
		if r := Check(line, DefaultGroups()); r != nil {
			t.Errorf("refused %q: %v", line, r)
		}
	}
}

// RMSY126k nit 1: neither the GM nor a hire may name which Claude Code the owner has (desktop/agentcheck.go's
// claude-install), by any spelling: "npm" written there would let an unsigned Claude Code start where a signed one
// ran. Only the refusal is checked: nothing is written.
func TestWhichClaudeCodeTheOwnerHasIsProtectedForTheGMAndAHire(t *testing.T) {
	lines := []string{
		"echo npm > ~/Library/Application\\ Support/AgentDeck/claude-install",
		"printf 'npm\\n' > \"$HOME/.config/smolcorp/claude-install\"",
		"rm ~/Library/Application\\ Support/AgentDeck/claude-install",
		// Other spellings: cased, quoted, by a pattern, Windows' folder and PowerShell, a script.
		"echo npm > ~/Library/Application\\ Support/AgentDeck/CLAUDE-INSTALL",
		"echo npm > ~/.config/smolcorp/cla'ude-ins'tall",
		"echo npm > ~/Library/Application\\ Support/AgentDeck/claude-?nstall",
		"cp /tmp/x ~/.config/smolcorp/claude-i*",
		"Set-Content $env:APPDATA\\Smolcorp\\claude-install npm",
		"python3 -c 'open(\"/home/u/.config/smolcorp/claude-install\", \"w\").write(\"npm\")'",
	}
	for name, groups := range map[string][]Group{"a hire": DefaultGroups(), "the GM": ForGM(DefaultGroups())} {
		for _, line := range lines {
			if Check(line, groups) == nil {
				t.Errorf("%s: let through: %s", name, line)
			}
		}
	}
	// Reading the code that keeps it, or Claude Code itself, is neither.
	for _, line := range []string{"cat desktop/agentcheck.go", "grep -n claudeInstalls desktop/*.go", "which claude",
		"ls ~/.claude*", "npm ls -g @anthropic-ai/claude-code"} {
		if r := Check(line, DefaultGroups()); r != nil {
			t.Errorf("refused %q: %v", line, r)
		}
	}
}

// MEM1: neither the GM nor a hire may name the GM's memory (desktop/memory.go's gm-memory), by any spelling or
// pattern: only the core writes it, and the GM reads it only through the core's routes. Only the refusal is checked:
// nothing is read or written.
func TestTheGMsMemoryIsProtectedForTheGMAndAHire(t *testing.T) {
	lines := []string{
		"cat ~/Library/Application\\ Support/AgentDeck/gm-memory/manifest.json",
		"cp /tmp/x.md ~/.config/smolcorp/gm-memory/",
		"ls \"$HOME/Library/Application Support/AgentDeck/gm-memory\"",
		"echo note >> ~/.config/smolcorp/gm-memory/0123.md",
		// Other spellings: cased, quoted, escaped, by a pattern, Windows' folder and PowerShell, a script.
		"cat ~/.config/smolcorp/GM-Memory/manifest.json",
		"cat ~/.config/smolcorp/gm'-mem'ory/manifest.json",
		"cat ~/.config/smolcorp/gm\\-memory/manifest.json",
		"cat ~/.config/smolcorp/gm-mem*/manifest.json",
		"cat ~/.config/smolcorp/gm-m?mory/manifest.json",
		"cat ~/.config/smolcorp/gm-[m]emory/manifest.json",
		"cat ~/Library/Application\\ Support/AgentDeck/g*mory/manifest.json",
		"cat ~/Library/Application\\ Support/AgentDeck/*/manifest.json",
		// Under the support folder, a pattern that fills in the folder's name, however few of its letters it writes
		// out, as for the other kept names there (RT126MEM1 B2).
		"cat ~/Library/Application\\ Support/AgentDeck/gm*/*.md",
		"ls ~/Library/Application\\ Support/AgentDeck/g*",
		"cat ~/.config/smolcorp/g*/0123.md",
		"cat ~/.config/smolcorp/*mem*/*.md",
		"cat ~/.config/smolcorp/?m-*/*.md",
		"cat ~/.config/smolcorp/*/*.md",
		"cat ~/Library/Application\\ Support/AgentDeck/*/0123.md",
		"cat ~/.config/smolcorp/{*,x/y}/*.md",
		"Get-Content $env:APPDATA\\Smolcorp\\g*\\*.md",
		// From inside the support folder, where no path before says so: three of its letters written out.
		"cat gm-mem*/0123.md",
		"cat g*mory/manifest.json",
		"ls gm-m?mory",
		"Get-Content $env:APPDATA\\Smolcorp\\gm-memory\\manifest.json",
		"python3 -c 'print(open(\"/home/u/.config/smolcorp/gm-memory/manifest.json\").read())'",
	}
	for name, groups := range map[string][]Group{"a hire": DefaultGroups(), "the GM": ForGM(DefaultGroups())} {
		for _, line := range lines {
			if Check(line, groups) == nil {
				t.Errorf("%s: let through: %s", name, line)
			}
		}
	}
	// The lexer hands on a brace's words one by one, so a / in a bracket or a group reaches skeletonParts only past
	// it: there the bracket or group stays whole in its part, where it may stand for the folder.
	for word, want := range map[string]int{"smolcorp/x/*.md": 3, "smolcorp/{*,x/y}/*.md": 3, "smolcorp/[x/y]*/*.md": 3, "smolcorp/{*/*.md": 2} {
		sk := skeletonOf(word, nil)
		if parts := skeletonParts(&sk); len(parts) != want {
			t.Errorf("%s: %d parts, not %d", word, len(parts), want)
		}
	}
	// Reading the code that keeps it, or the GM's own command, is neither.
	for _, line := range []string{"cat desktop/memory.go", "grep -n memoryFolder desktop/*.go", "go test -run Memory ./desktop/",
		"ls Sources/AgentDeck/*.swift", "cat docs/*/README.md", "ls web/*/manifest.json",
		"cat ~/Projects/smolcorp/docs/*.md", "ls ~/.config/smolcorp/*.md", "cat $HOME/Projects/smolcorp/desktop/*.go"} {
		if r := Check(line, DefaultGroups()); r != nil {
			t.Errorf("refused %q: %v", line, r)
		}
	}
	for _, line := range []string{"smolcorp gm memory", "smolcorp gm memory show tests"} {
		if r := Check(line, ForGM(DefaultGroups())); r != nil {
			t.Errorf("the GM refused %q: %v", line, r)
		}
	}
}
