/**
 * The walls' finishes besides paint, each drawn from `L` to `R` as the
 * back wall is, from its top down to the baseboard: its top edge, a
 * rail at 30 and the baseboard at 38, so what hangs on it sits as it
 * does on paint.
 */
const WALL_FINISHES = {
  /**
   * Citrine crystal wall. Smoky brown rock flecked with gold under a rind banded in honey and cream, pockets in it lined
   * with golden points, and clusters of them along under the ledge; a point glints now and then, and by night the
   * pockets glow warm and the points' tips shine.
   */
  geodecitrine(L, R) {
    const w = R - L, rock = 0x3a2a1e, rockLight = 0x4a3826, rockDark = 0x261a12;
    fill(L, 0, w, 40, rock);
    for (let x = from(L, 7, 2); x < R; x += 7) { const h = hash(x, 13); fill(x, 9 + (h % 17), 3 + (h % 4), 1, (h >>> 3) % 2 ? rockLight : rockDark); }
    for (let x = from(L, 3); x < R; x += 3) { const h = hash(x, 15); if (h % 5 === 0) dot(x, 8 + ((h >>> 4) % 20), 0x6a4a28, 0.6); }
    // The rind along the top, banded honey and cream
    for (let x = from(L, 4); x < R; x += 4) {
      const d = Math.round(Math.sin(x * 0.05) * 1.4 + Math.sin(x * 0.17) * 0.6);
      fill(x, 0, 4, 3 + d, 0x24180e); fill(x, 3 + d, 4, 1, 0x8a6a3a); fill(x, 4 + d, 4, 1, 0xe8d8b0); fill(x, 5 + d, 4, 1, 0xa87a30);
    }
    // Pockets in the rock, a rind round each, lined with golden points growing up and hanging down
    const tips = [], pockets = [];
    for (let x = from(L, 33, 9); x < R; x += 33) {
      const h = hash(x, 23), px = x + (h % 11), py = 15 + ((h >>> 5) % 7);
      // The hollow, oval, its rind round it, lit along its top
      for (let r = -4; r <= 4; r++) { const h = Math.round(7 * Math.sqrt(Math.max(0, 1 - (r / 4.6) ** 2))); fill(px - h, py + r, h * 2 + 1, 1, r < -2 ? 0x6a4a28 : 0x4a3420); }
      for (let r = -3; r <= 3; r++) { const h = Math.round(5.6 * Math.sqrt(Math.max(0, 1 - (r / 3.6) ** 2))); fill(px - h, py + r, h * 2 + 1, 1, 0x1e140a); }
      for (let dx = -4; dx <= 4; dx++) dot(px + dx, py - 4 + (Math.abs(dx) > 2 ? 1 : 0), 0xc8a050, 0.55);
      for (const [dx, hh, lean, i] of [[-4, 3, -0.35, 0], [-2, 5, -0.15, 1], [0, 6, 0, 2], [2, 5, 0.15, 0], [4, 3, 0.35, 1]]) {
        gdPoint(px + dx, py + 3, hh, 2, GD_CITRINE[i], -1, lean); tips.push([px + dx + Math.round((hh - 1) * lean) + 1, py + 4 - hh]);
      }
      for (const [dx, hh, i] of [[-3, 2, 2], [1, 3, 0], [3, 2, 1]]) gdPoint(px + dx, py - 3, hh, 1, GD_CITRINE[i], 1);
      pockets.push([px, py]);
    }
    // Gold flecks in the rock
    for (let x = from(L, 4, 1); x < R; x += 4) { const h = hash(x, 27); if (h % 7 === 0) dot(x, 8 + ((h >>> 4) % 20), 0xc89a3a, 0.7); }
    // The ledge at the rail, and clusters of golden points along under it
    fill(L, 29, w, 2, rockLight); fill(L, 29, w, 1, 0x6a4a28); fill(L, 31, w, 7, rockDark);
    for (let x = from(L, 13, 4); x < R; x += 13) {
      const h = hash(x, 31), big = h % 3 === 0;
      for (const [dx, hh, lean] of [[-3, 2 + (h % 2), -0.3], [-1, big ? 6 : 4, -0.05], [1, big ? 4 : 3, 0.25]]) gdPoint(x + dx, 37, hh, 2, GD_CITRINE[(h >>> 4) % 3], -1, lean);
      if (big) tips.push([x, 32]);
    }
    fill(L, 38, w, 2, 0x1a120c);
    gdTwinkle(tips, 1.7, 19);
    if (isNight) glowLate(() => {
      for (const [px, py] of pockets) glowAt(px, py + 1, 9, 7, 0xf0b040, 0.14);
      for (let x = from(L, 26, 7); x < R; x += 26) glowAt(x, 34, 10, 3, 0xf0b040, 0.08);
      for (const [tx, ty] of tips) dot(tx, ty, 0xfff0b0, 0.85);
    });
  },
  /**
   * Amethyst geode wall. The inside of a split geode: its banded rind along the top, violet crystals hanging
   * from it in clusters like chandeliers and a crystal bed growing up under the ledge, dark plum rock between with a druzy
   * sparkle; a point glints now and then, and by night the crystals' tips glow.
   */
  geodewall(L, R) {
    const w = R - L, rock = 0x2a1a3a, rockLight = 0x36244c, rockDark = 0x1a1026;
    const wave = (x) => Math.round(Math.sin(x * 0.045) * 1.5 + Math.sin(x * 0.13) * 0.7);
    const near = (x, step, off) => { const c = from(x, step, off) + step / 2; return 1 - Math.min(1, Math.abs(x - c) / (step / 2)); };
    fill(L, 0, w, 40, rock);
    for (let x = from(L, 9, 3); x < R; x += 9) { const h = hash(x, 11); fill(x, 12 + (h % 11), 4 + (h % 5), 2, (h >>> 3) % 2 ? rockLight : rockDark); }
    for (let x = from(L, 3); x < R; x += 3) { const h = hash(x, 5); if (h % 4 === 0) dot(x, 10 + ((h >>> 4) % 18), 0x6a4c98, 0.55); }
    // The rind, banded as a cut geode's edge is
    for (let x = from(L, 4), e; x < R; x = e) {
      const d = wave(x); e = x + 4; while (e < R && wave(e) === d) e += 4;
      fill(x, 0, e - x, 3 + d, 0x342824); fill(x, 3 + d, e - x, 1, 0x5e5666); fill(x, 4 + d, e - x, 1, 0xa8a0bc); fill(x, 5 + d, e - x, 1, 0x6e5e90);
    }
    // Crystals hanging from it, longest in the middle of each cluster
    const tips = [], bright = [];
    for (let x = from(L, 4, 1); x < R; x += 4) {
      const h = hash(x, 3), n = near(x, 29, 11), len = 3 + (h % 3) + Math.round(n * n * 10), d = wave(x);
      if (n < 0.3 && (x >> 2) % 2) continue;
      gdPoint(x + ((h >>> 9) % 2), 6 + d, len, n > 0.45 ? 3 : 2, GD_AMETHYST[(h >>> 7) % 3], 1);
      if (n > 0.5) { tips.push([x + 1, 6 + d + len - 3]); bright.push([x + 1 + ((h >>> 9) % 2), 6 + d + len - 1]); }
    }
    // The ledge at the rail, and the crystal bed under it, its clusters between the hanging ones
    fill(L, 29, w, 2, rockLight); fill(L, 29, w, 1, 0x4e3a70); fill(L, 31, w, 7, rockDark);
    for (let x = from(L, 4, 3); x < R; x += 4) {
      const h = hash(x, 7), n = near(x, 29, 25), len = 2 + (h % 3) + Math.round(n * n * 5);
      if (n < 0.3 && (x >> 2) % 2) continue;
      gdPoint(x, 37, len, n > 0.5 ? 3 : 2, GD_AMETHYST[(h >>> 6) % 3], -1);
      if (n > 0.6) { tips.push([x + 1, 38 - len + 1]); bright.push([x + 1, 37 - len + 1]); }
    }
    fill(L, 38, w, 2, 0x140a1e);
    gdTwinkle(tips, 1.4, 11); gdTwinkle(tips, 2.3, 23);
    if (isNight) {
      // The bed's glow and its bright tips in the wall itself, so the lift, the door and the counter stand in front of them
      for (let x = from(L, 29, 11) + 28; x < R + 8; x += 29) { gdGlow(x, 33, 9, 5, 0xa070f0, 0.42); gdGlow(x, 32, 4, 3, 0xc8a8ff, 0.3); }
      for (const [bx, by] of bright) if (by >= 26) { fill(bx - 1, by, 3, 1, 0xc8a8ff, 0.7); fill(bx, by - 1, 1, 3, 0xc8a8ff, 0.7); dot(bx, by, 0xf4ecff); }
      // The hanging points' glows, over the night's dark, but never on the lift or the door, whose tops reach up among them
      const liftL = ELEVATOR.x - 1, liftR = ELEVATOR.x + ELEVATOR.w + 1, doorL = DOOR_FRAME.left + shiftsOf().door, doorR = DOOR_FRAME.right + shiftsOf().door + 1;
      const clear = [[L, 0, R - L, 12], [L, 12, liftL - L, 14], [liftR, 12, doorL - liftR, 14], [doorR, 12, R - doorR, 14]].filter(([, , w]) => w > 0);
      glowLate(() => within(clear, () => {
        for (let x = from(L, 29, 11) + 14; x < R; x += 29) gdGlow(x, 12, 9, 7, 0xa070f0, 0.24);
        for (const [bx, by] of bright) if (by < 26) { fill(bx - 1, by, 3, 1, 0xc8a8ff, 0.45); dot(bx, by, 0xe0ccff, 0.85); }
      }));
    }
  },
  /** Cloud-room panels. Dark steel-blue panels with hairline seams, each with a little status light, over a near-black wainscot and an ion-blue rail. */
  grokbotwall(L, R) {
    const w = R - L, wall = 0x22252c, seam = 0x1a1c22, sheen = 0x2a2d35;
    fill(L, 0, w, 40, wall); fill(L, 0, w, 2, 0x16171b);
    for (let x = from(L, 24); x < R; x += 24) {
      fill(x, 2, 1, 28, seam); fill(x + 1, 2, 1, 28, sheen); fill(L, 16, w, 1, seam, 0.6);
      dot(x + 20, 5, hash(x, 5) % 3 ? 0x8ab4ff : 0xf4f1ea, 0.5);
    }
    fill(L, 30, w, 8, 0x1a1c22); fill(L, 30, w, 1, 0x8ab4ff, 0.75);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Graphite and white. Graphite panels with hairline seams under a white light line along the ceiling, a near-black wainscot and a white rail. */
  grokwall(L, R) {
    const w = R - L, wall = 0x24262c, seam = 0x1c1d22, sheen = 0x2c2f36, dark = 0x16171b;
    fill(L, 0, w, 40, wall); fill(L, 0, w, 2, dark); fill(L, 2, w, 1, 0xf4f1ea, 0.55); fill(L, 3, w, 1, 0xf4f1ea, 0.12);
    for (let x = from(L, 32); x < R; x += 32) { fill(x, 4, 1, 26, seam); fill(x + 1, 4, 1, 26, sheen); }
    fill(L, 30, w, 8, dark); fill(L, 30, w, 1, 0xd8dce4); fill(L, 31, w, 1, 0x3a3d47);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Dark acoustic slats. Upright felt-dark slats with shadow gaps, over a near-black wainscot and a thin ion-blue rail. */
  grokpanel(L, R) {
    const w = R - L, slat = 0x2e3038, slatLight = 0x363943, gap = 0x1a1b20;
    fill(L, 0, w, 40, slat); fill(L, 0, w, 2, 0x1e1f24);
    for (let x = from(L, 4); x < R; x += 4) { fill(x, 2, 1, 28, gap); fill(x + 1, 2, 1, 28, slatLight, 0.6); }
    fill(L, 30, w, 8, 0x1a1b20); fill(L, 30, w, 1, 0x8ab4ff, 0.8);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Panelled study. A hunting-green wall over a tall wood dado of raised panels, divided by a thin brass picture rail. */
  gentlemenpanel(L, R) {
    const w = R - L, green = 0x1e3328, greenDark = 0x152418, greenLight = 0x2a4636, wood = 0x4a3524, woodLight = 0x6a4a2e, woodDark = 0x2e2016, brass = 0xc8a050;
    fill(L, 0, w, 40, green); fill(L, 0, w, 2, greenDark); fill(L, 3, w, 1, greenLight, 0.3);
    fill(L, 24, w, 14, wood); fill(L, 24, w, 1, brass, 0.6); fill(L, 25, w, 1, woodLight);
    for (let x = from(L, 20); x < R; x += 20) { fill(x + 2, 27, 15, 8, woodDark, 0.35); fill(x + 2, 27, 15, 1, woodLight, 0.5); fill(x + 2, 34, 15, 1, woodDark); fill(x, 24, 1, 14, woodDark); }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Bridge bulkheads. Dark panelled bulkheads with a row of rounded back-lit LCARS blocks in command colours and an amber rail band; the panels glow softly at night. */
  startrekbulkhead(L, R) {
    const w = R - L, base = 0x2a3142, dark = 0x1e2630, panel = 0x3a4458, cols = [0xd4a02a, 0x4a8ec8, 0xc0566a, 0x6abf8a];
    fill(L, 0, w, 40, base); fill(L, 0, w, 2, dark);
    for (let x = from(L, 22); x < R; x += 22) {
      const h = hash(x, 2), c = cols[h % cols.length];
      fill(x + 1, 6, 18, 5, c, 0.85); fill(x + 1, 6, 18, 1, shaded(c, 1.2)); fill(x + 1, 10, 18, 1, dark);
      fill(x + 1, 13, 10, 3, panel); fill(x + 12, 13, 7, 3, cols[(h >> 3) % cols.length], 0.7);
      dot(x + 15, 8, WHITE, 0.5);
    }
    fill(L, 30, w, 8, dark); fill(L, 29, w, 2, 0xd4a02a); fill(L, 27, w, 1, 0xd4a02a, 0.4);
    fill(L, 38, w, 2, PALETTE.baseboard);
    if (isNight) for (let x = from(L, 22); x < R; x += 22) glowAt(x + 10, 8, 10, 4, cols[hash(x, 2) % cols.length], 0.1);
  },
  /** Bunker concrete. Raw grey formwork concrete with tie-holes, a riveted blast-door seam down the middle, and a torn flap of the cream Oval wallpaper peeling off the top-left corner — the bunker showing through. */
  paradiseconcrete(L, R) {
    const w = R - L, grey = 0x7a7e84, greyLight = 0x8f9298, greyDark = 0x5e6268, steel = 0x6e727c, cream = 0xe6dabd, creamDark = 0xcbb892;
    fill(L, 0, w, 40, grey); fill(L, 0, w, 2, greyDark);
    // The formwork: faint board seams across, with tie-holes between them
    for (let y = 5, r = 0; y < 30; y += 8, r++) {
      fill(L, y, w, 1, greyDark, 0.5); fill(L, y + 1, w, 1, greyLight, 0.25);
      for (let x = from(L, 16, (r % 2) * 8); x < R; x += 16) { dot(x, y + 4, greyDark); dot(x + 1, y + 4, greyLight, 0.4); }
    }
    // The blast-door seam down the middle, a steel channel riveted both sides
    const mid = Math.round((L + R) / 2);
    fill(mid - 2, 0, 4, 30, greyDark); fill(mid - 2, 0, 1, 30, steel); fill(mid + 1, 0, 1, 30, 0x44484e);
    for (let k = 2; k < 30; k += 5) { dot(mid - 1, k, steel); dot(mid + 1, k, 0x44484e); }
    // A torn flap of the old Oval wallpaper, curling off the top-left corner
    fill(L + 2, 0, 16, 10, cream); fill(L + 2, 0, 16, 1, 0xf0e6cc); fill(L + 2, 10, 16, 1, creamDark);
    for (let k = 0; k < 10; k++) dot(L + 18 - (k % 3), k, creamDark, 0.9);
    dot(L + 6, 4, 0xd6b25e); dot(L + 12, 6, 0xd6b25e, 0.7);
    fill(L, 30, w, 8, greyDark); fill(L, 30, w, 1, steel); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Lantern green. A near-black green wall with green energy lines running up from the rail like light from a lantern, the rail itself lit green; brighter at night. */
  dcmixlanternwall(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x142a1a); fill(L, 0, w, 2, 0x0c1a10);
    for (let x = from(L, 14); x < R; x += 14) { const h = hash(x, 3), top = 6 + h % 12; fill(x + (h % 5), top, 1, 30 - top, DC_GREEN, isNight ? 0.22 : 0.12); dot(x + (h % 5), top, DC_GREEN_LIGHT, 0.5); }
    fill(L, 30, w, 8, 0x101c14); fill(L, 30, w, 1, DC_GREEN); fill(L, 27, w, 3, DC_GREEN, isNight ? 0.12 : 0.05);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Cave stone. A rough dark rock face in uneven blocks, damp streaks running down it, stalactite drips along the top, and a steel rail with a yellow safety line. */
  batcavestone(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x2e3036);
    for (let y = 2, r = 0; y < 30; y += 7, r++) for (let x = from(L, 18, (r % 2) * 9); x < R; x += 18) {
      const h = hash(x, y);
      fill(x + 1, y + 1, 16, 5, [0x32343a, 0x2a2c32, 0x36383e][h % 3]); fill(x + 1, y + 1, 16, 1, 0x3e4048, 0.7); fill(x, y + 6, 18, 1, 0x1e1f24);
      if (h % 6 === 0) fill(x + 3 + (h >>> 4) % 12, y, 1, 7 + (h >>> 8) % 6, 0x24262c, 0.7);
    }
    for (let x = from(L, 11); x < R; x += 11) { const h = hash(x, 0); fill(x + (h % 5), 0, 2, 2 + (h >>> 3) % 4, 0x1c1d22); dot(x + (h % 5), 2 + (h >>> 3) % 4, 0x1c1d22); }
    fill(L, 30, w, 8, 0x24262c); fill(L, 30, w, 1, BC_STONE_LIGHT); fill(L, 31, w, 1, BC_YELLOW, 0.45);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Lab graphite. Dark glass panels on a graphite wall, a thin arc-blue light along the top and the rail washing the wall a little, brighter at night. */
  ironmanglass(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x22262e); fill(L, 0, w, 2, 0x16191f);
    for (let x = from(L, 32); x < R; x += 32) { fill(x, 3, 1, 26, 0x2e333c); fill(x + 1, 3, 1, 26, 0x1a1d24); }
    fill(L, 2, w, 1, IM_HOLO, 0.7); fill(L, 3, w, 2, IM_HOLO, isNight ? 0.12 : 0.05);
    fill(L, 30, w, 8, 0x1a1d24); fill(L, 30, w, 1, IM_HOLO, 0.8); fill(L, 27, w, 3, IM_HOLO, isNight ? 0.1 : 0.04);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Web-slinger wall. Spider-Man red above, a web strung down from the top every so often in thin dark lines, his blue below the rail. */
  marvel1webwall(L, R) {
    const w = R - L, thread = 0x3a0a10;
    fill(L, 0, w, 30, 0xb0262c); fill(L, 0, w, 2, 0x7a161a);
    for (let cx = from(L, 56, 20); cx < R + 28; cx += 56) {
      for (const d of [-26, -13, 0, 13, 26]) line(cx, 0, cx + d, 29, thread, 0.32);
      for (const rr of [8, 16, 24]) for (let k = 1; k < 12; k++) {
        const a = (k / 12) * Math.PI, px = cx + Math.round(Math.cos(a) * rr * 1.1), py = Math.round(Math.sin(a) * rr);
        if (py < 30 && px >= L && px < R) dot(px, py, thread, 0.32);
      }
    }
    fill(L, 30, w, 8, 0x1f3c88); fill(L, 30, w, 1, 0x14244e); fill(L, 31, w, 1, 0x3a5aa8, 0.6);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Last City white and gold. Clean white wall panels divided by thin gold lines, a teal light strip under a dark cornice, over a pale stone wainscot. */
  destinylastcity(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xeceae4); fill(L, 0, w, 3, 0x2a3a4a); fill(L, 3, w, 1, DST_TEAL, 0.8);
    for (let x = from(L, 32); x < R; x += 32) { fill(x, 4, 1, 26, DST_GOLD, 0.7); fill(x + 1, 4, 1, 26, WHITE, 0.6); }
    fill(L, 30, w, 8, 0xc8c4bc); fill(L, 30, w, 1, DST_GOLD); for (let x = from(L, 16); x < R; x += 16) fill(x, 31, 1, 7, 0xaeaaa2);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Void purple. A deep violet wall split by faint glowing fissures of Void light, darker below. */
  destinyvoid(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x2a1a3e); fill(L, 0, w, 2, 0x16101e);
    for (let x = from(L, 40); x < R; x += 40) {
      const h = hash(x, 13); let px = x + h % 30, py = 4 + (h >>> 5) % 6;
      for (let k = 0; k < 9; k++) { const nx = px + 1 + ((h >>> k) % 3) - 1, ny = py + 2; line(px, py, nx, ny, DST_VOID, 0.55); px = nx; py = ny; if (py > 28) break; }
    }
    fill(L, 30, w, 8, 0x1e1430); fill(L, 30, w, 1, DST_VOID, 0.8);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Vice City pink and teal. A pastel pink wall with three teal and white speed stripes along its top, over a teal wainscot edged in white. */
  gtavicecity(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xf8d0dc);
    fill(L, 3, w, 2, GTA_TEAL); fill(L, 6, w, 1, WHITE); fill(L, 8, w, 1, GTA_TEAL); fill(L, 10, w, 1, 0xf8a8c4);
    fill(L, 30, w, 8, 0x2ab8b0); fill(L, 30, w, 1, WHITE); fill(L, 33, w, 1, 0x5ad8d0);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Graffiti brick. Dark red brick in courses, tagged here and there in spray paint, over a grey concrete footing. */
  gtabrick(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x5a2a20);
    for (let y = 0, r = 0; y < 30; y += 4, r++) for (let x = from(L, 10, (r % 2) * 5); x < R; x += 10) {
      const h = hash(Math.floor(x / 5), r); fill(x, y, 9, 3, [0x8a3a2a, 0x7a3424, 0x9a4430][h % 3]); fill(x, y, 9, 1, 0xa0503a, 0.6);
    }
    for (let x = from(L, 48); x < R; x += 48) {
      const h = hash(x, 11), c = [GTA_PINK, 0x3ad86a, GTA_TEAL, 0xf2c43a][h % 4], tx = x + h % 24, ty = 8 + (h >>> 4) % 12;
      for (let k = 0; k < 8; k++) dot(tx + k, ty + Math.round(Math.sin(k * 1.3 + h) * 2), c, 0.85);
      line(tx + 1, ty - 2, tx + 3, ty + 3, c, 0.7);
    }
    fill(L, 30, w, 8, 0x3a3a40); fill(L, 30, w, 1, 0x6a6a72);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Stormwind stone. Pale ashlar blocks in courses, under a royal-blue band trimmed in gold, over a darker stone wainscot. */
  wowstormwind(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xd8ccb0);
    for (let y = 5, r = 0; y < 30; y += 6, r++) { fill(L, y, w, 1, 0xbcae90); for (let x = from(L, 16, (r % 2) * 8); x < R; x += 16) fill(x, y + 1, 1, 5, 0xbcae90); }
    for (let x = from(L, 16); x < R; x += 16) dot(x + 5 + hash(x, 1) % 6, 8 + hash(x, 2) % 18, 0xc4b698);
    fill(L, 0, w, 4, 0x2a4a8a); fill(L, 4, w, 1, 0xd8b04a);
    fill(L, 30, w, 8, 0x9a8a6a); fill(L, 30, w, 1, 0xd8b04a); for (let x = from(L, 12); x < R; x += 12) fill(x, 31, 1, 7, 0x7a6a50);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Orgrimmar hide. Red-brown hides stretched in panels, stitched along their seams, over an iron band with rivets and a wainscot of dark logs. */
  wowhorde(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x7a3a24); fill(L, 0, w, 2, 0x2a1a14);
    for (let x = from(L, 24); x < R; x += 24) { fill(x, 2, 1, 26, 0x5a2614); for (let y = 4; y < 28; y += 4) { dot(x - 1, y, 0xc8a878); dot(x + 1, y + 1, 0xc8a878); } fill(x + 6 + hash(x, 4) % 10, 6 + hash(x, 6) % 16, 3, 2, 0x8a4a2e); }
    fill(L, 26, w, 3, 0x6a6a72); fill(L, 26, w, 1, 0x9a9aa0); for (let x = from(L, 8); x < R; x += 8) dot(x + 3, 27, 0x3a3a42);
    fill(L, 29, w, 9, 0x4a2a1a); for (let x = from(L, 5); x < R; x += 5) { fill(x, 29, 1, 9, 0x2e1a10); dot(x + 2, 30, 0x6a4028); }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Rift jungle. A dark green wall hung with leafy vines, over a wainscot of mossy grey stone blocks. */
  loljungle(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x24402c); fill(L, 0, w, 2, 0x16281c);
    for (let x = from(L, 11); x < R; x += 11) {
      const h = hash(x, 3), len = 6 + h % 14;
      fill(x + h % 5, 0, 1, len, 0x3a6a3a);
      for (let k = 2; k < len; k += 3) dot(x + h % 5 + (k % 2 ? 1 : -1), k, 0x4a8a44);
    }
    fill(L, 28, w, 10, 0x5a5a4a);
    for (let x = from(L, 12), i = 0; x < R; x += 12, i++) {
      fill(x, 28, 1, 10, 0x3e3e34); fill(x + 6, 33, 1, 5, 0x3e3e34); fill(x, 33, 12, 1, 0x3e3e34);
      fill(x + 1, 28, 11, 1, 0x7a7a64);
      if (hash(x, 9) % 2) fill(x + 2 + hash(x, 5) % 5, 28, 4, 1, 0x4a7a3a);
    }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Route sky. A clear blue sky with a few white clouds (a starry night after dark), over a hedge of round green bushes along the bottom. */
  pokemonroute(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, isNight ? 0x16244a : 0x8cc8f0); fill(L, 0, w, 2, isNight ? 0x0e1a38 : 0x78b8e8); fill(L, 2, w, 6, isNight ? 0x132042 : 0x84c2ee);
    for (let x = from(L, 64); x < R; x += 64) {
      const h = hash(x, 7), cx = x + 6 + (h >>> 4) % 40, cy = 7 + h % 9;
      if (isNight) { dot(cx, cy, WHITE, 0.8); dot(cx + 17, cy + 9, WHITE, 0.6); dot(cx + 9, cy - 4, WHITE, 0.5); continue; }
      fill(cx, cy, 13, 3, 0xf6fbff); fill(cx + 3, cy - 2, 6, 2, 0xf6fbff); fill(cx + 1, cy + 2, 12, 1, 0xd6eaf8);
    }
    // The hedge: round bushes over a dark base
    fill(L, 30, w, 8, 0x2e7a2c);
    for (let x = from(L, 8); x < R; x += 8) { disc(x + 4, 30, 4, 0x3c8a34); dot(x + 2, 28, 0x5aac4a); dot(x + 3, 27, 0x5aac4a); dot(x + 6, 32, 0x24662a); }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Pastel pink. Soft pink walls with a frieze of little white paw prints walking along under the ceiling, over a cream wainscot of tongue-and-groove boards under a honey wood rail. */
  catcafepink(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xf2c8d0); fill(L, 0, w, 2, 0xe8b0bc);
    for (let x = from(L, 9); x < R; x += 9) { const py = 5 + ((x / 9) % 2) * 2; dot(x, py, WHITE, 0.8); dot(x - 1, py - 2, WHITE, 0.8); dot(x + 1, py - 2, WHITE, 0.8); dot(x - 2, py - 1, WHITE, 0.7); dot(x + 2, py - 1, WHITE, 0.7); }
    fill(L, 30, w, 1, 0xc8945a); fill(L, 31, w, 7, 0xf6eee0); for (let x = from(L, 4); x < R; x += 4) fill(x, 31, 1, 7, 0xe8dcc8);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Café mint. Pale mint walls with a border of little fish swimming along under the ceiling, over the same cream boards and honey rail. */
  catcafemint(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xc8e8d8); fill(L, 0, w, 2, 0xa8d4c0);
    for (let x = from(L, 12); x < R; x += 12) { fill(x, 5, 4, 2, 0xf6d878); dot(x + 4, 4, 0xf6d878); dot(x + 4, 7, 0xf6d878); dot(x + 1, 5, 0x3a3a40); }
    fill(L, 30, w, 1, 0xc8945a); fill(L, 31, w, 7, 0xf6eee0); for (let x = from(L, 4); x < R; x += 4) fill(x, 31, 1, 7, 0xe8dcc8);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Peeling damask. Dark plum Victorian damask wallpaper peeling in strips to show grey plaster, a paler patch where a picture once hung, over a black wainscot under a dark wood rail; cobwebs in the top corners. */
  halloweendxdamask(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x3a2440); fill(L, 0, w, 2, 0x24162a);
    for (let y = 4, r = 0; y < 30; y += 8, r++) for (let x = from(L, 10, (r % 2) * 5); x < R; x += 10) {
      for (const [dx, dy] of [[0, -2], [-1, -1], [1, -1], [-2, 0], [2, 0], [-1, 1], [1, 1], [0, 2], [0, 0]]) dot(x + dx, y + dy, 0x4a3054);
      dot(x, y, 0x5a3a64);
    }
    for (let x = from(L, 53, 17); x < R; x += 53) { const h = hash(x, 2), len = 6 + h % 8; fill(x, 3, 3, len, 0x4e424e); dot(x + 1, 3 + len, 0x4e424e); line(x + 3, 4, x + 4, 7 + (h % 3), 0x4a3054); }
    fill(L, 30, w, 1, 0x6a4a2a); fill(L, 31, w, 7, 0x1e1418); fill(L, 31, w, 1, 0x2a1e24);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Crypt stone. Big cold grey blocks of crypt stone, damp-stained and mossy at the joints, with a little arched niche holding a candle stub every few blocks; the stubs flicker. */
  halloweendxcrypt(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x24242a);
    for (let y = 1, r = 0; y < 30; y += 7, r++) for (let x = from(L, 16, (r % 2) * 8); x < R; x += 16) {
      const h = hash(x, y + 23), c = [0x3a3a40, 0x424248, 0x34343a][h % 3];
      fill(x, y, 15, 6, c); fill(x, y, 15, 1, shaded(c, 1.15)); if (h % 4 === 0) dot(x + (h >>> 4) % 14, y + 5, 0x4a6a3a);
    }
    for (let x = from(L, 64, 30); x < R; x += 64) { fill(x - 2, 12, 5, 6, 0x14141a); dot(x - 1, 11, 0x14141a); dot(x, 10, 0x14141a); dot(x + 1, 11, 0x14141a); fill(x, 15, 1, 3, 0xf2ead8); dot(x, 14, 0xffc060, candlelight(x * 0.1)); }
    fill(L, 30, w, 1, 0x5a4a3a); fill(L, 31, w, 7, 0x2a2a30); fill(L, 31, w, 1, 0x3a3a40);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Marquetry panels. Rich mahogany panelling inlaid with pale wood art deco diamonds and fans, framed in thin brass lines, as in a sleeping car's compartment. */
  orientmarquetry(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x6a2e1e); fill(L, 0, w, 2, 0x4a1e12); fill(L, 2, w, 1, 0xd8b45a);
    for (let x = from(L, 22); x < R; x += 22) {
      fill(x, 4, 1, 25, 0xd8b45a, 0.7); fill(x + 2, 6, 18, 21, 0x7a3a24); fill(x + 2, 6, 18, 1, 0x8a4a2e);
      const cx = x + 11;
      for (let k = 0; k < 6; k++) { fill(cx - k, 10 + k, 1 + k * 2, 1, 0xe8d0a0, 0.55); fill(cx - k, 21 - k, 1 + k * 2, 1, 0xe8d0a0, 0.55); }
      dot(cx, 16, 0xd8b45a);
    }
    fill(L, 30, w, 1, 0xd8b45a); fill(L, 31, w, 7, 0x3a180e); fill(L, 31, w, 1, 0x5a2a18);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Station cream. Glazed cream tiles in a station hall, a green tile band with a line of black and gold, over a green dado. */
  orientcream(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xd8c8a0);
    for (let y = 1, r = 0; y < 26; y += 4, r++) for (let x = from(L, 8, (r % 2) * 4); x < R; x += 8) { fill(x, y, 7, 3, 0xeee2c4); fill(x, y, 7, 1, 0xf6eed8); }
    fill(L, 26, w, 4, 0x2a5a4a); fill(L, 26, w, 1, 0x3a7a64); fill(L, 28, w, 1, 0xd8b45a, 0.8);
    fill(L, 30, w, 1, 0x1e3a30); fill(L, 31, w, 7, 0x2a5a4a); fill(L, 31, w, 1, 0x3a6a58);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Casino red. Deep red walls in a faint damask of darker scrolls, a gold picture rail, and black lacquer below with a gold line. */
  casinored(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x8a1a24); fill(L, 0, w, 2, 0x6a1018); fill(L, 2, w, 1, 0xd8b45a);
    for (let y = 6, r = 0; y < 28; y += 8, r++) for (let x = from(L, 10, (r % 2) * 5); x < R; x += 10) {
      for (const [dx, dy] of [[0, -2], [-1, -1], [1, -1], [-2, 0], [2, 0], [-1, 1], [1, 1], [0, 2]]) dot(x + dx, y + dy, 0x7a1420);
    }
    fill(L, 30, w, 1, 0xd8b45a); fill(L, 31, w, 7, 0x2a1418); fill(L, 34, w, 1, 0xd8b45a, 0.5);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Neon night. A near-black violet wall with a pink neon tube along the top and a cyan one at rail height, both glowing, more at night. */
  casinoneon(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x1e1a2e); fill(L, 0, w, 2, 0x14101e);
    for (let x = from(L, 6); x < R; x += 6) if (hash(x, 3) % 4 === 0) dot(x, 8 + hash(x, 5) % 18, 0x2a2440);
    fill(L, 4, w, 1, 0xff4ab8); fill(L, 3, w, 1, 0xff4ab8, 0.25); fill(L, 5, w, 1, 0xff4ab8, 0.25);
    fill(L, 30, w, 1, 0x4ae8f0); fill(L, 29, w, 1, 0x4ae8f0, 0.25); fill(L, 31, w, 7, 0x14101e);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Castle stone. Big grey-violet blocks of castle stone with dark mortar, a copper conduit pipe running along under the ceiling, over a darker stone wainscot under a copper rail. */
  madlabstone(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x3e3a44);
    for (let y = 2, r = 0; y < 30; y += 7, r++) for (let x = from(L, 14, (r % 2) * 7); x < R; x += 14) {
      const h = hash(x, y + 9), c = [0x5a5660, 0x625e68, 0x524e58][h % 3];
      fill(x, y, 13, 6, c); fill(x, y, 13, 1, shaded(c, 1.15)); if (h % 5 === 0) dot(x + 3 + (h >>> 4) % 8, y + 3, 0x4a6a4a);
    }
    fill(L, 3, w, 2, 0xb87333); fill(L, 3, w, 1, 0xe0a060); for (let x = from(L, 40, 12); x < R; x += 40) fill(x, 2, 2, 4, 0x8a4a22);
    fill(L, 30, w, 1, 0xb87333); fill(L, 31, w, 7, 0x3a3640); fill(L, 31, w, 1, 0x4a4650);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Asylum tile. Old pale green tiles up to the dado, some cracked, above a band of darker green tiles and a dark green rail. */
  madlabtile(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xa8b8a0);
    for (let y = 1, r = 0; y < 30; y += 5, r++) for (let x = from(L, 5); x < R; x += 5) {
      const h = hash(x, y + 13); fill(x, y, 4, 4, h % 11 === 0 ? 0xb8c8b0 : 0xc8d8c0); if (h % 23 === 0) line(x, y, x + 3, y + 3, 0x8a9a88);
    }
    fill(L, 30, w, 1, 0x4a6a4a); for (let x = from(L, 5); x < R; x += 5) fill(x, 31, 4, 7, 0x6a8a6a); fill(L, 31, w, 1, 0x7a9a7a);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Yellowed plaster. Old plaster gone the colour of weak tea, stained darker near the ceiling, with a frosted strip-light glow over a dark varnished wainscot and rail. */
  noirplaster(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xc8b48a); fill(L, 0, w, 3, 0xa8946a); fill(L, 3, w, 2, 0xb8a47a);
    for (let x = from(L, 23); x < R; x += 23) { const h = hash(x, 77); if (h % 3 === 0) fill(x + (h >>> 4) % 10, 5, 4, 6 + (h >>> 8) % 6, 0xb8a47a, 0.6); }
    fill(L, 30, w, 1, 0x2e1e14); fill(L, 31, w, 7, 0x4a3020); for (let x = from(L, 14); x < R; x += 14) fill(x, 31, 1, 7, 0x3a2418); fill(L, 31, w, 1, 0x6a4a32);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Precinct green. Institutional green paint, a darker band of it below a black line at waist height, as in every station house. */
  noirgreen(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x4a6a58); fill(L, 0, w, 2, 0x3a5446);
    fill(L, 22, w, 1, 0x1e2a22); fill(L, 23, w, 15, 0x2a3a30); fill(L, 23, w, 1, 0x3a4a40);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Ward mint. Soft mint-green walls with a wooden handrail along them on steel brackets and a paler mint bumper strip below, over a sage wainscot. */
  hospitalmint(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xb8dcc8); fill(L, 0, w, 2, 0xa0c8b4);
    fill(L, 22, w, 2, 0xc8a070); fill(L, 22, w, 1, 0xe0c090); fill(L, 24, w, 1, SHADOW);
    for (let x = from(L, 30, 6); x < R; x += 30) fill(x, 24, 1, 2, 0xa8adb6);
    fill(L, 30, w, 1, 0x8ab8a4); fill(L, 31, w, 7, 0x8ab8a4); fill(L, 31, w, 1, 0xa8d0bc);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Clinic white. Clean white walls with a single blue stripe at eye level and a blue wainscot under a darker blue rail. */
  hospitalwhite(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xf2f4f4); fill(L, 0, w, 2, 0xdce2e4);
    fill(L, 18, w, 2, 0x4c7ab8);
    fill(L, 30, w, 1, 0x2a5a9a); fill(L, 31, w, 7, 0x4c7ab8); fill(L, 31, w, 1, 0x6a94cc);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** White kitchen tile. White subway tiles in staggered rows with pale grey grout, a stainless steel rail and a brushed steel splash panel below. */
  kitchentile(L, R) {
    const w = R - L;
    fill(L, 0, w, 30, 0xd8dcd8);
    for (let y = 1, r = 0; y < 30; y += 4, r++) for (let x = from(L, 8, (r % 2) * 4); x < R; x += 8) {
      fill(x, y, 7, 3, 0xeef0ee); fill(x, y, 7, 1, 0xfafcfa); if (hash(x, y) % 9 === 0) dot(x + 2, y + 1, WHITE, 0.7);
    }
    fill(L, 30, w, 2, 0x6e727c); fill(L, 30, w, 1, 0xa8adb6);
    fill(L, 32, w, 6, 0xa8adb6); for (let x = from(L, 3); x < R; x += 3) fill(x, 32, 1, 6, 0xb8bcc4, 0.6); fill(L, 32, w, 1, 0xc8ccd2);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Bistro brick. Warm brick washed with old lime, some showing redder through it, over dark varnished wood panelling under a dark rail. */
  kitchenbrick(L, R) {
    const w = R - L;
    fill(L, 0, w, 30, 0xa8806a);
    for (let y = 1, r = 0; y < 30; y += 4, r++) for (let x = from(L, 10, (r % 2) * 5); x < R; x += 10) {
      const h = hash(x, y + 5), c = [0xc8a088, 0xd4b098, 0xb8846a, 0xc89a80][h % 4];
      fill(x, y, 9, 3, c); fill(x, y, 9, 1, shaded(c, 1.08));
    }
    fill(L, 30, w, 1, 0x3a2418); fill(L, 31, w, 7, 0x5a3b25); for (let x = from(L, 16); x < R; x += 16) fill(x, 31, 1, 7, 0x3e2818); fill(L, 31, w, 1, 0x7a5232);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Volcanic rock. Rough dark rock in lumps of brown-grey, split by thin zigzag veins of glowing lava, over a wainscot of black basalt under a rust-red rail. */
  volcanorock(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x3a302c); fill(L, 0, w, 3, 0x2a2220);
    for (let x = from(L, 9), c = 0; x < R; x += 9, c++) for (let y = 3, r = 0; y < 29; y += 6, r++) {
      const h = hash(x, y + 71), tone = [0x40342e, 0x362c28, 0x463a34][h % 3];
      fill(x + (r % 2) * 4, y, 6 + (h % 3), 4, tone); dot(x + (r % 2) * 4, y, shaded(tone, 1.25));
    }
    for (let x = from(L, 37, 11); x < R; x += 37) {
      let vx = x;
      for (let y = 4; y < 30; y++) { vx += (hash(x, y) % 3) - 1; dot(vx, y, y % 5 === 0 ? 0xffa040 : 0xc8401e); }
    }
    fill(L, 30, w, 1, 0x8a3a1e); fill(L, 31, w, 7, 0x221c1a); fill(L, 31, w, 1, 0x3a302c); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Lair steel. Rust-red riveted steel plates with dark seams, a band of yellow and black hazard stripes for a rail over a gunmetal wainscot. */
  volcanosteel(L, R) {
    const w = R - L;
    fill(L, 0, w, 30, 0x5a3a36); fill(L, 0, w, 2, 0x3e2826);
    for (let x = from(L, 24); x < R; x += 24) {
      fill(x, 2, 1, 28, 0x3e2826); fill(x + 1, 2, 1, 28, 0x6e4a44);
      for (const ry of [4, 15, 26]) { dot(x + 3, ry, 0x8a6a62); dot(x + 21, ry, 0x8a6a62); }
    }
    fill(L, 14, w, 1, 0x3e2826);
    for (let x = from(L, 4); x < R; x += 4) { fill(x, 30, 2, 2, 0xf2c830); fill(x + 2, 30, 2, 2, 0x1e1e22); }
    fill(L, 32, w, 6, 0x2a2a30); fill(L, 32, w, 1, 0x3a3c44); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Blush branch. Soft blush-pink plaster with faint painted blossom sprigs scattered across it, over a pale birch wainscot under a white rail. */
  sakurablush(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xf6d8e0); fill(L, 0, w, 2, 0xecc4d0);
    for (let x = from(L, 26, 7); x < R; x += 26) {
      const h = hash(x, 11), y0 = 6 + h % 16;
      line(x, y0, x + 6, y0 - 3, 0xc8a0a8, 0.7);
      for (const [dx, dy] of [[2, -1], [5, -3], [6, -2]]) { dot(x + dx, y0 + dy, 0xf6f2f2); dot(x + dx + 1, y0 + dy, 0xf2b8c8, 0.8); }
    }
    fill(L, 30, w, 1, 0xf6f2ea); fill(L, 31, w, 7, 0xe8d4b8); for (let x = from(L, 8); x < R; x += 8) fill(x, 31, 1, 7, 0xd8c4a8);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Spring sky. Pale spring-sky blue with soft white clouds drifting across it here and there, over a white wainscot under a pink rail. */
  sakurasky(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xc8e4f4); fill(L, 0, w, 2, 0xb0d4ec);
    for (let x = from(L, 40, 10); x < R; x += 40) { const h = hash(x, 21), cy = 6 + h % 14; fill(x, cy, 12, 2, WHITE, 0.75); fill(x + 3, cy - 2, 6, 2, WHITE, 0.75); fill(x + 8, cy - 1, 5, 1, WHITE, 0.6); }
    fill(L, 30, w, 1, 0xf28fb0); fill(L, 31, w, 7, 0xf6f2ea); fill(L, 31, w, 1, WHITE); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Station red. Insulated panels in polar-station red with riveted aluminium joins and a little frost in their lower corners, over a grey wainscot under a white rail. */
  arcticbasered(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xb8302a); fill(L, 0, w, 2, 0x9a2420);
    for (let x = from(L, 24); x < R; x += 24) { fill(x, 2, 2, 28, 0xa8b2ba); fill(x, 2, 1, 28, 0xd0d6da); for (let y = 5; y < 29; y += 6) dot(x, y, 0x6a747c); dot(x + 3, 28, WHITE, 0.6); dot(x + 4, 29, WHITE, 0.4); dot(x - 2, 28, WHITE, 0.5); }
    fill(L, 15, w, 1, 0x9a2420);
    fill(L, 30, w, 1, 0xe8ecee); fill(L, 31, w, 7, 0x5a626a); fill(L, 31, w, 1, 0x6a727a); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Pine common room. Warm knotty pine boards running upright, a shelf-rail of darker wood, over a wainscot of horizontal boards. */
  arcticbasepine(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xc8945a); fill(L, 0, w, 2, 0xa8784a);
    for (let x = from(L, 6); x < R; x += 6) { fill(x, 2, 1, 28, 0xa8784a); const h = hash(x, 2); if (h % 4 === 0) { dot(x + 3, 6 + h % 18, 0x8a5a32); dot(x + 3, 7 + h % 18, 0x9a6a3a); } }
    fill(L, 30, w, 1, 0x5a3b25); fill(L, 31, w, 7, 0x8a5a32); fill(L, 34, w, 1, 0x6a4428); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Research white. Padded white wall panels with rounded seams and little recessed lights, an orange safety stripe along them, over a grey wainscot under an orange rail. */
  submarinedeepwhite(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xe8ecee); fill(L, 0, w, 2, 0xd0d6da);
    for (let x = from(L, 18); x < R; x += 18) { fill(x, 2, 1, 28, 0xc8d0d4); fill(x + 1, 2, 1, 28, 0xf4f8fa); dot(x + 9, 4, 0xfff4c0); }
    fill(L, 9, w, 1, 0xd0d6da); fill(L, 20, w, 2, 0xe8702a); fill(L, 20, w, 1, 0xf2904a);
    fill(L, 30, w, 1, 0xe8702a); fill(L, 31, w, 7, 0x8a96a0); fill(L, 31, w, 1, 0xa8b2ba); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Abyss navy. Dark navy hull plating with fine seams and bolt heads, a thin strip of cyan light along it and another under the rail, over a darker wainscot. */
  submarinedeepnavy(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x12203a); fill(L, 0, w, 2, 0x0c162a);
    for (let x = from(L, 22); x < R; x += 22) { fill(x, 2, 1, 28, 0x0c162a); for (let y = 5; y < 28; y += 6) { dot(x + 2, y, 0x2a3a5a); dot(x - 2, y, 0x2a3a5a); } }
    fill(L, 15, w, 1, 0x2ad8d8, 0.7); fill(L, 16, w, 1, 0x2ad8d8, 0.2);
    fill(L, 30, w, 1, 0x2ad8d8); fill(L, 31, w, 7, 0x0c162a); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Mexican pink. Hot pink plaster with a stencilled band of little white cut-paper flowers along the top, over a turquoise wainscot under a yellow rail. */
  muertospink(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xe83a8a); fill(L, 0, w, 2, 0xc82a72);
    fill(L, 3, w, 6, 0xd02a7a);
    for (let x = from(L, 8); x < R; x += 8) { dot(x + 3, 5, WHITE, 0.7); dot(x + 2, 6, WHITE, 0.7); dot(x + 4, 6, WHITE, 0.7); dot(x + 3, 7, WHITE, 0.7); dot(x + 3, 6, 0xf2c41a); dot(x + 7, 4, WHITE, 0.4); dot(x + 7, 8, WHITE, 0.4); }
    fill(L, 30, w, 1, 0xf2c41a); fill(L, 31, w, 7, 0x1a8a8a); fill(L, 31, w, 1, 0x2aa8a8);
    for (let x = from(L, 10, 4); x < R; x += 10) { dot(x, 34, 0xf2c41a); dot(x + 1, 34, 0xe83a8a); }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Painted turquoise. Deep turquoise plaster painted with a vine of little folk flowers in pink, yellow and white winding along it, over a marigold wainscot under a yellow rail. */
  muertosteal(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x1a8a8a); fill(L, 0, w, 2, 0x12706e);
    for (let x = from(L, 2); x < R; x += 2) dot(x, 22 + Math.round(Math.sin(x * 0.25) * 2), 0x2a6a3a);
    for (let x = from(L, 14, 3); x < R; x += 14) {
      const c = [0xe83a8a, 0xf2c41a, 0xf6f2ea][hash(x, 4) % 3], cy = 22 + Math.round(Math.sin(x * 0.25) * 2) - 2;
      dot(x, cy - 1, c); dot(x - 1, cy, c); dot(x + 1, cy, c); dot(x, cy + 1, c); dot(x, cy, 0xe8902a); dot(x + 3, cy + 3, 0x3a8a4a);
    }
    fill(L, 30, w, 1, 0xf2c41a); fill(L, 31, w, 7, 0xe8902a); fill(L, 31, w, 1, 0xf2a84a); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Cream boiserie. Cream-painted wall panels outlined in thin gilt mouldings with little corner flourishes, over a panelled cream wainscot under a gilt rail. */
  pariscafeboiserie(L, R) {
    const w = R - L, gilt = 0xc8a050;
    fill(L, 0, w, 40, 0xefe4cc); fill(L, 0, w, 2, 0xe0d2b4); fill(L, 2, w, 1, gilt, 0.6);
    for (let x = from(L, 30, 3); x < R; x += 30) {
      fill(x, 5, 24, 1, gilt, 0.7); fill(x, 26, 24, 1, gilt, 0.7); fill(x, 5, 1, 22, gilt, 0.7); fill(x + 23, 5, 1, 22, gilt, 0.7);
      for (const [cx, cy] of [[x + 2, 7], [x + 21, 7], [x + 2, 24], [x + 21, 24]]) dot(cx, cy, gilt, 0.6);
    }
    fill(L, 30, w, 1, gilt); fill(L, 31, w, 7, 0xe0d2b4);
    for (let x = from(L, 15, 2); x < R; x += 15) { fill(x, 32, 12, 5, 0xe8dcc2); fill(x, 32, 12, 1, 0xd0c2a4); }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Bordeaux. Deep wine-red walls with a band of antique mirror tiles flecked with silver along the top, over dark wooden panelling under a brass rail. */
  pariscafebordeaux(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x5a1a24); fill(L, 0, w, 2, 0x40121a);
    fill(L, 3, w, 5, 0x8a8478); for (let x = from(L, 8); x < R; x += 8) { fill(x, 3, 1, 5, 0x5a5448); const h = hash(x, 3); dot(x + 2 + h % 5, 4 + (h >>> 3) % 3, 0xd8d4cc); }
    fill(L, 8, w, 1, 0xc8a050);
    fill(L, 30, w, 1, 0xc8a050); fill(L, 31, w, 7, 0x3a2418);
    for (let x = from(L, 12); x < R; x += 12) fill(x, 32, 1, 5, 0x2a1810);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Exposed brick. Old red brick in uneven tones with pale, worn mortar, a black steel beam along the top, over a dark painted wainscot. */
  nyloftbrick(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xc8b8a8); fill(L, 0, w, 3, 0x2a2a30); fill(L, 3, w, 1, 0x4a4a52);
    for (let x = from(L, 12); x < R; x += 12) dot(x + 6, 1, 0x5a5a62);
    for (let y = 4, r = 0; y < 30; y += 4, r++) for (let x = from(L, 10, (r % 2) * 5); x < R; x += 10) {
      const h = hash(x, r + 3), tone = [0x9a4a32, 0x8a4230, 0xa8563a, 0x7a3a2a][h % 4];
      fill(x, y, 9, 3, tone); fill(x, y, 9, 1, shaded(tone, 1.1)); if (h % 9 === 0) fill(x + 2, y + 1, 4, 1, 0xd8c8b8, 0.4);
    }
    fill(L, 30, w, 1, 0x2a2a30); fill(L, 31, w, 7, 0x3a3a40); fill(L, 31, w, 1, 0x4a4a52); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** White plaster. Warm white plaster with faint trowel marks, an exposed black steel beam along the top and round-headed rivets, over a pale wainscot. */
  nyloftplaster(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xece8e0); fill(L, 0, w, 4, 0x2a2a30); fill(L, 4, w, 1, 0x4a4a52);
    for (let x = from(L, 8); x < R; x += 8) dot(x + 4, 2, 0x5a5a62);
    for (let x = from(L, 23, 4); x < R; x += 23) { const h = hash(x, 5); fill(x, 8 + h % 14, 9, 1, 0xe0dcd2); fill(x + 4, 12 + (h >>> 4) % 12, 7, 1, 0xf4f2ec); }
    fill(L, 30, w, 1, 0x2a2a30); fill(L, 31, w, 7, 0xd8d4cc); fill(L, 31, w, 1, 0xe4e0d8); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Arena dark. Near-black walls with the soft round pools of distant spotlights on them, over a black wainscot under a red rail. */
  ufcarena(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x16161c); fill(L, 0, w, 2, 0x0e0e12);
    for (let x = from(L, 40, 12); x < R; x += 40) { fill(x - 5, 4, 10, 18, 0x22222c); fill(x - 3, 2, 6, 22, 0x22222c); fill(x - 2, 6, 4, 12, 0x2a2a36); dot(x, 3, 0xfff4c0, 0.6); }
    fill(L, 30, w, 1, 0xd8202a); fill(L, 31, w, 7, 0x0e0e12); fill(L, 31, w, 1, 0x1e1e26); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Gym brick. Old red-brown brick painted over in places, a black band with a red stripe along it under the rail, and a dark wainscot. */
  ufcgym(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x7a3a2a); fill(L, 0, w, 2, 0x5a2a1e);
    for (let y = 2, r = 0; y < 26; y += 4, r++) { fill(L, y + 3, w, 1, 0x5a2a1e); for (let x = from(L, 10, (r % 2) * 5); x < R; x += 10) { fill(x, y, 1, 3, 0x5a2a1e); if (hash(x, r) % 6 === 0) fill(x + 1, y, 9, 3, 0x8a4a36); } }
    fill(L, 26, w, 4, 0x1c1a22); fill(L, 27, w, 1, 0xd8202a);
    fill(L, 30, w, 1, 0xd8202a); fill(L, 31, w, 7, 0x2a2a30); fill(L, 31, w, 1, 0x3a3a42); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Pit garage white. Glossy white garage panels with fine seams, a red team stripe and a thin black one across the middle, over a grey wainscot under a red rail. */
  f1pitwall(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xeeeeee); fill(L, 0, w, 2, 0xdedede);
    for (let x = from(L, 24); x < R; x += 24) fill(x, 2, 1, 28, 0xdcdcdc);
    fill(L, 19, w, 3, 0xd81a1a); fill(L, 19, w, 1, 0xe84040); fill(L, 23, w, 1, 0x1c1a22);
    fill(L, 30, w, 1, 0xd81a1a); fill(L, 31, w, 7, 0x8a8e96); fill(L, 31, w, 1, 0xa8adb6); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Carbon weave. Dark carbon fibre in a fine two-tone twill, a red pinstripe under the rail and a darker wainscot. */
  f1carbon(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x24262a); fill(L, 0, w, 2, 0x1a1c20);
    for (let y = 2; y < 30; y += 2) for (let x = from(L, 4, y % 4); x < R; x += 4) fill(x, y, 2, 1, 0x30333a);
    fill(L, 29, w, 1, 0xd81a1a); fill(L, 30, w, 1, 0x3a3c42); fill(L, 31, w, 7, 0x16181c); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Mansion panelling. Deep green damask wallpaper, its little diamond motif in a lighter green, over raised mahogany panels under a brass rail. */
  xmenmansion(L, R) {
    const w = R - L, mot = 0x4a6a4a;
    fill(L, 0, w, 40, 0x3a5a3a); fill(L, 0, w, 2, 0x2a4a2a);
    for (let y = 5, r = 0; y < 28; y += 6, r++) for (let x = from(L, 10, (r % 2) * 5); x < R; x += 10) { dot(x, y - 1, mot); dot(x - 1, y, mot); dot(x + 1, y, mot); dot(x, y + 1, mot); dot(x, y, 0x5a7a4a); }
    fill(L, 30, w, 1, 0xc8a050); fill(L, 31, w, 7, 0x5a2e1e);
    for (let x = from(L, 16, 2); x < R; x += 16) { fill(x, 32, 13, 5, 0x6a3a26); fill(x, 32, 13, 1, 0x7a4a30); fill(x, 36, 13, 1, 0x4a2418); }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Hangar steel. Blue-grey steel plates with riveted seams, a yellow and black hazard band under the rail, over a dark steel wainscot. */
  xmenhangar(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x5a6270); fill(L, 0, w, 2, 0x4a5260);
    for (let x = from(L, 20); x < R; x += 20) { fill(x, 2, 1, 26, 0x4a5260); fill(x + 1, 2, 1, 26, 0x6a7280); for (let y = 5; y < 28; y += 7) { dot(x + 3, y, 0x7a8290); dot(x - 3, y, 0x7a8290); } }
    fill(L, 14, w, 1, 0x4a5260);
    for (let x = from(L, 6); x < R; x += 6) { fill(x, 27, 3, 3, 0xf2c41a); fill(x + 3, 27, 3, 3, 0x1c1a22); }
    fill(L, 30, w, 1, 0xf2c41a); fill(L, 31, w, 7, 0x3a4250); fill(L, 31, w, 1, 0x4a5260); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Graffiti brick. Dark plum brick in staggered courses, tagged here and there in pink, cyan and yellow spray with a drip or two, over a black wainscot under a red rail. */
  spiderversegraffiti(L, R) {
    const w = R - L, brick = 0x2a2438, mortar = 0x1e1a2a;
    fill(L, 0, w, 40, brick); fill(L, 0, w, 2, 0x1e1a2a);
    for (let y = 2, r = 0; y < 30; y += 4, r++) {
      fill(L, y + 3, w, 1, mortar);
      for (let x = from(L, 10, (r % 2) * 5); x < R; x += 10) { fill(x, y, 1, 3, mortar); if (hash(x, r + 7) % 5 === 0) fill(x + 1, y, 9, 1, 0x34304a); }
    }
    // The tags: a looping squiggle of spray, a drip below it
    for (let x = from(L, 46, 9); x < R; x += 46) {
      const h = hash(x, 91), c = [0xf28fb0, 0x30e0f0, 0xf2d35b][h % 3], ty = 10 + (h >>> 4) % 10;
      for (let k = 0; k < 9; k++) dot(x + k, ty + Math.round(Math.sin(k * 1.3 + h) * 2), c, 0.75);
      dot(x + 2, ty - 3, c, 0.6); dot(x + 6, ty + 3, c, 0.6); fill(x + 4, ty + 2, 1, 3 + (h >>> 7) % 4, c, 0.5);
    }
    fill(L, 30, w, 1, 0xd8202a); fill(L, 31, w, 7, 0x16141c); fill(L, 31, w, 1, 0x2a2630); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Gwen's pastel. Soft pink plaster washed with blotches of teal and lilac like watercolour, a halftone band of dots along the top, over a teal wainscot under a white rail. */
  spiderversegwen(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xf4b8d0); fill(L, 0, w, 2, 0xe8a0c0);
    for (let x = from(L, 3); x < R; x += 3) for (let y = 3; y < 9; y += 2) dot(x + (y % 4 === 1 ? 1 : 0), y, 0xe07aa8, 0.5 - (y - 3) * 0.07);
    for (let x = from(L, 34, 6); x < R; x += 34) {
      const h = hash(x, 57), c = h % 2 ? 0x5ac8c8 : 0xb8a0e0, wx = x + (h >>> 3) % 12, wy = 12 + (h >>> 6) % 8, ww = 10 + (h >>> 9) % 8;
      fill(wx, wy, ww, 6, c, 0.22); fill(wx + 2, wy - 1, ww - 4, 8, c, 0.14); fill(wx + 3, wy + 2, ww - 7, 2, c, 0.18);
    }
    fill(L, 30, w, 1, 0xf6f2ea); fill(L, 31, w, 7, 0x5ac8c8); fill(L, 31, w, 1, 0x7ad8d8); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Bathed in the Erdtree's light: warm gold-washed plaster with a frieze of little golden leaves and branching root lines, over dark oak. */
  eldenringerdtree(L, R) {
    const w = R - L, gold = 0xf2d070, goldDark = 0xb89040;
    fill(L, 0, w, 40, 0xc8a868); fill(L, 0, w, 2, 0xa88848);
    for (let x = from(L, 6); x < R; x += 6) { dot(x + 2, 5, gold); dot(x + 3, 4, gold); dot(x + 3, 6, goldDark); dot(x + 5, 5, goldDark, 0.6); }
    fill(L, 8, w, 1, goldDark, 0.6);
    for (let x = from(L, 24); x < R; x += 24) { line(x + 12, 10, x + 12, 28, goldDark, 0.35); line(x + 12, 16, x + 6, 22, goldDark, 0.3); line(x + 12, 14, x + 18, 20, goldDark, 0.3); }
    fill(L, 29, w, 1, gold); fill(L, 30, w, 8, 0x6a5434); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Stormveil Castle: weathered grey stone in uneven courses, streaked darker by the rain, with ivy creeping in at the top and a dark timber dado. */
  eldenringstormveil(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x4a4a46);
    for (let y = 0, r = 0; y < 30; y += 5, r++) for (let x = from(L, 11, (r % 2) * 5); x < R; x += 11) {
      const tone = [0x6a6a66, 0x72726c, 0x62625e][hash(x, r + 2) % 3];
      fill(x + 1, y + 1, 10, 4, tone); fill(x + 1, y + 1, 10, 1, shaded(tone, 1.12));
    }
    for (let x = from(L, 13); x < R; x += 13) for (let y = 4; y < 26; y += 3) dot(x + 6, y, 0x4a4a46, 0.5);
    for (let x = from(L, 7); x < R; x += 7) { const h = hash(x, 5) % 6; for (let k = 0; k < h; k++) dot(x + (k % 2), 1 + k, k % 2 ? 0x3a5a2a : 0x4a6a32); }
    fill(L, 29, w, 1, 0xb89a4a); fill(L, 30, w, 8, 0x3e3a34); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** The Grand Line: a bright blue sky with tall puffy anime clouds over a band of deep blue sea with white wave crests. */
  onepiecesea(L, R) {
    const w = R - L, cloud = isNight ? 0x6a7a98 : WHITE;
    fill(L, 0, w, 40, isNight ? 0x1a2650 : 0x5ab0e8);
    for (let x = from(L, 50); x < R + 20; x += 50) { for (const [dx, dy, r] of [[0, 18, 5], [6, 13, 6], [12, 9, 5], [17, 15, 5], [22, 19, 4]]) disc(x + dx, dy, r, cloud); fill(x - 4, 19, 30, 4, cloud); }
    fill(L, 24, w, 14, isNight ? 0x10204a : 0x1e5aa8);
    for (let x = from(L, 8); x < R; x += 8) { const ph = Math.floor(now / 2 + x / 8) % 3; dot(x + ph, 25 + (x % 3), WHITE, 0.8); dot(x + ph + 1, 26 + (x % 3), WHITE, 0.6); }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** The Thousand Sunny's cabin: warm honey planking with a rope swag looped along the top and brass portholes now and then, over dark wood with a sunny yellow rail. */
  onepiecesunny(L, R) {
    const w = R - L, rope = 0xd8b880, ropeDark = 0xa8885a;
    fill(L, 0, w, 40, 0xc8904e);
    for (let y = 4; y < 29; y += 5) fill(L, y, w, 1, 0xa8743e);
    for (let x = from(L, 20); x < R; x += 20) for (let k = 0; k < 20; k++) { const yy = 3 + Math.round(Math.sin((k / 19) * Math.PI) * 3); dot(x + k, yy, k % 2 ? rope : ropeDark); }
    fill(L, 0, w, 2, 0x8a5a32);
    fill(L, 29, w, 2, 0xf2d040); fill(L, 31, w, 7, 0x6a4424); fill(L, 31, w, 1, 0x8a5a32); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** A sietch's walls: warm orange rock cut in layers, the strata showing as wavy darker and lighter bands, over rough-hewn lower rock. */
  dunerock(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xb07a48);
    for (let k = 0; k < 6; k++) { const by = 4 + k * 4, c = k % 2 ? 0xa06e40 : 0xc08a56; for (let x = L; x < R; x++) dot(x, by + Math.round(Math.sin(x * 0.08 + k) * 1.4), c); }
    for (let x = from(L, 9); x < R; x += 9) dot(x + (hash(x, 7) % 7), 8 + (hash(x, 8) % 18), 0x8a5a34);
    fill(L, 29, w, 1, 0x5a3a22); fill(L, 30, w, 8, 0x8a5634); for (let x = from(L, 7); x < R; x += 7) fill(x, 31 + (hash(x, 9) % 4), 3, 1, 0x6a4228);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** The Residency at Arrakeen: dark green-black stone walls of House Atreides with tall narrow niches and a gold frieze of small hawks, over a band of dark carved wood. */
  duneatreides(L, R) {
    const w = R - L, gold = 0xc8a050;
    fill(L, 0, w, 40, 0x2a3a2e); fill(L, 0, w, 2, 0x1a261e); fill(L, 4, w, 1, gold); fill(L, 9, w, 1, gold);
    for (let x = from(L, 10); x < R; x += 10) { dot(x + 5, 6, gold); dot(x + 4, 7, gold); dot(x + 6, 7, gold); dot(x + 3, 6, gold, 0.7); dot(x + 7, 6, gold, 0.7); }
    for (let x = from(L, 20); x < R; x += 20) { fill(x + 8, 13, 4, 14, 0x1e2a22); fill(x + 9, 12, 2, 1, 0x1e2a22); }
    fill(L, 29, w, 1, gold); fill(L, 30, w, 8, 0x3a2e22); for (let x = from(L, 5); x < R; x += 5) dot(x + 2, 33, 0x5a4632); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** The opening titles: a black wall with a row of white circles running along it, as the gun-barrel dots roll across the screen, over a dark band and a thin red line. */
  bondgunwall(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x141418);
    for (let x = from(L, 16); x < R; x += 16) { disc(x + 8, 16, 4, 0xe8e8ec); disc(x + 8, 16, 2.5, 0x141418); dot(x + 7, 15, 0x3a3a42); }
    fill(L, 29, w, 1, 0xc8281e); fill(L, 30, w, 8, 0x1e1e24); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** M's office: rich walnut panelling in tall raised panels with brass picture lights, over a dark green leather dado with a brass rail. */
  bondwalnut(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x5a3a24); fill(L, 0, w, 3, 0x3a2416);
    for (let x = from(L, 14); x < R; x += 14) { fill(x + 2, 5, 10, 22, 0x6a4a2e); fill(x + 2, 5, 10, 1, 0x7a5a3a); fill(x + 2, 26, 10, 1, 0x4a2e1a); fill(x + 11, 5, 1, 22, 0x4a2e1a); }
    fill(L, 29, w, 1, 0xc8a050); fill(L, 30, w, 8, 0x2a5a3a); for (let x = from(L, 6); x < R; x += 6) dot(x + 2, 34, 0xc8a050, 0.7);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** The sky from the opening titles: bright blue with big puffy white clouds drifting very slowly along, over a strip of green Springfield grass. */
  simpsonssky(L, R) {
    const w = R - L;
    const cloud = isNight ? 0x6a7a98 : WHITE, cloudShade = isNight ? 0x5a6a88 : 0xd8e8f4;
    fill(L, 0, w, 40, isNight ? 0x1e2c5a : 0x6ab0f0); fill(L, 0, w, 8, isNight ? 0x16224a : 0x5aa0e8);
    if (isNight) for (let x = from(L, 23); x < R; x += 23) dot(x + (hash(x, 1) % 20), 3 + (hash(x, 2) % 8), 0xfff8d0, 0.8);
    const drift = Math.floor(now / 4) % 60;
    for (let x = from(L, 60, -drift); x < R + 30; x += 60) {
      for (const [dx, dy, r] of [[0, 14, 5], [6, 11, 6], [13, 13, 5], [18, 15, 4], [-5, 16, 3]]) disc(x + dx, dy, r, cloud);
      fill(x - 7, 16, 28, 4, cloud); fill(x - 6, 19, 26, 1, cloudShade);
    }
    fill(L, 30, w, 8, isNight ? 0x2a4a2a : 0x5a9a4a); fill(L, 30, w, 1, isNight ? 0x3a5a3a : 0x7aba5a); for (let x = from(L, 4); x < R; x += 4) dot(x + 1, 29, 0x7aba5a);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** The Simpsons' house: dusty pink walls with a faint diamond pattern, over a purple-brown skirting band. */
  simpsonspink(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xe8a8c0); fill(L, 0, w, 2, 0xc888a0);
    for (let y = 6, r = 0; y < 28; y += 6, r++) for (let x = from(L, 10, (r % 2) * 5); x < R; x += 10) { dot(x + 5, y, 0xd898b0); dot(x + 4, y + 1, 0xd898b0); dot(x + 6, y + 1, 0xd898b0); dot(x + 5, y + 2, 0xd898b0); }
    fill(L, 30, w, 1, 0x7a4a6a); fill(L, 31, w, 7, 0x9a6a8a); fill(L, 31, w, 1, 0xb07a9a); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Lannister crimson. Deep crimson walls hung with a gold border of little lion heads, over dark wood with a gold rail. */
  gotcrimson(L, R) {
    const w = R - L, gold = 0xd8b45a, goldDark = 0x9a7a30;
    fill(L, 0, w, 40, 0x8a1e1e); fill(L, 0, w, 2, 0x6a1414); fill(L, 3, w, 1, gold); fill(L, 9, w, 1, gold);
    for (let x = from(L, 10); x < R; x += 10) { fill(x + 4, 5, 3, 3, gold); dot(x + 3, 5, goldDark); dot(x + 7, 5, goldDark); dot(x + 5, 6, 0x6a1414); }
    for (let x = from(L, 20); x < R; x += 20) for (let y = 13; y < 28; y += 2) dot(x + 10, y, 0x7a1818);
    fill(L, 30, w, 1, gold); fill(L, 31, w, 7, 0x3a1a14); fill(L, 31, w, 1, 0x5a2a20); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Winterfell's walls: cold grey stone blocks, frost in the mortar near the top, over dark timber panelling. */
  gotstone(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x5a5e62);
    for (let y = 0, r = 0; y < 30; y += 6, r++) for (let x = from(L, 12, (r % 2) * 6); x < R; x += 12) {
      const tone = [0x7a7e82, 0x82868a, 0x72767a][hash(x, r + 5) % 3];
      fill(x + 1, y + 1, 11, 5, tone); fill(x + 1, y + 1, 11, 1, shaded(tone, 1.14));
      if (r === 0) dot(x + 4, y, 0xe8f0f6, 0.8);
    }
    fill(L, 29, w, 2, 0x3a2a1e); fill(L, 31, w, 7, 0x4a3a2a); for (let x = from(L, 8); x < R; x += 8) fill(x, 31, 1, 7, 0x3a2a1e);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** The Upside Down: a cold grey-violet wall overgrown with dark fleshy vines, a few spores hanging in the air. */
  strangerupsidewall(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x2e2834); fill(L, 0, w, 2, 0x1e1a24);
    for (let x = from(L, 16); x < R; x += 16) {
      const h = hash(x, 3);
      for (let y = 0; y < 30; y++) dot(x + Math.round(Math.sin(y * 0.4 + h) * 2) + 4, y, 0x4a1e28);
      for (let y = 4; y < 28; y += 7) { dot(x + 6, y, 0x5a2430); dot(x + 7, y + 1, 0x5a2430); }
      dot(x + 10 + (h % 4), 8 + (h % 12), 0x8a8490, 0.6);
    }
    fill(L, 30, w, 1, 0x5a2430); fill(L, 31, w, 7, 0x24202a); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** The Byers' living room: faded mustard wallpaper with a small brown-and-orange flower pattern, over dark wood-grain panelling. */
  strangerwallpaper(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xc8b078); fill(L, 0, w, 2, 0xa8905a);
    for (let y = 4, r = 0; y < 28; y += 6, r++) for (let x = from(L, 8, (r % 2) * 4); x < R; x += 8) {
      dot(x + 2, y, 0xc8602a); dot(x + 1, y + 1, 0x8a5a2a); dot(x + 3, y + 1, 0x8a5a2a); dot(x + 2, y + 2, 0xc8602a); dot(x + 2, y + 1, 0xe8a040);
    }
    fill(L, 29, w, 2, 0x4a3018); fill(L, 31, w, 7, 0x6a4a2a);
    for (let x = from(L, 5); x < R; x += 5) fill(x, 31, 1, 7, 0x5a3a20);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Albuquerque stucco. Warm peach adobe stucco, rough with little flecks, under a band of turquoise trim, over a terracotta wainscot. */
  breakingbadstucco(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xe0b890); fill(L, 0, w, 2, 0xc89870);
    for (let y = 3; y < 28; y += 3) for (let x = from(L, 7, (y % 2) * 3); x < R; x += 7) { const h = hash(x, y); dot(x + (h % 5), y, h % 2 ? 0xd4ac84 : 0xeac8a0); }
    fill(L, 5, w, 2, 0x5a9a9a); fill(L, 5, w, 1, 0x7ab8b8);
    fill(L, 29, w, 2, 0x5a8a8a); fill(L, 31, w, 7, 0x9a6a48); fill(L, 31, w, 1, 0xb07a52); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Superlab tiles. White ceramic tiles in a grid with grey grout, a yellow-and-black hazard band at waist height and grey steel kick plates below. */
  breakingbadsuperlab(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xc8ccd0);
    for (let y = 0; y < 28; y += 5) for (let x = from(L, 5); x < R; x += 5) { fill(x, y, 4, 4, 0xeef2f4); dot(x, y, WHITE, 0.8); }
    fill(L, 28, w, 3, 0xe8c838); for (let x = from(L, 6); x < R; x += 6) { dot(x, 28, 0x1e1e22); dot(x + 1, 29, 0x1e1e22); dot(x + 2, 30, 0x1e1e22); dot(x + 1, 28, 0x1e1e22); dot(x + 2, 29, 0x1e1e22); dot(x + 3, 30, 0x1e1e22); }
    fill(L, 31, w, 7, 0x8a9096); fill(L, 31, w, 1, 0xa8aeb4); for (let x = from(L, 12); x < R; x += 12) dot(x + 1, 34, 0x6e747c);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Rivendell. Pale elven stone with slender arches traced in relief and a frieze of climbing silver-green vines, over a soft green-grey wainscot. */
  lotrrivendell(L, R) {
    const w = R - L, vine = 0x7a9a6a, vineLight = 0xa8c098;
    fill(L, 0, w, 40, 0xd8d8c8); fill(L, 0, w, 2, 0xb8b8a8);
    for (let x = from(L, 20); x < R; x += 20) {
      fill(x + 2, 10, 1, 19, 0xc4c4b4); fill(x + 17, 10, 1, 19, 0xc4c4b4);
      for (let k = 0; k < 8; k++) { dot(x + 3 + k, 9 - Math.round(Math.sqrt(k) * 1.6), 0xc4c4b4); dot(x + 16 - k, 9 - Math.round(Math.sqrt(k) * 1.6), 0xc4c4b4); }
    }
    for (let x = L; x < R; x++) { const yy = 4 + Math.round(Math.sin(x * 0.5) * 1.5); dot(x, yy, vine); if (x % 5 === 0) { dot(x, yy - 1, vineLight); dot(x + 1, yy + 1, vineLight); } }
    fill(L, 30, w, 1, 0xc8c8b8); fill(L, 31, w, 7, 0x8a9a82); fill(L, 31, w, 1, 0xa8b89e); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Bag End. Warm cream plaster under a dark oak beam, a painted border of little green leaves and acorns, over honey oak panelling with a dark rail. */
  lotrbagend(L, R) {
    const w = R - L, leaf = 0x6a8a3a, acorn = 0x8a5a2a;
    fill(L, 0, w, 40, 0xefe2c4); fill(L, 0, w, 3, 0x5a3b25); fill(L, 3, w, 1, 0x7a5232);
    for (let x = from(L, 8); x < R; x += 8) { dot(x + 1, 6, leaf); dot(x + 2, 5, leaf); dot(x + 3, 6, shaded(leaf, 0.8)); dot(x + 5, 6, acorn); dot(x + 5, 5, 0x5a3b25); }
    for (let x = from(L, 16); x < R; x += 16) for (let y = 10; y < 28; y += 3) dot(x + ((y * 7) % 13), y, 0xe4d4b0, 0.7);
    fill(L, 29, w, 2, 0x5a3b25); fill(L, 31, w, 7, 0x8a5a32); fill(L, 31, w, 1, 0xa8703e);
    for (let x = from(L, 9); x < R; x += 9) fill(x, 32, 1, 6, 0x6a4424);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Castle stone. Big dressed stone blocks in staggered courses with pale mortar, each block a little different, darker toward the floor. */
  potterstone(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x5a5650);
    for (let y = 0, r = 0; y < 38; y += 7, r++) for (let x = from(L, 14, (r % 2) * 7); x < R; x += 14) {
      const h = hash(x, r + 91), tone = shaded([0x7a756c, 0x827d73, 0x736e66, 0x8a857c][h % 4], r > 3 ? 0.88 : 1);
      fill(x + 1, y + 1, 13, 6, tone); fill(x + 1, y + 1, 13, 1, shaded(tone, 1.12)); fill(x + 1, y + 6, 13, 1, shaded(tone, 0.85));
      if (h % 6 === 0) dot(x + 3 + ((h >>> 4) % 9), y + 3, shaded(tone, 0.8));
    }
    fill(L, 37, w, 1, 0x4a3020); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Gryffindor scarlet. Deep scarlet walls hung with a gold-fringed tapestry border at the top, little gold lions' crowns in a lattice, over dark oak panelling with a gold rail. */
  pottergryffindor(L, R) {
    const w = R - L, gold = 0xd3a625, goldDark = 0x9a7418;
    fill(L, 0, w, 40, 0x7a1c1c); fill(L, 0, w, 2, 0x5a1212);
    fill(L, 2, w, 3, 0x9a2a22); fill(L, 5, w, 1, gold);
    for (let x = from(L, 4); x < R; x += 4) { dot(x + 1, 6, gold); dot(x + 1, 7, goldDark, 0.7); }
    for (let y = 12, r = 0; y < 28; y += 8, r++) for (let x = from(L, 12, (r % 2) * 6); x < R; x += 12) {
      dot(x + 5, y, gold, 0.8); dot(x + 7, y, gold, 0.8); dot(x + 6, y - 1, gold, 0.8); fill(x + 5, y + 1, 3, 1, goldDark, 0.8);
    }
    fill(L, 30, w, 1, gold); fill(L, 31, w, 7, 0x4a3020); fill(L, 31, w, 1, 0x6a4a30);
    for (let x = from(L, 10); x < R; x += 10) { fill(x + 1, 32, 8, 5, 0x3a2416, 0.5); fill(x + 1, 32, 8, 1, 0x5a3a26, 0.6); }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Saffron and maroon. Pale saffron plaster under a painted frieze of little maroon cloud curls on a saffron band between gold lines, over a maroon wainscot with a gold rail. */
  buddhistsaffron(L, R) {
    const w = R - L, curl = 0x8a3038;
    fill(L, 0, w, 40, 0xf0d8a8); fill(L, 0, w, 2, 0xe0c088);
    fill(L, 2, w, 6, 0xe8a848); fill(L, 2, w, 1, 0xd8b45a); fill(L, 8, w, 1, 0xd8b45a);
    for (let x = from(L, 8); x < R; x += 8) { dot(x + 1, 5, curl); dot(x + 2, 4, curl); dot(x + 3, 4, curl); dot(x + 4, 5, curl); dot(x + 3, 6, curl); dot(x + 6, 5, 0xf2c070); }
    fill(L, 30, w, 1, 0xd8b45a); fill(L, 31, w, 7, 0x6a2028); fill(L, 31, w, 1, 0x8a3038); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Whitewash and oak. Whitewashed plaster with faint brush marks, over a wainscot of upright oak boards under a dark oak rail. */
  christianwhite(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xf4f0e6); fill(L, 0, w, 2, 0xe2dccc);
    for (let y = 3, r = 0; y < 29; y += 4, r++) for (let x = from(L, 11, (r * 5) % 11); x < R; x += 11) {
      const h = hash(x, r + 120);
      if (h % 3 === 0) fill(x + (h % 5), y, 3 + ((h >>> 3) % 3), 1, 0xe8e2d4, 0.8);
    }
    fill(L, 30, w, 1, 0x5a3b25); fill(L, 31, w, 7, 0x8a5a32);
    for (let x = from(L, 6); x < R; x += 6) { fill(x, 31, 1, 7, 0x6a4428); dot(x + 3, 32, 0xa8703e, 0.6); }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Teal and gold. Deep teal walls with a darker band and a thin gold line under the ceiling, a gold rail, and a dado of small glazed tiles in teal, terracotta and blue below it. */
  muslimteal(L, R) {
    const w = R - L, colours = [0x2f8a8a, 0xc8643a, 0x2f5f9e];
    fill(L, 0, w, 40, 0x2f7a7a); fill(L, 0, w, 2, 0x245e5e); fill(L, 2, w, 1, 0xd8b45a, 0.5);
    fill(L, 30, w, 1, 0xd8b45a); fill(L, 31, w, 7, 0xf2ead8);
    for (let x = from(L, 4); x < R; x += 4) {
      const k = ((Math.floor(x / 4) % 3) + 3) % 3;
      fill(x + 1, 32, 2, 2, colours[k]); fill(x + 3, 35, 2, 2, colours[(k + 2) % 3]);
    }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Carved plaster. Warm cream plaster with a frieze of small carved pointed arches under the ceiling, and a terracotta rail over a tiled terracotta dado. */
  muslimplaster(L, R) {
    const w = R - L, plaster = 0xefe2c8, shade = 0xd8c8a8, light = 0xfaf2e0;
    fill(L, 0, w, 40, plaster); fill(L, 0, w, 2, shade);
    fill(L, 2, w, 6, shade, 0.5); fill(L, 8, w, 1, shade);
    for (let x = from(L, 6); x < R; x += 6) { fill(x + 1, 4, 4, 4, shade); dot(x + 2, 3, shade); dot(x + 3, 3, shade); fill(x + 2, 5, 2, 3, plaster); dot(x + 1, 4, light); }
    fill(L, 30, w, 1, 0xa8502e); fill(L, 31, w, 7, 0xc8643a); fill(L, 31, w, 1, 0xd8845a);
    for (let x = from(L, 8); x < R; x += 8) fill(x, 31, 1, 7, 0xa8502e);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Jerusalem stone. Warm limestone blocks in staggered courses, each a slightly different cream, with chisel marks; a dark rail over a sandstone wainscot. */
  jewishsand(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xe2d0a8); fill(L, 0, w, 2, 0xcdb88c);
    for (let y = 2, r = 0; y < 30; y += 7, r++) for (let x = from(L, 18, (r % 2) * 9); x < R; x += 18) {
      const h = hash(x, r + 40), tone = [0xe8d8b4, 0xe2cfa6, 0xecdcbc, 0xdcc89c][h % 4];
      fill(x + 1, y, 17, 6, tone); fill(x + 1, y, 17, 1, shaded(tone, 1.05)); fill(x + 1, y + 5, 17, 1, shaded(tone, 0.92));
      if (h % 3 === 0) dot(x + 4 + ((h >>> 3) % 10), y + 2 + ((h >>> 7) % 3), shaded(tone, 0.86));
    }
    fill(L, 30, w, 1, 0x8a6a42); fill(L, 31, w, 7, 0xc8b48a); fill(L, 31, w, 1, 0xd8c49c); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** LED strip. A near-black wall with LED strips along its top and its rail, colours running slowly along them and washing the wall just above; brighter at night. */
  ledstrip(L, R) {
    const w = R - L, hue = Math.floor(now * 4) / 4;
    fill(L, 0, w, 40, 0x1a1a24); fill(L, 0, w, 2, 0x111118);
    fill(L, 30, w, 8, 0x14141c); fill(L, 38, w, 2, PALETTE.baseboard);
    for (let x = L; x < R; x += 4) {
      const c = rainbow((hue * 0.05 + x / 240) % 1);
      fill(x, 2, 4, 1, c); fill(x, 3, 4, 2, c, isNight ? 0.12 : 0.06);
      fill(x, 30, 4, 1, c); fill(x, 27, 4, 3, c, isNight ? 0.1 : 0.05);
    }
  },
  /** Imperial grey. Battle-station panels: dark grey plates with deep seams, a tall pale light strip in every other one and vents in the rest; a red strip along the rail. */
  starwarsimperial(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x3a3e48); fill(L, 0, w, 2, 0x2a2d35);
    for (let x = from(L, 16); x < R; x += 16) {
      fill(x, 2, 1, 28, 0x2a2d35); fill(x + 1, 2, 1, 28, 0x4a4e58);
      if (Math.floor(x / 16) % 2 === 0) { fill(x + 5, 6, 6, 20, 0x30333c); fill(x + 6, 7, 4, 18, 0xc8d4e0, 0.3); fill(x + 7, 7, 2, 18, 0xe8f0f8, 0.4); }
      else for (let k = 0; k < 4; k++) fill(x + 4, 9 + k * 4, 8, 1, 0x2a2d35);
    }
    fill(L, 30, w, 1, 0xc83a2e); fill(L, 31, w, 7, 0x24272e); fill(L, 31, w, 1, 0x34373f); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Hot pink stripes. Hot pink and lighter pink stripes down the wall above a white wainscot, a band of little white hearts under a gold rail. */
  girly1hotpink(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xe85aa0); fill(L, 0, w, 2, 0xc8307e);
    for (let x = from(L, 12); x < R; x += 12) fill(x, 2, 6, 26, 0xf070b0);
    for (let x = from(L, 12, 3); x < R; x += 12) g1Mark('smallheart', x, 25, WHITE, 0.8);
    fill(L, 29, w, 2, 0xe0b850); fill(L, 29, w, 1, 0xf8dc88); fill(L, 31, w, 7, 0xfaf0f4); fill(L, 31, w, 1, WHITE);
    for (let x = from(L, 8); x < R; x += 8) fill(x, 32, 1, 6, 0xeedae2);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Mint with polka dots. Mint walls dotted with pink and white, a scalloped cream rail and a baby pink wainscot. */
  girly2mintwall(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xc8ecd8); fill(L, 0, w, 2, 0xa8dcc0);
    for (let y = 5, r = 0; y < 28; y += 6, r++) for (let x = from(L, 10, (r % 2) * 5); x < R; x += 10) { fill(x, y, 2, 2, r % 2 ? WHITE : G2_PINK); }
    fill(L, 29, w, 2, G2_CREAM); for (let x = from(L, 4); x < R; x += 4) { fill(x, 31, 3, 1, G2_CREAM); }
    fill(L, 32, w, 6, 0xf8c8d8); fill(L, 32, w, 1, 0xfadce6); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Hacienda stucco. Cream stucco flecked with a darker grain over a dark wood rail and a band of blue and white tiles. */
  narcoshacienda(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, NC_CREAM); fill(L, 0, w, 2, 0xe0cca4);
    for (let y = 4, r = 0; y < 28; y += 5, r++) for (let x = from(L, 9, (r % 2) * 4); x < R; x += 9) if (hash(x, r) % 3 === 0) dot(x, y, 0xe0cca4);
    fill(L, 29, w, 2, 0x6a4a2a); fill(L, 29, w, 1, 0x8a6a3a);
    fill(L, 31, w, 7, 0xf4f0e8);
    for (let x = from(L, 7); x < R; x += 7) { fill(x, 31, 1, 7, 0x2a4a9a, 0.5); dot(x + 3, 34, 0x2a4a9a); dot(x + 2, 34, 0x2a4a9a, 0.6); dot(x + 4, 34, 0x2a4a9a, 0.6); dot(x + 3, 33, 0x2a4a9a, 0.6); dot(x + 3, 35, 0x2a4a9a, 0.6); }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Rainbow stripe. A warm white wall with the six rainbow stripes running along it under the rail, a beige wainscot below. */
  apple2rainbow(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0xf4f0e8); fill(L, 0, w, 2, 0xe4e0d8);
    A2_RAINBOW.forEach((c, i) => fill(L, 22 + i, w, 1, c));
    fill(L, 29, w, 2, 0xd8d0b8); fill(L, 29, w, 1, 0xe8e0c8); fill(L, 31, w, 7, A2_BEIGE); fill(L, 31, w, 1, A2_BEIGE_LIGHT); fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Hull panels. Status lights blink along the panels; a pipe runs along the rail over a hazard strip. */
  hull(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x3a404c); fill(L, 0, w, 2, 0x2a2f39);
    for (let x = from(L, 24); x < R; x += 24) {
      fill(x, 2, 1, 28, 0x2a2f39); fill(x + 1, 2, 1, 28, 0x4a515e);
      for (const ry of [4, 13, 18, 27]) { dot(x + 3, ry, 0x5f6775); dot(x + 21, ry, 0x5f6775); }
    }
    fill(L, 15, w, 1, 0x2a2f39); fill(L, 16, w, 1, 0x4a515e);
    for (let x = from(L, 48, 12); x < R; x += 48) {
      const beat = (Math.floor(now * 1.5) + Math.floor(x / 48)) % 3;
      fill(x - 1, 7, 5, 3, 0x1d1f25); dot(x, 8, 0x6fd36f, beat === 0 ? 1 : 0.3); dot(x + 2, 8, 0xf2d35b, beat === 1 ? 1 : 0.25);
    }
    fill(L, 30, w, 8, 0x2a2f39);
    fill(L, 31, w, 3, 0x6a7280); fill(L, 31, w, 1, 0x8a93a3); fill(L, 33, w, 1, 0x4a515e);
    for (let x = from(L, 40, 8); x < R; x += 40) fill(x, 30, 2, 5, 0x4a515e);
    for (let x = L; x < R; x++) for (const row of [36, 37]) dot(x, row, Math.floor((x + row) / 2) % 2 ? PALETTE.hazard : 0x1d1f25);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Ship's timbers. Planks pegged at their ends between heavy beams, and a rope for the rail. */
  timber(L, R) {
    const w = R - L;
    fill(L, 0, w, 30, 0x5a3b25); fill(L, 0, w, 2, 0x3a2616);
    for (let y = 2, row = 0; y < 30; y += 4, row++) {
      fill(L, y, w, 1, 0x4a301e); fill(L, y + 1, w, 1, 0x6b4a2e, 0.5);
      for (let x = from(L, 39, (row % 3) * 13); x < R; x += 39) { fill(x, y, 1, 4, 0x3a2616); dot(x + 2, y + 2, 0x8a6a42); dot(x - 2, y + 2, 0x8a6a42); }
    }
    for (let x = from(L, 64, 20); x < R; x += 64) { fill(x, 0, 5, 30, 0x4a301e); fill(x, 0, 1, 30, 0x6b4a2e); fill(x + 4, 0, 1, 30, 0x3a2616); }
    fill(L, 30, w, 8, 0x3a2616);
    for (let x = from(L, 16); x < R; x += 16) fill(x + 2, 32, 12, 5, 0x4a301e);
    for (let x = L; x < R; x++) { const k = ((x % 3) + 3) % 3; dot(x, 30, k === 0 ? 0x8a6a42 : 0xc9a06a); dot(x, 31, k === 1 ? 0x8a6a42 : 0xb8905a); }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Damask and gilt rail. Red damask over dark panelling, a gilt rail between them. */
  damask(L, R) {
    const w = R - L;
    fill(L, 0, w, 30, 0x6a1e2a); fill(L, 0, w, 2, 0x4e1620);
    for (let x = from(L, 8); x < R; x += 8) for (let y = 3, r = 0; y < 26; y += 7, r++) {
      const cx = x + (r % 2) * 4;
      for (const [dx, dy] of [[2, 0], [1, 1], [3, 1], [0, 2], [4, 2], [1, 3], [3, 3], [2, 4]]) dot(cx + dx, y + dy, PALETTE.brass, 0.32);
      dot(cx + 2, y + 2, PALETTE.brass, 0.7);
    }
    fill(L, 30, w, 8, 0x4a2618);
    for (let x = from(L, 20); x < R; x += 20) { fill(x + 2, 32, 16, 5, 0x5a301e); fill(x + 2, 32, 16, 1, 0x3a1c10); }
    fill(L, 30, w, 1, PALETTE.brass); fill(L, 31, w, 1, 0x9a7a3a);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Castle stone. Blocks of grey stone, moss in a few joints, a stone ledge for the rail. */
  stone(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x34363b);
    const block = (x, y, bw, bh, h) => {
      const tone = [0x55585e, 0x4d5056, 0x5c5f65][h % 3];
      fill(x, y, bw - 1, bh - 1, tone); fill(x, y, bw - 1, 1, 0x686b71);
      if (h % 7 === 0) { dot(x + 2, y + bh - 2, 0x3b5a3a); dot(x + 3, y + bh - 2, 0x4f7a3a); }
    };
    for (let y = 0, row = 0; y < 30; y += 6, row++) for (let x = from(L, 14, (row % 2) * 7); x < R; x += 14) block(x, y, 14, 6, hash(x, row));
    fill(L, 30, w, 2, 0x6a6d73); fill(L, 30, w, 1, 0x7a7d83);
    for (let x = from(L, 18); x < R; x += 18) block(x, 32, 18, 7, hash(x, 9));
    fill(L, 0, w, 2, 0x26282c);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Ice blocks. Blocks of ice catching the light, snow drifted along the bottom. */
  ice(L, R) {
    const w = R - L;
    fill(L, 0, w, 30, 0x9ec0d4); fill(L, 0, w, 2, 0x86aac0);
    for (let y = 2, row = 0; y < 30; y += 7, row++) for (let x = from(L, 16, (row % 2) * 8); x < R; x += 16) {
      const tone = hash(x, row) % 3 === 0 ? 0xc4dcea : 0xb4d0e2;
      fill(x, y, 15, 6, tone); fill(x, y, 15, 1, 0xe0eef6); dot(x + 1, y + 1, WHITE, 0.7); fill(x + 14, y + 1, 1, 5, 0xa4c4d8);
    }
    fill(L, 30, w, 8, 0xe8f0f6);
    for (let x = from(L, 12); x < R; x += 12) { fill(x + 2, 29, 7, 1, 0xe8f0f6); fill(x + 4, 28, 3, 1, 0xe8f0f6); fill(x, 35, 12, 3, 0xd2e0ea); }
    fill(L, 38, w, 2, 0x5a7a90);
  },
  /** Log cabin. Round logs stacked to the floor, chinked between, a knot here and there. */
  logs(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x3a2616);
    for (let y = 1, row = 0; y < 38; y += 5, row++) {
      fill(L, y, w, 4, 0x7a5232); fill(L, y, w, 1, 0x9a6a42); fill(L, y + 3, w, 1, 0x5a3b25); fill(L, y + 4, w, 1, 0x9a8a6a);
      for (let x = from(L, 23, (row * 7) % 23); x < R; x += 23) { dot(x, y + 1, 0x5a3b25); dot(x + 1, y + 2, 0x5a3b25); }
    }
    fill(L, 0, w, 1, 0x2a1d16);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Bare brick. Brick above, a painted band below. */
  brick(L, R) {
    const w = R - L;
    fill(L, 0, w, 30, 0x5a4a44);
    for (let y = 0, row = 0; y < 30; y += 4, row++) for (let x = from(L, 10, (row % 2) * 5); x < R; x += 10) {
      const tone = [0x8a3a2a, 0x9a4432, 0x7a3022, 0x8f4030][hash(x, row) % 4];
      fill(x, y + 1, 9, 3, tone); fill(x, y + 1, 9, 1, shaded(tone, 1.15));
    }
    fill(L, 0, w, 2, 0x3a2e2a);
    fill(L, 30, w, 8, 0x4a5060); fill(L, 30, w, 1, 0x5a6070);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Bamboo. Canes side by side, more laid across below and lashed with rope. */
  bamboo(L, R) {
    const w = R - L;
    fill(L, 0, w, 30, 0x3a3a20);
    for (let x = from(L, 5); x < R; x += 5) {
      const i = Math.floor(x / 5), cane = i % 2 ? 0xa8a050 : 0x9a9248;
      fill(x, 0, 4, 30, cane); fill(x, 0, 1, 30, shaded(cane, 1.15)); fill(x + 3, 0, 1, 30, shaded(cane, 0.8));
      for (let y = 4 + (hash(i, 3) % 7); y < 30; y += 11) { fill(x, y, 4, 1, 0x6a6430); fill(x, y + 1, 4, 1, shaded(cane, 1.2)); }
    }
    fill(L, 30, w, 8, 0x3a3a20);
    for (const y of [30, 33]) { fill(L, y, w, 3, 0x8a8440); fill(L, y, w, 1, 0xb8b05a); }
    for (let x = from(L, 20, 4); x < R; x += 20) { fill(x, 30, 2, 6, 0xc9a06a); dot(x + 2, 31, 0xc9a06a); }
    fill(L, 0, w, 2, 0x2a2a18);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Painted blockwork. Painted blocks with a stripe in the gym's colours, padded below. */
  block(L, R) {
    const w = R - L;
    fill(L, 0, w, 30, 0x80868e);
    for (let y = 0, row = 0; y < 30; y += 6, row++) for (let x = from(L, 16, (row % 2) * 8); x < R; x += 16) { fill(x, y, 15, 5, 0x9aa0a8); fill(x, y, 15, 1, 0xa8aeb6); }
    fill(L, 0, w, 2, 0x6a7078);
    fill(L, 19, w, 3, 0xd4140b); fill(L, 22, w, 1, PALETTE.hazard);
    fill(L, 30, w, 8, 0x2a2c30); for (let x = from(L, 12); x < R; x += 12) fill(x, 30, 1, 8, 0x1d1f25); fill(L, 30, w, 1, 0x3a3d42);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Acoustic foam. Foam tiles, turned this way and that, and an LED strip along the top that cycles the colours. */
  foam(L, R) {
    const w = R - L;
    fill(L, 0, w, 38, 0x16171c);
    for (let x = from(L, 8); x < R; x += 8) for (let y = 4, r = 0; y < 36; y += 8, r++) {
      const across = (Math.floor(x / 8) + r) % 2 === 0;
      for (let k = 0; k < 3; k++) { if (across) fill(x + 1, y + 1 + k * 2, 6, 1, 0x24262e); else fill(x + 1 + k * 2, y + 1, 1, 6, 0x24262e); }
    }
    fill(L, 0, w, 2, 0x0e0f13);
    for (let x = L; x < R; x++) { const c = rainbow((((now * 0.15) + x / 120) % 1 + 1) % 1); dot(x, 2, c); dot(x, 3, c, 0.25); dot(x, 4, c, 0.1); }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Broadcast set. Black studio panels between thin rules; along the rail, lower thirds: the move's blue tag, a white slab. */
  primetime(L, R) {
    const w = R - L;
    fill(L, 0, w, 40, 0x0e0e0e);
    fill(L, 0, w, 2, 0x080808);
    for (let x = from(L, 32); x < R; x += 32) fill(x, 2, 1, 28, 0x222222);
    fill(L, 15, w, 1, 0x1a1a1a);
    fill(L, 30, w, 8, 0x1a1a1a);
    fill(L, 30, w, 1, 0x333333);
    for (let x = from(L, 96, 8); x < R; x += 96) {
      fill(x, 32, 6, 4, 0x3558ce);
      fill(x + 6, 32, 22, 4, 0xf5f5f5);
      fill(x + 8, 33, 12, 1, 0x0e0e0e);
      fill(x + 8, 34, 8, 1, 0x8a8a8a);
    }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Arena seating. Tiers of seats, a crowd in them, a wave going round every so often; ad boards along the rail. */
  bleachers(L, R) {
    const w = R - L;
    fill(L, 0, w, 30, 0x1a1a1a);
    fill(L, 0, w, 2, 0x111111);
    const wave = (now % 45) / 3, crest = L - 10 + wave * (w + 20);
    const shirts = [0x3558ce, 0xf5f5f5, 0x8a8a8a, 0x3558ce, 0x45c285, 0x2e2e2e];
    for (let row = 0; row < 5; row++) {
      const y = 4 + row * 5;
      fill(L, y + 3, w, 2, row % 2 ? 0x2e2e2e : 0x363636);
      for (let x = from(L, 4, (row % 2) * 2); x < R; x += 4) {
        const h = hash(x, row);
        if (h % 5 === 0) continue;
        const up = wave < 1 && Math.abs(x - crest) < 4 ? 1 : 0, skin = SKINS[(h >>> 6) % SKINS.length];
        fill(x, y + 1 - up, 2, 2, shirts[(h >>> 3) % shirts.length]);
        dot(x, y - up, skin);
        if (up) { dot(x - 1, y - 2, skin); dot(x + 2, y - 2, skin); }
      }
    }
    fill(L, 30, w, 8, 0x0e0e0e);
    fill(L, 30, w, 1, 0x333333);
    for (let x = from(L, 64, 4); x < R; x += 64) {
      fill(x, 32, 8, 4, 0x3558ce);
      fill(x + 8, 32, 24, 4, 0xf5f5f5);
      fill(x + 10, 33, 14, 1, 0x0e0e0e);
    }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Padded station panels. White quilted cushions in two rows, a handrail on brackets now and then, cables clipped along the top. */
  stationwall(L, R) {
    const w = R - L;
    fill(L, 0, w, 30, 0xc4c9d2); fill(L, 0, w, 2, 0xa8aeb8);
    // The cushions, each puffed: lit along its top, shaded along its foot, a quilting button in its middle
    for (const [py, ph] of [[3, 12], [16, 13]]) for (let x = from(L, 20, py === 3 ? 0 : 10); x < R; x += 20) {
      fill(x + 1, py, 18, ph, 0xe8ebef); fill(x + 2, py, 16, 1, 0xf7f8fa); fill(x + 1, py + 1, 1, ph - 2, 0xf2f4f6);
      fill(x + 2, py + ph - 1, 16, 1, 0xd2d6dd); fill(x + 18, py + 1, 1, ph - 1, 0xd8dce2);
      dot(x + 10, py + Math.floor(ph / 2), 0xb8bec8); dot(x + 10, py + Math.floor(ph / 2) - 1, 0xffffff, 0.8);
    }
    // Cables clipped along the top, one dropping down to a little junction box every so often
    fill(L, 2, w, 1, 0x3a3f4a); fill(L, 3, w, 1, 0xd4703a, 0.85);
    for (let x = from(L, 32, 6); x < R; x += 32) fill(x, 1, 2, 4, 0x8a919c);
    for (let x = from(L, 96, 50); x < R; x += 96) {
      fill(x, 3, 1, 9, 0x3a3f4a); fill(x - 2, 12, 5, 4, 0x9aa0aa); fill(x - 1, 13, 3, 2, 0xb8bec8);
      dot(x, 13, (Math.floor(now / 7) + Math.floor(x / 96)) % 4 === 0 ? 0x6fd36f : 0x4a7a4a);
    }
    // Handrails to hold on to, on their brackets
    for (let x = from(L, 64, 22); x < R; x += 64) {
      fill(x, 21, 2, 4, 0x6a7280); fill(x + 18, 21, 2, 4, 0x6a7280);
      fill(x - 1, 21, 22, 2, 0x8a919c); fill(x - 1, 21, 22, 1, 0xd8a83a); fill(x - 1, 23, 22, 1, 0x5a6270, 0.6);
    }
    // The kick panel under the rail, its vents
    fill(L, 30, w, 8, 0xb0b6c0); fill(L, 30, w, 1, 0x8a919c); fill(L, 31, w, 1, 0xd2d6dd);
    for (let x = from(L, 40, 14); x < R; x += 40) for (const vy of [33, 35]) fill(x, vy, 8, 1, 0x8a919c);
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Shoji. Pale paper panels in a dark wooden lattice, posts between them, a dark wood skirting below. */
  shoji(L, R) {
    const w = R - L, paper = 0xece4d0, dim = 0xded4bc, wood = 0x4a3626, deep = 0x3a2a1e;
    fill(L, 0, w, 30, paper); fill(L, 0, w, 2, deep);
    for (let y = 2; y < 30; y += 7) fill(L, y + 3, w, 3, dim, 0.45);
    for (let x = from(L, 4); x < R; x += 4) fill(x, 2, 1, 28, wood, 0.55);
    for (let y = 9; y < 30; y += 7) fill(L, y, w, 1, wood, 0.75);
    for (let x = from(L, 32); x < R; x += 32) { fill(x - 1, 0, 3, 30, wood); fill(x - 1, 0, 1, 30, 0x5e4632); }
    fill(L, 2, w, 1, wood); fill(L, 29, w, 1, wood);
    fill(L, 30, w, 8, wood); fill(L, 30, w, 1, 0x5a4230);
    for (let x = from(L, 32); x < R; x += 32) { fill(x + 3, 32, 26, 4, deep); fill(x + 3, 32, 26, 1, 0x2e2018); }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
  /** Plank walls. Rough upright boards nailed to a cross rail; here and there a branch grows in through one, in leaf. */
  plankwall(L, R) {
    const w = R - L, tones = [0xa87a4a, 0x9a6e42, 0xb4864f, 0xa07446];
    fill(L, 0, w, 38, 0x6a4a2c);
    for (let x = from(L, 7); x < R; x += 7) {
      const h = hash(x, 3), tone = tones[h % 4];
      fill(x, 0, 6, 38, tone); fill(x, 0, 1, 38, shaded(tone, 1.12)); fill(x + 5, 0, 1, 38, shaded(tone, 0.85));
      if (h % 5 === 0) { dot(x + 2, 6 + ((h >>> 4) % 18), 0x6a4a2c); dot(x + 3, 7 + ((h >>> 4) % 18), shaded(tone, 0.8)); }
      dot(x + 3, 3, 0x5a5048); dot(x + 3, 34, 0x5a5048);
    }
    fill(L, 0, w, 2, 0x5a3e24);
    fill(L, 30, w, 3, 0x8a6038); fill(L, 30, w, 1, 0xa87a4a); fill(L, 33, w, 1, 0x5a3e24);
    // A branch in through a board, leaves on it, now and then one stirring
    for (let x = from(L, 160, 120); x < R; x += 160) {
      fill(x - 1, 9, 5, 5, 0x3a2616);
      line(x, 11, x + 14, 6, 0x6a4a2e); line(x, 12, x + 14, 7, 0x5a3b25); line(x + 7, 9, x + 11, 13, 0x5a3b25);
      const stir = (now + x * 0.11) % 17 < 0.6 ? 1 : 0;
      for (const [dx, dy] of [[4, 7], [9, 5], [15, 4], [16, 8], [11, 13], [6, 12]]) {
        fill(x + dx, dy - (dx === 15 ? stir : 0), 3, 2, PALETTE.leaf); dot(x + dx + 1, dy + 1, PALETTE.leafDark);
      }
    }
    fill(L, 38, w, 2, PALETTE.baseboard);
  },
};
