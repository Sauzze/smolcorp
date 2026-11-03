package guard

import (
	"bytes"
	"strings"
	"testing"
)

// The GM's profile frees its own `smolcorp gm` actions and nothing else.
func TestGMProfileAllowsItsOwnActionsAndKeepsTheRest(t *testing.T) {
	gm := ForGM(DefaultGroups())
	for _, line := range []string{
		"smolcorp gm hire builder", "smolcorp gm fire x-builder", "smolcorp gm brief x", "smolcorp gm tile", "smolcorp gm watch x",
		"smolcorp gm unwatch x", "smolcorp gm screen 2", "smolcorp gm --test 7895 hire builder",
		"/Applications/Smolcorp.app/Contents/MacOS/smolcorp gm fire x",
	} {
		if r := Check(line, gm); r != nil {
			t.Errorf("the gm profile refused %q: %v", line, r)
		}
		if r := Check(line, DefaultGroups()); r == nil {
			t.Errorf("the hire profile allowed %q", line)
		}
	}
	for _, line := range []string{
		"git push", "vercel deploy", "npm publish", "gh pr create", "aws s3 ls", "ssh host",
		"tmux -L smolcorp-x send-keys -t y Enter", "osascript -e 'x'", "security dump-keychain", "pass show x",
		"sh -c 'git push'", `tmux ls \; send-keys -t x Enter`,
	} {
		if Check(line, gm) == nil {
			t.Errorf("the gm profile allowed %q", line)
		}
	}
}

func TestGMProfileOnTheCommandLine(t *testing.T) {
	call := `{"tool_name":"Bash","tool_input":{"command":"smolcorp gm hire builder"}}`
	run := func(args ...string) string {
		var out bytes.Buffer
		Run(args, strings.NewReader(call), &out, &out)
		return out.String()
	}
	if got := run("--profile", "gm"); got != "" {
		t.Errorf("gm profile: %q", got)
	}
	if got := run("--profile", "hire"); !strings.Contains(got, "deny") {
		t.Errorf("hire profile: %q", got)
	}
	if got := run(); !strings.Contains(got, "deny") {
		t.Errorf("no profile: %q", got)
	}
	if got := run("--profile", "boss"); !strings.Contains(got, "deny") {
		t.Errorf("unknown profile: %q", got)
	}
	if got := run("--profile"); !strings.Contains(got, "deny") {
		t.Errorf("profile with no name: %q", got)
	}
}

// A refusal under the GM's profile speaks to the GM; a hire's keeps its words.
func TestRefusalWordsFollowTheProfile(t *testing.T) {
	line := "git push origin main"
	hire := Check(line, DefaultGroups())
	gm := Check(line, ForGM(DefaultGroups()))
	if hire == nil || gm == nil {
		t.Fatalf("hire %v, gm %v", hire, gm)
	}
	if h := hire.Error(); !strings.Contains(h, "A hire doesn't do this; ask the GM, who asks the owner.") {
		t.Errorf("hire: %q", h)
	}
	g := gm.Error()
	if !strings.Contains(g, "The General Manager doesn't do this; ask the owner.") || strings.Contains(g, "ask the GM") || strings.Contains(g, "hire guard") {
		t.Errorf("gm: %q", g)
	}
	// The rules that are always on too.
	if g := Check("cat gm-token", ForGM(DefaultGroups())).Error(); strings.Contains(g, "ask the GM") {
		t.Errorf("gm always-on: %q", g)
	}
}
