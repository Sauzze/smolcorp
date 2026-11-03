package desktop

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestOnlyOneInstanceHoldsTheLock(t *testing.T) {
	isolate(t)
	if instanceName != "" { // Windows: a name of its own, so a running Smolcorp doesn't count
		saved := instanceName
		instanceName = `Local\Smolcorp-test-` + strconv.Itoa(int(nowMillis()))
		t.Cleanup(func() { instanceName = saved })
	}
	if running, err := acquireInstance(); running || err != nil {
		t.Fatalf("the first instance: running=%v err=%v", running, err)
	}
	if running, err := acquireInstance(); !running || err != nil {
		t.Fatalf("the second instance: running=%v err=%v, want running", running, err)
	}
}

func TestRaiseIsAskedOnlyByThisComputerWithTheHookKey(t *testing.T) {
	d := newTestDeck(t)
	shown := 0
	saved := showOffice
	showOffice = func(*Deck) error { shown++; return nil }
	t.Cleanup(func() { showOffice = saved })
	phoneKey := pair(t, d)
	raise := func(method, from, host, key string) int {
		return serve(t, d, call{method: method, target: raisePath, host: host, from: from,
			headers: map[string]string{hookKeyHeader: key}}).Code
	}
	for _, c := range []struct {
		name, method, from, host, key string
		want                          int
	}{
		{"no key", "POST", thisComputer, "127.0.0.1:7710", "", http.StatusUnauthorized},
		{"a phone's key", "POST", thisComputer, "127.0.0.1:7710", phoneKey, http.StatusUnauthorized},
		{"another device", "POST", aPhone, deckOnWiFi, d.access.HookKey, http.StatusNotFound},
		{"a page rebound to this computer", "POST", thisComputer, "evil.example:7710", d.access.HookKey, http.StatusNotFound},
		{"GET", "GET", thisComputer, "127.0.0.1:7710", d.access.HookKey, http.StatusNotFound},
	} {
		if got := raise(c.method, c.from, c.host, c.key); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
	if shown != 0 {
		t.Fatalf("a refused request showed the office %d times", shown)
	}
	if got := raise("POST", thisComputer, "127.0.0.1:7710", d.access.HookKey); got != http.StatusNoContent || shown != 1 {
		t.Fatalf("the hook's own request: %d, shown %d times", got, shown)
	}
}

func TestASecondInstanceAsksTheFirstToShowTheOffice(t *testing.T) {
	d := newTestDeck(t)
	shown := 0
	saved := showOffice
	showOffice = func(*Deck) error { shown++; return nil }
	t.Cleanup(func() { showOffice = saved })
	server := httptest.NewServer(&router{deck: d})
	defer server.Close()
	address, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(address.Port())

	// No hook key on disk: nothing to ask with.
	if err := os.Remove(hookKeyFile); err != nil {
		t.Fatal(err)
	}
	if answered, _ := askRunningToShowOffice(port); answered {
		t.Fatal("asked with no key on disk")
	}
	writeFile(t, hookKeyFile, d.access.HookKey+"\n")
	if answered, failure := askRunningToShowOffice(port); !answered || failure != "" || shown != 1 {
		t.Fatalf("the running Smolcorp wasn't asked (shown %d)", shown)
	}
	// An old Smolcorp, or another program, on that port: false, and the user is told instead.
	writeFile(t, hookKeyFile, "not-the-key\n")
	if answered, _ := askRunningToShowOffice(port); answered || shown != 1 {
		t.Fatalf("a wrong key was taken for an answer (shown %d)", shown)
	}
	if answered, _ := askRunningToShowOffice(1); answered {
		t.Fatal("nothing listens on port 1")
	}
}

// handOver runs the second instance against a fake first one whose opener says
// err (nil: the browser opened), and returns what it printed and its exit code.
func handOver(t *testing.T, opener error) (string, int) {
	t.Helper()
	d := newTestDeck(t)
	saved, savedPort := showOffice, Port
	showOffice = func(*Deck) error { return opener }
	t.Cleanup(func() { showOffice, Port = saved, savedPort })
	server := httptest.NewServer(&router{deck: d})
	t.Cleanup(server.Close)
	address, _ := url.Parse(server.URL)
	Port, _ = strconv.Atoi(address.Port())
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

func TestASecondInstanceSaysTheOfficeIsComingWhenTheBrowserOpened(t *testing.T) {
	out, code := handOver(t, nil)
	want := instanceRaised + stopHint + "\n"
	if out != want || code != 0 {
		t.Fatalf("printed %q (exit %d), want %q (exit 0)", out, code, want)
	}
}

func TestASecondInstanceSaysWhatHappenedWhenNoBrowserOpened(t *testing.T) {
	reason := "exec: \"xdg-open\": executable file not found in $PATH"
	out, code := handOver(t, errors.New(reason))
	// Port is the fake first copy's random port here; it is named literally.
	want := "Smolcorp is already running, but no browser could open its office (" + reason + ").\n" +
		"To open it by hand, go to: http://localhost:" + strconv.Itoa(Port) + "/office\n" +
		"It may ask you to pair this browser first." + stopHint + "\n"
	if out != want || code != 1 {
		t.Fatalf("printed %q (exit %d), want %q (exit 1)", out, code, want)
	}
	if strings.Contains(out, "coming to the front") {
		t.Fatal("said the office was coming to the front")
	}
}

func TestTheNoBrowserMessageNamesTheDefaultPortAndKeepsTheAddressAlone(t *testing.T) {
	saved := Port
	Port = 7710 // only named in text; nothing binds it
	t.Cleanup(func() { Port = saved })
	want := "Smolcorp is already running, but no browser could open its office (boom).\n" +
		"To open it by hand, go to: http://localhost:7710/office\n" +
		"It may ask you to pair this browser first."
	if got := notRaisedMessage("boom"); got != want {
		t.Fatalf("message %q, want %q", got, want)
	}
	for _, line := range strings.Split(want, "\n") {
		if strings.HasSuffix(line, "/office") && !strings.HasPrefix(line, "To open it by hand, go to: ") {
			t.Fatalf("the address line is %q", line)
		}
	}
}

func TestRaiseAnswers502WhenTheBrowserCannotBeStarted(t *testing.T) {
	d := newTestDeck(t)
	saved := showOffice
	showOffice = func(*Deck) error { return errors.New("xdg-open: exit status 3") }
	t.Cleanup(func() { showOffice = saved })
	got := serve(t, d, call{method: "POST", target: raisePath, host: "127.0.0.1:7710", from: thisComputer,
		headers: map[string]string{hookKeyHeader: d.access.HookKey}})
	if got.Code != http.StatusBadGateway {
		t.Fatalf("status %d, want 502", got.Code)
	}
	if want := "The browser could not be started: xdg-open: exit status 3\n"; got.Body.String() != want {
		t.Fatalf("body %q, want %q", got.Body.String(), want)
	}
}

func TestRaiseWithAWrongKeyIs401EvenWhenTheBrowserCannotBeStarted(t *testing.T) {
	d := newTestDeck(t)
	opened := 0
	saved := showOffice
	showOffice = func(*Deck) error { opened++; return errors.New("xdg-open: exit status 3") }
	t.Cleanup(func() { showOffice = saved })
	got := serve(t, d, call{method: "POST", target: raisePath, host: "127.0.0.1:7710", from: thisComputer,
		headers: map[string]string{hookKeyHeader: "not-the-key"}})
	if got.Code != http.StatusUnauthorized || opened != 0 {
		t.Fatalf("status %d, opener called %d times, want 401 and 0", got.Code, opened)
	}
	if strings.Contains(got.Body.String(), "browser") {
		t.Fatalf("body %q says something about the browser", got.Body.String())
	}
}

// With no desktop, setup --open gives the office's plain address, and no token or key.
func TestSetupOpenWithNoDesktopGivesTheOfficeAddress(t *testing.T) {
	saved, savedSystem := setupHasDesktop, setupSystem
	t.Cleanup(func() { setupHasDesktop, setupSystem = saved, savedSystem })
	setupSystem = "linux"
	var out strings.Builder
	setupHasDesktop = func() bool { return false }
	setupClient{base: "http://localhost:7799"}.openSetup(&out)
	if !strings.Contains(out.String(), "To open it by hand, go to: http://localhost:7799/office\nIt may ask you to pair this browser first.") {
		t.Errorf("no desktop: %q", out.String())
	}
	if strings.Contains(out.String(), "?o=") || strings.Contains(out.String(), "#o=") {
		t.Errorf("a token in %q", out.String())
	}
	out.Reset()
	setupHasDesktop = func() bool { return true }
	setupClient{base: "http://localhost:7799"}.openSetup(&out)
	if strings.Contains(out.String(), "/office") {
		t.Errorf("with a desktop: %q", out.String())
	}
}
