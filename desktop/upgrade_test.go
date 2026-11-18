package desktop

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// UPGRADE1: 1.26 started over a working 1.24.2 office, as each system's 1.24.2 left it. The files below are in
// 1.24.2's own shapes (its desktop/*.go for Linux and Windows, its Sources/AgentDeck for the Mac's Swift engine):
// the team with a hire on an account, its contacts and send key, a phone (and on Windows the window's own device),
// the office's look and floors, the security settings, the guard. CI's upgrade job does the same from the real
// 1.24.2 release on Linux and Windows (scripts/upgrade); these rows are its in-process twin, and the Mac's only one.
// Fake keys only.

// office1242 writes a 1.24.2 office into the support folder, as the Mac's Swift engine (mac) or the Go app wrote
// it, and returns its project folder. strict: every security setting at its strictest; else as an owner who opened
// them up had them.
func office1242(t *testing.T, mac, strict bool) string {
	t.Helper()
	project := filepath.Join(homeDir, "project")
	account := filepath.Join(homeDir, ".claude-work")
	for _, dir := range []string{project, account} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(name string, value any) {
		data, ok := value.([]byte)
		if !ok {
			var err error
			if data, err = json.Marshal(value); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(supportDir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// What 1.24.2 writes at every start: its setup key and hook key (the Mac's engine, the hook's header too).
	write("setup-key", []byte("fake-1242-setup-key\n"))
	write("hook-key", []byte("fake-1242-hook-key\n"))
	// A phone, and on Windows the window's own paired device, which isn't a phone. The Mac's engine wrote its
	// dates as numbers (seconds since 2001), the Go app as text.
	paired := any("2026-10-01T10:00:00Z")
	if mac {
		paired = 781000000.0
		write("hook-header", []byte("X-Smolcorp-Hook-Key: fake-1242-hook-key\n"))
		write("mac-peers.json", []map[string]any{{"id": "M1", "name": "Studio", "keyHash": hashKey("fake-1242-peer-1")},
			{"id": "M2", "name": "Laptop", "keyHash": hashKey("fake-1242-peer-2")}})
	}
	write("phones.json", []map[string]any{
		{"id": "P1", "name": "Pixel", "pairedAt": paired, "lastSeen": paired, "keyHash": hashKey("fake-1242-phone")},
		{"id": "W1", "name": windowDeviceName, "pairedAt": paired, "lastSeen": paired, "keyHash": hashKey("fake-1242-window")},
	})
	// The team: two hires on the "work" account who may write to each other, the GM, and one fired. Started in
	// milliseconds, as both engines write it.
	hire := func(id, name, nickname, role, state string) map[string]any {
		return map[string]any{"id": id, "name": name, "nickname": nickname, "role": role, "folder": project, "branch": "hire/" + id,
			"worktree": filepath.Join(project, ".worktrees", id), "repo": project, "tmuxSocket": filepath.Join(homeDir, "tmux-"+id),
			"account": "work", "model": "", "ports": "", "brief": "", "started": 1761900000000.0, "state": state, "notes": ""}
	}
	write(registryName, map[string]any{"version": 1, "hires": []any{
		hire("h1", "Builder", "Ada", "builder", "working"), hire("h2", "Reviewer", "Bo", "reviewer", "working"),
		hire("gm1", "General Manager", "", gmRole, "working"), hire("h3", "Tester", "Cy", "tester", "fired"),
	}, "contacts": []map[string]string{{"from": "h1", "to": "h2"}, {"from": "h2", "to": "h1"}},
		"keys": map[string]string{"h1": hashKey("fake-1242-send-key")}})
	config := map[string]any{
		"hasCompletedSetup": true, "colours": map[string]string{"body": "#102030"}, "pins": []any{project},
		"floors": []map[string]string{{"name": "Lobby", "preset": ""}, {"name": "Lab", "preset": "zen"}}, "listen": "tailscale",
		"hiringEnabled": !strict, "managerEnabled": !strict,
		"webhook": map[string]any{"enabled": !strict, "url": "http://127.0.0.1:9/hook", "secret": "fake-1242-webhook", "outside": false, "events": []string{}},
	}
	if !strict {
		config["listen"] = "all"
	}
	if mac {
		// The Swift engine's own fields: the accounts, a GM started by hand, keys to other apps, the mail socket.
		// It saves the phone's text switch in every file (Config.swift, on unless set off); the Go app had none.
		delete(config, "listen")
		config["phoneMicEnabled"] = true
		config["claudeProfiles"] = []map[string]string{{"name": "work", "dir": account}}
		config["gmByHand"], config["keystrokesAllowed"], config["mailInboxSocket"] = !strict, !strict, !strict
		if !strict {
			config["keystrokeApps"] = []string{"com.example.notes"}
		}
	}
	write("config.json", config)
	// The guard: deploy switched off, and a pattern of the owner's added. Strict: every group on.
	write(hireGuardFile, map[string]any{"groups": []map[string]any{
		{"name": "deploy", "on": strict, "commands": []string{}}, {"name": guardExtraGroup, "on": true, "commands": []string{"terraform apply"}},
	}})
	return project
}

// settingsByName is v's fields by their names in config.json.
func settingsByName(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

// The update's line, and that it says each of want, in order, and none of unwanted.
func updateLineSays(t *testing.T, d *Deck, now time.Time, status string, want []string, unwanted ...string) check {
	t.Helper()
	c := d.updateCheck(now)
	if c == nil {
		t.Fatal("Start here says nothing of the update")
	}
	if c.ID != "update" || c.Title != updateTitle || c.Status != status {
		t.Errorf("the line: %+v", *c)
	}
	at := 0
	for _, words := range want {
		i := strings.Index(c.Detail[at:], words)
		if i < 0 {
			t.Errorf("Setup doesn't say %q (after %d) in %q", words, at, c.Detail)
			continue
		}
		at += i + len(words)
	}
	for _, words := range unwanted {
		if strings.Contains(c.Detail, words) {
			t.Errorf("Setup says %q in %q", words, c.Detail)
		}
	}
	return *c
}

// Off a Mac (the Linux and Windows app): the update keeps the team, its accounts and contacts, the office and the
// settings as 1.24.2 had them, lets every key and the phone go, and Start here says so, with the hires from 1.24
// to hire again by name, until they are. The line outlasts a restart, and goes two weeks after the last of them.
func TestTheUpdateFrom1242KeepsTheOfficeAndSaysWhatReset(t *testing.T) {
	isolate(t)
	project := office1242(t, false, false)
	started := time.Now()
	d := recordsDeck(t)

	// Kept.
	file := readRegistry(registryPath(), false)
	for _, id := range []string{"h1", "h2", "gm1"} {
		i := slices.IndexFunc(file.hires, func(h Hire) bool { return h.ID == id })
		if i < 0 || file.hires[i].Account != "work" || file.hires[i].Folder != project || !isOpen(file.hires[i]) {
			t.Errorf("hire %s wasn't kept as it was", id)
		}
	}
	if !file.mayWrite("h1", "h2") || !file.mayWrite("h2", "h1") {
		t.Error("who may write to whom wasn't kept")
	}
	d.mu.Lock()
	config := d.config
	kept := config.Colours["body"] == "#102030" && config.Floors[0].Name == "Lobby" && config.Floors[1].Name == "Lab" && config.Floors[1].Preset == "zen" &&
		config.Pins[0] != nil && *config.Pins[0] == project && config.Listen == "all" && config.HiringEnabled && config.ManagerEnabled &&
		config.Webhook.Enabled && config.Webhook.URL == "http://127.0.0.1:9/hook"
	d.mu.Unlock()
	if !kept {
		t.Errorf("the office or the settings weren't kept: %+v", settingsByName(t, config))
	}
	// Reset: the keys, and the phone.
	if d.access.Phones.WithKey("fake-1242-phone") != nil || isHireKey(registryPath(), "fake-1242-send-key") || d.access.HookKey == "fake-1242-hook-key" {
		t.Error("a key from 1.24.2 still works")
	}

	// A desk in T3 Code made since the update has no hook key either (hookkeys.go): it isn't from 1.24.
	if err := updateRegistry(func(f *registryFile) {
		f.hires = append(f.hires, Hire{ID: "t3a", Name: "Designer", Nickname: "Dee", Folder: project, State: "working", Engine: t3Engine,
			Started: float64(time.Now().UnixMilli())})
	}); err != nil {
		t.Fatal(err)
	}
	rehire := "Re-hire your hires from 1.24 (Ada, Bo and the General Manager): they still run as they started"
	want := []string{updateKept, updateKeys, updatePhone, rehire, "Fire each one and hire it again."}
	line := updateLineSays(t, d, time.Now(), checkTodo, want, "Cy", "Dee", updateByHand, "Mac", "fell back")
	// As `smolcorp setup` prints it (the self-test's own line is checked under the Mac app below).
	var printed bytes.Buffer
	printChecklist(&printed, []check{line})
	if !strings.Contains(printed.String(), "• "+updateTitle+"\n    "+updateKept) || !strings.Contains(printed.String(), rehire) {
		t.Errorf("smolcorp setup doesn't say it:\n%s", printed.String())
	}
	// Typing over Wi-Fi stays off, as a new setup has it (1.24.2's Go app had no phone text at all), though this
	// office let in every interface: a phone paired again types over the Wi-Fi only once the owner turns it on, and
	// Setup says so on a line of its own until then.
	d.mu.Lock()
	wifi := d.config.PhoneTextWiFi
	d.mu.Unlock()
	if wifi {
		t.Error("the update turned Typing over Wi-Fi on")
	}
	phone := pair(t, d)
	if got := serve(t, d, call{method: "POST", target: "/dictate?session=nobody&t=" + phone, host: deckOnWiFi, from: aPhone, body: `{"text":"hello"}`}); got.Code != http.StatusForbidden || !strings.Contains(got.Body.String(), "Typing over Wi-Fi is off") {
		t.Errorf("a phone paired again typed over the Wi-Fi after the update: %d %s, want 403", got.Code, got.Body)
	}
	wifiLine := check{ID: "update-wifityping", Status: checkInfo, Title: updateWiFiTitle, Detail: updateWiFiWords}
	if checks := d.updateChecks(time.Now()); len(checks) != 2 || checks[0] != line || checks[1] != wifiLine {
		t.Errorf("Start here doesn't offer Typing over Wi-Fi after the update: %+v", checks)
	}
	if !strings.Contains(updateWiFiWords, "turn on Typing over Wi-Fi in Setup, under Paired phones") || !strings.Contains(updateWiFiWords, "anyone on this Wi-Fi could then read what you type") {
		t.Errorf("the offer doesn't say where, or the warning: %q", updateWiFiWords)
	}
	serve(t, d, call{method: "POST", target: "/setup/wifityping?on=1", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	if checks := d.updateChecks(time.Now()); len(checks) != 1 || checks[0].ID != "update" {
		t.Errorf("with Typing over Wi-Fi on, Start here still offers it: %+v", checks)
	}
	serve(t, d, call{method: "POST", target: "/setup/wifityping?on=0", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})

	// A restart doesn't lose it: the note is in the marker, the hires from the registry the core holds.
	again := restart(t)
	updateLineSays(t, again, time.Now(), checkTodo, want)
	// Each re-hired: fired, and a hire of 1.26's own, with its hook key, in their place.
	if err := updateRegistry(func(f *registryFile) {
		for i := range f.hires {
			f.hires[i].State = "fired"
		}
		f.hires = append(f.hires, Hire{ID: "h4", Name: "Builder", Nickname: "Ada", Folder: project, State: "working", Started: float64(time.Now().UnixMilli())})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := newHookKey(registryPath(), "h4"); err != nil {
		t.Fatal(err)
	}
	updateLineSays(t, again, time.Now(), checkInfo, []string{updateKept, updateKeys, updatePhone}, "Re-hire")
	if c := again.updateCheck(started.Add((updateNoteDays + 1) * 24 * time.Hour)); c != nil {
		t.Errorf("two weeks on, with nothing left to do, Start here still says it: %+v", *c)
	}
	// A hire from before still open keeps it after the two weeks.
	if err := updateRegistry(func(f *registryFile) { f.hires[0].State = "working" }); err != nil {
		t.Fatal(err)
	}
	updateLineSays(t, again, started.Add(30*24*time.Hour), checkTodo, []string{"Re-hire your hires from 1.24 (Ada)"})
}

// Under the Mac app: the update to 1.26 seals what the Swift engine's 1.24.2 left as found (B1): the accounts, a GM
// started by hand, the guard. Setup also names the Macs to pair again, and the GM started by hand's new token.
func TestTheUpdateFrom1242OnAMacSealsTheOfficeAndSaysWhatReset(t *testing.T) {
	isolate(t)
	withMac(t)
	office1242(t, true, false)
	d := recordsDeck(t)
	if recordsState() != recordsOK {
		t.Fatalf("the records' state: %q", recordsState())
	}
	d.mu.Lock()
	config := d.config
	kept := len(config.ClaudeProfiles) == 1 && config.ClaudeProfiles[0].Name == "work" && config.GMByHand && config.KeystrokesAllowed &&
		slices.Equal(config.KeystrokeApps, []string{"com.example.notes"}) && config.MailInboxSocket && config.Colours["body"] == "#102030"
	d.mu.Unlock()
	if !kept || !readRegistry(registryPath(), false).mayWrite("h1", "h2") {
		t.Errorf("what the Swift engine left wasn't kept: %+v", settingsByName(t, config))
	}
	groups := heldGuardGroups(filepath.Join(supportDir, hireGuardFile))
	if deploy := slices.IndexFunc(groups, func(g guard.Group) bool { return g.Name == "deploy" }); deploy < 0 || groups[deploy].On {
		t.Errorf("the guard wasn't kept as found: %+v", groups)
	}
	for _, name := range []string{"phones.json", registryName, "config.json"} {
		data, _ := os.ReadFile(filepath.Join(supportDir, name))
		if seal, ok := sealOfFile(name, data); !ok || !seal.fits(recordKey()) {
			t.Errorf("%s isn't sealed after the update", name)
		}
	}
	line := updateLineSays(t, d, time.Now(), checkTodo, []string{updateKept, updateKeys, updatePhone,
		"The 2 Macs paired with this one before the update were let go: pair each one again.", updateByHand,
		"Re-hire your hires from 1.24 (Ada, Bo and the General Manager)"}, "fell back")
	// In the self-test, as Start here and `smolcorp setup` show it: first, before the app's install line.
	checks := d.selfTest()
	// Then the offer of Typing over Wi-Fi: the Swift engine's 1.24.2 typed over the Wi-Fi; 1.26 doesn't until asked.
	if i := slices.IndexFunc(checks, func(c check) bool { return c.ID == "update" }); i != 0 || checks[i] != line ||
		checks[1].ID != "update-wifityping" || checks[1].Detail != updateWiFiWords || checks[2].ID != "install" {
		t.Errorf("the self-test's update lines: %+v", checks)
	}
	d.mu.Lock()
	wifi := d.config.PhoneTextWiFi
	d.mu.Unlock()
	if wifi {
		t.Error("the update left Typing over Wi-Fi on")
	}
}

// An update the Mac can't take as found (the record key's item already there, so the files' seals can't fit): the
// security settings and the guard fall back to their safe defaults, never looser, and Setup says which, in the
// words they were dropped with, for as long as the line shows, not only for that start.
func TestAnUpdateWhoseRecordsDidntFitSaysWhatFellBack(t *testing.T) {
	isolate(t)
	mac := withMac(t)
	mac.recordItems = []bridgeRecordKey{newRecordItem("key-from-a-test-build")}
	office1242(t, true, false)
	d := recordsDeck(t)
	d.mu.Lock()
	looser := d.config.KeystrokesAllowed || d.config.GMByHand || d.config.HiringEnabled || d.config.Webhook.Enabled || d.config.MailInboxSocket
	d.mu.Unlock()
	if looser || recordsFirst() {
		t.Fatalf("records that didn't fit still counted: first %v", recordsFirst())
	}
	fellBack := "config.json's security settings were changed by another program, or their seal doesn't fit: they're back to their safe defaults"
	updateLineSays(t, d, time.Now(), checkTodo, []string{updateKeptBut, updateKeys, fellBack})
	updateLineSays(t, restart(t), time.Now(), checkTodo, []string{updateKeptBut, fellBack})
}

// A new support folder, or one where the Mac app wrote config.json before its core's first start: no update.
func TestANewFolderSaysNothingOfAnUpdate(t *testing.T) {
	isolate(t)
	if err := os.WriteFile(configFile, []byte(`{"engine":"go"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	d := recordsDeck(t)
	if c := d.updateCheck(time.Now()); c != nil {
		t.Errorf("a new folder says it was updated: %+v", *c)
	}
	data, _ := os.ReadFile(filepath.Join(supportDir, recordsRotatedFile))
	if string(data) != keysChangedText {
		t.Errorf("the marker: %q", data)
	}
	// The marker deleted to change every key again (keys-and-guard.md): the folder 1.26 made, its config.json with
	// 1.26's fields, isn't an update from 1.24, though its hook key, one of an earlier office's files, is there.
	d.mu.Lock()
	d.config.save()
	d.mu.Unlock()
	if _, err := os.Stat(filepath.Join(supportDir, "hook-key")); err != nil {
		t.Fatalf("1.26 left no hook key: %v", err)
	}
	if err := os.Remove(filepath.Join(supportDir, recordsRotatedFile)); err != nil {
		t.Fatal(err)
	}
	again := restart(t)
	if c := again.updateCheck(time.Now()); c != nil {
		t.Errorf("a folder 1.26 made says it was updated once its marker is deleted: %+v", *c)
	}
	// With a config.json 1.24 wrote in its place, the same files are the update again.
	if note := noteUpdate(supportDir, time.Now()); note != nil {
		t.Errorf("the folder 1.26 made: %+v", *note)
	}
	if err := os.WriteFile(configFile, []byte(`{"hasCompletedSetup":true,"phoneMicEnabled":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if noteUpdate(supportDir, time.Now()) == nil {
		t.Error("a 1.24 config.json among an earlier office's files isn't the update")
	}
}

// A hire from before the update that has a hook key of its own isn't named: T3 Code's hand-back gives a 1.24 hire
// one and keeps when it started (t3takeover_tmux.go), so Setup mustn't ask to hire it again.
func TestAHireFromBeforeWithAHookKeyOfItsOwnIsntNamed(t *testing.T) {
	isolate(t)
	office1242(t, false, false)
	d := recordsDeck(t)
	if _, err := newHookKey(registryPath(), "h2"); err != nil {
		t.Fatal(err)
	}
	updateLineSays(t, d, time.Now(), checkTodo, []string{"Re-hire your hires from 1.24 (Ada and the General Manager)"}, "Bo")
}

// Nothing is looser after the update than 1.24.2 had it, on any system: every security setting 1.24.2 kept is as it
// was or stricter, every one it didn't have is at its safe default, and no guard group it had on is off. Checked from
// a 1.24.2 office at its strictest, where any change at all would be a loosening, and as one opened up.
func TestNothingIsLooserAfterTheUpdateFrom1242(t *testing.T) {
	// The security settings, by their names in config.json, and what each is at its strictest.
	strictest := map[string]any{"hiringEnabled": false, "managerEnabled": false, "gmByHand": false, "keystrokesAllowed": false,
		"mailInboxSocket": false, "t3CodeEnabled": false, "floorsFromOtherMacs": false, "phoneMicSkipsConfirm": false,
		"phoneTextOverWiFi": false}
	// New since 1.24.2: at their safe default whatever 1.24.2 had. Typing over Wi-Fi among them: 1.24.2 had no
	// switch, and off a Mac no phone text at all, so it stays off on every system, the open office's included.
	newSince := []string{"t3CodeEnabled", "floorsFromOtherMacs", "phoneMicSkipsConfirm", "phoneTextOverWiFi"}
	for _, mac := range []bool{false, true} {
		for _, strict := range []bool{true, false} {
			t.Run(map[bool]string{true: "mac", false: "go"}[mac]+map[bool]string{true: "-strict", false: "-open"}[strict], func(t *testing.T) {
				isolate(t)
				if mac {
					withMac(t)
				}
				office1242(t, mac, strict)
				var before map[string]any
				data, _ := os.ReadFile(configFile)
				_ = json.Unmarshal(data, &before)
				d := recordsDeck(t)
				d.mu.Lock()
				after := settingsByName(t, d.config)
				d.mu.Unlock()
				for name, strictValue := range strictest {
					was, had := before[name]
					if !had || slices.Contains(newSince, name) {
						was = strictValue
					}
					held, ok := after[name]
					if !ok {
						held = false // left out of the file when off
					}
					if held != was && held != strictValue {
						t.Errorf("%s: 1.24.2 had %v, 1.26 holds %v", name, was, held)
					}
				}
				if listen := after["listen"]; listen == "all" && before["listen"] != "all" {
					t.Errorf("listen: 1.24.2 had %v, 1.26 holds all", before["listen"])
				}
				hook, was := after["webhook"].(map[string]any), before["webhook"].(map[string]any)
				if hook["enabled"] == true && was["enabled"] != true || hook["outside"] == true && was["outside"] != true || hook["enabled"] == true && hook["url"] != was["url"] {
					t.Errorf("the webhook: 1.24.2 had %v, 1.26 holds %v", was, hook)
				}
				if apps, _ := after["keystrokeApps"].([]any); len(apps) > 0 && before["keystrokeApps"] == nil {
					t.Errorf("keys to other apps: 1.26 holds %v", apps)
				}
				for _, group := range heldGuardGroups(filepath.Join(supportDir, hireGuardFile)) {
					if !group.On && (strict || group.Name != "deploy") {
						t.Errorf("guard group %s is off; 1.24.2 had it on", group.Name)
					}
				}
				if d.access.Phones.WithKey("fake-1242-phone") != nil || phonesOnly(d.access.Phones) != 0 {
					t.Error("a phone from 1.24.2 is let in")
				}
			})
		}
	}
}

// CI's upgrade from the real 1.24.2 (scripts/upgrade) looks for Setup's own words in `smolcorp setup --check`: the
// same text as here, so a change of words fails here before it fails there.
func TestTheUpgradeJobLooksForSetupsOwnWords(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "scripts", "upgrade", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, words := range []string{updateTitle, updateKept, updateKeys, updatePhone, updateWiFiTitle, updateWiFiWords, "Re-hire your hires from 1.24 ("} {
		quoted, _ := json.Marshal(words)
		if !bytes.Contains(data, quoted) {
			t.Errorf("scripts/upgrade/main.go doesn't look for %s", quoted)
		}
	}
}
