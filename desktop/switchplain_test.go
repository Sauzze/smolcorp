package desktop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// FXSWITCHPLAIN (the owner's Q8 = (a)): Setup's sandbox switch covers plain hires too. Off, a Mac's plain hire (another
// agent, no role, a Claude role with the manager off) starts outside its sandbox, tagged No sandbox, as the engine's
// desks started then do; on, it starts in one, as before. Linux and Windows: no plain hire has one, either way.

func TestAPlainHireIsSandboxedWhereTheLauncherMakesOneUnlessTheSwitchIsOff(t *testing.T) {
	if got := plainHireSandboxed(Config{}); got != launcherSandboxes {
		t.Errorf("the switch on: sandboxed %v, want %v (a Mac's launcher makes one)", got, launcherSandboxes)
	}
	if plainHireSandboxed(Config{SandboxOff: true}) {
		t.Error("the switch off: a plain hire is still sandboxed")
	}
}

// The launcher takes its sandbox away only when the core said so at the hire and the config's file says so as it
// starts: a changed file alone, or the word alone, leaves it on.
func TestALaunchersSandboxGoesOnlyWhenTheCoreAndTheFileBothSayOff(t *testing.T) {
	isolate(t)
	token := strings.Repeat("AB", 16)
	for _, c := range []struct {
		row           string
		said, fileOff bool
		want          bool
	}{
		{"neither", false, false, launcherSandboxes},
		{"the file alone", false, true, launcherSandboxes},
		{"the word alone", true, false, launcherSandboxes},
		{"both", true, true, false},
	} {
		writeFile(t, configFile, map[bool]string{true: `{"sandboxOff": true}`, false: `{}`}[c.fileOff])
		if got := (&launcherRun{hire: token, command: []string{"codex"}, sandboxOff: c.said}).sandboxed(); got != c.want {
			t.Errorf("%s: sandboxed %v, want %v", c.row, got, c.want)
		}
	}
	// An engine hire's agent starts in its desk's tmux, and one run by hand in no sandbox: the launcher makes none.
	writeFile(t, configFile, `{}`)
	if (&launcherRun{hire: token, hireID: "h1", role: "janitor"}).sandboxed() || (&launcherRun{command: []string{"codex"}}).sandboxed() {
		t.Error("the launcher would sandbox an engine hire's agent, or one run by hand")
	}
}

func TestSandboxOffIsAPlainHiresWordAlone(t *testing.T) {
	token := strings.Repeat("AB", 16)
	run, err := parseRun([]string{"--hire", token, "--pinned", "--sandbox-off", "codex", "--yolo"})
	if err != nil || !run.sandboxOff || !run.pinned || !slices.Equal(run.command, []string{"codex", "--yolo"}) {
		t.Errorf("a plain hire's start: %+v, %v", run, err)
	}
	if run, err = parseRun([]string{"--hire", token, "--agent", "codex", "--role", "tester"}); err != nil || run.sandboxOff {
		t.Errorf("without the word: %+v, %v", run, err)
	}
	if _, err := parseRun([]string{"--hire", token, "--hire-id", "h1", "--sandbox-off", "--role", "janitor"}); err == nil {
		t.Error("an engine hire's start took --sandbox-off")
	}
}

// sandboxListed is Setup's list (GET /setup/sandbox) as the deck answers it: the row whose id is id, if any, and the
// count of desks without their sandbox.
func sandboxListed(d *Deck, id string) (map[string]any, int) {
	state := d.sandboxState()
	desks, _ := state["desks"].([]map[string]any)
	n, _ := state["unsandboxed"].(int)
	for _, desk := range desks {
		if desk["id"] == id {
			return desk, n
		}
	}
	return nil, n
}

