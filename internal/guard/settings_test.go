package guard

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// sealedForTests writes a stand-in desk settings file in dir, sealed as the core seals it: its path and its seal.
func sealedForTests(t *testing.T, dir string) (string, string) {
	t.Helper()
	text := `{"hooks":{"PreToolUse":["--sum","` + SettingsSumBlank + `"],"PostToolUse":["--after","--sum","` + SettingsSumBlank + `"]}}` + "\n"
	sum, ok := SettingsSum([]byte(text))
	if !ok {
		t.Fatal("no blank")
	}
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(text, SettingsSumBlank, sum)), 0o400); err != nil {
		t.Fatal(err)
	}
	return path, sum
}

// guardSays runs the guard with args on a Bash call of `ls`: its answer, read, and nil for silence (allow).
func guardSays(t *testing.T, args []string) map[string]any {
	t.Helper()
	call, _ := json.Marshal(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "ls"}, "tool_response": strings.Repeat("x", 256<<10)})
	var out bytes.Buffer
	if code := Run(args, bytes.NewReader(call), &out, &bytes.Buffer{}); code != exitOK {
		t.Fatalf("%q exited %d", args, code)
	}
	if out.Len() == 0 {
		return nil
	}
	var answer map[string]any
	if err := json.Unmarshal(out.Bytes(), &answer); err != nil {
		t.Fatalf("%q: %s", args, out.Bytes())
	}
	return answer
}

// SETTINGSFILE1: the guard checks the desk's settings file against its seal before each call (PreToolUse) and after
// it (PostToolUse, PostToolUseFailure). The file as written lets `ls` run; a file changed in any way since, or a seal
// missing from the guard's own words, ends the session in words: the call denied before, blocked after.
func TestTheGuardChecksTheDesksSettingsFileOnBothSidesOfACall(t *testing.T) {
	dir := t.TempDir()
	rules := filepath.Join(dir, fileName) // none there: the default groups
	path, sum := sealedForTests(t, dir)
	before := []string{"--groups", GroupsArgument(DefaultGroups()), "--rules", rules, "--settings", path, "--sum", sum}
	after := []string{"--after", "--settings", path, "--sum", sum}
	if says := guardSays(t, before); says != nil {
		t.Fatalf("before a call, the file as written: %v", says)
	}
	if says := guardSays(t, after); says != nil {
		t.Fatalf("after a call, the file as written: %v", says)
	}
	stopped := func(name string, args []string, isAfter bool) {
		t.Helper()
		says := guardSays(t, args)
		reason, _ := says["stopReason"].(string)
		if says["continue"] != false || !strings.HasPrefix(reason, "Stopped by Smolcorp's hire guard: this desk's settings file isn't the one") {
			t.Errorf("%s: %v", name, says)
			return
		}
		if isAfter && (says["decision"] != "block" || says["reason"] != reason) {
			t.Errorf("%s, after: %v", name, says)
		}
		if decision, _ := says["hookSpecificOutput"].(map[string]any); !isAfter && (decision["permissionDecision"] != "deny" || decision["hookEventName"] != "PreToolUse") {
			t.Errorf("%s, before: %v", name, says)
		}
	}
	stopped("no seal", before[:6], false)
	stopped("no file", append(before[:4:4], "--sum", sum), false)
	stopped("after, with nothing", []string{"--after"}, true)
	stopped("a seal that isn't one", append(before[:7:7], SettingsSumBlank), false)

	changes := map[string]func(){
		"a second name": func() { must(t, os.Link(path, filepath.Join(dir, "NOTES.md"))) },
		"edited":        func() { must(t, os.Chmod(path, 0o600)); must(t, appendTo(path, " ")) },
		"replaced": func() { // by another file, under a seal of its own
			text := `{"hooks":{},"sum":"` + SettingsSumBlank + `"}`
			own, _ := SettingsSum([]byte(text))
			must(t, os.WriteFile(path+".new", []byte(strings.ReplaceAll(text, SettingsSumBlank, own)), 0o400))
			// Windows won't rename over a read-only file (0o400 is its read-only attribute), as it won't write one
			// ("edited"): the hire clears it first.
			must(t, os.Chmod(path, 0o600))
			must(t, os.Rename(path+".new", path))
		},
		"taken away": func() { must(t, os.Remove(path)) },
	}
	if runtime.GOOS != "windows" {
		changes["a link"] = func() { must(t, os.Rename(path, path+".aside")); must(t, os.Symlink(path+".aside", path)) }
	}
	for name, change := range changes {
		for _, f := range []string{path, path + ".aside", filepath.Join(dir, "NOTES.md")} {
			_ = os.Remove(f)
		}
		path, sum = sealedForTests(t, dir)
		before[7], after[4] = sum, sum
		if guardSays(t, before) != nil || guardSays(t, after) != nil {
			t.Fatalf("%s: the file as written was refused", name)
		}
		change()
		stopped(name, before, false)
		stopped(name, after, true)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func appendTo(path, text string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	_, err = f.WriteString(text)
	if closed := f.Close(); err == nil {
		err = closed
	}
	return err
}
