package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// requests are the stand-in's requests for action, in order.
func (m *macStandIn) requests(action string) []bridgeRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	var found []bridgeRequest
	for _, request := range m.asked {
		if request.Action == action {
			found = append(found, request)
		}
	}
	return found
}

// set changes the stand-in, safely: the deck's rounds may be asking it.
func (m *macStandIn) set(change func(m *macStandIn)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	change(m)
}

// Under the Mac app, setup's state says what the Swift engine's says of the app's window, panel, login item
// and Keychain (C1's A2, A3, A10, A11), and nothing more is asked of macOS than the app's own answers.
func TestUnderTheMacAppSetupHasItsWindowPanelLoginItemAndKeychain(t *testing.T) {
	mac := withMac(t)
	mac.set(func(m *macStandIn) { m.login = true })
	d := newTestDeck(t)
	state := (&router{deck: d}).setupState()
	if window := state["officeWindow"].(map[string]any); window["available"] != true || window["mac"] != true || window["onTop"] != false {
		t.Errorf("officeWindow: %v, want the Mac's own, not on top", window)
	}
	if state["hiring"].(map[string]any)["canChoose"] != true {
		t.Error("hiring.canChoose: the Mac's panel can be opened")
	}
	if state["checklist"].(map[string]any)["openAtLogin"] != true {
		t.Error("checklist.openAtLogin: the app's login item is on")
	}
	summary := state["summary"].(map[string]any)
	if summary["noStore"] != false {
		t.Error("summary.noStore: the Mac's Keychain keeps a pasted key")
	}
	providers := summary["providers"].([]map[string]any)
	if key := providers[len(providers)-1]; key["id"] != providerAPIKey || key["reason"] != "add a key below" {
		t.Errorf("the API key's line: %v", key)
	}
	if asked := mac.requests("login-item"); len(asked) == 0 || asked[0].On != nil {
		t.Errorf("the login item is read, never set, by the state: %+v", asked)
	}
}

// Without the Mac app, the state and the routes are as on Windows and Linux: nothing is asked of any app.
func TestWithoutTheMacAppTheWindowPanelAndLoginItemAreAsBefore(t *testing.T) {
	saved := host
	host = nil
	t.Cleanup(func() { host = saved })
	d := newTestDeck(t)
	state := (&router{deck: d}).setupState()
	if window := state["officeWindow"].(map[string]any); window["mac"] != false || window["available"] != hasNativeWindow {
		t.Errorf("officeWindow: %v", window)
	}
	if state["hiring"].(map[string]any)["canChoose"] != hasFolderPicker() {
		t.Error("hiring.canChoose: this system's own picker")
	}
	if state["checklist"].(map[string]any)["openAtLogin"] != isOpenAtLogin() {
		t.Error("checklist.openAtLogin: this program's own")
	}
	if !hasNativeWindow {
		if code, text := pageCall(t, d, "POST", "/setup/ontop?enabled=1"); code != http.StatusNotFound || !strings.Contains(text, "opens in the browser") {
			t.Errorf("on top with no window: %d %q", code, text)
		}
	}
	for _, check := range d.selfTest() {
		if check.ID == "accessibility" || strings.Contains(check.Title, "Mac") {
			t.Errorf("a Mac line without the Mac app: %+v", check)
		}
	}
}

// Keeping the office's window on top: told to the app at once, then kept, as the Swift engine's /setup/ontop.
func TestTheOfficeWindowIsKeptOnTopByTheApp(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	code, _ := pageCall(t, d, "POST", "/setup/ontop?enabled=1")
	if code != http.StatusOK || !loadConfig().AlwaysOnTop {
		t.Fatalf("on top: %d, kept %v", code, loadConfig().AlwaysOnTop)
	}
	if asked := mac.requests("office-ontop"); len(asked) != 1 || asked[0].On == nil || !*asked[0].On {
		t.Errorf("the app was asked %+v", asked)
	}
	if (&router{deck: d}).setupState()["officeWindow"].(map[string]any)["onTop"] != true {
		t.Error("the state says it's on top")
	}
	// The app's no, or no answer: as it was.
	mac.refusing(map[string]string{"office-ontop": "The Mac didn't answer in time."})
	if code, text := pageCall(t, d, "POST", "/setup/ontop?enabled=0"); code != http.StatusConflict || text != "The Mac didn't answer in time." || !loadConfig().AlwaysOnTop {
		t.Errorf("refused: %d %q, kept %v", code, text, loadConfig().AlwaysOnTop)
	}
}

