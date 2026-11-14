package desktop

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// withTailnet has Tailscale's command line answer found from now on, never the real one, and forgets the last answer.
func withTailnet(t *testing.T, found tailnetAnswer, installed bool) {
	t.Helper()
	saved := askTailnet
	askTailnet = func() (tailnetAnswer, bool) { return found, installed }
	forget := func() {
		tailnetLook.mu.Lock()
		tailnetLook.answer, tailnetLook.lookedAt, tailnetLook.looking = tailnetAnswer{}, time.Time{}, false
		tailnetLook.mu.Unlock()
	}
	forget()
	t.Cleanup(func() {
		askTailnet = saved
		forget()
	})
}

// phoneMicSaid is setup's phoneMic, as the page reads it.
func phoneMicSaid(t *testing.T, d *Deck) map[string]any {
	t.Helper()
	code, body := pageCall(t, d, "GET", "/setup/state")
	var state map[string]any
	if code != http.StatusOK || json.Unmarshal([]byte(body), &state) != nil {
		t.Fatalf("the setup state: %d", code)
	}
	mic, _ := state["phoneMic"].(map[string]any)
	return mic
}

// Setup's Phone microphone under the Mac app, as the Swift engine's: the switch and whether an older page's text is
// sent, kept in config.json; with it on, this Mac's tailnet address as Tailscale says it; and whether Tailscale hands
// the deck on, said whatever the switch. Off, a tailnet name is refused again at once.
func TestSetupsPhoneMicSwitchUnderTheMacApp(t *testing.T) {
	serving := true
	withTailnet(t, tailnetAnswer{url: "https://my-mac.tail1234.ts.net", serving: &serving}, true)
	withMac(t)
	d := newTestDeck(t)
	mic := phoneMicSaid(t, d)
	if mic["enabled"] != true || mic["autoSend"] != false {
		t.Fatalf("phoneMic: %v", mic)
	}
	eventually(t, "Tailscale asked", func() bool { return phoneMicSaid(t, d)["tailnet"] == "https://my-mac.tail1234.ts.net" })
	if mic := phoneMicSaid(t, d); mic["serving"] != true {
		t.Errorf("serving: %v", mic["serving"])
	}

	if code, _ := pageCall(t, d, "POST", "/setup/phonemic?autosend=1"); code != http.StatusOK || !loadConfig().PhoneMicAutoSend || !loadConfig().PhoneMicEnabled {
		t.Errorf("autosend: %d, %+v", code, loadConfig())
	}
	code, body := pageCall(t, d, "POST", "/setup/phonemic?enabled=0")
	var state struct{ PhoneMic map[string]any }
	_ = json.Unmarshal([]byte(body), &state)
	if code != http.StatusOK || loadConfig().PhoneMicEnabled || state.PhoneMic["enabled"] != false || state.PhoneMic["tailnet"] != nil ||
		state.PhoneMic["serving"] != true || state.PhoneMic["autoSend"] != true {
		t.Errorf("off: %d %v", code, state.PhoneMic)
	}
	viaTailnet := call{method: "GET", target: "/office", host: "my-mac.tail1234.ts.net", from: "127.0.0.1:50000"}
	if got := serve(t, d, viaTailnet); got.Code != http.StatusForbidden {
		t.Errorf("a tailnet name with the phone mic off: %d", got.Code)
	}
	// A tool may switch it too, as on the Mac (gm, page): never with no key at all.
	if got := serve(t, d, call{method: "POST", target: "/setup/phonemic?enabled=1", host: "localhost:7710", from: thisComputer, withoutSetupKey: true}); got.Code == http.StatusOK {
		t.Error("switched with no key")
	}
}

// The QR code at this Mac's tailnet address pairs with the same code, only with the phone mic on and an address.
func TestTheTailnetQRCodeNeedsThePhoneMicAndAnAddress(t *testing.T) {
	withTailnet(t, tailnetAnswer{url: "https://my-mac.tail1234.ts.net"}, true)
	withMac(t)
	d := newTestDeck(t)
	if code, _ := pageCall(t, d, "GET", "/setup/qr.png?via=tailnet"); code != http.StatusNotFound {
		t.Errorf("no code: %d", code)
	}
	d.mu.Lock()
	d.access.NewCode()
	key := d.access.PairingKey()
	d.mu.Unlock()
	// The key the code hands over goes after #, never sent (SECWIFI).
	eventually(t, "the address", func() bool { return d.tailnetPairingLink(key) == "https://my-mac.tail1234.ts.net/#pair="+key })
	if status, _ := pageCall(t, d, "GET", "/setup/qr.png?via=tailnet"); status != http.StatusOK {
		t.Errorf("with a code and an address: %d", status)
	}
	pageCall(t, d, "POST", "/setup/phonemic?enabled=0")
	if status, _ := pageCall(t, d, "GET", "/setup/qr.png?via=tailnet"); status != http.StatusNotFound {
		t.Errorf("the phone mic off: %d", status)
	}
}

