package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHooksReportHereOnlyWhenEveryAgentNamesThisCopy(t *testing.T) {
	everywhere := func(Agent) (bool, string) { return true, "" }
	connected := func(Agent) bool { return true }
	if c := hooksCheck(nil, connected, everywhere); c.Status != checkFix || c.Fix != "" || c.Detail != errNoAgents.Error() {
		t.Errorf("with no agent: %+v, want a fix only you can make", c)
	}
	if c := hooksCheck([]Agent{Claude, Codex}, connected, everywhere); c.Status != checkOK || !strings.Contains(c.Detail, "Claude Code, Codex") {
		t.Errorf("all connected here: %+v", c)
	}
	downloads := func(agent Agent) (bool, string) {
		if agent == Claude {
			return false, `C:/Users/someone/Downloads/Smolcorp/smolcorp-cli.exe hook`
		}
		return true, ""
	}
	notCodex := func(agent Agent) bool { return agent != Codex }
	c := hooksCheck([]Agent{Claude, Codex}, notCodex, downloads)
	if c.Status != checkFix || c.Fix != fixRepair {
		t.Fatalf("hooks to a copy in Downloads: %+v, want the repair", c)
	}
	for _, said := range []string{"Claude Code reports to another copy of Smolcorp (C:/Users/someone/Downloads", "Codex isn't connected"} {
		if !strings.Contains(c.Detail, said) {
			t.Errorf("the check says %q, not %q", c.Detail, said)
		}
	}
}

func TestAHookForAnotherCopyIsFoundAndConnectingPutsItRight(t *testing.T) {
	home := isolate(t)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := connectAgents(true); err != nil {
		t.Fatal(err)
	}
	if here, _ := hooksPointHere(Claude); !here {
		t.Fatal("just connected, the hooks don't name this copy")
	}
	// As a copy run from Downloads left them, in every event.
	settings := filepath.Join(home, ".claude", "settings.json")
	data, _ := os.ReadFile(settings)
	want, _ := entriesFor(Claude).handler["command"].(string)
	elsewhere := `\"C:/Users/someone/DOWNLO~1/SMOLCO~1/smolcorp-cli.exe\" hook`
	quotedWant, _ := json.Marshal(want)
	writeFile(t, settings, strings.ReplaceAll(string(data), strings.Trim(string(quotedWant), `"`), elsewhere))
	here, found := hooksPointHere(Claude)
	if here || !strings.Contains(found, "DOWNLO~1") {
		t.Fatalf("hooks naming a copy in Downloads: here %v, found %q", here, found)
	}
	if !isConnected(Claude) {
		t.Fatal("hooks naming another copy no longer count as connected: the check couldn't say which copy")
	}
	if _, err := connectAgents(true); err != nil {
		t.Fatal(err)
	}
	if here, _ := hooksPointHere(Claude); !here {
		t.Error("connecting again left the hooks naming the other copy")
	}
}

func TestThePhoneIsToldHowItReachesThisComputer(t *testing.T) {
	cases := []struct {
		scope     string
		tailscale []string
		status    string
		fix       string
	}{
		{listenTailscale, []string{"100.101.102.103"}, checkOK, fixHomeWiFi},
		{listenAll, []string{"100.101.102.103"}, checkOK, ""},
		// No phone can reach it: the Wi-Fi is the button, as it works at once.
		{listenTailscale, nil, checkTodo, fixHomeWiFi},
		{listenAll, nil, checkInfo, fixTailscale},
	}
	for _, c := range cases {
		got := networkCheck(c.scope, c.tailscale)
		if got.Status != c.status || got.Fix != c.fix {
			t.Errorf("%s with Tailscale %v: %s, fix %q; want %s, fix %q", c.scope, c.tailscale, got.Status, got.Fix, c.status, c.fix)
		}
		if len(c.tailscale) > 0 && !strings.Contains(got.Detail, c.tailscale[0]) {
			t.Errorf("%s: the detail doesn't give the address: %q", c.scope, got.Detail)
		}
	}
}

