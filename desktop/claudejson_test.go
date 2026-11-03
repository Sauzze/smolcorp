package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	editorTestKey  = "fake-key-0123456789ABCDEFGHIJ"
	editorTestTail = "0123456789ABCDEFGHIJ"
)

const claudeJSONBefore = `{
  "numStartups": 12,
  "bigNumber": 12345678901234567890,
  "theme": "dark",
  "customApiKeyResponses": {"approved": ["old-key-tail-0123456"], "rejected": ["nope"]},
  "projects": {"/other": {"hasTrustDialogAccepted": false, "allowedTools": []}}
}`

func tailOf(s string) *string { return &s }

func newClaudeJSON(t *testing.T, text string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), ".claude.json")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

func readClaudeJSON(t *testing.T, file string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, data)
	}
	return out
}

func TestApprovingAKeyAddsItsLast20CharactersFirstAndChangesNoOtherByte(t *testing.T) {
	isolate(t)
	if keyTail("  "+editorTestKey+"\n") != editorTestTail || keyTail("short") != "short" || keyTail(" \n") != "" ||
		keyTail("k-é"+strings.Repeat("ü", 20)) != strings.Repeat("ü", 20) {
		t.Error("the tail is the last 20 characters of the trimmed key")
	}
	file := newClaudeJSON(t, claudeJSONBefore)
	if err := preTrust("/other", file, keyApproval(editorTestKey)); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(file)
	// Byte for byte: the before text with the tail first in the list and /other's flag true, nothing else.
	want := strings.Replace(strings.Replace(claudeJSONBefore, `"approved": ["old-key-tail-0123456"`, `"approved": ["`+editorTestTail+`","old-key-tail-0123456"`, 1),
		`"hasTrustDialogAccepted": false`, `"hasTrustDialogAccepted": true`, 1)
	if string(data) != want {
		t.Errorf("got:\n%s\nwant:\n%s", data, want)
	}
	if strings.Contains(string(data), editorTestKey) {
		t.Error("the whole key must not be written")
	}
	if info, _ := os.Stat(file); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", info.Mode().Perm())
	}
	// Again: nothing more is added, nothing changes.
	if err := preTrust("/other", file, keyApproval(editorTestKey)); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(file); string(again) != want {
		t.Errorf("a second time changed the file:\n%s", again)
	}
}

func TestApprovingInAFileWithNoApprovedListMakesOne(t *testing.T) {
	isolate(t)
	for _, text := range []string{`{"theme":"dark"}`, `{"theme":"dark","customApiKeyResponses":{"rejected":["x"]}}`, `{"theme":"dark","customApiKeyResponses":{"approved":[]}}`} {
		file := newClaudeJSON(t, text)
		if err := preTrust("/w", file, tailOf(editorTestTail)); err != nil {
			t.Fatal(err)
		}
		got := readClaudeJSON(t, file)
		if got["theme"] != "dark" || !reflect.DeepEqual(got["customApiKeyResponses"].(map[string]any)["approved"], []any{editorTestTail}) {
			t.Errorf("%s -> %v", text, got)
		}
		if !strings.Contains(string(mustRead(t, file)), `"theme":"dark"`) {
			t.Errorf("the compact layout was changed: %s", mustRead(t, file))
		}
	}
}

