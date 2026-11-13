package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// Phone over HTTPS on every system (1.26, FR2). No test here runs the real `tailscale`: a stand-in, a small script
// that only prints what the test put in its folder (and writes down what it was asked to run), takes its place.

// tailscaleStandIn is a stand-in `tailscale`: its folder, where the test puts what it prints.
type tailscaleStandIn struct{ dir, tool string }

const tailscaleStandInScript = `#!/bin/sh
dir=$(dirname "$0")
case "$1 $2" in
"status --json") cat "$dir/status.json" ;;
"serve status") cat "$dir/serve.json" ;;
*)
	echo "$@" >> "$dir/ran"
	if [ -f "$dir/fail" ]; then cat "$dir/fail" >&2; exit 1; fi
	case "$2" in
	--bg) [ -f "$dir/after-on.json" ] && cp "$dir/after-on.json" "$dir/serve.json" ;;
	*) [ -f "$dir/after-off.json" ] && cp "$dir/after-off.json" "$dir/serve.json" ;;
	esac
	exit 0
	;;
esac
`

const (
	tsSignedIn  = `{"BackendState":"Running","Self":{"DNSName":"my-pc.tail1234.ts.net."}}`
	tsNothingOn = `{}`
)

// servingPort is `tailscale serve status --json` with a proxy to a port on this computer.
func servingPort(port int) string {
	return fmt.Sprintf(`{"TCP":{"443":{"HTTPS":true}},"Web":{"my-pc.tail1234.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:%d"}}}}}`, port)
}

// withStandInTailscale puts a stand-in `tailscale` where the app looks for it, and the app's own look in place of
// the tests' not-installed one, and forgets the last answer.
func withStandInTailscale(t *testing.T, status, serve string) *tailscaleStandIn {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	s := &tailscaleStandIn{dir: t.TempDir()}
	s.tool = filepath.Join(s.dir, "tailscale")
	if err := os.WriteFile(s.tool, []byte(tailscaleStandInScript), 0o755); err != nil {
		t.Fatal(err)
	}
	s.say("status.json", status)
	s.say("serve.json", serve)
	savedAsk, savedCandidates := askTailnet, tailnetCandidates
	askTailnet, tailnetCandidates = realAskTailnet, []string{s.tool}
	forgetTailnet()
	t.Cleanup(func() {
		askTailnet, tailnetCandidates = savedAsk, savedCandidates
		forgetTailnet()
	})
	return s
}

func forgetTailnet() {
	tailnetLook.mu.Lock()
	tailnetLook.answer, tailnetLook.lookedAt, tailnetLook.looking = tailnetAnswer{}, time.Time{}, false
	tailnetLook.mu.Unlock()
}

func (s *tailscaleStandIn) say(file, text string) {
	if err := os.WriteFile(filepath.Join(s.dir, file), []byte(text), 0o644); err != nil {
		panic(err)
	}
}

