package desktop

import (
	"path/filepath"
	"strings"
	"testing"
)

// A trackpad's or a wheel's scroll up or down over the room rides one floor, one a swipe (1.26.0); and the page
// tells the Mac's office window when Esc has something on it to close. In node, skipped without node.

// floorSwipe is the room's floor-swipe listener and its constants, from office.js, on stand-ins for the room: `wheel`
// sends it a scroll, `rides` is every floor it rode to.
func floorSwipe(t *testing.T, src string) string {
	t.Helper()
	return officePiece(t, src, "  const FLOOR_SWIPE =", ";\n") + `
let styling = false, arranging = null, picker = null, ride = null, floorShown = 2, floorWheel = null, now = 10, listener = null;
const rides = [];
const clock = () => now;
const rideTo = (n) => rides.push(n);
const canvas = { addEventListener(type, f, options) { if (type !== 'wheel' || !options || options.passive !== false) throw new Error('not a wheel listener that may prevent'); listener = f; } };
` + officePiece(t, src, "    canvas.addEventListener('wheel', (e) => {\n      if (e.ctrlKey) return;\n      if (e.defaultPrevented || styling", "\n    }, { passive: false });\n") + `
function wheel(deltaY, { deltaX = 0, deltaMode = 0, taken = false, ctrlKey = false } = {}) {
  const e = { deltaX, deltaY, deltaMode, ctrlKey, defaultPrevented: taken, preventDefault() { this.defaultPrevented = true; this.prevented = true; } };
  listener(e);
  return e;
}
const same = (got, want, what) => { if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(what + ': got ' + JSON.stringify(got) + ', want ' + JSON.stringify(want)); };
const gap = () => { now += FLOOR_SWIPE.ends + 0.05; };
`
}

// One swipe, one floor, as a page scrolls (the owner, 20 Nov: « for scrolling match computer setup »): what scrolls a page
// down (deltaY over 0, whichever way the computer's own setting has the fingers go) the floor below, the other way the
// floor above, however long the swipe and its momentum go on; a gap ends it. Lines and pages count as pixels; sideways
// does nothing; a pinch or Ctrl and the wheel (ctrlKey) is the browser's zoom, left to it.
func TestASwipeOverTheRoomRidesOneFloor(t *testing.T) {
	src := string(officeSource(t))
	runNode(t, floorSwipe(t, src)+`
// Short of the distance: nothing yet; past it, a page's down: the floor below, once, however long the swipe goes on.
for (let i = 0; i < 3; i++) wheel(10);
same(rides, [], 'short of a swipe');
const e = wheel(10);
same(rides, [1], 'a page down');
if (!e.prevented) throw new Error('the swipe was left to the page to scroll');
for (let i = 0; i < 40; i++) { now += 0.016; wheel(30); }
same(rides, [1], 'its momentum rode on');
// A gap ends it: a page's up goes to the floor above.
gap();
wheel(-45);
same(rides, [1, 3], 'a page up');
// Lines and pages, as pixels: one tick of a mouse wheel is a swipe, the same way round.
gap();
wheel(3, { deltaMode: 1 });
gap();
wheel(-0.2, { deltaMode: 2 });
same(rides, [1, 3, 1, 3], 'lines and pages');
// Sideways, or more sideways than not: nothing, and left to the page.
gap();
const side = wheel(10, { deltaX: 100 });
for (let i = 0; i < 5; i++) wheel(20, { deltaX: 80 });
same(rides, [1, 3, 1, 3], 'sideways');
if (side.prevented) throw new Error('a sideways scroll was taken');
// A pinch, or Ctrl and the wheel, either way and however far: the browser's zoom, never a floor.
gap();
for (const d of [60, -60, 3]) {
  const zoom = wheel(d, { ctrlKey: true, deltaMode: d === 3 ? 1 : 0 });
  if (zoom.prevented) throw new Error('a pinch or Ctrl and the wheel was taken from the browser\'s zoom');
}
same(rides, [1, 3, 1, 3], 'a pinch or Ctrl and the wheel');
// It spent nothing: the swipe just after rides.
wheel(60);
same(rides, [1, 3, 1, 3, 1], 'a swipe after the zoom');
`)
	if !strings.Contains(src, "const FLOOR_SWIPE = { wheel: 40, ends: 0.25, down: 1 };") {
		t.Fatal("FLOOR_SWIPE changed: update this test")
	}
	// The one constant turns it the other way.
	flipped := strings.Replace(src, "const FLOOR_SWIPE = { wheel: 40, ends: 0.25, down: 1 };", "const FLOOR_SWIPE = { wheel: 40, ends: 0.25, down: -1 };", 1)
	runNode(t, floorSwipe(t, flipped)+`
wheel(50);
gap();
wheel(-50);
same(rides, [3, 1], 'turned the other way');
`)
}

