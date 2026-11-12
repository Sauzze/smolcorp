package guard

import "testing"

// `smolcorp hire|say|open` (1.26 X3) are fenced next to `smolcorp gm …`: the command's own check for a hire is an
// environment variable, which a command line can empty or unset, so the guard is what refuses them.
func TestAHireCannotUseThePeoplesCommands(t *testing.T) {
	for _, line := range []string{
		"smolcorp hire builder --task x", "smolcorp say 3 hello", "smolcorp open 3", "smolcorp --port 7711 hire builder",
		"smolcorp-cli hire builder", "smolcorp-cli say ada hi", "smolcorp-cli --port 7711 open 3", "smolcorp -- hire builder",
		"/Applications/Smolcorp.app/Contents/MacOS/smolcorp hire builder", `C:\Tools\smolcorp-cli.exe hire builder`,
		"SMOLCORP HIRE builder",
		// The hire key emptied or unset first.
		"SMOLCORP_HIRE_KEY= smolcorp hire builder --task x", "SMOLCORP_HIRE_KEY= smolcorp say 3 hello", "SMOLCORP_HIRE_KEY= smolcorp open 3",
		"SMOLCORP_HIRE_KEY='' smolcorp-cli hire builder", "env SMOLCORP_HIRE_KEY= smolcorp hire builder",
		"env -u SMOLCORP_HIRE_KEY smolcorp hire builder", "env -u SMOLCORP_HIRE_KEY smolcorp say 3 hello",
		"env -u SMOLCORP_HIRE_KEY smolcorp-cli open 3", "env -i smolcorp hire builder", "sh -c 'smolcorp hire builder'",
		// smolcorp-cli's gm actions are fenced as smolcorp's are.
		"smolcorp-cli gm hire builder", "env -u SMOLCORP_HIRE_KEY smolcorp-cli gm fire x", "smolcorp-cli --port 7711 gm hire builder",
	} {
		if Check(line, DefaultGroups()) == nil {
			t.Errorf("a hire's %q was allowed", line)
		}
		// The GM is held to them too, except for its own `smolcorp gm` actions.
		gmFree := len(line) > 0 && (line == "smolcorp-cli gm hire builder" || line == "env -u SMOLCORP_HIRE_KEY smolcorp-cli gm fire x" || line == "smolcorp-cli --port 7711 gm hire builder")
		if r := Check(line, ForGM(DefaultGroups())); gmFree == (r != nil) {
			t.Errorf("the gm profile on %q: %v (its own gm actions free: %v)", line, r, gmFree)
		}
	}
	for _, line := range []string{
		"smolcorp status", "smolcorp desks --json", "smolcorp-cli status", "smolcorp gm team", "smolcorp gm send gm 'done'",
		"smolcorp-cli gm team", "smolcorp-cli --port 7711 gm inbox", "smolcorp guard --print", "smolcorp hook",
	} {
		if r := Check(line, DefaultGroups()); r != nil {
			t.Errorf("%q was refused: %v", line, r)
		}
	}
}
