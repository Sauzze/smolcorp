package desktop

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Another phone on the same Wi-Fi: someone who sniffed the first.
const anotherPhone = "192.168.1.30:50000"

// signedTarget is target as a phone on the Wi-Fi sends it (sign.js): the key in its ?t= taken out, and the
// rest signed with it, at the time given. A target with no ?t= is left as it is.
func signedTarget(method, target, body string, at time.Time) string {
	path, raw, _ := strings.Cut(target, "?")
	var key string
	var kept []string
	for _, part := range strings.Split(raw, "&") {
		if value, found := strings.CutPrefix(part, "t="); found {
			key, _ = url.QueryUnescape(value)
		} else if part != "" {
			kept = append(kept, part)
		}
	}
	if key == "" {
		return target
	}
	nonce := randomKey()[:22]
	kept = append(kept, "ts="+strconv.FormatInt(at.UnixMilli(), 10), "n="+nonce)
	return signWith(key, method, path, strings.Join(kept, "&"), body)
}

// signWith is path?query signed with key, as sign.js signs it.
func signWith(key, method, path, query, body string) string {
	secret := sha256.Sum256([]byte(key))
	bodySum := sha256.Sum256([]byte(body))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(strings.Join([]string{"smolcorp-v1", method, path, query, hex.EncodeToString(bodySum[:])}, "\n")))
	return path + "?" + query + "&sig=" + hex.EncodeToString(mac.Sum(nil))
}

// signedRefusalOf is a signed request's refusal: its reason, and the computer's time.
func signedRefusalOf(t *testing.T, w interface{ Bytes() []byte }) (why string, now int64) {
	t.Helper()
	var answer struct {
		Why string
		Now int64
	}
	_ = json.Unmarshal(w.Bytes(), &answer)
	return answer.Why, answer.Now
}

// A request sniffed on the Wi-Fi and sent again, as it was, does nothing: its nonce works once.
func TestAReplayedRequestIsRefused(t *testing.T) {
	d := newTestDeck(t)
	key := pair(t, d)
	target := signedTarget("POST", "/focus?session=nobody&t="+key, "", time.Now())
	first := serve(t, d, call{method: "POST", target: target, host: deckOnWiFi, from: aPhone})
	if first.Code != http.StatusNotFound {
		t.Fatalf("a signed request: %d %s, want it to reach the deck (404 for no session)", first.Code, first.Body)
	}
	again := serve(t, d, call{method: "POST", target: target, host: deckOnWiFi, from: anotherPhone})
	if why, _ := signedRefusalOf(t, again.Body); again.Code != http.StatusUnauthorized || why != signedReplay {
		t.Errorf("the same request sent again: %d %q, want 401 %q", again.Code, why, signedReplay)
	}
	// Typed text too, whether it was taken or not.
	typed := signedTarget("POST", "/dictate?session=nobody&t="+key, `{"text":"rm -rf"}`, time.Now())
	serve(t, d, call{method: "POST", target: typed, host: deckOnWiFi, from: aPhone, body: `{"text":"rm -rf"}`})
	if replayed := serve(t, d, call{method: "POST", target: typed, host: deckOnWiFi, from: aPhone, body: `{"text":"rm -rf"}`}); replayed.Code != http.StatusUnauthorized {
		t.Errorf("typed text sent again: %d, want 401", replayed.Code)
	}
}

// A request signed more than a minute from now, either way, is refused, with this computer's time for the page.
func TestAnOldTimestampIsRefused(t *testing.T) {
	d := newTestDeck(t)
	key := pair(t, d)
	for _, off := range []time.Duration{-2 * time.Minute, -signedWindow - time.Second, 2 * time.Minute} {
		target := signedTarget("POST", "/focus?session=nobody&t="+key, "", time.Now().Add(off))
		got := serve(t, d, call{method: "POST", target: target, host: deckOnWiFi, from: aPhone})
		why, now := signedRefusalOf(t, got.Body)
		if got.Code != http.StatusUnauthorized || why != signedClock {
			t.Errorf("signed %v from now: %d %q, want 401 %q", off, got.Code, why, signedClock)
		}
		if gap := time.Since(time.UnixMilli(now)); gap < 0 || gap > time.Minute {
			t.Errorf("signed %v from now: the refusal says the time is %d", off, now)
		}
	}
	// Within the minute, it's taken, once the app has been running that long.
	d.mu.Lock()
	d.access.started = time.Now().Add(-time.Hour)
	d.mu.Unlock()
	target := signedTarget("POST", "/focus?session=nobody&t="+key, "", time.Now().Add(-30*time.Second))
	if got := serve(t, d, call{method: "POST", target: target, host: deckOnWiFi, from: aPhone}); got.Code != http.StatusNotFound {
		t.Errorf("signed 30 s ago: %d, want it taken (404 for no session)", got.Code)
	}
}

