/**
 * The back wall's pieces besides a window, a whiteboard, a pinboard, a
 * poster, a shelf and a plant: each around its middle `x`, between the
 * wall's top and its rail at 30 unless it reaches the floor.
 */
const WALL_PIECES = {
  /** Viewport. Stars drift past a ringed planet. Space whatever the hour. */
  viewport(x) {
    const l = x - 16, t = 6, w = 32, h = 20;
    fill(l - 2, t - 2, w + 4, h + 4, 0x5a6270); fill(l - 2, t - 2, w + 4, 1, 0x8a93a3); fill(l - 2, t + h + 1, w + 4, 1, 0x3a3f4a);
    fill(l, t, w, h, 0x060a1a);
    for (let k = 0; k < 12; k++) {
      const sx = l + ((((k * 11 + 200 - now * ((k % 3) + 1) * 1.5) % w) + w) % w), sy = t + 1 + ((k * 7) % (h - 2));
      dot(sx, sy, WHITE, k % 3 === 0 ? 0.9 : 0.45);
    }
    const cx = l + 22, cy = t + 10;
    disc(cx, cy, 4.5, 0xb86a3a); disc(cx - 1, cy - 1, 3.2, 0xe0a060);
    fill(cx - 8, cy, 17, 1, PALETTE.brass, 0.9); dot(cx - 9, cy - 1, PALETTE.brass, 0.9); dot(cx + 9, cy + 1, PALETTE.brass, 0.9);
    for (const [dx, dy] of [[0, 0], [w - 1, 0], [0, h - 1], [w - 1, h - 1]]) dot(l + dx, t + dy, 0x5a6270);
    for (const bx of [l + 4, l + w - 5]) { dot(bx, t - 1, 0xb8c0cc); dot(bx, t + h, 0xb8c0cc); }
  },
  /** Porthole onto Earth. The Earth turns slowly below, clouds over it. */
  porthole(x) {
    const cx = x, cy = 16;
    disc(cx, cy, 10.5, 0x3a3f4a); disc(cx, cy, 9.5, 0x8a93a3); disc(cx, cy, 7.8, 0x060a1a);
    for (const [sx, sy] of [[-4, -5], [3, -6], [-6, -1], [5, -3]]) dot(cx + sx, cy + sy, WHITE, 0.6);
    const px = cx + 1, py = cy + 2, pr = 4.5;
    disc(px, py, pr, 0x3f7fc0);
    for (let k = 0; k < 6; k++) {
      const lx = px - 5 + ((((k * 3.3 + now * 0.8) % 11) + 11) % 11), ly = py - 3 + ((k * 5) % 7);
      if (Math.hypot(lx - px, ly - py) < pr - 0.5) { dot(Math.round(lx), ly, PALETTE.leaf); if (k % 2) dot(Math.round(lx) + 1, ly, PALETTE.leaf); }
    }
    const cl = px - 4 + ((((now * 1.3) % 9) + 9) % 9);
    if (Math.hypot(cl - px, -1) < pr - 0.5) fill(Math.round(cl), py - 1, 2, 1, WHITE, 0.75);
    for (let k = 0; k < 8; k++) { const a = (k * Math.PI) / 4; dot(cx + Math.round(Math.cos(a) * 9), cy + Math.round(Math.sin(a) * 9), 0xc8cdd6); }
    for (const [dx, dy] of [[-5, -4], [-4, -5], [-3, -6]]) dot(cx + dx, cy + dy, WHITE, 0.3);
  },
  /** Control panel. A wave runs across the scope, buttons blink in turn, one slider drifts. */
  panel(x) {
    const l = x - 13, t = 8;
    fill(l, t, 26, 19, 0x2a2e36); fill(l + 1, t + 1, 24, 17, 0x3a3f4a); fill(l + 1, t + 1, 24, 1, 0x5a6270);
    fill(l + 3, t + 3, 11, 7, 0x0e2a1a);
    for (let k = 0; k < 11; k++) dot(l + 3 + k, t + 6 + Math.round(Math.sin(k * 0.9 - now * 4) * 2), 0x6fd36f);
    const colours = [0xd96a5a, 0xf2d35b, 0x6fd36f, 0x4ad4e8];
    for (let i = 0; i < 9; i++) { const on = (Math.floor(now * 2.5) + i * 4) % 7 < 3; fill(l + 16 + (i % 3) * 3, t + 3 + Math.floor(i / 3) * 3, 2, 2, colours[i % 4], on ? 1 : 0.3); }
    fill(l + 3, t + 12, 20, 5, 0x2a2e36);
    [1, 3, Math.round(2 + 2 * Math.sin(now * 0.7)), 0, 4, 2].forEach((pos, k) => { fill(l + 4 + k * 3, t + 12, 1, 5, 0x1d1f25); fill(l + 3 + k * 3, t + 12 + pos, 3, 1, 0xc8cdd6); });
  },
  /** Pipes and gauge. The gauge needle twitches; steam puffs from a joint now and then. */
  pipes(x) {
    const steel = 0x6a7280, hi = 0x8a93a3, lo = 0x4a515e;
    fill(x - 12, 2, 3, 28, steel); fill(x - 12, 2, 1, 28, hi); fill(x - 10, 2, 1, 28, lo);
    fill(x - 9, 10, 20, 3, steel); fill(x - 9, 10, 20, 1, hi); fill(x - 9, 12, 20, 1, lo);
    fill(x + 9, 10, 3, 20, steel); fill(x + 9, 10, 1, 20, hi); fill(x + 11, 10, 1, 20, lo);
    fill(x - 13, 9, 5, 5, lo); fill(x + 8, 9, 5, 5, lo);
    ring(x - 2, 11, 2.5, 0xc9443a); dot(x - 2, 11, 0x9a2a2a);
    fill(x + 1, 13, 1, 4, steel);
    disc(x + 1, 21, 4.5, 0x3a3f4a); disc(x + 1, 21, 3.5, 0xf4f1ea);
    const a = -0.7 + 0.12 * Math.sin(now * 9) + 0.1 * Math.sin(now * 2.3);
    for (let r = 1; r <= 3; r++) dot(x + 1 + Math.round(Math.sin(a) * r), 21 - Math.round(Math.cos(a) * r), 0xd4140b);
    const t = ((now % 5) + 5) % 5;
    if (t < 1.2) { const rise = t / 1.2; fill(x + 13 + Math.round(rise * 3), 8 - Math.round(rise * 6), 2 + Math.round(rise * 2), 2, WHITE, 0.5 * (1 - rise)); }
  },
  /** Porthole onto the sea. Waves roll by, the sky keeps the time, and a ship crosses the horizon now and then. */
  seaPorthole(x) {
    const cx = x, cy = 16;
    disc(cx, cy, 10.5, 0x7a5a2a); disc(cx, cy, 9.5, PALETTE.brass); disc(cx, cy, 7.8, 0x9a7a3a);
    const [top, bottom] = sky();
    for (let row = -7; row <= 7; row++) { const half = Math.floor(Math.sqrt(49 - row * row)); fill(cx - half, cy + row, half * 2 + 1, 1, row <= 0 ? (row < -3 ? top : bottom) : 0x2f6a9a); }
    fill(cx - 6, cy + 1, 13, 1, 0x3f7fc0);
    for (let row = 2; row <= 6; row += 2) { const half = Math.floor(Math.sqrt(49 - row * row)); for (let k = -half; k <= half; k++) if ((((k + row * 3 + Math.floor(now * 3)) % 6) + 6) % 6 === 0) dot(cx + k, cy + row, WHITE, 0.55); }
    const lap = (now / 24) % 1, sx = cx - 8 + Math.round(lap * 16);
    if (sx > cx - 6 && sx < cx + 4) { fill(sx, cy, 3, 1, 0x2a1d16); fill(sx + 1, cy - 2, 1, 2, 0xeae6dc); }
    if (isNight) { fill(cx + 2, cy - 4, 2, 2, 0xf1ead2); dot(cx + 2, cy + 3, 0xf1ead2, 0.5); }
    for (let k = 0; k < 8; k++) { const a = (k * Math.PI) / 4 + Math.PI / 8; dot(cx + Math.round(Math.cos(a) * 9), cy + Math.round(Math.sin(a) * 9), 0x7a5a2a); }
    for (const [dx, dy] of [[-4, -4], [-3, -5]]) dot(cx + dx, cy + dy, WHITE, 0.4);
  },
  /** Treasure map. A dotted trail to the red X. */
  map(x) {
    const l = x - 13, t = 8, w = 26, h = 16;
    fill(l, t, w, h, 0xe0c890);
    fill(l - 1, t - 1, w + 2, 2, 0xc4a86a); fill(l - 1, t - 1, w + 2, 1, 0xd8bc80); fill(l - 1, t + h - 1, w + 2, 2, 0xb8985a);
    dot(l + w - 1, t + 2, 0xb8985a); dot(l, t + h - 3, 0xb8985a);
    fill(l + 6, t + 5, 10, 6, 0x9ab06a); fill(l + 8, t + 4, 6, 1, 0x9ab06a); fill(l + 7, t + 11, 8, 1, 0x9ab06a); fill(l + 5, t + 7, 1, 3, 0x9ab06a);
    fill(l + 9, t + 6, 3, 2, 0x6a8a4a);
    for (const [dx, dy] of [[2, 3], [3, 12], [20, 4], [21, 12], [18, 9]]) { dot(l + dx, t + dy, 0x6a8ab0); dot(l + dx + 1, t + dy - 1, 0x6a8ab0); }
    for (const [dx, dy] of [[2, 13], [4, 12], [6, 11], [8, 10], [10, 9], [12, 9]]) dot(l + dx, t + dy, 0xb8322a);
    for (const [dx, dy] of [[13, 6], [15, 6], [14, 7], [13, 8], [15, 8]]) dot(l + dx, t + dy, 0xd4140b);
    fill(l + 22, t + 2, 1, 5, 0x6e5233); fill(l + 20, t + 4, 5, 1, 0x6e5233); dot(l + 22, t + 2, 0xb8322a);
    dot(l + 1, t, 0x3a3a40); dot(l + w - 2, t, 0x3a3a40);
  },
  /** Jolly Roger. Pinned at its corners, folds in the cloth. */
  flag(x) {
    const l = x - 12, t = 8;
    fill(l, t, 24, 15, 0x1a1a1f); fill(l + 2, t + 15, 20, 1, 0x1a1a1f);
    fill(l + 7, t, 1, 15, 0x26262c); fill(l + 17, t, 1, 15, 0x26262c);
    fill(l + 10, t + 2, 5, 4, 0xeae6dc); fill(l + 11, t + 6, 3, 1, 0xeae6dc); dot(l + 11, t + 4, 0x1a1a1f); dot(l + 13, t + 4, 0x1a1a1f);
    for (let k = 0; k <= 10; k++) { dot(l + 7 + k, t + 8 + Math.round(k * 0.5), 0xeae6dc); dot(l + 17 - k, t + 8 + Math.round(k * 0.5), 0xeae6dc); }
    for (const [dx, dy] of [[6, 7], [6, 9], [18, 7], [18, 9], [6, 12], [6, 14], [18, 12], [18, 14]]) dot(l + dx, t + dy, 0xeae6dc);
    for (const [dx, dy] of [[0, 0], [23, 0], [1, 15], [22, 15]]) dot(l + dx, t + dy, PALETTE.brass);
  },
  /** Crossed cutlasses. On a wooden shield. */
  cutlasses(x) {
    fill(x - 6, 8, 13, 13, 0x6b4a2e); fill(x - 5, 21, 11, 2, 0x6b4a2e); fill(x - 3, 23, 7, 1, 0x6b4a2e); fill(x - 6, 8, 13, 1, 0x8c6744); fill(x + 6, 9, 1, 12, 0x4a301e);
    for (const dir of [1, -1]) {
      for (let k = 0; k <= 15; k++) { const px = x - dir * 8 + dir * k + dir * Math.round(Math.sin((k / 15) * Math.PI) * 1.5), py = 6 + k; dot(px, py, 0xc8cdd6); dot(px + dir, py, 0x9aa0a6); }
      const hx = x + dir * 8, hy = 22;
      fill(hx - 1, hy, 3, 1, PALETTE.brass); dot(hx + dir * 2, hy - 1, PALETTE.brass); fill(hx + dir, hy + 1, 1, 3, 0x3a2616); dot(hx + dir, hy + 4, PALETTE.brass);
    }
  },
  /** Royal banner. Hangs from a gilt rod and stirs slightly at its point. */
  tapestry(x) {
    fill(x - 11, 6, 23, 1, PALETTE.brass); fill(x - 12, 5, 2, 3, PALETTE.brass); fill(x + 11, 5, 2, 3, PALETTE.brass);
    const sway = Math.round(Math.sin(now * 0.8) * 0.7);
    for (let row = 7; row <= 27; row++) {
      const taper = row > 21 ? row - 21 : 0, s = row > 17 ? sway : 0, l = x - 9 + taper + s, w = 19 - taper * 2;
      if (w <= 0) continue;
      fill(l, row, w, 1, 0x9a2a3a); dot(l, row, PALETTE.brass); dot(l + w - 1, row, PALETTE.brass);
    }
    fill(x - 3, 11, 7, 3, PALETTE.brass); for (const dx of [-3, 0, 3]) dot(x + dx, 10, PALETTE.brass); dot(x, 12, 0xd4140b);
    fill(x - 3, 16, 7, 5, PALETTE.brass); fill(x - 2, 21, 5, 1, PALETTE.brass); dot(x, 22, PALETTE.brass); fill(x - 2, 17, 5, 3, 0x3f6fc4);
    dot(x + sway, 28, PALETTE.brass); dot(x + sway, 29, 0x9a7a3a);
  },
  /** Royal portrait. A king in ermine, lit by a picture light. */
  portrait(x) {
    fill(x - 8, 4, 17, 8, 0xffe08a, 0.07); fill(x - 4, 3, 9, 1, PALETTE.brass);
    fill(x - 9, 5, 19, 23, PALETTE.brass); fill(x + 9, 5, 1, 23, 0x9a7a3a); fill(x - 9, 27, 19, 1, 0x9a7a3a); fill(x - 9, 5, 19, 1, 0xf2d38a);
    fill(x - 7, 7, 15, 19, 0x2f4a3a);
    fill(x - 6, 20, 13, 6, 0x9a2a3a); fill(x - 5, 19, 11, 2, 0xeae6dc); dot(x - 3, 20, 0x1a1a1f); dot(x + 1, 19, 0x1a1a1f); dot(x + 4, 20, 0x1a1a1f);
    fill(x - 2, 13, 5, 6, 0xe8b48a); fill(x - 2, 17, 5, 2, 0x8a5a2b); dot(x - 1, 15, PALETTE.eye); dot(x + 1, 15, PALETTE.eye);
    fill(x - 2, 11, 5, 2, PALETTE.brass); for (const dx of [-2, 0, 2]) dot(x + dx, 10, PALETTE.brass); dot(x, 12, 0xd4140b);
  },
  /** Stained glass. Bright in daylight, dim at night. */
  stained(x) {
    const t = 5, h = 22, r = 9;
    const halfAt = (row) => (row < t + r ? Math.round(Math.sqrt(Math.max(0, r * r - (t + r - row) ** 2))) : r);
    for (let row = t - 1; row <= t + h + 1; row++) { const half = halfAt(Math.max(t, row)) + 2; fill(x - half, row, half * 2 + 1, 1, 0x6a6d72); }
    const panes = [0x9a2a3a, 0x3f6fc4, PALETTE.brass, 0x3f8a5a, 0x7d5ab8];
    const lit = isNight ? 0.45 : daylight >= 17 ? 0.8 : 1;
    for (let row = t; row <= t + h; row++) {
      const half = halfAt(row);
      for (let dx = -half; dx <= half; dx++) {
        const lead = ((dx + r) % 4 === 0) || ((row - t) % 5 === 0) || dx === -half || dx === half;
        dot(x + dx, row, lead ? PALETTE.frame : shaded(panes[(Math.floor((dx + r) / 4) + Math.floor((row - t) / 5)) % 5], lit));
      }
    }
    disc(x, t + 7, 2.5, shaded(PALETTE.brass, lit)); dot(x, t + 7, shaded(0xd4140b, lit));
    fill(x - 12, t + h + 2, 25, 1, 0x8a8d92);
  },
  /** Shield and swords. A heater shield in blue and gold over crossed swords. */
  shield(x) {
    for (const dir of [1, -1]) { line(x - dir * 10, 6, x + dir * 9, 27, 0xc8cdd6); fill(x - dir * 10 - 1, 5, 3, 1, PALETTE.brass); dot(x - dir * 11, 4, 0x3a2616); }
    for (let row = 9; row <= 26; row++) { const half = row < 20 ? 7 : 7 - Math.round((row - 20) * 1.15); if (half < 0) continue; fill(x - half, row, half * 2 + 1, 1, 0x3f6fc4); dot(x - half, row, PALETTE.brass); dot(x + half, row, PALETTE.brass); }
    fill(x - 7, 9, 15, 1, PALETTE.brass);
    for (let k = 0; k <= 6; k++) { dot(x - k, 13 + k, PALETTE.brass); dot(x + k, 13 + k, PALETTE.brass); dot(x - k, 14 + k, PALETTE.brass); dot(x + k, 14 + k, PALETTE.brass); }
  },
  /** Frosted window. Snowy hills by day, the aurora at night, snow always falling. */
  frost(x) {
    const l = x - 16, t = 6, w = 32, h = 20;
    fill(l - 1, t - 1, w + 2, h + 2, 0xdcebf4);
    skyPane(l, t, w, h);
    if (isNight) {
      for (let k = 0; k < w; k++) {
        fill(l + k, t + 4 + Math.round(Math.sin(k * 0.3 + now * 0.7) * 2), 1, 4, 0x6fd3a0, 0.3 + 0.2 * Math.sin(k * 0.2 + now));
        fill(l + k, t + 7 + Math.round(Math.sin(k * 0.25 + now * 0.5 + 1) * 2), 1, 2, 0x9a6ad0, 0.2);
      }
    }
    for (let k = 0; k < w; k++) { const hh = 4 + Math.round(2 * Math.sin(k * 0.22) + 1.5 * Math.sin(k * 0.5 + 1)); fill(l + k, t + h - hh, 1, hh, isNight ? 0x8a9ab8 : 0xf2f6fa); if (Math.cos(k * 0.22) < -0.2) dot(l + k, t + h - hh, isNight ? 0x6a7a98 : 0xc8d8e6); }
    for (let k = 0; k < 6; k++) { const px = l + ((k * 11 + l * 3) % w) + Math.round(Math.sin(now * 1.5 + k)), py = t + ((now * 5 + k * 7.3) % h); if (px >= l && px < l + w) dot(px, py, WHITE); }
    fill(x, t, 1, h, 0xdcebf4); fill(l, t + 10, w, 1, 0xdcebf4);
    for (let i = 0; i < 4; i++) { fill(l, t + i, 4 - i, 1, WHITE, 0.7); fill(l + w - 4 + i, t + i, 4 - i, 1, WHITE, 0.7); fill(l, t + h - 1 - i, 4 - i, 1, WHITE, 0.7); fill(l + w - 4 + i, t + h - 1 - i, 4 - i, 1, WHITE, 0.7); }
    fill(l - 2, t + h + 1, w + 4, 2, 0xf2f4f8); fill(l + 3, t + h, 6, 1, 0xf2f4f8); fill(l + 20, t + h, 8, 1, 0xf2f4f8);
    for (const [dx, len] of [[2, 2], [7, 1], [13, 3], [24, 2], [29, 1]]) fill(l + dx, t + h + 3, 1, len, 0xcfe3ef);
  },
  /** Skis and poles. A pair of skis on pegs, the poles crossed in front. */
  skis(x) {
    fill(x - 9, 13, 18, 1, 0x8c6744);
    for (const sx of [x - 5, x + 2]) { fill(sx, 7, 3, 21, 0xc9443a); fill(sx, 6, 3, 1, 0xd96a5a); dot(sx + 1, 5, 0xd96a5a); fill(sx + 1, 9, 1, 16, 0xeae6dc); fill(sx, 17, 3, 2, 0x1d1f25); }
    line(x - 9, 7, x + 8, 27, 0x9aa0a6); line(x + 8, 7, x - 9, 27, 0x9aa0a6);
    fill(x - 10, 6, 2, 3, 0x1d1f25); fill(x + 8, 6, 2, 3, 0x1d1f25);
    fill(x + 6, 25, 4, 1, 0x3a3d47); fill(x - 10, 25, 4, 1, 0x3a3d47);
  },
  /** Weather station. Thermometer, a wind dial that wanders, and the reading. */
  weather(x) {
    fill(x - 11, 7, 22, 21, 0x6e4a2e); fill(x - 10, 8, 20, 19, 0x8c6744);
    fill(x - 8, 10, 3, 12, 0xf4f1ea); fill(x - 7, 18, 1, 4, 0xd4140b); disc(x - 7, 23, 1.5, 0xd4140b);
    for (let k = 0; k < 5; k++) dot(x - 4, 11 + k * 2, 0x3a2616);
    disc(x + 4, 13, 4.5, PALETTE.brass); disc(x + 4, 13, 3.5, 0xf4f1ea);
    const a = 0.6 + 0.5 * Math.sin(now * 0.4) + 0.1 * Math.sin(now * 3);
    for (let r = 1; r <= 3; r++) dot(x + 4 + Math.round(Math.sin(a) * r), 13 - Math.round(Math.cos(a) * r), 0x1d1f25);
    fill(x - 2, 19, 12, 7, 0x1d1f25); pixelText('-27', x - 1, 20, 0x4ad4e8, false);
  },
  /** Wall torch. Flames flicker, light wavers on the wall, a wisp of smoke. */
  torch(x) {
    const flick = 0.5 + 0.5 * Math.sin(now * 13) * Math.sin(now * 7.3);
    fill(x - 9, 3, 19, 24, 0xff8a3d, 0.05 + 0.03 * flick); fill(x - 6, 5, 13, 16, 0xff8a3d, 0.06 + 0.04 * flick);
    fill(x - 3, 21, 7, 1, 0x3a3a40); fill(x - 2, 22, 5, 2, 0x2a2a2e); fill(x - 1, 24, 3, 4, 0x2a2a2e); dot(x, 28, 0x3a3a40);
    fill(x - 1, 14, 3, 8, 0x6e4a2e); fill(x - 1, 14, 1, 8, 0x8a6a42); fill(x - 2, 13, 5, 2, 0x5a3b25);
    for (let k = 0; k < 5; k++) {
      const h = 3 + Math.round(3 * Math.abs(Math.sin(now * (6 + k) + k * 1.9))) - Math.abs(k - 2);
      if (h <= 0) continue;
      const fx = x - 2 + k;
      fill(fx, 13 - h, 1, h, 0xd4140b); if (h > 1) fill(fx, 14 - h, 1, h - 1, 0xff8a3d); if (h > 3) fill(fx, 15 - h, 1, h - 2, 0xf2d35b);
    }
    const rise = (now * 0.5) % 1; dot(x + Math.round(Math.sin(rise * 6) * 1.5), 6 - Math.round(rise * 5), PALETTE.smoke, 0.4 * (1 - rise));
  },
  /** Arched stone window. A red moon at night with bats crossing it; clouds by day. */
  arch(x) {
    const t = 6, h = 20, r = 6;
    const halfAt = (row) => (row < t + r ? Math.round(Math.sqrt(Math.max(0, r * r - (t + r - row) ** 2))) : r);
    for (let row = t - 2; row <= t + h + 1; row++) { const half = halfAt(Math.max(t, row)) + 3; fill(x - half, row, half * 2 + 1, 1, 0x5a5d62); }
    for (let row = t - 2; row <= t + h + 1; row += 3) { dot(x - halfAt(Math.max(t, row)) - 3, row, 0x44464b); dot(x + halfAt(Math.max(t, row)) + 3, row, 0x44464b); }
    const [top, bottom] = sky();
    for (let row = t; row <= t + h; row++) { const half = halfAt(row); fill(x - half, row, half * 2 + 1, 1, row < t + h / 2 ? top : bottom); }
    if (isNight) {
      disc(x + 1, t + 6, 2.5, 0xd9806a);
      const lap = (now / 9) % 1, bx = x - 8 + Math.round(lap * 16), by = t + 9 + Math.round(Math.sin(lap * 12));
      if (bx > x - 6 && bx < x + 5) { dot(bx, by, PALETTE.frame); dot(bx - 1, by - 1 + (Math.floor(now * 8) % 2), PALETTE.frame); dot(bx + 1, by - 1 + (Math.floor(now * 8) % 2), PALETTE.frame); }
    } else { const cx = x - 5 + Math.round(((now * 0.6) % 14)); if (cx < x + 4) fill(cx, t + 7, 4, 1, WHITE, 0.8); }
    fill(x - 2, t + 1, 1, h, 0x2a2a2e); fill(x + 2, t + 1, 1, h, 0x2a2a2e);
    fill(x - 9, t + h + 2, 19, 1, 0x7a7d82);
  },
  /** Neon ghost. A pink arcade ghost and GG in cyan; it stutters off now and then. */
  neon(x) {
    const off = (now % 7) > 6.5 && Math.floor(now * 20) % 2 === 0;
    const ghost = ['...XXX...', '.XX...XX.', 'X..X.X..X', 'X.......X', 'X.......X', 'X.......X', 'X.X...X.X', 'XX.X.X.XX'];
    const tube = (px, py, colour) => { if (!off) fill(px - 1, py - 1, 3, 3, colour, 0.1); dot(px, py, off ? shaded(colour, 0.35) : lighter(colour)); };
    ghost.forEach((row, r) => [...row].forEach((c, k) => { if (c === 'X') tube(x - 4 + k, 7 + r, 0xff5aa8); }));
    for (const [gx] of [[x - 4], [x + 1]]) GLYPHS.G.forEach((row, r) => [...row].forEach((c, k) => { if (c === '1') tube(gx + k, 18 + r, 0x4ad4e8); }));
    dot(x - 6, 6, 0x3a3d47); dot(x + 6, 6, 0x3a3d47);
  },
  /** Rainy window. A night city glows magenta and cyan beyond the rain; every so often a drop runs down the glass. */
  rainwindow(x) {
    const l = x - 16, t = 6, w = 32, h = 20, frame = 0x2a2d38;
    fill(l - 2, t - 2, w + 4, h + 4, frame); fill(l - 2, t - 2, w + 4, 1, 0x4a4e5c); fill(l - 2, t + h + 1, w + 4, 1, 0x1a1c24);
    fill(l, t, w, h, isNight ? 0x140c26 : 0x2a2448); fill(l, t + 9, w, 11, isNight ? 0x2a1440 : 0x3e2e5e);
    fill(l, t + h - 7, w, 7, 0xff5aa8, 0.12);
    // The towers, in silhouette, their windows lit; a cyan sign up one
    for (const [dx, bh, bw] of [[0, 9, 5], [5, 14, 4], [9, 7, 6], [15, 12, 3], [18, 16, 5], [23, 8, 4], [27, 12, 5]]) {
      fill(l + dx, t + h - bh, bw, bh, 0x0e0a1a);
      for (let wy = t + h - bh + 2; wy < t + h - 1; wy += 2) {
        for (let wx = l + dx + 1; wx < l + dx + bw - 1; wx += 2) if ((wx * 7 + wy * 3) % 5 < 2) dot(wx, wy, (wx + wy) % 3 ? 0xff5aa8 : 0x4ad4e8, 0.7);
      }
    }
    fill(l + 19, t + 6, 1, 5, 0x7ae8f8); fill(l + 18, t + 5, 3, 7, 0x4ad4e8, 0.18);
    // The rain, streaking down
    for (let k = 0; k < 11; k++) {
      const rx = l + 1 + ((k * 13) % (w - 2)), ry = t + Math.floor(((k * 7 + now * 26) % h + h) % h);
      fill(rx, ry, 1, Math.min(3, t + h - ry), 0xb8d0f0, 0.5);
    }
    // A drop running down the glass, now and then, somewhere new each time
    const d = (now + x * 0.17) % 9;
    if (d < 2.5) {
      const dx = l + 4 + ((Math.floor((now + x * 0.17) / 9) * 11) % 24), dy = t + 1 + Math.round((d / 2.5) * (h - 3));
      fill(dx, t + 1, 1, dy - t - 1, 0xd8e8ff, 0.2); fill(dx, dy, 2, 2, 0xd8e8ff, 0.75); dot(dx, dy, WHITE, 0.9);
    }
    fill(x, t, 1, h, frame); fill(l, t + 10, w, 1, frame, 0.6);
    fill(l - 3, t + h + 2, w + 6, 2, 0x3a3d47); fill(l - 3, t + h + 2, w + 6, 1, 0x5a5f6c);
    if (isNight) glowAt(x, t + h + 3, 18, 2, 0xff5aa8, 0.05);
  },
  /** Neon sign. A bowl of noodles in cyan, chopsticks in it, and a sitting cat in magenta, no words; once in a while the cat flickers. */
  neonsign(x) {
    const CYAN = 0x4ad4e8, PINK = 0xff5aa8, t = (now + x * 0.13) % 13, flick = t < 0.7 && Math.floor(now * 16) % 2 === 0;
    const bowl = ['.........M.M', '.C..C...M.M.', '..C..C.M.M..', '.C..C.M.M...', 'CCCCCCCCCCC.', 'C.........C.', '.C.......C..',
      '..CC...CC...', '....CCC.....', '...CCCCC....'];
    const cat = ['X.....X', 'XX...XX', 'X.....X', 'X.C.C.X', 'X.....X', '.X...X.', 'X.....X..', 'X.....X.X', 'X.....X..X', 'X.....X.X', 'XXXXXXXX.'];
    glowAt(x, 14, 15, 7, PINK, (isNight ? 0.06 : 0.03) * (flick ? 0.4 : 1));
    const tube = (px, py, colour, off) => { if (!off) fill(px - 1, py - 1, 3, 3, colour, isNight ? 0.13 : 0.08); dot(px, py, off ? shaded(colour, 0.35) : lighter(colour)); };
    bowl.forEach((row, r) => [...row].forEach((c, k) => { if (c !== '.') tube(x - 13 + k, 9 + r, c === 'C' ? CYAN : PINK, false); }));
    cat.forEach((row, r) => [...row].forEach((c, k) => { if (c !== '.') tube(x + 3 + k, 8 + r, c === 'C' ? CYAN : PINK, flick); }));
    for (const [dx, dy] of [[-14, 6], [13, 6], [-14, 21], [13, 21]]) dot(x + dx, dy, 0x3a3d47);
  },
  /** Porthole onto the deep. Riveted steel round dark water; bubbles rise, sea grass sways, and now and then a fish or a jellyfish drifts by. */
  fishporthole(x) {
    const cx = x, cy = 16, R = 8;
    disc(cx, cy, 11.5, 0x2a2e36); disc(cx, cy, 10.5, 0x6a7280); disc(cx, cy, 9, 0x4a515e);
    for (let row = -R; row <= R; row++) {
      const half = Math.floor(Math.sqrt(R * R - row * row));
      fill(cx - half, cy + row, half * 2 + 1, 1, row < -3 ? 0x1f5a8a : row < 3 ? 0x17436e : 0x0e2c4e);
    }
    const wet = (px, py, colour, alpha = 1) => { if ((px - cx) ** 2 + (py - cy) ** 2 < (R - 0.3) ** 2) dot(px, py, colour, alpha); };
    for (let k = 0; k < 7; k++) { wet(cx - 5 + k, cy - 7 + k * 2, 0x6ab0e0, 0.22); wet(cx - 1 + k, cy - 8 + k * 2, 0x6ab0e0, 0.15); }
    for (const [gx, gh] of [[-4, 3], [-3, 4], [3, 3], [5, 2]]) {
      for (let k = 0; k < gh; k++) wet(cx + gx + Math.round(Math.sin(now * 0.8 + gx + k * 0.6) * (k / 3)), cy + 7 - k, 0x2f7a5a);
    }
    for (let k = 0; k < 3; k++) {
      const rise = (now * 0.22 + k / 3) % 1;
      wet(cx - 3 + k * 3 + Math.round(Math.sin(rise * 9 + k)), cy + 6 - Math.round(rise * 13), 0xcfe8f8, 0.75 - rise * 0.35);
    }
    // A visitor every so often, a fish one time and a jellyfish the next
    const at = now + x * 0.3, tt = at % 14;
    if (tt < 5) {
      if (Math.floor(at / 14) % 2 === 0) {
        const fx = cx - 10 + Math.round((tt / 5) * 19), fy = cy - 1 + Math.round(Math.sin(tt * 2)), flap = Math.floor(now * 6) % 2;
        for (const [dx, dy] of [[0, 0], [1, 0], [2, 0], [0, 1], [1, 1], [2, 1], [1, -1]]) wet(fx + dx, fy + dy, 0xf08a3a);
        wet(fx + 2, fy, 0x1a1a1f); wet(fx - 1, fy - flap, 0xf2b05a); wet(fx - 1, fy + 1 + (1 - flap), 0xf2b05a);
      } else {
        const jx = cx - 4 + Math.round((tt / 5) * 7), jy = cy + 9 - Math.round((tt / 5) * 16), pulse = Math.floor(now * 2) % 2;
        for (let k = 0; k < 3; k++) wet(jx + k, jy, 0xf2a0d8, 0.85);
        for (let k = -1; k < 4; k++) wet(jx + k, jy + 1, 0xf2a0d8, 0.85);
        for (const k of [-1, 1, 3]) wet(jx + k, jy + 2 + pulse, 0xf2a0d8, 0.5);
        wet(jx + 1, jy + 3, 0xf2a0d8, 0.4);
      }
    }
    for (let k = 0; k < 10; k++) { const a = (k * Math.PI) / 5; dot(cx + Math.round(Math.cos(a) * 10), cy + Math.round(Math.sin(a) * 10), 0xb8c0cc); }
    fill(cx + 11, cy - 2, 2, 5, 0x3a3f4a); fill(cx + 11, cy - 2, 1, 5, 0x6a7280);
    for (const [dx, dy] of [[-4, -5], [-5, -4], [-3, -6]]) dot(cx + dx, cy + dy, WHITE, 0.45);
  },
  /**
   * Big window onto the Earth. A big round porthole in a white frame onto the Earth seen close, its edge curving across:
   * clouds drift over the land, the day turns to night across it, and its cities light up while it's night in the office.
   * Now and then a satellite or the Moon crosses the black above.
   */
  earthwindow(x) {
    const cx = x, cy = 16, R = 11, ex = cx - 6, ey = cy + 20, ER = 22;
    disc(cx, cy, 14.5, 0x9aa0aa); disc(cx, cy, 13.5, 0xe6e9ee); disc(cx, cy, 12.2, 0x5a6270); disc(cx, cy, R + 0.5, 0x04060f);
    const inside = (px, py) => (px - cx) ** 2 + (py - cy) ** 2 < (R + 0.2) ** 2;
    const glass = (px, py, colour, alpha = 1) => { if (inside(px, py)) dot(px, py, colour, alpha); };
    for (const [sx, sy] of [[-6, -6], [2, -9], [6, -5], [-2, -3], [8, -1], [-9, -2]]) glass(cx + sx, cy + sy, WHITE, sx % 4 ? 0.5 : 0.85);
    const at = now + x * 0.2, tt = at % 27, which = Math.floor(at / 27) % 2;
    // The Moon, slowly, behind the Earth's edge
    if (tt < 9 && which === 1) {
      const mx = cx + 10 - Math.round((tt / 9) * 18), my = cy - 5;
      for (let dy = -2; dy <= 2; dy++) for (let dx = -2; dx <= 2; dx++) if (dx * dx + dy * dy <= 5) glass(mx + dx, my + dy, dx + dy > 1 ? 0xa8a8a0 : 0xe2e2d8);
      glass(mx - 1, my, 0xb8b8b0);
    }
    // The Earth: its land turning slowly, clouds drifting over faster, night on its right
    const land = Math.floor(now * 0.3), cloud = Math.floor(now * 0.7), lights = [];
    for (let py = cy - R; py <= cy + R; py++) for (let px = cx - R; px <= cx + R; px++) {
      const d = Math.hypot(px - ex, py - ey);
      if (!inside(px, py) || d > ER + 1) continue;
      if (d > ER) { dot(px, py, 0x4a8ad0, 0.55); continue; }
      const u = px - ex + land, v = py - ey, night = px - cx > 2 + (cy - py) * 0.35, dusk = !night && px - cx > (cy - py) * 0.35;
      const ground = Math.sin(u * 0.31) + Math.sin(u * 0.13 + v * 0.45) + 0.6 * Math.sin(v * 0.9 - u * 0.2), isLand = ground > 0.9;
      const cu = px - ex + cloud, isCloud = Math.sin(cu * 0.47 + v * 0.25) * Math.sin(v * 1.2 + cu * 0.06) > 0.55;
      let colour = isLand ? ((Math.floor(u / 7) + v) % 5 === 0 ? 0xc8a86a : 0x5a9a4a) : 0x2f72c0;
      if (night) colour = isLand ? 0x15283a : 0x0b1a36;
      dot(px, py, colour);
      if (dusk) dot(px, py, 0xe08a4a, 0.3);
      if (isCloud) dot(px, py, WHITE, night ? 0.18 : 0.85);
      else if (night && ground > 0 && isNight && hash(u, v) % 3 === 0) lights.push([px, py]);
      if (d > ER - 1) dot(px, py, 0x9fdcff, night ? 0.25 : 0.7);
    }
    if (lights.length) {
      const city = () => lights.forEach(([px, py], k) => dot(px, py, 0xffcf6a, k % 3 ? 0.85 : 0.55));
      city(); glowLate(city);
    }
    // A satellite, gliding across, its light blinking
    if (tt < 7 && which === 0) {
      const sx = cx - 12 + Math.round((tt / 7) * 24), sy = cy - 6 + Math.round((tt / 7) * 3);
      glass(sx, sy, 0xd8dce2); glass(sx + 1, sy, 0xb8bec8);
      for (const dx of [-2, -1, 2, 3]) glass(sx + dx, sy, 0x3f6fc4);
      if (Math.floor(now * 2) % 2) glass(sx, sy - 1, 0xff5a4a);
    }
    // The glass's shine, the frame's bolts, and a latch
    for (const [dx, dy] of [[-6, -7], [-7, -6], [-8, -5], [-5, -8]]) glass(cx + dx, cy + dy, WHITE, 0.3);
    for (let k = 0; k < 12; k++) { const a = (k * Math.PI) / 6; dot(cx + Math.round(Math.cos(a) * 13), cy + Math.round(Math.sin(a) * 13), 0x8a919c); }
    fill(cx + 14, cy - 3, 2, 6, 0x8a919c); fill(cx + 14, cy - 3, 1, 6, 0xc4c9d2); dot(cx + 15, cy, PALETTE.hazard);
  },
  /** Wall TV. A platformer plays: the hero runs, jumps the pipe, coins blink. */
  tv(x) {
    fill(x - 16, 5, 32, 21, 0x4c8bf5, 0.05);
    fill(x - 14, 7, 28, 17, 0x14161b); fill(x - 13, 8, 26, 15, 0x5a8ad0);
    fill(x - 13, 19, 26, 4, 0x8a5a2b); fill(x - 13, 19, 26, 1, 0x4f9a5a);
    fill(x + 1, 15, 4, 4, 0x3f9a4a); fill(x, 14, 6, 1, 0x4fae6a);
    fill(x - 9, 10, 5, 2, WHITE, 0.85);
    for (const cx of [x - 6, x + 8]) if (Math.floor(now * 3) % 2) fill(cx, 13, 1, 2, 0xf2d35b); else fill(cx - 0, 13, 2, 2, 0xf2d35b);
    const lap = (now * 0.25) % 1, hx = x - 12 + Math.round(lap * 22), jump = Math.max(0, 1 - Math.abs(hx - (x + 3)) / 6), hy = 16 - Math.round(jump * 5);
    fill(hx, hy, 2, 3, 0xd4140b); dot(hx, hy, 0xe8b48a);
    fill(x - 2, 24, 5, 1, 0x2a2a2e); dot(x + 12, 23, 0xd4140b);
  },
  /** Hex light panels. Each tile runs round the colour wheel a step behind the last. */
  hex(x) {
    const widths = [3, 5, 7, 7, 5, 3];
    [[-8, 8], [0, 8], [-12, 14], [-4, 14], [4, 14], [-8, 20], [0, 20]].forEach(([dx, dy], i) => {
      const c = rainbow((now * 0.12 + i * 0.13) % 1);
      fill(x + dx - 1, 7 + dy - 9, 9, 8, c, 0.06);
      widths.forEach((w, r) => fill(x + dx + 3 - (w - 1) / 2, dy + r - 1, w, 1, shaded(c, r < 2 ? 1 : 0.8)));
    });
  },
  /** Pegboard of tools. Hammer, wrenches, screwdriver, pliers and a tape measure. */
  pegboard(x) {
    const l = x - 13, t = 7, w = 26, h = 20;
    fill(l, t, w, h, 0xb08a5a); fill(l, t, w, 1, 0xc49a6c); fill(l, t + h - 1, w, 1, 0x8c6a40);
    for (let py = t + 2; py < t + h - 1; py += 3) for (let px = l + 2; px < l + w - 1; px += 3) dot(px, py, 0x7a5a36);
    fill(l + 3, t + 6, 1, 10, 0x8c6744); fill(l + 1, t + 4, 5, 2, 0x5a5f6c);
    fill(l + 8, t + 5, 1, 11, 0x9aa0a6); fill(l + 7, t + 3, 3, 2, 0x9aa0a6); dot(l + 8, t + 3, 0xb08a5a); fill(l + 7, t + 16, 3, 1, 0x9aa0a6);
    fill(l + 12, t + 3, 2, 5, 0xe8c04a); fill(l + 12, t + 8, 1, 7, 0xc8cdd6);
    fill(l + 16, t + 3, 3, 3, 0x9aa0a6); fill(l + 16, t + 6, 1, 8, 0xc9443a); fill(l + 18, t + 6, 1, 8, 0xc9443a);
    fill(l + 21, t + 4, 4, 4, 0xe8c04a); dot(l + 22, t + 5, 0x2a2a2e); fill(l + 21, t + 8, 1, 2, 0xe8c04a);
    ring(l + 22, t + 14, 2.5, 0x2a2a2e);
  },
  /** Roller door, half up. Opens onto the drive, the sky as the hour has it, the founder's car outside. */
  roller(x) {
    const l = x - 15, w = 30;
    skyPane(l, 19, w, 12);
    fill(l, 28, w, 3, 0x2f5a3a); for (let k = 0; k < w; k += 3) dot(l + k + 1, 27, 0x2f5a3a);
    fill(l, 31, w, 7, 0x8a8d92); fill(l, 31, w, 1, 0x9a9da2); fill(x - 1, 33, 2, 4, 0xeae6dc, 0.6);
    fill(x + 3, 30, 10, 3, 0xc9443a); fill(x + 5, 28, 6, 2, 0xc9443a); fill(x + 6, 28, 4, 1, 0x9fd3e8); dot(x + 5, 33, 0x1d1f25); dot(x + 11, 33, 0x1d1f25);
    fill(l, 6, w, 13, 0x9aa0a8); for (let y = 7; y < 19; y += 2) fill(l, y, w, 1, 0x7a7f88); fill(l, 18, w, 1, 0x5a5f6c); fill(x - 2, 17, 5, 1, 0x3a3f4a);
    fill(l - 2, 4, 2, 34, 0x5a5f6c); fill(l + w, 4, 2, 34, 0x5a5f6c); fill(l - 2, 3, w + 4, 3, 0x4a4e56); fill(l - 2, 3, w + 4, 1, 0x6a6e76);
    fill(l, 37, w, 1, 0x5a5f6c);
  },
  /** Bike on hooks. Hung on the wall; a wheel still turns slowly. */
  bike(x) {
    fill(x - 7, 7, 1, 2, 0x3a3f4a); fill(x + 6, 7, 1, 2, 0x3a3f4a);
    for (const [cx, spin] of [[x - 7, 0.6], [x + 7, 0]]) {
      ring(cx, 19, 5, 0x2a2a2e); ring(cx, 19, 4, 0x3a3d47, 0.6);
      for (let k = 0; k < 4; k++) { const a = now * spin + (k * Math.PI) / 4; for (let r = 1; r <= 3; r++) { dot(cx + Math.round(Math.cos(a) * r), 19 + Math.round(Math.sin(a) * r), 0x9aa0a6); dot(cx - Math.round(Math.cos(a) * r), 19 - Math.round(Math.sin(a) * r), 0x9aa0a6); } }
      dot(cx, 19, 0xc8cdd6);
    }
    const red = 0xc9443a;
    line(x - 7, 19, x - 1, 19, red); line(x - 7, 19, x - 3, 12, red); line(x - 3, 12, x - 1, 19, red); line(x - 3, 12, x + 5, 12, red); line(x - 1, 19, x + 5, 12, red); line(x + 5, 12, x + 7, 19, red);
    line(x - 3, 12, x - 4, 10, 0x5a5f6c); fill(x - 6, 9, 4, 1, 0x1d1f25);
    line(x + 5, 12, x + 5, 9, 0x5a5f6c); fill(x + 3, 9, 4, 1, 0x1d1f25);
    dot(x - 1, 19, 0xc8cdd6);
  },
  /** Gym mirror. A glint sweeps across it now and then. */
  mirror(x) {
    const l = x - 10, t = 5, w = 21, h = 25;
    fill(l - 1, t - 1, w + 2, h + 1, 0x8a8f9c); fill(l, t, w, h - 1, 0xa8c0d0); fill(l, t, w, 8, 0xb8ccda); fill(l, t + h - 7, w, 6, 0x7a8a96);
    fill(l + 2, t + 2, 1, 6, WHITE, 0.4); fill(l + 4, t + 2, 1, 3, WHITE, 0.3);
    const pos = ((now * 8) % 80) - 20;
    for (let row = 0; row < h - 1; row++) { const sx = Math.round(pos + row * 0.6); if (sx >= 0 && sx < w) dot(l + sx, t + row, WHITE, 0.45); if (sx + 2 >= 0 && sx + 2 < w) dot(l + sx + 2, t + row, WHITE, 0.2); }
  },
  /** Interval timer. Counts down the round in red, then the rest in green. */
  timer(x) {
    fill(x - 14, 9, 28, 13, 0x2a2c30); fill(x - 13, 10, 26, 11, 0x0e0f12);
    const t = Math.floor(now) % 60, working = t < 45, left = working ? 45 - t : 60 - t;
    const text = `0:${String(left).padStart(2, '0')}`;
    const colour = working ? 0xff3a2a : 0x3ddc84;
    pixelText(text, x - 7, 12, colour, false);
    [0, 1, 2].forEach((k) => dot(x - 3 + k * 3, 19, k === Math.floor(now / 60) % 3 ? 0xf2d35b : 0x3a3d47));
  },
  /** Climbing holds. A panel of holds in every colour. */
  climbing(x) {
    const l = x - 13, t = 5;
    fill(l, t, 26, 25, 0xc49a6c); fill(l, t, 26, 1, 0xd8b080); fill(l + 25, t, 1, 25, 0xa07a50);
    for (let py = t + 2; py < t + 25; py += 4) for (let px = l + 2; px < l + 25; px += 4) dot(px, py, 0x8c6a40);
    for (const [dx, dy, w, h, c] of [[3, 3, 3, 2, 0xd4140b], [12, 2, 2, 2, 0xf2d35b], [19, 5, 3, 3, 0x3f6fc4], [6, 9, 2, 2, 0x4f9a5a], [15, 10, 4, 2, 0xe07aa0], [3, 15, 3, 3, 0x7d5ab8], [11, 16, 2, 2, 0xe0703a], [20, 14, 2, 3, 0x3ddc84], [8, 21, 4, 2, 0x3f6fc4], [18, 20, 3, 2, 0xd4140b]]) {
      fill(l + dx, t + dy, w, h, c); fill(l + dx, t + dy + h, w, 1, shaded(c, 0.55)); dot(l + dx, t + dy, lighter(c));
    }
  },
  /** Hanging vines. They sway, most at their tips; a flower or two. */
  vines(x) {
    fill(x - 15, 2, 31, 1, 0x3b5a3a);
    for (const [dx, len, ph] of [[-13, 18, 0], [-8, 24, 1.1], [-3, 14, 2.3], [2, 26, 0.6], [7, 16, 1.8], [12, 22, 2.9]]) {
      let last = 0;
      for (let k = 0; k < len; k++) {
        const sway = k > 4 ? Math.round(Math.sin(now * 1.1 + ph + k * 0.25) * (k / len) * 1.8) : 0, vx = x + dx + sway, vy = 3 + k;
        dot(vx, vy, 0x3b6a3a); last = vx;
        if (k % 4 === 2) fill(vx + (k % 8 === 2 ? 1 : -2), vy, 2, 1, PALETTE.leaf);
        if (k % 4 === 0 && k > 0) dot(vx + (k % 8 === 0 ? -1 : 1), vy, PALETTE.leafDark);
      }
      if (ph > 1.5) fill(last - 1, 3 + len, 2, 2, ph > 2.5 ? 0xf28fb0 : 0xf2d35b);
    }
  },
  /** Tiki mask. Its eyes glow orange for a moment every so often. */
  tiki(x) {
    for (const [dx, h] of [[-5, 4], [-3, 6], [-1, 7], [1, 7], [3, 6], [5, 4]]) fill(x + dx, 9 - h, 1, h, dx % 4 === 1 || dx === -3 ? PALETTE.leafDark : PALETTE.leaf);
    fill(x - 5, 8, 11, 19, 0x8a5a2b); fill(x - 4, 27, 9, 1, 0x8a5a2b); fill(x - 5, 8, 1, 19, 0xa87048); fill(x + 5, 8, 1, 19, 0x6e4a2e);
    fill(x - 5, 12, 11, 1, 0x6e4a2e);
    const glow = (now % 8) < 0.8 ? 1 : 0;
    for (const ex of [x - 4, x + 2]) { fill(ex, 13, 3, 2, 0x1d1f25); if (glow) { fill(ex, 13, 3, 2, 0xff8a3d); fill(ex - 1, 12, 5, 4, 0xff8a3d, 0.2); } }
    fill(x - 1, 15, 3, 4, 0x6e4a2e); dot(x - 1, 18, 0x5a3b25); dot(x + 1, 18, 0x5a3b25);
    fill(x - 4, 20, 9, 4, 0x1d1f25); for (let k = 0; k < 9; k += 2) { dot(x - 4 + k, 20, 0xeae6dc); dot(x - 4 + k, 23, 0xeae6dc); }
    fill(x - 5, 16, 2, 1, 0xc9443a); fill(x + 4, 16, 2, 1, 0xc9443a); fill(x - 5, 18, 2, 1, 0xc9443a); fill(x + 4, 18, 2, 1, 0xc9443a);
  },
  /** Bamboo blind window. A blind half down, palms outside, the sky as the hour has it. */
  blind(x) {
    const l = x - 16, t = 6, w = 32, h = 20;
    fill(l - 1, t - 1, w + 2, h + 2, PALETTE.frame); skyPane(l, t, w, h);
    const palm = isNight ? 0x1a2a20 : 0x2f5a3a;
    fill(l + 3, t + 12, 2, 8, 0x5a3b25); for (const [dx, dy, w2] of [[-2, 11, 4], [4, 11, 4], [-3, 12, 2], [6, 12, 2], [0, 10, 5]]) fill(l + 3 + dx, t + dy, w2, 1, palm);
    fill(l + w - 7, t + 14, 2, 6, 0x5a3b25); for (const [dx, dy, w2] of [[-3, 13, 4], [2, 13, 4], [-1, 12, 5]]) fill(l + w - 7 + dx, t + dy, w2, 1, palm);
    for (let row = t; row < t + 9; row++) fill(l, row, w, 1, row % 2 ? 0xc9a06a : 0xb08a5a);
    fill(l, t + 9, w, 1, 0x8a6a42);
    fill(l - 2, t - 2, w + 4, 2, 0xc9a06a); fill(l - 2, t - 1, w + 4, 1, 0x8a6a42);
    fill(l + w - 3, t + 10, 1, 9, 0xeae6dc); fill(l + w - 4, t + 19, 3, 2, 0xc9443a);
    fill(l - 2, t + h + 1, w + 4, 1, PALETTE.sill);
  },
  /** Cabin window. Pines outside under the sky of the hour, gingham curtains tied back. */
  cabin(x) {
    const l = x - 14, t = 6, w = 28, h = 20;
    fill(l - 2, t - 2, w + 4, h + 4, 0x8c6744); skyPane(l, t, w, h);
    if (isNight) fill(l + w - 7, t + 3, 3, 3, 0xf1ead2);
    const pine = isNight ? 0x14261a : 0x2f5a3a;
    for (const [px, ph] of [[3, 9], [8, 12], [14, 8], [19, 11], [25, 9]]) for (let r = 0; r < ph; r++) { const half = Math.floor((r + 1) / 2); fill(l + px - half, t + h - ph + r, half * 2 + 1, 1, pine); }
    fill(x, t, 1, h, 0x8c6744); fill(l, t + 10, w, 1, 0x8c6744);
    for (const [cx, dir] of [[l - 3, 1], [l + w - 2, -1]]) for (let row = t - 2; row < t + h + 1; row++) {
      const wide = row < t + 12 ? 5 : row < t + 14 ? 2 : 3 + Math.floor((row - t - 14) / 3);
      for (let k = 0; k < wide; k++) { const px = dir > 0 ? cx + k : cx + 4 - k; dot(px, row, (Math.floor(px / 2) + Math.floor(row / 2)) % 2 ? 0xc9443a : 0xeae6dc); }
    }
    fill(l - 3, t + h + 2, w + 6, 1, 0xa87a56);
  },
  /** Trail map. Contours, a river, the trail, and a blinking "you are here". */
  trail(x) {
    const l = x - 13, t = 7, w = 26, h = 20;
    fill(l, t, w, h, 0x6e4a2e); fill(l + 1, t + 1, w - 2, 3, 0x2f5a3a); fill(l + 3, t + 2, 10, 1, 0xd8e0b0, 0.8);
    fill(l + 1, t + 4, w - 2, h - 5, 0xd8e0b0);
    for (const r of [2, 4, 6]) ring(l + 17, t + 11, r, 0xa8b878);
    dot(l + 17, t + 11, 0x6e4a2e);
    for (let k = 0; k < w - 2; k++) dot(l + 1 + k, t + 15 + Math.round(Math.sin(k * 0.5) * 1.2), 0x6a9ac8);
    for (const [dx, dy] of [[3, 17], [5, 15], [7, 13], [9, 12], [11, 11], [13, 11], [15, 11]]) dot(l + dx, t + dy, 0xc9443a);
    if (Math.floor(now * 2) % 2) fill(l + 3, t + 17, 2, 2, 0xd4140b);
    fill(l + 6, t + 6, 3, 2, 0xe0703a); dot(l + 7, t + 5, 0xe0703a);
  },
  /** Jack-o'-lantern shelf. Three carved pumpkins on a shelf, each a different face, candles flickering in them. */
  pumpkins(x) {
    fill(x - 15, 22, 30, 1, 0x7a5232);
    fill(x - 15, 23, 30, 1, 0x5a3b25);
    for (const bx of [x - 11, x + 10]) { fill(bx, 24, 1, 3, 0x3a2616); dot(bx - 1, 24, 0x3a2616); }
    drawPumpkin(x - 9, 22, false, 2, candlelight(0));
    drawPumpkin(x, 22, true, 0, candlelight(1));
    drawPumpkin(x + 9, 22, false, 1, candlelight(2));
    for (const [lx, c] of [[x - 14, LEAVES[0]], [x + 13, LEAVES[2]]]) fill(lx, 21, 2, 1, c);
  },
  /** Hanging lantern. An iron lantern on a chain from a wall bracket, its candle wavering, its light on the wall. */
  lanternhang(x) {
    const iron = 0x2a2a2e, glow = candlelight(x);
    glowAt(x, 14, 10, 9, 0xffb04a, (isNight ? 0.14 : 0.07) * glow);
    fill(x - 5, 2, 9, 1, iron);
    dot(x - 5, 3, iron);
    for (let cy = 3; cy < 8; cy++) dot(x, cy, cy % 2 ? 0x5a5f6c : iron);
    fill(x - 1, 8, 3, 1, iron);
    fill(x - 3, 9, 7, 1, iron);
    fill(x - 2, 10, 5, 7, 0xffc860, glow);
    fill(x, 12, 1, 3, 0xfff0b0);
    dot(x, 11, 0xffe08a, glow);
    fill(x - 3, 10, 1, 7, iron);
    fill(x + 3, 10, 1, 7, iron);
    fill(x - 3, 17, 7, 1, iron);
    fill(x - 2, 18, 5, 1, iron);
    dot(x, 19, iron);
  },
  /** Cobweb. A big web strung across the wall, a spider going up and down its thread. */
  cobweb(x) {
    const cy = 14;
    const at = (k, r) => [x + Math.round(Math.cos((k / 8) * Math.PI * 2 + 0.2) * r), cy + Math.round(Math.sin((k / 8) * Math.PI * 2 + 0.2) * r * 0.9)];
    for (let k = 0; k < 8; k++) line(x, cy, ...at(k, 12), WHITE, 0.35);
    for (const r of [4, 8, 12]) for (let k = 0; k < 8; k++) line(...at(k, r), ...at(k + 1, r), WHITE, 0.3);
    const sx = x + 5, sy = 22 + Math.round((Math.sin(now * 0.6) + 1) * 2.5);
    line(sx, cy + 4, sx, sy - 1, WHITE, 0.4);
    fill(sx - 1, sy, 3, 2, 0x1a1a1f);
    for (const [dx, dy] of [[-2, 0], [2, 0], [-2, 2], [2, 2], [-3, 1], [3, 1]]) dot(sx + dx, sy + dy, 0x3a3d47);
    dot(sx, sy + 1, 0xd4140b, 0.8);
  },
  /** Hanging ghosts. Three sheet ghosts pegged to a string, their hems fluttering. */
  ghosts(x) {
    const sag = (dx) => 5 + Math.round(Math.sin(((dx + 15) / 30) * Math.PI) * 3);
    for (let dx = -15; dx <= 15; dx++) dot(x + dx, sag(dx), 0x8a8f9c, 0.8);
    dot(x - 16, 4, PALETTE.frame);
    dot(x + 16, 4, PALETTE.frame);
    const ghost = ['..XXX..', '.XXXXX.', 'XXXXXXX', 'XXEXEXX', 'XXXXXXX', 'XXXMXXX', 'XXXXXXX', 'XXXXXXX'];
    [-9, 0, 9].forEach((gx, i) => {
      const top = sag(gx) + 1 + (i === 1 ? 2 : 0), l = x + gx - 3;
      const hem = (Math.floor(now * 1.5) + i) % 2 ? 'X.X.X.X' : '.X.X.X.';
      [...ghost, hem].forEach((row, r) => [...row].forEach((c, k) => {
        if (c === '.') return;
        dot(l + k, top + r, c === 'X' ? (k === 6 ? 0xd8d4c8 : 0xf4f1ea) : 0x1a1a1f);
      }));
      dot(x + gx, sag(gx), 0xc9443a);
    });
  },
  /** Witch's broom. A broom across two pegs, a pointed hat hung under it. */
  broom(x) {
    const wood = 0x6e4a2e;
    for (const px of [x - 8, x + 2]) fill(px, 9, 1, 2, 0x3a2616);
    fill(x - 13, 8, 19, 1, wood);
    fill(x - 13, 7, 19, 1, 0x8a6040);
    for (let k = 0; k < 7; k++) {
      const half = 1 + Math.floor(k / 2);
      fill(x + 7 + k, 8 - half, 1, half * 2 + 1, k % 2 ? 0xc9a06a : 0xa8803a);
    }
    fill(x + 5, 6, 2, 4, 0x5a3b25);
    // The hat, hung by its tip
    const hat = 0x2a2433, edge = 0x3e3650;
    dot(x - 2, 10, 0x3a2616);
    dot(x - 2, 11, hat);
    dot(x - 3, 12, hat);
    for (const [y, l, w] of [[13, -5, 3], [14, -5, 3], [15, -6, 5], [16, -6, 5], [17, -7, 7], [18, -7, 7]]) {
      fill(x + l, y, w, 1, hat);
      dot(x + l, y, edge);
    }
    fill(x - 7, 19, 7, 1, 0x7d5ab8);
    dot(x - 4, 19, PALETTE.brass);
    fill(x - 9, 20, 11, 1, hat);
    fill(x - 9, 20, 11, 1, edge, 0.5);
  },
  /**
   * Scarf hooks. A peg rail with a striped scarf, a beanie, a pair of mittens on a string and a long scarf hung up to
   * dry; now and then a drip gathers on a mitten and falls.
   */
  scarfhooks(x) {
    const berry = 0xb8303a, rib = 0x8a2430, cream = 0xf2ead8, pine = 0x2f6a4a, pineDark = 0x245a3c;
    fill(x - 16, 7, 33, 3, 0x8a6040); fill(x - 16, 7, 33, 1, 0xa8804e); fill(x - 16, 10, 33, 1, SHADOW);
    for (const px of [-12, -4, 4, 12]) fill(x + px, 8, 2, 2, 0x4a301e);
    // A berry scarf over the first peg, both ends hanging, cream stripes and a fringe
    fill(x - 14, 9, 6, 2, berry);
    for (const [tx, len] of [[-14, 13], [-10, 11]]) {
      for (let r = 0; r < len; r++) fill(x + tx, 11 + r, 2, 1, r % 4 === 2 ? cream : berry);
      dot(x + tx, 11 + len, cream); dot(x + tx + 1, 12 + len, cream);
    }
    // A pine beanie by its brim, ribbed, a cream bobble
    fill(x - 6, 10, 7, 2, cream); fill(x - 6, 12, 7, 4, pine); fill(x - 5, 16, 5, 1, pine);
    for (const dx of [-5, -3, -1]) fill(x + dx, 12, 1, 4, pineDark);
    fill(x - 4, 17, 3, 2, cream); dot(x - 4, 17, WHITE);
    // Mittens on a string, cream cuffs
    dot(x + 4, 10, cream); dot(x + 3, 11, cream); dot(x + 6, 11, cream); dot(x + 2, 12, cream); dot(x + 7, 12, cream);
    for (const mx of [1, 7]) {
      fill(x + mx - 1, 13, 3, 1, cream); fill(x + mx - 1, 14, 3, 4, berry); fill(x + mx, 18, 2, 1, berry);
      dot(x + mx + (mx < 4 ? -2 : 2), 15, berry); dot(x + mx + 1, 14, rib);
    }
    // A long pine scarf on the last peg, cream bands, a fringe
    for (let r = 0; r < 17; r++) fill(x + 11, 10 + r, 3, 1, r % 6 === 3 || r % 6 === 4 ? cream : r % 2 ? pine : pineDark);
    for (const dx of [11, 13]) dot(x + dx, 27, cream);
    // The drip: gathering on a mitten's tip, then falling
    const t = (now + x * 0.13) % 9;
    if (t < 0.7) dot(x + 8, 19, 0xbfe0f0, 0.5 + t * 0.5);
    else if (t < 1.5) {
      const fall = (t - 0.7) / 0.8;
      dot(x + 8, 19 + Math.round(fall * 12), 0xbfe0f0, 0.85 * (1 - fall * 0.6));
    }
  },
  /** Autumn garland. A swag of leaves along a string, a little pumpkin every so often. */
  /** Christmas garland. A swag of pine along the wall, red bows at its ends, its coloured lights blinking slowly in turn. */
  garland(x) {
    const sag = (dx) => 6 + Math.round(Math.sin(((dx + 18) / 36) * Math.PI) * 5), tick = Math.floor(now * 1.2);
    const bulbs = [0xe0402a, 0xf2d35b, 0x5aa0e8, 0x7ae08a];
    for (let dx = -18; dx <= 18; dx++) {
      const y = sag(dx);
      fill(x + dx, y, 1, 2, dx % 2 ? 0x2f6a3c : 0x3f8a4c);
      if (dx % 3 === 0) dot(x + dx, y - 1, 0x24502f);
      if (dx % 3 === 1) dot(x + dx, y + 2, 0x24502f);
      if ((dx + 18) % 5 !== 2) continue;
      const k = (dx + 16) / 5, colour = bulbs[k % 4], on = (k + tick) % 4 !== 0;
      dot(x + dx, y + 2, colour, on ? 1 : 0.35);
      dot(x + dx, y + 3, colour, on ? 0.8 : 0.25);
      if (on && isNight) glowLate(() => glowAt(x + dx, y + 3, 1, 1, colour, 0.35));
    }
    for (const bx of [x - 18, x + 18]) {
      fill(bx - 2, 4, 2, 2, 0xc8302a); fill(bx + 1, 4, 2, 2, 0xc8302a); dot(bx, 5, 0x8a2420);
      dot(bx - 1, 6, 0xc8302a); dot(bx + 1, 6, 0xc8302a); dot(bx - 1, 7, 0xc8302a); dot(bx + 1, 7, 0xc8302a);
    }
  },
  leafgarland(x) {
    const sag = (dx) => 5 + Math.round(Math.sin(((dx + 17) / 34) * Math.PI) * 4);
    for (let dx = -17; dx <= 17; dx++) {
      const y = sag(dx);
      dot(x + dx, y, 0x5a3b25);
      if (((dx + 17) % 11) === 5) {
        fill(x + dx - 1, y + 1, 3, 3, 0xe0702a);
        dot(x + dx + 1, y + 2, 0xb04e1a);
        dot(x + dx, y, 0x3f8a4a);
      } else if (dx % 2 === 0) {
        fill(x + dx, y + 1, 2, 2, LEAVES[Math.floor((dx + 17) / 2) % 4]);
        dot(x + dx + 1, y + 3, LEAVES[Math.floor((dx + 19) / 2) % 4]);
      }
    }
  },
  /**
   * Haunted hoop. Cobwebs on the backboard, a sheet ghost for a net; every
   * so often a jack-o'-lantern ball drops through it and the ghost bounces.
   */
  hauntedhoop(x) {
    const t = now % 20, dropping = t < 1.4, by = 4 + Math.round((t / 1.4) * 34);
    fill(x - 13, 2, 27, 15, 0x6a6e76);
    fill(x - 12, 3, 25, 13, 0xd8d4c8);
    fill(x - 12, 3, 25, 1, 0xe8e4d8);
    for (const [l, t2, w, h] of [[-5, 8, 11, 1], [-5, 8, 1, 7], [5, 8, 1, 7]]) fill(x + l, t2, w, h, 0x9a3a32, 0.7);
    // Cobwebs in its corners, and a spider let down from one
    for (const [cx, dir] of [[x - 12, 1], [x + 12, -1]]) {
      for (const r of [3, 6]) for (let a = 0; a <= 6; a++) {
        const angle = (a / 6) * (Math.PI / 2);
        dot(cx + dir * Math.round(Math.cos(angle) * r), 3 + Math.round(Math.sin(angle) * r), WHITE, 0.5);
      }
      line(cx, 3, cx + dir * 6, 9, WHITE, 0.4);
    }
    const sy = 21 + Math.round((Math.sin(now * 0.7) + 1) * 2);
    line(x + 10, 16, x + 10, sy - 1, WHITE, 0.4);
    fill(x + 9, sy, 3, 2, 0x1a1a1f);
    for (const [dx, dy] of [[-1, 0], [3, 0], [-1, 2], [3, 2]]) dot(x + 9 + dx, sy + dy, 0x3a3d47);
    if (dropping && by < 22) drawBall(x, by, 3, true);
    fill(x - 1, 16, 3, 3, 0x5a5f6c);
    fill(x - 6, 19, 13, 2, 0xe0602a);
    fill(x - 6, 19, 13, 1, 0xf08a4a);
    // The net, a ghost hung from the rim, its hem fluttering
    const bob = dropping && by >= 18 && by < 30 ? 1 : 0;
    const ghost = ['XXXXXXXXX', 'XXXXXXXXX', 'XXEXXXEXX', 'XXXXXXXXX', '.XXXOXXX.', '.XXXXXXX.', '..XXXXX..'];
    const hem = Math.floor(now * 1.5) % 2 ? '..X.X.X..' : '...X.X...';
    [...ghost, hem].forEach((row, r) => [...row].forEach((c, k) => {
      if (c !== '.') dot(x - 4 + k, 21 + r + bob, c === 'X' ? (k === 8 ? 0xd8d4c8 : 0xf4f1ea) : 0x1a1a1f, c === 'X' ? 0.92 : 1);
    }));
    if (dropping && by >= 30) drawBall(x, Math.min(by, 35), 3, true, 1 - (by - 30) / 10);
  },
  /** Spooky scoreboard. HOME 13, GHOSTS 31; every so often it flickers and the ghosts' score jumps. */
  spookyboard(x) {
    const flicker = (now % 9) < 0.35, dim = flicker && Math.floor(now * 20) % 2 === 0 ? 0.35 : 1;
    fill(x - 19, 5, 39, 21, 0x3a3d47);
    fill(x - 18, 6, 37, 19, 0x0e0e12);
    for (const [rx, ry] of [[-18, 6], [18, 6], [-18, 24], [18, 24]]) dot(x + rx, ry, 0x6a6e76);
    const amber = shaded(0xffb04a, dim), slime = shaded(0x6fe08a, dim);
    pixelText('HOME', x - 16, 9, amber, false);
    pixelText('13', x + 10, 9, amber, false);
    pixelText('GHOSTS', x - 16, 17, slime, false);
    pixelText(flicker ? '32' : '31', x + 10, 17, slime, false);
    fill(x - 16, 15, 33, 1, 0x26262c);
    // A bat on each top corner
    for (const bx of [x - 19, x + 17]) { fill(bx, 4, 3, 1, 0x1a1a1f); dot(bx - 1, 3, 0x1a1a1f); dot(bx + 3, 3, 0x1a1a1f); dot(bx + 1, 5, 0x1a1a1f); }
  },
  /** Candy-corn pennants. Pennants in candy corn's yellow, orange and white, along a swag of string. */
  candycorn(x) {
    const sag = (dx) => 5 + Math.round(Math.sin(((dx + 17) / 34) * Math.PI) * 3);
    for (let dx = -17; dx <= 17; dx++) dot(x + dx, sag(dx), 0x5a3b25);
    for (let dx = -16; dx <= 11; dx += 9) {
      const top = sag(dx + 3) + 1;
      [[7, 0xf2c43a], [7, 0xf2c43a], [5, 0xe8823a], [5, 0xe8823a], [3, 0xe8823a], [3, 0xf4f1ea], [1, 0xf4f1ea]]
        .forEach(([w, c], r) => fill(x + dx + (7 - w) / 2, top + r, w, 1, c));
    }
  },
  /** Hoop. A backboard with the move's blue square, an orange rim and a white net; now and then a ball drops through and the net swishes. */
  hoop(x) {
    const t = now % 20, dropping = t < 1.2, by = 4 + Math.round((t / 1.2) * 30);
    fill(x - 13, 2, 27, 15, 0x8a8f9c);
    fill(x - 12, 3, 25, 13, 0xf5f5f5);
    fill(x - 12, 3, 25, 1, WHITE);
    for (const [l, t2, w, h] of [[-5, 8, 11, 1], [-5, 8, 1, 7], [5, 8, 1, 7]]) fill(x + l, t2, w, h, 0x3558ce);
    if (dropping && by < 30) drawBall(x, by, 3);
    fill(x - 1, 16, 3, 3, 0x5a5f6c);
    fill(x - 6, 19, 13, 2, 0xe0602a);
    fill(x - 6, 19, 13, 1, 0xf08a4a);
    // The net, a mesh narrowing as it hangs, stretched for a moment by a ball through it
    const stretch = dropping && by >= 18 && by < 30 ? 1 : 0;
    for (let r = 0; r < 7 + stretch; r++) {
      const half = 5 - Math.floor(r / 2);
      for (let k = -half; k <= half; k++) if ((k + r) % 2 === 0) dot(x + k, 21 + r, 0xf4f1ea, 0.9);
    }
    if (dropping && by >= 30) drawBall(x, by, 3);
  },
  /** Score bug. The header bar in the move's blue, the night's time on it; the record under it, which settles digit by digit every so often. */
  scorebug(x) {
    const l = x - 20, t = now % 30, wipe = Math.min(1, t / 0.25);
    fill(l - 1, 5, 43, 22, 0x333333);
    fill(l, 6, Math.max(1, Math.round(41 * wipe)), 7, 0x3558ce);
    if (wipe >= 1) pixelText('WED 9:40', l + 2, 7, WHITE, false);
    fill(l, 13, 41, 13, 0x1a1a1a);
    // The record at twice the size, each digit dropping into place
    let at = l + 2;
    [...'5-3-1'].forEach((c, k) => {
      const land = Math.min(1, Math.max(0, (t - 0.3 - k * 0.09) / 0.09)), drop = Math.round((1 - land) * 3);
      if (land > 0) GLYPHS[c].forEach((row, r) => [...row].forEach((on, column) => { if (on === '1') fill(at + column * 2, 15 + r * 2 - drop, 2, 2, 0xf5f5f5); }));
      at += GLYPHS[c][0].length * 2 + 2;
    });
  },
  /** Lower third. The move's blue tag, MOVE on it, and the body beside it: wiped in left to right, held, wiped out. */
  lowerthird(x) {
    const l = x - 18, t = now % 24;
    const out = t < 22 ? 1 : Math.max(0, 1 - (t - 22) / 0.25);
    const tag = Math.min(1, t / 0.18) * out, body = Math.min(1, Math.max(0, (t - 0.18) / 0.26)) * out;
    if (tag <= 0) return;
    fill(l, 14, Math.max(1, Math.round(18 * tag)), 9, 0x3558ce);
    if (tag >= 1) pixelText('MOVE', l + 1, 16, WHITE, false);
    if (body <= 0) return;
    fill(l + 18, 14, Math.max(1, Math.round(19 * body)), 9, 0x1a1a1a);
    if (body < 1) return;
    fill(l + 18, 14, 19, 1, 0x333333);
    fill(l + 20, 16, 12, 1, 0xf5f5f5);
    fill(l + 20, 18, 15, 1, 0xa3a3a3);
    fill(l + 20, 20, 9, 1, 0x8a8a8a);
  },
  /** On-air sign. Lit red only while the mic is held: on air, for real. */
  onair(x) {
    const on = mic.held;
    if (on) glowAt(x, 15, 18, 9, 0xff2a1a, 0.14);
    fill(x - 1, 7, 3, 3, 0x333333);
    fill(x - 13, 10, 27, 11, 0x1a1a1a);
    fill(x - 12, 11, 25, 9, on ? 0xd4140b : 0x2a1010);
    pixelText('ON AIR', x - 11, 13, on ? WHITE : 0x5a2a2a, false);
  },
  /** Week strip. Monday to Sunday, tonight's cell inverted, a connector dropping from it into the move's lower third. */
  schedule(x) {
    const today = (new Date().getDay() + 6) % 7, l = x - 20;
    fill(l - 1, 6, 43, 10, 0x333333);
    for (let d = 0; d < 7; d++) {
      const cx = l + d * 6, on = d === today;
      fill(cx, 7, 5, 8, on ? 0xf5f5f5 : 0x1a1a1a);
      fill(cx + 1, 9, 3, 1, on ? 0x0e0e0e : 0xa3a3a3);
      fill(cx + 1, 12, 2, 1, on ? 0x555555 : 0x8a8a8a);
    }
    const lt = Math.min(l + today * 6, l + 41 - 18);
    fill(l + today * 6, 16, 1, 3, 0xf5f5f5);
    fill(lt, 19, 4, 5, 0x3558ce);
    fill(lt + 4, 19, 14, 5, 0xf5f5f5);
    fill(lt + 6, 21, 8, 1, 0x0e0e0e);
  },
  /** Pennants. Three felt pennants pinned one above another: the move's blue, the win's green, the loss's red. */
  pennants(x) {
    [[0x3558ce, 7], [0x177a4e, 15], [0xc22f26, 23]].forEach(([c, y], i) => {
      const l = x - 14 + i * 2;
      fill(l - 1, y - 3, 2, 7, 0xf5f5f5);
      for (let k = 0; k < 24; k++) { const half = Math.round(3 * (1 - k / 24)); fill(l + 1 + k, y - half, 1, half * 2 + 1, c); }
      fill(l + 3, y, 9, 1, 0xf5f5f5, 0.8);
      dot(l - 1, y - 3, 0x8a8f9c);
    });
  },
  /**
   * A string of pennants. Felt triangles in the two sides' colours along a sagging string, pinned at each end;
   * every so often a draught runs along them and they flutter, one after the next.
   */
  pennantline(x) {
    const l = x - 19, t = (now + x * 0.13) % 13, gust = t < 1.6;
    const sag = (k) => 7 + Math.round(Math.sin((k / 38) * Math.PI) * 4);
    for (let k = 0; k <= 38; k++) dot(l + k, sag(k), 0xeae6dc);
    for (const px of [l, l + 38]) { dot(px, 6, 0x8a8f9c); dot(px, 7, 0x5a5f6c); }
    for (let i = 0; i < 7; i++) {
      const k = 2 + i * 5.5, px = Math.round(l + k), top = sag(Math.round(k)) + 1;
      const c = i % 2 ? SIDE_ORANGE : SIDE_TEAL, shade = i % 2 ? SIDE_ORANGE_DARK : SIDE_TEAL_DARK;
      const sway = gust && Math.abs(t * 5 - i) < 1.4 ? (Math.floor(now * 8) % 2 ? 1 : -1) : 0;
      for (let r = 0; r < 7; r++) {
        const half = Math.max(0, Math.round(2.5 * (1 - r / 6))), lean = Math.round((sway * r) / 6);
        fill(px - half + lean, top + r, half * 2 + 1, 1, r === 0 ? shade : c);
      }
      dot(px + Math.round(sway * 0.5), top + 2, WHITE, 0.7);
    }
  },
  /**
   * Supporters' scarves. Three striped scarves hung by their middles on pegs along a little rail, their ends
   * fringed: teal and white, orange and white, and one of each. Now and then one sways, each in its own time.
   */
  scarfwall(x) {
    const l = x - 14;
    fill(l, 7, 29, 2, 0x7a5232); fill(l, 7, 29, 1, 0x9a6a42); fill(l + 1, 9, 27, 1, SHADOW);
    const scarves = [[SIDE_TEAL, WHITE, SIDE_TEAL], [SIDE_ORANGE, WHITE, SIDE_ORANGE], [SIDE_TEAL, SIDE_ORANGE, SIDE_TEAL]];
    scarves.forEach((bands, i) => {
      const px = l + 5 + i * 9, t = (now + i * 5.3 + x * 0.07) % 17, sway = t < 1.2 ? (Math.floor(t * 5) % 2 ? 1 : -1) : 0;
      fill(px, 6, 2, 2, 0x5a3b25);
      fill(px - 2, 8, 6, 2, bands[0]);
      for (const [side, len] of [[-2, 17], [2, 14]]) {
        for (let r = 0; r < len; r++) {
          const dx = px + side + (r > len / 2 ? sway : 0);
          fill(dx, 10 + r, 2, 1, bands[Math.floor(r / 3) % 3]);
        }
        const fx = px + side + sway, fy = 10 + len;
        dot(fx, fy, 0xeae6dc); dot(fx + 1, fy, bands[0]);
      }
    });
  },
  /** Framed jersey. A team jersey in the move's blue, its number in white, behind glass. */
  retired(x) {
    const mat = 0xeceae4;
    fill(x - 11, 5, 23, 24, 0x2a2a2a);
    fill(x - 10, 6, 21, 22, mat);
    fill(x - 6, 9, 13, 16, 0x3558ce);
    fill(x - 6, 9, 3, 4, mat);
    fill(x + 4, 9, 3, 4, mat);
    fill(x - 2, 9, 5, 2, mat);
    fill(x - 3, 9, 1, 4, WHITE);
    fill(x + 3, 9, 1, 4, WHITE);
    fill(x - 2, 11, 5, 1, WHITE);
    pixelText('12', x - 3, 15, WHITE, false);
    fill(x + 5, 7, 4, 1, WHITE, 0.4);
    fill(x + 7, 8, 2, 1, WHITE, 0.25);
  },
  /** Play board. X's and O's on the Sidelines board, the move's cut drawn in blue a dot at a time, wiped clean each minute. */
  /** Paper-heart garland. Hearts in pink, red and white, cut from paper, on a swag of string; now and then they sway. */
  heartgarland(x) {
    const sag = (dx) => 4 + Math.round(Math.sin(((dx + 17) / 34) * Math.PI) * 3);
    for (let dx = -17; dx <= 17; dx++) dot(x + dx, sag(dx), 0xc89aa4);
    dot(x - 18, 3, PALETTE.frame); dot(x + 18, 3, PALETTE.frame);
    const t = (now + x * 0.13) % 13, swaying = t < 2.6;
    const papers = [[0xf28aa8, 0xc8607e], [0xd8344a, 0x9e2234], [0xf6efe8, 0xd2c2bc], [0xf28aa8, 0xc8607e]];
    const shape = ['.XX.XX.', 'XXXXXXX', 'XXXXXXX', '.XXXXX.', '..XXX..', '...X...'];
    [-12, -4, 4, 12].forEach((hx, i) => {
      const [paper, fold] = papers[i];
      const swing = swaying ? Math.round(Math.sin(t * 5 + i * 1.7) * (1 - t / 2.6) * 1.6) : 0;
      const top = sag(hx) + 1;
      dot(x + hx, top, 0xc89aa4);
      const l = x + hx - 3 + swing, ht = top + 1;
      shape.forEach((row, r) => [...row].forEach((c, k) => {
        if (c === 'X') dot(l + k, ht + r, k >= 4 && r > 0 ? fold : paper);
      }));
      dot(l + 1, ht + 1, WHITE, 0.55);
    });
  },
  /** "Be mine" neon. A pink neon heart over "be mine" in lowercase, humming softly lit; once in a while it flickers. */
  bemine(x) {
    const blink = (now + x * 0.07) % 23, off = blink > 22.3 && Math.floor(now * 18) % 2 === 0;
    const hum = 0.9 + 0.1 * Math.sin(now * 7 + x);
    glowAt(x, 12, 17, 10, 0xff5aa8, (isNight ? 0.13 : 0.05) * (off ? 0.3 : hum));
    fill(x - 15, 3, 30, 18, 0x2a1e2a, 0.85); fill(x - 15, 3, 30, 1, 0x3e2e3e);
    dot(x - 13, 4, 0x8a8f9c); dot(x + 12, 4, 0x8a8f9c);
    const tube = (px, py, colour) => {
      if (!off) fill(px - 1, py - 1, 3, 3, colour, 0.1 * hum);
      dot(px, py, off ? shaded(colour, 0.35) : lighter(colour));
    };
    ['.XX.XX.', 'X..X..X', 'X.....X', '.X...X.', '..X.X..', '...X...'].forEach((row, r) => [...row].forEach((c, k) => {
      if (c === 'X') tube(x - 3 + k, 5 + r, 0xff3a6a);
    }));
    const letters = {
      b: ['1..', '1..', '111', '1.1', '1.1', '1.1', '111'], e: ['...', '...', '111', '1.1', '111', '1..', '111'],
      m: ['.....', '.....', '1111.', '1.1.1', '1.1.1', '1.1.1', '1.1.1'], i: ['.', '1', '.', '1', '1', '1', '1'],
      n: ['...', '...', '11.', '1.1', '1.1', '1.1', '1.1'], ' ': ['..'],
    };
    let at = x - 13;
    for (const c of 'be mine') {
      letters[c].forEach((row, r) => [...row].forEach((on, k) => { if (on === '1') tube(at + k, 12 + r, 0xff6aa8); }));
      at += letters[c][0].length + 1;
    }
  },
  playboard(x) {
    const l = x - 12, t = now % 60, ink = 0x15161a;
    fill(l, 8, 24, 17, 0xb8bfcc);
    fill(l + 1, 9, 22, 15, 0xf4f5f7);
    for (let gx = l + 4; gx < l + 23; gx += 4) for (let gy = 11; gy < 24; gy += 4) dot(gx, gy, 0xdde1e8);
    fill(l, 25, 24, 1, PALETTE.frame);
    ring(l + 20, 11, 1.5, 0xe0602a);
    if (t >= 56) return;
    const o = (cx, cy) => { dot(cx, cy - 1, ink); dot(cx - 1, cy, ink); dot(cx + 1, cy, ink); dot(cx, cy + 1, ink); };
    const cross = (cx, cy) => { for (const [dx, dy] of [[-1, -1], [1, -1], [0, 0], [-1, 1], [1, 1]]) dot(cx + dx, cy + dy, ink); };
    o(l + 5, 20); o(l + 11, 21); o(l + 17, 19);
    cross(l + 8, 16); cross(l + 15, 15);
    const steps = 14, shown = Math.floor(Math.min(1, t / 6) * steps);
    for (let k = 0; k <= shown; k++) if (k % 3 !== 2) dot(Math.round(l + 5 + (k / steps) * 13), Math.round(18 - (k / steps) * 6), 0x3558ce);
    if (shown === steps) { dot(l + 17, 12, 0x3558ce); dot(l + 18, 13, 0x3558ce); dot(l + 17, 13, 0x3558ce); }
  },
  /** Chalk menu. A café's chalkboard in a wooden frame, hung from a nail: a steaming cup, a leaf, squiggles and a heart, no words. */
  chalkmenu(x) {
    const chalk = 0xeef0ea, wood = 0x8a5a32, light = 0xa87448;
    line(x, 2, x - 6, 5, 0x5a4a3a); line(x, 2, x + 6, 5, 0x5a4a3a); dot(x, 2, 0x7a7f88);
    fill(x - 12, 5, 25, 22, wood); fill(x - 12, 5, 25, 1, light); fill(x - 12, 26, 25, 1, 0x5a3a24);
    fill(x - 11, 6, 23, 19, 0x2e3a33); fill(x - 11, 6, 23, 1, 0x3a4a40);
    // The cup, and its steam
    fill(x - 9, 12, 6, 1, chalk, 0.85); dot(x - 9, 13, chalk, 0.85); dot(x - 4, 13, chalk, 0.85); dot(x - 9, 14, chalk, 0.85); dot(x - 4, 14, chalk, 0.85);
    fill(x - 8, 15, 4, 1, chalk, 0.85); dot(x - 3, 13, chalk, 0.7); fill(x - 10, 16, 8, 1, chalk, 0.6);
    for (const [dx, dy] of [[-8, 10], [-7, 9], [-7, 8], [-6, 10], [-5, 9], [-5, 8]]) dot(x + dx, dy, chalk, 0.55);
    // The leaf, green chalk, its vein
    const green = 0x9ad0a0;
    for (const [dx, dy] of [[4, 12], [5, 11], [6, 10], [7, 9], [8, 9], [9, 10], [9, 11], [8, 12], [7, 13], [6, 13], [5, 13]]) dot(x + dx, dy, green, 0.85);
    line(x + 4, 13, x + 8, 10, green, 0.5); dot(x + 3, 14, green, 0.7);
    // Squiggles and dots, pink and yellow, and a little heart
    for (let k = 0; k < 17; k++) dot(x - 9 + k, 20 + (Math.floor(k / 2) % 2), 0xf0a8b8, 0.8);
    for (const [dx, dy] of [[-1, 9], [1, 11], [-2, 15], [10, 15]]) dot(x + dx, dy, 0xf2d35b, 0.75);
    dot(x + 6, 22, 0xf0a8b8); dot(x + 8, 22, 0xf0a8b8); fill(x + 6, 23, 3, 1, 0xf0a8b8); dot(x + 7, 24, 0xf0a8b8);
    fill(x - 9, 23, 6, 1, chalk, 0.45);
    // The tray, a stick of chalk and the duster
    fill(x - 11, 27, 23, 1, light); fill(x - 8, 26, 3, 1, chalk); fill(x + 5, 26, 4, 1, 0x8a8f9c);
  },
  /** Pendant lamps. Three warm lamps hung in a row on their cords, terracotta, cream and mustard; they glow at night. */
  pendantlamps(x) {
    const shades = [0xc8643a, 0xe8dcc4, 0xd89a3a], night = isNight;
    [-13, 0, 13].forEach((dx, k) => {
      const lx = x + dx, drop = k === 1 ? 3 : 0, flick = candlelight(k + x * 0.05);
      if (night) {
        glowAt(lx, 16 + drop, 10, 9, 0xffc070, 0.16 * flick);
        for (let r = 0; r < 14 - drop; r++) fill(lx - 1 - Math.floor(r / 3), 14 + drop + r, 3 + Math.floor(r / 3) * 2, 1, 0xffe0a0, 0.05);
      } else glowAt(lx, 14 + drop, 5, 4, 0xffc070, 0.06 * flick);
      fill(lx - 1, 0, 3, 1, 0x2a2a2e);
      fill(lx, 1, 1, 8 + drop, 0x2a2a2e);
      fill(lx - 1, 9 + drop, 3, 1, shades[k]); fill(lx - 2, 10 + drop, 5, 1, shades[k]); fill(lx - 3, 11 + drop, 7, 2, shades[k]);
      fill(lx - 3, 11 + drop, 1, 2, WHITE, 0.25); fill(lx - 3, 13 + drop, 7, 1, shaded(shades[k], 0.75));
      fill(lx - 1, 14 + drop, 3, 1, 0xfff0c0, night ? 1 : 0.85); dot(lx, 14 + drop, WHITE, night ? flick : 0.6);
    });
  },
  /**
   * Wanted posters. Two of them, tacked up: a goat with a cactus in its mouth and a raccoon in its mask, a band of
   * ink over each and a little star under, no words. Now and then a corner of one lifts in a draught.
   */
  wanted(x) {
    const paper = 0xe8d4a0, edge = 0xc8a870, ink = 0x5a3a1a, frameIn = 0xf2e2bc;
    for (const l of [x - 15, x + 2]) {
      fill(l + 1, 7, 13, 18, 0x000000, 0.12);
      fill(l, 6, 13, 18, paper); fill(l, 6, 13, 1, edge); fill(l, 23, 13, 1, edge); fill(l + 12, 6, 1, 18, edge);
      fill(l + 2, 8, 9, 2, ink); fill(l + 3, 9, 7, 1, 0x7a5a3a);
      fill(l + 2, 11, 9, 9, ink, 0.55); fill(l + 3, 12, 7, 7, frameIn);
      dot(l + 6, 21, ink); dot(l + 5, 22, ink, 0.7); dot(l + 7, 22, ink, 0.7); fill(l + 3, 21, 1, 1, ink, 0.5); fill(l + 9, 21, 1, 1, ink, 0.5);
      dot(l + 6, 6, 0x7a7f88);
    }
    // The goat: white, horned, bearded, a cactus in its mouth
    const g = x - 15;
    fill(g + 4, 14, 5, 4, 0xf4f1ea); fill(g + 5, 18, 3, 1, 0xf4f1ea); dot(g + 6, 19, 0xc8c0b0);
    dot(g + 4, 13, 0x7a6e5a); dot(g + 8, 13, 0x7a6e5a); dot(g + 3, 12, 0x7a6e5a); dot(g + 9, 12, 0x7a6e5a);
    dot(g + 3, 15, 0xf4f1ea); dot(g + 9, 15, 0xf4f1ea);
    dot(g + 5, 15, 0x1a1a1f); dot(g + 7, 15, 0x1a1a1f); dot(g + 5, 14, 0x8a8070); dot(g + 7, 14, 0x8a8070);
    fill(g + 3, 17, 3, 1, 0x5a9a4a); dot(g + 3, 16, 0x5a9a4a); dot(g + 4, 18, 0x3e7a34);
    // The raccoon: grey, its mask across its eyes, white muzzle, striped ear tips
    const r = x + 2;
    fill(r + 3, 14, 7, 4, 0x8a8f9c); fill(r + 4, 13, 5, 1, 0x8a8f9c); dot(r + 3, 13, 0x3a3d47); dot(r + 9, 13, 0x3a3d47);
    fill(r + 3, 15, 7, 1, 0x2a2a2e); dot(r + 5, 15, WHITE); dot(r + 7, 15, WHITE);
    fill(r + 5, 17, 3, 1, 0xeae6dc); dot(r + 6, 17, 0x1a1a1f); dot(r + 6, 18, 0xeae6dc);
    // A draught lifting the raccoon's corner now and then
    const t = (now + 5) % 13;
    if (t < 1.4) {
      const lift = Math.floor(now * 7) % 2 ? 2 : 1;
      for (let k = 0; k < lift + 1; k++) fill(r + 12 - k, 23 - lift + k, k + 1, 1, edge);
    }
  },
  /**
   * Countdown clock. A round party clock in a gold rim, a ring of little lights round its face: from 23:00 they light
   * one by one as the hour runs out, the newest blinking; at 00:00 they all flash for a minute; otherwise it just
   * keeps the time.
   */
  countdown(x) {
    const cy = 16, gold = 0xe8b84a, goldDark = 0xb8862e, face = 0x1d2240;
    const counting = hour === 23, striking = hour === 0 && minute === 0;
    const lit = counting ? Math.floor((minute + 1) / 5) : striking ? 12 : 0, flash = striking && Math.floor(now * 4) % 2 === 0;
    line(x, 2, x - 4, 6, 0x5a4a3a); line(x, 2, x + 4, 6, 0x5a4a3a); dot(x, 2, 0x7a7f88);
    if (striking) glowAt(x, cy, 13, 12, 0xffe08a, flash ? 0.22 : 0.1);
    disc(x, cy, 10.5, goldDark); disc(x, cy, 9.6, gold); disc(x, cy, 7.6, flash ? 0x3a3f78 : face);
    dot(x - 6, cy - 7, 0xfff0b8); dot(x - 7, cy - 6, 0xfff0b8);
    const colours = [0xfff0b8, 0xf28fb0, 0x86d6f0, 0x8ae07a];
    for (let k = 0; k < 12; k++) {
      const a = (k / 12) * Math.PI * 2, lx = Math.round(x + Math.sin(a) * 8.6), ly = Math.round(cy - Math.cos(a) * 8.6);
      if (k < lit) {
        const c = striking ? rainbow((k / 12 + now * 0.7) % 1) : colours[k % 4];
        const blink = (counting && k === lit - 1 && Math.floor(now * 2) % 2) || (striking && !flash && k % 2);
        if (!blink) { fill(lx - 1, ly - 1, 3, 3, c, 0.3); fill(lx, ly, 1, 1, WHITE); dot(lx - 1, ly, c); dot(lx + 1, ly, c); dot(lx, ly - 1, c); dot(lx, ly + 1, c); }
        else dot(lx, ly, shaded(c, 0.5));
      } else dot(lx, ly, counting ? 0x4a3a1e : goldDark);
    }
    for (const [dx, dy] of [[0, -6], [6, 0], [0, 6], [-6, 0]]) dot(x + dx, cy + dy, 0x6a7090);
    if (striking && flash) {
      for (const [dx, dy] of [[-12, -9], [12, -9], [-13, 2], [13, 2], [-10, 11], [10, 11]]) { dot(x + dx, cy + dy, 0xfff0b8); dot(x + dx + Math.sign(dx), cy + dy, 0xfff0b8, 0.5); }
    }
    const hand = (turns, length, colour) => {
      for (let step = 1; step <= length; step++) {
        const a = turns * 2 * Math.PI;
        dot(x + Math.round(Math.sin(a) * step), cy - Math.round(Math.cos(a) * step), colour);
      }
    };
    hand(minute / 60, 5, WHITE);
    hand(((hour % 12) + minute / 60) / 12, 3, gold);
    dot(x, cy, gold);
    // A bow of ribbon at its foot
    dot(x - 2, cy + 10, 0xe0508a); dot(x + 2, cy + 10, 0xe0508a); dot(x - 1, cy + 11, 0xe0508a); dot(x + 1, cy + 11, 0xe0508a);
    dot(x, cy + 10, 0xb83a6a); dot(x - 2, cy + 12, 0xe0508a); dot(x + 2, cy + 12, 0xe0508a);
  },
  /**
   * Streamers and stars. Two swags of twisted paper streamers between gold rosettes, in gold, silver and the agents'
   * colours, curls hanging from each rosette, and a garland of little stars below; now and then a draught sways them.
   */
  streamers(x) {
    const t = (now + x * 0.11) % 14, sway = t < 2.4 ? Math.sin((t / 2.4) * Math.PI * 3) * (1 - t / 2.4) * 1.6 : 0;
    const gold = 0xf2c94c, silver = 0xc8ccd4, colours = [gold, silver, 0xd97757, 0x10a37f, 0x4c8bf5];
    const tacks = [x - 19, x, x + 19];
    // The garland of stars, on its thread, behind the streamers
    const garland = (f) => 6 + Math.round(Math.sin(f * Math.PI) * (14 + sway * 1.5));
    for (let px = x - 19; px <= x + 19; px++) dot(px, garland((px - x + 19) / 38), 0x8a8f9c, 0.6);
    for (let i = 0; i < 9; i++) {
      const f = (i + 0.5) / 9, sx = Math.round(x - 19 + f * 38), sy = garland(f), c = i % 2 ? silver : gold;
      dot(sx, sy, lighter(c)); dot(sx - 1, sy, c); dot(sx + 1, sy, c); dot(sx, sy - 1, c); dot(sx, sy + 1, c);
    }
    // The streamers: two layers in each swag, twisted
    for (let k = 0; k < 2; k++) {
      const a = tacks[k], b = tacks[k + 1];
      [[colours[2 + k * 2], 8], [k ? silver : gold, 6]].forEach(([c, sag], layer) => {
        for (let px = a; px <= b; px++) {
          const f = (px - a) / (b - a), py = 4 + Math.round(Math.sin(f * Math.PI) * (sag + sway * (layer + 1) * 0.6));
          dot(px, py, (px + layer) % 3 === 0 ? shaded(c, 0.72) : c);
          if ((px + layer) % 3 === 1) dot(px, py + 1, shaded(c, 0.85));
        }
      });
    }
    // Curls hanging from each rosette, and the rosettes
    tacks.forEach((tx, i) => {
      const c = colours[[3, 2, 4][i]], long = i === 1 ? 7 : 11;
      for (let r = 0; r < long; r++) dot(tx + Math.round(Math.sin(r * 0.9 + sway * 1.5) * 1.2), 6 + r, r % 3 ? c : shaded(c, 0.7));
      fill(tx - 1, 3, 3, 3, gold); dot(tx, 4, 0xfff0b8); dot(tx - 1, 3, 0xb8862e); dot(tx + 1, 5, 0xb8862e);
    });
  },
  /**
   * Balloons. A bunch of five on their strings, tied to a hook with a curl of ribbon: gold, silver and the agents'
   * colours. Now and then one bobs up on its string and settles.
   */
  balloons(x) {
    const hx = x + 1, hy = 25;
    const set = [[-6, 10, 0xf2c94c], [6, 9, 0x4c8bf5], [0, 6, 0xd97757], [-3, 14, 0x10a37f], [4, 15, 0xc8ccd4]];
    const which = Math.floor((now + x * 0.07) / 9) % set.length, t = (now + x * 0.07) % 9;
    const bob = t < 1.6 ? Math.round(Math.sin((t / 1.6) * Math.PI) * 2) : 0;
    const at = set.map(([dx, cy, c], k) => [x + dx, cy - (k === which ? bob : 0), c]);
    for (const [bx, by] of at) line(hx, hy, bx, by + 4, 0xd8d8e0, 0.6);
    for (const [bx, by, c] of at) {
      fill(bx - 1, by - 3, 3, 1, c); fill(bx - 2, by - 2, 5, 1, c); fill(bx - 3, by - 1, 7, 3, c);
      fill(bx - 2, by + 2, 5, 1, c); fill(bx - 1, by + 3, 3, 1, c);
      fill(bx + 1, by + 1, 2, 2, shaded(c, 0.8)); dot(bx + 2, by - 1, shaded(c, 0.85)); dot(bx, by + 3, shaded(c, 0.8));
      dot(bx - 1, by - 2, WHITE, 0.75); dot(bx - 2, by - 1, WHITE, 0.4);
      dot(bx, by + 4, shaded(c, 0.65));
    }
    // The hook, the knot round it and a curl of ribbon below
    fill(hx - 1, hy, 3, 1, 0x7a7f88); dot(hx - 1, hy - 1, 0x7a7f88); dot(hx, hy + 1, 0x5a5f6c);
    for (const [dx, dy] of [[0, 2], [1, 3], [0, 4], [-1, 5], [0, 6]]) dot(hx + dx, hy + dy, 0xe0508a);
  },
  /** Crossed paddles. Two canoe paddles, their blades painted. */
  paddles(x) {
    for (const [dir, tipColour] of [[1, 0xc9443a], [-1, 0x3f6fc4]]) {
      line(x - dir * 9, 6, x + dir * 6, 22, 0xc49a6c);
      fill(x - dir * 9 - 1, 5, 3, 1, 0x8a6a42);
      const bx = x + dir * 7, by = 23;
      fill(bx - 2, by - 2, 5, 6, 0xd8b080); fill(bx - 1, by + 4, 3, 1, 0xd8b080); fill(bx - 2, by + 2, 5, 2, tipColour); dot(bx, by + 4, tipColour);
    }
  },
  /**
   * Cuckoo clock. A carved wooden house with a pitched roof, its face telling the real time, a pendulum ticking and
   * two pine-cone weights on chains. Every 45 s its little door opens and the cuckoo pops out, bobs twice and goes back.
   */
  cuckoo(x) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, roof = 0x4a3020;
    // The chains and pine-cone weights, then the pendulum, behind the case
    for (const [dx, low] of [[-4, 28], [4, 25]]) {
      for (let cy = 22; cy < low; cy++) dot(x + dx, cy, cy % 2 ? 0xc8a050 : 0x8a6a2a);
      fill(x + dx - 1, low, 3, 3, 0x6a4a2e); dot(x + dx, low + 3, 0x6a4a2e); dot(x + dx - 1, low, 0x8a6440); dot(x + dx + 1, low + 1, 0x8a6440);
    }
    const swing = [0, 1, 2, 1, 0, -1, -2, -1][Math.floor(now * 4) % 8];
    line(x, 22, x + swing, 27, 0xc8a050); disc(x + swing, 28, 1.2, 0xe8c060); dot(x + swing - 1, 27, 0xfff0b0);
    // The case and its ledge
    fill(x - 6, 9, 13, 12, wood); fill(x - 6, 9, 1, 12, woodLight); fill(x + 6, 9, 1, 12, woodDark);
    fill(x - 7, 21, 15, 2, woodDark); fill(x - 7, 21, 15, 1, woodLight);
    for (const lx of [-5, -1, 3]) fill(x + lx, 23, 2, 1, woodDark); // the carved fringe under it
    // The roof, its eaves past the case, a little carved bird on top
    for (let row = 0; row < 5; row++) fill(x - 1 - row * 2, 4 + row, 3 + row * 4, 1, row === 4 ? woodDark : roof);
    dot(x - 9, 9, roof); dot(x + 9, 9, roof); fill(x - 1, 2, 3, 2, woodDark); dot(x + 2, 2, woodDark);
    // The face, with the time as it is
    disc(x, 17, 3, 0xf4ecd8); ring(x, 17, 3.2, 0xc8a050, 0.9);
    const t = new Date(), mins = t.getMinutes(), hrs = (t.getHours() % 12) + mins / 60;
    const ma = (mins / 60) * Math.PI * 2 - Math.PI / 2, ha = (hrs / 12) * Math.PI * 2 - Math.PI / 2;
    line(x, 17, x + Math.round(Math.cos(ma) * 2.6), 17 + Math.round(Math.sin(ma) * 2.6), 0x2a2a30);
    line(x, 17, x + Math.round(Math.cos(ha) * 1.6), 17 + Math.round(Math.sin(ha) * 1.6), 0x5a2a20);
    // The little door, and the cuckoo when it's out
    const c = (now + x * 0.3) % 45, out = c < 3;
    if (!out) { fill(x - 1, 10, 3, 2, woodDark); dot(x + 1, 11, 0xc8a050); }
    else {
      fill(x - 1, 10, 3, 2, 0x1a1210); fill(x - 3, 10, 2, 2, woodLight); // the door swung open
      const bob = Math.floor(c * 3) % 2;
      fill(x - 1, 9 + bob, 3, 2, 0x4c8bf5); dot(x + 1, 9 + bob, 0x8fc0e8); dot(x, 9 + bob, PALETTE.eye);
      dot(x + 2, 10 + bob, 0xe8a030); dot(x - 2, 10 + bob, 0x3a6ab8);
    }
  },
  /**
   * Red paper lanterns. Three round lanterns on a cord, gold caps and tassels; every so often a draught sets them
   * swaying, one after the other, and at night they glow.
   */
  lunarlanterns(x) {
    const red = 0xc8302e, dark = 0x8e1e22, light = 0xe85a44, gold = 0xe8b84a, goldDark = 0xb8862e;
    const t = (now + x * 0.13) % 12;
    for (let dx = -17; dx <= 17; dx++) dot(x + dx, 3 + Math.round((1 - (dx / 17) ** 2) * 2), goldDark);
    [[-11, 7], [0, 9], [11, 7]].forEach(([dx, top], k) => {
      const lt = t - k * 0.3, sway = lt > 0 && lt < 2.2 ? Math.round(Math.sin(lt * 6) * (1 - lt / 2.2) * 1.6) : 0;
      const cx = x + dx + sway, glow = candlelight(x + k);
      for (let cy = 4 + (dx ? 1 : 2); cy < top - 1; cy++) dot(x + dx + (cy > top - 3 ? sway : 0), cy, goldDark);
      glowAt(cx, top + 3, 7, 6, 0xff7a4a, (isNight ? 0.16 : 0.05) * glow);
      fill(cx - 1, top - 1, 3, 1, gold);
      fill(cx - 2, top, 5, 1, red); fill(cx - 3, top + 1, 7, 4, red); fill(cx - 2, top + 5, 5, 1, red);
      fill(cx - 3, top + 1, 1, 4, dark); fill(cx + 3, top + 2, 1, 3, dark); dot(cx, top + 2, dark, 0.5); dot(cx, top + 4, dark, 0.5);
      dot(cx - 2, top + 1, light); dot(cx - 1, top + 2, light, isNight ? 0.9 * glow : 0.5);
      if (isNight) fill(cx - 1, top + 2, 3, 2, 0xffb070, 0.35 * glow);
      fill(cx - 1, top + 6, 3, 1, gold);
      dot(cx, top + 7, gold);
      fill(cx - 1, top + 8, 3, 2, red); dot(cx, top + 10, red); dot(cx - 1, top + 9, dark);
    });
  },
  /**
   * Paper screen onto a garden. A sliding shoji half open on a little garden: moss, a stone and a maple, its leaves
   * as the window's season has them; now and then a leaf drifts across.
   */
  paperscreen(x) {
    const wood = 0x3a2a1e, light = 0x5e4632, tones = leafTones();
    const l = x - 15, t = 5, w = 30, h = 22;
    fill(l - 2, t - 2, w + 4, h + 4, wood); fill(l - 2, t - 2, w + 4, 1, light);
    // The garden through the open half
    skyPane(l, t, 16, 13);
    fill(l, t + 13, 16, h - 13, 0x5a8a42); fill(l, t + 13, 16, 1, 0x6a9a4a);
    for (const [dx, dy] of [[2, 16], [7, 18], [12, 15], [4, 20]]) dot(l + dx, t + dy, 0x4a7a3a);
    fill(l + 10, t + 17, 4, 2, 0x8a8f9c); fill(l + 10, t + 17, 4, 1, 0xa8adb6);
    fill(l + 5, t + 9, 2, 8, 0x4a3020); dot(l + 4, t + 8, 0x4a3020); line(l + 6, t + 10, l + 10, t + 7, 0x4a3020);
    if (tones.leaves) {
      for (const [dx, dy, cw, ch] of [[0, 3, 9, 4], [6, 1, 9, 4], [2, 0, 7, 2], [9, 5, 6, 3]]) fill(l + dx, t + dy, cw, ch, tones.leaves[(dx + dy) % 3]);
      for (const [dx, dy] of [[1, 4], [5, 2], [11, 3], [8, 6], [13, 6], [3, 6]]) dot(l + dx, t + dy, tones.leaves[(dx * 2) % 3]);
      dot(l + 4, t + 3, WHITE, 0.25);
    } else for (const [dx, dy] of [[4, 5], [9, 6], [7, 3], [11, 7]]) dot(l + dx, t + dy, WHITE);
    if (isNight) fill(l, t, 16, h, 0x0a1030, 0.35);
    // A leaf drifting down across the garden now and then
    const lap = (now + x * 0.21) % 14;
    if (tones.leaves && lap < 3.5) {
      const k = lap / 3.5;
      dot(l + 1 + Math.round(k * 13), t + 2 + Math.round(k * 15 + Math.sin(k * 9) * 1.5), tones.fall);
    }
    // The screen slid across the other half, its lattice, its edge and pull
    const sx = l + 14, sw = w - 14;
    fill(sx, t, sw, h, isNight ? 0xf0dcb0 : 0xece4d0);
    for (let gx = sx + 3; gx < sx + sw; gx += 4) fill(gx, t, 1, h, wood, 0.6);
    for (let gy = t + 5; gy < t + h; gy += 5) fill(sx, gy, sw, 1, wood, 0.6);
    fill(sx, t, 2, h, wood); fill(sx, t, 1, h, light); dot(sx + 3, t + 11, 0x1a1a1f);
    fill(l - 2, t + h + 1, w + 4, 1, light);
    if (isNight) glowAt(sx + sw / 2, t + h / 2, 9, 10, 0xffd890, 0.08);
  },
  /**
   * Leafy window. A rough plank-framed window with leaves all round it and a branch across, in the season's colours;
   * now and then the leaves sway, and in summer a bird lands on the branch for a while.
   */
  leafwindow(x) {
    const frame = 0x7a5232, light = 0x9a6a42, dark = 0x4a3020, tones = leafTones();
    const l = x - 13, t = 7, w = 26, h = 18;
    skyPane(l, t, w, h);
    fill(l - 2, t - 2, w + 4, 2, frame); fill(l - 2, t + h, w + 4, 2, frame); fill(l - 2, t, 2, h, frame); fill(l + w, t, 2, h, frame);
    fill(l - 2, t - 2, w + 4, 1, light); fill(l + w / 2, t, 1, h, dark);
    fill(l - 3, t + h + 2, w + 6, 1, dark);
    // The branch across, and a twig
    line(l - 4, t + 12, l + w + 3, t + 6, 0x5a3b25); line(l - 4, t + 13, l + w + 3, t + 7, 0x4a3020);
    line(l + 8, t + 10, l + 11, t + 6, 0x5a3b25);
    // In summer a bird flies in, sits on the branch looking about, and flies off
    const bird = (now + x * 0.17) % 26;
    if (tones.summer && bird < 7) {
      const by0 = t + 6, bx0 = l + 17;
      const fly = bird < 1 ? 1 - bird : bird > 6 ? bird - 6 : 0;
      const bx = bx0 + Math.round(fly * 10), by = by0 - Math.round(fly * 8);
      const c = 0x6a4a3a;
      fill(bx, by, 3, 2, c); dot(bx + (Math.floor(bird * 1.3) % 3 ? 3 : -1), by, c); dot(bx + 1, by + 2, 0xd8783a);
      if (fly) { dot(bx - 1, by - 1, c); dot(bx + 3, by - 1, c); } else dot(bx + 1, by - 1, 0xe8a060, 0.8);
    }
    // Leaves all round, swaying now and then
    const t0 = (now + x * 0.23) % 11, sway = t0 < 1.6 ? Math.round(Math.sin(t0 * 7) * (1 - t0 / 1.6) * 1.5) : 0;
    const clumps = [[-5, -4], [0, -5], [6, -4], [12, -5], [18, -4], [24, -4], [-6, 2], [-6, 9], [-5, 15], [26, 0], [26, 8], [25, 15],
      [3, 19], [20, 19], [-3, 19], [12, 5], [16, 2]];
    clumps.forEach(([dx, dy], k) => {
      const cx = l + dx + (dy < 0 ? sway : 0), cy = t + dy;
      if (!tones.leaves) { if (k % 2 === 0) { line(cx, cy + 2, cx + 3, cy, 0x4a3020); dot(cx + 1, cy, WHITE); dot(cx + 2, cy - 1, WHITE); } return; }
      const a = tones.leaves[k % 3], b = tones.leaves[(k + 1) % 3];
      fill(cx, cy + 1, 5, 2, a); fill(cx + 1, cy, 3, 4, a); dot(cx + 1, cy + 1, b); dot(cx + 3, cy + 2, shaded(a, 0.75));
    });
    if (!tones.leaves) fill(l - 3, t - 3, w + 6, 1, WHITE);
  },
  /** Rope ladder. Hanging down the wall from a hatch in the ceiling, its wooden rungs; now and then it swings a little. */
  ropeladder(x) {
    const rope = 0xc8a868, twist = 0x9a7a48, rung = 0x8a5a32, rungLight = 0xa87448;
    // The hatch, open, a little light from the room above
    fill(x - 8, 0, 17, 4, 0x5a3e24); fill(x - 6, 0, 13, 3, 0x1a1410); fill(x - 6, 0, 13, 1, 0xffd890, isNight ? 0.4 : 0.15);
    fill(x - 8, 4, 17, 1, 0x3a2616);
    const t = (now + x * 0.31) % 13, swing = t < 3 ? Math.sin(t * 4) * (1 - t / 3) * 2 : 0;
    const at = (y) => Math.round(swing * ((y - 3) / 34));
    for (let y = 3; y < 37; y++) for (const side of [-4, 4]) dot(x + side + at(y), y, y % 2 ? rope : twist);
    for (let y = 7; y < 37; y += 5) {
      const dx = at(y);
      fill(x - 4 + dx, y, 9, 1, rungLight); fill(x - 4 + dx, y + 1, 9, 1, rung);
      dot(x - 4 + dx, y - 1, twist); dot(x + 4 + dx, y - 1, twist);
    }
    fill(x - 5 + at(37), 37, 3, 1, rope); fill(x + 3 + at(37), 37, 3, 1, rope);
  },
  /** A string of fairy lights draped across the wall with three polaroids pegged along it; one photo swings a little now and then, and at night the bulbs glow warm, one twinkling at a time. */
  photostring(x) {
    const cord = 0x6a5a48, peg = 0xc8a050, bulb = 0xf4d27a, white = 0xf4f0e6;
    const sag = (i) => 6 + Math.round(4 * (1 - ((i - 12) / 12) ** 2));
    // The cord, caught on two nails
    for (let i = 0; i <= 24; i++) dot(x - 12 + i, sag(i), cord);
    dot(x - 13, 5, 0x8a8f9c); dot(x + 13, 5, 0x8a8f9c);
    // The bulbs, in the gaps between the photos
    const lit = Math.floor(now * 0.7 + x * 0.1) % 6, bulbs = [1, 3, 8, 16, 21, 23];
    bulbs.forEach((i, n) => {
      const bx = x - 12 + i, by = sag(i) + 1, bright = n === lit;
      dot(bx, by, bright && isNight ? 0xfff4c0 : bulb); dot(bx, by + 1, shaded(bulb, 0.8));
      if (isNight) glowLate(() => { if (bright) glowAt(bx, by, 2.5, 2.5, 0xffd070, 0.25); dot(bx, by, 0xfff4c0, bright ? 1 : 0.75); });
    });
    // The polaroids: one of them swings, its lower half a pixel either way
    const pics = [[5, 0xe89a8a, 0x6aa8c8], [12, 0x8ac08a, 0xf2d35b], [19, 0x6a9ad8, 0xe8a060]];
    const swinger = Math.floor(x / 7) % 3, t = (now + x * 0.3) % 24;
    pics.forEach(([i, sky, ground], n) => {
      const px = x - 12 + i - 2, py = sag(i) + 1;
      const sw = n === swinger && t < 3 ? [0, 1, 0, -1][Math.floor(t * 4) % 4] : 0;
      dot(px + 2, py, peg); dot(px + 3, py, peg);
      fill(px, py + 1, 5, 3, white); fill(px + sw, py + 4, 5, 4, white);
      fill(px + 1, py + 2, 3, 2, sky); fill(px + 1 + sw, py + 4, 3, 2, ground);
      fill(px + 4 + sw, py + 4, 1, 4, 0xc8c4ba); fill(px + sw, py + 7, 5, 1, 0xc8c4ba);
      dot(px + 1, py + 1, WHITE, 0.7);
    });
  },

  /** A dartboard on a round cork backing: every 20 s a dart thunks in and its flight wobbles; after three darts they are pulled out and it starts again. */
  dartboard(x) {
    const cork = 0xb88a54, corkLight = 0xd0a46c, corkDark = 0x8a6238, cy = 17;
    const red = 0xc83a30, green = 0x3a8a4a, cream = 0xe8dcb4, ink = 0x2a2d36, steel = 0xa8adb6;
    // The cork backing, lit from the top left
    disc(x, cy, 9.5, corkDark); disc(x - 1, cy - 1, 8.5, corkLight); disc(x, cy, 8.5, cork);
    // The board: a dark number ring, a red double ring, a green treble ring, eight dark and cream wedges, the bull
    disc(x, cy, 7.5, ink);
    for (let dy = -7; dy <= 7; dy++) {
      for (let dx = -7; dx <= 7; dx++) {
        const r = Math.hypot(dx, dy);
        if (r > 6.9) continue;
        const odd = Math.floor(((Math.atan2(dy, dx) + Math.PI * 2 + Math.PI / 8) % (Math.PI * 2)) / (Math.PI / 4)) % 2 === 1;
        const colour = r < 0.9 ? red : r < 1.9 ? green : r > 5.8 ? red : r > 3.2 && r < 4.2 ? green : odd ? cream : ink;
        dot(x + dx, cy + dy, colour);
      }
    }
    dot(x - 4, cy - 5, WHITE, 0.35);
    // The darts: one lands every 20 s, each stuck at a slant with a blue flight; after the third they all come out
    const cycle = (now + x * 0.7) % 80, landed = Math.floor(cycle / 20), since = cycle % 20;
    const spots = [[2, -3], [-3, 1], [1, 3]];
    for (let i = 0; i < landed; i++) {
      const wobble = i === landed - 1 && since < 0.6 ? Math.floor(since * 6) % 2 : 0;
      const dx = x + spots[i][0], dy = cy + spots[i][1];
      dot(dx, dy, steel); dot(dx + 1, dy - 1, 0xe8e8ee);                          // tip and barrel
      fill(dx + 2 + wobble, dy - 2, 2, 1, 0x4c8bf5); dot(dx + 2 + wobble, dy - 3, 0x7aaaf8);   // the flight
    }
  },

  /** A split-flap clock: four black flaps with pale digits telling the real time; when a digit changes, its flap falls in two quick half-frames. */
  splitflap(x) {
    const body = 0x1e2026, bodyLight = 0x3a3d46, bodyDark = 0x101216, flap = 0x2a2d35, white = 0xf2efe6, white2 = 0xb8b6ae;
    const GLYPHS = ['111101101101111', '010110010010111', '111001111100111', '111001111001111', '101101111001001',
      '111100111001111', '111100111101111', '111001001001001', '111101111101111', '111101111001111'];
    // The case, lit from the top left, with a shadow strip under it
    fill(x - 10, 9, 21, 15, body); fill(x - 10, 9, 21, 1, bodyLight); fill(x - 10, 9, 1, 15, bodyLight);
    fill(x + 10, 10, 1, 14, bodyDark); fill(x - 9, 23, 20, 1, bodyDark); fill(x - 9, 24, 21, 1, SHADOW);
    // The time now and a second ago: a digit that changed in the last 0.6 s is mid-flip
    const d = new Date(), prev = new Date(d.getTime() - 1000);
    const four = (t) => [Math.floor(t.getHours() / 10), t.getHours() % 10, Math.floor(t.getMinutes() / 10), t.getMinutes() % 10];
    const digits = four(d), was = four(prev), since = d.getSeconds() === 0 ? d.getMilliseconds() / 1000 : 9;
    // Each digit 3 x 5 on a 4 x 11 flap, set towards the colon; the top half brighter than the bottom, a dark split between
    const glyph = (cx, n, rows, colourOf) => {
      for (let r of rows) for (let c = 0; c < 3; c++) if (GLYPHS[n][r * 3 + c] === '1') dot(cx + c + (cx > x ? 1 : 0), 14 + r, colourOf(r));
    };
    [x - 9, x - 4, x + 1, x + 6].forEach((cx, i) => {
      fill(cx, 11, 4, 11, flap); fill(cx, 11, 4, 1, bodyLight, 0.5);
      const n = digits[i], flipping = since < 0.6 && n !== was[i], shade = (r) => (r < 2 ? white : white2);
      if (!flipping) glyph(cx, n, [0, 1, 2, 3, 4], shade);
      else {
        // The new digit's top above; the old digit's bottom below until the flap lands, then the new one's
        const landed = since >= 0.3;
        glyph(cx, n, [0, 1], shade); glyph(cx, landed ? n : was[i], [3, 4], shade);
        fill(cx, landed ? 17 : 15, 4, 1, white2, 0.5);           // the falling flap's edge
      }
      fill(cx, 16, 4, 1, bodyDark, 0.6);                          // the split line
    });
    // The colon, blinking once a second
    const on = Math.floor(now) % 2 === 0;
    dot(x, 14, on ? white : white2, on ? 1 : 0.4); dot(x, 18, on ? white : white2, on ? 1 : 0.4);
    dot(x - 9, 10, WHITE, 0.25);
  },
  /** A periodic-table poster pinned to the wall: the groups' coloured blocks in their grid, with the two loose rows below; a highlight walks to a new element every few seconds. */
  periodictable(x) {
    const paper = PALETTE.paper, paperDark = 0xc8c2b4, ink = 0x3a3d48;
    const left = x - 9, top = 9;
    // The poster, a title stroke and two pins
    fill(x - 10, 5, 21, 25, paper); fill(x - 10, 5, 21, 1, WHITE, 0.5); fill(x + 10, 6, 1, 24, paperDark); fill(x - 9, 29, 20, 1, paperDark);
    fill(x - 4, 7, 9, 1, ink); fill(x - 7, 7, 2, 1, 0xd8402a);
    dot(x - 9, 6, 0xd8402a); dot(x + 9, 6, 0x4c8bf5);
    // Which columns each of the seven rows has, and each column's group colour
    const group = (row, col) => (row === 0 && col === 0 ? 0x6ab070 : col === 0 ? 0xd8402a : col === 1 ? 0xe8782e : col < 12 ? 0x7aa8f0
      : col < 14 ? 0xf2d35b : col < 17 ? 0x6ab070 : 0xb08ae0);
    const has = (row, col) => (row === 0 ? col === 0 || col === 17 : row < 3 ? col < 2 || col > 11 : true);
    const cells = [];
    for (let row = 0; row < 7; row++) for (let col = 0; col < 18; col++) if (has(row, col)) cells.push([col, row, group(row, col)]);
    // The two loose rows below, set in from the left
    for (let row = 0; row < 2; row++) for (let col = 3; col < 17; col++) cells.push([col, 7.5 + row, row ? 0xf28fb0 : 0x5ac8c0]);
    for (const [col, row, colour] of cells) {
      const cy = top + Math.floor(row * 2);
      dot(left + col, cy, colour); dot(left + col, cy + 1, shaded(colour, 0.85));
    }
    // The highlight: a different element every 3 s, a different walk per poster
    const pick = cells[hash(Math.floor((now + x * 0.4) / 3), x) % cells.length];
    const hy = top + Math.floor(pick[1] * 2);
    fill(left + pick[0], hy, 1, 2, WHITE); dot(left + pick[0], hy - 1, WHITE, 0.35);
  },
  /** A wall shelf of herb pots (basil, chives with a flower, a trailing mint) under a hanging grow-light bar that glows pink-violet at night; now and then a new basil leaf unfurls. */
  growshelf(x) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, steel = 0x6e727c, steelDark = 0x4a4e58;
    const pot = PALETTE.pot, potLight = 0xc07a52, potDark = 0x7a4630, leaf = PALETTE.leaf, leafDark = PALETTE.leafDark;
    const leafLight = 0x7ac070, mint = 0x5ab868, mintDark = 0x3e8a4e, glow = 0xe890ff;
    // The grow-light bar on two thin chains, lit underneath
    line(x - 8, 2, x - 8, 7, steel, 0.8); line(x + 8, 2, x + 8, 7, steel, 0.8);
    fill(x - 10, 7, 21, 2, steelDark); fill(x - 10, 7, 21, 1, steel); fill(x - 9, 9, 19, 1, glow, isNight ? 1 : 0.7);
    // The shelf on two brackets
    fill(x - 10, 24, 21, 2, wood); fill(x - 10, 24, 21, 1, woodLight); fill(x - 10, 26, 21, 1, woodDark);
    fill(x - 7, 27, 1, 2, steelDark); fill(x + 6, 27, 1, 2, steelDark);
    // Three clay pots
    for (const px of [x - 9, x - 2, x + 5]) {
      fill(px - 1, 19, 6, 1, potLight); fill(px, 20, 4, 3, pot); fill(px + 1, 23, 2, 1, potDark);
      fill(px, 20, 1, 3, potLight); fill(px + 3, 20, 1, 3, potDark); fill(px, 20, 4, 1, potDark, 0.5);
    }
    // The basil: broad leaves, and a new one at the top that unfurls now and then
    fill(x - 10, 16, 3, 2, leaf); fill(x - 6, 15, 3, 2, leaf); fill(x - 8, 13, 3, 2, leafLight); fill(x - 8, 17, 3, 2, leafDark);
    dot(x - 7, 18, leafDark); dot(x - 10, 16, leafLight); dot(x - 6, 15, leafLight);
    const u = (now + x * 0.37) % 40, grow = u < 2 ? 0 : u < 4 ? 1 : 2;
    dot(x - 7, 12, leaf); if (grow > 0) dot(x - 6, 11, leafLight); if (grow > 1) { dot(x - 5, 11, leafLight); dot(x - 6, 10, leaf); }
    // The chives: thin spears of two greens, one with a purple flower
    for (const [dx, top, c] of [[-2, 14, leafDark], [-1, 12, leaf], [0, 11, leafDark], [1, 13, leaf], [2, 15, leafDark]]) fill(x + dx, top, 1, 19 - top, c);
    dot(x, 10, 0xb08ae0); dot(x - 1, 10, 0xc8a8f0, 0.7); dot(x, 9, 0xc8a8f0, 0.7);
    // The mint: a bushy top trailing down over the shelf's edge
    fill(x + 4, 16, 6, 3, mint); fill(x + 5, 15, 4, 1, mint); fill(x + 4, 18, 6, 1, mintDark); dot(x + 5, 16, leafLight); dot(x + 8, 17, leafLight);
    for (const [dx, dy] of [[9, 19], [8, 21], [9, 23], [8, 26], [9, 27]]) { dot(x + dx, dy, mint); dot(x + dx, dy + 1, mintDark); }
    // The light: a pale pink wash by day, a pink-violet glow through the dark at night
    if (isNight) glowLate(() => { fill(x - 10, 10, 21, 14, glow, 0.1); fill(x - 9, 9, 19, 1, 0xf8d0ff); });
    else fill(x - 10, 10, 21, 9, glow, 0.04);
  },
  /** A short wooden peg rail with four mismatched mugs hanging by their handles (cream, teal, mustard, red-spotted): every 30 s or so one mug sways a pixel and settles. */
  mugrack(x) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25;
    const mugs = [[-7, 0xeee4c8, 0xcfc4a4], [-2, 0x3a9a9a, 0x2a7676], [3, 0xe0a82e, 0xb08418], [8, 0xe8e0d0, 0xc8bfae]];
    // The rail and its pegs
    fill(x - 10, 10, 22, 3, wood); fill(x - 10, 10, 22, 1, woodLight); fill(x - 10, 12, 22, 1, woodDark);
    fill(x - 10, 10, 1, 3, woodLight); fill(x + 11, 10, 1, 3, woodDark);
    // Which mug sways, and for how long (a 3 s event every 30 s, a different mug each time)
    const cycle = Math.floor(now / 30), pick = Math.imul(cycle + 7, 2654435761) >>> 29 & 3;
    const ph = now % 30, swayStep = ph < 3 ? [1, 1, 0, -1, -1, 0][Math.floor(ph * 2) % 6] : 0;
    mugs.forEach(([mx, c, cd], i) => {
      const cx = x + mx, sw = i === pick ? swayStep : 0;
      fill(cx, 12, 1, 2, woodDark); // peg
      fill(cx - 1 + sw, 14, 4, 6, c); fill(cx - 1 + sw, 14, 1, 6, shaded(c, 1.12)); fill(cx + 2 + sw, 14, 1, 6, cd);
      fill(cx - 1 + sw, 19, 4, 1, cd); fill(cx + sw, 14, 2, 1, shaded(c, 0.9)); // the rim and the base
      dot(cx + 3 + sw, 15, cd); dot(cx + 3 + sw, 16, cd); dot(cx + 3 + sw, 17, cd); // the handle
      if (i === 3) { for (const [dx, dy] of [[0, 16], [1, 15], [0, 18], [1, 17]]) dot(cx + dx + sw, dy, 0xd8402a); } // red spots
      if (i === 0) dot(cx + sw, 15, WHITE, 0.6);
    });
    fill(x - 9, 20, 20, 1, SHADOW);
  },
  /** A pale blue wall-mounted desk fan in a round wire cage with a red ribbon; every 20 s it oscillates left and right in steps for a few seconds, blades blurring and the ribbon fluttering out, then stands still. */
  wallfan(x) {
    const blue = 0x8cb8d8, blueLight = 0xb8dcf0, blueDark = 0x5a86a8, wire = 0xd4e4ee, ribbon = 0xd8402a;
    const t = (now + x * 0.31) % 20, on = t < 4;
    const sway = on ? [0, 1, 2, 1, 0, -1, -2, -1][Math.floor(now * 4) % 8] : 0;
    const cx = x + Math.round(sway / 2), cy = 14;
    // The wall bracket and the neck
    fill(x - 3, 25, 7, 3, blueDark); fill(x - 3, 25, 7, 1, blue); fill(x - 1, 21, 3, 4, blue); fill(x - 1, 21, 1, 4, blueLight);
    // The cage: a wire ring with an inner ring
    disc(cx, cy, 8, 0x3a4a58, 0.25); ring(cx, cy, 8, wire, 0.9); ring(cx, cy, 5, wire, 0.4);
    dot(cx - 8, cy, blueLight); dot(cx, cy - 8, blueLight); dot(cx + 8, cy, blueDark); dot(cx, cy + 8, blueDark);
    // The blades: three still, or a pale blur while it turns
    const blades = (a0) => { for (let k = 0; k < 3; k++) { const an = a0 + (k * Math.PI * 2) / 3; line(cx, cy, cx + Math.round(Math.cos(an) * 5), cy + Math.round(Math.sin(an) * 5), blue); } };
    if (on) { disc(cx, cy, 6, blueLight, 0.35); blades(Math.floor(now * 6) % 3 * 0.7); } else blades(-1.2);
    disc(cx, cy, 1.6, 0xc8402a); dot(cx - 1, cy - 1, 0xf08070);
    // The ribbon on the cage rim, streaming out when the fan is on
    const f = on ? Math.floor(now * 6) % 2 : 0;
    dot(cx + 7, cy - 6, ribbon); dot(cx + 8, cy - 5, ribbon);
    if (on) { fill(cx + 9, cy - 5 + f, 2, 1, ribbon); dot(cx + 11, cy - 4 - f, ribbon); dot(cx + 12, cy - 5 + f, shaded(ribbon, 0.85)); }
    else { dot(cx + 8, cy - 4, ribbon); dot(cx + 8, cy - 3, ribbon); dot(cx + 9, cy - 2, shaded(ribbon, 0.85)); }
    fill(x - 4, 29, 9, 1, SHADOW);
  },
  /** A big diamond kite pinned to the wall: red and yellow quarters, a cross spar and a ribbon tail with three bows; every 30 s the tail waves once, two frames of a gentle S, as if a draught caught it. */
  kite(x) {
    const red = 0xd8402a, yellow = 0xf2d35b, spar = 0x8a5a32, sparLight = 0xa8703e, string = 0xe8dcc0;
    const top = 3, mid = 12, bottom = 21;
    // The diamond, row by row: quarters swap colour across the cross spar
    for (let r = 0; r <= bottom - top; r++) {
      const y = top + r, w = Math.round((r <= mid - top ? r / (mid - top) : (bottom - top - r) / (bottom - mid)) * 8);
      const upper = y < mid;
      const left = upper ? red : yellow, right = upper ? yellow : red;
      fill(x - w, y, w, 1, left); fill(x + 1, y, w, 1, right);
      if (w > 0) { dot(x - w, y, shaded(left, 1.12)); dot(x + w, y, shaded(right, 0.85)); }
    }
    // The cross spar
    fill(x, top, 1, bottom - top + 1, spar); fill(x - 8, mid, 17, 1, spar);
    dot(x, top, sparLight); dot(x - 8, mid, sparLight); dot(x, mid, sparLight);
    // The pin at the top
    dot(x, top - 1, 0xc8a050); dot(x, top - 1, WHITE, 0.4);
    // The tail: a gentle S, waving once every 30 s for about 2 s
    const t = (now + x * 0.31) % 30, waving = t < 2 ? 1 + (Math.floor(t * 3) % 2) : 0;
    const phase = [0, 1.4, 3.2][waving], bows = [[0xd8402a, 3], [0x4c8bf5, 7], [0x4f9a5a, 11]];
    let prev = x;
    for (let i = 0; i < 14; i++) {
      const off = Math.round(Math.sin(i * 0.55 + phase) * (1 + i * 0.12)), y = bottom + 1 + i;
      fill(Math.min(prev, x + off), y, Math.abs(x + off - prev) + 1, 1, string);
      prev = x + off;
    }
    for (const [colour, i] of bows) {
      const off = Math.round(Math.sin(i * 0.55 + phase) * (1 + i * 0.12)), y = bottom + 1 + i;
      dot(x + off - 2, y - 1, colour); dot(x + off + 2, y - 1, colour); dot(x + off - 2, y + 1, shaded(colour, 0.85)); dot(x + off + 2, y + 1, shaded(colour, 0.85));
      fill(x + off - 1, y, 3, 1, colour); dot(x + off, y, shaded(colour, 1.12));
    }
  },
  /** A wind chime on a wall bracket: a round wooden top, five brass tubes of different lengths and a wooden sail; every 25 s the sail swings and the tubes sway out of step for 3 s, with a glint running along them. */
  windchime(x) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, cord = 0xe8dcc0;
    const t = (now + x * 0.31) % 25, active = t < 3;
    // The bracket on the wall and the cord down to the top
    fill(x - 9, 2, 10, 2, woodDark); fill(x - 9, 2, 10, 1, wood); fill(x - 9, 1, 2, 6, woodDark); dot(x - 9, 1, woodLight);
    dot(x, 4, cord); dot(x, 5, cord);
    // The round wooden top
    fill(x - 6, 6, 13, 1, woodLight); fill(x - 7, 7, 15, 2, wood); fill(x - 6, 9, 13, 1, woodDark); dot(x - 7, 7, woodLight); dot(x + 7, 8, woodDark);
    // Five tubes, of different lengths, swinging out of step
    const lengths = [10, 14, 9, 12, 8], sail = active ? Math.round(Math.sin(t * 5.5) * 2 * (1 - t / 3.4)) : 0;
    lengths.forEach((len, i) => {
      const tx = x - 6 + i * 3, sway = active ? Math.round(Math.sin(t * 5 + i * 1.7) * (1 - t / 3.4) * 1.4) : 0;
      dot(tx + 1, 10, cord);
      fill(tx, 11, 2, 3, brass); fill(tx, 11, 1, 3, brassLight);
      fill(tx + sway, 14, 2, len - 3, brass); fill(tx + sway, 14, 1, len - 3, brassLight); fill(tx + sway + 1, 14 + len - 4, 1, 1, brassDark);
      fill(tx + sway, 10 + len, 2, 1, brassDark);
      dot(tx + sway + 1, 12, brassDark, 0.5);
    });
    // The glint running along the tubes
    if (active) { const g = Math.floor(t * 6); if (g < 5) { const len = lengths[g]; dot(x - 6 + g * 3 + 0, 11 + (g * 2) % 5 + 1, WHITE, 0.9); dot(x - 6 + g * 3, 8 + len, WHITE, 0.5); } }
    // The sail on its cord from the clapper
    const sx = x + sail;
    fill(x, 22, 1, 2, cord); fill(sx - 1, 24, 3, 1, woodLight); fill(sx - 2, 25, 5, 4, wood); fill(sx - 2, 25, 1, 4, woodLight); fill(sx + 2, 25, 1, 4, woodDark); fill(sx - 1, 29, 3, 1, woodDark);
  },
  /** A Claude neon sign: the spark in orange tube over "Claude" in warm cream, on a dark backboard; it hums, and every half minute one letter stutters. */
  claudeneon(x) {
    const board = 0x2b2724, boardLight = 0x3d3833, spark = 0xff7a48, word = 0xffd2a8;
    const t = (now + x * 0.07) % 31, stutter = t > 30.2 ? Math.floor(x + now / 31) % 6 : -1;
    const flick = stutter >= 0 && Math.floor(now * 16) % 2 === 0, hum = 0.9 + 0.1 * Math.sin(now * 7 + x);
    // The backboard, on two screws
    fill(x - 12, 3, 23, 20, board, 0.9); fill(x - 12, 3, 23, 1, boardLight); fill(x - 12, 22, 23, 1, 0x1e1b19, 0.9);
    dot(x - 11, 4, 0x8a8f9c); dot(x + 9, 4, 0x8a8f9c);
    const letters = {
      C: ['.11', '1..', '1..', '1..', '.11'], l: ['1', '1', '1', '1', '1'], a: ['...', '.11', '1.1', '1.1', '.11'],
      u: ['...', '1.1', '1.1', '1.1', '.11'], d: ['..1', '.11', '1.1', '1.1', '.11'], e: ['...', '.1.', '111', '1..', '.11'],
    };
    /** The lit tubes: the spark with a halo round it, then the word, one letter dark while it stutters. */
    const lights = () => {
      glowAt(x - 1, 12, 13, 10, 0xff8a50, (isNight ? 0.1 : 0.04) * hum);
      const rays = [[0, 0], [1, 0], [2, 0], [3, 0], [-1, 0], [-2, 0], [-3, 0], [0, 1], [0, 2], [0, 3], [0, -1], [0, -2], [0, -3],
        [1, 1], [2, 2], [-1, 1], [-2, 2], [1, -1], [2, -2], [-1, -1], [-2, -2]];
      for (const [dx, dy] of rays) fill(x + dx - 2, 9 + dy - 1, 3, 3, spark, 0.08 * hum);
      for (const [dx, dy] of rays) dot(x - 1 + dx, 9 + dy, lighter(spark));
      dot(x - 1, 9, 0xffe4cc);
      let at = x - 11; // a pixel left of centre, so a window on the right covers only the board's edge
      [...'Claude'].forEach((c, n) => {
        const off = n === stutter && flick;
        letters[c].forEach((row, r) => [...row].forEach((on, k) => { if (on === '1') dot(at + k, 15 + r, off ? shaded(word, 0.35) : word); }));
        at += letters[c][0].length + 1;
      });
    };
    if (isNight) glowLate(lights); else lights();
  },
  /** An Opus, Sonnet and Haiku poster: three sparks, big to small, each by its name's line, on cream in a charcoal frame; every 30 s they twinkle in turn, top to bottom. */
  modelposter(x) {
    const frame = 0x3a3631, frameLight = 0x57514a, frameDark = 0x26231f, paper = 0xf2ebdc, ink = 0x5a524a, faint = 0xb8ac98;
    // The frame and the paper
    fill(x - 8, 3, 17, 25, frame); fill(x - 8, 3, 17, 1, frameLight); fill(x - 8, 3, 1, 25, frameLight);
    fill(x + 8, 4, 1, 24, frameDark); fill(x - 7, 27, 16, 1, frameDark);
    fill(x - 7, 4, 15, 23, paper); fill(x - 7, 4, 15, 1, 0xfffaf0);
    fill(x - 8, 28, 17, 1, SHADOW);
    // Opus, Sonnet and Haiku: a spark each, and its name in a line of type
    const t = (now + x * 0.07) % 30, lit = t < 3 ? Math.floor(t) : -1;
    const rows = [[9, 3, 0xb5573a, 5], [16, 2, 0xd97757, 4], [22, 1, 0xe8a07a, 3]];
    rows.forEach(([cy, r, colour, words], k) => {
      const grow = k === lit && Math.floor(now * 6) % 2 === 0 ? 1 : 0;
      if (r > 1) claudeSpark(x - 3, cy, r, colour, 1, grow);
      else { for (const [dx, dy] of [[0, 0], [1, 0], [-1, 0], [0, 1], [0, -1]]) dot(x - 3 + dx, cy + dy, colour); if (grow) { dot(x - 5, cy, colour); dot(x - 1, cy, colour); } }
      fill(x + 1, cy - 1, words, 1, ink); fill(x + 1, cy + 1, 6, 1, faint);
    });
    // A rule under the header
    fill(x - 6, 13, 13, 1, faint, 0.6); fill(x - 6, 19, 13, 1, faint, 0.6);
  },
  /** A framed "Hello, Claude" print: a little terminal window on a cream mat, a prompt, a line of greeting by the spark and a cursor that blinks. */
  hellocard(x) {
    const frame = 0x8a5a32, frameLight = 0xa8703e, frameDark = 0x5a3b25, mat = 0xf2ebdc;
    const term = 0x2b2724, bar = 0x3d3833, type = 0xf2ebdc, spark = 0xd97757;
    // The frame and its mat
    fill(x - 10, 5, 20, 18, frame); fill(x - 10, 5, 20, 1, frameLight); fill(x - 10, 5, 1, 18, frameLight);
    fill(x + 9, 6, 1, 17, frameDark); fill(x - 9, 22, 19, 1, frameDark);
    fill(x - 9, 6, 18, 16, mat);
    fill(x - 10, 23, 20, 1, SHADOW);
    // The terminal: its title bar with three dots, then its lines
    fill(x - 7, 8, 14, 12, term); fill(x - 7, 8, 14, 2, bar);
    dot(x - 6, 8, 0xd8604a); dot(x - 4, 8, 0xe8b04a); dot(x - 2, 8, 0x6aa85a);
    dot(x - 6, 11, spark); dot(x - 5, 12, spark); dot(x - 6, 13, spark);   // the prompt, >
    fill(x - 3, 12, 5, 1, type);
    for (const [dx, dy] of [[0, 0], [1, 0], [-1, 0], [0, 1], [0, -1]]) dot(x - 5 + dx, 16 + dy, spark);   // the spark, and its reply
    fill(x - 2, 16, 7, 1, type, 0.85); fill(x - 2, 18, 4, 1, type, 0.5);
    // The cursor, blinking
    if (Math.floor(now * 1.6 + x) % 2 === 0) fill(x + 3, 18, 1, 1, type);
  },
  /** A context-window clock: the time in cream digits over a terracotta bar that fills as the hour goes, like a context window; every 20 s its leading edge pulses. */
  contextclock(x) {
    const body = 0x2b2724, bodyLight = 0x433e38, bodyDark = 0x1c1a18, digit = 0xf2ebdc, track = 0x3d3833, bar = 0xd97757, barLight = 0xf0a080;
    // The case, lit from the top left
    fill(x - 10, 7, 20, 15, body); fill(x - 10, 7, 20, 1, bodyLight); fill(x - 10, 7, 1, 15, bodyLight);
    fill(x + 9, 8, 1, 14, bodyDark); fill(x - 9, 21, 19, 1, bodyDark);
    fill(x - 10, 22, 20, 1, SHADOW);
    // The time, hh:mm
    const digits = () => {
      let at = x - 8;
      for (const c of `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`) {
        GLYPHS[c].forEach((row, r) => [...row].forEach((on, k) => { if (on === '1') dot(at + k, 10 + r, digit, c === ':' ? 0.7 : 1); }));
        at += GLYPHS[c][0].length + 1;
      }
    };
    digits();
    // The window filling through the hour, in ticks of a quarter
    const filled = Math.max(1, Math.round((minute / 60) * 17));
    fill(x - 8, 17, 17, 2, track); fill(x - 8, 17, filled, 2, bar); fill(x - 8, 17, filled, 1, barLight, 0.5);
    for (const q of [4, 8, 12]) dot(x - 8 + q, 19, 0x6a625a);
    const t = (now + x * 0.07) % 20;
    if (t < 1.5 && Math.floor(t * 4) % 2 === 0) fill(x - 9 + filled, 17, 1, 2, 0xffe0c8);
    if (isNight) glowLate(() => { digits(); fill(x - 8, 17, filled, 2, bar, 0.8); });
  },
  /** A brass porthole onto murky green water; every 24 s a purple tentacle with pale suckers slides slowly past the glass from right to left. */
  tentacleporthole(x) {
    const cx = x, cy = 16, R = 8, brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a;
    // The brass rim with its bolts
    disc(cx, cy, 11.5, brassDark); disc(cx, cy, 10.5, brass); disc(cx, cy, 9, brassDark);
    for (let a = 0; a < 8; a++) dot(cx + Math.round(Math.cos(a * Math.PI / 4 + 0.4) * 10), cy + Math.round(Math.sin(a * Math.PI / 4 + 0.4) * 10), brassLight);
    for (let row = -R; row <= R; row++) {
      const half = Math.floor(Math.sqrt(R * R - row * row));
      fill(cx - half, cy + row, half * 2 + 1, 1, row < -3 ? 0x2e7a64 : row < 3 ? 0x1f5a4c : 0x123a34);
    }
    const wet = (px, py, colour, alpha = 1) => { if ((px - cx) ** 2 + (py - cy) ** 2 < (R - 0.3) ** 2) dot(px, py, colour, alpha); };
    for (let k = 0; k < 6; k++) wet(cx - 5 + k, cy - 7 + k * 2, 0x9affd0, 0.16);
    // The tentacle passing by
    const u = (now + x * 0.3) % 24;
    if (u < 7) {
      const lead = cx + 10 - Math.round(u / 7 * 22);
      for (let k = 0; k < 14; k++) {
        const px = lead + k, py = cy + 3 + Math.round(Math.sin(k * 0.55 + u * 1.2) * 2) - (k > 9 ? k - 9 : 0);
        for (let t = 0; t < 3; t++) wet(px, py + t, t === 0 ? 0x9a6ac0 : t === 1 ? 0x7a4aa0 : 0x5a3480);
        if (k % 3 === 1) wet(px, py + 3, 0xe8d8f0, 0.9);
      }
    }
    dot(cx - 5, cy - 5, WHITE, 0.6); dot(cx - 4, cy - 6, WHITE, 0.35);
    if (isNight) glowAt(cx, cy, 9, 9, 0x60ffc0, 0.09);
  },
  /** A framed star chart on dark blue: constellation lines that never quite close, an impossible triangle, and now and then one star slips to a new place. */
  starchartodd(x) {
    const wood = 0x5a3b25, woodLight = 0x8a5a32, sky = 0x16264a, line1 = 0xc8a050, starC = 0xfff0b0;
    fill(x - 10, 6, 21, 21, wood); fill(x - 10, 6, 21, 1, woodLight); fill(x - 10, 6, 1, 21, woodLight); fill(x + 10, 6, 1, 21, 0x3a2418); fill(x - 10, 26, 21, 1, 0x3a2418);
    fill(x - 8, 8, 17, 17, sky); fill(x - 8, 8, 17, 1, 0x0e1a38);
    // The rings of the chart
    ellipseRing(x, 16, 7, 7, line1, 0.3); ellipseRing(x, 16, 4, 6, line1, 0.25);
    // An impossible triangle: three bars that meet at the wrong corners
    line(x - 5, 21, x, 11, line1, 0.8); line(x, 11, x + 5, 20, line1, 0.8); line(x + 5, 20, x - 2, 20, line1, 0.8); line(x - 2, 19, x - 6, 22, line1, 0.5);
    // Stars, with one that wanders for 2 s every 25 s
    const t = (now + x * 0.3) % 25, drift = t < 2 ? 1 : 0;
    for (const [sx, sy] of [[-6, 11], [-4, 15], [3, 13], [6, 10], [2, 23], [-7, 19]]) dot(x + sx, sy, starC, 0.75);
    dot(x - 5 + drift * 3, 11 + drift * 2, 0xffffff);
    dot(x + 1, 16, starC); if (Math.floor(now * 2 + x) % 7 === 0) dot(x + 1, 15, starC, 0.6);
    dot(x - 9, 7, 0xe8c878); dot(x + 9, 7, 0xe8c878); dot(x - 9, 25, 0xe8c878); dot(x + 9, 25, 0xe8c878);
    dot(x - 7, 9, WHITE, 0.3);
    if (isNight) glowAt(x, 16, 10, 10, 0x7fa0ff, 0.06);
  },
  /** A stern portrait in a dark gilt frame: a pale man in a dark suit whose eyes slowly turn to follow, one way then another, and blink now and then. */
  followingportrait(x) {
    const gilt = 0x8a6a2a, giltLight = 0xc8a050, bg = 0x2a3a34, skin = 0xd8bca0, skinDark = 0xa88a70, hair = 0xc8c8c8, suit = 0x1c1e28;
    fill(x - 8, 5, 17, 23, gilt); fill(x - 8, 5, 17, 1, giltLight); fill(x - 8, 5, 1, 23, giltLight); fill(x + 8, 5, 1, 23, 0x5a4218); fill(x - 8, 27, 17, 1, 0x5a4218);
    fill(x - 6, 7, 13, 19, bg);
    // Shoulders, collar, head and hair
    fill(x - 5, 21, 11, 5, suit); fill(x - 1, 21, 3, 3, 0xeae6dc); dot(x, 24, 0x6a1a1a);
    fill(x - 3, 11, 7, 9, skin); fill(x - 3, 11, 1, 9, skinDark, 0.0); fill(x + 3, 12, 1, 8, skinDark);
    fill(x - 4, 9, 9, 3, hair); fill(x - 4, 12, 1, 3, hair); fill(x + 4, 12, 1, 3, hair); fill(x - 3, 9, 7, 1, 0xe8e8e8);
    fill(x - 1, 16, 2, 2, skinDark); // the nose
    fill(x - 2, 19, 4, 1, 0x8a5a50);
    // The eyes: they look left, centre or right in turn, and shut for a moment every 11 s
    const look = [0, -1, 0, 1, 1, 0, -1, -1][Math.floor((now + x * 0.3) / 4) % 8];
    const shut = (now + x * 0.7) % 11 < 0.25;
    for (const ex of [-2, 2]) {
      if (shut) fill(x + ex - 1, 14, 3, 1, skinDark);
      else { fill(x + ex - 1, 14, 3, 1, WHITE); dot(x + ex + look, 14, 0x1f1a1a); }
      fill(x + ex - 1, 13, 3, 1, hair, 0.8);
    }
    if (isNight) glowLate(() => { if (!shut) { dot(x - 2 + look, 14, 0x9affd0); dot(x + 2 + look, 14, 0x9affd0); } });
  },
  /** A painting of a drowned city: crooked towers of green stone under dark water, a few lit windows, and bubbles that rise now and then. */
  sunkencity(x) {
    const wood = 0x4a3020, woodLight = 0x7a5232, water = 0x143a44, waterLight = 0x1f5a60, stone = 0x0c2228, stoneLight = 0x1a4046;
    fill(x - 12, 8, 25, 19, wood); fill(x - 12, 8, 25, 1, woodLight); fill(x - 12, 8, 1, 19, woodLight); fill(x + 12, 8, 1, 19, 0x2a1a10); fill(x - 12, 26, 25, 1, 0x2a1a10);
    fill(x - 10, 10, 21, 15, water); fill(x - 10, 10, 21, 5, waterLight, 0.5); fill(x - 10, 10, 21, 1, 0x5a9a8a, 0.4);
    // The towers, leaning at wrong angles
    fill(x - 9, 19, 5, 6, stone); fill(x - 8, 16, 3, 3, stone); dot(x - 7, 15, stone);
    fill(x - 2, 14, 4, 11, stone); fill(x - 2, 14, 1, 11, stoneLight); fill(x - 1, 12, 2, 2, stone); dot(x, 11, stone);
    fill(x + 4, 17, 6, 8, stone); fill(x + 6, 15, 2, 2, stone); fill(x + 8, 14, 1, 3, stone); fill(x + 9, 16, 1, 1, stone);
    fill(x - 10, 24, 21, 1, stone);
    // Windows glowing sea-green; one flickers
    dot(x - 7, 21, 0x7fffd0); dot(x - 1, 17, 0x7fffd0); dot(x - 1, 21, 0x7fffd0, 0.6); dot(x + 6, 20, 0x7fffd0);
    if (Math.floor(now * 3 + x) % 9 !== 0) dot(x + 8, 22, 0xe8ffa0);
    // A pale shaft of light, and bubbles rising for 6 s every 20 s
    for (let k = 0; k < 4; k++) dot(x - 5 + k, 11 + k * 2, 0xcfffe8, 0.1);
    const b = (now + x * 0.3) % 20;
    if (b < 6) for (let k = 0; k < 3; k++) dot(x - 6 + k * 6 + (k % 2), 24 - Math.floor(b * 2.2 + k * 2) % 13, 0xcfffe8, 0.7);
    dot(x - 11, 9, WHITE, 0.3);
    if (isNight) glowAt(x, 18, 11, 8, 0x40d0b0, 0.07);
  },
  /** A hanging deep-green tapestry with a gold border and fringe, showing the Elder Sign: a branched stem under a watching eye; its fringe sways now and then and the eye glows at night. */
  eldertapestry(x) {
    const cloth = 0x24544a, clothLight = 0x2f6a5c, clothDark = 0x183a34, gold = 0xc8a050, goldLight = 0xe8c878, goldDark = 0x8a6a2a, wood = 0x5a3b25;
    // The rod and its finials
    fill(x - 9, 3, 19, 1, wood); fill(x - 9, 3, 19, 1, 0x8a5a32, 0.5); dot(x - 10, 3, goldLight); dot(x + 10, 3, goldLight);
    // The cloth and its border
    fill(x - 7, 4, 15, 23, cloth); fill(x - 7, 4, 1, 23, clothLight); fill(x + 7, 4, 1, 23, clothDark);
    fill(x - 6, 5, 13, 1, gold); fill(x - 6, 25, 13, 1, gold); fill(x - 6, 5, 1, 21, gold); fill(x + 6, 5, 1, 21, goldDark);
    // The Elder Sign: eye above a stem with branches
    line(x, 14, x, 23, gold); line(x, 17, x - 3, 14, gold); line(x, 17, x + 3, 14, gold); line(x, 20, x - 3, 17, gold); line(x, 20, x + 3, 17, gold); dot(x, 23, goldLight);
    ellipseRing(x, 10, 3, 2, gold, 0.9); fill(x - 1, 9, 3, 3, 0x0e2a24);
    const a = 0.5 + 0.5 * Math.sin(now * 0.8 + x * 0.2), blink = (now + x * 0.3) % 13 < 0.3;
    if (blink) fill(x - 1, 10, 3, 1, goldDark); else { dot(x, 10, 0x9affd0, 0.6 + a * 0.4); dot(x, 9, 0x9affd0, 0.3); }
    // The fringe, shifting a pixel for 2 s every 15 s
    const sway = (now + x * 0.3) % 15 < 2 ? Math.round(Math.sin(now * 5)) : 0;
    for (let fx = -7; fx <= 7; fx += 2) { dot(x + fx + (fx > 0 ? sway : 0), 27, gold); dot(x + fx + sway, 28, goldDark); }
    if (isNight) glowLate(() => { glowAt(x, 10, 5, 4, 0x7fffc0, 0.14); dot(x, 10, 0xe0fff0); });
  },
  /** A price ticker: a long black LED board on two brackets, coin marks, up and down arrows and little bars scrolling across in two lanes, one each way; it glows at night. */
  tickerboard(x) {
    const frame = 0x2a2e36, frameLight = 0x4a505c, frameDark = 0x16181e, screen = 0x060a0c, up = 0x3ae87a, down = 0xf0504a;
    const L = x - 11, W = 23;
    // The brackets and the board
    fill(L + 3, 8, 2, 3, frameDark); fill(L + W - 5, 8, 2, 3, frameDark);
    fill(L, 10, W, 13, frame); fill(L, 10, W, 1, frameLight); fill(L, 10, 1, 13, frameLight); fill(L + W - 1, 10, 1, 13, frameDark); fill(L, 22, W, 1, frameDark);
    fill(L + 1, 11, W - 2, 11, screen); fill(L + 1, 16, W - 2, 1, 0x141a1e);
    /** One lane's items, scrolled by `off`: a coin mark, an arrow and a bar for each. */
    const lane = (top, off, salt) => {
      for (let k = -1; k < 4; k++) {
        const slot = Math.floor(off / 11) + k, ix = L + 1 + k * 11 - (off % 11), h = hash(slot * 7 + salt, salt);
        const rising = h % 3 !== 0, c = rising ? up : down, kind = COIN_MARKS[(h >>> 3) % COIN_MARKS.length];
        const clip = (dx, dy, w, hh, col, a = 1) => { const x0 = Math.max(ix + dx, L + 1), x1 = Math.min(ix + dx + w, L + W - 1); if (x1 > x0) fill(x0, top + dy, x1 - x0, hh, col, a); };
        if (ix + 2 > L + 2 && ix + 2 < L + W - 3) coinMark(kind, ix + 2, top + 2, 2);
        if (rising) { clip(5, 1, 1, 1, c); clip(4, 2, 3, 1, c); } else { clip(4, 2, 3, 1, c); clip(5, 3, 1, 1, c); }
        clip(7, 2, 1 + ((h >>> 6) % 3), 1, c, 0.7);
      }
    };
    lane(11, Math.floor(now * 4 + x), 1);
    lane(17, 1000 - Math.floor(now * 3 + x), 2);
    const glow = () => glowAt(x, 16, 16, 8, 0x60ffb0, 0.08);
    if (isNight) glowLate(() => { glow(); fill(L + 1, 11, W - 2, 1, 0x60ffb0, 0.08); }); else fill(L + 2, 11, 4, 1, WHITE, 0.08);
  },
  /** A candle chart: a big wall screen in a slim black frame, green and red candles marching up and down over a faint grid with a dotted average; every 20 s a new candle grows at the right and the rest shuffle along. */
  candlechart(x) {
    const frame = 0x22252d, frameLight = 0x3e434e, frameDark = 0x121419, screen = 0x0a1218, grid = 0x18262c, up = 0x3ad87a, down = 0xe8483e;
    const L = x - 10, T = 5, W = 21, H = 20;
    fill(L, T, W, H, frame); fill(L, T, W, 1, frameLight); fill(L, T, 1, H, frameLight); fill(L + W - 1, T, 1, H, frameDark); fill(L, T + H - 1, W, 1, frameDark);
    fill(L + 1, T + 1, W - 2, H - 3, screen);
    for (const gy of [T + 5, T + 10, T + 15]) fill(L + 1, gy, W - 2, 1, grid);
    for (const gx of [L + 7, L + 14]) fill(gx, T + 1, 1, H - 3, grid);
    // The candles: each from the last one's close, from a stable random walk, a new one every 20 s
    const t = now + x * 0.37, era = Math.floor(t / 20), grow = Math.min(1, (t % 20) / 2.5);
    let level = 0;
    const walk = [];
    for (let k = era - 14; k <= era; k++) {
      const h = hash(k * 13 + x, 5), move = ((h % 7) - 3) + (k % 9 < 5 ? 1 : -1);
      const open = level, close = Math.max(-5, Math.min(5, level + move - Math.round(level / 3)));
      walk.push({ open, close, hi: Math.max(open, close) + 1 + ((h >>> 4) % 2), lo: Math.min(open, close) - 1 - ((h >>> 6) % 2) });
      level = close;
    }
    const base = T + 10, shown = walk.slice(-6);
    shown.forEach((c, i) => {
      const cx = L + 2 + i * 3, last = i === shown.length - 1, rising = c.close >= c.open, col = rising ? up : down;
      const close = last ? Math.round(c.open + (c.close - c.open) * grow) : c.close;
      const top = base - Math.max(c.open, close), bottom = base - Math.min(c.open, close);
      fill(cx + 1, base - c.hi, 1, c.hi - c.lo + 1, col, 0.6);
      fill(cx, top, 3, Math.max(1, bottom - top + 1), col); dot(cx, top, shaded(col, 1.25));
      if (i % 2 === 0) dot(cx + 1, base - Math.round((c.open + c.close) / 2) - 1, 0xf2d35b, 0.7);
    });
    // The foot strip, a status light and the glow
    fill(L + 1, T + H - 2, W - 2, 1, 0x161a20); dot(L + W - 3, T + H - 2, 0x3ad87a);
    dot(L + 2, T + 2, WHITE, 0.15);
    if (isNight) glowLate(() => glowAt(x, T + 9, 14, 10, 0x50c0a0, 0.07));
  },
  /** Market clocks: three small round clocks on a walnut strip, each set to a different far-off market's hour, a coloured tag under each; their hands keep the real time. */
  marketclocks(x) {
    const strip = 0x5a3e2a, stripLight = 0x7a5838, stripDark = 0x3a2618, rim = 0xc4c9d0, face = 0xf4f2ea, hand = 0x22252d;
    fill(x - 11, 21, 23, 3, strip); fill(x - 11, 21, 23, 1, stripLight); fill(x - 11, 23, 23, 1, stripDark);
    const d = new Date(), mins = d.getMinutes(), hours = d.getHours();
    [[-8, -5, 0x4c8bf5], [0, 0, 0xf2d35b], [8, 8, 0xd8402a]].forEach(([dx, shift, tag]) => {
      const cx = x + dx, cy = 13;
      disc(cx, cy, 3.4, rim); disc(cx, cy, 2.5, face); dot(cx - 2, cy - 2, WHITE, 0.8);
      const ma = (mins / 60) * Math.PI * 2 - Math.PI / 2, ha = (((hours + shift + 24) % 12 + mins / 60) / 12) * Math.PI * 2 - Math.PI / 2;
      line(cx, cy, cx + Math.round(Math.cos(ma) * 2), cy + Math.round(Math.sin(ma) * 2), hand);
      line(cx, cy, cx + Math.round(Math.cos(ha) * 1.4), cy + Math.round(Math.sin(ha) * 1.4), 0xd8402a);
      // The tag under it: a coloured block, no name
      fill(cx - 2, 22, 5, 1, tag); dot(cx - 2, 22, shaded(tag, 1.3));
    });
  },
  /** A moon-trip poster: a navy print in a thin gold frame, a big pale moon in one corner, stars, and a little rocket at the foot with a dotted path curving up to the moon; every 25 s the rocket makes the trip and is back. */
  moonposter(x) {
    const frame = 0xc8a050, frameLight = 0xe8c878, frameDark = 0x8a6a2a, sky = 0x1a2350, moon = 0xf2eac8, moonDark = 0xd8cca0;
    fill(x - 9, 4, 19, 23, frame); fill(x - 9, 4, 19, 1, frameLight); fill(x - 9, 4, 1, 23, frameLight); fill(x + 9, 5, 1, 22, frameDark); fill(x - 9, 26, 19, 1, frameDark);
    fill(x - 8, 5, 17, 21, sky);
    for (const [sx, sy] of [[-6, 8], [-2, 6], [2, 13], [-5, 15], [7, 17], [-7, 21], [4, 22]]) dot(x + sx, sy, WHITE, 0.6 + ((sx + sy) % 3) * 0.12);
    // The moon, its craters
    disc(x + 4, 10, 3.6, moon); dot(x + 3, 9, moonDark); dot(x + 5, 11, moonDark); dot(x + 6, 9, moonDark); dot(x + 2, 12, moonDark);
    // The path and the rocket on it
    const path = [[-6, 23], [-5, 20], [-4, 18], [-3, 16], [-1, 14], [0, 13], [1, 12]];
    path.forEach(([px, py], i) => { if (i % 2 === 0) dot(x + px, py, 0x7affb0, 0.7); });
    const t = (now + x * 0.29) % 25, at = t < 3 ? Math.floor((t / 3) * (path.length - 1)) : 0, [rx, ry] = path[at];
    fill(x + rx, ry - 2, 1, 3, WHITE); dot(x + rx, ry - 3, 0xd8402a); dot(x + rx - 1, ry, 0x3ad87a); dot(x + rx + 1, ry, 0x3ad87a);
    dot(x + rx, ry + 1, 0xffd860, t < 3 ? 1 : 0.5);
  },
  /** An up-only chart: a framed print of a bright green line climbing in steps across a pale grid to an arrowhead in the top corner, a gold star by it that twinkles every 15 s. */
  upchart(x) {
    const frame = 0x2a2e36, frameLight = 0x464c58, paper = 0xeef4ee, grid = 0xd4e0d6, up = 0x2ab868, upLight = 0x5ae88a;
    fill(x - 10, 5, 21, 20, frame); fill(x - 10, 5, 21, 1, frameLight); fill(x - 10, 5, 1, 20, frameLight);
    fill(x - 9, 6, 19, 18, paper);
    for (const gy of [10, 14, 18, 22]) fill(x - 9, gy, 19, 1, grid);
    for (const gx of [-5, 0, 5]) fill(x + gx, 6, 1, 18, grid);
    // The line, step by step, and its arrowhead
    const steps = [[-8, 21], [-5, 19], [-3, 20], [0, 16], [2, 17], [4, 13], [6, 10]];
    for (let k = 0; k < steps.length - 1; k++) { line(x + steps[k][0], steps[k][1], x + steps[k + 1][0], steps[k + 1][1], up); }
    line(x + 6, 10, x + 8, 8, up); fill(x + 7, 8, 2, 1, upLight); fill(x + 8, 8, 1, 2, upLight); dot(x + 6, 8, up); dot(x + 8, 10, up);
    // The star, twinkling
    const t = (now + x * 0.31) % 15, big = t < 1.5 && Math.floor(t * 4) % 2 === 0;
    dot(x + 3, 8, 0xf2c030); if (big) { dot(x + 2, 8, 0xffe070); dot(x + 4, 8, 0xffe070); dot(x + 3, 7, 0xffe070); dot(x + 3, 9, 0xffe070); }
    fill(x - 10, 24, 21, 1, 0x16181e);
  },
  /** A moon window: a round-topped window onto a sky with a huge moon hanging in it, pale by day and glowing after dark among twinkling stars; every 30 s a tiny rocket streaks across toward it. */
  moonwindow(x) {
    const frame = 0xe8e4da, frameDark = 0xb8b2a4, sky = isNight ? 0x0c1236 : 0x6aa8e0, moon = isNight ? 0xfff4c8 : 0xf4f0e0, crater = isNight ? 0xe0d4a0 : 0xd8d4c4;
    // The pane: a rectangle under a half-round top
    fill(x - 9, 12, 19, 16, frame); disc(x, 12, 9.5, frame);
    fill(x - 8, 12, 17, 15, sky); disc(x, 12, 8.4, sky);
    if (isNight) for (const [sx, sy] of [[-6, 9], [-3, 22], [6, 21], [-7, 16], [3, 6]]) dot(x + sx, sy, WHITE, Math.floor(now * 1.3 + sx) % 3 ? 0.8 : 0.3);
    else for (const [cx, cy] of [[-5, 22], [5, 24]]) { fill(x + cx - 2, cy, 5, 1, WHITE, 0.7); fill(x + cx - 1, cy - 1, 3, 1, WHITE, 0.7); }
    // The moon, big in the pane
    const glow = () => { glowAt(x + 1, 13, 9, 8, 0xfff0b0, 0.14); disc(x + 1, 13, 5.4, moon); for (const [mx, my] of [[-1, -1], [2, 1], [0, 3], [3, -2], [-2, 2]]) dot(x + 1 + mx, 13 + my, crater); };
    if (isNight) glowLate(glow); else glow();
    // The rocket, crossing for 2 s every 30
    const t = (now + x * 0.23) % 30;
    if (t < 2) { const rx = x - 8 + Math.floor(t * 7), ry = 24 - Math.floor(t * 4); dot(rx, ry, WHITE); dot(rx - 1, ry + 1, 0x7affb0, 0.8); dot(rx - 2, ry + 2, 0xffd860, 0.5); }
    // The glazing bar, the sill and the frame's shade
    fill(x, 12, 1, 15, frame); fill(x - 8, 19, 17, 1, frame);
    fill(x - 10, 27, 21, 2, frame); fill(x - 10, 28, 21, 1, frameDark); fill(x + 9, 12, 1, 15, frameDark);
    dot(x - 6, 13, WHITE, 0.5);
  },
  /** A falling chart: a wall screen of red candles stepping down over a dim grid toward the bottom of the screen, now and then a lone small green one; a new candle drops in every 20 s and it glows a dull red at night. */
  redchart(x) {
    const frame = 0x22252d, frameLight = 0x3e434e, frameDark = 0x121419, screen = 0x120a0c, grid = 0x2a1a1e, down = 0xe8483e, up = 0x3ad87a;
    const L = x - 10, T = 5, W = 21, H = 20;
    fill(L, T, W, H, frame); fill(L, T, W, 1, frameLight); fill(L, T, 1, H, frameLight); fill(L + W - 1, T, 1, H, frameDark); fill(L, T + H - 1, W, 1, frameDark);
    fill(L + 1, T + 1, W - 2, H - 3, screen);
    for (const gy of [T + 5, T + 10, T + 15]) fill(L + 1, gy, W - 2, 1, grid);
    // Six candles, each a step lower than the last, sliding along as a new one drops in
    const t = now + x * 0.37, era = Math.floor(t / 20), grow = Math.min(1, (t % 20) / 2.5);
    for (let i = 0; i < 6; i++) {
      const k = era - 5 + i, h = hash(k + x, 9), rise = h % 5 === 0, cx = L + 2 + i * 3;
      const top = T + 3 + i * 2 + (h % 2), len = 2 + ((h >>> 3) % 3), last = i === 5;
      const shown = last ? Math.max(1, Math.round(len * grow)) : len, col = rise ? up : down;
      fill(cx + 1, top - 1, 1, shown + 2, col, 0.55);
      fill(cx, top, 3, shown, col); dot(cx, top, shaded(col, 1.25));
    }
    fill(L + 1, T + H - 2, W - 2, 1, 0x1a1214); dot(L + W - 3, T + H - 2, 0xe8483e);
    dot(L + 2, T + 2, WHITE, 0.15);
    if (isNight) glowLate(() => glowAt(x, T + 10, 13, 10, 0xe85040, 0.06));
  },
  /** A stormy window: grey cloud low over a grey city, rain streaking down the glass and drops running along it; every 40 s lightning whitens the sky for a moment. */
  stormpane(x) {
    const frame = 0xb8bcc4, frameDark = 0x8a8e98, flash = (now + x * 0.21) % 40 < 0.25 || ((now + x * 0.21) % 40 > 0.4 && (now + x * 0.21) % 40 < 0.55);
    const sky = flash ? 0xd8e0f0 : isNight ? 0x1a2030 : 0x6a7280, cloud = flash ? 0xf0f4ff : isNight ? 0x262c3c : 0x7e8692, city = isNight ? 0x12161e : 0x4a505c;
    fill(x - 10, 6, 21, 22, frame); fill(x - 9, 7, 19, 20, sky);
    // The cloud, low and heavy, and the city under it
    fill(x - 9, 7, 19, 4, cloud); for (const [cx, w] of [[-8, 5], [-2, 6], [5, 4]]) fill(x + cx, 11, w, 1, cloud);
    for (const [bx, bh] of [[-9, 6], [-6, 9], [-2, 5], [1, 8], [5, 6], [8, 4]]) fill(x + bx, 27 - bh, 3, bh, city);
    if (isNight) for (const [wx, wy] of [[-5, 20], [2, 21], [6, 23]]) dot(x + wx, wy, 0xd8c070, 0.6);
    // The rain: short slanted streaks falling by steps
    for (let k = 0; k < 9; k++) {
      const rx = x - 9 + ((k * 7 + 3) % 19), ry = 8 + ((Math.floor(now * 12) + k * 5) % 18);
      dot(rx, ry, 0xc8d4e4, 0.55); if (ry + 1 < 27) dot(rx - 1, ry + 1, 0xc8d4e4, 0.35);
    }
    // Drops running down the glass, slowly
    for (let k = 0; k < 3; k++) { const dx = x - 6 + k * 6, dy = 8 + Math.floor((now * (0.8 + k * 0.3) + k * 3) % 18); dot(dx, dy, WHITE, 0.6); dot(dx, dy - 1, WHITE, 0.25); }
    // The glazing bars and the sill
    fill(x, 7, 1, 20, frame); fill(x - 9, 16, 19, 1, frame);
    fill(x - 11, 27, 23, 2, frame); fill(x - 11, 28, 23, 1, frameDark);
    if (flash && isNight) glowLate(() => glowAt(x, 16, 14, 12, 0xd8e4ff, 0.2));
  },
  /** A hang-in-there poster: a little brown bear cub clinging to a bare branch over a pale sky, legs dangling, no words; every 20 s it slips a pixel and hauls itself back up. */
  hanginthere(x) {
    const frame = 0x6a5a48, frameLight = 0x8a7860, paper = 0xdce6ee, sky = 0xc4d4e2, bark = 0x5a3e28, fur = 0x7a5232, furLight = 0x9a6a42, furDark = 0x5a3820;
    fill(x - 8, 4, 17, 23, frame); fill(x - 8, 4, 17, 1, frameLight); fill(x - 8, 4, 1, 23, frameLight);
    fill(x - 7, 5, 15, 21, paper); fill(x - 7, 16, 15, 10, sky);
    // The branch, across and down to the corner
    line(x - 7, 9, x + 7, 8, bark); line(x - 7, 10, x + 2, 9, bark); dot(x + 4, 7, bark); dot(x + 5, 6, bark);
    // The cub hanging below it by its paws
    const t = (now + x * 0.31) % 20, slip = t < 1.5 && t > 0.5 ? 1 : 0, by = 10 + slip;
    fill(x - 2, by, 1, 2, fur); fill(x + 2, by, 1, 2, fur);
    fill(x - 3, by + 2, 7, 4, fur); fill(x - 3, by + 2, 7, 1, furLight); fill(x - 2, by + 6, 5, 4, fur); fill(x + 2, by + 6, 1, 4, furDark);
    dot(x - 3, by + 2, furLight); dot(x + 3, by + 2, furLight);
    dot(x - 1, by + 3, PALETTE.eye); dot(x + 1, by + 3, PALETTE.eye); fill(x - 1, by + 4, 3, 1, 0xc8a080); dot(x, by + 4, 0x2a1c14);
    fill(x - 2, by + 10, 1, 2, furDark); fill(x + 2, by + 10, 1, 2 + slip, furDark);
  },
  /** A blackout curtain: two heavy black panels on a rod drawn over a window, a thin crack of light between them where dust drifts in the beam; by night the crack is a pale moonlit blue. */
  blackout(x) {
    const cloth = 0x1c1c22, clothLight = 0x2c2c34, clothDark = 0x101014, rod = 0x6e727c;
    fill(x - 11, 4, 23, 1, rod); dot(x - 12, 4, 0xa8adb6); dot(x + 12, 4, 0xa8adb6);
    // The panels, folds down them, bunched at the hem
    const crack = x + 1, beam = isNight ? 0xa8c4f0 : 0xfff0b0;
    fill(x - 11, 5, 23, 24, cloth);
    for (const fx of [-9, -6, -3, 4, 7, 10]) { fill(x + fx, 5, 1, 24, clothLight); fill(x + fx + 1, 5, 1, 24, clothDark); }
    for (let k = 0; k < 5; k++) dot(x - 11 + k * 5, 4, 0x8a8f9c);
    fill(x - 11, 28, 23, 1, clothDark);
    // The crack of light and the dust in it
    fill(crack, 5, 1, 23, beam, isNight ? 0.6 : 0.9); dot(crack, 5, WHITE, 0.8);
    const motes = () => { for (let k = 0; k < 3; k++) { const my = 8 + ((Math.floor(now * 2) + k * 7) % 18); dot(crack + (k % 2 ? 1 : -1), my, beam, 0.5); } glowAt(crack, 16, 3, 12, beam, 0.1); };
    if (isNight) glowLate(motes); else motes();
  },
  /** Taped-up posters: three prints stuck on askew with tape, a rocket heading for the moon, a candle chart and a big coin mark; every 25 s the coin poster's loose corner lifts in a draught and settles. */
  denposters(x) {
    const tape = 0xe8e0b8;
    // The rocket print, a little high on the left
    fill(x - 12, 6, 9, 11, 0x1a2350); disc(x - 6, 9, 1.6, 0xf2eac8); fill(x - 10, 12, 1, 3, WHITE); dot(x - 10, 11, 0xd8402a); dot(x - 10, 15, 0xffd860);
    dot(x - 8, 8, WHITE, 0.6); dot(x - 11, 9, WHITE, 0.5);
    fill(x - 12, 6, 2, 1, tape, 0.85); fill(x - 5, 16, 2, 1, tape, 0.85);
    // The candle chart, lower in the middle
    fill(x - 3, 12, 8, 10, 0xeef0f2); for (const [cx, top, h, c] of [[-2, 17, 3, 0x3ad87a], [0, 15, 3, 0xe8483e], [2, 14, 4, 0x3ad87a]]) fill(x + cx, top, 1, h, c);
    fill(x - 3, 12, 2, 1, tape, 0.85); fill(x + 3, 21, 2, 1, tape, 0.85);
    // The coin poster on the right, its bottom corner loose
    fill(x + 6, 5, 7, 10, 0x2a2e36); coinMark('diamond', x + 9, 10, 2);
    fill(x + 6, 5, 2, 1, tape, 0.85); fill(x + 11, 5, 2, 1, tape, 0.85);
    const t = (now + x * 0.29) % 25, lift = t < 2 ? [0, 1, 1, 0][Math.floor(t * 2) % 4] : 0;
    fill(x + 11, 14 - lift, 2, 1 + lift, lift ? 0xd8d4cc : 0x2a2e36);
  },
  /** A floodlight window onto a stadium: stands along the bottom and two lamp masts, their banks of lamps dark by day; at night the lamps come on one after another every 30 s, and the pitch glows below. */
  floodlights(x) {
    const frame = 0x1b2a40, frameLight = 0x34527a;
    fill(x - 12, 3, 25, 26, frame); fill(x - 12, 3, 25, 1, frameLight); fill(x - 12, 3, 1, 26, frameLight);
    const sky = isNight ? 0x0c1c3a : 0x8cc4f0;
    fill(x - 11, 4, 23, 24, sky); if (!isNight) fill(x - 11, 4, 23, 4, 0xa8d4f6);
    for (const sx of [-8, 5]) dot(x + sx, 7 + (sx % 3), isNight ? WHITE : 0xffffff, isNight ? 0.7 : 0.5); // a star, or a cloud's edge
    // The stand's roof line and the pitch below
    fill(x - 11, 20, 23, 8, 0x1c3a2c); fill(x - 11, 20, 23, 1, 0x6a7a8a); fill(x - 11, 22, 23, 6, 0x3a8836); fill(x - 11, 24, 23, 1, WHITE, 0.4);
    fill(x - 11, 18, 23, 2, 0x2a3a52); for (let k = 0; k < 23; k += 3) dot(x - 11 + k, 19, k % 6 ? PSG_RED : WHITE, 0.8);
    // The masts, their lamp banks three by two
    const t = (now + x * 0.3) % 30, lit = (m) => isNight && (t > 6 + m * 1.5 || t < 0.01) && true;
    [[-6, 0], [6, 1]].forEach(([mx, m]) => {
      fill(x + mx, 12, 1, 8, 0x6e727c);
      fill(x + mx - 3, 8, 7, 4, 0x3a3e48); fill(x + mx - 3, 8, 7, 1, 0x5a5e68);
      for (let k = 0; k < 3; k++) for (let r = 0; r < 2; r++) dot(x + mx - 2 + k * 2, 9 + r * 2, isNight ? 0xfff4c0 : 0xb8bcc8);
    });
    fill(x - 12, 28, 25, 1, frame); fill(x - 12, 29, 25, 1, SHADOW, 0.5);
    if (isNight) glowLate(() => { for (const mx of [-6, 6]) { glowAt(x + mx, 10, 8, 6, 0xfff0b0, 0.18); } glowAt(x, 24, 12, 4, 0x6ad86a, 0.08); });
  },
  /** A framed home shirt (brand nod: PSG): a navy shirt with its red stripe edged in white, a small crest on the chest, in a white frame on a navy mount; a glint sweeps the glass every 20 s. */
  framedjersey(x) {
    const frame = 0xeef0f4, frameDark = 0xb8bcc8;
    fill(x - 10, 4, 21, 25, frame); fill(x - 10, 4, 1, 25, WHITE); fill(x + 10, 4, 1, 25, frameDark); fill(x - 10, 28, 21, 1, frameDark);
    fill(x - 8, 6, 17, 21, 0x062a4c);
    // The shirt: body, sleeves and a white collar
    fill(x - 4, 9, 9, 14, PSG_NAVY); fill(x - 8, 9, 4, 5, PSG_NAVY); fill(x + 5, 9, 4, 5, PSG_NAVY);
    fill(x - 8, 13, 4, 1, PSG_NAVY_DARK); fill(x + 5, 13, 4, 1, PSG_NAVY_DARK); fill(x - 4, 9, 1, 14, PSG_NAVY_LIGHT);
    fill(x - 1, 9, 3, 14, WHITE); fill(x, 9, 1, 14, PSG_RED);
    fill(x - 2, 8, 5, 1, WHITE); dot(x, 9, PSG_NAVY);
    psgNod(x - 3, 12, 1.4);
    fill(x - 4, 22, 9, 1, PSG_NAVY_DARK);
    const g = (now + x * 0.21) % 20;
    if (g < 1.2) { const gx = x - 8 + Math.floor(g * 14); line(gx, 8, gx - 4, 24, WHITE, 0.5); }
  },
  /** Lakers banners (brand nod: Lakers). Three hanging banners in purple and gold, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  lakersbanner(x) { nbaBanner(NBA_TEAMS.lakers, x); },
  /** Celtics framed shirt (brand nod: Celtics). The home shirt in green trimmed in white with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  celticsbanner(x) { nbaBanner(NBA_TEAMS.celtics, x); },
  /** Warriors banners (brand nod: Warriors). Three hanging banners in blue and yellow, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  warriorsbanner(x) { nbaBanner(NBA_TEAMS.warriors, x); },
  /** Knicks framed shirt (brand nod: Knicks). The home shirt in blue trimmed in orange with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  knicksbanner(x) { nbaBanner(NBA_TEAMS.knicks, x); },
  /** Bulls banners (brand nod: Bulls). Three hanging banners in red and black, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  bullsbanner(x) { nbaBanner(NBA_TEAMS.bulls, x); },
  /** Heat framed shirt (brand nod: Heat). The home shirt in red trimmed in orange with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  heatbanner(x) { nbaBanner(NBA_TEAMS.heat, x); },
  /** Spurs banners (brand nod: Spurs). Three hanging banners in black and silver, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  spursbanner(x) { nbaBanner(NBA_TEAMS.spurs, x); },
  /** Thunder framed shirt (brand nod: Thunder). The home shirt in blue trimmed in orange with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  thunderbanner(x) { nbaBanner(NBA_TEAMS.thunder, x); },
  /** Timberwolves banners (brand nod: Timberwolves). Three hanging banners in navy and green, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  wolvesbanner(x) { nbaBanner(NBA_TEAMS.wolves, x); },
  /** Nuggets framed shirt (brand nod: Nuggets). The home shirt in navy trimmed in gold with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  nuggetsbanner(x) { nbaBanner(NBA_TEAMS.nuggets, x); },
  /** Hawks banners (brand nod: Hawks). Three hanging banners in red and volt, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  hawksbanner(x) { nbaBanner(NBA_TEAMS.hawks, x); },
  /** Nets framed shirt (brand nod: Nets). The home shirt in black trimmed in white with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  netsbanner(x) { nbaBanner(NBA_TEAMS.nets, x); },
  /** Hornets banners (brand nod: Hornets). Three hanging banners in teal and purple, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  hornetsbanner(x) { nbaBanner(NBA_TEAMS.hornets, x); },
  /** Cavaliers framed shirt (brand nod: Cavaliers). The home shirt in wine trimmed in gold with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  cavsbanner(x) { nbaBanner(NBA_TEAMS.cavs, x); },
  /** Mavericks banners (brand nod: Mavericks). Three hanging banners in blue and silver, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  mavsbanner(x) { nbaBanner(NBA_TEAMS.mavs, x); },
  /** Pistons framed shirt (brand nod: Pistons). The home shirt in blue trimmed in red with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  pistonsbanner(x) { nbaBanner(NBA_TEAMS.pistons, x); },
  /** Rockets banners (brand nod: Rockets). Three hanging banners in red and silver, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  rocketsbanner(x) { nbaBanner(NBA_TEAMS.rockets, x); },
  /** Pacers framed shirt (brand nod: Pacers). The home shirt in navy trimmed in gold with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  pacersbanner(x) { nbaBanner(NBA_TEAMS.pacers, x); },
  /** Clippers banners (brand nod: Clippers). Three hanging banners in red and blue, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  clippersbanner(x) { nbaBanner(NBA_TEAMS.clippers, x); },
  /** Grizzlies framed shirt (brand nod: Grizzlies). The home shirt in navy trimmed in blue with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  grizzliesbanner(x) { nbaBanner(NBA_TEAMS.grizzlies, x); },
  /** Bucks banners (brand nod: Bucks). Three hanging banners in green and cream, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  bucksbanner(x) { nbaBanner(NBA_TEAMS.bucks, x); },
  /** Pelicans framed shirt (brand nod: Pelicans). The home shirt in navy trimmed in red with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  pelicansbanner(x) { nbaBanner(NBA_TEAMS.pelicans, x); },
  /** Magic banners (brand nod: Magic). Three hanging banners in blue and black, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  magicbanner(x) { nbaBanner(NBA_TEAMS.magic, x); },
  /** 76ers framed shirt (brand nod: 76ers). The home shirt in blue trimmed in red with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  sixersbanner(x) { nbaBanner(NBA_TEAMS.sixers, x); },
  /** Suns banners (brand nod: Suns). Three hanging banners in purple and orange, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  sunsbanner(x) { nbaBanner(NBA_TEAMS.suns, x); },
  /** Trail Blazers framed shirt (brand nod: Trail Blazers). The home shirt in red trimmed in black with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  blazersbanner(x) { nbaBanner(NBA_TEAMS.blazers, x); },
  /** Kings banners (brand nod: Kings). Three hanging banners in purple and silver, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  kingsbanner(x) { nbaBanner(NBA_TEAMS.kings, x); },
  /** Raptors framed shirt (brand nod: Raptors). The home shirt in red trimmed in black with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  raptorsbanner(x) { nbaBanner(NBA_TEAMS.raptors, x); },
  /** Jazz banners (brand nod: Jazz). Three hanging banners in navy and yellow, the middle one longer and carrying the badge; they stir for a few seconds every 26 s. */
  jazzbanner(x) { nbaBanner(NBA_TEAMS.jazz, x); },
  /** Wizards framed shirt (brand nod: Wizards). The home shirt in navy trimmed in red with the badge on the chest, framed in white; a glint crosses the glass every 20 s. */
  wizardsbanner(x) { nbaBanner(NBA_TEAMS.wizards, x); },
  /** Cap's shield, mounted (brand nod: Marvel): the big round shield on a steel hook, its rings two pixels wide round the star; every 30 s a gleam runs round its rim as if it had just been thrown back. */
  marvel1shieldwall(x) {
    fill(x - 1, 3, 3, 3, M1_STEEL); dot(x - 1, 3, M1_SILVER); dot(x + 1, 5, M1_STEEL_DARK);
    disc(x + 1, 17, 9, BLACK, 0.22);
    m1Shield(x, 15, 9);
    const t = (now + x * 0.3) % 30;
    if (t < 1.6) {
      const a = (t / 1.6) * Math.PI * 2 - Math.PI / 2;
      for (let k = 0; k < 3; k++) dot(x + Math.round(Math.cos(a - k * 0.25) * 8), 15 + Math.round(Math.sin(a - k * 0.25) * 8), WHITE, 0.9 - k * 0.25);
    }
  },
  /** Mjolnir on its hook: Thor's hammer hanging head-down by its strap from a peg on a short oak shelf, runes on its side; every 22 s lightning crackles round it for a moment and it glows pale blue. */
  marvel1hammerwall(x) {
    const oak = 0x8a5a32;
    fill(x - 7, 4, 15, 2, oak); fill(x - 7, 4, 15, 1, 0xa8703e); fill(x - 7, 6, 15, 1, SHADOW); fill(x - 6, 6, 1, 2, 0x5a3b25); fill(x + 6, 6, 1, 2, 0x5a3b25);
    dot(x, 6, M1_STEEL); dot(x, 7, M1_STEEL_DARK);
    // The strap loop, the leather handle and the head
    line(x - 1, 8, x, 7, 0x4a3020); line(x + 1, 8, x, 7, 0x4a3020);
    fill(x, 9, 1, 9, 0x6a4a2e); for (let k = 10; k < 18; k += 2) dot(x, k, 0x4a3020);
    dot(x, 18, 0x8a8e98);
    fill(x - 4, 19, 9, 6, 0x8a8e98); fill(x - 4, 19, 9, 1, 0xb8bcc6); fill(x - 4, 19, 1, 6, 0xa8acb6); fill(x + 4, 19, 1, 6, 0x5a5e68); fill(x - 4, 24, 9, 1, 0x5a5e68);
    for (const [dx, dy] of [[-2, 21], [-1, 22], [1, 21], [2, 22]]) dot(x + dx, dy, 0x5a5e68);
    fill(x - 4, 26, 9, 1, SHADOW);
    const t = (now + x * 0.31) % 22;
    if (t < 1.4 && Math.floor(now * 10) % 2 === 0) {
      const seed = Math.floor(now * 10);
      m1Bolt(x - 7, 12, x - 4, 23, seed); m1Bolt(x + 7, 14, x + 4, 22, seed + 3);
      glowAt(x, 21, 9, 6, 0xbfe8ff, 0.16);
    }
  },
  /** Spidey swinging by: a window onto the city, a skyline of towers whose windows light up at night; every 24 s Spider-Man swings across it on a web line (brand nod: Marvel). */
  marvel1spidewindow(x) {
    const frame = 0x2a2e3a, frameLight = 0x4a5060;
    fill(x - 12, 3, 25, 26, frame); fill(x - 12, 3, 25, 1, frameLight); fill(x - 12, 3, 1, 26, frameLight);
    fill(x - 11, 4, 23, 24, isNight ? 0x0c1c3a : 0x8cc4f0);
    if (!isNight) fill(x - 11, 4, 23, 5, 0xa8d4f6); else { dot(x - 7, 7, WHITE, 0.7); dot(x + 6, 6, WHITE, 0.5); }
    // The skyline: towers of different heights, lit windows at night
    const towers = [[-11, 15, 4], [-7, 10, 5], [-2, 17, 4], [2, 8, 3], [5, 13, 4], [9, 18, 3]];
    for (const [dx, top, w] of towers) {
      fill(x + dx, top, w, 28 - top, isNight ? 0x1a2440 : 0x5a6a88); fill(x + dx, top, 1, 28 - top, isNight ? 0x24304e : 0x6a7a98);
      for (let wy = top + 2; wy < 27; wy += 2) for (let wx = 1; wx < w; wx += 2) if (hash(x + dx + wx, wy) % 3 === 0) dot(x + dx + wx, wy, isNight ? 0xffe08a : 0xb8d0ec, isNight ? 0.85 : 0.6);
    }
    fill(x + 3, 6, 1, 2, isNight ? 0x1a2440 : 0x5a6a88); // the spire
    // The swing
    const t = (now + x * 0.3) % 24;
    if (t < 2.2) {
      const q = t / 2.2, sx = x - 13 + Math.round(q * 26), sy = 10 + Math.round(Math.sin(q * Math.PI) * 7);
      if (sx > x - 12 && sx < x + 11) {
        line(x - 2, 4, sx, sy, M1_WEB, 0.85);
        fill(sx - 1, sy, 3, 2, M1_RED); fill(sx - 1, sy + 2, 3, 2, M1_BLUE); dot(sx - 1, sy, WHITE); dot(sx + 1, sy, WHITE);
        dot(sx + 2, sy + 3, M1_BLUE); dot(sx - 2, sy + 1, M1_RED);
      }
    }
    fill(x - 12, 15, 25, 1, frame); fill(x, 3, 1, 26, frame);
    fill(x - 12, 28, 25, 1, frame); fill(x - 13, 29, 27, 1, PALETTE.sill);
  },
  /** Assemble banner (brand nod: Marvel): a red banner with a gold edge hanging from a brass rod, the Avengers' white A in the middle and a pointed foot with a gold tassel; it stirs every 20 s. */
  marvel1assemble(x) {
    fill(x - 8, 3, 17, 1, M1_GOLD_DARK); fill(x - 8, 3, 17, 1, M1_GOLD, 0.8); dot(x - 9, 3, M1_GOLD); dot(x + 9, 3, M1_GOLD);
    const stir = (now + x * 0.33) % 20 < 2 ? Math.floor(now * 4) % 2 : 0;
    fill(x - 6, 4, 13, 19, M1_RED); fill(x - 6, 4, 1, 19, M1_RED_LIGHT); fill(x + 6, 4, 1, 19, M1_RED_DARK);
    fill(x - 6, 4, 13, 1, M1_GOLD); fill(x - 5, 21, 11, 1, M1_GOLD, 0.8);
    for (let k = 0; k < 4; k++) { fill(x - 6 + k + stir, 23 + k, 13 - 2 * k, 1, k === 3 ? M1_RED_DARK : M1_RED); }
    dot(x + stir, 27, M1_GOLD); dot(x + stir, 28, M1_GOLD_DARK);
    m1A(x, 12, M1_CREAM);
    fill(x - 6, 23, 13, 1, SHADOW, 0.5);
  },
  /** A Hulk-smashed wall (brand nod: Marvel): a jagged hole punched through the plaster, cracks running out from it and crumbs on the rail; every 35 s a big green fist punches back through and a puff of dust flies. */
  marvel1hulkhole(x) {
    const crack = 0x14161c;
    for (const [ex, ey] of [[-11, 6], [-10, 24], [11, 8], [10, 26], [-3, 2], [4, 29], [12, 17], [-12, 15]]) line(x, 16, x + ex, ey, crack, 0.35);
    fill(x - 5, 11, 11, 10, crack); fill(x - 6, 13, 1, 6, crack); fill(x + 6, 12, 1, 7, crack); fill(x - 3, 10, 6, 1, crack); fill(x - 4, 21, 8, 1, crack);
    fill(x - 4, 12, 9, 1, 0x2a2c34); fill(x - 5, 12, 1, 8, 0x3a3c44, 0.6);
    for (const [dx, dy] of [[-6, 12], [5, 11], [-4, 21], [6, 19], [2, 9]]) dot(x + dx, dy, 0xd8d0c0, 0.5);
    for (const [dx, c] of [[-4, 0x8a8478], [-1, 0xa8a090], [3, 0x8a8478], [5, 0x6e6a60]]) dot(x + dx, 29, c);
    const t = (now + x * 0.37) % 35;
    if (t < 1.8) {
      const out = t < 0.25 ? 1 : t > 1.5 ? 0 : 2;
      fill(x - 4, 12 - out, 9, 9 + out, M1_HULK); fill(x - 4, 12 - out, 9, 1, M1_HULK_LIGHT); fill(x + 4, 12 - out, 1, 9 + out, M1_HULK_DARK);
      for (const k of [-2, 0, 2]) fill(x + k, 13 - out, 1, 4, M1_HULK_DARK);
      fill(x - 4, 17, 9, 1, M1_HULK_DARK); fill(x - 3, 19, 7, 1, M1_HULK_LIGHT, 0.6); fill(x - 5, 20, 11, 1, 0x6a3a8a);
      if (t < 1) for (const [dx, dy] of [[-8, 9], [8, 10], [-9, 18], [9, 20], [0, 6]]) dot(x + dx + Math.sign(dx) * Math.floor(t * 4), dy - Math.floor(t * 3), 0xd8d0c0, 0.8 - t * 0.6);
    }
  },
  /** The Daily Bugle front page in a black frame: a bold masthead, a headline in thick black, and a grainy photo of Spider-Man mid-swing over columns of print; a glint crosses the glass every 26 s. */
  marvel1bugle(x) {
    const frame = 0x1e2026, paper = 0xeae6dc, ink = 0x2a2a30, grey = 0x9a968c;
    fill(x - 10, 4, 21, 25, frame); fill(x - 10, 4, 21, 1, 0x3a3c44); fill(x - 10, 29, 21, 1, SHADOW);
    fill(x - 8, 6, 17, 21, paper); fill(x - 8, 6, 1, 21, WHITE, 0.5);
    // The masthead and its rule, the headline
    fill(x - 7, 7, 15, 3, ink); for (let k = 0; k < 13; k += 2) dot(x - 6 + k, 8, paper, 0.8);
    fill(x - 7, 11, 15, 1, ink, 0.5);
    fill(x - 7, 12, 15, 2, ink); fill(x - 5, 12, 1, 2, paper); fill(x + 1, 12, 1, 2, paper);
    // The photo: grey sky, a web line, the hero
    fill(x - 7, 15, 9, 8, 0xb8b4aa); line(x - 6, 15, x - 3, 19, WHITE, 0.9);
    fill(x - 3, 19, 2, 2, M1_RED); fill(x - 3, 21, 2, 1, M1_BLUE); dot(x - 3, 19, WHITE); dot(x - 2, 19, WHITE, 0.7);
    for (let r = 0; r < 4; r++) fill(x + 3, 15 + r * 2, 5, 1, grey);
    for (let r = 0; r < 2; r++) fill(x - 7, 24 + r * 2, 15, 1, grey);
    const g = (now + x * 0.23) % 26;
    if (g < 1) { const k = Math.floor(g * 18); line(x - 8 + k, 6, x - 12 + k, 26, WHITE, 0.4); }
  },
  /** The Infinity Gauntlet in a wall case (brand nod: Marvel): a gold gauntlet raised on purple velvet behind glass, its six stones glinting one after another; every 30 s they all flare at once, as if about to snap. */
  marvel2gauntlet(x) {
    fill(x - 10, 3, 21, 26, 0x2a2430); fill(x - 10, 3, 21, 1, 0x4a4250); fill(x - 10, 29, 21, 1, SHADOW);
    fill(x - 8, 5, 17, 22, M2_PURPLE_DARK); fill(x - 8, 5, 17, 1, 0x241238);
    // The cuff, the back of the hand and its four raised fingers, the thumb
    fill(x - 3, 19, 7, 6, M2_GOLD); fill(x - 3, 19, 1, 6, M2_GOLD_LIGHT); fill(x + 3, 19, 1, 6, M2_GOLD_DARK); fill(x - 3, 21, 7, 1, M2_GOLD_DARK);
    fill(x - 3, 12, 7, 7, M2_GOLD); fill(x - 3, 12, 1, 7, M2_GOLD_LIGHT); fill(x + 3, 12, 1, 7, M2_GOLD_DARK);
    for (const [fx, top] of [[-3, 8], [-1, 7], [1, 7], [3, 8]]) { fill(x + fx, top, 1, 4, M2_GOLD); dot(x + fx, top, M2_GOLD_LIGHT); }
    fill(x - 5, 13, 2, 4, M2_GOLD); dot(x - 5, 13, M2_GOLD_LIGHT); dot(x - 4, 16, M2_GOLD_DARK);
    const all = (now + x * 0.3) % 30 < 1.4;
    m2Stones([[x - 3, 9], [x - 1, 8], [x + 1, 8], [x + 3, 9], [x - 5, 14], [x, 15]], all);
    // The glass
    fill(x - 8, 5, 17, 22, 0xdff0f6, 0.08); line(x - 6, 7, x - 2, 24, WHITE, 0.25); dot(x + 7, 6, WHITE, 0.6);
    if (isNight) glowLate(() => glowAt(x, 13, 7, 6, all ? 0xffe8a0 : M2_PURPLE, all ? 0.25 : 0.06));
  },
  /** The Sanctum window (brand nod: Marvel): the great round window of the Sanctum Sanctorum, a dark wood rim and three sweeping curved bars meeting in a ring at its heart; sky by day, a warm amber glow from within at night. */
  marvel2sanctumwindow(x) {
    const wood = isNight ? 0x24140a : 0x3a2416, woodLight = 0x5a3a24, cy = 16;
    disc(x, cy, 13, wood); ring(x, cy, 13, woodLight, 0.8);
    disc(x, cy, 11, isNight ? 0x9a6a30 : 0xa8d0f0);
    if (!isNight) { disc(x - 3, cy - 4, 5, 0xc8e4f8, 0.6); } else glowLate(() => glowAt(x, cy, 10, 10, 0xffb050, 0.18));
    // The three sweeping bars, each two wide, and the ring they meet in
    for (let b = 0; b < 3; b++) for (let k = 0; k <= 14; k++) {
      const t = k / 14, r = 3 + t * 8, a = b * ((Math.PI * 2) / 3) + t * 1.7 - 0.4;
      const px = x + Math.round(Math.cos(a) * r), py = cy + Math.round(Math.sin(a) * r);
      dot(px, py, wood); dot(px + (Math.cos(a) > 0 ? -1 : 1), py, wood, 0.8);
    }
    ring(x, cy, 3, wood); ring(x, cy, 11, wood);
    dot(x - 7, cy - 8, WHITE, isNight ? 0.2 : 0.6);
    fill(x - 10, 29, 21, 1, SHADOW, 0.5);
  },
  /** A Kamar-Taj scroll and portal (brand nod: Marvel): a hanging scroll painted with an orange mandala; every 26 s a sling-ring portal sparks open over it onto snowy mountains, holds a few seconds and closes in a spray of sparks. */
  marvel2portal(x) {
    fill(x - 8, 3, 17, 2, 0x5a3a24); dot(x - 9, 4, M2_GOLD); dot(x + 9, 4, M2_GOLD);
    fill(x - 7, 5, 15, 21, 0xeadcc0); fill(x - 7, 5, 1, 21, 0xf8f0dc); fill(x + 7, 5, 1, 21, 0xc8b898);
    ring(x, 14, 5, M2_SPELL, 0.85); ring(x, 14, 3, M2_SPELL, 0.6); dot(x, 14, M2_SPELL);
    for (let k = 0; k < 8; k++) { const a = (k / 8) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 6), 14 + Math.round(Math.sin(a) * 6), 0xc8602a, 0.7); }
    for (const ry of [22, 24]) fill(x - 5, ry, 11, 1, 0x8a6a4a, 0.6);
    fill(x - 8, 26, 17, 2, 0x5a3a24);
    const t = (now + x * 0.31) % 26;
    if (t < 4.5) {
      const r = t < 0.6 ? 2 + Math.floor(t * 12) : t > 3.9 ? Math.max(1, 9 - Math.floor((t - 3.9) * 14)) : 9;
      const scene = () => {
        disc(x, 15, r - 1, 0xb8d8f0);
        if (r > 4) { for (const [dx, h] of [[-5, 3], [-2, 6], [1, 4], [4, 5]]) fill(x + dx, 15 + Math.max(0, r - 1 - h) - 2, 3, h, 0x6a7a90); for (const dx of [-2, 4]) dot(x + dx, 15 + r - 7, WHITE); fill(x - r + 2, 15 + r - 3, (r - 2) * 2, 2, 0xf0f4f8); }
        m2Sigil(x, 15, r);
      };
      if (isNight) glowLate(scene); else scene();
    }
  },
  /** The Awesome Mix tape (brand nod: Marvel): Star-Lord's cassette framed on the wall, a cream and orange label with a scrawled title, two reels behind a window; every 15 s the reels spin a while as if the song had come on. */
  marvel2awesomemix(x) {
    fill(x - 10, 5, 21, 17, 0x1e1a22); fill(x - 10, 5, 21, 1, 0x3a3440); fill(x - 10, 22, 21, 1, SHADOW);
    fill(x - 8, 7, 17, 13, 0x2a2c34); fill(x - 8, 7, 17, 1, 0x4a4e58); fill(x + 8, 8, 1, 12, 0x14161c);
    fill(x - 7, 8, 15, 7, 0xf0e0b0); fill(x - 7, 8, 15, 2, M2_ORANGE); fill(x - 7, 10, 15, 1, 0xd8582a);
    for (let k = 0; k < 11; k += 2) dot(x - 5 + k, 9, 0x5a2a1a, 0.8);
    fill(x - 4, 12, 9, 3, 0x3a3440); fill(x - 4, 12, 9, 1, 0x24202a);
    const spin = (now + x * 0.29) % 15 < 5 ? Math.floor(now * 6) % 2 : 0;
    for (const rx of [-2, 3]) { disc(x + rx, 13, 1, WHITE, 0.9); dot(x + rx + (spin ? 1 : 0), 13 - (spin ? 0 : 1), 0x2a2c34); }
    fill(x - 5, 17, 11, 2, 0x3a3e48); for (const hx of [-3, 3]) dot(x + hx, 18, 0x14161c);
    dot(x - 7, 8, WHITE, 0.5);
  },
  /** A Black Panther tapestry (brand nod: Marvel): a long purple-black cloth on a rod with the panther's silver-lined mask woven in, white eye slits and pointed ears, a silver fringe; every 18 s violet kinetic light runs down its lines. */
  marvel2tapestry(x) {
    fill(x - 8, 3, 17, 1, M2_SILVER); dot(x - 9, 3, 0xa8acb8); dot(x + 9, 3, 0xa8acb8);
    fill(x - 7, 4, 15, 22, 0x4a2a72); fill(x - 7, 4, 1, 22, 0x6a4a92); fill(x + 7, 4, 1, 22, 0x2a1848); for (let k = 6; k < 26; k += 4) fill(x - 6, k, 13, 1, 0x5a3a82, 0.5);
    for (let k = 0; k < 15; k += 2) { dot(x - 7 + k, 26, M2_SILVER); dot(x - 7 + k, 27, M2_SILVER, 0.6); }
    // The mask: head, ears, the silver lines and the eyes
    fill(x - 4, 9, 9, 10, M2_PANTHER); fill(x - 3, 19, 7, 2, M2_PANTHER); fill(x - 2, 21, 5, 1, M2_PANTHER);
    dot(x - 4, 8, M2_PANTHER); dot(x - 4, 7, M2_PANTHER); dot(x + 4, 8, M2_PANTHER); dot(x + 4, 7, M2_PANTHER);
    const glow = (now + x * 0.27) % 18 < 2, lineC = glow ? M2_VIBE : M2_SILVER;
    line(x, 9, x, 18, lineC, 0.7); line(x - 4, 10, x - 1, 14, lineC, 0.6); line(x + 4, 10, x + 1, 14, lineC, 0.6);
    fill(x - 3, 13, 2, 1, WHITE); fill(x + 2, 13, 2, 1, WHITE); dot(x - 3, 12, WHITE, 0.6); dot(x + 3, 12, WHITE, 0.6);
    fill(x - 1, 19, 3, 1, lineC, 0.6);
    if (glow) glowAt(x, 14, 6, 7, M2_VIBE, 0.1);
  },
  /** Loki's helmet (brand nod: Marvel): his gold horned helmet mounted on a green shield-shaped plaque, the long horns sweeping up and out; a glint runs up a horn every 20 s, and a faint green glow at night. */
  marvel2lokihelm(x) {
    fill(x - 7, 6, 15, 18, M2_LOKI); fill(x - 7, 6, 15, 1, M2_LOKI_LIGHT); fill(x - 7, 6, 1, 18, M2_LOKI_LIGHT); fill(x + 7, 6, 1, 18, M2_LOKI_DARK);
    for (let k = 0; k < 4; k++) fill(x - 6 + k * 2, 24 + k, 13 - k * 4, 1, k === 3 ? M2_LOKI_DARK : M2_LOKI);
    fill(x - 7, 6, 15, 1, M2_GOLD_DARK, 0.5);
    // The helmet: dome, cheek guards, the long horns
    fill(x - 3, 13, 7, 5, M2_GOLD); fill(x - 2, 12, 5, 1, M2_GOLD); fill(x - 3, 13, 7, 1, M2_GOLD_LIGHT); fill(x + 3, 14, 1, 4, M2_GOLD_DARK);
    fill(x - 3, 18, 2, 4, M2_GOLD); fill(x + 2, 18, 2, 4, M2_GOLD_DARK); fill(x - 1, 18, 3, 3, 0x14261a);
    for (const side of [-1, 1]) {
      const pts = [[3, 13], [4, 12], [5, 11], [6, 9], [7, 7], [8, 5], [9, 3], [9, 2]];
      pts.forEach(([dx, dy], i) => { dot(x + side * dx, dy, side < 0 ? M2_GOLD_LIGHT : M2_GOLD); if (i < 5) dot(x + side * dx, dy + 1, M2_GOLD_DARK); });
    }
    const g = (now + x * 0.33) % 20;
    if (g < 1) { const k = Math.min(7, Math.floor(g * 8)); const pts = [[3, 13], [4, 12], [5, 11], [6, 9], [7, 7], [8, 5], [9, 3], [9, 2]]; dot(x - pts[k][0], pts[k][1], WHITE); }
    if (isNight) glowLate(() => glowAt(x, 15, 9, 9, M2_LOKI_LIGHT, 0.08));
  },
  /** The Hall of Armour (brand nod: Marvel): three suits standing in lit glass niches set into the wall, silver, red and gold, and War Machine grey; their eyes glow, and every 28 s the niche lights run along them one by one. */
  ironmanarmory(x) {
    fill(x - 15, 3, 31, 26, IM_GRAPHITE); fill(x - 15, 3, 31, 1, 0x4a4e58); fill(x - 15, 29, 31, 1, SHADOW);
    const t = (now + x * 0.3) % 28;
    [[-10, [IM_STEEL_LIGHT, IM_STEEL]], [0, [IM_RED, IM_GOLD]], [10, [0x5a5e68, 0x3a3e48]]].forEach(([dx, [body, trim]], i) => {
      const lit = t < 3 && Math.floor(t * 3) % 3 === i;
      fill(x + dx - 4, 5, 9, 22, 0x14161c); fill(x + dx - 4, 5, 9, 1, IM_HOLO, lit ? 1 : 0.5);
      if (lit) glowAt(x + dx, 15, 4, 9, IM_HOLO, 0.12);
      imSuit(x + dx - 2, 9, body, trim);
      fill(x + dx - 3, 24, 7, 1, IM_HOLO, 0.4);
      fill(x + dx - 4, 5, 9, 22, 0xdff0f6, 0.06);
    });
    if (isNight) glowLate(() => { for (const dx of [-10, 0, 10]) { dot(x + dx - 1, 10, IM_ARC_WHITE); dot(x + dx + 1, 10, IM_ARC_WHITE); fill(x + dx - 3, 5, 7, 1, IM_HOLO, 0.6); } });
  },
  /** A suit blueprint (brand nod: Marvel): a holographic panel on the wall, a blue wireframe of the armour with callout lines and figures; a scan line sweeps down it every few seconds and it glows at night. */
  ironmanblueprint(x) {
    const c = IM_HOLO;
    fill(x - 12, 4, 25, 24, 0x0e1a2a, 0.85); fill(x - 12, 4, 25, 1, c, 0.8); fill(x - 12, 27, 25, 1, c, 0.5); fill(x - 12, 4, 1, 24, c, 0.5); fill(x + 12, 4, 1, 24, c, 0.5);
    for (let k = -10; k < 12; k += 4) fill(x + k, 6, 1, 21, c, 0.08);
    // The wireframe: helmet, chest and reactor, arms, legs
    ring(x - 3, 9, 2, c, 0.8); fill(x - 6, 12, 7, 6, c, 0.15); for (const [x0, y0, x1, y1] of [[-6, 12, 0, 12], [-6, 12, -6, 17], [0, 12, 0, 17], [-7, 12, -9, 18], [1, 12, 3, 18], [-5, 18, -5, 25], [-1, 18, -1, 25]]) line(x + x0, y0, x + x1, y1, c, 0.8);
    dot(x - 3, 14, IM_ARC_WHITE);
    // Callouts
    line(x - 1, 9, x + 4, 7, c, 0.6); fill(x + 4, 7, 6, 1, c, 0.6); line(x - 3, 14, x + 4, 15, c, 0.6); fill(x + 4, 15, 5, 1, c, 0.6); fill(x + 4, 21, 6, 1, c, 0.4); fill(x + 4, 23, 4, 1, c, 0.4);
    const scan = 5 + Math.floor((now * 6 + x) % 22);
    fill(x - 11, scan, 23, 1, IM_ARC_WHITE, 0.35);
    if (isNight) glowLate(() => glowAt(x, 16, 12, 12, c, 0.06));
  },
  /** The Mark I mask (brand nod: Marvel): the crude first helmet built in a cave, dull grey iron riveted together with a slot for the eyes and a grille for a mouth, on a rough plank shelf; the eye slot glints every 24 s. */
  ironmanmark1(x) {
    const iron = 0x6a6a6e, ironLight = 0x8a8a90, ironDark = 0x45454a;
    fill(x - 8, 24, 17, 2, 0x6a4a2e); fill(x - 8, 24, 17, 1, 0x8a6440); fill(x - 8, 26, 17, 1, SHADOW); fill(x - 6, 26, 1, 2, 0x4a3020); fill(x + 6, 26, 1, 2, 0x4a3020);
    fill(x - 5, 9, 11, 15, iron); fill(x - 4, 8, 9, 1, iron); fill(x - 5, 9, 1, 15, ironLight); fill(x + 5, 9, 1, 15, ironDark); fill(x - 4, 8, 9, 1, ironLight);
    fill(x - 5, 15, 11, 1, ironDark); fill(x, 8, 1, 7, ironDark, 0.6);
    for (const [dx, dy] of [[-4, 10], [4, 10], [-4, 17], [4, 17], [-4, 22], [4, 22], [0, 9]]) dot(x + dx, dy, ironLight);
    fill(x - 3, 13, 7, 1, 0x14141a); fill(x - 2, 19, 5, 3, 0x2a2a30); for (const gx of [-2, 0, 2]) fill(x + gx, 19, 1, 3, ironDark);
    if ((now + x * 0.3) % 24 < 0.8) dot(x - 2 + Math.floor(((now + x * 0.3) % 24) * 8), 13, WHITE, 0.8);
  },
  /** A Stark Expo poster (brand nod: Marvel): a deep blue poster with a steel ring globe over a gold sunburst and two lines of gold lettering; a glint runs over the globe every 20 s. */
  ironmanexpo(x) {
    fill(x - 8, 3, 17, 25, 0x14284a); fill(x - 8, 3, 17, 1, 0x24406a); fill(x - 8, 28, 17, 1, SHADOW);
    for (let k = 0; k < 8; k++) { const a = (k / 8) * Math.PI * 2; line(x, 12, x + Math.round(Math.cos(a) * 7), 12 + Math.round(Math.sin(a) * 7), IM_GOLD, 0.35); }
    ring(x, 12, 5, IM_STEEL_LIGHT); ellipseRing(x, 12, 5, 2, IM_STEEL, 0.9); fill(x, 7, 1, 11, IM_STEEL, 0.9); fill(x - 5, 12, 11, 1, IM_STEEL, 0.6);
    fill(x - 1, 18, 3, 2, IM_STEEL_DARK); fill(x - 3, 20, 7, 1, IM_STEEL_DARK);
    fill(x - 6, 23, 13, 1, IM_GOLD); fill(x - 4, 25, 9, 1, IM_GOLD_LIGHT, 0.8);
    const g = (now + x * 0.29) % 20; if (g < 1) dot(x - 4 + Math.floor(g * 8), 10, WHITE);
  },
  /** Proof that Tony has a heart (brand nod: Marvel): his old arc reactor mounted in a glass display box on a black stand, a little engraved plate below it; it glows blue and pulses like a heartbeat every few seconds. */
  ironmanheart(x) {
    fill(x - 7, 5, 15, 20, 0x1a1a1f); fill(x - 7, 5, 15, 1, 0x3a3a40); fill(x - 7, 25, 15, 1, SHADOW);
    fill(x - 6, 6, 13, 18, 0xdff0f6, 0.12); fill(x - 6, 6, 1, 18, 0xdff0f6, 0.3); dot(x + 5, 7, WHITE, 0.7);
    fill(x - 4, 17, 9, 2, 0x2a2a30);
    const beat = (now * 1.1 + x * 0.1) % 4, pulse = beat < 0.2 || (beat > 0.35 && beat < 0.55) ? 1 : 0.5;
    imReactor(x, 12, 4, pulse);
    fill(x - 4, 21, 9, 2, IM_GOLD); fill(x - 3, 21, 7, 1, IM_GOLD_LIGHT); fill(x - 2, 22, 5, 1, IM_GOLD_DARK);
  },
  /** A workshop tool wall (brand nod: Marvel): a pegboard hung with wrenches, a hammer, pliers and a spare red and gold gauntlet; now and then the gauntlet's palm flickers blue. */
  ironmanpegboard(x) {
    fill(x - 14, 4, 29, 23, 0x8a6a4a); fill(x - 14, 4, 29, 1, 0xa8845a); fill(x - 14, 27, 29, 1, SHADOW);
    for (let r = 6; r < 26; r += 3) for (let c = -12; c < 14; c += 3) dot(x + c, r, 0x5a4430, 0.7);
    // Wrenches, a hammer, pliers
    for (const [dx, h] of [[-11, 12], [-9, 10], [-7, 8]]) { fill(x + dx, 7, 1, h, IM_STEEL); dot(x + dx - 1, 7, IM_STEEL); dot(x + dx + 1, 7, IM_STEEL); dot(x + dx, 7 + h, IM_STEEL_LIGHT); }
    fill(x - 4, 8, 4, 2, IM_STEEL_DARK); fill(x - 3, 10, 1, 9, 0xd8402a); fill(x - 3, 10, 1, 2, IM_STEEL);
    line(x + 1, 8, x + 3, 16, IM_STEEL); line(x + 3, 8, x + 1, 16, IM_STEEL); fill(x + 1, 16, 1, 3, 0xf2c030); fill(x + 3, 16, 1, 3, 0xf2c030);
    // The gauntlet, fingers down
    fill(x + 7, 7, 5, 6, IM_RED); fill(x + 7, 7, 5, 1, IM_GOLD); fill(x + 7, 7, 1, 6, IM_RED_LIGHT);
    fill(x + 6, 13, 7, 5, IM_GOLD); fill(x + 6, 13, 1, 5, IM_GOLD_LIGHT); for (const fx of [7, 9, 11]) fill(x + fx, 18, 1, 3, IM_GOLD_DARK);
    const zap = (now + x * 0.29) % 16 < 1;
    dot(x + 9, 15, zap ? IM_ARC_WHITE : IM_ARC); if (zap) glowAt(x + 9, 15, 3, 3, IM_ARC, 0.25);
  },
  /** The Bat-Signal (brand nod: DC): a window onto Gotham's skyline under low cloud; by night the signal's beam climbs from a rooftop and the bat shines on the clouds, flickering now and then; by day, grey sky and dark towers. */
  batcavesignal(x) {
    const frame = 0x22242a;
    fill(x - 12, 3, 25, 26, frame); fill(x - 12, 3, 25, 1, 0x3a3c44); fill(x - 12, 3, 1, 26, 0x3a3c44);
    fill(x - 11, 4, 23, 24, isNight ? 0x141a2a : 0x8a96a6);
    for (const [cx, cy, r] of [[-6, 6, 2], [2, 5, 3], [8, 7, 2]]) disc(x + cx, cy, r, isNight ? 0x222a3e : 0xb8c2d0, 0.6);
    // Gotham's towers, gothic spires, lit windows at night
    for (const [dx, top, w] of [[-11, 17, 4], [-7, 12, 3], [-4, 19, 5], [1, 14, 3], [4, 10, 4], [8, 18, 4]]) {
      fill(x + dx, top, w, 28 - top, isNight ? 0x0c0e16 : 0x3a3e4a); dot(x + dx + (w >> 1), top - 1, isNight ? 0x0c0e16 : 0x3a3e4a);
      if (isNight) for (let wy = top + 2; wy < 27; wy += 3) if (hash(x + dx, wy) % 3 === 0) dot(x + dx + 1, wy, 0xffd070, 0.8);
    }
    if (isNight) {
      const flick = (now + x * 0.3) % 17 < 0.3 ? 0.4 : 1;
      glowLate(() => {
        for (let k = 0; k < 10; k++) fill(x + 5 - Math.round(k * 0.4), 10 + k, 2 + Math.round(k * 0.3), 1, 0xfff4c0, 0.12 * flick);
        fill(x - 1, 5, 9, 5, 0xfff4c0, 0.75 * flick); fill(x - 2, 6, 11, 3, 0xfff4c0, 0.75 * flick); bcBat(x + 3, 7, BC_BLACK, 0.9);
        glowAt(x + 3, 7, 7, 4, 0xfff4c0, 0.15 * flick);
      });
    }
    fill(x - 12, 15, 25, 1, frame); fill(x, 3, 1, 26, frame);
    fill(x - 12, 28, 25, 1, frame); fill(x - 13, 29, 27, 1, PALETTE.sill);
  },
  /** The suit case (brand nod: DC): a tall lit glass case set into the rock, the grey batsuit standing in it with its cowl, black bat and yellow belt, the cape hanging behind; a light runs up the case every 24 s. */
  batcavesuits(x) {
    fill(x - 9, 2, 19, 28, BC_STONE_DARK); fill(x - 9, 2, 19, 1, BC_STONE_LIGHT); fill(x - 9, 30, 19, 1, SHADOW);
    fill(x - 8, 3, 17, 26, 0x14161c); fill(x - 8, 3, 17, 1, BC_SCREEN, 0.6);
    // The cape behind, the suit, the cowl
    fill(x - 6, 9, 13, 18, BC_BLACK); fill(x - 7, 12, 1, 14, BC_BLACK); fill(x + 7, 12, 1, 14, BC_BLACK);
    fill(x - 3, 9, 7, 9, BC_GREY); fill(x - 3, 9, 1, 9, BC_GREY_LIGHT); fill(x - 5, 10, 2, 7, BC_GREY); fill(x + 4, 10, 2, 7, 0x4a4e58);
    bcBat(x, 11, BC_BLACK); fill(x - 3, 17, 7, 1, BC_YELLOW); dot(x, 17, BC_YELLOW_DARK);
    fill(x - 3, 18, 3, 7, 0x3a3c46); fill(x + 1, 18, 3, 7, 0x2e3038); fill(x - 3, 24, 3, 2, BC_BLACK); fill(x + 1, 24, 3, 2, BC_BLACK);
    fill(x - 2, 4, 5, 5, BC_BLACK); dot(x - 2, 3, BC_BLACK); dot(x + 2, 3, BC_BLACK); fill(x - 1, 6, 1, 1, WHITE); fill(x + 1, 6, 1, 1, WHITE); fill(x - 1, 8, 3, 1, 0xc89070);
    const t = (now + x * 0.29) % 24; if (t < 1.4) fill(x - 8, 27 - Math.floor(t * 18), 17, 1, BC_SCREEN_LIGHT, 0.5);
    fill(x - 8, 3, 17, 26, 0xdff0f6, 0.05); line(x - 6, 5, x - 3, 27, WHITE, 0.15);
    if (isNight) glowLate(() => { fill(x - 8, 3, 17, 1, BC_SCREEN); glowAt(x, 14, 7, 11, BC_SCREEN, 0.06); fill(x - 1, 6, 1, 1, WHITE); fill(x + 1, 6, 1, 1, WHITE); });
  },
  /** Stalactites and bats: rock spikes hanging from the cave roof with a colony of little bats roosting upside down among them; they twitch their wings now and then, and every 30 s two drop off and flutter round before roosting again. */
  batcavestalactites(x) {
    for (const [dx, len, w] of [[-12, 9, 3], [-7, 14, 4], [-2, 7, 3], [3, 16, 4], [8, 10, 3], [12, 6, 2]]) {
      for (let k = 0; k < len; k++) { const ww = Math.max(1, Math.round(w * (1 - k / len))); fill(x + dx - (ww >> 1), k, ww, 1, k < 2 ? 0x7a7c86 : 0x5a5c66); dot(x + dx - (ww >> 1), k, 0x8a8c96); }
      dot(x + dx, len, 0x3a4a5a, 0.6);
    }
    const t = (now + x * 0.27) % 30, out = t < 4;
    [[-9, 3], [-4, 5], [1, 4], [6, 6], [10, 4]].forEach(([dx, dy], i) => {
      if (out && i % 2 === 1) return;
      const twitch = Math.floor(now * 2 + i * 3) % 9 === 0;
      fill(x + dx - 1, dy, 3, 3, BC_BLACK); dot(x + dx, dy + 3, BC_BLACK); if (twitch) { dot(x + dx - 2, dy + 1, BC_BLACK); dot(x + dx + 2, dy + 1, BC_BLACK); }
      dot(x + dx - 1, dy - 1, BC_BLACK); dot(x + dx + 1, dy - 1, BC_BLACK);
    });
    if (out) for (const [i, ph] of [[0, 0], [1, 1.7]]) { const a = now * 3 + ph; bcFlyer(x + Math.round(Math.cos(a) * 9), 14 + Math.round(Math.sin(a * 1.3) * 6), Math.floor(now * 8 + i) % 2); }
  },
  /** A case board: a cork board crowded with photos, clippings and a city map, red string pinned between them, a Joker playing card and a question mark among the clues; a pin's light blinks on the latest lead. */
  batcaveboard(x) {
    fill(x - 14, 4, 29, 22, 0x8a6a48); fill(x - 14, 4, 29, 1, 0xa8845a); fill(x - 14, 4, 1, 22, 0xa8845a); fill(x + 14, 4, 1, 22, 0x5a4430); fill(x - 14, 26, 29, 1, SHADOW);
    fill(x - 4, 9, 11, 9, 0xc8d0b8); for (const [x0, y0, x1, y1] of [[-3, 11, 6, 11], [-1, 9, -1, 17], [3, 13, 6, 16]]) line(x + x0, y0, x + x1, y1, 0x8a9a7a);
    const notes = [[-12, 6, 5, 6, 0xeae6dc], [-12, 15, 6, 5, 0xd8d0c0], [8, 6, 5, 6, 0xeae6dc], [9, 15, 4, 6, 0xf2f2ee], [-6, 19, 5, 5, 0xeae6dc]];
    for (const [dx, dy, w, h, c] of notes) { fill(x + dx, dy, w, h, c); fill(x + dx, dy + h - 1, w, 1, 0xb8b4aa); }
    fill(x - 11, 7, 3, 3, 0x6a6e78); fill(x + 9, 7, 3, 3, 0x6a6e78); dot(x - 10, 16, 0x2a2a30); fill(x - 11, 17, 4, 1, 0x9a968c);
    fill(x + 10, 16, 2, 4, WHITE); dot(x + 10, 16, 0xd84040); dot(x + 11, 18, 0x3a8a3a);
    line(x - 4, 21, x - 3, 21, 0x3a7a3a); dot(x - 4, 20, 0x3a7a3a); dot(x - 3, 22, 0x3a7a3a);
    const red = 0xd82a2a;
    for (const [x0, y0, x1, y1] of [[-10, 7, 1, 13], [1, 13, 10, 7], [1, 13, -9, 17], [1, 13, 10, 17], [-9, 17, -4, 21]]) line(x + x0, y0, x + x1, y1, red, 0.8);
    for (const [px, py] of [[-10, 7], [10, 7], [-9, 17], [10, 17], [-4, 21]]) dot(x + px, py, 0xe8c040);
    if (Math.floor(now * 2) % 2) dot(x + 1, 13, 0xff4040);
  },
  /** The giant Joker card (brand nod: DC): a huge playing card propped against the rock, cream with a green and purple border, a grinning jester's face in the middle and a corner index; every 25 s its grin twitches wider. */
  batcavecard(x) {
    const green = 0x3a9a4a, purple = 0x7a3aa8;
    fill(x - 8, 3, 17, 26, 0x2a2a30, 0.5);
    fill(x - 8, 2, 17, 26, 0xf2ecdc); fill(x - 8, 2, 17, 1, WHITE); fill(x + 8, 3, 1, 25, 0xc8c0aa); fill(x - 8, 27, 17, 1, 0xc8c0aa);
    fill(x - 6, 4, 13, 22, green, 0.0); for (let k = 0; k < 13; k++) { dot(x - 6 + k, 4, k % 2 ? purple : green); dot(x - 6 + k, 25, k % 2 ? green : purple); }
    for (let k = 0; k < 21; k++) { dot(x - 6, 4 + k, k % 2 ? purple : green); dot(x + 6, 4 + k, k % 2 ? green : purple); }
    // The jester: a two-pointed hat, a white face, red grin
    fill(x - 3, 10, 7, 7, WHITE); fill(x - 3, 10, 1, 7, 0xe8e4dc);
    line(x - 3, 9, x - 5, 6, purple); line(x + 3, 9, x + 5, 6, green); fill(x - 3, 9, 7, 1, purple); dot(x - 5, 5, 0xf2d02a); dot(x + 5, 5, 0xf2d02a);
    dot(x - 2, 12, BC_BLACK); dot(x + 2, 12, BC_BLACK); dot(x - 2, 11, green); dot(x + 2, 11, green);
    const wide = (now + x * 0.31) % 25 < 1.5 ? 1 : 0;
    fill(x - 2 - wide, 15, 5 + wide * 2, 1, 0xd82a2a); dot(x - 3 - wide, 14, 0xd82a2a); dot(x + 3 + wide, 14, 0xd82a2a);
    fill(x - 2, 18, 5, 4, purple); fill(x - 1, 18, 3, 1, green);
    fill(x - 6, 5, 2, 3, BC_BLACK, 0.0); dot(x - 5, 6, 0xd82a2a); dot(x - 5, 7, 0xd82a2a);
    dot(x - 7, 3, WHITE, 0.6);
  },
  /** A batarang rack (brand nod: DC): a black steel rack holding five batarangs in a fan above a yellow-lit shelf; every 20 s a glint runs across their edges. */
  batcavebatarangs(x) {
    fill(x - 13, 5, 27, 20, BC_BLACK); fill(x - 13, 5, 27, 1, 0x2e3038); fill(x - 13, 25, 27, 1, SHADOW);
    fill(x - 12, 22, 25, 2, 0x2e3038); fill(x - 12, 22, 25, 1, BC_YELLOW, 0.6);
    const g = Math.floor(((now + x * 0.3) % 20) * 4);
    [[-8, 9], [-4, 15], [0, 9], [4, 15], [8, 9]].forEach(([dx, dy], i) => {
      bcBat(x + dx, dy, 0x6e727c);
      for (let k = -4; k <= 4; k += 4) dot(x + dx + k, dy - 1, 0x9aa0aa);
      if (g === i) { dot(x + dx - 3, dy, WHITE); dot(x + dx + 3, dy, WHITE, 0.7); }
    });
    for (const dx of [-8, 0, 8]) dot(x + dx, 12, 0x3a3c46);
  },
  /** The Daily Planet globe (brand nod: DC): the newspaper's gold globe sign mounted on the wall, its continents picked out and a ring tilted round it on which a sparkle turns; lit warm at night. */
  dcmixplanetglobe(x) {
    fill(x - 1, 26, 3, 3, DC_GOLD_DARK); fill(x - 4, 28, 9, 1, DC_GOLD_DARK);
    disc(x, 15, 8, DC_GOLD_DARK); disc(x, 15, 7, DC_GOLD); disc(x - 2, 12, 4, 0xf8d880, 0.6);
    for (const [dx, dy, w] of [[-4, 11, 3], [-5, 13, 4], [-3, 15, 3], [1, 10, 4], [2, 12, 3], [1, 17, 3], [-2, 19, 2]]) fill(x + dx, dy, w, 1, DC_GOLD_DARK, 0.8);
    ellipseRing(x, 15, 12, 3, DC_GOLD); ellipseRing(x, 15, 12, 3, WHITE, 0.15);
    const a = now * 1.2 + x; dot(x + Math.round(Math.cos(a) * 12), 15 + Math.round(Math.sin(a) * 3), WHITE);
    if (isNight) glowLate(() => glowAt(x, 15, 10, 9, 0xffd870, 0.12));
  },
  /** The S shield (brand nod: DC): Superman's shield as a big plaque on the wall, red-rimmed yellow with the red S, on a blue backing; a crystal-white glint crosses it every 20 s. */
  dcmixshield(x) {
    fill(x - 8, 4, 17, 21, DC_BLUE_DARK); fill(x - 8, 4, 17, 1, DC_BLUE); fill(x - 8, 25, 17, 1, SHADOW);
    // The shield from a map: R red, Y yellow; its rim, the S in the middle, narrowing to the point
    const map = ['RRRRRRRRRRRRRRR', 'RYYYYYYYYYYYYYR', 'RYYRRRRRRRRRYYR', 'RYRRYYYYYYYRRYR', '.RRRYYYYYYYYYR.', '.RYRRRRRRRRYYR.', '..RYYYYYYYRRR..',
      '..RYYYYYYYYRR..', '...RRRRRRRRR...', '....RYYYYYR....', '.....RYYYR.....', '......RYR......', '.......R.......'];
    map.forEach((row, r) => { for (let k = 0; k < 15; k++) if (row[k] !== '.') fill(x - 7 + k, 6 + r * 1.5 | 0, 1, r === 12 ? 1 : 2, row[k] === 'R' ? DC_RED : DC_YELLOW); });
    fill(x - 7, 6, 15, 1, DC_RED_LIGHT, 0.6);
    const g = (now + x * 0.29) % 20; if (g < 1) line(x - 7 + Math.floor(g * 16), 6, x - 10 + Math.floor(g * 16), 18, WHITE, 0.6);
  },
  /** The Lasso of Truth and the tiara (brand nod: DC): Wonder Woman's golden lasso coiled on a peg, glowing softly, her silver-gold tiara with its red star hung above; the lasso's glow swells every 14 s. */
  dcmixlasso(x) {
    fill(x - 1, 4, 3, 2, 0x8a6a3a); dot(x, 3, 0xa88a5a);
    fill(x - 6, 7, 13, 3, DC_GOLD); fill(x - 6, 7, 13, 1, 0xf8d880); fill(x - 1, 6, 3, 2, DC_GOLD); dot(x, 7, DC_RED); dot(x - 1, 8, DC_RED); dot(x + 1, 8, DC_RED);
    const glow = 0.5 + 0.5 * Math.sin(((now + x * 0.3) % 14) / 14 * Math.PI * 2);
    for (const [ry, rx] of [[18, 7], [19, 6], [20, 7]]) ellipseRing(x, ry, rx, 4, DC_GOLD);
    line(x, 11, x - 2, 15, DC_GOLD); line(x + 6, 21, x + 7, 27, DC_GOLD); dot(x + 7, 28, DC_GOLD_DARK);
    glowAt(x, 19, 8, 5, 0xffe070, 0.06 + glow * 0.1);
    if (isNight) glowLate(() => { glowAt(x, 19, 9, 6, 0xffe070, 0.12 + glow * 0.15); ellipseRing(x, 19, 6, 4, 0xfff0a0, 0.6); });
  },
  /** The Flash running by (brand nod: DC): a window onto Central City's bright towers; every 16 s a red blur streaks across it trailing yellow lightning, gone in an instant, and the panes rattle. */
  dcmixflashwindow(x) {
    const frame = 0xd8dce4, t = (now + x * 0.29) % 16, rattle = t > 0.5 && t < 0.9 ? Math.floor(now * 20) % 2 : 0;
    fill(x - 12, 3, 25, 26, frame); fill(x - 12, 3, 25, 1, WHITE); fill(x + 12, 3, 1, 26, 0xa8acb6);
    fill(x - 11, 4, 23, 24, isNight ? 0x101c38 : 0x8ac8f8); if (!isNight) fill(x - 11, 4, 23, 6, 0xb0dcfc);
    for (const [dx, top, w] of [[-11, 13, 4], [-6, 9, 4], [-1, 15, 3], [3, 7, 4], [8, 12, 4]]) {
      fill(x + dx, top, w, 28 - top, isNight ? 0x1a2848 : 0xc8d4e4); fill(x + dx, top, 1, 28 - top, isNight ? 0x24345a : WHITE, 0.7);
      for (let wy = top + 2; wy < 27; wy += 2) if (hash(x + dx, wy) % 3 === 0) dot(x + dx + 1, wy, isNight ? 0xffe08a : 0x8aa0c0, 0.7);
    }
    if (t < 0.5) {
      const sx = x - 12 + Math.floor(t * 52);
      line(x - 11, 20, Math.min(x + 11, sx), 20, DC_RED, 0.8); fill(Math.min(x + 10, sx), 18, 2, 4, DC_RED);
      for (let k = x - 11; k < Math.min(x + 11, sx); k += 3) dot(k, 19 + (k % 2) * 2, DC_YELLOW);
    } else if (t < 1.2) for (let k = -10; k < 11; k += 4) dot(x + k, 20 + (k % 3), DC_YELLOW, 1.2 - t);
    fill(x - 12, 15, 25, 1, frame); fill(x + rattle, 3, 1, 26, frame);
    fill(x - 12, 28, 25, 1, frame); fill(x - 13, 29, 27, 1, PALETTE.sill);
  },
  /** Aquaman's trident (brand nod: DC): the golden five-pronged trident mounted across a sea-green plaque, water light rippling over the plaque; every 22 s its tines shimmer as if a wave had run over them. */
  dcmixtrident(x) {
    fill(x - 10, 5, 21, 21, 0x1a5a5a); fill(x - 10, 5, 21, 1, 0x2a7a7a); fill(x - 10, 5, 1, 21, 0x2a7a7a); fill(x + 10, 5, 1, 21, 0x0e3a3a); fill(x - 10, 26, 21, 1, SHADOW);
    for (let k = 0; k < 4; k++) { const wy = 8 + k * 5, sh = Math.round(Math.sin(now * 1.2 + k) * 2); line(x - 8 + sh, wy, x - 2 + sh, wy + 1, 0x4ab8b0, 0.4); line(x + 2 - sh, wy + 2, x + 8 - sh, wy + 1, 0x4ab8b0, 0.4); }
    line(x - 8, 24, x + 4, 8, DC_GOLD); line(x - 7, 24, x + 5, 8, DC_GOLD_DARK);
    const shimmer = (now + x * 0.3) % 22 < 1.2;
    const head = [[3, 8, 2, 4], [5, 7, 7, 3], [6, 9, 9, 6], [4, 10, 8, 10], [2, 9, 3, 5]];
    for (const [x0, y0, x1, y1] of head) line(x + x0, y0, x + x1, y1, shimmer ? WHITE : DC_GOLD);
    fill(x + 2, 8, 4, 3, DC_GOLD); dot(x + 3, 9, 0x2ab8d8);
    if (shimmer) glowAt(x + 6, 6, 4, 4, 0x8affff, 0.2);
  },
  /** A Harley Quinn banner (brand nod: DC): a hanging banner in red and black diamonds with a white jester's diamond in the middle, a little mallet hooked over its rod; it gives a cheeky swing every 18 s. */
  dcmixharley(x) {
    fill(x - 9, 3, 19, 1, 0x3a3a40); dot(x - 9, 2, 0xc8c8cc); dot(x + 9, 2, 0xc8c8cc);
    const swing = (now + x * 0.27) % 18 < 1.5 ? Math.round(Math.sin(now * 10)) : 0;
    for (let r = 0; r < 20; r++) for (let c = 0; c < 13; c++) {
      const d = ((r >> 2) + (c >> 2)) % 2;
      dot(x - 6 + c + (r > 14 ? swing : 0), 4 + r, d ? DC_RED : 0x1a1a1f);
    }
    for (let k = 0; k < 4; k++) fill(x - 6 + k * 2 + swing, 24 + k, 13 - k * 4, 1, k % 2 ? DC_RED : 0x1a1a1f);
    for (let k = 0; k < 4; k++) { fill(x - k, 10 + k, 1 + k * 2, 1, WHITE); fill(x - k, 17 - k, 1 + k * 2, 1, WHITE); }
    dot(x, 13, DC_RED); dot(x, 14, 0x1a1a1f);
    fill(x + 7, 1, 4, 3, 0xd8282a); fill(x + 7, 1, 1, 3, 0x1a1a1f); fill(x + 8, 4, 1, 6, 0x8a5a3a);
  },
  /** A gym badge case: a wooden case lined with blue felt holding the eight Kanto badges in two rows, Boulder to Earth; every 16 s a glint runs along them one by one. */
  pokemonbadges(x) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, felt = 0x2a3a78;
    fill(x - 12, 7, 25, 18, wood); fill(x - 12, 7, 25, 1, woodLight); fill(x - 12, 7, 1, 18, woodLight); fill(x + 12, 7, 1, 18, woodDark); fill(x - 12, 24, 25, 1, woodDark);
    fill(x - 10, 9, 21, 14, felt); fill(x - 10, 9, 21, 1, 0x1e2a5a); fill(x - 10, 9, 1, 14, 0x1e2a5a);
    // The badges, each in its own shape and colour
    const badges = [
      (bx, by) => { fill(bx, by, 3, 3, 0x9ca0aa); dot(bx, by, 0xc8ccd4); dot(bx + 2, by + 2, 0x6a6e78); },
      (bx, by) => { dot(bx + 1, by, 0x5ab0f0); fill(bx, by + 1, 3, 2, 0x5ab0f0); dot(bx, by + 1, 0x9ad8ff); },
      (bx, by) => { fill(bx, by + 1, 3, 1, 0xf09a30); fill(bx + 1, by, 1, 3, 0xf09a30); dot(bx + 1, by + 1, 0xf8d860); },
      (bx, by) => { dot(bx + 1, by, 0xe84a4a); dot(bx, by + 1, 0x4ab85a); dot(bx + 2, by + 1, 0x4a8ae8); dot(bx + 1, by + 2, 0xf080c0); dot(bx + 1, by + 1, 0xf8d860); },
      (bx, by) => { dot(bx, by, 0xf080b0); dot(bx + 2, by, 0xf080b0); fill(bx, by + 1, 3, 1, 0xf080b0); dot(bx + 1, by + 2, 0xd85a90); },
      (bx, by) => { fill(bx, by, 3, 3, 0xe8c040); dot(bx + 1, by + 1, 0xb08820); dot(bx, by, 0xf8e080); },
      (bx, by) => { dot(bx + 1, by, 0xf8a030); fill(bx, by + 1, 3, 2, 0xe84a2a); dot(bx + 1, by + 2, 0xf8a030); },
      (bx, by) => { dot(bx + 2, by, 0x4ab060); fill(bx, by + 1, 2, 2, 0x4ab060); dot(bx + 2, by + 1, 0x3a8a4a); dot(bx, by + 1, 0x7ad88a); },
    ];
    const g = (now + x * 0.21) % 16, shine = g < 2 ? Math.floor(g * 4) : -1;
    badges.forEach((draw, i) => {
      const bx = x - 8 + (i % 4) * 5, by = 11 + Math.floor(i / 4) * 6;
      draw(bx, by);
      if (i === shine) dot(bx, by, WHITE, 0.9);
    });
    fill(x - 12, 25, 25, 1, SHADOW);
  },
  /** A Kanto town map: green land on a blue sea in a dark frame, tan routes joining the towns, Cinnabar out at sea; a red cursor blinks on one town and hops along the route to the next every 4 s, from Pallet up to the Plateau. */
  pokemonmap(x) {
    const frame = 0x4a3424, frameLight = 0x6a4c34, sea = 0x5a9ad8, land = 0x6ab858, landDark = 0x4a9a44, road = 0xe8d8a0;
    fill(x - 15, 4, 31, 24, frame); fill(x - 15, 4, 31, 1, frameLight); fill(x - 15, 4, 1, 24, frameLight);
    fill(x - 14, 5, 29, 22, sea);
    // The land, with the mountains to the north-west
    fill(x - 13, 6, 27, 14, land); fill(x - 12, 20, 21, 3, land); fill(x - 6, 23, 14, 2, land); fill(x + 10, 20, 3, 2, land);
    for (const [mx, my] of [[-12, 7], [-10, 8], [-7, 7], [-12, 11], [2, 7]]) { dot(x + mx, my, 0x8a7a5a); dot(x + mx + 1, my, landDark); }
    dot(x - 10, 25, land); fill(x - 11, 25, 3, 1, land); // Cinnabar
    // The towns and the routes between them, in the order the cursor visits them
    const towns = [[-10, 21], [-10, 15], [-10, 8], [4, 8], [4, 13], [-3, 13], [11, 13], [4, 19], [-1, 23], [-10, 25], [-13, 9]];
    for (let i = 1; i < towns.length; i++) line(x + towns[i - 1][0], towns[i - 1][1], x + towns[i][0], towns[i][1], road, 0.85);
    for (const [tx, ty] of towns) { fill(x + tx - 1, ty - 1, 2, 2, 0xf4f0e0); dot(x + tx, ty - 1, 0xd8402e); }
    const at = towns[Math.floor((now + x * 0.13) / 4) % towns.length];
    if (Math.floor(now * 3) % 2) { ring(x + at[0], at[1], 2.5, 0xe8202a); }
    fill(x - 15, 27, 31, 1, 0x2e2018); fill(x - 15, 28, 31, 1, SHADOW);
  },
  /** A window onto Route 1: by day blue sky, a tree line, a dirt path through tall grass, and a Pidgey that flaps across every 20 s; by night a moon and stars, and now and then a Gastly drifts past, grinning, glowing faintly purple. */
  pokemonwindow(x) {
    const frame = 0xf2f0ea, frameDark = 0xc8c4bc, sill = 0xd8d2c4;
    fill(x - 14, 3, 29, 26, frame); fill(x + 14, 3, 1, 26, frameDark);
    const L = x - 13, W = 27;
    fill(L, 4, W, 24, isNight ? 0x101a3a : 0x9ad4f8);
    if (isNight) {
      disc(x + 8, 9, 2.6, 0xf4f0d0); dot(x + 7, 8, WHITE);
      for (const [sx, sy] of [[-11, 6], [-5, 9], [2, 5], [11, 13], [-8, 13]]) dot(x + sx, sy, WHITE, 0.6 + 0.4 * (Math.floor(now + sx) % 2));
    } else {
      fill(x - 9, 7, 8, 2, WHITE, 0.9); fill(x - 7, 6, 4, 1, WHITE, 0.9); fill(x + 4, 11, 6, 1, WHITE, 0.8);
    }
    // The tree line, the grass and the path
    for (let k = 0; k < W; k += 4) disc(L + 2 + k, 18, 3, isNight ? 0x1a3a24 : 0x2e7a34);
    fill(L, 18, W, 3, isNight ? 0x1a3a24 : 0x2e7a34);
    fill(L, 21, W, 7, isNight ? 0x24482a : 0x5cb848);
    for (let k = 0; k < 7; k++) fill(x - 3 + Math.floor(k / 2), 21 + k, 5, 1, isNight ? 0x4a4434 : 0xd8c088);
    for (const gx of [-11, -8, 7, 10]) { dot(x + gx, 23, isNight ? 0x1a3a24 : 0x2e7426); dot(x + gx + 2, 23, isNight ? 0x1a3a24 : 0x2e7426); fill(x + gx, 24, 3, 2, isNight ? 0x1e4226 : 0x3a8a30); }
    const t = (now + x * 0.37) % (isNight ? 30 : 20);
    if (!isNight && t < 4) {
      // A Pidgey: brown with a cream face, its wings beating
      const px = L - 3 + Math.floor(t * 8), up = Math.floor(now * 6) % 2;
      if (px > L - 1 && px < L + W - 4) {
        fill(px, 10, 4, 2, 0x9a6a3a); dot(px + 3, 10, 0xf0e0c0); dot(px + 4, 11, 0xe8a030); dot(px + 3, 10, PALETTE.eye);
        fill(px + 1, up ? 8 : 12, 2, 2, 0x7a4a24); dot(px - 1, 11, 0x7a4a24);
      }
    }
    if (isNight && t < 6) {
      const gx = L + W - Math.floor(t * 5), gy = 12 + Math.round(Math.sin(now * 2) * 1.5);
      if (gx > L + 2 && gx < L + W - 3) glowLate(() => {
        glowAt(gx, gy, 5, 5, 0xb070e0, 0.25);
        disc(gx, gy, 2.6, 0x2a1838); dot(gx - 1, gy - 1, WHITE); dot(gx + 1, gy - 1, WHITE); fill(gx - 1, gy + 1, 3, 1, 0xf0e0f0);
      });
    }
    // The cross bars and the sill
    fill(x, 4, 1, 24, frame); fill(L, 15, W, 1, frame);
    fill(x - 15, 28, 31, 2, sill); fill(x - 15, 28, 31, 1, WHITE); fill(x - 15, 30, 31, 1, SHADOW);
    if (isNight) glowLate(() => glowAt(x + 8, 9, 5, 4, 0xfff4c0, 0.12));
  },
  /** A Pikachu poster pinned up: Pikachu's face filling a sky-blue sheet scattered with lightning bolts, black-tipped ears, red cheeks; every 12 s it winks, and now and then its cheeks crackle. */
  pokemonposter(x) {
    const Y = PKM_YELLOW, tip = 0x2a2420, cheek = 0xe8402a;
    fill(x - 10, 4, 21, 24, 0xf6f2e6); fill(x - 9, 5, 19, 22, 0x7ac0f0); fill(x + 10, 4, 1, 24, 0xd8d2c4);
    for (const [bx, by] of [[-8, 7], [6, 6], [-7, 23], [7, 22]]) { dot(x + bx, by, Y); dot(x + bx + 1, by + 1, Y); dot(x + bx, by + 2, Y); dot(x + bx + 1, by + 3, Y); }
    // The ears, leaning out, black at the tips
    for (let k = 0; k < 6; k++) {
      const c = k >= 4 ? tip : Y;
      fill(x - 5 - Math.floor(k / 2), 12 - k, 2, 1, c); fill(x + 4 + Math.floor(k / 2), 12 - k, 2, 1, c);
    }
    // The face
    fill(x - 5, 12, 11, 1, Y); fill(x - 6, 13, 13, 7, Y); fill(x - 5, 20, 11, 1, Y); fill(x - 6, 13, 13, 1, PKM_YELLOW_LIGHT); fill(x - 5, 20, 11, 1, PKM_YELLOW_DARK);
    const wink = (now + x * 0.29) % 12 < 0.6;
    if (wink) fill(x - 4, 16, 2, 1, PALETTE.eye); else { fill(x - 4, 15, 2, 2, PALETTE.eye); dot(x - 4, 15, WHITE); }
    fill(x + 3, 15, 2, 2, PALETTE.eye); dot(x + 3, 15, WHITE);
    fill(x - 6, 17, 2, 2, cheek); fill(x + 5, 17, 2, 2, cheek);
    dot(x, 16, PALETTE.eye); dot(x - 1, 18, 0x8a3a20); dot(x + 1, 18, 0x8a3a20);
    if ((now + x * 0.17) % 9 < 0.8) for (const [sx, sy] of [[-8, 16], [-7, 19], [8, 16], [7, 19]]) if (Math.floor(now * 10 + sx) % 2) dot(x + sx, sy, 0xfff8a0);
    // The tacks
    dot(x - 9, 5, 0xd8402e); dot(x + 9, 5, 0xd8402e);
    fill(x - 10, 28, 21, 1, SHADOW);
  },
  /** A shelf of Poké Balls: a Poké Ball, a Great Ball, an Ultra Ball and a Master Ball on a wooden shelf; every 15 s one of them wobbles three times, then clicks still and a few stars pop over it. */
  pokemonballshelf(x) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25;
    fill(x - 14, 22, 29, 2, wood); fill(x - 14, 22, 29, 1, woodLight); fill(x - 14, 24, 29, 1, woodDark);
    for (const bx of [-11, 10]) { fill(x + bx, 25, 2, 3, woodDark); dot(x + bx, 25, wood); }
    fill(x - 14, 28, 29, 1, SHADOW);
    const t = (now + x * 0.23) % 15, which = Math.floor((now + x * 0.23) / 15) % 4;
    const balls = [PKM_RED, 0x3a6ad8, 0x2a2a32, 0x7a3aa8];
    balls.forEach((top, i) => {
      const wob = i === which && t < 1.8 ? [0, -1, 0, 1, 0, -1, 0, 1, 0][Math.floor(t * 5)] || 0 : 0;
      const bx = x - 14 + i * 7 + wob;
      pokeBall(bx, 15, 7, top);
      // Each ball's marks: the Great Ball's red flashes, the Ultra Ball's yellow H, the Master Ball's pink spots and M
      if (i === 1) { dot(bx + 1, 17, PKM_RED); dot(bx + 5, 17, PKM_RED); }
      if (i === 2) { fill(bx + 1, 16, 1, 2, 0xf8d030); fill(bx + 5, 16, 1, 2, 0xf8d030); fill(bx + 2, 17, 3, 1, 0xf8d030); }
      if (i === 3) { dot(bx + 1, 16, 0xf080c0); dot(bx + 5, 16, 0xf080c0); dot(bx + 3, 16, WHITE); }
      if (i === which && t >= 1.8 && t < 2.8) for (const [sx, sy] of [[0, 12], [3, 11], [6, 12]]) if (Math.floor(now * 6 + sx) % 2) dot(bx + sx, sy, 0xfff4a0);
    });
  },
  /** A Pokémon Center sign: a lightbox in red with a big Poké Ball on it and a white band below; it glows at night, and every 25 s it gives the two quick flashes of the healing jingle. */
  pokemonsign(x) {
    fill(x - 4, 3, 1, 3, 0x6e727c); fill(x + 4, 3, 1, 3, 0x6e727c);
    fill(x - 11, 5, 23, 19, PKM_RED); fill(x - 11, 5, 23, 1, PKM_RED_LIGHT); fill(x - 11, 5, 1, 19, PKM_RED_LIGHT); fill(x + 11, 6, 1, 18, PKM_RED_DARK);
    fill(x - 10, 19, 21, 4, PKM_WHITE); fill(x - 10, 22, 21, 1, PKM_GREY);
    for (let k = 0; k < 6; k++) fill(x - 8 + k * 3, 20, 2, 2, PKM_RED, 0.75);
    disc(x, 12, 6.6, PKM_RED_DARK); pokeBall(x - 5, 7, 11);
    fill(x - 11, 24, 23, 1, SHADOW);
    const f = (now + x * 0.19) % 25, flash = f < 1 && Math.floor(f * 6) % 3 !== 2;
    if (flash) glowAt(x, 12, 12, 10, 0xffe0d8, 0.25);
    if (isNight) glowLate(() => { glowAt(x, 13, 14, 12, 0xff6040, flash ? 0.3 : 0.16); pokeBall(x - 5, 7, 11, PKM_RED, 0.55); });
  },
  /** A Summoner's Rift minimap in a gold frame: three tan lanes, the blue river across, the blue base bottom-left and the red base top-right; champion dots skirmish in mid and every 10 s a ping flashes. */
  lolminimap(x) {
    hexPanel(x - 13, 3, 27, 26);
    const L = x - 11, T = 5, W = 23, road = 0xd8c890;
    fill(L, T, W, 22, 0x2e5a2e);
    for (let k = 0; k < 9; k++) dot(L + (hash(k, x) % W), T + (hash(x, k) % 22), 0x3a6e36);
    line(L, T, L + W - 1, T + 21, 0x3a7ab8, 0.9); line(L + 1, T, L + W - 1, T + 20, 0x3a7ab8, 0.5);
    fill(L + 1, T + 1, 1, 20, road); fill(L + 1, T + 1, W - 2, 1, road);
    fill(L + 1, T + 20, W - 2, 1, road); fill(L + W - 2, T + 1, 1, 20, road);
    line(L + 2, T + 19, L + W - 3, T + 2, road);
    fill(L, T + 18, 4, 4, 0x3a7ad8); dot(L + 1, T + 19, 0x9ad0ff);
    fill(L + W - 4, T, 4, 4, 0xc83a3a); dot(L + W - 2, T + 2, 0xffa0a0);
    // The champions: two blue and two red, meeting in mid
    const m = Math.sin(now * 0.4 + x) * 3;
    for (const [k, c] of [[-2, 0x6ab8ff], [-4, 0x6ab8ff], [2, 0xff6a6a], [4, 0xff6a6a]]) {
      const d = 11 + k + Math.round(m);
      dot(L + d, T + 21 - d, c); dot(L + d + 1, T + 21 - d, c, 0.6);
    }
    const g = (now + x * 0.3) % 10;
    if (g < 1) ring(L + 6 + (Math.floor(now / 10) % 3) * 5, T + 5, 1.5 + g * 3, 0xf2c43a, 1 - g);
  },
  /** A team banner on a gold rod: deep blue cloth with a gold border, a hextech crystal emblem, and a swallowtail hem that stirs now and then. */
  lolbanner(x) {
    const cloth = 0x1a3a7a, clothLight = 0x2a5aa8, clothDark = 0x0e2450;
    fill(x - 8, 4, 17, 1, LOL_GOLD); dot(x - 8, 4, LOL_GOLD_LIGHT); dot(x + 8, 4, LOL_GOLD_DARK); disc(x - 8, 4, 1, LOL_GOLD); disc(x + 8, 4, 1, LOL_GOLD);
    const stir = (now + x * 0.37) % 12 < 2 ? Math.floor(now * 4) % 2 : 0;
    fill(x - 6, 5, 13, 18, cloth); fill(x - 6, 5, 1, 18, clothLight); fill(x + 6, 5, 1, 18, clothDark);
    for (let k = 0; k < 4; k++) { fill(x - 6 + stir, 23 + k, 6 - k * 2 + (k ? 0 : 1), 1, cloth); fill(x + 1 + k * 2 + stir, 23 + k, 6 - k * 2, 1, cloth); }
    fill(x - 5, 6, 11, 1, LOL_GOLD); fill(x - 5, 21, 11, 1, LOL_GOLD); fill(x - 5, 6, 1, 16, LOL_GOLD_DARK); fill(x + 5, 6, 1, 16, LOL_GOLD_DARK);
    line(x, 9, x + 3, 13, LOL_GOLD); line(x + 3, 13, x, 18, LOL_GOLD); line(x, 18, x - 3, 13, LOL_GOLD); line(x - 3, 13, x, 9, LOL_GOLD);
    hexCrystal(x, 13, 7, (Math.sin(now * 1.1 + x) + 1) / 2);
  },
  /** A ranked crest: a Challenger emblem with spread gold wings, a navy shield set with a blue gem and a little crown on top; a gleam sweeps it every 15 s and the gem glows at night. */
  lolcrest(x) {
    // The wings, feathered in steps
    for (let k = 0; k < 5; k++) {
      const top = 12 - k * 2, h = 8 - k;
      fill(x - 7 - k * 2, top, 3, h, LOL_GOLD); fill(x + 5 + k * 2, top, 3, h, LOL_GOLD);
      dot(x - 7 - k * 2, top, LOL_GOLD_LIGHT); dot(x + 7 + k * 2, top, LOL_GOLD_LIGHT);
      fill(x - 7 - k * 2, top + h - 1, 3, 1, LOL_GOLD_DARK); fill(x + 5 + k * 2, top + h - 1, 3, 1, LOL_GOLD_DARK);
    }
    // The shield
    fill(x - 5, 8, 11, 12, LOL_GOLD); fill(x - 4, 9, 9, 10, LOL_NAVY);
    for (let k = 0; k < 4; k++) fill(x - 4 + k, 20 + k, 9 - k * 2, 1, k ? LOL_GOLD : LOL_NAVY);
    fill(x - 3, 20, 7, 1, LOL_NAVY); fill(x - 2, 21, 5, 1, LOL_NAVY); dot(x, 22, LOL_NAVY);
    hexCrystal(x, 15, 9, 0.6);
    // The crown
    fill(x - 3, 5, 7, 3, LOL_GOLD); dot(x - 3, 4, LOL_GOLD); dot(x, 3, LOL_GOLD_LIGHT); dot(x + 3, 4, LOL_GOLD); fill(x - 3, 7, 7, 1, LOL_GOLD_DARK); dot(x, 6, 0x6ab8ff);
    const g = (now + x * 0.23) % 15;
    if (g < 1) { const gx = x - 14 + Math.floor(g * 28); fill(gx, 8, 1, 12, WHITE, 0.45); }
    fill(x - 6, 25, 13, 1, SHADOW);
    if (isNight) glowLate(() => glowAt(x, 15, 6, 6, LOL_HEX, 0.14));
  },
  /** A window onto Piltover in a brass frame: by day brass towers under a warm golden sky, a hextech airship drifting across; by night the towers' windows lit, a blue hex-lamp on the tallest, Zaun's green haze rising below. */
  lolwindow(x) {
    const brass = 0xc8a050, brassDark = 0x8a6a2a;
    fill(x - 14, 3, 29, 26, brass); fill(x - 14, 3, 29, 1, 0xe8c878); fill(x + 14, 3, 1, 26, brassDark);
    const L = x - 13, W = 27;
    fill(L, 4, W, 24, isNight ? 0x101a38 : 0xf0c890); if (!isNight) fill(L, 4, W, 6, 0xe8a87a);
    // The towers
    const towers = [[-12, 14, 5], [-6, 9, 4], [-1, 6, 4], [4, 12, 5], [10, 10, 3]];
    for (const [dx, top, w] of towers) {
      fill(x + dx, top, w, 28 - top, isNight ? 0x2a2a3a : 0xb08a5a); fill(x + dx, top, 1, 28 - top, isNight ? 0x3a3a4e : 0xd0aa78);
      fill(x + dx, top - 1, w, 1, isNight ? 0x4a4a5a : LOL_GOLD);
      for (let wy = top + 2; wy < 25; wy += 3) for (let wx = 1; wx < w - 1; wx += 2) dot(x + dx + wx, wy, isNight ? (hash(wx + dx, wy) % 3 ? 0xffd890 : 0x3a3a4e) : 0x8a6a4a);
    }
    dot(x + 1, 5, isNight ? LOL_HEX_LIGHT : LOL_HEX); dot(x + 1, 4, LOL_GOLD);
    if (isNight) { fill(L, 24, W, 4, 0x3ad87a, 0.18); glowLate(() => glowAt(x + 1, 5, 4, 3, LOL_HEX, 0.3)); }
    else {
      // The airship, drifting across every 24 s
      const t = (now + x * 0.29) % 24, ax = L - 6 + Math.floor(t * 2);
      if (ax > L - 1 && ax < L + W - 6) { fill(ax, 8, 6, 2, 0x8a5a3a); fill(ax + 1, 7, 4, 1, 0xa87a4a); dot(ax + 2, 10, LOL_HEX); fill(ax + 1, 10, 1, 1, brassDark); }
    }
    fill(x, 4, 1, 24, brass); fill(L, 16, W, 1, brass);
    fill(x - 15, 28, 31, 2, brassDark); fill(x - 15, 28, 31, 1, brass); fill(x - 15, 30, 31, 1, SHADOW);
  },
  /** An ability bar: four gold-framed ability icons (a blue bolt, an orange shield, a green dash and a purple ultimate) over health and mana bars; each goes grey on cooldown and fills back up from the bottom, flashing when it's ready. */
  lolabilities(x) {
    hexPanel(x - 15, 9, 31, 17);
    const icons = [
      [0x2a5ab8, (ix) => { line(ix + 3, 12, ix + 1, 15, 0xbfe8ff); line(ix + 1, 15, ix + 3, 15, 0xbfe8ff); line(ix + 3, 15, ix + 1, 17, 0xbfe8ff); }],
      [0xa85a1a, (ix) => { fill(ix + 1, 12, 3, 3, 0xf8b050); dot(ix + 2, 15, 0xf8b050); }],
      [0x2a7a3a, (ix) => { fill(ix + 1, 14, 3, 1, 0xa8f0a0); dot(ix + 3, 13, 0xa8f0a0); dot(ix + 3, 15, 0xa8f0a0); }],
      [0x5a2a8a, (ix) => { disc(ix + 2, 14, 1.6, 0xe0a0ff); dot(ix + 2, 14, WHITE); }],
    ];
    icons.forEach(([bg, mark], i) => {
      const ix = x - 13 + i * 7, cd = [6, 9, 7, 30][i], t = (now + x * 0.17 + i * 2.3) % cd, cool = t < cd * 0.45;
      fill(ix - 1, 10, 7, 7, LOL_GOLD_DARK); fill(ix - 1, 10, 7, 1, LOL_GOLD);
      fill(ix, 11, 5, 5, bg); mark(ix);
      if (cool) { const left = Math.ceil((1 - t / (cd * 0.45)) * 5); fill(ix, 11, 5, left, 0x1a1a24, 0.7); }
      else if (t < cd * 0.45 + 0.3) fill(ix, 11, 5, 5, WHITE, 0.4);
    });
    // The health and mana bars
    const hp = 14 + Math.round(Math.sin(now * 0.3 + x) * 5);
    fill(x - 13, 19, 27, 2, 0x1a1a24); fill(x - 13, 19, hp, 2, 0x3ab84a); fill(x - 13, 19, hp, 1, 0x6ad86a);
    fill(x - 13, 22, 27, 2, 0x1a1a24); fill(x - 13, 22, 20, 2, 0x2a6ad8); fill(x - 13, 22, 20, 1, 0x5a9af0);
    fill(x - 15, 26, 31, 1, SHADOW);
  },
  /** A Pentakill frame: five champion portraits in a gold-framed plaque; every 20 s they light up one after another, and with the fifth the gold bar beneath blazes for a moment. */
  lolpenta(x) {
    hexPanel(x - 15, 7, 31, 18);
    const faces = [[0xd8a070, 0x3a8ae8], [0xf0c8a0, 0x2a2a30], [0x8a5a3a, 0xc83a3a], [0xe8d0b0, 0xf2d27a], [0x7a9a5a, 0x5a3a2a]];
    const t = (now + x * 0.21) % 20, lit = t < 4 ? Math.floor(t * 1.4) + 1 : 0;
    faces.forEach(([skin, hair], i) => {
      const px = x - 13 + i * 6;
      fill(px - 1, 9, 6, 7, i < lit ? LOL_GOLD_LIGHT : LOL_GOLD_DARK);
      fill(px, 10, 4, 5, 0x1e2d50); fill(px, 11, 4, 4, skin); fill(px, 10, 4, 2, hair); dot(px + 1, 13, PALETTE.eye); dot(px + 3, 13, PALETTE.eye);
      if (i >= lit && lit) fill(px, 10, 4, 5, 0x0a1428, 0.4);
    });
    const blaze = lit >= 5;
    fill(x - 13, 19, 27, 3, blaze ? LOL_GOLD_LIGHT : LOL_GOLD_DARK); fill(x - 13, 19, 27, 1, LOL_GOLD);
    for (let k = 0; k < 5; k++) dot(x - 10 + k * 5, 20, blaze ? 0xc83a3a : LOL_NAVY);
    if (blaze) glowAt(x, 20, 16, 5, 0xffd890, 0.3);
    fill(x - 15, 25, 31, 1, SHADOW);
  },
  /** An Alliance banner: royal blue cloth with a gold border and a gold lion's head, a swallowtail hem that stirs now and then, on an iron rod. */
  wowlion(x) {
    const cloth = WOW_BLUE, light = 0x2e62c0, dark = 0x12306a;
    fill(x - 8, 4, 17, 1, WOW_IRON); disc(x - 8, 4, 1, WOW_GOLD); disc(x + 8, 4, 1, WOW_GOLD);
    const stir = (now + x * 0.37) % 13 < 2 ? Math.floor(now * 4) % 2 : 0;
    fill(x - 6, 5, 13, 18, cloth); fill(x - 6, 5, 1, 18, light); fill(x + 6, 5, 1, 18, dark);
    for (let k = 0; k < 4; k++) { fill(x - 6 + stir, 23 + k, 6 - k * 2 + (k ? 0 : 1), 1, cloth); fill(x + 1 + k * 2 + stir, 23 + k, 6 - k * 2, 1, cloth); }
    fill(x - 5, 6, 11, 1, WOW_GOLD); fill(x - 5, 6, 1, 16, WOW_GOLD); fill(x + 5, 6, 1, 16, WOW_GOLD_DARK); fill(x - 5, 21, 11, 1, WOW_GOLD_DARK);
    // The lion's head: a mane round a face, ears up, a jaw
    disc(x, 13, 3.6, WOW_GOLD); fill(x - 2, 11, 5, 5, 0xf2cc6a); dot(x - 3, 9, WOW_GOLD); dot(x + 3, 9, WOW_GOLD);
    dot(x - 1, 12, WOW_BLUE); dot(x + 1, 12, WOW_BLUE); fill(x - 1, 15, 3, 1, WOW_GOLD_DARK); dot(x, 14, WOW_GOLD_DARK);
  },
  /** A Horde banner: blood-red cloth with the black Horde emblem, a jagged bar with its spikes, on a spiked iron rod, the frayed hem stirring now and then. */
  wowhordebanner(x) {
    const cloth = WOW_RED, light = 0xb83a2a, dark = 0x6a0e0e, black = 0x1a1414;
    fill(x - 8, 4, 17, 1, WOW_IRON); dot(x - 9, 3, 0x9a9aa0); dot(x + 9, 3, 0x9a9aa0); dot(x - 8, 3, WOW_IRON); dot(x + 8, 3, WOW_IRON);
    const stir = (now + x * 0.41) % 11 < 2 ? Math.floor(now * 4) % 2 : 0;
    fill(x - 6, 5, 13, 19, cloth); fill(x - 6, 5, 1, 19, light); fill(x + 6, 5, 1, 19, dark);
    for (let k = 0; k < 13; k += 2) fill(x - 6 + k + stir, 24, 1, 1 + (hash(k, x) % 3), cloth);
    // The emblem: a central bar, two hooked arms, spikes either side
    fill(x, 8, 1, 12, black); fill(x - 3, 10, 7, 1, black); fill(x - 3, 10, 1, 3, black); fill(x + 3, 10, 1, 3, black);
    fill(x - 4, 16, 9, 1, black); dot(x - 5, 15, black); dot(x + 5, 15, black); dot(x - 2, 19, black); dot(x + 2, 19, black); dot(x, 7, black);
  },
  /** A quest board: a wooden board on posts with parchment notices pinned to it, a wanted poster with a murloc's face, and a gold "!" floating over it; every 15 s a loose notice flutters. */
  wowquestboard(x) {
    fill(x - 13, 8, 2, 20, WOW_WOOD_DARK); fill(x + 12, 8, 2, 20, WOW_WOOD_DARK);
    fill(x - 12, 9, 25, 16, WOW_WOOD); fill(x - 12, 9, 25, 1, WOW_WOOD_LIGHT); fill(x - 12, 24, 25, 1, WOW_WOOD_DARK);
    for (let k = 0; k < 3; k++) fill(x - 12, 13 + k * 4, 25, 1, WOW_WOOD_DARK, 0.5);
    fill(x - 12, 7, 25, 2, WOW_WOOD_DARK); fill(x - 13, 6, 27, 1, WOW_WOOD_LIGHT);
    const paper = 0xeadcb4, ink = 0x5a4a3a, flap = (now + x * 0.29) % 15 < 1.5 ? Math.floor(now * 6) % 2 : 0;
    fill(x - 10, 11, 7, 9, paper); for (let k = 0; k < 3; k++) fill(x - 9, 14 + k * 2, 5, 1, ink, 0.6); dot(x - 7, 11, 0xc83a2a);
    fill(x - 1, 10, 7, 11, paper); fill(x, 11, 5, 1, ink); disc(x + 2, 15, 2, 0x4a9a8a); dot(x + 1, 14, 0xf2e05a); fill(x + 1, 16, 3, 1, 0x2a2a2a); fill(x, 18, 5, 1, ink, 0.6); dot(x + 2, 10, 0xc83a2a);
    fill(x + 7, 12 + flap, 5, 7 - flap, paper); fill(x + 8, 14, 3, 1, ink, 0.6); fill(x + 8, 16, 3, 1, ink, 0.6); dot(x + 9, 12, 0xc83a2a);
    questMark(x - 1, 0 + Math.round(Math.sin(now * 2 + x)), false);
    fill(x - 13, 28, 27, 1, SHADOW);
  },
  /** A window onto Stormwind: white towers with blue roofs against the sky, a gryphon flying past every 22 s; at night the moon, stars and the towers' lit windows. */
  wowwindow(x) {
    const frame = WOW_WOOD, frameDark = WOW_WOOD_DARK;
    fill(x - 14, 3, 29, 26, frame); fill(x - 14, 3, 29, 1, WOW_WOOD_LIGHT); fill(x + 14, 3, 1, 26, frameDark);
    const L = x - 13, W = 27;
    fill(L, 4, W, 24, isNight ? 0x0e1838 : 0x8ac4ec);
    if (isNight) { disc(x - 8, 8, 2.4, 0xeef0f4); for (const [sx, sy] of [[3, 6], [9, 9], [-3, 11], [11, 5]]) dot(x + sx, sy, WHITE, 0.7); }
    // The towers, their blue roofs and windows
    for (const [dx, top, w] of [[-11, 15, 5], [-5, 10, 5], [1, 13, 4], [6, 8, 5], [11, 16, 2]]) {
      fill(x + dx, top, w, 28 - top, isNight ? 0x4a4e60 : 0xe8e4dc); fill(x + dx, top, 1, 28 - top, isNight ? 0x5a5e70 : WHITE);
      for (let k = 0; k <= Math.floor(w / 2); k++) fill(x + dx + k, top - 3 + k, w - k * 2, 1, isNight ? 0x1e2e5a : 0x2a4a9a);
      dot(x + dx + Math.floor(w / 2), top + 3, isNight ? 0xffd080 : 0x3a3a4a); if (w > 3) dot(x + dx + 1, top + 7, isNight ? 0xffd080 : 0x3a3a4a);
    }
    fill(L, 24, W, 4, isNight ? 0x3a3e4e : 0xc8c0b0);
    if (!isNight) {
      const t = (now + x * 0.31) % 22, gx = L - 6 + Math.floor(t * 4), up = Math.floor(now * 4) % 2;
      if (t < 9 && gx > L - 1 && gx < L + W - 5) { fill(gx, 9, 4, 2, 0xb08a4a); fill(gx + 4, 8, 2, 2, WHITE); dot(gx + 6, 9, WOW_GOLD); fill(gx + 1, up ? 7 : 11, 3, 2, 0x7a5a3a); }
    }
    fill(x, 4, 1, 24, frame); fill(L, 15, W, 1, frame);
    fill(x - 15, 28, 31, 2, frameDark); fill(x - 15, 28, 31, 1, frame); fill(x - 15, 30, 31, 1, SHADOW);
  },
  /** A shield and crossed swords: a round oak shield rimmed in iron with an iron boss, painted blue with a gold chevron, over two crossed swords whose blades catch the light every 14 s. */
  wowshield(x) {
    const steel = 0xb8bcc8, steelDark = 0x6e727c;
    const g = (now + x * 0.27) % 14, glint = g < 1 ? Math.floor(g * 10) : -1;
    for (const d of [-1, 1]) {
      for (let k = 0; k < 18; k++) { const px = x + d * (-9 + k), py = 4 + k; dot(px, py, k === glint || k === glint + 1 ? WHITE : steel); dot(px + d, py, steelDark); }
      fill(x + d * 9 - 1, 21, 3, 1, WOW_GOLD); fill(x + d * 10, 22, 1, 3, WOW_WOOD_DARK); dot(x + d * 11, 25, WOW_GOLD);
    }
    disc(x, 14, 8.6, WOW_IRON); disc(x, 14, 7.6, WOW_BLUE); disc(x - 1, 13, 6, 0x2a5aaa, 0.5);
    line(x - 5, 17, x, 11, WOW_GOLD); line(x, 11, x + 5, 17, WOW_GOLD); line(x - 5, 18, x, 12, WOW_GOLD_DARK); line(x, 12, x + 5, 18, WOW_GOLD_DARK);
    disc(x, 14, 2, 0x8a8a92); dot(x - 1, 13, 0xd8dce4);
    for (let k = 0; k < 8; k++) { const a = k * Math.PI / 4; dot(x + Math.round(Math.cos(a) * 8), 14 + Math.round(Math.sin(a) * 8), 0x9a9aa0); }
    fill(x - 8, 24, 17, 1, SHADOW);
  },
  /** A map of Azeroth: Kalimdor and the Eastern Kingdoms on a blue sea, inked on parchment in a wooden frame; a dotted flight path runs between them and a gryphon speck flies along it. */
  wowmap(x) {
    fill(x - 15, 4, 31, 24, WOW_WOOD_DARK); fill(x - 15, 4, 31, 1, WOW_WOOD);
    fill(x - 14, 5, 29, 22, 0xe8d8b0); fill(x - 13, 6, 27, 20, 0x8ab0c8);
    const land = 0xc8b880, landDark = 0x9a8a58;
    // Kalimdor on the left, the Eastern Kingdoms on the right, Northrend across the top
    fill(x - 11, 9, 7, 14, land); fill(x - 10, 7, 4, 2, land); fill(x - 9, 23, 3, 2, land); fill(x - 12, 13, 1, 5, land); fill(x - 11, 9, 1, 14, landDark);
    fill(x + 4, 10, 6, 13, land); fill(x + 6, 8, 3, 2, land); fill(x + 3, 15, 1, 4, land); fill(x + 5, 23, 3, 2, land); fill(x + 9, 10, 1, 13, landDark);
    fill(x - 4, 6, 9, 2, 0xeef0f0); fill(x - 2, 8, 4, 1, 0xeef0f0);
    dot(x - 8, 12, 0x3a7a3a); dot(x - 7, 18, 0xc87a3a); dot(x + 6, 13, 0x3a7a3a); dot(x + 7, 19, 0x6a6a6a);
    // The flight path, and the gryphon on it
    const path = [[-7, 15], [-3, 14], [0, 14], [3, 15], [6, 16]];
    for (let i = 1; i < path.length; i++) dot(x + path[i][0], path[i][1], 0xc83a2a, 0.8);
    const k = ((now * 0.25 + x * 0.1) % 1) * (path.length - 1), i = Math.floor(k), f = k - i;
    const gx = x + Math.round(path[i][0] + (path[i + 1][0] - path[i][0]) * f), gy = Math.round(path[i][1] + (path[i + 1][1] - path[i][1]) * f);
    dot(gx, gy - 1, 0x5a3a1a); dot(gx - 1, gy - 2, 0x5a3a1a); dot(gx + 1, gy - 2, 0x5a3a1a);
    fill(x - 15, 28, 31, 1, SHADOW);
  },
  /** A window onto a Vice City sunset: bands of orange, pink and violet sky, the sun sinking into the sea, palm silhouettes and a speedboat crossing every 20 s; at night a violet sky over a skyline lit in pink and teal neon. */
  gtasunset(x) {
    fill(x - 14, 3, 29, 26, WHITE); fill(x + 14, 3, 1, 26, 0xd8d0d8);
    const L = x - 13, W = 27;
    if (isNight) {
      fill(L, 4, W, 24, 0x2a1240); fill(L, 4, W, 6, 0x1a0a2e);
      for (const [dx, top, w, c] of [[-12, 13, 5, GTA_PINK], [-6, 9, 4, GTA_TEAL], [-1, 12, 6, GTA_PINK], [6, 8, 4, GTA_TEAL], [11, 14, 2, GTA_PINK]]) {
        fill(x + dx, top, w, 20 - top, 0x1a1028); fill(x + dx, top, w, 1, c);
        for (let wy = top + 2; wy < 19; wy += 2) dot(x + dx + 1, wy, hash(dx, wy) % 2 ? 0xffd890 : 0x2a2038);
      }
      fill(L, 20, W, 8, 0x140a24); for (let k = 0; k < W; k += 3) dot(L + k, 21 + (k % 2), k % 2 ? GTA_PINK : GTA_TEAL, 0.5);
      glowLate(() => { glowAt(x, 13, 14, 6, GTA_PINK, 0.12); });
    } else {
      const bands = [0x6a3a9a, 0xb84a9a, 0xf05a8a, 0xf8884a, 0xf8b04a];
      bands.forEach((c, i) => fill(L, 4 + i * 3, W, 3, c));
      disc(x + 2, 19, 4, 0xfff0a0); fill(L, 19, W, 9, 0x3a6aa8);
      for (let k = 0; k < 4; k++) fill(x - 3 + k * 3, 20 + k * 2, 8 - k * 2, 1, 0xf8c070, 0.7);
      const t = (now + x * 0.31) % 20, bx = L + W - Math.floor(t * 4);
      if (t < 7 && bx > L && bx < L + W - 4) { fill(bx, 23, 5, 1, WHITE); fill(bx + 1, 22, 2, 1, WHITE); fill(bx + 5, 23, 3, 1, 0xd8f0ff, 0.6); }
    }
    // The palms
    for (const [px, h] of [[-10, 14], [10, 12]]) {
      const top = 28 - h, c = isNight ? 0x0a0614 : 0x2a1a24;
      for (let k = 0; k < h; k++) dot(x + px + Math.round(Math.sin(k / 4) * 1), 27 - k, c);
      for (const [dx, dy] of [[-3, 1], [-2, 0], [-1, -1], [1, -1], [2, 0], [3, 1], [-4, 2], [4, 2]]) dot(x + px + dx, top + dy, c);
    }
    fill(x, 4, 1, 24, WHITE); fill(x - 15, 28, 31, 2, 0xe8e0e8); fill(x - 15, 30, 31, 1, SHADOW);
  },
  /** A map of San Andreas: green hills, tan desert and grey city grids on a blue sea, a white arrow for you and a yellow mission blip that blinks; framed in black. */
  gtamap(x) {
    fill(x - 15, 4, 31, 24, 0x1a1a20); fill(x - 15, 4, 31, 1, 0x3a3a44);
    fill(x - 14, 5, 29, 22, 0x4a7ab8);
    fill(x - 12, 6, 25, 19, 0x6a9a4a); fill(x - 12, 6, 10, 9, 0xc8a870); fill(x + 4, 16, 9, 9, 0x8a8e94); fill(x - 12, 17, 8, 8, 0x8a8e94); fill(x + 2, 6, 6, 5, 0x8a8e94);
    for (const [gx, gy, gw, gh] of [[4, 16, 9, 9], [-12, 17, 8, 8], [2, 6, 6, 5]]) { for (let k = 2; k < gw; k += 3) fill(x + gx + k, gy, 1, gh, 0x5a5e66); for (let k = 2; k < gh; k += 3) fill(x + gx, gy + k, gw, 1, 0x5a5e66); }
    line(x - 8, 20, x + 8, 20, 0xe8e0c0); line(x - 2, 8, x - 2, 22, 0xe8e0c0);
    dot(x - 1, 14, WHITE); fill(x - 2, 15, 3, 1, WHITE); dot(x - 1, 13, WHITE, 0.6);
    if (Math.floor(now * 2 + x) % 2) { disc(x + 7, 9, 1.5, 0xf2c43a); }
    fill(x - 15, 28, 31, 1, SHADOW);
  },
  /** A neon palm sign: a palm tree drawn in pink and teal neon tubes over the word VICE, on a dark backboard; it glows, buzzes off for a flicker now and then, and blazes at night. */
  gtaneon(x) {
    fill(x - 13, 5, 27, 22, 0x1a1428); fill(x - 13, 5, 27, 1, 0x2e2440); fill(x + 13, 6, 1, 21, 0x0e0a18);
    const f = (now + x * 0.29) % 11, off = f < 0.5 && Math.floor(f * 12) % 2 === 0;
    const tubes = () => {
      const a = off ? 0.3 : 1;
      for (let k = 0; k < 8; k++) dot(x - 9 + Math.round(Math.sin(k / 3)), 18 - k, GTA_PINK, a);
      for (const [dx, dy] of [[-12, 11], [-11, 10], [-10, 10], [-8, 10], [-7, 10], [-6, 11], [-11, 12], [-7, 12], [-9, 9]]) dot(x + dx, dy, GTA_TEAL, a);
      gtaText(x - 4, 10, 'VICE', GTA_PINK, a);
      fill(x - 10, 19, 21, 1, GTA_TEAL, a); fill(x - 10, 21, 21, 1, GTA_PINK, 0.6 * a);
      glowAt(x, 15, 13, 9, GTA_PINK, (isNight ? 0.16 : 0.07) * a);
    };
    if (isNight) glowLate(tubes); else tubes();
    fill(x - 13, 27, 27, 1, SHADOW);
  },
  /** A loading-screen poster: a duotone figure in orange and teal, sunglasses on, a fistful of cash, against a big setting sun, in a white frame. */
  gtaposter(x) {
    fill(x - 10, 4, 21, 25, WHITE); fill(x + 10, 4, 1, 25, 0xd0d0d8);
    fill(x - 9, 5, 19, 23, 0x2a8a9a); fill(x - 9, 5, 19, 9, 0x3aa8b0);
    disc(x + 2, 13, 6, GTA_SUNSET); fill(x - 9, 19, 19, 9, 0x1e6a7a);
    // The figure, in silhouette with an orange rim of light
    const body = 0x14303a;
    fill(x - 4, 9, 5, 5, body); fill(x - 5, 14, 7, 9, body); fill(x - 6, 15, 2, 7, body); fill(x + 2, 13, 3, 2, body); fill(x - 4, 23, 2, 5, body); fill(x - 1, 23, 2, 5, body);
    fill(x - 4, 11, 5, 1, 0x0a1418); dot(x - 3, 11, GTA_SUNSET);
    fill(x, 9, 1, 5, GTA_SUNSET); fill(x + 1, 14, 1, 8, GTA_SUNSET, 0.7);
    fill(x + 4, 11, 3, 2, 0x6ac85a); dot(x + 5, 11, 0xa8f090);
    fill(x - 10, 29, 21, 1, SHADOW);
  },
  /** A Grove Street tag: a patch of brick sprayed with a big green GSF tag, drips running from it, and a green bandana pinned beside it; the paint catches the light now and then. */
  gtagrove(x) {
    for (let y = 6, r = 0; y < 26; y += 4, r++) for (let k = (r % 2) * 4 - 12; k < 12; k += 8) fill(x + k, y, Math.min(7, 12 - k), 3, (r + k) % 3 ? 0x8a3a2a : 0x7a3424);
    fill(x - 12, 26, 25, 1, SHADOW);
    const green = 0x2ab84a;
    gtaText(x - 10, 10, 'GSF', green); gtaText(x - 9, 10, 'GSF', green, 0.6);
    for (const dx of [-9, -4, 1]) { const d = 2 + hash(dx, x) % 3; fill(x + dx, 15, 1, d, green, 0.8); }
    line(x - 11, 18, x + 1, 17, green, 0.7);
    fill(x + 4, 8, 7, 4, green); fill(x + 5, 12, 5, 2, green); fill(x + 6, 14, 3, 2, green); dot(x + 7, 16, green);
    dot(x + 5, 9, WHITE, 0.7); dot(x + 8, 10, WHITE, 0.7); dot(x + 7, 13, WHITE, 0.7); dot(x + 7, 8, 0x8a8e94);
    if ((now + x * 0.23) % 12 < 0.4) dot(x - 6, 10, WHITE);
  },
  /** A row of licence plates nailed to the wall: San Andreas white with red letters, Vice City pink and teal, Liberty City yellow; a glint runs along them every 15 s. */
  gtaplates(x) {
    const plates = [[0xf4f2ee, 0xc83a3a, 0x3a4a8a], [0xf8d0dc, GTA_TEAL, GTA_PINK], [0xf2c43a, 0x2a2a30, 0x2a2a30]];
    const g = (now + x * 0.21) % 15, glint = g < 1.2 ? Math.floor(g * 25) : -1;
    plates.forEach(([bg, ink, band], i) => {
      const px = x - 14 + i * 10, py = 11 + (i % 2) * 3;
      fill(px, py, 9, 6, bg); fill(px, py, 9, 1, band); fill(px + 8, py + 1, 1, 5, shaded(bg, 0.8)); fill(px, py + 5, 9, 1, shaded(bg, 0.85));
      for (let k = 0; k < 4; k++) fill(px + 1 + k * 2, py + 2, 1, 2, ink);
      dot(px + 1, py + 1, 0x8a8e94); dot(px + 7, py + 1, 0x8a8e94);
      if (glint >= px - x + 14 && glint < px - x + 23) dot(px + glint - (px - x + 14), py + 1, WHITE, 0.9);
      fill(px, py + 6, 9, 1, SHADOW);
    });
  },
  /** A window onto the Last City: towers under a dusky sky, the great white sphere of the Traveler hanging over them, a jumpship crossing every 20 s; at night the Traveler glows softly and the city's windows are lit. */
  destinywindow(x) {
    fill(x - 14, 3, 29, 26, 0x2a3a4a); fill(x - 14, 3, 29, 1, DST_GOLD); fill(x + 14, 3, 1, 26, 0x1a2430);
    const L = x - 13, W = 27;
    fill(L, 4, W, 24, isNight ? 0x101a30 : 0x9ab8d8); if (!isNight) fill(L, 4, W, 8, 0xb8c8e0); else fill(L, 4, W, 6, 0x0a1224);
    traveler(x + 4, 10, 4.6, isNight ? 0.25 : 0.12);
    for (const [dx, top, w] of [[-13, 17, 4], [-9, 13, 3], [-6, 16, 4], [-2, 12, 3], [1, 18, 4], [5, 15, 3], [8, 17, 5]]) {
      fill(x + dx, top, w, 28 - top, isNight ? 0x2a3040 : 0x6a7a8a); fill(x + dx, top, 1, 28 - top, isNight ? 0x3a4050 : 0x8a9aa8);
      for (let wy = top + 2; wy < 27; wy += 3) dot(x + dx + 1, wy, isNight ? (hash(dx, wy) % 3 ? 0xffd890 : 0x2a3040) : 0x4a5a6a);
    }
    if (!isNight) {
      const t = (now + x * 0.29) % 20, sx = L + W - Math.floor(t * 5);
      if (t < 5 && sx > L && sx < L + W - 4) { fill(sx, 8, 4, 1, 0x4a4e58); dot(sx + 1, 7, 0x4a4e58); dot(sx + 4, 8, DST_ARC); }
    }
    fill(x - 15, 28, 31, 2, 0x2a3a4a); fill(x - 15, 28, 31, 1, DST_GOLD); fill(x - 15, 30, 31, 1, SHADOW);
  },
  /** A Hunter banner: deep blue cloth on a gold rod with the Hunter's sigil, a peaked triangle crossed with bars, in white; the hem stirs now and then. */
  destinyhunter(x) { classBanner(x, 0x2a4a8a, (cx) => { for (let k = 0; k < 6; k++) { dot(cx - k, 9 + k * 1.6 | 0, DST_WHITE); dot(cx + k, 9 + k * 1.6 | 0, DST_WHITE); } fill(cx - 3, 14, 7, 1, DST_WHITE); fill(cx - 2, 17, 5, 1, DST_WHITE); }); },
  /** A Titan banner: dark red cloth on a gold rod with the Titan's sigil, a shield of stacked bars in white; the hem stirs now and then. */
  destinytitan(x) { classBanner(x, 0x8a2a2a, (cx) => { fill(cx - 4, 9, 9, 2, DST_WHITE); fill(cx - 3, 12, 7, 2, DST_WHITE); fill(cx - 2, 15, 5, 2, DST_WHITE); dot(cx, 18, DST_WHITE); }); },
  /** A Warlock banner: gold-ochre cloth on a gold rod with the Warlock's sigil, an eye of light inside a diamond, in white; the hem stirs now and then. */
  destinywarlock(x) { classBanner(x, 0x9a7a2a, (cx) => { line(cx, 8, cx + 5, 13, DST_WHITE); line(cx + 5, 13, cx, 18, DST_WHITE); line(cx, 18, cx - 5, 13, DST_WHITE); line(cx - 5, 13, cx, 8, DST_WHITE); disc(cx, 13, 1.6, DST_WHITE); dot(cx, 13, 0x9a7a2a); }); },
  /** The Director: a star map on a dark screen, the Sun at its heart and the planets turning slowly on their rings, Earth blue, Mars red, Venus gold, Saturn ringed; a cursor brackets one destination at a time. */
  destinydirector(x) {
    fill(x - 15, 4, 31, 24, 0x2a3a4a); fill(x - 15, 4, 31, 1, DST_GOLD);
    fill(x - 14, 5, 29, 22, 0x0a1020);
    for (let k = 0; k < 8; k++) dot(x - 13 + hash(k, x) % 27, 6 + hash(x, k) % 20, WHITE, 0.4);
    const cx = x, cy = 16;
    disc(cx, cy, 2, 0xffe080); glowAt(cx, cy, 5, 4, 0xffd060, 0.2);
    const planets = [[5, 3, 0.5, 0xe8c070, 1], [8, 4, 0.35, 0x4a8ae8, 1.4], [11, 5, 0.25, 0xd8603a, 1.2], [14, 7, 0.15, 0xe8d8a0, 1.6]];
    let pick = null;
    planets.forEach(([rx, ry, sp, c, size], i) => {
      ellipseRing(cx, cy, rx, ry * 0.9, 0x3a5a7a, 0.6);
      const a = now * sp + i * 1.7 + x, px = cx + Math.round(Math.cos(a) * rx), py = cy + Math.round(Math.sin(a) * ry * 0.9);
      disc(px, py, size, c); if (i === 3) fill(px - 2, py, 5, 1, 0xc8b880);
      if (i === Math.floor((now + x) / 5) % 4) pick = [px, py];
    });
    if (pick && Math.floor(now * 2) % 2) { const [px, py] = pick; dot(px - 3, py - 3, DST_TEAL); dot(px + 3, py - 3, DST_TEAL); dot(px - 3, py + 3, DST_TEAL); dot(px + 3, py + 3, DST_TEAL); }
    fill(x - 15, 28, 31, 1, SHADOW);
  },
  /** A portrait of Xûr: the hooded agent in green, his face a dark writhing mass under the hood with two pale eyes that brighten every few seconds, in a gilt frame; the eyes glow at night. */
  destinyxur(x) {
    fill(x - 10, 4, 21, 25, 0xb8962a); fill(x - 10, 4, 21, 1, 0xe8c860); fill(x + 10, 4, 1, 25, 0x7a5a1a);
    fill(x - 8, 6, 17, 21, 0x1a2420);
    fill(x - 5, 9, 11, 18, 0x3a5a3a); fill(x - 4, 8, 9, 2, 0x3a5a3a); fill(x - 5, 9, 1, 18, 0x5a7a4a); fill(x + 5, 10, 1, 17, 0x24361e);
    fill(x - 3, 11, 7, 7, 0x0e120e);
    const w = Math.floor(now * 3 + x) % 3;
    for (let k = 0; k < 3; k++) line(x - 2 + k * 2, 15, x - 2 + k * 2 + (w === k ? 1 : 0), 18, 0x2a3a2a);
    const t = (now + x * 0.31) % 7, bright = t < 1.2;
    dot(x - 1, 13, bright ? 0xe8fff0 : 0x9ac8a8); dot(x + 1, 13, bright ? 0xe8fff0 : 0x9ac8a8);
    fill(x - 5, 21, 11, 1, DST_GOLD, 0.6);
    fill(x - 10, 29, 21, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x, 13, 4, 2, 0x9affc0, 0.2); dot(x - 1, 13, 0xe8fff0); dot(x + 1, 13, 0xe8fff0); });
  },
  /** A bridge viewport onto space: two mullions over the stars and a sandy planet; every 30 s the stars stretch into lightspeed lines for two seconds and the panes flash pale blue. */
  starwarshyper(x) {
    fill(x - 12, 4, 25, 24, SW_GREY_DARK); fill(x - 12, 4, 25, 1, SW_GREY_LIGHT); fill(x - 12, 4, 1, 24, SW_GREY);
    fill(x - 11, 5, 23, 22, 0x05070e);
    const t = (now + x * 0.3) % 30;
    if (t < 2) {
      for (let i = 0; i < 16; i++) {
        const a = i * 0.39 + 0.2, r0 = 2 + t * 3, r1 = r0 + 2 + t * 4;
        line(Math.round(x + Math.cos(a) * r0), Math.round(16 + Math.sin(a) * r0 * 0.9), Math.round(x + Math.cos(a) * Math.min(r1, 11)), Math.round(16 + Math.sin(a) * Math.min(r1, 10)), i % 3 ? WHITE : 0xb8e4ff, 0.85);
      }
    } else {
      for (let i = 0; i < 12; i++) { const h = hash(x + i, i * 7); dot(x - 10 + h % 21, 6 + (h >>> 5) % 20, WHITE, 0.4 + ((h >>> 9) % 5) / 10); }
      fill(x + 4, 19, 5, 7, 0xd8a868); fill(x + 3, 20, 7, 5, 0xd8a868); fill(x + 3, 20, 1, 5, 0xf0c890); fill(x + 4, 19, 2, 1, 0xf0c890);
        fill(x + 8, 20, 2, 5, 0x8a6038); fill(x + 7, 24, 2, 2, 0x8a6038); fill(x + 4, 22, 3, 1, 0xc49050);
    }
    if (t >= 2 && t < 2.5) fill(x - 11, 5, 23, 22, 0xc8ecff, 0.4);
    // The mullions and the sill
    fill(x - 4, 5, 1, 22, SW_GREY); fill(x + 4, 5, 1, 22, SW_GREY); fill(x - 12, 27, 25, 2, SW_GREY); fill(x - 12, 27, 25, 1, SW_GREY_LIGHT);
    dot(x - 9, 28, 0xff4a3a); dot(x + 9, 28, 0x5aff7a, (Math.floor(now) % 2) ? 1 : 0.4);
    if (isNight) glowLate(() => { dot(x - 9, 28, 0xffa090); if (t < 2.5) glowAt(x, 16, 12, 10, 0xc8ecff, 0.15); });
  },
  /** A twin-suns poster: two suns setting over orange dunes, a small figure and a vaporator against the sky, a yellow title bar along the foot; every 35 s a speeder zips across the dunes. */
  starwarssuns(x) {
    fill(x - 9, 3, 19, 26, 0x1a1a20); fill(x - 9, 3, 19, 1, 0x3a3a44);
    fill(x - 8, 4, 17, 8, 0xf0a050); fill(x - 8, 12, 17, 6, 0xe8806a); fill(x - 8, 4, 17, 2, 0xf8c070);
    disc(x - 2, 12, 2, 0xfff0a0); disc(x + 3, 14, 1.5, 0xffd070); dot(x - 3, 11, WHITE, 0.8);
    // The dunes, the figure and the vaporator
    fill(x - 8, 18, 17, 6, 0xc89a5a); fill(x - 8, 18, 9, 1, 0xd8b070); fill(x - 8, 21, 17, 3, 0x9a6a3a);
    fill(x - 5, 14, 1, 4, 0x2a1a14); dot(x - 5, 13, 0x2a1a14); fill(x + 5, 13, 1, 5, 0x3a2a24); fill(x + 4, 13, 3, 1, 0x3a2a24); dot(x + 5, 12, 0x3a2a24);
    const t = (now + x * 0.21) % 35;
    if (t < 2) { const sx = x - 8 + Math.floor(t * 8.5); fill(sx, 20, 3, 1, 0x8a909c); dot(sx - 1, 21, 0xd8b070, 0.6); }
    // The title bar
    fill(x - 8, 24, 17, 4, 0x0e0e12); fill(x - 6, 25, 13, 1, 0xf2d35b); fill(x - 5, 26, 11, 1, 0xf2d35b, 0.6);
  },
  /** A lightsaber rack: three hilts on a dark plate, blue, green and red, their three crystals glinting on a ledge; every 20 s one of them ignites, its blade sliding out across the plate, hums for three seconds and slides back. */
  starwarssabers(x) {
    fill(x - 12, 6, 25, 21, 0x24262c); fill(x - 12, 6, 25, 1, 0x3a3d46); fill(x - 12, 6, 1, 21, 0x34363e); fill(x + 12, 7, 1, 20, 0x16171b);
    const t = (now + x * 0.17) % 20, which = Math.floor((now + x * 0.17) / 20) % 3;
    [[10, 0x4aa8ff], [16, 0x4aff6a], [22, 0xff3a3a]].forEach(([ry, colour], i) => {
      dot(x - 10, ry + 1, 0x16171b); dot(x - 3, ry + 1, 0x16171b);
      fill(x - 10, ry, 7, 2, 0xb8bcc4); fill(x - 10, ry, 7, 1, 0xd8dce4); fill(x - 8, ry, 3, 2, 0x26262c); dot(x - 7, ry + 1, 0x8a909c);
      fill(x - 3, ry, 1, 2, 0xd0d4dc); dot(x - 9, ry, colour, 0.9);
      if (i === which && t < 3.4) {
        const len = Math.min(13, Math.floor(Math.min(t, 3.4 - t) * 40));
        if (len > 0) { swBlade(x - 2, ry, x - 2 + len, ry, colour); fill(x - 2, ry + 1, len + 1, 1, colour, 0.5); }
      }
      // Its crystal on the ledge, glinting in turn
      const cx = x + 3 + i * 3, lit = Math.floor(now * 0.7 + i) % 3 === 0;
      dot(cx, 23, colour); dot(cx, 22, shaded(colour, 1.3)); dot(cx, 24, shaded(colour, 0.7)); if (lit) dot(cx, 22, WHITE, 0.8);
      if (isNight) glowLate(() => glowAt(cx, 23, 2, 2, colour, 0.18));
    });
    fill(x + 1, 25, 10, 1, 0x3a3d46); fill(x + 1, 26, 10, 1, 0x16171b);
  },
  /** A smuggler frozen in carbonite: a dark slab with a raised face and hands, a control box below whose lights cycle red and green and glow at night. */
  starwarscarbonite(x) {
    const slab = 0x4a4e56, light = 0x6a6e78, dark = 0x2e3136;
    fill(x - 7, 3, 15, 25, slab); fill(x - 7, 3, 15, 1, light); fill(x - 7, 3, 1, 25, light); fill(x + 7, 4, 1, 24, dark);
    fill(x - 8, 5, 1, 21, dark); fill(x + 8, 5, 1, 21, dark);
    // The relief: a head thrown back, hands raised beside it, the body below
    fill(x - 2, 7, 5, 5, light); fill(x - 2, 7, 5, 1, 0x7e828c); fill(x + 2, 8, 1, 4, dark); fill(x - 1, 9, 1, 1, dark); fill(x + 1, 9, 1, 1, dark); fill(x - 1, 11, 3, 1, dark);
    fill(x - 6, 6, 2, 4, light); dot(x - 6, 5, light); fill(x - 5, 10, 1, 2, dark);
    fill(x + 4, 8, 2, 4, light); dot(x + 5, 7, light); fill(x + 4, 12, 1, 1, dark);
    fill(x - 3, 13, 7, 9, 0x545860); fill(x - 3, 13, 1, 9, light); fill(x + 3, 13, 1, 9, dark); fill(x - 1, 15, 3, 1, dark, 0.6);
    // The control box
    fill(x - 6, 23, 13, 4, 0x26282e); fill(x - 6, 23, 13, 1, 0x3a3d46);
    const beat = Math.floor((now + x * 0.2) * 1.5) % 4;
    [0xff4a3a, 0x5aff7a, 0xff4a3a, 0xf2d35b].forEach((c, i) => dot(x - 4 + i * 3, 25, c, beat === i ? 1 : 0.35));
    if (isNight) glowLate(() => { glowAt(x, 25, 7, 3, beat % 2 ? 0x5aff7a : 0xff4a3a, 0.12); dot(x - 4 + beat * 3, 25, WHITE, 0.8); });
  },
  /** A Rebel banner (brand nod: Star Wars): an orange-red cloth on a grey rod, the starbird in cream in the middle and a swallowtail foot whose tails sway for a moment every 12 s. */
  starwarsbanner(x) {
    fill(x - 7, 3, 15, 1, SW_GREY_LIGHT); dot(x - 7, 3, WHITE, 0.6); dot(x + 7, 3, SW_GREY_DARK);
    const sway = (now + x * 0.29) % 12 < 1.6 ? (Math.floor(now * 4) % 2 ? 1 : -1) : 0;
    fill(x - 6, 4, 13, 20, 0xd0502a); fill(x - 6, 4, 1, 20, 0xe8704a); fill(x + 6, 4, 1, 20, 0x9a3418);
    fill(x - 5, 5, 11, 1, 0xf0e2c8, 0.8);
    swMark('rebel', x, 13, 0xf0e2c8);
    // The swallowtail
    for (let k = 0; k < 4; k++) { fill(x - 6 + sway, 24 + k, 6 - k, 1, 0xd0502a); fill(x + 1 + k + sway, 24 + k, 6 - k, 1, 0xb84020); }
    fill(x - 6, 24, 13, 1, SHADOW);
  },
  /** The battle-station plans: a blueprint pinned at its corners, a grid, the round station with its trench and dish, and a dotted run to the exhaust port, whose red mark blinks and gets bracketed every 12 s. */
  starwarsplans(x) {
    const paper = 0x2a5a9a, grid = 0x3a6cae, ink = 0xd8ecff;
    fill(x - 11, 4, 23, 23, paper); fill(x - 11, 4, 23, 1, 0x4a7cbe); fill(x + 11, 5, 1, 22, 0x1e4478); fill(x - 11, 26, 23, 1, 0x1e4478);
    for (let k = 3; k < 22; k += 4) { fill(x - 11 + k, 5, 1, 21, grid); fill(x - 10, 4 + k, 21, 1, grid); }
    ring(x - 1, 15, 7, ink, 0.9); fill(x - 8, 15, 15, 1, ink, 0.6); ring(x - 4, 12, 2, ink, 0.8); dot(x - 4, 12, ink);
    for (let k = 0; k < 4; k++) dot(x + 1 + k * 2, 17 + (k >> 1), ink, 0.7);
    const t = (now + x * 0.23) % 12, blink = Math.floor(now * 2) % 2;
    dot(x + 8, 19, 0xff4a3a, blink ? 1 : 0.5);
    if (t < 2) { dot(x + 6, 17, 0xf2d35b); dot(x + 10, 17, 0xf2d35b); dot(x + 6, 21, 0xf2d35b); dot(x + 10, 21, 0xf2d35b); }
    for (const [px, py] of [[x - 10, 5], [x + 10, 5], [x - 10, 25], [x + 10, 25]]) dot(px, py, 0xd8402a);
  },
  /** A Hollywood mirror: a white frame ringed with round bulbs over a pale pink glass with a streak of reflection; every 14 s the bulbs chase round it, and at night they glow warm. */
  girly1mirror(x) {
    fill(x - 11, 4, 23, 22, G1_WHITE); fill(x - 11, 4, 23, 1, WHITE); fill(x + 11, 5, 1, 21, 0xd8c0c8); fill(x - 11, 25, 23, 1, 0xd8c0c8);
    fill(x - 8, 7, 17, 16, 0xf4dce6); fill(x - 8, 7, 17, 4, 0xfae8f0); line(x - 6, 20, x + 1, 9, WHITE, 0.7); line(x - 4, 21, x + 3, 10, WHITE, 0.4);
    const t = (now + x * 0.19) % 14, chase = t < 2.5 ? Math.floor(t * 8) : -1;
    const bulbs = [];
    for (let k = 0; k < 5; k++) bulbs.push([x - 8 + k * 4, 5]);
    for (let k = 0; k < 4; k++) bulbs.push([x + 10, 9 + k * 4]);
    for (let k = 0; k < 4; k++) bulbs.push([x - 10, 21 - k * 4]);
    bulbs.forEach(([bx, by], i) => {
      const on = chase < 0 || chase % bulbs.length !== i;
      dot(bx, by, on ? 0xfff4c8 : 0xe8d8a0); dot(bx, by + 1, G1_GOLD_DARK, 0.6);
      if (isNight && on) glowLate(() => { glowAt(bx, by, 3, 3, 0xffe08a, 0.18); dot(bx, by, 0xfff8e0); });
    });
    fill(x - 11, 26, 23, 1, SHADOW);
  },
  /** A pink neon heart (brand nod: Barbie): a heart outline in hot pink tube with a script B inside, on a dark plate; it flickers once every 25 s and glows hard at night. */
  girly1neon(x) {
    fill(x - 10, 5, 21, 21, 0x3a1a30); fill(x - 10, 5, 21, 1, 0x5a2a4a); fill(x + 10, 6, 1, 20, 0x220e1c);
    const t = (now + x * 0.23) % 25, off = t < 0.6 && Math.floor(t * 10) % 2 === 0;
    const tube = (alpha) => {
      const c = 0xff5ab8;
      // The heart's outline
      for (const [dx, dy] of [[-6, 9], [-7, 10], [-7, 11], [-7, 12], [-6, 13], [-5, 14], [-4, 15], [-3, 16], [-2, 17], [-1, 18], [0, 19], [-5, 8], [-4, 8], [-3, 9], [-2, 9], [-1, 10]]) { dot(x + dx, dy, c, alpha); dot(x - dx, dy, c, alpha); }
      dot(x, 10, c, alpha);
      g1Mark('b', x - 2, 11, 0xffd0ec, alpha);
    };
    tube(off ? 0.25 : 1);
    if (!off) glowAt(x, 13, 9, 8, 0xff5ab8, isNight ? 0.12 : 0.06);
    if (isNight && !off) glowLate(() => { glowAt(x, 13, 10, 9, 0xff5ab8, 0.2); tube(0.6); });
  },
  /** A shoe shelf: two white shelves with gold edges holding high heels in pink, gold, white and red; every 10 s a sparkle catches one of them. */
  girly1heels(x) {
    const shelf = (sy) => { fill(x - 12, sy, 25, 2, G1_WHITE); fill(x - 12, sy, 25, 1, G1_GOLD); fill(x - 12, sy + 2, 25, 1, SHADOW); };
    const heel = (hx, sy, c) => {
      fill(hx, sy - 3, 3, 1, c); fill(hx + 2, sy - 2, 2, 1, c); dot(hx + 4, sy - 2, shaded(c, 0.8)); dot(hx, sy - 2, c); dot(hx, sy - 1, shaded(c, 0.7));
      fill(hx + 1, sy - 4, 1, 1, shaded(c, 1.2)); dot(hx + 4, sy - 1, shaded(c, 0.7));
    };
    [[-11, G1_PINK], [-5, G1_GOLD], [1, WHITE], [7, 0xd8402a]].forEach(([dx, c]) => heel(x + dx, 15, c));
    [[-11, 0xf8b8d4], [-5, G1_PINK_DARK], [1, G1_GOLD_LIGHT], [7, G1_PINK_LIGHT]].forEach(([dx, c]) => heel(x + dx, 25, c));
    shelf(15); shelf(25);
    const t = (now + x * 0.29) % 10, which = Math.floor((now + x * 0.29) / 10) % 8;
    if (t < 1) g1Twinkle(x - 10 + (which % 4) * 6, which < 4 ? 11 : 21, t);
  },
  /** A convertible poster in a gold frame: a hot pink open-top car on a seaside road under a sunset, a palm behind; every 18 s a little heart floats up from the car. */
  girly1carposter(x) {
    fill(x - 10, 4, 21, 23, G1_GOLD); fill(x - 10, 4, 21, 1, G1_GOLD_LIGHT); fill(x + 10, 5, 1, 22, G1_GOLD_DARK); fill(x - 10, 26, 21, 1, G1_GOLD_DARK);
    fill(x - 9, 5, 19, 7, 0xf890b0); fill(x - 9, 5, 19, 2, 0xfab0c8); fill(x - 9, 12, 19, 4, 0x7ac8e8); fill(x - 9, 16, 19, 1, 0xd0f0ff, 0.6);
    disc(x + 4, 12, 2.5, 0xffe08a); fill(x - 9, 12, 19, 1, 0xf8b080, 0.5);
    // The palm
    fill(x - 6, 9, 1, 9, 0x8a5a32); for (const [dx, dy] of [[-3, 9], [-2, 8], [1, 8], [2, 9], [-1, 8], [0, 8]]) dot(x - 6 + dx, dy, 0x3a8a4a);
    // The road and the car
    fill(x - 9, 17, 19, 9, 0x6a6a74); fill(x - 9, 21, 19, 1, WHITE, 0.6);
    fill(x - 5, 17, 11, 3, G1_PINK); fill(x - 3, 16, 5, 1, G1_PINK); fill(x - 5, 17, 11, 1, G1_PINK_LIGHT); fill(x + 1, 15, 1, 2, 0xdff0f6, 0.8);
    dot(x - 3, 20, 0x2a2a30); dot(x + 3, 20, 0x2a2a30); dot(x + 5, 18, 0xfff4c0);
    const t = (now + x * 0.27) % 18;
    if (t < 3) g1Mark('smallheart', x - 2, 14 - Math.floor(t * 3), 0xff5ab8, 1 - t / 3);
  },
  /** A heart clock: a hot pink heart with a gold rim around a white face telling the real time, a little gold heart pendulum swinging beneath it. */
  girly1heartclock(x) {
    const swing = [0, 1, 2, 1, 0, -1, -2, -1][Math.floor(now * 4) % 8];
    line(x, 20, x + swing, 25, G1_GOLD); g1Mark('smallheart', x - 1 + swing, 25, G1_GOLD);
    const rows = [[-6, -2, 4], [2, 6, 4], [-7, 7, 5], [-7, 7, 6], [-7, 7, 7], [-7, 7, 8], [-6, 6, 9], [-6, 6, 10], [-5, 5, 11], [-4, 4, 12], [-3, 3, 13], [-2, 2, 14], [-1, 1, 15], [0, 0, 16]];
    rows.forEach(([a, b, ry]) => fill(x + a, ry + 1, b - a + 1, 1, G1_PINK));
    rows.forEach(([a, b, ry]) => { dot(x + a, ry + 1, G1_GOLD); dot(x + b, ry + 1, G1_GOLD_DARK); });
    fill(x - 5, 5, 3, 1, G1_GOLD_LIGHT); fill(x + 3, 5, 3, 1, G1_GOLD);
    disc(x, 10, 3.5, G1_WHITE); ring(x, 10, 3.7, G1_GOLD, 0.8);
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12) / 12 + d.getMinutes() / 720) * Math.PI * 2 - Math.PI / 2;
    line(x, 10, x + Math.round(Math.cos(ma) * 3), 10 + Math.round(Math.sin(ma) * 3), 0x3a2030);
    line(x, 10, x + Math.round(Math.cos(ha) * 2), 10 + Math.round(Math.sin(ha) * 2), G1_PINK_DARK);
    dot(x - 4, 6, WHITE, 0.6);
  },
  /** A window with frilly pink curtains tied back with gold, the sky beyond (a moon at night) and a pot of pink flowers on the sill; every 12 s the curtains sway in a breeze. */
  girly1curtains(x) {
    fill(x - 11, 4, 23, 24, G1_WHITE); fill(x - 10, 5, 21, 21, isNight ? 0x1a2050 : 0x9ad4f4);
    if (isNight) { disc(x + 5, 9, 2, 0xfff4c8); for (const [dx, dy] of [[-6, 8], [-2, 12], [2, 7], [7, 15]]) dot(x + dx, dy, WHITE, 0.7); }
    else { fill(x - 6, 9, 6, 2, WHITE, 0.8); fill(x - 5, 8, 3, 1, WHITE, 0.8); fill(x + 3, 13, 5, 1, WHITE, 0.7); }
    fill(x, 5, 1, 21, G1_WHITE); fill(x - 10, 15, 21, 1, G1_WHITE);
    // The curtains: frilled pelmet and two tied drapes
    const sway = (now + x * 0.31) % 12 < 2 ? Math.floor(now * 3) % 2 : 0;
    fill(x - 13, 2, 27, 3, G1_PINK_LIGHT); for (let k = 0; k < 27; k += 2) dot(x - 13 + k, 5, G1_PINK_LIGHT);
    fill(x - 13, 2, 27, 1, 0xf890c4);
    for (const side of [-1, 1]) {
      const ox = side < 0 ? x - 13 : x + 9;
      fill(ox, 5, 5, 8, G1_PINK); fill(ox + (side < 0 ? 1 : 0) + sway * side, 13, 3, 3, G1_PINK); fill(ox + sway * side, 16, 5, 12, G1_PINK);
      fill(ox + 2, 5, 1, 8, G1_PINK_DARK); fill(ox + 2 + sway * side, 16, 1, 12, G1_PINK_DARK);
      fill(ox, 13, 5, 1, G1_GOLD); dot(ox + 2, 14, G1_GOLD_LIGHT);
    }
    // The sill and the flowers
    fill(x - 11, 26, 23, 2, G1_WHITE); fill(x - 11, 28, 23, 1, SHADOW);
    fill(x - 2, 23, 4, 3, 0xd8806a); fill(x - 2, 23, 4, 1, 0xe89a80); dot(x - 2, 21, G1_PINK_LIGHT); dot(x + 1, 21, 0xff9ad0); dot(x - 1, 20, G1_PINK); dot(x, 22, 0x4a9a5a); dot(x - 1, 22, 0x4a9a5a);
  },
  /** A pastel rainbow painted on the wall: five soft bands arching between two puffy clouds; every 15 s a sparkle runs along it. */
  girly2rainbow(x) {
    const bands = [0xf8b8c8, 0xfcd0b0, 0xfff0a8, 0xbce8cc, 0xbcd4f4];
    bands.forEach((c, i) => { const r = 11 - i; for (let a = 0; a <= 32; a++) { const t = Math.PI + (a / 32) * Math.PI; dot(x + Math.round(Math.cos(t) * r), 22 + Math.round(Math.sin(t) * r * 0.9), c); dot(x + Math.round(Math.cos(t) * r), 23 + Math.round(Math.sin(t) * r * 0.9), c); } });
    for (const cx of [x - 10, x + 10]) { fill(cx - 4, 21, 9, 4, WHITE); fill(cx - 2, 19, 5, 2, WHITE); fill(cx - 4, 24, 9, 1, 0xe0e4f0); dot(cx - 1, 22, G2_PINK_DARK, 0.6); dot(cx + 2, 22, G2_PINK_DARK, 0.6); }
    const t = (now + x * 0.2) % 15;
    if (t < 2) { const a = Math.PI + (t / 2) * Math.PI; g1Twinkle(x + Math.round(Math.cos(a) * 9), 22 + Math.round(Math.sin(a) * 8), (t * 3) % 1); }
  },
  /** A kitty frame (brand nod: Hello Kitty): a round pink scalloped frame around her face on a mint ground, her bow red and her nose yellow; every 20 s she winks. */
  girly2kittyframe(x) {
    disc(x, 15, 9, G2_PINK_DARK); for (let k = 0; k < 12; k++) { const a = k * Math.PI / 6; dot(x + Math.round(Math.cos(a) * 9.5), 15 + Math.round(Math.sin(a) * 9.5), G2_PINK_DARK); }
    disc(x, 15, 7.5, G2_MINT); dot(x - 5, 10, WHITE, 0.6);
    g2Kitty(x, 16, 1, (now + x * 0.23) % 20 < 0.6);
    ring(x, 15, 9, 0xf8d8e4, 0.6);
  },
  /** A string of photos: four little instant photos of a cat, a heart, a star and a cloud pegged on a cord with fairy lights between them that glow at night; every 11 s one photo sways. */
  girly2polaroids(x) {
    line(x - 13, 6, x, 9, 0xb8a8c8); line(x, 9, x + 13, 6, 0xb8a8c8);
    const t = (now + x * 0.29) % 11, which = Math.floor((now + x * 0.29) / 11) % 4;
    [[-10, 8], [-4, 9], [3, 9], [9, 8]].forEach(([dx, dy], i) => {
      const sw = i === which && t < 1.5 ? (Math.floor(t * 4) % 2 ? 1 : -1) : 0, px = x + dx - 2 + sw;
      fill(px, dy + 1, 6, 7, G2_CREAM); fill(px, dy + 1, 6, 1, WHITE); fill(px, dy + 7, 6, 1, 0xe0d8d0);
      fill(px + 1, dy + 2, 4, 4, [G2_MINT, G2_PINK, G2_LILAC, 0xbce4f8][i]);
      if (i === 0) { dot(px + 2, dy + 4, WHITE); dot(px + 3, dy + 4, WHITE); dot(px + 2, dy + 3, WHITE); dot(px + 3, dy + 3, WHITE); dot(px + 2, dy + 5, G2_BOW); }
      if (i === 1) { g1Mark('smallheart', px + 2, dy + 3, G2_BOW); }
      if (i === 2) { dot(px + 2, dy + 3, G2_NOSE); dot(px + 3, dy + 4, G2_NOSE); dot(px + 2, dy + 4, G2_NOSE); dot(px + 1, dy + 4, G2_NOSE); dot(px + 2, dy + 5, G2_NOSE); }
      if (i === 3) { fill(px + 1, dy + 4, 4, 1, WHITE); dot(px + 2, dy + 3, WHITE); }
      fill(px + 2, dy, 2, 2, G2_PINK_DARK);
    });
    [[-7, 7], [0, 9], [6, 8]].forEach(([dx, dy], i) => { const c = [0xfff0a0, G2_PINK_DARK, 0xa8e0f8][i]; dot(x + dx, dy, c); if (isNight) glowLate(() => { glowAt(x + dx, dy, 3, 3, c, 0.2); dot(x + dx, dy, WHITE); }); });
  },
  /** A sleepy moon mobile: a pale yellow crescent with a closed eye and a pink cheek hanging from a cream bar, two little stars dangling beside it; it bobs gently, and glows softly at night. */
  girly2moon(x) {
    fill(x - 8, 3, 17, 1, G2_CREAM); dot(x, 2, G2_CREAM);
    const bob = Math.round(Math.sin(now * 0.7 + x) * 1), bob2 = Math.round(Math.sin(now * 0.9 + x + 2) * 1);
    line(x, 4, x, 9 + bob, 0xd8d0c8); line(x - 7, 4, x - 7, 15 + bob2, 0xd8d0c8); line(x + 7, 4, x + 7, 12 - bob2, 0xd8d0c8);
    const moon = () => {
      for (let dy = -6; dy <= 6; dy++) for (let dx = -6; dx <= 6; dx++) if (dx * dx + dy * dy <= 36 && (dx - 3) * (dx - 3) + (dy + 2) * (dy + 2) > 25) dot(x + dx, 16 + bob + dy, dx < -4 ? 0xfff8c8 : 0xfff0a0);
      fill(x - 4, 15 + bob, 2, 1, 0x8a6a40); dot(x - 3, 17 + bob, G2_PINK_DARK);
    };
    moon();
    for (const [sx, sy] of [[x - 7, 16 + bob2], [x + 7, 13 - bob2]]) { dot(sx, sy, 0xfff0a0); dot(sx - 1, sy, 0xfff0a0); dot(sx + 1, sy, 0xfff0a0); dot(sx, sy - 1, 0xfff0a0); dot(sx, sy + 1, 0xfff0a0); }
    if (isNight) glowLate(() => { glowAt(x - 1, 16 + bob, 8, 7, 0xfff0a0, 0.16); });
  },
  /** A cloud shelf of plushies: a white kitty with her bow, a pink bunny and a lilac bear sitting on a puffy white cloud; every 13 s one of them gives a little hop. */
  girly2plushshelf(x) {
    const t = (now + x * 0.31) % 13, which = Math.floor((now + x * 0.31) / 13) % 3, hop = (i) => (i === which && t < 0.8 ? 1 : 0);
    // The bunny, the kitty and the bear
    let h = hop(0); fill(x - 10, 14 - h, 5, 6, G2_PINK); fill(x - 10, 9 - h, 2, 5, G2_PINK); fill(x - 7, 9 - h, 2, 5, G2_PINK); dot(x - 9, 10 - h, WHITE); fill(x - 9, 15 - h, 3, 2, WHITE); dot(x - 9, 15 - h, 0x1f1a1a); dot(x - 7, 15 - h, 0x1f1a1a);
    h = hop(1); g2Kitty(x, 15 - h, 1); fill(x - 2, 18 - h, 5, 3, 0x8ab4e8);
    h = hop(2); fill(x + 6, 14 - h, 6, 6, G2_LILAC); dot(x + 6, 13 - h, G2_LILAC); dot(x + 11, 13 - h, G2_LILAC); fill(x + 8, 16 - h, 2, 2, 0xe8dcf8); dot(x + 7, 15 - h, 0x1f1a1a); dot(x + 10, 15 - h, 0x1f1a1a); dot(x + 8, 16 - h, 0x5a4a6a);
    // The cloud shelf
    fill(x - 12, 21, 25, 4, WHITE); fill(x - 10, 20, 6, 1, WHITE); fill(x - 1, 20, 7, 1, WHITE); fill(x + 8, 20, 4, 1, WHITE);
    fill(x - 12, 24, 25, 1, 0xe0e4f0); fill(x - 12, 25, 25, 1, SHADOW);
  },
  /** A cloud clock: a puffy white cloud with rosy cheeks round a cream face telling the real time; every 30 s the cloud's sleepy eyes blink open. */
  girly2cloudclock(x) {
    fill(x - 9, 10, 19, 9, WHITE); fill(x - 6, 7, 6, 3, WHITE); fill(x + 1, 6, 5, 4, WHITE); fill(x - 10, 12, 1, 6, WHITE); fill(x + 10, 12, 1, 6, WHITE);
    fill(x - 9, 18, 19, 1, 0xe0e4f0); dot(x - 5, 7, 0xf4f8ff);
    disc(x, 13, 4, G2_CREAM); ring(x, 13, 4.2, G2_LILAC, 0.9);
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12) / 12 + d.getMinutes() / 720) * Math.PI * 2 - Math.PI / 2;
    line(x, 13, x + Math.round(Math.cos(ma) * 3), 13 + Math.round(Math.sin(ma) * 3), 0x6a5a7a); line(x, 13, x + Math.round(Math.cos(ha) * 2), 13 + Math.round(Math.sin(ha) * 2), G2_PINK_DARK);
    const awake = (now + x * 0.2) % 30 < 2;
    for (const ex of [x - 7, x + 7]) { if (awake) dot(ex, 13, 0x1f1a1a); else { dot(ex - 1, 13, 0x8a8088); dot(ex, 14, 0x8a8088); dot(ex + 1, 13, 0x8a8088); } dot(ex, 16, G2_PINK_DARK, 0.8); }
    fill(x - 8, 19, 17, 1, SHADOW);
  },
  /** A wanted poster: yellowed paper with a dark title band, a portrait of a curly-haired man with a moustache, a reward line with a dollar sign; every 15 s a breeze lifts its bottom corner. */
  narcoswanted(x) {
    const paper = 0xe8d8a8, ink = 0x3a2a1a;
    fill(x - 7, 4, 15, 23, paper); fill(x - 7, 4, 15, 1, 0xf4e8c0); fill(x + 7, 5, 1, 22, 0xc8b480);
    fill(x - 6, 5, 13, 3, ink); for (let k = 0; k < 6; k++) fill(x - 5 + k * 2, 6, 1, 1, paper);
    fill(x - 5, 9, 11, 10, 0xc8b48a); ncBoss(x, 10);
    fill(x - 1, 17, 3, 2, 0x8ab8e0);
    fill(x - 5, 21, 1, 3, ink); dot(x - 6, 22, ink); dot(x - 4, 22, ink); fill(x - 3, 22, 9, 1, ink); fill(x - 3, 24, 7, 1, ink, 0.6);
    dot(x - 6, 5, 0xd8402a); dot(x + 6, 5, 0xd8402a);
    const lift = (now + x * 0.27) % 15 < 1.5;
    if (lift) { fill(x + 5, 25, 3, 2, 0xc8b480); dot(x + 6, 24, 0xf4e8c0); } else fill(x + 7, 26, 1, 1, 0xc8b480);
  },
  /** A route map in a dark frame: blue sea, a green coast with mountains to the south-west, a peninsula to the north-east, red pins, and a dotted route along which a tiny plane flies every 20 s. */
  narcosmap(x) {
    fill(x - 12, 4, 25, 22, 0x3a2a1a); fill(x - 12, 4, 25, 1, 0x5a4030);
    fill(x - 11, 5, 23, 20, 0x8ac8e0); for (let k = 0; k < 23; k += 4) fill(x - 11 + k, 5, 1, 20, 0x9ad4ea, 0.6);
    fill(x - 11, 17, 9, 8, 0x8ab86a); fill(x - 11, 15, 6, 2, 0x8ab86a); fill(x - 9, 14, 3, 1, 0x8ab86a); dot(x - 8, 18, 0x6a8a4a); dot(x - 7, 19, 0x6a8a4a); dot(x - 9, 20, 0x6a8a4a);
    fill(x + 6, 5, 6, 3, 0x8ab86a); fill(x + 7, 8, 3, 5, 0x8ab86a); dot(x + 8, 13, 0x8ab86a);
    const route = [[-7, 16], [-5, 14], [-3, 13], [-1, 12], [1, 11], [3, 11], [5, 10], [7, 9]];
    route.forEach(([dx, dy], i) => { if (i % 2 === 0) dot(x + dx, dy, 0xf4f0e8); });
    dot(x - 7, 16, 0xd8202a); dot(x - 7, 15, 0xff6a5a); dot(x + 8, 9, 0xd8202a); dot(x + 8, 8, 0xff6a5a); dot(x - 9, 21, 0xd8202a);
    const t = (now + x * 0.3) % 20;
    if (t < 4) { const [dx, dy] = route[Math.min(7, Math.floor(t * 2))]; fill(x + dx - 1, dy, 3, 1, WHITE); dot(x + dx, dy - 1, WHITE); }
  },
  /** The boss's portrait: a gilded frame round an oil painting of a curly-haired man with a moustache in a pale blue shirt against a deep green ground, a brass picture light above that glows at night. */
  narcosportrait(x) {
    fill(x - 9, 5, 19, 23, NC_GOLD); fill(x - 9, 5, 19, 1, NC_GOLD_LIGHT); fill(x - 9, 5, 1, 23, NC_GOLD_LIGHT); fill(x + 9, 6, 1, 22, NC_GOLD_DARK); fill(x - 9, 27, 19, 1, NC_GOLD_DARK);
    for (const [cx, cy] of [[x - 9, 5], [x + 9, 5], [x - 9, 27], [x + 9, 27]]) { dot(cx, cy, NC_GOLD_LIGHT); }
    fill(x - 7, 7, 15, 19, 0x1e3a28); fill(x - 7, 7, 15, 3, 0x2a4a32);
    ncBoss(x, 10);
    fill(x - 5, 19, 11, 7, 0x8ab8e0); fill(x - 5, 19, 1, 7, 0xa8d0f0); fill(x - 1, 19, 3, 2, 0xd8a878); fill(x - 2, 19, 1, 2, 0xe8f0f8); fill(x + 2, 19, 1, 2, 0xe8f0f8);
    fill(x - 4, 3, 9, 2, 0xb89040); fill(x - 3, 3, 7, 1, 0xd8b060); fill(x, 2, 1, 1, 0x8a6a28);
    glowAt(x, 9, 9, 5, 0xffe0a0, isNight ? 0.1 : 0.04);
    if (isNight) glowLate(() => { glowAt(x, 8, 10, 6, 0xffe0a0, 0.16); fill(x - 3, 4, 7, 1, 0xfff0c0); });
  },
  /** A Miami window: a pink and orange sunset over a teal sea with two palms against it, purple with stars at night and the frame's pink neon trim glowing; every 30 s a bird crosses. */
  narcospalms(x) {
    fill(x - 12, 4, 25, 24, 0xf4f0ec); fill(x - 11, 5, 23, 22, isNight ? 0x3a1a5a : 0xf8a080);
    if (isNight) { fill(x - 11, 5, 23, 6, 0x24104a); for (const [dx, dy] of [[-8, 7], [-3, 9], [3, 6], [8, 10], [0, 12]]) dot(x + dx, dy, WHITE, 0.7); }
    else { fill(x - 11, 5, 23, 5, 0xf870a0); fill(x - 11, 10, 23, 3, 0xf888a0); }
    disc(x + 3, 17, 3, isNight ? 0xf0e0f8 : 0xffd060); fill(x - 11, 18, 23, 9, isNight ? 0x1a4a5a : 0x3ab8b0); fill(x - 11, 18, 23, 1, isNight ? 0x6a5a8a : 0xffc080, 0.8);
    for (const k of [0, 6, 13]) fill(x - 10 + k, 21 + (k % 3), 4, 1, WHITE, 0.4);
    // The palms
    for (const [px, top] of [[x - 7, 8], [x + 8, 11]]) {
      line(px, 26, px + 1, top, 0x2a1a24); dot(px + 1, top + 4, 0x2a1a24);
      for (const [dx, dy] of [[-3, 0], [-2, 0], [-1, -1], [0, -1], [1, -1], [2, 0], [3, 0], [-4, 1], [4, 1], [-5, 2], [5, 2], [-5, 3], [5, 3], [-2, 1], [2, 1], [-3, 2], [3, 2], [0, 0]]) dot(px + 1 + dx, top + dy, (dx + dy) % 2 ? 0x1a3a24 : 0x24502e);
    }
    const t = (now + x * 0.21) % 30;
    if (t < 3) { const bx = x - 11 + Math.floor(t * 7.5), w = Math.floor(now * 4) % 2; dot(bx, 9 + w, 0x2a1a24); dot(bx + 1, 10, 0x2a1a24); dot(bx + 2, 9 + w, 0x2a1a24); }
    fill(x, 5, 1, 22, 0xf4f0ec); fill(x - 12, 27, 25, 1, SHADOW);
    if (isNight) glowLate(() => { for (const [px, py, w, h] of [[x - 12, 4, 25, 1], [x - 12, 4, 1, 24], [x + 12, 4, 1, 24]]) fill(px, py, w, h, 0xff6ac0, 0.8); glowAt(x, 15, 14, 13, 0xff6ac0, 0.08); });
  },
  /** A wall safe: a steel door set in the wall with a dial and a handle; every 25 s it swings open for four seconds, showing gold bars and stacks of cash glinting inside. */
  narcossafe(x) {
    const steel = 0x6e727c, light = 0xa8adb6, dark = 0x3a3e48;
    fill(x - 7, 6, 15, 18, 0x2a2a30); fill(x - 6, 7, 13, 16, 0x1a1a20);
    const t = (now + x * 0.29) % 25;
    if (t < 4) {
      fill(x - 6, 15, 13, 1, dark); ncStack(x - 5, 11, 5, 4); ncStack(x + 1, 11, 5, 4);
      for (const gx of [x - 5, x - 1, x + 3]) { fill(gx, 20, 3, 2, NC_GOLD); dot(gx, 20, NC_GOLD_LIGHT); }
      dot(x - 1 + Math.floor(t * 2) % 4, 20, WHITE, 0.9);
      fill(x - 9, 7, 3, 16, steel); fill(x - 9, 7, 1, 16, light);
    } else {
      fill(x - 6, 7, 13, 16, steel); fill(x - 6, 7, 13, 1, light); fill(x - 6, 7, 1, 16, light); fill(x + 6, 8, 1, 15, dark); fill(x - 6, 22, 13, 1, dark);
      disc(x - 1, 14, 3, dark); disc(x - 1, 14, 2, 0x8a8e98); dot(x - 1, 12, WHITE); const d = Math.floor(now * 0.5 + x) % 4; dot(x - 1 + [0, 1, 0, -1][d], 14 + [-1, 0, 1, 0][d], dark);
      fill(x + 3, 12, 2, 5, light); fill(x + 3, 12, 2, 1, WHITE);
    }
    fill(x - 7, 24, 15, 1, SHADOW);
  },
  /** A framed football shirt (brand nod: Atlético Nacional): green and white vertical stripes, a white collar and a small green crest, in a dark frame; a glint crosses the glass every 20 s. */
  narcosjersey(x) {
    fill(x - 10, 4, 21, 24, 0x2a2a30); fill(x - 10, 4, 21, 1, 0x4a4a54); fill(x - 8, 6, 17, 20, 0xe8e4dc);
    const G = 0x1a8a4a;
    fill(x - 4, 9, 9, 14, WHITE); fill(x - 8, 9, 4, 5, WHITE); fill(x + 5, 9, 4, 5, WHITE);
    for (const dx of [-3, -1, 1, 3]) fill(x + dx, 9, 1, 14, G); fill(x - 7, 9, 1, 5, G); fill(x - 5, 9, 1, 5, G); fill(x + 6, 9, 1, 5, G); fill(x + 8, 9, 1, 5, G);
    fill(x - 2, 8, 5, 1, G); dot(x, 9, 0xe8e4dc); dot(x + 2, 11, 0x0e5a2a); dot(x + 2, 12, NC_GOLD);
    fill(x - 4, 22, 9, 1, 0xc8c4bc);
    const g = (now + x * 0.21) % 20;
    if (g < 1.2) { const gx = x - 8 + Math.floor(g * 14); line(gx, 7, gx - 4, 24, WHITE, 0.5); }
  },
  /** A backlit apple (brand nod: Apple): a big white apple mark on a space grey plaque, glowing softly round its edge by day and brightly at night. */
  apple1logo(x) {
    fill(x - 9, 3, 19, 25, A1_SPACE); fill(x - 8, 2, 17, 1, A1_SPACE); fill(x - 8, 28, 17, 1, A1_SPACE); fill(x - 9, 3, 19, 1, 0x5a5a5e); fill(x + 9, 4, 1, 24, 0x2c2c2e);
    glowAt(x, 15, 10, 10, WHITE, isNight ? 0.12 : 0.1);
    appleMark('big', x - 7, 7, 0x8a8a90, 1, 2);
    appleMark('big', x - 7, 6, WHITE, 1, 2);
    if (isNight) glowLate(() => { glowAt(x, 15, 12, 12, 0xf0f4ff, 0.22); appleMark('big', x - 7, 6, WHITE, 0.6, 2); });
  },
  /** A tall glass wall onto a campus: trees and lawn beyond with the curve of a ring building, the sky blue by day; at night dark blue, the ring's windows lit. */
  apple1window(x) {
    fill(x - 13, 3, 27, 26, A1_SILVER); fill(x - 12, 4, 25, 24, isNight ? 0x101a38 : 0xa8d8f4);
    if (!isNight) { fill(x - 12, 4, 25, 5, 0xc8e8fa); fill(x - 8, 7, 6, 1, WHITE, 0.8); }
    // The ring building's curve and its glass band
    for (let k = -12; k <= 12; k++) { const yy = 15 + Math.round((k * k) / 40); fill(x + k, yy, 1, 3, isNight ? 0x2a3450 : 0xe8eaec); dot(x + k, yy + 1, isNight ? ((k + 13) % 3 ? 0xffe8a0 : 0x2a3450) : 0x9ac0d8); }
    // Trees and lawn
    fill(x - 12, 22, 25, 6, isNight ? 0x1a3a24 : 0x6ab85a);
    for (const tx of [-10, -5, 1, 7, 11]) { disc(x + tx, 21, 2.6, isNight ? 0x1e4a2a : 0x3a8a3a); dot(x + tx - 1, 20, isNight ? 0x2a5a34 : 0x5aa84a); }
    fill(x, 4, 1, 24, A1_SILVER); fill(x - 13, 28, 27, 1, SHADOW);
    if (isNight) glowLate(() => glowAt(x, 16, 12, 3, 0xffe8a0, 0.08));
  },
  /** A wallpaper poster in a thin white frame: soft swirls of purple, pink and orange flowing into each other; every 20 s the swirl shifts a step. */
  apple1poster(x) {
    fill(x - 9, 3, 19, 25, WHITE); fill(x - 9, 27, 19, 1, 0xd8d8dc);
    const k = Math.floor((now + x * 0.2) / 20) % 3, [c1, c2, c3] = A1_WALLS[k];
    fill(x - 8, 4, 17, 23, c1);
    for (let r = 0; r < 23; r++) { const w = Math.round(8 + Math.sin(r * 0.35 + k) * 5); fill(x - 8 + 17 - w, 4 + r, w, 1, c2, 0.85); }
    for (let r = 8; r < 23; r++) { const w = Math.round(4 + Math.sin(r * 0.5 + 1 + k) * 3 + (r - 8) * 0.4); fill(x - 8, 4 + r, w, 1, c3, 0.8); }
    line(x - 6, 24, x + 6, 6, WHITE, 0.15);
  },
  /** A phone wall: a white shelf with three phones standing on it, their screens bright with wallpapers that change every 15 s. */
  apple1phones(x) {
    fill(x - 12, 22, 25, 2, WHITE); fill(x - 12, 24, 25, 1, 0xd8d8dc); fill(x - 12, 25, 25, 1, SHADOW);
    const k = Math.floor((now + x * 0.3) / 15);
    [-8, 0, 8].forEach((dx, i) => {
      const [c1, c2, c3] = A1_WALLS[(k + i) % 3], px = x + dx - 2;
      fill(px, 9, 6, 13, 0x2c2c2e); fill(px + 1, 10, 4, 11, c1); fill(px + 1, 15, 4, 3, c2); fill(px + 1, 18, 4, 3, c3); fill(px + 2, 10, 2, 1, 0x1c1c1e);
      dot(px + 1, 11, WHITE, 0.6);
      if (isNight) glowLate(() => glowAt(px + 3, 15, 4, 6, c2, 0.12));
    });
  },
  /** A watch display: four watches on a white panel, their bands in coral, blue, green and black, faces dark with the time in white; every 12 s one wakes, its face lighting up with activity rings. */
  apple1watches(x) {
    fill(x - 11, 5, 23, 22, WHITE); fill(x - 11, 26, 23, 1, 0xd8d8dc);
    const k = Math.floor((now + x * 0.25) / 12) % 4, awake = (now + x * 0.25) % 12 < 2.5;
    [[-8, 0xf07a6a], [-3, A1_BLUE], [2, 0x5ac88a], [7, 0x2c2c2e]].forEach(([dx, band], i) => {
      fill(x + dx - 1, 7, 3, 18, band); fill(x + dx - 1, 7, 1, 18, shaded(band, 1.15));
      fill(x + dx - 2, 13, 5, 6, A1_SILVER); fill(x + dx - 1, 14, 3, 4, 0x0c0c0e);
      if (i === k && awake) { ring(x + dx, 16, 1.4, 0xff2d55); dot(x + dx, 16, 0x34c759); dot(x + dx + 1, 15, 0x5ac8fa); }
      else { dot(x + dx, 15, WHITE, 0.8); dot(x + dx, 16, WHITE, 0.5); }
    });
  },
  /** A "hello" card: the word written in one flowing line of colour on white, the stroke passing from green through yellow and orange to pink and blue; every 30 s it writes itself again from the left. */
  apple1hello(x) {
    fill(x - 13, 7, 27, 17, WHITE); fill(x - 13, 23, 27, 1, 0xd8d8dc); fill(x - 13, 24, 27, 1, SHADOW);
    const path = [[0, 0], [0, -1], [0, -2], [0, -3], [0, -4], [0, -5], [0, -6], [1, -7], [1, -3], [2, -3], [3, -2], [3, -1], [3, 0], [4, 0],
      [5, -1], [6, -1], [7, -2], [6, -3], [5, -2], [5, -1], [6, 0], [7, 0], [8, 0], [9, -1], [9, -2], [9, -3], [9, -4], [9, -5], [10, -6], [11, -5], [10, -4],
      [10, 0], [11, 0], [12, -1], [12, -2], [12, -3], [12, -4], [12, -5], [13, -6], [14, -5], [13, -4], [13, 0], [14, 0], [15, -1], [15, -2], [16, -3], [17, -3], [18, -2], [18, -1], [17, 0], [16, 0], [19, -2], [20, -2]];
    const t = (now + x * 0.17) % 30, shown = t < 2 ? Math.floor((t / 2) * path.length) : path.length;
    const cols = [0x34c759, 0xffcc00, 0xff9500, 0xff2d55, 0xaf52de, A1_BLUE];
    for (let i = 0; i < shown; i++) { const [dx, dy] = path[i]; dot(x - 10 + dx, 18 + dy, cols[Math.min(5, Math.floor((dx / 21) * 6))]); }
  },
  /** The rainbow apple (brand nod: Apple): a big six-stripe apple sign standing off the wall on a beige plaque; every 20 s a shine runs down it. */
  apple2logo(x) {
    fill(x - 9, 4, 19, 22, A2_BEIGE); fill(x - 9, 4, 19, 1, A2_BEIGE_LIGHT); fill(x + 9, 5, 1, 21, A2_BEIGE_DARK); fill(x - 9, 25, 19, 1, A2_BEIGE_DARK);
    a2Apple(x - 7, 7, 2);
    const g = (now + x * 0.2) % 20;
    if (g < 1.2) fill(x - 7, 7 + Math.floor(g * 13), 14, 1, WHITE, 0.4);
    fill(x - 9, 26, 19, 1, SHADOW);
  },
  /** A "think different" poster: a black-and-white portrait of a wild-haired thinker on black, a white caption line and a little rainbow apple in the corner; every 25 s a glint crosses it. */
  apple2think(x) {
    fill(x - 9, 3, 19, 25, 0x0e0e10); fill(x - 9, 3, 19, 1, 0x2a2a2e);
    // The wild white hair, the face, the moustache
    for (const [dx, dy] of [[-4, 6], [-3, 5], [-2, 5], [-1, 4], [0, 4], [1, 4], [2, 5], [3, 5], [4, 6], [-5, 8], [5, 8], [-5, 10], [5, 10], [-4, 7], [4, 7], [3, 4], [-2, 4]]) dot(x + dx, dy, 0xe8e8e8);
    fill(x - 3, 7, 7, 8, 0xb8b8b8); fill(x - 4, 9, 9, 4, 0xb8b8b8); fill(x - 3, 7, 1, 8, 0xd0d0d0); fill(x + 3, 8, 1, 7, 0x8a8a8a);
    dot(x - 2, 10, A2_INK); dot(x + 2, 10, A2_INK); fill(x - 2, 13, 5, 1, 0xe8e8e8); dot(x, 12, 0x8a8a8a);
    fill(x - 4, 15, 9, 6, 0x4a4a4e); fill(x - 1, 15, 3, 2, 0xd0d0d0);
    fill(x - 7, 23, 9, 1, 0xe8e8e8); fill(x - 7, 25, 6, 1, 0xa8a8a8);
    appleMark('small', x + 4, 21, (r) => A2_RAINBOW[r]);
    const g = (now + x * 0.23) % 25;
    if (g < 1.2) { const gx = x - 9 + Math.floor(g * 16); line(gx, 4, gx - 4, 26, WHITE, 0.25); }
  },
  /** A 1984 poster: a grey-blue screen with a stern face looming over rows of grey figures, and a runner in red shorts mid-swing; every 20 s the hammer flies, the screen flashes white and the figures glow. */
  apple21984(x) {
    fill(x - 12, 3, 25, 25, 0x2a3038); fill(x - 12, 3, 25, 1, 0x4a5058);
    const t = (now + x * 0.27) % 20, flash = t > 1.2 && t < 1.8;
    fill(x - 9, 5, 19, 12, flash ? WHITE : 0x8a9aa8); fill(x - 9, 5, 19, 1, flash ? WHITE : 0xa8b8c4);
    if (!flash) { fill(x - 3, 7, 7, 8, 0x5a6a78); fill(x - 2, 6, 5, 1, 0x5a6a78); dot(x - 2, 10, 0x1a2228); dot(x + 2, 10, 0x1a2228); fill(x - 1, 13, 3, 1, 0x1a2228); fill(x - 3, 9, 7, 1, 0x4a5a68, 0.6); }
    for (let k = 0; k < 6; k++) { fill(x - 11 + k * 4, 22, 2, 5, 0x5a6068); dot(x - 11 + k * 4, 21, 0x6a7078); if (flash) dot(x - 10 + k * 4, 22, WHITE, 0.6); }
    // The runner and the hammer
    const fly = t < 1.2 ? Math.floor(t * 6) : -1;
    fill(x + 7, 19, 3, 3, A2_PAPER); fill(x + 7, 22, 3, 2, 0xe03a3e); fill(x + 7, 24, 1, 3, 0xe0b090); fill(x + 9, 24, 1, 3, 0xe0b090); fill(x + 7, 17, 3, 2, 0xf0d070); dot(x + 7, 18, 0xe0b090);
    if (fly >= 0) { line(x + 6, 19, x + 3 - fly, 16 - fly, 0x8a8a90); fill(x + 2 - fly, 15 - fly, 2, 2, 0x4a4a52); }
    else { line(x + 10, 19, x + 12, 15, 0x8a8a90); fill(x + 11, 14, 2, 2, 0x4a4a52); }
    if (isNight && flash) glowLate(() => glowAt(x, 11, 11, 7, WHITE, 0.25));
  },
  /** A framed floppy disk: a blue three-and-a-half-inch disk with its metal shutter and a white label carrying a rainbow apple; every 15 s the shutter slides open and back. */
  apple2floppy(x) {
    fill(x - 8, 4, 17, 21, A2_BEIGE_DARK); fill(x - 7, 5, 15, 19, A2_PAPER);
    fill(x - 5, 7, 11, 14, 0x2a5ab8); fill(x - 5, 7, 11, 1, 0x4a7ad8); fill(x + 5, 8, 1, 13, 0x1a3a80); dot(x + 5, 7, A2_PAPER);
    const open = (now + x * 0.29) % 15 < 2 ? 2 : 0;
    fill(x - 2 + open, 7, 6, 5, 0xc8ccd4); fill(x - 2 + open, 7, 6, 1, 0xe8ecf2); fill(x - 1 + open, 8, 2, 3, 0x6a6e78); if (open) fill(x - 2, 8, 2, 3, 0x1a1a1a);
    fill(x - 4, 14, 9, 6, A2_PAPER); fill(x - 4, 14, 9, 1, 0xd8d8d0); appleMark('small', x - 2, 14, (r) => A2_RAINBOW[r]);
  },
  /** A framed paint window: a striped title bar, a column of tools down the left and a white canvas where a little house and sun are drawn in black; every 18 s the pencil adds the path to the door, dot by dot. */
  apple2macpaint(x) {
    fill(x - 12, 4, 25, 22, A2_INK); fill(x - 11, 5, 23, 20, A2_PAPER);
    for (let k = 0; k < 3; k++) fill(x - 11, 6 + k, 23, 1, k % 2 ? A2_PAPER : A2_INK, 0.8); fill(x - 3, 5, 7, 4, A2_PAPER); fill(x - 2, 6, 5, 1, A2_INK, 0.6);
    fill(x - 11, 9, 23, 1, A2_INK);
    fill(x - 8, 10, 1, 15, A2_INK); for (let k = 0; k < 6; k++) { fill(x - 11, 11 + k * 2, 2, 1, A2_INK, 0.7); }
    // The drawing: a sun, a house with a roof and door
    ring(x + 7, 13, 2, A2_INK); for (const [dx, dy] of [[0, -4], [4, 0], [-4, 0], [3, 3], [-3, -3], [3, -3]]) dot(x + 7 + dx, 13 + dy, A2_INK);
    line(x - 4, 17, x, 13, A2_INK); line(x, 13, x + 4, 17, A2_INK); fill(x - 3, 17, 7, 1, A2_INK); fill(x - 3, 17, 1, 5, A2_INK); fill(x + 3, 17, 1, 5, A2_INK); fill(x - 3, 21, 7, 1, A2_INK); fill(x, 19, 1, 2, A2_INK);
    const t = (now + x * 0.31) % 18, n = Math.min(6, Math.floor(t * 2));
    for (let k = 0; k < n; k++) dot(x + 1 + k, 22 + (k % 2), A2_INK, 0.7);
    line(x + 1 + n, 22, x + 3 + n, 19, 0x8a8a90); dot(x + 1 + n, 22, A2_INK);
  },
  /** A framed system error: an old dialog box with a double border, a round bomb with a lit fuse and a "restart" button; the fuse sparks and the button flashes every 12 s. */
  apple2bomb(x) {
    fill(x - 11, 5, 23, 19, A2_INK); fill(x - 10, 6, 21, 17, A2_PAPER); fill(x - 9, 7, 19, 15, A2_INK); fill(x - 8, 8, 17, 13, A2_PAPER);
    disc(x - 4, 14, 3, A2_INK); dot(x - 5, 13, A2_PAPER); line(x - 2, 11, x, 9, A2_INK);
    const spark = Math.floor(now * 6) % 2; dot(x + 1, 8, spark ? 0xf5821f : A2_INK); dot(x, 9 - spark, 0xfdb827, 0.8);
    fill(x + 1, 12, 7, 1, A2_INK, 0.7); fill(x + 1, 14, 5, 1, A2_INK, 0.7);
    const t = (now + x * 0.21) % 12, press = t < 0.8;
    fill(x + 1, 17, 7, 3, press ? A2_INK : A2_PAPER); ring(x + 4.5, 18, 0, A2_INK, 0.0); fill(x + 1, 17, 7, 1, A2_INK); fill(x + 1, 19, 7, 1, A2_INK); fill(x + 1, 17, 1, 3, A2_INK); fill(x + 7, 17, 1, 3, A2_INK);
    fill(x + 3, 18, 3, 1, press ? A2_PAPER : A2_INK);
  },
  /** A Jerusalem-stone arched window onto golden hills, the old city's wall with a tower, and a cypress; by night a deep blue sky with stars and lit windows in the wall. Now and then two birds cross the view. */
  jewisharch(x) {
    const stone = 0xe2d0a8, light = 0xf2e6cc, dark = 0xb8a47c, sky = isNight ? 0x1c2a58 : 0x9ccaf0, haze = isNight ? 0x2e3e74 : 0xf2dcae;
    const hill = isNight ? 0x5a4e3a : 0xd8b878, wall = isNight ? 0x7a6a4e : 0xe8d2a0, wallDark = isNight ? 0x5a4c38 : 0xc4a874, cypress = isNight ? 0x1e3a2a : 0x3a6a3e;
    // The stone surround: a round arch on two jambs, with a keystone
    fill(x - 4, 3, 9, 1, stone); fill(x - 6, 4, 13, 1, stone); fill(x - 8, 5, 17, 1, stone); fill(x - 9, 6, 19, 20, stone);
    fill(x - 4, 3, 9, 1, light); dot(x - 6, 4, light); dot(x - 8, 5, light); fill(x - 9, 6, 1, 20, light);
    dot(x + 6, 4, dark); dot(x + 8, 5, dark); fill(x + 9, 6, 1, 20, dark);
    fill(x - 1, 2, 3, 3, light); dot(x + 1, 4, dark);
    // The opening: sky, fading to haze at the horizon
    fill(x - 3, 6, 7, 1, sky); fill(x - 5, 7, 11, 1, sky); fill(x - 6, 8, 13, 17, sky); fill(x - 6, 13, 13, 4, haze, 0.6);
    // Hills, the old city's wall with its battlements and a tower, and a cypress
    fill(x - 6, 17, 13, 8, hill); fill(x - 6, 16, 5, 1, hill); fill(x + 3, 16, 4, 1, hill);
    fill(x - 6, 19, 13, 6, wall); for (let k = 0; k < 13; k += 2) dot(x - 6 + k, 18, wall); fill(x - 6, 24, 13, 1, wallDark);
    fill(x - 1, 15, 3, 4, wall); dot(x - 1, 14, wall); dot(x + 1, 14, wall); fill(x + 1, 15, 1, 4, wallDark);
    fill(x + 4, 13, 2, 6, cypress); dot(x + 4, 12, cypress); dot(x + 5, 14, 0x5a8a5a, isNight ? 0.3 : 0.8);
    if (isNight) {
      for (const [sx, sy, k] of [[-4, 9, 0], [2, 8, 1], [5, 10, 2], [-2, 12, 3]]) dot(x + sx, sy, 0xf4f0ff, 0.5 + 0.4 * Math.sin(now * 1.3 + k * 2));
      glowLate(() => { dot(x - 4, 21, 0xffd88a); dot(x + 2, 22, 0xffd88a); dot(x, 16, 0xffe0a0, 0.9); });
    } else { fill(x - 4, 9, 2, 2, 0xfff4c8); dot(x - 4, 9, WHITE); }
    // Now and then two birds cross the view
    const t = (now + x * 0.41) % 34;
    if (t < 3) {
      const bx = x - 6 + Math.floor(t * 4.4), fl = Math.floor(now * 6) % 2, c = isNight ? 0xc0c4d0 : 0x4a4a5a;
      for (const [ox, oy] of [[0, 0], [-3, 1]]) if (bx + ox >= x - 6 && bx + ox <= x + 5) { dot(bx + ox, 10 + oy - fl, c); dot(bx + ox + 1, 10 + oy, c); }
    }
    // The sill
    fill(x - 10, 25, 21, 2, light); fill(x - 10, 26, 21, 1, dark); fill(x - 9, 27, 19, 1, SHADOW);
  },
  /** A shelf of holy books on carved brackets, high on the wall as is proper: dark spines banded in gold (no lettering) and a blue velvet bag for a prayer shawl beside them; now and then a gleam runs along the gilt. */
  jewishbooks(x) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, gold = 0xd8b45a, goldLight = 0xfff0b0, velvet = 0x2a4a8a, velvetLight = 0x4a6ab0;
    // The shelf board on two carved brackets
    fill(x - 11, 19, 23, 2, wood); fill(x - 11, 19, 23, 1, woodLight); fill(x - 11, 21, 23, 1, woodDark);
    for (const bx of [x - 8, x + 7]) { fill(bx, 22, 2, 2, wood); dot(bx, 24, wood); dot(bx + 1, 24, woodDark); dot(bx, 25, woodDark); }
    fill(x - 11, 22, 23, 1, SHADOW);
    // The books, upright, of different heights, two gold bands on each spine; a gleam runs along them now and then
    const books = [[0x3a2a5a, 9], [0x6a2430, 10], [0x1e3a5a, 8], [0x3a2a5a, 10], [0x5a3a24, 9], [0x6a2430, 8], [0x1e3a5a, 9]];
    const at = Math.floor(((now + x * 0.31) % 22) * 4);
    books.forEach(([c, h], i) => {
      const bx = x - 10 + i * 2, top = 19 - h;
      fill(bx, top, 2, h, c); fill(bx, top, 1, h, shaded(c, 1.25)); dot(bx + 1, top, shaded(c, 0.8));
      fill(bx, top + 2, 2, 1, gold); fill(bx, 16, 2, 1, gold);
      if (at === i) { dot(bx, top + 2, goldLight); dot(bx, 16, goldLight); }
    });
    // The velvet bag, its drawstring gathered at the top, an embroidered band
    fill(x + 5, 13, 6, 6, velvet); fill(x + 6, 12, 4, 1, velvet); fill(x + 5, 13, 1, 6, velvetLight); fill(x + 10, 13, 1, 6, shaded(velvet, 0.75));
    fill(x + 5, 16, 6, 1, gold, 0.85); dot(x + 7, 11, gold); dot(x + 8, 11, gold);
  },
  /** A framed papercut in white on blue: a scalloped border, a tree of life hung with red pomegranates, and two birds facing each other at its foot that bow their heads in turn now and then. */
  jewishpapercut(x) {
    const frame = 0xb08850, frameLight = 0xc8a068, frameDark = 0x7a5a30, blue = 0x2f5f9e, paper = 0xf6f2e8, red = 0xc0283a, redLight = 0xe04a58, leaf = 0x5a8a4a;
    // The olivewood frame and the blue ground
    fill(x - 8, 5, 17, 21, frame); fill(x - 8, 5, 17, 1, frameLight); fill(x - 8, 5, 1, 21, frameLight); fill(x + 8, 5, 1, 21, frameDark); fill(x - 8, 25, 17, 1, frameDark);
    fill(x - 7, 6, 15, 19, blue);
    // The cut paper: a scalloped border, and the tree with its spreading branches
    for (let k = 0; k < 15; k += 2) { dot(x - 7 + k, 6, paper); dot(x - 7 + k, 24, paper); }
    for (let k = 0; k < 19; k += 2) { dot(x - 7, 6 + k, paper); dot(x + 7, 6 + k, paper); }
    fill(x, 11, 1, 11, paper); fill(x - 2, 22, 5, 1, paper);
    line(x, 18, x - 4, 14, paper); line(x, 18, x + 4, 14, paper); line(x, 14, x - 3, 10, paper); line(x, 14, x + 3, 10, paper);
    // Pomegranates on the branches, each with its crown, and leaves
    for (const [px, py] of [[-5, 13], [5, 13], [-3, 9], [3, 9], [0, 8]]) { fill(x + px - 1, py, 2, 2, red); dot(x + px - 1, py, redLight); dot(x + px, py - 1, paper); }
    for (const [lx, ly] of [[-2, 16], [2, 16], [-1, 12], [1, 12]]) dot(x + lx, ly, leaf);
    // The two birds; one bows its head, then the other
    const t = (now + x * 0.37) % 20, bow = t < 1.2 ? 1 : 0, bow2 = t > 1.4 && t < 2.6 ? 1 : 0;
    fill(x - 5, 20, 3, 2, paper); dot(x - 2, 19 + bow, paper); dot(x - 6, 20, paper);
    fill(x + 3, 20, 3, 2, paper); dot(x + 2, 19 + bow2, paper); dot(x + 6, 20, paper);
    fill(x - 7, 26, 15, 1, SHADOW);
  },
  /** A brass hanukkiah on a little stone shelf: eight cups in a row and the shamash raised in the middle. In the eight nights of Hanukkah it holds candles, one more each night, lit and flickering, glowing at night; the rest of the year it stands polished and empty, a gleam running along its arm now and then. */
  jewishhanukkiah(x) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, stone = 0xe2d0a8, stoneLight = 0xf2e6cc, stoneDark = 0xb8a47c;
    // The stone shelf
    fill(x - 11, 25, 23, 2, stone); fill(x - 11, 25, 23, 1, stoneLight); fill(x - 11, 27, 23, 1, stoneDark); fill(x - 9, 28, 19, 1, SHADOW);
    // The foot, the stem with the shamash's cup on top, and the arm with its eight cups
    fill(x - 3, 24, 7, 1, brass); fill(x - 2, 23, 5, 1, brass); dot(x - 3, 24, brassLight); dot(x + 3, 24, brassDark);
    fill(x, 12, 1, 11, brass); dot(x, 20, brassLight); fill(x - 1, 11, 3, 1, brass); dot(x - 1, 11, brassLight);
    fill(x - 9, 18, 19, 1, brass); dot(x - 9, 18, brassLight); dot(x + 9, 18, brassDark);
    const cups = [-9, -7, -5, -3, 3, 5, 7, 9];
    for (const cx of cups) { fill(x + cx, 16, 1, 2, brass); dot(x + cx, 15, brassLight); }
    const night = hanukkahNight();
    if (night) {
      // Blue and white candles, the newest on the left, all lit by the shamash
      const candle = (cx, top, k) => {
        fill(x + cx, top, 1, 3, k % 2 ? 0x8ab4ec : 0xf6f4ee);
        const f = candlelight(k + x * 0.1);
        dot(x + cx, top - 1, 0xffd36b, f); dot(x + cx, top - 2, 0xff8a3d, 0.55 * f);
      };
      const lit = cups.slice(8 - night);
      candle(0, 8, 9); lit.forEach((cx, i) => candle(cx, 12, i));
      glowAt(x, 11, 12, 6, 0xffc060, 0.08);
      if (isNight) glowLate(() => { glowAt(x, 11, 11, 5, 0xffc060, 0.16); lit.forEach((cx) => dot(x + cx, 11, 0xfff4c0)); dot(x, 7, 0xfff4c0); });
    } else {
      const g = (now + x * 0.29) % 18;
      if (g < 2) dot(x - 9 + Math.floor(g * 9.5), 18, WHITE, 0.8);
    }
  },
  /** Two silver Shabbat candlesticks with white candles on a little shelf draped in a white cloth with a blue-stitched hem. On Friday evenings the candles are lit and glow; the rest of the week a gleam slides down the silver now and then. */
  jewishshabbat(x) {
    const silver = 0xc8ccd4, silverLight = 0xeef0f4, silverDark = 0x8a8e98, wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, cloth = 0xf6f4ee, blue = 0x2f5f9e;
    // The shelf, and the cloth hanging over its edge
    fill(x - 8, 24, 17, 2, wood); fill(x - 8, 24, 17, 1, woodLight); fill(x - 8, 26, 17, 1, woodDark); fill(x - 6, 27, 13, 1, SHADOW);
    fill(x - 6, 23, 13, 4, cloth); fill(x - 6, 26, 13, 1, blue); for (let k = 1; k < 13; k += 3) dot(x - 6 + k, 25, blue, 0.6);
    // The candlesticks: a foot, a knopped stem, a cup, a white candle
    const sticks = [[x - 3, 0], [x + 3, 1]];
    for (const [cx] of sticks) {
      fill(cx - 1, 22, 3, 1, silver); dot(cx - 1, 22, silverLight); fill(cx, 17, 1, 5, silver); dot(cx, 19, silverLight);
      fill(cx - 1, 16, 3, 1, silver); dot(cx + 1, 16, silverDark);
      fill(cx, 11, 1, 5, cloth); dot(cx, 11, 0xdedad0);
    }
    if (shabbatEve()) {
      for (const [cx, k] of sticks) { const f = candlelight(k + x * 0.2); dot(cx, 10, 0xffd36b, f); dot(cx, 9, 0xff8a3d, 0.55 * f); }
      glowAt(x, 11, 9, 6, 0xffc060, 0.08);
      if (isNight) glowLate(() => { glowAt(x, 10, 7, 4, 0xffc060, 0.18); for (const [cx] of sticks) dot(cx, 10, 0xfff4c0); });
    } else {
      const g = (now + x * 0.17) % 15;
      if (g < 1) dot(x - 3, 17 + Math.floor(g * 5), WHITE, 0.8);
    }
  },
  /** A ram's-horn shofar mounted on an olivewood plaque, curving up from its narrow mouthpiece to its wide bell, ivory darkening to brown, with a blue tasselled cord; a gleam runs along the horn now and then. */
  jewishshofar(x) {
    const wood = 0xb08850, woodLight = 0xc8a068, woodDark = 0x7a5a30, blue = 0x2f5f9e;
    // The plaque, its corners rounded
    fill(x - 9, 7, 19, 17, wood); fill(x - 10, 8, 21, 15, wood);
    fill(x - 9, 7, 19, 1, woodLight); fill(x - 10, 8, 1, 15, woodLight); fill(x + 10, 8, 1, 15, woodDark); fill(x - 9, 23, 19, 1, woodDark);
    fill(x - 9, 24, 19, 1, SHADOW);
    // The horn: narrow at the mouthpiece, widening and curling up to the bell
    const horn = [[-8, 19, 1], [-7, 19, 1], [-6, 18, 2], [-5, 18, 2], [-4, 18, 2], [-3, 17, 2], [-2, 17, 3], [-1, 17, 3], [0, 16, 3], [1, 16, 3],
      [2, 15, 3], [3, 14, 4], [4, 13, 4], [5, 12, 4], [6, 11, 4], [7, 10, 5]];
    const tones = [0xf0e2c0, 0xe8d4a8, 0xd8b884, 0xc89a64, 0xa87848];
    const at = Math.floor(((now + x * 0.23) % 20) * 8);
    horn.forEach(([hx, hy, h], i) => {
      const c = tones[Math.min(4, Math.floor(i / 3.4))];
      fill(x + hx, hy, 1, h, c); dot(x + hx, hy, shaded(c, 1.1)); dot(x + hx, hy + h - 1, shaded(c, 0.82));
      if (i % 3 === 1) dot(x + hx, hy + 1, shaded(c, 0.9));
      if (at === i) dot(x + hx, hy, WHITE, 0.8);
    });
    fill(x + 8, 9, 1, 6, 0x5a3e28); dot(x + 8, 9, 0x8a6440);
    // The cord tied round it, and its tassel
    fill(x - 1, 17, 1, 3, blue); fill(x - 1, 20, 1, 2, blue); dot(x - 2, 22, blue); dot(x, 22, blue);
  },
  /** A mashrabiya window: a carved wooden lattice of turned spindles in a pointed-arch frame; by day sunlight falls through it in small bright specks on the wall below that shift now and then, by night the sky behind is deep blue with a thin crescent moon. */
  muslimmashrabiya(x) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, sky = isNight ? 0x1c2a58 : 0xf2e2b0;
    // The frame, its top a pointed arch
    fill(x - 1, 3, 3, 1, wood); fill(x - 4, 4, 9, 1, wood); fill(x - 7, 5, 15, 1, wood); fill(x - 8, 6, 17, 19, wood);
    dot(x - 1, 3, woodLight); dot(x - 4, 4, woodLight); dot(x - 7, 5, woodLight); fill(x - 8, 6, 1, 19, woodLight); fill(x + 8, 6, 1, 19, woodDark);
    // Behind the lattice, the sky, and at night the moon
    fill(x - 3, 6, 7, 1, sky); fill(x - 6, 7, 13, 16, sky);
    if (isNight) { disc(x + 3, 10, 2.2, 0xf4eec8); disc(x + 4, 9, 2.2, sky); }
    // The lattice: spindles in a grid, knobs where they cross
    for (let gy = 8; gy < 23; gy += 3) fill(x - 6, gy, 13, 1, woodDark);
    for (let gx = x - 6; gx <= x + 6; gx += 3) fill(gx, 7, 1, 16, woodDark);
    for (let gy = 8; gy < 23; gy += 3) for (let gx = x - 6; gx <= x + 6; gx += 3) dot(gx, gy, woodLight);
    // The sill
    fill(x - 9, 24, 19, 2, woodLight); fill(x - 9, 25, 19, 1, woodDark);
    // The specks of sunlight on the wall below
    if (!isNight) { const s = Math.floor((now + x * 0.2) / 6) % 3; for (let k = 0; k < 5; k++) dot(x - 6 + k * 3 + ((k + s) % 2), 27 + ((k + s) % 2), 0xfff4c8, 0.6); }
    else glowLate(() => dot(x + 2, 10, 0xfff8d8, 0.9));
  },
  /** A pierced brass lantern hanging on a chain: a domed cap, panels of red and blue glass and a little flame inside; it sways a step now and then and glows, through the night too. In Ramadan a small crescent tops its chain and it shines brighter. */
  muslimlantern(x) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, red = 0xe8503a, blue = 0x4c8bf5, ram = inRamadan();
    const t = (now + x * 0.37) % 15, sw = t < 2 ? [0, 1, 0, -1][Math.floor(t * 4) % 4] : 0, cx = x + sw;
    // The chain, and in Ramadan the crescent at its top
    line(x, 1, cx, 7, brassDark);
    if (ram) { dot(x - 1, 1, 0xf2d35b); dot(x - 2, 2, 0xf2d35b); dot(x - 1, 3, 0xf2d35b); }
    // The cap
    fill(cx - 1, 8, 3, 1, brass); fill(cx - 3, 9, 7, 2, brass); dot(cx - 3, 9, brassLight); dot(cx + 3, 10, brassDark);
    // The body: glass panels between brass ribs, the flame behind
    const glow = (0.75 + 0.25 * candlelight(x * 0.2)) * (ram ? 1 : 0.9);
    fill(cx - 3, 11, 7, 7, brassDark);
    fill(cx - 2, 12, 2, 5, red, glow); fill(cx + 1, 12, 2, 5, blue, glow); fill(cx, 12, 1, 5, brass);
    dot(cx - 2, 12, 0xffd0a0, glow); dot(cx + 1, 12, 0xc0e0ff, glow); dot(cx, 14, 0xfff4c0, glow);
    // The bottom and its finial
    fill(cx - 2, 18, 5, 1, brass); fill(cx - 1, 19, 3, 1, brass); dot(cx, 20, brassDark); dot(cx, 21, brassLight);
    glowAt(cx, 14, 7, 6, 0xffc060, ram ? 0.1 : 0.06);
    if (isNight) glowLate(() => { glowAt(cx, 14, 7, 6, 0xffb050, ram ? 0.24 : 0.16); fill(cx - 2, 12, 2, 5, 0xff8a6a, 0.7); fill(cx + 1, 12, 2, 5, 0x8ab0ff, 0.7); dot(cx, 14, 0xfff4c0); });
  },
  /** A framed panel of six glazed tiles in two rows, each an eight-pointed star in blue or turquoise on white with a gold heart and terracotta corners, in a carved wooden frame; a gleam crosses the glaze now and then. */
  muslimtilepanel(x) {
    const wood = 0x6a4228, woodLight = 0x8a5a36, woodDark = 0x40281a, glaze = 0xf4eee0, blue = 0x2f5f9e, teal = 0x3a9a9a, terra = 0xc8643a, gold = 0xf2d35b;
    fill(x - 11, 6, 23, 18, wood); fill(x - 11, 6, 23, 1, woodLight); fill(x - 11, 6, 1, 18, woodLight); fill(x + 11, 6, 1, 18, woodDark); fill(x - 11, 23, 23, 1, woodDark);
    fill(x - 10, 24, 22, 1, SHADOW);
    const g = (now + x * 0.27) % 16;
    for (let r = 0; r < 2; r++) for (let c = 0; c < 3; c++) {
      const tx = x - 10 + c * 7, ty = 7 + r * 8, star = (r + c) % 2 ? blue : teal;
      fill(tx, ty, 7, 7, glaze);
      fill(tx + 2, ty + 2, 3, 3, star);
      for (const [dx, dy] of [[3, 1], [3, 5], [1, 3], [5, 3], [1, 1], [5, 1], [1, 5], [5, 5]]) dot(tx + dx, ty + dy, star);
      dot(tx + 3, ty + 3, gold);
      for (const [dx, dy] of [[0, 0], [6, 0], [0, 6], [6, 6]]) dot(tx + dx, ty + dy, terra);
      if (g < 1.5 && Math.floor(g * 4) === r * 3 + c) dot(tx + 2, ty + 2, WHITE, 0.8);
    }
    fill(x - 10, 14, 21, 1, woodDark);
  },
  /** An arabesque panel: a deep green cartouche framed in gold, filled with gold vine scrolls curling out from a central stem in mirror image, with small leaves and buds; pure ornament with no lettering. A gleam travels along the scrolls now and then. */
  muslimarabesque(x) {
    const green = 0x1f4a3a, gold = 0xd8b45a, goldLight = 0xfff0b0, leaf = 0x6aa060;
    // The cartouche in its gold frame, rounded at the ends
    fill(x - 8, 7, 17, 17, gold); fill(x - 10, 9, 21, 13, gold); fill(x - 9, 8, 19, 15, gold);
    fill(x - 7, 8, 15, 15, green); fill(x - 9, 10, 19, 11, green); fill(x - 8, 9, 17, 13, green);
    fill(x - 8, 24, 17, 1, SHADOW);
    // The scrolls: from a small bud in the middle, on each side a vine curling up into a spiral and a tendril curling down, mirrored
    const scroll = [[1, 15], [2, 14], [3, 13], [4, 12], [5, 12], [6, 13], [6, 14], [5, 15], [4, 14],
      [1, 16], [2, 17], [3, 18], [4, 19], [5, 19], [6, 18], [7, 17], [8, 16], [8, 15]];
    const at = Math.floor(((now + x * 0.21) % 18) * 6);
    scroll.forEach(([dx, dy], i) => { const c = at === i ? goldLight : gold; dot(x + dx, dy, c); dot(x - dx, dy, c); });
    for (const [dx, dy] of [[3, 11], [7, 12], [6, 20]]) { dot(x + dx, dy, leaf); dot(x - dx, dy, leaf); }
    for (const [dx, dy] of [[8, 13], [3, 20]]) { dot(x + dx, dy, goldLight); dot(x - dx, dy, goldLight); }
    fill(x, 15, 1, 2, goldLight); dot(x, 14, gold);
  },
  /** A horseshoe-arch window in a carved plaster frame onto a city of domes and a slender minaret: a clear sky by day, by night a deep blue one with a crescent moon, stars and lit windows. Now and then a flock of birds passes. */
  muslimarch(x) {
    const plaster = 0xf2e6cc, light = 0xfff8e8, dark = 0xd0bfa0, sky = isNight ? 0x1c2a58 : 0xa8d4f0, haze = isNight ? 0x3a3a70 : 0xf6d8a8;
    const city = isNight ? 0x3a3448 : 0xe8c890, cityDark = isNight ? 0x2a2438 : 0xc8a068;
    // The carved frame
    fill(x - 7, 3, 15, 1, plaster); fill(x - 9, 4, 19, 21, plaster); fill(x - 7, 3, 15, 1, light); fill(x - 9, 4, 1, 21, light); fill(x + 9, 4, 1, 21, dark);
    for (let k = 0; k < 19; k += 3) dot(x - 9 + k, 5, dark, 0.6);
    // The horseshoe opening: a round top swelling wider than the jambs
    for (const [ry, hw] of [[6, 2], [7, 4], [8, 5], [9, 6], [10, 6], [11, 6], [12, 5]]) fill(x - hw, ry, hw * 2 + 1, 1, sky);
    fill(x - 5, 13, 11, 11, sky); fill(x - 5, 16, 11, 4, haze, 0.6);
    // The city: two domes, a minaret with its balcony
    fill(x - 5, 20, 11, 4, city); disc(x - 1, 19, 2.5, city); dot(x - 1, 15, cityDark); disc(x + 3, 20, 1.8, city); dot(x + 3, 17, cityDark);
    fill(x - 5, 12, 2, 8, city); dot(x - 5, 10, city); dot(x - 4, 11, city); dot(x - 5, 11, city); fill(x - 5, 14, 2, 1, cityDark); fill(x - 4, 12, 1, 8, cityDark);
    fill(x - 5, 23, 11, 1, cityDark);
    if (isNight) {
      disc(x + 3, 9, 1.8, 0xf4eec8); disc(x + 4, 8, 1.8, sky);
      for (const [sx, sy, k] of [[-3, 8, 0], [1, 7, 1], [5, 13, 2]]) dot(x + sx, sy, 0xf4f0ff, 0.5 + 0.4 * Math.sin(now * 1.3 + k * 2));
      glowLate(() => { dot(x - 3, 22, 0xffd88a); dot(x + 2, 22, 0xffd88a); dot(x - 5, 15, 0xffe0a0, 0.9); dot(x + 2, 8, 0xfff8d8, 0.8); });
    }
    // Now and then a flock of birds
    const t = (now + x * 0.43) % 30;
    if (t < 2.6) {
      const bx = x - 5 + Math.floor(t * 4.2), fl = Math.floor(now * 6) % 2, c = isNight ? 0xc0c4d0 : 0x4a4a5a;
      for (const [ox, oy] of [[0, 0], [-2, 1], [-4, 0]]) if (bx + ox >= x - 5 && bx + ox <= x + 5) dot(bx + ox, 11 + oy - fl, c);
    }
    // The sill
    fill(x - 10, 24, 21, 2, light); fill(x - 10, 25, 21, 1, dark); fill(x - 9, 26, 19, 1, SHADOW);
  },
  /** A carved wooden shelf high on the wall holding the Quran, closed in its green cover with a gold border and no lettering, resting on a folding wooden stand on a white cloth, with a small vase of jasmine beside it; a soft gleam crosses the gold now and then. */
  muslimquranshelf(x) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, cover = 0x1f5a3a, gold = 0xd8b45a, cloth = 0xf6f4ee, pages = 0xf2ead8;
    // The shelf on its brackets, with a scalloped apron
    fill(x - 9, 18, 19, 2, wood); fill(x - 9, 18, 19, 1, woodLight); fill(x - 9, 20, 19, 1, woodDark);
    for (let k = 1; k < 19; k += 3) dot(x - 9 + k, 21, wood);
    for (const bx of [x - 7, x + 6]) { fill(bx, 21, 2, 3, wood); dot(bx + 1, 23, woodDark); }
    fill(x - 9, 22, 19, 1, SHADOW);
    // The cloth, the stand, and the book resting on it
    fill(x - 7, 17, 11, 1, cloth);
    line(x - 5, 16, x - 1, 13, woodDark); line(x + 1, 16, x - 3, 13, wood);
    fill(x - 6, 6, 9, 7, cover); fill(x - 6, 6, 9, 1, gold); fill(x - 6, 12, 9, 1, gold); fill(x - 6, 6, 1, 7, gold); fill(x + 2, 6, 1, 7, gold);
    fill(x - 3, 8, 3, 3, gold, 0.85); dot(x - 2, 9, cover); fill(x + 3, 7, 1, 6, pages);
    const g = (now + x * 0.19) % 16;
    if (g < 1.4) dot(x - 6 + Math.floor(g * 6), 6, 0xfff0b0);
    // The jasmine in its little blue vase
    fill(x + 5, 14, 3, 3, 0x2f5f9e); dot(x + 5, 14, 0x5a8ad0); line(x + 6, 13, x + 6, 10, 0x4a7a3a); dot(x + 7, 11, 0x4a7a3a);
    for (const [fx, fy] of [[5, 10], [7, 9], [6, 9], [8, 11]]) dot(x + fx, fy, WHITE);
  },
  /** A tall pointed stained-glass window in a stone surround: small diamond panes of ruby, sapphire, emerald, amber and violet between lead lines, no figures; by day it throws a few coloured specks on the sill and a gleam crosses it now and then, by night the colours deepen. */
  christianstainedglass(x) {
    const stone = 0xd8d0c0, light = 0xeee8dc, dark = 0xa8a090;
    fill(x - 1, 2, 3, 1, stone); fill(x - 3, 3, 7, 1, stone); fill(x - 5, 4, 11, 1, stone); fill(x - 6, 5, 13, 21, stone);
    dot(x - 1, 2, light); dot(x - 3, 3, light); dot(x - 5, 4, light); fill(x - 6, 5, 1, 21, light); fill(x + 6, 5, 1, 21, dark);
    const rowWidth = (r) => [1, 3, 5, 7][r] ?? 9;
    for (let r = 0; r < 21; r++) { const rw = rowWidth(r); fill(x - (rw >> 1), 4 + r, rw, 1, isNight ? 0x1a2040 : 0xf8f4e8); }
    chapelGlass(x - 4, 4, 9, 21, isNight ? 0.5 : 0.92, rowWidth);
    fill(x - 7, 25, 15, 2, light); fill(x - 7, 26, 15, 1, dark); fill(x - 6, 27, 13, 1, SHADOW);
    if (!isNight) {
      dot(x - 3, 25, 0xc8303a, 0.5); dot(x, 25, 0x2f5fb0, 0.5); dot(x + 3, 25, 0xf2b030, 0.5);
      const g = (now + x * 0.3) % 20;
      if (g < 2) { const k = Math.floor(g * 4); dot(x - 4 + k, 10 + k, WHITE, 0.6); dot(x - 3 + k, 10 + k, WHITE, 0.4); }
    }
  },
  /** A plain oak cross on the wall, with no figure on it, a sprig of olive tucked behind its arm; a soft gleam slides down it now and then. */
  christiancross(x) {
    const oak = 0x8a5a32, light = 0xa8703e, dark = 0x5a3b25;
    fill(x, 6, 3, 21, SHADOW); fill(x - 4, 11, 11, 3, SHADOW);
    fill(x - 1, 5, 3, 21, oak); fill(x - 5, 10, 11, 3, oak);
    fill(x - 1, 5, 1, 21, light); fill(x - 5, 10, 11, 1, light); fill(x + 1, 13, 1, 13, dark); fill(x - 5, 12, 11, 1, dark); fill(x + 1, 5, 1, 5, dark);
    line(x + 2, 13, x + 5, 17, 0x6a7a40); dot(x + 4, 15, 0x7a9a5a); dot(x + 5, 16, 0x8aaa66); dot(x + 3, 16, 0x6a8a4a);
    const g = (now + x * 0.27) % 22;
    if (g < 1.6) dot(x - 1, 6 + Math.floor(g * 12), 0xd8a868, 0.9);
  },
  /** A round rose window in a stone ring: eight petals of coloured glass in two rings round a golden heart, between lead lines; now and then the light runs round its petals one by one; by night the colours deepen. */
  christianrosewindow(x) {
    const stone = 0xd8d0c0, light = 0xeee8dc, dark = 0xa8a090, cy = 15, glass = [0xc8303a, 0x2f5fb0, 0xf2b030, 0x3a9a5a];
    const a = isNight ? 0.5 : 0.95, g = (now + x * 0.2) % 20, lit = g < 2.4 ? Math.floor(g * 10 / 3) % 8 : -1;
    for (let dy = -10; dy <= 10; dy++) for (let dx = -10; dx <= 10; dx++) {
      const r = Math.hypot(dx, dy), px = x + dx, py = cy + dy;
      if (r > 10.2) continue;
      if (r > 8) { dot(px, py, dx + dy < -4 ? light : dx + dy > 4 ? dark : stone); continue; }
      if (isNight) dot(px, py, 0x1a2040);
      if (r < 2) { dot(px, py, 0xf2d35b, a); continue; }
      const ang = ((Math.atan2(dy, dx) + Math.PI) / (Math.PI * 2)) * 8, sec = Math.floor(ang) % 8, frac = ang - Math.floor(ang);
      if (frac < 0.14 || (r > 4.5 && r < 5.4) || r > 7.4) { dot(px, py, 0x2a2a30); continue; }
      const c = glass[(sec + (r > 5 ? 1 : 0)) % 4];
      dot(px, py, sec === lit ? shaded(c, 1.35) : c, a);
    }
    fill(x - 6, 26, 13, 1, SHADOW);
  },
  /** A wooden wall shelf holding the Bible upright on a small easel: black leather with gold page edges and a small gold cross, its red ribbon hanging over the shelf's edge, a brass candlestick beside it; a gleam crosses the cross now and then. */
  christianbibleshelf(x) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, cover = 0x2a2426, coverLight = 0x4a4044, gold = 0xd8b45a, red = 0xa8283a, brass = 0xc8a050;
    fill(x - 9, 20, 19, 2, wood); fill(x - 9, 20, 19, 1, woodLight); fill(x - 9, 22, 19, 1, woodDark);
    for (const bx of [x - 7, x + 6]) { fill(bx, 23, 2, 2, wood); dot(bx + 1, 25, woodDark); }
    fill(x - 9, 23, 19, 1, SHADOW);
    // The easel, the book on it, its ribbon
    line(x - 5, 19, x - 3, 9, woodDark); line(x + 2, 19, x, 9, woodDark);
    fill(x - 6, 9, 9, 9, cover); fill(x - 6, 9, 9, 1, coverLight); fill(x - 6, 9, 1, 9, coverLight); fill(x + 3, 10, 1, 7, gold);
    fill(x - 2, 11, 1, 5, gold); fill(x - 3, 12, 3, 1, gold);
    fill(x + 1, 18, 1, 5, red);
    const g = (now + x * 0.23) % 18;
    if (g < 1) dot(x - 2, 11 + Math.floor(g * 5), 0xfff0b0);
    // The candlestick
    fill(x + 5, 19, 3, 1, brass); fill(x + 6, 17, 1, 2, brass); fill(x + 6, 12, 1, 5, 0xf6efdc); dot(x + 6, 11, 0x3a3238);
  },
  /** A wrought-iron wall sconce: a backplate with two curling arms, each holding a white candle that flickers, glowing warm and through the night. */
  christiansconce(x) {
    const iron = 0x2a2a30, ironLight = 0x4a4a54, wax = 0xf6efdc;
    fill(x - 1, 8, 3, 14, iron); fill(x - 1, 8, 1, 14, ironLight); dot(x, 7, ironLight); dot(x, 22, iron);
    line(x, 18, x - 4, 16, iron); line(x, 18, x + 4, 16, iron); dot(x - 5, 17, iron); dot(x + 5, 17, iron);
    const cups = [x - 4, x + 4];
    cups.forEach((cx, k) => {
      fill(cx - 1, 15, 3, 1, iron); fill(cx, 10, 1, 5, wax); dot(cx, 10, 0xe8e0c8);
      const f = candlelight(x * 0.1 + k); dot(cx, 9, 0xffd36b, f); dot(cx, 8, 0xff8a3d, 0.55 * f);
    });
    glowAt(x, 10, 9, 6, 0xffc060, 0.07);
    if (isNight) glowLate(() => { glowAt(x, 10, 9, 6, 0xffc060, 0.18); for (const cx of cups) dot(cx, 9, 0xfff4c0); });
  },
  /** A round wreath of olive leaves with small olives and a cream ribbon bow; from December to Epiphany it turns to evergreen with red berries and a red bow. Its ribbon tails flutter now and then. */
  christianwreath(x) {
    const xmas = christmasTime(), leaf = xmas ? 0x2f6a3a : 0x6a8a4a, leafLight = xmas ? 0x4a8a50 : 0x8aaa66, leafDark = xmas ? 0x1f4a28 : 0x4a6a34;
    const ribbon = xmas ? 0xc0283a : 0xf2e6cc, ribbonDark = xmas ? 0x8a1a28 : 0xd8c8a8, cy = 14;
    dot(x, 4, 0x4a4a54); line(x, 4, x - 2, 7, ribbonDark); line(x, 4, x + 2, 7, ribbonDark);
    for (let k = 0; k < 24; k++) {
      const a = (k / 24) * Math.PI * 2, r = 6 + (k % 2) * 0.6;
      dot(Math.round(x + Math.cos(a) * r), Math.round(cy + Math.sin(a) * r), k % 3 === 0 ? leafLight : leaf);
      dot(Math.round(x + Math.cos(a) * (r - 1.4)), Math.round(cy + Math.sin(a) * (r - 1.4)), leafDark);
      if (k % 3 === 1) dot(Math.round(x + Math.cos(a) * (r + 1.2)), Math.round(cy + Math.sin(a) * (r + 1.2)), leafLight);
    }
    for (const [bx, by] of [[-4, 9], [5, 11], [-6, 16], [3, 19], [6, 16]]) dot(x + bx, by, xmas ? 0xe8303a : 0x4a5a2a);
    const t = (now + x * 0.29) % 18, fl = t < 2 ? Math.floor(now * 6) % 2 : 0;
    fill(x - 2, 20, 2, 2, ribbon); fill(x + 1, 20, 2, 2, ribbon); dot(x, 20, ribbonDark); dot(x, 21, ribbon);
    line(x - 1, 22, x - 2 - fl, 25, ribbon); line(x + 1, 22, x + 2 + fl, 25, ribbon);
  },
  /** A string of prayer flags in the five colours, blue, white, red, green and yellow, sagging between two pins with a small printed mark on each (no lettering); now and then a breeze sets them fluttering. */
  buddhistflags(x) {
    const cols = [0x2f6ad0, 0xf4f2ea, 0xd8302a, 0x3a9a4a, 0xf2c830];
    dot(x - 11, 4, 0x5a3b25); dot(x + 11, 4, 0x5a3b25);
    const t = (now + x * 0.3) % 10, fl = t < 2 ? Math.floor(now * 6) % 2 : 0;
    for (let k = 0; k < 7; k++) {
      const fx = x - 10 + k * 3, fy = 4 + Math.round(Math.sin((k / 6) * Math.PI) * 4), c = cols[k % 5], flap = fl && (k % 2) ? 1 : 0;
      dot(fx, fy, 0x8a7a5a); dot(fx + 1, fy, 0x8a7a5a); dot(fx + 2, fy, 0x8a7a5a);
      fill(fx, fy + 1, 3, 4 - flap, c); dot(fx, fy + 1, shaded(c, 1.1)); dot(fx + 1, fy + 2, shaded(c, 0.75)); dot(fx + 2, fy + 4 - flap, shaded(c, 0.85));
    }
    dot(x + 11, 5, 0x8a7a5a);
  },
  /** A small golden seated Buddha high on a red lacquer wall shelf, resting on a pink lotus pedestal, a soft halo of light behind, with a candle on each side; the candles flicker and glow through the night. */
  buddhistaltarshelf(x) {
    const gold = 0xd8b45a, goldLight = 0xf2d888, goldDark = 0x9a7a30, lac = 0xa8282a, lacLight = 0xc8403a, lacDark = 0x7a1a1c, pink = 0xf2a0b8, pinkDark = 0xd8708e;
    // The shelf on its brackets
    fill(x - 9, 21, 19, 2, lac); fill(x - 9, 21, 19, 1, lacLight); fill(x - 9, 23, 19, 1, gold);
    for (const bx of [x - 7, x + 6]) { fill(bx, 24, 2, 2, lacDark); dot(bx + 1, 26, lacDark); }
    fill(x - 9, 24, 19, 1, SHADOW);
    // The halo, then the figure: a quiet golden form, hands resting in its lap
    glowAt(x, 11, 5, 5, 0xffe8a0, isNight ? 0.14 : 0.1);
    dot(x, 7, gold); fill(x - 1, 8, 3, 3, gold); dot(x - 1, 8, goldLight); dot(x - 2, 9, goldDark); dot(x + 2, 9, goldDark);
    fill(x - 3, 11, 7, 1, gold); fill(x - 3, 12, 7, 4, gold); fill(x - 3, 12, 1, 4, goldLight); fill(x + 3, 12, 1, 4, goldDark);
    line(x - 2, 11, x + 2, 15, goldDark, 0.5); fill(x - 1, 15, 3, 1, goldLight);
    fill(x - 4, 16, 9, 2, gold); fill(x - 4, 16, 1, 2, goldLight); fill(x + 4, 16, 1, 2, goldDark);
    // The lotus pedestal
    fill(x - 5, 18, 11, 3, pink); for (let k = -5; k <= 5; k += 2) dot(x + k, 18, 0xf8c8d4); fill(x - 5, 20, 11, 1, pinkDark);
    // The candles
    const cand = [x - 7, x + 7];
    cand.forEach((cx, k) => {
      fill(cx, 17, 1, 4, 0xf6efdc);
      const f = candlelight(k + x * 0.1); dot(cx, 16, 0xffd36b, f); dot(cx, 15, 0xff8a3d, 0.5 * f);
    });
    if (isNight) glowLate(() => { for (const cx of cand) { glowAt(cx, 16, 3, 3, 0xffc060, 0.2); dot(cx, 16, 0xfff4c0); } });
  },
  /** A hanging scroll on a blue-grey silk mount, painted with a single pink lotus rising on its stem over a round leaf, ink-washed water lines below; its bottom rod sways now and then. */
  buddhistlotusscroll(x) {
    const silk = 0x6a7a88, silkLight = 0x8a9aa8, paper = 0xf2ead8, wood = 0x5a3b25, pink = 0xf2a0b8, pinkDark = 0xd8708e, leaf = 0x4a7a4a, ink = 0x8a9090;
    const t = (now + x * 0.29) % 20, sw = t < 2 ? [0, 1, 0, -1][Math.floor(t * 4) % 4] : 0;
    dot(x, 2, wood); line(x, 2, x - 4, 4, 0x8a7a5a); line(x, 2, x + 4, 4, 0x8a7a5a);
    fill(x - 6, 4, 13, 1, wood); fill(x - 5, 5, 11, 20, silk); fill(x - 5, 5, 1, 20, silkLight);
    fill(x - 4, 7, 9, 15, paper);
    // The painting
    line(x, 11, x, 19, leaf); fill(x - 3, 17, 4, 2, leaf); dot(x - 4, 18, leaf); dot(x - 2, 16, 0x6a9a5a);
    fill(x - 1, 10, 3, 2, pink); dot(x, 9, pink); dot(x - 2, 10, pinkDark); dot(x + 2, 10, pinkDark); dot(x, 11, 0xf8d0dc);
    for (const [lx, ly, w] of [[-3, 20, 3], [1, 21, 3], [-2, 21, 1]]) fill(x + lx, ly, w, 1, ink, 0.6);
    fill(x - 6 + sw, 25, 13, 1, wood); dot(x - 7 + sw, 25, wood); dot(x + 7 + sw, 25, wood);
    fill(x - 5, 26, 11, 1, SHADOW);
  },
  /** A round moon window in a plaster wall, a few stalks of bamboo outside it: a soft sky by day, by night a deep blue one with a full moon; the bamboo leaves stir now and then. */
  buddhistmoonwindow(x) {
    const plaster = 0xf2e6cc, light = 0xfff8e8, dark = 0xd0bfa0, sky = isNight ? 0x1c2a58 : 0xcfe4f0, cy = 14;
    const stalk = isNight ? 0x2a4a3a : 0x6a9a4a, stalkDark = isNight ? 0x1e3a2a : 0x4a7a3a, leaf = isNight ? 0x2a5a3a : 0x5a9a4a;
    // The round opening in its plaster ring
    for (let dy = -10; dy <= 10; dy++) for (let dx = -10; dx <= 10; dx++) {
      const r = Math.hypot(dx, dy);
      if (r <= 10.2) dot(x + dx, cy + dy, r > 8 ? (dx + dy < -4 ? light : dx + dy > 4 ? dark : plaster) : sky);
    }
    if (isNight) { disc(x + 3, cy - 4, 2.2, 0xf4eec8); glowLate(() => { glowAt(x + 3, cy - 4, 4, 4, 0xf4eec8, 0.15); dot(x + 3, cy - 4, 0xfffbe8); }); }
    // The bamboo: three stalks with their nodes, and leaves that stir now and then
    const t = (now + x * 0.31) % 12, sway = t < 2 ? Math.floor(t * 3) % 2 : 0;
    for (const [bx, top] of [[-5, -7], [-2, -8], [4, -6]]) {
      for (let yy = cy + top; yy <= cy + 7; yy++) if (Math.hypot(bx, yy - cy) <= 8) dot(x + bx, yy, stalk);
      for (const ny of [cy - 3, cy + 2]) if (Math.hypot(bx, ny - cy) <= 8) dot(x + bx, ny, stalkDark);
    }
    for (const [lx, ly, dir] of [[-5, -2, 1], [-2, -6, 1], [-2, 0, -1], [4, -3, -1], [4, 3, 1]]) {
      const px = x + lx, py = cy + ly;
      dot(px + dir, py, leaf); dot(px + dir * 2 + (dir > 0 ? sway : -sway), py - 1, leaf); dot(px + dir * 3, py - 1, leaf, 0.8);
    }
    fill(x - 6, 25, 13, 1, SHADOW);
  },
  /** A golden Dharma wheel with eight spokes on a round maroon plaque; every 20 s it turns a step, and a gleam runs round its rim now and then. */
  buddhistwheel(x) {
    const gold = 0xd8b45a, goldLight = 0xf2d888, goldDark = 0x9a7a30, maroon = 0x6a2028, maroonDark = 0x4a1418, cy = 14;
    disc(x, cy, 8.4, goldDark); disc(x, cy, 7.6, maroon); disc(x + 1, cy + 1, 7.6, maroonDark, 0.3);
    ring(x, cy, 5.6, gold); ring(x, cy, 4.8, gold);
    const turn = (Math.floor((now + x * 0.3) / 20) % 2) * (Math.PI / 8);
    for (let k = 0; k < 8; k++) {
      const a = (k / 8) * Math.PI * 2 + turn;
      line(x, cy, x + Math.round(Math.cos(a) * 5), cy + Math.round(Math.sin(a) * 5), gold);
      dot(x + Math.round(Math.cos(a) * 6.6), cy + Math.round(Math.sin(a) * 6.6), goldLight);
    }
    disc(x, cy, 1.6, gold); dot(x, cy, goldLight);
    const g = (now + x * 0.17) % 15;
    if (g < 1.6) { const a = (g / 1.6) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 5.2), cy + Math.round(Math.sin(a) * 5.2), WHITE, 0.9); }
    fill(x - 6, 24, 13, 1, SHADOW);
  },
  /** Three round paper lanterns, red, saffron and pink, hanging from a cord with tassels below; they sway a little now and then and glow softly, through the night too. In May, for Vesak, they shine brighter and a little lotus-shaped one joins them. */
  buddhistlanterns(x) {
    const vesak = vesakTime(), cord = 0x5a3b25, cols = [0xd8302a, 0xe8902a, 0xf2a0b8];
    line(x - 11, 3, x + 11, 3, cord);
    const t = (now + x * 0.21) % 14, sw = t < 2 ? [0, 1, 0, -1][Math.floor(t * 4) % 4] : 0;
    const spots = [[x - 7, 9], [x, 11], [x + 7, 9]];
    const lanterns = (a) => spots.forEach(([lx, ly], k) => {
      const cx = lx + (k === 1 ? sw : -sw), c = cols[k];
      line(lx, 3, cx, ly - 4, cord);
      fill(cx - 1, ly - 4, 3, 1, 0x2a2a30); fill(cx - 2, ly - 3, 5, 5, c, a); fill(cx - 3, ly - 2, 7, 3, c, a);
      for (const ry of [ly - 2, ly, ly + 1]) fill(cx - 3, ry, 7, 1, shaded(c, 0.85), a);
      dot(cx - 2, ly - 2, shaded(c, 1.25), a); fill(cx - 1, ly + 2, 3, 1, 0x2a2a30); fill(cx, ly + 3, 1, 3, 0xd8b45a);
    });
    lanterns(0.95);
    if (vesak) { fill(x - 1, 18, 3, 2, 0xf8c8d4); dot(x, 17, 0xf2a0b8); dot(x - 2, 18, 0xf2a0b8); dot(x + 2, 18, 0xf2a0b8); }
    const glow = vesak ? 0.12 : 0.06;
    for (const [lx, ly] of spots) glowAt(lx, ly, 5, 4, 0xffc070, glow);
    if (isNight) glowLate(() => { for (const [lx, ly] of spots) glowAt(lx, ly, 5, 4, 0xffb060, vesak ? 0.26 : 0.16); lanterns(0.55); });
  },
  /** A riveted steel window onto the volcano's caldera: dark rock walls falling to a lake of glowing lava whose crust drifts and bubbles burst now and then, smoke above; it lights the wall, and shines through the night. */
  volcanocaldera(x) {
    const steel = 0x5a5e68, steelLight = 0x7e828c, steelDark = 0x3a3c44, rock = 0x2a2020, rockLight = 0x4a3a34;
    const sky = isNight ? 0x2a1418 : 0x7a5a52, lava = 0xe8501e, lavaHot = 0xffa040, crust = 0x8a2a14;
    fill(x - 12, 4, 25, 22, steelDark); fill(x - 11, 5, 23, 20, steel); fill(x - 11, 5, 23, 1, steelLight); fill(x - 11, 5, 1, 20, steelLight);
    fill(x - 10, 6, 21, 18, sky);
    // Smoke in the sky, the far cone and the rock walls
    for (const [sx, sy] of [[-6, 7], [-3, 8], [2, 7], [5, 9]]) fill(x + sx, sy, 3, 1, isNight ? 0x3a2228 : 0x8a6e66);
    for (let r = 0; r < 6; r++) fill(x - 3 - r * 2, 12 + r, 7 + r * 4, 1, r < 2 ? rockLight : rock);
    for (let r = 0; r < 9; r++) { fill(x - 10, 10 + r, 3 - (r % 2), 1, rock); fill(x + 8 + (r % 2), 10 + r, 3, 1, rock); }
    // The lava lake: its crust drifts slowly, and a bubble rises and bursts now and then
    const lake = () => {
      fill(x - 9, 18, 19, 6, lava); fill(x - 9, 18, 19, 1, lavaHot);
      const drift = Math.floor(now * 0.5) % 19;
      for (let k = 0; k < 3; k++) fill(x - 9 + ((drift + k * 7) % 17), 20 + (k % 2) * 2, 3, 1, crust);
      const b = (now + x * 0.3) % 5;
      if (b < 1.2) { const bx = x - 4 + (Math.floor((now + x) / 5) % 3) * 4, r = Math.floor(b * 3); if (r < 2) dot(bx, 18 - r, 0xffd060); else { dot(bx - 1, 17, 0xffd060, 0.6); dot(bx + 1, 17, 0xffd060, 0.6); } }
    };
    lake();
    glowAt(x, 20, 10, 4, 0xff6020, 0.08);
    for (const [rx, ry] of [[-11, 5], [11, 5], [-11, 24], [11, 24]]) dot(x + rx, ry, 0xa8adb6);
    dot(x - 8, 7, WHITE, 0.35);
    fill(x - 11, 26, 23, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x, 21, 10, 4, 0xff6020, 0.14); lake(); });
  },
  /** A world domination map on a steel board: green blobs of land on a dark gridded sea, four targets marked on it, and a red ring that hops from one to the next, a dotted line following; the targets glow at night. */
  volcanomap(x) {
    const frame = 0x2a2c34, frameLight = 0x4a4e58, sea = 0x14283e, gridC = 0x1e3a54, land = 0x3a6a4a, landLight = 0x4a8a5a, red = 0xe8302a;
    fill(x - 12, 5, 25, 19, frame); fill(x - 12, 5, 25, 1, frameLight); fill(x - 11, 6, 23, 17, sea);
    for (let gx = x - 9; gx < x + 12; gx += 5) fill(gx, 6, 1, 17, gridC);
    for (let gy = 9; gy < 23; gy += 5) fill(x - 11, gy, 23, 1, gridC);
    for (const [lx, ly, w, h] of [[-10, 8, 5, 4], [-9, 12, 3, 6], [-3, 7, 4, 3], [-2, 10, 4, 7], [3, 7, 7, 5], [5, 12, 3, 3], [7, 17, 3, 3]]) {
      fill(x + lx, ly, w, h, land); fill(x + lx, ly, w, 1, landLight);
    }
    const targets = [[-8, 9], [0, 13], [5, 9], [8, 18]], at = Math.floor((now + x * 0.2) / 1.6) % 4;
    const marks = () => targets.forEach(([tx, ty], k) => {
      dot(x + tx, ty, red, k === at ? 1 : 0.55);
      if (k === at) ring(x + tx, ty, 2, red, 0.9);
    });
    const [ax, ay] = targets[at], [bx, by] = targets[(at + 1) % 4];
    for (let k = 1; k < 6; k += 2) dot(Math.round(x + ax + (bx - ax) * k / 6), Math.round(ay + (by - ay) * k / 6), 0xf2c830, 0.8);
    marks();
    fill(x - 11, 24, 23, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x + ax, ay, 3, 3, red, 0.25); marks(); });
  },
  /** A red alarm beacon in a wire cage on a steel bracket over a yellow hazard sign; every 30 s or so it goes off for a few seconds, its light spinning and throwing red sweeps across the wall, through the night too. */
  volcanobeacon(x) {
    const steel = 0x5a5e68, steelDark = 0x3a3c44, cage = 0xa8adb6, yellow = 0xf2c830, black = 0x1e1e22;
    const alarm = (now + x * 0.3) % 30 < 4, col = Math.floor(now * 8) % 4;
    fill(x - 1, 4, 3, 3, steelDark); fill(x - 4, 14, 9, 2, steel); fill(x - 4, 14, 9, 1, 0x7e828c);
    fill(x - 3, 8, 7, 6, alarm ? 0xd8281e : 0x8a1a14); fill(x - 2, 7, 5, 1, alarm ? 0xd8281e : 0x8a1a14);
    if (alarm) fill(x - 3 + (col < 3 ? col * 2 + 1 : 3), 8, 1, 5, 0xffd0c0); else dot(x - 2, 9, 0xd85a4a);
    for (const cx of [-3, 0, 3]) fill(x + cx, 8, 1, 6, cage, 0.6);
    fill(x - 3, 10, 7, 1, cage, 0.5);
    // The hazard sign: a yellow triangle with a mark
    for (let r = 0; r < 7; r++) fill(x - Math.floor(r * 0.8), 18 + r, 1 + Math.floor(r * 0.8) * 2, 1, yellow);
    fill(x - 6, 25, 13, 1, black); fill(x, 20, 1, 3, black); dot(x, 24, black);
    fill(x - 5, 26, 11, 1, SHADOW);
    const sweep = () => { glowAt(x + [-9, 0, 9, 0][col], 11, 9, 6, 0xff2a1a, 0.2); dot(x - 3 + (col < 3 ? col * 2 + 1 : 3), 9, 0xffe0d0); };
    if (alarm) { if (isNight) glowLate(sweep); else sweep(); }
  },
  /** A run of steel pipes along the wall: two across, one down to a red valve wheel, a pressure gauge whose needle trembles, and every 9 s a puff of steam from a leaky joint. */
  volcanopipes(x) {
    const steel = 0x6e727c, steelLight = 0xa8adb6, steelDark = 0x4a4e58, copper = 0xb87333, red = 0xc8281e;
    fill(x - 14, 8, 28, 3, steel); fill(x - 14, 8, 28, 1, steelLight); fill(x - 14, 10, 28, 1, steelDark);
    fill(x - 14, 14, 28, 2, copper); fill(x - 14, 14, 28, 1, 0xd8935a);
    fill(x + 6, 11, 3, 15, steel); fill(x + 6, 11, 1, 15, steelLight); fill(x + 8, 11, 1, 15, steelDark);
    for (const jx of [-10, 0, 10]) fill(x + jx, 7, 2, 5, steelDark);
    // The valve wheel
    ring(x + 7, 21, 3, red); dot(x + 7, 21, steelDark); dot(x + 5, 20, 0xe85a4a);
    // The gauge, its needle trembling
    disc(x - 6, 21, 3.6, steelDark); disc(x - 6, 21, 2.8, 0xf4eedc); fill(x - 6, 15, 1, 3, steelDark);
    const a = -Math.PI * 0.75 + Math.PI * 0.9 + Math.sin(now * 9 + x) * 0.15;
    line(x - 6, 21, x - 6 + Math.round(Math.cos(a) * 2.4), 21 + Math.round(Math.sin(a) * 2.4), red); dot(x - 4, 19, red, 0.5);
    // The leaky joint's puff
    const t = (now + x * 0.37) % 9;
    if (t < 2) { const r = Math.floor(t * 3); fill(x - 1 + r, 6 - r, 2 + r, 1, WHITE, 0.6 - r * 0.1); dot(x + r, 5 - r, WHITE, 0.4); }
    fill(x - 13, 26, 26, 1, SHADOW);
  },
  /** A round brass-rimmed porthole into a tank of piranhas: three little silver fish with red bellies patrol the murky green water, turning at the glass, as bubbles rise; the tank is lit at night. */
  volcanoporthole(x) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, water = isNight ? 0x1a3a34 : 0x2a5a4a, cy = 14;
    disc(x, cy, 10, brassDark); disc(x, cy, 9.2, brass); ring(x, cy, 9, brassLight, 0.5); disc(x, cy, 7.6, water);
    for (let k = 0; k < 8; k++) { const a = (k / 8) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 8.6), cy + Math.round(Math.sin(a) * 8.6), brassDark); }
    const fish = () => [[-3, 0.7, 0], [2, 0.5, 2], [5, 0.9, 4]].forEach(([fy, speed, off]) => {
      const ph = ((now * speed + off + x * 0.1) % 4) / 2, dir = ph < 1 ? 1 : -1, fx = Math.round(x - 5 + (ph < 1 ? ph : 2 - ph) * 10);
      if (Math.hypot(fx - x, fy) > 6.5) return;
      fill(fx - 1, cy + fy, 3, 2, 0xa8b0b4); dot(fx - 1, cy + fy + 1, 0xd8402a); dot(fx + 1, cy + fy + 1, 0xd8402a);
      dot(fx - 2 * dir, cy + fy, 0x7a8488); dot(fx + dir, cy + fy, PALETTE.eye);
    });
    fish();
    const b = (now + x * 0.21) % 4;
    dot(x - 2, cy + 5 - Math.floor(b * 3), 0xc8e8e0, 0.6); if (b < 2) dot(x + 3, cy + 3 - Math.floor(b * 3), 0xc8e8e0, 0.5);
    fill(x - 5, cy - 5, 3, 1, WHITE, 0.35); dot(x - 6, cy - 4, WHITE, 0.3);
    fill(x - 6, 25, 13, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x, cy, 7, 7, 0x6ad8b0, 0.14); fish(); });
  },
  /** A doomsday countdown panel: red digits counting down minutes and seconds from 9:59 behind dark glass, framed in yellow and black; in its last ten seconds the whole panel flashes, then it starts again. The digits shine through the night. */
  volcanocountdown(x) {
    const yellow = 0xf2c830, black = 0x1e1e22, red = 0xff3a28, off = 0x3a1210;
    const DIGITS = ['111101101101111', '010110010010111', '111001111100111', '111001111001111', '101101111001001', '111100111001111', '111100111101111', '111001001001001', '111101111101111', '111101111001111'];
    fill(x - 12, 7, 25, 15, yellow);
    for (let k = 0; k < 25; k += 3) { dot(x - 12 + k, 7, black); dot(x - 12 + k, 21, black); }
    fill(x - 10, 9, 21, 11, 0x14100e);
    const left = 599 - Math.floor(now + x * 0.7) % 600, m = Math.floor(left / 60), s = left % 60, flash = left < 10 && Math.floor(now * 4) % 2;
    const digits = (a) => {
      [m, -1, Math.floor(s / 10), s % 10].forEach((d, k) => {
        const dx = x - 9 + [0, 4, 6, 10][k] + (k > 1 ? 1 : 0);
        if (d < 0) { dot(dx, 12, red, a); dot(dx, 16, red, a); return; }
        for (let i = 0; i < 15; i++) dot(dx + (i % 3), 12 + Math.floor(i / 3), DIGITS[d][i] === '1' ? red : off, DIGITS[d][i] === '1' ? a : 1);
      });
      if (flash) fill(x - 10, 9, 21, 11, red, 0.25);
    };
    digits(1);
    fill(x - 11, 22, 23, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x, 14, 9, 5, red, 0.12); digits(0.9); });
  },
  /** A ticket rail: a steel bar with order tickets clipped along it, each scribbled in pencil, one with a red allergy line; every 15 s a new ticket slides in from the left and the others shuffle along. */
  kitchenrail(x) {
    const steel = 0xc8ccd2, steelLight = 0xe4e8ec, steelDark = 0x8a9098, paper = 0xf8f6ee, pencil = 0x8a8a90;
    fill(x - 13, 7, 27, 3, steel); fill(x - 13, 7, 27, 1, steelLight); fill(x - 13, 9, 27, 1, steelDark);
    dot(x - 14, 8, steelDark); dot(x + 14, 8, steelDark);
    const t = (now + x * 0.3) % 15, slide = t < 1.5 ? Math.round((1 - t / 1.5) * 5) : 0, n = Math.floor((now + x * 0.3) / 15);
    for (let k = 0; k < 5; k++) {
      const tx = x - 12 + k * 5 + (k === 0 ? -slide : 0), h = hash(n - k, 7), len = 9 + (h % 4);
      if (tx < x - 13) continue;
      fill(tx, 10, 4, len, paper); fill(tx + 3, 10, 1, len, 0xd8d6ce); fill(tx, 10, 4, 1, 0xa8adb6);
      for (let ly = 12; ly < 10 + len - 1; ly += 2) fill(tx, ly, 2 + ((h >>> ly) % 2), 1, pencil, 0.8);
      if (h % 3 === 0) fill(tx, 14, 3, 1, 0xd8402a);
    }
    fill(x - 13, 23, 27, 1, SHADOW);
  },
  /** Copper pans hanging by their handles from S-hooks on a black iron rail, from a big rondeau to a little butter pan, their tin-lined insides dark; a gleam runs across them every 12 s and one swings a little. */
  kitchenpans(x) {
    const iron = 0x2a2a30, copper = 0xc87a3a, copperLight = 0xe8a060, copperDark = 0x8a4a22;
    fill(x - 13, 4, 27, 2, iron); fill(x - 13, 4, 27, 1, 0x4a4a52);
    const t = (now + x * 0.17) % 12, sw = t < 2 ? [0, 1, 0, -1][Math.floor(t * 4) % 4] : 0;
    [[-9, 5, 0], [-2, 4, 1], [5, 3, 0], [10, 2, 0]].forEach(([px, r, swings], k) => {
      const cx = x + px + (swings ? sw : 0), cy = 13 + r;
      dot(x + px, 6, 0x6a6a70); fill(cx, 7, 1, cy - r - 7, copperDark);   // hook and handle
      disc(cx, cy, r, copperDark); disc(cx, cy, r - 0.8, copper); if (r > 3) ring(cx, cy, r - 2, copperDark, 0.6);
      dot(cx - Math.ceil(r / 2), cy - Math.ceil(r / 2), copperLight);
      if (t < 1.2 && Math.floor(t * 4) === k) dot(cx - 1, cy - r + 1, WHITE, 0.9);
    });
    fill(x - 12, 24, 25, 1, SHADOW);
  },
  /** A magnetic knife strip on the tiles holding a chef's knife, a slicer, a cleaver, a paring knife and a honing steel, handles down; a glint runs down one blade now and then. */
  kitchenknives(x) {
    const strip = 0x2a2a30, blade = 0xc8ccd2, bladeLight = 0xeef2f6, bladeDark = 0x8a9098, handle = 0x2a2220, rivet = 0xa8adb6;
    fill(x - 10, 6, 21, 3, strip); fill(x - 10, 6, 21, 1, 0x4a4a52);
    const knives = [[-8, 11, 2], [-4, 13, 1], [1, 7, 5], [7, 6, 1]];
    knives.forEach(([kx, len, w]) => {
      fill(x + kx, 5, w, len, blade); fill(x + kx, 5, 1, len, bladeLight); if (w > 1) fill(x + kx + w - 1, 5, 1, len, bladeDark);
      fill(x + kx, 5 + len, Math.max(1, Math.min(w, 2)), 5, handle); dot(x + kx, 7 + len, rivet);
    });
    dot(x + 1, 6, bladeDark); fill(x + 1, 5, 5, 1, bladeLight);   // the cleaver's spine and hole
    fill(x + 9, 4, 1, 13, 0x6e727c); fill(x + 9, 17, 1, 4, handle); dot(x + 9, 21, rivet);   // the honing steel
    const t = (now + x * 0.23) % 14;
    if (t < 1.2) dot(x - 8, 5 + Math.floor(t * 9), WHITE, 0.9);
    fill(x - 9, 23, 19, 1, SHADOW);
  },
  /** Three heat lamps hanging over the pass on their cords, steel shades over red-orange bulbs throwing warm light down; through the night too. */
  kitchenheatlamps(x) {
    const steel = 0xa8adb6, steelDark = 0x6e727c, bulb = 0xff6a2a;
    const spots = [x - 8, x, x + 8], lens = [8, 12, 8];
    spots.forEach((lx, k) => {
      fill(lx, 1, 1, lens[k], 0x2a2a30);
      const y = 1 + lens[k];
      fill(lx - 2, y, 5, 2, steel); fill(lx - 3, y + 2, 7, 2, steel); fill(lx - 3, y + 2, 7, 1, 0xd8dce2); fill(lx + 2, y + 2, 2, 2, steelDark);
      fill(lx - 2, y + 4, 5, 1, bulb); dot(lx, y + 4, 0xffc080);
    });
    const glow = () => spots.forEach((lx, k) => { glowAt(lx, 7 + lens[k], 5, 6, 0xff7a3a, 0.14); fill(lx - 2, 5 + lens[k], 5, 1, 0xff8a3a); });
    glowAt(x, 22, 13, 5, 0xff8a3a, 0.07);
    if (isNight) glowLate(glow); else spots.forEach((lx, k) => glowAt(lx, 7 + lens[k], 4, 5, 0xff7a3a, 0.08));
  },
  /** A stainless extractor hood, its canopy flaring to a lip, grease filters in rows and a little lamp under the lip; wisps of steam rise up into it from below every 8 s. */
  kitchenhood(x) {
    const steel = 0xc8ccd2, steelLight = 0xe4e8ec, steelDark = 0x8a9098;
    fill(x - 4, 0, 9, 6, steel); fill(x - 4, 0, 1, 6, steelLight); fill(x + 4, 0, 1, 6, steelDark);
    for (let r = 0; r < 8; r++) fill(x - 5 - r, 6 + r, 11 + r * 2, 1, r === 0 ? steelLight : steel);
    fill(x - 12, 14, 25, 2, steelDark); fill(x - 12, 14, 25, 1, steelLight);
    for (let fx = x - 9; fx < x + 10; fx += 4) for (let fy = 8; fy < 13; fy += 2) if (Math.abs(fx - x) < 4 + (fy - 6)) fill(fx, fy, 3, 1, steelDark, 0.6);
    dot(x - 8, 16, 0xffe08a); dot(x + 8, 16, 0xffe08a);
    const t = (now + x * 0.29) % 8;
    if (t < 3) for (let k = 0; k < 3; k++) { const ph = (t / 3 + k / 3) % 1; fill(x - 4 + k * 4 + Math.round(Math.sin(ph * 6) * 1.5), 26 - Math.round(ph * 9), 2, 1, WHITE, 0.45 * (1 - ph)); }
    if (isNight) glowLate(() => { glowAt(x, 18, 9, 3, 0xffe08a, 0.12); dot(x - 8, 16, 0xfff4c0); dot(x + 8, 16, 0xfff4c0); });
  },
  /** A wooden spice shelf: jars of paprika, turmeric, dried herbs and peppercorns with cork stoppers, a stone mortar and pestle, a braid of garlic and a bunch of bay hanging below; now and then the garlic sways. */
  kitchenspices(x) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, cork = 0xc8a070;
    fill(x - 11, 15, 23, 2, wood); fill(x - 11, 15, 23, 1, woodLight); fill(x - 11, 17, 23, 1, woodDark);
    for (const bx of [x - 9, x + 9]) fill(bx, 18, 2, 2, woodDark);
    [[-10, 0xd8402a], [-6, 0xf2b830], [-2, 0x6a8a3a], [2, 0x2a2a2a]].forEach(([jx, c]) => {
      fill(x + jx, 9, 3, 6, 0xdff0f6, 0.5); fill(x + jx, 11, 3, 4, c); fill(x + jx, 8, 3, 1, cork); dot(x + jx, 10, WHITE, 0.6);
    });
    fill(x + 6, 12, 5, 3, 0x8a8e88); fill(x + 6, 12, 5, 1, 0xa8aca6); line(x + 8, 12, x + 10, 8, 0x6e726c);   // mortar and pestle
    const t = (now + x * 0.21) % 16, sw = t < 2 ? [0, 1, 0, -1][Math.floor(t * 4) % 4] : 0;
    for (let k = 0; k < 3; k++) { dot(x - 7 + sw, 19 + k * 2, 0xf2ead8); dot(x - 6 + sw, 20 + k * 2, 0xe8dcc0); }
    dot(x - 7, 18, 0xc8b890);
    line(x + 4, 18, x + 4, 22, 0x6a8a3a); dot(x + 3, 21, 0x4a7a3a); dot(x + 5, 20, 0x4a7a3a); dot(x + 3, 23, 0x4a7a3a);
    fill(x - 10, 25, 21, 1, SHADOW);
  },
  /** A wall-mounted vital signs monitor on an arm: a green heartbeat trace running across its dark screen, the pulse number blinking, a blue oxygen line and a yellow one under it; it glows at night. */
  hospitalecg(x) {
    const case_ = 0xe8ecee, caseDark = 0xb8bec4, scr = 0x0a1a14;
    fill(x - 1, 2, 3, 4, caseDark);
    fill(x - 11, 6, 23, 17, case_); fill(x - 11, 6, 23, 1, WHITE); fill(x + 11, 6, 1, 17, caseDark); fill(x - 11, 22, 23, 1, caseDark);
    const screen = (a) => {
      fill(x - 9, 8, 17, 12, scr);
      const BEAT = [0, 0, 0, 0, 1, -4, 3, 0, 0, -1, 0, 0];
      const off = Math.floor(now * 8);
      for (let k = 0; k < 15; k++) dot(x - 9 + k, 12 + BEAT[(k + off) % 12], 0x5aff6a, a);
      for (let k = 0; k < 15; k++) dot(x - 9 + k, 16 + Math.round(Math.sin((k + off) * 0.6)), 0x5ad8f0, a * 0.9);
      for (let k = 0; k < 15; k += 2) dot(x - 9 + k, 18, 0xf2d35b, a * 0.7);
      const beat = (now % 0.85) < 0.15; fill(x + 6, 9, 2, 2, 0x5aff6a, beat ? a : a * 0.5);
    };
    screen(1);
    dot(x + 9, 21, 0x5aff6a); fill(x - 9, 21, 4, 1, caseDark);
    fill(x - 10, 23, 21, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x, 14, 9, 6, 0x5aff6a, 0.1); screen(0.85); });
  },
  /** An X-ray lightbox: a chest film clipped to a glowing white panel, the ribs, spine and collarbones pale against the dark; it lights up its corner of the wall, through the night too. */
  hospitalxray(x) {
    const frame = 0xc8ccd2, frameDark = 0x8a9098;
    fill(x - 10, 4, 21, 20, frame); fill(x - 10, 4, 21, 1, WHITE); fill(x + 10, 4, 1, 20, frameDark); fill(x - 10, 23, 21, 1, frameDark);
    const film = (a) => {
      fill(x - 9, 5, 19, 18, 0xeef6fa, a);
      fill(x - 7, 6, 15, 16, 0x1e2a34, a);
      fill(x, 6, 1, 16, 0xc8d4dc, a);
      for (let r = 0; r < 5; r++) { const ry = 9 + r * 2; line(x - 1, ry, x - 6, ry + 2, 0xa8b8c4, a); line(x + 1, ry, x + 6, ry + 2, 0xa8b8c4, a); }
      line(x - 6, 8, x - 1, 7, 0xc8d4dc, a); line(x + 1, 7, x + 6, 8, 0xc8d4dc, a);
      fill(x - 4, 19, 9, 2, 0x8a9aa8, a * 0.6);
    };
    film(1);
    fill(x - 2, 4, 5, 2, 0x6e727c);
    glowAt(x, 14, 12, 10, 0xe8f4ff, 0.08);
    fill(x - 9, 24, 19, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x, 14, 13, 11, 0xe8f4ff, 0.16); film(0.8); });
  },
  /** An eye chart: rows of black tumbling Es getting smaller down a white card, pointing every which way, a red line under the row for good sight. */
  hospitaleyechart(x) {
    fill(x - 7, 3, 15, 23, 0xf8f8f4); fill(x - 7, 3, 15, 1, WHITE); fill(x + 7, 3, 1, 23, 0xd8d8d4); fill(x - 7, 25, 15, 1, 0xd8d8d4);
    const E = (cx, cy, s, dir) => {
      // A tumbling E, s pixels a side, its open side facing dir (0 right, 1 down, 2 left, 3 up)
      for (let i = 0; i < s; i++) for (let j = 0; j < s; j++) {
        const [u, v] = [[i, j], [j, i], [s - 1 - i, j], [j, s - 1 - i]][dir];
        if (u === 0 || (v % 2 === 0 && s > 2) || (s <= 2 && (u === 0 || v === 0))) dot(cx + i, cy + j, 0x1e1e22);
      }
    };
    E(x - 2, 5, 5, 0);
    [[x - 4, 12, 3, 1], [x + 2, 12, 3, 3]].forEach(([ex, ey, s, d]) => E(ex, ey, s, d));
    [[x - 5, 17, 2, 2], [x - 1, 17, 2, 0], [x + 3, 17, 2, 3]].forEach(([ex, ey, s, d]) => E(ex, ey, s, d));
    fill(x - 5, 20, 11, 1, 0xd8402a);
    for (let k = -5; k <= 5; k += 2) dot(x + k, 22, 0x1e1e22, 0.8);
    fill(x - 6, 26, 13, 1, SHADOW);
  },
  /** A big round ward clock in a white rim with black hour marks, showing the real time, its red second hand sweeping round in ticks. */
  hospitalclock(x) {
    const cy = 14;
    disc(x, cy, 8, 0xc8ccd2); disc(x, cy, 7, 0xfafafa); ring(x, cy, 8, 0x8a9098, 0.6);
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 6), cy + Math.round(Math.sin(a) * 6), 0x1e1e22); }
    const d = new Date(), h = ((d.getHours() % 12) + d.getMinutes() / 60) / 12 * Math.PI * 2 - Math.PI / 2, m = d.getMinutes() / 60 * Math.PI * 2 - Math.PI / 2;
    const sec = Math.floor(now) % 60 / 60 * Math.PI * 2 - Math.PI / 2;
    line(x, cy, x + Math.round(Math.cos(h) * 3.4), cy + Math.round(Math.sin(h) * 3.4), 0x1e1e22);
    line(x, cy, x + Math.round(Math.cos(m) * 5.2), cy + Math.round(Math.sin(m) * 5.2), 0x1e1e22);
    line(x, cy, x + Math.round(Math.cos(sec) * 5.6), cy + Math.round(Math.sin(sec) * 5.6), 0xd8402a);
    dot(x, cy, 0xd8402a);
    fill(x - 6, 24, 13, 1, SHADOW);
  },
  /** A green emergency exit sign: a white running figure heading for a doorway on a lit green panel, glowing softly, brighter at night. */
  hospitalexit(x) {
    const green = 0x1e9a4a, greenLight = 0x3ab86a;
    fill(x - 1, 4, 3, 3, 0x8a9098);
    const panel = (a) => {
      fill(x - 8, 7, 17, 10, green, a); fill(x - 8, 7, 17, 1, greenLight, a);
      fill(x + 3, 9, 4, 7, WHITE, a); fill(x + 4, 10, 2, 6, green, a);
      dot(x - 2, 9, WHITE, a); fill(x - 3, 10, 2, 3, WHITE, a); line(x - 3, 11, x - 5, 13, WHITE, a); line(x - 1, 11, x + 1, 10, WHITE, a);
      line(x - 3, 13, x - 5, 15, WHITE, a); line(x - 2, 13, x, 15, WHITE, a);
      fill(x - 7, 15, 3, 1, WHITE, a * 0.8); dot(x - 4, 14, WHITE, a * 0.8);
    };
    panel(1);
    glowAt(x, 12, 10, 6, 0x5aff8a, 0.08);
    fill(x - 7, 17, 15, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x, 12, 11, 7, 0x5aff8a, 0.2); panel(0.9); });
  },
  /** A wall rack of three glove boxes, small, medium and large in blue, purple and white with gloves poking out of their slots, and a hand sanitiser pump beside them that drips now and then. */
  hospitalgloves(x) {
    const wire = 0xa8adb6;
    fill(x - 12, 18, 18, 1, wire);
    [[x - 11, 0x4c8bf5], [x - 5, 0x8a6ad8], [x + 1, 0xf2f4f4]].forEach(([bx, c]) => {
      fill(bx, 9, 5, 9, 0xf2f4f4); fill(bx, 9, 5, 1, WHITE); fill(bx + 4, 9, 1, 9, 0xc8ccd2); fill(bx + 1, 11, 3, 1, c); fill(bx + 1, 15, 3, 2, c, 0.8);
      fill(bx + 1, 7, 3, 2, c); dot(bx + 2, 6, c);
    });
    fill(x + 8, 6, 4, 10, 0xf2f4f4); fill(x + 8, 6, 4, 1, WHITE); fill(x + 11, 6, 1, 10, 0xc8ccd2); fill(x + 8, 9, 4, 3, 0x5ad8f0, 0.6); fill(x + 9, 16, 2, 1, 0x8a9098);
    const t = (now + x * 0.3) % 14;
    if (t < 1.2) dot(x + 10, 17 + Math.floor(t * 5), 0xb8e8f8, 0.8);
    fill(x - 12, 19, 25, 1, SHADOW);
  },
  /** A tall window with venetian blinds half drawn, rain running down the glass in streaks; by night a red neon sign outside flickers through the slats and throws striped light on the wall. */
  noirblinds(x) {
    const frame = 0x2e1e14, slat = 0xe8dcc0, slatDark = 0xb8ac90;
    fill(x - 12, 3, 25, 25, frame); fill(x - 11, 4, 23, 23, isNight ? 0x141a2a : 0x8a9aa8);
    if (isNight) { const on = (Math.floor(now * 3) % 11) !== 0; fill(x + 2, 12, 8, 4, on ? 0xff3a5a : 0x5a1a24); fill(x + 3, 13, 6, 2, on ? 0xff8aa0 : 0x5a1a24); }
    for (let k = 0; k < 5; k++) { const rx = x - 9 + ((k * 5 + Math.floor(now * 1.3 + k)) % 21); const ry = 4 + ((Math.floor(now * 6) + k * 7) % 22); dot(rx, ry, 0xd8e4f0, 0.6); dot(rx, ry + 1, 0xd8e4f0, 0.35); }
    for (let ry = 4; ry < 17; ry += 2) { fill(x - 11, ry, 23, 1, slat); dot(x + 11, ry, slatDark); }
    fill(x - 11, 17, 23, 1, 0xa8906a);
    fill(x - 1, 4, 1, 14, 0xd8ccb0, 0.8);
    fill(x - 13, 27, 27, 2, frame); fill(x - 13, 27, 27, 1, 0x4a3020);
    if (isNight) glowLate(() => {
      const on = (Math.floor(now * 3) % 11) !== 0;
      if (on) { glowAt(x + 6, 14, 8, 6, 0xff3a5a, 0.18); for (let ry = 29; ry < 37; ry += 2) fill(x - 9, ry, 18, 1, 0xff3a5a, 0.12); }
    });
  },
  /** An evidence board of cork: pinned photographs, a scrap of map and newspaper clippings joined by red string from pin to pin; every 20 s another photo is pinned up and the string pulled to it. */
  noirevidence(x) {
    const cork = 0xb8875a, corkDark = 0x8a5a3a, frame = 0x4a3020, red = 0xd8302a;
    fill(x - 13, 4, 27, 20, frame); fill(x - 12, 5, 25, 18, cork);
    for (let k = 0; k < 6; k++) dot(x - 10 + ((hash(k, 3) % 22)), 6 + (hash(k, 5) % 16), corkDark);
    const items = [[-10, 7, 0xe8e4d8, 1], [-3, 13, 0xd8d0b8, 0], [4, 7, 0xe8e4d8, 1], [-9, 16, 0xf2ecd8, 0], [6, 15, 0xe8e4d8, 1]];
    const shown = 3 + Math.floor((now + x * 0.3) / 20) % 3;
    const pins = items.slice(0, shown).map(([px, py, c, photo]) => {
      fill(x + px, py, 5, 5, c); if (photo) { fill(x + px + 1, py + 1, 3, 3, 0x5a5a60); dot(x + px + 2, py + 1, 0x8a8a90); } else for (let ly = py + 1; ly < py + 5; ly += 2) fill(x + px + 1, ly, 3, 1, 0x6a6a70);
      dot(x + px + 2, py, red); return [x + px + 2, py];
    });
    for (let k = 1; k < pins.length; k++) line(pins[k - 1][0], pins[k - 1][1], pins[k][0], pins[k][1], red, 0.8);
    fill(x - 12, 24, 25, 1, SHADOW);
  },
  /** A line-up wall: a height chart of black lines every few inches on pale grey, longer lines with tick bands, and a bright light above it that buzzes and flickers now and then. */
  noirheight(x) {
    fill(x - 10, 4, 21, 23, 0xd8d8d4); fill(x - 10, 4, 21, 1, 0xe8e8e4);
    for (let ly = 7; ly < 27; ly += 2) fill(x - 10, ly, (ly - 7) % 6 === 0 ? 21 : 6, 1, 0x2a2a30, (ly - 7) % 6 === 0 ? 0.8 : 0.5);
    for (let ly = 7; ly < 27; ly += 6) fill(x + 7, ly - 1, 2, 3, 0x2a2a30, 0.6);
    const t = (now + x * 0.3) % 13, flick = t < 0.6 && Math.floor(now * 10) % 2;
    fill(x - 6, 2, 13, 2, 0x5a5e68); fill(x - 5, 4, 11, 1, flick ? 0x8a8a80 : 0xfff8d8);
    if (!flick) glowAt(x, 8, 10, 5, 0xfff8d8, 0.1);
    fill(x - 9, 27, 19, 1, SHADOW);
  },
  /** A city street map pinned to the wall, a grid of streets with a river winding through and a park, coloured pins in it, and one red pin whose district is ringed in grease pencil. */
  noircitymap(x) {
    fill(x - 11, 5, 23, 18, 0xe8e0c8); fill(x - 11, 5, 23, 1, 0xf6f0dc); fill(x + 11, 5, 1, 18, 0xc8c0a8);
    for (let gx = x - 9; gx < x + 11; gx += 4) fill(gx, 6, 1, 16, 0xc8b898);
    for (let gy = 8; gy < 22; gy += 3) fill(x - 11, gy, 23, 1, 0xc8b898);
    for (let k = 0; k < 23; k++) dot(x - 11 + k, 14 + Math.round(Math.sin(k * 0.5) * 2), 0x6a9ac8);
    fill(x + 3, 7, 5, 4, 0x8ab87a);
    for (const [px, py, c] of [[-7, 9, 0x4c8bf5], [-2, 18, 0xf2d35b], [8, 19, 0x3ab86a]]) dot(x + px, py, c);
    const blink = Math.floor(now * 1.5 + x) % 3;
    ellipseRing(x - 4, 11, 3, 2, 0xd8302a, 0.7); dot(x - 4, 11, blink ? 0xd8302a : 0xff8070);
    for (const [cx, cy] of [[-11, 5], [11, 5], [-11, 22], [11, 22]]) dot(x + cx, cy, 0xd8b45a);
    fill(x - 10, 23, 21, 1, SHADOW);
  },
  /** A bentwood hat rack on the wall with a grey fedora and a beige trench coat hanging from its hooks; now and then the coat's belt swings. */
  noirhatrack(x) {
    const wood = 0x5a3b25, woodLight = 0x7a5232;
    fill(x - 7, 5, 15, 2, wood); fill(x - 7, 5, 15, 1, woodLight);
    for (const hx of [-5, 0, 5]) { dot(x + hx, 7, wood); dot(x + hx, 8, woodLight); }
    fill(x - 4, 3, 7, 2, 0x6a6a70); fill(x - 5, 5, 9, 1, 0x5a5a60); fill(x - 4, 4, 7, 1, 0x1e1e22); dot(x - 1, 2, 0x6a6a70);
    const coat = 0xc8b088, coatDark = 0xa08a62, t = (now + x * 0.31) % 16, sw = t < 1.6 ? [0, 1, 0, -1][Math.floor(t * 5) % 4] : 0;
    fill(x + 3, 9, 5, 14, coat); fill(x + 2, 11, 7, 12, coat); fill(x + 8, 11, 1, 12, coatDark); line(x + 4, 9, x + 5, 13, coatDark); line(x + 7, 9, x + 6, 13, coatDark);
    fill(x + 2, 16, 7, 1, coatDark); fill(x + 4 + sw, 17, 1, 4, coatDark);
    fill(x - 6, 24, 13, 1, SHADOW);
  },
  /** A frosted glass office partition in a dark wooden frame, lit softly from the room behind; now and then a shadowy figure in a hat stops behind it, then moves on. Lit through the night. */
  noirfrosted(x) {
    const wood = 0x3a2418, woodLight = 0x5a3b25, glass = 0xe0dccc;
    fill(x - 11, 3, 23, 26, wood); fill(x - 11, 3, 23, 1, woodLight);
    const pane = (a) => { fill(x - 9, 5, 19, 22, glass, a); fill(x - 9, 5, 19, 1, 0xf2eee0, a); for (let k = 0; k < 4; k++) dot(x - 7 + k * 5, 8 + k * 4, WHITE, 0.4 * a); };
    pane(1);
    const t = (now + x * 0.27) % 28;
    if (t < 6) {
      const sx = Math.round(x - 12 + Math.min(1, t / 2) * 12 + Math.max(0, t - 4) * 6);
      if (sx > x - 9 && sx < x + 8) { fill(sx, 12, 5, 15, 0x4a4438, 0.45); fill(sx + 1, 9, 3, 3, 0x4a4438, 0.45); fill(sx - 1, 8, 7, 1, 0x4a4438, 0.45); fill(sx, 7, 5, 1, 0x4a4438, 0.45); }
    }
    fill(x - 11, 16, 23, 1, woodLight, 0.8);
    if (isNight) glowLate(() => { glowAt(x, 15, 10, 11, 0xffe8b0, 0.12); });
  },
  /** A great knife switch on a slate panel: a copper blade on its hinge with a black handle; every 22 s it's thrown, the blade slamming home into its clips with a shower of blue sparks, then pulled back. */
  madlablever(x) {
    const slate = 0x3a3c44, slateLight = 0x5a5e68, copper = 0xb87333, copperLight = 0xe0a060;
    fill(x - 8, 4, 17, 22, slate); fill(x - 8, 4, 17, 1, slateLight); for (const [rx, ry] of [[-7, 5], [7, 5], [-7, 24], [7, 24]]) dot(x + rx, ry, 0x8a8a90);
    fill(x - 4, 21, 9, 2, copper); fill(x - 4, 21, 9, 1, copperLight);   // the hinge block
    fill(x - 4, 7, 2, 3, copper); fill(x + 3, 7, 2, 3, copper);          // the clips
    const t = (now + x * 0.3) % 22, on = t < 3;
    if (on) {
      fill(x - 3, 9, 1, 12, copperLight); fill(x + 3, 9, 1, 12, copperLight); fill(x - 3, 9, 7, 1, copper); fill(x - 1, 6, 3, 3, 0x1e1e22);
      if (t < 0.8) { for (const [sx, sy] of [[-6, 7], [6, 6], [-5, 11], [7, 10], [0, 4]]) dot(x + sx, sy + (Math.floor(now * 12) % 2), 0x8ae0ff); glowAt(x, 9, 7, 4, 0x8ae0ff, 0.3); }
    } else {
      line(x - 3, 21, x - 8, 13, copperLight); line(x + 3, 21, x - 2, 13, copperLight); line(x - 8, 13, x - 2, 13, copper); fill(x - 7, 10, 3, 3, 0x1e1e22);
    }
    fill(x - 7, 26, 15, 1, SHADOW);
    if (isNight && on && t < 0.8) glowLate(() => glowAt(x, 9, 8, 5, 0x8ae0ff, 0.35));
  },
  /** A Jacob's ladder: two tall copper rods in a V on a porcelain base, an electric arc climbing between them, widening as it rises until it snaps at the top and starts again at the bottom; it glows, more at night. */
  madlabladder(x) {
    const copper = 0xb87333, base = 0xf2ecd8;
    fill(x - 5, 24, 11, 3, base); fill(x - 5, 24, 11, 1, WHITE); fill(x - 4, 27, 9, 1, 0xc8c0a8);
    line(x - 1, 23, x - 5, 4, copper); line(x + 1, 23, x + 5, 4, copper);
    const ph = ((now * 0.9) + x * 0.07) % 1, ay = Math.round(22 - ph * 17), half = 1 + Math.round((22 - ay) * 4 / 18);
    const arc = () => {
      for (let k = -half; k <= half; k++) dot(x + k, ay + (Math.floor(now * 20 + k) % 2), 0xd8f4ff);
      dot(x, ay - 1, 0x8ae0ff); glowAt(x, ay, half + 2, 3, 0x8ae0ff, 0.25);
    };
    arc();
    if (isNight) glowLate(arc);
  },
  /** A wooden shelf of big glass jars glowing in green, violet and amber, each with something floating in it, an eye, a seed pod, a little tentacled thing, bubbles rising slowly; they glow through the night. */
  madlabjars(x) {
    const wood = 0x5a3b25, woodLight = 0x7a5232;
    fill(x - 12, 22, 25, 2, wood); fill(x - 12, 22, 25, 1, woodLight); for (const bx of [x - 10, x + 9]) fill(bx, 24, 2, 3, wood);
    const jars = [[x - 8, 0x7ae04a], [x, 0xb88aff], [x + 8, 0xf2b840]];
    const draw = (a) => jars.forEach(([jx, c], k) => {
      fill(jx - 3, 11, 7, 11, c, 0.5 * a); fill(jx - 3, 10, 7, 1, 0x6e727c); fill(jx - 2, 9, 5, 1, 0x8a8e98);
      if (k === 0) { disc(jx, 16, 1.6, 0xf2f2ee, a); dot(jx, 16, 0x3a6ac8, a); dot(jx, 16, PALETTE.eye, a * 0.5); }
      if (k === 1) { fill(jx - 1, 14, 3, 3, 0x6a3a8a, a); for (const dx of [-1, 0, 1]) dot(jx + dx, 17 + (Math.floor(now + dx) % 2), 0x6a3a8a, a); }
      if (k === 2) { fill(jx - 1, 15, 3, 4, 0x8a6a2a, a); dot(jx, 14, 0x5a9a4a, a); }
      const b = (now * 0.6 + k) % 1; dot(jx + 1, 20 - Math.floor(b * 9), WHITE, 0.5 * a);
      dot(jx - 2, 12, WHITE, 0.6);
    });
    draw(1);
    fill(x - 11, 27, 23, 1, SHADOW);
    if (isNight) glowLate(() => { jars.forEach(([jx, c]) => glowAt(jx, 16, 5, 6, c, 0.14)); draw(0.7); });
  },
  /** A blackboard covered in chalk: rows of scrawled formulas, a circled atom with electrons that move round it, a diagram of arrows; a line of chalk gets underlined twice every 20 s. */
  madlabchalk(x) {
    const frame = 0x6a4428, board = 0x2a3a32, chalk = 0xe8ece4;
    fill(x - 13, 4, 27, 20, frame); fill(x - 12, 5, 25, 18, board);
    for (const [lx, ly, w] of [[-10, 7, 9], [-10, 10, 6], [-10, 13, 8], [-10, 16, 5], [-10, 19, 10]]) for (let k = 0; k < w; k++) if (hash(x + k, ly) % 4) dot(x + lx + k, ly, chalk, 0.8);
    dot(x - 3, 13, chalk); dot(x - 2, 12, chalk); dot(x - 2, 14, chalk);   // an equals and a plus
    ellipseRing(x + 6, 12, 5, 2, chalk, 0.7); ellipseRing(x + 6, 12, 2, 5, chalk, 0.7); dot(x + 6, 12, 0xf2d35b);
    const a = now * 1.5; dot(x + 6 + Math.round(Math.cos(a) * 5), 12 + Math.round(Math.sin(a) * 2), 0x8ae0ff);
    line(x + 1, 19, x + 5, 19, chalk, 0.7); dot(x + 5, 18, chalk, 0.7); dot(x + 5, 20, chalk, 0.7); fill(x + 7, 18, 3, 3, chalk, 0.5);
    const t = (now + x * 0.3) % 20;
    if (t < 3) { const n = Math.min(10, Math.floor(t * 6)); fill(x - 10, 21, n, 1, chalk, 0.8); if (t > 1.6) fill(x - 10, 22, Math.min(10, Math.floor((t - 1.6) * 8)), 1, chalk, 0.6); }
    fill(x - 12, 24, 25, 1, 0x8a6a4a); fill(x - 9, 23, 3, 1, chalk);
  },
  /** A brass panel of gauges and lamps: three round dials whose needles drift and twitch, a row of little bulbs blinking in turn, and one that flashes red now and then; the bulbs glow at night. */
  madlabgauges(x) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a;
    fill(x - 11, 5, 23, 20, brassDark); fill(x - 10, 6, 21, 18, brass); fill(x - 10, 6, 21, 1, brassLight);
    [[x - 6, 11], [x + 1, 11], [x + 7, 15]].forEach(([dx, dy], k) => {
      disc(dx, dy, 3, 0x3a2a1a); disc(dx, dy, 2.4, 0xf4eedc);
      const a = -Math.PI / 2 + Math.sin(now * (0.4 + k * 0.3) + k) * 1.2 + (k === 2 && (now % 7) < 0.4 ? 0.8 : 0);
      line(dx, dy, dx + Math.round(Math.cos(a) * 2), dy + Math.round(Math.sin(a) * 2), 0xd8302a);
    });
    const lamps = () => { for (let k = 0; k < 5; k++) { const on = Math.floor(now * 3) % 5 === k; dot(x - 9 + k * 3, 20, on ? 0xf2d35b : 0x6a5a2a); } dot(x + 8, 20, (now % 5) < 0.5 ? 0xff3020 : 0x5a1a14); };
    lamps();
    fill(x - 10, 25, 21, 1, SHADOW);
    if (isNight) glowLate(lamps);
  },
  /** A tall gothic arched window with leaded panes onto a stormy sky, dark towers below; every 16 s lightning forks across it and lights the room for a blink. */
  madlabwindow(x) {
    const stone = 0x6a6670, stoneLight = 0x8a8690, lead = 0x2a2a30;
    const t = (now + x * 0.3) % 16, flash = t < 0.15 || (t > 0.3 && t < 0.4);
    const sky = flash ? 0xd8e0ff : isNight ? 0x1a1830 : 0x5a5a78;
    for (let dy = 3; dy < 27; dy++) {
      const half = dy < 9 ? Math.round(Math.sqrt(Math.max(0, 36 - (9 - dy) * (9 - dy)))) + 1 : 7;
      fill(x - half - 1, dy, half * 2 + 3, 1, dy < 9 && half < 4 ? stoneLight : stone);
      fill(x - half + 1, dy, half * 2 - 1, 1, sky);
    }
    fill(x - 6, 20, 13, 7, sky);
    fill(x - 6, 21, 3, 6, 0x14121c); fill(x + 2, 19, 2, 8, 0x14121c); fill(x + 1, 18, 4, 1, 0x14121c); fill(x - 2, 23, 4, 4, 0x14121c);
    fill(x, 4, 1, 23, lead); for (const ly of [10, 16, 22]) fill(x - 6, ly, 13, 1, lead);
    if (t < 0.5) { line(x - 3, 4, x - 1, 9, 0xffffff); line(x - 1, 9, x - 4, 13, 0xffffff); line(x - 4, 13, x - 2, 17, 0xe8f0ff); }
    fill(x - 9, 27, 19, 2, stone); fill(x - 9, 27, 19, 1, stoneLight);
    if (flash) glowAt(x, 18, 18, 12, 0xd8e0ff, 0.25);
  },
  /** A Vegas marquee: a gold-framed sign of red and gold rays round a big white star, ringed with bulbs that chase round it; brighter at night. */
  casinomarquee(x) {
    const gold = 0xd8b45a, goldDark = 0x9a7a30;
    fill(x - 12, 5, 25, 19, goldDark); fill(x - 11, 6, 23, 17, 0xa8182a);
    for (let k = 0; k < 8; k++) { const a = (k / 8) * Math.PI * 2; line(x, 14, x + Math.round(Math.cos(a) * 10), 14 + Math.round(Math.sin(a) * 7), k % 2 ? 0xd8302a : 0xf2b840); }
    for (const [dx, dy] of [[0, -3], [-1, -1], [1, -1], [-3, -1], [3, -1], [-2, 0], [2, 0], [-1, 1], [1, 1], [-2, 2], [2, 2], [0, 0], [0, 1], [0, -2], [-1, 0], [1, 0]]) dot(x + dx, 14 + dy, WHITE);
    const bulbs = []; for (let bx = -11; bx <= 11; bx += 2) { bulbs.push([bx, 5], [bx, 23]); } for (let by = 7; by <= 21; by += 2) { bulbs.push([-12, by], [12, by]); }
    const lit = (a) => bulbs.forEach(([bx, by], k) => dot(x + bx, by, (k + Math.floor(now * 6)) % 3 === 0 ? 0xfff4c0 : gold, a));
    lit(1);
    fill(x - 11, 24, 23, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x, 14, 13, 10, 0xffd070, 0.08); lit(1); });
  },
  /** A prize wheel of red, black and gold slices with a gold hub and a pointer at the top; every 30 s it's spun, whirling round and clicking to a stop. */
  casinowheel(x) {
    const cy = 14, cols = [0xd8302a, 0x1e1e22, 0xf2b840, 0x1e1e22, 0xd8302a, 0x1e1e22, 0x3ab86a, 0x1e1e22];
    disc(x, cy, 9.4, 0x9a7a30);
    const t = (now + x * 0.3) % 30, spin = t < 4 ? (1 - t / 4) * (1 - t / 4) * 40 : 0, base = Math.floor((now + x * 0.3) / 30) * 2.3;
    const a0 = base + (t < 4 ? 40 * (t / 4 - (t / 4) * (t / 4) / 2) : 40 / 2);
    for (let dy = -8; dy <= 8; dy++) for (let dx = -8; dx <= 8; dx++) {
      const r = Math.hypot(dx, dy); if (r > 8.4) continue;
      const s = Math.floor((((Math.atan2(dy, dx) - a0) / (Math.PI * 2)) % 1 + 1) % 1 * 8);
      dot(x + dx, cy + dy, r > 7.4 ? 0xd8b45a : cols[s]);
    }
    disc(x, cy, 1.6, 0xf2d888);
    fill(x - 1, 3, 3, 2, 0xf6f6f2); dot(x, 5, 0xf6f6f2);
    void spin;
    fill(x - 7, 24, 15, 1, SHADOW);
  },
  /** A neon sign of a tilted cocktail glass with an olive, in pink and cyan tubes on a dark backing; it flickers now and then and glows, brightest at night. */
  casinoneonsign(x) {
    fill(x - 9, 5, 19, 19, 0x14101e); fill(x - 9, 5, 19, 1, 0x2a2440);
    const flick = (Math.floor(now * 4 + x) % 23) === 0;
    const tubes = (a) => {
      const pink = 0xff4ab8, cyan = 0x4ae8f0;
      line(x - 6, 8, x + 6, 8, cyan, a); line(x - 6, 8, x, 15, cyan, a); line(x + 6, 8, x, 15, cyan, a); fill(x, 15, 1, 5, cyan, a); fill(x - 3, 20, 7, 1, cyan, a);
      if (!flick) { disc(x + 2, 10, 1.3, pink, a); line(x + 2, 10, x + 5, 6, pink, a); }
    };
    tubes(1);
    fill(x - 8, 24, 17, 1, SHADOW);
    glowAt(x, 13, 9, 8, 0xff4ab8, 0.06);
    if (isNight) glowLate(() => { glowAt(x, 13, 10, 9, 0x4ae8f0, 0.12); tubes(1); });
  },
  /** A framed royal flush: five playing cards fanned out under glass, ace to ten of hearts, on green felt in a gold frame; a gleam crosses the glass now and then. */
  casinoflush(x) {
    fill(x - 11, 6, 23, 17, 0xd8b45a); fill(x - 10, 7, 21, 15, 0x1e6a3a);
    for (let k = 0; k < 5; k++) {
      const cx = x - 9 + k * 4, cy = 10 - Math.abs(k - 2);
      fill(cx, cy, 4, 7, 0xf6f6f2); fill(cx, cy, 4, 1, WHITE); fill(cx + 3, cy, 1, 7, 0xc8c8c4);
      dot(cx + 1, cy + 1, 0xd8302a); if (k === 4) { dot(cx + 2, cy + 3, 0xd8302a); dot(cx + 1, cy + 4, 0xd8302a); dot(cx + 3, cy + 4, 0xd8302a); dot(cx + 2, cy + 5, 0xd8302a); }
    }
    const t = (now + x * 0.2) % 13; if (t < 1) line(x - 10 + Math.floor(t * 24), 7, x - 14 + Math.floor(t * 24), 21, WHITE, 0.4);
    fill(x - 10, 23, 21, 1, SHADOW);
  },
  /** A jackpot meter: a long gold frame with red digits on black, the progressive total always ticking up, the last digits rolling, and a row of lights chasing under it. Lit through the night. */
  casinojackpot(x) {
    const DIGITS = ['111101101101111', '010110010010111', '111001111100111', '111001111001111', '101101111001001', '111100111001111', '111100111101111', '111001001001001', '111101111101111', '111101111001111'];
    fill(x - 13, 8, 27, 13, 0xd8b45a); fill(x - 13, 8, 27, 1, 0xf2d888); fill(x - 12, 9, 25, 9, 0x0a0808);
    const total = 482000 + Math.floor((now + x) * 3.7);
    const show = (a) => {
      String(total).padStart(6, '0').slice(-6).split('').forEach((c, k) => {
        const d = DIGITS[+c], dx = x - 11 + k * 4;
        for (let i = 0; i < 15; i++) if (d[i] === '1') dot(dx + (i % 3), 11 + Math.floor(i / 3), 0xff3a28, a);
      });
      for (let k = 0; k < 12; k++) dot(x - 11 + k * 2, 19, (k + Math.floor(now * 8)) % 4 === 0 ? 0xfff4c0 : 0x9a7a30, a);
    };
    show(1);
    fill(x - 12, 21, 25, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x, 14, 12, 4, 0xff3a28, 0.12); show(1); });
  },
  /** A crystal chandelier hanging from the ceiling on a gold chain: gold arms with candle bulbs and tiers of glass drops that sparkle in turn, glowing warm, through the night too. */
  casinochandelier(x) {
    const gold = 0xd8b45a, goldDark = 0x9a7a30;
    for (let cy = 0; cy < 6; cy += 2) dot(x, cy, goldDark);
    fill(x - 1, 6, 3, 3, gold); fill(x - 9, 10, 19, 1, gold); line(x - 9, 10, x - 10, 8, gold); line(x + 9, 10, x + 10, 8, gold);
    const bulbs = [-10, -5, 0, 5, 10];
    const lit = () => bulbs.forEach((bx) => { fill(x + bx, 6, 1, 2, 0xfff4c0); dot(x + bx, 5, 0xffe08a); });
    for (const bx of bulbs) fill(x + bx, 8, 1, 2, 0xf6f0dc);
    const drops = [];
    for (let k = -8; k <= 8; k += 2) drops.push([k, 12 + (Math.abs(k) < 5 ? 2 : 0)]);
    for (let k = -5; k <= 5; k += 2) drops.push([k, 17 - Math.abs(k) / 2]);
    drops.forEach(([dx, dy], k) => { const sp = (k + Math.floor(now * 3)) % 7 === 0; dot(x + dx, Math.round(dy), sp ? WHITE : 0xc8e8f4); dot(x + dx, Math.round(dy) + 1, 0xa8d0e8, 0.6); });
    fill(x - 1, 19, 3, 2, gold); dot(x, 21, 0xc8e8f4);
    lit(); glowAt(x, 9, 12, 6, 0xffe08a, 0.08);
    if (isNight) glowLate(() => { glowAt(x, 10, 13, 8, 0xffe08a, 0.18); lit(); });
  },
  /** A double-faced station clock hanging from an ornate cast-iron bracket, a white dial with black numerals in a gilt rim, showing the real time; its gilt rim catches the light now and then. */
  orientclock(x) {
    const cy = 15, iron = 0x2a2a30;
    fill(x - 1, 1, 3, 4, iron); line(x - 4, 3, x + 4, 3, iron); dot(x - 5, 4, iron); dot(x + 5, 4, iron);
    disc(x, cy, 8, 0xd8b45a); disc(x, cy, 7, 0xf8f4ea); ring(x, cy, 7.4, 0x9a7a30, 0.6);
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2; const r = k % 3 ? 5.6 : 5.2; dot(x + Math.round(Math.cos(a) * r), cy + Math.round(Math.sin(a) * r), 0x1e1e22); if (!(k % 3)) dot(x + Math.round(Math.cos(a) * 4.6), cy + Math.round(Math.sin(a) * 4.6), 0x1e1e22); }
    const d = new Date(), h = ((d.getHours() % 12) + d.getMinutes() / 60) / 12 * Math.PI * 2 - Math.PI / 2, m = d.getMinutes() / 60 * Math.PI * 2 - Math.PI / 2;
    line(x, cy, x + Math.round(Math.cos(h) * 3), cy + Math.round(Math.sin(h) * 3), 0x1e1e22); line(x, cy, x + Math.round(Math.cos(m) * 5), cy + Math.round(Math.sin(m) * 5), 0x1e1e22);
    dot(x, cy, 0xd8b45a); fill(x - 1, 23, 3, 2, 0xd8b45a); dot(x, 25, 0x9a7a30);
    if ((now + x * 0.2) % 11 < 0.6) dot(x - 5, cy - 5, WHITE);
  },
  /** A carriage window with mahogany frame and a little brass rail and drawn-back velvet curtains; the countryside slides past outside, hills, poplars and telegraph poles, and by night a dark landscape under the moon with the odd lit farmhouse window. */
  orientwindow(x) {
    const wood = 0x6a2e1e, woodLight = 0x8a4a2e, velvet = 0x8a1a2a, velvetDark = 0x5a1018;
    fill(x - 12, 4, 25, 20, wood); fill(x - 12, 4, 25, 1, woodLight);
    const L = x - 10, W = 21, sky = isNight ? 0x141e3a : 0xb8d8f0, hill = isNight ? 0x1e2a2a : 0x6a9a5a, far = isNight ? 0x24304a : 0x9ab8c8;
    fill(L, 6, W, 16, sky);
    const off = now * 6 + x * 3;
    for (let k = 0; k < W; k++) {
      const wx = k + off, fh = 13 + Math.round(Math.sin(wx * 0.07) * 2), nh = 17 + Math.round(Math.sin(wx * 0.13 + 1) * 1.5);
      fill(L + k, fh, 1, 22 - fh, far); fill(L + k, nh, 1, 22 - nh, hill);
      const n = Math.floor(wx);
      if (hash(n, 3) % 11 === 0) { fill(L + k, nh - 4, 1, 4, isNight ? 0x141e1a : 0x3a6a3a); dot(L + k, nh - 5, isNight ? 0x141e1a : 0x4a7a3a); }
      if (isNight && hash(n, 8) % 29 === 0) dot(L + k, nh + 1, 0xffe08a);
    }
    for (let px = ((-Math.floor(off * 2)) % 14 + 14) % 14; px < W; px += 14) { fill(L + px, 8, 1, 14, isNight ? 0x0e1420 : 0x5a4a3a); dot(L + px - 1, 9, isNight ? 0x0e1420 : 0x5a4a3a); dot(L + px + 1, 9, isNight ? 0x0e1420 : 0x5a4a3a); }
    if (isNight) { dot(x + 6, 9, 0xf4eec8); glowLate(() => dot(x + 6, 9, 0xfffbe8)); }
    // The curtains, tied back
    fill(x - 12, 4, 3, 19, velvet); fill(x - 12, 4, 1, 19, velvetDark); fill(x + 10, 4, 3, 19, velvet); fill(x + 12, 4, 1, 19, velvetDark);
    dot(x - 10, 14, 0xd8b45a); dot(x + 10, 14, 0xd8b45a);
    fill(x - 12, 24, 25, 1, 0xd8b45a); fill(x - 12, 25, 25, 1, SHADOW);
  },
  /** An art deco travel poster: a streamlined locomotive rushing out of a golden sunburst on a navy sky above a pale band, in a thin gold frame; its headlamp gleams now and then. */
  orientposter(x) {
    fill(x - 8, 3, 17, 23, 0xd8b45a); fill(x - 7, 4, 15, 21, 0x1e2a4a);
    for (let k = 0; k < 7; k++) { const a = Math.PI + (k / 6) * Math.PI; line(x, 15, x + Math.round(Math.cos(a) * 8), 15 + Math.round(Math.sin(a) * 9), 0xe8b040, 0.8); }
    disc(x, 15, 3, 0xf2d35b);
    fill(x - 7, 17, 15, 8, 0xeee2c4); fill(x - 7, 17, 15, 1, 0xd8c8a0);
    fill(x - 6, 15, 11, 4, 0x2a2a30); fill(x + 4, 16, 2, 3, 0x2a2a30); fill(x - 6, 15, 11, 1, 0xc8302a); dot(x - 7, 16, 0x2a2a30);
    for (const wx of [-4, -1, 2]) disc(x + wx, 19, 1, 0xc8302a);
    dot(x + 5, 16, (now + x * 0.3) % 7 < 0.6 ? WHITE : 0xf2d35b);
    for (let k = -6; k < 6; k += 2) dot(x + k, 22, 0x1e2a4a, 0.6);
    fill(x - 7, 26, 15, 1, SHADOW);
  },
  /** A brass luggage rack along the wall with leather suitcases, a striped hatbox and a carpet bag on it, travel labels pasted on; now and then the rack rattles as if the train went over points. */
  orientluggage(x) {
    const brass = 0xc8a050, brassLight = 0xe8c878;
    const t = (now + x * 0.3) % 14, sh = t < 1 ? (Math.floor(now * 12) % 2) : 0;
    fill(x - 13, 16, 27, 1, brassLight); fill(x - 13, 18, 27, 1, brass); for (const bx of [-12, 12]) { line(x + bx, 16, x + bx, 22, brass); dot(x + bx, 22, brass); }
    fill(x - 11, 8 - sh, 10, 8, 0x8a4a2a); fill(x - 11, 8 - sh, 10, 1, 0xa8643a); fill(x - 7, 7 - sh, 3, 1, 0x5a2e18); fill(x - 9, 10 - sh, 3, 2, 0xf2ead8); dot(x - 8, 10 - sh, 0xc8302a);
    fill(x, 10, 7, 6, 0xf2ead8); for (let k = 0; k < 7; k += 2) fill(x + k, 10, 1, 6, 0x2a5a4a); fill(x, 9, 7, 1, 0x2a5a4a); fill(x + 2, 8, 3, 1, 0xc8302a);
    fill(x + 8, 11 - sh, 5, 5, 0x8a2a3a); fill(x + 8, 11 - sh, 5, 1, 0xa83a4a); dot(x + 10, 10 - sh, brass); dot(x + 9, 13 - sh, 0xd8b45a);
    fill(x - 12, 23, 25, 1, SHADOW);
  },
  /** An art deco wall sconce: a fan of frosted glass on a brass mount throwing warm light up the wall, through the night too. */
  orientsconce(x) {
    fill(x - 1, 18, 3, 4, 0xc8a050); fill(x - 2, 21, 5, 1, 0xe8c878);
    const fan = (a) => { for (let k = 0; k < 5; k++) { const ang = Math.PI + 0.3 + (k / 4) * (Math.PI - 0.6); line(x, 17, x + Math.round(Math.cos(ang) * 5), 17 + Math.round(Math.sin(ang) * 6), 0xfff4d8, a); } };
    fan(0.85); fill(x - 3, 16, 7, 2, 0xc8a050); dot(x, 17, 0xfff4c0);
    glowAt(x, 12, 6, 8, 0xffe0a0, 0.08);
    if (isNight) glowLate(() => { glowAt(x, 11, 7, 9, 0xffe0a0, 0.2); fan(1); });
  },
  /** A long route map across a continent on aged paper in a mahogany frame: a red line from city to city across rivers and mountains, and a little gold train that creeps along it, its stops lighting as it passes. */
  orientroute(x) {
    fill(x - 13, 6, 27, 17, 0x6a2e1e); fill(x - 12, 7, 25, 15, 0xeee2c4);
    for (let k = 0; k < 25; k++) dot(x - 12 + k, 18 + Math.round(Math.sin(k * 0.4) * 1.5), 0x8ab8d8, 0.8);
    for (const [mx, my] of [[-4, 10], [-2, 9], [0, 10], [7, 11], [9, 10]]) { dot(x + mx, my, 0x8a7a5a); dot(x + mx - 1, my + 1, 0x8a7a5a); dot(x + mx + 1, my + 1, 0x8a7a5a); }
    const stops = [[-10, 16], [-5, 13], [1, 15], [6, 13], [11, 14]];
    for (let k = 1; k < stops.length; k++) line(x + stops[k - 1][0], stops[k - 1][1], x + stops[k][0], stops[k][1], 0xc8302a, 0.8);
    const prog = ((now * 0.04 + x * 0.01) % 1) * 4, seg = Math.floor(prog), f = prog - seg;
    stops.forEach(([sx, sy], k) => dot(x + sx, sy, k <= seg ? 0xd8b45a : 0x5a4a3a));
    const [ax, ay] = stops[seg], [bx, by] = stops[seg + 1];
    fill(Math.round(x + ax + (bx - ax) * f) - 1, Math.round(ay + (by - ay) * f) - 1, 2, 2, 0xd8b45a);
    fill(x - 12, 23, 25, 1, SHADOW);
  },
  /** A tall gothic window onto a huge harvest moon behind ragged clouds that drift across it, black branches clawing in front; bats flit past now and then, and every 40 s a witch on her broom crosses the moon. Moonlight spills from it, through the night. */
  halloweendxmoon(x) {
    const stone = 0x4a4650, stoneLight = 0x6a6670, lead = 0x14101a;
    const sky = isNight ? 0x1a1030 : 0x4a2a5a, moon = 0xf2d890, moonDark = 0xd8b870;
    for (let dy = 3; dy < 27; dy++) {
      const half = dy < 10 ? Math.round(Math.sqrt(Math.max(0, 81 - (10 - dy) * (10 - dy)))) + 1 : 10;
      fill(x - half - 1, dy, half * 2 + 3, 1, dy < 10 && half < 5 ? stoneLight : stone); fill(x - half + 1, dy, half * 2 - 1, 1, sky);
    }
    const inner = () => {
      disc(x + 1, 14, 6.4, moon); for (const [mx, my, r] of [[-1, 12, 1.4], [3, 16, 1], [2, 11, 0.8], [-2, 17, 0.8]]) disc(x + mx, my, r, moonDark);
      const cl = Math.floor(now * 0.8 + x) % 30;
      for (const [cx, cy, w] of [[cl - 12, 10, 8], [cl - 22, 18, 10], [cl - 2, 21, 7]]) if (cx > -10 && cx < 10) fill(x + cx, cy, Math.min(w, 10 - cx), 2, isNight ? 0x2a1e3a : 0x6a4a6a, 0.85);
      const w = (now + x * 0.3) % 40;
      if (w < 4) { const wx = Math.round(x - 10 + (w / 4) * 20), wy = 13 - Math.round(Math.sin((w / 4) * Math.PI) * 3); if (Math.abs(wx - x) < 9) { line(wx - 4, wy + 2, wx + 3, wy, 0x0a0810); fill(wx - 1, wy - 3, 2, 3, 0x0a0810); dot(wx, wy - 4, 0x0a0810); dot(wx - 1, wy - 5, 0x0a0810); fill(wx - 2, wy - 4, 4, 1, 0x0a0810); for (const dx of [-5, -6]) dot(wx + dx, wy + 2 + (dx + 5), 0x0a0810); } }
      const bt = (now + x * 0.17) % 9;
      if (bt < 2.5) for (let k = 0; k < 2; k++) { const bx = Math.round(x + 9 - (bt / 2.5) * 18 + k * 3), by = 8 + k * 3 + (Math.floor(now * 10 + k) % 2), flap = Math.floor(now * 12 + k) % 2; if (Math.abs(bx - x) < 9) { dot(bx, by, 0x0a0810); dot(bx - 1, by - flap, 0x0a0810); dot(bx + 1, by - flap, 0x0a0810); } }
    };
    inner();
    line(x - 9, 24, x - 4, 18, 0x0a0810); line(x - 4, 18, x - 6, 14, 0x0a0810); line(x - 4, 18, x, 16, 0x0a0810); line(x + 9, 22, x + 5, 20, 0x0a0810);
    for (let ly = 4; ly < 27; ly++) if (Math.hypot(1, ly - 14) > 7) dot(x, ly, lead); for (const ly of [22, 25]) fill(x - 9, ly, 19, 1, lead, 0.8);
    fill(x - 12, 27, 25, 2, stone); fill(x - 12, 27, 25, 1, stoneLight);
    if (isNight) glowLate(() => { glowAt(x + 1, 14, 11, 10, 0xf2d890, 0.06); glowAt(x + 1, 14, 8, 7, 0xf2d890, 0.08); inner(); for (let ry = 30; ry < 38; ry += 2) fill(x - 7, ry, 15, 1, 0xf2d890, 0.06); });
  },
  /** A gilded portrait of a pale lady in a high black collar, her eyes dark; every 18 s lightning flashes, and for that blink she is a grinning skull, then herself again. Two candles flicker below it, glowing at night. */
  halloweendxportrait(x) {
    const gold = 0xc8a050, goldDark = 0x8a6a2a;
    fill(x - 8, 3, 17, 21, goldDark); fill(x - 7, 4, 15, 19, gold); fill(x - 6, 5, 13, 17, 0x1e1a24);
    const t = (now + x * 0.3) % 18, flash = t < 0.18 || (t > 0.3 && t < 0.42);
    if (flash) {
      fill(x - 6, 5, 13, 17, 0xd8dcf0); fill(x - 3, 8, 7, 6, 0xf2eee0); fill(x - 2, 14, 5, 2, 0xf2eee0); fill(x - 2, 10, 2, 2, 0x1e1a24); fill(x + 1, 10, 2, 2, 0x1e1a24); dot(x, 12, 0x1e1a24);
      for (let k = -2; k <= 2; k += 2) dot(x + k, 15, 0x1e1a24);
      fill(x - 4, 17, 9, 5, 0x1e1a24);
    } else {
      fill(x - 3, 7, 7, 3, 0x2a1e18); fill(x - 2, 9, 5, 5, 0xe8d8c8); fill(x - 3, 8, 1, 5, 0x2a1e18); fill(x + 3, 8, 1, 5, 0x2a1e18);
      dot(x - 1, 11, 0x2a1e24); dot(x + 1, 11, 0x2a1e24); dot(x, 13, 0xa83a4a);
      fill(x - 4, 14, 9, 8, 0x14101a); fill(x - 2, 14, 5, 2, 0x2a1e30); dot(x, 16, 0xc8203a);
    }
    const cand = [x - 5, x + 5];
    for (const cx of cand) { fill(cx, 25, 1, 3, 0xf2ead8); dot(cx, 28, 0xd8ccb0); const f = candlelight(cx * 0.3); dot(cx, 24, 0xffd060, f); dot(cx, 23, 0xff8a3a, 0.5 * f); }
    fill(x - 7, 28, 15, 1, 0x3a2a1e);
    if (isNight) glowLate(() => { for (const cx of cand) { glowAt(cx, 24, 4, 4, 0xffb050, 0.18); dot(cx, 24, 0xfff0b0); } if (flash) glowAt(x, 14, 12, 12, 0xd8dcf0, 0.2); });
  },
  /** A tall black wall clock with a cracked face and a swinging pendulum, its hands stuck near midnight; every 30 s a tiny door opens and a bat pops out on a spring, flapping, then ducks back in. */
  halloweendxclock(x) {
    const wood = 0x2a1e24, woodLight = 0x4a3640;
    fill(x - 5, 3, 11, 25, wood); fill(x - 5, 3, 11, 1, woodLight); fill(x - 5, 3, 1, 25, woodLight); for (let r = 0; r < 3; r++) fill(x - 3 + r, 1 + r - 1, 7 - r * 2, 1, wood);
    disc(x, 10, 3.6, 0xe8dcc0); line(x - 2, 8, x + 1, 11, 0x8a7a6a); line(x, 10, x, 7, 0x1e1a1a); line(x, 10, x + 1, 7, 0x1e1a1a);
    fill(x - 3, 15, 7, 11, 0x14101a);
    const sw = [0, 1, 2, 1, 0, -1, -2, -1][Math.floor(now * 4) % 8];
    line(x, 15, x + sw, 22, 0xc8a050); disc(x + sw, 23, 1.4, 0xd8b45a);
    const t = (now + x * 0.3) % 30;
    if (t < 2.5) { fill(x - 1, 4, 3, 2, 0x0a0810); const out = Math.min(3, Math.floor(t * 6)), flap = Math.floor(now * 10) % 2; dot(x, 4 - out, 0x1e1418); dot(x - 1, 4 - out - flap, 0x3a2a40); dot(x + 1, 4 - out - flap, 0x3a2a40); dot(x, 4 - out, 0xff5a3a); } else fill(x - 1, 4, 3, 2, woodLight);
    fill(x - 4, 28, 9, 1, SHADOW);
  },
  /** A garland strung across the wall of cut-paper bats, crescent moons and little pumpkins in black, gold and orange; it sways in a draught now and then, and the bats' eyes glint. */
  halloweendxgarland(x) {
    const t = (now + x * 0.29) % 11, sw = t < 2 ? [0, 1, 0, -1][Math.floor(t * 4) % 4] : 0;
    dot(x - 13, 4, 0x5a3b25); dot(x + 13, 4, 0x5a3b25);
    for (let k = 0; k <= 26; k++) dot(x - 13 + k, 4 + Math.round(Math.sin((k / 26) * Math.PI) * 5), 0x8a7a5a);
    for (let k = 0; k < 7; k++) {
      const gx = x - 11 + k * 4 + (k % 2 ? sw : -sw), gy = 5 + Math.round(Math.sin(((k * 4 + 2) / 26) * Math.PI) * 5);
      if (k % 3 === 0) { fill(gx - 1, gy, 3, 2, 0x1e1418); dot(gx - 2, gy - 1, 0x1e1418); dot(gx + 2, gy - 1, 0x1e1418); dot(gx - 3, gy, 0x1e1418); dot(gx + 3, gy, 0x1e1418); dot(gx, gy + 2, 0x1e1418); dot(gx, gy, (now + k) % 5 < 0.3 ? 0xffd080 : 0xc83a2a); }
      else if (k % 3 === 1) { fill(gx - 1, gy, 2, 4, 0xe8c060); dot(gx + 1, gy, 0xe8c060); dot(gx + 1, gy + 3, 0xe8c060); }
      else { fill(gx - 1, gy + 1, 4, 3, 0xe8782e); dot(gx, gy, 0x4a7a3a); dot(gx, gy + 2, 0x1e1418); dot(gx + 2, gy + 2, 0x1e1418); }
    }
  },
  /** A crooked shelf of spell books bound in purple, green and oxblood with a skull on top of them holding a dripping candle, a jar of green glowing liquid and a dried bundle of herbs; the candle flickers and the jar glows at night. */
  halloweendxshelf(x) {
    const wood = 0x3a2a24, woodLight = 0x5a4434;
    line(x - 11, 22, x + 12, 21, woodLight); fill(x - 11, 23, 24, 1, wood); for (const bx of [x - 9, x + 9]) fill(bx, 24, 2, 3, wood);
    [[-11, 9, 0x5a2a6a], [-8, 8, 0x2a5a3a], [-5, 10, 0x6a1e24], [-2, 7, 0x3a3a6a]].forEach(([bx, h, c]) => { fill(x + bx, 22 - h, 3, h, c); fill(x + bx, 22 - h, 1, h, shaded(c, 1.3)); dot(x + bx + 1, 20 - h + 4, 0xd8b45a); });
    fill(x + 3, 15, 5, 5, 0xe8dcc0); fill(x + 3, 20, 4, 1, 0xd8ccb0); dot(x + 4, 17, 0x1e1a1a); dot(x + 6, 17, 0x1e1a1a); dot(x + 5, 19, 0x1e1a1a);
    fill(x + 5, 11, 1, 4, 0xf2ead8); dot(x + 4, 14, 0xf2ead8); dot(x + 6, 15, 0xf2ead8);
    const f = candlelight(x * 0.27); dot(x + 5, 10, 0xffd060, f); dot(x + 5, 9, 0xff8a3a, 0.5 * f);
    const jar = (a) => { fill(x + 9, 15, 3, 6, 0x7aff4a, 0.6 * a); fill(x + 9, 14, 3, 1, 0x6a5a4a); dot(x + 10, 17, 0xc8ffb0, a); };
    jar(1);
    line(x - 3, 10, x - 1, 4, 0x8a7a4a); line(x - 1, 10, x - 1, 5, 0x6a8a4a); dot(x - 2, 4, 0x6a8a4a);
    fill(x - 10, 27, 21, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x + 5, 10, 4, 4, 0xffb050, 0.18); dot(x + 5, 10, 0xfff0b0); glowAt(x + 10, 17, 3, 4, 0x7aff4a, 0.15); jar(1); });
  },
  /** An oval mirror in a tarnished silver frame reflecting the dim room; every 25 s a pale face rises in the glass, stares, and fades away again, and the glass mists over. Faintly lit at night. */
  halloweendxmirror(x) {
    const silver = 0x8a8e98, silverDark = 0x5a5e68, cy = 14;
    for (let dy = -10; dy <= 10; dy++) for (let dx = -7; dx <= 7; dx++) {
      const r = Math.hypot(dx / 7, dy / 10); if (r > 1.02) continue;
      dot(x + dx, cy + dy, r > 0.82 ? (dx + dy < 0 ? silver : silverDark) : (dy < 0 ? 0x3a3448 : 0x2a2434));
    }
    dot(x, 3, silver); dot(x - 1, 4, silver); dot(x + 1, 4, silver); dot(x - 3, 8, WHITE, 0.4); dot(x - 4, 10, WHITE, 0.3);
    const t = (now + x * 0.3) % 25;
    if (t < 4) {
      const a = Math.min(1, t / 1, (4 - t) / 1.2) * 0.85;
      fill(x - 3, cy - 4, 6, 7, 0xd8e4f0, a); fill(x - 2, cy + 3, 4, 2, 0xd8e4f0, a); fill(x - 2, cy - 2, 1, 2, 0x1a1428, a); fill(x + 1, cy - 2, 1, 2, 0x1a1428, a); fill(x - 1, cy + 1, 2, 2, 0x1a1428, a);
      fill(x - 5, cy - 7, 11, 14, 0xd8e4f0, 0.08 * a);
    }
    fill(x - 6, 26, 13, 1, SHADOW);
    if (isNight) glowLate(() => glowAt(x, cy, 5, 7, 0xa8c8e8, 0.06));
  },
  /** A cat walkway of three little wooden shelves stepping up the wall; a cat sits on the top shelf, and every 20 s one strolls across them, hopping from shelf to shelf. */
  catcafeshelves(x) {
    const wood = 0xc8945a, woodLight = 0xe0b078, woodDark = 0x8a5a32;
    const shelves = [[x - 12, 26], [x - 4, 19], [x + 5, 12]];
    for (const [sx, sy] of shelves) { fill(sx, sy, 8, 2, wood); fill(sx, sy, 8, 1, woodLight); fill(sx + 1, sy + 2, 1, 2, woodDark); fill(sx + 6, sy + 2, 1, 2, woodDark); fill(sx, sy + 2, 8, 1, SHADOW); }
    const t = (now + x * 0.3) % 20, kind = ['catcafecalico', 'catcafeblue', 'catcafetabby'][Math.floor((now + x * 0.3) / 20) % 3];
    if (t < 4) {
      const f = t / 4, seg = Math.min(2, Math.floor(f * 3)), g = f * 3 - seg, [sx, sy] = shelves[seg], ny = shelves[Math.min(2, seg + 1)][1];
      const cx = Math.round(sx + 2 + g * 6), cy = sy - (g > 0.8 && seg < 2 ? Math.round((g - 0.8) * 5 * (sy - ny)) : 0);
      cafeCatAt(cx, cy, kind, 'walk', false, Math.floor(now * 8) % 2);
    } else cafeCatAt(x + 8, 12, kind, (t % 10) < 5 ? 'sit' : 'loaf');
  },
  /** A little gallery of the resident cats in gilt and painted frames, four faces in their colours, one frame hanging crooked; now and then one of the painted cats blinks. */
  catcafeportraits(x) {
    const frames = [[x - 11, 7, 0xc8a050, 'catcafeginger'], [x - 3, 5, 0xf2a0b8, 'catcafecalico'], [x + 5, 8, 0x8ac8b8, 'catcafeblue'], [x - 6, 16, 0xc8a050, 'catcafemaine'], [x + 3, 17, 0xd8c8ec, 'catcafesphynx']];
    const blinker = Math.floor((now + x * 0.2) / 3) % 5, shut = (now + x * 0.2) % 3 < 0.25;
    frames.forEach(([fx, fy, c, kind], k) => {
      const cat = CAFE_CATS[kind];
      fill(fx, fy, 7, 7, c); fill(fx, fy, 7, 1, shaded(c, 1.15)); fill(fx + 1, fy + 1, 5, 5, 0xf6eee0);
      fill(fx + 2, fy + 3, 3, 3, cat.fur); dot(fx + 2, fy + 2, cat.fur); dot(fx + 4, fy + 2, cat.fur);
      if (cat.mark === 'calico') dot(fx + 2, fy + 3, cat.patch); if (cat.tufts) { dot(fx + 2, fy + 1, cat.dark); dot(fx + 4, fy + 1, cat.dark); }
      const sh = shut && k === blinker; dot(fx + 2, fy + 4, sh ? cat.dark : cat.eye); dot(fx + 4, fy + 4, sh ? cat.dark : cat.eye); dot(fx + 3, fy + 5, cat.nose);
      fill(fx + 1, fy + 7, 5, 1, SHADOW);
    });
  },
  /** A sunny window with a cushion on its sill where a cat sits with its back to the room watching the street; its tail hangs over the edge and flicks, and when a bird flits past outside its head turns to follow. A soft night sky after dark. */
  catcafewindow(x) {
    const frame = 0xf6eee0, frameDark = 0xd8ccb8, sky = isNight ? 0x1e2a50 : 0xb8dcf0;
    fill(x - 11, 3, 23, 22, frame); fill(x - 10, 4, 21, 18, sky); fill(x, 4, 1, 18, frame); fill(x - 10, 12, 21, 1, frame);
    if (!isNight) { fill(x - 9, 17, 6, 5, 0x8ac87a); fill(x + 3, 15, 7, 7, 0x7ab86a); } else { dot(x + 6, 7, 0xf4eec8); dot(x - 6, 9, WHITE, 0.6); }
    const t = (now + x * 0.3) % 12, bird = !isNight && t < 2.5, bx = Math.round(x - 10 + (t / 2.5) * 21);
    if (bird) { dot(bx, 8, 0x3a3a40); dot(bx - 1, 7 - (Math.floor(now * 10) % 2), 0x3a3a40); dot(bx + 1, 7 - (Math.floor(now * 10) % 2), 0x3a3a40); }
    fill(x - 12, 22, 25, 3, frameDark); fill(x - 12, 22, 25, 1, frame); fill(x - 6, 20, 12, 2, 0xf2a0b8);
    // The cat from behind
    const c = CAFE_CATS.catcafeginger, look = bird ? Math.sign(bx - x) : 0;
    fill(x - 3, 13, 7, 7, c.fur); fill(x - 2, 12, 5, 1, c.fur); for (const k of [-2, 0, 2]) fill(x + k, 13, 1, 4, c.dark);
    fill(x - 2 + look, 8, 5, 4, c.fur); dot(x - 2 + look, 7, c.fur); dot(x + 2 + look, 7, c.fur); dot(x + look, 8, c.dark);
    const flick = Math.floor(now * 1.5 + x) % 3;
    fill(x + 3, 20, 1, 5, c.fur); dot(x + 3 + (flick === 0 ? 1 : 0), 25, c.dark); dot(x + 3, 23, c.dark);
    fill(x - 11, 25, 23, 1, SHADOW);
  },
  /** Bunting across the wall of little pastel pennants, alternating a fish, a paw print and a heart; it sways in a draught now and then. */
  catcafebunting(x) {
    const cols = [0xf2a0b8, 0x8ac8b8, 0xf6d878, 0xc8b0e8];
    const t = (now + x * 0.27) % 12, sw = t < 2 ? [0, 1, 0, -1][Math.floor(t * 4) % 4] : 0;
    dot(x - 13, 4, 0xc8945a); dot(x + 13, 4, 0xc8945a);
    for (let k = 0; k <= 26; k++) dot(x - 13 + k, 4 + Math.round(Math.sin((k / 26) * Math.PI) * 4), 0xa8987a);
    for (let k = 0; k < 6; k++) {
      const px = x - 11 + k * 4 + (k % 2 ? sw : 0), py = 5 + Math.round(Math.sin(((k * 4 + 2) / 26) * Math.PI) * 4), c = cols[k % 4];
      for (let r = 0; r < 5; r++) fill(px - Math.floor((4 - r) / 2) + 1 - 1, py + r, Math.max(1, 4 - r), 1, c);
      const m = k % 3; if (m === 0) { dot(px, py + 1, WHITE); dot(px + 1, py + 1, WHITE); } else if (m === 1) { dot(px, py + 1, WHITE); dot(px + 1, py + 2, WHITE, 0.8); } else { dot(px, py + 1, 0xd8302a, 0.8); dot(px + 1, py + 1, 0xd8302a, 0.8); }
    }
  },
  /** A wicker basket of yarn balls on a little wall shelf, pink, mint, butter and lilac, a loose thread dangling; every 12 s a kitten's paw pokes out from among them and bats at the thread. */
  catcafeyarn(x) {
    fill(x - 9, 22, 19, 2, 0xc8945a); fill(x - 9, 22, 19, 1, 0xe0b078); for (const bx of [x - 7, x + 7]) fill(bx, 24, 1, 2, 0x8a5a32);
    fill(x - 8, 16, 17, 6, 0xb88a52); for (let k = 0; k < 17; k += 2) fill(x - 8 + k, 16, 1, 6, 0x9a7040, 0.6); fill(x - 8, 16, 17, 1, 0xd8a868);
    [[x - 5, 13, 0xf2a0b8], [x, 12, 0x8ac8b8], [x + 5, 13, 0xf6d878], [x + 2, 15, 0xc8b0e8]].forEach(([bx, by, c]) => { disc(bx, by, 2.6, c); line(bx - 2, by - 1, bx + 2, by + 1, shaded(c, 0.8)); dot(bx - 1, by - 2, WHITE, 0.6); });
    const t = (now + x * 0.3) % 12, bat = t < 2 ? Math.round(Math.sin(t * 9) * 2) : 0;
    line(x + 8, 17, x + 9 + bat, 26, 0xf2a0b8);
    if (t < 2) { const c = CAFE_CATS.catcafekitten; fill(x + 6, 18 + Math.abs(bat), 2, 3, c.fur); dot(x + 7 + bat, 21, c.light); }
    fill(x - 8, 26, 17, 1, SHADOW);
  },
  /** A macramé hanger holding a pot of bright cat grass whose blades stir; every 16 s a cat's paw reaches up from below to pull a blade down. */
  catcafeplanter(x) {
    const cord = 0xe8dcc4;
    line(x, 1, x - 4, 12, cord); line(x, 1, x + 4, 12, cord); line(x, 1, x, 12, cord); for (const k of [-3, 3]) dot(x + k, 9, 0xc8b898);
    fill(x - 5, 12, 11, 7, 0xe8a070); fill(x - 5, 12, 11, 1, 0xf2c090); fill(x + 4, 13, 1, 6, 0xc87a50);
    const t = (now + x * 0.3) % 16, pull = t < 2;
    for (let k = -4; k <= 4; k++) { const sway = Math.round(Math.sin(now * 2 + k) * 0.6) + (pull && k === 3 ? 2 : 0); line(x + k, 12, x + k + sway, 6 + Math.abs(k) % 3 + (pull && k === 3 ? 5 : 0), k % 2 ? 0x6ac84a : 0x8ad85a); }
    if (pull) { const c = CAFE_CATS.catcafetortie, up = Math.min(5, Math.floor(t * 6)); fill(x + 4, 24 - up, 2, up + 1, c.fur); dot(x + 5, 23 - up, c.dark); }
    fill(x - 4, 26, 9, 1, SHADOW);
  },
  /** A Brooklyn window: brick-red and violet rooftops with water towers against a halftone sky, lit windows after dark; every so often a little Spider-Man swings past on his web. */
  spiderversewindow(x) {
    const frame = 0x2a2438, frameLight = 0x3e3650, sky = isNight ? 0x2a1e4a : 0x8fc0e8, low = isNight ? 0x6a2a6a : 0xf2b8c8;
    fill(x - 12, 5, 25, 23, frame); fill(x - 12, 5, 25, 1, frameLight);
    fill(x - 11, 6, 23, 11, sky); fill(x - 11, 17, 23, 9, low);
    for (let yy = 7; yy < 16; yy += 3) for (let xx = x - 10 + (yy % 2); xx < x + 12; xx += 3) dot(xx, yy, WHITE, isNight ? 0.08 : 0.2);
    // The rooftops, their windows, a water tower
    const blocks = [[-11, 15, 5, 0x6a3a4a], [-6, 12, 4, 0x4a3a5a], [-2, 16, 5, 0x7a4040], [3, 11, 4, 0x3a3050], [7, 14, 5, 0x5a3a50]];
    for (const [dx, top, w, c] of blocks) {
      fill(x + dx, top, w, 26 - top, c); fill(x + dx, top, w, 1, shaded(c, 1.2));
      for (let wy = top + 2; wy < 25; wy += 3) for (let wx = 1; wx < w - 1; wx += 2) {
        const lit = isNight ? hash(x + dx + wx, wy) % 3 !== 0 : false;
        dot(x + dx + wx, wy, lit ? 0xffd870 : shaded(c, 0.75));
      }
    }
    fill(x - 5, 8, 3, 3, 0x5a3b25); dot(x - 5, 11, 0x3a2418); dot(x - 3, 11, 0x3a2418); fill(x - 6, 7, 5, 1, 0x3a2418);
    // Spider-Man, swinging past now and then
    const t = (now + x * 0.37) % 24;
    if (t < 1.6) {
      const k = Math.floor(t * 10) / 16, sx = Math.round(x - 10 + k * 21), sy = Math.round(9 + Math.sin(k * Math.PI) * 5);
      line(x + 1, 6, sx, sy, WHITE, 0.8); fill(sx - 1, sy, 2, 2, 0xd8202a); dot(sx - 1, sy, WHITE); fill(sx - 1, sy + 2, 2, 2, 0x2a4ab0); dot(sx, sy + 4, 0xd8202a);
    }
    fill(x, 6, 1, 20, frame); fill(x - 11, 16, 23, 1, frame);
    fill(x - 13, 26, 27, 2, 0x5a4e6a); fill(x - 13, 26, 27, 1, 0x6a5e7a);
    if (isNight) glowLate(() => { for (const [dx, top] of blocks) dot(x + dx + 1, top + 2, 0xffe08a); glowAt(x, 18, 10, 6, 0xffb060, 0.06); });
  },
  /** A comic panel in a white border: a yellow starburst on halftone pink with a red-gloved hand shooting a web through it; every so often the burst pops bigger and the web shoots out. */
  spiderversecomic(x) {
    const ink = 0x1c1a22, yellow = 0xf2d35b, red = 0xd8202a;
    fill(x - 10, 5, 21, 22, WHITE); fill(x - 9, 6, 19, 20, ink); fill(x - 8, 7, 17, 18, 0xf28fb0);
    for (let yy = 8; yy < 25; yy += 2) for (let xx = x - 8 + (yy % 4 === 0 ? 1 : 0); xx < x + 9; xx += 2) dot(xx, yy, 0xe06a98);
    const t = (now + x * 0.29) % 15, pop = t < 1.2 ? 1 : 0;
    // The burst
    const r = 6 + pop;
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2, rr = k % 2 ? r : r - 3; line(x + 1, 13, x + 1 + Math.round(Math.cos(a) * rr), 13 + Math.round(Math.sin(a) * rr), yellow); }
    disc(x + 1, 13, 3.5, yellow); fill(x - 2, 12, 7, 1, ink); dot(x - 2, 13, ink); dot(x + 1, 14, ink); dot(x + 4, 14, ink);
    // The hand and its web
    fill(x - 7, 21, 4, 3, red); fill(x - 8, 22, 1, 2, red); dot(x - 4, 20, red); dot(x - 6, 21, ink, 0.6);
    const reach = t < 1.2 ? Math.floor(t * 10) : 12;
    line(x - 3, 20, x - 3 + Math.min(reach, 11), 20 - Math.min(reach, 11), WHITE, 0.9);
    fill(x - 10, 27, 21, 1, SHADOW);
  },
  /** A glitch portal: a jagged rectangular tear in the wall showing another dimension's swirl in magenta, cyan and yellow, its edges split into red and cyan; now and then it flickers into misplaced slices. It glows at night. */
  spiderverseportal(x) {
    const cy = 16, mag = 0xff3aa0, cyan = 0x30e0f0, yellow = 0xf2d35b;
    const edge = (dx, c, a) => { fill(x - 8 + dx, 7, 17, 18, c, a); };
    edge(-1, 0xff3a6a, 0.35); edge(1, cyan, 0.35);
    fill(x - 8, 7, 17, 18, 0x1a1030);
    // The swirl: rings stepping round
    const turn = Math.floor(now * 3) % 8;
    for (let k = 0; k < 4; k++) ellipseRing(x, cy, 7 - k * 1.6, 7.5 - k * 1.6, [mag, cyan, yellow, mag][k], 0.7);
    for (let k = 0; k < 6; k++) { const a = ((k + turn / 8) / 6) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 5), cy + Math.round(Math.sin(a) * 5), WHITE, 0.8); }
    disc(x, cy, 1.5, WHITE, 0.9);
    // The jagged edge
    for (let k = 0; k < 17; k += 2) { dot(x - 8 + k, 7, 0x2a2438); dot(x - 7 + k, 24, 0x2a2438); }
    for (let k = 0; k < 18; k += 3) { dot(x - 8, 7 + k, 0x2a2438); dot(x + 8, 8 + k, 0x2a2438); }
    const t = (now + x * 0.41) % 17;
    if (t < 0.6) for (let s = 0; s < 3; s++) { const sy = 9 + ((Math.floor(now * 20) + s * 5) % 14); fill(x - 8 + (s % 2 ? 2 : -2), sy, 17, 2, [mag, cyan, yellow][s], 0.7); }
    glowAt(x, cy, 10, 10, mag, isNight ? 0.12 : 0.05);
    if (isNight) glowLate(() => { glowAt(x, cy, 9, 9, 0xc040ff, 0.16); disc(x, cy, 1.5, WHITE, 0.9); });
  },
  /** A rack of four masks on pegs along a plank: Miles's black one with red webbing, Gwen's white hood, Peter B.'s red one and a little pig's; now and then one mask's eyes narrow, as a mask's eyes do in the comics. */
  spiderversemasks(x) {
    const plank = 0x8a5a32, plankLight = 0xa8703e, ink = 0x1c1a22;
    fill(x - 13, 8, 26, 2, plank); fill(x - 13, 8, 26, 1, plankLight); fill(x - 13, 10, 26, 1, 0x5a3b25, 0.6);
    const masks = [[-9, 0x1c1a22, 0xd8202a], [-3, 0xf4f2ee, 0xf28fb0], [3, 0xd8202a, ink], [9, 0xf2a0b0, 0xd8202a]];
    const t = (now + x * 0.33) % 18, sq = t < 1.4 ? Math.floor((now + x) / 18) % 4 : -1;
    masks.forEach(([dx, c, web], i) => {
      const mx = x + dx;
      dot(mx, 10, 0x5a3b25);
      fill(mx - 2, 11, 5, 6, c); fill(mx - 1, 17, 3, 1, c); fill(mx - 2, 11, 1, 6, shaded(c, 1.12)); fill(mx + 2, 11, 1, 6, shaded(c, 0.85));
      if (i === 1) { fill(mx - 3, 11, 1, 6, 0xf4f2ee); fill(mx + 3, 11, 1, 6, 0xf4f2ee); dot(mx - 2, 11, 0xf28fb0); dot(mx + 2, 11, 0xf28fb0); }
      else if (i === 3) { fill(mx - 1, 15, 3, 2, 0xe07a90); dot(mx - 1, 15, 0x8a3a4a); dot(mx + 1, 15, 0x8a3a4a); dot(mx - 3, 11, 0xf2a0b0); dot(mx + 3, 11, 0xf2a0b0); }
      else { dot(mx, 12, web, 0.7); dot(mx, 14, web, 0.7); dot(mx - 1, 16, web, 0.7); dot(mx + 1, 16, web, 0.7); }
      const eye = sq === i ? 1 : 2, ey = 13;
      fill(mx - 2, ey, 2, eye, WHITE); fill(mx + 1, ey, 2, eye, WHITE);
    });
    fill(x - 12, 18, 25, 1, SHADOW);
  },
  /** The Spot's sheet: a white lab sheet pinned to the wall, covered in black spots; every 20 s or so one spot opens into a hole and a red-gloved hand reaches out of it, feels about and goes back. */
  spiderversespot(x) {
    const sheet = 0xf4f2ee, sheetDark = 0xd8d4cc, spot = 0x14121a;
    fill(x - 9, 5, 19, 22, sheet); fill(x - 9, 5, 19, 1, WHITE); fill(x + 9, 5, 1, 22, sheetDark); fill(x - 9, 26, 19, 1, sheetDark);
    dot(x - 8, 6, 0xd8202a); dot(x + 8, 6, 0xd8202a);
    const spots = [[-5, 10, 2.2], [3, 9, 1.6], [-1, 16, 2.6], [5, 18, 1.8], [-6, 22, 1.4], [2, 23, 1.2]];
    for (const [dx, sy, r] of spots) disc(x + dx, sy, r, spot);
    const t = (now + x * 0.27) % 20;
    if (t < 2.5) {
      const out = Math.min(3, Math.floor(t * 4)) - (t > 2 ? Math.floor((t - 2) * 6) : 0);
      disc(x - 1, 16, 2.6, 0x3a2a6a); dot(x - 1, 16, 0xa080ff);
      if (out > 0) { fill(x - 1, 16 - out, 2, out, 0xd8202a); dot(x - 2, 16 - out, 0xd8202a); dot(x + 1, 15 - out, 0xd8202a); }
    }
    fill(x - 8, 27, 18, 1, SHADOW);
  },
  /** A school pennant in navy and gold for the academy Miles goes to, hung from a tack, a little spider sticker in its corner; now and then a draught lifts its tip. */
  spiderversepennant(x) {
    const navy = 0x1e2e5a, navyLight = 0x2e4278, gold = 0xe8b84a;
    dot(x - 8, 8, 0xc8a050);
    const t = (now + x * 0.3) % 13, lift = t < 1.5 ? Math.floor(t * 4) % 2 : 0;
    for (let k = 0; k < 16; k++) { const h = Math.max(1, Math.round(9 - k * 0.55)), top = 9 + Math.round(k * 0.28) - (k > 12 ? lift : 0); fill(x - 8 + k, top, 1, h, k === 0 ? navyLight : navy); }
    fill(x - 8, 9, 16, 1, gold); for (let k = 0; k < 8; k++) dot(x - 5 + k, 13, gold);
    dot(x - 6, 15, gold); dot(x - 5, 14, gold);
    fill(x - 7, 10, 2, 2, WHITE); dot(x - 7, 10, 0xd8202a);
    fill(x - 7, 19, 14, 1, SHADOW);
  },
  /** The X emblem: a big black disc ringed in red with the yellow X across it, on a steel backing plate; every 15 s a gleam sweeps across it. It glows a little red at night. */
  xmenlogo(x) {
    fill(x - 10, 5, 21, 22, 0x4a5260); fill(x - 10, 5, 21, 1, 0x7a8290); fill(x + 10, 5, 1, 22, 0x2e3440); fill(x - 10, 26, 21, 1, 0x2e3440);
    for (const [dx, dy] of [[-9, 6], [9, 6], [-9, 25], [9, 25]]) dot(x + dx, dy, 0xa8adb6);
    xmenX(x, 16, 8.5); ring(x, 16, 9.5, 0xd8202a, 0.6);
    const t = (now + x * 0.3) % 15;
    if (t < 1) { const k = Math.floor(t * 14) - 7; line(x + k - 2, 9, x + k + 3, 23, WHITE, 0.45); }
    if (isNight) glowLate(() => glowAt(x, 16, 9, 9, 0xff3030, 0.08));
  },
  /** A window onto the mansion's grounds, lawn and dark trees under the sky (stars at night); now and then the Blackbird, the team's black jet, streaks across. */
  xmenblackbird(x) {
    const frame = 0xe8e2d4, frameDark = 0xb8b2a4, sky = isNight ? 0x101a3a : 0x8fc0e8;
    fill(x - 12, 5, 25, 22, frame); fill(x - 11, 6, 23, 20, sky);
    if (isNight) for (let k = 0; k < 6; k++) dot(x - 10 + ((k * 7) % 22), 7 + ((k * 5) % 8), WHITE, 0.7);
    else { fill(x + 3, 8, 5, 2, WHITE, 0.8); fill(x + 4, 7, 3, 1, WHITE, 0.8); }
    fill(x - 11, 19, 23, 7, isNight ? 0x1a3a22 : 0x4a8a4a); fill(x - 11, 19, 23, 1, isNight ? 0x2a4a2a : 0x5a9a5a);
    for (const [dx, h] of [[-9, 6], [-5, 4], [6, 7], [9, 5]]) { fill(x + dx - 1, 19 - h, 3, h, isNight ? 0x0e2216 : 0x2e5a32); dot(x + dx, 19 - h - 1, isNight ? 0x0e2216 : 0x2e5a32); }
    fill(x - 4, 14, 7, 5, isNight ? 0x3a3448 : 0x8a7a6a); fill(x - 5, 13, 9, 1, isNight ? 0x2a2438 : 0x5a4a4a); dot(x - 2, 16, isNight ? 0xffd870 : 0x3a3a48); dot(x + 1, 16, isNight ? 0xffd870 : 0x3a3a48);
    const t = (now + x * 0.37) % 26;
    if (t < 1.4) {
      const jx = Math.round(x - 14 + t * 22), jy = 10 - Math.floor(t * 2);
      if (jx > x - 12 && jx < x + 10) { fill(jx, jy, 5, 1, 0x1c1a22); fill(jx + 1, jy - 1, 2, 1, 0x1c1a22); fill(jx + 1, jy + 1, 3, 1, 0x2a2a30); dot(jx + 5, jy, 0x3a3a48); dot(jx - 1, jy, 0xffa040); }
    }
    fill(x, 6, 1, 20, frame); fill(x - 11, 15, 23, 1, frame);
    fill(x - 13, 26, 27, 2, frameDark); fill(x - 13, 26, 27, 1, frame);
  },
  /** The team roster: a corkboard of six little portraits pinned in two rows: Cyclops's visor, Wolverine's mask, Storm's white hair, Jean's red hair, Rogue's white streak and Gambit's glowing eyes; Cyclops's visor flares now and then. */
  xmenroster(x) {
    fill(x - 13, 5, 26, 21, 0x8a5a32); fill(x - 12, 6, 24, 19, 0xc89a6a); for (let k = 0; k < 8; k++) dot(x - 11 + ((k * 7) % 22), 7 + ((k * 5) % 17), 0xa87a4a);
    const faces = [
      [0x2a3a8a, 0xf2c8a0, (fx, fy) => { fill(fx, fy + 2, 6, 1, 0xd8202a); }],
      [0xf2c41a, 0xf2c41a, (fx, fy) => { dot(fx, fy - 1, 0x1c1a22); dot(fx + 5, fy - 1, 0x1c1a22); fill(fx + 1, fy + 2, 4, 1, 0x1c1a22); dot(fx + 1, fy + 2, WHITE); dot(fx + 4, fy + 2, WHITE); }],
      [0xf4f2ee, 0x7a4a2a, (fx, fy) => { dot(fx + 1, fy + 2, WHITE); dot(fx + 4, fy + 2, WHITE); }],
      [0xd84a2a, 0xf2c8a0, (fx, fy) => { dot(fx + 1, fy + 2, 0x3a8a4a); dot(fx + 4, fy + 2, 0x3a8a4a); }],
      [0x6a3a24, 0xf2c8a0, (fx, fy) => { fill(fx + 2, fy, 2, 2, 0xf4f2ee); dot(fx + 1, fy + 2, 0x3a8a4a); dot(fx + 4, fy + 2, 0x3a8a4a); }],
      [0x5a3a2a, 0xe8c09a, (fx, fy) => { dot(fx + 1, fy + 2, 0xff3aa0); dot(fx + 4, fy + 2, 0xff3aa0); }],
    ];
    const t = (now + x * 0.29) % 20;
    faces.forEach(([hair, skin, feat], i) => {
      const fx = x - 11 + (i % 3) * 8, fy = 8 + Math.floor(i / 3) * 9;
      fill(fx - 1, fy - 1, 8, 8, 0xf4f2ee); fill(fx, fy, 6, 6, 0x8fb0d0);
      fill(fx + 1, fy + 1, 4, 5, skin); fill(fx, fy, 6, 2, hair); fill(fx, fy + 1, 1, 4, hair); fill(fx + 5, fy + 1, 1, 4, hair);
      feat(fx, fy);
      dot(fx + 2, fy - 1, [0xd8202a, 0x2a4ab0, 0xf2c41a][i % 3]);
    });
    if (t < 1.2) { fill(x - 11, 10, 6, 1, 0xff6050); line(x - 5, 10, x - 2 + Math.floor(t * 8), 10, 0xff3030, 0.6); }
    fill(x - 12, 26, 25, 1, SHADOW);
  },
  /** The school's crest on a dark wooden plaque: a gold shield with an X, laurels either side and a brass nameplate below (no lettering); a gleam crosses the shield now and then. */
  xmenschool(x) {
    const wood = 0x4a2a1a, woodLight = 0x6a3a26, gold = 0xd8b45a, goldLight = 0xf2d888, goldDark = 0x9a7a30;
    fill(x - 8, 5, 17, 21, wood); fill(x - 8, 5, 17, 1, woodLight); fill(x - 8, 5, 1, 21, woodLight); fill(x + 8, 5, 1, 21, 0x2e1a10);
    fill(x - 4, 8, 9, 8, gold); fill(x - 3, 16, 7, 1, gold); fill(x - 2, 17, 5, 1, gold); dot(x, 18, gold); fill(x - 4, 8, 9, 1, goldLight); fill(x + 4, 9, 1, 7, goldDark);
    line(x - 2, 10, x + 2, 15, 0x1c1a22); line(x + 2, 10, x - 2, 15, 0x1c1a22);
    for (let k = 0; k < 5; k++) { dot(x - 6, 9 + k * 2, 0x6a8a3a); dot(x - 7, 10 + k * 2, 0x6a8a3a); dot(x + 6, 9 + k * 2, 0x6a8a3a); dot(x + 7, 10 + k * 2, 0x6a8a3a); }
    fill(x - 5, 21, 11, 3, gold); fill(x - 5, 21, 11, 1, goldLight); for (let k = 0; k < 4; k++) dot(x - 3 + k * 2, 22, goldDark);
    const t = (now + x * 0.31) % 17;
    if (t < 0.8) { const k = Math.floor(t * 12); dot(x - 4 + k, 8 + k, WHITE, 0.7); dot(x - 3 + k, 8 + k, WHITE, 0.4); }
    fill(x - 7, 26, 16, 1, SHADOW);
  },
  /** The Danger Room's alert: a red beacon in a wire cage over a hazard-striped plate; every 25 s or so a session starts and it spins and flashes red for a few seconds, through the night too. */
  xmenalert(x) {
    fill(x - 6, 12, 13, 12, 0x3a4250); fill(x - 6, 12, 13, 1, 0x5a6270);
    for (let k = 0; k < 11; k += 4) { line(x - 5 + k, 22, x - 3 + k, 18, 0xf2c41a); line(x - 4 + k, 22, x - 2 + k, 18, 0xf2c41a); }
    fill(x - 5, 18, 11, 1, 0x1c1a22, 0.4);
    const t = (now + x * 0.23) % 25, on = t < 3.5, turn = Math.floor(now * 8) % 4;
    fill(x - 3, 7, 7, 5, on ? 0xff3a2a : 0x8a2020); fill(x - 2, 6, 5, 1, on ? 0xff6a50 : 0xa83030);
    for (const k of [-3, 0, 3]) fill(x + k, 6, 1, 6, 0x6e727c);
    if (on) {
      dot(x - 2 + turn, 8, WHITE, 0.8);
      glowAt(x, 9, 9, 6, 0xff2020, 0.18);
      const beam = turn % 2 ? -1 : 1;
      line(x + beam * 4, 9, x + beam * 9, 7 + turn, 0xff4030, 0.35);
    }
    fill(x - 5, 24, 11, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x, 9, on ? 10 : 4, on ? 7 : 3, 0xff2020, on ? 0.3 : 0.1); if (on) dot(x - 2 + turn, 8, 0xffc0b0); });
  },
  /** A spare Cerebro helmet on a wall bracket: a domed steel cap trailing cables down into a junction box, a row of blue lights along its rim that pulse slowly. They glow at night. */
  xmenhelmet(x) {
    const steel = 0xa8adb6, steelMid = 0x8a8e98, steelDark = 0x5a5e68;
    fill(x - 2, 5, 5, 3, 0x3a4250);
    fill(x - 6, 9, 13, 2, steel); fill(x - 7, 11, 15, 5, steel); fill(x - 7, 11, 1, 5, 0xd0d4dc); fill(x + 7, 11, 1, 5, steelDark); fill(x - 7, 15, 15, 1, steelDark);
    fill(x - 4, 8, 9, 1, steelMid); dot(x - 3, 9, WHITE, 0.6);
    const pulse = Math.floor(now * 2) % 6;
    for (let k = 0; k < 6; k++) dot(x - 5 + k * 2, 14, k === pulse ? 0x9ad8ff : 0x3a7ad0);
    line(x - 4, 16, x - 6, 23, 0x2a2a30); line(x, 16, x - 1, 23, 0x2a2a30); line(x + 4, 16, x + 5, 23, 0x2a2a30);
    fill(x - 8, 23, 16, 4, 0x3a4250); fill(x - 8, 23, 16, 1, 0x5a6270); dot(x + 5, 25, 0x5ad86a);
    fill(x - 7, 27, 15, 1, SHADOW);
    if (isNight) glowLate(() => { for (let k = 0; k < 6; k++) dot(x - 5 + k * 2, 14, k === pulse ? 0xc0ecff : 0x5a9ae8); glowAt(x, 14, 8, 3, 0x4a90ff, 0.12); });
  },
  /** The start lights: a black gantry with five pairs of red lights; about every 20 s they light up one pair at a time, hold, and all go out together: lights out and away. They shine through the night. */
  f1lights(x) {
    const body = 0x1c1a22;
    fill(x - 13, 5, 27, 2, 0x4a4e58); fill(x - 1, 2, 3, 3, 0x4a4e58);
    fill(x - 13, 7, 27, 12, body); fill(x - 13, 7, 27, 1, 0x2e2a36);
    const t = (now + x * 0.17) % 20, lit = t < 5 ? Math.min(5, Math.floor(t / 0.9)) : 0;
    const lamps = (a) => { for (let k = 0; k < 5; k++) for (const ly of [10, 14]) { const on = k < lit; fill(x - 11 + k * 5, ly, 3, 3, on ? 0xff2a1a : 0x3a1414, a); if (on) dot(x - 11 + k * 5, ly, 0xffa090, a); } };
    lamps(1);
    fill(x - 12, 19, 25, 1, SHADOW);
    if (lit) glowAt(x, 13, 14, 6, 0xff2010, 0.06 * lit);
    if (isNight && lit) glowLate(() => { glowAt(x, 13, 14, 6, 0xff2010, 0.05 * lit); lamps(0.9); });
  },
  /** A wall rack of three tyres side on, their bands in red, yellow and white for soft, medium and hard; a glint runs round the soft now and then. */
  f1tyres(x) {
    fill(x - 13, 24, 26, 2, 0x6e727c); fill(x - 13, 24, 26, 1, 0xa8adb6); fill(x - 12, 26, 2, 2, 0x4a4e58); fill(x + 10, 26, 2, 2, 0x4a4e58);
    [[-8, 0xd81a1a], [0, 0xf2c41a], [8, 0xf4f4f4]].forEach(([dx, band]) => {
      const cx = x + dx, cy = 17;
      disc(cx, cy, 6.5, 0x1c1c20); ring(cx, cy, 4.5, band); disc(cx, cy, 2.5, 0x6e727c); dot(cx, cy, 0x2a2a30); dot(cx - 4, cy - 4, 0x3a3a40);
    });
    const t = (now + x * 0.21) % 11;
    if (t < 0.8) { const a = t * 8; dot(x - 8 + Math.round(Math.cos(a) * 5), 17 + Math.round(Math.sin(a) * 5), WHITE, 0.8); }
  },
  /** A glass display case with a yellow helmet striped in green and blue, its visor dark; a glint runs down the glass now and then. */
  f1helmetcase(x) {
    fill(x - 7, 24, 15, 3, 0x2a2a30); fill(x - 7, 24, 15, 1, 0x4a4e58);
    fill(x - 4, 12, 9, 8, 0xf2d020); fill(x - 3, 11, 7, 1, 0xf2d020); fill(x - 4, 20, 9, 2, 0xf2d020);
    fill(x - 4, 14, 9, 1, 0x2a8a3a); fill(x - 4, 15, 9, 1, 0x2a5ab0); fill(x - 1, 16, 6, 3, 0x1c1a22); dot(x + 3, 16, 0x5a6a8a);
    fill(x - 4, 12, 1, 9, 0xf8e870);
    fill(x - 7, 7, 15, 17, 0xdff0f6, 0.3); fill(x - 7, 7, 15, 1, 0xdff0f6, 0.7); fill(x - 7, 7, 1, 17, 0xdff0f6, 0.6); fill(x + 7, 7, 1, 17, 0xdff0f6, 0.6);
    const t = (now + x * 0.29) % 13;
    dot(x - 5, 9 + (t < 1 ? Math.floor(t * 12) : 0), WHITE, 0.7);
    fill(x - 6, 27, 13, 1, SHADOW);
  },
  /** The timing tower: a tall black board of six rows, each a position marker, a team's colour and a driver's short name in white dots; every 12 s two of them swap places and one flashes purple for the fastest lap. */
  f1timing(x) {
    fill(x - 7, 3, 15, 25, 0x14141a); fill(x - 7, 3, 15, 1, 0x2a2a30); fill(x - 6, 4, 13, 2, 0xd81a1a);
    const teams = [0xd81a1a, 0x00a19c, 0xff8000, 0x1e2a8a, 0x2a8a5a, 0xf4f4f4];
    const t = (now + x * 0.23) % 12, swap = Math.floor((now + x * 0.23) / 12) % 2;
    for (let r = 0; r < 6; r++) {
      const k = r === 1 || r === 2 ? (swap ? 3 - r : r) : r, ry = 7 + r * 3;
      dot(x - 6, ry, 0xa8adb6); fill(x - 4, ry, 1, 2, teams[k]);
      for (let j = 0; j < 3; j++) dot(x - 2 + j * 2, ry, WHITE, 0.8);
      if (t < 1.5 && r === 1) fill(x - 3, ry, 9, 2, 0xa040ff, 0.5);
    }
    fill(x - 6, 27, 13, 1, SHADOW);
  },
  /** A framed circuit map: a winding track drawn in black on white with a red start line, a little red dot lapping it slowly. */
  f1circuit(x) {
    fill(x - 11, 5, 23, 20, 0x2a2a30); fill(x - 10, 6, 21, 18, 0xf4f4f4);
    const path = [[-7, 20], [-3, 20], [2, 20], [6, 19], [8, 16], [7, 13], [4, 12], [3, 9], [0, 8], [-3, 9], [-4, 12], [-7, 13], [-8, 16]];
    for (let k = 0; k < path.length; k++) { const [ax, ay] = path[k], [bx, by] = path[(k + 1) % path.length]; line(x + ax, ay, x + bx, by, 0x1c1a22); }
    fill(x - 1, 19, 1, 3, 0xd81a1a);
    const pos = (now * 1.5 + x * 0.3) % path.length, i = Math.floor(pos), f = pos - i, [ax, ay] = path[i], [bx, by] = path[(i + 1) % path.length];
    dot(x + Math.round(ax + (bx - ax) * f), Math.round(ay + (by - ay) * f), 0xff2a1a);
    fill(x - 10, 25, 22, 1, SHADOW);
  },
  /** A trophy shelf: a tall silver cup, a gold one and a small one on a plank, a champagne bottle beside them; a sparkle hops between them now and then. */
  f1trophies(x) {
    fill(x - 13, 22, 26, 2, 0x8a5a32); fill(x - 13, 22, 26, 1, 0xa8703e); fill(x - 12, 24, 2, 2, 0x5a3b25); fill(x + 10, 24, 2, 2, 0x5a3b25);
    const silver = 0xc8ccd4, gold = 0xd8b45a;
    fill(x - 10, 19, 5, 3, 0x2a2a30); fill(x - 9, 14, 3, 5, silver); fill(x - 11, 8, 7, 6, silver); fill(x - 11, 8, 1, 6, WHITE, 0.6); dot(x - 12, 9, silver); dot(x - 4, 9, silver);
    fill(x - 2, 19, 5, 3, 0x2a2a30); fill(x - 1, 15, 3, 4, gold); fill(x - 3, 10, 7, 5, gold); fill(x - 3, 10, 1, 5, 0xf2d888); dot(x - 4, 11, gold); dot(x + 4, 11, gold);
    fill(x + 5, 19, 3, 3, 0x2a2a30); fill(x + 5, 15, 3, 4, silver);
    fill(x + 9, 14, 3, 8, 0x1a4a2a); fill(x + 10, 11, 1, 3, 0x1a4a2a); fill(x + 9, 16, 3, 2, gold); dot(x + 10, 10, gold);
    const t = (now + x * 0.27) % 9, k = Math.floor((now + x) / 9) % 3;
    if (t < 0.6) dot([x - 9, x, x + 6][k], [8, 10, 15][k], WHITE);
  },
  /** A length of the octagon's fence: black padded posts either side of a chain-link panel, a red pad band along its foot; now and then the panel shudders as if someone's been thrown into it. */
  ufcfence(x) {
    const t = (now + x * 0.29) % 16, shake = t < 0.6 ? Math.floor(now * 14) % 2 : 0;
    fill(x - 12, 6 + shake, 24, 18, 0x1a1a22, 0.3); ufcFence(x - 12, 6 + shake, 24, 18);
    fill(x - 12, 5, 24, 1, 0x4a4e58);
    fill(x - 14, 4, 3, 24, 0x1c1a22); fill(x - 14, 4, 1, 24, 0x3a3a42); fill(x + 12, 4, 3, 24, 0x1c1a22); fill(x + 12, 4, 1, 24, 0x3a3a42);
    fill(x - 12, 24, 24, 4, 0x1c1a22); fill(x - 12, 24, 24, 1, 0xd8202a);
    fill(x - 13, 28, 27, 1, SHADOW);
  },
  /** A fight-night poster: two fighters squared up nose to nose, red corner and blue corner, under a gold title bar; it catches the light now and then. */
  ufcposter(x) {
    fill(x - 8, 4, 17, 23, 0x1c1a22); fill(x - 7, 5, 15, 21, 0x2a1a20);
    fill(x - 7, 5, 8, 21, 0x5a1a1a, 0.6); fill(x + 1, 5, 7, 21, 0x1a2a5a, 0.6);
    fill(x - 6, 6, 13, 3, 0xd8b45a); for (let k = 0; k < 5; k++) dot(x - 5 + k * 2, 7, 0x1c1a22);
    fill(x - 6, 11, 4, 4, 0xc89068); fill(x - 6, 10, 4, 1, 0x2a1a14); fill(x - 7, 15, 6, 9, 0xd8202a); fill(x - 2, 16, 2, 2, 0xd8202a);
    fill(x + 3, 11, 4, 4, 0x8a5a3a); fill(x + 3, 10, 4, 1, 0x1a1a1a); fill(x + 2, 15, 6, 9, 0x2a4ab0); fill(x + 1, 16, 2, 2, 0x2a4ab0);
    fill(x - 6, 24, 13, 1, 0xd8b45a);
    const t = (now + x * 0.21) % 14;
    if (t < 0.6) line(x - 6 + Math.floor(t * 20), 5, x - 8 + Math.floor(t * 20), 25, WHITE, 0.3);
    fill(x - 7, 27, 16, 1, SHADOW);
  },
  /** The title belt on a wall mount: a black leather strap with a big gold centre plate set with a red stone and two side plates; a sparkle runs over the gold now and then. */
  ufcbelt(x) {
    fill(x - 12, 13, 25, 6, 0x1c1a22); fill(x - 12, 13, 25, 1, 0x2a2a32);
    fill(x - 6, 9, 13, 13, 0xd8b45a); fill(x - 5, 8, 11, 1, 0xd8b45a); fill(x - 5, 22, 11, 1, 0xd8b45a); fill(x - 6, 9, 1, 13, 0xf2d888); fill(x + 6, 9, 1, 13, 0x9a7a30);
    ring(x, 15, 4, 0x9a7a30); disc(x, 15, 2, 0xd8202a); dot(x - 1, 14, 0xff8a8a);
    for (const sx of [-10, 9]) { fill(x + sx, 13, 3, 6, 0xd8b45a); dot(x + sx, 13, 0xf2d888); }
    dot(x - 2, 4, 0x6e727c); dot(x + 2, 4, 0x6e727c); line(x - 2, 4, x - 6, 9, 0x6e727c); line(x + 2, 4, x + 6, 9, 0x6e727c);
    const t = (now + x * 0.23) % 9;
    if (t < 0.8) { const k = Math.floor(t * 10); dot(x - 5 + k, 10 + Math.floor(k / 2), WHITE, 0.9); }
    fill(x - 11, 24, 23, 1, SHADOW);
  },
  /** The round clock: a black box with the round in red and the time left in big red digits, counting down from five minutes; at zero a bell flashes. It glows at night. */
  ufcclock(x) {
    fill(x - 9, 7, 19, 15, 0x1c1a22); fill(x - 9, 7, 19, 1, 0x3a3a42); fill(x - 8, 8, 17, 13, 0x0e0e12);
    const left = 300 - Math.floor((now + x) % 300), m = Math.floor(left / 60), s = left % 60, digit = (d, dx, dy) => {
      const segs = [[0, 0, 3, 1], [2, 0, 1, 3], [2, 2, 1, 3], [0, 4, 3, 1], [0, 2, 1, 3], [0, 0, 1, 3], [0, 2, 3, 1]];
      const on = ['1111110', '0110000', '1101101', '1111001', '0110011', '1011011', '1011111', '1110000', '1111111', '1111011'][d];
      segs.forEach(([sx, sy, w, h], i) => { if (on[i] === '1') fill(dx + sx, dy + sy, w, h, 0xff2a1a); });
    };
    digit(m, x - 7, 14); dot(x - 3, 15, 0xff2a1a); dot(x - 3, 17, 0xff2a1a); digit(Math.floor(s / 10), x - 1, 14); digit(s % 10, x + 3, 14);
    for (let k = 0; k < 3; k++) dot(x - 6 + k * 2, 10, k < 3 ? 0xd8b45a : 0x3a3a42);
    dot(x + 6, 10, left > 295 ? 0xffd040 : 0x3a3a22);
    fill(x - 8, 22, 17, 1, SHADOW);
    if (isNight) glowLate(() => glowAt(x, 16, 9, 4, 0xff2a1a, 0.12));
  },
  /** Four pairs of fight gloves hung by their laces from a row of pegs: red, blue, black and gold; now and then one pair swings. */
  ufcgloves(x) {
    fill(x - 12, 6, 24, 2, 0x8a5a32); fill(x - 12, 6, 24, 1, 0xa8703e);
    const t = (now + x * 0.31) % 12, k = Math.floor((now + x) / 12) % 4, sw = t < 1.5 ? [0, 1, 0, -1][Math.floor(t * 6) % 4] : 0;
    [0xd8202a, 0x2a4ab0, 0x1c1a22, 0xd8b45a].forEach((c, i) => {
      const gx = x - 9 + i * 6 + (i === k ? sw : 0);
      dot(x - 9 + i * 6, 8, 0x5a3b25); line(x - 9 + i * 6, 8, gx, 11, 0xd8d4cc);
      fill(gx - 2, 11, 4, 5, c); fill(gx - 2, 11, 1, 5, shaded(c, 1.2)); fill(gx - 2, 16, 4, 2, 0xf4f4f4); dot(gx + 2, 13, c);
      fill(gx - 1, 18, 4, 4, shaded(c, 0.85)); fill(gx - 1, 22, 4, 1, 0xf4f4f4);
    });
  },
  /** A speed bag on its wall platform: a teardrop leather bag under a round board; every 15 s or so it's worked in a quick rattling blur for a few seconds, then hangs still. */
  ufcspeedbag(x) {
    fill(x - 1, 3, 3, 4, 0x4a4e58); fill(x - 8, 7, 17, 3, 0x6a3a26); fill(x - 8, 7, 17, 1, 0x8a5236); fill(x - 8, 9, 17, 1, 0x4a2418);
    const t = (now + x * 0.27) % 15, hit = t < 3, f = hit ? Math.floor(now * 14) % 4 : 0, dx = [0, 2, 0, -2][f], dy = hit ? [0, -1, 0, -1][f] : 0;
    dot(x, 10, 0x2a2a30);
    fill(x - 2 + dx, 11 + dy, 5, 4, 0xa83a2a); fill(x - 1 + dx, 15 + dy, 3, 2, 0xa83a2a); dot(x + dx, 17 + dy, 0x8a2a20); dot(x - 2 + dx, 11 + dy, 0xc85a4a);
    if (hit) { dot(x - 5, 13, WHITE, 0.4); dot(x + 5, 12, WHITE, 0.4); }
  },
  /** A tall steel factory window of many small panes onto a skyline of brick and a wooden water tower; after dark the city's windows light up, and a pane or two glints by day. */
  nyloftwindow(x) {
    const steel = 0x2a2a30, sky = isNight ? 0x14183a : 0xa8d0ec;
    fill(x - 13, 3, 27, 26, steel);
    fill(x - 12, 4, 25, 24, sky);
    if (!isNight) { fill(x - 12, 18, 25, 6, 0xd8e8f0, 0.5); } else for (let k = 0; k < 5; k++) dot(x - 10 + k * 5, 6 + (k % 2) * 3, WHITE, 0.6);
    const blocks = [[-12, 16, 6, 0x8a4a3a], [-6, 12, 5, 0x6a5a5a], [-1, 18, 5, 0x9a5a42], [4, 10, 4, 0x5a5a6a], [8, 15, 5, 0x7a4a3a]];
    for (const [dx, top, w, c] of blocks) {
      fill(x + dx, top, w, 28 - top, isNight ? shaded(c, 0.5) : c); fill(x + dx, top, w, 1, isNight ? shaded(c, 0.6) : shaded(c, 1.15));
      for (let wy = top + 2; wy < 27; wy += 3) for (let wx = 1; wx < w - 1; wx += 2) dot(x + dx + wx, wy, isNight ? (hash(x + dx + wx, wy) % 3 ? 0xffd870 : 0x2a2a3a) : shaded(c, 0.7));
    }
    fill(x - 7, 7, 4, 4, 0x7a5a3a); fill(x - 8, 6, 6, 1, 0x4a3a2a); dot(x - 7, 11, 0x3a2a1e); dot(x - 4, 11, 0x3a2a1e); dot(x - 5, 5, 0x4a3a2a);
    for (const gx of [-4, 5]) fill(x + gx, 4, 1, 24, steel);
    for (const gy of [10, 16, 22]) fill(x - 12, gy, 25, 1, steel);
    fill(x, 4, 1, 24, steel);
    const t = (now + x * 0.21) % 13;
    if (!isNight && t < 0.6) dot(x - 10 + Math.floor(t * 30), 5 + Math.floor(t * 10), WHITE, 0.8);
    fill(x - 14, 28, 29, 2, 0x4a4a52); fill(x - 14, 28, 29, 1, 0x6a6a72);
    if (isNight) glowLate(() => { for (const [dx, top] of blocks) dot(x + dx + 1, top + 2, 0xffe08a); glowAt(x, 20, 12, 7, 0xffc070, 0.06); });
  },
  /** A framed transit map: coloured lines in red, green, yellow and blue crossing a pale city with a blue river down one side and white station dots; a little train dot runs along the red line now and then. */
  nyloftsubway(x) {
    fill(x - 11, 5, 23, 21, 0x2a2a30); fill(x - 10, 6, 21, 19, 0xf2efe6); fill(x + 6, 6, 4, 19, 0x9ac8e8);
    line(x - 8, 8, x - 8, 23, 0xd8302a); line(x - 8, 15, x + 4, 15, 0x3a9a4a); line(x - 3, 7, x + 2, 23, 0xf2c41a); line(x - 9, 20, x + 5, 11, 0x2a5ab0);
    for (const [dx, dy] of [[-8, 9], [-8, 15], [-8, 21], [-3, 15], [2, 15], [0, 18], [-1, 10]]) dot(x + dx, dy, WHITE);
    const t = (now + x * 0.29) % 10;
    if (t < 3) dot(x - 8, 8 + Math.floor(t * 5), 0x1c1a22);
    fill(x - 10, 26, 22, 1, SHADOW);
  },
  /** A floating shelf of records leaning on each other, a little trailing plant at one end and a small brass speaker at the other; now and then a leaf of the plant sways. */
  nyloftshelf(x) {
    fill(x - 13, 21, 26, 2, 0x6a4a2e); fill(x - 13, 21, 26, 1, 0x8a6440); fill(x - 12, 23, 2, 2, 0x2a2a30); fill(x + 10, 23, 2, 2, 0x2a2a30);
    const sleeves = [0xd8502a, 0x2a4a8a, 0xf2c41a, 0x1c1a22, 0x8a3a8a, 0x3a8a6a];
    sleeves.forEach((c, i) => { fill(x - 6 + i * 2, 12 + (i % 2), 2, 9 - (i % 2), c); dot(x - 6 + i * 2, 12 + (i % 2), shaded(c, 1.2)); });
    disc(x + 2, 16, 3.5, 0x1c1a22); dot(x + 2, 16, 0xd8502a);
    fill(x - 12, 17, 5, 4, 0xe8e2d4); dot(x - 12, 17, WHITE);
    const t = (now + x * 0.19) % 11, sw = t < 1.5 ? Math.floor(t * 4) % 2 : 0;
    for (let k = 0; k < 4; k++) { dot(x - 11 + (k % 2), 22 + k, 0x4a8a4a); dot(x - 9 + sw, 22 + k * 2, 0x5a9a5a); }
    dot(x - 11, 16, 0x4a8a4a); dot(x - 9, 15, 0x5a9a5a);
    fill(x + 7, 15, 5, 6, 0xc8a050); fill(x + 7, 15, 5, 1, 0xe8c878); disc(x + 9, 18, 1.5, 0x5a4a2a);
  },
  /** A neon heart on a black backing board, its glass tube in warm pink; it hums on steadily, flickers once in a while, and glows brightly through the night. */
  nyloftneon(x) {
    fill(x - 8, 6, 17, 17, 0x1c1a22); fill(x - 8, 6, 17, 1, 0x2e2a36);
    const t = (now + x * 0.37) % 23, off = t < 0.5 && Math.floor(now * 12) % 2;
    const pink = off ? 0x6a2a4a : 0xff6aa8;
    const heart = (c, a) => {
      for (const [dx, dy] of [[-4, 11], [-5, 12], [-5, 13], [-4, 15], [-3, 16], [-2, 17], [-1, 18], [0, 19], [1, 18], [2, 17], [3, 16], [4, 15], [5, 13], [5, 12], [4, 11], [-3, 10], [-2, 10], [3, 10], [2, 10], [-1, 11], [0, 12], [1, 11], [-5, 14], [5, 14]]) dot(x + dx, dy, c, a);
    };
    heart(pink, 1);
    if (!off) glowAt(x, 14, 8, 7, 0xff6aa8, 0.12);
    if (isNight && !off) glowLate(() => { glowAt(x, 14, 9, 8, 0xff6aa8, 0.22); heart(0xffb0d0, 0.9); });
  },
  /** A big station clock in a black steel case with a cream face and bold hands telling the real time; its red second hand ticks round. */
  nyloftclock(x) {
    disc(x, 15, 8, 0x2a2a30); disc(x, 15, 7, 0xf2ead8); ring(x, 15, 7.2, 0x4a4a52);
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 6), 15 + Math.round(Math.sin(a) * 6), 0x2a2a30); }
    const d = new Date(), ha = ((d.getHours() % 12) / 12 + d.getMinutes() / 720) * Math.PI * 2 - Math.PI / 2, ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, sa = (Math.floor(now) % 60 / 60) * Math.PI * 2 - Math.PI / 2;
    line(x, 15, x + Math.round(Math.cos(ha) * 3.5), 15 + Math.round(Math.sin(ha) * 3.5), 0x1c1a22);
    line(x, 15, x + Math.round(Math.cos(ma) * 5.5), 15 + Math.round(Math.sin(ma) * 5.5), 0x1c1a22);
    line(x, 15, x + Math.round(Math.cos(sa) * 5.5), 15 + Math.round(Math.sin(sa) * 5.5), 0xd8302a);
    dot(x, 15, 0xd8302a); fill(x - 1, 5, 3, 2, 0x2a2a30);
  },
  /** A bicycle hung on two wall hooks, steel-blue frame, cream tyres and a brown leather saddle and bar tape; now and then its front wheel turns a little. */
  nyloftbike(x) {
    const frame = 0x3a6a9a, tyre = 0xe8e0c8;
    dot(x - 6, 8, 0x2a2a30); dot(x + 6, 8, 0x2a2a30);
    const t = (now + x * 0.27) % 15, turn = t < 2 ? Math.floor(t * 4) % 4 : 0;
    for (const [cx, sp] of [[-7, 0], [7, turn]]) {
      ring(x + cx, 18, 5.5, 0x2a2a30); ring(x + cx, 18, 5, tyre); dot(x + cx, 18, 0x8a8e98);
      const a = (sp / 4) * Math.PI; line(x + cx - Math.round(Math.cos(a) * 4), 18 - Math.round(Math.sin(a) * 4), x + cx + Math.round(Math.cos(a) * 4), 18 + Math.round(Math.sin(a) * 4), 0xa8adb6, 0.6);
    }
    line(x - 7, 18, x - 1, 18, frame); line(x - 1, 18, x - 3, 11, frame); line(x - 3, 11, x + 4, 11, frame); line(x + 4, 11, x - 1, 18, frame); line(x + 4, 11, x + 7, 18, frame); line(x - 7, 18, x - 3, 11, frame);
    fill(x - 5, 9, 4, 1, 0x7a4a2a); line(x + 4, 11, x + 5, 8, frame); fill(x + 4, 8, 4, 1, 0x7a4a2a);
    dot(x - 1, 18, 0x8a8e98);
  },
  /** A tall window onto the city's roofs: cream stone fronts with grey zinc roofs and chimney pots, and the iron tower in the distance; after dark the windows glow, the tower is lit gold, and for five minutes on the hour it sparkles. */
  pariscafewindow(x) {
    const frame = 0xf2ead8, frameDark = 0xc8bca4, sky = isNight ? 0x1a1e44 : 0xb8d8f0;
    fill(x - 12, 3, 25, 25, frame); fill(x - 11, 4, 23, 23, sky);
    // The tower
    const tc = isNight ? 0xd8a040 : 0x6a5a4a, tx = x + 6;
    line(tx - 3, 22, tx, 7, tc); line(tx + 3, 22, tx, 7, tc); fill(tx - 2, 15, 5, 1, tc); fill(tx - 1, 11, 3, 1, tc); dot(tx, 6, tc); fill(tx - 3, 19, 7, 1, tc);
    const d = new Date(), sparkle = isNight && d.getMinutes() < 5;
    // The roofs
    fill(x - 11, 18, 23, 9, isNight ? 0x6a6050 : 0xe8dcc0);
    fill(x - 11, 16, 12, 3, isNight ? 0x3a4050 : 0x7a8496); fill(x + 2, 17, 10, 2, isNight ? 0x3a4050 : 0x7a8496);
    for (const cx of [x - 9, x - 4, x + 4]) { fill(cx, 13 + (cx % 2), 2, 3, isNight ? 0x5a4a3a : 0xb8704a); }
    for (let wx = x - 10; wx < x + 11; wx += 3) for (const wy of [21, 24]) fill(wx, wy, 2, 2, isNight ? (hash(wx, wy) % 3 ? 0xffd870 : 0x3a3448) : 0x6a7a8a);
    fill(x, 4, 1, 23, frame); fill(x - 11, 14, 23, 1, frame);
    // The iron balconette
    fill(x - 12, 26, 25, 1, 0x2a2a30); for (let k = 0; k < 25; k += 2) dot(x - 12 + k, 25, 0x2a2a30);
    fill(x - 13, 28, 27, 1, frameDark);
    if (isNight) glowLate(() => {
      glowAt(tx, 15, 4, 8, 0xffc060, 0.12);
      if (sparkle) for (let k = 0; k < 5; k++) { const h = hash(Math.floor(now * 6), k); dot(tx - 2 + (h % 5), 8 + ((h >>> 4) % 14), WHITE); }
      dot(tx, 6, 0xfff4c0);
    });
  },
  /** The chalk menu: a slate in a wooden frame, lines of chalk writing in white and a heading underlined in pink, a little cup drawn in the corner; every so often a line is wiped and written again. */
  pariscafemenu(x) {
    fill(x - 9, 4, 19, 23, 0x8a5a32); fill(x - 8, 5, 17, 21, 0x2a2e2a);
    fill(x - 6, 7, 12, 1, WHITE, 0.85); fill(x - 6, 9, 12, 1, 0xf28fb0, 0.7);
    const t = (now + x * 0.29) % 22, wiping = t < 3;
    for (let r = 0; r < 5; r++) {
      const len = 6 + (hash(x, r) % 7), shown = wiping && r === 2 ? Math.floor(t / 3 * len) : len;
      for (let k = 0; k < shown; k += 1 + (k % 3 === 2 ? 1 : 0)) dot(x - 6 + k, 12 + r * 3, WHITE, 0.75);
      dot(x + 5, 12 + r * 3, 0xf2d35b, 0.8);
    }
    fill(x + 3, 22, 3, 2, WHITE, 0.6); dot(x + 6, 22, WHITE, 0.6);
  },
  /** A tall bistro mirror in a gilt frame with curled corners; its glass shows a soft blur of the room, and a gleam slides across it now and then. */
  pariscafemirror(x) {
    const gilt = 0xc8a050, giltLight = 0xe8c878, giltDark = 0x8a6a2a;
    fill(x - 10, 3, 21, 25, gilt); fill(x - 10, 3, 21, 1, giltLight); fill(x - 10, 3, 1, 25, giltLight); fill(x + 10, 3, 1, 25, giltDark);
    dot(x - 11, 3, gilt); dot(x + 11, 3, gilt); dot(x, 2, giltLight); dot(x - 1, 2, gilt); dot(x + 1, 2, gilt);
    fill(x - 8, 5, 17, 21, 0xb8c4c8); fill(x - 8, 16, 17, 10, 0xa89a88, 0.6); fill(x - 6, 12, 4, 6, 0x8a5a3a, 0.4); fill(x + 2, 10, 5, 8, 0x5a1a24, 0.3);
    const t = (now + x * 0.23) % 12;
    if (t < 1.2) { const k = Math.floor(t * 18) - 8; line(x + k - 3, 6, x + k + 3, 24, WHITE, 0.35); }
    dot(x - 7, 6, WHITE, 0.6);
  },
  /** An art nouveau poster: a woman with long flowing hair in curling strands against a gold halo, framed in green whiplash curves; a curl of her hair lifts now and then. */
  pariscafeposter(x) {
    fill(x - 7, 3, 15, 25, 0x2a4a3a); fill(x - 6, 4, 13, 23, 0xf2e2b8);
    disc(x, 11, 5, 0xe8b84a); ring(x, 11, 5, 0xc8902a);
    fill(x - 1, 9, 3, 4, 0xf2d0b0); dot(x, 13, 0xf2d0b0);
    const t = (now + x * 0.31) % 14, lift = t < 1.5 ? 1 : 0;
    fill(x - 3, 8, 2, 8, 0xc8603a); fill(x + 2, 8, 2, 7, 0xc8603a); fill(x - 2, 7, 5, 2, 0xc8603a); dot(x - 4, 15 - lift, 0xc8603a); dot(x + 4, 14 - lift, 0xc8603a); dot(x - 5, 16 - lift, 0xc8603a);
    fill(x - 3, 15, 7, 9, 0x6a8a5a); fill(x - 2, 15, 1, 9, 0x8aaa7a);
    for (let k = 0; k < 4; k++) { dot(x - 6 + k, 24 - k, 0x2a4a3a); dot(x + 6 - k, 24 - k, 0x2a4a3a); }
    fill(x - 5, 25, 11, 1, 0xc8603a, 0.6);
  },
  /** An ornate brass wall clock with a cream enamel face and black roman-style ticks, showing the real time, a little pendulum swinging under it. */
  pariscafeclock(x) {
    const brass = 0xc8a050, brassLight = 0xe8c878;
    const sw = [0, 1, 1, 0, -1, -1][Math.floor(now * 3) % 6];
    line(x, 18, x + sw, 25, brass); disc(x + sw, 26, 1.3, brassLight);
    disc(x, 12, 7, brass); disc(x, 12, 5.5, 0xf4ecd8); dot(x, 4, brassLight); dot(x - 1, 5, brass); dot(x + 1, 5, brass);
    for (let k = 0; k < 12; k += 3) { const a = (k / 12) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 4.5), 12 + Math.round(Math.sin(a) * 4.5), 0x2a2a30); }
    const d = new Date(), ha = ((d.getHours() % 12) / 12 + d.getMinutes() / 720) * Math.PI * 2 - Math.PI / 2, ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2;
    line(x, 12, x + Math.round(Math.cos(ha) * 2.5), 12 + Math.round(Math.sin(ha) * 2.5), 0x1c1a22);
    line(x, 12, x + Math.round(Math.cos(ma) * 4), 12 + Math.round(Math.sin(ma) * 4), 0x1c1a22);
    dot(x - 5, 9, brassLight);
  },
  /** A shelf of bottles behind the bar: wine in dark green, a pale green absinthe, an amber cognac and a clear one, catching the light in turn. */
  pariscafebottles(x) {
    fill(x - 13, 22, 26, 2, 0x5a3b25); fill(x - 13, 22, 26, 1, 0x8a5a32); fill(x - 13, 13, 26, 1, 0x5a3b25);
    const bottles = [[-11, 0x1a3a22, 9], [-7, 0x1a3a22, 9], [-3, 0x8ad07a, 8], [1, 0xc87a2a, 7], [5, 0xdff0f6, 9], [9, 0x5a1a24, 9]];
    const k = Math.floor((now + x) / 3) % bottles.length;
    bottles.forEach(([dx, c, h], i) => {
      fill(x + dx, 22 - h + 3, 3, h - 3, c, c === 0xdff0f6 ? 0.6 : 1); fill(x + dx + 1, 22 - h, 1, 3, c, c === 0xdff0f6 ? 0.6 : 1); dot(x + dx + 1, 22 - h - 1, 0xc8a050);
      fill(x + dx, 22 - h + 5, 3, 2, 0xf2ead8, 0.85);
      dot(x + dx, 22 - h + 3, WHITE, i === k ? 0.8 : 0.3);
    });
    for (let k2 = 0; k2 < 4; k2++) { fill(x - 11 + k2 * 6, 10, 3, 3, 0xdff0f6, 0.5); dot(x - 10 + k2 * 6, 12, 0xdff0f6, 0.7); }
  },
  /** A string of papel picado: cut-paper flags in pink, orange, turquoise, yellow and purple, each punched with a little skull or flower; now and then a breeze sets them rippling. */
  muertospapel(x) {
    const cols = [0xe83a8a, 0xe8902a, 0x1aa8a8, 0xf2c41a, 0x8a3ab0];
    line(x - 13, 4, x + 13, 4, 0x6a4a3a);
    const t = (now + x * 0.21) % 9, rip = t < 2;
    for (let k = 0; k < 5; k++) {
      const fx = x - 12 + k * 5, c = cols[k], wave = rip ? Math.round(Math.sin(now * 8 + k)) : 0;
      fill(fx, 5, 5, 7 + wave, c); for (let j = 0; j < 5; j += 2) dot(fx + j, 12 + wave, c);
      if (k % 2) { dot(fx + 1, 7, 0x6a4a3a, 0.5); dot(fx + 3, 7, 0x6a4a3a, 0.5); dot(fx + 2, 9, 0x6a4a3a, 0.5); }
      else { dot(fx + 2, 7, 0x6a4a3a, 0.5); dot(fx + 1, 8, 0x6a4a3a, 0.5); dot(fx + 3, 8, 0x6a4a3a, 0.5); dot(fx + 2, 9, 0x6a4a3a, 0.5); }
    }
    line(x - 13, 15, x + 13, 15, 0x6a4a3a);
    for (let k = 0; k < 5; k++) { const fx = x - 12 + k * 5, c = cols[(k + 2) % 5]; fill(fx, 16, 5, 6, c); dot(fx + 2, 18, 0x6a4a3a, 0.5); dot(fx + 1, 19, 0x6a4a3a, 0.5); dot(fx + 3, 19, 0x6a4a3a, 0.5); }
  },
  /** A shelf of family photographs in punched tin frames, a little candle burning in front of each and marigolds between them; the candles flicker and glow at night. */
  muertosretablo(x) {
    fill(x - 13, 22, 26, 2, 0x8a5a32); fill(x - 13, 22, 26, 1, 0xa8703e);
    const frames = [[-9, 10, 7], [0, 8, 8], [9, 11, 7]];
    frames.forEach(([dx, top, h], i) => {
      fill(x + dx - 3, top, 7, h, 0xb8bcc4); fill(x + dx - 3, top, 7, 1, 0xe0e4ea); for (let k = 0; k < 7; k += 2) dot(x + dx - 3 + k, top + h - 1, 0x8a8e98);
      fill(x + dx - 2, top + 1, 5, h - 2, 0xd8c8a8); fill(x + dx - 1, top + 2, 3, 2, 0x8a6a4a); fill(x + dx - 2, top + 4, 5, h - 6, [0x5a4a6a, 0x3a5a4a, 0x6a3a3a][i]);
    });
    const lamp = (a) => { for (const dx of [-9, 0, 9]) { fill(x + dx, 19, 1, 3, 0xf6f2ea); dot(x + dx, 18, 0xffd36b, a * candlelight(dx + x)); } };
    lamp(1);
    for (const dx of [-5, 5]) { fill(x + dx - 1, 19, 3, 3, 0xe8902a); dot(x + dx, 18, 0xf2b03a); }
    if (isNight) glowLate(() => { for (const dx of [-9, 0, 9]) glowAt(x + dx, 18, 4, 3, 0xffc060, 0.2); lamp(0.9); });
  },
  /** A portrait of an elegant skeleton lady in a wide hat heaped with flowers and a feather, a lace collar at her neck, in a carved gold frame; her flowers stir now and then. */
  muertoscatrina(x) {
    fill(x - 8, 3, 17, 25, 0xc8a050); fill(x - 8, 3, 17, 1, 0xe8c878); fill(x + 8, 3, 1, 25, 0x8a6a2a); fill(x - 7, 4, 15, 23, 0x3a1a4a);
    fill(x - 6, 9, 13, 2, 0x1c1a22); fill(x - 4, 7, 9, 2, 0x1c1a22);
    const t = (now + x * 0.27) % 12, stir = t < 1 ? 1 : 0;
    for (const [dx, c] of [[-4, 0xe83a8a], [-1, 0xe8902a], [2, 0x8a3ab0], [4, 0xf2c41a]]) { fill(x + dx, 5 - (dx === 2 ? stir : 0), 2, 2, c); }
    line(x + 5, 6, x + 7, 3 - stir, 0xf6f2ea);
    fill(x - 2, 11, 5, 6, 0xf6f2ea); fill(x - 1, 17, 3, 1, 0xf6f2ea);
    fill(x - 1, 13, 1, 1, 0x1c1a22); fill(x + 1, 13, 1, 1, 0x1c1a22); ring(x - 1, 13, 1.2, 0xe83a8a, 0.6); ring(x + 1, 13, 1.2, 0x1a8a8a, 0.6); dot(x, 15, 0x1c1a22); dot(x, 16, 0x1c1a22, 0.6);
    fill(x - 5, 18, 11, 9, 0x1c1a22); for (let k = 0; k < 9; k += 2) dot(x - 4 + k, 18, 0xf6f2ea); dot(x, 20, 0xe83a8a);
  },
  /** A punched tin sun with a calm face and wavy rays, painted gold and pink; it catches the light now and then as if turning. */
  muertostinsun(x) {
    const tin = 0xd8c870, tinLight = 0xf2e8a0, tinDark = 0xa89a4a;
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2, r = k % 2 ? 9 : 7; line(x + Math.round(Math.cos(a) * 5), 15 + Math.round(Math.sin(a) * 5), x + Math.round(Math.cos(a) * r), 15 + Math.round(Math.sin(a) * r), k % 2 ? tin : 0xe8902a); }
    disc(x, 15, 5, tin); ring(x, 15, 5, tinDark); dot(x - 2, 13, tinDark); dot(x + 2, 13, tinDark); line(x - 1, 17, x + 1, 17, 0xc84a5a); dot(x - 3, 16, 0xf28fb0); dot(x + 3, 16, 0xf28fb0);
    for (let k = 0; k < 8; k++) { const a = (k / 8) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 3.5), 15 + Math.round(Math.sin(a) * 3.5), tinLight, 0.7); }
    const t = (now + x * 0.23) % 10;
    if (t < 0.8) { const a = t * 8; dot(x + Math.round(Math.cos(a) * 6), 15 + Math.round(Math.sin(a) * 6), WHITE); }
  },
  /** A painted guitar hung on a peg: a pale wooden body painted with flowers round its sound hole, a dark neck and a ribbon in pink and green; now and then a string quivers. */
  muertosguitar(x) {
    dot(x, 3, 0x5a3b25); fill(x - 1, 4, 3, 2, 0x3a2418); fill(x, 6, 1, 10, 0x4a2a1a);
    fill(x - 3, 16, 7, 5, 0xe8c890); fill(x - 4, 21, 9, 6, 0xe8c890); fill(x - 3, 27, 7, 1, 0xe8c890); fill(x - 4, 21, 1, 6, 0xf2d8a8); fill(x + 4, 21, 1, 6, 0xc8a870);
    disc(x, 21, 1.5, 0x3a2418);
    for (const [dx, dy, c] of [[-2, 18, 0xe83a8a], [2, 18, 0x1a8a8a], [-3, 24, 0xf2c41a], [3, 25, 0xe83a8a]]) dot(x + dx, dy, c);
    const t = (now + x * 0.31) % 9, q = t < 0.6 ? Math.floor(now * 20) % 2 : 0;
    fill(x + q, 6, 1, 20, 0xe0e4ea, 0.5);
    line(x + 1, 7, x + 4, 11, 0xe83a8a); line(x + 1, 8, x + 3, 12, 0x3a9a4a);
  },
  /** A window onto the town at night or by day: pastel houses up a hill and a church dome, with strings of lights and candles in the windows after dark, and marigolds on the sill. */
  muertoswindow(x) {
    const frame = 0x1a8a8a, sky = isNight ? 0x1a1440 : 0x9ad0f0;
    fill(x - 11, 4, 23, 23, frame); fill(x - 10, 5, 21, 20, sky);
    if (isNight) for (let k = 0; k < 5; k++) dot(x - 8 + k * 4, 6 + (k % 2) * 2, WHITE, 0.7);
    const houses = [[-10, 15, 5, 0xf28fb0], [-5, 13, 4, 0xf2d35b], [-1, 16, 5, 0x6ad0c8], [4, 12, 6, 0xe8902a]];
    for (const [dx, top, w, c] of houses) { fill(x + dx, top, w, 25 - top, isNight ? shaded(c, 0.55) : c); dot(x + dx + 1, top + 3, isNight ? 0xffd870 : 0x3a3a48); dot(x + dx + w - 2, top + 3, isNight ? 0xffd870 : 0x3a3a48); }
    disc(x + 7, 11, 2.5, isNight ? 0x8a7a5a : 0xf2ead8); fill(x + 7, 7, 1, 2, isNight ? 0x8a7a5a : 0xf2ead8);
    for (let k = 0; k < 21; k += 2) dot(x - 10 + k, 9 + Math.round(Math.sin(k * 0.3)), [0xe83a8a, 0xf2c41a, 0x6ad0c8][k % 3], isNight ? 1 : 0.7);
    fill(x, 5, 1, 20, frame);
    fill(x - 12, 25, 25, 3, 0x12706e); for (let k = 0; k < 23; k += 3) { fill(x - 11 + k, 23, 2, 2, 0xe8902a); dot(x - 11 + k, 25, 0x3a7a3a); }
    if (isNight) glowLate(() => { for (let k = 0; k < 21; k += 2) dot(x - 10 + k, 9 + Math.round(Math.sin(k * 0.3)), [0xff6aa8, 0xffe070, 0x8af0e8][k % 3]); glowAt(x, 18, 10, 6, 0xffb060, 0.08); });
  },
  /** A big round viewport in a bolted steel ring onto the abyss: dark water with drifting specks, a glowing jellyfish bobbing past and, now and then, the shadow of a great whale gliding by. It glows faintly blue at night. */
  submarinedeepviewport(x) {
    const cx = x, cy = 15;
    disc(cx, cy, 11, 0x8a96a0); ring(cx, cy, 11, 0x5a6670); disc(cx, cy, 9, 0x0a1830);
    for (let k = 0; k < 10; k++) { const a = (k / 10) * Math.PI * 2; dot(cx + Math.round(Math.cos(a) * 10), cy + Math.round(Math.sin(a) * 10), 0xd0d6da); }
    fill(cx - 8, cy + 3, 17, 4, 0x0e2440, 0.8);
    for (let k = 0; k < 6; k++) { const sy = cy - 7 + ((Math.floor(now * 2) + k * 3) % 15); dot(cx - 6 + ((k * 5) % 13), sy, 0x8ab0d0, 0.5); }
    const t = (now + x * 0.37) % 30;
    if (t < 4) { const wx = Math.round(cx - 14 + t * 7); for (let k = 0; k < 12; k++) { const px = wx + k - 6; if (Math.abs(px - cx) < 8) fill(px, cy - 1 + (k < 3 || k > 9 ? 1 : 0), 1, k > 2 && k < 10 ? 4 : 2, 0x1c3050); } }
    const jy = cy + Math.round(Math.sin(now * 0.8 + x) * 3) - 1, jx = cx + 3;
    fill(jx - 1, jy, 3, 2, 0xc080ff, 0.9); dot(jx - 1, jy + 2, 0xc080ff, 0.6); dot(jx + 1, jy + 3, 0xc080ff, 0.6); dot(jx, jy + 3, 0xc080ff, 0.4);
    dot(cx - 6, cy - 6, WHITE, 0.4); dot(cx - 5, cy - 7, WHITE, 0.3);
    glowAt(cx, cy, 9, 9, 0x2a90d0, isNight ? 0.12 : 0.05);
    if (isNight) glowLate(() => { glowAt(jx, jy, 3, 3, 0xc080ff, 0.3); fill(jx - 1, jy, 3, 2, 0xe0c0ff, 0.9); });
  },
  /** A depth gauge: a round white dial in a brass-free steel bezel with an orange needle on the depth, a small green readout beneath; the needle wavers now and then as the sub trims. */
  submarinedeepdepth(x) {
    disc(x, 13, 7, 0x8a96a0); disc(x, 13, 6, 0xf4f6f8); ring(x, 13, 6, 0x5a6670);
    for (let k = 0; k < 9; k++) { const a = Math.PI * 0.75 + (k / 8) * Math.PI * 1.5; dot(x + Math.round(Math.cos(a) * 5), 13 + Math.round(Math.sin(a) * 5), k > 6 ? 0xd8302a : 0x2a2a30); }
    const t = (now + x * 0.21) % 12, waver = t < 1.5 ? Math.sin(now * 10) * 0.12 : 0, a = Math.PI * 0.75 + 0.62 * Math.PI * 1.5 + waver;
    line(x, 13, x + Math.round(Math.cos(a) * 4.5), 13 + Math.round(Math.sin(a) * 4.5), 0xe8702a); dot(x, 13, 0x2a2a30);
    fill(x - 5, 21, 11, 5, 0x1c242e); for (let k = 0; k < 4; k++) dot(x - 4 + k * 2, 23, 0x5ad86a); dot(x + 4, 23, Math.floor(now * 2) % 2 ? 0x5ad86a : 0x2a4a30);
    if (isNight) glowLate(() => { for (let k = 0; k < 4; k++) dot(x - 4 + k * 2, 23, 0x8af09a); });
  },
  /** A seabed map on a dark screen: green contour lines round a trench, a cyan route dotted along it and a blinking orange dot where the sub is now. It glows at night. */
  submarinedeepmap(x) {
    fill(x - 11, 5, 23, 19, 0x2a3440); fill(x - 10, 6, 21, 17, 0x0a1a24);
    for (let r = 0; r < 4; r++) ellipseRing(x - 1, 14, 3 + r * 2.5, 2 + r * 1.6, 0x3aa86a, 0.7 - r * 0.12);
    for (let k = 0; k < 7; k++) dot(x - 9 + k * 3, 20 - Math.round(Math.sin(k * 0.7) * 3), 0x2ad8d8);
    const blink = Math.floor(now * 2) % 2, pos = Math.floor((now / 4) % 7);
    dot(x - 9 + pos * 3, 20 - Math.round(Math.sin(pos * 0.7) * 3), blink ? 0xff8a3a : 0x8a4a1a);
    fill(x - 11, 24, 23, 1, SHADOW);
    if (isNight) glowLate(() => glowAt(x, 14, 10, 8, 0x2ad8a0, 0.08));
  },
  /** A run of pipes along the wall with two red valve wheels and a pressure gauge; now and then a little puff of steam hisses from a joint. */
  submarinedeepvalves(x) {
    fill(x - 13, 8, 26, 3, 0x8a96a0); fill(x - 13, 8, 26, 1, 0xb8c2ca); fill(x - 13, 18, 26, 3, 0x6a7680); fill(x - 13, 18, 26, 1, 0x8a96a0);
    fill(x - 6, 11, 3, 7, 0x8a96a0); fill(x + 5, 11, 3, 7, 0x6a7680);
    for (const [vx, vy] of [[-5, 9], [6, 19]]) { ring(x + vx, vy, 3, 0xd8302a); line(x + vx - 3, vy, x + vx + 3, vy, 0xa82020); line(x + vx, vy - 3, x + vx, vy + 3, 0xa82020); dot(x + vx, vy, 0x2a2a30); }
    disc(x + 1, 14, 2.5, 0xf4f6f8); ring(x + 1, 14, 2.5, 0x5a6670); line(x + 1, 14, x + 2, 13, 0xd8302a);
    const t = (now + x * 0.27) % 13;
    if (t < 1.4) { const r = Math.floor(t * 4); dot(x + 10 + r, 7 - r, WHITE, 0.6 - r * 0.1); dot(x + 11 + r, 6 - r, WHITE, 0.4); }
    for (const jx of [-11, 11]) { fill(x + jx, 7, 2, 5, 0x5a6670); fill(x + jx, 17, 2, 5, 0x4a5660); }
  },
  /** An open dive locker: an old brass-trimmed diving helmet on the top shelf, a pair of yellow fins and a mask hung below, a tank in the corner; the mask swings a little now and then. */
  submarinedeeplocker(x) {
    fill(x - 10, 3, 21, 24, 0x5a6670); fill(x - 9, 4, 19, 22, 0x2a3440); fill(x - 9, 12, 19, 1, 0x8a96a0);
    disc(x - 2, 8, 3.5, 0xc8a050); disc(x - 2, 8, 2, 0x1a3a5a); dot(x - 3, 7, 0x8ad0f0); fill(x - 5, 11, 7, 1, 0xa8803a);
    fill(x + 4, 5, 4, 7, 0xe8702a); fill(x + 4, 5, 4, 1, 0xf2904a); fill(x + 5, 4, 2, 1, 0x2a2a30);
    const t = (now + x * 0.31) % 11, sw = t < 1.5 ? [0, 1, 0, -1][Math.floor(t * 4) % 4] : 0;
    line(x - 5, 13, x - 5 + sw, 16, 0x2a2a30); fill(x - 7 + sw, 16, 5, 3, 0x1c1e26); fill(x - 6 + sw, 17, 3, 1, 0x8ad0f0);
    fill(x + 1, 14, 3, 11, 0xf2c41a); fill(x + 5, 14, 3, 11, 0xf2c41a); fill(x + 1, 22, 3, 3, 0xd8a41a); fill(x + 5, 22, 3, 3, 0xd8a41a);
    fill(x - 10, 27, 21, 1, SHADOW);
  },
  /** A shelf of specimen jars in a steel rack, each holding a strange deep-sea creature, a squid, a glowing jellyfish, a tiny fish; they glow faintly at night, one pulsing now and then. */
  submarinedeepsamples(x) {
    fill(x - 13, 22, 26, 2, 0x8a96a0); fill(x - 13, 22, 26, 1, 0xb8c2ca); fill(x - 13, 12, 26, 1, 0x8a96a0);
    const jars = [[-10, 0xc080ff], [-4, 0x4ae0ff], [2, 0x8af09a], [8, 0xff8ad0]];
    const pulse = Math.floor((now + x) / 3) % 4;
    const draw = (glowA) => jars.forEach(([dx, c], i) => {
      fill(x + dx, 14, 5, 8, 0xdff0f6, 0.35); fill(x + dx, 13, 5, 1, 0x5a6670); fill(x + dx, 14, 1, 8, 0xdff0f6, 0.6);
      fill(x + dx + 1, 16, 3, 2, c, i === pulse ? 1 : 0.75); dot(x + dx + 1, 18, c, 0.6); dot(x + dx + 3, 19, c, 0.6);
      if (glowA) glowAt(x + dx + 2, 17, 3, 3, c, glowA * (i === pulse ? 1.6 : 1));
    });
    draw(0);
    for (let k = 0; k < 3; k++) fill(x - 10 + k * 8, 7, 5, 5, 0xdff0f6, 0.3);
    if (isNight) glowLate(() => draw(0.12));
  },
  /** A triple-glazed window onto the ice: white plains and blue ridges under a pale sky, the aurora rippling green after dark; now and then a polar bear plods across. */
  arcticbasewindow(x) {
    const frame = 0xe8ecee, sky = isNight ? 0x0a1430 : 0xc8e0f0;
    fill(x - 12, 4, 25, 23, frame); fill(x - 11, 5, 23, 21, sky);
    if (isNight) { for (let k = 0; k < 5; k++) dot(x - 9 + k * 5, 6 + (k % 2), WHITE, 0.6); }
    fill(x - 11, 15, 23, 3, isNight ? 0x3a4a6a : 0x8aaacc); fill(x - 6, 13, 6, 2, isNight ? 0x3a4a6a : 0x8aaacc); fill(x + 3, 14, 5, 1, isNight ? 0x3a4a6a : 0x8aaacc);
    fill(x - 11, 18, 23, 8, isNight ? 0x8a9ab8 : 0xf4f8fa); fill(x - 11, 18, 23, 1, isNight ? 0xa8b8d0 : WHITE);
    const t = (now + x * 0.31) % 40;
    if (t < 8) { const bx = Math.round(x - 13 + t * 3.2), bob = Math.floor(t * 3) % 2; if (bx > x - 11 && bx < x + 9) { fill(bx, 19 - bob, 5, 2, isNight ? 0xc8d0dc : 0xf2ece0); fill(bx + 4, 18 - bob, 2, 2, isNight ? 0xc8d0dc : 0xf2ece0); dot(bx + 5, 18 - bob, 0x1c1a22); dot(bx, 21, 0xd8d0c0); dot(bx + 3, 21, 0xd8d0c0); } }
    const aurora = (a) => { for (let k = 0; k < 23; k++) { const ay = 7 + Math.round(Math.sin(k * 0.4 + now * 0.6) * 2); fill(x - 11 + k, ay, 1, 3, 0x5af0a0, a); dot(x - 11 + k, ay - 1, 0xa070ff, a * 0.6); } };
    if (isNight) aurora(0.6);
    fill(x, 5, 1, 21, frame); fill(x - 11, 15, 23, 1, frame, 0.5);
    fill(x - 13, 26, 27, 2, 0xd0d6da); dot(x - 10, 25, WHITE); dot(x + 8, 25, WHITE);
    if (isNight) glowLate(() => aurora(0.45));
  },
  /** A polar map on a board: the white ice cap in the middle of pale blue sea, the station marked with a red pin, and a dotted route out to a blue flag that blinks now and then. */
  arcticbasemap(x) {
    fill(x - 11, 5, 23, 20, 0x8a5a32); fill(x - 10, 6, 21, 18, 0x9ac8e8);
    disc(x - 1, 15, 7, 0xf4f8fa); disc(x + 3, 13, 4, 0xf4f8fa); fill(x - 6, 18, 4, 2, 0xd8e8f0);
    for (let k = 0; k < 4; k++) ring(x - 1, 15, 2 + k * 2.5, 0x5a8ab0, 0.15);
    dot(x - 3, 16, 0xd8302a); dot(x - 3, 15, 0xd8302a); dot(x - 3, 17, 0x2a2a30);
    for (let k = 0; k < 5; k++) dot(x - 2 + k * 2, 15 - k, 0x2a2a30, 0.7);
    const blink = Math.floor(now * 1.5) % 2;
    fill(x + 7, 8, 1, 4, 0x2a2a30); fill(x + 8, 8, 2, 2, blink ? 0x2a5ab0 : 0x5a8ad0);
    fill(x - 10, 25, 22, 1, SHADOW);
  },
  /** The weather panel: a grey box with a round wind dial whose needle swings in the gusts, a red digital temperature far below zero, and a little anemometer spinning on top. */
  arcticbaseweather(x) {
    fill(x - 10, 8, 21, 18, 0x5a626a); fill(x - 10, 8, 21, 1, 0x7a828a); fill(x - 9, 9, 19, 16, 0x3a4048);
    disc(x - 4, 16, 5, 0xf4f6f8); ring(x - 4, 16, 5, 0x8a929a);
    const a = Math.PI * (0.9 + 0.5 * (Math.sin(now * 0.7 + x) * 0.5 + 0.5)) + Math.sin(now * 5) * 0.08;
    line(x - 4, 16, x - 4 + Math.round(Math.cos(a) * 4), 16 + Math.round(Math.sin(a) * 4), 0xd8302a);
    fill(x + 2, 12, 7, 5, 0x1c1a22); fill(x + 3, 14, 1, 1, 0xff3a2a); fill(x + 5, 13, 1, 3, 0xff3a2a); fill(x + 7, 13, 1, 3, 0xff3a2a);
    fill(x + 2, 19, 7, 4, 0x1c1a22); dot(x + 3, 21, 0x5ad86a); dot(x + 5, 21, 0x5ad86a); dot(x + 7, 21, (Math.floor(now * 2)) % 2 ? 0x5ad86a : 0x2a4a30);
    fill(x, 4, 1, 4, 0x8a929a); const sp = Math.floor(now * 8) % 2; fill(x - 3, 3 + sp, 7, 1, 0xa8b2ba); dot(x - 3, 3 + sp, 0xd8302a); dot(x + 3, 3 + sp, 0xd8302a);
    fill(x - 10, 26, 21, 1, SHADOW);
  },
  /** A field radio set on a shelf: a khaki box of dials and a meter, a coiled handset and headphones hung beside it; its meter needle dances and a green lamp blinks while a call comes in now and then. */
  arcticbaseradio(x) {
    fill(x - 12, 22, 24, 2, 0x8a5a32); fill(x - 12, 22, 24, 1, 0xa8703e);
    fill(x - 10, 12, 16, 10, 0x6a6a4a); fill(x - 10, 12, 16, 1, 0x8a8a62); fill(x + 5, 12, 1, 10, 0x4a4a32);
    disc(x - 7, 18, 1.5, 0x2a2a30); disc(x - 3, 18, 1.5, 0x2a2a30); fill(x, 14, 4, 3, 0xe8e2c8);
    const t = (now + x * 0.23) % 16, call = t < 3;
    line(x + 2, 17, x + 2 + (call ? Math.round(Math.sin(now * 12)) : -1), 15, 0x1c1a22);
    dot(x + 3, 19, call && Math.floor(now * 4) % 2 ? 0x5ad86a : 0x2a4a30);
    fill(x + 7, 14, 3, 6, 0x2a2a30); for (let k = 0; k < 5; k++) dot(x + 6 - (k % 2), 15 + k * 2, 0x2a2a30);
    ring(x + 10, 9, 2, 0x2a2a30); fill(x + 8, 9, 1, 3, 0x2a2a30); fill(x + 11, 9, 1, 3, 0x2a2a30); dot(x + 10, 6, 0x5a3b25);
  },
  /** A pair of wooden cross-country skis crossed on the wall with their poles, red tips curling up and leather bindings; now and then a flake of snow drops off a tip. */
  arcticbaseskis(x) {
    line(x - 8, 26, x + 6, 4, 0xa8703e); line(x - 7, 26, x + 7, 4, 0x8a5a32); line(x + 8, 26, x - 6, 4, 0xa8703e); line(x + 7, 26, x - 7, 4, 0x8a5a32);
    fill(x + 6, 3, 2, 2, 0xd8302a); fill(x - 8, 3, 2, 2, 0xd8302a);
    fill(x - 2, 13, 5, 2, 0x5a3a24); line(x - 9, 6, x + 9, 24, 0x5a626a); line(x + 9, 6, x - 9, 24, 0x5a626a); disc(x - 8, 22, 1.5, 0x2a2a30, 0.6);
    const t = (now + x * 0.17) % 14;
    if (t < 1.5) dot(x + 7, 5 + Math.floor(t * 8), WHITE);
    dot(x + 6, 4, WHITE, 0.8);
  },
  /** An ice-core chart: a long paper chart pinned to the wall with banded layers of ice drawn down one side and a wandering blue temperature line beside them; a pen marks a new point now and then. */
  arcticbasechart(x) {
    fill(x - 10, 4, 21, 23, 0xf4f2ea); dot(x - 9, 5, 0xd8302a); dot(x + 9, 5, 0xd8302a);
    for (let k = 0; k < 9; k++) fill(x - 8, 7 + k * 2, 5, 2, k % 2 ? 0xc8dcec : 0xe8f0f6);
    for (let k = 0; k < 18; k++) dot(x - 1 + Math.round(Math.sin(k * 0.6) * 3 + (k > 12 ? 2 : 0)), 7 + k, 0x2a5ab0);
    for (let k = 0; k < 4; k++) fill(x + 5, 8 + k * 4, 4, 1, 0xb8b4aa);
    const t = (now + x * 0.29) % 12;
    if (t < 1.2) dot(x + 3, 24, 0xd8302a);
    fill(x - 9, 27, 20, 1, SHADOW);
  },
  /** A blossoming cherry branch reaching in across the wall from one side, dark twisting wood laden with pale pink flowers; now and then a petal lets go and drifts down. */
  sakurabranch(x) {
    const wood = 0x5a3a2e;
    line(x - 14, 8, x - 6, 11, wood); line(x - 6, 11, x + 4, 9, wood); line(x + 4, 9, x + 13, 12, wood); line(x - 14, 9, x - 6, 12, wood);
    line(x - 4, 11, x - 1, 17, wood); line(x + 4, 9, x + 6, 4, wood); line(x + 8, 10, x + 10, 16, wood);
    const flowers = [[-12, 7], [-9, 10], [-7, 8], [-4, 12], [-2, 9], [-1, 16], [1, 10], [3, 7], [5, 4], [7, 6], [8, 11], [10, 14], [11, 10], [13, 12], [-5, 14], [0, 13]];
    for (const [dx, dy] of flowers) { fill(x + dx - 1, dy - 1, 3, 3, 0xf6c8d4); dot(x + dx, dy - 1, WHITE); dot(x + dx, dy, 0xe87a98); }
    const t = (now + x * 0.29) % 9;
    if (t < 3) { const k = Math.floor((now + x) / 9) % flowers.length, [dx, dy] = flowers[k]; dot(x + dx + Math.round(Math.sin(t * 3) * 2), dy + Math.floor(t * 5), 0xf6c8d4); }
  },
  /** A window onto a spring park: a canal edged with blossoming cherry trees in clouds of pink, under a pale sky; petals blow past the glass now and then, and at night the trees are lit by lanterns along the path. */
  sakurawindow(x) {
    const frame = 0xf6f2ea, sky = isNight ? 0x1a2048 : 0xc8e4f4;
    fill(x - 12, 4, 25, 23, frame); fill(x - 11, 5, 23, 21, sky);
    fill(x - 11, 20, 23, 6, isNight ? 0x2a3a5a : 0x8ac0e0); fill(x - 11, 18, 23, 2, isNight ? 0x3a4a3a : 0x8ac06a);
    for (const [dx, top] of [[-9, 9], [-2, 11], [6, 8]]) {
      fill(x + dx, top + 6, 1, 18 - top - 6 + 2, 0x5a3a2e);
      disc(x + dx, top + 3, 4, isNight ? 0xb87898 : 0xf6c8d4); dot(x + dx - 2, top + 1, isNight ? 0xd8a0b8 : WHITE); dot(x + dx + 2, top + 4, isNight ? 0x9a5a7a : 0xe8a0b8);
    }
    const t = (now + x * 0.23) % 7;
    if (t < 2) for (let k = 0; k < 3; k++) dot(x - 10 + Math.floor(t * 10) + k * 4, 7 + k * 4 + Math.floor(t * 3), 0xf6c8d4);
    fill(x, 5, 1, 21, frame); fill(x - 11, 15, 23, 1, frame);
    fill(x - 13, 26, 27, 2, 0xe0d4c0);
    if (isNight) glowLate(() => { for (const lx of [-7, 2, 9]) { dot(x + lx, 17, 0xffd890); glowAt(x + lx, 16, 3, 3, 0xffc070, 0.25); } });
  },
  /** An open folding fan pinned to the wall, its paper painted with a blossom branch and a little bird on pale pink, a tassel hanging from its handle that sways now and then. */
  sakurafan(x) {
    const cy = 22;
    for (let k = 0; k <= 12; k++) { const a = Math.PI + (k / 12) * Math.PI; line(x, cy, x + Math.round(Math.cos(a) * 10), cy + Math.round(Math.sin(a) * 10), k % 2 ? 0xf6e0e6 : 0xf2d0da); }
    for (let k = 0; k <= 12; k++) { const a = Math.PI + (k / 12) * Math.PI; dot(x + Math.round(Math.cos(a) * 10), cy + Math.round(Math.sin(a) * 10), 0x8a5a32); }
    line(x - 6, 17, x + 3, 15, 0x5a3a2e); for (const [dx, dy] of [[-4, 16], [-1, 15], [2, 14], [-6, 17]]) { dot(x + dx, dy, WHITE); dot(x + dx, dy - 1, 0xf28fb0); }
    fill(x + 4, 17, 2, 2, 0x8ab06a); dot(x + 5, 17, WHITE);
    for (let k = 0; k < 3; k++) line(x, cy, x - 2 + k * 2, cy - 4, 0x8a5a32);
    const t = (now + x * 0.31) % 10, sw = t < 1.5 ? [0, 1, 0, -1][Math.floor(t * 4) % 4] : 0;
    line(x, cy, x + sw, cy + 4, 0xd8302a); fill(x - 1 + sw, cy + 4, 3, 2, 0xd8302a);
  },
  /** A little pole flying three carp streamers, black, red and blue, their scales painted on and big round eyes at their mouths; now and then a breeze fills them and they ripple out. */
  sakurastreamers(x) {
    fill(x - 10, 2, 1, 27, 0x8a8e98); disc(x - 10, 2, 1.2, 0xf2c41a);
    const t = (now + x * 0.21) % 9, wind = t < 3;
    [[0x2a2a3a, 5], [0xd8302a, 12], [0x2a5ab0, 19]].forEach(([c, cy], i) => {
      const len = 16 - i * 2;
      for (let k = 0; k < len; k++) {
        const wave = wind ? Math.round(Math.sin(now * 6 + k * 0.6)) : (k > len - 5 ? Math.floor((k - len + 5) / 2) : 0);
        fill(x - 9 + k, cy + wave, 1, 4 - (k > len - 3 ? 2 : 0), k % 3 === 1 ? shaded(c, 1.3) : c);
      }
      disc(x - 7, cy + 1.5, 1.2, WHITE); dot(x - 7, cy + 1, 0x1c1a22);
    });
  },
  /** A woodblock-style print in a thin frame: a snow-capped mountain over a lake under a pale sky, a blossoming branch across the top corner. */
  sakuraprint(x) {
    fill(x - 7, 4, 15, 22, 0x2a2a30); fill(x - 6, 5, 13, 20, 0xf2ead8);
    fill(x - 6, 5, 13, 9, 0xd8e8f0);
    for (let k = 0; k < 6; k++) fill(x - k, 9 + k, 1 + k * 2, 1, k < 2 ? WHITE : 0x3a5a8a);
    fill(x - 6, 15, 13, 3, 0x6a9ac0); fill(x - 6, 15, 13, 1, 0x8ab8d8);
    fill(x - 6, 18, 13, 7, 0xe8dcc0); fill(x - 4, 20, 9, 1, 0xd8c8a8);
    line(x - 6, 6, x - 1, 8, 0x5a3a2e); for (const [dx, dy] of [[-5, 6], [-3, 7], [-1, 8]]) { dot(x + dx, dy, 0xf6c8d4); dot(x + dx, dy + 1, 0xe87a98); }
    fill(x + 4, 22, 2, 2, 0xd8302a);
  },
  /** A hanging spring calendar on a nail, its page painted with a pink blossom, the days in neat rows; a red circle marks today and the page lifts in a draught now and then. */
  sakuracalendar(x) {
    dot(x, 4, 0x5a3b25); line(x - 2, 7, x, 4, 0x8a8e98); line(x + 2, 7, x, 4, 0x8a8e98);
    const t = (now + x * 0.33) % 15, lift = t < 0.8 ? 1 : 0;
    fill(x - 6, 7, 13, 18, WHITE); fill(x - 6, 7, 13, 7, 0xf6d8e0);
    disc(x, 10, 2.5, 0xf6b8c8); dot(x, 10, 0xf2c41a); line(x - 3, 12, x + 3, 13, 0x5a3a2e);
    for (let r = 0; r < 4; r++) for (let c = 0; c < 5; c++) dot(x - 5 + c * 2 + 1, 16 + r * 2, 0x8a8e98);
    const d = new Date().getDate(), cell = (d - 1) % 20; ring(x - 4 + (cell % 5) * 2, 16 + Math.floor(cell / 5) * 2, 1, 0xd8302a, 0.8);
    if (lift) fill(x - 6, 24, 13, 1, 0xe0e0e0);
    fill(x - 5, 25, 12, 1, SHADOW);
  },
  /** A portrait in a heavy gilt frame of an old wizard in a plum robe and pointed hat with a long white beard; now and then he waves, and every so often he gets up and leaves his frame, leaving an empty red chair, then comes back. */
  potterportrait(x) {
    const gold = 0xd3a625, goldLight = 0xf0cf5a, goldDark = 0x9a7418, bg = 0x2a3a2e, bgLight = 0x3e5040;
    const robe = 0x5a2a6a, skin = 0xe8c8a0, beard = 0xf0f0ec;
    fill(x - 8, 3, 17, 22, gold); fill(x - 8, 3, 17, 1, goldLight); fill(x - 8, 3, 1, 22, goldLight); fill(x + 8, 3, 1, 22, goldDark); fill(x - 8, 24, 17, 1, goldDark);
    for (const [cx, cy] of [[x - 8, 3], [x + 8, 3], [x - 8, 24], [x + 8, 24]]) dot(cx, cy, 0xfff0a0);
    fill(x - 6, 5, 13, 18, bg); fill(x - 6, 5, 13, 7, bgLight, 0.5);
    // The chair, then its sitter unless he's away
    fill(x - 4, 12, 9, 11, 0x7a2020); fill(x - 4, 12, 9, 1, 0x9a3030); fill(x - 5, 17, 1, 6, 0x6a1a1a); fill(x + 5, 17, 1, 6, 0x6a1a1a);
    const t = (now + x * 0.27) % 45, away = t > 32 && t < 39, wave = t < 3;
    if (!away) {
      fill(x - 3, 15, 7, 8, robe); fill(x - 3, 15, 1, 8, shaded(robe, 1.25)); fill(x + 3, 15, 1, 8, shaded(robe, 0.8));
      fill(x - 2, 10, 5, 4, skin); dot(x - 1, 11, PALETTE.eye); dot(x + 1, 11, PALETTE.eye);
      fill(x - 2, 13, 5, 3, beard); fill(x - 1, 16, 3, 3, beard); dot(x, 19, beard);
      fill(x - 3, 9, 7, 1, robe); fill(x - 2, 8, 5, 1, robe); fill(x - 1, 7, 3, 1, robe); dot(x + 1, 6, robe); dot(x + 2, 5, robe);
      dot(x - 2, 8, shaded(robe, 1.3)); dot(x, 7, 0xf2d35b);
      if (wave) { const up = Math.floor(now * 5) % 2; fill(x + 4, 12 - up, 2, 2, skin); fill(x + 4, 14 - up, 2, 2, robe); }
      else fill(x + 3, 19, 2, 1, skin);
    }
    fill(x - 7, 25, 15, 1, SHADOW);
  },
  /** The four house-points hourglasses: tall glass tubes on a dark oak stand, filled with rubies, emeralds, yellow diamonds and sapphires, Gryffindor ahead; every 20 s a few gems tumble into one of them as points are given. */
  potterhousepoints(x) {
    const cols = [0xd8302a, 0x2aa84a, 0xf2c830, 0x3a6ad8], wood = 0x4a3020, woodLight = 0x6a4a30, brass = 0xc8a050;
    const cycle = Math.floor((now + x * 0.3) / 20), t = (now + x * 0.3) % 20, who = cycle % 4;
    fill(x - 12, 2, 25, 2, wood); fill(x - 12, 2, 25, 1, woodLight);
    fill(x - 12, 24, 25, 3, wood); fill(x - 12, 24, 25, 1, woodLight); fill(x - 12, 27, 25, 1, SHADOW);
    for (let k = 0; k < 4; k++) {
      const tx = x - 11 + k * 6, c = cols[k], level = [13, 9, 7, 10][k] + (k === who && t > 2 ? 1 : 0);
      fill(tx, 4, 5, 20, 0xdff0f6, 0.3);
      fill(tx + 1, 24 - level, 3, level, c);
      for (let yy = 24 - level; yy < 24; yy++) dot(tx + 1 + ((yy + k) % 3), yy, shaded(c, (yy + k) % 2 ? 1.35 : 0.75));
      fill(tx, 4, 1, 20, 0xdff0f6, 0.6); fill(tx + 4, 4, 1, 20, 0xdff0f6, 0.45); dot(tx + 1, 6, WHITE, 0.7);
      fill(tx, 4, 5, 1, brass); fill(tx, 23, 5, 1, brass, 0.8);
      if (k === who && t < 2) for (let g = 0; g < 3; g++) { const gy = 5 + Math.floor((t * 9 + g * 4) % (19 - level)); dot(tx + 1 + g, gy, shaded(c, 1.3)); }
    }
  },
  /** A pointed Gothic castle window with leaded panes and a stone surround: hills and the sky by day, a deep blue night full of stars and a moon; every 25 s a Quidditch player in scarlet streaks past on a broom, chasing the snitch. */
  potterwindow(x) {
    const stone = 0x8a857c, stoneLight = 0xa8a298, stoneDark = 0x5e5a54, sky = isNight ? 0x16204a : 0x9cc8ec, skyLow = isNight ? 0x24306a : 0xc8e0f2;
    const top = 3, rows = 23, half = (r) => r >= 12 ? 8 : Math.max(0, Math.round(Math.sqrt(169 - (12.4 - r) ** 2) - 4.5));
    // The stone surround, then the glass
    for (let r = -1; r < rows + 1; r++) { const h = half(Math.max(0, r)) + 2; fill(x - h, top + r, h * 2 + 1, 1, stone); dot(x - h, top + r, stoneLight); dot(x + h, top + r, stoneDark); }
    for (let r = 0; r < rows; r++) { const h = half(r); if (h > 0) fill(x - h, top + r, h * 2 + 1, 1, r > 14 ? skyLow : sky); }
    // Hills, and the stars and moon at night
    fill(x - 8, top + 19, 17, 4, isNight ? 0x1a2a24 : 0x5a8a4a); fill(x - 8, top + 18, 8, 1, isNight ? 0x1a2a24 : 0x5a8a4a); fill(x + 3, top + 17, 3, 2, isNight ? 0x22303a : 0x6a6a72);
    if (isNight) {
      for (const [sx, sy] of [[-5, 8], [3, 6], [6, 12], [-2, 12], [0, 4]]) dot(x + sx, top + sy, 0xfff8d0, 0.8);
      disc(x - 4, top + 13, 1.6, 0xf4eec8);
      glowLate(() => { glowAt(x + 4, top + 15, 1, 1, 0xffd070, 0.6); dot(x + 4, top + 15, 0xffe8a0); });
    }
    // The rider streaking past, behind the leading
    const t = (now + x * 0.37) % 25;
    if (t < 1.6) {
      const rx = x - 9 + Math.round(t / 1.6 * 19), ry = top + 10 + Math.round(Math.sin(t * 4) * 2);
      const inside = (px, py) => Math.abs(px - x) <= half(py - top) && py - top < rows && py >= top;
      for (const [dx, dy, c] of [[-3, 1, 0x6a3a1a], [-2, 1, 0x6a3a1a], [-1, 1, 0x6a3a1a], [0, 1, 0x6a3a1a], [-4, 1, 0xb08a4a], [-4, 2, 0xb08a4a], [-1, 0, 0xa8201c], [0, 0, 0xa8201c], [-1, -1, 0xa8201c], [0, -2, 0xe8c8a0]]) if (inside(rx + dx, ry + dy)) dot(rx + dx, ry + dy, c);
      if (inside(rx + 4, ry - 2)) { dot(rx + 4, ry - 2, 0xf2c830); dot(rx + 5, ry - 3, 0xe8e8f0, 0.7); }
    }
    // The leading: a mullion, a transom and diamond panes
    fill(x, top + 2, 1, rows - 2, stoneDark, 0.9); fill(x - 8, top + 13, 17, 1, stoneDark, 0.8);
    for (let r = 2; r < rows; r += 4) { const h = half(r); for (let d = -h; d <= h; d += 4) dot(x + d + (r % 8 ? 2 : 0), top + r, 0x3a3a40, 0.35); }
    fill(x - 10, top + rows + 1, 21, 1, stoneLight); fill(x - 10, top + rows + 2, 21, 1, SHADOW);
  },
  /** A Nimbus racing broom resting on two brass hooks, its gold name on the handle and its twigs neatly bound, with a scarlet-and-gold scarf hanging from one hook; every 30 s the broom twitches and floats up off its hooks, eager to fly. */
  potterbroom(x) {
    const handle = 0x6a3a1a, handleLight = 0x8a5a2a, twig = 0xb08a4a, twigDark = 0x8a6a34, brass = 0xc8a050, gold = 0xd3a625, red = 0x8a1e1e;
    const t = (now + x * 0.29) % 30, hov = t < 3 ? 1 + (Math.floor(t * 6) % 2) : 0;
    for (const hx of [x - 8, x + 4]) { dot(hx, 15, brass); dot(hx, 14, brass); dot(hx + 1, 14, 0xe8c878); }
    // The scarf on the left hook
    for (let r = 0; r < 11; r++) { const c = r % 4 < 2 ? red : gold; fill(x - 9, 15 + r, 2, 1, c); fill(x - 7, 15 + r + (r > 6 ? 1 : 0), 2, 1, c); }
    for (const fx of [x - 9, x - 8, x - 7, x - 6]) dot(fx, fx % 2 ? 26 : 27, gold, 0.8);
    // The broom
    const by = 12 - hov;
    fill(x - 13, by, 19, 1, handleLight); fill(x - 13, by + 1, 19, 1, handle); dot(x - 13, by + 1, 0x4a2a10);
    for (const gx of [x - 7, x - 5, x - 4, x - 2]) dot(gx, by, gold);
    fill(x + 6, by - 1, 2, 4, 0x8a6a2a); dot(x + 6, by - 1, 0xc8a050);
    for (let k = 0; k < 5; k++) line(x + 8, by - 1 + k, x + 13, by - 3 + k * 2, k % 2 ? twig : twigDark);
    if (hov) fill(x - 12, 16, 22, 1, SHADOW);
  },
  /** The Marauder's Map pinned open: folded parchment with corridors and rooms drawn in brown ink, and little pairs of footprints walking the corridors, appearing and fading as they go. */
  pottermap(x) {
    const parch = 0xeadcb4, parchLight = 0xf6ecd0, parchDark = 0xc8b890, ink = 0x5a4030;
    fill(x - 11, 5, 23, 19, parch); fill(x - 11, 5, 23, 1, parchLight); fill(x + 11, 5, 1, 19, parchDark); fill(x - 11, 23, 23, 1, parchDark);
    fill(x - 4, 5, 1, 19, parchDark, 0.7); fill(x + 4, 5, 1, 19, parchDark, 0.7);
    // The title band and the castle's corridors and rooms
    fill(x - 9, 7, 19, 1, ink, 0.35);
    for (const [rx, ry, w, h] of [[-9, 10, 5, 4], [-2, 10, 5, 3], [5, 10, 5, 5], [-9, 17, 4, 4], [-1, 16, 6, 5], [7, 18, 3, 3]]) {
      fill(x + rx, ry, w, 1, ink, 0.7); fill(x + rx, ry + h, w, 1, ink, 0.7); fill(x + rx, ry, 1, h, ink, 0.7); fill(x + rx + w - 1, ry, 1, h + 1, ink, 0.7);
    }
    fill(x - 9, 15, 19, 1, ink, 0.45); fill(x - 3, 10, 1, 11, ink, 0.45);
    // The footprints, two walkers
    const path = [[-8, 15], [-6, 15], [-4, 15], [-3, 14], [-3, 12], [-3, 10], [-1, 9], [1, 9], [3, 9], [5, 9], [7, 9], [9, 9], [9, 11], [9, 13], [9, 15], [7, 15], [5, 15], [3, 15], [1, 15], [-1, 15]];
    [0, 9].forEach((off) => {
      const i = (Math.floor(now * 1.5 + x * 0.1) + off) % path.length;
      for (let back = 0; back < 3; back++) {
        const [px, py] = path[(i - back + path.length) % path.length];
        dot(x + px, py + (back % 2), ink, 0.85 - back * 0.25);
      }
    });
    dot(x - 11, 5, 0xc83a2a); dot(x + 11, 5, 0xc83a2a);
    fill(x - 10, 24, 22, 1, SHADOW);
  },
  /** The four house banners on a black iron rail: scarlet and gold, green and silver, yellow and black, blue and bronze, each with its swallowtail and a little beast's shape; now and then a draught sets them swaying. */
  potterbanners(x) {
    const houses = [[0x8a1e1e, 0xd3a625], [0x1a5a32, 0xb8bcc4], [0xe8b830, 0x2a2420], [0x1e3a7a, 0xb07a3a]];
    fill(x - 14, 3, 29, 1, 0x2a2a30); dot(x - 14, 2, 0x4a4a52); dot(x + 14, 2, 0x4a4a52);
    const t = (now + x * 0.23) % 16, sw = t < 2 ? [0, 1, 0, -1][Math.floor(t * 4) % 4] : 0;
    houses.forEach(([c, trim], k) => {
      const bx = x - 13 + k * 7;
      fill(bx, 4, 6, 15, c); fill(bx, 4, 1, 15, shaded(c, 1.2)); fill(bx + 5, 4, 1, 15, shaded(c, 0.8));
      fill(bx + sw * (k % 2 ? 1 : 0), 19, 2, 2, c); fill(bx + 4 + sw * (k % 2 ? 1 : 0), 19, 2, 2, c);
      dot(bx + sw * (k % 2 ? 1 : 0), 21, c); dot(bx + 5 + sw * (k % 2 ? 1 : 0), 21, c);
      fill(bx, 4, 6, 1, trim); fill(bx, 18, 6, 1, trim, 0.8);
      // The beast's shape: lion, serpent, badger, eagle
      const shapes = [[[2, 8], [3, 8], [2, 9], [3, 9], [4, 10], [1, 10], [2, 11], [3, 11], [1, 7]], [[1, 8], [2, 9], [3, 10], [4, 11], [3, 12], [2, 13], [1, 7]], [[1, 9], [2, 9], [3, 9], [4, 9], [1, 10], [4, 10], [2, 8], [3, 8]], [[2, 8], [3, 8], [1, 9], [4, 9], [0, 10], [5, 10], [2, 10], [3, 10], [2, 11]]];
      for (const [dx, dy] of shapes[k]) dot(bx + dx, dy + 1, trim);
    });
    fill(x - 13, 22, 27, 1, SHADOW);
  },
  /** A round hobbit window in a green frame, looking out on the rolling green hills of the Shire under a blue sky (a starry one at night), a little smial with a yellow door on the far hill; a butterfly flits past every 20 s. */
  lotrwindow(x) {
    const frame = 0x3a6a3a, frameLight = 0x5a8a4a, frameDark = 0x2a4a2a, cy = 14;
    const sky = isNight ? 0x1a2650 : 0xa8d4f0, hill = isNight ? 0x24402a : 0x6aaa4a, hillFar = isNight ? 0x2a4a34 : 0x8ac06a;
    for (let dy = -10; dy <= 10; dy++) for (let dx = -10; dx <= 10; dx++) {
      const r = Math.hypot(dx, dy);
      if (r > 10.2) continue;
      let c = r > 8.2 ? (dx + dy < -3 ? frameLight : dx + dy > 3 ? frameDark : frame) : sky;
      if (r <= 8.2) {
        const far = cy + 3 - Math.round(Math.sin((dx + 10) * 0.35) * 2), near = cy + 6 - Math.round(Math.cos((dx + 4) * 0.3) * 2);
        if (cy + dy >= near) c = hill; else if (cy + dy >= far) c = hillFar;
      }
      dot(x + dx, cy + dy, c);
    }
    fill(x - 8, cy, 17, 1, frame); fill(x, cy - 8, 1, 17, frame);
    // The smial on the far hill: a round door in the green
    dot(x + 4, cy + 2, isNight ? 0xffd070 : 0xf2c830); dot(x + 5, cy + 2, isNight ? 0xffd070 : 0xf2c830);
    if (isNight) {
      for (const [sx, sy] of [[-5, -5], [3, -6], [-2, -3], [6, -2]]) dot(x + sx, cy + sy, 0xfff8d0, 0.8);
      glowLate(() => { glowAt(x + 4, cy + 2, 2, 2, 0xffc060, 0.4); dot(x + 4, cy + 2, 0xffe8a0); });
    } else dot(x - 4, cy - 5, WHITE, 0.8);
    const t = (now + x * 0.29) % 20;
    if (t < 3 && !isNight) { const bx = x - 7 + Math.round(t * 5), by = cy - 1 + Math.round(Math.sin(t * 6) * 2); if (Math.hypot(bx - x, by - cy) < 8) { dot(bx, by, 0xf2a030); if (Math.floor(now * 8) % 2) dot(bx + 1, by - 1, 0xf2a030); } }
    fill(x - 6, 25, 13, 1, SHADOW);
  },
  /** The map of Middle-earth on old parchment, pinned with brass tacks: inked mountain ranges, the Shire in green, a river, and Mordor in the east, where Mount Doom's tiny red fire glows and pulses. */
  lotrmap(x) {
    const parch = 0xe8d8b0, parchDark = 0xc8b48a, ink = 0x5a4030;
    fill(x - 12, 5, 25, 19, parch); fill(x - 12, 5, 25, 1, 0xf4e8c8); fill(x + 12, 5, 1, 19, parchDark); fill(x - 12, 23, 25, 1, parchDark);
    dot(x - 12, 5, parchDark); dot(x + 12, 23, 0xb8a47a);
    // The Shire, the Misty Mountains, a river, the mountains round Mordor
    fill(x - 10, 14, 4, 3, 0x7aa05a, 0.7); dot(x - 9, 15, 0x5a8a3a);
    for (let k = 0; k < 6; k++) { dot(x - 4, 8 + k * 2, ink); dot(x - 5, 9 + k * 2, ink, 0.6); dot(x - 3, 9 + k * 2, ink, 0.6); }
    for (let k = 0; k < 10; k++) dot(x - 8 + k, 19 + Math.round(Math.sin(k * 0.8)), 0x4a7ac8, 0.8);
    for (let k = 0; k < 4; k++) { dot(x + 4 + k * 2, 10, ink); dot(x + 4 + k * 2, 18, ink); }
    for (let k = 0; k < 4; k++) { dot(x + 4, 11 + k * 2, ink); dot(x + 11, 11 + k * 2, ink); }
    fill(x + 5, 11, 6, 7, 0x6a5a4a, 0.35);
    const pulse = 0.6 + 0.4 * Math.sin(now * 2 + x);
    dot(x + 8, 14, 0xe8402a, pulse); dot(x + 8, 13, 0xffa040, pulse * 0.7);
    fill(x - 8, 6, 9, 1, ink, 0.4);
    for (const [px, py] of [[x - 12, 5], [x + 12, 5]]) dot(px, py, 0xc8a050);
    if (isNight) glowLate(() => glowAt(x + 8, 14, 2, 2, 0xff6020, 0.3 * pulse));
    fill(x - 11, 24, 24, 1, SHADOW);
  },
  /** A dark painting of Barad-dûr: the black tower against a red sky with the lidless Eye burning at its top; the Eye's pupil slowly looks from side to side, and at night its fire glows through the dark. */
  lotreye(x) {
    const frame = 0x2a2420, frameLight = 0x4a403a, sky = 0x8a2a1a, skyDark = 0x5a1a10, tower = 0x141010;
    fill(x - 9, 3, 19, 23, frame); fill(x - 9, 3, 19, 1, frameLight); fill(x - 9, 3, 1, 23, frameLight);
    fill(x - 7, 5, 15, 19, skyDark); fill(x - 7, 5, 15, 9, sky); fill(x - 7, 13, 15, 3, 0xa8401a, 0.6);
    // The tower: a tapering spire with two horns at the top
    fill(x - 2, 12, 5, 12, tower); fill(x - 1, 9, 3, 3, tower); fill(x - 3, 21, 7, 3, tower);
    dot(x - 2, 7, tower); dot(x - 2, 8, tower); dot(x + 2, 7, tower); dot(x + 2, 8, tower);
    fill(x - 6, 22, 13, 2, 0x2a1810);
    // The Eye
    const look = Math.round(Math.sin((now + x * 0.2) * 0.4) * 1.2);
    const eye = () => { fill(x - 2, 6, 5, 1, 0xff8a20); fill(x - 1, 5, 3, 1, 0xffc040); fill(x - 1, 7, 3, 1, 0xffc040); dot(x - 3, 6, 0xe8501a); dot(x + 3, 6, 0xe8501a); dot(x + look, 6, 0x1a0808); dot(x + look, 5, 0x3a1008); };
    eye();
    glowAt(x, 6, 5, 3, 0xff7020, 0.12);
    if (isNight) glowLate(() => { glowAt(x, 6, 6, 4, 0xff6010, 0.3); eye(); });
    fill(x - 8, 26, 17, 1, SHADOW);
  },
  /** Sting, Bilbo's elven short-sword, hung across a dark wooden plaque with the mithril shirt glinting beside it; every 30 s its blade glows blue, as if orcs were near. */
  lotrsting(x) {
    const wood = 0x4a3020, woodLight = 0x6a4a30, steel = 0xc8d0d8, steelLight = 0xf0f4f8, steelDark = 0x8a929c, leather = 0x5a3a22, mithril = 0xd8e0e8;
    fill(x - 11, 7, 22, 14, wood); fill(x - 11, 7, 22, 1, woodLight); fill(x + 10, 7, 1, 14, 0x2e1c12);
    const t = (now + x * 0.27) % 30, glow = t < 4 ? Math.min(1, t, 4 - t) : 0;
    // The blade, crossguard and hilt
    fill(x - 8, 11, 13, 2, steel); fill(x - 8, 11, 13, 1, steelLight); dot(x - 9, 12, steel); dot(x + 4, 12, steelDark);
    fill(x + 5, 9, 1, 6, 0xb8bcc4); fill(x + 6, 11, 3, 2, leather); dot(x + 9, 11, 0xb8bcc4); dot(x + 9, 12, 0xb8bcc4);
    if (glow) {
      fill(x - 8, 11, 13, 2, 0x6ab0ff, 0.7 * glow); glowAt(x - 2, 12, 9, 3, 0x6ab0ff, 0.2 * glow);
      if (isNight) glowLate(() => { glowAt(x - 2, 12, 9, 4, 0x4a90ff, 0.3 * glow); fill(x - 8, 11, 13, 2, 0x9ad0ff, 0.6 * glow); });
    }
    // The mithril shirt below, glinting
    fill(x - 7, 15, 9, 5, mithril); fill(x - 9, 15, 2, 3, mithril); fill(x + 2, 15, 2, 3, mithril);
    for (let k = 0; k < 9; k += 2) dot(x - 7 + k, 17, 0xb8c0c8);
    if ((now + x * 0.13) % 6 < 0.4) dot(x - 4, 16, WHITE);
    fill(x - 10, 21, 21, 1, SHADOW);
  },
  /** The black banner of Gondor: the White Tree in flower beneath seven stars and a crown, hung from a silver rod; it stirs now and then. */
  lotrgondor(x) {
    const black = 0x14161c, blackLight = 0x2a2e38, white = 0xf2f2ee, silver = 0xb8bcc4;
    fill(x - 8, 3, 17, 1, silver); dot(x - 9, 3, 0x8a8e96); dot(x + 9, 3, 0x8a8e96);
    const t = (now + x * 0.31) % 18, sw = t < 2 ? [0, 1, 0, -1][Math.floor(t * 4) % 4] : 0;
    fill(x - 6, 4, 13, 20, black); fill(x - 6, 4, 1, 20, blackLight);
    fill(x - 6 + sw, 24, 13, 1, black); for (let k = 0; k < 13; k += 3) dot(x - 6 + k + sw, 25, black);
    // The stars and crown, then the tree
    for (const [sx, sy] of [[-4, 6], [-2, 5], [0, 5], [2, 5], [4, 6], [-3, 7], [3, 7]]) dot(x + sx, sy, white, 0.9);
    fill(x - 1, 8, 3, 1, silver); dot(x - 1, 7, silver); dot(x + 1, 7, silver);
    fill(x, 11, 1, 10, white); fill(x - 2, 21, 5, 1, white);
    for (const [bx, by, dir] of [[0, 12, -1], [0, 12, 1], [0, 15, -1], [0, 15, 1], [0, 18, -1], [0, 18, 1]]) line(x, by, x + dir * (by < 14 ? 3 : by < 17 ? 4 : 3), by - 2, white);
    fill(x - 5, 26, 11, 1, SHADOW);
  },
  /** The green banner of Rohan with its white horse running, gold-edged, from a wooden pole; a breeze lifts it every 15 s. */
  lotrrohan(x) {
    const green = 0x2a6a3a, greenLight = 0x3a8a4a, white = 0xf2f2ee, gold = 0xd8b45a;
    fill(x - 8, 3, 17, 1, 0x6a4a2a); dot(x - 9, 3, 0x8a5a32); dot(x + 9, 3, 0x8a5a32);
    const t = (now + x * 0.19) % 15, lift = t < 2 ? Math.floor(t * 4) % 2 : 0;
    fill(x - 6, 4, 13, 18, green); fill(x - 6, 4, 1, 18, greenLight); fill(x - 6, 4, 13, 1, gold); fill(x - 6, 21, 13, 1, gold);
    fill(x - 6, 22, 5, 2 - lift, green); fill(x + 2, 22, 5, 2 + lift, green);
    // The running horse
    fill(x - 3, 12, 6, 3, white); fill(x + 2, 9, 2, 4, white); fill(x + 3, 9, 2, 2, white); dot(x + 4, 8, white);
    line(x - 3, 15, x - 5, 18, white); line(x - 2, 15, x - 1, 18, white); line(x + 2, 15, x + 3, 18, white); line(x + 1, 15, x + 4, 17, white);
    line(x - 3, 12, x - 5, 14, white); dot(x + 1, 9, white, 0.8);
    fill(x - 5, 25, 11, 1, SHADOW);
  },
  /** The show's title as two green periodic-table tiles, Br and Ba, with their little atomic numbers, on a smoky backboard; a wisp of yellow-green smoke curls past now and then. */
  breakingbadlogo(x) {
    const back = 0x2a3a2a, green = 0x2a8a3a, greenLight = 0x5aba6a, greenDark = 0x1a5a2a, W = 0xf2f2ee;
    fill(x - 13, 6, 27, 18, back); fill(x - 13, 6, 27, 1, 0x3a4a3a); fill(x - 13, 24, 27, 1, SHADOW);
    const tile = (tx, ty) => { fill(tx, ty, 10, 10, green); fill(tx, ty, 10, 1, greenLight); fill(tx, ty, 1, 10, greenLight); fill(tx + 9, ty, 1, 10, greenDark); fill(tx, ty + 9, 10, 1, greenDark); };
    tile(x - 12, 8); tile(x + 1, 12);
    // "Br" and "Ba", with "35" and "56"
    const B = (bx, by) => { fill(bx, by, 1, 5, W); fill(bx + 1, by, 1, 1, W); fill(bx + 2, by + 1, 1, 1, W); fill(bx + 1, by + 2, 1, 1, W); fill(bx + 2, by + 3, 1, 1, W); fill(bx + 1, by + 4, 1, 1, W); };
    B(x - 10, 11); fill(x - 7, 13, 1, 3, W); dot(x - 6, 13, W);
    B(x + 3, 15); fill(x + 6, 17, 2, 1, W); dot(x + 7, 18, W); fill(x + 6, 19, 2, 1, W); dot(x + 6, 18, W, 0.6);
    for (const [nx, ny] of [[x - 4, 9], [x - 3, 9], [x + 9, 13], [x + 10, 13]]) dot(nx, ny, W, 0.8);
    const t = (now + x * 0.21) % 14;
    if (t < 3) for (let k = 0; k < 4; k++) { const ph = t / 3; dot(x - 10 + k * 5 + Math.round(ph * 6), 22 - Math.round(ph * 10) - k, 0xc8e070, 0.5 * (1 - ph)); }
  },
  /** The Los Pollos Hermanos sign: two cartoon chickens side by side on a red and yellow board; now and then one of the brothers gives a wink. */
  breakingbadpollos(x) {
    const red = 0xc8281e, yellow = 0xf2c830, orange = 0xe8782e, W = 0xf6f2ea;
    fill(x - 11, 5, 23, 18, yellow); fill(x - 11, 5, 23, 1, 0xfff0a0); fill(x - 11, 22, 23, 1, 0xc89a1a);
    fill(x - 10, 6, 21, 3, red); fill(x - 10, 19, 21, 3, red);
    for (let k = -9; k < 10; k += 2) { dot(x + k, 7, W); dot(x + k, 20, W); }
    const wink = (now + x * 0.3) % 9 < 0.4;
    for (const [cx, w] of [[x - 4, false], [x + 4, wink]]) {
      fill(cx - 3, 11, 6, 7, W); fill(cx - 2, 10, 4, 1, W); fill(cx - 1, 9, 2, 1, red); dot(cx, 8, red);
      fill(cx - 3, 16, 6, 2, 0xe8e2d0); dot(cx + 3, 13, orange); dot(cx + 4, 13, orange); dot(cx + 3, 14, red);
      if (w) fill(cx, 12, 2, 1, 0x2a2a30); else { dot(cx, 12, 0x2a2a30); dot(cx + 1, 12, 0x2a2a30, 0.5); }
    }
    fill(x - 10, 23, 21, 1, SHADOW);
  },
  /** A police sketch of Heisenberg pinned to the wall: a pale paper with the porkpie hat, dark glasses and goatee drawn in charcoal; the paper's corner lifts in a draught now and then. */
  breakingbadheisenberg(x) {
    const paper = 0xf2eee4, ink = 0x2a2a30, shade = 0x8a8a90;
    fill(x - 7, 4, 15, 20, paper); fill(x - 7, 4, 15, 1, WHITE); fill(x + 7, 4, 1, 20, 0xd8d4c8); fill(x - 7, 23, 15, 1, 0xd8d4c8);
    // The hat
    fill(x - 5, 10, 11, 1, ink); fill(x - 3, 7, 7, 3, ink); fill(x - 3, 9, 7, 1, 0x5a5a60);
    // The face, glasses, goatee
    fill(x - 3, 11, 7, 7, 0xe0dcd2); fill(x - 3, 11, 1, 7, shade, 0.5);
    fill(x - 3, 13, 3, 2, ink); fill(x + 1, 13, 3, 2, ink); dot(x, 13, ink);
    fill(x - 1, 17, 3, 2, ink); fill(x - 2, 16, 5, 1, shade); dot(x, 19, ink);
    fill(x - 4, 19, 9, 3, shade, 0.4);
    const lift = (now + x * 0.29) % 13 < 1 ? 1 : 0;
    if (lift) { fill(x + 5, 21, 3, 3, paper); dot(x + 6, 20, 0xd8d4c8); }
    dot(x, 4, 0xc8281e);
    fill(x - 6, 24, 14, 1, SHADOW);
  },
  /** A loud "Better Call Saul!" poster: Saul's grinning face and pointing finger beside big red-white-and-blue letters on a starry band; it flickers like cheap TV now and then. */
  breakingbadsaul(x) {
    const blue = 0x2a3a8a, red = 0xc8281e, W = 0xf6f2ea, skin = 0xe8c0a0;
    fill(x - 12, 5, 25, 19, W); fill(x - 12, 5, 25, 4, blue); fill(x - 12, 20, 25, 4, red);
    for (let k = -11; k < 13; k += 3) dot(x + k, 6 + (k % 2 ? 1 : 0), W);
    // Saul
    fill(x - 10, 10, 6, 6, skin); fill(x - 10, 9, 6, 2, 0x6a4a2a); dot(x - 9, 12, 0x2a2a30); dot(x - 6, 12, 0x2a2a30); fill(x - 9, 14, 4, 1, 0xc85a5a);
    fill(x - 11, 16, 8, 4, 0x6a8ac8); dot(x - 7, 16, red); dot(x - 7, 17, red); fill(x - 4, 13, 3, 1, skin);
    // "CALL" and "SAUL!" in blocky letters
    const letters = (lx, ly, c) => { for (let k = 0; k < 4; k++) { fill(lx + k * 4, ly, 3, 4, c); fill(lx + k * 4 + 1, ly + 1, 1, 2, W); } };
    letters(x - 1, 10, blue); letters(x - 1, 15, red); fill(x + 15, 15, 1, 3, red);
    const t = (now + x * 0.17) % 11;
    if (t < 0.5) fill(x - 12, 5 + Math.floor(t * 30) % 19, 25, 2, WHITE, 0.6);
    fill(x - 11, 24, 24, 1, SHADOW);
  },
  /** A window onto the New Mexico desert: a red mesa under a huge blue sky with a vapour trail, a sunset-orange glow and stars at night; every 30 s the old RV trundles along the dirt road, a puff of dust behind it. */
  breakingbadwindow(x) {
    const frame = 0xd8d0c0, frameDark = 0xa89a88, sky = isNight ? 0x1a2040 : 0x6ab0f0, skyLow = isNight ? 0x3a2a4a : 0xf0c890;
    fill(x - 13, 5, 27, 20, frame); fill(x - 13, 5, 27, 1, WHITE); fill(x + 13, 5, 1, 20, frameDark);
    fill(x - 11, 7, 23, 16, sky); fill(x - 11, 16, 23, 3, skyLow);
    // The mesa and the ground
    fill(x - 9, 14, 10, 4, isNight ? 0x3a2228 : 0xb0502a); fill(x - 8, 13, 8, 1, isNight ? 0x3a2228 : 0xc8603a); fill(x + 3, 16, 6, 2, isNight ? 0x3a2228 : 0xa04a28);
    fill(x - 11, 18, 23, 5, isNight ? 0x2a2420 : 0xd8a870); fill(x - 11, 20, 23, 1, isNight ? 0x3a3028 : 0xc8986a);
    if (isNight) { for (const [sx, sy] of [[-8, 9], [-2, 8], [4, 10], [9, 8], [6, 12]]) dot(x + sx, sy, 0xfff8d0, 0.9); }
    else { line(x - 6, 10, x + 6, 8, WHITE, 0.6); disc(x + 8, 11, 1.5, 0xfff4c0); }
    // The RV on the road
    const t = (now + x * 0.31) % 30;
    if (t < 5) {
      const rx = x - 13 + Math.round(t / 5 * 26);
      const vis = (px) => px >= x - 11 && px <= x + 11;
      for (let k = 0; k < 6; k++) if (vis(rx + k)) { fill(rx + k, 18, 1, 2, k < 5 ? 0xf2ead8 : 0x6a8ab0); dot(rx + k, 19, 0x8a6a4a); }
      if (vis(rx - 2)) dot(rx - 2, 19, 0xe8d0a8, 0.6); if (vis(rx - 3)) dot(rx - 3, 18, 0xe8d0a8, 0.4);
    }
    fill(x, 7, 1, 16, frame);
    fill(x - 13, 25, 27, 1, SHADOW);
  },
  /** Two respirator gas masks hung on steel hooks beside a yellow hazmat hood, the masks' filters round and pink; now and then one sways on its hook. */
  breakingbadmasks(x) {
    const steel = 0x8a8e96, mask = 0x2a2a30, maskLight = 0x4a4a52, filter = 0xe890a8, yellow = 0xe8c838, yellowDark = 0xb8941a, glass = 0x8ab0c8;
    fill(x - 10, 6, 21, 1, steel); for (const hx of [x - 7, x, x + 7]) { dot(hx, 7, steel); dot(hx, 8, steel); }
    const sw = (now + x * 0.23) % 12 < 1.2 ? [0, 1, 0, -1][Math.floor(now * 5) % 4] : 0;
    for (const [mx, s] of [[x - 7, sw], [x + 7, 0]]) {
      const cx = mx + s;
      fill(cx - 3, 9, 7, 6, mask); fill(cx - 3, 9, 7, 1, maskLight);
      fill(cx - 2, 10, 2, 2, glass); fill(cx + 1, 10, 2, 2, glass); dot(cx - 2, 10, WHITE, 0.7);
      fill(cx - 4, 13, 2, 3, filter); fill(cx + 3, 13, 2, 3, filter); fill(cx - 1, 14, 3, 2, maskLight);
    }
    // The hazmat hood
    fill(x - 3, 9, 7, 11, yellow); fill(x - 3, 9, 1, 11, 0xf8e070); fill(x + 3, 9, 1, 11, yellowDark); fill(x - 2, 11, 5, 4, glass, 0.8); dot(x - 2, 11, WHITE, 0.7);
    fill(x - 9, 21, 19, 1, SHADOW);
  },
  /** Joyce's alphabet wall: the letters painted in black on the wallpaper under a string of coloured Christmas bulbs; now and then the bulbs light one after another to spell R, U, N, and at night they glow. */
  strangerlights(x) {
    const cols = [0xe8402a, 0x3ac85a, 0x3a8ae8, 0xf2c830, 0xe880c8];
    const t = (now + x * 0.13) % 20, word = [17, 20, 13], lit = t < 4.5 ? word[Math.min(2, Math.floor(t / 1.5))] : -1;
    for (let k = 0; k < 26; k++) {
      const row = Math.floor(k / 9), col = k % 9, lx = x - 14 + col * 3 + (row === 1 ? 1 : 0), ly = 10 + row * 7;
      const sag = Math.round(Math.sin((col / 8) * Math.PI) * 1);
      const by = ly - 3 + sag, c = cols[k % 5], on = k === lit;
      dot(lx, by, on ? WHITE : c, on ? 1 : 0.85);
      if (on) { glowAt(lx, by, 3, 3, c, 0.35); if (isNight) glowLate(() => { glowAt(lx, by, 4, 4, c, 0.4); dot(lx, by, WHITE); }); }
      else if (isNight) glowLate(() => dot(lx, by, c, 0.7));
      dot(lx, ly, 0x1e1e22); dot(lx, ly + 1, 0x1e1e22, 0.8); if (k % 2) dot(lx + 1, ly, 0x1e1e22, 0.7);
    }
    for (let row = 0; row < 3; row++) for (let k = 0; k < 27; k++) dot(x - 14 + k, 7 + row * 7 + Math.round(Math.sin((k / 26) * Math.PI)), 0x3a5a2a, 0.6);
  },
  /** The Hellfire Club's banner: black cloth with a red horned demon skull over two crossed swords and a red border; it stirs now and then. */
  strangerhellfire(x) {
    const black = 0x14141a, red = 0xc8281e, redDark = 0x8a1a14, bone = 0xe8dcc8;
    fill(x - 9, 3, 19, 1, 0x4a4a52);
    const sw = (now + x * 0.21) % 14 < 1.5 ? [0, 1, 0, -1][Math.floor(now * 4) % 4] : 0;
    fill(x - 8, 4, 17, 20, black); fill(x - 8, 4, 17, 1, red); fill(x - 8, 4, 1, 20, red); fill(x + 8, 4, 1, 20, red);
    fill(x - 8 + sw, 24, 8, 2, black); fill(x + 1 + sw, 24, 8, 2, black); dot(x + sw, 24, red);
    line(x - 6, 21, x + 5, 9, 0xb8bcc4); line(x + 6, 21, x - 5, 9, 0xb8bcc4);
    fill(x - 3, 11, 7, 6, red); fill(x - 2, 17, 5, 2, red); dot(x - 4, 10, redDark); dot(x - 5, 9, redDark); dot(x + 4, 10, redDark); dot(x + 5, 9, redDark);
    fill(x - 2, 13, 2, 2, black); fill(x + 1, 13, 2, 2, black); dot(x, 15, black); for (const dx of [-1, 1]) dot(x + dx, 18, bone);
    fill(x - 7, 26, 15, 1, SHADOW);
  },
  /** A suburban window looking onto Hawkins: trees and a blue sky by day; at night the sky turns the Upside Down's red, with forked red lightning flashing every 18 s and spores drifting past. */
  strangerwindow(x) {
    const frame = 0xf2ead8, frameDark = 0xc8bca0, sky = isNight ? 0x3a1a24 : 0x8ac0e8, tree = isNight ? 0x1a1418 : 0x3a6a3a;
    fill(x - 12, 5, 25, 20, frame); fill(x + 12, 5, 1, 20, frameDark); fill(x - 12, 24, 25, 1, frameDark);
    fill(x - 10, 7, 21, 16, sky);
    for (let k = -10; k < 11; k += 3) { const h = 4 + (hash(k + 20, 2) % 4); fill(x + k, 23 - h, 3, h, tree); dot(x + k + 1, 22 - h, tree); }
    if (isNight) {
      const t = (now + x * 0.17) % 18;
      if (t < 0.6 && Math.floor(t * 10) % 2 === 0) glowLate(() => { fill(x - 10, 7, 21, 16, 0xe83a3a, 0.35); line(x - 2, 7, x + 1, 12, 0xffc0c0); line(x + 1, 12, x - 1, 15, 0xffc0c0); line(x + 1, 12, x + 5, 16, 0xffa0a0); });
      for (let k = 0; k < 4; k++) dot(x - 8 + ((Math.floor(now * 2) + k * 5) % 19), 9 + ((k * 7 + Math.floor(now)) % 10), 0xb0a0a8, 0.6);
    } else dot(x - 6, 10, WHITE, 0.8);
    fill(x, 7, 1, 16, frame); fill(x - 10, 14, 21, 1, frame);
    fill(x - 11, 25, 23, 1, SHADOW);
  },
  /** The Scoops Ahoy ice-cream sign from the Starcourt Mall: a red-and-white striped awning edge over a navy board with a white anchor and a three-scoop cone; the cone's cherry bobs now and then. */
  strangerscoops(x) {
    const navy = 0x1e2a5a, red = 0xd8302a, W = 0xf6f2ea, cone = 0xd8a050;
    fill(x - 11, 6, 23, 16, navy); fill(x - 11, 6, 23, 1, 0x3a4a7a);
    for (let k = 0; k < 23; k++) fill(x - 11 + k, 4, 1, 3, (k >> 1) % 2 ? red : W); for (let k = 0; k < 23; k += 2) dot(x - 11 + k, 7, (k >> 1) % 2 ? red : W);
    // The anchor
    fill(x - 6, 10, 1, 9, W); fill(x - 8, 11, 5, 1, W); ring(x - 6, 9, 1, W); line(x - 9, 16, x - 6, 19, W); line(x - 3, 16, x - 6, 19, W);
    // The cone
    for (let r = 0; r < 6; r++) fill(x + 4 + Math.floor(r / 2), 15 + r, 5 - Math.floor(r / 2) * 2 + (r % 2 ? 0 : 0), 1, cone);
    disc(x + 6, 14, 2.2, 0xf2b8c8); disc(x + 4, 12, 1.8, 0xf6f2ea); disc(x + 8, 12, 1.8, 0x8a5a3a);
    const bob = (now + x * 0.2) % 10 < 1 ? Math.floor(now * 6) % 2 : 0; dot(x + 6, 9 - bob, red); dot(x + 6, 10 - bob, red);
    fill(x - 10, 22, 21, 1, SHADOW);
  },
  /** A tall dark grandfather clock face mounted on the wall, its pendulum swinging slowly; every 40 s it chimes, its face cracks red and a red glow pulses round it, then all is still again. */
  strangerclock(x) {
    const wood = 0x3a2420, woodLight = 0x5a3a30, face = 0xe8dcc0, gold = 0xc8a050;
    fill(x - 6, 3, 13, 25, wood); fill(x - 6, 3, 1, 25, woodLight); fill(x - 5, 2, 11, 1, wood); fill(x - 3, 1, 7, 1, wood);
    disc(x, 9, 4, face); ring(x, 9, 4.2, gold);
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, ha = (d.getHours() % 12 / 12) * Math.PI * 2 - Math.PI / 2;
    line(x, 9, x + Math.round(Math.cos(ma) * 3), 9 + Math.round(Math.sin(ma) * 3), 0x1e1e22); line(x, 9, x + Math.round(Math.cos(ha) * 2), 9 + Math.round(Math.sin(ha) * 2), 0x1e1e22);
    fill(x - 4, 15, 9, 11, 0x1e1210);
    const sw = [0, 1, 2, 1, 0, -1, -2, -1][Math.floor(now * 2) % 8];
    line(x, 15, x + sw, 22, gold); disc(x + sw, 23, 1.6, gold);
    const t = (now + x * 0.19) % 40;
    if (t < 3) {
      const a = Math.min(1, t * 2, 3 - t);
      line(x - 2, 6, x + 1, 9, 0xc8281e, a); line(x + 1, 9, x, 12, 0xc8281e, a);
      glowAt(x, 12, 9, 13, 0xe82a2a, 0.15 * a);
      if (isNight) glowLate(() => glowAt(x, 12, 10, 14, 0xe82a2a, 0.25 * a));
    }
    fill(x - 5, 28, 11, 1, SHADOW);
  },
  /** The fenced Hawkins National Laboratory: a white sign with its name in blocky letters on a chain-link fence panel, a red KEEP OUT strip below; a security light over it blinks at night. */
  strangerhawkins(x) {
    const fence = 0x8a8e96, W = 0xf2f2ee, ink = 0x2a3a5a, red = 0xc8281e;
    fill(x - 12, 6, 25, 20, fence, 0.25);
    for (let k = -12; k < 13; k += 3) { line(x + k, 6, x + k + 3, 25, fence, 0.7); line(x + k + 3, 6, x + k, 25, fence, 0.7); }
    fill(x - 12, 5, 25, 1, 0x6e727c); fill(x - 12, 25, 25, 1, 0x6e727c);
    fill(x - 9, 9, 19, 10, W); fill(x - 9, 9, 19, 1, 0xd8d8d8);
    for (let k = 0; k < 7; k++) fill(x - 8 + k * 2 + (k > 3 ? 1 : 0), 11, 1, 3, ink);
    for (let k = 0; k < 8; k++) dot(x - 8 + k * 2, 15, ink, 0.7);
    fill(x - 9, 17, 19, 2, red); for (let k = -7; k < 9; k += 2) dot(x + k, 17, W, 0.8);
    fill(x - 1, 3, 3, 2, 0x4a4a52); const on = isNight && Math.floor(now) % 3 !== 0;
    dot(x, 4, on ? 0xfff0b0 : 0x8a8a70);
    if (on) glowLate(() => glowAt(x, 5, 6, 4, 0xfff0b0, 0.2));
  },
  /** The banner of House Stark: a grey direwolf running on an icy white field, hung from an iron rod; it stirs in a draught now and then. */
  gotstark(x) {
    const W = 0xeeeeea, grey = 0x6a6e78, greyDark = 0x4a4e58;
    fill(x - 8, 3, 17, 1, 0x3a3a42);
    const sw = (now + x * 0.23) % 15 < 1.5 ? [0, 1, 0, -1][Math.floor(now * 4) % 4] : 0;
    fill(x - 6, 4, 13, 20, W); fill(x - 6, 4, 1, 20, WHITE); fill(x + 6, 4, 1, 20, 0xc8c8c4);
    for (let k = 0; k < 13; k++) dot(x - 6 + k + (k % 2 ? sw : 0), 24 + (k % 3 === 1 ? 1 : 0), W);
    // The running wolf, head up
    fill(x - 4, 13, 7, 3, grey); fill(x + 2, 10, 3, 3, grey); dot(x + 5, 11, grey); dot(x + 2, 9, grey); dot(x + 4, 9, grey);
    line(x - 4, 16, x - 5, 19, greyDark); line(x - 2, 16, x - 1, 19, greyDark); line(x + 1, 16, x + 3, 19, greyDark); line(x + 2, 15, x + 4, 18, greyDark);
    line(x - 4, 13, x - 6, 11, grey); dot(x + 3, 11, 0x2a2a30);
    fill(x - 5, 26, 11, 1, SHADOW);
  },
  /** The banner of House Lannister: a gold lion rampant on crimson, gold-fringed; it stirs now and then. */
  gotlannister(x) {
    const red = 0x9a1e1e, gold = 0xe0b84a, goldDark = 0xa8842a;
    fill(x - 8, 3, 17, 1, gold);
    const sw = (now + x * 0.27) % 15 < 1.5 ? [0, 1, 0, -1][Math.floor(now * 4) % 4] : 0;
    fill(x - 6, 4, 13, 20, red); fill(x - 6, 4, 1, 20, 0xb83030); fill(x + 6, 4, 1, 20, 0x7a1414);
    for (let k = 0; k < 13; k += 2) dot(x - 6 + k + sw, 24, gold);
    // The lion rampant
    fill(x - 1, 9, 4, 4, gold); fill(x - 2, 10, 1, 3, goldDark); dot(x + 3, 10, gold); dot(x + 2, 11, red);
    fill(x - 1, 13, 3, 5, gold); line(x + 2, 13, x + 4, 11, gold); line(x + 2, 15, x + 4, 14, gold);
    line(x - 1, 18, x - 3, 21, gold); line(x + 1, 18, x + 2, 21, gold); line(x - 1, 16, x - 4, 15, goldDark); dot(x - 4, 14, goldDark);
    fill(x - 5, 26, 11, 1, SHADOW);
  },
  /** The banner of House Targaryen: a red three-headed dragon on black; at night its red seems to smoulder. */
  gottargaryen(x) {
    const black = 0x14141a, red = 0xc8281e, redDark = 0x8a1a14;
    fill(x - 8, 3, 17, 1, 0x3a3a42);
    fill(x - 6, 4, 13, 20, black); fill(x - 6, 4, 1, 20, 0x2a2a32);
    for (let k = 0; k < 13; k += 3) { dot(x - 6 + k, 24, black); dot(x - 5 + k, 24, black); }
    // The body and wings, and the three necks and heads
    fill(x - 2, 14, 5, 4, red); line(x - 2, 14, x - 5, 10, redDark); line(x + 2, 14, x + 5, 10, redDark); fill(x - 5, 10, 2, 4, red); fill(x + 4, 10, 2, 4, red);
    for (const [nx, hy] of [[-2, 8], [0, 7], [2, 8]]) { line(x + nx, 14, x + nx, hy + 1, red); fill(x + nx - 1, hy, 2, 2, red); }
    line(x, 18, x - 2, 21, red); line(x - 2, 21, x + 1, 22, red);
    if (isNight) glowLate(() => glowAt(x, 14, 4, 5, 0xe83a2a, 0.08));
    fill(x - 5, 26, 11, 1, SHADOW);
  },
  /** A deep-set stone window looking north to the Wall, a long white line on the horizon under a grey sky; snow drifts past, and every 30 s a dragon's dark shape crosses the sky. At night the sky is deep blue with a cold moon. */
  gotwindow(x) {
    const stone = 0x6a6e72, stoneLight = 0x8a8e92, stoneDark = 0x4a4e52, sky = isNight ? 0x141c34 : 0xb8c4cc;
    fill(x - 11, 4, 23, 22, stone); fill(x - 11, 4, 23, 1, stoneLight); fill(x + 11, 4, 1, 22, stoneDark);
    fill(x - 8, 7, 17, 16, sky); fill(x - 6, 6, 13, 1, sky);
    fill(x - 8, 16, 17, 2, isNight ? 0x8a9ab8 : 0xf2f6fa); fill(x - 8, 18, 17, 5, isNight ? 0x2a3448 : 0x8a9aa4); fill(x - 8, 16, 17, 1, isNight ? 0xb8c8e0 : WHITE);
    if (isNight) { disc(x + 4, 10, 1.6, 0xe8eef4); for (const [sx, sy] of [[-5, 9], [-2, 12], [6, 13]]) dot(x + sx, sy, 0xe8f0ff, 0.8); }
    for (let k = 0; k < 6; k++) dot(x - 7 + ((k * 5 + Math.floor(now * 1.5)) % 16), 7 + ((k * 3 + Math.floor(now * 3)) % 14), WHITE, 0.85);
    const t = (now + x * 0.3) % 30;
    if (t < 2.5) { const dx = x - 8 + Math.round(t / 2.5 * 16), dy = 10; const f = Math.floor(now * 6) % 2; dot(dx, dy, 0x1e1a1e); dot(dx + 1, dy, 0x1e1a1e); dot(dx + 2, dy - 1, 0x1e1a1e); dot(dx - 1, dy - 1 - f, 0x1e1a1e); dot(dx + 1, dy - 1 - f, 0x1e1a1e); }
    fill(x, 7, 1, 16, stoneDark);
    fill(x - 11, 26, 23, 1, SHADOW);
  },
  /** Two great swords crossed on a dark wooden shield-plaque: the bastard sword with its white wolf's-head pommel and the greatsword with dark rippled Valyrian steel; the steel ripples catch the light now and then. */
  gotswords(x) {
    const wood = 0x4a3020, woodLight = 0x6a4a30, steel = 0xb8c0c8, valyrian = 0x5a5e68, valLight = 0x8a8e98, hilt = 0x3a2a20;
    disc(x, 14, 9, wood); disc(x - 1, 13, 8, woodLight, 0.4); ring(x, 14, 9, 0x3a3a42);
    line(x - 10, 24, x + 9, 4, steel); line(x - 9, 24, x + 10, 4, 0xe0e8f0); line(x - 8, 20, x - 6, 22, hilt); fill(x - 12, 25, 3, 2, 0xf2f2ee); dot(x - 13, 26, 0x8a8a8a);
    line(x + 10, 24, x - 9, 4, valyrian); line(x + 9, 24, x - 10, 4, valLight); line(x + 8, 20, x + 6, 22, hilt); fill(x + 10, 25, 3, 2, hilt);
    const g = (now + x * 0.21) % 9; if (g < 1) { const k = Math.floor(g * 18); dot(x + 9 - k, 24 - k, WHITE, 0.9); }
    for (let k = 2; k < 18; k += 4) dot(x + 9 - k, 23 - k, 0x3a3e48);
    fill(x - 8, 28, 17, 1, SHADOW);
  },
  /** A carving cut from a heart tree: pale white weirwood bark with a solemn face whose eyes weep red sap, framed by blood-red leaves that rustle now and then. */
  gotweirwood(x) {
    const bark = 0xece6da, barkDark = 0xc8c0b0, red = 0xa81e1e, redLight = 0xd8402a, sap = 0x8a1414;
    fill(x - 6, 4, 13, 22, bark); fill(x - 6, 4, 1, 22, WHITE); fill(x + 6, 4, 1, 22, barkDark);
    for (let y = 6; y < 26; y += 3) dot(x - 4 + (y % 5), y, barkDark);
    // The face
    fill(x - 4, 11, 3, 2, barkDark); fill(x + 2, 11, 3, 2, barkDark); dot(x - 3, 12, 0x6a5a4a); dot(x + 3, 12, 0x6a5a4a);
    fill(x - 3, 13, 1, 4, sap); fill(x + 3, 13, 1, 3, sap); dot(x - 3, 17, sap, 0.6);
    fill(x - 1, 14, 2, 3, barkDark); fill(x - 3, 19, 7, 1, 0x8a7a6a); dot(x - 3, 18, barkDark); dot(x + 3, 18, barkDark);
    // The leaves
    const rs = (now + x * 0.17) % 10 < 1.5 ? Math.floor(now * 5) % 2 : 0;
    for (const [lx, ly] of [[-10, 4], [-8, 7], [-10, 10], [8, 4], [9, 8], [7, 11], [-7, 2], [5, 2], [-1, 2], [-9, 14], [9, 14]]) {
      fill(x + lx + (lx > 0 ? rs : -rs), ly, 3, 2, red); dot(x + lx + 1 + (lx > 0 ? rs : -rs), ly, redLight);
    }
    fill(x - 5, 26, 11, 1, SHADOW);
  },
  /** The sailboat painting that hangs over the Simpsons' couch: a little white-sailed boat on blue water under a pale sky, in a plain brown frame, hung just a little crooked. */
  simpsonsboat(x) {
    const frame = 0x8a5a32, frameLight = 0xa8703e;
    fill(x - 12, 6, 25, 16, frame); fill(x - 12, 6, 25, 1, frameLight); fill(x - 12, 6, 1, 16, frameLight);
    fill(x - 10, 8, 21, 7, 0xc8e0f0); fill(x - 10, 15, 21, 5, 0x3a7ac8); for (let k = -9; k < 10; k += 4) dot(x + k, 17, 0x6a9ae0);
    fill(x - 3, 15, 8, 2, 0x8a3a2a); fill(x - 2, 17, 6, 1, 0x6a2a1a);
    fill(x + 1, 9, 1, 6, 0x5a3a22); for (let r = 0; r < 5; r++) fill(x + 2, 10 + r, 1 + r, 1, WHITE); for (let r = 0; r < 4; r++) fill(x - r, 11 + r, r, 1, 0xf2f2ea);
    dot(x + 1, 8, 0xd8302a);
    fill(x - 11, 22, 24, 1, SHADOW);
  },
  /** The Lard Lad Donuts sign: a giant pink-frosted donut with sprinkles on a striped board; now and then its lights chase round the donut. */
  simpsonslardlad(x) {
    const board = 0xf2e8d0, stripe = 0xd8302a, dough = 0xd89a5a, pink = 0xf28ab0;
    fill(x - 11, 5, 23, 19, board); for (let k = 0; k < 23; k += 4) fill(x - 11 + k, 5, 2, 19, stripe, 0.25);
    fill(x - 11, 5, 23, 1, stripe); fill(x - 11, 23, 23, 1, stripe);
    disc(x, 14, 7, dough); disc(x, 13, 6.2, pink); disc(x, 14, 2.4, board);
    for (const [dx, dy, c] of [[-4, 10, 0x3a8ae8], [3, 9, 0xf2e040], [-5, 14, 0x3ac85a], [5, 13, 0xf6f6f6], [-2, 9, 0xf2e040], [4, 16, 0x3a8ae8], [-3, 17, 0xd8302a]]) dot(x + dx, dy, c);
    const t = (now + x * 0.21) % 12;
    if (t < 3) { const a = t * 6; for (let k = 0; k < 3; k++) { const b = a + k * 2.1; dot(x + Math.round(Math.cos(b) * 9), 14 + Math.round(Math.sin(b) * 8), 0xfff070); } }
    fill(x - 10, 24, 21, 1, SHADOW);
  },
  /** A poster of Krusty the Clown: the green curly hair round his bald head, a big red nose and grin on a yellow background; his eyebrows waggle now and then. */
  simpsonskrusty(x) {
    const yellow = 0xf2d040, green = 0x4aaa3a, skin = 0xf2e8d8, red = 0xd8302a;
    fill(x - 8, 4, 17, 21, yellow); fill(x - 8, 4, 17, 1, 0xfff070); fill(x + 8, 4, 1, 21, 0xc8a830);
    fill(x - 4, 8, 9, 10, skin); disc(x - 5, 12, 2.4, green); disc(x + 5, 12, 2.4, green); dot(x - 7, 10, green); dot(x + 7, 10, green); fill(x - 1, 6, 3, 2, green);
    const brow = (now + x * 0.17) % 6 < 0.8 ? Math.floor(now * 6) % 2 : 0;
    fill(x - 3, 10 - brow, 2, 1, 0x2a2a30); fill(x + 2, 10 - brow, 2, 1, 0x2a2a30);
    dot(x - 2, 11, 0x2a2a30); dot(x + 2, 11, 0x2a2a30); disc(x, 13, 1.2, red);
    fill(x - 3, 16, 7, 1, red); dot(x - 3, 15, red); dot(x + 3, 15, red);
    fill(x - 5, 18, 11, 5, 0x3a6ac8); dot(x, 19, 0xf2f2ea);
    fill(x - 7, 25, 15, 1, SHADOW);
  },
  /** A window looking out over Springfield: green hills and the power plant's two cooling towers, steam puffing from them, under a blue sky with puffy clouds; at night their warning lights blink red and the sky is deep blue. */
  simpsonswindow(x) {
    const frame = 0xf2ead8, frameDark = 0xc8bca0, sky = isNight ? 0x1a2650 : 0x6ab0f0, tower = isNight ? 0x4a5058 : 0xa8aca8;
    fill(x - 13, 5, 27, 20, frame); fill(x + 13, 5, 1, 20, frameDark);
    fill(x - 11, 7, 23, 16, sky);
    if (!isNight) { disc(x - 6, 10, 2, WHITE); disc(x - 3, 10, 2.5, WHITE); fill(x - 8, 11, 8, 2, WHITE); }
    fill(x - 11, 19, 23, 4, isNight ? 0x1e3a24 : 0x5a9a4a);
    for (const tx of [x + 2, x + 7]) {
      for (let r = 0; r < 9; r++) { const w = 4 - Math.round(Math.sin((r / 8) * Math.PI) * 1); fill(tx - Math.floor(w / 2), 11 + r, w, 1, r % 4 ? tower : shaded(tower, 0.85)); }
      const ph = (now * 0.5 + tx * 0.1) % 1; disc(tx, 10 - Math.round(ph * 3), 1.5 + ph, WHITE, isNight ? 0.4 : 0.85 * (1 - ph * 0.5));
      if (isNight && Math.floor(now + tx) % 2) glowLate(() => dot(tx, 11, 0xff3a2a));
    }
    if (isNight) for (const [sx, sy] of [[-8, 9], [-4, 12], [-9, 14]]) dot(x + sx, sy, 0xfff8d0, 0.8);
    fill(x, 7, 1, 16, frame); fill(x - 11, 14, 23, 1, frame);
    fill(x - 12, 25, 26, 1, SHADOW);
  },
  /** The power plant's safety board: "Days without an accident" over a flip number; it counts up day by day, and every 40 s it gets knocked back to 0, the digits flipping. */
  simpsonsaccident(x) {
    const board = 0x2a5a3a, W = 0xf2f2ea, yellow = 0xf2d040;
    fill(x - 11, 5, 23, 19, board); fill(x - 11, 5, 23, 1, 0x3a7a4a); fill(x - 11, 23, 23, 1, 0x1a3a24);
    for (let k = 0; k < 5; k++) fill(x - 9 + k * 4, 7, 3, 1, W, 0.8);
    for (let k = 0; k < 4; k++) fill(x - 9 + k * 5, 9, 4, 1, W, 0.6);
    fill(x - 4, 12, 9, 9, 0x1a1a1e); fill(x, 12, 1, 9, 0x3a3a42);
    const t = (now + x * 0.19) % 40, zero = t < 6;
    const digit = (dx, n) => {
      const segs = { 0: [1, 1, 1, 0, 1, 1, 1], 1: [0, 0, 1, 0, 0, 1, 0], 2: [1, 0, 1, 1, 1, 0, 1], 3: [1, 0, 1, 1, 0, 1, 1], 4: [0, 1, 1, 1, 0, 1, 0], 5: [1, 1, 0, 1, 0, 1, 1], 6: [1, 1, 0, 1, 1, 1, 1], 7: [1, 0, 1, 0, 0, 1, 0], 8: [1, 1, 1, 1, 1, 1, 1], 9: [1, 1, 1, 1, 0, 1, 1] }[n];
      const X = x + dx, c = yellow;
      if (segs[0]) fill(X, 13, 3, 1, c); if (segs[1]) fill(X, 13, 1, 3, c); if (segs[2]) fill(X + 2, 13, 1, 3, c); if (segs[3]) fill(X, 16, 3, 1, c);
      if (segs[4]) fill(X, 16, 1, 3, c); if (segs[5]) fill(X + 2, 16, 1, 3, c); if (segs[6]) fill(X, 19, 3, 1, c);
    };
    const days = zero ? 0 : 1 + Math.floor((t - 6) / 4);
    if (zero && t < 0.8 && Math.floor(now * 10) % 2) fill(x - 4, 12, 9, 9, W, 0.4);
    digit(-3, Math.floor(days / 10) % 10); digit(2, days % 10);
    fill(x - 10, 24, 21, 1, SHADOW);
  },
  /** A Duff Beer neon sign: the red Duff name in an oval of white neon tubing; it hums and glows, a letter flickering out now and then, and shines through the night. */
  simpsonsduff(x) {
    const red = 0xff3a3a, white = 0xf8f4f0;
    fill(x - 10, 7, 21, 14, 0x2a2228, 0.85);
    const tubes = (a) => {
      ellipseRing(x, 14, 9, 5.5, white, a);
      const flick = Math.floor(now * 3 + x) % 23 === 0 ? 1 : -1;
      // D U F F
      const letters = [[-7, [[0, 0], [0, 1], [0, 2], [0, 3], [1, 0], [2, 1], [2, 2], [1, 3]]], [-3, [[0, 0], [0, 1], [0, 2], [1, 3], [2, 0], [2, 1], [2, 2]]], [1, [[0, 0], [0, 1], [0, 2], [0, 3], [1, 0], [2, 0], [1, 2]]], [5, [[0, 0], [0, 1], [0, 2], [0, 3], [1, 0], [2, 0], [1, 2]]]];
      letters.forEach(([lx, pts], k) => { if (k === flick + 3 && flick > 0) return; for (const [dx, dy] of pts) dot(x + lx + dx, 12 + dy, red, a); });
    };
    tubes(1);
    glowAt(x, 14, 10, 6, 0xff4a4a, 0.08);
    if (isNight) glowLate(() => { glowAt(x, 14, 11, 7, 0xff4a4a, 0.22); tubes(0.9); });
  },
  /** The gun-barrel opening in a round frame: the rifled spiral of a barrel in white and grey; every 20 s a tiny figure in black walks into the circle, turns and fires, and red runs down over it, then it all clears. */
  bondgunbarrel(x) {
    const cy = 14, t = (now + x * 0.27) % 20;
    disc(x, cy, 11, 0x0c0c10);
    for (let k = 0; k < 4; k++) { const a = k * Math.PI / 2 + t * 0.15; for (let r = 4; r < 10; r++) dot(x + Math.round(Math.cos(a + r * 0.3) * r), cy + Math.round(Math.sin(a + r * 0.3) * r), 0x5a5a62); }
    ring(x, cy, 10, 0x8a8a92); disc(x, cy, 4.5, 0xf2f2ee);
    if (t < 5) {
      const walk = Math.min(1, t / 2.5), fx = x - 4 + Math.round(walk * 4), turned = t > 2.5;
      fill(fx, cy - 2, 1, 2, 0x1e1e22); dot(fx, cy - 3, 0x1e1e22); dot(fx - (t % 0.5 < 0.25 ? 1 : 0), cy, 0x1e1e22); dot(fx + 1, cy, 0x1e1e22);
      if (turned) { dot(fx + 1, cy - 2, 0x1e1e22); if (t > 3 && t < 3.2) dot(fx + 2, cy - 2, 0xffe060); }
      if (t > 3.2) { const run = Math.min(9, Math.round((t - 3.2) * 6)); fill(x - 4, cy - 4, 9, run, 0xc8181e, 0.85); }
    }
    fill(x - 9, 26, 19, 1, SHADOW);
  },
  /** A framed 007 poster: the white 007 logo with the pistol for the 7 on black, a red stripe across the bottom; the gun's muzzle flashes now and then. */
  bond007(x) {
    const frame = 0xc8a050, black = 0x16161c, W = 0xf2f2ee;
    fill(x - 9, 4, 19, 22, frame); fill(x - 8, 5, 17, 20, black);
    ring(x - 5, 13, 2.4, W); ring(x - 1, 13, 2.4, W);
    fill(x + 2, 10, 6, 2, W); line(x + 6, 12, x + 3, 18, W); line(x + 7, 12, x + 4, 18, W); fill(x + 7, 12, 1, 2, W);
    if ((now + x * 0.21) % 9 < 0.3) { dot(x + 8, 10, 0xffe060); dot(x + 9, 10, 0xffa030); }
    fill(x - 8, 21, 17, 2, 0xc8281e);
    fill(x - 8, 26, 17, 1, SHADOW);
  },
  /** MI6's operations map: a dark screen with the continents traced in green, little red dots blinking where agents are, and a line tracking across to a new mission now and then. */
  bondworldmap(x) {
    const frame = 0x3a3a42, screen = 0x0c1a14, land = 0x2a8a4a;
    fill(x - 14, 5, 29, 18, frame); fill(x - 13, 6, 27, 16, screen);
    for (const [lx, ly, w, h] of [[-12, 8, 6, 4], [-10, 12, 3, 6], [-3, 8, 4, 3], [-3, 11, 3, 6], [2, 8, 9, 5], [8, 14, 3, 3], [-1, 18, 2, 1]]) fill(x + lx, ly, w, h, land, 0.8);
    for (let k = -12; k < 14; k += 4) fill(x + k, 6, 1, 16, 0x1a3a2a, 0.5);
    const dots = [[-9, 10], [-1, 9], [5, 10], [9, 15], [-10, 15]];
    dots.forEach(([dx, dy], k) => { const on = (Math.floor(now * 2) + k) % 3 !== 0; dot(x + dx, dy, on ? 0xff3a2a : 0x6a1a14); if (isNight && on) glowLate(() => dot(x + dx, dy, 0xff5a3a)); });
    const t = (now + x * 0.19) % 16;
    if (t < 3) { const p = t / 3, sx = x - 1, sy = 9, ex = x + 9, ey = 15; line(sx, sy, Math.round(sx + (ex - sx) * p), Math.round(sy + (ey - sy) * p), 0xf2d040); }
    glowAt(x, 14, 13, 7, 0x2aff8a, 0.04);
    fill(x - 13, 23, 27, 1, SHADOW);
  },
  /** A tall window looking down on Monte Carlo harbour: yachts on blue water below the hillside town, the Casino's dome; at night the town's lights twinkle gold and the sea is dark. */
  bondwindow(x) {
    const frame = 0xf2ead8, frameDark = 0xc8bca0, sky = isNight ? 0x1a2050 : 0x8ac8f0, sea = isNight ? 0x10203a : 0x2a7ac8, hill = isNight ? 0x2a2a30 : 0xd8c8a8;
    fill(x - 13, 4, 27, 22, frame); fill(x + 13, 4, 1, 22, frameDark);
    fill(x - 11, 6, 23, 18, sky); fill(x - 11, 18, 23, 6, sea);
    fill(x - 11, 12, 23, 6, hill); for (let k = -11; k < 12; k += 3) fill(x + k, 11 + (hash(k + 30, 4) % 3), 2, 2, isNight ? 0x3a3a42 : 0xf2ead8);
    disc(x + 3, 12, 2, isNight ? 0x4a4a52 : 0xb8c8a8); dot(x + 3, 9, isNight ? 0x6a6a72 : 0x8a9a7a);
    if (isNight) {
      for (let k = 0; k < 10; k++) { const lx = x - 10 + ((k * 7) % 21), ly = 12 + (k % 5); if ((Math.floor(now) + k) % 4) glowLate(() => dot(lx, ly, 0xffd070)); }
      for (const [sx, sy] of [[-7, 8], [5, 7], [9, 9]]) dot(x + sx, sy, 0xfff8d0, 0.8);
    }
    for (const yx of [-7, 2, 8]) { fill(x + yx, 20, 3, 1, WHITE); dot(x + yx + 1, 19, WHITE); if (isNight) glowLate(() => dot(x + yx, 20, 0xfff0b0)); }
    fill(x, 6, 1, 18, frame); fill(x - 11, 15, 23, 1, frame, 0.6);
    fill(x - 12, 26, 26, 1, SHADOW);
  },
  /** Q Branch's gadget pegboard: a wristwatch with a laser, a fountain pen, a cigarette case, a keyring and a little grappling gun hung on hooks; every 15 s the watch's laser flicks out a thin red line. */
  bondgadgets(x) {
    const board = 0xc8b890, peg = 0x8a7a5a, steel = 0xa8adb6, brass = 0xc8a050;
    fill(x - 13, 5, 27, 20, board); fill(x - 13, 5, 27, 1, 0xdccca4); for (let k = -12; k < 14; k += 3) for (let r = 7; r < 24; r += 3) dot(x + k, r, peg, 0.6);
    disc(x - 9, 11, 2.2, steel); disc(x - 9, 11, 1.4, 0x1e2a4a); fill(x - 10, 13, 3, 3, 0x3a3a42); fill(x - 10, 8, 3, 1, 0x3a3a42);
    const t = (now + x * 0.21) % 15; if (t < 1) line(x - 7, 11, x + 2, 11, 0xff2a2a);
    fill(x - 3, 15, 7, 1, 0x1e1e22); dot(x + 3, 15, brass);
    fill(x + 6, 9, 5, 3, steel); fill(x + 6, 9, 5, 1, 0xd0d4da); dot(x + 8, 10, 0x6e727c);
    ring(x - 8, 20, 1.5, brass); line(x - 7, 21, x - 5, 22, steel);
    fill(x + 3, 18, 6, 2, 0x3a3a42); fill(x + 4, 20, 2, 3, 0x3a3a42); line(x + 9, 18, x + 11, 16, steel);
    fill(x - 12, 25, 25, 1, SHADOW);
  },
  /** The Union Jack parachute from the ski-jump stunt, a little model hanging open on the wall from its cords, with a tiny skier dangling beneath; it sways now and then. */
  bondparachute(x) {
    const blue = 0x1e3a8a, red = 0xc8281e, W = 0xf2f2ee;
    const sw = (now + x * 0.19) % 14 < 2 ? [0, 1, 0, -1][Math.floor(now * 4) % 4] : 0;
    for (let r = 0; r < 8; r++) { const half = Math.round(Math.sqrt(64 - (8 - r) * (8 - r)) * 1.4); fill(x - half, 4 + r, half * 2 + 1, 1, blue); }
    fill(x - 11, 8, 23, 1, W); fill(x - 1, 4, 3, 8, W); fill(x, 4, 1, 8, red); fill(x - 11, 9, 23, 1, red);
    line(x - 8, 6, x + 8, 11, W); line(x + 8, 6, x - 8, 11, W);
    line(x - 10, 12, x + sw, 21, 0x8a8a8a); line(x + 10, 12, x + sw, 21, 0x8a8a8a); line(x, 12, x + sw, 21, 0x8a8a8a);
    fill(x - 1 + sw, 21, 2, 3, 0xf2d040); dot(x + sw, 20, 0xe8c8a0); line(x - 3 + sw, 24, x + 3 + sw, 24, 0x3a3a42);
    fill(x - 9, 26, 19, 1, SHADOW);
  },
  /** A tall window onto the deep desert: rolling orange dunes under a hazy sky by day; at night two moons hang over silver dunes. Every 30 s a great sandworm breaches far out, arcing out of the sand and back under. */
  dunewindow(x) {
    const frame = 0x5a4632, frameLight = 0x7a6248, sky = isNight ? 0x1a1e3a : 0xe8c890, dune = isNight ? 0x6a6478 : 0xd8984e, duneDark = isNight ? 0x4a465a : 0xb87a3a;
    fill(x - 13, 4, 27, 22, frame); fill(x - 13, 4, 27, 1, frameLight);
    fill(x - 11, 6, 23, 18, sky);
    for (let k = -11; k < 12; k++) { const h1 = 15 + Math.round(Math.sin(k * 0.3) * 2), h2 = 19 + Math.round(Math.sin(k * 0.45 + 2) * 1.5); fill(x + k, h1, 1, 24 - h1, duneDark); fill(x + k, h2, 1, 24 - h2, dune); }
    if (isNight) { disc(x - 6, 9, 1.8, 0xe8e8f0); disc(x + 5, 8, 1.2, 0xc8c8d8); for (const [sx, sy] of [[-2, 7], [8, 11], [-9, 12]]) dot(x + sx, sy, 0xfff8d0, 0.8); }
    else disc(x + 6, 9, 2, 0xfff4d0);
    const t = (now + x * 0.29) % 30;
    if (t < 3) { const p = t / 3; for (let k = 0; k < 8; k++) { const a = Math.PI * (k / 7), wx = x - 6 + k * 2, wy = 16 - Math.round(Math.sin(a) * 5 * Math.sin(p * Math.PI)); if (wy < 16) dot(wx, wy, isNight ? 0x5a5868 : 0x8a6a48); } dot(x - 7, 16, isNight ? 0x8a8898 : 0xe8c890); dot(x + 9, 16, isNight ? 0x8a8898 : 0xe8c890); }
    fill(x, 6, 1, 18, frame);
    fill(x - 12, 26, 26, 1, SHADOW);
  },
  /** The banner of House Atreides: the red hawk on green, gold-edged, hung from a gold rod; it stirs in the dry air now and then. */
  duneatreidesbanner(x) {
    const green = 0x1e4a2e, greenLight = 0x2e6a42, red = 0xc8281e, gold = 0xc8a050;
    fill(x - 8, 3, 17, 1, gold);
    const sw = (now + x * 0.19) % 14 < 1.5 ? [0, 1, 0, -1][Math.floor(now * 4) % 4] : 0;
    fill(x - 6, 4, 13, 19, green); fill(x - 6, 4, 1, 19, greenLight); fill(x - 6, 4, 13, 1, gold); fill(x - 6, 22, 13, 1, gold);
    for (let r = 0; r < 3; r++) fill(x - 6 + r + sw, 23 + r, 13 - r * 2, 1, green); dot(x + sw, 26, gold);
    fill(x - 1, 11, 3, 5, red); line(x - 1, 12, x - 5, 8, red); line(x + 1, 12, x + 5, 8, red); line(x - 1, 13, x - 4, 11, red); line(x + 1, 13, x + 4, 11, red);
    dot(x, 10, red); line(x - 1, 16, x - 2, 18, red); line(x + 1, 16, x + 2, 18, red);
    fill(x - 5, 27, 11, 1, SHADOW);
  },
  /** The banner of House Harkonnen: a pale blue griffin on black, hung from a black iron rod; its edges glint a cold blue at night. */
  duneharkonnen(x) {
    const black = 0x0e0e12, blue = 0x6a8ac8, blueLight = 0x9ab8e8;
    fill(x - 8, 3, 17, 1, 0x2a2a30);
    fill(x - 6, 4, 13, 20, black); fill(x - 6, 4, 1, 20, 0x1e1e26);
    for (let k = 0; k < 13; k += 4) fill(x - 6 + k, 24, 2, 2, black);
    fill(x - 2, 11, 4, 6, blue); fill(x + 1, 8, 2, 3, blue); dot(x + 3, 9, blueLight); line(x - 2, 11, x - 5, 7, blue); line(x - 2, 12, x - 5, 10, blue);
    line(x - 2, 17, x - 3, 20, blue); line(x + 1, 17, x + 2, 20, blue); line(x - 2, 16, x - 4, 18, blueLight);
    if (isNight) glowLate(() => { fill(x - 6, 4, 1, 20, blue, 0.4); fill(x + 6, 4, 1, 20, blue, 0.4); });
    fill(x - 5, 26, 11, 1, SHADOW);
  },
  /** The mounted head of the great black bull that killed the Old Duke, on a dark wooden shield, its long horns sweeping out; its eyes gleam now and then. */
  dunebullhead(x) {
    const wood = 0x3a2416, woodLight = 0x5a3a24, hide = 0x1e1a1a, hideLight = 0x3a3232, horn = 0xe8dcc0, hornDark = 0xb8a888;
    fill(x - 5, 6, 11, 18, wood); fill(x - 4, 5, 9, 1, wood); fill(x - 4, 24, 9, 1, wood); fill(x - 5, 6, 1, 18, woodLight);
    fill(x - 3, 9, 7, 10, hide); fill(x - 2, 19, 5, 3, hide); fill(x - 3, 9, 1, 10, hideLight); fill(x - 1, 20, 3, 1, 0x4a3a3a);
    dot(x - 1, 21, 0x0a0a0a); dot(x + 1, 21, 0x0a0a0a);
    line(x - 3, 10, x - 9, 7, horn); line(x - 9, 7, x - 10, 3, horn); line(x - 3, 11, x - 8, 9, hornDark);
    line(x + 3, 10, x + 9, 7, horn); line(x + 9, 7, x + 10, 3, horn); line(x + 3, 11, x + 8, 9, hornDark);
    const g = (now + x * 0.23) % 12 < 0.4;
    dot(x - 2, 13, g ? 0xff6a3a : 0x6a1a14); dot(x + 2, 13, g ? 0xff6a3a : 0x6a1a14);
    fill(x - 4, 26, 9, 1, SHADOW);
  },
  /** A Fremen crysknife, its curved milky-white blade made from a sandworm's tooth, crossed in its sheath on a woven cloth mount; the blade shimmers now and then. */
  dunecrysknife(x) {
    const cloth = 0x8a3a24, clothDark = 0x6a2a1a, blade = 0xf2ece0, bladeShade = 0xd0c8b8, hilt = 0x3a2a20, sheath = 0x5a4632;
    fill(x - 10, 7, 21, 14, cloth); fill(x - 10, 7, 21, 1, 0xa84a2e); for (let k = -9; k < 11; k += 4) { dot(x + k, 10, 0x2a3a6a); dot(x + k + 2, 17, 0x2a3a6a); } fill(x - 10, 20, 21, 1, clothDark);
    for (let k = 0; k < 13; k++) { const bx = x - 7 + k, by = 15 - Math.round(Math.sin((k / 12) * Math.PI) * 3); dot(bx, by, blade); dot(bx, by + 1, bladeShade); }
    fill(x + 6, 14, 4, 2, hilt); dot(x + 10, 14, 0xc8a050);
    line(x - 6, 18, x + 8, 18, sheath);
    const g = (now + x * 0.17) % 10; if (g < 1) dot(x - 7 + Math.floor(g * 13), 15 - Math.round(Math.sin((Math.floor(g * 13) / 12) * Math.PI) * 3), WHITE);
    fill(x - 9, 21, 19, 1, SHADOW);
  },
  /** A map of Arrakis on an orange-tinted chart: the Shield Wall, Arrakeen, the sietches marked, and the spice fields shaded rust; a harvester marker blinks out in the open sand. */
  dunespicemap(x) {
    const chart = 0xe8c890, chartDark = 0xc8a46a, ink = 0x5a3a22, spice = 0xc86a3a;
    fill(x - 12, 5, 25, 19, chart); fill(x - 12, 5, 25, 1, 0xf6dcae); fill(x + 12, 5, 1, 19, chartDark); fill(x - 12, 23, 25, 1, chartDark);
    for (let k = 0; k < 18; k++) dot(x - 10 + k, 10 + Math.round(Math.sin(k * 0.6) * 1.5), ink);
    fill(x - 8, 7, 3, 2, ink); dot(x - 7, 9, 0xc8281e);
    for (const [sx, sy, w, h] of [[-4, 14, 6, 4], [4, 16, 5, 4], [-10, 18, 4, 3]]) fill(x + sx, sy, w, h, spice, 0.5);
    for (const [px, py] of [[6, 8], [-9, 13], [9, 12]]) { dot(x + px, py, ink); dot(x + px + 1, py, ink); }
    if (Math.floor(now * 2 + x) % 3) dot(x + 1, 18, 0xff3a2a);
    fill(x - 11, 24, 24, 1, SHADOW);
  },
  /** The Straw Hat Pirates' flag: a black flag with a grinning white skull wearing a straw hat over crossed bones, flying from a pole; it ripples in the sea wind now and then. */
  onepiecejollyroger(x) {
    const black = 0x16161c, W = 0xf2f2ee, straw = 0xf2d040, band = 0xd8302a;
    fill(x - 10, 3, 1, 22, 0x6a4424); dot(x - 10, 2, 0xc8a050);
    const t = (now + x * 0.19) % 9, rip = t < 2 ? Math.floor(now * 6) % 2 : 0;
    for (let k = 0; k < 18; k++) { const wave = Math.round(Math.sin(k * 0.5 + now * (rip ? 6 : 1)) * (rip ? 1 : 0)); fill(x - 9 + k, 4 + wave, 1, 15, black); }
    line(x - 6, 9, x + 6, 17, W); line(x + 6, 9, x - 6, 17, W);
    fill(x - 3, 10, 7, 6, W); fill(x - 2, 16, 5, 1, W); dot(x - 1, 12, black); dot(x + 2, 12, black); dot(x, 14, black); for (const dx of [-1, 1]) dot(x + dx, 15, black);
    fill(x - 5, 9, 11, 1, straw); fill(x - 3, 7, 7, 2, straw); fill(x - 3, 8, 7, 1, band);
    fill(x - 9, 25, 18, 1, SHADOW);
  },
  /** A Marine wanted poster for the Straw Hat captain: "WANTED" across the top, a grinning portrait in a straw hat, "DEAD OR ALIVE" and a huge bounty in berries; its corner curls in the breeze now and then. */
  onepiecewanted(x) {
    const paper = 0xe8dcb4, paperDark = 0xc8b88a, ink = 0x3a2a1e, skin = 0xf0c8a0, straw = 0xf2d040;
    fill(x - 7, 3, 15, 23, paper); fill(x - 7, 3, 15, 1, 0xf6ecd0); fill(x + 7, 3, 1, 23, paperDark);
    for (let k = 0; k < 6; k++) fill(x - 6 + k * 2, 5, 1, 2, ink);
    fill(x - 5, 8, 11, 9, 0xd8c89e); fill(x - 3, 11, 7, 5, skin); dot(x - 1, 13, ink); dot(x + 2, 13, ink); fill(x - 1, 15, 4, 1, ink); dot(x - 1, 14, 0xb85a3a);
    fill(x - 5, 10, 11, 1, straw); fill(x - 3, 8, 7, 2, straw); fill(x - 3, 9, 7, 1, 0xd8302a); fill(x - 3, 11, 1, 2, 0x1e1612); fill(x + 3, 11, 1, 2, 0x1e1612);
    for (let k = 0; k < 5; k++) fill(x - 5 + k * 2, 18, 1, 1, ink); for (let k = 0; k < 10; k++) dot(x - 5 + k, 21, ink); dot(x - 6, 21, ink);
    if ((now + x * 0.21) % 11 < 1) { fill(x + 5, 23, 3, 3, paperDark); dot(x + 7, 22, paper); }
    dot(x, 3, 0xd8302a);
    fill(x - 6, 26, 14, 1, SHADOW);
  },
  /** A brass ship's porthole looking out on the open sea: blue water and sky with a wave line rocking; every 25 s a huge Sea King's head rises from the waves and sinks again. At night the sea is dark under the stars. */
  onepieceporthole(x) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, cy = 14;
    const sky = isNight ? 0x1a2650 : 0x8ac8f0, sea = isNight ? 0x0e1e40 : 0x2a6ac8;
    const rock = Math.round(Math.sin(now * 1.2 + x) * 1.2);
    for (let dy = -8; dy <= 8; dy++) for (let dx = -8; dx <= 8; dx++) {
      const r = Math.hypot(dx, dy);
      if (r > 8.4) continue;
      if (r > 6.6) { dot(x + dx, cy + dy, dx + dy < -2 ? brassLight : dx + dy > 2 ? brassDark : brass); continue; }
      dot(x + dx, cy + dy, dy > rock ? sea : sky);
    }
    if (isNight) for (const [sx, sy] of [[-3, -3], [2, -4], [4, -1]]) dot(x + sx, cy + sy, 0xfff8d0, 0.8);
    const t = (now + x * 0.23) % 25;
    if (t < 4) {
      const up = Math.round(Math.sin(t / 4 * Math.PI) * 5), hx = x + 1;
      for (let k = 0; k < up; k++) { fill(hx - 2, cy + rock - k, 4, 1, 0x3a8a6a); }
      if (up > 2) { fill(hx - 3, cy + rock - up, 6, 2, 0x3a8a6a); dot(hx + 1, cy + rock - up, 0xf2d040); dot(hx - 2, cy + rock - up + 1, 0xf2f2ee); }
    }
    for (let k = -6; k <= 6; k += 3) dot(x + k, cy + rock + 1, WHITE, 0.7);
    for (const [bx, by] of [[0, -8], [8, 0], [0, 8], [-8, 0]]) dot(x + bx, cy + by, brassDark);
    fill(x - 6, 24, 13, 1, SHADOW);
  },
  /** A ship's wheel of honey-coloured wood with brass-capped spokes, mounted on the wall; every 18 s it spins round a turn, as if someone took the helm. */
  onepiecehelm(x) {
    const wood = 0xa8743e, woodLight = 0xc8904e, woodDark = 0x6a4424, brass = 0xc8a050, cy = 14;
    const t = (now + x * 0.27) % 18, turn = t < 2 ? (t / 2) * Math.PI / 4 : 0;
    for (let k = 0; k < 8; k++) {
      const a = k * Math.PI / 4 + turn;
      line(x, cy, x + Math.round(Math.cos(a) * 10), cy + Math.round(Math.sin(a) * 10), woodDark);
      dot(x + Math.round(Math.cos(a) * 10), cy + Math.round(Math.sin(a) * 10), brass);
    }
    ring(x, cy, 7, wood); ring(x, cy, 6.3, woodLight, 0.8); disc(x, cy, 2.4, wood); disc(x, cy, 1.2, brass);
    fill(x - 8, 26, 17, 1, SHADOW);
  },
  /** A sea chart of the Grand Line on old parchment: the red line of the continent running down it, scattered islands, and a dotted course with a little ship that moves along it island by island. */
  onepiecemap(x) {
    const parch = 0xeadcb4, parchDark = 0xc8b48a, sea = 0x8ab8d8, ink = 0x5a4030;
    fill(x - 13, 5, 27, 19, parch); fill(x - 13, 5, 27, 1, 0xf6ecd0); fill(x + 13, 5, 1, 19, parchDark); fill(x - 13, 23, 27, 1, parchDark);
    fill(x - 11, 7, 23, 15, sea, 0.45);
    fill(x - 2, 7, 3, 15, 0xc8402a, 0.8);
    const isles = [[-9, 10], [-7, 17], [-4, 13], [4, 9], [7, 15], [10, 11], [5, 19]];
    for (const [ix, iy] of isles) { fill(x + ix - 1, iy, 3, 2, 0x8aa05a); dot(x + ix, iy - 1, 0x6a8a3a); }
    for (let k = 0; k < isles.length - 1; k++) { const [ax, ay] = isles[k], [bx, by] = isles[k + 1]; for (let s = 0; s < 6; s += 2) dot(x + Math.round(ax + (bx - ax) * s / 6), Math.round(ay + (by - ay) * s / 6) + 1, ink, 0.6); }
    const at = Math.floor((now + x * 0.1) / 4) % isles.length, [sx, sy] = isles[at];
    fill(x + sx - 1, sy - 3, 3, 1, 0x8a5a32); dot(x + sx, sy - 4, WHITE); dot(x + sx, sy - 5, 0xd8302a);
    fill(x - 12, 24, 26, 1, SHADOW);
  },
  /** Three katana on a wall rack, the swordsman's three: a white-hilted one, a black one with a red-lacquered sheath, and a cursed one with a purple sheath; one blade slides a little way out of its sheath now and then, gleaming. */
  onepieceswords(x) {
    const rack = 0x5a3a22, rackLight = 0x7a5232;
    fill(x - 11, 6, 2, 18, rack); fill(x + 10, 6, 2, 18, rack); fill(x - 11, 6, 1, 18, rackLight);
    const swords = [[9, 0xf2f2ee, 0x2a2a30], [14, 0x2a2a30, 0x8a1e1e], [19, 0x6a2a7a, 0x3a1a4a]];
    const t = (now + x * 0.21) % 16, who = Math.floor((now + x * 0.21) / 16) % 3, draw = t < 2 ? Math.round(Math.sin(t / 2 * Math.PI) * 4) : 0;
    swords.forEach(([sy, hilt, sheath], k) => {
      fill(x - 9, sy, 15, 2, sheath); fill(x - 9, sy, 15, 1, shaded(sheath, 1.3)); fill(x + 6, sy, 1, 2, 0xc8a050); fill(x + 7, sy, 4, 2, hilt); for (let j = 7; j < 11; j += 2) dot(x + j, sy, shaded(hilt, 0.7));
      if (k === who && draw) { fill(x + 6 - draw, sy, draw, 1, 0xe0e8f0); fill(x + 7 - draw, sy, 5, 2, hilt); dot(x + 6 - draw, sy, WHITE); }
    });
    fill(x - 10, 26, 21, 1, SHADOW);
  },
  /** A tall arched window looking out over the Lands Between to the vast golden Erdtree on the horizon, its canopy glowing against the sky; it glows through the night, golden leaves drifting from it. */
  eldenringerdtreewindow(x) {
    const stone = 0x6a6a66, stoneLight = 0x8a8a84, sky = isNight ? 0x1a1e3a : 0x9ab8c8, land = isNight ? 0x2a3024 : 0x6a7a4a, gold = 0xf2c850;
    fill(x - 13, 3, 27, 23, stone); fill(x - 13, 3, 27, 1, stoneLight);
    for (let r = 0; r < 20; r++) { const h = r < 5 ? Math.round(Math.sqrt(25 - (5 - r) ** 2) * 2.2) : 11; fill(x - h, 5 + r, h * 2 + 1, 1, r > 15 ? land : sky); }
    // The Erdtree: a trunk rising to a vast glowing crown
    const tree = (a) => { fill(x + 3, 10, 2, 11, 0xd8a838, a); for (let k = 0; k < 9; k++) { disc(x + 4 + Math.round(Math.cos(k * 0.7) * 5), 9 + Math.round(Math.sin(k * 0.7) * 2.5), 2.4, gold, a); } disc(x + 4, 8, 3, 0xfff0a0, a); };
    glowAt(x + 4, 10, 9, 8, 0xf2c850, 0.18); tree(1);
    for (let k = 0; k < 3; k++) { const ph = (now * 0.2 + k / 3) % 1; dot(x + 2 + k * 3 - Math.round(ph * 4), 12 + Math.round(ph * 8), gold, 1 - ph); }
    if (isNight) glowLate(() => { glowAt(x + 4, 10, 10, 9, 0xf2c850, 0.3); tree(0.8); });
    fill(x, 5, 1, 20, stone); fill(x - 11, 14, 23, 1, stone, 0.7);
    fill(x - 12, 26, 26, 1, SHADOW);
  },
  /** The Elden Ring itself, worked in gold on a round dark stone plaque: interlocking golden rings round a vertical stroke; a glimmer runs round it now and then, and it glows at night. */
  eldenringsigil(x) {
    const stone = 0x2a2620, gold = 0xd8b45a, goldLight = 0xf2d888, cy = 14;
    disc(x, cy, 10, stone); ring(x, cy, 10, 0x4a443a);
    const sig = (c, a) => { ring(x, cy, 6, c, a); ring(x - 3, cy - 2, 4, c, a * 0.85); ring(x + 3, cy - 2, 4, c, a * 0.85); ring(x, cy + 3, 3.4, c, a * 0.8); fill(x, cy - 8, 1, 16, c, a); };
    sig(gold, 1);
    const g = (now + x * 0.19) % 11; if (g < 1.4) { const a = (g / 1.4) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 6), cy + Math.round(Math.sin(a) * 6), WHITE); }
    if (isNight) glowLate(() => { glowAt(x, cy, 8, 8, 0xf2d070, 0.15); sig(goldLight, 0.6); });
    fill(x - 8, 26, 17, 1, SHADOW);
  },
  /** A map of the Lands Between on old vellum: Limgrave's green, the grey castle on its cliff, the lake, the capital in gold under the Erdtree, the snowy mountain tops; a golden grace marker glows on it, moving now and then. */
  eldenringmap(x) {
    const vellum = 0xe0d0a8, vellumDark = 0xb8a47a, ink = 0x4a3a2a;
    fill(x - 13, 5, 27, 19, vellum); fill(x - 13, 5, 27, 1, 0xf0e2c0); fill(x + 13, 5, 1, 19, vellumDark); fill(x - 13, 23, 27, 1, vellumDark);
    fill(x - 11, 15, 8, 6, 0x8aa05a, 0.7); fill(x - 3, 14, 6, 6, 0x6a9ab8, 0.6); fill(x + 3, 8, 8, 6, 0xd8b45a, 0.6); fill(x + 6, 6, 5, 2, 0xf2f2ee, 0.9);
    fill(x - 9, 11, 3, 4, 0x6a6a66); dot(x - 8, 10, 0x6a6a66);
    for (let k = 0; k < 12; k++) dot(x - 10 + k * 2, 13 + Math.round(Math.sin(k) * 1.5), ink, 0.5);
    const spots = [[-7, 18], [-1, 17], [5, 12], [8, 7]], at = Math.floor((now + x * 0.1) / 5) % spots.length, [gx, gy] = spots[at];
    dot(x + gx, gy, 0xf2d040); dot(x + gx, gy - 1, 0xfff0a0, 0.7);
    if (isNight) glowLate(() => glowAt(x + gx, gy, 2, 2, 0xf2d040, 0.4));
    fill(x - 12, 24, 26, 1, SHADOW);
  },
  /** A katana with a pale blue-glowing blade, Moonveil, resting on a dark wall rack above a blood-red one; every 16 s the blue blade flashes with a crescent of moonlight. It glows faintly at night. */
  eldenringmoonveil(x) {
    const rack = 0x2a2420, blue = 0x8ab8f0, blueLight = 0xd8ecff, red = 0x8a1e1e, hilt = 0x1e1e26;
    fill(x - 11, 8, 2, 14, rack); fill(x + 10, 8, 2, 14, rack);
    fill(x - 10, 11, 15, 1, blue); fill(x - 10, 12, 15, 1, 0x5a7aa8); fill(x + 5, 10, 1, 4, 0xc8a050); fill(x + 6, 11, 5, 2, hilt);
    fill(x - 10, 17, 15, 1, red); fill(x - 10, 18, 15, 1, 0x5a1414); fill(x + 5, 16, 1, 4, 0x8a8e96); fill(x + 6, 17, 5, 2, 0x5a1414);
    const t = (now + x * 0.23) % 16;
    if (t < 1.2) { const a = Math.min(1, t * 3, 1.2 - t) * 1.5; ellipseRing(x - 3, 11, 9, 3, blueLight, 0.8 * Math.min(1, a)); }
    if (isNight) glowLate(() => { fill(x - 10, 11, 15, 1, blueLight, 0.7); glowAt(x - 3, 11, 8, 3, 0x8ab8f0, 0.15); });
    fill(x - 10, 24, 21, 1, SHADOW);
  },
  /** A portrait in a dark frame of the moon witch: pale blue skin, a tall wide-brimmed witch's hat, a white fur mantle, under a cold full moon; she blinks one eye now and then. */
  eldenringranni(x) {
    const frame = 0x3a3a46, frameLight = 0x5a5a6a, bg = 0x1a2040, skin = 0x9ab8d8, hat = 0x2a2a3a, fur = 0xf2f2ee;
    fill(x - 8, 3, 17, 23, frame); fill(x - 8, 3, 17, 1, frameLight); fill(x - 6, 5, 13, 19, bg);
    disc(x + 3, 9, 2.4, 0xe8eef8);
    fill(x - 4, 17, 9, 7, fur); fill(x - 4, 17, 9, 1, WHITE);
    fill(x - 2, 11, 5, 6, skin); fill(x - 3, 11, 1, 5, 0xc8402a); fill(x + 3, 11, 1, 5, 0xc8402a);
    const wink = (now + x * 0.29) % 9 < 0.3;
    dot(x - 1, 13, 0x1a2a4a); dot(x + 1, 13, wink ? skin : 0x1a2a4a); dot(x, 15, 0x6a8ab0);
    fill(x - 6, 10, 13, 1, hat); fill(x - 3, 6, 7, 4, hat); fill(x - 2, 5, 5, 1, hat); dot(x - 6, 10, 0x4a4a5a);
    fill(x - 7, 26, 15, 1, SHADOW);
  },
  /** A dark band of a plaque on the wall that is usually blank; every 45 s the words YOU DIED fade up across it in red, hang there and fade away. */
  eldenringdied(x) {
    fill(x - 14, 10, 29, 9, 0x101012, 0.9); fill(x - 14, 10, 29, 1, 0x2a2a30);
    const t = (now + x * 0.17) % 45;
    if (t < 5) {
      const a = Math.min(1, t, 5 - t) * 0.95, c = 0xb81e1e;
      fill(x - 14, 12, 29, 5, 0x3a0a0a, 0.5 * a);
      // Y O U   D I E D, three pixels wide each
      const L = { Y: [[0, 0], [2, 0], [1, 1], [1, 2], [1, 3]], O: [[0, 0], [1, 0], [2, 0], [0, 1], [2, 1], [0, 2], [2, 2], [0, 3], [1, 3], [2, 3]], U: [[0, 0], [2, 0], [0, 1], [2, 1], [0, 2], [2, 2], [0, 3], [1, 3], [2, 3]], D: [[0, 0], [1, 0], [0, 1], [2, 1], [0, 2], [2, 2], [0, 3], [1, 3]], I: [[1, 0], [1, 1], [1, 2], [1, 3], [0, 0], [2, 0], [0, 3], [2, 3]], E: [[0, 0], [1, 0], [2, 0], [0, 1], [0, 2], [1, 2], [0, 3], [1, 3], [2, 3]] };
      let cx = x - 13;
      for (const ch of 'YOU DIED') { if (ch === ' ') { cx += 3; continue; } for (const [dx, dy] of L[ch]) dot(cx + dx, 13 + dy, c, a); cx += 4; }
      if (isNight) glowLate(() => glowAt(x, 14, 14, 3, 0xb81e1e, 0.15 * a));
    }
    fill(x - 13, 19, 27, 1, SHADOW);
  },
  /** A gilt-framed portrait of Lumon's founder: a stern bewhiskered man in a dark suit against a pale field. */
  sevkierportrait(x) {
    const frame = 0xbfa24e, frameLight = 0xe0c878, frameDark = 0x8a7230, bg = 0xdfe6df, skin = 0xe3bb92, hair = 0x6a5a44, suit = 0x2a2d33;
    fill(x - 9, 4, 18, 24, frameDark); fill(x - 8, 5, 16, 22, frame); fill(x - 8, 5, 16, 1, frameLight); fill(x - 8, 26, 16, 1, frameDark); // the gilt frame
    fill(x - 6, 7, 12, 18, bg); // the pale portrait field
    fill(x - 3, 20, 6, 5, suit); fill(x - 3, 20, 6, 1, 0x3a3d45); // shoulders in a dark suit
    fill(x - 2, 13, 4, 7, skin); fill(x - 2, 13, 4, 1, 0xf0cda4); // the face
    fill(x - 3, 11, 6, 3, hair); fill(x, 11, 1, 2, bg); // the parted hair
    fill(x - 3, 15, 1, 4, hair); fill(x + 2, 15, 1, 4, hair); // mutton-chop sideburns
    dot(x - 1, 16, PALETTE.eye); dot(x + 1, 16, PALETTE.eye); fill(x - 1, 18, 3, 1, 0xb98a68); // stern eyes and set mouth
    dot(x - 7, 6, WHITE, 0.6); // a glint on the frame
  },
  /** A framed plaque of the Nine Core Principles: a teal title bar over nine neat lines of unreadable text. */
  sevprinciples(x) {
    const frame = 0x4a4f52, bg = 0xeef1ec, head = 0x2f6456, line = 0x9aa39c;
    fill(x - 8, 4, 16, 24, frame); fill(x - 7, 5, 14, 22, bg); fill(x - 7, 5, 14, 1, 0x6a7073); // the framed plaque
    fill(x - 7, 5, 14, 4, head); fill(x - 6, 6, 12, 1, 0xa8d8c8); fill(x - 6, 7, 8, 1, 0x7fc0ac); // the teal title bar
    for (let i = 0; i < 9; i++) fill(x - 6, 10 + i * 2, 3 + (hash(x, i) % 9), 1, line); // nine principle lines
    dot(x - 7, 6, WHITE, 0.5);
  },
  /** A plain institutional wall clock, white face and black rim, ticking the real time. */
  sevclock(x) {
    const rim = 0x3a3d40, face = 0xf4f6f2;
    disc(x, 16, 7, rim); disc(x, 16, 6, face); ring(x, 16, 6.2, 0xb8beb8, 0.8); // the clock
    for (let k = 0; k < 12; k++) { const a = k / 12 * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 5), 16 + Math.round(Math.sin(a) * 5), rim, 0.7); }
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12) / 12 + d.getMinutes() / 720) * Math.PI * 2 - Math.PI / 2;
    line(x, 16, x + Math.round(Math.cos(ha) * 3), 16 + Math.round(Math.sin(ha) * 3), rim); // the hour hand
    line(x, 16, x + Math.round(Math.cos(ma) * 5), 16 + Math.round(Math.sin(ma) * 5), rim); // the minute hand
    dot(x, 16, 0xd8402a); dot(x - 2, 13, WHITE, 0.6);
  },
  /** A steel doorhead with a backlit red "Break Room" sign; it glows at night. */
  sevbreakroom(x) {
    const sign = 0x8a1a12, signLight = 0xc8402a, metal = 0x6a6e70;
    fill(x - 11, 20, 22, 10, metal); fill(x - 11, 20, 22, 1, 0x8a8e90); // the steel doorhead
    fill(x - 10, 6, 20, 9, sign); fill(x - 10, 6, 20, 1, signLight); fill(x - 10, 14, 20, 1, 0x5a0e08); // the backlit sign
    for (let i = 0; i < 9; i++) fill(x - 9 + i * 2, 9, 1, 3, 0xf2d8c0, 0.9); // suggested lettering
    dot(x - 9, 7, WHITE, 0.5);
    if (isNight) glowLate(() => glowAt(x, 10, 12, 6, 0xd8402a, 0.22)); // it glows at night
  },
  /** A cheery cloth banner strung across the wall, a waffle motif and bits of blue lettering: a waffle party. */
  sevbanner(x) {
    const cloth = 0xe8e2d2, clothDark = 0xcfc8b4, string = 0x8a8e88, waffle = 0xd8a85a;
    line(x - 14, 7, x + 14, 7, string); // the string it hangs from
    fill(x - 13, 8, 26, 9, cloth); fill(x - 13, 8, 26, 1, WHITE, 0.5); fill(x - 13, 16, 26, 1, clothDark); // the banner cloth
    for (let i = -12; i < 13; i += 6) { line(x + i, 17, x + i + 3, 19, clothDark); line(x + i + 3, 19, x + i + 6, 17, clothDark); } // its scalloped hem
    fill(x - 3, 10, 6, 5, waffle); for (let gx = 0; gx < 3; gx++) for (let gy = 0; gy < 2; gy++) dot(x - 2 + gx * 2, 11 + gy * 2, 0xb5863f); // a waffle
    fill(x - 11, 11, 6, 1, 0x3a5b8c); fill(x + 5, 13, 6, 1, 0x3a5b8c); // bits of blue lettering
  },
  /** A recessed fluorescent ceiling panel on the wall, its twin tubes behind a bright diffuser; it casts a sterile glow. */
  sevlight(x) {
    const frame = 0xc4cac3, tube = 0xf6faf4;
    fill(x - 13, 6, 26, 10, frame); fill(x - 13, 6, 26, 1, 0xe0e4df); fill(x - 13, 15, 26, 1, 0x9aa09a); // the housing
    fill(x - 11, 8, 22, 6, tube); fill(x - 11, 8, 22, 1, WHITE); // the diffuser
    fill(x - 5, 8, 1, 6, 0xdfe6df); fill(x + 4, 8, 1, 6, 0xdfe6df); // the twin tubes' seams
    if (isNight) glowLate(() => glowAt(x, 11, 15, 6, 0xf0f4ec, 0.18));
    else glowAt(x, 11, 14, 5, 0xffffff, 0.06);
  },
  /** A Scranton window of grey overcast sky behind half-drawn venetian blinds, with a pull cord. */
  offwindow(x) {
    const frame = 0xc8c4b6, sky = 0x9fb0bc, blind = 0xe4e1d6, slat = 0xcdcabd, cord = 0x8a8678;
    fill(x - 17, 3, 34, 26, frame); fill(x - 16, 4, 32, 22, sky); fill(x - 16, 4, 32, 1, 0x8aa0ae); // the window and grey sky
    if (!isNight) { dot(x - 8, 9, 0xc8d4dc, 0.8); dot(x + 6, 7, 0xc8d4dc, 0.6); } else { for (let k = 0; k < 6; k++) dot(x - 14 + k * 5, 6 + (k % 3), WHITE, 0.5); }
    for (let i = 0; i < 6; i++) { fill(x - 16, 5 + i * 4, 32, 2, blind, 0.82); fill(x - 16, 6 + i * 4, 32, 1, slat, 0.82); } // the blinds
    fill(x + 13, 4, 1, 14, cord); fill(x - 17, 3, 34, 1, 0xe0dccf); fill(x - 17, 28, 34, 1, 0x9a9686);
  },
  /** A cheesy motivational poster: a snow-capped mountain under a blue sky with a bold word along the bottom. */
  offmotivation(x) {
    const frame = 0x2a2a2e, sky = 0x4a7aa8, skyLight = 0x7aa8d0, mount = 0x3a4a5a, snow = 0xe8eef2, word = 0xf2d84a;
    fill(x - 11, 4, 22, 22, frame); fill(x - 10, 5, 20, 20, sky); fill(x - 10, 5, 20, 8, skyLight); // the framed poster
    for (let i = -10; i < 11; i++) { const h = Math.round(Math.abs(Math.sin(i * 0.5)) * 6); fill(x + i, 17 - h, 1, 8 + h, mount); } // the mountain
    fill(x - 2, 11, 5, 3, snow); dot(x, 10, snow); // the snowy peak
    fill(x - 7, 21, 14, 2, word); dot(x - 10, 6, WHITE, 0.5); // the word bar
  },
  /** A whiteboard with a rising sales line in blue marker, a red heading and a marker tray. */
  offwhiteboard(x) {
    const frame = 0xb8bcc0, board = 0xf4f6f4, tray = 0x9aa0a4, blue = 0x2a6ab0, red = 0xc83a2a, green = 0x3a9a4a;
    fill(x - 15, 4, 30, 21, frame); fill(x - 14, 5, 28, 18, board); fill(x - 14, 5, 28, 1, WHITE); // the whiteboard
    fill(x - 14, 23, 28, 2, tray); // the marker tray
    let py = 19; for (let i = 0; i < 9; i++) { const ny = 19 - Math.min(13, i * 2 - (i % 2)); line(x - 13 + i * 3, py, x - 10 + i * 3, ny, blue); py = ny; } // the rising line
    fill(x - 13, 7, 10, 1, red); fill(x - 13, 9, 7, 1, 0x5a5e62); fill(x + 2, 7, 1, 12, green); fill(x - 13, 19, 26, 1, green, 0.5); // heading, note, axes
    dot(x - 12, 6, 0x4a4e52);
  },
  /** A cork notice board crowded with pinned paper memos of different colours. */
  offcorkboard(x) {
    const frame = 0x7a5a38, cork = 0xcaa36a, pin = 0xc83a2a;
    fill(x - 12, 4, 24, 22, frame); fill(x - 11, 5, 22, 20, cork); fill(x - 11, 5, 22, 1, 0xd8b47c); // the cork board
    for (let i = 0; i < 40; i++) { const h = hash(x + i, i); dot(x - 10 + h % 22, 6 + (h >>> 3) % 18, 0xb8925a, 0.5); } // cork texture
    const memos = [[-9, 7, 7, 6, 0xf2efe6], [1, 6, 8, 7, 0xeef0f4], [-8, 15, 6, 7, 0xf4ead8], [3, 15, 7, 6, 0xeaf0e8]];
    for (const [dx, dy, w, h, c] of memos) { fill(x + dx, dy, w, h, c); fill(x + dx, dy, w, 1, shaded(c, 1.08)); for (let k = 1; k < h - 1; k += 2) fill(x + dx + 1, dy + k, w - 2, 1, 0xa8a89a, 0.7); dot(x + dx + (w >> 1), dy, pin); } // pinned memos
  },
  /** A plain round office clock showing the real time, a blue second dot at its centre. */
  offclock(x) {
    const rim = 0x3a3d40, face = 0xf4f6f2;
    disc(x, 15, 7, rim); disc(x, 15, 6, face); ring(x, 15, 6.2, 0xb8beb8, 0.8);
    for (let k = 0; k < 12; k++) { const a = k / 12 * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 5), 15 + Math.round(Math.sin(a) * 5), rim, 0.7); }
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12) / 12 + d.getMinutes() / 720) * Math.PI * 2 - Math.PI / 2;
    line(x, 15, x + Math.round(Math.cos(ha) * 3), 15 + Math.round(Math.sin(ha) * 3), rim);
    line(x, 15, x + Math.round(Math.cos(ma) * 5), 15 + Math.round(Math.sin(ma) * 5), rim);
    dot(x, 15, 0x2a5a8a); dot(x - 2, 12, WHITE, 0.6);
  },
  /** A mounted Dundie award: a wooden shield with a little gold cup and an engraved nameplate. */
  offdundieplaque(x) {
    const wood = 0x7a5024, woodLight = 0x9a6a38, gold = 0xd6b25e, goldLight = 0xf0d884, plate = 0xcfc090;
    fill(x - 9, 6, 18, 20, wood); fill(x - 9, 6, 18, 1, woodLight); fill(x - 9, 25, 18, 1, 0x5a3a18); // the shield
    fill(x - 4, 9, 8, 5, gold); fill(x - 3, 8, 6, 1, goldLight); fill(x - 5, 9, 1, 2, gold); fill(x + 4, 9, 1, 2, gold); // the gold cup
    fill(x - 2, 14, 4, 2, gold); fill(x - 3, 16, 6, 1, gold); // its stem and base
    fill(x - 6, 19, 12, 4, plate); fill(x - 5, 20, 10, 1, 0x6a5a2a); fill(x - 5, 22, 7, 1, 0x6a5a2a); // the nameplate
    dot(x - 8, 7, WHITE, 0.4); if (isNight) glowAt(x, 11, 5, 4, goldLight, 0.06);
  },
  /** A chalk betting board: dark slate with a yellow heading and five runners with their odds. */
  peakyoddsboard(x) {
    const frame = 0x4a3526, slate = 0x242826, chalk = 0xe8e4d8, chalkY = 0xe8c84a;
    fill(x - 15, 4, 30, 22, frame); fill(x - 14, 5, 28, 20, slate); fill(x - 14, 5, 28, 1, 0x3a3e3c); // the slate board
    fill(x - 12, 7, 16, 1, chalkY); // a chalk heading
    for (let r = 0; r < 5; r++) { fill(x - 12, 10 + r * 3, 10 + (hash(x, r) % 6), 1, chalk, 0.85); fill(x + 7, 10 + r * 3, 5, 1, chalkY, 0.85); } // runners and odds
    dot(x - 13, 6, WHITE, 0.4);
  },
  /** A frosted pub window with a gilt lettering arc and cross muntins; it glows warmly from within at night. */
  peakywindow(x) {
    const frame = 0x3a2a1c, glass = 0x6a7a6e, gold = 0xc8a04a, goldLight = 0xe8c878, muntin = 0x2e2016;
    fill(x - 16, 3, 32, 25, frame); fill(x - 15, 4, 30, 22, glass, 0.85); fill(x - 15, 4, 30, 1, 0x8a9a8e); // the frosted glass
    fill(x - 1, 4, 2, 22, muntin); fill(x - 15, 14, 30, 2, muntin); // the cross muntins
    for (let i = -12; i <= 12; i += 2) { const h = Math.round(Math.sin((i + 12) * 0.26) * 2); dot(x + i, 9 - h, i % 4 === 0 ? goldLight : gold); } // the gilt arc
    fill(x - 8, 12, 16, 1, gold, 0.8); // an underline
    if (isNight) glowLate(() => glowAt(x, 10, 15, 10, 0xe8b85a, 0.12)); else glowAt(x, 10, 14, 9, 0xd8c89a, 0.05);
  },
  /** A sepia family portrait in an ornate gilt frame: two figures in dark suits and flat caps. */
  peakyportrait(x) {
    const frame = 0x8a6a2a, frameLight = 0xc8a04a, sepia = 0xb89a6a, sepiaDark = 0x8a6e48, dark = 0x3a2e22;
    fill(x - 10, 4, 20, 23, frame); fill(x - 9, 5, 18, 21, frameLight); fill(x - 8, 6, 16, 19, sepiaDark); fill(x - 7, 7, 14, 17, sepia); // the ornate frame
    fill(x - 5, 16, 4, 8, dark); fill(x + 1, 16, 4, 8, dark); // two figures
    fill(x - 4, 11, 2, 4, 0xcaa682); fill(x + 2, 11, 2, 4, 0xcaa682); // faces
    fill(x - 5, 10, 3, 2, 0x2a2018); fill(x + 1, 10, 3, 2, 0x2a2018); // flat caps
    dot(x - 9, 6, WHITE, 0.5);
  },
  /** A pub shelf lined with whisky bottles in amber, green and purple glass. */
  peakywhiskyshelf(x) {
    const shelf = 0x4a3526, shelfLight = 0x6a4e38, b1 = 0x7a4a1a, b2 = 0x3a5a3a, b3 = 0x5a3a5a, amber = 0xc87a2a, cork = 0xd8c090;
    fill(x - 13, 20, 26, 2, shelf); fill(x - 13, 20, 26, 1, shelfLight); // the shelf plank
    fill(x - 13, 8, 26, 2, shelf); fill(x - 13, 8, 26, 1, shelfLight); // an upper shelf
    const bot = [[-11, b1], [-7, amber], [-3, b2], [1, b1], [5, b3], [9, amber]];
    for (const [dx, c] of bot) { fill(x + dx, 12, 3, 8, c); fill(x + dx, 12, 1, 8, shaded(c, 1.2)); fill(x + dx + 1, 9, 1, 3, c); dot(x + dx + 1, 8, cork); dot(x + dx, 13, WHITE, 0.3); } // bottles
  },
  /** A tweed flat cap hung on a peg, its stiff peak catching the light. */
  peakycap(x) {
    const tweed = 0x5a5244, tweedLight = 0x766c58, tweedDark = 0x3e382d, peg = 0x6a4e38;
    fill(x - 1, 8, 2, 3, peg); dot(x, 11, 0x8a6a42); // the peg
    fill(x - 6, 13, 12, 4, tweed); fill(x - 6, 13, 12, 1, tweedLight); fill(x - 6, 16, 12, 1, tweedDark); // the crown
    fill(x - 7, 16, 8, 2, tweed); fill(x - 7, 17, 8, 1, tweedDark); // the peak
    for (let i = -5; i < 6; i += 2) dot(x + i, 14, tweedDark, 0.5); dot(x - 5, 13, WHITE, 0.3); // fleck and glint
  },
  /** A brass pub clock over a carved name plate, showing the real time. */
  peakyclock(x) {
    const brass = 0xc8a04a, brassDark = 0x8a6a2a, face = 0xe8dcc0, sign = 0x3a2a1c;
    disc(x, 13, 7, brassDark); disc(x, 13, 6, brass); disc(x, 13, 5, face); // the brass clock
    for (let k = 0; k < 12; k++) { const a = k / 12 * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 4), 13 + Math.round(Math.sin(a) * 4), brassDark, 0.8); }
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12) / 12 + d.getMinutes() / 720) * Math.PI * 2 - Math.PI / 2;
    line(x, 13, x + Math.round(Math.cos(ha) * 2.6), 13 + Math.round(Math.sin(ha) * 2.6), sign);
    line(x, 13, x + Math.round(Math.cos(ma) * 4), 13 + Math.round(Math.sin(ma) * 4), sign); dot(x, 13, brassDark);
    fill(x - 6, 22, 12, 4, sign); fill(x - 6, 22, 12, 1, 0x5a4a30); fill(x - 4, 24, 8, 1, brass); dot(x - 2, 10, WHITE, 0.5); // the name plate
  },
  /** A big Manhattan picture window: a skyline whose windows light up at night behind slim raised blinds. */
  madmwindow(x) {
    const frame = 0x8a5a32, frameLight = 0xa8703e, sky = isNight ? 0x1a2240 : 0x8ab4d8, skyLow = isNight ? 0x2a3a5a : 0xbcd8e8, b = 0x2a3344;
    fill(x - 18, 3, 36, 26, frame); fill(x - 17, 4, 34, 22, sky); fill(x - 17, 15, 34, 11, skyLow); // the window and sky
    for (let i = -16; i < 17; i += 3) { const h = 6 + hash(x + i, 1) % 10; fill(x + i, 25 - h, 2, h, b); for (let w = 0; w < h; w += 2) if (hash(x + i, w) % 3 === 0) dot(x + i + (w % 2), 25 - w, isNight ? 0xf2d86a : 0x9ab0c0, isNight ? 0.9 : 0.5); } // the skyline
    for (let i = 0; i < 4; i++) fill(x - 17, 5 + i * 3, 34, 1, 0xe8e2d2, 0.3); // raised blinds
    fill(x - 18, 3, 36, 1, frameLight); fill(x - 18, 27, 36, 1, 0x5a3b25);
    if (isNight) glowLate(() => glowAt(x, 14, 16, 10, 0xf2d86a, 0.08));
  },
  /** A bold mid-century abstract painting: fields of red, blue and orange with a dark gestural stroke. */
  madmart(x) {
    const c1 = 0xd84a2a, c2 = 0xe8a030, c3 = 0x2a6a8a, c4 = 0xe8dcc0;
    fill(x - 11, 4, 22, 22, 0x3a2e22); fill(x - 10, 5, 20, 20, c4); // the canvas and frame
    fill(x - 9, 6, 18, 8, c1); fill(x - 9, 6, 18, 1, 0xe86a4a); // a red field
    fill(x - 9, 15, 11, 9, c3); fill(x + 3, 15, 6, 9, c2); // blue and orange blocks
    fill(x - 4, 10, 8, 2, 0x2a2024, 0.7); dot(x - 9, 6, WHITE, 0.4); // a dark stroke
  },
  /** A brass sunburst wall clock, rays of two lengths around a dark face, showing the real time. */
  madmclock(x) {
    const ray = 0xc8a04a, rayLight = 0xe8c878, face = 0x2a2a30, hand = 0xe8dcc0;
    for (let k = 0; k < 16; k++) { const a = k / 16 * Math.PI * 2, r = (k % 2 ? 11 : 8); line(x, 14, x + Math.round(Math.cos(a) * r), 14 + Math.round(Math.sin(a) * r), k % 2 ? ray : rayLight); dot(x + Math.round(Math.cos(a) * r), 14 + Math.round(Math.sin(a) * r), rayLight); } // the rays
    disc(x, 14, 4, 0x8a6a2a); disc(x, 14, 3, face); // the dark face
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12) / 12 + d.getMinutes() / 720) * Math.PI * 2 - Math.PI / 2;
    line(x, 14, x + Math.round(Math.cos(ha) * 1.8), 14 + Math.round(Math.sin(ha) * 1.8), hand);
    line(x, 14, x + Math.round(Math.cos(ma) * 3), 14 + Math.round(Math.sin(ma) * 3), hand); dot(x, 14, ray);
  },
  /** A pitch board on an easel: a bold headline, a product image block and lines of copy. */
  madmboard(x) {
    const board = 0xf2ecde, frame = 0x5a3b25, img = 0x2a6a8a, red = 0xd84a2a, ink = 0x3a3730;
    fill(x - 13, 4, 26, 21, frame); fill(x - 12, 5, 24, 19, board); fill(x - 12, 5, 24, 1, WHITE); // the board
    fill(x - 10, 7, 20, 2, red); // the headline bar
    fill(x - 10, 11, 12, 10, img); fill(x - 10, 11, 12, 1, 0x4a8aaa); fill(x - 8, 14, 8, 4, 0xe8dcc0); // the image block
    for (let i = 0; i < 4; i++) fill(x + 3, 12 + i * 2, 8, 1, ink, 0.6); // copy
    fill(x - 1, 23, 2, 3, frame); dot(x - 11, 6, WHITE, 0.4); // the easel leg
  },
  /** A mid-century shelf with books, a brass sculpture and a glass decanter. */
  madmshelf(x) {
    const shelf = 0x8a5a32, shelfLight = 0xa8703e, glass = 0xcfe0e4, book = [0xd84a2a, 0x2a6a8a, 0xe8a030], gold = 0xc8a04a;
    fill(x - 14, 14, 28, 2, shelf); fill(x - 14, 14, 28, 1, shelfLight); // the shelf
    fill(x - 13, 5, 28, 2, shelf); fill(x - 13, 5, 28, 1, shelfLight); // an upper shelf
    for (let i = 0; i < 4; i++) fill(x - 12 + i * 2, 8, 1, 6, book[i % 3]); // books
    fill(x - 2, 9, 3, 5, gold); fill(x - 2, 8, 3, 1, 0xe8c878); dot(x - 1, 7, gold); // a brass sculpture
    fill(x + 5, 9, 5, 5, glass, 0.6); fill(x + 5, 9, 5, 1, WHITE, 0.4); for (let i = 0; i < 3; i++) fill(x + 6 + i, 9 + i, 1, 1, 0xc87a2a, 0.7); dot(x - 12, 6, WHITE, 0.3); // a decanter
  },
  /** A vintage ad poster: a bold product on a mustard field over a slogan banner. */
  madmposter(x) {
    const frame = 0x3a2e22, bg = 0xe8c84a, prod = 0xd84a2a, white = 0xf2ecde, blue = 0x2a6a8a;
    fill(x - 9, 4, 18, 22, frame); fill(x - 8, 5, 16, 20, bg); fill(x - 8, 5, 16, 1, 0xf2d86a); // the poster field
    fill(x - 4, 9, 8, 9, prod); fill(x - 4, 9, 8, 1, 0xe86a4a); fill(x - 3, 11, 6, 3, white); // a bold product
    fill(x - 6, 20, 12, 3, blue); fill(x - 5, 21, 10, 1, white); dot(x - 7, 6, WHITE, 0.4); // a slogan banner
  },
  /** A dark sign bearing the three game symbols in neon pink: a circle, a triangle and a square. */
  squidshapes(x) {
    const panel = 0x1a1a24, panelLight = 0x2a2a34, pink = 0xe84a7a;
    fill(x - 12, 6, 24, 18, panel); fill(x - 12, 6, 24, 1, panelLight); fill(x - 12, 23, 24, 1, 0x101018); // the dark sign
    ring(x - 6, 15, 3.4, pink); // the circle
    line(x, 12, x - 3, 18, pink); line(x, 12, x + 3, 18, pink); line(x - 3, 18, x + 3, 18, pink); // the triangle
    for (let k = 0; k < 6; k++) { dot(x + 4 + k, 12, pink); dot(x + 4 + k, 17, pink); dot(x + 4, 12 + k, pink); dot(x + 9, 12 + k, pink); } // the square
    if (isNight) glowLate(() => glowAt(x, 15, 12, 8, pink, 0.1));
  },
  /** The giant glass piggy bank hanging from the ceiling, stuffed with prize cash. */
  squidpiggybank(x) {
    const glass = 0xbfe0e4, glassEdge = 0x8ab0b4, cash = 0xd8c86a, cashDark = 0xb8a84a, gold = 0xd6b25e, cord = 0x6a6e72;
    fill(x, 4, 1, 4, cord); // it hangs from the ceiling
    fill(x - 10, 8, 20, 14, glass, 0.5); fill(x - 10, 8, 20, 1, WHITE, 0.4); fill(x - 10, 21, 20, 1, glassEdge); // the glass body
    for (let i = 0; i < 18; i += 2) fill(x - 9 + i, 15 + (hash(x, i) % 3), 2, 3, (i % 4) ? cash : cashDark); // the cash inside
    fill(x + 9, 12, 3, 3, glass, 0.6); fill(x - 2, 6, 4, 2, glass, 0.6); // snout and slot
    dot(x - 8, 10, WHITE, 0.6); if (isNight) glowAt(x, 16, 11, 7, gold, 0.08);
  },
  /** Four pastel playground ladders in pink, teal, yellow and blue, their rungs criss-crossing. */
  squidladders(x) {
    const cols = [0xf0a0b8, 0x7ad0c0, 0xf0d080, 0x90b0e0];
    for (let i = 0; i < 4; i++) { const lx = x - 12 + i * 8, c = cols[i]; fill(lx, 5, 2, 20, c); fill(lx + 5, 5, 2, 20, shaded(c, 0.9)); for (let r = 0; r < 5; r++) fill(lx, 7 + r * 4, 7, 2, c); }
  },
  /** A dark board filled with a grid of glowing player numbers. */
  squidnumbers(x) {
    const board = 0x2a2a34, boardLight = 0x3a3a44, num = 0x7ad0c0, frame = 0x4a4a54;
    fill(x - 13, 5, 26, 20, frame); fill(x - 12, 6, 24, 18, board); fill(x - 12, 6, 24, 1, boardLight); // the board
    for (let r = 0; r < 4; r++) for (let c = 0; c < 5; c++) { const h = hash(x + c, r); fill(x - 11 + c * 5, 8 + r * 4, 3, 2, h % 3 ? num : 0x5a5a64, h % 3 ? 0.9 : 0.5); } // numbers
    dot(x - 12, 6, WHITE, 0.4);
  },
  /** A framed poster of a masked guard: a pink jumpsuit and a black helmet with a shape. */
  squidguardframe(x) {
    const frame = 0x2a2a34, bg = 0xe890a8, suit = 0xe84a7a, suitDark = 0xc8305a, mask = 0x1a1a20, shape = 0xf0a0b8;
    fill(x - 9, 4, 18, 23, frame); fill(x - 8, 5, 16, 21, bg); // the framed poster
    fill(x - 5, 14, 10, 11, suit); fill(x - 5, 14, 10, 1, 0xf06a94); fill(x - 5, 24, 10, 1, suitDark); // the jumpsuit
    fill(x - 3, 8, 6, 7, mask); fill(x - 3, 8, 6, 1, 0x3a3a44); // the mask
    ring(x, 11, 2, shape); dot(x - 8, 6, WHITE, 0.4); // a shape on the mask
  },
  /** A stark facility clock with a white face, a pink second dot and the real time. */
  squidclock(x) {
    const rim = 0x2a2a34, face = 0xf0eeea, pink = 0xe84a7a;
    disc(x, 15, 7, rim); disc(x, 15, 6, face); ring(x, 15, 6.2, 0xb8b4b0, 0.8);
    for (let k = 0; k < 12; k++) { const a = k / 12 * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 5), 15 + Math.round(Math.sin(a) * 5), rim, 0.7); }
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12) / 12 + d.getMinutes() / 720) * Math.PI * 2 - Math.PI / 2;
    line(x, 15, x + Math.round(Math.cos(ha) * 3), 15 + Math.round(Math.sin(ha) * 3), rim);
    line(x, 15, x + Math.round(Math.cos(ma) * 5), 15 + Math.round(Math.sin(ma) * 5), rim);
    dot(x, 15, pink); dot(x - 2, 12, WHITE, 0.6);
  },
  /** A floor-to-ceiling corporate window over a gleaming city skyline whose towers light up at night. */
  boyswindow(x) {
    const frame = 0xb8bcc0, sky = isNight ? 0x101830 : 0x6a9ad0, skyLow = isNight ? 0x1a2444 : 0xa8c8e8, b = 0x2a3a52;
    fill(x - 18, 3, 36, 26, frame); fill(x - 17, 4, 34, 24, sky); fill(x - 17, 16, 34, 12, skyLow); // the window
    for (let i = -16; i < 17; i += 4) { const h = 8 + hash(x + i, 2) % 12; fill(x + i, 27 - h, 3, h, b); for (let w = 0; w < h; w += 2) if (hash(x + i, w) % 3 === 0) dot(x + i + (w % 3), 27 - w, isNight ? 0xf2d86a : 0xb8d0e0, isNight ? 0.9 : 0.5); } // skyline
    fill(x - 1, 4, 2, 24, frame); fill(x - 17, 15, 34, 1, frame); fill(x - 18, 3, 36, 1, 0xd0d4d6); fill(x - 18, 27, 36, 1, 0x7a7e82);
    if (isNight) glowLate(() => glowAt(x, 16, 16, 10, 0xf2d86a, 0.07));
  },
  /** A backlit team poster: seven hero silhouettes, the central one in red, under a gold star. */
  boysseven(x) {
    const bg = 0x101828, frame = 0x3a3a44, red = 0xd21f2a, hero = 0x2a3a5a, light = 0x5a6a9a, gold = 0xd6b25e;
    fill(x - 14, 4, 28, 22, frame); fill(x - 13, 5, 26, 20, bg); fill(x - 13, 5, 26, 10, 0x1a2a4a); // the poster
    for (let i = 0; i < 7; i++) { const hx = x - 11 + i * 3.4 | 0, h = 9 + (i % 3); fill(hx, 23 - h, 2, h, i === 3 ? red : hero); fill(hx, 23 - h, 2, 1, i === 3 ? 0xe8404a : light); fill(hx, 23 - h - 2, 2, 2, 0xcaa682); } // silhouettes
    for (let k = 0; k < 5; k++) { const a = k / 5 * Math.PI * 2 - Math.PI / 2; dot(x + Math.round(Math.cos(a) * 3), 9 + Math.round(Math.sin(a) * 3), gold); } // a star
    if (isNight) glowLate(() => glowAt(x, 10, 13, 6, 0x4a6aaa, 0.12));
  },
  /** A corporate approval screen: a big red percent over a little bar chart. */
  boysstat(x) {
    const screen = 0x0e1826, frame = 0x3a3a44, red = 0xd21f2a, blue = 0x3a7adc, green = 0x3ac86a, txt = 0xe8eef4;
    fill(x - 12, 4, 24, 20, frame); fill(x - 11, 5, 22, 18, screen); fill(x - 11, 5, 22, 1, 0x1a2a3a); // the screen
    fill(x - 9, 7, 10, 4, red); fill(x + 2, 7, 6, 4, txt); // the approval percent
    for (let i = 0; i < 5; i++) { const h = 3 + (hash(x, i) % 8); fill(x - 9 + i * 4, 21 - h, 3, h, [blue, green, red, blue, green][i]); } // bars
    fill(x - 9, 21, 20, 1, 0x3a4a5a); dot(x - 11, 6, 0x4a6a8a);
  },
  /** A big backlit red emblem with a white five-point star; it glows. */
  boyslogo(x) {
    const red = 0xd21f2a, redLight = 0xe8404a, redDark = 0x9a1420, steel = 0xb8bcc0;
    disc(x, 15, 10, redDark); disc(x, 15, 9, red); ring(x, 15, 9.2, steel, 0.6); // the red disc
    for (let k = 0; k < 5; k++) { const a = k / 5 * Math.PI * 2 - Math.PI / 2, a2 = (k + 0.5) / 5 * Math.PI * 2 - Math.PI / 2; line(x, 15, x + Math.round(Math.cos(a) * 7), 15 + Math.round(Math.sin(a) * 7), 0xf0f0ee); line(x, 15, x + Math.round(Math.cos(a2) * 3), 15 + Math.round(Math.sin(a2) * 3), redLight); } // the star
    disc(x, 15, 2, 0xf0f0ee);
    if (isNight) glowLate(() => glowAt(x, 15, 12, 11, red, 0.14)); else glowAt(x, 15, 11, 10, red, 0.05);
  },
  /** A glass shelf of gold cups and a crystal award. */
  boysaward(x) {
    const shelf = 0xb8bcc0, shelfLight = 0xd0d4d6, gold = 0xd6b25e, goldLight = 0xf0d884, glass = 0xcfe0e4;
    fill(x - 13, 16, 26, 2, shelf); fill(x - 13, 16, 26, 1, shelfLight); // the glass shelf
    for (let i = 0; i < 3; i++) { const tx = x - 9 + i * 8; fill(tx, 10, 4, 4, gold); fill(tx, 9, 4, 1, goldLight); fill(tx - 1, 9, 1, 2, gold); fill(tx + 4, 9, 1, 2, gold); fill(tx + 1, 14, 2, 2, gold); fill(tx, 15, 4, 1, gold); dot(tx + 1, 10, goldLight); } // gold cups
    fill(x + 7, 8, 5, 8, glass, 0.5); fill(x + 7, 8, 5, 1, WHITE, 0.4); // a crystal award
    if (isNight) glowAt(x - 5, 11, 7, 4, goldLight, 0.06);
  },
  /** A sleek steel-rimmed corporate clock with a dark face and a red second dot. */
  boysclock(x) {
    const rim = 0xb8bcc0, rimDark = 0x7a7e82, face = 0x101828, mark = 0xe8eef4, red = 0xd21f2a;
    disc(x, 14, 7, rimDark); disc(x, 14, 6, rim); disc(x, 14, 5, face); // the clock
    for (let k = 0; k < 12; k++) { const a = k / 12 * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 4), 14 + Math.round(Math.sin(a) * 4), mark, 0.8); }
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12) / 12 + d.getMinutes() / 720) * Math.PI * 2 - Math.PI / 2;
    line(x, 14, x + Math.round(Math.cos(ha) * 2.4), 14 + Math.round(Math.sin(ha) * 2.4), mark);
    line(x, 14, x + Math.round(Math.cos(ma) * 4), 14 + Math.round(Math.sin(ma) * 4), mark); dot(x, 14, red);
  },
  /** A recessed cog vault door: a heavy steel gear with rim bolts, its spokes turning slowly, and a yellow number plate. */
  falloutvaultdoor(x) {
    const steel = 0x7a7e86, steelLight = 0xa8adb6, steelDark = 0x4a4e56, inner = 0x5a5e66, bolt = 0x9aa0a8, yellow = 0xf0c020;
    disc(x, 16, 15, steelDark); disc(x, 16, 14, steel); disc(x, 16, 11, steelDark); disc(x, 16, 10, inner); // the cog door
    for (let k = 0; k < 12; k++) { const a = k / 12 * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 12.5), 16 + Math.round(Math.sin(a) * 12.5), bolt); } // rim bolts
    for (let k = 0; k < 8; k++) { const a = k / 8 * Math.PI * 2 + now * 0.05; line(x, 16, x + Math.round(Math.cos(a) * 9), 16 + Math.round(Math.sin(a) * 9), steelDark); } // spokes turning slowly
    disc(x, 16, 3, steel); ring(x, 16, 3.2, steelLight, 0.8); dot(x, 16, steelDark); // the hub
    fill(x - 4, 28, 8, 2, yellow); fill(x - 3, 29, 2, 1, steelDark); // a number plate
  },
  /** A Vault Boy poster: a blond cartoon mascot in a blue vault suit giving a cheery thumbs-up. */
  falloutvaultboy(x) {
    const frame = 0x3a3e44, bg = 0xf0ead0, skin = 0xf0d0a8, hair = 0xd8c050, suit = 0x2e4e82, yellow = 0xf0c020;
    fill(x - 9, 4, 18, 23, frame); fill(x - 8, 5, 16, 21, bg); // the framed poster
    fill(x - 3, 13, 6, 10, suit); fill(x - 3, 13, 6, 1, 0x3a5e9a); fill(x - 1, 14, 2, 8, yellow); // the vault suit
    fill(x - 2, 8, 4, 5, skin); fill(x - 2, 7, 4, 1, hair); fill(x - 1, 7, 2, 1, hair); // the head and hair
    dot(x - 1, 10, PALETTE.eye); fill(x - 1, 11, 2, 1, 0xd89088); // a wink and a grin
    fill(x + 3, 12, 2, 2, skin); fill(x + 4, 10, 1, 2, skin); dot(x - 8, 6, WHITE, 0.4); // the thumbs-up
  },
  /** A retro propaganda poster: an atom over sunburst rays with a red slogan banner. */
  falloutposter(x) {
    const frame = 0x3a3e44, sky = 0xf0c858, ray = 0xf0e0a0, blue = 0x2e4e82, white = 0xf0ead0, red = 0xd21f2a;
    fill(x - 9, 4, 18, 23, frame); fill(x - 8, 5, 16, 21, sky); // the framed poster
    for (let k = 0; k < 8; k++) { const a = k / 8 * Math.PI * 2; line(x, 11, x + Math.round(Math.cos(a) * 7), 11 + Math.round(Math.sin(a) * 7), ray, 0.5); } // sunburst rays
    disc(x, 11, 3, blue); for (let r = 0; r < 3; r++) ring(x, 11, 1 + r, white, 0.8); // an atom
    fill(x - 7, 20, 14, 3, red); fill(x - 6, 21, 12, 1, white); dot(x - 8, 6, WHITE, 0.4); // a slogan banner
  },
  /** A wall-mounted green-phosphor terminal, lines of text scrolling and a blinking cursor; it glows at night. */
  falloutterminal(x) {
    const casing = 0x5a5e5a, caseLight = 0x7a7e7a, screen = 0x0a2010, green = 0x3ae87a;
    fill(x - 12, 5, 24, 20, casing); fill(x - 12, 5, 24, 1, caseLight); fill(x - 12, 24, 24, 1, 0x3a3e3a); // the case
    fill(x - 10, 7, 20, 15, screen); // the screen
    for (let r = 0; r < 6; r++) fill(x - 9, 8 + r * 2, 4 + (hash(x, r) % 13), 1, green, 0.85); // green text
    if (Math.floor(now * 2) % 2) fill(x - 9 + (hash(x, 9) % 10), 20, 2, 1, green); // a blinking cursor
    dot(x - 11, 6, WHITE, 0.4);
    if (isNight) glowLate(() => glowAt(x, 14, 11, 8, green, 0.12));
  },
  /** A yellow hazard-striped metal sign with a vault number panel and rust spots. */
  falloutsign(x) {
    const metal = 0x6e727c, yellow = 0xf0c020, black = 0x2a2a2a, rust = 0x8a5a2a;
    fill(x - 12, 8, 24, 14, metal); fill(x - 12, 8, 24, 1, 0x9aa0a8); fill(x - 12, 21, 24, 1, 0x4a4e56); // the plate
    fill(x - 10, 10, 20, 10, yellow); for (let i = -10; i < 10; i += 4) line(x + i, 20, x + i + 3, 10, black); // hazard stripes
    fill(x - 5, 12, 10, 6, metal); fill(x - 4, 13, 8, 4, 0xe8e2d0); fill(x - 3, 14, 6, 2, black); // a number panel
    dot(x - 10, 11, rust, 0.6); dot(x + 9, 19, rust, 0.6); // rust spots
  },
  /** A retro institutional clock with a green second dot, showing the real time. */
  falloutclock(x) {
    const rim = 0x4a4e56, face = 0xe8e2d0, green = 0x3ae87a;
    disc(x, 15, 7, rim); disc(x, 15, 6, face); ring(x, 15, 6.2, 0x8a8e94, 0.8);
    for (let k = 0; k < 12; k++) { const a = k / 12 * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 5), 15 + Math.round(Math.sin(a) * 5), rim, 0.7); }
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12) / 12 + d.getMinutes() / 720) * Math.PI * 2 - Math.PI / 2;
    line(x, 15, x + Math.round(Math.cos(ha) * 3), 15 + Math.round(Math.sin(ha) * 3), rim);
    line(x, 15, x + Math.round(Math.cos(ma) * 5), 15 + Math.round(Math.sin(ma) * 5), rim);
    dot(x, 15, green); dot(x - 2, 12, WHITE, 0.6);
  },
  /** A glowing green portal swirling on the wall; its spiral turns. */
  rickportalwall(x) {
    const g1 = 0x1a9a2a, g2 = 0x3ae84a, g3 = 0x7af06a, g4 = 0xc8ffb0;
    for (let r = 11; r > 0; r--) { const c = r > 8 ? g1 : r > 5 ? g2 : r > 2 ? g3 : g4; ring(x, 16, r, c, r > 8 ? 0.8 : 0.9); } // concentric rings
    const sw = now * 1.5; for (let k = 0; k < 14; k++) { const a = sw + k * 0.5, rr = (k / 14) * 10; dot(x + Math.round(Math.cos(a) * rr), 16 + Math.round(Math.sin(a) * rr), k % 2 ? g3 : g4, 0.8); } // the swirl
    disc(x, 16, 2, g4);
    glowAt(x, 16, 12, 11, g2, isNight ? 0.22 : 0.12);
    if (isNight) glowLate(() => glowAt(x, 16, 13, 12, g2, 0.16));
  },
  /** A pegboard hung with alien tools and gadgets, one with a blinking light. */
  rickpegboard(x) {
    const board = 0xc8a060, boardDark = 0xa8803e, tool = 0x8a8e94, green = 0x3ae84a, red = 0xd84a2a, blue = 0x4a8ad8;
    fill(x - 14, 4, 28, 22, board); fill(x - 14, 4, 28, 1, 0xd8b070); fill(x - 14, 25, 28, 1, boardDark); // the pegboard
    for (let r = 0; r < 4; r++) for (let c = 0; c < 7; c++) dot(x - 12 + c * 4, 7 + r * 5, boardDark, 0.6); // peg holes
    fill(x - 11, 7, 2, 8, tool); dot(x - 11, 6, green); // a gadget
    fill(x - 5, 8, 5, 2, tool); fill(x - 3, 10, 1, 4, tool); // a wrench
    disc(x + 3, 10, 2, red); fill(x + 3, 12, 1, 4, tool); // a ray-gun shape
    fill(x + 8, 8, 4, 4, blue); dot(x + 9, 9, 0x8ac0f0); // a blue device
    fill(x - 9, 17, 8, 3, tool); fill(x - 8, 17, 1, 3, green); // a long tool
  },
  /** A chalkboard scrawled with impossible equations, a few circled in green. */
  rickchalkboard(x) {
    const frame = 0x6a4a2a, slate = 0x1a2a24, chalk = 0xe8e4d8, chalkG = 0x7af06a;
    fill(x - 15, 4, 30, 21, frame); fill(x - 14, 5, 28, 19, slate); fill(x - 14, 5, 28, 1, 0x2a3a34); // the chalkboard
    for (let r = 0; r < 5; r++) fill(x - 12, 8 + r * 3, 6 + (hash(x, r) % 16), 1, r % 3 === 0 ? chalkG : chalk, 0.8); // equations
    for (let i = 0; i < 3; i++) { const cx = x - 8 + i * 9; ring(cx, 12 + (i % 2) * 6, 1.6, chalk, 0.6); } // circled bits
    fill(x + 2, 18, 8, 1, chalkG, 0.7); dot(x - 13, 6, WHITE, 0.3);
  },
  /** A shelf of specimen jars: green goo, a brain and a couple of eyeballs. */
  rickjars(x) {
    const shelf = 0x6a4a2a, shelfLight = 0x8a6a3a, glass = 0xcfe0e4, lid = 0x8a8e94, goo = 0x6adb5a, brain = 0xe8a0b0, eye = 0xf0f0ee;
    fill(x - 13, 16, 26, 2, shelf); fill(x - 13, 16, 26, 1, shelfLight); // the shelf
    fill(x - 11, 8, 6, 8, glass, 0.5); fill(x - 11, 11, 6, 5, goo, 0.7); fill(x - 11, 7, 6, 1, lid); disc(x - 8, 13, 1.4, 0x8af06a); // a goo jar
    fill(x - 3, 8, 6, 8, glass, 0.5); fill(x - 3, 7, 6, 1, lid); disc(x, 12, 2, brain); dot(x - 1, 11, 0xf0b8c8); // a brain jar
    fill(x + 5, 9, 6, 7, glass, 0.5); fill(x + 5, 8, 6, 1, lid); disc(x + 7, 12, 1, eye); dot(x + 7, 12, PALETTE.eye); disc(x + 9, 13, 1, eye); dot(x + 9, 13, PALETTE.eye); // an eyeball jar
    dot(x - 10, 9, WHITE, 0.4);
    if (isNight) glowAt(x - 8, 13, 4, 3, goo, 0.1);
  },
  /** A teal catchphrase banner with bold green-and-white lettering and a little star. */
  rickflag(x) {
    const cloth = 0x3a8a8a, clothDark = 0x2a6a6a, green = 0x3ae84a, white = 0xf0f0ee;
    fill(x - 12, 6, 24, 12, cloth); fill(x - 12, 6, 24, 1, 0x4a9a9a); fill(x - 12, 17, 24, 1, clothDark); // the banner
    fill(x - 10, 9, 20, 2, green); fill(x - 9, 13, 14, 1, white); fill(x - 9, 15, 10, 1, white); // lettering
    disc(x + 8, 9, 1.4, green); dot(x + 8, 9, white); // a star
    for (let i = -12; i < 13; i += 5) line(x + i, 18, x + i + 2, 21, clothDark); // a ragged hem
  },
  /** A plain wonky clock with a green second dot, showing the real time. */
  rickclock(x) {
    const rim = 0x8a8e94, face = 0xe8e4d8, green = 0x3ae84a;
    disc(x, 15, 7, rim); disc(x, 15, 6, face); ring(x, 15, 6.2, 0x6a6e74, 0.8);
    for (let k = 0; k < 12; k++) { const a = k / 12 * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 5), 15 + Math.round(Math.sin(a) * 5), rim, 0.7); }
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12) / 12 + d.getMinutes() / 720) * Math.PI * 2 - Math.PI / 2;
    line(x, 15, x + Math.round(Math.cos(ha) * 3), 15 + Math.round(Math.sin(ha) * 3), 0x3a3a40);
    line(x, 15, x + Math.round(Math.cos(ma) * 5), 15 + Math.round(Math.sin(ma) * 5), 0x3a3a40);
    dot(x, 15, green); dot(x - 2, 12, WHITE, 0.6);
  },
  /** A big wall screen showing a watching eye, with the occasional glitch line. */
  mirrorscreen(x) {
    const bezel = 0x1a1c22, screen = 0x0e1826, iris = 0x4ab0f0, irisL = 0x8ad0f8, white = 0xeaf2f8, glitch = 0xe83a4a;
    fill(x - 15, 4, 30, 21, bezel); fill(x - 14, 5, 28, 19, screen); fill(x - 14, 5, 28, 1, 0x2a3a4a); // the screen
    fill(x - 10, 9, 20, 11, white, 0.9); disc(x, 14, 5, iris); disc(x, 14, 2.4, 0x0e1826); dot(x - 1, 13, irisL); // a watching eye
    if (Math.floor(now * 3) % 7 === 0) { fill(x - 14, 11 + (hash(x, 1) % 8), 28, 1, glitch, 0.6); fill(x - 14, 16, 28, 1, iris, 0.4); } // a glitch line
    if (isNight) glowLate(() => glowAt(x, 14, 14, 9, iris, 0.12)); else glowAt(x, 14, 13, 8, iris, 0.05);
  },
  /** A social-rating panel: a big score and five stars, four of them gold. */
  mirrorrating(x) {
    const panel = 0xf0f2f4, frame = 0xd8dce0, num = 0x2a2d35, gold = 0xf0c84a, grey = 0xc8cdd4;
    fill(x - 11, 4, 22, 22, frame); fill(x - 10, 5, 20, 20, panel); fill(x - 10, 5, 20, 1, WHITE); // the panel
    fill(x - 6, 8, 5, 8, num); fill(x + 1, 8, 1, 8, num); fill(x + 3, 13, 3, 3, num); // a big score
    for (let i = 0; i < 5; i++) { const sx = x - 8 + i * 4, filled = i < 4; for (let k = 0; k < 5; k++) { const a = k / 5 * Math.PI * 2 - Math.PI / 2; dot(sx + Math.round(Math.cos(a) * 1.4), 20 + Math.round(Math.sin(a) * 1.4), filled ? gold : grey); } dot(sx, 20, filled ? gold : grey); } // five stars
    dot(x - 9, 6, WHITE, 0.5);
  },
  /** A clean cityscape window with little drones drifting across, towers lit at night. */
  mirrorwindow(x) {
    const frame = 0xd8dce0, sky = isNight ? 0x10182a : 0x9ac0e0, skyLow = isNight ? 0x1a2a44 : 0xc8e0f0, tower = isNight ? 0x1a2436 : 0x8aa0b8;
    fill(x - 17, 3, 34, 26, frame); fill(x - 16, 4, 32, 22, sky); fill(x - 16, 15, 32, 11, skyLow); // the window
    for (let i = -15; i < 16; i += 5) { const h = 6 + hash(x + i, 1) % 10; fill(x + i, 26 - h, 3, h, tower); for (let w = 0; w < h; w += 3) if (isNight) dot(x + i + 1, 26 - w, 0x8ad0f8, 0.7); } // towers
    for (let i = 0; i < 3; i++) { const dx = x - 10 + ((now * 6 + i * 20) % 26 | 0), dy = 8 + i * 3; dot(dx, dy, 0x4ab0f0); dot(dx + 1, dy, 0x8ad0f8, 0.6); } // drones
    fill(x - 1, 4, 2, 22, frame); fill(x - 16, 15, 32, 1, frame); fill(x - 17, 3, 34, 1, 0xeef0f2);
  },
  /** A dark panel with a white blocked silhouette and a soft halo: someone censored. */
  mirrorblocked(x) {
    const panel = 0x1a1c22, white = 0xf4f6f8, red = 0xe83a4a, glow = 0xeaf2f8;
    fill(x - 9, 4, 18, 23, panel); fill(x - 9, 4, 18, 1, 0x2a2d35); // the panel
    fill(x - 3, 15, 6, 10, white); disc(x, 11, 3, white); // the silhouette
    for (let k = 0; k < 10; k++) { const a = k / 10 * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 9), 14 + Math.round(Math.sin(a) * 9), glow, 0.3); } // a halo
    fill(x - 6, 20, 12, 1, red, 0.4);
    if (isNight) glowLate(() => glowAt(x, 14, 10, 11, glow, 0.1));
  },
  /** A minimalist gradient orb on a glossy white canvas. */
  mirrorart(x) {
    const frame = 0xe0e4e8, bg = 0xf4f6f8, c1 = 0x4ab0f0, c2 = 0xe83a7a;
    fill(x - 11, 4, 22, 22, frame); fill(x - 10, 5, 20, 20, bg); fill(x - 10, 5, 20, 1, WHITE); // the canvas
    for (let r = 7; r > 0; r--) disc(x, 14, r, r > 4 ? c1 : c2, 0.9); // a gradient orb
    disc(x - 1, 11, 1.4, WHITE, 0.6); dot(x - 9, 6, WHITE, 0.5); // a glossy highlight
  },
  /** A minimalist digital clock glowing cyan, showing the real time. */
  mirrorclock(x) {
    const casing = 0x1a1c22, screen = 0x0e1826, cyan = 0x5adce8;
    fill(x - 9, 10, 18, 10, casing); fill(x - 8, 11, 16, 8, screen); fill(x - 8, 11, 16, 1, 0x2a3a4a);
    const d = new Date(), s = String(d.getHours()).padStart(2, '0') + String(d.getMinutes()).padStart(2, '0');
    for (let i = 0; i < 4; i++) { const n = +s[i], dx = x - 7 + i * 4 + (i > 1 ? 1 : 0); fill(dx, 13, 3, 5, 0x14242e); fill(dx, 13, 3, 1, cyan); fill(dx, 15, 3, 1, (n === 1) ? 0x14242e : cyan); fill(dx, 17, 3, 1, cyan); fill(dx, 13, 1, 3, cyan); fill(dx + 2, 15, 1, 3, cyan); }
    dot(x, 14, cyan); dot(x, 16, cyan);
    if (isNight) glowLate(() => glowAt(x, 15, 9, 5, cyan, 0.1));
  },
  /** A glass vat with a host suspended in milky fluid, faint bubbles rising; it glows softly at night. */
  westhostvat(x) {
    const frame = 0xb0b4b6, glass = 0xd0e0e0, fluid = 0xdfeae8, host = 0xc8b8a8, hostDark = 0xa89888, cyan = 0x8ad0d8;
    fill(x - 10, 4, 20, 23, frame); fill(x - 9, 5, 18, 21, glass, 0.6); fill(x - 9, 5, 18, 1, WHITE, 0.5); // the glass cylinder
    fill(x - 8, 7, 16, 18, fluid, 0.7); // the milky fluid
    fill(x - 3, 11, 6, 13, host); fill(x - 3, 11, 6, 1, hostDark); disc(x, 9, 2.4, host); // a suspended host
    fill(x - 3, 15, 6, 1, hostDark, 0.5); fill(x - 2, 11, 1, 12, 0xe8dccc, 0.4); // a spine seam and a sheen
    for (let i = 0; i < 4; i++) dot(x - 6 + (i * 4), 20 - (hash(x, i) % 6), cyan, 0.4); dot(x - 8, 6, WHITE, 0.5); // rising bubbles
    if (isNight) glowLate(() => glowAt(x, 14, 10, 11, cyan, 0.08));
  },
  /** A design screen showing a wireframe host figure in a circle beside a little DNA helix. */
  westvitruvian(x) {
    const bezel = 0x2a2d32, screen = 0x0e1a1e, line = 0x8ad0d8, lineL = 0xbfeef0, gold = 0xd6b25e;
    fill(x - 13, 4, 26, 21, bezel); fill(x - 12, 5, 24, 19, screen); fill(x - 12, 5, 24, 1, 0x2a3a40); // the screen
    ring(x - 3, 8, 1.4, line); fill(x - 4, 10, 3, 6, line, 0.7); fill(x - 6, 11, 2, 1, line); fill(x - 1, 11, 2, 1, line); fill(x - 4, 16, 1, 4, line); fill(x - 2, 16, 1, 4, line); // a wireframe figure
    ring(x - 3, 12, 6, line, 0.4); // the surrounding circle
    for (let i = 0; i < 6; i++) { const yy = 7 + i * 3; dot(x + 6 + Math.round(Math.sin(i) * 2), yy, lineL); dot(x + 6 - Math.round(Math.sin(i) * 2), yy, gold); } // a helix
    if (isNight) glowLate(() => glowAt(x, 14, 13, 9, line, 0.1));
  },
  /** A window over the desert: mesa buttes under a low sun by day, a starfield by night. */
  westwindow(x) {
    const frame = 0x7a5230, sky = isNight ? 0x1a2240 : 0xe0a860, skyLow = isNight ? 0x2a2a44 : 0xf0c878, mesa = isNight ? 0x3a2a3a : 0xa85a3a, mesaDark = isNight ? 0x2a1f2a : 0x8a4a2e;
    fill(x - 17, 3, 34, 26, frame); fill(x - 16, 4, 32, 22, sky); fill(x - 16, 14, 32, 12, skyLow); // the window and desert sky
    for (let i = -16; i < 17; i += 8) { const h = 5 + hash(x + i, 1) % 7; fill(x + i, 22 - h, 7, h + 4, mesa); fill(x + i, 22 - h, 7, 1, shaded(mesa, 1.15)); fill(x + i, 22 - h, 1, h + 4, mesaDark); } // mesa buttes
    if (isNight) { for (let k = 0; k < 8; k++) dot(x - 14 + k * 4, 6 + (k % 3), WHITE, 0.6); } else { disc(x + 8, 9, 2.4, 0xf8e088); glowAt(x + 8, 9, 4, 3, 0xf8e088, 0.2); } // stars or a low sun
    fill(x - 1, 4, 2, 22, frame); fill(x - 16, 14, 32, 1, frame); fill(x - 17, 3, 34, 1, 0x9a6a3e);
  },
  /** A round wooden plaque carved with the concentric rings of the maze. */
  westmaze(x) {
    const wood = 0x7a5230, woodLight = 0x9a6a3e, gold = 0xd6b25e, goldDark = 0x8a6a2a, dark = 0x3a2a1a;
    disc(x, 15, 9, wood); disc(x, 15, 8, woodLight); disc(x, 15, 7, wood); // the plaque
    for (let r = 6; r > 0; r -= 1.5) ring(x, 15, r, gold, 0.9); // maze rings
    for (let k = 0; k < 4; k++) { const a = k / 4 * Math.PI * 2; line(x, 15, x + Math.round(Math.cos(a) * 6), 15 + Math.round(Math.sin(a) * 6), wood); } // radial breaks
    dot(x, 15, dark); fill(x - 1, 8, 2, 2, goldDark); dot(x - 3, 11, WHITE, 0.4); // the centre and a notch
  },
  /** An aged wanted poster with a rough portrait and reward text. */
  westwanted(x) {
    const paper = 0xd8c49a, paperDark = 0xb8a47a, ink = 0x3a2a1a, face = 0x8a6a4a;
    fill(x - 9, 4, 18, 22, paper); fill(x - 9, 4, 18, 1, 0xe8d4aa); fill(x - 9, 25, 18, 1, paperDark); // the poster
    fill(x - 7, 6, 14, 2, ink); // "WANTED"
    fill(x - 5, 10, 10, 8, paperDark); fill(x - 4, 11, 8, 6, face); dot(x - 2, 13, PALETTE.eye); dot(x + 1, 13, PALETTE.eye); fill(x - 2, 15, 3, 1, 0x5a4030); // a portrait
    fill(x - 6, 20, 12, 1, ink); fill(x - 5, 22, 7, 1, ink); dot(x - 8, 5, WHITE, 0.3); // reward text
  },
  /** A set of mounted antlers over a wooden plaque. */
  westantlers(x) {
    const bone = 0xd8cdb4, boneDark = 0xb0a488, wood = 0x6a4a2a, woodLight = 0x8a6a3a;
    fill(x - 6, 18, 12, 6, wood); fill(x - 6, 18, 12, 1, woodLight); fill(x - 6, 23, 12, 1, 0x4a3018); // the plaque
    disc(x, 16, 1.6, boneDark); // the mount
    for (const dir of [-1, 1]) { line(x, 16, x + dir * 3, 12, bone); line(x + dir * 3, 12, x + dir * 6, 9, bone); line(x + dir * 3, 12, x + dir * 2, 8, bone); line(x + dir * 6, 9, x + dir * 8, 10, bone); line(x + dir * 6, 9, x + dir * 6, 6, bone); } // the antlers
    dot(x - 5, 9, WHITE, 0.3);
  },
  /** The Hill Valley clock tower: a stone tower with a clock stuck at 10:04, and a lightning bolt that cracks down it now and then. */
  bttfclocktower(x) {
    const stone = 0xb8ac8e, stoneL = 0xd0c4a4, stoneD = 0x8a7e64, face = 0xeee6d2, bolt = 0xbfeaff;
    fill(x - 5, 10, 11, 20, stone); fill(x - 5, 10, 1, 20, stoneL); fill(x + 5, 10, 1, 20, stoneD);
    for (let k = 0; k < 4; k++) fill(x - 5 - k, 6 + k, 11 + k * 2, 1, k === 3 ? stoneD : stone); // the roof
    disc(x, 16, 4, face); ring(x, 16, 4, stoneD, 0.8);
    line(x, 16, x, 13, 0x2a2a30); line(x, 16, x + 2, 17, 0x2a2a30);            // hands at 10:04
    const c = (now + x * 0.3) % 14;                                           // lightning every ~14 s
    if (c < 0.6) { const path = [0, 1, 0, -1, 0, 1, 2, 1, 0, -1]; for (let yy = 0; yy < 10; yy++) dot(x + path[yy], yy, bolt); glowAt(x, 8, 6, 8, bolt, 0.4); }
    if (isNight) glowLate(() => glowAt(x, 16, 5, 5, 0xffe0a0, 0.18));
  },
  /** A diner's neon sign: a pink-outlined face with an arrow, its tubes buzzing and flickering on at night. */
  bttfneondiner(x) {
    const frame = 0x2a2e36, pink = 0xf06aa8, cyan = 0x5ad0e0, warm = 0xf2d35b;
    fill(x - 9, 10, 18, 12, frame); fill(x - 9, 10, 18, 1, 0x3e424c); fill(x - 9, 21, 18, 1, 0x14161c);
    const on = !isNight || Math.floor(now * 6) % 7 !== 0;                     // a tube flickers at night
    ring(x, 16, 5, on ? pink : 0x7a3a58, on ? 0.9 : 0.5); fill(x - 3, 15, 6, 2, on ? warm : 0x7a6a3a);
    for (let k = -7; k <= 7; k += 2) dot(x + k, 12, on ? cyan : 0x3a6a72);     // the marquee bulbs
    fill(x + 7, 16, 3, 1, on ? pink : 0x7a3a58); fill(x + 9, 15, 1, 3, on ? pink : 0x7a3a58); // the arrow
    if (isNight && on) glowLate(() => glowAt(x, 16, 10, 7, pink, 0.22));
  },
  /** A green highway sign welcoming you to Hill Valley, on two little posts, with reflective glints in the dark. */
  bttfroadsign(x) {
    const green = 0x2f7a3a, greenL = 0x4a9a52, greenD = 0x1e5a28, white = 0xeef2ea, post = 0x9a9ea6;
    fill(x - 2, 20, 1, 10, post); fill(x + 2, 20, 1, 10, post);                // the posts
    fill(x - 11, 10, 23, 11, green); fill(x - 11, 10, 23, 1, greenL); fill(x - 11, 20, 23, 1, greenD); fill(x - 11, 10, 1, 11, greenL);
    fill(x - 9, 12, 19, 1, white); fill(x - 7, 15, 15, 1, white, 0.9); fill(x - 5, 17, 11, 2, white); // the text rows
    dot(x - 10, 11, WHITE, 0.5);
    if (isNight) { dot(x - 9, 12, WHITE, 0.6); dot(x + 8, 18, WHITE, 0.6); }
  },
  /** A 'Save the Clock Tower' flyer pinned to the wall, its corner lifting now and then as if in a draft. */
  bttfsaveclock(x) {
    const paper = 0xeee6d2, paperD = 0xcabf9e, ink = 0x3a3e46, red = 0xd8402a;
    fill(x - 6, 10, 12, 16, paper); fill(x - 6, 10, 1, 16, 0xf6efdc); fill(x + 5, 10, 1, 16, paperD);
    const t = (now + x * 0.2) % 8; if (t < 1) fill(x + 3, 10, 3, 3, paperD);   // the corner lifts
    fill(x - 4, 12, 8, 1, red); fill(x - 4, 14, 8, 1, ink, 0.7); fill(x - 4, 16, 6, 1, ink, 0.6); // headline + text
    disc(x, 20, 2, ink, 0.8); line(x, 20, x, 18, red);                         // a little clock
    dot(x, 9, red);                                                            // the pin
  },
  /** A diner menu board: a chrome-framed black board with cream lettering and a daily special that swaps now and then. */
  bttfmenuboard(x) {
    const chrome = 0xcfd6dd, chromeD = 0x8a9098, board = 0x20242a, cream = 0xeee6d2, red = 0xd8402a;
    fill(x - 9, 10, 18, 14, chrome); fill(x - 9, 10, 18, 1, 0xe6ecf2); fill(x - 9, 23, 18, 1, chromeD);
    fill(x - 7, 12, 14, 10, board);
    fill(x - 6, 13, 10, 1, red); for (let r = 0; r < 3; r++) fill(x - 6, 15 + r * 2, 8 - r, 1, cream, 0.8); // title + lines
    const sp = Math.floor((now + x) / 4) % 3, lens = [8, 6, 10];               // the special swaps
    fill(x - 6, 21, lens[sp], 1, 0xf2d35b);
  },
  /** A big round wall clock with a chrome rim showing the real time, its red second hand ticking. */
  bttfwallclock(x) {
    const rim = 0xcfd6dd, rimD = 0x8a9098, face = 0xeee6d2, ink = 0x2a2e36, red = 0xd8402a;
    disc(x, 16, 7, rim); disc(x, 16, 6, face); ring(x, 16, 6, rimD, 0.6);
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 5), 16 + Math.round(Math.sin(a) * 5), ink); }
    const d = new Date(), ha = ((d.getHours() % 12) + d.getMinutes() / 60) / 12 * Math.PI * 2 - Math.PI / 2;
    const ma = d.getMinutes() / 60 * Math.PI * 2 - Math.PI / 2, sa = d.getSeconds() / 60 * Math.PI * 2 - Math.PI / 2;
    line(x, 16, x + Math.round(Math.cos(ha) * 3), 16 + Math.round(Math.sin(ha) * 3), ink);
    line(x, 16, x + Math.round(Math.cos(ma) * 4), 16 + Math.round(Math.sin(ma) * 4), ink);
    line(x, 16, x + Math.round(Math.cos(sa) * 5), 16 + Math.round(Math.sin(sa) * 5), red);
    dot(x, 16, rimD);
  },
  /** The great wooden park gates with iron bands and a flaming torch at each side. */
  jpgate(x) {
    const wood = 0x6a4a2a, woodL = 0x8a6038, woodD = 0x442c18, iron = 0x3a3e46, flame = 0xf2a030;
    fill(x - 12, 10, 24, 20, wood); fill(x - 12, 10, 24, 1, woodL); fill(x - 1, 10, 2, 20, woodD);
    for (let k = -10; k < 12; k += 3) fill(x + k, 11, 1, 18, woodD, 0.5);
    fill(x - 12, 14, 24, 2, iron); fill(x - 12, 24, 24, 2, iron);
    for (const tx of [x - 14, x + 13]) { fill(tx, 16, 1, 10, 0x3a2e1a); const fl = 0.7 + 0.3 * Math.sin(now * 6 + tx); fill(tx - 1, 12, 3, 4, flame, fl); dot(tx, 11, 0xffe08a, fl); if (isNight) glowLate(() => glowAt(tx, 13, 4, 5, flame, 0.3 * fl)); }
  },
  /** A canvas banner on poles showing a long-neck dinosaur silhouette over amber and red bars. */
  jpbanner(x) {
    const canvas = 0xe6dcc0, canvasD = 0xc2b48c, amber = 0xe8a830, red = 0xd83020, dino = 0x2a2e22;
    fill(x - 9, 9, 18, 1, 0x6a4a2a);
    fill(x - 8, 10, 16, 14, canvas); fill(x - 8, 10, 16, 1, 0xf0e6ce); fill(x - 8, 23, 16, 1, canvasD);
    fill(x - 8, 20, 16, 2, red); fill(x - 8, 22, 16, 1, amber);
    fill(x - 4, 15, 7, 3, dino); line(x + 3, 16, x + 6, 12, dino); dot(x + 6, 11, dino); line(x - 4, 17, x - 5, 20, dino); line(x + 2, 17, x + 3, 20, dino);
    dot(x - 8, 11, WHITE, 0.4);
  },
  /** An electric-fence hazard sign: yellow-and-black stripes with a lightning bolt that flickers live. */
  jpwarnfence(x) {
    const yellow = 0xe8c838, black = 0x1e1e1a, bolt = 0xffe060, post = 0x7a7e86;
    fill(x, 18, 1, 12, post);
    fill(x - 7, 10, 14, 11, yellow); fill(x - 7, 10, 14, 1, 0xf6dc58);
    for (let k = -7; k < 7; k += 3) fill(x + k, 11, 2, 9, black);
    const live = Math.floor(now * 5) % 4 !== 0, path = [0, 1, 0, -1, 0, 1];
    for (let yy = 0; yy < 6; yy++) dot(x + path[yy], 12 + yy, live ? bolt : 0x8a7a30);
    if (isNight && live) glowLate(() => glowAt(x, 15, 3, 4, bolt, 0.22));
  },
  /** A framed park map: green land, a blue river, numbered paddock sectors and a blinking "you are here" dot. */
  jpparkmap(x) {
    const frame = 0x3a2e1a, land = 0x5a7a3a, river = 0x3a7ac0, road = 0xc8b48a, mark = 0xd83020;
    fill(x - 9, 10, 18, 14, frame); fill(x - 7, 12, 14, 10, land); fill(x - 7, 12, 14, 1, 0x6a8a4a);
    for (let yy = 13; yy < 21; yy++) fill(x - 3 + Math.round(Math.sin(yy) * 2), yy, 1, 1, river);
    fill(x - 6, 18, 12, 1, road, 0.8); fill(x - 1, 13, 1, 8, road, 0.8);
    for (const p of [[x - 4, 14], [x + 3, 15], [x + 2, 20]]) ring(p[0], p[1], 1.4, 0xe8c838, 0.8);
    dot(x - 1, 17, mark, Math.floor(now * 2) % 2 ? 1 : 0.5);
  },
  /** A fossil display: a long-jawed dinosaur skull mounted on a dark plaque, lit softly from below. */
  jpfossilwall(x) {
    const plaque = 0x2a241a, bone = 0xe6dcc0, boneD = 0xb8a884, tag = 0xe8c838;
    fill(x - 8, 10, 16, 14, plaque); fill(x - 8, 10, 16, 1, 0x3a3226);
    fill(x - 6, 14, 10, 4, bone); fill(x - 6, 14, 10, 1, 0xf2ead2);
    fill(x + 4, 15, 3, 2, bone); dot(x + 6, 16, boneD);
    for (let k = -5; k < 4; k += 2) fill(x + k, 18, 1, 2, bone);
    dot(x - 3, 15, PALETTE.eye); fill(x - 6, 20, 8, 1, boneD, 0.6);
    fill(x - 2, 22, 4, 1, tag);
    if (isNight) glowLate(() => glowAt(x - 1, 17, 7, 5, 0xffe0a0, 0.14));
  },
  /** A steel sign bolted to the wall: a red danger bar over a raptor-claw pictogram. */
  jppaddocksign(x) {
    const steel = 0x9aa0a8, steelD = 0x6a6e76, red = 0xd83020, dark = 0x1e1e1a;
    fill(x - 8, 10, 16, 13, steel); fill(x - 8, 10, 16, 1, 0xc0c6ce); fill(x - 8, 22, 16, 1, steelD);
    for (const bx of [x - 7, x + 6]) dot(bx, 11, steelD);
    fill(x - 7, 12, 14, 3, red); fill(x - 6, 13, 12, 1, 0xf2b0a0, 0.6);
    line(x - 3, 20, x + 1, 17, dark); line(x + 1, 17, x + 4, 19, dark); dot(x + 4, 19, dark); line(x - 3, 20, x - 2, 21, dark);
  },
  /** A vertical hotel neon sign in gold-green tube letters that buzz and flicker on at night. */
  wickneon(x) {
    const frame = 0x1a1a1e, tube = 0x3ad0b0, gold = 0xf2d888;
    fill(x - 4, 9, 8, 20, frame); fill(x - 4, 9, 8, 1, 0x2a2a30);
    const on = !isNight || Math.floor(now * 6) % 8 !== 0;
    for (let k = 0; k < 5; k++) { const yy = 11 + k * 3; fill(x - 2, yy, 4, 2, on ? tube : 0x1f5a4e); dot(x, yy, on ? gold : 0x5a5a3a); }
    ring(x, 25, 2, on ? tube : 0x1f5a4e, 0.9);
    if (isNight && on) glowLate(() => glowAt(x, 18, 5, 11, tube, 0.2));
  },
  /** A gilt-framed noir portrait: a figure in a dark suit against a shadowed ground, under a picture light. */
  wickportrait(x) {
    const frame = 0xd6b25e, frameD = 0x9a7e3a, bg = 0x2a2630, suit = 0x15151a, skin = 0xc89a78;
    fill(x - 8, 10, 16, 15, frame); fill(x - 8, 10, 16, 1, 0xf2d888); fill(x - 8, 24, 16, 1, frameD);
    fill(x - 6, 12, 12, 11, bg);
    fill(x - 3, 16, 6, 7, suit); fill(x - 1, 13, 3, 3, skin); fill(x - 1, 15, 3, 1, suit); // figure
    dot(x, 14, PALETTE.eye);
    fill(x - 3, 9, 6, 1, frameD); const li = 0xffe0a0; dot(x, 10, li);          // the picture light
    if (isNight) glowLate(() => glowAt(x, 12, 6, 4, li, 0.12));
  },
  /** A grand lobby clock: a gold-rimmed dark face showing the real time, Roman ticks, a slow sweep hand. */
  wickclock(x) {
    const gold = 0xd6b25e, goldD = 0x9a7e3a, face = 0x1e1e24, hand = 0xf2d888;
    disc(x, 16, 7, gold); disc(x, 16, 6, face); ring(x, 16, 6, goldD, 0.6);
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 5), 16 + Math.round(Math.sin(a) * 5), gold); }
    const d = new Date(), ha = ((d.getHours() % 12) + d.getMinutes() / 60) / 12 * Math.PI * 2 - Math.PI / 2, ma = d.getMinutes() / 60 * Math.PI * 2 - Math.PI / 2;
    line(x, 16, x + Math.round(Math.cos(ha) * 3), 16 + Math.round(Math.sin(ha) * 3), hand);
    line(x, 16, x + Math.round(Math.cos(ma) * 4), 16 + Math.round(Math.sin(ma) * 4), hand);
    dot(x, 16, gold);
  },
  /** A tall gilded mirror with a dim reflection of the room and a soft candle glow in the glass. */
  wickmirror(x) {
    const gold = 0xd6b25e, goldD = 0x9a7e3a, glass = 0x3a4048, glassL = 0x4a5058;
    fill(x - 6, 9, 12, 21, gold); fill(x - 6, 9, 12, 1, 0xf2d888); fill(x + 5, 9, 1, 21, goldD);
    fill(x - 4, 11, 8, 17, glass); fill(x - 4, 11, 8, 1, glassL);
    line(x - 3, 13, x - 1, 24, glassL, 0.5); line(x + 1, 12, x + 3, 25, glassL, 0.4); // the sheen streaks
    dot(x + 2, 15, 0xf2d888, 0.5);
    if (isNight) glowLate(() => glowAt(x, 18, 4, 8, 0xffe0a0, 0.08));
  },
  /** The management board: a dark panel of brass room-keys on hooks, a couple swinging as guests come and go. */
  wickboard(x) {
    const panel = 0x2a1e18, panelL = 0x3e2e24, brass = 0xd6b25e, brassD = 0x9a7e3a;
    fill(x - 8, 10, 16, 14, panel); fill(x - 8, 10, 16, 1, panelL); fill(x - 8, 23, 16, 1, 0x1a120e);
    for (let r = 0; r < 2; r++) for (let c = 0; c < 4; c++) { const kx = x - 6 + c * 4, ky = 13 + r * 6; dot(kx, ky, brassD); const sw = (Math.floor(now * 2 + c + r) % 7 === 0) ? 1 : 0; fill(kx, ky + 1, 1, 3, brass); dot(kx + sw, ky + 3, brass); }
  },
  /** A brass wall sconce with two candle flames that flicker warmly, brightest at night. */
  wicksconce(x) {
    const brass = 0xd6b25e, brassD = 0x9a7e3a, flame = 0xffd36b;
    fill(x - 1, 16, 3, 8, brass); fill(x - 1, 16, 1, 8, 0xf2d888); fill(x, 23, 1, 2, brassD); // the backplate
    fill(x - 5, 15, 3, 1, brass); fill(x + 3, 15, 3, 1, brass); fill(x - 4, 14, 1, 2, brassD); fill(x + 4, 14, 1, 2, brassD); // the arms
    for (const fx of [x - 4, x + 4]) { const f = candlelight(fx * 0.3); fill(fx, 11, 1, 3, 0xf2ead8); dot(fx, 10, flame, f); if (isNight) glowLate(() => glowAt(fx, 11, 4, 5, flame, 0.26 * f)); }
  },
  /** A tailor's three-panel fitting mirror in gold frames, the side panels angled, a soft reflection within. */
  kingsmantriplemirror(x) {
    const gold = 0xc8a24a, goldL = 0xe8c878, glass = 0x3a4650, glassL = 0x586470;
    for (let p = -1; p <= 1; p++) { const px = x + p * 7; fill(px - 3, 10 + (p ? 1 : 0), 6, 18 - (p ? 2 : 0), gold); fill(px - 3, 10 + (p ? 1 : 0), 6, 1, goldL);
      fill(px - 2, 12 + (p ? 1 : 0), 4, 14 - (p ? 2 : 0), glass); fill(px - 2, 12 + (p ? 1 : 0), 1, 14 - (p ? 2 : 0), glassL); }
    dot(x, 15, glassL); dot(x + 1, 20, 0xe8c878, 0.4);
  },
  /** A brass rail of bespoke suit jackets on hangers in navy, grey and tweed, swaying slightly. */
  kingsmansuitrack(x) {
    const rail = 0xc8a24a, navy = 0x1e2a44, grey = 0x4a4e58, tweed = 0x6a6050, hang = 0x9a9ea6;
    fill(x - 9, 11, 18, 1, rail);
    const coats = [navy, grey, tweed, navy, grey];
    for (let k = 0; k < 5; k++) { const cx = x - 7 + k * 3.5, sw = Math.round(Math.sin(now * 1.2 + k) * 0.6); fill(Math.round(cx) + sw, 11, 1, 2, hang); fill(Math.round(cx) - 1 + sw, 13, 4, 11, coats[k]); fill(Math.round(cx) - 1 + sw, 13, 1, 11, shaded(coats[k], 1.15)); fill(Math.round(cx) + 1 + sw, 14, 1, 7, shaded(coats[k], 0.8)); }
  },
  /** A shelf of rolled cloth bolts -- navy, houndstooth, burgundy, tweed -- their ends facing out. */
  kingsmanfabrics(x) {
    const shelf = 0x3a2a1e, navy = 0x1e2a44, burg = 0x6a2020, tweed = 0x7a7060, check = 0xc8b878;
    fill(x - 9, 10, 18, 7, shelf); fill(x - 9, 16, 18, 1, 0x241a12); fill(x - 9, 17, 18, 7, shelf); fill(x - 9, 23, 18, 1, 0x241a12);
    const rolls = [navy, check, burg, tweed, navy, check];
    for (let r = 0; r < 2; r++) for (let c = 0; c < 3; c++) { const bx = x - 7 + c * 6, by = 11 + r * 7, col = rolls[r * 3 + c]; fill(bx, by, 5, 5, col); fill(bx, by, 5, 1, shaded(col, 1.15)); disc(bx + 4, by + 2, 2, shaded(col, 0.9)); if (col === check) { dot(bx + 1, by + 1, 0x2a2a30); dot(bx + 3, by + 3, 0x2a2a30); } }
  },
  /** An oval gilt-framed portrait of a moustachioed gentleman in a dark suit. */
  kingsmanportrait(x) {
    const gold = 0xc8a24a, goldL = 0xe8c878, bg = 0x2a2e3a, suit = 0x1a1a22, skin = 0xd0a480;
    ellipseRing(x, 17, 8, 9, gold, 1); ellipseRing(x, 17, 7, 8, goldL, 0.5);
    for (let yy = 10; yy < 25; yy++) { const w = Math.round(Math.sqrt(Math.max(0, 1 - ((yy - 17) / 8) ** 2)) * 6); if (w > 0) fill(x - w, yy, w * 2, 1, bg); }
    fill(x - 3, 18, 6, 6, suit); fill(x - 1, 14, 3, 3, skin); fill(x - 2, 16, 5, 1, 0xc0946e); // body + face + moustache
    dot(x - 1, 15, PALETTE.eye); dot(x + 1, 15, PALETTE.eye);
  },
  /** A tall regulator wall clock: a dark case, a pale face with the real time, and a brass pendulum swinging below. */
  kingsmanclock(x) {
    const wood = 0x3a2a1e, woodL = 0x5a4232, brass = 0xc8a24a, face = 0xeee6d2, ink = 0x2a2a30;
    fill(x - 4, 10, 8, 20, wood); fill(x - 4, 10, 8, 1, woodL); fill(x + 3, 10, 1, 20, 0x241a12);
    disc(x, 15, 3, face); ring(x, 15, 3, brass, 0.7);
    const d = new Date(), ma = d.getMinutes() / 60 * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12) + d.getMinutes() / 60) / 12 * Math.PI * 2 - Math.PI / 2;
    line(x, 15, x + Math.round(Math.cos(ha) * 1.6), 15 + Math.round(Math.sin(ha) * 1.6), ink); line(x, 15, x + Math.round(Math.cos(ma) * 2.4), 15 + Math.round(Math.sin(ma) * 2.4), ink);
    const sw = [0, 1, 1, 0, -1, -1][Math.floor(now * 2) % 6]; line(x, 20, x + sw, 27, brass); disc(x + sw, 28, 1.4, 0xe8c878); // the pendulum
  },
  /** A pair of tailor's shears mounted open on a dark plaque, their blades catching a glint. */
  kingsmanshears(x) {
    const plaque = 0x2a1e18, steel = 0xb4b8c0, steelL = 0xe0e4e8, steelD = 0x7e828a;
    fill(x - 8, 11, 16, 12, plaque); fill(x - 8, 11, 16, 1, 0x3a2a20);
    line(x - 5, 21, x + 2, 14, steel); line(x + 5, 21, x - 2, 14, steel);       // the two blades
    dot(x, 17, steelD); ring(x - 4, 21, 1.6, steel, 0.9); ring(x + 4, 21, 1.6, steel, 0.9); // the pivot + bows
    dot(x - 1, 15, steelL, 0.8);
  },
  /** The grand factory gates: tall wrought-iron gates with gold candy finials and a big 'W' crest. */
  wonkagates(x) {
    const iron = 0x3a3a46, ironL = 0x55555f, gold = 0xe8c838, goldL = 0xf8e488, brick = 0x7a4a36;
    fill(x - 11, 26, 4, 4, brick); fill(x + 7, 26, 4, 4, brick);                // the gateposts
    fill(x - 10, 11, 20, 1, iron);
    for (let k = -9; k <= 9; k += 2) { fill(x + k, 12, 1, 16, iron); dot(x + k, 11, gold); } // the bars + finials
    fill(x - 1, 11, 2, 17, iron); fill(x - 1, 11, 2, 17, ironL, 0.4);           // the centre seam
    disc(x, 18, 4, gold); disc(x, 18, 3, iron); fill(x - 2, 16, 4, 1, goldL); line(x - 1, 16, x, 20, goldL); line(x + 1, 16, x, 20, goldL); line(x - 1, 20, x + 1, 20, goldL); // the W crest
    if (isNight) glowLate(() => glowAt(x, 18, 5, 4, gold, 0.1));
  },
  /** Great glass candy pipes carrying chocolate up the wall, a brown slug pumping along each one. */
  wonkapipes(x) {
    const glass = 0xbfe0ea, glassD = 0x7aa0b0, choc = 0x5a3418, chocL = 0x7a4a26, brass = 0xc8a24a;
    for (const px of [x - 6, x + 1]) { fill(px, 10, 5, 18, glass, 0.4); fill(px, 10, 1, 18, glass, 0.7); fill(px + 4, 10, 1, 18, glassD, 0.5);
      fill(px, 13, 5, 1, brass); fill(px, 24, 5, 1, brass);
      const t = (now * 6 + px) % 20, sy = 26 - t; fill(px + 1, sy, 3, 3, choc); fill(px + 1, sy, 3, 1, chocL); }
    if (isNight) glowLate(() => glowAt(x, 18, 6, 9, 0x8ac0d0, 0.06));
  },
  /** Lickable fruit wallpaper: a cheerful grid of painted strawberries, oranges and plums on cream. */
  wonkawallpaper(x) {
    const cream = 0xf2ead2, red = 0xd83050, orange = 0xe8782e, plum = 0x8a4ac0, leaf = 0x4aa84a;
    fill(x - 8, 10, 16, 15, cream); fill(x - 8, 10, 16, 1, 0xfbf4de);
    const fruit = [red, orange, plum];
    for (let r = 0; r < 3; r++) for (let c = 0; c < 4; c++) { const fx = x - 6 + c * 4, fy = 12 + r * 4, col = fruit[(r + c) % 3]; disc(fx, fy, 1.4, col); dot(fx, fy - 2, leaf); dot(fx - 1, fy - 1, shaded(col, 1.2)); }
  },
  /** A theatre-style marquee spelling out sweets in chasing bulbs that twinkle round the frame. */
  wonkamarquee(x) {
    const frame = 0x8a4ac0, panel = 0x2a1e3a, gold = 0xe8c838, pink = 0xf06aa8;
    fill(x - 9, 11, 18, 11, frame); fill(x - 7, 13, 14, 7, panel);
    fill(x - 5, 15, 4, 3, pink); fill(x + 1, 15, 4, 3, gold);                    // two bright letters
    const n = Math.floor(now * 4);
    for (let k = 0; k < 16; k++) { const on = (k + n) % 3 === 0; const bx = x - 8 + (k % 8) * 2.3, by = k < 8 ? 11 : 21; dot(Math.round(bx), by, on ? gold : 0x6a5a2a); }
    if (isNight) glowLate(() => glowAt(x, 16, 9, 5, pink, 0.14));
  },
  /** A shelf of glass jars brimming with coloured sweets -- gumballs, drops and swirls. */
  wonkacandyshelf(x) {
    const shelf = 0x6a3a1e, glass = 0xdff0f6, cols = [0xf06aa8, 0x5ad0a0, 0xe8c838, 0x8a4ac0, 0xd83050];
    fill(x - 9, 17, 18, 2, shelf); fill(x - 9, 17, 18, 1, 0x8a4a26); fill(x - 9, 23, 18, 2, shelf);
    for (let k = 0; k < 4; k++) { const jx = x - 7 + k * 4; fill(jx, 11, 3, 6, glass, 0.5); fill(jx, 13, 3, 4, cols[k], 0.85); for (let d = 0; d < 3; d++) dot(jx + (d % 3), 14 + d, shaded(cols[k], 1.2)); fill(jx, 10, 3, 1, 0x9aa0a8); } // the jars + lids
    for (let k = 0; k < 3; k++) { const jx = x - 6 + k * 5; fill(jx, 19, 3, 4, glass, 0.5); fill(jx, 20, 3, 3, cols[k + 1], 0.85); }
  },
  /** A whimsical candy clock: a peppermint-swirl face with mismatched hands and a lollipop pendulum. */
  wonkaclock(x) {
    const red = 0xd83050, white = 0xfff2f6, gold = 0xe8c838, mint = 0x5ad0a0;
    disc(x, 16, 6, white); for (let k = 0; k < 8; k++) { const a = (k / 8) * Math.PI * 2; if (k % 2) line(x, 16, x + Math.round(Math.cos(a) * 5), 16 + Math.round(Math.sin(a) * 5), red); } // the swirl
    ring(x, 16, 6, gold, 0.8);
    const d = new Date(), ma = d.getMinutes() / 60 * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12)) / 12 * Math.PI * 2 - Math.PI / 2;
    line(x, 16, x + Math.round(Math.cos(ha) * 2.4), 16 + Math.round(Math.sin(ha) * 2.4), 0x8a4ac0); line(x, 16, x + Math.round(Math.cos(ma) * 4), 16 + Math.round(Math.sin(ma) * 4), red);
    const sw = [0, 1, 0, -1][Math.floor(now * 2) % 4]; line(x, 22, x + sw, 26, gold); disc(x + sw, 27, 1.4, mint);
  },
  /** A gold-framed formal portrait of a pale youth holding a small golden apple, flatly centred. */
  wesportrait(x) {
    const gold = 0xc8a24a, goldL = 0xe8c878, bg = 0x3a4a3a, coat = 0x2a2a30, skin = 0xe8c0a8, apple = 0xd8a848;
    fill(x - 7, 10, 14, 15, gold); fill(x - 7, 10, 14, 1, goldL); fill(x + 6, 10, 1, 15, 0x8a6a2a);
    fill(x - 5, 12, 10, 11, bg);
    fill(x - 2, 13, 4, 3, skin); fill(x - 3, 16, 6, 6, coat); dot(x - 1, 14, PALETTE.eye); dot(x + 1, 14, PALETTE.eye);
    disc(x + 2, 19, 1.4, apple); dot(x + 2, 18, goldL);                          // the little apple
  },
  /** A symmetrical brass lobby clock on a salmon roundel, showing the real time. */
  wesclock(x) {
    const salmon = 0xe89878, brass = 0xc8a24a, face = 0xf2ead2, ink = 0x2a2a30;
    disc(x, 16, 7, salmon); disc(x, 16, 5.5, face); ring(x, 16, 5.5, brass, 0.8);
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 4.5), 16 + Math.round(Math.sin(a) * 4.5), brass); }
    const d = new Date(), ha = ((d.getHours() % 12) + d.getMinutes() / 60) / 12 * Math.PI * 2 - Math.PI / 2, ma = d.getMinutes() / 60 * Math.PI * 2 - Math.PI / 2;
    line(x, 16, x + Math.round(Math.cos(ha) * 2.6), 16 + Math.round(Math.sin(ha) * 2.6), ink); line(x, 16, x + Math.round(Math.cos(ma) * 3.6), 16 + Math.round(Math.sin(ma) * 3.6), ink);
    dot(x, 16, brass);
  },
  /** A brass sunburst mirror: a round mirror ringed by radiating gold spokes, perfectly symmetrical. */
  wessunburst(x) {
    const brass = 0xc8a24a, brassL = 0xe8c878, glass = 0x9ac0d0, glassL = 0xc0dce6;
    for (let k = 0; k < 16; k++) { const a = (k / 16) * Math.PI * 2, r1 = 5, r2 = k % 2 ? 9 : 7; line(x + Math.round(Math.cos(a) * r1), 17 + Math.round(Math.sin(a) * r1), x + Math.round(Math.cos(a) * r2), 17 + Math.round(Math.sin(a) * r2), k % 2 ? brass : brassL); }
    disc(x, 17, 4, glass); disc(x - 1, 16, 1.6, glassL); ring(x, 17, 4, brass, 0.8);
  },
  /** The reception key rack: a grid of little pigeonholes, each with a brass key tag, a few empty. */
  wespigeonholes(x) {
    const wood = 0xd8a848, woodL = 0xe8c070, woodD = 0x9a7428, hole = 0x3a2e1a, brass = 0xc8a24a;
    fill(x - 9, 10, 18, 14, wood); fill(x - 9, 10, 18, 1, woodL); fill(x - 9, 23, 18, 1, woodD);
    for (let r = 0; r < 2; r++) for (let c = 0; c < 4; c++) { const hx = x - 7 + c * 4, hy = 12 + r * 6; fill(hx, hy, 3, 4, hole); if (hash(c, r) % 4 !== 0) dot(hx + 1, hy + 3, brass); } // keys in some holes
  },
  /** A framed alpine landscape: a pale pink sky over a mustard valley and a single white peak, centred. */
  weslandscape(x) {
    const frame = 0xc8a24a, sky = 0xf0c8c0, peak = 0xf2ead2, peakD = 0xc8c0b0, valley = 0xd8a848;
    fill(x - 9, 10, 18, 14, frame); fill(x - 7, 12, 14, 10, sky); fill(x - 7, 12, 14, 1, 0xf8d8d0);
    for (let k = 0; k < 7; k++) fill(x - 3 + k, 14 + Math.abs(k - 3), 1, 20 - (14 + Math.abs(k - 3)) + 6, peak); // the peak
    fill(x - 4, 17, 1, 2, peakD); fill(x, 16, 1, 2, peakD);
    fill(x - 7, 20, 14, 2, valley); fill(x - 7, 20, 14, 1, 0xe8c070);
  },
  /** A hanging hotel banner: a pink pennant with a mustard border and a small symmetrical crest. */
  wesbanner(x) {
    const cloth = 0xe8a0a8, clothL = 0xf4c0c4, border = 0xd8a848, crest = 0x7a2a3a, gold = 0xc8a24a;
    fill(x - 7, 10, 14, 1, 0x8a6a2a);
    fill(x - 6, 11, 12, 11, cloth); fill(x - 6, 11, 12, 1, clothL); fill(x - 6, 11, 1, 11, border); fill(x + 5, 11, 1, 11, border);
    for (let k = 0; k < 4; k++) fill(x - 6 + k * 3, 22, 2, 1 + (k % 2), cloth);   // the scalloped hem
    disc(x, 15, 2, crest); dot(x, 15, gold); line(x - 2, 18, x + 2, 18, gold);
  },
  /** A back-lit monitoring world map in blue, blips pinging as aliens are tracked around the globe. */
  mibworldmap(x) {
    const frame = 0x2a2e36, sea = 0x16304a, land = 0x2a6a8a, blip = 0x5ad86a, red = 0xd83020;
    fill(x - 9, 10, 18, 14, frame); fill(x - 7, 12, 14, 10, sea);
    for (const [cx, cy, w, h] of [[-4, 14, 3, 2], [-1, 16, 4, 3], [4, 13, 3, 3], [2, 18, 2, 2]]) fill(x + cx, cy, w, h, land);
    const n = Math.floor(now * 2); for (let k = 0; k < 4; k++) { const on = (k + n) % 3 === 0; const bx = x - 5 + (k * 3), by = 14 + (k % 2) * 4; dot(bx, by, on ? (k === 1 ? red : blip) : 0x2a5a6a); if (on) ring(bx, by, 1.5, blip, 0.5); }
    if (isNight) glowLate(() => glowAt(x, 16, 9, 6, 0x2a6a9a, 0.1));
  },
  /** A bank of six small surveillance screens, each with a different flickering green feed. */
  mibscreens(x) {
    const frame = 0x1a1a1e, screen = 0x143a24, feed = 0x5ad86a;
    fill(x - 9, 10, 18, 14, frame);
    for (let r = 0; r < 2; r++) for (let c = 0; c < 3; c++) { const sx = x - 8 + c * 6, sy = 11 + r * 6; fill(sx, sy, 5, 5, screen); const on = (Math.floor(now * 3) + r * 3 + c) % 4 !== 0; if (on) { fill(sx + 1, sy + 1, 3, 1, feed, 0.7); dot(sx + 1 + (c % 2), sy + 3, feed); } }
    if (isNight) glowLate(() => glowAt(x, 17, 9, 6, feed, 0.1));
  },
  /** A row of four little world clocks -- different times, brass rims -- over labelled zones. */
  mibzoneclocks(x) {
    const brass = 0xc8a24a, face = 0xe8eaee, ink = 0x2a2a30, label = 0x3a3e46;
    for (let k = 0; k < 4; k++) { const cx = x - 6 + k * 4; disc(cx, 15, 2.2, face); ring(cx, 15, 2.2, brass, 0.8);
      const a = (k * 1.7 + now * 0.05) % (Math.PI * 2); line(cx, 15, cx + Math.round(Math.cos(a) * 1.4), 15 + Math.round(Math.sin(a) * 1.4), ink);
      fill(cx - 2, 19, 4, 2, label); fill(cx - 2, 19, 4, 1, 0x4a4e56); }
  },
  /** A recruitment poster: a black silhouette in shades under a starry sky and a single bold line of text. */
  mibposter(x) {
    const frame = 0x3a3e46, sky = 0x15151f, star = 0xbfd0ff, fig = 0x05050a, shade = 0x9ac0e0, text = 0xe8eaee;
    fill(x - 7, 10, 14, 15, frame); fill(x - 6, 11, 12, 11, sky);
    for (let k = 0; k < 6; k++) dot(x - 5 + (hash(k, 2) % 11), 12 + (hash(k, 5) % 6), star, 0.8);
    fill(x - 2, 15, 4, 7, fig); fill(x - 2, 14, 4, 2, 0x1a1a22); dot(x - 1, 15, shade); dot(x + 1, 15, shade); // the agent
    fill(x - 5, 23, 11, 1, text, 0.8);                                           // the tagline
  },
  /** Pneumatic transport tubes running up the wall, a little capsule whooshing up each one in turn. */
  mibtubes(x) {
    const glass = 0xbfe0ea, glassD = 0x7aa0b0, ring = 0x8a8e96, cap = 0xd83020, capL = 0xf26a4a;
    for (const px of [x - 6, x + 1]) { fill(px, 10, 5, 18, glass, 0.35); fill(px, 10, 1, 18, glass, 0.6); fill(px + 4, 10, 1, 18, glassD, 0.4);
      for (let y = 13; y < 28; y += 5) fill(px, y, 5, 1, ring, 0.6);
      const t = (now * 8 + (px === x - 6 ? 0 : 10)) % 20, cy = 27 - t; if (cy > 9 && cy < 28) { fill(px + 1, cy, 3, 3, cap); fill(px + 1, cy, 3, 1, capL); } }
  },
  /** An alien wanted board: three mugshots of odd creatures, a red 'WANTED' bar and a pinned note. */
  mibwanted(x) {
    const board = 0x2a2420, paper = 0xe8e2d2, red = 0xd83020, g1 = 0x5ad86a, g2 = 0x9a7ae0, g3 = 0xe8a848, ink = 0x2a2a30;
    fill(x - 9, 10, 18, 14, board); fill(x - 9, 10, 18, 1, 0x3a342e);
    fill(x - 8, 11, 16, 2, red); fill(x - 7, 11, 14, 1, 0xf26a4a);               // the WANTED bar
    const faces = [g1, g2, g3];
    for (let k = 0; k < 3; k++) { const fx = x - 6 + k * 5; fill(fx, 15, 4, 6, paper); disc(fx + 2, 17, 1.6, faces[k]); dot(fx + 1, 17, ink); dot(fx + 3, 17, ink); fill(fx, 20, 4, 1, ink, 0.4); }
    dot(x + 6, 15, red);                                                         // a pin
  },
  /** A dark wall panel of glowing cyan circuitry, a pulse of light travelling along its traces. */
  trongrid(x) {
    const bg = 0x0e1620, trace = 0x3ad0e8, traceD = 0x1a5a6a, node = 0x9ff0ff;
    fill(x - 9, 10, 18, 14, bg); fill(x - 9, 10, 18, 1, 0x1a2630);
    for (let gy = 12; gy < 23; gy += 3) fill(x - 8, gy, 16, 1, traceD, 0.6);
    for (let gx = x - 7; gx < x + 8; gx += 4) fill(gx, 11, 1, 12, traceD, 0.5);
    const p = Math.floor(now * 6) % 16; fill(x - 8, 15, p, 1, trace); dot(x - 8 + p, 15, node);
    const q = Math.floor(now * 4) % 12; fill(x + 2, 12 + q, 1, 1, trace); dot(x + 2, 11 + q, node);
    if (isNight) glowLate(() => glowAt(x, 16, 9, 6, trace, 0.14));
  },
  /** A Recognizer looming on the wall: a dark inverted-U sentinel outlined in cyan, scanning slowly. */
  tronrecognizer(x) {
    const body = 0x16222e, bodyL = 0x24323e, cyan = 0x3ad0e8, cyanL = 0x9ff0ff;
    fill(x - 7, 11, 5, 13, body); fill(x + 3, 11, 5, 13, body); fill(x - 7, 11, 15, 4, body); // the inverted U
    fill(x - 7, 11, 15, 1, cyan); fill(x - 7, 11, 1, 13, cyan, 0.7); fill(x + 7, 11, 1, 13, cyan, 0.7);
    fill(x - 2, 13, 4, 2, 0x0a1016); const sc = Math.floor(now * 2) % 3 - 1; dot(x + sc, 14, cyanL); // the scanning eye
    fill(x - 7, 23, 5, 1, cyanL, 0.7); fill(x + 3, 23, 5, 1, cyanL, 0.7);
    if (isNight) glowLate(() => glowAt(x, 15, 8, 6, cyan, 0.14));
  },
  /** The I/O tower: a slender dark spire shooting a bright white-cyan beam straight up from its tip. */
  trontower(x) {
    const tower = 0x16222e, towerL = 0x2a3a48, cyan = 0x3ad0e8, white = 0xe8f8ff;
    for (let yy = 12; yy < 30; yy++) { const w = 1 + Math.floor((30 - yy) / 6); fill(x - w, yy, w * 2, 1, tower); dot(x - w, yy, towerL); }
    const b = 0.7 + 0.3 * Math.sin(now * 4); fill(x, 10, 1, 12, white, b); fill(x - 1, 10, 3, 2, cyan, b * 0.6); dot(x, 9, white, b);
    glowAt(x, 14, 3, 8, cyan, (isNight ? 0.2 : 0.1) * b);
    if (isNight) glowLate(() => glowAt(x, 12, 4, 10, cyan, 0.26 * b));
  },
  /** A framed map of the Grid: dark terrain carved by glowing cyan canyons, a moving locator dot. */
  tronmap(x) {
    const frame = 0x1a2630, bg = 0x0a1016, cyan = 0x3ad0e8, cyanD = 0x1a5a6a, orange = 0xe8782e;
    fill(x - 9, 10, 18, 14, frame); fill(x - 7, 12, 14, 10, bg);
    for (const [ax, ay, bx, by] of [[-6, 14, 2, 14], [2, 14, 2, 20], [-4, 18, 5, 18], [-2, 12, -2, 21]]) line(x + ax, ay, x + bx, by, cyanD);
    line(x - 6, 16, x + 4, 16, cyan, 0.8);                                       // a lit route
    const t = Math.floor(now * 3) % 10; dot(x - 5 + t, 16, orange);              // the locator
    if (isNight) glowLate(() => glowAt(x, 16, 8, 5, cyan, 0.1));
  },
  /** A glowing portal ring set in the wall, a shaft of white light pouring down through its centre. */
  tronportal(x) {
    const rim = 0x3ad0e8, rimL = 0x9ff0ff, beam = 0xe8f8ff;
    ring(x, 16, 6.5, rim, 0.85); ring(x, 16, 5.5, rimL, 0.4);
    const b = 0.6 + 0.4 * Math.sin(now * 2); fill(x, 11, 1, 18, beam, b * 0.7); fill(x - 1, 11, 3, 18, rim, b * 0.3);
    for (let k = 0; k < 8; k++) { const a = (k / 8) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 6.5), 16 + Math.round(Math.sin(a) * 6.5), rimL, 0.7); }
    glowAt(x, 16, 7, 9, rim, isNight ? 0.2 : 0.1);
    if (isNight) glowLate(() => glowAt(x, 16, 9, 11, rim, 0.26));
  },
  /** A game-grid scoreboard: dark panel with two columns of glowing scores, cyan versus orange, ticking up. */
  tronleaderboard(x) {
    const bg = 0x0e1620, cyan = 0x3ad0e8, orange = 0xe8782e;
    fill(x - 9, 10, 18, 14, bg); fill(x - 9, 10, 18, 1, 0x1a2630);
    fill(x - 1, 11, 2, 12, 0x1a2630);                                            // the divider
    for (let r = 0; r < 4; r++) { const sy = 13 + r * 3, lc = 2 + ((Math.floor(now) + r) % 5), lo = 2 + ((Math.floor(now) + r + 2) % 5); fill(x - 8, sy, lc, 1, cyan, 0.85); fill(x + 2, sy, lo, 1, orange, 0.85); }
    if (isNight) glowLate(() => { glowAt(x - 4, 17, 5, 6, cyan, 0.1); glowAt(x + 4, 17, 5, 6, orange, 0.1); });
  },
  /** A round red plaque bearing the white 'turtle' kanji of the Turtle School, rope-hung on the wall. */
  dbzturtlekanji(x) {
    const red = 0xd83020, redD = 0xa01e12, white = 0xf2ead2, rope = 0xc8a060;
    fill(x - 1, 9, 2, 3, rope);
    disc(x, 17, 7, red); ring(x, 17, 7, redD, 0.6); disc(x, 17, 6, red);
    // a simple blocky kanji glyph in white
    fill(x - 3, 14, 6, 1, white); fill(x - 1, 13, 2, 8, white); fill(x - 4, 17, 8, 1, white); fill(x - 3, 19, 6, 1, white); dot(x - 4, 20, white); dot(x + 3, 20, white);
    dot(x - 3, 13, WHITE, 0.4);
  },
  /** A tournament banner: a bright cloth with a martial-arts fist emblem and two little flags. */
  dbzbanner(x) {
    const cloth = 0xe8c050, clothL = 0xf8e488, clothD = 0xb8902a, red = 0xd83020, fist = 0xc88a4a;
    fill(x - 9, 9, 18, 1, 0x8a5a32);
    fill(x - 8, 10, 16, 12, cloth); fill(x - 8, 10, 16, 1, clothL); fill(x - 8, 21, 16, 1, clothD);
    fill(x - 8, 11, 16, 2, red);                                                 // the top bar
    disc(x, 17, 3, fist); fill(x - 2, 16, 4, 2, 0xa86a2a); dot(x - 1, 16, 0xd8a86a); // the fist emblem
    fill(x - 10, 11, 2, 4, red); fill(x + 8, 11, 2, 4, red);                      // the little flags
  },
  /** The Capsule Corp sign: a blue-bordered white panel with a bold 'CC' capsule emblem. */
  dbzcapsulesign(x) {
    const blue = 0x3a6ac0, blueL = 0x5a8ae0, white = 0xf2f6fa, yellow = 0xf2c84a;
    fill(x - 9, 11, 18, 11, blue); fill(x - 9, 11, 18, 1, blueL); fill(x - 7, 13, 14, 7, white);
    disc(x - 2, 16, 2.4, yellow); ring(x - 2, 16, 2.4, blue, 0.6);                // the capsule emblem
    fill(x + 1, 14, 4, 1, blue); fill(x + 1, 16, 3, 1, blue); fill(x + 1, 18, 4, 1, blue); // "CC"-ish mark
    dot(x - 8, 12, WHITE, 0.4);
  },
  /** A hanging training scroll: cream paper with brushed black characters and red wooden rollers. */
  dbzscroll(x) {
    const paper = 0xeee6d2, paperD = 0xcabf9e, roller = 0xd83020, ink = 0x2a2a30;
    fill(x - 5, 10, 10, 1, roller); fill(x - 5, 22, 10, 1, roller);
    fill(x - 4, 11, 8, 11, paper); fill(x - 4, 11, 1, 11, 0xf6efdc); fill(x + 3, 11, 1, 11, paperD);
    for (let k = 0; k < 3; k++) { fill(x - 1, 13 + k * 3, 2, 1, ink); dot(x, 14 + k * 3, ink); dot(x - 1 + (k % 2), 13 + k * 3, ink); }
  },
  /** A wooden training board on the wall, well-worn, that shudders now and then as if just struck. */
  dbzpunchboard(x) {
    const wood = 0x8a5a32, woodL = 0xa8703e, woodD = 0x5a3b25, mark = 0x6a4228;
    const hit = (now + x * 0.3) % 5 < 0.2, dx = hit ? 1 : 0;
    fill(x - 7 + dx, 11, 14, 12, wood); fill(x - 7 + dx, 11, 14, 1, woodL); fill(x + 6 + dx, 11, 1, 12, woodD);
    for (let k = -5; k < 6; k += 3) fill(x + k + dx, 12, 1, 10, woodD, 0.4);
    disc(x + dx, 16, 2, mark, 0.6); disc(x - 3 + dx, 19, 1.4, mark, 0.5);        // the worn spots
    if (hit) { dot(x - 6, 16, 0xf2e86a); dot(x + 6, 18, 0xf2e86a); }             // impact flecks
  },
  /** A simple round dojo clock on a dark wooden mount showing the real time. */
  dbzclock(x) {
    const wood = 0x5a3b25, face = 0xeee6d2, rim = 0xd83020, ink = 0x2a2a30;
    disc(x, 16, 6, wood); disc(x, 16, 5, face); ring(x, 16, 5, rim, 0.8);
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 4), 16 + Math.round(Math.sin(a) * 4), ink); }
    const d = new Date(), ha = ((d.getHours() % 12) + d.getMinutes() / 60) / 12 * Math.PI * 2 - Math.PI / 2, ma = d.getMinutes() / 60 * Math.PI * 2 - Math.PI / 2;
    line(x, 16, x + Math.round(Math.cos(ha) * 2.4), 16 + Math.round(Math.sin(ha) * 2.4), ink); line(x, 16, x + Math.round(Math.cos(ma) * 3.4), 16 + Math.round(Math.sin(ma) * 3.4), ink);
    dot(x, 16, rim);
  },
  /** A floor vent in the wall; its lid pops open with a puff now and then -- someone just vented. */
  amongusvent(x) {
    const metal = 0x56626e, metalL = 0x74808c, metalD = 0x3a4450, dark = 0x10151a, puff = 0xbfc8d0;
    const t = (now + x * 0.3) % 11, open = t < 1;
    fill(x - 6, 16, 12, 8, metal); fill(x - 6, 16, 12, 1, metalL); fill(x - 6, 23, 12, 1, metalD);
    if (open) { fill(x - 5, 13, 10, 3, metalD); fill(x - 5, 13, 10, 1, metalL); fill(x - 4, 17, 8, 5, dark); fill(x - 3, 15, 6, 2, puff, 0.5); }
    else for (let k = -4; k < 5; k += 2) fill(x + k, 17, 1, 6, metalD);          // the slats
  },
  /** The admin map screen: a cyan wireframe of the ship's rooms with blinking crew dots moving between them. */
  amongusmap(x) {
    const frame = 0x2a333c, bg = 0x0c1824, line_ = 0x3aa0c0, room = 0x163044, dot_ = 0xf2d84a;
    fill(x - 9, 10, 18, 14, frame); fill(x - 7, 12, 14, 10, bg);
    for (const [rx, ry, rw, rh] of [[-6, 13, 4, 3], [-1, 13, 5, 4], [-5, 18, 6, 3], [2, 18, 4, 3]]) { fill(x + rx, ry, rw, rh, room); fill(x + rx, ry, rw, 1, line_, 0.5); }
    line(x - 2, 14, x + 1, 16, line_); line(x - 3, 17, x - 2, 19, line_);
    const n = Math.floor(now * 2); for (let k = 0; k < 3; k++) { const on = (k + n) % 2 === 0; dot(x - 4 + k * 3, 15 + (k % 2) * 4, on ? dot_ : 0x6a6e76); }
    if (isNight) glowLate(() => glowAt(x, 16, 9, 6, line_, 0.1));
  },
  /** An open wall wiring panel, bundles of coloured cables inside and a pulse travelling along one. */
  amonguswiring(x) {
    const panel = 0x2a333c, panelL = 0x44505a, dark = 0x10151a, cols = [0xd83020, 0x3a6ad8, 0xf2d84a, 0x4ad04a];
    fill(x - 8, 11, 16, 12, panel); fill(x - 8, 11, 16, 1, panelL); fill(x - 6, 13, 12, 8, dark);
    for (let k = 0; k < 4; k++) { const yy = 14 + k * 2; fill(x - 5, yy, 10, 1, cols[k], 0.8); }
    const p = Math.floor(now * 8) % 10; dot(x - 5 + p, 14 + (Math.floor(now) % 4) * 2, WHITE); // a pulse
    fill(x - 8, 11, 2, 2, 0x56626e); fill(x + 6, 11, 2, 2, 0x56626e);            // the hinges
  },
  /** A round ship viewport onto space: black sky, drifting stars and a slow red planet beyond the glass. */
  amongusviewport(x) {
    const rim = 0x56626e, rimD = 0x3a4450, space = 0x080c16, star = 0xbfd0ff, planet = 0xc85a3a;
    disc(x, 16, 8, rim); disc(x, 16, 7, space); ring(x, 16, 7, rimD, 0.6);
    for (let k = 0; k < 7; k++) { const sx = x - 5 + (hash(k, 2) % 11), sy = 11 + (hash(k, 5) % 10); if ((sx - x) ** 2 + (sy - 16) ** 2 < 42) dot(sx, sy, star, (Math.floor(now * 2) + k) % 3 ? 0.9 : 0.4); }
    const px = x + 3 + Math.round(Math.sin(now * 0.3) * 1); disc(px, 19, 2, planet); dot(px - 1, 18, 0xe07a5a);
    if (isNight) glowLate(() => glowAt(x, 16, 8, 8, 0x1a2a4a, 0.1));
  },
  /** A task list panel: a column of jobs with little tick boxes, a few turning green as they're done. */
  amongustasklist(x) {
    const panel = 0x1a2430, panelL = 0x2a3644, text = 0x8aa0b0, green = 0x4ad04a, box = 0x3a4450;
    fill(x - 8, 10, 16, 14, panel); fill(x - 8, 10, 16, 1, panelL);
    fill(x - 6, 11, 10, 1, 0x3a7ac0);                                            // the header
    for (let r = 0; r < 4; r++) { const ry = 14 + r * 2; const done = (Math.floor(now / 2) % 4) > r; fill(x - 6, ry, 2, 1, done ? green : box); fill(x - 3, ry, 7, 1, text, 0.6); if (done) dot(x - 5, ry, 0x9ff0a8); }
    if (isNight) glowLate(() => glowAt(x, 17, 8, 5, green, 0.08));
  },
  /** A hull-breach warning light: a red-and-black striped panel with a beacon that pulses urgently. */
  amonguswarning(x) {
    const black = 0x1a1a1e, yellow = 0xe8c838, red = 0xd83020, redL = 0xf05a40;
    fill(x - 8, 12, 16, 10, black);
    for (let k = -8; k < 8; k += 3) fill(x + k, 12, 2, 10, yellow, 0.85);        // hazard stripes
    const b = 0.5 + 0.5 * Math.sin(now * 6); disc(x, 17, 3, red, 0.5 + b * 0.5); dot(x, 17, redL, b);
    if (isNight) glowLate(() => glowAt(x, 17, 4, 4, red, 0.3 * b));
  },
  /** A tall vertical neon signboard on a bracket, three stacked tiles of abstract glyphs in pink, cyan and gold; the tiles blink on in turn and the whole thing glows at night. */
  tokyosign(x) {
    const frame = 0x2a2230;
    fill(x - 3, 3, 6, 25, frame); fill(x - 3, 3, 6, 1, 0x3e3448); fill(x + 2, 3, 1, 25, 0x18141e);
    const cols = [0xff4ab8, 0x3ae8ff, 0xf2d35b];
    for (let r = 0; r < 3; r++) {
      const on = (r + Math.floor(now * 1.5)) % 3 !== 0, c = on ? cols[r] : shaded(cols[r], 0.4);
      fill(x - 2, 5 + r * 8, 4, 6, 0x120e16);
      for (const [dx, dy] of [[0, 0], [0, 1], [0, 2], [-1, 1], [1, 3], [0, 4], [1, 0]]) dot(x + dx, 6 + r * 8 + dy, c);
    }
    fill(x - 4, 3, 2, 1, 0x3a3340); // the bracket to the wall
    if (isNight) glowLate(() => { for (let r = 0; r < 3; r++) if ((r + Math.floor(now * 1.5)) % 3 !== 0) glowAt(x, 8 + r * 8, 4, 4, cols[r], 0.18); });
  },
  /** A string of three red paper chochin lanterns hung from a rod, each with a dark cap and base and a painted band; they sway gently together and glow warm at night. */
  tokyolanterns(x) {
    fill(x - 13, 4, 26, 1, 0x2a2420); // the rod
    const sway = Math.sin(now * 1.1 + x * 0.1);
    for (let k = 0; k < 3; k++) {
      const lx = x - 9 + k * 9 + Math.round(sway * (1 + k * 0.1)), ly = 8;
      fill(lx - 1, ly - 3, 2, 1, 0x2a2420); fill(lx - 2, ly, 5, 8, 0xd83a2a); fill(lx - 3, ly + 2, 7, 4, 0xd83a2a);
      fill(lx - 2, ly, 5, 1, 0xe85a3a); fill(lx - 3, ly + 3, 7, 1, 0xf2d35b, 0.9); fill(lx - 2, ly + 8, 5, 1, 0x2a1a16);
      dot(lx - 1, ly + 1, WHITE, 0.4);
    }
    if (isNight) glowLate(() => { for (let k = 0; k < 3; k++) glowAt(x - 9 + k * 9 + Math.round(sway), 12, 5, 5, 0xffa040, 0.16); });
  },
  /** A shop noren: a wooden rod with split indigo cloth panels hanging down, a round white crest in the middle, the panels lifting now and then as if in a draught. */
  tokyonoren(x) {
    const indigo = 0x2a3e6a, indigoDark = 0x1c2c4e;
    fill(x - 11, 5, 22, 1, 0x5a3b25);
    const lift = (now % 9 < 0.8) ? 1 : 0;
    for (let k = 0; k < 4; k++) {
      const px = x - 10 + k * 5, h = 20 - (k === lift ? 3 : 0);
      fill(px, 6, 4, h, k % 2 ? indigoDark : indigo); fill(px, 6, 4, 1, 0x3e5486); fill(px, 6, 1, h, 0x34487a);
    }
    disc(x, 15, 3, 0xeae6dc); ring(x, 15, 3.2, indigoDark, 0.8); fill(x - 1, 14, 2, 2, indigo, 0.6);
  },
  /** A big lit advertising billboard in a steel frame: a bold abstract pattern of blocks in pink, cyan and white with a bar of light sweeping across it; it glows at night. */
  tokyobillboard(x) {
    const frame = 0x3a3a44;
    fill(x - 13, 4, 26, 20, frame); fill(x - 13, 4, 26, 1, 0x55555e); fill(x - 12, 5, 24, 18, 0x14101c);
    const blocks = [[-10, 7, 7, 6, 0xff4ab8], [-2, 7, 11, 4, 0x3ae8ff], [-10, 15, 5, 6, 0xf2d35b], [-3, 13, 6, 8, 0xf6f6f2], [4, 13, 7, 8, 0xff4ab8]];
    for (const [dx, dy, w, h, c] of blocks) { fill(x + dx, dy, w, h, c, 0.85); fill(x + dx, dy, w, 1, shaded(c, 1.3)); }
    // A light bar sweeping across
    const sweep = Math.floor((now * 8) % 26) - 13; fill(x + sweep, 5, 1, 18, WHITE, 0.3);
    fill(x - 2, 24, 2, 4, 0x2a2a30); // the post
    if (isNight) glowLate(() => glowAt(x, 14, 13, 9, 0xff6ab0, 0.14));
  },
  /** A glowing drink vending machine: a tall lit cabinet with rows of bright cans behind glass, a coin slot and a dim product light that hums; its face glows at night. */
  tokyovending(x) {
    const body = 0xd83a3a, bodyDark = 0xa82424;
    fill(x - 8, 6, 16, 22, body); fill(x - 8, 6, 16, 1, 0xe86a5a); fill(x + 7, 6, 1, 22, bodyDark); fill(x - 8, 27, 16, 1, 0x6a1818);
    fill(x - 6, 8, 12, 12, 0x14202a); // the glass
    const cans = [0xf2d35b, 0x3ae8ff, 0xff4ab8, 0x5aff8a, 0xf6f6f2, 0xff8a3a];
    for (let r = 0; r < 3; r++) for (let c = 0; c < 4; c++) { const col = cans[(r * 4 + c + Math.floor(now * 0.5)) % cans.length]; fill(x - 5 + c * 3, 9 + r * 4, 2, 3, col); dot(x - 5 + c * 3, 9 + r * 4, shaded(col, 1.3)); }
    fill(x - 6, 20, 12, 1, 0x3ae8ff, 0.7); // the lit shelf strip
    fill(x + 2, 22, 4, 4, 0x1a1a1e); dot(x + 4, 23, 0x3ae8ff); fill(x - 6, 22, 5, 2, 0x2a2a30); // coin slot and tray
    if (isNight) glowLate(() => glowAt(x, 14, 9, 10, 0xbfe8ff, 0.14));
  },
  /** A tangle of alley power lines: a grey transformer drum on the wall with a bundle of black cables sagging across, one loose wire swaying, and now and then a crow landing on it. */
  tokyocables(x) {
    fill(x - 4, 4, 8, 7, 0x6a6e76); fill(x - 4, 4, 8, 1, 0x8a8e96); fill(x - 4, 10, 8, 1, 0x3a3e46); // the transformer
    fill(x - 2, 11, 1, 2, 0x4a4e56); fill(x + 1, 11, 1, 2, 0x4a4e56);
    // Sagging cables
    for (const [y0, sag] of [[6, 5], [8, 7], [5, 4]]) for (let dx = -15; dx <= 15; dx++) { const yy = y0 + Math.round(Math.sin(((dx + 15) / 30) * Math.PI) * sag); dot(x + dx, yy, 0x1a1a1e); }
    // One loose wire swaying at the end
    const sway = Math.round(Math.sin(now * 0.9) * 2); line(x + 13, 7, x + 13 + sway, 16, 0x1a1a1e);
    // A crow drops by now and then
    const t = (now + x * 0.3) % 22; if (t < 7) { const cx = x - 6, cy = 5; fill(cx, cy, 3, 2, 0x12120f); dot(cx + 3, cy, 0x12120f); if (Math.floor(now * 3) % 4 === 0) dot(cx + 1, cy - 1, 0x12120f); }
  },
  /** A carved stela: a tall sandstone slab bordered in gold with three rows of painted hieroglyph squares in ochre, blue and black. */
  egypthiero(x) {
    const stone = 0xd0b074, gold = 0xd8b24e;
    fill(x - 6, 3, 12, 25, stone); fill(x - 6, 3, 12, 1, 0xe0c488); fill(x + 5, 3, 1, 25, 0x9a7a44);
    fill(x - 6, 3, 12, 1, gold); fill(x - 6, 27, 12, 1, gold); fill(x - 6, 3, 1, 25, gold); fill(x + 5, 3, 1, 25, gold);
    const cols = [0xb84a32, 0x1f4e8a, 0x3ab8a0, 0x241a12];
    for (let r = 0; r < 4; r++) for (let c = 0; c < 3; c++) { const gx = x - 4 + c * 3, gy = 6 + r * 5, col = cols[hash(gx, gy) % 4]; fill(gx, gy, 2, 3, col, 0.9); dot(gx, gy, shaded(col, 1.4)); }
  },
  /** A seated Anubis: the black jackal-headed guardian on a gold plinth, pointed ears and snout, a gold collar; its eye catches the light. */
  egyptanubis(x) {
    const black = 0x241f22, blackLight = 0x3a3238, gold = 0xd8b24e;
    fill(x - 4, 26, 8, 2, gold); fill(x - 4, 26, 8, 1, 0xf2d888); // the plinth
    fill(x - 3, 16, 6, 10, black); fill(x - 3, 16, 1, 10, blackLight); // the seated body
    fill(x - 1, 8, 4, 7, black); fill(x - 1, 8, 1, 7, blackLight); // the head/neck
    fill(x - 2, 6, 2, 3, black); fill(x + 2, 6, 2, 3, black); // the tall ears
    fill(x + 2, 11, 3, 2, black); // the snout
    fill(x - 1, 15, 4, 1, gold); dot(x, 10, 0xf2d888); // collar + eye glint
    fill(x - 3, 25, 6, 1, 0x1a1410, 0.4);
  },
  /** A painted tomb frieze: a king in profile in a blue-and-gold nemes facing an offering, a register line above and below, in the flat Egyptian style. */
  egyptpharaoh(x) {
    const stone = 0xd0b074, gold = 0xd8b24e, blue = 0x1f4e8a, ochre = 0xc87a4a, black = 0x241a12, red = 0xb84a32;
    fill(x - 13, 5, 26, 22, stone); fill(x - 13, 5, 26, 1, gold); fill(x - 13, 26, 26, 1, gold);
    // the king, in profile, facing right
    fill(x - 6, 12, 4, 10, ochre); fill(x - 7, 20, 6, 2, 0xeae2cc); // body + kilt
    fill(x - 6, 8, 5, 5, blue); for (let k = 0; k < 5; k++) fill(x - 6, 8 + k, 5, 1, k % 2 ? gold : blue); // nemes headcloth
    fill(x - 1, 10, 2, 2, ochre); fill(x - 2, 6, 1, 3, gold); // face + crown spike
    fill(x - 1, 14, 5, 1, ochre); fill(x + 3, 13, 1, 3, ochre); // outstretched arm
    // an offering stand / gods to the right
    fill(x + 6, 12, 2, 10, 0x3ab8a0); fill(x + 5, 10, 4, 2, gold); dot(x + 7, 8, red);
    for (let k = 0; k < 3; k++) dot(x - 11 + k * 2, 7, [red, blue, black][k]); // glyph row
  },
  /** A wall torch: an iron bracket holding a bundle that burns with a flickering flame, trailing a little smoke; it glows warm, brightest at night. */
  egypttorch(x) {
    const iron = 0x3a3a40, wood = 0x6a4a2e;
    fill(x - 1, 16, 2, 10, iron); fill(x - 2, 25, 4, 1, iron); // the bracket
    fill(x - 1, 13, 2, 4, wood); // the bundle
    const f = candlelight(now + x), h = 3 + Math.round(f * 2);
    fill(x - 1, 13 - h, 2, h, 0xe8782e); fill(x - 1, 12 - h, 1, 1, 0xffd36b); dot(x, 11 - h, 0xfff0c0, f);
    dot(x - 1, 14 - h, 0xf2d35b);
    // a wisp of smoke
    if ((now + x) % 3 < 1.5) dot(x, 9 - h - Math.floor(((now + x) % 1.5) * 3), 0x6a6460, 0.3);
    if (isNight) glowLate(() => { glowAt(x, 12, 6, 7, 0xffb050, 0.22); fill(x - 1, 13 - h, 2, h, 0xffa040); });
    else glowAt(x, 12, 5, 5, 0xffb050, 0.06);
  },
  /** A gold ankh set into the wall inside a painted blue niche, the looped cross gleaming; a glint runs across it now and then. */
  egyptankh(x) {
    const gold = 0xd8b24e, goldLight = 0xf2d888, blue = 0x1f4e8a;
    fill(x - 6, 4, 12, 22, blue); fill(x - 6, 4, 12, 1, 0x3a6ab0); fill(x - 5, 5, 10, 20, 0x163a66);
    ring(x, 11, 3, gold, 1); fill(x - 1, 14, 2, 10, gold); fill(x - 4, 16, 8, 2, gold); // the ankh
    fill(x - 1, 14, 1, 10, goldLight); dot(x, 9, goldLight);
    const t = (now + x * 0.3) % 6; if (t < 0.5) dot(x - 1 + Math.floor(t * 4), 15, WHITE, 0.8);
    if (isNight) glowLate(() => glowAt(x, 15, 5, 8, 0xd8b24e, 0.12));
  },
  /** A Horus falcon relief: the falcon god in profile wearing the double crown, carved and painted on a sandstone panel bordered in gold; its eye glints. */
  egyptfalcon(x) {
    const stone = 0xd0b074, gold = 0xd8b24e, black = 0x2a2420, white = 0xeae2cc, red = 0xb84a32, blue = 0x1f4e8a;
    fill(x - 9, 5, 18, 22, stone); fill(x - 9, 5, 18, 1, gold); fill(x - 9, 26, 18, 1, gold);
    // the falcon body facing right
    fill(x - 5, 14, 8, 8, black); fill(x - 5, 14, 8, 1, 0x4a4038); // body
    fill(x + 2, 11, 4, 4, black); fill(x + 5, 12, 2, 2, 0xd8a030); // head + beak
    dot(x + 3, 12, white); dot(x + 4, 12, PALETTE.eye); // the eye
    fill(x - 6, 16, 3, 5, black); // folded wing markings
    for (let k = 0; k < 4; k++) fill(x - 5, 16 + k, 7, 1, k % 2 ? 0x3a3028 : black);
    // the double crown (red over white)
    fill(x + 2, 7, 3, 4, white); fill(x + 2, 7, 3, 2, red); dot(x + 4, 6, red);
    fill(x - 5, 21, 8, 1, blue); // a base line of colour
  },
  /** A round wooden shield painted in red and cream quarters with an iron boss, two bearded axes crossed behind it. */
  vikingshield(x) {
    const red = 0xb83a2a, cream = 0xe0d2a8, iron = 0x6a6e76, wood = 0x6a4a2e;
    // Crossed axes behind
    line(x - 8, 26, x + 7, 7, wood); line(x + 8, 26, x - 7, 7, wood);
    fill(x - 9, 5, 4, 3, iron); fill(x + 6, 5, 4, 3, iron); dot(x - 8, 6, 0x9aa0a8); dot(x + 7, 6, 0x9aa0a8);
    // The round shield
    disc(x, 16, 8, wood);
    for (let dy = -7; dy <= 7; dy++) for (let dx = -7; dx <= 7; dx++) { if (Math.hypot(dx, dy) > 7.4) continue; const q = (dx >= 0) === (dy >= 0); dot(x + dx, 16 + dy, q ? red : cream); }
    ring(x, 16, 7.6, 0x4a3220, 1); disc(x, 16, 2, iron); dot(x - 1, 15, 0x9aa0a8);
  },
  /** A great stag's antlers mounted on a carved oak board, broad and many-tined, with a little iron plaque below. */
  vikingantlers(x) {
    const bone = 0xd8ccae, boneLight = 0xe8dcc0, wood = 0x5a3b25, iron = 0x6a6e76;
    fill(x - 7, 18, 14, 8, wood); fill(x - 7, 18, 14, 1, 0x7a5636); fill(x - 7, 25, 14, 1, 0x3a2818);
    fill(x - 2, 14, 4, 5, 0x9a8a6a); // the skull plate
    // Antlers sweeping up and out
    for (const dir of [-1, 1]) {
      const bx = x + dir * 2; line(bx, 15, bx + dir * 7, 4, bone); line(bx, 15, bx + dir * 7, 4, bone);
      for (const [tx, ty] of [[3, 10], [5, 7], [6, 4], [2, 12]]) { const px = bx + dir * tx; line(px, 15 - (ty - 4), px + dir * 2, 15 - ty, boneLight); }
    }
    fill(x - 3, 21, 6, 2, iron); dot(x, 22, 0xbfc4cc);
  },
  /** A woven wall tapestry of a red longship on a blue sea under a yellow sun, a row of round shields on its side, fringed at the bottom; it stirs faintly. */
  vikingtapestry(x) {
    const cloth = 0xcabf9a, sea = 0x2a5a7a, ship = 0xb83a2a, sail = 0xe0d2a8, sun = 0xe0b84a;
    fill(x - 13, 4, 26, 22, cloth); fill(x - 13, 4, 26, 1, 0xdcd0ac); fill(x - 13, 25, 26, 1, 0x9a8c66);
    fill(x - 12, 18, 24, 5, sea); for (let k = x - 12; k < x + 12; k += 3) dot(k, 19 + (hash(k, 1) % 2), 0x3a6a8a);
    disc(x + 8, 8, 2.4, sun);
    // the longship hull with a dragon prow
    fill(x - 9, 15, 18, 3, ship); fill(x - 10, 14, 2, 2, ship); dot(x - 11, 13, ship); fill(x + 8, 13, 2, 2, ship);
    for (let k = 0; k < 5; k++) disc(x - 7 + k * 3, 15, 1, k % 2 ? sea : sail); // shields on the side
    fill(x - 1, 8, 2, 7, 0x6a4a2e); fill(x - 5, 9, 10, 5, sail); fill(x - 5, 9, 10, 1, 0xf0e4c0); // mast + square sail
    const stir = Math.floor(now * 2) % 2; for (let k = x - 11; k < x + 12; k += 3) dot(k, 25 + (k % 2 ? stir : 0), 0x9a8c66);
  },
  /** A rack of three curved drinking horns hung by iron rings on a wooden rail, their silver rims gleaming. */
  vikinghornrack(x) {
    const wood = 0x5a3b25, horn = 0xcfbf96, hornDark = 0x9a8a62, silver = 0xc8ccd2, iron = 0x4a4e56;
    fill(x - 9, 7, 18, 2, wood); fill(x - 9, 7, 18, 1, 0x7a5636);
    for (let k = 0; k < 3; k++) {
      const hx = x - 7 + k * 7; dot(hx, 9, iron); // the ring
      fill(hx - 1, 10, 3, 2, horn); fill(hx, 12, 2, 4, horn); fill(hx + 1, 15, 2, 3, hornDark);
      fill(hx - 1, 10, 3, 1, silver); dot(hx, 11, 0xe8e0cc);
    }
  },
  /** A carved rune stone: a tall grey standing stone with a looping serpent band of red-painted runes around its edge and a little cross at the centre. */
  vikingrunestone(x) {
    const stone = 0x8a8a84, stoneLight = 0xa4a49e, stoneDark = 0x5a5a54, rune = 0xb83a2a;
    fill(x - 6, 5, 12, 21, stone); fill(x - 5, 4, 10, 1, stone); fill(x - 6, 5, 1, 21, stoneLight); fill(x + 5, 5, 1, 21, stoneDark);
    // the serpent band of runes round the edge
    ring(x, 15, 5.5, rune, 0.9);
    for (let k = 0; k < 8; k++) { const a = (k / 8) * Math.PI * 2, rx = x + Math.round(Math.cos(a) * 5.5), ry = 15 + Math.round(Math.sin(a) * 5.5); dot(rx, ry, 0xd8604a); }
    fill(x - 1, 12, 2, 6, rune); fill(x - 2, 14, 4, 1, rune); // a central mark
  },
  /** A wall torch in an iron ring bracket, a bound bundle burning with a flickering flame and a trail of smoke; glows warm, brightest at night. */
  vikingtorch(x) {
    const iron = 0x4a4e56, wood = 0x6a4a2e;
    ring(x, 20, 2, iron, 1); fill(x - 1, 20, 2, 6, iron);
    fill(x - 1, 14, 2, 6, wood); fill(x - 1, 14, 1, 6, 0x8a6440);
    const f = candlelight(now + x), h = 4 + Math.round(f * 3);
    fill(x - 1, 14 - h, 2, h, 0xe8782e); fill(x - 1, 13 - h, 1, 1, 0xffd36b); dot(x, 12 - h, 0xfff0c0, f);
    if ((now + x) % 3 < 1.5) dot(x, 10 - h - Math.floor(((now + x) % 1.5) * 3), 0x6a6460, 0.3);
    if (isNight) glowLate(() => { glowAt(x, 13, 6, 8, 0xffb050, 0.22); fill(x - 1, 14 - h, 2, h, 0xffa040); }); else glowAt(x, 13, 5, 5, 0xffb050, 0.06);
  },
  /** The great sun stone: a round carved calendar disc of grey stone with a central face, four ray-panels around it and a ring of glyphs, picked out in red and gold. */
  aztecsunstone(x) {
    const stone = 0x8a7f6e, stoneDark = 0x55504a, red = 0xb83a2a, gold = 0xd8b24e, jade = 0x3ab8a0;
    disc(x, 16, 11, stone); ring(x, 16, 11, stoneDark, 1); ring(x, 16, 8, red, 0.8);
    for (let k = 0; k < 8; k++) { const a = (k / 8) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 9.5), 16 + Math.round(Math.sin(a) * 9.5), k % 2 ? gold : jade); }
    for (const [dx, dy] of [[0, -6], [0, 6], [-6, 0], [6, 0]]) { fill(x + dx - 1, 16 + dy - 1, 3, 3, red, 0.7); }
    disc(x, 16, 3.5, 0x6a6054); fill(x - 2, 14, 4, 1, gold); dot(x - 1, 16, PALETTE.eye); dot(x + 1, 16, PALETTE.eye); fill(x - 1, 17, 3, 1, red); // the central face + tongue
  },
  /** A stone idol: a blocky carved temple figure with a big square headdress, round staring eyes and bared teeth, moss on its shoulders. */
  aztecidol(x) {
    const stone = 0x8a7f6e, stoneLight = 0xa89c88, stoneDark = 0x55504a, jade = 0x3ab8a0, moss = 0x4a5a3a;
    fill(x - 6, 6, 12, 5, stone); fill(x - 6, 6, 12, 1, stoneLight); fill(x - 4, 7, 8, 2, jade, 0.6); // the headdress
    fill(x - 5, 11, 10, 11, stone); fill(x - 5, 11, 1, 11, stoneLight); fill(x + 4, 11, 1, 11, stoneDark); // the face/body
    ring(x - 2, 14, 1.6, stoneDark, 1); ring(x + 2, 14, 1.6, stoneDark, 1); dot(x - 2, 14, PALETTE.eye); dot(x + 2, 14, PALETTE.eye); // round eyes
    fill(x - 3, 17, 6, 2, stoneDark); for (let k = 0; k < 3; k++) fill(x - 3 + k * 2, 17, 1, 2, stoneLight); // bared teeth
    fill(x - 5, 11, 4, 1, moss, 0.6); fill(x + 1, 11, 4, 1, moss, 0.6);
  },
  /** A feathered-serpent frieze: a long carved stone serpent undulating across the wall, its body ringed with green feathers and a fanged head, painted red and jade. */
  aztecserpent(x) {
    const stone = 0x8a7f6e, stoneDark = 0x55504a, feather = 0x3a9a5a, red = 0xb83a2a, jade = 0x3ab8a0;
    fill(x - 13, 10, 26, 10, stone); fill(x - 13, 10, 26, 1, 0xa89c88); fill(x - 13, 19, 26, 1, stoneDark);
    // the undulating serpent body
    for (let dx = -11; dx <= 7; dx++) { const by = 15 + Math.round(Math.sin((dx + 11) / 4) * 2); fill(x + dx, by, 1, 2, red); dot(x + dx, by - 1, feather); }
    // the head at the right with fangs
    fill(x + 7, 13, 5, 5, stone); fill(x + 7, 13, 5, 1, 0xa89c88); dot(x + 11, 14, PALETTE.eye); fill(x + 10, 17, 2, 1, 0xeae2cc); dot(x + 11, 18, 0xeae2cc);
    fill(x + 5, 11, 2, 1, feather); fill(x + 4, 12, 2, 1, jade); // feather crest
  },
  /** A turquoise mosaic mask mounted on the wall: a face of tiny jade-green tiles with gold around the eyes and bared white teeth, staring out. */
  aztecmask(x) {
    const jade = 0x3ab8a0, jadeDark = 0x2a8a7a, gold = 0xd8b24e, white = 0xeae2cc, black = 0x1a1a1e;
    fill(x - 6, 6, 12, 16, jadeDark);
    for (let gy = 6; gy < 20; gy++) for (let gx = -5; gx <= 5; gx++) if ((gx + gy) % 2 === 0) dot(x + gx, gy, jade);
    fill(x - 4, 11, 3, 2, gold); fill(x + 1, 11, 3, 2, gold); dot(x - 3, 11, black); dot(x + 2, 11, black); // gold-rimmed eyes
    fill(x - 3, 17, 6, 2, white); for (let k = 0; k < 3; k++) dot(x - 2 + k * 2, 17, jadeDark); // teeth
    fill(x - 6, 6, 12, 1, 0x5ad8c0);
  },
  /** A thick curtain of hanging jungle vines and broad leaves trailing down the wall, swaying gently, a bright flower here and there. */
  aztecvines(x) {
    const vine = 0x3b7a46, leaf = 0x4f9a5a, leafDark = 0x2e5e36;
    fill(x - 11, 3, 22, 2, 0x2e4a2e); // the top runner
    const sway = Math.sin(now * 0.8 + x * 0.1);
    for (let k = 0; k < 7; k++) {
      const vx = x - 9 + k * 3, len = 14 + (hash(vx, 2) % 10), off = Math.round(sway * (1 + k % 3));
      for (let d = 0; d < len; d++) dot(vx + Math.round(off * d / len), 5 + d, d % 3 === 0 ? leaf : vine);
      if (hash(vx, 5) % 2) { const ly = 5 + Math.floor(len * 0.6); fill(vx - 1 + off, ly, 3, 1, leaf); dot(vx + off, ly - 1, leafDark); }
      if (hash(vx, 9) % 3 === 0) dot(vx + off, 5 + len, 0xe84a6a); // a flower
    }
  },
  /** A carved glyph panel: a grid of Aztec day-sign glyphs in little sunken squares, painted red, jade and cream on grey stone, framed in gold. */
  aztecglyphs(x) {
    const stone = 0x8a7f6e, stoneDark = 0x55504a, gold = 0xd8b24e, cols = [0xb83a2a, 0x3ab8a0, 0xeae2cc, 0x6a6054];
    fill(x - 10, 5, 20, 20, stone); fill(x - 10, 5, 20, 1, gold); fill(x - 10, 24, 20, 1, gold); fill(x - 10, 5, 1, 20, gold); fill(x + 9, 5, 1, 20, gold);
    for (let r = 0; r < 3; r++) for (let c = 0; c < 3; c++) { const gx = x - 7 + c * 6, gy = 8 + r * 6, cc = cols[hash(gx, gy) % 4]; fill(gx, gy, 4, 4, stoneDark); fill(gx + 1, gy + 1, 2, 2, cc); dot(gx + 1, gy + 1, shaded(cc, 1.3)); }
  },
  /** A big framed star chart: a dark blue map scattered with white stars joined by faint lines into a figure, a grid behind it, in a brass frame; a star twinkles now and then. */
  obschart(x) {
    const frame = 0xc8a050, frameDark = 0x8a6a2a, sky = 0x1a2142, grid = 0x2e3a5e, star = 0xf6f6f2;
    fill(x - 12, 5, 24, 20, frame); fill(x - 12, 5, 24, 1, 0xe8c878); fill(x - 11, 6, 22, 18, sky);
    for (let gx = x - 9; gx < x + 10; gx += 4) fill(gx, 6, 1, 18, grid, 0.4); for (let gy = 8; gy < 24; gy += 4) fill(x - 11, gy, 22, 1, grid, 0.4);
    const pts = [[-7, 10], [-3, 8], [1, 12], [5, 9], [7, 15], [2, 18], [-4, 16]];
    for (let i = 0; i < pts.length - 1; i++) line(x + pts[i][0], pts[i][1], x + pts[i + 1][0], pts[i + 1][1], 0x6a7aa8, 0.7);
    pts.forEach(([dx, dy], i) => { const tw = (Math.floor(now * 2) % pts.length) === i; dot(x + dx, dy, tw ? 0xf2d888 : star); });
    void frameDark;
  },
  /** A round framed photo of a spiral galaxy: a bright core fading to violet-blue arms speckled with stars, in a thin brass ring. */
  obsconstellation(x) {
    const brass = 0xc8a050, core = 0xf2e0b0, arm = 0x6a5ad8, armDark = 0x3a3a7a;
    disc(x, 15, 9, 0x14102a); ring(x, 15, 9, brass, 1);
    for (let a = 0; a < 2 * Math.PI; a += 0.5) for (let r = 2; r < 8; r += 1.5) { const rr = r, ang = a + r * 0.5, px = x + Math.round(Math.cos(ang) * rr), py = 15 + Math.round(Math.sin(ang) * rr * 0.9); dot(px, py, r < 4 ? arm : armDark, 0.7); }
    disc(x, 15, 2.5, core); disc(x, 15, 1, WHITE);
    for (let k = 0; k < 6; k++) dot(x - 6 + hash(k, 2) % 12, 10 + hash(k, 5) % 10, WHITE, 0.6);
  },
  /** A moon-phase panel: a row of little discs showing the moon from new to full and back, carved in a dark frame; the real current phase is lit brighter. */
  obsmoonphase(x) {
    const frame = 0x2a3044, moon = 0xd8d8e0, shadow = 0x3a3e52;
    fill(x - 12, 9, 24, 12, frame); fill(x - 12, 9, 24, 1, 0x3e4458); fill(x - 12, 20, 24, 1, 0x1a1e2c);
    const phase = ((new Date().getDate() % 30) / 30);
    for (let k = 0; k < 7; k++) { const mx = x - 9 + k * 3, lit = Math.abs(k / 6 - phase) < 0.1; disc(mx, 15, 1.4, shadow); const frac = k / 6; if (frac > 0) fill(mx - 1, 14, Math.max(1, Math.round(frac * 3)), 3, lit ? 0xf2f0e0 : moon); dot(mx, 15, lit ? WHITE : moon, lit ? 1 : 0.7); }
    if (isNight) glowLate(() => glowAt(x, 15, 12, 4, 0xbfc4e0, 0.06));
  },
  /** A zodiac wheel: a round brass dial divided into twelve segments, each with a tiny star-sign mark, a slowly turning pointer in the middle. */
  obszodiac(x) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, sky = 0x1a2142;
    disc(x, 15, 9, sky); ring(x, 15, 9, brass, 1); ring(x, 15, 6, brassDark, 0.6);
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2; line(x + Math.round(Math.cos(a) * 6), 15 + Math.round(Math.sin(a) * 6), x + Math.round(Math.cos(a) * 9), 15 + Math.round(Math.sin(a) * 9), brassDark, 0.6); dot(x + Math.round(Math.cos(a) * 7.5), 15 + Math.round(Math.sin(a) * 7.5), k % 3 ? brassLight : 0xf2d888); }
    const pa = now * 0.2; line(x, 15, x + Math.round(Math.cos(pa) * 5), 15 + Math.round(Math.sin(pa) * 5), brassLight); disc(x, 15, 1.4, brass);
  },
  /** A solar-system poster: the sun at one edge and a row of planets of different sizes and colours along an arc, Saturn with its ring, on dark paper in a thin frame. */
  obsplanetposter(x) {
    const frame = 0x3a3040, sky = 0x161228, sun = 0xf2b040;
    fill(x - 11, 6, 22, 18, frame); fill(x - 10, 7, 20, 16, sky);
    disc(x - 9, 15, 3, sun); glowAt(x - 9, 15, 4, 4, sun, 0.12);
    const planets = [[-4, 0x8a7a6a, 1], [-1, 0x4a7ad8, 1.4], [2, 0xd86a3a, 1], [5, 0xe0c080, 1.6], [8, 0x5ac8e8, 1.2]];
    planets.forEach(([dx, c, r], i) => { const py = 15 + (i % 2 ? -3 : 3); disc(x + dx, py, r, c); if (i === 3) { ellipseRing(x + dx, py, 3, 1, 0xe8d8a8, 0.9); } dot(x + dx - 1, py - 1, shaded(c, 1.3)); });
  },
  /** A brass astrolabe mounted on the wall: a disc of fine engraved rings and a pierced star-pointer plate (the rete) over it, with a hanging ring at the top; it gleams. */
  obssextant(x) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, dark = 0x2a2416;
    dot(x, 4, brass); fill(x - 1, 5, 2, 2, brass); // the hanging ring
    disc(x, 16, 8, dark); ring(x, 16, 8, brass, 1); ring(x, 16, 5.5, brassDark, 0.8); ring(x, 16, 3, brassDark, 0.6);
    for (let k = 0; k < 8; k++) { const a = (k / 8) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 7), 16 + Math.round(Math.sin(a) * 7), brassLight); }
    // the pierced rete: a few pointers with star tips
    const rot = now * 0.15; for (let k = 0; k < 4; k++) { const a = rot + (k / 4) * Math.PI * 2; line(x, 16, x + Math.round(Math.cos(a) * 6), 16 + Math.round(Math.sin(a) * 6), brass); dot(x + Math.round(Math.cos(a) * 6), 16 + Math.round(Math.sin(a) * 6), brassLight); }
    disc(x, 16, 1.4, brassLight);
  },
  /** A carved wooden tiki mask mounted on the wall: a tall totem face with a heavy brow, round shell eyes, a broad nose and a toothy grimace, flanked by two small torch flames. */
  tikimask(x) {
    const wood = 0x7a5230, woodLight = 0x9a6c42, woodDark = 0x4a2e18, shell = 0xe8dcc0;
    fill(x - 5, 4, 10, 23, wood); fill(x - 5, 4, 1, 23, woodLight); fill(x + 4, 4, 1, 23, woodDark);
    fill(x - 5, 4, 10, 3, woodDark); // the brow
    fill(x - 4, 9, 3, 3, shell); fill(x + 1, 9, 3, 3, shell); dot(x - 3, 10, woodDark); dot(x + 2, 10, woodDark); // round eyes
    fill(x - 1, 12, 2, 4, woodDark); fill(x - 2, 15, 4, 1, woodDark); // nose
    fill(x - 4, 18, 8, 3, woodDark); for (let k = 0; k < 4; k++) fill(x - 3 + k * 2, 18, 1, 3, shell); // toothy mouth
    for (const tx of [x - 7, x + 7]) { const f = candlelight(now + tx); fill(tx, 10 - Math.round(f * 2), 1, 3, 0xe8782e); dot(tx, 8, 0xffd36b, f); }
    if (isNight) glowLate(() => { for (const tx of [x - 7, x + 7]) glowAt(tx, 10, 3, 4, 0xffb050, 0.18); });
  },
  /** A pair of surfboards leaning on the wall: a teal one with a white stripe and a coral one with a sun, their fins showing, beside a hung flower lei. */
  tikisurfboard(x) {
    const teal = 0x2a9aa8, coral = 0xe8604a, white = 0xf0ece0, sun = 0xf2c040, lei = 0xf06a9a;
    fill(x - 4, 4, 3, 22, teal); fill(x - 4, 4, 1, 22, 0x3ab8c0); fill(x - 3, 6, 1, 18, white); fill(x - 3, 25, 2, 2, teal); // teal board + fin
    fill(x + 1, 4, 3, 22, coral); fill(x + 1, 4, 1, 22, 0xf2806a); disc(x + 2, 10, 1.4, sun); fill(x + 1, 25, 2, 2, coral); // coral board + sun
    for (let k = 0; k < 5; k++) dot(x - 1 + Math.round(Math.sin(now * 1.2 + k) * 0.5), 6 + k * 4, k % 2 ? lei : 0xf2b040); // the hung lei between
  },
  /** A bamboo tiki torch: a tall pole with a bowl of flame at the top that flickers and trails a little smoke; it glows warm, brightest at night. */
  tikitorch(x) {
    const bamboo = 0xb89a5a, bambooDark = 0x8a6a3a, bowl = 0x3a2a1e;
    fill(x - 1, 16, 2, 11, bamboo); fill(x - 1, 16, 1, 11, 0xd8bc7e); for (const ny of [19, 23]) fill(x - 1, ny, 2, 1, bambooDark);
    fill(x - 2, 14, 4, 2, bowl); fill(x - 2, 14, 4, 1, 0x5a4330);
    const f = candlelight(now + x), h = 4 + Math.round(f * 3);
    fill(x - 1, 14 - h, 2, h, 0xe8782e); fill(x - 1, 13 - h, 1, 1, 0xffd36b); dot(x, 12 - h, 0xfff0c0, f);
    if ((now + x) % 3 < 1.5) dot(x, 10 - h - Math.floor(((now + x) % 1.5) * 3), 0x6a6460, 0.3);
    if (isNight) glowLate(() => { glowAt(x, 13, 6, 8, 0xffb050, 0.22); fill(x - 1, 14 - h, 2, h, 0xffa040); }); else glowAt(x, 13, 5, 5, 0xffb050, 0.06);
  },
  /** A dried pufferfish lamp hung from the thatch: a round spiky blowfish lit warm from within, swinging a little; it glows at night. */
  tikipuffer(x) {
    const body = 0xd8b060, bodyLight = 0xf0d088, belly = 0xe8dcc0, spike = 0x9a7a3a;
    const sway = Math.round(Math.sin(now * 1.1 + x * 0.1));
    fill(x - 1, 4, 2, 4, spike); // the cord
    const cx = x + sway;
    disc(cx, 15, 6, body); disc(cx, 17, 4, belly, 0.6); disc(cx - 1, 13, 2, bodyLight, 0.7);
    for (let a = 0; a < 2 * Math.PI; a += 0.5) dot(cx + Math.round(Math.cos(a) * 7), 15 + Math.round(Math.sin(a) * 7), spike); // spikes
    dot(cx - 2, 14, PALETTE.eye); dot(cx + 2, 14, PALETTE.eye); fill(cx - 1, 18, 3, 1, 0xc89050); // eyes + mouth
    if (isNight) glowLate(() => glowAt(cx, 15, 7, 7, 0xffcf70, 0.18));
  },
  /** A hand-painted beach-bar sign on two posts: a weathered plank with a palm tree and a cocktail glass daubed on it, swinging gently. */
  tikisign(x) {
    const wood = 0x8a6a42, woodLight = 0xa88a5a, plank = 0xcab488, green = 0x4f9a5a, trunk = 0x8a5a32, drink = 0xf2b040;
    fill(x - 10, 5, 2, 8, wood); fill(x + 8, 5, 2, 8, wood); fill(x - 10, 5, 20, 1, wood); // posts + top rail
    const sway = Math.round(Math.sin(now * 0.9) * 0.5);
    fill(x - 9, 13 + sway, 18, 10, plank); fill(x - 9, 13 + sway, 18, 1, 0xdcc89a); fill(x - 9, 22 + sway, 18, 1, 0x9a8460);
    // a palm tree
    fill(x - 5, 16 + sway, 1, 5, trunk); for (const [dx, dy] of [[-3, -1], [-2, -2], [-1, -1], [0, -2], [1, -1]]) dot(x - 5 + dx, 15 + sway + dy, green);
    // a cocktail glass
    fill(x + 2, 16 + sway, 5, 1, drink); fill(x + 3, 17 + sway, 3, 1, drink); dot(x + 4, 18 + sway, drink); fill(x + 4, 19 + sway, 1, 2, 0xd8d0b8); dot(x + 6, 15 + sway, 0xe8404a);
  },
  /** A luau display on the thatch: a hula grass skirt hung up with a flower lei draped over it and a couple of hibiscus blooms, the fringe stirring. */
  tikihula(x) {
    const skirt = 0x8aae4a, skirtDark = 0x6a8e36, band = 0x8a5a32, lei = 0xf06a9a, hib = 0xe8404a, yellow = 0xf2c040;
    fill(x - 10, 7, 20, 2, band); fill(x - 10, 7, 20, 1, 0xa86a3a); // the waistband
    const sway = Math.floor(now * 3) % 2;
    for (let k = 0; k < 18; k++) { const sx = x - 9 + k, len = 11 + (hash(sx, 3) % 4), off = (k % 3 === sway) ? 1 : 0; for (let d = 0; d < len; d++) dot(sx, 9 + d, d > len - 2 ? skirtDark : skirt); if (off) dot(sx, 9 + len, skirtDark); }
    // the lei draped across
    for (let k = 0; k < 11; k++) dot(x - 9 + k * 2, 10 + Math.round(Math.sin(k * 0.6) * 1.5), k % 2 ? lei : yellow);
    dot(x - 6, 12, hib); dot(x + 5, 13, hib); dot(x - 6, 12, 0xf2b0c0); // hibiscus blooms
  },
  /** A neon diner sign: a chrome-framed panel with a big pink neon diamond and a cyan arrow pointing in, chasing bulbs round the edge; it buzzes bright at night. */
  dinerneon(x) {
    const frame = 0x8a8e96, pink = 0xff5ab0, cyan = 0x4ae8f0, dark = 0x1a1a22;
    fill(x - 9, 6, 18, 18, dark); fill(x - 9, 6, 18, 1, 0xa8adb6); fill(x - 9, 6, 1, 18, frame); fill(x + 8, 6, 1, 18, 0x5a5e66);
    // the neon diamond
    for (let k = 0; k <= 5; k++) { dot(x, 9 + k, pink); dot(x, 21 - k, pink); dot(x - k, 15, pink); dot(x + k, 15, pink); }
    for (let k = 1; k <= 4; k++) { dot(x - k, 15 - (5 - k), pink); dot(x + k, 15 - (5 - k), pink); dot(x - k, 15 + (5 - k), pink); dot(x + k, 15 + (5 - k), pink); }
    // a cyan arrow through it
    fill(x - 7, 15, 14, 1, cyan); fill(x + 4, 13, 1, 5, cyan); dot(x + 5, 14, cyan); dot(x + 5, 16, cyan);
    // chasing bulbs round the edge
    const bulbs = []; for (let bx = -8; bx <= 8; bx += 2) { bulbs.push([bx, 6], [bx, 23]); } for (let by = 8; by <= 22; by += 2) { bulbs.push([-9, by], [8, by]); }
    const lit = (a) => bulbs.forEach(([bx, by], k) => dot(x + bx, by, (k + Math.floor(now * 5)) % 3 === 0 ? 0xfff4c0 : 0x8a7a40, a));
    lit(1);
    if (isNight) glowLate(() => { glowAt(x, 15, 9, 9, pink, 0.16); glowAt(x, 15, 9, 3, cyan, 0.1); lit(1); });
  },
  /** A 1950s atomic starburst clock: a sunburst of gold spokes with ball tips around a round face with the real time, in diner teal and gold. */
  dinerclock(x) {
    const gold = 0xd8b24e, goldLight = 0xf2d888, teal = 0x4ab8a8, face = 0xf0ece0;
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2, len = k % 3 === 0 ? 10 : 8; line(x, 15, x + Math.round(Math.cos(a) * len), 15 + Math.round(Math.sin(a) * len), k % 2 ? gold : teal); dot(x + Math.round(Math.cos(a) * (len + 1)), 15 + Math.round(Math.sin(a) * (len + 1)), k % 2 ? goldLight : 0x6ad8c8); }
    disc(x, 15, 5, face); ring(x, 15, 5, gold, 1);
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12) / 12) * Math.PI * 2 - Math.PI / 2;
    line(x, 15, x + Math.round(Math.cos(ha) * 2.6), 15 + Math.round(Math.sin(ha) * 2.6), 0x2a2a30);
    line(x, 15, x + Math.round(Math.cos(ma) * 3.8), 15 + Math.round(Math.sin(ma) * 3.8), 0x2a2a30); dot(x, 15, teal);
  },
  /** A diner menu board: a cream panel with a red header bar, little rows of 'menu' lines and prices, and a chrome frame; the daily special line blinks. */
  dinermenu(x) {
    const frame = 0xc8ccd2, cream = 0xf0ece0, red = 0xe03a4a, line = 0x8a8e96, teal = 0x4ab8a8;
    fill(x - 12, 5, 24, 20, frame); fill(x - 11, 6, 22, 18, cream); fill(x - 11, 6, 22, 1, 0xf8f4ea);
    fill(x - 11, 6, 22, 3, red); // the header bar
    for (let r = 0; r < 5; r++) { fill(x - 9, 11 + r * 3, 11, 1, line, 0.7); fill(x + 5, 11 + r * 3, 4, 1, teal, 0.8); } // menu rows + prices
    const blink = (now % 3) < 1.5; fill(x - 9, 23, 13, 1, blink ? red : line, 0.8); // the special
  },
  /** A refrigerated pie display case: a chrome-and-glass cabinet with a revolving stand of coloured pies on two shelves; the stand turns slowly. */
  dinerpie(x) {
    const chrome = 0xc8ccd2, chromeLight = 0xe8ecf0, glass = 0xcfe6ee, pie = [0xe8a030, 0xd8405a, 0x8a5a32, 0xf0e0a0];
    fill(x - 9, 6, 18, 18, chrome); fill(x - 8, 7, 16, 16, 0x2a3034); fill(x - 8, 7, 16, 16, glass, 0.25); fill(x - 9, 6, 18, 1, chromeLight);
    fill(x - 8, 14, 16, 1, chrome); // the middle shelf
    const turn = Math.floor(now * 0.8) % 4;
    for (let r = 0; r < 2; r++) for (let k = 0; k < 3; k++) { const px = x - 6 + k * 5, py = 10 + r * 7, c = pie[(k + r + turn) % 4]; fill(px, py, 4, 2, c); fill(px, py - 1, 4, 1, shaded(c, 1.2)); dot(px + 1, py, 0xf8f0d8); }
    dot(x - 1, 13, chromeLight);
  },
  /** A milkshake sign cut in the shape of a tall glass with a swirl of cream, a cherry and two straws, outlined in neon that glows at night. */
  dinershake(x) {
    const glass = 0xcfe6ee, shake = 0xf2d8e0, cream = 0xf8f0f0, cherry = 0xe03a4a, neon = 0xff5ab0;
    fill(x - 3, 10, 6, 11, glass, 0.5); fill(x - 3, 10, 1, 11, 0xe8f4f8); fill(x - 3, 20, 6, 1, 0x9ab0ba); // the glass
    fill(x - 2, 11, 4, 8, shake); fill(x - 2, 8, 4, 3, cream); dot(x - 1, 7, cream); dot(x + 1, 6, cream); // shake + swirl
    dot(x, 5, cherry); fill(x - 1, 6, 1, 4, 0xe8a030); fill(x + 1, 6, 1, 4, 0xe8a030); // cherry + straws
    ring(x, 13, 7, neon, 0.4);
    if (isNight) glowLate(() => { ring(x, 13, 7, neon, 0.8); glowAt(x, 13, 6, 8, neon, 0.14); });
  },
  /** A row of vintage enamel plates and road-trip signs hung on the wall: coloured discs and little shields in red, teal and cream with a star or two. */
  dinerplates(x) {
    const cols = [0xe03a4a, 0x4ab8a8, 0xf0ece0, 0xf2c040];
    for (let k = 0; k < 3; k++) { const px = x - 8 + k * 8, c = cols[k]; disc(px, 11, 3.4, c); ring(px, 11, 3.4, 0xf0ece0, 0.8); dot(px, 11, 0xf0ece0); }
    // a couple of little route shields below
    for (let k = 0; k < 2; k++) { const px = x - 4 + k * 8; fill(px - 2, 17, 5, 6, cols[(k + 1) % 4]); fill(px - 2, 17, 5, 1, 0xf0ece0); fill(px - 1, 22, 3, 1, 0xf0ece0); dot(px, 19, 0xf0ece0); }
    dot(x + 8, 20, 0xf2c040);
  },
  /** A big brass scales of justice mounted on the oak wall: a central column with a balanced beam and two shallow pans, tipping gently to level; the brass gleams. */
  courtscales(x) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a;
    const tip = Math.round(Math.sin(now * 1.1 + x * 0.1));
    fill(x, 8, 1, 16, brass); fill(x, 8, 1, 16, brassLight); fill(x - 3, 23, 7, 1, brass); fill(x - 2, 24, 5, 1, brassDark); // column + base
    disc(x, 8, 1.4, brassLight); // finial
    fill(x - 7, 11 + tip, 15, 1, brass); dot(x - 7, 11 + tip, brassLight); dot(x + 7, 11 - tip + tip, brassLight); // beam
    // two pans on chains
    for (const [px, pt] of [[x - 7, tip], [x + 7, -tip]]) { line(px, 11 + pt, px, 15 + pt, brassDark); fill(px - 2, 15 + pt, 5, 1, brass); fill(px - 1, 16 + pt, 3, 1, brassDark); }
    if ((now + x * 0.3) % 6 < 0.4) dot(x, 12, WHITE, 0.7);
  },
  /** A standing flag on a brass pole with a gold eagle finial: a plain deep-blue banner with a gold laurel wreath and star, furling slowly; no real nation. */
  courtflag(x) {
    const pole = 0xc8a050, cloth = 0x1e2a56, clothDark = 0x14204a, gold = 0xc8a050, goldLight = 0xe8c878;
    fill(x - 5, 4, 1, 24, pole); fill(x - 5, 4, 1, 24, 0xaa842a); dot(x - 5, 3, goldLight); fill(x - 6, 2, 3, 2, gold); // pole + finial
    const furl = Math.round(Math.sin(now * 0.8 + x) * 1);
    for (let r = 0; r < 14; r++) { const w = 11 + (r % 3 === 0 ? furl : 0); fill(x - 4, 6 + r, w, 1, (r + furl) % 4 === 0 ? clothDark : cloth); }
    ring(x + 1, 13, 3, gold, 0.8); for (const [dx, dy] of [[0, -2], [0, 2], [-2, 0], [2, 0], [0, 0]]) dot(x + 1 + dx, 13 + dy, goldLight); // laurel + star
  },
  /** A great seal on the wall: a round carved medallion in oak and gold with a laurel border, a central star and a ribbon banner below; the gold catches the light. */
  courtseal(x) {
    const oak = 0x6a4a2e, gold = 0xc8a050, goldLight = 0xe8c878, goldDark = 0x8a6a2a, cream = 0xe8dcc0;
    disc(x, 13, 9, oak); ring(x, 13, 9, gold, 1); ring(x, 13, 6.5, goldDark, 0.7);
    for (let k = 0; k < 16; k++) { const a = (k / 16) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 8), 13 + Math.round(Math.sin(a) * 8), k % 2 ? goldLight : gold); } // laurel beads
    disc(x, 13, 4, cream); for (const [dx, dy] of [[0, -3], [0, 3], [-3, 0], [3, 0], [-2, -2], [2, 2], [2, -2], [-2, 2]]) dot(x + dx, 13 + dy, gold); dot(x, 13, goldDark); // central star
    fill(x - 5, 22, 10, 2, 0x8a2a32); fill(x - 6, 23, 2, 2, 0x6a1e26); fill(x + 4, 23, 2, 2, 0x6a1e26); // ribbon banner
  },
  /** An institutional courthouse clock: a round black-rimmed face with bold numerals, brass surround and the real time on simple hands. */
  courtclock(x) {
    const brass = 0xc8a050, rim = 0x2a2a30, face = 0xf0ece0;
    disc(x, 14, 9, brass); disc(x, 14, 8, rim); disc(x, 14, 7, face); ring(x, 14, 7, 0x4a4a52, 0.6);
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 5.6), 14 + Math.round(Math.sin(a) * 5.6), k % 3 === 0 ? rim : 0x6a6a72); }
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2, ha = ((d.getHours() % 12) / 12) * Math.PI * 2 - Math.PI / 2;
    line(x, 14, x + Math.round(Math.cos(ha) * 3), 14 + Math.round(Math.sin(ha) * 3), rim);
    line(x, 14, x + Math.round(Math.cos(ma) * 4.6), 14 + Math.round(Math.sin(ma) * 4.6), rim); dot(x, 14, brass);
  },
  /** A law-library shelf: two rows of thick bound legal volumes in oxblood, green and tan with gold spine bands, a little bust on the top shelf. */
  courtbooks(x) {
    const shelf = 0x4a3220, cols = [0x7a2a2e, 0x2a5a3a, 0xa8824a, 0x3a3a5a, 0x6a2a4a];
    fill(x - 13, 5, 26, 20, shelf); fill(x - 13, 5, 26, 1, 0x6a4a2e); fill(x - 12, 14, 24, 1, 0x2a1c12); fill(x - 12, 24, 24, 1, 0x2a1c12);
    for (let row = 0; row < 2; row++) for (let k = 0; k < 11; k++) { const bx = x - 11 + k * 2, by = 7 + row * 10, c = cols[hash(bx, row) % 5], h = 6 + hash(bx, by) % 2; fill(bx, by + (7 - h), 2, h, c); fill(bx, by + (7 - h), 1, h, shaded(c, 1.3)); if (hash(bx, k) % 2) fill(bx, by + (7 - h) + 1, 2, 1, 0xc8a050, 0.7); }
    fill(x + 7, 6, 4, 1, 0xd8d0c0); fill(x + 8, 3, 2, 3, 0xc8c0b0); dot(x + 8, 3, 0xe0d8c8); // a little bust on top
  },
  /** A judge's portrait in a heavy gilt frame: a dignified figure in a dark robe and white collar against a muted background; the frame gleams gold. */
  courtportrait(x) {
    const frame = 0xc8a050, frameLight = 0xe8c878, frameDark = 0x8a6a2a, bg = 0x4a3e4a, robe = 0x24242c, skin = 0xd79a64, hair = 0x9a9088, collar = 0xe8e4da;
    fill(x - 9, 4, 18, 21, frame); fill(x - 9, 4, 18, 1, frameLight); fill(x + 8, 4, 1, 21, frameDark); fill(x - 8, 5, 16, 19, bg);
    fill(x - 4, 14, 8, 10, robe); fill(x - 4, 14, 8, 1, 0x34343e); // robe/shoulders
    fill(x - 2, 9, 4, 5, skin); fill(x - 3, 8, 6, 2, hair); fill(x - 3, 10, 1, 3, hair); fill(x + 2, 10, 1, 3, hair); // head + grey hair
    fill(x - 1, 14, 2, 3, collar); dot(x - 1, 11, PALETTE.eye); dot(x + 1, 11, PALETTE.eye);
    fill(x - 9, 24, 18, 1, frameDark);
  },
  /** A framed abstract painting in a gilt frame: bold blocks and a swoop of warm colour over a cream ground, a highlight gleaming on the glass. */
  artcanvas(x) {
    const frame = 0xc8a050, frameLight = 0xe8c878, frameDark = 0x8a6a2a, ground = 0xeae2cc;
    fill(x - 11, 5, 22, 20, frame); fill(x - 11, 5, 22, 1, frameLight); fill(x + 10, 5, 1, 20, frameDark); fill(x - 11, 24, 22, 1, frameDark);
    fill(x - 9, 7, 18, 16, ground);
    fill(x - 8, 9, 7, 8, 0x4a8ad8, 0.85); fill(x + 1, 12, 7, 6, 0xe8404a, 0.85); fill(x - 4, 16, 10, 4, 0xf2c040, 0.8);
    for (let k = -7; k < 8; k++) dot(x + k, 13 + Math.round(Math.sin(k * 0.4) * 3), 0x2a2a30, 0.7); // a swooping brush line
    dot(x - 7, 8, WHITE, 0.6);
  },
  /** A giant wooden artist's palette hung on the wall, thumb-hole and all, loaded with big bright blobs of paint and a couple of brushes laid across it. */
  artpalette(x) {
    const wood = 0xc8a060, woodLight = 0xe0bc78, woodDark = 0x9a7840;
    disc(x, 15, 9, wood); disc(x - 1, 14, 8, woodLight, 0); fill(x - 9, 15, 18, 1, woodDark, 0.4);
    disc(x + 4, 20, 2, wood); disc(x + 4, 20, 1.4, 0xd8d0c0); // the thumb hole (shown as a light cut)
    const blobs = [[-5, 11, 0xe8404a], [-1, 10, 0x4a8ad8], [3, 11, 0xf2c040], [-6, 16, 0x4ac86a], [-2, 17, 0xff5ab0], [2, 16, 0xf0ece0], [-4, 20, 0xe8822a]];
    for (const [dx, by, c] of blobs) { disc(x + dx, by, 1.6, c); dot(x + dx - 1, by - 1, shaded(c, 1.3)); }
    line(x - 6, 8, x + 7, 22, 0x8a6a3a); line(x - 3, 7, x + 8, 20, 0x8a6a3a); dot(x - 6, 8, 0xe8404a); dot(x - 3, 7, 0x4a8ad8); // two brushes
  },
  /** A cluster of pinned sketches and studies on the wall: little papers at slight angles with quick line drawings, held by pins and bits of tape. */
  artsketch(x) {
    const paper = 0xf0ece0, paperAlt = 0xe8e0cc, lin = 0x6a5a4a, pin = 0xe8404a;
    const sheets = [[-10, 6, 9, 8, 0], [0, 5, 10, 9, 0], [-7, 15, 8, 8, 1], [3, 15, 9, 8, 0]];
    for (const [dx, dy, w, h, alt] of sheets) {
      fill(x + dx, dy, w, h, alt ? paperAlt : paper); fill(x + dx, dy, w, 1, 0xf8f4ea); fill(x + dx, dy + h - 1, w, 1, 0xd0c8b4);
      for (let s = 0; s < 3; s++) { const ly = dy + 2 + s * 2; line(x + dx + 2, ly, x + dx + w - 2, ly + (s % 2 ? -1 : 1), lin, 0.7); }
      dot(x + dx + Math.floor(w / 2), dy, pin);
    }
  },
  /** A pitched glass skylight set into the wall, panes crossed by white glazing bars showing sky — blue by day, deep blue with a star or two by night; soft light spills from it. */
  artskylight(x) {
    const bar = 0xd8d0c0, barDark = 0xaaa290, day = 0x8ec0e0, night = 0x1a2340;
    fill(x - 13, 5, 26, 19, barDark); fill(x - 12, 6, 24, 17, isNight ? night : day);
    for (let gx = x - 8; gx < x + 9; gx += 5) fill(gx, 6, 1, 17, bar); for (let gy = 10; gy < 23; gy += 5) fill(x - 12, gy, 24, 1, bar);
    fill(x - 13, 5, 26, 1, 0xe0d8c8); fill(x - 1, 5, 2, 19, bar); // ridge bar
    if (isNight) { for (let k = 0; k < 5; k++) dot(x - 9 + hash(k, 3) % 18, 8 + hash(k, 7) % 12, hash(k, 1) % 3 ? 0xf6f6f2 : 0xf2d888, 0.9); }
    else { fill(x + 3, 7, 5, 3, 0xf0f4f8, 0.5); dot(x - 4, 9, WHITE, 0.6); }
    if (isNight) glowLate(() => glowAt(x, 14, 13, 9, 0xbfd0e8, 0.08)); else glowAt(x, 14, 12, 8, 0xeaf2f8, 0.06);
  },
  /** A studio supply shelf: jars of brushes, stacked paint tubes, a little plaster bust and rolled canvases leaning, all a cheerful clutter. */
  artshelf(x) {
    const shelf = 0x9a7a4a, shelfLight = 0xb89460, jar = 0xcfe6ee, plaster = 0xe8e4da;
    fill(x - 12, 15, 24, 2, shelf); fill(x - 12, 15, 24, 1, shelfLight); fill(x - 12, 23, 24, 2, shelf); fill(x - 12, 23, 24, 1, shelfLight);
    // brush jars
    for (const jx of [-10, -6]) { fill(x + jx, 9, 4, 6, jar, 0.6); fill(x + jx, 9, 4, 1, 0xe8f4f8); for (let k = 0; k < 3; k++) { fill(x + jx + 1 + k, 5, 1, 5, 0x8a6a3a); dot(x + jx + 1 + k, 5, [0xe8404a, 0x4a8ad8, 0xf2c040][k]); } }
    // stacked paint tubes
    for (let k = 0; k < 4; k++) { fill(x - 1 + k * 2, 11, 1, 4, [0xe8404a, 0x4ac86a, 0x4a8ad8, 0xff5ab0][k]); dot(x - 1 + k * 2, 11, 0xd8d0c0); }
    // a plaster bust on the lower shelf
    fill(x + 6, 18, 4, 5, plaster); fill(x + 7, 16, 2, 3, plaster); fill(x + 6, 23, 4, 1, 0xc8c0b0); dot(x + 7, 17, 0xf0ece0);
    fill(x - 10, 17, 2, 6, 0xe0d4b8); fill(x - 10, 17, 1, 6, 0xf0e8d0); // a rolled canvas leaning
  },
  /** A framed modern-art exhibition poster: a bold minimal composition of a circle and two bars in primary colours on cream, with a block of fine 'text' lines below. */
  artposter(x) {
    const frame = 0x2a2a30, cream = 0xeee6d4, red = 0xe8404a, blue = 0x4a8ad8, yellow = 0xf2c040;
    fill(x - 9, 5, 18, 20, frame); fill(x - 8, 6, 16, 18, cream);
    disc(x - 2, 11, 3, red); fill(x + 2, 9, 4, 8, blue); fill(x - 6, 15, 10, 2, yellow);
    for (let r = 0; r < 3; r++) fill(x - 6, 19 + r, 12 - r * 2, 1, 0x6a6a72, 0.7); // text lines
  },
  /** A big reef-tank window set in the wall: lit blue water with coral, waving weed and a few bright fish drifting across; it glows at night. */
  aquariumreef(x) {
    const frame = 0x3a4048, water = 0x14466e, waterLight = 0x1e5a86, sand = 0xd8c48a, weed = 0x3a9a5a, coral = 0xf08a9a;
    fill(x - 13, 5, 26, 20, frame); fill(x - 12, 6, 24, 18, water); fill(x - 12, 6, 24, 1, waterLight); fill(x - 12, 22, 24, 2, sand);
    for (const wx of [x - 9, x - 7, x + 8]) for (let k = 0; k < 6; k++) dot(wx + Math.round(Math.sin(now * 1.4 + wx + k) * 1), 22 - k, weed);
    fill(x + 2, 19, 4, 4, coral); dot(x + 1, 18, 0xf6a0b0); dot(x + 6, 19, 0x9a6ad8);
    const fishes = [[0, 0xf0824a, 13], [1, 0xf2c040, 10], [2, 0x5ac8e8, 16]];
    for (const [i, c, fy] of fishes) { const fx = x - 10 + Math.round((Math.sin(now * 0.7 + i * 2.1) * 0.5 + 0.5) * 20); fill(fx, fy, 2, 1, c); dot(fx - 1, fy, c); dot(fx + 2, fy, c); }
    fill(x - 13, 5, 26, 1, 0x55606a);
    if (isNight) glowLate(() => glowAt(x, 15, 13, 10, 0x2a7ac0, 0.16));
  },
  /** A tall jellyfish tank glowing violet: pale moon-jellies pulsing and drifting up and down in a dark column; it glows softly at night. */
  aquariumjelly(x) {
    const frame = 0x2a2e3a, water = 0x1a1440, glow = 0x9a6ad8;
    fill(x - 9, 4, 18, 22, frame); fill(x - 8, 5, 16, 20, water);
    for (let k = 0; k < 4; k++) {
      const t = (now * 0.4 + k * 1.3) % 4, jy = 23 - t * 4.5, jx = x - 5 + k * 3 + Math.round(Math.sin(now + k) * 1), pulse = Math.floor(now * 3 + k) % 2;
      fill(jx - 1, jy, 3 + pulse, 2, 0xe8d0f0, 0.8); fill(jx - 1, jy + 2, 1, 2, glow, 0.6); fill(jx + 1, jy + 2, 1, 2, glow, 0.6); dot(jx, jy, WHITE, 0.7);
    }
    fill(x - 9, 4, 18, 1, 0x3e4250);
    if (isNight) glowLate(() => glowAt(x, 14, 9, 11, glow, 0.2)); else glowAt(x, 14, 8, 10, glow, 0.08);
  },
  /** A big shark-tank window: deep dark-blue water with a sweep of pale light and the silhouette of a shark gliding slowly across, fish scattering; it glows at night. */
  aquariumshark(x) {
    const frame = 0x3a4048, water = 0x0e3356, waterLight = 0x164066, shark = 0x3a5068;
    fill(x - 13, 5, 26, 20, frame); fill(x - 12, 6, 24, 18, water); fill(x - 12, 6, 24, 2, waterLight);
    for (let k = 0; k < 4; k++) fill(x - 11 + k * 2, 7, 1, 16, 0x1a4870, 0.25); // shafts of light
    // the gliding shark
    const sx = x - 14 + ((now * 10) % 30), dir = 1; const bx = Math.round(sx);
    fill(bx, 14, 8, 3, shark); fill(bx + 7, 14, 3, 2, shark); fill(bx + 1, 12, 2, 1, shark); dot(bx + 10, 15, shark); // body, tail hint, dorsal
    fill(bx - 1, 15, 1, 1, shark); dot(bx, 14, 0x2a3e52);
    // a couple of little fish scattering
    for (let k = 0; k < 3; k++) { const fx = x + 2 + k * 3, fy = 10 + k * 3; dot(fx, fy, 0x5ac8e8); dot(fx + 1, fy, 0x5ac8e8, 0.6); }
    fill(x - 13, 5, 26, 1, 0x55606a);
    if (isNight) glowLate(() => glowAt(x, 15, 13, 10, 0x1a6ab0, 0.14));
  },
  /** A slim seahorse tank: warm green-lit water with two seahorses curled on a strand of weed, bobbing gently; it glows at night. */
  aquariumseahorse(x) {
    const frame = 0x3a4048, water = 0x13504a, weed = 0x3a9a5a, horse = 0xf2b84a, horse2 = 0xf08a9a;
    fill(x - 6, 4, 12, 22, frame); fill(x - 5, 5, 10, 20, water); fill(x - 5, 5, 10, 1, 0x1e6a60);
    fill(x, 10, 1, 14, weed); for (let k = 0; k < 6; k++) dot(x + Math.round(Math.sin(now + k) * 1), 23 - k * 2, weed);
    const bob = Math.round(Math.sin(now * 1.2));
    // two seahorses
    for (const [hx, hy, c] of [[x - 3, 12 + bob, horse], [x + 2, 16 - bob, horse2]]) { fill(hx, hy, 1, 4, c); dot(hx + 1, hy, c); dot(hx + 1, hy - 1, c); dot(hx, hy + 4, c); dot(hx - 1, hy + 3, c); dot(hx + 2, hy + 1, PALETTE.eye); }
    fill(x - 6, 4, 12, 1, 0x3e4250);
    if (isNight) glowLate(() => glowAt(x, 14, 6, 11, 0x2a9a7a, 0.14));
  },
  /** A starfish display: three starfish and a scallop shell mounted on a sand-coloured board in a driftwood frame, in orange, pink and purple. */
  aquariumstar(x) {
    const frame = 0x8a6a4a, board = 0xd8c48a, stars = [0xf0824a, 0xf06a9a, 0x9a6ad8];
    fill(x - 8, 5, 16, 20, frame); fill(x - 7, 6, 14, 18, board); fill(x - 7, 6, 14, 1, 0xe8d8b0);
    const spots = [[-3, 11], [4, 10], [0, 18]];
    spots.forEach(([dx, dy], i) => { const c = stars[i], cx = x + dx, cy = dy; for (const a of [0, 1.26, 2.51, 3.77, 5.03]) { const ex = cx + Math.round(Math.cos(a - 1.57) * 3), ey = cy + Math.round(Math.sin(a - 1.57) * 3); line(cx, cy, ex, ey, c); dot(ex, ey, shaded(c, 1.2)); } dot(cx, cy, shaded(c, 0.8)); });
    // a scallop shell
    fill(x + 3, 19, 4, 3, 0xe8c0a0); for (let k = 0; k < 3; k++) fill(x + 3 + k, 19, 1, 3 - k % 2, 0xd8a888); dot(x + 5, 22, 0xc89880);
  },
  /** A bubble column: a tall clear tube of water full of rising bubbles lit from below, glowing cyan; the bubbles stream up continuously and it glows at night. */
  aquariumtube(x) {
    const tube = 0xbfe4f0, water = 0x1a5a7a, cap = 0x8a8e96;
    fill(x - 3, 4, 6, 22, water, 0.85); fill(x - 3, 4, 1, 22, 0x2a7a9a); fill(x + 2, 4, 1, 22, 0x0e3a52);
    fill(x - 4, 3, 8, 2, cap); fill(x - 4, 24, 8, 2, cap); fill(x - 4, 24, 8, 1, 0x6a6e76);
    for (let k = 0; k < 10; k++) { const t = (now * 1.4 + k * 0.7) % 3.2, by = 24 - t * 6, bx = x + Math.round(Math.sin(t * 3 + k) * 1.4); dot(bx, Math.round(by), tube, 0.9); if (k % 3 === 0) ring(bx, Math.round(by), 1.2, tube, 0.7); }
    if (isNight) glowLate(() => glowAt(x, 14, 4, 11, 0x4ac8e8, 0.2)); else glowAt(x, 14, 3, 10, 0x4ac8e8, 0.08);
  },
  /** A vintage show poster in a gilt frame: a red banner, a golden starburst with a little performer in silhouette, and a line of showbill type. */
  circusposter(x) {
    const frame = 0xb88a2a, cream = 0xf2e8d8, red = 0xd8402a, gold = 0xe8b84a, navy = 0x2a3a6a;
    fill(x - 8, 7, 16, 22, frame); fill(x - 7, 8, 14, 20, cream);
    fill(x - 7, 8, 14, 4, red); fill(x - 7, 8, 14, 1, 0xf05a40);
    for (let k = 0; k < 8; k++) { const a = (k / 8) * Math.PI; line(x, 19, x + Math.round(Math.cos(a) * 6), 19 - Math.round(Math.sin(a) * 6), gold, 0.5); }
    disc(x, 19, 3, gold); disc(x, 19, 2, 0xf2d888);
    fill(x - 1, 18, 2, 5, navy); dot(x, 16, navy); fill(x - 2, 20, 5, 1, navy);
    fill(x - 5, 25, 11, 1, 0x5a4a3a); fill(x - 3, 27, 7, 1, 0x5a4a3a);
  },
  /** A line of little triangular pennants in clashing colours strung across the wall, each one swaying a touch out of time with its neighbours. */
  circusbunting(x) {
    const cols = [0xd8402a, 0xe8b84a, 0x4c8bf5, 0x5aa84a, 0xf05a40, 0x3fa0d8];
    for (let k = -11; k <= 11; k++) dot(x + k, 6 - Math.round(Math.cos(k / 11 * 1.4) * 2) + 2, 0x6a5a4a);
    for (let i = 0; i < 8; i++) {
      const cx = x - 10 + i * 3, top = 8 - Math.round(Math.cos((cx - x) / 11 * 1.4) * 2), sway = Math.round(Math.sin(now * 1.3 + i * 0.7));
      const c = cols[i % cols.length];
      fill(cx - 1, top, 3, 1, c); fill(cx + (sway > 0 ? 1 : 0), top + 1, sway ? 1 : 2, 1, c); dot(cx + sway, top + 2, c);
    }
  },
  /** A light-up marquee board: a gold star blazing in the middle, framed by bulbs that chase round the border; it glows at night. */
  circussign(x) {
    const board = 0xaa2a1a, boardL = 0xd8402a, gold = 0xe8b84a, bulbOn = 0xffe08a, bulbOff = 0xb88a2a;
    fill(x - 9, 9, 18, 14, board); fill(x - 9, 9, 18, 1, boardL); fill(x - 9, 22, 18, 1, 0x7a1e12);
    fill(x - 1, 13, 2, 7, gold); fill(x - 3, 15, 7, 2, gold); dot(x - 2, 14, gold); dot(x + 1, 14, gold); dot(x - 2, 17, gold); dot(x + 1, 17, gold);
    const ph = Math.floor(now * 4); let b = 0;
    for (let k = -8; k <= 8; k += 2) { dot(x + k, 9, (b + ph) % 3 === 0 ? bulbOn : bulbOff); dot(x + k, 22, (b + ph + 1) % 3 === 0 ? bulbOn : bulbOff); b++; }
    for (let yy = 11; yy <= 21; yy += 2) { dot(x - 9, yy, (b + ph) % 3 === 0 ? bulbOn : bulbOff); dot(x + 9, yy, (b + ph + 1) % 3 === 0 ? bulbOn : bulbOff); b++; }
    if (isNight) glowLate(() => glowAt(x, 16, 11, 9, 0xffe08a, 0.18));
  },
  /** A trapeze hung from the rafters, swinging gently to and fro with a tiny acrobat dangling from the bar by both hands. */
  circustrapeze(x) {
    const bar = 0xe8b84a, rope = 0xcfc4a8, suit = 0xd8402a, skin = 0xf0c89a;
    const ax = x + Math.round(Math.sin(now * 1.3) * 5), ay = 23;
    fill(x - 5, 2, 11, 1, 0x6a5a4a);
    line(x - 4, 3, ax - 3, ay - 1, rope); line(x + 4, 3, ax + 3, ay - 1, rope);
    fill(ax - 4, ay, 8, 1, bar); dot(ax - 4, ay, 0xf2d888);
    fill(ax - 1, ay + 1, 3, 3, suit); dot(ax, ay, skin); fill(ax - 1, ay + 4, 3, 1, suit); dot(ax - 2, ay + 1, skin); dot(ax + 2, ay + 1, skin);
  },
  /** A big wheel of fortune: six bright wedges turning slowly, a ring of bulbs blinking round the rim and a little pointer ticking at the top. It glows at night. */
  circuswheel(x) {
    const cols = [0xd8402a, 0xe8b84a, 0x4c8bf5, 0x5aa84a, 0xf2e8d8, 0xf28fb0];
    const cy = 16, r = 8, a0 = now * 0.6;
    disc(x, cy, r + 1, 0xf2e8d8); disc(x, cy, r, 0xcfc4a8);
    for (let sgt = 0; sgt < 6; sgt++) { const a = a0 + sgt / 6 * Math.PI * 2; for (let rr = 1; rr <= r - 1; rr++) fill(x + Math.round(Math.cos(a) * rr), cy + Math.round(Math.sin(a) * rr), 2, 2, cols[sgt]); }
    ring(x, cy, r, 0xb88a2a); disc(x, cy, 1.6, 0xe8b84a);
    for (let sgt = 0; sgt < 8; sgt++) { const a = sgt / 8 * Math.PI * 2; dot(x + Math.round(Math.cos(a) * (r + 1)), cy + Math.round(Math.sin(a) * (r + 1)), (Math.floor(now * 3) + sgt) % 2 ? 0xffe08a : 0xb88a2a); }
    fill(x, cy - r - 2, 1, 2, 0x2a2a2e); dot(x, cy - r, 0x2a2a2e);
    if (isNight) glowLate(() => glowAt(x, cy, r + 2, r + 2, 0xffd060, 0.12));
  },
  /** A funhouse mirror in an ornate gold oval frame; its glass warps and ripples, a reflection streak sliding slowly across it. */
  circusmirror(x) {
    const frame = 0xe8b84a, frameD = 0xb88a2a, glass = 0xbfe0e8;
    ellipseRing(x, 16, 8, 11, frame); ellipseRing(x, 16, 7, 10, frameD, 0.5);
    for (let yy = 6; yy <= 26; yy++) { const w = Math.round(6 - Math.abs(yy - 16) * 0.3); if (w <= 0) continue; const wob = Math.round(Math.sin(yy * 0.9 + now) * 1); fill(x - w + wob, yy, w * 2, 1, glass, 0.5); }
    for (let yy = 8; yy <= 24; yy++) dot(x - 2 + Math.round(Math.sin(yy * 0.7 + now * 0.8) * 2), yy, WHITE, 0.35);
    dot(x + 3, 10, WHITE, 0.7);
  },
  /** A fossil T-rex skull mounted on a dark plaque with a brass nameplate: a long toothed snout and a deep eye socket. */
  museumtrex(x) {
    const bone = 0xe8dcc0, boneD = 0xcfc0a0, boneS = 0xb4a583, plaque = 0x5a3b25, brass = 0xc8a44e;
    fill(x - 10, 10, 20, 16, plaque); fill(x - 10, 10, 20, 1, 0x6f4a30); fill(x - 10, 25, 20, 1, 0x3e2819);
    fill(x - 9, 24, 18, 1, brass);
    fill(x - 7, 14, 10, 6, bone); fill(x + 2, 15, 6, 4, bone); fill(x + 7, 16, 3, 2, bone);
    fill(x - 7, 14, 10, 1, 0xf2e8cc); fill(x - 7, 19, 10, 1, boneD);
    dot(x - 4, 16, 0x2a2018); dot(x - 3, 16, 0x2a2018); dot(x - 5, 16, boneS);
    fill(x - 6, 20, 14, 1, boneD); for (let k = x - 5; k < x + 8; k += 2) dot(k, 21, bone);
  },
  /** A lit diorama window: a little savanna at golden hour, an acacia and an elephant in silhouette; it glows warm at night. */
  museumdiorama(x) {
    const frame = 0x5a3b25, frameL = 0x7a5436, sky = 0xe8b86a, skyT = 0xf2d59a, ground = 0x8a7a3e, tree = 0x3a5a2e, animal = 0x4a3a2a;
    fill(x - 11, 8, 22, 18, frame); fill(x - 11, 8, 22, 1, frameL);
    fill(x - 9, 10, 18, 10, sky); fill(x - 9, 10, 18, 2, skyT); disc(x + 5, 13, 2, 0xf2e2a8);
    fill(x - 9, 18, 18, 6, ground); fill(x - 9, 18, 18, 1, 0x9a8a4a);
    fill(x - 6, 14, 1, 5, tree); fill(x - 8, 13, 5, 2, tree);
    fill(x + 2, 15, 2, 4, animal); fill(x + 1, 14, 4, 2, animal); fill(x + 4, 16, 2, 3, animal);
    if (isNight) glowLate(() => glowAt(x, 15, 11, 8, 0xf2d59a, 0.14));
  },
  /** A glazed insect case: rows of pinned butterflies in bright colours on a cream mat, a glint on the glass. */
  museumbutterflies(x) {
    const frame = 0x3a2e1e, mat = 0xe8e2d4;
    fill(x - 9, 8, 18, 18, frame); fill(x - 8, 9, 16, 16, mat);
    const cols = [0x4c6ad0, 0xe08a2a, 0x8a4ad0, 0xd04a6a, 0x3aa06a, 0xd0a83a];
    for (let i = 0; i < 6; i++) { const bx = x - 6 + (i % 3) * 5, by = 12 + Math.floor(i / 3) * 7, c = cols[i]; fill(bx, by, 1, 3, 0x2a2018); dot(bx - 1, by, c); dot(bx + 1, by, c); dot(bx - 1, by + 1, shaded(c, 0.85)); dot(bx + 1, by + 1, shaded(c, 0.85)); }
    dot(x + 7, 10, WHITE, 0.4);
  },
  /** A lit mineral cabinet: two shelves of amethyst, emerald and amber crystals on dark glass; they glow at night. */
  museumgems(x) {
    const box = 0x2a2e38, boxL = 0x3e4452;
    fill(x - 8, 9, 16, 17, box); fill(x - 8, 9, 16, 1, boxL); fill(x - 8, 25, 16, 1, 0x1a1c22);
    const gems = [[x - 5, 14, 0x9a6ad8], [x, 14, 0x4ad0a0], [x + 4, 15, 0xe0922a], [x - 3, 20, 0x4ad0a0], [x + 3, 20, 0x9a6ad8]];
    for (const [gx, gy, c] of gems) { fill(gx, gy - 2, 2, 3, c); dot(gx, gy - 3, shaded(c, 1.3)); dot(gx + 1, gy, shaded(c, 0.8)); dot(gx, gy - 2, WHITE, 0.4); }
    fill(x - 8, 17, 16, 1, 0x1a1c22);
    if (isNight) glowLate(() => { for (const [gx, gy, c] of gems) glowAt(gx, gy - 1, 3, 3, c, 0.2); });
  },
  /** An oil portrait of a bewhiskered museum founder in a gilt frame; he blinks now and then. */
  museumportrait(x) {
    const frame = 0xc8a44e, frameL = 0xe8c878, bg = 0x3a2e2a, coat = 0x2a2420, skin = 0xd8a888, hair = 0xe8e2d4;
    fill(x - 8, 8, 16, 19, frame); fill(x - 8, 8, 16, 1, frameL); fill(x - 7, 9, 14, 17, bg);
    fill(x - 4, 18, 9, 7, coat); fill(x - 2, 13, 5, 6, skin); fill(x - 2, 12, 5, 2, hair);
    fill(x - 3, 15, 1, 2, hair); fill(x + 3, 15, 1, 2, hair); fill(x - 1, 17, 3, 1, hair);
    const blink = (now + x) % 5 < 0.2; dot(x - 1, 15, blink ? skin : PALETTE.eye); dot(x + 1, 15, blink ? skin : PALETTE.eye);
  },
  /** An antique world map on parchment, blobby continents on a pale sea, a dotted route and a compass rose turning slowly. */
  museummap(x) {
    const parch = 0xe0cfa2, parchD = 0xc8b482, land = 0x8a9a5a, sea = 0xaec6c0, ink = 0x6a4a2a;
    fill(x - 12, 9, 24, 16, parch); fill(x - 12, 9, 24, 1, 0xeedcb0); fill(x - 12, 24, 24, 1, parchD); fill(x - 11, 11, 22, 12, sea);
    for (const [cx, cy, w, h] of [[-7, 13, 5, 4], [0, 15, 6, 5], [6, 12, 4, 3]]) fill(x + cx, cy, w, h, land);
    for (let k = -8; k < 8; k += 2) dot(x + k, 18 + Math.round(Math.sin(k) * 1), ink);
    const a = now * 0.3; line(x + 8, 21, x + 8 + Math.round(Math.cos(a) * 2), 21 + Math.round(Math.sin(a) * 2), 0x8a2a2e); line(x + 8, 21, x + 8 - Math.round(Math.cos(a) * 2), 21 - Math.round(Math.sin(a) * 2), ink); dot(x + 8, 21, 0x8a2a2e);
  },
  /** A lush holly wreath with berries, a red bow and tiny lights that twinkle; it glows warm at night. */
  northpolewreath(x) {
    const green = 0x2e6b3e, greenD = 0x1f4e2c, greenL = 0x3f8a52, berry = 0xd83030, bow = 0xc62828, gold = 0xe8c24a, cx = x, cy = 17;
    ring(cx, cy, 8, green); ring(cx, cy, 7, greenD); ring(cx, cy, 6, greenL, 0.7);
    for (let a = 0; a < 16; a++) { const an = a / 16 * Math.PI * 2, r = 7 + (a % 2); dot(cx + Math.round(Math.cos(an) * r), cy + Math.round(Math.sin(an) * r), a % 2 ? green : greenD); }
    for (let a = 0; a < 6; a++) { const an = a / 6 * Math.PI * 2, lx = cx + Math.round(Math.cos(an) * 7), ly = cy + Math.round(Math.sin(an) * 7); dot(lx, ly, (Math.floor(now * 2) + a) % 3 ? berry : gold); }
    fill(cx - 2, cy + 7, 5, 2, bow); fill(cx - 1, cy + 6, 3, 1, 0xe34b3c); fill(cx - 1, cy + 9, 1, 2, bow); fill(cx + 1, cy + 9, 1, 2, bow);
    if (isNight) glowLate(() => glowAt(cx, cy, 9, 9, 0xffd36b, 0.12));
  },
  /** A mantel beam with four hung stockings in red, green and white, name-tags on their cuffs, swaying a little. */
  northpolestockings(x) {
    const beam = 0x6a4428, beamL = 0x8a5a32, cols = [0xc62828, 0x2e6b3e, 0xf2f6fb, 0xd83030], trim = 0xf2f6fb;
    fill(x - 14, 11, 28, 2, beam); fill(x - 14, 11, 28, 1, beamL);
    for (let i = 0; i < 4; i++) { const sx = x - 11 + i * 7, sw = Math.round(Math.sin(now * 1.2 + i) * 1), c = cols[i]; fill(sx + sw, 13, 4, 7, c); fill(sx + sw, 13, 4, 2, trim); fill(sx + sw, 20, 5, 2, c); dot(sx + 1 + sw, 14, 0x2a2a2e); }
  },
  /** A frosted cabin window: a little snowy pine outside, snow falling, frost in the corners; warm from inside at night. */
  northpolewindow(x) {
    const frame = 0x6a4428, frameL = 0x8a5a32, sky = 0x6a86a8, skyN = 0x24304a, snow = 0xeef4fb, hill = 0xdfe8f0, night = isNight;
    fill(x - 15, 6, 30, 22, frame); fill(x - 15, 6, 30, 1, frameL); fill(x - 13, 8, 28, 18, night ? skyN : sky);
    fill(x - 13, 20, 28, 6, night ? 0x4a5a70 : hill);
    fill(x - 4, 16, 1, 5, night ? 0x2e4030 : 0x2e6b3e); fill(x - 6, 15, 5, 2, night ? 0x2e4030 : 0x2e6b3e); fill(x - 7, 17, 7, 2, night ? 0x2e4030 : 0x2e6b3e);
    for (let k = 0; k < 10; k++) dot(x - 12 + ((k * 37 + Math.floor(now * 6)) % 26), 9 + ((k * 23 + Math.floor(now * 8)) % 16), snow, 0.8);
    fill(x - 1, 8, 1, 18, frame); fill(x - 13, 16, 28, 1, frame);
    for (const [fx, fy] of [[x - 12, 9], [x + 12, 9], [x - 12, 24], [x + 12, 24]]) { dot(fx, fy, WHITE, 0.6); dot(fx + 1, fy, WHITE, 0.3); dot(fx, fy + 1, WHITE, 0.3); }
    if (night) glowLate(() => glowAt(x, 16, 15, 10, 0xffd36b, 0.08));
  },
  /** The Nice List on a rolled scroll: a gold header and rows of names ticked green, one flipping to a red cross now and then. */
  northpolelist(x) {
    const paper = 0xf2ead8, paperD = 0xd8ccb4, ink = 0x3a2e2a, green = 0x2e8b3e, red = 0xc62828, gold = 0xe8c24a;
    fill(x - 7, 8, 14, 18, paper); fill(x - 7, 8, 14, 1, 0xfbf4e4); fill(x - 7, 25, 14, 1, paperD);
    fill(x - 7, 7, 14, 2, paperD); fill(x - 7, 25, 14, 2, paperD); fill(x - 5, 10, 10, 1, gold);
    for (let i = 0; i < 6; i++) { const ly = 13 + i * 2; fill(x - 5, ly, 7, 1, ink, 0.6); const naughty = (Math.floor(now / 3) + i) % 6 === 0; dot(x + 4, ly, naughty ? red : green); dot(x + 5, ly, naughty ? red : green); }
  },
  /** A swag of big coloured bulbs blinking in sequence across the wall; they glow at night. */
  northpolelights(x) {
    const wire = 0x2e4030, cols = [0xd83030, 0xe8c24a, 0x2e8b3e, 0x4c8bf5, 0xf2f6fb];
    for (let k = -16; k <= 16; k++) dot(x + k, 7 + Math.round(Math.sin((k + 16) / 32 * Math.PI) * 3), wire);
    for (let i = 0; i < 9; i++) { const bx = x - 16 + i * 4, by = 8 + Math.round(Math.sin((bx - x + 16) / 32 * Math.PI) * 3), c = cols[i % cols.length], on = (Math.floor(now * 3) + i) % 3 !== 0; fill(bx, by, 2, 3, on ? c : shaded(c, 0.6)); dot(bx, by + 3, shaded(c, 1.3)); if (on && isNight) dot(bx, by + 1, WHITE, 0.5); }
    if (isNight) glowLate(() => { for (let i = 0; i < 9; i++) { const bx = x - 16 + i * 4, by = 10 + Math.round(Math.sin((bx - x + 16) / 32 * Math.PI) * 3); if ((Math.floor(now * 3) + i) % 3 !== 0) glowAt(bx, by, 3, 3, cols[i % cols.length], 0.18); } });
  },
  /** The workshop's brass clockwork on a wooden board: three meshed gears turning against each other. */
  northpolegears(x) {
    const brass = 0xc8a44e, brassL = 0xe8c878, brassD = 0x8a6a2a, wood = 0x6a4428;
    fill(x - 12, 8, 24, 18, wood); fill(x - 12, 8, 24, 1, 0x8a5a32);
    const gear = (gx, gy, r, spd) => { const a = now * spd; for (let t = 0; t < 8; t++) { const ta = a + t / 8 * Math.PI * 2; dot(gx + Math.round(Math.cos(ta) * (r + 1)), gy + Math.round(Math.sin(ta) * (r + 1)), brass); } disc(gx, gy, r, brass); ring(gx, gy, r, brassD); disc(gx, gy, 1.4, brassD); for (let t = 0; t < 4; t++) { const ta = a + t / 4 * Math.PI * 2; line(gx, gy, gx + Math.round(Math.cos(ta) * (r - 1)), gy + Math.round(Math.sin(ta) * (r - 1)), brassL, 0.5); } };
    gear(x - 5, 16, 5, 0.6); gear(x + 5, 20, 4, -0.75); gear(x + 6, 12, 3, 0.9);
  },
  /** A lighting truss of six studio lamps, some warm tungsten and some blue-gelled; one goes dark and re-strikes now and then. They glow at night. */
  soundstagerig(x) {
    const truss = 0x4a4e56, trussL = 0x6a6e76, trussD = 0x3a3e46, lamp = 0x2a2a2e, warm = 0xffcf6b, blue = 0x6aa0f8, cols = [0xffcf6b, 0x6aa0f8, 0xffcf6b, 0xffcf6b, 0x6aa0f8, 0xffcf6b];
    fill(x - 18, 6, 36, 3, truss); fill(x - 18, 6, 36, 1, trussL);
    for (let k = x - 16; k < x + 18; k += 4) dot(k, 7, trussD);
    for (let i = 0; i < 6; i++) { const lx = x - 15 + i * 6, on = i !== (Math.floor(now * 1.5) % 6), c = cols[i]; fill(lx, 9, 4, 4, lamp); fill(lx, 9, 4, 1, 0x3a3a40); fill(lx + 1, 12, 2, 1, on ? c : 0x3a3a40); if (on) for (let b = 0; b < 4; b++) dot(lx + 1 + (b % 2), 13 + b, c, 0.22 - b * 0.05); }
    if (isNight) glowLate(() => { for (let i = 0; i < 6; i++) { const lx = x - 15 + i * 6; if (i !== (Math.floor(now * 1.5) % 6)) glowAt(lx + 2, 14, 4, 5, cols[i], 0.14); } });
  },
  /** A dramatic movie poster in a frame: a lone hero in silhouette under a golden sun, a red title bar below. */
  soundstageposter(x) {
    const frame = 0x2a2a2e, frameL = 0x4a4a52, sky = 0x2a3a6a, skyL = 0x4a5a8a, figure = 0x1a1a1e, gold = 0xe8c24a, red = 0xd83030;
    fill(x - 9, 7, 18, 22, frame); fill(x - 9, 7, 18, 1, frameL); fill(x - 8, 8, 16, 20, sky); fill(x - 8, 8, 16, 6, skyL);
    disc(x + 4, 12, 2, gold); fill(x - 1, 16, 2, 7, figure); dot(x, 15, figure); fill(x - 3, 23, 6, 2, figure);
    fill(x - 8, 25, 16, 3, red); fill(x - 6, 26, 12, 1, gold);
  },
  /** A projection screen cycling through scenes - a landscape, a starry night, a western street - a cue dot flashing in the corner at the cut; glows at night. */
  soundstagescreen(x) {
    const frame = 0x2a2a2e;
    fill(x - 16, 6, 32, 20, frame); fill(x - 15, 7, 30, 18, 0x1a1a1e);
    const scene = Math.floor(now / 3) % 3;
    if (scene === 0) { fill(x - 14, 8, 28, 10, 0x6aa0f8); fill(x - 14, 16, 28, 7, 0x3a6a4a); disc(x + 8, 11, 2, 0xffe9b0); }
    else if (scene === 1) { fill(x - 14, 8, 28, 16, 0x2a2a4a); for (let k = 0; k < 10; k++) dot(x - 12 + (hash(k, scene) % 26), 9 + (hash(k, 7) % 14), WHITE, 0.7); disc(x, 14, 3, 0xe8c24a); }
    else { fill(x - 14, 8, 28, 16, 0xc08a4a); fill(x - 14, 18, 28, 6, 0x8a5a2e); fill(x + 2, 11, 4, 8, 0x5a3b25); }
    if ((now % 3) > 2.7) dot(x + 13, 8, WHITE, 0.9);
    if (isNight) glowLate(() => glowAt(x, 15, 16, 10, 0x8ab0e8, 0.12));
  },
  /** A Walk-of-Fame terrazzo star: a gold five-point star on a rose-red field with a blank brass name bar; a glint crosses it. */
  soundstagestar(x) {
    const terr = 0x7a2230, star = 0xe8c24a, starL = 0xf2d878, brass = 0xc8a44e;
    disc(x, 16, 10, terr); ring(x, 16, 10, 0x5a1620);
    for (let a = 0; a < 5; a++) { const an = -Math.PI / 2 + a / 5 * Math.PI * 2, ox = Math.round(Math.cos(an) * 6), oy = Math.round(Math.sin(an) * 6); line(x, 16, x + ox, 16 + oy, star); dot(x + ox, 16 + oy, starL); const an2 = an + Math.PI / 5, ix = Math.round(Math.cos(an2) * 3), iy = Math.round(Math.sin(an2) * 3); line(x, 16, x + ix, 16 + iy, star); }
    disc(x, 16, 2, starL);
    if ((now + x * 0.2) % 6 < 0.4) dot(x + 3, 13, WHITE, 0.9);
    fill(x - 4, 22, 8, 1, brass);
  },
  /** A pinned storyboard of six little sketched frames; a red marker ring moves from panel to panel. */
  soundstagestoryboard(x) {
    const board = 0x3a3e46, paper = 0xe8e4d8, ink = 0x5a5a5a, red = 0xd83030;
    fill(x - 16, 7, 32, 20, board); fill(x - 16, 7, 32, 1, 0x4a4e56);
    for (let i = 0; i < 6; i++) { const px = x - 14 + (i % 3) * 10, py = 9 + Math.floor(i / 3) * 9; fill(px, py, 8, 7, paper); fill(px, py, 8, 1, 0xfbf6ea); fill(px + 1, py + 4, 6, 1, ink, 0.6); dot(px + 3, py + 2, ink); dot(px + 4, py + 2, ink); fill(px + 2, py + 5, 2, 1, ink, 0.5); dot(px + 4, py, red); }
    const act = Math.floor(now / 2) % 6, ax = x - 14 + (act % 3) * 10, ay = 9 + Math.floor(act / 3) * 9; ring(ax + 4, ay + 3, 4, red, 0.8);
  },
  /** Two film reels mounted on a board, the strip threaded between them; they turn slowly. */
  soundstagereel(x) {
    const metal = 0x5a5e66, metalL = 0x8a8e96, metalD = 0x3a3e46, film = 0x2a2a2e;
    fill(x - 11, 8, 22, 18, 0x4a3020); fill(x - 11, 8, 22, 1, 0x6a4a30);
    const reel = (rx, ry, spd) => { disc(rx, ry, 5, metal); ring(rx, ry, 5, metalD); disc(rx, ry, 1.5, metalD); const a = now * spd; for (let h = 0; h < 5; h++) { const ha = a + h / 5 * Math.PI * 2; disc(rx + Math.round(Math.cos(ha) * 3), ry + Math.round(Math.sin(ha) * 3), 1, metalD); } dot(rx - 1, ry - 1, metalL); };
    reel(x - 5, 14, 0.8); reel(x + 5, 18, 0.8);
    for (let k = 0; k < 10; k++) dot(x - 5 + k, 19 + Math.round(Math.sin(k * 0.6) * 1), film);
  },
  /** A great painted all-seeing eye in a halo of golden rays; its iris drifts to watch, and it blinks slowly. It glows faintly at night. */
  culteye(x) {
    const paint = 0x3a3a5a, white = 0xeceae2, iris = 0xc08a5e, gold = 0xe0b24a, lid = 0xdcd4c2;
    for (let a = 0; a < 12; a++) { const an = a / 12 * Math.PI * 2; line(x, 16, x + Math.round(Math.cos(an) * 10), 16 + Math.round(Math.sin(an) * 10), gold, 0.25); }
    const blink = (now + x) % 7 < 0.3;
    fill(x - 7, 15, 14, 3, white); fill(x - 7, 15, 14, 1, lid); fill(x - 5, 14, 10, 1, white); fill(x - 5, 18, 10, 1, white);
    if (blink) fill(x - 7, 16, 14, 1, lid); else { const look = Math.round(Math.sin(now * 0.4) * 3); disc(x + look, 16, 2, iris); dot(x + look, 16, paint); dot(x + look - 1, 15, white, 0.6); }
    if (isNight) glowLate(() => glowAt(x, 16, 9, 6, gold, 0.1));
  },
  /** A radiant golden sun mandala with layered rays turning slowly, a serene closed-eyed face at its heart; it glows at night. */
  cultsun(x) {
    const gold = 0xe0b24a, goldL = 0xf2d070, goldD = 0xb8862a, a0 = now * 0.1;
    for (let a = 0; a < 16; a++) { const an = a0 + a / 16 * Math.PI * 2, r2 = a % 2 ? 11 : 9; line(x + Math.round(Math.cos(an) * 6), 16 + Math.round(Math.sin(an) * 6), x + Math.round(Math.cos(an) * r2), 16 + Math.round(Math.sin(an) * r2), a % 2 ? gold : goldD); }
    disc(x, 16, 6, gold); ring(x, 16, 6, goldD); disc(x, 16, 5, goldL);
    dot(x - 2, 15, goldD); dot(x + 2, 15, goldD); fill(x - 1, 18, 3, 1, goldD);
    if (isNight) glowLate(() => glowAt(x, 16, 10, 10, gold, 0.14));
  },
  /** A framed portrait of the Leader - serene, haloed, with an unnaturally wide smile and eyes that seem to follow you. */
  cultportrait(x) {
    const frame = 0xc08a5e, frameL = 0xd8a878, bg = 0xe8dcc0, robe = 0xf2ead8, skin = 0xe8c0a0, hair = 0xdcd8cc, gold = 0xe0b24a;
    fill(x - 8, 8, 16, 19, frame); fill(x - 8, 8, 16, 1, frameL); fill(x - 7, 9, 14, 17, bg);
    ring(x, 14, 6, gold, 0.5); ring(x, 14, 5, gold, 0.3);
    fill(x - 4, 19, 9, 7, robe); fill(x - 3, 12, 7, 6, skin); fill(x - 3, 11, 7, 2, hair); fill(x - 4, 12, 1, 4, hair); fill(x + 3, 12, 1, 4, hair);
    const look = Math.round(Math.sin(now * 0.3) * 1); dot(x - 2 + look, 14, PALETTE.eye); dot(x + 2 + look, 14, PALETTE.eye);
    fill(x - 2, 16, 5, 1, 0xc08060); dot(x - 2, 15, 0xc08060); dot(x + 2, 15, 0xc08060);
  },
  /** A board of the commune's tenets: a golden sun header over rows of calm script, each bulleted with a little sun; one line underlines in gold now and then. */
  culttenets(x) {
    const board = 0xe8dcc0, frame = 0xc08a5e, ink = 0x5a4a3a, gold = 0xe0b24a;
    fill(x - 9, 7, 18, 20, frame); fill(x - 8, 8, 16, 18, board); fill(x - 8, 8, 16, 1, 0xf2ead8);
    disc(x, 11, 2, gold); for (let a = 0; a < 6; a++) { const an = a / 6 * Math.PI * 2; dot(x + Math.round(Math.cos(an) * 3), 11 + Math.round(Math.sin(an) * 3), gold); }
    for (let i = 0; i < 5; i++) { const ly = 15 + i * 2; dot(x - 6, ly, gold); fill(x - 4, ly, 8, 1, ink, 0.6); }
    fill(x - 4, 15 + (Math.floor(now / 3) % 5) * 2 + 1, 8, 1, gold, 0.6);
  },
  /** A window onto the commune's fields: even rows of crops under a radiant sun, and one figure standing perfectly still. It glows at dusk. */
  cultwindow(x) {
    const frame = 0xc8bca0, frameL = 0xe0d6bc, sky = 0xbfd0e0, skyL = 0xdce6f0, field = 0x8a9a5a, fieldD = 0x6a7a4a, sun = 0xf2d070, figure = 0xeceae2;
    fill(x - 15, 6, 30, 22, frame); fill(x - 15, 6, 30, 1, frameL); fill(x - 13, 8, 28, 10, sky); fill(x - 13, 8, 28, 2, skyL);
    disc(x + 7, 12, 3, sun); for (let a = 0; a < 8; a++) { const an = a / 8 * Math.PI * 2; dot(x + 7 + Math.round(Math.cos(an) * 4), 12 + Math.round(Math.sin(an) * 4), sun); }
    fill(x - 13, 17, 28, 9, field); for (let k = x - 12; k < x + 14; k += 3) fill(k, 18, 1, 7, fieldD, 0.5);
    fill(x - 2, 19, 2, 5, figure); dot(x - 1, 18, figure);
    fill(x - 1, 8, 1, 18, frame); fill(x - 13, 16, 28, 1, frame);
    if (isNight) glowLate(() => glowAt(x + 7, 12, 6, 5, sun, 0.1));
  },
  /** A wrought-iron sconce of five candles, their flames flickering together; glows warm at night. */
  cultcandles(x) {
    const iron = 0x3a3630, ironL = 0x5a5648, wax = 0xe8e0cc, flame = 0xffd36b, flameC = 0xfff0b0;
    fill(x - 9, 20, 18, 2, iron); fill(x - 9, 20, 18, 1, ironL);
    for (let i = 0; i < 5; i++) { const cx = x - 8 + i * 4; fill(cx, 14, 2, 6, wax); fill(cx, 14, 1, 6, 0xf2ead8); const f = candlelight(cx * 0.3); fill(cx, 11, 2, 3, flame, f); dot(cx, 11, flameC, f); dot(cx, 13, flame); }
    if (isNight) glowLate(() => { for (let i = 0; i < 5; i++) { const cx = x - 8 + i * 4; glowAt(cx + 1, 12, 3, 4, flame, 0.14 * candlelight(cx)); } });
  },
  /** A jumbotron over the arena: a scoreboard bar up top and two teams of dots skirmishing across a dark playfield, a kill-feed flashing; it glows at night. */
  esportsscreen(x) {
    const frame = 0x2a2a36, dark = 0x0e0e18, rgb = (k) => [0x2ad8e8, 0x8a3ae8, 0xe82ad8, 0xf03a8a, 0x9ae82a, 0x3a6aff][((k % 6) + 6) % 6];
    fill(x - 18, 5, 36, 22, frame); fill(x - 18, 5, 36, 1, 0x3a3a46); fill(x - 16, 7, 32, 18, dark);
    fill(x - 16, 7, 32, 3, 0x1a1a26); fill(x - 15, 8, 4, 1, 0x2ad8e8); fill(x + 11, 8, 4, 1, 0xe82ad8); fill(x - 2, 7, 4, 3, 0x1a1a26); fill(x - 1, 8, 2, 1, 0xf0f4ff);
    const t = Math.floor(now * 3);
    for (let i = 0; i < 5; i++) { dot(x - 12 + ((t + i * 3) % 24), 14 + (i % 3) * 3, 0x2ad8e8); dot(x + 12 - ((t + i * 2) % 24), 16 - (i % 3) * 3, 0xe82ad8); }
    if (t % 6 < 2) fill(x - 15, 22, 10, 1, rgb(t));
    if (isNight) glowLate(() => glowAt(x, 15, 17, 10, 0x3a6aff, 0.12));
  },
  /** A tournament bracket converging to a golden champion slot; the advancing team's path lights up cyan. */
  esportsbracket(x) {
    const board = 0x1e1e28, line = 0x3a3a4a, lit = 0x2ad8e8, gold = 0xe8c24a;
    fill(x - 13, 7, 26, 20, board); fill(x - 13, 7, 26, 1, 0x2a2a36);
    const hi = Math.floor(now / 2) % 4;
    for (let i = 0; i < 4; i++) { const by = 10 + i * 4; fill(x - 11, by, 4, 1, i === hi ? lit : line); fill(x - 7, by, 1, 2, i === hi ? lit : line); }
    for (let i = 0; i < 2; i++) { const by = 12 + i * 8; fill(x - 7, by, 1, 4, line); fill(x - 6, by + 2, 4, 1, (hi >> 1) === i ? lit : line); }
    fill(x - 2, 16, 1, 4, line); fill(x - 1, 18, 5, 1, lit);
    fill(x + 4, 16, 5, 5, 0x2a2a1e); fill(x + 4, 16, 5, 1, gold); dot(x + 6, 18, gold);
  },
  /** A live leaderboard: ranked rows with scores, the top row gold-lit, a tick blinking as it updates. */
  esportsleaderboard(x) {
    const board = 0x14141c, row = 0x1e1e28, lit = 0x2ad8e8, gold = 0xe8c24a, txt = 0x6a6a7a;
    fill(x - 8, 6, 16, 21, board); fill(x - 8, 6, 16, 1, 0x2a2a36); fill(x - 7, 7, 14, 2, 0x2a2a36); fill(x - 6, 8, 4, 1, 0x2ad8e8);
    for (let i = 0; i < 6; i++) { const ry = 10 + i * 2; fill(x - 7, ry, 14, 1, i === 0 ? 0x2a3a2e : row); dot(x - 6, ry, i === 0 ? gold : txt); fill(x - 4, ry, 6, 1, i === 0 ? lit : txt, 0.7); fill(x + 3, ry, 3, 1, txt, 0.6); }
    if (Math.floor(now * 2) % 2) dot(x + 6, 10, lit);
  },
  /** A team pennant banner hanging from a rod, a neon chevron logo on it, swaying; glows at night. */
  esportsbanner(x) {
    const cloth = 0x2a1e4a, clothD = 0x1e1640, neon = 0x8a3ae8, neonL = 0xb86af0, white = 0xf0f4ff, sway = Math.round(Math.sin(now * 1.0) * 1);
    fill(x - 7, 6, 14, 2, 0x3a3a46);
    for (let r = 0; r < 18; r++) { const w = 14 - Math.floor(r / 3); fill(x - Math.floor(w / 2) + (r > 8 ? sway : 0), 8 + r, w, 1, r % 6 === 5 ? clothD : cloth); }
    fill(x - 2 + sway, 13, 5, 1, neon); fill(x - 1 + sway, 14, 3, 1, neonL); dot(x + sway, 15, white);
    fill(x - 6, 8, 1, 16, neon, 0.6); fill(x + 5, 8, 1, 16, neon, 0.6);
    if (isNight) glowLate(() => glowAt(x, 16, 7, 9, neon, 0.1));
  },
  /** A neon team-logo sign - a stylised beast head in cyan and pink - that buzzes and flickers now and then; strong glow at night. */
  esportsneon(x) {
    const neon = 0x2ad8e8, neonL = 0xaef4fb, off = 0x18343a, pink = 0xf03a8a;
    const on = !(Math.floor(now * 6) % 11 === 0), c = on ? neon : off;
    line(x - 7, 20, x - 3, 10, c); line(x - 3, 10, x, 16, c); line(x, 16, x + 3, 10, c); line(x + 3, 10, x + 7, 20, c);
    line(x - 7, 20, x + 7, 20, c); line(x - 3, 15, x - 1, 17, on ? pink : off); line(x + 3, 15, x + 1, 17, on ? pink : off);
    if (on) { dot(x - 2, 14, neonL); dot(x + 2, 14, neonL); }
    if (on && isNight) glowLate(() => glowAt(x, 15, 9, 7, neon, 0.2));
  },
  /** A lit glass trophy shelf: championship cups and medals on two shelves, a spotlight glint crossing them; glows faintly at night. */
  esportsshelf(x) {
    const shelf = 0x1e1e28, glass = 0x2ad8e8, gold = 0xe8c24a, goldL = 0xf2d878, silver = 0xb8bcc4;
    fill(x - 9, 7, 18, 20, shelf); fill(x - 9, 7, 18, 1, 0x2a2a36); fill(x - 8, 15, 16, 1, glass, 0.4); fill(x - 8, 23, 16, 1, glass, 0.4);
    fill(x - 5, 10, 2, 4, gold); fill(x - 6, 10, 4, 1, gold); dot(x - 5, 9, goldL); fill(x - 5, 14, 2, 1, gold);
    fill(x + 1, 11, 2, 3, silver); fill(x, 11, 4, 1, silver); disc(x + 5, 12, 1.5, gold);
    fill(x - 4, 18, 3, 4, gold); fill(x - 5, 18, 5, 1, gold); dot(x - 3, 17, goldL); disc(x + 3, 20, 1.5, silver);
    if (Math.floor(now * 2) % 5 === 0) dot(x - 4, 10, WHITE, 0.9);
    if (isNight) glowLate(() => glowAt(x, 16, 9, 9, glass, 0.08));
  },
  /** Stencilled ochre and red handprints scattered across the rock, with a few blown dots between them. */
  cavemanhandprints(x) {
    const red = 0xa0472a, redD = 0x7a2e1a, ochre = 0xc89a3e, ochreD = 0x9a742a;
    for (const [dx, dy, c] of [[-6, 12, red], [2, 10, ochre], [-2, 18, ochre], [5, 16, red], [-8, 20, red]]) { const hx = x + dx, hy = dy; fill(hx, hy, 3, 3, c); for (const [fx, fy] of [[0, -1], [1, -2], [2, -1], [-1, 0], [3, 0]]) dot(hx + fx, hy + fy, c); dot(hx + 1, hy + 1, shaded(c, 0.8)); }
    for (let k = 0; k < 6; k++) dot(x - 7 + (hash(k, 3) % 14), 8 + (hash(k, 7) % 18), k % 2 ? ochreD : redD, 0.6);
  },
  /** A cave-painting hunt scene: stick hunters with spears closing on a great beast, all in ochre; one hunter strides now and then. */
  cavemanhunt(x) {
    const paint = 0xa0472a, paintD = 0x7a2e1a, ochre = 0xc89a3e;
    fill(x - 16, 8, 32, 18, 0x645850); fill(x - 16, 8, 32, 1, 0x766658);
    fill(x + 4, 15, 8, 5, paint); fill(x + 3, 14, 3, 2, paint); fill(x + 11, 16, 2, 3, paint);
    line(x + 2, 13, x, 11, paintD); line(x + 6, 13, x + 4, 11, paintD);
    fill(x + 5, 20, 1, 3, paint); fill(x + 9, 20, 1, 3, paint);
    for (let i = 0; i < 3; i++) { const hx = x - 12 + i * 4, mv = (Math.floor(now * 2) % 3 === i) ? 1 : 0; fill(hx, 15, 1, 4, ochre); dot(hx, 14, ochre); line(hx, 19, hx - 1 + mv, 22, ochre); line(hx, 19, hx + 1, 22, ochre); line(hx + 1, 16, hx + 4, 14, ochre); }
  },
  /** A mammoth skull mounted on a rock ledge, its two great tusks curving down and out. */
  cavemanbones(x) {
    const bone = 0xe0d6b8, boneD = 0xc8bc98, boneS = 0xb0a480, rock = 0x4a4038;
    fill(x - 11, 22, 22, 4, rock); fill(x - 11, 22, 22, 1, 0x5e524a);
    fill(x - 5, 11, 10, 9, bone); fill(x - 5, 11, 10, 1, 0xf0e8cc); fill(x - 5, 19, 10, 1, boneD);
    fill(x - 3, 20, 6, 2, bone); fill(x - 2, 13, 4, 3, boneD); dot(x - 3, 14, 0x2a2018); dot(x + 2, 14, 0x2a2018);
    for (const [base, dir] of [[-4, -1], [4, 1]]) for (let k = 0; k < 8; k++) { const ty = 19 + k; const tx = x + base + dir * Math.round(Math.sin(k / 7 * Math.PI) * 4); if (ty > 29) break; dot(tx, ty, k > 5 ? boneS : bone); dot(tx + dir, ty, boneD); }
  },
  /** The first wheel, proudly displayed: a chisel-cut stone disc with a square axle hole, turning slowly, a gold plaque beneath. */
  cavemanwheel(x) {
    const stone = 0x6e6056, stoneL = 0x8a7c6e, stoneD = 0x4a4038, wood = 0x6a4a2a, a = now * 0.3;
    disc(x, 16, 9, stone); ring(x, 16, 9, stoneD); disc(x, 16, 8, stoneL, 0.3);
    for (let sp = 0; sp < 6; sp++) { const an = a + sp / 6 * Math.PI * 2; dot(x + Math.round(Math.cos(an) * 6), 16 + Math.round(Math.sin(an) * 6), stoneD); }
    fill(x - 1, 15, 2, 2, wood); fill(x - 5, 26, 10, 1, 0xc89a3e);
  },
  /** A cave mouth opening onto a prehistoric vista: ferns below, a smoking volcano in the distance and a pterodactyl gliding past; the lava glows at night. */
  cavemanwindow(x) {
    const rock = 0x3a322a, rockL = 0x5a4e44, sky = 0xe0885a, skyT = 0xf2b07a, volc = 0x4a3a32, lava = 0xe8782e, fern = 0x5a6a3a, bird = 0x2a2420;
    fill(x - 15, 6, 30, 22, rock);
    fill(x - 12, 9, 24, 15, sky); fill(x - 12, 9, 24, 3, skyT);
    for (let k = -12; k <= 12; k++) dot(x + k, 9 + (hash(k + 20, 2) % 2) - 1, rockL);
    fill(x + 4, 14, 7, 6, volc); fill(x + 5, 13, 5, 1, lava); dot(x + 7, 12, 0x6a6058, 0.6); dot(x + 7, 11, 0x5a5048, 0.5);
    const bx = x - 8 + Math.round((now * 4) % 16), by = 12 + Math.round(Math.sin(now) * 1); line(bx - 2, by, bx, by - 1, bird); line(bx, by - 1, bx + 2, by, bird);
    fill(x - 12, 20, 24, 4, fern); for (let k = x - 10; k < x + 12; k += 4) { line(k, 23, k - 1, 20, 0x6a7a4a); line(k, 23, k + 1, 20, 0x6a7a4a); }
    if (isNight) glowLate(() => glowAt(x + 7, 14, 4, 4, lava, 0.14));
  },
  /** A wall torch: a bound stick in an iron bracket with a bright flame that flickers and lights the rock; glows at night. */
  cavemantorch(x) {
    const stick = 0x5a3a20, bind = 0x8a6a42, flame = 0xffb04a, flameC = 0xffe08a, flameD = 0xe8782e, bracket = 0x3a3630;
    fill(x - 1, 18, 3, 1, bracket); fill(x, 18, 1, 2, bracket);
    fill(x - 1, 14, 2, 8, stick); fill(x - 1, 14, 1, 8, 0x6a4a2a); fill(x - 1, 16, 2, 1, bind);
    const f = candlelight(x * 0.4);
    fill(x - 2, 8, 4, 6, flameD, f); fill(x - 1, 7, 2, 5, flame, f); fill(x - 1, 8, 1, 3, flameC, f); dot(x, 6, flameC, f);
    if (isNight) glowLate(() => glowAt(x, 10, 5, 6, flame, 0.2 * f));
  },
  /** The eye-in-triangle emblem in gold on a dark panel, ringed with rays; the eye drifts to watch and blinks slowly; glows at night. */
  lodgeeye(x) {
    const gold = 0xc8a44e, goldL = 0xe8c878, white = 0xe0d2b0, iris = 0x3a2418;
    for (let a = 0; a < 12; a++) { const an = a / 12 * Math.PI * 2; line(x, 16, x + Math.round(Math.cos(an) * 10), 16 + Math.round(Math.sin(an) * 10), gold, 0.22); }
    line(x - 8, 20, x, 8, gold); line(x, 8, x + 8, 20, gold); line(x - 8, 20, x + 8, 20, gold); line(x - 7, 20, x + 1, 9, goldL, 0.5);
    const blink = (now + x) % 8 < 0.3;
    fill(x - 4, 15, 9, 3, white); fill(x - 3, 14, 7, 1, white); fill(x - 3, 18, 7, 1, white);
    if (blink) fill(x - 4, 16, 9, 1, 0xb8a888); else { const look = Math.round(Math.sin(now * 0.4) * 2); disc(x + look, 16, 1.6, iris); dot(x + look, 16, 0x1a1210); dot(x + look - 1, 15, white, 0.5); }
    if (isNight) glowLate(() => glowAt(x, 14, 9, 7, gold, 0.12));
  },
  /** The two lodge pillars in pale stone, each with a golden globe on its capital, a small checkerboard step laid between them. */
  lodgepillars(x) {
    const stone = 0xcfc6b0, stoneD = 0xa8a088, stoneL = 0xe0d8c2, gold = 0xc8a44e, dark = 0x3a342e;
    for (const px of [x - 13, x + 9]) { fill(px, 24, 5, 2, stoneD); fill(px, 24, 5, 1, stone); fill(px + 1, 10, 3, 14, stone); fill(px + 1, 10, 1, 14, stoneL); fill(px + 3, 10, 1, 14, stoneD); dot(px + 2, 14, stoneD); dot(px + 2, 18, stoneD); fill(px, 9, 5, 1, stoneD); disc(px + 2, 6, 2, gold); dot(px + 1, 5, 0xe8c878); }
    for (let c = 0; c < 6; c++) fill(x - 6 + c * 2, 24, 2, 2, c % 2 ? dark : stone);
  },
  /** A stern Grand Master in a heavy gilt frame, wearing a collar of office and a sash; his eyes follow you. */
  lodgeportrait(x) {
    const frame = 0xc8a44e, frameL = 0xe8c878, bg = 0x2a1a1a, coat = 0x1a1612, skin = 0xd8b090, hair = 0xb8b0a0, gold = 0xc8a44e, sash = 0x5a1e24;
    fill(x - 8, 8, 16, 19, frame); fill(x - 8, 8, 16, 1, frameL); fill(x - 7, 9, 14, 17, bg);
    fill(x - 4, 18, 9, 7, coat); fill(x - 2, 20, 1, 5, sash); fill(x + 1, 20, 1, 5, gold);
    fill(x - 3, 12, 7, 6, skin); fill(x - 3, 11, 7, 2, hair); fill(x - 4, 13, 1, 3, hair); fill(x + 3, 13, 1, 3, hair);
    const look = Math.round(Math.sin(now * 0.3)); dot(x - 2 + look, 14, PALETTE.eye); dot(x + 2 + look, 14, PALETTE.eye);
    fill(x - 1, 17, 3, 1, hair); dot(x, 16, 0xa09888);
  },
  /** A framed charter on parchment with a gold flourish, lines of oath and a red wax seal that glints. */
  lodgecharter(x) {
    const frame = 0x3a2418, parch = 0xe0d2b0, ink = 0x3a2a1a, wax = 0x8a1e1e, gold = 0xc8a44e;
    fill(x - 9, 7, 18, 20, frame); fill(x - 8, 8, 16, 18, parch); fill(x - 8, 8, 16, 1, 0xeedcb0);
    fill(x - 5, 10, 10, 1, gold); dot(x, 10, 0xe8c878);
    for (let i = 0; i < 6; i++) fill(x - 6, 13 + i * 2, 10 - (i === 5 ? 4 : 0), 1, ink, 0.6);
    disc(x + 4, 24, 2, wax); ring(x + 4, 24, 2, 0x6a1414); dot(x + 4, 24, 0xa82a2a);
    if ((now + x) % 6 < 0.4) dot(x + 3, 23, WHITE, 0.6);
  },
  /** A panel of arcane emblems - compass and square, a star, an hourglass, sun and moon - each lighting gold in turn. */
  lodgesymbols(x) {
    const panel = 0x2a1810, panelL = 0x3a2418, gold = 0xc8a44e, goldL = 0xe8c878, pale = 0xd8cbb0, hi = Math.floor(now / 2) % 4;
    fill(x - 11, 7, 22, 20, panel); fill(x - 11, 7, 22, 1, panelL);
    const c0 = hi === 0 ? goldL : gold; line(x - 8, 16, x - 5, 11, c0); line(x - 5, 11, x - 2, 16, c0); line(x - 8, 14, x - 2, 14, c0);
    const c1 = hi === 1 ? goldL : gold; for (let a = 0; a < 5; a++) { const an = -Math.PI / 2 + a / 5 * Math.PI * 2; dot(x + 5 + Math.round(Math.cos(an) * 3), 13 + Math.round(Math.sin(an) * 3), c1); } dot(x + 5, 13, c1);
    const c2 = hi === 2 ? goldL : gold; line(x - 8, 20, x - 4, 20, c2); line(x - 8, 24, x - 4, 24, c2); line(x - 8, 20, x - 4, 24, c2); line(x - 4, 20, x - 8, 24, c2);
    const c3 = hi === 3 ? goldL : gold; disc(x + 3, 22, 1.5, c3); for (let a = 0; a < 6; a++) { const an = a / 6 * Math.PI * 2; dot(x + 3 + Math.round(Math.cos(an) * 3), 22 + Math.round(Math.sin(an) * 3), c3); } dot(x + 7, 21, pale);
  },
  /** A brass wall sconce holding two candles, their flames flickering; glows at night. */
  lodgesconce(x) {
    const brass = 0xc8a44e, brassL = 0xe8c878, brassD = 0x8a6a2a, wax = 0xe8e0cc, flame = 0xffd36b, flameC = 0xfff0b0;
    fill(x - 1, 18, 3, 1, brass); fill(x, 18, 1, 4, brassD); line(x, 20, x - 4, 17, brass); line(x, 20, x + 4, 17, brass);
    for (const ax of [x - 4, x + 4]) { fill(ax, 13, 2, 4, wax); fill(ax, 13, 1, 4, 0xf2ead8); const f = candlelight(ax * 0.3); fill(ax, 10, 2, 3, flame, f); dot(ax, 9, flameC, f); }
    dot(x, 19, brassL);
    if (isNight) glowLate(() => { glowAt(x - 4, 11, 3, 4, flame, 0.14); glowAt(x + 4, 11, 3, 4, flame, 0.14); });
  },
  /** A situation board: a map patch strung with pinned photos and notes connected by red thread; a pin pulses now and then. */
  prepperboard(x) {
    const board = 0x4a463e, boardL = 0x5e5a50, map = 0x7a8a6a, note = 0xe8e4d0, red = 0xc0392b, pin = 0xc8a82a;
    fill(x - 17, 6, 34, 20, board); fill(x - 17, 6, 34, 1, boardL);
    fill(x - 15, 8, 14, 16, map); fill(x - 15, 8, 14, 1, 0x8a9a7a); for (let k = 0; k < 6; k++) dot(x - 14 + (hash(k, 3) % 12), 9 + (hash(k, 5) % 14), 0x5a6a4a, 0.6);
    const notes = [[2, 9], [9, 10], [4, 17], [11, 18]];
    for (const [dx, dy] of notes) { fill(x + dx, dy, 5, 4, note); fill(x + dx, dy, 5, 1, 0xf2eee0); fill(x + dx + 1, dy + 1, 3, 1, 0x6a665c, 0.5); dot(x + dx + 2, dy, red); }
    line(x + 4, 11, x + 11, 12, red); line(x + 11, 12, x + 6, 19, red); line(x + 6, 19, x + 13, 20, red); line(x - 8, 16, x + 4, 11, red);
    const hi = Math.floor(now) % notes.length; dot(x + notes[hi][0] + 2, notes[hi][1], pin);
  },
  /** A steel shelving unit stacked with canned food, water jugs and crates. */
  preppershelf(x) {
    const steel = 0x5a5e66, can = 0xb4702a, canL = 0xd08a3a, water = 0x4c8ba0, crate = 0x8a6a42, label = 0xe8e4d0;
    fill(x - 11, 7, 22, 19, 0x3a3e42);
    for (let sh = 0; sh < 3; sh++) { const sy = 11 + sh * 5; fill(x - 11, sy, 22, 1, steel); fill(x - 11, sy, 22, 1, 0x6e727c); }
    fill(x - 11, 7, 1, 19, steel); fill(x + 10, 7, 1, 19, steel);
    for (let i = 0; i < 5; i++) { const cx = x - 9 + i * 4; fill(cx, 8, 3, 3, can); fill(cx, 8, 3, 1, canL); fill(cx, 9, 3, 1, label); }
    for (let i = 0; i < 3; i++) { const cx = x - 9 + i * 7; fill(cx, 13, 5, 3, water); fill(cx, 13, 5, 1, 0x6aa8c0); fill(cx + 1, 12, 2, 1, 0x3a6a80); }
    fill(x - 9, 18, 8, 3, crate); fill(x - 9, 18, 8, 1, 0xa8824a); fill(x + 1, 18, 9, 3, crate); fill(x + 1, 18, 9, 1, 0xa8824a);
  },
  /** A radiation warning sign - the trefoil on caution yellow with striped borders and a blinking red light. */
  prepperhazard(x) {
    const sign = 0xd8a82a, signD = 0xa8821a, black = 0x2a2620, red = 0xe02a2a;
    for (let k = -11; k <= 11; k += 2) { dot(x + k, 7, (((k) / 2 | 0) % 2) ? sign : black); dot(x + k, 25, (((k) / 2 | 0) % 2) ? sign : black); }
    fill(x - 10, 8, 20, 17, sign); fill(x - 10, 8, 20, 1, 0xe8c04a); fill(x - 10, 24, 20, 1, signD);
    disc(x, 16, 2, black);
    for (let b = 0; b < 3; b++) { const a0 = -Math.PI / 2 + b / 3 * Math.PI * 2; for (let r = 3; r <= 6; r++) { dot(x + Math.round(Math.cos(a0 - 0.35) * r), 16 + Math.round(Math.sin(a0 - 0.35) * r), black); dot(x + Math.round(Math.cos(a0 + 0.35) * r), 16 + Math.round(Math.sin(a0 + 0.35) * r), black); if (r <= 5) dot(x + Math.round(Math.cos(a0) * r), 16 + Math.round(Math.sin(a0) * r), black); } }
    fill(x - 6, 22, 12, 1, black);
    const bl = Math.floor(now * 2) % 2; dot(x + 8, 10, bl ? red : 0x5a1a1a);
    if (bl && isNight) glowLate(() => glowAt(x + 8, 10, 2, 2, red, 0.14));
  },
  /** A pegboard rack of survival tools - an axe, crowbar, machete, wrench and a coil of rope - each with its painted outline. */
  preppertools(x) {
    const board = 0x5a4e3a, hole = 0x3a3226, steel = 0x8a8e96, steelL = 0xb8bcc4, wood = 0x8a5a32, rope = 0xa89668, outline = 0xcfc6b0;
    fill(x - 11, 7, 22, 20, board); fill(x - 11, 7, 22, 1, 0x6a5e4a);
    for (let yy = 9; yy < 25; yy += 3) for (let xx = x - 9; xx < x + 10; xx += 3) dot(xx, yy, hole, 0.5);
    fill(x - 8, 10, 1, 10, wood); fill(x - 9, 9, 4, 3, steel); fill(x - 9, 9, 4, 1, steelL);
    fill(x - 3, 10, 1, 11, steel); fill(x - 4, 10, 2, 1, steel); fill(x - 3, 20, 2, 1, steel); dot(x - 3, 10, steelL);
    fill(x + 2, 11, 1, 9, steel); fill(x + 2, 11, 1, 9, steelL); fill(x + 2, 20, 1, 2, wood);
    fill(x + 6, 11, 1, 8, steel); fill(x + 5, 10, 3, 2, steel); fill(x + 5, 18, 3, 2, steel);
    ring(x + 9, 22, 2, rope); dot(x + 9, 22, 0x8a7a50); fill(x - 1, 22, 4, 3, outline, 0.15);
  },
  /** A doomsday clock set near midnight over a red countdown readout; the second hand sweeps and the digits flicker. It glows at night. */
  prepperclock(x) {
    const frame = 0x3a3630, face = 0x1a1a1e, tick = 0x6a665c, red = 0xe02a2a, hand = 0xcfc6b0;
    fill(x - 10, 7, 20, 20, frame); fill(x - 10, 7, 20, 1, 0x5a564c);
    disc(x, 14, 7, 0x2a2a30); ring(x, 14, 7, 0x4a4e56);
    for (let h = 0; h < 12; h++) { const a = h / 12 * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 6), 14 + Math.round(Math.sin(a) * 6), tick); }
    line(x, 14, x - 1, 9, hand); line(x, 14, x + 1, 10, hand);
    const sa = (now % 60) / 60 * Math.PI * 2 - Math.PI / 2; line(x, 14, x + Math.round(Math.cos(sa) * 5), 14 + Math.round(Math.sin(sa) * 5), red); disc(x, 14, 1, red);
    fill(x - 7, 22, 14, 3, face); fill(x - 7, 22, 14, 1, 0x0a0a0e);
    for (let d = 0; d < 4; d++) fill(x - 6 + d * 3 + (d >= 2 ? 1 : 0), 23, 2, 1, (Math.floor(now * 2) + d) % 5 === 0 ? 0x5a1a1a : red); dot(x, 23, red);
    if (isNight) glowLate(() => glowAt(x, 23, 7, 2, red, 0.12));
  },
  /** A ventilation duct with a fan spinning behind a grille and a valve wheel on the end. */
  preppervent(x) {
    const metal = 0x5a5e66, metalL = 0x7a7e86, metalD = 0x3a3e46, grille = 0x2a2e36, valve = 0xb4702a;
    fill(x - 15, 10, 30, 10, metal); fill(x - 15, 10, 30, 1, metalL); fill(x - 15, 19, 30, 1, metalD);
    for (let bx = x - 12; bx < x + 15; bx += 7) { fill(bx, 10, 1, 10, metalD); dot(bx, 11, metalL); dot(bx, 18, metalD); }
    disc(x - 7, 15, 4, grille); const a = now * 2; for (let sp = 0; sp < 4; sp++) { const sa = a + sp / 4 * Math.PI * 2; line(x - 7, 15, x - 7 + Math.round(Math.cos(sa) * 3), 15 + Math.round(Math.sin(sa) * 3), 0x4a4e56); } for (let g = -3; g <= 3; g += 2) dot(x - 7, 15 + g, metalL, 0.6);
    ring(x + 8, 15, 3, metalD); for (let sp = 0; sp < 6; sp++) { const sa = sp / 6 * Math.PI * 2; dot(x + 8 + Math.round(Math.cos(sa) * 3), 15 + Math.round(Math.sin(sa) * 3), valve); } disc(x + 8, 15, 1, valve);
  },
  /** Seal plaque. A round gilt plaque: a ring of tiny stars round a cream field with a generic spread-winged eagle in gold. */
  paradisesealplaque(x) {
    const gold = 0xd6b25e, goldLight = 0xeccf84, goldDark = 0xa8863a, cream = 0xeae3d2, navy = 0x1f3a66;
    fill(x - 9, 29, 18, 1, SHADOW);
    ring(x, 16, 9, goldDark); ring(x, 16, 8.3, gold); ring(x, 16, 7, goldLight, 0.6);
    disc(x, 16, 6.6, cream); disc(x, 16, 6.6, navy, 0.12);
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 5.4), 16 + Math.round(Math.sin(a) * 5.4), gold); }
    // The eagle: body, spread wings, a sprig of claws below
    fill(x - 1, 13, 3, 6, goldDark); dot(x, 12, gold);
    fill(x - 6, 15, 5, 1, gold); fill(x - 5, 16, 4, 1, goldDark); fill(x + 2, 15, 5, 1, gold); fill(x + 2, 16, 4, 1, goldDark);
    dot(x, 19, gold); dot(x - 1, 20, goldDark); dot(x + 1, 20, goldDark);
    dot(x - 8, 13, WHITE, 0.5);
  },
  /** Fake sky window. Red drapes under a gilt pelmet framing an LED panel of too-perfect blue sky — an even gradient, a round sun, a horizon seam and faint scanlines; it blinks for an instant now and then and glows all night, never off. */
  paradisefakewindow(x) {
    const drape = 0x8a2230, drapeLight = 0xb03040, drapeDark = 0x5e141e, gold = 0xd6b25e, frame = 0x3a2e22;
    const sky = 0x6fb8e8, skyLow = 0x9fd4f2, cloud = 0xf2f6fb;
    fill(x - 10, 3, 20, 25, frame); fill(x - 9, 4, 18, 23, 0x2a2016);
    const flick = (now + x * 0.2) % 11 < 0.12;
    for (let r = 0; r < 20; r++) fill(x - 8, 5 + r, 16, 1, r < 10 ? sky : skyLow, flick ? 0.5 : 1);
    fill(x - 5, 9, 6, 2, cloud); fill(x - 6, 10, 8, 1, cloud); fill(x + 2, 13, 5, 1, cloud, 0.8);
    fill(x - 8, 15, 16, 1, 0x4a90c8, 0.5);
    for (let r = 6; r < 24; r += 3) fill(x - 8, r, 16, 1, 0x000000, 0.06);
    disc(x + 5, 8, 2.2, 0xfff2c0); glowAt(x + 5, 8, 4, 4, 0xfff2c0, 0.12);
    for (const [sx, dir] of [[x - 14, 1], [x + 11, -1]]) {
      fill(sx, 2, 4, 26, drape); fill(sx + (dir > 0 ? 0 : 3), 2, 1, 26, drapeLight); fill(sx + (dir > 0 ? 3 : 0), 2, 1, 26, drapeDark);
      dot(sx + 1, 14, drapeDark); fill(sx, 14, 4, 1, gold);
    }
    fill(x - 15, 1, 30, 2, gold); fill(x - 15, 1, 30, 1, 0xeccf84);
    // The panel never goes dark — a fake daytime sky glowing on through the night
    if (isNight) glowLate(() => {
      for (let r = 0; r < 20; r++) fill(x - 8, 5 + r, 16, 1, r < 10 ? sky : skyLow, 0.72);
      fill(x - 5, 9, 6, 2, cloud, 0.7); fill(x - 6, 10, 8, 1, cloud, 0.7); disc(x + 5, 8, 2.2, 0xfff2c0);
      glowAt(x, 14, 12, 10, skyLow, 0.12);
    });
  },
  /** Surveillance monitors. A dark wall rack of six little CRT feeds of the too-perfect suburb outside — blue sky, green lawns, identical houses, the odd too-still figure; one channel drops to red snow for a moment now and then. */
  paradisemonitors(x) {
    const case_ = 0x2a2e36, caseLight = 0x3a3f48, screen = 0x16352a, grass = 0x3a7a46, house = 0xd8cdb8, roof = 0x8a4a36, sky = 0x4a6a7a;
    fill(x - 15, 28, 30, 1, SHADOW);
    fill(x - 15, 3, 30, 24, case_); fill(x - 15, 3, 30, 1, caseLight); fill(x - 15, 26, 30, 1, 0x1a1d22);
    for (let i = 0; i < 6; i++) {
      const sx = x - 14 + (i % 3) * 10, sy = 5 + Math.floor(i / 3) * 11;
      const glitch = ((now + i * 1.7 + x * 0.1) % 9) < 0.5 && i === ((Math.floor(now / 9) + x) % 6);
      fill(sx, sy, 9, 9, glitch ? 0x5a1a1a : screen);
      if (glitch) { for (let k = 0; k < 4; k++) fill(sx, sy + 1 + k * 2, 9, 1, 0x8a2a2a, 0.6); dot(sx + 4, sy + 4, 0xff6a5a); }
      else {
        fill(sx, sy, 9, 4, sky); fill(sx, sy + 5, 9, 4, grass);
        fill(sx + 2, sy + 3, 4, 3, house); fill(sx + 2, sy + 2, 4, 1, roof); dot(sx + 3, sy + 4, 0x6a7a4a);
        if (i % 2) dot(sx + 6, sy + 6, 0xe8d0a0);
      }
      fill(sx, sy, 9, 1, 0xffffff, 0.08);
    }
    fill(x - 15, 3, 1, 24, caseLight); fill(x + 14, 3, 1, 24, 0x1a1d22);
  },
  /** Blast hatch. A round riveted steel vault hatch set in the wall, its wheel turning a notch every 18 s, a red status light blinking on the frame. */
  paradiseblasthatch(x) {
    const steel = 0x6e727c, steelLight = 0xa8adb6, steelDark = 0x4a4e58;
    fill(x - 11, 28, 22, 1, SHADOW);
    fill(x - 11, 4, 22, 23, steelDark); fill(x - 11, 4, 22, 1, steel);
    disc(x, 16, 10, steel); ring(x, 16, 10, steelDark); disc(x, 16, 9, steelLight, 0.18);
    for (let k = 0; k < 10; k++) { const a = (k / 10) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 9), 16 + Math.round(Math.sin(a) * 9), steelDark); }
    const turned = Math.floor(now / 18) % 2;
    ring(x, 16, 5, steelDark); disc(x, 16, 1.6, steelLight);
    if (turned) { line(x - 4, 12, x + 4, 20, steel); line(x + 4, 12, x - 4, 20, steel); } else { line(x, 10, x, 22, steel); line(x - 6, 16, x + 6, 16, steel); }
    const on = Math.floor(now * 1.5) % 2;
    fill(x + 7, 6, 3, 2, steelDark); dot(x + 8, 7, on ? 0xff3a2a : 0x6a1a14);
    if (isNight && on) glowLate(() => glowAt(x + 8, 7, 3, 3, 0xff3a2a, 0.2));
  },
  /** Doomsday clock. A gilt-ringed black dial a few minutes to midnight, the amber minute hand creeping, over a red alert strip that blinks; it glows red, more at night. */
  paradisedoomsday(x) {
    const case_ = 0x1a1d22, ring_ = 0xc8a050, face = 0x14110e, red = 0xff3a2a, amber = 0xf2a81a;
    fill(x - 9, 28, 18, 1, SHADOW);
    fill(x - 9, 4, 18, 23, case_); fill(x - 9, 4, 18, 1, 0x2e333c);
    disc(x, 14, 8, face); ring(x, 14, 8, ring_);
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2; dot(x + Math.round(Math.cos(a) * 6.5), 14 + Math.round(Math.sin(a) * 6.5), k % 3 === 0 ? ring_ : 0x6a5a2a); }
    const creep = Math.sin(now * 0.4) * 0.04, ma = -Math.PI / 2 + 0.18 + creep;
    line(x, 14, x, 8, red); line(x, 14, x + Math.round(Math.cos(ma) * 5.5), 14 + Math.round(Math.sin(ma) * 5.5), amber);
    disc(x, 14, 1.3, ring_);
    const blink = Math.floor(now * 2) % 2;
    fill(x - 8, 23, 16, 3, blink ? red : 0x5a1612); for (let k = 0; k < 4; k++) dot(x - 5 + k * 3, 24, 0x14110e, 0.5);
    glowAt(x, 14, 9, 9, red, isNight ? 0.14 : 0.07);
    if (isNight) glowLate(() => { glowAt(x, 14, 10, 10, red, 0.16); if (blink) glowAt(x, 24, 9, 3, red, 0.2); });
  },
  /** President's portrait. A flattering official portrait in a gilt frame: a figure in a navy suit and red tie against a slate-blue ground. */
  paradiseportrait(x) {
    const gold = 0xd6b25e, goldLight = 0xeccf84, goldDark = 0xa8863a, bg = 0x2a3550, navy = 0x1f2a44, skin = 0xe8c0a0, red = 0xc62828, hair = 0xe8d08a;
    fill(x - 9, 28, 18, 1, SHADOW);
    fill(x - 9, 3, 18, 24, goldDark); fill(x - 8, 4, 16, 22, gold); fill(x - 8, 4, 16, 1, goldLight); fill(x - 7, 5, 14, 20, bg);
    fill(x - 4, 9, 8, 7, navy); fill(x - 4, 9, 1, 7, 0x2e3c5e);
    fill(x - 2, 10, 4, 3, skin); fill(x - 2, 8, 4, 2, hair); dot(x - 1, 11, 0x5a4634); dot(x + 1, 11, 0x5a4634);
    fill(x, 13, 1, 4, red); dot(x, 12, 0xe24a4a);
    fill(x - 1, 13, 3, 1, 0xf2efe8);
    dot(x - 8, 5, WHITE, 0.5);
  },
  /** Main viewscreen. A big framed screen of stars that, every 16 s, stretch into warp streaks for a few seconds; it glows blue, more at warp and at night. */
  startrekviewscreen(x) {
    const frame = 0x3a4458, frameLight = 0x5a6478, space = 0x0a1028, star = 0xdfe8ff;
    fill(x - 17, 28, 34, 1, SHADOW);
    fill(x - 17, 3, 34, 24, frame); fill(x - 17, 3, 34, 1, frameLight); fill(x - 17, 26, 34, 1, 0x1a1f28);
    fill(x - 15, 5, 30, 20, space);
    const warp = (now + x * 0.1) % 16 < 3;
    for (let i = 0; i < 26; i++) {
      const h = hash(i * 7 + x, 1), sx = x - 15 + (h % 30), sy = 5 + ((h >> 5) % 20);
      if (warp) { const len = 2 + (h % 5); fill(sx - len, sy, len + 1, 1, star, 0.7); }
      else dot(sx, sy, star, 0.5 + (h % 5) / 8);
    }
    if (warp) { fill(x - 15, 14, 30, 1, 0x4a6ea8, 0.2); fill(x - 15, 15, 30, 1, 0x6a8ec8, 0.15); }
    dot(x - 16, 5, frameLight);
    if (isNight) glowLate(() => glowAt(x, 15, 16, 10, warp ? 0x6a8ec8 : 0x3a5a9a, 0.09));
  },
  /** Control panels. A bank of back-lit LCARS panels: a rail of coloured blocks, a grid of readouts and a little bar graph that rises and falls. */
  startreklcars(x) {
    const dark = 0x1a1f28, cols = [0xd4a02a, 0x4a8ec8, 0xc0566a, 0x6abf8a, 0xe08a3a];
    fill(x - 15, 28, 30, 1, SHADOW);
    fill(x - 15, 3, 30, 24, 0x242a38); fill(x - 15, 3, 30, 1, 0x3a4458);
    for (let i = 0; i < 5; i++) { const c = cols[i % cols.length]; fill(x - 14, 5 + i * 4, 6, 3, c, 0.9); fill(x - 14, 5 + i * 4, 6, 1, shaded(c, 1.2)); }
    for (let r = 0; r < 4; r++) for (let cc = 0; cc < 5; cc++) {
      const h = hash(cc * 3 + x, r), on = h % 3;
      fill(x - 6 + cc * 4, 5 + r * 5, 3, 3, on ? cols[(h >> 2) % cols.length] : dark, on ? 0.85 : 1);
    }
    const bar = Math.floor((Math.sin(now * 0.8 + x) + 1) / 2 * 5);
    for (let k = 0; k < 6; k++) fill(x + 11, 24 - k * 3, 3, 2, k <= bar ? 0x6abf8a : dark);
    if (isNight) glowLate(() => glowAt(x - 8, 15, 10, 10, 0xd4a02a, 0.06));
  },
  /** Dedication plaque. A brushed-silver plaque with an engraved saucer-and-nacelle ship and lines of fine text. */
  startrekplaque(x) {
    const silver = 0xb8bcc4, silverLight = 0xe0e4ec, silverDark = 0x7a7e86, ink = 0x2a2f3a;
    fill(x - 9, 28, 18, 1, SHADOW);
    fill(x - 9, 5, 18, 18, silver); fill(x - 9, 5, 18, 1, silverLight); fill(x - 9, 22, 18, 1, silverDark); fill(x + 8, 5, 1, 18, silverDark);
    disc(x - 1, 10, 3, silverDark); fill(x - 1, 12, 7, 1, silverDark); fill(x + 3, 13, 3, 1, silverDark); dot(x + 6, 13, 0x4a8ec8);
    for (let r = 0; r < 3; r++) fill(x - 6, 16 + r * 2, 12 - r * 2, 1, ink, 0.5);
  },
  /** Red-alert strip. An illuminated bar in a dark frame that pulses red; it glows, more at night. */
  startrekalert(x) {
    const frame = 0x2a2f3a, red = 0xd81a1a, redLight = 0xff5a4a, redDark = 0x6a0e0e;
    fill(x - 15, 28, 30, 1, SHADOW);
    fill(x - 15, 10, 30, 10, frame); fill(x - 15, 10, 30, 1, 0x3a4048);
    const pulse = 0.4 + 0.6 * ((Math.sin(now * 3) + 1) / 2);
    fill(x - 13, 12, 26, 6, redDark); fill(x - 13, 13, 26, 4, red, pulse); fill(x - 13, 13, 26, 1, redLight, pulse);
    for (let k = 0; k < 6; k++) fill(x - 12 + k * 4, 14, 1, 2, 0x2a0a0a, 0.6);
    glowAt(x, 15, 15, 5, red, isNight ? 0.12 * pulse : 0.06 * pulse);
    if (isNight) glowLate(() => glowAt(x, 15, 16, 6, red, 0.15 * pulse));
  },
  /** Astrometrics chart. A round star chart in a frame: faint orbit rings, a gold sun, scattered stars and a green blip orbiting round. */
  startrekstarmap(x) {
    const frame = 0x2a2f3a, space = 0x0c1430, ring = 0x4a8ec8, star = 0xdfe8ff, sun = 0xf2d87a;
    fill(x - 9, 28, 18, 1, SHADOW);
    fill(x - 10, 4, 20, 20, frame); fill(x - 10, 4, 20, 1, 0x3a4458);
    disc(x, 14, 9, space);
    ellipseRing(x, 14, 8, 6, ring, 0.4); ellipseRing(x, 14, 5, 3.5, ring, 0.5);
    disc(x, 14, 1.6, sun);
    for (let i = 0; i < 10; i++) { const h = hash(i + x, 3); dot(x - 7 + (h % 14), 8 + ((h >> 4) % 12), star, 0.6); }
    const a = now * 0.5; dot(x + Math.round(Math.cos(a) * 8), 14 + Math.round(Math.sin(a) * 6), 0x6abf8a);
    if (isNight) glowLate(() => glowAt(x, 14, 10, 8, ring, 0.08));
  },
  /** Starfield viewport. An angled window onto space with scattered stars and a ringed blue planet. */
  startrekport(x) {
    const frame = 0x3a4458, frameLight = 0x5a6478, space = 0x0a1028, star = 0xdfe8ff, planet = 0x3a6ea8, planetLight = 0x5a8ec8;
    fill(x - 11, 28, 22, 1, SHADOW);
    fill(x - 11, 3, 22, 24, frame); fill(x - 11, 3, 22, 1, frameLight);
    fill(x - 9, 5, 18, 20, space);
    for (let i = 0; i < 16; i++) { const h = hash(i * 5 + x, 2); dot(x - 9 + (h % 18), 5 + ((h >> 4) % 20), star, 0.4 + (h % 4) / 8); }
    disc(x + 3, 18, 4, planet); disc(x + 2, 17, 2.4, planetLight, 0.5); fill(x + 4, 15, 1, 7, 0x2a4a7a, 0.4);
    ellipseRing(x + 3, 18, 6, 2, 0x8aa8d0, 0.5);
    fill(x - 9, 14, 18, 1, frame, 0.6); fill(x, 5, 1, 20, frame, 0.6);
    if (isNight) glowLate(() => glowAt(x + 3, 18, 6, 6, planetLight, 0.08));
  },
  /** A stag head. A mounted stag on a wooden shield plaque, antlers spreading, a noble face with dark eyes. */
  gentlemenstag(x) {
    const wood = 0x5a3b25, woodLight = 0x7a5232, woodDark = 0x3a2616, fur = 0x6a4a2e, furLight = 0x8a6038, furDark = 0x3a2616, antler = 0xcabf9a, nose = 0x2a1a12;
    fill(x - 9, 28, 18, 1, SHADOW);
    fill(x - 7, 20, 14, 8, wood); fill(x - 7, 20, 14, 1, woodLight); fill(x - 6, 27, 12, 1, woodDark); fill(x - 5, 28, 10, 1, woodDark);
    fill(x - 3, 14, 6, 8, fur); fill(x - 3, 14, 6, 1, furLight); fill(x - 3, 20, 6, 2, furDark);
    fill(x - 2, 20, 4, 3, fur);
    dot(x, 22, nose); dot(x - 2, 17, PALETTE.eye); dot(x + 2, 17, PALETTE.eye);
    fill(x - 4, 14, 1, 2, furDark); fill(x + 4, 14, 1, 2, furDark);
    for (const dir of [-1, 1]) {
      line(x + dir, 14, x + dir * 6, 6, antler); line(x + dir * 4, 9, x + dir * 7, 7, antler); line(x + dir * 3, 11, x + dir * 5, 8, antler);
      dot(x + dir * 6, 5, antler); dot(x + dir * 7, 7, antler);
    }
  },
  /** An oil portrait. An old-master gentleman in a dark coat and white cravat, in a heavy gilt frame. */
  gentlemenportrait(x) {
    const gold = 0xc8a050, goldLight = 0xeccf84, goldDark = 0x8a6a2a, bg = 0x2a241c, coat = 0x3a2a1a, skin = 0xd8a888, hair = 0x5a4634, white = 0xd8d0c0;
    fill(x - 9, 28, 18, 1, SHADOW);
    fill(x - 9, 4, 18, 23, goldDark); fill(x - 8, 5, 16, 21, gold); fill(x - 8, 5, 16, 1, goldLight); fill(x - 7, 6, 14, 19, bg);
    fill(x - 4, 14, 8, 11, coat); fill(x - 4, 14, 1, 11, 0x4a3524);
    fill(x - 1, 15, 2, 4, white);
    fill(x - 2, 10, 4, 4, skin); fill(x - 2, 8, 4, 2, hair); dot(x - 1, 12, PALETTE.eye); dot(x + 1, 12, PALETTE.eye);
    dot(x - 2, 13, 0xb87858);
    dot(x - 7, 6, WHITE, 0.4);
  },
  /** A gun cabinet. A glass-fronted wooden cabinet with two shotguns standing inside and a little brass lock. */
  gentlemenguncabinet(x) {
    const wood = 0x4a3524, woodLight = 0x6a4a2e, woodDark = 0x2e2016, glass = 0xafc8d0, steel = 0x8a8f9c, barrel = 0x3a3d47, stock = 0x5a3b25;
    fill(x - 11, 28, 22, 1, SHADOW);
    fill(x - 11, 4, 22, 24, wood); fill(x - 11, 4, 22, 1, woodLight); fill(x - 11, 27, 22, 1, woodDark);
    fill(x - 9, 6, 18, 20, 0x2a2e30); fill(x - 9, 6, 18, 20, glass, 0.3); fill(x - 9, 6, 18, 1, glass, 0.6);
    for (const gx of [x - 5, x + 2]) { fill(gx, 7, 1, 13, barrel); fill(gx + 1, 7, 1, 13, 0x4a4e58); fill(gx - 1, 18, 3, 6, stock); dot(gx, 7, steel); }
    fill(x - 9, 15, 18, 1, woodDark, 0.5);
    dot(x + 8, 16, PALETTE.brass);
    dot(x - 8, 7, WHITE, 0.4);
  },
  /** An estate map. A framed parchment map with green fields, a winding river and a little manor house. */
  gentlemenmap(x) {
    const frame = 0x4a3524, frameLight = 0x6a4a2e, parch = 0xd8c89a, parchDark = 0xc0ac7a, ink = 0x5a4a2e, green = 0x6a8a4a, water = 0x6a9ac0, house = 0x8a3a2a;
    fill(x - 11, 28, 22, 1, SHADOW);
    fill(x - 11, 4, 22, 23, frame); fill(x - 11, 4, 22, 1, frameLight);
    fill(x - 9, 6, 18, 19, parch); fill(x - 9, 6, 18, 1, 0xe8d8a8);
    fill(x - 7, 9, 6, 5, green, 0.4); fill(x + 1, 15, 6, 6, green, 0.4);
    for (let k = 0; k < 9; k++) dot(x - 8 + k * 2, 8 + ((k * 5) % 12), water, 0.6);
    fill(x - 2, 11, 3, 2, house); dot(x - 1, 10, 0x5a2a1e);
    for (let r = 0; r < 3; r++) fill(x - 7, 20 + r * 2, 10, 1, ink, 0.35);
    fill(x - 9, 6, 1, 19, parchDark);
  },
  /** A grow-room window. A window onto a hidden grow-room: rows of plants under pink grow-lights behind misted glass; it glows at night. */
  gentlemengrowroom(x) {
    const frame = 0x3a4048, frameLight = 0x5a6068, glass = 0xbfe0d0, leaf = 0x3a8a3a, leafLight = 0x5aba5a, pot = 0x5a3b25, grow = 0xd060d0, growLight = 0xf0a0f0;
    fill(x - 15, 28, 30, 1, SHADOW);
    fill(x - 15, 3, 30, 25, frame); fill(x - 15, 3, 30, 1, frameLight);
    fill(x - 13, 5, 26, 21, 0x14211a);
    for (let gx = x - 11; gx < x + 12; gx += 6) { fill(gx, 6, 4, 1, grow); dot(gx + 1, 6, growLight); }
    for (let row = 0; row < 2; row++) for (let px = x - 12; px < x + 12; px += 5) {
      const py = 14 + row * 8;
      fill(px, py, 3, 2, pot); fill(px - 1, py - 4, 5, 4, leaf); dot(px, py - 5, leafLight); dot(px + 2, py - 3, leafLight);
    }
    fill(x - 13, 5, 26, 21, glass, 0.12); for (let k = 0; k < 5; k++) dot(x - 10 + k * 5, 5, WHITE, 0.3);
    fill(x - 9, 5, 1, 21, glass, 0.3); fill(x + 2, 5, 1, 21, glass, 0.3);
    glowAt(x, 10, 15, 6, grow, isNight ? 0.16 : 0.08);
    if (isNight) glowLate(() => glowAt(x, 13, 16, 12, grow, 0.18));
  },
  /** A hunting print. A framed scene of a red-coated rider and hounds across a green field under a pale sky. */
  gentlemenhunt(x) {
    const gold = 0xc8a050, goldDark = 0x8a6a2a, sky = 0xbcd0dc, field = 0x6a8a4a, fieldDark = 0x4a6a32, horse = 0x5a3b25, coat = 0x9a2a2a, hound = 0xe0d0b0;
    fill(x - 9, 28, 18, 1, SHADOW);
    fill(x - 10, 4, 20, 20, goldDark); fill(x - 9, 5, 18, 18, gold); fill(x - 8, 6, 16, 16, sky);
    fill(x - 8, 15, 16, 7, field); fill(x - 8, 15, 16, 1, fieldDark);
    fill(x - 2, 12, 4, 3, horse); fill(x - 2, 15, 1, 2, horse); fill(x + 1, 15, 1, 2, horse);
    fill(x - 1, 10, 2, 3, coat); dot(x, 9, 0xd8a888);
    dot(x + 4, 17, hound); dot(x + 6, 17, hound); dot(x + 3, 18, hound);
    dot(x - 8, 6, WHITE, 0.4);
  },
  /** A GROK neon: the word in white tube letters on a black acrylic plate, an ion-blue halo; every 25 s one letter stutters out and back. Without GROK_NODS, a lit ring. */
  grokneon(x) {
    const plate = 0x15161a, plateEdge = 0x2a2c33, tube = 0xf4f1ea, halo = 0x8ab4ff;
    fill(x - 11, 9, 23, 13, plate); fill(x - 11, 9, 23, 1, plateEdge); fill(x - 11, 9, 1, 13, plateEdge); fill(x - 10, 22, 22, 1, SHADOW);
    dot(x - 10, 10, 0x6e727c); dot(x + 10, 10, 0x6e727c); // its standoffs
    const t = (now + x * 0.37) % 25, out = t < 1.2 && Math.floor(t * 8) % 2 === 0 ? Math.floor(hash(x, Math.floor(now / 25)) % 4) : -1;
    const word = () => {
      if (!GROK_NODS) { ring(x, 15, 3.4, tube); return; }
      [...'GROK'].forEach((c, i) => { if (i !== out) pixelText(c, x - 7 + i * 4, 13, tube, false); });
    };
    glowAt(x, 15, 11, 5, halo, isNight ? 0.2 : 0.08);
    word();
    if (isNight) glowLate(() => { glowAt(x, 15, 10, 4, halo, 0.14); word(); });
  },
  /** A framed spiral galaxy: a deep-space print in a thin white frame, two arms of pale stars round a warm core; its stars twinkle in turn. */
  grokgalaxy(x) {
    const frame = 0xe8e6e0, frameDark = 0xb8bac0, space = 0x0e1018, core = 0xffe6b0, arm = 0xa8b8e8;
    fill(x - 10, 6, 21, 17, frame); fill(x + 10, 6, 1, 17, frameDark); fill(x - 10, 22, 21, 1, frameDark); fill(x - 9, 23, 21, 1, SHADOW);
    fill(x - 8, 8, 17, 13, space);
    // Two arms, wound round the core
    for (let k = 0; k < 18; k++) {
      const a = k * 0.42, r = 1 + k * 0.38;
      for (const turn of [0, Math.PI]) {
        const px = x + Math.round(Math.cos(a + turn) * r * 1.3), py = 14 + Math.round(Math.sin(a + turn) * r * 0.8);
        if (px > x - 8 && px < x + 9 && py > 8 && py < 21) dot(px, py, arm, 0.9 - k * 0.04);
      }
    }
    disc(x, 14, 1.4, core); dot(x, 14, WHITE);
    // Background stars, one bright in turn
    const bright = Math.floor(now * 1.5 + x) % 5;
    [[-6, 9], [6, 10], [-5, 19], [7, 18], [3, 9]].forEach(([dx, dy], i) => dot(x + dx, dy, WHITE, i === bright ? 1 : 0.45));
    if (isNight) glowAt(x, 14, 4, 3, core, 0.15);
  },
  /** A 42 plaque: a brushed-steel plate on standoffs with a big 42 in black, and a small engraved line under it; it catches a glint every 20 s. */
  grok42(x) {
    const steel = 0xa8adb6, steelLight = 0xc8ccd4, steelDark = 0x6e727c, ink = 0x15161a;
    fill(x - 8, 9, 17, 14, steel); fill(x - 8, 9, 17, 1, steelLight); fill(x - 8, 9, 1, 14, steelLight); fill(x + 8, 9, 1, 14, steelDark); fill(x - 8, 22, 17, 1, steelDark);
    fill(x - 7, 23, 17, 1, SHADOW);
    for (const [dx, dy] of [[-7, 10], [7, 10], [-7, 21], [7, 21]]) dot(x + dx, dy, steelDark);
    // The 4 and the 2, twice the font's size
    const big = (rows, left) => rows.forEach((row, r) => [...row].forEach((on, c) => { if (on === '1') fill(left + c * 2, 11 + r * 2, 2, 2, ink); }));
    big(['101', '101', '111', '001', '001'], x - 6); big(['111', '001', '111', '100', '111'], x + 1);
    fill(x - 5, 21, 11, 1, steelDark, 0.6);
    const t = (now + x * 0.21) % 20;
    if (t < 1.5) { const g = x - 8 + Math.floor(t * 12); for (let k = 0; k < 3; k++) if (g + k > x - 8 && g + k < x + 8) dot(g + k, 10 + k * 4, WHITE, 0.7); }
  },
  /** A whiteboard of the universe: orbits, a light cone and a page of sums in black and blue marker; every 30 s a few more marks are added and then wiped clean. */
  grokboard(x) {
    const board = 0xf4f4f0, edge = 0xc8ccd4, edgeDark = 0x8a8e98, ink = 0x2a2c33, blue = 0x4c7be8;
    fill(x - 14, 5, 29, 19, edge); fill(x - 13, 6, 27, 17, board); fill(x - 14, 23, 29, 1, edgeDark); fill(x - 13, 24, 27, 1, SHADOW);
    fill(x - 10, 24, 20, 1, edgeDark); dot(x + 6, 23, blue); fill(x + 7, 23, 2, 1, ink); // the tray and its markers
    // The orbits, left
    ellipseRing(x - 7, 13, 4, 2, ink, 0.9); ellipseRing(x - 7, 13, 6, 4, ink, 0.6); dot(x - 7, 13, blue); dot(x - 3, 12, ink); dot(x - 2, 16, blue);
    // A light cone, middle
    line(x - 1, 8, x + 3, 13, blue); line(x + 7, 8, x + 3, 13, blue); line(x + 3, 13, x - 1, 18, blue); line(x + 3, 13, x + 7, 18, blue);
    // The sums, right
    const step = Math.floor(((now + x * 0.3) % 30) / 3);
    for (let r = 0; r < 4; r++) {
      const n = r < 2 ? 4 : Math.max(0, Math.min(4, step - r * 2));
      for (let c = 0; c < n; c++) dot(x + 9 + c, 9 + r * 3, c % 2 ? blue : ink);
    }
    dot(x + 9, 20, ink); dot(x + 10, 21, ink); dot(x + 11, 20, ink); // a check mark
  },
  /** A window on a launch: a tall pane on a dusky coast with a pad on the horizon; every 60 s a rocket rises off it on a white trail, at night a bright flame. */
  grokwindow(x) {
    const frame = 0x2a2c33, frameLight = 0x3a3d47, sky = isNight ? 0x0e1426 : 0x9ab8d8, skyLow = isNight ? 0x1a2238 : 0xf0c8a0, sea = isNight ? 0x101828 : 0x5a7a98;
    fill(x - 9, 4, 19, 26, frame); fill(x - 9, 4, 19, 1, frameLight); fill(x - 9, 4, 1, 26, frameLight);
    fill(x - 8, 5, 17, 18, sky); fill(x - 8, 17, 17, 6, skyLow); fill(x - 8, 23, 17, 6, sea);
    if (isNight) for (const [dx, dy] of [[-6, 7], [4, 6], [-2, 10], [6, 12]]) dot(x + dx, dy, WHITE, 0.7);
    // The pad and its tower on the far shore
    fill(x + 2, 21, 5, 2, 0x3a3d47); fill(x + 5, 16, 1, 5, 0x3a3d47); dot(x + 5, 15, 0xff6a5a);
    fill(x - 8, 23, 17, 1, isNight ? 0x24304a : 0x7a98b8);
    const t = (now + x * 0.13) % 60;
    if (t < 8) {
      const up = Math.floor(t * t * 0.4), ry = 18 - up;
      for (let k = 1; k <= Math.min(up, 14); k++) if (ry + 2 + k < 23) dot(x + 4, ry + 2 + k, WHITE, 0.8 - k * 0.04);
      if (ry > 4) { fill(x + 4, Math.max(5, ry - 1), 1, Math.min(3, ry - 4), 0xf4f1ea); dot(x + 4, ry + 2, 0xffb648); }
      if (isNight && ry > 4) glowLate(() => { dot(x + 4, ry + 2, 0xfff0c0); glowAt(x + 4, ry + 2, 2, 2, 0xffb648, 0.3); });
    } else fill(x + 4, 18, 1, 3, 0xe8e6e0);
    // The mullion and the sill
    fill(x, 5, 1, 24, frame, 0.6); fill(x - 10, 29, 21, 1, frameLight); fill(x - 10, 30, 21, 1, SHADOW);
  },
  /** A Fun mode switch: a big toggle on a black plate under FUN, its lamp lit white while it's on; every 30 s it flips to off for a few seconds, the lamp dark, then back on. */
  grokfun(x) {
    const plate = 0x15161a, plateEdge = 0x2a2c33, bone = 0xf4f1ea, steel = 0xa8adb6, ion = 0x8ab4ff;
    fill(x - 7, 7, 15, 17, plate); fill(x - 7, 7, 15, 1, plateEdge); fill(x - 7, 7, 1, 17, plateEdge); fill(x - 6, 24, 15, 1, SHADOW);
    if (GROK_NODS) pixelText('FUN', x - 5, 9, bone, false); else fill(x - 4, 11, 9, 1, bone, 0.6);
    const on = (now + x * 0.29) % 30 > 4;
    // The toggle's well, its bat up for on
    fill(x - 2, 16, 5, 6, 0x0a0a0c); fill(x - 1, 18, 3, 2, 0x2a2c33);
    if (on) { fill(x, 15, 1, 4, steel); dot(x, 14, WHITE); } else { fill(x, 19, 1, 4, steel); dot(x, 23, WHITE); }
    // Its lamp
    dot(x + 5, 18, on ? bone : 0x3a3d47);
    if (on && isNight) glowLate(() => { dot(x + 5, 18, WHITE); glowAt(x + 5, 18, 2, 2, ion, 0.25); });
  },
  /** A mission board: three columns on a black board, queued, running and done, each with its cards; every 20 s the running card slides over to done and a queued one takes its place. */
  grokbotmissions(x) {
    const board = 0x15161a, edge = 0x2a2c33, queued = 0x6e727c, running = 0x8ab4ff, done = 0xf4f1ea;
    fill(x - 13, 5, 27, 20, edge); fill(x - 12, 6, 25, 18, board); fill(x - 12, 25, 27, 1, SHADOW);
    // The columns' heads
    [[x - 11, queued], [x - 3, running], [x + 5, done]].forEach(([cx, c]) => { fill(cx, 7, 7, 1, c); fill(cx + 7, 8, 1, 15, 0x24262c); });
    const t = (now + x * 0.17) % 20, moving = t < 2, slide = moving ? Math.floor(t * 4) : 0;
    const card = (cx, cy, c, alpha = 1) => { fill(cx, cy, 6, 3, c, alpha); fill(cx, cy, 6, 1, WHITE, 0.25 * alpha); };
    // Queued: three, the top one lifting away while a card moves
    for (let k = 0; k < 3; k++) card(x - 11, 9 + k * 4, queued, k === 0 && moving ? 0.4 : 1);
    // Running: one, its progress bar filling
    if (!moving) { card(x - 3, 9, running); fill(x - 3, 13, Math.floor(((t - 2) / 18) * 6), 1, running); }
    else card(x - 3 + slide, 9, running, 0.8);
    // Done: a growing pile, capped at four
    const pile = 1 + (Math.floor((now + x * 0.17) / 20) % 4);
    for (let k = 0; k < pile; k++) card(x + 5, 9 + k * 4, done, 0.85);
    if (isNight) glowLate(() => { fill(x - 3, 7, 7, 1, running); if (!moving) dot(x - 3, 10, running); });
  },
  /** A routine clock: a dark dial ringed with ticks, a white sweep going round once a minute, and an ion-blue mark for the next run; when the sweep reaches it the mark flashes and a ring goes out. */
  grokbotroutine(x) {
    const face = 0x15161a, rim = 0x6e727c, rimLight = 0xa8adb6, tick = 0x3a3d47, ion = 0x8ab4ff, cy = 15;
    disc(x, cy, 8.5, rim); disc(x, cy, 7.5, face); ring(x, cy, 8.5, rimLight, 0.5); fill(x - 6, 24, 13, 1, SHADOW);
    for (let k = 0; k < 12; k++) { const a = (k / 12) * Math.PI * 2; dot(x + Math.round(Math.sin(a) * 6), cy - Math.round(Math.cos(a) * 6), k % 3 ? tick : 0x8a8e98); }
    const d = new Date(), sec = d.getSeconds() + d.getMilliseconds() / 1000, mark = 45 + (hash(x, 1) % 10);
    const ma = (mark / 60) * Math.PI * 2, mx = x + Math.round(Math.sin(ma) * 6), my = cy - Math.round(Math.cos(ma) * 6);
    const since = (sec - mark + 60) % 60, firing = since < 2;
    const sa = (sec / 60) * Math.PI * 2;
    line(x, cy, x + Math.round(Math.sin(sa) * 5), cy - Math.round(Math.cos(sa) * 5), 0xf4f1ea);
    dot(x, cy, 0xf4f1ea);
    const lit = () => { fill(mx - 1, my, 3, 1, firing ? WHITE : ion); fill(mx, my - 1, 1, 3, firing ? WHITE : ion); if (firing) ring(x, cy, 8.5 + since * 2, ion, 0.5 - since * 0.25); };
    lit();
    if (isNight) glowLate(lit);
  },
  /** A mailbox for reports: a black wall box with a slot and a little flag; every 40 s a white report drops in through the slot and the flag swings up, staying up a while. */
  grokbotmailbox(x) {
    const box = 0x1a1b20, boxLight = 0x3a3d47, boxDark = 0x0e0f12, slot = 0x050506, flag = 0x8ab4ff;
    fill(x - 6, 10, 12, 14, box); fill(x - 6, 10, 12, 1, boxLight); fill(x - 6, 10, 1, 14, boxLight); fill(x + 5, 10, 1, 14, boxDark);
    fill(x - 5, 9, 10, 1, boxLight); fill(x - 5, 24, 12, 1, SHADOW);
    fill(x - 4, 13, 8, 1, slot); fill(x - 4, 14, 8, 1, boxDark);
    grokMark(x - 1, 19, 0x6e727c);
    const t = (now + x * 0.23) % 40;
    // The report, dropping in
    if (t < 1.2) { const drop = Math.floor(t * 5); fill(x - 3, 6 + drop, 6, Math.max(1, 7 - drop - (drop > 4 ? 2 : 0)), 0xf4f1ea); fill(x - 2, 7 + drop, 4, 1, 0xa8adb6); }
    // The flag on its pivot: up from the drop for 15 s
    fill(x + 6, 17, 1, 2, boxLight);
    if (t > 1 && t < 16) { fill(x + 7, 10, 1, 8, 0x8a8e98); fill(x + 8, 10, 3, 2, flag); if (isNight) glowLate(() => fill(x + 8, 10, 3, 2, flag)); }
    else fill(x + 7, 17, 5, 1, 0x8a8e98);
  },
  /** An uptime screen: a black panel with OK in white over a running ion-blue line of the bots' load and a row of green days; now and then the line spikes and settles. */
  grokbotstatus(x) {
    const bezel = 0x2a2c33, screen = 0x0c0d10, ion = 0x8ab4ff, ok = 0x6fd88a;
    fill(x - 12, 6, 25, 18, bezel); fill(x - 12, 6, 25, 1, 0x3a3d47); fill(x - 11, 7, 23, 16, screen); fill(x - 11, 24, 25, 1, SHADOW);
    const draw = () => {
      pixelText('OK', x - 10, 9, 0xf4f1ea, false);
      for (let k = 0; k < 4; k++) dot(x - 2 + k * 3, 11, ok);
      const shift = Math.floor(now * 2);
      for (let c = 0; c < 21; c++) {
        const h = hash(c + shift, 3), spike = (c + shift) % 37 === 0 ? 4 : 0;
        dot(x - 10 + c, 20 - (h % 3) - spike, ion);
      }
      fill(x - 10, 21, 21, 1, ion, 0.2);
    };
    draw();
    if (isNight) glowLate(() => { glowAt(x, 15, 12, 8, ion, 0.08); draw(); });
  },
  /**
   * The geode vault door (the hero). A great round door cut from a geode, set in a gold ring in the crystal
   * wall: a band of agate round a hollow of violet crystals all pointing in to a glowing heart, a jewel set in the ring
   * for each of the eleven bots, two gold bolts across its seam and two hinges. When a cart of gems comes (gdDelivery)
   * the jewels light one by one, the bolts draw back and the two halves part, light pouring out; by night its heart glows.
   */
  geodevault(x) {
    const cx = x, cy = 22, gold = 0xc8a050, goldLight = 0xf0d080, goldDark = 0x7a5a24;
    gdVaultX = x;
    const { lit, shut, open } = gdDelivery(x, seen.left);
    gdDisc(cx, cy, 19.5, 0x4a3466); gdDisc(cx, cy, 18.5, 0x120818);
    gdDisc(cx, cy, 17.6, goldDark); gdDisc(cx - 0.7, cy - 0.7, 17, goldLight); gdDisc(cx, cy, 16.6, gold);
    for (const hy of [cy - 10, cy + 6]) { fill(cx - 20, hy, 5, 4, gold); fill(cx - 20, hy, 5, 1, goldLight); fill(cx - 20, hy + 3, 5, 1, goldDark); dot(cx - 19, hy + 1, goldDark); }
    const jewels = [0xe02040, 0xf4f4f8, 0x2a4ad8, 0xf09a2a, 0x9a1830, 0xf8c840, 0xfaf6ee, 0xb8d8f8, 0x2a9a5a, 0x5ac87a, 0x2a3ab0];
    const at = (i) => { const a = -Math.PI / 2 + ((i + 0.5) / 11) * Math.PI * 2; return [cx + Math.round(Math.cos(a) * 16.8) - 1, cy + Math.round(Math.sin(a) * 16.8) - 1]; };
    jewels.forEach((c, i) => { const [jx, jy] = at(i); fill(jx, jy, 2, 2, i < lit ? c : 0x3a2c22); dot(jx, jy, i < lit ? shaded(c, 1.4) : goldDark); });
    gdDisc(cx, cy, 15.5, open ? 0xfaf4ff : 0x0e0814);
    const face = () => {
      for (const [r, c] of [[15, 0x3a2c28], [14, 0x6a6274], [13, 0xc8c0d8], [12, 0x8a7aac], [11, 0xb8b0cc], [10, 0x1a0e2c], [6.5, 0x261640], [4, 0x3a2266]]) gdDisc(cx, cy, r, c);
      // The crystals in the hollow, every one pointing in to the heart
      for (let k = 0; k < 12; k++) {
        const a = (k / 12) * Math.PI * 2 + 0.13, ca = Math.cos(a), sa = Math.sin(a), long = k % 2 ? 4 : 2.6, pal = GD_AMETHYST[k % 3];
        const ex = cx + Math.round(ca * 9.4), ey = cy + Math.round(sa * 9.4), tx = cx + Math.round(ca * long), ty = cy + Math.round(sa * long);
        line(ex, ey, tx, ty, pal[1]); line(ex - Math.round(sa), ey + Math.round(ca), tx - Math.round(sa), ty + Math.round(ca), pal[ca + sa > 0 ? 0 : 2]);
        dot(tx, ty, pal[3]);
      }
      gdDisc(cx, cy, 2.2, 0xb090e8); fill(cx - 1, cy - 1, 2, 2, 0xe0ccff); dot(cx - 1, cy - 1, WHITE);
    };
    if (open) {
      within([[cx - 17, cy - 17, 17, 34]], () => alongWall(-open, face));
      within([[cx, cy - 17, 17, 34]], () => alongWall(open, face));
      fill(cx + 2 + open, cy - 13, 2, 2, gold); fill(cx + 2 + open, cy + 11, 2, 2, gold);
    } else {
      face();
      for (let k = -15; k <= 15; k++) if (Math.abs(k) > 9) dot(cx + [0, 0, 1, 0, 0, -1][(k + 30) % 6], cy + k, 0x120818);
      const len = 2 + Math.round(shut * 5);
      for (const by of [cy - 13, cy + 11]) { fill(cx + 4 - len, by, len, 2, gold); fill(cx + 4 - len, by, len, 1, goldLight); }
    }
    gdGlow(cx, cy, 5, 5, 0xc0a0ff, isNight ? 0.12 : 0.08);
    if (open) { glowAt(cx, cy, open, 15, 0xfaf4ff, 0.45); glowAt(cx, cy, 4 + open * 2, 16, 0xe0c8ff, 0.1); }
    if (isNight) glowLate(() => {
      if (!open) { glowAt(cx, cy, 7, 7, 0xb080ff, 0.2); dot(cx - 1, cy - 1, WHITE); dot(cx, cy, 0xe0ccff); }
      jewels.slice(0, lit).forEach((c, i) => { const [jx, jy] = at(i); fill(jx, jy, 2, 2, c); glowAt(jx, jy, 2, 2, c, 0.3); });
      if (open) { glowAt(cx, cy, open, 16, 0xfaf4ff, 0.5); fill(cx - open + 1, cy - 14, open * 2 - 1, 28, 0xfaf4ff, 0.6); glowAt(cx, 46, 12 + open * 4, 5, 0xc8a8ff, 0.22); }
    });
  },
  /** A brass lantern: hung from a bracket on the crystal wall, its flame wavering behind amber glass; its warm light falls on the wall round it, and by night on the floor below. */
  geodelantern(x) {
    const brass = 0xc8a050, brassLight = 0xf0d080, brassDark = 0x7a5a24, f = candlelight(x * 0.13), ly = 10;
    fill(x - 5, 5, 2, 4, brassDark); fill(x - 5, 5, 7, 1, brass); fill(x + 1, 6, 1, 2, brassDark);
    fill(x, ly - 2, 3, 1, brass); dot(x + 1, ly - 3, brassLight);
    fill(x - 1, ly - 1, 5, 1, brassDark); fill(x - 2, ly, 7, 2, brass); fill(x - 2, ly, 7, 1, brassLight);
    fill(x - 2, ly + 2, 7, 8, 0x3a2a1a); fill(x - 1, ly + 2, 5, 8, 0xf0b048, 0.5 + 0.35 * f); fill(x + 1, ly + 2, 1, 8, 0x3a2a1a, 0.6);
    fill(x, ly + 5, 3, 3, 0xffe0a0, f); dot(x + 1, ly + 4, 0xffffff, f); dot(x - 1, ly + 2, WHITE, 0.5);
    fill(x - 2, ly + 10, 7, 2, brass); fill(x - 2, ly + 11, 7, 1, brassDark); fill(x - 1, ly + 12, 5, 1, brassDark);
    gdGlow(x + 1, ly + 6, 11, 10, 0xffc860, (isNight ? 0.16 : 0.07) * f);
    if (isNight) glowLate(() => { gdGlow(x + 1, ly + 6, 15, 13, 0xffc860, 0.13 * f); gdGlow(x + 1, ly + 6, 4, 4, 0xffe8a0, 0.35 * f); fill(x, ly + 5, 3, 3, 0xfff0c0); gdGlow(x + 1, 43, 15, 3, 0xffc860, 0.1 * f); });
  },
  /** An amethyst cluster: a big druse of chunky violet points fanning out of a rock on the wall, tallest in the middle; a point glints now and then, and by night it glows and its tips shine. */
  geodecluster(x) {
    fill(x - 11, 24, 23, 5, 0x3c2852); fill(x - 9, 22, 19, 2, 0x3c2852); fill(x - 11, 24, 23, 1, 0x4c3866); fill(x - 11, 28, 23, 1, 0x1e1229);
    for (const [dx, dy] of [[-7, 26], [2, 27], [7, 25]]) dot(x + dx, dy, 0x5a4480);
    const pts = [[-10, 25, 6, 3, -0.7], [-7, 24, 11, 4, -0.38], [-3, 23, 17, 4, -0.12], [1, 23, 19, 4, 0.06], [5, 23, 13, 4, 0.32], [8, 24, 8, 3, 0.62], [-5, 26, 6, 3, -0.3], [4, 26, 5, 3, 0.3]];
    pts.forEach(([dx, dy, h, w, lean], i) => gdPoint(x + dx, dy, h, w, GD_AMETHYST[i % 3], -1, lean, 0.55));
    const tips = pts.map(([dx, dy, h, w, lean]) => [x + dx + Math.round((h - 1) * lean) + (w >> 1), dy - h + 1]);
    gdTwinkle(tips, 2.7, x);
    gdGlow(x, 14, 10, 10, 0x9a6ae0, isNight ? 0.12 : 0.06);
    if (isNight) glowLate(() => { gdGlow(x, 14, 13, 12, 0xa070f0, 0.15); for (const [tx, ty] of tips) dot(tx, ty, 0xeee0ff); });
  },
  /**
   * The shift bell: a brass bell hung from an iron bracket in the rock, its pull-cord down to the ledge. When the gem cart
   * comes in to the vault (see gdDelivery) it rings: it swings, its clapper knocks, and rings of sound spread from it.
   */
  geodebell(x) {
    const brass = 0xc8a050, brassLight = 0xf0d080, brassDark = 0x7a5a24, iron = 0x3a3640, ironLight = 0x5e5a68;
    // It rings for a little over 2 s as the cart reaches the vault, six seconds into each 50
    const t = isCalm() ? 99 : now % 50, ring = t > 5.4 && t < 7.8 ? t - 5.4 : null;
    const fade = ring === null ? 0 : Math.sqrt(1 - ring / 2.4), swing = ring === null ? 0 : Math.round(Math.sin(ring * 9) * 2.5 * fade);
    // The bracket: a plate in the rock, an arm out over the bell, a scroll under the arm
    fill(x - 10, 3, 3, 10, iron); fill(x - 10, 3, 1, 10, ironLight); dot(x - 9, 5, 0x8a8692); dot(x - 9, 11, 0x8a8692);
    fill(x - 7, 4, 9, 2, iron); fill(x - 7, 4, 9, 1, ironLight); line(x - 7, 8, x - 3, 6, iron); dot(x - 7, 7, iron);
    // The bell, hung from its yoke: each row leans with the swing, the lower the more
    fill(x - 1, 6, 3, 2, brassDark);
    const rows = [2, 3, 3, 3, 4, 4, 4, 5, 6];
    rows.forEach((half, r) => {
      const lean = Math.round((swing * (r + 2)) / rows.length), by = 8 + r, last = r === rows.length - 1;
      fill(x - half + lean, by, half * 2 + 1, 1, last ? brassDark : brass);
      if (!last) { dot(x - half + lean, by, brassLight); dot(x + half + lean, by, brassDark); }
    });
    fill(x - 3 + Math.round(swing * 0.6), 10, 1, 4, brassLight); dot(x - 2 + Math.round(swing * 0.6), 9, WHITE, 0.6);
    // The clapper, and the cord from it down to the ledge
    const cx = x - swing, mouth = 17;
    dot(cx, mouth, 0x5a5a62); dot(cx, mouth + 1, 0x3a3a42);
    line(cx, mouth + 2, x, 26, 0xa08868); fill(x - 1, 26, 3, 2, 0x8a6a48); dot(x, 28, 0x6a5038);
    // The rings of sound, either side, while it rings
    if (ring !== null) {
      const a = 0.9 * fade, beat = Math.floor(ring * 3) % 2;
      for (const side of [-1, 1]) {
        const ox = x + side * (8 + beat);
        for (const [dx, dy] of [[0, 9], [1, 10], [1, 11], [1, 12], [0, 13]]) dot(ox + side * dx, dy, brassLight, a);
        for (const [dx, dy] of [[0, 8], [2, 10], [2, 12], [0, 14]]) dot(ox + side * (dx + 2), dy, brassLight, a * 0.5);
      }
    }
    if (isNight) glowLate(() => dot(x - 2 + Math.round(swing * 0.6), 9, 0xfff0c0, 0.7));
  },
  /**
   * A drip clock: a stalactite drips into a brass-bound glass on the ledge under it, marked in hours, the water standing
   * at the hour of the day. A drop falls every few seconds and rings spread on the water; by night the water glows a little.
   */
  geodedrip(x) {
    const rock = 0x4e3e64, rockLight = 0x6e5a8a, rockDark = 0x2e2240, brass = 0xc8a050, brassLight = 0xf0d080, brassDark = 0x7a5a24;
    // The stalactite, wet at its tip
    [7, 6, 6, 5, 4, 4, 3, 3, 2, 2, 1].forEach((w, r) => { const l = x - (w >> 1); fill(l, r, w, 1, rock); dot(l, r, rockLight); if (w > 2) dot(l + w - 1, r, rockDark); });
    dot(x, 11, 0x9ad0f0, 0.8); dot(x - 1, 4, WHITE, 0.3);
    // The glass, its brass bands and hour marks, standing on the ledge
    const l = x - 4, top = 15, bottom = 28;
    fill(l, top, 9, bottom - top, 0xdff0f6, 0.32);
    fill(l, top, 1, bottom - top, 0xdff0f6, 0.65); fill(l + 8, top, 1, bottom - top, 0xdff0f6, 0.5);
    fill(l - 1, top, 11, 1, brass); fill(l - 1, top, 11, 1, brassLight, 0.5); fill(l - 1, bottom, 11, 2, brass); fill(l - 1, bottom + 1, 11, 1, brassDark);
    for (let k = 0; k <= 12; k++) dot(l + 7, bottom - 1 - k, brassDark, k % 3 === 0 ? 0.9 : 0.45);
    // The water: an hour a mark, from midnight at the bottom
    const d = new Date(), hours = d.getHours() + d.getMinutes() / 60, level = Math.max(1, Math.round((hours / 24) * 12)), surface = bottom - level;
    fill(l + 1, surface, 7, level, 0x5aa0d8, 0.55); fill(l + 1, surface, 7, 1, 0xa8e0ff, 0.8);
    dot(l + 1, top + 2, WHITE, 0.7);
    // A drop now and then, and its rings on the water
    const t = isCalm() ? 9 : (now + x * 0.3) % 3.6;
    if (t < 0.55) dot(x, 12 + Math.round((t / 0.55) * (surface - 13)), 0xb8e4ff);
    else if (t < 1.5) { const k = (t - 0.55) / 0.95; for (const dx of [-1, 1]) dot(x + dx * Math.round(1 + k * 2.5), surface, WHITE, 0.8 * (1 - k)); }
    if (isNight) glowLate(() => { glowAt(x, surface + level / 2, 5, level / 2 + 2, 0x6ac0f0, 0.18); fill(l + 1, surface, 7, 1, 0xc8f0ff, 0.7); });
  },
  /**
   * A window on the deep: a ragged hole broken through the geode's wall, its rim banded and lined with crystal, onto the
   * cavern far below, ledge under ledge of violet rock going down into the dark, crystals on them and a rope ladder.
   * Now and then a miner's lamp crosses far down; by night the deep crystals glow.
   */
  geodedeep(x) {
    const cy = 16, rx = 13, ry = 11;
    const half = (y) => { const k = (y - cy) / (ry + 0.5); return Math.abs(k) >= 1 ? -1 : Math.round(rx * Math.sqrt(1 - k * k)) + ((hash(x, y) % 3) - 1); };
    const rows = [];
    for (let y = cy - ry; y <= cy + ry; y++) { const h = half(y); if (h >= 0) rows.push([y, h]); }
    // The rim: the wall's thickness, banded like the geode's edge
    for (const [y, h] of rows) { fill(x - h - 2, y, h * 2 + 5, 1, 0x5e5666); fill(x - h - 1, y, h * 2 + 3, 1, 0xa8a0bc); }
    // The deep: darker the further down
    for (const [y, h] of rows) fill(x - h, y, h * 2 + 1, 1, y < cy - 4 ? 0x1e1230 : y < cy + 3 ? 0x170d26 : 0x0e0818);
    const clip = rows.map(([y, h]) => [x - h, y, h * 2 + 1, 1]);
    const deep = [];
    within(clip, () => {
      // Ledges going down, each a little darker and smaller
      for (const [lx, ly, w, c] of [[-13, 12, 11, 0x3a2652], [4, 17, 10, 0x2e1e44], [-9, 22, 9, 0x241838], [2, 26, 12, 0x1c122e]]) {
        fill(x + lx, ly, w, 2, c); fill(x + lx, ly, w, 1, shaded(c, 1.35)); fill(x + lx, ly + 2, w, 3, shaded(c, 0.7));
      }
      // Crystals on them
      for (const [dx, y0, h, i] of [[-10, 12, 4, 0], [-8, 12, 3, 2], [7, 17, 3, 1], [9, 17, 2, 0], [-6, 22, 2, 2], [6, 26, 2, 1]]) {
        gdPoint(x + dx, y0 - 1, h, 2, GD_AMETHYST[i], -1); deep.push([x + dx + 1, y0 - h]);
      }
      dot(x + 9, 25, 0x4ab0d0); dot(x + 10, 25, 0x8ae0e8); deep.push([x + 10, 25]);
      // A rope ladder down from the first ledge
      for (const lx of [x - 3, x]) line(lx, 13, lx, 24, 0x6a5038);
      for (let ry2 = 15; ry2 < 24; ry2 += 2) fill(x - 3, ry2, 4, 1, 0x8a6a48);
      // Far down, now and then, a miner's lamp crossing
      const t = isCalm() ? 99 : (now + x * 0.7) % 31;
      if (t < 9) { const lx = x - 7 + Math.round((t / 9) * 14); dot(lx, 25, 0xffd080); glowAt(lx, 25, 3, 2, 0xffc060, 0.3); }
    });
    // Crystal teeth round the top of the rim
    for (const [dx, h] of [[-9, 3], [-6, 4], [-2, 3], [2, 4], [5, 3], [8, 4]]) gdPoint(x + dx, cy - ry + (Math.abs(dx) > 6 ? 2 : 0), h, 2, GD_AMETHYST[(dx + 9) % 3], 1);
    gdTwinkle(deep, 3.3, x);
    if (isNight) glowLate(() => {
      for (const [dx2, dy2] of deep) { glowAt(dx2, dy2 + 1, 3, 2, 0x9a6ae0, 0.25); dot(dx2, dy2, 0xd8c4f8); }
      const t = isCalm() ? 99 : (now + x * 0.7) % 31;
      if (t < 9) { const lx = x - 7 + Math.round((t / 9) * 14); glowAt(lx, 25, 4, 3, 0xffc060, 0.35); dot(lx, 25, 0xffe8b0); }
    });
  },
};
