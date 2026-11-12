package desktop

import (
	"os"
	"strings"
	"testing"
)

// The talk bubble itself (1.26, P5): talk/bubble.js in node, its plain functions and its live bubble on a stand-in
// for the page; and deck.html giving the room the bubble's network only in the office's own window. Skipped without
// node.

func talkBubbleSources(t *testing.T) string {
	t.Helper()
	var all strings.Builder
	for _, name := range []string{"anchors.js", "style.js", "bubble.js"} {
		src, err := os.ReadFile("../Resources/web/office/talk/" + name)
		if err != nil {
			t.Fatal(err)
		}
		all.Write(src)
		all.WriteString("\n")
	}
	return all.String()
}

const talkSame = `
const same = (got, want, what) => { if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(what + ': got ' + JSON.stringify(got) + ', want ' + JSON.stringify(want)); };
const B = TALK_BUBBLE;
const item = (n, kind, text, from) => ({ id: '0:' + n, offset: n * 100, n: 0, kind, text, ...(from ? { from } : {}) });
const question = (pickable) => ({ since: 1761000545000, kind: 'question', tool: 'AskUserQuestion', label: 'AskUserQuestion', pickable,
  questions: [{ question: 'Run the migration now?', header: 'Migrate', multiSelect: false,
    options: [{ n: 1, label: 'Yes', description: '' }, { n: 2, label: 'Later', description: 'after the release' }] }] });
const permission = { since: 1761000541000, kind: 'permission', tool: 'Bash', label: 'Bash: make', questions: [], pickable: true };
const typingOK = { ok: true, lines: true, why: '' };
`