// The rules as a Windows tester's PC had them before and after its rules were fixed by hand, and as
// Windows' own "Allow access" box makes them.
const installed = `C:\Users\someone\AppData\Local\Smolcorp\Smolcorp.exe`

var downloadsCopy = `C:\Users\someone\Downloads\Smolcorp-Windows-x86_64\Smolcorp.exe`

func TestTheFirewallIsRightOnlyWithTailscalesRuleForThisCopy(t *testing.T) {
	publicToAll := func(program, protocol string) firewallRule {
		return firewallRule{DisplayName: "smolcorp.exe", Program: program, Profile: "Public", RemoteAddress: "Any",
			Protocol: protocol, Action: "Allow", Enabled: true}
	}
	before := []firewallRule{publicToAll(downloadsCopy, "TCP"), publicToAll(downloadsCopy, "UDP"),
		publicToAll(installed, "TCP"), publicToAll(installed, "UDP")}
	c := firewallVerdict(before, installed, listenTailscale)
	if c.Status != checkFix || c.Fix != fixFirewall {
		t.Fatalf("the tester's rules before: %+v", c)
	}
	for _, said := range []string{"another copy of Smolcorp (" + downloadsCopy, "lets anyone on a public network in", "no rule for Tailscale"} {
		if !strings.Contains(c.Detail, said) {
			t.Errorf("the check says %q, without %q", c.Detail, said)
		}
	}
	afterHisScript := []firewallRule{
		{DisplayName: "smolcorp.exe", Program: installed, Profile: "Private", RemoteAddress: "100.64.0.0/255.192.0.0", Protocol: "TCP", Action: "Allow", Enabled: true},
		{DisplayName: "smolcorp.exe", Program: installed, Profile: "Private", RemoteAddress: "100.64.0.0/255.192.0.0", Protocol: "UDP", Action: "Allow", Enabled: true},
	}
	if c := firewallVerdict(afterHisScript, installed, listenTailscale); c.Status != checkOK {
		t.Errorf("the tester's rules after they were fixed by hand: %+v", c)
	}
	ours := []firewallRule{{DisplayName: firewallTailscaleRule, Program: `%LOCALAPPDATA%\Smolcorp\Smolcorp.exe`, Profile: "Any",
		RemoteAddress: "100.64.0.0/10", Protocol: "TCP", Action: "Allow", Enabled: true}}
	t.Setenv("LOCALAPPDATA", `C:\Users\someone\AppData\Local`)
	if c := firewallVerdict(ours, installed, listenTailscale); c.Status != checkOK || strings.Contains(c.Detail, "home") {
		t.Errorf("the script's rule, its program as %%LOCALAPPDATA%%: %+v", c)
	}
	if c := firewallVerdict(ours, installed, listenAll); c.Status != checkFix || !strings.Contains(c.Detail, "home network") {
		t.Errorf("on the home Wi-Fi too, with no rule for it: %+v", c)
	}
	home := append(ours, firewallRule{DisplayName: firewallHomeRule, Program: installed, Profile: "Private", RemoteAddress: "LocalSubnet",
		Protocol: "TCP", Action: "Allow", Enabled: true})
	if c := firewallVerdict(home, installed, listenAll); c.Status != checkOK {
		t.Errorf("both of the script's rules: %+v", c)
	}
	// A rule Windows' box made since, that lets anyone in, spoils it; one turned off or blocking doesn't.
	allowed := append(append([]firewallRule{}, ours...), publicToAll(installed, "TCP"))
	if c := firewallVerdict(allowed, installed, listenTailscale); c.Status != checkFix {
		t.Errorf("with an Allow access rule beside it: %+v", c)
	}
	off := publicToAll(installed, "TCP")
	off.Enabled = false
	blocking := publicToAll(installed, "UDP")
	blocking.Action = "Block"
	if c := firewallVerdict(append(append([]firewallRule{}, ours...), off, blocking), installed, listenTailscale); c.Status != checkOK {
		t.Errorf("with a rule turned off and one that blocks: %+v", c)
	}
}

