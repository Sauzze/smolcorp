package desktop

import (
	"os"
	"strings"
	"testing"
)

// The office's TVs (1.26 TV1): office.js built from its sources and drawn in node on the stand-in for its page
// (officeRig, office_talk_test.go), with every filled rectangle and every line of text recorded, so the TV on air is
// found by its screen's colour and its tag by its words. Skipped without node.

// tvRig is officeRig with four changes: the room's `on.tv` (given only when asked), the room it's furnished as, the
// office it made, and a recording of what it fills, as [colour, x, y, w, h], and writes.
func tvRig(t *testing.T) string {
	t.Helper()
	rig := officeRig
	for _, change := range [][2]string{
		{"return () => {};", "if (k === 'fillRect') return (x, y, w, h) => { rects.push([store.fillStyle, x, y, w, h]); };\n      if (k === 'fillText') return (text) => { texts.push(text); };\n      return () => {};"},
		{"let T = 1000;", "let T = 1000;\nconst rects = [], texts = [];"},
		{"function rig(talk, sessions) {", "function rig(talk, sessions, tv, room) {"},
		{"hint() {}, held() {} };", "hint() {}, held() {} };\n  if (tv) on.tv = (id, url) => calls.push(['tv', id, url]);"},
		{"layout: {},", "layout: room ? { room } : {},"},
		{"const r = {\n    calls, canvas,", "const r = {\n    calls, canvas, office,"},
	} {
		if strings.Count(rig, change[0]) != 1 {
			t.Fatalf("the test rig changed (%q): update this test", change[0])
		}
		rig = strings.Replace(rig, change[0], change[1], 1)
	}
	return rig + `
const css = (hex) => 'rgb(' + [(hex >> 16) & 255, (hex >> 8) & 255, hex & 255].join(',') + ')';
const ON_AIR = css(0x0e1622);
// One frame further on: what it filled, as [colour, x, y, w, h], and what it wrote.
const frame = (room) => {
  rects.length = 0;
  texts.length = 0;
  room.step(1);
  return { rects: rects.map((x) => x.slice()), texts: texts.slice() };
};
// The screens on air in a frame, in art pixels (one is one here): each [x, y, w, h], from the screen's backing alone.
const screens = (f) => f.rects.filter((x) => x[0] === ON_AIR && x[3] >= 20 && x[4] >= 12).map((x) => x.slice(1));
const tags = (f) => f.texts.filter((text) => text.startsWith('LIVE '));
const same = (got, want, what) => { if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(what + ': got ' + JSON.stringify(got) + ', want ' + JSON.stringify(want)); };
// The room is drawn in art pixels and tapped in CSS pixels, two of them to one here.
const middleOf = ([x, y, w, h]) => [(x + w / 2) * 2, (y + h / 2) * 2];
const sessions = () => [session('a'), session('b')];
const WALL_TV = { wall: ['tv', 'whiteboard', 'window', 'pinboard', 'window'] };
const LOUNGE_TV = { corner: 'simpsonstv' };
const BOTH = { ...WALL_TV, ...LOUNGE_TV };
// The lounge TV's screen, where the room as it comes has its corner piece
const LOUNGE_SCREEN = [262, 60, 26, 16];
const tvCalls = (room, n) => room.since(n).filter((c) => c[0] === 'tv');
`
}