// Rows folded as the mockup (a run of tools and their results, one dim line), a choice's request (n and since, or
// the keys' /act), the input greyed while it asks and with typing.why, woken after No and only after No, digits never
// taken from inside a box, the slate's page merge.
func TestTheTalkBubbleFoldsRowsAndGatesItsChoicesAndInput(t *testing.T) {
	runNode(t, talkBubbleSources(t)+talkSame+`
// The mockup's conversation: the question, two tools (with their results) on one line, the answer.
const items = [
  item(1, 'user', 'The checkout test is failing. Can you fix it?'),
  item(2, 'tool', 'Edit: /work/shop/src/cart.ts', 'Edit'), item(3, 'result', 'The file was updated', 'Edit'),
  item(4, 'tool', 'Bash: npm test', 'Bash'), item(5, 'result', '42 passed', 'Bash'),
  item(6, 'tool', 'Edit: /work/shop/src/cart.ts', 'Edit'),
  item(7, 'agent', 'The checkout fix is ready and all 42 tests pass.'),
];
same(B.fold(items).map((r) => [r.kind, r.text]), [['user', 'The checkout test is failing. Can you fix it?'],
  ['tools', 'Changed cart.ts, ran npm test'], ['agent', 'The checkout fix is ready and all 42 tests pass.']], 'folded as the mockup');
// The newest four rows; a long run says how many more.
const many = [item(1, 'user', 'go'), ...['a', 'b', 'c', 'd', 'e'].map((f, i) => item(2 + i, 'tool', 'Read: /x/' + f + '.go', 'Read')),
  item(9, 'agent', 'one'), item(10, 'user', 'two'), item(11, 'agent', 'three'), item(12, 'question', 'Which?')];
same(B.fold(many).map((r) => r.text), ['one', 'two', 'three', 'Which?'], 'the newest four');
same(B.fold(many, 6)[1].text, 'Read a.go, read b.go, read c.go, and 2 more', 'a long run');
same(B.didOf(item(1, 'tool', 'WebFetch: https://example.com', 'WebFetch')), 'looked on the web', 'a tool in words');
same(B.didOf(item(1, 'tool', 'mcp__x', 'mcp__x')), 'used mcp__x', 'any other tool');

// A question's choices by label, and No; a permission's Yes and No; not pickable, none: the four keys answer it.
same(B.choicesOf(question(true)).map((c) => [c.n || c.act, c.label]), [[1, 'Yes'], [2, 'Later'], ['reject', 'No']], 'a question');
same(B.choicesOf(permission).map((c) => [c.n, c.label, !!c.no]), [[1, 'Yes', false], [2, 'No', true]], 'a permission');
same([B.choicesOf(question(false)), B.choicesOf(null)], [null, null], 'not pickable');
same(B.KEYS.map((k) => k.act), ['up', 'down', 'accept', 'reject'], 'the four keys');
// A choice's request: its number and the prompt's since, never any text; a key, /act.
same(B.sendOf('s1', question(true), B.choicesOf(question(true))[1]),
  { path: '/talk/pick', params: { session: 's1', n: '2', since: '1761000545000' } }, 'a question\'s option');
same(B.sendOf('s2', permission, B.choicesOf(permission)[1]), { path: '/talk/pick', params: { session: 's2', n: '2', since: '1761000541000' } }, 'a permission\'s No');
same(B.sendOf('s1', question(true), B.choicesOf(question(true))[2]), { path: '/act', params: { session: 's1', action: 'reject' } }, 'a question\'s No');
same(B.sendOf('s1', question(false), B.KEYS[2]), { path: '/act', params: { session: 's1', action: 'accept' } }, '✓');
same([B.questionOf(question(true)), B.questionOf(permission), B.questionOf({ kind: 'unknown', questions: [] })],
  ['Run the migration now?', 'Allow Bash: make?', 'It\'s asking you something.'], 'what it asks');

// The input: greyed while it asks; one plain line in its place where the window can't type; else the name's
// placeholder, a limit said under it. After No it wakes; after any other choice it waits for the desk.
const asking = { state: 'asking', asking: permission, last: '', size: 10, typing: { ok: false, lines: false, why: 'It\'s asking you something: answer it first, above.' } };
same(B.inputOf(asking, 'Rex'), { shown: true, disabled: true, placeholder: 'Answer above first', line: '' }, 'while it asks');
const tab = 'The bubble can\'t type into a Terminal, iTerm2 or Ghostty tab: type in its own window.';
same(B.inputOf({ state: 'idle', asking: null, typing: { ok: false, lines: false, why: tab } }, 'Rex'),
  { shown: false, disabled: true, placeholder: '', line: tab }, 'a tab hire: one plain line');
same(B.inputOf({ state: 'idle', asking: null, typing: { ok: true, lines: false, why: 'One line at a time.' } }, 'Rex'),
  { shown: true, disabled: false, placeholder: 'Tell Rex what to do', line: 'One line at a time.' }, 'one line only');
same(B.inputOf(null, 'Rex').disabled, true, 'before its card');
const afterNo = B.answered({ ...asking, typing: typingOK }, B.choicesOf(permission)[1]);
same([afterNo.asking, B.inputOf(afterNo, 'Rex').disabled, B.inputOf(afterNo, 'Rex').placeholder], [null, false, 'Tell Rex what to do'], 'woken after No');
same(B.inputOf(B.answered(asking, B.choicesOf(permission)[0]), 'Rex').disabled, true, 'still greyed after Yes, till the desk says');
same(B.answered(asking, B.KEYS[3]).asking, null, '✕ is a No too');

// Digits pick only outside a box, with no modifier.
const key = (k, tagName, more = {}) => ({ key: k, target: { tagName }, ...more });
same([B.digitOf(key('2', 'INPUT')), B.digitOf(key('2', 'TEXTAREA')), B.digitOf(key('2', 'DIV', { target: { tagName: 'DIV', isContentEditable: true } }))],
  [null, null, null], 'inside a box');
same([B.digitOf(key('2', 'BUTTON')), B.digitOf(key('9', 'DIV')), B.digitOf(key('0', 'DIV')), B.digitOf(key('a', 'DIV')),
  B.digitOf(key('2', 'DIV', { metaKey: true }))], [2, 9, null, null, null], 'on the bubble');

// The name: a nickname, else the role, else the project.
same([B.nameOf({ project: 'shop', role: { name: 'Bug hunter' } }, 'Rex'), B.nameOf({ project: 'shop', role: { name: 'Bug hunter' } }),
  B.nameOf({ project: 'shop', name: 'src' }), B.nameOf({})], ['Rex', 'Bug hunter', 'shop', 'Agent'], 'names');

// The slate's merge: new ones added in order, nothing new unchanged, a gap replaces what was there.
const shown = { items: [item(1, 'user', 'a'), item(2, 'agent', 'b')], before: 0 };
let m = B.merge(shown, { items: [item(2, 'agent', 'b'), item(3, 'agent', 'c')], before: 50 });
same([m.items.map((i) => i.text), m.before, m.changed], [['a', 'b', 'c'], 0, true], 'new ones');
m = B.merge(shown, { items: [item(1, 'user', 'a'), item(2, 'agent', 'b')], before: null });
same(m.changed, false, 'nothing new');
m = B.merge(shown, { items: [item(8, 'agent', 'h'), item(9, 'agent', 'i')], before: 700 });
same([m.items.map((i) => i.text), m.before], [['h', 'i'], 700], 'after a gap');
same(B.merge({ items: [], before: null }, { items: [item(1, 'user', 'a')], before: 40 }).before, 40, 'the first page');
`)
}