func TestOnlyAPhoneCountsAsPairedAndLoginIsOffered(t *testing.T) {
	if c := phoneCheck(0, ""); c.Status != checkTodo || c.Detail != "Open Pair a phone, then scan its QR code with your phone's camera, or type its 6-digit code on your phone." {
		t.Errorf("no phone: %+v", c)
	}
	if c := phoneCheck(2, ""); c.Status != checkOK || !strings.HasPrefix(c.Detail, "2 paired") {
		t.Errorf("two phones: %+v", c)
	}
	// Records whose key failed its check are said, as no phone counts till the owner resets them (records.go).
	if c := phoneCheck(2, recordsText(recordsInterrupted, false)); c.Status != checkTodo || !strings.Contains(c.Detail, "interrupted") || !strings.Contains(c.Detail, "Reset them") {
		t.Errorf("an interrupted update: %+v", c)
	}
	if c := loginCheck(false, true); c.Status != checkTodo || c.Fix != fixLogin {
		t.Errorf("not at login: %+v", c)
	}
	if !ready([]check{{Status: checkOK}, {Status: checkTodo}, {Status: checkInfo}}) || ready([]check{{Status: checkOK}, {Status: checkFix}}) {
		t.Error("only a check to fix is in the way")
	}
}

func TestOpensAtLoginOnlyPromisesATrayWhereThereIsOne(t *testing.T) {
	savedStarted, savedHost := trayStarted.Load(), hasTrayHost
	t.Cleanup(func() { trayStarted.Store(savedStarted); hasTrayHost = savedHost })
	trayStarted.Store(true)
	for _, host := range []bool{true, false} {
		hasTrayHost = func() bool { return host }
		c := loginCheck(true, hasTray())
		if c.Status != checkOK || strings.Contains(c.Detail, "tray") != host {
			t.Errorf("tray host %v: %+v", host, c)
		}
	}
}

func TestTheChecklistIsSetupsOwnAndItsFixesAreByName(t *testing.T) {
	d := newTestDeck(t)
	if refused := serve(t, d, call{method: "GET", target: "/setup/selftest", host: "localhost:7710", from: thisComputer, withoutSetupKey: true}); refused.Code != http.StatusUnauthorized {
		t.Errorf("the checklist without the setup key: %d", refused.Code)
	}
	if away := serve(t, d, call{method: "GET", target: "/setup/selftest", host: deckOnWiFi, from: aPhone}); away.Code != http.StatusNotFound {
		t.Errorf("the checklist from a phone: %d", away.Code)
	}
	got := serve(t, d, call{method: "GET", target: "/setup/selftest", host: "localhost:7710", from: thisComputer})
	var report selfTestAnswer
	if err := json.Unmarshal(got.Body.Bytes(), &report); err != nil || got.Code != http.StatusOK {
		t.Fatalf("the checklist: %d %s", got.Code, got.Body)
	}
	ids := []string{}
	for _, c := range report.Checks {
		ids = append(ids, c.ID)
	}
	if strings.Join(ids, " ") != strings.Join(wantChecks(), " ") {
		t.Errorf("the checks are %v, want %v", ids, wantChecks())
	}
	if report.Ready {
		t.Error("with no agent on this computer, the checklist says it's ready")
	}
	unknown := serve(t, d, call{method: "POST", target: "/setup/fix?fix=teleport", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}})
	if unknown.Code != http.StatusBadRequest {
		t.Errorf("an unknown fix: %d", unknown.Code)
	}
	if noHeader := serve(t, d, call{method: "POST", target: "/setup/fix?fix=repair", host: "localhost:7710", from: thisComputer}); noHeader.Code != http.StatusForbidden {
		t.Errorf("a fix without setup's header: %d", noHeader.Code)
	}
}

