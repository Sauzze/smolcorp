/**
 * The office's monitors besides the standard one, from behind, each at
 * the top-left `x`, `y` of where a standard one goes, and where its
 * screen is, if it isn't there: for the screen's light.
 */
const MONITORS = {
  /** Broadcast monitor. Flat and black, a rule round it, a lower third's blue tag and white slab on its bezel. */
  broadcast: {
    draw(x, y) {
      fill(x - 1, y - 1, 15, 11, 0x333333); fill(x, y, 13, 9, 0x0e0e0e); fill(x + 1, y + 1, 11, 6, 0x1a1a1a);
      fill(x + 1, y + 8, 2, 1, 0x4a6be0); fill(x + 3, y + 8, 4, 1, 0xf5f5f5);
      fill(x + 5, y + 10, 3, 1, 0x1a1a1a); fill(x + 3, y + 11, 7, 1, 0x333333);
    },
  },
  /**
   * Mini scoreboard. The monitor's back as a scoreboard: a digit for each side under its colour, flipping now and then as
   * one of them scores, each desk's game in its own time, and a shot clock running down along its foot.
   */
  scoreboard: {
    draw(x, y) {
      fill(x - 1, y - 1, 15, 11, 0x2a2a2e); fill(x, y, 13, 9, 0x0e0e0e);
      fill(x + 1, y + 1, 5, 1, SIDE_TEAL); fill(x + 7, y + 1, 5, 1, SIDE_ORANGE);
      const t = now + x * 0.53 + y * 0.29, game = Math.floor(t / 9), flip = t % 9 < 0.3;
      [(game * 3) % 10, (game * 7 + 4) % 10].forEach((n, side) => {
        const dx = x + 2 + side * 6, scored = flip && (game + side) % 2 === 0;
        GLYPHS[n].forEach((row, r) => [...row].forEach((on, c) => {
          if (on === '1' && !(scored && r < 2)) dot(dx + c, y + 2 + r, 0xffb03a);
        }));
        if (scored) fill(dx, y + 3, 3, 1, 0x5a3a12);
      });
      dot(x + 6, y + 3, 0x5a5a5a); dot(x + 6, y + 5, 0x5a5a5a);
      const left = 1 - (t % 24) / 24;
      fill(x + 1, y + 8, Math.max(1, Math.round(11 * left)), 1, left < 0.2 ? 0xff3a2a : 0xe0503a);
      fill(x + 5, y + 10, 3, 1, 0x1a1a1a); fill(x + 3, y + 11, 7, 1, 0x2a2a2e);
    },
  },
  /**
   * Haunted CRT. An old set in plum-grey, a cobweb in its corner, the static inside glinting green through its vents;
   * now and then a little ghost drifts up out of it, looks round and sinks back in, each desk's in its own time.
   */
  ghostcrt: {
    draw(x, y) {
      const flicker = (now + x * 0.21) % 6 < 0.12 ? 0.45 : 1, fizz = Math.floor(now * 3);
      fill(x - 1, y - 1, 15, 11, 0x8af0b0, 0.06 * flicker);
      fill(x, y, 13, 9, 0x3e3844); fill(x, y, 13, 1, 0x5a5262); fill(x + 12, y, 1, 9, 0x2a2630);
      fill(x + 2, y + 2, 9, 6, 0x322d38); fill(x + 2, y + 2, 9, 1, 0x463f4e);
      for (const r of [4, 6]) for (let k = 0; k < 5; k++) dot(x + 4 + k, y + r, (k * 7 + r + fizz) % 4 ? 0x2c4a3c : 0x7ae0a8, flicker);
      for (const [dx, dy] of [[1, 1], [2, 1], [3, 1], [1, 2], [1, 3], [2, 2]]) dot(x + dx, y + dy, 0xd8d8e0, 0.45);
      fill(x + 4, y + 9, 5, 1, 0x322d38); fill(x + 3, y + 10, 7, 1, 0x2a2630);
      // The ghost, up from behind the set and back down, only above its top
      const t = (now + x * 0.37) % 13;
      if (t >= 3.2) return;
      const rise = Math.sin((t / 3.2) * Math.PI), gy = y + 3 - Math.round(rise * 9), look = t > 1.2 && t < 2 ? 1 : 0;
      within([[x - 2, y - 12, 17, 12]], () => {
        const sheet = 0xf4f1ea;
        fill(x + 5, gy, 3, 1, sheet, 0.9); fill(x + 4, gy + 1, 5, 6, sheet, 0.9);
        dot(x + 5 + look, gy + 2, 0x1a1a1f); dot(x + 7 + look, gy + 2, 0x1a1a1f); dot(x + 6 + look, gy + 4, 0x1a1a1f, 0.7);
        glowAt(x + 6, gy + 3, 4, 4, 0xc8f0d8, 0.08);
      });
    },
  },
  /**
   * Ouija frame. The screen set in a talking board: a sun and a moon in its top corners, its letters in two arcs and
   * GOODBYE along the foot; the planchette on top slides across it every so often, as if someone were asking.
   */
  ouija: {
    draw(x, y) {
      const board = 0xc89a62, rim = 0x7a5232, ink = 0x3a2616;
      fill(x - 1, y - 1, 15, 11, rim); fill(x, y, 13, 9, board); fill(x, y, 13, 1, 0xdcb47c);
      dot(x + 1, y + 1, 0xf2b03a); dot(x + 11, y + 1, 0xeae6dc); dot(x + 12, y + 1, board);
      for (let k = 0; k < 9; k++) {
        dot(x + 2 + k, y + 3 - (k > 1 && k < 7 ? 1 : 0), ink, k % 2 ? 0.55 : 0.85);
        dot(x + 2 + k, y + 5 - (k > 2 && k < 6 ? 1 : 0), ink, k % 2 ? 0.85 : 0.55);
      }
      fill(x + 3, y + 7, 7, 1, ink, 0.6);
      fill(x + 5, y + 10, 3, 1, rim); fill(x + 3, y + 11, 7, 1, ink);
      // The planchette: still at YES, then across to NO and back, now and then
      const t = (now + x * 0.29) % 17, glide = t < 4 ? Math.sin((t / 4) * Math.PI) : 0, px = x + 1 + Math.round(glide * 8);
      fill(px, y - 3, 3, 2, 0xeae0c8); dot(px + 1, y - 4, 0xeae0c8); dot(px + 1, y - 3, 0x5a3b25);
    },
  },
  /** Candy-cane frame. The monitor's back in a frame striped red and white like a candy cane, a green bow on top. */
  candycane: {
    draw(x, y) {
      const red = 0xd42a2a, white = 0xf6f2ea;
      fill(x - 1, y - 1, 15, 11, white);
      for (let i = -1; i <= 13; i++) for (const j of [-1, 9]) if ((i + j + 41) % 4 < 2) dot(x + i, y + j, red);
      for (let j = 0; j <= 8; j++) for (const i of [-1, 13]) if ((i + j + 41) % 4 < 2) dot(x + i, y + j, red);
      fill(x, y, 13, 9, PALETTE.monitor); fill(x + 1, y + 1, 11, 7, PALETTE.monitorPanel);
      fill(x + 5, y + 10, 3, 1, red); fill(x + 5, y + 11, 3, 1, white); fill(x + 3, y + 12, 7, 1, red);
      const bow = 0x2f8a46, knot = 0x1f5a2e;
      fill(x + 3, y - 4, 3, 3, bow); fill(x + 8, y - 4, 3, 3, bow); dot(x + 4, y - 3, knot); dot(x + 9, y - 3, knot);
      fill(x + 6, y - 3, 2, 2, knot); dot(x + 5, y - 1, bow); dot(x + 8, y - 1, bow);
    },
  },
  /** Wrapped like a present. The monitor's back in red paper, a gold ribbon round it, a big bow on top; its tag flutters now and then. */
  giftwrap: {
    draw(x, y) {
      const paper = 0xc8302a, edge = 0x9a2420, gold = 0xf2c84a, shine = 0xfae29a;
      fill(x - 1, y - 1, 15, 11, edge); fill(x, y, 13, 9, paper); fill(x, y, 13, 1, 0xe0504a);
      for (const [dx, dy] of [[2, 2], [10, 2], [3, 7], [10, 7], [8, 5]]) dot(x + dx, y + dy, 0xf6f2ea, 0.75);
      fill(x + 6, y - 1, 2, 11, gold); fill(x - 1, y + 4, 15, 1, gold); dot(x + 6, y + 1, shine); dot(x + 2, y + 4, shine);
      fill(x + 2, y - 4, 4, 3, gold); fill(x + 8, y - 4, 4, 3, gold); dot(x + 3, y - 3, edge); dot(x + 10, y - 3, edge);
      fill(x + 6, y - 3, 2, 2, 0xd6a03a); dot(x + 5, y - 1, gold); dot(x + 8, y - 1, gold);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      // The tag, on a string from the ribbon, lifting in a draught now and then
      const t = (now + x * 0.27) % 12, flap = t < 1.2 ? Math.floor(now * 8) % 2 : 0;
      dot(x + 13, y + 5, 0xf6f2ea, 0.7);
      fill(x + 13 + flap, y + 6 - flap, 3, 3, 0xf4ecd8); dot(x + 14 + flap, y + 7 - flap, 0xc8302a);
    },
  },
  /** Knitted cover. The monitor's back in a ribbed pine cosy with a cream band of berry zigzags; its bobble nods now and then. */
  knitmon: {
    draw(x, y) {
      const pine = 0x2f6a4a, rib = 0x245a3c, cream = 0xf2ead8, berry = 0xb8303a;
      fill(x - 1, y - 1, 15, 11, pine);
      for (let i = 1; i < 15; i += 2) fill(x - 1 + i, y - 1, 1, 11, rib);
      fill(x - 1, y + 2, 15, 3, cream);
      for (let i = 0; i < 15; i++) dot(x - 1 + i, y + 3 + [0, -1, 0, 1][i % 4], berry);
      fill(x - 1, y + 8, 15, 2, rib); fill(x - 1, y - 1, 15, 1, 0x3f7a5a);
      dot(x - 1, y - 1, rib); dot(x + 13, y - 1, rib);
      fill(x + 5, y + 10, 3, 1, PALETTE.monitorStand); fill(x + 3, y + 11, 7, 1, PALETTE.monitorStand);
      // The bobble, on a knot of yarn
      const t = (now + x * 0.21) % 14, nod = t < 1.2 ? (Math.floor(now * 6) % 2 ? 1 : -1) : 0;
      dot(x + 6, y - 2, cream);
      fill(x + 5 + nod, y - 5, 3, 3, cream); dot(x + 5 + nod, y - 5, WHITE); dot(x + 7 + nod, y - 3, 0xc8bca0);
    },
  },
  /** Tombstone monitor. A headstone for a back, rounded at the top, RIP cut in it, moss at its foot; fog drifts by now and then. */
  tombstone: {
    draw(x, y) {
      const stone = 0x8a8f9c, light = 0xa8adb6, dark = 0x6a6f7a;
      fill(x + 3, y - 2, 7, 1, stone); fill(x + 1, y - 1, 11, 1, stone); fill(x, y, 13, 10, stone);
      fill(x + 3, y - 2, 7, 1, light); fill(x + 1, y - 1, 2, 1, light); fill(x, y, 1, 10, light);
      fill(x + 12, y, 1, 10, dark); dot(x + 11, y - 1, dark);
      pixelText('RIP', x + 1, y + 1, 0x55585e, false);
      fill(x - 1, y + 10, 15, 1, 0x5a5f6c);
      for (const dx of [0, 1, 4, 10, 12]) dot(x + dx, y + 9, 0x4f7a3a);
      dot(x + 2, y + 8, 0x4f7a3a);
      const t = (now + x * 0.17) % 14;
      if (t < 5) {
        const drift = Math.round(-6 + (t / 5) * 20), a = 0.28 * Math.sin((t / 5) * Math.PI);
        fill(x + drift, y + 7, 9, 1, 0xdfe6ea, a); fill(x + drift + 2, y + 6, 5, 1, 0xdfe6ea, a * 0.7);
      }
    },
  },
  /**
   * Heart frame. A pink back studded with little hearts, a red bow tied on top; now and then one of the hearts
   * twinkles, each desk's in its own time.
   */
  heartframe: {
    draw(x, y) {
      const pink = 0xf2a6bc, rim = 0xd47e98, light = 0xf8c8d6, red = 0xd8344a;
      fill(x - 1, y - 1, 15, 11, rim); fill(x, y, 13, 9, pink); fill(x, y, 13, 1, light); fill(x + 12, y, 1, 9, 0xe08ea6);
      [[x + 1, y + 2, red], [x + 5, y + 4, WHITE], [x + 9, y + 2, red], [x + 1, y + 6, 0xf6e6ea], [x + 9, y + 6, 0xf6e6ea]]
        .forEach(([hx, hy, c]) => tinyHeart(hx, hy, c));
      const t = (now + x * 0.23) % 9;
      if (t < 0.5) { dot(x + 6, y + 4, WHITE); dot(x + 6, y + 3, WHITE, 0.6); dot(x + 5, y + 5, WHITE, 0.4); dot(x + 7, y + 5, WHITE, 0.4); }
      // The bow on top, its knot in the middle and its tails hanging over the edge
      fill(x + 3, y - 3, 3, 2, red); fill(x + 8, y - 3, 3, 2, red); fill(x + 6, y - 2, 2, 2, 0xa82234);
      dot(x + 4, y - 3, 0xf06a80); dot(x + 9, y - 3, 0xf06a80);
      dot(x + 5, y, red); dot(x + 4, y + 1, red); dot(x + 8, y, red); dot(x + 9, y + 1, red);
      fill(x + 5, y + 10, 3, 1, rim); fill(x + 3, y + 11, 7, 1, 0xb86a82);
    },
  },
  /**
   * Heart locket. A gold heart for a back, an engraved heart in it and its catch at the top, on a stand that's a short
   * length of chain; now and then the catch catches the light.
   */
  locket: {
    draw(x, y) {
      const gold = 0xd6b25e, shade = 0xa8863a, light = 0xf2d38a, line = 0x9a7a32;
      const shape = heartShape(15, 12);
      shape.rows.forEach((runs, r) => runs.forEach(([a, b]) => {
        fill(x + 6 + a, y - 2 + r, b - a + 1, 1, gold);
        if (b - a > 4) fill(x + 6 + b - 2, y - 2 + r, 2, 1, shade);
        if (r > 1 && r < 7) dot(x + 6 + a + 1, y - 2 + r, light);
      }));
      for (const [dx, dy] of shape.edge) dot(x + 6 + dx, y - 2 + dy, line);
      for (const [dx, dy] of heartShape(7, 6).edge) dot(x + 6 + dx, y + 1 + dy, line, 0.7);
      // The catch, at the dip of its top
      fill(x + 5, y - 2, 3, 2, light); dot(x + 6, y - 2, WHITE, 0.7);
      const t = (now + x * 0.19) % 11;
      if (t < 0.7) {
        const a = Math.sin((t / 0.7) * Math.PI);
        dot(x + 6, y - 3, WHITE, a); dot(x + 6, y - 5, WHITE, a * 0.6); dot(x + 4, y - 3, WHITE, a * 0.5); dot(x + 8, y - 3, WHITE, a * 0.5);
      }
      // A short chain for a stand, and its foot
      for (let k = 0; k < 2; k++) { dot(x + 5 + (k % 2) * 2, y + 10, k ? shade : gold); }
      dot(x + 6, y + 10, line); dot(x + 6, y + 9, gold);
      fill(x + 3, y + 11, 7, 1, shade); fill(x + 4, y + 11, 5, 1, gold);
    },
  },
  /**
   * Cat nap. A monitor from behind with a ginger cat asleep along its top, warm from the screen: its tail hangs down
   * and flicks now and then, an ear twitches, and it breathes slow; each desk's cat in its own time.
   */
  catnap: {
    draw(x, y) {
      fill(x, y, 13, 9, PALETTE.monitor);
      fill(x + 1, y + 1, 11, 7, PALETTE.monitorPanel);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      const fur = 0xe0a050, stripe = 0xb87430, belly = 0xf2d0a0, dark = 0x5a3a24, k = x * 0.37;
      const breathe = Math.sin(now * 1.4 + k) > 0.6 ? 1 : 0;
      // The tail down the monitor's side, curling up and back now and then
      const t = (now + k * 3) % 9, flick = t < 0.9 ? (Math.floor(t * 6) % 2 ? 2 : 1) : 0;
      fill(x - 1, y, 1, 6 - flick * 2, fur); dot(x - 1, y + 5 - flick * 2, stripe);
      if (flick) { dot(x - 2, y + 4 - flick * 2, fur); dot(x - 2, y + 5 - flick * 2, stripe); }
      // Curled along the top: back, haunch, and its stripes
      fill(x, y - 3 - breathe, 10, 3 + breathe, fur); fill(x + 1, y - 4 - breathe, 7, 1, fur);
      for (const sx of [2, 5, 8]) fill(x + sx, y - 3 - breathe, 1, 2, stripe);
      fill(x, y - 1, 10, 1, belly);
      // Its head on its paws at the right, eyes shut
      fill(x + 9, y - 4, 4, 4, fur); fill(x + 10, y - 1, 3, 1, belly);
      const twitch = (now + k * 5) % 11 < 0.25;
      dot(x + 9, y - 5 - (twitch ? 1 : 0), fur); dot(x + 12, y - 5, fur);
      dot(x + 9, y - 4, 0xe88a7a); dot(x + 12, y - 4, 0xe88a7a);
      fill(x + 10, y - 3, 1, 1, dark); fill(x + 12, y - 3, 1, 1, dark); dot(x + 11, y - 2, 0xd07060);
      // A little z, drifting off it now and then
      const z = (now * 0.4 + k) % 1;
      if ((now + k) % 14 < 5) {
        const zx = x + 13 + Math.round(z * 3), zy = y - 6 - Math.round(z * 5);
        fill(zx, zy, 2, 1, WHITE, 0.7 * (1 - z)); dot(zx + 1, zy + 1, WHITE, 0.7 * (1 - z)); fill(zx, zy + 2, 2, 1, WHITE, 0.7 * (1 - z));
      }
    },
  },
  /** CRT monitor. A deep beige case, vents on its back. */
  crt: {
    draw(x, y) {
      fill(x, y, 13, 9, 0xcfc8b4); fill(x, y, 13, 1, 0xe2dccb); fill(x + 12, y, 1, 9, 0xa8a08a);
      fill(x + 2, y + 2, 9, 6, 0xb8b09a); fill(x + 2, y + 2, 9, 1, 0xc8c0aa); for (const r of [4, 6]) fill(x + 4, y + r, 5, 1, 0x8a846f);
      fill(x + 4, y + 9, 5, 1, 0xb8b09a); fill(x + 3, y + 10, 7, 1, 0xa8a08a);
    },
  },
  /** Hologram screen. A pane of light over a projector puck; a scan line runs down it and it flickers now and then. */
  holo: {
    draw(x, y) {
      const cyan = 0x4ad4e8, flicker = (now % 6) < 0.15 ? 0.4 : 1;
      fill(x + 3, y + 9, 7, 2, 0x3a3f4a); fill(x + 4, y + 9, 5, 1, cyan);
      fill(x, y, 13, 9, cyan, 0.14 * flicker); fill(x, y, 13, 1, cyan, 0.6 * flicker); fill(x, y + 8, 13, 1, cyan, 0.4 * flicker); fill(x, y, 1, 9, cyan, 0.5 * flicker); fill(x + 12, y, 1, 9, cyan, 0.5 * flicker);
      fill(x + 1, y + 1 + (Math.floor(now * 6) % 7), 11, 1, cyan, 0.35 * flicker);
    },
  },
  /** Ship's monitor. A wooden case with brass corners and rivets, on a brass post. */
  brass: {
    draw(x, y) {
      fill(x, y, 13, 9, 0x6b4a2e); fill(x + 1, y + 1, 11, 7, 0x5a3b25);
      for (const [dx, dy] of [[0, 0], [11, 0], [0, 7], [11, 7]]) fill(x + dx, y + dy, 2, 2, PALETTE.brass);
      for (let k = 3; k < 11; k += 3) { dot(x + k, y, 0xa8894a); dot(x + k, y + 8, 0xa8894a); }
      fill(x + 6, y + 9, 1, 1, PALETTE.brass); fill(x + 4, y + 10, 5, 1, 0x9a7a3a);
    },
  },
  /** Crystal ball. Purple mist swirls inside, on a claw stand. Its light is a ring round it. */
  crystal: {
    screen: (x, y) => ({ x: x + 2, y, w: 9, h: 9, round: true }),
    draw(x, y) {
      const cx = x + 6, cy = y + 4;
      fill(cx - 5, cy - 5, 11, 11, 0x9a6ad0, 0.08);
      disc(cx, cy, 4.3, 0x4a2a7a); disc(cx, cy, 3.3, 0x6a3fb5);
      for (let k = 0; k < 3; k++) { const a = now * 1.4 + (k * Math.PI * 2) / 3; dot(cx + Math.round(Math.cos(a) * 2), cy + Math.round(Math.sin(a) * 1.5), 0xd0a0ff); }
      dot(cx - 2, cy - 2, WHITE, 0.8); dot(cx - 1, cy - 3, WHITE, 0.4);
      fill(x + 3, y + 8, 7, 2, 0x3a3a40); dot(x + 3, y + 7, 0x3a3a40); dot(x + 9, y + 7, 0x3a3a40); fill(x + 2, y + 10, 9, 1, 0x2a2a2e);
    },
  },
  /** Gilt-framed screen. Gold frame, red velvet back, a crest. */
  gilt: {
    draw(x, y) {
      fill(x, y, 13, 9, PALETTE.brass); fill(x + 12, y, 1, 9, 0x9a7a3a); fill(x, y + 8, 13, 1, 0x9a7a3a); fill(x, y, 13, 1, 0xf2d38a);
      fill(x + 1, y + 1, 11, 7, 0x5a1e2a); fill(x + 5, y + 3, 3, 3, PALETTE.brass); dot(x + 6, y + 6, PALETTE.brass); dot(x + 6, y + 4, 0xd4140b);
      fill(x + 6, y + 9, 1, 1, PALETTE.brass); fill(x + 4, y + 10, 5, 1, PALETTE.brass); dot(x + 3, y + 10, 0x9a7a3a); dot(x + 9, y + 10, 0x9a7a3a);
    },
  },
  /** Frozen monitor. Frosted over, icicles hanging; a drop falls now and then. */
  ice: {
    draw(x, y) {
      fill(x, y, 13, 9, 0x9ec4d8); fill(x + 1, y + 1, 11, 7, 0x7aa4bc); fill(x, y, 13, 1, 0xdcebf4);
      for (const [dx, dy] of [[1, 1], [2, 2], [10, 1], [11, 2], [1, 6], [11, 6], [6, 3]]) dot(x + dx, y + dy, WHITE, 0.7);
      for (const [dx, len] of [[1, 1], [3, 2], [9, 1], [11, 2]]) fill(x + dx, y + 9, 1, len, 0xcfe3ef, 0.85);
      const drop = (now % 3) / 3; if (drop < 0.4) dot(x + 3, y + 11 + Math.round(drop * 5), 0xcfe3ef, 0.8);
      fill(x + 5, y + 9, 3, 2, 0x8ab0c4);
    },
  },
  /** Curved ultrawide. Wider than the desk's usual, its back lit round the colour wheel. */
  ultrawide: {
    screen: (x, y) => ({ x: x - 3, y: y + 1, w: 19, h: 8 }),
    draw(x, y) {
      fill(x - 2, y + 1, 17, 1, PALETTE.monitor); fill(x - 3, y + 2, 19, 7, PALETTE.monitor); fill(x - 2, y + 2, 17, 6, PALETTE.monitorPanel);
      for (let k = 0; k < 15; k++) dot(x - 1 + k, y + 5, rainbow((now * 0.25 + k / 15) % 1));
      fill(x + 5, y + 9, 3, 1, PALETTE.monitorStand); fill(x + 2, y + 10, 9, 1, PALETTE.monitorStand);
    },
  },
  /** Two monitors. Side by side on one arm. */
  dual: {
    draw(x, y) {
      for (const mx of [x - 5, x + 7]) { fill(mx, y + 1, 11, 8, PALETTE.monitor); fill(mx + 1, y + 2, 9, 6, PALETTE.monitorPanel); }
      fill(x + 6, y + 4, 1, 6, PALETTE.monitorStand); fill(x + 4, y + 10, 5, 1, PALETTE.monitorStand);
    },
  },
  /** Rugged laptop. Rubber corners and stickers on its lid. */
  laptop: {
    screen: (x, y) => ({ x, y: y + 1, w: 13, h: 8 }),
    draw(x, y) {
      fill(x, y + 1, 13, 8, 0x3a3f36); fill(x + 1, y + 2, 11, 6, 0x4a5240);
      for (const [dx, dy] of [[0, 1], [11, 1], [0, 7], [11, 7]]) fill(x + dx, y + dy, 2, 2, 0xe0703a);
      dot(x + 4, y + 3, 0xeae6dc); fill(x + 3, y + 4, 3, 1, 0xeae6dc); disc(x + 8, y + 5, 1, 0xf2d35b); fill(x + 3, y + 6, 2, 1, 0x4ad4e8);
      fill(x - 1, y + 9, 15, 1, PALETTE.keys);
    },
  },
  /** Bamboo-framed screen. Framed in cane, a vine curling round it, its leaves stirring. */
  bamboo: {
    draw(x, y) {
      fill(x - 1, y - 1, 15, 11, 0xa8a050); fill(x - 1, y - 1, 15, 1, 0xc8c070); for (const [dx, dy] of [[-1, 4], [13, 4], [6, -1], [6, 9]]) dot(x + dx, y + dy, 0x6a6430);
      fill(x, y, 13, 9, PALETTE.monitor); fill(x + 1, y + 1, 11, 7, PALETTE.monitorPanel);
      const sway = Math.round(Math.sin(now * 1.2) * 0.6);
      fill(x - 2, y - 1, 3, 1, PALETTE.leaf); dot(x - 1, y - 2, PALETTE.leafDark); dot(x + 13, y + 2 + sway, PALETTE.leaf); fill(x + 13, y + 5, 2, 1, PALETTE.leafDark); dot(x + 14 + sway, y + 7, PALETTE.leaf);
      fill(x + 5, y + 10, 3, 1, 0x8a8440);
    },
  },
  /** Tablet on an arm. On a gooseneck, a towel over its corner. */
  tablet: {
    screen: (x, y) => ({ x: x + 1, y: y + 1, w: 11, h: 7 }),
    draw(x, y) {
      for (const [dx, dy] of [[12, 10], [13, 9], [13, 8], [12, 7], [11, 6]]) dot(x + dx, y + dy, 0x5a5f6c);
      fill(x + 11, y + 10, 3, 1, 0x3a3d47);
      fill(x + 1, y + 1, 11, 7, 0x1d1f25); fill(x + 2, y + 2, 9, 5, 0x2a2d35);
      fill(x + 8, y, 5, 2, 0xe8e4da); fill(x + 11, y + 2, 2, 3, 0xe8e4da); dot(x + 11, y + 2, 0xc8c4ba);
    },
  },
  /** Portrait monitor. Turned on its side for long code. */
  portrait: {
    screen: (x, y) => ({ x: x + 2, y: y - 4, w: 9, h: 13 }),
    draw(x, y) {
      fill(x + 2, y - 4, 9, 13, PALETTE.monitor); fill(x + 3, y - 3, 7, 11, PALETTE.monitorPanel); fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /**
   * Red lantern. The monitor's back made a round red paper lantern, gold caps top and bottom, glowing a little at
   * night; the gold tassel hanging under it sways every so often, each desk's in its own time.
   */
  lanternmon: {
    draw(x, y) {
      const red = 0xc8302e, dark = 0x9a2226, light = 0xe85a44, gold = 0xe8b84a, goldDark = 0xb8862e;
      const t = (now + x * 0.33) % 9, sway = t < 1.8 ? Math.round(Math.sin(t * 7) * (1 - t / 1.8) * 1.4) : 0;
      glowAt(x + 6, y + 4, 9, 7, 0xff7a4a, isNight ? 0.12 * candlelight(x) : 0.04);
      fill(x + 1, y, 11, 1, red); fill(x, y + 1, 13, 7, red); fill(x + 1, y + 8, 11, 1, red);
      for (const rx of [3, 6, 9]) fill(x + rx, y + 1, 1, 7, dark);
      fill(x, y + 2, 1, 5, dark); fill(x + 12, y + 2, 1, 5, dark); fill(x + 1, y + 2, 1, 4, light);
      fill(x + 3, y - 1, 7, 1, gold); fill(x + 4, y - 2, 5, 1, goldDark);
      fill(x + 4, y + 9, 5, 1, gold); fill(x + 5, y + 10, 3, 1, goldDark);
      dot(x + 6 + sway, y + 11, gold); fill(x + 5 + sway, y + 12, 3, 1, gold); dot(x + 6 + sway, y + 12, goldDark);
    },
  },
  /** A monitor back overgrown with moss along its top and a fern frond at one corner; now and then a tiny mushroom pops up and shrinks back, and a dewdrop glints. */
  mossframe: {
    draw(x, y) {
      const shell = PALETTE.monitor, panel = PALETTE.monitorPanel, stand = PALETTE.monitorStand;
      const moss = 0x5a9a48, mossLight = 0x82c060, mossDark = 0x3b6e34;
      // The monitor's back, its panel and its stand
      fill(x, y, 13, 9, shell); fill(x + 1, y + 1, 11, 7, panel); fill(x, y, 13, 1, shaded(shell, 1.3)); fill(x + 12, y, 1, 9, shaded(shell, 0.8));
      fill(x + 5, y + 9, 3, 2, stand);
      // The moss: a thick pelt along the top edge, creeping down the sides in patches
      fill(x - 1, y - 1, 15, 2, moss); fill(x, y - 2, 4, 1, moss); fill(x + 6, y - 2, 5, 1, moss); fill(x + 2, y - 3, 2, 1, mossLight);
      fill(x - 1, y + 1, 2, 3, moss); fill(x + 12, y + 1, 2, 2, moss); fill(x + 1, y + 1, 3, 1, mossDark); fill(x + 8, y + 1, 3, 1, mossDark);
      for (const [dx, dy] of [[1, -2], [5, -2], [9, -3], [12, -1], [3, 1], [-1, 2]]) dot(x + dx, y + dy, mossLight);
      dot(x + 6, y + 2, mossDark); dot(x + 10, y + 2, moss);
      // A fern frond curling up from the top right corner
      line(x + 12, y - 2, x + 13, y - 6, mossDark); line(x + 13, y - 6, x + 15, y - 8, mossDark);
      for (const [dx, dy] of [[11, -4], [14, -4], [12, -5], [14, -6], [13, -7], [16, -8], [15, -9]]) dot(x + dx, y + dy, moss);
      dot(x + 14, y - 7, mossLight); dot(x + 12, y - 3, mossLight);
      // A tiny mushroom pops up, stays a moment, then shrinks back
      const t = (now + x * 0.41) % 26, grow = t < 1 ? 1 : t < 4.5 ? 2 : t < 5.5 ? 1 : 0;
      if (grow) {
        fill(x + 7, y - 3 - grow, 1, grow + 1, 0xece2cc);
        fill(x + 6, y - 4 - grow, 3, 1, 0xd8402a); dot(x + 7, y - 5 - grow + 1, WHITE, 0.9);
        if (grow > 1) dot(x + 6, y - 4 - grow, 0xf08070);
      }
      // A dewdrop on the moss that glints now and then
      const g = (now + x * 0.2) % 7;
      dot(x + 3, y - 1, 0xcfeaf4, 0.85);
      if (g < 0.5) dot(x + 3, y - 2, WHITE, 0.9);
    },
  },
  /** A monitor back with a tiny satellite dish clamped to its top: every minute the dish turns to a new heading, and its feed horn blinks green as it locks on. */
  satdish: {
    draw(x, y) {
      const shell = PALETTE.monitor, panel = PALETTE.monitorPanel, stand = PALETTE.monitorStand;
      const dish = 0xd8dce2, dishLight = 0xf2f5fa, dishDark = 0xa8adb6, steel = 0x6e727c, steelDark = 0x4a4e58;
      // The monitor's back, its panel and its stand
      fill(x, y, 13, 9, shell); fill(x + 1, y + 1, 11, 7, panel); fill(x, y, 13, 1, shaded(shell, 1.3)); fill(x + 12, y, 1, 9, shaded(shell, 0.8));
      fill(x + 5, y + 9, 3, 2, stand);
      // The clamp over the top edge, a cable down the back, and the short mast
      fill(x + 8, y - 1, 3, 2, steelDark); fill(x + 8, y - 1, 3, 1, steel); fill(x + 9, y + 1, 1, 4, 0x1e2129);
      const mx = x + 9;
      fill(mx, y - 5, 1, 4, steel);
      // The heading: left, up, right, up, one a minute (each monitor its own), slewing through 'up' for a second
      const t = (now + x * 1.7) % 240, at = Math.floor(t / 60), into = t % 60;
      let heading = [-1, 0, 1, 0][at];
      if (into < 1 && heading !== 0) heading = 0;
      // The dish, seen from behind: a round pale bowl with a dark hub, or turned side-on with its feed arm out to the horn
      const cx = mx, cy = y - 7;
      let horn;
      if (heading === 0) {
        for (const [dy, half] of [[-2, 2], [-1, 3], [0, 3], [1, 3], [2, 2]]) fill(cx - half, cy + dy, half * 2 + 1, 1, dish);
        fill(cx - 2, cy - 2, 5, 1, dishLight); fill(cx - 3, cy - 1, 1, 2, dishLight);
        fill(cx - 2, cy + 2, 5, 1, dishDark); fill(cx + 3, cy, 1, 2, dishDark);
        dot(cx, cy, steelDark); horn = [cx, cy - 3];
      } else {
        const s = heading;
        for (const [dy, half] of [[-2, 1], [-1, 2], [0, 2], [1, 2], [2, 1]]) fill(cx - half, cy + dy, half * 2 + 1, 1, dish);
        fill(cx - 2 * s, cy - 1, 1, 3, dishDark); dot(cx - s, cy + 2, dishDark); dot(cx - s, cy - 2, dishDark);
        fill(cx + 2 * s, cy - 1, 1, 2, dishLight); dot(cx, cy - 2, dishLight); dot(cx + s, cy - 2, dishLight);
        dot(cx, cy, steelDark); line(cx + s, cy - 1, cx + 3 * s, cy - 3, steel); horn = [cx + 4 * s, cy - 4];
      }
      // The feed horn: blinking green for 3 s once it has locked on, then a dim steady green
      const locking = into >= 1 && into < 4, blink = locking && Math.floor(now * 4) % 2 === 0;
      dot(horn[0], horn[1], blink ? 0x6af07a : locking ? steelDark : 0x3a7a46);
      if (isNight && blink) glowLate(() => { glowAt(horn[0], horn[1], 1.5, 1.5, 0x6af07a, 0.2); dot(horn[0], horn[1], 0xb8ffc0); });
    },
  },
  /** The monitor propped up on a stack of three chunky hardbacks (red, green, navy, the middle one stuck out), a ribbon bookmark hanging from it: every 35 s the ribbon swings once. */
  bookstack: {
    screen: (x, y) => ({ x: x + 1, y: y - 4, w: 11, h: 7 }),
    draw(x, y) {
      const red = 0xb83a30, green = 0x3f7a4c, navy = 0x2f4678, page = 0xe8e0c8, ribbon = 0xf2d35b;
      const top = y - 5; // the body is lifted five pixels off the desk
      fill(x, top, 13, 9, PALETTE.monitor); fill(x + 1, top + 1, 11, 7, PALETTE.monitorPanel);
      fill(x + 5, top + 9, 3, 1, PALETTE.monitorStand);
      // The books, from the top: red, green (pushed out a little), navy
      fill(x + 1, y + 5, 11, 1, SHADOW, 0.5);
      fill(x + 1, y + 4, 11, 2, red); fill(x + 1, y + 4, 11, 1, shaded(red, 1.2)); fill(x + 1, y + 5, 11, 1, shaded(red, 0.85));
      dot(x + 3, y + 4, 0xf2d35b); dot(x + 4, y + 4, 0xf2d35b, 0.6);
      fill(x + 2, y + 6, 12, 2, green); fill(x + 2, y + 6, 12, 1, shaded(green, 1.2)); fill(x + 2, y + 7, 12, 1, shaded(green, 0.8));
      fill(x + 13, y + 6, 1, 2, page); dot(x + 4, y + 6, 0xe8c878); dot(x + 8, y + 6, 0xe8c878);
      fill(x, y + 8, 11, 3, navy); fill(x, y + 8, 11, 1, shaded(navy, 1.25)); fill(x, y + 10, 11, 1, shaded(navy, 0.75));
      fill(x + 10, y + 8, 1, 3, page); dot(x + 2, y + 9, 0xe8c878); dot(x + 3, y + 9, 0xe8c878, 0.6);
      fill(x + 1, y + 11, 13, 1, SHADOW);
      // The ribbon hanging from the middle book, swinging once every 35 s
      const ph = (now + x * 0.37) % 35, sw = ph < 1.5 ? [1, 2, 1, 0, -1, 0][Math.floor(ph * 4)] || 0 : 0;
      fill(x + 9 + (sw > 0 ? 1 : 0), y + 8, 1, 2, ribbon); dot(x + 9 + sw, y + 10, ribbon); dot(x + 9 + sw + (sw > 0 ? 0 : 0), y + 10, shaded(ribbon, 0.8));
    },
  },
  /** A monitor in a walnut-veneer 1970s cabinet seen from behind: rounded top, a vent grille, an amber pilot lamp that blinks briefly every 20 s and glows at night. */
  woodcase: {
    draw(x, y) {
      const walnut = 0x7a4a2a, walnutLight = 0x9a6438, walnutDark = 0x4e2e1a, grille = 0x2a1a10;
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      // The cabinet with its rounded top corners
      fill(x, y + 1, 13, 8, walnut); fill(x + 1, y, 11, 1, walnut);
      fill(x + 1, y, 11, 1, walnutLight); fill(x, y + 1, 1, 8, walnutLight); fill(x + 12, y + 1, 1, 8, walnutDark); fill(x, y + 8, 13, 1, walnutDark);
      dot(x, y + 1, walnut); dot(x + 12, y + 1, walnutDark);
      // The vent grille: slats across the upper back
      for (let i = 0; i < 3; i++) fill(x + 2, y + 2 + i * 2, 7, 1, grille);
      fill(x + 3, y + 3, 5, 1, walnutDark, 0.6);
      // The pilot lamp: dim amber, bright for a moment every 20 s
      const t = (now + x * 0.31) % 20, flash = t < 0.8;
      dot(x + 10, y + 6, flash ? 0xffc040 : 0xa86a1c);
      if (isNight) glowLate(() => { glowAt(x + 10, y + 6, 3, 3, 0xffa830, flash ? 0.25 : 0.12); dot(x + 10, y + 6, flash ? 0xfff0b0 : 0xffb848); });
    },
  },
  /** A bird's nest on top of the monitor: twigs and two pale-blue eggs; every 45 s a fluffy grey chick pops up between them with its yellow beak open for 2 s, then ducks back down. */
  nestmonitor: {
    draw(x, y) {
      const twig = 0x8a6a42, twigLight = 0xb08a58, twigDark = 0x5a4228, egg = 0xa8d8e8, eggLight = 0xdff0f6, fluff = 0xb8bcc4, fluffLight = 0xdadde2, beak = 0xf2c030;
      // The back of the monitor, with its stand
      fill(x, y, 13, 9, PALETTE.monitor); fill(x + 1, y + 1, 11, 7, PALETTE.monitorPanel); fill(x, y, 13, 1, 0x333845); fill(x + 12, y, 1, 9, 0x1a1d26);
      dot(x + 6, y + 4, 0x3d4250);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand); fill(x + 3, y + 11, 7, 1, PALETTE.monitorStand);
      // The back of the nest and the eggs resting in its cup
      fill(x + 1, y - 3, 11, 3, twigDark); fill(x + 2, y - 4, 9, 1, twig);
      fill(x + 3, y - 5, 3, 2, egg); dot(x + 3, y - 5, eggLight); dot(x + 4, y - 5, eggLight, 0.8);
      fill(x + 8, y - 5, 3, 2, egg); dot(x + 8, y - 5, eggLight); dot(x + 9, y - 5, eggLight, 0.8); dot(x + 10, y - 4, 0x7ab4c8);
      // The chick: pops up between the eggs every 45 s for about 2 s
      const t = (now + x * 0.31) % 45, rise = t < 0.3 ? 1 : t < 0.6 ? 2 : t < 2.0 ? 3 : t < 2.3 ? 2 : t < 2.6 ? 1 : 0;
      if (rise > 0) {
        const top = y - 3 - rise;
        fill(x + 5, top + 1, 4, rise + 1, fluff); fill(x + 6, top, 2, 1, fluffLight); dot(x + 5, top + 1, fluffLight);
        if (rise === 3) {
          dot(x + 6, top + 2, PALETTE.eye); dot(x + 8, top + 2, PALETTE.eye);
          const open = Math.floor(now * 4) % 2 === 0;
          fill(x + 6, top + 3, 2, 1, beak); if (open) fill(x + 6, top + 4, 2, 1, 0xd8402a, 0.9);
        }
      }
      // The front rim of the nest, twigs criss-crossing
      fill(x, y - 2, 13, 2, twig); fill(x + 1, y - 1, 11, 1, twigDark, 0.7);
      for (let i = 0; i < 6; i++) { dot(x + 1 + i * 2, y - 2, i % 2 ? twigLight : twigDark); dot(x + 2 + i * 2, y - 1, i % 2 ? twigDark : twigLight); }
      dot(x - 1, y - 3, twig); dot(x - 1, y - 1, twigDark); dot(x + 13, y - 3, twig); dot(x + 13, y - 2, twigLight); dot(x + 12, y - 4, twig); dot(x + 1, y - 4, twigLight);
    },
  },
  /**
   * A Claude Code terminal. A charcoal monitor showing a terminal: the spark spinning by a "Thinking…" line for a few
   * seconds every 20, then a reply, and the prompt with its cursor blinking; it glows at night.
   */
  claudeterm: {
    draw(x, y) {
      const caseC = 0x3a3631, caseLight = 0x57514a, caseDark = 0x26231f, screen = 0x1c1a18;
      const spark = 0xe8825e, type = 0xf2ebdc, dim = 0x8a8278;
      fill(x - 1, y - 1, 15, 11, caseC); fill(x - 1, y - 1, 15, 1, caseLight); fill(x - 1, y, 1, 10, caseLight);
      fill(x + 13, y, 1, 10, caseDark); fill(x, y + 9, 14, 1, caseDark);
      fill(x, y, 13, 9, screen);
      fill(x + 5, y + 10, 3, 1, caseDark); fill(x + 3, y + 11, 7, 1, caseC);
      const t = (now + x * 0.37) % 20, thinking = t < 5;
      const lines = () => {
        // The spark, turning through its shapes while it thinks, and "Thinking…" with its dots
        const turn = thinking ? Math.floor(now * 5) % 3 : 2;
        dot(x + 2, y + 2, spark);
        if (turn !== 1) { dot(x + 1, y + 2, spark); dot(x + 3, y + 2, spark); dot(x + 2, y + 1, spark); dot(x + 2, y + 3, spark); }
        if (turn !== 2) { dot(x + 1, y + 1, spark, 0.75); dot(x + 3, y + 1, spark, 0.75); dot(x + 1, y + 3, spark, 0.75); dot(x + 3, y + 3, spark, 0.75); }
        if (thinking) {
          fill(x + 5, y + 2, 4, 1, spark, 0.8);
          for (let k = 0; k < Math.floor(t * 2) % 4; k++) dot(x + 9 + k, y + 2, spark, 0.8);
        } else {
          fill(x + 5, y + 2, 6, 1, type, 0.85); fill(x + 5, y + 4, 4, 1, dim);
        }
        // The prompt, and its cursor
        dot(x + 1, y + 5, spark); dot(x + 2, y + 6, spark); dot(x + 1, y + 7, spark);
        if (Math.floor(now * 1.6 + x) % 2 === 0) fill(x + 4, y + 5, 1, 3, type);
      };
      lines();
      if (isNight) glowLate(lines);
    },
  },
  /**
   * A spark screensaver. A cream monitor asleep, the Claude spark drifting across its dark screen a step every half
   * second, bouncing off its sides; it lights the screen at night.
   */
  sparksaver: {
    draw(x, y) {
      const caseC = 0xe6dccb, caseLight = 0xf6efe2, caseDark = 0xbfb39e, screen = 0x221f1c, spark = 0xd97757;
      fill(x - 1, y - 1, 15, 11, caseC); fill(x - 1, y - 1, 15, 1, caseLight); fill(x - 1, y, 1, 10, caseLight);
      fill(x + 13, y, 1, 10, caseDark); fill(x, y + 9, 14, 1, caseDark);
      fill(x + 1, y + 1, 11, 7, screen);
      dot(x + 11, y + 9, 0x7ac080);   // the power light
      fill(x + 5, y + 10, 3, 1, caseDark); fill(x + 3, y + 11, 7, 1, caseC);
      // The spark bounces round the screen, a step every half second
      const step = Math.floor(now * 2 + x * 0.7), bounce = (n, span) => { const m = n % (span * 2); return m < span ? m : span * 2 - m; };
      const sx = x + 4 + bounce(step, 4), sy = y + 4;
      const draw = () => { claudeSpark(sx, sy, 3, spark); dot(sx, sy, 0xf0a080); };
      draw();
      if (isNight) glowLate(draw);
    },
  },
  /** A brass-and-walnut wireless set seen from behind: a vented back with two valves glowing amber, a lit dial scale whose needle drifts to a new station now and then, and a wire aerial. */
  wirelessset: {
    draw(x, y) {
      const walnut = 0x5a3b25, walnutLight = 0x7a5232, walnutDark = 0x3a2418, brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a;
      // The case and its brass trim
      fill(x, y, 13, 9, walnut); fill(x, y, 13, 1, walnutLight); fill(x, y, 1, 9, walnutLight); fill(x + 12, y, 1, 9, walnutDark); fill(x, y + 8, 13, 1, walnutDark);
      fill(x + 1, y, 11, 1, brass); dot(x + 1, y, brassLight); fill(x + 1, y + 8, 11, 1, brassDark);
      // The back panel: a lit dial scale at the top, then vents with two glowing valves
      fill(x + 2, y + 2, 9, 2, 0x2a1a10); fill(x + 3, y + 2, 7, 2, 0xf2c060, isNight ? 1 : 0.8);
      const stop = [0, 2, 5, 3, 6, 1][Math.floor((now + x * 0.3) / 7) % 6];
      fill(x + 3 + stop, y + 2, 1, 2, 0x4a2a10);
      fill(x + 2, y + 5, 9, 3, walnutDark);
      const glow = 0.7 + 0.3 * Math.sin(now * 1.1 + x * 0.2);
      for (const vx of [x + 4, x + 8]) { fill(vx, y + 5, 2, 3, 0xe8782e, glow); dot(vx, y + 5, 0xffe08a, glow); }
      for (const vx of [x + 3, x + 6, x + 7, x + 10]) fill(vx, y + 5, 1, 3, walnut, 0.7);
      // The aerial: a wire to a little ball
      line(x + 11, y - 1, x + 11, y - 4, 0x4a4e58); dot(x + 11, y - 5, brassLight);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      glowAt(x + 6, y + 3, 9, 6, 0xffb04a, isNight ? 0.14 : 0.04);
      if (isNight) glowLate(() => { glowAt(x + 6, y + 3, 8, 5, 0xffc060, 0.14); dot(x + 4 + stop, y + 3, 0xfff4c0); });
    },
  },
  /** A scrying crystal screen seen from behind: a dark-bronze claw frame holding a faceted sea-green crystal in which a pale wisp slowly circles; it leaks a green glow at its edges, brighter at night. */
  scryingscreen: {
    draw(x, y) {
      const bronze = 0x4a3a2a, bronzeLight = 0x7a6244, bronzeDark = 0x2a2018, crystal = 0x2f8a78, crystalLight = 0x5ac8a8, crystalDark = 0x1a5a50;
      // The claw frame: four prongs and a rim
      fill(x + 1, y, 11, 9, bronzeDark); fill(x + 2, y, 9, 1, bronze); fill(x + 2, y + 8, 9, 1, bronze);
      for (const [px, py] of [[0, 0], [11, 0], [0, 7], [11, 7]]) { fill(x + px, y + py, 2, 2, bronze); dot(x + px, y + py, bronzeLight); }
      dot(x + 6, y, bronzeLight);
      // The crystal's back: facets in rows
      for (const [dx, w, dy] of [[3, 7, 1], [2, 9, 2], [2, 9, 3], [2, 9, 4], [2, 9, 5], [2, 9, 6], [3, 7, 7]]) fill(x + dx, y + dy, w, 1, crystal);
      fill(x + 3, y + 1, 7, 1, crystalLight, 0.6); fill(x + 2, y + 2, 1, 5, crystalLight, 0.5); fill(x + 10, y + 3, 1, 4, crystalDark);
      line(x + 3, y + 7, x + 6, y + 2, crystalDark, 0.8); line(x + 9, y + 7, x + 6, y + 2, crystalLight, 0.35); line(x + 3, y + 3, x + 10, y + 5, crystalDark, 0.5);
      // A wisp circling slowly inside, in steps
      const a = Math.floor(now * 1.5 + x * 0.3) % 8 * Math.PI / 4;
      dot(x + 6 + Math.round(Math.cos(a) * 2.5), y + 4 + Math.round(Math.sin(a) * 2), 0xe8fff4, 0.9);
      dot(x + 6 + Math.round(Math.cos(a - 0.8) * 2.5), y + 4 + Math.round(Math.sin(a - 0.8) * 2), 0xcfffe8, 0.4);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      glowAt(x + 6, y + 4, 9, 7, 0x60ffc0, isNight ? 0.16 : 0.05);
      if (isNight) glowLate(() => { glowAt(x + 6, y + 4, 8, 6, 0x7fffd0, 0.14); dot(x + 4, y + 3, 0xe8fff8, 0.7); });
    },
  },
  /** Order-book monitor. A slim graphite monitor whose back carries a little depth display: green bids on the left, red asks on the right, their bars shifting as orders come and go, each desk's at its own pace. */
  orderbook: {
    draw(x, y) {
      const shell = 0x2a2e36, shellLight = 0x3e434e, panel = 0x0a0e12;
      fill(x, y, 13, 9, shell); fill(x, y, 13, 1, shellLight); fill(x, y, 1, 9, shellLight);
      fill(x + 1, y + 1, 11, 7, panel);
      const tick = Math.floor(now * 1.5 + x * 0.7);
      for (let r = 0; r < 3; r++) {
        const bid = 1 + (r + hash(tick + r, x) % 3), ask = 1 + (2 - r + hash(tick + r + 9, y) % 3);
        fill(x + 6 - bid, y + 2 + r * 2, bid, 1, 0x3ad87a, 0.8);
        fill(x + 7, y + 2 + r * 2, ask, 1, 0xe8483e, 0.8);
      }
      fill(x + 6, y + 1, 1, 7, 0x2a3038);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 6, 4, 0x80c0a0, 0.08));
    },
  },
  /** Green-candle screen. A monitor whose back is a little chart: green candles climb one by one, each a step higher, and when they reach the top a gold spark pops and they start again from the bottom, each desk in its own time. */
  greencandles: {
    draw(x, y) {
      const shell = 0x22302a, shellLight = 0x34483e, panel = 0x07120c;
      fill(x, y, 13, 9, shell); fill(x, y, 13, 1, shellLight); fill(x, y, 1, 9, shellLight);
      fill(x + 1, y + 1, 11, 7, panel);
      const t = (now + x * 0.37) % 9, shown = Math.min(5, Math.floor(t * 0.8) + 1);
      for (let k = 0; k < shown; k++) {
        const top = y + 6 - k, cx = x + 2 + k * 2;
        fill(cx, top, 1, 2, 0x3ad87a); dot(cx, top - 1, 0x2a9a5a);
      }
      if (shown === 5 && t > 7.4) { const s = Math.floor(t * 6) % 2; dot(x + 11, y + 1, 0xffd860); if (s) { dot(x + 10, y + 1, 0xffe08a, 0.6); dot(x + 11, y + 2, 0xffe08a, 0.6); } }
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 6, 4, 0x3ad87a, 0.1));
    },
  },
  /** Red-candle screen. A monitor whose back is a little chart: red candles stepping down one by one to the bottom, a pause, then a small green one tries to climb back before it starts again, each desk in its own time. */
  redcandles: {
    draw(x, y) {
      const shell = 0x302226, shellLight = 0x483438, panel = 0x120709;
      fill(x, y, 13, 9, shell); fill(x, y, 13, 1, shellLight); fill(x, y, 1, 9, shellLight);
      fill(x + 1, y + 1, 11, 7, panel);
      const t = (now + x * 0.37) % 10, shown = Math.min(5, Math.floor(t * 0.7) + 1);
      for (let k = 0; k < shown; k++) { const top = y + 2 + k, cx = x + 2 + k * 2; fill(cx, top, 1, 2, 0xe8483e); dot(cx, top + 2, 0x9a2a2a); }
      if (t > 8) { dot(x + 11, y + 6, 0x3ad87a); if (t > 9) dot(x + 11, y + 5, 0x3ad87a); }
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 6, 4, 0xe8483e, 0.08));
    },
  },
  /** Stickered monitor. A charcoal monitor whose back is covered in stickers: an orange coin, a slanted-bars sticker and a little heart, one sticker's corner peeling; every 20 s a small LED on its edge blinks. */
  stickerback: {
    draw(x, y) {
      const shell = 0x2e3038, shellLight = 0x464a56;
      fill(x, y, 13, 9, shell); fill(x, y, 13, 1, shellLight); fill(x, y, 1, 9, shellLight);
      coinMark('orange', x + 3, y + 3, 2);
      coinMark('bars', x + 9, y + 4, 2);
      dot(x + 5, y + 6, 0xf28fb0); dot(x + 7, y + 6, 0xf28fb0); fill(x + 5, y + 7, 3, 1, 0xf28fb0); dot(x + 6, y + 8, 0xf28fb0);
      dot(x + 11, y + 7, 0xeae6dc); dot(x + 12, y + 8, 0xeae6dc, 0.7);
      if ((now + x * 0.3) % 20 < 0.5) dot(x + 12, y + 1, 0x3ad87a);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** Match monitor (brand nod: PSG). A navy monitor whose back carries the little badge; every 30 s a goal goes in and its edge lights flash red and white for three seconds, the glow flaring with them. */
  matchcast: {
    draw(x, y) {
      const shell = 0x0e2a46, shellLight = 0x1a4a78;
      fill(x, y, 13, 9, shell); fill(x, y, 13, 1, shellLight); fill(x, y, 1, 9, shellLight); fill(x + 12, y + 1, 1, 8, 0x08192c);
      psgNod(x + 6, y + 4, 3);
      const g = (now + x * 0.37) % 30, goal = g < 3, flash = goal && Math.floor(now * 6) % 2 === 0;
      for (let k = 1; k < 12; k += 2) dot(x + k, y, goal ? (flash === (k % 4 === 1) ? PSG_RED : WHITE) : shellLight);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (goal) glowAt(x + 6, y + 4, 8, 6, flash ? PSG_RED : WHITE, 0.12);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 6, 4, 0x3aa84a, 0.08));
    },
  },
  /** The Lakers game monitor (brand nod: Lakers). A purple shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in gold and white. */
  lakerscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.lakers, x, y); },
  },
  /** The Celtics game monitor (brand nod: Celtics). A green shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in white and gold. */
  celticscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.celtics, x, y); },
  },
  /** The Warriors game monitor (brand nod: Warriors). A blue shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in yellow and white. */
  warriorscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.warriors, x, y); },
  },
  /** The Knicks game monitor (brand nod: Knicks). A blue shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in orange and silver. */
  knickscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.knicks, x, y); },
  },
  /** The Bulls game monitor (brand nod: Bulls). A red shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in black and white. */
  bullscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.bulls, x, y); },
  },
  /** The Heat game monitor (brand nod: Heat). A red shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in orange and black. */
  heatcast: {
    draw(x, y) { nbaCast(NBA_TEAMS.heat, x, y); },
  },
  /** The Spurs game monitor (brand nod: Spurs). A black shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in silver and grey. */
  spurscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.spurs, x, y); },
  },
  /** The Thunder game monitor (brand nod: Thunder). A blue shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in orange and yellow. */
  thundercast: {
    draw(x, y) { nbaCast(NBA_TEAMS.thunder, x, y); },
  },
  /** The Timberwolves game monitor (brand nod: Timberwolves). A navy shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in green and blue. */
  wolvescast: {
    draw(x, y) { nbaCast(NBA_TEAMS.wolves, x, y); },
  },
  /** The Nuggets game monitor (brand nod: Nuggets). A navy shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in gold and red. */
  nuggetscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.nuggets, x, y); },
  },
  /** The Hawks game monitor (brand nod: Hawks). A red shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in volt and black. */
  hawkscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.hawks, x, y); },
  },
  /** The Nets game monitor (brand nod: Nets). A black shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in white and grey. */
  netscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.nets, x, y); },
  },
  /** The Hornets game monitor (brand nod: Hornets). A teal shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in purple and silver. */
  hornetscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.hornets, x, y); },
  },
  /** The Cavaliers game monitor (brand nod: Cavaliers). A wine shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in gold and navy. */
  cavscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.cavs, x, y); },
  },
  /** The Mavericks game monitor (brand nod: Mavericks). A blue shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in silver and navy. */
  mavscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.mavs, x, y); },
  },
  /** The Pistons game monitor (brand nod: Pistons). A blue shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in red and silver. */
  pistonscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.pistons, x, y); },
  },
  /** The Rockets game monitor (brand nod: Rockets). A red shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in silver and black. */
  rocketscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.rockets, x, y); },
  },
  /** The Pacers game monitor (brand nod: Pacers). A navy shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in gold and grey. */
  pacerscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.pacers, x, y); },
  },
  /** The Clippers game monitor (brand nod: Clippers). A red shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in blue and white. */
  clipperscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.clippers, x, y); },
  },
  /** The Grizzlies game monitor (brand nod: Grizzlies). A navy shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in blue and gold. */
  grizzliescast: {
    draw(x, y) { nbaCast(NBA_TEAMS.grizzlies, x, y); },
  },
  /** The Bucks game monitor (brand nod: Bucks). A green shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in cream and blue. */
  buckscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.bucks, x, y); },
  },
  /** The Pelicans game monitor (brand nod: Pelicans). A navy shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in red and gold. */
  pelicanscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.pelicans, x, y); },
  },
  /** The Magic game monitor (brand nod: Magic). A blue shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in black and silver. */
  magiccast: {
    draw(x, y) { nbaCast(NBA_TEAMS.magic, x, y); },
  },
  /** The 76ers game monitor (brand nod: 76ers). A blue shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in red and navy. */
  sixerscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.sixers, x, y); },
  },
  /** The Suns game monitor (brand nod: Suns). A purple shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in orange and yellow. */
  sunscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.suns, x, y); },
  },
  /** The Trail Blazers game monitor (brand nod: Trail Blazers). A red shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in black and white. */
  blazerscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.blazers, x, y); },
  },
  /** The Kings game monitor (brand nod: Kings). A purple shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in silver and black. */
  kingscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.kings, x, y); },
  },
  /** The Raptors game monitor (brand nod: Raptors). A red shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in black and silver. */
  raptorscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.raptors, x, y); },
  },
  /** The Jazz game monitor (brand nod: Jazz). A navy shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in yellow and sky blue. */
  jazzcast: {
    draw(x, y) { nbaCast(NBA_TEAMS.jazz, x, y); },
  },
  /** The Wizards game monitor (brand nod: Wizards). A navy shell with the badge on its back; every 30 s a dunk replay flashes its edge lights in red and silver. */
  wizardscast: {
    draw(x, y) { nbaCast(NBA_TEAMS.wizards, x, y); },
  },
  /** Shield-back monitor (brand nod: Marvel). A navy monitor carrying Cap's shield on its back; every 25 s the shield's star spins round and its edge lights run red, white and blue; a soft blue glow at night. */
  marvel1shieldmon: {
    draw(x, y) {
      fill(x, y, 13, 9, 0x1c2e5e); fill(x, y, 13, 1, 0x34508a); fill(x, y, 1, 9, 0x34508a); fill(x + 12, y + 1, 1, 8, 0x101c3c);
      m1Shield(x + 6, y + 4, 4);
      const t = (now + x * 0.37) % 25;
      if (t < 2.4) {
        const k = Math.floor(now * 8) % 4;
        dot(x + 6 + [0, 1, 0, -1][k], y + 4 + [-1, 0, 1, 0][k], WHITE);
        for (let i = 1; i < 12; i += 2) dot(x + i, y, [M1_RED, WHITE, M1_BLUE_LIGHT][(i + k) % 3]);
      }
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 6, 4, M1_BLUE_LIGHT, 0.08));
    },
  },
  /** A Kimoyo display (brand nod: Marvel): a slim black Wakandan screen seen from behind, a violet edge light and a triangle sigil glowing on its back; every 20 s a ring of kimoyo beads floats up over it and spins, glowing at night. */
  marvel2kimoyo: {
    draw(x, y) {
      fill(x, y, 13, 9, 0x1a1620); fill(x, y, 13, 1, M2_PURPLE); fill(x, y, 1, 9, 0x2e2840); fill(x + 12, y + 1, 1, 8, 0x0e0c14);
      line(x + 4, y + 7, x + 6, y + 2, M2_VIBE, 0.8); line(x + 6, y + 2, x + 8, y + 7, M2_VIBE, 0.8); fill(x + 4, y + 7, 5, 1, M2_VIBE, 0.6); dot(x + 6, y + 5, WHITE, 0.7);
      fill(x + 5, y + 9, 3, 2, 0x2e2840);
      const t = (now + x * 0.33) % 20;
      if (t < 3) {
        const rise = Math.min(3, Math.floor(t * 4));
        for (let k = 0; k < 6; k++) { const a = now * 3 + k * 1.05; dot(x + 6 + Math.round(Math.cos(a) * 4), y - 1 - rise + Math.round(Math.sin(a) * 1.5), k % 2 ? M2_VIBE : 0x3a3048); }
      }
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 6, 4, M2_VIBE, 0.08));
    },
  },
  /** A JARVIS screen (brand nod: Marvel): a graphite monitor whose back carries JARVIS's round blue display, its ring of segments turning slowly round a bright centre; every 22 s it pings, the ring flashing outward; a blue glow at night. */
  ironmanjarvis: {
    draw(x, y) {
      fill(x, y, 13, 9, IM_GRAPHITE); fill(x, y, 13, 1, 0x4a4e58); fill(x, y, 1, 9, 0x3a3e48); fill(x + 12, y + 1, 1, 8, 0x16181e);
      const cx = x + 6, cy = y + 4, turn = Math.floor(now * 2) % 8;
      for (let k = 0; k < 8; k++) { const a = (k / 8) * Math.PI * 2; if ((k + turn) % 4 !== 0) dot(cx + Math.round(Math.cos(a) * 3), cy + Math.round(Math.sin(a) * 3), IM_HOLO, 0.9); }
      dot(cx, cy, IM_ARC_WHITE); ring(cx, cy, 1.4, IM_HOLO, 0.6);
      const ping = (now + x * 0.37) % 22;
      if (ping < 0.8) ring(cx, cy, 3 + Math.floor(ping * 4), IM_ARC_WHITE, 0.8 - ping);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => { glowAt(cx, cy, 6, 4, IM_HOLO, 0.12); dot(cx, cy, IM_ARC_WHITE); });
    },
  },
  /** A Batcomputer screen (brand nod: DC): a black monitor with a blue glow round its edge and the bat stamped on its back; every 20 s its edge light flashes red, an alert coming in; blue light spills round it at night. */
  batcavescreen: {
    draw(x, y) {
      fill(x, y, 13, 9, BC_BLACK); fill(x, y, 13, 1, 0x2e3038); fill(x + 12, y + 1, 1, 8, 0x0c0c10);
      const alert = (now + x * 0.37) % 20 < 2 && Math.floor(now * 4) % 2 === 0, edge = alert ? 0xff3a3a : BC_SCREEN;
      fill(x - 1, y + 1, 1, 7, edge, 0.7); fill(x + 13, y + 1, 1, 7, edge, 0.7);
      bcBat(x + 6, y + 4, 0x3a3c46);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 9, 5, edge, 0.1));
    },
  },
  /** A Lantern monitor (brand nod: DC): a black monitor with the Lantern Corps symbol on its back; every 20 s it charges up and blazes emerald for a moment, green light washing round it; a soft green glow at night. */
  dcmixlanternmon: {
    draw(x, y) {
      fill(x, y, 13, 9, 0x101814); fill(x, y, 13, 1, 0x2a3a2e); fill(x, y, 1, 9, 0x2a3a2e); fill(x + 12, y + 1, 1, 8, 0x080c0a);
      const t = (now + x * 0.37) % 20, blaze = t < 1.8;
      dcLantern(x + 6, y + 4, blaze ? DC_GREEN_LIGHT : DC_GREEN);
      if (blaze) glowAt(x + 6, y + 4, 9, 6, DC_GREEN, 0.18);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => { glowAt(x + 6, y + 4, 7, 5, DC_GREEN, blaze ? 0.3 : 0.1); dcLantern(x + 6, y + 4, DC_GREEN, 0.8); });
    },
  },
  /** A Pokédex monitor: a red Pokédex-shaped case seen from the back, its hinge down the left side, the big blue lens in a white ring at the top with three small lights beside it; every 12 s the lens blinks bright as it scans, and it glows at night. */
  pokemondex: {
    draw(x, y) {
      fill(x, y, 13, 9, PKM_RED); fill(x, y, 13, 1, PKM_RED_LIGHT); fill(x + 12, y + 1, 1, 8, PKM_RED_DARK); fill(x, y + 8, 13, 1, PKM_RED_DARK);
      fill(x, y + 1, 1, 7, PKM_RED_LIGHT); fill(x + 1, y + 1, 1, 7, PKM_RED_DARK);
      const scan = (now + x * 0.29) % 12, on = scan < 1.5 && Math.floor(scan * 6) % 2 === 0;
      disc(x + 5, y + 3, 2.5, PKM_WHITE); disc(x + 5, y + 3, 1.6, on ? 0x9ad8ff : PKM_BLUE); dot(x + 4, y + 2, WHITE, 0.9);
      dot(x + 9, y + 2, 0xe83a2a); dot(x + 10, y + 2, 0xf8d030); dot(x + 11, y + 2, 0x4ac85a);
      fill(x + 3, y + 6, 8, 1, PKM_RED_DARK);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (on) glowAt(x + 5, y + 3, 5, 4, 0x6ab8ff, 0.18);
      if (isNight) glowLate(() => { glowAt(x + 5, y + 3, 4, 3, 0x6ab8ff, 0.15); dot(x + 5, y + 3, on ? 0xd8f0ff : 0x7ab0f0); });
    },
  },
  /** A Hextech monitor: a navy back in a gold frame with studded corners and a blue hextech crystal set in the middle, pulsing slowly; every 15 s it flares bright, and it glows at night. */
  lolhexmon: {
    draw(x, y) {
      hexPanel(x, y, 13, 9);
      const f = (now + x * 0.33) % 15, flare = f < 1;
      hexCrystal(x + 6, y + 4, 7, flare ? 1 : (Math.sin(now * 1.4 + x) + 1) / 4);
      if (flare) glowAt(x + 6, y + 4, 8, 6, LOL_HEX, 0.22 * (1 - f));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => { glowAt(x + 6, y + 4, 5, 4, LOL_HEX, 0.14); dot(x + 6, y + 3, LOL_HEX_LIGHT); });
    },
  },
  /** A mage portal monitor: a carved grey stone frame set with violet rune stones, a swirling portal of violet and blue inside it turning slowly; it glows at night. */
  wowportal: {
    draw(x, y) {
      fill(x, y, 13, 9, 0x6a6a74); fill(x, y, 13, 1, 0x9a9aa4); fill(x, y, 1, 9, 0x8a8a94); fill(x + 12, y + 1, 1, 8, 0x4a4a54);
      fill(x + 2, y + 1, 9, 7, 0x2a1a4a);
      for (let k = 0; k < 6; k++) {
        const a = now * 1.5 + k * Math.PI / 3, r = 1 + (k % 3);
        dot(x + 6 + Math.round(Math.cos(a) * r * 1.4), y + 4 + Math.round(Math.sin(a) * r * 0.9), k % 2 ? WOW_ARCANE : 0x5aa8f0);
      }
      dot(x + 6, y + 4, 0xe8d8ff);
      for (const [rx, ry] of [[1, 0], [11, 0], [1, 8], [11, 8], [6, 0]]) dot(x + rx, y + ry, WOW_ARCANE);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      glowAt(x + 6, y + 4, 4, 3, WOW_ARCANE, 0.1);
      if (isNight) glowLate(() => { glowAt(x + 6, y + 4, 6, 4, WOW_ARCANE, 0.18); dot(x + 6, y + 4, 0xf4ecff); });
    },
  },
  /** A radar monitor: a black back with the round radar of the streets on it, a white arrow at its middle and a blip that blinks, and five little wanted stars along the top that flash red and blue for a while every 20 s. */
  gtawanted: {
    draw(x, y) {
      fill(x, y, 13, 9, 0x1a1a20); fill(x, y, 13, 1, 0x3a3a44); fill(x + 12, y + 1, 1, 8, 0x0e0e14);
      disc(x + 4, y + 5, 3.4, 0x3a4a3a); ring(x + 4, y + 5, 3.6, 0x0a0a0e); fill(x + 1, y + 5, 7, 1, 0x7a7e84); fill(x + 4, y + 2, 1, 7, 0x7a7e84);
      dot(x + 4, y + 5, WHITE); if (Math.floor(now * 2 + x) % 2) dot(x + 6, y + 3, 0xf2c43a);
      const t = (now + x * 0.27) % 20, wanted = t < 4;
      for (let k = 0; k < 5; k++) dot(x + 8 + k, y + 2, wanted ? ((Math.floor(now * 6) + k) % 2 ? 0xff3a4a : 0x3a6aff) : 0x5a5a64);
      fill(x + 8, y + 4, 4, 1, 0x3ab84a); fill(x + 8, y + 6, 4, 1, 0x3a8ae8);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (wanted) glowAt(x + 10, y + 2, 4, 2, Math.floor(now * 6) % 2 ? 0xff3a4a : 0x3a6aff, 0.2);
      if (isNight) glowLate(() => glowAt(x + 4, y + 5, 4, 4, 0x6ac86a, 0.1));
    },
  },
  /** A Traveler monitor: a dark navy back with a gold rim and the Traveler on it, a pale sphere with its scar and a halo of Light that breathes slowly; it glows softly at night. */
  destinytraveler: {
    draw(x, y) {
      fill(x, y, 13, 9, 0x1a2434); fill(x, y, 13, 1, DST_GOLD); fill(x + 12, y + 1, 1, 8, 0x0e141e); fill(x, y + 8, 13, 1, 0x0e141e);
      const breath = (Math.sin(now * 0.9 + x) + 1) / 2;
      traveler(x + 6, y + 4, 2.6, 0.06 + 0.08 * breath);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 5, 4, 0xfff8e0, 0.12 + 0.06 * breath));
    },
  },
  /** A starfighter monitor: seen from behind, a round cockpit ball with its hatch between two hexagonal wing panels; the twin engines at its back glow red, flaring every 25 s. */
  starwarsmon: {
    draw(x, y) {
      const wing = 0x3a3e48, wingLight = 0x5a5e68, wingDark = 0x24262c, ball = 0x6a6e78;
      // The wings: tall hex panels, ribbed
      for (const wx of [x, x + 11]) {
        fill(wx, y + 1, 2, 7, wing); dot(wx, y, wing); dot(wx + 1, y + 8, wingDark); fill(wx, y + 1, 1, 7, wingLight);
        dot(wx + 1, y + 2, wingDark); dot(wx + 1, y + 4, wingDark); dot(wx + 1, y + 6, wingDark);
      }
      fill(x + 2, y + 4, 9, 1, SW_GREY_DARK);
      // The ball, its hatch and the engines
      disc(x + 6.5, y + 4.5, 3.6, ball); dot(x + 5, y + 2, 0x9a9ea8); ring(x + 6.5, y + 4.5, 1.6, wingDark, 0.9);
      const g = (now + x * 0.37) % 25, flare = g < 1.5;
      for (const ex of [x + 5, x + 8]) dot(ex, y + 7, flare ? 0xffa060 : 0xd84a2a);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (flare) glowAt(x + 6, y + 7, 5, 3, 0xff7040, 0.15);
      if (isNight) glowLate(() => { glowAt(x + 6.5, y + 7, 4, 2, 0xff6a3a, flare ? 0.25 : 0.12); });
    },
  },
  /** A pink monitor: its back hot pink with a rounded rim, a white script B in the middle (brand nod: Barbie) and rhinestones along the top that sparkle in turn every 20 s; a soft pink glow at night. */
  girly1mon: {
    draw(x, y) {
      fill(x + 1, y, 11, 9, G1_PINK); fill(x, y + 1, 13, 7, G1_PINK); fill(x + 1, y, 11, 1, G1_PINK_LIGHT); fill(x, y + 1, 1, 7, G1_PINK_LIGHT); fill(x + 12, y + 1, 1, 7, G1_PINK_DARK); fill(x + 1, y + 8, 11, 1, G1_PINK_DARK);
      g1Mark('b', x + 5, y + 2, G1_WHITE);
      const t = (now + x * 0.37) % 20;
      for (let k = 0; k < 5; k++) { dot(x + 2 + k * 2, y + 1, 0xffd0ec, 0.8); if (t < 2.5 && Math.floor(t * 2) === k) g1Twinkle(x + 2 + k * 2, y + 1, (t * 2) % 1); }
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 7, 5, 0xff7ac0, 0.1));
    },
  },
  /** A cat-ear monitor: a white rounded back with two pointed ears on top, pink inside, a red bow on the left one and three whiskers each side; every 18 s an ear flicks. A soft pink glow at night. */
  girly2mon: {
    draw(x, y) {
      const flick = (now + x * 0.37) % 18 < 0.5 ? 1 : 0;
      fill(x + 1, y - 2 + flick, 3, 2, 0xfffcfa); dot(x + 2, y - 3 + flick, 0xfffcfa); dot(x + 2, y - 1 + flick, G2_PINK_DARK);
      fill(x + 9, y - 2, 3, 2, 0xfffcfa); dot(x + 10, y - 3, 0xfffcfa); dot(x + 10, y - 1, G2_PINK_DARK);
      fill(x + 1, y, 11, 9, 0xfffcfa); fill(x, y + 1, 13, 7, 0xfffcfa); fill(x, y + 1, 1, 7, WHITE); fill(x + 12, y + 1, 1, 7, 0xe0d8dc); fill(x + 1, y + 8, 11, 1, 0xe0d8dc);
      fill(x, y - 3 + flick, 2, 2, G2_BOW); fill(x + 3, y - 3 + flick, 2, 2, G2_BOW); dot(x + 2, y - 2 + flick, 0xb81e34);
      for (const wy of [3, 5]) { fill(x + 1, y + wy, 3, 1, 0xd0c8cc); fill(x + 9, y + wy, 3, 1, 0xd0c8cc); }
      dot(x + 6, y + 4, G2_NOSE);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 7, 5, 0xffb0d0, 0.09));
    },
  },
  /** A gold-plated monitor: its back solid gold with a raised border and an engraved dollar sign; every 20 s a shine sweeps across it. */
  narcosmon: {
    draw(x, y) {
      fill(x, y, 13, 9, NC_GOLD); fill(x, y, 13, 1, NC_GOLD_LIGHT); fill(x, y, 1, 9, NC_GOLD_LIGHT); fill(x + 12, y + 1, 1, 8, NC_GOLD_DARK); fill(x + 1, y + 8, 12, 1, NC_GOLD_DARK);
      fill(x + 2, y + 2, 9, 5, 0xc8a038);
      fill(x + 5, y + 2, 3, 1, NC_GOLD_DARK); dot(x + 5, y + 3, NC_GOLD_DARK); fill(x + 5, y + 4, 3, 1, NC_GOLD_DARK); dot(x + 7, y + 5, NC_GOLD_DARK); fill(x + 5, y + 6, 3, 1, NC_GOLD_DARK); fill(x + 6, y + 1, 1, 7, NC_GOLD_DARK, 0.7);
      const g = (now + x * 0.37) % 20;
      if (g < 1) { const gx = x + Math.floor(g * 16); line(gx, y + 8, gx + 3, y, NC_GOLD_LIGHT); line(gx + 1, y + 8, gx + 4, y, WHITE, 0.6); }
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** An all-in-one from behind: a pastel blue back with rounded corners and a slightly darker apple in the middle (brand nod: Apple), on a slim silver foot; every 25 s a reflection slides across it. */
  apple1imac: {
    draw(x, y) {
      const back = 0x8ab4d8, light = 0xa8cce8, dark = 0x6a94b8;
      fill(x + 1, y, 11, 9, back); fill(x, y + 1, 13, 7, back); fill(x + 1, y, 11, 1, light); fill(x, y + 1, 1, 7, light); fill(x + 12, y + 1, 1, 7, dark); fill(x + 1, y + 8, 11, 1, dark);
      appleMark('small', x + 5, y + 2, dark);
      const g = (now + x * 0.37) % 25;
      if (g < 1) { const gx = x + Math.floor(g * 15); line(gx, y + 8, gx + 3, y, WHITE, 0.5); }
      fill(x + 5, y + 9, 3, 2, A1_SILVER); fill(x + 4, y + 10, 5, 1, A1_SILVER_DARK);
    },
  },
  /** A compact all-in-one from behind: a tall beige case with a carry handle cut into its top, rows of vents and a small rainbow apple low on its back (brand nod: Apple); its little power light blinks now and then. */
  apple2classic: {
    draw(x, y) {
      fill(x + 1, y, 11, 9, A2_BEIGE); fill(x + 1, y, 11, 1, A2_BEIGE_LIGHT); fill(x + 1, y, 1, 9, A2_BEIGE_LIGHT); fill(x + 11, y + 1, 1, 8, A2_BEIGE_DARK); fill(x + 1, y + 8, 11, 1, A2_BEIGE_DARK);
      fill(x + 4, y + 1, 5, 1, A2_BEIGE_DARK); fill(x + 4, y + 2, 5, 1, 0x8a8270);
      for (let k = 0; k < 3; k++) fill(x + 3, y + 4 + k, 7, 1, 0xc8c0a8);
      appleMark('small', x + 2, y + 3, (r) => A2_RAINBOW[r], 0.0);
      for (let k = 0; k < 4; k++) dot(x + 2, y + 4 + k, A2_RAINBOW[k + 1]);
      if ((now + x * 0.3) % 7 < 0.4) dot(x + 10, y + 7, 0x61bb46);
      fill(x + 5, y + 9, 3, 2, A2_BEIGE_DARK);
    },
  },
  /** A white monitor seen from behind with two blue stripes across its back and a pomegranate sticker between them; its little power light breathes blue, and shines at night. */
  jewishmon: {
    draw(x, y) {
      const white = 0xf2f0ea, edge = 0xd8d4ca, blue = 0x2f5f9e, blueLight = 0x5a8ad0;
      fill(x, y, 13, 9, white); fill(x, y, 13, 1, WHITE); fill(x, y, 1, 9, WHITE); fill(x + 12, y, 1, 9, edge); fill(x, y + 8, 13, 1, edge);
      // The stripes, and the pomegranate with its crown
      fill(x + 1, y + 1, 11, 1, blue); fill(x + 1, y + 7, 11, 1, blue); dot(x + 1, y + 1, blueLight); dot(x + 1, y + 7, blueLight);
      fill(x + 5, y + 3, 3, 3, 0xc0283a); dot(x + 5, y + 3, 0xe04a58); dot(x + 7, y + 5, 0x801828); dot(x + 6, y + 2, 0xc8a050);
      // The power light
      const a = 0.55 + 0.45 * Math.sin(now * 1.2 + x * 0.3);
      dot(x + 10, y + 4, 0x5ab0ff, a);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => { glowAt(x + 10, y + 4, 2, 2, 0x5ab0ff, 0.2 * a); dot(x + 10, y + 4, 0xb8e0ff, a); });
    },
  },
  /** A monitor seen from behind clad in a brass lattice screen; a warm amber light breathes through its openings, and shines through the night. */
  muslimmon: {
    draw(x, y) {
      const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, back = 0x3a2a1e;
      fill(x, y, 13, 9, brass); fill(x, y, 13, 1, brassLight); fill(x, y, 1, 9, brassLight); fill(x + 12, y, 1, 9, brassDark); fill(x, y + 8, 13, 1, brassDark);
      fill(x + 1, y + 1, 11, 7, back);
      // The lattice, the light showing through its openings
      const a = 0.55 + 0.35 * Math.sin(now * 0.9 + x * 0.2), holes = [];
      for (let gy = 0; gy < 3; gy++) for (let gx = 0; gx < 4; gx++) holes.push([x + 2 + gx * 3, y + 2 + gy * 2]);
      for (const [hx, hy] of holes) { dot(hx, hy, 0xffb050, a); dot(hx - 1, hy, brass, 0.8); }
      fill(x + 1, y + 3, 11, 1, brassDark, 0.8); fill(x + 1, y + 5, 11, 1, brassDark, 0.8);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => { glowAt(x + 6, y + 4, 8, 6, 0xffb050, 0.12); for (const [hx, hy] of holes) dot(hx, hy, 0xffd890, a); });
    },
  },
  /** A monitor seen from behind in an oak case with a little round rose window of coloured glass set in its back, glowing softly, and through the night. */
  christianmon: {
    draw(x, y) {
      const oak = 0x8a5a32, light = 0xa8703e, dark = 0x5a3b25, glass = [0xc8303a, 0x2f5fb0, 0xf2b030, 0x3a9a5a], cx = x + 6, cy = y + 4;
      fill(x, y, 13, 9, oak); fill(x, y, 13, 1, light); fill(x, y, 1, 9, light); fill(x + 12, y, 1, 9, dark); fill(x, y + 8, 13, 1, dark);
      disc(cx, cy, 3.4, 0x2a2a30);
      const petals = (a) => {
        for (let k = 0; k < 8; k++) { const ang = (k / 8) * Math.PI * 2; dot(cx + Math.round(Math.cos(ang) * 2), cy + Math.round(Math.sin(ang) * 2), glass[k % 4], a); }
        dot(cx, cy, 0xf2d35b, a);
      };
      petals(0.9);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => { glowAt(cx, cy, 5, 4, 0xffe0a0, 0.12); petals(0.8); });
    },
  },
  /** A monitor seen from behind in red lacquer with a thin gold border and a gold lotus in the middle, which glows faintly at night. */
  buddhistmon: {
    draw(x, y) {
      const lac = 0xa8282a, lacLight = 0xc8403a, lacDark = 0x7a1a1c, gold = 0xd8b45a, goldDark = 0xb8943a;
      fill(x, y, 13, 9, lac); fill(x, y, 13, 1, lacLight); fill(x, y, 1, 9, lacLight); fill(x + 12, y, 1, 9, lacDark); fill(x, y + 8, 13, 1, lacDark);
      fill(x + 1, y + 1, 11, 1, gold, 0.5); fill(x + 1, y + 7, 11, 1, gold, 0.5);
      const lotus = (a) => { dot(x + 6, y + 2, gold, a); fill(x + 5, y + 3, 3, 2, gold, a); dot(x + 4, y + 4, gold, a); dot(x + 8, y + 4, gold, a); fill(x + 4, y + 5, 5, 1, goldDark, a); };
      lotus(1);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => { glowAt(x + 6, y + 4, 5, 3, 0xffd890, 0.08); lotus(0.6); });
    },
  },
  /** A monitor seen from behind in gunmetal, a little orange volcano on its back, yellow hazard dots along its foot and a red status light that blinks; it glows faintly at night. */
  volcanomon: {
    draw(x, y) {
      const steel = 0x3a3c44, light = 0x5a5e68, dark = 0x24262c;
      fill(x, y, 13, 9, steel); fill(x, y, 13, 1, light); fill(x, y, 1, 9, light); fill(x + 12, y, 1, 9, dark); fill(x, y + 8, 13, 1, dark);
      dot(x + 6, y + 2, 0xffa040); fill(x + 5, y + 3, 3, 1, 0xe8501e); fill(x + 4, y + 4, 5, 1, 0x8a3a1e); fill(x + 3, y + 5, 7, 1, 0x6a2e1a);
      for (let k = 0; k < 11; k += 2) dot(x + 1 + k, y + 7, 0xf2c830);
      const on = (now + x * 0.37) % 3 < 1.5;
      dot(x + 11, y + 1, on ? 0xff3020 : 0x5a1a14);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => { if (on) dot(x + 11, y + 1, 0xff6040); glowAt(x + 6, y + 3, 4, 3, 0xff7030, 0.08); });
    },
  },
  /** The back of a kitchen ticket screen in stainless steel, a little ticket printer clamped on top; every 18 s it chatters and a new ticket feeds up out of it, then tears off. */
  kitchenmon: {
    draw(x, y) {
      const steel = 0x6e727c, steelLight = 0xa8adb6, steelDark = 0x4a4e58;
      fill(x, y, 13, 9, steel); fill(x, y, 13, 1, steelLight); fill(x, y, 1, 9, steelLight); fill(x + 12, y, 1, 9, steelDark); fill(x, y + 8, 13, 1, steelDark);
      for (let k = 0; k < 3; k++) fill(x + 3, y + 2 + k * 2, 7, 1, steelDark);
      dot(x + 2, y + 7, 0x5aff6a);
      fill(x + 8, y - 2, 4, 2, 0x3a3c44); fill(x + 8, y - 2, 4, 1, 0x5a5e68);
      const t = (now + x * 0.29) % 18, feed = 1 + (t < 2.5 ? Math.min(4, Math.floor(t * 2)) : 0);
      fill(x + 9, y - 2 - feed, 2, feed, 0xf8f6ee); dot(x + 9, y - 1 - feed, 0xd8d6ce);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor on a medical cart seen from behind: a pale grey case with a carry handle on top and a blue band, a little green power light. */
  hospitalmon: {
    draw(x, y) {
      const c = 0xe0e4e6, light = 0xf6f8f8, dark = 0xa8b0b6;
      fill(x, y, 13, 9, c); fill(x, y, 13, 1, light); fill(x, y, 1, 9, light); fill(x + 12, y, 1, 9, dark); fill(x, y + 8, 13, 1, dark);
      fill(x + 3, y - 2, 7, 1, dark); dot(x + 3, y - 1, dark); dot(x + 9, y - 1, dark);
      fill(x + 1, y + 6, 11, 1, 0x4c8bf5); for (let k = 0; k < 3; k++) fill(x + 4, y + 2 + k, 5, 1, dark, 0.5);
      dot(x + 11, y + 1, 0x5aff6a);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => dot(x + 11, y + 1, 0xa0ffb0));
    },
  },
  /** A boxy old set in a varnished wood case seen from behind, cooling vents in its back panel and a fat cable; the glow of its tube shows through the vents at night. */
  noirmon: {
    draw(x, y) {
      const wood = 0x6a4428, woodLight = 0x8a5a32, woodDark = 0x4a2e1a;
      fill(x, y, 13, 9, wood); fill(x, y, 13, 1, woodLight); fill(x, y, 1, 9, woodLight); fill(x + 12, y, 1, 9, woodDark); fill(x, y + 8, 13, 1, woodDark);
      fill(x + 2, y + 2, 9, 5, 0x2a1c12); for (let k = 0; k < 5; k += 2) fill(x + 3, y + 2 + k, 7, 1, 0x4a3020);
      line(x + 11, y + 8, x + 13, y + 10, 0x1e1e22);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => { for (let k = 1; k < 5; k += 2) fill(x + 3, y + 2 + k, 7, 1, 0x8ab8a0, 0.35); });
    },
  },
  /** An iron cabinet seen from behind, its back studded with rivets, a round porthole where the green tube inside glows and a copper cable trailing from it. */
  madlabmon: {
    draw(x, y) {
      const iron = 0x4a4440, ironLight = 0x6a625c, ironDark = 0x2e2a28;
      fill(x, y, 13, 9, iron); fill(x, y, 13, 1, ironLight); fill(x, y, 1, 9, ironLight); fill(x + 12, y, 1, 9, ironDark); fill(x, y + 8, 13, 1, ironDark);
      for (const [rx, ry] of [[1, 1], [11, 1], [1, 7], [11, 7]]) dot(x + rx, y + ry, 0x8a827a);
      const tube = () => { disc(x + 6, y + 4, 2, 0x3ab84a); dot(x + 6, y + 4, 0xb8ff8a); };
      disc(x + 6, y + 4, 2.8, 0xb87333); tube();
      line(x + 12, y + 7, x + 14, y + 10, 0xb87333);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => { glowAt(x + 6, y + 4, 4, 4, 0x7aff4a, 0.15); tube(); });
    },
  },
  /** A monitor in a gilded casing seen from behind, a little red and gold topper light on top like a slot machine's that blinks now and then. */
  casinomon: {
    draw(x, y) {
      const gold = 0xd8b45a, goldLight = 0xf2d888, goldDark = 0x9a7a30;
      fill(x, y, 13, 9, 0x2a1418); fill(x, y, 13, 1, goldLight); fill(x, y, 1, 9, gold); fill(x + 12, y, 1, 9, goldDark); fill(x, y + 8, 13, 1, goldDark);
      for (let k = 2; k < 11; k += 3) dot(x + k, y + 4, gold);
      fill(x + 5, y - 2, 3, 2, 0xd8302a); const on = (now + x * 0.3) % 4 < 0.6; dot(x + 6, y - 2, on ? 0xfff4c0 : 0xe85a4a);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight && on) glowLate(() => glowAt(x + 6, y - 2, 3, 2, 0xff5a3a, 0.3));
    },
  },
  /** A monitor in a mahogany case seen from behind, inlaid with a pale wood sunrise of rays in the art deco manner and edged in brass. */
  orientmon: {
    draw(x, y) {
      const wood = 0x6a2e1e, woodLight = 0x8a4a2e, woodDark = 0x4a1e12, inlay = 0xe8d0a0;
      fill(x, y, 13, 9, wood); fill(x, y, 13, 1, 0xd8b45a); fill(x, y, 1, 9, woodLight); fill(x + 12, y, 1, 9, woodDark); fill(x, y + 8, 13, 1, 0xd8b45a);
      for (let k = 0; k < 5; k++) { const a = Math.PI + 0.2 + (k / 4) * (Math.PI - 0.4); line(x + 6, y + 7, x + 6 + Math.round(Math.cos(a) * 4), y + 7 + Math.round(Math.sin(a) * 4), inlay, 0.8); }
      dot(x + 6, y + 7, 0xd8b45a);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor in an ornate black carved frame seen from behind, a little black raven perched on top that turns its head this way and that and blinks its red eyes. */
  halloweendxmon: {
    draw(x, y) {
      const frame = 0x1e1820, frameLight = 0x3a2e3a;
      fill(x, y, 13, 9, frame); fill(x, y, 13, 1, frameLight); fill(x, y, 1, 9, frameLight); fill(x + 12, y, 1, 9, 0x0e0a0e); fill(x, y + 8, 13, 1, 0x0e0a0e);
      for (const [cx, cy] of [[1, 1], [11, 1], [1, 7], [11, 7]]) dot(x + cx, y + cy, 0x6a4a7a);
      for (let k = 3; k < 10; k += 2) dot(x + k, y + 4, 0x3a2e3a);
      const look = Math.floor((now + x * 0.3) / 3) % 2 ? 1 : -1, blink = (now + x * 0.21) % 5 < 0.2;
      fill(x + 4, y - 3, 4, 3, 0x14101a); fill(x + 4 + (look > 0 ? 3 : -1), y - 4, 2, 2, 0x14101a); dot(x + (look > 0 ? 9 : 2), y - 3, 0x3a3a40);
      if (!blink) dot(x + (look > 0 ? 8 : 3), y - 4, 0xff3a3a);
      dot(x + 3, y - 1, 0x14101a); dot(x + 2, y - 2, 0x14101a);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight && !blink) glowLate(() => dot(x + (look > 0 ? 8 : 3), y - 4, 0xff7070));
    },
  },
  /** A monitor seen from behind in soft pink with two little cat ears on its top edge that twitch now and then, and a paw print sticker. */
  catcafemon: {
    draw(x, y) {
      const c = 0xf2c0c8, light = 0xf8d8de, dark = 0xd898a4;
      fill(x, y, 13, 9, c); fill(x, y, 13, 1, light); fill(x, y, 1, 9, light); fill(x + 12, y, 1, 9, dark); fill(x, y + 8, 13, 1, dark);
      const tw = (now + x * 0.3) % 7 < 0.3 ? 1 : 0;
      fill(x + 1, y - 2 + tw, 3, 2, c); dot(x + 2, y - 3 + tw, c); dot(x + 2, y - 1, 0xf28fb0); fill(x + 9, y - 2, 3, 2, c); dot(x + 10, y - 3, c); dot(x + 10, y - 1, 0xf28fb0);
      dot(x + 6, y + 5, WHITE); for (const [dx, dy] of [[5, 3], [7, 3], [4, 4], [8, 4]]) dot(x + dx, y + dy, WHITE);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A matte-black monitor seen from behind with a red spider sticker in the middle and a few torn comic stickers; now and then it glitches into red and cyan copies for a blink. The spider glows red at night. */
  spiderversemon: {
    draw(x, y) {
      const body = 0x1c1a22, bodyLight = 0x2e2a36, red = 0xd8202a;
      const g = (now + x * 0.37) % 13 < 0.3;
      if (g) { fill(x - 1, y, 13, 9, 0xff3a6a, 0.5); fill(x + 1, y, 13, 9, 0x30e0f0, 0.5); }
      fill(x, y, 13, 9, body); fill(x, y, 13, 1, bodyLight); fill(x, y, 1, 9, bodyLight); fill(x + 12, y, 1, 9, 0x101016);
      const spider = (a) => { fill(x + 6, y + 2, 1, 5, red, a); dot(x + 6, y + 3, red, a); for (const s of [-1, 1]) { dot(x + 6 + s, y + 3, red, a); dot(x + 6 + s * 2, y + 2, red, a); dot(x + 6 + s * 2, y + 5, red, a); dot(x + 6 + s, y + 5, red, a); } };
      spider(1);
      fill(x + 1, y + 6, 3, 2, 0xf2d35b); dot(x + 2, y + 6, 0x1c1a22); fill(x + 10, y + 1, 2, 2, 0x30e0f0);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => { glowAt(x + 6, y + 4, 4, 3, 0xff3040, 0.1); spider(0.7); });
    },
  },
  /** A monitor seen from behind in the team's yellow and black, the X emblem in the middle, which glows a little at night. */
  xmenmon: {
    draw(x, y) {
      fill(x, y, 13, 9, 0xf2c41a); fill(x, y, 13, 1, 0xf8d860); fill(x, y, 1, 9, 0xf8d860); fill(x + 12, y, 1, 9, 0xc89a10); fill(x, y + 8, 13, 1, 0xc89a10);
      fill(x + 1, y + 1, 3, 7, 0x1c1a22); fill(x + 9, y + 1, 3, 7, 0x1c1a22);
      xmenX(x + 6, y + 4, 3);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 4, 3, 0xffd040, 0.1));
    },
  },
  /** A timing monitor seen from behind in carbon, a red team stripe across it and a little yellow shield badge. */
  f1mon: {
    draw(x, y) {
      fill(x, y, 13, 9, 0x24262a); fill(x, y, 13, 1, 0x3a3c42); fill(x, y, 1, 9, 0x3a3c42); fill(x + 12, y, 1, 9, 0x16181c);
      for (let k = 1; k < 12; k += 2) dot(x + k, y + 2 + (k % 4 ? 0 : 1), 0x30333a);
      fill(x + 1, y + 5, 11, 2, 0xd81a1a); fill(x + 5, y + 2, 3, 3, 0xf2c41a); dot(x + 6, y + 3, 0x1c1a22); dot(x + 6, y + 5, 0xf2c41a);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A black monitor seen from behind with a gold belt-plate badge set with a red stone in its middle and a red stripe along its foot. */
  ufcmon: {
    draw(x, y) {
      fill(x, y, 13, 9, 0x1c1a22); fill(x, y, 13, 1, 0x2e2a36); fill(x, y, 1, 9, 0x2e2a36); fill(x + 12, y, 1, 9, 0x101016);
      fill(x + 4, y + 2, 5, 5, 0xd8b45a); dot(x + 4, y + 2, 0xf2d888); dot(x + 6, y + 4, 0xd8202a); fill(x + 1, y + 4, 3, 1, 0x9a7a30); fill(x + 9, y + 4, 3, 1, 0x9a7a30);
      fill(x + 1, y + 7, 11, 1, 0xd8202a);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor seen from behind in a walnut-veneer case with a thin brass trim line and a round cable port. */
  nyloftmon: {
    draw(x, y) {
      fill(x, y, 13, 9, 0x6a4a2e); fill(x, y, 13, 1, 0x8a6440); fill(x, y, 1, 9, 0x8a6440); fill(x + 12, y, 1, 9, 0x4a3420); fill(x, y + 8, 13, 1, 0x4a3420);
      for (let k = 2; k < 12; k += 3) fill(x + k, y + 2, 1, 5, 0x5a3e26, 0.6);
      fill(x + 1, y + 7, 11, 1, 0xc8a050, 0.6); disc(x + 6, y + 4, 1, 0x2a2a30);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor seen from behind in cream enamel edged with a thin gilt line, a small painted rose in the middle. */
  pariscafemon: {
    draw(x, y) {
      fill(x, y, 13, 9, 0xf2ead8); fill(x, y, 13, 1, WHITE); fill(x, y, 1, 9, WHITE); fill(x + 12, y, 1, 9, 0xd8ccb0); fill(x, y + 8, 13, 1, 0xd8ccb0);
      fill(x + 1, y + 1, 11, 1, 0xc8a050, 0.6); fill(x + 1, y + 7, 11, 1, 0xc8a050, 0.6);
      fill(x + 5, y + 3, 3, 2, 0xc83a4a); dot(x + 6, y + 3, 0xe85a6a); dot(x + 4, y + 5, 0x4a8a4a); dot(x + 8, y + 5, 0x4a8a4a);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor seen from behind, painted marigold orange with a white sugar skull in the middle and little pink flowers in its corners. */
  muertosmon: {
    draw(x, y) {
      fill(x, y, 13, 9, 0xe8902a); fill(x, y, 13, 1, 0xf2b03a); fill(x, y, 1, 9, 0xf2b03a); fill(x + 12, y, 1, 9, 0xc87020); fill(x, y + 8, 13, 1, 0xc87020);
      muertosSkull(x + 6, y + 4);
      for (const [dx, dy] of [[1, 1], [11, 1], [1, 7], [11, 7]]) dot(x + dx, y + dy, 0xe83a8a);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A rugged monitor seen from behind: a white case with orange rubber bumpers on its corners and a row of cooling slots. */
  submarinedeepmon: {
    draw(x, y) {
      fill(x, y, 13, 9, 0xe8ecee); fill(x, y, 13, 1, WHITE); fill(x + 12, y, 1, 9, 0xb8c2ca); fill(x, y + 8, 13, 1, 0xb8c2ca);
      for (const [dx, dy] of [[0, 0], [11, 0], [0, 7], [11, 7]]) fill(x + dx, y + dy, 2, 2, 0xe8702a);
      for (let k = 3; k < 10; k += 2) fill(x + k, y + 3, 1, 3, 0x8a96a0);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A rugged grey monitor seen from behind, frost feathering in from its corners and a red station sticker on it. */
  arcticbasemon: {
    draw(x, y) {
      fill(x, y, 13, 9, 0x6a727a); fill(x, y, 13, 1, 0x8a929a); fill(x, y, 1, 9, 0x8a929a); fill(x + 12, y, 1, 9, 0x4a525a);
      for (const [dx, dy] of [[1, 1], [2, 1], [1, 2], [11, 7], [10, 7], [11, 6], [1, 7], [11, 1]]) dot(x + dx, y + dy, 0xd8f0ff);
      fill(x + 5, y + 3, 3, 3, 0xb8302a); dot(x + 6, y + 4, WHITE);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor seen from behind in pastel pink with a white five-petal blossom in its middle and a little gold centre. */
  sakuramon: {
    draw(x, y) {
      fill(x, y, 13, 9, 0xf6c8d4); fill(x, y, 13, 1, 0xfadce4); fill(x, y, 1, 9, 0xfadce4); fill(x + 12, y, 1, 9, 0xe0a8b8); fill(x, y + 8, 13, 1, 0xe0a8b8);
      for (const [dx, dy] of [[0, -2], [-2, 0], [2, 0], [-1, 2], [1, 2]]) dot(x + 6 + dx, y + 4 + dy, WHITE);
      for (const [dx, dy] of [[-1, -1], [1, -1], [-1, 1], [1, 1], [0, 1]]) dot(x + 6 + dx, y + 4 + dy, 0xfaf0f2);
      dot(x + 6, y + 4, 0xf2c41a);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor seen from behind in dark oak, the Hogwarts crest on it: a gold-rimmed shield quartered in scarlet, green, blue and yellow, glinting now and then. */
  pottermon: {
    draw(x, y) {
      const oak = 0x4a3020, oakLight = 0x6a4a30, oakDark = 0x2e1c12, gold = 0xd3a625, goldLight = 0xf0cf5a;
      fill(x, y, 13, 9, oak); fill(x, y, 13, 1, oakLight); fill(x, y, 1, 9, oakLight); fill(x + 12, y, 1, 9, oakDark); fill(x, y + 8, 13, 1, oakDark);
      fill(x + 3, y + 1, 7, 1, gold); fill(x + 3, y + 2, 1, 4, gold); fill(x + 9, y + 2, 1, 4, gold);
      fill(x + 4, y + 2, 2, 2, 0xa8281e); fill(x + 7, y + 2, 2, 2, 0x1a7a3a); fill(x + 4, y + 4, 2, 2, 0x2a4aa8); fill(x + 7, y + 4, 2, 2, 0xe8b830);
      fill(x + 6, y + 2, 1, 5, gold); fill(x + 4, y + 6, 5, 1, gold); dot(x + 5, y + 7, gold); dot(x + 6, y + 7, gold); dot(x + 7, y + 7, gold);
      if ((now + x * 0.19) % 14 < 0.5) dot(x + 3, y + 1, WHITE, 0.9); else dot(x + 3, y + 1, goldLight);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor seen from behind in black, a thin gold ring at its centre; every 16 s the ring's fiery letters glow round it and fade. */
  lotrmon: {
    draw(x, y) {
      const black = 0x1e1a1a, blackLight = 0x3a3434, gold = 0xe8b830, goldDark = 0xa87a18;
      fill(x, y, 13, 9, black); fill(x, y, 13, 1, blackLight); fill(x, y, 1, 9, blackLight); fill(x + 12, y, 1, 9, 0x100c0c);
      fill(x + 5, y + 2, 3, 1, gold); fill(x + 5, y + 6, 3, 1, goldDark); fill(x + 4, y + 3, 1, 3, gold); fill(x + 8, y + 3, 1, 3, goldDark); dot(x + 5, y + 3, 0xfff0a0, 0.6);
      const t = (now + x * 0.23) % 16;
      if (t < 2.5) {
        const a = Math.min(1, t, 2.5 - t);
        for (const [dx, dy] of [[1, 1], [3, 0], [6, 0], [9, 0], [11, 1], [12, 4], [11, 7], [9, 8], [6, 8], [3, 8], [1, 7], [0, 4]]) dot(x + dx, y + dy, 0xff6a20, 0.9 * a);
        if (isNight) glowLate(() => glowAt(x + 6, y + 4, 6, 4, 0xff7a30, 0.18 * a));
      }
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor seen from behind in dark green, the show's two green periodic-table tiles on it, Br and Ba, in white. */
  breakingbadmon: {
    draw(x, y) {
      const back = 0x1e3a28, green = 0x2a8a3a, greenLight = 0x4aaa5a, W = 0xf2f2ee;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, 0x2e4a38); fill(x + 12, y, 1, 9, 0x142a1c);
      fill(x + 1, y + 1, 5, 5, green); fill(x + 1, y + 1, 5, 1, greenLight); fill(x + 6, y + 3, 6, 5, green); fill(x + 6, y + 3, 6, 1, greenLight);
      // B r, B a
      dot(x + 2, y + 2, W); dot(x + 2, y + 3, W); dot(x + 2, y + 4, W); dot(x + 3, y + 3, W); dot(x + 4, y + 4, W);
      dot(x + 7, y + 4, W); dot(x + 7, y + 5, W); dot(x + 7, y + 6, W); dot(x + 8, y + 5, W); dot(x + 9, y + 6, W); dot(x + 10, y + 5, W); dot(x + 10, y + 6, W);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** The back of a beige 80s monitor with cooling slots, a sticker of the Hellfire Club's red horned skull on it. */
  strangermon: {
    draw(x, y) {
      const beige = 0xd8ccb0, beigeLight = 0xeae0c8, beigeDark = 0xa89a80, red = 0xc8281e;
      fill(x, y, 13, 9, beige); fill(x, y, 13, 1, beigeLight); fill(x, y, 1, 9, beigeLight); fill(x + 12, y, 1, 9, beigeDark); fill(x, y + 8, 13, 1, beigeDark);
      for (let k = 2; k < 11; k += 2) fill(x + k, y + 1, 1, 2, beigeDark);
      fill(x + 4, y + 4, 5, 4, 0x1a1a1e); fill(x + 5, y + 5, 3, 2, red); dot(x + 4, y + 3, red); dot(x + 8, y + 3, red); dot(x + 5, y + 5, 0x1a1a1e); dot(x + 7, y + 5, 0x1a1a1e);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor seen from behind dressed as a round wooden shield: the grey direwolf of House Stark's head on white, rimmed in iron. */
  gotmon: {
    draw(x, y) {
      const iron = 0x4a4e58, W = 0xeeeeea, grey = 0x6a6e78;
      fill(x, y, 13, 9, iron); fill(x, y, 13, 1, 0x6e727c); fill(x + 1, y + 1, 11, 7, W); fill(x + 1, y + 1, 11, 1, WHITE);
      fill(x + 4, y + 3, 5, 3, grey); dot(x + 4, y + 2, grey); dot(x + 7, y + 2, grey); dot(x + 9, y + 5, grey); dot(x + 10, y + 5, grey); fill(x + 5, y + 6, 2, 1, grey); dot(x + 7, y + 4, 0x2a2a30);
      for (const [dx, dy] of [[0, 0], [12, 0], [0, 8], [12, 8]]) dot(x + dx, y + dy, 0x8a8e96);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor seen from behind with a giant pink-frosted donut painted on it, coloured sprinkles all over the icing. */
  simpsonsmon: {
    draw(x, y) {
      const back = 0xe8dcc0, dough = 0xd89a5a, pink = 0xf28ab0;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, 0xf6ecd8); fill(x + 12, y, 1, 9, 0xc8bca0);
      disc(x + 6, y + 4, 3.8, dough); disc(x + 6, y + 4, 3.2, pink); disc(x + 6, y + 4, 1.1, back);
      for (const [dx, dy, c] of [[4, 2, 0x3a8ae8], [8, 3, 0xf2e040], [5, 6, 0x3ac85a], [8, 6, 0xf6f6f6], [3, 4, 0xf2e040]]) dot(x + dx, y + dy, c);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor seen from behind in black, the 007 logo on it in white, the 7 drawn out into a pistol. */
  bondmon: {
    draw(x, y) {
      const black = 0x16161c, W = 0xf2f2ee;
      fill(x, y, 13, 9, black); fill(x, y, 13, 1, 0x2e2e36); fill(x + 12, y, 1, 9, 0x0c0c10);
      ring(x + 2, y + 4, 1.3, W); ring(x + 5, y + 4, 1.3, W);
      fill(x + 7, y + 2, 5, 1, W); line(x + 10, y + 3, x + 8, y + 6, W); fill(x + 11, y + 3, 1, 1, W);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor seen from behind in House Atreides green, the red hawk with spread wings on it. */
  dunemon: {
    draw(x, y) {
      const green = 0x1e4a2e, greenLight = 0x2e6a42, red = 0xc8281e;
      fill(x, y, 13, 9, green); fill(x, y, 13, 1, greenLight); fill(x + 12, y, 1, 9, 0x12301e);
      fill(x + 5, y + 3, 3, 3, red); line(x + 5, y + 4, x + 1, y + 2, red); line(x + 7, y + 4, x + 11, y + 2, red); dot(x + 6, y + 2, red); dot(x + 5, y + 6, red); dot(x + 7, y + 6, red); dot(x + 6, y + 7, red);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor seen from behind in black with the Straw Hat Pirates' Jolly Roger in white: a grinning skull in a straw hat over crossbones. */
  onepiecemon: {
    draw(x, y) {
      const black = 0x16161c, W = 0xf2f2ee, straw = 0xf2d040, band = 0xd8302a;
      fill(x, y, 13, 9, black); fill(x, y, 13, 1, 0x2e2e36);
      line(x + 2, y + 2, x + 10, y + 7, W); line(x + 10, y + 2, x + 2, y + 7, W);
      fill(x + 4, y + 3, 5, 4, W); dot(x + 5, y + 4, black); dot(x + 7, y + 4, black); dot(x + 6, y + 6, black);
      fill(x + 3, y + 2, 7, 1, straw); fill(x + 4, y + 1, 5, 1, straw); fill(x + 4, y + 2, 5, 1, band);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor seen from behind in dark stone, the Elden Ring's sigil on it in gold: a ring with a vertical stroke through it; it shines softly now and then. */
  eldenringmon: {
    draw(x, y) {
      const stone = 0x2a2620, gold = 0xd8b45a, goldLight = 0xf2d888;
      fill(x, y, 13, 9, stone); fill(x, y, 13, 1, 0x3e3a32); fill(x + 12, y, 1, 9, 0x1a1814);
      const shine = (now + x * 0.21) % 12 < 1, c = shine ? goldLight : gold;
      ellipseRing(x + 6, y + 4, 4, 2.4, c); fill(x + 6, y + 1, 1, 7, c); dot(x + 4, y + 3, c, 0.6); dot(x + 8, y + 3, c, 0.6);
      if (shine && isNight) glowLate(() => glowAt(x + 6, y + 4, 5, 4, 0xf2d070, 0.2));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A boxy cream Lumon terminal seen from behind, vented, with a small blue nameplate that leaks a faint green glow. */
  sevmon: {
    draw(x, y) {
      const cream = 0xcdc6b0, creamLight = 0xe0dac6, creamDark = 0x9a9480, glow = 0x3df2a0;
      fill(x, y, 13, 9, cream); fill(x, y, 13, 1, creamLight); fill(x + 12, y, 1, 9, creamDark); fill(x, y + 8, 13, 1, creamDark); // the retro terminal back
      for (let i = 2; i < 11; i += 2) fill(x + i, y + 2, 1, 5, creamDark, 0.7); // vent slots
      fill(x + 4, y + 2, 5, 4, 0x27406a); fill(x + 5, y + 3, 1, 2, glow); fill(x + 6, y + 3, 2, 1, glow); // the nameplate with leaking glow
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 4, 3, glow, 0.14));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A chunky beige CRT monitor seen from behind, vented, a yellow sticky note stuck to it and a power LED. */
  offmon: {
    draw(x, y) {
      const beige = 0xcac3ad, beigeLight = 0xdcd6c2, beigeDark = 0x9a9276, note = 0xf2e85a, led = 0x3adb6a;
      fill(x, y, 13, 9, beige); fill(x, y, 13, 1, beigeLight); fill(x + 12, y, 1, 9, beigeDark); fill(x, y + 8, 13, 1, beigeDark); // the CRT back
      for (let i = 2; i < 11; i++) if (i % 2) fill(x + i, y + 2, 1, 4, beigeDark, 0.6); // vent slots
      fill(x + 8, y + 1, 4, 4, note); fill(x + 8, y + 1, 4, 1, 0xfff0a0); fill(x + 9, y + 3, 2, 1, 0xb8a830); // a sticky note
      dot(x + 2, y + 7, led); fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A wood-cased terminal seen from behind, a race card tucked in and a brass trim. */
  peakymon: {
    draw(x, y) {
      const wood = 0x4a3526, woodLight = 0x6a4e38, brass = 0xc8a04a, card = 0xe8dcc0, ink = 0x5a3a1a;
      fill(x, y, 13, 9, wood); fill(x, y, 13, 1, woodLight); fill(x + 12, y, 1, 9, 0x2e2016); fill(x, y + 8, 13, 1, 0x2e2016); // the wooden back
      fill(x + 1, y + 1, 11, 7, 0x3a2a1c); fill(x + 3, y + 2, 7, 5, card); for (let i = 0; i < 4; i++) fill(x + 4, y + 3 + i, 4 - (i % 2), 1, ink, 0.7); // a race card
      fill(x, y, 13, 1, brass, 0.5); dot(x + 1, y, brass); // a brass trim
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A mid-century teak console TV seen from behind, with a speaker-cloth panel and chrome trim. */
  madmmon: {
    draw(x, y) {
      const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, cloth = 0xcfc090, chrome = 0xc0c4c8;
      fill(x, y, 13, 9, wood); fill(x, y, 13, 1, woodLight); fill(x + 12, y, 1, 9, woodDark); fill(x, y + 8, 13, 1, woodDark); // the teak back
      fill(x + 2, y + 2, 9, 5, cloth); for (let i = 0; i < 9; i += 2) fill(x + 2 + i, y + 2, 1, 5, 0xb8a878, 0.6); // speaker cloth
      fill(x + 1, y + 1, 1, 7, chrome, 0.5); fill(x + 3, y, 7, 1, chrome, 0.4); // chrome trim
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A black terminal seen from behind, the three game shapes glowing pink on its case. */
  squidmon: {
    draw(x, y) {
      const black = 0x1a1a24, blackLight = 0x2a2a34, pink = 0xe84a7a;
      fill(x, y, 13, 9, black); fill(x, y, 13, 1, blackLight); fill(x + 12, y, 1, 9, 0x101018);
      ring(x + 3, y + 4, 1.6, pink); line(x + 6, y + 2, x + 5, y + 6, pink); line(x + 6, y + 2, x + 8, y + 6, pink); line(x + 5, y + 6, x + 8, y + 6, pink); fill(x + 9, y + 3, 3, 3, pink); // the three shapes
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 4, 3, pink, 0.12));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A steel terminal seen from behind, a red star emblem on its case. */
  boysmon: {
    draw(x, y) {
      const steel = 0x3a4250, steelLight = 0x5a6270, red = 0xd21f2a, white = 0xf0f0ee;
      fill(x, y, 13, 9, steel); fill(x, y, 13, 1, steelLight); fill(x + 12, y, 1, 9, 0x2a303a); fill(x, y + 8, 13, 1, 0x2a303a);
      disc(x + 6, y + 4, 3, red); for (let k = 0; k < 5; k++) { const a = k / 5 * Math.PI * 2 - Math.PI / 2; dot(x + 6 + Math.round(Math.cos(a) * 2), y + 4 + Math.round(Math.sin(a) * 2), white); } dot(x + 6, y + 4, white); // the star emblem
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 4, 3, red, 0.12));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A green-phosphor CRT terminal seen from behind, its glow leaking round the case, a power light. */
  falloutmon: {
    draw(x, y) {
      const casing = 0x6a6e64, caseLight = 0x8a8e84, caseDark = 0x4a4e44, screen = 0x0a2010, green = 0x3ae87a;
      fill(x, y, 13, 9, casing); fill(x, y, 13, 1, caseLight); fill(x + 12, y, 1, 9, caseDark); fill(x, y + 8, 13, 1, caseDark); // the terminal back
      fill(x + 2, y + 2, 9, 5, screen); for (let r = 0; r < 3; r++) fill(x + 3, y + 3 + r, 3 + (r * 2), 1, green, 0.8); // green text
      dot(x + 2, y + 7, green); // a power light
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 4, 3, green, 0.14));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A terminal seen from behind with a green portal-swirl decal and a turning spark. */
  rickmon: {
    draw(x, y) {
      const casing = 0x3a4248, caseLight = 0x5a626a, green = 0x3ae84a, greenL = 0x7af06a;
      fill(x, y, 13, 9, casing); fill(x, y, 13, 1, caseLight); fill(x + 12, y, 1, 9, 0x2a3036); fill(x, y + 8, 13, 1, 0x2a3036);
      for (let r = 3; r > 0; r--) ring(x + 6, y + 4, r, r > 1 ? green : greenL, 0.9); // a portal-swirl decal
      const a = now * 2; dot(x + 6 + Math.round(Math.cos(a) * 2), y + 4 + Math.round(Math.sin(a) * 1.5), greenL); // a turning spark
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 4, 3, green, 0.12));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A glossy bezel-less screen seen from behind, a glowing cyan status ring on its case. */
  mirrormon: {
    draw(x, y) {
      const white = 0xeef0f2, whiteLight = 0xf8fafc, grey = 0xc8cdd4, cyan = 0x5adce8;
      fill(x, y, 13, 9, white); fill(x, y, 13, 1, whiteLight); fill(x + 12, y, 1, 9, grey); fill(x, y + 8, 13, 1, grey); // the glossy back
      ring(x + 6, y + 4, 2.4, cyan, 0.9); disc(x + 6, y + 4, 1, cyan); // a status ring
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 3, 2, cyan, 0.12));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A clean terminal seen from behind with a cyan vitruvian-figure mark. */
  westmon: {
    draw(x, y) {
      const casing = 0xdfe0de, caseLight = 0xf0f0ee, caseDark = 0xc0c4c2, cyan = 0x8ad0d8;
      fill(x, y, 13, 9, casing); fill(x, y, 13, 1, caseLight); fill(x + 12, y, 1, 9, caseDark); fill(x, y + 8, 13, 1, caseDark); // the clean back
      ring(x + 6, y + 4, 2.2, cyan, 0.9); fill(x + 6, y + 2, 1, 5, cyan, 0.8); fill(x + 4, y + 4, 5, 1, cyan, 0.8); // a figure mark
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 3, 2, cyan, 0.12));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** Time-circuits display seen from behind: three rows of glowing digits -- red, green, amber -- flickering as if ready to jump. */
  bttftimecircuits: {
    draw(x, y) {
      const back = 0x262a33, backL = 0x363b46, red = 0xff4a3a, green = 0x6ef06a, amber = 0xf2b24a;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, backL); fill(x, y + 8, 13, 1, 0x161920);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      const rows = [red, green, amber];
      for (let r = 0; r < 3; r++) { const flick = Math.floor(now * 3 + r) % 5 === 0 ? 0.5 : 1; for (let c = 0; c < 4; c++) fill(x + 2 + c * 3, y + 1 + r * 3, 2, 1, rows[r], flick); }
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 7, 5, 0x88ff88, 0.18));
    },
  },
  /** The park control system seen from behind: a green-on-black schematic with a status row that blinks. */
  jpmon: {
    draw(x, y) {
      const back = 0x14201a, grid = 0x2a5a3a, linec = 0x6ef06a, amber = 0xe8c838, red = 0xff4a3a;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, 0x244a34); fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      for (let gx = x + 1; gx < x + 12; gx += 3) fill(gx, y + 1, 1, 7, grid, 0.5);
      line(x + 2, y + 6, x + 6, y + 3, linec); line(x + 6, y + 3, x + 10, y + 5, linec);
      for (let k = 0; k < 3; k++) dot(x + 2 + k * 3, y + 7, (Math.floor(now * 2) + k) % 3 === 0 ? red : amber);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 7, 5, linec, 0.16));
    },
  },
  /** The Continental switchboard seen from behind: a dark reservations screen with gold rows of names that update. */
  wickmon: {
    draw(x, y) {
      const back = 0x1a1a1e, gold = 0xd6b25e, goldL = 0xf2d888, green = 0x3ad0b0;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, 0x2a2a30); fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      for (let r = 0; r < 4; r++) { const len = 4 + (hash(r, Math.floor(now / 3)) % 6); fill(x + 2, y + 1 + r * 2, len, 1, (r === (Math.floor(now) % 4)) ? goldL : gold, 0.85); }
      dot(x + 11, y + 1, green);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 7, 5, gold, 0.14));
    },
  },
  /** A mission-briefing screen seen from behind: a blue schematic globe with a sweeping target reticle. */
  kingsmanmon: {
    draw(x, y) {
      const back = 0x0e1828, blue = 0x4aa0e0, blueL = 0x9fd0f0, gold = 0xc8a24a;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, 0x1a2e46); fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      disc(x + 6, y + 4, 3, 0x163450); ring(x + 6, y + 4, 3, blue, 0.8); fill(x + 3, y + 4, 7, 1, blue, 0.5);
      const a = now * 1.2; dot(x + 6 + Math.round(Math.cos(a) * 3), y + 4 + Math.round(Math.sin(a) * 2), gold); // the reticle
      dot(x + 2, y + 1, blueL); dot(x + 10, y + 7, blueL);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 7, 5, blue, 0.16));
    },
  },
  /** A recipe screen seen from behind: a cheerful candy-coloured display with a turning swirl and bouncing sweets. */
  wonkamon: {
    draw(x, y) {
      const back = 0x2a1e3a, pink = 0xf06aa8, mint = 0x5ad0a0, gold = 0xe8c838;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, 0x3e2e54); fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      const a = now * 2; for (let k = 0; k < 6; k++) { const t = a + k * 1.0; dot(x + 6 + Math.round(Math.cos(t) * (k * 0.3)), y + 4 + Math.round(Math.sin(t) * (k * 0.25)), k % 2 ? pink : mint); }
      const b = Math.floor(now * 3) % 3; dot(x + 2, y + 6 - b, gold); dot(x + 10, y + 2 + b, pink);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 7, 5, pink, 0.14));
    },
  },
  /** A pastel departures board seen from behind: neat cream rows on salmon that flip over now and then. */
  wesmon: {
    draw(x, y) {
      const back = 0x7a2a3a, salmon = 0xe89878, cream = 0xf2ead2, mustard = 0xd8a848;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, 0x9a3a4a); fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      const flip = Math.floor(now) % 4;
      for (let r = 0; r < 4; r++) { fill(x + 1, y + 1 + r * 2, 11, 1, r === flip ? mustard : salmon); fill(x + 2, y + 1 + r * 2, 3 + (hash(r, 1) % 5), 1, cream, 0.8); }
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 7, 5, salmon, 0.12));
    },
  },
  /** A tracking screen seen from behind: a green radar sweep with blips and a scrolling data strip. */
  mibmon: {
    draw(x, y) {
      const back = 0x0e1e14, green = 0x5ad86a, greenD = 0x2a6a3a, amber = 0xe8a830;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, 0x1a3a26); fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      ring(x + 6, y + 4, 3, greenD, 0.7); ring(x + 6, y + 4, 1.5, greenD, 0.5);
      const a = now * 2; line(x + 6, y + 4, x + 6 + Math.round(Math.cos(a) * 3), y + 4 + Math.round(Math.sin(a) * 3), green);
      dot(x + 8, y + 3, green); dot(x + 4, y + 6, amber);
      const sx = x + 1 + Math.floor(now * 3) % 10; dot(sx, y + 7, green);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 7, 5, green, 0.16));
    },
  },
  /** A grid-code screen seen from behind: cascading cyan code over a dark grid, a bright cursor blinking. */
  tronmon: {
    draw(x, y) {
      const back = 0x0a1016, grid = 0x163848, cyan = 0x3ad0e8, cyanL = 0x9ff0ff;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, 0x1a3040); fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      for (let gx = x + 1; gx < x + 12; gx += 3) fill(gx, y + 1, 1, 7, grid, 0.5);
      for (let c = 0; c < 4; c++) { const col = ((now * 6 + c * 5) % 8); fill(x + 2 + c * 3, y + 1 + Math.floor(col), 1, 2, cyan, 0.85); }
      if (Math.floor(now * 2) % 2) dot(x + 10, y + 6, cyanL);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 7, 5, cyan, 0.16));
    },
  },
  /** A power-level readout seen from behind: a green bar climbing and spiking past big numbers -- it's over 9000. */
  dbzmon: {
    draw(x, y) {
      const back = 0x0e2016, green = 0x5ad04a, greenL = 0x9ff0a8, amber = 0xf2c84a, red = 0xf24a3a;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, 0x1a3a26); fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      const lvl = (Math.sin(now * 1.5) * 0.5 + 0.5), over = lvl > 0.85;
      for (let c = 0; c < 10; c++) { const h = Math.max(0, Math.round((lvl * 10 - c) )); if (h > 0) fill(x + 1 + c, y + 6 - Math.min(5, h), 1, Math.min(5, h), c > 7 ? red : (c > 4 ? amber : green)); }
      if (over) { dot(x + 10, y + 1, red); dot(x + 11, y + 1, greenL); }
      fill(x + 1, y + 7, 11, 1, green, 0.5);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 7, 5, green, 0.14));
    },
  },
  /** A security feed seen from behind: a four-camera grid, crew dots moving through, one view flickering to static. */
  amongusmon: {
    draw(x, y) {
      const back = 0x0c1420, grid = 0x24303c, feed = 0x163044, dot_ = 0xf2d84a, line_ = 0x3aa0c0;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, 0x1a2a3a); fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      for (let r = 0; r < 2; r++) for (let c = 0; c < 2; c++) { const fx = x + 1 + c * 6, fy = y + 1 + r * 4; fill(fx, fy, 5, 3, feed); const stat = (Math.floor(now) % 4) === (r * 2 + c); if (stat) { for (let k = 0; k < 4; k++) dot(fx + (hash(k, Math.floor(now * 6)) % 5), fy + (k % 3), grid); } else dot(fx + 1 + (Math.floor(now * 2) % 3), fy + 1, dot_); }
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 7, 5, line_, 0.1));
    },
  },
  /** A matte-black monitor seen from behind, ringed with a neon tube that runs pink down one edge and cyan down the other and a row of katakana-like glyphs across the top that flicker on; it glows at night. */
  tokyomon: {
    draw(x, y) {
      const body = 0x1a1620, pink = 0xff4ab8, cyan = 0x3ae8ff;
      fill(x, y, 13, 9, body); fill(x, y, 1, 9, pink); fill(x + 12, y, 1, 9, cyan); fill(x, y, 13, 1, 0x2a2434); fill(x, y + 8, 13, 1, 0x100c16);
      // A ticker of little glyphs that blink
      for (let k = 0; k < 4; k++) { const on = (k + Math.floor(now * 2)) % 3 !== 0; dot(x + 3 + k * 2, y + 3, on ? 0x5af0ff : 0x204048); fill(x + 3 + k * 2, y + 5, 1, 2, on ? pink : 0x401830, on ? 1 : 0.6); }
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => { glowAt(x, y + 4, 3, 5, pink, 0.22); glowAt(x + 12, y + 4, 3, 5, cyan, 0.22); });
    },
  },
  /** A gold-cased monitor seen from behind, a painted Eye of Horus (wedjat) on the back in blue and black with a gold brow; the eye gleams now and then. */
  egyptmon: {
    draw(x, y) {
      const gold = 0xd8b24e, goldLight = 0xf2d888, goldDark = 0x9a7a2a, blue = 0x1f4e8a;
      fill(x, y, 13, 9, gold); fill(x, y, 13, 1, goldLight); fill(x, y, 1, 9, goldLight); fill(x + 12, y, 1, 9, goldDark); fill(x, y + 8, 13, 1, goldDark);
      // the wedjat eye
      fill(x + 4, y + 3, 6, 1, 0x241a12); fill(x + 4, y + 4, 6, 2, 0xeae2cc); fill(x + 5, y + 4, 3, 2, blue);
      const gleam = (now + x * 0.3) % 5 < 0.6; dot(x + 6, y + 4, gleam ? 0x5af0ff : 0x3a6ab0);
      fill(x + 4, y + 6, 4, 1, 0x241a12); dot(x + 4, y + 7, 0x241a12); // the tail and teardrop
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight && gleam) glowLate(() => glowAt(x + 6, y + 4, 3, 2, 0x5af0ff, 0.2));
    },
  },
  /** A monitor in a dark oak case seen from behind, carved with an interlaced knotwork border and a little dragon head at the top; iron corner-bands. */
  vikingmon: {
    draw(x, y) {
      const wood = 0x6a4a2e, woodLight = 0x8a6440, woodDark = 0x3a2818, iron = 0x4a4e56;
      fill(x, y, 13, 9, wood); fill(x, y, 13, 1, woodLight); fill(x + 12, y, 1, 9, woodDark); fill(x, y + 8, 13, 1, woodDark);
      // knotwork border
      for (let k = 2; k < 11; k += 2) { dot(x + k, y + 2, woodDark); dot(x + k + 1, y + 3, woodLight); dot(x + k, y + 6, woodLight); }
      fill(x, y, 1, 2, iron); fill(x + 12, y + 7, 1, 2, iron); // iron bands
      fill(x + 5, y - 2, 3, 2, woodDark); dot(x + 5, y - 2, woodLight); dot(x + 7, y - 1, PALETTE.eye); // the dragon head
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A carved stone monitor seen from behind with a little sun-stone face on the back: a central disc with four rays and a ring of glyph squares; it glows gold now and then. */
  aztecmon: {
    draw(x, y) {
      const stone = 0x7a7268, stoneLight = 0x968c7e, stoneDark = 0x55504a, gold = 0xd8b24e, jade = 0x3ab8a0;
      fill(x, y, 13, 9, stone); fill(x, y, 13, 1, stoneLight); fill(x + 12, y, 1, 9, stoneDark); fill(x, y + 8, 13, 1, stoneDark);
      const lit = (now + x * 0.3) % 6 < 0.8;
      ring(x + 6, y + 4, 3, lit ? gold : 0x8a7f6e, 1); disc(x + 6, y + 4, 1.4, lit ? 0xf2d888 : jade);
      for (const [dx, dy] of [[0, -3], [0, 3], [-3, 0], [3, 0]]) dot(x + 6 + dx, y + 4 + dy, gold); // the four rays
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight && lit) glowLate(() => glowAt(x + 6, y + 4, 4, 3, gold, 0.18));
    },
  },
  /** A dark monitor seen from behind with a little radar-style sky sweep on the back: a green circular grid with a sweeping line that lights blips as it passes; it glows at night. */
  obsmon: {
    draw(x, y) {
      const body = 0x23283a, brass = 0xc8a050, green = 0x3ae87a, dark = 0x12331e;
      fill(x, y, 13, 9, body); fill(x, y, 13, 1, 0x3a4052); fill(x, y, 1, 9, 0x3a4052); fill(x + 12, y, 1, 9, 0x12161f); fill(x, y + 8, 13, 1, 0x12161f);
      disc(x + 6, y + 4, 3.4, dark); ring(x + 6, y + 4, 3.4, green, 0.6); ring(x + 6, y + 4, 2, green, 0.3); fill(x + 6, y + 1, 1, 7, green, 0.25); fill(x + 3, y + 4, 7, 1, green, 0.25);
      const a = now * 1.6, ex = x + 6 + Math.round(Math.cos(a) * 3), ey = y + 4 + Math.round(Math.sin(a) * 3); line(x + 6, y + 4, ex, ey, green, 0.9);
      const b = Math.floor(now) % 3; dot(x + 6 + [2, -1, 1][b], y + 4 + [-1, 2, 1][b], green);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 4, 3, green, 0.16));
    },
  },
  /** A monitor in a carved tiki-wood case seen from behind: dark wood with a stylised tiki face — brow, round eyes and a wide mouth — and a little frond on top. */
  tikimon: {
    draw(x, y) {
      const wood = 0x6a4a2e, woodLight = 0x8a6440, woodDark = 0x3a2818, green = 0x4f9a5a;
      fill(x, y, 13, 9, wood); fill(x, y, 13, 1, woodLight); fill(x + 12, y, 1, 9, woodDark); fill(x, y + 8, 13, 1, woodDark);
      fill(x + 2, y + 2, 9, 1, woodDark); // the heavy brow
      fill(x + 3, y + 3, 2, 2, 0xe0cc94); fill(x + 8, y + 3, 2, 2, 0xe0cc94); dot(x + 3, y + 3, woodDark); dot(x + 9, y + 3, woodDark); // round eyes
      fill(x + 3, y + 6, 7, 1, woodDark); for (let k = 0; k < 3; k++) dot(x + 4 + k * 2, y + 6, 0xe0cc94); // wide toothy mouth
      fill(x + 5, y - 2, 3, 2, green); dot(x + 4, y - 1, green); dot(x + 8, y - 1, green); // a frond
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor in a polished chrome case seen from behind, red enamel side panels and a little tabletop-jukebox song-selector dome on top with a blinking light. */
  dinermon: {
    draw(x, y) {
      const chrome = 0xc8ccd2, chromeLight = 0xe8ecf0, chromeDark = 0x8a8e96, red = 0xe03a4a;
      fill(x, y, 13, 9, chrome); fill(x, y, 13, 1, chromeLight); fill(x + 12, y, 1, 9, chromeDark); fill(x, y + 8, 13, 1, chromeDark);
      fill(x, y, 2, 9, red); fill(x + 11, y, 2, 9, red); fill(x, y, 1, 9, 0xf05a68); // red side panels
      fill(x + 4, y - 2, 5, 2, chrome); fill(x + 4, y - 2, 5, 1, chromeLight); // the selector dome
      const on = (now + x * 0.3) % 2 < 1; dot(x + 6, y - 1, on ? red : 0x8a2028); fill(x + 4, y + 3, 5, 1, 0x3a3a42, 0.5);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight && on) glowLate(() => glowAt(x + 6, y - 1, 2, 2, red, 0.2));
    },
  },
  /** A monitor in a dark oak case seen from behind, a brass scales-of-justice emblem on the back and a brass nameplate strip; the brass gleams. */
  courtmon: {
    draw(x, y) {
      const wood = 0x6a4a2e, woodLight = 0x8a6440, woodDark = 0x4a3220, brass = 0xc8a050, brassLight = 0xe8c878;
      fill(x, y, 13, 9, wood); fill(x, y, 13, 1, woodLight); fill(x + 12, y, 1, 9, woodDark); fill(x, y + 8, 13, 1, woodDark);
      // the scales emblem
      fill(x + 6, y + 2, 1, 4, brass); fill(x + 4, y + 3, 5, 1, brass); fill(x + 4, y + 4, 1, 1, brassLight); fill(x + 8, y + 4, 1, 1, brassLight); dot(x + 6, y + 2, brassLight);
      fill(x + 3, y + 6, 7, 1, brass, 0.8); // nameplate strip
      const gleam = (now + x * 0.3) % 6 < 0.5; if (gleam) dot(x + 6, y + 3, WHITE, 0.7);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor built into a little canvas stretcher-frame seen from behind: raw wood bars with canvas stretched over, a few paint-smudges and a dab of colour in each corner. */
  artmon: {
    draw(x, y) {
      const wood = 0xc8a060, woodLight = 0xe0bc78, woodDark = 0x9a7840, canvas = 0xe8e0cc;
      fill(x, y, 13, 9, canvas); fill(x, y, 13, 1, 0xf0ece0); fill(x, y + 8, 13, 1, 0xd0c8b0);
      // the stretcher bars framing it
      fill(x, y, 2, 9, wood); fill(x + 11, y, 2, 9, wood); fill(x, y, 13, 2, woodLight); fill(x, y + 7, 13, 2, woodDark);
      // paint smudges + corner dabs
      for (const [dx, dy, c] of [[4, 4, 0xe8404a], [9, 3, 0x4a8ad8], [6, 6, 0xf2c040], [3, 6, 0x4ac86a]]) dot(x + dx, y + dy, c, 0.8);
      fill(x + 5, y + 4, 2, 1, 0xd8a0a0, 0.5);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor in a round brass porthole case seen from behind, deep-blue water inside with a little fish swimming across and rising bubbles; it glows at night. */
  aquariummon: {
    draw(x, y) {
      const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, water = 0x14406a, fish = 0xf0824a;
      fill(x, y, 13, 9, 0x1a2a3a); ring(x + 6, y + 4, 5, brass, 1); disc(x + 6, y + 4, 4, water);
      for (let k = 0; k < 6; k++) dot(x + 1 + k * 2, y, (k % 2) ? brassLight : brassDark); // bolts round the rim
      const fx = x + 3 + Math.floor((now * 2) % 7); fill(fx, y + 4, 2, 1, fish); dot(fx - 1, y + 4, fish); dot(fx + 2, y + 3, fish); // the fish
      const bub = (y + 2) - Math.floor(now * 3) % 4; dot(x + 8, bub, 0xbfe4f0, 0.8);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      if (isNight) glowLate(() => glowAt(x + 6, y + 4, 4, 4, 0x3a9ad8, 0.2));
    },
  },
  /** A monitor back in big-top cream with red stripes and a little gold star badge that twinkles now and then. */
  circusmon: {
    draw(x, y) {
      const red = 0xd8402a, cream = 0xf2e8d8, gold = 0xe8b84a;
      fill(x, y, 13, 9, cream); fill(x, y, 13, 1, 0xfff4e4); fill(x + 12, y, 1, 9, 0xd8ccb8);
      for (let k = 1; k < 13; k += 3) fill(x + k, y + 1, 1, 7, red);
      const shine = (now + x * 0.2) % 10 < 1, c = shine ? 0xfff0a0 : gold;
      dot(x + 6, y + 2, c); dot(x + 6, y + 5, c); dot(x + 4, y + 4, c); dot(x + 8, y + 4, c); dot(x + 6, y + 4, c);
      if (shine && isNight) glowLate(() => glowAt(x + 6, y + 4, 5, 4, 0xf2d070, 0.18));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor back in dark wood with a brass surround and a bone-white ammonite spiral that shines now and then. */
  museummon: {
    draw(x, y) {
      const brass = 0xc8a44e, brassL = 0xe8c878, dark = 0x2a2620, bone = 0xe8dcc0;
      fill(x, y, 13, 9, dark); fill(x, y, 13, 1, brass); fill(x + 12, y, 1, 9, 0x1a1814);
      const cx = x + 6, cy = y + 4, shine = (now + x * 0.2) % 11 < 1;
      for (let k = 0; k < 14; k++) { const a = k * 0.6, r = 0.5 + k * 0.22; dot(cx + Math.round(Math.cos(a) * r), cy + Math.round(Math.sin(a) * r), shine ? brassL : bone); }
      if (shine && isNight) glowLate(() => glowAt(cx, cy, 4, 3, 0xe8c878, 0.16));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor back in frosted blue with a white snowflake that brightens now and then; frost glints in the corners. */
  northpolemon: {
    draw(x, y) {
      const frost = 0xcfe0ee, frostD = 0x9ab4ca, flake = 0xf2f6fb, cx = x + 6, cy = y + 4;
      fill(x, y, 13, 9, frost); fill(x, y, 13, 1, 0xe4eef6); fill(x + 12, y, 1, 9, frostD);
      const tw = (now + x * 0.2) % 8 < 1;
      for (let a = 0; a < 6; a++) { const an = a / 6 * Math.PI * 2; for (let r = 1; r <= 3; r++) dot(cx + Math.round(Math.cos(an) * r), cy + Math.round(Math.sin(an) * r), tw ? WHITE : flake); }
      dot(cx, cy, flake); dot(x + 1, y + 1, WHITE, 0.6); dot(x + 11, y + 7, WHITE, 0.4);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor back as a camera body with a flip-out screen framing a shot, a blinking red REC dot and a battery bar. */
  soundstagemon: {
    draw(x, y) {
      const body = 0x2a2a2e, bodyL = 0x4a4a52, screen = 0x3a5a4a, red = 0xd83030;
      fill(x, y, 13, 9, body); fill(x, y, 13, 1, bodyL); fill(x + 12, y, 1, 9, 0x1a1a1e);
      fill(x + 1, y + 1, 8, 6, screen); fill(x + 1, y + 1, 8, 1, 0x5a7a6a);
      dot(x + 2, y + 2, WHITE, 0.5); dot(x + 7, y + 2, WHITE, 0.5); dot(x + 2, y + 5, WHITE, 0.5); dot(x + 7, y + 5, WHITE, 0.5);
      const rec = Math.floor(now * 1.5) % 2; dot(x + 10, y + 2, rec ? red : 0x5a2020); fill(x + 10, y + 5, 2, 1, 0x3aa05a);
      if (rec && isNight) glowLate(() => glowAt(x + 10, y + 2, 2, 2, red, 0.14));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor back as a radiant eye on a deep-indigo panel: golden rays round an iris that drifts and blinks; it glows at night. */
  cultmon: {
    draw(x, y) {
      const back = 0x3a3a5a, gold = 0xe0b24a, goldL = 0xf2d070, white = 0xeceae2, iris = 0xc08a5e;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, 0x4a4a6a); fill(x + 12, y, 1, 9, 0x2a2a44);
      for (let a = 0; a < 8; a++) { const an = a / 8 * Math.PI * 2; dot(x + 6 + Math.round(Math.cos(an) * 5), y + 4 + Math.round(Math.sin(an) * 4), gold); }
      const blink = (now + x * 0.2) % 6 < 0.4;
      if (blink) fill(x + 3, y + 4, 7, 1, gold); else { fill(x + 3, y + 3, 7, 3, white); const look = Math.round(Math.sin(now * 0.4) * 2); disc(x + 6 + look, y + 4, 1.6, iris); dot(x + 6 + look, y + 4, back); }
      if (!blink && isNight) glowLate(() => glowAt(x + 6, y + 4, 4, 3, goldL, 0.14));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A gaming-monitor back with a glowing team logo and an RGB underglow strip that cycles; glows at night. */
  esportsmon: {
    draw(x, y) {
      const body = 0x1a1a22, bodyL = 0x2a2a36, rgb = (k) => [0x2ad8e8, 0x8a3ae8, 0xe82ad8, 0xf03a8a, 0x9ae82a, 0x3a6aff][((k % 6) + 6) % 6];
      fill(x, y, 13, 9, body); fill(x, y, 13, 1, bodyL); fill(x + 12, y, 1, 9, 0x101016);
      const base = Math.floor(now * 2), c = rgb(base);
      fill(x + 5, y + 3, 3, 1, c); fill(x + 6, y + 2, 1, 3, c); dot(x + 6, y + 3, WHITE, 0.7);
      for (let k = 0; k < 13; k++) dot(x + k, y + 8, rgb(base + k), 0.9);
      if (isNight) glowLate(() => glowAt(x + 6, y + 9, 7, 2, rgb(base), 0.16));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor back as a rough stone slab carved with an aurochs glyph and set with a glowing blue crystal that pulses; glows at night. */
  cavemanmon: {
    draw(x, y) {
      const stone = 0x5a4e44, stoneL = 0x6e6056, stoneD = 0x3a322a, glyph = 0xa0472a, crys = 0x8ab8c8, crysL = 0xc8e8f0;
      fill(x, y, 13, 9, stone); fill(x, y, 13, 1, stoneL); fill(x + 12, y, 1, 9, stoneD);
      fill(x + 2, y + 3, 4, 2, glyph); dot(x + 1, y + 3, glyph); dot(x + 2, y + 2, glyph); dot(x + 5, y + 2, glyph); fill(x + 2, y + 5, 1, 1, glyph); fill(x + 5, y + 5, 1, 1, glyph);
      const pulse = (Math.sin(now * 1.5 + x) + 1) / 2;
      fill(x + 9, y + 3, 2, 3, crys); dot(x + 9, y + 3, crysL); dot(x + 10, y + 4, WHITE, 0.3 + pulse * 0.4);
      if (isNight) glowLate(() => glowAt(x + 10, y + 4, 3, 3, crys, 0.1 + pulse * 0.1));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A monitor back in dark wood with the gilded eye-in-triangle emblem that shines now and then; glows at night. */
  lodgemon: {
    draw(x, y) {
      const wood = 0x2a1810, woodL = 0x3a2418, gold = 0xc8a44e, goldL = 0xe8c878, pale = 0xd8cbb0;
      fill(x, y, 13, 9, wood); fill(x, y, 13, 1, woodL); fill(x + 12, y, 1, 9, 0x1a0e08);
      const shine = (now + x * 0.2) % 11 < 1, c = shine ? goldL : gold;
      line(x + 2, y + 7, x + 6, y + 1, c); line(x + 6, y + 1, x + 10, y + 7, c); line(x + 2, y + 7, x + 10, y + 7, c);
      dot(x + 6, y + 4, pale); dot(x + 6, y + 4, shine ? 0x1a1210 : 0x3a2418);
      if (shine && isNight) glowLate(() => glowAt(x + 6, y + 4, 4, 3, gold, 0.16));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A rugged military-terminal back with a small green CRT scrolling text, a blinking cursor and a signal bar; glows green at night. */
  preppermon: {
    draw(x, y) {
      const body = 0x3a4038, bodyL = 0x4e5448, bodyD = 0x242a24, screen = 0x0a1a0e, green = 0x3aa05a, greenL = 0x6ad08a;
      fill(x, y, 13, 9, body); fill(x, y, 13, 1, bodyL); fill(x + 12, y, 1, 9, bodyD);
      fill(x + 1, y + 1, 9, 6, screen);
      for (let r = 0; r < 3; r++) fill(x + 2, y + 2 + r, 2 + (hash(r, Math.floor(now * 2)) % 6), 1, green, 0.8);
      dot(x + 8, y + 5, Math.floor(now * 2) % 2 ? greenL : screen);
      for (let b = 0; b < 3; b++) fill(x + 10, y + 6 - b, 1, 1 + b, b <= (Math.floor(now) % 3) ? green : 0x2a3a2e);
      if (isNight) glowLate(() => glowAt(x + 5, y + 4, 5, 4, green, 0.12));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** Secure terminal. An old government CRT from behind: an olive-beige case with cooling vents, a red SECURE strip, a green power LED pulsing and a cable coiling off the stand. */
  paradiseterminal: {
    draw(x, y) {
      const case_ = 0x8a8470, caseLight = 0xa8a088, caseDark = 0x5e5a48, vent = 0x4a4636, red = 0xc62828, green = 0x5ae88a;
      fill(x, y, 13, 9, case_); fill(x, y, 13, 1, caseLight); fill(x, y, 1, 9, caseLight); fill(x + 12, y, 1, 9, caseDark); fill(x, y + 8, 13, 1, caseDark);
      for (let r = 1; r < 8; r += 2) fill(x + 2, y + r, 9, 1, vent, 0.7);
      fill(x + 2, y + 1, 5, 1, red); dot(x + 10, y + 7, green, 0.6 + 0.4 * Math.sin(now * 3));
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
      fill(x + 1, y + 9, 1, 2, caseDark); dot(x + 1, y + 10, caseDark);
    },
  },
  /** Console screen. A bridge console from behind: a dark case with an amber LCARS side strip, faint blue readout lines and a green status LED. */
  startrekconsole: {
    draw(x, y) {
      const case_ = 0x2a3142, caseLight = 0x3a4458, caseDark = 0x1a1f28, amber = 0xd4a02a, blue = 0x4a8ec8;
      fill(x, y, 13, 9, case_); fill(x, y, 13, 1, caseLight); fill(x + 12, y, 1, 9, caseDark); fill(x, y + 8, 13, 1, caseDark);
      fill(x + 1, y + 1, 3, 7, amber, 0.8); fill(x + 1, y + 1, 1, 7, caseDark);
      for (let r = 0; r < 3; r++) fill(x + 5, y + 2 + r * 2, 6, 1, blue, 0.5);
      dot(x + 10, y + 7, 0x6abf8a, 0.7);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** Wood-cased screen. A modern monitor in a walnut case from behind, a brass-framed vent, a maker's plate and a green power light. */
  gentlemenwoodcase: {
    draw(x, y) {
      const wood = 0x5a3b25, woodLight = 0x7a5232, woodDark = 0x3a2616, brass = 0xc8a050, green = 0x6abf8a;
      fill(x, y, 13, 9, wood); fill(x, y, 13, 1, woodLight); fill(x + 12, y, 1, 9, woodDark); fill(x, y + 8, 13, 1, woodDark);
      fill(x + 2, y + 2, 9, 4, woodDark); for (let r = 0; r < 2; r++) fill(x + 3, y + 3 + r, 7, 1, 0x2a1d12);
      fill(x + 4, y + 6, 5, 1, brass, 0.7);
      dot(x + 10, y + 7, green, 0.6);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
  /** A black monitor seen from behind: a slim graphite back with the mark in white, the screen's white light spilling round its edges at night. */
  grokmon: {
    draw(x, y) {
      const back = 0x1a1b20, edge = 0x3a3d47, dark = 0x0e0f12;
      if (isNight) glowLate(() => { glowAt(x + 6, y + 4, 9, 6, 0xdfe8ff, 0.1); fill(x - 1, y, 1, 9, 0xdfe8ff, 0.35); fill(x + 13, y, 1, 9, 0xdfe8ff, 0.35); });
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, edge); fill(x, y, 1, 9, edge); fill(x + 12, y, 1, 9, dark); fill(x, y + 8, 13, 1, dark);
      grokMark(x + 6, y + 4, 0xf4f1ea, 0.85);
      fill(x + 5, y + 9, 3, 2, 0x2a2c33); fill(x + 4, y + 10, 5, 1, 0x3a3d47);
    },
  },
  /** A monitor with a status light, seen from behind: a black back with the mark, a stub antenna on its corner whose ion-blue light blinks as it reports in. */
  grokbotmon: {
    draw(x, y) {
      const back = 0x1a1b20, edge = 0x34363e, dark = 0x0e0f12, ion = 0x8ab4ff;
      fill(x, y, 13, 9, back); fill(x, y, 13, 1, edge); fill(x, y, 1, 9, edge); fill(x + 12, y, 1, 9, dark); fill(x, y + 8, 13, 1, dark);
      grokMark(x + 6, y + 4, 0xc8ccd4, 0.8);
      fill(x + 11, y - 3, 1, 3, 0x6e727c);
      const on = (now + x * 0.11) % 3 < 0.4, led = () => dot(x + 11, y - 4, on ? WHITE : ion, on ? 1 : 0.6);
      led();
      if (isNight) glowLate(led);
      fill(x + 5, y + 9, 3, 2, 0x2a2c33);
    },
  },
  /** A geode-backed monitor: from behind, its back is half a geode, rough grey-brown rind with pits in it, violet crystal teeth showing round its rim. */
  geodemon: {
    draw(x, y) {
      fill(x, y + 1, 13, 7, 0x4a3c34); fill(x + 1, y, 11, 9, 0x4a3c34);
      fill(x + 1, y, 11, 1, 0x6a5c50); fill(x, y + 1, 1, 7, 0x6a5c50); fill(x + 12, y + 1, 1, 7, 0x2e241e); fill(x + 1, y + 8, 11, 1, 0x2e241e);
      for (const [dx, dy] of [[3, 3], [7, 2], [9, 5], [4, 6], [6, 4]]) dot(x + dx, y + dy, 0x3a2e26);
      for (let k = 1; k < 12; k += 2) dot(x + k, y - 1, k % 4 === 1 ? 0x9a78d8 : 0x6a44a8);
      dot(x - 1, y + 3, 0x7a52b8); dot(x - 1, y + 5, 0x9a78d8); dot(x + 13, y + 4, 0x7a52b8);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    },
  },
};
