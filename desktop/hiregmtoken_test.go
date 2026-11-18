//go:build !windows

package desktop

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

// The GM's token: made new at its hire through the engine, in its own start
// script alone (after the script unsets any it inherited), never beside a send
// key; taken away when the hire doesn't go ahead, never starts, or is fired.

func TestTheEngineGMsScriptAloneHoldsItsNewToken(t *testing.T) {
	r := newEngineRig(t)
	gm, err := runManagedHire(r.request("gm"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	script := r.script(gm)
	unset, export := strings.Index(script, "unset SMOLCORP_GM_TOKEN\n"), strings.Index(script, "export SMOLCORP_GM_TOKEN='fake-gm-token-1'\n")
	if unset < 0 || export < unset || scriptSendKey(script) != "" {
		t.Errorf("the GM's script:\n%s", script)
	}
	if key := readRegistry(registryPath(), false).keys[gm.ID]; key != "" {
		t.Error("the GM has a send key in the registry")
	}
	other, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.script(other), "SMOLCORP_GM_TOKEN=") {
		t.Errorf("another hire's script holds the GM's token:\n%s", r.script(other))
	}
	// Told who it is; no plugin bundled beside this stand-in program, so none is loaded.
	if !strings.Contains(script, "You are the General Manager of this team. Your folder is ") || strings.Contains(script, "--plugin-dir") {
		t.Errorf("the GM's introduction, without a plugin:\n%s", script)
	}
	if strings.Contains(r.script(other), "General Manager of this team") {
		t.Error("another hire is told it's the GM")
	}

	// Bundled beside the program, as in Smolcorp.app: the GM loads it, and its tools run this program.
	writeFile(t, filepath.Join(filepath.Dir(filepath.Dir(r.o.CLI)), "gm", "plugin", ".claude-plugin", "plugin.json"), "{}")
	gm, err = runManagedHire(r.request("gm"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	if script := r.script(gm); !strings.Contains(script, "--agent=smolcorp-gm:general-manager") || !strings.Contains(script, "export SMOLCORP_CLI=") {
		t.Errorf("the GM's script with the plugin bundled:\n%s", script)
	}
}

func TestAGMHireWithNoTokenIsRefusedAndLeavesNothing(t *testing.T) {
	r := newEngineRig(t)
	request := r.request("gm")
	for name, token := range map[string]func() string{"none asked": nil, "an empty one": func() string { return "" }} {
		request.GMToken = token
		if _, err := runManagedHire(request, r.o); err == nil || !strings.Contains(err.Error(), "General Manager's token") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if file := readRegistry(registryPath(), false); len(file.hires) != 0 || len(file.keys) != 0 || branches(t, r.repo) != "" || len(r.tmux.asked) != 0 {
		t.Errorf("left behind: %+v %v %q, tmux %q", file.hires, file.keys, branches(t, r.repo), r.tmux.asked)
	}
}

func TestOnlyTheGMsScriptMayHoldTheTokenAndNeverWithASendKey(t *testing.T) {
	for name, start := range map[string]hireStart{
		"another role":      {Role: Role{ID: "janitor", Name: "Janitor", Agent: "claude"}, Message: "x", GMToken: "fake-gm-token-1"},
		"the GM with a key": {Role: Role{ID: "gm", Name: "GM", Agent: "claude"}, Message: "x", GMToken: "fake-gm-token-1", SendKey: "k"},
	} {
		if script, err := start.startText(); err == nil {
			t.Errorf("%s: a script was made:\n%s", name, script)
		}
	}
}

func TestADeckGMHireGetsItsTokenAndAFireTakesItAway(t *testing.T) {
	d, r, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	if status, message := d.Perform("hire", "", url.Values{"role": {"gm"}}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	d.watching.Wait()
	hires := registered(t)
	if len(hires) != 1 || hires[0].State != "working" {
		t.Fatalf("registry: %+v", hires)
	}
	d.mu.Lock()
	token := d.gmToken
	d.mu.Unlock()
	if token == "" || !strings.Contains(r.script(hires[0]), "export SMOLCORP_GM_TOKEN='"+token+"'\n") {
		t.Fatalf("the GM's token %q isn't in its script:\n%s", token, r.script(hires[0]))
	}
	windowShows(r, pane(t, "idle-empty"), true)
	d.engine.Tmux = r.o.Tmux
	if status, message, handled := d.fireTab("tmux:%1@"+hires[0].TmuxSocket, false); !handled || status != http.StatusNoContent {
		t.Fatalf("fire: %d %q %v", status, message, handled)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.isGMToken(token) || d.gmToken != "" {
		t.Error("a fired GM's token still works")
	}
}

func TestAGMThatDoesntGoAheadOrNeverStartsTakesItsTokenAway(t *testing.T) {
	d, r, _ := engineDeck(t)
	r.tmux.starts = false
	d.engine.Tmux = r.tmux.run
	if status, _ := d.Perform("hire", "", url.Values{"role": {"gm"}}); status != http.StatusInternalServerError {
		t.Fatalf("a GM whose tmux didn't start: %d", status)
	}
	d.mu.Lock()
	if d.gmToken != "" {
		t.Error("a refused GM hire left its token working")
	}
	d.mu.Unlock()
	// One that starts but never reports is cleaned up, and its token with it.
	r.tmux.starts = true
	d.lastHire = d.lastHire.AddDate(-1, 0, 0)
	if status, message := d.Perform("hire", "", url.Values{"role": {"gm"}}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	d.watching.Wait()
	if hires := registered(t); len(hires) != 1 || hires[0].State != "failed" {
		t.Fatalf("registry: %+v", hires)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.gmToken != "" {
		t.Error("a GM that never started left its token working")
	}
}

func TestClearingAnOldGMsTokenNeverTakesANewerOne(t *testing.T) {
	d := newTestDeck(t)
	old := d.newGMToken()
	newer := d.newGMToken()
	d.clearGMTokenIf(old)
	if d.gmToken != newer {
		t.Fatal("an old GM's token took the newer one away")
	}
	d.clearGMTokenIf(newer)
	if d.gmToken != "" {
		t.Error("the GM's own token wasn't taken away")
	}
}

// A fired hire's waiting mail fails at the fire, each "<name> is gone."; mail
// to anyone else, or already sent, is left alone.
func TestAFireFailsTheHiresWaitingMailAtOnce(t *testing.T) {
	d, r, hire := hiredForFire(t, "")
	to := func(id, name, msg string) mailRequest {
		return mailRequest{From: mailFromGM, To: name, Text: "hello", ID: msg, IDGiven: true,
			Resolve: func() (mailDest, *mailError) { return mailDest{ID: id, Name: name}, nil }}
	}
	for _, req := range []mailRequest{to(hire.ID, "Janitor", "m1"), to(hire.ID, "Janitor", "m2"), to("abcdef123456", "Rev", "m3"), to(hire.ID, "Janitor", "m4")} {
		if _, err := d.mail.add(req); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// m4 has a try out: it is left for that try to finish.
	d.mail.mu.Lock()
	d.mail.inFlight["m4"] = true
	d.mail.mu.Unlock()
	d.config.HiringEnabled = true
	windowShows(r, pane(t, "idle-empty"), true)
	d.engine = r.o
	if status, message, handled := d.fireTab("tmux:%1@"+hire.TmuxSocket, false); !handled || status != http.StatusNoContent {
		t.Fatalf("fire: %d %q %v", status, message, handled)
	}
	for id, want := range map[string]mailState{"m1": mailFailed, "m2": mailFailed, "m3": mailQueued, "m4": mailQueued} {
		m, ok := d.mail.get(id)
		if !ok || m.State != want || (want == mailFailed && m.Note != "Janitor is gone.") {
			t.Errorf("%s: %+v, want %s", id, m, want)
		}
	}
}

// A GM hired the moment the old one was marked fired keeps its token: the
// fire takes the token away only when no GM is open any more.
func TestAFiredGMNeverTakesANewerGMsToken(t *testing.T) {
	d := newTestDeck(t)
	file := readRegistry(registryPath(), true)
	file.hires = append(file.hires, Hire{ID: "gmold", Name: "GM", Role: "gm", Folder: homeDir, State: "fired"},
		Hire{ID: "gmnew", Name: "GM", Role: "gm", Folder: homeDir, State: "starting"})
	if err := file.save(registryPath()); err != nil {
		t.Fatal(err)
	}
	newer := d.newGMToken()
	d.clearFiredGMToken()
	if d.gmToken != newer {
		t.Fatal("the old GM's fire took the newer GM's token away")
	}
	file = readRegistry(registryPath(), true)
	file.hires[1].State = "fired"
	if err := file.save(registryPath()); err != nil {
		t.Fatal(err)
	}
	d.clearFiredGMToken()
	if d.gmToken != "" {
		t.Error("with no GM open, the token still works")
	}
}

// Mail to a hire still starting (its row is written before its tmux server)
// waits; to an open hire with no server of its own, it can't go.
func TestMailToAHireStillStartingWaits(t *testing.T) {
	shortSupport(t)
	tmuxPath = func() string { return "/usr/bin/tmux" }
	d := newTestDeck(t)
	if r := d.mailTypeIntoHire(Hire{ID: "ab12cd34ef56", Name: "Janitor", State: "starting"}, "hi", func() bool { return true }); r.Outcome != mailWaiting {
		t.Errorf("starting: %+v", r)
	}
	if r := d.mailTypeIntoHire(Hire{ID: "ab12cd34ef56", Name: "Janitor", State: "working"}, "hi", func() bool { return true }); r.Outcome != mailCannot {
		t.Errorf("working with no socket: %+v", r)
	}
}
