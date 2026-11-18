package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The talk bubble's anchors (1.26, P4): talk/anchors.js on its own in node, and the office built from it driven in
// node on a stand-in for its page. Skipped without node.

func anchorsSource(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("../Resources/web/office/talk/anchors.js")
	if err != nil {
		t.Fatal(err)
	}
	return string(src)
}

func runNode(t *testing.T, script string, args ...string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	if out, err := exec.Command(node, append([]string{"-e", script}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

// The bubble's box and tail over a head, padded to at least 12 × 12 art pixels, slid inside the room at its edges;
// the hover's quarter second and its once-a-second asking; the small bubbles' overlap fallback and Needs you's order
// (longest waiting first, unknown last, by id); the page's CSS pixels from the room's art pixels.
func TestTalkAnchorsPlaceTheBubbleAndOrderWhoIsWaiting(t *testing.T) {
	script := anchorsSource(t) + `
const same = (got, want, what) => { if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(what + ': got ' + JSON.stringify(got) + ', want ' + JSON.stringify(want)); };
const room = { width: 320, height: 180 };
const A = TALK_ANCHORS;

// The drawn bubble (11 × 10, tail last) and its pad: at least 12 each way, sideways about the middle, upwards only.
same(A.boxOf(100, 50), { left: 95, right: 106, top: 40, bottom: 50 }, 'the bubble as drawn');
same(A.tipOf(100, 50), { x: 99, y: 49 }, 'the tail');
same(A.padded(A.boxOf(100, 50)), { left: 95, right: 107, top: 38, bottom: 50 }, 'padded');
same(A.padded({ left: 0, right: 30, top: 0, bottom: 20 }), { left: 0, right: 30, top: 0, bottom: 20 }, 'a box already big enough stays');
// Inside the room at its edges, whole, however close the head is.
for (const [x, top] of [[0, 0], [2, 3], [319, 179], [318, 5], [160, 1], [1, 175]]) {
  const box = A.hitBox(x, top, room);
  if (box.left < 1 || box.top < 1 || box.right > 319 || box.bottom > 179) throw new Error('outside the room: ' + JSON.stringify([x, top, box]));
  if (box.right - box.left < 12 || box.bottom - box.top < 12) throw new Error('smaller than 12: ' + JSON.stringify([x, top, box]));
}
same(A.hitBox(0, 0, room), { left: 1, right: 13, top: 1, bottom: 13 }, 'a head in the corner');
same(A.fitInside({ left: 0, right: 400, top: 0, bottom: 10 }, room), { left: 1, right: 401, top: 1, bottom: 11 }, 'too big: its top left wins');

// Hover: a quarter second on a bubble before the tag, off a bubble at once, another bubble starts the wait again.
let s = A.hover(), r = A.hoverStep(s, 'a', 10);
same([r.show, r.hide, Math.round(r.wait * 100)], [null, false, 25], 'just arrived');
s = r.state; r = A.hoverStep(s, 'a', 10.1);
same([r.show, r.hide, Math.round(r.wait * 100)], [null, false, 15], 'a tenth in');
s = r.state; r = A.hoverStep(s, 'a', 10.25);
same([r.show, r.hide, r.wait], ['a', false, null], 'after a quarter second');
s = r.state; r = A.hoverStep(s, 'a', 10.4);
same([r.show, r.hide], [null, false], 'already shown, not shown again');
s = r.state; r = A.hoverStep(s, 'b', 10.5);
same([r.show, r.hide, Math.round(r.wait * 100)], [null, true, 25], 'on to another bubble: the tag goes, the wait starts again');
s = r.state; r = A.hoverStep(s, null, 10.6);
same([r.show, r.hide, r.state], [null, false, A.hover()], 'off before the tag: nothing to hide');
r = A.hoverStep({ id: 'a', since: 1, shown: 'a' }, null, 5);
same([r.show, r.hide], [null, true], 'off a shown tag hides it');
same([A.mayAsk(undefined, 5), A.mayAsk(5, 5.5), A.mayAsk(5, 6)], [true, false, true], 'a desk is asked for once a second');

// Needs you: waiting only, longest first, unknown last, ties by id.
const waiting = [
  { id: 'd', x: 40, top: 50, state: 'asking', since: 30 }, { id: 'w', x: 80, top: 50, state: 'thinking', since: null },
  { id: 'c', x: 120, top: 50, state: 'asking', since: 10 }, { id: 'z', x: 160, top: 50, state: 'asking', since: null },
  { id: 'b', x: 200, top: 50, state: 'asking', since: 10 }, { id: 'y', x: 240, top: 50, state: 'asking', since: null },
];
same(A.needsYou(waiting).map((b) => b.id), ['b', 'c', 'd', 'y', 'z'], 'longest waiting first');

// Small bubbles: 60 × 24 each, tail on the head. Heads 80 apart both fit; closer, the one that waited less (or whose
// start isn't known) falls back to the "?"; one at the room's edge is slid in, its tail still on the head.
const size = { w: 60, h: 24 };
let l = A.layoutSmall(waiting, size, room);
same([l.small.map((p) => p.id), l.marks, l.order], [['b', 'c', 'd'], ['y', 'z'], ['b', 'c', 'd', 'y', 'z']], 'overlap falls back');
const edge = A.layoutSmall([{ id: 'e', x: 3, top: 20, state: 'asking', since: 1 }], size, room).small[0];
same([edge.box.left, edge.tip], [1, { x: 2, y: 19 }], 'at the edge');
same(A.layoutSmall([{ id: 'e', x: 100, top: 90, state: 'thinking', since: null }], size, room).small, [], 'only the waiting ask');

// The page's CSS pixels: art × scale, from the room's corner, over the canvas's pixels to a CSS pixel, from the canvas's corner.
const view = { scale: 4, ox: 20, oy: 10, k: 2, left: 100, top: 50 };
const anchor = A.anchorOf({ id: 'a', x: 100, top: 50, state: 'asking', since: 7 }, room, view);
same([anchor.id, anchor.state, anchor.since], ['a', 'asking', 7], 'the anchor says who');
same(anchor.art, { box: { left: 95, right: 107, top: 38, bottom: 50 }, tip: { x: 99, y: 49 } }, 'art pixels');
same(anchor.css, { box: { left: 100 + (20 + 95 * 4) / 2, right: 100 + (20 + 107 * 4) / 2, top: 50 + (10 + 38 * 4) / 2, bottom: 50 + (10 + 50 * 4) / 2 },
  tip: { x: 100 + (20 + 99 * 4) / 2, y: 50 + (10 + 49 * 4) / 2 } }, 'CSS pixels');

// Recorded once for each person, however often the room is drawn.
const list = [];
A.record(list, { id: 'a', x: 1 }); A.record(list, { id: 'a', x: 2 }); A.record(list, { id: 'b', x: 3 });
same(list.map((b) => [b.id, b.x]), [['a', 1], ['b', 3]], 'one each');
`
	runNode(t, script)
}

// The room, built from its sources and drawn by node on a stand-in for its page: with `on.talk`, the bubble is its
// own target (a click opens it, ahead of the person under it, a mouse resting on it for a quarter second hovers,
// a touch has no hover and opens on lift) and the four choice keys are gone; without, nothing changes: the bubble
// selects, and the keys are there. Staff and a hire in T3 Code have no bubble.
const officeRig = `
const fs = require('fs');
let T = 1000;
global.location = { href: 'http://127.0.0.1/' };
global.localStorage = { getItem: () => null, setItem() {} };
global.performance = { now: () => T };
const context = () => {
  const store = {};
  return new Proxy({}, {
    get(t, k) {
      if (k in store) return store[k];
      if (k === 'measureText') return () => ({ width: 10 });
      if (k === 'getImageData') return () => ({ data: new Uint8ClampedArray(4) });
      if (k === 'createLinearGradient' || k === 'createRadialGradient') return () => ({ addColorStop() {} });
      return () => {};
    },
    set(t, k, v) { store[k] = v; return true; },
  });
};
const spare = () => ({ width: 0, height: 0, style: {}, getContext: context, addEventListener() {} });
const win = {
  devicePixelRatio: 1, document: { hidden: false, addEventListener() {}, removeEventListener() {}, createElement: spare },
  ResizeObserver: class { observe() {} disconnect() {} },
  requestAnimationFrame: (f) => { win.raf = f; return 1; }, cancelAnimationFrame() {},
  setTimeout: (f) => timers.push(f), clearTimeout() {}, addEventListener() {}, removeEventListener() {},
};
let timers = [];
global.window = win;
global.document = win.document;
(0, eval)(fs.readFileSync(process.argv[1], 'utf8'));

const session = (id, extra = {}) => ({ id, name: id, state: 'thinking', effort: '', agent: 'claude', model: '', tool: 'Edit', subagents: [],
  maxPressedAt: null, turnStartedAt: null, dial: false, mic: false, ...extra });

// A room 640 × 360 CSS pixels, its art pixel two of them, with the sessions at its first desks.
function rig(talk, sessions) {
  const handlers = {}, calls = [];
  const canvas = Object.assign(spare(), {
    ownerDocument: win.document,
    getBoundingClientRect: () => ({ left: 0, top: 0, width: 640, height: 360 }),
    addEventListener: (type, f) => { (handlers[type] = handlers[type] || []).push(f); },
    setPointerCapture() {}, releasePointerCapture() {},
  });
  win.document.defaultView = win;
  const on = { select: (id) => calls.push(['select', id]), choose: (action) => calls.push(['choose', action]), hint() {}, held() {} };
  if (talk) on.talk = { open: (id, anchor) => calls.push(['open', id, anchor]), hover: (id, anchor) => calls.push(['hover', id, anchor]) };
  const office = win.AgentOffice.create(canvas, on);
  office.show({ now: Date.now(), keys: sessions.map((s) => ({ session: s })), layout: {}, hiring: { fire: false, hire: false } });
  office.select(sessions[0].id);
  office.setVisible(true);
  canvas.width = 640;
  canvas.height = 360;
  const r = {
    calls, canvas,
    step(n) {
      for (let i = 0; i < n; i++) {
        T += 100;
        if (win.raf) { const f = win.raf; win.raf = null; f(T); }
        const due = timers; timers = [];
        due.forEach((f) => f());
      }
    },
    send(type, x, y, kind = 'mouse') { (handlers[type] || []).forEach((f) => f({ clientX: x, clientY: y, pointerId: 1, pointerType: kind, preventDefault() {} })); },
    click(x, y, kind = 'mouse') { r.send('pointerdown', x, y, kind); r.send('pointerup', x, y, kind); },
    since(n) { return calls.slice(n); },
    // The first anchor the page is told of for a session, by moving a mouse over the room and resting on what it can click
    find(id) {
      for (let y = 0; y < 360; y += 3) for (let x = 0; x < 640; x += 3) {
        r.send('pointermove', x, y);
        if (canvas.style.cursor !== 'pointer') continue;
        r.step(3);
        const at = calls.find((c) => c[0] === 'hover' && c[1] === id);
        if (at) { r.send('pointerleave'); return at[2]; }
        r.send('pointerleave');
      }
      return null;
    },
  };
  r.step(200); // everyone in, and at their desks
  return r;
}
// What the page was told, but for a tag put away (hover(null)), which opening a bubble does first.
const kinds = (calls) => calls.filter((c) => !(c[0] === 'hover' && c[1] === null)).map((c) => c[0]).join(',');
const need = (ok, what) => { if (!ok) throw new Error(what); };
`

func TestTheBubbleIsTheTalkTargetAndTheFourKeysGoWithIt(t *testing.T) {
	office := string(officeSource(t))
	for _, want := range []string{"const TALK_ANCHORS = {", "kind: 'talk', id: bubble.id", "if (framed || on.talk) return null;"} {
		if !strings.Contains(office, want) {
			t.Fatalf("office.js lost %q: update this test", want)
		}
	}
	path, err := filepath.Abs("../Resources/web/office.js")
	if err != nil {
		t.Fatal(err)
	}
	runNode(t, officeRig+`
const sessions = () => [session('a', { state: 'asking' }), session('b'), session('t', { t3: true })];

// With on.talk.
const room = rig(true, sessions());
const anchor = room.find('a');
need(anchor, 'resting a mouse on the asking agent\'s bubble never told the page');
need(anchor.again === true && anchor.id === 'a' && anchor.state === 'asking', 'the first hover is for a, and may ask: ' + JSON.stringify(anchor));
const [box, tip] = [anchor.css.box, anchor.css.tip];
need(box.right - box.left >= 24 && box.bottom - box.top >= 24, 'the bubble is a target of at least 12 art pixels, 24 here: ' + JSON.stringify(box));
need(box.left >= 0 && box.top >= 0 && box.right <= 640 && box.bottom <= 360, 'the box is inside the room');
need(anchor.css.tip.x === anchor.art.tip.x * 2 && anchor.css.tip.y === anchor.art.tip.y * 2, 'CSS pixels are art pixels at this scale');
const middle = [(box.left + box.right) / 2, (box.top + box.bottom) / 2];

// Hover: nothing at first, the tag after a quarter second, gone when the mouse leaves; once a second a desk's card is asked for.
let n = room.calls.length;
room.send('pointermove', 0, 0);
room.step(1);
n = room.calls.length;
room.send('pointermove', ...middle);
need(room.since(n).length === 0, 'a mouse that has only just arrived is already told');
room.step(1);
need(room.since(n).length === 0, 'told after a tenth of a second');
room.step(2);
need(kinds(room.since(n)) === 'hover' && room.since(n)[0][1] === 'a', 'not told after a quarter second: ' + kinds(room.since(n)));
room.send('pointermove', middle[0] + 1, middle[1]);
room.step(3);
need(kinds(room.since(n)) === 'hover', 'moving about on the same bubble tells again');
room.send('pointerleave');
need(room.calls[room.calls.length - 1][0] === 'hover' && room.calls[room.calls.length - 1][1] === null, 'leaving the bubble doesn\'t hide the tag');
room.send('pointermove', 0, 0);
room.step(11);
n = room.calls.length;
room.send('pointermove', ...middle);
room.step(3);
need(room.since(n)[0][2].again === true, 'a desk is asked for again after a second');

// A touch has no hover, and opens on lift, not on landing.
n = room.calls.length;
room.send('pointermove', ...middle, 'touch');
room.step(4);
need(room.since(n).length === 0, 'a touch hovered');
room.send('pointerdown', ...middle, 'touch');
need(room.since(n).length === 0, 'a touch opened on landing');
room.send('pointerup', ...middle, 'touch');
need(kinds(room.since(n)) === 'open', 'a tap on the bubble: ' + kinds(room.since(n)));

// A click on the bubble opens it and picks nobody; one on the person under it, or their desk, picks them, as before.
n = room.calls.length;
room.click(...middle);
const opened = room.since(n);
need(kinds(opened) === 'open', 'a click on the bubble: ' + kinds(opened));
const open = opened.find((c) => c[0] === 'open');
need(open[1] === 'a' && open[2].art.tip.x === anchor.art.tip.x, 'the click opens a with its anchor');
n = room.calls.length;
room.click(tip.x, tip.y + 24);
need(kinds(room.since(n)) === 'select' && room.since(n)[0][1] === 'a', 'a click on the employee picks them: ' + kinds(room.since(n)));

// The four keys are off: nothing in or round the bubble's head sends ▲ ▼ ✓ ✕.
const keys = (r) => {
  const n = r.calls.length, seen = new Set();
  for (let y = tip.y - 110; y <= tip.y + 20; y += 3) for (let x = tip.x - 90; x <= tip.x + 90; x += 3) {
    r.click(x, y);
    r.step(1);
  }
  r.since(n).filter((c) => c[0] === 'choose').forEach((c) => seen.add(c[1]));
  return [...seen].sort().join(',');
};
need(keys(room) === '', 'the choice keys are still out with on.talk: ' + keys(room));

// The staff and a hire in T3 Code have no bubble; another agent has.
need(room.find('b'), 'a working agent has no bubble to hover');
need(room.find('t') === null, 'a hire in T3 Code has a talk bubble');

// Without on.talk, as before: the bubble selects, the four keys are out, nothing is hovered.
const plain = rig(false, sessions());
const before = plain.calls.length;
plain.send('pointermove', ...middle);
plain.step(5);
need(plain.calls.length === before, 'hovering without on.talk told someone');
plain.click(...middle);
need(!plain.calls.some((c) => c[0] === 'open'), 'a bubble opened without on.talk');
need(keys(plain) === 'accept,down,reject,up', 'the four choice keys are not all there without on.talk: ' + keys(plain));
`, path)
}
