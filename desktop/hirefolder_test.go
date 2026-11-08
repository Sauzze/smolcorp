package desktop

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Start here's hires folder (T781): as the Mac app's HireFolderTests.

// hireFolderKeys are the route's JSON keys, on a Mac and on Linux/Windows alike.
var hireFolderKeys = []string{"exists", "fix", "folder", "home", "problem", "suggested"}

func hireFolderHome(t *testing.T) (home, base string) {
	t.Helper()
	home = t.TempDir()
	return home, realPath(home)
}

func mkdirs(t *testing.T, home string, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Join(home, filepath.FromSlash(p)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// lockFolder makes a folder this user can't write in, where the system has such a thing.
func lockFolder(t *testing.T, folder string) bool {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return false
	}
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(folder, 0o755) })
	return true
}

func TestTheHireFolderSuggestionIsTheFirstLikelyFolderThere(t *testing.T) {
	home, base := hireFolderHome(t)
	if got := hireFolderSuggestion(home); got != filepath.Join(base, "Smolcorp") {
		t.Fatalf("none there: %q", got)
	}
	if _, err := os.Stat(filepath.Join(home, "Smolcorp")); err == nil {
		t.Fatal("suggesting made the folder")
	}
	for _, name := range []string{"src", "Code", "Developer", "Projects"} {
		mkdirs(t, home, name)
		if got := hireFolderSuggestion(home); got != filepath.Join(base, name) {
			t.Errorf("with %s there: %q", name, got)
		}
	}
}

