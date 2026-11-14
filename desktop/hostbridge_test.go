package desktop

import (
	"errors"
	"net/http"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"
)

// fakeHost stands in for the Mac app: it keeps what it was asked and answers
// with answer, or with err.
type fakeHost struct {
	asked  []bridgeRequest
	answer bridgeAnswer
	err    error
}

func (f *fakeHost) call(request bridgeRequest) (bridgeAnswer, error) {
	f.asked = append(f.asked, request)
	return f.answer, f.err
}

// withHost puts a fake Mac app in place for one test.
func withHost(t *testing.T, answer bridgeAnswer) *fakeHost {
	t.Helper()
	saved := host
	t.Cleanup(func() { host = saved })
	fake := &fakeHost{answer: answer}
	host = fake
	return fake
}

func TestNoHostKeepsTheTabsAsTheyAre(t *testing.T) {
	d := newTestDeck(t)
	saved := host
	host = nil
	t.Cleanup(func() { host = saved })
	if _, ok := d.tab("tmux:%3@/tmp/tmux-501/default").(commandTab); !ok {
		t.Error("a tmux pane without the Mac app is a commandTab, as on Linux")
	}
	if tab := d.tab("headless:3f2a9c1e"); tab != nil {
		t.Errorf("a headless run has no tab, got %#v", tab)
	}
}

func TestTheMacAppDrivesItsOwnTerminals(t *testing.T) {
	d := newTestDeck(t)
	fake := withHost(t, bridgeAnswer{OK: true})
	tab := d.tab("tty:/dev/ttys003")
	mac, ok := tab.(macTab)
	if !ok || mac.pane != nil {
		t.Fatalf("Terminal.app's tab is the app's alone, got %#v", tab)
	}
	if _, ok := paneOf(tab); ok {
		t.Error("no pane of this program's behind a Terminal.app tab")
	}
	if err := tab.Press(Keystroke{Key: Escape, Times: 2}); err != nil {
		t.Fatal(err)
	}
	if err := tab.Type("hello"); err != nil {
		t.Fatal(err)
	}
	if err := tab.Scroll(-3); err != nil {
		t.Fatal(err)
	}
	if err := tab.ScrollToBottom(); err != nil {
		t.Fatal(err)
	}
	if err := tab.Focus(); err != nil {
		t.Fatal(err)
	}
	want := []bridgeRequest{
		{Action: "press", Tab: "tty:/dev/ttys003", Key: Escape, Times: 2},
		{Action: "type", Tab: "tty:/dev/ttys003", Text: "hello"},
		{Action: "scroll", Tab: "tty:/dev/ttys003", Lines: -3},
		{Action: "scroll-bottom", Tab: "tty:/dev/ttys003"},
		{Action: "focus", Tab: "tty:/dev/ttys003"},
	}
	if len(fake.asked) != len(want) {
		t.Fatalf("asked %+v", fake.asked)
	}
	for i := range want {
		if !reflect.DeepEqual(fake.asked[i], want[i]) {
			t.Errorf("request %d: got %+v, want %+v", i, fake.asked[i], want[i])
		}
	}
	// A key pressed with no count is pressed once.
	fake.asked = nil
	_ = tab.Press(Keystroke{Key: Enter})
	if fake.asked[0].Times != 1 {
		t.Errorf("times = %d, want 1", fake.asked[0].Times)
	}
}

func TestAPaneKeepsItsKeysAndTheAppBringsItForward(t *testing.T) {
	d := newTestDeck(t)
	fake := withHost(t, bridgeAnswer{OK: true, Fired: "fired"})
	tab := d.tab("tmux:%3@/tmp/tmux-501/default")
	pane, ok := paneOf(tab)
	if !ok || pane.kind != "tmux" || pane.id != "%3" || pane.socket != "/tmp/tmux-501/default" {
		t.Fatalf("the pane behind it: %#v, %v", pane, ok)
	}
	if err := tab.Focus(); err != nil {
		t.Fatal(err)
	}
	if err := tab.Close(Claude); err != nil {
		t.Fatal(err)
	}
	if len(fake.asked) != 2 || fake.asked[0].Action != "focus" || fake.asked[1].Action != "close" || fake.asked[1].Agent != Claude {
		t.Errorf("asked %+v", fake.asked)
	}
}