// A signature that isn't the phone's key's, or that doesn't cover what's sent, is refused.
func TestABadSignatureIsRefused(t *testing.T) {
	d := newTestDeck(t)
	key := pair(t, d)
	good := signedTarget("POST", "/dictate?session=nobody&t="+key, `{"text":"hello"}`, time.Now())
	flipped := good[:len(good)-1] + map[bool]string{true: "0", false: "1"}[strings.HasSuffix(good, "1")]
	query := "session=nobody&ts=" + strconv.FormatInt(time.Now().UnixMilli(), 10) + "&n=" + randomKey()[:22]
	for name, c := range map[string]call{
		"a flipped signature": {target: flipped, body: `{"text":"hello"}`},
		"another key":         {target: signWith("not-the-phones-key", "POST", "/dictate", query, `{"text":"hello"}`), body: `{"text":"hello"}`},
		"another body":        {target: good, body: `{"text":"rm -rf ~"}`},
		"another session":     {target: strings.Replace(good, "session=nobody", "session=s1", 1), body: `{"text":"hello"}`},
		"another path":        {target: strings.Replace(good, "/dictate", "/choose", 1), body: `{"text":"hello"}`},
		"no nonce":            {target: signWith(key, "POST", "/dictate", "session=nobody&ts="+strconv.FormatInt(time.Now().UnixMilli(), 10), ""), body: ""},
		"the key inside":      {target: signWith(key, "POST", "/dictate", query+"&t="+key, ""), body: ""},
	} {
		c.method, c.host, c.from, c.bare = "POST", deckOnWiFi, aPhone, true
		got := serve(t, d, c)
		if why, _ := signedRefusalOf(t, got.Body); got.Code != http.StatusUnauthorized || why != signedBad {
			t.Errorf("%s: %d %q, want 401 %q", name, got.Code, why, signedBad)
		}
	}
}

// Over the Wi-Fi, the key itself, as an old page sent it, is no key at all; on this computer (the screen
// saver, the office's own window) it still is.
func TestTheKeyItselfIsRefusedOverTheWiFi(t *testing.T) {
	d := newTestDeck(t)
	key := pair(t, d)
	for name, c := range map[string]call{
		"in the address": {method: "POST", target: "/focus?session=nobody&t=" + key},
		"in a header":    {method: "POST", target: "/focus?session=nobody", headers: map[string]string{hookKeyHeader: key}},
		"the stream":     {method: "GET", target: "/paired?t=" + key},
	} {
		c.host, c.from, c.bare = deckOnWiFi, aPhone, true
		if got := serve(t, d, c); got.Code != http.StatusUnauthorized {
			t.Errorf("the key itself %s, over the Wi-Fi: %d, want 401", name, got.Code)
		}
	}
	if page := serve(t, d, call{method: "GET", target: "/?t=" + key, host: deckOnWiFi, from: aPhone, bare: true}).Body.String(); !strings.Contains(page, "Pair with your computer") {
		t.Error("a page saved with its key got the deck over the Wi-Fi")
	}
	if got := serve(t, d, call{method: "GET", target: "/paired", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{hookKeyHeader: key}}); got.Code != http.StatusNoContent {
		t.Errorf("the screen saver's key on this computer: %d, want 204", got.Code)
	}
}

