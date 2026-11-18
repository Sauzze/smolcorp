/**
 * The Pixel Terminal (1.26, Talk Bubble's Expand), in the office's own window only: Expand on an open bubble, or the ▤
 * key, opens that desk's whole conversation on a large chalkboard monitor, the room zooming toward the desk and dimming
 * behind it (a cross-fade alone with reduced motion). Its header: Back to office (Esc), Shrink to bubble (the office
 * again, that desk's bubble open), the desk's status as a word, Next question N (the next desk waiting on you, longest
 * waiting first, N of them; greyed with none) and a Menu with Open in Terminal where the page offers it. Its body: the
 * conversation with Older messages, following new lines only while its foot is in sight (the ▤ slate's rule); the
 * desk's question with its choices, and the four keys under them (D4); and a roomy input, Enter sends, Shift+Enter a
 * new line. Its side: a small picture of the office (a click goes back) and the other desks, waiting ones first, a
 * click switching the terminal to that desk. One desk at a time.
 *
 * The bubble's rules hold: text is shown as text, never as HTML; nothing is typed into a prompt (the input is greyed
 * while the desk asks, and wakes after a No); a desk the window can't type into says so in one plain line.
 *
 * The office's extension seam (F-O): `PIXEL_TERMINAL.addPanel(name, draw)` adds a panel under the conversation, and
 * `TALK_BUBBLE.addKey(name, key)` a key on the bubble and in this menu. A feature lives in a file of its own,
 * `talk/<name>.js`, included after this one, that registers itself as its const is made:
 *
 *   const TALK_DIFF = {
 *     draw(box, desk) { … },
 *     added: PIXEL_TERMINAL.addPanel('diff', (box, desk) => TALK_DIFF.draw(box, desk)),
 *   };
 *
 * so it touches only buildoffice's module list and one //#include line.
 */
