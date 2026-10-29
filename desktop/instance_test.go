package desktop

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
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
	showOffice = func(*Deck) { shown++ }
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
	showOffice = func(*Deck) { shown++ }
	t.Cleanup(func() { showOffice = saved })
	server := httptest.NewServer(&router{deck: d})
	defer server.Close()
	address, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(address.Port())

	// No hook key on disk: nothing to ask with.
	if err := os.Remove(hookKeyFile); err != nil {
		t.Fatal(err)
	}
	if askRunningToShowOffice(port) {
		t.Fatal("asked with no key on disk")
	}
	writeFile(t, hookKeyFile, d.access.HookKey+"\n")
	if !askRunningToShowOffice(port) || shown != 1 {
		t.Fatalf("the running Smolcorp wasn't asked (shown %d)", shown)
	}
	// An old Smolcorp, or another program, on that port: false, and the user is told instead.
	writeFile(t, hookKeyFile, "not-the-key\n")
	if askRunningToShowOffice(port) || shown != 1 {
		t.Fatalf("a wrong key was taken for an answer (shown %d)", shown)
	}
	if askRunningToShowOffice(1) {
		t.Fatal("nothing listens on port 1")
	}
}
