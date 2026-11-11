package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// claudeAccountsCall asks GET or POST /setup/claude-accounts, as the page (or, asTool, a tool with the setup key).
func claudeAccountsCall(t *testing.T, d *Deck, method, body string, asTool bool) (int, string, map[string]any) {
	t.Helper()
	c := call{method: method, target: "/setup/claude-accounts", host: "localhost:7710", from: thisComputer, body: body, asTool: asTool,
		headers: map[string]string{setupHeader: "1"}}
	w := serve(t, d, c)
	var answer map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &answer)
	return w.Code, w.Body.String(), answer
}

func claudeFolder(t *testing.T, path string, files ...string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(path, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// listOf is answer[key] as a list of objects; a missing or odd one fails the test rather than panicking.
func listOf(t *testing.T, answer map[string]any, key string) []map[string]any {
	t.Helper()
	list, ok := answer[key].([]any)
	if !ok {
		t.Fatalf("%s: %v", key, answer[key])
	}
	out := make([]map[string]any, len(list))
	for i, item := range list {
		if out[i], ok = item.(map[string]any); !ok {
			t.Fatalf("%s[%d]: %v", key, i, item)
		}
	}
	return out
}

func profileNames(answer map[string]any, key string) []string {
	names := []string{}
	list, _ := answer[key].([]any)
	for _, p := range list {
		names = append(names, p.(map[string]any)["name"].(string))
	}
	return names
}

func TestClaudeAccountsListTheDefaultAndOfferFoldersThatLookLikeOne(t *testing.T) {
	d := newTestDeck(t)
	home := homeDir
	claudeFolder(t, filepath.Join(home, ".claude"))
	work := claudeFolder(t, filepath.Join(home, ".claude-work"), "settings.json")
	claudeFolder(t, filepath.Join(home, ".claude-empty"))
	claudeFolder(t, filepath.Join(home, ".claude_old.x"), ".claude.json")
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o600); err != nil { // a file, not a folder
		t.Fatal(err)
	}
	for _, asTool := range []bool{false, true} {
		status, text, got := claudeAccountsCall(t, d, "GET", "", asTool)
		if status != http.StatusOK {
			t.Fatalf("GET as tool=%v: %d %s", asTool, status, text)
		}
		profiles := listOf(t, got, "profiles")
		first := profiles[0]
		if len(profiles) != 1 || first["name"] != "default" || first["dir"] != filepath.Join(home, ".claude") || first["default"] != true || first["connected"] != false {
			t.Errorf("profiles: %v", profiles)
		}
		found := listOf(t, got, "found")
		if len(found) == 0 {
			t.Fatalf("found: %v", got["found"])
		}
		if names := strings.Join(profileNames(got, "found"), ","); names != "work,old-x" || found[0]["dir"] != work {
			t.Errorf("found, by folder name: %v", found)
		}
		if got["most"] != float64(8) {
			t.Errorf("most: %v", got["most"])
		}
		if len(first) != 4 || len(found[0]) != 2 {
			t.Errorf("an account says its name, folder, default and connected, a found one its name and folder: %v %v", first, found[0])
		}
	}
}

func TestClaudeAccountsAreAddedAndTakenOutWhole(t *testing.T) {
	d := newTestDeck(t)
	work := claudeFolder(t, filepath.Join(homeDir, ".claude-work"), "settings.json")
	other := claudeFolder(t, filepath.Join(homeDir, "elsewhere"), "sessions", "x")
	_ = os.Remove(filepath.Join(other, "x"))
	body := `{"profiles":[{"name":"work","dir":"~/.claude-work"},{"name":" side ","dir":"` + filepath.ToSlash(other) + `"}]}`
	status, text, got := claudeAccountsCall(t, d, "POST", body, false)
	if status != http.StatusOK {
		t.Fatalf("POST: %d %s", status, text)
	}
	if names := strings.Join(profileNames(got, "profiles"), ","); names != "default,work,side" {
		t.Errorf("after adding: %s", names)
	}
	if len(profileNames(got, "found")) != 0 {
		t.Errorf("an added folder is no longer offered: %v", got["found"])
	}
	if dir := listOf(t, got, "profiles")[1]["dir"]; dir != work {
		t.Errorf("~ is the home folder: %v", dir)
	}
	// Kept: the config on disk has them, and a deck started from it lists them.
	if saved := loadConfig().ClaudeProfiles; len(saved) != 2 || saved[0].Name != "work" || saved[0].Dir != work {
		t.Errorf("config: %+v", saved)
	}
	_, _, again := claudeAccountsCall(t, d, "GET", "", true)
	if names := strings.Join(profileNames(again, "profiles"), ","); names != "default,work,side" {
		t.Errorf("a tool reads them: %s", names)
	}
	// Taken out: the list is sent whole, so one left off is gone; the folder itself stays.
	_, _, got = claudeAccountsCall(t, d, "POST", `{"profiles":[{"name":"side","dir":"`+filepath.ToSlash(other)+`"}]}`, false)
	if names := strings.Join(profileNames(got, "profiles"), ","); names != "default,side" {
		t.Errorf("after taking work out: %s", names)
	}
	if names := strings.Join(profileNames(got, "found"), ","); names != "work" {
		t.Errorf("a folder taken out is offered again: %s", names)
	}
	if _, err := os.Stat(filepath.Join(work, "settings.json")); err != nil {
		t.Errorf("the account's own folder was touched: %v", err)
	}
	_, _, got = claudeAccountsCall(t, d, "POST", `{"profiles":[]}`, false)
	if names := strings.Join(profileNames(got, "profiles"), ","); names != "default" || len(loadConfig().ClaudeProfiles) != 0 {
		t.Errorf("after taking all out: %s, %+v", names, loadConfig().ClaudeProfiles)
	}
}

func TestClaudeAccountsRefuseAListThatIsntWholeAndKeepNothing(t *testing.T) {
	d := newTestDeck(t)
	good := claudeFolder(t, filepath.Join(homeDir, ".claude-good"), "settings.json")
	plain := claudeFolder(t, filepath.Join(homeDir, "plain"))
	claudeFolder(t, filepath.Join(homeDir, ".claude"), "settings.json")
	if status, _, _ := claudeAccountsCall(t, d, "POST", `{"profiles":[{"name":"good","dir":"`+filepath.ToSlash(good)+`"}]}`, false); status != http.StatusOK {
		t.Fatalf("a good list: %d", status)
	}
	many := make([]string, 9)
	for i := range many {
		many[i] = `{"name":"a` + string(rune('a'+i)) + `","dir":"` + filepath.ToSlash(good) + `"}`
	}
	for _, c := range []struct{ name, body, want string }{
		{"no list", `{}`, "Send {profiles: [{name, dir}]}."},
		{"not json", `nope`, "Send {profiles: [{name, dir}]}."},
		{"not objects", `{"profiles":["work"]}`, "Send {profiles: [{name, dir}]}."},
		{"a name with a space inside", `{"profiles":[{"name":"my work","dir":"` + filepath.ToSlash(good) + `"}]}`, "A name is up to 30 letters, digits, dashes or underscores."},
		{"no name", `{"profiles":[{"dir":"` + filepath.ToSlash(good) + `"}]}`, "A name is up to 30 letters"},
		{"a name too long", `{"profiles":[{"name":"` + strings.Repeat("a", 31) + `","dir":"` + filepath.ToSlash(good) + `"}]}`, "A name is up to 30 letters"},
		{"a folder that isn't a full path", `{"profiles":[{"name":"w","dir":".claude-good"}]}`, "Give the account's folder as a full path, such as ~/.claude-work."},
		{"default", `{"profiles":[{"name":"default","dir":"` + filepath.ToSlash(good) + `"}]}`, `"default" is ~/.claude, which is always there.`},
		{"~/.claude itself", `{"profiles":[{"name":"mine","dir":"~/.claude"}]}`, `"default" is ~/.claude, which is always there.`},
		{"a folder that isn't there", `{"profiles":[{"name":"w","dir":"~/nowhere"}]}`, "nowhere isn't a folder on this computer."},
		{"a folder that doesn't look like an account", `{"profiles":[{"name":"w","dir":"` + filepath.ToSlash(plain) + `"}]}`, "doesn't look like a Claude Code folder"},
		{"a second folder that isn't there", `{"profiles":[{"name":"w","dir":"` + filepath.ToSlash(good) + `"},{"name":"v","dir":"` + filepath.ToSlash(good) + `2"}]}`, "isn't a folder"},
		{"a folder twice", `{"profiles":[{"name":"w","dir":"` + filepath.ToSlash(good) + `"},{"name":"v","dir":"` + filepath.ToSlash(good) + `/"}]}`, "is listed twice."},
		{"more than the most", `{"profiles":[` + strings.Join(many, ",") + `]}`, "That's the most accounts there can be: 8."},
	} {
		status, text, _ := claudeAccountsCall(t, d, "POST", c.body, false)
		if status != http.StatusBadRequest || !strings.Contains(text, c.want) {
			t.Errorf("%s: %d %q, want 400 with %q", c.name, status, text, c.want)
		}
	}
	if saved := loadConfig().ClaudeProfiles; len(saved) != 1 || saved[0].Name != "good" {
		t.Errorf("a refused list changed the list: %+v", saved)
	}
	// Two accounts of one name.
	second := claudeFolder(t, filepath.Join(homeDir, ".claude-second"), "settings.json")
	body := `{"profiles":[{"name":"w","dir":"` + filepath.ToSlash(good) + `"},{"name":"w","dir":"` + filepath.ToSlash(second) + `"}]}`
	if status, text, _ := claudeAccountsCall(t, d, "POST", body, false); status != http.StatusBadRequest || !strings.Contains(text, "Two accounts are called w.") {
		t.Errorf("a name twice: %d %q", status, text)
	}
}

func TestOnlyThePageChangesTheClaudeAccounts(t *testing.T) {
	d := newTestDeck(t)
	good := claudeFolder(t, filepath.Join(homeDir, ".claude-good"), "settings.json")
	body := `{"profiles":[{"name":"good","dir":"` + filepath.ToSlash(good) + `"}]}`
	if status, _, _ := claudeAccountsCall(t, d, "POST", body, true); status != http.StatusForbidden {
		t.Errorf("a tool's POST: %d", status)
	}
	if len(loadConfig().ClaudeProfiles) != 0 {
		t.Error("a tool's POST changed the list")
	}
}

// An account is connected only when Smolcorp's hooks are really in its settings.json, as for ~/.claude.
func TestAClaudeAccountIsConnectedOnlyWhenItsHooksAre(t *testing.T) {
	d := newTestDeck(t)
	claudeFolder(t, filepath.Join(homeDir, ".claude"))
	work := claudeFolder(t, filepath.Join(homeDir, ".claude-work"), "settings.json")
	claudeFolder(t, filepath.Join(homeDir, ".claude-bare"), ".claude.json")
	body := `{"profiles":[{"name":"work","dir":"` + filepath.ToSlash(work) + `"},{"name":"bare","dir":"~/.claude-bare"}]}`
	connectedOf := func() map[string]any {
		_, _, got := claudeAccountsCall(t, d, "GET", "", false)
		out := map[string]any{}
		for _, p := range listOf(t, got, "profiles") {
			name, _ := p["name"].(string)
			out[name] = p["connected"]
		}
		return out
	}
	claudeAccountsCall(t, d, "POST", body, false)
	if got := connectedOf(); got["default"] != false || got["work"] != false || got["bare"] != false {
		t.Fatalf("nothing is connected: %v", got)
	}
	if err := connect(Claude, false); err != nil {
		t.Fatal(err)
	}
	if got := connectedOf(); got["default"] != true || got["work"] != false || got["bare"] != false {
		t.Errorf("only ~/.claude's hooks are in: %v", got)
	}
	data, err := os.ReadFile(filepath.Join(homeDir, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "settings.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := connectedOf(); got["work"] != true || got["bare"] != false {
		t.Errorf("work has the hooks now: %v", got)
	}
}

func TestTheSetupStateHasTheClaudeAccounts(t *testing.T) {
	d := newTestDeck(t)
	accounts, ok := (&router{deck: d}).setupState()["claudeAccounts"].(map[string]any)
	if !ok {
		t.Fatalf("claudeAccounts: %v", accounts)
	}
	if profiles, _ := accounts["profiles"].([]map[string]any); len(profiles) != 1 || accounts["most"] != claudeAccountsMost {
		t.Errorf("claudeAccounts: %v", accounts)
	}
}

// A folder outside the home folder is never kept, looked in or read, however the path gets out: it is named
// outright, goes up with "..", is a link that leads out, or (on Windows) is a network or device path.
func TestClaudeAccountsRefuseAFolderOutsideTheHomeFolder(t *testing.T) {
	d := newTestDeck(t)
	outside := claudeFolder(t, filepath.Join(t.TempDir(), "elsewhere"), "settings.json")
	up, err := filepath.Rel(homeDir, outside)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(homeDir, "link")
	linked := os.Symlink(outside, link) == nil
	if !linked {
		t.Log("no symlinks here (Windows without the right): that case is skipped")
	}
	// A full path outside is said as that (C3 A10); one that isn't a plain full path, as before.
	notFull := "Give the account's folder as a full path, such as ~/.claude-work."
	outsideOf := func(dir string) string {
		return dir + " isn't inside your home folder: an account's folder is one of yours, such as ~/.claude-work."
	}
	for _, c := range []struct {
		name, dir string
		skip      bool
		want      string
	}{
		{"named outright", filepath.ToSlash(outside), false, outsideOf(outside)},
		{"up with ..", "~/" + filepath.ToSlash(up), false, outsideOf(outside)},
		{"a link that leads out", filepath.ToSlash(link), !linked, outsideOf(link)},
		{"a network path", `\\\\server\\share\\x`, false, notFull},
		{"a network path with slashes", `//server/share/x`, false, notFull},
		{"a device path", `\\\\?\\C:\\x`, false, notFull},
	} {
		if c.skip {
			continue
		}
		status, text, _ := claudeAccountsCall(t, d, "POST", `{"profiles":[{"name":"w","dir":"`+c.dir+`"}]}`, false)
		if status != http.StatusBadRequest || text != c.want {
			t.Errorf("%s: %d %q, want 400 %q", c.name, status, text, c.want)
		}
	}
	if saved := loadConfig().ClaudeProfiles; len(saved) != 0 {
		t.Errorf("a refused list was kept: %+v", saved)
	}
	// A list already saved that now leads out (the link was moved) is left off the list, not looked in.
	if linked {
		inside := claudeFolder(t, filepath.Join(homeDir, ".claude-in"), "settings.json")
		d.mu.Lock()
		d.config.ClaudeProfiles = []claudeProfile{{Name: "in", Dir: inside}, {Name: "out", Dir: link}}
		d.mu.Unlock()
		_, _, got := claudeAccountsCall(t, d, "GET", "", false)
		if names := strings.Join(profileNames(got, "profiles"), ","); names != "default,in" {
			t.Errorf("a saved folder that leads out: %s", names)
		}
	}
}

// Adding an account or taking one out only changes the list: nothing is written into the account's folder, not
// its settings.json (the Mac puts its hooks in and takes them out; this app doesn't) and no new file.
func TestAddingOrTakingOutAnAccountWritesNothingInItsFolder(t *testing.T) {
	d := newTestDeck(t)
	claudeFolder(t, filepath.Join(homeDir, ".claude"))
	if err := connect(Claude, false); err != nil { // the Mac connects an added account only when ~/.claude is
		t.Fatal(err)
	}
	hooks, err := os.ReadFile(filepath.Join(homeDir, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	work := claudeFolder(t, filepath.Join(homeDir, ".claude-work"), "settings.json")
	state := func() (string, string) {
		data, err := os.ReadFile(filepath.Join(work, "settings.json"))
		if err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(work)
		if err != nil {
			t.Fatal(err)
		}
		files := []string{}
		for _, e := range entries {
			files = append(files, e.Name())
		}
		return string(data), strings.Join(files, ",")
	}
	body := `{"profiles":[{"name":"work","dir":"` + filepath.ToSlash(work) + `"}]}`
	data, files := state()
	if status, text, _ := claudeAccountsCall(t, d, "POST", body, false); status != http.StatusOK {
		t.Fatalf("add: %d %s", status, text)
	}
	if after, afterFiles := state(); after != data || afterFiles != files {
		t.Errorf("adding wrote in the folder: %q %q, was %q %q", after, afterFiles, data, files)
	}
	// Hooks in it first, as if put there by hand: taking it out leaves them, as it leaves everything.
	if err := os.WriteFile(filepath.Join(work, "settings.json"), hooks, 0o600); err != nil {
		t.Fatal(err)
	}
	data, files = state()
	if status, text, _ := claudeAccountsCall(t, d, "POST", `{"profiles":[]}`, false); status != http.StatusOK {
		t.Fatalf("remove: %d %s", status, text)
	}
	if after, afterFiles := state(); after != data || afterFiles != files {
		t.Errorf("taking out wrote in the folder: %q %q, was %q %q", after, afterFiles, data, files)
	}
}

// A name's letters and digits are the Mac's alphanumerics: combining marks and other numbers count.
func TestAClaudeAccountsNameMayHaveMarksAndOtherNumbers(t *testing.T) {
	d := newTestDeck(t)
	good := claudeFolder(t, filepath.Join(homeDir, ".claude-good"), "settings.json")
	body := `{"profiles":[{"name":"café-2²","dir":"` + filepath.ToSlash(good) + `"}]}`
	if status, text, _ := claudeAccountsCall(t, d, "POST", body, false); status != http.StatusOK {
		t.Errorf("a name with a combining accent and a superscript two: %d %s", status, text)
	}
}

// The accounts a hire can work on are setup's list: the default, then the saved ones the Mac would keep (well
// formed, still folders, no name or folder twice), and the named one is picked as the Mac's ClaudeProfiles does.
func TestClaudeAccountsAreTheMacs(t *testing.T) {
	isolate(t)
	home := homeDir
	for _, dir := range []string{".claude", ".claude-work", ".claude-test"} {
		os.MkdirAll(filepath.Join(home, dir), 0o700)
	}
	saved := []claudeProfile{{"work", "~/.claude-work"}, {"bad name", "~/.claude-test"}, {"gone", "~/.claude-gone"},
		{"again", "~/.claude-work"}, {"default", "~/.claude-test"}, {"me", "relative"}, {"home", "~/.claude"}, {Name: "nofolder"},
		{"test", filepath.Join(home, ".claude-test") + "/"}}
	got := claudeAccountList(saved)
	want := []claudeProfile{{"default", filepath.Join(home, ".claude")}, {"work", filepath.Join(home, ".claude-work")}, {"test", filepath.Join(home, ".claude-test")}}
	if len(got) != len(want) {
		t.Fatalf("accounts %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("account %d: %v, want %v", i, got[i], want[i])
		}
	}
	if only := claudeAccountList(nil); len(only) != 1 || only[0].Name != "default" {
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

// badlyFormedAccounts are saved entries no hire gets a rule for: network paths, a relative path and one with a NUL.
var badlyFormedAccounts = []claudeProfile{{Name: "share", Dir: "//server/share/.claude-s"}, {Name: "unc", Dir: `\\server\share\.claude-u`},
	{Name: "rel", Dir: "rel/.claude-r"}, {Name: "nul", Dir: "~/.claude-n\x00"}}

// deniesTheAccounts checks that a hire's settings deny each of dirs' settings and transcripts, and give no rule
// for any of badlyFormedAccounts.
func deniesTheAccounts(t *testing.T, where, settings string, dirs ...string) {
	t.Helper()
	for _, dir := range dirs {
		for _, rule := range []string{"Edit(" + absoluteRulePath(dir) + "/settings*.json)", "Read(" + absoluteRulePath(dir) + "/projects/**)"} {
			if !strings.Contains(settings, rule) {
				t.Errorf("%s: %s isn't denied:\n%s", where, rule, settings)
			}
		}
	}
	for _, part := range []string{"server/share", "server\\\\share", ".claude-s", ".claude-u", ".claude-r", ".claude-n"} {
		if strings.Contains(settings, part) {
			t.Errorf("%s: a rule for a badly formed folder (%s):\n%s", where, part, settings)
		}
	}
}
