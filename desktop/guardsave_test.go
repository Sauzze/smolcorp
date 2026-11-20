package desktop

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// Setup's guard controls, as the page sends them (setup.html's saveGuard): a
// group switched, a pattern of your own added, one removed. Only the page may
// change the file, and the guard itself then refuses or allows by it. The
// Mac's own answers are in the shared fixture (briefs_fixture_test.go).
func TestSetupsGuardControlsChangeWhatTheGuardRefuses(t *testing.T) {
	d := newTestDeck(t)
	path := filepath.Join(supportDir, hireGuardFile)
	post := func(c call, body string) (int, map[string]any, string) {
		t.Helper()
		c.method, c.target, c.host, c.from, c.body = "POST", "/setup/guard", "localhost:7710", thisComputer, body
		c.headers = mergeHeaders(c.headers, map[string]string{setupHeader: "1"})
		w := serve(t, d, c)
		var answer map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &answer)
		return w.Code, answer, w.Body.String()
	}
	refuses := func(line string) string {
		t.Helper()
		groups, problem := guard.Load(path)
		if problem != "" {
			t.Fatalf("the guard can't read what setup wrote: %s", problem)
		}
		if refusal := guard.Check(line, groups); refusal != nil {
			return refusal.Group
		}
		return ""
	}
	if refuses("git push origin main") != "push" {
		t.Fatal("push isn't refused with no file")
	}

	// Nobody but the page changes it: not a tool with the setup key, not the GM, not a caller without either.
	body := `{"groups":[{"name":"push","on":false}]}`
	for name, c := range map[string]call{
		"a tool": {asTool: true},
		"the GM": {withoutSetupKey: true, headers: map[string]string{gmTokenHeader: d.newGMToken()}},
		"nobody": {withoutSetupKey: true},
	} {
		if code, _, _ := post(c, body); code != http.StatusForbidden && code != http.StatusUnauthorized {
			t.Errorf("%s switched the guard: %d", name, code)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the file was written for a caller that isn't the page: %v", err)
	}

	// The switch: push off, and hires may push; back on, and they may not.
	code, answer, _ := post(call{}, body)
	if groups, _ := answer["groups"].([]any); code != http.StatusOK || len(groups) != 6 || groups[0].(map[string]any)["on"] != false {
		t.Fatalf("push off: %d %v", code, answer)
	}
	if group := refuses("git push origin main"); group != "" {
		t.Errorf("push is off, but the guard refuses it (%s)", group)
	}
	if refuses("vercel deploy") != "deploy" {
		t.Error("switching push off loosened deploy")
	}
	if code, _, _ = post(call{}, `{"groups":[{"name":"push","on":true}]}`); code != http.StatusOK || refuses("git push") != "push" {
		t.Errorf("push back on: %d, refused by %q", code, refuses("git push"))
	}

	// Another command to refuse, then removed: the page sends the whole list each time.
	if refuses("terraform import x y") != "" {
		t.Fatal("terraform import is refused before it's added")
	}
	if code, answer, _ = post(call{}, `{"extra":["terraform import"]}`); code != http.StatusOK || refuses("terraform import x y") != guardExtraGroup {
		t.Errorf("a pattern added: %d %v", code, answer)
	}
	if extra, _ := answer["extra"].([]any); len(extra) != 1 || extra[0] != "terraform import" {
		t.Errorf("the page isn't shown its pattern: %v", answer["extra"])
	}
	if code, answer, _ = post(call{}, `{"extra":[]}`); code != http.StatusOK || refuses("terraform import x y") != "" {
		t.Errorf("a pattern removed: %d %v", code, answer)
	}

	// A refusal is the Mac's sentence, as text, and the file stays as it was.
	before, _ := os.ReadFile(path)
	if code, _, text := post(call{}, `{"extra":["*"]}`); code != http.StatusBadRequest ||
		text != `A pattern starts with the command's name, such as "terraform import"; "*" isn't one.` {
		t.Errorf("a bad pattern: %d %q", code, text)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Error("a refused save changed the file")
	}

	// Written whole and private: readable by you alone, nothing left beside it.
	if info, err := os.Stat(path); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("the guard file: %v %v", info.Mode(), err)
	}
	entries, _ := os.ReadDir(supportDir)
	for _, entry := range entries {
		if strings.Contains(entry.Name(), hireGuardFile) && entry.Name() != hireGuardFile {
			t.Errorf("left beside the guard file: %s", entry.Name())
		}
	}
}

