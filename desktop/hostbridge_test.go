package desktop

import (
	"errors"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"
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