// A wall TV and a lounge TV each go on air with the picked desk's preview: LIVE and its port on the screen, the whole
// address on a tag under it; the TV plays as before with no preview, or another desk picked.
func TestTheFloorsTVShowsThePickedDesksPreview(t *testing.T) {
	office := string(officeSource(t))
	for _, want := range []string{"const TV_KINDS = { wall: ['tv'], lounge: ['simpsonstv'] };", "kind: 'tv', part", "setPreview(shown) {", "function drawOnAir(l, t, w, h) {"} {
		if !strings.Contains(office, want) {
			t.Fatalf("office.js lost %q: update this test", want)
		}
	}
	runNode(t, tvRig(t)+`
for (const [what, furnished] of [['the wall TV', WALL_TV], ['the lounge TV', LOUNGE_TV]]) {
  const room = rig(false, sessions(), true, furnished);
  room.step(5);
  same(screens(frame(room)), [], what + ' is on air with no preview');

  room.office.setPreview({ id: 'a', url: 'http://localhost:3000' });
  const f = frame(room), on = screens(f);
  same(on.length, 1, what + ': the screens on air');
  same(tags(f), ['LIVE localhost:3000'], what + ': the tag');
  if (what === 'the lounge TV') same(on[0], LOUNGE_SCREEN, 'the lounge TV on air, over its own screen');
  else if (on[0][1] !== 8 || on[0][2] !== 26 || on[0][3] !== 15) throw new Error('the wall TV on air, not over its own screen: ' + JSON.stringify(on[0]));
  // A path goes on the tag too
  room.office.setPreview({ id: 'a', url: 'http://localhost:5173/app/' });
  same(tags(frame(room)), ['LIVE localhost:5173/app/'], what + ': the tag with a path');

  // Another desk picked: its own preview, not a's
  room.office.select('b');
  const g = frame(room);
  same([screens(g), tags(g)], [[], []], what + ' still shows a\'s preview with b picked');
  room.office.select('a');
  same(screens(frame(room)).length, 1, what + ' with a picked again');
  // Taken away: plays as before
  room.office.setPreview(null);
  const h = frame(room);
  same([screens(h), tags(h)], [[], []], what + ' with the preview taken away');
}

// Only a localhost http address is a preview: anything else the page could pass is none.
const room = rig(false, sessions(), true, WALL_TV);
room.step(5);
for (const bad of [undefined, null, 'http://localhost:3000', { url: 'http://localhost:3000' }, { id: 'a' }, { id: '', url: 'http://localhost:3000' },
  { id: 'a', url: 'http://127.0.0.1:3000' }, { id: 'a', url: 'https://localhost:3000' }, { id: 'a', url: 'http://localhost' },
  { id: 'a', url: 'http://localhost:0' }, { id: 'a', url: 'http://localhost:65536' }, { id: 'a', url: 'http://localhost:3000?x=1' },
  { id: 'a', url: 'http://localhost:3000/#top' }, { id: 'a', url: 'http://localhost:3000/a b' }, { id: 'a', url: 'http://localhost:3000\\\\x' },
  { id: 'a', url: 'http://user@localhost:3000' }, { id: 'a', url: 'http://localhost.evil:3000' }, { id: 'a', url: 'javascript:alert(1)' },
  { id: 'a', url: 'http://localhost:3000/' + 'x'.repeat(2100) }, { id: 'a', url: 42 }, { id: 7, url: 'http://localhost:3000' }]) {
  room.office.setPreview(bad);
  const f = frame(room);
  if (screens(f).length || tags(f).length) throw new Error(JSON.stringify(bad) + ' went on air');
}
room.office.setPreview({ id: 'a', url: 'http://localhost:65535/' });
same(tags(frame(room)), ['LIVE localhost:65535/'], 'the highest port');
`, officePath(t))
}

// One TV per floor: with a wall TV and a lounge TV in the room, only the wall's goes on air, and the lounge's plays on;
// both are something to click, a click is on.tv for the picked desk with the address on air, and a touch is sent on lift.
func TestTheTVsAreClickTargetsAndOneIsOnAir(t *testing.T) {
	runNode(t, tvRig(t)+`
const room = rig(false, sessions(), true, BOTH);
room.step(5);
room.office.setPreview({ id: 'a', url: 'http://localhost:3000' });
const f = frame(room), on = screens(f);
same(on.length, 1, 'screens on air with two TVs in the room');
if (on[0][1] !== 8) throw new Error('the TV on air is not the wall\'s: ' + JSON.stringify(on[0]));
same(tags(f), ['LIVE localhost:3000'], 'one tag');

// The wall TV, clicked: one call, the picked desk and its address; the cursor is a hand over it.
const wall = middleOf(on[0]);
let n = room.calls.length;
room.click(...wall);
same(tvCalls(room, n), [['tv', 'a', 'http://localhost:3000']], 'a click on the wall TV');
if (room.since(n).some((c) => c[0] === 'select')) throw new Error('a click on the TV picked a desk');
room.send('pointermove', ...wall);
same(room.canvas.style.cursor, 'pointer', 'the cursor over the TV');
room.send('pointerleave');

// The lounge TV, off air, is clickable too, for the same desk and address.
n = room.calls.length;
room.click(...middleOf(LOUNGE_SCREEN));
same(tvCalls(room, n), [['tv', 'a', 'http://localhost:3000']], 'a click on the lounge TV');

// A touch: on lift, not on landing, and not if taken away.
n = room.calls.length;
room.send('pointerdown', ...wall, 'touch');
same(tvCalls(room, n), [], 'a touch sent on landing');
room.send('pointerup', ...wall, 'touch');
same(tvCalls(room, n), [['tv', 'a', 'http://localhost:3000']], 'a tap on the TV');
n = room.calls.length;
room.send('pointerdown', ...wall, 'touch');
room.send('pointercancel', ...wall, 'touch');
same(tvCalls(room, n), [], 'a touch taken away still sent');

// No preview: still a TV to click, saying so with a null address; nobody picked, a null desk.
room.office.setPreview(null);
n = room.calls.length;
room.click(...wall);
same(tvCalls(room, n), [['tv', 'a', null]], 'a click on a TV off air');
room.office.select(null);
n = room.calls.length;
room.click(...wall);
same(tvCalls(room, n), [['tv', null, null]], 'a click with nobody picked');

// The lounge TV alone in the room is the floor's, and goes on air.
const lounge = rig(false, sessions(), true, LOUNGE_TV);
lounge.step(5);
lounge.office.setPreview({ id: 'a', url: 'http://localhost:8080' });
same(screens(frame(lounge)), [LOUNGE_SCREEN], 'the lounge TV alone');
n = lounge.calls.length;
lounge.click(...middleOf(LOUNGE_SCREEN));
same(tvCalls(lounge, n), [['tv', 'a', 'http://localhost:8080']], 'a click on the lounge TV alone');

// A room with no TV has nothing on air and no TV to click.
const none = rig(false, sessions(), true);
none.step(5);
none.office.setPreview({ id: 'a', url: 'http://localhost:3000' });
const g = frame(none);
same([screens(g), tags(g)], [[], []], 'a room with no TV');
n = none.calls.length;
none.click(...wall);
none.click(...middleOf(LOUNGE_SCREEN));
same(tvCalls(none, n), [], 'a TV clicked in a room with none');
`, officePath(t))
}

