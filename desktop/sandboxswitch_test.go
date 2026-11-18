package desktop

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// SA-X13: the sandbox's switch (AQ1), each desk's tag, words between desks on either side of the sandbox (AQ10,
// §A5.3), and the routes a desk in its sandbox still reaches over TCP.

// The switch is Setup's page's alone: never the GM, a tool, a hire, nobody, an MCP key or a night job. It is no
// phone's route either, so the access dump (phoneRouteAccess) has no row for it.
func TestTheSandboxSwitchIsThePagesAlone(t *testing.T) {
	// Played as a Mac, whose desks start in their sandbox, on every system: off a Mac the switch answers even the page
	// 409, as it has nothing to switch, and who may use it is the same.
	playSandboxedHere(t, true)
	d := newTestDeck(t)
	gm, hireKey := withGMAndAHire(t, d)
	for _, method := range []string{"GET", "POST"} {
		for _, who := range allCallers {
			if got := routeAllows(who, method, "/setup/sandbox"); got != (who == callerPage) {
				t.Errorf("%s: routeAllows(%s) = %v", method, who.name(), got)
			}
			c := asCaller(who, gm, hireKey)
			c.method, c.target, c.host, c.from = method, "/setup/sandbox?on=1", "localhost:7710", thisComputer
			if method == "POST" {
				c.headers = mergeHeaders(c.headers, map[string]string{setupHeader: "1"})
			}
			want := map[caller]int{callerPage: 200, callerNobody: 401}[who]
			if want == 0 {
				want = 403
			}
			if w := serve(t, d, c); w.Code != want {
				t.Errorf("%s as %s: %d %s, want %d", method, who.name(), w.Code, w.Body, want)
			}
		}
		for _, a := range []actor{mcpActor("claude-desktop"), nightActor("n1"), actorGM, actorHire, actorTool} {
			if v := d.decide(a, action{route: &routeAsk{method: method, path: "/setup/sandbox"}}); v.allowed() {
				t.Errorf("%s: %s may use the switch", method, a)
			}
		}
	}
	for _, route := range everyPhoneRoute {
		if strings.HasPrefix(route[1], "/setup/sandbox") {
			t.Errorf("the switch is a phone route: %v", route)
		}
	}
}

func sandboxSwitch(t *testing.T, d *Deck, on string) (int, map[string]any) {
	t.Helper()
	w := serve(t, d, call{method: "POST", target: "/setup/sandbox?on=" + on, host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}})
	var answer map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &answer)
	return w.Code, answer
}

// playSandboxedHere plays a system whose desks start in their sandbox (a Mac), or one whose desks start in none
// (Linux and Windows in 1.26), for the switch and the tags, on whatever system the test runs.
func playSandboxedHere(t *testing.T, here bool) {
	saved, savedCheck := desksSandboxedHere, checkSandbox
	t.Cleanup(func() { desksSandboxedHere, checkSandbox = saved, savedCheck })
	desksSandboxedHere = here
	// Smolcorp's bubblewrap in place, so a Linux run's sandbox line is the row's alone (MSYSTACK4: sandboxLine takes
	// L1APPARMOR1's fix only when that check fails, as TestTheChecklistShowsTheSandboxsOneCommandOnLinux shows).
	checkSandbox = func() error { return nil }
}

func sandboxOff(d *Deck) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.config.SandboxOff
}

// On until the owner turns it off; off is a sealed setting that lasts a restart, and one written into the file by
// another program counts for nothing: it's on again.
func TestTheSandboxIsOnUntilThePageTurnsItOffAndItsSwitchIsSealed(t *testing.T) {
	d, _, _, _ := macDeck(t)
	playSandboxedHere(t, true)
	if sandboxOff(d) {
		t.Fatal("the sandbox starts off")
	}
	if code, answer := sandboxSwitch(t, d, "maybe"); code != 400 || answer != nil {
		t.Errorf("a bad value: %d %v", code, answer)
	}
	code, answer := sandboxSwitch(t, d, "0")
	if code != 200 || answer["on"] != false || answer["here"] != true || !sandboxOff(d) {
		t.Fatalf("off: %d %v", code, answer)
	}
	if part := configSecurityPart(&Config{SandboxOff: true}); !bytes.Contains(part, []byte(`"sandboxOff":true`)) {
		t.Errorf("the switch isn't in the sealed part: %s", part)
	}
	if part := configSecurityPart(&Config{}); bytes.Contains(part, []byte("sandboxOff")) {
		t.Errorf("a file sealed before the switch no longer fits: %s", part)
	}
	if d = restart(t); !sandboxOff(d) {
		t.Fatal("off didn't last a restart: its seal doesn't fit")
	}
	if code, answer := sandboxSwitch(t, d, "1"); code != 200 || answer["on"] != true || sandboxOff(d) {
		t.Errorf("on again: %d %v", code, answer)
	}
	rewrite(t, "config.json", func(f map[string]any) { f["sandboxOff"] = true })
	if d = restart(t); sandboxOff(d) {
		t.Error("the sandbox turned off by another program's write")
	}
}

