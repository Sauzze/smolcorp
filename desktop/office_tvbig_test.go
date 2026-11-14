package desktop

import (
	"os"
	"strings"
	"testing"
)

// The TV's big preview (1.26 TV3): talk/tv.js in node on a stand-in page that records how each element is made (its
// attributes, the properties a frame's address could go in, and when it joins the page), alone and in the room
// (tvRig, office_tv_test.go). Skipped without node.

// tvElements are stand-in elements that bubble their events up to the document (docListeners) and keep a log of what
// was done to them: their attributes, a frame's address (src, which notes whether the frame was in the page yet), and
// each joining the page; every element made is in made, and an href or HTML written fails the test.
const tvElements = `
const log = [], made = [], docListeners = {};
let focused = null;
class El {
  constructor(tag) { Object.assign(this, { tagName: tag.toUpperCase(), children: [], style: {}, className: '', hidden: false, attrs: {}, listeners: {}, parent: null, props: {} }); made.push(this); }
  set textContent(t) { this.children = [String(t)]; }
  get textContent() { return this.children.map((c) => (typeof c === 'string' ? c : c.textContent)).join(''); }
  set src(v) { this.props.src = v; log.push(['src', this.tagName, v, !!this.connected()]); }
  get src() { return this.props.src; }
  set href(v) { throw new Error('an href was set: ' + v); }
  set innerHTML(v) { throw new Error('HTML was written: ' + v); }
  connected() { for (let el = this; el; el = el.parent) if (el === body) return true; return false; }
  append(...kids) { for (const k of kids) { if (typeof k !== 'string') { k.parent = this; log.push(['in', k.tagName]); } this.children.push(k); } }
  prepend(...kids) { for (const k of kids.reverse()) { if (typeof k !== 'string') { k.parent = this; log.push(['in', k.tagName]); } this.children.unshift(k); } }
  replaceChildren(...kids) { this.children = []; this.append(...kids); }
  remove() { if (this.parent) this.parent.children = this.parent.children.filter((c) => c !== this); this.parent = null; }
  setAttribute(k, v) { this.attrs[k] = String(v); log.push(['attr', this.tagName, k, String(v)]); }
  addEventListener(type, f) { (this.listeners[type] = this.listeners[type] || []).push(f); }
  dispatch(type, e = {}) {
    const ev = { target: this, preventDefault() { ev.defaultPrevented = true; }, stopPropagation() { ev.stopped = true; }, ...e };
    for (let el = this; el && !ev.stopped; el = el.parent) (el.listeners[type] || []).forEach((f) => f(ev));
    if (!ev.stopped) (docListeners[type] || []).forEach((f) => f(ev));
    return ev;
  }
  all() { return this.children.filter((c) => typeof c !== 'string').flatMap((c) => [c, ...c.all()]); }
  find(cls) { return this.all().filter((c) => c.className.split(' ').includes(cls)); }
  focus() { focused = this; }
  select() {}
}
const body = new El('body'), head = new El('head');
`

// tvPage is a stand-in page on tvElements: `doc`, and `win`, whose open() fails the test (nothing opens a window), with
// a clipboard that keeps what it's given and a text box's copy that may fail.
const tvPage = tvElements + `
const doc = { body, head, createElement: (t) => new El(t), getElementById: (id) => head.all().find((c) => c.id === id) || null,
  addEventListener: (type, f) => (docListeners[type] = docListeners[type] || []).push(f),
  removeEventListener: (type, f) => { docListeners[type] = (docListeners[type] || []).filter((g) => g !== f); },
  execCommand: (what) => { copiedBy.push(what); return execWorks; } };
const timers = [];
const clipboard = [], copiedBy = [];
let clipboardWorks = true, execWorks = true;
const win = { document: doc, location: { port: '7799' }, setTimeout: (f, ms) => { timers.push([f, ms]); return timers.length; }, clearTimeout(n) { if (timers[n - 1]) timers[n - 1][0] = () => {}; },
  open() { throw new Error('a window was opened'); },
  navigator: { clipboard: { writeText: async (text) => { if (!clipboardWorks) throw new Error('not allowed'); clipboard.push(text); } } } };
const settle = () => new Promise((r) => setTimeout(r, 5));
const same = (got, want, what) => { if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(what + ': got ' + JSON.stringify(got) + ', want ' + JSON.stringify(want)); };
const over = () => body.find('tv-over')[0];
const frames = () => body.all().filter((c) => c.tagName === 'IFRAME');
const keyNamed = (text) => over().find('tv-key').find((k) => k.textContent === text);
const shownText = (cls) => { const el = over().find(cls)[0]; return el && !el.hidden ? el.textContent : null; };
`

