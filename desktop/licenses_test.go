package desktop

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// anAppWithNotices is a Smolcorp.app's shape in a temp folder: the program in Resources/bin, the notices in
// Resources/licenses, and the program named as the one running.
func anAppWithNotices(t *testing.T, notices string) {
	t.Helper()
	resources := filepath.Join(t.TempDir(), "Smolcorp.app", "Contents", "Resources")
	writeFile(t, filepath.Join(resources, "bin", "smolcorp"), "#!/bin/sh\n")
	if notices != "" {
		writeFile(t, filepath.Join(resources, "licenses", licensesName), notices)
	}
	saved := executable
	t.Cleanup(func() { executable = saved })
	executable = func() (string, error) { return filepath.Join(resources, "bin", "smolcorp"), nil }
}

// /licenses is the notices file as it is, as plain text, for a phone on the Wi-Fi with no key as much as for this
// computer: it holds nothing of yours.
func TestTheLicensesAreServedAsTheFileToAnyone(t *testing.T) {
	notices := "tmux is ISC licensed.\n\nlibevent is BSD licensed.\n"
	anAppWithNotices(t, notices)
	d := newTestDeck(t)
	for _, c := range []call{
		{method: "GET", target: "/licenses", host: "localhost:7710", from: thisComputer},
		{method: "GET", target: "/licenses", host: deckOnWiFi, from: aPhone},
	} {
		got := serve(t, d, c)
		if got.Code != http.StatusOK || got.Body.String() != notices {
			t.Errorf("%s from %s: %d %q", c.target, c.from, got.Code, got.Body.String())
		}
		if kind := got.Header().Get("Content-Type"); kind != "text/plain; charset=utf-8" {
			t.Errorf("the notices are %q, want plain text", kind)
		}
		if got.Header().Get("X-Content-Type-Options") != "nosniff" || got.Header().Get("X-Frame-Options") != "DENY" {
			t.Errorf("the notices can be sniffed or framed: %v", got.Header())
		}
	}
}

// The route serves the one file and nothing a request names: a query, a path beneath it or another method reach
// no other file, and the route is for GET only.
func TestTheLicensesRouteServesThatFileAndNothingElse(t *testing.T) {
	anAppWithNotices(t, "the notices\n")
	d := newTestDeck(t)
	secret := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, secret, "not for anyone\n")
	for _, c := range []call{
		{method: "GET", target: "/licenses?file=" + secret, host: "localhost:7710", from: thisComputer},
		{method: "GET", target: "/licenses?path=../../../etc/passwd", host: "localhost:7710", from: thisComputer},
	} {
		got := serve(t, d, c)
		if got.Code != http.StatusOK || got.Body.String() != "the notices\n" {
			t.Errorf("%s: %d %q", c.target, got.Code, got.Body.String())
		}
	}
	for _, c := range []call{
		{method: "GET", target: "/licenses/../setup", host: deckOnWiFi, from: aPhone},
		{method: "GET", target: "/licenses/THIRD-PARTY-NOTICES.txt", host: deckOnWiFi, from: aPhone},
		{method: "POST", target: "/licenses", host: deckOnWiFi, from: aPhone},
		{method: "PUT", target: "/licenses", host: deckOnWiFi, from: aPhone},
	} {
		got := serve(t, d, c)
		if got.Code != http.StatusUnauthorized || strings.Contains(got.Body.String(), "notices") {
			t.Errorf("%s %s: %d %q, want the same 401 as any route that isn't public", c.method, c.target, got.Code, got.Body.String())
		}
	}
}

// A copy with no notices (built from the source folder) says so in a line, rather than an empty page or an error.
func TestTheLicensesRouteSaysWhenThisCopyHasNone(t *testing.T) {
	anAppWithNotices(t, "")
	d := newTestDeck(t)
	got := serve(t, d, call{method: "GET", target: "/licenses", host: "localhost:7710", from: thisComputer})
	if got.Code != http.StatusNotFound || !strings.Contains(got.Body.String(), "aren't in this copy") {
		t.Fatalf("/licenses with no notices: %d %q", got.Code, got.Body.String())
	}
}

// A Linux or Windows download keeps them beside the program, with the licence.
func TestTheLicensesRouteFindsNoticesBesideTheProgram(t *testing.T) {
	folder := t.TempDir()
	writeFile(t, filepath.Join(folder, licensesName), "beside the program\n")
	saved := executable
	t.Cleanup(func() { executable = saved })
	executable = func() (string, error) { return filepath.Join(folder, "smolcorp"), nil }
	d := newTestDeck(t)
	got := serve(t, d, call{method: "GET", target: "/licenses", host: "localhost:7710", from: thisComputer})
	if got.Code != http.StatusOK || got.Body.String() != "beside the program\n" {
		t.Fatalf("/licenses beside the program: %d %q", got.Code, got.Body.String())
	}
}

// Setup links to the notices in its Getting started pane, and the Mac's build script puts them in the app.
func TestSetupLinksToTheNoticesAndTheAppScriptShipsThem(t *testing.T) {
	setup, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(setup), `<a href="/licenses" target="_blank" rel="noopener noreferrer">Open-source notices</a>`) {
		t.Error("setup has no Open-source notices link to /licenses")
	}
	script, err := os.ReadFile("../scripts/build-app.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"$APP/Contents/Resources/bin/tmux"`, `Contents/Resources/licenses/THIRD-PARTY-NOTICES.txt`,
		`--identifier io.github.sauzze.smolcorp-tmux`,
	} {
		if !strings.Contains(string(script), want) {
			t.Errorf("scripts/build-app.sh has no %q", want)
		}
	}
	release, err := os.ReadFile("../scripts/release.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`--options runtime --timestamp --sign "$DEVELOPER_ID" --identifier io.github.sauzze.smolcorp-tmux`,
		`codesign --verify --deep --strict --verbose=2 "$TMUX_FILE"`,
	} {
		if !strings.Contains(string(release), want) {
			t.Errorf("scripts/release.sh has no %q", want)
		}
	}
	if strings.Contains(string(release), `--entitlements packaging/macos/Smolcorp.entitlements "$TMUX_FILE"`) {
		t.Error("scripts/release.sh gives tmux entitlements, which it doesn't need")
	}
}
