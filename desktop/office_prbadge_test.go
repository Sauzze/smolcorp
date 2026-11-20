package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The pull request badge on the desk (1.26 R2): office.js built from its sources and drawn in node on the stand-in
// for its page (officeRig, office_talk_test.go), with the canvas's fillRect recorded so the badge can be found by its
// colours. Skipped without node.

// prRig is officeRig with two changes: the room's `pr` (on.pr given only when asked) and its office, and a
// recording of every filled rectangle, as [colour, x, y, w, h].
func prRig(t *testing.T) string {
	t.Helper()
	rig := officeRig
	for _, change := range [][2]string{
		{"return () => {};", "return k === 'fillRect' ? (x, y, w, h) => { rects.push([store.fillStyle, x, y, w, h]); } : () => {};"},
		{"let T = 1000;", "let T = 1000;\nconst rects = [];"},
		{"function rig(talk, sessions) {", "function rig(talk, sessions, pr) {"},
		{"hint() {}, held() {} };", "hint() {}, held() {} };\n  if (pr) on.pr = (id) => calls.push(['pr', id]);"},
		{"const r = {\n    calls, canvas,", "const r = {\n    calls, canvas, office,"},
	} {
		if strings.Count(rig, change[0]) != 1 {
			t.Fatalf("the test rig changed (%q): update this test", change[0])
		}
		rig = strings.Replace(rig, change[0], change[1], 1)
	}
	return rig + `
const css = (hex) => 'rgb(' + [(hex >> 16) & 255, (hex >> 8) & 255, hex & 255].join(',') + ')';
const TONES = { open: css(0x2f4f8f), approved: css(0x2f7d4a), changes: css(0xb4532a), merged: css(0x8250df), closed: css(0x5b6070) };
// One frame further on: what it filled, as [colour, x, y, w, h].
const frame = (room) => {
  rects.length = 0;
  room.step(1);
  return rects.map((x) => x.slice());
};
// The tag of a colour in a frame: the box round what was filled in it, as [x, y, w, h], or null.
const tagIn = (list, colour) => {
  const found = list.filter((x) => x[0] === colour);
  if (!found.length) return null;
  const left = Math.min(...found.map((x) => x[1])), top = Math.min(...found.map((x) => x[2]));
  return [left, top, Math.max(...found.map((x) => x[1] + x[3])) - left, Math.max(...found.map((x) => x[2] + x[4])) - top];
};
const tagOf = (room, kind) => tagIn(frame(room), TONES[kind]);
// Whether any of the badge's colours is drawn in a frame (as a tag does, with its ink)
const anyTag = (list) => Object.values(TONES).some((colour) => tagIn(list, colour));
const same = (got, want, what) => { if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(what + ': got ' + JSON.stringify(got) + ', want ' + JSON.stringify(want)); };
`
}

const prRoom = `
const sessions = () => [session('a'), session('b'), session('c', { t3: true })];
const shown = (room, pr, kind) => {
  room.office.setPR({ a: pr });
  return tagOf(room, kind);
};
`

