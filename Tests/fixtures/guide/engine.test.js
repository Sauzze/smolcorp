/*
 * The first-run guide's engine (Resources/web/guide.js) run in node, on a tiny stand-in for the page:
 * just enough of the DOM for the guide (elements with a box, a few selectors, clicks and keys, timers that
 * only run when told). It checks what a click counts for, Esc and Skip, a dialog over the page, the
 * keyboard moved only when the step changes, a tour of steps already done, the last step waiting while the
 * GM starts and docked when its control never comes, and stop() leaving no timer or listener behind. Where
 * the bubble and the arrow go, on the fresh-eyes shots' Start here (1100×780 and 390×844), and the pill off
 * Easy setup; round 2's shots with their text, the bubble out of the way once its control is pressed, and the
 * pill's × that only hides it; round 3's shots: never on the logo or Search, a "Continue below" chip for a
 * control cut by the window's edge, the bubble against its arrow on a phone, and the phone's pill shrinking
 * once you scroll. Then setup.html's welcome, which waits for the guide on a first run, and whose last step
 * names the General Manager while none is there.
 *
 * Run by desktop/guide_test.go: node engine.test.js <guide.js> <setup.html>. It throws on the first failure.
 */
'use strict';
const fs = require('fs');
const guidePath = process.argv[2], setupPath = process.argv[3];
const g = require(guidePath);

let failures = 0;
const assert = (ok, what) => { if (!ok) { failures++; console.error('FAIL: ' + what); } };

/* ------------------------------------------------------------------ the stand-in page */

