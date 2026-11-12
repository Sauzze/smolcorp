package desktop

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Setup on the Go engine answers as the Mac's (C1's comparison of the two engines): under the Mac app in the Mac's
// words and formats, and on every computer where the Mac's way is the only one.

// The locked page's answer names the Mac app under it, so its note tells the Mac's way in (C1 A4).
func TestTheLockedStateNamesTheMacUnderTheMacApp(t *testing.T) {
	withMac(t)
	d := newTestDeck(t)
	w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, withoutSetupKey: true})
	if w.Code != http.StatusUnauthorized || w.Header().Get(appHeader) != "mac" {
		t.Errorf("the locked state under the Mac app: %d, app %q, want 401 and mac", w.Code, w.Header().Get(appHeader))
	}
}

// The phone's pairing page and the refusals say "Mac" under the Mac app, as the Swift engine's; elsewhere "computer"
// (C1 A5).
func TestPairingAndRefusalsSayMacUnderTheMacApp(t *testing.T) {
	for _, mac := range []bool{false, true} {
		word := map[bool]string{false: "computer", true: "Mac"}[mac]
		if mac {
			withMac(t)
		}
		d := newTestDeck(t)
		page := serve(t, d, call{method: "GET", target: "/", host: deckOnWiFi, from: aPhone}).Body.String()
		if !strings.Contains(page, "<h1>Pair with your "+word+"</h1>") || strings.Contains(page, "smolcorp quit") == mac ||
			strings.Contains(page, "Could not reach your "+word+".") != true {
			t.Errorf("mac %v: the pairing page isn't in the %s's words", mac, word)
		}
		if mac && (strings.Contains(page, "menu bar or tray") || !strings.Contains(page, "On your Mac, click the Smolcorp icon in the menu bar and choose")) {
			t.Error("the pairing page under the Mac app still names the tray")
		}
		wrong := serve(t, d, call{method: "POST", target: "/pair", host: deckOnWiFi, from: aPhone, body: `{"code":"000000"}`})
		if wrong.Code != http.StatusForbidden || !strings.Contains(wrong.Body.String(), "Get a new one on your "+word+".") {
			t.Errorf("mac %v: a wrong code: %d %s", mac, wrong.Code, wrong.Body)
		}
		status, said, _ := claudeAccountsCall(t, d, "POST", `{"profiles":[{"name":"w","dir":"~/.claude-gone"}]}`, false)
		if status != http.StatusBadRequest || !strings.HasSuffix(said, string(filepath.Separator)+".claude-gone isn't a folder on this "+word+".") {
			t.Errorf("mac %v: an account folder that isn't there: %d %q", mac, status, said)
		}
		key := pair(t, d)
		for target, want := range map[string]string{"/act?action=settings&t=" + key: "Settings open on your " + word + ".",
			"/t3code?t=" + key: "T3 Code opens on your " + word + "."} {
			w := serve(t, d, call{method: "POST", target: target, host: deckOnWiFi, from: aPhone})
			if w.Code != http.StatusForbidden || w.Body.String() != want {
				t.Errorf("mac %v: %s from a phone: %d %q, want %q", mac, strings.Split(target, "?")[0], w.Code, w.Body, want)
			}
		}
	}
}