// lines is what the stand-in was asked to run, one line each (a status look isn't a run).
func (s *tailscaleStandIn) lines() []string {
	data, _ := os.ReadFile(filepath.Join(s.dir, "ran"))
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// httpsSaid is setup's Phone over HTTPS as the page reads it, once Tailscale has been looked at.
func httpsSaid(t *testing.T, d *Deck) map[string]any {
	t.Helper()
	var said map[string]any
	eventually(t, "Tailscale looked at", func() bool {
		code, body := pageCall(t, d, "GET", "/setup/tailnet/status")
		said = nil
		return code == http.StatusOK && json.Unmarshal([]byte(body), &said) == nil && said["status"] != httpsChecking
	})
	return said
}

// The four statuses, and the address, as Tailscale's command line says them, on every system.
func TestPhoneOverHTTPSSaysWhereTailscaleStands(t *testing.T) {
	for _, c := range []struct {
		name, status, serve string
		installed           bool
		want, address       string
		busy                bool
	}{
		{"not installed", "", "", false, httpsNotInstalled, "", false},
		{"not signed in", `{"BackendState":"NeedsLogin"}`, tsNothingOn, true, httpsSignedOut, "", false},
		{"stopped", `{"BackendState":"Stopped","Self":{"DNSName":"my-pc.tail1234.ts.net."}}`, tsNothingOn, true, httpsSignedOut, "https://my-pc.tail1234.ts.net", false},
		{"serve off", tsSignedIn, tsNothingOn, true, httpsOff, "https://my-pc.tail1234.ts.net", false},
		{"serve on", tsSignedIn, servingPort(Port), true, httpsOn, "https://my-pc.tail1234.ts.net", false},
		// Something else is handed on, not this app: off, and turning it on would replace it.
		{"serve on for another port", tsSignedIn, servingPort(Port + 1), true, httpsOff, "https://my-pc.tail1234.ts.net", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := withStandInTailscale(t, c.status, c.serve)
			if !c.installed {
				tailnetCandidates = []string{filepath.Join(s.dir, "no-such-tailscale")}
			}
			d := newTestDeck(t)
			said := httpsSaid(t, d)
			if said["status"] != c.want || (c.address == "") != (said["address"] == nil) || c.address != "" && said["address"] != c.address ||
				said["busy"] != c.busy {
				t.Errorf("%v, want status %s, address %q, busy %v", said, c.want, c.address, c.busy)
			}
			// The one line to copy, with this app's own port, and the line that stops it.
			if said["command"] != fmt.Sprintf("tailscale serve --bg --https=443 http://127.0.0.1:%d", Port) ||
				said["offCommand"] != "tailscale serve --https=443 off" || said["port"] != float64(Port) {
				t.Errorf("the lines: %v", said)
			}
			if got := s.lines(); len(got) != 0 {
				t.Errorf("a look ran %q", got)
			}
		})
	}
}

// Every system looks for Tailscale's command line where it's put, then along PATH.
func TestTailscaleIsLookedForWhereEachSystemPutsIt(t *testing.T) {
	if got := tailnetCandidatesOn("darwin"); len(got) != 3 || got[0] != "/Applications/Tailscale.app/Contents/MacOS/Tailscale" {
		t.Errorf("a Mac: %v", got)
	}
	if got := tailnetCandidatesOn("linux"); !slices.Contains(got, "/usr/bin/tailscale") {
		t.Errorf("Linux: %v", got)
	}
	t.Setenv("ProgramFiles", `C:\Program Files`)
	if got := tailnetCandidatesOn("windows"); len(got) == 0 || !strings.HasSuffix(got[0], "tailscale.exe") || !strings.Contains(got[0], "Tailscale") {
		t.Errorf("Windows: %v", got)
	}
	// On PATH, when it's in none of those.
	savedCandidates, savedPath := tailnetCandidates, tailnetLookPath
	t.Cleanup(func() { tailnetCandidates, tailnetLookPath = savedCandidates, savedPath })
	tailnetCandidates = []string{filepath.Join(t.TempDir(), "nothing")}
	tailnetLookPath = func(name string) (string, error) { return "/somewhere/" + name, nil }
	if got := tailscaleTool(); got != "/somewhere/tailscale" {
		t.Errorf("on PATH: %q", got)
	}
}

func TestTheStatusReadsAreOnlyWhatTailscaleSays(t *testing.T) {
	for _, c := range []struct {
		status             string
		signedIn, anything bool
	}{
		{`{"BackendState":"Running"}`, true, false},
		{`{"BackendState":"NeedsLogin"}`, false, false},
		{`{"BackendState":"Starting"}`, false, false},
		{`not json`, false, false},
	} {
		if got := tailnetSignedIn([]byte(c.status)); got != c.signedIn {
			t.Errorf("%s: signed in %v", c.status, got)
		}
	}
	for serve, want := range map[string]bool{tsNothingOn: false, servingPort(1234): true, `{"TCP":{"22":{"TCPForward":"127.0.0.1:22"}}}`: true, `not json`: false} {
		if got := servesAnything([]byte(serve)); got != want {
			t.Errorf("%s: serves anything %v", serve, got)
		}
	}
}