func TestCloseSaysWhatTheAppDid(t *testing.T) {
	d := newTestDeck(t)
	for fired, want := range map[string]error{"fired": nil, "not-stopped": errNotStopped} {
		withHost(t, bridgeAnswer{OK: true, Fired: fired})
		if err := d.tab("iterm:BD2206F6-EE01").Close(Claude); !errors.Is(err, want) {
			t.Errorf("%s: got %v, want %v", fired, err, want)
		}
	}
	withHost(t, bridgeAnswer{OK: true, Fired: "gone"})
	if err := d.tab("iterm:BD2206F6-EE01").Close(Claude); err == nil {
		t.Error("a tab that's gone isn't closed by this")
	}
	withHost(t, bridgeAnswer{OK: false, Error: "Accessibility is off"})
	if err := d.tab("tty:/dev/ttys003").Focus(); err == nil || err.Error() != "Accessibility is off" {
		t.Errorf("the app's refusal: %v", err)
	}
	// A terminal the app has no driver for: "can't do that", not "Tab not found".
	withHost(t, bridgeAnswer{OK: false, Error: "This terminal can't do that.", Unsupported: true})
	if err := d.tab("ghostty:/dev/ttys004").Focus(); !errors.Is(err, errUnsupported) {
		t.Errorf("unsupported: %v", err)
	}
	if code, said := d.inTab("ghostty:/dev/ttys004", func(tab Tab) error { return tab.Focus() }); code != http.StatusNotFound || said != "This terminal can't do that" {
		t.Errorf("%d %q", code, said)
	}
	fake := withHost(t, bridgeAnswer{})
	fake.err = errors.New("no socket")
	if err := d.tab("tty:/dev/ttys003").Press(press(Enter)); err == nil {
		t.Error("an app that can't be reached is an error")
	}
}

func TestTheAppsOwnItems(t *testing.T) {
	fake := &fakeHost{answer: bridgeAnswer{OK: true, Value: "sk-fake", Found: true}}
	value, found, err := hostReadSecret(fake, "summary-key")
	if err != nil || !found || value != "sk-fake" {
		t.Errorf("read: %q %v %v", value, found, err)
	}
	if err := hostWriteSecret(fake, "gm-token", "fake-token"); err != nil {
		t.Fatal(err)
	}
	if err := hostWriteSecret(fake, "gm-token", ""); err != nil {
		t.Fatal(err)
	}
	on := true
	fake.answer = bridgeAnswer{OK: true, On: true}
	if got, err := hostLoginItem(fake, &on); err != nil || !got {
		t.Errorf("login item: %v %v", got, err)
	}
	script := filepath.Join(supportDir, "hires", "hire-abc.sh")
	fake.answer = bridgeAnswer{OK: true, Tab: "iterm:BD2206F6"}
	if tab, _, err := hostOpenTerminal(fake, "iterm", "/Users/me/project", script); err != nil || tab != "iterm:BD2206F6" {
		t.Errorf("open: %q %v", tab, err)
	}
	want := []string{"secret-read", "secret-write", "secret-delete", "login-item", "open"}
	for i, action := range want {
		if fake.asked[i].Action != action {
			t.Errorf("request %d is %q, want %q", i, fake.asked[i].Action, action)
		}
	}
	if fake.asked[4].Script != script || fake.asked[4].Terminal != "iterm" {
		t.Errorf("open asked %+v", fake.asked[4])
	}
	if fake.asked[0].Item != bridgeSecrets[0] {
		t.Errorf("the Summary key is the first of the app's items: %q", bridgeSecrets[0])
	}
}

// watchOnly is a deck with a Claude Code session in a terminal Smolcorp can only watch, a tmux one, and a headless run, and
// a paired phone: asks gives what each of the phone's controls answers.
type watchOnly struct {
	d     *Deck
	phone string
}

func newWatchOnly(t *testing.T) watchOnly {
	t.Helper()
	d := newTestDeck(t)
	hookOn(d, "watch", "SessionStart", "/work/watch", "other:/dev/ttys009", "/work/watch", 1793500000000)
	hookOn(d, "tmuxed", "SessionStart", "/work/tmuxed", "tmux:%1@/tmp/fxc2b-no-such-socket", "/work/tmuxed", 1793500000001)
	hookOn(d, "headless", "SessionStart", "/work/headless", "headless:3f2a9c1e", "/work/headless", 1793500000002)
	d.mu.Lock()
	d.config.ScrollEnabled = true
	d.mu.Unlock()
	return watchOnly{d: d, phone: pair(t, d)}
}

func (w watchOnly) post(t *testing.T, path, query string) (int, string) {
	t.Helper()
	got := serve(t, w.d, call{method: "POST", target: path + "?" + query + "&t=" + w.phone, host: deckOnWiFi, from: aPhone})
	return got.Code, got.Body.String()
}

