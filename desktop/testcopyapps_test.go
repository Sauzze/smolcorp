package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// A test copy never brings another app forward or touches a real terminal app (RTCH N1), on either engine:
// the sentences are the Mac's, word for word, and each route says one instead of acting.

// macOtherApps is the Mac's TestCopy.OtherApp sentences, read from its source, so the two can't drift apart.
func macOtherApps(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "Sources", "AgentDeck", "Paths.swift"))
	if err != nil {
		t.Skip("the Mac app's source isn't here")
	}
	found := regexp.MustCompile(`return "(This is a test copy of Smolcorp: it never [^"]*)"`).FindAllStringSubmatch(string(data), -1)
	var said []string
	for _, match := range found {
		said = append(said, match[1])
	}
	return said
}

func TestATestCopysSentencesAreTheMacs(t *testing.T) {
	said := macOtherApps(t)
	for _, want := range []string{testCopyKeepsTerminal, testCopyKeepsWindows, testCopyKeepsGrok, testCopyKeepsT3Code, testCopyKeepsAccessibility,
		testCopyKeepsFixes, testCopyKeepsMusic, testCopyKeepsMicrophone} {
		found := false
		for _, mac := range said {
			found = found || mac == want
		}
		if !found {
			t.Errorf("the Mac says nothing like %q: %q", want, said)
		}
	}
	if len(said) != 8 {
		t.Errorf("the Mac has %d sentences, this has 8", len(said))
	}
}

// onMarkedPort makes this a marked test copy on port: SMOLCORP_TEST_SUPPORT_DIR beside it, as the Mac shell gives.
func onMarkedPort(t *testing.T, port int) {
	t.Helper()
	onPort(t, port)
	t.Setenv(testSupportVariable, t.TempDir())
}

// macTabDeck is a deck under the stand-in Mac app with desk s1, a Claude Code session in an iTerm2 tab.
func macTabDeck(t *testing.T) (*Deck, *macStandIn) {
	t.Helper()
	mac := withMac(t)
	d := newTestDeck(t)
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, Claude, "iterm:AB12-CD34", "", 100)
	d.mu.Unlock()
	return d, mac
}

// The office's own brings the tab forward through the Mac app; a test copy asks it nothing (RTCH N1, RWTC N8).
func TestATestCopyBringsNoTerminalForwardOnAnyComputer(t *testing.T) {
	d, mac := macTabDeck(t)
	onDefaultPort(t)
	_, _ = d.Focus("s1")
	if asked := mac.requests("focus"); len(asked) != 1 {
		t.Errorf("the office's own tap asked the Mac %+v", asked)
	}
	onMarkedPort(t, 7799)
	if status, why := d.Focus("s1"); status != http.StatusConflict || why != testCopyKeepsTerminal {
		t.Errorf("a test copy's tap: %d %q, want 409 and the sentence", status, why)
	}
	if asked := mac.requests("focus"); len(asked) != 1 {
		t.Errorf("a test copy's tap asked the Mac: %+v", asked)
	}
	// A window of a hire's terminal, either entry.
	if status, why := d.openHireWindow(Hire{ID: "h1", State: "working"}); status != http.StatusConflict || why != testCopyKeepsTerminal {
		t.Errorf("a test copy's Open a window: %d %q", status, why)
	}
}

