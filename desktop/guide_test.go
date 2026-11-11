package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The first-run guide shows on a new computer only: a config from before it, of someone who had set up
// (setup opened, a hires folder chosen, or a GM hired, by hand too), counts as done, and what's saved is kept.
func TestTheGuideIsForAFirstRunOnly(t *testing.T) {
	isolate(t)
	if loadConfig().GuideDone {
		t.Fatal("no config yet: the guide counts as done")
	}
	for _, file := range []string{`{"hasCompletedSetup": true}`, `{"hireFolder": "/work"}`, `{"managerEnabled": true}`, `{"gmByHand": true}`} {
		writeFile(t, configFile, file)
		if !loadConfig().GuideDone {
			t.Errorf("%s, from before the guide: it would be shown", file)
		}
	}
	writeFile(t, configFile, `{"hasCompletedSetup": false, "hireFolder": "", "gmByHand": false}`)
	if loadConfig().GuideDone {
		t.Error("a config of someone who never set up: the guide counts as done")
	}
	// Setup opened (hasCompletedSetup) during the guide doesn't end it: what's saved is.
	writeFile(t, configFile, `{"hasCompletedSetup": true, "hireFolder": "/work", "guideDone": false}`)
	config := loadConfig()
	if config.GuideDone {
		t.Fatal("guideDone false loads as done")
	}
	config.save()
	if loadConfig().GuideDone {
		t.Error("saved and loaded again, the guide counts as done")
	}
}

