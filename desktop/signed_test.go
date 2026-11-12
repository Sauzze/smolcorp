package desktop

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	// Within the minute, it's taken.
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
