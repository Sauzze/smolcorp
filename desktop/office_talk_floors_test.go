package desktop

import (
	"path/filepath"
	"testing"
)

// The talk bubble in the office's own window (1.26.0): on every floor, not only the ground floor; an open bubble
// stays over its head on the frames the room draws no bubble there; a tap on a person opens their bubble. The room
// is built from its sources and driven by node on a stand-in page with its network, as the other talk rows. Skipped
// without node.

// A room on the page with its network: the ground floor's sessions, floor 2's, and what the room told the page.
const talkFloorRig = `
dressDoc(win.document, spare);
const res = (status, body) => ({ status, ok: status >= 200 && status < 300, json: async () => body, text: async () => String(body || '') });
const settle = () => new Promise((r) => setTimeout(r, 5));
const card = (s) => ({ state: s.state, asking: s.state === 'asking' ? { since: 5, kind: 'permission', label: 'Bash: make', questions: [], pickable: true } : null,
  size: 1, typing: { ok: true, lines: true, why: '' } });
// A room on the page with its network, the ground floor's sessions and floor 2's, and what it told the page
function floorRig(ground, upstairs, opts = {}) {
  const handlers = {}, calls = [];
  const canvas = Object.assign(new El('canvas'), {
    ownerDocument: win.document, width: 640, height: 360, getContext: context,
    getBoundingClientRect: () => ({ left: 0, top: 0, right: 640, bottom: 360, width: 640, height: 360 }),
    addEventListener: (type, f) => { (handlers[type] = handlers[type] || []).push(f); },
    setPointerCapture() {}, releasePointerCapture() {},
  });
  win.document.body.append(canvas);
  win.document.defaultView = win;
  const all = () => [...ground, ...upstairs];
  const talk = { open: (id) => calls.push(['open', id]), hover: (id) => calls.push(['hover', id]),
    get: async (path, params) => (path === '/talk' ? res(200, card(all().find((s) => s.id === params.session))) : path === '/conversation' ? res(200, { items: [], before: null, size: 1 })
      : res(200, { asking: all().filter((s) => s.state === 'asking').map((s) => ({ session: s.id, ...card(s).asking })) })),
    post: async () => res(204), flash() {}, tick() {} };
  const office = win.AgentOffice.create(canvas, { select: (id) => calls.push(['select', id]), seen: (id) => calls.push(['seen', id]), hint() {}, held() {}, ...(opts.plain ? {} : { talk }) });
  const show = () => office.show({ now: Date.now(), keys: ground.map((s) => ({ session: s })), layout: {}, hiring: { fire: false, hire: false },
    floors: [{ name: '1F', desks: [] }, { name: '2F', preset: '', desks: upstairs }] });
  show();
  office.setVisible(true);
  const step = (n) => {
    for (let i = 0; i < n; i++) {
      T += 100;
      if (win.raf) { const f = win.raf; win.raf = null; f(T); }
      const due = timers; timers = [];
      due.forEach((f) => f());
    }
  };
  const send = (type, x, y, kind = 'mouse') => (handlers[type] || []).forEach((f) => f({ clientX: x, clientY: y, pointerId: 1, pointerType: kind, preventDefault() {} }));
  const r = { office, canvas, calls, step, send, show,
    click(x, y, kind = 'mouse') { send('pointerdown', x, y, kind); send('pointerup', x, y, kind); },
    since(n) { return calls.slice(n); },
    // Where a mouse resting on the room is told of id's bubble, or null: where it rested
    bubble(id) {
      for (let y = 0; y < 360; y += 3) for (let x = 0; x < 640; x += 3) {
        send('pointermove', x, y);
        if (canvas.style.cursor !== 'pointer') continue;
        const n = calls.length;
        step(3);
        if (calls.slice(n).some((c) => c[0] === 'hover' && c[1] === id)) { send('pointerleave'); return [x, y]; }
        send('pointerleave');
      }
      return null;
    },
  };
  step(200);
  return r;
}
const panels = () => win.document.body.find('talk-bubble').map((b) => b.attrs['aria-label']);
const deskList = () => { const d = win.document.body.all().find((c) => c.className.split(' ').includes('talk-desks')); return d ? d.children.map((k) => k.textContent) : []; };
const up = (id, extra = {}) => session(id, { project: id, drivable: true, ...extra });
`

