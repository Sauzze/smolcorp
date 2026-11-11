package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// The accounts are the default, then the saved ones the Mac would keep: well
// formed, still folders, no name or folder twice.
func TestClaudeAccountsAreTheMacs(t *testing.T) {
	home := t.TempDir()
	for _, dir := range []string{".claude", ".claude-work", ".claude-test"} {
		os.MkdirAll(filepath.Join(home, dir), 0o700)
	}
	saved := `[{"name":"work","dir":"~/.claude-work"},{"name":"bad name","dir":"~/.claude-test"},{"name":"gone","dir":"~/.claude-gone"},
		{"name":"again","dir":"~/.claude-work"},{"name":"default","dir":"~/.claude-test"},{"name":"me","dir":"relative"},
		{"name":"home","dir":"~/.claude"},{"name":7},{"name":"test","dir":"` + filepath.Join(home, ".claude-test") + `/"}]`
	c := &Config{unknown: map[string]json.RawMessage{"claudeProfiles": json.RawMessage(saved)}}
	got := claudeAccounts(c, home)
	want := []claudeProfile{{"default", filepath.Join(home, ".claude")}, {"work", filepath.Join(home, ".claude-work")}, {"test", filepath.Join(home, ".claude-test")}}
	if len(got) != len(want) {
		t.Fatalf("accounts %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("account %d: %v, want %v", i, got[i], want[i])
		}
	}
	if only := claudeAccounts(&Config{}, home); len(only) != 1 || only[0].Name != "default" {
		t.Errorf("none saved: %v", only)
	}
	if one, ok := claudeAccountNamed("", only1(home)); !ok || one.Name != "default" {
		t.Error("with one account, none named is that one")
	}
	if _, ok := claudeAccountNamed("", got); ok {
		t.Error("with several, none named is none")
	}
	for _, c := range []struct {
		named  string
		among  []claudeProfile
		status int
		why    string
	}{
		{"", only1(home), 0, ""},
		{"work", got, 0, ""},
		{"", got, http.StatusBadRequest, "Say which Claude account to hire on: default, work, test."},
		{"home", got, http.StatusNotFound, "There's no Claude account called home. Pick one of: default, work, test."},
	} {
		if status, why := claudeAccountRefusal(c.named, c.among); status != c.status || why != c.why {
			t.Errorf("%q: %d %q", c.named, status, why)
		}
	}
}

func only1(home string) []claudeProfile {
	return []claudeProfile{{Name: "default", Dir: filepath.Join(home, ".claude")}}
}

// A test copy's hire sends the copy's own hook key, from the folder its start
// script names, never the real app's (FXM1's N11, for the hook).
func TestAHookInATestCopyReadsTheCopysKey(t *testing.T) {
	copyFolder := t.TempDir()
	if got := hookKeyFileFor(copyFolder); got != filepath.Join(copyFolder, "hook-key") {
		t.Errorf("the copy's: %q", got)
	}
	for _, none := range []string{"", "relative/folder"} {
		if got := hookKeyFileFor(none); got != hookKeyFile {
			t.Errorf("%q: %q, want the app's", none, got)
		}
	}
}