// RWTC N1: every control, the phone's words and a peek, in a tab that isn't the copy's own: refused in a sentence,
// on either engine, and nothing reaches the tab. The office's own goes through.
func TestATestCopyDoesNothingInATabThatIsntItsOwn(t *testing.T) {
	d, mac := macTabDeck(t)
	d.mu.Lock()
	d.config.ScrollEnabled, d.config.HiringEnabled, d.config.PhoneMicEnabled = true, true, true
	d.mu.Unlock()
	onMarkedPort(t, 7799)
	query := url.Values{"lines": {"3"}, "level": {"high"}, "button": {"clear"}, "force": {"1"}, "phase": {"down"}}
	for _, action := range []string{"accept", "send", "reject", "esc", "up", "down", "scroll", "mic", "fire", "clear", "button", "effort"} {
		if status, why := d.Perform(action, "s1", query); status != http.StatusConflict || why != testCopyKeepsTerminal {
			t.Errorf("%s on a test copy: %d %q", action, status, why)
		}
	}
	if status, why := d.TypePhoneText("hello", "s1", nil); status != http.StatusConflict || why != testCopyKeepsTerminal {
		t.Errorf("the phone's words on a test copy: %d %q", status, why)
	}
	if status, _, why := d.Peek("s1"); status != http.StatusConflict || why != testCopyKeepsTerminal {
		t.Errorf("a peek on a test copy: %d %q", status, why)
	}
	for _, action := range []string{"press", "type", "scroll", "scroll-bottom", "close", "phone-text", "peek", "focus"} {
		if asked := mac.requests(action); len(asked) != 0 {
			t.Errorf("the Mac app was asked %s: %+v", action, asked)
		}
	}
	// The office's own presses it.
	onDefaultPort(t)
	_, _ = d.Perform("accept", "s1", url.Values{})
	if asked := mac.requests("press"); len(asked) != 1 {
		t.Errorf("the office's own accept asked the Mac %+v", asked)
	}
}

// A launcher (Linux, Windows) is the owner's terminal window too: a test copy tells it nothing.
func TestATestCopyTellsALauncherNothing(t *testing.T) {
	d, tab := madeButtonDeck(t)
	onMarkedPort(t, 7799)
	for _, action := range []string{"accept", "clear", "esc"} {
		if status, why := d.Perform(action, "s1", url.Values{}); status != http.StatusConflict || why != testCopyKeepsTerminal {
			t.Errorf("%s: %d %q", action, status, why)
		}
	}
	select {
	case command := <-tab.commands:
		t.Errorf("the launcher was told %+v", command)
	default:
	}
}

// A tap on a Grok Bot app desk opens nothing on a test copy, but its news is seen (RWTC N9).
func TestATestCopyOpensNoGrokAppButSeesItsNews(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	bot := standardCloudBotNamed("mini", "Mini")
	bot.Source = CloudSource{Kind: "grokBotApp", Bot: "Mini"}
	withCloudBots(d, bot)
	reported(d, "mini", &CloudAction{Text: "Sent a report", Count: 1})
	unread := func() bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		s := d.cloud.sessions[bot.sessionID()]
		return s != nil && s.State == Unread
	}
	if !unread() {
		t.Fatal("the report isn't news")
	}
	onMarkedPort(t, 7799)
	if status, why := d.Focus("cloud-mini"); status != http.StatusConflict || why != testCopyKeepsGrok {
		t.Errorf("a Grok Bot app desk's tap on a test copy: %d %q", status, why)
	}
	if unread() {
		t.Error("its news wasn't seen")
	}
	// A bot with no app behind it has nothing to open: seen, as ever.
	plain := standardCloudBotNamed("plain", "Plain")
	withCloudBots(d, plain)
	if status, _ := d.Focus("cloud-plain"); status != http.StatusNoContent {
		t.Errorf("a bot of the event route alone: %d", status)
	}
	if asked := mac.requests("open"); len(asked) != 0 {
		t.Errorf("the Mac app was asked to open: %+v", asked)
	}
}