// Without the Mac app setup has no Phone microphone section: no phoneMic, and its route is 404.
func TestWithoutTheMacAppThereIsNoPhoneMicSection(t *testing.T) {
	withTailnet(t, tailnetAnswer{url: "https://my-pc.tail1234.ts.net"}, true)
	saved := host
	host = nil
	t.Cleanup(func() { host = saved })
	d := newTestDeck(t)
	if mic := phoneMicSaid(t, d); mic != nil {
		t.Errorf("phoneMic: %v", mic)
	}
	if code, _ := pageCall(t, d, "POST", "/setup/phonemic?enabled=0"); code != http.StatusNotFound || !loadConfig().PhoneMicEnabled {
		t.Errorf("the route: %d", code)
	}
	if code, _ := pageCall(t, d, "GET", "/setup/qr.png?via=tailnet"); code != http.StatusNotFound {
		t.Errorf("the tailnet QR code: %d", code)
	}
}

// Tailscale's answers as the Mac's Tailnet reads them: the MagicDNS name, and whether `tailscale serve` hands the
// deck on, by a proxy or a TCP forward to its port on this computer.
func TestTailscalesAnswersAreReadAsTheMacReadsThem(t *testing.T) {
	for status, want := range map[string]string{
		`{"Self":{"DNSName":"My-Mac.tail1234.ts.net."}}`: "https://my-mac.tail1234.ts.net",
		`{"Self":{"DNSName":"my-mac.example.com."}}`:     "",
		`{"Self":{}}`: "",
		`not json`:    "",
	} {
		if got := tailnetURL([]byte(status)); got != want {
			t.Errorf("%s: %q, want %q", status, got, want)
		}
	}
	for status, want := range map[string]any{
		`{"Web":{"my-mac.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:7710"}}}}}`:  true,
		`{"Web":{"my-mac.ts.net:443":{"Handlers":{"/":{"Proxy":"http://localhost:7710/"}}}}}`: true,
		`{"TCP":{"443":{"TCPForward":"[::1]:7710"}}}`:                                         true,
		`{"Web":{"my-mac.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:3000"}}}}}`:  false,
		`{}`:     false,
		`[]`:     nil,
		`broken`: nil,
	} {
		got := servesDeck([]byte(status), 7710)
		switch {
		case want == nil && got != nil, want != nil && (got == nil || *got != want):
			t.Errorf("%s: %v, want %v", status, got, want)
		}
	}
	// Since SN1, on a Mac, a proxy to this deck's serve.sock (`unix:` and its path, exactly), and nothing else.
	const socket = "/Users/me/Library/Application Support/AgentDeck/serve.sock"
	for status, want := range map[string]any{
		`{"Web":{"my-mac.ts.net:443":{"Handlers":{"/":{"Proxy":"unix:/Users/me/Library/Application Support/AgentDeck/serve.sock"}}}}}`:   true,
		`{"Web":{"my-mac.ts.net:443":{"Handlers":{"/":{"Proxy":"unix:/Users/me/Library/Application Support/AgentDeck/./serve.sock"}}}}}`: true,
		`{"Web":{"my-mac.ts.net:443":{"Handlers":{"/":{"Proxy":"unix:/Users/me/other.sock"}}}}}`:                                         false,
		`{"Web":{"my-mac.ts.net:443":{"Handlers":{"/":{"Proxy":"unix:"}}}}}`:                                                             false,
		`{"Web":{"my-mac.ts.net:443":{"Handlers":{"/":{"Proxy":"/Users/me/Library/Application Support/AgentDeck/serve.sock"}}}}}`:        false,
		`{"Web":{"my-mac.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:7710"}}}}}`:                                             false,
		`{}`:     false,
		`broken`: nil,
	} {
		got := servesSocket([]byte(status), socket)
		switch {
		case want == nil && got != nil, want != nil && (got == nil || *got != want):
			t.Errorf("%s: %v, want %v", status, got, want)
		}
	}
}
