/*
 * The first-run guide's engine (Resources/web/guide.js) run in node, on a tiny stand-in for the page:
 * just enough of the DOM for the guide (elements with a box, a few selectors, clicks and keys, timers that
 * only run when told). It checks what a click counts for, Esc and Skip, a dialog over the page, the
 * keyboard moved only when the step changes, a tour of steps already done, the last step waiting while the
 * GM starts and docked when its control never comes, and stop() leaving no timer or listener behind. Where
 * the bubble and the arrow go, on the fresh-eyes shots' Start here (1100×780 and 390×844), and the pill off
 * Easy setup. Then setup.html's welcome, which waits for the guide on a first run.
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
  // A control out of sight: the arrow at the edge nearest it, pointing that way, all inside the window.
  for (const view of [{ width: 1100, height: 780 }, { width: 390, height: 844 }]) {
    const bubble = { width: 326, height: 90 };
    let at = g.place(box(100, 3000, 100, 30), view, bubble, arrow);
    assert(at.point === 'down' && at.arrow.y + 33 <= view.height && at.bubble.y + 90 <= at.arrow.y, view.width + ': a control below the window');
    at = g.place(box(100, -900, 100, 30), view, bubble, arrow);
    assert(at.point === 'up' && at.arrow.y >= 0 && at.bubble.y >= at.arrow.y + 33, view.width + ': a control above the window');
  }
}

// In the page: the bubble off what the page keeps, the arrow on the control; elsewhere on the page, a small
// pill (Next step in Easy setup → Show me, ×), and Skip says it can be undone.
{
  const p = page();
  current = p;
  const s = { away: false };
  p.add('header', 'head', [252, 26, 816, 76]);
  p.add('div', 'card-head', [269, 136, 782, 28]);
  const card = p.add('div', 'folder', [269, 182, 782, 124]);
  p.add('button', 'use', [284, 266, 112, 26], card);
  // Another pane: Easy setup, and the card with it, hidden.
  const away = (on) => { s.away = on; card.hidden = on; };
  const guide = g.create({ document: p.doc, keep: '#head, #card-head', awayText: 'Next step in Easy setup', skipSays: 'Skip: Show me around again brings it back',
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

if (failures) { console.error(`${failures} failed`); process.exit(1); }
console.log('ok');