const PIXEL_TERMINAL = {
  /** How much the room grows toward the desk behind the terminal, and how long it takes, in ms. */
  ZOOM: 1.6,
  ZOOM_MS: 200,
  /** The office's picture in the side, its width in CSS pixels, drawn again at most this often (ms). */
  PICTURE: 184,
  PICTURE_EVERY: 250,
  /** The panels other files added (F-O): name → draw(box, desk). */
  panels: new Map(),

  /**
   * A panel under the conversation, from another file: `draw(box, desk)` fills `box`, an element of its own (empty,
   * nothing shows), each time the terminal shows a desk or reads its card again (every 2 or 3 seconds), so it draws
   * only what changed; on another desk, the box starts empty again. `desk` is TALK_BUBBLE.deskOf's. Added again
   * under the same name, it takes the first one's place. Gives the name.
   */
  addPanel(name, draw) {
    PIXEL_TERMINAL.panels.set(String(name), draw);
    return name;
  },

  /**
   * Next question: of the desks waiting on you (`order`, longest waiting first), how many there are besides `id`, and
   * the one Next goes to: the one after `id` in that order, round to the first; the first, where `id` isn't waiting.
   */
  nextOf(order, id) {
    const at = order.indexOf(id), others = order.filter((o) => o !== id);
    const next = at < 0 ? others[0] : [...order.slice(at + 1), ...order.slice(0, at)][0];
    return { count: others.length, next: next || null };
  },

  /**
   * The side's desk list: every desk with a talk bubble but `id`, those waiting on you first (longest waiting first,
   * as Needs you), then the rest by name. `desks`: [{ id, state, since }].
   */
  listOf(desks, id, nameOf) {
    const waiting = TALK_ANCHORS.needsYou(desks).filter((d) => d.id !== id);
    const rest = desks.filter((d) => d.state !== 'asking' && d.id !== id)
      .sort((a, b) => nameOf(a.id).localeCompare(nameOf(b.id)) || String(a.id).localeCompare(String(b.id)));
    return [...waiting, ...rest].map((d) => ({ id: d.id, state: d.state, name: nameOf(d.id) }));
  },

  /** An older page of /conversation put above what's shown ({ items, before }): its items not there yet, in the file's order. */
  older(shown, page) {
    const known = new Set(shown.items.map((i) => i.id));
    return { items: [...page.items.filter((i) => !known.has(i.id)), ...shown.items].sort(TALK_BUBBLE.byOrder), before: page.before };
  },

  /**
   * Tab inside the terminal, `count` keys and boxes in it, the focus on the one at `at` (-1: outside it): round from
   * the last to the first (Shift+Tab, `back`, the other way), and in from outside; null where the page's own Tab does.
   */
  wrapOf(count, at, back) {
    if (!count) return null;
    if (at < 0) return back ? count - 1 : 0;
    if (back && at === 0) return count - 1;
    return !back && at === count - 1 ? 0 : null;
  },

  /** Whether the conversation's foot is in sight, give or take 40 px: only then do new lines take it down. */
  atFoot(el) {
    return el.scrollTop + el.clientHeight >= el.scrollHeight - 40;
  },

  /** A key in the roomy input: Enter sends; Shift+Enter is a new line, and so is Enter while a letter is being composed. */
  keyOf(e) {
    return e.key === 'Enter' && !e.shiftKey && !e.isComposing && e.keyCode !== 229 ? 'send' : null;
  },

  /** The room's zoom toward the desk: the point on the canvas it grows from (the head, else the middle). None with reduced motion. */
  zoomOf(tip, rect, reduced) {
    if (reduced) return null;
    const x = tip ? tip.x - rect.left : rect.width / 2, y = tip ? tip.y - rect.top : rect.height / 2;
    return { origin: `${Math.round(x)}px ${Math.round(y)}px`, scale: PIXEL_TERMINAL.ZOOM };
  },

  /**
   * The live terminal on `win`'s page, with the bubble's `page` and `room` (see TALK_BUBBLE.create), and `bubble`,
   * whose open(id) Shrink calls. New lines are said in a live region of its own, inside the dialog (a screen reader
   * may not hear one outside a modal dialog). `page.openInTerminal`, where given,
   * is the menu's Open in Terminal: { shown(), open(id) }. Gives { open(id, from), close(), shown(), frame(bubbles),
   * remove() }; `from` is what the bubble showed ({ card, items, before, size }), so the terminal opens at the same
   * place in the conversation.
   */
  create(win, page, room, bubble) {
    const T = PIXEL_TERMINAL, B = TALK_BUBBLE, doc = win.document;
    const make = (tag, cls, text) => {
      const el = doc.createElement(tag);
      if (cls) el.className = cls;
      if (text !== undefined) el.textContent = text;
      return el;
    };
    const key = (cls, text, press) => {
      const b = make('button', `talk-key ${cls}`, text);
      b.type = 'button';
      b.addEventListener('click', (e) => {
        e.stopPropagation();
        press();
      });
      return b;
    };
    let p = null;            // the terminal's parts, made on first open
    let term = null;         // the desk shown: { id, card, items, before, size, loaded, timer, round, drawn, at, listed, boxes }
    let desks = [];          // the desks with talk bubbles, from the room's frames: { id, state, since }
    let pictured = 0;
    let sending = false;     // a /talk/say on its way: Enter or Send again sends nothing more until it's back
    const drafts = new Map(); // words left in the input, by desk, so switching never sends them to another
    const reduced = () => !!(win.matchMedia && win.matchMedia('(prefers-reduced-motion: reduce)').matches);
    const order = () => TALK_ANCHORS.needsYou(desks).map((d) => d.id);
    const deskOf = (id) => B.deskOf(page, room, id, term && term.id === id ? term.card : null);

    // Esc goes back to the office (or shuts the menu, the focus back on Menu); Tab stays inside the terminal; from the
    // terminal itself, outside the input, N is Next question, a digit picks a choice, and an arrow moves between choices.
    const keyed = (e) => {
      if (!term || e.defaultPrevented || e.isComposing || e.keyCode === 229) return;
      const t = e.target, inBox = t && (/^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName) || t.isContentEditable), mine = B.within(p.el, t) && !inBox;
      const asking = term.card && term.card.asking, headless = (room.session(term.id) || {}).headless === true;
      const n = B.digitOf(e), choice = mine && n && asking && !headless && (B.choicesOf(asking) || []).find((c) => c.n === n);
      const keys = /^Arrow/.test(e.key) && B.within(p.ask, t) ? [...p.ask.querySelectorAll('.talk-key')] : [];
      const to = B.stepOf(e.key, keys.indexOf(t), keys.length);
      if (e.key === 'Escape') {
        if (p.menu.hidden) api.close();
        else {
          p.menu.hidden = true;
          p.more.focus();
        }
      } else if (e.key === 'Tab' && !e.altKey && !e.ctrlKey && !e.metaKey) {
        if (!wrap(e.shiftKey, t)) return;
      } else if ((e.key === 'n' || e.key === 'N') && mine && !e.altKey && !e.ctrlKey && !e.metaKey) next();
      else if (choice) answer(term, asking, choice);
      else if (to !== null) keys[to].focus();
      else return;
      e.preventDefault();
      e.stopPropagation();
    };

    function build() {
      p = { dim: make('div', 'pt-dim'), el: make('div', 'pt'), menu: make('div', 'pt-menu talk-edge') };
      p.el.setAttribute('role', 'dialog');
      p.el.setAttribute('aria-modal', 'true');
      p.el.tabIndex = -1;
      p.live = make('div', 'talk-live');
      p.live.setAttribute('role', 'status');
      p.live.setAttribute('aria-live', 'polite');
      p.menu.setAttribute('role', 'menu');
      const head = make('div', 'pt-head');
      p.dot = make('span', 'talk-dot');
      p.name = make('span', 'talk-name');
      p.state = make('span', 'talk-state');
      p.next = key('go', '', next);
      // The menu is drawn as it opens, so what it offers is what's true now
      p.more = key('dark', 'Menu ▾', () => {
        p.menu.hidden = !p.menu.hidden;
        if (p.menu.hidden) return;
        drawMenu();
        p.menu.querySelector('.talk-key')?.focus();
      });
      p.more.setAttribute('aria-haspopup', 'menu');
      head.append(key('dark', 'Back to office', () => api.close()), key('dark', 'Shrink to bubble', shrink), p.dot, p.name, p.state,
        p.next, p.more, p.menu);
      p.body = make('div', 'pt-body');
      p.ask = make('div', 'talk-ask');
      p.panels = make('div', 'pt-panels');
      p.form = make('form', 'pt-input');
      p.input = make('textarea');
      p.input.rows = 3;
      p.input.maxLength = 2000;
      p.input.autocomplete = 'off';
      p.send = make('button', 'talk-key go', 'Send');
      p.send.type = 'submit';
      p.form.append(p.input, p.send);
      p.why = make('p', 'talk-why');
      p.problem = make('p', 'talk-problem');
      const main = make('div', 'pt-main');
      main.append(p.body, p.ask, p.panels, p.form, p.why, p.problem);
      p.picture = make('canvas', 'pt-picture');
      const office = key('pt-office', '', () => api.close());
      office.setAttribute('aria-label', 'Back to office');
      office.title = 'Back to office';
      office.append(p.picture);
      p.list = make('div', 'pt-list');
      const side = make('div', 'pt-side');
      side.append(office, make('div', 'pt-list-head', 'Desks'), p.list);
      const cols = make('div', 'pt-cols');
      cols.append(main, side);
      p.el.append(head, cols, p.live);
      p.form.addEventListener('submit', (e) => {
        e.preventDefault();
        say();
      });
      // A held Enter repeats: only the first press sends, and the rest add no new lines either
      p.input.addEventListener('keydown', (e) => {
        if (T.keyOf(e) !== 'send') return;
        e.preventDefault();
        if (!e.repeat) say();
      });
      doc.addEventListener('keydown', keyed);
      p.dim.hidden = p.el.hidden = true;
      doc.body.append(p.dim, p.el);
    }

    /**
     * Tab from the last key round to the first, Shift+Tab from the first to the last; in from outside, or from the
     * terminal itself. False: the page's own Tab, from key to key in between, so each is reached in order. A stop of the
     * page's that isn't a key (Chromium makes the conversation one, as it scrolls) steps as the page has it too, but
     * for one with no key after it (Shift+Tab: before it), round.
     */
    function wrap(back, t) {
      if (!p.el.querySelectorAll) return false;
      const all = [...p.el.querySelectorAll('button, textarea')].filter((el) => !el.disabled && el.getClientRects().length);
      const at = all.indexOf(t);
      if (at < 0 && t !== p.el && B.within(p.el, t)) {
        if (!t.compareDocumentPosition) return false;
        // 4: the key comes after it in the page (inside it too)
        const after = all.filter((el) => t.compareDocumentPosition(el) & 4).length;
        if (back ? after < all.length : after > 0) return false;
      }
      const to = T.wrapOf(all.length, at, back);
      if (to === null) return false;
      all[to].focus();
      return true;
    }

    /** The room behind grows toward the desk, or back. */
    function zoom(id) {
      const style = room.canvas.style, anchor = id && room.anchor(id);
      const z = id && T.zoomOf(anchor && anchor.css.tip, room.canvas.getBoundingClientRect(), reduced());
      style.transition = reduced() ? '' : `transform ${T.ZOOM_MS}ms steps(4, end), transform-origin ${T.ZOOM_MS}ms steps(4, end)`;
      if (z) style.transformOrigin = z.origin;
      style.transform = z ? `scale(${z.scale})` : '';
    }

    /** `id`'s desk in the terminal, from what was shown of it (`from`), or fresh. */
    function show(id, from = {}) {
      if (term) {
        win.clearTimeout(term.timer);
        drafts.set(term.id, p.input.value);
      }
      term = { id, card: from.card || null, items: from.items || [], before: from.before ?? null, size: from.size ?? null,
        loaded: !!(from.items && from.items.length), timer: 0, drawn: '', at: null, listed: '', boxes: new Map() };
      p.input.value = drafts.get(id) || '';
      p.menu.hidden = true;
      p.panels.replaceChildren();
      problem('');
      if (!p.el.hidden) zoom(id);
      render();
      look(term);
    }

    function render() {
      if (!term) return;
      const card = term.card, session = room.session(term.id) || {}, name = room.name(term.id), state = card ? card.state : session.state;
      const typed = B.within(p.form, doc.activeElement);
      p.el.setAttribute('aria-label', `${name}: conversation`);
      p.dot.className = `talk-dot ${state}`;
      p.name.textContent = name;
      p.state.textContent = B.STATES[state] || '';
      drawLines(false);
      drawAsk(session);
      const input = B.inputOf(card, name);
      p.form.hidden = !input.shown;
      B.lock(p.input, input.disabled);
      if (input.disabled || sending) B.offSend(doc, p.send, p.form.hidden ? p.el : p.input, { preventScroll: true });
      p.send.disabled = input.disabled || sending;
      p.input.placeholder = input.disabled ? input.placeholder : `${input.placeholder} (Shift+Enter: a new line)`;
      p.why.textContent = input.line;
      p.why.hidden = !input.line;
      p.more.hidden = !menu().length;
      // The form lost the focus all the same (hidden under it): back in the input (read-only now) or on the terminal,
      // never on a choice
      if (typed && !B.within(p.form, doc.activeElement)) (p.form.hidden ? p.el : p.input).focus({ preventScroll: true });
      drawSide();
      const desk = deskOf(term.id);
      for (const [panel, draw] of T.panels) {
        let box = term.boxes.get(panel);
        if (!box) {
          term.boxes.set(panel, box = make('section', 'pt-panel'));
          box.setAttribute('data-panel', panel);
          p.panels.append(box);
        }
        try {
          draw(box, desk);
        } catch (err) {
          win.console?.error?.(err);
        }
      }
    }

    /** The whole conversation, drawn again only when there's more of it; new lines followed only while its foot is in sight. */
    function drawLines(force) {
      const none = (room.session(term.id) || {}).conversation === false, items = term.items;
      const drawn = none ? 'none'
        : [items.length, items[0] && items[0].id, items.length && items[items.length - 1].id, term.before, !!term.loadingOlder, term.loaded].join();
      if (drawn === term.drawn && !force) return;
      const follow = !term.drawn || T.atFoot(p.body);
      term.drawn = drawn;
      const top = make('div', 'board-top');
      if (none) top.textContent = 'No conversation for this agent';
      else if (!items.length && !term.loaded) top.textContent = 'Loading…';
      else if (term.before !== null) {
        const older = key('dark', term.loadingOlder ? 'Loading…' : 'Older messages', olderPage);
        older.disabled = !!term.loadingOlder;
        top.append(older);
      } else top.textContent = items.length ? 'The start of the conversation' : 'Nothing written yet';
      p.body.replaceChildren(top, ...(none ? [] : items.map((i) => B.row(doc, i))));
      if (follow) p.body.scrollTop = p.body.scrollHeight;
    }

    /**
     * The desk's question, its choices by label where they can be offered, and the four keys under them (D4); a headless
     * run's, the question alone.
     */
    function drawAsk(session) {
      const asking = term.card && term.card.asking, at = asking ? `${asking.since} ${asking.pickable} ${session.headless}` : '';
      if (at === term.at) return;
      term.at = at;
      p.ask.hidden = !asking;
      if (!asking) return p.ask.replaceChildren();
      const shown = term, headless = session.headless === true, choices = headless ? null : B.choicesOf(asking);
      const row = (list) => {
        const keys = make('div', 'talk-keys');
        for (const choice of list) {
          const b = key(choice.n === 1 ? 'go' : '', '', () => answer(shown, asking, choice));
          if (choice.n) b.append(make('i', '', String(choice.n)));
          b.append(choice.label);
          if (choice.name) b.setAttribute('aria-label', choice.name);
          if (choice.description) b.title = choice.description;
          keys.append(b);
        }
        return keys;
      };
      const hint = choices ? 'Something else? Tap No, then type. Or move in its menu:'
        : 'Its menu can\'t be read from here: ▲ ▼ move, ✓ picks, ✕ says No.';
      p.ask.replaceChildren(make('div', 'talk-q', B.questionOf(asking)), ...(headless ? [] : [...(choices ? [row(choices)] : []),
        make('div', 'talk-hint', hint), row(B.KEYS)]));
    }

    /** Next question's count and the desk list, each drawn again only when it changed. */
    function drawSide() {
      const { count, next: to } = T.nextOf(order(), term.id);
      if (p.next.textContent !== `Next question ${count}`) p.next.replaceChildren('Next question ', make('i', '', String(count)));
      p.next.disabled = !to;
      const rows = T.listOf(desks, term.id, room.name), listed = rows.map((r) => `${r.id} ${r.state} ${r.name}`).join('|');
      if (listed === term.listed) return;
      term.listed = listed;
      p.list.replaceChildren(...(rows.length ? rows.map((r) => {
        const b = key(`pt-desk ${r.state}`, '', () => show(r.id));
        b.append(make('span', `talk-dot ${r.state}`), make('span', 'pt-desk-name', r.name),
          make('span', 'pt-desk-state', B.STATES[r.state] || ''));
        return b;
      }) : [make('div', 'pt-none', 'No other desks')]));
    }

    /** The menu: Open in Terminal where the page offers it (a setting), then the keys other files added. */
    function menu() {
      const id = term.id, offered = page.openInTerminal;
      const items = offered && offered.shown() ? [{ label: 'Open in Terminal', press: () => offered.open(id) }] : [];
      return [...items, ...B.keysFor(deskOf(id))];
    }

    function drawMenu() {
      p.menu.replaceChildren(...menu().map((item) => {
        const b = key('dark', item.label, () => {
          p.menu.hidden = true;
          try {
            item.press(deskOf(term.id));
          } catch (err) {
            win.console?.error?.(err);
          }
        });
        b.setAttribute('role', 'menuitem');
        if (item.title) b.title = item.title;
        return b;
      }));
    }

    /** The office's picture in the side: the room's own canvas, small. */
    function picture() {
      const from = room.canvas, to = p.picture, ctx = from.width && from.height && to.getContext && to.getContext('2d');
      if (!ctx) return;
      const w = T.PICTURE, h = Math.round((w * from.height) / from.width);
      if (to.width !== w * 2 || to.height !== h * 2) Object.assign(to, { width: w * 2, height: h * 2 });
      ctx.imageSmoothingEnabled = false;
      ctx.drawImage(from, 0, 0, to.width, to.height);
    }

    function problem(text) {
      p.problem.textContent = text;
      p.problem.hidden = !text;
    }

    function next() {
      const { next: to } = term ? T.nextOf(order(), term.id) : {};
      if (to) show(to);
    }

    function shrink() {
      const id = term && term.id;
      api.close();
      if (id) bubble.open(id);
    }

    /** A choice tapped, as in the bubble: a No wakes the input. */
    async function answer(shown, asking, choice) {
      page.tick?.();
      const { path, params } = B.sendOf(shown.id, asking, choice);
      const res = await page.post(path, params);
      if (term !== shown) return;
      if (!res) return page.flash?.('Can\'t reach your computer');
      if (!res.ok) page.flash?.(await B.said(res));
      else shown.card = B.answered(shown.card, choice);
      render();
      if (res.ok && choice.no) p.input.focus();
      look(shown);
    }

    /**
     * The input's words to the desk, with Return; several lines as one paste where it takes them; once, however often
     * Enter or Send is pressed while they're on their way (a refresh meanwhile doesn't wake Send). Sent, they leave the
     * box, and what was typed meanwhile stays; refused, or unanswered in SAY_WAIT, they stay, the reason under them.
     */
    async function say() {
      const shown = term;
      if (!shown || sending || p.input.readOnly) return;
      const words = p.input.value, text = words.trim();
      if (!text) return;
      page.tick?.();
      // Send clicked has the focus (in Chromium): into the input before it's greyed
      B.offSend(doc, p.send, p.input, { preventScroll: true });
      sending = p.send.disabled = true;
      let res;
      try {
        res = await page.post('/talk/say', { session: shown.id }, B.limit(win), { text, send: true });
      } finally {
        sending = false;
      }
      const sent = !!(res && res.ok), rest = (left) => (left.startsWith(words) ? left.slice(words.length).trimStart() : left);
      if (term !== shown) {
        // Another desk is shown now: what went isn't kept as this desk's draft
        if (sent && drafts.has(shown.id)) drafts.set(shown.id, rest(drafts.get(shown.id)));
        if (p) p.send.disabled = p.input.readOnly;
        return;
      }
      p.send.disabled = p.input.readOnly;
      if (sent) {
        p.input.value = rest(p.input.value);
        problem('');
        return look(shown);
      }
      problem(res ? await B.said(res) : 'Can\'t reach your computer.');
    }

    /** The desk's card again, and its newest messages when the transcript grew; then again in 2 or 3 seconds. */
    async function look(shown) {
      if (term !== shown) return;
      win.clearTimeout(shown.timer);
      const mine = shown.round = (shown.round || 0) + 1, current = () => term === shown && shown.round === mine;
      const res = await page.get('/talk', { session: shown.id });
      if (!current()) return;
      if (res && res.status === 404) return api.close();
      const card = res && res.ok ? await res.json().catch(() => null) : null;
      if (card && current()) shown.card = card;
      if (shown.card && shown.card.size !== shown.size && (room.session(shown.id) || {}).conversation !== false) await newest(shown);
      if (!current()) return;
      shown.loaded = true;
      render();
      if (!shown.focused) {
        shown.focused = true;
        // Without scrolling: the question's first line and the terminal's header stay in sight in a small window
        const first = p.ask.querySelector('.talk-key');
        (first || (p.input.readOnly ? p.el : p.input)).focus({ preventScroll: true });
      }
      shown.timer = win.setTimeout(() => look(shown), shown.card && shown.card.asking ? B.ASKING_EVERY : B.EVERY);
    }

    async function newest(shown) {
      const params = { session: shown.id };
      if (shown.size !== null && shown.items.length) params.size = shown.size;
      const res = await page.get('/conversation', params);
      const got = res && res.status === 200 ? await res.json().catch(() => null) : null;
      if (!got || term !== shown) return;
      const merged = B.merge(shown, got);
      const said = shown.loaded ? B.linesSaid(room.name(shown.id), shown.items, merged.items) : [];
      Object.assign(shown, { items: merged.items, before: merged.before, size: got.size });
      if (said.length) p.live.replaceChildren(...said.map((line) => make('p', '', line)));
    }

    /** Older messages: the page before what's shown, put above it, where you were kept in sight. */
    async function olderPage() {
      const shown = term;
      if (!shown || shown.before === null || shown.loadingOlder) return;
      const fromFoot = p.body.scrollHeight - p.body.scrollTop, kept = () => { p.body.scrollTop = p.body.scrollHeight - fromFoot; };
      shown.loadingOlder = true;
      drawLines(false);
      kept();
      const res = await page.get('/conversation', { session: shown.id, before: shown.before });
      shown.loadingOlder = false;
      if (term !== shown) return;
      const got = res && res.status === 200 ? await res.json().catch(() => null) : null;
      if (!got) problem(res ? await B.said(res) : 'Can\'t reach your computer.');
      else Object.assign(shown, T.older(shown, got));
      drawLines(true);
      kept();
    }

    const api = {
      /** The terminal on `id`'s desk, from what the bubble showed of it; false for a desk the room doesn't have. */
      open(id, from) {
        if (!room.session(id)) return false;
        if (!p) build();
        const was = !!term;
        show(id, from);
        if (!was) {
          p.dim.hidden = p.el.hidden = false;
          for (const el of [p.dim, p.el]) {
            el.classList.remove('enter');
            el.classList.add('enter');
          }
          win.setTimeout(() => p && [p.dim, p.el].forEach((el) => el.classList.remove('enter')), T.ZOOM_MS + 20);
          zoom(id);
          pictured = 0;
          picture();
          // The keys are the terminal's at once; its first choice or input takes the focus once its card is read
          p.el.focus({ preventScroll: true });
        }
        return true;
      },
      /** Back to the office: the terminal goes, the room comes back to its size, and the room has the focus again. */
      close() {
        if (!term) return;
        drafts.set(term.id, p.input.value);
        win.clearTimeout(term.timer);
        term = null;
        p.dim.hidden = p.el.hidden = p.menu.hidden = true;
        zoom(null);
        room.canvas.focus?.();
      },
      shown() {
        return !!term;
      },
      /** A frame of the room, with the bubbles it drew: the desks and Next question kept up, and the office's picture. */
      frame(bubbles) {
        // Upstairs or on the ride the room draws no bubbles: the list stays as it was rather than empty
        desks = (bubbles.length || !term ? bubbles : desks).filter((b) => room.session(b.id)).map((b) => ({ id: b.id, state: b.state, since: b.since }));
        if (!term) return;
        if (!room.session(term.id)) return api.close();
        const now = Date.now();
        if (now - pictured < T.PICTURE_EVERY) return;
        pictured = now;
        picture();
        drawSide();
      },
      remove() {
        api.close();
        if (!p) return;
        p.dim.remove();
        p.el.remove();
        doc.removeEventListener('keydown', keyed);
        p = null;
      },
    };
    return api;
  },
};
