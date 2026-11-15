package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// SETTINGSFILE1 (SIERRA): a desk's settings are a file in its own folder, sealed by its hash, and its start names the
// file alone. The guard is given the file and its seal before every call and after it; the GM's memory switch
// (MEM4b) and SA-X8's rules are in the file.
func TestADesksSettingsAreASealedFileItsStartNamesAlone(t *testing.T) {
	r := newHireSettingsRig(t)
	for _, profile := range []guardProfile{guardHire, guardGM} {
		written, err := r.writeFile("abc123", func(o *hireSettingsOptions) { o.Profile, o.OwnClaude = profile, true })
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(r.support, "hires", "abc123", hireSettingsFile); written.Path != want || written.refusal() != nil {
			t.Fatalf("%v: written to %s (%v), want %s", profile, written.Path, written.refusal(), want)
		}
		if info, err := os.Lstat(written.Path); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0o400 {
			t.Errorf("%v: the file's mode %v (%v)", profile, info.Mode(), err)
		}
		text := fileSettings(written.Path)
		var settings struct {
			Hooks map[string][]struct {
				Matcher string
				Hooks   []struct {
					Command string
					Args    []string
				}
			}
			Env               map[string]string
			AutoMemoryEnabled *bool
			PlansDirectory    string
		}
		if err := json.Unmarshal([]byte(text), &settings); err != nil {
			t.Fatal(err)
		}
		seal := []string{"--settings", written.Path, "--sum", written.Sum}
		pre := settings.Hooks["PreToolUse"][0].Hooks[0].Args
		if !slices.Equal(pre[len(pre)-4:], seal) || pre[0] != "guard" {
			t.Errorf("%v: the guard's words before a call: %q", profile, pre)
		}
		for _, event := range []string{"PostToolUse", "PostToolUseFailure"} {
			if groups := settings.Hooks[event]; len(groups) != 1 || groups[0].Matcher != "*" ||
				!slices.Equal(groups[0].Hooks[0].Args, append([]string{"guard", "--after"}, seal...)) || groups[0].Hooks[0].Command != r.cli {
				t.Errorf("%v: the guard after a call (%s): %+v", profile, event, groups)
			}
		}
		// SA-X8's rules and plans, MEM4b's memory switch for the GM alone.
		if !strings.Contains(text, "/hires/*/"+deskClaudeName+"/**)") || settings.PlansDirectory != deskPlansFor("abc123") {
			t.Errorf("%v: SA-X8's rules or plans aren't in the file", profile)
		}
		memoryOff := settings.AutoMemoryEnabled != nil && !*settings.AutoMemoryEnabled && settings.Env[claudeMemoryEnv] == "1"
		if memoryOff != (profile == guardGM) || profile == guardHire && strings.Contains(text, claudeMemoryEnv) {
			t.Errorf("%v: Claude Code's memory switch in the file: %v", profile, memoryOff)
		}
		// The start names the file, and nothing else of the settings.
		plan, err := hireStart{Role: testHireRole(), Message: "go", DefaultAccount: true, Settings: written}.plan()
		if err != nil || plan.Words[0] != "--settings="+written.Path || plan.Settings != written || strings.Contains(strings.Join(plan.Words, " "), "{") {
			t.Errorf("%v: the start's words %q (%v)", profile, plan.Words, err)
		}
	}
	// Settings in any other form never reach a start.
	for _, settings := range []sealedSettings{{Path: `{"permissions":{}}`, Sum: strings.Repeat("a", 64)}, {Path: "/s/settings.json"}, {Sum: strings.Repeat("a", 64)}} {
		if _, err := (hireStart{Role: testHireRole(), Message: "go", DefaultAccount: true, Settings: settings}).plan(); err == nil {
			t.Errorf("a start took %+v", settings)
		}
	}
}