func TestWithTheSandboxOffAPlainHireStartsOutsideItTaggedNoSandbox(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	d.config.HireAgent = "codex"
	d.config.SandboxOff = true
	if status, message := d.Perform("hire", "", url.Values{"desk": {"2"}}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	command, token := hiredWith(t, h.windows[0])
	// Its launcher is told, as it reads it.
	run, err := parseRun(command[2:])
	if err != nil || run.sandboxOff != launcherSandboxes || slices.Contains(command, "--sandbox-off") != launcherSandboxes {
		t.Fatalf("the window runs %q (%+v, %v): want --sandbox-off %v", command, run, err, launcherSandboxes)
	}
	tab := d.launcherConnected("P1", Codex, "/work/api", "", token)
	desk, n := sandboxListed(d, "launcher:P1")
	if !launcherSandboxes {
		if desk != nil || n != 0 {
			t.Errorf("no plain hire has a sandbox here, yet it's listed: %v, %d", desk, n)
		}
		return
	}
	if desk == nil || desk["tag"] != plainHireTag || desk["sandboxed"] != false || desk["restart"] != nil || n != 1 ||
		!strings.Contains(desk["restartWhy"].(string), "fire it and hire it again") || desk["name"] != "Codex in api" {
		t.Fatalf("Setup lists it as %v, %d without the sandbox", desk, n)
	}
	if c := d.sandboxCheck(nil); !strings.HasPrefix(c.Detail, "Off: the desks you start now run without their sandbox") {
		t.Errorf("Start here says %q", c.Detail)
	}
	// The switch turned on again changes no desk that's running: it keeps its tag, and Start here counts it.
	d.config.SandboxOff = false
	if desk, n := sandboxListed(d, "launcher:P1"); desk == nil || n != 1 {
		t.Errorf("after the switch is on again: %v, %d", desk, n)
	}
	// Setup can't restart it there: Start here says what can be done (FXSWITCHPLAINR1, O3).
	if c := d.sandboxCheck(nil); !strings.Contains(c.Detail, "while 1 desk runs without it") ||
		!strings.HasSuffix(c.Detail, "Setup can't restart it in its sandbox: fire it and hire it again, or let it finish.") ||
		strings.Contains(c.Detail, "restart them there") {
		t.Errorf("Start here says %q", c.Detail)
	}
	// With an engine desk out of its sandbox too: that one restarts there, the plain one doesn't.
	if c := d.sandboxCheck([]Hire{{ID: "old000000001", Name: "Ada", State: "working"}}); !strings.Contains(c.Detail, "while 2 desks run without it") ||
		!strings.HasSuffix(c.Detail, "restart them there, or let them finish. Setup can't restart 1 of them: fire it and hire it again.") {
		t.Errorf("Start here says %q", c.Detail)
	}
	// Gone with its window.
	d.launcherGone("P1", tab)
	if desk, n := sandboxListed(d, "launcher:P1"); desk != nil || n != 0 {
		t.Errorf("after its window closed: %v, %d", desk, n)
	}
}

func TestWithTheSandboxOnAPlainHireStartsInItAsBefore(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	d.config.HireAgent = "claude" // the one plain hire that starts in a Mac's sandbox in 1.26.0 (FXPLANTSTERM's notonamac.go)
	if status, message := d.Perform("hire", "", url.Values{"desk": {"2"}}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	command, token := hiredWith(t, h.windows[0])
	if slices.Contains(command, "--sandbox-off") {
		t.Errorf("the window runs %q", command)
	}
	d.launcherConnected("P1", Claude, "/work/api", "", token)
	// Turned off after its hire: it started in its sandbox, so it isn't listed.
	d.config.SandboxOff = true
	if desk, n := sandboxListed(d, "launcher:P1"); desk != nil || n != 0 {
		t.Errorf("a plain hire made with the sandbox on is listed: %v, %d", desk, n)
	}
}

// A launcher whose agent runs without its sandbox says so at each connection (its token spent after the core
// restarts): an office hire's keeps its tag; one run by hand isn't an office hire, and says nothing that counts.
func TestALauncherWithoutItsSandboxKeepsItsTagAfterARestart(t *testing.T) {
	d := newTestDeck(t)
	server := newLocalServer(t, d)
	ctx, hangUp := context.WithCancel(context.Background())
	defer hangUp()
	for _, c := range []struct{ id, hire string }{{"P2", strings.Repeat("CD", 16)}, {"P3", ""}} {
		query := url.Values{"id": {c.id}, "agent": {"codex"}, "folder": {"/work/api"}, "hire": {c.hire}, "sandbox": {"off"}}
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/launcher?"+query.Encode(), nil)
		request.Header.Set(hookKeyHeader, d.access.HookKey)
		response, err := server.Client().Do(request)
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("%s's stream: %v", c.id, err)
		}
		defer response.Body.Close()
		var first launcherCommand
		_ = json.NewDecoder(response.Body).Decode(&first)
	}
	if desk, n := sandboxListed(d, "launcher:P2"); (desk != nil) != launcherSandboxes || n != map[bool]int{true: 1}[launcherSandboxes] {
		t.Errorf("the office's hire, connected again: %v, %d", desk, n)
	}
	if desk, _ := sandboxListed(d, "launcher:P3"); desk != nil {
		t.Errorf("a launcher run by hand is listed: %v", desk)
	}
}

// A General Manager started outside its sandbox is told, as the engine's is (setup.html: "The General Manager is
// told when it starts").
func TestAGMsFirstMessageEndsWithWhatItIsTold(t *testing.T) {
	isolate(t)
	writeFile(t, configFile, `{"roles": [{"id": "gm", "name": "General Manager", "agent": "codex", "instructions": "Run the team."}]}`)
	args, err := roleArguments(gmRole, "/work", hiredIntoGeneralFolder, "", gmSandboxOffLine)
	if err != nil || !strings.HasSuffix(args[len(args)-1], "Run the team. "+gmSandboxOffLine) {
		t.Errorf("the GM starts as %q, %v", args, err)
	}
}