// Open the office, from the page: the Mac's own window, never the browser.
func TestOpenTheOfficeShowsTheMacsWindow(t *testing.T) {
	mac := withMac(t)
	saved := openURL
	t.Cleanup(func() { openURL = saved })
	var opened []string
	openURL = func(url string) error { opened = append(opened, url); return nil }
	d := newTestDeck(t)
	if code, _ := pageCall(t, d, "POST", "/setup/office"); code != http.StatusNoContent {
		t.Errorf("office: %d", code)
	}
	if len(mac.requests("office")) != 1 || len(opened) != 0 {
		t.Errorf("the app asked %d times, the browser opened %v", len(mac.requests("office")), opened)
	}
	mac.refusing(map[string]string{"office": "The Mac didn't answer in time."})
	if code, _ := pageCall(t, d, "POST", "/setup/office"); code != http.StatusConflict || len(opened) != 0 {
		t.Errorf("refused: %d, the browser opened %v", code, opened)
	}
}

// Open at login: the app's login item, set and read through the bridge; a test copy's app says no, and
// setup answers as it is, as the Swift engine's own LoginItem leaves a test copy's alone.
func TestOpenAtLoginIsTheAppsLoginItem(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	if code, _ := pageCall(t, d, "POST", "/setup/login?enabled=1"); code != http.StatusOK {
		t.Fatalf("login on: %d", code)
	}
	if set := mac.requests("login-item"); len(set) == 0 || set[0].On == nil || !*set[0].On {
		t.Errorf("the login item was asked %+v", set)
	}
	if !d.openAtLogin() || (&router{deck: d}).setupState()["checklist"].(map[string]any)["openAtLogin"] != true {
		t.Error("the state says it opens at login")
	}
	mac.refusing(map[string]string{"login-item": "A test copy has no login item: it's the copy in use's."})
	if code, _ := pageCall(t, d, "POST", "/setup/login?enabled=0"); code != http.StatusOK {
		t.Errorf("a test copy's login off: %d, want 200 as the Mac's", code)
	}
	if d.openAtLogin() {
		t.Error("an app that says no isn't a login item that's on")
	}
	// The self-test's fix: done by the app, and a test copy's no isn't a failure.
	if status, message := d.fix(fixLogin); status != http.StatusNoContent {
		t.Errorf("fix login in a test copy: %d %q", status, message)
	}
}

// The Mac's folder panel: opened by the app where the hires' folder is, for the hires or a floor in the
// app's own words; with pick=1 only the folder picked is answered, otherwise it's kept, as the Swift engine's.
func TestTheFolderPanelIsTheMacsThroughTheApp(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	start, project := filepath.Join(homeDir, "hires"), filepath.Join(homeDir, "project")
	for _, folder := range []string{start, project} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	d.mu.Lock()
	d.config.HireFolder = start
	d.mu.Unlock()
	mac.set(func(m *macStandIn) { m.picked = project })

	code, text := pageCall(t, d, "POST", "/setup/hiring/choose?pick=1&for=floor")
	if want, _ := json.Marshal(map[string]string{"picked": project}); code != http.StatusOK || text != string(want) {
		t.Errorf("picked for a floor: %d %s", code, text)
	}
	asked := mac.requests("choose-folder")
	if len(asked) != 1 || asked[0].For != folderForFloor || asked[0].Folder != start || asked[0].wait < folderPanelWait {
		t.Errorf("the panel asked %+v, want for a floor at %s, waiting for you", asked, start)
	}
	if loadConfig().HireFolder == project {
		t.Error("a folder only picked isn't kept")
	}
	// for=floor counts only with pick=1, as the Mac's.
	if code, _ := pageCall(t, d, "POST", "/setup/hiring/choose?for=floor"); code != http.StatusOK || loadConfig().HireFolder != project {
		t.Errorf("chosen: %d, kept %q", code, loadConfig().HireFolder)
	}
	if asked := mac.requests("choose-folder"); asked[1].For != folderForHires {
		t.Errorf("the hires' panel asked %+v", asked[1])
	}
	// Nothing picked (cancelled, or a panel already open, brought forward): nothing changes.
	mac.set(func(m *macStandIn) { m.picked = "" })
	if code, _ := pageCall(t, d, "POST", "/setup/hiring/choose"); code != http.StatusOK || loadConfig().HireFolder != project {
		t.Errorf("none picked: %d, kept %q", code, loadConfig().HireFolder)
	}
	if code, text := pageCall(t, d, "POST", "/setup/hiring/choose?pick=1"); code != http.StatusOK || text != `{"picked":""}` {
		t.Errorf("none picked, only picking: %d %s", code, text)
	}
	// A folder typed on the page while the panel is open closes it.
	if code, _ := pageCall(t, d, "POST", "/setup/hire-folder?folder="+start); code != http.StatusOK || len(mac.requests("folder-panel-close")) != 1 {
		t.Errorf("a folder typed: %d, the panel closed %d times", code, len(mac.requests("folder-panel-close")))
	}
	if code, _ := pageCall(t, d, "POST", "/setup/hiring?folder="+project); code != http.StatusOK || len(mac.requests("folder-panel-close")) != 2 {
		t.Errorf("a hiring folder typed: %d, the panel closed %d times", code, len(mac.requests("folder-panel-close")))
	}
	mac.refusing(map[string]string{"choose-folder": "The Mac didn't answer in time."})
	if code, _ := pageCall(t, d, "POST", "/setup/hiring/choose"); code != http.StatusConflict {
		t.Errorf("the app's no: %d", code)
	}
}