// A layout's fixed key count, sent by setup, imported in a profile or found in config.json, is read but not kept,
// as the Mac's Layout: desks are as many as there are sessions (C1 A6).
func TestAFixedKeyCountIsReadButNotKept(t *testing.T) {
	isolate(t)
	if err := os.WriteFile(configFile, []byte(`{"layout": {"keys": 6, "dial": false}, "profiles": [{"name": "Couch", "layout": {"keys": 3}}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c := loadConfig()
	if c.Layout.Keys != Auto || c.Layout.Dial || len(c.Profiles) != 1 || c.Profiles[0].Layout.Keys != Auto {
		t.Errorf("config.json's fixed counts were kept: %v and %+v", c.Layout.Keys, c.Profiles)
	}
	if layout, err := parseLayout([]byte(`{"keys": 7}`)); err != nil || layout.Keys != Auto {
		t.Errorf("a layout with 7 keys: %v, %v", layout.Keys, err)
	}
}

// Floors and rules that aren't lists are refused in the Mac's words, and nothing is changed: never rules emptied
// (C1 A7).
func TestFloorsThatArentAListChangeNothing(t *testing.T) {
	d := newTestDeck(t)
	rules := floorRuleList{{"/work/p1", 3, false}}
	d.mu.Lock()
	d.config.FloorRules = rules
	floors := slices.Clone(d.config.Floors)
	d.mu.Unlock()
	for _, body := range []string{`{"floors":"x","rules":"y"}`, `{"rules":{"folder":"/work"}}`, `{"floors":null,"rules":null}`, `{"rules":7}`} {
		w := serve(t, d, call{method: "POST", target: "/setup/floors", host: "localhost:7710", from: thisComputer, body: body,
			headers: map[string]string{setupHeader: "1"}})
		if w.Code != http.StatusBadRequest || w.Body.String() != `Send the floors as a list: {"floors": [{"name": …, "preset": …}]}.` {
			t.Errorf("%s: %d %q, want 400 and the Mac's words", body, w.Code, w.Body)
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if !slices.Equal(d.config.FloorRules, rules) || !slices.Equal(d.config.Floors, floors) {
		t.Errorf("refused floors changed them: %v %v", d.config.FloorRules, d.config.Floors)
	}
	if got, ok := floorRulesSent([]byte(`{"rules": []}`)); !ok || len(got) != 0 {
		t.Errorf("an empty list of rules is a list: %v %v", got, ok)
	}
}

// A webhook delivery's time is the Mac's short time under the Mac app, as your Region settings write it; elsewhere
// 24-hour, as before (C1 A8).
func TestADeliverysTimeIsTheMacsUnderTheMacApp(t *testing.T) {
	at := time.Date(2026, 11, 11, 13, 13, 0, 0, time.Local)
	for _, c := range []struct {
		settings macTimeSettings
		want     string
	}{
		{macTimeSettings{locale: "en_US"}, "1:13 PM"},
		{macTimeSettings{locale: "en_GB"}, "13:13"},
		{macTimeSettings{locale: "fr_FR"}, "13:13"},
		{macTimeSettings{locale: "en_GB@rg=uszzzz"}, "1:13 PM"},
		{macTimeSettings{locale: "en_US@rg=frzzzz"}, "13:13"},
		{macTimeSettings{locale: "en_US", force24: "1"}, "13:13"},
		{macTimeSettings{locale: "fr_FR", force12: "1"}, "1:13 PM"},
		{macTimeSettings{}, "13:13"},
	} {
		if got := macShortTime(at, c.settings); got != c.want {
			t.Errorf("%+v: %q, want %q", c.settings, got, c.want)
		}
	}
	saved := macTimePrefs
	t.Cleanup(func() { macTimePrefs = saved })
	macTimePrefs = func() macTimeSettings { return macTimeSettings{locale: "en_US"} }
	plain := newTestDeck(t)
	if got := describedDelivery("204", plain.deliveryClock); !strings.HasPrefix(got, "Sent: 204 at ") || strings.Contains(got, "M") {
		t.Errorf("without the Mac app: %q, want 24-hour", got)
	}
	withMac(t)
	d := newTestDeck(t)
	if got := describedDelivery("204", d.deliveryClock); !strings.HasPrefix(got, "Sent: 204 at ") || !strings.HasSuffix(got, "M") {
		t.Errorf("under the Mac app: %q, want the Mac's 12-hour time", got)
	}
}

// The setup page's cookie is named per port, as the Mac's: a copy on another port has its own, so it never signs
// the real office's page out, nor opens with the real one's (C1 A9).
func TestTheSetupCookieIsNamedPerPort(t *testing.T) {
	onDefaultPort(t)
	if setupCookie() != "smolcorp-setup" {
		t.Errorf("the default port's cookie: %q", setupCookie())
	}
	onPort(t, 7750)
	if setupCookie() != "smolcorp-setup-7750" {
		t.Errorf("a copy on 7750's cookie: %q", setupCookie())
	}
	d := newTestDeck(t)
	d.mu.Lock()
	token := d.access.NewSetupToken()
	d.mu.Unlock()
	session := serve(t, d, call{method: "POST", target: "/setup/session", host: "localhost:7750", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupHeader: "1", setupTokenHeader: token}})
	if !strings.HasPrefix(session.Header().Get("Set-Cookie"), "smolcorp-setup-7750=") {
		t.Errorf("the session's cookie: %q", session.Header().Get("Set-Cookie"))
	}
	d.mu.Lock()
	value := d.access.NewSetupSession()
	d.mu.Unlock()
	real := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7750", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{"Cookie": "smolcorp-setup=" + value}})
	if real.Code != http.StatusUnauthorized {
		t.Errorf("the real office's cookie name opened a copy's setup: %d", real.Code)
	}
}

// Under the Mac app, a tool only repairs (with auto=1), as the Mac's: installing is the page's, refused in the Mac's
// words. Linux and Windows' smolcorp setup still installs (C1 A14).
func TestAToolCantInstallUnderTheMacApp(t *testing.T) {
	if fixAllowed(callerTool, fixInstall, "", true) || fixAllowed(callerTool, fixFirewall, "", true) || !fixAllowed(callerTool, fixRepair, "1", true) {
		t.Error("under the Mac app a tool may do more than repair, or not repair")
	}
	if !fixAllowed(callerPage, fixInstall, "", true) {
		t.Error("under the Mac app the page can't install")
	}
	withMac(t)
	d := newTestDeck(t)
	w := serve(t, d, call{method: "POST", target: "/setup/fix?fix=install", host: "localhost:7710", from: thisComputer, asTool: true,
		headers: map[string]string{setupHeader: "1"}})
	if w.Code != http.StatusForbidden || w.Body.String() != "This is for the setup page the menu opens." {
		t.Errorf("a tool's install under the Mac app: %d %q", w.Code, w.Body)
	}
}

// A 404 is empty, as the Mac's: setup from the network, the viewer's QR code with no link (C1 A15).
func TestNotFoundIsEmpty(t *testing.T) {
	d := newTestDeck(t)
	for _, c := range []call{
		{method: "GET", target: "/setup/state", host: deckOnWiFi, from: aPhone},
		{method: "GET", target: "/setup", host: deckOnWiFi, from: aPhone},
		{method: "GET", target: "/setup/viewer/qr.png", host: "localhost:7710", from: thisComputer},
		{method: "POST", target: "/hook", host: deckOnWiFi, from: aPhone},
	} {
		w := serve(t, d, c)
		if w.Code != http.StatusNotFound || w.Body.Len() != 0 {
			t.Errorf("%s %s from %s: %d %q, want an empty 404", c.method, c.target, c.from, w.Code, w.Body)
		}
	}
}

// Setup's page takes "launcher" in the state for a Linux or Windows computer: its step, its tmux label, its
// "secret store" for the Keychain. Under the Mac app none is sent, whether or not the app answers the facts
// (M7WORDS), so a Mac reads its own words, as the Swift engine's. Elsewhere it names the command.
func TestSetupSendsNoLauncherUnderTheMacApp(t *testing.T) {
	state := func(d *Deck) map[string]any { return (&router{deck: d}).setupStateFor(callerPage) }
	elsewhere := state(newTestDeck(t))
	if command, _ := elsewhere["launcher"].(string); !strings.HasSuffix(command, " run") {
		t.Errorf("without the Mac app the launcher is %v, want its command", elsewhere["launcher"])
	}
	for _, answering := range []bool{true, false} {
		mac := withMac(t)
		if !answering {
			mac.refusing(map[string]string{"facts": "The app is busy."})
		}
		got := state(newTestDeck(t))
		if _, sent := got["launcher"]; sent {
			t.Errorf("under the Mac app (answering %v) the state carries launcher %v", answering, got["launcher"])
		}
		if _, ok := got["terminalCommand"]; ok != answering {
			t.Errorf("under the Mac app (answering %v) the Mac's facts are in the state: %v", answering, ok)
		}
	}
}