// What a paired phone sends never holds its key: the page reads it from after # and signs every request with
// it. Checked in the page's source, and by running sign.js in node, where there is node, against this router.
func TestThePhonesRequestsNeverCarryTheKey(t *testing.T) {
	deck := readWeb(t, "deck.html")
	if !strings.Contains(deck, `new URLSearchParams(location.hash.slice(1)).get('k')`) || !strings.Contains(deck, `<script src="/sign.js">`) {
		t.Fatal("the deck no longer takes its key from after # and signs with sign.js")
	}
	// Every request goes by address(), which signs whenever the key came after #.
	if n := len(regexp.MustCompile(`fetch\(`).FindAllString(deck, -1)); n != len(regexp.MustCompile(`fetch\(address\(`).FindAllString(deck, -1)) {
		t.Errorf("the deck has %d fetch calls, not all by address()", n)
	}
	if strings.Count(deck, "new EventSource(") != strings.Count(deck, "new EventSource(address(") {
		t.Error("the deck's stream isn't opened by address()")
	}
	if strings.Count(deck, "t: token") != 1 || !strings.Contains(deck, "const token = phoneKey ? '' :") {
		t.Error("the deck can send the key itself when it came after #")
	}
	pairing := readWeb(t, "pair.html")
	if strings.Contains(pairing, "?t=${") || !strings.Contains(pairing, "?deck#k=${encodeURIComponent(key)}") || !strings.Contains(pairing, "location.hash.slice(1)).get('pair')") {
		t.Error("the pairing page puts the key somewhere a browser sends it")
	}

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: sign.js can't be run")
	}
	d := newTestDeck(t)
	key := pair(t, d)
	script := string(readWebBytes(t, "sign.js")) + `
const sign = smolcorpSign.signer(process.argv[2]);
console.log(JSON.stringify([sign('GET', '/events'), sign('POST', '/focus', { session: 'nobody one' }), sign('POST', '/dictate', { session: 'nobody' }, '{"text":"héllo ✓"}')]));`
	file := filepath.Join(t.TempDir(), "sign.js")
	if err := os.WriteFile(file, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, file, key).Output()
	if err != nil {
		t.Fatal(err)
	}
	var sniffed []string
	if err := json.Unmarshal(out, &sniffed); err != nil || len(sniffed) != 3 {
		t.Fatalf("sign.js gave %s", out)
	}
	hash := hashKey(key)
	for _, target := range sniffed {
		if strings.Contains(target, key) || strings.Contains(target, url.QueryEscape(key)) || strings.Contains(target, hash) {
			t.Errorf("a signed request carries the key: %s", target)
		}
	}
	if got := serve(t, d, call{method: "POST", target: sniffed[1], host: deckOnWiFi, from: aPhone}); got.Code != http.StatusNotFound {
		t.Errorf("sign.js's request: %d %s, want it taken (404 for no session)", got.Code, got.Body)
	}
	d.mu.Lock()
	d.config.PhoneTextWiFi = true
	d.mu.Unlock()
	if got := serve(t, d, call{method: "POST", target: sniffed[2], host: deckOnWiFi, from: aPhone, body: `{"text":"héllo ✓"}`}); got.Code != http.StatusNotFound {
		t.Errorf("sign.js's typed text: %d %s, want it taken (404 for no session)", got.Code, got.Body)
	}
}