// Not while styling (the styles' own listener turns their page), over the roles strip (theirs slides them), arranging,
// nor while the elevator rides or its list is open, which spends the swipe till it ends.
func TestTheFloorSwipeLeavesTheStylesTheRolesAndTheElevatorAlone(t *testing.T) {
	src := string(officeSource(t))
	// The styles' and the roles' listeners come first and say they took it.
	style := strings.Index(src, "canvas.addEventListener('wheel', (e) => {\n      if (!styling || !styleKey()) return;\n      e.preventDefault();")
	roles := strings.Index(src, "canvas.addEventListener('wheel', (e) => {\n      if (!rolesAt")
	floor := strings.Index(src, "canvas.addEventListener('wheel', (e) => {\n      if (e.ctrlKey) return;\n      if (e.defaultPrevented || styling")
	if style < 0 || roles < 0 || floor < 0 {
		t.Fatal("the room's wheel listeners changed: update this test")
	}
	if !(style < floor && roles < floor) {
		t.Error("the floor swipe listens before the styles or the roles: it can't tell what they took")
	}
	runNode(t, floorSwipe(t, src)+`
styling = true;
let e = wheel(100);
if (e.prevented) throw new Error('taken while styling');
styling = false;
gap();
wheel(100, { taken: true }); // the roles strip took it
gap();
arranging = { picked: null };
wheel(100);
arranging = null;
same(rides, [], 'styles, roles or arranging');
// The elevator riding: taken (no page scroll) but spent, even once the ride is over, till the swipe ends.
gap();
ride = { from: 1, to: 2, at: 0 };
e = wheel(100);
if (!e.prevented) throw new Error('a swipe during the ride was left to the page');
ride = null;
now += 0.016;
wheel(100);
same(rides, [], 'during the ride');
gap();
wheel(100);
same(rides, [1], 'the next swipe once it got there');
// The floor list open: the same.
gap();
picker = { focus: 2 };
wheel(100);
picker = null;
now += 0.016;
wheel(100);
same(rides, [1], 'with the floor list open');
`)
}