// A tap on a T3 desk opens nothing on a test copy, but its done is seen (RWTC N8, N9); the Take over's own open
// says the same.
func TestATestCopyOpensNoT3CodeButSeesItsDone(t *testing.T) {
	r, d, h := t3DeskRig(t)
	desk := t3DeskID(h.ID)
	d.mu.Lock()
	d.config.T3CodeEnabled = true
	d.mu.Unlock()
	// Done, as T3 Code said it last: green till seen.
	d.t3.Lock()
	d.t3.seen[h.ID] = t3Glance{Status: "done"}
	d.t3.Unlock()
	d.mu.Lock()
	_, _ = d.t3HireAtDesk(desk)
	state := d.t3Desks.byHire[h.ID]
	d.mu.Unlock()
	if state == nil || state.State != Unread {
		t.Fatalf("the desk before the tap: %+v", state)
	}
	onMarkedPort(t, 7799)
	if status, why := d.Focus(desk); status != http.StatusConflict || why != testCopyKeepsT3Code || len(r.opened) != 0 {
		t.Errorf("a T3 desk's tap on a test copy: %d %q, opened %v", status, why, r.opened)
	}
	d.mu.Lock()
	seen := d.t3Desks.seenDone[h.ID]
	d.mu.Unlock()
	if !seen {
		t.Error("its done wasn't seen")
	}
	status, answer := t3OpenOnce(h, t3Machine{})
	if why, _ := answer.(string); status != http.StatusConflict || why != testCopyKeepsT3Code {
		t.Errorf("T3 Code's own open on a test copy: %d %v", status, answer)
	}
	if len(r.opened) != 0 {
		t.Errorf("opened %v", r.opened)
	}
}

func TestATestCopyAsksMacOSForNothingAndRunsNoFixThatOpensAnApp(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	onMarkedPort(t, 7799)
	for fix, want := range map[string]string{fixAccessibility: testCopyKeepsAccessibility, fixTailscale: testCopyKeepsFixes,
		fixFirewall: testCopyKeepsFixes, fixHomeWiFi: testCopyKeepsFixes} {
		if status, why := d.fix(fix); status != http.StatusConflict || why != want {
			t.Errorf("fix %s on a test copy: %d %q", fix, status, why)
		}
	}
	w := serve(t, d, call{method: "POST", target: "/setup/accessibility", host: "localhost:7799", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}})
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), testCopyKeepsAccessibility) {
		t.Errorf("POST /setup/accessibility on a test copy: %d %q", w.Code, w.Body)
	}
	if asked := mac.requests("accessibility"); len(asked) != 0 {
		t.Errorf("macOS was asked for Accessibility: %+v", asked)
	}
	// The office's own asks, as ever.
	onDefaultPort(t)
	if status, _ := d.fix(fixAccessibility); status != http.StatusNoContent {
		t.Errorf("the office's own Accessibility fix: %d", status)
	}
	if asked := mac.requests("accessibility"); len(asked) != 1 {
		t.Errorf("the Mac app was asked %+v", asked)
	}
}

// A hire's window is the one place a terminal app is opened, and a test copy never gets past it.
func TestATestCopyOpensNoTerminalWindowForAHire(t *testing.T) {
	opened := false
	terminal := hireOpener{id: "terminal", name: "Terminal", open: func(string, []string) (string, error) { opened = true; return "", nil }}
	if _, err := showWindow(terminal, t.TempDir(), []string{"true"}); err != nil || !opened {
		t.Fatalf("the office's own window: %v, opened %v", err, opened)
	}
	opened = false
	onMarkedPort(t, 7799)
	if _, err := showWindow(terminal, t.TempDir(), []string{"true"}); err == nil || err.Error() != testCopyKeepsTerminal || opened {
		t.Errorf("a test copy's window: %v, opened %v", err, opened)
	}
}

// A test copy's hires always run in tmux, whatever the switch says (RTCH N1): the window above is never even tried
// while tmux is here. Without tmux it is the window's turn, and showWindow refuses (RMSY4 nit 2).
func TestATestCopysHiresAlwaysRunInTmux(t *testing.T) {
	onDefaultPort(t)
	off := &Config{}
	if backgroundHires(off, true) {
		t.Error("the office's own hires run in the background with the switch off")
	}
	if !backgroundHires(&Config{HiresInBackground: true}, true) {
		t.Error("the office's own hires don't run in the background with the switch on")
	}
	onPort(t, portFromEnvironment("")+89)
	if !backgroundHires(off, true) {
		t.Error("a test copy's hire would open a window with tmux here")
	}
	if backgroundHires(off, false) {
		t.Error("a test copy's hire runs in the background with no tmux")
	}
}

