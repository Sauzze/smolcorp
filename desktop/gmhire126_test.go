package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// FXGMHIRE126: a hire through the launcher (Windows' way, hirelauncher.go) starts without a question at its brief or
// its folder's trust, takes the model and nickname the GM's hire names, and a GM in a folder that isn't a repo keeps
// its brief and board where it may write. No build tag: the Windows CI job runs these on the real Windows paths; here
// they run on the launcher's way with launcherHires switched on (newWinRig).

// launcherPermissions are the permissions of hire hireID's settings, as its launcher was handed them.
func launcherPermissions(t *testing.T, w *winRig, hireID string) (allow, deny []string) {
	t.Helper()
	var settings struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
		} `json:"permissions"`
	}
	w.mu.Lock()
	text := w.settings[hireID]
	w.mu.Unlock()
	if err := json.Unmarshal([]byte(text), &settings); err != nil {
		t.Fatalf("hire %s's settings: %v\n%s", hireID, err, text)
	}
	return settings.Permissions.Allow, settings.Permissions.Deny
}

// toolRules are rules' patterns for tool (Read, Edit) written as Read rules, so readDenied matches them.
func toolRules(rules []string, tool string) []string {
	var as []string
	for _, rule := range rules {
		if pattern, ok := strings.CutPrefix(rule, tool+"("); ok {
			as = append(as, "Read("+pattern)
		}
	}
	return as
}

// briefsWritten are the files in Smolcorp's briefs folder (its templates folder aside).
func briefsWritten() []string {
	var files []string
	entries, _ := os.ReadDir(briefsKeptDir())
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files
}

// launcherStartEnv is what the launcher of the hire opened by command is handed, read as its window opens.
func launcherStartEnv(d *Deck, command []string) map[string]any {
	d.mu.Lock()
	defer d.mu.Unlock()
	pending := d.hires[after(command, "--hire")]
	if pending == nil || pending.env == nil {
		return nil
	}
	data, _ := json.Marshal(*pending.env)
	var env map[string]any
	_ = json.Unmarshal(data, &env)
	return env
}

// #3b: a launcher hire starts in Claude Code's default mode, where every Read outside its folder asks: its settings
// let it read its own folder (its brief), and nothing else: not another hire's folder, not Smolcorp's briefs folder,
// and never a write.
func TestALauncherHireMayReadItsOwnFolderAndNoOtherHires(t *testing.T) {
	w := newWinRig(t)
	first, second := w.hire("janitor"), w.hire("janitor")
	allow, deny := launcherPermissions(t, w, second.ID)
	if len(allow) != 1 || len(toolRules(allow, "Read")) != 1 {
		t.Fatalf("the hire's settings allow %q, want one Read rule", allow)
	}
	own := hireFolder(supportDir, second.ID)
	for _, c := range []struct {
		file string
		want bool
	}{
		{filepath.Join(own, "BRIEF.md"), true},
		{filepath.Join(own, "notes", "plan.md"), true}, // whatever its brief has it read there
		{filepath.Join(hireFolder(supportDir, first.ID), "BRIEF.md"), false},
		{filepath.Join(supportDir, "hires", second.ID+"x", "BRIEF.md"), false},
		{filepath.Join(supportDir, "briefs", "0-janitor-20261118-0900.md"), false},
		{filepath.Join(supportDir, "registry.json"), false},
	} {
		if got := readDenied(toolRules(allow, "Read"), homeDir, c.file); got != c.want {
			t.Errorf("Read of %s allowed: %v, want %v (allow %q)", c.file, got, c.want, allow)
		}
	}
	// Nothing is allowed to be written, and the denies still hold in its own folder: its settings can't be changed.
	if !readDenied(toolRules(deny, "Edit"), homeDir, filepath.Join(own, hireSettingsFile)) {
		t.Errorf("the hire's own settings file isn't denied to Edit: %q", deny)
	}
	// The first hire's settings were written while it was alone: they name its own folder, never the second's.
	allow, _ = launcherPermissions(t, w, first.ID)
	if len(allow) != 1 || readDenied(toolRules(allow, "Read"), homeDir, filepath.Join(own, "BRIEF.md")) {
		t.Errorf("the first hire's settings allow %q", allow)
	}
}