// In the room itself: a page's up rides from the ground floor to the floor above, and after the ride a page's down back. A scroll
// over the talk bubble (its lines scroll there), the Pixel Terminal or the desk list is theirs: they're the page's,
// over the room, and the room never hears it.
func TestAScrollOverTheBubbleIsTheBubblesAndOverTheRoomRides(t *testing.T) {
	path, err := filepath.Abs("../Resources/web/office.js")
	if err != nil {
		t.Fatal(err)
	}
	runNode(t, officeRig+talkDOM+`
dressDoc(win.document, spare);
const floors = [];
localStorage.setItem = (key, value) => { if (key === 'office-floor') floors.push(value); };
const res = (status, body) => ({ status, ok: status >= 200 && status < 300, json: async () => body, text: async () => '' });
const settle = () => new Promise((r) => setTimeout(r, 5));
const handlers = {};
const canvas = Object.assign(new El('canvas'), {
  ownerDocument: win.document, width: 640, height: 360, getContext: context,
  getBoundingClientRect: () => ({ left: 0, top: 0, right: 640, bottom: 360, width: 640, height: 360 }),
  addEventListener: (type, f) => { (handlers[type] = handlers[type] || []).push(f); },
  setPointerCapture() {}, releasePointerCapture() {},
});
win.document.body.append(canvas);
win.document.defaultView = win;
const card = { state: 'idle', asking: null, size: 1, typing: { ok: true, lines: true, why: '' } };
const talk = { open() {}, hover() {}, get: async (path) => (path === '/talk' ? res(200, card) : path === '/conversation' ? res(200, { items: [], before: null, size: 1 }) : res(200, { asking: [] })),
  post: async () => res(204), flash() {}, tick() {} };
const office = win.AgentOffice.create(canvas, { select() {}, hint() {}, held() {}, talk });
office.show({ now: Date.now(), keys: [{ session: session('i', { state: 'idle', name: 'Ida' }) }], layout: {}, hiring: { fire: false, hire: false } });
office.setVisible(true);
const step = (n) => {
  for (let i = 0; i < n; i++) {
    T += 100;
    if (win.raf) { const f = win.raf; win.raf = null; f(T); }
    const due = timers; timers = [];
    due.forEach((f) => f());
  }
};
// A scroll on the room: each of its wheel listeners in turn, as the page sends it.
const scroll = (deltaY) => {
  const e = { deltaX: 0, deltaY, deltaMode: 0, clientX: 300, clientY: 150, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } };
  (handlers.wheel || []).forEach((f) => f(e));
  return e;
};
(async () => {
  step(200);
  // The bubble open over Ida's desk, then a scroll over its lines: the bubble's own, the floor as it was.
  office.select('i');
  canvas.focus();
  canvas.dispatch('keydown', { key: 'Enter' });
  await settle();
  step(3);
  const bubble = win.document.body.find('talk-bubble')[0];
  need(bubble, 'no bubble open');
  const lines = bubble.querySelector('.talk-lines') || bubble;
  const over = lines.dispatch('wheel', { deltaX: 0, deltaY: 120, deltaMode: 0 });
  need(!over.defaultPrevented, 'a scroll over the bubble was taken from it');
  const list = win.document.body.find('talk-desks')[0];
  if (list) need(!list.dispatch('wheel', { deltaX: 0, deltaY: 120, deltaMode: 0 }).defaultPrevented, 'a scroll over the desk list was taken from it');
  step(3);
  need(floors.length === 0, 'a scroll over the bubble or the desk list changed floor: ' + JSON.stringify(floors));
  // On the room: a page's up (deltaY under 0) is a floor up, once.
  canvas.dispatch('keydown', { key: 'Escape' });
  step(2);
  need(scroll(-60).defaultPrevented, 'the room left its swipe to the page');
  scroll(-60);
  need(JSON.stringify(floors) === '["2"]', 'up: ' + JSON.stringify(floors));
  // The Pixel Terminal over the room: a scroll there is its own too.
  T += 1000;
  step(10);
  scroll(60);
  need(JSON.stringify(floors) === '["2","1"]', 'down after the ride: ' + JSON.stringify(floors));
  T += 1000;
  step(10);
  need(office.expand('i'), 'no Pixel Terminal');
  step(3);
  // Its picture of the room is a canvas of the rig's own, not an element: walked past.
  const walk = (el) => el.children.filter((c) => c instanceof El).flatMap((c) => [c, ...walk(c)]);
  const terminal = walk(win.document.body).find((el) => el.className.split(' ').includes('pt') && el.getClientRects().length);
  need(terminal, 'the Pixel Terminal is not shown');
  need(!terminal.dispatch('wheel', { deltaX: 0, deltaY: 120, deltaMode: 0 }).defaultPrevented, 'a scroll over the Pixel Terminal was taken from it');
  need(floors.length === 2, 'a scroll over the Pixel Terminal changed floor: ' + JSON.stringify(floors));
  office.close();
})().catch((e) => { console.error(e); process.exit(1); });
`, path)
}