func mustRead(t *testing.T, file string) []byte {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheEditRefusesWhatItDoesNotUnderstandAndChangesNothing(t *testing.T) {
	isolate(t)
	for name, text := range map[string]string{
		"empty": "", "not json": "nope", "array": "[]", "two documents": `{} {}`, "cut short": `{"a":`,
		"responses a list": `{"customApiKeyResponses":[]}`, "approved an object": `{"customApiKeyResponses":{"approved":{}}}`,
		"approved null": `{"customApiKeyResponses":{"approved":null}}`, "projects a list": `{"projects":[]}`,
		"project a string": `{"projects":{"/w":"x"}}`,
	} {
		file := newClaudeJSON(t, text)
		if err := preTrust("/w", file, tailOf(editorTestTail)); err == nil {
			t.Errorf("%s: should be refused", name)
		}
		if data, _ := os.ReadFile(file); string(data) != text {
			t.Errorf("%s: file changed to %q", name, data)
		}
	}
	if err := preTrust("/w", filepath.Join(t.TempDir(), "missing.json"), nil); err == nil {
		t.Error("a missing file is refused, not made")
	}
}

func TestTheEditWritesThroughASymlink(t *testing.T) {
	isolate(t)
	target := newClaudeJSON(t, `{"theme":"dark"}`)
	link := filepath.Join(t.TempDir(), ".claude.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("no symlinks here")
	}
	if err := preTrust("/w", link, tailOf(editorTestTail)); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Lstat(link); info.Mode()&os.ModeSymlink == 0 {
		t.Error("the link was replaced by a file")
	}
	got := readClaudeJSON(t, target)
	if !reflect.DeepEqual(got["customApiKeyResponses"].(map[string]any)["approved"], []any{editorTestTail}) {
		t.Errorf("target: %v", got)
	}
}

func TestPreTrustSetsOnlyTheWorktreesTrust(t *testing.T) {
	home := isolate(t)
	if claudeJSON("") != filepath.Join(home, ".claude.json") || claudeJSON("/acct") != "/acct/.claude.json" {
		t.Error("an account keeps its .claude.json in its own folder, ~/.claude's in the home")
	}
	file := newClaudeJSON(t, claudeJSONBefore)
	if err := preTrust("/work/tree", file, nil); err != nil {
		t.Fatal(err)
	}
	got, want := readClaudeJSON(t, file), readClaudeJSON(t, newClaudeJSON(t, claudeJSONBefore))
	want["projects"].(map[string]any)["/work/tree"] = map[string]any{"hasTrustDialogAccepted": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	if data := mustRead(t, file); !strings.Contains(string(data), "12345678901234567890") || strings.Contains(string(data), "approved\": [\""+editorTestTail) {
		t.Errorf("numbers stay as written, and a trust-only edit approves no key:\n%s", data)
	}
	if info, _ := os.Stat(file); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", info.Mode().Perm())
	}
}

func TestAStaleTempFileIsLeftAloneAndTheResultIsPrivate(t *testing.T) {
	isolate(t)
	file := newClaudeJSON(t, `{"theme":"dark"}`)
	if err := os.WriteFile(file+".tmp", []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file+".tmp", 0o644); err != nil { // whatever the umask made it
		t.Fatal(err)
	}
	if err := preTrust("/w", file, nil); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(file); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", info.Mode().Perm())
	}
	if !isTrusted("/w", mustRead(t, file)) {
		t.Errorf("not trusted:\n%s", mustRead(t, file))
	}
	if got := mustRead(t, file+".tmp"); string(got) != "stale" {
		t.Errorf("the stale file was written through: %q", got)
	}
	if left, _ := filepath.Glob(file + ".*.tmp"); len(left) != 0 {
		t.Errorf("temporary files left behind: %v", left)
	}
}

func TestAStaleTempSymlinkIsNotWrittenThrough(t *testing.T) {
	isolate(t)
	file := newClaudeJSON(t, `{"theme":"dark"}`)
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(victim, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(victim, 0o644); err != nil { // whatever the umask made it
		t.Fatal(err)
	}
	if err := os.Symlink(victim, file+".tmp"); err != nil {
		t.Skip("no symlinks here")
	}
	if err := preTrust("/w", file, tailOf(editorTestTail)); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, victim); string(got) != "mine" {
		t.Errorf("the link's target was written through: %q", got)
	}
	if info, _ := os.Stat(victim); info.Mode().Perm() != 0o644 {
		t.Errorf("the link's target's mode changed to %v", info.Mode().Perm())
	}
	if info, _ := os.Lstat(file); info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 {
		t.Errorf("the file should be a private regular file, mode %v", info.Mode())
	}
	if !isApproved(editorTestTail, mustRead(t, file)) {
		t.Errorf("the edit is missing:\n%s", mustRead(t, file))
	}
}

func TestAFileAlreadyRightEndsPrivate(t *testing.T) {
	isolate(t)
	file := newClaudeJSON(t, `{"projects":{"/w":{"hasTrustDialogAccepted":true}},"customApiKeyResponses":{"approved":["`+editorTestTail+`"]}}`)
	before := mustRead(t, file)
	if err := os.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := preTrust("/w", file, tailOf(editorTestTail)); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(file); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", info.Mode().Perm())
	}
	if string(mustRead(t, file)) != string(before) {
		t.Error("a file already right was changed")
	}
}

func TestAnAPIKeyHireWithNoKeyIsRefusedAndNothingChanges(t *testing.T) {
	isolate(t)
	for _, key := range []string{"", " \n"} {
		file := newClaudeJSON(t, claudeJSONBefore)
		if err := preTrust("/w", file, keyApproval(key)); err == nil {
			t.Errorf("key %q: should be refused", key)
		}
		if string(mustRead(t, file)) != claudeJSONBefore {
			t.Errorf("key %q: the file changed", key)
		}
	}
	file := newClaudeJSON(t, claudeJSONBefore)
	if err := preTrust("/w", file, nil); err != nil || !isTrusted("/w", mustRead(t, file)) {
		t.Errorf("a subscription hire (no key asked for) is trusted: %v", err)
	}
}

func TestParallelPreTrustsOnOneFileAllLand(t *testing.T) {
	isolate(t)
	file := newClaudeJSON(t, claudeJSONBefore)
	folders := []string{"/a", "/b", "/c", "/d"}
	errs := make(chan error, len(folders)*10)
	var group sync.WaitGroup
	for _, folder := range folders {
		group.Add(1)
		go func() {
			defer group.Done()
			for round := range 10 {
				errs <- preTrust(folder+strconv.Itoa(round), file, tailOf(editorTestTail))
			}
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	data := mustRead(t, file)
	for _, folder := range folders {
		for round := range 10 {
			if !isTrusted(folder+strconv.Itoa(round), data) {
				t.Errorf("%s%d is not trusted", folder, round)
			}
		}
	}
}
