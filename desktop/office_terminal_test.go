package desktop

import (
	"os"
	"strings"
	"testing"
)

// The Pixel Terminal (1.26, P6): talk/terminal.js in node, its plain functions and the live terminal on a stand-in for
// the page; the office's extension seam (F-O); deck.html's ▤ key. Skipped without node.

// A stand-in page for the live bubble and terminal (after talkSame, whose item, permission and typingOK it uses): elements that bubble their events up to the document, a canvas
// for the room and the office's picture, the computer's answers per desk, and every request kept in `sent`.
const terminalStandIn = `
class El {
  constructor(tag) { Object.assign(this, { tagName: tag.toUpperCase(), children: [], style: { setProperty() {} }, className: '', hidden: false,
    disabled: false, value: '', placeholder: '', listeners: {}, attrs: {}, parent: null, offsetWidth: 100, offsetHeight: 20, scrollTop: 0, clientHeight: 60 }); }
  get scrollHeight() { return this.children.length * 30; }
  get classList() { const el = this; return { add(c) { el.className += ' ' + c; }, remove(c) { el.className = el.className.split(' ').filter((x) => x !== c).join(' '); } }; }
  set textContent(t) { this.children = [String(t)]; }
  get textContent() { return this.children.map((c) => (typeof c === 'string' ? c : c.textContent)).join(''); }
  append(...kids) { for (const k of kids) { if (typeof k !== 'string') k.parent = this; this.children.push(k); } }
  replaceChildren(...kids) { this.children = []; this.append(...kids); }
  remove() { if (this.parent) this.parent.children = this.parent.children.filter((c) => c !== this); }
  setAttribute(k, v) { this.attrs[k] = v; }
  addEventListener(type, f) { (this.listeners[type] = this.listeners[type] || []).push(f); }
  dispatch(type, e = {}) {
    const ev = { target: this, preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; }, ...e };
    for (let el = this; el && !ev.stopped; el = el.parent) (el.listeners[type] || []).forEach((f) => f(ev));
    if (!ev.stopped) (docListeners[type] || []).forEach((f) => f(ev));
    return ev;
  }
  all() { return this.children.filter((c) => typeof c !== 'string').flatMap((c) => [c, ...c.all()]); }
  querySelector(sel) { return this.all().find((c) => c.className.split(' ').includes(sel.slice(1))) || null; }
  find(cls) { return this.all().filter((c) => c.className.split(' ').includes(cls) && !c.hidden); }
  focus() { focused = this; }
  getContext() { return this.tagName === 'CANVAS' ? { drawImage: (from) => drawn.push(from) } : null; }
}
let focused = null;
const drawn = [], docListeners = {};
const body = new El('body'), head = new El('head');
const doc = { body, head, createElement: (t) => new El(t), getElementById: (id) => head.all().find((c) => c.id === id) || null,
  addEventListener: (type, f) => (docListeners[type] = docListeners[type] || []).push(f),
  removeEventListener: (type, f) => { docListeners[type] = (docListeners[type] || []).filter((g) => g !== f); } };
const timers = [];
const win = { document: doc, innerHeight: 800, setTimeout: (f) => timers.push(f), clearTimeout() {} };
const settle = () => new Promise((r) => setTimeout(r, 5));
const res = (status, body) => ({ status, ok: status >= 200 && status < 300, json: async () => body, text: async () => String(body || '') });
// Four desks: Rex (s1) idle with a long conversation, Bo (s2) asking since 300, Al (s3) asking since 100, Cy (s4) working
const names = { s1: 'Rex', s2: 'Bo', s3: 'Al', s4: 'Cy' };
const cards = { s1: { state: 'idle', asking: null, last: 'Done', size: 40, typing: typingOK },
  s2: { state: 'asking', asking: permission, last: '', size: 10, typing: { ok: false, lines: false, why: 'answer first' } },
  s3: { state: 'asking', asking: { ...permission, since: 100 }, last: '', size: 10, typing: { ok: false, lines: false, why: 'answer first' } },
  s4: { state: 'thinking', asking: null, last: '', size: 10, typing: typingOK } };
const bubbles = [{ id: 's1', state: 'idle', x: 40, top: 50 }, { id: 's2', state: 'asking', since: 300, x: 90, top: 50 },
  { id: 's3', state: 'asking', since: 100, x: 140, top: 50 }, { id: 's4', state: 'thinking', x: 40, top: 100 }];
const sent = [];
let refuse = null;   // the next /talk/say's refusal, if any
let hold = null;     // while set, a promise each /talk/say waits on, as a slow computer would
const page = {
  get: async (path, params) => {
    sent.push(['get', path, params]);
    if (path === '/talk') return res(200, cards[params.session]);
    if (path === '/conversation' && params.before === 100) return res(200, { items: [item(0, 'user', 'start here')], before: null, size: 40 });
    if (path === '/conversation') return res(200, { items: [item(1, 'user', 'build it'), item(2, 'agent', '<b>built</b>')], before: params.session === 's1' ? 100 : null, size: 40 });
    return res(200, { asking: [] });
  },
  post: async (path, params, signal, json) => {
    sent.push(['post', path, params, json]);
    if (path === '/talk/say' && hold) await hold;
    if (path === '/talk/say' && refuse) { const r = res(409, refuse); refuse = null; return r; }
    return res(204);
  },
  flash() {}, tick() {},
};
const canvas = { width: 640, height: 360, style: {}, focus() {}, getBoundingClientRect: () => ({ left: 0, top: 0, right: 640, bottom: 360, width: 640, height: 360 }) };
const anchor = { id: 's1', state: 'idle', since: 1, art: {}, css: { box: { left: 290, right: 314, top: 100, bottom: 124 }, tip: { x: 300, y: 122 } } };
const room = { canvas, anchor: () => anchor, view: () => ({ scale: 2, ox: 0, oy: 0, k: 1, left: 0, top: 0 }), cog: () => null,
  shape: () => ({ width: 320, height: 180 }), session: (id) => (names[id] ? { id, state: cards[id].state, conversation: true, project: 'shop' } : null),
  name: (id) => names[id] || 'Agent' };
const T = PIXEL_TERMINAL;
const bubble = B.create(win, page, room);
const bubblePanel = () => body.children.find((c) => c.className && c.className.includes('talk-bubble'));
const term = () => body.children.find((c) => c.className && c.className.split(' ')[0] === 'pt');
const keyNamed = (el, text) => el.all().find((c) => c.className.includes('talk-key') && c.textContent === text && !c.hidden);
// What's in sight on the page: each element's class and its own text, in order
const visible = (el) => el.children.filter((c) => typeof c !== 'string' && !c.hidden).flatMap((c) => [c, ...visible(c)]);
const outline = () => visible(body).map((c) => c.className + (c.children.some((k) => typeof k === 'string') ? ':' + c.children.filter((k) => typeof k === 'string').join('') : ''));
const expand = async () => { bubble.open('s1'); await settle(); keyNamed(bubblePanel(), 'Expand').dispatch('click'); await settle(); timers.splice(0).forEach((f) => f()); await settle(); };
`

