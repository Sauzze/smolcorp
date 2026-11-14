package desktop

import (
	"path/filepath"
	"strings"
	"testing"
)

// The talk bubble's keyboard and screen reader (1.26, P7a): the focus order, Esc giving the focus back, digits only
// where the plan says, the live region, the hidden desk list, reduced motion and contrast; in node, on a stand-in
// page, and in the room itself (Enter and Tab). Skipped without node.

// A stand-in document whose elements know their parent, what has the focus (dropped to the body when it leaves the
// page, as a browser does), their order in the page, and a few selectors: `.class`, a tag, or several with commas.
// With El.strict, a key or box greyed drops the focus and takes none, as an engine that blurs it at once would.
// With El.late ('microtask' or 'timer'), it takes none either, but drops the focus to the body only after the call
// that greyed it has returned, as Chromium does at its next frame (RT126P7AR2 B2b).
// dressDoc(doc) gives an existing stand-in a body, a head and these elements, canvases left to `canvas` if given.
const talkDOM = `
let theDoc = null;
class El {
  static strict = false;
  static late = null;
  get disabled() { return !!this.off; }
  set disabled(v) {
    this.off = !!v;
    if (this.off && El.strict && theDoc.focused === this) theDoc.focused = null;
    const drop = () => { if (this.off && theDoc.focused === this) theDoc.focused = null; };
    if (this.off && El.late && theDoc.focused === this) (El.late === 'microtask' ? queueMicrotask : setTimeout)(drop);
  }
  compareDocumentPosition(other) { const all = theDoc.body.all(), a = all.indexOf(this), b = all.indexOf(other); return b > a ? 4 : b < a ? 2 : 0; }
  constructor(tag) {
    Object.assign(this, { tagName: tag.toUpperCase(), children: [], style: { setProperty() {} }, className: '', hidden: false, disabled: false,
      value: '', placeholder: '', type: '', listeners: {}, attrs: {}, parentNode: null, offsetWidth: 100, offsetHeight: 20, scrollTop: 0,
      scrollHeight: 0, clientHeight: 60, width: 0, height: 0 });
  }
  get classList() { const el = this; return { add(c) { el.className += ' ' + c; }, remove(c) { el.className = el.className.split(' ').filter((x) => x !== c).join(' '); } }; }
  set textContent(t) { this.replaceChildren(String(t)); }
  get textContent() { return this.children.map((c) => (typeof c === 'string' ? c : c.textContent)).join(''); }
  get isConnected() { let n = this; while (n.parentNode) n = n.parentNode; return n === theDoc.body || n === theDoc.head; }
  append(...kids) { for (const k of kids) { if (typeof k !== 'string') { if (k.parentNode) k.remove(); k.parentNode = this; } this.children.push(k); } }
  replaceChildren(...kids) { for (const c of this.children) if (typeof c !== 'string') c.parentNode = null; this.children = []; this.append(...kids); }
  remove() { if (this.parentNode) { this.parentNode.children = this.parentNode.children.filter((c) => c !== this); this.parentNode = null; } }
  setAttribute(k, v) { this.attrs[k] = String(v); }
  getAttribute(k) { return k in this.attrs ? this.attrs[k] : null; }
  addEventListener(type, f) { (this.listeners[type] = this.listeners[type] || []).push(f); }
  dispatch(type, e = {}) {
    const ev = { target: this, preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; }, ...e };
    for (let el = this; el && !ev.stopped; el = el.parentNode) (el.listeners[type] || []).forEach((f) => f(ev));
    if (!ev.stopped) (theDoc.listeners[type] || []).slice().forEach((f) => f(ev));
    return ev;
  }
  matches(sel) { return sel.split(',').map((s) => s.trim()).some((s) => (s[0] === '.' ? this.className.split(' ').includes(s.slice(1)) : this.tagName === s.toUpperCase())); }
  all() { return this.children.filter((c) => typeof c !== 'string').flatMap((c) => [c, ...c.all()]); }
  querySelector(sel) { return this.all().find((c) => c.matches(sel)) || null; }
  querySelectorAll(sel) { return this.all().filter((c) => c.matches(sel)); }
  find(cls) { return this.all().filter((c) => c.className.split(' ').includes(cls) && c.getClientRects().length); }
  getClientRects() { for (let n = this; n; n = n.parentNode) if (n.hidden) return []; return [{}]; }
  focus() { if (!((El.strict || El.late) && this.disabled)) theDoc.focused = this; }
  getContext() { return this.tagName === 'CANVAS' ? { drawImage() {} } : null; }
}
function dressDoc(doc, canvas) {
  const body = new El('body'), head = new El('head');
  theDoc = Object.assign(doc, { body, head, focused: null, listeners: {},
    createElement: (t) => (t === 'canvas' && canvas ? canvas() : new El(t)),
    getElementById: (id) => head.all().find((c) => c.id === id) || null,
    addEventListener: (type, f) => (doc.listeners[type] = doc.listeners[type] || []).push(f),
    removeEventListener: (type, f) => { doc.listeners[type] = (doc.listeners[type] || []).filter((g) => g !== f); } });
  Object.defineProperty(doc, 'activeElement', { configurable: true, get: () => (doc.focused && doc.focused.isConnected ? doc.focused : body) });
  return doc;
}
`