// RT126GMHIRE126 N1: no hire may edit a brief, its own or another hire's, a hire started after its settings were
// written included (the verifier's probe: two hires with no repo, the first's settings written while it was alone).
// Its status file stays its own to write; another's, by name, is denied only to the hires started after it.
func TestNoHireMayEditABriefItsOwnOrALaterHires(t *testing.T) {
	w := newWinRig(t)
	first, second := w.hire("janitor"), w.hire("janitor")
	// Where an engine hire's brief is written (hireBrief), with or without a repo; the rules are patterns, so no brief
	// need be there.
	mine, later := hireFolder(supportDir, first.ID), hireFolder(supportDir, second.ID)
	myBrief, laterBrief := filepath.Join(mine, "BRIEF.md"), filepath.Join(later, "BRIEF.md")
	_, deny := launcherPermissions(t, w, first.ID)
	edits := toolRules(deny, "Edit")
	for _, c := range []struct {
		file   string
		denied bool
	}{
		{laterBrief, true},
		{filepath.Join(later, "BRIEF-2.md"), true},
		{filepath.Join(later, "BRIEF-tester.md"), true},
		{myBrief, true},
		{filepath.Join(mine, "STATUS.md"), false},
		{filepath.Join(mine, "notes.md"), false},
	} {
		if got := readDenied(edits, homeDir, c.file); got != c.denied {
			t.Errorf("the first hire's Edit of %s denied: %v, want %v", c.file, got, c.denied)
		}
	}
	_, deny = launcherPermissions(t, w, second.ID)
	if edits := toolRules(deny, "Edit"); !readDenied(edits, homeDir, myBrief) || !readDenied(edits, homeDir, laterBrief) ||
		!readDenied(edits, homeDir, filepath.Join(mine, "STATUS.md")) || readDenied(edits, homeDir, filepath.Join(later, "STATUS.md")) {
		t.Errorf("the second hire's Edit denies: %q", deny)
	}
}

// RT126GMHIRE126 O1: a nickname is refused with any control character in it (ESC, NUL, TAB, BEL, DEL, the C1 ones), as
// well as a line's end; a nickname of letters, blanks and marks from any language is taken.
func TestANicknameWithAControlCharacterIsRefused(t *testing.T) {
	for _, nickname := range []string{"Ada\x1b[31m", "Ada\x00", "Ada\tLovelace", "Ada\a", "Ada\x7f", "Ada\u009b", "Ada\u0085",
		"Ada\nLovelace", "Ada\rx", "Ada\u2028x", "Ada\u2029x"} {
		if hireNicknameAllowed(nickname) {
			t.Errorf("%q is allowed", nickname)
		}
	}
	for _, nickname := range []string{"", "Ada", "Ada Lovelace", "Zoë", "李雷", "Ada-2 (QA)", strings.Repeat("a", longestNickname)} {
		if !hireNicknameAllowed(nickname) {
			t.Errorf("%q is refused", nickname)
		}
	}
	w := newWinRig(t)
	if status, text := w.post("/setup/gm/hire", map[string]any{"role": "janitor", "nickname": "Ada\x1b]0;x\x07"}, winAsGM); status != http.StatusBadRequest || text != hireNicknameRefusal {
		t.Errorf("the GM's hire: %d %q, want 400 %q", status, text, hireNicknameRefusal)
	}
	if len(w.opened) != 0 || len(winRegistry(t)) != 0 {
		t.Fatalf("a refused hire opened %q, registry %+v", w.opened, winRegistry(t))
	}
}

