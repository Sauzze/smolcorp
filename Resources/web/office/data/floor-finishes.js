/** The office's floors besides wood, tiles, concrete and carpet, each from `L`, `T` to `R`, `B`, lined up from the room's corner. */
const FLOOR_FINISHES = {
  /** Deck plating. Riveted plates with a grille every so often. */
  plating(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x50565f);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 16); x < R; x += 16) {
      const c = Math.floor(x / 16);
      if (((c * 3 + r * 5) % 7 + 7) % 7 === 0) { fill(x + 2, y + 2, 13, 9, 0x2a2e36); for (let k = 0; k < 4; k++) fill(x + 3, y + 3 + k * 2, 11, 1, 0x454b55); }
      else for (let k = 0; k < 4; k++) fill(x + 4 + k * 3, y + 4 + (k % 2) * 3, 2, 1, 0x5a606a);
      fill(x, y, 16, 1, 0x3c4149); fill(x, y, 1, 12, 0x3c4149); fill(x + 1, y + 1, 15, 1, 0x5f6670);
      for (const [dx, dy] of [[2, 2], [14, 2], [2, 10], [14, 10]]) dot(x + dx, y + dy, 0x7a828f);
    }
  },
  /** Ship's deck. Wide planks, tarred seams, pegged at their ends. */
  deck(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x8a6440);
    for (let y = T, row = 0; y < B; y += 4, row++) {
      fill(L, y, R - L, 1, 0x3a2616);
      if (row % 2) fill(L, y + 1, R - L, 1, 0x9a7450, 0.5);
      for (let x = from(L, 48, (row % 4) * 12); x < R; x += 48) { fill(x, y, 1, 4, 0x3a2616); dot(x + 2, y + 2, 0x5a3b25); dot(x - 2, y + 2, 0x5a3b25); }
    }
  },
  /** Chequered marble. Black and white marble, veined. */
  marble(L, T, R, B) {
    for (let y = T, r = 0; y < B; y += 10, r++) for (let x = from(L, 10); x < R; x += 10) {
      const c = Math.floor(x / 10), dark = ((c + r) % 2 + 2) % 2 === 1;
      fill(x, y, 10, 10, dark ? 0x3a3540 : 0xe4e0d8);
      const vein = dark ? 0x4e4856 : 0xc8c2b8, k = hash(c, r) % 6;
      for (let s = 0; s < 6; s++) dot(x + ((k + s) % 10), y + s + 2, vein);
    }
  },
  /** Flagstones. Uneven stones, moss in a few gaps. */
  flagstone(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2a2c2a);
    for (let y = T, r = 0; y < B; y += 9, r++) for (let x = from(L, 15, (r % 2) * 7); x < R; x += 15) {
      const h = hash(x, r), w = 13 - (h % 3), tone = [0x5a5d5a, 0x63665f, 0x55585b, 0x5e5a52][(h >>> 3) % 4];
      fill(x + 1, y + 1, w, 7, tone); fill(x + 1, y + 1, w, 1, shaded(tone, 1.15)); fill(x + 1, y + 7, w, 1, shaded(tone, 0.85));
      if (h % 5 === 0) { dot(x + w + 1, y + 3, 0x3b5a3a); dot(x + w + 1, y + 4, 0x4f7a3a); }
    }
  },
  /** Packed snow. Wind lines in the snow, a glint here and there. */
  snow(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xdbe6ef);
    for (let y = T, r = 0; y < B; y += 6, r++) for (let x = from(L, 14, (r % 2) * 7); x < R; x += 14) {
      const h = hash(x, r);
      fill(x + (h % 5), y + 2, 5 + ((h >>> 3) % 4), 1, 0xc0d2e2);
      if ((h >>> 5) % 3 === 0) fill(x + ((h >>> 6) % 7), y + 4, 3, 1, 0xccdae6);
    }
  },
  /** Grass. Tufts and the odd flower. */
  grass(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x4a7a3a);
    for (let y = T, r = 0; y < B; y += 8, r++) for (let x = from(L, 8); x < R; x += 8) {
      const h = hash(x, r);
      if (((Math.floor(x / 8) + r) % 2 + 2) % 2) fill(x, y, 8, 8, 0x467238);
      const tx = x + (h % 6), ty = y + 1 + ((h >>> 2) % 6);
      dot(tx, ty, 0x6a9a4a); dot(tx + 1, ty - 1, 0x6a9a4a); dot(tx + 2, ty, 0x5a8a42);
      if ((h >>> 7) % 9 === 0) dot(x + ((h >>> 3) % 7), y + ((h >>> 1) % 7), h % 2 ? 0xf2d35b : 0xf6b8cc);
    }
  },
  /** Forest floor. Earth, fallen leaves, pine needles and pebbles. */
  forest(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x5a4632);
    for (let y = T, r = 0; y < B; y += 6, r++) for (let x = from(L, 7); x < R; x += 7) {
      const h = hash(x, r);
      if ((h >>> 13) % 5 === 0) fill(x, y, 5, 3, 0x4e3c2a);
      if (h % 3 === 0) fill(x + (h % 5), y + ((h >>> 3) % 5), 2, 1, [0xb8562a, 0xd9893a, 0xe0b040][(h >>> 4) % 3]);
      if ((h >>> 9) % 4 === 0) fill(x + ((h >>> 5) % 6), y + ((h >>> 7) % 5), 2, 1, 0x3b5a3a);
      if ((h >>> 11) % 11 === 0) dot(x + 3, y + 2, 0x8a8f9c);
    }
  },
  /** Rubber gym tiles. Interlocking tiles, flecked. */
  rubber(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x26282d);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 16); x < R; x += 16) {
      fill(x, y, 16, 1, 0x18191c); fill(x, y, 1, 16, 0x18191c);
      fill(x + 6, y, 4, 1, 0x26282d); fill(x + 6, y + 1, 4, 1, 0x18191c);
      fill(x, y + 6, 1, 4, 0x26282d); fill(x + 1, y + 6, 1, 4, 0x18191c);
      for (let k = 0; k < 7; k++) { const h = hash(x + k, r); dot(x + 2 + (h % 13), y + 2 + ((h >>> 4) % 13), (h >>> 8) % 3 ? 0x34373e : 0x3f5a8a); }
    }
  },
  /** Garage epoxy. Flecked epoxy with a yellow line and an oil stain or two. */
  epoxy(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x5f646c);
    for (let y = T, r = 0; y < B; y += 5, r++) for (let x = from(L, 6); x < R; x += 6) { const h = hash(x, r); dot(x + (h % 6), y + ((h >>> 3) % 5), [0x6e737c, 0x52565d, 0x7d828a][(h >>> 6) % 3]); }
    for (let x = from(L, 120, 60); x < R; x += 120) fill(x, T, 2, B - T, 0xe8c04a);
    for (let x = from(L, 120, 20); x < R; x += 120) { const y = T + 14; fill(x, y, 7, 3, 0x3e4148, 0.7); fill(x + 1, y - 1, 5, 5, 0x3e4148, 0.7); dot(x + 2, y, 0x8a7ab0, 0.4); }
    for (let x = from(L, 40, 10); x < R; x += 40) fill(x, T + 3, 1, B - T - 3, WHITE, 0.05);
  },
  /** Haunted court. Old dark boards, cracked, the lines worn pale, the key painted purple. Fog creeps over it: see FLOOR_AIRS. */
  hauntedcourt(L, T, R, B) {
    drawCourt(L, T, R, B, { board: 0x5e4a38, alt: 0x665140, seam: 0x4e3c2e, line: 0xc8d8c0, lineAlpha: 0.5, paint: 0x4a2e62, paintAlpha: 0.6 });
    for (let y = T, r = 0; y < B; y += 9, r++) for (let x = from(L, 13, (r % 2) * 6); x < R; x += 13) {
      const h = hash(x, r);
      if (h % 7 === 0) { dot(x + 2, y + 3, 0x2a1e16); dot(x + 3, y + 4, 0x2a1e16); dot(x + 4, y + 4, 0x2a1e16); }
      if ((h >>> 5) % 9 === 0) dot(x + ((h >>> 9) % 11), y + ((h >>> 3) % 8), 0x8a8f7c);
    }
  },
  /** Hardwood court. Pale maple boards, white lines, the key painted the move's blue. */
  court(L, T, R, B) {
    drawCourt(L, T, R, B, { board: 0xc89a62, alt: 0xcea26b, seam: 0xb68856, line: 0xf5f5f5, lineAlpha: 0.9, paint: 0x3558ce, paintAlpha: 0.85 });
  },
  /** Studio floor. Polished black squares, the lights caught in them, spike tape where people are to stand. */
  studio(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x151515);
    for (let y = T, r = 0; y < B; y += 24, r++) for (let x = from(L, 24); x < R; x += 24) {
      fill(x, y, 24, 1, 0x1f1f1f);
      fill(x, y, 1, 24, 0x1f1f1f);
      const h = hash(x, r);
      if (h % 3 === 0) { line(x + 4, y + 19, x + 13, y + 10, WHITE, 0.05); line(x + 6, y + 19, x + 15, y + 10, WHITE, 0.03); }
      if (h % 17 === 0) {
        const tape = (h >>> 4) % 2 ? 0xf5f5f5 : 0x3558ce, tx = x + 6 + ((h >>> 6) % 10), ty = y + 6 + ((h >>> 9) % 10);
        fill(tx, ty, 3, 1, tape, 0.6);
        dot(tx + 1, ty + 1, tape, 0.6);
      }
    }
  },
  /** Station floor. Light grey tiles with grip strips, a hatch here and there, and a yellow-black stripe along its edge. */
  stationfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xc4c8ce);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 16); x < R; x += 16) {
      const c = Math.floor(x / 16), h = hash(c, r);
      if ((c + r) % 2) fill(x + 1, y + 1, 15, 11, 0xbcc1c8);
      fill(x, y, 16, 1, 0x9aa0a8); fill(x, y, 1, 12, 0x9aa0a8); fill(x + 1, y + 1, 15, 1, 0xdde0e4);
      if (h % 13 === 0) {
        // A hatch, screwed down, its finger recess
        fill(x + 3, y + 3, 10, 7, 0x9aa0a8); fill(x + 4, y + 4, 8, 5, 0xb4b9c1); fill(x + 4, y + 4, 8, 1, 0xd2d6dc);
        fill(x + 7, y + 7, 2, 1, 0x6a7078);
        for (const [dx, dy] of [[4, 4], [11, 4], [4, 8], [11, 8]]) dot(x + dx, y + dy, 0x7a828c);
      } else {
        // Grip strips, on every other tile
        if ((c + r) % 2 === 0) for (const gx of [4, 7, 10]) fill(x + gx, y + 4, 1, 5, 0xa4aab2);
      }
    }
    for (let x = L; x < R; x++) for (const row of [T, T + 1]) dot(x, row, Math.floor((x + row) / 2) % 2 ? PALETTE.hazard : 0x26282d);
  },
  /** Arcade carpet. The dark carpet with neon squiggles every arcade had. */
  arcade(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1c1a33);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 12, (r % 2) * 6); x < R; x += 12) {
      const h = hash(x, r) % 4;
      if (h === 0) for (let k = 0; k < 5; k++) dot(x + 2 + k, y + 3 + (k % 2), 0xe0507a);
      else if (h === 1) { dot(x + 4, y + 6, 0x4ad4e8); dot(x + 5, y + 5, 0x4ad4e8); dot(x + 6, y + 6, 0x4ad4e8); dot(x + 5, y + 7, 0x4ad4e8); }
      else if (h === 2) { dot(x + 3, y + 8, 0xf2d35b); fill(x + 3, y + 9, 2, 1, 0xf2d35b); fill(x + 3, y + 10, 3, 1, 0xf2d35b); }
      else { dot(x + 8, y + 2, 0x9a6ad0); dot(x + 2, y + 9, 0x4ad4e8); }
    }
  },
  /** Raked sand. Pale sand raked in long lines, curving round mossy stones here and there. */
  zensand(L, T, R, B) {
    const sand = 0xd8d0b8, groove = 0xc2b89c, crest = 0xe8e2cf;
    fill(L, T, R - L, B - T, sand);
    for (let y = T + 1; y < B; y += 3) { fill(L, y, R - L, 1, groove); fill(L, y + 1, R - L, 1, crest, 0.6); }
    // The stones, one to a patch of sand, the lines raked round each in rings
    for (let y = T, r = 0; y < B; y += 34, r++) for (let x = from(L, 72, (r % 2) * 36); x < R; x += 72) {
      const h = hash(x, r);
      if (h % 3 === 2) continue;
      const cx = x + 16 + (h % 36), cy = y + 10 + ((h >>> 5) % 14), size = 3 + ((h >>> 9) % 2);
      if (cy + 4 > B) continue;
      for (let row = -(size + 5); row <= size + 5; row++) {
        const half = Math.round(Math.sqrt(Math.max(0, (size + 5) ** 2 - row * row)) * 2);
        fill(cx - half, cy + Math.round(row / 1.4), half * 2 + 1, 1, sand);
      }
      for (const rr of [size + 1.5, size + 3.5]) {
        const steps = Math.ceil(rr * 14);
        for (let k = 0; k < steps; k++) {
          const a = (k / steps) * Math.PI * 2, px = cx + Math.round(Math.cos(a) * rr * 2), py = cy + Math.round(Math.sin(a) * rr / 1.4);
          dot(px, py, groove); dot(px, py + 1, crest, 0.6);
        }
      }
      // The stone, dark grey, moss over its top
      fill(cx - size - 1, cy - 1, size * 2 + 3, 3, 0x6a6e70); fill(cx - size, cy - 2, size * 2 + 1, 1, 0x7a7e80);
      fill(cx - size, cy + 2, size * 2 + 1, 1, 0x4e5254);
      fill(cx - size + 1, cy - 2, size * 2 - 1, 1, 0x5a8a42); dot(cx - 1, cy - 1, 0x6a9a4a); dot(cx + 1, cy - 1, 0x4f7a3a);
      fill(cx - size - 1, cy + 3, size * 2 + 3, 1, 0xb0a688, 0.6);
    }
  },
  /** Rough planks. Wide boards, knotted, their ends nailed; through the odd gap, leaves far below. */
  planks(L, T, R, B) {
    const tones = [0xa07444, 0x96693c, 0xac7e4c, 0x9a6e40];
    fill(L, T, R - L, B - T, 0x5a3b25);
    for (let y = T, row = 0; y < B; y += 7, row++) {
      for (let x = from(L, 42, (row % 3) * 14); x < R; x += 42) {
        const h = hash(x, row + 40), tone = tones[h % 4];
        fill(x, y + 1, 41, 6, tone); fill(x, y + 1, 41, 1, shaded(tone, 1.12)); fill(x, y + 6, 41, 1, shaded(tone, 0.85));
        fill(x + 4 + (h % 20), y + 3, 8, 1, shaded(tone, 0.9));
        if ((h >>> 5) % 3 === 0) { const kx = x + 8 + ((h >>> 7) % 24); fill(kx, y + 3, 3, 2, shaded(tone, 0.7)); dot(kx + 1, y + 3, 0x5a3b25); }
        dot(x + 2, y + 3, 0x5a5048, 0.7); dot(x + 2, y + 5, 0x5a5048, 0.7);
        // A gap now and then, and leaves showing far below through it
        if ((h >>> 11) % 9 === 0) {
          fill(x + 10, y, 22, 2, 0x1e2a18);
          for (let k = 0; k < 6; k++) dot(x + 11 + k * 4 + ((h >>> k) % 2), y + ((h >>> (k + 3)) % 2), k % 2 ? 0x3e6a34 : 0x5a8a42);
        }
      }
    }
  },
  /** Wet flagstones. Dark blue-green slabs, glossy, with puddles that hold a pale reflection, pale seams and a little moss in the cracks. */
  wetflags(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x171f22);
    for (let y = T, r = 0; y < B; y += 9, r++) for (let x = from(L, 15, (r % 2) * 7); x < R; x += 15) {
      const h = hash(x, r), w = 13 - (h % 3), tone = [0x2b3a3c, 0x2f3e40, 0x273436, 0x2d3a3a][(h >>> 3) % 4];
      fill(x + 1, y + 1, w, 7, tone); fill(x + 1, y + 1, w, 1, shaded(tone, 1.25)); fill(x + 1, y + 7, w, 1, shaded(tone, 0.8));
      // A puddle and its glint on some slabs, a streak of wet shine on others
      if (h % 4 === 0) { fill(x + 3 + (h >>> 5) % 4, y + 3, 5, 2, 0x3f6a6a, 0.8); fill(x + 4 + (h >>> 5) % 4, y + 3, 2, 1, 0x8ac8c0, 0.6); }
      else if (h % 4 === 1) { dot(x + 3, y + 2, 0x6a9a98, 0.5); dot(x + 4, y + 3, 0x6a9a98, 0.35); }
      if (h % 5 === 0) { dot(x + w + 1, y + 3, 0x27493a); dot(x + w + 1, y + 4, 0x3a6a4a); }
    }
  },
  /** Polished concrete. Smooth grey slabs with fine saw-cut joints, flecks of aggregate and long soft sheens where the ceiling lights catch it. */
  polished(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x7e8188);
    // Sheens: wide pale bands down the floor, fading at their edges
    for (let x = from(L, 64, 18); x < R; x += 64) { fill(x, T, 10, B - T, WHITE, 0.05); fill(x + 2, T, 6, B - T, WHITE, 0.05); fill(x + 4, T, 2, B - T, WHITE, 0.04); }
    // Flecks of aggregate
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 7); x < R; x += 7) {
      const h = hash(x, r);
      if (h % 3 === 0) dot(x + ((h >>> 3) % 7), y + ((h >>> 6) % 4), [0x8c9097, 0x72757c, 0x969aa1][(h >>> 9) % 3]);
    }
    // Saw-cut joints every 48 across and 32 down
    for (let x = from(L, 48); x < R; x += 48) fill(x, T, 1, B - T, 0x6a6d74);
    for (let y = T + 31; y < B; y += 32) fill(L, y, R - L, 1, 0x6a6d74);
  },
  /** Emerald and gold tiles. Glossy green tiles in a few shades, set in dark green grout, a small gold diamond at every fourth crossing. */
  emeraldgold(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x123e28);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 12); x < R; x += 12) {
      const h = hash(x, r), tone = [0x1f6a46, 0x23744c, 0x1b6040, 0x267a50][h % 4];
      fill(x + 1, y + 1, 11, 11, tone); fill(x + 1, y + 1, 11, 1, shaded(tone, 1.15)); fill(x + 1, y + 11, 11, 1, shaded(tone, 0.85));
      if ((h >>> 4) % 5 === 0) { dot(x + 3, y + 3, WHITE, 0.22); dot(x + 4, y + 3, WHITE, 0.12); }
      if (Math.floor(x / 12) % 2 === 0 && r % 2 === 0) { dot(x, y - 1, 0xb08a3a); dot(x - 1, y, 0xb08a3a); dot(x + 1, y, 0xb08a3a); dot(x, y + 1, 0xb08a3a); dot(x, y, 0xf4d878); }
    }
  },
  /** Worn carpet. Dark grey-brown office carpet in a small tile pattern, worn to a paler path here and there, with a few old stains. */
  worncarpet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x3e3a3a);
    for (let y = T, r = 0; y < B; y += 8, r++) for (let x = from(L, 8); x < R; x += 8) {
      const h = hash(x, r);
      if ((Math.floor(x / 8) + r) % 2 === 0) fill(x, y, 8, 8, 0x423e3d);
      dot(x + (h % 8), y + ((h >>> 3) % 8), 0x4a4645);
      if ((h >>> 6) % 4 === 0) dot(x + ((h >>> 9) % 8), y + ((h >>> 12) % 8), 0x343030);
    }
    // Worn patches, paler, and old stains
    for (let y = T + 10, r = 0; y < B; y += 40, r++) for (let x = from(L, 90, (r % 2) * 45); x < R; x += 90) {
      const h = hash(x, r + 50);
      fill(x, y, 18, 6, 0x5a5452, 0.35); fill(x + 3, y - 1, 12, 8, 0x5a5452, 0.25);
      if (h % 2) { fill(x + 30, y + 14, 5, 3, 0x2a2420, 0.5); fill(x + 31, y + 13, 3, 5, 0x2a2420, 0.4); }
    }
  },
  /** A floor of cables. Dark vinyl planks, and here and there a power strip with its little green light and a few cables, black, blue and red, curling away from it. */
  cablefloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2a2c32);
    for (let y = T, r = 0; y < B; y += 6, r++) {
      fill(L, y, R - L, 1, 0x26282d);
      for (let x = from(L, 40, (r % 3) * 13); x < R; x += 40) fill(x, y, 1, 6, 0x26282d);
    }
    // Power strips, each with three short cables curling off it
    for (let y = T + 22, r = 0; y < B - 6; y += 64, r++) for (let x = from(L, 120, (r % 2) * 60); x < R; x += 120) {
      const runs = [[0x16171b, [[2, 3], [2, 6], [0, 8], [-4, 9], [-8, 9]]], [0x2a4a8a, [[5, 3], [5, 6], [7, 8], [11, 9], [15, 8]]],
        [0x8a2a2a, [[8, 3], [10, 4], [13, 4], [16, 2], [18, 2]]]];
      for (const [colour, path] of runs) for (let k = 0; k < path.length - 1; k++) line(x + path[k][0], y + path[k][1], x + path[k + 1][0], y + path[k + 1][1], colour);
      fill(x, y, 10, 3, 0xd8d4ca); fill(x, y + 2, 10, 1, 0xa8a296); for (const k of [2, 5]) dot(x + k, y + 1, 0x2a2c32); dot(x + 8, y + 1, 0x3ad87a);
    }
  },
  /** A pitch. Grass cut in wide light and dark stripes, speckled with blades, and now and then a white touchline with a centre circle. */
  pitchgrass(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x3a8836);
    for (let x = from(L, 24), i = Math.floor(from(L, 24) / 24); x < R; x += 24, i++) if (i % 2 === 0) fill(x, T, 12, B - T, 0x3f9039);
    for (let y = T, r = 0; y < B; y += 9, r++) for (let x = from(L, 18, (r % 2) * 9); x < R; x += 18) {
      const h = hash(x, r);
      if (h % 3 === 0) dot(x + (h >>> 3) % 9, y + (h >>> 7) % 9, 0x2e7a2e, 0.8);
      if (h % 7 === 1) dot(x + (h >>> 5) % 9, y + (h >>> 9) % 9, 0x52a84a, 0.8);
    }
    // The lines: a halfway line with a centre circle, and a goal-line cross every so often
    for (let x = from(L, 192, 48); x < R; x += 192) { fill(x, T, 1, B - T, WHITE, 0.55); ring(x, T + Math.floor((B - T) / 2), 14, WHITE, 0.5); disc(x, T + Math.floor((B - T) / 2), 1, WHITE, 0.6); }
  },
  /** Lakers court (brand nod: Lakers). The hardwood in maple planks, a centre circle in purple ringed in gold with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  lakerscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.lakers, L, T, R, B); },
  /** Celtics court (brand nod: Celtics). The hardwood in parquet squares, a centre circle in green ringed in white with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  celticscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.celtics, L, T, R, B); },
  /** Warriors court (brand nod: Warriors). The hardwood in maple planks, a centre circle in blue ringed in yellow with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  warriorscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.warriors, L, T, R, B); },
  /** Knicks court (brand nod: Knicks). The hardwood in maple planks, a centre circle in blue ringed in orange with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  knickscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.knicks, L, T, R, B); },
  /** Bulls court (brand nod: Bulls). The hardwood in maple planks, a centre circle in red ringed in black with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  bullscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.bulls, L, T, R, B); },
  /** Heat court (brand nod: Heat). The hardwood in maple planks, a centre circle in red ringed in orange with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  heatcourt(L, T, R, B) { nbaCourt(NBA_TEAMS.heat, L, T, R, B); },
  /** Spurs court (brand nod: Spurs). The hardwood in maple planks, a centre circle in black ringed in silver with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  spurscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.spurs, L, T, R, B); },
  /** Thunder court (brand nod: Thunder). The hardwood in maple planks, a centre circle in blue ringed in orange with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  thundercourt(L, T, R, B) { nbaCourt(NBA_TEAMS.thunder, L, T, R, B); },
  /** Timberwolves court (brand nod: Timberwolves). The hardwood in maple planks, a centre circle in navy ringed in green with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  wolvescourt(L, T, R, B) { nbaCourt(NBA_TEAMS.wolves, L, T, R, B); },
  /** Nuggets court (brand nod: Nuggets). The hardwood in maple planks, a centre circle in navy ringed in gold with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  nuggetscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.nuggets, L, T, R, B); },
  /** Hawks court (brand nod: Hawks). The hardwood in maple planks, a centre circle in red ringed in volt with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  hawkscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.hawks, L, T, R, B); },
  /** Nets court (brand nod: Nets). The hardwood in maple planks, a centre circle in black ringed in white with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  netscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.nets, L, T, R, B); },
  /** Hornets court (brand nod: Hornets). The hardwood in maple planks, a centre circle in teal ringed in purple with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  hornetscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.hornets, L, T, R, B); },
  /** Cavaliers court (brand nod: Cavaliers). The hardwood in maple planks, a centre circle in wine ringed in gold with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  cavscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.cavs, L, T, R, B); },
  /** Mavericks court (brand nod: Mavericks). The hardwood in maple planks, a centre circle in blue ringed in silver with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  mavscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.mavs, L, T, R, B); },
  /** Pistons court (brand nod: Pistons). The hardwood in maple planks, a centre circle in blue ringed in red with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  pistonscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.pistons, L, T, R, B); },
  /** Rockets court (brand nod: Rockets). The hardwood in maple planks, a centre circle in red ringed in silver with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  rocketscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.rockets, L, T, R, B); },
  /** Pacers court (brand nod: Pacers). The hardwood in maple planks, a centre circle in navy ringed in gold with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  pacerscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.pacers, L, T, R, B); },
  /** Clippers court (brand nod: Clippers). The hardwood in maple planks, a centre circle in red ringed in blue with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  clipperscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.clippers, L, T, R, B); },
  /** Grizzlies court (brand nod: Grizzlies). The hardwood in maple planks, a centre circle in navy ringed in blue with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  grizzliescourt(L, T, R, B) { nbaCourt(NBA_TEAMS.grizzlies, L, T, R, B); },
  /** Bucks court (brand nod: Bucks). The hardwood in maple planks, a centre circle in green ringed in cream with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  buckscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.bucks, L, T, R, B); },
  /** Pelicans court (brand nod: Pelicans). The hardwood in maple planks, a centre circle in navy ringed in red with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  pelicanscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.pelicans, L, T, R, B); },
  /** Magic court (brand nod: Magic). The hardwood in maple planks, a centre circle in blue ringed in black with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  magiccourt(L, T, R, B) { nbaCourt(NBA_TEAMS.magic, L, T, R, B); },
  /** 76ers court (brand nod: 76ers). The hardwood in maple planks, a centre circle in blue ringed in red with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  sixerscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.sixers, L, T, R, B); },
  /** Suns court (brand nod: Suns). The hardwood in maple planks, a centre circle in purple ringed in orange with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  sunscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.suns, L, T, R, B); },
  /** Trail Blazers court (brand nod: Trail Blazers). The hardwood in maple planks, a centre circle in red ringed in black with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  blazerscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.blazers, L, T, R, B); },
  /** Kings court (brand nod: Kings). The hardwood in maple planks, a centre circle in purple ringed in silver with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  kingscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.kings, L, T, R, B); },
  /** Raptors court (brand nod: Raptors). The hardwood in maple planks, a centre circle in red ringed in black with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  raptorscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.raptors, L, T, R, B); },
  /** Jazz court (brand nod: Jazz). The hardwood in maple planks, a centre circle in navy ringed in yellow with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  jazzcourt(L, T, R, B) { nbaCourt(NBA_TEAMS.jazz, L, T, R, B); },
  /** Wizards court (brand nod: Wizards). The hardwood in maple planks, a centre circle in navy ringed in red with the badge at its heart, and a painted lane and free-throw arc at each baseline. */
  wizardscourt(L, T, R, B) { nbaCourt(NBA_TEAMS.wizards, L, T, R, B); },
  /** Avengers Tower floor. Polished blue-grey stone in a soft checker of big tiles with silver joints and pale flecks, and now and then the Avengers' A set into it in silver (brand nod: Marvel). */
  marvel1tower(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x3c4452);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 16); x < R; x += 16) {
      if ((Math.floor(x / 16) + r) % 2) fill(x, y, 16, Math.min(16, B - y), 0x424b5a);
      fill(x, y, 16, 1, 0x56606e); fill(x, y, 1, Math.min(16, B - y), 0x56606e, 0.6);
      const h = hash(x, y);
      if (h % 5 === 0) dot(x + 3 + (h >>> 3) % 10, y + 3 + (h >>> 7) % 10, 0x8a96a8, 0.5);
      if (h % 11 === 1) line(x + 4, y + 12, x + 8, y + 8, WHITE, 0.08);
    }
    for (let y = T + 24, r = 0; y + 6 < B; y += 64, r++) for (let x = from(L, 128, (r % 2) * 64) + 56; x < R; x += 128) m1A(x, y, 0x9aa6b8, 0.5);
  },
  /** Star-spangled carpet. A deep navy pile scattered with small white stars in staggered rows, edged top and bottom with red and white stripes. */
  marvel1stars(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1c2e5e);
    for (let y = T + 8, r = 0; y < B - 8; y += 12, r++) for (let x = from(L, 16, (r % 2) * 8) + 4; x < R; x += 16) {
      dot(x, y - 1, M1_CREAM, 0.85); fill(x - 1, y, 3, 1, M1_CREAM, 0.85); dot(x - 1, y + 1, M1_CREAM, 0.7); dot(x + 1, y + 1, M1_CREAM, 0.7);
    }
    for (const [y, c] of [[T, M1_RED], [T + 2, M1_CREAM], [T + 3, M1_RED], [B - 4, M1_RED], [B - 3, M1_CREAM], [B - 2, M1_RED]]) fill(L, y, R - L, y === T ? 2 : 1, c, 0.9);
  },
  /** Vibranium floor. Dark slate laid in big tiles crossed by faint violet lines in triangles, as Wakanda's labs are, with a lit node here and there where the lines meet. */
  marvel2vibranium(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1e1a28);
    for (let y = T, r = 0; y < B; y += 24, r++) for (let x = from(L, 24); x < R; x += 24) {
      if ((Math.floor(x / 24) + r) % 2) fill(x, y, 24, Math.min(24, B - y), 0x221d2e);
      fill(x, y, 24, 1, 0x2e2840);
      line(x, y + 23, x + 12, y + 1, M2_PURPLE, 0.22); line(x + 12, y + 1, x + 23, y + 23, M2_PURPLE, 0.22);
      const h = hash(x, y);
      if (h % 4 === 0) { dot(x + 12, y + 1, M2_VIBE, 0.7); dot(x + 12, y, M2_VIBE, 0.25); }
      if (h % 7 === 2) dot(x + 3 + (h >>> 4) % 18, y + 6 + (h >>> 9) % 14, 0x4a4060, 0.6);
    }
  },
  /** Cosmic carpet. Deep space: a dark violet pile with soft purple and teal nebulae, scattered stars and a few four-pointed bright ones. */
  marvel2cosmos(L, T, R, B) {
    fill(L, T, R - L, B - T, M2_SPACE);
    for (let y = T, r = 0; y < B; y += 40, r++) for (let x = from(L, 56, (r % 2) * 28); x < R; x += 56) {
      const h = hash(x, y);
      if (h % 3 === 0) { disc(x + 20, y + 20, 11, M2_PURPLE_DARK, 0.45); disc(x + 23, y + 18, 6, M2_PURPLE, 0.2); }
      else if (h % 3 === 1) { disc(x + 18, y + 22, 9, 0x123a4a, 0.5); disc(x + 15, y + 22, 4, 0x2a7a8a, 0.25); }
    }
    for (let y = T, r = 0; y < B; y += 7, r++) for (let x = from(L, 9, (r % 3) * 3); x < R; x += 9) {
      const h = hash(x, y + 3);
      if (h % 5 === 0) dot(x + (h >>> 3) % 9, y + (h >>> 7) % 7, WHITE, 0.35 + ((h >>> 11) % 5) / 10);
      if (h % 61 === 7) { const sx = x + 4, sy = y + 3; dot(sx, sy, WHITE); dot(sx - 1, sy, 0xd8e8ff, 0.6); dot(sx + 1, sy, 0xd8e8ff, 0.6); dot(sx, sy - 1, 0xd8e8ff, 0.6); dot(sx, sy + 1, 0xd8e8ff, 0.6); }
    }
  },
  /** Workshop floor. Dark sealed concrete with a soft sheen in long bays, oil spots and scuffs here and there, and a yellow and black hazard stripe marking a bay's edge every so often. */
  ironmangarage(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2a2c32);
    for (let y = T; y < B; y += 32) fill(L, y, R - L, 1, 0x34363e);
    for (let x = from(L, 48); x < R; x += 48) fill(x, T, 1, B - T, 0x24262c);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 16, (r % 2) * 8); x < R; x += 16) {
      const h = hash(x, y);
      if (h % 9 === 0) { fill(x + (h >>> 3) % 10, y + (h >>> 7) % 8, 3, 2, 0x1c1d22, 0.7); }
      if (h % 13 === 3) line(x + 2, y + 6, x + 8, y + 5, 0x3e4048, 0.6);
    }
    for (let y = T + 60; y < B; y += 96) for (let x = from(L, 8); x < R; x += 8) { fill(x, y, 4, 3, 0xf2c030); fill(x + 4, y, 4, 3, 0x1a1a1f); }
  },
  /** Hex glass floor. Dark smoked glass laid in hexagonal tiles, their edges a faint arc-reactor blue, a glint caught in some. */
  ironmanhexglass(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x161a22);
    for (let y = T - 6, r = 0; y < B; y += 9, r++) for (let x = from(L, 16, (r % 2) * 8); x < R; x += 16) {
      const c = IM_HOLO;
      line(x, y + 3, x + 4, y, c, 0.22); line(x + 4, y, x + 8, y + 3, c, 0.22); fill(x, y + 3, 1, 4, c, 0.22); fill(x + 8, y + 3, 1, 4, c, 0.18);
      const h = hash(x, y);
      if (h % 6 === 0) { fill(x + 2, y + 3, 5, 4, 0x1e2430); dot(x + 3, y + 3, WHITE, 0.25); }
    }
  },
  /** Batcave steel deck. Dark steel plates over the cave, a diamond tread pattern, bolt heads at their corners and a yellow safety line along every few rows. */
  batcavedeck(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x24262c);
    for (let y = T; y < B; y += 20) {
      fill(L, y, R - L, 1, 0x32343c); fill(L, y + 19, R - L, 1, 0x1a1b20);
      for (let x = from(L, 30); x < R; x += 30) {
        fill(x, y, 1, 20, 0x1a1b20); fill(x + 1, y, 1, 20, 0x30323a);
        for (const [bx, by] of [[3, 3], [26, 3], [3, 16], [26, 16]]) dot(x + bx, y + by, 0x4a4c54);
        for (let ty = 5; ty < 16; ty += 4) for (let tx = 6; tx < 26; tx += 5) dot(x + tx + ((ty >> 2) % 2) * 2, y + ty, 0x30323a);
      }
    }
    for (let y = T + 59; y < B; y += 80) fill(L, y, R - L, 1, BC_YELLOW, 0.55);
  },
  /** Cave rock floor. Rough dark rock in uneven slabs with cracks between them, pebbles, and small still pools that catch a pale glint. */
  batcaverock(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2a2b30);
    for (let y = T, r = 0; y < B; y += 14, r++) for (let x = from(L, 22, (r % 2) * 11); x < R; x += 22) {
      const h = hash(x, y), c = [0x2e2f35, 0x26272c, 0x32333a][h % 3];
      fill(x + 1, y + 1, 20, 12, c); fill(x + 1, y + 1, 20, 1, 0x3a3b42, 0.7); fill(x, y + 13, 22, 1, 0x1a1b1f);
      if (h % 5 === 0) dot(x + 4 + (h >>> 4) % 14, y + 4 + (h >>> 9) % 7, 0x4a4c54);
      if (h % 11 === 3) { fill(x + 6, y + 5, 7, 3, 0x1e2a36); fill(x + 7, y + 5, 4, 1, 0x3a5068); dot(x + 8, y + 6, WHITE, 0.4); }
    }
  },
  /** Watchtower floor. Silver-blue deck panels, the Justice League's satellite, with thin blue-lit seams between them and a round lit node at every other crossing. */
  dcmixwatchtower(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xa8b2c4);
    for (let y = T, r = 0; y < B; y += 20, r++) for (let x = from(L, 32); x < R; x += 32) {
      if ((Math.floor(x / 32) + r) % 2) fill(x + 1, y + 1, 31, 19, 0xb0bacb);
      fill(x, y, 32, 1, 0x8ab0e8, 0.7); fill(x, y, 1, 20, 0x8ab0e8, 0.6); fill(x + 1, y + 1, 31, 1, 0xd8dee8, 0.6);
      if ((Math.floor(x / 32) + r) % 2 === 0) { dot(x, y, 0x4a8af0); dot(x + 1, y, 0x9ac0ff, 0.7); }
      const h = hash(x, y); if (h % 7 === 0) line(x + 6, y + 14, x + 12, y + 8, WHITE, 0.4);
    }
  },
  /** Themysciran marble. Cream marble flagged in big squares with soft grey veins, edged with a gold Greek key border along the top. */
  dcmixthemyscira(L, T, R, B) {
    fill(L, T, R - L, B - T, DC_MARBLE);
    for (let y = T + 8, r = 0; y < B; y += 24, r++) for (let x = from(L, 24, (r % 2) * 12); x < R; x += 24) {
      fill(x, y, 24, 1, DC_MARBLE_DARK, 0.6); fill(x, y, 1, 24, DC_MARBLE_DARK, 0.5);
      const h = hash(x, y);
      line(x + 3 + h % 8, y + 3, x + 10 + h % 9, y + 18, 0xb8b0a0, 0.25); if (h % 3 === 0) line(x + 14, y + 6, x + 20, y + 10, 0xb8b0a0, 0.25);
    }
    // The Greek key along the top edge
    fill(L, T, R - L, 7, 0xd8c890);
    for (let x = from(L, 8); x < R; x += 8) {
      fill(x, T + 1, 6, 1, DC_GOLD); fill(x + 5, T + 1, 1, 4, DC_GOLD); fill(x + 2, T + 4, 4, 1, DC_GOLD); fill(x + 2, T + 3, 1, 1, DC_GOLD); fill(x, T + 1, 1, 5, DC_GOLD); fill(x, T + 5, 8, 1, DC_GOLD);
    }
    fill(L, T + 7, R - L, 1, DC_GOLD_DARK, 0.6);
  },
  /** Pokémon Center tiles. Big cream tiles on fine grout, every other one a shade warmer, and here and there a tile inlaid with a red and white Poké Ball. */
  pokemontiles(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xd6caae);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 12); x < R; x += 12) {
      const c = Math.floor(x / 12);
      fill(x, y, 11, 11, (c + r) % 2 ? 0xf2ead8 : 0xeae0ca); fill(x, y, 11, 1, 0xf8f2e6); fill(x, y + 10, 11, 1, 0xe0d4bc);
      if (hash(c, r) % 31 === 0) { ring(x + 5, y + 5, 3.8, 0xd6caae, 0.8); pokeBall(x + 2, y + 2, 7, PKM_RED, 0.45); }
    }
  },
  /** Route grass. Bright short grass in a soft checker of tufts, with clumps of tall grass here and there; every 9 s one clump rustles, as if something wild were in it. */
  pokemongrass(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x5cb848);
    for (let y = T, r = 0; y < B; y += 8, r++) for (let x = from(L, 8); x < R; x += 8) {
      if ((Math.floor(x / 8) + r) % 2) continue;
      dot(x + 2, y + 3, 0x74cc5c); dot(x + 3, y + 2, 0x74cc5c); dot(x + 5, y + 5, 0x48a038); dot(x + 6, y + 4, 0x48a038);
    }
    // The tall grass: clumps of six tufts on a 24-px grid
    const round = Math.floor(now / 9), rustling = now % 9 < 1.5, sway = Math.floor(now * 8) % 2 ? 1 : -1;
    for (let y = T + 2, r = 0; y < B - 12; y += 24, r++) for (let x = from(L, 24); x < R; x += 24) {
      const h = hash(Math.floor(x / 24), r + 40);
      if (h % 4 !== 0) continue;
      const s = rustling && (h >>> 3) % 3 === round % 3 ? sway : 0;
      for (let i = 0; i < 3; i++) for (let j = 0; j < 2; j++) {
        const tx = x + 3 + i * 5 + j * 2, ty = y + 4 + j * 5;
        fill(tx, ty + 1, 4, 3, 0x3a8a30); fill(tx, ty + 3, 4, 1, 0x2e7426);
        dot(tx + s, ty, 0x2e7426); dot(tx + 2 + s, ty, 0x2e7426); dot(tx + 1 + s, ty + 1, 0x58b048); dot(tx + 3 + s, ty + 1, 0x58b048);
      }
    }
  },
  /** Hextech tiles. Big deep-navy stone tiles on dark grout, a small gold stud at every corner, and here and there a tile with a faint blue hextech rune. */
  lolhextiles(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x0c1424);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 16); x < R; x += 16) {
      const c = Math.floor(x / 16);
      fill(x + 1, y + 1, 15, 15, (c + r) % 2 ? 0x16213a : 0x1a2744); fill(x + 1, y + 1, 15, 1, 0x24345a);
      dot(x, y, LOL_GOLD, 0.75);
      if (hash(c, r) % 9 === 0) { line(x + 8, y + 4, x + 12, y + 8, LOL_HEX, 0.25); line(x + 12, y + 8, x + 8, y + 12, LOL_HEX, 0.25); line(x + 8, y + 12, x + 4, y + 8, LOL_HEX, 0.25); line(x + 4, y + 8, x + 8, y + 4, LOL_HEX, 0.25); dot(x + 8, y + 8, LOL_HEX_LIGHT, 0.35); }
    }
  },
  /** Summoner's Rift. Deep green grass, mottled and tufted, with worn flagstones of the lane set into it here and there. */
  lolrift(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x4a7a3a);
    for (let y = T, r = 0; y < B; y += 10, r++) for (let x = from(L, 10); x < R; x += 10) {
      const h = hash(Math.floor(x / 10), r);
      if (h % 3 === 0) fill(x + h % 6, y + (h >>> 3) % 6, 3, 2, 0x55883f);
      if (h % 5 === 1) { dot(x + (h >>> 5) % 8, y + (h >>> 7) % 8, 0x3a6a2e); dot(x + 1 + (h >>> 5) % 8, y - 1 + (h >>> 7) % 8, 0x3a6a2e); }
    }
    for (let y = T + 3, r = 0; y < B - 6; y += 13, r++) for (let x = from(L, 20); x < R; x += 20) {
      const h = hash(Math.floor(x / 20), r + 70);
      if (h % 4) continue;
      const sx = x + h % 6;
      fill(sx, y, 11, 6, 0x9a9070); fill(sx, y, 11, 1, 0xb8ae8a); fill(sx, y + 5, 11, 1, 0x7a7058); fill(sx + 5, y + 1, 1, 4, 0x7a7058);
    }
  },
  /** Inn floorboards. Long warm planks with dark seams, their ends staggered and nailed, one a shade lighter or darker than the next. */
  wowinn(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x5e3a20);
    for (let y = T, r = 0; y < B; y += 6, r++) {
      const off = (r * 17) % 40;
      for (let x = from(L, 40, off); x < R; x += 40) {
        const h = hash(Math.floor((x - off) / 40), r), c = [0x8a5a32, 0x9a6a3a, 0x7e5230][h % 3];
        fill(x, y, 39, 5, c); fill(x, y, 39, 1, shaded(c, 1.12));
        dot(x + 1, y + 2, 0x4a2c18);
        if (h % 4 === 0) fill(x + 8 + h % 20, y + 3, 6, 1, shaded(c, 0.85));
      }
    }
  },
  /** Stormwind cobbles. Rounded blue-grey stones set in staggered rows, each lit on top, in dark joints. */
  wowcobble(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x585c64);
    for (let y = T, r = 0; y < B; y += 6, r++) for (let x = from(L, 8, (r % 2) * 4); x < R; x += 8) {
      const h = hash(Math.floor(x / 4), r), c = [0x7a7e88, 0x8a8e98, 0x6e727c, 0x80786e][h % 4];
      fill(x + 1, y + 1, 6, 4, c); fill(x + 2, y, 4, 1, c); fill(x + 2, y + 1, 4, 1, shaded(c, 1.15)); fill(x + 1, y + 4, 6, 1, shaded(c, 0.82));
    }
  },
  /** Vice City terrazzo. Pale pink and white tiles flecked with colour, a teal inlay line between every few rows. */
  gtavice(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xe8c0cc);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 12); x < R; x += 12) {
      const c = Math.floor(x / 12), h = hash(c, r);
      fill(x, y, 11, 11, (c + r) % 2 ? 0xf8e2e8 : 0xf6f2ec);
      for (let k = 0; k < 3; k++) dot(x + (h >>> (k * 4)) % 10, y + (h >>> (k * 4 + 2)) % 10, [GTA_PINK, GTA_TEAL, 0xc8c0b8][k], 0.55);
      if (r % 4 === 3) fill(x, y + 11, 12, 1, GTA_TEAL, 0.8);
    }
  },
  /** Street asphalt. Dark grey tarmac with grit, cracks here and there, a dashed yellow line down every lane and the odd manhole cover. */
  gtaasphalt(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x3a3c40);
    for (let y = T, r = 0; y < B; y += 6, r++) for (let x = from(L, 6); x < R; x += 6) {
      const h = hash(Math.floor(x / 6), r);
      if (h % 3 === 0) dot(x + h % 5, y + (h >>> 4) % 5, 0x4a4c52); if (h % 7 === 0) dot(x + (h >>> 6) % 5, y + (h >>> 9) % 5, 0x2e3034);
    }
    for (let y = T + 20; y < B; y += 48) for (let x = from(L, 16); x < R; x += 16) fill(x, y, 9, 2, 0xe8c030);
    for (let y = T, r = 0; y < B; y += 40, r++) for (let x = from(L, 64); x < R; x += 64) {
      const h = hash(Math.floor(x / 64), r + 30);
      if (h % 3 === 0) { const mx = x + 10 + h % 40, my = y + 6 + (h >>> 5) % 20; disc(mx, my, 4, 0x55585e); ring(mx, my, 4.2, 0x2a2c30); fill(mx - 2, my, 5, 1, 0x3a3c40); fill(mx, my - 2, 1, 5, 0x3a3c40); }
      if (h % 4 === 1) { const cx = x + h % 50, cy = y + 30; line(cx, cy, cx + 4, cy + 2, 0x26282c); line(cx + 4, cy + 2, cx + 7, cy + 1, 0x26282c); }
    }
  },
  /** Tower plaza stone. Long pale stone slabs in staggered courses, a gold inlay line every few rows and the odd teal light set into the floor. */
  destinytower(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xaeaaa2);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 24, (r % 2) * 12); x < R; x += 24) {
      const h = hash(Math.floor(x / 12), r);
      fill(x, y, 23, 11, h % 2 ? 0xd0ccc4 : 0xc8c4bc); fill(x, y, 23, 1, 0xdcd8d0);
      if (h % 17 === 0) { dot(x + 11, y + 5, DST_TEAL); dot(x + 12, y + 5, 0x9af0f0, 0.6); }
      if (r % 3 === 2) fill(x, y + 11, 24, 1, DST_GOLD, 0.7);
    }
  },
  /** Cosmodrome deck plates. Rusted red-brown steel plates, riveted at the corners, streaked with rust, a dusting of snow drifted against their seams. */
  destinyrust(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x4a3628);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 16); x < R; x += 16) {
      const h = hash(Math.floor(x / 16), r), c = [0x7a5a46, 0x6e5240, 0x846250][h % 3];
      fill(x + 1, y + 1, 15, 15, c); fill(x + 1, y + 1, 15, 1, shaded(c, 1.15));
      for (const [dx, dy] of [[2, 2], [13, 2], [2, 13], [13, 13]]) dot(x + dx, y + dy, 0x3a2a20);
      if (h % 3 === 0) fill(x + 3 + h % 9, y + 3, 1, 6 + (h >>> 4) % 6, 0xa86a3a, 0.6);
      if (h % 4 === 1) { fill(x + 1, y + 13, 7 + h % 6, 2, 0xe8eef2, 0.85); dot(x + 2, y + 12, 0xe8eef2, 0.7); }
    }
  },
  /** A battle-station deck: dark grey plates with light top edges and dark seams, now and then a vent of slits or a pair of tiny blue floor lights. */
  starwarsdeck(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x3c4048);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 16, (r % 2) * 8); x < R; x += 16) {
      fill(x, y, 16, 1, 0x4c505a); fill(x, y, 1, 12, 0x464a54); fill(x + 15, y, 1, 12, 0x2c2f36); fill(x, y + 11, 16, 1, 0x2a2d34);
      const h = hash(x, r);
      if (h % 5 === 0) for (let k = 0; k < 3; k++) fill(x + 4, y + 3 + k * 2, 8, 1, 0x26282e);
      else if (h % 7 === 1) { dot(x + 3, y + 6, 0x6ab8ff, 0.8); dot(x + 12, y + 6, 0x6ab8ff, 0.8); }
    }
  },
  /** Desert sand: warm sand with wind ripples in light and dark dashes, a few pebbles and, rarely, a half-buried bolt. */
  starwarssand(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xd8b878);
    for (let y = T, r = 0; y < B; y += 6, r++) for (let x = from(L, 14, (r % 2) * 7); x < R; x += 14) {
      const h = hash(x, r), off = h % 5;
      fill(x + off, y + 2, 6, 1, 0xe8cc94); fill(x + off + 2, y + 3, 5, 1, 0xc4a062, 0.8);
      if (h % 9 === 0) dot(x + (h >>> 4) % 12, y + 5, 0x9a7a4a);
      if (h % 61 === 3) { dot(x + 8, y + 1, 0x8a909c); dot(x + 9, y + 1, 0xb8bcc4); }
    }
  },
  /** Pink marble: big white and blush tiles in a checker, gold grout, and pale veins wandering across them. */
  girly1marble(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xfbeef2);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 16), c = Math.floor(from(L, 16) / 16); x < R; x += 16, c++) {
      if ((r + c) % 2) fill(x, y, 16, 16, 0xf4cad8);
      fill(x, y, 16, 1, 0xe8c878, 0.55); fill(x, y, 1, 16, 0xe8c878, 0.55);
      const h = hash(x, r), vx = h % 12, dir = (h >>> 4) % 2 ? 1 : -1;
      for (let k = 0; k < 6; k++) dot(x + 2 + vx + Math.round(k * 0.6 * dir), y + 3 + k * 2, (r + c) % 2 ? 0xe8a8c0 : 0xead8de, 0.8);
    }
  },
  /** A hot pink shag rug: deep pink with little tufts in lighter and darker pink scattered all over it. */
  girly1shag(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xe0408a);
    for (let y = T, r = 0; y < B; y += 3, r++) for (let x = from(L, 4, (r % 2) * 2); x < R; x += 4) {
      const h = hash(x, r);
      dot(x + (h % 3), y + 1, h % 3 ? 0xf068a8 : 0xb82a6e, 0.9);
      if (h % 5 === 0) dot(x + 2, y, 0xf890c0, 0.8);
    }
  },
  /** Mint gingham: pale mint with crossing mint bands, darker where they meet. */
  girly2mint(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xe4f8ee);
    for (let x = from(L, 10); x < R; x += 10) fill(x, T, 5, B - T, G2_MINT, 0.55);
    for (let y = T; y < B; y += 10) fill(L, y, R - L, 5, G2_MINT, 0.55);
  },
  /** A lilac cloud carpet: soft lilac with little white puffy clouds and tiny yellow stars scattered over it. */
  girly2cloud(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xd8c8f0);
    for (let y = T, r = 0; y < B; y += 14, r++) for (let x = from(L, 20, (r % 2) * 10); x < R; x += 20) {
      const h = hash(x, r);
      if (h % 3 === 0) { fill(x + 3, y + 5, 7, 2, 0xf4f0fc); fill(x + 4, y + 4, 3, 1, 0xf4f0fc); fill(x + 7, y + 3, 2, 2, 0xf4f0fc); fill(x + 3, y + 7, 7, 1, 0xc4b0e4); }
      else if (h % 3 === 1) { dot(x + 12, y + 8, 0xfff0a0); dot(x + 11, y + 8, 0xfff0a0, 0.5); dot(x + 13, y + 8, 0xfff0a0, 0.5); dot(x + 12, y + 7, 0xfff0a0, 0.5); dot(x + 12, y + 9, 0xfff0a0, 0.5); }
      else dot(x + (h >>> 4) % 18, y + (h >>> 8) % 12, 0xe8dcfa);
    }
  },
  /** Hacienda tiles: terracotta squares with darker grout and lighter top edges, a little cream diamond set where four tiles meet. */
  narcostiles(L, T, R, B) {
    fill(L, T, R - L, B - T, NC_TERRA);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 12); x < R; x += 12) {
      fill(x, y, 12, 1, NC_TERRA_DARK); fill(x, y, 1, 12, NC_TERRA_DARK); fill(x + 1, y + 1, 11, 1, 0xd87a4a);
      const h = hash(x, r); if (h % 4 === 0) dot(x + 3 + h % 6, y + 4 + (h >>> 3) % 6, 0xb05a34);
      dot(x, y, NC_CREAM); dot(x - 1, y, NC_CREAM, 0.8); dot(x + 1, y, NC_CREAM, 0.8); dot(x, y - 1, NC_CREAM, 0.8); dot(x, y + 1, NC_CREAM, 0.8);
    }
  },
  /** A leopard rug: golden fur covered in dark broken rosettes with tawny middles. */
  narcosleopard(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xd8a850);
    for (let y = T, r = 0; y < B; y += 8, r++) for (let x = from(L, 10, (r % 2) * 5); x < R; x += 10) {
      const h = hash(x, r), cx = x + h % 4, cy = y + (h >>> 3) % 3;
      fill(cx + 1, cy + 1, 2, 2, 0xc08838); dot(cx, cy + 1, 0x3a2410); dot(cx + 1, cy, 0x3a2410); dot(cx + 3, cy + 1, 0x3a2410); dot(cx + 2, cy + 3, 0x3a2410); dot(cx + 3, cy + 2, 0x3a2410, 0.7);
      if (h % 3 === 0) dot(x + 7, y + 5, 0x3a2410);
    }
  },
  /** Light oak: long pale boards in staggered rows with a fine grain and the odd darker knot. */
  apple1oak(L, T, R, B) {
    fill(L, T, R - L, B - T, A1_OAK_LIGHT);
    for (let y = T, r = 0; y < B; y += 6, r++) {
      fill(L, y + 5, R - L, 1, A1_OAK, 0.8);
      for (let x = from(L, 40, (r % 3) * 13); x < R; x += 40) {
        fill(x, y, 1, 5, A1_OAK, 0.8);
        const h = hash(x, r);
        fill(x + 3 + h % 10, y + 2, 9, 1, 0xdcc094, 0.7); fill(x + 18 + (h >>> 4) % 10, y + 3, 7, 1, 0xdcc094, 0.6);
        if (h % 11 === 0) dot(x + 20, y + 2, A1_OAK_DARK);
      }
    }
  },
  /** Pale stone: big light grey slabs with fine seams and a scatter of tiny darker and lighter specks. */
  apple1stone(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xd8d6d2);
    for (let y = T, r = 0; y < B; y += 24, r++) for (let x = from(L, 24, (r % 2) * 12); x < R; x += 24) {
      fill(x, y, 24, 1, 0xc4c2be); fill(x, y, 1, 24, 0xc4c2be); fill(x + 1, y + 1, 23, 1, 0xe4e2de);
      for (let k = 0; k < 6; k++) { const h = hash(x + k, r * 7 + k); dot(x + 2 + h % 21, y + 2 + (h >>> 5) % 21, (h >>> 9) % 2 ? 0xb8b6b2 : 0xeceae6); }
    }
  },
  /** An 80s office carpet: warm brown with a small repeating diamond in tan and a darker fleck. */
  apple2carpet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x8a6a4a);
    for (let y = T, r = 0; y < B; y += 8, r++) for (let x = from(L, 8, (r % 2) * 4); x < R; x += 8) {
      dot(x + 3, y + 2, 0xa88660); dot(x + 2, y + 3, 0xa88660); dot(x + 4, y + 3, 0xa88660); dot(x + 3, y + 4, 0xa88660);
      if (hash(x, r) % 3 === 0) dot(x + 7, y + 6, 0x6a4a34);
    }
  },
  /** A one-bit floor: white tiles filled with the old paint program's patterns in black (fine dots, bricks, diagonals and checks), one pattern per tile. */
  apple2dither(L, T, R, B) {
    fill(L, T, R - L, B - T, A2_PAPER);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 16), c = 0; x < R; x += 16, c++) {
      const kind = hash(x, r) % 4;
      for (let yy = 0; yy < 16; yy += 2) for (let xx = 0; xx < 16; xx += 2) {
        const on = kind === 0 ? (xx + yy) % 4 === 0 : kind === 1 ? yy % 4 === 0 || (xx + (yy % 8 < 4 ? 0 : 4)) % 8 === 0 : kind === 2 ? (xx - yy + 32) % 6 === 0 : (xx / 2 + yy / 2) % 2 === 0 && xx % 4 === 0;
        if (on) dot(x + xx, y + yy, A2_INK, 0.55);
      }
      fill(x, y, 16, 1, A2_INK, 0.7); fill(x, y, 1, 16, A2_INK, 0.7);
    }
  },
  /** Jerusalem stone. Pale honey limestone pavers laid in staggered rows, each a slightly different cream, with chisel marks and the odd warm vein. */
  jewishstone(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xbfae88);
    for (let y = T, r = 0; y < B; y += 8, r++) for (let x = from(L, 16, (r % 2) * 8); x < R; x += 16) {
      const h = hash(x, r), tone = [0xe2d2ae, 0xdccaa2, 0xe8d9b8, 0xd8c69c][h % 4];
      fill(x + 1, y + 1, 15, 7, tone); fill(x + 1, y + 1, 15, 1, shaded(tone, 1.06)); fill(x + 1, y + 7, 15, 1, shaded(tone, 0.9));
      // Chisel marks, and now and then a warm vein
      if (h % 3 === 0) { dot(x + 3 + ((h >>> 4) % 9), y + 3, shaded(tone, 0.88)); dot(x + 6 + ((h >>> 6) % 7), y + 5, shaded(tone, 0.9)); }
      if (h % 7 === 0) fill(x + 4, y + 4, 6, 1, 0xd4b47e, 0.6);
    }
  },
  /** Blue and white tiles. A calm checker of glazed white and pale blue tiles, a small blue dot in the middle of each white one, with light grout. */
  jewishtiles(L, T, R, B) {
    const white = 0xf2f0ea, pale = 0xc6d8ee, blue = 0x2f5f9e;
    fill(L, T, R - L, B - T, 0xd4d6d8);
    for (let y = T, r = 0; y < B; y += 8, r++) for (let x = from(L, 8); x < R; x += 8) {
      const odd = (Math.floor(x / 8) + r) % 2 === 1, tone = odd ? pale : white;
      fill(x, y, 7, 7, tone); fill(x, y, 7, 1, shaded(tone, 1.04)); fill(x + 6, y, 1, 7, shaded(tone, 0.93));
      if (!odd) fill(x + 3, y + 3, 2, 2, blue);
    }
  },
  /** Star tiles. Glazed cream tiles in pale grout, every other one bearing an eight-pointed teal star with a gold heart. */
  muslimzellige(L, T, R, B) {
    const glaze = 0xf2ead8, teal = 0x2f8a8a, tealLight = 0x5ab0aa, terra = 0xc8643a;
    fill(L, T, R - L, B - T, 0xd4c8b0);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 12); x < R; x += 12) {
      fill(x, y, 11, 11, glaze); fill(x, y, 11, 1, 0xfaf4e6); fill(x + 10, y, 1, 11, 0xe2d8c4);
      if ((Math.floor(x / 12) + r) % 2) { dot(x + 5, y + 5, terra, 0.5); continue; }
      fill(x + 3, y + 3, 5, 5, teal); for (const [dx, dy] of [[5, 2], [5, 8], [2, 5], [8, 5]]) dot(x + dx, y + dy, teal);
      dot(x + 3, y + 3, tealLight); dot(x + 5, y + 5, 0xf2d35b);
    }
  },
  /** Red carpet. A deep red wool carpet patterned all over with navy lozenge medallions edged in cream, a gold heart in each and small cream flowers between, with a soft wool texture. */
  muslimcarpet(L, T, R, B) {
    const red = 0x9a2a2a, redDark = 0x7a1e22, navy = 0x1e2e5a, cream = 0xf2e2c0, gold = 0xd8b45a;
    fill(L, T, R - L, B - T, red);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 16, (r % 2) * 8); x < R; x += 16) {
      for (let k = 0; k < 4; k++) {
        fill(x + 8 - k, y + 2 + k, 1 + k * 2, 1, navy); fill(x + 8 - k, y + 9 - k, 1 + k * 2, 1, navy);
        dot(x + 7 - k, y + 2 + k, cream, 0.8); dot(x + 9 + k, y + 2 + k, cream, 0.8); dot(x + 7 - k, y + 9 - k, cream, 0.8); dot(x + 9 + k, y + 9 - k, cream, 0.8);
      }
      fill(x + 8, y + 5, 1, 2, gold);
      dot(x, y + 6, cream, 0.7); dot(x + 1, y + 6, gold, 0.6);
      const h = hash(x, r);
      for (let k = 0; k < 3; k++) dot(x + ((h >>> (k * 4)) % 16), y + ((h >>> (k * 4 + 2)) % 12), redDark, 0.6);
    }
  },
  /** Chapel flagstones. Worn grey-beige stone flags in staggered rows, each a slightly different tone, with a few pits and smoothed hollows. */
  christianflags(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x8a8070);
    for (let y = T, r = 0; y < B; y += 10, r++) for (let x = from(L, 14, (r % 3) * 5); x < R; x += 14) {
      const h = hash(x, r + 90), tone = [0xb8ab94, 0xb0a38c, 0xc0b49e, 0xaa9e88][h % 4];
      fill(x + 1, y + 1, 13, 9, tone); fill(x + 1, y + 1, 13, 1, shaded(tone, 1.08)); fill(x + 1, y + 9, 13, 1, shaded(tone, 0.88));
      if (h % 3 === 0) dot(x + 3 + ((h >>> 4) % 9), y + 4 + ((h >>> 8) % 3), shaded(tone, 0.85));
      if (h % 5 === 0) fill(x + 5, y + 5, 4, 1, shaded(tone, 0.94));
    }
  },
  /** Oak boards. Long honey oak planks of staggered lengths with dark grain streaks and a pair of nail heads at each end. */
  christianoak(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x6a4a2a);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 40, (r * 13) % 40); x < R; x += 40) {
      const h = hash(x, r + 70), tone = [0xa8783e, 0x9c6e38, 0xb08244, 0xa07240][h % 4];
      fill(x, y, 39, 3, tone); fill(x, y, 39, 1, shaded(tone, 1.08));
      for (let k = 0; k < 2; k++) fill(x + 4 + ((h >>> (k * 5)) % 28), y + 1 + (k % 2), 6, 1, shaded(tone, 0.88));
      dot(x + 1, y + 1, 0x5a3b25, 0.6); dot(x + 37, y + 1, 0x5a3b25, 0.6);
    }
  },
  /** Tatami mats. Woven straw mats in staggered rows, each in a slightly different green-gold, edged along their long sides with dark cloth borders. */
  buddhisttatami(L, T, R, B) {
    const light = 0xd8d0a0, weave = 0xb4ac74, edge = 0x2f3a2a;
    fill(L, T, R - L, B - T, 0x8a8458);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 32, (r % 2) * 16); x < R; x += 32) {
      const tone = hash(x, r + 33) % 2 ? 0xc8c088 : 0xc2ba82;
      fill(x, y, 31, 15, tone); fill(x, y + 2, 31, 1, light);
      for (let wy = y + 4; wy < y + 13; wy += 2) fill(x + 1, wy, 29, 1, weave, 0.5);
      fill(x, y, 31, 2, edge); fill(x, y + 13, 31, 2, edge); fill(x, y, 31, 1, 0x4a5a40);
    }
  },
  /** Slate and moss. Grey slate stepping stones of different sizes set in soft green moss, with lighter tufts here and there. */
  buddhistslate(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x4a6a3a);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 18, (r % 2) * 9); x < R; x += 18) {
      const h = hash(x, r + 51), tone = [0x5a6266, 0x666e72, 0x525a5e][h % 3], w = 13 + (h % 3), hh = 7 + ((h >>> 3) % 2);
      fill(x + 2, y + 2, w, hh, tone); fill(x + 3, y + 1, w - 2, 1, tone); fill(x + 3, y + 2 + hh, w - 2, 1, shaded(tone, 0.8));
      fill(x + 3, y + 2, w - 2, 1, shaded(tone, 1.15));
      dot(x + ((h >>> 5) % 18), y + ((h >>> 9) % 12), 0x6a8a4a); dot(x + ((h >>> 13) % 18), y + ((h >>> 17) % 12), 0x3a5a2e);
    }
  },
  /** Basalt and lava. Dark basalt slabs in staggered rows, each bevelled lighter at the top and darker below, with now and then a glowing seam of lava between two and a pale fleck of pumice. */
  volcanobasalt(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1a1616);
    for (let y = T, r = 0; y < B; y += 9, r++) for (let x = from(L, 12, (r % 2) * 6); x < R; x += 12) {
      const h = hash(x, r + 91), tone = [0x2e2a2a, 0x342e2c, 0x2a2626][h % 3];
      fill(x + 1, y + 1, 10, 7, tone); fill(x + 1, y + 1, 10, 1, shaded(tone, 1.3)); fill(x + 1, y + 7, 10, 1, shaded(tone, 0.8));
      if (h % 13 === 0) { fill(x, y, 1, 9, 0xc8401e); dot(x, y + 3 + ((h >>> 5) % 4), 0xffa040); }
      if (h % 17 === 1) { fill(x + 1, y + 8, 10, 1, 0xa83418); dot(x + 3 + ((h >>> 7) % 6), y + 8, 0xff8a2a); }
      if (h % 5 === 0) dot(x + 3 + ((h >>> 4) % 6), y + 3 + ((h >>> 8) % 3), 0x5a5050);
    }
  },
  /** Lair grating. Steel floor panels of slotted grating with dark seams and a rivet at each corner; under a few of them the slots glow lava-red from below. */
  volcanogrid(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2a2c34);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 16); x < R; x += 16) {
      const h = hash(x, r + 17), hot = h % 6 === 0;
      fill(x + 1, y + 1, 15, 15, 0x4a4e58); fill(x + 1, y + 1, 15, 1, 0x6e727c); fill(x + 1, y + 15, 15, 1, 0x363840);
      for (let sy = y + 3; sy < y + 14; sy += 2) fill(x + 3, sy, 11, 1, hot ? (sy % 4 ? 0xa83418 : 0xc8501e) : 0x2a2c34);
      for (const [rx, ry] of [[2, 2], [14, 2], [2, 14], [14, 14]]) dot(x + rx, y + ry, 0xa8adb6);
    }
  },
  /** Quarry tiles. Square red-brown terracotta quarry tiles in slightly different tones with dark grout lines, worn paler here and there, and now and then a little steel floor drain. */
  kitchenquarry(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x6a3a2a);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 12); x < R; x += 12) {
      const h = hash(x, r + 43), tone = [0xa84a32, 0x9a4430, 0xb05238, 0xa24834][h % 4];
      fill(x + 1, y + 1, 11, 11, tone); fill(x + 1, y + 1, 11, 1, shaded(tone, 1.12)); fill(x + 1, y + 11, 11, 1, shaded(tone, 0.88));
      if (h % 5 === 0) fill(x + 3 + ((h >>> 4) % 4), y + 4 + ((h >>> 7) % 4), 3, 2, shaded(tone, 1.08));
      if (h % 41 === 0) { fill(x + 4, y + 4, 5, 5, 0x8a9098); for (let k = 0; k < 5; k += 2) fill(x + 4, y + 4 + k, 5, 1, 0x4a4e58); }
    }
  },
  /** Kitchen mats. Black rubber anti-fatigue mats laid end to end, honeycombed with little drainage holes in rows, with a bevelled edge at each seam. */
  kitchenmat(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2a2a2e);
    for (let y = T, r = 0; y < B; y += 20, r++) for (let x = from(L, 30); x < R; x += 30) {
      fill(x, y, 30, 1, 0x3e3e44); fill(x, y, 1, 20, 0x3e3e44); fill(x + 29, y, 1, 20, 0x1a1a1e);
      for (let hy = y + 3, k = 0; hy < y + 18; hy += 3, k++) for (let hx = x + 3 + (k % 2) * 2; hx < x + 28; hx += 4) { dot(hx, hy, 0x141416); dot(hx + 1, hy, 0x141416); }
    }
  },
  /** Ward linoleum. Pale sage-green sheet vinyl flecked with darker and lighter specks, laid in wide strips with faint welded seams, polished to a soft shine in places. */
  hospitallino(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xc8dcd0);
    for (let x = from(L, 40); x < R; x += 40) fill(x, T, 1, B - T, 0xb4cabe);
    for (let y = T; y < B; y += 3) for (let x = from(L, 5, y % 2 ? 2 : 0); x < R; x += 5) {
      const h = hash(x, y); if (h % 3 === 0) dot(x + (h >>> 4) % 4, y, h % 2 ? 0xa8c0b2 : 0xe2eee8);
    }
    for (let y = T + 10; y < B; y += 46) for (let x = from(L, 70, 20); x < R; x += 70) fill(x, y, 14, 1, WHITE, 0.25);
  },
  /** Wayfinding floor. Blue-grey vinyl with coloured guide lines running along it, green, blue and yellow, turning at the corners, to lead the way to the wards. */
  hospitallines(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xa8b4bc);
    for (let y = T; y < B; y += 4) for (let x = from(L, 7, y % 8 ? 3 : 0); x < R; x += 7) if (hash(x, y) % 4 === 0) dot(x, y, 0x98a4ac);
    const lines = [[0x3ab86a, 30], [0x4c8bf5, 34], [0xf2c830, 38]];
    for (let y = from(T, 60); y < B; y += 60) for (const [c, off] of lines) if (y + off >= T && y + off < B) fill(L, y + off, R - L, 2, c, 0.85);
    for (let x = from(L, 120, 60); x < R; x += 120) for (const [c, off] of lines) fill(x + off - 30, T, 2, B - T, c, 0.5);
  },
  /** Worn checker. Black and cream linoleum squares, scuffed and faded, the cream gone yellowish and the black grey in a path where feet go most. */
  noirchecker(L, T, R, B) {
    for (let y = from(T, 8), r = 0; y < B; y += 8, r++) for (let x = from(L, 8); x < R; x += 8) {
      const dark = ((x / 8) + (y / 8)) % 2 === 0, h = hash(x, y + 3);
      const c = dark ? [0x2a2a2e, 0x34343a, 0x3a3a40][h % 3] : [0xd8d0bc, 0xd0c6ae, 0xc8bea4][h % 3];
      fill(x, y, 8, 8, c); if (h % 7 === 0) fill(x + 1 + (h >>> 4) % 4, y + 2 + (h >>> 7) % 4, 3, 1, dark ? 0x4a4a52 : 0xb8ae94);
    }
  },
  /** Scuffed boards. Narrow dark oak floorboards, long and staggered, scuffed lighter where chairs have dragged, a few nail heads. */
  noirboards(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x24180f);
    for (let y = T, r = 0; y < B; y += 5, r++) for (let x = from(L, 48, (hash(r, 9) % 4) * 12); x < R; x += 48) {
      const h = hash(x, r + 61), tone = [0x3a2a20, 0x42301f, 0x352619][h % 3];
      fill(x + 1, y, 47, 4, tone); fill(x + 1, y, 47, 1, shaded(tone, 1.2));
      if (h % 4 === 0) fill(x + 8 + (h >>> 5) % 24, y + 2, 8, 1, shaded(tone, 1.35));
      dot(x + 3, y + 2, 0x6a5a4a); dot(x + 45, y + 2, 0x6a5a4a);
    }
  },
  /** Cracked lab tile. Old pale green-grey square tiles, some cracked, some stained, with a round iron drain and a glowing green puddle here and there. */
  madlabdrain(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x7a8a7a);
    for (let y = T, r = 0; y < B; y += 10, r++) for (let x = from(L, 10); x < R; x += 10) {
      const h = hash(x, r + 17), tone = [0xa8b8a8, 0xa0b0a0, 0xb0c0ae][h % 3];
      fill(x + 1, y + 1, 9, 9, tone); fill(x + 1, y + 1, 9, 1, shaded(tone, 1.08));
      if (h % 9 === 0) { line(x + 2, y + 2, x + 6, y + 6, 0x6a7a6a); line(x + 6, y + 6, x + 8, y + 5, 0x6a7a6a); }
      if (h % 13 === 1) fill(x + 3, y + 4, 4, 3, 0x8a9a7a, 0.6);
      if (h % 113 === 2) { disc(x + 5, y + 5, 3, 0x4a4e58); for (let k = -2; k <= 2; k += 2) fill(x + 3, y + 5 + k, 5, 1, 0x2a2c34); }
      if (h % 71 === 5) { fill(x + 2, y + 6, 6, 3, 0x7ae04a, 0.7); fill(x + 3, y + 5, 4, 1, 0x7ae04a, 0.7); dot(x + 3, y + 6, 0xc8ff9a); }
    }
  },
  /** Riveted iron. Dark iron floor plates with a raised tread, rivets round their edges, and bundles of copper cable snaking across them between the desks. */
  madlabplate(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2e2a28);
    for (let y = T, r = 0; y < B; y += 20, r++) for (let x = from(L, 20); x < R; x += 20) {
      fill(x + 1, y + 1, 19, 19, 0x4a4440); fill(x + 1, y + 1, 19, 1, 0x5e5650);
      for (let ty = y + 4; ty < y + 18; ty += 4) for (let tx = x + 3 + ((ty / 4) % 2) * 2; tx < x + 18; tx += 4) { dot(tx, ty, 0x5a524c); dot(tx + 1, ty + 1, 0x3a3430); }
      for (const [rx, ry] of [[2, 2], [18, 2], [2, 18], [18, 18], [10, 2], [10, 18]]) dot(x + rx, y + ry, 0x7a726a);
    }
    for (let x = from(L, 90, 30); x < R; x += 90) for (let y = T; y < B; y++) { const wob = Math.round(Math.sin(y * 0.15 + x) * 2); dot(x + wob, y, 0xb87333); dot(x + wob + 1, y, 0x8a4a22); }
  },
  /** Casino carpet. A busy deep crimson carpet in a repeat of gold scrolls and violet starbursts, the kind that keeps you awake. */
  casinocarpet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x7a1e3a);
    for (let y = from(T, 16), r = 0; y < B; y += 16, r++) for (let x = from(L, 16, (r % 2) * 8); x < R; x += 16) {
      for (const [dx, dy] of [[0, -3], [0, 3], [-3, 0], [3, 0], [-2, -2], [2, 2], [2, -2], [-2, 2]]) dot(x + 8 + dx, y + 8 + dy, 0xb85ad8);
      dot(x + 8, y + 8, 0xf2d35b);
      line(x + 1, y + 3, x + 4, y + 1, 0xd8b45a, 0.8); line(x + 4, y + 1, x + 5, y + 3, 0xd8b45a, 0.8); dot(x + 3, y + 4, 0xd8b45a, 0.8);
      fill(x + 12, y + 13, 2, 1, 0x3a8aa8);
    }
  },
  /** Black and gold marble. Polished black marble slabs with pale grey veins, set in a grid of thin gold inlay lines with a gold diamond at each crossing. */
  casinomarble(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1e1a1e);
    for (let y = from(T, 24), r = 0; y < B; y += 24, r++) for (let x = from(L, 24); x < R; x += 24) {
      const h = hash(x, r + 7); let vx = x + (h % 20);
      for (let vy = y + 1; vy < y + 23; vy++) { vx += ((hash(vx, vy) % 3) - 1); if (vx > x && vx < x + 23) dot(vx, vy, 0x5a5460, 0.7); }
      if (h % 3 === 0) fill(x + 4, y + 6, 6, 1, WHITE, 0.08);
      fill(x, y, 24, 1, 0xa8843a); fill(x, y, 1, 24, 0xa8843a); dot(x, y, 0xf2d888); dot(x - 1, y, 0xd8b45a); dot(x + 1, y, 0xd8b45a); dot(x, y - 1, 0xd8b45a); dot(x, y + 1, 0xd8b45a);
    }
  },
  /** Station terrazzo. A pale cream terrazzo floor flecked with marble chips, laid in big squares with thin brass strips and a fan of sunrays in black and gold in each corner, as in a grand station hall. */
  orientterrazzo(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xd8ccb0);
    for (let y = T; y < B; y += 2) for (let x = from(L, 3, y % 4 ? 1 : 0); x < R; x += 3) { const h = hash(x, y); if (h % 5 === 0) dot(x, y, [0xb8a888, 0xe8dcc0, 0xa89878, 0x8a8a80][h % 4]); }
    for (let y = from(T, 40); y < B; y += 40) for (let x = from(L, 40); x < R; x += 40) {
      fill(x, y, 40, 1, 0xc8a050); fill(x, y, 1, 40, 0xc8a050);
      for (let k = 0; k < 5; k++) { const a = (k / 4) * (Math.PI / 2); line(x + 1, y + 1, x + 1 + Math.round(Math.cos(a) * 7), y + 1 + Math.round(Math.sin(a) * 7), k % 2 ? 0x2a2a30 : 0xd8b45a); }
    }
  },
  /** Carriage carpet. A deep midnight-blue carpet patterned with little gold four-petalled flowers in a diamond grid, edged with a gold border band. */
  orientcarpet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1e2a4a);
    for (let y = from(T, 10), r = 0; y < B; y += 10, r++) for (let x = from(L, 10, (r % 2) * 5); x < R; x += 10) {
      dot(x + 5, y + 4, 0xd8b45a); dot(x + 4, y + 5, 0xd8b45a); dot(x + 6, y + 5, 0xd8b45a); dot(x + 5, y + 6, 0xd8b45a); dot(x + 5, y + 5, 0xc8302a);
    }
    for (let y = from(T, 60, 2); y < B; y += 60) { fill(L, y, R - L, 2, 0x8a6a2a); fill(L, y, R - L, 1, 0xd8b45a); }
  },
  /** Haunted boards. Old warped floorboards of near-black wood, split and gapped, with fallen autumn leaves, orange, red and brown, lying here and there and the odd nail working loose. */
  halloweendxboards(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x140e14);
    for (let y = T, r = 0; y < B; y += 6, r++) for (let x = from(L, 40, (hash(r, 13) % 5) * 8); x < R; x += 40) {
      const h = hash(x, r + 29), tone = [0x2a2228, 0x30262c, 0x262024][h % 3];
      fill(x + 1, y, 39, 5, tone); fill(x + 1, y, 39, 1, shaded(tone, 1.2));
      if (h % 5 === 0) line(x + 6 + (h >>> 4) % 20, y + 1, x + 14 + (h >>> 4) % 20, y + 3, 0x1a1418);
      if (h % 9 === 1) dot(x + 3, y + 2, 0x6a6a70);
      if (h % 7 === 2) { const lx = x + 4 + (h >>> 6) % 30, c = [0xe8782e, 0xc8402a, 0x8a5a2a, 0xd8a040][(h >>> 9) % 4]; fill(lx, y + 1, 2, 2, c); dot(lx + 2, y + 2, shaded(c, 0.8)); }
    }
  },
  /** Graveyard ground. Dark earth with tufts of dead grass, scattered moss, little stones and fallen leaves, and here and there a pale mushroom. */
  halloweendxsoil(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2e3424);
    for (let y = T; y < B; y += 3) for (let x = from(L, 4, y % 2 ? 2 : 0); x < R; x += 4) {
      const h = hash(x, y);
      if (h % 6 === 0) dot(x, y, 0x3a4430); else if (h % 6 === 1) dot(x, y, 0x24281c);
      if (h % 47 === 3) { dot(x, y, 0x5a6a3a); dot(x - 1, y - 1, 0x5a6a3a); dot(x + 1, y - 1, 0x5a6a3a); }
      if (h % 61 === 5) fill(x, y, 2, 1, 0x6a6a64);
      if (h % 53 === 7) { const c = [0xe8782e, 0xc8402a, 0xd8a040][(h >>> 7) % 3]; fill(x, y, 2, 1, c); }
      if (h % 211 === 9) { fill(x - 1, y - 2, 3, 1, 0xe8dcc8); dot(x, y - 1, 0xd8ccb0); }
    }
  },
  /** Herringbone oak. Light honey oak blocks laid in a herringbone, a little paler where the sun falls, with a trail of tiny dusty paw prints here and there. */
  catcafeoak(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xb8884e);
    for (let y = from(T, 6), r = 0; y < B; y += 6, r++) for (let x = from(L, 12, (r % 2) * 6); x < R; x += 12) {
      const h = hash(x, r + 71), tone = [0xd0a066, 0xd4a66c, 0xcc9e62][h % 3];
      fill(x, y, 6, 3, tone); fill(x, y, 6, 1, shaded(tone, 1.1)); fill(x + 6, y + 3, 6, 3, tone); fill(x + 6, y + 3, 6, 1, shaded(tone, 1.1));
      fill(x + 5, y, 1, 6, shaded(tone, 0.93));
      if (h % 31 === 0) for (let k = 0; k < 3; k++) { const px = x + k * 4, py = y + 1 + (k % 2) * 2; dot(px, py, 0x8a6a40, 0.6); dot(px + 1, py - 1, 0x8a6a40, 0.5); dot(px - 1, py - 1, 0x8a6a40, 0.5); }
    }
  },
  /** Patchwork carpet. A soft carpet of pastel squares, pink, mint, butter and lilac, with little stitched paw prints and fish on some of them. */
  catcafepatch(L, T, R, B) {
    for (let y = from(T, 14), r = 0; y < B; y += 14, r++) for (let x = from(L, 14); x < R; x += 14) {
      const h = hash(x, y + 9), c = [0xf2c0c8, 0xc0e4d0, 0xf6e4a8, 0xd8c8ec][h % 4];
      fill(x, y, 14, 14, c); for (let k = 0; k < 14; k += 2) { dot(x + k, y, shaded(c, 0.88)); dot(x, y + k, shaded(c, 0.88)); }
      if (h % 3 === 0) { const px = x + 6, py = y + 7; dot(px, py, WHITE, 0.8); dot(px - 1, py - 2, WHITE, 0.8); dot(px + 1, py - 2, WHITE, 0.8); dot(px - 2, py - 1, WHITE, 0.8); dot(px + 2, py - 1, WHITE, 0.8); }
      else if (h % 3 === 1) { fill(x + 4, y + 6, 4, 2, WHITE, 0.6); dot(x + 8, y + 5, WHITE, 0.6); dot(x + 8, y + 8, WHITE, 0.6); }
    }
  },
  /** Brooklyn rooftop. Dark grey tar-paper laid in long sheets with lighter lapped seams, sprinkled with pale gravel, a painted white spot here and there. */
  spiderverserooftop(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x3a3a44);
    for (let y = T, r = 0; y < B; y += 10, r++) {
      fill(L, y, R - L, 1, 0x4a4a56); fill(L, y + 1, R - L, 1, 0x2e2e36, 0.6);
      for (let x = from(L, 40, (r % 2) * 20); x < R; x += 40) fill(x, y, 1, 10, 0x2e2e36, 0.7);
      for (let x = from(L, 7, r * 3); x < R; x += 7) { const h = hash(x, y); if (h % 3 === 0) dot(x + (h >>> 3) % 6, y + 2 + (h >>> 6) % 7, 0x6a6a74); if (h % 11 === 0) dot(x + 2, y + 5, 0x8a8a92); }
    }
  },
  /** Halftone carpet. A comic-book red carpet printed in halftone: rows of darker dots that swell into bigger ones in diagonal bands. */
  spiderversehalftone(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xe84a6a);
    for (let y = T, r = 0; y < B; y += 3, r++) for (let x = from(L, 3, (r % 2) * 1); x < R; x += 3) {
      const band = Math.floor((x + y) / 18) % 3;
      if (band === 0) fill(x, y, 2, 2, 0xc02a50); else if (band === 1) dot(x, y, 0xc02a50); else dot(x, y, 0xf2788e);
    }
  },
  /** Danger Room grid. Blue-grey steel tiles with dark seams and riveted corners, a faint blue light glowing in each tile's middle. */
  xmendanger(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x4a5260);
    for (let y = T; y < B; y += 16) {
      fill(L, y, R - L, 1, 0x3a4250); fill(L, y + 1, R - L, 1, 0x5a6270);
      for (let x = from(L, 16); x < R; x += 16) { fill(x, y, 1, 16, 0x3a4250); fill(x + 1, y + 1, 1, 15, 0x5a6270); dot(x + 2, y + 2, 0x7a8290); dot(x + 14, y + 14, 0x2e3440); fill(x + 7, y + 7, 2, 2, 0x5a8ad0, 0.5); }
    }
  },
  /** Mansion parquet. Polished oak laid in a basket weave: squares of three planks, turned each way in turn, with a dark seam between. */
  xmenparquet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x5a3b25);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 12), c = 0; x < R; x += 12, c++) {
      const across = (Math.floor((x - L) / 12) + r) % 2 === 0;
      for (let k = 0; k < 3; k++) {
        const tone = [0x8a5a32, 0x9a663a, 0x7e522e][(k + hash(x, y)) % 3];
        if (across) { fill(x, y + k * 4, 11, 3, tone); fill(x, y + k * 4, 11, 1, shaded(tone, 1.12)); }
        else { fill(x + k * 4, y, 3, 11, tone); fill(x + k * 4, y, 1, 11, shaded(tone, 1.12)); }
      }
    }
  },
  /** Grid tarmac. Dark asphalt flecked with lighter stones, the white brackets of grid slots painted on it in staggered pairs. */
  f1tarmac(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x3a3c40);
    for (let y = T; y < B; y += 4) for (let x = from(L, 5, (y >> 2) % 5); x < R; x += 5) { const h = hash(x, y); if (h % 3 === 0) dot(x, y + (h >>> 4) % 4, h % 2 ? 0x4a4c52 : 0x2e3034); }
    for (let y = T + 6, r = 0; y < B; y += 28, r++) for (let x = from(L, 48, (r % 2) * 24); x < R; x += 48) {
      fill(x, y, 14, 1, 0xe8e8e8, 0.85); fill(x, y, 1, 4, 0xe8e8e8, 0.85); fill(x + 13, y, 1, 4, 0xe8e8e8, 0.85);
    }
  },
  /** Chequered carpet. A paddock carpet in the chequered flag's black and white squares, worn a little grey here and there. */
  f1chequer(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xe8e8e8);
    for (let y = T, r = 0; y < B; y += 6, r++) for (let x = from(L, 6), c = 0; x < R; x += 6, c++) {
      if ((Math.floor((x - L) / 6) + r) % 2) fill(x, y, 6, 6, 0x24262a); else if (hash(x, y) % 9 === 0) fill(x + 1, y + 1, 3, 2, 0xd4d4d8);
    }
  },
  /** Octagon canvas. An off-white canvas mat with faint scuffs and a broad dark border line running through it in long octagon-cut segments. */
  ufccanvas(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xe8e6e0);
    for (let y = T; y < B; y += 5) for (let x = from(L, 9, y % 9); x < R; x += 9) { const h = hash(x, y); if (h % 4 === 0) fill(x, y + (h >>> 3) % 5, 2 + (h >>> 6) % 3, 1, 0xd4d0c8); }
    for (let y = T + 30; y < B; y += 60) { fill(L, y, R - L, 2, 0x2a2a3a, 0.75); fill(L, y + 2, R - L, 1, 0xd8202a, 0.5); }
  },
  /** Gym mats. Black interlocking foam mats with jigsaw edges, a red one every so often. */
  ufcmats(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2a2a30);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 16), c = 0; x < R; x += 16, c++) {
      const red = hash(x, y) % 13 === 0, base = red ? 0x5a2226 : 0x2a2a30;
      fill(x, y, 15, 15, base); fill(x, y, 15, 1, shaded(base, 1.2));
      fill(x + 15, y, 1, 16, 0x1a1a1e); fill(x, y + 15, 16, 1, 0x1a1a1e);
      for (const k of [3, 9]) { dot(x + 15, y + k, base); dot(x + k, y + 15, base); dot(x + 16, y + k + 1, base); }
    }
  },
  /** Reclaimed boards. Wide old pine floorboards in honey and amber tones, worn paler in places, with dark nail heads at their ends. */
  nyloftboards(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x6a4a2e);
    for (let y = T, r = 0; y < B; y += 7, r++) for (let x = from(L, 56, (hash(r, 9) % 4) * 14); x < R; x += 56) {
      const tone = [0xa87a4a, 0x9a6e40, 0xb8884e, 0x946a3e][hash(x, y) % 4];
      fill(x, y, 55, 6, tone); fill(x, y, 55, 1, shaded(tone, 1.1));
      if (hash(y, x) % 3 === 0) fill(x + 12, y + 2, 18, 2, shaded(tone, 1.12), 0.6);
      dot(x + 2, y + 3, 0x3a2a1e); dot(x + 52, y + 3, 0x3a2a1e);
    }
  },
  /** Kilim rug. A flat-woven rug in brick red with rows of stepped diamonds in cream, indigo and ochre, and a cream border band. */
  nyloftrug(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xa84a3a);
    for (let y = T, r = 0; y < B; y += 12, r++) {
      fill(L, y, R - L, 1, 0xe8dcc0); fill(L, y + 11, R - L, 1, 0x8a3a2e);
      for (let x = from(L, 16, (r % 2) * 8); x < R; x += 16) {
        const c = [0xe8dcc0, 0x2e3a6a, 0xd8a040][(r + Math.floor((x - L) / 16)) % 3];
        fill(x + 7, y + 3, 2, 1, c); fill(x + 5, y + 4, 6, 1, c); fill(x + 3, y + 5, 10, 2, c); fill(x + 5, y + 7, 6, 1, c); fill(x + 7, y + 8, 2, 1, c);
        dot(x + 8, y + 5, 0xa84a3a); dot(x + 7, y + 6, 0xa84a3a);
      }
    }
  },
  /** Bistro mosaic. Small white hexagon-like mosaic tiles with a black border band and a little black flower picked out every so often. */
  pariscafetiles(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xd8d2c4);
    for (let y = T, r = 0; y < B; y += 3, r++) for (let x = from(L, 3, (r % 2) * 1); x < R; x += 3) fill(x, y, 2, 2, 0xf0ece2);
    for (let y = T + 6, r = 0; y < B; y += 18, r++) for (let x = from(L, 18, (r % 2) * 9); x < R; x += 18) {
      for (const [dx, dy] of [[1, 0], [0, 1], [2, 1], [1, 2], [1, 1]]) fill(x + dx * 3, y + dy * 3, 2, 2, dx === 1 && dy === 1 ? 0xb83a32 : 0x24242a);
    }
  },
  /** Chevron parquet. Honey oak laid in a point-de-Hongrie chevron, the boards meeting in neat Vs along each row, waxed to a soft shine. */
  pariscafechevron(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x6a4628);
    const tones = [0xa8743e, 0xb8844a, 0x9a6a38, 0xb07a42];
    for (let x = from(L, 24); x < R; x += 24) for (let c = 0; c < 8; c++) {
      const cx = x + c * 3, rise = c < 4 ? c * 3 : (7 - c) * 3;
      for (let y = T - 12 + rise, k = 0; y < B; y += 8, k++) {
        if (y + 7 <= T) continue;
        const tone = tones[(k + (c < 4 ? 0 : 2)) % 4], top = Math.max(T, y);
        fill(cx, top, 3, y + 7 - top, tone); if (y >= T) fill(cx, y, 3, 1, shaded(tone, 1.1));
      }
      if (c === 3) fill(cx + 2, T, 1, B - T, 0x6a4628, 0.7);
    }
  },
  /** Talavera terracotta. Warm terracotta floor tiles set with a small blue and white painted Talavera tile at every other corner. */
  muertostiles(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x9a4a2a);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 12), c = 0; x < R; x += 12, c++) {
      const tone = [0xc86a3a, 0xb85e32, 0xd0743e][hash(x, y) % 3];
      fill(x, y, 11, 11, tone); fill(x, y, 11, 1, shaded(tone, 1.1));
      if ((Math.floor((x - L) / 12) + r) % 2 === 0) { fill(x + 9, y + 9, 5, 5, 0xf2ece0); dot(x + 11, y + 11, 0x2a4ab0); dot(x + 10, y + 11, 0x2a4ab0); dot(x + 12, y + 11, 0x2a4ab0); dot(x + 11, y + 10, 0x2a4ab0); dot(x + 11, y + 12, 0x2a4ab0); dot(x + 11, y + 11, 0xf2c41a); }
    }
  },
  /** Marigold path. Dark earth strewn thick with marigold petals in orange and gold, with a few magenta cockscomb petals among them. */
  muertospetals(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x5a3a24);
    for (let y = T; y < B; y += 2) for (let x = from(L, 3, (y >> 1) % 3); x < R; x += 3) {
      const h = hash(x, y);
      if (h % 5 < 3) dot(x, y, [0xe8902a, 0xf2b03a, 0xd8782a][h % 3]);
      else if (h % 23 === 0) dot(x, y, 0xc82a6a);
    }
  },
  /** Orange grating. Steel floor grating painted safety orange, its bars casting little shadows, with pipes and cool blue light glimpsed beneath. */
  submarinedeepgrate(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xb8601e);
    for (let y = T, r = 0; y < B; y += 6, r++) for (let x = from(L, 6); x < R; x += 6) {
      const deep = hash(x >> 5, y >> 5) % 3 === 0;
      fill(x + 1, y + 1, 4, 4, deep ? 0x1e3a54 : 0x24323e); dot(x + 1, y + 1, 0x14202a);
    }
    for (let y = T; y < B; y += 30) fill(L, y, R - L, 1, 0xd8803a);
  },
  /** Hex deck mat. A dark blue rubber mat moulded in little raised hexagons, worn paler along a walkway. */
  submarinedeepmat(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1e2a44);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 6, (r % 2) * 3); x < R; x += 6) { fill(x, y, 3, 2, 0x2a3a5a); dot(x, y, 0x3a4a6a); dot(x + 1, y + 2, 0x16203a); }
  },
  /** Tread plate. Cold grey aluminium diamond-tread plates bolted in squares, a little grit of salt here and there. */
  arcticbasetread(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x8a929a);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 6, (r % 2) * 3); x < R; x += 6) { fill(x, y, 2, 1, 0xb0b8c0); dot(x + 1, y + 1, 0x6a727a); }
    for (let y = T; y < B; y += 32) fill(L, y, R - L, 1, 0x5a626a);
    for (let x = from(L, 48); x < R; x += 48) fill(x, T, 1, B - T, 0x5a626a);
    for (let y = T + 2; y < B; y += 32) for (let x = from(L, 48, 2); x < R; x += 48) { dot(x, y, 0x4a525a); dot(x + 43, y, 0x4a525a); }
  },
  /** Nordic rug. A thick wool rug in red with bands of white snowflake stars and little reindeer-like zigzags. */
  arcticbaserug(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xb82a2a);
    for (let y = T, r = 0; y < B; y += 14, r++) {
      fill(L, y, R - L, 1, 0xf2ece0); fill(L, y + 12, R - L, 1, 0xf2ece0);
      for (let x = from(L, 4); x < R; x += 4) { dot(x, y + 2, 0xf2ece0); dot(x + 2, y + 10, 0xf2ece0); }
      for (let x = from(L, 12, (r % 2) * 6); x < R; x += 12) { dot(x + 3, y + 4, 0xf2ece0); dot(x + 3, y + 8, 0xf2ece0); fill(x + 1, y + 6, 5, 1, 0xf2ece0); fill(x + 3, y + 5, 1, 3, 0xf2ece0); dot(x + 2, y + 5, 0xf2ece0); dot(x + 4, y + 7, 0xf2ece0); dot(x + 4, y + 5, 0xf2ece0); dot(x + 2, y + 7, 0xf2ece0); }
    }
  },
  /** Petal boards. Pale ash floorboards with a soft grain, a few fallen cherry petals lying here and there. */
  sakuraboards(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xc8b090);
    for (let y = T, r = 0; y < B; y += 6, r++) for (let x = from(L, 44, (hash(r, 3) % 4) * 11); x < R; x += 44) {
      const tone = [0xe8d4b8, 0xe0ccae, 0xecdac0][hash(x, y) % 3];
      fill(x, y, 43, 5, tone); fill(x, y, 43, 1, shaded(tone, 1.05)); fill(x + 6 + hash(y, x) % 20, y + 2, 9, 1, shaded(tone, 0.94));
      if (hash(x, y + 1) % 3 === 0) { const px = x + hash(x + 1, y) % 40; dot(px, y + 3, 0xf6b8c8); dot(px + 1, y + 3, 0xfff0f4); }
    }
  },
  /** Clover lawn. Fresh spring grass dotted with clover leaves, little white and yellow flowers, and fallen pink petals. */
  sakuralawn(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x8ac06a);
    for (let y = T; y < B; y += 2) for (let x = from(L, 3, (y >> 1) % 3); x < R; x += 3) {
      const h = hash(x, y);
      if (h % 4 === 0) dot(x, y, 0x6aa04a); else if (h % 4 === 1) dot(x, y, 0x9ad07a);
      if (h % 37 === 0) { dot(x, y, WHITE); dot(x + 1, y, 0xf2c41a); } else if (h % 29 === 0) dot(x, y, 0xf6b8c8);
    }
  },
  /** Castle flagstones. Big worn flags of warm honey-brown sandstone, long and short in staggered rows on dark mortar, each a slightly different tone, worn paler on top, with a pit or a speck of moss here and there. */
  potterflags(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x3e342a);
    for (let y = T, r = 0; y < B; y += 12, r++) {
      let x = from(L, 44, (r % 2) * 22) - 44;
      for (let k = 0; x < R; k++) {
        const h = hash(x, r + 71), w = [20, 24, 18, 26][(h >>> 3) % 4], tone = [0x8a7656, 0x92805e, 0x806c4e, 0x988664][h % 4];
        fill(x + 1, y + 1, w - 1, 11, tone); fill(x + 1, y + 1, w - 1, 1, shaded(tone, 1.14)); fill(x + 1, y + 1, 1, 10, shaded(tone, 1.06));
        fill(x + 1, y + 11, w - 1, 1, shaded(tone, 0.82)); fill(x + w - 1, y + 2, 1, 9, shaded(tone, 0.9));
        if (h % 5 === 0) { const px = x + 3 + ((h >>> 4) % (w - 6)), py = y + 4 + ((h >>> 8) % 5); dot(px, py, shaded(tone, 0.78)); dot(px + 1, py, shaded(tone, 0.86)); }
        if (h % 7 === 1) dot(x + 2 + ((h >>> 5) % (w - 4)), y + 10, 0x5a7040, 0.7);
        x += w;
      }
    }
  },
  /** Gryffindor common room carpet. A deep scarlet rug worked with small gold diamonds in staggered rows, with a darker woven line between each row. */
  pottercommon(L, T, R, B) {
    const gold = 0xd3a625, goldDark = 0x9a7418;
    fill(L, T, R - L, B - T, 0x8a1e1e);
    for (let y = T, r = 0; y < B; y += 12, r++) {
      fill(L, y + 11, R - L, 1, 0x6e1616);
      for (let x = from(L, 12, (r % 2) * 6); x < R; x += 12) {
        dot(x + 6, y + 3, gold); dot(x + 5, y + 4, gold); dot(x + 7, y + 4, gold); dot(x + 6, y + 5, goldDark); dot(x + 6, y + 4, 0xa02a24);
        dot(x + 1, y + 8, 0xa02a24, 0.6);
      }
    }
  },
  /** Bag End's floor. Wide honey-coloured oak boards in long staggered lengths with darker seams, a knot here and there and a warm sheen along each board. */
  lotrshire(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x6a4424);
    for (let y = T, r = 0; y < B; y += 6, r++) {
      for (let x = from(L, 40, (r * 13) % 40) - 40; x < R; x += 40) {
        const h = hash(x, r + 11), tone = [0xa8743e, 0xb07c44, 0x9c6a38, 0xb48450][h % 4];
        fill(x + 1, y, 39, 5, tone); fill(x + 1, y, 39, 1, shaded(tone, 1.12)); fill(x + 1, y + 4, 39, 1, shaded(tone, 0.88));
        if (h % 4 === 0) { const kx = x + 6 + ((h >>> 4) % 26); dot(kx, y + 2, 0x6a4424); dot(kx + 1, y + 2, shaded(tone, 0.8)); }
        fill(x + 4 + ((h >>> 7) % 20), y + 2, 6, 1, shaded(tone, 1.06), 0.7);
      }
    }
  },
  /** The floor of Meduseld, the Golden Hall: green stone flags set with interlocking gold knotwork bands in a grid. */
  lotrmeduseld(L, T, R, B) {
    const gold = 0xc8a050, goldDark = 0x8a6a2a;
    fill(L, T, R - L, B - T, 0x4a5a32);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 16); x < R; x += 16) {
      const tone = (hash(x, r + 5) % 2) ? 0x5a6a3a : 0x52623a;
      fill(x + 1, y + 1, 14, 14, tone); fill(x + 1, y + 1, 14, 1, shaded(tone, 1.15));
      fill(x, y, 16, 1, gold, 0.85); fill(x, y, 1, 16, gold, 0.85);
      for (let k = 2; k < 16; k += 4) { dot(x + k, y, goldDark); dot(x, y + k, goldDark); }
      dot(x + 8, y + 8, gold); dot(x + 7, y + 7, gold, 0.6); dot(x + 9, y + 9, gold, 0.6); dot(x + 7, y + 9, gold, 0.6); dot(x + 9, y + 7, gold, 0.6);
    }
  },
  /** The superlab floor. Pale grey sealed concrete in big square slabs with steel drain grates here and there and a yellow safety line every few rows. */
  breakingbadlab(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x8a9096);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 16); x < R; x += 16) {
      const h = hash(x, r + 61), tone = h % 3 ? 0xa8aeb4 : 0xa0a6ac;
      fill(x + 1, y + 1, 15, 15, tone); fill(x + 1, y + 1, 15, 1, 0xbcc2c8);
      if (h % 29 === 0) { fill(x + 5, y + 5, 6, 6, 0x6e747c); for (let k = 0; k < 6; k += 2) fill(x + 5, y + 5 + k, 6, 1, 0x4a4e58); }
      if (h % 11 === 3) dot(x + 3 + (h >>> 4) % 10, y + 3 + (h >>> 8) % 10, 0x7a8088);
    }
    for (let y = T + 40; y < B; y += 64) fill(L, y, R - L, 2, 0xe8c838);
  },
  /** New Mexico desert: dusty tan ground with pebbles, cracks in the dried earth and little tufts of dry scrub. */
  breakingbaddesert(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xc89a6a);
    for (let y = T, r = 0; y < B; y += 8, r++) for (let x = from(L, 12, (r % 2) * 6); x < R; x += 12) {
      const h = hash(x, r + 43);
      if (h % 3 === 0) dot(x + (h >>> 3) % 12, y + (h >>> 6) % 8, 0xa87a4e);
      if (h % 4 === 1) dot(x + (h >>> 5) % 12, y + (h >>> 9) % 8, 0xdcb488);
      if (h % 13 === 2) { const cx = x + 2, cy = y + 3; dot(cx, cy, 0x9a6e44); dot(cx + 1, cy + 1, 0x9a6e44); dot(cx + 2, cy + 1, 0x9a6e44); dot(cx + 3, cy + 2, 0x9a6e44); }
      if (h % 17 === 5) { dot(x + 4, y + 4, 0x8a8a4a); dot(x + 3, y + 3, 0x9a9a5a); dot(x + 5, y + 3, 0x7a7a3a); }
    }
  },
  /** The Palace Arcade's carpet: near-black with bright neon squiggles, triangles and rings in pink, cyan and yellow scattered over it. */
  strangerarcade(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1a1430);
    const cols = [0xf04aa8, 0x3ad8e8, 0xf2d040, 0x8a5af0];
    for (let y = T, r = 0; y < B; y += 10, r++) for (let x = from(L, 18, (r % 2) * 9); x < R; x += 18) {
      const h = hash(x, r + 7), c = cols[h % 4], sx = x + (h >>> 3) % 8, sy = y + (h >>> 6) % 5;
      switch ((h >>> 9) % 3) {
        case 0: dot(sx, sy, c); dot(sx + 1, sy + 1, c); dot(sx + 2, sy, c); dot(sx + 3, sy + 1, c); break;
        case 1: dot(sx + 1, sy, c); dot(sx, sy + 2, c); dot(sx + 2, sy + 2, c); dot(sx + 1, sy + 2, c); break;
        default: dot(sx, sy, c); dot(sx + 2, sy, c); dot(sx + 1, sy - 1, c); dot(sx + 1, sy + 1, c);
      }
    }
  },
  /** The Upside Down's ground: dark ashen earth threaded with fleshy black-red vines, grey spores lying in drifts. */
  strangerupside(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2a2430);
    for (let y = T, r = 0; y < B; y += 6, r++) for (let x = from(L, 10, (r % 2) * 5); x < R; x += 10) {
      const h = hash(x, r + 29);
      dot(x + (h % 10), y + ((h >>> 4) % 6), 0x4a4450);
      if (h % 5 === 0) { const vx = x + (h >>> 3) % 6; dot(vx, y + 1, 0x4a1e24); dot(vx + 1, y + 2, 0x5a2430); dot(vx + 2, y + 2, 0x4a1e24); dot(vx + 3, y + 3, 0x3a1a20); }
      if (h % 7 === 2) dot(x + 2, y + 3, 0x8a8490, 0.7);
    }
  },
  /** Winterfell's great hall: big dark blue-grey slate flags worn smooth, in staggered rows, with a few dry rushes strewn against the cold. */
  gotwinterfell(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2a2e34);
    for (let y = T, r = 0; y < B; y += 14, r++) for (let x = from(L, 24, (r % 2) * 12); x < R; x += 24) {
      const h = hash(x, r + 83), tone = [0x4e545c, 0x555b63, 0x484e56, 0x5a6068][h % 4];
      fill(x + 1, y + 1, 23, 13, tone); fill(x + 1, y + 1, 23, 1, shaded(tone, 1.18)); fill(x + 1, y + 13, 23, 1, shaded(tone, 0.82));
      if (h % 3 === 0) fill(x + 4 + (h >>> 5) % 10, y + 4 + (h >>> 9) % 6, 6, 1, shaded(tone, 1.08), 0.8);
      if (h % 7 === 0) { const rx = x + 3 + (h >>> 4) % 12, ry = y + 4 + (h >>> 8) % 6; line(rx, ry, rx + 5, ry + 1, 0xb8a46a); line(rx + 1, ry + 2, rx + 6, ry, 0xa8945a); }
    }
  },
  /** The Red Keep: polished marble in a checker of deep red and warm cream, a thin gold line between the squares. */
  gotredkeep(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xc8a050);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 12), c = 0; x < R; x += 12, c++) {
      const red = ((Math.floor((x - L) / 12) + r) % 2) === 0, tone = red ? 0xa84a3a : 0xe8dcc4;
      fill(x + 1, y + 1, 11, 11, tone); fill(x + 1, y + 1, 11, 1, shaded(tone, 1.12));
      const h = hash(x, r + 3); if (h % 3 === 0) line(x + 2, y + 3 + h % 6, x + 8, y + 2 + (h >>> 3) % 8, shaded(tone, 0.88));
    }
  },
  /** Springfield Nuclear Power Plant: scuffed pale green-grey vinyl tiles with a yellow safety line now and then and the odd green glowing drip. */
  simpsonsplant(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x7a8870);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 12); x < R; x += 12) {
      const h = hash(x, r + 47), tone = h % 3 ? 0x9aa890 : 0x92a088;
      fill(x + 1, y + 1, 11, 11, tone); fill(x + 1, y + 1, 11, 1, 0xaab8a0);
      if (h % 23 === 0) { dot(x + 5, y + 6, 0x7aff5a); dot(x + 6, y + 6, 0x5ae84a); dot(x + 5, y + 7, 0x5ae84a, 0.7); }
      if (h % 9 === 4) dot(x + (h >>> 4) % 10 + 1, y + (h >>> 8) % 10 + 1, 0x7a8870);
    }
    for (let y = T + 46; y < B; y += 72) fill(L, y, R - L, 2, 0xe8c838);
  },
  /** The Simpsons' living room carpet: a worn tan wall-to-wall carpet with a faint darker flecking and a few lighter scuffs. */
  simpsonsliving(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xc8a878);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 6, (r % 3) * 2); x < R; x += 6) {
      const h = hash(x, r + 61);
      if (h % 3 === 0) dot(x + (h % 6), y + ((h >>> 3) % 4), 0xb89868);
      if (h % 11 === 0) dot(x + (h >>> 4) % 6, y + ((h >>> 7) % 4), 0xd8bc90);
    }
  },
  /** MI6 headquarters: a deep navy carpet in big square tiles laid in alternating directions, a faint grey pinstripe in each. */
  bondmi6(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x222a3c);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 16), c = 0; x < R; x += 16, c++) {
      const across = (Math.floor((x - L) / 16) + r) % 2;
      fill(x + 1, y + 1, 15, 15, across ? 0x2a3448 : 0x283244);
      for (let k = 3; k < 15; k += 4) { if (across) fill(x + 1, y + k, 15, 1, 0x34405a, 0.6); else fill(x + k, y + 1, 1, 15, 0x34405a, 0.6); }
    }
  },
  /** A villain's lair: polished black stone tiles reflecting faint light, with a strip of red guide lights set into the floor every few rows. */
  bondlair(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x0c0c10);
    for (let y = T, r = 0; y < B; y += 14, r++) for (let x = from(L, 20, (r % 2) * 10); x < R; x += 20) {
      fill(x + 1, y + 1, 19, 13, 0x16161c); fill(x + 1, y + 1, 19, 1, 0x26262e); fill(x + 3, y + 3, 6, 1, 0x2a2a34, 0.6);
    }
    for (let y = T + 34; y < B; y += 56) for (let x = from(L, 8); x < R; x += 8) { dot(x + 3, y, 0xe82a2a); if (isNight) glowLate(() => dot(x + 3, y, 0xff4a4a)); }
  },
  /** A Fremen sietch: floors cut from warm orange-brown rock in rough square slabs, worn smooth, a woven rug strip in rust and indigo every few rows. */
  dunesietch(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x7a5434);
    for (let y = T, r = 0; y < B; y += 14, r++) for (let x = from(L, 16, (r % 2) * 8); x < R; x += 16) {
      const h = hash(x, r + 17), tone = [0xa8784a, 0xb08250, 0x9c6e44, 0xb48a58][h % 4];
      fill(x + 1, y + 1, 15, 13, tone); fill(x + 1, y + 1, 15, 1, shaded(tone, 1.12)); fill(x + 1, y + 13, 15, 1, shaded(tone, 0.84));
      if (h % 6 === 0) dot(x + 3 + (h >>> 4) % 10, y + 4 + (h >>> 8) % 7, shaded(tone, 0.8));
    }
    for (let y = T + 28; y < B; y += 70) { fill(L, y, R - L, 6, 0x8a3a24); for (let x = from(L, 6); x < R; x += 6) { dot(x + 2, y + 2, 0x2a3a6a); dot(x + 3, y + 3, 0x2a3a6a); dot(x + 1, y + 3, 0xd8a868); } fill(L, y, R - L, 1, 0xa84a2e); }
  },
  /** The open desert of Arrakis: deep orange sand rippled into long wavy lines by the wind, a darker trough here and there. */
  dunesand(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xd8a060);
    for (let y = T; y < B; y += 4) {
      const ph = y * 0.37;
      for (let x = L; x < R; x += 2) { const yy = y + Math.round(Math.sin(x * 0.12 + ph) * 1.5); dot(x, yy, 0xc88a4e); if ((x + y) % 7 === 0) dot(x + 1, yy - 1, 0xe8b878); }
    }
  },
  /** A ship's deck: long sea-weathered grey-brown planks running across, caulked with dark seams and studded with rows of trenail pegs. */
  onepiecedeck(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x3e3024);
    for (let y = T, r = 0; y < B; y += 7, r++) for (let x = from(L, 48, (r * 17) % 48) - 48; x < R; x += 48) {
      const tone = [0x8a7058, 0x947a60, 0x7e664e][hash(x, r) % 3];
      fill(x + 1, y, 47, 6, tone); fill(x + 1, y, 47, 1, shaded(tone, 1.12)); fill(x + 1, y + 5, 47, 1, shaded(tone, 0.86));
      dot(x + 3, y + 3, 0x6a4424); dot(x + 44, y + 3, 0x6a4424);
    }
  },
  /** The Thousand Sunny's lawn deck: bright green grass edged by planks, little tufts and a clover flower here and there. */
  onepiecelawn(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x6ab84a);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 5, (r % 2) * 2); x < R; x += 5) {
      const h = hash(x, r + 9);
      dot(x + (h % 5), y + ((h >>> 3) % 4), h % 2 ? 0x5aa83a : 0x8ad06a);
      if (h % 31 === 0) { dot(x + 1, y + 1, WHITE); dot(x + 2, y + 1, 0xf2d040); }
    }
  },
  /** The Royal Capital's floors: pale gold-grey stone in big slabs, faint veins of golden light from the Erdtree's roots running through them. */
  eldenringleyndell(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x9a8e74);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 24, (r % 2) * 12); x < R; x += 24) {
      const h = hash(x, r + 37), tone = [0xc8bca0, 0xc0b498, 0xd0c4a8][h % 3];
      fill(x + 1, y + 1, 23, 15, tone); fill(x + 1, y + 1, 23, 1, shaded(tone, 1.1)); fill(x + 1, y + 15, 23, 1, shaded(tone, 0.86));
      if (h % 7 === 0) { let vx = x + 3 + (h >>> 4) % 10, vy = y + 3; for (let k = 0; k < 10; k++) { dot(vx, vy, 0xe8c860, 0.7); vx += (h >>> k) % 2 ? 1 : 0; vy += 1; if (vy > y + 14) break; } }
    }
  },
  /** Limgrave: soft green meadow grass dotted with small yellow and white flowers and a few fallen golden Erdtree leaves. */
  eldenringlimgrave(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x6a8a3a);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 5, (r % 2) * 2); x < R; x += 5) {
      const h = hash(x, r + 71);
      dot(x + (h % 5), y + ((h >>> 3) % 4), h % 2 ? 0x5a7a2e : 0x7a9a4a);
      if (h % 37 === 0) dot(x + 2, y + 1, 0xf2d040);
      if (h % 41 === 3) dot(x + 1, y + 2, WHITE);
      if (h % 53 === 7) { dot(x + 2, y + 2, 0xe8b830); dot(x + 3, y + 2, 0xd8a028); }
    }
  },
  /** Lumon's macrodata floor: deep institutional teal low-pile carpet, flecked, with a faint sterile grid ruled across it. */
  sevcarpet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2f6456); // the deep teal carpet
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 5, (r % 2) * 2); x < R; x += 5) {
      const h = hash(x, r + 11);
      dot(x + (h % 5), y + ((h >>> 3) % 4), h % 2 ? 0x2a5a4d : 0x356f60); // flecked wool
    }
    for (let x = from(L, 40, 0); x < R; x += 40) fill(x, T, 1, B - T, 0x274f45, 0.55); // the faint grid
    for (let y = from(T, 40, 0); y < B; y += 40) fill(L, y, R - L, 1, 0x274f45, 0.55);
  },
  /** The severed hallways' floor: off-white vinyl tiles in a seamed grid, a couple of shades mixed for that clinical glare. */
  sevtile(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xdfe4df);
    for (let y = T; y < B; y += 16) for (let x = from(L, 16, 0); x < R; x += 16) {
      const h = hash(x, y + 5), tone = [0xdfe4df, 0xd8ddd7, 0xe6ebe5][h % 3];
      fill(x, y, 15, 15, tone); fill(x, y, 15, 1, 0xeef2ec); fill(x, y + 15, 15, 1, 0xc4cac3); fill(x + 15, y, 1, 15, 0xc4cac3); // tile with its seams
    }
  },
  /** Dunder Mifflin's floor: muted grey-beige commercial carpet, flecked, seamed into big carpet tiles. */
  offcarpet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x9a978a);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 5, (r % 2) * 2); x < R; x += 5) { const h = hash(x, r + 9); dot(x + (h % 5), y + ((h >>> 3) % 4), h % 2 ? 0x8f8c7f : 0xa5a295); }
    for (let x = from(L, 48, 0); x < R; x += 48) fill(x, T, 1, B - T, 0x87857a, 0.5); // carpet-tile seams
  },
  /** The break room's floor: off-white speckled linoleum in a tiled grid. */
  offlino(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xdedbce);
    for (let y = T; y < B; y += 12) for (let x = from(L, 12, 0); x < R; x += 12) { const h = hash(x, y + 3); fill(x, y, 11, 11, h % 3 ? 0xdedbce : 0xd4d1c2); fill(x, y, 11, 1, 0xeceae0); fill(x, y + 11, 11, 1, 0xc4c1b4); }
    for (let y = T, r = 0; y < B; y += 2, r++) for (let x = from(L, 3, 0); x < R; x += 3) { if (hash(x, r + 5) % 7 === 0) dot(x, y, 0xb8b5a8, 0.5); }
  },
  /** The betting den's floor: dark grimy wood planks, horizontal, with worn board ends. */
  peakyboards(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x4a3526);
    for (let y = T, r = 0; y < B; y += 6, r++) { fill(L, y, R - L, 1, 0x3a2a1c); for (let x = from(L, 7, (r % 2) * 3); x < R; x += 7) { const h = hash(x, r + 7); dot(x + h % 6, y + 2 + (h >>> 3) % 3, h % 2 ? 0x55412e : 0x40301f); } }
    for (let x = from(L, 34, 0); x < R; x += 34) fill(x, T, 1, B - T, 0x2e2016, 0.6); // board ends
  },
  /** A wet Birmingham cobblestone street: rounded setts with a cold sheen. */
  peakycobble(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x4a4842);
    for (let y = T, r = 0; y < B; y += 5, r++) for (let x = from(L, 7, (r % 2) * 3); x < R; x += 7) { const h = hash(x, r + 3), tone = [0x55534c, 0x4a4842, 0x5f5d55][h % 3]; fill(x, y, 6, 4, tone); fill(x, y, 6, 1, shaded(tone, 1.12)); fill(x, y + 4, 6, 1, 0x35332e); if (h % 11 === 0) dot(x + 2, y + 1, 0x8a94a0, 0.4); }
  },
  /** The agency floor: warm olive-gold low pile flecked, with a faint grid of gold diamonds. */
  madmcarpet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x8a7836);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 5, (r % 2) * 2); x < R; x += 5) { const h = hash(x, r + 4); dot(x + h % 5, y + (h >>> 3) % 4, h % 2 ? 0x7e6c2e : 0x96823e); }
    for (let y = T; y < B; y += 12) for (let x = from(L, 12, 0); x < R; x += 12) { dot(x + 6, y + 6, 0xb89a46, 0.5); dot(x + 5, y + 6, 0xb89a46, 0.3); dot(x + 7, y + 6, 0xb89a46, 0.3); dot(x + 6, y + 5, 0xb89a46, 0.3); dot(x + 6, y + 7, 0xb89a46, 0.3); } // gold diamonds
  },
  /** A warm herringbone parquet of teak blocks in a couple of tones. */
  madmparquet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x8a5a32);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 8, (r % 2) * 4); x < R; x += 8) { const h = hash(x, r + 6), tone = [0x8a5a32, 0x9a6a3a, 0x7a4e2a][h % 3]; fill(x, y, 7, 3, tone); fill(x, y, 7, 1, shaded(tone, 1.12)); fill(x, y + 3, 7, 1, 0x5a3b25); fill(x + (r % 2 ? 0 : 3), y, 1, 3, 0x6a4628, 0.6); }
  },
  /** The playground field: dusty tan dirt, flecked, with faint painted game lines. */
  squidfield(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xcbb089);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 5, (r % 2) * 2); x < R; x += 5) { const h = hash(x, r + 8); dot(x + h % 5, y + (h >>> 3) % 4, h % 2 ? 0xbfa47d : 0xd6bc94); }
    for (let x = from(L, 60, 10); x < R; x += 60) fill(x, T, 2, B - T, 0xe8e2d0, 0.5); // painted lines
  },
  /** The dormitory floor: teal tiles in two shades with seams. */
  squidteal(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x3a8a8a);
    for (let y = T; y < B; y += 16) for (let x = from(L, 16, 0); x < R; x += 16) { fill(x, y, 15, 15, (hash(x, y) % 2) ? 0x3a8a8a : 0x359080); fill(x, y, 15, 1, 0x4aa89a); fill(x, y + 15, 15, 1, 0x2e7470); fill(x + 15, y, 1, 15, 0x2e7470); }
  },
  /** The Vought lobby floor: polished white marble in big slabs with grey veins and a faint red inlay line. */
  boysmarble(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xe6e4de);
    for (let y = T; y < B; y += 16) for (let x = from(L, 16, 0); x < R; x += 16) { const h = hash(x, y + 2), tone = [0xe6e4de, 0xdedcd4, 0xeceae2][h % 3]; fill(x, y, 15, 15, tone); fill(x, y, 15, 1, 0xf2f0ea); fill(x, y + 15, 15, 1, 0xcfcdc4); fill(x + 15, y, 1, 15, 0xcfcdc4); if (h % 4 === 0) { let vx = x + 3 + h % 9, vy = y + 2; for (let k = 0; k < 10; k++) { dot(vx, vy, 0xc8c6bc, 0.5); vx += (h >>> k) % 2; vy += 1; if (vy > y + 13) break; } } }
    for (let x = from(L, 48, 0); x < R; x += 48) fill(x, T, 1, B - T, 0xd21f2a, 0.3); // a red inlay line
  },
  /** A rich red executive carpet, flecked, seamed into broad runs. */
  boyscarpet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x9a1a24);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 5, (r % 2) * 2); x < R; x += 5) { const h = hash(x, r + 5); dot(x + h % 5, y + (h >>> 3) % 4, h % 2 ? 0x8a1620 : 0xac2430); }
    for (let x = from(L, 40, 0); x < R; x += 40) fill(x, T, 1, B - T, 0x7a1018, 0.5);
  },
  /** The vault's floor: riveted steel plates in a staggered grid, two tones. */
  falloutvault(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x5a5e64);
    for (let y = T; y < B; y += 8) for (let x = from(L, 8, (((y - T) / 8 | 0) % 2) * 4); x < R; x += 8) { const h = hash(x, y + 3); fill(x, y, 7, 7, h % 3 ? 0x5a5e64 : 0x52565c); fill(x, y, 7, 1, 0x6e727c); fill(x, y + 7, 7, 1, 0x3e4248); dot(x + 3, y + 3, 0x7a7e84, 0.6); dot(x + 1, y + 1, 0x4a4e54); } // riveted plates
  },
  /** Cracked irradiated concrete, flecked, with a few long cracks. */
  falloutconcrete(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x8a8a80);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 5, (r % 2) * 2); x < R; x += 5) { const h = hash(x, r + 7); dot(x + h % 5, y + (h >>> 3) % 4, h % 2 ? 0x7e7e74 : 0x96968c); }
    for (let i = 0; i < 6; i++) { let cx = from(L, 40, i * 7) + (hash(L, i) % 30), cy = T + (hash(cx, i) % Math.max(1, B - T - 6)); for (let k = 0; k < 8; k++) { dot(cx, cy, 0x6a6a60, 0.6); cx += (hash(cx, k) % 3) - 1; cy += 1; } } // cracks
  },
  /** The garage floor: grey concrete, flecked, with dark oil stains and slab seams. */
  rickgarage(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x7a7e78);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 5, (r % 2) * 2); x < R; x += 5) { const h = hash(x, r + 6); dot(x + h % 5, y + (h >>> 3) % 4, h % 2 ? 0x6e726c : 0x868a84); }
    for (let i = 0; i < 4; i++) { const sx = from(L, 70, i * 11) + hash(L, i) % 40, sy = T + hash(sx, i) % Math.max(1, B - T - 8); disc(sx, sy, 3 + hash(sx, 1) % 3, 0x3a3e3a, 0.4); } // oil stains
    for (let x = from(L, 48, 0); x < R; x += 48) fill(x, T, 1, B - T, 0x6a6e68, 0.5); // slab seams
  },
  /** Alien portal ground: mottled green with glowing goo pools. */
  rickportal(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2a5a3a);
    for (let y = T, r = 0; y < B; y += 4, r++) for (let x = from(L, 5, (r % 2) * 2); x < R; x += 5) { const h = hash(x, r + 9); dot(x + h % 5, y + (h >>> 3) % 4, h % 2 ? 0x246a34 : 0x3a8a4a); }
    for (let i = 0; i < 5; i++) { const px = from(L, 50, i * 9) + hash(L, i + 3) % 30, py = T + hash(px, i) % Math.max(1, B - T - 6); disc(px, py, 2, 0x5af06a, 0.3); dot(px, py, 0x7af08a, 0.4); } // goo pools
  },
  /** A glossy white minimalist floor in big seamed tiles with soft reflection streaks. */
  mirrorfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xeef0f2);
    for (let y = T; y < B; y += 20) for (let x = from(L, 20, 0); x < R; x += 20) { fill(x, y, 19, 19, (hash(x, y) % 2) ? 0xeef0f2 : 0xe8eaee); fill(x, y, 19, 1, 0xf6f8fa); fill(x, y + 19, 19, 1, 0xd8dce0); fill(x + 19, y, 1, 19, 0xd8dce0); }
    for (let x = from(L, 40, 10); x < R; x += 40) fill(x, T, 2, B - T, 0xf8fafc, 0.4); // reflection streaks
  },
  /** A dark reflective floor in seamed tiles with a cold reflection. */
  mirrordark(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1c1e24);
    for (let y = T; y < B; y += 20) for (let x = from(L, 20, 0); x < R; x += 20) { fill(x, y, 19, 19, (hash(x, y) % 2) ? 0x1c1e24 : 0x202228); fill(x, y, 19, 1, 0x2a2d35); fill(x, y + 19, 19, 1, 0x141519); }
    for (let x = from(L, 44, 14); x < R; x += 44) fill(x, T, 1, B - T, 0x3a4a6a, 0.3); // a cold reflection
  },
  /** The host-repair lab floor: clean white tiles with a faint light strip. */
  westlab(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xe8eae8);
    for (let y = T; y < B; y += 16) for (let x = from(L, 16, 0); x < R; x += 16) { fill(x, y, 15, 15, (hash(x, y) % 3) ? 0xe8eae8 : 0xe2e4e2); fill(x, y, 15, 1, 0xf2f4f2); fill(x, y + 15, 15, 1, 0xd4d6d4); fill(x + 15, y, 1, 15, 0xd4d6d4); }
    for (let x = from(L, 80, 20); x < R; x += 80) fill(x, T, 2, B - T, 0xeaf2f0, 0.5); // a light strip
  },
  /** The saloon floor: warm wooden planks, worn, with dark board ends. */
  westsaloon(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x7a5230);
    for (let y = T, r = 0; y < B; y += 6, r++) { fill(L, y, R - L, 1, 0x5a3a1e); for (let x = from(L, 7, (r % 2) * 3); x < R; x += 7) { const h = hash(x, r + 4); dot(x + h % 6, y + 2 + (h >>> 3) % 3, h % 2 ? 0x8a5e38 : 0x6a4626); } }
    for (let x = from(L, 40, 0); x < R; x += 40) fill(x, T, 1, B - T, 0x4a3018, 0.6); // board ends
  },
  /** A garage floor: grey sealed concrete in big slabs with a yellow lane stripe and a couple of oil stains. */
  bttfgaragefloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x8f8c86);
    for (let y = T, r = 0; y < B; y += 20, r++) { fill(L, y, R - L, 1, 0x7a776f); for (let x = from(L, 40, (r % 2) * 20); x < R; x += 40) fill(x, y, 1, 20, 0x7a776f); }
    for (let x = from(L, 7); x < R; x += 7) for (let y = T; y < B; y += 7) if (hash(x, y) % 11 === 0) dot(x, y, 0x807d77);
    for (let x = from(L, 6); x < R; x += 6) fill(x, T + Math.floor((B - T) * 0.62), 3, 2, 0xd8b43a, 0.7); // the lane stripe
    for (const [sx, sy] of [[L + 30, T + 24], [L + 110, T + 70]]) if (sx < R && sy < B) disc(sx, sy, 5, 0x3a3730, 0.4);
  },
  /** A classic diner floor: black-and-white checker tiles with a soft chrome sheen along the grout. */
  bttfdinerfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xe8e4da);
    for (let y = T, r = 0; y < B; y += 10, r++) for (let x = from(L, 10, (r % 2) * 10); x < R; x += 20) fill(x, y, 10, 10, 0x24262c);
    for (let x = from(L, 10); x < R; x += 10) fill(x, T, 1, B - T, 0xf2eee6, 0.3);
  },
  /** The visitor-center floor: big polished sandstone tiles, a faint circular emblem inlaid here and there. */
  jpvisitorfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xb9b2a2);
    for (let y = T, r = 0; y < B; y += 18, r++) for (let x = from(L, 28, (r % 2) * 14); x < R; x += 28) {
      const tone = [0xc6bfae, 0xbcb5a4, 0xccc5b4][hash(x, r + 5) % 3];
      fill(x + 1, y + 1, 27, 17, tone); fill(x + 1, y + 1, 27, 1, shaded(tone, 1.1)); fill(x + 1, y + 17, 27, 1, shaded(tone, 0.86));
      if ((x + r) % 3 === 0) ring(x + 14, y + 9, 4, 0xa89a6a, 0.4);
    }
  },
  /** A jungle path: dark wet earth with grass tufts, scattered leaves and the odd puddle. */
  jpjunglefloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x4a5a30);
    for (let x = from(L, 5); x < R; x += 5) for (let y = T; y < B; y += 5) { const h = hash(x, y); if (h % 3 === 0) dot(x, y, 0x3a4a28); else if (h % 7 === 0) dot(x, y, 0x6a8a3a); }
    for (let x = from(L, 11); x < R; x += 11) for (let y = T + 4; y < B; y += 9) { fill(x, y - 2, 1, 3, 0x6a8a3a); dot(x + 1, y - 1, 0x7a9a4a); }
    for (const p of [[L + 40, T + 20], [L + 120, T + 60]]) if (p[0] < R && p[1] < B) disc(p[0], p[1], 4, 0x3a4a3a, 0.4);
  },
  /** The lobby floor: pale marble in big diamond tiles with thin gold inlay lines and a soft polish. */
  wicklobbyfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xe6e2da);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 24, (r % 2) * 12); x < R; x += 24) {
      const tone = [0xeae6de, 0xdedad0, 0xf0ece4][hash(x, r + 3) % 3];
      fill(x + 1, y + 1, 23, 15, tone); fill(x + 1, y + 1, 23, 1, shaded(tone, 1.06)); fill(x + 1, y + 15, 23, 1, shaded(tone, 0.9));
      fill(x, y, 1, 16, 0xcfae5e, 0.4); fill(x, y, 24, 1, 0xcfae5e, 0.4);
    }
  },
  /** The vault floor: dark polished stone strewn with loose gold coins that catch the light. */
  wickvaultfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2a241c);
    for (let x = from(L, 6); x < R; x += 6) for (let y = T; y < B; y += 6) { const h = hash(x, y); if (h % 5 === 0) { dot(x, y, 0xd6b25e); dot(x, y + 1, 0x9a7e3a); } else if (h % 11 === 0) dot(x + 1, y, 0xf2d888); }
  },
  /** The shop floor: warm herringbone parquet, little chevrons of light and dark oak. */
  kingsmanshopfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x8a6038);
    for (let y = T, r = 0; y < B; y += 6, r++) for (let x = from(L, 8, (r % 2) * 4); x < R; x += 8) {
      const tone = [0x9a6c40, 0x7e5530, 0xa8784a][hash(x, r) % 3];
      fill(x, y, 7, 3, tone); fill(x, y, 7, 1, shaded(tone, 1.12)); fill(x, y + 2, 7, 1, shaded(tone, 0.82));
    }
  },
  /** The HQ floor: dark slate with a round gold Kingsman-style seal inlaid here and there. */
  kingsmanhqfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x343e46);
    for (let x = from(L, 7); x < R; x += 7) for (let y = T; y < B; y += 7) if (hash(x, y) % 9 === 0) dot(x, y, 0x3e4850);
    for (let x = from(L, 90); x < R; x += 90) for (let y = T + 30; y < B; y += 70) if (x + 10 < R && y + 10 < B) { ring(x, y, 7, 0xc8a24a, 0.5); ring(x, y, 4, 0xc8a24a, 0.4); dot(x, y, 0xe8c878, 0.5); }
  },
  /** The river-room floor: lush green banks with a rich brown chocolate stream running in waves through them. */
  wonkariverfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x4a8a3a);
    for (let x = from(L, 6); x < R; x += 6) for (let y = T; y < B; y += 6) if (hash(x, y) % 4 === 0) dot(x, y, 0x3a7a2e);
    const cy = T + Math.floor((B - T) * 0.5);
    for (let x = L; x < R; x++) { const wv = Math.round(Math.sin(x * 0.12 + now) * 2); fill(x, cy + wv - 3, 1, 7, 0x5a3418); dot(x, cy + wv - 3, 0x7a4a26); if ((x + Math.floor(now * 4)) % 9 === 0) dot(x, cy + wv - 2, 0x9a6a3e); }
  },
  /** The candy-room floor: glossy pastel tiles -- pink, mint and lemon -- like squares of fondant. */
  wonkacandyfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xf0b8d0);
    const cols = [0xf6c0d8, 0xb8ecd4, 0xf6ecb8];
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 12, (r % 2) * 6); x < R; x += 12) { const c = cols[hash(x, r) % 3]; fill(x, y, 11, 11, c); fill(x, y, 11, 1, shaded(c, 1.08)); fill(x, y + 10, 11, 1, shaded(c, 0.92)); dot(x + 2, y + 2, WHITE, 0.3); }
  },
  /** The lobby floor: a crisp pink-and-cream diagonal checker, buffed to a soft sheen. */
  weslobbyfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xf2ead2);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 12, (r % 2) * 12); x < R; x += 24) { fill(x, y, 12, 12, 0xf0c0c8); fill(x, y, 12, 1, 0xf8d0d6); }
    for (let x = from(L, 12); x < R; x += 12) fill(x, T, 1, B - T, 0xd8a0a8, 0.25);
  },
  /** The corridor carpet: deep red with a repeating cream-and-mustard diamond motif. */
  wescorridorfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xb84a3a);
    for (let y = T, r = 0; y < B; y += 10, r++) for (let x = from(L, 14, (r % 2) * 7); x < R; x += 14) { dot(x, y + 4, 0xf2ead2); dot(x - 1, y + 4, 0xd8a848); dot(x + 1, y + 4, 0xd8a848); dot(x, y + 3, 0xd8a848); dot(x, y + 5, 0xd8a848); }
    for (let x = from(L, 7); x < R; x += 7) fill(x, T, 1, B - T, 0xa03a2c, 0.3);
  },
  /** The HQ floor: glossy off-white panels with thin dark seams and a faint reflective sheen. */
  mibhqfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xdfe2e6);
    for (let y = T, r = 0; y < B; y += 16, r++) { fill(L, y, R - L, 1, 0xc4c8ce); for (let x = from(L, 22, (r % 2) * 11); x < R; x += 22) { fill(x, y, 1, 16, 0xc4c8ce); dot(x + 2, y + 2, WHITE, 0.4); } }
  },
  /** The tunnel floor: dark steel grating in strips, the odd rivet catching a cold glint. */
  mibtunnelfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x44484e);
    for (let y = T; y < B; y += 3) fill(L, y, R - L, 1, 0x34383e);
    for (let x = from(L, 6); x < R; x += 6) { fill(x, T, 1, B - T, 0x2e3238); for (let y = T + 2; y < B; y += 12) dot(x, y, 0x6a6e76); }
  },
  /** The Grid floor: near-black glass scored by a perspective grid of glowing cyan lines. */
  trongridfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x0a1016);
    for (let y = T; y < B; y += 8) fill(L, y, R - L, 1, 0x163848, 0.8);
    for (let x = from(L, 10); x < R; x += 10) fill(x, T, 1, B - T, 0x163848, 0.7);
    for (let x = from(L, 10); x < R; x += 10) for (let y = T; y < B; y += 8) dot(x, y, 0x3ad0e8, 0.6);
  },
  /** The arena floor: dark panels with faint cyan seams and two glowing light-trail stripes crossing it. */
  tronarenafloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x141c28);
    for (let y = T, r = 0; y < B; y += 14, r++) for (let x = from(L, 18, (r % 2) * 9); x < R; x += 18) { fill(x, y, 17, 13, 0x101822); fill(x, y, 17, 1, 0x1e2a38); }
    fill(L, T + Math.floor((B - T) * 0.35), R - L, 1, 0x3ad0e8, 0.6);
    fill(L, T + Math.floor((B - T) * 0.7), R - L, 1, 0xe8782e, 0.6);
  },
  /** The tournament ring floor: pale sand-coloured boards with a bold red boundary line running through. */
  dbzdojofloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xd8b878);
    for (let y = T, r = 0; y < B; y += 8, r++) { fill(L, y, R - L, 1, 0xc0a058); for (let x = from(L, 40, (r % 2) * 20); x < R; x += 40) fill(x, y, 1, 8, 0xc0a058); }
    fill(L, T + Math.floor((B - T) * 0.5) - 1, R - L, 3, 0xd83020); fill(L, T + Math.floor((B - T) * 0.5) - 1, R - L, 1, 0xf05a40);
  },
  /** The Capsule Corp lab floor: clean pale-blue tiles with thin white grout and a soft sheen. */
  dbzlabfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xd8e0ea);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 14, (r % 2) * 7); x < R; x += 14) { const c = (hash(x, r) % 4 === 0) ? 0xcad6e4 : 0xdfe6ee; fill(x, y, 13, 11, c); fill(x, y, 13, 1, 0xeef2f8); dot(x + 2, y + 2, WHITE, 0.3); }
  },
  /** The ship deck: dark grey metal panels with bolted seams and the odd caution stripe. */
  amongusshipfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x3a4450);
    for (let y = T, r = 0; y < B; y += 16, r++) { fill(L, y, R - L, 1, 0x2a333c); for (let x = from(L, 26, (r % 2) * 13); x < R; x += 26) { fill(x, y, 1, 16, 0x2a333c); dot(x + 1, y + 1, 0x56626e); dot(x + 1, y + 14, 0x56626e); } }
    for (let x = from(L, 90); x < R; x += 90) for (let y = T + 40; y < B; y += 80) if (x + 8 < R && y < B) { fill(x, y, 8, 2, 0xe8c838, 0.5); fill(x, y + 3, 8, 2, 0x2a2e36, 0.5); }
  },
  /** The cafeteria floor: teal-tinted metal tiles with a lighter ring inlaid where the meeting table stands. */
  amonguscafefloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x2a5a5a);
    for (let y = T, r = 0; y < B; y += 14, r++) for (let x = from(L, 14, (r % 2) * 7); x < R; x += 14) { const c = (hash(x, r) % 4 === 0) ? 0x245050 : 0x2e6262; fill(x, y, 13, 13, c); fill(x, y, 13, 1, 0x3a7272); }
    for (let x = from(L, 110); x < R; x += 110) for (let y = T + 50; y < B; y += 90) if (x < R && y < B) ring(x, y, 9, 0x4a9090, 0.5);
  },
  /** Wet neon asphalt. Dark rain-slicked tarmac with faint kerb lines and long smeared reflections of pink and cyan signage pooling across it, as in a midnight alley. */
  tokyofloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x15181f);
    for (let y = T; y < B; y += 2) for (let x = from(L, 3, (y % 4) ? 1 : 0); x < R; x += 3) { const h = hash(x, y); if (h % 7 === 0) dot(x, y, h % 2 ? 0x1e222c : 0x101218); }
    // Smeared neon reflections: vertical streaks of pink and cyan, brightest near where a sign would hang
    for (let x = from(L, 18); x < R; x += 18) {
      const h = hash(x, 3), c = h % 2 ? 0xff4ab8 : 0x3ae8ff, len = 6 + h % 10;
      for (let k = 0; k < len; k++) dot(x + (h % 3) - 1, T + 4 + k, c, 0.18 - k * 0.012);
    }
    for (let x = from(L, 16, 6); x < R; x += 16) fill(x, T, 1, B - T, WHITE, 0.03);
  },
  /** Tatami mat. Pale woven rush mats laid in a brick-offset grid, each panel's weave running in alternate directions and bound with a dark cloth border. */
  tokyotatami(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xb6a06a);
    for (let y = from(T, 16), r = 0; y < B; y += 16, r++) for (let x = from(L, 32, (r % 2) * 16); x < R; x += 32) {
      const horiz = (r % 2) === 0;
      for (let k = 1; k < 16; k += 2) { if (horiz) fill(x + 1, y + k, 30, 1, 0xa89058, 0.6); else fill(x + k, y + 1, 1, 15, 0xa89058, 0.6); }
      fill(x, y, 32, 1, 0x3a3228); fill(x, y, 1, 16, 0x3a3228);
      if (hash(x, r) % 4 === 0) dot(x + 8 + hash(x, y) % 12, y + 4 + hash(y, x) % 8, 0xc8b47e);
    }
  },
  /** Sandstone slabs. Big weathered limestone blocks in a brick-offset grid, pale and warm, flecked and cracked, with dark mortar joints. */
  egyptfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xc9a86a);
    for (let y = from(T, 16), r = 0; y < B; y += 16, r++) for (let x = from(L, 32, (r % 2) * 16); x < R; x += 32) {
      fill(x, y, 32, 1, 0x9a7a44); fill(x, y, 1, 16, 0x9a7a44); fill(x + 1, y + 1, 31, 1, 0xe0c488, 0.5);
      const h = hash(x, r); for (let k = 0; k < 3; k++) dot(x + 4 + (hash(x + k, y) % 24), y + 3 + (hash(y, x + k) % 11), h % 2 ? 0xb89658 : 0xd8bc80);
      if (h % 4 === 0) line(x + 6, y + 4, x + 10 + h % 8, y + 10, 0x9a7a44, 0.5);
    }
  },
  /** Lapis and gold tiles. A deep blue faience floor laid in a grid of gold bands, a small gold lotus or star tile at each crossing, polished so it catches the light. */
  egyptmosaic(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1f4e8a);
    for (let y = from(T, 20), r = 0; y < B; y += 20, r++) for (let x = from(L, 20); x < R; x += 20) {
      const h = hash(x, r + 3);
      if (h % 3 === 0) fill(x + 4, y + 4, 12, 12, 0x3a6ab0, 0.6);
      fill(x, y, 20, 1, 0xd8b24e); fill(x, y, 1, 20, 0xd8b24e);
      for (const [dx, dy] of [[0, -2], [0, 2], [-2, 0], [2, 0]]) dot(x + dx, y + dy, 0xf2d888);
      dot(x, y, 0x3ab8a0);
    }
  },
  /** Longhouse planks. Wide hand-hewn oak boards running lengthwise, dark seams between them, knots and adze marks, with a worn fur rug laid across now and then. */
  vikingfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x6a4a2e);
    for (let y = from(T, 8); y < B; y += 8) { fill(L, y, R - L, 1, 0x3a2818); fill(L, y + 1, R - L, 1, 0x7a5636, 0.5); }
    for (let y = from(T, 8); y < B; y += 8) for (let x = L; x < R; x += 1) { const h = hash(x, y); if (h % 23 === 0) fill(x, y + 2, 1 + h % 2, 4, 0x4a3220, 0.5); }
    // a few fur rugs
    for (let x = from(L, 56, 20); x < R; x += 56) { const h = hash(x, 2); if (h % 2) { fill(x, from(T, 8) + 10, 22, 6, 0x9a8a70); fill(x, from(T, 8) + 10, 22, 1, 0xb8a888); for (let k = 0; k < 10; k++) dot(x + (h + k * 7) % 22, from(T, 8) + 10 + (k % 6), 0x7a6a54); } }
  },
  /** Hearth flagstones. Big rough grey flagstones in a loose grid, soot-darkened near the middle, worn smooth and pale at the edges. */
  vikingstone(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x5a5a56);
    for (let y = from(T, 18), r = 0; y < B; y += 18, r++) for (let x = from(L, 22, (r % 2) * 11); x < R; x += 22) {
      const h = hash(x, r); fill(x, y, 22, 1, 0x3e3e3a); fill(x, y, 1, 18, 0x3e3e3a); fill(x + 1, y + 1, 21, 1, 0x6e6e68, 0.5);
      if (h % 3 === 0) fill(x + 4, y + 5, 10, 6, 0x4a4a46, 0.5);
      for (let k = 0; k < 2; k++) dot(x + 4 + (hash(x + k, y) % 14), y + 3 + (hash(y, x + k) % 12), h % 2 ? 0x6a6a64 : 0x4e4e4a);
    }
  },
  /** Carved stone slabs. Big grey temple flagstones in a tight grid, a faint carved glyph or spiral pressed into some of them, with moss creeping into the joints. */
  aztecfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x7a7268);
    for (let y = from(T, 18), r = 0; y < B; y += 18, r++) for (let x = from(L, 24, (r % 2) * 12); x < R; x += 24) {
      const h = hash(x, r); fill(x, y, 24, 1, 0x55504a); fill(x, y, 1, 18, 0x55504a); fill(x + 1, y + 1, 23, 1, 0x968c7e, 0.5);
      if (h % 3 === 0) { ring(x + 12, y + 9, 3, 0x5e584e, 0.6); dot(x + 12, y + 9, 0x5e584e); }
      else if (h % 3 === 1) for (let k = 0; k < 3; k++) fill(x + 6 + k * 4, y + 8, 2, 1, 0x5e584e, 0.5);
      if (h % 4 === 0) fill(x, y + 16, 24, 2, 0x4a5a3a, 0.4); // moss in the joint
    }
  },
  /** Mossy jungle stone. Weathered green-grey stone overgrown with moss and scattered leaf litter, lighter stone showing through in patches. */
  aztecjungle(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x4a5a3a);
    for (let y = T; y < B; y += 2) for (let x = from(L, 2, (y % 4) ? 1 : 0); x < R; x += 2) { const h = hash(x, y); if (h % 4 === 0) dot(x, y, [0x3a4a2e, 0x5a6a46, 0x6a7a52][h % 3]); }
    for (let y = from(T, 20); y < B; y += 20) for (let x = from(L, 26); x < R; x += 26) { if (hash(x, y) % 2) fill(x, y, 10, 6, 0x6a685a, 0.5); }
    for (let k = 0; k < (R - L) / 8; k++) { const lx = L + (hash(k, 3) % (R - L)), ly = T + (hash(k, 7) % (B - T)); fill(lx, ly, 2, 1, hash(k, 1) % 2 ? 0x4f9a5a : 0x8a7a3a, 0.6); }
  },
  /** Zodiac terrazzo. A dark polished floor flecked with pale chips, laid in big squares with thin brass strips and a small brass star or sun inlaid at each crossing, like an observatory hall. */
  obsfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x23283a);
    for (let y = T; y < B; y += 2) for (let x = from(L, 3, (y % 4) ? 1 : 0); x < R; x += 3) { const h = hash(x, y); if (h % 6 === 0) dot(x, y, [0x3a4052, 0x4a5066, 0x2e3346][h % 3]); }
    for (let y = from(T, 32); y < B; y += 32) for (let x = from(L, 32); x < R; x += 32) {
      fill(x, y, 32, 1, 0x8a6a2a, 0.7); fill(x, y, 1, 32, 0x8a6a2a, 0.7);
      for (const [dx, dy] of [[0, -2], [0, 2], [-2, 0], [2, 0], [-1, -1], [1, 1], [1, -1], [-1, 1]]) dot(x + dx, y + dy, 0xc8a050); dot(x, y, 0xe8c878);
    }
  },
  /** Starfield carpet. A deep midnight-blue carpet strewn with little white and gold stars of different sizes and a faint band of pale 'milky way' speckle running across. */
  obscarpet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1a2040);
    for (let k = 0; k < (R - L) * (B - T) / 60; k++) { const sx = L + hash(k, 2) % (R - L), sy = T + hash(k, 9) % (B - T), m = hash(k, 5) % 10; if (m === 0) { dot(sx, sy, 0xf2d888); for (const [dx, dy] of [[-1, 0], [1, 0], [0, -1], [0, 1]]) dot(sx + dx, sy + dy, 0xf2d888, 0.5); } else dot(sx, sy, m < 4 ? 0xf6f6f2 : 0x8a90b0, m < 4 ? 0.9 : 0.5); }
    for (let x = L; x < R; x++) { if (hash(x, 1) % 3 === 0) dot(x, T + 14 + (hash(x, 3) % 12), 0xb0b8d8, 0.25); }
  },
  /** Bamboo deck. Rows of split bamboo poles laid lengthwise, lashed with twine at intervals, pale gold with darker gaps and node rings. */
  tikifloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xb89a5a);
    for (let y = from(T, 6); y < B; y += 6) { fill(L, y, R - L, 1, 0x7a5e2a); fill(L, y + 1, R - L, 1, 0xd8bc7e, 0.6); fill(L, y + 4, R - L, 1, 0x9a7e42, 0.5); }
    for (let y = from(T, 6); y < B; y += 6) for (let x = from(L, 20, (y % 12) ? 10 : 0); x < R; x += 20) { fill(x, y, 1, 5, 0x8a6a3a); } // node rings
    for (let x = from(L, 40, 8); x < R; x += 40) fill(x, T, 2, B - T, 0x6a5228, 0.4); // twine lashings
  },
  /** Beach sand. Warm pale sand finely speckled, with a few scattered shells and a faint rake-line or two, lighter where the sun hits. */
  tikisand(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xe0cc94);
    for (let y = T; y < B; y += 2) for (let x = from(L, 2, (y % 4) ? 1 : 0); x < R; x += 2) { const h = hash(x, y); if (h % 5 === 0) dot(x, y, h % 2 ? 0xf0e0b0 : 0xc8b074); }
    for (let y = from(T, 10); y < B; y += 10) fill(L, y, R - L, 1, 0xd0ba80, 0.3); // faint rake lines
    for (let k = 0; k < (R - L) / 30; k++) { const sx = L + hash(k, 3) % (R - L), sy = T + hash(k, 7) % (B - T); dot(sx, sy, hash(k, 1) % 2 ? 0xf2d8c0 : 0xd8c0a0); dot(sx + 1, sy, 0xe8d0b8); } // shells
  },
  /** Checkerboard tile. A classic black-and-white diner floor of big square tiles in a checker pattern, the pale ones with a faint sheen and the odd scuff. */
  dinerfloor(L, T, R, B) {
    for (let y = from(T, 12), r = 0; y < B; y += 12, r++) for (let x = from(L, 12), c = 0; x < R; x += 12, c++) {
      const dark = (r + c) % 2 === 0;
      fill(x, y, 12, 12, dark ? 0x2a2a30 : 0xe8e4da);
      if (!dark) { dot(x + 2, y + 2, 0xf4f0e8, 0.6); if (hash(x, y) % 5 === 0) dot(x + 6 + hash(x, r) % 4, y + 7, 0xd0ccc0, 0.5); }
      else dot(x + 2, y + 2, 0x3a3a42, 0.6);
    }
  },
  /** Speckled terrazzo. A warm cream terrazzo floor flecked with red, teal and charcoal chips, laid in big squares with thin brass dividing strips. */
  dinerterrazzo(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xd8c8b0);
    for (let y = T; y < B; y += 2) for (let x = from(L, 2, (y % 4) ? 1 : 0); x < R; x += 2) { const h = hash(x, y); if (h % 4 === 0) dot(x, y, [0xe03a4a, 0x4ab8a8, 0x3a3a40, 0xf0e4c8][h % 4]); }
    for (let y = from(T, 30); y < B; y += 30) fill(L, y, R - L, 1, 0xc8a050, 0.6); for (let x = from(L, 30); x < R; x += 30) fill(x, T, 1, B - T, 0xc8a050, 0.6);
  },
  /** Courthouse marble. Big cream marble slabs with faint grey veining, laid in a grid of thin inlaid brass lines with a small brass diamond at each crossing, polished to a sheen. */
  courtfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xc8bca8);
    for (let y = from(T, 24), r = 0; y < B; y += 24, r++) for (let x = from(L, 24); x < R; x += 24) {
      const h = hash(x, r + 2); let vx = x + (h % 18);
      for (let vy = y + 2; vy < y + 22; vy++) { vx += ((hash(vx, vy) % 3) - 1); if (vx > x && vx < x + 23) dot(vx, vy, 0xaaa090, 0.6); }
      if (h % 3 === 0) fill(x + 4, y + 5, 7, 1, 0xe0d8c8, 0.3);
      fill(x, y, 24, 1, 0xc8a050, 0.6); fill(x, y, 1, 24, 0xc8a050, 0.6); dot(x, y, 0xe8c878); dot(x - 1, y, 0xc8a050); dot(x + 1, y, 0xc8a050);
    }
  },
  /** Burgundy carpet. A deep wine-red institutional carpet with a thin gold border grid and a small gold fleur motif in each panel, a muted pile texture. */
  courtcarpet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x5a2028);
    for (let y = T; y < B; y += 2) for (let x = from(L, 2, (y % 4) ? 1 : 0); x < R; x += 2) { if (hash(x, y) % 5 === 0) dot(x, y, 0x4a181e, 0.6); }
    for (let y = from(T, 28); y < B; y += 28) for (let x = from(L, 28); x < R; x += 28) {
      fill(x, y, 28, 1, 0xc8a050, 0.5); fill(x, y, 1, 28, 0xc8a050, 0.5);
      const cx = x + 14, cy = y + 14; for (const [dx, dy] of [[0, -2], [0, 2], [-1, 0], [1, 0], [0, 0]]) dot(cx + dx, cy + dy, 0xc8a050, 0.6);
    }
  },
  /** Paint-splattered boards. Pale worn wood floorboards scattered all over with bright dabs and drips of dried paint in every colour, as in a working studio. */
  artfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xc8b48a);
    for (let y = from(T, 7); y < B; y += 7) { fill(L, y, R - L, 1, 0x9a8460); fill(L, y + 1, R - L, 1, 0xe0cc9e, 0.4); }
    const cols = [0xe8404a, 0x4a8ad8, 0xf2c040, 0x4ac86a, 0xff5ab0, 0xf0ece0, 0xe8822a, 0x8a5ad8];
    for (let k = 0; k < (R - L) * (B - T) / 90; k++) { const sx = L + hash(k, 3) % (R - L), sy = T + hash(k, 7) % (B - T), c = cols[hash(k, 2) % cols.length]; dot(sx, sy, c, 0.8); if (hash(k, 5) % 3 === 0) { dot(sx + 1, sy, c, 0.6); dot(sx, sy + 1, c, 0.5); } if (hash(k, 9) % 5 === 0) fill(sx, sy, 1, 2 + hash(k, 1) % 3, c, 0.4); }
  },
  /** Studio rug. A worn warm-red kilim rug with a cream diamond-and-zigzag border and little geometric motifs, the pile faded and soft. */
  artrug(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x7a3a3a);
    for (let y = T; y < B; y += 2) for (let x = from(L, 2, (y % 4) ? 1 : 0); x < R; x += 2) { if (hash(x, y) % 6 === 0) dot(x, y, 0x6a3030, 0.6); }
    for (let y = from(T, 24); y < B; y += 24) for (let x = from(L, 24); x < R; x += 24) {
      for (const [dx, dy] of [[12, 6], [12, 18], [6, 12], [18, 12]]) dot(x + dx, y + dy, 0xd8c0a0, 0.8);
      fill(x + 10, y + 10, 5, 5, 0x3a5a6a, 0.5); dot(x + 12, y + 12, 0xd8c0a0);
    }
    for (let x = L; x < R; x++) { if (x % 2 === 0) { dot(x, T + 2, 0xd8c0a0, 0.5); dot(x, B - 3, 0xd8c0a0, 0.5); } }
  },
  /** Rippled blue tile. Dark teal floor tiles washed with wavering caustic light — pale rippling lines that drift slowly, as if lit from a pool overhead. */
  aquariumfloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1e5266);
    for (let y = from(T, 16), r = 0; y < B; y += 16, r++) for (let x = from(L, 16); x < R; x += 16) { fill(x, y, 16, 1, 0x164050, 0.6); fill(x, y, 1, 16, 0x164050, 0.6); dot(x + 1, y + 1, 0x2e6a80, 0.5); }
    // drifting caustic ripples (coarse, to stay cheap)
    for (let y = from(T, 2); y < B; y += 2) for (let x = from(L, 3); x < R; x += 3) {
      const v = Math.sin((x * 0.12) + (y * 0.08) + now * 0.6) + Math.sin((x * 0.07) - (y * 0.11) + now * 0.4);
      if (v > 1.5) dot(x, y, 0x6ac0d8, 0.28); else if (v > 1.1) dot(x, y, 0x3a9ab0, 0.16);
    }
  },
  /** Glass tunnel floor. A walkway of thick glass panels with brass edge-strips, deep blue water below showing a pale sandy bottom and a fish-shadow or two gliding past. */
  aquariumglass(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x12436a);
    for (let x = L; x < R; x++) { const d = (x % 40); if (d < 20) dot(x, T + 6 + (x % 5), 0xcfa868, 0.3); } // hint of sandy bottom
    for (let y = from(T, 20); y < B; y += 20) fill(L, y, R - L, 1, 0xc8a050, 0.5); for (let x = from(L, 34); x < R; x += 34) fill(x, T, 1, B - T, 0xc8a050, 0.5); // brass strips
    // reflections and a gliding fish shadow
    for (let x = from(L, 3); x < R; x += 3) dot(x, T + 2 + Math.round(Math.sin(x * 0.2 + now) * 1), 0x3a9ab0, 0.2);
    const fx = L + ((now * 18) % (R - L + 40)) - 20, fy = T + (B - T) * 0.6; fill(Math.round(fx), Math.round(fy), 5, 2, 0x0a2a44, 0.4); fill(Math.round(fx) + 5, Math.round(fy), 2, 1, 0x0a2a44, 0.4);
  },
  /** The ring's raked sawdust: warm tan scattered with paler flecks and a few trodden-dark patches. */
  circussawdust(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xc89a5a);
    for (let y = T; y < B; y++) for (let x = from(L, 1, 0); x < R; x++) {
      const h = hash(x, y);
      if (h % 5 === 0) dot(x, y, 0xd8ae72, 0.7); else if (h % 9 === 0) dot(x, y, 0xa87a42, 0.6);
    }
  },
  /** The performers' entrance carpet: deep red with a soft pile fleck and a gold runner edge at the front. */
  circusredcarpet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x8a2420);
    for (let y = T; y < B; y++) for (let x = from(L, 1, 0); x < R; x++) { if (hash(x, y) % 6 === 0) dot(x, y, 0x9a2e28, 0.6); }
    fill(L, B - 2, R - L, 1, 0xd8a84a, 0.8); fill(L, T, R - L, 1, 0x6a1a18, 0.5);
  },
  /** The grand hall's marble: pale cream slabs with soft grey veining and a cool polished sheen. */
  museummarble(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xd8d2c4);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 26, (r % 2) * 13); x < R; x += 26) {
      const h = hash(x, r + 11), tone = [0xdcd6c8, 0xe4ded0, 0xd2ccbe][h % 3];
      fill(x + 1, y + 1, 24, 14, tone); fill(x + 1, y + 1, 24, 1, 0xefeadf); fill(x + 1, y + 15, 24, 1, 0xc2bba8);
      if (h % 3 === 0) { let vx = x + 4 + h % 14, vy = y + 2; for (let k = 0; k < 11; k++) { dot(vx, vy, 0xbdb6a2, 0.6); vx += (h >> k) % 2; vy += 1; if (vy > y + 14) break; } }
    }
  },
  /** The reading-room floor: warm herringbone parquet in alternating oak tones. */
  museumparquet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x9a6a3e);
    for (let y = T; y < B; y += 4) for (let x = from(L, 8, (((y / 4) | 0) % 2) * 4); x < R; x += 8) {
      const h = hash(x, y), tone = [0xa6703e, 0x946238, 0xb07c48][h % 3];
      fill(x, y, 7, 3, tone); fill(x, y, 7, 1, shaded(tone, 1.1)); fill(x + 6, y, 1, 3, shaded(tone, 0.85));
    }
  },
  /** Packed snow: cool blue-white with a sparkle here and there and a few trodden dimples. */
  northpolesnow(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xe6eef5);
    for (let y = T; y < B; y++) for (let x = from(L, 1, 0); x < R; x++) { const h = hash(x, y); if (h % 11 === 0) dot(x, y, 0xf6fbff, 0.8); else if (h % 17 === 0) dot(x, y, 0xcdd8e4, 0.6); }
    for (let y = T; y < B; y += 16) for (let x = from(L, 22, 0); x < R; x += 22) if (hash(x, y) % 3 === 0) dot(x + 4, y + 8, 0xd4dde8, 0.5);
  },
  /** Workshop planks: warm sawn wood with a candy-cane runner striped along the front. */
  northpoleplank(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x9a6a3e);
    for (let y = T; y < B; y += 5) { fill(L, y, R - L, 1, 0x7a5230); for (let x = from(L, 1, 0); x < R; x++) { if (hash(x, y) % 7 === 0) dot(x, y + 2, 0xa8784a, 0.5); } }
    for (let x = from(L, 4, 0); x < R; x += 4) fill(x, B - 2, 2, 1, (((x / 4) | 0) % 2) ? 0xd83030 : 0xf2f6fb, 0.8);
  },
  /** The studio floor: polished near-black with faint reflections and coloured gaffer-tape marks where actors hit their spots. */
  soundstagefloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1f2024);
    for (let y = T; y < B; y++) for (let x = from(L, 1, 0); x < R; x++) { if (hash(x, y) % 23 === 0) dot(x, y, 0x2e3036, 0.7); }
    for (let x = from(L, 40, 0); x < R; x += 40) fill(x, T, 2, B - T, 0x2a2c32, 0.3);
    for (let y = T; y < B; y += 18) for (let x = from(L, 30, 0); x < R; x += 30) { const h = hash(x, y); if (h % 2 === 0) { const c = [0xd83030, 0xe8c24a, 0x4c8bf5][h % 3], mx = x + 6, my = y + 9; fill(mx - 2, my, 5, 1, c, 0.8); fill(mx, my - 2, 1, 5, c, 0.8); } }
  },
  /** The green-room carpet: deep red with a soft fleck and faint gold stars woven in. */
  soundstagelounge(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x7a2230);
    for (let y = T; y < B; y++) for (let x = from(L, 1, 0); x < R; x++) { if (hash(x, y) % 7 === 0) dot(x, y, 0x8a2e3a, 0.5); }
    for (let y = T; y < B; y += 16) for (let x = from(L, 24, 0); x < R; x += 24) if (hash(x, y) % 3 === 0) { dot(x + 6, y + 7, 0xe8c24a, 0.5); dot(x + 5, y + 7, 0xe8c24a, 0.3); dot(x + 7, y + 7, 0xe8c24a, 0.3); dot(x + 6, y + 6, 0xe8c24a, 0.3); dot(x + 6, y + 8, 0xe8c24a, 0.3); }
  },
  /** Raked clay: warm earth combed into perfectly even horizontal furrows, with a soft grain. */
  cultclay(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xc8a884);
    for (let y = T; y < B; y++) { if ((y - T) % 3 === 0) fill(L, y, R - L, 1, 0xb89470, 0.35); for (let x = from(L, 1, 0); x < R; x++) { if (hash(x, y) % 11 === 0) dot(x, y, 0xd4b896, 0.5); } }
  },
  /** Woven reed matting in a close, even basket-weave of two straw tones. */
  cultmat(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xc8b488);
    for (let y = T; y < B; y++) for (let x = from(L, 4, (y % 8 < 4) ? 0 : 2); x < R; x += 4) fill(x, y, 2, 1, (((x + y) / 4 | 0) % 2) ? 0xd4c498 : 0xb8a478);
  },
  /** A dark stage floor lit by a glowing cyan LED grid that pulses at the intersections. */
  esportsstagefloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x16161f);
    for (let y = from(T, 16, 0); y < B; y += 16) fill(L, y, R - L, 1, 0x1e3a4a, 0.6);
    for (let x = from(L, 16, 0); x < R; x += 16) fill(x, T, 1, B - T, 0x1e3a4a, 0.6);
    const pulse = 0.4 + (Math.sin(now * 2) + 1) / 2 * 0.4;
    for (let y = from(T, 16, 0); y < B; y += 16) for (let x = from(L, 16, 0); x < R; x += 16) dot(x, y, 0x2ad8e8, pulse);
  },
  /** A deep-blue arena carpet flecked with team-colour chevrons. */
  esportscarpet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x1e1e40);
    for (let y = T; y < B; y++) for (let x = from(L, 1, 0); x < R; x++) { if (hash(x, y) % 9 === 0) dot(x, y, 0x2a2a56, 0.5); }
    for (let y = from(T, 14, 0); y < B; y += 14) for (let x = from(L, 20, (((y / 14) | 0) % 2) * 10); x < R; x += 20) { const c = [0x2ad8e8, 0xe82ad8, 0x9ae82a][hash(x, y) % 3]; dot(x, y + 6, c, 0.4); dot(x + 1, y + 7, c, 0.3); dot(x - 1, y + 7, c, 0.3); }
  },
  /** Rough cave stone: uneven slabs in warm grey-brown with cracks and the odd pebble. */
  cavemanrock(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x5a4e44);
    for (let y = T, r = 0; y < B; y += 16, r++) for (let x = from(L, 22, (r % 2) * 11); x < R; x += 22) { const h = hash(x, r + 9), tone = [0x5e5248, 0x544840, 0x665a4e][h % 3]; fill(x + 1, y + 1, 20, 14, tone); fill(x + 1, y + 1, 20, 1, shaded(tone, 1.12)); fill(x + 1, y + 15, 20, 1, shaded(tone, 0.82)); if (h % 4 === 0) dot(x + 4 + h % 12, y + 6 + (h >> 3) % 6, 0x7a6a5a, 0.6); }
  },
  /** An animal-hide rug: tan with irregular dark patches and a shaggy fur fringe at the front. */
  cavemanhide(L, T, R, B) {
    fill(L, T, R - L, B - T, 0xb89a6a);
    for (let y = T; y < B; y++) for (let x = from(L, 1, 0); x < R; x++) { if (hash(x, y) % 8 === 0) dot(x, y, 0xc8aa7a, 0.5); }
    for (let y = from(T, 14, 0); y < B; y += 14) for (let x = from(L, 20, 0); x < R; x += 20) if (hash(x, y) % 2 === 0) fill(x, y + 4, 4 + hash(x, y) % 4, 3, 0x8a6a42, 0.6);
    for (let x = from(L, 2, 0); x < R; x += 2) fill(x, B - 2, 1, 2, (((x / 2) | 0) % 2) ? 0x9a7a4a : 0x7a5a3a);
  },
  /** A black-and-white checkerboard marble floor, each square softly bevelled. */
  lodgechecker(L, T, R, B) {
    const tile = 14;
    for (let y = T; y < B; y += tile) for (let x = from(L, tile, 0); x < R; x += tile) { const tx = Math.floor((x - L) / tile), ty = Math.floor((y - T) / tile), dk = (tx + ty) % 2, base = dk ? 0x2a2420 : 0xcfc6b0, lite = dk ? 0x3a342e : 0xe0d8c2, lo = dk ? 0x1a1612 : 0xb8b096; fill(x, y, tile, tile, base); fill(x, y, tile, 1, lite); fill(x, y + tile - 1, tile, 1, lo); }
  },
  /** A deep-burgundy carpet with a gold border line and small fleur motifs along the front. */
  lodgecarpet(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x5a1e24);
    for (let y = T; y < B; y++) for (let x = from(L, 1, 0); x < R; x++) { if (hash(x, y) % 9 === 0) dot(x, y, 0x6a242a, 0.5); }
    fill(L, T + 2, R - L, 1, 0xc8a44e, 0.5); fill(L, B - 3, R - L, 1, 0xc8a44e, 0.5);
    for (let x = from(L, 24, 0); x < R; x += 24) { dot(x, B - 6, 0xc8a44e, 0.5); dot(x - 1, B - 7, 0xc8a44e, 0.3); dot(x + 1, B - 7, 0xc8a44e, 0.3); dot(x, B - 8, 0xc8a44e, 0.3); }
  },
  /** Bare concrete: grey with pits, hairline cracks and a painted caution walkway line along the front. */
  prepperconcrete(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x6a665c);
    for (let y = T; y < B; y++) for (let x = from(L, 1, 0); x < R; x++) { const h = hash(x, y); if (h % 13 === 0) dot(x, y, 0x5a564c, 0.6); else if (h % 23 === 0) dot(x, y, 0x7a766c, 0.5); }
    for (let x = from(L, 40, 0); x < R; x += 40) { let cx = x, cy = T + 4; for (let k = 0; k < 20 && cy < B; k++) { dot(cx, cy, 0x4a463e, 0.6); cx += (hash(cx, cy) % 3) - 1; cy += 1; } }
    fill(L, B - 10, R - L, 1, 0xc8a82a, 0.5); for (let x = from(L, 6, 0); x < R; x += 6) fill(x, B - 10, 3, 1, 0xc8a82a, 0.3);
  },
  /** Steel checker-plate flooring with a diamond tread and panel seams. */
  prepperrubber(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x5a5e62);
    for (let y = T; y < B; y += 4) for (let x = from(L, 4, (((y / 4) | 0) % 2) * 2); x < R; x += 4) { dot(x, y, 0x72767c); dot(x + 1, y + 1, 0x42464c); }
    for (let y = from(T, 16, 0); y < B; y += 16) fill(L, y, R - L, 1, 0x42464c, 0.4);
  },
  /** Oval seal rug. Warm parquet with a big oval presidential rug over it: a navy field inside a gold border, a gold ring and a generic star-ringed eagle medallion in the middle. */
  paradiseovalrug(L, T, R, B) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, seam = 0x3a2616;
    const navy = 0x1f3a66, gold = 0xd6b25e, goldLight = 0xeccf84, cream = 0xe6dabd;
    fill(L, T, R - L, B - T, wood);
    for (let y = T, r = 0; y < B; y += 6, r++) {
      fill(L, y, R - L, 1, seam, 0.6);
      for (let x = from(L, 44, (r % 2) * 22); x < R; x += 44) { fill(x, y, 1, 6, seam, 0.5); dot(x + 6, y + 2, woodLight, 0.4); dot(x + 24, y + 3, woodDark, 0.5); }
    }
    // The oval rug, centred in the room: a navy field inside a gold border
    const cx = Math.round((L + R) / 2), cy = Math.round((T + B) / 2) + 2;
    const rx = Math.min(72, Math.round((R - L) * 0.3)), ry = Math.min(42, Math.round((B - T) * 0.4));
    if (rx < 16 || ry < 11) return;
    for (let dy = -ry; dy <= ry; dy++) {
      const hw = Math.round(rx * Math.sqrt(Math.max(0, 1 - (dy / ry) ** 2)));
      if (hw <= 0) continue;
      fill(cx - hw, cy + dy, hw * 2, 1, navy);
      const band = Math.min(hw * 2, Math.abs(dy) > ry - 3 ? hw * 2 : 3);
      fill(cx - hw, cy + dy, band, 1, gold); fill(cx + hw - band, cy + dy, band, 1, gold);
    }
    ellipseRing(cx, cy, rx - 4, ry - 3, goldLight, 0.8);
    ellipseRing(cx, cy, Math.round(rx * 0.42), Math.round(ry * 0.42), gold, 0.9);
    // The generic seal: a pale medallion, a ring of stars, a spread-winged eagle
    disc(cx, cy, Math.min(ry - 6, 9), cream, 0.22);
    for (let k = 0; k < 13; k++) { const a = (k / 13) * Math.PI * 2; dot(cx + Math.round(Math.cos(a) * rx * 0.3), cy + Math.round(Math.sin(a) * ry * 0.3), goldLight); }
    fill(cx - 7, cy - 1, 15, 2, gold); dot(cx - 8, cy, gold); dot(cx + 8, cy, gold);
    fill(cx - 2, cy - 4, 5, 7, goldLight); dot(cx, cy - 5, gold); dot(cx - 1, cy + 3, gold); dot(cx + 1, cy + 3, gold);
  },
  /** Bunker concrete. Sealed grey concrete in big slabs with expansion joints, mottled stains, and a painted yellow hazard line near the back. */
  paradisebunkerfloor(L, T, R, B) {
    const base = 0x6c7076, light = 0x7e828a, dark = 0x565a60, joint = 0x4a4e54, hazard = 0xd4a82a;
    fill(L, T, R - L, B - T, base);
    for (let y = T, r = 0; y < B; y += 14, r++) {
      fill(L, y, R - L, 1, joint);
      for (let x = from(L, 28, (r % 2) * 14); x < R; x += 28) {
        fill(x, y, 1, 14, joint);
        const h = hash(x, r);
        for (let k = 0; k < 4; k++) dot(x + 3 + ((h >> k) % 22), y + 2 + ((h >> (k + 2)) % 11), (h >> k) % 2 ? light : dark, 0.6);
      }
    }
    fill(L, T + 3, R - L, 2, dark);
    for (let x = from(L, 6); x < R; x += 6) fill(x, T + 3, 3, 2, hazard, 0.85);
  },
  /** Starship deck. Grey deck panels in a grid with darker seams and a thin blue accent trim running along every few rows. */
  startrekdeck(L, T, R, B) {
    const base = 0x3a4048, light = 0x4a515c, dark = 0x2a2f36, accent = 0x4a6ea8;
    fill(L, T, R - L, B - T, base);
    for (let y = T, r = 0; y < B; y += 12, r++) for (let x = from(L, 20, (r % 2) * 10); x < R; x += 20) {
      fill(x, y, 19, 11, ((Math.floor(x / 20) + r) % 2) ? light : base);
      fill(x, y, 19, 1, dark); fill(x, y, 1, 11, dark);
      if ((Math.floor(x / 20) + r) % 5 === 0) fill(x + 2, y + 5, 15, 1, accent, 0.5);
    }
  },
  /** Deck carpet. A darker blue-grey deck with seam lines and small teal guidance lights set into it here and there (lounge). */
  startrekpad(L, T, R, B) {
    const base = 0x243442, dark = 0x1a2630, led = 0x5ad0e0;
    fill(L, T, R - L, B - T, base);
    for (let y = T, r = 0; y < B; y += 10, r++) { fill(L, y, R - L, 1, dark); for (let x = from(L, 16, (r % 2) * 8); x < R; x += 16) { fill(x, y, 1, 10, dark, 0.6); if (hash(x, r) % 9 === 0) dot(x + 8, y + 5, led, 0.6); } }
  },
  /** Herringbone parquet. Warm wood in a two-tone herringbone, each short plank with a diagonal grain line and a dark seam. */
  gentlemenherringbone(L, T, R, B) {
    const tones = [0x6a4a2e, 0x7a5636], seam = 0x2e2016, light = 0x9a7048;
    fill(L, T, R - L, B - T, tones[0]);
    const pw = 7, ph = 4;
    for (let y = T, r = 0; y < B; y += ph, r++) for (let x = from(L, pw, (r % 2) * Math.floor(pw / 2)); x < R; x += pw) {
      const c = tones[(Math.floor(x / pw) + r) % 2];
      fill(x, y, pw - 1, ph - 1, c); fill(x, y, pw - 1, 1, light, 0.25);
      for (let k = 0; k < ph; k++) dot(x + k, y + k, seam, 0.3);
      fill(x, y, 1, ph, seam, 0.4);
    }
  },
  /** Tartan rug. A deep-red tartan with crossing dark and green bands and a thin gold overcheck (lounge). */
  gentlementartan(L, T, R, B) {
    const base = 0x5a2a2a, dark = 0x3a1a1a, green = 0x2a4a2a, gold = 0xc8a050;
    fill(L, T, R - L, B - T, base);
    for (let x = from(L, 14); x < R; x += 14) { fill(x, T, 4, B - T, dark, 0.5); fill(x + 6, T, 1, B - T, green, 0.5); fill(x + 2, T, 1, B - T, gold, 0.22); }
    for (let y = from(T, 14); y < B; y += 14) { fill(L, y, R - L, 4, dark, 0.5); fill(L, y + 6, R - L, 1, green, 0.5); fill(L, y + 2, R - L, 1, gold, 0.22); }
  },
  /** Black stone. Big honed graphite slabs with hairline seams, a few pale flecks, and a soft sheen at each slab's top left. */
  grokfloor(L, T, R, B) {
    const stone = 0x34363e, seam = 0x26282e, sheen = 0x3e414a, fleck = 0x5a5e68;
    fill(L, T, R - L, B - T, stone);
    for (let y = from(T, 24), r = 0; y < B; y += 24, r++) for (let x = from(L, 24, r % 2 ? 12 : 0); x < R; x += 24) {
      fill(x, y, 24, 1, seam); fill(x, y, 1, 24, seam);
      fill(x + 1, y + 1, 9, 1, sheen); fill(x + 1, y + 2, 1, 5, sheen);
      for (let k = 0; k < 3; k++) { const h = hash(x + k, y); dot(x + 3 + (h % 19), y + 3 + ((h >> 5) % 19), fleck, 0.6); }
    }
  },
  /** Orbit rug. A charcoal rug traced with thin orbit rings round a pale dot, each ring with its own little planet. */
  grokrug(L, T, R, B) {
    const rug = 0x26282e, ring = 0x3a3d47, planet = 0xc8ccd4;
    fill(L, T, R - L, B - T, rug);
    for (let y = from(T, 40); y < B; y += 40) for (let x = from(L, 48); x < R; x += 48) {
      const cx = x + 24, cy = y + 20;
      for (const [rx, ry] of [[6, 3], [12, 6], [19, 10]]) ellipseRing(cx, cy, rx, ry, ring, 0.9);
      dot(cx, cy, planet); dot(cx - 1, cy, planet, 0.5); dot(cx + 1, cy, planet, 0.5);
      const h = hash(x, y), a = (h % 628) / 100;
      dot(cx + Math.round(Math.cos(a) * 12), cy + Math.round(Math.sin(a) * 6), 0x8ab4ff);
      dot(cx + Math.round(Math.cos(a + 2.4) * 19), cy + Math.round(Math.sin(a + 2.4) * 10), planet, 0.8);
    }
  },
  /** Raised black floor. Square graphite tiles on a raised floor, a pale edge on each, and here and there a vented tile with a faint ion-blue glow coming up. */
  grokbotfloor(L, T, R, B) {
    const tile = 0x26282e, edge = 0x30333a, seam = 0x1a1b20, vent = 0x15161a;
    fill(L, T, R - L, B - T, tile);
    for (let y = from(T, 12), row = Math.floor(y / 12); y < B; y += 12, row++) {
      fill(L, y, R - L, 1, seam);
      for (let x = from(L, 12), column = Math.floor(x / 12); x < R; x += 12, column++) {
        fill(x, y, 1, 12, seam); fill(x + 1, y + 1, 10, 1, edge);
        if (hash(column, row) % 7 !== 0) continue;
        for (let dy = 3; dy < 11; dy += 2) for (let dx = 3; dx < 11; dx += 2) dot(x + dx, y + dy, vent);
        fill(x + 3, y + 3, 7, 7, 0x8ab4ff, 0.06);
      }
    }
  },
  /**
   * Slate with amethyst flecks. Dark plum-grey slate in long uneven slabs, a few violet crystal flecks in it,
   * darker towards the walls, and along the back of the floor the brass mine-cart line to the vault.
   */
  geodefloor(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x28222e);
    for (let y = T, r = 0; y < B; y += 11, r++) {
      for (let x = -140 - (r % 3) * 9; x < R; ) {
        const h = hash(x, y), len = 18 + (h % 17), c = [0x2d2734, 0x29232f, 0x25202a][(h >>> 4) % 3];
        fill(x, y, len, 11, c); fill(x, y, len, 1, shaded(c, 1.14)); fill(x, y + 1, 1, 10, 0x1d1922);
        x += len;
      }
    }
    for (let y = T + 2; y < B; y += 3) for (let x = from(L, 4, (y * 3) % 4); x < R; x += 4) {
      const h = hash(x, y);
      if (h % 29 === 0) { dot(x, y, 0x5a3e88); if ((h >>> 5) % 3 === 0) dot(x + 1, y, 0x7a5ab0); }
    }
    // The cave's dark: against the wall, and towards the sides and the front
    fill(L, T, R - L, 9, BLACK, 0.18); fill(L, T + 9, R - L, 6, BLACK, 0.08);
    for (let k = 0; k < 4; k++) { fill(L, T, 6 + k * 6, B - T, BLACK, 0.06); fill(R - 6 - k * 6, T, 6 + k * 6, B - T, BLACK, 0.06); fill(L, B - 6 - k * 6, R - L, 6 + k * 6, BLACK, 0.05); }
    // Little crystal clusters and rubble at the floor's edges, out of everyone's way (they glow by night: see FLOOR_AIRS)
    for (const [fx, fy, k] of gdFloorClusters(L, T, R, B)) {
      for (const [dx, h, w, lean] of [[-4, 4, 2, -0.4], [-1, 7, 3, -0.1], [2, 5, 2, 0.35]]) gdPoint(fx + dx, fy, h, w, GD_AMETHYST[(k + dx + 4) % 3], -1, lean);
      fill(fx - 5, fy + 1, 10, 1, SHADOW); dot(fx - 7, fy, 0x4a4050); dot(fx + 5, fy + 1, 0x4a4050); dot(fx + 7, fy - 1, 0x3a3444);
    }
    gdRails(L, R, 42, GD_LINE);
  },
  /**
   * Agate-slice paving. The cave's polished floor: big slices of agate laid as flagstones in dark grout, each cut through
   * its bands, plum and grey close in tone with one pale line, round an off-centre heart, one slice in four with a little
   * heart of violet crystal; darker towards the walls.
   */
  geodeagate(L, T, R, B) {
    fill(L, T, R - L, B - T, 0x161018);
    const oval = (cx, cy, rx, ry, c) => {
      for (let r = -Math.floor(ry); r <= Math.floor(ry); r++) { const h = Math.round(rx * Math.sqrt(Math.max(0, 1 - (r / (ry + 0.5)) ** 2))); fill(cx - h, cy + r, h * 2 + 1, 1, c); }
    };
    const BANDS = [[0x2a2232, 0x2f2638, 0x2b2334, 0x332a3e, 0x2e2538], [0x29242e, 0x2e2934, 0x2a2530, 0x322c3a, 0x2c2732], [0x2c2230, 0x322636, 0x2e2332, 0x372a3c, 0x302535]];
    for (let y = T, r = 0; y < B; y += 23, r++) {
      for (let x = from(L, 35, (r % 2) * 17); x < R; x += 35) {
        const h = hash(x, y), pal = BANDS[(h >>> 3) % BANDS.length], cx = x + 9 + (h % 17), cy = y + 6 + ((h >>> 5) % 10);
        within([[x, y, 34, 22]], () => {
          fill(x, y, 34, 22, pal[0]);
          for (let k = 1; k < pal.length; k++) { const rx = 30 - k * 5.5; oval(cx, cy, rx, rx * 0.6, pal[k]); }
          // A pale line through the bands, and in one slice in four a little heart of violet crystal
          const pale = 30 - (1 + ((h >>> 9) % 3)) * 5.5 + 2.7;
          for (let a = 0; a < Math.PI * 2; a += 0.07) dot(cx + Math.round(Math.cos(a) * pale), cy + Math.round(Math.sin(a) * pale * 0.6), 0x5a4c6a, 0.4);
          if ((h >>> 13) % 4 === 0) { oval(cx, cy, 2, 1, 0x3a2a52); dot(cx - 1, cy, 0x7a52b8); dot(cx + 1, cy, 0x9a78d8); dot(cx, cy - 1, 0x6a44a8); }
          fill(x, y, 34, 1, WHITE, 0.04); fill(x, y + 21, 34, 1, BLACK, 0.12);
        });
      }
    }
    // The cave's dark against the walls
    fill(L, T, R - L, 8, BLACK, 0.18); for (let k = 0; k < 3; k++) fill(R - 6 - k * 6, T, 6 + k * 6, B - T, BLACK, 0.06);
  },
};