// A group the page doesn't know, someone's own in the file, is kept as it
// was and still refuses what it says.
func TestASaveKeepsTheGroupsThePageDoesntKnow(t *testing.T) {
	path := filepath.Join(t.TempDir(), hireGuardFile)
	mine := `{"groups":[{"name":"mine","on":true,"commands":["make ship"]},{"name":"cloud","on":false,"commands":["aws"]}]}`
	if err := os.WriteFile(path, []byte(mine), 0o600); err != nil {
		t.Fatal(err)
	}
	if answer := saveGuard([]byte(`{"groups":[{"name":"push","on":false}]}`), path, runtime.GOOS); answer.status != http.StatusOK {
		t.Fatalf("%d %s", answer.status, answer.message)
	}
	groups, problem := guard.Load(path)
	if problem != "" {
		t.Fatal(problem)
	}
	if refusal := guard.Check("make ship now", groups); refusal == nil || refusal.Group != "mine" {
		t.Errorf("your own group is lost: %v", refusal)
	}
	if guard.Check("aws s3 ls", groups) != nil || guard.Check("gcloud run deploy", groups) != nil {
		t.Error("cloud, off in the file with its own commands, came back on")
	}
	if guard.Check("git push", groups) != nil || guard.Check("ssh host", groups) == nil {
		t.Error("only push was switched")
	}
}

// A library brief you edited and saved with Windows' line ends reads as one with
// plain ones: its front matter and slots are still seen.
func TestALibraryBriefSavedWithWindowsLineEndsStillReads(t *testing.T) {
	shipped := briefLibrary{files: bundledBriefs()}
	text, ok := shipped.read("tester.md")
	if !ok {
		t.Fatal("no tester brief")
	}
	edited := briefLibrary{files: fstest.MapFS{"tester.md": {Data: []byte(strings.ReplaceAll(text, "\n", "\r\n"))}}}
	got, want := edited.entries(), shipped.entries()
	if len(got) != 1 || got[0].ID != "tester" || got[0].Name != "Tester" || got[0].Summary == "" ||
		len(got[0].Questions) == 0 || mustJSON(t, got[0]) != mustJSON(t, want[1]) {
		t.Errorf("read with CRLF: %+v\nwith LF: %+v", got, want[1])
	}
}

// "Back to how it came" compares a role with the one that came with the app,
// as JSON: a shipped role you haven't changed is sent the same both ways, its
// brief included, so the button is off until you change it.
func TestAShippedRoleYouHaventChangedIsSentAsItCame(t *testing.T) {
	d := newTestDeck(t)
	w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer})
	var state struct {
		Roles struct {
			Roles   []json.RawMessage `json:"roles"`
			Shipped []json.RawMessage `json:"shipped"`
		} `json:"roles"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &state) != nil || len(state.Roles.Shipped) != len(shippedRoles) {
		t.Fatalf("setup state: %d", w.Code)
	}
	kept := map[string]string{}
	for _, raw := range state.Roles.Roles {
		var role Role
		_ = json.Unmarshal(raw, &role)
		kept[role.ID] = string(raw)
	}
	same := 0
	for _, raw := range state.Roles.Shipped {
		var role Role
		_ = json.Unmarshal(raw, &role)
		if role.Brief == nil {
			t.Errorf("%s is sent without its brief", role.ID)
		}
		if saved, ok := kept[role.ID]; ok {
			same++
			if saved != string(raw) {
				t.Errorf("%s, unchanged, differs from how it came:\n%s\n%s", role.ID, saved, raw)
			}
		}
	}
	if same == 0 {
		t.Fatal("no shipped role among the roles kept")
	}
}

// What setup writes for "cloud off, one pattern of your own" is the Mac's file,
// byte for byte (GuardFileTests.swift writes the fixture; the guard reads it),
// at mode 0600 (on Windows, which has no such mode, the support folder in your
// profile keeps it yours: see modeIs); and no patterns is written as the Mac
// writes an empty list.
func TestSavingTheGuardWritesTheMacsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), hireGuardFile)
	if answer := saveGuard([]byte(`{"groups":[{"name":"cloud","on":false}],"extra":["terraform import"]}`), path, runtime.GOOS); answer.status != http.StatusOK {
		t.Fatalf("%d %s", answer.status, answer.message)
	}
	written, _ := os.ReadFile(path)
	fixture, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "frontdesk", "hire-guard.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written, fixture) {
		t.Errorf("the file isn't the Mac's:\n%s", written)
	}
	if info, _ := os.Stat(path); !modeIs(info, 0o600) {
		t.Errorf("mode %v", info.Mode().Perm())
	}
	if answer := saveGuard([]byte(`{"extra":[]}`), path, runtime.GOOS); answer.status != http.StatusOK {
		t.Fatal(answer.status)
	}
	if written, _ := os.ReadFile(path); !strings.Contains(string(written), "\"commands\" : [\n\n      ],\n      \"name\" : \"extra\"") {
		t.Errorf("no patterns isn't written as the Mac writes it:\n%s", written)
	}
}