// #4: the GM's `gm hire --model opus --nickname Ada` on the launcher's way: 200, the row has both, the hire's worktree
// name its nickname would give, and the launcher's start carries the model (the role it reads again from the config
// says sonnet). A model or nickname that isn't one is still refused, with nothing opened.
func TestTheGMHireOnTheLauncherCarriesItsModelAndNickname(t *testing.T) {
	w := newWinRig(t)
	var env map[string]any
	open := w.d.launcherEngine.Open
	w.d.launcherEngine.Open = func(command []string, folder string) error {
		env = launcherStartEnv(w.d, command)
		return open(command, folder)
	}
	for _, body := range []map[string]any{
		{"role": "janitor", "model": "op us"},
		{"role": "janitor", "model": "opus; rm -rf ~"},
		{"role": "janitor", "model": 5},
		{"role": "janitor", "model": strings.Repeat("o", setupHireModelLongest+1)},
		{"role": "janitor", "nickname": "Ada\nLovelace"},
		{"role": "janitor", "nickname": strings.Repeat("a", longestNickname+1)},
		{"role": "janitor", "nickname": 7},
	} {
		want := hireModelRefusal
		if body["nickname"] != nil {
			want = hireNicknameRefusal
		}
		if status, text := w.post("/setup/gm/hire", body, winAsGM); status != http.StatusBadRequest || text != want {
			t.Errorf("%.60v: %d %q, want 400 %q", body, status, text, want)
		}
	}
	if len(w.opened) != 0 || len(winRegistry(t)) != 0 {
		t.Fatalf("a refused hire opened %q, registry %+v", w.opened, winRegistry(t))
	}
	status, text := w.post("/setup/gm/hire", map[string]any{"role": "janitor", "model": " opus ", "nickname": " Ada "}, winAsGM)
	if status != http.StatusOK {
		t.Fatalf("hire: %d %q", status, text)
	}
	rows := winRegistry(t)
	if len(rows) != 1 || !rows[0].ViaLauncher || len(w.opened) != 1 {
		t.Fatalf("not made by the launcher: opened %q, registry %+v", w.opened, rows)
	}
	if rows[0].Model != "opus" || rows[0].Nickname != "Ada" {
		t.Errorf("the row has model %q and nickname %q, want opus and Ada", rows[0].Model, rows[0].Nickname)
	}
	if env["model"] != "opus" {
		t.Errorf("the launcher's start doesn't carry the model: %v", env)
	}
	// Only ids and names on the command line: the model rides with the start environment, never as an argument.
	if slices.ContainsFunc(w.opened[0], func(a string) bool { return strings.Contains(a, "opus") }) {
		t.Errorf("the model is on the launcher's command line: %q", w.opened[0])
	}
}

