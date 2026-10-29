/*
 * Smolcorp for iPhone: where the office's styles key is on the screen, for
 * the app's mic button to sit beside it. The office draws the key in its
 * canvas and keeps where to itself, so this works it out the office's own way
 * (office.js: fitRoom, drawOffice's window, drawStyleKey) for the deck's
 * phone page: floating on the screen, 6 px of margin, the room held upright
 * when the canvas is taller than wide. Sent to the app whenever it moves.
 */
(() => {
  const WIDE = { width: 320, height: 180 }, TALL = { width: 224, height: 372 };
  const MARGIN = 6, REACH = 1.5, KEY = { w: 15, h: 13, inset: 6 };
  let last = '';

  function measure() {
    const canvas = document.getElementById('office-canvas');
    if (!canvas || !canvas.width || !canvas.height) return null;
    const rect = canvas.getBoundingClientRect();
    const ratio = window.devicePixelRatio || 1;
    const width = canvas.width, height = canvas.height;
    const shape = height > width ? TALL : WIDE;
    const inset = MARGIN * ratio;
    const fit = Math.min((width - inset * 2) / shape.width, (height - inset * 2) / shape.height);
    const scale = fit >= 1 ? Math.floor(fit) : fit;
    const ox = Math.round((width - shape.width * scale) / 2), oy = Math.round((height - shape.height * scale) / 2);
    const w = Math.min(width - inset * 2, shape.width * scale * REACH), h = Math.min(height - inset * 2, shape.height * scale * REACH);
    const left = Math.floor((Math.round((width - w) / 2) - ox) / scale), top = Math.floor((Math.round((height - h) / 2) - oy) / scale);
    const css = rect.width / width;
    return {
      x: rect.left + (ox + (left + KEY.inset) * scale) * css,
      y: rect.top + (oy + (top + KEY.inset) * scale) * css,
      w: KEY.w * scale * css,
      h: KEY.h * scale * css,
    };
  }

  function report() {
    const key = measure();
    const text = JSON.stringify(key);
    if (text === last) return;
    last = text;
    window.webkit.messageHandlers.styleKey.postMessage(key);
  }

  addEventListener('resize', () => requestAnimationFrame(report));
  addEventListener('load', report);
  setInterval(report, 1000);
})();