// Upstairs, each floor has its own TV: a floor furnished as the ground floor (no style of its own) has its wall TV on
// air with the picked desk's preview, one screen, and a click on it is on.tv for that desk.
func TestEachFloorUpstairsHasItsTV(t *testing.T) {
	runNode(t, tvRig(t)+`
const room = rig(false, sessions(), true, BOTH);
room.step(5);
room.office.show({ now: Date.now(), keys: sessions().map((s) => ({ session: s })), layout: { room: BOTH }, hiring: { fire: false, hire: false },
  floors: [{ desks: [] }, { name: '2F', preset: '', desks: [session('c', { drivable: true })] }] });
room.office.ride(2);
room.step(80);
room.office.select('c');
room.office.setPreview({ id: 'c', url: 'http://localhost:4000' });
const f = frame(room), on = screens(f);
same(on.length, 1, 'screens on air upstairs');
same(tags(f), ['LIVE localhost:4000'], 'the tag upstairs');
const n = room.calls.length;
room.click(...middleOf(on[0]));
same(tvCalls(room, n), [['tv', 'c', 'http://localhost:4000']], 'a click on the TV upstairs');
// Its preview is the picked desk's: a's, picked, isn't c's
room.office.select('a');
same(screens(frame(room)), [], 'upstairs, c\'s preview with a picked');
`, officePath(t))
}

// The TV is for the computer's own window alone: a room with no on.tv (the phone's, the preview's, the viewer's)
// shows nothing on air and has no TV to click, whatever it's told; the page gives on.tv and asks /desk/preview only
// there; nothing opens the address in a browser (owner, F8 a); and the viewer neither asks nor shows.
func TestTheTVIsNeverOnAPhoneOrTheViewer(t *testing.T) {
	runNode(t, tvRig(t)+`
const asked = rig(false, sessions(), true, BOTH);
asked.step(5);
asked.office.setPreview({ id: 'a', url: 'http://localhost:3000' });
const on = screens(frame(asked));
same(on.length, 1, "the window's room on air");

const without = rig(false, sessions(), false, BOTH);
without.step(5);
without.office.setPreview({ id: 'a', url: 'http://localhost:3000' });
for (let k = 0; k < 5; k++) {
  const f = frame(without);
  if (screens(f).length || tags(f).length) throw new Error('a room with no on.tv went on air');
}
const n = without.calls.length;
without.click(...middleOf(on[0]));
without.click(...middleOf(LOUNGE_SCREEN));
if (without.since(n).some((c) => c[0] === 'tv')) throw new Error('a room with no on.tv has a TV to click');
// Nor is it something to click at all: no hand over it, where the window's room has one
asked.send('pointermove', ...middleOf(on[0]));
same(asked.canvas.style.cursor, 'pointer', "the cursor over the window's TV");
without.send('pointermove', ...middleOf(on[0]));
same(without.canvas.style.cursor, '', 'the cursor over a TV in a room with no on.tv');
`, officePath(t))

	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	deck, viewer := read("../Resources/web/deck.html"), read("../viewer/public/viewer.js")
	for _, want := range []string{
		"const gitHere = whole && !preview && !saver;",
		"...(gitHere ? { tv(id, url) {",
		"if (!gitHere || previewOff",
		"if (gitHere) setInterval(askPreview, PREVIEW_EVERY);",
		"if (gitHere && selected !== previewFor) askPreview();",
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("deck.html lost %q: the TV must be only for the computer's own window", want)
		}
	}
	if strings.Count(deck, "'/desk/preview'") != 1 {
		t.Errorf("deck.html asks for /desk/preview somewhere else than the one place behind gitHere")
	}
	// F8 (a): no "Open in browser". The TV's handler and its words open nothing, and send nothing to the computer.
	start := strings.Index(deck, "...(gitHere ? { tv(id, url) {")
	handler := deck[start : start+strings.Index(deck[start:], "\n")]
	line := deck[strings.Index(deck, "function tvLine(id, url) {"):]
	line = line[:strings.Index(line, "\n  }\n")]
	for _, banned := range []string{"window.open", "location", "send(", "post(", "/open", "href", "innerHTML"} {
		if strings.Contains(handler, banned) || strings.Contains(line, banned) {
			t.Errorf("the TV's click in deck.html uses %q: a desk's page is never opened from Smolcorp", banned)
		}
	}
	for _, banned := range []string{"setPreview", "on.tv", "tv(id", "/desk/preview"} {
		if strings.Contains(viewer, banned) {
			t.Errorf("viewer.js mentions %q: the portable viewer has no TV preview", banned)
		}
	}
}