// wantChecks is the checklist's lines on this system, in order.
func wantChecks() []string {
	ids := []string{}
	if installCheck() != nil {
		ids = append(ids, "install")
	}
	ids = append(ids, "hooks", "network")
	if firewallCheck(listenAll) != nil {
		ids = append(ids, "firewall")
	}
	return append(ids, "phone", "login", "limits")
}

func TestSetupFromATerminalReadsTheChecklistWithTheCheckOnlyKey(t *testing.T) {
	d := newTestDeck(t)
	server := httptest.NewServer(&router{deck: d})
	defer server.Close()
	client := setupClient{base: server.URL, http: server.Client()}
	// As the app writes it at every start: without it, a terminal has no way in.
	os.Remove(selftestKeyFile)
	if _, err := client.selfTest(); err == nil {
		t.Fatal("read the checklist with no check-only key written")
	}
	writeFile(t, selftestKeyFile, d.access.SelftestKey+"\n")
	if !client.answers() {
		t.Fatal("the test server doesn't answer")
	}
	report, err := client.selfTest()
	if err != nil || len(report.Checks) == 0 {
		t.Fatalf("the checklist from a terminal: %+v, %v", report, err)
	}
	// A tool only repairs: any other fix, even one that isn't, is the page's.
	if err := toolFix(server, d.access.SetupKey, "teleport"); err == nil || !strings.Contains(err.Error(), "This is for the setup page") {
		t.Errorf("an unknown fix from a tool: %v", err)
	}
	// The page asking for one that doesn't exist is told so.
	if response := pagePost(t, d, server.URL, "/setup/fix?fix=teleport"); response.StatusCode != http.StatusBadRequest {
		t.Errorf("an unknown fix from the page: %d, want 400", response.StatusCode)
	} else {
		response.Body.Close()
	}
	defer func(was string) { setupSystem = was }(setupSystem)
	setupSystem = "linux"
	var out bytes.Buffer
	printChecklist(&out, []check{
		{Title: "Your agents report to Smolcorp", Status: checkOK, Detail: "Claude Code."},
		{Title: "A phone is paired", Status: checkTodo, Detail: "Scan the code."},
		{Title: "Your phone can reach this computer", Status: checkTodo, Detail: "Get it.", Fix: fixTailscale, Label: "Get Tailscale"},
	})
	for _, said := range []string{"✓ Your agents report to Smolcorp\n    Claude Code.", "• A phone is paired", "On the setup page: Get Tailscale.", "Ready.", "choose Open Setup… from the tray icon; with no tray, use the link Smolcorp prints as it starts (for a new one, run smolcorp quit (or press Ctrl+C in its terminal), then start Smolcorp again)"} {
		if !strings.Contains(out.String(), said) {
			t.Errorf("the terminal shows\n%s\nwithout %q", out.String(), said)
		}
	}
}

func TestStartHereRetriesAFailedSelfTestAndThenSaysSo(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer})
	if page.Code != http.StatusOK {
		t.Fatalf("the setup page: %d", page.Code)
	}
	html := page.Body.String()
	if !strings.Contains(html, "const startWait = 45000;") {
		t.Error("the self-test wait isn't 45 s")
	}
	start := strings.Index(html, "async function loadStartHere()")
	end := strings.Index(html, "function renderStartHere(")
	if start < 0 || end < start {
		t.Fatal("the page has no loadStartHere")
	}
	load := html[start:end]
	// A failed or slow first check isn't dropped: it's tried again, then Start here is shown with the error and a Retry.
	for _, want := range []string{"setTimeout(loadStartHere, startRetries[", "showStartHereFailed()", "AbortSignal.timeout && AbortSignal.timeout(startWait)", "startRunning"} {
		if !strings.Contains(load, want) {
			t.Errorf("loadStartHere doesn't have %q:\n%s", want, load)
		}
	}
	shown := html[strings.Index(html, "function showStartHereFailed()"):end]
	for _, want := range []string{"$('start-here').hidden = false", "startSaid(", "startButton('Retry'", "if (startRunning) return;"} {
		if !strings.Contains(shown, want) {
			t.Errorf("the failed Start here doesn't have %q", want)
		}
	}
}

