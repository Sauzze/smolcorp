/*
 * The first-run guide: a pixel arrow on the one control to use next, and a
 * short bubble saying what to do, step by step. Each step moves on by what you
 * do (the folder saved, the checks green, the hire sent), never by a Next
 * button alone. The rest of the page is dimmed, but nothing is covered: every
 * control still works, the step's own most of all. Skip ends it; Esc only puts
 * it away until the page opens again.
 *
 * No dependency, and nothing of yours: the page that uses it gives the steps
 * and remembers when it's over (setup.html, "the first-run guide"). Its plain
 * functions, `pick` and `place`, are exported for the tests.
 */
(function (root) {
  'use strict';

  /** One pixel of the art, in CSS pixels. */
  const PIXEL = 3;
  /**
   * The arrow, pointing down, drawn as the office draws: k its edge, y its
   * fill, h its lit side. Three colours, crisp, no smoothing.
   */
  const ARROW = [
    '...kkkkk...',
    '...khyyk...',
    '...khyyk...',
    '...khyyk...',
    '...khyyk...',
    'kkkkhyykkkk',
    '.khhyyyyyk.',
    '..khyyyyk..',
    '...khyyk...',
    '....kyk....',
    '.....k.....',
  ];
  const COLOURS = { k: '#1b1e2a', y: '#ffb000', h: '#ffe7a3' };
  /** How long a step's control may be missing before the guide looks past it, in ms. */
  const GRACE = 2500;
  /** Between the arrow and what it points at, and from the window's edges. */
  const GAP = 4;
  const MARGIN = 12;
  /** The light round what's to be used, beyond its box. */
  const PAD = 6;
  /** The ring round what's lit: two pixels of the art, kept inside the window. */
  const RING = PIXEL * 2;

  const clamp = (value, low, high) => Math.min(Math.max(value, low), Math.max(low, high));

  /**
   * Which step to show. Each of `steps` says how it is: { done, shown (its
   * control is on screen), away (it's in another part of the page), missingFor
   * (ms its control has been nowhere), patience (how long that may last before
   * it's looked past, GRACE if not said), optional (shown only when its control
   * is there, never waited for), waiting (it has words for while its control is
   * waited for) }. The first step not done is shown; one whose
   * control has been missing longer than its patience is looked past, for the
   * next one that's there, so a missing control never holds the guide up.
   * `index` -1 is nothing to show for now; `finished`, every step done, or the
   * last, which says you're there. `lost`: every step before the last is done
   * but the last one's control never came, so it's shown docked, with words of
   * its own, rather than the guide running on unseen. `waiting`: the step's
   * control is still waited for, and it says why meanwhile, docked.
   */
  function pick(steps) {
    if (!steps.length || steps[steps.length - 1].done || steps.every((step) => step.done)) return { index: -1, finished: true };
    const last = steps.length - 1;
    for (let i = 0; i < steps.length; i++) {
      const step = steps[i];
      if (step.done) continue;
      if (step.shown || step.away) return { index: i, finished: false };
      if (step.optional) continue;
      if (!(step.missingFor >= (step.patience || GRACE))) return step.waiting ? { index: i, finished: false, waiting: true } : { index: -1, finished: false };
      if (i === last && steps.slice(0, last).every((before) => before.done || before.optional)) return { index: i, finished: false, lost: true };
    }
    return { index: -1, finished: false };
  }

  /**
   * Where the arrow and the bubble go, for a control whose box in the window
   * is `box` ({ top, left, width, height }), lit with the box `around` (the
   * control's, if not given), in a window `view` ({ width, height }), with a
   * bubble and an arrow of the sizes given: above what's lit, pointing down,
   * where there's room, else below it, pointing up, so neither covers what's
   * lit; centred on the control; both kept inside the window. A control out of
   * sight gets the arrow at the edge nearest it.
   */
  function place(box, view, bubble, arrow, around = box) {
    const bottom = around.top + around.height;
    const need = arrow.height + GAP + bubble.height + MARGIN;
    const above = around.top, below = view.height - bottom;
    const down = above >= need || (below < need && above > below);
    const centre = clamp(box.left + box.width / 2, MARGIN + arrow.width / 2, view.width - MARGIN - arrow.width / 2);
    let y = down ? around.top - GAP - arrow.height : bottom + GAP;
    y = clamp(y, MARGIN + (down ? bubble.height : 0), view.height - MARGIN - arrow.height - (down ? 0 : bubble.height));
    return {
      down,
      arrow: { x: Math.round(centre - arrow.width / 2), y: Math.round(y) },
      bubble: {
        x: Math.round(clamp(centre - bubble.width / 2, MARGIN, view.width - MARGIN - bubble.width)),
        y: Math.round(down ? y - bubble.height : y + arrow.height),
      },
    };
  }

  const STYLE = `
.smol-guide { position: fixed; inset: 0; z-index: 2147483000; pointer-events: none; }
.smol-guide[hidden] { display: none !important; }
.smol-guide-shade { position: fixed; border-radius: 6px; pointer-events: none;
  box-shadow: 0 0 0 ${PIXEL}px ${COLOURS.y}, 0 0 0 ${PIXEL * 2}px ${COLOURS.k}, 0 0 0 200vmax rgb(0 0 0 / .38); }
.smol-guide-arrow { position: fixed; left: 0; top: 0; pointer-events: none; }
.smol-guide-arrow.up { transform: rotate(180deg); }
.smol-guide-arrow svg { display: block; shape-rendering: crispEdges; animation: smol-guide-bob 1.1s steps(1, end) infinite; }
@keyframes smol-guide-bob { 0% { transform: translateY(0); } 50% { transform: translateY(${PIXEL}px); } }
.smol-guide-bubble { position: fixed; left: 0; top: 0; margin: ${PIXEL}px; pointer-events: auto; box-sizing: border-box;
  width: max-content; max-width: min(320px, calc(100vw - ${MARGIN * 2 + PIXEL * 2}px)); padding: 10px 12px;
  background: #f4f1ea; color: ${COLOURS.k}; border: 0; border-radius: 0;
  box-shadow: 0 -${PIXEL}px 0 0 ${COLOURS.k}, 0 ${PIXEL}px 0 0 ${COLOURS.k}, -${PIXEL}px 0 0 0 ${COLOURS.k}, ${PIXEL}px 0 0 0 ${COLOURS.k},
    0 ${PIXEL * 2}px 0 0 rgb(0 0 0 / .25);
  font: 500 13px/1.4 -apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", system-ui, sans-serif; text-align: left; }
.smol-guide-count { font-size: 11px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase; color: #5b5f6e; margin-bottom: 2px; }
.smol-guide-text { margin: 0; overflow-wrap: anywhere; }
.smol-guide-text b { font-weight: 700; white-space: nowrap; }
.smol-guide-tools { display: flex; gap: 10px; align-items: center; justify-content: flex-end; margin-top: 8px; }
.smol-guide-tools button { font: inherit; font-size: 12px; font-weight: 700; cursor: pointer; border: 0; border-radius: 0; margin: 0; }
.smol-guide-go { background: ${COLOURS.k}; color: #f4f1ea; padding: 4px 10px; }
.smol-guide-skip { background: none; color: ${COLOURS.k}; padding: 4px 2px; text-decoration: underline; text-underline-offset: 2px; }
.smol-guide-tools button:focus-visible { outline: ${PIXEL}px solid ${COLOURS.y}; outline-offset: 2px; }
.smol-guide.docked .smol-guide-shade, .smol-guide.docked .smol-guide-arrow { display: none; }
.smol-guide.docked .smol-guide-bubble { left: 50% !important; top: auto !important; bottom: ${MARGIN + PIXEL}px; transform: translateX(-50%); }
.smol-guide-room { pointer-events: none; }
@media (prefers-reduced-motion: reduce) { .smol-guide-arrow svg { animation: none; } }
`;

  /** The arrow as an SVG: a rect for each run of one colour along a row. */
  function arrowSVG(doc) {
    const ns = 'http://www.w3.org/2000/svg';
    const svg = doc.createElementNS(ns, 'svg');
    const width = ARROW[0].length, height = ARROW.length;
    svg.setAttribute('viewBox', `0 0 ${width} ${height}`);
    svg.setAttribute('width', String(width * PIXEL));
    svg.setAttribute('height', String(height * PIXEL));
    svg.setAttribute('aria-hidden', 'true');
    ARROW.forEach((row, y) => {
      for (let x = 0; x < row.length;) {
        const colour = COLOURS[row[x]];
        let end = x + 1;
        while (end < row.length && row[end] === row[x]) end++;
        if (colour) {
          const rect = doc.createElementNS(ns, 'rect');
          for (const [name, value] of [['x', x], ['y', y], ['width', end - x], ['height', 1], ['fill', colour]]) rect.setAttribute(name, String(value));
          svg.append(rect);
        }
        x = end;
      }
    });
    return svg;
  }

  const FOCUSABLE = 'button:not([disabled]), a[href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';
  const TYPING = 'input, textarea, select, [contenteditable=""], [contenteditable="true"]';

  /**
   * The guide, for `options.steps`, each { id, target (a selector, an element
   * or a function giving one), area (what's lit, if more than the target),
   * say, press (the control's name, in bold after an arrow), button (a label:
   * pressing it counts as done), ackOnClick (a click on it counts as done:
   * only for a step whose click is all it asks), done (a function: whether
   * what it asks is done), away (whether it's in a part of the page not shown
   * now), reveal (shows that part), optional and patience (see `pick`), lost
   * (what it says, docked, when it's the last and its control never came),
   * wait (a function: what it says, docked, while its control is waited for,
   * or nothing; its patience counts from when it starts saying it),
   * doneTarget and doneSay (where it points and what it says when it's shown
   * done, in a tour), n (its number) }.
   * `options.total` is how many steps the count says; `options.modals`, a
   * selector for what covers the page besides a <dialog>, under which the guide
   * waits; `options.awayText`, what the bubble says while the step is
   * elsewhere; `options.onFinish(reason)`, called once it's over: "done",
   * "skipped" (Skip), or "hidden" (Esc: put away until the page opens again).
   */
  function create(options) {
    const doc = options.document || root.document;
    const win = doc.defaultView || root;
    const steps = options.steps;
    const reduced = () => !!(win.matchMedia && win.matchMedia('(prefers-reduced-motion: reduce)').matches);

    if (!doc.getElementById('smol-guide-style')) {
      const style = doc.createElement('style');
      style.id = 'smol-guide-style';
      style.textContent = STYLE;
      doc.head.append(style);
    }
    const make = (tag, className) => Object.assign(doc.createElement(tag), { className });
    const layer = make('div', 'smol-guide');
    layer.hidden = true;
    const shade = make('div', 'smol-guide-shade');
    const arrow = make('div', 'smol-guide-arrow');
    arrow.append(arrowSVG(doc));
    const bubble = make('div', 'smol-guide-bubble');
    bubble.setAttribute('role', 'group');
    bubble.setAttribute('aria-label', 'Guide');
    const count = make('div', 'smol-guide-count');
    const text = make('p', 'smol-guide-text');
    text.id = 'smol-guide-text';
    text.setAttribute('aria-live', 'polite');
    const tools = make('div', 'smol-guide-tools');
    const skip = Object.assign(make('button', 'smol-guide-skip'), { type: 'button', textContent: 'Skip' });
    skip.setAttribute('aria-label', 'Skip the guide');
    const go = Object.assign(make('button', 'smol-guide-go'), { type: 'button' });
    tools.append(skip, go);
    bubble.append(count, text, tools);
    layer.append(shade, arrow, bubble);
    /** Room at the foot of the page while the bubble is docked there, so it never sits on a pane's last row. */
    const room = make('div', 'smol-guide-room');
    room.setAttribute('aria-hidden', 'true');

    let running = false, tour = false;
    let timer = 0, frame = 0;
    /** The steps counted as done by a click or their button (in a tour, every step passed), and since when each control has been missing. */
    const acked = new Set();
    const missingSince = new Map();
    /** The steps saying why they wait: each one's patience counts from when it started to. */
    const waiting = new Set();
    /**
     * What's shown: the step, how ("here", "away" in another part of the page, "waiting" for its control,
     * docked with its own words, or "lost", docked as the last), whether it was done when it came, its control, and the control that was given the guide's
     * words. Kept while the guide is hidden under a dialog, so it doesn't count as new when it's back.
     */
    let shown = -1, shownHow = '', shownDone = false, shownTarget = null, described = null;
    /**
     * The step the keyboard and the scroll were last brought to (ADOPT: the next one shown is taken as
     * where you already are); `settle` is set when the next one shown should be brought to all the same.
     */
    const ADOPT = -2;
    let settled = -1, settle = false;

    const call = (fn, fallback) => { try { return fn ? fn() : fallback; } catch { return fallback; } };
    const find = (target) => {
      const found = typeof target === 'function' ? call(target, null) : target;
      return typeof found === 'string' ? doc.querySelector(found) : found || null;
    };
    const visible = (el) => !!el && el.isConnected && el.getClientRects().length > 0 && win.getComputedStyle(el).visibility !== 'hidden';
    const modalsOpen = () => [...doc.querySelectorAll('dialog[open]' + (options.modals ? `, ${options.modals}` : ''))].filter(visible);
    /** Something covers the page that the step's control isn't in: the guide waits for it to close. */
    const covered = (target) => modalsOpen().some((modal) => !target || !modal.contains(target));
    const patience = (step) => call(step.patience, 0) || GRACE;

    /**
     * How each step is now. On a first run a step is done when what it asks is. In a tour every step is
     * walked, done or not: one is passed by its Next, by being done while it's shown, or by being nowhere
     * (all but the last, which then says where to look).
     */
    function look() {
      const now = Date.now();
      const last = steps.length - 1;
      return steps.map((step, i) => {
        const already = !!call(step.done, false);
        if (tour && i === shown && !shownDone && already) acked.add(step.id);
        let done = acked.has(step.id) || (!tour && already);
        const target = done ? null : find(already && step.doneTarget ? step.doneTarget : step.target);
        const isShown = visible(target);
        const away = !done && !isShown && !!call(step.away, false);
        const waits = !done && !!call(step.wait, '');
        if (done || isShown || away) missingSince.delete(i);
        else if (!missingSince.has(i) || (waits && !waiting.has(i))) missingSince.set(i, now);
        if (waits) waiting.add(i); else waiting.delete(i);
        const missingFor = missingSince.has(i) ? now - missingSince.get(i) : 0;
        if (tour && i < last && !done && !isShown && !away && !step.optional && missingFor >= patience(step)) {
          acked.add(step.id);
          done = true;
        }
        return { done, already, shown: isShown && !done, away, missingFor, patience: patience(step), optional: !!step.optional, waiting: waits, target: done ? null : target };
      });
    }

    function tick() {
      if (!running) return;
      const states = look();
      const { index, finished, lost, waiting: waits } = pick(states);
      if (finished) return finish('done');
      const how = index < 0 ? '' : lost ? 'lost' : waits ? 'waiting' : states[index].away ? 'away' : 'here';
      if (index < 0 || (how === 'here' && covered(states[index].target)) || (how !== 'here' && covered(null))) return hide();
      show(index, states[index], how);
    }

    function hide() {
      layer.hidden = true;
      room.remove();
      undescribe();
    }

    function describe(el) {
      if (el === described) return;
      undescribe();
      if (!el) return;
      const ids = (el.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean);
      if (!ids.includes(text.id)) el.setAttribute('aria-describedby', [...ids, text.id].join(' '));
      described = el;
    }
    function undescribe() {
      if (!described) return;
      const ids = (described.getAttribute('aria-describedby') || '').split(/\s+/).filter((id) => id && id !== text.id);
      if (ids.length) described.setAttribute('aria-describedby', ids.join(' '));
      else described.removeAttribute('aria-describedby');
      described = null;
    }

    /** The control the keyboard goes to: the step's own, or the first one inside it. */
    const control = (target) => (target.matches(FOCUSABLE) ? target : target.querySelector(FOCUSABLE)) || target;
    /** The keyboard to the step's control, unless you're typing somewhere: what you type stays where it is. */
    function focusOn(target) {
      const active = doc.activeElement;
      if (active && active !== doc.body && !layer.contains(active) && active.matches(TYPING)) return;
      const into = control(target);
      if (into !== target || target.matches(FOCUSABLE)) into.focus({ preventScroll: true });
    }

    function fill(step, i, how, done) {
      count.textContent = `Step ${step.n || i + 1} of ${options.total || steps.length}${done ? ' · done' : ''}`;
      if (how === 'away') {
        text.textContent = options.awayText || 'Your next step is in another part of this page.';
      } else if (how === 'lost') {
        text.textContent = step.lost || step.say;
      } else if (how === 'waiting') {
        text.textContent = call(step.wait, '') || step.say;
      } else if (done) {
        text.textContent = step.doneSay || step.say;
      } else {
        text.textContent = step.say + (step.press ? ' → ' : '');
        if (step.press) text.append(Object.assign(doc.createElement('b'), { textContent: step.press }));
      }
      const label = how === 'away' ? (step.reveal ? 'Show me' : '')
        : how === 'lost' ? step.button || 'Done'
        : how === 'waiting' ? ''
        : done ? (i === steps.length - 1 ? step.button || 'Done' : 'Next')
        : step.button || '';
      go.textContent = label;
      go.hidden = !label;
    }

    function show(i, state, how) {
      const step = steps[i];
      const docked = how !== 'here';
      const done = tour && state.already;
      // In a <dialog> that's open, the guide goes in it: nothing outside one is drawn over it.
      const host = (!docked && state.target.closest('dialog[open]')) || doc.body;
      if (layer.parentNode !== host) host.append(layer);
      layer.classList.toggle('docked', docked);
      layer.hidden = false;
      if (i !== shown || how !== shownHow || state.target !== shownTarget) {
        if (i !== shown) shownDone = done;
        shown = i;
        shownHow = how;
        shownTarget = docked ? null : state.target;
        fill(step, i, how, shownDone);
      }
      if (docked) {
        undescribe();
        room.style.height = `${bubble.offsetHeight + MARGIN * 2 + PIXEL * 2}px`;
        if (room.parentNode !== doc.body) doc.body.append(room);
      } else {
        room.remove();
        describe(control(state.target));
        // The scroll and the keyboard move to a step only when it's new, or when you asked to be shown it:
        // never just because the guide is back after a dialog or another pane.
        if (settled === ADOPT && !settle) {
          settled = i;
        } else if (i !== settled || settle) {
          settled = i;
          settle = false;
          const box = state.target.getBoundingClientRect();
          if (box.top < 0 || box.top + box.height > win.innerHeight) state.target.scrollIntoView({ block: 'center', behavior: reduced() ? 'auto' : 'smooth' });
          focusOn(state.target);
        }
      }
      position();
    }

    function position() {
      frame = 0;
      if (!running || layer.hidden || shown < 0 || shownHow !== 'here' || !shownTarget || !shownTarget.isConnected) return;
      const step = steps[shown];
      const area = find(step.area);
      const lit = (visible(area) ? area : shownTarget).getBoundingClientRect();
      const view = { width: doc.documentElement.clientWidth || win.innerWidth, height: win.innerHeight };
      // The ring stays inside the window, however wide what's lit.
      const left = Math.max(lit.left - PAD, RING), right = Math.min(lit.left + lit.width + PAD, view.width - RING);
      Object.assign(shade.style, { left: `${left}px`, top: `${lit.top - PAD}px`, width: `${Math.max(0, right - left)}px`, height: `${lit.height + PAD * 2}px` });
      const around = { left: left - RING, top: lit.top - PAD - RING, width: Math.max(0, right - left) + RING * 2, height: lit.height + (PAD + RING) * 2 };
      const size = { width: bubble.offsetWidth + PIXEL * 2, height: bubble.offsetHeight + PIXEL * 2 };
      const at = place(shownTarget.getBoundingClientRect(), view, size, { width: ARROW[0].length * PIXEL, height: ARROW.length * PIXEL }, around);
      arrow.classList.toggle('up', !at.down);
      Object.assign(arrow.style, { left: `${at.arrow.x}px`, top: `${at.arrow.y}px` });
      Object.assign(bubble.style, { left: `${at.bubble.x}px`, top: `${at.bubble.y}px` });
    }
    const follow = () => { if (!frame) frame = win.requestAnimationFrame(position); };

    /** A click on the step's control counts, when that's all it asks. Seen first, before the page acts on it. */
    function clicked(e) {
      if (!running || layer.hidden || shown < 0 || shownHow !== 'here' || !shownTarget) return;
      const step = steps[shown];
      const area = find(step.area);
      if (step.ackOnClick && (shownTarget.contains(e.target) || (area && area.contains(e.target)))) {
        acked.add(step.id);
        win.setTimeout(tick, 60);
      }
    }
    function keyed(e) {
      if (!running || layer.hidden || e.key !== 'Escape' || e.defaultPrevented || covered(shownTarget)) return;
      // Esc in a field empties it, and in a <dialog> closes it: theirs, unless you're in the bubble.
      const inBubble = bubble.contains(e.target);
      if (!inBubble && ((e.target.matches && e.target.matches(TYPING)) || (shownTarget && shownTarget.closest('dialog[open]')))) return;
      e.preventDefault();
      // Put away, not over: it's back the next time the page opens.
      finish('hidden');
    }

    skip.onclick = () => finish('skipped');
    go.onclick = () => {
      if (shown < 0) return;
      const step = steps[shown];
      if (shownHow === 'away') {
        // You asked to be shown it: the scroll and the keyboard go there once it's in sight.
        settle = true;
        call(step.reveal);
      } else {
        acked.add(step.id);
      }
      tick();
    };

    function listen(on) {
      const how = on ? 'addEventListener' : 'removeEventListener';
      win[how]('scroll', follow, { capture: true, passive: true });
      win[how]('resize', follow);
      doc[how]('click', clicked, true);
      doc[how]('keydown', keyed);
    }

    /** Over: the guide goes, and `onFinish` is told why, once. */
    function finish(reason) {
      if (!running) return;
      stop();
      call(() => options.onFinish && options.onFinish(reason));
    }

    function stop() {
      if (!running) return;
      running = false;
      win.clearInterval(timer);
      if (frame) win.cancelAnimationFrame(frame);
      timer = frame = 0;
      listen(false);
      hide();
      layer.remove();
    }

    /**
     * From the beginning, with nothing counted from before. `how.tour`: every step is walked from the
     * first, done or not (Show me around again); `how.focus`: the keyboard and the scroll go to the first
     * step shown, as you asked for it. Otherwise, as on a page that has just opened, they're left where
     * they are until the step changes.
     */
    function start(how = {}) {
      if (running) return;
      acked.clear();
      missingSince.clear();
      waiting.clear();
      tour = !!how.tour;
      shown = -1;
      shownHow = '';
      shownTarget = null;
      settled = how.focus ? -1 : ADOPT;
      settle = false;
      running = true;
      doc.body.append(layer);
      listen(true);
      timer = win.setInterval(tick, 400);
      tick();
    }

    return {
      start, stop,
      /** Looks again now, as after something the page did. */
      check: tick,
      get running() { return running; },
    };
  }

  const api = { create, pick, place, ARROW, COLOURS, GRACE, PIXEL };
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.SmolcorpGuide = api;
})(typeof window !== 'undefined' ? window : globalThis);