// post is the page's tap: a POST with the page's header and what it sends back.
func tap(t *testing.T, d *Deck, who caller, gm, hireKey, body string) (int, string) {
	t.Helper()
	c := asCaller(who, gm, hireKey)
	c.method, c.target, c.host, c.from, c.body = "POST", "/setup/tailnet/serve", "localhost:7710", thisComputer, body
	headers := map[string]string{setupHeader: "1"}
	for name, value := range c.headers {
		headers[name] = value
	}
	c.headers = headers
	w := serve(t, d, c)
	return w.Code, strings.TrimSpace(w.Body.String())
}

func tapBody(action, command string) string {
	data, _ := json.Marshal(map[string]string{"action": action, "command": command})
	return string(data)
}

// A tap runs exactly the command the confirmation showed, on this app's own port, and says what it exposes first.
func TestTurningItOnAndOffRunsExactlyTheCommandItShowedAfterAConfirmation(t *testing.T) {
	s := withStandInTailscale(t, tsSignedIn, tsNothingOn)
	s.say("after-on.json", servingPort(Port))
	s.say("after-off.json", tsNothingOn)
	d := newTestDeck(t)
	if said := httpsSaid(t, d); said["status"] != httpsOff {
		t.Fatalf("to begin with: %v", said)
	}

	// The confirmation: the exact command, and what it exposes. It runs nothing.
	code, body := pageCall(t, d, "GET", "/setup/tailnet/serve?action=on")
	var plan struct {
		Command string
		Exposes []string
	}
	wantOn := fmt.Sprintf("tailscale serve --bg --https=443 http://127.0.0.1:%d", Port)
	if code != http.StatusOK || json.Unmarshal([]byte(body), &plan) != nil || plan.Command != wantOn {
		t.Fatalf("the confirmation: %d %s", code, body)
	}
	said := strings.Join(plan.Exposes, " ")
	for _, part := range []string{fmt.Sprint(Port), "tailnet", "not Funnel", "public internet"} {
		if !strings.Contains(strings.ToLower(said), strings.ToLower(part)) {
			t.Errorf("the confirmation doesn't say %q: %s", part, said)
		}
	}
	if got := s.lines(); len(got) != 0 {
		t.Fatalf("the confirmation ran %q", got)
	}

	// A tap that wasn't shown the command, or was shown another, runs nothing.
	for _, c := range []struct {
		name, body string
		want       int
	}{
		{"no command", `{"action":"on"}`, http.StatusPreconditionRequired},
		{"no action", tapBody("", wantOn), http.StatusBadRequest},
		{"not json", `on`, http.StatusBadRequest},
		{"another command", tapBody("on", "tailscale funnel --bg 443"), http.StatusConflict},
		{"another port", tapBody("on", fmt.Sprintf("tailscale serve --bg --https=443 http://127.0.0.1:%d", Port+1)), http.StatusConflict},
		{"another address", tapBody("on", fmt.Sprintf("tailscale serve --bg --https=443 http://0.0.0.0:%d", Port)), http.StatusConflict},
		{"the off command for on", tapBody("on", "tailscale serve --https=443 off"), http.StatusConflict},
		{"an action that isn't one", tapBody("funnel", wantOn), http.StatusBadRequest},
	} {
		if code, text := tap(t, d, callerPage, "", "", c.body); code != c.want {
			t.Errorf("%s: %d %s, want %d", c.name, code, text, c.want)
		}
	}
	if got := s.lines(); len(got) != 0 {
		t.Fatalf("a refused tap ran %q", got)
	}

	// Confirmed: exactly that, and the status after is on.
	code, body = tap(t, d, callerPage, "", "", tapBody("on", plan.Command))
	var after map[string]any
	if code != http.StatusOK || json.Unmarshal([]byte(body), &after) != nil || after["status"] != httpsOn {
		t.Fatalf("on: %d %s", code, body)
	}
	if got := s.lines(); !slices.Equal(got, []string{fmt.Sprintf("serve --bg --https=443 http://127.0.0.1:%d", Port)}) {
		t.Fatalf("it ran %q", got)
	}
	// On already: no second run.
	if code, _ := tap(t, d, callerPage, "", "", tapBody("on", plan.Command)); code != http.StatusConflict || len(s.lines()) != 1 {
		t.Errorf("on twice: %d, %q", code, s.lines())
	}

	// Turning it off is the same, with its own confirmation.
	code, body = pageCall(t, d, "GET", "/setup/tailnet/serve?action=off")
	if code != http.StatusOK || json.Unmarshal([]byte(body), &plan) != nil || plan.Command != "tailscale serve --https=443 off" {
		t.Fatalf("the off confirmation: %d %s", code, body)
	}
	if code, _ := tap(t, d, callerPage, "", "", tapBody("off", wantOn)); code != http.StatusConflict || len(s.lines()) != 1 {
		t.Errorf("off with the on command: %d, %q", code, s.lines())
	}
	if code, _ := tap(t, d, callerPage, "", "", `{"action":"off"}`); code != http.StatusPreconditionRequired || len(s.lines()) != 1 {
		t.Errorf("off with no command: %d, %q", code, s.lines())
	}
	code, body = tap(t, d, callerPage, "", "", tapBody("off", plan.Command))
	if code != http.StatusOK || json.Unmarshal([]byte(body), &after) != nil || after["status"] != httpsOff {
		t.Fatalf("off: %d %s", code, body)
	}
	if got := s.lines(); !slices.Equal(got, []string{fmt.Sprintf("serve --bg --https=443 http://127.0.0.1:%d", Port), "serve --https=443 off"}) {
		t.Errorf("it ran %q", got)
	}
	for _, line := range s.lines() {
		if strings.Contains(line, "funnel") {
			t.Errorf("funnel: %q", line)
		}
	}
}