// ✨ Summary's key, pasted in setup: kept in the Mac's Keychain through the app, never in this program's
// files, and read back from there.
func TestTheSummaryKeyIsKeptInTheMacsKeychain(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	// Shaped as Anthropic makes keys, but built here, so no key-shaped text is committed.
	key := "sk-" + "ant-fake-key-0123456789"
	w := serve(t, d, call{method: "POST", target: "/setup/summary/key", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}, body: `{"key":"` + key + `"}`})
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), key) {
		t.Fatalf("saved: %d, the key in the answer: %v", w.Code, strings.Contains(w.Body.String(), key))
	}
	writes := mac.requests("secret-write")
	if len(writes) != 1 || writes[0].Item != summaryKeyItem || writes[0].Value != key {
		t.Errorf("the app was asked to keep %d items", len(writes))
	}
	if d.summaryKeys.Read() != key || d.summaryKeys.Shown() != "store" || d.summaryKeys.NoStore() {
		t.Error("the key is read back from the Keychain")
	}
	// Nowhere in the support folder.
	_ = filepath.WalkDir(supportDir, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			if data, _ := os.ReadFile(path); strings.Contains(string(data), key) {
				t.Errorf("the key is in %s", path)
			}
		}
		return nil
	})
	if code, _ := pageCall(t, d, "POST", "/setup/summary/key?remove=1"); code != http.StatusOK || len(mac.requests("secret-delete")) != 1 {
		t.Errorf("removed: %d", code)
	}
	if d.summaryKeys.Saved() != "" {
		t.Error("a removed key is gone")
	}
	// An app that doesn't answer: no store, as setup says it.
	store := summaryStore(&fakeHost{err: errors.New("no socket")})
	if _, err := store.get(); err == nil {
		t.Error("an app that doesn't answer is an error")
	}
}

// The self-test under the Mac app: Accessibility after the hooks, then the network, phone and login lines
// in the Mac's words, as SelfTest.swift has them; its fixes asked of the app.
func TestTheSelfTestHasTheMacsLinesUnderTheMacApp(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	checks := d.selfTest()
	var ids []string
	for _, check := range checks {
		ids = append(ids, check.ID)
	}
	at := slices.Index(ids, "hooks")
	if at < 0 || !slices.Equal(ids[at:], []string{"hooks", "accessibility", "network", "phone", "login"}) {
		t.Fatalf("the checks: %v", ids)
	}
	// The install line is the app's own, first, as the Swift engine's self-test has it (A13).
	if !slices.Equal(ids[:at], []string{"install"}) || checks[0].Detail != "As /Applications/Smolcorp.app." {
		t.Errorf("the install line: %v, %+v", ids[:at], checks[0])
	}
	accessibility, network, login := checks[at+1], checks[at+2], checks[at+4]
	// The fixture's facts: Accessibility not given, a test copy.
	if accessibility.Status != checkTodo || accessibility.Fix != fixAccessibility || accessibility.Label != "Allow" ||
		accessibility.Title != "Smolcorp can use your keyboard" {
		t.Errorf("accessibility: %+v", accessibility)
	}
	if network.Title != "Your phone can reach this Mac" || strings.Contains(network.Detail, "computer") || !strings.HasPrefix(network.Detail, "On this Wi-Fi") {
		t.Errorf("network: %+v", network)
	}
	if login.Status != checkInfo || login.Detail != "A test copy leaves the login item to the Smolcorp you use." || login.Fix != "" {
		t.Errorf("a test copy's login: %+v", login)
	}
	if status, _ := d.fix(fixAccessibility); status != http.StatusNoContent || len(mac.requests("accessibility")) != 1 {
		t.Errorf("fix accessibility: %d, asked %d", status, len(mac.requests("accessibility")))
	}
	// Not a test copy, Accessibility given, the login item off.
	mac.set(func(m *macStandIn) { m.facts.TestCopy, m.facts.Accessibility = false, true })
	checks = d.selfTest()
	if c := checks[at+1]; c.Status != checkOK || c.Fix != "" {
		t.Errorf("accessibility given: %+v", c)
	}
	if c := checks[at+4]; c.Status != checkTodo || c.Fix != fixLogin || c.Label != "Open at login" {
		t.Errorf("the login item off: %+v", c)
	}
	mac.set(func(m *macStandIn) { m.login = true })
	if c := d.selfTest()[at+4]; c.Status != checkOK || c.Detail != "It's in the menu bar from the moment you log in." {
		t.Errorf("the login item on: %+v", c)
	}
	// The key Smolcorp is signed with, unlocked on this Mac: a line of advice last, with nothing to press (SD1). Locked,
	// or not said (an ad hoc app, a test copy, an app from before): no line.
	for _, unlocked := range []*bool{nil, new(bool)} {
		mac.set(func(m *macStandIn) { m.facts.SigningKeyUnlocked = unlocked })
		for _, check := range d.selfTest() {
			if check.ID == "signing" {
				t.Errorf("a signing line with the key not said unlocked (%v): %+v", unlocked, check)
			}
		}
	}
	yes := true
	mac.set(func(m *macStandIn) { m.facts.SigningKeyUnlocked = &yes })
	checks = d.selfTest()
	signing := checks[len(checks)-1]
	if signing.ID != "signing" || signing.Status != checkTodo || signing.Fix != "" ||
		!strings.Contains(signing.Detail, "another program could sign a fake Smolcorp") || !strings.Contains(signing.Detail, "Keys, records and the guard") {
		t.Errorf("the signing key unlocked: %+v", signing)
	}
	if ready(checks[:len(checks)-1]) != ready(checks) {
		t.Error("advice about the signing key stands in the way of being ready")
	}
	mac.set(func(m *macStandIn) { m.facts.SigningKeyUnlocked = nil })
	// No facts from the app: no Accessibility line rather than a made-up one.
	mac.refusing(map[string]string{"facts": "The Mac didn't answer in time."})
	for _, check := range d.selfTest() {
		if check.ID == "accessibility" {
			t.Error("an Accessibility line the app didn't say")
		}
	}
	// Nor an install line the app didn't say.
	mac.refusing(map[string]string{"install-check": "The Mac didn't answer in time."})
	for _, check := range d.selfTest() {
		if check.ID == "install" {
			t.Error("an install line the app didn't say")
		}
	}
}