// Next question's order and count (longest waiting first, round to the first, greyed with none), the desk list (waiting
// first, then by name), the scroll-back merge (the slate's), Enter and Shift+Enter, and the zoom's point.
func TestThePixelTerminalsPlainFunctions(t *testing.T) {
	runNode(t, talkBubbleSources(t)+talkSame+`
const T = PIXEL_TERMINAL;
same([T.nextOf(['a', 'b', 'c'], 'a'), T.nextOf(['a', 'b', 'c'], 'c'), T.nextOf(['a', 'b', 'c'], 'x'), T.nextOf(['a'], 'a'), T.nextOf([], 'x')],
  [{ count: 2, next: 'b' }, { count: 2, next: 'a' }, { count: 3, next: 'a' }, { count: 0, next: null }, { count: 0, next: null }], 'Next question');
const desks = [{ id: 's1', state: 'idle' }, { id: 's2', state: 'asking', since: 300 }, { id: 's3', state: 'asking', since: 100 },
  { id: 's4', state: 'thinking' }, { id: 's5', state: 'asking' }];
const names = { s1: 'Zed', s2: 'Bo', s3: 'Al', s4: 'Cy', s5: 'Di' };
same(TALK_ANCHORS.needsYou(desks).map((d) => d.id), ['s3', 's2', 's5'], 'the order Next walks: longest waiting first, unknown last');
same(T.listOf(desks, 's2', (id) => names[id]).map((d) => [d.id, d.name, d.state]),
  [['s3', 'Al', 'asking'], ['s5', 'Di', 'asking'], ['s4', 'Cy', 'thinking'], ['s1', 'Zed', 'idle']], 'the desk list: waiting first, then by name');
// Scroll-back: an older page goes above what's shown, nothing twice, and says where the page before it starts
let shown = { items: [item(5, 'agent', 'e'), item(6, 'user', 'f')], before: 400 };
shown = T.older(shown, { items: [item(3, 'user', 'c'), item(4, 'agent', 'd'), item(5, 'agent', 'e')], before: 200 });
same([shown.items.map((i) => i.text), shown.before], [['c', 'd', 'e', 'f'], 200], 'an older page above');
shown = T.older(shown, { items: [item(1, 'user', 'a'), item(2, 'agent', 'b')], before: null });
same([shown.items.map((i) => i.text), shown.before], [['a', 'b', 'c', 'd', 'e', 'f'], null], 'back to the start');
same([T.keyOf({ key: 'Enter' }), T.keyOf({ key: 'Enter', shiftKey: true }), T.keyOf({ key: 'Enter', isComposing: true }),
  T.keyOf({ key: 'Enter', keyCode: 229 }), T.keyOf({ key: 'a' })], ['send', null, null, null, null], 'Enter sends, Shift+Enter a new line');
same(T.zoomOf({ x: 110, y: 70 }, { left: 10, top: 20, width: 640, height: 360 }, false), { origin: '100px 50px', scale: 1.6 }, 'toward the head');
same(T.zoomOf(null, { left: 0, top: 0, width: 640, height: 360 }, false).origin, '320px 180px', 'or the middle');
same(T.zoomOf({ x: 1, y: 1 }, { left: 0, top: 0, width: 1, height: 1 }, true), null, 'a cross-fade alone with reduced motion');
`)
}