// A tap never changes what it shouldn't: nothing when Tailscale isn't here or isn't signed in, and never an off for
// what's handed on that isn't this app.
func TestATapRunsNothingWhereTailscaleStandsInTheWay(t *testing.T) {
	wantOn := fmt.Sprintf("tailscale serve --bg --https=443 http://127.0.0.1:%d", Port)
	for _, c := range []struct {
		name, status, serve, action, command string
	}{
		{"not signed in, on", `{"BackendState":"NeedsLogin"}`, tsNothingOn, "on", wantOn},
		{"not signed in, off", `{"BackendState":"NeedsLogin"}`, servingPort(Port), "off", "tailscale serve --https=443 off"},
		{"off with nothing handed on", tsSignedIn, tsNothingOn, "off", "tailscale serve --https=443 off"},
		// Something else on 443: it isn't this app's to stop.
		{"off for another port's", tsSignedIn, servingPort(Port + 1), "off", "tailscale serve --https=443 off"},
		{"on where it already is", tsSignedIn, servingPort(Port), "on", wantOn},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := withStandInTailscale(t, c.status, c.serve)
			d := newTestDeck(t)
			if code, text := tap(t, d, callerPage, "", "", tapBody(c.action, c.command)); code != http.StatusConflict || text == "" {
				t.Errorf("%d %s", code, text)
			}
			if got := s.lines(); len(got) != 0 {
				t.Errorf("it ran %q", got)
			}
		})
	}
	t.Run("not installed", func(t *testing.T) {
		s := withStandInTailscale(t, tsSignedIn, tsNothingOn)
		tailnetCandidates = []string{filepath.Join(s.dir, "no-such-tailscale")}
		d := newTestDeck(t)
		if code, text := tap(t, d, callerPage, "", "", tapBody("on", wantOn)); code != http.StatusConflict || !strings.Contains(text, "isn't installed") {
			t.Errorf("%d %s", code, text)
		}
	})
}