// The page says when Esc has something to close on it, to the Mac's window only (officeEscape, OfficeWindow.swift):
// a talk bubble open, the Pixel Terminal, a field with the focus; and when it's closed again, or the room is put
// away. Said once each time it changes; a page without the window's handler says nothing.
func TestThePageTellsTheMacsWindowWhenEscIsItsOwn(t *testing.T) {
	src := string(officeSource(t))
	tell := officePiece(t, src, "    function tellEscape(open) {", "\n    }\n")
	for _, want := range []string{"picker", "maxing", "keysFor", "held", "talkPanels.shown()", "tvBig.shown()", "INPUT|TEXTAREA|SELECT", "isContentEditable"} {
		if !strings.Contains(tell, want) {
			t.Errorf("Esc's word to the window leaves out %q:\n%s", want, tell)
		}
	}
	path, err := filepath.Abs("../Resources/web/office.js")
	if err != nil {
		t.Fatal(err)
	}
	runNode(t, officeRig+talkDOM+`
dressDoc(win.document, spare);
const said = [];
const res = (status, body) => ({ status, ok: status >= 200 && status < 300, json: async () => body, text: async () => '' });
const settle = () => new Promise((r) => setTimeout(r, 5));
const handlers = {};
const canvas = Object.assign(new El('canvas'), {
  ownerDocument: win.document, width: 640, height: 360, getContext: context,
  getBoundingClientRect: () => ({ left: 0, top: 0, right: 640, bottom: 360, width: 640, height: 360 }),
  addEventListener: (type, f) => { (handlers[type] = handlers[type] || []).push(f); },
  setPointerCapture() {}, releasePointerCapture() {},
});
win.document.body.append(canvas);
win.document.defaultView = win;
const card = { state: 'idle', asking: null, size: 1, typing: { ok: true, lines: true, why: '' } };
const talk = { open() {}, hover() {}, get: async (path) => (path === '/talk' ? res(200, card) : path === '/conversation' ? res(200, { items: [], before: null, size: 1 }) : res(200, { asking: [] })),
  post: async () => res(204), flash() {}, tick() {} };
const office = win.AgentOffice.create(canvas, { select() {}, hint() {}, held() {}, talk });
office.show({ now: Date.now(), keys: [{ session: session('i', { state: 'idle', name: 'Ida' }) }], layout: {}, hiring: { fire: false, hire: false } });
const step = (n) => {
  for (let i = 0; i < n; i++) {
    T += 100;
    if (win.raf) { const f = win.raf; win.raf = null; f(T); }
    const due = timers; timers = [];
    due.forEach((f) => f());
  }
};
(async () => {
  // A page without the window's handler (a phone, a browser): nothing to tell, and nothing goes wrong.
  office.setVisible(true);
  step(50);
  win.webkit = { messageHandlers: { officeEscape: { postMessage: (open) => said.push(open) } } };
  step(5);
  need(said.length === 0, 'nothing open, and the page said ' + JSON.stringify(said));
  // A bubble open: said once, however many frames.
  office.select('i');
  canvas.focus();
  canvas.dispatch('keydown', { key: 'Enter' });
  await settle();
  step(5);
  need(JSON.stringify(said) === '[true]', 'a bubble open: ' + JSON.stringify(said));
  // Esc closes it, handled (so WebKit doesn't beep), and the window is told Esc is its own again.
  need(canvas.dispatch('keydown', { key: 'Escape' }).defaultPrevented, 'the Esc that closed the bubble was left unhandled');
  step(5);
  need(JSON.stringify(said) === '[true,false]', 'the bubble closed: ' + JSON.stringify(said));
  // A field with the focus.
  const field = new El('input');
  win.document.body.append(field);
  field.focus();
  step(2);
  canvas.focus();
  step(2);
  need(JSON.stringify(said) === '[true,false,true,false]', 'a field: ' + JSON.stringify(said));
  // Nothing open: Esc is left alone (the Mac's window hides on it before the page hears it).
  need(!canvas.dispatch('keydown', { key: 'Escape' }).defaultPrevented, 'an Esc with nothing open was taken');
  // The Pixel Terminal, and its own Esc.
  need(office.expand('i'), 'no Pixel Terminal');
  step(3);
  need(said[said.length - 1] === true, 'the Pixel Terminal open: ' + JSON.stringify(said));
  need(canvas.dispatch('keydown', { key: 'Escape' }).defaultPrevented, 'the Esc that closed the Pixel Terminal was left unhandled');
  step(3);
  need(said[said.length - 1] === false, 'the Pixel Terminal closed: ' + JSON.stringify(said));
  // Open again, then the room put away: no frames to say it, so it's said at once.
  canvas.dispatch('keydown', { key: 'Enter' });
  await settle();
  step(3);
  need(said[said.length - 1] === true, 'the bubble open again: ' + JSON.stringify(said));
  office.setVisible(false);
  need(said[said.length - 1] === false, 'the room put away with a bubble open: ' + JSON.stringify(said));
  office.close();
})().catch((e) => { console.error(e); process.exit(1); });
`, path)
}