// Move to Applications is the app's: asked of it, and its no (a test copy stays where it was built) is the 409 of
// the Swift engine's fix route. A tool may not press it under the Mac app, as the Mac's own fix route says.
func TestMoveToApplicationsIsAskedOfTheMacApp(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	if status, message := d.fix(fixInstall); status != http.StatusNoContent || message != "" || len(mac.requests("install")) != 1 {
		t.Errorf("fix install: %d %q, asked %d", status, message, len(mac.requests("install")))
	}
	mac.refusing(map[string]string{"install": "A test copy stays where it was built."})
	if status, message := d.fix(fixInstall); status != http.StatusConflict || message != "A test copy stays where it was built." {
		t.Errorf("a refused install: %d %q", status, message)
	}
	mac.refusing(nil)
	if fixAllowed(callerTool, fixInstall, "", true) || !fixAllowed(callerPage, fixInstall, "", true) {
		t.Error("under the Mac app, only the page presses Move to Applications")
	}
}

// The office key's refusals are the Mac's bytes: the sentence, no newline after it (as every setup refusal).
func TestTheOfficeKeysRefusalsHaveNoTrailingNewline(t *testing.T) {
	withMac(t)
	d := newTestDeck(t)
	for _, keys := range []string{"", "K", "shift+K", "cmd+Q", "cmd+Tab"} {
		got := serve(t, d, call{method: "POST", target: "/setup/officekey?keys=" + url.QueryEscape(keys), host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}})
		if got.Code != http.StatusBadRequest || got.Body.Len() == 0 || strings.HasSuffix(got.Body.String(), "\n") {
			t.Errorf("keys %q: %d %q", keys, got.Code, got.Body.String())
		}
	}
}

// The Mac's own sentences, word for word as SelfTest.swift says them.
func TestTheMacsNetworkLine(t *testing.T) {
	with := macNetworkCheck([]string{"100.64.0.1"})
	if with.Status != checkOK || with.Detail != "On this Wi-Fi, and through Tailscale at 100.64.0.1 from anywhere." || with.Fix != "" {
		t.Errorf("with Tailscale: %+v", with)
	}
	without := macNetworkCheck(nil)
	if without.Status != checkInfo || without.Fix != fixTailscale || without.Label != "Get Tailscale" ||
		without.Detail != "On this Wi-Fi. With Tailscale, free, on this Mac and your phone, it reaches you from anywhere." {
		t.Errorf("without: %+v", without)
	}
}