function page() {
  let clock = 1000;
  const timers = new Map();
  let nextTimer = 1;
  const listeners = { doc: [], win: [] };

  const parse = (one) => {
    let rest = one.trim();
    const nots = [], attrs = [];
    rest = rest.replace(/:not\(\[([^\]=]+)(?:="([^"]*)")?\]\)/g, (_, n, v) => { nots.push([n, v]); return ''; });
    rest = rest.replace(/\[([^\]=]+)(?:="([^"]*)")?\]/g, (_, n, v) => { attrs.push([n, v]); return ''; });
    let id = null;
    rest = rest.replace(/#([\w-]+)/, (_, found) => { id = found; return ''; });
    return { tag: rest || null, id, attrs, nots };
  };
  const attr = (el, name) => {
    if (['hidden', 'open', 'disabled'].includes(name)) return el[name] ? '' : null;
    if (name === 'href' || name === 'tabindex' || name === 'contenteditable' || name === 'aria-describedby' || name === 'role') return el.attrs[name] ?? null;
    return el.attrs[name] ?? null;
  };
  const matches = (el, selector) => selector.split(',').some((one) => {
    const { tag, id, attrs, nots } = parse(one);
    const has = ([n, v]) => { const a = attr(el, n); return a != null && (v === undefined || a === v); };
    return (!tag || el.tagName === tag.toUpperCase()) && (!id || el.id === id) && attrs.every(has) && !nots.some(has);
  });

  class El {
    constructor(tag) {
      Object.assign(this, { tagName: tag.toUpperCase(), children: [], parentNode: null, attrs: {}, style: {}, dataset: {},
        hidden: false, id: '', className: '', box: null, focused: 0, scrolled: 0, _text: '' });
      this.classList = { toggle: (name, on) => { const set = new Set(this.className.split(' ').filter(Boolean)); (on ?? !set.has(name)) ? set.add(name) : set.delete(name); this.className = [...set].join(' '); },
        contains: (name) => this.className.split(' ').includes(name) };
    }
    append(...kids) { for (const kid of kids) { if (kid.parentNode) kid.remove(); kid.parentNode = this; this.children.push(kid); } }
    remove() { if (this.parentNode) { this.parentNode.children = this.parentNode.children.filter((c) => c !== this); this.parentNode = null; } }
    set textContent(value) { this._text = String(value); this.children = []; }
    get textContent() { return this._text + this.children.map((c) => c.textContent).join(''); }
    setAttribute(n, v) { this.attrs[n] = String(v); }
    getAttribute(n) { return this.attrs[n] ?? null; }
    removeAttribute(n) { delete this.attrs[n]; }
    get isConnected() { let el = this; while (el.parentNode) el = el.parentNode; return el === doc.documentElement; }
    get shownHere() { let el = this; while (el) { if (el.hidden) return false; el = el.parentNode; } return this.isConnected; }
    getClientRects() { return this.shownHere && this.box ? [this.box] : []; }
    getBoundingClientRect() { return this.box || { left: 0, top: 0, width: 0, height: 0 }; }
    get offsetWidth() { return 260; }
    get offsetHeight() { return 80; }
    matches(selector) { return matches(this, selector); }
    closest(selector) { let el = this; while (el && el.tagName) { if (el.matches(selector)) return el; el = el.parentNode; } return null; }
    contains(other) { while (other) { if (other === this) return true; other = other.parentNode; } return false; }
    all() { return this.children.flatMap((c) => [c, ...c.all()]); }
    querySelectorAll(selector) { return this.all().filter((el) => el.matches(selector)); }
    querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
    focus() { this.focused++; doc.activeElement = this; }
    scrollIntoView() { this.scrolled++; }
  }

  const doc = {
    documentElement: new El('html'),
    createElement: (tag) => new El(tag),
    createElementNS: (_, tag) => new El(tag),
    getElementById: (id) => doc.documentElement.all().find((el) => el.id === id) || null,
    querySelector: (s) => doc.documentElement.querySelector(s),
    querySelectorAll: (s) => doc.documentElement.querySelectorAll(s),
    addEventListener: (type, fn, capture) => listeners.doc.push({ type, fn, capture }),
    removeEventListener: (type, fn) => { listeners.doc = listeners.doc.filter((l) => l.type !== type || l.fn !== fn); },
    activeElement: null,
  };
  doc.head = new El('head');
  doc.body = new El('body');
  doc.documentElement.append(doc.head, doc.body);
  doc.documentElement.clientWidth = 1100;
  doc.activeElement = doc.body;

  const win = {
    innerWidth: 1100, innerHeight: 780,
    getComputedStyle: () => ({ visibility: 'visible' }),
    matchMedia: () => ({ matches: false }),
    setInterval: (fn, ms) => { const id = nextTimer++; timers.set(id, { fn, ms, every: true, at: clock + ms }); return id; },
    setTimeout: (fn, ms) => { const id = nextTimer++; timers.set(id, { fn, ms, every: false, at: clock + ms }); return id; },
    clearInterval: (id) => timers.delete(id),
    clearTimeout: (id) => timers.delete(id),
    requestAnimationFrame: (fn) => { const id = nextTimer++; timers.set(id, { fn, ms: 16, every: false, at: clock + 16, frame: true }); return id; },
    cancelAnimationFrame: (id) => timers.delete(id),
    addEventListener: (type, fn) => listeners.win.push({ type, fn }),
    removeEventListener: (type, fn) => { listeners.win = listeners.win.filter((l) => l.type !== type || l.fn !== fn); },
  };
  doc.defaultView = win;

  const now = () => clock;
  /** Time passes: every timer due runs, in order. */
  function wait(ms) {
    const until = clock + ms;
    for (;;) {
      const due = [...timers.entries()].filter(([, t]) => t.at <= until).sort((a, b) => a[1].at - b[1].at)[0];
      if (!due) break;
      const [id, t] = due;
      clock = t.at;
      if (t.every) t.at += t.ms; else timers.delete(id);
      t.fn();
    }
    clock = until;
  }
  function fire(type, target, extra = {}) {
    const event = { type, target, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, ...extra };
    for (const l of listeners.doc.filter((l) => l.type === type && l.capture)) l.fn(event);
    if (type === 'click' && target.onclick) target.onclick(event);
    for (const l of listeners.doc.filter((l) => l.type === type && !l.capture)) l.fn(event);
    return event;
  }
  /** An element in the page, with a box (shown) or none. */
  function add(tag, id, box, parent = doc.body) {
    const el = new El(tag);
    el.id = id;
    el.box = box ? { left: box[0], top: box[1], width: box[2], height: box[3] } : null;
    parent.append(el);
    return el;
  }
  return { doc, win, timers, listeners, now, wait, fire, add };
}

Date.now = () => current.now();
let current = null;
const layerOf = (p) => p.doc.documentElement.all().find((el) => el.className.split(' ').includes('smol-guide'));
const textOf = (p) => { const t = p.doc.getElementById('smol-guide-text'); return t ? t.textContent : ''; };
const countOf = (p) => { const l = layerOf(p); const c = l && l.all().find((el) => el.className === 'smol-guide-count'); return c ? c.textContent : ''; };
const goOf = (p) => layerOf(p).all().find((el) => el.className === 'smol-guide-go');
const skipOf = (p) => layerOf(p).all().find((el) => el.className === 'smol-guide-skip');
const showing = (p) => { const l = layerOf(p); return !!l && !l.hidden; };

const STARTING = 'Your General Manager is starting: if its terminal asks to trust the folder, answer Yes.';
const docked = (p) => showing(p) && layerOf(p).classList.contains('docked');

/** Start here, as the contract has it: the folder, the checks, the hire, the GM at its desk. */
function startHere() {
  const p = page();
  current = p;
  const s = { folder: false, green: false, gm: false, hiring: false, sheetOpen: false };
  p.add('button', 'use', [40, 300, 120, 28]);
  p.add('div', 'checks', [40, 360, 600, 60]);
  p.add('button', 'hire', [40, 440, 180, 28]);
  const desk = p.add('div', 'desk', [40, 500, 600, 120]);
  desk.hidden = true;
  p.add('button', 'office', [60, 560, 120, 28], desk);
  const said = [];
  const steps = [
    { id: 'folder', target: '#use', doneTarget: '#checks', say: 'Pick where your hires work', press: 'Use this folder', doneSay: 'Folder set', done: () => s.folder },
    { id: 'checks', target: '#checks', say: 'Make these checks green', doneSay: 'Green', done: () => s.green },
    { id: 'hire', target: '#hire', say: 'Now your first hire', press: 'Hire your General Manager', doneSay: 'Hired', done: () => s.gm || s.hiring || s.sheetOpen },
    { id: 'sheet', target: '#sheet-hire', optional: true, say: 'Filled in', press: 'Hire', done: () => s.gm || s.hiring },
    // As setup.html's: while the GM is starting (hired, none seated), waited for five minutes, saying why.
    { id: 'desk', target: '#desk', ackOnClick: true, button: 'Done', say: 'At its desk', press: 'Open the office', lost: 'Open the office to see your General Manager.',
      patience: () => (s.hiring && !s.gm ? 300000 : 0), wait: () => (s.hiring && !s.gm ? STARTING : '') },
  ];
  const guide = g.create({ document: p.doc, steps, onFinish: (reason) => said.push(reason) });
  return { p, s, guide, said, el: (id) => p.doc.getElementById(id) };
}

/* ------------------------------------------------------------------ the tests */

// What a click counts for: on the hire step, nothing (the page says when a hire is under way); on the
// last step, whose click is all it asks, it's done. A refused hire brings the arrow back. While the GM is
// starting, the last step waits, docked, saying what its terminal asks, and never offers Done.
{
  const { p, s, guide, said, el } = startHere();
  s.folder = s.green = true;
  guide.start();
  assert(showing(p) && textOf(p).startsWith('Now your first hire'), 'the hire step first: ' + textOf(p));
  p.fire('click', el('hire'));
  p.wait(500);
  assert(textOf(p).startsWith('Now your first hire'), 'a click on Hire alone is not a hire: ' + textOf(p));
  s.hiring = true;
  p.wait(450);
  assert(docked(p) && textOf(p) === STARTING && countOf(p) === '', 'the GM starting: docked, saying what its terminal asks, no count: ' + textOf(p));
  assert(goOf(p).hidden && !skipOf(p).hidden, 'nothing to press but Skip while it starts');
  assert(p.doc.body.children.some((c) => c.className === 'smol-guide-room'), 'room at the foot of the page while docked');
  s.hiring = false;
  p.wait(450);
  assert(showing(p) && !docked(p) && textOf(p).startsWith('Now your first hire'), 'the hire refused: the arrow is back on Hire');
  // Hired again: the wait starts again from this hire, however long the page has been open.
  p.wait(300000);
  s.hiring = true;
  p.wait(g.GRACE + 500);
  assert(docked(p) && textOf(p) === STARTING, 'well past GRACE, still starting: still waiting, not lost: ' + textOf(p));
  p.wait(240000);
  assert(docked(p) && textOf(p) === STARTING, 'four minutes on: still waiting');
  p.wait(60000);
  assert(docked(p) && textOf(p) === 'Open the office to see your General Manager.', 'five minutes and no GM: docked, saying where to look: ' + textOf(p));
  assert(goOf(p).textContent === 'Done' && !goOf(p).hidden, 'with Done');
  s.gm = true;
  el('desk').hidden = false;
  p.wait(450);
  assert(showing(p) && !layerOf(p).classList.contains('docked') && textOf(p).startsWith('At its desk'), 'the GM sat down: the arrow on its card');
  assert(!p.doc.body.children.some((c) => c.className === 'smol-guide-room'), 'no room kept once not docked');
  p.fire('click', el('office'));
  p.wait(100);
  assert(said.join() === 'done' && !guide.running, 'a click in the last step finishes it: ' + said.join());
}

// A GM seated whose card never shows: docked as lost after GRACE, as before.
{
  const { p, s, guide, said } = startHere();
  s.folder = s.green = s.hiring = true;
  guide.start();
  assert(docked(p) && textOf(p) === STARTING, 'starting: waiting');
  s.gm = true;
  p.wait(450);
  assert(!showing(p) || textOf(p) !== STARTING, 'seated: no longer says it is starting');
  p.wait(g.GRACE + 500);
  assert(docked(p) && textOf(p) === 'Open the office to see your General Manager.', 'seated, no card: docked after GRACE: ' + textOf(p));
  goOf(p).onclick();
  assert(said.join() === 'done', 'Done ends it');
}

// Esc puts the guide away, not over; Esc in a field is the field's; Skip is over.
{
  const { p, guide, said, el } = startHere();
  guide.start();
  const field = p.add('input', 'name', [10, 10, 100, 20]);
  p.fire('keydown', field, { key: 'Escape' });
  assert(guide.running && said.length === 0, 'Esc in a field is left to it');
  const esc = p.fire('keydown', p.doc.body, { key: 'Escape' });
  assert(!guide.running && said.join() === 'hidden' && esc.defaultPrevented, 'Esc: put away, said as hidden, not skipped: ' + said.join());
  assert(!layerOf(p), 'nothing of it left in the page');
  guide.start();
  skipOf(p).onclick();
  assert(said.join() === 'hidden,skipped', 'Skip: over');
  assert(el('use').getAttribute('aria-describedby') === null, 'its words taken off the control');
}

// A dialog over the page: the guide waits under it, and is back where it was when it closes, without
// taking the keyboard or the scroll again. A step whose control is in the dialog goes inside it.
{
  const { p, s, guide, el } = startHere();
  s.folder = s.green = true;
  guide.start({ focus: true });
  assert(el('hire').focused === 1, 'asked for (Show me around again): the keyboard on the step');
  const dialog = p.add('dialog', 'sheet', [100, 100, 400, 300]);
  dialog.open = true;
  p.wait(450);
  assert(!showing(p), 'a dialog over the page: the guide waits');
  dialog.open = false;
  p.wait(450);
  assert(showing(p) && el('hire').focused === 1 && el('hire').scrolled === 0, 'back after the dialog: the keyboard left where it was');
  // The sheet open on the GM: its Hire is the step, inside the dialog, and the guide goes in with it.
  p.add('button', 'sheet-hire', [300, 350, 80, 28], dialog);
  dialog.open = true;
  s.sheetOpen = true;
  p.wait(450);
  assert(showing(p) && layerOf(p).parentNode === dialog && textOf(p).startsWith('Filled in'), 'a step in an open dialog: the guide inside it');
  const esc = p.fire('keydown', p.doc.body, { key: 'Escape' });
  assert(guide.running && !esc.defaultPrevented, "Esc while the step is in a dialog is the dialog's");
  guide.stop();
}

// A plain open leaves the keyboard alone; a step change brings it.
{
  const { p, s, guide, el } = startHere();
  guide.start();
  assert(showing(p) && el('use').focused === 0 && el('use').scrolled === 0, 'opened, not asked for: the keyboard left alone');
  s.folder = true;
  p.wait(450);
  assert(textOf(p).startsWith('Make these checks green'), 'the folder set: the checks next');
  s.green = true;
  p.wait(450);
  assert(el('hire').focused === 1, 'a new step: the keyboard on it');
  guide.stop();
}

// A tour (Show me around again): every step from the first, done or not; done ones say so, with Next.
{
  const { p, s, guide, said, el } = startHere();
  s.folder = s.green = s.gm = true;
  guide.start({ tour: true, focus: true });
  assert(showing(p) && textOf(p) === 'Folder set' && countOf(p) === 'Done', 'a tour starts at step 1, done: ' + textOf(p));
  assert(el('checks').focused === 0 && goOf(p).textContent === 'Next', 'done: with Next, pointing where it was done');
  goOf(p).onclick();
  assert(textOf(p) === 'Green', 'Next: step 2, done: ' + textOf(p));
  goOf(p).onclick();
  assert(textOf(p) === 'Hired', 'Next: step 3, done: ' + textOf(p));
  goOf(p).onclick();
  // The sheet is optional and not there; the desk card isn't shown: docked, after a wait.
  assert(!showing(p), 'the desk card not there: waited for');
  p.wait(g.GRACE + 500);
  assert(showing(p) && textOf(p) === 'Open the office to see your General Manager.', 'then docked at the end of the tour: ' + textOf(p));
  goOf(p).onclick();
  assert(said.join() === 'done' && !guide.running, 'Done ends the tour');
}

// A tour moves on from a step that's done while it's shown, as a first run does.
{
  const { p, s, guide } = startHere();
  guide.start({ tour: true });
  assert(textOf(p).startsWith('Pick where your hires work'), 'not done: as on a first run');
  s.folder = true;
  p.wait(450);
  assert(textOf(p).startsWith('Make these checks green'), 'done while shown: on to the next, no Next needed: ' + textOf(p));
  guide.stop();
}

// stop() leaves nothing running: no timer, no frame, no listener, nothing in the page.
{
  const { p, guide } = startHere();
  guide.start();
  for (const l of [...p.listeners.win]) if (l.type === 'scroll') l.fn();
  assert(p.timers.size >= 1, 'running: its timer and a frame');
  guide.stop();
  assert(p.timers.size === 0, 'stopped: no timer or frame left: ' + p.timers.size);
  assert(p.listeners.doc.length === 0 && p.listeners.win.length === 0, 'stopped: no listener left');
  assert(!layerOf(p) && !guide.running, 'stopped: nothing in the page');
  p.wait(5000);
  assert(!layerOf(p), 'and nothing comes back');
}

// Placement, on the fresh-eyes shots' Start here (shots 01, 01c, 02, 05): the bubble beside or below what's
// lit, never on it, on the page's title and Start here's header, or on what the step talks about; the
// arrow's tip on the control the bubble names; else the window's edge.
{
  const arrow = { width: 33, height: 33 };
  const LIT = 12; // PAD and RING: what's lit, as the guide draws it round the step's block.
  const box = (left, top, width, height) => ({ left, top, width, height });
  const grow = (b) => box(b.left - LIT, b.top - LIT, b.width + LIT * 2, b.height + LIT * 2);
  const overlaps = (a, b) => a.left < b.left + b.width && a.left + a.width > b.left && a.top < b.top + b.height && a.top + a.height > b.top;
  /** The arrow's tip: the end it points with. */
  const tip = (at) => ({ up: [at.arrow.x + 16.5, at.arrow.y], down: [at.arrow.x + 16.5, at.arrow.y + 33],
    left: [at.arrow.x, at.arrow.y + 16.5], right: [at.arrow.x + 33, at.arrow.y + 16.5] })[at.point];
  const cases = [
    // Mac, step 1: the folder card under Start here's header and the page's title.
    { name: 'mac folder', view: [1100, 780], control: box(284, 266, 112, 26), lit: box(269, 182, 782, 124),
      keep: [box(252, 26, 816, 76), box(269, 136, 782, 28)], side: 'below' },
    // Phone, step 1: Start here's header right above the card.
    { name: 'phone folder', view: [390, 844], control: box(48, 420, 118, 28), lit: box(34, 336, 322, 124),
      keep: [box(16, 118, 358, 80), box(33, 250, 324, 70)], side: 'below' },
    // Mac, step 2: the checks, the other checks above them, Hire (greyed), Check again and why Hire is greyed below.
    { name: 'mac checks', view: [1100, 780], control: box(269, 273, 782, 232), lit: box(269, 273, 782, 232),
      keep: [box(269, 0, 782, 252), box(269, 526, 186, 28), box(463, 526, 100, 28), box(269, 571, 640, 16)], side: 'below' },
    // Phone, step 2: Check again wrapped under Hire.
    { name: 'phone checks', view: [390, 844], control: box(33, 300, 324, 230), lit: box(33, 300, 324, 230),
      keep: [box(33, 0, 324, 280), box(33, 546, 230, 40), box(33, 594, 110, 40), box(33, 644, 324, 40)] },
    // Mac, step 3: Hire, the checks just green above it; then with a refusal said under it.
    { name: 'mac hire', view: [1100, 780], control: box(269, 377, 186, 26), lit: box(269, 377, 186, 26),
      keep: [box(269, 0, 782, 121), box(269, 141, 782, 216)], side: 'right' },
    { name: 'mac hire refused', view: [1100, 780], control: box(269, 377, 186, 26), lit: box(269, 377, 186, 26),
      keep: [box(269, 0, 782, 121), box(269, 141, 782, 216), box(269, 423, 770, 34)] },
    // Phone, step 3, with and without a refusal said.
    { name: 'phone hire', view: [390, 844], control: box(33, 450, 230, 40), lit: box(33, 450, 230, 40),
      keep: [box(33, 200, 324, 230)] },
    { name: 'phone hire refused', view: [390, 844], control: box(33, 450, 230, 40), lit: box(33, 450, 230, 40),
      keep: [box(33, 200, 324, 230), box(33, 500, 324, 60)] },
  ];
  for (const c of cases) {
    const view = { width: c.view[0], height: c.view[1] };
    for (const bubble of [{ width: 326, height: 74 }, { width: 326, height: 110 }]) {
      const around = grow(c.lit);
      const at = g.place(c.control, view, bubble, arrow, around, c.keep);
      const b = box(at.bubble.x, at.bubble.y, bubble.width, bubble.height);
      const what = `${c.name} (${bubble.height}): ${JSON.stringify(at)}`;
      assert(b.left >= 12 && b.top >= 12 && b.left + b.width <= view.width - 12 && b.top + b.height <= view.height - 12, 'the bubble inside the window: ' + what);
      assert(!overlaps(b, around), "the bubble off what's lit: " + what);
      assert(c.keep.every((k) => !overlaps(b, k)), 'the bubble off the title, the header and what the step talks about: ' + what);
      if (c.side) assert(at.side === c.side, `${c.side}: ` + what);
      // The tip on the control, or just off what's lit round it, pointing at it.
      const [x, y] = tip(at);
      const r = c.control;
      const dx = Math.max(r.left - x, 0, x - (r.left + r.width)), dy = Math.max(r.top - y, 0, y - (r.top + r.height));
      assert(Math.hypot(dx, dy) <= LIT + 4 + 1, 'the arrow at the control: ' + what);
      const toward = { up: y >= r.top + r.height, down: y <= r.top, left: x >= r.left + r.width, right: x <= r.left }[at.point];
      assert(toward && (at.point === 'up' || at.point === 'down' ? x >= r.left && x <= r.left + r.width : y >= r.top && y <= r.top + r.height), 'the arrow points at the control: ' + what);
      // Nothing kept under the arrow either.
      assert(c.keep.every((k) => !overlaps(box(at.arrow.x, at.arrow.y, 33, 33), k)), 'the arrow off what the step talks about: ' + what);
    }
  }
  // A control out of sight: no bubble parked away from it, but a chip at the edge nearest it, toward it.
  for (const view of [{ width: 1100, height: 780 }, { width: 390, height: 844 }]) {
    const bubble = { width: 326, height: 90 }, chip = { width: 150, height: 32 };
    let at = g.place(box(100, 3000, 100, 30), view, bubble, arrow, undefined, [], { chip });
    assert(at.side === 'chip' && at.point === 'down' && at.bubble.y + 32 === view.height - 12, view.width + ': a control below the window: ' + JSON.stringify(at));
    at = g.place(box(100, -900, 100, 30), view, bubble, arrow, undefined, [], { chip });
    assert(at.side === 'chip' && at.point === 'up' && at.bubble.y === 12, view.width + ': a control above the window: ' + JSON.stringify(at));
  }
}

// Round 2's shots (fresh-eyes-2 01, 03, 01p), with their lines of text: the bubble covers none near what's
// lit (the checklist's rows under the folder card, the red line under Hire), and once Hire is pressed it's
// out of the way at once, in a corner.
{
  const arrow = { width: 33, height: 33 };
  const LIT = 12;
  const box = (left, top, width, height) => ({ left, top, width, height });
  const grow = (b) => box(b.left - LIT, b.top - LIT, b.width + LIT * 2, b.height + LIT * 2);
  const overlaps = (a, b) => a.left < b.left + b.width && a.left + a.width > b.left && a.top < b.top + b.height && a.top + a.height > b.top;
  const apart = (a, b) => Math.max(b.left - (a.left + a.width), a.left - (b.left + b.width), b.top - (a.top + a.height), a.top - (b.top + b.height), 0);
  const sidebar = [box(58, 18, 66, 18), box(38, 74, 42, 16), box(49, 117, 66, 17), box(49, 150, 92, 17), box(49, 183, 90, 17), box(49, 216, 40, 17),
    box(49, 249, 100, 17), box(18, 288, 30, 14), box(49, 315, 48, 17), box(49, 348, 56, 17)];
  // Shot 01: Start here's rows under the folder card, each a title and its detail.
  const rows01 = [box(316, 337, 200, 17), box(316, 358, 388, 16), box(316, 399, 196, 17), box(316, 418, 494, 16),
    box(316, 460, 204, 17), box(533, 461, 36, 14), box(316, 480, 620, 16), box(316, 497, 120, 16), box(992, 479, 32, 16),
    box(316, 537, 200, 17), box(316, 557, 520, 16), box(316, 598, 112, 17), box(437, 600, 36, 14), box(316, 617, 568, 16),
    box(316, 634, 34, 16), box(949, 616, 76, 16), box(316, 675, 150, 17), box(316, 694, 328, 16), box(316, 756, 226, 17), box(969, 766, 56, 14)];
  // Shot 03: what's under Hire: the red line (once refused), then Roles.
  const refusal = [box(271, 556, 737, 16), box(271, 577, 50, 16), box(338, 576, 110, 17)];
  const roles = [box(269, 652, 46, 20), box(351, 655, 110, 16), box(882, 655, 58, 16), box(972, 655, 64, 16), box(283, 698, 180, 17),
    box(283, 718, 465, 16), box(269, 754, 560, 16)];
  const above03 = [box(316, 8, 620, 16), box(316, 25, 100, 16), box(992, 9, 32, 16), box(316, 76, 200, 17), box(316, 95, 520, 16)];
  // Shot 01p: the rows under the card on a phone, and the bar at the top.
  const rows01p = [box(68, 18, 80, 20), box(45, 64, 110, 18), box(80, 488, 180, 17), box(80, 508, 240, 16), box(80, 526, 130, 16),
    box(80, 567, 206, 17), box(80, 587, 240, 16), box(80, 605, 250, 16), box(80, 646, 150, 17), box(80, 666, 110, 17), box(149, 668, 42, 14),
    box(80, 687, 250, 16), box(80, 704, 252, 16), box(80, 722, 250, 16), box(60, 756, 46, 16), box(80, 803, 203, 17), box(80, 823, 215, 16)];
  const cases = [
    { name: 'mac folder (01)', view: [1100, 780], control: box(284, 266, 112, 26), lit: box(269, 182, 782, 124),
      keep: [box(252, 26, 816, 76), box(269, 136, 782, 28)], text: [...sidebar, ...rows01], side: { 74: 'below' } },
    // On a phone there's no room clear of the rows: since round 3, against its arrow rather than in a corner.
    { name: 'phone folder (01p)', view: [390, 844], control: box(48, 420, 118, 28), lit: box(34, 336, 322, 124),
      keep: [box(16, 118, 358, 80), box(33, 250, 324, 70)], text: rows01p, side: { 74: 'attached', 110: 'attached' }, next: true },
    { name: 'mac hire (03)', view: [1100, 780], control: box(269, 510, 186, 27), lit: box(269, 510, 186, 27),
      keep: [box(269, 275, 782, 214), box(269, 0, 782, 253)], text: [...sidebar, ...above03, ...roles], side: { 74: 'right' } },
    { name: 'mac hire refused (03)', view: [1100, 780], control: box(269, 510, 186, 27), lit: box(269, 510, 186, 27),
      keep: [box(269, 275, 782, 214), box(269, 0, 782, 253)], text: [...sidebar, ...above03, ...refusal, ...roles], side: { 74: 'edge', 110: 'edge' } },
    { name: 'mac hire pressed (03)', aside: true, view: [1100, 780], control: box(269, 510, 186, 27), lit: box(269, 510, 186, 27),
      keep: [box(269, 275, 782, 214), box(269, 0, 782, 253)], text: [...sidebar, ...above03, ...roles], side: { 74: 'edge', 110: 'edge' }, after: refusal },
  ];
  for (const c of cases) {
    const view = { width: c.view[0], height: c.view[1] };
    for (const bubble of [{ width: Math.min(326, view.width - 30), height: 74 }, { width: Math.min(326, view.width - 30), height: 110 }]) {
      const around = grow(c.lit);
      const at = g.place(c.control, view, bubble, arrow, around, c.keep, { text: c.text, aside: !!c.aside });
      const b = box(at.bubble.x, at.bubble.y, bubble.width, bubble.height);
      const what = `${c.name} (${bubble.height}): ${JSON.stringify(at)}`;
      assert(b.left >= 12 && b.top >= 12 && b.left + b.width <= view.width - 12 && b.top + b.height <= view.height - 12, 'the bubble inside the window: ' + what);
      assert(!overlaps(b, around) && c.keep.every((k) => !overlaps(b, k)), "the bubble off what's lit and kept: " + what);
      // Every line of text within 120 px of what's lit stays readable; so does what the press brings. On a
      // phone (next), the bubble is against its arrow instead.
      const near = [...c.text, ...(c.after || [])].filter((line) => apart(line, around) <= 120);
      const hidden = near.filter((line) => overlaps(b, line));
      if (c.next) assert(apart(b, box(at.arrow.x, at.arrow.y, 33, 33)) === 0, 'the bubble against its arrow: ' + what);
      else assert(!hidden.length, 'the bubble off the text near what it lights: ' + JSON.stringify(hidden) + ' ' + what);
      if (c.side && c.side[bubble.height]) assert(at.side === c.side[bubble.height], `${c.side[bubble.height]}: ` + what);
      assert(c.keep.every((k) => !overlaps(box(at.arrow.x, at.arrow.y, 33, 33), k)), 'the arrow off what is kept: ' + what);
    }
  }
  // Without text, as before: what's lit wider than the bubble, something kept under the arrow: the bubble
  // along what's lit rather than over it.
  const at = g.place(box(284, 266, 112, 26), { width: 1100, height: 780 }, { width: 326, height: 74 }, arrow, grow(box(269, 182, 782, 124)), [box(269, 330, 420, 40)]);
  assert(at.side === 'below' && at.bubble.x >= 269 + 420, 'along what is lit, off what is kept: ' + JSON.stringify(at));
}

// Round 3's shots (fresh-eyes-3 04, 05/12, 01 phone), with the logo and Search kept as setup.html keeps
// them: the bubble never on them; with the checks cut by the window's foot (after Open the office, the page
// scrolled up), no bubble parked in a corner and no arrow on whatever row is at the edge, but a chip toward
// them; on a phone, the bubble against its arrow; and the chip off the phone's bar across the top.
{
  const arrow = { width: 33, height: 33 }, chip = { width: 150, height: 32 };
  const LIT = 12;
  const box = (left, top, width, height) => ({ left, top, width, height });
  const grow = (b) => box(b.left - LIT, b.top - LIT, b.width + LIT * 2, b.height + LIT * 2);
  const overlaps = (a, b) => a.left < b.left + b.width && a.left + a.width > b.left && a.top < b.top + b.height && a.top + a.height > b.top;
  const apart = (a, b) => Math.max(b.left - (a.left + a.width), a.left - (b.left + b.width), b.top - (a.top + a.height), a.top - (b.top + b.height), 0);
  // The Mac's side column: the logo and its name, then Search (kept); the sections under them (text).
  const brand = box(10, 14, 200, 38), search = box(10, 66, 199, 31);
  const nav = [box(49, 117, 66, 17), box(49, 150, 92, 17), box(49, 183, 90, 17), box(49, 216, 40, 17), box(49, 249, 100, 17),
    box(18, 288, 30, 14), box(49, 315, 48, 17), box(49, 348, 56, 17)];
  const head = box(252, 26, 816, 76), cardHead = box(269, 136, 782, 28), startChecks = box(269, 221, 782, 396);
  // Shot 05: the other checks above (each a title and its detail, and their buttons).
  const rows05 = [box(316, 236, 168, 17), box(316, 253, 388, 16), box(316, 296, 194, 17), box(316, 314, 493, 16), box(316, 355, 204, 17),
    box(533, 357, 36, 14), box(316, 374, 620, 16), box(316, 391, 120, 16), box(992, 373, 32, 16), box(316, 433, 200, 17), box(316, 451, 520, 16),
    box(316, 493, 112, 17), box(316, 512, 568, 16), box(949, 511, 76, 16), box(316, 571, 150, 17), box(316, 589, 328, 16),
    box(271, 42, 90, 22), box(341, 70, 390, 16), box(341, 89, 96, 16), box(269, 140, 72, 18), box(351, 144, 68, 14), box(945, 142, 94, 16)];
  // Shot 04: the checks in the middle; Hire, Check again and why Hire is greyed under them (kept); Roles below.
  const checks04 = box(269, 252, 782, 276);
  const keep04 = [brand, search, box(269, -165, 782, 396), box(269, 548, 186, 27), box(463, 548, 100, 27), box(269, 594, 446, 16)];
  const text04 = [...nav, box(316, 12, 520, 16), box(316, 54, 236, 17), box(316, 72, 520, 16), box(316, 114, 166, 17), box(437, 116, 36, 14),
    box(316, 133, 556, 16), box(316, 150, 50, 16), box(949, 125, 76, 16), box(316, 192, 150, 17), box(316, 211, 328, 16),
    box(269, 666, 40, 20), box(318, 668, 105, 16), box(881, 664, 60, 16), box(972, 664, 66, 16), box(282, 712, 180, 17), box(282, 731, 466, 16)];
  // Phone, shot 01: the bar across the top (the logo, the section, Search), the title, Start here's header.
  const bar = box(0, 0, 390, 104);
  const phoneKeep = [bar, box(16, 124, 358, 92), box(33, 254, 324, 30)];
  const rows01 = [box(80, 463, 180, 17), box(80, 483, 240, 16), box(80, 500, 130, 16), box(80, 542, 206, 17), box(80, 561, 240, 16),
    box(80, 579, 250, 16), box(80, 621, 150, 17), box(80, 641, 110, 17), box(149, 643, 42, 14), box(80, 661, 250, 16), box(80, 679, 252, 16),
    box(80, 696, 250, 16), box(48, 726, 58, 28), box(80, 786, 203, 17), box(80, 805, 215, 16), box(80, 822, 100, 16)];
  const cases = [
    // Shot 04: the checks in sight: below them, past Hire and Check again, off the logo and Search.
    { name: 'mac checks (04)', view: [1100, 780], control: checks04, lit: checks04, keep: keep04, text: text04, side: 'below' },
    // Shots 05 and 12: the page scrolled up to Open the office, the checks cut by the window's foot.
    { name: 'mac checks cut (05)', view: [1100, 780], control: box(269, 640, 782, 276), lit: box(269, 640, 782, 276),
      keep: [brand, search, head, cardHead, startChecks], text: [...nav, ...rows05], side: 'chip', point: 'down' },
    // The same, further down: its middle out of the window.
    { name: 'mac checks below (05)', view: [1100, 780], control: box(269, 760, 782, 276), lit: box(269, 760, 782, 276),
      keep: [brand, search, head, cardHead, startChecks], text: [...nav, ...rows05], side: 'chip', point: 'down' },
    // A button in them pressed, while cut: the chip all the same, small at the foot, out of the way.
    { name: 'mac checks cut, pressed (05)', aside: true, view: [1100, 780], control: box(269, 640, 782, 276), lit: box(269, 640, 782, 276),
      keep: [brand, search, head, cardHead, startChecks], text: [...nav, ...rows05], side: 'chip', point: 'down' },
    // Phone, shot 01: no room clear of the rows: against its arrow, under Use this folder.
    { name: 'phone folder (01)', view: [390, 844], control: box(48, 388, 118, 29), lit: box(36, 306, 318, 121),
      keep: phoneKeep, text: rows01, side: 'attached', point: 'up', next: true },
    // Phone: the checks cut by the foot, or scrolled past under the bar: a chip, never on the bar.
    { name: 'phone checks cut', view: [390, 844], control: box(33, 600, 324, 330), lit: box(33, 600, 324, 330),
      keep: phoneKeep, text: rows01, side: 'chip', point: 'down' },
    { name: 'phone checks above', view: [390, 844], control: box(33, -420, 324, 330), lit: box(33, -420, 324, 330),
      keep: [bar], text: [], side: 'chip', point: 'up' },
  ];
  for (const c of cases) {
    const view = { width: c.view[0], height: c.view[1] };
    for (const bubble of [{ width: Math.min(326, view.width - 30), height: 74 }, { width: Math.min(326, view.width - 30), height: 110 }]) {
      const around = grow(c.lit);
      const at = g.place(c.control, view, bubble, arrow, around, c.keep, { text: c.text, aside: !!c.aside, chip });
      const size = at.side === 'chip' ? chip : bubble;
      const b = box(at.bubble.x, at.bubble.y, size.width, size.height);
      const what = `${c.name} (${bubble.height}): ${JSON.stringify(at)}`;
      assert(at.side === c.side && (!c.point || at.point === c.point), `${c.side} ${c.point || ''}: ` + what);
      assert(b.left >= 12 && b.top >= 12 && b.left + b.width <= view.width - 12 && b.top + b.height <= view.height - 12, 'inside the window: ' + what);
      // Never on the logo, Search, the bar across the top, the title or anything else kept.
      assert(c.keep.every((k) => !overlaps(b, k)), 'off the logo, Search and what is kept: ' + what);
      if (at.side === 'chip') {
        // At the edge toward what's to be used, under the control's middle as far as the window lets it.
        assert(c.point === 'down' ? b.top + b.height >= view.height - 12 - 1 : b.top < c.control.top + c.control.height + view.height, 'the chip at that edge: ' + what);
        continue;
      }
      const a = box(at.arrow.x, at.arrow.y, 33, 33);
      assert(!overlaps(b, around), "the bubble off what's lit: " + what);
      assert(c.keep.every((k) => !overlaps(a, k)), 'the arrow off what is kept: ' + what);
      // The arrow on what it names, inside the window: never on a row at the edge for want of the rest.
      assert(apart(a, c.control) <= LIT + 4 + 1 && a.top >= 0 && a.top + 33 <= view.height, 'the arrow at the control: ' + what);
      if (c.next) assert(apart(b, a) === 0, 'the bubble against its arrow: ' + what);
      if (!c.aside && !c.next) {
        const hidden = c.text.filter((line) => apart(line, around) <= 120 && overlaps(b, line));
        assert(!hidden.length, 'off the text near what it lights: ' + JSON.stringify(hidden) + ' ' + what);
      }
    }
  }
  // Under the phone's bar: a chip toward a control above is moved below the bar.
  const up = g.place(box(33, -420, 324, 330), { width: 390, height: 844 }, { width: 326, height: 74 }, arrow, undefined, [bar], { chip });
  assert(up.bubble.y >= 104 + 4, 'the chip below the bar across the top: ' + JSON.stringify(up));
  // Shot 05 before the fix: every corner covered something, and the top-left one (the logo and Search) won.
  // Pressed with what's lit low in the window, in sight, the corner of last resort is now just below Search
  // (a bubble is wider than the side column: it may touch the title's edge, never the logo or Search).
  const corner = g.place(box(269, 690, 782, 60), { width: 1100, height: 780 }, { width: 326, height: 92 }, arrow, grow(box(269, 690, 782, 60)),
    [brand, search, head, cardHead, startChecks], { text: [...nav, ...rows05], aside: true });
  const cb = box(corner.bubble.x, corner.bubble.y, 326, 92);
  assert(corner.bubble.x === 12 && corner.bubble.y >= 66 + 31 + 4 && corner.bubble.y <= 12 + 120 && !overlaps(cb, brand) && !overlaps(cb, search),
    'the corner of last resort just below Search: ' + JSON.stringify(corner));
}

// In the page: a step whose control is cut by the window's foot shows a chip ("↓ Continue below") in place
// of the bubble and the arrow; the chip brings it into sight; back in sight, the bubble and the arrow.
{
  const p = page();
  current = p;
  const list = p.add('div', 'checks', [269, 640, 782, 276]);
  const guide = g.create({ document: p.doc, steps: [{ id: 'checks', target: '#checks', say: 'Make these checks green', done: () => false }] });
  guide.start();
  const layer = layerOf(p);
  const more = layer.all().find((el) => el.className === 'smol-guide-more');
  assert(layer.classList.contains('more') && more.textContent === '↓ Continue below', 'cut by the foot: the chip: ' + layer.className + ' ' + more.textContent);
  assert(parseFloat(more.style.top) + 80 + 6 === 780 - 12, 'the chip at the foot of the window: ' + more.style.top);
  const before = list.scrolled;
  more.onclick();
  assert(list.scrolled === before + 1, 'the chip brings the checks into sight');
  list.box = { left: 269, top: 200, width: 782, height: 276 };
  p.wait(450);
  assert(!layer.classList.contains('more'), 'in sight: the bubble and the arrow again, no chip');
  list.box = { left: 269, top: -500, width: 782, height: 276 };
  p.wait(450);
  assert(layer.classList.contains('more') && more.textContent === '↑ Continue above', 'scrolled past: the chip, toward it: ' + more.textContent);
  guide.stop();
}

// In the page: on a phone the pill (Next step in Easy setup → Show me ×) shrinks to a small chip once you
// scroll, as it would sit on what you read; it's full size again in the next section; never on a Mac.
{
  for (const phone of [true, false]) {
    const p = page();
    current = p;
    if (phone) p.win.matchMedia = (q) => ({ matches: q === g.NARROW });
    const s = { pane: 'general' };
    p.add('button', 'use', [40, 300, 120, 28]).hidden = true;
    const guide = g.create({ document: p.doc, awayText: 'Next step in Easy setup', where: () => s.pane,
      steps: [{ id: 'folder', target: '#use', away: () => true, reveal: () => {}, say: 'Pick where your hires work', done: () => false }] });
    guide.start();
    const layer = layerOf(p);
    const scroll = (y) => { p.win.scrollY = y; for (const l of p.listeners.win.filter((l) => l.type === 'scroll')) l.fn(); };
    assert(layer.classList.contains('pill') && !layer.classList.contains('small') && goOf(p).textContent === 'Show me', 'first the full pill');
    scroll(30);
    assert(!layer.classList.contains('small'), 'a nudge: still the full pill');
    scroll(400);
    if (!phone) {
      assert(!layer.classList.contains('small') && goOf(p).textContent === 'Show me', 'on a Mac it stays as it is');
      guide.stop();
      continue;
    }
    assert(layer.classList.contains('small') && goOf(p).textContent === 'Next step →', 'scrolled on a phone: a small chip');
    assert(goOf(p).title === 'Next step in Easy setup' && goOf(p).getAttribute('aria-label') === 'Next step in Easy setup: show me', 'the chip says what it is: ' + goOf(p).title);
    p.wait(450);
    assert(layer.classList.contains('small'), 'it stays small');
    s.pane = 'phone';
    p.wait(450);
    assert(!layer.classList.contains('small') && goOf(p).textContent === 'Show me' && goOf(p).title === '', 'the next section: the full pill again');
    guide.stop();
  }
}

// In the page: the step's control pressed sends the bubble out of the way at once, before what the press
// brings, and it stays there until the step changes; a click elsewhere in the page doesn't.
{
  const { p, s, guide, el } = startHere();
  s.folder = s.green = true;
  guide.start();
  const bubble = layerOf(p).all().find((e) => e.className === 'smol-guide-bubble');
  const where = () => [parseFloat(bubble.style.left), parseFloat(bubble.style.top)];
  const LIT_EDGE = 12; // PAD and RING

  const before = where();
  assert(textOf(p).startsWith('Now your first hire') && before[0] === 40 + 180 + LIT_EDGE + 4 + 33, 'beside Hire first: ' + before);
  p.fire('click', p.add('button', 'elsewhere', [700, 20, 80, 20]));
  assert(where().join() === before.join(), 'a click elsewhere: the bubble stays');
  p.fire('click', el('hire'));
  const pressed = where();
  assert(pressed.join() !== before.join() && pressed[1] === 780 - 12 - (80 + 6), 'Hire pressed: in a corner at once, before any tick: ' + pressed);
  p.wait(900);
  assert(where().join() === pressed.join() && textOf(p).startsWith('Now your first hire'), 'refused, still the hire step: still out of the way');
  s.hiring = true;
  p.wait(450);
  s.hiring = false;
  p.wait(450);
  assert(where().join() === before.join(), 'the step shown anew: beside Hire again, until it is pressed: ' + where());
  guide.stop();
}

// In the page: the bubble off what the page keeps, the arrow on the control; elsewhere on the page, a small
// pill (Next step in Easy setup → Show me, ×), and Skip says it can be undone.
{
  const p = page();
  current = p;
  const s = { away: false, pane: 'easy' };
  p.add('header', 'head', [252, 26, 816, 76]);
  p.add('div', 'card-head', [269, 136, 782, 28]);
  const card = p.add('div', 'folder', [269, 182, 782, 124]);
  p.add('button', 'use', [284, 266, 112, 26], card);
  // Another pane: Easy setup, and the card with it, hidden.
  const away = (on, pane = on ? 'general' : 'easy') => { s.away = on; s.pane = pane; card.hidden = on; };
  const said = [];
  const guide = g.create({ document: p.doc, keep: '#head, #card-head', awayText: 'Next step in Easy setup', skipSays: 'Skip: Show me around again brings it back',
    where: () => s.pane, hideSays: "Hide for now: it's back in the next section you open", onFinish: (why) => said.push(why),
    steps: [{ id: 'folder', target: '#use', area: '#folder', away: () => s.away, reveal: () => away(false), say: 'Pick where your hires work', press: 'Use this folder', done: () => false }] });
  guide.start();
  const layer = layerOf(p);
  const bubble = layer.all().find((el) => el.className === 'smol-guide-bubble');
  const arrowEl = layer.all().find((el) => el.className === 'smol-guide-arrow');
  const at = { left: parseFloat(bubble.style.left), top: parseFloat(bubble.style.top), width: 266, height: 86 };
  const hits = (id) => { const b = p.doc.getElementById(id).box; return at.left < b.left + b.width && at.left + at.width > b.left && at.top < b.top + b.height && at.top + at.height > b.top; };
  assert(!hits('head') && !hits('card-head') && !hits('folder'), 'the bubble off the title, the header and the card: ' + JSON.stringify(at));
  assert(arrowEl.dataset.point === 'up' && parseFloat(arrowEl.style.top) === 266 + 26 + 4, 'the arrow under Use this folder, pointing up at it: ' + arrowEl.style.top);
  assert(countOf(p) === '' && skipOf(p).textContent === 'Skip', 'no count; Skip');
  assert(skipOf(p).title === 'Skip: Show me around again brings it back' && skipOf(p).getAttribute('aria-label') === skipOf(p).title, 'Skip says it can be undone');
  away(true);
  p.wait(450);
  assert(layer.classList.contains('pill') && layer.classList.contains('docked'), 'elsewhere: a pill');
  assert(textOf(p) === 'Next step in Easy setup →' && goOf(p).textContent === 'Show me' && skipOf(p).textContent === '×', 'the pill: ' + textOf(p));
  assert(skipOf(p).title === "Hide for now: it's back in the next section you open" && skipOf(p).getAttribute('aria-label') === skipOf(p).title, 'the pill\'s × says it only hides it');
  // The ×: hidden in this section only, the guide still on; back in the next section; Skip again at the step.
  skipOf(p).onclick();
  p.wait(900);
  assert(guide.running && !said.length && layer.hidden, 'the pill\'s ×: hidden here, the guide not over: ' + said.join());
  away(true, 'phone');
  p.wait(450);
  assert(!layer.hidden && layer.classList.contains('pill'), 'another section: the pill is back');
  skipOf(p).onclick();
  away(false);
  p.wait(450);
  assert(!layer.hidden && !layer.classList.contains('pill') && skipOf(p).textContent === 'Skip', 'back on Easy setup: the step, with Skip');
  assert(skipOf(p).title === 'Skip: Show me around again brings it back', 'Skip says it can be undone again');
  away(true);
  p.wait(450);
  goOf(p).onclick();
  assert(!layer.classList.contains('pill') && textOf(p).startsWith('Pick where your hires work'), 'Show me: back to the step, no pill');
  guide.stop();
}

/* ------------------------------------------------------------------ setup.html's welcome */

// On a first run the welcome waits for the guide; finished or skipped, it opens as before; an app
// without the guide, and anyone the guide is over for, see it as before.
{
  const setup = fs.readFileSync(setupPath, 'utf8');
  const from = setup.indexOf('  function showInvite() {');
  const to = setup.indexOf('\n  }\n', from) + 4;
  assert(from > 0 && to > from, 'setup.html has showInvite');
  const opened = [];
  const make = new Function('window', 'state', 'opened', `
    let welcomeOpen = false;
    const invited = () => false, layout = { room: {} }, STANDARD_ROOM = {}, petsOf = () => [];
    const $ = () => ({ hidden: true });
    const openWelcome = () => opened.push(1);
    ${setup.slice(from, to)}
    return showInvite;`);
  const run = (window, state) => { opened.length = 0; make(window, state, opened)(); return opened.length === 1; };
  const app = { AgentOffice: {}, SmolcorpGuide: {} };
  assert(!run(app, { guide: { done: false } }), 'a first run: the welcome waits for the guide');
  assert(run(app, { guide: { done: true } }), 'the guide over: the welcome opens');
  assert(run({ AgentOffice: {} }, { guide: { done: false } }), 'no guide on the page: as before');
  assert(run(app, {}), 'an app without the guide: as before');
  assert(/render\(\);\s*\n\s*syncGuide\(asked\);/.test(setup) && /\n    showInvite\(\);\n/.test(setup), 'each state asks again, so it opens once the guide is over');
}

// The welcome's last step never says the office is ready while no General Manager is there: it says
// what's missing and leads to Hire your General Manager. Hired, started by hand, or its hire under way: All set.
{
  const setup = fs.readFileSync(setupPath, 'utf8');
  const from = setup.indexOf('  function welcomeEnd() {');
  const to = setup.indexOf('\n  }\n', from) + 4;
  assert(from > 0 && to > from, 'setup.html has welcomeEnd');
  const run = (vars) => {
    const els = {};
    const $ = (id) => (els[id] = els[id] || { hidden: id === 'start-gm' ? !!vars.byHand : false, textContent: '' });
    const end = new Function('$', 'state', 'gmKnown', 'gmHired', 'gmHireRunning', `${setup.slice(from, to)}\nreturn welcomeEnd;`)(
      $, vars.state || {}, vars.known !== false, !!vars.hired, !!vars.running);
    const missing = end();
    return { missing, title: $('welcome-end-title').textContent, ready: !$('welcome-ready').hidden, lead: !$('welcome-gm-missing').hidden };
  };
  let r = run({});
  assert(r.missing && r.title === 'Next: hire your General Manager' && !r.ready && r.lead, 'no GM: says what is missing, not ready: ' + JSON.stringify(r));
  r = run({ hired: true });
  assert(!r.missing && r.title === 'All set' && r.ready && !r.lead, 'a GM hired: All set');
  assert(!run({ byHand: true }).missing && !run({ running: true }).missing, 'started by hand, or its hire under way: not missing');
  assert(run({ known: false, state: { gmHired: false } }).missing && !run({ known: false, state: { gmHired: true } }).missing, 'not known yet: as the state says');
  assert(/\$\('welcome-gm'\)\.onclick = \(\) => \{\n    answerInvite\(null\);\n    go\('easy', \$\('start-gm'\)\);/.test(setup), 'its button leads to Hire your General Manager on Easy setup');
  assert(setup.includes('<button class="btn primary" id="welcome-gm">Hire your General Manager</button>'), 'the welcome has the button');
}

if (failures) { console.error(`${failures} failed`); process.exit(1); }
console.log('ok');
