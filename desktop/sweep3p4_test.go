package desktop

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RT126SWEEP3 part 4's rows A, D and F3 (FXSWEEP3P4). These rows use only what the tree had before the fix, with the
// words it gives written out here, so they run on that tree too, and fail there (the fail-first); A2 and B are in
// sweep3p4_darwin_test.go, under the real profile.

// The words the fix gives (routes.go's hireFolderPageOnly and hiringOffOnly, act.go's officeWindowOnly).
const (
	sweepFolderPageOnly = "Only the owner chooses the folder for hires, in Setup: every hire's sandbox holds it."
	sweepHiringOffOnly  = "The GM can only turn hiring off. Turning it back on is the owner's, in Setup."
	sweepOfficeWindow   = "Do this in Smolcorp's own office window on your Mac, not in a browser."
)

// Row A (B1): the folder for hires is the projects folder every hire's sandbox holds (sweepStartHolds), so the
// owner's page alone moves it. The GM's token, a tool's key and a phone's key are refused on both routes that save it
// (create=1 included), and under the Mac app a browser's Setup beside the window is too. The saved folder, and so what
// a start holds, stays the owner's, and create=1 makes nothing. Smolcorp's own Settings window still moves it.
func TestOnlyTheOwnersPageMovesTheFolderForHires(t *testing.T) {
	d := newTestDeck(t)
	gm := d.newGMToken()
	phone := pair(t, d)
	projects, elsewhere, notYet := filepath.Join(homeDir, "Projects"), filepath.Join(homeDir, "Elsewhere"), filepath.Join(homeDir, "NotYet")
	for _, dir := range []string{projects, elsewhere} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if code, text := pageCall(t, d, "POST", "/setup/hire-folder?folder="+url.QueryEscape(projects)); code != http.StatusOK {
		t.Fatalf("the owner's page sets the folder: %d %q", code, text)
	}
	held := sweepStartHolds()
	if held == "" {
		t.Fatal("a start holds no projects folder")
	}
	moves := []string{
		"/setup/hire-folder?create=1&folder=" + url.QueryEscape(notYet),
		"/setup/hire-folder?folder=" + url.QueryEscape(elsewhere),
		"/setup/hiring?folder=" + url.QueryEscape(elsewhere),
		"/setup/hiring?enabled=1&folder=" + url.QueryEscape(elsewhere),
	}
	unchanged := func(after string) {
		t.Helper()
		d.mu.Lock()
		saved := d.config.HireFolder
		d.mu.Unlock()
		if got := sweepStartHolds(); got != held || savedConfig().HireFolder != saved {
			t.Errorf("after %s: a start holds %q, saved %q; want %q", after, got, savedConfig().HireFolder, held)
		}
		if _, err := os.Lstat(notYet); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("after %s: create=1 made %s", after, notYet)
		}
	}
	for _, move := range moves {
		for name, c := range map[string]call{
			"the GM's token": {withoutSetupKey: true, headers: map[string]string{gmTokenHeader: gm}},
			"a tool's key":   {asTool: true},
			"a phone's key":  {withoutSetupKey: true, headers: map[string]string{hookKeyHeader: phone}},
		} {
			c.method, c.target, c.host, c.from = "POST", move, "localhost:7710", thisComputer
			c.headers = mergeHeaders(c.headers, map[string]string{setupHeader: "1"})
			got := serve(t, d, c)
			want := http.StatusForbidden
			if name == "a phone's key" {
				want = http.StatusUnauthorized // a phone is nobody to Setup
			}
			if got.Code != want {
				t.Errorf("%s, POST %s: %d %q, want %d", name, move, got.Code, got.Body, want)
			}
			if name == "the GM's token" && strings.HasPrefix(move, "/setup/hiring") &&
				strings.TrimSpace(got.Body.String()) != sweepFolderPageOnly {
				t.Errorf("the GM's token, POST %s: told %q", move, got.Body)
			}
			unchanged(name + ", POST " + move)
		}
	}
	// Under the Mac app: a browser's Setup only looks; Smolcorp's own Settings window moves the folder.
	hostedByTheMacApp(d)
	for _, move := range moves {
		if got := withSession(t, d, browserSession(d), "POST", move, ""); got.Code != http.StatusForbidden ||
			strings.TrimSpace(got.Body.String()) != strictChangeWords {
			t.Errorf("a browser's Setup, POST %s: %d %q", move, got.Code, got.Body)
		}
		unchanged("a browser's Setup, POST " + move)
	}
	if got := withSession(t, d, windowSession(d), "POST", "/setup/hire-folder?folder="+url.QueryEscape(elsewhere), ""); got.Code != http.StatusOK {
		t.Fatalf("Smolcorp's own Settings window: %d %q", got.Code, got.Body)
	}
	if got := sweepStartHolds(); got == held || filepath.Base(got) != "Elsewhere" {
		t.Errorf("after the window's move a start holds %q, want Elsewhere", got)
	}
}