// RTCH N2: a test copy's self-test describes what it does: it listens on this computer only.
func TestATestCopysSelfTestReadsWhereItReallyListens(t *testing.T) {
	d := newTestDeck(t)
	d.mu.Lock()
	d.config.Listen = listenTailscale
	d.mu.Unlock()
	onDefaultPort(t)
	if got := effectiveScope(d.config); got != listenTailscale {
		t.Errorf("the office's scope is %q, the config's", got)
	}
	onPort(t, 7799)
	t.Setenv(testListenVariable, "")
	if got := effectiveScope(d.config); got != listenThisComputer {
		t.Errorf("a test copy's scope is %q, want this computer", got)
	}
	t.Setenv(testListenVariable, " LAN ")
	if got := effectiveScope(d.config); got != listenAll {
		t.Errorf("a test copy let onto the network (with spaces, as the Mac reads it) is %q", got)
	}
	t.Setenv(testListenVariable, "")
	c := networkCheck(listenThisComputer, []string{"100.64.0.1"})
	if c.Status != checkInfo || c.Fix != "" || !strings.Contains(c.Detail, "this computer only") {
		t.Errorf("a test copy's network line: %+v", c)
	}
	for _, line := range d.selfTest() {
		if line.ID == "firewall" || line.Fix == fixHomeWiFi || line.Fix == fixTailscale {
			t.Errorf("a test copy's self-test offers %+v", line)
		}
	}
}

// RFXC1a N1/N3: a copy on another port that isn't marked a test copy says it isn't connected, and the GM's preflight
// doesn't offer Connect on either.
func TestAPortOnlyCopySaysItsAgentsAreNotConnected(t *testing.T) {
	d := newTestDeck(t)
	onPort(t, 7799)
	t.Setenv(testSupportVariable, "")
	if testCopyMarked() {
		t.Fatal("a port alone is marked a test copy")
	}
	var hooks check
	for _, line := range d.selfTest() {
		if line.ID == "hooks" {
			hooks = line
		}
	}
	if hooks.Status == checkOK || hooks.Fix != "" || !strings.Contains(hooks.Detail, "Not connected: Smolcorp is on port 7799") {
		t.Errorf("a port-only copy's hooks line: %+v", hooks)
	}
	t.Setenv(testSupportVariable, t.TempDir())
	if !testCopyMarked() {
		t.Fatal("a port and a support folder aren't a test copy")
	}
	for _, line := range d.selfTest() {
		if line.ID == "hooks" {
			hooks = line
		}
	}
	if hooks.Status != checkOK || hooks.Fix != "" || !strings.HasPrefix(hooks.Detail, testCopyHires) {
		t.Errorf("a test copy's hooks line is the Mac's: %+v", hooks)
	}
	// The office's own port is never a test copy, whatever the folder.
	onDefaultPort(t)
	if testCopyMarked() {
		t.Error("the office's own port is marked a test copy")
	}
}

func TestThePreflightOffersNoConnectOnATestCopy(t *testing.T) {
	d, _, _, _ := preflightDeck(t)
	onPort(t, 7799)
	for _, marked := range []bool{true, false} {
		folder := ""
		if marked {
			folder = t.TempDir()
		}
		t.Setenv(testSupportVariable, folder)
		for _, item := range d.gmPreflight().Items {
			if item.Action != nil && item.Action.Fix == fixRepair {
				t.Errorf("marked %v: the preflight offers Connect: %+v", marked, item)
			}
			if item.ID == "hooks" && item.OK != marked {
				t.Errorf("marked %v: the hooks line %+v", marked, item)
			}
		}
		if line := preflightLine(t, d.gmPreflight(), "hooks"); line.Action != nil || (marked && line.Title != testCopyHires) {
			t.Errorf("marked %v: the hooks line is %+v", marked, line)
		}
	}
}