// What Tailscale says when it fails is said, and the status stays what it was.
func TestATapThatTailscaleRefusesSaysWhy(t *testing.T) {
	s := withStandInTailscale(t, tsSignedIn, tsNothingOn)
	s.say("fail", "access denied: use 'sudo tailscale', or set the operator")
	d := newTestDeck(t)
	code, text := tap(t, d, callerPage, "", "", tapBody("on", fmt.Sprintf("tailscale serve --bg --https=443 http://127.0.0.1:%d", Port)))
	if code != http.StatusBadGateway || !strings.Contains(text, "access denied") {
		t.Errorf("%d %s", code, text)
	}
	if said := httpsSaid(t, d); said["status"] != httpsOff {
		t.Errorf("after: %v", said)
	}
}

// The setup page's own owner action, and no one else's: not a tool with the setup key, the GM (and so not MCP, which
// acts as the GM), a hire, nor a stranger; not a phone, nor the office's window. Nothing runs for any of them.
func TestOnlyTheSetupPageCanTurnPhoneOverHTTPSOnOrOff(t *testing.T) {
	s := withStandInTailscale(t, tsSignedIn, tsNothingOn)
	d := newTestDeck(t)
	gm, hireKey := withGMAndAHire(t, d)
	wantOn := fmt.Sprintf("tailscale serve --bg --https=443 http://127.0.0.1:%d", Port)
	for _, who := range []caller{callerTool, callerGM, callerHire, callerNobody} {
		// No one at all is turned away at the door (401), as for any setup route; the rest are told no (403).
		refused := http.StatusForbidden
		if who == callerNobody {
			refused = http.StatusUnauthorized
		}
		if code, text := tap(t, d, who, gm, hireKey, tapBody("on", wantOn)); code != refused {
			t.Errorf("a POST by %s: %d %s", who.name(), code, text)
		}
		for _, target := range []string{"/setup/tailnet/serve?action=on", "/setup/tailnet/status", "/setup/tailnet/other"} {
			c := asCaller(who, gm, hireKey)
			c.method, c.target, c.host, c.from = "GET", target, "localhost:7710", thisComputer
			if got := serve(t, d, c); got.Code != refused {
				t.Errorf("GET %s by %s: %d %s", target, who.name(), got.Code, got.Body)
			}
		}
		// A method the table has no row for goes past the table: it is still the page's alone.
		for _, method := range []string{"PUT", "DELETE", "PATCH"} {
			c := asCaller(who, gm, hireKey)
			c.method, c.target, c.host, c.from, c.body = method, "/setup/tailnet/serve", "localhost:7710", thisComputer, tapBody("on", wantOn)
			c.headers = map[string]string{setupHeader: "1", gmTokenHeader: c.headers[gmTokenHeader], hireKeyHeader: c.headers[hireKeyHeader]}
			if got := serve(t, d, c); got.Code != refused {
				t.Errorf("%s by %s: %d", method, who.name(), got.Code)
			}
		}
	}
	// A tool that also holds the GM's token is the GM here, as for any route kept from a tool.
	c := call{method: "POST", target: "/setup/tailnet/serve", host: "localhost:7710", from: thisComputer, asTool: true, body: tapBody("on", wantOn),
		headers: map[string]string{setupHeader: "1", gmTokenHeader: gm}}
	if got := serve(t, d, c); got.Code != http.StatusForbidden {
		t.Errorf("a tool with the GM's token: %d", got.Code)
	}
	// The page, without its header, as another site would send it: no.
	c = call{method: "POST", target: "/setup/tailnet/serve", host: "localhost:7710", from: thisComputer, body: tapBody("on", wantOn)}
	if got := serve(t, d, c); got.Code == http.StatusOK {
		t.Errorf("the page without its header: %d", got.Code)
	}

	// A phone never reaches setup: over the Wi-Fi, through `tailscale serve`, or at Tailscale's address; nor does the
	// window, which has its key and no cookie.
	d.mu.Lock()
	phone, window := d.access.Phones.Pair("Phone"), d.access.WindowKey
	d.mu.Unlock()
	for _, c := range []call{
		{host: deckOnWiFi, from: aPhone},
		{host: "my-pc.tail1234.ts.net", from: thisComputer, headers: map[string]string{"X-Forwarded-For": "100.101.102.103"}},
		{host: "100.64.1.2:7710", from: "100.101.102.103:50000"},
		{host: "localhost:7710", from: thisComputer},
	} {
		for _, key := range []string{phone, window} {
			c.method, c.target, c.body, c.withoutSetupKey = "POST", "/setup/tailnet/serve?t="+key, tapBody("on", wantOn), true
			headers := map[string]string{setupHeader: "1", hookKeyHeader: key}
			for name, value := range c.headers {
				headers[name] = value
			}
			c.headers = headers
			if got := serve(t, d, c); got.Code == http.StatusOK || got.Code == http.StatusBadGateway {
				t.Errorf("%s from %s: %d", c.host, c.from, got.Code)
			}
			c.headers = nil
		}
	}
	if got := s.lines(); len(got) != 0 {
		t.Errorf("it ran %q", got)
	}
}