// Row D (F2): the GM may turn hiring off, never on. The owner turns it on; the GM's token turns it off; the GM's
// enabled=1, with a folder or without, is refused in plain words and changes nothing; the GM still changes how hires
// start; the owner turns it on again.
func TestTheGMTurnsHiringOffButNeverBackOn(t *testing.T) {
	d := newTestDeck(t)
	gm := d.newGMToken()
	projects, elsewhere := filepath.Join(homeDir, "Projects"), filepath.Join(homeDir, "Elsewhere")
	for _, dir := range []string{projects, elsewhere} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	asGM := func(query string) (int, string) {
		w := serve(t, d, call{method: "POST", target: "/setup/hiring?" + query, host: "localhost:7710", from: thisComputer,
			withoutSetupKey: true, headers: map[string]string{gmTokenHeader: gm, setupHeader: "1"}})
		return w.Code, strings.TrimSpace(w.Body.String())
	}
	if code, text := pageCall(t, d, "POST", "/setup/hire-folder?folder="+url.QueryEscape(projects)); code != http.StatusOK {
		t.Fatalf("the folder: %d %q", code, text)
	}
	if code, text := pageCall(t, d, "POST", "/setup/hiring?enabled=1"); code != http.StatusOK || !savedConfig().HiringEnabled {
		t.Fatalf("the owner turns hiring on: %d %q", code, text)
	}
	folder := savedConfig().HireFolder
	if code, text := asGM("enabled=0"); code != http.StatusOK || savedConfig().HiringEnabled {
		t.Fatalf("the GM turns hiring off: %d %q, on %v", code, text, savedConfig().HiringEnabled)
	}
	for _, query := range []string{"enabled=1", "enabled=1&folder=" + url.QueryEscape(elsewhere)} {
		code, text := asGM(query)
		want := sweepHiringOffOnly
		if strings.Contains(query, "folder=") {
			want = sweepFolderPageOnly
		}
		if code != http.StatusForbidden || text != want {
			t.Errorf("the GM's %s: %d %q, want 403 %q", query, code, text, want)
		}
		if config := savedConfig(); config.HiringEnabled || config.HireFolder != folder {
			t.Errorf("after the GM's %s: hiring on %v, folder %q", query, config.HiringEnabled, config.HireFolder)
		}
	}
	if code, text := asGM("tmux=0"); code != http.StatusOK {
		t.Errorf("the GM on how hires start: %d %q", code, text)
	}
	if code, text := pageCall(t, d, "POST", "/setup/hiring?enabled=1"); code != http.StatusOK || !savedConfig().HiringEnabled {
		t.Errorf("the owner turns hiring on again: %d %q", code, text)
	}
}

// F3: under the Mac app the bare POST /raise (a second Smolcorp, `smolcorp` run again) shows the office in the app's
// own window, as the menu's Show Office does, and opens no browser, which would mint the office's browser key into
// its history. Off the Mac app it opens the browser, as before.
func TestUnderTheMacAppTheBareRaiseShowsTheOfficeInItsWindow(t *testing.T) {
	isolate(t)
	mac := withMac(t)
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	var opened []string
	saved := openURL
	openURL = func(link string) error { opened = append(opened, link); return nil }
	t.Cleanup(func() { openURL = saved })
	raise := func() int {
		return serve(t, d, call{method: "POST", target: raisePath, host: "127.0.0.1:7710", from: thisComputer,
			headers: map[string]string{hookKeyHeader: d.access.HookKey}}).Code
	}
	if code := raise(); code != http.StatusNoContent || len(mac.requests("office")) != 1 || len(opened) != 0 {
		t.Errorf("under the Mac app: %d, the app's window asked %d times, the browser opened %v", code, len(mac.requests("office")), opened)
	}
	d.mu.Lock()
	d.mac.host = nil
	d.mu.Unlock()
	if code := raise(); code != http.StatusNoContent || len(opened) != 1 || !strings.Contains(opened[0], "/office?o=") {
		t.Errorf("off the Mac app: %d, the browser opened %v", code, opened)
	}
}

// F3: under the Mac app, the office in a browser (its key the browser's history keeps) takes no hire over or back,
// lets none go, and opens no hire's window and no T3 Code: those are the Mac app's own window's. The window is let on
// to the route (each answers it as before: no such desk here), and off the Mac app the browser is too.
func TestUnderTheMacAppTheOfficeInABrowserTakesNoHireOverOrOut(t *testing.T) {
	d := newTestDeck(t)
	d.mu.Lock()
	d.config.T3CodeEnabled = true // so T3 Code's routes are reached; with no such hire, nothing is run or opened
	d.mu.Unlock()
	routes := []string{"/act?action=takeover", "/act?action=handback", "/act?action=letgo", "/hire/window", "/t3code/open", "/t3code"}
	post := func(target, key string) (int, string) {
		w := serve(t, d, call{method: "POST", target: target + sep(target) + "session=nobody&t=" + url.QueryEscape(key),
			host: "localhost:7710", from: thisComputer, withoutSetupKey: true})
		return w.Code, strings.TrimSpace(w.Body.String())
	}
	check := func(where string, browserRefused bool) {
		t.Helper()
		for _, target := range routes {
			if code, text := post(target, d.access.WindowKey); text == sweepOfficeWindow {
				t.Errorf("%s, the window: POST %s: %d %q", where, target, code, text)
			}
			code, text := post(target, d.access.BrowserKey)
			if refused := code == http.StatusForbidden && text == sweepOfficeWindow; refused != browserRefused {
				t.Errorf("%s, the office in a browser: POST %s: %d %q, refused %v", where, target, code, text, browserRefused)
			}
		}
	}
	check("off the Mac app", false)
	hostedByTheMacApp(d)
	check("under the Mac app", true)
}

// sep is what joins a query to target.
func sep(target string) string {
	if strings.Contains(target, "?") {
		return "&"
	}
	return "?"
}