// The seal can fail, whatever way the file is changed: in place, replaced by a rename, rewritten under a seal of its
// own, made a link, given a second name, or taken away. Each is refused in words; the file as written isn't.
func TestADesksSettingsFileChangedInAnyWayIsRefused(t *testing.T) {
	r := newHireSettingsRig(t)
	write := func() sealedSettings {
		written, err := r.writeFile("abc123", nil)
		if err != nil {
			t.Fatal(err)
		}
		return written
	}
	loosened := func(text string) string { return strings.Replace(text, `"Bash(git push:*)",`, "", 1) }
	for _, c := range []struct {
		name   string
		change func(t *testing.T, s sealedSettings)
	}{
		{"edited in place", func(t *testing.T, s sealedSettings) {
			must0(t, os.Chmod(s.Path, 0o600))
			must0(t, os.WriteFile(s.Path, []byte(loosened(fileSettings(s.Path))+"\n"), 0o600))
		}},
		{"replaced by a rename", func(t *testing.T, s sealedSettings) {
			must0(t, os.WriteFile(s.Path+".new", []byte(loosened(fileSettings(s.Path))+"\n"), 0o600))
			must0(t, os.Rename(s.Path+".new", s.Path))
		}},
		{"rewritten under a seal of its own", func(t *testing.T, s sealedSettings) {
			text := strings.ReplaceAll(loosened(fileSettings(s.Path)), s.Sum, guard.SettingsSumBlank)
			sum, _ := guard.SettingsSum([]byte(text))
			must0(t, writeSettingsFile(s.Path, []byte(strings.ReplaceAll(text, guard.SettingsSumBlank, sum))))
		}},
		{"a link in its place", func(t *testing.T, s sealedSettings) {
			if runtime.GOOS == "windows" {
				t.Skip("a link needs a privilege on Windows: the reader refuses a reparse point (internal/guard)")
			}
			must0(t, os.Rename(s.Path, s.Path+".aside"))
			must0(t, os.Symlink(s.Path+".aside", s.Path))
		}},
		{"a second name", func(t *testing.T, s sealedSettings) {
			must0(t, os.Link(s.Path, filepath.Join(filepath.Dir(s.Path), "NOTES.md")))
		}},
		{"taken away", func(t *testing.T, s sealedSettings) { must0(t, os.Remove(s.Path)) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			written := write()
			if refusal := written.refusal(); refusal != nil {
				t.Fatalf("the file as written: %v", refusal)
			}
			c.change(t, written)
			refusal := written.refusal()
			if refusal == nil || refusal.Status != http.StatusConflict || !strings.Contains(refusal.Message, "isn't the one Smolcorp wrote") {
				t.Errorf("%s: %v", c.name, refusal)
			}
			_ = os.Remove(filepath.Join(filepath.Dir(written.Path), "NOTES.md"))
		})
	}
	written := write()
	for _, s := range []sealedSettings{{Path: written.Path, Sum: guard.SettingsSumBlank}, {Path: written.Path}, {}} {
		if s.refusal() == nil {
			t.Errorf("a start without the file's seal took %+v", s)
		}
	}
}

func must0(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// A settings file that can't be written refuses the start, in words: there is no other form to fall back on.
func TestADesksSettingsFileThatCantBeWrittenStartsNothing(t *testing.T) {
	r := newHireSettingsRig(t)
	must0(t, os.WriteFile(filepath.Join(r.support, "hires", "abc999"), nil, 0o600)) // its folder's place taken by a file
	written, err := r.writeFile("abc999", nil)
	var refusal *hireRefusal
	if !errors.As(err, &refusal) || refusal.Status != http.StatusInternalServerError || written != (sealedSettings{}) ||
		!strings.HasPrefix(refusal.Message, "Smolcorp couldn't write this desk's settings file, where the desk can't change it (") {
		t.Fatalf("%+v %v", written, err)
	}
}

// The Windows launcher's route checks the file before it gives the start (and the start is spent); the launcher says
// why in the hire's window. Untouched, the route gives its name and seal, and nothing of the settings.
func TestTheLaunchersRouteRefusesAChangedSettingsFile(t *testing.T) {
	d := newTestDeck(t)
	r := newHireSettingsRig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) { d.startEnvRoute(w, q.URL.Query()) }))
	t.Cleanup(server.Close)
	for _, changed := range []bool{false, true} {
		written, err := r.writeFile("abc123", nil)
		if err != nil {
			t.Fatal(err)
		}
		token := strings.ToUpper(newID())
		d.mu.Lock()
		d.hires[token] = &pendingHire{at: time.Now(), hireID: "abc123", env: &hireStartEnv{Settings: written.Path, SettingsSum: written.Sum}}
		d.mu.Unlock()
		if changed {
			must0(t, os.Link(written.Path, written.Path+".md"))
			t.Cleanup(func() { os.Remove(written.Path + ".md") })
		}
		env, err := fetchHireStartEnv(server.Client(), server.URL, "fake-hook-key", token, "abc123")
		if !changed && (err != nil || env.Settings != written.Path || env.SettingsSum != written.Sum) {
			t.Errorf("untouched: %+v %v", env, err)
		}
		if changed && (err == nil || !strings.Contains(err.Error(), "settings file isn't the one Smolcorp wrote")) {
			t.Errorf("changed: %+v %v", env, err)
		}
		if _, again := fetchHireStartEnv(server.Client(), server.URL, "fake-hook-key", token, "abc123"); again == nil {
			t.Error("the start was given twice")
		}
	}
}
