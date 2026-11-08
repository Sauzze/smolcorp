/*
 * The first-run guide: a pixel arrow on the one control to use next, and a
 * short bubble saying what to do, step by step. Each step moves on by what you
 * do (the folder saved, the checks green, the hire sent), never by a Next
 * button alone. The rest of the page is dimmed, but nothing is covered: every
 * control still works, the step's own most of all, and the bubble sits beside
 * or below what's lit, off what the page asks to keep in sight, else right
 * against its arrow. While the step's control is off the window's edge, it's a
 * small "↓ Continue below" chip that brings it into sight. While the step is in
 * another part of the page, it's a small pill in a corner, which on a phone
 * shrinks to a chip once you scroll. Skip ends it; Esc only puts it away until
 * the page opens again.
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
  const ARROW_SIZE = { width: ARROW[0].length * PIXEL, height: ARROW.length * PIXEL };
  /** How long a step's control may be missing before the guide looks past it, in ms. */
  const GRACE = 2500;
  /** Between the arrow and what it points at, and from the window's edges. */
  const GAP = 4;
  const MARGIN = 12;
  /** The light round what's to be used, beyond its box. */
  const PAD = 6;
  /** The ring round what's lit: two pixels of the art, kept inside the window. */
  const RING = PIXEL * 2;
  /** The pill's widest, in CSS pixels, its ring in: it fits the foot of a side column (setup's is 220). */
  const PILL = 196;
  /** The chip shown while the step's control is off the window's edge, its ring in, until it's measured. */
  const CHIP = { width: 150, height: 32 };
  /** A window this wide or less is a phone's (setup's own breakpoint): the bubble stays against its arrow there. */
  const PHONE = 760;
  /** A phone, or a finger: where the pill shrinks to a chip once you scroll, as it's in the way of what you read. */
  const NARROW = `(max-width: ${PHONE}px), (pointer: coarse)`;
  /** How far the page scrolls before the pill shrinks, in CSS pixels. */
  const SHRINK = 48;

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

  /** How far below what's lit the bubble may move to keep something in sight, before it goes elsewhere. */
  const SLIDE = 120;

  const overlaps = (a, b) => a.left < b.left + b.width && a.left + a.width > b.left && a.top < b.top + b.height && a.top + a.height > b.top;
  const overlap = (a, b) => Math.max(0, Math.min(a.left + a.width, b.left + b.width) - Math.max(a.left, b.left))
    * Math.max(0, Math.min(a.top + a.height, b.top + b.height) - Math.max(a.top, b.top));

  /**
   * Where the arrow and the bubble go, for a control whose box in the window is `box` ({ top, left, width,
   * height }), lit with the box `around` (the control's, if not given), in a window `view` ({ width,
   * height }), with a bubble and an arrow of the sizes given, the bubble kept off the boxes in `keep` (the
   * page's title, the items the step talks about) and off `how.text` (the lines of text on screen) near
   * what's lit: wherever a bubble beside or below it could reach. The arrow's tip is on the control. The
   * bubble goes beside what's lit, right then left, else below it, along it or moved down past what's kept
   * if need be; never on what's lit or kept, and inside the window. A control off the window's edge (its
   * middle out of the window, or cut by its edge with no room beside or below) gets no bubble and no arrow,
   * but a chip (`how.chip`, its size) at that edge, off what's kept: "chip", pointing "up" or "down"; it's
   * never parked away from it, nor the arrow on whatever row is at the edge. Failing those, on a phone
   * (`view` no wider than PHONE), the bubble goes right against its arrow, under it or over
   * it ("attached"), on nothing lit or kept, over as little of the text near it as can be. Else, or with
   * `how.aside` (its control was just pressed: out of the way of what that brings), it goes in the corner
   * that covers least (what's lit, the arrow, what's kept, the text near what's lit, then any other
   * text); so it does when even its arrow has no room to be next to. In a corner the arrow
   * stays on the control, on a side where it covers nothing kept. `side` says where the bubble went
   * ("right", "left", "below", "attached", "edge", "chip"); `point`, the way the arrow points ("up",
   * "down", "left", "right").
   */
  function place(box, view, bubble, arrow, around = box, keep = [], how = {}) {
    const aw = arrow.width, ah = arrow.height, bw = bubble.width, bh = bubble.height;
    const cx = box.left + box.width / 2, cy = box.top + box.height / 2;
    const right = around.left + around.width, bottom = around.top + around.height;
    const rect = (x, y, width, height) => ({ left: x, top: y, width, height });
    // The text near what's lit is kept as the page's keep is: a status line, an error, the line under a
    // button. The rest is only covered as little as can be, in a corner.
    const reach = rect(around.left - bw - aw - GAP, around.top - bh - GAP, around.width + (bw + aw + GAP) * 2, around.height + bh * 2 + ah + SLIDE + GAP * 2);
    const near = [], far = [];
    for (const line of how.text || []) (overlaps(line, reach) ? near : far).push(line);
    const asked = keep;
    keep = [...asked, ...near];
    const inside = (r) => r.left >= MARGIN && r.top >= MARGIN && r.left + r.width <= view.width - MARGIN && r.top + r.height <= view.height - MARGIN;
    const clearOf = (r, boxes) => inside(r) && boxes.every((k) => !overlaps(r, k));
    const free = (x, y) => clearOf(rect(x, y, bw, bh), [around, ...keep]);
    // The arrow along the control, centred on it, inside the window; under or over it, moved along it, if
    // need be, to cover nothing kept.
    const ax = clamp(cx - aw / 2, MARGIN, view.width - MARGIN - aw);
    const ay = clamp(cy - ah / 2, MARGIN, view.height - MARGIN - ah);
    const along = (y) => {
      const low = Math.max(MARGIN, Math.min(box.left, ax)), high = Math.min(view.width - MARGIN - aw, Math.max(box.left + box.width - aw, ax));
      const xs = [ax, ...keep.flatMap((k) => [k.left + k.width + GAP, k.left - GAP - aw])].map((x) => clamp(x, low, high));
      return { x: xs.find((x) => clearOf(rect(x, y, aw, ah), keep)) ?? ax, y };
    };
    const result = (side, point, arrowAt, x, y) => ({
      side, point, down: point === 'down',
      arrow: { x: Math.round(arrowAt.x), y: Math.round(arrowAt.y) },
      bubble: { x: Math.round(x), y: Math.round(y) },
    });

    // A box at the top or the foot of the window moved off it, down or up, past what the page keeps there
    // (a bar across the top, the logo and Search), inside the window: where it stops on the way.
    const stops = (x, y, w, h, down) => {
      const at = [];
      for (let moved = true, n = 0; moved && n <= asked.length; n++) {
        moved = false;
        for (const k of asked) {
          if (!overlaps(rect(x, y, w, h), k)) continue;
          y = clamp(down ? k.top + k.height + GAP : k.top - GAP - h, MARGIN, view.height - MARGIN - h);
          at.push(y);
          moved = true;
        }
      }
      return at;
    };
    const past = (x, y, w, h, down) => stops(x, y, w, h, down).pop() ?? y;
    // Off the window's edge: a chip at that edge, toward it, off what the page keeps.
    const chip = (up) => {
      const { width: cw, height: ch } = how.chip || CHIP;
      const x = clamp(cx - cw / 2, MARGIN, view.width - MARGIN - cw);
      const y = past(x, up ? MARGIN : view.height - MARGIN - ch, cw, ch, up);
      return result('chip', up ? 'up' : 'down', { x, y }, x, y);
    };
    // Its middle out of the window: there's nothing of it to point at but an edge.
    if (cy < 0 || cy > view.height) return chip(cy < 0);

    // Beside what's lit: the arrow just off it, level with the control, and the bubble level with the
    // arrow, centred on it, or moved up or down to keep clear of what's kept.
    for (const side of how.aside ? [] : ['right', 'left']) {
      const arrowAt = { x: side === 'right' ? right + GAP : around.left - GAP - aw, y: ay };
      if (!clearOf(rect(arrowAt.x, arrowAt.y, aw, ah), keep)) continue;
      const x = side === 'right' ? arrowAt.x + aw : arrowAt.x - bw;
      const low = Math.min(ay + ah - bh, ay), high = Math.max(ay + ah - bh, ay);
      const ys = [cy - bh / 2, ...keep.flatMap((k) => [k.top + k.height + GAP, k.top - GAP - bh]), low, high].map((y) => clamp(y, low, high));
      const y = ys.find((at) => free(x, at));
      if (y !== undefined) return result(side, side === 'right' ? 'left' : 'right', arrowAt, x, y);
    }

    // Below what's lit: the arrow's tip on the control's foot, pointing up, the bubble under what's lit,
    // under the arrow, else along what's lit (as near the arrow as it can), else moved down past what's
    // kept if that's all it takes.
    const under = along(box.top + box.height + GAP);
    let x = under.x + aw / 2 - bw / 2;
    if (bw <= around.width) x = clamp(x, around.left, right - bw);
    x = clamp(x, Math.max(MARGIN, under.x + aw - bw), Math.min(view.width - MARGIN - bw, under.x));
    // Along what's lit, when it's wider than the bubble; under the arrow when it isn't.
    const [lowX, highX] = bw <= around.width
      ? [Math.max(MARGIN, around.left), Math.min(view.width - MARGIN - bw, right - bw)]
      : [Math.max(MARGIN, under.x + aw - bw), Math.min(view.width - MARGIN - bw, under.x)];
    const xs = [x, ...keep.flatMap((k) => [k.left + k.width + GAP, k.left - GAP - bw])]
      .map((at) => clamp(at, lowX, highX)).sort((a, b) => Math.abs(a - x) - Math.abs(b - x));
    if (!how.aside && clearOf(rect(under.x, under.y, aw, ah), keep)) {
      const from = Math.max(under.y + ah, bottom + GAP);
      const ys = [from, ...keep.map((k) => k.top + k.height + GAP).filter((y) => y > from && y <= from + SLIDE).sort((a, b) => a - b)];
      for (const y of ys) {
        const at = xs.find((bx) => free(bx, y));
        if (at !== undefined) return result('below', 'up', under, at, y);
      }
    }

    // Cut by the window's edge, with no room beside or below: a chip toward the rest of it, as long as it
    // fits in the window once brought into sight (else it's as good as it gets).
    if ((box.top < 0 || box.top + box.height > view.height) && box.height <= view.height - MARGIN * 2) return chip(box.top < 0);

    // The window's edge: the arrow on the control where it covers nothing kept, the bubble in the corner
    // that covers least: never what's lit if it can help it, then the arrow, what the page keeps, the text
    // near what's lit, any other text.
    const sides = [
      ['up', under],
      ['left', { x: right + GAP, y: ay }],
      ['down', along(box.top - GAP - ah)],
      ['right', { x: around.left - GAP - aw, y: ay }],
    ];
    const [point, arrowAt] = sides.find(([, at]) => clearOf(rect(at.x, at.y, aw, ah), keep))
      || (box.top - GAP - ah >= MARGIN ? sides[2] : sides[0]);
    arrowAt.x = clamp(arrowAt.x, MARGIN, view.width - MARGIN - aw);
    arrowAt.y = clamp(arrowAt.y, MARGIN, view.height - MARGIN - ah);
    const pointer = rect(arrowAt.x, arrowAt.y, aw, ah);
    // Each corner, then each moved a little off its edge (SLIDE at most), past what's kept there: never on
    // the logo or Search for want of a corner clear of them.
    const corners = [
      [view.width - MARGIN - bw, view.height - MARGIN - bh], [MARGIN, view.height - MARGIN - bh],
      [view.width - MARGIN - bw, MARGIN], [MARGIN, MARGIN],
    ].map(([cornerX, cornerY]) => [Math.max(MARGIN, cornerX), Math.max(MARGIN, cornerY)]);
    corners.push(...corners.flatMap(([cornerX, cornerY]) => stops(cornerX, cornerY, bw, bh, cornerY === MARGIN)
      .filter((y) => Math.abs(y - cornerY) <= SLIDE).map((y) => [cornerX, y])));
    const covers = ([cornerX, cornerY]) => {
      const r = rect(cornerX, cornerY, bw, bh);
      const sum = (boxes) => boxes.reduce((total, k) => total + overlap(r, k), 0);
      return [overlap(r, around), overlap(r, pointer), sum(asked), sum(near), sum(far)];
    };
    // The first corner of those that cover least, in that order of what matters.
    const less = (a, b) => { const i = a.findIndex((n, at) => n !== b[at]); return i >= 0 && a[i] < b[i]; };
    let corner = corners[0], least = covers(corner);
    for (const at of corners.slice(1)) {
      const these = covers(at);
      if (less(these, least)) [corner, least] = [at, these];
    }
    // On a phone: right against its arrow, rather than in a corner far from it. Under the arrow, moved down
    // past what's kept if that's all it takes; else over it. On nothing lit or kept, over as little of the
    // text near it as can be, then as near the middle. Its control just pressed, or a wider window (where
    // the corner is clear of the rows near what's lit): the corner, as it is.
    if (how.aside || view.width > PHONE) return result('edge', point, arrowAt, corner[0], corner[1]);
    const nearOn = (r) => near.reduce((total, k) => total + overlap(r, k), 0);
    const next = (at, y) => {
      const mid = at.x + aw / 2 - bw / 2;
      const lowAt = Math.max(MARGIN, at.x + aw - bw), highAt = Math.min(view.width - MARGIN - bw, at.x);
      const options = [mid, lowAt, highAt, ...[...asked, ...near].flatMap((k) => [k.left + k.width + GAP, k.left - GAP - bw])]
        .map((bx) => clamp(bx, lowAt, highAt))
        .filter((bx) => clearOf(rect(bx, y, bw, bh), [around, rect(at.x, at.y, aw, ah), ...asked]));
      const score = (bx) => [nearOn(rect(bx, y, bw, bh)), Math.abs(bx - mid)];
      return options.sort((a, b) => (less(score(a), score(b)) ? -1 : less(score(b), score(a)) ? 1 : 0))[0];
    };
    if (clearOf(rect(under.x, under.y, aw, ah), asked)) {
      const from = Math.max(under.y + ah, bottom + GAP);
      for (const y of [from, ...asked.map((k) => k.top + k.height + GAP).filter((y) => y > from && y <= from + SLIDE).sort((a, b) => a - b)]) {
        const at = next(under, y);
        if (at !== undefined) return result('attached', 'up', under, at, y);
      }
    }
    const over = along(box.top - GAP - ah);
    if (over.y >= MARGIN && clearOf(rect(over.x, over.y, aw, ah), asked)) {
      const y = Math.min(over.y, around.top - GAP) - bh;
      const at = next(over, y);
      if (at !== undefined) return result('attached', 'down', over, at, y);
    }
    return result('edge', point, arrowAt, corner[0], corner[1]);
  }

  const STYLE = `
.smol-guide { position: fixed; inset: 0; z-index: 2147483000; pointer-events: none; }
.smol-guide[hidden] { display: none !important; }
.smol-guide-shade { position: fixed; border-radius: 6px; pointer-events: none;
  box-shadow: 0 0 0 ${PIXEL}px ${COLOURS.y}, 0 0 0 ${PIXEL * 2}px ${COLOURS.k}, 0 0 0 200vmax rgb(0 0 0 / .38); }
.smol-guide-arrow { position: fixed; left: 0; top: 0; pointer-events: none; }
.smol-guide-arrow[data-point="up"] { transform: rotate(180deg); }
.smol-guide-arrow[data-point="left"] { transform: rotate(90deg); }
.smol-guide-arrow[data-point="right"] { transform: rotate(-90deg); }
.smol-guide-arrow svg { display: block; shape-rendering: crispEdges; animation: smol-guide-bob 1.1s steps(1, end) infinite; }
@keyframes smol-guide-bob { 0% { transform: translateY(0); } 50% { transform: translateY(${PIXEL}px); } }
.smol-guide-bubble { position: fixed; left: 0; top: 0; margin: ${PIXEL}px; pointer-events: auto; box-sizing: border-box;
  width: max-content; max-width: min(320px, calc(100vw - ${MARGIN * 2 + PIXEL * 2}px)); padding: 10px 12px;
  background: #f4f1ea; color: ${COLOURS.k}; border: 0; border-radius: 0;
  box-shadow: 0 -${PIXEL}px 0 0 ${COLOURS.k}, 0 ${PIXEL}px 0 0 ${COLOURS.k}, -${PIXEL}px 0 0 0 ${COLOURS.k}, ${PIXEL}px 0 0 0 ${COLOURS.k},
    0 ${PIXEL * 2}px 0 0 rgb(0 0 0 / .25);
  font: 500 13px/1.4 -apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", system-ui, sans-serif; text-align: left; }
.smol-guide-count[hidden] { display: none; }
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
.smol-guide.pill .smol-guide-bubble { left: ${MARGIN}px !important; transform: none; max-width: min(${PILL}px, calc(100vw - ${MARGIN * 2 + PIXEL * 2}px));
  display: flex; flex-wrap: wrap; align-items: center; column-gap: 6px; padding: 5px 4px 5px 9px; font-size: 12px; }
.smol-guide.pill .smol-guide-count { display: none; }
.smol-guide.pill .smol-guide-text { flex: 1 1 0; min-width: 0; }
.smol-guide.pill .smol-guide-tools { display: contents; }
.smol-guide.pill .smol-guide-skip { order: 1; align-self: flex-start; padding: 0 5px; font-size: 15px; line-height: 1.1; text-decoration: none; }
.smol-guide.pill .smol-guide-go { order: 2; padding: 2px 8px; margin-top: 3px; }
/* On a phone, and under a finger: the pill on one line, Show me then ×, apart, each at least 44 px. */
@media (max-width: 760px), (pointer: coarse) {
  .smol-guide.pill .smol-guide-bubble { max-width: calc(100vw - ${MARGIN * 2 + PIXEL * 2}px); flex-wrap: nowrap; column-gap: 12px; padding: 4px 4px 4px 12px; font-size: 13px; }
  .smol-guide.pill .smol-guide-go { order: 1; min-height: 44px; padding: 0 16px; margin-top: 0; }
  .smol-guide.pill .smol-guide-skip { order: 2; align-self: center; min-width: 44px; min-height: 44px; padding: 0; font-size: 20px; }
}
/* Shrunk, on a phone once you scroll: only the button, at the right, so it covers next to nothing. */
.smol-guide.pill.small .smol-guide-bubble { left: auto !important; right: ${MARGIN}px; padding: 3px; }
.smol-guide.pill.small .smol-guide-text, .smol-guide.pill.small .smol-guide-skip { display: none; }
/* The step's control off the window's edge: a chip toward it, in place of the bubble and the arrow. Hidden
   rather than gone, either way, so each can be measured. */
.smol-guide-more { position: fixed; left: 0; top: 0; margin: ${PIXEL}px; visibility: hidden; pointer-events: auto; box-sizing: border-box;
  min-height: 26px; padding: 4px 12px; border: 0; border-radius: 0; background: ${COLOURS.k}; color: #f4f1ea; cursor: pointer;
  box-shadow: 0 -${PIXEL}px 0 0 ${COLOURS.y}, 0 ${PIXEL}px 0 0 ${COLOURS.y}, -${PIXEL}px 0 0 0 ${COLOURS.y}, ${PIXEL}px 0 0 0 ${COLOURS.y};
  font: 700 12px/1.4 -apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", system-ui, sans-serif; white-space: nowrap; }
.smol-guide-more:focus-visible { outline: ${PIXEL}px solid ${COLOURS.y}; outline-offset: 2px; }
.smol-guide.more .smol-guide-more { visibility: visible; }
.smol-guide.more .smol-guide-bubble, .smol-guide.more .smol-guide-arrow { visibility: hidden; }
@media (max-width: ${PHONE}px), (pointer: coarse) { .smol-guide-more { min-height: 44px; padding: 0 16px; font-size: 13px; } }
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
   * done, in a tour), keep (what the bubble stays off on this step: a selector,
   * an element, a function or a list of them) }.
   * `options.keep`, what the bubble stays off on every step (the page's title),
   * besides the text near what's lit, which it always stays off;
   * `options.modals`, a selector for what covers the page besides a <dialog>,
   * under which the guide waits; `options.awayText`, what the pill says while
   * the step is elsewhere; `options.skipSays`, what Skip says it does, on hover
   * and to a screen reader; `options.where` (a function: which part of the page
   * is shown) and `options.hideSays`: the pill's × only hides it there, and says
   * so; it's back in the next part you open, as the guide is only over when it's
   * done or skipped where its step is. `options.onFinish(reason)`, called once
   * it's over: "done", "skipped" (Skip), or "hidden" (Esc: put away until the
   * page opens again).
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
    const skipSays = options.skipSays || 'Skip the guide', hideSays = options.hideSays || 'Hide for now';
    skip.setAttribute('aria-label', skipSays);
    skip.title = skipSays;
    const go = Object.assign(make('button', 'smol-guide-go'), { type: 'button' });
    tools.append(skip, go);
    bubble.append(count, text, tools);
    /** In place of the bubble while the step's control is off the window's edge: brings it into sight. */
    const more = Object.assign(make('button', 'smol-guide-more'), { type: 'button', textContent: '↓ Continue below' });
    layer.append(shade, arrow, bubble, more);
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
    /** The step whose control was just pressed: its bubble stays out of the way of what that brings, until the step changes. */
    let pressed = -1;
    /** Where the pill was hidden (its ×): not shown there again, but in the next part of the page you open. */
    let pillHiddenIn = null;
    const where = () => String(call(options.where, '') ?? '');
    /** The pill shrunk to a chip (a phone, once you scroll), where it was shown and from which scroll. */
    let pillSmall = false, pillIn = null, pillFrom = 0;
    const scrollTop = () => win.scrollY || win.pageYOffset || (doc.documentElement && doc.documentElement.scrollTop) || 0;
    const narrow = () => !!(win.matchMedia && win.matchMedia(NARROW).matches);

    const call = (fn, fallback) => { try { return fn ? fn() : fallback; } catch { return fallback; } };
    const find = (target) => {
      const found = typeof target === 'function' ? call(target, null) : target;
      return typeof found === 'string' ? doc.querySelector(found) : found || null;
    };
    /** Every element meant: a selector (all it matches), an element, a function giving any of those, or a list. */
    const all = (what) => {
      const found = typeof what === 'function' ? call(what, null) : what;
      if (!found) return [];
      if (typeof found === 'string') return [...doc.querySelectorAll(found)];
      return Array.isArray(found) ? found.flatMap(all) : [found];
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
      // The pill hidden by its ×: back at the step, or in the next part of the page you open.
      if (pillHiddenIn !== null && ((index >= 0 && how !== 'away') || where() !== pillHiddenIn)) pillHiddenIn = null;
      if (index < 0 || pillHiddenIn !== null || (how === 'here' && covered(states[index].target)) || (how !== 'here' && covered(null))) return hide();
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
      // No count: Start here says its own (N steps left), and two counts that disagree is one too many.
      count.textContent = done ? 'Done' : '';
      count.hidden = !done;
      skip.textContent = how === 'away' ? '×' : 'Skip';
      // The pill's × hides it here, and says so; Skip ends the guide, and says how to have it back.
      skip.title = how === 'away' ? hideSays : skipSays;
      skip.setAttribute('aria-label', skip.title);
      if (how === 'away') {
        text.textContent = (options.awayText || 'Next step in another part of this page') + (step.reveal ? ' →' : '');
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
      // The pill at its full size wherever it's first shown, in each part of the page; shrunk once you scroll there.
      if (how !== 'away' || shownHow !== 'away' || i !== shown || where() !== pillIn) {
        pillSmall = false;
        pillIn = how === 'away' ? where() : null;
        pillFrom = scrollTop();
      }
      // A step's control pressed counts until the step changes, or you're shown it again.
      if (i !== shown || how !== 'here') pressed = -1;
      // In a <dialog> that's open, the guide goes in it: nothing outside one is drawn over it.
      const host = (!docked && state.target.closest('dialog[open]')) || doc.body;
      if (layer.parentNode !== host) host.append(layer);
      layer.classList.toggle('docked', docked);
      layer.classList.toggle('pill', how === 'away');
      layer.hidden = false;
      if (i !== shown || how !== shownHow || state.target !== shownTarget) {
        if (i !== shown) shownDone = done;
        shown = i;
        shownHow = how;
        shownTarget = docked ? null : state.target;
        fill(step, i, how, shownDone);
      }
      layer.classList.toggle('small', how === 'away' && pillSmall);
      if (how === 'away' && step.reveal) go.textContent = pillSmall ? 'Next step →' : 'Show me';
      // Shrunk, its words are gone: the button says them on hover and to a screen reader.
      go.title = how === 'away' && pillSmall ? text.textContent.replace(/ →$/, '') : '';
      if (go.title) go.setAttribute('aria-label', `${go.title}: show me`);
      else go.removeAttribute('aria-label');
      if (docked) {
        layer.classList.toggle('more', false);
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
          // Brought into sight; or to the middle, when the bubble would go to the window's edge here and
          // beside or below what's lit there.
          const box = state.target.getBoundingClientRect();
          const laid = position();
          const beside = (at) => ['right', 'left', 'below'].includes(at.side);
          if (box.top < 0 || box.top + box.height > win.innerHeight || (laid && !beside(laid.at) && beside(centred(laid)))) {
            state.target.scrollIntoView({ block: 'center', behavior: reduced() ? 'auto' : 'smooth' });
          }
          focusOn(state.target);
        }
      }
      position();
    }

    /**
     * What the bubble stays off, as it is now: the page's and the step's, those shown, and not what's lit,
     * what's in it or holds it. While the step is in a <dialog>, only what's in the dialog.
     */
    const FIELDS = 'input:not([type="hidden"]), select, textarea';
    /**
     * The text on screen, line by line, and the fields: near what's lit the bubble stays off them (see
     * `place`), elsewhere it covers as little of them as it can. Not the guide's own, nor what's lit. While
     * the step is in a <dialog>, only what's in it. Pages without the DOM's walker and ranges have none.
     */
    function lines(litEl, view) {
      if (!doc.createTreeWalker || !doc.createRange) return [];
      const scope = shownTarget.closest('dialog[open]') || doc.body;
      const range = doc.createRange();
      const onScreen = (r) => r.width > 0 && r.height > 0 && r.top < view.height && r.top + r.height > 0 && r.left < view.width && r.left + r.width > 0;
      // Accept 1, reject (with all inside) 2, skip (but look inside) 3.
      const walker = doc.createTreeWalker(scope, 1 | 4, { acceptNode: (node) => {
        if (node.nodeType === 3) return /\S/.test(node.data) ? 1 : 3;
        if (node === layer || node === litEl || /^(SCRIPT|STYLE|TEMPLATE|NOSCRIPT)$/.test(node.tagName)) return 2;
        if (!node.getClientRects().length && win.getComputedStyle(node).display !== 'contents') return 2;
        return node.matches(FIELDS) ? 1 : 3;
      } });
      const boxes = [];
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        let rects = node.getClientRects ? node.getClientRects() : null;
        if (!rects) {
          range.selectNodeContents(node);
          rects = range.getClientRects();
        }
        const shown = [...rects].filter(onScreen);
        if (shown.length && (!node.parentElement || win.getComputedStyle(node.parentElement).visibility !== 'hidden')) {
          for (const r of shown) boxes.push({ left: r.left, top: r.top, width: r.width, height: r.height });
        }
      }
      return boxes;
    }

    function kept(step, litEl) {
      const dialog = shownTarget.closest('dialog[open]');
      return [...all(options.keep), ...all(step.keep)]
        .filter((el) => visible(el) && !el.contains(litEl) && !litEl.contains(el) && (!dialog || dialog.contains(el)))
        .map((el) => el.getBoundingClientRect())
        .filter((box) => box.width > 0 && box.height > 0);
    }

    function position() {
      frame = 0;
      if (!running || layer.hidden || shown < 0 || shownHow !== 'here' || !shownTarget || !shownTarget.isConnected) return null;
      const step = steps[shown];
      const area = find(step.area);
      const litEl = visible(area) ? area : shownTarget;
      const lit = litEl.getBoundingClientRect();
      const view = { width: doc.documentElement.clientWidth || win.innerWidth, height: win.innerHeight };
      // The ring stays inside the window, however wide what's lit.
      const left = Math.max(lit.left - PAD, RING), right = Math.min(lit.left + lit.width + PAD, view.width - RING);
      Object.assign(shade.style, { left: `${left}px`, top: `${lit.top - PAD}px`, width: `${Math.max(0, right - left)}px`, height: `${lit.height + PAD * 2}px` });
      const around = { left: left - RING, top: lit.top - PAD - RING, width: Math.max(0, right - left) + RING * 2, height: lit.height + (PAD + RING) * 2 };
      const size = { width: bubble.offsetWidth + PIXEL * 2, height: bubble.offsetHeight + PIXEL * 2 };
      const chip = { width: more.offsetWidth + PIXEL * 2, height: more.offsetHeight + PIXEL * 2 };
      const box = shownTarget.getBoundingClientRect(), keep = kept(step, litEl), text = lines(litEl, view);
      // Its control just pressed: out of the way at once, of what the press brings.
      const at = place(box, view, size, ARROW_SIZE, around, keep, { text, aside: pressed === shown, chip });
      // Off the window's edge: the chip toward it, in place of the bubble and the arrow.
      const off = at.side === 'chip';
      layer.classList.toggle('more', off);
      if (off) {
        more.textContent = at.point === 'up' ? '↑ Continue above' : '↓ Continue below';
        Object.assign(more.style, { left: `${at.bubble.x}px`, top: `${at.bubble.y}px` });
        return { at, box, around, keep, text, view, size };
      }
      arrow.dataset.point = at.point;
      Object.assign(arrow.style, { left: `${at.arrow.x}px`, top: `${at.arrow.y}px` });
      Object.assign(bubble.style, { left: `${at.bubble.x}px`, top: `${at.bubble.y}px` });
      return { at, box, around, keep, text, view, size };
    }
    /** Where it would all go with what's lit scrolled to the middle of the window. */
    function centred({ box, around, keep, text, view, size }) {
      const by = view.height / 2 - (around.top + around.height / 2);
      const moved = (b) => ({ left: b.left, top: b.top + by, width: b.width, height: b.height });
      return place(moved(box), view, size, ARROW_SIZE, moved(around), keep.map(moved), { text: text.map(moved) });
    }
    const follow = () => { if (!frame) frame = win.requestAnimationFrame(position); };
    /** Scrolled: the bubble follows; and on a phone the pill, in the way of what you read, shrinks to a chip. */
    function scrolled() {
      if (running && !layer.hidden && shownHow === 'away' && !pillSmall && narrow() && Math.abs(scrollTop() - pillFrom) > SHRINK) {
        pillSmall = true;
        tick();
      }
      follow();
    }

    /**
     * A click on the step's control counts, when that's all it asks. Seen first, before the page acts on it.
     * A control of the step pressed: the bubble goes out of the way at once, before what the press brings
     * (a status line, an error) is there to be covered, and stays there until the step changes.
     */
    function clicked(e) {
      if (!running || layer.hidden || shown < 0 || shownHow !== 'here' || !shownTarget) return;
      const step = steps[shown];
      const area = find(step.area);
      const inStep = (el) => !!el && (shownTarget.contains(el) || (!!area && area.contains(el)));
      if (!inStep(e.target)) return;
      if (inStep(e.target.closest && e.target.closest(FOCUSABLE))) {
        pressed = shown;
        position();
      }
      if (step.ackOnClick) {
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

    skip.onclick = () => {
      // The pill's × hides it in this part of the page only: the guide is over only where its step is.
      if (shownHow === 'away') {
        pillHiddenIn = where();
        return hide();
      }
      finish('skipped');
    };
    // The step's control off the window's edge: brought into sight, and the keyboard with it.
    more.onclick = () => {
      if (!running || shown < 0 || shownHow !== 'here' || !shownTarget) return;
      shownTarget.scrollIntoView({ block: 'center', behavior: reduced() ? 'auto' : 'smooth' });
      focusOn(shownTarget);
    };
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
      win[how]('scroll', scrolled, { capture: true, passive: true });
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
      pressed = -1;
      pillHiddenIn = null;
      pillSmall = false;
      pillIn = null;
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

  const api = { create, pick, place, ARROW, COLOURS, GRACE, PIXEL, NARROW };
  if (typeof module === 'object' && module.exports) module.exports = api;
  else root.SmolcorpGuide = api;
})(typeof window !== 'undefined' ? window : globalThis);
