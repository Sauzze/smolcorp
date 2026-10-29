/*
 * Smolcorp for iPhone: the deck page's network, handed to the app.
 *
 * The page is the Mac's own deck.html, unchanged. Here, before it runs, its
 * EventSource and its POSTs go to the app instead, which holds the key,
 * talks to the Mac over the Wi-Fi or Tailscale, and reconnects. The page
 * never sees the key, and never reaches the network itself.
 */
(() => {
  const handlers = window.webkit.messageHandlers;
  const streams = new Set();

  class AppEventSource extends EventTarget {
    constructor(url) {
      super();
      this.url = String(url);
      this.readyState = AppEventSource.CONNECTING;
      this.onopen = this.onmessage = this.onerror = null;
      streams.add(this);
      handlers.deck.postMessage({ op: 'events' });
    }
    close() {
      this.readyState = AppEventSource.CLOSED;
      streams.delete(this);
    }
  }
  AppEventSource.CONNECTING = 0;
  AppEventSource.OPEN = 1;
  AppEventSource.CLOSED = 2;
  window.EventSource = AppEventSource;

  function fire(stream, event) {
    stream.dispatchEvent(event);
    const handler = stream['on' + event.type];
    if (handler) handler.call(stream, event);
  }

  // What the app calls with the stream's news.
  window.__smolcorp = {
    message(data) {
      for (const stream of [...streams]) {
        stream.readyState = AppEventSource.OPEN;
        fire(stream, new MessageEvent('message', { data }));
      }
    },
    ping() {
      for (const stream of [...streams]) fire(stream, new MessageEvent('ping', { data: '1' }));
    },
    // Out of reach: the page says so, and keeps its stream for when it's back.
    error() {
      for (const stream of [...streams]) {
        stream.readyState = AppEventSource.CONNECTING;
        fire(stream, new Event('error'));
      }
    },
  };

  /*
   * The office the page makes, and what the page told it to call: kept as
   * office.js hands AgentOffice over, so the app can do what a tap would, in
   * the page's own way. A desk picked (a notification tapped), the lite
   * office switched, the styles' panel opened (from the app's settings).
   */
  let made = null;
  let agentOffice;
  Object.defineProperty(window, 'AgentOffice', {
    configurable: true,
    get: () => agentOffice,
    set(value) {
      agentOffice = value && {
        ...value,
        create(canvas, on) {
          const office = value.create(canvas, on);
          made = { canvas, on, office };
          return office;
        },
      };
    },
  });
  const LITE_KEY = 'office-lite';
  const appCalls = {
    /** Picks the desk of session `id`, as a tap on it does: false while the office has no such desk drawn yet. */
    select(id) {
      if (!made || !made.on.select) return false;
      made.on.select(id);
      return true;
    },
    /** The lite office, as the page keeps it: on unless this phone said otherwise. */
    liteOn() {
      let kept = null;
      try { kept = localStorage.getItem(LITE_KEY); } catch {}
      return kept ? kept === 'on' : true;
    },
    setLite(on) {
      if (made && made.on.lite) return made.on.lite(!!on);
      try { localStorage.setItem(LITE_KEY, on ? 'on' : 'off'); } catch {}
    },
    /** A tap on the styles' key, at `x`, `y` in the page: the office opens its panel as for a finger. */
    openStyles(x, y) {
      if (!made) return false;
      const { canvas } = made;
      const capture = canvas.setPointerCapture;
      canvas.setPointerCapture = () => {};
      try {
        const at = { clientX: x, clientY: y, pointerId: 7101, pointerType: 'mouse', isPrimary: true, button: 0, bubbles: true };
        canvas.dispatchEvent(new PointerEvent('pointerdown', at));
        canvas.dispatchEvent(new PointerEvent('pointerup', at));
      } finally {
        canvas.setPointerCapture = capture;
      }
      return true;
    },
  };
  Object.assign(window.__smolcorp, appCalls);

  // POSTs go through the app; the page's own files come from the app too.
  const pageFetch = window.fetch.bind(window);
  window.fetch = async (input, init = {}) => {
    const method = String(init.method || (input instanceof Request && input.method) || 'GET').toUpperCase();
    if (method !== 'POST') return pageFetch(input, init);
    const url = new URL(input instanceof Request ? input.url : String(input), location.href);
    const body = init.body == null ? null : String(init.body);
    const answer = await handlers.deckFetch.postMessage({ path: url.pathname, query: url.search, body });
    if (!answer) throw new TypeError('Load failed');
    const empty = [204, 205, 304].includes(answer.status);
    return new Response(empty ? null : answer.body, {
      status: answer.status,
      headers: { 'Content-Type': answer.type || 'text/plain; charset=utf-8' },
    });
  };

  // The app's own haptics, for the page's taps and reminders.
  navigator.vibrate = (pattern) => {
    handlers.haptic.postMessage(Array.isArray(pattern) ? pattern.length : 1);
    return true;
  };
  // An app, not a tab: no "Add to Home Screen" hint.
  Object.defineProperty(Navigator.prototype, 'standalone', { get: () => true, configurable: true });
})();