// The live terminal on a stand-in page: Expand opens it at the same place in the conversation (no page read again),
// the room zoomed and the small bubbles gone; Enter sends several lines as one, Shift+Enter doesn't, a refusal keeps
// the words; N outside the box goes to the longest waiting desk, a digit picks there and No wakes the input; the desk
// list switches desks and keeps each one's words; Older messages comes in above, where you were kept; Shrink opens the
// same desk's bubble; Esc goes back; Open in Terminal shows only where the page offers it.
func TestTheLivePixelTerminal(t *testing.T) {
	runNode(t, talkBubbleSources(t)+talkSame+terminalStandIn+`
(async () => {
  bubble.frame(bubbles);
  await settle();
  const opened = [];
  let offered = false;
  page.openInTerminal = { shown: () => offered, open: (id) => opened.push(id) };
  bubble.open('s1');
  await settle();
  const n0 = sent.length;
  keyNamed(bubblePanel(), 'Expand').dispatch('click');
  await settle();
  timers.splice(0).forEach((f) => f());
  const pt = term();
  same([pt.hidden, pt.attrs.role, pt.attrs['aria-label'], bubblePanel()], [false, 'dialog', 'Rex: conversation', undefined], 'Expand: the terminal, the bubble gone');
  same(sent.slice(n0).map((s) => s[1]).filter((p) => p !== '/talk'), [], 'the same place in the conversation: nothing read again');
  same(pt.find('pt-body')[0].find('board-row').map((r) => r.textContent), ['build it', '<b>built</b>'], 'its rows, as text');
  same([canvas.style.transform, canvas.style.transformOrigin], ['scale(1.6)', '300px 122px'], 'the room zoomed toward the desk');
  same([body.find('talk-small').length, body.find('talk-needs').length], [0, 0], 'no small bubble or Needs you over it');
  bubble.frame(bubbles);
  same([body.find('talk-small').length, drawn.length > 0], [0, true], 'still none, and the office\'s picture drawn');
  same([keyNamed(pt, 'Next question 2') !== undefined, keyNamed(pt, 'Menu ▾')], [true, undefined], 'Next question 2; no menu with nothing in it');

  // The roomy input: Shift+Enter is a new line; Enter sends every line as one; refused, the words stay with the reason.
  const input = pt.find('pt-input')[0].children[0];
  same([input.tagName, input.readOnly, input.placeholder], ['TEXTAREA', false, 'Tell Rex what to do (Shift+Enter: a new line)'], 'the roomy input');
  input.value = 'first line\nsecond line';
  let n = sent.length;
  same(input.dispatch('keydown', { key: 'Enter', shiftKey: true }).defaultPrevented, undefined, 'Shift+Enter left to the box');
  await settle();
  same(sent.slice(n), [], 'Shift+Enter sent nothing');
  input.dispatch('keydown', { key: 'Enter' });
  await settle();
  same(sent[n], ['post', '/talk/say', { session: 's1' }, { text: 'first line\nsecond line', send: true }], 'Enter sends the lines as one');
  same(input.value, '', 'emptied once it went');
  refuse = 'WezTerm takes one line at a time.';
  input.value = 'one\ntwo';
  input.dispatch('keydown', { key: 'Enter' });
  await settle();
  same([input.value, pt.find('talk-problem').map((p) => p.textContent)], ['one\ntwo', ['WezTerm takes one line at a time.']], 'refused: the words kept, the reason under them');

  // N in the box is a letter; outside it, Next question: Al, the longest waiting.
  input.dispatch('keydown', { key: 'n' });
  same(pt.find('talk-name')[0].textContent, 'Rex', 'N typed in the box');
  pt.dispatch('keydown', { key: 'N' });
  await settle();
  same([pt.find('talk-name')[0].textContent, input.value, input.readOnly, input.placeholder], ['Al', '', true, 'Answer above first'], 'Next: Al, its input greyed');
  same(keyNamed(pt, 'Next question 1') !== undefined, true, 'one more waiting');
  same(pt.find('talk-ask')[0].find('talk-key').map((k) => k.textContent), ['1Yes', '2No', '▲', '▼', '✓', '✕'], 'Yes and No, and the four keys');
  // A digit in the box picks nothing; outside it, it picks; No wakes the input.
  n = sent.length;
  input.dispatch('keydown', { key: '2' });
  await settle();
  same(sent.slice(n).filter((s) => s[0] === 'post'), [], 'a digit in the box');
  cards.s3 = { ...cards.s3, state: 'idle', asking: null, typing: typingOK };
  pt.dispatch('keydown', { key: '2' });
  await settle();
  same(sent.slice(n).find((s) => s[0] === 'post'), ['post', '/talk/pick', { session: 's3', n: '2', since: '100' }, undefined], 'No by its digit');
  same([input.readOnly, focused === input], [false, true], 'woken after No');

  // The desk list: waiting first (Bo), then by name; a click switches, and each desk keeps its own words.
  same(pt.find('pt-list')[0].find('pt-desk-name').map((d) => d.textContent), ['Bo', 'Cy', 'Rex'], 'the other desks, waiting first');
  input.value = 'half a thought';
  pt.find('pt-list')[0].find('pt-desk').find((d) => d.textContent.includes('Rex')).dispatch('click');
  await settle();
  same([pt.find('talk-name')[0].textContent, input.value], ['Rex', 'one\ntwo'], 'Rex again, its words back');
  pt.find('pt-list')[0].find('pt-desk').find((d) => d.textContent.includes('Al')).dispatch('click');
  await settle();
  same(input.value, 'half a thought', 'Al\'s words kept for Al');
  pt.find('pt-list')[0].find('pt-desk').find((d) => d.textContent.includes('Rex')).dispatch('click');
  await settle();

  // Older messages: the page before, above, where you were kept in sight; then the start.
  const lines = pt.find('pt-body')[0];
  lines.scrollTop = 20;
  const fromFoot = lines.scrollHeight - lines.scrollTop;
  keyNamed(lines, 'Older messages').dispatch('click');
  await settle();
  same(sent.filter((s) => s[1] === '/conversation' && s[2].before === 100).length, 1, 'the page before asked for');
  same([lines.find('board-row').map((r) => r.textContent), lines.scrollHeight - lines.scrollTop, lines.children[0].textContent],
    [['start here', 'build it', '<b>built</b>'], fromFoot, 'The start of the conversation'], 'older above, the place kept');

  // Open in Terminal: only where the page offers it.
  offered = true;
  timers.splice(0).forEach((f) => f());
  await settle();
  keyNamed(pt, 'Menu ▾').dispatch('click');
  keyNamed(pt.find('pt-menu')[0], 'Open in Terminal').dispatch('click');
  same([opened, pt.find('pt-menu').length], [['s1'], 0], 'Open in Terminal, the menu shut');

  // Shrink: the office, the same desk's bubble open; Esc and Back to office close it, the room as it was.
  keyNamed(pt, 'Shrink to bubble').dispatch('click');
  await settle();
  same([pt.hidden, canvas.style.transform, bubblePanel() && bubblePanel().attrs['aria-label']], [true, '', 'Rex: conversation'], 'Shrink back to the same bubble');
  keyNamed(bubblePanel(), 'Expand').dispatch('click');
  await settle();
  same(pt.hidden, false, 'Expand again');
  body.dispatch('keydown', { key: 'Escape' });
  same([pt.hidden, canvas.style.transform, focused === null || focused.className !== 'pt'], [true, '', true], 'Esc: back to the office');
  await expand();
  keyNamed(pt, 'Back to office').dispatch('click');
  same(pt.hidden, true, 'Back to office');
  bubble.frame(bubbles);
  same(body.find('talk-needs').length, 1, 'Needs you back with the office');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}

// Enter sends once (RT126P6 B1): a second Enter, a held Enter's repeats or Send while the words are on their way post
// nothing more, and the words come back only if refused; what's typed meanwhile stays in the box, and what went isn't
// kept as a draft for a desk switched away from. And: Esc while a letter is composed is the letter's; an added key
// presses as it's registered now; a feature gets a copy of the session.
func TestThePixelTerminalSendsOnce(t *testing.T) {
	runNode(t, talkBubbleSources(t)+talkSame+terminalStandIn+`
(async () => {
  bubble.frame(bubbles);
  await settle();
  await expand();
  const pt = term(), input = pt.find('pt-input')[0].children[0], send = keyNamed(pt, 'Send');
  const says = (from) => sent.slice(from).filter((s) => s[1] === '/talk/say').length;
  let release;
  const slow = () => { hold = new Promise((r) => { release = r; }); };

  // Two Enters, then Send, while the first is on its way: one POST
  slow();
  input.value = 'run the tests';
  let n = sent.length;
  input.dispatch('keydown', { key: 'Enter' });
  input.dispatch('keydown', { key: 'Enter' });
  await settle();
  pt.find('pt-input')[0].dispatch('submit');
  await settle();
  same(says(n), 1, 'two Enters and Send: one POST');
  input.value += ' then lint';
  hold = null;
  release();
  await settle();
  same([says(n), input.value], [1, 'then lint'], 'still one; what was typed meanwhile stays');

  // A held Enter: its repeats send nothing, and add no new lines
  slow();
  input.value = 'ship it';
  n = sent.length;
  input.dispatch('keydown', { key: 'Enter' });
  const repeats = [1, 2, 3].map(() => input.dispatch('keydown', { key: 'Enter', repeat: true }).defaultPrevented);
  hold = null;
  release();
  await settle();
  same(input.value, '', 'ship it went');
  input.value = 'still held';
  input.dispatch('keydown', { key: 'Enter', repeat: true });
  await settle();
  same([says(n), repeats, input.value], [1, [true, true, true], 'still held'], 'a held Enter: one POST, even once the first is back');

  // Sent again once it's back
  input.value = 'and again';
  n = sent.length;
  input.dispatch('keydown', { key: 'Enter' });
  await settle();
  same([says(n), input.value], [1, ''], 'the next Enter sends');

  // Switched to another desk while the words were on their way: they aren't Rex's draft once they went
  slow();
  input.value = 'from Rex';
  input.dispatch('keydown', { key: 'Enter' });
  pt.find('pt-list')[0].find('pt-desk').find((d) => d.textContent.includes('Cy')).dispatch('click');
  await settle();
  same(input.value, '', 'Cy\'s own box');
  hold = null;
  release();
  await settle();
  pt.find('pt-list')[0].find('pt-desk').find((d) => d.textContent.includes('Rex')).dispatch('click');
  await settle();
  same([input.value, send.disabled], ['', false], 'back on Rex: what went isn\'t a draft, and Send is there');

  // Esc while a letter is composed is the letter's; then it goes back to the office
  input.dispatch('keydown', { key: 'Escape', isComposing: true });
  same(pt.hidden, false, 'Esc composing a letter');
  input.dispatch('keydown', { key: 'Escape' });
  same(pt.hidden, true, 'Esc');

  // An added key presses as it's registered now, on a bubble already open
  const pressed = [];
  B.addKey('again', { label: 'Again', press: () => pressed.push('first') });
  bubble.open('s1');
  await settle();
  B.addKey('again', { label: 'Again', press: () => pressed.push('second') });
  keyNamed(bubblePanel(), 'Again').dispatch('click');
  same(pressed, ['second'], 'the key as it\'s registered now');

  // A feature gets a copy of the session, not the room's own
  const own = { id: 'x', state: 'idle' }, desk = B.deskOf(page, { session: () => own, name: () => 'X' }, 'x', null);
  desk.session.state = 'gone';
  same([own.state, desk.session.id, B.deskOf(page, { session: () => null, name: () => 'X' }, 'y', null).session], ['idle', 'x', null], 'a copy of the session');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}

// The extension seam (F-O): a feature in a file of its own registers a panel and a key as its const is made; the
// panel draws under the conversation for each desk shown, the key shows on the bubble and in the terminal's menu and
// presses for that desk; and nothing else on the page, nor what it asks the computer, changes.
func TestTheOfficeExtensionSeam(t *testing.T) {
	// A feature's file, as talk/<name>.js would hold it: one const, closed by "};", that registers itself.
	feature := `
const TALK_TEST = {
  pressed: [],
  draw(box, desk) { box.textContent = 'Changes for ' + desk.name; },
  panel: PIXEL_TERMINAL.addPanel('test', (box, desk) => TALK_TEST.draw(box, desk)),
  key: TALK_BUBBLE.addKey('test', { label: 'Test', title: 'A test key', shown: (desk) => desk.id !== 's4', press: (desk) => TALK_TEST.pressed.push(desk.id) }),
};
`
	if !strings.HasPrefix(feature, "\nconst TALK_TEST = {\n") || !strings.HasSuffix(feature, "\n};\n") {
		t.Fatal("the feature isn't shaped as buildoffice includes a file")
	}
	runNode(t, talkBubbleSources(t)+talkSame+terminalStandIn+`
const feature = process.argv[1];
(async () => {
  bubble.frame(bubbles);
  await settle();
  // The page and its requests, with nothing added: the bubble, then the terminal on two desks
  const visit = async () => {
    const from = sent.length, seen = [];
    await expand();
    seen.push(outline());
    term().find('pt-desk').find((d) => d.textContent.includes('Cy')).dispatch('click');
    await settle();
    seen.push(outline());
    keyNamed(term(), 'Back to office').dispatch('click');
    bubble.open('s1');
    await settle();
    seen.push(outline());
    bubble.open('s1');
    timers.splice(0).forEach((f) => f());
    return { seen, asked: sent.slice(from).map((s) => s[1] + ' ' + JSON.stringify(s[2])) };
  };
  const before = await visit();
  same([T.panels.size, B.added.size], [0, 0], 'nothing added yet');
  new Function('PIXEL_TERMINAL', 'TALK_BUBBLE', feature + '; globalThis.TALK_TEST = TALK_TEST;')(PIXEL_TERMINAL, TALK_BUBBLE);
  same([[...T.panels.keys()], [...B.added.keys()], TALK_TEST.panel, TALK_TEST.key], [['test'], ['test'], 'test', 'test'], 'registered as its const is made');
  const after = await visit();
  same(after.asked, before.asked, 'the computer asked the same');
  // Each page, less what was added, is what it was
  const added = (row) => /^pt-panel(:|$)/.test(row) || row === 'talk-key dark:Test' || row === 'talk-key dark:Menu ▾';
  for (const [i, seen] of after.seen.entries()) same(seen.filter((row) => !added(row)), before.seen[i].filter((row) => !added(row)), 'nothing else changed (' + i + ')');
  same(after.seen.map((seen) => seen.filter(added)), [['talk-key dark:Menu ▾', 'pt-panel:Changes for Rex'], ['pt-panel:Changes for Cy'], ['talk-key dark:Test']],
    'the panel drawn for each desk, the menu on Rex (not Cy, whose key isn\'t shown), the key on the bubble');
  // The key presses for its desk, on the bubble and in the terminal's menu
  bubble.open('s1');
  await settle();
  keyNamed(bubblePanel(), 'Test').dispatch('click');
  bubble.open('s1');
  timers.splice(0).forEach((f) => f());
  await expand();
  keyNamed(term(), 'Menu ▾').dispatch('click');
  keyNamed(term().find('pt-menu')[0], 'Test').dispatch('click');
  same(TALK_TEST.pressed, ['s1', 's1'], 'pressed for its desk, on the bubble and in the menu');
  // A panel that throws doesn't take the terminal with it
  T.addPanel('test', () => { throw new Error('broken panel'); });
  console.error = () => {};
  term().find('pt-desk').find((d) => d.textContent.includes('Bo')).dispatch('click');
  await settle();
  same(term().find('talk-name')[0].textContent, 'Bo', 'a broken panel, the terminal still goes');
})().catch((e) => { console.error(e); process.exit(1); });
`, feature)
}

// Text is put on the page as text, never as HTML; and in the office's own window the ▤ key opens the Pixel Terminal
// (the slate where the room has no talk bubble, or for a T3 Code desk).
func TestThePixelTerminalWritesTextAndTheConversationKeyOpensIt(t *testing.T) {
	for _, name := range []string{"terminal.js", "bubble.js"} {
		src, err := os.ReadFile("../Resources/web/office/talk/" + name)
		if err != nil {
			t.Fatal(err)
		}
		for _, html := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "createContextualFragment"} {
			if strings.Contains(string(src), html) {
				t.Errorf("%s uses %s: text goes on the page as text", name, html)
			}
		}
	}
	page, err := os.ReadFile("../Resources/web/deck.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "if (bare && action === `button:${BOARD_BUTTON}`) return (talkHere && office.expand?.(selected)) || openBoard(selected);") {
		t.Error("deck.html's ▤ key doesn't open the Pixel Terminal in the office's own window")
	}
	office := string(officeSource(t))
	if !strings.Contains(office, "return !!(talkPanels && session && !session.t3 && talkPanels.expand(id));") {
		t.Error("the room's expand(id) no longer keeps a T3 Code desk, or a page with no talk, on the slate")
	}
}