func TestALikelyFolderThatFailsAChecksIsPassedOver(t *testing.T) {
	home, base := hireFolderHome(t)
	if err := os.WriteFile(filepath.Join(home, "Projects"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	mkdirs(t, home, "Code")
	if got := hireFolderSuggestion(home); got != filepath.Join(base, "Code") {
		t.Errorf("~/Projects a file: %q", got)
	}
	_ = os.Remove(filepath.Join(home, "Code"))
	mkdirs(t, home, "Developer")
	if !lockFolder(t, filepath.Join(home, "Developer")) {
		return
	}
	if got := hireFolderSuggestion(home); got != filepath.Join(base, "Smolcorp") {
		t.Errorf("~/Developer locked: %q", got)
	}
}

// hireFolderCase is a folder given, and what its check should say.
type hireFolderCase struct {
	given, problem, fix string
	exists              bool
}

func TestEveryHireFolderCheckSaysWhatsWrongWithItsFix(t *testing.T) {
	home, _ := hireFolderHome(t)
	mkdirs(t, home, "Projects/app", "Library/Work", ".config/x", "Projects/smolcorp/.team/scratch", "AppData/Roaming", "Applications/Smolcorp.app/Contents", "Locked")
	if err := os.WriteFile(filepath.Join(home, "notes.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	sep := string(filepath.Separator)
	example := "such as ~" + sep + "Projects" // ~\Projects on Windows
	system := "That folder holds your computer's or your apps' own files. Choose a project folder, " + example + "."
	// A hidden (dot) folder says so, not "your computer's own files" (fresh eyes 3, problem 4).
	insideHidden := "A folder inside a hidden folder (one whose name starts with '.') can't be used: choose one outside it."
	hidden := "A hidden folder (one whose name starts with '.') can't be used. Choose a project folder, " + example + "."
	wholeDisk := "That's a whole disk. Choose a project folder, " + example + "."
	wholeHome := "That's your whole home folder. Choose a folder inside it, " + example + ", so hires keep to their projects."
	root := sep
	if runtime.GOOS == "windows" {
		root = filepath.VolumeName(home) + sep
	}
	cases := []hireFolderCase{
		{"", "Type the folder's full path, " + example + ".", "choose", false},
		{"Projects", "Give the folder's full path, " + example + ".", "choose", false},
		{root, wholeDisk, "choose", false},
		{home, wholeHome, "choose", false},
		{home + sep, wholeHome, "choose", false},
		{"~", wholeHome, "choose", false},
		{filepath.Join(home, "Library", "Work"), system, "choose", false},
		{filepath.Join(home, "LIBRARY", "New"), system, "choose", false},
		{filepath.Join(home, ".config", "x"), insideHidden, "choose", false},
		{filepath.Join(home, ".config"), hidden, "choose", false},
		{filepath.Join(home, "Projects", "smolcorp", ".team", "scratch"), insideHidden, "choose", false},
		{filepath.Join(home, "Projects", "smolcorp", ".team"), hidden, "choose", false},
		{"~/Projects/.Hidden/New", insideHidden, "choose", false},
		// A system or app folder is still said as one, hidden folder or not.
		{filepath.Join(home, "Library", ".cache"), system, "choose", false},
		{filepath.Join(home, "AppData", "Roaming"), system, "choose", false},
		{filepath.Join(home, "Applications", "Smolcorp.app", "Contents"), system, "choose", false},
		{filepath.Join(home, "Missing", "deeper"), "There's no folder at ~" + sep + "Missing" + sep + "deeper yet.", "create", false},
		{filepath.Join(home, "notes.md"), "~" + sep + "notes.md is a file, not a folder.", "choose", true},
		{filepath.Join(home, "Projects", "app"), "", "", true},
		{" " + filepath.Join(home, "Projects", "app") + sep + " ", "", "", true},
		{"~/Projects/app", "", "", true},
		{filepath.Join(home, "Projects", "..", "Projects", "app"), "", "", true},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases,
			hireFolderCase{`C:\Windows\Temp\x`, system, "choose", false},
			hireFolderCase{`C:\Program Files\x`, system, "choose", false},
		)
	} else {
		// Another disk is fine, but not the whole disk; nor the system's own folders, by a symlink either.
		if err := os.Symlink("/usr", filepath.Join(home, "out")); err != nil {
			t.Fatal(err)
		}
		disk := "smolcorp-no-such-disk"
		cases = append(cases,
			hireFolderCase{"/Volumes/" + disk, wholeDisk, "choose", false},
			hireFolderCase{"/mnt/" + disk, wholeDisk, "choose", false},
			hireFolderCase{"/media/me/" + disk, wholeDisk, "choose", false},
			hireFolderCase{"/Volumes/" + disk + "/code", "There's no folder at /Volumes/" + disk + "/code yet.", "create", false},
			hireFolderCase{"/mnt/" + disk + "/code", "There's no folder at /mnt/" + disk + "/code yet.", "create", false},
			hireFolderCase{"/usr/local/new", system, "choose", false},
			hireFolderCase{"/etc", system, "choose", false},
			hireFolderCase{filepath.Join(home, "out"), system, "choose", false},
		)
	}
	if lockFolder(t, filepath.Join(home, "Locked")) {
		cases = append(cases, hireFolderCase{filepath.Join(home, "Locked"), "Smolcorp can't write in ~" + sep + "Locked. Choose a folder of your own, " + example + ".", "choose", true})
	}
	for _, c := range cases {
		got := checkHireFolder(c.given, home, choosingHireFolder)
		if got.problem != c.problem || got.fix != c.fix || got.exists != c.exists {
			t.Errorf("%q: %+v, want problem %q fix %q exists %v", c.given, got, c.problem, c.fix, c.exists)
		}
	}
}

// A folder on another drive is fine for a new one: D:\code on Windows.
func TestAHireFolderOnAnotherDriveIsFine(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("drive letters are Windows'")
	}
	home, _ := hireFolderHome(t)
	other := "D:"
	if strings.EqualFold(filepath.VolumeName(home), other) {
		other = "E:"
	}
	got := checkHireFolder(other+`\code`, home, choosingHireFolder)
	if got.fix == "choose" {
		t.Errorf("%s\\code refused: %+v", other, got)
	}
}

// A folder set before these checks is judged only by what hiring needs, so an older config stays as it was.
func TestASavedHireFolderOutsideHomeHasNoProblem(t *testing.T) {
	home, _ := hireFolderHome(t)
	elsewhere := t.TempDir() // outside home, and on a Mac under /private, where a new folder may not be
	for _, saved := range []string{elsewhere, home} {
		state := hireFolderState(saved, nil, home)
		if state["problem"] != "" || state["fix"] != "" || state["exists"] != true || state["folder"] != saved {
			t.Errorf("saved %q: %v", saved, state)
		}
	}
	if got := checkHireFolder(elsewhere, home, savedHireFolder); got.problem != "" {
		t.Errorf("saved %q: %+v", elsewhere, got)
	}
	// Gone since: still said, with Create it.
	if state := hireFolderState(filepath.Join(elsewhere, "Gone"), nil, home); state["fix"] != "create" || state["exists"] != false {
		t.Errorf("gone: %v", state)
	}
}

func TestKeepingTheHireFolderMakesItOnlyWhenAskedAndOnlyWhereItMay(t *testing.T) {
	home, base := hireFolderHome(t)
	missing := filepath.Join(home, "Smolcorp")
	if _, refused := keepHireFolder(missing, false, home); refused == nil || refused.fix != "create" {
		t.Fatalf("kept a folder that isn't there: %+v", refused)
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("made without being asked")
	}
	if kept, refused := keepHireFolder(missing, true, home); refused != nil || kept != filepath.Join(base, "Smolcorp") {
		t.Fatalf("made and kept: %q %+v", kept, refused)
	}
	if info, err := os.Stat(missing); err != nil || !info.IsDir() {
		t.Fatal("not made once accepted")
	}
	mkdirs(t, home, "Library")
	never := []string{filepath.Join(home, "Library", "New"), filepath.Join(home, ".new"), filepath.Join(home, "LIBRARY", "Made")}
	if runtime.GOOS != "windows" {
		// A symlink in home to its Library: the folder would really be made in the Library.
		if err := os.Symlink(filepath.Join(home, "Library"), filepath.Join(home, "lib")); err != nil {
			t.Fatal(err)
		}
		never = append(never, filepath.Join(home, "lib", "New"))
	}
	for _, place := range append([]string{string(filepath.Separator), home}, never...) {
		if _, refused := keepHireFolder(place, true, home); refused == nil || refused.fix != "choose" {
			t.Errorf("%q: %+v", place, refused)
		}
	}
	for _, folder := range append(never, filepath.Join(home, "Library", "Made")) {
		if _, err := os.Stat(folder); err == nil {
			t.Errorf("made %q", folder)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(home, "Library")); len(entries) != 0 {
		t.Errorf("made in the Library: %v", entries)
	}
}

func TestTheHireFolderStateHasTheMacsKeys(t *testing.T) {
	home, base := hireFolderHome(t)
	state := hireFolderState("", nil, home)
	keys := make([]string, 0, len(state))
	for k := range state {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, hireFolderKeys) {
		t.Fatalf("keys %v, want %v", keys, hireFolderKeys)
	}
	want := map[string]any{"folder": "", "suggested": filepath.Join(base, "Smolcorp"), "exists": false, "problem": "", "fix": "", "home": base}
	for k, v := range want {
		if state[k] != v {
			t.Errorf("%s: %v, want %v", k, state[k], v)
		}
	}
	// Set, but gone since: its problem, and Create it.
	gone := filepath.Join(base, "Gone")
	state = hireFolderState(gone, nil, home)
	if state["folder"] != gone || state["fix"] != "create" || state["suggested"] != "" {
		t.Errorf("gone: %v", state)
	}
	// As JSON: sorted keys, as the Mac writes them.
	body, _ := json.Marshal(hireFolderJSON(hireFolderCheck{}, "", home))
	quoted, _ := json.Marshal(base)
	if string(body) != `{"exists":false,"fix":"","folder":"","home":`+string(quoted)+`,"problem":"","suggested":""}` {
		t.Errorf("JSON %s", body)
	}
}

func TestSetupKeepsTheHireFolderFromStartHere(t *testing.T) {
	d := newTestDeck(t)
	base := realPath(homeDir)
	ask := func(method, query string) (int, map[string]any) {
		r := serve(t, d, call{method: method, target: "/setup/hire-folder" + query, host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}})
		var answer map[string]any
		if err := json.Unmarshal(r.Body.Bytes(), &answer); err != nil {
			t.Fatalf("%s %s: %d %s", method, query, r.Code, r.Body)
		}
		return r.Code, answer
	}
	code, answer := ask("GET", "")
	if code != 200 || answer["suggested"] != filepath.Join(base, "Smolcorp") || answer["folder"] != "" {
		t.Fatalf("nothing set: %d %v", code, answer)
	}
	code, answer = ask("GET", "?check="+url.QueryEscape("~/Library"))
	if code != 200 || !strings.Contains(answer["problem"].(string), "own files") {
		t.Errorf("checked as typed: %d %v", code, answer)
	}
	// Not there yet: refused with Create it, and nothing kept.
	code, answer = ask("POST", "?folder="+url.QueryEscape("~/Smolcorp"))
	if code != 400 || answer["fix"] != "create" || d.config.HireFolder != "" {
		t.Fatalf("not there: %d %v, kept %q", code, answer, d.config.HireFolder)
	}
	code, answer = ask("POST", "?folder="+url.QueryEscape("~/Smolcorp")+"&create=1")
	kept := filepath.Join(base, "Smolcorp")
	if code != 200 || answer["folder"] != kept || answer["problem"] != "" || d.config.HireFolder != kept {
		t.Fatalf("made and kept: %d %v, kept %q", code, answer, d.config.HireFolder)
	}
	// What can't be a hires folder is refused, and the one set stays.
	code, answer = ask("POST", "?folder="+url.QueryEscape(homeDir))
	if code != 400 || answer["fix"] != "choose" || d.config.HireFolder != kept {
		t.Errorf("home refused: %d %v, kept %q", code, answer, d.config.HireFolder)
	}
	if code, answer = ask("GET", ""); answer["folder"] != kept || answer["suggested"] != "" || answer["exists"] != true {
		t.Errorf("once set: %d %v", code, answer)
	}
}

// A hire from the setup page, any role, with no folder for your hires (or one that's gone) is refused
// as the GM route refuses it, never started in the home folder; nothing opens and the cooldown is kept.
func TestASetupHireWithNoHiresFolderIsRefusedNotStartedAtHome(t *testing.T) {
	for _, folder := range []string{"", filepath.Join(t.TempDir(), "gone")} {
		for _, role := range []string{"", "reviewer", "gm"} {
			d, h, _ := hiringDeck(t, "")
			hooksOn(t)
			d.config.HireFolder = folder
			status, message := d.hireFromSetup(role)
			if status != http.StatusConflict || message != "Choose the folder for your hires first: it's the first step of Start here." {
				t.Errorf("folder %q, role %q: %d %q", folder, role, status, message)
			}
			if len(h.windows) != 0 || !d.lastHire.IsZero() {
				t.Errorf("folder %q, role %q: the refused hire opened %q or spent the cooldown", folder, role, h.windows)
			}
		}
	}
	// With the folder set, the same hire goes ahead there.
	d, h, folder := hiringDeck(t, "")
	hooksOn(t)
	if status, message := d.hireFromSetup(""); status != http.StatusNoContent || len(h.windows) != 1 {
		t.Fatalf("hire into %s: %d %q, windows %q", folder, status, message, h.windows)
	}
}