// #5: a GM hired into a folder that isn't a repo (no worktree) on the launcher's way: its brief is in its own folder,
// hires/<id>/, with its status file and TEAM-BOARD beside it, which its fence lets it edit; Smolcorp's briefs folder,
// which holds other desks' briefs, is still denied whole. Its first message names the brief there.
func TestAGMInAFolderThatIsNoRepoKeepsItsBriefAndBoardInItsOwnFolder(t *testing.T) {
	w := newWinRig(t)
	w.d.mu.Lock()
	w.d.config.Roles[1].Brief = &RoleBrief{Kind: "library", ID: "gm"}
	w.d.mu.Unlock()
	var env map[string]any
	open := w.d.launcherEngine.Open
	w.d.launcherEngine.Open = func(command []string, folder string) error {
		env = launcherStartEnv(w.d, command)
		return open(command, folder)
	}
	gm := w.hire("gm")
	own := hireFolder(supportDir, gm.ID)
	if gm.Worktree != "" || gm.Brief != filepath.Join(own, "BRIEF.md") {
		t.Fatalf("the GM's brief is %q (worktree %q), want %q", gm.Brief, gm.Worktree, filepath.Join(own, "BRIEF.md"))
	}
	brief, err := os.ReadFile(gm.Brief)
	if err != nil || !strings.Contains(string(brief), tildePath(filepath.Join(own, "STATUS.md"))) || !strings.Contains(string(brief), tildePath(filepath.Join(own, "TEAM-BOARD.md"))) {
		t.Errorf("the brief doesn't name its status and board beside it (%v):\n%s", err, brief)
	}
	if message, _ := env["message"].(string); !strings.Contains(message, tildePath(gm.Brief)) && !strings.Contains(message, gm.Brief) {
		t.Errorf("the first message doesn't name the brief %s: %q", gm.Brief, message)
	}
	if left := briefsWritten(); len(left) != 0 {
		t.Errorf("a brief was written in Smolcorp's briefs folder: %v", left)
	}
	allow, deny := launcherPermissions(t, w, gm.ID)
	edits := toolRules(deny, "Edit")
	for _, name := range []string{"STATUS.md", "TEAM-BOARD.md"} {
		if file := filepath.Join(own, name); readDenied(edits, homeDir, file) {
			t.Errorf("Edit of %s is denied", file)
		}
	}
	// Its brief is the core's to append to, not its own to change (RT126GMHIRE126 N1).
	if !readDenied(edits, homeDir, gm.Brief) {
		t.Errorf("Edit of its own brief isn't denied: %q", deny)
	}
	if !readDenied(edits, homeDir, filepath.Join(supportDir, "briefs", "0-tester-20261118-0900.md")) {
		t.Errorf("Edit of Smolcorp's briefs folder isn't denied: %q", deny)
	}
	if !readDenied(toolRules(allow, "Read"), homeDir, gm.Brief) {
		t.Errorf("the GM's settings don't let it read its own brief: %q", allow)
	}
	// Its recap reads its status from there.
	if err := os.WriteFile(filepath.Join(own, "STATUS.md"), []byte("The plan is agreed; two hires at work.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	recap, _ := os.ReadFile(writeRecap(gm, "", false, time.Now()))
	if !strings.Contains(string(recap), "two hires at work") {
		t.Errorf("the recap doesn't read the GM's status in its own folder:\n%s", recap)
	}
}

// #3a, on this system: a launcher hire into the folder for hires, saved in another spelling than the one physical
// gives (on Windows: its drive letter in lower case; on windows-latest the temp folder's 8.3 short name RUNNER~1 as
// well), which is no repo, so it has no worktree: the folder its window opens in is trusted in the account's
// .claude.json under exactly that spelling on Windows (and its resolved form elsewhere, where the kernel resolves a
// working folder), with the drive letter in upper case and no short name.
func TestALauncherHiresFolderIsTrustedUnderTheSpellingItStartsIn(t *testing.T) {
	w := newWinRig(t)
	saved := filepath.Join(homeDir, "proj") // as the temp folder spells it, not resolved
	if len(saved) >= 2 && saved[1] == ':' {
		saved = strings.ToLower(saved[:1]) + saved[1:]
	}
	var openedIn []string
	open := w.d.launcherEngine.Open
	w.d.launcherEngine.Open = func(command []string, folder string) error {
		openedIn = append(openedIn, folder)
		return open(command, folder)
	}
	w.d.mu.Lock()
	w.d.config.HireFolder, w.d.lastHire = saved, time.Time{}
	w.d.mu.Unlock()
	if status, text := w.d.Perform("hire", "", map[string][]string{"role": {"janitor"}}); status != http.StatusNoContent {
		t.Fatalf("the office's hire: %d %q", status, text)
	}
	w.d.watching.Wait()
	rows := winRegistry(t)
	if len(rows) != 1 || !rows[0].ViaLauncher || rows[0].Worktree != "" || len(openedIn) != 1 {
		t.Fatalf("not one launcher hire with no worktree: opened in %q, registry %+v", openedIn, rows)
	}
	data, err := os.ReadFile(claudeJSON(""))
	if err != nil {
		t.Fatal(err)
	}
	start := openedIn[0]
	if runtime.GOOS != "windows" {
		start = physical(start)
	}
	if !isTrusted(start, data) {
		t.Errorf("the folder the hire starts in, %q, isn't trusted:\n%s", start, data)
	}
	if runtime.GOOS == "windows" {
		if start != physical(saved) || strings.Contains(start, "~") || start[:1] != strings.ToUpper(start[:1]) {
			t.Errorf("on Windows it starts in %q, want the long form with the drive in upper case, %q", start, physical(saved))
		}
		if trustBothWindowsSpellings && !isTrusted(strings.ReplaceAll(start, `\`, "/"), data) {
			t.Errorf("the forward-slash spelling of %q isn't trusted:\n%s", start, data)
		}
	}
}
