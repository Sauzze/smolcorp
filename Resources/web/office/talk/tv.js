/**
 * The TV's big preview (1.26 TV3), in the computer's own office window only (the room's `on.tv`): a click on a TV with
 * the picked desk's preview on air opens that page live over the office, in a frame, with Reload, Copy address and
 * Close (Esc, or a click beside it). Nothing else opens a desk's page: no "Open in browser" and no link (owner's
 * answer F8 a), only the address to copy.
 *
 * The frame is a desk's own dev server, which any program of the desk wrote, so it's kept in its frame:
 *   - its sandbox lets it run its scripts, keep its own origin (its storage, and calls to its own server) and send its
 *     forms, and nothing more: no allow-top-navigation of any kind, so it can never take the window's place; no
 *     allow-popups, so it opens no window or tab; no allow-modals, so it puts no dialog on the screen; no downloads;
 *   - its address is checked again here (localhost, http, a port that isn't Smolcorp's own, no ? # or \), and is given
 *     to the frame as a property, after the sandbox, never written as HTML;
 *   - it sends no referrer, so the office's address never reaches it;
 *   - the page's CSP (server.go's pageFrames, TV2) lets a frame show http://localhost:* alone, never 127.0.0.1, where
 *     Setup and its cookie live; a frame that goes anywhere else is refused, and this preview says so.
 * Its origin is localhost at its own port, never the office's: the Mac's window, and a browser office opened from
 * Setup's line, are at 127.0.0.1:<Port>, Setup's own host; the computer's window is at localhost:<Port>, a port it's
 * never framed at. So it can't read the office or its key, and every window route wants that key. Setup is another
 * site from the frame's (127.0.0.1 against localhost), so its SameSite=Strict cookie is never sent with what the frame
 * asks, whatever the office's own address; Setup's changes also want its own header, whose preflight the core never
 * answers (tvframe_test.go). The Mac's window lets a frame load http://localhost:<port> and nothing else of anyone's
 * (OfficeWindow.decide), so the frame shows there too.
 *
 * An address found among the desk's own listening ports is a suggestion, never proof of what the desk runs
 * (RT126V1R2): the preview says so under its title. A page that won't show (refused by the CSP, or no answer in WAIT)
 * falls back to its address as text, to copy; one its own server refuses to have framed (X-Frame-Options) shows blank
 * in some browsers with no sign the page can read, so the hint under the frame says what to do then.
 *
 * Tab stays in the preview: past its last part it comes back to Reload, and back past Reload to Close. Esc closes it
 * from the preview's own keys; once a click has put the keyboard in the desk's page, the page has it (another origin's
 * keys never reach the office), and Close or a click beside it still close it.
 */