// RFXC1a N5: the Mac says how it writes a short time, and the core writes it so.
func TestAMacShortTimeIsWrittenAsTheMacsPatternSaysIt(t *testing.T) {
	at := time.Date(2026, 11, 11, 13, 5, 9, 0, time.Local)
	midnight := time.Date(2026, 11, 11, 0, 7, 0, 0, time.Local)
	narrow := " "
	cases := []struct {
		format macTimeFormat
		t      time.Time
		want   string
		ok     bool
	}{
		{macTimeFormat{"h:mm" + narrow + "a", "AM", "PM"}, at, "1:05" + narrow + "PM", true},
		{macTimeFormat{"h:mm" + narrow + "a", "AM", "PM"}, midnight, "12:07" + narrow + "AM", true},
		{macTimeFormat{"HH:mm", "", ""}, at, "13:05", true},
		{macTimeFormat{"HH:mm", "", ""}, midnight, "00:07", true},
		{macTimeFormat{"H:mm", "", ""}, midnight, "0:07", true},
		{macTimeFormat{"a h:mm", "午前", "午後"}, at, "午後 1:05", true},
		{macTimeFormat{"HH 'h' mm", "", ""}, at, "13 h 05", true},
		{macTimeFormat{"HH.mm 'o''clock'", "", ""}, at, "13.05 o'clock", true},
		{macTimeFormat{"K:mm a", "am", "pm"}, at, "1:05 pm", true},
		{macTimeFormat{"k:mm", "", ""}, midnight, "24:07", true},
		{macTimeFormat{"h:mm:ss a", "AM", "PM"}, at, "1:05:09 PM", true},
		{macTimeFormat{"", "", ""}, at, "", false},
		{macTimeFormat{"h:mm B", "AM", "PM"}, at, "", false}, // a day period: the Region settings decide
		{macTimeFormat{"h:mm a", "", ""}, at, "", false},
	}
	for _, c := range cases {
		got, ok := c.format.format(c.t)
		if ok != c.ok || got != c.want {
			t.Errorf("%+v at %v: %q %v, want %q %v", c.format, c.t.Format("15:04"), got, ok, c.want, c.ok)
		}
	}
}

// RWTC N7: a delivery's time asks the Mac only how it writes a time, never the whole facts.
func TestADeliveryIsTimedAsTheMacSaysIt(t *testing.T) {
	mac := withMac(t)
	// A deck that isn't started: a running deck's own rounds ask the facts on their own, which isn't the delivery's ask.
	d := &Deck{}
	d.mac.host = mac
	saved := macTimePrefs
	t.Cleanup(func() { macTimePrefs = saved })
	macTimePrefs = func() macTimeSettings { return macTimeSettings{force24: "1"} }
	at := time.Date(2026, 11, 11, 20, 41, 0, 0, time.Local)
	// The Mac's pattern beats the Region settings' reading.
	mac.set(func(m *macStandIn) { m.timeFormat = macTimeFormat{"h:mm a", "am", "pm"} })
	before := len(mac.requests("facts"))
	if got := d.deliveryClock(at); got != "8:41 pm" {
		t.Errorf("with the Mac's pattern: %q", got)
	}
	if asked := mac.requests("facts"); len(asked) != before {
		t.Errorf("the facts were asked for a delivery's time: %+v", asked[before:])
	}
	// No pattern: the Region settings, as before; and so for an app that doesn't know the action.
	mac.set(func(m *macStandIn) { m.timeFormat = macTimeFormat{} })
	if got := d.deliveryClock(at); got != "20:41" {
		t.Errorf("with no pattern: %q", got)
	}
	mac.refusing(map[string]string{"time-format": "This Mac app can't do time-format for the engine yet."})
	if got := d.deliveryClock(at); got != "20:41" {
		t.Errorf("from an older app: %q", got)
	}
}