// POST /setup/guide keeps whether the guide is over, and setup's state says it.
func TestTheGuideIsKeptAndSetupsStateSaysIt(t *testing.T) {
	d := newTestDeck(t)
	d.config.GuideDone = false
	done := func() bool {
		t.Helper()
		var state struct{ Guide struct{ Done *bool } }
		got := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer})
		if json.Unmarshal(got.Body.Bytes(), &state) != nil || state.Guide.Done == nil {
			t.Fatalf("setup's state has no guide.done: %s", got.Body.String())
		}
		return *state.Guide.Done
	}
	set := func(value string) {
		t.Helper()
		got := serve(t, d, call{method: "POST", target: "/setup/guide?done=" + value, host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}})
		if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"guide"`) {
			t.Fatalf("done=%s: %d %s", value, got.Code, got.Body.String())
		}
	}
	if done() {
		t.Fatal("a first run: setup says the guide is over")
	}
	set("1")
	if !done() || !d.config.GuideDone || !loadConfig().GuideDone {
		t.Fatalf("finished: setup says %v, the config %v, the saved config %v", done(), d.config.GuideDone, loadConfig().GuideDone)
	}
	set("0")
	if done() || loadConfig().GuideDone {
		t.Fatal("shown again: the guide still counts as done")
	}
}

// The tray's Show Me Around Again turns the guide on again, saved, and opens setup on #guide.
func TestShowMeAroundAgainTurnsTheGuideOnAndOpensSetupOnIt(t *testing.T) {
	d := newTestDeck(t)
	d.config.GuideDone = true
	d.config.save()
	opened := 0
	was := openGuidePage
	openGuidePage = func(*Deck) error { opened++; return nil }
	t.Cleanup(func() { openGuidePage = was })
	if err := d.showGuideAgain(); err != nil {
		t.Fatal(err)
	}
	if d.config.GuideDone || loadConfig().GuideDone || opened != 1 {
		t.Fatalf("after Show Me Around Again: done %v, saved %v, setup opened %d times", d.config.GuideDone, loadConfig().GuideDone, opened)
	}
	if !strings.HasSuffix(setupAddress("o=token", "guide"), "#o=token&guide") {
		t.Error("setup's address doesn't end in &guide, which the page starts the guide on")
	}
	app, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	tray := string(app)
	if !strings.Contains(tray, `systray.AddMenuItem("Show Me Around Again"`) || !strings.Contains(tray, "case <-t.guide.ClickedCh:\n\t\t\t_ = t.deck.showGuideAgain()") {
		t.Error("the tray has no Show Me Around Again that calls showGuideAgain")
	}
}

// guide.js is served as office.js is, with no key: it holds nothing of yours.
func TestTheGuideScriptIsServed(t *testing.T) {
	d := newTestDeck(t)
	got := serve(t, d, call{method: "GET", target: "/guide.js", host: "localhost:7710", from: thisComputer})
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "SmolcorpGuide") {
		t.Fatalf("/guide.js: %d", got.Code)
	}
}

// Setup loads the guide, has its steps on the 1.24.3 contract's ids, and starts it again from #guide and its button.
func TestSetupHasTheGuidesStepsOnTheContractsIds(t *testing.T) {
	data, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	setup := string(data)
	for _, want := range []string{
		`<script src="/guide.js"></script>`,
		`target: '#first-folder-use', area: '#first-folder'`, `target: '#first-gm-checks'`, `target: '#start-gm'`, `target: '#hs-hire', optional: true`,
		`lost: 'Open the office to see your General Manager.'`, `checks.dataset.ok`,
		`patience: () => (gmStarting() ? 300000 : 0)`, `'Your General Manager is starting: if its terminal asks to trust the folder, answer Yes.'`,
		`doneTarget: () => firstGmDesk() || $('start-office')`, `if (!res || !res.ok) guideTour = false;`,
		`$('first-gm-next')`, `$('start-office')`,
		`id="guide-again"`, `'/setup/guide?done=1'`, `'/setup/guide?done=0'`, `['pair', 'unpair', 'viewer', 'guide']`, `location.hash !== '#guide'`,
	} {
		if !strings.Contains(setup, want) {
			t.Errorf("setup.html has no %s", want)
		}
	}
	steps := setup[strings.Index(setup, "steps: ["):strings.Index(setup, "onFinish:")]
	order := []string{"id: 'folder'", "id: 'checks'", "id: 'hire'", "id: 'sheet'", "id: 'desk'"}
	for i := 1; i < len(order); i++ {
		if strings.Index(steps, order[i-1]) < 0 || strings.Index(steps, order[i-1]) > strings.Index(steps, order[i]) {
			t.Errorf("the steps aren't in the contract's order: %s before %s", order[i-1], order[i])
		}
	}
	// Only the last step counts a click: the hire is done while it's really under way, not when pressed.
	if strings.Count(steps, "ackOnClick") != 1 || strings.Index(steps, "ackOnClick") < strings.Index(steps, "id: 'desk'") {
		t.Error("a step other than the GM at its desk counts a click as done")
	}
	if !strings.Contains(steps, "done: () => gmSessions().length > 0 || gmHiring || gmSheetOpen() }") {
		t.Error("the hire step isn't done by a hire under way")
	}
}

// The bubble keeps off the page's title and Start here's header, and off what the checks and hire steps talk
// about; it has no count of its own (Start here's "N steps left" is the one); off Easy setup it's a pill
// whose Skip says it can be undone.
func TestTheGuidesBubbleKeepsOffWhatItTalksAbout(t *testing.T) {
	data, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	setup := string(data)
	guide := setup[strings.Index(setup, "SmolcorpGuide.create({"):strings.Index(setup, "onFinish:")]
	for _, want := range []string{
		`keep: '.sidebar > .brand, .sidebar > .search, .topbar, .pane.current > .pane-head, #easy-attention, #start-here > .card-head'`,
		`keep: '#start-checks, #start-gm-account, #start-gm, #first-gm-recheck, #first-gm-why'`,
		`keep: '#first-gm-checks, #start-checks, #start-gm-account, #first-gm-recheck, #first-gm-why, #start-said'`,
		`awayText: 'Next step in Easy setup'`, `skipSays: 'Skip: Show me around again brings it back'`,
		`where: () => document.querySelector('.pane.current')?.dataset.pane || ''`,
		`hideSays: "Hide for now: it's back in the next section you open"`,
	} {
		if !strings.Contains(guide, want) {
			t.Errorf("the guide has no %s", want)
		}
	}
	if strings.Contains(guide, "total:") || regexp.MustCompile(`\bn: \d`).MatchString(guide) {
		t.Error("the guide still counts its steps beside Start here's own count")
	}
}

// Fresh eyes, round 2: the bubble keeps off any text near what's lit and goes out of the way once its
// control is pressed; the pill's × only hides it, and on a phone Show me and × are apart, 44 px each; the
// welcome's last step names the General Manager while none is there, and leads to Hire.
func TestTheGuideKeepsOffTextAndTheWelcomeNamesWhatsMissing(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	script, setup := read("../Resources/web/guide.js"), read("../Resources/web/setup.html")
	for _, want := range []string{
		"function lines(litEl, view) {",
		"{ text, aside: pressed === shown, chip }",
		"pressed = shown;\n        position();",
		"pillHiddenIn = where();\n        return hide();",
		".smol-guide.pill .smol-guide-skip { order: 2; align-self: center; min-width: 44px; min-height: 44px;",
		".smol-guide.pill .smol-guide-go { order: 1; min-height: 44px;",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("guide.js has no %q", want)
		}
	}
	for _, want := range []string{
		`<h2 id="welcome-end-title">All set</h2>`,
		`<p id="welcome-ready">Your office is ready: open it on this computer, or on a paired phone.</p>`,
		`<button class="btn primary" id="welcome-gm">Hire your General Manager</button>`,
		"'Next: hire your General Manager' : 'All set'",
		"go('easy', $('start-gm'));",
	} {
		if !strings.Contains(setup, want) {
			t.Errorf("setup.html has no %q", want)
		}
	}
	if strings.Contains(setup, "Your office is ready: open it on this computer, or on a paired phone. Want desks") {
		t.Error("the welcome's last step still says the office is ready whatever is missing")
	}
}

// gmHiring, the hire under way the guide's hire step goes by: set once Hire your General Manager has
// sent the hire, and cleared by hireEnded on every way out but a GM seated, so the arrow comes back.
func TestTheGuidesHireStepFollowsTheHireNotTheClick(t *testing.T) {
	data, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	setup := string(data)
	// The button runs hireGm (one hire at a time); the hire itself is in there.
	start := strings.Index(setup, "async function hireGm() {")
	if start < 0 {
		t.Fatal("setup.html has no hireGm for Hire your General Manager")
	}
	hire := setup[start : start+strings.Index(setup[start:], "\n  }\n")]
	set, sent := strings.Index(hire, "gmHiring = true;"), strings.Index(hire, "await fetch(`/setup/hire/gm")
	if set < 0 || sent < set {
		t.Errorf("gmHiring isn't set before the hire is sent:\n%s", hire)
	}
	// Every way out after the hire is sent ends the hire first (no answer, a refusal, a GM that never
	// started, five minutes with no GM at a desk), but one: the GM seated ("Hired:").
	if !strings.Contains(setup, "function hireEnded() {\n    gmHiring = false;") {
		t.Error("setup.html has no hireEnded that clears gmHiring")
	}
	after := hire[sent:]
	outs, prev := 0, 0
	for i := 0; ; {
		at := strings.Index(after[i:], "return ")
		if at < 0 {
			break
		}
		at += i
		if !strings.Contains(after[prev:at], "Hired:") {
			outs++
			if !strings.Contains(after[prev:at], "hireEnded();") {
				t.Errorf("a way out of the hire doesn't call hireEnded first: %s", strings.SplitN(after[at:], "\n", 2)[0])
			}
		}
		prev = at + len("return ")
		i = prev
	}
	if outs < 4 {
		t.Errorf("expected the no-answer, refused, didn't-start and timed-out ways out after the hire is sent, found %d", outs)
	}
	if strings.Index(hire, "Choose the Claude account") > set {
		t.Error("gmHiring is set before the account is checked")
	}
	sheet := setup[strings.Index(setup, "$('hs-hire').onclick = async () => {"):]
	sheet = sheet[:strings.Index(sheet, "closeHireSheet();")]
	if !strings.Contains(sheet, "if (gmHire) gmHiring = true;") || !strings.Contains(sheet, "if (gmHire) hireEnded();") ||
		!strings.Contains(sheet, "if (gmHire) { const mine = ++gmHireSerial; setTimeout(() => { if (mine === gmHireSerial) hireEnded(); }, 300000); }") {
		t.Error("the hire sheet doesn't say when a GM's hire is under way")
	}
}

