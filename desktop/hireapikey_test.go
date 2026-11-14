package desktop

import (
	"errors"
	"strings"
	"testing"
)

// A stand-in key: short, and never one of Anthropic's.
const fakeHireKey = "fk-m4c-key-0123456789XYZ"

// On Linux and Windows, the API-key account is offered only with a key in the
// secret store (never ANTHROPIC_API_KEY alone) that the script can read, and
// never on Windows; each refusal says why, the notice is always the Mac's.
func TestTheAPIKeyAccountIsOfferedOnlyFromTheSecretStore(t *testing.T) {
	yes, no := func() bool { return true }, func() bool { return false }
	for _, c := range []struct {
		goos, shown string
		noStore     bool
		tool        func() bool
		want        apiKeyOffer
	}{
		{"linux", "store", false, yes, apiKeyOffer{Available: true, Notice: apiKeyNotice}},
		{"linux", "environment", false, yes, apiKeyOffer{Notice: apiKeyNotice, Refusal: apiKeyNotInStore}},
		{"linux", "", false, yes, apiKeyOffer{Notice: apiKeyNotice, Refusal: apiKeyNotInStore}},
		{"linux", "", true, yes, apiKeyOffer{Notice: apiKeyNotice, Refusal: apiKeyNoStore}},
		{"linux", "store", false, no, apiKeyOffer{Notice: apiKeyNotice, Refusal: apiKeyNoSecretTool}},
		{"windows", "store", false, yes, apiKeyOffer{Notice: apiKeyNotice, Refusal: apiKeyNotHere}},
	} {
		if got := localAPIKeyOffer(c.goos, c.shown, c.noStore, c.tool); got != c.want {
			t.Errorf("%s %q: %+v, want %+v", c.goos, c.shown, got, c.want)
		}
	}
	if state := (apiKeyOffer{Available: true, Notice: apiKeyNotice, Refusal: "x"}).state(); len(state) != 2 || state["available"] != true || state["notice"] != apiKeyNotice {
		t.Errorf("setup's hiring.apiKey: %v", state)
	}
}

// Under the Mac app, the offer and the key are the app's, in its words: an app
// that can't be asked offers nothing, and a key it won't give is its refusal.
func TestUnderTheMacAppTheAPIKeyIsTheAppsAndReadSafely(t *testing.T) {
	refusal := "There's no Anthropic API key in your Keychain. Save one under Summaries, or hire on a Claude account."
	for _, c := range []struct {
		name   string
		answer bridgeAnswer
		err    error
		want   apiKeyOffer
	}{
		{"offered", bridgeAnswer{OK: true, Available: true, Notice: "N"}, nil, apiKeyOffer{Available: true, Notice: "N"}},
		{"not offered", bridgeAnswer{OK: true, Notice: "N", Refusal: refusal}, nil, apiKeyOffer{Notice: "N", Refusal: refusal}},
		{"not offered, no words", bridgeAnswer{OK: true}, nil, apiKeyOffer{Notice: apiKeyNotice, Refusal: apiKeyMacUnread}},
		{"the app said no", bridgeAnswer{Error: "Not this app's engine."}, nil, apiKeyOffer{Notice: apiKeyNotice, Refusal: apiKeyMacUnasked}},
		{"no answer", bridgeAnswer{}, errors.New("gone"), apiKeyOffer{Notice: apiKeyNotice, Refusal: apiKeyMacUnasked}},
	} {
		fake := &fixtureHost{answer: c.answer}
		if got := macAPIKeyOffer(&errorHost{fake, c.err}); got != c.want || fake.asked == nil || fake.asked.Action != "api-key-offer" {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
	for _, c := range []struct {
		name   string
		answer bridgeAnswer
		err    error
		key    string
		why    string
	}{
		{"given", bridgeAnswer{OK: true, Found: true, Value: fakeHireKey}, nil, fakeHireKey, ""},
		{"refused, in the app's words", bridgeAnswer{Error: refusal}, nil, "", refusal},
		{"none found", bridgeAnswer{OK: true}, nil, "", apiKeyMacUnread},
		{"not one word", bridgeAnswer{OK: true, Found: true, Value: "fk m4c'; rm -rf ~"}, nil, "", apiKeyMacUnread},
		{"no answer", bridgeAnswer{}, errors.New("gone"), "", apiKeyMacUnread},
	} {
		d := &Deck{}
		fake := &fixtureHost{answer: c.answer}
		d.mac.host = &errorHost{fake, c.err}
		key, err := d.hireAPIKeyText()
		why := ""
		if refused, ok := err.(*hireRefusal); ok && refused.Status == 409 {
			why = refused.Message
		} else if err != nil {
			why = "not a 409: " + err.Error()
		}
		if key != c.key || why != c.why || fake.asked.Action != "api-key-read" {
			t.Errorf("%s: %q %q, want %q %q", c.name, key, why, c.key, c.why)
		}
	}
	// Off the Mac app, nothing is asked: the script reads the secret store itself.
	if key, err := (&Deck{}).hireAPIKeyText(); key != "" || err != nil {
		t.Errorf("no Mac app: %q %v", key, err)
	}
}

// errorHost answers as its fixtureHost does, or fails as the transport would.
type errorHost struct {
	*fixtureHost
	err error
}

func (e *errorHost) call(request bridgeRequest) (bridgeAnswer, error) {
	answer, _ := e.fixtureHost.call(request)
	return answer, e.err
}

// The key the Mac app gave is set in the desk's start, once, which reaches its pane only by its ticket; it's
// given only to a hire on the API-key account, and only a key that is one word. Without it, the secret store is
// read when the pane asks (deskstart.go).
func TestAnAPIKeyHiresStartSetsTheKeyTheMacAppGave(t *testing.T) {
	start := hireStart{Role: Role{ID: "janitor", Name: "Janitor", Agent: "claude"}, Message: "go", APIKey: true, KeyText: fakeHireKey}
	script, err := start.startText()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "export ANTHROPIC_API_KEY='"+fakeHireKey+"'\n") || strings.Count(script, fakeHireKey) != 1 {
		t.Errorf("the key isn't set once:\n%s", script)
	}
	if strings.Contains(script, "secret store") {
		t.Errorf("the Mac app's key and the secret store both:\n%s", script)
	}
	for _, bad := range []hireStart{
		{Role: start.Role, KeyText: fakeHireKey},                            // not on the API-key account
		{Role: start.Role, APIKey: true, KeyText: "fk m4c"},                 // not one word
		{Role: start.Role, APIKey: true, KeyText: "fk-m4c'\nrm -rf ~ #"},    // nor a way out of its quotes
		{Role: start.Role, APIKey: true, KeyText: strings.Repeat("k", 513)}, // nor more than a key
	} {
		if text, err := bad.startText(); err == nil {
			t.Errorf("a script was written for %q:\n%s", bad.KeyText, text)
		}
	}
	start.KeyText = ""
	if plan, _ := start.plan(); !plan.APIKey || strings.Contains(planText(plan), "export ANTHROPIC_API_KEY='") {
		t.Errorf("off the Mac app the start reads the secret store: %+v", plan)
	}
}