func officePath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("../Resources/web/office.js")
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// Each state has its own look, a tag of its own colour with its number, and the badge only comes with what the
// computer gave: an unknown state, a bad number or a wrong kind of answer draws nothing.
func TestThePRBadgeLooksOfItsState(t *testing.T) {
	for _, want := range []string{"function drawGitBadge(desk) {", "kind: 'pr', id", "setPR(found) {"} {
		if !strings.Contains(string(officeSource(t)), want) {
			t.Fatalf("office.js lost %q: update this test", want)
		}
	}
	runNode(t, prRig(t)+prRoom+`
const room = rig(false, sessions(), true);
room.step(5);
if (anyTag(frame(room))) throw new Error('a badge colour is in the room with no pull request');
const states = [
  [{ number: 42, state: 'OPEN', reviewDecision: '' }, 'open'],
  [{ number: 42, state: 'OPEN' }, 'open'],
  [{ number: 42, state: 'OPEN', reviewDecision: 'REVIEW_REQUIRED' }, 'open'],
  [{ number: 42, state: 'OPEN', reviewDecision: 'CHANGES_REQUESTED' }, 'changes'],
  [{ number: 42, state: 'OPEN', reviewDecision: 'APPROVED' }, 'approved'],
  [{ number: 42, state: 'MERGED', reviewDecision: 'APPROVED' }, 'merged'],
  [{ number: 42, state: 'CLOSED', reviewDecision: '' }, 'closed'],
];
for (const [pr, kind] of states) {
  if (!shown(room, pr, kind)) throw new Error(JSON.stringify(pr) + ' draws no ' + kind + ' tag');
  // Only its own colour: none of the other four is drawn with it
  const list = frame(room);
  for (const other of Object.keys(TONES)) if (other !== kind && tagIn(list, TONES[other])) throw new Error(JSON.stringify(pr) + ' is drawn as ' + other + ' too');
}

// Numbers: its mark and "PR" while there is room, then without the mark, then the number alone. (One art pixel is one here.)
const widths = {}, rights = new Set();
for (const number of [7, 42, 123, 1234, 12345]) {
  const tag = shown(room, { number, state: 'OPEN', reviewDecision: '' }, 'open');
  widths[number] = tag[2];
  rights.add(tag[0] + tag[2]);
}
same(widths, { 7: 24, 42: 28, 123: 27, 1234: 22, 12345: 26 }, 'the tag widths');
same(rights.size, 1, 'the tag does not keep its right edge as the number grows');

// What the computer wouldn't send is no badge, not a guess.
for (const bad of [null, undefined, 'PR #42', 42, [], {}, { number: 0, state: 'OPEN' }, { number: -3, state: 'OPEN' }, { number: 1.5, state: 'OPEN' },
  { number: '42', state: 'OPEN' }, { number: 100000001, state: 'OPEN' }, { number: 42, state: 'DRAFT' }, { number: 42, state: 'open' },
  { number: 42, state: 'OPEN', reviewDecision: 'MAYBE' }, { number: 42 }]) {
  room.office.setPR({ a: bad });
  if (anyTag(frame(room))) throw new Error(JSON.stringify(bad) + ' drew a badge');
}
`, officePath(t))
}

// No badge when the computer gives none, and nothing jumps when one comes or goes: the tag is drawn over the desk
// where it is, always in the same place whatever the room does, and the room's own drawing is the same size and place
// with or without it; a desk with no pull request of its own has none.
func TestThePRBadgeComesAndGoesWithoutMovingAnything(t *testing.T) {
	runNode(t, prRig(t)+prRoom+`
const room = rig(false, sessions(), true);
room.step(5);
const lengths = [];
for (let k = 0; k < 8; k++) lengths.push(frame(room).filter((x) => x[0] !== TONES.open).length);
const bare = Math.max(...lengths);

room.office.setPR({ b: { number: 9, state: 'OPEN', reviewDecision: 'APPROVED' } });
const places = new Set();
for (let k = 0; k < 30; k++) {
  const list = frame(room);
  places.add(JSON.stringify(tagIn(list, TONES.approved)));
  if (list.length > bare + 60) throw new Error('the badge drew far more than a tag: ' + (list.length - bare));
}
same(places.size, 1, 'the tag moved about as the room went on: ' + [...places]);
const [, , w, h] = JSON.parse([...places][0]);
if (h !== 7 || w !== 24) throw new Error('the tag is the wrong size: ' + [w, h]);

// Gone as it came: nothing left behind.
room.office.setPR({});
if (anyTag(frame(room))) throw new Error('a badge removed is still drawn');
room.office.setPR({ nobody: { number: 9, state: 'OPEN' }, b: null });
if (anyTag(frame(room))) throw new Error('a pull request for a desk nobody sits at, or none for b, drew a badge');

// Two desks each have their own, in their own place; the hire in T3 Code's has its tag too, at the desk's right.
room.office.setPR({ a: { number: 1, state: 'OPEN' }, c: { number: 2, state: 'MERGED' } });
const list = frame(room), open = tagIn(list, TONES.open), merged = tagIn(list, TONES.merged);
if (!open || !merged) throw new Error('a desk with a pull request has no tag');
if (open[0] === merged[0] && open[1] === merged[1]) throw new Error('two desks, one tag place');
`, officePath(t))
}