// The QR code's link holds the key it hands over after # only: never in its path or query, which a browser
// sends and logs keep. It is claimed once, with a part of the phone's own; a typed code, whose answer is the
// key, is refused on the Wi-Fi.
func TestTheQRCarriesTheKeyOnlyInItsFragment(t *testing.T) {
	d := newTestDeck(t)
	serve(t, d, call{method: "POST", target: "/setup/pair", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	d.mu.Lock()
	key, code := d.access.PairingKey(), d.access.ActiveCode()
	d.mu.Unlock()
	link, err := url.Parse(pairingLink(key))
	if err != nil || key == "" {
		t.Fatalf("the QR code's link %v: %v", link, err)
	}
	if link.Fragment != "pair="+key || link.RawQuery != "" || strings.Contains(link.Path, key) || strings.Contains(link.Path, code) {
		t.Errorf("the QR code's link is %s, want the key after # only", link)
	}
	typed := serve(t, d, call{method: "POST", target: "/pair", host: deckOnWiFi, from: aPhone, body: `{"code":"` + code + `"}`})
	if typed.Code != http.StatusForbidden || strings.Contains(typed.Body.String(), "key\"") || !strings.Contains(typed.Body.String(), "scan the QR code") {
		t.Errorf("a typed code over the Wi-Fi: %d %s, want it refused", typed.Code, typed.Body)
	}
	part := randomKey()
	wrong := serve(t, d, call{method: "POST", target: "/pair?p=" + part + "&t=not-the-codes-key", host: deckOnWiFi, from: aPhone})
	noPart := serve(t, d, call{method: "POST", target: "/pair?t=" + key, host: deckOnWiFi, from: aPhone})
	claimed := serve(t, d, call{method: "POST", target: "/pair?p=" + part + "&t=" + key, host: deckOnWiFi, from: aPhone})
	again := serve(t, d, call{method: "POST", target: "/pair?p=" + randomKey() + "&t=" + key, host: deckOnWiFi, from: anotherPhone})
	if wrong.Code != http.StatusForbidden || noPart.Code != http.StatusForbidden || claimed.Code != http.StatusNoContent || again.Code != http.StatusForbidden {
		t.Errorf("claims: a wrong key %d, no part of the phone's own %d, the key %d, the key again %d; want 403, 403, 204, 403",
			wrong.Code, noPart.Code, claimed.Code, again.Code)
	}
	if strings.Contains(claimed.Body.String(), key) {
		t.Error("the claim's answer holds the key")
	}
	// The phone's key is made of the code's and its own part: a photo of the QR code isn't it.
	d.mu.Lock()
	paired, photographed := d.access.Phones.WithKey(phoneKeyFrom(key, part)) != nil, d.access.Phones.WithKey(key) != nil
	d.mu.Unlock()
	if !paired || photographed {
		t.Errorf("after the claim: the phone's own key paired %v, the QR code's key paired %v; want true, false", paired, photographed)
	}
	photo := signedTarget("POST", "/focus?session=nobody&t="+key, "", time.Now())
	if got := serve(t, d, call{method: "POST", target: photo, host: deckOnWiFi, from: anotherPhone}); got.Code != http.StatusUnauthorized {
		t.Errorf("signed with the QR code's key after the claim: %d, want 401", got.Code)
	}
	// sign.js makes the phone's key as this computer does.
	if node, err := exec.LookPath("node"); err == nil {
		file := filepath.Join(t.TempDir(), "derive.js")
		script := string(readWebBytes(t, "sign.js")) + "\nconsole.log(smolcorpSign.phoneKey(process.argv[2], process.argv[3]));"
		if err := os.WriteFile(file, []byte(script), 0o600); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(node, file, key, part).Output(); err != nil || strings.TrimSpace(string(out)) != phoneKeyFrom(key, part) {
			t.Errorf("sign.js's phone key %q (%v), want %q", out, err, phoneKeyFrom(key, part))
		}
	}
	// On this computer (the screen saver), a typed code still pairs.
	serve(t, d, call{method: "POST", target: "/setup/pair", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	d.mu.Lock()
	code = d.access.ActiveCode()
	d.mu.Unlock()
	if local := serve(t, d, call{method: "POST", target: "/pair", host: "localhost:7710", from: thisComputer, body: `{"code":"` + code + `"}`}); local.Code != http.StatusOK {
		t.Errorf("a typed code on this computer: %d, want 200", local.Code)
	}
}

// Typing over Wi-Fi is off for a new setup: the phone's keys work over the Wi-Fi, its text doesn't. A setup
// from before the switch keeps typing; only the page turns it on.
func TestTypingOverWiFiIsOffByDefault(t *testing.T) {
	d := newTestDeck(t)
	key := pair(t, d)
	d.mu.Lock()
	on := d.config.PhoneTextWiFi
	d.mu.Unlock()
	if on {
		t.Fatal("typing over Wi-Fi is on for a new setup")
	}
	state := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer}).Body.String()
	if !strings.Contains(state, `"wifiTyping":false`) {
		t.Error("setup's state doesn't say typing over Wi-Fi is off")
	}
	for _, target := range []string{"/dictate?session=nobody&t=" + key, "/choose?session=nobody&option=other&choice=0&t=" + key} {
		got := serve(t, d, call{method: "POST", target: target, host: deckOnWiFi, from: aPhone, body: `{"text":"hello"}`})
		if got.Code != http.StatusForbidden || !strings.Contains(got.Body.String(), "Typing over Wi-Fi is off") {
			t.Errorf("POST %s over the Wi-Fi with it off: %d %s, want 403", target, got.Code, got.Body)
		}
	}
	if keys := serve(t, d, call{method: "POST", target: "/act?action=send&session=nobody&t=" + key, host: deckOnWiFi, from: aPhone}); keys.Code != http.StatusNotFound {
		t.Errorf("a key over the Wi-Fi: %d, want it to reach the deck (404 for no session)", keys.Code)
	}
	for _, who := range []call{{asTool: true}, {withoutSetupKey: true, headers: map[string]string{gmTokenHeader: "x"}}} {
		who.method, who.target, who.host, who.from = "POST", "/setup/wifityping?on=1", "localhost:7710", thisComputer
		if who.headers == nil {
			who.headers = map[string]string{}
		}
		who.headers[setupHeader] = "1"
		serve(t, d, who)
	}
	d.mu.Lock()
	on = d.config.PhoneTextWiFi
	d.mu.Unlock()
	if on {
		t.Error("a tool or an agent turned typing over Wi-Fi on")
	}
	serve(t, d, call{method: "POST", target: "/setup/wifityping?on=1", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	if got := serve(t, d, call{method: "POST", target: "/dictate?session=nobody&t=" + key, host: deckOnWiFi, from: aPhone, body: `{"text":"hello"}`}); got.Code != http.StatusNotFound {
		t.Errorf("typing over the Wi-Fi with it on: %d %s, want it to reach the deck (404 for no session)", got.Code, got.Body)
	}
	page := readWeb(t, "setup.html")
	if !strings.Contains(page, "Anyone on this Wi-Fi could read what you type. The Tailscale link is private.") {
		t.Error("turning typing over Wi-Fi on doesn't warn")
	}
}

func TestASetupFromBeforeTheSwitchKeepsTyping(t *testing.T) {
	for _, c := range []struct {
		file string
		want bool
	}{
		{`{"hasCompletedSetup": true}`, true},
		{`{"hasCompletedSetup": true, "phoneMicEnabled": false}`, false},
		{`{"hasCompletedSetup": false}`, false},
		{`{"hasCompletedSetup": true, "phoneTextOverWiFi": false}`, false},
		{`{"phoneTextOverWiFi": true}`, true},
	} {
		isolate(t)
		if err := os.WriteFile(configFile, []byte(c.file), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := loadConfig().PhoneTextWiFi; got != c.want {
			t.Errorf("%s: typing over Wi-Fi %v, want %v", c.file, got, c.want)
		}
	}
}

// The nonces kept are bounded: once full, those whose time has run out make way, and until then new
// requests wait rather than any being forgotten early.
func TestTheNoncesKeptAreBounded(t *testing.T) {
	a := &Access{}
	now := time.Now()
	for i := range mostNonces {
		if why := a.fresh(signedParts{at: now, nonce: strconv.Itoa(i)}, now); why != "" {
			t.Fatalf("nonce %d: %q", i, why)
		}
	}
	if why := a.fresh(signedParts{at: now, nonce: "one-more"}, now); why != signedBusy {
		t.Errorf("past the most nonces: %q, want %q", why, signedBusy)
	}
	later := now.Add(signedWindow + time.Second)
	if why := a.fresh(signedParts{at: later, nonce: "one-more"}, later); why != "" || len(a.nonces) != 1 {
		t.Errorf("once they ran out: %q with %d kept, want taken with 1", why, len(a.nonces))
	}
}

func readWeb(t *testing.T, name string) string { return string(readWebBytes(t, name)) }

func readWebBytes(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "Resources", "web", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// A phone paired before signing (RSECWIFI B1) sent its key as ?t= over the Wi-Fi, and an old saved page still
// does: anyone who read it there could sign with it. Its key works bare over Tailscale and on this computer
// only, never signed from the Wi-Fi; setup says so.
func TestAKeyFromBeforeSigningDoesntSignOverTheWiFi(t *testing.T) {
	d := newTestDeck(t)
	d.mu.Lock()
	// As every phone paired before 1.25, and a setup from before that typed over the Wi-Fi, which keeps it on.
	legacy := d.access.Phones.Pair("iPhone", false)
	d.config.PhoneTextWiFi = true
	d.mu.Unlock()
	// The old page opened after the upgrade: refused, but its key crossed the Wi-Fi.
	serve(t, d, call{method: "GET", target: "/?t=" + legacy, host: deckOnWiFi, from: aPhone, bare: true})
	for _, c := range []call{
		{method: "POST", target: "/focus?session=nobody&t=" + legacy},
		{method: "POST", target: "/dictate?session=nobody&t=" + legacy, body: `{"text":"x"}`},
		{method: "GET", target: "/paired?t=" + legacy},
	} {
		c.host, c.from = deckOnWiFi, anotherPhone
		if got := serve(t, d, c); got.Code != http.StatusUnauthorized {
			t.Errorf("%s %s signed with a key from before signing, from the Wi-Fi: %d, want 401", c.method, c.target, got.Code)
		}
	}
	// Bare, on this computer and handed on by `tailscale serve`, it still works.
	if got := serve(t, d, call{method: "GET", target: "/paired?t=" + legacy, host: "localhost:7710", from: thisComputer}); got.Code != http.StatusNoContent {
		t.Errorf("its key on this computer: %d, want 204", got.Code)
	}
	// Handed on by the real `tailscale serve` (a stand-in peer, SN1); a program here sending its headers isn't.
	if got := serve(t, d, call{method: "GET", target: "/paired?t=" + legacy, host: "127.0.0.1:7710", from: thisComputer,
		headers: map[string]string{"X-Forwarded-For": "100.101.102.103"}}); letIn(got.Code) {
		t.Errorf("its key from a program here with a proxy's headers: %d, want it refused", got.Code)
	}
	standInTailscalePeer(t)
	if got := serve(t, d, call{method: "GET", target: "/paired?t=" + legacy, host: "127.0.0.1:7710", from: thisComputer,
		headers: map[string]string{"X-Forwarded-For": "100.101.102.103"}}); got.Code != http.StatusNoContent {
		t.Errorf("its key over Tailscale: %d, want 204", got.Code)
	}
	// Setup lists it as from before, and, once a phone of that name pairs again, offers to remove it.
	before := func() map[string]any {
		state := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer})
		var answer struct{ Phones []map[string]any }
		_ = json.Unmarshal(state.Body.Bytes(), &answer)
		for _, phone := range answer.Phones {
			if phone["beforeSigning"] == true {
				return phone
			}
		}
		return nil
	}
	if phone := before(); phone == nil || phone["pairedAgain"] != false {
		t.Fatalf("setup's phone from before signing: %v, want it listed, not yet paired again", phone)
	}
	pair(t, d)
	if phone := before(); phone == nil || phone["pairedAgain"] != true {
		t.Errorf("setup's phone from before signing, once an iPhone paired again: %v, want pairedAgain", phone)
	}
	page := readWeb(t, "setup.html")
	if !strings.Contains(page, "Paired before 1.25: its key crossed the Wi-Fi in clear. Unpair it and pair again to use the Wi-Fi.") {
		t.Error("setup doesn't say a phone from before signing should pair again")
	}
}

// A signed request whose body never comes (RSECWIFI B2) holds its own connection only: never the deck's lock,
// which every phone, the office, the hooks and setup wait on. No key is needed to send one.
func TestAStalledBodyLeavesTheDeckFree(t *testing.T) {
	for _, path := range []string{"/events", "/act", "/pair"} {
		t.Run(path, func(t *testing.T) {
			d := newTestDeck(t)
			if path == "/pair" {
				serve(t, d, call{method: "POST", target: "/setup/pair", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
			}
			server := httptest.NewServer(&router{deck: d})
			defer server.Close()
			conn, err := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			method := map[bool]string{true: "GET", false: "POST"}[path == "/events"]
			fmt.Fprintf(conn, "%s %s?ts=%d&n=aaaaaaaaaaaaaaaaaaaa&sig=%s HTTP/1.1\r\nHost: %s\r\nContent-Length: 1000\r\n\r\nx",
				method, path, time.Now().UnixMilli(), strings.Repeat("ab", sha256.Size), deckOnWiFi)
			time.Sleep(300 * time.Millisecond)
			free := make(chan struct{})
			go func() { d.mu.Lock(); d.mu.Unlock(); close(free) }()
			select {
			case <-free:
			case <-time.After(2 * time.Second):
				t.Errorf("%s %s, wrongly signed, its body stalled: the deck's lock is held", method, path)
			}
			conn.Close()
			<-free
		})
	}
}

// A signed GET hashes an empty body: one that comes with a body is refused before it's read.
func TestASignedGetWithABodyIsRefused(t *testing.T) {
	d := newTestDeck(t)
	key := pair(t, d)
	query := "ts=" + strconv.FormatInt(time.Now().UnixMilli(), 10) + "&n=" + randomKey()[:22]
	target := signWith(key, "GET", "/paired", query, "hello")
	if got := serve(t, d, call{method: "GET", target: target, body: "hello", host: deckOnWiFi, from: aPhone, bare: true}); got.Code != http.StatusUnauthorized {
		t.Errorf("a signed GET with a body: %d, want 401", got.Code)
	}
}

// A request sniffed in the minute before the app started can't be sent after: the nonces it remembers are
// gone with the last start, so nothing signed before this one is taken (RSECWIFI N2).
func TestARequestFromBeforeThisStartIsRefused(t *testing.T) {
	d := newTestDeck(t)
	key := pair(t, d)
	d.mu.Lock()
	d.access.started = time.Now()
	d.mu.Unlock()
	target := signedTarget("POST", "/focus?session=nobody&t="+key, "", time.Now().Add(-10*time.Second))
	got := serve(t, d, call{method: "POST", target: target, host: deckOnWiFi, from: aPhone, bare: true})
	if why, _ := signedRefusalOf(t, got.Body); got.Code != http.StatusUnauthorized || why != signedClock {
		t.Errorf("signed before this start: %d %q, want 401 %q", got.Code, why, signedClock)
	}
}

// A photo of the QR code alone isn't the phone's key; with the claim read off the Wi-Fi as the phone pairs, it
// is (RSECWIFI N1: no key exchange in 1.25.0). The docs say so, and keep "keep it to yourself".
func TestAPhotoOfTheQRAndASniffedClaimAreTheKeyAsTheDocsSay(t *testing.T) {
	d := newTestDeck(t)
	serve(t, d, call{method: "POST", target: "/setup/pair", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	d.mu.Lock()
	codeKey := d.access.PairingKey()
	d.mu.Unlock()
	part := randomKey()
	if got := serve(t, d, call{method: "POST", target: "/pair?p=" + part + "&t=" + codeKey, host: deckOnWiFi, from: aPhone}); got.Code != http.StatusNoContent {
		t.Fatalf("the claim: %d", got.Code)
	}
	if got := serve(t, d, call{method: "POST", target: "/focus?session=nobody&t=" + phoneKeyFrom(codeKey, part), host: deckOnWiFi, from: anotherPhone}); got.Code != http.StatusNotFound {
		t.Errorf("the photo's key with the sniffed part: %d, want it taken, as the docs warn", got.Code)
	}
	docs, err := os.ReadFile(filepath.Join("..", "docs", "phone.md"))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(strings.Fields(string(docs)), " ")
	for _, want := range []string{"a photo of the QR code alone, without also watching your Wi-Fi as your phone pairs, isn't your phone's key", "keep it to yourself"} {
		if !strings.Contains(joined, want) {
			t.Errorf("docs/phone.md doesn't say %q", want)
		}
	}
	if strings.Contains(joined, "so a photo of the QR code isn't your phone's key") {
		t.Error("docs/phone.md still says a photo of the QR code isn't the key, with no word of the claim")
	}
}

// Every route a paired phone reaches, and whether it carries text the phone typed or heard (RSECWIFI N3).
// A route with text must ask d.typesFrom(r): over the Wi-Fi, anyone there could read it. A new route goes here,
// with the answer, or this test fails. Routes are in router.handle's switch or the route table (phoneroutes.go).
var phoneRoutes = map[string]bool{
	"/paired": false, "/events": false, "/focus": false,
	// Its actions are keys, floors, styles, music, a hire's desk and role: never free text.
	"/act": false, "/hire/window": false, "/t3code": false, "/t3code/open": false,
	"/dictate": true, "/peek": false, "/choose": true, "/choice": false, "/approvals": false,
	"/conversation": false, "/session/info": false, "/summary": false,
	// The office's own window alone (1.26): no phone reaches them, over the Wi-Fi or anywhere.
	"/desk/git": false, "/desk/pr/open": false, "/talk": false, "/talk/asking": false, "/talk/say": false, "/talk/pick": false, "/talk/listen": false,
	// The TV's preview (V1, preview.go): the office's own window alone, with a loopback address, never a phone's text.
	"/desk/preview": false,
	// A photo (PH1, photo.go): a picture, never a phone's words, and the path typed is the app's own. POST /photo is
	// Tailscale only (accessPhoneSecure), so the Wi-Fi never reaches it, and sendPhoto asks the phone-text switch;
	// POST /talk/photo is the window's alone.
	"/photo": false, "/talk/photo": false,
	// Another Mac's alone (M1, peers_mac.go): no phone or window reaches it, and it answers a name, reading no body.
	// POST /peer/pair isn't a row: servePeer answers it before the table, with a code and a name, never a phone's text.
	"/peer/hello": false,
	// The owner's asks and his yes or no to one (FS2, grants.go): a paired phone over Tailscale alone, never the
	// Wi-Fi (accessOwnerTap), with an ask's id and yes or no, never a phone's text.
	"/grants": false, "/grants/answer": false,
	// A phone's notifications (PUSH1, pushsubs.go): a paired phone over Tailscale alone (accessPhoneSecure), with a
	// browser's subscription to a push service, never a phone's text.
	"/push": false, "/push/off": false,
}

// phoneRouteCode is one route's code as this test reads it: its paths, its code, and whether only the window reaches it.
type phoneRouteCode struct {
	paths      []string
	code       ast.Node
	windowOnly bool
}

// phoneRouteCodes is every phone route's code: each case of router.handle's switch (the one with /events), and each
// row registered in the route table (registerPhoneRoute, in any of the package's files).
func phoneRouteCodes(t *testing.T) []phoneRouteCode {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var codes []phoneRouteCode
	var routes *ast.SwitchStmt
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if name == "server.go" && n.Name.Name == "handle" && n.Recv != nil {
					ast.Inspect(n.Body, func(n ast.Node) bool {
						if s, ok := n.(*ast.SwitchStmt); ok && routes == nil && slices.Contains(pathsIn(s), "/events") {
							routes = s
						}
						return true
					})
				}
			case *ast.CallExpr:
				if fn, ok := n.Fun.(*ast.Ident); !ok || fn.Name != "registerPhoneRoute" || len(n.Args) != 1 {
					return true
				}
				row, ok := n.Args[0].(*ast.CompositeLit)
				if !ok {
					return true
				}
				code := phoneRouteCode{code: row}
				for _, field := range row.Elts {
					kv, ok := field.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					switch key, _ := kv.Key.(*ast.Ident); {
					case key == nil:
					case key.Name == "path":
						if lit, ok := kv.Value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
							code.paths = append(code.paths, strings.Trim(lit.Value, `"`))
						}
					case key.Name == "access":
						if id, ok := kv.Value.(*ast.Ident); ok && id.Name == "accessWindow" {
							code.windowOnly = true
						}
					}
				}
				if len(code.paths) == 0 {
					t.Errorf("%s: a registerPhoneRoute row whose path isn't written as text", name)
				}
				codes = append(codes, code)
			}
			return true
		})
	}
	if routes == nil {
		t.Fatal("no switch of the phone's routes in router.handle")
	}
	for _, clause := range routes.Body.List {
		clause := clause.(*ast.CaseClause)
		code := phoneRouteCode{code: clause}
		for _, test := range clause.List {
			code.paths = append(code.paths, routePaths(test)...)
		}
		codes = append(codes, code)
	}
	if len(phoneRouteTable) == 0 || len(codes) <= len(phoneRouteTable) {
		t.Fatalf("%d routes read, with %d table rows: the table's rows weren't all found", len(codes), len(phoneRouteTable))
	}
	return codes
}

func TestEveryPhoneRouteWithTextAsksTypesFrom(t *testing.T) {
	seen := map[string]bool{}
	for _, route := range phoneRouteCodes(t) {
		asks, readsBody, windowOnly := false, false, route.windowOnly
		ast.Inspect(route.code, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := n.X.(*ast.Ident); ok && id.Name == "d" && n.Sel.Name == "typesFrom" {
					asks = true
				}
				if id, ok := n.X.(*ast.Ident); ok && id.Name == "r" && n.Sel.Name == "Body" {
					readsBody = true
				}
			case *ast.BinaryExpr:
				if x, ok := n.X.(*ast.Ident); ok && x.Name == "phoneID" && n.Op == token.NEQ {
					if y, ok := n.Y.(*ast.Ident); ok && y.Name == "officeWindowID" {
						windowOnly = true
					}
				}
			}
			return true
		})
		for _, path := range route.paths {
			seen[path] = true
			text, known := phoneRoutes[path]
			switch {
			case !known:
				t.Errorf("%s: a phone route this test doesn't know: add it to phoneRoutes, saying whether it carries text", path)
			case text && !asks:
				t.Errorf("%s carries a phone's text but doesn't ask d.typesFrom(r)", path)
			case readsBody && !asks && !windowOnly:
				t.Errorf("%s reads a body from a phone but doesn't ask d.typesFrom(r)", path)
			}
		}
	}
	for path := range phoneRoutes {
		if !seen[path] {
			t.Errorf("%s is in phoneRoutes but not among the phone's routes (router.handle's switch or the route table)", path)
		}
	}
}

// pathsIn is every route path a switch's cases name.
func pathsIn(s *ast.SwitchStmt) []string {
	var paths []string
	for _, clause := range s.Body.List {
		for _, test := range clause.(*ast.CaseClause).List {
			paths = append(paths, routePaths(test)...)
		}
	}
	return paths
}

// routePaths is every "/path" a case compares r.URL.Path with.
func routePaths(test ast.Expr) []string {
	var paths []string
	ast.Inspect(test, func(n ast.Node) bool {
		if b, ok := n.(*ast.BinaryExpr); ok && b.Op == token.EQL {
			if sel, ok := b.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "Path" {
				if lit, ok := b.Y.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					paths = append(paths, strings.Trim(lit.Value, `"`))
				}
			}
		}
		return true
	})
	return paths
}