// RFXC1a N4: the sentences the Mac says "Mac" in, said so under the Mac app.
func TestTheMacsWordsReachTheRemainingSentences(t *testing.T) {
	for _, message := range []string{hireWindowElsewhere, backgroundNeedsTmux, hiringOff} {
		mac, found := macWords[message]
		if !found || !strings.Contains(mac, "Mac") || strings.Contains(mac, "computer") {
			t.Errorf("%q has no Mac wording: %q", message, mac)
		}
	}
	saved := host
	t.Cleanup(func() { host = saved })
	host = &fakeHost{}
	if got := inMacWords(backgroundNeedsTmux); got != "Running hires in the background needs tmux, which isn't on this Mac." {
		t.Errorf("under the Mac app: %q", got)
	}
	host = nil
	if got := inMacWords(backgroundNeedsTmux); !strings.Contains(got, "this computer") {
		t.Errorf("elsewhere: %q", got)
	}
	// The folder a hire can't use.
	home := t.TempDir()
	library := filepath.Join(home, "Library")
	if got := hireFolderRefusal(library, home); !strings.Contains(got, "your computer's or") {
		t.Errorf("elsewhere: %q", got)
	}
	host = &fakeHost{}
	if got := hireFolderRefusal(library, home); !strings.Contains(got, "your Mac's or") {
		t.Errorf("under the Mac app: %q", got)
	}
}