// The live bubble on a stand-in page: opened, it reads the desk's card and conversation, shows a permission's Yes and
// No with the input greyed; a digit typed in the input picks nothing, one on the bubble picks; No sends the prompt's
// since, and the input wakes; Send says the words to /talk/say. A tab hire's card: one plain line, no input. A card it
// can't offer by label: the four keys. A click on the same bubble closes it.
func TestTheLiveTalkBubbleAnswersOnlyByItsButtons(t *testing.T) {
	runNode(t, talkBubbleSources(t)+talkSame+`
class El {
  constructor(tag) { Object.assign(this, { tagName: tag.toUpperCase(), children: [], style: { setProperty() {} }, className: '', hidden: false,
    disabled: false, value: '', placeholder: '', listeners: {}, attrs: {}, parent: null, offsetWidth: 100, offsetHeight: 20, scrollHeight: 0 }); }
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
    return ev;
  }
  all() { return this.children.filter((c) => typeof c !== 'string').flatMap((c) => [c, ...c.all()]); }
  querySelector(sel) { return this.all().find((c) => c.className.split(' ').includes(sel.slice(1))) || null; }
  find(cls) { return this.all().filter((c) => c.className.split(' ').includes(cls) && !c.hidden); }
  focus() { focused = this; }
}
let focused = null;
const body = new El('body'), head = new El('head');
const doc = { body, head, createElement: (t) => new El(t), getElementById: (id) => head.all().find((c) => c.id === id) || null, addEventListener() {} };
const timers = [];
const win = { document: doc, innerHeight: 800, setTimeout: (f) => timers.push(f), clearTimeout() {} };
const settle = () => new Promise((r) => setTimeout(r, 5));

let card = { state: 'asking', asking: permission, last: 'Running make', size: 40, typing: { ok: false, lines: false, why: 'answer first' } };
const sent = [];
const res = (status, body) => ({ status, ok: status >= 200 && status < 300, json: async () => body, text: async () => String(body || '') });
const page = {
  get: async (path, params) => {
    sent.push(['get', path, params]);
    if (path === '/talk') return res(200, card);
    if (path === '/conversation') return res(200, { items: [item(1, 'user', 'build it'), item(2, 'tool', 'Bash: make', 'Bash')], before: null, size: 40 });
    return res(200, { asking: [] });
  },
  post: async (path, params, signal, json) => { sent.push(['post', path, params, json]); return res(204); },
  flash() {}, tick() {},
};
const anchor = { id: 's1', state: 'asking', since: 1, art: {}, css: { box: { left: 290, right: 314, top: 300, bottom: 324 }, tip: { x: 300, y: 322 } } };
const room = { canvas: { getBoundingClientRect: () => ({ left: 0, top: 0, right: 640, bottom: 360, width: 640, height: 360 }) },
  anchor: () => anchor, view: () => ({ scale: 2, ox: 0, oy: 0, k: 1, left: 0, top: 0 }), shape: () => ({ width: 320, height: 180 }),
  session: (id) => ({ id, state: 'asking', conversation: true, project: 'shop' }), name: () => 'Rex' };
const bubble = B.create(win, page, room);
const panel = () => body.children.find((c) => c.className && c.className.includes('talk-bubble'));
const reopen = async () => { bubble.open('s1'); timers.splice(0).forEach((f) => f()); bubble.open('s1'); await settle(); };
const keys = () => panel().find('talk-key').filter((k) => !['Esc', 'Send'].includes(k.textContent));

(async () => {
  bubble.open('s1');
  await settle();
  const p = panel();
  same([p.attrs.role, p.attrs['aria-label']], ['dialog', 'Rex: conversation'], 'a dialog named for its desk');
  same(sent.slice(0, 2).map((s) => s[1]), ['/talk', '/conversation'], 'its card, then its conversation');
  same(p.find('board-row').map((r) => r.textContent), ['build it', 'Ran make'], 'its rows, folded');
  same(keys().map((k) => k.textContent), ['1Yes', '2No'], 'Yes and No');
  const input = p.find('talk-input')[0].children[0];
  same([input.disabled, input.placeholder], [true, 'Answer above first'], 'greyed while it asks');
  same(focused && focused.textContent, '1Yes', 'focus on the first choice');

  // A digit in the input picks nothing; on the bubble, it picks.
  let n = sent.length;
  input.dispatch('keydown', { key: '1' });
  await settle();
  same(sent.slice(n).filter((s) => s[0] === 'post'), [], 'a digit typed in the input picked');
  card = { ...card, asking: null, state: 'idle', typing: typingOK };
  keys()[1].dispatch('keydown', { key: '2' });
  await settle();
  same(sent.slice(n).find((s) => s[0] === 'post'), ['post', '/talk/pick', { session: 's1', n: '2', since: '1761000541000' }, undefined], 'No by its digit');
  same([input.disabled, input.placeholder, focused === input], [false, 'Tell Rex what to do', true], 'woken after No, and focused');

  // Send: the words and Return, to /talk/say; the box empties.
  input.value = '  run the tests instead ';
  n = sent.length;
  p.find('talk-input')[0].dispatch('submit');
  await settle();
  same(sent[n], ['post', '/talk/say', { session: 's1' }, { text: 'run the tests instead', send: true }], 'Send');
  same(input.value, '', 'emptied once it went');

  // A tab hire on a Mac: one plain line, no input.
  card = { state: 'idle', asking: null, last: '', size: 40, typing: { ok: false, lines: false, why: 'The bubble can\'t type into a Terminal, iTerm2 or Ghostty tab: type in its own window.' } };
  await reopen();
  const q = panel();
  same([q.find('talk-input').length, q.find('talk-why').map((w) => w.textContent)],
    [0, ['The bubble can\'t type into a Terminal, iTerm2 or Ghostty tab: type in its own window.']], 'a tab hire\'s line');

  // A prompt it can't offer by label: the four keys, ✕ an /act reject.
  card = { state: 'asking', asking: { ...permission, tool: 'WebFetch', label: 'WebFetch: example.com', pickable: false }, size: 40, typing: { ok: false, why: 'answer first' } };
  await reopen();
  same(keys().map((k) => k.textContent), ['▲', '▼', '✓', '✕'], 'the four keys');
  n = sent.length;
  keys()[3].dispatch('click');
  await settle();
  same(sent[n], ['post', '/act', { session: 's1', action: 'reject' }, undefined], '✕');

  // The same bubble clicked again closes it; Esc does too.
  bubble.open('s1');
  timers.splice(0).forEach((f) => f());
  same(panel(), undefined, 'closed by a second click');
  bubble.open('s1');
  await settle();
  panel().dispatch('keydown', { key: 'Escape' });
  timers.splice(0).forEach((f) => f());
  same(panel(), undefined, 'closed by Esc');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}

// deck.html gives the room the talk bubble's network only in the office's own window on this computer: a phone's
// page, a browser paired with its own key, the screen saver and the setup page's preview have none (and keep ✓ ✕).
func TestTheDeckGivesTheRoomTalkOnlyInItsOwnWindow(t *testing.T) {
	data, err := os.ReadFile("../Resources/web/deck.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(data)
	from, to := "  const preview = location.pathname", "const talkHere = ownWindow && whole && !preview && !saver;"
	start, end := strings.Index(page, from), strings.Index(page, to)
	if start < 0 || end < start || !strings.Contains(page, "...(talkHere ? { talk: { get, post, flash, tick } } : {}),") {
		t.Fatal("deck.html no longer decides talkHere as this test expects: update it")
	}
	runNode(t, `
const lines = process.argv[1];
const check = (address, want) => {
  const url = new URL('http://127.0.0.1:7710' + address);
  const location = { pathname: url.pathname, search: url.search };
  const document = { documentElement: { classList: { add() {} } } };
  const talkHere = new Function('location', 'document', lines + '; return talkHere;')(location, document);
  if (talkHere !== want) throw new Error(address + ': talk ' + talkHere + ', want ' + want);
};
check('/office?window&t=k', true);       // the Mac app's own window
check('/office?t=k&own', true);          // the office's window on Linux and Windows
check('/office?t=k', false);             // a browser paired with its own key
check('/?t=k', false);                   // a phone's deck
check('/office?saver&t=k&own', false);   // the screen saver
check('/setup/preview?t=k&own', false);  // setup's preview
`, page[start:end+len(to)])
}
