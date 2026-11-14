/**
 * Where the talk bubbles are (1.26, Talk Bubble). The bubble over each head is
 * what a click goes to, once the page gives the room a `talk` (see `on.talk`
 * above): this works out its box, the point its tail aims at, how long a mouse
 * rests on it before the tag shows, which waiting agents' small question
 * bubbles fit and which fall back to the "?", and who is waiting longest.
 *
 * Everything here is a plain function of what it's given, art pixels in and
 * out unless it says CSS pixels, so it runs on its own in node tests, and the
 * hit test, the hover and the bubble panels share one copy. A box is
 * { left, right, top, bottom }, its right and bottom edges just past its last
 * pixel; a bubble, as the room records it, is
 * { id, x, top, state, since }: the person's column and top row, their state,
 * and when they started waiting on you (the office's clock, null if unknown);
 * `quiet` too for an idle head, which draws no bubble and so has no hit box:
 * its desk is still opened by Enter or the hidden desk list.
 */
const TALK_ANCHORS = {
  /** The bubble `drawBubble` draws is 11 wide and 10 tall, its tail's last pixel included; a target is at least this many each way. */
  SMALLEST: 12,
  /** A mouse rests on a bubble this long before its tag shows, in seconds. */
  HOVER_AFTER: 0.25,
  /** A desk's card is asked for at most this often by hovering, in seconds. */
  HOVER_EVERY: 1,
  /** A box kept at least this far from the room's edges. */
  MARGIN: 1,

  /** The bubble over a head whose top row is `top`, in column `x`, as it rests (its shaking, when kept waiting, leaves it where it is). */
  boxOf(x, top) {
    return { left: x - 5, right: x + 6, top: top - 10, bottom: top };
  },

  /** The pixel the bubble's tail ends on: the point a panel grown from it points at. */
  tipOf(x, top) {
    return { x: x - 1, y: top - 1 };
  },

  /** `box` grown to at least `least` pixels each way: sideways about its middle, and only upwards, as its tail points down at the head. */
  padded(box, least = TALK_ANCHORS.SMALLEST) {
    const across = Math.max(0, least - (box.right - box.left)), down = Math.max(0, least - (box.bottom - box.top));
    return { left: box.left - Math.floor(across / 2), right: box.right + Math.ceil(across / 2), top: box.top - down, bottom: box.bottom };
  },

  /** `box` slid, not resized, to lie `margin` inside a room `room.width` × `room.height`; where it's too big, its top left wins. */
  fitInside(box, room, margin = TALK_ANCHORS.MARGIN) {
    const w = box.right - box.left, h = box.bottom - box.top;
    const left = Math.max(margin, Math.min(room.width - margin - w, box.left));
    const top = Math.max(margin, Math.min(room.height - margin - h, box.top));
    return { left, right: left + w, top, bottom: top + h };
  },

  /** What a click on the bubble over a head reaches: its box, padded, and inside the room even where the head is at its edge. */
  hitBox(x, top, room) {
    return TALK_ANCHORS.fitInside(TALK_ANCHORS.padded(TALK_ANCHORS.boxOf(x, top)), room);
  },

  /** Whether two boxes share a pixel. */
  overlap(a, b) {
    return a.left < b.right && b.left < a.right && a.top < b.bottom && b.top < a.bottom;
  },

  /** Notes a bubble the room has just drawn on `list`, once for each person however often the room is drawn in a frame. */
  record(list, bubble) {
    if (!list.some((one) => one.id === bubble.id)) list.push(bubble);
  },

  /**
   * What the page is told of a bubble: its id and state, and its box and tail point in art pixels and in CSS pixels
   * on the page. `view` says where the room is: { scale, ox, oy } its art pixel's size and corner in canvas pixels,
   * `k` the canvas pixels to a CSS pixel, and { left, top } the canvas's corner on the page.
   */
  anchorOf(bubble, room, view) {
    const box = TALK_ANCHORS.hitBox(bubble.x, bubble.top, room), tip = TALK_ANCHORS.tipOf(bubble.x, bubble.top);
    const css = (x, y) => ({ x: view.left + (view.ox + x * view.scale) / view.k, y: view.top + (view.oy + y * view.scale) / view.k });
    const from = css(box.left, box.top), to = css(box.right, box.bottom);
    return {
      id: bubble.id, state: bubble.state, since: bubble.since,
      art: { box, tip },
      css: { box: { left: from.x, right: to.x, top: from.y, bottom: to.y }, tip: css(tip.x, tip.y) },
    };
  },

  /** A mouse over nothing yet. */
  hover() {
    return { id: null, since: 0, shown: null };
  },

  /**
   * The mouse is over bubble `id` (null: over none) at `now`, in seconds; `state` is what this returned before.
   * Says the next state, who the tag is to show for now (`show`), whether a tag shown before goes away (`hide`), and
   * how many seconds to wait before looking again (`wait`, null for no need). Moving to another bubble starts the wait again.
   */
  hoverStep(state, id, now) {
    const hide = state.shown !== null && state.shown !== id;
    if (id === null) return { state: TALK_ANCHORS.hover(), show: null, hide, wait: null };
    const since = state.id === id ? state.since : now;
    const due = since + TALK_ANCHORS.HOVER_AFTER;
    if (now < due) return { state: { id, since, shown: null }, show: null, hide, wait: due - now };
    return { state: { id, since, shown: id }, show: state.shown === id ? null : id, hide, wait: null };
  },

  /** Whether a desk's card may be asked for again, `last` being when it last was (undefined if never), in seconds. */
  mayAsk(last, now) {
    return last === undefined || now - last >= TALK_ANCHORS.HOVER_EVERY;
  },

  /** The waiting bubbles, longest waiting first, as the room's agitation grows; those whose start isn't known last, by id. */
  needsYou(bubbles) {
    const since = (b) => (typeof b.since === 'number' ? b.since : Infinity);
    return bubbles.filter((b) => b.state === 'asking')
      .sort((a, b) => (since(a) === since(b) ? String(a.id).localeCompare(String(b.id)) : since(a) - since(b)));
  },

  /**
   * Where each waiting desk's small question bubble goes, `size` { w, h } art pixels, its tail on the head and the
   * whole kept inside `room`, longest waiting first. One that would cover a bubble placed before it falls back to
   * the "?" bubble: `small` is { id, box, tip } for those that fit, `marks` the ids of those that fall back, and
   * `order` all of them in the order the Needs you key walks.
   */
  layoutSmall(bubbles, size, room) {
    const order = TALK_ANCHORS.needsYou(bubbles), small = [], marks = [];
    for (const b of order) {
      const tip = TALK_ANCHORS.tipOf(b.x, b.top), left = tip.x - Math.floor(size.w / 2);
      const box = TALK_ANCHORS.fitInside({ left, right: left + size.w, top: tip.y - size.h, bottom: tip.y }, room);
      if (small.some((placed) => TALK_ANCHORS.overlap(placed.box, box))) marks.push(b.id);
      else small.push({ id: b.id, box, tip });
    }
    return { small, marks, order: order.map((b) => b.id) };
  },
};