// Fresh eyes, round 3: the bubble never sits on the logo or Search (the Mac's side column, a phone's bar);
// a step whose control is off the window's edge shows a chip toward it, not a parked bubble nor an arrow on
// a row at the edge; on a phone the bubble is against its arrow, and the pill shrinks to a chip once you
// scroll. Where it all lands is checked in node (Tests/fixtures/guide/engine.test.js).
func TestTheGuideStaysByItsControlOrSaysContinueBelow(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	script, setup := read("../Resources/web/guide.js"), read("../Resources/web/setup.html")
	for _, want := range []string{
		"textContent: '↓ Continue below'",
		"more.textContent = at.point === 'up' ? '↑ Continue above' : '↓ Continue below';",
		"if (cy < 0 || cy > view.height) return chip(cy < 0);",
		"if (how.aside || view.width > PHONE) return result('edge'",
		".smol-guide.pill.small .smol-guide-bubble",
		"go.textContent = pillSmall ? 'Next step →' : 'Show me';",
		"win[how]('scroll', scrolled, { capture: true, passive: true });",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("guide.js has no %q", want)
		}
	}
	if !strings.Contains(setup, "keep: '.sidebar > .brand, .sidebar > .search, .topbar, ") {
		t.Error("the guide doesn't keep off the logo and Search")
	}
	if !strings.Contains(setup, `<div class="brand">`) || !strings.Contains(setup, `<div class="topbar">`) || !strings.Contains(setup, `<nav class="sidebar"`) {
		t.Error("the logo and Search the guide keeps off aren't where it looks for them")
	}
}

