package desktop

import (
	"net/http"
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
	for _, want := range []string{testCopyKeepsTerminal, testCopyKeepsWindows, testCopyKeepsGrok, testCopyKeepsT3Code, testCopyKeepsAccessibility, testCopyKeepsFixes} {
		found := false
		for _, mac := range said {
			found = found || mac == want
		}
		if !found {
			t.Errorf("the Mac says nothing like %q: %q", want, said)
		}
	}
	if len(said) != 6 {
		t.Errorf("the Mac has %d sentences, this has 6", len(said))
	}
}

func TestATestCopyBringsNoTerminalForwardOnAnyComputer(t *testing.T) {
	d, tab := madeButtonDeck(t)
	go func() {
		for range tab.commands {
			t.Error("a tab was told something")
		}
	}()
	// The office's own focuses it, as before.
	onDefaultPort(t)
	if status, why := d.Focus("s1"); status == http.StatusConflict && why == testCopyKeepsTerminal {
		t.Errorf("the office's own port refused: %d %q", status, why)
	}
	onPort(t, 7799)
	if status, why := d.Focus("s1"); status != http.StatusConflict || why != testCopyKeepsTerminal {
		t.Errorf("a test copy's tap: %d %q, want 409 and the sentence", status, why)
	}
	// A window of a hire's terminal, either entry.
	if status, why := d.openHireWindow(Hire{ID: "h1", State: "working"}); status != http.StatusConflict || why != testCopyKeepsTerminal {
		t.Errorf("a test copy's Open a window: %d %q", status, why)
	}
}

func TestATestCopyOpensNoGrokAppAndNoT3Code(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	bot := standardCloudBotNamed("mini", "Mini")
	bot.Source = CloudSource{Kind: "grokBotApp", Bot: "Mini"}
	withCloudBots(d, bot)
	onPort(t, 7799)
	if status, why := d.Focus("cloud-mini"); status != http.StatusConflict || why != testCopyKeepsGrok {
		t.Errorf("a Grok Bot app desk's tap on a test copy: %d %q", status, why)
	}
	// A bot with no app behind it has nothing to open: seen, as ever.
	plain := standardCloudBotNamed("plain", "Plain")
	withCloudBots(d, plain)
	if status, _ := d.Focus("cloud-plain"); status != http.StatusNoContent {
		t.Errorf("a bot of the event route alone: %d", status)
	}
	// T3 Code, any way it's asked: the open by the desk's tap, by the route, by the Take over's own open.
	for _, name := range []string{"focus", "open"} {
		status, answer := t3OpenOnce(Hire{ID: "h-" + name}, t3Machine{})
		why, _ := answer.(string)
		if status != http.StatusConflict || why != testCopyKeepsT3Code {
			t.Errorf("T3 Code on a test copy (%s): %d %v", name, status, answer)
		}
	}
	for _, action := range []string{"t3-open-app", "t3-link", "open", "focus"} {
		if asked := mac.requests(action); len(asked) != 0 {
			t.Errorf("the Mac app was asked %s: %+v", action, asked)
		}
	}
}

func TestATestCopyAsksMacOSForNothingAndRunsNoFixThatOpensAnApp(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	onPort(t, 7799)
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
	onPort(t, 7799)
	if _, err := showWindow(terminal, t.TempDir(), []string{"true"}); err == nil || err.Error() != testCopyKeepsTerminal || opened {
		t.Errorf("a test copy's window: %v, opened %v", err, opened)
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

func TestADeliveryIsTimedAsTheMacsFactsSayIt(t *testing.T) {
	mac := withMac(t)
	d := newTestDeck(t)
	saved := macTimePrefs
	t.Cleanup(func() { macTimePrefs = saved })
	macTimePrefs = func() macTimeSettings { return macTimeSettings{force24: "1"} }
	at := time.Date(2026, 11, 11, 20, 41, 0, 0, time.Local)
	// The facts' pattern beats the Region settings' reading.
	mac.set(func(m *macStandIn) { m.facts.TimeFormat = macTimeFormat{"h:mm a", "am", "pm"} })
	if got := d.deliveryClock(at); got != "8:41 pm" {
		t.Errorf("with the Mac's pattern: %q", got)
	}
	// No pattern (an older app): the Region settings, as before.
	mac.set(func(m *macStandIn) { m.facts.TimeFormat = macTimeFormat{} })
	if got := d.deliveryClock(at); got != "20:41" {
		t.Errorf("with no pattern: %q", got)
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