// On a desk in a terminal Smolcorp can only watch, under the Mac app, every control but Let go says "Smolcorp can't do
// that in this terminal yet." (409), the effort dial included, and a tap on the desk (/focus) only picks it (204), as
// the Swift engine does. A terminal it has a driver for takes the effort; a headless run is as it was for the controls.
func TestAWatchOnlyDeskAnswersAsTheMacsEngineDoes(t *testing.T) {
	withMac(t)
	w := newWatchOnly(t)
	for _, action := range []string{"effort&level=high", "send", "accept", "reject", "esc", "up", "down", "scroll&lines=3", "scroll&to=bottom", "mic&phase=down", "clear"} {
		if status, said := w.post(t, "/act", "action="+action+"&session=watch"); status != http.StatusConflict || said != notHere {
			t.Errorf("%s: %d %q", action, status, said)
		}
	}
	// The terminal is looked at before the level, as the Mac's order is.
	if status, said := w.post(t, "/act", "action=effort&level=nope&session=watch"); status != http.StatusConflict || said != notHere {
		t.Errorf("a bad level on a watch-only desk: %d %q", status, said)
	}
	if status, said := w.post(t, "/focus", "session=watch"); status != http.StatusNoContent || said != "" {
		t.Errorf("focus: %d %q", status, said)
	}
	// A headless run: nothing to bring forward either (the Mac's 204), and its controls say it runs headless.
	if status, said := w.post(t, "/focus", "session=headless"); status != http.StatusNoContent || said != "" {
		t.Errorf("focus, headless: %d %q", status, said)
	}
	if status, said := w.post(t, "/act", "action=effort&level=high&session=headless"); status != http.StatusConflict || said != headlessNoControls {
		t.Errorf("effort, headless: %d %q", status, said)
	}
	// The effort is taken where the terminal has a driver, and queued until the agent is free.
	if status, said := w.post(t, "/act", "action=effort&level=high&session=tmuxed"); status != http.StatusNoContent && status != http.StatusAccepted || said != "" {
		t.Errorf("effort in tmux: %d %q", status, said)
	}
	// A watch-only desk's effort wasn't taken.
	w.d.mu.Lock()
	pending := w.d.sessions.Get("watch").PendingEffort
	w.d.mu.Unlock()
	if pending != "" {
		t.Errorf("the effort was kept: %q", pending)
	}
}

// Without the Mac app, Linux and Windows keep what they answered: the dial's effort is taken on any desk (their desks
// show the dial everywhere), a control the terminal can't do is 404 "This terminal can't do that", a tap on a watch-only
// desk is that 404 and on a headless run its own 409.
func TestWithoutTheMacAppAWatchOnlyDeskIsAsItWas(t *testing.T) {
	w := newWatchOnly(t)
	for _, action := range []string{"send", "accept", "esc", "scroll&lines=3", "mic&phase=down", "clear"} {
		if status, said := w.post(t, "/act", "action="+action+"&session=watch"); status != http.StatusNotFound || said != "This terminal can't do that" {
			t.Errorf("%s: %d %q", action, status, said)
		}
	}
	if status, said := w.post(t, "/act", "action=effort&level=high&session=watch"); status != http.StatusNoContent || said != "" {
		t.Errorf("effort: %d %q", status, said)
	}
	if status, said := w.post(t, "/focus", "session=watch"); status != http.StatusNotFound || said != "This terminal can't do that" {
		t.Errorf("focus: %d %q", status, said)
	}
	if status, said := w.post(t, "/focus", "session=headless"); status != http.StatusConflict || said != headlessNoControls {
		t.Errorf("focus, headless: %d %q", status, said)
	}
}

// A queued effort is typed by one of the deck's rounds, so a test's deck, stopped as the test ends, waits for it: it
// never runs into the next test, where its look at the GM's role reads the folders that test puts in (FXMEM3RACE).
func TestStoppingADeckWaitsForTheEffortItQueued(t *testing.T) {
	w := newWatchOnly(t)
	fake := withHost(t, bridgeAnswer{OK: true})
	// The key worker is kept busy, so the effort waits behind it.
	underWay, finish := make(chan struct{}), make(chan struct{})
	w.d.work <- func() { close(underWay); <-finish }
	<-underWay
	if status, said := w.post(t, "/act", "action=effort&level=high&session=watch"); status != http.StatusNoContent || said != "" {
		t.Fatalf("effort: %d %q", status, said)
	}
	stopped := make(chan struct{})
	go func() { w.d.stopWatchers(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("the stop didn't wait for the effort queued")
	case <-time.After(50 * time.Millisecond):
	}
	close(finish)
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("the deck did not stop")
	}
	if len(fake.asked) != 1 || fake.asked[0].Action != "type" || fake.asked[0].Text != "/effort high" {
		t.Errorf("asked %+v, want /effort high typed before the stop's end", fake.asked)
	}
	w.d.mu.Lock()
	effort := w.d.sessions.Get("watch").Effort
	w.d.mu.Unlock()
	if effort != "high" {
		t.Errorf("the effort: %q", effort)
	}
}

// The items the core asks the Mac app for are bridgeSecrets, every one and no other, as HostBridge.swift's secrets
// are (HostBridgeTests checks the app's side): the Macs this one joined among them (FL0), or a Mac couldn't join,
// and the VAPID key (PUSH1), or a Mac couldn't send a notification.
func TestTheCoreAsksTheAppForItsOwnItemsOnly(t *testing.T) {
	asked := []string{summaryKeyItem, gmTokenItem, peerKeysItem, pushKeyItem}
	for _, item := range viewerItems {
		asked = append(asked, item)
	}
	slices.Sort(asked)
	if items := slices.Sorted(slices.Values(bridgeSecrets)); !slices.Equal(asked, items) {
		t.Errorf("the core asks for %v, the app's items are %v", asked, items)
	}
}
