package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// SA-X14: Setup's Sandbox row, its two routes, and its line in Start here.

func sandboxAnswer(t *testing.T, d *Deck) map[string]any {
	t.Helper()
	w := serve(t, d, call{method: "GET", target: "/setup/sandbox", host: "localhost:7710", from: thisComputer})
	var answer map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil || w.Code != 200 {
		t.Fatalf("GET /setup/sandbox: %d %s", w.Code, w.Body)
	}
	return answer
}

func pagePOST(t *testing.T, d *Deck, target string) (int, string) {
	t.Helper()
	w := serve(t, d, call{method: "POST", target: target, host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	return w.Code, w.Body.String()
}

// The opt-in and the restart are Setup's page's alone, as the switch is (AQ1): never the GM, a tool, a hire, nobody,
// an MCP key or a night job, and no phone's route (phoneRouteAccess has no row for them).
func TestTheSandboxRowsRoutesAreThePagesAlone(t *testing.T) {
	playSandboxedHere(t, true)
	freshChecks(t)
	d := newTestDeck(t)
	gm, hireKey := withGMAndAHire(t, d)
	page := map[string]int{"/setup/sandbox/database?on=0": 200, "/setup/sandbox/restart?id=nobody000001": 404,
		"/setup/sandbox/gitplant?id=0123456789abcdef": 404}
	if runtime.GOOS == "windows" {
		page["/setup/sandbox/restart?id=nobody000001"] = 409 // no sandbox to restart a desk in
	}
	for target, pageGets := range page {
		path, _, _ := strings.Cut(target, "?")
		for _, who := range allCallers {
			if got := routeAllows(who, "POST", path); got != (who == callerPage) {
				t.Errorf("routeAllows(%s, %s) = %v", who.name(), path, got)
			}
			c := asCaller(who, gm, hireKey)
			c.method, c.target, c.host, c.from = "POST", target, "localhost:7710", thisComputer
			c.headers = mergeHeaders(c.headers, map[string]string{setupHeader: "1"})
			want := map[caller]int{callerPage: pageGets, callerNobody: 401}[who]
			if want == 0 {
				want = 403
			}
			if w := serve(t, d, c); w.Code != want {
				t.Errorf("%s as %s: %d %s, want %d", target, who.name(), w.Code, w.Body, want)
			}
		}
		for _, a := range []actor{mcpActor("claude-desktop"), nightActor("n1"), actorGM, actorHire, actorTool} {
			if v := d.decide(a, action{route: &routeAsk{method: "POST", path: path}}); v.allowed() {
				t.Errorf("%s may use %s", a, path)
			}
		}
	}
	for _, route := range everyPhoneRoute {
		if strings.HasPrefix(route[1], "/setup/sandbox") {
			t.Errorf("a sandbox route is a phone route: %v", route)
		}
	}
}

// AQ21's opt-in: taken once no local database lets Smolcorp in without a password, and said in the row; refused with
// what got in otherwise, and off then; off always taken. Off a Mac there's no sandbox to apply it to.
func TestTheDatabaseOptInIsTakenOnlyOnceAPasswordlessLoginFails(t *testing.T) {
	playSandboxedHere(t, true)
	freshChecks(t)
	d := newTestDeck(t)
	if answer := sandboxAnswer(t, d); answer["databaseOptIn"] != false {
		t.Errorf("on before it was asked: %v", answer["databaseOptIn"])
	}
	if code, body := pagePOST(t, d, "/setup/sandbox/database?on=1"); code != 200 || !strings.Contains(body, `"databaseOptIn":true`) {
		t.Errorf("on, nothing lets it in: %d %s", code, body)
	}
	if code, body := pagePOST(t, d, "/setup/sandbox/database?on=0"); code != 200 || !strings.Contains(body, `"databaseOptIn":false`) {
		t.Errorf("off: %d %s", code, body)
	}
	if code, _ := pagePOST(t, d, "/setup/sandbox/database?on=yes"); code != 400 {
		t.Errorf("on=yes: %d", code)
	}
	// A place that answers, but not as a database it can tell: it can't be told, so the opt-in stays off.
	quickChecks(t)
	closes := checkStandIn(t, func(net.Conn) {})
	onlyPlaces(t, dbPlace{Kind: "Postgres", Network: "tcp", Address: "127.0.0.1:" + strconv.Itoa(closes)})
	if code, body := pagePOST(t, d, "/setup/sandbox/database?on=1"); code != 409 || !strings.Contains(body, "Postgres") {
		t.Errorf("a place it can't tell: %d %s", code, body)
	}
	if answer := sandboxAnswer(t, d); answer["databaseOptIn"] != false {
		t.Errorf("on after a refusal: %v", answer["databaseOptIn"])
	}
	playSandboxedHere(t, false)
	if code, body := pagePOST(t, d, "/setup/sandbox/database?on=0"); code != 409 || body != noSandboxToSwitch {
		t.Errorf("off a Mac: %d %q", code, body)
	}
}

// Start here's Sandbox line on a Mac: on, with every desk in its sandbox; to do while the sandbox is off or desks run
// without it, by count, never by name (the self-test is a tool's too); its place before the signing key's advice.
// Off a Mac: no sandbox on this computer yet, with what Smolcorp does instead, never in the way of being ready.
func TestStartHereSaysWhetherDesksRunInTheirSandbox(t *testing.T) {
	playSandboxedHere(t, true)
	freshChecks(t)
	d := newTestDeck(t)
	line := func() check {
		t.Helper()
		for _, c := range d.selfTest() {
			if c.ID == "sandbox" {
				return c
			}
		}
		t.Fatal("no sandbox line")
		return check{}
	}
	if c := line(); c.Status != checkOK || !strings.Contains(c.Detail, "can't read Smolcorp's keys") {
		t.Errorf("no desks: %+v", c)
	}
	writeRegistryRows(t, Hire{ID: "in0000000001", Name: "Ada", Sandboxed: true, State: "working"},
		Hire{ID: "old000000001", Name: "Secretname", State: "working"}, Hire{ID: "old000000002", Name: "Gone", State: "fired"})
	if c := line(); c.Status != checkTodo || !strings.Contains(c.Detail, "while 1 desk runs without it") || strings.Contains(c.Detail, "Secretname") {
		t.Errorf("one desk without it: %+v", c)
	}
	if answer := sandboxAnswer(t, d); answer["unsandboxed"] != float64(1) {
		t.Errorf("the row's count: %v", answer["unsandboxed"])
	}
	if code, _ := pagePOST(t, d, "/setup/sandbox?on=0"); code != 200 {
		t.Fatal(code)
	}
	if c := line(); c.Status != checkTodo || !strings.HasPrefix(c.Detail, "Off:") {
		t.Errorf("the sandbox off: %+v", c)
	}
	playSandboxedHere(t, false)
	c := line()
	if c.Status != checkInfo || c.Title != "No sandbox on this computer yet" || c.Detail != noSandboxHere || !ready([]check{c}) {
		t.Errorf("off a Mac: %+v", c)
	}
	// Off a Mac the limits line stays last, the sandbox's just before it.
	checks := d.selfTest()
	if n := len(checks); d.macHost() == nil && (checks[n-1].ID != "limits" || checks[n-2].ID != "sandbox") {
		t.Errorf("the order: %v", checks)
	}
}

// Linux and Windows say plainly that there's no sandbox, desk by desk and in the row, and what Smolcorp does instead:
// the same words in the page and the self-test, and nothing that says or hints a sandbox holds a desk there: not a
// desk's tag, whatever the desk (a T3 Code one included: "No sandbox: it runs in T3 Code" would hint the others have
// one), and not the end of the brief each desk is given, which says nothing of Smolcorp's sandbox and turns no tool's
// own sandbox off (RT126SAX14 B1).
func TestOffAMacNothingClaimsASandbox(t *testing.T) {
	isolate(t)
	playSandboxedHere(t, false)
	for _, goos := range []string{"linux", "windows"} {
		if got := noSandboxOnThisComputer(goos); got != "No sandbox on this computer yet." {
			t.Errorf("%s: %q", goos, got)
		}
	}
	every := []Hire{
		{ID: "old000000001", Name: "Old", State: "working"},
		{ID: "in0000000001", Name: "Ada", Sandboxed: true, State: "working"},
		{ID: "tko000000001", Name: "Taken", Sandboxed: true, State: "working", T3Session: "s"},
		{ID: "t3d000000001", Name: "T", Engine: t3Engine, State: "working"},
		{ID: "gm0000000001", Name: "GM", Role: gmRole, State: "working"},
	}
	for _, h := range every {
		if tag := sandboxTag(h); tag != "No sandbox on this computer yet." {
			t.Errorf("%s's tag: %q", h.Name, tag)
		}
	}
	d := newTestDeck(t)
	writeRegistryRows(t, every...)
	for _, desk := range sandboxAnswer(t, d)["desks"].([]any) {
		if row := desk.(map[string]any); row["tag"] != "No sandbox on this computer yet." || row["restart"] != nil {
			t.Errorf("the row's desk: %v", row)
		}
	}
	for _, goos := range []string{"linux", "windows"} {
		if text := briefSandboxTools(goos, false); text != "" {
			t.Errorf("%s: the brief ends with %q", goos, text)
		}
	}
	role := Role{ID: "tester", Name: "Tester", Agent: "claude", Brief: &RoleBrief{Kind: "library", ID: "tester"}}
	made, err := makeBrief(briefRequest{role: role, library: briefLibrary{files: bundledBriefs()}, folder: t.TempDir(),
		date: time.Date(2027, 1, 15, 8, 0, 0, 0, time.UTC), kept: briefsKeptDir(), write: true})
	if err != nil || made == nil {
		t.Fatalf("the brief: %v", err)
	}
	for _, claim := range []string{"Smolcorp's sandbox", "Smolcorp's,", "--no-sandbox", "--disable-sandbox", "Tools with a sandbox", "inside"} {
		if strings.Contains(made.markdown, claim) {
			t.Errorf("a desk's brief here says %q:\n%s", claim, made.markdown)
		}
	}
	for _, said := range []string{"the fence and the guard", "Setup's key stays in Smolcorp's memory", "keys of its own",
		"only in its own desk's environment", "Keys, records and the guard"} {
		if !strings.Contains(noSandboxHere, said) {
			t.Errorf("the protections don't say %q", said)
		}
	}
	for _, claim := range []string{"in its sandbox", "sandboxed", "sealed", "protected by"} {
		if strings.Contains(noSandboxHere, claim) {
			t.Errorf("off a Mac, the row says %q", claim)
		}
	}
	page, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "<div class=\"detail\">"+noSandboxHere+"</div>") {
		t.Error("Setup's row and Start here say different things off a Mac")
	}
	// The GM is told when it starts outside its sandbox (§A5.2), in words that don't make it turn the sandbox back on.
	if !strings.Contains(gmSandboxOffLine, "Only the owner can turn it back on") {
		t.Errorf("the GM's line: %q", gmSandboxOffLine)
	}
}

// freshGitPlants starts a test with no old git plants kept, and puts back what was kept after it.
func freshGitPlants(t *testing.T) {
	t.Helper()
	oldGitPlants.Lock()
	saved := oldGitPlants.of
	oldGitPlants.of = map[string]gitPlantsFound{}
	oldGitPlants.Unlock()
	t.Cleanup(func() { oldGitPlants.Lock(); oldGitPlants.of = saved; oldGitPlants.Unlock() })
}

// rowPlants is GET /setup/sandbox's old git plants, by file and key.
func rowPlants(t *testing.T, d *Deck) map[string]map[string]any {
	t.Helper()
	plants := map[string]map[string]any{}
	for _, p := range sandboxAnswer(t, d)["gitPlants"].([]any) {
		row := p.(map[string]any)
		key, _ := row["key"].(string)
		plants[row["file"].(string)+" "+key] = row
	}
	return plants
}

// RT126SAX14 B2 (SANDBOX-PLAN §A5.3, NB2): the old git plants desk starts find in a repo's shared .git are shown in
// Setup's row, once each however many desks found them, with a "Remove" that takes out only what was listed, as it
// was. One that changed since is refused, nothing removed, and looked at again; one Smolcorp can't remove says to
// look at it yourself. Start here counts them, never naming one. Smolcorp removes nothing by itself.
func TestOldGitPlantsAreShownInTheRowAndRemovedOnlyByTheOwner(t *testing.T) {
	playSandboxedHere(t, true)
	freshChecks(t)
	freshGitPlants(t)
	d := newTestDeck(t)
	if plants := rowPlants(t, d); len(plants) != 0 {
		t.Fatalf("before any start: %v", plants)
	}
	git := gitPlantTree(t)
	listed := findOldGitPlants(context.Background(), git)
	keepOldGitPlants("aaaa00000001", git, listed)
	keepOldGitPlants("aaaa00000002", git, findOldGitPlants(context.Background(), git))
	plants := rowPlants(t, d)
	hook, pushHook := filepath.Join(git, "hooks", "pre-commit")+" ", filepath.Join(git, "modules", "sub", "hooks", "pre-push")+" "
	if n := len(sandboxAnswer(t, d)["gitPlants"].([]any)); n != len(listed) || plants[hook] == nil || plants[hook]["removable"] != true ||
		plants[filepath.Join(git, "config")+" core.fsmonitor"]["removable"] != true {
		t.Fatalf("the row's plants (%d listed, once each): %v", len(listed), plants)
	}
	for _, c := range d.selfTest() {
		if c.ID == "sandbox" && (c.Status != checkTodo || !strings.Contains(c.Detail, strconv.Itoa(len(listed))+" hooks or settings") ||
			strings.Contains(c.Detail, "pre-commit") || strings.Contains(c.Detail, git)) {
			t.Errorf("Start here: %+v", c)
		}
	}
	if _, err := os.Stat(strings.TrimSpace(hook)); err != nil {
		t.Fatalf("something was removed before a Remove: %v", err)
	}
	// Remove: the hook goes, and the row shows what's there now.
	if code, body := pagePOST(t, d, "/setup/sandbox/gitplant?id="+plants[hook]["id"].(string)); code != 200 || strings.Contains(body, `hooks/pre-commit"`) {
		t.Errorf("Remove: %d %s", code, body)
	}
	if _, err := os.Lstat(strings.TrimSpace(hook)); !os.IsNotExist(err) {
		t.Errorf("the hook stayed: %v", err)
	}
	if code, body := pagePOST(t, d, "/setup/sandbox/gitplant?id="+plants[hook]["id"].(string)); code != 404 || body != gitPlantGone {
		t.Errorf("again: %d %s", code, body)
	}
	// Changed since it was listed: nothing removed, and looked at again, so the next Remove is of what's there now.
	if err := os.WriteFile(strings.TrimSpace(pushHook), []byte("#!/bin/sh\necho changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, body := pagePOST(t, d, "/setup/sandbox/gitplant?id="+plants[pushHook]["id"].(string)); code != 409 || body != gitPlantGone {
		t.Errorf("changed: %d %s", code, body)
	}
	if data, err := os.ReadFile(strings.TrimSpace(pushHook)); err != nil || !strings.Contains(string(data), "changed") {
		t.Fatalf("a changed hook was touched: %v", err)
	}
	if code, _ := pagePOST(t, d, "/setup/sandbox/gitplant?id="+rowPlants(t, d)[pushHook]["id"].(string)); code != 200 {
		t.Errorf("once looked at again: %d", code)
	}
	// One Smolcorp can't remove: said, with its file, and nothing touched.
	empty := t.TempDir()
	unread := gitPlant(oldGitPlant{File: filepath.Join(empty, "modules"), Why: whyUnread})
	keepOldGitPlants("aaaa00000003", empty, []oldGitPlant{unread})
	if row := rowPlants(t, d)[unread.File+" "]; row == nil || row["removable"] != false {
		t.Errorf("one it can't remove: %v", row)
	}
	if code, body := pagePOST(t, d, "/setup/sandbox/gitplant?id="+unread.ID); code != 409 || body != "Smolcorp couldn't remove it, so look at it yourself: "+unread.File {
		t.Errorf("one it can't remove: %d %s", code, body)
	}
}

// RT126SAX14 nit 4: Start here, which a tool and the check-only key ask too, dials nothing: it says what the last look
// found. Setup's row looks again once the last answer is old, and callers at the same moment dial once between them.
func TestStartHereDialsNothingAndOneLookAtATimeDials(t *testing.T) {
	playSandboxedHere(t, true)
	freshChecks(t) // every look finds the last answer old
	d := newTestDeck(t)
	var mu sync.Mutex
	dials := 0
	port := ":" + strconv.Itoa(remoteLoginPort)
	dial := checkDial
	t.Cleanup(func() { checkDial = dial })
	checkDial = func(_ context.Context, _, address string) (net.Conn, error) {
		if strings.HasSuffix(address, port) {
			mu.Lock()
			dials++
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
		}
		return nil, errors.New("stand-in: no network")
	}
	count := func() int { mu.Lock(); defer mu.Unlock(); n := dials; dials = 0; return n }
	deskStartChecks("in0000000001", false)
	one := count()
	if one == 0 {
		t.Fatal("a desk's start dialed nothing")
	}
	d.selfTest()
	if n := count(); n != 0 {
		t.Errorf("Start here dialed %d times", n)
	}
	sandboxAnswer(t, d)
	if n := count(); n != one {
		t.Errorf("the row's look dialed %d times, not %d", n, one)
	}
	checked.Lock()
	checked.remoteLoginNew, checked.remoteLoginAt = time.Hour, time.Now().Add(-2*time.Hour)
	checked.Unlock()
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; restartToApply(false) }()
	}
	close(start)
	wg.Wait()
	if n := count(); n != one {
		t.Errorf("8 looks at once dialed %d times, not %d", n, one)
	}
}