// Over the roles strip under a held desk, a scroll is the strip's, even when every role fits and there's nothing to
// slide: never a floor's ride, which would put the held desk down (RT126OFFICEKEYS N2). Off the strip, it rides.
func TestAScrollOverTheRolesStripNeverRidesEvenWhenTheyFit(t *testing.T) {
	src := string(officeSource(t))
	listeners := officePiece(t, src, "    canvas.addEventListener('wheel', (e) => {\n      if (!rolesAt", "FLOOR_SWIPE.down);\n    }, { passive: false });\n")
	runNode(t, officePiece(t, src, "  const FLOOR_SWIPE =", ";\n")+`
let styling = false, arranging = null, picker = null, ride = null, floorShown = 2, floorWheel = null, now = 10, roleOffset = 0;
let rolesAt = { roles: [], box: { left: 100, top: 300, right: 400, bottom: 320 }, most: 0, arrows: [] };
const rides = [], wheels = [];
const clock = () => now;
const rideTo = (n) => rides.push(n);
const roleRow = () => true;
const roomAt = (x, y) => ({ x, y, scale: 1, ratio: 1 });
const canvas = { addEventListener(type, f) { if (type === 'wheel') wheels.push(f); } };
`+listeners+`
const same = (got, want, what) => { if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(what + ': got ' + JSON.stringify(got) + ', want ' + JSON.stringify(want)); };
function wheel(clientX, clientY, deltaY, deltaX = 0) {
  const e = { clientX, clientY, deltaX, deltaY, deltaMode: 0, ctrlKey: false, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } };
  wheels.forEach((f) => f(e));
  now += 1;
  return e;
}
if (wheels.length !== 2) throw new Error('the roles and the floor listen to the wheel: ' + wheels.length);
// Every role fits: over the strip, up or down or sideways, taken, nothing slides, no floor.
for (const [dy, dx] of [[100, 0], [-100, 0], [0, 100]]) {
  const e = wheel(250, 310, dy, dx);
  if (!e.defaultPrevented) throw new Error('a scroll over the strip was left to the page');
}
same(rides, [], 'a scroll over a strip that fits');
same(roleOffset, 0, 'a strip that fits slid');
// Off the strip: it rides.
wheel(250, 100, 100);
same(rides, [1], 'off the strip');
// Too many roles to fit: over the strip it slides them, still no floor.
rolesAt.most = 80;
wheel(250, 310, 50);
same(roleOffset, 50, 'the roles slid');
same(rides, [1], 'a scroll over a strip that slides');
// No desk held, no strip: it rides.
rolesAt = null;
wheel(250, 310, -100);
same(rides, [1, 3], 'with no strip');
`)
}

// Esc that puts away the max-effort question, or a held desk's keys and chips, is handled (preventDefault), so the
// Mac's WebKit doesn't take it as a key nobody heard and beep (RT126OFFICEKEYS O2). Any other key, or nothing to put
// away, is left as it was.
func TestTheEscThatPutsAwayTheQuestionOrTheKeysIsHandled(t *testing.T) {
	src := string(officeSource(t))
	branch := officePiece(t, src, "        // Esc that closes something is handled", "          e.preventDefault();\n        }\n")
	runNode(t, `
let maxing = null, keysFor = null, held = null;
function typed(e) {
`+branch+`
}
const press = (key) => { const e = { key, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } }; typed(e); return e; };
const need = (ok, what) => { if (!ok) throw new Error(what); };
maxing = { id: 'i' };
need(press('Escape').defaultPrevented && maxing === null, 'the max-effort question');
keysFor = 'i';
need(press('Escape').defaultPrevented && keysFor === null, 'the keys');
held = 'i';
need(press('Escape').defaultPrevented && held === null, 'a held desk');
need(!press('Escape').defaultPrevented, 'nothing to put away, and the Esc was taken');
held = 'i';
need(!press('Enter').defaultPrevented && held === 'i', 'another key');
`)
}
