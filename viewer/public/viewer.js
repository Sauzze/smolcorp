/*
 * The portable office viewer: spec v1 §6.
 *
 * /v/<id>#k=<key>: polls GET /api/s/<id> every 5 s while the page is seen,
 * opens the body in the browser (viewer-crypto.js), checks its version and
 * time, and draws the office read-only. Anything wrong, at any point, is the
 * same "Not active", with the last frame wiped.
 *
 * View only: the office gets no handlers, the canvas takes no pointer
 * events, and every tap, key and gesture is stopped before office.js could
 * hear it (it would otherwise start its sound on a tap). The one tap let
 * through is on the elevator, and only as a ride to the other floor, asked
 * of the office here: nothing reaches it as an event. The only request
 * is the GET. The key is never stored, logged or sent: it leaves the
 * fragment once, into a non-extractable CryptoKey. What is kept, in this
 * browser's storage, is the time of the newest frame shown for each id, so
 * a reload can't be shown an older one again.
 */
(() => {
  'use strict';

  const POLL = 5000;          // ms between polls while live, or nearly
  const SLOW = 30000;         // ms between polls after MISSES misses in a row
  const MISSES = 3;
  const MAX_AGE = 10 * 60000; // the newest frame may be this old…
  const SKEW = 2 * 60000;     // …give or take the two clocks' difference

  const C = window.ViewerCrypto;
  const root = document.documentElement;
  const canvas = document.getElementById('room');
  const label = document.getElementById('live');

  /* ---------------------------------------------------- nothing gets in */

  // A press held this long on a desk upstairs opens its card, as the office's own hold does.
  const HOLD_MS = 450;
  let holding = null;
  const stop = (e) => {
    e.stopPropagation();
    e.stopImmediatePropagation();
    if (e.cancelable && !e.type.startsWith('key')) e.preventDefault();
    // The elevator, and nothing else: a press on it rides to the other floor, and a mouse over it shows it can be clicked.
    const lift = office && lastAt && e.isPrimary !== false && (e.type === 'pointerdown' || e.type === 'pointermove')
      && typeof office.onElevator === 'function' && office.onElevator(e.clientX, e.clientY);
    if (e.type === 'pointerdown' && lift) office.ride();
    if (e.type === 'pointermove') root.style.cursor = lift ? 'pointer' : '';
    // And upstairs, a desk held: its card says who hired its agent, till the next press anywhere.
    if (!office || !lastAt || e.isPrimary === false || typeof office.hold !== 'function') return;
    if (e.type === 'pointerdown') {
      clearTimeout(holding);
      office.hold(null);
      const desk = lift ? null : office.upstairsDeskAt(e.clientX, e.clientY);
      holding = desk ? setTimeout(() => office.hold(desk), HOLD_MS) : null;
    } else if (e.type === 'pointerup' || e.type === 'pointercancel') {
      clearTimeout(holding);
      holding = null;
    }
  };
  for (const type of ['pointerdown', 'pointerup', 'pointermove', 'pointercancel', 'touchstart', 'touchmove', 'touchend',
    'mousedown', 'mouseup', 'click', 'dblclick', 'contextmenu', 'wheel', 'gesturestart', 'keydown', 'keyup', 'keypress',
    'selectstart', 'dragstart']) {
    window.addEventListener(type, stop, { capture: true, passive: false });
  }

  /* ------------------------------------------------------------- states */

  let office = null;   // made on the first good frame
  let key = null;      // CryptoKey, non-extractable
  let id = '';
  let lastAt = 0;      // the newest `at` shown; 0 when nothing is (drives the label)
  let highestAt = 0;   // the newest `at` ever shown for this id, here or before a reload; "Not active" never resets it
  let misses = 0;
  let timer = null;
  let polling = false;
  let ticking = null;

  function wipe() {
    if (office) {
      office.close();
      office.setVisible(false);
    }
    // Resetting its size empties the bitmap, whatever was drawn on it: the room's
    // canvas, and any the office drew beside it, as its art layer.
    for (const drawn of document.querySelectorAll('.room canvas')) {
      drawn.width = 1;
      drawn.height = 1;
    }
  }

  /** The newest `at` shown for `id` in this browser, kept across reloads; 0 where storage can't be used. */
  const SEEN = 'viewer-newest:';
  function newestSeen(id) {
    try { return Number(localStorage.getItem(SEEN + id)) || 0; } catch { return 0; }
  }
  function rememberNewest(id, at) {
    try { localStorage.setItem(SEEN + id, String(at)); } catch {}
  }

  function notActive() {
    lastAt = 0;
    wipe();
    // Once more after any frame already asked for.
    requestAnimationFrame(() => { if (!lastAt) wipe(); });
    root.classList.remove('is-live');
    label.textContent = '';
    if (ticking) { clearInterval(ticking); ticking = null; }
  }

  function ago(ms) {
    const s = Math.max(0, Math.round(ms / 1000));
    if (s < 60) return `${s} s ago`;
    const m = Math.round(s / 60);
    return m < 60 ? `${m} min ago` : 'a while ago';
  }

  /** Once a second while live: the label, and "Not active" once the newest frame is too old. */
  function tickLabel() {
    if (!lastAt) return;
    const age = Date.now() - lastAt;
    if (age > MAX_AGE + SKEW) return notActive();
    label.textContent = `Live · ${ago(age)}`;
  }

  /** Headless runs upstairs, as the snapshot has them: a name and a state each, and how many more there are. */
  function upstairsOf(given) {
    const desks = given && Array.isArray(given.desks) ? given.desks : [];
    return {
      desks: desks.filter((d) => d && typeof d === 'object').slice(0, 12)
        .map((d) => ({
          name: typeof d.name === 'string' ? d.name : '',
          state: typeof d.state === 'string' ? d.state : 'idle',
          // Who hired it, by name only, for its card.
          hiredBy: d.hiredBy && typeof d.hiredBy.name === 'string' ? { name: d.hiredBy.name.slice(0, 64) } : null,
        })),
      overflow: given && Number.isFinite(given.overflow) && given.overflow > 0 ? Math.floor(given.overflow) : 0,
    };
  }

  /** The office as the viewer draws it: nothing to press, and silent whatever the room says. */
  function viewOnly(snapshot) {
    const layout = snapshot.layout && typeof snapshot.layout === 'object' ? snapshot.layout : {};
    const room = layout.room && typeof layout.room === 'object' ? layout.room : {};
    const keys = Array.isArray(snapshot.keys) ? snapshot.keys : [];
    return {
      ...snapshot,
      keys: keys.map((k) => (k && k.session && typeof k.session === 'object'
        ? { ...k, session: { ...k.session, dial: false, mic: false, dictates: false, keys: false, fire: false, summary: false } }
        : k)),
      // As deck.html does: the office's own colours.
      upstairs: upstairsOf(snapshot.upstairs),
      theme: {},
      scroll: false,
      dictation: null,
      hiring: { fire: false, hire: false, roles: [] },
      layout: { ...layout, rows: [], room: { ...room, sound: 'off' } },
    };
  }

  function live(payload) {
    if (!office) {
      office = window.AgentOffice.create(canvas, {});
      office.setFramed(false);
      office.setFloating('screen');
    }
    const wasLive = !!lastAt;
    lastAt = payload.at;
    highestAt = payload.at;
    rememberNewest(id, payload.at);
    root.classList.add('is-live');
    office.show(viewOnly(payload.office));
    if (!wasLive) office.setVisible(!document.hidden);
    tickLabel();
    if (!ticking) ticking = setInterval(tickLabel, 1000);
  }

  /* ------------------------------------------------------------ polling */

  function schedule() {
    clearTimeout(timer);
    timer = null;
    if (document.hidden) return;
    timer = setTimeout(poll, misses >= MISSES ? SLOW : POLL);
  }

  function miss(gone) {
    misses += 1;
    // Gone from the relay (stopped, expired) is "Not active" at once; a
    // hiccup keeps the last frame until it's too old.
    if (gone) notActive();
  }

  async function poll() {
    if (polling) return;
    polling = true;
    try {
      const reply = await fetch(`/api/s/${id}`, {
        method: 'GET', cache: 'no-store', credentials: 'omit', referrerPolicy: 'no-referrer', redirect: 'error',
      });
      if (reply.status === 404) return miss(true);
      if (!reply.ok) return miss(false);
      const body = new Uint8Array(await reply.arrayBuffer());
      const plain = await C.openBody(body, key, id, crypto.subtle);
      const payload = plain && C.parsePayload(plain);
      if (plain) plain.fill(0);
      if (!payload) return miss(true); // wrong key, other id, tampered, other version: alike
      const now = Date.now();
      if (payload.at > now + SKEW || now - payload.at > MAX_AGE + SKEW) return miss(true);
      // Older than, or the same as, anything ever shown (even before a
      // "Not active"): a replay, ignored. It never brings back "Live".
      if (payload.at <= highestAt) { misses = 0; return; }
      misses = 0;
      live(payload);
    } catch {
      miss(false);
    } finally {
      polling = false;
      schedule();
    }
  }

  document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
      clearTimeout(timer);
      timer = null;
      if (office && lastAt) office.setVisible(false);
    } else {
      if (office && lastAt) office.setVisible(true);
      tickLabel();
      poll();
    }
  });

  /* -------------------------------------------------------------- start */

  async function start() {
    notActive();
    const path = /^\/v\/([A-Za-z0-9_-]{22})$/.exec(location.pathname);
    const hash = /^#k=([A-Za-z0-9_-]{43})$/.exec(location.hash);
    if (!path || !hash || !C || !window.AgentOffice || !window.crypto || !crypto.subtle) return;
    id = path[1];
    highestAt = newestSeen(id);
    key = await C.importKey(hash[1], crypto.subtle);
    if (!key) return;
    poll();
  }

  // A different link pasted over this one: start again from it.
  window.addEventListener('hashchange', () => location.reload());
  start();
})();
