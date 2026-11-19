package desktop

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Round 1 on RT126SWEEP3 part 4 (FXSWEEP3P4R1). As in sweep3p4_test.go, these rows use only what the tree had
// before the round, with the words written out, so they fail there.

// What an ask no program holds any more is told (askprogram.go's askNoAsker): N2.
const sweepNoAskerWords = "Smolcorp couldn't see which program asked, so it didn't take the ask. " +
	"Run the command again in your own terminal."

// secondCopy runs a second Smolcorp against the running deck d, as `smolcorp` run again does: what it printed, and
// its exit code.
func secondCopy(t *testing.T, d *Deck) (string, int) {
	t.Helper()
	server := httptest.NewServer(&router{deck: d})
	t.Cleanup(server.Close)
	savedPort := Port
	t.Cleanup(func() { Port = savedPort })
	Port, _ = strconv.Atoi(server.URL[strings.LastIndex(server.URL, ":")+1:])
	writeFile(t, hookKeyFile, d.access.HookKey+"\n")
	savedStderr := os.Stderr
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = write
	code := handOverToRunning(true)
	os.Stderr = savedStderr
	write.Close()
	out, _ := io.ReadAll(read)
	return string(out), code
}

// O2: under the Mac app, when the app can't show its office window, /raise says the window didn't open, and so does
// the second Smolcorp: no browser was tried, so neither names one, nor an address to pair a browser at.
func TestUnderTheMacAppAFailedRaiseSaysTheWindowDidntOpen(t *testing.T) {
	isolate(t)
	mac := withMac(t)
	const why = "the office window wasn't made"
	mac.mu.Lock()
	mac.refuse = map[string]string{"office": why}
	mac.mu.Unlock()
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	var opened []string
	saved := openURL
	openURL = func(link string) error { opened = append(opened, link); return nil }
	t.Cleanup(func() { openURL = saved })

	raised := serve(t, d, call{method: "POST", target: raisePath, host: "127.0.0.1:7710", from: thisComputer,
		headers: map[string]string{hookKeyHeader: d.access.HookKey}})
	if want := "The office window could not be opened: " + why + "\n"; raised.Code != http.StatusBadGateway || raised.Body.String() != want {
		t.Errorf("/raise: %d %q, want 502 %q", raised.Code, raised.Body.String(), want)
	}
	out, code := secondCopy(t, d)
	want := "Smolcorp is already running, but its office window didn't open (" + why + ").\n" +
		"To open it, choose Show Office from Smolcorp's menu bar icon." + stopHint + "\n"
	if out != want || code != 1 {
		t.Errorf("the second copy printed %q (exit %d), want %q (exit 1)", out, code, want)
	}
	for _, said := range []string{raised.Body.String(), out} {
		if lower := strings.ToLower(said); strings.Contains(lower, "browser") || strings.Contains(lower, "pair") {
			t.Errorf("under the Mac app, %q names a browser", said)
		}
	}
	if len(opened) != 0 || len(mac.requests("office")) != 2 {
		t.Errorf("the browser opened %v; the app's window asked %d times", opened, len(mac.requests("office")))
	}
}

// O2, the other side: off the Mac app a failed /raise still says the browser didn't start, and the second Smolcorp
// gives the office's address to open by hand, as before.
func TestOffTheMacAppAFailedRaiseStillSaysTheBrowser(t *testing.T) {
	d := newTestDeck(t)
	const why = "xdg-open: exit status 3"
	saved := showOffice
	showOffice = func(*Deck) error { return errors.New(why) } // so no window opens on Windows, where it's the office's
	t.Cleanup(func() { showOffice = saved })
	raised := serve(t, d, call{method: "POST", target: raisePath, host: "127.0.0.1:7710", from: thisComputer,
		headers: map[string]string{hookKeyHeader: d.access.HookKey}})
	if want := "The browser could not be started: " + why + "\n"; raised.Code != http.StatusBadGateway || raised.Body.String() != want {
		t.Errorf("/raise: %d %q, want 502 %q", raised.Code, raised.Body.String(), want)
	}
	out, code := secondCopy(t, d)
	want := "Smolcorp is already running, but no browser could open its office (" + why + ").\n" +
		"To open it by hand, go to: http://localhost:" + strconv.Itoa(Port) + "/office\n" +
		"It may ask you to pair this browser first." + stopHint + "\n"
	if out != want || code != 1 {
		t.Errorf("the second copy printed %q (exit %d), want %q (exit 1)", out, code, want)
	}
}