// The engine itself, run in node on a small stand-in page (Tests/fixtures/guide/engine.test.js): what a
// click counts for, Esc and Skip, a dialog over the page, the keyboard moved only on a new step, a tour,
// the last step docked when its control never comes, stop() leaving nothing running; and setup's welcome
// waiting for the guide on a first run. Without node the test is skipped.
func TestTheGuidesEngineInNode(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: guide.js can't be run")
	}
	var paths []string
	for _, path := range []string{"../Tests/fixtures/guide/engine.test.js", "../Resources/web/guide.js", "../Resources/web/setup.html"} {
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, abs)
	}
	out, err := exec.Command(node, paths...).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("%v\n%s", err, out)
	}
}

// The guide's own rules, run in node on guide.js as it is: which step shows, where the arrow and the bubble go,
// and the arrow in three colours. Without node the test is skipped.
func TestTheGuidePicksTheNextStepAndPlacesItsArrowInNode(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: guide.js can't be run")
	}
	guide, err := filepath.Abs("../Resources/web/guide.js")
	if err != nil {
		t.Fatal(err)
	}
	script := `
const g = require(` + strconvQuote(guide) + `);
const assert = (ok, what) => { if (!ok) throw new Error(what); };
const step = (o) => Object.assign({ done: false, shown: false, away: false, missingFor: 0 }, o);

// The first step not done is shown, whatever is done after it.
assert(g.pick([step({ done: true }), step({ shown: true }), step({ shown: true })]).index === 1, 'the step after the one done');
assert(g.pick([step({ shown: true }), step({ done: true }), step({ shown: true })]).index === 0, 'a later step done never skips an earlier one');
assert(g.pick([step({ done: true }), step({ done: true })]).finished, 'every step done is finished');
assert(g.pick([step({ shown: true }), step({ missingFor: g.GRACE }), step({ done: true })]).finished, 'the last step done is finished');
// A missing control is waited for, then looked past; one elsewhere on the page is shown docked.
assert(g.pick([step({ missingFor: 100 }), step({ shown: true })]).index === -1, 'a step missing a moment is waited for');
assert(g.pick([step({ missingFor: g.GRACE }), step({ shown: true })]).index === 1, 'a step missing for long is looked past');
assert(g.pick([step({ missingFor: g.GRACE }), step({ missingFor: 0 }), step({ shown: true })]).index === -1, 'the next one is waited for too');
let r = g.pick([step({ missingFor: g.GRACE }), step({ missingFor: g.GRACE })]);
assert(r.index === -1 && !r.finished, 'nothing there: nothing shown, never finished');
assert(g.pick([step({ away: true }), step({ shown: true })]).index === 0, 'a step elsewhere on the page is still the one');
// The last step's control never comes, every step before it done: shown docked, not run unseen.
r = g.pick([step({ done: true }), step({ done: true }), step({ missingFor: g.GRACE })]);
assert(r.index === 2 && r.lost && !r.finished, 'the last step lost: docked');
assert(!g.pick([step({ missingFor: g.GRACE }), step({ missingFor: g.GRACE })]).lost, 'not lost while a step before it is not done');
assert(g.pick([step({ done: true }), step({ optional: true }), step({ missingFor: g.GRACE })]).lost, 'an optional step before it is no hold-up');
// An optional step is never waited for; a step's own patience is.
assert(g.pick([step({ optional: true }), step({ shown: true })]).index === 1, 'an optional step not there is passed at once');
assert(g.pick([step({ missingFor: g.GRACE, patience: 15000 }), step({ shown: true })]).index === -1, 'a step given more patience is waited for longer');
// A step that says why it waits is shown, docked, while it does; past its patience, as any other.
r = g.pick([step({ done: true }), step({ done: true }), step({ missingFor: g.GRACE, patience: 300000, waiting: true })]);
assert(r.index === 2 && r.waiting && !r.lost && !r.finished, 'the last step waited for, saying why');
r = g.pick([step({ done: true }), step({ done: true }), step({ missingFor: 300000, patience: 300000, waiting: true })]);
assert(r.index === 2 && r.lost && !r.waiting, 'past its patience: lost');

const arrow = { width: 33, height: 33 };
const inside = (p, bubble, view) => p.bubble.x >= 0 && p.bubble.y >= 0 && p.bubble.x + bubble.width <= view.width && p.bubble.y + bubble.height <= view.height
  && p.arrow.x >= 0 && p.arrow.y >= 0 && p.arrow.x + arrow.width <= view.width && p.arrow.y + arrow.height <= view.height;
const on = (p, bubble, b) => p.bubble.x < b.left + b.width && p.bubble.x + bubble.width > b.left && p.bubble.y < b.top + b.height && p.bubble.y + bubble.height > b.top;
for (const view of [{ width: 1100, height: 780 }, { width: 390, height: 844 }]) {
  const bubble = { width: Math.min(326, view.width - 24), height: 90 };
  // A small control with room beside it (not on a phone): the bubble to its right, the arrow's tip on it,
  // pointing left. On a phone, below it.
  let box = { left: 40, top: 400, width: 20, height: 28 };
  let p = g.place(box, view, bubble, arrow);
  if (view.width > 500) {
    assert(p.side === 'right' && p.point === 'left' && p.arrow.x === box.left + box.width + 4, view.width + ': beside a small control');
    assert(Math.abs(p.arrow.y + arrow.height / 2 - (box.top + box.height / 2)) <= 1, view.width + ': the arrow level with it');
    assert(p.bubble.x === p.arrow.x + arrow.width, view.width + ': the bubble on the arrow');
  } else {
    assert(p.side === 'below' && p.point === 'up' && p.arrow.y === box.top + box.height + 4, view.width + ': no room beside: below');
  }
  assert(inside(p, bubble, view) && !on(p, bubble, box), view.width + ': the bubble off the control, inside the window');
  // A block as wide as the window: below it, the arrow's tip on the control's foot, pointing up.
  const lit = { left: 12, top: 200, width: view.width - 24, height: 150 };
  box = { left: 40, top: 300, width: 120, height: 28 };
  p = g.place(box, view, bubble, arrow, lit);
  assert(p.side === 'below' && p.point === 'up' && p.arrow.y === box.top + box.height + 4, view.width + ': below a wide block');
  assert(Math.abs(p.arrow.x + arrow.width / 2 - (box.left + box.width / 2)) <= 1 && !on(p, bubble, lit) && inside(p, bubble, view), view.width + ': centred on the control, off the block');
  // Something to keep in sight just under the block, beside the arrow: the bubble moves down past it.
  const kept = { left: 200, top: 360, width: 160, height: 30 };
  p = g.place(box, view, bubble, arrow, lit, [kept]);
  assert(p.side === 'below' && !on(p, bubble, kept) && !on(p, bubble, lit), view.width + ': off what is kept');
  // No room beside or below: a corner of the window, never on the block, the arrow still on the control;
  // on a phone, right against the arrow (over it here), still off the block.
  const low = { left: 12, top: view.height - 200, width: view.width - 24, height: 180 };
  box = { left: 40, top: view.height - 120, width: 120, height: 28 };
  p = g.place(box, view, bubble, arrow, low);
  assert(p.side === (view.width > 760 ? 'edge' : 'attached') && !on(p, bubble, low) && inside(p, bubble, view), view.width + ': no room: a corner off the block, or against the arrow');
  if (p.side === 'attached') assert(p.point === 'down' && p.bubble.y + bubble.height <= p.arrow.y, view.width + ': the bubble over its arrow');
  // Out of sight, below or above, or cut by the window's foot with no room: a chip at that edge, toward it.
  const chip = { width: 150, height: 32 };
  p = g.place({ left: 100, top: 3000, width: 100, height: 30 }, view, bubble, arrow, undefined, [], { chip });
  assert(p.side === 'chip' && p.down && p.bubble.y === view.height - 12 - chip.height, view.width + ': a control below the window');
  p = g.place({ left: 100, top: -900, width: 100, height: 30 }, view, bubble, arrow, undefined, [], { chip });
  assert(p.side === 'chip' && p.point === 'up' && p.bubble.y === 12, view.width + ': a control above the window');
  const cut = { left: 12, top: view.height - 150, width: view.width - 24, height: 280 };
  p = g.place(cut, view, bubble, arrow, cut, [], { chip });
  assert(p.side === 'chip' && p.down, view.width + ': a list cut by the foot of the window: ' + JSON.stringify(p));
}

// The arrow: a square grid of three colours, as the office's pixel UI.
const used = new Set(g.ARROW.join('').replace(/\./g, ''));
assert(g.ARROW.every((row) => row.length === g.ARROW[0].length), 'every row the same width');
assert(used.size >= 2 && used.size <= 3 && [...used].every((c) => /^#[0-9a-f]{6}$/.test(g.COLOURS[c])), 'two or three colours, each one set');
`
	file := filepath.Join(t.TempDir(), "guide.js")
	if err := os.WriteFile(file, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, file).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

// strconvQuote is a path as a JavaScript string.
func strconvQuote(path string) string {
	quoted, _ := json.Marshal(path)
	return string(quoted)
}