// copyCall is a setup page's POST on the test copy's own port, from this computer.
func copyCall(t *testing.T, d *Deck, target string) (int, string) {
	t.Helper()
	w := serve(t, d, call{method: "POST", target: target, host: "localhost:7799", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	return w.Code, strings.TrimSpace(w.Body.String())
}

// RWTC N2: a test copy's jukebox plays nothing and leaves the volume, and its dictation never asks for the microphone.
func TestATestCopyLeavesTheMusicAndTheMicrophoneAlone(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	d.mu.Lock()
	d.config.MusicEnabled = true // as in a settings file it was given
	d.config.DictationEnabled = true
	d.mu.Unlock()
	pressed := false
	savedPress, savedSet := pressMusicKey, setMusicVolume
	t.Cleanup(func() { pressMusicKey, setMusicVolume = savedPress, savedSet })
	pressMusicKey = func(string) error { pressed = true; return nil }
	setMusicVolume = func(int) error { pressed = true; return nil }
	onMarkedPort(t, 7799)
	for _, query := range []url.Values{{"do": {"playpause"}}, {"do": {"next"}}, {"do": {"volume"}, "level": {"30"}}} {
		if status, why := d.Perform("music", "", query); status != http.StatusConflict || why != testCopyKeepsMusic {
			t.Errorf("the jukebox %v on a test copy: %d %q", query, status, why)
		}
	}
	d.followMusic()
	if code, text := copyCall(t, d, "/setup/music?enabled=1"); code != http.StatusConflict || text != testCopyKeepsMusic {
		t.Errorf("the jukebox turned on: %d %q", code, text)
	}
	if code, _ := copyCall(t, d, "/setup/music?enabled=0"); code != http.StatusOK {
		t.Errorf("the jukebox turned off: %d", code)
	}
	if code, text := copyCall(t, d, "/setup/dictation?enabled=1"); code != http.StatusConflict || text != testCopyKeepsMicrophone {
		t.Errorf("dictation turned on: %d %q", code, text)
	}
	// Dictation already on: its language changes, and the microphone still isn't asked for.
	if code, _ := copyCall(t, d, "/setup/dictation?claude=1"); code != http.StatusOK {
		t.Errorf("dictation's Claude switch: %d", code)
	}
	if pressed {
		t.Error("a jukebox key or the volume reached the computer")
	}
	// The Mac app is asked none of these: the jukebox, dictation made ready, or a recording (RGAPA2 N-a).
	for _, action := range []string{"music", "volume", "music-state", "dictation", "dictation-start"} {
		if asked := mac.requests(action); len(asked) != 0 {
			t.Errorf("the Mac app was asked %s: %+v", action, asked)
		}
	}
	// The office's own plays it.
	onDefaultPort(t)
	d.mu.Lock()
	d.config.MusicEnabled = true
	d.mu.Unlock()
	if status, why := d.Perform("music", "", url.Values{"do": {"playpause"}}); status != http.StatusNoContent || !pressed {
		t.Errorf("the office's own jukebox: %d %q", status, why)
	}
}

// RWTC N3: on Linux and Windows a test copy's self-test never sets opening at login or installs itself; under the
// Mac app the login item is the app's to refuse, as before.
func TestATestCopyLeavesLoginAndInstallToTheSmolcorpInUse(t *testing.T) {
	saved := host
	t.Cleanup(func() { host = saved })
	host = nil
	d := newTestDeck(t)
	onMarkedPort(t, 7799)
	for fix, want := range map[string]string{fixLogin: testCopyKeepsLogin, fixInstall: testCopyKeepsInstall} {
		if status, why := d.fix(fix); status != http.StatusConflict || why != want {
			t.Errorf("fix %s on a test copy: %d %q", fix, status, why)
		}
	}
	// Under the Mac app the login item is asked of the app, which keeps it for the Smolcorp in use (SelfTest.testCopyLogin).
	mac := withMac(t)
	d = newTestDeck(t)
	if status, _ := d.fix(fixLogin); status != http.StatusNoContent || len(mac.requests("login-item")) != 1 {
		t.Errorf("under the Mac app: %d, asked %+v", status, mac.requests("login-item"))
	}
}

// RWTC N4: a copy only on another port (not marked a test copy) refuses the same things, in words that don't call it one.
func TestAPortOnlyCopySaysItsPortNotTestCopy(t *testing.T) {
	saved := host
	t.Cleanup(func() { host = saved })
	host = nil
	d := newTestDeck(t)
	onPort(t, 7799)
	t.Setenv(testSupportVariable, "")
	for why, want := range map[string]string{
		testCopyKeepsTerminal: "Smolcorp on port 7799 never brings a terminal app forward, opens a window in one or touches its tabs: only the one on port 7710 does that.",
		testCopyKeepsFixes:    "Smolcorp on port 7799 never opens another app, a download page or this computer's settings to fix something: only the one on port 7710 does that.",
		testCopyKeepsLogin:    "Smolcorp on port 7799 leaves opening at login to the one on port 7710.",
		testCopyKeepsInstall:  "Smolcorp on port 7799 stays where it was built: only the one on port 7710 installs itself.",
	} {
		if got := testCopyKeepsAway(why); got != want {
			t.Errorf("%q: %q", why, got)
		}
	}
	if status, why := d.fix(fixLogin); status != http.StatusConflict || strings.Contains(why, "test copy") || !strings.Contains(why, "port 7799") {
		t.Errorf("a port-only copy's login fix: %d %q", status, why)
	}
	if _, err := showWindow(hireOpener{id: "terminal", open: func(string, []string) (string, error) { return "", nil }}, t.TempDir(), []string{"true"}); err == nil ||
		strings.Contains(err.Error(), "test copy") {
		t.Errorf("a port-only copy's hire window: %v", err)
	}
	// Marked, it says it's a test copy, as the Mac does.
	t.Setenv(testSupportVariable, t.TempDir())
	if got := testCopyKeepsAway(testCopyKeepsTerminal); got != testCopyKeepsTerminal {
		t.Errorf("a marked test copy: %q", got)
	}
}

// RWTC N5: off the Mac a test copy's fixes speak of this computer, and Accessibility is no fix at all, as for the
// office's own; under the Mac app the Mac's words, word for word.
func TestATestCopysFixesOffTheMacSpeakOfThisComputer(t *testing.T) {
	saved := host
	t.Cleanup(func() { host = saved })
	host = nil
	d := newTestDeck(t)
	onMarkedPort(t, 7799)
	for _, fix := range []string{fixFirewall, fixHomeWiFi, fixTailscale} {
		status, why := d.fix(fix)
		if status != http.StatusConflict || strings.Contains(why, "System Settings") || !strings.Contains(why, "this computer's settings") {
			t.Errorf("fix %s off the Mac: %d %q", fix, status, why)
		}
	}
	if status, why := d.fix(fixAccessibility); status != http.StatusBadRequest || why != errUnknownFix.Error() {
		t.Errorf("Accessibility off the Mac: %d %q", status, why)
	}
	withMac(t)
	d = newTestDeck(t)
	if status, why := d.fix(fixFirewall); status != http.StatusConflict || why != testCopyKeepsFixes {
		t.Errorf("under the Mac app: %d %q", status, why)
	}
}
