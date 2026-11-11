/**
 * Each emergency button but the red dome: `look(x, y, sink, since)` draws it in its 15 × 17 art pixels (`sink` is 1
 * while it's pressed, `since` how long ago it was, or Infinity once its reaction is over) and `react({ t, fade, x, y })`
 * plays the room's reaction over everything, local and slow enough not to strobe. See BUTTON_FEEL for the rest.
 */
const BUTTON_STYLES = {
  /** A big red mushroom on a yellow housing, like a machine's stop. Alarm lights turn red over the floor. */
  mushroom: {
    look(x, y, sink) {
      fill(x - 6, y + 10, 13, 6, 0xe0b020); fill(x - 6, y + 10, 13, 1, 0xf5d24a); fill(x - 6, y + 10, 1, 6, 0xf5d24a);
      fill(x + 6, y + 10, 1, 6, 0xa88410); fill(x - 6, y + 15, 13, 1, 0xa88410);
      for (const dx of [-4, 4]) dot(x + dx, y + 12, 0x7a5a08);
      fill(x - 3, y + 8, 7, 2, 0x6e727c); fill(x - 1, y + 6, 3, 2, 0x4a4e58);
      fill(x - 3, y + 1 + sink, 7, 1, 0xe8503c); fill(x - 5, y + 2 + sink, 11, 3, 0xd8301f); fill(x - 5, y + 5 + sink, 11, 1, 0x8a1a10);
      dot(x - 3, y + 2 + sink, 0xff9a8a); dot(x - 2, y + 2 + sink, 0xff9a8a); dot(x - 4, y + 3 + sink, 0xff9a8a, 0.7);
    },
    react({ t, fade, x, y }) {
      glowAt(x, y + 8, 14, 9, 0xff2a1a, 0.16 * fade * (0.65 + 0.35 * Math.sin(t * 6)));
      // Two beams from the lamp, turning round the floor
      const turn = t * 5;
      for (const side of [0, Math.PI]) {
        const dx = Math.cos(turn + side), dy = Math.sin(turn + side) * 0.5;
        for (let d = 4; d < 34; d++) {
          const a = 0.5 * (1 - d / 34) * fade;
          dot(x + dx * d, y + 9 + dy * d, 0xff4a30, a);
          if (d > 10) dot(x + dx * d, y + 9 + dy * d + 1, 0xff4a30, a * 0.6);
          if (d > 20) dot(x + dx * d, y + 9 + dy * d - 1, 0xff4a30, a * 0.6);
        }
      }
      dot(x, y + 4, 0xffffff, 0.8 * fade);
      onlookers(['exclaim'], [0xff5a40], 0.3, 3.2);
    },
  },

  /** A red fire alarm box on a post with a white handle that pulls down. A sprinkler drizzles over the floor. */
  pullstation: {
    look(x, y, sink) {
      fill(x - 1, y + 12, 3, 3, 0x6e727c); fill(x - 3, y + 15, 7, 1, 0x4a4e58);
      fill(x - 5, y + 1, 11, 12, 0xd8402a); fill(x - 5, y + 1, 11, 1, 0xf06a50); fill(x - 5, y + 1, 1, 12, 0xf06a50);
      fill(x + 5, y + 1, 1, 12, 0xa02a18); fill(x - 5, y + 12, 11, 1, 0xa02a18);
      fill(x - 3, y + 3, 7, 2, 0xeae6dc); dot(x - 2, y + 3, WHITE, 0.8);
      fill(x - 2, y + 6, 5, 4, 0x6a1a10);
      fill(x - 3, y + 6 + sink * 2, 7, 3, 0xeae6dc); fill(x - 3, y + 8 + sink * 2, 7, 1, 0xc4c0b4); dot(x - 2, y + 6 + sink * 2, WHITE);
    },
    react({ t, fade, x, y }) {
      // The sprinkler head, and its drizzle
      const top = y - 30, floor = y + 14;
      fill(x - 1, top - 2, 3, 2, 0xc8a050); fill(x, top, 1, 1, 0x8a6a2a);
      for (let k = 0; k < 44; k++) {
        const r1 = hash(k, 3) / 4294967296, r2 = hash(k, 7) / 4294967296;
        const spread = (r1 - 0.5) * 34, land = floor + Math.floor(r2 * 4), run = land - top;
        const py = top + 1 + ((t * 46 + r2 * run) % run);
        const px = x + spread * (0.25 + 0.75 * (py - top) / run);
        fill(px, py, 1, 2, 0x9fd8ff, 0.8 * fade);
        if (land - py < 3) dot(px + (k % 2 ? 1 : -1), land - 1, WHITE, 0.6 * fade);
      }
      const rx = Math.min(15, 4 + t * 6);
      fill(x - rx, y + 15, rx * 2, 2, 0x9fd8ff, 0.28 * fade); fill(x - rx + 2, y + 15, rx * 2 - 4, 1, WHITE, 0.2 * fade);
      onlookers(['drop'], [0x9fd8ff], 0.4, 3);
    },
  },

  /** A red case with a glass front and a hammer on a chain; behind it a red button. The glass flies. */
  glassbox: {
    look(x, y, sink, s) {
      const broken = s < Infinity;
      fill(x - 1, y + 13, 3, 2, 0x6e727c); fill(x - 3, y + 15, 7, 1, 0x4a4e58);
      fill(x - 5, y + 2, 11, 11, 0xd8402a); fill(x - 5, y + 2, 11, 1, 0xf06a50); fill(x - 5, y + 2, 1, 11, 0xf06a50);
      fill(x + 5, y + 2, 1, 11, 0xa02a18); fill(x - 5, y + 12, 11, 1, 0xa02a18);
      fill(x - 3, y + 4, 7, 7, 0x2a1410);
      fill(x - 1, y + 7 + (broken ? 1 : 0), 3, 2, 0xff5a40); dot(x - 1, y + 7 + (broken ? 1 : 0), 0xffb0a0);
      if (!broken) {
        fill(x - 3, y + 4, 7, 7, 0xdff0f6, 0.42); dot(x - 2, y + 5, WHITE, 0.9); dot(x - 1, y + 6, WHITE, 0.6);
      } else {
        for (const [dx, dy] of [[-3, 4], [3, 4], [-3, 10], [3, 10], [-2, 4], [2, 10]]) dot(x + dx, y + dy, 0xdff0f6, 0.7);
        for (const dx of [-6, -3, 2, 5]) dot(x + dx, y + 16, 0xdff0f6, 0.7);
      }
      fill(x + 6, y + 3, 1, 6, 0x8a8f9c); fill(x + 6, y + 9, 2, 2, 0x5a3b25);
    },
    react({ t, fade, x, y }) {
      const cx = x, cy = y + 7;
      if (t < 0.14) glowAt(cx, cy, 10, 8, WHITE, 0.5 * (1 - t / 0.14));
      spray(t, { x: cx, y: cy, n: 26, seed: 4, angle: [-Math.PI * 0.95, -Math.PI * 0.05], speed: [16, 46], g: 120, life: 1.7,
        colors: [0xdff0f6, 0xbfe0ee, WHITE, 0x9fd0e6], floor: y + 15, alpha: fade });
      spray(t, { x: cx, y: cy, n: 8, seed: 12, angle: [-Math.PI * 0.9, -Math.PI * 0.1], speed: [12, 30], g: 120, life: 1.7,
        colors: [0xdff0f6, WHITE], size: 2, floor: y + 15, alpha: fade });
      for (let k = 0; k < 5; k++) {
        const step = Math.floor(t * 8 + k * 3);
        if (step % 4 === 0 && t < 2) dot(cx - 11 + (hash(step, k) % 23), cy - 9 + (hash(k, step) % 17), WHITE, 0.9 * fade);
      }
      onlookers(['exclaim'], [0xf2d35b], 0.4, 2.8);
    },
  },

  /** A launch console on a post with two keys in it and a missile lamp. The keys turn, then a rocket goes up. */
  keyswitch: {
    look(x, y, sink, s) {
      const turned = s < Infinity, armed = s > 0.7 && s < Infinity;
      fill(x - 1, y + 13, 3, 2, 0x4a4e58); fill(x - 4, y + 15, 9, 1, 0x4a4e58);
      fill(x - 6, y + 6, 13, 8, 0x6e727c); fill(x - 6, y + 6, 13, 1, 0xa8adb6); fill(x - 6, y + 6, 1, 8, 0xa8adb6);
      fill(x + 6, y + 6, 1, 8, 0x4a4e58); fill(x - 6, y + 13, 13, 1, 0x4a4e58);
      for (const [kx, flip] of [[-4, -1], [4, 1]]) {
        dot(x + kx, y + 8, 0x1d1f25);
        if (!turned) { fill(x + kx, y + 5, 1, 3, 0xe8c878); fill(x + kx - 1, y + 3, 3, 2, 0xc8a050); dot(x + kx - 1, y + 3, 0xfff0b0); }
        else { fill(x + kx, y + 7, 1, 1, 0xe8c878); fill(x + kx + (flip < 0 ? -3 : 1), y + 7, 3, 2, 0xc8a050); }
      }
      // A hazard stripe, and the missile lamps under it, red once it's armed
      for (let k = 0; k < 11; k++) dot(x - 5 + k, y + 10, k % 2 ? 0x2a2c30 : 0xf2c200);
      for (const dx of [-3, 0, 3]) fill(x + dx - 1, y + 12, 2, 1, armed ? 0xff5a2a : 0x6a1a14);
      if (armed) { if (isNight) glowLate(() => glowAt(x, y + 12, 8, 4, 0xff7a40, 0.3)); }
      else if (isNight) glowLate(() => glowAt(x, y + 12, 6, 3, 0xff5a2a, 0.08));
    },
    react({ t, fade, x, y }) {
      if (t > 0.8 && t < 1.95) {
        // The rocket, lifting off the console
        const rise = t - 0.8, ry = y + 4 - rise * rise * 28 - rise * 20;
        for (let i = 0; i < 10; i++) fill(x, ry + 2 + i * 1.5, 1, 1, i < 3 ? 0xffe090 : 0xff8a30, 1 - i * 0.09);
        fill(x - 1, ry + 3, 3, 1, 0xff8a30, 0.5);
        fill(x, ry - 1, 1, 3, WHITE); dot(x, ry - 2, 0xffd0a0);
        glowAt(x, ry + 2, 4, 5, 0xffa040, 0.25);
      }
      const top = y - 60, since = t - 1.95;
      if (since > 0 && since < 1.4) {
        if (since < 0.18) glowAt(x, top, 12, 10, 0xfff0c0, 0.5);
        const r = since * 20;
        ellipseRing(x, top, r, r * 0.8, 0xffd070, Math.max(0, 0.8 - since * 0.6));
        spray(since, { x, y: top, n: 26, seed: 5, angle: [0, Math.PI * 2], speed: [8, 28], g: 20, life: 1.4, drag: 1.4,
          colors: [0xffd070, 0xff8a30, WHITE, 0xff5a2a] });
      }
      glowAt(x, y + 11, 8, 5, 0xff7a40, 0.12 * fade);
      onlookers(['exclaim', 'bolt'], [0xff9a40, 0xf2d35b], 0.3, 3.2);
    },
  },

  /** A big round green button in a pale dish, with a tick on it. A cheer: stars and sparkles burst round it. */
  easy: {
    look(x, y, sink) {
      fill(x - 7, y + 11, 15, 5, 0xe9edf2); fill(x - 7, y + 11, 15, 1, WHITE, 0.7); fill(x + 7, y + 11, 1, 5, 0xb4bac4); fill(x - 7, y + 15, 15, 1, 0xb4bac4);
      fill(x - 3, y + 4 + sink, 7, 1, 0x4cd088); fill(x - 5, y + 5 + sink, 11, 5, 0x34b36a); fill(x - 6, y + 7 + sink, 13, 3, 0x34b36a);
      fill(x - 6, y + 10 + sink, 13, 1, 0x1f7f48); fill(x + 5, y + 6 + sink, 1, 4, 0x2a9a58);
      for (const [dx, dy] of [[-2, 7], [-1, 8], [0, 7], [1, 6], [2, 5]]) dot(x + dx, y + dy + sink, WHITE);
      dot(x - 4, y + 6 + sink, 0x9af0c0);
    },
    react({ t, fade, x, y }) {
      glowAt(x, y + 7, 18, 11, 0xf2d35b, 0.12 * fade);
      spray(t, { x, y: y + 4, n: 34, seed: 8, angle: [-Math.PI * 0.92, -Math.PI * 0.08], speed: [24, 58], g: 70, life: 2, drag: 0.9,
        colors: [0xf2d35b, 0xff7ab8, 0x6ad1ff, WHITE, 0x8be88b], alpha: fade });
      for (let k = 0; k < 6; k++) {
        const age = t - 0.15 - k * 0.12;
        if (age < 0 || age > 1) continue;
        const px = x - 16 + (hash(k, 2) % 33), py = y - 4 - (hash(k, 6) % 22);
        const arm = age < 0.5 ? 1 : 2;
        fill(px - arm, py, arm * 2 + 1, 1, WHITE, (1 - age) * fade); fill(px, py - arm, 1, arm * 2 + 1, 0xf2d35b, (1 - age) * fade);
      }
      onlookers(['star', 'heart'], [0xf2d35b, 0xff7ab8], 0.3, 3);
    },
  },

  /** A gumball-style globe of sweets on a red base with a coin knob. Sweets rain down and bounce. */
  candy: {
    look(x, y, sink) {
      fill(x - 4, y + 10, 9, 5, 0xc83a3a); fill(x - 4, y + 10, 9, 1, 0xe85a5a); fill(x - 4, y + 10, 1, 5, 0xe85a5a);
      fill(x + 4, y + 10, 1, 5, 0x8a2020); fill(x - 4, y + 14, 9, 1, 0x8a2020);
      fill(x - 1, y + 11, 3, 2, 0xd6d9e0); dot(x + (sink ? 1 : -1), y + 11, 0x8a8f9c);
      fill(x - 2, y + 13, 5, 1, 0x3a1410);
      disc(x, y + 5, 5, 0xdff0f6, 0.4);
      for (const [dx, dy, c] of [[-2, 7, 0xff7ab8], [0, 8, 0x6ad1ff], [2, 7, 0xf2d35b], [-3, 5, 0x8be88b], [1, 5, 0xff7a40], [-1, 3, 0xb08aff], [3, 4, 0xff7ab8]]) fill(x + dx, y + dy, 2, 2, c);
      ring(x, y + 5, 5, WHITE, 0.55); dot(x - 3, y + 2, WHITE, 0.9); fill(x - 3, y + 3, 1, 2, WHITE, 0.6);
      fill(x - 2, y + 9, 5, 1, 0xdff0f6, 0.5);
    },
    react({ t, fade, x, y }) {
      const colours = [0xff7ab8, 0x6ad1ff, 0xf2d35b, 0x8be88b, 0xff7a40, 0xb08aff];
      for (let k = 0; k < 30; k++) {
        const r1 = hash(k, 5) / 4294967296, r2 = hash(k, 9) / 4294967296, r3 = hash(k, 14) / 4294967296;
        const age = t - 0.1 - r2 * 1.2;
        if (age < 0 || age > 2.4) continue;
        const px = x - 18 + r1 * 36, top = y - 38 - r2 * 8, land = y + 12 + r3 * 4, G = 150;
        const fall = Math.sqrt((2 * (land - top)) / G), after = age - fall;
        const py = after < 0 ? top + 0.5 * G * age * age : land - Math.abs(Math.sin(after * 7)) * Math.max(0, 5 - after * 6);
        const c = colours[k % colours.length];
        fill(px, py, 2, 2, c, fade); dot(px, py, WHITE, 0.5 * fade);
        if (k % 3 === 0) { dot(px - 1, py, shaded(c, 0.85), fade); dot(px + 2, py + 1, shaded(c, 0.85), fade); }
      }
      glowAt(x, y + 6, 12, 8, 0xff7ab8, 0.1 * fade);
      onlookers(['heart', 'star'], [0xff7ab8, 0xf2d35b], 0.5, 3.4);
    },
  },

  /** A slot machine with a lever. Its reels line up and coins pour out of it, jackpot. */
  slots: {
    look(x, y, sink, s) {
      const won = s > 0.5 && s < Infinity;
      fill(x - 6, y + 1, 9, 14, 0xc8302a); fill(x - 6, y + 1, 9, 1, 0xe8c050); fill(x - 6, y + 1, 1, 14, 0xe8604a); fill(x + 2, y + 1, 1, 14, 0x8a1e1a);
      fill(x - 6, y + 14, 9, 1, 0x8a1e1a);
      for (let k = 0; k < 4; k++) dot(x - 5 + k * 2, y + 2, (Math.floor(now * (won ? 6 : 2)) + k) % 2 ? 0xfff0a0 : 0xe8a020);
      fill(x - 5, y + 4, 7, 5, 0xeae6dc);
      const symbols = won ? [0xf2d35b, 0xf2d35b, 0xf2d35b] : [0xd8402a, 0xf2d35b, 0x4c8bf5];
      symbols.forEach((c, i) => { fill(x - 4 + i * 2, y + 5, 1, 3, c); if (won) dot(x - 4 + i * 2, y + 5, WHITE); });
      fill(x - 5, y + 10, 7, 1, 0xe8c050); fill(x - 3, y + 11, 3, 1, 0x1d1f25); fill(x - 4, y + 13, 5, 1, 0x3a1410);
      // The lever, up until it's pulled
      if (!sink) { fill(x + 5, y + 4, 1, 5, 0xa8adb6); disc(x + 5, y + 2, 1, 0xd8402a); dot(x + 4, y + 1, 0xff9a8a); }
      else { fill(x + 5, y + 8, 1, 2, 0xa8adb6); disc(x + 5, y + 10, 1, 0xd8402a); dot(x + 4, y + 9, 0xff9a8a); }
      if (isNight) glowLate(() => { glowAt(x - 2, y + 6, 6, 4, 0xfff0c0, won ? 0.25 : 0.1); });
    },
    react({ t, fade, x, y }) {
      glowAt(x - 2, y + 7, 14, 10, 0xf2d35b, 0.14 * fade);
      spray(t, { x: x - 2, y: y + 13, n: 26, seed: 6, angle: [-Math.PI * 0.8, -Math.PI * 0.2], speed: [30, 64], g: 120, life: 2.2,
        colors: [0xf2d35b, 0xe8c050, 0xfff0a0], size: 2, floor: y + 14, stagger: 0.8, delay: 0.5, alpha: fade });
      for (let k = 0; k < 4; k++) {
        const step = Math.floor(t * 6 + k * 2);
        if (step % 3 === 0 && t > 0.5) dot(x - 14 + (hash(step, k) % 24), y + 2 + (hash(k, step) % 12), WHITE, 0.9 * fade);
      }
      onlookers(['coin'], [0xf2d35b], 0.7, 3.2);
    },
  },

  /** A lit disco tile and a little mirror ball over it. Coloured lights wheel round the floor. */
  disco: {
    look(x, y, sink, s) {
      const on = s < Infinity, step = Math.floor(now * 2.5);
      const cols = [0xd878b8, 0x58b8d8, 0xd8c048, 0x8860d0, 0x68c878, 0xd8802a];
      fill(x, y + 3, 1, 9, 0x6e727c);
      disc(x, y + 3, 3, 0xc8ccd6); for (const [dx, dy] of [[-2, 2], [0, 3], [2, 2], [-1, 4], [1, 1]]) dot(x + dx, y + dy, 0x8a8f9c);
      dot(x - 1, y + 1, WHITE);
      fill(x - 6, y + 11, 13, 5, 0x2a2f45);
      for (let c = 0; c < 3; c++) for (let r = 0; r < 2; r++) {
        const col = cols[(c + r * 3 + (on ? step : 0)) % cols.length];
        fill(x - 6 + c * 4 + (c ? 0 : 0), y + 11 + r * 2 + (r ? 0 : 0), 4, 2, on ? col : shaded(col, 0.62));
      }
      fill(x - 6, y + 15, 13, 1, 0x161a2a);
      if (isNight) glowLate(() => glowAt(x, y + 13, 8, 4, cols[step % cols.length], on ? 0.22 : 0.07));
    },
    react({ t, fade, x, y }) {
      const cols = [0xff7ab8, 0x6ad1ff, 0xf2d35b, 0x8860d0, 0x8be88b];
      cols.forEach((c, i) => {
        const a = t * 1.5 + i * 1.2566, r = 12 + 6 * Math.sin(i * 1.7 + t * 0.8);
        const px = x + Math.cos(a) * r * 1.5, py = y + 8 + Math.sin(a) * r * 0.65;
        disc(px, py, 4, c, 0.3 * fade); disc(px, py, 2, c, 0.4 * fade);
      });
      const step = Math.floor(t * 4);
      for (let k = 0; k < 7; k++) dot(x - 12 + (hash(step, k) % 25), y - 6 + (hash(k, step + 3) % 19), WHITE, 0.85 * fade);
      onlookers(['note'], [0xff7ab8, 0x6ad1ff, 0xf2d35b], 0.3, 3.4);
    },
  },

  /** A party popper standing on end, striped, with a pull cord. Confetti bursts out of its mouth. */
  confetti: {
    look(x, y, sink) {
      const stripes = [0xd8402a, 0xf2d35b, 0x4c8bf5];
      for (let r = 0; r < 12; r++) {
        const half = 4 - Math.floor(r * 0.28);
        for (let c = -half; c <= half; c++) dot(x + c, y + 3 + r, stripes[(Math.floor((c + r * 1) / 2) + 9) % 3]);
      }
      fill(x - 4, y + 2, 9, 1, 0xeae6dc); fill(x - 3, y + 3, 7, 1, 0x2a1a2a); dot(x - 4, y + 3, 0xeae6dc);
      fill(x - 3, y + 15, 7, 1, 0x5a3b25);
      fill(x + 5, y + 6 + sink * 2, 1, 5, 0xeae6dc); dot(x + 5, y + 11 + sink * 2, 0xf2d35b); dot(x + 4, y + 6 + sink * 2, 0xeae6dc);
    },
    react({ t, fade, x, y }) {
      if (t < 0.12) glowAt(x, y + 2, 8, 6, WHITE, 0.4 * (1 - t / 0.12));
      spray(t, { x, y: y + 2, n: 72, seed: 10, angle: [-Math.PI * 0.78, -Math.PI * 0.22], speed: [34, 82], g: 24, life: 3.2, drag: 1.2, sway: 3,
        colors: [0xd8402a, 0xf2d35b, 0x4c8bf5, 0x8be88b, 0xff7ab8, WHITE], flip: true, floor: y + 15, alpha: fade });
      onlookers(['star'], [0xf2d35b, 0xff7ab8], 0.4, 3.4);
    },
  },

  /** A bronze gong in a wooden frame. It hums and rings spread out from it. */
  gong: {
    look(x, y, sink, s) {
      const sway = s < 1.6 ? Math.round(Math.sin(s * 38) * (1 - s / 1.6) * 1.2) : 0;
      fill(x - 7, y, 2, 16, 0x5a3b25); fill(x + 6, y, 2, 16, 0x5a3b25); fill(x - 7, y, 15, 2, 0x8a5a32); fill(x - 7, y, 15, 1, 0xa8703e);
      fill(x - 7, y + 15, 3, 1, 0x3a2616); fill(x + 5, y + 15, 3, 1, 0x3a2616);
      fill(x - 3 + sway, y + 2, 1, 3, 0xd6b25e); fill(x + 3 + sway, y + 2, 1, 3, 0xd6b25e);
      disc(x + sway, y + 9, 5, 0xc8a050); ring(x + sway, y + 9, 5, 0xe8c878); disc(x + sway, y + 9, 3, 0xb08a38);
      disc(x + sway, y + 9, 1, 0x8a6a2a); dot(x - 3 + sway, y + 6, 0xfff0b0); dot(x - 4 + sway, y + 8, 0xe8c878);
    },
    react({ t, fade, x, y }) {
      for (let i = 0; i < 3; i++) {
        const age = (t - 0.1 - i * 0.5) / 1.7;
        if (age < 0 || age >= 1) continue;
        const r = 6 + age * 30;
        ellipseRing(x, y + 9, r, r * 0.75, 0xffe08a, (1 - age) * fade);
        ellipseRing(x, y + 9, r - 1, (r - 1) * 0.75, 0xfff0b0, (1 - age) * 0.35 * fade);
      }
      glowAt(x, y + 9, 12, 10, 0xe8c878, 0.12 * fade * (0.6 + 0.4 * Math.sin(t * 7)));
      onlookers(['note'], [0xe8c878], 0.4, 3.2);
    },
  },

  /** A bike horn on handlebars: a rubber bulb and a brass trumpet. It honks twice, in bubbles and sound rings. */
  bikehorn: {
    look(x, y, sink) {
      fill(x - 1, y + 9, 3, 6, 0x6e727c); fill(x - 3, y + 15, 7, 1, 0x4a4e58);
      fill(x - 7, y + 8, 15, 2, 0xa8adb6); fill(x - 7, y + 8, 15, 1, 0xd0d4dc); fill(x - 7, y + 7, 2, 4, 0x2a2c30); fill(x + 6, y + 7, 2, 4, 0x2a2c30);
      fill(x - 6, y + 4 + sink, 5, sink ? 3 : 4, 0x2a2c30); fill(x - 5, y + 3 + sink, 3, 1, 0x2a2c30); dot(x - 5, y + 4 + sink, 0x6a7080); dot(x - 1, y + 5 + sink, 0x1d1f25);
      fill(x - 1, y + 4 + sink, 3, 3, 0xe8c878); fill(x + 2, y + 3 + sink, 2, 5, 0xe8c878); fill(x + 4, y + 2 + sink, 3, 7, 0xd6b25e);
      fill(x + 6, y + 3 + sink, 1, 5, 0x5a3b25); dot(x - 1, y + 4 + sink, 0xfff0b0); dot(x + 4, y + 2 + sink, 0xfff0b0);
    },
    react({ t, fade, x, y }) {
      for (const start of [0.05, 1.3]) {
        for (let i = 0; i < 3; i++) {
          const age = (t - start - i * 0.15) / 0.9;
          if (age < 0 || age >= 1) continue;
          const out = 3 + age * 15;
          for (let dy = -3; dy <= 3; dy++) {
            const bend = Math.round(Math.abs(dy) * 0.7);
            dot(x + 8 + out - bend, y + 5 + dy, i === 0 ? WHITE : 0xf2d35b, (1 - age) * 0.9 * fade);
            dot(x - 9 - out * 0.4 + bend, y + 5 + dy, WHITE, (1 - age) * 0.3 * fade);
          }
        }
        const age = t - start - 0.05;
        if (age >= 0 && age < 0.9) {
          // A bubble over the horn, with the honk in it
          const bx = x - 5, by = y - 14 - (age < 0.1 ? 1 : 0);
          fill(bx + 1, by, 8, 7, WHITE, 0.95 * fade); fill(bx, by + 1, 10, 5, WHITE, 0.95 * fade); fill(bx + 2, by + 7, 2, 2, WHITE, 0.95 * fade);
          fill(bx + 3, by + 1, 1, 4, 0xd8402a, fade); dot(bx + 3, by + 6, 0xd8402a, fade);
          fill(bx + 6, by + 1, 1, 4, 0xd8402a, fade); dot(bx + 6, by + 6, 0xd8402a, fade);
        }
      }
      glowAt(x + 2, y + 5, 12, 8, 0xf2d35b, 0.1 * fade);
      onlookers(['exclaim', 'drop'], [0xf2d35b, 0x9fd8ff], 0.3, 2.6);
    },
  },

  /** A coral button with a small star of rays on it, in a dark housing. Rays burst out of it. */
  spark: {
    look(x, y, sink) {
      fill(x - 6, y + 10, 13, 6, 0x3a2e28); fill(x - 6, y + 10, 13, 1, 0x5a4a40); fill(x - 6, y + 10, 1, 6, 0x5a4a40);
      fill(x + 6, y + 10, 1, 6, 0x241c18); fill(x - 6, y + 15, 13, 1, 0x241c18);
      fill(x - 3, y + 8, 7, 2, 0x6a5a50);
      fill(x - 3, y + 1 + sink, 7, 1, 0xe8886a); fill(x - 5, y + 2 + sink, 11, 4, 0xd97757); fill(x - 5, y + 6 + sink, 11, 1, 0xa84e32);
      claudeSpark(x, y + 4 + sink, 2, 0xfbe9dc, 0.95);
      if (isNight) glowLate(() => glowAt(x, y + 5, 8, 5, 0xd97757, 0.14));
    },
    react({ t, fade, x, y }) {
      glowAt(x, y + 5, 16, 11, 0xd97757, 0.16 * fade);
      const rays = 14;
      for (let k = 0; k < rays; k++) {
        const a = (k / rays) * Math.PI * 2 + (k % 2) * 0.1, start = 3 + t * 22, len = k % 2 ? 4 : 7;
        if (t > 1.8) continue;
        for (let d = 0; d < len; d++) {
          dot(x + Math.cos(a) * (start + d), y + 5 + Math.sin(a) * (start + d) * 0.85, d < 2 ? 0xfbe9dc : 0xd97757, Math.max(0, 1 - t / 1.8) * fade);
        }
      }
      for (const [dx, dy, delay, r] of [[-13, -8, 0.2, 3], [12, -12, 0.45, 4], [-4, -22, 0.7, 5], [15, 2, 0.9, 2]]) {
        const age = t - delay;
        if (age < 0 || age > 1.2) continue;
        claudeSpark(x + dx, y + 5 + dy, r, age < 0.8 ? 0xfbe9dc : 0xd97757, Math.min(1, (1.2 - age) * 2) * fade, age < 0.3 ? 0 : -1);
      }
      spray(t, { x, y: y + 4, n: 16, seed: 3, angle: [-Math.PI * 0.9, -Math.PI * 0.1], speed: [14, 34], g: -8, life: 2.2, drag: 0.8,
        colors: [0xd97757, 0xfbe9dc, 0xe8886a], alpha: fade });
      onlookers(['star'], [0xd97757, 0xfbe9dc], 0.3, 3);
    },
  },

  /** An upright stone with the Elder Sign cut into it, glowing green. Tentacles feel their way up out of the floor. */
  elder: {
    look(x, y, sink, s) {
      const lit = s < Infinity, glyph = lit ? 0x8affc0 : 0x3a8a60;
      fill(x - 5, y + 2 + sink, 11, 14 - sink, 0x5a6360); fill(x - 4, y + 1 + sink, 9, 1, 0x5a6360); fill(x - 3, y + sink, 7, 1, 0x5a6360);
      fill(x - 5, y + 2 + sink, 1, 14 - sink, 0x7a8680); fill(x + 5, y + 2 + sink, 1, 14 - sink, 0x3f4846); fill(x - 5, y + 15, 11, 1, 0x3f4846);
      dot(x + 3, y + 5 + sink, 0x4a524f); dot(x + 3, y + 6 + sink, 0x4a524f); dot(x - 3, y + 12, 0x4a524f);
      // The sign: a branch, a star of five rays, and the eye in the middle
      const cy = y + 8 + sink;
      line(x, cy - 4, x, cy + 4, glyph); line(x, cy, x - 3, cy - 3, glyph); line(x, cy, x + 3, cy - 3, glyph);
      line(x, cy, x - 3, cy + 3, glyph); line(x, cy, x + 3, cy + 3, glyph); dot(x, cy, WHITE, lit ? 0.9 : 0.5);
      if (isNight) glowLate(() => glowAt(x, cy, 7, 7, 0x6fe0a0, lit ? 0.25 : 0.09));
    },
    react({ t, fade, x, y }) {
      glowAt(x, y + 8, 14, 12, 0x6fe0a0, 0.14 * fade * (0.7 + 0.3 * Math.sin(t * 5)));
      const floor = y + 16;
      [[-14, 0, 22, 0], [-9, 0.18, 17, 2], [10, 0.1, 19, 4], [15, 0.28, 23, 6]].forEach(([dx, delay, reach, seed]) => {
        const rise = Math.min(1, Math.max(0, (t - delay) / 0.55)) * Math.min(1, (BUTTON_FEEL.elder.length - t) / 0.6);
        const count = Math.floor(rise * (reach / 2));
        if (count < 1) return;
        fill(x + dx - 3, floor, 7, 1, 0x1a2a1a, 0.5 * fade);
        for (let s = 0; s < count; s++) {
          const w = s < count * 0.45 ? 3 : s < count * 0.8 ? 2 : 1;
          const px = x + dx + Math.round(Math.sin(t * 3.1 + s * 0.55 + seed) * (1 + s * 0.2)) + (dx < 0 ? 0 : 0);
          fill(px - Math.floor(w / 2), floor - s * 2 - 2, w, 2, s % 2 ? 0x5a7a52 : 0x4a6a4a, fade);
          if (w > 1) dot(px - Math.floor(w / 2), floor - s * 2 - 2, 0x7a9a72, fade);
          if (s % 3 === 1 && w > 1) dot(px + 1, floor - s * 2 - 1, 0xa8d8a0, 0.9 * fade);
        }
      });
      onlookers(['exclaim', 'question'], [0x8affc0, 0x8affc0], 0.5, 3.4);
    },
  },

  /** A dark panel with a green up arrow and a lever to pull. A chart of green candles shoots up from the floor. */
  buy: {
    look(x, y, sink, s) {
      const lit = s < Infinity;
      fill(x - 6, y + 3, 10, 13, 0x1d2438); fill(x - 6, y + 3, 10, 1, 0x3a4468); fill(x - 6, y + 3, 1, 13, 0x3a4468); fill(x + 3, y + 3, 1, 13, 0x0f1424);
      const up = lit ? 0x7affa8 : 0x3ddc7a;
      dot(x - 1, y + 5, up); fill(x - 2, y + 6, 3, 1, up); fill(x - 3, y + 7, 5, 1, up); fill(x - 2, y + 8, 3, 3, up);
      fill(x - 5, y + 12, 1, 3, 0x3ddc7a); fill(x - 3, y + 13, 1, 2, 0xd8402a); fill(x - 1, y + 11, 1, 4, 0x3ddc7a); fill(x + 1, y + 12, 1, 3, 0x3ddc7a);
      fill(x + 5, y + 5, 1, 10, 0x0d1018); fill(x + 4, y + 15, 3, 1, 0x4a4e58);
      const ky = sink ? y + 10 : y + 3;
      fill(x + 4, ky, 3, 3, 0x3ddc7a); dot(x + 4, ky, 0xb0ffd0); fill(x + 5, ky + 3, 1, sink ? 1 : 2, 0xa8adb6);
      if (isNight) glowLate(() => glowAt(x - 1, y + 8, 6, 6, 0x3ddc7a, lit ? 0.25 : 0.1));
    },
    react({ t, fade, x, y }) {
      const floor = y + 15;
      glowAt(x, y + 2, 20, 18, 0x3ddc7a, 0.07 * fade);
      const tops = [];
      for (let i = 0; i < 9; i++) {
        const px = x - 17 + i * 4, target = 6 + i * 4 + (hash(i, 4) % 4);
        const grow = Math.min(1, Math.max(0, (t - i * 0.1) / 0.4)), h = Math.round(target * (1 - (1 - grow) * (1 - grow)));
        if (h < 2) continue;
        const body = Math.max(2, Math.round(h * 0.5));
        candle(px, floor - h, body, i === 3 ? 0xd8402a : 0x3ddc7a, i === 3 ? 0xff8a7a : 0x9affc0, fade);
        tops.push([px + 1, floor - h]);
      }
      for (let i = 1; i < tops.length; i++) line(tops[i - 1][0], tops[i - 1][1], tops[i][0], tops[i][1], 0x9affc0, 0.45 * fade);
      if (tops.length === 9 && t > 1.3) buttonIcon('up', x + 17, floor - 54 - Math.floor(t * 4) % 2, 0x7affa8, fade);
      onlookers(['up', 'coin'], [0x7affa8, 0xf2d35b], 1, 3.4);
    },
  },

  /** A bear trap, jaws open, a plate in the middle. It snaps shut, and a chart of red candles crashes down. */
  beartrap: {
    look(x, y, sink, s) {
      const shut = s < 2.6, steel = 0x8a8f9c;
      fill(x - 7, y + 14, 15, 2, 0x4a4e58); fill(x - 7, y + 14, 15, 1, 0x6e727c);
      for (const side of [-1, 1]) {
        if (!shut) {
          for (let i = 0; i < 10; i++) {
            const px = x + side * (7 - Math.round(i * 0.4)), py = y + 13 - i;
            fill(px - (side > 0 ? 1 : 0), py, 2, 1, i > 6 ? 0xc8ccd6 : steel);
            if (i % 2 === 0 && i > 1) dot(px - side * 2, py, 0xd8dce4);
          }
        } else {
          for (let i = 0; i < 8; i++) {
            const px = x + side * (7 - i), py = y + 13 - Math.round(i * 0.7);
            fill(px - (side > 0 ? 1 : 0), py, 2, 1, steel);
            if (i % 2 === 1) dot(px, py - 1, 0xd8dce4);
          }
        }
      }
      fill(x - 2, y + 12 + sink, 5, 2, 0xa0603e); dot(x - 2, y + 12 + sink, 0xc88a60);
      dot(x + 7, y + 12, 0x6e727c); dot(x + 8, y + 11, 0x6e727c); dot(x + 8, y + 10, 0x6e727c);
    },
    react({ t, fade, x, y }) {
      glowAt(x, y + 10, 16, 10, 0xff3a3a, 0.1 * fade);
      const tops = [];
      for (let i = 0; i < 9; i++) {
        const px = x - 17 + i * 4, rest = y - 30 + i * 4.5 + (hash(i, 8) % 3);
        const drop = Math.min(1, Math.max(0, (t - 0.1 - i * 0.1) / 0.3));
        if (drop <= 0) continue;
        const py = rest - (1 - drop) * 16, h = 5 + (hash(i, 2) % 4);
        candle(px, py, h, 0xd8402a, 0xff8a7a, fade * (0.4 + 0.6 * drop));
        tops.push([px + 1, py + h]);
      }
      for (let i = 1; i < tops.length; i++) line(tops[i - 1][0], tops[i - 1][1], tops[i][0], tops[i][1], 0xff8a7a, 0.4 * fade);
      if (t > 1.1) buttonIcon('down', x + 17, y - 6 + Math.floor(t * 4) % 2, 0xff5a4a, fade);
      spray(t, { x, y: y + 14, n: 14, seed: 4, angle: [-Math.PI * 0.95, -Math.PI * 0.05], speed: [8, 26], g: 40, life: 0.9, colors: [0xb4b0a4, 0x8a8f9c], alpha: fade });
      onlookers(['down', 'drop'], [0xff5a4a, 0x9fd8ff], 0.8, 3.4);
    },
  },

  /** A basketball on a black pad, a buzzer. A ball arcs through a hoop with a swish, and confetti falls. */
  buzzer: {
    look(x, y, sink) {
      fill(x - 6, y + 11, 13, 5, 0x23262e); fill(x - 6, y + 11, 13, 1, 0x4a4e5c); fill(x - 6, y + 15, 13, 1, 0x14161c);
      disc(x, y + 6 + sink, 5, 0xe8782e); ring(x, y + 6 + sink, 5, 0xc4601c);
      fill(x, y + 1 + sink, 1, 11, 0x4a2410); fill(x - 5, y + 6 + sink, 11, 1, 0x4a2410);
      for (const dy of [3, 4, 5, 7, 8, 9]) { dot(x - 3 - (dy === 5 || dy === 7 ? 1 : 0), y + dy + sink, 0x4a2410); dot(x + 3 + (dy === 5 || dy === 7 ? 1 : 0), y + dy + sink, 0x4a2410); }
      dot(x - 2, y + 3 + sink, 0xffb070); dot(x - 3, y + 4 + sink, 0xffb070, 0.7);
    },
    react({ t, fade, x, y }) {
      const hx = x + 12, hy = y - 24;
      const alpha = Math.min(1, t / 0.2) * fade;
      // The hoop, the board behind it and the net
      fill(hx + 5, hy - 10, 2, 11, WHITE, alpha); fill(hx + 5, hy - 10, 2, 1, 0xd8402a, alpha); fill(hx - 3, hy, 9, 1, 0xd8402a, alpha);
      const swish = t > 1.1 && t < 1.7 ? 1 : 0;
      for (let r = 1; r <= 4; r++) {
        const w = 8 - r + (swish && r < 3 ? 1 : 0);
        const off = swish ? Math.round(Math.sin(t * 22) * (r < 3 ? 1 : 0)) : 0;
        for (let c = 0; c < w; c += 2) dot(hx - 3 + r * 0.5 + c + off, hy + r + (swish ? 1 : 0), WHITE, 0.8 * alpha);
      }
      // The ball: in from the left, over the rim, down through the net to the floor
      const fly = (t - 0.15) / 1;
      if (fly > 0 && fly < 1) {
        const bx = x - 20 + (hx + 1 - (x - 20)) * fly, by = y - 2 + (hy - 3 - (y - 2)) * fly - Math.sin(fly * Math.PI) * 16;
        disc(bx, by, 2, 0xe8782e, fade); dot(bx - 1, by - 1, 0xffb070, fade);
      } else if (fly >= 1 && t < 1.7) {
        const drop = (t - 1.15) / 0.55;
        disc(hx + 1, hy - 1 + drop * (y + 12 - hy), 2, 0xe8782e, fade);
      } else if (t >= 1.7) {
        const hop = Math.abs(Math.sin((t - 1.7) * 6)) * Math.max(0, 4 - (t - 1.7) * 3);
        disc(hx + 1 + (t - 1.7) * 3, y + 12 - hop, 2, 0xe8782e, fade);
      }
      if (t > 1.15 && t < 1.4) glowAt(x, y + 6, 12, 8, 0xff4020, 0.2);
      spray(t, { x: hx + 1, y: hy, n: 36, seed: 7, angle: [-Math.PI * 0.95, -Math.PI * 0.05], speed: [18, 50], g: 40, life: 2.2, delay: 1.15, drag: 1,
        colors: [0xe8782e, WHITE, 0x1b2a5c, 0xf2d35b], flip: true, floor: y + 15, alpha: fade });
      onlookers(['star', 'up'], [0xe8782e, 0xf2d35b], 1.2, 3.6);
    },
  },

  /** A navy air horn with a red bell and a white band. Navy and red flares flame up either side of it. */
  goalhorn: {
    look(x, y, sink) {
      fill(x - 6, y + 6, 6, 10, 0x1b2a5c); fill(x - 6, y + 6, 1, 10, 0x2f4590); fill(x - 1, y + 6, 1, 10, 0x101a3c);
      fill(x - 6, y + 9, 6, 1, WHITE, 0.9); fill(x - 6, y + 10, 6, 1, 0xd8402a); fill(x - 6, y + 15, 6, 1, 0x101a3c);
      fill(x - 5, y + 4 + sink, 4, 2, 0x2a2c30); fill(x - 4, y + 3 + sink, 2, 1, 0x4a4e5c);
      fill(x, y + 8, 2, 2, 0xd8402a); fill(x + 2, y + 7, 2, 4, 0xd8402a); fill(x + 4, y + 5, 2, 8, 0xe8503c); fill(x + 6, y + 3, 2, 12, 0xd8402a);
      fill(x + 7, y + 4, 1, 10, 0x6a1410); dot(x + 6, y + 3, 0xff9a8a); dot(x + 4, y + 5, 0xff9a8a); dot(x + 2, y + 7, 0xff9a8a);
    },
    react({ t, fade, x, y }) {
      const floor = y + 15;
      for (const [fx, spark, smoke, glow] of [[x - 13, [0xff3a2a, 0xffb090, WHITE], 0xd8402a, 0xff3a2a], [x + 13, [0x3a6aff, 0x90b0ff, WHITE], 0x2f4590, 0x4c6ad8]]) {
        fill(fx, floor - 7, 1, 7, 0x3a3d47); fill(fx - 1, floor - 8, 3, 2, spark[1]); dot(fx, floor - 9, WHITE);
        glowAt(fx, floor - 9, 8, 7, glow, 0.2 * fade);
        // Smoke rising, wider as it goes
        for (let k = 0; k < 7; k++) {
          const age = (t * 0.7 + k / 7) % 1, r = 2 + age * 3;
          disc(fx + Math.sin(k * 2 + t) * 2 * age + (fx < x ? -age * 3 : age * 3), floor - 12 - age * 30, r, smoke, 0.3 * (1 - age) * fade);
        }
        spray(t, { x: fx, y: floor - 10, n: 36, seed: fx, angle: [-Math.PI * 0.64, -Math.PI * 0.36], speed: [22, 48], g: 52, life: 0.9, stagger: 2.4,
          colors: spark, alpha: fade });
      }
      const wave = (t * 1.2) % 1;
      if (t < 2.4) ellipseRing(x + 7, y + 9, 4 + wave * 16, (4 + wave * 16) * 0.8, WHITE, (1 - wave) * 0.4 * fade);
      onlookers(['star', 'bolt'], [WHITE, 0xff5a4a], 0.3, 3.4);
    },
  },

  /** A console with a flip switch and a small readout. A red light pulses and a count runs, then it's all fine: a thumbs-up. */
  selfdestruct: {
    look(x, y, sink, s) {
      const done = s > 2 && s < Infinity;
      fill(x - 6, y + 8, 13, 8, 0x1b1f2e); fill(x - 5, y + 6, 11, 2, 0x2f3550); fill(x - 5, y + 6, 11, 1, 0x4a5478);
      fill(x - 6, y + 8, 1, 8, 0x2f3550); fill(x + 6, y + 8, 1, 8, 0x0f1220);
      fill(x - 4, y + 9, 9, 4, 0x0a0c14);
      const c = done ? 0x4ade80 : 0xd8402a;
      fill(x - 3, y + 11, 2, 1, c, 0.7); fill(x, y + 10, 2, 1, c, 0.7); fill(x + 3, y + 11, 1, 1, c, 0.7);
      dot(x - 5, y + 14, 0xe8a020); dot(x + 5, y + 14, done ? 0x4ade80 : 0xe8a020);
      // The switch under its cover, flipped when it's pressed
      fill(x, sink ? y + 4 : y + 1, 1, sink ? 2 : 5, 0xa8adb6);
      fill(x - 1, sink ? y + 3 : y, 3, 2, 0xd8402a); dot(x - 1, sink ? y + 3 : y, 0xff9a8a);
      fill(x - 3, y + 15, 7, 1, 0x0f1220);
      if (isNight) glowLate(() => glowAt(x, y + 11, 7, 4, c, 0.14));
    },
    react({ t, fade, x, y }) {
      const green = t >= 2;
      if (!green) {
        const pulse = 0.5 + 0.5 * Math.sin(t * 8);
        glowAt(x, y + 8, 18, 11, 0xff2a1a, 0.2 * pulse * fade);
        const ring = (t * 1.4) % 1;
        ellipseRing(x, y + 14, 4 + ring * 24, (4 + ring * 24) * 0.5, 0xff4a30, (1 - ring) * 0.7);
        const digit = String(3 - Math.min(2, Math.floor(t / 0.7)));
        pixelText(digit, x - 1, y - 12, 0xff5040);
        onlookers(['exclaim'], [0xff5a40], 0.2, 2);
      } else {
        const age = t - 2;
        glowAt(x, y + 8, 16, 10, 0x4ade80, 0.16 * fade);
        buttonIcon('thumb', x - 3, y - 14 - (age < 0.15 ? 1 : 0), 0x4ade80, fade);
        spray(age, { x, y: y - 8, n: 14, seed: 11, angle: [-Math.PI * 0.95, -Math.PI * 0.05], speed: [10, 26], g: 20, life: 1.5, colors: [0x4ade80, 0xbfffd0, WHITE], alpha: fade });
        onlookers(['thumb'], [0x4ade80], 2.1, 4);
      }
    },
  },

  /** A projector puck with a button of light floating over it. The picture glitches in waves. */
  hologram: {
    look(x, y, sink, s) {
      const glitch = s < 1.4;
      fill(x - 5, y + 13, 11, 3, 0x20242e); fill(x - 5, y + 13, 11, 1, 0x3a4050); fill(x - 4, y + 14, 9, 1, 0x4adcf0);
      fill(x - 2, y + 12, 5, 1, 0x4adcf0, 0.3); fill(x - 3, y + 10, 7, 2, 0x4adcf0, 0.1);
      for (const [dx, c] of glitch ? [[-1, 0xff4ad8], [1, 0x4adcf0]] : [[0, 0x4adcf0]]) {
        fill(x - 4 + dx, y + 3 + sink, 9, 5, c, 0.42);
        fill(x - 4 + dx, y + 3 + sink, 9, 1, 0xbff8ff, 0.85);
        for (const row of [5, 7]) fill(x - 4 + dx, y + row + sink, 9, 1, 0x0a1a24, 0.28);
      }
      ring(x, y + 5 + sink, 2, WHITE, 0.55); dot(x - 3, y + 4 + sink, WHITE, 0.8);
      if (isNight) glowLate(() => glowAt(x, y + 7, 9, 8, 0x4adcf0, glitch ? 0.22 : 0.13));
      else glowAt(x, y + 7, 8, 6, 0x4adcf0, 0.05);
    },
    react({ t, fade, x, y }) {
      const left = x - 17, width = 35, top = y - 22, bottom = y + 16;
      for (const start of [0, 1.5]) {
        const band = bottom - (t - start) * 46;
        if (t < start || band < top - 8) continue;
        for (let row = Math.max(top, band - 4); row < Math.min(bottom, band + 4); row++) {
          const off = (hash(row, Math.floor(t * 9)) % 9) - 4, w = 10 + (hash(row, 5) % 18);
          fill(left + 6 + off, row, w, 1, 0x4adcf0, 0.38 * fade);
          fill(left + 8 + off, row, w - 3, 1, 0xff4ad8, 0.26 * fade);
        }
      }
      const step = Math.floor(t * 8);
      for (let k = 0; k < 12; k++) {
        if (t > 2.8 && k > 5) continue;
        const c = [0x4adcf0, 0xff4ad8, WHITE][hash(k, step) % 3];
        fill(left + (hash(step, k) % width), top + (hash(k, step + 2) % (bottom - top)), 2 + (k % 3), 1, c, 0.7 * fade);
      }
      onlookers(['question'], [0x4adcf0, 0xff4ad8], 0.4, 3);
    },
  },

  /** A console with a slot and a lever to push right through. Stars streak out from the middle of the room. */
  warp: {
    look(x, y, sink, s) {
      const lit = s < Infinity;
      fill(x - 6, y + 3, 13, 13, 0x6e727c); fill(x - 6, y + 3, 13, 1, 0xa8adb6); fill(x - 6, y + 3, 1, 13, 0xa8adb6);
      fill(x + 6, y + 3, 1, 13, 0x4a4e58); fill(x - 6, y + 15, 13, 1, 0x4a4e58);
      fill(x - 1, y + 5, 3, 9, 0x14161c);
      const hy = sink ? y + 10 : y + 5;
      fill(x - 3, hy, 7, 2, 0xe8782e); fill(x - 3, hy, 7, 1, 0xffa860); fill(x, hy + 2, 1, 1, 0xa8adb6, sink ? 0 : 1);
      for (const [dx, dy] of [[-5, 5], [-5, 8], [-5, 11], [5, 5], [5, 8], [5, 11]]) dot(x + dx, y + dy, lit ? 0x8fd8ff : 0x4c8bf5, lit ? 1 : 0.7);
      if (isNight) glowLate(() => glowAt(x, y + 8, 7, 7, 0x4c8bf5, lit ? 0.22 : 0.1));
    },
    react({ t, fade, x, y }) {
      const ox2 = x, oy2 = y - 18;
      if (t < 0.18) glowAt(ox2, oy2, 12, 10, WHITE, 0.4 * (1 - t / 0.18));
      glowAt(ox2, oy2, 24, 16, 0x8fd8ff, 0.1 * fade);
      for (let k = 0; k < 32; k++) {
        const a = (k / 32) * Math.PI * 2 + (hash(k, 3) % 100) / 400;
        const age = (t - (hash(k, 9) % 60) / 100) / 1.6;
        if (age < 0 || age > 1) continue;
        const head = 3 + 62 * age * age, tail = Math.max(2, head - 4 - 26 * age);
        const c = k % 3 === 0 ? 0x8fd8ff : WHITE;
        const dx = Math.cos(a), dy = Math.sin(a) * 0.78;
        if (head > 36) continue;
        line(ox2 + dx * tail, oy2 + dy * tail, ox2 + dx * head, oy2 + dy * head, c, (1 - age * 0.6) * fade);
      }
      disc(ox2, oy2, 1 + (Math.floor(t * 5) % 2), WHITE, 0.9 * fade);
      onlookers(['star', 'bolt'], [0x8fd8ff, WHITE], 0.3, 3.2);
    },
  },
};