// The plain functions: arrows round the choices, Tab round the terminal, what the live region and the desk list say,
// a prompt said once, a refusal's words when the computer never answered; and contrast at WCAG AA on both panels,
// from the colours the style itself sets, and reduced motion there.
func TestTheTalkKeysAndWordsForAScreenReader(t *testing.T) {
	runNode(t, talkBubbleSources(t)+talkSame+`
const T = PIXEL_TERMINAL;
// ↑ ↓ (and ← →) move round the choices, from the one with the focus; any other key, or none focused, nothing
same([B.stepOf('ArrowDown', 0, 3), B.stepOf('ArrowDown', 2, 3), B.stepOf('ArrowUp', 0, 3), B.stepOf('ArrowLeft', 1, 3), B.stepOf('ArrowRight', 1, 3)],
  [1, 0, 2, 0, 2], 'arrows round the choices');
same([B.stepOf('Enter', 0, 3), B.stepOf('ArrowDown', -1, 3), B.stepOf('ArrowDown', 0, 0)], [null, null, null], 'not an arrow, or no choice focused');
// Tab round the terminal: from the last to the first, Shift+Tab from the first to the last, in from outside; else the page's own
same([T.wrapOf(4, 3, false), T.wrapOf(4, 0, true), T.wrapOf(4, -1, false), T.wrapOf(4, -1, true), T.wrapOf(4, 1, false), T.wrapOf(4, 2, true), T.wrapOf(0, -1, false)],
  [0, 3, 0, 3, null, null, null], 'Tab stays in the terminal');
const box = { parentNode: null }, inner = { parentNode: box }, deep = { parentNode: inner };
same([B.within(box, deep), B.within(inner, box), B.within(box, null)], [true, false, false], 'within');

// The live region: a desk starting to ask, by name; its new lines, the agent's and other sessions', never the owner's
// own words, tools or questions, the newest three
same(B.askingSaid('Rex', question(true)), 'Rex is asking: Run the migration now?', 'asking');
same(B.askingSaid('Bo', permission), 'Bo is asking: Allow Bash: make?', 'a permission');
const before = [item(1, 'user', 'go'), item(2, 'agent', 'on it')];
const after = [...before, item(3, 'tool', 'Bash: make', 'Bash'), item(4, 'agent', 'Built it.'), item(5, 'user', 'thanks'),
  item(6, 'question', 'Ship it?'), item(7, 'peer', 'Merged on main', 'Lead')];
same(B.linesSaid('Rex', before, after), ['Rex: Built it.', 'Lead to Rex: Merged on main'], 'its new lines');
same(B.linesSaid('Rex', after, after), [], 'nothing new');
const many = ['a', 'b', 'c', 'd', 'e'].map((t, i) => item(10 + i, 'agent', t));
same(B.linesSaid('Rex', [], many), ['Rex: c', 'Rex: d', 'Rex: e'], 'the newest three');
same(B.linesSaid('Rex', [], [item(1, 'agent', 'x'.repeat(500))])[0].length, 'Rex: '.length + 200, 'each cut');
// Each prompt said once; a desk that stopped asking is forgotten, so its next one is new
const heard = new Map();
const row = (session, since) => ({ session, since, kind: 'permission', label: 'Bash: make', questions: [] });
same(B.newlyAsking(heard, [row('s1', 1), row('s2', 2)]).map((r) => r.session), ['s1', 's2'], 'first');
same(B.newlyAsking(heard, [row('s1', 1), row('s2', 2), row('s3', 3)]).map((r) => r.session), ['s3'], 'only the new one');
same(B.newlyAsking(heard, [row('s1', 9), row('s3', 3)]).map((r) => r.session), ['s1'], 'a new prompt on the same desk');
same(B.newlyAsking(heard, [row('s1', 9), row('s2', 2), row('s3', 3)]).map((r) => r.session), ['s2'], 'asking again after it stopped');
// The desk list: name, state as a word, and the question
same([B.deskSaid('Rex', 'idle', null), B.deskSaid('Bo', 'asking', permission), B.deskSaid('Cy', 'thinking', null)],
  ['Rex, idle', 'Bo, waiting for you: Allow Bash: make?', 'Cy, working'], 'the desk list');

// A computer that never answers: the deck's post gives { timedOut }, and the words wait with that said
(async () => {
  same(await B.said({ timedOut: true }), 'Your computer didn\'t answer in time. Look at the desk before you try again.', 'no answer in time');
  same(await B.said({ status: 409, text: async () => 'It\'s asking you something.' }), 'It\'s asking you something.', 'a refusal');
  same(B.limit({}), undefined, 'no signal without one to make');
  same(B.limit({ AbortSignal: { timeout: (ms) => 'gives up after ' + ms } }), 'gives up after 20000', 'twenty seconds, as the deck');

  // Contrast, WCAG AA: reading text 4.5:1, a focus ring 3:1, from each panel's own colours in the style
  const css = TALK_STYLE.CSS;
  const block = (sel) => { const at = css.indexOf(sel + ' {'); if (at < 0) throw new Error('no ' + sel); return css.slice(at, css.indexOf('}', at)); };
  const vars = (sel) => Object.fromEntries([...block(sel).matchAll(/--([a-z]+): (#[0-9a-f]{6})/g)].map((m) => [m[1], m[2]]));
  const bub = vars('.talk-desks, .talk-bubble, .talk-small, .talk-tag, .talk-needs'), pt = vars('.pt, .pt-dim');
  const lum = (hex) => [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
    .reduce((s, c, i) => s + c * [0.2126, 0.7152, 0.0722][i], 0);
  const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
  const mix = (a, b, t) => '#' + [1, 3, 5].map((i) => Math.round(parseInt(a.slice(i, i + 2), 16) * t + parseInt(b.slice(i, i + 2), 16) * (1 - t)).toString(16).padStart(2, '0')).join('');
  const has = (text) => { if (!css.includes(text)) throw new Error('the style no longer has ' + text + ': update this test'); return true; };
  const ask = '#252c3d', grey = '#e3ded2', hint = '#4b5263', muted = '#5b6170', problem = '#a12d2d';
  has('background: #252c3d'); has('background: ' + grey + '; color: ' + hint); has('::placeholder { color: ' + muted); has('.talk-problem { padding-top: 0; color: ' + problem);
  has('.pt-body .board-row.result { padding-left: 1.6em; opacity: .9; }');
  const text = [
    ['bubble: the state on its header', bub.dim, bub.edge], ['bubble: the conversation', bub.chalk, bub.screen], ['bubble: a tool line', bub.dim, bub.screen],
    ['bubble: who it is from', bub.orange, bub.screen], ['bubble: your words', bub.edge, bub.paper], ['bubble: the question', bub.yellow, ask],
    ['bubble: the hint', bub.dim, ask], ['bubble: a key', bub.edge, bub.cream], ['bubble: an orange key', bub.edge, bub.orange],
    ['bubble: a dark key', bub.cream, bub.slate], ['bubble: the line under the input', hint, bub.cream], ['bubble: a refusal', problem, bub.cream],
    ['bubble: the placeholder', muted, bub.paper], ['bubble: the greyed input\'s placeholder', muted, grey], ['bubble: the greyed input', hint, grey],
    ['terminal: the conversation', pt.chalk, pt.board], ['terminal: a tool line', pt.dim, pt.board], ['terminal: a result line', mix(pt.dim, pt.board, 0.9), pt.board],
    ['terminal: the state', pt.dim, pt.edge], ['terminal: a question line', pt.yellow, pt.board], ['terminal: the hint', pt.dim, ask],
    ['terminal: a waiting desk', pt.edge, pt.yellow], ['terminal: a desk\'s state', hint, pt.yellow], ['terminal: a refusal', '#f2a19b', pt.board],
  ];
  const ring = [['on the bubble\'s header and question', bub.orange, bub.edge], ['on its question', bub.orange, ask], ['on its cream', bub.edge, bub.cream],
    ['on the terminal\'s board', pt.orange, pt.board], ['on its header and input', pt.orange, pt.edge], ['over the room, against Needs you\'s edge', bub.yellow, bub.edge]];
  has('.talk-input, .talk-small, .pt-menu, .talk-desks { --focus: var(--edge); }'); has('.talk-needs:focus-visible { outline-color: var(--yellow)');
  const low = [...text.filter(([, a, b]) => ratio(a, b) < 4.5), ...ring.filter(([, a, b]) => ratio(a, b) < 3)].map(([what, a, b]) => what + ' ' + ratio(a, b).toFixed(2));
  same(low, [], 'contrast at WCAG AA');
  // Reduced motion: nothing grows or shrinks, the room doesn't zoom (zoomOf, above), the keys don't move as pressed
  same([/prefers-reduced-motion: reduce\) \{ \.talk-bubble\.grow, \.talk-bubble\.shrink \{ animation: none/.test(css),
    /prefers-reduced-motion: reduce\) \{\s*\.talk-key:active:not\(:disabled\) \{ transform: none/.test(css)], [true, true], 'reduced motion');
  // The live region and the desk list are out of sight, the list shown while the keyboard is in it
  same([/\.talk-live, \.talk-desks:not\(:focus-within\) \{[^}]*clip-path: inset\(50%\)/.test(css), /\.talk-desks:focus-within \{[^}]*left: 8px/.test(css)],
    [true, true], 'for a screen reader');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}

// The live bubble on a stand-in page, for the tests below: Rex (s1) idle, Bo (s2) and Al (s3) asking, Cy (s4) working,
// Hal (s5) a headless run asking; `press(key)` a key as a browser has it.
const talkStage = `
const doc = dressDoc({}), body = doc.body;
const timers = [];
const win = { document: doc, innerHeight: 800, setTimeout: (f) => timers.push(f), clearTimeout() {} };
const settle = () => new Promise((r) => setTimeout(r, 5));
const tick = async () => { timers.splice(0).forEach((f) => f()); await settle(); };
const res = (status, body) => ({ status, ok: status >= 200 && status < 300, json: async () => body, text: async () => String(body || '') });
// Rex (s1) idle, Bo (s2) asking since 300, Al (s3) asking since 100, Cy (s4) working, Hal (s5) a headless run asking
const names = { s1: 'Rex', s2: 'Bo', s3: 'Al', s4: 'Cy', s5: 'Hal' };
const waiting = { ok: false, lines: false, why: 'answer first' };
const cards = { s1: { state: 'idle', asking: null, last: '', size: 40, typing: typingOK },
  s2: { state: 'asking', asking: permission, size: 10, typing: waiting }, s3: { state: 'asking', asking: { ...permission, since: 100 }, size: 10, typing: waiting },
  s4: { state: 'thinking', asking: null, size: 10, typing: typingOK }, s5: { state: 'asking', asking: { ...permission, since: 50 }, size: 10, typing: waiting } };
const lines = { s1: [item(1, 'user', 'build it'), item(2, 'agent', 'built')] };
let bubbles = [{ id: 's1', state: 'idle', x: 40, top: 50, quiet: true }, { id: 's2', state: 'asking', since: 300, x: 90, top: 50 },
  { id: 's3', state: 'asking', since: 100, x: 140, top: 50 }, { id: 's4', state: 'thinking', x: 40, top: 100 }];
const sent = [];
let hold = null, answer = null;
const page = {
  get: async (path, params) => {
    sent.push(['get', path, params]);
    if (path === '/talk') return res(200, cards[params.session]);
    if (path === '/conversation') return res(200, { items: lines[params.session] || [], before: null, size: cards[params.session].size });
    return res(200, { asking: Object.keys(cards).filter((id) => cards[id].asking && bubbles.some((b) => b.id === id)).map((id) => ({ session: id, ...cards[id].asking })) });
  },
  post: async (path, params, signal, json) => {
    sent.push(['post', path, params, json]);
    if (hold) await hold;
    if (answer) { const a = answer; answer = null; return a; }
    return res(204);
  },
  flash() {}, tick() {}, openInTerminal: { shown: () => true, open() {} },
};
const canvas = new El('canvas');
canvas.getBoundingClientRect = () => ({ left: 0, top: 0, right: 640, bottom: 360, width: 640, height: 360 });
Object.assign(canvas, { width: 640, height: 360 });
body.append(canvas);
const anchorOf = (id) => ({ id, state: cards[id].state, since: 1, art: {}, css: { box: { left: 290, right: 314, top: 100, bottom: 124 }, tip: { x: 300, y: 122 } } });
const room = { canvas, anchor: (id) => (names[id] ? anchorOf(id) : null), view: () => ({ scale: 2, ox: 0, oy: 0, k: 1, left: 0, top: 0 }), cog: () => null,
  shape: () => ({ width: 320, height: 180 }), session: (id) => (names[id] ? { id, state: cards[id].state, conversation: true, project: 'shop', headless: id === 's5' } : null),
  name: (id) => names[id] || 'Agent' };
const bubble = B.create(win, page, room);
const panel = () => body.find('talk-bubble')[0];
const term = () => body.children.find((c) => c.className.split(' ')[0] === 'pt');
const said = () => body.find('talk-live')[0].children.map((p) => p.textContent);
const focusedText = () => doc.activeElement.textContent;
const deskKeys = () => body.find('talk-desks')[0].children;
const posts = () => sent.filter((s) => s[0] === 'post');
// A key as a browser has it: to what has the focus, then, unless taken there, its own job. Enter clicks a key (a Send
// sends its form) or sends an input's form; Tab steps to the next stop in the page, Shift+Tab back: Chromium's stops,
// a key or box not greyed, and the terminal's conversation, a scroller.
const stop = (el) => el.getClientRects().length && !el.disabled && (/^(BUTTON|TEXTAREA|INPUT)$/.test(el.tagName) || el.className.split(' ').includes('pt-body'));
function press(key, shiftKey = false) {
  const at = doc.activeElement, form = at.parentNode && at.parentNode.tagName === 'FORM' ? at.parentNode : null;
  if (at.dispatch('keydown', { key, shiftKey }).defaultPrevented) return;
  if (key === 'Enter' && at.tagName === 'BUTTON') {
    at.dispatch('click');
    if (at.type === 'submit' && form) form.dispatch('submit');
  } else if (key === 'Enter' && at.tagName === 'INPUT' && form) form.dispatch('submit');
  else if (key === 'Tab') {
    const all = body.all(), i = all.indexOf(at);
    ((shiftKey ? all.slice(0, Math.max(i, 0)).reverse() : all.slice(i + 1)).find(stop) || body).focus();
  }
}
`

// The live bubble and terminal on a stand-in page: the desk list and the live region; Tab to the small bubbles, longest
// waiting first; a small bubble opens its bubble with the focus on the first choice, arrows go round, Esc gives the
// focus to the room; from the desk list, back to its key; Enter on an idle desk; new lines and a new prompt said once;
// the focus kept in the bubble as its keys change; no digit for a headless run; Send once across a refresh, and a
// computer that never answers; in the terminal, digits and N only from inside it, Tab round it, Esc out of its menu
// back onto Menu, arrows round its choices, and Esc back to the room.
func TestTheTalkBubblesFocusOrderAndLiveRegion(t *testing.T) {
	runNode(t, talkBubbleSources(t)+talkSame+talkDOM+talkStage+`

(async () => {
  bubble.frame(bubbles);
  await settle();
  bubble.frame(bubbles);
  // The room takes the focus back, out of Tab's way; the live region is polite; nothing said of those already waiting
  same([canvas.tabIndex, canvas.attrs['data-talk-room']], [-1, ''], 'the room focusable, out of Tab\'s way');
  const live = body.find('talk-live')[0];
  same([live.attrs.role, live.attrs['aria-live'], said()], ['status', 'polite', []], 'a polite live region, quiet for those already waiting');
  // The desk list mirrors the room: waiting first, longest first, each with its state as a word and its question
  const desks = body.find('talk-desks')[0];
  same([desks.tagName, desks.attrs['aria-label'], deskKeys().map((k) => k.textContent)],
    ['NAV', 'Desks', ['Al, waiting for you: Allow Bash: make?', 'Bo, waiting for you: Allow Bash: make?', 'Cy, working', 'Rex, idle']], 'the desk list');
  same(deskKeys().every((k) => k.tagName === 'BUTTON'), true, 'each a real key');

  // Tab from the room: the small bubbles, longest waiting first (Al, then Bo), each a key named for its desk
  same(body.find('talk-smalls')[0].children.map((s) => s.querySelector('.talk-small-q').attrs['aria-label']),
    ['Al is asking: Allow Bash: make?', 'Bo is asking: Allow Bash: make?'], 'the small bubbles in Tab\'s order');
  same([bubble.tab(false), doc.activeElement.attrs['aria-label']], [true, 'Al is asking: Allow Bash: make?'], 'Tab: the longest waiting');
  same([bubble.tab(true), doc.activeElement.attrs['aria-label']], [true, 'Bo is asking: Allow Bash: make?'], 'Shift+Tab: the last');
  bubble.tab(false);
  // Enter on it (a key's click): its bubble, the focus on its first choice; arrows go round the choices
  doc.activeElement.dispatch('click');
  await settle();
  bubble.frame(bubbles);
  same([panel().attrs['aria-label'], focusedText()], ['Al: conversation', '1Yes'], 'opened, the focus on the first choice');
  doc.activeElement.dispatch('keydown', { key: 'ArrowDown' });
  same(focusedText(), '2No', '↓');
  doc.activeElement.dispatch('keydown', { key: 'ArrowDown' });
  same(focusedText(), '1Yes', '↓ round to the first');
  doc.activeElement.dispatch('keydown', { key: 'ArrowUp' });
  same(focusedText(), '2No', '↑ round to the last');
  same(posts(), [], 'no arrow picked anything');
  // Esc: closed, and the room has the focus (its small bubble went as it opened)
  doc.activeElement.dispatch('keydown', { key: 'Escape' });
  await tick();
  same([panel(), doc.activeElement === canvas], [undefined, true], 'Esc gives the focus back to the room');

  // From the desk list: Rex's bubble (an idle desk, no bubble drawn over its head), the focus in its input; Esc, back on Rex's key
  bubble.frame(bubbles);
  const rexKey = deskKeys().find((k) => k.textContent === 'Rex, idle');
  rexKey.focus();
  rexKey.dispatch('click');
  await settle();
  const input = () => panel().find('talk-input')[0].children[0];
  same([panel().attrs['aria-label'], doc.activeElement === input()], ['Rex: conversation', true], 'an idle desk from the list, the focus in its input');
  same(said(), [], 'nothing said of what was there as it opened');
  // New lines said once: the agent's, not the owner's own nor a tool
  lines.s1 = [...lines.s1, item(3, 'tool', 'Bash: npm test', 'Bash'), item(4, 'agent', 'All 42 tests pass.'), item(5, 'user', 'thanks')];
  cards.s1 = { ...cards.s1, size: 50 };
  await tick();
  same(said(), ['Rex: All 42 tests pass.'], 'its new line said');
  input().dispatch('keydown', { key: 'Escape' });
  await tick();
  same(doc.activeElement === deskKeys().find((k) => k.textContent === 'Rex, idle'), true, 'Esc: back on its key in the desk list');

  // Enter in the room on the picked desk: opened; again, the focus back in it; a desk the room hasn't, nothing
  canvas.focus();
  same([bubble.enter('s1'), panel() && panel().attrs['aria-label']], [true, 'Rex: conversation'], 'Enter opens the picked desk');
  await settle();
  canvas.focus();
  same([bubble.enter('s1'), doc.activeElement === input()], [true, true], 'Enter again: the focus back in it');
  same([bubble.enter('nobody'), bubble.enter(null)], [false, false], 'no bubble, nothing');

  // Send goes once, even when a refresh comes while the words are on their way; then Send is live again
  let release;
  hold = new Promise((r) => { release = r; });
  input().value = 'run the tests';
  panel().find('talk-input')[0].dispatch('submit');
  await tick();
  const send = panel().find('talk-input')[0].children[1];
  same(send.disabled, true, 'Send stays greyed across a refresh');
  panel().find('talk-input')[0].dispatch('submit');
  hold = null;
  release();
  await settle();
  same([posts().length, send.disabled, input().value], [1, false, ''], 'sent once, Send live again');
  // The computer never answers: the words stay, and say so
  answer = { timedOut: true };
  input().value = 'and lint';
  panel().find('talk-input')[0].dispatch('submit');
  await settle();
  same([input().value, panel().find('talk-problem')[0].textContent, send.disabled],
    ['and lint', 'Your computer didn\'t answer in time. Look at the desk before you try again.', false], 'no answer in time: the words kept');
  input().value = '';

  // A bubble whose keys are drawn again (the prompt answered from the terminal or the phone): the focus stays in it
  bubble.open('s2');
  await settle();
  same(focusedText(), '1Yes', 'Bo: the first choice');
  cards.s2 = { state: 'idle', asking: null, size: 10, typing: typingOK };
  await tick();
  same(doc.activeElement === input(), true, 'its choices gone, the focus in its input, not dropped to the page');
  cards.s2 = { state: 'asking', asking: permission, size: 10, typing: waiting };

  // A headless run's bubble has no keys, and its digits pick nothing
  bubbles = [...bubbles, { id: 's5', state: 'asking', since: 50, x: 190, top: 100 }];
  bubble.frame(bubbles);
  await settle();
  same(said(), ['Hal is asking: Allow Bash: make?'], 'Hal starts asking');
  bubble.open('s5');
  await settle();
  const before = posts().length;
  panel().dispatch('keydown', { key: '1' });
  await settle();
  same([panel().find('talk-key').filter((k) => /Yes|No/.test(k.textContent)).length, posts().length], [0, before], 'no digit for a headless run');
  bubble.close(true);
  await tick();

  // A desk starting to ask is said once, by name; those already waiting aren't said again
  cards.s4 = { state: 'asking', asking: { ...permission, since: 500, label: 'Bash: npm publish' }, size: 10, typing: waiting };
  bubbles = bubbles.map((b) => (b.id === 's4' ? { ...b, state: 'asking', since: 500 } : b));
  bubble.frame(bubbles);
  await settle();
  same(said(), ['Cy is asking: Allow Bash: npm publish?'], 'Cy starts asking');
  bubble.frame(bubbles);
  await settle();
  same(deskKeys()[3].textContent, 'Cy, waiting for you: Allow Bash: npm publish?', 'the desk list keeps up');

  // The terminal: modal; digits and N only from inside it, not from the room behind
  same(bubble.expand('s2'), true, 'Expand');
  await tick();
  const pt = term();
  same([pt.attrs['aria-modal'], pt.attrs['aria-label'], focusedText()], ['true', 'Bo: conversation', '1Yes'], 'a modal dialog, the focus on the first choice');
  same([body.find('talk-desks').length, body.find('talk-small').length], [0, 0], 'no desk list or small bubbles under it');
  canvas.dispatch('keydown', { key: '1' });
  canvas.dispatch('keydown', { key: 'N' });
  await settle();
  same([posts().length, pt.find('talk-name')[0].textContent], [before, 'Bo'], 'a digit and N from outside the terminal do nothing');
  // Arrows round its choices: Yes, No, then the four keys
  doc.activeElement.dispatch('keydown', { key: 'ArrowRight' });
  same(focusedText(), '2No', '→ in the terminal');
  doc.activeElement.dispatch('keydown', { key: 'ArrowLeft' });
  doc.activeElement.dispatch('keydown', { key: 'ArrowLeft' });
  same(focusedText(), '✕', '← round to the last key');
  // Tab round it: from its last key or box to its first; Shift+Tab back; from outside, in
  const stops = pt.querySelectorAll('button, textarea').filter((el) => !el.disabled && el.getClientRects().length);
  stops[stops.length - 1].focus();
  same(stops[stops.length - 1].dispatch('keydown', { key: 'Tab' }).defaultPrevented, true, 'Tab at the end taken');
  same(doc.activeElement === stops[0], true, 'round to the first');
  stops[0].dispatch('keydown', { key: 'Tab', shiftKey: true });
  same(doc.activeElement === stops[stops.length - 1], true, 'Shift+Tab round to the last');
  stops[1].focus();
  same(stops[1].dispatch('keydown', { key: 'Tab' }).defaultPrevented, undefined, 'Tab in the middle left to the page');
  canvas.dispatch('keydown', { key: 'Tab' });
  same(doc.activeElement === stops[0], true, 'Tab from outside comes in');
  // Esc in the menu shuts it and puts the focus back on Menu; Esc again goes back to the room
  const menuKey = pt.find('talk-key').find((k) => k.textContent === 'Menu ▾');
  menuKey.dispatch('click');
  same(focusedText(), 'Open in Terminal', 'the menu, its first item focused');
  doc.activeElement.dispatch('keydown', { key: 'Escape' });
  same([pt.find('pt-menu').length, doc.activeElement === menuKey, pt.hidden], [0, true, false], 'Esc shuts the menu, the focus on Menu');
  // N from inside: the next waiting desk
  pt.dispatch('keydown', { key: 'N' });
  await settle();
  same(pt.find('talk-name')[0].textContent !== 'Bo', true, 'N from inside: Next question');
  doc.activeElement.dispatch('keydown', { key: 'Escape' });
  same([pt.hidden, doc.activeElement === canvas], [true, true], 'Esc: back to the room, which has the focus');
  bubble.remove();
  same(body.children.filter((c) => c !== canvas && c.className.split(' ')[0] !== 'pt' && c.className.split(' ')[0] !== 'pt-dim').map((c) => c.className), [], 'nothing left over the page');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}

// Tab in the Pixel Terminal (RT126P7A B1), as Chromium has it (the Windows window's engine, whose conversation is a
// stop of its own as it scrolls): every stop in order, the header, the conversation, the question's choices and the
// four keys, the input (greyed while it asks, still a stop), the office's picture and the desk list; then round to the
// first, never round a few. Shift+Tab the same, backwards. From the conversation and from the terminal itself, too.
func TestThePixelTerminalsTabReachesEveryStop(t *testing.T) {
	runNode(t, talkBubbleSources(t)+talkSame+talkDOM+talkStage+`
const named = (el) => (el === body ? 'the page' : el.className.split(' ').includes('pt-body') ? 'the conversation' : el.tagName === 'TEXTAREA' ? 'the input'
  : el.className.split(' ').includes('pt-desk') ? 'desk ' + el.find('pt-desk-name')[0].textContent : el.attrs['aria-label'] || el.textContent);
const walk = (back, n) => Array.from({ length: n }, () => { press('Tab', back); return named(doc.activeElement); });

(async () => {
  bubble.frame(bubbles);
  await settle();
  bubble.frame(bubbles);
  same(bubble.expand('s2'), true, 'Expand on Bo, asking');
  await tick();
  const pt = term();
  same(focusedText(), '1Yes', 'the focus on its first choice');
  const order = ['Back to office', 'Shrink to bubble', 'Next question 1', 'Menu ▾', 'the conversation', '1Yes', '2No', 'Up', 'Down', 'Pick', 'No',
    'the input', 'Back to office', 'desk Al', 'desk Cy', 'desk Rex'];
  const stops = body.all().filter(stop);
  same(stops.map(named), order, 'the terminal\'s stops in the page, and none outside it');
  // Forward from the first: each stop once, in order, then round to the first, twice over
  stops[0].focus();
  same(walk(false, order.length * 2), [...order.slice(1), ...order, order[0]], 'Tab: every stop, then round');
  // Backward from the last: the same, the other way
  stops[stops.length - 1].focus();
  const back = [...order].reverse();
  same(walk(true, order.length * 2), [...back.slice(1), ...back, back[0]], 'Shift+Tab: every stop, then round');
  // From the conversation: on to the question, or back to Menu
  pt.find('pt-body')[0].focus();
  same(walk(false, 1), ['1Yes'], 'Tab from the conversation: its question');
  pt.find('pt-body')[0].focus();
  same(walk(true, 1), ['Menu ▾'], 'Shift+Tab from the conversation: Menu');
  // From the terminal itself (as it opens): in at the first, or the last
  pt.focus();
  same(walk(false, 1), ['Back to office'], 'Tab from the terminal itself');
  pt.focus();
  same(walk(true, 1), ['desk Rex'], 'Shift+Tab from the terminal itself');
  // From the page behind: in
  body.focus();
  same(walk(false, 1), ['Back to office'], 'Tab from the page comes in');
  same(posts(), [], 'no Tab answered anything');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}

// A prompt arriving while you type (RT126P7A B2), on an engine that drops the focus from a box as it's greyed: the
// focus stays in the input, greyed (read-only); Enter there answers nothing and sends nothing, and a digit picks
// nothing; the words stay. With Send focused instead, the focus goes back to the input, never onto a choice. In the
// bubble, then in the terminal.
func TestAPromptArrivingWhileYouTypeKeepsYouInTheInput(t *testing.T) {
	runNode(t, talkBubbleSources(t)+talkSame+talkDOM+talkStage+`
El.strict = true;
const idle = { state: 'idle', asking: null, size: 40, typing: typingOK };
const ask = (since) => { cards.s1 = { state: 'asking', asking: { ...permission, since }, size: 40, typing: waiting }; };
const answered = () => posts().filter((s) => s[1] === '/talk/pick' || s[1] === '/act' || s[1] === '/talk/say');

(async () => {
  bubble.frame(bubbles);
  await settle();
  // The bubble: Rex, idle, the focus in its input, words being typed
  bubble.open('s1');
  await settle();
  const form = () => panel().find('talk-input')[0], input = () => form().children[0], send = () => form().children[1];
  same(doc.activeElement === input(), true, 'Rex: the focus in its input');
  input().value = 'deploy when ready';
  ask(900);
  await tick();
  same([panel().find('talk-ask')[0].find('talk-key').map((k) => k.textContent), doc.activeElement === input()], [['1Yes', '2No'], true],
    'Rex asks: Yes and No shown, the focus still in the input');
  same([input().readOnly, input().attrs['aria-disabled'], input().value, send().disabled], [true, 'true', 'deploy when ready', true], 'greyed, the words kept');
  press('Enter');
  await settle();
  press('1');
  press('Enter');
  await settle();
  same([answered(), doc.activeElement === input(), input().value], [[], true, 'deploy when ready'], 'Enter and a digit in the box: nothing answered, nothing sent');
  // Send focused as it asks: greyed, it drops the focus, which goes back in the input, not onto Yes
  cards.s1 = idle;
  await tick();
  same([input().readOnly, send().disabled], [false, false], 'it stopped asking: the input awake');
  send().focus();
  ask(901);
  await tick();
  same([doc.activeElement === input(), send().disabled], [true, true], 'Send greyed: the focus back in the input');
  press('Enter');
  await settle();
  same(answered(), [], 'Enter: nothing answered, nothing sent');
  press('Escape');
  await tick();

  // The terminal: Rex, idle, the focus in its input
  cards.s1 = idle;
  same(bubble.expand('s1'), true, 'Expand on Rex');
  await tick();
  const pt = term(), box = pt.find('pt-input')[0].children[0], ptSend = pt.find('pt-input')[0].children[1];
  same(doc.activeElement === box, true, 'the focus in the roomy input');
  box.value = 'deploy when ready';
  ask(902);
  await tick();
  same([pt.find('talk-ask')[0].find('talk-key').length > 0, doc.activeElement === box, box.readOnly, box.value], [true, true, true, 'deploy when ready'],
    'Rex asks: the focus still in the input, greyed, the words kept');
  press('Enter');
  press('2');
  press('Enter');
  await settle();
  same([answered(), doc.activeElement === box, box.value], [[], true, 'deploy when ready'], 'Enter and a digit in the box: nothing answered, nothing sent');
  cards.s1 = idle;
  await tick();
  ptSend.focus();
  ask(903);
  await tick();
  same(doc.activeElement === box, true, 'Send greyed: the focus back in the input');
  press('Enter');
  await settle();
  same(answered(), [], 'Enter: nothing answered, nothing sent');
  // Once it stops asking, the words go as typed
  cards.s1 = idle;
  await tick();
  press('Enter');
  await settle();
  same(answered().map((s) => [s[1], s[3].text]), [['/talk/say', 'deploy when ready']], 'it stopped asking: Enter sends the words');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}

// Send with the focus as it's greyed, on an engine that lets go of it only later, onto the page, as Chromium does
// (RT126P7AR2 B2b), there the room's Enter giving the open bubble the focus on Yes: the focus goes to the input first,
// so Enter, Enter answers nothing and sends nothing. Tab to Send, then a prompt arrives; Send clicked, words on their
// way, then a prompt arrives. In the bubble, then in the terminal; the engine letting go after a microtask, then a timer.
func TestSendGreyedWithTheFocusLeavesItInTheInputOnAnyEngine(t *testing.T) {
	for _, late := range []string{"microtask", "timer"} {
		t.Run(late, func(t *testing.T) {
			runNode(t, talkBubbleSources(t)+talkSame+talkDOM+talkStage+`
El.late = '`+late+`';
const idle = { state: 'idle', asking: null, size: 40, typing: typingOK };
const ask = (since) => { cards.s1 = { state: 'asking', asking: { ...permission, since }, size: 40, typing: waiting }; };
const answered = () => posts().filter((s) => s[1] === '/talk/pick' || s[1] === '/act' || s[1] === '/talk/say');
// main.js's room rule: Enter on the room or the page gives the picked desk's open bubble the focus, on its first choice
doc.addEventListener('keydown', (e) => { if ((e.target === body || e.target === canvas) && e.key === 'Enter' && bubble.enter('s1')) e.preventDefault(); });
// A click on Send, as Chromium has it: the focus on it, then its form sent
const click = (key) => { key.focus(); key.dispatch('click'); key.parentNode.dispatch('submit'); };
let release = null;
const holding = () => { hold = new Promise((r) => { release = () => { hold = null; r(); }; }); };

(async () => {
  bubble.frame(bubbles);
  await settle();
  // The bubble: Rex, idle, words typed, Tab to Send; Rex asks
  bubble.open('s1');
  await settle();
  const form = () => panel().find('talk-input')[0], input = () => form().children[0], send = () => form().children[1];
  input().value = 'deploy when ready';
  press('Tab');
  same(doc.activeElement === send(), true, 'Tab from the input: on Send');
  ask(910);
  await tick();
  await settle();
  same([doc.activeElement === input(), send().disabled, input().readOnly], [true, true, true], 'Rex asks with Send focused: the focus in the input, greyed');
  press('Enter');
  await settle();
  press('Enter');
  await settle();
  same([answered(), doc.activeElement === input(), input().value], [[], true, 'deploy when ready'], 'Enter, Enter: nothing answered, nothing sent');
  // Send clicked: the words on their way, Send greyed; the focus in the input, and a prompt arriving meanwhile answers nothing
  cards.s1 = idle;
  await tick();
  holding();
  click(send());
  await settle();
  same([doc.activeElement === input(), send().disabled], [true, true], 'Send clicked, the words on their way: the focus in the input');
  ask(911);
  await tick();
  await settle();
  press('Enter');
  await settle();
  press('Enter');
  await settle();
  same([answered().map((s) => s[1]), doc.activeElement === input()], [['/talk/say'], true], 'then Rex asks, Enter, Enter: the words sent once, nothing answered');
  release();
  await settle();
  press('Escape');
  await tick();

  // The terminal: Rex, idle, words typed, Tab to Send; Rex asks
  sent.length = 0;
  cards.s1 = idle;
  same(bubble.expand('s1'), true, 'Expand on Rex');
  await tick();
  const pt = term(), box = pt.find('pt-input')[0].children[0], ptSend = pt.find('pt-input')[0].children[1];
  box.value = 'deploy when ready';
  press('Tab');
  same(doc.activeElement === ptSend, true, 'Tab from the roomy input: on Send');
  ask(912);
  await tick();
  await settle();
  same([doc.activeElement === box, ptSend.disabled, box.readOnly], [true, true, true], 'Rex asks with Send focused: the focus in the input, greyed');
  press('Enter');
  await settle();
  press('Enter');
  await settle();
  same([answered(), doc.activeElement === box, box.value], [[], true, 'deploy when ready'], 'Enter, Enter: nothing answered, nothing sent');
  // Send clicked, then Rex asks
  cards.s1 = idle;
  await tick();
  holding();
  click(ptSend);
  await settle();
  same([doc.activeElement === box, ptSend.disabled], [true, true], 'Send clicked, the words on their way: the focus in the input');
  ask(913);
  await tick();
  await settle();
  press('Enter');
  await settle();
  press('Enter');
  await settle();
  same([answered().map((s) => s[1]), doc.activeElement === box], [['/talk/say'], true], 'then Rex asks, Enter, Enter: the words sent once, nothing answered');
  release();
  await settle();
})().catch((e) => { console.error(e); process.exit(1); });
`)
		})
	}
}

// The room itself: an idle head draws no bubble and so is no click target, yet its desk is in the desk list; Enter
// with the room's focus opens the picked desk's bubble (an idle one's too), never from a key that has the focus; Tab
// from the room goes to the small question bubbles; without the page's network, Enter and Tab are the page's own.
func TestTheRoomsEnterAndTabOpenTheTalkBubbles(t *testing.T) {
	path, err := filepath.Abs("../Resources/web/office.js")
	if err != nil {
		t.Fatal(err)
	}
	office := string(officeSource(t))
	for _, want := range []string{"if (!bubble.quiet) consider({ kind: 'talk'", "talkPanels.enter(selected)", "talkPanels.tab(e.shiftKey)"} {
		if !strings.Contains(office, want) {
			t.Fatalf("office.js lost %q: update this test", want)
		}
	}
	runNode(t, officeRig+talkDOM+`
dressDoc(win.document, spare);
const res = (status, body) => ({ status, ok: status >= 200 && status < 300, json: async () => body, text: async () => String(body || '') });
const settle = () => new Promise((r) => setTimeout(r, 5));
function talkRig(sessions, network) {
  const handlers = {}, calls = [];
  const canvas = Object.assign(new El('canvas'), {
    ownerDocument: win.document, width: 640, height: 360, getContext: context,
    getBoundingClientRect: () => ({ left: 0, top: 0, right: 640, bottom: 360, width: 640, height: 360 }),
    addEventListener: (type, f) => { (handlers[type] = handlers[type] || []).push(f); },
    setPointerCapture() {}, releasePointerCapture() {},
  });
  win.document.body.append(canvas);
  win.document.defaultView = win;
  const cards = Object.fromEntries(sessions.map((s) => [s.id, { state: s.state, asking: s.state === 'asking' ? { since: 5, kind: 'permission', label: 'Bash: make', questions: [], pickable: true } : null,
    size: 1, typing: { ok: true, lines: true, why: '' } }]));
  const talk = { open: (id) => calls.push(['open', id]), hover: (id) => calls.push(['hover', id]) };
  if (network) Object.assign(talk, {
    get: async (path, params) => (path === '/talk' ? res(200, cards[params.session]) : path === '/conversation' ? res(200, { items: [], before: null, size: 1 })
      : res(200, { asking: Object.keys(cards).filter((id) => cards[id].asking).map((id) => ({ session: id, ...cards[id].asking })) })),
    post: async () => res(204), flash() {}, tick() {},
  });
  const office = win.AgentOffice.create(canvas, { select: (id) => calls.push(['select', id]), hint() {}, held() {}, talk });
  office.show({ now: Date.now(), keys: sessions.map((s) => ({ session: s })), layout: {}, hiring: { fire: false, hire: false } });
  office.setVisible(true);
  const step = (n) => {
    for (let i = 0; i < n; i++) {
      T += 100;
      if (win.raf) { const f = win.raf; win.raf = null; f(T); }
      const due = timers; timers = [];
      due.forEach((f) => f());
    }
  };
  step(200);
  return { office, canvas, calls, step, send(type, x, y) { (handlers[type] || []).forEach((f) => f({ clientX: x, clientY: y, pointerId: 1, pointerType: 'mouse', preventDefault() {} })); } };
}
const open = () => win.document.body.find('talk-bubble').map((b) => b.attrs['aria-label']);

(async () => {
  const r = talkRig([session('a', { state: 'asking', name: 'Ann' }), session('i', { state: 'idle', name: 'Ida' }), session('b', { name: 'Bob' })], true);
  await settle();
  r.step(3);
  await settle();
  r.step(3);
  // An idle head: no click target (nothing hovers over it), yet its desk is in the desk list
  const desks = () => win.document.body.find('talk-desks')[0];
  need(desks() && desks().children.some((k) => k.textContent === 'Ida, idle'), 'the idle desk is not in the desk list: ' + (desks() ? desks().textContent : 'no list'));
  let hovered = false;
  for (let y = 0; y < 360 && !hovered; y += 3) for (let x = 0; x < 640; x += 3) {
    r.send('pointermove', x, y);
    if (r.canvas.style.cursor === 'pointer') { r.step(3); hovered = hovered || r.calls.some((c) => c[0] === 'hover' && c[1] === 'i'); }
  }
  r.send('pointerleave');
  need(!hovered, 'the empty air over an idle head is a talk target');
  // Enter from a key that has the focus: that key's, not the room's
  r.office.select('i');
  const elsewhere = new El('button');
  win.document.body.append(elsewhere);
  elsewhere.focus();
  need(elsewhere.dispatch('keydown', { key: 'Enter' }).defaultPrevented !== true && open().length === 0, 'Enter on a key opened a bubble');
  // Enter with the room's focus: the picked desk's bubble, an idle one's too
  r.canvas.focus();
  need(r.canvas.dispatch('keydown', { key: 'Enter' }).defaultPrevented === true, 'Enter in the room wasn\'t taken');
  await settle();
  need(open().join() === 'Ida: conversation', 'Enter opened ' + JSON.stringify(open()));
  r.canvas.focus();
  r.canvas.dispatch('keydown', { key: 'Escape' });
  r.step(2);
  // Tab from the room: Ann's small question bubble
  r.canvas.focus();
  need(r.canvas.dispatch('keydown', { key: 'Tab' }).defaultPrevented === true, 'Tab in the room wasn\'t taken');
  need(win.document.activeElement.attrs['aria-label'] === 'Ann is asking: Allow Bash: make?', 'Tab went to ' + win.document.activeElement.tagName);
  // A key or modifier held, the page's own
  r.canvas.focus();
  need(r.canvas.dispatch('keydown', { key: 'Enter', metaKey: true }).defaultPrevented !== true, 'Cmd+Enter taken');
  r.office.close();

  // Without the page's network (a phone, the portable viewer): no talk panels, Enter and Tab untouched
  win.document.body.replaceChildren();
  const plain = talkRig([session('a', { state: 'asking' }), session('i', { state: 'idle' })], false);
  plain.office.select('i');
  plain.canvas.focus();
  need(plain.canvas.dispatch('keydown', { key: 'Enter' }).defaultPrevented !== true, 'Enter taken without the network');
  need(plain.canvas.dispatch('keydown', { key: 'Tab' }).defaultPrevented !== true, 'Tab taken without the network');
  need(win.document.body.find('talk-desks').length === 0, 'a desk list without the network');
})().catch((e) => { console.error(e); process.exit(1); });
`, path)
}