func TestSetupsRepairLeavesAgentsYouDisconnectedUntilYouConnect(t *testing.T) {
	d := newTestDeck(t)
	if err := os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(&router{deck: d})
	defer server.Close()
	// Connecting and disconnecting are the page's; the repair is a tool's (a terminal you allowed).
	post := func(path string) {
		t.Helper()
		response := pagePost(t, d, server.URL, path)
		if response.StatusCode >= 400 {
			t.Fatalf("POST %s: %v", path, response)
		}
		response.Body.Close()
	}

	post("/setup/hooks")
	if !isConnected(Claude) {
		t.Fatal("Connect didn't connect")
	}
	post("/setup/hooks?remove=1")
	if isConnected(Claude) {
		t.Fatal("Disconnect didn't disconnect")
	}
	// What a tool's repair does for the hooks line.
	if err := toolFix(server, d.access.SetupKey, fixRepair); err == nil || !strings.Contains(err.Error(), "disconnected") || !strings.Contains(err.Error(), "Connect on the setup page") {
		t.Errorf("setup's repair after a disconnect said %v, want that they are disconnected and how to connect", err)
	}
	if isConnected(Claude) {
		t.Error("setup's repair connected agents you had disconnected")
	}
	// The flag outlives a restart: it's in the config file.
	if !loadConfig().HooksDisconnected {
		t.Error("the disconnect wasn't saved")
	}

	post("/setup/hooks")
	post("/setup/hooks?remove=1")
	post("/setup/hooks")
	if err := toolFix(server, d.access.SetupKey, fixRepair); err != nil || !isConnected(Claude) {
		t.Errorf("setup's repair after a Connect: %v, connected %v", err, isConnected(Claude))
	}
	if loadConfig().HooksDisconnected {
		t.Error("Connect left the disconnect remembered")
	}
}

// The home Wi-Fi restarts Smolcorp and makes a new setup key, so the setup page asks first and says what to do after.
func TestLettingInTheHomeWiFiAsksFirstAndSaysHowToGetTheNewLink(t *testing.T) {
	raw, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	start := strings.Index(page, "async function startFix(fix)")
	if start < 0 {
		t.Fatal("setup.html has no startFix")
	}
	body := page[start:]
	if end := strings.Index(body, "$('start-fix-all')"); end > 0 {
		body = body[:end]
	}
	ask := strings.Index(body, "fix === 'homewifi' && !confirm(")
	post := strings.Index(body, "fetch(`/setup/fix?")
	if ask < 0 || post < 0 || ask > post {
		t.Fatal("startFix does not ask for confirmation for the home Wi-Fi before it posts the fix")
	}
	// 1.24: setup --open no longer makes a link (any command could be a hire's), so it isn't offered.
	if strings.Contains(body[ask:post], "setup --open") {
		t.Error("the home Wi-Fi confirmation still offers smolcorp setup --open, which makes no link in 1.24")
	}
	for _, want := range []string{"Smolcorp restarts", "already open stops working", "Open Setup…", "tray icon", "run smolcorp quit (or press Ctrl+C in its terminal), then start Smolcorp again", "new link"} {
		if !strings.Contains(body[ask:post], want) {
			t.Errorf("the home Wi-Fi confirmation does not say %q", want)
		}
	}
}

// toolFix is a tool's repair, as the route answers it (the fix and auto=1), with key: its error is what the app said.
func toolFix(server *httptest.Server, key, name string) error {
	request, err := http.NewRequest(http.MethodPost, server.URL+"/setup/fix?fix="+name+"&auto=1", nil)
	if err != nil {
		return err
	}
	request.Header.Set(setupKeyHeader, key)
	request.Header.Set(setupHeader, "1")
	response, err := server.Client().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		said, _ := io.ReadAll(io.LimitReader(response.Body, 1000))
		return errors.New(strings.TrimSpace(string(said)))
	}
	return nil
}