const TALK_TV = {
  ID: 'talk-tv-style',
  /** What the framed page may do: exactly these, never allow-top-navigation*, allow-popups* or allow-modals. */
  SANDBOX: 'allow-scripts allow-same-origin allow-forms',
  /** How long a page has to load before the preview offers its address instead, in ms (it shows if it loads later). */
  WAIT: 15000,
  /** The real apps' ports (7710, 7720, 7730), each serving an office with its window's key: never framed. */
  OWN_PORTS: ['7710', '7720', '7730'],

  CSS: `
.tv-over { --edge: #1b1e2a; --cream: #f4f1ea; --screen: #171b26; --dim: #8d95a6; --red: #ff4b4b; --orange: #ff8a1f;
  position: fixed; inset: 0; z-index: 8; display: flex; align-items: center; justify-content: center; padding: 3vh 3vw;
  box-sizing: border-box; background: rgba(12, 15, 22, .72); font: 13px/1.4 -apple-system, system-ui, sans-serif; color: var(--cream); }
.tv-over[hidden], .tv-over [hidden] { display: none !important; }
.tv-over * { box-sizing: border-box; }
.tv-box { display: flex; flex-direction: column; width: 100%; height: 100%; max-width: 1400px; background: var(--edge);
  box-shadow: 0 -4px 0 #3e495b, 0 4px 0 #3e495b, -4px 0 0 #3e495b, 4px 0 0 #3e495b, 0 10px 0 rgba(12, 15, 22, .45); padding: 10px; gap: 8px; }
.tv-head { display: flex; align-items: center; gap: 8px; min-width: 0; }
.tv-live { color: var(--red); font-weight: 700; letter-spacing: .08em; }
.tv-title { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tv-key { font: 700 12px/1 -apple-system, system-ui, sans-serif; text-transform: uppercase; color: var(--edge); background: var(--cream);
  border: 0; padding: 7px 10px; cursor: pointer; box-shadow: 0 2px 0 #8d8270; }
.tv-key:active { transform: translateY(2px); box-shadow: none; }
.tv-key:focus-visible { outline: 3px solid var(--orange); outline-offset: 2px; }
.tv-note, .tv-hint { color: var(--dim); font-size: 12px; }
.tv-screen { position: relative; flex: 1; min-height: 0; background: var(--screen); }
.tv-frame { width: 100%; height: 100%; border: 0; background: #fff; display: block; }
.tv-refused { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 10px;
  padding: 24px; text-align: center; background: var(--screen); }
.tv-address { font: 13px/1.4 ui-monospace, Menlo, monospace; user-select: all; -webkit-user-select: all; word-break: break-all; color: var(--cream); }
@media (prefers-reduced-motion: reduce) { .tv-key:active { transform: none; } }
`,

  /** The page's CSS, once. */
  inject(doc) {
    if (doc.getElementById(TALK_TV.ID)) return;
    const style = doc.createElement('style');
    style.id = TALK_TV.ID;
    style.textContent = TALK_TV.CSS;
    doc.head.append(style);
  },

  /**
   * The address to frame, or null: http://localhost:<port> with a path, at most 2048 characters, without ?, # or \,
   * and never at `ownPort` (the page's own) or one of OWN_PORTS. As onAirOf in main.js, plus Smolcorp's ports.
   */
  addressOf(url, ownPort) {
    if (typeof url !== 'string' || url.length > 2048 || /[?#\\]/.test(url)) return null;
    const found = /^http:\/\/localhost:([1-9][0-9]{0,4})(\/[\x21-\x7e]*)?$/.exec(url);
    if (!found || Number(found[1]) > 65535) return null;
    if (found[1] === String(ownPort || '') || TALK_TV.OWN_PORTS.includes(found[1])) return null;
    return url;
  },

  /** What the preview's head says for `shown` ({ id, url, found }) at desk `name`: its title, and the note under it. */
  wordsOf(shown, name) {
    const where = shown.url.replace(/^http:\/\//, '');
    return {
      title: `${where} · ${name}`,
      note: shown.found === true ? `Found on a port ${name}'s own programs listen on: a suggestion, not proof of what it's building.` : '',
    };
  },

  /** Whether a securitypolicyviolation is a frame refused by the page's CSP (frame-src, or what stands in for it). */
  refusedBy(e) {
    return /^(frame-src|child-src|default-src)\b/.test(String((e && (e.effectiveDirective || e.violatedDirective)) || ''));
  },

  /** Puts `text` on the clipboard, from a click: the clipboard API, else a selected text box copied. Whether it went. */
  async copy(win, text) {
    try {
      if (win.navigator && win.navigator.clipboard) {
        await win.navigator.clipboard.writeText(text);
        return true;
      }
    } catch { /* the older way below */ }
    const doc = win.document, box = doc.createElement('textarea');
    try {
      box.value = text;
      box.setAttribute('readonly', '');
      box.style.position = 'fixed';
      box.style.opacity = '0';
      doc.body.append(box);
      box.select();
      return !!doc.execCommand('copy');
    } catch {
      return false;
    } finally {
      box.remove();
    }
  },

  /**
   * The live preview on `win`'s page. `room`: { canvas, name(id) }, the focus going back to the canvas as it closes.
   * Gives { open(shown), close(), shown() }: `shown` is the room's preview on air, { id, url, found }; open again, it
   * shows that one instead.
   */
  create(win, room) {
    const T = TALK_TV, doc = win.document;
    const make = (tag, cls, text) => {
      const el = doc.createElement(tag);
      if (cls) el.className = cls;
      if (text !== undefined) el.textContent = text;
      return el;
    };
    const key = (text, press) => {
      const b = make('button', 'tv-key', text);
      b.type = 'button';
      b.addEventListener('click', (e) => {
        e.stopPropagation();
        press(b);
      });
      return b;
    };
    let p = null;        // the parts, made on first open
    let now = null;      // what's shown: { id, url, found, frame, loaded, refused, timer }
    let copied = null;   // the timer putting Copy address's own words back

    const refuse = (why) => {
      if (!now) return;
      now.refused = why;
      if (now.frame) now.frame.hidden = true;
      p.why.textContent = why === 'csp' ? 'Smolcorp shows only a page at http://localhost here, and this one went somewhere else.'
        : why === 'address' ? 'This address can\'t be shown here: only a desk\'s own http://localhost:<port> can.'
          : 'It hasn\'t answered yet. Is its server running? Reload tries again.';
      p.refused.hidden = false;
    };
    // The frame: its sandbox and referrer first, its address last, as a property; nothing loads until it's in the page.
    const frameOf = (url, name) => {
      const frame = make('iframe', 'tv-frame');
      frame.setAttribute('sandbox', T.SANDBOX);
      frame.referrerPolicy = 'no-referrer';
      frame.title = `Preview of ${name}`;
      frame.addEventListener('load', () => {
        if (!now || now.frame !== frame) return;
        now.loaded = true;
        // Late, but here: shown after all (a refusal by the CSP stays).
        if (now.refused === 'slow') {
          now.refused = null;
          frame.hidden = false;
          p.refused.hidden = true;
        }
      });
      frame.src = url;
      return frame;
    };
    const load = () => {
      if (now.timer !== null) win.clearTimeout(now.timer);
      now.timer = null;
      now.loaded = false;
      now.refused = null;
      if (now.frame) now.frame.remove();
      now.frame = null;
      p.refused.hidden = true;
      const url = T.addressOf(now.url, win.location && win.location.port);
      if (!url) return refuse('address');
      const frame = frameOf(url, now.name);
      now.frame = frame;
      p.screen.prepend(frame);
      // Only for this frame: one Reload or Close took away is nothing to it, its timer cleared or not.
      now.timer = win.setTimeout(() => {
        if (!now || now.frame !== frame) return;
        now.timer = null;
        if (!now.loaded && !now.refused) refuse('slow');
      }, T.WAIT);
    };
    const violated = (e) => {
      if (now && T.refusedBy(e)) refuse('csp');
    };
    const keyed = (e) => {
      if (e.key !== 'Escape') return;
      e.preventDefault();
      e.stopPropagation();
      close();
    };
    const build = () => {
      T.inject(doc);
      p = { over: make('div', 'tv-over'), box: make('div', 'tv-box'), head: make('div', 'tv-head'), title: make('span', 'tv-title'),
        note: make('div', 'tv-note'), screen: make('div', 'tv-screen'), refused: make('div', 'tv-refused'), why: make('p', 'tv-why'),
        address: make('code', 'tv-address'), first: make('span', 'tv-trap'), last: make('span', 'tv-trap'),
        hint: make('div', 'tv-hint', 'Nothing showing? Some dev servers refuse to be shown inside another page: Copy address to take it elsewhere.') };
      p.over.setAttribute('role', 'dialog');
      p.over.setAttribute('aria-modal', 'true');
      p.reload = key('Reload', () => now && load());
      p.copy = key('Copy address', async (b) => {
        if (!now) return;
        const went = await T.copy(win, now.url);
        b.textContent = went ? 'Copied' : 'Select the address to copy';
        if (copied !== null) win.clearTimeout(copied);
        copied = win.setTimeout(() => { copied = null; b.textContent = 'Copy address'; }, 2000);
      });
      p.close = key('Close', () => close());
      p.head.append(make('span', 'tv-live', 'LIVE'), p.title, p.reload, p.copy, p.close);
      p.refused.append(p.why, p.address);
      p.screen.append(p.refused);
      p.box.append(p.head, p.note, p.screen, p.hint);
      // Tab past the preview's last part, the frame's page included, comes back to its first key, and back past that
      // to its last: the room behind isn't reached while it's open.
      for (const [trap, to] of [[p.first, p.close], [p.last, p.reload]]) {
        trap.setAttribute('tabindex', '0');
        trap.addEventListener('focus', () => to.focus());
      }
      p.over.append(p.first, p.box, p.last);
      p.over.addEventListener('keydown', keyed);
      // A click beside the preview closes it; one inside it stays there.
      p.over.addEventListener('click', (e) => { if (e.target === p.over) close(); });
      p.over.hidden = true;
      doc.body.append(p.over);
    };
    function close() {
      if (!now) return;
      if (now.timer !== null) win.clearTimeout(now.timer);
      // The page stops with its frame: nothing of it runs once the preview is closed.
      if (now.frame) now.frame.remove();
      now = null;
      doc.removeEventListener('securitypolicyviolation', violated, true);
      p.over.hidden = true;
      if (room.canvas && room.canvas.focus) room.canvas.focus();
    }
    return {
      open(shown) {
        if (!shown || typeof shown.url !== 'string' || typeof shown.id !== 'string') return;
        if (!p) build();
        if (now && now.timer !== null) win.clearTimeout(now.timer);
        const name = room.name(shown.id);
        const words = T.wordsOf(shown, name);
        if (!now) doc.addEventListener('securitypolicyviolation', violated, true);
        now = { id: shown.id, url: shown.url, found: shown.found === true, name, frame: now && now.frame, loaded: false, refused: null, timer: null };
        p.over.setAttribute('aria-label', `Preview of ${name}`);
        p.title.textContent = words.title;
        p.note.textContent = words.note;
        p.note.hidden = !words.note;
        p.address.textContent = shown.url;
        p.over.hidden = false;
        load();
        p.close.focus();
      },
      close,
      shown: () => (now ? { id: now.id, url: now.url } : null),
    };
  },
};