func tvSource(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("../Resources/web/office/talk/tv.js")
	if err != nil {
		t.Fatal(err)
	}
	return string(src)
}

// The preview frames the desk's page sandboxed: exactly scripts, its own origin and forms, never top navigation,
// popups or dialogs; the sandbox and the referrer go on first, the address last, as a property, before the frame joins
// the page. Reload makes the frame again; Close (or Esc, or a click beside it) takes it away, page and all.
func TestTheTVsBigPreviewIsASandboxedFrame(t *testing.T) {
	runNode(t, tvSource(t)+tvPage+`
(async () => {
  const sandbox = TALK_TV.SANDBOX.split(' ');
  same(sandbox.slice().sort(), ['allow-forms', 'allow-same-origin', 'allow-scripts'], 'the sandbox');
  for (const never of sandbox) if (/^allow-(top-navigation|popups|modals|downloads|presentation|pointer-lock|storage-access)/.test(never)) throw new Error('the sandbox lets the page out: ' + never);

  const names = { a: 'Rex', b: 'Bo' };
  const canvas = { focus() { focused = 'canvas'; } };
  const tv = TALK_TV.create(win, { canvas, name: (id) => names[id] || 'Agent' });
  tv.open({ id: 'a', url: 'http://localhost:3000/app/', found: false });
  same(frames().length, 1, 'one frame');
  const f = frames()[0];
  same(f.attrs, { sandbox: TALK_TV.SANDBOX }, 'the frame\'s attributes: its sandbox alone, no src');
  same([f.props.src, f.referrerPolicy, f.title], ['http://localhost:3000/app/', 'no-referrer', 'Preview of Rex'], 'its address, referrer and title');
  const order = log.filter((l) => l[1] === 'IFRAME' || (l[0] === 'in' && l[1] === 'IFRAME')).map((l) => l[0] + (l[0] === 'src' ? ':' + l[3] : l[0] === 'attr' ? ':' + l[2] : ''));
  same(order, ['attr:sandbox', 'src:false', 'in'], 'the sandbox, then the address (out of the page), then into the page');
  same([over().hidden, over().attrs.role, over().attrs['aria-modal'], over().attrs['aria-label']], [false, 'dialog', 'true', 'Preview of Rex'], 'the dialog');
  same([shownText('tv-title'), shownText('tv-note'), focused === keyNamed('Close')], ['localhost:3000/app/ · Rex', null, true], 'its head, no note, the focus on Close');
  same(made.filter((el) => el.tagName === 'A').length, 0, 'no link anywhere');

  // Exactly these parts, and nothing else that answers a click, a key or the focus: no Open in browser (owner's F8 a),
  // under any name or class (RT126TV3 R3)
  same(over().find('tv-head')[0].children.map((c) => c.className), ['tv-live', 'tv-title', 'tv-key', 'tv-key', 'tv-key'], 'the head');
  same(over().find('tv-key').map((k) => k.textContent), ['Reload', 'Copy address', 'Close'], 'the keys');
  same(made.filter((el) => el.tagName === 'BUTTON').map((k) => k.textContent), ['Reload', 'Copy address', 'Close'], 'every button made');
  same(over().find('tv-refused')[0].children.map((c) => c.className), ['tv-why', 'tv-address'], 'the refused box: why, and the address as text');
  const answering = [over(), ...over().all()].filter((el) => Object.keys(el.listeners).length)
    .map((el) => [el.tagName + '.' + el.className + (el.tagName === 'BUTTON' ? ':' + el.textContent : ''), Object.keys(el.listeners)]);
  same(answering, [['DIV.tv-over', ['keydown', 'click']], ['SPAN.tv-trap', ['focus']], ['BUTTON.tv-key:Reload', ['click']],
    ['BUTTON.tv-key:Copy address', ['click']], ['BUTTON.tv-key:Close', ['click']], ['IFRAME.tv-frame', ['load']], ['SPAN.tv-trap', ['focus']]],
    'what answers anything in the preview');
  // Tab stays in it (RT126TV3 N3): a trap each side of the box, past the end back to Reload, before the start to Close
  const traps = over().find('tv-trap');
  same([traps.length, traps.map((x) => x.attrs.tabindex), over().children[0] === traps[0], over().children[2] === traps[1]], [2, ['0', '0'], true, true], 'the traps');
  traps[1].dispatch('focus');
  same(focused === keyNamed('Reload'), true, 'Tab past the end: Reload');
  traps[0].dispatch('focus');
  same(focused === keyNamed('Close'), true, 'Shift-Tab before the start: Close');

  // Reload: a new frame in the old one's place, sandboxed as the first
  keyNamed('Reload').dispatch('click');
  same(frames().length, 1, 'one frame after Reload');
  if (frames()[0] === f || f.parent) throw new Error('Reload kept the old frame');
  same([frames()[0].attrs, frames()[0].props.src], [{ sandbox: TALK_TV.SANDBOX }, 'http://localhost:3000/app/'], 'the frame Reload made');

  // Esc: closed, the frame gone with its page, the focus back on the room
  over().dispatch('keydown', { key: 'Escape' });
  same([over().hidden, frames().length, focused, tv.shown()], [true, 0, 'canvas', null], 'Esc');
  // Open again, another desk: a click inside the preview keeps it, one beside it closes it
  tv.open({ id: 'b', url: 'http://localhost:5173', found: true });
  same([shownText('tv-title'), frames().length, frames()[0].props.src], ['localhost:5173 · Bo', 1, 'http://localhost:5173'], 'opened again for Bo');
  over().find('tv-box')[0].dispatch('click');
  same(over().hidden, false, 'a click inside the preview');
  over().dispatch('click');
  same([over().hidden, frames().length], [true, 0], 'a click beside it');
  tv.open({ id: 'a', url: 'http://localhost:3000' });
  keyNamed('Close').dispatch('click');
  same([over().hidden, frames().length], [true, 0], 'Close');
  same(made.filter((el) => el.className === 'tv-over').length, 1, 'one preview on the page, made once');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}

// A found address is shown as a suggestion; a declared one isn't. Copy address copies it (the clipboard, else a
// selected box), and says when it couldn't.
func TestTheTVsBigPreviewSaysAFoundAddressIsASuggestionAndCopiesIt(t *testing.T) {
	runNode(t, tvSource(t)+tvPage+`
(async () => {
  const tv = TALK_TV.create(win, { canvas: null, name: () => 'Rex' });
  tv.open({ id: 'a', url: 'http://localhost:3000', found: true });
  const note = shownText('tv-note');
  if (!note || !note.includes('suggestion') || !note.includes('Rex')) throw new Error('a found address, no suggestion said: ' + note);
  tv.open({ id: 'a', url: 'http://localhost:3000', found: 'yes' });
  same(shownText('tv-note'), null, 'found must be true itself');
  tv.open({ id: 'a', url: 'http://localhost:3000' });
  same(shownText('tv-note'), null, 'a declared address');

  const copy = keyNamed('Copy address');
  copy.dispatch('click');
  await settle();
  same([clipboard, copy.textContent], [['http://localhost:3000'], 'Copied'], 'copied by the clipboard');
  timers.splice(0).forEach(([f]) => f());
  same(copy.textContent, 'Copy address', 'its words back');
  clipboardWorks = false;
  copy.dispatch('click');
  await settle();
  same([copiedBy, copy.textContent], [['copy'], 'Copied'], 'copied by a selected box');
  if (made.some((el) => el.tagName === 'TEXTAREA' && el.parent)) throw new Error('the box stayed on the page');
  execWorks = false;
  copy.dispatch('click');
  await settle();
  same(copy.textContent, 'Select the address to copy', 'nothing copied');
})().catch((e) => { console.error(e); process.exit(1); });
`)
}

// A frame that won't show falls back to its address as text: one the page's CSP refused (it went off localhost), one
// that hasn't loaded in WAIT (shown after all if it loads later), and an address that is no desk's preview, which is
// never framed at all: 127.0.0.1, https, Smolcorp's ports, the page's own, and every shape onAirOf refuses.
func TestTheTVsBigPreviewFallsBackToTheAddress(t *testing.T) {
	runNode(t, tvSource(t)+tvPage+`
(async () => {
  const tv = TALK_TV.create(win, { canvas: null, name: () => 'Rex' });
  const refused = () => shownText('tv-refused');
  tv.open({ id: 'a', url: 'http://localhost:3000' });
  same(refused(), null, 'nothing refused yet');
  // The CSP refused the frame (a redirect to 127.0.0.1, say): the address, to copy, and the frame out of sight
  doc.body.find('tv-frame')[0].dispatch('securitypolicyviolation', { effectiveDirective: 'img-src' });
  same(refused(), null, 'another directive is not the frame');
  docListeners.securitypolicyviolation.forEach((f) => f({ effectiveDirective: 'frame-src', blockedURI: 'http://127.0.0.1:3000' }));
  const words = refused();
  if (!words || !words.includes('http://localhost:3000') || !words.includes('somewhere else')) throw new Error('refused by the CSP: ' + words);
  same(frames()[0].hidden, true, 'the refused frame out of sight');
  frames()[0].dispatch('load');
  same(frames()[0].hidden, true, 'a refusal by the CSP stays, loaded or not');

  // Slow: the address after WAIT; the page loading later shows after all
  keyNamed('Reload').dispatch('click');
  same(refused(), null, 'Reload tries again');
  const wait = timers.filter(([, ms]) => ms === TALK_TV.WAIT);
  wait[wait.length - 1][0]();
  if (!(refused() || '').includes('hasn\'t answered')) throw new Error('slow: ' + refused());
  frames()[0].dispatch('load');
  same([refused(), frames()[0].hidden], [null, false], 'loaded late, shown');
  // Loaded in time: the wait running out changes nothing
  keyNamed('Reload').dispatch('click');
  frames()[0].dispatch('load');
  timers.filter(([, ms]) => ms === TALK_TV.WAIT).forEach(([f]) => f());
  same(refused(), null, 'loaded in time');
  tv.close();
  same(docListeners.securitypolicyviolation.length, 0, 'closed, the CSP is no longer listened to');

  // Never a desk's preview: never framed
  for (const bad of ['http://127.0.0.1:3000', 'https://localhost:3000', 'http://localhost:7710', 'http://localhost:7720/', 'http://localhost:7730',
    'http://localhost:7799', 'http://localhost', 'http://localhost:0', 'http://localhost:65536', 'http://localhost:3000?x=1', 'http://localhost:3000/#top',
    'http://localhost:3000\\x', 'http://localhost:3000/a b', 'http://user@localhost:3000', 'http://localhost.evil:3000', 'javascript:alert(1)',
    'http://localhost:3000/' + 'x'.repeat(2100), '//localhost:3000', 'HTTP://localhost:3000']) {
    tv.open({ id: 'a', url: bad });
    same(frames().length, 0, bad + ' was framed');
    const why = refused() || '';
    if (!why.includes(bad) || !why.includes('can\'t be shown here')) throw new Error(bad + ': ' + why);
  }
  for (const bad of [null, undefined, { id: 'a' }, { url: 'http://localhost:3000' }, { id: 'a', url: 7 }]) {
    tv.close();
    tv.open(bad);
    same([frames().length, over().hidden], [0, true], JSON.stringify(bad) + ' opened something');
  }
  same([TALK_TV.addressOf('http://localhost:65535/x', '8080'), TALK_TV.addressOf('http://localhost:8080', '8080')], ['http://localhost:65535/x', null], 'addressOf');
  same(made.filter((el) => el.tagName === 'A').length, 0, 'no link anywhere');
})().catch((e) => { console.error(e); process.exit(1); });
`)
	// Text goes on the page as text; nothing opens a window, a tab or a link (owner's F8 a: Copy address only).
	src := tvSource(t)
	for _, never := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "createContextualFragment",
		"win.open", "window.open", ".href", "'a'", "location.assign", "location.replace", "location =", "_blank", "_top"} {
		if strings.Contains(src, never) {
			t.Errorf("talk/tv.js uses %q: the preview shows text as text and opens nothing", never)
		}
	}
}

// In the room: a click on a TV on air opens the big preview of the picked desk's address, found or declared; a click
// on a TV with nothing on air, or a room without on.tv, opens none.
func TestATVOnAirOpensTheBigPreview(t *testing.T) {
	// The page says which addresses were found (deck.html), and a page the computer won't tell isn't told to wait for one.
	page, err := os.ReadFile("../Resources/web/deck.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"{ id, url: answer.url, found: answer.source === 'found' }",
		"if (previewOff) return 'The TV shows a desk\\'s preview only in Smolcorp\\'s own window';"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("deck.html lost %q", want)
		}
	}
	runNode(t, tvRig(t)+`
const shownText = (cls) => { const el = win.document.body.find(cls)[0]; return el && !el.hidden ? el.textContent : null; };
const frames = () => win.document.body.all().filter((c) => c.tagName === 'IFRAME');
const room = rig(false, sessions(), true, WALL_TV);
room.step(5);
const wall = middleOf([29, 8, 26, 15]);
let n = room.calls.length;
room.click(...wall);
same([tvCalls(room, n), frames().length], [[['tv', 'a', null]], 0], 'off air: on.tv, no preview');

room.office.setPreview({ id: 'a', url: 'http://localhost:3000', found: true });
room.step(1);
n = room.calls.length;
room.click(...wall);
same(tvCalls(room, n), [['tv', 'a', 'http://localhost:3000']], 'on air: on.tv as before');
same([frames().length, frames()[0].props.src, frames()[0].attrs.sandbox], [1, 'http://localhost:3000', 'allow-scripts allow-same-origin allow-forms'], 'the big preview, framed');
if (!(shownText('tv-note') || '').includes('suggestion')) throw new Error('found, but not said to be a suggestion');
same((shownText('tv-title') || '').startsWith('localhost:3000 · '), true, 'its title');
// Closed by the room going away
room.office.close();
same(frames().length, 0, 'the room closed, the preview with it');

// Another desk picked: nothing on air, nothing opens
room.office.setPreview({ id: 'a', url: 'http://localhost:4000' });
room.office.select('b');
room.step(1);
room.click(...wall);
same(frames().length, 0, 'another desk picked');

// A room without on.tv opens nothing
const bare = rig(false, sessions(), false, WALL_TV);
bare.office.setPreview({ id: 'a', url: 'http://localhost:3000' });
bare.step(2);
bare.click(...wall);
same(frames().length, 0, 'a room without on.tv');
`, officePath(t))
}
