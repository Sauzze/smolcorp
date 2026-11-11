/**
 * Each workspace's parts: its back, drawn behind whoever is at it, and
 * its front, drawn over them. Each takes the desk's corner `x`, `y`, the
 * seat `s`, `tint` for its colours (its effort's paint over them, if
 * any), the office's desk `finish`, and `occupant`, who is at it now.
 */
const WORKSPACE_PARTS = {
  desk: {
    front(d) {
      drawMonitor(d.x + 6, d.y - 10);
      drawDeskBody(d);
    },
  },

  /** A desk raised on legs, its monitor on an arm, worked at standing up. */
  standing: {
    front({ x, y, s, tint, finish }) {
      drawMonitor(x + 6, y - 12);
      fill(x + 11, y - 1, 3, 1, PALETTE.monitorStand);
      const steel = 0x5a5f6c, foot = 0x3c404a;
      fill(x + 3, y + 5, 2, 10, steel);
      fill(x + 39, y + 5, 2, 10, steel);
      fill(x + 5, y + 9, 34, 1, foot);
      fill(x + 1, y + 15, 6, 1, foot);
      fill(x + 37, y + 15, 6, 1, foot);
      fill(x + 17, y + 5, 10, 2, foot);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      fill(x, y, DESK_WIDTH, 1, tint(finish.edge, 1));
      fill(x, y + 1, DESK_WIDTH, 3, tint(finish.top, 0.9));
      fill(x, y + 4, DESK_WIDTH, 2, tint(finish.front, 0.72));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      // The height buttons, under the top's front edge
      fill(x + 33, y + 6, 4, 1, foot);
      dot(x + 34, y + 6, 0x6fd36f);
      drawKeyboard(s, y + 2);
    },
  },

  /** A black desk with lights that cycle round the colour wheel, a racing chair and a tower under it. */
  gaming: {
    back({ s, y }) {
      const shell = 0x1d1f25, red = 0xd4140b;
      fill(s.x - 4, y - 15, 9, 15, shell);
      fill(s.x - 6, y - 8, 13, 7, shell);
      fill(s.x - 6, y - 8, 2, 7, red);
      fill(s.x + 5, y - 8, 2, 7, red);
      fill(s.x - 2, y - 15, 5, 1, red);
      fill(s.x - 1, y - 14, 3, 1, 0x3a3d47);
    },
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const glow = (i, across) => rainbow((now * 0.25 + i / across) % 1);
      for (let i = 0; i < 9; i++) dot(x + 8 + i, y - 3, glow(i, 9));
      fill(x, y, DESK_WIDTH, 1, tint(0x4a4e5a, 1));
      fill(x, y + 1, DESK_WIDTH, 4, tint(0x2a2d35, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x + 18, y + 1, 24, 3, 0x14161b);
      fill(x, y + 5, DESK_WIDTH, 3, tint(0x1f2228, 0.72));
      for (let i = 0; i < DESK_WIDTH - 2; i++) dot(x + 1 + i, y + 7, glow(i, DESK_WIDTH - 2));
      // Legs bent like a Z, and the tower, its fans lit
      fill(x + 2, y + 8, 3, 7, 0x1d1f25);
      fill(x + 1, y + 14, 7, 1, 0xd4140b);
      fill(x + 30, y + 8, 10, 8, 0x1d1f25);
      fill(x + 31, y + 9, 8, 6, 0x14161b);
      for (const [fy, i] of [[10, 0], [13, 3]]) {
        fill(x + 33, y + fy, 3, 2, glow(i, 6));
        dot(x + 34, y + fy, 0x14161b);
      }
      fill(x + 41, y + 8, 2, 7, 0x1d1f25);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
      for (let i = 0; i < 8; i++) dot(s.x - 4 + i, y + 2, glow(i, 8));
    },
  },

  /**
   * A tent, its flaps tied back, a lantern swinging from its ridge, and
   * whoever works in it in its doorway, a laptop on their knees. Its
   * canvas takes the effort's paint.
   */
  tent: {
    back({ x, y, tint }) {
      const canvas = 0x6f8a4a, cx = x + 29, peak = y - 24, base = y + 6;
      // Guy ropes out to their pegs
      for (let k = 0; k < 7; k++) {
        dot(x + 12 - k, y - 6 + k + (k > 4 ? 1 : 0), 0xc9c3b8, 0.7);
        dot(x + 46 + k, y - 6 + k + (k > 4 ? 1 : 0), 0xc9c3b8, 0.7);
      }
      fill(x + 4, y + 2, 2, 1, PALETTE.frame);
      fill(x + 53, y + 2, 2, 1, PALETTE.frame);
      fill(cx - 17, base + 1, 35, 1, SHADOW);
      for (let row = peak; row <= base; row++) {
        const half = Math.round(((row - peak) * 17) / 30);
        fill(cx - half, row, half * 2 + 1, 1, tint(canvas, 1));
        dot(cx - half, row, tint(canvas, 1.25));
        dot(cx + half, row, tint(canvas, 0.8));
        // The way in, dark inside
        const inner = row >= y - 20 ? Math.round(((row - (y - 20)) * 11) / 26) + 1 : -1;
        if (inner >= 0) fill(cx - inner, row, inner * 2 + 1, 1, tint(canvas, 0.35));
      }
      fill(cx, peak, 1, 4, tint(canvas, 1.25));
      // A groundsheet, out in front
      fill(cx - 13, base, 27, 1, tint(canvas, 0.6));
      // The lantern, swinging on its cord, lit
      const swing = Math.round(Math.sin(now * 1.6));
      dot(cx, y - 20, PALETTE.frame);
      dot(cx + swing, y - 19, PALETTE.frame);
      fill(cx - 1 + swing, y - 18, 3, 3, PALETTE.brass);
      dot(cx + swing, y - 17, 0xfff0b0);
      fill(cx - 4 + swing, y - 20, 9, 8, 0xffd36b, 0.12);
    },
    front({ x, y, s, tint, occupant }) {
      const canvas = 0x6f8a4a, cx = x + 29;
      // The flaps, rolled and tied either side of the way in
      for (let row = y - 8; row <= y + 5; row++) {
        const inner = Math.round(((row - (y - 20)) * 11) / 26) + 1;
        fill(cx - inner - 1, row, 2, 1, tint(canvas, 1.15));
        fill(cx + inner, row, 2, 1, tint(canvas, 1.15));
      }
      fill(cx - 10, y - 2, 2, 1, 0xc9c3b8);
      fill(cx + 9, y - 2, 2, 1, 0xc9c3b8);
      // Their feet, either side of the laptop, while they sit
      if (occupant && poseOf(occupant, now).kind === 'sit') {
        const outfit = outfitOf(occupant.session.agent, occupant.session.model, memberOf(occupant.session), 'tent', occupant.session.role, occupant.desk);
        fill(s.x - 5, y + 5, 3, 2, outfit.shoes);
        fill(s.x + 3, y + 5, 3, 2, outfit.shoes);
      }
      drawLaptop(x + 22, y - 3);
    },
  },

  /** A cardboard box for a desk, its flaps open, and another for a stool. */
  box: {
    back({ s, y, tint }) {
      fill(s.x - 4, y - 6, 9, 7, tint(0xa07a4a, 0.82));
      fill(s.x - 4, y - 6, 9, 1, tint(0xc9a06a, 1));
      fill(s.x, y - 6, 1, 7, 0xe0cfa0);
    },
    front({ x, y, s, tint }) {
      const face = 0xb58a55, top = 0xc9a06a, ink = 0x6e5233;
      fill(x - 2, y + 1, 3, 3, tint(top, 0.95));
      fill(x + 43, y + 1, 3, 3, tint(top, 0.95));
      fill(x + 1, y, 42, 1, tint(0xdcb78c, 1));
      fill(x + 1, y + 1, 42, 4, tint(top, 0.9));
      fill(x + 1, y + 5, 42, 11, tint(face, 0.72));
      fill(x + 1, y + 5, 42, 1, tint(0x8c6744, 0.62));
      fill(x + 40, y + 5, 3, 11, tint(0x8c6744, 0.62));
      // Tape down the middle, arrows for this way up, and a glass for fragile
      fill(x + 20, y, 4, 16, 0xe0cfa0, 0.75);
      for (const ax of [x + 6, x + 11]) {
        dot(ax, y + 8, ink);
        fill(ax - 1, y + 9, 3, 1, ink);
        fill(ax, y + 10, 1, 3, ink);
      }
      fill(x + 31, y + 8, 5, 3, ink);
      fill(x + 32, y + 11, 3, 1, ink);
      fill(x + 33, y + 12, 1, 2, ink);
      fill(x + 31, y + 14, 5, 1, ink);
      fill(x + 2, y + 16, 41, 1, SHADOW);
      drawLaptop(x + 6, y - 9);
      // A keyboard of its own in front of whoever sits there, the laptop being out of reach
      fill(s.x - 4, y + 1, 9, 2, 0x3a3d47);
      fill(s.x - 3, y + 1, 7, 1, 0x5a5f6c);
    },
  },

  /** A beanbag on a round rug, a laptop on the lap of whoever sinks into it. */
  beanbag: {
    back({ x, y, tint }) {
      const rug = 0x4a5a7a;
      fill(x + 10, y + 9, 34, 6, rug);
      fill(x + 13, y + 8, 28, 1, rug);
      fill(x + 13, y + 15, 28, 1, rug);
      fill(x + 12, y + 10, 30, 4, lighter(rug), 0.25);
      const bag = 0xc0584a;
      fill(x + 21, y - 10, 14, 1, tint(bag, 1.1));
      fill(x + 19, y - 9, 18, 1, tint(bag, 1.05));
      fill(x + 18, y - 8, 20, 12, tint(bag, 1));
      fill(x + 18, y - 8, 1, 1, tint(bag, 0.8));
      fill(x + 37, y - 8, 1, 1, tint(bag, 0.8));
      fill(x + 21, y - 9, 4, 1, WHITE, 0.25);
      fill(x + 34, y - 7, 3, 10, tint(bag, 0.85));
    },
    front({ x, y, s, tint, occupant }) {
      const bag = 0xc0584a;
      fill(x + 17, y + 1, 22, 7, tint(bag, 1.1));
      fill(x + 18, y + 8, 20, 2, tint(bag, 0.8));
      fill(x + 17, y + 1, 1, 1, tint(bag, 0.8));
      fill(x + 38, y + 1, 1, 1, tint(bag, 0.8));
      fill(x + 18, y + 10, 21, 1, SHADOW);
      // Legs out in front, crossed at the ankles, while they sit
      if (occupant && poseOf(occupant, now).kind === 'sit') {
        const outfit = outfitOf(occupant.session.agent, occupant.session.model, memberOf(occupant.session), 'beanbag', occupant.session.role, occupant.desk);
        fill(s.x - 3, y + 2, 7, 3, outfit.pants);
        fill(s.x - 4, y + 5, 3, 2, outfit.shoes);
        fill(s.x + 2, y + 5, 3, 2, outfit.shoes);
      }
      drawLaptop(x + 22, y - 4);
    },
  },

  /** An igloo of snow blocks, and a desk cut from ice, snow drifting down. */
  igloo: {
    back({ x, y }) {
      const snow = 0xe8f0f6, joint = 0xc4d2de, cx = x + 28, base = y + 4;
      for (let row = y - 19; row <= base; row++) {
        const t = (base - row) / 23, half = Math.round(18 * Math.sqrt(Math.max(0, 1 - t * t)));
        fill(cx - half, row, half * 2 + 1, 1, (base - row) % 5 === 0 ? joint : snow);
        fill(cx + half - 2, row, 3, 1, 0xd2dee8);
        if ((base - row) % 5 !== 0) {
          const course = Math.floor((base - row) / 5);
          for (let jx = cx - half + ((course % 2) * 4) + 3; jx < cx + half - 1; jx += 8) dot(jx, row, joint);
        }
      }
      // The way in, an arch
      for (let row = y - 14; row <= base; row++) {
        const half = [3, 5, 6][row - (y - 14)] ?? 6;
        fill(cx - half, row, half * 2 + 1, 1, 0x2a3a52);
      }
      fill(cx - 7, y - 14, 15, 1, 0xf8fbfd, 0.6);
      for (let k = 0; k < 4; k++) {
        const fall = (now * 5 + k * 6.5) % 26;
        dot(x + [6, 18, 36, 44][k], y - 22 + Math.round(fall), WHITE, 0.85 * (1 - fall / 26));
      }
    },
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, DESK_WIDTH, 1, tint(0xf4fbfe, 1));
      fill(x, y + 1, DESK_WIDTH, 4, tint(0xcfe6f2, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, DESK_WIDTH, 11, tint(0x9fc8dc, 0.72));
      fill(x, y + 5, DESK_WIDTH, 1, tint(0x86b4cc, 0.62));
      // Light caught in the ice
      for (const [sx, sy] of [[5, 7], [22, 9], [34, 6]]) {
        for (let k = 0; k < 4; k++) dot(x + sx + k, y + sy + 4 - k, WHITE, 0.45);
      }
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },

  /**
   * A camper van, its side hatch propped up as an awning over a counter,
   * fairy lights under it, and whoever works in it seen through the hatch.
   */
  van: {
    back({ x, y }) {
      fill(x + 9, y - 14, 33, 15, 0x3a2f2a);
      fill(x + 9, y - 14, 33, 2, 0x2a221e);
      fill(x + 36, y - 11, 5, 1, 0x6b4e37);
      fill(x + 36, y - 13, 2, 2, PALETTE.mug);
    },
    front({ x, y, tint }) {
      const cream = 0xece4cf, paint = 0x4f9a9a;
      fill(x, y - 21, 44, 1, cream);
      fill(x - 2, y - 20, 49, 6, cream);
      fill(x - 2, y - 14, 11, 15, cream);
      fill(x + 42, y - 14, 5, 15, cream);
      // The cab's window, the counter and the paint below it
      fill(x - 1, y - 13, 8, 7, 0x9fc8dc);
      fill(x, y - 12, 2, 4, WHITE, 0.4);
      fill(x + 7, y, 37, 2, PALETTE.deskTop);
      fill(x + 7, y + 2, 37, 1, PALETTE.deskFront);
      fill(x - 2, y + 1, 9, 2, cream);
      fill(x + 44, y + 1, 3, 2, cream);
      fill(x - 2, y + 3, 49, 9, tint(paint, 0.85));
      fill(x - 2, y + 3, 49, 1, tint(paint, 1.1));
      fill(x - 2, y + 7, 3, 2, 0xf2d35b);
      fill(x + 44, y + 7, 3, 2, 0xd04a4a);
      fill(x + 19, y + 5, 1, 6, tint(paint, 0.6));
      // Wheels
      for (const wx of [x + 3, x + 33]) {
        fill(wx, y + 10, 9, 6, 0x1d1f25);
        fill(wx - 1, y + 11, 11, 4, 0x1d1f25);
        fill(wx + 3, y + 12, 3, 2, 0xc9c3b8);
      }
      fill(x - 2, y + 16, 49, 1, SHADOW);
      // The awning, striped, and its fairy lights
      fill(x + 7, y - 18, 37, 3, 0xd96a5a);
      for (let k = 0; k < 37; k += 6) fill(x + 7 + k, y - 18, 3, 3, 0xf4f1ea);
      fill(x + 7, y - 15, 37, 1, SHADOW);
      for (let k = 0; k < 8; k++) {
        const lit = (Math.floor(now * 1.5) + k) % 3 !== 0;
        dot(x + 11 + k * 4, y - 14 + (k % 2), [0xf2d35b, 0xff8a3d, 0x6fd36f, 0x8fc0e8][k % 4], lit ? 1 : 0.3);
      }
      drawMonitor(x + 11, y - 10);
    },
  },

  /** A spaceship's cockpit: a console of lights, a pilot's seat, a canopy of glass and a blinking aerial. */
  cockpit: {
    back({ s, y }) {
      fill(s.x - 5, y - 15, 11, 15, 0x3a3f4a);
      fill(s.x - 5, y - 15, 1, 15, 0x8a93a3);
      fill(s.x + 5, y - 15, 1, 15, 0x8a93a3);
      fill(s.x - 3, y - 14, 7, 3, 0x2a2e36);
    },
    front({ x, y, s, tint }) {
      const hull = 0x5a6270, cx = x + 28, glass = 0x9fe6f2;
      // The canopy, a bubble round the seat
      for (let row = y - 20; row <= y; row++) {
        const t = (y - row) / 20, half = Math.round(14 * Math.sqrt(Math.max(0, 1 - t * t)));
        fill(cx - half, row, half * 2 + 1, 1, glass, 0.1);
        dot(cx - half, row, glass, 0.75);
        dot(cx + half, row, glass, 0.75);
        if (row === y - 20) fill(cx - half, row, half * 2 + 1, 1, glass, 0.75);
      }
      for (let k = 0; k < 6; k++) dot(cx - 9 + Math.round(k * 0.8), y - 17 + k, WHITE, 0.5);
      fill(cx, y - 23, 1, 3, 0x8a93a3);
      dot(cx, y - 24, PALETTE.alarm, Math.floor(now * 1.5) % 2 ? 1 : 0.25);
      drawMonitor(x + 6, y - 10);
      fill(x + 6, y - 11, 13, 1, 0x4ad4e8, 0.8);
      // The console, its lights blinking in turn
      fill(x, y, DESK_WIDTH, 1, tint(0x8a93a3, 1));
      fill(x, y + 1, DESK_WIDTH, 4, tint(hull, 0.9));
      fill(x, y + 5, DESK_WIDTH, 11, tint(0x3a3f4a, 0.72));
      fill(x, y + 5, DESK_WIDTH, 1, tint(0x2a2e36, 0.62));
      const lights = [0x6fd36f, 0xf2d35b, 0xd96a5a, 0x4ad4e8];
      for (let i = 0; i < 7; i++) {
        const on = (Math.floor(now * 3) + i * 5) % 7 < 4;
        dot(x + 2 + i * 2, y + 2 + (i % 2), lights[i % 4], on ? 1 : 0.35);
        dot(x + 35 + (i % 4) * 2, y + 2 + Math.floor(i / 4), lights[(i + 1) % 4], on ? 0.35 : 1);
      }
      for (let k = 0; k < 10; k++) fill(x + 3 + k * 4, y + 13, 2, 2, k % 2 ? PALETTE.hazard : PALETTE.machine);
      fill(x + 4, y + 8, 12, 3, 0x1d1f25);
      fill(x + 5, y + 9, Math.round(5 + Math.sin(now * 2) * 5), 1, 0x4ad4e8);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
      fill(s.x + 8, y - 2, 1, 3, PALETTE.frame);
      dot(s.x + 8, y - 3, PALETTE.alarm);
    },
  },

  /** The deck of a pirate ship: a barrel to sit on, the Jolly Roger flying, and a wheel that turns while they work. */
  ship: {
    back({ x, y, s }) {
      // The flag, on its staff at the stern
      fill(x + 1, y - 23, 1, 23, 0x5a3b25);
      dot(x + 1, y - 24, PALETTE.brass);
      const wave = Math.floor(now * 3) % 2;
      for (let c = 0; c < 9; c++) fill(x + 2 + c, y - 23 + (c > 1 && (c + wave) % 2 ? 1 : 0), 1, 6, 0x1a1a1f);
      fill(x + 5, y - 22, 3, 2, WHITE);
      dot(x + 5, y - 19, WHITE);
      dot(x + 7, y - 19, WHITE);
      // The barrel they sit on
      fill(s.x - 4, y - 6, 9, 10, 0x7a5232);
      fill(s.x - 4, y - 6, 9, 1, 0x8c6744);
      fill(s.x - 4, y - 4, 9, 1, 0x3a3a40);
      fill(s.x - 4, y, 9, 1, 0x3a3a40);
    },
    front({ x, y, tint, occupant }) {
      drawMonitor(x + 6, y - 10);
      const hull = 0x6b4a2e;
      // The rail on its posts, and the hull below, curving in at the keel
      fill(x - 1, y, 46, 1, 0xa87a56);
      fill(x - 1, y + 1, 46, 1, 0x8c6744);
      fill(x - 1, y + 2, 46, 3, 0x2a1d16);
      for (let k = 0; k < 12; k++) fill(x + 1 + k * 4, y + 2, 1, 3, 0x8c6744);
      fill(x - 1, y + 5, 46, 1, 0x4a301e);
      fill(x - 1, y + 6, 46, 8, tint(hull, 0.8));
      fill(x + 1, y + 14, 42, 1, tint(hull, 0.65));
      fill(x + 4, y + 15, 36, 1, tint(hull, 0.55));
      for (const py of [8, 11]) fill(x - 1, y + py, 46, 1, tint(hull, 0.6));
      // Portholes, ringed in brass
      for (const px of [x + 6, x + 32]) {
        fill(px, y + 7, 4, 4, PALETTE.brass);
        fill(px + 1, y + 8, 2, 2, 0x2f7fb0);
      }
      fill(x + 2, y + 16, 40, 1, SHADOW);
      // The wheel, turning while someone works at it
      const cx = x + 39, cy = y - 5;
      const working = occupant && occupant.session.state === 'thinking';
      const turn = working ? now * 1.2 : 0.3;
      fill(cx - 1, y - 1, 3, 2, 0x5a3b25);
      for (let k = 0; k < 8; k++) {
        const a = turn + (k * Math.PI) / 4;
        for (let r = 1; r <= 6; r++) dot(cx + Math.round(Math.cos(a) * r), cy + Math.round(Math.sin(a) * r), r > 4 ? 0xc49a6c : 0x6b4e37);
      }
      for (let k = 0; k < 24; k++) {
        const a = (k * Math.PI) / 12;
        dot(cx + Math.round(Math.cos(a) * 4), cy + Math.round(Math.sin(a) * 4), 0xa87a56);
      }
      fill(cx - 1, cy - 1, 3, 3, PALETTE.brass);
    },
  },

  /** A throne of gold and red velvet, a jewel glinting on top, at a desk trimmed in gold. Whoever sits in it wears a crown. */
  throne: {
    back({ x, y }) {
      const gold = 0xd6b25e, velvet = 0x9a2a3a;
      fill(x + 25, y - 22, 7, 1, gold);
      fill(x + 21, y - 21, 15, 25, gold);
      fill(x + 23, y - 18, 11, 22, velvet);
      fill(x + 23, y - 18, 11, 1, shaded(velvet, 0.7));
      dot(x + 21, y - 23, gold);
      dot(x + 35, y - 23, gold);
      fill(x + 19, y - 4, 3, 2, gold);
      fill(x + 35, y - 4, 3, 2, gold);
      fill(x + 27, y - 21, 3, 2, 0xd4140b);
      dot(x + 28, y - 22, WHITE, 0.4 + 0.6 * Math.max(0, Math.sin(now * 2.2)));
    },
    front(d) {
      const { x, y, s, occupant } = d;
      drawMonitor(x + 6, y - 10);
      drawDeskBody(d);
      fill(x, y + 5, DESK_WIDTH, 1, PALETTE.brass);
      fill(x + 30, y + 8, 10, 1, PALETTE.brass);
      fill(x + 34, y + 10, 2, 1, PALETTE.brass);
      if (!occupant) return;
      const top = topOf(occupant, poseOf(occupant, now));
      fill(s.x - 2, top - 2, 5, 1, PALETTE.brass);
      for (const dx of [-2, 0, 2]) dot(s.x + dx, top - 3, PALETTE.brass);
      dot(s.x, top - 2, 0xd4140b);
    },
  },

  /** A greenhouse of glass, plants hanging from its roof and potted along its sides, a potting bench in front. */
  greenhouse: {
    back({ x, y }) {
      const bar = 0xe8ece6, cx = x + 29;
      fill(x + 13, y - 14, 33, 18, 0xa8d8b0, 0.16);
      for (let row = y - 23; row < y - 14; row++) {
        const half = Math.round(((row - (y - 23)) * 16) / 9);
        fill(cx - half, row, half * 2 + 1, 1, 0xa8d8b0, 0.16);
      }
      for (const bx of [x + 21, x + 37]) fill(bx, y - 14, 1, 18, bar, 0.5);
      // Vines from the roof, swaying, and pots along the walls
      for (const [vx, length, phase] of [[x + 16, 9, 0], [x + 42, 7, 1.3], [x + 33, 5, 2.1]]) {
        for (let k = 0; k < length; k++) {
          const sway = k > 2 ? Math.round(Math.sin(now * 1.2 + phase + k * 0.4) * 0.8) : 0;
          dot(vx + sway, y - 14 + k, k % 2 ? PALETTE.leafDark : PALETTE.leaf);
        }
      }
      for (const [px, flower] of [[x + 15, 0xf28fb0], [x + 40, 0xf2d35b]]) {
        fill(px, y - 2, 4, 3, PALETTE.pot);
        fill(px - 1, y - 8, 6, 6, PALETTE.leaf);
        fill(px, y - 9, 4, 1, PALETTE.leafDark);
        dot(px + 1, y - 6, PALETTE.leafDark);
        dot(px, y - 8, flower);
        dot(px + 3, y - 6, flower);
      }
      // A hanging basket, in flower
      fill(cx + 7, y - 21, 1, 3, PALETTE.frame);
      fill(cx + 5, y - 18, 5, 2, PALETTE.pot);
      fill(cx + 4, y - 19, 7, 1, PALETTE.leaf);
      dot(cx + 6, y - 20, 0xf28fb0);
      dot(cx + 9, y - 20, 0xf2d35b);
    },
    front(d) {
      const { x, y } = d, bar = 0xe8ece6, cx = x + 29;
      // The frame, over the panes either side of the open door
      fill(x + 13, y - 14, 1, 18, bar);
      fill(x + 45, y - 14, 1, 18, bar);
      fill(x + 13, y - 14, 33, 1, bar);
      for (let row = y - 23; row < y - 14; row++) {
        const half = Math.round(((row - (y - 23)) * 16) / 9);
        dot(cx - half, row, bar);
        dot(cx + half, row, bar);
      }
      fill(cx, y - 23, 1, 9, bar, 0.6);
      fill(x + 14, y - 13, 7, 17, 0xcfeee0, 0.18);
      fill(x + 38, y - 13, 7, 17, 0xcfeee0, 0.18);
      fill(x + 15, y - 12, 1, 4, WHITE, 0.5);
      drawMonitor(x + 4, y - 10);
      drawDeskBody(d);
      // Seedlings in a tray on the bench
      fill(x + 19, y + 1, 9, 2, 0x5a3b25);
      for (let k = 0; k < 4; k++) dot(x + 20 + k * 2, y, PALETTE.leaf);
    },
  },

  /* Desks of other kinds: the office's chair at each, its monitor where a desk has it */

  /** Console desk. Lights blink in rows, a readout rises and falls, a glowing strip along its lip. */
  console: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xb8c0cc, 1)); fill(x, y + 1, 44, 4, tint(0x8a93a3, 0.9)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(0x4a515e, 0.72)); fill(x, y + 5, 44, 1, tint(0x3a3f4a, 0.62)); fill(x, y + 6, 44, 1, 0x4ad4e8, 0.45);
      fill(x + 3, y + 8, 22, 5, 0x1d1f25);
      const lights = [0x6fd36f, 0xf2d35b, 0xd96a5a, 0x4ad4e8];
      for (let i = 0; i < 10; i++) { const on = (Math.floor(now * 3) + i * 3) % 7 < 4; dot(x + 4 + (i % 5) * 2, y + 9 + Math.floor(i / 5) * 2, lights[i % 4], on ? 1 : 0.3); }
      fill(x + 15, y + 9, 9, 3, 0x0e2a1a); fill(x + 16, y + 10, Math.round(4 + Math.sin(now * 1.7) * 3), 1, 0x6fd36f);
      for (let k = 0; k < 3; k++) fill(x + 29, y + 8 + k * 2, 12, 1, 0x2a2e36);
      fill(x, y + 14, 44, 2, 0x2a2e36); fill(x + 1, y + 16, 42, 1, SHADOW); drawKeyboard(s, y + 2);
    },
  },
  /** Hover desk. No legs: two thrusters underneath, their glow pulsing. */
  hover: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const glow = 0.5 + 0.5 * Math.sin(now * 5);
      fill(x + 4, y + 16, 36, 1, SHADOW, 0.5); fill(x + 8, y + 16, 28, 1, SHADOW);
      fill(x + 1, y, 42, 1, tint(0xdcebf4, 1)); fill(x, y + 1, 44, 4, tint(0xb8c4d0, 0.9)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 3, tint(0x6a7280, 0.72)); fill(x, y + 5, 44, 1, tint(0x5a6270, 0.62)); fill(x + 1, y + 8, 42, 1, tint(0x4a515e, 0.62));
      for (const tx of [x + 5, x + 33]) { fill(tx, y + 9, 6, 2, 0x3a3f4a); fill(tx + 1, y + 11, 4, 1, 0x4ad4e8, 0.6 + 0.4 * glow); fill(tx + 1, y + 12, 4, 2, 0x4ad4e8, 0.1 + 0.25 * glow); fill(tx + 2, y + 14, 2, 1, 0x9fe6f2, 0.2); }
      drawKeyboard(s, y + 2);
    },
  },
  /** Sea-chest desk. Iron bands, brass corners and a brass lock, a rope handle at its end. */
  chest: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xa87a56, 1)); fill(x, y + 1, 44, 4, tint(0x8c6744, 0.9)); fill(x + 14, y + 1, 1, 4, 0x6b4a2e); fill(x + 29, y + 1, 1, 4, 0x6b4a2e);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(0x6b4a2e, 0.72)); fill(x, y + 5, 44, 1, tint(0x4a301e, 0.62)); fill(x, y + 9, 44, 1, tint(0x4a301e, 0.62)); fill(x, y + 12, 44, 1, tint(0x4a301e, 0.62));
      for (const bx of [x + 4, x + 38]) { fill(bx, y + 5, 2, 11, 0x3a3a40); for (const ry of [7, 10, 13]) dot(bx, y + ry, 0x6a6a72); }
      fill(x + 19, y + 6, 6, 6, PALETTE.brass); fill(x + 19, y + 6, 6, 1, 0xf2d38a); dot(x + 21, y + 8, 0x1d1f25); fill(x + 21, y + 9, 1, 2, 0x1d1f25);
      fill(x, y + 14, 2, 2, PALETTE.brass); fill(x + 42, y + 14, 2, 2, PALETTE.brass); fill(x - 1, y + 8, 1, 4, 0xc9a06a);
      fill(x + 1, y + 16, 42, 1, SHADOW); drawKeyboard(s, y + 2);
    },
  },
  /** Gilded writing desk. Mahogany with a green leather top, gilt trim and curved legs. */
  gilded: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const gold = PALETTE.brass, front = 0x5a221c;
      fill(x, y, 44, 1, tint(0x9a4a3a, 1)); fill(x, y + 1, 44, 4, tint(0x6a2a22, 0.9)); fill(x + 3, y + 1, 38, 3, 0x2f5a3a); fill(x + 3, y + 1, 38, 1, 0x3f7a4c);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 6, tint(front, 0.72)); fill(x, y + 5, 44, 1, gold);
      dot(x + 10, y + 8, gold); dot(x + 33, y + 8, gold); fill(x + 20, y + 7, 4, 3, gold); dot(x + 21, y + 8, 0xd4140b);
      for (let k = 0; k < 44; k += 4) fill(x + k + 1, y + 11, 2, 1, tint(front, 0.72));
      for (const [lx, dir] of [[x + 1, 1], [x + 40, -1]]) { fill(lx, y + 11, 3, 2, tint(front, 0.72)); fill(lx + (dir > 0 ? 1 : 0), y + 13, 2, 2, tint(front, 0.72)); fill(lx + (dir > 0 ? 0 : 1), y + 15, 3, 1, gold); }
      fill(x + 1, y + 16, 42, 1, SHADOW); drawKeyboard(s, y + 2);
    },
  },
  /** Expedition cases. An aluminium top on two orange field cases, snow on their corners, a fuel can between. */
  crates: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xe0e4ea, 1)); fill(x, y + 1, 44, 4, tint(0xb8bec8, 0.9)); for (let k = 4; k < 44; k += 8) dot(x + k, y + 4, 0x9aa0a8);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand); fill(x, y + 5, 44, 1, tint(0x8a909a, 0.62));
      fill(x + 14, y + 6, 16, 10, 0x1d1f25, 0.45);
      for (const cx of [x + 1, x + 29]) {
        fill(cx, y + 6, 14, 10, tint(0xe0703a, 0.85)); fill(cx, y + 6, 14, 1, tint(0xf08a50, 1)); fill(cx, y + 12, 14, 1, tint(0xb85a2a, 0.7));
        fill(cx + 2, y + 11, 2, 2, 0x1d1f25); fill(cx + 10, y + 11, 2, 2, 0x1d1f25); fill(cx + 5, y + 8, 4, 1, 0x1d1f25);
        dot(cx + 1, y + 6, WHITE, 0.8); dot(cx + 13, y + 6, WHITE, 0.6);
      }
      fill(x + 18, y + 11, 5, 5, 0xc9443a); fill(x + 19, y + 10, 2, 1, 0x3a3a40);
      fill(x + 1, y + 16, 42, 1, SHADOW); drawKeyboard(s, y + 2);
    },
  },
  /** Stone altar. A slab on two pedestals, a rune glowing in the middle, a candle's drips. */
  altar: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x - 1, y, 46, 1, tint(0x9a9da2, 1)); fill(x - 1, y + 1, 46, 4, tint(0x7a7d82, 0.9)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x - 1, y + 5, 46, 3, tint(0x62656a, 0.72)); fill(x - 1, y + 5, 46, 1, tint(0x4a4c52, 0.62));
      for (const px of [x + 2, x + 31]) { fill(px, y + 8, 11, 8, tint(0x5a5d62, 0.8)); fill(px, y + 11, 11, 1, 0x44464b); fill(px + 5, y + 8, 1, 3, 0x44464b); fill(px + 3, y + 12, 1, 4, 0x44464b); }
      fill(x + 13, y + 8, 18, 8, tint(0x4e5156, 0.75));
      const glow = 0.45 + 0.4 * Math.sin(now * 1.6), rune = 0xc05aff;
      fill(x + 18, y + 8, 9, 8, rune, glow * 0.12); fill(x + 21, y + 9, 1, 6, rune, glow); dot(x + 22, y + 10, rune, glow); dot(x + 23, y + 11, rune, glow); dot(x + 22, y + 12, rune, glow); dot(x + 20, y + 13, rune, glow);
      dot(x - 1, y + 7, 0x3b5a3a); fill(x + 41, y + 5, 3, 1, 0x4f7a3a); dot(x + 2, y + 15, 0x3b5a3a);
      fill(x + 2, y, 1, 3, 0xeae6dc); dot(x + 3, y + 1, 0xeae6dc);
      fill(x, y + 16, 44, 1, SHADOW); drawKeyboard(s, y + 2);
    },
  },
  /** Split-log desk. A slab of log, bark on its edge, on two stumps. */
  log: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xe8c89a, 1)); fill(x, y + 1, 44, 4, tint(0xc9a06a, 0.9));
      for (const [gx, gy, gw] of [[3, 2, 10], [16, 3, 14], [30, 2, 9], [8, 4, 7]]) fill(x + gx, y + gy, gw, 1, tint(0xb08a5a, 0.85));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 5, tint(0x6e4a2e, 0.72)); fill(x, y + 5, 44, 1, tint(0x5a3b25, 0.62)); for (let k = 1; k < 44; k += 3) fill(x + k, y + 6 + (k % 2), 1, 3, 0x5a3b25);
      for (const lx of [x + 3, x + 34]) { fill(lx, y + 10, 7, 6, tint(0x6e4a2e, 0.72)); fill(lx + 1, y + 10, 1, 6, 0x5a3b25); fill(lx + 4, y + 11, 1, 4, 0x5a3b25); fill(lx, y + 10, 7, 1, tint(0xc9a06a, 0.85)); }
      fill(x - 1, y + 1, 1, 8, 0xd8b07a); dot(x - 1, y + 4, 0xa87a50);
      fill(x + 1, y + 16, 42, 1, SHADOW); drawKeyboard(s, y + 2);
    },
  },
  /** Workbench. Butcher-block top on steel legs, paint and a toolbox on the shelf, a vice at the end. */
  bench: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const steel = 0x5a5f6c;
      fill(x, y, 44, 1, tint(0xdcb78c, 1)); fill(x, y + 1, 44, 4, tint(0xc49a6c, 0.9)); for (let k = 5; k < 44; k += 6) fill(x + k, y + 1, 1, 4, tint(0xb08a5a, 0.85));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 2, tint(0x8c6744, 0.72)); fill(x + 1, y + 7, 2, 9, steel); fill(x + 41, y + 7, 2, 9, steel); fill(x + 1, y + 12, 42, 1, steel);
      fill(x + 5, y + 9, 4, 3, 0x3f6fc4); fill(x + 5, y + 9, 4, 1, 0x9aa0a6); fill(x + 10, y + 10, 3, 2, 0xeae6dc);
      fill(x + 28, y + 8, 9, 4, 0xc9443a); fill(x + 31, y + 7, 3, 1, 0x3a3a40); fill(x + 28, y + 9, 9, 1, 0x9a2a2a);
      fill(x + 20, y + 7, 1, 4, 0x9aa0a6); fill(x + 19, y + 11, 3, 1, 0x9aa0a6);
      fill(x, y - 2, 5, 3, 0x3f6fc4); fill(x, y - 2, 5, 1, 0x5a8ad0); fill(x - 2, y - 1, 2, 1, 0x9aa0a6);
      fill(x + 1, y + 16, 42, 1, SHADOW); drawKeyboard(s, y + 2);
    },
  },
  /** Bamboo desk. Canes laid across for a top, more standing for its front, lashed at the corners. */
  bamboo: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xd8d070, 1)); fill(x, y + 1, 44, 4, tint(0xb8b05a, 0.9)); fill(x, y + 2, 44, 1, tint(0x9a9248, 0.85)); fill(x, y + 4, 44, 1, tint(0x9a9248, 0.85));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand); fill(x, y + 5, 44, 1, tint(0x6a6430, 0.62));
      for (let k = 0; k < 44; k += 3) { const c = (k / 3) % 2 ? 0xa8a050 : 0x98904a; fill(x + k, y + 6, 2, 10, tint(c, 0.8)); fill(x + k + 2, y + 6, 1, 10, 0x3a3a20); fill(x + k, y + 9 + ((k / 3) % 3), 2, 1, tint(0x6a6430, 0.7)); }
      for (const lx of [x + 1, x + 40]) { fill(lx, y + 5, 3, 3, 0xc9a06a); dot(lx + 1, y + 6, 0x8a6a42); }
      fill(x + 1, y + 16, 42, 1, SHADOW); drawKeyboard(s, y + 2);
    },
  },
  /** Weight-rack desk. Rubber top on a red rack, plates stored on its peg, a kettlebell under. */
  rack: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const steel = 0x2a2c30;
      fill(x, y, 44, 1, tint(0x5a5f6c, 1)); fill(x, y + 1, 44, 4, tint(0x3c404a, 0.9)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 2, tint(0xc9443a, 0.8)); fill(x, y + 5, 44, 1, tint(0x9a2a2a, 0.7));
      fill(x + 1, y + 7, 3, 9, steel); fill(x + 40, y + 7, 3, 9, steel); fill(x + 4, y + 14, 36, 1, steel);
      disc(x + 30, y + 12, 3, 0xe8c04a); disc(x + 30, y + 12, 1, 0x1d1f25);
      disc(x + 35, y + 11, 4, 0x3f6fc4); disc(x + 35, y + 11, 2, 0x2a4a8a); dot(x + 35, y + 11, 0x9aa0a6);
      fill(x + 10, y + 12, 5, 3, 0x3a3d47); fill(x + 11, y + 10, 3, 1, 0x3a3d47); dot(x + 10, y + 11, 0x3a3d47); dot(x + 14, y + 11, 0x3a3d47);
      fill(x + 1, y + 16, 42, 1, SHADOW); drawKeyboard(s, y + 2);
    },
  },
  /** Drafting table. A blueprint taped down under a parallel rule, on crossed legs. */
  drafting: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xdcb78c, 1)); fill(x, y + 1, 44, 4, tint(0xc49a6c, 0.9));
      fill(x + 31, y + 1, 12, 4, 0x3f6fc4); fill(x + 33, y + 2, 6, 1, 0xa8c8f0); fill(x + 40, y + 2, 1, 2, 0xa8c8f0); for (const [dx, dy] of [[31, 1], [42, 1], [31, 4], [42, 4]]) dot(x + dx, y + dy, 0xe8e4d0);
      fill(x + 29, y + 4, 15, 1, 0x2a2a2e);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 2, tint(0x8c6744, 0.72)); fill(x, y + 5, 44, 1, tint(0x76573a, 0.62));
      for (const lx of [x + 2, x + 31]) { line(lx, y + 7, lx + 10, y + 15, tint(0x76573a, 0.62)); line(lx + 10, y + 7, lx, y + 15, tint(0x76573a, 0.62)); }
      fill(x + 1, y + 16, 42, 1, SHADOW); drawKeyboard(s, y + 2);
    },
  },
  /** Glass desk. A glass top on chrome legs, the floor showing through. */
  glass: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const chrome = 0xc8cdd6;
      for (const lx of [x + 3, x + 39]) { fill(lx, y + 1, 2, 15, chrome); fill(lx, y + 1, 1, 15, WHITE, 0.4); fill(lx - 1, y + 15, 4, 1, 0x9aa0a6); }
      fill(x, y, 44, 1, tint(0xeef6fa, 1), 0.9); fill(x, y + 1, 44, 4, tint(0xa8d0e0, 1), 0.45); fill(x, y + 5, 44, 1, tint(0x8ab8cc, 1), 0.8);
      fill(x + 2, y + 1, 10, 1, WHITE, 0.35); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x + 1, y + 16, 42, 1, SHADOW, 0.5); drawKeyboard(s, y + 2);
    },
  },
  /** Scorer's table. A black skirt carrying a lower third, and a shot clock on its front counting down from 24. */
  scorer: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x3a3a3a, 1)); fill(x, y + 1, 44, 4, tint(0x1a1a1a, 0.9)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(0x0e0e0e, 1)); fill(x, y + 5, 44, 1, tint(0x333333, 1));
      fill(x + 3, y + 9, 3, 3, 0x3558ce); fill(x + 6, y + 9, 12, 3, 0xf5f5f5);
      fill(x + 8, y + 10, 7, 1, 0x555555);
      fill(x + 27, y + 7, 13, 7, 0x1a1a1a); fill(x + 27, y + 7, 13, 1, 0x333333);
      // Each table's clock in its own time
      pixelText(String(24 - ((Math.floor(now) + x * 7 + y * 3) % 25)).padStart(2, '0'), x + 30, y + 8, 0xe0503a, false);
      fill(x + 1, y + 16, 42, 1, SHADOW); drawKeyboard(s, y + 2);
    },
  },
  /** Anchor desk. A studio desk in black, rounding in below, its edge lit in the move's blue, a lower third on its front. */
  anchor: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x3a3a3a, 1)); fill(x, y + 1, 44, 4, tint(0x1a1a1a, 0.9)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 1, 0x4a6be0);
      fill(x + 1, y + 6, 42, 8, tint(0x0e0e0e, 1)); fill(x + 3, y + 14, 38, 2, tint(0x0e0e0e, 1));
      fill(x + 14, y + 8, 4, 4, 0x3558ce); fill(x + 18, y + 8, 12, 4, 0xf5f5f5); fill(x + 20, y + 9, 7, 1, 0x0e0e0e); fill(x + 20, y + 10, 5, 1, 0x8a8a8a);
      fill(x + 3, y + 16, 38, 1, SHADOW); drawKeyboard(s, y + 2);
    },
  },
  /**
   * Plaid desk. Honey wood, a red-and-black plaid runner laid along its top and hanging over both ends with a cream
   * fringe, and a mug in a knitted cosy by the monitor.
   */
  plaiddesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const red = 0xb8302a, black = 0x1f1a1c, cross = 0x6a1e20, cream = 0xf2ead8;
      fill(x, y, 44, 1, tint(0xdcb078, 1)); fill(x, y + 1, 44, 4, tint(0xc48e56, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(0x9a6a3e, 0.72)); fill(x, y + 5, 44, 1, tint(0x7a5232, 0.62));
      for (const gx of [14, 29]) fill(x + gx, y + 6, 1, 10, tint(0x7a5232, 0.62));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      // The runner: along the top, and down over each end
      const plaid = (px, py) => { const a = px % 4 === 0, b = py % 3 === 0; return a && b ? black : a || b ? cross : red; };
      for (let dx = 1; dx < 43; dx++) for (let dy = 2; dy <= 4; dy++) dot(x + dx, y + dy, plaid(dx, dy));
      for (const ex of [1, 37]) {
        for (let dx = 0; dx < 6; dx++) for (let dy = 5; dy <= 11; dy++) dot(x + ex + dx, y + dy, plaid(ex + dx, dy));
        for (let dx = 0; dx < 6; dx += 2) dot(x + ex + dx, y + 12, cream);
      }
      // The mug, in a berry cosy with cream stripes
      fill(x + 1, y - 5, 4, 1, WHITE); fill(x + 2, y - 5, 2, 1, 0x6a3a22);
      fill(x + 1, y - 4, 4, 4, red); fill(x + 1, y - 3, 4, 1, cream); fill(x + 1, y - 1, 4, 1, cream);
      dot(x, y - 3, WHITE); dot(x, y - 2, WHITE);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Gingerbread desk. Biscuit-brown, white icing piped along its edge in scallops and in a zigzag round its front, and a
   * row of gumdrops; now and then one of them wobbles.
   */
  gingerdesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const icing = 0xfaf6ee, crumb = 0x9a5a28;
      fill(x, y, 44, 1, tint(0xe0a868, 1)); fill(x, y + 1, 44, 4, tint(0xc4803e, 0.9));
      for (const dx of [24, 30, 36, 27, 33]) dot(x + dx, y + 2 + (dx % 2), crumb);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(0xa8642e, 0.72)); fill(x, y + 5, 44, 1, tint(0x8a4e22, 0.62));
      // The icing: along the top's edge in scallops, down its corners, and a zigzag near its foot
      fill(x, y + 4, 44, 1, icing);
      for (let dx = 0; dx < 44; dx++) {
        if (dx % 4 < 3) dot(x + dx, y + 5, icing);
        if (dx % 4 === 1) dot(x + dx, y + 6, icing);
        dot(x + dx, y + 13 + (dx % 4 < 2 ? 0 : 1), icing, 0.9);
      }
      fill(x, y + 5, 1, 11, icing); fill(x + 43, y + 5, 1, 11, icing);
      // The gumdrops, one wobbling now and then
      const wobbling = Math.floor((now + x * 0.11) / 9) % 5, wob = (now + x * 0.11) % 9 < 1 ? (Math.floor(now * 8) % 2 ? 1 : -1) : 0;
      [[6, 0xe0302a], [14, 0x3fae4a], [22, 0xf2c83a], [30, 0x9a5ad0], [37, 0xf27a3a]].forEach(([gx, c], k) => {
        const wx = x + gx + (k === wobbling ? wob : 0);
        fill(wx, y + 9, 3, 2, c); dot(wx + 1, y + 8, c); dot(wx, y + 9, WHITE, 0.6); dot(wx + 2, y + 10, darker(c));
      });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Court desk. Its top laid with hardwood court boards, a painted sideline along its edge; on its front the key in
   * teal, outlined in white, under the three-point arc, and a ball at rest in a little rack between its legs.
   */
  courtdesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const board = 0xd9a35e, alt = 0xcc9450, seam = 0xa87038, paint = 0xf4f1ea;
      fill(x, y, 44, 1, tint(0xeec48a, 1)); fill(x, y + 1, 44, 4, tint(board, 0.9));
      fill(x, y + 2, 44, 1, tint(alt, 0.9));
      for (const [jx, row] of [[x + 7, 1], [x + 21, 3], [x + 30, 1], [x + 38, 3]]) fill(jx, y + row, 1, 1, tint(seam, 0.9));
      fill(x, y + 4, 44, 1, paint, 0.85);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      // The front: an apron of boards, the key and the arc painted on it
      fill(x, y + 5, 44, 7, tint(board, 0.75));
      for (let r = 0; r < 7; r += 3) fill(x, y + 5 + r, 44, 1, tint(seam, 0.75), 0.6);
      for (const [jx, r] of [[x + 9, 1], [x + 33, 1], [x + 15, 4], [x + 39, 4]]) fill(jx, y + 5 + r, 1, 2, tint(seam, 0.75), 0.6);
      fill(x + 16, y + 5, 12, 6, SIDE_TEAL);
      fill(x + 16, y + 5, 1, 6, paint); fill(x + 27, y + 5, 1, 6, paint); fill(x + 16, y + 10, 12, 1, paint);
      for (let k = 0; k <= 40; k++) {
        const a = (k / 40) * Math.PI, ax = x + 22 + Math.round(Math.cos(a) * 19), ay = y + 5 + Math.round(Math.sin(a) * 6);
        if (ax < x + 16 || ax > x + 27 || ay > y + 10) dot(ax, ay, paint, 0.9);
      }
      fill(x, y + 11, 44, 1, tint(0x8a5a2e, 0.75));
      // Legs, and the ball in its rack between the left pair
      for (const lx of [x + 1, x + 13, x + 41]) fill(lx, y + 12, 2, 4, tint(0x6e4a2e, 0.75));
      fill(x + 4, y + 15, 8, 1, 0x5a5f6c); dot(x + 4, y + 14, 0x8a8f9c); dot(x + 11, y + 14, 0x8a8f9c);
      drawBall(x + 8, y + 12, 2);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Santa's workbench. Pale wood on stout legs, a vice on its end, a toy soldier stood beside it half painted, wood
   * shavings on the top and the floor; every so often a little hammer gets up and taps.
   */
  workbench: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const front = 0x9a6e42, steel = 0x7a808c, shaving = 0xf0d8a8;
      fill(x, y, 44, 1, tint(0xead09a, 1)); fill(x, y + 1, 44, 4, tint(0xd8b07a, 0.9));
      for (const [gx, gy, gw] of [[3, 2, 6], [21, 3, 9], [34, 2, 7]]) fill(x + gx, y + gy, gw, 1, tint(0xc49a6c, 0.85));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      // Its rail, legs and the shelf below with offcuts on it
      fill(x, y + 5, 44, 2, tint(front, 0.72)); fill(x, y + 5, 44, 1, tint(0x7a5232, 0.62));
      for (const lx of [x + 1, x + 40]) { fill(lx, y + 7, 3, 9, tint(front, 0.72)); dot(lx, y + 7, tint(0x7a5232, 0.62)); }
      fill(x + 1, y + 12, 42, 2, tint(front, 0.72)); fill(x + 1, y + 12, 42, 1, tint(0xb08254, 0.72));
      fill(x + 8, y + 10, 6, 2, 0xc49a6c); fill(x + 16, y + 11, 4, 1, 0xd8b07a); fill(x + 28, y + 9, 2, 3, 0xb08254);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      // The vice, clamped to its end
      fill(x - 2, y + 1, 6, 3, steel); fill(x - 2, y + 1, 6, 1, 0xa8adb6); fill(x - 2, y + 4, 2, 3, steel);
      fill(x - 5, y + 5, 4, 1, 0x5a5f6c); dot(x - 5, y + 4, 0x5a5f6c); dot(x - 5, y + 6, 0x5a5f6c);
      // The toy soldier: hat, face, red coat, one leg painted blue and the other bare wood
      const sx = x + 1;
      fill(sx, y - 9, 3, 2, 0x1a1a1f); fill(sx, y - 7, 3, 1, 0xf0c8a0); fill(sx, y - 6, 3, 2, 0xc8302a); dot(sx + 1, y - 5, 0xf2d35b);
      dot(sx, y - 4, 0x3558ce); dot(sx, y - 3, 0x3558ce); dot(sx + 2, y - 4, 0xe0c090); dot(sx + 2, y - 3, 0xe0c090);
      fill(sx - 1, y - 2, 5, 1, 0x7a5232);
      // Shavings, curled on the top and fallen to the floor
      for (const [dx, dy] of [[30, 3], [31, 2], [33, 3], [7, 17], [8, 16], [22, 17], [35, 17], [36, 16]]) dot(x + dx, y + dy, shaving);
      // The hammer, lying by the keyboard, up and tapping now and then
      const t = (now + x * 0.19) % 15;
      if (t < 1.5) {
        const up = Math.floor(t * 4) % 2;
        fill(x + 31, y - 3 - up, 1, 5, 0x8a6040); fill(x + 30, y - 5 - up, 3, 2, steel);
        if (!up) { dot(x + 29, y + 1, shaving); dot(x + 33, y + 1, shaving); }
      } else {
        fill(x + 29, y + 2, 5, 1, 0x8a6040); fill(x + 33, y + 1, 2, 3, steel);
      }
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Coffin desk. A coffin on two trestles, its lid the desk's top, a sliver of purple satin showing under it, brass
   * handles along its side and a plate in the middle; now and then a bony hand pokes out from under the lid and waves.
   */
  coffin: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const lid = 0x5e3a34, side = 0x42282a, seam = 0x241618;
      // The trestles, under it
      for (const tx of [x + 6, x + 33]) {
        for (let k = 0; k < 6; k++) { dot(tx + k, y + 10 + k, 0x6e4a2e); dot(tx + 5 - k, y + 10 + k, 0x5a3b25); }
      }
      // The lid, its ends cut in, and its edge
      fill(x + 2, y, 40, 1, tint(0x7a4e44, 1)); fill(x + 1, y + 1, 42, 1, tint(lid, 0.9)); fill(x, y + 2, 44, 3, tint(lid, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 1, seam); fill(x + 3, y + 5, 30, 1, 0x7a4aa0); fill(x + 5, y + 5, 6, 1, 0x9a6ac0);
      // Its side, narrowing toward the foot
      fill(x + 1, y + 6, 43, 5, tint(side, 0.72)); fill(x + 2, y + 11, 40, 1, tint(side, 0.62));
      fill(x + 1, y + 6, 43, 1, tint(0x5a3836, 0.72)); fill(x + 1, y + 10, 43, 1, seam, 0.5);
      for (const hx of [x + 5, x + 16, x + 31]) { fill(hx, y + 8, 5, 1, PALETTE.brass); dot(hx, y + 7, 0x9a7a3a); dot(hx + 4, y + 7, 0x9a7a3a); }
      fill(x + 24, y + 7, 4, 3, 0xc8cdd6); dot(x + 25, y + 8, 0x8a8f9c);
      fill(x + 3, y + 16, 38, 1, SHADOW);
      // The hand, from under the lid at the foot end, its fingers waggling
      const t = (now + x * 0.31) % 19;
      if (t < 1.6) {
        const out = Math.min(1, t / 0.3, (1.6 - t) / 0.3), wag = Math.floor(now * 6) % 2, bone = 0xeae6dc;
        if (out > 0.5) {
          fill(x + 38, y + 5, 3, 2, bone);
          for (let f = 0; f < 3; f++) dot(x + 38 + f, y + 7 + ((f + wag) % 2), bone);
        } else {
          fill(x + 38, y + 5, 3, 1, bone);
        }
      }
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Candle-wax desk. Dark old wood, three candles burning at its back corner, and years of their wax run down its
   * front in long drips; a drop gathers at the end of one and falls, every so often.
   */
  waxdesk: {
    front({ x, y, s, tint }) {
      const wax = 0xeee4c8, waxShade = 0xcfc2a0, flick = candlelight(x * 0.1);
      glowAt(x + 3, y - 4, 9, 5, 0xffb04a, 0.08 * flick);
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x7e5a46, 1)); fill(x, y + 1, 44, 4, tint(0x5a3b2e, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(0x4a2e24, 0.72)); fill(x, y + 5, 44, 1, tint(0x3a2218, 0.62));
      for (const lx of [x + 1, x + 40]) fill(lx, y + 6, 3, 10, tint(0x3a2218, 0.7));
      // The candles, short and tall, their flames wavering
      for (const [cx, h, k] of [[x + 1, 6, 0], [x + 3, 4, 1], [x + 41, 3, 2]]) {
        fill(cx, y + 2 - h, 2, h, wax); fill(cx + 1, y + 2 - h, 1, h, waxShade);
        const lit = candlelight(k + x * 0.1);
        dot(cx, y + 1 - h, 0xffd85a, lit); dot(cx, y - h, 0xff8a3d, 0.6 * lit);
      }
      // Wax pooled on the top and run down the front
      fill(x, y + 3, 6, 2, wax); fill(x + 39, y + 3, 5, 2, wax);
      fill(x, y + 5, 7, 1, wax); fill(x + 38, y + 5, 6, 1, wax);
      for (const [dx, len] of [[1, 4], [3, 2], [5, 7], [13, 1], [20, 2], [39, 5], [42, 3]]) {
        fill(x + dx, y + 5, 1, len, wax); fill(x + dx, y + 5 + len, 2, 1, waxShade); dot(x + dx, y + 5 + len, wax);
      }
      // A drop, gathering at the end of the longest drip, then falling to the floor
      const t = (now + x * 0.23) % 7;
      if (t < 2) dot(x + 5, y + 13, wax, t / 2);
      else if (t < 2.4) dot(x + 5, y + 13 + Math.round(((t - 2) / 0.4) * 3), wax);
      fill(x + 1, y + 16, 42, 1, SHADOW); fill(x + 4, y + 16, 3, 1, wax, 0.7);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Haunted-wood desk. Grey warped boards, claw marks raked across its front, and a knot in the wood that, every so
   * often, opens a yellow eye, looks left and right, and shuts it again.
   */
  clawdesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const board = 0x6a6058, groove = 0x3e3630, light = 0x8a8078;
      fill(x, y, 44, 1, tint(0x9a9088, 1)); fill(x, y + 1, 44, 4, tint(0x7a7068, 0.9));
      for (const [gx, gy, gw] of [[2, 2, 9], [18, 3, 12], [33, 2, 8], [8, 4, 6]]) fill(x + gx, y + gy, gw, 1, tint(0x6a6058, 0.85));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 10, tint(board, 0.72)); fill(x, y + 5, 44, 1, tint(groove, 0.62));
      for (const py of [8, 11]) fill(x, y + py, 44, 1, tint(groove, 0.7));
      // Warped: a board's end lifting, a nail standing proud
      dot(x + 43, y + 7, light); dot(x + 43, y + 8, groove); dot(x + 21, y + 6, 0x9aa0a6);
      // Two sets of claw marks, three scratches each
      for (const [cx, cy] of [[x + 4, y + 6], [x + 22, y + 7]]) {
        for (let k = 0; k < 3; k++) {
          for (let d = 0; d < 6; d++) { dot(cx + k * 3 + d, cy + d, 0x241e1a); if (d < 5) dot(cx + k * 3 + d, cy + d + 1, light, 0.35); }
        }
      }
      // The knot, and its eye
      fill(x + 34, y + 9, 5, 3, 0x4a4038); fill(x + 35, y + 10, 3, 1, 0x2a2420);
      const t = (now + x * 0.41) % 11;
      if (t < 1.8) {
        const look = t < 0.6 ? 0 : t < 1.2 ? -1 : 1;
        fill(x + 35, y + 10, 3, 1, 0xf2d35b); dot(x + 36 + look, y + 10, 0x1a1a1f);
        glowAt(x + 36, y + 10, 3, 1, 0xf2d35b, 0.12);
      }
      // Its legs, carved and twisted
      for (const lx of [x + 2, x + 39]) { fill(lx, y + 15, 3, 1, tint(board, 0.72)); dot(lx + 1, y + 15, groove); }
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Pastel pink desk. Its top edged in a cream scalloped trim, two drawers with heart-shaped pulls, and a heart
   * sticker stuck between them, which catches the light now and then.
   */
  pinkdesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const top = 0xf8ccd8, face = 0xeea6ba, line = 0xcc7e96, cream = 0xfbeee6;
      fill(x, y, 44, 1, tint(0xfde0e8, 1)); fill(x, y + 1, 44, 4, tint(top, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 9, tint(face, 0.72));
      // The scalloped trim under the top
      fill(x, y + 5, 44, 1, cream);
      for (let k = 0; k < 44; k++) if (k % 4 !== 3) dot(x + k, y + 6, cream);
      for (let k = 1; k < 44; k += 4) dot(x + k, y + 7, cream);
      // Two drawers, each with a heart for a pull
      for (const dx of [3, 27]) {
        fill(x + dx, y + 8, 14, 5, tint(0xf4b6c8, 0.72));
        fill(x + dx, y + 8, 14, 1, line); fill(x + dx, y + 8, 1, 5, line); fill(x + dx + 13, y + 8, 1, 5, line);
        tinyHeart(x + dx + 6, y + 10, 0xd8344a);
      }
      // The heart sticker, between them
      smallHeart(x + 20, y + 9, 0xf06080, 1, null);
      dot(x + 21, y + 9, WHITE, (now + x * 0.17) % 10 < 0.4 ? 1 : 0.5);
      // Its legs, cream, a little curved
      for (const lx of [x + 1, x + 40]) { fill(lx, y + 14, 3, 2, tint(cream, 0.85)); dot(lx + 1, y + 15, 0xe8d0c8); }
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** Cyber desk. Black, with glowing cyan strips along its edges that brighten and dim, slowly. */
  cyberdesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const cyan = 0x4ad4e8, glow = 0.55 + 0.45 * Math.sin(now * 1.1 + x * 0.07);
      fill(x, y, 44, 1, tint(0x3a3d4a, 1)); fill(x, y + 1, 44, 3, tint(0x1c1e26, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 4, 44, 1, cyan, glow);
      fill(x + 1, y + 5, 42, 10, tint(0x121319, 0.72)); fill(x + 1, y + 5, 42, 1, tint(0x0a0b0f, 0.62));
      for (const px of [x + 15, x + 29]) fill(px, y + 6, 1, 7, 0x262833);
      fill(x, y + 5, 1, 11, cyan, glow * 0.9); fill(x + 43, y + 5, 1, 11, cyan, glow * 0.9);
      fill(x + 3, y + 13, 38, 1, cyan, glow * 0.6);
      dot(x + 38, y + 7, 0xff5aa8, 0.6 + 0.4 * glow); dot(x + 40, y + 7, 0x4ad4e8, 0.8);
      fill(x + 1, y + 16, 42, 1, SHADOW); fill(x + 4, y + 16, 36, 1, cyan, 0.14 * glow);
      drawKeyboard(s, y + 2);
    },
  },
  /** Submarine console. Riveted steel, two dials whose needles waver, a row of toggle switches, and a red light that blinks now and then. */
  subconsole: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const dark = 0x3a3f4a, rivet = 0xb0b8c4;
      fill(x, y, 44, 1, tint(0x9aa2b0, 1)); fill(x, y + 1, 44, 4, tint(0x7a8290, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 10, tint(0x545b68, 0.72)); fill(x, y + 5, 44, 1, tint(dark, 0.62)); fill(x, y + 14, 44, 1, tint(dark, 0.7));
      for (const [dx, dy] of [[1, 6], [42, 6], [1, 13], [42, 13], [21, 6], [21, 13]]) dot(x + dx, y + dy, rivet);
      // Two dials, their needles wavering
      for (const [dx, k] of [[6, 0], [14, 1]]) {
        disc(x + dx, y + 10, 3, dark); disc(x + dx, y + 10, 2.2, 0xeae6dc);
        const a = -0.6 + k * 0.9 + 0.15 * Math.sin(now * (0.9 + k * 0.4) + x);
        dot(x + dx + Math.round(Math.sin(a) * 2), y + 10 - Math.round(Math.cos(a) * 2), 0xc9302a); dot(x + dx, y + 10, 0x2a2a2e);
      }
      // Toggle switches, one flipped down; a green lamp over them
      for (let k = 0; k < 5; k++) { const sx = x + 24 + k * 3; fill(sx, y + 9, 2, 3, 0x2a2e36); fill(sx, k === 2 ? y + 11 : y + 8, 2, 1, 0xd8dce4); }
      dot(x + 26, y + 7, 0x6fe08a); dot(x + 32, y + 7, 0x6fe08a, 0.5);
      // The red light, blinking now and then
      const t = (now + x * 0.29) % 6, on = t < 0.9 && Math.floor(t * 5) % 2 === 0;
      fill(x + 38, y + 8, 2, 2, on ? 0xff3a2a : 0x6a2420);
      if (on) glowAt(x + 38, y + 8, 3, 2, 0xff3a2a, 0.25);
      fill(x + 2, y + 15, 3, 1, tint(dark, 0.7)); fill(x + 39, y + 15, 3, 1, tint(dark, 0.7));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Café table. A small round table in warm wood on one turned pedestal, its foot splayed on the floor, and a little
   * plant in a terracotta pot beside the monitor, its leaves stirring now and then.
   */
  cafetable: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const top = 0xb07a4a, edge = 0xd09a66, side = 0x7e5232, leg = 0x5a3a24;
      // The top: round, so its ends are cut in, and its rim
      fill(x + 3, y, 38, 1, tint(edge, 1)); fill(x + 1, y + 1, 42, 3, tint(top, 0.9)); fill(x + 3, y + 4, 38, 1, tint(top, 0.85));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x + 2, y + 4, 40, 1, tint(side, 0.72)); fill(x + 5, y + 5, 34, 1, tint(side, 0.62));
      // The pedestal, turned, and its foot
      fill(x + 20, y + 6, 4, 9, tint(leg, 0.72)); fill(x + 21, y + 6, 1, 9, tint(side, 0.8));
      fill(x + 19, y + 9, 6, 1, tint(side, 0.72));
      fill(x + 13, y + 15, 18, 1, tint(leg, 0.72)); dot(x + 12, y + 15, tint(side, 0.72)); dot(x + 31, y + 15, tint(side, 0.72));
      fill(x + 10, y + 16, 24, 1, SHADOW);
      // The little plant, left of the monitor
      const stir = (now + x * 0.19) % 12 < 0.5 ? 1 : 0;
      fill(x + 1, y - 3, 4, 3, 0xc8643a); fill(x + 1, y - 3, 4, 1, 0xe0804a);
      dot(x + 2, y - 4, PALETTE.leaf); dot(x + 1 - stir, y - 5, PALETTE.leaf); dot(x + 3, y - 5, PALETTE.leafDark);
      dot(x + 4 + stir, y - 6, PALETTE.leaf); dot(x + 2, y - 6, PALETTE.leaf); dot(x + 3, y - 7, PALETTE.leafDark);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Red lacquer desk. Glossy red, gold trim along its top and foot, gold cloud scrolls in its front corners; every
   * so often a glint runs along the trim.
   */
  lacquerdesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const red = 0xb8282c, deep = 0x861a20, gloss = 0xd84a44, gold = 0xe8b84a, goldDark = 0xb8862e;
      fill(x, y, 44, 1, tint(gloss, 1)); fill(x, y + 1, 44, 4, tint(red, 0.9)); fill(x + 2, y + 1, 30, 1, tint(gloss, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 10, tint(deep, 0.72));
      fill(x, y + 5, 44, 1, tint(gold, 1)); fill(x, y + 14, 44, 1, tint(goldDark, 1));
      // The cloud scrolls, one in each front corner, mirrored
      for (const [cx, dir] of [[x + 2, 1], [x + 41, -1]]) {
        for (const [dx, dy] of [[0, 1], [1, 1], [2, 1], [3, 2], [0, 2], [0, 3], [1, 4], [2, 4], [2, 3], [4, 1], [5, 1]]) dot(cx + dir * dx, y + 6 + dy, tint(gold, 1));
      }
      // A round gold medallion in the middle
      fill(x + 21, y + 8, 3, 1, tint(gold, 1)); fill(x + 21, y + 11, 3, 1, tint(goldDark, 1));
      fill(x + 20, y + 9, 1, 2, tint(gold, 1)); fill(x + 24, y + 9, 1, 2, tint(goldDark, 1)); dot(x + 22, y + 9, tint(0xd84a44, 1));
      // The glint along the trim
      const t = (now + x * 0.29) % 13;
      if (t < 1.2) {
        const gx = x + Math.round((t / 1.2) * 43);
        dot(gx, y + 5, 0xfff6d0); dot(gx - 1, y + 5, 0xffe08a, 0.7); dot(gx, y + 4, WHITE, 0.4);
      }
      for (const lx of [x + 2, x + 39]) { fill(lx, y + 15, 3, 1, tint(deep, 0.8)); dot(lx + 1, y + 15, tint(gold, 1)); }
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Love-letter desk. Deep red wood with a white lace runner across it and over its front, and a pink box of love
   * letters tied with a ribbon; now and then a letter peeks out of the box, and slips back in.
   */
  lacedesk: {
    front({ x, y, s, tint }) {
      const lace = 0xf6f0ea, hole = 0xd8ccc4;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xc04050, 1)); fill(x, y + 1, 44, 4, tint(0xa82838, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 10, tint(0x8a1c2a, 0.72)); fill(x, y + 5, 44, 1, tint(0x6a1420, 0.62));
      for (const lx of [x + 1, x + 40]) fill(lx, y + 6, 3, 9, tint(0x6a1420, 0.7));
      // The lace runner, along the top and hanging over the front, its edge scalloped and pierced
      fill(x + 18, y + 2, 16, 3, lace);
      for (let k = 0; k < 16; k += 2) dot(x + 18 + k, y + 3, hole);
      fill(x + 18, y + 5, 16, 3, lace);
      for (let k = 0; k < 16; k++) if (k % 3 !== 2) dot(x + 18 + k, y + 8, lace);
      for (let k = 1; k < 16; k += 3) { dot(x + 18 + k, y + 6, hole); dot(x + 18 + k, y + 9, lace); }
      // The box of love letters, its ribbon tied in a bow
      const box = 0xf2a6bc;
      const peek = (now + x * 0.29) % 14, up = peek < 2 ? Math.round(Math.sin((peek / 2) * Math.PI) * 3) : 0;
      if (up) { fill(x + 1, y - 4 - up, 4, up + 1, 0xf6f0ea); dot(x + 2, y - 4 - up, 0xd8243a); }
      fill(x, y - 4, 6, 5, box); fill(x, y - 4, 6, 1, 0xf8c8d6); fill(x + 5, y - 4, 1, 5, 0xd47e98);
      fill(x + 2, y - 4, 1, 5, 0xd8243a); fill(x, y - 2, 6, 1, 0xd8243a);
      dot(x + 1, y - 5, 0xd8243a); dot(x + 3, y - 5, 0xd8243a);
      fill(x + 3, y + 16, 38, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Barrel desk. Sun-bleached planks laid across two barrels, a nail or two standing proud, the barrels' iron hoops
   * and their staves; every so often a drip from one barrel's spigot.
   */
  barreldesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const plank = 0xd2b07a, light = 0xe8cc96, gap = 0x8a6a42, stave = 0x8a5a32, staveDark = 0x6a4224, hoop = 0x3e3a3a;
      // The barrels, bellied, their staves and hoops
      for (const bx of [x + 2, x + 31]) {
        for (let row = 0; row < 13; row++) {
          const bulge = row > 2 && row < 10 ? 0 : 1;
          fill(bx + bulge, y + 4 + row, 11 - bulge * 2, 1, tint(stave, 0.75));
          for (const sx of [3, 6, 9]) if (sx - bulge < 11 - bulge * 2) dot(bx + sx - (bulge && sx === 9 ? 1 : 0), y + 4 + row, tint(staveDark, 0.75));
        }
        for (const hy of [6, 13]) fill(bx, y + hy, 11, 1, tint(hoop, 0.9));
        fill(bx + 1, y + 7, 1, 6, tint(0xa87448, 0.8));
      }
      // The spigot on the right barrel, a drop gathering and falling
      fill(x + 30, y + 10, 2, 1, PALETTE.brass);
      const t = (now + x * 0.27) % 15;
      if (t < 1.5) dot(x + 30, y + 11, 0x9a6a2a, t / 1.5);
      else if (t < 1.8) dot(x + 30, y + 11 + Math.round(((t - 1.5) / 0.3) * 4), 0x9a6a2a);
      // The planks across them: three boards, the gaps between, their front edge
      fill(x, y, 44, 4, tint(plank, 0.95)); fill(x, y, 44, 1, tint(light, 1));
      fill(x + 15, y, 1, 4, tint(gap, 0.9)); fill(x + 30, y, 1, 4, tint(gap, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 4, 44, 1, tint(gap, 0.75));
      for (const nx of [x + 2, x + 13, x + 33, x + 41]) dot(nx, y + 2, 0x7a7f88);
      dot(x + 24, y + 3, gap); dot(x + 25, y + 3, gap);
      fill(x + 1, y + 17, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Disco desk. A black desk, its front scattered with glitter that catches the light in turn, and a string of tiny
   * lights along its edge, twinkling slowly.
   */
  discodesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x5a5f6c, 1)); fill(x, y + 1, 44, 3, tint(0x2a2c34, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 4, 44, 11, tint(0x17161e, 0.8)); fill(x, y + 14, 44, 1, 0x0d0d12);
      // The glitter, each fleck catching the light in its turn
      const flecks = [0xd8dce4, 0xf2d35b, 0xf28fb0, 0x9ad8f0];
      for (let k = 0; k < 28; k++) {
        const gx = x + 1 + ((k * 17) % 42), gy = y + 7 + ((k * 5) % 7);
        dot(gx, gy, flecks[k % 4], 0.3 + 0.3 * Math.max(0, Math.sin(now * 0.8 + k * 2.3)));
      }
      // The string of lights, on its wire along the edge
      for (let px = x + 1; px < x + 43; px++) dot(px, y + 4 + ((px - x) % 4 === 3 ? 1 : 0), 0x0d0d12);
      const bulbs = [0xf2d35b, 0xf28fb0, 0x86d6f0, 0x8ae07a];
      for (let k = 0; k < 11; k++) {
        const bx = x + 2 + k * 4, by = y + 5, c = bulbs[k % 4];
        const on = 0.5 + 0.5 * Math.sin(now * 1.1 + k * 1.9 + x * 0.1);
        dot(bx, by, c, 0.45 + 0.55 * on);
        if (on > 0.75) fill(bx - 1, by - 1, 3, 3, c, 0.18);
      }
      fill(x + 3, y + 16, 38, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Low table. A low table in dark wood on short, thick legs, knelt at on a flat cushion with tassels at its corners;
   * no chair.
   */
  lowtable: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const wood = 0x3a2a1e, edge = 0x5e4632, deep = 0x2a1e16, cushion = 0x3a5a4a, cushionTop = 0x4e7a62;
      // The cushion under the table, where they kneel
      fill(s.x - 8, y + 9, 17, 4, cushion); fill(s.x - 8, y + 9, 17, 1, cushionTop); fill(s.x - 7, y + 12, 15, 1, shaded(cushion, 0.75));
      for (const cx of [s.x - 9, s.x + 9]) { dot(cx, y + 9, 0xd89a3a); dot(cx, y + 12, 0xd89a3a); }
      fill(s.x - 8, y + 13, 17, 1, SHADOW);
      // The top, its thick edge, the apron and the legs
      fill(x, y, 44, 1, tint(edge, 1)); fill(x, y + 1, 44, 3, tint(wood, 0.95)); fill(x + 2, y + 1, 28, 1, tint(0x4a3626, 1));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 4, 44, 1, tint(deep, 0.8)); fill(x + 2, y + 5, 40, 2, tint(wood, 0.72));
      for (const lx of [x + 2, x + 38]) { fill(lx, y + 5, 4, 6, tint(wood, 0.8)); fill(lx, y + 5, 1, 6, tint(edge, 0.8)); }
      fill(x + 1, y + 11, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Crate desk. A rough plank laid across an upturned slatted crate and a sawn stump, nails in it, and a jar lamp
   * glowing at its end, flickering softly.
   */
  cratedesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const plank = 0xa07444, light = 0xbc8e58, dark = 0x6a4a2c, slat = 0xc49a62, slatDark = 0x8a6a42;
      // The stump on the left: bark, its rings on top hidden under the plank
      fill(x + 3, y + 4, 8, 12, tint(0x6a4a2e, 0.8)); fill(x + 3, y + 4, 1, 12, tint(0x8a6440, 0.8)); fill(x + 7, y + 6, 1, 8, tint(0x4a3020, 0.8));
      // The crate on the right, upturned: slats, its corner posts and a brace across
      fill(x + 22, y + 4, 21, 12, tint(slatDark, 0.8));
      for (const sy of [4, 8, 12]) { fill(x + 22, y + sy, 21, 3, tint(slat, 0.85)); fill(x + 22, y + sy, 21, 1, tint(0xd8b07a, 0.9)); }
      line(x + 24, y + 14, x + 40, y + 5, tint(dark, 0.85));
      for (const px of [x + 22, x + 41]) fill(px, y + 4, 2, 12, tint(dark, 0.85));
      for (const [nx, ny] of [[23, 5], [42, 5], [23, 14], [42, 14]]) dot(x + nx, y + ny, 0x5a5048);
      // The plank, a little longer than the desk, its grain and a knot
      fill(x - 1, y, 46, 1, tint(light, 1)); fill(x - 1, y + 1, 46, 3, tint(plank, 0.95)); fill(x - 1, y + 4, 46, 1, tint(dark, 0.8));
      fill(x + 20, y + 2, 9, 1, tint(shaded(plank, 0.88), 1)); dot(x + 33, y + 2, tint(dark, 1)); dot(x + 34, y + 2, tint(dark, 1));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      // The jar lamp on the plank's left end
      const glow = candlelight(x * 0.07);
      glowAt(x + 2, y - 3, isNight ? 7 : 4, isNight ? 6 : 4, 0xffc060, (isNight ? 0.22 : 0.08) * glow);
      fill(x, y - 6, 5, 6, 0xcfe4e8, 0.45); fill(x, y - 6, 1, 6, WHITE, 0.4); fill(x + 4, y - 6, 1, 6, 0x8aa4aa, 0.6);
      fill(x, y - 7, 5, 1, 0x8a8f9c); fill(x + 1, y - 8, 3, 1, 0x6a6e78);
      dot(x + 2, y - 4, 0xffe08a, glow); dot(x + 2, y - 3, 0xffc060, 0.8 * glow); dot(x + 1, y - 3, 0xffc060, 0.4 * glow);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Scriptorium desk. Dark oak with carved front panels and brass pulls, a taper burning at one end whose flame wavers, and a quill that dips into a pot of ink every so often.
   */
  scriptorium: {
    front({ x, y, s, tint }) {
      const oak = 0x3e2a1e, oakLight = 0x5e4230, oakDark = 0x24160e, brass = 0xc8a050;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(oakLight, 1)); fill(x, y + 1, 44, 4, tint(oak, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(oak, 0.8)); fill(x, y + 5, 44, 1, tint(oakDark, 0.8)); fill(x, y + 15, 44, 1, tint(oakDark, 0.7));
      // The carved panels, with brass pulls and a keyhole
      for (const px of [x + 3, x + 16, x + 29]) { fill(px, y + 7, 12, 7, tint(oakDark, 0.85)); fill(px + 1, y + 8, 10, 5, tint(oak, 0.75)); fill(px, y + 7, 12, 1, tint(oakLight, 0.75)); }
      for (const px of [x + 9, x + 22]) { fill(px, y + 10, 2, 1, brass); dot(px, y + 10, 0xe8c878); }
      dot(x + 35, y + 9, brass); dot(x + 35, y + 10, 0x1a1008);
      // The taper on the left corner, flame wavering
      const flick = candlelight(x * 0.1);
      fill(x + 1, y - 4, 2, 4, 0xeee4c8); dot(x + 2, y - 4, 0xcfc2a0, 0.0); fill(x, y - 1, 4, 1, brass);
      dot(x + 1, y - 5, 0xffd85a, flick); dot(x + 1, y - 6, 0xff8a3d, 0.6 * flick);
      glowAt(x + 2, y - 5, 8, 6, 0xffb04a, 0.08 * flick);
      if (isNight) glowLate(() => { glowAt(x + 2, y - 5, 9, 7, 0xffb04a, 0.16 * flick); dot(x + 1, y - 5, 0xfff4c0); });
      // A quill in an ink pot at the right, dipped now and then
      const t = (now + x * 0.29) % 18, dip = t < 1.5 ? 1 : 0;
      fill(x + 38, y - 2, 3, 2, 0x2a3350); dot(x + 38, y - 2, 0x4a5a80);
      line(x + 39 + dip, y - 3 + dip, x + 42, y - 8, 0xeae6dc); dot(x + 42, y - 8, WHITE, 0.8);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Captain's chart table. Polished mahogany with a brass rail, a nautical chart pinned across its front showing a coast and a dotted course, and a little red pin that crawls along the course now and then.
   */
  charttable: {
    front({ x, y, s, tint }) {
      const wood = 0x6a3a28, woodLight = 0x8a5238, woodDark = 0x3e2018, brass = 0xc8a050, brassLight = 0xe8c878;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodLight, 1)); fill(x, y + 1, 44, 4, tint(wood, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(wood, 0.8)); fill(x, y + 15, 44, 1, tint(woodDark, 0.8));
      fill(x, y + 5, 44, 1, brass); fill(x, y + 5, 44, 1, brassLight, 0.0); for (const bx of [x + 1, x + 22, x + 42]) dot(bx, y + 5, brassLight);
      // The chart: parchment, a ragged coast, a dotted course and a compass rose
      const sea = 0xd8cca0, land = 0xa89868, ink = 0x5a4a30;
      fill(x + 4, y + 7, 36, 8, sea); fill(x + 4, y + 7, 36, 1, 0xeae0b8); fill(x + 4, y + 14, 36, 1, 0xb0a478);
      for (const [lx, ly, lw] of [[5, 8, 5], [5, 9, 7], [5, 10, 4], [30, 13, 8], [33, 12, 6], [35, 11, 4]]) fill(x + lx, y + ly, lw, 1, land);
      const course = [[13, 12], [17, 10], [21, 11], [25, 9], [28, 11], [31, 10]];
      course.forEach(([cx, cy], i) => { if (i % 1 === 0) dot(x + cx, y + cy, ink, 0.7); });
      for (let k = 0; k < course.length - 1; k++) dot(x + Math.round((course[k][0] + course[k + 1][0]) / 2), y + Math.round((course[k][1] + course[k + 1][1]) / 2), ink, 0.45);
      dot(x + 17, y + 9, ink, 0.0); dot(x + 36, y + 8, 0xa83a30); dot(x + 35, y + 8, ink, 0.6); dot(x + 37, y + 8, ink, 0.6); dot(x + 36, y + 9, ink, 0.6);
      // The pin, creeping along the course for 5 s every 20 s, then waiting at its place
      const t = (now + x * 0.3) % 20, prog = Math.min(1, t / 5), leg = Math.min(course.length - 2, Math.floor(prog * (course.length - 1)));
      const a = course[leg], b = course[leg + 1], f = prog * (course.length - 1) - leg;
      dot(x + Math.round(a[0] + (b[0] - a[0]) * f), y + Math.round(a[1] + (b[1] - a[1]) * f), 0xd8402a); dot(x + Math.round(a[0] + (b[0] - a[0]) * f), y + Math.round(a[1] + (b[1] - a[1]) * f) - 1, 0xff8a70, 0.8);
      // Brass dividers resting on the desktop
      line(x + 38, y - 1, x + 40, y - 5, brass); line(x + 41, y - 1, x + 40, y - 5, brass); dot(x + 40, y - 5, brassLight);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** Trading desk. A white desk with a graphite front where a slim LED ticker strip scrolls along, green and red, over a cable tray; it glows faintly at night. */
  tradingdesk: {
    front({ x, y, s, tint }) {
      const top = 0xe8eaee, topLight = 0xf8f9fb, front = 0x2e323c, frontLight = 0x444a56;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(topLight, 1)); fill(x, y + 1, 44, 4, tint(top, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(front, 0.85)); fill(x, y + 5, 44, 1, tint(frontLight, 0.85)); fill(x, y + 15, 44, 1, 0x1c1f26);
      // The ticker strip: little green and red dashes scrolling left
      fill(x + 3, y + 8, 38, 3, 0x080a0e);
      const off = Math.floor(now * 5 + x);
      for (let k = 0; k < 8; k++) {
        const slot = Math.floor(off / 5) + k, px = x + 3 + k * 5 - (off % 5), h = hash(slot, 3), c = h % 3 ? 0x3ad87a : 0xe8483e;
        if (px >= x + 3 && px + 2 <= x + 41) { fill(px, y + 9, 2 + (h >>> 4) % 2, 1, c); if (h % 3 && px + 3 <= x + 41) dot(px + 1, y + 8, c, 0.6); }
      }
      // The cable tray and its legs
      fill(x + 2, y + 12, 2, 3, 0x1c1f26); fill(x + 40, y + 12, 2, 3, 0x1c1f26); fill(x + 6, y + 13, 32, 1, 0x3a404c);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      if (isNight) glowLate(() => glowAt(x + 22, y + 9, 20, 3, 0x60e0a0, 0.06));
      drawKeyboard(s, y + 2);
    },
  },
  /** Bull desk. A glossy emerald desk with a gold edge and a gold pair of horns on its front; every 12 s a sparkle runs along the gold trim. */
  bulldesk: {
    front({ x, y, s, tint }) {
      const green = 0x1f7a4a, greenLight = 0x2e9a5e, greenDark = 0x125232, gold = 0xd8b04a, goldLight = 0xf4d878, goldDark = 0x9a7a2a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, goldLight); fill(x, y + 1, 44, 4, tint(green, 0.95)); fill(x, y + 1, 44, 1, tint(greenLight, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(green, 0.82)); fill(x, y + 5, 44, 1, gold); fill(x, y + 15, 44, 1, tint(greenDark, 0.85));
      fill(x, y + 5, 1, 11, tint(greenLight, 0.8)); fill(x + 43, y + 5, 1, 11, tint(greenDark, 0.8));
      // The horns on its front: a gold boss with two curving up
      const cx = x + 22;
      fill(cx - 2, y + 9, 5, 4, gold); fill(cx - 2, y + 9, 5, 1, goldLight); fill(cx - 1, y + 12, 3, 1, goldDark);
      fill(cx - 5, y + 9, 3, 1, gold); dot(cx - 6, y + 8, gold); dot(cx - 7, y + 7, goldLight);
      fill(cx + 3, y + 9, 3, 1, gold); dot(cx + 6, y + 8, gold); dot(cx + 7, y + 7, goldLight);
      dot(cx - 1, y + 10, greenDark); dot(cx + 1, y + 10, greenDark);
      // The sparkle along the trim
      const t = (now + x * 0.29) % 12;
      if (t < 1.5) { const sx = x + Math.floor((t / 1.5) * 43); dot(sx, y + 5, WHITE); dot(sx, y, WHITE, 0.8); }
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** Cold desk. A tired grey desk, a plaid blanket hung over its front corner, a dim little lamp on its other end that flickers now and then, and a small heater underneath with its bars glowing orange. */
  heaterdesk: {
    front({ x, y, s, tint }) {
      const top = 0x6a6e76, topLight = 0x868a92, front = 0x52565e, frontDark = 0x3a3d44, plaid = 0x8a2a2a, plaidLine = 0x3a3a4a;
      // The lamp on the left corner: a bent arm and a small shade, dim, flickering now and then
      const flick = (now + x * 0.29) % 23 < 0.6 && Math.floor(now * 9) % 2 === 0 ? 0.3 : 1;
      fill(x, y - 1, 5, 1, 0x3a3d44); fill(x + 3, y - 7, 1, 6, 0x4a4e58); line(x + 3, y - 7, x + 1, y - 9, 0x4a4e58);
      fill(x - 1, y - 11, 4, 2, 0x3e6a4e); dot(x - 1, y - 11, 0x5a8a6a); dot(x + 1, y - 9, 0xffd890, flick);
      glowAt(x + 1, y - 6, 5, 4, 0xffd890, 0.05 * flick);
      if (isNight) glowLate(() => { glowAt(x + 1, y - 7, 4, 3, 0xffc870, 0.1 * flick); dot(x + 1, y - 9, 0xfff0c0, flick); });
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(topLight, 1)); fill(x, y + 1, 44, 4, tint(top, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(front, 0.85)); fill(x, y + 5, 44, 1, tint(frontDark, 0.85)); fill(x, y + 15, 44, 1, tint(frontDark, 0.8));
      // The heater under it, its bars glowing
      fill(x + 4, y + 9, 10, 6, 0xd8d4cc); fill(x + 4, y + 9, 10, 1, 0xf0ece4); fill(x + 5, y + 10, 8, 4, 0x2a1a14);
      const glow = 0.75 + 0.25 * Math.sin(now * 1.3 + x);
      for (const by of [11, 13]) fill(x + 5, y + by, 8, 1, 0xff7a2a, glow);
      glowAt(x + 9, y + 13, 7, 4, 0xff8a3a, isNight ? 0 : 0.08);
      if (isNight) glowLate(() => { glowAt(x + 9, y + 12, 6, 3, 0xff8a3a, 0.1); for (const by of [11, 13]) fill(x + 5, y + by, 8, 1, 0xff9a4a, 0.7 * glow); });
      // The blanket thrown over the right end
      fill(x + 34, y + 4, 10, 8, plaid); fill(x + 34, y + 4, 10, 1, shaded(plaid, 1.25)); fill(x + 35, y + 12, 8, 2, plaid);
      for (const px of [36, 40]) fill(x + px, y + 4, 1, 10, plaidLine, 0.55); fill(x + 34, y + 8, 10, 1, plaidLine, 0.5);
      fill(x + 35, y + 13, 8, 1, shaded(plaid, 0.7)); fill(x + 43, y + 5, 1, 7, shaded(plaid, 0.75));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A wall of screens. A black desk under six screens, four small ones in a row along the top and one beside the main one, each with a light strip along its top in its own colour of a slowly turning rainbow; an RGB racing chair, and a light strip along the desk's foot. */
  screenwall: {
    back({ s, y }) {
      const shell = 0x1d1f25, glow = rainbow((now * 0.15) % 1);
      fill(s.x - 4, y - 15, 9, 15, shell); fill(s.x - 6, y - 8, 13, 7, shell);
      fill(s.x - 6, y - 8, 1, 7, glow); fill(s.x + 6, y - 8, 1, 7, glow); fill(s.x - 4, y - 15, 9, 1, glow);
      fill(s.x - 1, y - 14, 3, 1, 0x3a3d47);
    },
    front({ x, y, s, tint }) {
      // The small screens, backs to us, each with a halo in its colour
      const smalls = [[0, -17], [6, -17], [12, -17], [18, -17], [0, -10]];
      smalls.forEach(([dx, dy], i) => {
        const c = rainbow((now * 0.15 + i / 6) % 1);
        fill(x + dx, y + dy, 5, 5, PALETTE.monitor); fill(x + dx, y + dy, 5, 1, c); fill(x + dx + 1, y + dy + 1, 3, 3, PALETTE.monitorPanel);
        if (isNight) glowLate(() => { glowAt(x + dx + 2, y + dy - 1, 4, 2, c, 0.06); fill(x + dx, y + dy, 5, 1, c); });
      });
      fill(x + 2, y - 12, 1, 2, PALETTE.monitorStand); fill(x + 9, y - 12, 1, 2, PALETTE.monitorStand); fill(x + 15, y - 12, 1, 2, PALETTE.monitorStand);
      fill(x + 2, y - 5, 1, 5, PALETTE.monitorStand);
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x3a3e48, 1)); fill(x, y + 1, 44, 4, tint(0x22252d, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 10, tint(0x1a1c22, 0.85)); fill(x, y + 5, 44, 1, tint(0x2e323c, 0.85));
      // The light strip along its foot, colours running along it
      for (let i = 0; i < 42; i++) dot(x + 1 + i, y + 14, rainbow((now * 0.15 + i / 42) % 1));
      fill(x + 3, y + 7, 1, 7, 0x2a2e36); fill(x + 40, y + 7, 1, 7, 0x2a2e36);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      if (isNight) glowLate(() => glowAt(x + 22, y + 15, 22, 3, rainbow((now * 0.15 + 0.5) % 1), 0.12));
      drawKeyboard(s, y + 2);
      for (let i = 0; i < 8; i++) dot(s.x - 4 + i, y + 2, rainbow((now * 0.15 + i / 8) % 1));
    },
  },
  /** A matchday desk. A white desk with a navy front and a red stripe down it edged in white, a small navy and red badge on the stripe (brand nod: PSG), and a corner flag in the left end whose red pennant flutters now and then. */
  pitchdesk: {
    front({ x, y, s, tint }) {
      // The corner flag on the left end
      const flutter = (now + x * 0.23) % 9 < 2 ? Math.floor(now * 5) % 2 : 0;
      fill(x + 1, y - 9, 1, 9, 0xeef0f4); fill(x + 2, y - 9, 3 + flutter, 2, PSG_RED); dot(x + 2, y - 9, shaded(PSG_RED, 1.3));
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xffffff, 1)); fill(x, y + 1, 44, 4, tint(0xe4e8ee, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(PSG_NAVY, 0.95)); fill(x, y + 5, 44, 1, tint(PSG_NAVY_LIGHT, 0.95)); fill(x, y + 15, 44, 1, tint(PSG_NAVY_DARK, 0.95));
      fill(x + 29, y + 5, 1, 11, WHITE); fill(x + 30, y + 5, 3, 11, PSG_RED); fill(x + 33, y + 5, 1, 11, WHITE);
      psgNod(x + 12, y + 10, 4);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** The Lakers scorer's table (brand nod: Lakers). A wood top over a front in purple with a gold band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  lakersdesk: {
    front(d) { nbaDesk(NBA_TEAMS.lakers, d); },
  },
  /** The Celtics scorer's table (brand nod: Celtics). A wood top over a front in green with a white band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  celticsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.celtics, d); },
  },
  /** The Warriors scorer's table (brand nod: Warriors). A wood top over a front in blue with a yellow band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  warriorsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.warriors, d); },
  },
  /** The Knicks scorer's table (brand nod: Knicks). A wood top over a front in blue with a orange band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  knicksdesk: {
    front(d) { nbaDesk(NBA_TEAMS.knicks, d); },
  },
  /** The Bulls scorer's table (brand nod: Bulls). A wood top over a front in red with a black band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  bullsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.bulls, d); },
  },
  /** The Heat scorer's table (brand nod: Heat). A wood top over a front in red with a orange band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  heatdesk: {
    front(d) { nbaDesk(NBA_TEAMS.heat, d); },
  },
  /** The Spurs scorer's table (brand nod: Spurs). A wood top over a front in black with a silver band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  spursdesk: {
    front(d) { nbaDesk(NBA_TEAMS.spurs, d); },
  },
  /** The Thunder scorer's table (brand nod: Thunder). A wood top over a front in blue with a orange band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  thunderdesk: {
    front(d) { nbaDesk(NBA_TEAMS.thunder, d); },
  },
  /** The Timberwolves scorer's table (brand nod: Timberwolves). A wood top over a front in navy with a green band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  wolvesdesk: {
    front(d) { nbaDesk(NBA_TEAMS.wolves, d); },
  },
  /** The Nuggets scorer's table (brand nod: Nuggets). A wood top over a front in navy with a gold band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  nuggetsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.nuggets, d); },
  },
  /** The Hawks scorer's table (brand nod: Hawks). A wood top over a front in red with a volt band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  hawksdesk: {
    front(d) { nbaDesk(NBA_TEAMS.hawks, d); },
  },
  /** The Nets scorer's table (brand nod: Nets). A wood top over a front in black with a white band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  netsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.nets, d); },
  },
  /** The Hornets scorer's table (brand nod: Hornets). A wood top over a front in teal with a purple band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  hornetsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.hornets, d); },
  },
  /** The Cavaliers scorer's table (brand nod: Cavaliers). A wood top over a front in wine with a gold band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  cavsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.cavs, d); },
  },
  /** The Mavericks scorer's table (brand nod: Mavericks). A wood top over a front in blue with a silver band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  mavsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.mavs, d); },
  },
  /** The Pistons scorer's table (brand nod: Pistons). A wood top over a front in blue with a red band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  pistonsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.pistons, d); },
  },
  /** The Rockets scorer's table (brand nod: Rockets). A wood top over a front in red with a silver band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  rocketsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.rockets, d); },
  },
  /** The Pacers scorer's table (brand nod: Pacers). A wood top over a front in navy with a gold band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  pacersdesk: {
    front(d) { nbaDesk(NBA_TEAMS.pacers, d); },
  },
  /** The Clippers scorer's table (brand nod: Clippers). A wood top over a front in red with a blue band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  clippersdesk: {
    front(d) { nbaDesk(NBA_TEAMS.clippers, d); },
  },
  /** The Grizzlies scorer's table (brand nod: Grizzlies). A wood top over a front in navy with a blue band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  grizzliesdesk: {
    front(d) { nbaDesk(NBA_TEAMS.grizzlies, d); },
  },
  /** The Bucks scorer's table (brand nod: Bucks). A wood top over a front in green with a cream band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  bucksdesk: {
    front(d) { nbaDesk(NBA_TEAMS.bucks, d); },
  },
  /** The Pelicans scorer's table (brand nod: Pelicans). A wood top over a front in navy with a red band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  pelicansdesk: {
    front(d) { nbaDesk(NBA_TEAMS.pelicans, d); },
  },
  /** The Magic scorer's table (brand nod: Magic). A wood top over a front in blue with a black band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  magicdesk: {
    front(d) { nbaDesk(NBA_TEAMS.magic, d); },
  },
  /** The 76ers scorer's table (brand nod: 76ers). A wood top over a front in blue with a red band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  sixersdesk: {
    front(d) { nbaDesk(NBA_TEAMS.sixers, d); },
  },
  /** The Suns scorer's table (brand nod: Suns). A wood top over a front in purple with a orange band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  sunsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.suns, d); },
  },
  /** The Trail Blazers scorer's table (brand nod: Trail Blazers). A wood top over a front in red with a black band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  blazersdesk: {
    front(d) { nbaDesk(NBA_TEAMS.blazers, d); },
  },
  /** The Kings scorer's table (brand nod: Kings). A wood top over a front in purple with a silver band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  kingsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.kings, d); },
  },
  /** The Raptors scorer's table (brand nod: Raptors). A wood top over a front in red with a black band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  raptorsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.raptors, d); },
  },
  /** The Jazz scorer's table (brand nod: Jazz). A wood top over a front in navy with a yellow band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  jazzdesk: {
    front(d) { nbaDesk(NBA_TEAMS.jazz, d); },
  },
  /** The Wizards scorer's table (brand nod: Wizards). A wood top over a front in navy with a red band and the badge, and a shot clock that counts down for a few seconds every 40 s. */
  wizardsdesk: {
    front(d) { nbaDesk(NBA_TEAMS.wizards, d); },
  },
  /** An Avengers briefing desk (brand nod: Marvel). A silver top over a navy front with a red and white band, the A badge on a white disc, and Cap's shield leaning at its right end; a glint runs over the shield every 16 s. */
  marvel1deck: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xdfe4ec, 1)); fill(x, y + 1, 44, 4, tint(0xa8b0bc, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(0x1c2e5e, 0.95)); fill(x, y + 5, 44, 1, tint(0x34508a, 0.95)); fill(x, y + 15, 44, 1, tint(0x101c3c, 0.95));
      fill(x, y + 8, 44, 2, M1_RED); fill(x, y + 10, 44, 1, M1_CREAM, 0.8); fill(x, y + 8, 44, 1, M1_RED_LIGHT, 0.6);
      // The badge, and the shield leaning at the end
      disc(x + 11, y + 10, 5, M1_CREAM); m1A(x + 11, y + 9, M1_RED_DARK);
      fill(x + 33, y + 16, 10, 1, SHADOW); m1Shield(x + 37, y + 11, 5);
      const g = (now + x * 0.29) % 16;
      if (g < 0.9) { const k = Math.floor(g * 9); line(x + 33 + k, y + 14, x + 35 + k, y + 8, WHITE, 0.55); }
      fill(x + 1, y + 16, 32, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A Milano console (brand nod: Marvel): Star-Lord's ship at a desk, a steel-blue hull front with orange flashes and a yellow pinstripe, a pointed orange nose at the right end, a row of indicator lights running along it and a vent below. */
  marvel2milano: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xa8b0c0, 1)); fill(x, y + 1, 44, 4, tint(0x6a7484, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(M2_HULL, 0.95)); fill(x, y + 5, 44, 1, tint(M2_HULL_LIGHT, 0.95)); fill(x, y + 15, 44, 1, tint(M2_HULL_DARK, 0.95));
      // Orange flashes sweeping back from the nose, the yellow pinstripe
      for (let k = 0; k < 5; k++) fill(x + 2 + k * 2, y + 6 + k, 6, 1, M2_ORANGE);
      fill(x, y + 12, 34, 1, 0xf2c040, 0.9);
      fill(x + 34, y + 5, 10, 11, M2_ORANGE); fill(x + 34, y + 5, 10, 1, 0xf8a060); fill(x + 43, y + 6, 1, 10, 0xb8501a);
      for (let k = 0; k < 4; k++) fill(x + 34, y + 9 + k, 1 + k, 1, M2_HULL);
      // The lights, one running along, and the vent
      const run = Math.floor(now * 3 + x) % 6;
      for (let k = 0; k < 6; k++) dot(x + 16 + k * 3, y + 8, k === run ? 0x7affc0 : 0x2a4a3a);
      for (let k = 0; k < 4; k++) fill(x + 18 + k * 3, y + 14, 2, 1, M2_HULL_DARK);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** Tony's workbench (brand nod: Marvel): a scuffed steel top with a vise clamped at its left end, a hot-rod red tool chest below with gold-handled drawers and an arc-blue light strip under the top; every 18 s the vise throws a shower of welding sparks. */
  ironmanbench: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(IM_STEEL_LIGHT, 1)); fill(x, y + 1, 44, 4, tint(IM_STEEL, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      for (const sx of [20, 31]) line(x + sx, y + 2, x + sx + 3, y + 3, IM_STEEL_DARK, 0.6);
      fill(x, y + 5, 44, 1, IM_ARC, 0.85); if (isNight) glowLate(() => fill(x, y + 5, 44, 1, IM_ARC_WHITE, 0.7));
      fill(x, y + 6, 44, 10, tint(IM_RED, 0.95)); fill(x, y + 6, 1, 10, tint(IM_RED_LIGHT, 0.95)); fill(x + 43, y + 6, 1, 10, tint(IM_RED_DARK, 0.95)); fill(x, y + 15, 44, 1, tint(IM_RED_DARK, 0.95));
      for (const [dy, h] of [[7, 2], [10, 2], [13, 2]]) { fill(x + 2, y + dy + h, 40, 1, IM_RED_DARK); for (const hx of [8, 20, 32]) fill(x + hx, y + dy, 4, 1, IM_GOLD); }
      // The vise on the left end, and its sparks now and then
      fill(x - 1, y - 4, 6, 4, IM_STEEL_DARK); fill(x - 1, y - 4, 6, 1, IM_STEEL); fill(x + 1, y - 6, 2, 2, IM_STEEL_DARK); fill(x - 2, y - 3, 1, 2, IM_STEEL);
      const t = (now + x * 0.29) % 18;
      if (t < 1.5) {
        const burst = () => { for (let k = 0; k < 6; k++) { const h = Math.imul(k + 1 + Math.floor(now * 10), 2654435761) >>> 0; dot(x + 2 + ((h >>> 3) % 7) - 3, y - 6 - ((h >>> 9) % 5), k % 2 ? 0xfff0a0 : 0xffb040); } glowAt(x + 2, y - 6, 4, 3, 0xffd070, 0.2); };
        if (isNight) glowLate(burst); else burst();
      }
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A Batcomputer console (brand nod: DC): a black console with a sloped front of blue-lit key panels and readouts, the yellow bat emblem in the middle and a row of status lights that scan along it; now and then one goes red. */
  batcaveconsole: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x3a3c46, 1)); fill(x, y + 1, 44, 4, tint(0x24262e, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(BC_BLACK, 0.95)); fill(x, y + 5, 44, 1, tint(0x2e3038, 0.95)); fill(x, y + 15, 44, 1, tint(0x0c0c10, 0.95));
      // Key panels either side, lit blue
      for (const px of [2, 30]) { fill(x + px, y + 7, 12, 6, 0x1a2a44); for (let r = 0; r < 3; r++) for (let c = 0; c < 6; c++) dot(x + px + 1 + c * 2, y + 8 + r * 2, (r + c + Math.floor(now)) % 5 ? BC_SCREEN : BC_SCREEN_LIGHT, 0.85); }
      bcEmblem(x + 22, y + 10);
      const scan = Math.floor(now * 3 + x) % 11, alert = (now + x * 0.3) % 21 < 1.5;
      for (let k = 0; k < 11; k++) dot(x + 17 + k, y + 6, k === scan ? (alert ? 0xff3a3a : 0x6affb0) : 0x1e3a2e);
      if (isNight) glowLate(() => { for (const px of [2, 30]) fill(x + px, y + 7, 12, 6, BC_SCREEN, 0.15); });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A Daily Planet desk (brand nod: DC): a reporter's warm oak desk with a brass-trimmed front carrying the gold globe-and-ring of the Daily Planet, a stack of newspapers and a PRESS card at its left end; the globe's ring catches the light every 18 s. */
  dcmixplanetdesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xc89a6a, 1)); fill(x, y + 1, 44, 4, tint(0xa87a4a, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(0x7a5232, 0.95)); fill(x, y + 5, 44, 1, tint(0x9a6a42, 0.95)); fill(x, y + 15, 44, 1, tint(0x4a3020, 0.95));
      fill(x + 2, y + 7, 18, 7, 0x6a4628); fill(x + 24, y + 7, 18, 7, 0x6a4628); fill(x + 2, y + 7, 18, 1, 0x8a5e3a); fill(x + 24, y + 7, 18, 1, 0x8a5e3a);
      // The globe and its ring
      disc(x + 22, y + 10, 4, DC_GOLD); disc(x + 21, y + 9, 2, 0xf8d880, 0.6); line(x + 18, y + 10, x + 26, y + 10, DC_GOLD_DARK); fill(x + 22, y + 6, 1, 9, DC_GOLD_DARK, 0.7);
      ellipseRing(x + 22, y + 10, 7, 2, DC_GOLD); const g = (now + x * 0.29) % 18; if (g < 1) dot(x + 15 + Math.floor(g * 14), y + 10 + (g < 0.5 ? 2 : -2), WHITE);
      // The papers and the press card on the left end
      fill(x + 1, y - 3, 8, 3, 0xeae6dc); fill(x + 1, y - 3, 8, 1, WHITE); fill(x + 2, y - 2, 6, 1, 0x9a968c); fill(x + 1, y - 1, 8, 1, 0xd8d4c8);
      fill(x + 2, y - 5, 3, 2, WHITE); fill(x + 2, y - 5, 3, 1, DC_RED);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A Poké Ball desk: a white top over a front that is a Poké Ball laid on its side, red above a black band and white below, with the big button in the middle; every 20 s the button lights and pulses twice, and it glows softly at night. */
  pokemonballdesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xfaf8f4, 1)); fill(x, y + 1, 44, 4, tint(0xe8e6e0, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      // The front: red above, the band, white below, the ends rounded off in shade
      fill(x, y + 5, 44, 4, PKM_RED); fill(x, y + 5, 44, 1, PKM_RED_LIGHT);
      fill(x, y + 9, 44, 2, PKM_BAND);
      fill(x, y + 11, 44, 5, PKM_WHITE); fill(x, y + 15, 44, 1, PKM_GREY);
      for (const [ex, k] of [[x, 1.12], [x + 43, 0.8]]) { fill(ex, y + 5, 1, 4, shaded(PKM_RED, k)); fill(ex, y + 11, 1, 5, shaded(PKM_WHITE, k)); }
      const c = (now + x * 0.31) % 20, lit = c < 1.6 && Math.floor(c * 4) % 2 === 0;
      disc(x + 22, y + 10, 3.6, PKM_BAND); disc(x + 22, y + 10, 2.4, lit ? 0xff8a70 : PKM_WHITE);
      dot(x + 21, y + 9, WHITE, 0.8);
      if (lit) glowAt(x + 22, y + 10, 6, 5, 0xff5a3a, 0.16);
      if (isNight) glowLate(() => glowAt(x + 22, y + 10, 4, 3, 0xffe0d0, lit ? 0.3 : 0.1));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A Hextech desk: a dark top edged in gold over a navy front in a gold frame, gold filigree either side and, in the middle, a blue hextech crystal in a gold diamond that pulses slowly and glows at night. */
  lolhexdesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, LOL_GOLD_LIGHT); fill(x, y + 1, 44, 4, tint(0x1e2d50, 1)); fill(x, y + 4, 44, 1, LOL_GOLD_DARK);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      hexPanel(x, y + 5, 44, 11);
      for (const fx of [x + 3, x + 28]) { fill(fx, y + 10, 13, 1, LOL_GOLD_DARK); dot(fx + 6, y + 9, LOL_GOLD); dot(fx + 6, y + 11, LOL_GOLD); dot(fx, y + 10, LOL_GOLD); dot(fx + 12, y + 10, LOL_GOLD); }
      line(x + 22, y + 6, x + 26, y + 10, LOL_GOLD); line(x + 26, y + 10, x + 22, y + 14, LOL_GOLD); line(x + 22, y + 14, x + 18, y + 10, LOL_GOLD); line(x + 18, y + 10, x + 22, y + 6, LOL_GOLD);
      const pulse = (Math.sin(now * 1.6 + x * 0.1) + 1) / 2;
      hexCrystal(x + 22, y + 10, 7, pulse);
      if (isNight) glowLate(() => { glowAt(x + 22, y + 10, 7, 6, LOL_HEX, 0.12 + 0.1 * pulse); dot(x + 22, y + 10, LOL_HEX_LIGHT); });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A quest-giver's desk: an oak desk bound in iron with riveted corners and a gold emblem; a gold "!" floats over its left end, bobbing, and for a while every 30 s it turns to a "?", a quest done. It glows at night. */
  wowquestdesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xa8784a, 1)); fill(x, y + 1, 44, 4, tint(0x8a5a32, 1)); fill(x, y + 4, 44, 1, WOW_WOOD_DARK);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, WOW_WOOD);
      for (let k = 0; k < 44; k += 7) { fill(x + k, y + 5, 1, 11, WOW_WOOD_DARK); fill(x + k + 1, y + 5, 1, 11, WOW_WOOD_LIGHT); }
      for (const ex of [x, x + 40]) { fill(ex, y + 5, 4, 11, WOW_IRON); fill(ex, y + 5, 4, 1, 0x8a8a92); dot(ex + 1, y + 7, 0x3a3a42); dot(ex + 2, y + 13, 0x3a3a42); }
      fill(x, y + 9, 44, 2, WOW_IRON); fill(x, y + 9, 44, 1, 0x8a8a92);
      disc(x + 22, y + 10, 3, WOW_GOLD); disc(x + 22, y + 10, 1.6, WOW_BLUE); dot(x + 21, y + 9, 0xfff0a0);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      const done = (now + x * 0.17) % 30 < 8, bob = Math.round(Math.sin(now * 2.2 + x) * 1);
      questMark(x + 2, y - 15 + bob, done);
      if (isNight) glowLate(() => questMark(x + 2, y - 15 + bob, done, 0.6));
      drawKeyboard(s, y + 2);
    },
  },
  /** A lowrider desk: a candy-purple front with a metal-flake sparkle, a chrome grille and bumper, round headlights and a little plate; every 20 s the hydraulics kick in and the front hops, headlights flashing. The lights glow at night. */
  gtalowrider: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x8a4ab8, 1)); fill(x, y + 1, 44, 4, tint(GTA_PURPLE, 1)); fill(x, y + 4, 44, 1, 0x4a1a6a);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      const t = (now + x * 0.29) % 20, hop = t < 1.6 ? [0, -2, -1, 0, -1, 0, 0, 0][Math.floor(t * 5)] || 0 : 0, fy = y + 5 + hop;
      fill(x, y + 5, 44, 11, 0x2a1030);
      fill(x, fy, 44, 9, GTA_PURPLE); fill(x, fy, 44, 1, 0x9a5ad8); fill(x, fy, 1, 9, 0x8a4ab8); fill(x + 43, fy, 1, 9, 0x3a1060);
      for (let k = 0; k < 6; k++) dot(x + 3 + (hash(k, x) % 38), fy + 1 + (hash(x, k) % 7), 0xd8a8ff, 0.6);
      // The grille, the headlights, the bumper and the plate
      fill(x + 15, fy + 2, 14, 5, GTA_CHROME_DARK); for (let k = 0; k < 14; k += 2) fill(x + 15 + k, fy + 2, 1, 5, GTA_CHROME); fill(x + 15, fy + 2, 14, 1, WHITE);
      const flash = hop !== 0 && Math.floor(now * 8) % 2;
      for (const hx of [x + 6, x + 37]) { disc(hx, fy + 4, 2.4, GTA_CHROME); disc(hx, fy + 4, 1.6, flash ? WHITE : 0xfff0b0); }
      fill(x, fy + 9, 44, 2, GTA_CHROME); fill(x, fy + 9, 44, 1, WHITE); fill(x, fy + 10, 44, 1, GTA_CHROME_DARK);
      fill(x + 19, fy + 7, 6, 2, 0xf4f2ee); fill(x + 20, fy + 8, 4, 1, 0x3a4a8a);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      if (isNight) glowLate(() => { for (const hx of [x + 6, x + 37]) { glowAt(hx, fy + 4, 5, 3, 0xfff0b0, flash ? 0.4 : 0.18); dot(hx, fy + 4, WHITE); } });
      drawKeyboard(s, y + 2);
    },
  },
  /** A Vanguard console: a white desk trimmed in gold with a teal light strip across its front and the Vanguard's gold emblem; a purple engram floats over its left end, turning, and for a while every 30 s it shines exotic gold. */
  destinyvanguard: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xf6f4f0, 1)); fill(x, y + 1, 44, 4, tint(0xe0ded8, 0.95)); fill(x, y + 4, 44, 1, DST_GOLD);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, DST_WHITE); fill(x, y + 5, 1, 11, WHITE); fill(x + 43, y + 5, 1, 11, 0xb8bcc4); fill(x, y + 15, 44, 1, 0xc8c4bc);
      const pulse = (Math.sin(now * 1.4 + x) + 1) / 2;
      fill(x + 2, y + 8, 40, 1, DST_TEAL, 0.5 + 0.4 * pulse);
      // The emblem: three gold blades round a diamond
      fill(x + 19, y + 9, 7, 5, 0x2a3a4a); line(x + 22, y + 9, x + 25, y + 11, DST_GOLD); line(x + 25, y + 11, x + 22, y + 13, DST_GOLD); line(x + 22, y + 13, x + 19, y + 11, DST_GOLD); line(x + 19, y + 11, x + 22, y + 9, DST_GOLD); dot(x + 22, y + 11, DST_TEAL);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      const exotic = (now + x * 0.17) % 30 < 6, bob = Math.round(Math.sin(now * 2 + x));
      engram(x + 3, y - 10 + bob, 7, exotic ? DST_EXOTIC : DST_LEGEND, now * 2);
      if (isNight) glowLate(() => { glowAt(x + 3, y - 10 + bob, 5, 5, exotic ? DST_EXOTIC : DST_LEGEND, 0.25); fill(x + 2, y + 8, 40, 1, 0x9af0f0); });
      drawKeyboard(s, y + 2);
    },
  },
  /** An Imperial console: a dark grey desk with a top of blinking buttons at its right end, a front of light grids that twinkle in red, green and white, a vent and an embossed cog. */
  starwarsconsole: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(SW_GREY_LIGHT, 1)); fill(x, y + 1, 44, 4, tint(SW_GREY, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      // The buttons on the top's right end
      for (let k = 0; k < 5; k++) { const on = (Math.floor(now * 2 + k * 1.7 + x) % 4) !== 0; dot(x + 31 + k * 2, y + 2, [0xff4a3a, 0x5aff7a, 0xf2d35b, 0x6ab8ff, WHITE][k], on ? 0.95 : 0.3); }
      fill(x + 30, y + 3, 11, 1, tint(SW_GREY_DARK, 0.9));
      // The front: dark, a grid of lights, a vent and the cog
      fill(x, y + 5, 44, 11, tint(SW_GREY_DARK, 0.95)); fill(x, y + 5, 44, 1, tint(SW_GREY_LIGHT, 0.9)); fill(x, y + 15, 44, 1, tint(0x1e2026, 0.9));
      fill(x + 3, y + 7, 15, 7, 0x1a1c22);
      for (let r = 0; r < 3; r++) for (let c = 0; c < 7; c++) {
        const h = hash(x + c, r), on = (Math.floor(now * 1.5) + h) % 5 !== 0;
        dot(x + 4 + c * 2, y + 8 + r * 2, [0xff4a3a, 0x5aff7a, WHITE, 0x6ab8ff][h % 4], on ? 0.9 : 0.25);
      }
      for (let k = 0; k < 4; k++) fill(x + 21, y + 7 + k * 2, 8, 1, 0x1a1c22);
      swMark('empire', x + 36, y + 10, tint(SW_GREY, 0.95));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      if (isNight) glowLate(() => glowAt(x + 10, y + 10, 9, 5, 0x6ab8ff, 0.07));
      drawKeyboard(s, y + 2);
    },
  },
  /** A vanity desk: a white top over a hot pink front with three white drawers and gold heart pulls, a scalloped hem, and a little round mirror ringed with bulbs on its left end that twinkle now and then. */
  girly1vanity: {
    front({ x, y, s, tint }) {
      // The little bulb mirror on the left end
      fill(x + 2, y - 2, 1, 2, G1_GOLD); ring(x + 2, y - 6, 3, G1_GOLD, 0.95); disc(x + 2, y - 6, 2.2, 0xf4dce6); dot(x + 1, y - 7, WHITE, 0.8);
      const tw = (now + x * 0.21) % 11;
      for (const [dx, dy, k] of [[-2, -6, 0], [2, -10, 1], [6, -6, 2], [2, -2, 3]]) { dot(x + 2 + dx, y + dy, 0xfff4c8); if (tw < 2 && Math.floor(tw * 2) === k) g1Twinkle(x + 2 + dx, y + dy, (tw * 2) % 1); }
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(WHITE, 1)); fill(x, y + 1, 44, 4, tint(G1_WHITE, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 10, tint(G1_PINK, 0.95)); fill(x, y + 5, 44, 1, tint(G1_PINK_LIGHT, 0.95));
      for (const dx of [3, 16, 29]) { fill(x + dx, y + 7, 12, 6, tint(G1_WHITE, 0.95)); fill(x + dx, y + 7, 12, 1, WHITE); fill(x + dx, y + 12, 12, 1, tint(0xe0c8d0, 0.95)); g1Mark('smallheart', x + dx + 5, y + 9, G1_GOLD); }
      for (let k = 0; k < 44; k += 4) { fill(x + k, y + 15, 3, 1, tint(G1_PINK, 0.9)); dot(x + k + 1, y + 16, tint(G1_PINK_DARK, 0.9)); }
      fill(x + 1, y + 17, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A kawaii desk: a white top over three pastel drawers (pink, mint, lilac) with round knobs, star and heart stickers, a red bow in the middle and a lace hem; a tiny cactus in a kitty pot on its left end. */
  girly2desk: {
    front({ x, y, s, tint }) {
      // The kitty pot and its cactus
      fill(x + 1, y - 4, 4, 4, 0xfffcfa); dot(x + 1, y - 5, 0xfffcfa); dot(x + 4, y - 5, 0xfffcfa); dot(x + 2, y - 3, 0x1f1a1a); dot(x + 3, y - 2, G2_NOSE);
      fill(x + 2, y - 8, 2, 3, 0x6ac88a); dot(x + 1, y - 7, 0x6ac88a); dot(x + 2, y - 8, 0x8ae0a8); dot(x + 3, y - 9, G2_PINK_DARK);
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(WHITE, 1)); fill(x, y + 1, 44, 4, tint(0xfaf6f8, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 10, tint(0xfaf6f8, 0.95)); fill(x, y + 5, 44, 1, tint(0xe8e0e4, 0.95));
      [[2, G2_PINK], [16, G2_MINT], [30, G2_LILAC]].forEach(([dx, c]) => { fill(x + dx, y + 7, 12, 6, tint(c, 0.95)); fill(x + dx, y + 7, 12, 1, tint(shaded(c, 1.08), 0.95)); fill(x + dx, y + 12, 12, 1, tint(shaded(c, 0.88), 0.95)); dot(x + dx + 6, y + 9, WHITE); dot(x + dx + 6, y + 10, tint(shaded(c, 0.8), 0.95)); });
      g1Mark('smallheart', x + 4, y + 8, G2_BOW, 0.9); for (const [dx, dy] of [[0, -1], [-1, 0], [0, 0], [1, 0], [0, 1]]) dot(x + 40 + dx, y + 9 + dy, G2_NOSE);
      fill(x + 20, y + 5, 2, 2, G2_BOW); fill(x + 23, y + 5, 2, 2, G2_BOW); dot(x + 22, y + 5, 0xb81e34); dot(x + 21, y + 7, G2_BOW); dot(x + 23, y + 7, G2_BOW);
      for (let k = 0; k < 44; k += 3) { dot(x + k, y + 15, tint(WHITE, 0.95)); dot(x + k + 1, y + 15, tint(G2_PINK, 0.9)); dot(x + k + 1, y + 16, tint(WHITE, 0.9)); }
      fill(x + 1, y + 17, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A boss's desk: deep mahogany with gold trim, two carved panels and a gold nameplate on its front, and two bundles of cash stacked on its left end. */
  narcosdesk: {
    front({ x, y, s, tint }) {
      ncStack(x + 1, y - 3, 5, 3); ncStack(x + 2, y - 6, 5, 3);
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(NC_WOOD_LIGHT, 1)); fill(x, y + 1, 44, 4, tint(NC_WOOD, 0.95)); fill(x, y + 4, 44, 1, tint(NC_GOLD, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(NC_WOOD, 0.9)); fill(x, y + 15, 44, 1, tint(NC_WOOD_DARK, 0.9));
      for (const px of [x + 3, x + 29]) { fill(px, y + 7, 12, 7, tint(NC_WOOD_DARK, 0.9)); fill(px + 1, y + 8, 10, 5, tint(NC_WOOD_LIGHT, 0.85)); fill(px + 2, y + 9, 8, 3, tint(NC_WOOD, 0.85)); }
      fill(x + 17, y + 8, 10, 4, NC_GOLD); fill(x + 17, y + 8, 10, 1, NC_GOLD_LIGHT); fill(x + 19, y + 10, 6, 1, NC_GOLD_DARK);
      fill(x, y + 5, 1, 11, tint(NC_GOLD, 0.9)); fill(x + 43, y + 5, 1, 11, tint(NC_GOLD_DARK, 0.9));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A store table: a thick slab of light oak with softly rounded ends on two slim aluminium legs, open underneath, and a phone glowing on a clear stand at its left end. */
  apple1table: {
    front({ x, y, s, tint }) {
      // The phone on its stand
      fill(x + 2, y - 1, 3, 1, 0xdff0f6, 0.7); fill(x + 2, y - 8, 3, 7, 0x2c2c2e); fill(x + 3, y - 7, 1, 5, A1_WALLS[Math.floor((now + x * 0.2) / 15) % 3][0]); dot(x + 3, y - 4, A1_WALLS[Math.floor((now + x * 0.2) / 15) % 3][1]);
      drawMonitor(x + 6, y - 10);
      fill(x + 1, y, 42, 1, tint(A1_OAK_LIGHT, 1)); fill(x, y + 1, 44, 3, tint(A1_OAK, 0.95)); fill(x + 1, y + 4, 42, 1, tint(A1_OAK_DARK, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      for (let k = 4; k < 40; k += 9) fill(x + k, y + 2, 5, 1, tint(0xdcc094, 0.9));
      for (const lx of [x + 3, x + 39]) { fill(lx, y + 5, 2, 11, A1_SILVER); fill(lx, y + 5, 1, 11, A1_SILVER_LIGHT); }
      fill(x + 2, y + 16, 40, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** An 80s desk: a wood-veneer top and front with beige metal drawers and chrome pulls, and a little box of coloured floppy disks on its left end. */
  apple2desk: {
    front({ x, y, s, tint }) {
      fill(x + 1, y - 4, 5, 4, A2_BEIGE_DARK); fill(x + 1, y - 4, 5, 1, A2_BEIGE);
      [0x2a5ab8, 0xe03a3e, 0x61bb46, 0xfdb827].forEach((c, i) => fill(x + 1 + i, y - 6 - (i % 2), 1, 2 + (i % 2), c));
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xc8884a, 1)); fill(x, y + 1, 44, 4, tint(A2_VENEER, 0.95));
      for (let k = 3; k < 44; k += 7) fill(x + k, y + 2, 4, 1, tint(0x9a5e30, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(A2_VENEER, 0.9)); fill(x, y + 5, 44, 1, tint(A2_VENEER_DARK, 0.9)); fill(x, y + 15, 44, 1, tint(A2_VENEER_DARK, 0.9));
      fill(x + 28, y + 6, 14, 9, A2_BEIGE); fill(x + 28, y + 6, 14, 1, A2_BEIGE_LIGHT); fill(x + 28, y + 10, 14, 1, A2_BEIGE_DARK);
      for (const py of [8, 12]) { fill(x + 33, y + py, 4, 1, 0xc8ccd4); dot(x + 33, y + py, WHITE); }
      for (let k = 3; k < 26; k += 6) fill(x + k, y + 7, 1, 7, tint(A2_VENEER_DARK, 0.8));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Olivewood desk. Honey-coloured olivewood with its wavy grain, a band of blue-and-white tiles along the front and two
   * drawers with silver pulls; a little blue vase on the left corner holds an olive sprig whose leaf stirs now and then.
   */
  jewishdesk: {
    front({ x, y, s, tint }) {
      const olive = 0xb08850, oliveLight = 0xc8a068, oliveDark = 0x7a5a30, grain = 0x9a7440, blue = 0x2f5f9e, blueLight = 0x5a8ad0, white = 0xf2f0ea, silver = 0xc8ccd4;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(oliveLight, 1)); fill(x, y + 1, 44, 4, tint(olive, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      // The front: the tile band, then wavy grain and the drawers
      fill(x, y + 5, 44, 11, tint(olive, 0.82)); fill(x, y + 15, 44, 1, tint(oliveDark, 0.8)); fill(x + 43, y + 5, 1, 11, tint(oliveDark, 0.8));
      fill(x + 1, y + 6, 42, 3, white); fill(x + 1, y + 6, 42, 1, 0xfbfaf6);
      for (let k = 0; k < 14; k++) { const tx = x + 2 + k * 3; dot(tx, y + 7, k % 2 ? blue : blueLight); dot(tx + 1, y + (k % 2 ? 6 : 8), blue, 0.7); }
      fill(x, y + 9, 44, 1, tint(oliveDark, 0.8));
      for (let k = 0; k < 6; k++) { const gx = x + 3 + k * 7; line(gx, y + 11, gx + 3, y + 14, tint(grain, 0.8)); }
      for (const dx of [8, 30]) { fill(x + dx, y + 11, 6, 1, silver); dot(x + dx, y + 11, 0xeef0f4); }
      // The vase and its olive sprig
      const t = (now + x * 0.23) % 16, sway = t < 2 ? Math.floor(t * 3) % 2 : 0;
      fill(x + 1, y - 4, 3, 4, blue); dot(x + 1, y - 4, blueLight); dot(x + 2, y - 5, blue);
      line(x + 2, y - 6, x + 3 + sway, y - 10, 0x6a7a40); dot(x + 1, y - 8, 0x7a9a5a); dot(x + 4 + sway, y - 9, 0x8aaa66); dot(x + 3, y - 7, 0x6a8a4a);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Inlaid desk. Dark walnut with a band of mother-of-pearl inlay in a lattice of little diamonds along the front, brass
   * corner plates and drawer pulls, and a small brass bowl of water with a floating rose petal on the left; a gleam
   * crosses the inlay now and then.
   */
  muslimdesk: {
    front({ x, y, s, tint }) {
      const wood = 0x4a2e1e, woodLight = 0x6a4430, woodDark = 0x2e1a10, pearl = 0xf2eee4, pearlDim = 0xc8c4bc, brass = 0xc8a050, brassLight = 0xe8c878;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodLight, 1)); fill(x, y + 1, 44, 4, tint(wood, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(wood, 0.85)); fill(x, y + 15, 44, 1, tint(woodDark, 0.8));
      // The inlay band, a gleam crossing it now and then
      fill(x + 1, y + 6, 42, 3, tint(woodDark, 0.85));
      const g = (now + x * 0.21) % 15, at = g < 1.5 ? Math.floor(g * 9) : -1;
      for (let k = 0; k < 14; k++) {
        const ix = x + 2 + k * 3;
        dot(ix + 1, y + 6, pearlDim); dot(ix, y + 7, pearl); dot(ix + 2, y + 7, pearl); dot(ix + 1, y + 8, pearlDim);
        if (k === at) dot(ix + 1, y + 7, WHITE, 0.9);
      }
      // Brass corner plates and the pulls
      for (const [dx, dy] of [[0, 5], [42, 5], [0, 14], [42, 14]]) fill(x + dx, y + dy, 2, 2, brass);
      for (const dx of [10, 28]) { fill(x + dx, y + 12, 6, 1, brass); dot(x + dx, y + 12, brassLight); }
      // The bowl of water with its petal
      const drift = Math.floor((now + x) * 0.25) % 2;
      fill(x + 1, y - 2, 5, 2, brass); dot(x + 1, y - 2, brassLight); fill(x + 2, y - 3, 3, 1, 0x8ac8e0, 0.8); dot(x + 2 + drift, y - 3, 0xf28fb0);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Carved oak desk. Warm oak with three pointed Gothic arches carved into its front, like a pew's end, and a brass
   * candleholder on the left corner with a white candle whose flame wavers, glowing at night.
   */
  christiandesk: {
    front({ x, y, s, tint }) {
      const oak = 0x8a5a32, oakLight = 0xa8703e, oakDark = 0x5a3b25, carve = 0x4a2e1c, brass = 0xc8a050;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(oakLight, 1)); fill(x, y + 1, 44, 4, tint(oak, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(oak, 0.85)); fill(x, y + 5, 44, 1, tint(oakDark, 0.8)); fill(x, y + 15, 44, 1, tint(oakDark, 0.8));
      // The three carved arches
      for (const ax of [x + 5, x + 18, x + 31]) {
        fill(ax + 3, y + 7, 2, 1, tint(carve, 0.85)); fill(ax + 1, y + 8, 6, 1, tint(carve, 0.85)); fill(ax, y + 9, 8, 5, tint(carve, 0.85));
        fill(ax + 3, y + 9, 2, 1, tint(oak, 0.7)); fill(ax + 1, y + 10, 6, 4, tint(oak, 0.7)); fill(ax + 1, y + 10, 1, 4, tint(oakLight, 0.7));
      }
      // The candle on its brass holder
      const f = candlelight(x * 0.1);
      fill(x + 1, y - 1, 4, 1, brass); fill(x + 2, y - 5, 2, 4, 0xf6efdc); dot(x + 3, y - 5, 0xe0d6c0);
      dot(x + 2, y - 6, 0xffd36b, f); dot(x + 2, y - 7, 0xff8a3d, 0.55 * f);
      glowAt(x + 2, y - 6, 6, 5, 0xffb04a, 0.07 * f);
      if (isNight) glowLate(() => { glowAt(x + 2, y - 6, 8, 6, 0xffb04a, 0.16 * f); dot(x + 2, y - 6, 0xfff4c0); });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Lacquered desk. Dark teak with a red lacquer panel across its front painted with three small gold lotuses, and a
   * little pot of bamboo on the left corner whose leaves stir now and then.
   */
  buddhistdesk: {
    front({ x, y, s, tint }) {
      const teak = 0x5a3a24, teakLight = 0x7a5232, teakDark = 0x3a2416, lac = 0xa8282a, lacLight = 0xc8403a, lacDark = 0x7a1a1c, gold = 0xd8b45a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(teakLight, 1)); fill(x, y + 1, 44, 4, tint(teak, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(teak, 0.85)); fill(x, y + 15, 44, 1, tint(teakDark, 0.8));
      // The lacquer panel and its lotuses
      fill(x + 3, y + 7, 38, 7, lac); fill(x + 3, y + 7, 38, 1, lacLight); fill(x + 3, y + 13, 38, 1, lacDark);
      for (const lx of [x + 10, x + 22, x + 34]) {
        dot(lx, y + 9, gold); dot(lx - 1, y + 10, gold); dot(lx + 1, y + 10, gold); dot(lx, y + 10, 0xf2d888);
        fill(lx - 2, y + 11, 5, 1, gold, 0.8);
      }
      // The bamboo in its celadon pot
      const t = (now + x * 0.31) % 14, sway = t < 2 ? Math.floor(t * 3) % 2 : 0;
      fill(x + 1, y - 3, 4, 3, 0x7aa898); dot(x + 1, y - 3, 0x9ac8b8);
      fill(x + 2, y - 10, 1, 7, 0x6a9a4a); fill(x + 3, y - 8, 1, 5, 0x7aaa5a); dot(x + 2, y - 7, 0x4a7a3a); dot(x + 3, y - 5, 0x4a7a3a);
      line(x + 2, y - 9, x + 0 - sway, y - 11, 0x5a9a4a); line(x + 3, y - 8, x + 5 + sway, y - 10, 0x5a9a4a);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Lair console. A gunmetal top over a front of black obsidian, set with a strip of coloured buttons that wink on and off,
   * a red lamp at the end that flashes now and then, and a yellow and black hazard stripe along the foot.
   */
  volcanodesk: {
    front({ x, y, s, tint }) {
      const steel = 0x4a4e58, steelLight = 0x6e727c, steelDark = 0x2e3038, obs = 0x221c26, yellow = 0xf2c830, black = 0x1e1e22;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(steelLight, 1)); fill(x, y + 1, 44, 4, tint(steel, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, obs); fill(x, y + 5, 44, 1, 0x3a3040); fill(x, y + 15, 44, 1, 0x120e16);
      // The button strip: they wink on and off, slowly
      fill(x + 3, y + 7, 30, 3, steelDark); fill(x + 3, y + 7, 30, 1, steel);
      const cols = [0xd8281e, 0xf2c830, 0x3ab84a, 0x4c8bf5];
      for (let k = 0; k < 9; k++) { const on = (Math.floor(now * 0.7 + k * 1.7 + x) % 4) !== 0; fill(x + 5 + k * 3, y + 8, 2, 1, cols[k % 4], on ? 1 : 0.3); }
      // The big lamp
      const flash = (now + x * 0.3) % 9 < 0.8;
      fill(x + 36, y + 7, 5, 3, steelDark); fill(x + 37, y + 7, 3, 2, flash ? 0xff3a28 : 0x8a1a14); if (flash) dot(x + 37, y + 7, 0xffd0c0);
      // The hazard stripe
      for (let k = 0; k < 44; k += 4) { fill(x + k, y + 12, 2, 1, yellow); fill(x + k + 2, y + 12, 2, 1, black); fill(x + k + 1, y + 13, 2, 1, yellow); fill(x + ((k + 3) % 44), y + 13, 1, 1, black); }
      fill(x + 1, y + 16, 42, 1, SHADOW);
      if (isNight && flash) glowLate(() => { glowAt(x + 38, y + 8, 4, 3, 0xff3020, 0.25); dot(x + 38, y + 8, 0xffb0a0); });
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Steel station. A stainless steel prep bench at standing height, its brushed front panel over an open shelf stacked with
   * white plates and a hotel pan, and a blue-striped side towel hanging from its edge that stirs now and then.
   */
  kitchendesk: {
    front({ x, y, s, tint }) {
      const steel = 0xc8ccd2, steelLight = 0xe4e8ec, steelDark = 0x8a9098;
      drawMonitor(x + 6, y - 12);
      fill(x + 11, y - 1, 3, 1, PALETTE.monitorStand);
      fill(x, y, 44, 1, tint(steelLight, 1)); fill(x, y + 1, 44, 3, tint(steel, 0.95)); fill(x, y + 4, 44, 2, steelDark);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      // The legs and the shelf, with plates and a hotel pan on it
      fill(x + 1, y + 6, 2, 10, steel); fill(x + 41, y + 6, 2, 10, steelDark); fill(x + 1, y + 12, 42, 2, steel); fill(x + 1, y + 12, 42, 1, steelLight);
      fill(x + 3, y + 6, 38, 6, 0x5a5e66, 0.35);
      for (let k = 0; k < 4; k++) fill(x + 6, y + 11 - k, 11, 1, k % 2 ? 0xf6f6f2 : 0xdcdcd8);
      fill(x + 22, y + 9, 12, 3, steel); fill(x + 22, y + 9, 12, 1, steelLight);
      // The side towel
      const t = (now + x * 0.31) % 15, flap = t < 1.5 ? Math.floor(t * 4) % 2 : 0;
      fill(x + 36 + flap, y + 4, 4, 6, 0xf2f2ee); fill(x + 36 + flap, y + 6, 4, 1, 0x4c6aa8); fill(x + 36 + flap, y + 8, 4, 1, 0x4c6aa8);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Nurses' station. A white counter with a raised front of pale blue panels and a ledge on top, a sanitiser pump at one end
   * and a little bell; a rack of patient files shows at the side.
   */
  hospitaldesk: {
    front({ x, y, s, tint }) {
      const white = 0xf6f8f8, shade = 0xd8dee2, blue = 0x8ab8e0, blueDark = 0x5a8ab8;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(white, 1)); fill(x, y + 1, 44, 4, tint(shade, 1));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, white); fill(x, y + 5, 44, 1, WHITE); fill(x, y + 15, 44, 1, shade);
      for (const px of [2, 16, 30]) { fill(x + px, y + 7, 12, 7, blue); fill(x + px, y + 7, 12, 1, 0xb0d4f0); fill(x + px, y + 13, 12, 1, blueDark); }
      fill(x + 20, y + 9, 4, 1, WHITE); fill(x + 21, y + 8, 2, 3, WHITE);
      // The sanitiser pump and the files
      fill(x + 1, y - 4, 3, 4, 0xf2f4f4); fill(x + 1, y - 3, 3, 2, 0x5ad8f0, 0.7); fill(x + 2, y - 6, 1, 2, 0x8a9098); fill(x + 2, y - 6, 2, 1, 0x8a9098);
      for (const [fx, c] of [[0, 0xf2d35b], [1, 0x4c8bf5], [2, 0xf28fb0]]) fill(x + 5 + fx, y - 5, 1, 5, c);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Detective's desk. A heavy dark oak desk with panelled drawers and brass pulls, a green-shaded banker's lamp glowing warm
   * on its left corner, a wire in-tray stacked with files beside it.
   */
  noirdesk: {
    front({ x, y, s, tint }) {
      const oak = 0x4a2e1e, oakLight = 0x6a4a32, oakDark = 0x2e1c12, brass = 0xc8a050;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(oakLight, 1)); fill(x, y + 1, 44, 4, tint(oak, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, oak); fill(x, y + 15, 44, 1, oakDark);
      for (const [px, w] of [[2, 12], [16, 12], [30, 12]]) { fill(x + px, y + 7, w, 7, oakDark, 0.45); fill(x + px, y + 7, w, 1, oakLight, 0.6); dot(x + px + 6, y + 10, brass); }
      // The banker's lamp
      const lamp = () => { fill(x, y - 7, 6, 2, 0x2a7a4a); fill(x + 1, y - 8, 4, 1, 0x3a9a5a); fill(x + 1, y - 5, 4, 1, 0xffe08a); };
      fill(x + 2, y - 4, 1, 4, brass); fill(x + 1, y - 1, 4, 1, brass); lamp();
      glowAt(x + 3, y - 2, 6, 3, 0xffd070, 0.1);
      fill(x + 6, y - 2, 5, 2, 0xa8adb6); fill(x + 6, y - 3, 5, 1, 0xf2ecd8); fill(x + 6, y - 4, 5, 1, 0xe8d8b0);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      if (isNight) glowLate(() => { glowAt(x + 3, y - 1, 8, 4, 0xffd070, 0.22); lamp(); });
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Lab workbench. A heavy black-stained bench with an iron-bound front, a rack of three test tubes bubbling in colours on its
   * left corner and a copper coil fixed to its front that sparks blue now and then.
   */
  madlabdesk: {
    front({ x, y, s, tint }) {
      const wood = 0x2e2420, woodLight = 0x4a3a32, iron = 0x5a5650, copper = 0xb87333;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodLight, 1)); fill(x, y + 1, 44, 4, tint(wood, 1));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, wood); fill(x, y + 5, 44, 1, woodLight); fill(x, y + 15, 44, 1, 0x1a1412);
      for (const bx of [2, 40]) { fill(x + bx, y + 5, 2, 11, iron); for (const ry of [7, 13]) dot(x + bx, y + ry, 0x8a8a90); }
      // The copper coil, sparking now and then
      for (let k = 0; k < 6; k++) { fill(x + 18 + k * 2, y + 8, 1, 4, copper); dot(x + 18 + k * 2, y + 8, 0xe0a060); }
      fill(x + 17, y + 12, 13, 1, iron);
      if ((now + x * 0.27) % 9 < 0.5) { line(x + 17, y + 7, x + 20, y + 5, 0x8ae0ff); line(x + 30, y + 7, x + 27, y + 5, 0x8ae0ff); }
      // The test tube rack
      fill(x, y - 2, 7, 2, 0x8a5a32); fill(x, y - 2, 7, 1, 0xa8703e);
      [[1, 0x7ae04a], [3, 0xb88aff], [5, 0xf2b840]].forEach(([tx, c], k) => {
        fill(x + tx, y - 7, 1, 5, 0xdff0f6, 0.6); fill(x + tx, y - 5, 1, 3, c); if (Math.floor(now * 2 + k + x) % 3 === 0) dot(x + tx, y - 6, WHITE, 0.8);
      });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Blackjack table. A half-moon of green felt behind a padded black leather rail and a gold trim, a dealer's chip tray set in
   * its edge, the front in dark wood; the chips catch the light now and then.
   */
  casinodesk: {
    front({ x, y, s, tint }) {
      const felt = 0x1e6a3a, feltLight = 0x2a8a4a, rail = 0x1e1a1a, wood = 0x3a1e14, gold = 0xd8b45a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(feltLight, 1)); fill(x, y + 1, 44, 3, tint(felt, 1));
      ellipseRing(x + 22, y + 2, 14, 1.4, 0xf2d888, 0.4);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 4, 44, 3, rail); fill(x, y + 4, 44, 1, 0x3a3434); fill(x, y + 7, 44, 1, gold);
      fill(x, y + 8, 44, 8, wood); fill(x, y + 15, 44, 1, 0x1e0e08);
      for (let k = 0; k < 5; k++) fill(x + 3 + k * 9, y + 10, 6, 4, 0x2a140c, 0.6);
      // The chip tray on the felt's far corner
      fill(x + 1, y - 2, 8, 2, 0x1e1a1a);
      [0xd8302a, 0x2a4ab8, 0x1e1e22, 0x3ab86a].forEach((c, k) => { fill(x + 1 + k * 2, y - 3, 2, 1, c); dot(x + 1 + k * 2, y - 3, WHITE, 0.4); });
      if ((now + x * 0.3) % 9 < 0.4) dot(x + 3, y - 3, WHITE);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Carriage writing desk. A mahogany desk with an inlaid front of pale wood diamonds in brass lines, the kind fitted in a
   * sleeping compartment, and a little brass table lamp with a pleated silk shade glowing warm on its corner.
   */
  orientdesk: {
    front({ x, y, s, tint }) {
      const wood = 0x6a2e1e, woodLight = 0x8a4a2e, woodDark = 0x4a1e12, inlay = 0xe8d0a0, gold = 0xd8b45a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodLight, 1)); fill(x, y + 1, 44, 4, tint(wood, 1));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, wood); fill(x, y + 5, 44, 1, gold); fill(x, y + 15, 44, 1, woodDark);
      for (const dx of [7, 22, 37]) { dot(x + dx, y + 7, inlay); fill(x + dx - 1, y + 8, 3, 1, inlay); fill(x + dx - 2, y + 9, 5, 2, inlay); fill(x + dx - 1, y + 11, 3, 1, inlay); dot(x + dx, y + 12, inlay); dot(x + dx, y + 10, 0xc87a3a); }
      fill(x + 1, y + 14, 42, 1, gold, 0.6);
      // The lamp
      const lamp = () => { fill(x + 1, y - 9, 5, 3, 0xe8a870); fill(x + 2, y - 10, 3, 1, 0xe8a870); for (const k of [2, 4]) fill(x + k, y - 9, 1, 3, 0xc88850); fill(x + 1, y - 6, 5, 1, 0xfff0c0); };
      fill(x + 3, y - 5, 1, 5, gold); fill(x + 2, y - 1, 3, 1, gold); lamp();
      glowAt(x + 3, y - 5, 6, 4, 0xffd090, 0.1);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      if (isNight) glowLate(() => { glowAt(x + 3, y - 4, 8, 5, 0xffd090, 0.22); lamp(); });
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Gothic desk. A carved black desk on lion's claw feet, its front panelled in pointed arches with purple inlay, three
   * candles of different heights dripping wax on its corner, their flames flickering and glowing through the night, and a
   * scrap of cobweb in its far corner.
   */
  halloweendxdesk: {
    front({ x, y, s, tint }) {
      const wood = 0x1e1820, woodLight = 0x3a2e3a, woodDark = 0x0e0a0e, inlay = 0x6a3a8a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodLight, 1)); fill(x, y + 1, 44, 4, tint(wood, 1));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 10, wood); fill(x, y + 5, 44, 1, woodLight);
      for (const ax of [3, 17, 31]) { fill(x + ax, y + 9, 10, 5, woodDark); fill(x + ax + 1, y + 8, 8, 1, woodDark); fill(x + ax + 3, y + 7, 4, 1, woodDark); fill(x + ax + 4, y + 10, 2, 3, inlay); dot(x + ax + 5, y + 9, inlay); }
      for (const fx of [0, 39]) { fill(x + fx, y + 15, 5, 1, woodLight); dot(x + fx, y + 16, woodLight); dot(x + fx + 2, y + 16, woodLight); dot(x + fx + 4, y + 16, woodLight); }
      // The candles
      const cands = [[1, 6], [3, 9], [5, 5]];
      const flames = () => cands.forEach(([cx, h], k) => { const f = candlelight(x * 0.2 + k); dot(x + cx, y - h - 1, 0xffd060, f); dot(x + cx, y - h - 2, 0xff8a3a, 0.5 * f); });
      cands.forEach(([cx, h]) => { fill(x + cx, y - h, 1, h, 0xf2ead8); dot(x + cx + 1, y - h + 2, 0xe8dcc0); });
      fill(x, y - 1, 7, 1, 0xe8dcc0); flames();
      glowAt(x + 3, y - 7, 6, 5, 0xffb050, 0.08);
      line(x + 44, y - 8, x + 40, y, 0xd8d4dc, 0.35); line(x + 44, y - 4, x + 42, y, 0xd8d4dc, 0.35); line(x + 41, y - 3, x + 44, y - 6, 0xd8d4dc, 0.3);
      fill(x + 1, y + 17, 42, 1, SHADOW);
      if (isNight) glowLate(() => { glowAt(x + 3, y - 7, 8, 6, 0xffb050, 0.12); glowAt(x + 3, y - 7, 4, 4, 0xffb050, 0.12); flames(); });
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Café table. A cream café table with a scalloped pink valance along its front, and on the open shelf beneath it a round cat
   * bed where a cat sleeps curled up, its tail twitching now and then.
   */
  catcafedesk: {
    front({ x, y, s, tint }) {
      const cream = 0xf6eee0, creamDark = 0xd8ccb8, pink = 0xf2a0b8, pinkDark = 0xd8789a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(WHITE, 1)); fill(x, y + 1, 44, 4, tint(cream, 1));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      for (let k = 0; k < 44; k += 4) { fill(x + k, y + 5, 4, 2, pink); dot(x + k + 1, y + 7, pink); dot(x + k + 2, y + 7, pink); dot(x + k + 3, y + 6, pinkDark); }
      fill(x + 2, y + 7, 2, 9, creamDark); fill(x + 40, y + 7, 2, 9, creamDark); fill(x + 2, y + 14, 40, 2, cream); fill(x + 2, y + 14, 40, 1, WHITE);
      // The cat bed and its sleeper
      fill(x + 14, y + 10, 16, 4, 0x8ac8b8); fill(x + 15, y + 9, 14, 1, 0xa8dccc); fill(x + 16, y + 11, 12, 2, 0xc8ecdc);
      const kind = ['catcafecalico', 'catcafepersian', 'catcafetortie', 'catcafekitten'][Math.floor(x / 60) % 4];
      cafeCatAt(x + 22, y + 13, kind, 'sleep');
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Comic-panel desk. A black desk whose front is a strip of three comic panels on halftone yellow, cyan and pink: a city
   * skyline, a spider emblem and a "pow" burst; now and then one panel glitches a pixel aside in red and cyan.
   */
  spiderversedesk: {
    front({ x, y, s, tint }) {
      const top = 0x2e2a36, body = 0x1c1a22, ink = 0x14121a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x4a4456, 1)); fill(x, y + 1, 44, 4, tint(top, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(body, 0.9)); fill(x, y + 15, 44, 1, tint(ink, 0.9));
      // The three panels
      const t = (now + x * 0.31) % 16, gl = t < 0.4 ? Math.floor(now * 7) % 3 : -1;
      [[0xf2d35b, 0xe0b830], [0x5ac8c8, 0x3aa8b0], [0xf28fb0, 0xe06a98]].forEach(([c, dots], i) => {
        const px = x + 2 + i * 14, py = y + 6;
        if (gl === i) { fill(px - 1, py, 12, 8, 0xff3a6a, 0.6); fill(px + 1, py, 12, 8, 0x30e0f0, 0.6); }
        fill(px, py, 12, 8, c); for (let k = 0; k < 12; k += 2) for (let j = 0; j < 8; j += 2) dot(px + k + (j % 4 ? 1 : 0), py + j, dots);
        if (i === 0) { fill(px + 1, py + 4, 3, 4, ink); fill(px + 4, py + 2, 2, 6, ink); fill(px + 7, py + 5, 4, 3, ink); dot(px + 5, py + 4, 0xffd870); }
        if (i === 1) { fill(px + 6, py + 1, 1, 6, ink); for (const sd of [-1, 1]) { dot(px + 6 + sd, py + 3, ink); dot(px + 6 + sd * 2, py + 2, ink); dot(px + 6 + sd * 2, py + 6, ink); dot(px + 6 + sd, py + 5, ink); } }
        if (i === 2) { disc(px + 6, py + 4, 3, 0xf2d35b); fill(px + 4, py + 3, 5, 1, 0xd8202a); dot(px + 5, py + 5, 0xd8202a); dot(px + 7, py + 5, 0xd8202a); }
        fill(px, py, 12, 1, WHITE, 0.25);
      });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * X-console desk. A dark steel console with a curved lip, strips of blue light across its front and the X emblem in the
   * middle; little status lights along its edge blink in turn.
   */
  xmendesk: {
    front({ x, y, s, tint }) {
      const top = 0x4a5260, body = 0x2e3440, lip = 0x7a8290;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(lip, 1)); fill(x, y + 1, 44, 4, tint(top, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(body, 0.9)); fill(x, y + 5, 44, 1, tint(lip, 0.8)); fill(x, y + 15, 44, 1, tint(0x1e2430, 0.9));
      fill(x + 3, y + 8, 14, 1, 0x4a90ff); fill(x + 27, y + 8, 14, 1, 0x4a90ff); fill(x + 3, y + 12, 14, 1, 0x2a5aa0); fill(x + 27, y + 12, 14, 1, 0x2a5aa0);
      xmenX(x + 22, y + 10, 4);
      const k = Math.floor(now * 3 + x) % 6;
      for (let j = 0; j < 6; j++) dot(x + 3 + j * 2, y + 14, j === k ? 0x5ad86a : 0x2a4a30);
      if (isNight) glowLate(() => { fill(x + 3, y + 8, 14, 1, 0x7ab0ff); fill(x + 27, y + 8, 14, 1, 0x7ab0ff); });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Pit-wall desk. A carbon pit-wall stand painted down its front in team red with a white swoosh, a yellow shield and a
   * white number roundel; a row of shift lights along its lip runs green to red to blue now and then.
   */
  f1desk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x3a3c42, 1)); fill(x, y + 1, 44, 4, tint(0x24262a, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(0xd81a1a, 0.95)); fill(x, y + 5, 44, 1, tint(0xe84040, 1)); fill(x, y + 15, 44, 1, tint(0x9a1010, 0.9));
      line(x + 2, y + 13, x + 24, y + 9, WHITE); line(x + 2, y + 14, x + 24, y + 10, WHITE, 0.6);
      fill(x + 28, y + 7, 4, 5, 0xf2c41a); dot(x + 29, y + 8, 0x1c1a22); dot(x + 30, y + 9, 0x1c1a22); dot(x + 29, y + 11, 0x2a8a3a); dot(x + 31, y + 11, 0xd81a1a);
      disc(x + 37, y + 10, 3, WHITE); fill(x + 37, y + 8, 1, 5, 0x1c1a22); dot(x + 36, y + 9, 0x1c1a22);
      const t = (now + x * 0.31) % 9, n = t < 2 ? Math.floor(t * 6) : 0;
      for (let k = 0; k < 9; k++) dot(x + 15 + k * 2, y + 3, k < n ? (k < 3 ? 0x5ad86a : k < 6 ? 0xff3a2a : 0x4a8aff) : 0x3a3c42);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Cage-panel desk. A black desk with a red padded top edge, its front a panel of chain-link fence over dark red between two
   * black padded posts; the fence shudders now and then as if it took a hit.
   */
  ufcdesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xe84040, 1)); fill(x, y + 1, 44, 4, tint(0xd8202a, 0.85));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(0x2a1a1e, 0.9)); fill(x, y + 15, 44, 1, tint(0x14141a, 0.9));
      const t = (now + x * 0.33) % 18, sh = t < 0.5 ? Math.floor(now * 14) % 2 : 0;
      ufcFence(x + 4, y + 6 + sh, 36, 9, 0.75);
      fill(x, y + 5, 3, 11, tint(0x1c1a22, 1)); fill(x + 41, y + 5, 3, 11, tint(0x1c1a22, 1)); fill(x, y + 5, 1, 11, 0x3a3a42);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Steel-leg desk. A thick slab of reclaimed wood with its knots and saw marks, set on black steel trestle legs crossed
   * with a bar; a little brass lamp on the left corner clicks on at night.
   */
  nyloftdesk: {
    front({ x, y, s, tint }) {
      const slab = 0x9a6a3e, slabLight = 0xb8844e, slabDark = 0x6a4a2a, steel = 0x2a2a30;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(slabLight, 1)); fill(x, y + 1, 44, 4, tint(slab, 0.95)); fill(x, y + 4, 44, 1, tint(slabDark, 0.9));
      dot(x + 12, y + 2, slabDark); dot(x + 31, y + 3, slabDark); fill(x + 20, y + 2, 5, 1, slabDark, 0.4);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      // The trestle legs and their bar
      for (const lx of [x + 3, x + 37]) { line(lx, y + 5, lx + 4, y + 15, steel); line(lx + 4, y + 5, lx, y + 15, steel); fill(lx, y + 5, 5, 1, steel); fill(lx, y + 15, 5, 1, steel); }
      fill(x + 8, y + 10, 29, 1, steel);
      // The lamp
      fill(x + 1, y - 1, 4, 1, 0xc8a050); fill(x + 2, y - 7, 1, 6, 0xc8a050); line(x + 2, y - 7, x + 5, y - 9, 0xc8a050); fill(x + 4, y - 10, 3, 2, 0xc8a050);
      if (isNight) glowLate(() => { glowAt(x + 5, y - 7, 6, 5, 0xffd890, 0.2); dot(x + 5, y - 8, 0xfff4c0); });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Zinc counter desk. A café counter with a pewter-grey zinc top and rolled edge over a front of dark wood panels with
   * brass studs, a brass foot rail running along its foot; a little sugar bowl sits on its left end.
   */
  pariscafedesk: {
    front({ x, y, s, tint }) {
      const zinc = 0x9aa0a8, zincLight = 0xc8ccd4, zincDark = 0x6e747c, wood = 0x5a3424, woodDark = 0x3a2016, brass = 0xc8a050;
      drawMonitor(x + 6, y - 10);
      fill(x - 1, y, 46, 1, tint(zincLight, 1)); fill(x - 1, y + 1, 46, 3, tint(zinc, 0.95)); fill(x - 1, y + 4, 46, 1, tint(zincDark, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(wood, 0.95)); fill(x, y + 15, 44, 1, tint(woodDark, 0.9));
      for (const px of [x + 2, x + 16, x + 30]) { fill(px, y + 7, 12, 6, tint(woodDark, 0.9), 0.6); fill(px, y + 7, 12, 1, 0x7a4a32); dot(px + 6, y + 10, brass); }
      fill(x + 1, y + 13, 42, 1, brass); fill(x + 1, y + 13, 42, 1, 0xe8c878, 0.4);
      fill(x + 1, y - 3, 4, 3, WHITE); fill(x + 1, y - 4, 4, 1, 0xd8d4cc); dot(x + 2, y - 5, 0xc8a050);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Painted folk desk. A wooden desk painted turquoise, its front a panel of hand-painted flowers in pink, yellow and white
   * on a vine round a sugar skull, with a yellow border; a little marigold tucked on its left corner nods now and then.
   */
  muertosdesk: {
    front({ x, y, s, tint }) {
      const teal = 0x1a8a8a, tealLight = 0x2aa8a8, tealDark = 0x0e6a68;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xe8902a, 1)); fill(x, y + 1, 44, 4, tint(0xc86a3a, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(teal, 0.95)); fill(x, y + 5, 44, 1, tint(tealLight, 1)); fill(x, y + 15, 44, 1, tint(tealDark, 0.9));
      fill(x + 2, y + 7, 40, 1, 0xf2c41a); fill(x + 2, y + 13, 40, 1, 0xf2c41a);
      for (let k = 0; k < 38; k += 2) dot(x + 3 + k, y + 10 + Math.round(Math.sin(k * 0.4)), 0x2a6a3a);
      for (const [fx, c] of [[6, 0xe83a8a], [12, 0xf6f2ea], [30, 0xf6f2ea], [36, 0xe83a8a], [16, 0xf2c41a], [27, 0xf2c41a]]) { const fy = y + 10 + Math.round(Math.sin((fx - 3) * 0.4)); dot(x + fx, fy - 1, c); dot(x + fx - 1, fy, c); dot(x + fx + 1, fy, c); dot(x + fx, fy, 0xe8902a); }
      muertosSkull(x + 22, y + 10);
      const nod = (now + x * 0.29) % 10 < 1 ? 1 : 0;
      fill(x + 1, y - 3, 3, 3, 0xc86a3a); fill(x + 2, y - 7 + nod, 1, 4 - nod, 0x3a7a3a); fill(x + 1, y - 9 + nod, 3, 2, 0xe8902a); dot(x + 2, y - 9 + nod, 0xf2b03a);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Research console. A curved white console with an orange edge, its front set with a round sonar screen sweeping in green,
   * a strip of status lights and two little toggle switches; the sweep pings a contact now and then.
   */
  submarinedeepdesk: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xe8702a, 1)); fill(x, y + 1, 44, 4, tint(0xe8ecee, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(0xd0d6da, 0.95)); fill(x, y + 5, 44, 1, tint(0xf4f8fa, 1)); fill(x, y + 15, 44, 1, tint(0x8a96a0, 0.9));
      disc(x + 10, y + 10, 4, 0x0a1a14); ring(x + 10, y + 10, 4, 0x5a6670);
      const a = now * 2 + x; line(x + 10, y + 10, x + 10 + Math.round(Math.cos(a) * 3), y + 10 + Math.round(Math.sin(a) * 3), 0x5ad86a);
      if ((now + x) % 5 < 0.6) dot(x + 12, y + 8, 0x8af09a);
      for (let k = 0; k < 6; k++) dot(x + 20 + k * 3, y + 8, (Math.floor(now * 2) + k) % 6 ? 0x2ad8d8 : 0xe8702a);
      for (const tx of [x + 24, x + 32]) { fill(tx, y + 11, 3, 3, 0x2a3440); dot(tx + 1, y + 11, 0xa8b2ba); }
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
      if (isNight) glowLate(() => glowAt(x + 10, y + 10, 4, 4, 0x5ad86a, 0.15));
    },
  },
  /**
   * Flight-case desk. A desk built on a grey aluminium flight case with ridged edges, steel corner caps and two chunky
   * latches, a red station sticker and a strip of tape with a scribbled label; a little frost glints on its corners.
   */
  arcticbasedesk: {
    front({ x, y, s, tint }) {
      const alu = 0xa8b0b8, aluLight = 0xd0d6da, aluDark = 0x6a727a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(aluLight, 1)); fill(x, y + 1, 44, 4, tint(alu, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(0x8a929a, 0.95)); fill(x, y + 5, 44, 1, tint(aluLight, 1)); fill(x, y + 15, 44, 1, tint(aluDark, 0.9));
      for (let k = 0; k < 44; k += 4) fill(x + k, y + 9, 2, 1, aluDark, 0.5);
      for (const [cx, cy] of [[0, 5], [41, 5], [0, 13], [41, 13]]) { fill(x + cx, y + cy, 3, 3, 0x4a525a); dot(x + cx, y + cy, 0x8a929a); }
      for (const lx of [x + 12, x + 28]) { fill(lx, y + 7, 4, 4, 0x5a626a); fill(lx + 1, y + 8, 2, 2, 0xd0d6da); }
      fill(x + 18, y + 11, 6, 3, 0xb8302a); dot(x + 20, y + 12, WHITE); fill(x + 33, y + 11, 6, 2, 0xf2e6a0); dot(x + 34, y + 12, 0x2a2a30); dot(x + 36, y + 12, 0x2a2a30);
      if ((now + x * 0.2) % 9 < 0.5) dot(x + 1, y + 6, WHITE);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Birch blossom desk. A pale birch desk whose front panel is painted with a branch of cherry blossom across soft pink,
   * and a slim vase on its left corner holding a single blossoming twig, a petal falling from it now and then.
   */
  sakuradesk: {
    front({ x, y, s, tint }) {
      const birch = 0xe8d8bc, birchLight = 0xf4ead4, birchDark = 0xc8b494;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(birchLight, 1)); fill(x, y + 1, 44, 4, tint(birch, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(birch, 0.92)); fill(x, y + 15, 44, 1, tint(birchDark, 0.9));
      fill(x + 3, y + 7, 38, 7, 0xf6d8e0); fill(x + 3, y + 7, 38, 1, 0xfae4ea);
      line(x + 4, y + 13, x + 20, y + 9, 0x5a3a2e); line(x + 20, y + 9, x + 38, y + 11, 0x5a3a2e); line(x + 14, y + 11, x + 17, y + 8, 0x5a3a2e);
      for (const fx of [8, 13, 17, 22, 27, 32, 36]) { const fy = y + 10 + (fx % 3) - 1; dot(x + fx, fy, WHITE); dot(x + fx - 1, fy + 1, 0xf6b8c8); dot(x + fx + 1, fy + 1, 0xf6b8c8); dot(x + fx, fy + 1, 0xe87a98); }
      fill(x + 1, y - 5, 3, 5, 0xc8e0e8); fill(x + 1, y - 5, 1, 5, WHITE); line(x + 2, y - 5, x + 3, y - 11, 0x5a3a2e);
      for (const [dx, dy] of [[2, -9], [4, -11], [3, -8]]) { dot(x + dx, y + dy, 0xf6c8d4); dot(x + dx + 1, y + dy, WHITE); }
      const t = (now + x * 0.27) % 12;
      if (t < 1.6) dot(x + 4 + Math.round(Math.sin(t * 4)), y - 8 + Math.floor(t * 5), 0xf6c8d4);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Hogwarts school desk. Dark oak with three carved Gothic arches across its front, a stack of spellbooks on the left
   * corner and a candle on top of them whose flame wavers and glows warm at night.
   */
  potterdesk: {
    front({ x, y, s, tint }) {
      const oak = 0x5a3a22, oakLight = 0x7a5232, oakDark = 0x3a2414;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(oakLight, 1)); fill(x, y + 1, 44, 4, tint(oak, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(oak, 0.85)); fill(x, y + 15, 44, 1, tint(oakDark, 0.8));
      // The carved arches
      for (const ax of [x + 3, x + 17, x + 31]) {
        fill(ax, y + 9, 10, 5, oakDark, 0.55); fill(ax + 1, y + 8, 8, 1, oakDark, 0.55); fill(ax + 3, y + 7, 4, 1, oakDark, 0.55);
        fill(ax, y + 9, 1, 5, oakLight, 0.6); dot(ax + 1, y + 8, oakLight, 0.6); fill(ax + 4, y + 9, 2, 5, oakDark, 0.3);
      }
      // The spellbooks and the candle
      fill(x, y - 3, 6, 3, 0x7a1c1c); fill(x, y - 3, 6, 1, 0x9a2a22); dot(x + 5, y - 2, 0xd3a625);
      fill(x + 1, y - 5, 5, 2, 0x2a4a3a); fill(x + 1, y - 5, 5, 1, 0x3a6a4a);
      fill(x + 3, y - 9, 1, 4, 0xf2ead8); dot(x + 3, y - 9, WHITE, 0.6);
      const f = candlelight(x * 0.11);
      dot(x + 3, y - 10, 0xffd36b, f); dot(x + 3, y - 11, 0xff8a3d, 0.5 * f);
      if (isNight) glowLate(() => { glowAt(x + 3, y - 10, 6, 5, 0xffc060, 0.2 * f); dot(x + 3, y - 10, 0xfff4c0); });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Bilbo's writing desk. Rounded honey oak with a carved leaf border and little round drawer knobs, the Red Book open on the
   * left corner beside a quill in its ink, and a candle that flickers and glows warm at night.
   */
  lotrdesk: {
    front({ x, y, s, tint }) {
      const oak = 0xa8743e, oakLight = 0xc8904e, oakDark = 0x6a4424, brass = 0xc8a050;
      drawMonitor(x + 6, y - 10);
      fill(x + 1, y, 42, 1, tint(oakLight, 1)); dot(x, y + 1, tint(oakLight, 1)); dot(x + 43, y + 1, tint(oakLight, 1));
      fill(x, y + 1, 44, 4, tint(oak, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 10, tint(oak, 0.85)); fill(x + 1, y + 15, 42, 1, tint(oakDark, 0.8));
      // Two round-topped drawers with knobs, and the leaf border
      for (const dx of [4, 25]) { fill(x + dx, y + 7, 15, 6, oakDark, 0.35); fill(x + dx + 1, y + 6, 13, 1, oakDark, 0.35); dot(x + dx + 7, y + 10, brass); dot(x + dx + 7, y + 11, 0x8a6a2a); }
      for (let k = 2; k < 42; k += 4) dot(x + k, y + 5, 0x6a8a3a, 0.8);
      // The Red Book, the quill and ink, the candle
      fill(x, y - 2, 7, 2, 0x9a2a22); fill(x, y - 3, 3, 1, 0xf2ead8); fill(x + 4, y - 3, 3, 1, 0xf2ead8); dot(x + 3, y - 3, 0x6a1a14);
      fill(x + 21, y - 2, 2, 2, 0x2a2a30); dot(x + 21, y - 2, 0x4a4a52); line(x + 22, y - 3, x + 24, y - 7, 0xf2f0ea); dot(x + 24, y - 8, 0xd8d4cc);
      fill(x + 2, y - 8, 1, 5, 0xf2ead8);
      const f = candlelight(x * 0.13); dot(x + 2, y - 9, 0xffd36b, f); dot(x + 2, y - 10, 0xff8a3d, 0.5 * f);
      if (isNight) glowLate(() => { glowAt(x + 2, y - 9, 6, 5, 0xffc060, 0.2 * f); dot(x + 2, y - 9, 0xfff4c0); });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Lab bench. A stainless steel bench with a chemical-resistant black top, a rack of glassware on the left corner — a round
   * flask on a burner whose little blue flame flickers, its liquid bubbling now and then — and steel cupboard doors below.
   */
  breakingbaddesk: {
    front({ x, y, s, tint }) {
      const steel = 0xa8adb6, steelLight = 0xd0d4da, steelDark = 0x6e727c, top = 0x2a2e34;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x4a4e58, 1)); fill(x, y + 1, 44, 4, tint(top, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(steel, 0.95)); fill(x, y + 5, 44, 1, steelLight); fill(x, y + 15, 44, 1, steelDark);
      for (const dx of [2, 16, 30]) { fill(x + dx, y + 7, 12, 7, steelDark, 0.25); fill(x + dx, y + 7, 1, 7, steelLight, 0.7); fill(x + dx + 10, y + 9, 1, 3, steelDark); }
      // The flask on its burner
      fill(x + 1, y - 2, 4, 2, 0x3a3e46); const f = Math.floor(now * 8) % 2; dot(x + 2 + f, y - 3, 0x4a8aff); dot(x + 3 - f, y - 3, 0x8ac8ff, 0.8);
      fill(x + 1, y - 7, 5, 4, 0xdff0f6, 0.5); fill(x + 2, y - 9, 3, 2, 0xdff0f6, 0.5); fill(x + 1, y - 6, 5, 3, 0x6ac8f0, 0.85); dot(x + 1, y - 7, WHITE, 0.7);
      if ((now + x * 0.2) % 5 < 1.5) { dot(x + 3, y - 7 - Math.floor(now * 4) % 2, 0xc8f0ff); dot(x + 2, y - 11, 0xeef6fa, 0.5); }
      if (isNight) glowLate(() => glowAt(x + 3, y - 3, 3, 2, 0x4a8aff, 0.3));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * D&D table. A wood-panelled 80s desk with a laminate top, the campaign laid out on the left corner — a folded Dungeon
   * Master's screen and a pile of dice — and a walkie-talkie standing by, its little light blinking red now and then.
   */
  strangerdesk: {
    front({ x, y, s, tint }) {
      const top = 0xd8c8a0, panel = 0x7a5232, panelDark = 0x5a3a22;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xeadcb4, 1)); fill(x, y + 1, 44, 4, tint(top, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(panel, 0.9)); for (let k = 2; k < 44; k += 4) fill(x + k, y + 5, 1, 11, panelDark, 0.6);
      fill(x, y + 5, 44, 1, 0x9a6a40); fill(x, y + 15, 44, 1, panelDark);
      // The DM's screen and dice
      fill(x, y - 5, 5, 5, 0x8a2a2a); fill(x, y - 5, 5, 1, 0xa8403a); dot(x + 2, y - 3, 0xd8b45a);
      dot(x + 1, y - 1, 0xc83a8a); dot(x + 3, y - 1, 0x3a8ac8); dot(x + 5, y - 1, 0xe8c838);
      // The walkie-talkie
      fill(x + 20, y - 6, 3, 6, 0x2a2a30); fill(x + 21, y - 10, 1, 4, 0x4a4a52); dot(x + 21, y - 4, 0x6a6a72);
      const blink = (now + x * 0.2) % 6 < 0.4; dot(x + 22, y - 6, blink ? 0xff3a2a : 0x5a1a1a);
      if (isNight && blink) glowLate(() => dot(x + 22, y - 6, 0xff5a3a));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Small council table. Heavy dark oak with iron-bound corners and a carved band of interlocking knots, a stack of letters
   * sealed in red wax and a tall candle on the left corner whose flame wavers and glows at night.
   */
  gotdesk: {
    front({ x, y, s, tint }) {
      const oak = 0x4a3020, oakLight = 0x6a4a30, oakDark = 0x2e1c12, iron = 0x3a3a42;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(oakLight, 1)); fill(x, y + 1, 44, 4, tint(oak, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(oak, 0.85)); fill(x, y + 15, 44, 1, tint(oakDark, 0.8));
      for (let k = 3; k < 41; k += 4) { dot(x + k, y + 9, oakLight, 0.7); dot(x + k + 1, y + 10, oakLight, 0.7); dot(x + k + 2, y + 9, oakLight, 0.7); }
      fill(x + 2, y + 8, 40, 1, oakDark, 0.6); fill(x + 2, y + 12, 40, 1, oakDark, 0.6);
      for (const cx of [x, x + 41]) { fill(cx, y + 5, 3, 3, iron); fill(cx, y + 13, 3, 3, iron); dot(cx + 1, y + 6, 0x6a6a72); }
      // The letters and the candle
      fill(x, y - 3, 6, 3, 0xeadcb4); fill(x, y - 3, 6, 1, 0xf6ecd0); dot(x + 3, y - 2, 0xa8201c); dot(x + 1, y - 1, 0xc8b890);
      fill(x + 3, y - 10, 2, 7, 0xeee6d4); dot(x + 3, y - 10, WHITE, 0.6);
      const f = candlelight(x * 0.17); dot(x + 3, y - 11, 0xffd36b, f); dot(x + 3, y - 12, 0xff8a3d, 0.5 * f);
      if (isNight) glowLate(() => { glowAt(x + 3, y - 11, 6, 5, 0xffc060, 0.2 * f); dot(x + 3, y - 11, 0xfff4c0); });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Sector 7G console. A grey control panel desk from the power plant with rows of coloured buttons and two dials across its
   * front, a pink sprinkled donut left on the corner and a big red button that blinks now and then.
   */
  simpsonsconsole: {
    front({ x, y, s, tint }) {
      const grey = 0xa8aca0, greyLight = 0xc8ccc0, greyDark = 0x6e7268;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(greyLight, 1)); fill(x, y + 1, 44, 4, tint(grey, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(grey, 0.85)); fill(x, y + 5, 44, 1, greyLight); fill(x, y + 15, 44, 1, greyDark);
      const cols = [0xd8302a, 0xf2c830, 0x3ac85a, 0x3a8ae8];
      for (let k = 0; k < 8; k++) dot(x + 3 + k * 2, y + 8, cols[(k + Math.floor(now + x)) % 4]);
      for (let k = 0; k < 8; k++) dot(x + 3 + k * 2, y + 11, cols[(k * 3) % 4], 0.8);
      for (const dx of [25, 34]) { disc(x + dx, y + 10, 3, WHITE); ring(x + dx, y + 10, 3, greyDark); const a = Math.sin(now * 0.7 + dx) * 1.2 - Math.PI / 2; line(x + dx, y + 10, x + dx + Math.round(Math.cos(a) * 2), y + 10 + Math.round(Math.sin(a) * 2), 0xd8302a); }
      const blink = (now + x * 0.2) % 5 < 0.5; fill(x + 40, y + 8, 2, 2, blink ? 0xff5040 : 0xa8201c);
      // The donut
      fill(x, y - 2, 5, 2, 0xd89a5a); fill(x, y - 3, 5, 1, 0xf28ab0); dot(x + 2, y - 2, 0x8a5a2a); dot(x + 1, y - 3, 0x3a8ae8); dot(x + 3, y - 3, 0xf2e040); dot(x + 4, y - 2, 0xf28ab0);
      if (isNight && blink) glowLate(() => glowAt(x + 41, y + 9, 3, 2, 0xff5040, 0.3));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * M's desk. Polished walnut with a tooled green leather top and brass drawer pulls, and a red telephone on the left corner
   * whose handset shakes as it rings now and then, its dial lit at night.
   */
  bonddesk: {
    front({ x, y, s, tint }) {
      const walnut = 0x5a3a24, walnutLight = 0x7a5232, walnutDark = 0x3a2416, leather = 0x2a5a3a, brass = 0xc8a050;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(walnutLight, 1)); fill(x, y + 1, 44, 4, tint(leather, 1)); fill(x + 1, y + 1, 42, 1, 0xc8a050, 0.5); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(walnut, 0.9)); fill(x, y + 15, 44, 1, walnutDark);
      for (const dx of [3, 16, 29]) { fill(x + dx, y + 7, 12, 6, walnutDark, 0.35); fill(x + dx, y + 7, 12, 1, walnutLight, 0.6); fill(x + dx + 4, y + 10, 4, 1, brass); }
      // The red telephone
      const ring = (now + x * 0.23) % 18 < 1.2 ? Math.floor(now * 14) % 2 : 0;
      fill(x, y - 3, 6, 3, 0xc8281e); fill(x, y - 3, 6, 1, 0xe8503a); fill(x + ring, y - 5, 6, 2, 0xa01e1a); dot(x + 2, y - 2, WHITE, 0.8);
      if (isNight) glowLate(() => dot(x + 2, y - 2, 0xfff4c0, 0.8));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Arrakeen stone desk. A slab of dark polished stone on carved legs with a hawk inlaid in gold across its front, a woven
   * Fremen cloth over its left corner and a squat brass lamp of spice-orange glass that glows at night.
   */
  dunedesk: {
    front({ x, y, s, tint }) {
      const stone = 0x3a3e38, stoneLight = 0x5a5e56, stoneDark = 0x24261e, gold = 0xc8a050;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(stoneLight, 1)); fill(x, y + 1, 44, 4, tint(stone, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(stone, 0.85)); fill(x, y + 15, 44, 1, stoneDark);
      // The gold hawk, wings spread
      const cx = x + 22, cy = y + 10;
      fill(cx - 1, cy - 1, 3, 3, gold); line(cx - 1, cy, cx - 8, cy - 3, gold); line(cx + 1, cy, cx + 8, cy - 3, gold); line(cx - 1, cy + 1, cx - 6, cy - 1, gold, 0.7); line(cx + 1, cy + 1, cx + 6, cy - 1, gold, 0.7);
      dot(cx, cy - 2, gold); dot(cx, cy + 2, gold); dot(cx - 1, cy + 3, gold); dot(cx + 1, cy + 3, gold);
      // The cloth and the lamp
      fill(x, y, 6, 1, 0x8a3a24); fill(x, y + 1, 5, 7, 0x8a3a24); for (let r = 1; r < 8; r += 2) dot(x + 2, y + r, 0x2a3a6a); dot(x + 4, y + 8, 0xd8a868);
      fill(x + 1, y - 2, 4, 2, 0xc8a050); fill(x + 1, y - 5, 4, 3, 0xe8782e, 0.85); dot(x + 1, y - 5, 0xffb060); fill(x + 2, y - 6, 2, 1, 0xc8a050);
      glowAt(x + 3, y - 4, 4, 3, 0xff9a40, 0.08);
      if (isNight) glowLate(() => { glowAt(x + 3, y - 4, 7, 5, 0xff8a30, 0.22); fill(x + 2, y - 4, 2, 1, 0xffd090); });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Navigator's chart table. A ship's table of honey wood with rope-bound corners and a sea chart pinned across its front,
   * a Log Pose and a brass spyglass on the left corner; the Log Pose's needle wobbles now and then.
   */
  onepiecedesk: {
    front({ x, y, s, tint }) {
      const wood = 0xa8743e, woodLight = 0xc8904e, woodDark = 0x6a4424, rope = 0xd8b880;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodLight, 1)); fill(x, y + 1, 44, 4, tint(wood, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(wood, 0.85)); fill(x, y + 15, 44, 1, woodDark);
      // The chart
      fill(x + 6, y + 6, 32, 8, 0xeadcb4); fill(x + 6, y + 6, 32, 1, 0xf6ecd0);
      for (let k = 0; k < 30; k += 2) dot(x + 7 + k, y + 10 + Math.round(Math.sin(k * 0.4) * 1.5), 0x4a7ac8, 0.7);
      fill(x + 14, y + 8, 3, 2, 0x6a9a4a); fill(x + 28, y + 11, 4, 2, 0x6a9a4a); dot(x + 22, y + 9, 0xd8302a);
      for (const cx of [x, x + 42]) for (let k = 0; k < 4; k++) dot(cx + (k % 2), y + 6 + k * 2, rope);
      // The Log Pose and the spyglass
      fill(x, y - 2, 5, 2, 0x8a5a32); disc(x + 2, y - 4, 2, 0xdff0f6, 0.6); const a = (now + x * 0.2) % 8 < 1 ? Math.sin(now * 12) * 0.6 : 0; line(x + 2, y - 4, x + 2 + Math.round(Math.sin(a) * 2), y - 6, 0xd8302a);
      line(x + 20, y - 1, x + 26, y - 3, 0xc8a050); dot(x + 26, y - 3, 0xe8c878);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Roundtable Hold desk. Heavy dark oak with an iron-bound front inlaid with the golden ring and stroke of the Elden Ring, a tall candelabra of three candles
   * on the left corner whose flames waver and glow warm at night, and a worn map rolled beside it.
   */
  eldenringdesk: {
    front({ x, y, s, tint }) {
      const oak = 0x3e2a1c, oakLight = 0x5e4230, oakDark = 0x24180e, iron = 0x3a3a42, gold = 0xd8b45a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(oakLight, 1)); fill(x, y + 1, 44, 4, tint(oak, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(oak, 0.85)); fill(x, y + 15, 44, 1, oakDark); fill(x, y + 5, 44, 1, iron); fill(x, y + 14, 44, 1, iron);
      ellipseRing(x + 22, y + 10, 6, 3, gold, 0.85); ellipseRing(x + 22, y + 9, 3, 2, gold, 0.6); fill(x + 22, y + 6, 1, 9, gold, 0.85);
      // The candelabra and the rolled map
      fill(x + 1, y - 1, 5, 1, iron); fill(x + 3, y - 6, 1, 5, iron); fill(x + 1, y - 6, 5, 1, iron);
      for (const cx of [x + 1, x + 3, x + 5]) { fill(cx, y - 9 + (cx === x + 3 ? -1 : 0), 1, 3, 0xf2ead8); const f = candlelight(cx * 0.3); dot(cx, y - 10 + (cx === x + 3 ? -1 : 0), 0xffd36b, f); }
      fill(x + 20, y - 2, 8, 2, 0xc8b890); fill(x + 20, y - 2, 8, 1, 0xe0d0a8); dot(x + 27, y - 2, 0xa89870);
      if (isNight) glowLate(() => { glowAt(x + 3, y - 10, 7, 5, 0xffc060, 0.22); for (const cx of [x + 1, x + 3, x + 5]) dot(cx, y - 10 + (cx === x + 3 ? -1 : 0), 0xfff4c0); });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** Macrodata Refinement desk: a cream laminate console with a steel drawer unit and the retro Lumon terminal on top. */
  sevdesk: {
    front({ x, y, s, tint }) {
      const top = 0xcfc9b6, body = 0xb8b2a0, dark = 0x8c8775, steel = 0x9aa0a2;
      drawMonitor(x + 6, y - 10); // the macrodata terminal
      fill(x, y, 44, 1, tint(0xe2dcc8, 1)); fill(x, y + 1, 44, 4, tint(top, 0.96)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand); // the laminate top
      fill(x, y + 5, 44, 11, tint(body, 0.9)); fill(x, y + 15, 44, 1, tint(dark, 0.85)); // the desk box
      fill(x + 2, y + 7, 12, 7, tint(steel, 0.9)); fill(x + 2, y + 7, 12, 1, tint(0xc0c4c6, 0.95)); dot(x + 12, y + 10, tint(0x4a4e50, 1)); // a steel drawer unit
      drawKeyboard(s, y + 2); // the keypad
    },
  },
  /** A boxy beige Dunder Mifflin desk with a drawer stack, a chunky CRT and a little paper pile. */
  offdesk: {
    front({ x, y, s, tint }) {
      const top = 0xc8b48a, body = 0xb49e74, dark = 0x8a7850, panel = 0x9a8a60;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xdcc89a, 1)); fill(x, y + 1, 44, 4, tint(top, 0.96)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand); // the laminate top
      fill(x, y + 5, 44, 11, tint(body, 0.9)); fill(x, y + 15, 44, 1, tint(dark, 0.85)); // the desk box
      fill(x + 30, y + 6, 12, 8, tint(panel, 0.92)); fill(x + 30, y + 6, 12, 1, tint(0xb09a70, 0.95)); fill(x + 31, y + 9, 10, 1, dark); // a drawer stack
      fill(x + 3, y + 2, 8, 2, tint(0xf2efe6, 1)); // a stack of paper
      drawKeyboard(s, y + 2);
    },
  },
  /** A heavy dark-wood betting desk with a brass banker's lamp, a red ledger and a brass drawer pull. */
  peakydesk: {
    front({ x, y, s, tint }) {
      const wood = 0x5a4030, dark = 0x2e2016, brass = 0xc8a04a, green = 0x2a5a3a, ledger = 0x6a2a2a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x7a5a40, 1)); fill(x, y + 1, 44, 4, tint(wood, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand); // the top
      fill(x, y + 5, 44, 11, tint(0x4a3526, 0.9)); fill(x, y + 15, 44, 1, tint(dark, 0.85)); // the desk box
      fill(x + 1, y + 7, 7, 7, tint(0x3a2a1c, 0.9)); dot(x + 7, y + 10, tint(brass, 1)); // a drawer with a brass pull
      fill(x + 30, y + 1, 4, 2, green); fill(x + 30, y, 4, 1, 0x3a7a4a); fill(x + 31, y - 3, 2, 4, brass); fill(x + 30, y - 4, 4, 1, 0x8a6a2a); // a brass banker's lamp
      fill(x + 20, y + 2, 7, 2, ledger); fill(x + 20, y + 2, 7, 1, 0x8a3a3a); // a red ledger
      drawKeyboard(s, y + 2);
      if (isNight) glowAt(x + 32, y - 1, 5, 4, 0x6aca8a, 0.12); // the lamp's glow
    },
  },
  /** A sleek teak executive desk with chrome drawer pulls, a rotary phone and an inbox tray. */
  madmdesk: {
    front({ x, y, s, tint }) {
      const top = 0xa8703e, body = 0x8a5a32, dark = 0x5a3b25, chrome = 0xc0c4c8, phone = 0x2a2a30;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xc08a4e, 1)); fill(x, y + 1, 44, 4, tint(top, 0.96)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand); // the teak top
      fill(x, y + 5, 44, 11, tint(body, 0.9)); fill(x, y + 15, 44, 1, tint(dark, 0.85)); fill(x, y + 5, 44, 1, tint(0x9a6a3a, 0.95)); // the desk box
      fill(x + 1, y + 7, 3, 1, chrome); fill(x + 20, y + 7, 3, 1, chrome); // chrome drawer pulls
      fill(x + 30, y - 2, 6, 3, phone); fill(x + 30, y - 2, 6, 1, 0x4a4a52); dot(x + 33, y - 3, chrome); line(x + 36, y - 1, x + 38, y + 1, phone); // a rotary phone
      fill(x + 2, y - 2, 7, 2, tint(0x6a4a2a, 1)); fill(x + 3, y - 2, 5, 1, tint(0xe8dcc0, 1)); // an inbox tray
      drawKeyboard(s, y + 2);
    },
  },
  /** A teal dormitory-style desk with a pink trim line and a white player-number plate. */
  squiddesk: {
    front({ x, y, s, tint }) {
      const top = 0x3a9a8a, body = 0x2e8072, dark = 0x1f5a50, pink = 0xe84a7a, plate = 0xf0eeea;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x4ab0a0, 1)); fill(x, y + 1, 44, 4, tint(top, 0.96)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand); // the top
      fill(x, y + 5, 44, 11, tint(body, 0.9)); fill(x, y + 15, 44, 1, tint(dark, 0.85)); fill(x, y + 5, 44, 1, tint(pink, 0.95)); // the box with a pink trim
      fill(x + 31, y + 7, 10, 6, tint(plate, 0.96)); fill(x + 31, y + 7, 10, 1, WHITE); fill(x + 33, y + 9, 6, 2, tint(0x2a2a34, 1)); // a number plate
      drawKeyboard(s, y + 2);
    },
  },
  /** A sleek glass-and-steel corporate desk with a red trim line and a brushed-steel panel. */
  boysdesk: {
    front({ x, y, s, tint }) {
      const top = 0xdadee2, body = 0x3a4250, dark = 0x2a303a, steel = 0xb8bcc0, red = 0xd21f2a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xf0f2f4, 1)); fill(x, y + 1, 44, 4, tint(top, 0.96)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand); // a glass top
      fill(x, y + 5, 44, 11, tint(body, 0.9)); fill(x, y + 15, 44, 1, tint(dark, 0.85)); // the steel body
      fill(x, y + 5, 44, 1, tint(red, 0.95)); fill(x + 2, y + 8, 14, 5, tint(0x2a303a, 0.9)); fill(x + 2, y + 8, 14, 1, tint(steel, 0.9)); // a red trim and a panel
      fill(x + 30, y + 8, 12, 1, tint(steel, 0.9)); // a brushed-steel line
      drawKeyboard(s, y + 2);
    },
  },
  /** A riveted steel vault desk with a yellow hazard stripe, a drawer and a little Nuka-Cola. */
  falloutdesk: {
    front({ x, y, s, tint }) {
      const top = 0x7a7e84, body = 0x5a5e64, dark = 0x3e4248, steel = 0x9aa0a8, yellow = 0xf0c020, red = 0xd21f2a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x9aa0a8, 1)); fill(x, y + 1, 44, 4, tint(top, 0.96)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand); // the steel top
      fill(x, y + 5, 44, 11, tint(body, 0.9)); fill(x, y + 15, 44, 1, tint(dark, 0.85)); fill(x, y + 5, 44, 1, tint(yellow, 0.9)); // the box and a hazard stripe
      for (let i = 0; i < 4; i++) dot(x + 3 + i * 11, y + 13, tint(steel, 0.9)); // rivets
      fill(x + 2, y + 8, 12, 5, tint(dark, 0.9)); fill(x + 2, y + 8, 12, 1, tint(steel, 0.9)); // a riveted drawer
      fill(x + 30, y - 4, 2, 4, tint(red, 1)); dot(x + 30, y - 5, tint(0xf0f0ee, 1)); // a Nuka-Cola
      drawKeyboard(s, y + 2);
    },
  },
  /** A cluttered workbench: scratched wood, a drawer of wires and a little flask of glowing green goo. */
  rickdesk: {
    front({ x, y, s, tint }) {
      const top = 0x8a6a42, body = 0x6a4a2a, dark = 0x4a3420, metal = 0x8a8e94, green = 0x3ae84a, red = 0xd84a2a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xa07a4a, 1)); fill(x, y + 1, 44, 4, tint(top, 0.95)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand); // the benchtop
      fill(x, y + 5, 44, 11, tint(body, 0.9)); fill(x, y + 15, 44, 1, tint(dark, 0.85)); // the bench box
      fill(x + 2, y + 7, 12, 7, tint(0x3a4248, 0.9)); fill(x + 3, y + 8, 3, 1, tint(green, 0.9)); fill(x + 7, y + 10, 3, 1, tint(red, 0.9)); // a drawer of junk
      fill(x + 30, y - 4, 2, 4, tint(metal, 1)); fill(x + 30, y - 6, 2, 2, tint(green, 1)); dot(x + 31, y - 7, tint(0x7af06a, 1)); // a goo flask
      drawKeyboard(s, y + 2);
      if (isNight) glowAt(x + 31, y - 5, 3, 3, green, 0.12);
    },
  },
  /** A minimalist glossy white desk with an embedded cyan glass control strip. */
  mirrordesk: {
    front({ x, y, s, tint }) {
      const top = 0xeef0f2, body = 0xe0e4e8, dark = 0xc8cdd4, cyan = 0x5adce8, glass = 0xcfe0e8;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xf6f8fa, 1)); fill(x, y + 1, 44, 4, tint(top, 0.97)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand); // the glossy top
      fill(x, y + 5, 44, 11, tint(body, 0.94)); fill(x, y + 15, 44, 1, tint(dark, 0.9)); // the clean body
      fill(x + 2, y + 2, 10, 2, tint(glass, 0.9)); fill(x + 2, y + 2, 10, 1, tint(cyan, 0.9)); // a glass control strip
      fill(x + 30, y + 8, 12, 1, tint(dark, 0.9)); // a seam
      drawKeyboard(s, y + 2);
      if (isNight) glowAt(x + 7, y + 2, 5, 2, cyan, 0.1);
    },
  },
  /** A clean white diagnostic desk with a glowing tablet and a host-arm component laid out. */
  westdesk: {
    front({ x, y, s, tint }) {
      const top = 0xe8eae8, body = 0xdcdedc, dark = 0xc0c4c2, cyan = 0x8ad0d8, metal = 0x9aa0a8, host = 0xc8b8a8;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xf2f4f2, 1)); fill(x, y + 1, 44, 4, tint(top, 0.97)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand); // the clean top
      fill(x, y + 5, 44, 11, tint(body, 0.94)); fill(x, y + 15, 44, 1, tint(dark, 0.9)); // the body
      fill(x + 2, y + 2, 9, 3, tint(0x1a2a2e, 0.9)); fill(x + 3, y + 3, 3, 1, tint(cyan, 0.9)); // a diagnostic tablet
      fill(x + 28, y - 1, 10, 2, tint(host, 1)); fill(x + 28, y - 1, 10, 1, tint(0xe0d0c0, 1)); fill(x + 36, y - 2, 3, 2, tint(metal, 1)); // a host-arm component
      drawKeyboard(s, y + 2);
      if (isNight) glowAt(x + 5, y + 3, 4, 2, cyan, 0.1);
    },
  },
  /** A garage workbench: a steel-topped bench with a pegboard apron and a little bench vise at one end. */
  bttfbench: {
    front({ x, y, s, tint }) {
      const steel = 0xb4b8c0, steelL = 0xd6dae0, steelD = 0x7e828a, red = 0xd8402a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(steelL, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 1, 44, 4, tint(steel, 0.95)); fill(x, y + 5, 44, 2, tint(steelD, 0.8)); // the top
      fill(x, y + 7, 44, 9, tint(0x6a6e76, 0.7));                              // the apron
      for (let px = x + 3; px < x + 42; px += 4) for (let py = y + 9; py < y + 15; py += 3) dot(px, py, tint(0x4a4e56, 0.9)); // pegboard holes
      fill(x + 36, y - 2, 6, 3, steelD); fill(x + 37, y - 3, 2, 1, red);       // the vise
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A park control console: a grey desk under the monitor with rows of status lights and chunky switches. */
  jpcontroldesk: {
    front({ x, y, s, tint }) {
      const grey = 0x9a9e98, greyL = 0xc0c4be, greyD = 0x6a6e68, amber = 0xe8c838, green = 0x6ef06a, red = 0xd83020;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(greyL, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 1, 44, 5, tint(grey, 0.95)); fill(x, y + 6, 44, 10, tint(greyD, 0.8));
      for (let k = 0; k < 10; k++) { const lx = x + 3 + k * 4, on = (Math.floor(now * 2) + k) % 3 === 0; dot(lx, y + 3, on ? green : 0x3a5a3a); dot(lx, y + 5, (k % 2) ? amber : red); }
      for (let k = 0; k < 5; k++) fill(x + 4 + k * 8, y + 8, 3, 4, tint(0x4a4e48, 0.9));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A concierge desk: dark polished wood with a gold front rail, a brass bell and a ledger. */
  wickconcierge: {
    front({ x, y, s, tint }) {
      const wood = 0x2a1e18, woodL = 0x4a3426, woodD = 0x180f0a, gold = 0xd6b25e, goldL = 0xf2d888;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodL, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 1, 44, 5, tint(wood, 0.95)); fill(x, y + 6, 44, 10, tint(woodD, 0.85));
      fill(x, y + 6, 44, 1, tint(gold, 0.9)); fill(x, y + 15, 44, 1, tint(goldL, 0.7)); // the gold rails
      disc(x + 36, y - 1, 2, gold); dot(x + 35, y - 2, goldL);                  // the bell
      fill(x + 3, y - 2, 6, 2, 0xe6dcc0); fill(x + 3, y - 2, 6, 1, 0xf2ead2);    // the ledger
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A tailor's cutting table: a cloth-topped bench with chalk lines, a pinned pattern and a row of spooled thread. */
  kingsmancuttingtable: {
    front({ x, y, s, tint }) {
      const wood = 0x5a4230, woodL = 0x745838, woodD = 0x3a2a1c, cloth = 0x1e2a44, chalk = 0xdfe3ea, gold = 0xc8a24a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodL, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 1, 44, 5, tint(wood, 0.95)); fill(x, y + 6, 44, 10, tint(woodD, 0.85));
      fill(x + 2, y + 1, 26, 4, tint(cloth, 0.95));                             // the bolt of cloth
      for (const [ax, ay, bx, by] of [[4, 2, 14, 2], [6, 4, 24, 4], [16, 1, 16, 5]]) line(x + ax, y + ay, x + bx, y + by, chalk); // chalk lines
      for (let k = 0; k < 4; k++) { const sx = x + 32 + k * 3; fill(sx, y + 8, 2, 4, [0x8a1a1a, 0x1e2a44, gold, 0x2a2a30][k]); } // thread spools
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** An inventing-room desk: dark wood cluttered with bubbling beakers of candy syrup and a little copper contraption. */
  wonkadesk: {
    front({ x, y, s, tint }) {
      const wood = 0x5a3418, woodL = 0x7a4a26, woodD = 0x3a2010, copper = 0xc8824a, pink = 0xf06aa8, mint = 0x5ad0a0, gold = 0xe8c838;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodL, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 1, 44, 5, tint(wood, 0.95)); fill(x, y + 6, 44, 10, tint(woodD, 0.85));
      // beakers of syrup
      for (let k = 0; k < 3; k++) { const bx = x + 3 + k * 5, c = [pink, mint, gold][k]; fill(bx, y - 4, 3, 4, c, 0.85); fill(bx, y - 5, 3, 1, 0xdfe3ea); const bub = Math.floor(now * 3 + k) % 3; dot(bx + 1, y - 3 - bub, shaded(c, 1.3)); }
      // the copper contraption
      fill(x + 32, y - 5, 8, 5, copper); fill(x + 32, y - 5, 8, 1, 0xe0a070); fill(x + 35, y - 7, 2, 2, copper);
      const sp = Math.floor(now * 4) % 2; dot(x + 36, y - 3, sp ? gold : 0xb06030);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A reception desk: cream-panelled with a mustard trim, a brass bell and a tidy ledger, symmetrical. */
  wesconcierge: {
    front({ x, y, s, tint }) {
      const cream = 0xe8dcc0, creamL = 0xf2ead2, creamD = 0xc0b088, mustard = 0xd8a848, brass = 0xc8a24a, rose = 0xd87a8a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(creamL, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 1, 44, 5, tint(cream, 0.95)); fill(x, y + 6, 44, 10, tint(creamD, 0.85));
      fill(x, y + 6, 44, 1, tint(mustard, 0.9)); fill(x + 4, y + 9, 36, 5, tint(creamD, 0.75)); // trim + panel
      for (let k = 0; k < 4; k++) fill(x + 6 + k * 9, y + 9, 5, 5, tint(cream, 0.9)); // the symmetrical panels
      disc(x + 36, y - 1, 2, brass); dot(x + 35, y - 2, 0xe8c878);              // the bell
      fill(x + 3, y - 2, 6, 2, rose); fill(x + 3, y - 2, 6, 1, 0xe89aa6);        // the ledger
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** An agent desk: a brushed-steel console with a green readout strip, a row of toggle lights and a gadget drawer. */
  mibdesk: {
    front({ x, y, s, tint }) {
      const steel = 0x9aa0a8, steelL = 0xc4c8ce, steelD = 0x5a5e66, green = 0x5ad86a, red = 0xd83020, amber = 0xe8a830;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(steelL, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 1, 44, 5, tint(steel, 0.95)); fill(x, y + 6, 44, 10, tint(steelD, 0.85));
      fill(x + 3, y + 2, 20, 1, tint(0x143a24, 1)); for (let k = 0; k < 6; k++) dot(x + 4 + k * 3, y + 2, (Math.floor(now * 3) + k) % 4 ? green : 0x2a6a3a); // readout
      for (let k = 0; k < 5; k++) dot(x + 28 + k * 2, y + 3, (k % 2) ? amber : red); // toggles
      fill(x + 30, y + 8, 10, 5, tint(0x6a6e76, 0.8)); fill(x + 33, y + 10, 4, 1, steelL); // the drawer
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A terminal desk: a black slab edged in glowing cyan light with a bright trace running along its front. */
  trondesk: {
    front({ x, y, s, tint }) {
      const slab = 0x12202c, slabL = 0x22303c, slabD = 0x0a1016, cyan = 0x3ad0e8, cyanL = 0x9ff0ff;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(cyan, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 1, 44, 5, tint(slab, 0.95)); fill(x, y + 6, 44, 10, tint(slabD, 0.9));
      fill(x, y + 6, 44, 1, tint(cyan, 0.9));                                     // the front light line
      const p = Math.floor(now * 8) % 44; fill(x + p, y + 6, 4, 1, cyanL);        // a travelling pulse
      for (let k = 0; k < 3; k++) fill(x + 4 + k * 14, y + 9, 10, 1, tint(0x1a5a6a, 0.8));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      if (isNight) glowLate(() => glowAt(x + 22, y + 6, 22, 2, cyan, 0.12));
      drawKeyboard(s, y + 2);
    },
  },
  /** A training console: a white Capsule Corp desk with a blue gravity-setting dial and a row of readout lights. */
  dbzdesk: {
    front({ x, y, s, tint }) {
      const white = 0xdfe6ee, whiteL = 0xf0f4f8, whiteD = 0xb0b8c2, blue = 0x3a6ac0, green = 0x5ad04a, orange = 0xe87828;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(whiteL, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 1, 44, 5, tint(white, 0.97)); fill(x, y + 6, 44, 10, tint(whiteD, 0.9));
      fill(x, y + 6, 44, 1, tint(blue, 0.9));
      disc(x + 36, y + 10, 3, tint(blue, 0.9)); const a = now; line(x + 36, y + 10, x + 36 + Math.round(Math.cos(a) * 2), y + 10 + Math.round(Math.sin(a) * 2), whiteL); // gravity dial
      for (let k = 0; k < 6; k++) dot(x + 4 + k * 3, y + 3, (Math.floor(now * 2) + k) % 3 ? green : orange); // readout lights
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A control station: a dark metal console with a coloured task-light strip and a chunky lever. */
  amongusdesk: {
    front({ x, y, s, tint }) {
      const metal = 0x3a4450, metalL = 0x56626e, metalD = 0x24303a, red = 0xd83020, green = 0x4ad04a, blue = 0x3a6ad8, yellow = 0xf2d84a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(metalL, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 1, 44, 5, tint(metal, 0.95)); fill(x, y + 6, 44, 10, tint(metalD, 0.9));
      const cols = [red, green, blue, yellow]; for (let k = 0; k < 8; k++) dot(x + 4 + k * 3, y + 3, (Math.floor(now * 2) + k) % 3 ? cols[k % 4] : 0x2a2e36); // task lights
      fill(x + 32, y + 8, 8, 5, tint(metalD, 0.8)); fill(x + 35, y + 6, 2, 3, tint(metalL, 0.9)); dot(x + 35, y + 6, red); // the lever
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Izakaya counter. A warm cedar bar top with a grain line and a dark front of slats, a little row of sake bottles
   * on the far corner and a tiny red paper lantern on a cord that sways and glows warm at night.
   */
  tokyodesk: {
    front({ x, y, s, tint }) {
      const cedar = 0xc89a5e, cedarLight = 0xe0b878, cedarDark = 0x6f5236, slat = 0x4a3828, red = 0xd83a2a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(cedarLight, 1)); fill(x, y + 1, 44, 4, tint(cedar, 1));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 3, 44, 1, 0xb08a52, 0.6);
      fill(x, y + 5, 44, 11, slat); fill(x, y + 5, 44, 1, 0x5e4836);
      for (let k = x + 4; k < x + 44; k += 6) fill(k, y + 6, 1, 9, 0x362a1e);
      fill(x, y + 15, 44, 1, 0x241a12);
      // The sake bottles on the far corner
      for (const [bx, c] of [[x + 1, 0x3a6a4a], [x + 4, 0xd8d0c0]]) { fill(bx, y - 4, 2, 4, c); dot(bx, y - 5, c); dot(bx, y - 4, shaded(c, 1.3)); }
      // The little hanging lantern
      const sway = Math.round(Math.sin(now * 1.3 + x * 0.1));
      fill(x + 38 + sway, y - 8, 1, 2, 0x3a2a1e); fill(x + 36 + sway, y - 6, 5, 4, red); fill(x + 36 + sway, y - 6, 5, 1, 0xe85a3a); fill(x + 37 + sway, y - 3, 3, 1, 0xf2d35b);
      dot(x + 38 + sway, y - 5, WHITE, 0.4);
      if (isNight) glowLate(() => { glowAt(x + 38 + sway, y - 4, 5, 4, 0xffb060, 0.22); fill(x + 36 + sway, y - 6, 5, 4, 0xff7a4a); });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Scribe's bench. A sandstone bench faced with a painted frieze of hieroglyph squares in ochre and blue between gold bands,
   * a canopic jar with a jackal head on the far corner and a small papyrus roll; the whole thing banded in gold.
   */
  egyptdesk: {
    front({ x, y, s, tint }) {
      const stone = 0xc9a86a, stoneLight = 0xe0c488, stoneDark = 0x9a7a44, gold = 0xd8b24e, ochre = 0xb84a32, blue = 0x1f4e8a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(stoneLight, 1)); fill(x, y + 1, 44, 4, tint(stone, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, stone); fill(x, y + 5, 44, 1, gold); fill(x, y + 15, 44, 1, stoneDark); fill(x, y + 10, 44, 1, gold);
      // A register of little hieroglyph squares
      for (let k = 0; k < 9; k++) { const gx = x + 3 + k * 5, c = [ochre, blue, 0x3ab8a0, 0x241a12][hash(gx, 7) % 4]; fill(gx, y + 6, 3, 3, c, 0.9); dot(gx + 1, y + 7, shaded(c, 1.4)); }
      for (let k = 0; k < 7; k++) { const gx = x + 5 + k * 5; fill(gx, y + 12, 3, 2, [blue, ochre, gold][hash(gx, 3) % 3], 0.85); }
      // The canopic jar and the papyrus roll
      fill(x + 1, y - 4, 4, 4, 0xe0d2a8); fill(x + 1, y - 6, 4, 2, 0x3a6ab0); fill(x + 1, y - 7, 4, 1, 0x241a12); dot(x + 2, y - 6, 0x241a12); dot(x + 4, y - 6, 0x241a12);
      fill(x + 37, y - 2, 6, 2, 0xe8dcbe); fill(x + 37, y - 2, 6, 1, 0xf4ecd8); dot(x + 37, y - 1, ochre); dot(x + 42, y - 1, ochre);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Carved oak table. A massive dark oak bench banded with iron and carved with a run of knotwork along the front, a drinking
   * horn resting in an iron bracket on the far corner and a little carved rune burned into the wood.
   */
  vikingdesk: {
    front({ x, y, s, tint }) {
      const oak = 0x6a4a2e, oakLight = 0x8a6440, oakDark = 0x3a2818, iron = 0x4a4e56, ironLight = 0x6a6e76, horn = 0xe0d2a8;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(oakLight, 1)); fill(x, y + 1, 44, 4, tint(oak, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, oak); fill(x, y + 5, 44, 1, oakLight); fill(x, y + 15, 44, 1, oakDark);
      // Iron corner-bands
      for (const bx of [x + 1, x + 40]) { fill(bx, y + 5, 3, 11, iron); fill(bx, y + 5, 1, 11, ironLight); for (const by of [y + 7, y + 13]) dot(bx + 1, by, 0x2a2e36); }
      // A run of knotwork
      for (let k = x + 6; k < x + 38; k += 4) { line(k, y + 8, k + 2, y + 12, oakDark); line(k + 2, y + 8, k, y + 12, oakDark); dot(k + 1, y + 10, oakLight); }
      // The drinking horn in its bracket, and a burned rune
      fill(x + 36, y - 3, 8, 2, horn); fill(x + 42, y - 4, 2, 2, horn); fill(x + 36, y - 3, 1, 2, 0xcfc0a0); fill(x + 35, y - 1, 10, 1, iron);
      fill(x + 20, y + 10, 1, 4, oakDark); fill(x + 18, y + 11, 5, 1, oakDark); fill(x + 20, y + 11, 3, 2, oakDark);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Stone altar desk. A heavy carved stone block faced with a band of Aztec glyphs in red and jade between thin gold lines,
   * a small gold idol on the far corner and a stepped-fret carving down the middle; moss in the lower joints.
   */
  aztecdesk: {
    front({ x, y, s, tint }) {
      const stone = 0x8a7f6e, stoneLight = 0xa89c88, stoneDark = 0x55504a, gold = 0xd8b24e, red = 0xb83a2a, jade = 0x3ab8a0, moss = 0x4a5a3a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(stoneLight, 1)); fill(x, y + 1, 44, 4, tint(stone, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, stone); fill(x, y + 5, 44, 1, stoneLight); fill(x, y + 15, 44, 1, stoneDark); fill(x, y + 9, 44, 1, gold); fill(x, y + 13, 44, 1, gold);
      // A band of glyph squares
      for (let k = 0; k < 9; k++) { const gx = x + 3 + k * 5, c = [red, jade, 0x6a6054, stoneDark][hash(gx, 5) % 4]; fill(gx, y + 10, 3, 3, c, 0.9); dot(gx + 1, y + 11, shaded(c, 1.4)); }
      // A stepped-fret (greca) motif
      for (let k = x + 4; k < x + 40; k += 8) { fill(k, y + 6, 4, 1, stoneDark); fill(k + 3, y + 6, 1, 2, stoneDark); fill(k + 3, y + 7, 3, 1, stoneDark); }
      // the gold idol and moss
      fill(x + 1, y - 5, 4, 5, gold); fill(x + 1, y - 5, 4, 1, 0xf2d888); fill(x + 1, y - 6, 4, 1, gold); dot(x + 2, y - 4, red); dot(x + 4, y - 4, red); fill(x + 1, y - 2, 4, 1, stoneDark);
      fill(x, y + 15, 10, 1, moss, 0.5); fill(x + 34, y + 15, 10, 1, moss, 0.5);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Astronomer's desk. A dark walnut desk with a brass-edged top, a pinned star chart across the front, a little brass desk
   * telescope on a tripod on the far corner and a scatter of pale star marks on the chart.
   */
  obsdesk: {
    front({ x, y, s, tint }) {
      const wood = 0x3a3040, woodLight = 0x4e4456, woodDark = 0x241e2a, brass = 0xc8a050, brassLight = 0xe8c878, chart = 0x24304a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodLight, 1)); fill(x, y + 1, 44, 4, tint(wood, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 1, brass); fill(x, y + 6, 44, 10, wood); fill(x, y + 15, 44, 1, woodDark);
      // the pinned star chart
      fill(x + 4, y + 7, 36, 8, chart); fill(x + 4, y + 7, 36, 1, 0x34446a); dot(x + 4, y + 7, brass); dot(x + 39, y + 7, brass);
      for (let k = 0; k < 14; k++) { const sx = x + 6 + (hash(x + k, 3) % 34), sy = y + 8 + (hash(k, x) % 6); dot(sx, sy, hash(k, 2) % 4 ? 0xcfd6ea : 0xf2d888, 0.8); }
      // a couple of constellation lines
      line(x + 9, y + 9, x + 14, y + 11, 0x8a94b4, 0.6); line(x + 14, y + 11, x + 19, y + 10, 0x8a94b4, 0.6);
      // the desk telescope
      fill(x + 38, y - 1, 1, 1, brass); line(x + 36, y, x + 40, y, 0x6a5a3a); fill(x + 37, y - 6, 2, 1, brass); line(x + 37, y - 5, x + 41, y - 8, brass); fill(x + 41, y - 9, 2, 2, brassLight); dot(x + 41, y - 9, 0xfff0c0);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Tiki bar counter. A bamboo-fronted bar with a split-bamboo facing lashed with twine, a dark varnished wood top, a thatch
   * trim along the front edge and a little paper umbrella drink on the far corner.
   */
  tikidesk: {
    front({ x, y, s, tint }) {
      const bamboo = 0xb89a5a, bambooLight = 0xd8bc7e, bambooDark = 0x8a6a3a, top = 0x5a3b25, thatch = 0x9a7a3a, twine = 0x6a5228;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0x7a5230, 1)); fill(x, y + 1, 44, 4, tint(top, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      // thatch trim
      for (let k = x; k < x + 44; k += 3) { fill(k, y + 5, 2, 2, thatch); dot(k, y + 6, 0x7a5e2a); }
      // split-bamboo facing
      fill(x, y + 7, 44, 9, bamboo); for (let k = x + 1; k < x + 44; k += 3) { fill(k, y + 7, 1, 9, bambooDark, 0.6); fill(k - 1, y + 7, 1, 9, bambooLight, 0.4); }
      for (const ly of [y + 9, y + 13]) fill(x, ly, 44, 1, twine, 0.7); // twine lashings
      fill(x, y + 15, 44, 1, 0x3a2418);
      // the umbrella drink
      fill(x + 38, y - 4, 3, 4, 0xf2b040); fill(x + 38, y - 4, 3, 1, 0xf6d070); fill(x + 39, y - 7, 1, 3, 0x6a4a2e);
      fill(x + 36, y - 8, 7, 1, 0xe8404a); fill(x + 37, y - 9, 5, 1, 0xf28fb0); dot(x + 41, y - 5, 0x4f9a5a);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Chrome booth table. A diner table with a red boomerang-pattern laminate top edged in ribbed chrome, a chrome-legged
   * red vinyl front, and a little tabletop napkin caddy with salt and pepper on the far corner.
   */
  dinerdesk: {
    front({ x, y, s, tint }) {
      const chrome = 0xc8ccd2, chromeLight = 0xe8ecf0, chromeDark = 0x8a8e96, laminate = 0xe8e0d0, red = 0xe03a4a, vinyl = 0xd83040, cream = 0xf0ece0;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(chromeLight, 1)); fill(x, y + 1, 44, 3, tint(laminate, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      for (let k = x + 2; k < x + 44; k += 6) { dot(k, y + 2, 0xb83040); line(k, y + 2, k + 3, y + 3, 0xb83040); } // boomerang flecks
      fill(x, y + 4, 44, 2, chrome); fill(x, y + 4, 44, 1, chromeLight); fill(x, y + 5, 44, 1, chromeDark); // ribbed chrome edge
      fill(x, y + 6, 44, 9, vinyl); fill(x, y + 6, 44, 1, red); fill(x, y + 15, 44, 1, 0x8a1820);
      for (let k = x + 3; k < x + 44; k += 7) fill(k, y + 7, 1, 7, 0xb82838, 0.6); // vinyl pleats
      fill(x + 4, y + 15, 2, 3, chromeDark); fill(x + 38, y + 15, 2, 3, chromeDark); // chrome legs
      // the napkin caddy + salt and pepper
      fill(x + 38, y - 3, 4, 3, chrome); fill(x + 38, y - 3, 4, 1, chromeLight); fill(x + 39, y - 4, 2, 1, cream);
      fill(x + 35, y - 2, 1, 2, cream); dot(x + 35, y - 3, 0xc8ccd2); fill(x + 43, y - 2, 1, 2, red); dot(x + 43, y - 3, 0xc8ccd2);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Counsel's bench. A solid dark-oak table with a panelled front and a brass trim rail, a green glass banker's lamp glowing
   * warm on the far corner and a neat stack of case files with a red ribbon on the near side.
   */
  courtcounsel: {
    front({ x, y, s, tint }) {
      const oak = 0x6a4a2e, oakLight = 0x8a6440, oakDark = 0x4a3220, brass = 0xc8a050, green = 0x2a6a4a, paper = 0xf0ece0;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(oakLight, 1)); fill(x, y + 1, 44, 4, tint(oak, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 1, brass); fill(x, y + 6, 44, 9, oak); fill(x, y + 15, 44, 1, oakDark);
      // raised panels on the front
      for (const px of [x + 3, x + 18, x + 33]) { fill(px, y + 8, 9, 5, oakDark, 0.4); fill(px, y + 8, 9, 1, oakLight, 0.4); }
      // the banker's lamp
      const lamp = () => { fill(x + 37, y - 6, 7, 3, green); fill(x + 37, y - 6, 7, 1, 0x3a8a5a); fill(x + 37, y - 3, 7, 1, 0x1a4a30); fill(x + 38, y - 3, 5, 1, 0xd8f0d0); };
      fill(x + 40, y - 2, 1, 2, brass); fill(x + 38, y, 5, 1, brass); lamp();
      if (isNight) glowLate(() => { glowAt(x + 40, y - 3, 7, 4, 0x6ad89a, 0.2); lamp(); });
      else glowAt(x + 40, y - 3, 6, 3, 0x6ad89a, 0.06);
      // a stack of case files with a ribbon
      fill(x + 1, y - 3, 7, 3, paper); fill(x + 1, y - 3, 7, 1, 0xf8f4ea); fill(x + 1, y - 1, 7, 1, 0xd0c8b4); fill(x + 4, y - 4, 1, 4, 0xd83040);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Drafting table. A pale-wood slanted drafting table with a paper-clamp rail, a pinned sketch taped to the front, a little
   * trough of pencils and a few paint dabs on the wood, a clip-on lamp arcing over one corner.
   */
  artdesk: {
    front({ x, y, s, tint }) {
      const wood = 0xc8b48a, woodLight = 0xe0cc9e, woodDark = 0x9a8460, paper = 0xf0ece0, metal = 0x8a8e96;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodLight, 1)); fill(x, y + 1, 44, 4, tint(wood, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 1, metal); // the clamp rail
      fill(x, y + 6, 44, 9, wood); fill(x, y + 6, 44, 1, woodLight); fill(x, y + 15, 44, 1, woodDark);
      // a pinned sketch taped to the front
      fill(x + 5, y + 7, 14, 7, paper); fill(x + 5, y + 7, 14, 1, 0xf8f4ea); for (const cx of [x + 5, x + 18]) fill(cx, y + 7, 1, 1, 0xd8d0b8);
      line(x + 8, y + 10, x + 13, y + 9, 0x8a7a6a, 0.8); line(x + 13, y + 9, x + 16, y + 12, 0x8a7a6a, 0.8); dot(x + 11, y + 11, 0x6a5a4a);
      // paint dabs + a pencil trough
      dot(x + 24, y + 8, 0xe8404a); dot(x + 27, y + 10, 0x4a8ad8); dot(x + 30, y + 8, 0xf2c040);
      fill(x + 34, y + 11, 8, 2, woodDark); for (let k = 0; k < 4; k++) fill(x + 35 + k * 2, y + 9, 1, 2, [0xe8404a, 0x4ac86a, 0x4a8ad8, 0xf2c040][k]);
      // the clip-on lamp
      fill(x + 40, y - 1, 1, 1, metal); line(x + 40, y - 1, x + 43, y - 6, metal); fill(x + 42, y - 8, 3, 2, metal); fill(x + 42, y - 6, 3, 1, 0xfff0c0);
      if (isNight) glowLate(() => glowAt(x + 43, y - 5, 6, 4, 0xffe0a0, 0.16));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * Tank-front desk. A pale desk with a built-in slimline aquarium across its front: lit blue water with a sandy base, a few
   * strands of green weed, a little pink coral and two small fish drifting back and forth; bubbles rise in one corner.
   */
  aquariumdesk: {
    front({ x, y, s, tint }) {
      const frame = 0xd8dce2, frameDark = 0x9aa0a8, water = 0x14466e, waterLight = 0x1e5a86, sand = 0xd8c48a, weed = 0x3a9a5a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(0xe8ecf0, 1)); fill(x, y + 1, 44, 3, tint(frame, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 4, 44, 1, frameDark); // the tank top rail
      fill(x + 1, y + 5, 42, 10, water); fill(x + 1, y + 5, 42, 1, waterLight); fill(x + 1, y + 13, 42, 2, sand); fill(x, y + 5, 1, 10, frame); fill(x + 43, y + 5, 1, 10, frameDark);
      // weed and coral
      for (const wx of [x + 6, x + 8, x + 36]) { for (let k = 0; k < 5; k++) dot(wx + Math.round(Math.sin(now * 1.5 + wx + k) * 1), y + 13 - k, weed); }
      fill(x + 28, y + 11, 3, 3, 0xf08a9a); dot(x + 27, y + 10, 0xf6a0b0); dot(x + 31, y + 11, 0xf6a0b0);
      // two drifting fish
      for (const [i, c] of [[0, 0xf0824a], [1, 0xf2c040]]) { const fx = x + 10 + Math.round((Math.sin(now * 0.8 + i * 2) * 0.5 + 0.5) * 24), fy = y + 8 + i * 3; fill(fx, fy, 2, 1, c); dot(fx - 1, fy, c); dot(fx + 2, fy, c); }
      // rising bubbles
      const bub = (y + 14) - Math.floor(now * 4) % 9; dot(x + 38, bub, 0xbfe4f0, 0.8); dot(x + 40, bub - 2, 0xbfe4f0, 0.6);
      fill(x, y + 15, 44, 1, 0x3a4048);
      if (isNight) glowLate(() => glowAt(x + 22, y + 9, 20, 6, 0x2a7ac0, 0.14));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** The ringmaster's steamer trunk for a desk: dark wood bound with brass bands and studs, a red lid label, and a gold-tipped baton resting across the corner. */
  circusdesk: {
    front({ x, y, s, tint }) {
      const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, brass = 0xe8b84a, red = 0xd8402a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodLight, 1)); fill(x, y + 1, 44, 14, tint(wood, 0.95)); fill(x, y + 15, 44, 1, woodDark);
      fill(x, y + 1, 1, 14, woodLight); fill(x + 43, y + 1, 1, 14, woodDark);
      for (const bx of [x + 2, x + 40]) { fill(bx, y + 1, 2, 14, brass); fill(bx, y + 1, 1, 14, 0xf2d888); }
      fill(x, y + 6, 44, 2, brass); fill(x, y + 6, 44, 1, 0xf2d888);
      for (let k = x + 7; k < x + 40; k += 6) dot(k, y + 7, woodDark);
      fill(x + 14, y + 10, 16, 3, red); fill(x + 14, y + 10, 16, 1, 0xf05a40);
      fill(x + 20, y - 2, 10, 1, 0x2a2a2e); dot(x + 20, y - 2, brass); dot(x + 29, y - 2, 0xf2ead8);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A curator's dark-wood specimen table: brass-pulled drawers, an open field ledger and a little brass microscope on the corner. */
  museumdesk: {
    front({ x, y, s, tint }) {
      const wood = 0x5a3b25, woodL = 0x7a5436, woodD = 0x3a2819, brass = 0xc8a44e, paper = 0xe8e2d4;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodL, 1)); fill(x, y + 1, 44, 14, tint(wood, 0.95)); fill(x, y + 15, 44, 1, woodD); fill(x, y + 5, 44, 1, woodD);
      fill(x + 4, y + 9, 3, 1, brass); fill(x + 20, y + 9, 3, 1, brass); fill(x + 36, y + 9, 3, 1, brass);
      fill(x + 2, y - 2, 8, 3, paper); fill(x + 6, y - 2, 1, 3, woodD); fill(x + 3, y - 1, 2, 1, 0x8a8276); fill(x + 7, y - 1, 2, 1, 0x8a8276);
      fill(x + 32, y - 1, 6, 1, brass); fill(x + 34, y - 7, 2, 6, brass); fill(x + 33, y - 8, 4, 2, brass); fill(x + 35, y - 10, 1, 3, brass); dot(x + 35, y - 10, 0xe8c878);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A toymaker's workbench: sawn wood with a little vice, a hammer and saw lying out, and a half-built wooden toy horse on the corner. */
  northpolebench: {
    front({ x, y, s, tint }) {
      const wood = 0x8a5a32, woodL = 0xa8703e, woodD = 0x5a3b25, steel = 0x8a8e96, red = 0xc62828, toy = 0xc89a5a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodL, 1)); fill(x, y + 1, 44, 14, tint(wood, 0.95)); fill(x, y + 15, 44, 1, woodD); fill(x, y + 4, 44, 1, woodD);
      fill(x + 1, y - 2, 4, 2, steel); fill(x + 2, y - 1, 2, 1, 0x6a6e78);
      fill(x + 8, y - 1, 6, 1, woodD); fill(x + 13, y - 2, 2, 2, steel);
      fill(x + 18, y - 1, 7, 1, steel); fill(x + 18, y - 1, 1, 1, red);
      fill(x + 32, y - 4, 6, 3, toy); fill(x + 32, y - 1, 1, 1, toy); fill(x + 34, y - 1, 1, 1, toy); fill(x + 37, y - 1, 1, 1, toy); fill(x + 37, y - 6, 2, 2, toy); dot(x + 38, y - 5, PALETTE.eye);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A dark editing desk: an audio waveform glowing along the front, a little striped clapperboard on one corner and a to-go coffee on the other. */
  soundstagedesk: {
    front({ x, y, s, tint }) {
      const wood = 0x2a2a30, woodL = 0x44444c, woodD = 0x18181c, black = 0x1a1a1e, white = 0xe8e4d8, red = 0xd83030;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodL, 1)); fill(x, y + 1, 44, 14, tint(wood, 0.95)); fill(x, y + 15, 44, 1, woodD);
      for (let k = 0; k < 40; k++) { const h2 = 1 + (hash(x + k, 3) % 3); fill(x + 2 + k, y + 11 - h2, 1, h2 * 2, k % 7 < 4 ? 0x3a6a8a : woodD); }
      fill(x + 1, y - 4, 8, 4, black); fill(x + 1, y - 5, 8, 1, white); for (let k = 0; k < 4; k++) fill(x + 1 + k * 2, y - 5, 1, 1, k % 2 ? black : white); dot(x + 3, y - 2, white); dot(x + 6, y - 2, white);
      fill(x + 36, y - 4, 4, 4, white); fill(x + 36, y - 5, 4, 1, 0x8a2a1e); fill(x + 37, y - 3, 2, 1, red);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A pale-wood offering desk: a lit candle on one corner, a small bowl of fruit on the other, and a sun-stamped pamphlet laid in the middle. */
  cultdesk: {
    front({ x, y, s, tint }) {
      const wood = 0xcbb894, woodL = 0xe0d0ae, woodD = 0xa8926e, white = 0xeceae2, flame = 0xffd36b, bowl = 0xc08a5e, gold = 0xe0b24a;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodL, 1)); fill(x, y + 1, 44, 14, tint(wood, 0.95)); fill(x, y + 15, 44, 1, woodD); fill(x, y + 7, 44, 1, woodD, 0.5);
      fill(x + 3, y - 1, 2, 1, 0xd8d0bc); fill(x + 3, y - 4, 2, 3, white); const f = candlelight(x * 0.3); fill(x + 3, y - 6, 2, 2, flame, f); dot(x + 3, y - 7, 0xfff0b0, f);
      fill(x + 34, y - 2, 7, 2, bowl); fill(x + 34, y - 2, 7, 1, 0xd8a878); dot(x + 36, y - 3, 0xd06a4a); dot(x + 38, y - 3, 0xe0b24a); dot(x + 37, y - 4, 0x8a9a5a);
      fill(x + 18, y - 2, 6, 2, white); disc(x + 21, y - 1, 1, gold);
      if (isNight) glowLate(() => glowAt(x + 4, y - 5, 3, 4, flame, 0.14));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A gaming rig: a dark desk with an RGB edge-light under the lip, a headset on a stand and a mouse on a glowing pad. */
  esportsrig: {
    front({ x, y, s, tint }) {
      const desk = 0x1a1a22, deskL = 0x2a2a36, deskD = 0x101016, rgb = (k) => [0x2ad8e8, 0x8a3ae8, 0xe82ad8, 0xf03a8a, 0x9ae82a, 0x3a6aff][((k % 6) + 6) % 6];
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(deskL, 1)); fill(x, y + 1, 44, 14, tint(desk, 0.95)); fill(x, y + 15, 44, 1, deskD);
      const base = Math.floor(now * 2); for (let k = 0; k < 44; k++) dot(x + k, y + 15, rgb(base + k), 0.5);
      fill(x + 3, y - 7, 1, 7, 0x3a3a46); fill(x + 1, y - 9, 5, 1, 0x2a2a36); fill(x + 1, y - 8, 1, 3, 0x2a2a36); fill(x + 5, y - 8, 1, 3, 0x2a2a36); dot(x + 1, y - 6, rgb(base));
      fill(x + 35, y - 2, 8, 2, 0x14141c); fill(x + 35, y - 2, 8, 1, rgb(base + 3), 0.6); fill(x + 38, y - 3, 2, 2, 0x2a2a36);
      if (isNight) glowLate(() => { glowAt(x + 22, y + 15, 22, 2, rgb(base), 0.12); glowAt(x + 3, y - 7, 5, 4, rgb(base), 0.1); });
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A heavy stone-slab desk in stacked rough stone, a carved tablet on one corner and a bone stylus and flint chisel on the other. */
  cavemanslab: {
    front({ x, y, s, tint }) {
      const stone = 0x6e6056, stoneL = 0x8a7c6e, stoneD = 0x4a4038, tablet = 0x5a4e44, glyph = 0xa0472a, bone = 0xe0d6b8;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(stoneL, 1)); fill(x, y + 1, 44, 14, tint(stone, 0.95)); fill(x, y + 15, 44, 1, stoneD);
      fill(x, y + 5, 44, 1, stoneD, 0.5); fill(x, y + 10, 44, 1, stoneD, 0.5); dot(x + 3, y, stoneD); dot(x + 20, y, stoneD); dot(x + 40, y, stoneL);
      fill(x + 2, y - 3, 7, 3, tablet); fill(x + 2, y - 3, 7, 1, 0x6a5e54); dot(x + 4, y - 2, glyph); dot(x + 6, y - 2, glyph); dot(x + 5, y - 1, glyph);
      fill(x + 34, y - 1, 6, 1, bone); dot(x + 34, y - 1, 0xf0e8cc); fill(x + 36, y - 3, 3, 2, 0x8a8e96); dot(x + 37, y - 3, 0xb8bcc4);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A dark carved-wood secretary's desk with a green leather inlay, a lit candle, a wax-sealed letter and a little gavel. */
  lodgedesk: {
    front({ x, y, s, tint }) {
      const wood = 0x3a2418, woodL = 0x5a3a24, woodD = 0x24160e, baize = 0x2a4a3a, candle = 0xe8e0cc, flame = 0xffd36b, wax = 0x8a1e1e, brass = 0xc8a44e;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(woodL, 1)); fill(x, y + 1, 44, 14, tint(wood, 0.95)); fill(x, y + 15, 44, 1, woodD);
      fill(x + 8, y + 3, 28, 6, tint(baize, 1)); fill(x + 8, y + 3, 28, 1, 0x3a5a4a);
      fill(x + 4, y + 11, 3, 1, brass); fill(x + 37, y + 11, 3, 1, brass);
      fill(x + 2, y - 1, 2, 1, brass); fill(x + 2, y - 5, 2, 4, candle); const f = candlelight(x * 0.3); fill(x + 2, y - 7, 2, 2, flame, f); dot(x + 2, y - 8, 0xfff0b0, f);
      fill(x + 18, y - 2, 8, 2, 0xe0d2b0); fill(x + 18, y - 2, 8, 1, 0xeedcb0); dot(x + 22, y - 2, wax);
      fill(x + 36, y - 2, 4, 2, woodL); fill(x + 38, y - 4, 1, 3, 0x5a3a24);
      if (isNight) glowLate(() => glowAt(x + 3, y - 6, 3, 4, flame, 0.14));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A steel workbench with a bolted top, a ham radio with a lit green meter and dials, a clipboard checklist and a wrench. */
  prepperdesk: {
    front({ x, y, s, tint }) {
      const steel = 0x5a5e66, steelL = 0x7a7e86, steelD = 0x3a3e46, radio = 0x3a4038, dial = 0xd8a82a, green = 0x3aa05a, paper = 0xe8e4d0;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(steelL, 1)); fill(x, y + 1, 44, 14, tint(steel, 0.95)); fill(x, y + 15, 44, 1, steelD);
      for (let k = x + 3; k < x + 42; k += 8) dot(k, y + 1, steelD); fill(x, y + 8, 44, 1, steelD, 0.5);
      fill(x + 1, y - 5, 9, 5, radio); fill(x + 1, y - 5, 9, 1, 0x4e5448); fill(x + 2, y - 4, 4, 2, 0x0a1a0e); fill(x + 2, y - 4, 3, 1, green);
      dot(x + 7, y - 4, dial); dot(x + 9, y - 4, dial); dot(x + 8, y - 2, dial); fill(x + 1, y - 6, 1, 2, steelL);
      fill(x + 20, y - 3, 6, 3, paper); fill(x + 20, y - 3, 6, 1, 0xf2eee0); fill(x + 21, y - 2, 3, 1, 0x6a665c, 0.6); dot(x + 24, y - 2, green);
      fill(x + 34, y - 1, 7, 1, steelL); fill(x + 34, y - 2, 2, 2, steel); fill(x + 40, y - 2, 1, 2, steel);
      if (isNight) glowLate(() => glowAt(x + 4, y - 3, 3, 2, green, 0.1));
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** Resolute desk. A carved dark-walnut presidential pedestal desk with a gilt-trimmed top, carved roundel panels either side of a recessed kneehole, and a small furled flag on a gold stand at the left end. */
  paradiseresolute: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const wood = 0x5a3b25, woodLight = 0x7a5232, woodDark = 0x3a2616, gold = 0xc8a050, goldLight = 0xeccf84, cloth = 0x9a2a1e;
      fill(x, y, 44, 1, tint(goldLight, 1)); fill(x, y + 1, 44, 4, tint(wood, 0.9)); fill(x, y + 1, 44, 1, tint(woodLight, 0.95));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(wood, 0.72)); fill(x, y + 5, 44, 1, tint(woodDark, 0.8));
      fill(x + 15, y + 6, 14, 10, tint(woodDark, 0.85));
      for (const px of [x + 2, x + 31]) {
        fill(px, y + 7, 11, 8, tint(woodLight, 0.7)); fill(px, y + 7, 11, 1, tint(gold, 0.9)); fill(px, y + 14, 11, 1, tint(woodDark, 0.8));
        ring(px + 5, y + 10, 2.4, gold, 0.8); dot(px + 5, y + 10, goldLight);
      }
      fill(x + 1, y - 9, 1, 9, gold); dot(x + 1, y - 10, goldLight);
      fill(x + 2, y - 9, 5, 5, cloth); fill(x + 2, y - 8, 5, 1, 0xe8e3da); fill(x + 2, y - 6, 5, 1, 0xe8e3da);
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** Bridge console. A sweeping grey console desk with a curved LCARS control face — amber, blue and red blocks and a row of buttons that chase along it. */
  startrekbridge: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const base = 0x3a4458, baseLight = 0x5a6478, baseDark = 0x242a38, amber = 0xd4a02a, blue = 0x4a8ec8, red = 0xc0566a;
      fill(x, y, 44, 1, tint(baseLight, 1)); fill(x, y + 1, 44, 4, tint(base, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(base, 0.72)); fill(x, y + 5, 44, 1, tint(baseDark, 0.8));
      fill(x + 2, y + 6, 8, 3, amber, 0.9); fill(x + 2, y + 6, 8, 1, shaded(amber, 1.2));
      fill(x + 11, y + 6, 6, 3, blue, 0.85); fill(x + 34, y + 6, 8, 3, red, 0.85);
      for (let k = 0; k < 10; k++) { const c = [amber, blue, 0x6abf8a, red][(k + Math.floor(now * 2)) % 4]; dot(x + 4 + k * 3, y + 11, k % 3 === 0 ? c : baseDark, 0.8); }
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** Partners' desk. A leather-topped mahogany partners' desk: a green leather inlay with a gold-tooled edge, a carved front and two drawers with brass handles. */
  gentlemenpartners: {
    front({ x, y, s, tint }) {
      drawMonitor(x + 6, y - 10);
      const wood = 0x4a2a1a, woodLight = 0x6a4028, woodDark = 0x2e1a10, leather = 0x2a4a2a, leatherLight = 0x3a6a3a, gold = 0xc8a050;
      fill(x, y, 44, 1, tint(gold, 1));
      fill(x, y + 1, 44, 4, tint(wood, 0.9)); fill(x, y + 1, 44, 1, tint(woodLight, 0.95));
      fill(x + 2, y + 2, 40, 2, tint(leather, 0.85)); fill(x + 2, y + 2, 40, 1, tint(leatherLight, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 11, tint(wood, 0.72)); fill(x, y + 5, 44, 1, tint(woodDark, 0.8));
      for (const dx of [x + 4, x + 28]) { fill(dx, y + 7, 12, 7, tint(woodLight, 0.7)); fill(dx, y + 7, 12, 1, tint(0x8a5a32, 0.8)); fill(dx + 5, y + 10, 2, 1, gold); }
      fill(x + 1, y + 16, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /** A black slab desk: a thick graphite top with a pale bevel, a near-black front with a flush pull, and a white light line under the top that glows at night. */
  grokdesk: {
    front({ x, y, s, tint }) {
      const top = 0x2a2c33, topLight = 0x5a5e68, front = 0x1a1b20, frontDark = 0x111216, light = 0xf4f1ea;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(topLight, 1)); fill(x, y + 1, 44, 4, tint(top, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, 44, 1, light, isNight ? 0.5 : 0.7);
      fill(x + 1, y + 6, 42, 10, front); fill(x + 1, y + 15, 42, 1, frontDark); fill(x + 1, y + 6, 1, 10, 0x2a2c33);
      fill(x + 33, y + 9, 7, 1, 0x3a3d47); // a flush pull
      fill(x + 1, y + 16, 42, 1, SHADOW);
      if (isNight) glowLate(() => { fill(x, y + 5, 44, 1, light, 0.85); glowAt(x + 22, y + 6, 22, 2, light, 0.08); });
      drawKeyboard(s, y + 2);
    },
  },
  /** A bot's console pod: a low black console with rounded ends, vent slats along its front and a strip of ion-blue lights that run while its bot works. */
  grokbotpod: {
    front({ x, y, s, tint, occupant }) {
      const top = 0x2a2c33, topLight = 0x4a4d57, body = 0x1a1b20, bodyDark = 0x0e0f12, slat = 0x24262c, ion = 0x8ab4ff;
      drawMonitor(x + 6, y - 10);
      fill(x + 1, y, 42, 1, tint(topLight, 1)); fill(x, y + 1, 44, 4, tint(top, 1)); fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x + 1, y + 5, 42, 10, body); fill(x + 2, y + 15, 40, 1, bodyDark); fill(x + 1, y + 5, 1, 10, 0x2a2c33); fill(x + 42, y + 5, 1, 10, bodyDark);
      for (let k = x + 4; k < x + 26; k += 3) fill(k, y + 8, 2, 5, slat);
      const working = occupant && occupant.session && occupant.session.state === 'thinking', run = Math.floor(now * 6) % 6;
      const strip = () => { for (let k = 0; k < 6; k++) dot(x + 30 + k * 2, y + 10, ion, working ? (k === run ? 1 : 0.35) : k === 0 ? 0.8 : 0.15); };
      strip();
      if (isNight) glowLate(strip);
      fill(x + 2, y + 16, 40, 1, SHADOW);
      drawKeyboard(s, y + 2);
    },
  },
  /**
   * A geode desk: a slab of dark stone edged in brass, its front a polished geode slice, agate bands in
   * grey, white and lavender round a hollow of violet crystal that glitters, on two rough rock feet; by night its heart glows.
   */
  geodedesk: {
    front({ x, y, s, tint }) {
      const brass = 0xc8a050, brassLight = 0xf0d080, cx = x + 22, cy = y + 10;
      drawMonitor(x + 6, y - 10);
      fill(x, y, 44, 1, tint(brassLight, 1)); fill(x, y + 1, 44, 3, tint(0x2c2632, 1)); fill(x, y + 4, 44, 1, brass);
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      // The slice: its rind, then band inside band to the crystal hollow
      fill(x, y + 5, 44, 10, 0x342824); fill(x, y + 5, 44, 1, 0x4a3c34);
      for (const [rx, ry, c] of [[21, 4.6, 0x5e5868], [19, 4.1, 0x9a92ae], [15, 3.3, 0x6e5e90], [11, 2.5, 0x8a80a6], [7, 1.7, 0x1e1036]]) {
        const halfAt = (r) => Math.round(rx * Math.sqrt(Math.max(0, 1 - (r / (ry + 0.5)) ** 2)));
        for (let r = -Math.floor(ry), n; r <= Math.floor(ry); r += n) {
          const half = halfAt(r);
          n = 1; while (r + n <= Math.floor(ry) && halfAt(r + n) === half) n++;
          fill(cx - half, cy + r, half * 2 + 1, n, c);
        }
      }
      for (const [dx, dy, c] of [[-4, 0, 0x9a78d8], [-1, -1, 0xc8a8f8], [2, 0, 0x7a52b8], [4, 1, 0x9a78d8], [0, 1, 0x6a44a8], [-3, 1, 0xb090e8]]) dot(cx + dx, cy + dy, c);
      if ((now + x * 0.21) % 9 < 0.5) gdSparkle(cx - 1, cy - 1, ((now + x * 0.21) % 9) / 0.5);
      fill(x + 1, y + 15, 5, 2, 0x3a2e2a); fill(x + 38, y + 15, 5, 2, 0x3a2e2a); dot(x + 1, y + 15, 0x5a4a40); dot(x + 38, y + 15, 0x5a4a40);
      fill(x + 1, y + 17, 42, 1, SHADOW);
      drawKeyboard(s, y + 2);
      if (isNight) glowLate(() => gdGlow(cx, cy, 8, 2, 0xa070f0, 0.16));
    },
  },
};