// One click or tap on the tag is the page's on.pr for that desk, and nothing else; a tap on the desk beside it is
// still the desk's.
func TestThePRBadgeIsAClickTarget(t *testing.T) {
	runNode(t, prRig(t)+prRoom+`
const room = rig(false, sessions(), true);
room.step(5);
const tag = shown(room, { number: 42, state: 'OPEN', reviewDecision: 'CHANGES_REQUESTED' }, 'changes');
if (!tag) throw new Error('no tag to click');
// The room is drawn in art pixels and tapped in CSS pixels, two of them to one here.
const middle = [(tag[0] + tag[2] / 2) * 2, (tag[1] + tag[3] / 2) * 2];

// A mouse click: at once, one call, and picks nobody.
let n = room.calls.length;
room.click(...middle);
same(room.since(n), [['pr', 'a']], 'a click on the tag');
// The mouse over it is a hand.
room.send('pointermove', ...middle);
same(room.canvas.style.cursor, 'pointer', 'the cursor over the tag');
room.send('pointerleave');

// A touch is sent on lift, not on landing, and not if the finger was taken away.
n = room.calls.length;
room.send('pointerdown', ...middle, 'touch');
same(room.since(n), [], 'a touch opened on landing');
room.send('pointerup', ...middle, 'touch');
same(room.since(n), [['pr', 'a']], 'a tap on the tag');
n = room.calls.length;
room.send('pointerdown', ...middle, 'touch');
room.send('pointercancel', ...middle, 'touch');
same(room.since(n), [], 'a touch taken away still opened it');

// The room well away from the tag is not a pull request.
n = room.calls.length;
room.click(middle[0], middle[1] + 120);
if (room.since(n).some((c) => c[0] === 'pr')) throw new Error('the desk below the tag opened a pull request');

// Once the pull request is gone, nothing is there to click.
room.office.setPR({});
n = room.calls.length;
room.click(...middle);
if (room.since(n).some((c) => c[0] === 'pr')) throw new Error('a click where the tag was opened a pull request');
`, officePath(t))
}

// The badge is for the computer's own window alone: a page that gives no on.pr (the phone's, the preview's, the
// viewer's) draws and taps nothing, whatever it is told; the page gives on.pr only there; and the viewer's page
// neither asks for pull requests nor shows them.
func TestThePRBadgeIsNeverOnAPhoneOrTheViewer(t *testing.T) {
	runNode(t, prRig(t)+prRoom+`
const found = { a: { number: 42, state: 'OPEN', reviewDecision: 'APPROVED' }, b: { number: 7, state: 'MERGED' } };
const asked = rig(false, sessions(), true);
asked.step(5);
asked.office.setPR(found);
const tag = tagOf(asked, 'approved');
if (!tag) throw new Error("the window's room has no tag");

const without = rig(false, sessions(), false);
without.step(5);
without.office.setPR(found);
for (let k = 0; k < 5; k++) if (anyTag(frame(without))) throw new Error('a room with no on.pr drew a badge');
// And a tap where the tag would be is no click on it
const n = without.calls.length;
without.click((tag[0] + tag[2] / 2) * 2, (tag[1] + tag[3] / 2) * 2);
if (without.since(n).some((c) => c[0] === 'pr')) throw new Error('a room with no on.pr opened a pull request');
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
		"...(gitHere ? { pr(id) {",
		"if (!gitHere || gitOff",
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("deck.html lost %q: the badge must be only for the computer's own window", want)
		}
	}
	if strings.Count(deck, "'/desk/git'") != 1 || strings.Count(deck, "'/desk/pr/open'") != 1 {
		t.Errorf("deck.html asks for /desk/git or opens /desk/pr/open somewhere else than the one place behind gitHere")
	}
	for _, banned := range []string{"setPR", "on.pr", "pr(id", "/desk/git", "/desk/pr", "/pull/"} {
		if strings.Contains(viewer, banned) {
			t.Errorf("viewer.js mentions %q: the portable viewer has no pull request badge", banned)
		}
	}
}