func runTalkFloorRow(t *testing.T, script string) {
	t.Helper()
	path, err := filepath.Abs("../Resources/web/office.js")
	if err != nil {
		t.Fatal(err)
	}
	runNode(t, officeRig+talkDOM+talkFloorRig+script, path)
}

// Floor 2 in view: its waiting desk's bubble hovers and opens on a click, its desks are in the desk list and Enter
// opens the picked one there. On the ride down, nothing opens and floor 2's open bubble closes; the ground floor's
// bubbles are back on arrival. (On 1.26.0 as released, floor 2 has no bubbles and no desk list.)
func TestTheTalkBubblesWorkOnEveryFloor(t *testing.T) {
	runTalkFloorRow(t, `
(async () => {
  // Floor 2 in view: its waiting desk's bubble hovers, opens on a click, its desks are in the desk list, Enter opens the picked one
  const r = floorRig([session('a', { state: 'asking', name: 'Ann' })], [up('u', { state: 'asking' }), up('v', { state: 'idle' })]);
  const atA = r.bubble('a');
  need(atA, 'no talk bubble to hover on the ground floor');
  r.office.ride(2);
  r.step(10);
  await settle(); r.step(3); await settle(); r.step(3);
  need(deskList().join('|') === 'u, waiting for you: Allow Bash: make?|v, idle', 'floor 2\'s desk list: ' + JSON.stringify(deskList()));
  const at = r.bubble('u');
  need(at, 'no talk bubble to hover on floor 2');
  let n = r.calls.length;
  r.click(...at);
  await settle();
  need(r.since(n).some((c) => c[0] === 'open' && c[1] === 'u') && panels().length === 1, 'a click on floor 2\'s bubble opened ' + JSON.stringify(panels()));
  // Back to the ground floor with it open: on the ride, nothing to hover, and the bubble closes
  r.office.ride(1);
  r.step(2);
  need(panels().length === 0, 'floor 2\'s bubble still open on the ride down: ' + JSON.stringify(panels()));
  n = r.calls.length;
  r.click(...atA);
  need(!r.since(n).some((c) => c[0] === 'open'), 'a bubble opened on the ride');
  r.step(10);
  need(r.bubble('a'), 'the ground floor\'s bubble is gone after the ride');
  // Enter on a picked desk upstairs
  r.office.ride(2);
  r.step(10);
  r.office.select('v');
  r.step(2);
  r.canvas.focus();
  need(r.canvas.dispatch('keydown', { key: 'Enter' }).defaultPrevented === true, 'Enter upstairs wasn\'t taken');
  await settle();
  need(panels().length === 1 && !win.document.body.find('talk-bubble')[0].hidden, 'Enter upstairs opened ' + JSON.stringify(panels()));
  r.office.close();
  win.document.body.replaceChildren();
  console.log('floors ok');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}

// An open bubble whose agent the room draws no bubble over (out of order, overloaded: watching the storm) stays shown
// on every frame, not hidden and shown again; its bubble back, still shown; its desk gone, closed. (On 1.26.0 as
// released, it hides on the first such frame.)
func TestAnOpenTalkBubbleStaysOverItsHead(t *testing.T) {
	runTalkFloorRow(t, `
(async () => {
  // An open bubble whose head the room draws no bubble over for a while (out of order, overloaded: watching the storm)
  // stays open and shown, over its head; gone, it closes
  const a = session('a', { name: 'Ann' });
  const r = floorRig([a, session('b', { name: 'Bob' })], [up('u')]);
  const at = r.bubble('a');
  need(at, 'no bubble over a working agent');
  r.click(...at);
  await settle();
  const panel = () => win.document.body.find('talk-bubble')[0] || win.document.body.all().find((c) => c.className.includes('talk-bubble'));
  need(panels().length === 1, 'the click opened ' + JSON.stringify(panels()));
  Object.assign(a, { state: 'error', errorKind: 'overloaded' });
  r.show();
  for (let i = 0; i < 60; i++) {
    r.step(1);
    need(panel() && !panel().hidden, 'the open bubble hid on frame ' + i + ' with no bubble drawn over its head');
  }
  Object.assign(a, { state: 'thinking', errorKind: '' });
  r.show();
  r.step(5);
  need(panel() && !panel().hidden, 'the open bubble hid once its bubble was back');
  // Its desk gone: closed
  r.office.show({ now: Date.now(), keys: [{ session: session('b', { name: 'Bob' }) }], layout: {}, hiring: { fire: false, hire: false },
    floors: [{ name: '1F', desks: [] }, { name: '2F', preset: '', desks: [up('u')] }] });
  r.step(5);
  need(panels().length === 0, 'the open bubble outlived its desk');
  r.office.close();
  win.document.body.replaceChildren();
  console.log('steady ok');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}

// A click on a person opens their desk's bubble and selects nobody: idle away from the desk, working, and upstairs; a
// hire in T3 Code has none. (On 1.26.0 as released, a click on a person selects them, and an idle one has no bubble.)
func TestATapOnAPersonOpensTheirTalkBubble(t *testing.T) {
	runTalkFloorRow(t, `
(async () => {
  // A click on a person opens their desk's bubble, whatever they're doing and wherever they are: an idle one away from
  // the desk, one working, and upstairs too; never selects them; a hire in T3 Code still selects, with no bubble
  const r = floorRig([session('i', { state: 'idle', name: 'Ida' }), session('w', { name: 'Wes' }), session('t', { t3: true })], [up('v', { state: 'idle' })]);
  // The first click on the room that opens id's bubble, and what that click told the page
  const tapOn = (id) => {
    for (let y = 0; y < 360; y += 2) for (let x = 0; x < 640; x += 2) {
      const n = r.calls.length;
      r.click(x, y);
      const told = r.since(n);
      if (told.some((c) => c[0] === 'open' && c[1] === id)) return told;
    }
    return null;
  };
  for (const [id, floor] of [['i', 1], ['w', 1], ['v', 2]]) {
    if (floor === 2) { r.office.ride(2); r.step(10); }
    const told = tapOn(id);
    need(told, 'no click on ' + id + ' (floor ' + floor + ') opened their bubble');
    need(!told.some((c) => c[0] === 'select'), 'the click that opened ' + id + '\'s bubble selected too: ' + JSON.stringify(told));
    await settle();
    r.step(3);
    need(panels().length === 1 && !win.document.body.find('talk-bubble')[0].hidden, id + '\'s bubble isn\'t shown: ' + JSON.stringify(panels()));
  }
  r.office.ride(1);
  r.step(10);
  need(tapOn('t') === null, 'a hire in T3 Code has a talk bubble');
  r.office.close();
  win.document.body.replaceChildren();
  console.log('person ok');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}

// With the talk bubbles, a person held is still their desk: a hold on someone not picked picks them and puts out their
// chips, by mouse or by touch, and held then moved carries them to another desk, as on 1.26.0 as released; a tap opens
// their bubble. Without the bubbles, all as before. (On tree 6fd09473, a hold opened the bubble and nothing came out,
// and a touch drag carried nobody.)
func TestAHoldOnAPersonIsStillTheirDesk(t *testing.T) {
	path, err := filepath.Abs("../Resources/web/office.js")
	if err != nil {
		t.Fatal(err)
	}
	runNode(t, officeRig+`
const dialed = () => [session('a', { dial: true }), session('b', { dial: true }), session('c', { dial: true })];
// b's person and c's, under their bubbles' tips; a is picked
const spots = (() => { const r = rig(true, dialed()); const at = (id) => { const a = r.find(id); return a && [a.css.tip.x, a.css.tip.y + 24]; }; return { b: at('b'), c: at('c') }; })();
need(spots.b && spots.c, 'no bubble over b or c: ' + JSON.stringify(spots));
// What a finger on b tells the page: a tap, a hold, or a hold then a carry to c's desk
const gesture = (talk, kind, hold, drag) => {
  const r = rig(talk, dialed(), true);
  const n = r.calls.length;
  r.send('pointerdown', ...spots.b, kind);
  r.step(hold ? 6 : 1);
  if (drag) for (let i = 1; i <= 8; i++) { r.send('pointermove', spots.b[0] + (spots.c[0] - spots.b[0]) * i / 8, spots.b[1] + (spots.c[1] - spots.b[1]) * i / 8, kind); r.step(1); }
  r.send('pointerup', ...(drag ? spots.c : spots.b), kind);
  r.step(2);
  return r.since(n);
};
for (const talk of [true, false]) for (const kind of ['mouse', 'touch']) {
  const say = (what) => (talk ? 'with' : 'without') + ' the bubbles, a ' + kind + ' ' + what;
  const tap = gesture(talk, kind, false, false);
  need(kinds(tap) === (talk ? 'open' : 'select') && tap.find((c) => c[0] !== 'hover')[1] === 'b', say('tap on b: ') + JSON.stringify(tap));
  const hold = gesture(talk, kind, true, false);
  need(kinds(hold) === 'select,held' && hold[0][1] === 'b', say('hold on b: ') + JSON.stringify(hold));
  const drag = gesture(talk, kind, true, true);
  const swap = drag[drag.length - 1];
  need(kinds(drag) === 'select,held,held,held,swap' && drag[0][1] === 'b' && swap[1] === 1 && swap[2] === 2, say('hold on b carried to c: ') + JSON.stringify(drag));
}
console.log('hold ok');
`, path)
}

// A key armed for one agent (/clear says SURE?) is put away when another is held, as a press on them did on 1.26.0 as
// released: with the talk bubbles or without, by mouse or by touch, /clear tapped once after b is held only arms it
// again, for b. (On tree 930552a9, with the bubbles, that one tap sent /clear to b.)
func TestHoldingAnotherAgentDisarmsTheirKeys(t *testing.T) {
	path, err := filepath.Abs("../Resources/web/office.js")
	if err != nil {
		t.Fatal(err)
	}
	runNode(t, officeRig+`
const dialed = () => [session('a', { dial: true }), session('b', { dial: true }), session('c', { dial: true })];
// The page's keys too: what a key sends is kept
const keyed = (calls) => ({ key: (action) => calls.push(['key', action]) });
const hold = (r, at, kind) => { r.send('pointerdown', ...at, kind); r.step(6); r.send('pointerup', ...at, kind); r.step(2); };
const tap = (r, at, kind) => { const n = r.calls.length; r.send('pointerdown', ...at, kind); r.step(1); r.send('pointerup', ...at, kind); r.step(1); return r.since(n); };
// a's person and b's, under their bubbles' tips; a is picked
const spots = (() => { const r = rig(true, dialed()); const at = (id) => { const a = r.find(id); return a && [a.css.tip.x, a.css.tip.y + 24]; }; return { a: at('a'), b: at('b') }; })();
need(spots.a && spots.b, 'no bubble over a or b: ' + JSON.stringify(spots));
// Where /clear is, with a held: along the keys at the foot of the room, the key whose first tap arms it and second sends it
const clear = (() => {
  const y = 336, over = [];
  const r = rig(true, dialed(), true, keyed);
  hold(r, spots.a, 'mouse');
  for (let x = 0; x < 640; x += 4) { r.send('pointermove', x, y); if (r.canvas.style.cursor === 'pointer') over.push(x); else if (over.length) break; }
  for (const x of over.filter((x, i) => i % 2 === 0)) {
    const r = rig(true, dialed(), true, keyed);
    hold(r, spots.a, 'mouse');
    if (kinds(tap(r, [x, y], 'mouse')) === 'held' && JSON.stringify(tap(r, [x, y], 'mouse')) === '[["key","clear"]]') return [x, y];
  }
  return null;
})();
need(clear, 'no /clear among the keys held out for a');
for (const talk of [true, false]) for (const kind of ['mouse', 'touch']) {
  const say = (what) => (talk ? 'with' : 'without') + ' the bubbles, by ' + kind + ', ' + what;
  const r = rig(talk, dialed(), true, keyed);
  hold(r, spots.a, kind);
  const armed = tap(r, clear, kind);
  need(kinds(armed) === 'held', say('/clear tapped once for a: ') + JSON.stringify(armed));
  const n = r.calls.length;
  hold(r, spots.b, kind);
  need(kinds(r.since(n)) === 'select,held' && r.since(n)[0][1] === 'b', say('b held: ') + JSON.stringify(r.since(n)));
  const once = tap(r, clear, kind);
  need(kinds(once) === 'held', say('/clear tapped once for b, after it was armed for a: ') + JSON.stringify(once));
}
console.log('armed ok');
`, path)
}

// A cloud bot (a Grok Bot) has no terminal and nothing to type into, so no talk bubble, on the ground floor or
// upstairs: none recorded (the desk list leaves it out, Enter on it opens nothing), a mouse resting on it is told
// nothing, and a click or a tap on its person is its desk's, as on 1.26.0 as released: on the ground floor it's
// selected, upstairs its card opens and its news is seen; held upstairs, its card only. A normal agent beside it keeps
// its bubble. (On tree 9d5e50b1, a tap on it opened a bubble, upstairs with its card.)
func TestACloudBotHasNoTalkBubble(t *testing.T) {
	runTalkFloorRow(t, `
(async () => {
  const fresh = () => floorRig([session('a', { name: 'Ann' }), session('g', { state: 'unread', cloud: true, name: 'Gus', project: 'Gus' })],
    [up('u'), up('v', { state: 'idle' }), up('c', { state: 'unread', cloud: true, drivable: false, name: 'Cal', project: 'Cal' })]);
  const done = (r) => { r.office.close(); win.document.body.replaceChildren(); };
  const upTo2 = (r) => { r.office.ride(2); r.step(10); };
  const shown = async (r) => { await settle(); r.step(3); await settle(); r.step(3); };
  // Every click on the room, by mouse or touch, and what each told the page; a card it opened put away again
  const everyClick = (r, kind) => {
    const told = [];
    for (let y = 0; y < 360; y += 3) for (let x = 0; x < 640; x += 3) {
      const n = r.calls.length;
      r.click(x, y, kind);
      told.push({ at: [x, y], calls: r.since(n) });
      r.office.hold(null);
    }
    return told;
  };
  const has = (calls, what, id) => calls.some((c) => c[0] === what && c[1] === id);
  for (const [bot, mate, floor] of [['g', 'a', 1], ['c', 'u', 2]]) {
    const where = ' (floor ' + floor + ')';
    // Hover: the agent's bubble is told, the bot's never
    let r = fresh();
    if (floor === 2) upTo2(r);
    need(r.bubble(mate), 'no bubble to hover over ' + mate + where);
    need(r.bubble(bot) === null, 'a mouse resting on the cloud bot was told of its bubble' + where);
    // No bubble recorded: not in the desk list, and Enter on it, picked, opens nothing
    await shown(r);
    need(deskList().length > 0 && !deskList().some((d) => d.startsWith(floor === 1 ? 'Gus' : 'Cal')), 'the desk list has the cloud bot' + where + ': ' + JSON.stringify(deskList()));
    r.office.select(bot);
    r.step(2);
    r.canvas.focus();
    r.canvas.dispatch('keydown', { key: 'Enter' });
    await settle();
    need(panels().length === 0, 'Enter on the cloud bot opened ' + JSON.stringify(panels()) + where);
    done(r);
    for (const kind of ['mouse', 'touch']) {
      r = fresh();
      if (floor === 2) upTo2(r);
      const told = everyClick(r, kind);
      need(!told.some((t) => has(t.calls, 'open', bot)), 'a ' + kind + ' click opened the cloud bot\'s bubble' + where);
      need(told.some((t) => has(t.calls, 'open', mate)), 'no ' + kind + ' click opened ' + mate + '\'s bubble' + where);
      // Its desk's, as on 1.26.0: selected downstairs; upstairs its card, its news seen, and nothing else
      const own = told.filter((t) => t.calls.some((c) => c[1] === bot));
      need(own.length > 0, 'no ' + kind + ' click reached the cloud bot' + where);
      for (const t of own) {
        const ok = floor === 1 ? JSON.stringify(t.calls) === '[["select","g"]]' : JSON.stringify(t.calls) === '[["seen","c"]]';
        need(ok, 'a ' + kind + ' click on the cloud bot at ' + t.at + where + ': ' + JSON.stringify(t.calls));
      }
      done(r);
      // Held upstairs: its card only
      if (floor === 2) {
        const spot = own[own.length - 1].at;
        r = fresh();
        upTo2(r);
        const n = r.calls.length;
        r.send('pointerdown', ...spot, kind);
        r.step(6);
        r.send('pointerup', ...spot, kind);
        r.step(2);
        need(JSON.stringify(r.since(n)) === '[["seen","c"]]', 'a ' + kind + ' hold on the cloud bot: ' + JSON.stringify(r.since(n)));
        done(r);
      }
    }
  }
  console.log('cloud ok');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}
