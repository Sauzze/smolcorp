/**
 * The talk bubble (1.26, Talk Bubble), in the office's own window only. A click on the bubble over an agent's head
 * grows it into a small conversation: its last messages, its question with a button per choice, a one-line input. A
 * mouse resting on a bubble shows a one-line tag; each desk waiting on you shows a small question bubble, and a Needs
 * you key in the room's corner walks through them, longest waiting first. Its Expand opens the desk's whole
 * conversation in the Pixel Terminal (PIXEL_TERMINAL).
 *
 * The plain functions come first: what the bubble shows of a conversation, a prompt's choices, what a choice sends,
 * whether the input takes words, which key picks by its digit. The ▤ slate (deck.html) draws its rows and merges its
 * pages with the same `row` and `merge`. Then `create`, the live part: `page` is the network deck.html passes in
 * `on.talk` (get, post, flash, tick), `room` where each bubble is and who sits there. Nothing is ever typed into a
 * prompt: a choice is a labelled button, and words wait until the desk has stopped asking ("tap No, then type").
 */
const TALK_BUBBLE = {
  /** How often an open bubble looks again, in ms: while its desk asks, and otherwise (the slate's BOARD_EVERY). */
  ASKING_EVERY: 2000,
  EVERY: 3000,
  /** The newest rows an open bubble shows. */
  SHOWN: 4,
  /** A hover's tag and a small bubble's question, cut to this many characters. */
  TAG_MOST: 60,
  /** A small question bubble, in art pixels: narrower than two desks side by side (50 apart), so neighbours both fit. */
  SMALL: { w: 46, h: 16 },
  /** A small question bubble shows its keys only where it is this many CSS pixels wide: narrower, the keys would be cut, so it is the question alone (the open bubble has the keys). */
  SMALL_KEYS_LEAST: 110,
  /** The open bubble goes over the head where this many CSS pixels are free above it (and its own height, if more); else beside it. */
  ABOVE_LEAST: 260,
  /** Beside the head, the bubble may be narrowed to the room that side has, down to this many CSS pixels; under that, it goes over the head. */
  SIDE_LEAST: 220,
  /** Each state as a word: the status is never colour alone. */
  STATES: { thinking: 'working', asking: 'waiting for you', idle: 'idle', unread: 'done', error: 'stopped on an error' },
  /** The four keys, where a prompt's choices can't be offered by label: ✕ says No, as a permission's No does. */
  KEYS: [{ act: 'up', label: '▲', name: 'Up' }, { act: 'down', label: '▼', name: 'Down' },
    { act: 'accept', label: '✓', name: 'Pick' }, { act: 'reject', label: '✕', name: 'No', no: true }],
  /** The keys other files added (F-O, see PIXEL_TERMINAL): name → { label, title, shown(desk), press(desk) }. */
  added: new Map(),

  /**
   * A key from another file, on the open bubble's header and in the Pixel Terminal's menu: `press(desk)` what it does,
   * `shown(desk)`, if given, whether it's there for that desk; `desk` as deskOf gives it. Added again under the same
   * name, it takes the first one's place. Gives the name.
   */
  addKey(name, key) {
    TALK_BUBBLE.added.set(String(name), key);
    return name;
  },

  /** The added keys there for `desk`, in the order they were added. */
  keysFor(desk) {
    return [...TALK_BUBBLE.added].filter(([, key]) => {
      try {
        return !key.shown || key.shown(desk);
      } catch {
        return false;
      }
    }).map(([name, key]) => ({ ...key, name }));
  },

  /**
   * What an added key or panel is told of a desk: its id, name, card (GET /talk), a copy of its session, and the page's
   * get, post and flash. Those carry the window's key: a feature file is trusted code, reviewed as the core is.
   */
  deskOf(page, room, id, card) {
    const session = room.session(id);
    return { id, name: room.name(id), card: card || null, session: session ? { ...session } : null, get: page.get, post: page.post, flash: page.flash };
  },

  /** A conversation's items in the file's order. */
  byOrder(a, b) {
    return a.offset - b.offset || a.n - b.n;
  },

  /**
   * The newest page of /conversation merged into what's shown ({ items, before }): its new items added, or, after a
   * gap (a long time away), the page in place of what was there. `changed` says whether there's anything to draw.
   */
  merge(shown, page) {
    const known = new Set(shown.items.map((i) => i.id));
    const fresh = page.items.filter((i) => !known.has(i.id));
    const first = page.items[0], last = shown.items[shown.items.length - 1];
    const before = shown.items.length ? shown.before : page.before;
    if (first && last && TALK_BUBBLE.byOrder(first, last) > 0 && page.before !== null && !known.has(first.id)) {
      return { items: page.items, before: page.before, changed: true };
    }
    if (fresh.length) return { items: [...shown.items, ...fresh].sort(TALK_BUBBLE.byOrder), before, changed: true };
    return { items: shown.items, before, changed: false };
  },

  /** One item of the conversation as a row, on `doc`: who it's from over a peer's message or a question. */
  row(doc, item) {
    const row = doc.createElement('div');
    row.className = `board-row ${item.kind}`;
    if (item.kind === 'peer' || item.kind === 'question') {
      const from = doc.createElement('span');
      from.className = 'from';
      from.textContent = item.kind === 'question' ? 'Asked you' : item.from || 'Another session';
      row.append(from);
    }
    const prefix = { tool: '⚙︎ ', result: '↳ ', answer: '→ ' }[item.kind] || '';
    row.append(prefix + item.text);
    if (item.kind === 'tool' || item.kind === 'result' || item.kind === 'tools') row.title = item.text;
    if (item.more) {
      const more = doc.createElement('span');
      more.className = 'more';
      more.textContent = ' … more';
      row.append(more);
    }
    return row;
  },

  /** `text` on one line, cut to `most` characters. */
  cut(text, most) {
    const line = String(text || '').replace(/\s+/g, ' ').trim();
    return line.length > most ? `${line.slice(0, most - 1).trimEnd()}…` : line;
  },

  /** What a tool line did, in a few words: "changed cart.ts", "ran npm test". Its item is "Tool: what", from the tool. */
  didOf(item) {
    const text = String(item.text), at = text.indexOf(': ');
    const tool = item.from || (at > 0 ? text.slice(0, at) : text), what = at > 0 ? text.slice(at + 2) : '';
    const file = what.split(/[\\/]/).pop();
    if (['Edit', 'Write', 'MultiEdit', 'NotebookEdit'].includes(tool)) return file ? `changed ${file}` : 'changed a file';
    if (tool === 'Read') return file ? `read ${file}` : 'read a file';
    if (tool === 'Bash') return what ? `ran ${TALK_BUBBLE.cut(what, 32)}` : 'ran a command';
    if (tool === 'Grep' || tool === 'Glob') return 'searched';
    if (tool === 'WebFetch' || tool === 'WebSearch') return 'looked on the web';
    if (tool === 'Task' || tool === 'Agent') return 'asked a helper';
    return `used ${tool}`;
  },

  /**
   * What the open bubble shows of a conversation: its newest `most` rows, each run of tools (and their results) folded
   * into one dim line, "Changed cart.ts, ran npm test", the same thing done twice said once.
   */
  fold(items, most = TALK_BUBBLE.SHOWN) {
    const rows = [];
    for (const item of items) {
      const last = rows[rows.length - 1], run = last && last.kind === 'tools' ? last : null;
      if (item.kind === 'result') continue;
      if (item.kind !== 'tool') {
        rows.push(item);
        continue;
      }
      const did = TALK_BUBBLE.didOf(item);
      if (!run) rows.push({ id: item.id, kind: 'tools', did: [did] });
      else if (!run.did.includes(did)) run.did.push(did);
    }
    return rows.slice(-most).map((r) => {
      if (r.kind !== 'tools') return r;
      const said = r.did.slice(0, 3).join(', ') + (r.did.length > 3 ? `, and ${r.did.length - 3} more` : '');
      return { id: r.id, kind: 'tools', text: said.charAt(0).toUpperCase() + said.slice(1) };
    });
  },

  /** A desk's name: its nickname, else its role, else its project. */
  nameOf(session, nickname) {
    return nickname || (session.role && session.role.name) || session.project || session.name || 'Agent';
  },

  /** What a prompt asks, on one line: the question's text, or the tool it wants to use. */
  questionOf(asking) {
    const question = asking.kind === 'question' && asking.questions && asking.questions[0];
    if (question) return asking.questions.length > 1 ? `${question.question} (and ${asking.questions.length - 1} more)` : question.question;
    if (asking.kind === 'permission') return `Allow ${asking.label || asking.tool}?`;
    return 'It\'s asking you something.';
  },

  /**
   * A prompt's buttons, when its choices can be offered by label (`pickable`): a question's options by their numbers
   * and No (Escape, as ✕), or a permission's Yes and No (1 and 2, D4). Null where they can't: the four keys answer it.
   */
  choicesOf(asking) {
    if (!asking || !asking.pickable) return null;
    if (asking.kind === 'permission') return [{ n: 1, label: 'Yes' }, { n: 2, label: 'No', no: true }];
    const options = asking.questions[0].options.map((o) => ({ n: o.n, label: o.label, description: o.description }));
    return [...options, { act: 'reject', label: 'No', no: true }];
  },

  /** What a choice sends: a numbered one, POST /talk/pick with the prompt's `since`; a key, POST /act. */
  sendOf(id, asking, choice) {
    if (choice.act) return { path: '/act', params: { session: id, action: choice.act } };
    return { path: '/talk/pick', params: { session: id, n: String(choice.n), since: String(asking.since) } };
  },

  /** A desk's card once a choice went: after a No it no longer asks, so its input wakes for what to do instead. */
  answered(card, choice) {
    return card && choice.no ? { ...card, asking: null } : card;
  },

  /**
   * The input, from the desk's card: greyed while it asks ("Answer above first"); in its place, one plain line where
   * the window can't type into it at all (`typing.why`: a Mac terminal app's tab, headless, no terminal…); otherwise
   * "Tell <name> what to do", with a limit (one line at a time) said under it.
   */
  inputOf(card, name) {
    if (!card) return { shown: true, disabled: true, placeholder: 'Loading…', line: '' };
    if (card.asking) return { shown: true, disabled: true, placeholder: 'Answer above first', line: '' };
    const typing = card.typing || {};
    if (!typing.ok) return { shown: false, disabled: true, placeholder: '', line: typing.why || 'The bubble can\'t type here.' };
    return { shown: true, disabled: false, placeholder: `Tell ${name} what to do`, line: typing.why || '' };
  },

  /** The choice a key picks by its digit, 1 to 9: never one typed in a box, nor with a modifier held. */
  digitOf(e) {
    const target = e.target;
    if (target && (/^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName) || target.isContentEditable)) return null;
    if (e.altKey || e.ctrlKey || e.metaKey) return null;
    return /^[1-9]$/.test(e.key) ? Number(e.key) : null;
  },

  /** What the computer said to a refusal, short enough to read, or its status. */
  async said(res) {
    const text = (await res.text().catch(() => '')).trim();
    return text && text.length <= 200 ? text : `Your computer answered ${res.status}.`;
  },

  /**
   * The live bubbles on `win`'s page. `room`: { canvas, anchor(id), view(), shape(), session(id), name(id) }, from the
   * room. Gives { open(id, anchor), hover(id, anchor), expand(id), frame(bubbles), close(), remove() }: the room's click
   * and hover on a bubble, the Pixel Terminal on a desk (Expand, the ▤ key), each frame the room draws (the bubbles it
   * drew), a click on the floor, and the room leaving the page.
   */
  create(win, page, room) {
    const B = TALK_BUBBLE, doc = win.document;
    TALK_STYLE.inject(doc);
    const make = (tag, cls, text) => {
      const el = doc.createElement(tag);
      if (cls) el.className = cls;
      if (text !== undefined) el.textContent = text;
      return el;
    };
    const key = (choice, small) => {
      const b = make('button', `talk-key${choice.n === 1 ? ' go' : ''}`);
      b.type = 'button';
      if (choice.n && !small) b.append(make('i', '', String(choice.n)));
      b.append(choice.label);
      if (choice.name) b.setAttribute('aria-label', choice.name);
      if (choice.description) b.title = choice.description;
      return b;
    };
    const cards = new Map();   // each desk's last card (GET /talk), by id
    let asked = new Map();     // the waiting desks' prompts (GET /talk/asking), by id
    const smalls = new Map();  // the small question bubbles shown, by id: { el, drawn }
    let open = null;           // the open bubble: { id, card, items, before, size, loaded, timer, parts, at, drawn, fit, place }
    let askingNow = '', smallsAt = '', order = [], tagFor = null;
    const tag = make('div', 'talk-tag talk-edge');
    const needs = make('button', 'talk-needs talk-key go');
    needs.type = 'button';
    tag.hidden = needs.hidden = true;
    doc.body.append(tag, needs);
    // Expand: the Pixel Terminal, whose Shrink opens the same desk's bubble again
    const terminal = PIXEL_TERMINAL.create(win, page, room, { open: (id) => api.open(id) });
    // Needs you: the next waiting desk after the open one, longest waiting first.
    needs.addEventListener('click', () => {
      const next = order[(order.indexOf(open && open.id) + 1) % order.length];
      if (next && !(open && open.id === next)) api.open(next);
    });
    // Esc closes the open bubble wherever the focus is, but where the bubble said it already.
    const escaped = (e) => {
      if (open && e.key === 'Escape' && !e.defaultPrevented && !e.isComposing) api.close();
    };
    doc.addEventListener('keydown', escaped);
    const rect = () => room.canvas.getBoundingClientRect();
    const clamp = (v, low, high) => Math.max(low, Math.min(high, v));

    /** The prompt's question and its buttons, into `box`: the open bubble's, or a small one's (no hint, keys only if they fit). */
    function drawAsk(box, id, asking, small, width) {
      const choices = B.choicesOf(asking), session = room.session(id) || {};
      const shown = session.headless === true ? [] : choices || (small ? [] : B.KEYS);
      const fits = !small || (shown.length <= 3 && shown.reduce((n, c) => n + c.label.length + 3, 0) <= 24 && width >= B.SMALL_KEYS_LEAST);
      const keys = make('div', 'talk-keys');
      for (const choice of fits ? shown : []) {
        const b = key(choice, small);
        b.addEventListener('click', (e) => {
          e.stopPropagation();
          answer(id, asking, choice);
        });
        keys.append(b);
      }
      const q = make('div', small ? 'talk-small-q' : 'talk-q', small ? B.cut(B.questionOf(asking), B.TAG_MOST) : B.questionOf(asking));
      const hint = choices ? 'Something else? Tap No, then type.' : 'Its menu can\'t be read from here: ▲ ▼ move, ✓ picks, ✕ says No.';
      box.replaceChildren(q, ...(keys.children.length ? [keys] : []), ...(small || !shown.length ? [] : [make('div', 'talk-hint', hint)]));
    }

    /** A button tapped: the choice goes, and a No wakes the input. */
    async function answer(id, asking, choice) {
      page.tick?.();
      const { path, params } = B.sendOf(id, asking, choice);
      const res = await page.post(path, params);
      if (!res) return page.flash?.('Can\'t reach your computer');
      if (!res.ok) {
        page.flash?.(await B.said(res));
        askingNow = ''; // asked again: it may ask something else now
      } else if (choice.no) {
        cards.set(id, B.answered(cards.get(id), choice));
        asked.delete(id);
        smallsAt = '';
      }
      if (open && open.id === id) {
        if (res.ok) open.card = B.answered(open.card, choice);
        render();
        if (res.ok && choice.no) open.parts.input.focus();
        look(open);
      }
    }

    function build() {
      const el = make('div', 'talk-bubble talk-edge grow');
      el.setAttribute('role', 'dialog');
      el.tabIndex = -1;
      const head = make('div', 'talk-head'), dot = make('span', 'talk-dot'), name = make('span', 'talk-name'), state = make('span', 'talk-state');
      const added = make('span', 'talk-added'), expand = make('button', 'talk-key dark', 'Expand'), esc = make('button', 'talk-key dark', 'Esc');
      expand.type = esc.type = 'button';
      expand.addEventListener('click', () => open && api.expand(open.id));
      esc.addEventListener('click', () => api.close());
      head.append(dot, name, state, added, expand, esc);
      const form = make('form', 'talk-input'), input = make('input'), send = make('button', 'talk-key go', 'Send');
      input.type = 'text';
      input.maxLength = 2000;
      input.autocomplete = 'off';
      send.type = 'submit';
      form.append(input, send);
      const parts = { el, dot, name, state, added, lines: make('div', 'talk-lines'), ask: make('div', 'talk-ask'), form, input, send,
        why: make('p', 'talk-why'), problem: make('p', 'talk-problem'), tail: make('div', 'talk-tail') };
      el.append(head, parts.lines, parts.ask, form, parts.why, parts.problem, parts.tail);
      form.addEventListener('submit', (e) => {
        e.preventDefault();
        say();
      });
      el.addEventListener('keydown', (e) => {
        const n = B.digitOf(e), choice = n && open && (B.choicesOf(open.card && open.card.asking) || []).find((c) => c.n === n);
        if (e.isComposing || e.keyCode === 229) return;
        if (e.key === 'Escape') api.close();
        else if (choice) answer(open.id, open.card.asking, choice);
        else return;
        e.preventDefault();
        e.stopPropagation();
      });
      win.setTimeout(() => el.classList.remove('grow'), 140);
      doc.body.append(el);
      return parts;
    }

    /** The open bubble drawn from its card and items; each part only when it changed, so a focused button stays. */
    function render() {
      if (!open) return;
      const { parts, card } = open, session = room.session(open.id) || {}, name = room.name(open.id);
      const state = card ? card.state : session.state;
      let grew = false;   // whether anything that changes the bubble's height was drawn, for place() to measure it again
      parts.el.setAttribute('aria-label', `${name}: conversation`);
      parts.dot.className = `talk-dot ${state}`;
      parts.name.textContent = name;
      parts.state.textContent = B.STATES[state] || '';
      // The keys other files added, drawn again only when which are there changed
      const id = open.id, keys = B.keysFor(B.deskOf(page, room, id, card)), names = keys.map((k) => k.name).join(' ');
      if (names !== open.added) {
        open.added = names;
        parts.added.replaceChildren(...keys.map((k) => {
          const b = make('button', 'talk-key dark', k.label);
          b.type = 'button';
          if (k.title) b.title = k.title;
          b.addEventListener('click', (e) => {
            e.stopPropagation();
            try {
              // The key as it's registered now: added again under the same name, the new one presses
              (B.added.get(k.name) || k).press(B.deskOf(page, room, id, open && open.id === id ? open.card : null));
            } catch (err) {
              win.console?.error?.(err);
            }
          });
          return b;
        }));
      }
      const rows = session.conversation === false ? null : B.fold(open.items);
      const drawn = rows ? rows.map((r) => r.id + (r.text || '').length).join() + open.loaded : 'none';
      if (drawn !== open.drawn) {
        open.drawn = drawn;
        grew = true;
        const none = !rows ? 'No conversation for this agent' : !rows.length ? (open.loaded ? 'Nothing written yet' : 'Loading…') : '';
        parts.lines.replaceChildren(...(none ? [make('div', 'talk-none', none)] : rows.map((r) => B.row(doc, r))));
        parts.lines.scrollTop = parts.lines.scrollHeight;
      }
      const asking = card && card.asking;
      const at = asking ? `${asking.since} ${asking.pickable} ${session.headless}` : '';
      if (at !== open.at) {
        open.at = at;
        grew = true;
        parts.ask.hidden = !asking;
        if (asking) drawAsk(parts.ask, open.id, asking, false);
        else parts.ask.replaceChildren();
      }
      const input = B.inputOf(card, name);
      parts.form.hidden = !input.shown;
      parts.input.disabled = parts.send.disabled = input.disabled;
      parts.input.placeholder = input.placeholder;
      parts.why.textContent = input.line;
      parts.why.hidden = !input.line;
      const form = `${input.shown} ${input.line}`;
      if (form !== open.form) grew = true;
      open.form = form;
      if (grew) {
        open.fit += 1;
        place();
      }
    }

    function problem(text) {
      if (!open) return;
      open.parts.problem.textContent = text;
      open.parts.problem.hidden = !text;
      open.fit += 1;
    }

    /** The input's words to the desk, with Return; refused, they stay in the box with the reason under it. */
    async function say() {
      const shown = open;
      if (!shown || shown.parts.input.disabled) return;
      const text = shown.parts.input.value.trim();
      if (!text) return;
      page.tick?.();
      shown.parts.send.disabled = true;
      const res = await page.post('/talk/say', { session: shown.id }, undefined, { text, send: true });
      if (open !== shown) return;
      shown.parts.send.disabled = false;
      if (res && res.ok) {
        shown.parts.input.value = '';
        problem('');
        return look(shown);
      }
      problem(res ? await B.said(res) : 'Can\'t reach your computer.');
    }

    /** The open desk's card again, and its newest messages when the transcript grew; then again in 2 or 3 seconds. */
    async function look(shown) {
      if (!shown || open !== shown) return;
      win.clearTimeout(shown.timer);
      // Only the newest look goes on to set the next timer: one asked for while another is in flight takes over from it
      const mine = shown.round = (shown.round || 0) + 1, current = () => open === shown && shown.round === mine;
      const res = await page.get('/talk', { session: shown.id });
      if (!current()) return;
      if (res && res.status === 404) return api.close();
      const card = res && res.ok ? await res.json().catch(() => null) : null;
      if (card && current()) cards.set(shown.id, shown.card = card);
      if (shown.card && shown.card.size !== shown.size && (room.session(shown.id) || {}).conversation !== false) await newest(shown);
      if (!current()) return;
      shown.loaded = true;
      render();
      if (!shown.focused) {
        shown.focused = true;
        const first = shown.parts.ask.querySelector('.talk-key');
        (first || (shown.parts.input.disabled ? shown.parts.el : shown.parts.input)).focus();
      }
      shown.timer = win.setTimeout(() => look(shown), shown.card && shown.card.asking ? B.ASKING_EVERY : B.EVERY);
    }

    async function newest(shown) {
      const params = { session: shown.id };
      if (shown.size !== null && shown.items.length) params.size = shown.size;
      const res = await page.get('/conversation', params);
      const got = res && res.status === 200 ? await res.json().catch(() => null) : null;
      if (!got || open !== shown) return;
      const merged = B.merge(shown, got);
      Object.assign(shown, { items: merged.items, before: merged.before, size: got.size });
    }

    /**
     * The open bubble over its head, its tail on the head's bubble, inside the room: above it where it fits there, else
     * beside it (right, or left near the room's edge), its tail pointing across at the head. Beside, it never leaves
     * the room: where neither side has its full width it is narrowed to the larger side's, and where that is too
     * narrow too it goes over the head, kept inside, with no tail. Its height is the bubble's own, measured (capped at
     * the room's): it sits as high as it needs to be whole, and a bubble taller than the room scrolls inside its box
     * (see the style) with its input kept at the foot.
     */
    function place() {
      const anchor = open && room.anchor(open.id);
      if (!open) return;
      open.parts.el.hidden = !anchor;
      if (!anchor) return;
      const r = rect(), tip = anchor.css.tip, box = anchor.css.box;
      const at = [Math.round(tip.x), Math.round(tip.y), Math.round(box.top), r.left, r.top, r.width, r.height, win.innerHeight, open.fit].join();
      if (at === open.place) return;
      open.place = at;
      const el = open.parts.el, style = el.style, tail = open.parts.tail.style;
      // The bubble's own height at a width, as it would be with nothing holding it in
      const natural = (width) => {
        Object.assign(style, { width: `${width}px`, maxHeight: 'none' });
        return Math.min(el.offsetHeight, r.height - 8);
      };
      const above = box.top - 10 - (r.top + 4);
      let w = Math.min(320, r.width - 8), h = natural(w), way = 'above';
      if (above < Math.max(h, B.ABOVE_LEAST)) {
        const free = { right: r.right - 4 - (tip.x + 20), left: tip.x - 20 - (r.left + 4) };
        way = free.right >= w ? 'right' : free.left >= w ? 'left' : free.right >= free.left ? 'right' : 'left';
        if (free[way] < w) {
          if (free[way] >= B.SIDE_LEAST) w = Math.floor(free[way]);
          else way = 'over';
        }
        h = natural(w);
      }
      const side = way === 'right' || way === 'left';
      const centred = clamp(tip.x - w / 2, r.left + 4, r.right - 4 - w);
      const left = Math.round(way === 'right' ? tip.x + 20 : way === 'left' ? tip.x - 20 - w : centred);
      const top = Math.round(clamp(tip.y - 48, r.top + 4, r.bottom - 4 - h));
      Object.assign(style, { left: `${left}px`, width: `${w}px`, maxHeight: `${Math.round(way === 'above' ? above : r.bottom - 4 - top)}px` });
      tail.display = way === 'over' ? 'none' : '';
      if (way === 'above') {
        Object.assign(style, { bottom: `${Math.round(win.innerHeight - box.top + 10)}px`, top: '' });
        Object.assign(tail, { left: `${Math.round(clamp(tip.x - left - 7, 6, w - 20))}px`, top: 'calc(100% + 2px)', transform: '' });
      } else {
        Object.assign(style, { top: `${top}px`, bottom: '' });
        if (side) Object.assign(tail, { left: way === 'right' ? '-13px' : 'calc(100% - 1px)', top: `${Math.round(tip.y - top - 4)}px`,
          transform: `rotate(${way === 'right' ? 90 : -90}deg)` });
      }
    }

    /** Every waiting desk's prompt, when who is waiting changed. */
    async function askAll() {
      const res = await page.get('/talk/asking');
      const got = res && res.ok ? await res.json().catch(() => null) : null;
      if (!got) return;
      asked = new Map(got.asking.map((row) => [row.session, row]));
      smallsAt = '';
    }

    /** The small question bubbles that fit, the rest left to the room's "?", and the Needs you key's count. */
    function layout(waiting) {
      const view = room.view(), shape = room.shape(), r = rect(), cog = room.cog && room.cog();
      const list = waiting.filter((b) => asked.has(b.id) && !(open && open.id === b.id));
      const at = JSON.stringify([list.map((b) => [b.id, b.x, b.top, asked.get(b.id).since]), view, cog, r.left, r.top, waiting.length, open && open.id]);
      if (at === smallsAt) return;
      smallsAt = at;
      order = TALK_ANCHORS.needsYou(waiting).map((b) => b.id);
      needs.replaceChildren('Needs you ', make('i', '', String(order.length)));
      needs.hidden = !order.length;
      const px = view.scale / view.k;
      const css = (x, y) => ({ x: view.left + (view.ox + x * view.scale) / view.k, y: view.top + (view.oy + y * view.scale) / view.k });
      // In the room's top corner, right of the settings cog, level with it: from where the room drew the cog (the room
      // is letterboxed in a small window, so neither the canvas's corner nor the plan's origin is the room's corner).
      // With no cog drawn, the canvas's corner.
      if (cog) {
        const end = css(cog.x + cog.w, cog.y), foot = css(cog.x, cog.y + cog.h);
        Object.assign(needs.style, { left: `${Math.round(end.x + clamp(12 * px, 8, 24))}px`, top: `${Math.round((end.y + foot.y - (needs.offsetHeight || 28)) / 2)}px` });
      } else Object.assign(needs.style, { left: `${Math.round(r.left + 76)}px`, top: `${Math.round(r.top + 16)}px` });
      const { small } = TALK_ANCHORS.layoutSmall(list, B.SMALL, shape);
      const kept = new Set(small.map((s) => s.id));
      for (const [id, s] of smalls) if (!kept.has(id)) { s.el.remove(); smalls.delete(id); }
      for (const { id, box } of small) {
        let s = smalls.get(id);
        if (!s) {
          s = { el: make('div', 'talk-small talk-edge'), drawn: '' };
          s.el.addEventListener('click', () => api.open(id));
          doc.body.append(s.el);
          smalls.set(id, s);
        }
        const from = css(box.left, box.top), asking = asked.get(id);
        Object.assign(s.el.style, { left: `${from.x}px`, top: `${from.y}px`, width: `${B.SMALL.w * px}px`, height: `${B.SMALL.h * px}px` });
        s.el.style.setProperty('--talk-font', `${Math.round(clamp(px * 4.5, 9, 14))}px`);
        const wide = B.SMALL.w * px, drawn = `${asking.since} ${asking.pickable} ${wide >= B.SMALL_KEYS_LEAST}`;
        if (drawn !== s.drawn) {
          s.drawn = drawn;
          drawAsk(s.el, id, asking, true, wide);
        }
      }
    }

    function showTag(id, anchor) {
      const card = cards.get(id), row = asked.get(id);
      const asking = (card && card.asking) || row;
      const text = asking ? B.questionOf(asking) : card ? card.last || B.STATES[card.state] || '' : '…';
      tagFor = id;
      tag.textContent = B.cut(text, B.TAG_MOST);
      tag.hidden = !tag.textContent;
      const r = rect();
      tag.style.left = `${Math.round(clamp(anchor.css.tip.x - tag.offsetWidth / 2, r.left + 4, r.right - 4 - tag.offsetWidth))}px`;
      tag.style.top = `${Math.round(Math.max(r.top + 4, anchor.css.box.top - tag.offsetHeight - 6))}px`;
    }

    /** Under the terminal, none of the small bubbles, Needs you or a tag: laid out again once it goes. */
    function under() {
      for (const s of smalls.values()) s.el.remove();
      smalls.clear();
      smallsAt = '';
      tag.hidden = needs.hidden = true;
    }

    const api = {
      /** The bubble over `id`'s head opened, or, open already, closed (a click on the same bubble). */
      open(id) {
        if (open && open.id === id) return api.close();
        api.close();
        terminal.close();
        page.tick?.();
        api.hover(null);
        open = { id, card: cards.get(id) || null, items: [], before: null, size: null, loaded: false, timer: 0, fit: 0 };
        open.parts = build();
        problem('');
        smallsAt = '';
        place();
        render();
        look(open);
      },
      /** The mouse rests on `id`'s bubble (null: on none): its tag, and its card asked for again when it may be. */
      hover(id, anchor) {
        tagFor = null;
        tag.hidden = true;
        if (id === null || !anchor || (open && open.id === id) || terminal.shown()) return;
        showTag(id, anchor);
        if (!anchor.again) return;
        page.get('/talk', { session: id }).then(async (res) => {
          const card = res && res.ok ? await res.json().catch(() => null) : null;
          if (card) cards.set(id, card);
          if (card && tagFor === id) showTag(id, anchor);
        });
      },
      /**
       * The Pixel Terminal on `id`'s desk, from the open bubble at the same place in its conversation (or fresh), the
       * bubble closed behind it; false for a desk the room doesn't have.
       */
      expand(id) {
        const from = open && open.id === id ? { card: open.card, items: open.items, before: open.before, size: open.size } : { card: cards.get(id) || null };
        if (!terminal.open(id, from)) return false;
        api.close();
        api.hover(null);
        under();
        return true;
      },
      /** A frame of the room, with the bubbles it drew: the open bubble follows its head; the small ones are laid out. */
      frame(bubbles) {
        terminal.frame(bubbles);
        if (terminal.shown()) return under();
        if (open && !room.session(open.id)) api.close();
        place();
        const waiting = bubbles.filter((b) => b.state === 'asking');
        const now = waiting.map((b) => b.id).sort().join(' ');
        if (now !== askingNow) {
          askingNow = now;
          if (waiting.length) askAll();
          else asked = new Map();
        }
        layout(waiting);
      },
      close() {
        if (!open) return;
        const shown = open;
        open = null;
        smallsAt = '';
        win.clearTimeout(shown.timer);
        shown.parts.el.classList.add('shrink');
        win.setTimeout(() => shown.parts.el.remove(), 120);
      },
      /** The room closed (its stream broke): nothing of the talk layers stays over the page. The next frame draws them again. */
      away() {
        api.close();
        terminal.close();
        for (const s of smalls.values()) s.el.remove();
        smalls.clear();
        asked = new Map();
        askingNow = smallsAt = '';
        tagFor = null;
        tag.hidden = needs.hidden = true;
      },
      remove() {
        api.close();
        terminal.remove();
        for (const s of smalls.values()) s.el.remove();
        smalls.clear();
        tag.remove();
        needs.remove();
        doc.removeEventListener('keydown', escaped);
      },
    };
    return api;
  },
};