// Whatever called it, the run itself is only ever the two commands: never funnel, never another port.
func TestTheRunnerRefusesAnythingButTheTwoCommands(t *testing.T) {
	s := withStandInTailscale(t, tsSignedIn, tsNothingOn)
	for _, args := range [][]string{
		{"funnel", "--bg", fmt.Sprint(Port)},
		{"serve", "--bg", "--https=443", fmt.Sprintf("http://127.0.0.1:%d", Port+1)},
		{"serve", "--bg", "--https=8443", fmt.Sprintf("http://127.0.0.1:%d", Port)},
		{"serve", "--bg", "--funnel", "--https=443", fmt.Sprintf("http://127.0.0.1:%d", Port)},
		{"serve", "--bg", "--https=443", fmt.Sprintf("http://127.0.0.1:%d", Port), "&&", "id"},
		{"serve", "reset"},
		{"set", "--operator=nobody"},
		nil,
	} {
		if _, err := runTailscaleServe(s.tool, args); err == nil {
			t.Errorf("ran %q", args)
		}
	}
	if got := s.lines(); len(got) != 0 {
		t.Errorf("it ran %q", got)
	}
	// The two it does run are the ones shown, word for word, and neither is a funnel.
	for _, args := range [][]string{serveOnArgs(), serveOffArgs()} {
		if strings.Contains(commandLine(args), "funnel") || strings.Fields(commandLine(args))[0] != "tailscale" {
			t.Errorf("%q", commandLine(args))
		}
	}
	if _, err := runTailscaleServe(s.tool, serveOnArgs()); err != nil {
		t.Errorf("the on command: %v", err)
	}
	if got := s.lines(); !slices.Equal(got, []string{strings.TrimPrefix(commandLine(serveOnArgs()), "tailscale ")}) {
		t.Errorf("it ran %q", got)
	}
}

// The tap's route has a row of its own, the page's alone.
func TestThePhoneOverHTTPSRoutesAreThePagesAlone(t *testing.T) {
	for _, route := range [][2]string{{"GET", "/setup/tailnet/status"}, {"GET", "/setup/tailnet/serve"}, {"POST", "/setup/tailnet/serve"}} {
		row := routeFor(route[0], route[1])
		if row == nil || len(row.callers) != 1 || row.callers[0] != "page" {
			t.Errorf("%v: %+v", route, row)
		}
	}
}

// The page's only way to the tap is its Run it button, after the confirmation, and it sends the command it was shown.
func TestThePageAsksBeforeItRunsAnything(t *testing.T) {
	data, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(data)
	if n := strings.Count(page, "fetch('/setup/tailnet/serve', {"); n != 1 {
		t.Fatalf("the page POSTs to the tap in %d places", n)
	}
	i := strings.Index(page, "fetch('/setup/tailnet/serve', {")
	before := page[strings.LastIndex(page[:i], "\n  $('https-go').onclick"):i]
	if !strings.Contains(before, "httpsAction") || !strings.Contains(page[i:i+300], "JSON.stringify({ action, command })") {
		t.Errorf("the tap isn't Run it's, with the command it showed")
	}
	if strings.Contains(strings.ToLower(page), "tailscale funnel") || strings.Contains(page, "serve --funnel") {
		t.Error("the page runs a funnel")
	}
}