// While the records' key can't be read, off is taken for the run only, as the other switches that let more happen.
func TestOnAMacWhileTheKeyIsUnreadableTheSandboxSwitchIsHeld(t *testing.T) {
	_, mac, _, _ := macDeck(t)
	playSandboxedHere(t, true)
	mac.recordLook = "unreadable"
	held := restart(t)
	if !recordsHeldOnly() {
		t.Fatalf("the records' state: %q", recordsState())
	}
	if code, _ := sandboxSwitch(t, held, "0"); code != 200 || !sandboxOff(held) {
		t.Errorf("off while the key can't be read: %d", code)
	}
	mac.recordLook = ""
	if d := restart(t); sandboxOff(d) {
		t.Error("off, taken while the key couldn't be read, lasted the restart")
	}
}

// Each open desk with its tag, and the GM's team row says which are in their sandbox.
func TestEachDeskIsTaggedAndTheGMSeesWhichAreSandboxed(t *testing.T) {
	playSandboxedHere(t, true)
	inside := Hire{ID: "in0000000001", Name: "Ada", Sandboxed: true, State: "working"}
	cases := []struct {
		h    Hire
		in   bool
		want string
	}{
		{inside, true, ""},
		{Hire{ID: "old000000001", Name: "Old", State: "working"}, false, "No sandbox: it started without one"},
		// As Take over writes the row it doesn't clear (C6 clears Sandboxed in the same save): its T3 session alone
		// takes the desk out (RT126SAX13 N1, N5).
		{func() Hire { h := inside; h.T3Session = "s"; return h }(), false, "No sandbox: taken over in T3 Code."},
		{Hire{ID: "t3d000000001", Name: "T", Engine: t3Engine, Sandboxed: true}, false, "No sandbox: it runs in T3 Code."},
	}
	for _, c := range cases {
		if inSandbox(c.h) != c.in || !strings.HasPrefix(sandboxTag(c.h), c.want) || (c.want == "") != (sandboxTag(c.h) == "") {
			t.Errorf("%s: in %v, tag %q", c.h.Name, inSandbox(c.h), sandboxTag(c.h))
		}
	}
	d := newTestDeck(t)
	writeRegistryRows(t, inside, cases[1].h, Hire{ID: "gone00000001", Name: "Gone", Sandboxed: true, State: "fired"})
	w := serve(t, d, call{method: "GET", target: "/setup/sandbox", host: "localhost:7710", from: thisComputer})
	var answer struct {
		On, GMByHand bool
		Desks        []struct {
			ID, Name, Tag string
			Sandboxed     bool
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil || w.Code != 200 || !answer.On || answer.GMByHand || len(answer.Desks) != 2 ||
		!answer.Desks[0].Sandboxed || answer.Desks[0].Tag != "" || answer.Desks[1].Sandboxed || answer.Desks[1].Tag != sandboxTag(cases[1].h) {
		t.Errorf("%d %s", w.Code, w.Body)
	}
	rows := teamRows([]Hire{inside, cases[1].h}, nil, nil, 0, func(string) string { return "" })
	if rows[0]["sandboxed"] != true || rows[1]["sandboxed"] != nil {
		t.Errorf("the team rows: %v, %v", rows[0]["sandboxed"], rows[1]["sandboxed"])
	}
}

func writeRegistryRows(t *testing.T, hires ...Hire) {
	t.Helper()
	if err := updateRegistry(func(f *registryFile) { f.hires = hires }); err != nil {
		t.Fatal(err)
	}
}

// sandboxMail is the mail rig with Rev and Twin in their sandbox, Janitor and Twin's twin outside one, the GM in
// its sandbox, and every pair of them allowed to write to each other.
func sandboxMail(t *testing.T) *mailDeck {
	m := newMailDeck(t)
	in := map[string]bool{"a1b2c3": true, "t1": true, "g1": true}
	if err := updateRegistry(func(f *registryFile) {
		for i := range f.hires {
			f.hires[i].Sandboxed = in[f.hires[i].ID]
		}
		for _, from := range []string{"a1b2c3", "d4e5f6", "t1", "t2"} {
			for _, to := range []string{"a1b2c3", "d4e5f6", "t1", "t2"} {
				if from != to {
					f.contacts = append(f.contacts, Contact{From: from, To: to})
				}
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	return m
}

const (
	intoSandboxed = " runs in its sandbox, and a desk without one can't type or send mail into it. Only the owner can."
	outOfSandbox  = " runs without the sandbox, so a desk in its sandbox can't type or send mail into it. Only the owner can."
)

// Mail goes between two desks in their sandbox, or two without one; across, it's refused both ways, with nothing
// kept in the outbox. The owner's goes anywhere.
func TestMailGoesOnlyBetweenDesksOnTheSameSideOfTheSandbox(t *testing.T) {
	m := sandboxMail(t)
	cases := []struct {
		as   who
		to   string
		want string // "" for sent
	}{
		{asRev, "t1", ""},
		{asRev, "gm", ""},
		{asRev, "Janitor", "Janitor" + outOfSandbox},
		{asJanitor, "Rev", "Rev" + intoSandboxed},
		{asJanitor, "t1", "Twin" + intoSandboxed},
		{asJanitor, "gm", "The GM" + intoSandboxed},
		{asJanitor, "t2", ""},
		{asGM, "Rev", ""},
		{asGM, "Janitor", "Janitor" + outOfSandbox},
		{asPage, "Janitor", ""},
		{asPage, "Rev", ""},
	}
	before := len(m.outbox(t, asPage, ""))
	sent := 0
	for i, c := range cases {
		w := m.send(t, c.as, `{"to":"`+c.to+`","text":"hello"}`)
		switch {
		case c.want == "" && w.Code != 200:
			t.Errorf("%d to %s: %d %s", i, c.to, w.Code, w.Body)
		case c.want != "" && (w.Code != 403 || w.Body.String() != c.want):
			t.Errorf("%d to %s: %d %q, want %q", i, c.to, w.Code, w.Body, c.want)
		case c.want == "":
			sent++
		}
	}
	if got := len(m.outbox(t, asPage, "")) - before; got != sent {
		t.Errorf("the outbox holds %d new messages, %d were sent", got, sent)
	}
}

// The GM's brief added to is words for a desk too: across the sandbox, neither its file nor its mail.
func TestTheGMsBriefIsRefusedAcrossTheSandbox(t *testing.T) {
	m := sandboxMail(t)
	folder := t.TempDir()
	brief := filepath.Join(folder, "BRIEF.md")
	if err := os.WriteFile(brief, []byte("# Brief\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := updateRegistry(func(f *registryFile) {
		for i := range f.hires {
			if f.hires[i].ID == "d4e5f6" || f.hires[i].ID == "t2" {
				f.hires[i].Folder, f.hires[i].Brief = folder, brief
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	w := m.call(t, asGM, "POST", "/setup/gm/brief", `{"id":"d4e5f6","text":"More."}`)
	if data, _ := os.ReadFile(brief); w.Code != 403 || w.Body.String() != "Janitor"+outOfSandbox || string(data) != "# Brief\n" {
		t.Errorf("%d %q, the brief: %q", w.Code, w.Body, data)
	}
	if w := m.call(t, asPage, "POST", "/setup/gm/brief", `{"id":"d4e5f6","text":"More."}`); w.Code != 200 {
		t.Errorf("the owner's: %d %s", w.Code, w.Body)
	}
}

// While a GM started by hand is allowed, the GM's token may be held outside any sandbox (`smolcorp gm login`) or by
// the hired GM, and the hired GM's own requests mark it started by hand (noteGM): with the hired GM in its sandbox,
// the GM counts as on either side, and words go neither way (RT126SAX13 B1). With the hired GM outside one, every
// holder is outside.
func TestAGMStartedByHandIsOutsideTheSandbox(t *testing.T) {
	d, _, _, _ := macDeck(t)
	inside := []Hire{{ID: "g1", Name: "Boss", Role: gmRole, State: "working", Sandboxed: true}}
	outside := []Hire{{ID: "g1", Name: "Boss", Role: gmRole, State: "working"}}
	if end := d.gmEnd(inside); !end.inSandbox || end.eitherSide {
		t.Fatalf("the hired GM in its sandbox: %+v", end)
	}
	d.mu.Lock()
	d.config.GMByHand, d.gmHand.seen = true, true
	d.mu.Unlock()
	if end := d.gmEnd(inside); end.inSandbox || !end.eitherSide {
		t.Errorf("a GM started by hand allowed, the hired GM in its sandbox: %+v", end)
	}
	if end := d.gmEnd(outside); end.inSandbox || end.eitherSide {
		t.Errorf("a GM started by hand allowed, the hired GM outside: %+v", end)
	}
	if end := d.gmEnd(nil); end.inSandbox || end.eitherSide {
		t.Errorf("no GM hired: %+v", end)
	}
}

// The reviewer's probe (RT126SAX13 B1): with the owner's "GM by hand" on, the hired GM in its sandbox, whose own
// request is all that marked the token started by hand, mails no desk, in its sandbox or not, and no desk mails it;
// its queued mail isn't typed in either. A bot's mailbox and the owner's mail are no desk's input. With the setting
// off, the hired GM writes to the desks in their sandbox again, and never to one without.
func TestASandboxedHiredGMWritesToNoDeskWhileAGMStartedByHandIsAllowed(t *testing.T) {
	d, _, _, _ := macDeck(t)
	gm := Hire{ID: "g1", Name: "Boss", Role: gmRole, State: "working", Sandboxed: true}
	old := Hire{ID: "old000000001", Name: "Old", Role: "dev", State: "working"}
	in := Hire{ID: "in0000000001", Name: "Ada", Role: "dev", State: "working", Sandboxed: true}
	writeRegistryRows(t, gm, old, in)
	toOld, toIn := mailDest{ID: old.ID, Name: "Old"}, mailDest{ID: in.ID, Name: "Ada"}
	gmMail := mailMessage{From: mailFromGM}
	if r := d.mailSandboxRefusal(callerGM, nil, toIn); r != nil {
		t.Fatalf("the setting off, the GM to a desk in its sandbox: %v", r.Message)
	}
	if r := d.mailSandboxRefusal(callerGM, nil, toOld); r == nil {
		t.Fatal("the setting off, the GM in its sandbox mails a desk without one")
	}
	d.mu.Lock()
	d.config.GMByHand = true
	d.noteGM("") // what server.go's callerAndGM does at each of the GM's requests: the hired GM's own, here
	d.mu.Unlock()
	for _, dest := range []mailDest{toOld, toIn} {
		r := d.mailSandboxRefusal(callerGM, nil, dest)
		if r == nil || r.Status != 403 || r.Message != "The GM can't type or send mail into "+dest.Name+gmOnEitherSide {
			t.Errorf("the setting on, the GM to %s: %+v", dest.Name, r)
		}
	}
	for _, from := range []Hire{old, in} {
		r := d.mailSandboxRefusal(callerHire, &from, mailDest{GM: true, Name: "the GM"})
		if r == nil || r.Status != 403 || r.Message != "A desk can't type or send mail into the GM"+gmOnEitherSide {
			t.Errorf("the setting on, %s to the GM: %+v", from.Name, r)
		}
	}
	hires := []Hire{gm, old, in}
	for _, h := range []Hire{old, in} {
		if why := d.mailDeliveryRefusal(gmMail, h, hires); why == "" {
			t.Errorf("the setting on, the GM's queued mail typed into %s", h.Name)
		}
	}
	if r := d.mailSandboxRefusal(callerGM, nil, mailDest{ID: mailBotPrefix + "b1", Name: "Bot"}); r != nil {
		t.Errorf("the GM's mail for a bot: %v", r.Message)
	}
	if r := d.mailSandboxRefusal(callerPage, nil, toIn); r != nil {
		t.Errorf("the owner's mail: %v", r.Message)
	}
	d.mu.Lock()
	d.config.GMByHand = false
	d.mu.Unlock()
	if d.mailSandboxRefusal(callerGM, nil, toIn) != nil || d.mailSandboxRefusal(callerGM, nil, toOld) == nil ||
		d.mailDeliveryRefusal(gmMail, in, hires) != "" {
		t.Error("the setting off again, the hired GM in its sandbox doesn't write as before")
	}
}

// MARK: Off a Mac (RT126SAX13 B2)

// On a system whose desks start in no sandbox (Linux and Windows in 1.26), the switch never says desks start in
// one, and there is nothing to turn off or on: 409, with nothing saved. Each desk there is tagged so, and Windows's
// words say no sandbox is coming in this release.
func TestOffAMacTheSwitchClaimsNoSandboxAndTurnsNothing(t *testing.T) {
	d := newTestDeck(t)
	playSandboxedHere(t, false)
	writeRegistryRows(t, Hire{ID: "old000000001", Name: "Old", State: "working"})
	for _, on := range []string{"0", "1"} {
		w := serve(t, d, call{method: "POST", target: "/setup/sandbox?on=" + on, host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}})
		if w.Code != 409 || w.Body.String() != noSandboxToSwitch || sandboxOff(d) {
			t.Errorf("on=%s: %d %q, off saved %v", on, w.Code, w.Body, sandboxOff(d))
		}
	}
	w := serve(t, d, call{method: "GET", target: "/setup/sandbox", host: "localhost:7710", from: thisComputer})
	var answer map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil || w.Code != 200 || answer["on"] != false || answer["here"] != false {
		t.Errorf("GET: %d %s", w.Code, w.Body)
	}
	if desks, _ := answer["desks"].([]any); len(desks) != 1 || desks[0].(map[string]any)["tag"] != noSandboxOnThisComputer(runtime.GOOS) {
		t.Errorf("the desks: %v", answer["desks"])
	}
	if got := noSandboxOnThisComputer("windows"); got != "No sandbox on this computer yet." { // SA-X14: SA-W4's words
		t.Errorf("Windows: %q", got)
	}
	if got := noSandboxOnThisComputer("linux"); got != "No sandbox on this computer yet." {
		t.Errorf("Linux: %q", got)
	}
}

// FXSETUPHELPLIMITS (SIERRA's R1 ruling, a known limit for 1.26): Setup's sandbox row tells you not to edit your
// dotfiles while hires work, in SIERRA's words, and why in one line. It sits in the row that says what a desk in its
// sandbox can't change, shown only where desks start in one.
func TestSetupsSandboxRowSaysNotToEditYourDotfilesWhileHiresWork(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	start := strings.Index(page, `<div class="step" id="step-sandbox"`)
	end := strings.Index(page, `<div class="step" id="step-remote-login"`)
	if start < 0 || end < start {
		t.Fatal("the setup page has no sandbox row before Remote Login's")
	}
	row := page[start:end]
	at := strings.Index(row, `<p class="choice-line" id="sandbox-dotfiles">`)
	if at < 0 {
		t.Fatal("the sandbox row has no dotfiles line")
	}
	line := row[at : at+strings.Index(row[at:], "</p>")]
	for _, said := range []string{
		"While hires are working, don't edit your shell or git dotfiles in your home folder, or your project's git settings and hook scripts, with any tool; fire the hires first.",
		"Tools that save through a temporary file give a hire a moment to change that file.",
	} {
		if !strings.Contains(line, said) {
			t.Errorf("the line doesn't say %q: %s", said, line)
		}
	}
	if !strings.Contains(page, "$('step-sandbox').hidden = $('step-remote-login').hidden = $('step-database').hidden = !here;") {
		t.Error("the sandbox row isn't shown only where desks start in a sandbox")
	}
}

// A message queued while both ends were on one side is looked at again at each try: its desk started again in its
// sandbox meanwhile, and it fails with the words, never typed.
func TestQueuedMailIsLookedAtAgainWhenItsDeskStartsOnTheOtherSide(t *testing.T) {
	d := newTestDeck(t)
	r := newPumpRig(t)
	r.pump.across = d.mailDeliveryRefusal
	r.hires = append(r.hires, Hire{ID: "cccccc333333", Name: "Cy", State: "working"})
	req := mailRequest{From: mailFromHire, SignerID: "cccccc333333", SignerName: "Cy", To: "Rev", Text: "hi", ID: "m-1", IDGiven: true,
		Resolve: func() (mailDest, *mailError) { return mailDest{ID: "abcdef123456", Name: "Rev"}, nil }}
	if _, refusal := r.o.add(req); refusal != nil {
		t.Fatal(refusal)
	}
	r.hires[0].Sandboxed = true // Rev started again, in its sandbox
	r.pump.round()
	if m := r.msg("m-1"); len(r.typed) != 0 || m.State != mailFailed || m.Note != "Rev"+intoSandboxed {
		t.Errorf("typed %q, the message %+v", r.typed, m)
	}
	// Both on one side again: typed.
	r.hires[2].Sandboxed = true
	if _, refusal := r.o.add(mailRequest{From: mailFromHire, SignerID: "cccccc333333", SignerName: "Cy", To: "Rev", Text: "again", ID: "m-2", IDGiven: true,
		Resolve: req.Resolve}); refusal != nil {
		t.Fatal(refusal)
	}
	r.pump.round()
	if len(r.typed) != 1 || !strings.HasPrefix(r.typed[0], "abcdef123456 ") || !strings.HasSuffix(r.typed[0], "\n\nagain") {
		t.Errorf("typed %q", r.typed)
	}
}

// A desk that leaves its sandbox (Take over, a restart while it's off) takes authority away: written under a new
// record key, so a copy of the registry from before, that still has it in its sandbox, no longer fits.
func TestADeskLeavingItsSandboxTightensTheRegistry(t *testing.T) {
	in := registryFile{hires: []Hire{{ID: "a", Role: "dev", State: "working", Sandboxed: true}}}
	out := registryFile{hires: []Hire{{ID: "a", Role: "dev", State: "working"}}}
	if !registryTightens(in, out) {
		t.Error("leaving the sandbox doesn't tighten the registry")
	}
	if registryTightens(out, in) {
		t.Error("starting in the sandbox rotates the records' key")
	}
}

// MARK: The routes a desk in its sandbox reaches over TCP

// The list in INVENTORY.md names every route a desk in its sandbox can still use with what it holds (its hook key,
// a hire's send key, the GM's token) or with no key, /terminal/ask among them (RT126SK2's three asks); every route
// the route table gives the GM or a hire is on it, so a new one can't be missed.
func TestTheRoutesADeskInItsSandboxReachesAreListed(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(contractFolder(), "INVENTORY.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, found := strings.Cut(string(data), "\n## What a desk in its sandbox reaches over TCP\n")
	if !found {
		t.Fatal("INVENTORY.md has no list of what a desk in its sandbox reaches")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	listed := map[string]string{} // "METHOD /path" to what it holds
	row := regexp.MustCompile("^\\| ([^|]+) \\| ([A-Z*, ]+) \\| `([^`]+)` \\|")
	for _, line := range strings.Split(section, "\n") {
		if m := row.FindStringSubmatch(line); m != nil {
			for _, method := range strings.Split(m[2], ", ") {
				listed[method+" "+m[3]] = m[1]
			}
		}
	}
	for _, table := range [][]routeRow{routeTable, goOnlyRoutes} {
		for _, r := range table {
			for _, c := range r.callers {
				want := map[string]string{"gm": "the GM's token", "hire": "a hire's send key"}[c]
				if got, ok := listed[r.method+" "+r.path]; want != "" && (!ok || !strings.Contains(got, want)) {
					t.Errorf("%s %s (%s) isn't listed with %q: %q", r.method, r.path, c, want, got)
				}
			}
		}
	}
	for _, keyless := range []string{"POST /terminal/ask", "GET /terminal/ask", "POST /setup/tap", "GET /icon.png", "GET /office.js", "GET /sign.js"} {
		if got := listed[keyless]; got != "nothing" {
			t.Errorf("%s: %q, want it listed as reached with nothing", keyless, got)
		}
	}
	if got := listed["POST /hook"]; got != "its own hook key" {
		t.Errorf("POST /hook: %q", got)
	}
	// Nothing listed that the table doesn't give: a GM or hire row is one of the table's.
	for route, holds := range listed {
		method, path, _ := strings.Cut(route, " ")
		if strings.HasPrefix(path, "/setup/") && path != "/setup/tap" && path != "/setup/session" {
			r := routeFor(method, path)
			if r == nil || !slices.ContainsFunc(r.callers, func(c string) bool { return c == "gm" || c == "hire" }) {
				t.Errorf("%s is listed (%s), and the route table gives it no desk", route, holds)
			}
		}
	}
}
