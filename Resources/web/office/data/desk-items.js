/** What else can sit at the end of a desk whose corner is at `x`, `y`, where the plant, papers or lamp go. */
const DESK_ITEMS = {
  /** Space helmet. */
  helmet(x, y) {
    fill(x + 36, y - 5, 6, 5, 0xe8e8ec); fill(x + 37, y - 6, 4, 1, 0xe8e8ec); fill(x + 37, y - 4, 4, 2, PALETTE.brass); dot(x + 37, y - 4, WHITE, 0.7); fill(x + 36, y, 6, 1, 0x8a93a3); dot(x + 41, y - 2, 0xd4140b);
  },
  /** Model rocket. */
  rocket(x, y) {
    fill(x + 37, y, 5, 1, 0x3a3f4a); fill(x + 39, y - 7, 2, 6, 0xe8e8ec); fill(x + 39, y - 8, 2, 1, 0xd4140b); dot(x + 39, y - 5, 0x4ad4e8); fill(x + 38, y - 2, 1, 2, 0xd4140b); fill(x + 41, y - 2, 1, 2, 0xd4140b);
  },
  /** Plant under glass. */
  terrarium(x, y) {
    fill(x + 35, y, 8, 1, 0x6e4a2e); fill(x + 37, y - 3, 1, 3, PALETTE.leafDark); fill(x + 38, y - 4, 2, 2, PALETTE.leaf); dot(x + 39, y - 2, PALETTE.leaf);
    fill(x + 36, y - 6, 6, 6, 0xcfeee0, 0.18); fill(x + 37, y - 7, 4, 1, 0xcfeee0, 0.7); fill(x + 36, y - 6, 1, 6, 0xcfeee0, 0.6); fill(x + 41, y - 6, 1, 6, 0xcfeee0, 0.6); dot(x + 37, y - 5, WHITE, 0.6);
  },
  /** Quill and inkpot. */
  quill(x, y) {
    fill(x + 35, y - 2, 4, 3, 0x1d1f25); fill(x + 36, y - 3, 2, 1, 0x3a3d47); line(x + 37, y - 3, x + 40, y - 8, 0xf4f1ea); fill(x + 40, y - 9, 2, 3, 0xf4f1ea); dot(x + 41, y - 10, 0xf4f1ea); dot(x + 39, y - 6, 0xc8c4ba);
  },
  /** Candelabra. */
  candelabra(x, y) {
    const gold = PALETTE.brass;
    fill(x + 36, y - 9, 7, 6, 0xffd36b, 0.1);
    fill(x + 37, y, 5, 1, gold); fill(x + 39, y - 5, 1, 5, gold); fill(x + 36, y - 5, 7, 1, gold); dot(x + 36, y - 6, gold); dot(x + 42, y - 6, gold);
    for (const [cx, top] of [[36, -8], [39, -9], [42, -8]]) { fill(x + cx, y + top + 1, 1, -top - 7 + 1, 0xf4f1ea); dot(x + cx, y + top, Math.floor(now * 6 + cx) % 2 ? 0xffd36b : 0xff8a3d); }
  },
  /** Jewelled goblet. */
  goblet(x, y) {
    const gold = PALETTE.brass; fill(x + 37, y - 6, 4, 3, gold); fill(x + 37, y - 6, 4, 1, 0x6a1e2a); dot(x + 38, y - 4, 0xd4140b); fill(x + 38, y - 3, 2, 2, 0x9a7a3a); fill(x + 37, y - 1, 4, 1, gold);
  },
  /** Bottle of rum. */
  rum(x, y) {
    fill(x + 38, y - 5, 3, 5, 0x3b6a3a); fill(x + 39, y - 8, 1, 3, 0x3b6a3a); dot(x + 39, y - 9, 0xc9a06a); fill(x + 38, y - 3, 3, 2, 0xe0c890); dot(x + 38, y - 5, WHITE, 0.4);
  },
  /** Brass compass. */
  compass(x, y) {
    fill(x + 36, y - 7, 5, 3, 0x9a7a3a); fill(x + 37, y - 6, 3, 1, 0xf4f1ea);
    fill(x + 36, y - 3, 5, 3, PALETTE.brass); fill(x + 37, y - 4, 3, 1, PALETTE.brass); fill(x + 37, y - 3, 3, 2, 0xf4f1ea);
    dot(x + 38 + Math.round(Math.sin(now * 2.5) * 0.7), y - 3, 0xd4140b);
  },
  /** Skull candle. */
  skull(x, y) {
    fill(x + 36, y - 4, 5, 4, 0xe8e4d6); dot(x + 37, y - 3, 0x1d1f25); dot(x + 39, y - 3, 0x1d1f25); dot(x + 37, y - 1, 0x8a846f); dot(x + 39, y - 1, 0x8a846f);
    fill(x + 38, y - 7, 2, 3, 0xf4f1ea); dot(x + 37, y - 5, 0xf4f1ea); dot(x + 40, y - 4, 0xf4f1ea);
    dot(x + 38 + (Math.floor(now * 5) % 2), y - 8, 0xffd36b); dot(x + 38, y - 9, 0xff8a3d, 0.6); fill(x + 36, y - 10, 5, 4, 0xffd36b, 0.08);
  },
  /** A mug of cocoa, marshmallows on top and a candy cane in it; steam curls up from it now and then. */
  cocoa(x, y) {
    const mug = 0xc8302a, red = 0xd42a2a, white = 0xf6f2ea;
    fill(x + 37, y - 4, 4, 4, mug); fill(x + 37, y - 2, 4, 1, white); dot(x + 41, y - 3, mug); dot(x + 41, y - 2, mug);
    fill(x + 37, y - 4, 4, 1, 0x6a3a22); dot(x + 37, y - 5, white); dot(x + 38, y - 4, white);
    dot(x + 40, y - 5, red); dot(x + 40, y - 6, white); dot(x + 40, y - 7, red); dot(x + 39, y - 8, white); dot(x + 38, y - 7, red);
    const t = (now + x * 0.07) % 9;
    if (t < 3) {
      for (let k = 0; k < 2; k++) {
        const rise = (t / 3 + k * 0.5) % 1;
        dot(x + 38 + Math.round(Math.sin(rise * 6 + k * 2)), y - 6 - Math.round(rise * 5), WHITE, 0.5 * (1 - rise));
      }
    }
  },
  /** A round berry teapot under a cream knitted tea cosy with a pine band and bobble; steam from its spout now and then. */
  teapot(x, y) {
    const cream = 0xf2ead8, shade = 0xd0c4a8, pine = 0x2f6a4a, pot = 0xb8303a;
    fill(x + 35, y - 1, 9, 1, 0x8a2430);
    fill(x + 38, y - 8, 3, 1, cream); fill(x + 37, y - 7, 5, 1, cream); fill(x + 36, y - 6, 7, 5, cream);
    for (const dx of [38, 40]) fill(x + dx, y - 6, 1, 5, shade);
    fill(x + 42, y - 6, 1, 5, shade); fill(x + 36, y - 4, 7, 1, pine);
    fill(x + 38, y - 10, 3, 2, pine); dot(x + 38, y - 10, 0x4a8a64);
    for (const [dx, dy] of [[35, -2], [34, -3], [33, -4], [32, -5], [43, -4], [44, -3], [44, -2], [43, -2]]) dot(x + dx, y + dy, pot);
    const t = (now + x * 0.09) % 10;
    if (t < 3) {
      for (let k = 0; k < 2; k++) {
        const rise = (t / 3 + k * 0.5) % 1;
        dot(x + 32 + Math.round(Math.sin(rise * 6 + k * 2)), y - 6 - Math.round(rise * 5), WHITE, 0.5 * (1 - rise));
      }
    }
  },
  /** Snow globe. */
  snowglobe(x, y) {
    fill(x + 36, y - 1, 7, 2, 0x8c6744); disc(x + 39, y - 5, 3.2, 0xcfe3ef, 0.35); ring(x + 39, y - 5, 3, 0xcfe3ef, 0.7);
    fill(x + 38, y - 5, 3, 3, 0x2f5a3a); dot(x + 39, y - 6, 0x2f5a3a);
    for (let k = 0; k < 3; k++) { const f = (now * 0.6 + k / 3) % 1; dot(x + 37 + k * 2, y - 7 + Math.round(f * 5), WHITE); }
    dot(x + 37, y - 7, WHITE, 0.6);
  },
  /** Thermos and cup. */
  thermos(x, y) {
    fill(x + 37, y - 6, 4, 6, 0xc9443a); fill(x + 37, y - 8, 4, 2, 0x8a8f9c); fill(x + 37, y - 3, 4, 1, 0x9a2a2a);
    fill(x + 41, y - 2, 2, 2, 0x8a8f9c); const rise = (now * 0.6) % 1; dot(x + 41, y - 3 - Math.round(rise * 4), WHITE, 0.6 * (1 - rise));
  },
  /** Penguin figure. */
  penguin(x, y) {
    fill(x + 37, y - 6, 4, 6, 0x1d1f25); fill(x + 38, y - 4, 2, 4, 0xf4f1ea); dot(x + 38, y - 5, 0xe8823a); dot(x + 37, y - 5, WHITE); fill(x + 37, y, 4, 1, 0xe8823a);
  },
  /** Potions. */
  potions(x, y) {
    fill(x + 36, y - 4, 4, 4, 0x5ad46a, 0.85); fill(x + 37, y - 6, 2, 2, 0xcfe3ef, 0.6); dot(x + 37, y - 7, 0xc9a06a);
    const rise = (now * 0.9) % 1; dot(x + 37 + (Math.floor(now * 3) % 2), y - 1 - Math.round(rise * 3), 0xb8f0c0, 1 - rise);
    fill(x + 41, y - 6, 2, 6, 0x9a6ad0, 0.85); dot(x + 41, y - 7, 0xcfe3ef, 0.6); dot(x + 41, y - 8, 0xc9a06a);
  },
  /** Dragon egg. */
  egg(x, y) {
    const t = now % 6, wob = t < 0.6 ? Math.round(Math.sin(t * 30)) : 0;
    fill(x + 35, y, 8, 1, 0xc9a06a); dot(x + 35, y - 1, 0xc9a06a); dot(x + 42, y - 1, 0xc9a06a);
    fill(x + 37 + wob, y - 6, 4, 1, 0x3f8a6a); fill(x + 36 + wob, y - 5, 6, 5, 0x3f8a6a); fill(x + 41 + wob, y - 5, 1, 5, 0x2f6a52);
    dot(x + 37 + wob, y - 4, PALETTE.brass); dot(x + 39 + wob, y - 2, PALETTE.brass); dot(x + 40 + wob, y - 5, PALETTE.brass); dot(x + 37 + wob, y - 5, WHITE, 0.4);
  },
  /** Spellbook. */
  spellbook(x, y) {
    fill(x + 34, y - 1, 9, 2, 0x5a2a6a); fill(x + 35, y - 2, 7, 2, PALETTE.paper); fill(x + 38, y - 2, 1, 2, 0xc8c4ba);
    const rise = (now * 0.5) % 1; dot(x + 38, y - 4 - Math.round(rise * 4), 0xc05aff, 1 - rise); dot(x + 37 + (Math.floor(now * 2) % 3), y - 3, 0xc05aff, 0.5);
  },
  /** Game controller. */
  controller(x, y) {
    fill(x + 36, y - 2, 7, 2, 0x3a3d47); fill(x + 36, y, 2, 1, 0x3a3d47); fill(x + 41, y, 2, 1, 0x3a3d47);
    dot(x + 37, y - 2, 0x6a7080); dot(x + 41, y - 2, 0xd4140b); dot(x + 42, y - 1, 0x4c8bf5); dot(x + 40, y - 1, 0x6fd36f);
  },
  /** Energy drinks. */
  energy(x, y) {
    fill(x + 37, y - 5, 3, 5, 0x2a2d35); fill(x + 37, y - 6, 3, 1, 0x9aa0a6); fill(x + 37, y - 3, 3, 1, 0x6fd36f); dot(x + 38, y - 4, 0x6fd36f);
    fill(x + 40, y - 1, 3, 1, 0x2a2d35); dot(x + 41, y - 1, 0x6fd36f); dot(x + 40, y - 2, 0x2a2d35);
  },
  /** Headset on a stand, in grey with a light rim so it shows on a dark wall, its cups lit round the colours. */
  headset(x, y) {
    fill(x + 39, y - 7, 1, 7, 0x4a4e56); fill(x + 37, y, 5, 1, 0x4a4e56); fill(x + 37, y - 8, 5, 1, 0x8a93a3);
    for (const cx of [x + 36, x + 41]) {
      fill(cx, y - 7, 2, 3, 0x5a5f6c); fill(cx, y - 7, 2, 1, 0x8a93a3);
      dot(cx + (cx > x + 38 ? 1 : 0), y - 6, rainbow((now * 0.3) % 1));
    }
  },
  /** Board and soldering iron. */
  circuit(x, y) {
    fill(x + 34, y - 1, 6, 2, 0x2f7a4a); dot(x + 35, y - 1, 0x1d1f25); dot(x + 37, y - 1, PALETTE.brass); dot(x + 38, y, 0x1d1f25);
    fill(x + 41, y - 1, 2, 1, 0x8a8f9c); line(x + 41, y - 2, x + 43, y - 6, 0xc9443a); dot(x + 40, y - 1, 0xc8cdd6);
    const rise = (now * 0.4) % 1; dot(x + 40 + Math.round(Math.sin(rise * 6)), y - 3 - Math.round(rise * 5), PALETTE.smoke, 0.5 * (1 - rise));
  },
  /** Pizza box. */
  pizza(x, y) {
    fill(x + 34, y - 7, 9, 5, 0xb8905a); dot(x + 38, y - 5, 0xc9443a);
    fill(x + 34, y - 2, 9, 3, 0xc9a06a); fill(x + 35, y - 2, 7, 2, 0xe8b04a); dot(x + 36, y - 2, 0xc9443a); dot(x + 39, y - 1, 0xc9443a); fill(x + 40, y - 2, 2, 2, 0xc9a06a);
  },
  /** Coffee machine. */
  coffeepot(x, y) {
    fill(x + 38, y - 8, 5, 8, 0x2a2a2e); fill(x + 38, y - 8, 5, 1, 0x3a3a40); dot(x + 42, y - 6, 0xd4140b);
    fill(x + 36, y - 3, 4, 3, 0xcfe3ef, 0.45); fill(x + 36, y - 2, 4, 2, 0x5a3a24);
    const rise = (now * 0.6) % 1; dot(x + 37, y - 4 - Math.round(rise * 4), WHITE, 0.5 * (1 - rise));
  },
  /** Dumbbell. */
  dumbbell(x, y) {
    fill(x + 36, y - 2, 2, 3, 0x2a2c30); fill(x + 38, y - 1, 3, 1, 0x8a8f9c); fill(x + 41, y - 2, 2, 3, 0x2a2c30); dot(x + 36, y - 2, 0x4a4e56);
  },
  /** Protein shaker. */
  shaker(x, y) {
    fill(x + 37, y - 6, 4, 6, 0xe8e4da, 0.85); fill(x + 37, y - 4, 4, 4, 0xc9a06a); fill(x + 37, y - 8, 4, 2, 0x1d1f25); dot(x + 39, y - 9, 0x1d1f25); dot(x + 37, y - 6, WHITE, 0.6);
  },
  /** Kettlebell. */
  kettlebell(x, y) {
    fill(x + 37, y - 6, 4, 1, 0x1d1f25); dot(x + 37, y - 5, 0x1d1f25); dot(x + 40, y - 5, 0x1d1f25); fill(x + 36, y - 4, 6, 4, 0xe0703a); dot(x + 37, y - 3, 0xf08a50);
  },
  /** Coconut drink. */
  coconut(x, y) {
    fill(x + 36, y - 4, 5, 4, 0x6e4a2e); fill(x + 36, y - 4, 5, 1, 0xe8e4da); line(x + 39, y - 4, x + 41, y - 8, 0xe07aa0);
    fill(x + 36, y - 8, 5, 1, 0xf2d35b); fill(x + 37, y - 9, 3, 1, 0xf2d35b); fill(x + 38, y - 7, 1, 3, 0x8a6a42);
  },
  /** Pineapple. */
  pineapple(x, y) {
    fill(x + 37, y - 5, 4, 5, 0xe8c04a); for (const [dx, dy] of [[0, 4], [2, 4], [1, 3], [3, 3], [0, 2], [2, 2], [1, 1], [3, 1]]) dot(x + 37 + dx, y - dy, 0xb8902a);
    fill(x + 38, y - 8, 2, 3, PALETTE.leaf); dot(x + 37, y - 7, PALETTE.leafDark); dot(x + 40, y - 7, PALETTE.leafDark); dot(x + 39, y - 9, PALETTE.leaf);
  },
  /** Tiki mug. */
  tikimug(x, y) {
    fill(x + 37, y - 6, 4, 6, 0x8a5a2b); dot(x + 37, y - 4, 0x1d1f25); dot(x + 40, y - 4, 0x1d1f25); fill(x + 38, y - 2, 2, 1, 0x1d1f25); dot(x + 41, y - 4, 0x8a5a2b); dot(x + 41, y - 3, 0x8a5a2b);
    fill(x + 38, y - 8, 1, 2, 0xe07aa0); dot(x + 39, y - 7, PALETTE.leaf);
  },
  /** Enamel camp mug. */
  enamel(x, y) {
    fill(x + 37, y - 3, 4, 3, 0x3f6fc4); fill(x + 37, y - 3, 4, 1, 0x1d1f25); dot(x + 38, y - 1, WHITE); dot(x + 40, y - 2, WHITE); fill(x + 41, y - 2, 1, 2, 0x3f6fc4);
    const rise = (now * 0.6) % 1; dot(x + 38, y - 4 - Math.round(rise * 4), WHITE, 0.6 * (1 - rise));
  },
  /** Camp lantern. */
  lantern(x, y) {
    const flick = 0.75 + 0.25 * Math.sin(now * 11) * Math.sin(now * 5.3);
    fill(x + 35, y - 9, 9, 9, 0xffd36b, 0.1 * flick);
    fill(x + 37, y - 1, 5, 1, 0x2f5a3a); fill(x + 37, y - 5, 5, 4, 0xffd36b, flick); fill(x + 37, y - 5, 1, 4, 0x2f5a3a); fill(x + 41, y - 5, 1, 4, 0x2f5a3a);
    fill(x + 37, y - 6, 5, 1, 0x2f5a3a); dot(x + 38, y - 8, 0x2f5a3a); dot(x + 40, y - 8, 0x2f5a3a); dot(x + 39, y - 9, 0x2f5a3a);
  },
  /** Pinecone. */
  pinecone(x, y) {
    fill(x + 37, y - 4, 4, 4, 0x8a5a2b); fill(x + 38, y - 5, 2, 1, 0x8a5a2b); for (const [dx, dy] of [[0, 1], [2, 1], [1, 2], [3, 2], [0, 3], [2, 3]]) dot(x + 37 + dx, y - dy, 0xa87048);
  },
  /** Jack-o'-lantern, a different face from desk to desk. */
  jackolantern(x, y) {
    drawPumpkin(x + 39, y, false, Math.floor(x / 66 + y / 44) % 4, candlelight(x + y));
  },
  /** Three candles, dripping, their flames wavering. */
  candles(x, y) {
    glowAt(x + 39, y - 6, 6, 5, 0xffd36b, 0.08 * candlelight(x));
    fill(x + 35, y, 9, 1, 0x5a5f6c);
    [[35, 5], [38, 7], [41, 4]].forEach(([dx, h], k) => {
      fill(x + dx, y - h, 2, h, 0xf6efdc);
      fill(x + dx + 1, y - h, 1, h, 0xd6cbad);
      dot(x + dx, y - h + 2, 0xd6cbad);
      dot(x + dx + 1, y - h - 1, 0x2a2a2e);
      dot(x + dx + 1, y - h - 2, 0xffd36b, candlelight(x + k));
      dot(x + dx + 1, y - h - 3, 0xff8a3d, 0.5 * candlelight(x + k));
    });
  },
  /** A bowl of candy corn. */
  candy(x, y) {
    for (const [dx, dy] of [[36, -3], [38, -4], [40, -3], [42, -3], [39, -3]]) {
      dot(x + dx, y + dy, 0xf2d35b);
      dot(x + dx, y + dy - 1, 0xe8823a);
      dot(x + dx, y + dy - 2, 0xf4f1ea);
    }
    fill(x + 35, y - 2, 9, 2, 0x3a2a4a);
    fill(x + 35, y - 2, 9, 1, 0x5a4a6a);
    fill(x + 36, y, 7, 1, 0x2a1e36);
  },
  /** A jack-o'-lantern basketball on a little stand. */
  pumpkinball(x, y) {
    fill(x + 37, y, 5, 1, 0x5a5f6c);
    drawBall(x + 39, y - 4, 3, true, candlelight(x));
  },
  /** A basketball on a little stand. */
  basketball(x, y) {
    fill(x + 37, y, 5, 1, 0x5a5f6c);
    drawBall(x + 39, y - 4, 3);
  },
  /** A whistle on its lanyard, the lanyard in the move's blue. */
  whistle(x, y) {
    for (const [dx, dy] of [[35, 0], [34, -1], [34, -2], [35, -3], [36, -4], [38, -4], [39, -3]]) dot(x + dx, y + dy, 0x3558ce);
    fill(x + 37, y - 2, 4, 2, 0xc8cdd6); fill(x + 41, y - 2, 2, 1, 0xc8cdd6);
    dot(x + 37, y - 2, WHITE); dot(x + 38, y - 1, 0x8a8f9c);
  },
  /** A stopwatch, its hand going round. */
  stopwatch(x, y) {
    disc(x + 39, y - 4, 3, 0x9aa0a6); disc(x + 39, y - 4, 2, 0xf4f1ea);
    fill(x + 38, y - 9, 3, 1, 0x9aa0a6); dot(x + 39, y - 8, 0x9aa0a6);
    const a = ((now % 60) / 60) * Math.PI * 2;
    dot(x + 39 + Math.round(Math.sin(a) * 2), y - 4 - Math.round(Math.cos(a) * 2), 0xd4140b);
    dot(x + 39, y - 4, 0x1a1a1a);
  },
  /** A high-top sneaker, white with a stripe in the move's blue. */
  sneaker(x, y) {
    fill(x + 36, y - 1, 7, 1, 0x1a1a1a); fill(x + 36, y - 3, 7, 2, 0xf5f5f5); fill(x + 37, y - 6, 3, 3, 0xf5f5f5);
    fill(x + 37, y - 2, 4, 1, 0x3558ce); dot(x + 38, y - 5, 0x8a8a8a); dot(x + 38, y - 4, 0x8a8a8a); dot(x + 42, y - 2, 0xd8d8d8);
  },
  /** A foam finger in the move's blue, a white 1 on it. */
  foamfinger(x, y) {
    fill(x + 37, y - 4, 5, 4, 0x3558ce); fill(x + 39, y - 9, 2, 5, 0x3558ce); dot(x + 36, y - 3, 0x3558ce);
    fill(x + 41, y - 4, 1, 4, 0x2a45a8); fill(x + 40, y - 9, 1, 5, 0x2a45a8);
    fill(x + 38, y - 3, 1, 2, WHITE);
  },
  /** A championship cup, a blue ribbon round it. */
  cup(x, y) {
    const gold = PALETTE.brass;
    fill(x + 37, y, 5, 1, 0x3a2616); fill(x + 39, y - 2, 1, 2, gold); fill(x + 38, y - 3, 3, 1, gold);
    fill(x + 37, y - 7, 5, 4, gold); fill(x + 36, y - 8, 7, 1, 0xf2d38a); fill(x + 41, y - 7, 1, 4, darker(gold));
    dot(x + 35, y - 7, gold); dot(x + 35, y - 6, gold); dot(x + 43, y - 7, gold); dot(x + 43, y - 6, gold);
    fill(x + 37, y - 5, 5, 1, 0x3558ce);
  },
  /** A clipboard with a play on it. */
  /** Red roses in a glass vase; now and then a petal drops and lies a while on the desk. */
  roses(x, y) {
    const red = 0xd4203a, deep = 0x9a1428, stem = 0x3f7a3a;
    for (const [dx, h] of [[37, 7], [39, 8], [41, 6]]) fill(x + dx, y - h, 1, h - 3, stem);
    dot(x + 36, y - 6, 0x4f9a4a); dot(x + 42, y - 5, 0x4f9a4a);
    for (const [dx, dy] of [[36, -10], [39, -11], [41, -9]]) {
      fill(x + dx, y + dy, 2, 2, red); dot(x + dx + 1, y + dy + 1, deep); dot(x + dx, y + dy, 0xf05a6a);
    }
    fill(x + 37, y - 4, 5, 4, 0xbfe0ee, 0.45); fill(x + 37, y - 4, 1, 4, WHITE, 0.5); fill(x + 37, y - 1, 5, 1, 0x9ac8dc, 0.6);
    const t = (now + x * 0.27) % 11;
    if (t < 1.4) dot(x + 42 + Math.round(Math.sin(t * 5)), y - 8 + Math.round((t / 1.4) * 8), red);
    else if (t < 7) dot(x + 43, y, red);
  },
  /** A heart-shaped box of chocolates; now and then its lid lifts to show them, and settles back. */
  chocolates(x, y) {
    const t = (now + x * 0.31) % 15, lift = t < 2.4 ? Math.round(Math.sin((t / 2.4) * Math.PI) * 4) : 0;
    fill(x + 35, y - 2, 9, 2, 0x9a1a2c); fill(x + 35, y - 1, 9, 1, 0x7a1424);
    if (lift) {
      for (const dx of [36, 38, 40, 42]) { dot(x + dx, y - 3, 0x5a3020); dot(x + dx, y - 4, 0x7a4a30); }
      dot(x + 39, y - 4, 0xf2d38a);
    }
    heart(x + 35, y - 8 - lift, 9, 7, 0xd8344a);
    for (const [dx, dy] of heartShape(9, 7).edge) if (dy > 2) dot(x + 39 + dx, y - 8 - lift + dy, 0xa82234);
    dot(x + 37, y - 7 - lift, 0xf2a0b0);
    fill(x + 38, y - 5 - lift, 3, 1, PALETTE.brass);
  },
  clipboard(x, y) {
    fill(x + 36, y - 8, 7, 8, 0x8a6040); fill(x + 38, y - 9, 3, 1, 0x9aa0a6); fill(x + 37, y - 7, 5, 6, 0xf4f5f7);
    dot(x + 38, y - 3, 0x15161a); dot(x + 40, y - 5, 0x15161a); dot(x + 39, y - 4, 0x3558ce); dot(x + 40, y - 6, 0x3558ce);
  },
  /** Nachos. A tray of tortilla chips, cheese over them, and a pot of salsa dip beside. */
  nachos(x, y) {
    fill(x + 34, y - 1, 10, 1, 0x5a5f6c); fill(x + 35, y - 2, 8, 1, 0x8a8f9c);
    for (const [dx, dy, c] of [[35, -3, 0xf2c94c], [37, -4, 0xe8b23a], [36, -5, 0xf2c94c], [38, -3, 0xf2c94c], [39, -4, 0xd9a42a]]) {
      dot(x + dx, y + dy, c);
    }
    fill(x + 36, y - 3, 3, 1, 0xffb81f); dot(x + 37, y - 4, 0xffd04a); dot(x + 38, y - 5, 0xf2c94c);
    fill(x + 40, y - 4, 4, 2, 0xeae6dc); fill(x + 40, y - 4, 4, 1, 0xc9443a); dot(x + 41, y - 4, 0xe86a4a);
  },
  /** Latte. A wide cup on its saucer, a heart poured in the foam, and a wisp of steam now and then. */
  latte(x, y) {
    fill(x + 35, y - 1, 8, 1, 0xf4efe6); fill(x + 36, y, 6, 1, 0xd8d0c0);
    fill(x + 36, y - 4, 6, 3, 0xf4efe6); fill(x + 37, y - 2, 4, 1, 0xe2dacb);
    fill(x + 42, y - 4, 1, 2, 0xf4efe6);
    fill(x + 36, y - 5, 6, 1, 0xb07a50); fill(x + 37, y - 6, 4, 1, 0xc8956a); dot(x + 36, y - 5, 0xf4efe6); dot(x + 41, y - 5, 0xf4efe6);
    dot(x + 38, y - 6, 0xfaf4ea); dot(x + 40, y - 6, 0xfaf4ea); dot(x + 39, y - 5, 0xfaf4ea);
    const t = (now + x * 0.13) % 8;
    if (t < 2.4) { const rise = t / 2.4; dot(x + 39 + Math.round(Math.sin(rise * 5)), y - 8 - Math.round(rise * 4), WHITE, 0.55 * (1 - rise)); }
  },
  /** Champagne. A bottle on ice in a little silver bucket, a flute beside it; now and then a bubble rises in the flute. */
  champagne(x, y) {
    fill(x + 35, y - 10, 3, 6, 0x1f4a2e); fill(x + 35, y - 10, 1, 6, 0x3a6a48);
    fill(x + 36, y - 12, 1, 2, 0xe8b84a); fill(x + 35, y - 11, 3, 1, 0xe8b84a); dot(x + 36, y - 13, 0xc8a050);
    fill(x + 33, y - 5, 7, 1, 0xe4e8ee); fill(x + 33, y - 4, 7, 4, 0xb8bec8); fill(x + 33, y - 4, 1, 4, 0xd8dce4);
    fill(x + 39, y - 4, 1, 4, 0x8a919c); fill(x + 34, y - 2, 5, 1, 0x9aa0aa);
    dot(x + 34, y - 6, 0xeaf6ff); dot(x + 38, y - 6, 0xeaf6ff, 0.8);
    fill(x + 41, y - 8, 3, 4, 0xf2d88a, 0.85); fill(x + 41, y - 8, 3, 1, 0xfff4cc);
    dot(x + 40, y - 8, WHITE, 0.4); dot(x + 44, y - 8, WHITE, 0.4);
    dot(x + 42, y - 4, 0xdfe6ee); dot(x + 42, y - 3, 0xdfe6ee); dot(x + 42, y - 2, 0xdfe6ee); fill(x + 41, y - 1, 3, 1, 0xdfe6ee);
    const t = (now + x * 0.17) % 7;
    if (t < 1.4) dot(x + 42, y - 5 - Math.round((t / 1.4) * 2), WHITE, 0.9);
  },
  /** Noisemaker. A party horn, its paper curl rolled up, lying on a little heap of confetti. */
  noisemaker(x, y) {
    for (const [dx, dy, k] of [[34, -1, 0], [35, -1, 4], [36, -1, 2], [37, -1, 5], [38, -1, 1], [39, -1, 3], [40, -1, 0], [41, -1, 4],
      [35, -2, 1], [36, -2, 3], [38, -2, 4], [39, -2, 2], [36, -3, 5], [33, -1, 2], [43, -1, 5]]) {
      dot(x + dx, y + dy, CONFETTI_COLOURS[k]);
    }
    dot(x + 36, y - 4, 0xf4f1ea);
    for (let k = 0; k < 5; k++) dot(x + 37 + k, y - 4, k % 2 ? 0xe0508a : 0xf2d35b);
    fill(x + 42, y - 6, 1, 5, 0xf2d35b); fill(x + 43, y - 7, 1, 7, 0xe0508a); dot(x + 42, y - 4, 0xe0508a);
    ring(x + 38, y - 6, 1.5, 0x4c8bf5); dot(x + 38, y - 6, 0x2a5aa8);
    if ((now + x * 0.21) % 13 < 0.5) dot(x + 36, y - 3, WHITE);
  },
  /** A zero-g mug. A squeeze mug with its lid and straw, floating above the desk and bobbing; now and then a drop of coffee floats off. */
  floatmug(x, y) {
    const b = Math.round(Math.sin(now * 1.3 + x * 0.37) * 1.2), top = y - 10 + b;
    fill(x + 36, y, 7, 1, BLACK, 0.12 + 0.05 * b);
    fill(x + 36, top + 2, 6, 5, 0xf2f2ee); fill(x + 36, top + 2, 1, 5, WHITE); fill(x + 41, top + 2, 1, 5, 0xc8ccd2);
    fill(x + 36, top + 4, 6, 1, 0x4c8bf5); fill(x + 36, top + 7, 6, 1, 0xc8ccd2);
    fill(x + 35, top + 1, 8, 1, 0x8a919c); fill(x + 36, top, 6, 1, 0xa8aeb8);
    fill(x + 40, top - 2, 1, 2, 0xd4703a); dot(x + 41, top - 3, 0xd4703a);
    fill(x + 42, top + 3, 2, 1, 0xe2e5ea); fill(x + 43, top + 4, 1, 2, 0xe2e5ea); dot(x + 42, top + 6, 0xe2e5ea);
    const t = (now + x * 0.11) % 9;
    if (t < 3.5) {
      const k = t / 3.5, dx = 42 + Math.round(k * 3), dy = top - 3 - Math.round(k * 4 + Math.sin(k * 7) * 0.8);
      fill(x + dx, dy, 2, 2, 0x6a4028, 1 - k * 0.6); dot(x + dx, dy, 0xb07a50, 1 - k * 0.6);
    }
  },
  /** A floating plant. A sprig in a clear sphere, tethered to a puck on the desk, bobbing slowly; its leaves stir now and then. */
  floatplant(x, y) {
    const b = Math.round(Math.sin(now * 0.9 + x * 0.29 + 1.5) * 1.2), cx = x + 39, cy = y - 10 + b;
    fill(x + 37, y - 1, 5, 1, 0x5a6270); fill(x + 38, y - 2, 3, 1, 0x8a93a3); dot(cx, y - 2, 0x6fd36f);
    for (let ty = y - 3; ty > cy + 4; ty--) dot(cx + ((ty + Math.floor(now)) % 4 === 0 ? 1 : 0), ty, 0xd8dce2, 0.8);
    disc(cx, cy, 4.2, 0xcfeee0, 0.16);
    fill(cx - 2, cy + 1, 5, 2, 0x6e4a2e); fill(cx - 1, cy + 3, 3, 1, 0x5a3b25);
    const stir = (now + x * 0.17) % 12 < 1 ? Math.round(Math.sin(now * 9)) : 0;
    fill(cx, cy - 2, 1, 3, PALETTE.leafDark);
    dot(cx - 1 + stir, cy - 2, PALETTE.leaf); dot(cx - 2 + stir, cy - 3, PALETTE.leaf); dot(cx + 1 - stir, cy - 1, PALETTE.leaf);
    dot(cx + 2 - stir, cy - 2, PALETTE.leaf); dot(cx, cy - 3, 0xf28fb0);
    ring(cx, cy, 4.4, 0xdff4ea, 0.7); dot(cx - 2, cy - 3, WHITE, 0.85); dot(cx - 3, cy - 2, WHITE, 0.5);
  },
  /** A lava lamp: magenta liquid in a tapered glass on a steel base, two orange blobs drifting slowly up and down; it glows at night. */
  lavalamp(x, y) {
    const steel = 0x6e727c, steelLight = 0xa8adb6, liquid = 0xc84a8a, blob = 0xffa040;
    fill(x + 37, y - 3, 5, 3, steel); fill(x + 38, y - 3, 1, 3, steelLight); fill(x + 36, y - 1, 7, 1, 0x4a4e58);
    fill(x + 38, y - 12, 3, 1, steel); dot(x + 38, y - 12, steelLight);
    fill(x + 38, y - 11, 3, 4, liquid); fill(x + 37, y - 7, 5, 4, liquid);
    // The blobs: one rising as the other sinks, slowly
    const s = Math.sin(now * 0.35 + x * 0.13), up = Math.round(((s + 1) / 2) * 6), down = 6 - up;
    fill(x + 38, y - 5 - up, 2, 2, blob); dot(x + 39, y - 6 - up, 0xffc070);
    fill(x + 39, y - 5 - down, 2, 1, blob); dot(x + 39, y - 4 - down, blob, 0.7);
    fill(x + 37, y - 4, 5, 1, blob, 0.8); // the pool at the bottom
    dot(x + 38, y - 10, WHITE, 0.5); dot(x + 37, y - 6, WHITE, 0.4);
    if (isNight) glowAt(x + 39, y - 7, 5, 5, 0xff70b0, 0.14);
  },
  /** An hourglass in a turned-wood frame: its sand runs down over a minute, a thin stream through the neck; then it turns itself over. */
  hourglass(x, y) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, sand = 0xe8c070, sandDark = 0xc89a48, glass = 0xdff0f6, cx = x + 39;
    const run = 60, at = (now + x * 0.37) % run, p = Math.min(1, at / (run - 4));
    fill(x + 36, y - 1, 7, 1, wood); fill(x + 36, y - 12, 7, 1, wood); fill(x + 36, y - 12, 7, 1, woodLight, 0.5);
    const rows = [[-11, 2], [-10, 2], [-9, 1], [-8, 1], [-7, 0], [-6, 1], [-5, 1], [-4, 2], [-3, 2]];
    for (const [dy, half] of rows) fill(cx - half, y + dy, half * 2 + 1, 1, glass, 0.45);
    // The sand left above, settled at the bottom of the top bulb; and the heap below, growing
    const above = Math.round((1 - p) * 4), below = Math.round(p * 4);
    for (let k = 0; k < above; k++) { const [dy, half] = rows[3 - k]; fill(cx - half, y + dy, half * 2 + 1, 1, k === above - 1 ? sand : sandDark); }
    for (let k = 0; k < below; k++) { const [dy, half] = rows[8 - k]; fill(cx - half, y + dy, half * 2 + 1, 1, k === below - 1 ? sand : sandDark); }
    if (p < 1) { dot(cx, y - 7, sand); for (let dy = -6; dy < -2 - below; dy++) if ((dy + Math.floor(now * 8)) % 2) dot(cx, y + dy, sand, 0.8); }
    // The frame's posts, and a glint off the glass
    for (const px of [x + 36, x + 42]) fill(px, y - 11, 1, 10, wood);
    dot(cx - 1, y - 10, WHITE, 0.7);
  },
  /** A drinking bird: a glass bird in a top hat on a wire stand, rocking further and further until it dips its beak in a glass of water, every 10 s. */
  drinkingbird(x, y) {
    const red = 0xd8402a, redLight = 0xf07060, glass = 0xdff0f6, wire = 0x6e727c, hat = 0x2a3a6a, px = x + 38, py = y - 7;
    // The glass of water in front, and the stand's base and legs
    fill(x + 42, y - 5, 4, 5, glass, 0.4); fill(x + 42, y - 3, 4, 3, 0x6ab0e0, 0.6); dot(x + 42, y - 5, WHITE, 0.6);
    fill(x + 33, y - 1, 8, 1, 0x3c404a);
    for (let s = 1; s <= 6; s++) { dot(px - Math.round(s / 2), py + s, wire); dot(px + Math.round(s / 3), py + s, wire, 0.7); }
    // Its tilt: rocking a little more each swing, then the dip, then back up with a wobble (each desk on its own clock)
    const t = (now + x * 0.23) % 10;
    let a = t < 7 ? (t / 7) ** 2 * 0.9 + Math.sin(t * 3) * 0.15 : t < 8 ? 1.7 : 1.7 * (1 - (t - 8) / 2) + Math.sin((t - 8) * 6) * 0.2 * (10 - t);
    a = Math.round(a / 0.2) * 0.2;
    const ux = Math.sin(a), uy = -Math.cos(a), at = (r) => [Math.round(px + ux * r), Math.round(py + uy * r)];
    // The tube, pale glass with the red rising up from the bulb; the bulb; the head with its beak and hat
    for (let r = -3; r <= 3; r++) { const [tx, ty] = at(r); dot(tx, ty, r < 0 ? red : glass, r < 0 ? 1 : 0.7); }
    const [bx, by] = at(-4); fill(bx - 1, by - 1, 3, 3, red); dot(bx - 1, by - 1, redLight);
    const [hx, hy] = at(4); fill(hx - 1, hy - 1, 2, 2, red); dot(hx - 1, hy - 1, redLight);
    dot(a < 1.3 ? hx + 1 : hx, a < 1.3 ? hy : hy + 1, 0xf2b030); // the beak, pointing forward
    const [cx2, cy2] = at(6);
    dot(Math.round(hx - 1 + ux * 1.5), Math.round(hy - 1 + uy * 1.5), hat); dot(cx2 - 1, cy2 - 1, hat); dot(Math.round(hx + ux * 1.5), Math.round(hy - 1 + uy * 1.5), hat);
    dot(px, py, 0x9aa0aa); // the pivot
  },
  /** A gumball machine: a glass globe of bright gumballs on a red stand with a brass coin knob; now and then one rolls out. */
  gumball(x, y) {
    const red = 0xc8302a, redLight = 0xe04a3e, redDark = 0x8a2020, cx = x + 39, cy = y - 8;
    fill(x + 37, y - 4, 5, 4, red); fill(x + 37, y - 4, 1, 4, redLight); fill(x + 41, y - 4, 1, 4, redDark);
    fill(x + 36, y - 1, 7, 1, redDark); dot(cx, y - 3, PALETTE.brass); fill(x + 38, y - 2, 3, 1, 0x3a1a1a);
    disc(cx, cy, 3.4, 0xe8f4f8, 0.45);
    const balls = [[-2, 1, 0xf2d35b], [0, 2, 0x4c8bf5], [2, 1, 0x6fd36f], [-1, 0, 0xf28fb0], [1, 0, 0xe8782e], [-2, -1, 0x4c8bf5],
      [0, -1, 0xd8402a], [2, -1, 0xf6f2ea], [-1, 2, 0xd8402a], [1, 2, 0xf28fb0]];
    for (const [bx, by, col] of balls) dot(cx + bx, cy + by, col);
    ring(cx, cy, 3.6, 0xdff4fa, 0.75); dot(cx - 2, cy - 2, WHITE, 0.9);
    fill(cx - 1, cy - 5, 3, 1, red); dot(cx, cy - 6, redDark);
    // Now and then a gumball drops out of the chute and rolls a little way
    const t = (now + x * 0.21) % 23;
    if (t < 2.5) {
      const k = Math.min(1, t / 0.6), roll = t < 0.6 ? 0 : Math.round((t - 0.6) * 1.6);
      dot(cx + roll, y - 2 + Math.round(k), [0xf2d35b, 0x4c8bf5, 0xf28fb0, 0x6fd36f][Math.floor((now + x * 0.21) / 23) % 4]);
    }
  },
  /** A bat-wing foam finger, its wings flapping now and then. */
  batfinger(x, y) {
    const foam = 0x2a2433, flap = (now % 4) < 0.6 && Math.floor(now * 6) % 2 ? -1 : 0;
    fill(x + 38, y - 4, 4, 4, foam);
    fill(x + 39, y - 8, 2, 4, foam);
    dot(x + 37, y - 3, foam);
    dot(x + 39, y - 2, 0x9a6ad0);
    dot(x + 40, y - 2, 0x9a6ad0);
    for (const [wx, dir] of [[x + 37, -1], [x + 42, 1]]) {
      fill(wx + (dir < 0 ? -2 : 0), y - 6 + flap, 3, 1, 0x3e3650);
      dot(wx + dir * 2, y - 5 + flap, 0x3e3650);
      dot(wx, y - 5, 0x3e3650);
    }
  },
  /** A fan of red envelopes, a gold mark on each; one catches the light now and then. */
  redenvelopes(x, y) {
    const red = 0xc8302e, deep = 0x7a1a1e, light = 0xe85a44, gold = 0xe8b84a;
    // Two leaning out either side, one standing in front of them, each edged so they read apart
    for (const ex of [34, 41]) {
      fill(x + ex, y - 6, 4, 6, deep); fill(x + ex, y - 5, 4, 5, red); fill(x + ex, y - 5, 4, 1, light); dot(x + ex + (ex < 40 ? 1 : 2), y - 3, gold);
    }
    fill(x + 37, y - 9, 6, 9, deep); fill(x + 38, y - 8, 4, 8, red); fill(x + 38, y - 8, 4, 1, light); fill(x + 39, y - 6, 2, 2, gold);
    fill(x + 35, y, 10, 1, SHADOW);
    const t = (now + x * 0.17) % 8;
    if (t < 0.6) {
      const gx = x + [36, 39, 42][Math.floor(x / 44) % 3], gy = y - 5 - (gx === x + 39 ? 2 : 0);
      dot(gx, gy - 1, WHITE); dot(gx - 1, gy, 0xfff0b0, 0.7); dot(gx + 1, gy, 0xfff0b0, 0.7); dot(gx, gy + 1, 0xfff0b0, 0.5);
    }
  },
  /** A bowl of mandarins, leaves on the top one. */
  tangerines(x, y) {
    const orange = 0xf08a24, shade = 0xc8661a, light = 0xffb860, leaf = 0x3a8a3a;
    for (const [dx, dy] of [[36, -5], [40, -5], [38, -7]]) {
      fill(x + dx, y + dy, 3, 3, orange); dot(x + dx + 2, y + dy + 2, shade); dot(x + dx, y + dy, light);
    }
    dot(x + 39, y - 8, 0x5a3a24); dot(x + 40, y - 9, leaf); dot(x + 41, y - 9, leaf); dot(x + 38, y - 9, 0x2a6a2e);
    fill(x + 35, y - 2, 9, 1, 0xf0f2f4); fill(x + 35, y - 1, 9, 1, 0xc8d4e0); fill(x + 36, y, 7, 1, 0x9aa8b8);
    for (const dx of [36, 39, 42]) dot(x + dx, y - 2, 0x3f6fc4);
  },
  /** A mint-green typewriter with a sheet in its roller: the carriage creeps left a step at a time as a line types out, then the bell rings (a tiny "!") and it slides back; each desk keeps its own rhythm. */
  typewriter(x, y) {
    const body = 0x6aa898, bodyLight = 0x8cc8b6, bodyDark = 0x467a6c, roller = 0x2a2c34, steel = 0x6e727c, steelLight = 0xa8adb6;
    const key = 0xf0ead8, ink = 0x4a4a52, x0 = x + 33;
    // The cycle: typing for 9 s, the bell, then the carriage returns
    const t = (now + x * 0.37) % 16;
    const typed = t < 9 ? Math.floor((t / 9) * 4) : 0, off = t >= 9.5 && t < 10 ? -1 : -typed;
    // The sheet standing up out of the roller: a finished line, and the one being typed growing from the print point
    fill(x0 + 3 + off, y - 12, 6, 4, PALETTE.paper); fill(x0 + 8 + off, y - 12, 1, 4, 0xc8c4ba); dot(x0 + 3 + off, y - 12, WHITE, 0.7);
    fill(x0 + 4 + off, y - 11, 4, 1, ink, 0.75);
    fill(x0 + 4 + off, y - 9, 1 + typed, 1, ink, 0.75);
    // The carriage: a dark roller with a steel knob at each end and the return lever
    fill(x0 + 1 + off, y - 8, 11, 2, roller); fill(x0 + 1 + off, y - 8, 11, 1, 0x4a4c56);
    fill(x0 + off, y - 8, 1, 2, steel); dot(x0 + off, y - 8, steelLight); fill(x0 + 12 + off, y - 8, 1, 2, steel); dot(x0 + 12 + off, y - 8, steelLight);
    dot(x0 + 1 + off, y - 9, steelLight); dot(x0 + off, y - 10, steelLight);
    // The body, lit from the top left, with two rows of round keys and the spacebar
    fill(x0 + 1, y - 6, 11, 5, body); fill(x0, y - 4, 13, 3, body);
    fill(x0 + 1, y - 6, 11, 1, bodyLight); fill(x0 + 1, y - 6, 1, 2, bodyLight); fill(x0, y - 4, 1, 3, bodyLight);
    fill(x0 + 11, y - 6, 1, 2, bodyDark); fill(x0 + 12, y - 4, 1, 3, bodyDark);
    fill(x0 + 2, y - 5, 9, 3, bodyDark);
    for (const kx of [2, 4, 6, 8, 10]) dot(x0 + kx, y - 5, key);
    for (const kx of [3, 5, 7, 9]) dot(x0 + kx, y - 4, key);
    fill(x0 + 4, y - 3, 5, 1, key);
    const down = t < 9 ? Math.floor(now * 5 + x) % 5 : -1;   // one key pressed at a time while it types
    if (down >= 0) dot(x0 + 2 + down * 2, y - 5, 0x9a9484);
    // Its base on the desk
    fill(x0, y - 1, 13, 1, bodyDark);
    dot(x0 + 2, y - 6, WHITE, 0.5);
    // The bell's ring: a tiny "!"
    if (t >= 9 && t < 9.6) { fill(x0 + 13, y - 13, 1, 2, 0xf2d35b); dot(x0 + 13, y - 10, 0xf2d35b); }
  },

  /** An ant farm in a wooden frame: tunnels and a nest chamber in the sand between two panes of glass; three ants trickle along the tunnels a step at a time, and a new tunnel inches deeper over the hours. */
  antfarm(x, y) {
    const sand = 0xd4b078, sandLight = 0xe6c994, sandDark = 0xb08c58, tunnel = 0x7a5636, ant = 0x2a1a14;
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, glass = 0xdff0f6;
    const ox = x + 35, oy = y - 9;                                   // the sand's top-left
    // The frame: a base, a lid and two side posts
    fill(x + 34, y - 2, 11, 2, wood); fill(x + 34, y - 2, 11, 1, woodLight); fill(x + 44, y - 2, 1, 2, woodDark);
    fill(x + 34, y - 12, 11, 1, wood); fill(x + 34, y - 12, 11, 1, woodLight, 0.5);
    fill(x + 34, y - 11, 1, 9, wood); fill(x + 44, y - 11, 1, 9, woodDark);
    // The sand, with a little air above it and two mounds by the hole
    fill(ox, y - 11, 9, 2, glass, 0.35);
    fill(ox, oy, 9, 7, sand); fill(ox, oy, 9, 1, sandLight); fill(ox, oy + 6, 9, 1, sandDark);
    dot(ox + 1, oy - 1, sandLight); dot(ox + 3, oy - 1, sandLight);
    // Tunnels: a shaft down from the hole, two galleries, a nest with an egg; one more branch creeps deeper by the hour
    const main = [[2, 0], [2, 1], [2, 2], [3, 2], [4, 2], [5, 2], [6, 2], [6, 3], [6, 4], [5, 4], [4, 4], [3, 4]];
    const extra = [[2, 4], [1, 4], [1, 5]];
    const grown = Math.floor(Date.now() / 3600000) % (extra.length + 1);
    const path = main.concat(extra.slice(0, grown));
    for (const [dx, dy] of path) dot(ox + dx, oy + dy, tunnel);
    fill(ox + 6, oy + 4, 2, 2, tunnel); dot(ox + 7, oy + 5, 0xf4ecd8);   // the nest
    for (const [dx, dy] of [[0, 2], [8, 3], [4, 5], [0, 6]]) dot(ox + dx, oy + dy, sandDark, 0.7); // pebbles
    // Three ants, each a step along the tunnels about once a second
    for (let k = 0; k < 3; k++) {
      const i = Math.floor(now * 0.9 + k * 4 + x * 0.37) % path.length;
      dot(ox + path[i][0], oy + path[i][1], ant);
    }
    // The glass front: a pale edge and a glint
    fill(ox, y - 11, 9, 1, glass, 0.6);
    dot(ox + 1, y - 10, WHITE, 0.8); dot(ox + 8, oy + 1, WHITE, 0.35);
  },
  /** A plasma globe: a dark glass sphere on a steel base, violet filaments from a bright centre that shift slowly between four positions; at night it glows through the dark. */
  plasmaball(x, y) {
    const steel = 0x6e727c, steelLight = 0xa8adb6, steelDark = 0x4a4e58, glass = 0x1c1230, violet = 0xb070ff, violetLight = 0xe8d0ff;
    const cx = x + 39, cy = y - 8;
    // The base
    fill(cx - 3, y - 3, 7, 3, steel); fill(cx - 3, y - 3, 1, 3, steelLight); fill(cx + 3, y - 3, 1, 3, steelDark); fill(cx - 2, y - 4, 5, 1, steelDark);
    // The dark glass sphere
    disc(cx, cy, 4.6, glass); ring(cx, cy, 4.6, 0xdff0f6, 0.3);
    // The filaments: four shapes, changing every couple of seconds (a different one per desk)
    const shape = Math.floor((now + x * 0.37) / 2.2) % 4;
    const arms = [
      [[-2, -2], [-3, -1], [3, -3], [2, 1], [0, -4]],
      [[-3, -3], [-2, 1], [3, -1], [1, -4], [-1, 3]],
      [[-3, 0], [-1, -4], [2, -3], [3, 1], [1, 3]],
      [[-2, -3], [-3, 2], [0, 4], [3, -2], [2, 3]]][shape];
    for (const [ax, ay] of arms) {
      line(cx, cy, cx + ax, cy + ay, violet, 0.75);
      dot(cx + ax, cy + ay, violetLight, 0.6);
    }
    disc(cx, cy, 1.2, violetLight);
    dot(cx - 3, cy - 3, WHITE, 0.7); dot(cx - 2, cy - 4, WHITE, 0.4);
    if (isNight) glowLate(() => { glowAt(cx, cy, 8, 8, 0xa060ff, 0.2); dot(cx, cy, 0xf4e8ff); });
    else glowAt(cx, cy, 5, 5, violet, 0.05);
  },
  /** A chrome two-slot toaster, lever down and its slots glowing faintly orange; every 40 s two slices pop up with a little hop, then it sinks them again. */
  toaster(x, y) {
    const chrome = 0xc4c9d2, chromeLight = 0xf2f5fa, chromeDark = 0x7a808c, slot = 0x2a2a30, ember = 0xe8782e;
    const bread = 0xe8c48a, crust = 0xa86a30;
    const t = (now + x * 0.29) % 40, up = t < 4, hop = up && t < 0.35 ? 1 : 0;
    // The slices, behind the body: up for 4 s with a hop as they pop
    if (up) for (const sx of [x + 36, x + 40]) {
      const top = y - 12 - hop;
      fill(sx, top + 1, 3, 3, bread); fill(sx + 1, top, 1, 1, crust); dot(sx, top + 1, crust); dot(sx + 2, top + 1, crust);
      fill(sx, top + 2, 1, 2, crust, 0.7); fill(sx + 2, top + 2, 1, 2, crust, 0.7); dot(sx + 1, top + 2, 0xf4dcae);
    }
    // The body: rounded chrome with a lit left edge and a dark base
    fill(x + 34, y - 7, 11, 6, chrome); fill(x + 35, y - 8, 9, 1, chrome);
    fill(x + 34, y - 7, 1, 6, chromeLight); fill(x + 35, y - 8, 9, 1, chromeLight, 0.6); fill(x + 44, y - 7, 1, 6, chromeDark);
    fill(x + 34, y - 2, 11, 1, chromeDark); fill(x + 35, y - 1, 2, 1, slot); fill(x + 42, y - 1, 2, 1, slot);
    dot(x + 36, y - 6, WHITE, 0.8); fill(x + 37, y - 4, 5, 1, chromeDark, 0.35);
    // The two slots on top, glowing while the bread toasts
    for (const sx of [x + 36, x + 40]) { fill(sx, y - 8, 3, 1, slot); if (!up) fill(sx, y - 8, 3, 1, ember, 0.55); }
    // The lever on its side: down while toasting, up when it pops; a dial below it
    fill(x + 45, up ? y - 6 : y - 3, 1, 1, 0x3a3a40); fill(x + 45, up ? y - 7 : y - 4, 1, 1, 0x4a4e58);
    dot(x + 42, y - 4, 0x3a3a40);
    if (!up && isNight) glowLate(() => { glowAt(x + 39, y - 8, 4, 2, 0xff9040, 0.18); fill(x + 36, y - 8, 3, 1, 0xffa050, 0.7); fill(x + 40, y - 8, 3, 1, 0xffa050, 0.7); });
  },
  /** A Venus flytrap in a small clay pot: every 25 s a fly buzzes round it and lands, the trap snaps shut, stays shut a while and slowly opens again. */
  flytrap(x, y) {
    const pot = PALETTE.pot, potLight = 0xc07a52, potDark = 0x7a4630, soil = 0x4a3222;
    const leaf = PALETTE.leaf, leafDark = PALETTE.leafDark, leafLight = 0x7ac070, inside = 0xd8504a, insideLight = 0xf08070;
    const cx = x + 40, ty = y - 9;
    // The pot, its rim and the soil
    fill(x + 37, y - 4, 6, 4, pot); fill(x + 37, y - 4, 1, 4, potLight); fill(x + 42, y - 4, 1, 4, potDark);
    fill(x + 36, y - 5, 8, 1, potLight); fill(x + 37, y - 5, 6, 1, soil);
    // The rosette: two low leaves, a small trap open beside, and the stem of the big one
    fill(x + 36, y - 6, 2, 1, leaf); fill(x + 42, y - 6, 2, 1, leafDark);
    dot(x + 37, y - 7, leafLight); dot(x + 36, y - 8, inside); dot(x + 38, y - 8, leafLight); dot(x + 37, y - 8, insideLight, 0.6);
    line(cx, ty + 1, cx, y - 6, leafDark); dot(cx + 1, y - 7, leaf);
    // The big trap: open, snapped shut once the fly lands, then opening slowly
    const t = (now + x * 0.43) % 25, shut = t >= 4 && t < 13, half = t >= 13 && t < 15;
    if (shut) {
      fill(cx - 1, ty - 3, 1, 3, leaf); fill(cx + 1, ty - 3, 1, 3, leafDark); fill(cx, ty - 3, 1, 3, inside);
      dot(cx - 1, ty - 4, leafLight); dot(cx + 1, ty - 4, leafLight); dot(cx, ty, leaf);
    } else if (half) {
      dot(cx - 2, ty - 3, leaf); dot(cx - 1, ty - 2, leaf); dot(cx + 2, ty - 3, leafDark); dot(cx + 1, ty - 2, leafDark);
      dot(cx - 1, ty - 3, inside); dot(cx + 1, ty - 3, inside); dot(cx, ty - 1, inside); dot(cx, ty, leaf);
      dot(cx - 2, ty - 4, leafLight); dot(cx + 2, ty - 4, leafLight);
    } else {
      // Two lobes open like a clam on their hinge, red inside, with pale teeth along their rims
      fill(cx - 3, ty - 3, 2, 3, leaf); fill(cx - 2, ty - 3, 1, 3, inside); dot(cx - 2, ty - 2, insideLight);
      fill(cx + 2, ty - 3, 2, 3, leafDark); fill(cx + 2, ty - 3, 1, 3, inside);
      fill(cx - 1, ty - 1, 3, 1, inside); fill(cx - 1, ty, 3, 1, leaf); dot(cx + 1, ty, leafDark);
      for (const [dx, dy] of [[-4, -4], [-3, -4], [-2, -4], [2, -4], [3, -4], [4, -4]]) dot(cx + dx, ty + dy, leafLight, Math.abs(dx) === 3 ? 1 : 0.7);
    }
    // The fly: buzzing round for 3 s, landing in the open trap, gone once it shuts
    if (t < 4) {
      const a = now * 9, r = t < 3 ? 3 : 0;
      const fx = cx + Math.round(Math.cos(a) * r * 1.3), fy = ty - 3 - (t < 3 ? 2 : 0) + Math.round(Math.sin(a) * r * 0.6);
      dot(fx, fy, 0x2a2a30); dot(fx, fy - 1, WHITE, t < 3 && Math.floor(now * 12) % 2 ? 0.7 : 0.35);
    }
  },
  /** A portable cassette player with foam headphones on a cable: every so often the two reels behind its little window turn for a few seconds and a tiny note floats off the headphones. */
  cassette(x, y) {
    const body = 0x6e727c, bodyLight = 0xa8adb6, bodyDark = 0x4a4e58, foam = 0xe8782e, foamDark = 0xa8501e, band = 0x2a2d36;
    const t = (now + x * 0.53) % 22, playing = t < 6;
    // The headphones lying beside it: a dark band arching over two orange foam pads
    dot(x + 33, y - 5, band); dot(x + 34, y - 6, band); fill(x + 35, y - 7, 2, 1, band); dot(x + 37, y - 6, band); dot(x + 38, y - 5, band);
    fill(x + 32, y - 4, 3, 3, foam); fill(x + 32, y - 2, 3, 1, foamDark); dot(x + 32, y - 4, 0xf8a868);
    fill(x + 37, y - 4, 3, 3, foam); fill(x + 37, y - 2, 3, 1, foamDark); dot(x + 37, y - 4, 0xf8a868);
    // The cable from the right pad to the player's socket
    dot(x + 40, y - 1, band); dot(x + 41, y - 1, band, 0.7);
    // The player: a steel case with a lit top edge, a window onto the tape, orange keys along the top
    fill(x + 40, y - 8, 6, 7, body); fill(x + 40, y - 8, 6, 1, bodyLight); fill(x + 40, y - 8, 1, 7, bodyLight); fill(x + 45, y - 7, 1, 6, bodyDark);
    fill(x + 41, y - 9, 1, 1, foam); fill(x + 43, y - 9, 1, 1, playing ? foamDark : foam);
    // The window: the tape's cream label across the top, two brown reels of tape, a white hub on each that turns as it plays
    fill(x + 41, y - 6, 4, 3, 0x3a3d48); fill(x + 41, y - 6, 4, 1, PALETTE.paper, 0.85);
    fill(x + 41, y - 5, 4, 2, 0x6a4a30); dot(x + 42, y - 4, 0x3a3d48); dot(x + 43, y - 4, 0x3a3d48);
    const spin = playing ? Math.floor(now * 6) % 2 : 0;
    dot(x + 41, y - 5 + spin, 0xeae6dc, 0.8); dot(x + 44, y - 4 - spin, 0xeae6dc, 0.8);
    dot(x + 41, y - 2, bodyDark); dot(x + 44, y - 2, 0x6af07a, playing ? 1 : 0.25);
    // A tiny note drifting up off the headphones while it plays
    if (playing && t > 1) {
      const rise = Math.floor((t - 1) * 1.6), nx = x + 35 + (rise % 2), ny = y - 9 - rise;
      if (ny >= y - 15) { dot(nx, ny, WHITE, 0.85); dot(nx, ny - 1, WHITE, 0.6); dot(nx + 1, ny - 2, WHITE, 0.6); dot(nx - 1, ny, WHITE, 0.6); }
    }
  },
  /** A ball-and-stick model of a DNA double helix on a wooden stand: it turns slowly, a step every few seconds, its rungs shifting as it turns. */
  dnamodel(x, y) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, steel = 0xa8adb6;
    const strands = [0xd8402a, 0x4c8bf5], rungs = [0xf2d35b, 0x6ab070, 0xf28fb0, 0xe8782e];
    const cx = x + 39, turn = Math.floor((now + x * 0.3) / 6) * 0.5;
    // The stand: a round wooden base and a thin steel rod up the middle
    fill(cx - 3, y - 2, 7, 2, wood); fill(cx - 3, y - 2, 7, 1, woodLight); fill(cx - 2, y - 1, 6, 1, woodDark);
    fill(cx, y - 14, 1, 12, steel, 0.45);
    // The helix, row by row: two strands winding round each other, a coloured rung across every other row
    for (let r = 0; r < 11; r++) {
      const a = r * 0.62 + turn, off = Math.round(Math.sin(a) * 3), ry = y - 13 + r, aFront = Math.cos(a) > 0;
      // A rung of two halves, a base pair, across every other row where the strands are apart
      if (r % 2 === 0 && Math.abs(off) > 1) {
        const pair = (r / 2) % 2, left = cx - Math.abs(off) + 1, half = Math.abs(off);
        fill(left, ry, half, 1, rungs[pair * 2], 0.9); fill(left + half, ry, half - 1, 1, rungs[pair * 2 + 1], 0.9);
      }
      // The strand behind, a shade darker, then the one in front
      const [backX, backC, frontX, frontC] = aFront ? [cx - off, strands[1], cx + off, strands[0]] : [cx + off, strands[0], cx - off, strands[1]];
      dot(backX, ry, shaded(backC, 0.75));
      dot(frontX, ry, frontC);
      if (Math.abs(off) === 3) dot(frontX, ry, WHITE, 0.2);
    }
  },
  /** A small wooden dice tower with a tray: every 30 s a die tumbles out of the bottom, bounces twice along the tray and settles on a new face. */
  dicetower(x, y) {
    const wood = 0xa8703e, woodLight = 0xc8905a, woodDark = 0x6a4428, felt = 0x3b7a46, pip = 0x2a2d36;
    const cycle = Math.floor((now + x * 0.61) / 30), t = (now + x * 0.61) % 30;
    // The tray: a green felt floor inside low wooden walls
    fill(x + 37, y - 3, 7, 3, woodDark); fill(x + 38, y - 3, 5, 2, felt); fill(x + 37, y - 3, 7, 1, woodLight); fill(x + 37, y - 1, 7, 1, wood);
    // The tower: a tall box with a slot on top, a little roof, a stair-step front and its arched exit by the tray
    fill(x + 33, y - 12, 5, 12, wood); fill(x + 33, y - 12, 1, 12, woodLight); fill(x + 37, y - 12, 1, 12, woodDark);
    fill(x + 32, y - 13, 7, 1, woodDark); fill(x + 34, y - 12, 3, 1, 0x1a1210);
    fill(x + 34, y - 9, 3, 1, woodDark, 0.6); fill(x + 34, y - 6, 3, 1, woodDark, 0.6);
    fill(x + 36, y - 4, 2, 3, 0x1a1210); dot(x + 37, y - 5, woodDark);
    // The die: hidden in the tower for a second, out of the exit, two bounces left along the tray, then settled
    const tumbling = t >= 1 && t < 2, face = tumbling ? 1 + (Math.floor(now * 10) % 6) : 1 + (hash(cycle, x) % 6);
    let dx = null, dy = 0;
    if (t < 1) dx = null;
    else if (t < 1.3) dx = x + 37;
    else if (t < 1.6) { dx = x + 38; dy = -1; }
    else if (t < 1.8) dx = x + 39;
    else if (t < 2) { dx = x + 40; dy = -1; }
    else dx = x + 39;
    if (dx !== null) {
      const dtop = y - 4 + dy;
      fill(dx, dtop, 3, 3, WHITE); fill(dx + 2, dtop, 1, 3, 0xd8d4cc); fill(dx, dtop + 2, 3, 1, 0xd8d4cc);
      const pips = { 1: [[1, 1]], 2: [[0, 0], [2, 2]], 3: [[0, 0], [1, 1], [2, 2]], 4: [[0, 0], [2, 0], [0, 2], [2, 2]],
        5: [[0, 0], [2, 0], [1, 1], [0, 2], [2, 2]], 6: [[0, 0], [0, 1], [0, 2], [2, 0], [2, 1], [2, 2]] }[face];
      for (const [px, py] of pips) dot(dx + px, dtop + py, face === 1 ? 0xd8402a : pip);
    }
  },
  /** A tomato-shaped kitchen timer with a green stalk and a cream dial band: the dial creeps one notch at a time, and every 40 s it rattles side to side with a yellow ding glint, then resets. */
  eggtimer(x, y) {
    const red = 0xd8402a, redLight = 0xf06a4c, redDark = 0x9a2a1c, cream = 0xf0e6c8, green = 0x4f9a5a, greenDark = 0x3b7a46;
    const t = (now + x * 0.31) % 40, ringing = t > 36.5, shake = ringing ? (Math.floor(now * 12) % 2 ? 1 : -1) : 0;
    const notch = Math.floor(t / 40 * 8) % 8;
    const bx = x + 35 + shake;
    fill(x + 34, y, 11, 1, SHADOW);
    // The round red body
    fill(bx + 1, y - 9, 7, 8, red); fill(bx, y - 8, 9, 6, red); fill(bx + 2, y - 1, 5, 1, redDark);
    fill(bx, y - 7, 1, 4, redLight); fill(bx + 1, y - 9, 2, 1, redLight); fill(bx + 8, y - 6, 1, 4, redDark); fill(bx + 7, y - 2, 1, 1, redDark);
    // The cream dial band, with its tick marks and a hand that moves one notch at a time
    fill(bx, y - 6, 9, 3, cream); fill(bx, y - 4, 9, 1, 0xd8cca8);
    for (const tx of [1, 4, 7]) dot(bx + tx, y - 6, redDark, 0.7);
    dot(bx + Math.min(7, 1 + notch), y - 5, redDark);
    // The green stalk and leaves on top
    fill(bx + 4, y - 11, 1, 2, greenDark); fill(bx + 2, y - 10, 2, 1, green); fill(bx + 5, y - 10, 2, 1, green); dot(bx + 2, y - 10, greenDark);
    dot(bx + 2, y - 8, WHITE, 0.6);
    // The ding
    if (ringing) { dot(bx + 10, y - 10, 0xf2d35b); dot(bx + 11, y - 8, 0xf2d35b, 0.7); dot(bx - 2, y - 9, 0xf2d35b, 0.7); }
  },
  /** One tall sunflower in a terracotta pot, its head turning toward the window over the day (three positions by the hour) and drooping at night; every 45 s a leaf gives a small bounce. */
  sunflower(x, y) {
    const pot = 0xa0603e, potLight = 0xbc7a52, potDark = 0x7a4426, stem = 0x3b7a46, leaf = 0x4f9a5a, leafLight = 0x6ab474;
    const petal = 0xf2c230, petalLight = 0xffe070, petalDark = 0xd49a1c, seeds = 0x5a3b25, seedsDark = 0x3a2616;
    const look = isNight ? 3 : daylight < 11 ? 0 : daylight < 15 ? 1 : 2; // left, up, right, drooped
    const t = (now + x * 0.29) % 45, bounce = t < 1.2 ? [1, 2, 1, 0, 1][Math.floor(t * 4)] || 0 : 0;
    const drop = look === 3 ? 2 : 0, lean = [-2, 0, 2, 1][look];
    fill(x + 36, y, 8, 1, SHADOW);
    // The terracotta pot
    fill(x + 37, y - 3, 6, 3, pot); fill(x + 36, y - 4, 8, 1, potLight); fill(x + 37, y - 3, 1, 3, potLight); fill(x + 42, y - 3, 1, 3, potDark); fill(x + 38, y - 1, 4, 1, potDark, 0.5);
    // The stem leans a little toward the sun
    const hx = x + 40 + lean, hy = y - 9 + drop;
    line(x + 40, y - 4, hx, hy + 2, stem); line(x + 41, y - 4, hx + 1, hy + 2, stem);
    // Two broad leaves, the left one bouncing now and then
    fill(x + 36, y - 6 - bounce, 4, 2, leaf); fill(x + 35, y - 5 - bounce, 2, 1, leaf); dot(x + 37, y - 6 - bounce, leafLight);
    fill(x + 42, y - 7 + drop, 4, 2, leaf); fill(x + 45, y - 6 + drop, 1, 1, leaf); dot(x + 43, y - 7 + drop, leafLight);
    // The head: a ring of petals round a brown middle, which shifts toward where it looks
    for (const [dx, dy] of [[0, -3], [-3, 0], [3, 0], [0, 3], [-2, -2], [2, -2], [-2, 2], [2, 2]]) fill(hx + dx - (dx === 0 ? 0 : 0), hy + dy, dx === 0 ? 1 : 1, 1, petal);
    disc(hx, hy, 2.6, petal); fill(hx - 1, hy - 4, 3, 1, petal); fill(hx - 1, hy + 4, 3, 1, petalDark); fill(hx - 4, hy - 1, 1, 3, petal); fill(hx + 4, hy - 1, 1, 3, petalDark);
    dot(hx - 3, hy - 3, petalLight); dot(hx - 1, hy - 4, petalLight); dot(hx + 3, hy + 3, petalDark); dot(hx - 4, hy - 1, petalLight);
    const mx = hx + [-1, 0, 1, 0][look], my = hy + (look === 3 ? 1 : 0);
    disc(mx, my, 1.9, seeds); dot(mx + 1, my + 1, seedsDark); dot(mx - 1, my - 1, 0x7a5232);
  },
  /** A glass jar of sourdough starter under a string-tied cloth: the pale dough slowly rises up the glass over a 2-minute cycle, a bubble or two pops at the surface, then it sinks back. */
  sourdough(x, y) {
    const glass = 0xdff0f6, dough = 0xf0e2bc, doughDark = 0xd8c690, cloth = 0xf4f0e6, clothDark = 0xcfc8b6, string = 0xb8883e;
    const c = ((now + x * 0.7) % 120) / 120;
    const rise = c < 0.6 ? c / 0.6 : c < 0.7 ? 1 : 1 - (c - 0.7) / 0.3; // up for a minute, a pause at the top, then back down
    const level = 1 + Math.round(rise * 4); // dough height in pixels, 1..5 (jar interior is 7 high)
    fill(x + 36, y, 9, 1, SHADOW);
    // The jar: glass walls and base, then the dough inside
    fill(x + 37, y - 9, 7, 9, glass, 0.42); fill(x + 37, y - 9, 1, 9, glass, 0.7); fill(x + 43, y - 9, 1, 9, 0xa8c4cc, 0.6); fill(x + 37, y - 1, 7, 1, 0xa8c4cc, 0.6);
    fill(x + 38, y - 1 - level, 5, level, dough); fill(x + 38, y - 1 - level, 5, 1, 0xfff6dc); fill(x + 38, y - 2, 5, 1, doughDark);
    for (const [bx, by] of [[39, 2], [41, 3], [40, 1]]) if (level > by) dot(x + bx, y - 1 - by, doughDark); // bubbles inside
    // A bubble pops at the surface now and then, only when the dough is high
    const pop = Math.floor(now * 2 + x) % 7;
    if (level >= 4 && pop < 2) dot(x + 39 + pop * 2, y - 2 - level, 0xfff6dc, 0.9);
    // The cloth on top, tied with string
    fill(x + 36, y - 12, 9, 3, cloth); fill(x + 36, y - 12, 9, 1, WHITE, 0.6); fill(x + 36, y - 10, 9, 1, clothDark);
    dot(x + 36, y - 9, cloth); dot(x + 44, y - 9, cloth); dot(x + 35, y - 10, cloth, 0.8); dot(x + 45, y - 10, cloth, 0.8);
    fill(x + 37, y - 9, 7, 1, string); dot(x + 44, y - 8, string);
    dot(x + 38, y - 7, WHITE, 0.8); fill(x + 38, y - 6, 1, 3, WHITE, 0.35);
  },
  /** A round wicker basket with two yarn balls (pink, sage) and two needles through a half-knitted scarf over the rim: every 25 s the needles click together twice and the scarf grows a row, up to a limit, then starts again. */
  knitbasket(x, y) {
    const wick = 0xc89a5a, wickLight = 0xe0b878, wickDark = 0x8a6234, pink = 0xf28fb0, pinkDark = 0xc8688a, sage = 0x9ab890, sageDark = 0x6e8c68;
    const scarf = 0xe8c070, scarfDark = 0xc89c48, needle = 0xd8d4c8;
    const n = Math.floor((now + x * 0.9) / 25), rows = n % 4, t = (now + x * 0.9) % 25;
    const click = t < 1 && Math.floor(t * 4) % 2 === 0 ? 1 : 0; // the needles close for a moment, twice
    fill(x + 33, y, 13, 1, SHADOW);
    // The two yarn balls, with a wound strand across each
    disc(x + 37, y - 6, 3, pink); disc(x + 42, y - 6, 3, sage);
    line(x + 35, y - 7, x + 39, y - 5, pinkDark); line(x + 40, y - 7, x + 44, y - 5, sageDark); dot(x + 36, y - 8, WHITE, 0.6); dot(x + 41, y - 8, WHITE, 0.5);
    // The scarf, hanging over the right rim and growing a row at a time
    fill(x + 44, y - 6, 2, 1, scarf);
    fill(x + 45, y - 5, 2, 2 + rows, scarf); fill(x + 46, y - 5, 1, 2 + rows, scarfDark);
    for (let r = 0; r < rows + 2; r += 2) dot(x + 45, y - 4 + r, scarfDark);
    // The needles, crossing over the balls
    line(x + 36, y - 12, x + 41 - click, y - 6, needle); line(x + 43 - click, y - 12, x + 40, y - 6, needle);
    dot(x + 36, y - 12, WHITE, 0.8); dot(x + 43 - click, y - 12, WHITE, 0.8);
    if (click) dot(x + 40, y - 13, 0xf2d35b, 0.9);
    // The wicker basket, with its woven rows
    fill(x + 34, y - 4, 11, 4, wick); fill(x + 33, y - 5, 13, 1, wickLight); fill(x + 33, y - 5, 1, 2, wickLight); fill(x + 45, y - 5, 1, 2, wickDark);
    fill(x + 35, y - 1, 9, 1, wickDark); for (let i = 0; i < 5; i++) { dot(x + 35 + i * 2, y - 3, wickDark); dot(x + 36 + i * 2, y - 2, wickDark); }
    fill(x + 34, y - 4, 1, 3, wickLight);
  },
  /** A glossy red rotary phone with a cream dial and its handset on the cradle, a coiled cord curling off; every 50 s it rings: the handset jiggles for 2 s with two tiny ring glints. */
  rotaryphone(x, y) {
    const red = 0xc8321f, redLight = 0xe8604a, redDark = 0x8e2214, cream = 0xf0e6cc, dial = 0x3a2a28;
    const t = (now + x * 0.31) % 50, ringing = t < 2, f = ringing ? Math.floor(now * 8) % 2 : 0;
    fill(x + 35, y, 10, 1, SHADOW);
    // The cord, coiling off to the left
    for (let i = 0; i < 4; i++) { dot(x + 32 + i, y - 1 - (i % 2), redDark); }
    // The body: a rounded slope with the dial on its face
    fill(x + 35, y - 4, 10, 4, red); fill(x + 37, y - 6, 6, 2, red); fill(x + 38, y - 7, 4, 1, red);
    fill(x + 35, y - 4, 1, 4, redLight); fill(x + 37, y - 6, 1, 2, redLight); fill(x + 44, y - 4, 1, 4, redDark); fill(x + 35, y - 1, 10, 1, redDark);
    disc(x + 40, y - 3, 2.2, cream); dot(x + 40, y - 3, dial);
    dot(x + 39, y - 4, dial, 0.6); dot(x + 41, y - 4, dial, 0.6); dot(x + 39, y - 2, dial, 0.6); dot(x + 41, y - 2, dial, 0.6);
    // The handset lying across the cradle, jiggling when it rings
    const hy = y - 10 + f, hx = x + 35 + (ringing ? f - 1 : 0) * 0;
    fill(hx, hy, 11, 2, red); fill(hx, hy + 2, 3, 2, red); fill(hx + 8, hy + 2, 3, 2, red);
    fill(hx, hy, 11, 1, redLight); fill(hx + 8, hy + 3, 3, 1, redDark); fill(hx, hy + 3, 3, 1, redDark);
    if (ringing) {
      const g = Math.floor(now * 4) % 2;
      dot(x + 34 - g, y - 11 - g, 0xffe08a); dot(x + 47 - 2 + g, y - 11 - g, 0xffe08a);
      dot(x + 33 - g, y - 9, 0xffe08a, 0.6); dot(x + 46 + g, y - 9, 0xffe08a, 0.6);
    }
    dot(x + 36, y - 3, WHITE, 0.5);
  },
  /** A chunky grey adding machine with a paper roll on top and a side lever; every 30 s the lever pulls down and back and the paper tape grows a line over the front edge, tearing off after six pulls. */
  addingmachine(x, y) {
    const grey = 0x8a8e98, greyLight = 0xb4b8c0, greyDark = 0x5a5e68, paper = 0xf0ece0, key = 0xd8d4c8;
    const cyc = (now + x * 0.31) % 180, pulls = Math.floor(cyc / 30), tc = cyc % 30;
    const pulling = tc < 1.2 && pulls > 0 || (cyc < 1.2 && false);
    const lever = pulling ? (tc < 0.6 ? 2 : 1) : 0;
    fill(x + 34, y, 11, 1, SHADOW);
    // The body, wider at the bottom
    fill(x + 34, y - 6, 10, 6, grey); fill(x + 33, y - 3, 12, 3, grey);
    fill(x + 34, y - 6, 10, 1, greyLight); fill(x + 33, y - 3, 1, 3, greyLight); fill(x + 44, y - 3, 1, 3, greyDark); fill(x + 33, y - 1, 12, 1, greyDark);
    // The keys: a little grid of pale dots
    for (let r = 0; r < 2; r++) for (let c = 0; c < 4; c++) dot(x + 35 + c * 2, y - 5 + r * 2 + (r ? 0 : 0), key);
    // The paper roll on top with its tape
    fill(x + 37, y - 10, 5, 4, paper); fill(x + 37, y - 10, 1, 4, WHITE, 0.5); fill(x + 41, y - 10, 1, 4, 0xc8c4b4);
    fill(x + 36, y - 10, 1, 4, greyDark); fill(x + 42, y - 10, 1, 4, greyDark);
    // The tape curling down the front, one printed line per pull
    const len = Math.min(pulls + (pulling ? 1 : 0), 6);
    if (len > 0) { fill(x + 38, y - 6, 3, 1, paper); fill(x + 38, y - 5, 3, Math.min(len, 4), paper); for (let i = 0; i < Math.min(len, 4); i++) if (i % 2 === 0) fill(x + 39, y - 5 + i, 1, 1, 0x8a8e98); if (len > 4) dot(x + 41, y - 2, paper); }
    // The side lever, down when pulled
    const ly = [[-8, -9], [-6, -8], [-3, -6]][lever];
    line(x + 45, y - 4, x + 45 + (lever === 0 ? 0 : 1), y + ly[0] - 1, greyDark);
    const kx = x + 45 + (lever === 0 ? 0 : 1), ky = y + ly[0] - 3 + (lever === 2 ? 3 : 0); fill(kx - 1, ky + 1, 2, 2, 0xc83a2a); dot(kx - 1, ky + 1, 0xe8604a); dot(kx, ky + 2, 0x9a2a1e);
  },
  /** A soldering station: a squat grey base with a red dial and a lit LED, the iron resting tip-down in its coil stand with its blue grip up; every 25 s the tip glows orange and a thin wisp of smoke rises in three steps. */
  solderstation(x, y) {
    const grey = 0x7a7e88, greyLight = 0xa8adb6, greyDark = 0x4a4e58;
    const t = (now + x * 0.31) % 25, hot = t < 3.5, step = Math.floor(t * 1.2);
    fill(x + 33, y, 14, 1, SHADOW);
    // The base with its dial and LED
    fill(x + 33, y - 4, 6, 4, grey); fill(x + 33, y - 4, 6, 1, greyLight); fill(x + 33, y - 4, 1, 4, greyLight); fill(x + 38, y - 4, 1, 4, greyDark); fill(x + 33, y - 1, 6, 1, greyDark);
    disc(x + 35, y - 2, 1, 0xd8402a); dot(x + 35, y - 3, 0xf08070); dot(x + 37, y - 3, 0x6af07a);
    // The coil stand: a little foot and a steel spring tilted up to the right
    fill(x + 39, y - 1, 5, 1, greyDark); fill(x + 39, y - 1, 1, 1, greyLight);
    for (const [cx, cy] of [[40, -2], [41, -3], [42, -4]]) { dot(x + cx, y + cy, greyLight); dot(x + cx + 1, y + cy, greyDark); }
    // The iron in it: the steel tip down in the coil, a dark blue grip sticking up and out
    const tipC = hot ? 0xff8030 : 0xb0b4bc;
    dot(x + 40, y - 3, tipC); dot(x + 41, y - 4, 0xb0b4bc); dot(x + 42, y - 5, 0x8a8e98);
    fill(x + 42, y - 6, 2, 1, 0x3a4a6a); fill(x + 43, y - 7, 2, 1, 0x3a4a6a); fill(x + 44, y - 8, 1, 1, 0x2a3650); dot(x + 42, y - 6, 0x5a6e94);
    // The smoke wisp, three steps up from the tip
    if (hot && step < 4) { const s = Math.min(step, 2); dot(x + 40 - (s % 2), y - 5 - s * 2, WHITE, 0.55 - s * 0.12); if (s > 0) dot(x + 40, y - 5 - s, WHITE, 0.3); }
    if (hot && isNight) glowLate(() => { glowAt(x + 40, y - 3, 3, 3, 0xff8030, 0.2); dot(x + 40, y - 3, 0xffd090); });
  },
  /** A chunky cream instant camera with a rainbow stripe and a big lens; every 60 s it flashes and a photo slides out of the top slot, darkening from grey to a little picture in three steps, then slides back in. */
  instantcam(x, y) {
    const cream = 0xeee4cc, creamLight = 0xfaf4e4, creamDark = 0xb8ac90, lens = 0x2a3040;
    const t = (now + x * 0.31) % 60;
    fill(x + 34, y, 11, 1, SHADOW);
    // The photo, sliding out of the slot in steps, developing, then sliding back
    const out = t < 0.4 ? 0 : t < 1.2 ? 1 : t < 7 ? 3 : t < 7.6 ? 2 : t < 8.2 ? 1 : 0;
    if (out > 0) {
      const dev = t < 3 ? 0 : t < 5 ? 1 : 2, py = y - 9 - out;
      fill(x + 35, py, 7, 5, 0xf8f6ee); fill(x + 35, py, 1, 5, WHITE, 0.6); fill(x + 41, py, 1, 5, 0xd0ccc0);
      const pic = [[0xa0a4a8, 0xa0a4a8], [0x6a8a9a, 0x7a7a6a], [0x4c8bf5, 0x4f9a5a]][dev];
      fill(x + 36, py + 1, 5, 3, pic[0]); if (dev === 2) { fill(x + 36, py + 3, 5, 1, pic[1]); dot(x + 39, py + 2, 0xf2d35b); } else fill(x + 36, py + 3, 5, 1, pic[1]);
    }
    // The body: a chunky box, a darker top plate with the slot, a hump for the viewfinder
    fill(x + 34, y - 9, 11, 9, cream); fill(x + 34, y - 9, 11, 1, creamLight); fill(x + 34, y - 9, 1, 9, creamLight);
    fill(x + 44, y - 9, 1, 9, creamDark); fill(x + 34, y - 1, 11, 1, creamDark); fill(x + 35, y - 10, 7, 1, creamDark); fill(x + 36, y - 9, 5, 1, 0x4a4438);
    fill(x + 42, y - 11, 2, 2, creamDark); dot(x + 42, y - 11, creamLight);
    // The rainbow stripe across the front
    fill(x + 34, y - 4, 11, 1, 0xd8402a); fill(x + 34, y - 3, 11, 1, 0xe8782e); fill(x + 34, y - 2, 11, 1, 0xf2d35b);
    // The lens: a big dark disc with a glass glint
    disc(x + 38.5, y - 5.5, 3.4, creamDark); disc(x + 38.5, y - 5.5, 2.6, lens); disc(x + 38.5, y - 5.5, 1.2, 0x4c6a9a); dot(x + 37, y - 7, WHITE, 0.8);
    // The flash window, bursting white when it shoots
    const f = t < 0.4;
    fill(x + 42, y - 8, 2, 2, f ? WHITE : 0xd8e4ea);
    if (f) { dot(x + 41, y - 10, WHITE, 0.8); dot(x + 45, y - 9, WHITE, 0.8); dot(x + 43, y - 12, WHITE, 0.7); dot(x + 46, y - 7, WHITE, 0.6); dot(x + 40, y - 8, WHITE, 0.5); }
    if (f && isNight) glowLate(() => glowAt(x + 43, y - 7, 6, 5, 0xffffff, 0.3));
  },
  /** A garden pinwheel: four swirled vanes (red, yellow, blue, green) on a stick in a small pot of soil; every 20 s it spins for 3 s in 45° steps, then slows to rest at a new angle. */
  pinwheel(x, y) {
    const red = 0xd8402a, yellow = 0xf2d35b, blue = 0x4c8bf5, green = 0x4f9a5a, stick = 0xa8703e;
    const pot = PALETTE.pot, potLight = 0xc07a52, potDark = 0x7a452a, soil = 0x4a3020;
    const cx = x + 39, cy = y - 9;
    // The pot, its soil and the shadow
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 36, y - 4, 7, 4, pot); fill(x + 36, y - 4, 1, 4, potLight); fill(x + 42, y - 4, 1, 4, potDark);
    fill(x + 35, y - 5, 9, 1, potLight); fill(x + 36, y - 1, 7, 1, potDark);
    fill(x + 36, y - 5, 7, 1, soil); dot(x + 38, y - 5, 0x6a4a2e); dot(x + 41, y - 5, 0x6a4a2e);
    // The stick
    fill(cx, cy + 1, 1, y - 6 - cy, stick); dot(cx, cy + 2, 0xc8884a);
    // The vanes: four solid swirled triangles round the hub; for 3 s every 20 s they spin in 45° steps, then rest at a new angle
    const tt = now + x * 0.31, cycle = Math.floor(tt / 20), t = tt % 20;
    const angleOf = (c) => (Math.imul(c + 3, 2654435761) >>> 28) % 8;
    const pos = t < 3 ? (Math.floor(t * (t < 2 ? 8 : 4)) + angleOf(cycle - 1)) % 8 : angleOf(cycle);
    // One vane pointing up, square-on and turned 45°; the last pixel is its darker tip
    const shapes = [[[0, -1], [0, -2], [1, -2], [0, -3], [1, -3], [2, -3]], [[1, -1], [1, -2], [2, -2], [2, -3], [3, -3]]];
    const vanes = [red, yellow, blue, green], shape = shapes[pos % 2], turns = Math.floor(pos / 2);
    for (let k = 0; k < 4; k++) {
      const r = (k + turns) % 4, colour = vanes[k];
      shape.forEach(([dx, dy], i) => {
        let ux = dx, uy = dy;
        for (let q = 0; q < r; q++) [ux, uy] = [-uy, ux];
        dot(cx + ux, cy + uy, i === shape.length - 1 ? shaded(colour, 0.85) : i === 0 ? shaded(colour, 1.12) : colour);
      });
    }
    dot(cx, cy, 0xf4ecd8); dot(cx, cy, WHITE, 0.6);
  },
  /** A Claude spark lamp: a terracotta starburst on a charcoal stem; now and then it twinkles, and it glows softly at night. */
  sparklamp(x, y) {
    const coal = 0x3a3631, coalLight = 0x57514a, coalDark = 0x26231f, spark = 0xd97757, sparkLight = 0xf0a080, core = 0xfbe3c8;
    const cx = x + 39, cy = y - 8;
    // The round base and the short stem
    fill(x + 36, y - 2, 7, 2, coal); fill(x + 36, y - 2, 7, 1, coalLight); fill(x + 42, y - 1, 1, 1, coalDark);
    fill(cx, y - 4, 1, 2, coalLight);
    fill(x + 35, y, 9, 1, SHADOW);
    // The spark, its rays a step longer while it twinkles (3 s every 26 s)
    const t = (now + x * 0.31) % 26, grow = t < 3 && Math.floor(t * 3) % 2 === 0 ? 1 : 0;
    if (isNight) glowLate(() => { glowAt(cx, cy, 7, 6, 0xffa070, 0.16); claudeSpark(cx, cy, 3, sparkLight, 1, grow); dot(cx, cy, core); });
    claudeSpark(cx, cy, 3, spark, 1, grow);
    dot(cx - 1, cy - 1, sparkLight); dot(cx, cy - 2, sparkLight); dot(cx, cy, core, 0.9);
  },
  /** A Claude mug: a tall cream mug with the terracotta spark on it; now and then a curl of steam rises from the coffee. */
  claudemug(x, y) {
    const cream = 0xf2ebdc, creamLight = 0xfffaf0, creamDark = 0xcfc3ad, coffee = 0x4a2e1e, spark = 0xd97757;
    // The mug, its rim of coffee and its handle
    fill(x + 36, y - 7, 7, 7, cream); fill(x + 36, y - 7, 1, 7, creamLight); fill(x + 42, y - 7, 1, 7, creamDark);
    fill(x + 37, y - 7, 5, 1, coffee); fill(x + 36, y - 1, 7, 1, creamDark);
    fill(x + 43, y - 6, 2, 1, cream); fill(x + 44, y - 5, 1, 2, creamDark); fill(x + 43, y - 3, 2, 1, creamDark);
    fill(x + 35, y, 10, 1, SHADOW);
    // The spark on its side
    claudeSpark(x + 39, y - 4, 2, spark); dot(x + 39, y - 4, 0xf0a080);
    dot(x + 37, y - 6, WHITE, 0.6);
    // Steam, 4 s in every 15
    const t = (now + x * 0.31) % 15;
    if (t < 4) {
      const k = Math.floor(t * 2);
      dot(x + 38 + (k % 2), y - 9 - (k % 4), WHITE, 0.55 - (k % 4) * 0.1);
      dot(x + 40 - (k % 2), y - 8 - ((k + 2) % 4), WHITE, 0.45 - ((k + 2) % 4) * 0.08);
    }
  },
  /** A Claude plushie: the little terracotta Claude Code critter in felt, stubby arms and four legs; it blinks now and then. */
  clawdplush(x, y) {
    const felt = 0xd97757, feltLight = 0xeb977a, feltDark = 0xb35a3c, eye = 0x2a2420;
    // The body, its arms and its four legs
    fill(x + 36, y - 7, 7, 5, felt); fill(x + 36, y - 7, 7, 1, feltLight); fill(x + 36, y - 6, 1, 4, feltLight);
    fill(x + 42, y - 6, 1, 4, feltDark); fill(x + 37, y - 3, 5, 1, feltDark);
    fill(x + 35, y - 5, 1, 2, felt); fill(x + 43, y - 5, 1, 2, feltDark);
    for (const lx of [36, 38, 40, 42]) fill(x + lx, y - 2, 1, 2, lx === 42 ? feltDark : felt);
    fill(x + 35, y, 9, 1, SHADOW);
    // Its eyes, shut for a moment every 9 s
    const t = (now + x * 0.31) % 9, shut = t < 0.25;
    for (const ex of [38, 40]) fill(x + ex, y - (shut ? 5 : 6), 1, shut ? 1 : 2, eye);
  },
  /** A Necronomicon-style tome lying open: cracked purple leather, brass corners, a faint green rune breathing on the page; now and then a page lifts and turns. */
  necrotome(x, y) {
    const cover = 0x4a2c38, coverLight = 0x6a4252, coverDark = 0x2e1a24, page = 0xe4dcc0, pageDark = 0xbcb08e, ink = 0x6a5a48, rune = 0x7fffc0, brass = 0xc8a050;
    fill(x + 33, y, 14, 1, SHADOW);
    // The covers and the two page blocks
    fill(x + 34, y - 5, 12, 5, cover); fill(x + 34, y - 5, 12, 1, coverLight); fill(x + 34, y - 1, 12, 1, coverDark);
    fill(x + 35, y - 5, 5, 4, page); fill(x + 41, y - 5, 5, 4, page);
    fill(x + 40, y - 5, 1, 4, pageDark); fill(x + 35, y - 2, 5, 1, pageDark); fill(x + 41, y - 2, 5, 1, pageDark);
    // Crabbed lines of ink on the left page, a rune on the right
    for (const ly of [-4, -3]) { fill(x + 36, y + ly, 3, 1, ink, 0.7); }
    dot(x + 36, y - 2, ink, 0.7);
    const pulse = 0.6 + 0.4 * Math.sin(now * 1.5 + x);
    dot(x + 43, y - 4, rune, pulse); dot(x + 42, y - 3, rune, pulse * 0.8); dot(x + 44, y - 3, rune, pulse * 0.8); dot(x + 43, y - 2, rune, pulse);
    // Brass corners
    dot(x + 34, y - 5, brass); dot(x + 45, y - 5, brass); dot(x + 34, y - 1, brass); dot(x + 45, y - 1, brass);
    // A page turning every 16 s
    const t = (now + x * 0.31) % 16;
    if (t < 1.2) {
      const f = Math.floor(t * 5);
      if (f < 2) { fill(x + 41, y - 8 + f, 1, 4, page); dot(x + 41, y - 8 + f, WHITE, 0.6); }
      else if (f < 4) { fill(x + 40, y - 9, 1, 5, page); dot(x + 40, y - 9, WHITE, 0.6); }
      else { fill(x + 38, y - 8, 2, 4, page); fill(x + 38, y - 8, 1, 4, pageDark); }
    }
    if (isNight) glowLate(() => glowAt(x + 43, y - 3, 6, 4, rune, 0.16));
  },
  /** An inkwell with a little green tentacle for a quill, swaying; every so often it dips its feather tip into the ink. */
  tentacleink(x, y) {
    const glass = 0x2a3350, glassLight = 0x4a5a80, ink = 0x14102a, flesh = 0x5fa888, fleshLight = 0x8fd4b0, fleshDark = 0x3a7860, feather = 0xeae6dc;
    fill(x + 34, y, 10, 1, SHADOW);
    // The well: a squat dark bottle with a brass-less neck
    fill(x + 35, y - 5, 8, 5, glass); fill(x + 35, y - 5, 1, 5, glassLight); fill(x + 42, y - 5, 1, 5, 0x1a2038);
    fill(x + 37, y - 6, 4, 1, glass); fill(x + 37, y - 6, 4, 1, ink, 0.6); dot(x + 36, y - 4, WHITE, 0.5);
    // The tentacle curling up out of the neck
    const t = (now + x * 0.31) % 22, dip = t < 2.5 ? Math.round(Math.sin(t / 2.5 * Math.PI) * 3) : 0;
    const sway = Math.round(Math.sin(now * 0.7 + x * 0.2));
    const curve = [0, 0, 1, 2, 3, 3, 2];
    for (let i = 0; i < 7; i++) {
      const cy = y - 7 - i + (i >= 4 ? dip : 0), cx = x + 38 + curve[i] + (i > 2 ? sway : 0);
      fill(cx, cy, 2, 1, i % 2 ? flesh : fleshDark); dot(cx, cy, fleshLight);
      if (i % 2 === 0 && i > 0) dot(cx + 1, cy, fleshLight, 0.6);
    }
    // The quill feather held at its tip
    const tx = x + 40 + curve[6] + sway, ty = y - 13 + dip;
    line(tx, ty + 1, tx + 2, ty - 3, feather); dot(tx + 2, ty - 3, WHITE, 0.8); dot(tx, ty + 1, ink);
  },
  /** A small jade idol: a squat winged figure with a head of dangling tentacles on a stone plinth; its eyes glow for a moment now and then. */
  greenidol(x, y) {
    const jade = 0x5a9a6a, jadeLight = 0x8ac49a, jadeDark = 0x34603f, stone = 0x6a7078, stoneLight = 0x8a9098, stoneDark = 0x484e56, eyeC = 0xe0ff7a;
    fill(x + 34, y, 12, 1, SHADOW);
    // The plinth
    fill(x + 35, y - 2, 10, 2, stone); fill(x + 35, y - 2, 10, 1, stoneLight); fill(x + 35, y - 1, 10, 1, stoneDark);
    // Folded wings, the body and the head
    fill(x + 36, y - 8, 2, 6, jadeDark); fill(x + 42, y - 8, 2, 6, jadeDark);
    fill(x + 37, y - 7, 6, 5, jade); fill(x + 37, y - 7, 1, 5, jadeLight); fill(x + 42, y - 7, 1, 5, jadeDark);
    fill(x + 37, y - 12, 6, 5, jade); fill(x + 38, y - 13, 4, 1, jade); fill(x + 37, y - 12, 1, 5, jadeLight); fill(x + 42, y - 12, 1, 5, jadeDark);
    // The tentacles of its face
    for (const [dx, dy] of [[38, 7], [39, 6], [40, 7], [41, 6]]) { dot(x + dx, y - dy, jadeDark); dot(x + dx, y - dy + 1, jadeDark); }
    // Claws on its knees
    dot(x + 38, y - 3, jadeLight); dot(x + 41, y - 3, jadeLight);
    // Eyes, lit for 2 s every 14 s
    const t = (now + x * 0.31) % 14, lit = t < 2;
    dot(x + 38, y - 10, lit ? eyeC : jadeDark); dot(x + 41, y - 10, lit ? eyeC : jadeDark);
    dot(x + 38, y - 12, WHITE, 0.45);
    if (lit && isNight) glowLate(() => { glowAt(x + 40, y - 10, 6, 4, eyeC, 0.2); dot(x + 38, y - 10, 0xf8ffc0); dot(x + 41, y - 10, 0xf8ffc0); });
  },
  /** An Elder Sign amulet: a carved grey-green stone showing the branched sign, propped on a little wooden stand; its centre glows softly and slowly. */
  eldersignstand(x, y) {
    const stone = 0x7a8a82, stoneLight = 0xa4b4ac, stoneDark = 0x4e5e56, carve = 0x2e3a36, wood = 0x8a5a32, woodDark = 0x5a3b25, glow = 0x7fffc0;
    fill(x + 35, y, 10, 1, SHADOW);
    // The stand: a foot and a cradle
    fill(x + 36, y - 2, 8, 2, wood); fill(x + 36, y - 2, 8, 1, 0xa8703e); fill(x + 36, y - 1, 8, 1, woodDark);
    fill(x + 38, y - 4, 1, 2, woodDark); fill(x + 42, y - 4, 1, 2, woodDark);
    // The stone disc, with its chain loop
    disc(x + 40, y - 8, 4.2, stone); fill(x + 36, y - 9, 1, 3, stoneLight); fill(x + 44, y - 9, 1, 3, stoneDark); dot(x + 38, y - 11, stoneLight);
    dot(x + 40, y - 13, 0xc8a050); dot(x + 39, y - 12, 0xc8a050); dot(x + 41, y - 12, 0xc8a050);
    // The sign: a stem with branching strokes and an eye at the crown
    line(x + 40, y - 6, x + 40, y - 10, carve); line(x + 40, y - 9, x + 38, y - 11, carve); line(x + 40, y - 9, x + 42, y - 11, carve); line(x + 40, y - 7, x + 38, y - 9, carve); line(x + 40, y - 7, x + 42, y - 9, carve);
    const a = 0.5 + 0.5 * Math.sin(now * 0.9 + x * 0.3);
    dot(x + 40, y - 11, glow, 0.4 + a * 0.6);
    if (isNight) glowLate(() => { glowAt(x + 40, y - 8, 6, 6, glow, 0.08 + a * 0.1); dot(x + 40, y - 11, 0xd8fff0); });
  },
  /** A specimen jar: a pale-purple many-tentacled thing floats in green fluid with one big eye that blinks now and then, and bobs slowly. */
  specimenjar(x, y) {
    const glass = 0xbfe8d8, brass = 0xc8a050, brassDark = 0x8a6a2a, body = 0xc8a8d8, bodyDark = 0x9a78b0;
    fill(x + 35, y, 10, 1, SHADOW);
    // The jar, its fluid and the brass lid
    fill(x + 36, y - 9, 8, 9, glass, 0.45); fill(x + 36, y - 9, 1, 9, WHITE, 0.45); fill(x + 43, y - 9, 1, 9, glass, 0.7); fill(x + 36, y - 1, 8, 1, 0x6aa890, 0.6);
    fill(x + 36, y - 11, 8, 2, brass); fill(x + 36, y - 11, 8, 1, 0xe8c878); fill(x + 36, y - 10, 8, 1, brassDark);
    // The specimen, bobbing one pixel
    const bob = Math.round(Math.sin(now * 0.6 + x * 0.2));
    fill(x + 38, y - 6 + bob, 4, 3, body); fill(x + 38, y - 6 + bob, 1, 3, 0xe0c8ee); fill(x + 41, y - 5 + bob, 1, 2, bodyDark);
    for (const dx of [38, 39, 40, 41]) dot(x + dx, y - 3 + bob + (dx % 2), bodyDark);
    // The eye, shut for a moment every 8 s
    const shut = (now + x * 0.31) % 8 < 0.3;
    if (shut) fill(x + 39, y - 5 + bob, 2, 1, bodyDark); else { fill(x + 39, y - 5 + bob, 2, 1, WHITE); dot(x + 40, y - 5 + bob, 0x1f1a1a); }
    dot(x + 37, y - 8, WHITE, 0.8);
    if (isNight) glowAt(x + 40, y - 5, 6, 6, 0x80ffc0, 0.1);
  },
  /** A shining trapezohedron: a dark faceted crystal held in a brass cage, with a light inside that slowly swells and fades. */
  trapezohedron(x, y) {
    const crystal = 0x34305e, crystalLight = 0x5a54a0, crystalDark = 0x201c40, core = 0x80ffd8, brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a;
    fill(x + 35, y, 10, 1, SHADOW);
    // The brass foot
    fill(x + 36, y - 2, 8, 2, brass); fill(x + 36, y - 2, 8, 1, brassLight); fill(x + 37, y - 1, 6, 1, brassDark);
    // The crystal, row by row
    const rows = [[2, 12], [4, 11], [6, 10], [6, 9], [6, 8], [6, 7], [4, 6], [2, 5]];
    for (const [w, ry] of rows) { fill(x + 40 - w / 2, y - ry, w, 1, crystal); }
    for (const [w, ry] of rows) { dot(x + 40 - w / 2, y - ry, crystalLight); dot(x + 40 + w / 2 - 1, y - ry, crystalDark); }
    line(x + 40, y - 12, x + 40, y - 5, crystalDark, 0.6);
    // The light inside, swelling slowly
    const s = 0.5 + 0.5 * Math.sin(now * 0.8 + x * 0.17), big = s > 0.6;
    fill(x + 39, y - 9, 2, 2, core, 0.5 + s * 0.5); if (big) { dot(x + 38, y - 8, core, 0.6); dot(x + 41, y - 8, core, 0.6); dot(x + 40, y - 10, core, 0.6); dot(x + 40, y - 7, core, 0.6); }
    // The cage bars
    line(x + 36, y - 3, x + 37, y - 11, brassDark); line(x + 44, y - 3, x + 43, y - 11, brassDark); dot(x + 40, y - 13, brass); dot(x + 39, y - 12, brass, 0.0);
    glowAt(x + 40, y - 8, 6, 6, core, isNight ? 0.12 + s * 0.12 : 0.04);
    if (isNight) glowLate(() => { glowAt(x + 40, y - 8, 7, 7, core, 0.1 + s * 0.12); dot(x + 39, y - 9, 0xe8fff8, 0.9); });
  },
  /** A cream-and-pink conch shell on its side; every so often it whispers, and three tiny wavy ripple lines rise from its mouth. */
  whisperconch(x, y) {
    const shell = 0xe8cdb4, shellLight = 0xfae6d4, shellDark = 0xb8967c, mouth = 0xf09a9a, pale = 0xcfffe8;
    fill(x + 34, y, 12, 1, SHADOW);
    // The whorled body and its spire to the left
    fill(x + 36, y - 6, 8, 6, shell); fill(x + 37, y - 7, 6, 1, shell); fill(x + 36, y - 6, 8, 1, shellLight); fill(x + 36, y - 1, 8, 1, shellDark);
    fill(x + 34, y - 4, 2, 3, shell); dot(x + 33, y - 3, shellDark); fill(x + 35, y - 5, 1, 1, shellLight);
    for (const lx of [37, 39, 41]) { fill(x + lx, y - 6, 1, 5, shellDark, 0.55); }
    dot(x + 38, y - 6, shellLight); dot(x + 40, y - 7, shellLight);
    // The pink mouth, to the right
    fill(x + 44, y - 5, 2, 4, mouth); fill(x + 44, y - 5, 1, 4, 0xd47a80); dot(x + 45, y - 5, 0xffc8c8);
    // Whisper ripples rising from the mouth for 3 s every 12 s
    const t = (now + x * 0.31) % 12;
    if (t < 3) {
      for (let k = 0; k < 3; k++) {
        const age = t - k * 0.5; if (age < 0) continue;
        const ry = y - 8 - k * 2 - Math.floor(age), wob = (k + Math.floor(t * 3)) % 2 ? 1 : 0;
        fill(x + 42 + wob, ry, 3, 1, pale, Math.max(0.15, 0.75 - age * 0.2));
      }
    }
  },
  /** A hardware wallet in its little dock: a slim graphite stick with a tiny screen and two buttons, a steel seed plate leaning beside it; every 20 s its screen asks, a button is pressed and a green tick shows. */
  hardwallet(x, y) {
    const shell = 0x4a505e, shellLight = 0x6e7684, shellDark = 0x2a2e38, steel = 0x9aa0aa, steelLight = 0xc4c9d0, steelDark = 0x6a707a;
    fill(x + 34, y, 11, 1, SHADOW);
    // The seed plate, leaning, its punched rows
    fill(x + 35, y - 7, 5, 7, steel); fill(x + 35, y - 7, 5, 1, steelLight); fill(x + 35, y - 7, 1, 7, steelLight); fill(x + 39, y - 6, 1, 6, steelDark);
    for (const ry of [-5, -3, -1]) for (const rx of [36, 38]) dot(x + rx, y + ry, steelDark);
    // The dock and the stick standing in it
    fill(x + 40, y - 2, 5, 2, shellDark); fill(x + 40, y - 2, 5, 1, shellLight);
    fill(x + 41, y - 10, 3, 8, shell); fill(x + 41, y - 10, 1, 8, shellLight); fill(x + 43, y - 10, 1, 8, shellDark);
    // Its screen: asking, then a tick, 3 s in every 20
    const t = (now + x * 0.31) % 20, screen = 0x0c1418;
    fill(x + 41, y - 9, 3, 3, screen);
    if (t < 1.5) { dot(x + 42, y - 8, 0xd8e4f0, Math.floor(t * 4) % 2 ? 1 : 0.4); }
    else if (t < 3) { dot(x + 41, y - 8, 0x4ad87a); dot(x + 42, y - 7, 0x4ad87a); dot(x + 43, y - 9, 0x4ad87a); }
    else { dot(x + 42, y - 8, 0x7ab8e0); dot(x + 41, y - 7, 0x3a6a8a); dot(x + 43, y - 7, 0x3a6a8a); }
    // The buttons, one pressed in while it confirms
    const pressed = t > 1.3 && t < 1.6;
    dot(x + 41, y - 5, pressed ? shellDark : steelLight); dot(x + 43, y - 5, steelDark);
    dot(x + 41, y - 10, WHITE, 0.4);
    if (isNight && t < 3) glowLate(() => glowAt(x + 42, y - 8, 3, 3, t < 1.5 ? 0xa0c0e0 : 0x4ad87a, 0.18));
  },
  /** A gm mug: a matte charcoal mug with "gm" in mint letters; now and then a curl of steam rises from the coffee. */
  gmmug(x, y) {
    const body = 0x2a2e38, bodyLight = 0x3e4452, bodyDark = 0x1c1f27, ink = 0x5ae8a0, coffee = 0x3a2416;
    fill(x + 33, y, 13, 1, SHADOW);
    // The mug, its coffee and its handle on the left
    fill(x + 35, y - 8, 10, 8, body); fill(x + 35, y - 8, 1, 8, bodyLight); fill(x + 44, y - 8, 1, 8, bodyDark); fill(x + 35, y - 1, 10, 1, bodyDark);
    fill(x + 36, y - 8, 8, 1, coffee); dot(x + 36, y - 8, 0x5a3a24);
    fill(x + 33, y - 7, 2, 1, body); fill(x + 33, y - 6, 1, 3, bodyDark); fill(x + 33, y - 3, 2, 1, body);
    // "gm"
    const g = ['111', '1.1', '111', '..1', '11.'], m = ['1111.', '1.1.1', '1.1.1'];
    g.forEach((row, r) => [...row].forEach((on, k) => { if (on === '1') dot(x + 36 + k, y - 6 + r, ink); }));
    m.forEach((row, r) => [...row].forEach((on, k) => { if (on === '1') dot(x + 40 + k - 1, y - 6 + r, ink); }));
    // Steam, 4 s in every 16
    const t = (now + x * 0.31) % 16;
    if (t < 4) {
      const k = Math.floor(t * 2);
      dot(x + 38 + (k % 2), y - 10 - (k % 4), WHITE, 0.55 - (k % 4) * 0.1);
      dot(x + 41 - (k % 2), y - 9 - ((k + 2) % 4), WHITE, 0.45 - ((k + 2) % 4) * 0.08);
    }
  },
  /** A desk ticker: a little black LED bar on a brushed stand, a green line climbing and dipping across it and an arrow at its end that turns green or red as it does. */
  tickercube(x, y) {
    const shell = 0x22252d, shellLight = 0x3a3e48, steel = 0x8a909a, steelLight = 0xb8bdc6, screen = 0x081010;
    fill(x + 33, y, 13, 1, SHADOW);
    // The stand and the bar
    fill(x + 38, y - 2, 3, 2, steel); fill(x + 38, y - 2, 1, 2, steelLight); fill(x + 36, y - 1, 7, 1, steel);
    fill(x + 33, y - 9, 13, 7, shell); fill(x + 33, y - 9, 13, 1, shellLight); fill(x + 33, y - 9, 1, 7, shellLight);
    fill(x + 34, y - 8, 11, 5, screen);
    // The line, a step every half second, and its arrow
    const step = Math.floor(now * 2 + x * 0.7);
    let last = 0, prev = 0;
    for (let k = 0; k < 8; k++) { const v = hash(step + k, 11) % 5; dot(x + 34 + k, y - 4 - v, 0x3ad87a, 0.85); prev = last; last = v; }
    const up = last >= prev, arrow = up ? 0x4ae88a : 0xf0504a;
    if (up) { dot(x + 43, y - 8, arrow); fill(x + 42, y - 7, 3, 1, arrow); fill(x + 43, y - 6, 1, 2, arrow); }
    else { fill(x + 43, y - 8, 1, 2, arrow); fill(x + 42, y - 6, 3, 1, arrow); dot(x + 43, y - 5, arrow); }
    if (isNight) glowLate(() => glowAt(x + 39, y - 6, 7, 4, up ? 0x3ad87a : 0xf0504a, 0.12));
  },
  /** A coin on a stand: a big orange coin in a clear acrylic holder on a walnut block; every 15 s it spins round once. */
  coinstand(x, y) {
    const wood = 0x6a4a32, woodLight = 0x8a6444, woodDark = 0x4a321e, acrylic = 0xdff0f6;
    fill(x + 34, y, 11, 1, SHADOW);
    // The block and the clear holder
    fill(x + 35, y - 3, 9, 3, wood); fill(x + 35, y - 3, 9, 1, woodLight); fill(x + 43, y - 2, 1, 2, woodDark);
    fill(x + 37, y - 4, 5, 1, acrylic, 0.6); fill(x + 37, y - 4, 1, 1, WHITE, 0.6);
    // The coin, edge-on for a moment while it spins (1.5 s every 15)
    const t = (now + x * 0.31) % 15, cx = x + 39, cy = y - 8;
    if (t < 1.5) {
      const w = [3, 1, 3, 2, 3, 1, 3][Math.floor(t * 4.6) % 7], face = 0xf7931a;
      fill(cx - w, cy - 3, w * 2 + 1, 7, face); fill(cx - w, cy - 3, 1, 7, shaded(face, 1.25)); fill(cx + w, cy - 3, 1, 7, shaded(face, 0.75));
    } else coinMark('orange', cx, cy, 3);
    fill(cx - 4, cy - 4, 9, 9, acrylic, 0.12); fill(cx - 4, cy - 4, 1, 9, WHITE, 0.25);
  },
  /** A moonshot rocket: a little white rocket with a green band on a grey launch pad by its gantry; every 40 s it lifts off on a green flame, vanishes upward and is back on the pad in a puff of smoke. */
  moonshot(x, y) {
    const hull = 0xf2f2ee, hullDark = 0xc8c8c4, band = 0x3ad87a, nose = 0xd8402a, pad = 0x6e727c, padLight = 0xa8adb6;
    fill(x + 34, y, 11, 1, SHADOW);
    // The pad and the gantry beside it
    fill(x + 36, y - 2, 8, 2, pad); fill(x + 36, y - 2, 8, 1, padLight);
    fill(x + 35, y - 12, 1, 10, 0x8a909a); for (const gy of [-11, -8, -5]) dot(x + 36, y + gy, 0x8a909a);
    // Its flight: up for 2 s, gone for 1, then back with a puff
    const t = (now + x * 0.31) % 40, lift = t < 2 ? Math.floor(t * t * 3) : 0, away = t >= 2 && t < 3;
    if (t < 2) {
      const flick = Math.floor(now * 12) % 2;
      fill(x + 39, y - 2 - lift + 0, 2, 2 + flick, 0x9affc0); dot(x + 39 + flick, y - lift + 1, 0xffe08a);
      for (let k = 0; k < 3; k++) dot(x + 37 + k * 2, y - 2 - Math.floor(t * 2) + k % 2, WHITE, 0.5 - t * 0.2);
      if (isNight) glowLate(() => glowAt(x + 40, y - 2 - lift, 4, 4, 0x6affa0, 0.2));
    }
    if (t >= 3 && t < 4) for (let k = 0; k < 4; k++) dot(x + 36 + k * 2, y - 3 - (k % 2), WHITE, 0.6 * (4 - t));
    if (away || y - 12 - lift < y - 24) return;
    const ry = y - 3 - lift;
    // The rocket: fins, body with its band and port, the nose
    dot(x + 38, ry, hullDark); dot(x + 41, ry, hullDark); fill(x + 38, ry - 1, 4, 1, band);
    fill(x + 39, ry - 8, 2, 8, hull); fill(x + 40, ry - 8, 1, 8, hullDark);
    fill(x + 39, ry - 5, 2, 1, band); dot(x + 39, ry - 7, 0x4c8bf5);
    dot(x + 39, ry - 9, nose); dot(x + 40, ry - 9, shaded(nose, 0.8)); dot(x + 39, ry - 10, nose);
  },
  /** A pile of gold coins: three stacks of gold coins with loose ones at their feet and a coin of each colour on top; every 18 s a glint runs up the tallest stack. */
  coinpile(x, y) {
    const gold = 0xe8b830, goldLight = 0xffd860, goldDark = 0xa87a20;
    fill(x + 33, y, 13, 1, SHADOW);
    // The stacks: coins 5 wide, a dark line under each
    const stacks = [[34, 4], [39, 7], [43, 3]];
    for (const [sx, h] of stacks) for (let k = 0; k < h; k++) {
      const cy = y - 1 - k;
      fill(x + sx - 1, cy, 4, 1, k % 2 ? gold : shaded(gold, 0.92)); dot(x + sx - 1, cy, goldLight); dot(x + sx + 2, cy, goldDark);
    }
    // Loose coins at their feet, and marked coins on top
    dot(x + 37, y - 1, goldDark); dot(x + 36, y - 1, gold); dot(x + 42, y - 1, gold);
    coinMark('orange', x + 40, y - 10, 2);
    coinMark('tile', x + 35, y - 7, 2);
    coinMark('roundc', x + 44, y - 5, 2);
    // The glint, climbing the tall stack
    const t = (now + x * 0.31) % 18;
    if (t < 1.4) dot(x + 38 + (Math.floor(t * 5) % 2), y - 1 - Math.floor(t * 5), WHITE, 0.9);
  },
  /** A bull bobblehead: a big-headed brown bull in a green jersey on a round base, gold ring in its nose; every 9 s its head nods away for a moment. */
  bullbobble(x, y) {
    const hide = 0x7a4a2a, hideLight = 0x9a6438, hideDark = 0x52301a, horn = 0xf2ead2, jersey = 0x2a9a5a, base = 0x1e5a38;
    fill(x + 34, y, 11, 1, SHADOW);
    // The base and the body in its jersey
    fill(x + 35, y - 2, 9, 2, base); fill(x + 35, y - 2, 9, 1, shaded(base, 1.3));
    fill(x + 37, y - 6, 5, 4, jersey); fill(x + 37, y - 6, 1, 4, shaded(jersey, 1.25)); dot(x + 39, y - 5, WHITE, 0.8);
    // The head, nodding 3 s in every 9
    const t = (now + x * 0.31) % 9, nod = t < 3 ? [0, 1, 0, -1][Math.floor(t * 6) % 4] : 0, hx = x + 36, hy = y - 12 + Math.max(0, nod);
    fill(hx, hy, 7, 5, hide); fill(hx, hy, 7, 1, hideLight); fill(hx, hy, 1, 5, hideLight); fill(hx + 6, hy + 1, 1, 4, hideDark);
    fill(hx + 1, hy + 3, 5, 2, 0xc89a7a); dot(hx + 2, hy + 4, hideDark); dot(hx + 4, hy + 4, hideDark);
    dot(hx + 2, hy + 1, PALETTE.eye); dot(hx + 4, hy + 1, PALETTE.eye);
    dot(hx + 3, hy + 5, 0xe8c040);
    // The horns, curving up and out
    dot(hx - 1, hy, horn); dot(hx - 2, hy - 1, horn); dot(hx + 7, hy, horn); dot(hx + 8, hy - 1, horn);
  },
  /** Instant noodles: a white foam cup with a red band, its paper lid peeled back and a fork stuck in; steam curls up from it most of the time. */
  noodlecup(x, y) {
    const cup = 0xf2efe6, cupLight = 0xffffff, cupDark = 0xc8c2b4, band = 0xc83a2a, noodle = 0xe8c87a, steel = 0xa8adb6;
    fill(x + 35, y, 10, 1, SHADOW);
    // The cup, wider at its mouth, its band
    fill(x + 36, y - 8, 8, 1, cupDark); fill(x + 36, y - 7, 8, 3, cup); fill(x + 37, y - 4, 6, 4, cup);
    fill(x + 36, y - 7, 1, 3, cupLight); fill(x + 43, y - 7, 1, 3, cupDark); fill(x + 42, y - 4, 1, 4, cupDark);
    fill(x + 36, y - 5, 8, 2, band); dot(x + 36, y - 5, shaded(band, 1.25));
    // Noodles at the rim, the lid peeled back, the fork
    fill(x + 37, y - 8, 6, 1, noodle); dot(x + 38, y - 8, 0xf8e0a0); dot(x + 41, y - 8, 0xc8a050);
    fill(x + 42, y - 11, 3, 3, 0xd8d2c0); fill(x + 42, y - 11, 3, 1, 0xeae4d4); dot(x + 44, y - 9, band);
    line(x + 38, y - 8, x + 37, y - 12, steel); dot(x + 37, y - 13, steel); dot(x + 36, y - 13, steel);
    // Steam, all but 3 s in every 12
    const t = (now + x * 0.31) % 12;
    if (t < 9) {
      const k = Math.floor(t * 2);
      dot(x + 39 + (k % 2), y - 10 - (k % 4), WHITE, 0.5 - (k % 4) * 0.1);
      dot(x + 41 - (k % 2), y - 11 - ((k + 2) % 4), WHITE, 0.4 - ((k + 2) % 4) * 0.08);
    }
  },
  /** A survival kit: an olive tin box with its lid propped open, a can of beans, a candle stub and one orange coin wrapped up safe inside; a little torch clipped to it blinks now and then. */
  hodlkit(x, y) {
    const olive = 0x5a6a3a, oliveLight = 0x7a8a50, oliveDark = 0x3a4626, inside = 0x2a3220;
    fill(x + 33, y, 13, 1, SHADOW);
    // The lid, propped open behind
    fill(x + 34, y - 11, 11, 4, olive); fill(x + 34, y - 11, 11, 1, oliveLight); fill(x + 44, y - 11, 1, 4, oliveDark);
    fill(x + 37, y - 10, 5, 2, 0xe8e4d8); dot(x + 39, y - 10, 0xc83a2a); fill(x + 38, y - 9, 3, 1, 0xc83a2a);
    // The box and what's in it
    fill(x + 34, y - 7, 11, 1, inside);
    fill(x + 35, y - 9, 3, 3, 0xb8bdc6); fill(x + 35, y - 8, 3, 1, 0xd88a3a);
    fill(x + 39, y - 9, 1, 3, 0xf2ead2); dot(x + 39, y - 10, 0xffc860, candlelight(x * 0.2));
    coinMark('orange', x + 42, y - 8, 2);
    fill(x + 34, y - 6, 11, 6, olive); fill(x + 34, y - 6, 11, 1, oliveLight); fill(x + 34, y - 6, 1, 6, oliveLight); fill(x + 44, y - 6, 1, 6, oliveDark);
    fill(x + 38, y - 4, 3, 2, oliveDark); dot(x + 39, y - 4, 0xa8adb6);
    // The torch on its side, blinking for a moment every 14 s
    fill(x + 33, y - 3, 2, 3, 0x2a2e36); const on = (now + x * 0.31) % 14 < 0.8;
    dot(x + 33, y - 3, on ? 0xfff4c0 : 0x6a6e78);
    if (on && isNight) glowLate(() => glowAt(x + 33, y - 3, 3, 3, 0xfff0a0, 0.25));
  },
  /** An empty wallet: a worn brown leather wallet lying open, nothing in it but a receipt; every 15 s a little moth flutters out of it and away. */
  emptywallet(x, y) {
    const leather = 0x6a4228, leatherLight = 0x8a5a38, leatherDark = 0x4a2c18, lining = 0x3a2418;
    fill(x + 34, y, 12, 1, SHADOW);
    // The two halves, open flat, a stitch line round each
    fill(x + 34, y - 4, 11, 4, leather); fill(x + 34, y - 4, 11, 1, leatherLight); fill(x + 34, y - 1, 11, 1, leatherDark);
    fill(x + 39, y - 4, 1, 4, leatherDark);
    fill(x + 35, y - 3, 3, 1, lining); fill(x + 41, y - 3, 3, 1, lining);
    for (const sx of [35, 37, 41, 43]) dot(x + sx, y - 2, 0xc8a070, 0.6);
    // The receipt poking out
    fill(x + 41, y - 6, 3, 3, 0xeeeae0); dot(x + 42, y - 5, 0x9a968c);
    // The moth, out for 3 s every 15
    const t = (now + x * 0.31) % 15;
    if (t < 3) {
      const mx = x + 37 + Math.round(t * 2 + Math.sin(t * 6) * 1.5), my = y - 4 - Math.floor(t * 3), flap = Math.floor(now * 10) % 2;
      dot(mx, my, 0x8a8478); dot(mx - 1, my - flap, 0xb8b0a0, 0.9); dot(mx + 1, my - flap, 0xb8b0a0, 0.9);
    }
  },
  /** A tower of cans: empty energy-drink cans, lime and violet, stacked three, two, one into a little pyramid; every 30 s the top one wobbles. */
  cantower(x, y) {
    const lime = 0x6ad83a, violet = 0x8a4ad8, top = 0xc4c9d0;
    fill(x + 34, y, 11, 1, SHADOW);
    /** One can, 3 wide and 4 tall, its top at `cy`: its colour with a dark band, a steel lid and a glint. */
    const can = (cx, cy, colour) => {
      fill(cx, cy, 3, 4, colour); fill(cx, cy, 1, 4, shaded(colour, 1.3)); fill(cx + 2, cy, 1, 4, shaded(colour, 0.7));
      fill(cx, cy, 3, 1, top); fill(cx, cy + 2, 3, 1, shaded(colour, 0.55)); dot(cx + 1, cy + 1, WHITE, 0.5);
    };
    can(x + 35, y - 4, lime); can(x + 38, y - 4, violet); can(x + 41, y - 4, lime);
    can(x + 36, y - 8, violet); can(x + 40, y - 8, lime);
    const t = (now + x * 0.31) % 30, wob = t < 1.6 ? [0, 1, 0, -1][Math.floor(t * 6) % 4] : 0;
    can(x + 38 + wob, y - 12, violet);
  },
  /** A stack of pizza boxes: three greasy cardboard boxes piled a little askew, the top one's lid propped open on a last cold slice; now and then a fly buzzes round it. */
  pizzastack(x, y) {
    const card = 0xc8a070, cardLight = 0xe0bc88, cardDark = 0x9a7650, grease = 0xa88050;
    fill(x + 33, y, 13, 1, SHADOW);
    // The boxes, each a little off the one below
    for (const [dx, dy] of [[34, -3], [35, -6], [33, -9]]) {
      fill(x + dx, y + dy, 12, 3, card); fill(x + dx, y + dy, 12, 1, cardLight); fill(x + dx, y + dy + 2, 12, 1, cardDark);
      dot(x + dx + 11, y + dy + 1, cardDark);
    }
    dot(x + 37, y - 5, grease); dot(x + 41, y - 2, grease);
    // The open lid and the slice under it
    line(x + 33, y - 10, x + 41, y - 13, cardDark); line(x + 34, y - 10, x + 42, y - 13, card);
    fill(x + 36, y - 10, 5, 1, 0xe8a040); dot(x + 37, y - 10, 0xc83a2a); dot(x + 39, y - 10, 0xc83a2a); fill(x + 41, y - 10, 2, 1, 0xd8b070);
    // The fly, 3 s in every 20
    const t = (now + x * 0.31) % 20;
    if (t < 3) dot(x + 40 + Math.round(Math.cos(t * 7) * 4), y - 14 + Math.round(Math.sin(t * 9) * 2), 0x2a2a30);
  },
  /** A coin lamp: a big glowing orange coin on a short black stand, its light swelling and fading slowly; it lights the desk at night. */
  coinlamp(x, y) {
    const stand = 0x22252d, standLight = 0x3e434e;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 2, 7, 2, stand); fill(x + 36, y - 2, 7, 1, standLight); fill(x + 39, y - 4, 1, 2, stand);
    const pulse = 0.75 + 0.25 * Math.sin(now * 0.8 + x * 0.3);
    coinMark('orange', x + 39, y - 8, 3);
    glowAt(x + 39, y - 8, 6, 6, 0xffa040, 0.06 * pulse);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 7, 8, 7, 0xffa040, 0.2 * pulse); coinMark('orange', x + 39, y - 8, 3, 0.35 * pulse); });
  },
  /** A matchday ball: a white football with navy panels sitting on a little red cushion; every 14 s it hops twice, each hop lower, and settles. */
  matchball(x, y) {
    const hop = (now + x * 0.27) % 14, h = hop < 1.6 ? [0, 4, 6, 4, 0, 2, 3, 2, 0][Math.floor(hop * 5)] || 0 : 0;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 2, 7, 2, PSG_RED); fill(x + 36, y - 2, 7, 1, shaded(PSG_RED, 1.2)); fill(x + 36, y - 1, 7, 1, PSG_RED_DARK);
    if (h) fill(x + 37, y - 3, 5, 1, SHADOW);
    const by = y - 8 - (h >> 1);
    disc(x + 39.5, by + 2.5, 3, 0xeef0f4); fill(x + 38, by + 1, 3, 3, 0xeef0f4);
    dot(x + 39, by + 2, PSG_NAVY); dot(x + 40, by + 2, PSG_NAVY); dot(x + 39, by + 3, PSG_NAVY); dot(x + 37, by + 4, PSG_NAVY); dot(x + 42, by + 1, PSG_NAVY);
    fill(x + 37, by + 5, 5, 1, 0xb8bcc8); dot(x + 37, by + 1, WHITE, 0.9);
  },
  /** A scarf on a stand: a steel post with a crossbar and a navy scarf hung over it, a red band and two white pinstripes; the fringe flutters for a few seconds every 18 s. */
  psgscarf(x, y) {
    const steel = 0x6e727c, steelLight = 0xa8adb6, steelDark = 0x4a4e58;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 2, 7, 2, steelDark); fill(x + 36, y - 2, 7, 1, steel); fill(x + 39, y - 11, 1, 9, steel); dot(x + 39, y - 11, steelLight);
    fill(x + 35, y - 12, 9, 1, steel); fill(x + 35, y - 12, 9, 1, steelLight); dot(x + 35, y - 12, steel); dot(x + 43, y - 12, steelDark);
    // The scarf: two hanging lengths, striped across
    const flap = (now + x * 0.31) % 18 < 2.5 ? Math.floor(now * 5) % 2 : 0;
    for (const sx of [36, 40]) {
      const w = 4, top = y - 12, len = sx === 36 ? 9 : 8;
      fill(x + sx, top, w, len, PSG_NAVY); fill(x + sx, top, 1, len, PSG_NAVY_LIGHT); fill(x + sx + w - 1, top, 1, len, PSG_NAVY_DARK);
      fill(x + sx, top + 3, w, 2, PSG_RED); fill(x + sx, top + 2, w, 1, WHITE); fill(x + sx, top + 5, w, 1, WHITE);
      for (let k = 0; k < w; k += 2) { fill(x + sx + k + (flap && k ? 1 : 0), top + len, 1, 2, WHITE, 0.9); }
    }
    fill(x + 36, y - 12, 8, 1, PSG_NAVY_LIGHT);
  },
  /** A little Eiffel Tower: a navy and red desk model with a white crossbar, a lit beacon on the tip that blinks, and at night a sparkle of white lights that dances over it for a few seconds every 25 s. */
  eiffelmini(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    const cx = x + 39;
    fill(x + 36, y - 1, 7, 1, PSG_NAVY_DARK);
    // The legs spread at the base, the first deck, the waist, the second deck and the mast
    for (let k = 0; k < 4; k++) { dot(cx - 3 + k, y - 2 - k, PSG_NAVY); dot(cx + 3 - k, y - 2 - k, PSG_NAVY_DARK); }
    fill(cx - 3, y - 2, 1, 1, PSG_NAVY_LIGHT);
    fill(cx - 2, y - 5, 5, 1, WHITE);
    fill(cx - 1, y - 6, 3, 2, PSG_NAVY); fill(cx - 1, y - 6, 1, 2, PSG_NAVY_LIGHT);
    fill(cx - 2, y - 8, 5, 1, PSG_RED); dot(cx - 2, y - 8, shaded(PSG_RED, 1.25));
    fill(cx - 1, y - 11, 3, 3, PSG_NAVY); fill(cx - 1, y - 11, 1, 3, PSG_NAVY_LIGHT);
    fill(cx, y - 12, 1, 1, PSG_NAVY_DARK); dot(cx, y - 13, PSG_RED_DARK);
    const beacon = Math.floor(now * 1.5 + x) % 3 === 0;
    dot(cx, y - 13, beacon ? 0xffe0d8 : PSG_RED);
    const glint = (now + x * 0.37) % 25;
    if (glint < 3) for (const [dx, dy] of [[-2, -2], [2, -3], [-1, -7], [1, -9], [0, -4], [-1, -10]]) if ((Math.floor(now * 8) + dx * 3 + dy) % 3 === 0) dot(cx + dx, y + dy, WHITE, 0.9);
    if (isNight) glowLate(() => { glowAt(cx, y - 6, 6, 8, 0xff6a50, 0.1); dot(cx, y - 13, 0xfff0e8); });
  },
  /** A mini trophy: a wooden base with a purple plate, a gold stem and a basketball cradled on top; a glint every 20 s. (brand nod: Lakers) */
  lakerstrophy(x, y) { nbaItem(NBA_TEAMS.lakers, 'trophy', x, y); },
  /** A basketball on a green cushion; it spins for two seconds every 16 s. (brand nod: Celtics) */
  celticsball(x, y) { nbaItem(NBA_TEAMS.celtics, 'ball', x, y); },
  /** A foam finger in blue with a yellow cuff; the tip wiggles now and then. (brand nod: Warriors) */
  warriorsfinger(x, y) { nbaItem(NBA_TEAMS.warriors, 'finger', x, y); },
  /** A high-top in blue with orange laces; a glint runs along the sole every 22 s. (brand nod: Knicks) */
  knickssneaker(x, y) { nbaItem(NBA_TEAMS.knicks, 'sneaker', x, y); },
  /** A faceless bobblehead in the red home shirt; its head nods for two seconds every 12 s. (brand nod: Bulls) */
  bullsbobble(x, y) { nbaItem(NBA_TEAMS.bulls, 'bobble', x, y); },
  /** A felt pennant in red with a orange stripe on a wooden stick; the tip flutters every 14 s. (brand nod: Heat) */
  heatpennant(x, y) { nbaItem(NBA_TEAMS.heat, 'pennant', x, y); },
  /** A mini trophy: a wooden base with a black plate, a gold stem and a basketball cradled on top; a glint every 20 s. (brand nod: Spurs) */
  spurstrophy(x, y) { nbaItem(NBA_TEAMS.spurs, 'trophy', x, y); },
  /** A foam finger in blue with a orange cuff; the tip wiggles now and then. (brand nod: Thunder) */
  thunderfinger(x, y) { nbaItem(NBA_TEAMS.thunder, 'finger', x, y); },
  /** A faceless bobblehead in the navy home shirt; its head nods for two seconds every 12 s. (brand nod: Timberwolves) */
  wolvesbobble(x, y) { nbaItem(NBA_TEAMS.wolves, 'bobble', x, y); },
  /** A felt pennant in navy with a gold stripe on a wooden stick; the tip flutters every 14 s. (brand nod: Nuggets) */
  nuggetspennant(x, y) { nbaItem(NBA_TEAMS.nuggets, 'pennant', x, y); },
  /** A high-top in red with volt laces; a glint runs along the sole every 22 s. (brand nod: Hawks) */
  hawkssneaker(x, y) { nbaItem(NBA_TEAMS.hawks, 'sneaker', x, y); },
  /** A basketball on a black cushion; it spins for two seconds every 16 s. (brand nod: Nets) */
  netsball(x, y) { nbaItem(NBA_TEAMS.nets, 'ball', x, y); },
  /** A faceless bobblehead in the teal home shirt; its head nods for two seconds every 12 s. (brand nod: Hornets) */
  hornetsbobble(x, y) { nbaItem(NBA_TEAMS.hornets, 'bobble', x, y); },
  /** A mini trophy: a wooden base with a wine plate, a gold stem and a basketball cradled on top; a glint every 20 s. (brand nod: Cavaliers) */
  cavstrophy(x, y) { nbaItem(NBA_TEAMS.cavs, 'trophy', x, y); },
  /** A foam finger in blue with a silver cuff; the tip wiggles now and then. (brand nod: Mavericks) */
  mavsfinger(x, y) { nbaItem(NBA_TEAMS.mavs, 'finger', x, y); },
  /** A felt pennant in blue with a red stripe on a wooden stick; the tip flutters every 14 s. (brand nod: Pistons) */
  pistonspennant(x, y) { nbaItem(NBA_TEAMS.pistons, 'pennant', x, y); },
  /** A high-top in red with silver laces; a glint runs along the sole every 22 s. (brand nod: Rockets) */
  rocketssneaker(x, y) { nbaItem(NBA_TEAMS.rockets, 'sneaker', x, y); },
  /** A basketball on a navy cushion; it spins for two seconds every 16 s. (brand nod: Pacers) */
  pacersball(x, y) { nbaItem(NBA_TEAMS.pacers, 'ball', x, y); },
  /** A faceless bobblehead in the red home shirt; its head nods for two seconds every 12 s. (brand nod: Clippers) */
  clippersbobble(x, y) { nbaItem(NBA_TEAMS.clippers, 'bobble', x, y); },
  /** A foam finger in navy with a blue cuff; the tip wiggles now and then. (brand nod: Grizzlies) */
  grizzliesfinger(x, y) { nbaItem(NBA_TEAMS.grizzlies, 'finger', x, y); },
  /** A mini trophy: a wooden base with a green plate, a gold stem and a basketball cradled on top; a glint every 20 s. (brand nod: Bucks) */
  buckstrophy(x, y) { nbaItem(NBA_TEAMS.bucks, 'trophy', x, y); },
  /** A felt pennant in navy with a red stripe on a wooden stick; the tip flutters every 14 s. (brand nod: Pelicans) */
  pelicanspennant(x, y) { nbaItem(NBA_TEAMS.pelicans, 'pennant', x, y); },
  /** A high-top in blue with black laces; a glint runs along the sole every 22 s. (brand nod: Magic) */
  magicsneaker(x, y) { nbaItem(NBA_TEAMS.magic, 'sneaker', x, y); },
  /** A faceless bobblehead in the blue home shirt; its head nods for two seconds every 12 s. (brand nod: 76ers) */
  sixersbobble(x, y) { nbaItem(NBA_TEAMS.sixers, 'bobble', x, y); },
  /** A basketball on a purple cushion; it spins for two seconds every 16 s. (brand nod: Suns) */
  sunsball(x, y) { nbaItem(NBA_TEAMS.suns, 'ball', x, y); },
  /** A foam finger in red with a black cuff; the tip wiggles now and then. (brand nod: Trail Blazers) */
  blazersfinger(x, y) { nbaItem(NBA_TEAMS.blazers, 'finger', x, y); },
  /** A mini trophy: a wooden base with a purple plate, a gold stem and a basketball cradled on top; a glint every 20 s. (brand nod: Kings) */
  kingstrophy(x, y) { nbaItem(NBA_TEAMS.kings, 'trophy', x, y); },
  /** A felt pennant in red with a black stripe on a wooden stick; the tip flutters every 14 s. (brand nod: Raptors) */
  raptorspennant(x, y) { nbaItem(NBA_TEAMS.raptors, 'pennant', x, y); },
  /** A high-top in navy with yellow laces; a glint runs along the sole every 22 s. (brand nod: Jazz) */
  jazzsneaker(x, y) { nbaItem(NBA_TEAMS.jazz, 'sneaker', x, y); },
  /** A basketball on a navy cushion; it spins for two seconds every 16 s. (brand nod: Wizards) */
  wizardsball(x, y) { nbaItem(NBA_TEAMS.wizards, 'ball', x, y); },
  /** The Tesseract (brand nod: Marvel): a glowing blue cube with a brighter cube floating inside, on a small steel cradle; it pulses gently, flares with light every 18 s, and glows through the dark at night. */
  marvel1tesseract(x, y) {
    const pulse = 0.75 + 0.25 * Math.sin(now * 2 + x);
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, M1_STEEL); fill(x + 35, y - 2, 9, 1, M1_SILVER); fill(x + 36, y - 3, 1, 1, M1_STEEL); fill(x + 42, y - 3, 1, 1, M1_STEEL_DARK);
    fill(x + 36, y - 10, 7, 7, M1_CUBE, 0.55 * pulse + 0.2); fill(x + 36, y - 10, 7, 1, 0xbff4ff); fill(x + 36, y - 10, 1, 7, 0xbff4ff, 0.8);
    fill(x + 42, y - 9, 1, 6, 0x2a8ac8); fill(x + 37, y - 4, 6, 1, 0x2a8ac8);
    fill(x + 38, y - 8, 3, 3, 0xdffcff, pulse); dot(x + 37, y - 9, WHITE, 0.8);
    glowAt(x + 39, y - 6, 6, 6, M1_CUBE, 0.08 * pulse);
    const f = (now + x * 0.31) % 18;
    if (f < 1.2) for (const [dx, dy] of [[-5, -6], [5, -6], [0, -12], [-4, -11], [4, -11]]) dot(x + 39 + dx, y + dy, WHITE, 1 - f * 0.7);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 7, 9, 8, M1_CUBE, 0.2 * pulse); fill(x + 38, y - 8, 3, 3, 0xdffcff); });
  },
  /** A mini Mjolnir: Thor's hammer standing on its head, handle up, its strap looped at the top and runes on its side; every 20 s little bolts crackle off it, glowing at night. */
  marvel1minimjolnir(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 5, 9, 5, 0x8a8e98); fill(x + 35, y - 5, 9, 1, 0xb8bcc6); fill(x + 35, y - 5, 1, 5, 0xa8acb6); fill(x + 43, y - 5, 1, 5, 0x5a5e68); fill(x + 35, y - 1, 9, 1, 0x5a5e68);
    dot(x + 37, y - 3, 0x5a5e68); dot(x + 39, y - 2, 0x5a5e68); dot(x + 41, y - 3, 0x5a5e68);
    fill(x + 39, y - 11, 1, 6, 0x6a4a2e); dot(x + 39, y - 10, 0x4a3020); dot(x + 39, y - 8, 0x4a3020); dot(x + 39, y - 12, 0x8a8e98);
    dot(x + 38, y - 13, 0x4a3020); dot(x + 40, y - 13, 0x4a3020); dot(x + 39, y - 14, 0x4a3020);
    const t = (now + x * 0.29) % 20;
    if (t < 1.4 && Math.floor(now * 10) % 2 === 0) {
      const seed = Math.floor(now * 10);
      m1Bolt(x + 35, y - 6, x + 32, y - 13, seed); m1Bolt(x + 43, y - 6, x + 46, y - 12, seed + 5);
      glowAt(x + 39, y - 5, 7, 5, 0xbfe8ff, 0.14);
      if (isNight) glowLate(() => glowAt(x + 39, y - 5, 8, 6, 0xbfe8ff, 0.2));
    }
  },
  /** A web-shooter on a little stand (brand nod: Marvel): a red wrist band with a silver nozzle and a blue cartridge; every 15 s it fires a web line up to the left that sticks with a splat, then fades. */
  marvel1webshooter(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 2, 8, 2, M1_STEEL_DARK); fill(x + 36, y - 2, 8, 1, M1_STEEL); fill(x + 39, y - 4, 2, 2, M1_STEEL);
    fill(x + 36, y - 8, 8, 4, M1_RED); fill(x + 36, y - 8, 8, 1, M1_RED_LIGHT); fill(x + 36, y - 5, 8, 1, M1_RED_DARK);
    fill(x + 38, y - 7, 3, 2, M1_BLUE); dot(x + 38, y - 7, M1_BLUE_LIGHT);
    fill(x + 35, y - 10, 3, 2, M1_SILVER); dot(x + 35, y - 10, WHITE); dot(x + 34, y - 10, M1_STEEL_DARK);
    const t = (now + x * 0.27) % 15;
    if (t < 3) {
      const reach = Math.min(1, t / 0.3), fade = t < 1.5 ? 1 : 1 - (t - 1.5) / 1.5;
      const ex = x + 34 - Math.round(reach * 6), ey = y - 10 - Math.round(reach * 6);
      line(x + 34, y - 10, ex, ey, M1_WEB, 0.9 * fade);
      if (reach >= 1) for (const [dx, dy] of [[-1, 0], [1, 0], [0, -1], [0, 1], [-1, -1], [1, 1]]) dot(ex + dx, ey + dy, M1_WEB, 0.8 * fade);
    }
  },
  /** A Spider-Man bobblehead (brand nod: Marvel): a small red and blue body on a round base, a big masked head with white eyes on a spring; every 9 s it bobs side to side for a moment. */
  marvel1spideybobble(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 2, 7, 2, M1_BLUE_DARK); fill(x + 36, y - 2, 7, 1, M1_BLUE);
    fill(x + 38, y - 6, 3, 4, M1_RED); fill(x + 38, y - 3, 3, 1, M1_BLUE); dot(x + 37, y - 5, M1_BLUE); dot(x + 41, y - 5, M1_BLUE);
    dot(x + 39, y - 5, 0x1a1a1f);
    const t = (now + x * 0.23) % 9, bob = t < 1.5 ? [0, 1, 0, -1][Math.floor(now * 8) % 4] : 0;
    m1Spidey(x + 36 + bob, y - 13, 7);
    dot(x + 37 + bob, y - 13, WHITE, 0.5);
  },
  /** A Hulk foam fist (brand nod: Marvel): a huge green fist on its forearm standing up from a purple cuff, knuckles and thumb picked out; every 20 s it smashes down on the desk, the desk shakes and a puff of dust flies. */
  marvel1hulkfist(x, y) {
    const t = (now + x * 0.31) % 20, smash = t < 0.9, down = smash ? [0, 2, 3, 1, 0, 1, 0][Math.floor(t * 8)] || 0 : 0;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 3, 7, 3, 0x6a3a8a); fill(x + 36, y - 3, 7, 1, 0x8a5aaa); fill(x + 42, y - 3, 1, 3, 0x4a2a6a);
    fill(x + 37, y - 6 + down, 5, 3 - down, M1_HULK); fill(x + 37, y - 6 + down, 1, 3 - down, M1_HULK_LIGHT);
    const fy = y - 13 + down;
    fill(x + 35, fy, 9, 7, M1_HULK); fill(x + 35, fy, 9, 1, M1_HULK_LIGHT); fill(x + 35, fy, 1, 7, M1_HULK_LIGHT); fill(x + 43, fy, 1, 7, M1_HULK_DARK); fill(x + 35, fy + 6, 9, 1, M1_HULK_DARK);
    for (const k of [37, 39, 41]) fill(x + k, fy + 1, 1, 3, M1_HULK_DARK);
    fill(x + 35, fy + 4, 5, 1, M1_HULK_DARK); dot(x + 36, fy + 3, M1_HULK_LIGHT);
    if (smash && t > 0.15 && t < 0.7) for (const [dx, dy] of [[-2, -1], [10, -1], [-3, -3], [11, -2]]) dot(x + 34 + dx + Math.sign(dx - 4) * Math.floor(t * 6), y + dy - Math.floor(t * 4), 0xd8d0c0, 0.8);
  },
  /** Ant-Man in a jar: a glass jar with a cork lid and a tiny red and black Ant-Man inside who shrinks to a speck and grows back every 12 s; an ant wanders over the lid. */
  marvel1antjar(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 11, 7, 11, 0xdff0f6, 0.4); fill(x + 36, y - 11, 1, 11, 0xdff0f6, 0.7); fill(x + 42, y - 11, 1, 11, 0xa8c0cc, 0.7); fill(x + 36, y - 1, 7, 1, 0xa8c0cc, 0.7);
    fill(x + 36, y - 13, 7, 2, 0xb88a5a); fill(x + 36, y - 13, 7, 1, 0xd8a870); fill(x + 42, y - 13, 1, 2, 0x8a6a3a);
    const t = (now + x * 0.21) % 12, small = t > 6 && t < 11;
    if (small) {
      dot(x + 39, y - 2, M1_RED); dot(x + 39, y - 3, 0x2a2a30);
      if (t < 6.4) dot(x + 39, y - 5, WHITE, 0.6);
    } else {
      fill(x + 38, y - 9, 3, 3, M1_RED); fill(x + 38, y - 9, 3, 1, 0x2a2a30); dot(x + 38, y - 8, M1_SILVER); dot(x + 40, y - 8, M1_SILVER);
      fill(x + 38, y - 6, 3, 3, M1_RED); dot(x + 39, y - 6, 0x2a2a30); fill(x + 38, y - 3, 1, 2, 0x2a2a30); fill(x + 40, y - 3, 1, 2, 0x2a2a30);
    }
    dot(x + 37, y - 9, WHITE, 0.7);
    const ax = x + 36 + Math.floor((now * 1.5 + x) % 7);
    dot(ax, y - 14, 0x2a1a14); dot(ax + 1, y - 14, 0x2a1a14, 0.7);
  },
  /** Shawarma, after the battle: a wrap in white paper folded open over the meat, on a small plate with a pickle; a wisp of steam rises from it now and then. */
  marvel1shawarma(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 34, y - 2, 11, 2, 0xe8e4dc); fill(x + 34, y - 2, 11, 1, WHITE); fill(x + 35, y - 1, 9, 1, 0xc8c4bc);
    // The wrap: paper round its foot, the bread and the filling at the top
    fill(x + 37, y - 10, 5, 8, 0xd8a868); fill(x + 37, y - 10, 1, 8, 0xe8c088);
    fill(x + 37, y - 11, 5, 2, 0x8a4a2a); dot(x + 38, y - 12, 0x4a8a3a); dot(x + 40, y - 12, 0xd84a3a); dot(x + 41, y - 11, 0xf0e8d0);
    fill(x + 36, y - 6, 7, 4, 0xf4f2ec); fill(x + 36, y - 6, 7, 1, WHITE); fill(x + 42, y - 6, 1, 4, 0xc8c4bc); line(x + 36, y - 6, x + 39, y - 4, 0xc8c4bc);
    fill(x + 43, y - 4, 2, 2, 0x6a9a3a); dot(x + 43, y - 4, 0x8aba5a);
    const t = (now * 0.5 + x * 0.17) % 4;
    if (t < 2) { dot(x + 39 + (t > 1 ? 1 : 0), y - 13 - Math.floor(t * 1.5), WHITE, 0.5 - t * 0.2); }
  },
  /** Cap's pocket compass: a worn brass compass on its chain, lying on the desk; every 16 s its lid springs open on a tiny sepia photo, the needle swinging, then it clicks shut. */
  marvel1compass(x, y) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a;
    fill(x + 34, y, 11, 1, SHADOW);
    // The chain coiled beside it
    for (const [dx, dy] of [[34, -1], [35, -2], [36, -1], [44, -1], [43, -2]]) dot(x + dx, y + dy, brassDark);
    const t = (now + x * 0.27) % 16, open = t < 4;
    disc(x + 39, y - 3, 3, brassDark); disc(x + 39, y - 3, 2.4, brass); dot(x + 37, y - 4, brassLight);
    fill(x + 39, y - 7, 1, 1, brassDark); dot(x + 39, y - 8, brassLight);
    if (open) {
      // The lid up behind, its photo; the dial with its swinging needle
      disc(x + 39, y - 10, 3, brassDark); disc(x + 39, y - 10, 2.2, 0xc8a878); fill(x + 38, y - 11, 3, 3, 0x8a6a4a); dot(x + 39, y - 11, 0xe8d0a8); dot(x + 39, y - 9, 0x5a3a2a);
      disc(x + 39, y - 3, 1.6, 0xf0e8d0);
      const a = Math.sin(t * 5) * (1 - t / 4) * 1.4 - Math.PI / 2;
      dot(x + 39 + Math.round(Math.cos(a) * 1.4), y - 3 + Math.round(Math.sin(a) * 1.4), M1_RED); dot(x + 39, y - 3, 0x2a2a30);
    } else {
      dot(x + 39, y - 3, brassDark); dot(x + 40, y - 2, brassDark, 0.6);
    }
  },
  /** Star-Lord's walkman (brand nod: Marvel): a blue and silver cassette player with orange foam headphones draped over it; every 14 s the tape turns, the headphones bob to the beat and two notes float up. */
  marvel2walkman(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 9, 8, 9, 0x3a6ab8); fill(x + 36, y - 9, 1, 9, 0x5a8ad8); fill(x + 43, y - 9, 1, 9, 0x24447a);
    fill(x + 37, y - 7, 6, 4, M2_SILVER); fill(x + 38, y - 6, 4, 2, 0x2a2c34);
    const playing = (now + x * 0.27) % 14 < 5, spin = playing ? Math.floor(now * 6) % 2 : 0;
    dot(x + 38 + spin, y - 6, WHITE, 0.8); dot(x + 41 - spin, y - 5, WHITE, 0.8);
    for (let k = 0; k < 3; k++) dot(x + 37 + k * 2, y - 2, k === 0 ? 0xd84a3a : 0x8a8e98);
    // The headphones: band over the top, pads either side
    const bob = playing ? Math.floor(now * 4) % 2 : 0;
    line(x + 35, y - 10 - bob, x + 39, y - 13 - bob, 0x8a8e98); line(x + 39, y - 13 - bob, x + 44, y - 10 - bob, 0x8a8e98);
    fill(x + 34, y - 10 - bob, 2, 3, M2_ORANGE); fill(x + 43, y - 10 - bob, 2, 3, M2_ORANGE); dot(x + 34, y - 10 - bob, 0xf8a060);
    if (playing) {
      const q = ((now + x * 0.27) % 14) / 5;
      dot(x + 46, y - 12 - Math.floor(q * 4), WHITE, 0.8 - q * 0.6); dot(x + 46, y - 13 - Math.floor(q * 4), WHITE, 0.8 - q * 0.6);
      dot(x + 32, y - 9 - Math.floor(q * 5), WHITE, 0.7 - q * 0.5);
    }
  },
  /** The Eye of Agamotto (brand nod: Marvel): a gold amulet like a closed eye, hung on a little dark stand; every 18 s its lids part on the green glow of the Time Stone, and it turns a ring of green runes before closing. */
  marvel2eye(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 2, 7, 2, 0x3a2416); fill(x + 36, y - 2, 7, 1, 0x5a3a24); fill(x + 39, y - 13, 1, 11, 0x3a2416); fill(x + 37, y - 13, 5, 1, 0x5a3a24);
    const cx = x + 39, cy = y - 8, t = (now + x * 0.31) % 18, open = t < 3;
    disc(cx, cy, 3.3, M2_GOLD_DARK); disc(cx, cy, 2.6, M2_GOLD); dot(cx - 2, cy - 2, M2_GOLD_LIGHT);
    fill(cx - 4, cy, 1, 1, M2_GOLD); fill(cx + 4, cy, 1, 1, M2_GOLD);
    if (open) {
      fill(cx - 2, cy - 1, 5, 3, 0x1a3a22); fill(cx - 1, cy - 1, 3, 3, 0x3ae87a); dot(cx, cy, 0xc8ffd8);
      for (let k = 0; k < 6; k++) { const a = now * 2 + k * 1.05; dot(cx + Math.round(Math.cos(a) * 6), cy + Math.round(Math.sin(a) * 5), 0x3ae87a, 0.7); }
      glowAt(cx, cy, 5, 4, 0x3ae87a, 0.12);
      if (isNight) glowLate(() => { glowAt(cx, cy, 7, 6, 0x3ae87a, 0.22); dot(cx, cy, 0xc8ffd8); });
    } else {
      fill(cx - 2, cy, 5, 1, M2_GOLD_DARK); dot(cx, cy - 1, M2_GOLD_DARK, 0.6);
    }
  },
  /** The Orb (brand nod: Marvel): a scuffed silver sphere carved with rings, on a three-footed stand; every 22 s it splits open along its middle and a purple light spills out, then it closes again. */
  marvel2orb(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    line(x + 39, y - 3, x + 36, y - 1, 0x4a4e58); line(x + 39, y - 3, x + 42, y - 1, 0x4a4e58); dot(x + 39, y - 2, 0x4a4e58);
    const t = (now + x * 0.23) % 22, gap = t < 3 ? (t < 0.4 || t > 2.6 ? 1 : 2) : 0;
    const cx = x + 39, cy = y - 7;
    disc(cx, cy + gap, 3.4, 0x8a8e98); disc(cx, cy - gap, 3.4, 0xa8acb6);
    if (gap) { fill(cx - 3, cy - gap + 1, 7, gap * 2 - 1, M2_PURPLE); fill(cx - 2, cy, 5, 1, M2_VIBE); glowAt(cx, cy, 6, 3, M2_VIBE, 0.2); }
    else { fill(cx - 3, cy, 7, 1, 0x6a6e78); }
    ring(cx, cy, 2, 0x6e727c, 0.6); dot(cx - 2, cy - 2 - gap, WHITE, 0.7);
    if (gap && isNight) glowLate(() => glowAt(cx, cy, 8, 4, M2_VIBE, 0.25));
  },
  /** Loki's scepter (brand nod: Marvel): a gold and silver staff in a little stand, a curved blade at its head cradling a blue gem that pulses; every 16 s the gem flares and lights the desk. */
  marvel2scepter(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 2, 5, 2, 0x2a2c34); fill(x + 37, y - 2, 5, 1, 0x4a4e58);
    fill(x + 39, y - 12, 1, 10, M2_GOLD); dot(x + 39, y - 9, M2_SILVER); dot(x + 39, y - 6, M2_SILVER);
    // The head: two curved blades, the gem between them
    line(x + 39, y - 12, x + 36, y - 15, M2_SILVER); line(x + 36, y - 15, x + 37, y - 17, M2_SILVER);
    line(x + 40, y - 12, x + 42, y - 15, M2_GOLD); line(x + 42, y - 15, x + 42, y - 18, M2_GOLD); dot(x + 41, y - 13, M2_GOLD_DARK);
    const pulse = 0.6 + 0.4 * Math.sin(now * 2.2 + x), flare = (now + x * 0.29) % 16 < 1.2;
    fill(x + 38, y - 15, 3, 3, 0x3a8aff); dot(x + 39, y - 14, 0xbfe0ff, pulse);
    glowAt(x + 39, y - 14, 4, 4, 0x6ab0ff, (flare ? 0.25 : 0.06) * pulse);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 14, 6, 5, 0x6ab0ff, flare ? 0.35 : 0.15); dot(x + 39, y - 14, 0xdff0ff); });
  },
  /** A Black Panther mask on a stand (brand nod: Marvel): the black mask with its pointed ears, silver lines and white eye slits on a small plinth, a silver claw necklace below; every 16 s a ripple of violet kinetic light runs over it. */
  marvel2pantherbust(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, 0x2a2430); fill(x + 35, y - 2, 9, 1, 0x4a4250);
    fill(x + 37, y - 4, 5, 2, M2_PANTHER); for (let k = 0; k < 5; k++) dot(x + 37 + k, y - 4, M2_SILVER);
    fill(x + 36, y - 12, 7, 8, M2_PANTHER); dot(x + 36, y - 13, M2_PANTHER); dot(x + 42, y - 13, M2_PANTHER); fill(x + 37, y - 13, 5, 1, M2_PANTHER);
    const t = (now + x * 0.27) % 16, ripple = t < 1.6, row = Math.floor(t * 6);
    const c = ripple ? M2_VIBE : 0x8a8e98;
    line(x + 39, y - 12, x + 39, y - 6, c, 0.8); dot(x + 37, y - 11, c, 0.7); dot(x + 41, y - 11, c, 0.7);
    fill(x + 37, y - 9, 2, 1, WHITE); fill(x + 40, y - 9, 2, 1, WHITE);
    dot(x + 36, y - 12, 0x3a3a48);
    if (ripple && row < 9) fill(x + 36, y - 13 + row, 7, 1, M2_VIBE, 0.45);
    if (isNight) glowLate(() => { fill(x + 37, y - 9, 2, 1, WHITE); fill(x + 40, y - 9, 2, 1, WHITE); if (ripple) glowAt(x + 39, y - 8, 5, 6, M2_VIBE, 0.2); });
  },
  /** The Cloak of Levitation (brand nod: Marvel): a red cloak with its high collar and gold clasps hung on a little stand; every 12 s it stirs by itself, lifting a corner and waving it, then hangs still. */
  marvel2cloak(x, y) {
    const red = 0xb02a2a, redLight = 0xd84a42, redDark = 0x7a1a1a;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 1, 5, 1, 0x3a2416); fill(x + 39, y - 13, 1, 12, 0x5a3a24);
    const t = (now + x * 0.3) % 12, lift = t < 2.5 ? [0, 1, 2, 3, 2, 3, 2, 1, 0][Math.floor(t * 3.4)] || 0 : 0;
    // The collar, the shoulders and the hanging folds
    fill(x + 36, y - 14, 7, 2, red); fill(x + 36, y - 14, 1, 2, redLight); fill(x + 42, y - 14, 1, 2, redDark);
    fill(x + 35, y - 12, 9, 2, red); fill(x + 35, y - 12, 9, 1, redLight);
    fill(x + 35, y - 10, 9, 7, red); fill(x + 35, y - 10, 1, 7, redLight); for (const fx of [37, 40]) fill(x + fx, y - 9, 1, 6, redDark);
    fill(x + 35, y - 3, 6, 1, redDark); fill(x + 41, y - 3 - lift, 3, 1, redDark); if (lift) { fill(x + 43, y - 4 - lift, 2, lift, red); dot(x + 44, y - 4 - lift, redLight); }
    dot(x + 37, y - 12, M2_GOLD); dot(x + 41, y - 12, M2_GOLD); fill(x + 38, y - 11, 3, 1, M2_GOLD_DARK);
  },
  /** Kimoyo beads (brand nod: Marvel): a bracelet of dark beads lying in a ring, each marked with a violet rune; every 10 s one bead lights and throws a tiny violet hologram up above it, a call coming in. */
  marvel2beads(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    const pts = [[35, -2], [37, -3], [39, -3], [41, -3], [43, -2], [42, -1], [40, -1], [38, -1], [36, -1]];
    const t = (now + x * 0.31) % 10, lit = t < 3 ? Math.floor((now + x) % 9) : -1;
    pts.forEach(([dx, dy], i) => { fill(x + dx, y + dy, 2, 1, 0x2a2430); dot(x + dx, y + dy, i === lit ? M2_VIBE : 0x5a4a78); });
    if (lit >= 0) {
      const [dx, dy] = pts[lit], hx = x + dx, rise = Math.min(6, Math.floor(t * 8));
      line(hx, y + dy - 1, hx, y + dy - rise, M2_VIBE, 0.5);
      if (rise >= 6) { fill(hx - 2, y + dy - 11, 5, 4, M2_VIBE, 0.3); dot(hx - 1, y + dy - 10, WHITE, 0.6); dot(hx + 1, y + dy - 10, WHITE, 0.6); fill(hx - 1, y + dy - 8, 3, 1, WHITE, 0.4); }
      if (isNight) glowLate(() => glowAt(hx, y + dy - 6, 4, 6, M2_VIBE, 0.2));
    }
  },
  /** A TVA desk clock (brand nod: Marvel): a chunky orange retro clock on a beige base with a cartoon face, big eyes and a grin, telling the real time; every 15 s she gives a wink. */
  marvel2tvaclock(x, y) {
    const orange = 0xe88a2a, orangeLight = 0xf8b05a, orangeDark = 0xa85a1a;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, 0xd8c8a0); fill(x + 35, y - 2, 9, 1, 0xf0e4c4);
    disc(x + 39, y - 8, 5, orange); disc(x + 38, y - 9, 2, orangeLight, 0.6); ring(x + 39, y - 8, 5, orangeDark, 0.5);
    disc(x + 39, y - 8, 3.4, 0xfff0d0);
    const wink = (now + x * 0.29) % 15 < 0.6;
    dot(x + 38, y - 10, 0x2a1a10); if (wink) dot(x + 40, y - 10, 0x2a1a10, 0.5); else { dot(x + 40, y - 10, 0x2a1a10); dot(x + 40, y - 11, 0x2a1a10, 0.4); }
    fill(x + 38, y - 7, 3, 1, 0x8a2a1a); dot(x + 37, y - 8, 0x8a2a1a, 0.6); dot(x + 41, y - 8, 0x8a2a1a, 0.6);
    const d = new Date(), ma = (d.getMinutes() / 60) * Math.PI * 2 - Math.PI / 2;
    dot(x + 39 + Math.round(Math.cos(ma) * 3), y - 8 + Math.round(Math.sin(ma) * 3), orangeDark);
    fill(x + 35, y - 13, 2, 2, orangeDark); fill(x + 42, y - 13, 2, 2, orangeDark);
  },
  /** An arc reactor desk lamp: a round reactor on a short steel stem and a heavy base, its blue coils and white core pulsing softly and shining through the dark at night. */
  ironmanreactor(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, IM_STEEL_DARK); fill(x + 35, y - 2, 9, 1, IM_STEEL); fill(x + 39, y - 4, 1, 2, IM_STEEL);
    imReactor(x + 39, y - 8, 4, 0.75 + 0.25 * Math.sin(now * 1.6 + x));
  },
  /** A mini Iron Man helmet (brand nod: Marvel): the red and gold helmet on a little black plinth; every 14 s its faceplate lifts open and shuts again, and its eyes glow at night. */
  ironmanhelmet(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, IM_GRAPHITE); fill(x + 35, y - 2, 9, 1, 0x4a4e58);
    const t = (now + x * 0.27) % 14, up = t < 2.5 ? (t < 0.3 || t > 2.2 ? 1 : 3) : 0;
    fill(x + 36, y - 11, 7, 9, IM_RED); fill(x + 37, y - 12, 5, 1, IM_RED); fill(x + 36, y - 11, 1, 9, IM_RED_LIGHT); fill(x + 42, y - 11, 1, 9, IM_RED_DARK);
    if (up) { fill(x + 37, y - 9, 5, 5, 0xc89070); dot(x + 38, y - 7, PALETTE.eye); dot(x + 40, y - 7, PALETTE.eye); fill(x + 37, y - 9 - up, 5, 2, IM_GOLD); }
    else imFace(x + 36, y - 11, [IM_RED, IM_GOLD]);
    if (!up && isNight) glowLate(() => { fill(x + 37, y - 8, 2, 1, IM_ARC_WHITE); fill(x + 40, y - 8, 2, 1, IM_ARC_WHITE); glowAt(x + 39, y - 8, 4, 2, IM_ARC, 0.15); });
  },
  /** A repulsor gauntlet (brand nod: Marvel): a red and gold armoured glove standing palm-out on its wrist; every 12 s the palm charges up, glows white-blue and lets off a soft pulse. */
  ironmangauntlet(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 4, 5, 4, IM_RED); fill(x + 37, y - 4, 5, 1, IM_GOLD); fill(x + 37, y - 4, 1, 4, IM_RED_LIGHT);
    fill(x + 36, y - 10, 7, 6, IM_GOLD); fill(x + 36, y - 10, 1, 6, IM_GOLD_LIGHT); fill(x + 42, y - 10, 1, 6, IM_GOLD_DARK);
    for (const fx of [36, 38, 40, 42]) { fill(x + fx, y - 13, 1, 3, IM_RED); dot(x + fx, y - 13, IM_RED_LIGHT); }
    fill(x + 34, y - 8, 2, 3, IM_RED); dot(x + 34, y - 8, IM_RED_LIGHT);
    const t = (now + x * 0.31) % 12, charge = t < 2 ? t / 2 : 0;
    disc(x + 39, y - 7, 1.4, charge ? IM_ARC_WHITE : IM_ARC); dot(x + 39, y - 7, WHITE, 0.6 + charge * 0.4);
    if (charge) { glowAt(x + 39, y - 7, 3 + Math.round(charge * 4), 3 + Math.round(charge * 3), IM_ARC, 0.1 + charge * 0.2); if (charge > 0.8) ring(x + 39, y - 7, 4, IM_ARC_WHITE, 0.6); }
    if (isNight) glowLate(() => { dot(x + 39, y - 7, IM_ARC_WHITE); glowAt(x + 39, y - 7, 4, 4, IM_ARC, 0.15 + charge * 0.2); });
  },
  /** A cheeseburger, the first thing he asked for: a sesame bun, a patty, melted cheese and lettuce on a paper wrapper; now and then a wisp of steam rises. */
  ironmanburger(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 34, y - 1, 11, 1, 0xf0ece0); fill(x + 35, y - 2, 9, 1, 0xe0dccc);
    fill(x + 36, y - 4, 7, 2, 0xd8a050); fill(x + 36, y - 4, 7, 1, 0xe8b868);
    fill(x + 35, y - 6, 9, 2, 0x6a3a22); fill(x + 35, y - 6, 9, 1, 0x8a4a2a);
    fill(x + 35, y - 7, 9, 1, 0xf2c030); dot(x + 37, y - 6, 0xf2c030); dot(x + 41, y - 6, 0xf2c030);
    fill(x + 35, y - 8, 9, 1, 0x5aa83e); dot(x + 34, y - 8, 0x5aa83e);
    fill(x + 36, y - 11, 7, 3, 0xd89040); fill(x + 37, y - 12, 5, 1, 0xd89040); fill(x + 36, y - 11, 7, 1, 0xe8a858);
    for (const [dx, dy] of [[38, -11], [40, -12], [41, -10]]) dot(x + dx, y + dy, 0xfff4d8);
    const t = (now * 0.5 + x * 0.17) % 4;
    if (t < 2) dot(x + 39 + (t > 1 ? 1 : 0), y - 14 - Math.floor(t * 1.5), WHITE, 0.5 - t * 0.2);
  },
  /** Tony's shades: a pair of red-tinted wraparound sunglasses folded on a little glass stand; every 17 s a glint slides across the lenses. */
  ironmanshades(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, 0xdff0f6, 0.4); fill(x + 35, y - 2, 9, 1, 0xdff0f6, 0.7);
    fill(x + 34, y - 6, 11, 1, 0x2a2a30); fill(x + 35, y - 5, 4, 3, 0xc8343a); fill(x + 40, y - 5, 4, 3, 0xc8343a); dot(x + 39, y - 5, 0x2a2a30);
    fill(x + 35, y - 5, 4, 1, 0xe85a5a); fill(x + 40, y - 5, 4, 1, 0xe85a5a);
    line(x + 34, y - 6, x + 36, y - 9, 0x2a2a30); line(x + 44, y - 6, x + 42, y - 9, 0x2a2a30);
    const g = (now + x * 0.23) % 17; if (g < 0.8) dot(x + 35 + Math.floor(g * 11), y - 4, WHITE, 0.9);
  },
  /** A glass Stark phone: a clear glass slab standing in a dark dock, faint blue icons glowing in the glass and a lit edge; every 13 s a little blue hologram pops up off it, a spinning globe, then folds away. */
  ironmanphone(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, IM_GRAPHITE); fill(x + 35, y - 2, 9, 1, 0x4a4e58);
    fill(x + 37, y - 11, 5, 9, 0xdff0f6, 0.45); fill(x + 37, y - 11, 1, 9, 0xdff0f6, 0.8); fill(x + 41, y - 11, 1, 9, IM_HOLO, 0.8); fill(x + 37, y - 11, 5, 1, 0xdff0f6, 0.8);
    for (const [dx, dy] of [[38, -9], [40, -9], [38, -7], [40, -7], [39, -4]]) dot(x + dx, y + dy, IM_HOLO, 0.8);
    dot(x + 38, y - 10, WHITE, 0.8);
    const t = (now + x * 0.29) % 13;
    if (t < 3) {
      const h = Math.min(3, Math.floor(t * 6));
      line(x + 39, y - 12, x + 39, y - 12 - h, IM_HOLO, 0.5);
      if (h >= 3) { ring(x + 39, y - 18, 3, IM_HOLO, 0.8); ellipseRing(x + 39, y - 18, Math.abs(Math.round(Math.cos(now * 3) * 3)), 3, IM_HOLO, 0.6); fill(x + 36, y - 18, 7, 1, IM_HOLO, 0.4); }
    }
    if (isNight) glowLate(() => { glowAt(x + 39, y - 7, 4, 5, IM_HOLO, t < 3 ? 0.25 : 0.12); for (const [dx, dy] of [[38, -9], [40, -9]]) dot(x + dx, y + dy, IM_HOLO); });
  },
  /** A box of scraps: a battered cardboard box full of salvaged parts, a wrench and a bent pipe sticking out of it; now and then a little spark jumps between the wires. */
  ironmanscraps(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 6, 9, 6, 0xb88a5a); fill(x + 35, y - 6, 9, 1, 0xd0a070); fill(x + 43, y - 6, 1, 6, 0x8a6a3a); fill(x + 35, y - 6, 1, 6, 0xc89a68);
    fill(x + 34, y - 7, 3, 1, 0xb88a5a); fill(x + 42, y - 7, 3, 1, 0xa87a4a);
    line(x + 37, y - 7, x + 36, y - 12, IM_STEEL); dot(x + 35, y - 12, IM_STEEL); dot(x + 37, y - 12, IM_STEEL);
    line(x + 41, y - 7, x + 43, y - 11, 0x8a6a4a); fill(x + 39, y - 9, 2, 2, 0x4a4e58); dot(x + 38, y - 8, 0xd8402a); dot(x + 40, y - 8, 0x3a8a3a);
    line(x + 38, y - 8, x + 41, y - 9, 0xd8402a, 0.8);
    if ((now + x * 0.31) % 9 < 0.4) { dot(x + 40, y - 10, 0xfff0a0); dot(x + 41, y - 11, WHITE, 0.8); if (isNight) glowLate(() => glowAt(x + 40, y - 10, 3, 3, 0xffd070, 0.25)); }
  },
  /** A giant-donut box: a pink-frosted donut with sprinkles sitting on its little white box, a nod to breakfast on a rooftop donut; a sprinkle twinkles now and then. */
  ironmandonut(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 4, 9, 4, 0xf4f0e6); fill(x + 35, y - 4, 9, 1, WHITE); fill(x + 43, y - 4, 1, 4, 0xc8c4b8); fill(x + 37, y - 3, 5, 1, 0xd84a6a, 0.7);
    fill(x + 35, y - 9, 9, 5, 0xd8a058); fill(x + 36, y - 10, 7, 1, 0xd8a058);
    fill(x + 35, y - 9, 9, 3, 0xf08ab0); fill(x + 36, y - 10, 7, 1, 0xf8a8c8); fill(x + 38, y - 8, 3, 1, 0x8a5a2a);
    const tw = Math.floor(now * 2 + x) % 5;
    [[36, -9, 0xf2d35b], [42, -9, 0x4c8bf5], [37, -7, 0x5ad86a], [41, -7, WHITE], [39, -10, 0xd8402a]].forEach(([dx, dy, c], i) => dot(x + dx, y + dy, i === tw ? WHITE : c));
  },
  /** A batarang on a stand (brand nod: DC): a black steel batarang held upright in a little clip on a stone base; every 15 s a glint slides along its blade. */
  batcavebatarang(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 3, 9, 3, BC_STONE); fill(x + 35, y - 3, 9, 1, BC_STONE_LIGHT); fill(x + 39, y - 5, 1, 2, BC_GREY);
    bcBat(x + 39, y - 9, 0x2a2c34, 1, true);
    for (const k of [-5, -4, -1, 1, 4, 5]) dot(x + 39 + k, y - 10, 0xa8acb6); dot(x + 34, y - 9, 0xa8acb6); dot(x + 44, y - 9, 0x6e727c);
    const g = (now + x * 0.29) % 15; if (g < 1) dot(x + 34 + Math.floor(g * 11), y - 8, WHITE);
  },
  /** A mini cowl (brand nod: DC): Batman's black cowl on a small bust, pointed ears, white eye slits and a hard jaw below; at night its eyes glint white now and then. */
  batcavecowl(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, BC_STONE); fill(x + 35, y - 2, 9, 1, BC_STONE_LIGHT);
    fill(x + 36, y - 4, 7, 2, BC_GREY); bcBat(x + 39, y - 3, BC_BLACK);
    fill(x + 36, y - 12, 7, 7, BC_BLACK); fill(x + 36, y - 12, 1, 7, 0x2e3038); dot(x + 36, y - 13, BC_BLACK); dot(x + 36, y - 14, BC_BLACK); dot(x + 42, y - 13, BC_BLACK); dot(x + 42, y - 14, BC_BLACK);
    fill(x + 37, y - 6, 5, 2, 0xc89070); fill(x + 38, y - 6, 3, 1, 0xa87050); fill(x + 37, y - 5, 5, 1, 0xb8805e);
    fill(x + 37, y - 9, 2, 1, WHITE); fill(x + 40, y - 9, 2, 1, WHITE);
    if (isNight && (now + x * 0.3) % 9 < 0.4) glowLate(() => { fill(x + 37, y - 9, 2, 1, WHITE); fill(x + 40, y - 9, 2, 1, WHITE); });
  },
  /** Alfred's tea tray: a silver tray with a white teapot, a cup and saucer and a shortbread biscuit; a thread of steam rises from the pot now and then. */
  batcavetea(x, y) {
    fill(x + 33, y, 13, 1, SHADOW);
    fill(x + 33, y - 1, 13, 1, 0xa8acb6); fill(x + 33, y - 2, 13, 1, 0xd8dce4); dot(x + 33, y - 2, WHITE);
    fill(x + 34, y - 7, 6, 5, WHITE); fill(x + 35, y - 8, 4, 1, WHITE); dot(x + 36, y - 9, 0xd8dce4); fill(x + 34, y - 7, 1, 5, 0xf8f8f8); fill(x + 39, y - 6, 1, 4, 0xd8d8dc);
    dot(x + 40, y - 6, WHITE); dot(x + 41, y - 5, WHITE); dot(x + 33, y - 5, WHITE);
    fill(x + 41, y - 4, 4, 2, WHITE); fill(x + 41, y - 4, 4, 1, 0x8a5a3a); fill(x + 40, y - 2, 6, 1, 0xf0f0f0);
    fill(x + 35, y - 3, 3, 1, 0xe8c070, 0.0);
    const t = (now * 0.5 + x * 0.17) % 4;
    if (t < 2) dot(x + 36 + (t > 1 ? 1 : 0), y - 10 - Math.floor(t * 1.5), WHITE, 0.5 - t * 0.2);
  },
  /** A utility belt (brand nod: DC): Batman's yellow belt coiled on the desk, its row of pouches and the bat on the buckle; a pouch flips open now and then and shuts again. */
  batcavebelt(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 34, y - 4, 11, 4, BC_YELLOW); fill(x + 34, y - 4, 11, 1, 0xf8e470); fill(x + 34, y - 1, 11, 1, BC_YELLOW_DARK);
    fill(x + 36, y - 3, 7, 2, 0x8a7410, 0.4);
    const open = Math.floor((now + x * 0.29) % 14) === 0;
    for (const px of [34, 37, 41, 43]) { fill(x + px, y - 6, 2, 3, BC_YELLOW_DARK); dot(x + px, y - 6, BC_YELLOW); }
    if (open) { fill(x + 41, y - 8, 2, 2, BC_YELLOW); dot(x + 42, y - 7, BC_GREY); }
    fill(x + 38, y - 7, 3, 3, 0x6e727c); fill(x + 38, y - 7, 3, 1, 0xa8acb6); dot(x + 39, y - 6, BC_BLACK);
  },
  /** The Bat-Phone (brand nod: DC): a red telephone under a glass dome on a black stand; every 20 s it rings, the receiver jumping in its cradle and a red light on the stand flashing. */
  batcavephone(x, y) {
    const red = 0xd8202a, redLight = 0xf04a4a, redDark = 0x8a1018;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 34, y - 2, 11, 2, BC_BLACK); fill(x + 34, y - 2, 11, 1, 0x2e3038);
    const ring_ = (now + x * 0.31) % 20 < 2.4, jump = ring_ ? Math.floor(now * 10) % 2 : 0;
    fill(x + 36, y - 6, 7, 4, red); fill(x + 36, y - 6, 7, 1, redLight); fill(x + 42, y - 6, 1, 4, redDark);
    disc(x + 39, y - 4, 1, redDark); dot(x + 39, y - 4, 0xeae6dc);
    fill(x + 35, y - 8 - jump, 9, 2, red); fill(x + 35, y - 8 - jump, 9, 1, redLight); fill(x + 35, y - 7 - jump, 2, 2, red); fill(x + 42, y - 7 - jump, 2, 2, red);
    fill(x + 35, y - 13, 9, 11, 0xdff0f6, 0.18); fill(x + 36, y - 14, 7, 1, 0xdff0f6, 0.5); fill(x + 35, y - 13, 1, 11, 0xdff0f6, 0.5); dot(x + 37, y - 12, WHITE, 0.8); dot(x + 39, y - 15, 0x9aa0aa);
    dot(x + 43, y - 1, ring_ && Math.floor(now * 6) % 2 ? 0xff3030 : 0x4a1010);
    if (ring_) { dot(x + 33, y - 9 - jump, BC_YELLOW, 0.8); dot(x + 46, y - 9 + jump, BC_YELLOW, 0.8); if (isNight) glowLate(() => glowAt(x + 43, y - 1, 3, 2, 0xff3030, 0.3)); }
  },
  /** A grapple gun (brand nod: DC): the black pistol-gripped launcher standing on a little stand; every 15 s it fires, a line whipping up with its hook, holds, and reels back in. */
  batcavegrapple(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, BC_STONE); fill(x + 35, y - 2, 9, 1, BC_STONE_LIGHT);
    fill(x + 38, y - 5, 2, 3, 0x2e3038); fill(x + 36, y - 8, 8, 3, BC_BLACK); fill(x + 36, y - 8, 8, 1, 0x3a3c46); fill(x + 43, y - 9, 2, 5, 0x3a3c46); dot(x + 44, y - 9, 0x6e727c);
    dot(x + 37, y - 7, BC_YELLOW);
    const t = (now + x * 0.27) % 15;
    if (t < 2.5) {
      const len = t < 0.4 ? Math.floor(t * 30) : t > 1.8 ? Math.max(0, 12 - Math.floor((t - 1.8) * 18)) : 12;
      line(x + 44, y - 10, x + 44 + Math.round(len * 0.4), y - 10 - len, 0x9aa0aa);
      const hx = x + 44 + Math.round(len * 0.4), hy = y - 10 - len; dot(hx, hy, 0xc8ccd4); dot(hx - 1, hy, 0xc8ccd4); dot(hx + 1, hy, 0xc8ccd4); dot(hx - 1, hy + 1, 0x6e727c); dot(hx + 1, hy + 1, 0x6e727c);
    } else { dot(x + 44, y - 10, 0xc8ccd4); }
  },
  /** Chattering teeth (brand nod: DC): a pair of wind-up joke teeth on little orange feet, white teeth in red gums and a key at the back; every 12 s they wind up and chatter, hopping along a pixel. */
  batcaveteeth(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    const t = (now + x * 0.29) % 12, go = t < 2.2, open = go && Math.floor(now * 10) % 2, hop = go && Math.floor(now * 5) % 2 ? 1 : 0;
    fill(x + 36, y - 1, 2, 1, 0xe8782e); fill(x + 41, y - 1, 2, 1, 0xe8782e);
    const by = y - 3 - hop;
    fill(x + 35, by - 1, 9, 2, 0xc8343a); fill(x + 36, by - 2, 7, 1, WHITE); for (const k of [37, 39, 41]) dot(x + k, by - 2, 0xd8d4cc);
    fill(x + 35, by - 4 - open * 2, 9, 2, 0xc8343a); fill(x + 36, by - 2 - open * 2, 7, 1, WHITE); for (const k of [38, 40]) dot(x + k, by - 2 - open * 2, 0xd8d4cc);
    if (open) fill(x + 36, by - 2, 7, 1, 0x4a1418, 0.0);
    fill(x + 44, by - 3, 1, 2, 0xc8a050); dot(x + 45, by - 4, 0xc8a050); dot(x + 45, by - 1, 0xc8a050);
  },
  /** The Penguin's umbrella (brand nod: DC): a black umbrella with purple trim and a hooked handle standing in a little stand; every 22 s it pops open into a canopy, then folds shut. */
  batcaveumbrella(x, y) {
    const purple = 0x6a3a8a;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 4, 5, 4, 0x3a3c46); fill(x + 37, y - 4, 5, 1, 0x5a5e68);
    const open = (now + x * 0.31) % 22 < 3;
    if (open) {
      fill(x + 39, y - 9, 1, 6, 0x2a2a30);
      for (let r = 0; r < 4; r++) fill(x + 39 - 2 - r * 1.5, y - 14 + r, 5 + r * 3, 1, BC_BLACK);
      for (let k = 0; k < 6; k++) dot(x + 33 + k * 2 + (k > 2 ? 1 : 0), y - 10, purple);
      dot(x + 39, y - 15, 0x9aa0aa);
    } else {
      fill(x + 38, y - 12, 3, 8, BC_BLACK); fill(x + 38, y - 12, 1, 8, 0x2e3038); dot(x + 39, y - 13, 0x9aa0aa); fill(x + 38, y - 8, 3, 1, purple);
      dot(x + 39, y - 4, 0x2a2a30);
    }
    fill(x + 39, y - 4, 1, 1, 0x6a4a2e); dot(x + 40, y - 2, 0x6a4a2e, 0.0);
  },
  /** A power battery (brand nod: DC): the Green Lantern's lantern, a green-glowing barrel in a dark frame with its handle on top; the light pulses, and every 16 s it flares as if a ring were being charged. */
  dcmixlanternbattery(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, 0x1a2a1e); fill(x + 35, y - 2, 9, 1, 0x2a4a32);
    const pulse = 0.6 + 0.4 * Math.sin(now * 2 + x), flare = (now + x * 0.29) % 16 < 1.4;
    fill(x + 36, y - 10, 7, 8, DC_GREEN_DARK); fill(x + 37, y - 9, 5, 6, DC_GREEN, 0.6 + 0.4 * pulse); fill(x + 38, y - 8, 3, 4, flare ? WHITE : DC_GREEN_LIGHT, 0.8);
    for (const fx of [36, 42]) fill(x + fx, y - 10, 1, 8, 0x1a2a1e);
    fill(x + 36, y - 11, 7, 1, 0x1a2a1e); fill(x + 37, y - 12, 5, 1, 0x2a4a32); line(x + 37, y - 12, x + 39, y - 14, 0x2a4a32); line(x + 41, y - 12, x + 39, y - 14, 0x2a4a32);
    glowAt(x + 39, y - 6, 6, 5, DC_GREEN, (flare ? 0.25 : 0.08) * pulse);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 6, 8, 7, DC_GREEN, flare ? 0.35 : 0.18); fill(x + 38, y - 8, 3, 4, DC_GREEN_LIGHT); });
  },
  /** Kryptonite in a lead box (brand nod: DC): a grey lead box on the desk; every 15 s its lid lifts and the green crystal inside blazes, washing the desk green, then the lid drops shut. */
  dcmixkryptonite(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 5, 9, 5, 0x5a5e66); fill(x + 35, y - 5, 9, 1, 0x7a7e86); fill(x + 43, y - 5, 1, 5, 0x3a3e46); fill(x + 35, y - 1, 9, 1, 0x3a3e46);
    const t = (now + x * 0.31) % 15, open = t < 3.5;
    if (open) {
      fill(x + 35, y - 12, 9, 2, 0x5a5e66); fill(x + 35, y - 12, 9, 1, 0x7a7e86); fill(x + 35, y - 10, 1, 5, 0x4a4e56);
      for (const [dx, dy, h] of [[37, -7, 3], [39, -9, 5], [41, -8, 4]]) { fill(x + dx, y + dy, 2, h, DC_KRYPTO); dot(x + dx, y + dy, 0xd0ffd8); }
      glowAt(x + 39, y - 7, 7, 5, DC_KRYPTO, 0.18);
      if (isNight) glowLate(() => { glowAt(x + 39, y - 7, 9, 6, DC_KRYPTO, 0.3); dot(x + 39, y - 9, 0xd0ffd8); });
    } else { fill(x + 35, y - 6, 9, 1, 0x7a7e86); fill(x + 38, y - 6, 3, 1, 0x4a4e56); dot(x + 39, y - 3, 0x3a3e46); }
  },
  /** Wonder Woman's tiara (brand nod: DC): the gold tiara with its red star resting on a small marble block; a glint runs over the star every 12 s. */
  dcmixtiara(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 4, 9, 4, DC_MARBLE); fill(x + 35, y - 4, 9, 1, WHITE); fill(x + 43, y - 4, 1, 4, DC_MARBLE_DARK);
    fill(x + 35, y - 6, 9, 2, DC_GOLD); fill(x + 35, y - 6, 9, 1, 0xf8d880); fill(x + 37, y - 8, 5, 2, DC_GOLD); fill(x + 38, y - 9, 3, 1, DC_GOLD);
    dot(x + 39, y - 8, DC_RED); dot(x + 38, y - 7, DC_RED); dot(x + 40, y - 7, DC_RED); dot(x + 39, y - 7, DC_RED_LIGHT); dot(x + 39, y - 9, DC_RED);
    if ((now + x * 0.27) % 12 < 0.6) { dot(x + 39, y - 8, WHITE); dot(x + 41, y - 9, WHITE, 0.7); }
  },
  /** Clark's glasses (brand nod: DC): a pair of black-rimmed glasses folded on top of a Daily Planet, its headline in thick black; every 20 s a glint crosses the lenses. */
  dcmixglasses(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 34, y - 3, 11, 3, 0xeae6dc); fill(x + 34, y - 3, 11, 1, WHITE); fill(x + 35, y - 2, 9, 1, 0x2a2a30); fill(x + 35, y - 1, 9, 1, 0x9a968c, 0.7);
    fill(x + 34, y - 5, 11, 2, 0xe0dccf); fill(x + 35, y - 5, 4, 1, 0x2a2a30); dot(x + 41, y - 5, 0x2a2a30, 0.5);
    fill(x + 35, y - 8, 4, 3, 0x1a1a20); fill(x + 40, y - 8, 4, 3, 0x1a1a20); fill(x + 36, y - 7, 2, 1, 0xa8c8e8, 0.7); fill(x + 41, y - 7, 2, 1, 0xa8c8e8, 0.7); fill(x + 39, y - 8, 1, 1, 0x1a1a20);
    const g = (now + x * 0.3) % 20; if (g < 0.6) dot(x + 36 + Math.floor(g * 12), y - 7, WHITE);
  },
  /** A Mother Box (brand nod: DC): a dark alien cube on the desk carved with glowing orange circuit lines; it hums, and every 18 s its lines race with light and it lets out a pulse. */
  dcmixmotherbox(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 8, 9, 8, 0x2a2a30); fill(x + 35, y - 8, 9, 1, 0x4a4a52); fill(x + 35, y - 8, 1, 8, 0x3a3a42); fill(x + 43, y - 7, 1, 7, 0x16161a);
    const race = (now + x * 0.29) % 18 < 1.6, step_ = Math.floor(now * 8) % 6;
    const c = 0xff8a3a, lines = [[37, -6, 5, 1], [37, -6, 1, 4], [41, -6, 1, 3], [37, -3, 3, 1], [39, -4, 1, 2]];
    lines.forEach(([dx, dy, w, h], i) => fill(x + dx, y + dy, w, h, race && i === step_ % 5 ? 0xfff0c0 : c, race ? 1 : 0.55 + 0.25 * Math.sin(now * 2 + i)));
    if (race) { ring(x + 39, y - 4, 6 + (step_ % 3), c, 0.4); glowAt(x + 39, y - 4, 6, 5, c, 0.15); }
    if (isNight) glowLate(() => glowAt(x + 39, y - 4, 6, 5, c, race ? 0.3 : 0.1));
  },
  /** Harley's mallet (brand nod: DC): a big red and black carnival mallet standing on its head, a diamond on the side; every 14 s it hops and thumps back down with a little "pow" star. */
  dcmixmallet(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    const t = (now + x * 0.27) % 14, hop = t < 0.8 ? [0, 2, 3, 2, 0, 1, 0][Math.floor(t * 8)] || 0 : 0;
    fill(x + 35, y - 6 - hop, 4, 6, DC_RED); fill(x + 39, y - 6 - hop, 4, 6, 0x1a1a1f); fill(x + 35, y - 6 - hop, 8, 1, 0xf05a50); fill(x + 35, y - 1 - hop, 8, 1, 0x5a0a0a);
    dot(x + 39, y - 4 - hop, WHITE); dot(x + 38, y - 3 - hop, WHITE); dot(x + 40, y - 3 - hop, WHITE); dot(x + 39, y - 2 - hop, WHITE);
    fill(x + 39, y - 13 - hop, 1, 7, 0x8a5a3a); dot(x + 39, y - 14 - hop, 0x1a1a1f);
    if (t > 0.6 && t < 1.1) for (const [dx, dy] of [[33, -2], [45, -2], [32, -4], [46, -4]]) dot(x + dx, y + dy, DC_YELLOW);
  },
  /** A Fortress crystal (brand nod: DC): a cluster of tall white-blue crystal shards from the Fortress of Solitude, standing in a little ice base; a cold light runs up through them every 13 s. */
  dcmixfortress(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, 0xc8e4f0); fill(x + 35, y - 2, 9, 1, WHITE);
    const shards = [[36, 6], [38, 11], [40, 8], [42, 5], [39, 13]];
    const t = (now + x * 0.31) % 13, run = t < 1.4 ? Math.floor(t * 10) : -1;
    for (const [dx, h] of shards) { fill(x + dx, y - 2 - h, 2, h, 0xdff4ff, 0.75); fill(x + dx, y - 2 - h, 1, h, WHITE, 0.85); dot(x + dx, y - 3 - h, WHITE); if (run >= 0 && run < h) dot(x + dx + 1, y - 3 - run, 0x8ad8ff); }
    glowAt(x + 39, y - 8, 5, 6, 0xbfe8ff, 0.06);
    if (isNight) glowLate(() => glowAt(x + 39, y - 8, 6, 8, 0xbfe8ff, 0.14));
  },
  /** The Flash's ring (brand nod: DC): a gold ring with a red lightning emblem on a little red cushion; every 15 s its emblem pops open and a tiny red suit springs out, then tucks back in. */
  dcmixflashring(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 3, 9, 3, DC_RED); fill(x + 35, y - 3, 9, 1, DC_RED_LIGHT); fill(x + 35, y - 1, 9, 1, DC_RED_DARK);
    ring(x + 39, y - 6, 2, DC_GOLD); dot(x + 39, y - 8, DC_GOLD); disc(x + 39, y - 9, 1.5, DC_RED); dot(x + 39, y - 9, DC_YELLOW);
    const t = (now + x * 0.27) % 15;
    if (t < 2) {
      const up = Math.min(4, Math.floor(t * 8));
      fill(x + 38, y - 10 - up, 3, up, DC_RED); dot(x + 39, y - 11 - up, DC_RED); dot(x + 39, y - 9 - Math.floor(up / 2), DC_YELLOW);
      if (up >= 4) { dot(x + 36, y - 13, DC_YELLOW); dot(x + 42, y - 14, DC_YELLOW); }
    }
  },
  /** A Poké Ball on a little black stand: every 16 s it wobbles three times as if something inside were trying to get out, then the button flashes and a few stars pop over it, caught. */
  pokemonball(x, y) {
    const t = (now + x * 0.27) % 16, wob = t < 1.8 ? [0, -1, 0, 1, 0, -1, 0, 1, 0][Math.floor(t * 5)] || 0 : 0, caught = t >= 1.8 && t < 2.8;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 2, 7, 2, 0x2a2a32); fill(x + 36, y - 2, 7, 1, 0x4a4a54);
    pokeBall(x + 35 + wob, y - 11, 9, PKM_RED, 1, caught);
    if (caught) for (const [dx, dy] of [[0, -13], [4, -14], [8, -13]]) if (Math.floor(now * 6 + dx) % 2) dot(x + 35 + dx, y + dy, 0xfff4a0);
    if (isNight && caught) glowLate(() => glowAt(x + 39, y - 7, 5, 4, 0xff6a50, 0.2));
  },
  /** A Game Boy: the grey brick with its pea-green screen showing a tiny battle (a sprite bobbing, an HP bar that drains and refills), a dark D-pad and two plum buttons; the screen glows faintly at night. */
  pokemongameboy(x, y) {
    const body = 0xc8c4bc, light = 0xdedad2, dark = 0x9a968e, scr = 0x9bbc0f, scrDark = 0x306230, pad = 0x3a3a44, plum = 0x9a2a5a;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 12, 9, 12, body); fill(x + 35, y - 12, 9, 1, light); fill(x + 35, y - 12, 1, 12, light); fill(x + 43, y - 11, 1, 11, dark); fill(x + 35, y - 1, 9, 1, dark);
    fill(x + 36, y - 11, 7, 6, 0x5a5a6a); fill(x + 37, y - 10, 5, 4, scr);
    const hp = 3 - Math.floor(((now + x * 0.2) % 12) / 4);
    fill(x + 37, y - 10, hp, 1, scrDark);
    const bob = Math.floor(now * 2 + x) % 2;
    fill(x + 39, y - 9 + bob, 2, 2, scrDark); dot(x + 40, y - 9 + bob, 0x8bac0f);
    fill(x + 36, y - 4, 3, 1, pad); fill(x + 37, y - 5, 1, 3, pad);
    dot(x + 41, y - 3, plum); dot(x + 42, y - 4, plum);
    dot(x + 39, y - 2, dark); dot(x + 40, y - 2, dark);
    if (isNight) glowLate(() => glowAt(x + 39, y - 8, 4, 3, 0xb0d830, 0.12));
  },
  /** A Pokémon egg in an incubator: a cream egg with green spots under a glass dome on a red base with a little light; every 12 s the egg wiggles, now and then a crack shows for a moment, and the light glows at night. */
  pokemonegg(x, y) {
    const egg = 0xf6eedc, eggDark = 0xd8ccb4, spot = 0x5ab86a;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 3, 9, 3, PKM_RED); fill(x + 35, y - 3, 9, 1, PKM_RED_LIGHT); fill(x + 35, y - 1, 9, 1, PKM_RED_DARK); dot(x + 42, y - 2, 0x8af0a0);
    const t = (now + x * 0.33) % 12, wig = t < 1.2 ? [0, 1, 0, -1][Math.floor(t * 8) % 4] : 0, ex = x + 37 + wig;
    fill(ex + 1, y - 10, 3, 1, egg); fill(ex, y - 9, 5, 5, egg); fill(ex + 1, y - 4, 3, 1, egg);
    dot(ex, y - 8, WHITE, 0.7); fill(ex + 4, y - 8, 1, 4, eggDark); dot(ex + 3, y - 4, eggDark);
    dot(ex + 1, y - 7, spot); dot(ex + 3, y - 9, spot); dot(ex + 2, y - 5, spot); dot(ex + 4, y - 6, spot);
    if ((now + x * 0.41) % 47 < 1.2) { dot(ex + 1, y - 7, 0x6a5a40); dot(ex + 2, y - 8, 0x6a5a40); dot(ex + 3, y - 7, 0x6a5a40); }
    // The dome
    fill(x + 36, y - 12, 7, 9, 0xdff0f6, 0.3); fill(x + 36, y - 12, 7, 1, 0xdff0f6, 0.7); fill(x + 36, y - 11, 1, 8, 0xdff0f6, 0.6); fill(x + 42, y - 11, 1, 8, 0xdff0f6, 0.6);
    dot(x + 37, y - 11, WHITE, 0.8);
    if (isNight) glowLate(() => { glowAt(x + 42, y - 2, 3, 2, 0x8af0a0, 0.25); dot(x + 42, y - 2, 0xc0ffd0); });
  },
  /** A Potion: the purple spray bottle with its white trigger top and label; every 20 s it gives a little spray, a puff of mist fading off to the left. */
  pokemonpotion(x, y) {
    const purple = 0x8a4ac8, purpleLight = 0xa86ae0, purpleDark = 0x5e2e94;
    fill(x + 36, y, 8, 1, SHADOW);
    fill(x + 37, y - 8, 6, 8, purple); fill(x + 37, y - 8, 1, 8, purpleLight); fill(x + 42, y - 8, 1, 8, purpleDark); fill(x + 37, y - 1, 6, 1, purpleDark);
    fill(x + 37, y - 6, 6, 3, PKM_WHITE); fill(x + 38, y - 5, 4, 1, 0x8a8a94); dot(x + 42, y - 5, PKM_GREY);
    fill(x + 38, y - 10, 4, 2, PKM_WHITE); fill(x + 37, y - 12, 5, 2, PKM_WHITE); dot(x + 36, y - 12, PKM_GREY); fill(x + 41, y - 10, 1, 2, PKM_GREY);
    dot(x + 38, y - 7, WHITE, 0.6);
    const t = (now + x * 0.39) % 20;
    if (t < 1.4) for (let k = 0; k < 5; k++) { const d = Math.floor(t * 4) + k; dot(x + 35 - Math.floor(d / 2), y - 12 + ((k * 3) % 4) - 1, WHITE, Math.max(0, 0.8 - t * 0.5)); }
  },
  /** A deck of Pokémon cards: a stack of blue card backs, and a holo card leaning on it, yellow-bordered with a fire-red creature on sky; every 9 s a rainbow shimmer sweeps across the holo. */
  pokemoncards(x, y) {
    const back = 0x2a4aa8, border = 0xf8d030, art = 0x9ad4f8;
    fill(x + 33, y, 12, 1, SHADOW);
    fill(x + 34, y - 4, 6, 4, back); for (let k = 0; k < 3; k++) fill(x + 34, y - 3 + k, 6, 1, k % 2 ? 0xe8e8f0 : back); fill(x + 34, y - 4, 6, 1, 0x4a6ac8);
    dot(x + 36, y - 4, PKM_RED); dot(x + 37, y - 4, 0xf8d030);
    // The holo card, leaning back
    fill(x + 39, y - 12, 6, 11, border); fill(x + 39, y - 12, 6, 1, PKM_YELLOW_LIGHT); fill(x + 44, y - 11, 1, 10, PKM_YELLOW_DARK);
    fill(x + 40, y - 10, 4, 4, art); fill(x + 41, y - 9, 2, 2, 0xe8682a); dot(x + 42, y - 10, 0xf8a030); dot(x + 40, y - 8, 0xffd040);
    fill(x + 40, y - 5, 4, 3, 0xf4ecd0); fill(x + 40, y - 4, 3, 1, 0xb8b0a0); dot(x + 43, y - 11, PKM_RED);
    const s = (now + x * 0.23) % 9;
    if (s < 1.2) { const k = Math.floor(s * 6); for (let i = 0; i < 4; i++) { const sx = k - i; if (sx >= 0 && sx < 4) dot(x + 40 + sx, y - 10 + i, rainbow((now * 0.8 + i * 0.2) % 1), 0.75); } }
  },
  /** A Magikarp bowl: a round glass bowl of water with a red-orange Magikarp, white-bellied with yellow fins and whiskers, swimming to and fro; every 10 s it flops up out of the water with a splash and drops back. */
  pokemonmagikarp(x, y) {
    const fish = 0xe8582a, fishDark = 0xb03a1a, fin = 0xf8d030, water = 0x6ab8e8;
    fill(x + 34, y, 11, 1, SHADOW);
    // The water, and the fish: swimming, or a flop
    fill(x + 35, y - 7, 9, 6, water, 0.5); fill(x + 36, y - 1, 7, 1, water, 0.5); fill(x + 35, y - 7, 9, 1, 0xb8e4f8, 0.8);
    const t = (now + x * 0.31) % 10, flop = t < 1 ? [2, 4, 5, 5, 4, 2, 0, 0][Math.floor(t * 8)] : 0;
    const swim = Math.round(Math.sin(now * 0.7 + x) * 1.5), fx = x + 38 + swim, fy = y - 5 - flop, right = Math.cos(now * 0.7 + x) >= 0;
    fill(fx, fy, 4, 3, fish); fill(fx, fy + 2, 4, 1, PKM_WHITE); dot(fx + (right ? 3 : 0), fy, PALETTE.eye);
    dot(right ? fx - 1 : fx + 4, fy + 1, fin); dot(right ? fx - 1 : fx + 4, fy, fin); dot(fx + 1, fy - 1, fin); dot(right ? fx + 4 : fx - 1, fy + 2, fin);
    dot(fx + 1, fy + 1, fishDark);
    if (flop && t < 0.9) for (const sx of [-2, 5]) dot(fx + sx, y - 8 - Math.floor(t * 4), 0xb8e4f8, 0.9);
    // The bowl's glass
    fill(x + 35, y - 10, 9, 1, 0xdff0f6, 0.7); fill(x + 34, y - 9, 1, 8, 0xdff0f6, 0.6); fill(x + 44, y - 9, 1, 8, 0xdff0f6, 0.6); fill(x + 35, y - 1, 9, 1, 0xdff0f6, 0.75);
    fill(x + 35, y - 9, 9, 8, 0xdff0f6, 0.15); dot(x + 36, y - 8, WHITE, 0.8);
  },
  /** A singing Jigglypuff: a round pink puff with big blue-green eyes and a curl on top, holding a little microphone; every 20 s it sings for a few seconds, mouth open and notes rising, and then its eyes droop shut for a moment. */
  pokemonjiggly(x, y) {
    const pink = 0xf8b0c8, pinkLight = 0xfcd0e0, pinkDark = 0xd8809c, eyeC = 0x2a8a9a;
    fill(x + 34, y, 11, 1, SHADOW);
    const t = (now + x * 0.43) % 20, singing = t < 4, sleepy = t >= 4 && t < 6;
    fill(x + 36, y - 9, 6, 1, pink); fill(x + 35, y - 8, 8, 6, pink); fill(x + 36, y - 2, 6, 1, pink);
    fill(x + 35, y - 8, 1, 5, pinkLight); fill(x + 36, y - 9, 3, 1, pinkLight); fill(x + 42, y - 7, 1, 5, pinkDark); fill(x + 36, y - 2, 6, 1, pinkDark);
    dot(x + 38, y - 10, pink); dot(x + 39, y - 11, pink); dot(x + 39, y - 10, pinkDark);
    fill(x + 36, y - 1, 2, 1, pinkDark); fill(x + 40, y - 1, 2, 1, pinkDark);
    if (sleepy) { fill(x + 36, y - 6, 2, 1, PALETTE.eye); fill(x + 40, y - 6, 2, 1, PALETTE.eye); }
    else { fill(x + 36, y - 7, 2, 2, eyeC); fill(x + 40, y - 7, 2, 2, eyeC); dot(x + 36, y - 7, WHITE); dot(x + 40, y - 7, WHITE); }
    if (singing) dot(x + 39, y - 4, 0x8a2a4a); else dot(x + 39, y - 4, pinkDark);
    // The mic, held to one side
    fill(x + 43, y - 5, 1, 3, 0x3a3a44); fill(x + 43, y - 6, 2, 1, 0x6e727c);
    if (singing) for (let k = 0; k < 2; k++) {
      const n = (t + k * 2) % 4, nx = x + 43 + k * 1 - Math.floor(n), ny = y - 10 - Math.floor(n * 1.2);
      dot(nx, ny + 2, 0x4a2a5a, 1 - n / 4); fill(nx + 1, ny, 1, 3, 0x4a2a5a, 1 - n / 4);
    }
  },
  /** Evolution stones on a velvet tray: a Fire Stone, a Water Stone and a Thunder Stone in front, a Moon Stone propped up behind; a glint runs over them every 10 s, and at night each glows faintly in its colour, the Thunder Stone crackling. */
  pokemonstones(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 34, y - 3, 11, 3, 0x6a2a3a); fill(x + 34, y - 3, 11, 1, 0x8a3a4a); fill(x + 34, y - 1, 11, 1, 0x4a1a28);
    // The Moon Stone behind, on a little wire stand
    fill(x + 38, y - 10, 4, 5, 0x6a6a84); fill(x + 39, y - 11, 2, 1, 0x6a6a84); dot(x + 38, y - 10, 0x9a9ab4); dot(x + 41, y - 6, 0x4a4a60); dot(x + 39, y - 8, 0xd8d8f0);
    fill(x + 39, y - 5, 2, 2, 0x8a8e98);
    // The three in front, each with its mark
    const stones = [[35, 0xf07830, 0xf8d040], [38, 0x4a9ae8, 0xe8f4ff], [41, 0x5ac870, 0xf8f040]];
    const g = (now + x * 0.19) % 10, shine = g < 1.5 ? Math.floor(g * 2) : -1;
    stones.forEach(([sx, c, mark], i) => {
      fill(x + sx, y - 5, 3, 3, c); dot(x + sx, y - 5, shaded(c, 1.2)); dot(x + sx + 2, y - 3, shaded(c, 0.75)); dot(x + sx + 1, y - 4, mark);
      if (i === shine) dot(x + sx, y - 5, WHITE, 0.9);
    });
    if (isNight) glowLate(() => {
      stones.forEach(([sx, c]) => glowAt(x + sx + 1, y - 4, 3, 3, c, 0.18));
      if (Math.floor(now * 5 + x) % 7 === 0) dot(x + 43, y - 6, 0xfff8a0);
    });
  },
  /** A jar of Poro-Snax: a glass jar of golden star-shaped biscuits under a blue lid with a gold knob; every 18 s the lid hops up and a biscuit peeks over the rim. */
  lolsnax(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    const t = (now + x * 0.31) % 18, hop = t < 0.8 ? 2 : 0;
    for (let k = 0; k < 8; k++) { const bx = x + 36 + (k % 3) * 2 + (k > 5 ? 1 : 0), by = y - 2 - Math.floor(k / 3) * 2; fill(bx, by, 2, 2, k % 2 ? 0xe0a040 : 0xc88830); dot(bx, by, 0xf8d080); }
    if (hop) { fill(x + 39, y - 10, 2, 2, 0xe0a040); dot(x + 39, y - 10, 0xf8d080); }
    fill(x + 35, y - 9, 9, 9, 0xdff0f6, 0.3); fill(x + 35, y - 9, 1, 9, 0xdff0f6, 0.6); fill(x + 43, y - 9, 1, 9, 0xdff0f6, 0.6); fill(x + 35, y - 1, 9, 1, 0xdff0f6, 0.7);
    dot(x + 36, y - 8, WHITE, 0.8);
    fill(x + 35, y - 11 - hop, 9, 2, 0x2a5ab8); fill(x + 35, y - 11 - hop, 9, 1, 0x4a7ad8); dot(x + 39, y - 12 - hop, LOL_GOLD); dot(x + 43, y - 10 - hop, 0x1a3a7a);
  },
  /** A stealth ward: a dark wooden totem on a little base, its yellow-green eye wide open at the top, blinking now and then; a faint ring of vision glows round it at night. */
  lolward(x, y) {
    const wood = 0x3a2a24, woodLight = 0x5a443a;
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 36, y - 2, 7, 2, wood); fill(x + 36, y - 2, 7, 1, woodLight);
    fill(x + 38, y - 9, 3, 7, wood); fill(x + 38, y - 9, 1, 7, woodLight);
    fill(x + 37, y - 12, 5, 4, 0x2a1e1a); dot(x + 37, y - 12, woodLight); dot(x + 36, y - 11, wood); dot(x + 42, y - 11, wood);
    const blink = (now + x * 0.27) % 7 < 0.25;
    if (blink) fill(x + 38, y - 10, 3, 1, 0xc8d84a); else { fill(x + 38, y - 11, 3, 2, 0xe8f04a); dot(x + 39, y - 11, 0x1a1a10); }
    glowAt(x + 39, y - 10, 3, 2, 0xe8f04a, 0.12);
    if (isNight) glowLate(() => { ellipseRing(x + 39, y - 1, 7, 2, 0xe8f04a, 0.25); glowAt(x + 39, y - 10, 4, 3, 0xe8f04a, 0.25); if (!blink) dot(x + 38, y - 11, 0xf8ffa0); });
  },
  /** A health potion: a round glass flask of red potion with a cork and a gold neck band, bubbles rising through it; a soft red glow at night. */
  lolpotion(x, y) {
    const red = 0xd8283a, redLight = 0xf05a6a, redDark = 0x9a1a2a;
    fill(x + 35, y, 9, 1, SHADOW);
    disc(x + 39.5, y - 4, 4.4, 0xdff0f6, 0.5); disc(x + 39.5, y - 3.5, 3.6, red);
    fill(x + 37, y - 6, 5, 1, redLight); fill(x + 41, y - 4, 2, 3, redDark);
    fill(x + 38, y - 11, 3, 3, 0xdff0f6, 0.55); fill(x + 38, y - 9, 3, 1, LOL_GOLD); fill(x + 38, y - 12, 3, 2, 0x9a6a3a); dot(x + 38, y - 12, 0xba8a5a);
    for (let k = 0; k < 2; k++) { const b = (now * 0.8 + k * 0.5 + x * 0.1) % 1; dot(x + 38 + k * 2, y - 2 - Math.floor(b * 5), 0xffc0c8, 1 - b); }
    dot(x + 37, y - 7, WHITE, 0.8);
    if (isNight) glowLate(() => glowAt(x + 39, y - 4, 5, 4, red, 0.18));
  },
  /** A Hextech crystal held up in a gold claw on a navy base, pulsing slowly blue; every 12 s it flares and a few sparks spin off it, and it glows at night. */
  lolcrystal(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, LOL_NAVY); fill(x + 35, y - 2, 9, 1, LOL_GOLD); fill(x + 39, y - 4, 1, 2, LOL_GOLD_DARK);
    const f = (now + x * 0.27) % 12, flare = f < 1.2, pulse = flare ? 1 : (Math.sin(now * 1.5 + x) + 1) / 2;
    hexCrystal(x + 39, y - 8, 9, pulse);
    line(x + 36, y - 4, x + 37, y - 9, LOL_GOLD); line(x + 42, y - 4, x + 41, y - 9, LOL_GOLD); dot(x + 36, y - 4, LOL_GOLD_LIGHT);
    if (flare) for (let k = 0; k < 3; k++) { const a = now * 4 + k * 2.1; dot(x + 39 + Math.round(Math.cos(a) * 4), y - 8 + Math.round(Math.sin(a) * 5), LOL_HEX_LIGHT); }
    if (isNight) glowLate(() => { glowAt(x + 39, y - 8, 6, 7, LOL_HEX, 0.15 + 0.1 * pulse); dot(x + 39, y - 9, WHITE, 0.7); });
  },
  /** A Teemo figure: the little scout with his big furry ears, tan face and green hat with goggles; every 20 s a spotted mushroom pops up beside him, puffs a cloud of green and vanishes. */
  lolteemo(x, y) {
    const fur = 0xd8b070, furDark = 0xa8803a, green = 0x5a7a2a, greenLight = 0x7a9a3a, greenDark = 0x3a5a1a;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 38, y - 5, 4, 5, green); fill(x + 38, y - 5, 1, 5, greenLight); fill(x + 41, y - 5, 1, 5, greenDark);
    fill(x + 38, y - 1, 1, 1, furDark); fill(x + 41, y - 1, 1, 1, furDark);
    fill(x + 37, y - 9, 6, 4, fur); dot(x + 38, y - 7, PALETTE.eye); dot(x + 41, y - 7, PALETTE.eye); dot(x + 39, y - 6, 0x3a2a1a); dot(x + 40, y - 6, 0x3a2a1a);
    fill(x + 35, y - 9, 2, 2, fur); dot(x + 34, y - 9, furDark); fill(x + 43, y - 9, 2, 2, fur); dot(x + 44, y - 9, furDark);
    fill(x + 37, y - 11, 6, 2, green); fill(x + 36, y - 10, 8, 1, greenDark); fill(x + 37, y - 11, 6, 1, greenLight);
    dot(x + 38, y - 11, 0xd8e8f0); dot(x + 41, y - 11, 0xd8e8f0); dot(x + 42, y - 12, 0xc83a3a);
    const t = (now + x * 0.41) % 20;
    if (t < 2.4) {
      const up = Math.min(3, Math.floor(t * 6));
      if (t < 1.8) { fill(x + 34, y - up, 2, up, 0xe8dcc0); fill(x + 33, y - up - 2, 4, 2, 0x8a6a2a); dot(x + 34, y - up - 2, 0xe8dcc0); dot(x + 36, y - up - 1, 0xc83a3a); }
      else { const r = (t - 1.8) * 6; disc(x + 34, y - 3, 1 + r, 0x8ac83a, 0.6 - (t - 1.8)); }
    }
  },
  /** A Hextech chest: navy with gold bands and corners, a blue crystal lock; every 20 s the lid lifts, blue light spills out with a few sparkles, and it shuts again. */
  lolchest(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    const t = (now + x * 0.29) % 20, open = t < 2.5 ? (t < 0.4 || t > 2.1 ? 1 : 3) : 0;
    fill(x + 35, y - 6, 9, 6, LOL_NAVY); fill(x + 35, y - 6, 1, 6, LOL_GOLD); fill(x + 43, y - 6, 1, 6, LOL_GOLD_DARK); fill(x + 35, y - 1, 9, 1, LOL_GOLD_DARK);
    fill(x + 38, y - 6, 1, 6, LOL_GOLD_DARK); fill(x + 41, y - 6, 1, 6, LOL_GOLD_DARK);
    if (open) { fill(x + 36, y - 7, 7, 1, LOL_HEX_LIGHT); glowAt(x + 39, y - 8, 6, 4, LOL_HEX, 0.3); for (let k = 0; k < 3; k++) if (Math.floor(now * 6 + k) % 2) dot(x + 37 + k * 2, y - 9 - k - Math.floor(t * 2) % 3, LOL_HEX_LIGHT); }
    const ly = y - 9 - open;
    fill(x + 35, ly, 9, 3, LOL_NAVY_LIGHT); fill(x + 35, ly, 9, 1, LOL_GOLD); fill(x + 35, ly, 1, 3, LOL_GOLD); fill(x + 43, ly, 1, 3, LOL_GOLD_DARK);
    fill(x + 38, y - 7, 3, 3, LOL_GOLD); dot(x + 39, y - 6, open ? LOL_HEX_LIGHT : LOL_HEX);
    if (isNight) glowLate(() => glowAt(x + 39, y - 6, 3, 3, LOL_HEX, open ? 0.35 : 0.12));
  },
  /** A Blitzcrank figure: the round brass steam robot, a slit of blue eye, a vent puffing now and then; every 16 s one long arm shoots out sideways in a rocket grab and reels back in. */
  lolblitz(x, y) {
    const brass = 0xd8a830, brassLight = 0xf0c858, brassDark = 0x9a7420, iron = 0x5a5a64;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 1, 2, 1, iron); fill(x + 41, y - 1, 2, 1, iron);
    fill(x + 36, y - 8, 8, 7, brass); fill(x + 36, y - 8, 1, 7, brassLight); fill(x + 43, y - 8, 1, 7, brassDark); fill(x + 36, y - 2, 8, 1, brassDark);
    fill(x + 38, y - 11, 4, 3, brass); fill(x + 38, y - 11, 4, 1, brassLight); fill(x + 38, y - 10, 4, 1, 0x1a2a3a); fill(x + 39, y - 10, 2, 1, 0x6ad8ff);
    fill(x + 38, y - 6, 4, 2, iron); dot(x + 39, y - 5, 0xf2c43a);
    const t = (now + x * 0.33) % 16, reach = t < 1.4 ? [1, 2, 4, 6, 6, 4, 2, 1][Math.floor(t * 6)] || 0 : 0;
    fill(x + 35, y - 7, 1, 4, iron); fill(x + 34, y - 4, 2, 2, brassDark);
    if (reach) { fill(x + 44, y - 6, reach, 1, iron); fill(x + 44 + reach, y - 7, 2, 3, brassDark); dot(x + 44 + reach, y - 7, brass); }
    else { fill(x + 44, y - 7, 1, 4, iron); fill(x + 44, y - 4, 2, 2, brassDark); }
    const puff = (now + x * 0.2) % 6;
    if (puff < 1) dot(x + 37, y - 12 - Math.floor(puff * 3), WHITE, 0.7 - puff * 0.6);
    if (isNight) glowLate(() => glowAt(x + 40, y - 10, 3, 2, 0x6ad8ff, 0.2));
  },
  /** A gaming mouse on a navy mousepad with a gold emblem: an RGB strip down the mouse runs through the colours, and every few seconds it clicks, the button dipping. */
  lolmouse(x, y) {
    fill(x + 33, y, 12, 1, SHADOW);
    fill(x + 33, y - 2, 12, 2, LOL_NAVY); fill(x + 33, y - 2, 12, 1, LOL_NAVY_LIGHT); dot(x + 35, y - 1, LOL_GOLD);
    const click = (now * 1.3 + x) % 3 < 0.15 ? 1 : 0, glow = rainbow((now * 0.2 + x * 0.01) % 1);
    fill(x + 38, y - 7, 5, 5, 0x2a2a34); fill(x + 39, y - 8 + click, 3, 1, 0x3a3a46); fill(x + 38, y - 7, 1, 5, 0x3e3e4a);
    fill(x + 40, y - 7 + click, 1, 2, 0x1a1a22); fill(x + 40, y - 4, 1, 2, glow);
    fill(x + 38, y - 3, 5, 1, 0x1a1a22);
    line(x + 40, y - 8, x + 42, y - 12, 0x2a2a34); dot(x + 43, y - 12, 0x2a2a34);
    if (isNight) glowLate(() => { glowAt(x + 40, y - 3, 4, 2, glow, 0.2); fill(x + 40, y - 4, 1, 2, glow); });
  },
  /** A Hearthstone: a smooth blue-grey stone with a glowing rune carved in it; every 15 s it's cast, the rune flaring and blue motes spiralling up from it. It glows at night. */
  wowhearth(x, y) {
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 36, y - 6, 7, 6, 0x6a7a8a); fill(x + 37, y - 7, 5, 1, 0x6a7a8a); fill(x + 36, y - 6, 1, 5, 0x8a9aaa); fill(x + 42, y - 5, 1, 5, 0x4a5a6a); fill(x + 37, y - 1, 5, 1, 0x4a5a6a);
    const t = (now + x * 0.31) % 15, cast = t < 2, rune = cast ? 0xbfe8ff : 0x5ab0f0;
    dot(x + 39, y - 5, rune); dot(x + 40, y - 4, rune); dot(x + 39, y - 3, rune); dot(x + 38, y - 4, rune); dot(x + 39, y - 4, cast ? WHITE : 0x3a7ab8);
    if (cast) for (let k = 0; k < 4; k++) { const s = (t * 0.8 + k / 4) % 1, a = s * 8 + k; dot(x + 39 + Math.round(Math.cos(a) * 3), y - 7 - Math.floor(s * 6), 0x9ad8ff, 1 - s); }
    glowAt(x + 39, y - 4, 3, 3, 0x5ab0f0, cast ? 0.3 : 0.1);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 4, 4, 4, 0x5ab0f0, 0.2); dot(x + 39, y - 4, 0xd8f0ff); });
  },
  /** A mana potion: a round glass bottle of glowing blue potion with a cork, bubbles rising through it; it glows softly at night. */
  wowmana(x, y) {
    const blue = 0x2a6ae8, blueLight = 0x6aa8ff, blueDark = 0x1a3a9a;
    fill(x + 35, y, 9, 1, SHADOW);
    disc(x + 39.5, y - 4, 4.4, 0xdff0f6, 0.5); disc(x + 39.5, y - 3.5, 3.6, blue);
    fill(x + 37, y - 6, 5, 1, blueLight); fill(x + 41, y - 4, 2, 3, blueDark);
    fill(x + 38, y - 11, 3, 3, 0xdff0f6, 0.55); fill(x + 38, y - 12, 3, 2, 0x9a6a3a); dot(x + 38, y - 12, 0xba8a5a); fill(x + 37, y - 9, 5, 1, WOW_GOLD_DARK);
    for (let k = 0; k < 2; k++) { const b = (now * 0.7 + k * 0.5 + x * 0.1) % 1; dot(x + 38 + k * 2, y - 2 - Math.floor(b * 5), 0xc8e8ff, 1 - b); }
    dot(x + 37, y - 7, WHITE, 0.8);
    glowAt(x + 39, y - 4, 4, 3, blue, 0.08);
    if (isNight) glowLate(() => glowAt(x + 39, y - 4, 5, 4, blue, 0.22));
  },
  /** A pile of gold: a heap of gold coins by a small tied sack spilling more; a glint skips over the coins, and every 20 s another coin drops onto the pile with a bounce. */
  wowgold(x, y) {
    const gold = WOW_GOLD, goldLight = 0xfff0a0, goldDark = WOW_GOLD_DARK;
    fill(x + 33, y, 12, 1, SHADOW);
    fill(x + 35, y - 7, 5, 7, 0x8a6a3a); fill(x + 36, y - 8, 3, 1, 0x8a6a3a); fill(x + 35, y - 7, 1, 7, 0xaa8a5a); fill(x + 36, y - 9, 3, 1, 0x6a4a2a); dot(x + 37, y - 10, 0x6a4a2a);
    fill(x + 38, y - 3, 7, 3, gold); fill(x + 39, y - 4, 5, 1, gold); fill(x + 40, y - 5, 3, 1, gold); fill(x + 38, y - 1, 7, 1, goldDark);
    for (const [cx, cy] of [[39, -3], [41, -2], [43, -3], [41, -4], [40, -1]]) dot(x + cx, y + cy, goldDark);
    const g = Math.floor(now * 3 + x) % 9; if (g < 5) dot(x + 38 + g, y - 3 - (g % 2), goldLight);
    const t = (now + x * 0.37) % 20;
    if (t < 1) { const fy = [-12, -9, -6, -5, -7, -6, -5][Math.floor(t * 7)]; fill(x + 41, y + fy, 2, 1, gold); dot(x + 41, y + fy, goldLight); }
  },
  /** A tankard of ale: a wooden tankard bound with iron hoops, a handle on the side and a head of foam; every 16 s the foam swells and a drop runs down the side. */
  wowale(x, y) {
    fill(x + 35, y, 10, 1, SHADOW);
    fill(x + 36, y - 9, 7, 9, WOW_WOOD); fill(x + 36, y - 9, 1, 9, WOW_WOOD_LIGHT); fill(x + 42, y - 9, 1, 9, WOW_WOOD_DARK);
    for (let k = 0; k < 3; k++) fill(x + 38 + k * 2, y - 9, 1, 9, WOW_WOOD_DARK, 0.5);
    for (const hy of [-8, -2]) { fill(x + 36, y + hy, 7, 1, WOW_IRON); dot(x + 36, y + hy, 0x8a8a92); }
    fill(x + 43, y - 7, 2, 1, WOW_IRON); fill(x + 44, y - 7, 1, 4, WOW_IRON); fill(x + 43, y - 4, 2, 1, WOW_IRON);
    const t = (now + x * 0.29) % 16, swell = t < 2 ? 1 : 0;
    fill(x + 36, y - 11 - swell, 7, 2 + swell, 0xf6efd8); dot(x + 37, y - 12 - swell, 0xf6efd8); dot(x + 40, y - 12 - swell, 0xf6efd8); dot(x + 38, y - 11 - swell, WHITE);
    if (t < 2) dot(x + 36, y - 9 + Math.floor(t * 4), 0xf6efd8);
  },
  /** Frostmourne on a stand: the great runeblade upright on a dark stand, its pale blade carved with blue runes that pulse, a skull-and-horns hilt; wisps of frost curl off it now and then, and the runes glow at night. */
  wowfrostmourne(x, y) {
    const steel = 0xc8d8e8, steelDark = 0x7a8a9a, rune = 0x6ad8ff, dark = 0x2a2a34;
    fill(x + 35, y, 10, 1, SHADOW);
    fill(x + 36, y - 2, 8, 2, dark); fill(x + 36, y - 2, 8, 1, 0x4a4a54);
    fill(x + 39, y - 11, 3, 9, steel); fill(x + 41, y - 11, 1, 9, steelDark); dot(x + 40, y - 12, steel);
    const pulse = (Math.sin(now * 1.8 + x) + 1) / 2;
    for (let k = 0; k < 4; k++) dot(x + 40, y - 10 + k * 2, rune, 0.5 + 0.5 * pulse);
    fill(x + 37, y - 3, 7, 1, dark); dot(x + 37, y - 4, dark); dot(x + 43, y - 4, dark); fill(x + 39, y - 3, 3, 1, 0x6a6a7a); dot(x + 40, y - 3, rune);
    const t = (now + x * 0.23) % 11;
    if (t < 2) { const s = t / 2; dot(x + 38 - Math.floor(s * 3), y - 8 - Math.floor(s * 3), 0xe8f4ff, 0.7 * (1 - s)); dot(x + 42 + Math.floor(s * 2), y - 6 - Math.floor(s * 4), 0xe8f4ff, 0.6 * (1 - s)); }
    glowAt(x + 40, y - 7, 3, 6, rune, 0.06 + 0.06 * pulse);
    if (isNight) glowLate(() => { glowAt(x + 40, y - 7, 4, 7, rune, 0.15 + 0.1 * pulse); for (let k = 0; k < 4; k++) dot(x + 40, y - 10 + k * 2, 0xb8f0ff); });
  },
  /** An epic loot bag: a tied leather sack giving off a purple glow, a beam of violet light rising off it and sparkles drifting up; the glow pulses, brighter at night. */
  wowloot(x, y) {
    const leather = 0x8a5a32, light = 0xaa7a4a, dark = 0x5a3a1e, epic = 0xa335ee;
    fill(x + 34, y, 11, 1, SHADOW);
    const pulse = (Math.sin(now * 2 + x) + 1) / 2;
    fill(x + 39, y - 12, 1, 8, epic, 0.15 + 0.15 * pulse);
    fill(x + 36, y - 6, 7, 6, leather); fill(x + 35, y - 5, 9, 4, leather); fill(x + 35, y - 5, 1, 4, light); fill(x + 43, y - 5, 1, 4, dark); fill(x + 36, y - 1, 7, 1, dark);
    fill(x + 37, y - 8, 5, 2, leather); fill(x + 37, y - 7, 5, 1, 0xc8a04a); dot(x + 38, y - 9, leather); dot(x + 40, y - 9, leather);
    dot(x + 37, y - 4, light);
    for (let k = 0; k < 3; k++) { const s = (now * 0.5 + k / 3 + x * 0.01) % 1; dot(x + 37 + k * 2, y - 8 - Math.floor(s * 5), 0xe0b0ff, 1 - s); }
    glowAt(x + 39, y - 5, 6, 5, epic, 0.1 + 0.08 * pulse);
    if (isNight) glowLate(() => glowAt(x + 39, y - 6, 6, 6, epic, 0.15 + 0.1 * pulse));
  },
  /** A quest scroll: a parchment half unrolled on the desk, lines of ink and a red wax seal; every 25 s it rolls up a little and opens out again. */
  wowscroll(x, y) {
    const paper = 0xeadcb4, paperDark = 0xc8b88a, ink = 0x5a4a3a;
    fill(x + 33, y, 12, 1, SHADOW);
    const t = (now + x * 0.21) % 25, curl = t < 1.6 ? [1, 2, 3, 3, 2, 1][Math.floor(t * 4)] || 0 : 0;
    fill(x + 34 + curl, y - 6, 9 - curl, 5, paper); fill(x + 34 + curl, y - 6, 9 - curl, 1, WHITE, 0.5);
    for (let k = 0; k < 2; k++) fill(x + 36 + curl, y - 4 + k * 2, 5 - curl, 1, ink, 0.6);
    fill(x + 42, y - 7, 3, 7, paperDark); fill(x + 42, y - 7, 1, 7, paper); dot(x + 43, y - 8, paperDark); dot(x + 43, y, paperDark);
    fill(x + 33 + curl, y - 7, 2, 7, paperDark); fill(x + 33 + curl, y - 7, 1, 7, paper);
    disc(x + 39, y - 2, 1.5, 0xb02a2a); dot(x + 39, y - 2, 0xd84a3a);
  },
  /** A fel brazier: an iron bowl on three legs full of green fel fire, the flames leaping and flickering, a few green embers rising; it lights the desk green at night. */
  wowfel(x, y) {
    fill(x + 35, y, 9, 1, SHADOW);
    line(x + 37, y - 4, x + 36, y - 1, WOW_IRON); line(x + 42, y - 4, x + 43, y - 1, WOW_IRON); fill(x + 39, y - 4, 1, 4, WOW_IRON);
    fill(x + 35, y - 6, 9, 2, WOW_IRON); fill(x + 35, y - 6, 9, 1, 0x8a8a92); fill(x + 36, y - 4, 7, 1, 0x3a3a42);
    const f = Math.floor(now * 7 + x) % 3;
    fill(x + 36, y - 8, 7, 2, 0x3ab82a); fill(x + 37, y - 10 + (f === 1 ? 1 : 0), 2, 2, WOW_FEL); fill(x + 40, y - 11 + (f === 2 ? 1 : 0), 2, 3, WOW_FEL);
    dot(x + 39, y - 9, 0xd8ffb0); dot(x + 38 + f, y - 12 + (f % 2), 0xa8ff80);
    const e = (now * 0.9 + x * 0.1) % 1; dot(x + 39 + Math.round(Math.sin(now * 3) * 2), y - 12 - Math.floor(e * 4), WOW_FEL, 1 - e);
    glowAt(x + 39, y - 9, 5, 4, WOW_FEL, 0.12);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 8, 8, 6, WOW_FEL, 0.22); dot(x + 39, y - 9, 0xe8ffd0); });
  },
  /** A briefcase of cash: an open black case, its lid up behind, packed with banded bundles of green notes; every 14 s a loose note slides off the top and flutters down beside it. */
  gtacash(x, y) {
    const note = 0x7ab86a, noteLight = 0xa8d898, band = 0xf6f0dc, case_ = 0x1e1e24, caseLight = 0x3a3a44;
    fill(x + 33, y, 12, 1, SHADOW);
    fill(x + 34, y - 11, 11, 6, case_); fill(x + 34, y - 11, 11, 1, caseLight); fill(x + 35, y - 10, 9, 4, 0x3a2a4a);
    fill(x + 34, y - 5, 11, 5, case_); fill(x + 34, y - 5, 11, 1, caseLight); dot(x + 39, y - 1, 0xc8a050);
    for (let k = 0; k < 3; k++) { const bx = x + 35 + k * 3; fill(bx, y - 6, 3, 2, note); fill(bx, y - 6, 3, 1, noteLight); dot(bx + 1, y - 6, band); dot(bx + 1, y - 5, band); }
    const t = (now + x * 0.31) % 14;
    if (t < 1.5) { const k = Math.floor(t * 6); fill(x + 42 + (k % 2), y - 12 + k * 2, 3, 1 + (k % 2), noteLight); }
  },
  /** A flip phone: a silver phone open on the desk, its green screen lit; every 18 s it rings, buzzing from side to side as the screen flashes. The screen glows at night. */
  gtaphone(x, y) {
    fill(x + 35, y, 9, 1, SHADOW);
    const t = (now + x * 0.37) % 18, ring = t < 2.5, bz = ring ? (Math.floor(now * 12) % 2 ? 1 : -1) : 0, px = x + 37 + bz;
    fill(px, y - 4, 5, 4, 0x9aa0aa); fill(px, y - 4, 5, 1, GTA_CHROME); for (let k = 0; k < 3; k++) { dot(px + 1, y - 3 + k, 0x4a4e58); dot(px + 3, y - 3 + k, 0x4a4e58); }
    fill(px, y - 11, 5, 7, 0x9aa0aa); fill(px, y - 11, 1, 7, GTA_CHROME); fill(px + 1, y - 10, 3, 4, ring && Math.floor(now * 4) % 2 ? 0xd8ffb0 : 0x7ac87a);
    dot(px + 2, y - 12, 0x4a4e58);
    if (ring) { dot(px - 2, y - 9, WHITE, 0.7); dot(px + 6, y - 9, WHITE, 0.7); dot(px - 2, y - 7, WHITE, 0.5); dot(px + 6, y - 7, WHITE, 0.5); }
    if (isNight) glowLate(() => glowAt(px + 2, y - 8, 4, 3, 0x7ac87a, ring ? 0.3 : 0.12));
  },
  /** A can of Sprunk: a green soda can with its white and yellow label, beads of cold on it; every 15 s the tab pops and a fizz of bubbles rises. */
  gtasprunk(x, y) {
    const can = 0x2ab84a, canLight = 0x5ad870, canDark = 0x1a7a30;
    fill(x + 36, y, 7, 1, SHADOW);
    fill(x + 37, y - 9, 5, 9, can); fill(x + 37, y - 9, 1, 9, canLight); fill(x + 41, y - 9, 1, 9, canDark);
    fill(x + 37, y - 10, 5, 1, GTA_CHROME); fill(x + 37, y - 1, 5, 1, GTA_CHROME_DARK);
    fill(x + 37, y - 6, 5, 2, WHITE); dot(x + 39, y - 6, 0xf2c43a); dot(x + 40, y - 5, 0xf2c43a);
    dot(x + 38, y - 8, WHITE, 0.7); dot(x + 40, y - 3, WHITE, 0.5);
    const t = (now + x * 0.33) % 15;
    if (t < 1.8) for (let k = 0; k < 3; k++) { const s = (t + k * 0.3) % 1; dot(x + 39 + (k - 1), y - 11 - Math.floor(s * 4), WHITE, 0.8 * (1 - s)); }
  },
  /** Fuzzy dice: a pair of fluffy pink dice with white spots hanging from a little chrome stand, swinging gently, and now and then knocked into a bigger swing. */
  gtadice(x, y) {
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 36, y - 1, 7, 1, GTA_CHROME_DARK); fill(x + 42, y - 12, 1, 11, GTA_CHROME); fill(x + 36, y - 12, 7, 1, GTA_CHROME);
    const t = (now + x * 0.27) % 12, big = t < 2, sw = Math.round(Math.sin(now * (big ? 6 : 2) + x) * (big ? 2 : 1));
    for (const [dx, len] of [[37, 4], [40, 6]]) {
      line(x + dx, y - 11, x + dx + sw, y - 11 + len, 0x2a2a30);
      const cx = x + dx + sw - 1, cy = y - 11 + len;
      fill(cx, cy, 3, 3, GTA_PINK); dot(cx, cy, 0xf8a0d0); dot(cx + 1, cy + 1, WHITE); dot(cx + 2, cy + 2, 0xb83a80);
    }
  },
  /** A toy sports car: a little yellow supercar with dark windows and chrome wheels; every 14 s it zips forward and back across the desk end, its headlights on at night. */
  gtatoycar(x, y) {
    const body = 0xf2c43a, bodyLight = 0xf8e07a, bodyDark = 0xb88a20;
    const t = (now + x * 0.29) % 14, zoom = t < 1.4 ? [0, 2, 4, 3, 1, -1, -2, 0][Math.floor(t * 6)] || 0 : 0, cx = x + 34 + zoom;
    fill(cx, y, 11, 1, SHADOW);
    fill(cx, y - 3, 11, 2, body); fill(cx + 3, y - 5, 5, 2, body); fill(cx, y - 3, 11, 1, bodyLight); fill(cx, y - 1, 11, 1, bodyDark);
    fill(cx + 4, y - 5, 3, 1, 0x2a3a4a); fill(cx + 3, y - 4, 5, 1, 0x3a4a5a);
    for (const wx of [cx + 2, cx + 8]) { fill(wx, y - 1, 2, 1, 0x1a1a1a); dot(wx, y - 1, GTA_CHROME); }
    dot(cx + 10, y - 3, 0xfff8d0); dot(cx, y - 3, 0xd83a3a);
    if (isNight) glowLate(() => { glowAt(cx + 12, y - 3, 3, 2, 0xfff0b0, 0.3); dot(cx + 10, y - 3, WHITE); });
  },
  /** A car radio: a chrome-faced radio with a tuning dial and a little display, a row of equaliser bars bouncing to the music and a pink light that glows at night. */
  gtaradio(x, y) {
    fill(x + 33, y, 12, 1, SHADOW);
    fill(x + 34, y - 7, 11, 7, 0x2a2a30); fill(x + 34, y - 7, 11, 1, GTA_CHROME); fill(x + 34, y - 1, 11, 1, GTA_CHROME_DARK);
    disc(x + 36, y - 4, 1.6, GTA_CHROME); dot(x + 36, y - 5, 0x2a2a30);
    fill(x + 38, y - 6, 6, 2, 0x1a3a3a); dot(x + 38 + Math.floor(now) % 6, y - 6, GTA_TEAL);
    for (let k = 0; k < 6; k++) { const h = 1 + Math.abs(Math.round(Math.sin(now * 5 + k * 1.7 + x) * 1.5)); fill(x + 38 + k, y - 1 - h, 1, h, k % 2 ? GTA_PINK : GTA_TEAL); }
    dot(x + 43, y - 4, GTA_PINK);
    if (isNight) glowLate(() => { glowAt(x + 40, y - 4, 6, 3, GTA_PINK, 0.15); dot(x + 43, y - 4, 0xffd0e8); });
  },
  /** A Burger Shot meal: a burger on its red and yellow wrapper and a tall drink cup with a striped straw; a wisp of steam curls off the burger now and then. */
  gtaburger(x, y) {
    fill(x + 33, y, 12, 1, SHADOW);
    fill(x + 33, y - 1, 8, 1, 0xd83a2a); fill(x + 33, y - 1, 3, 1, 0xf2c43a);
    fill(x + 34, y - 3, 6, 2, 0xc88a3a); fill(x + 34, y - 4, 6, 1, 0x5aa83a); fill(x + 34, y - 5, 6, 1, 0x7a3a1e); fill(x + 34, y - 7, 6, 2, 0xd8983a); fill(x + 35, y - 8, 4, 1, 0xd8983a); dot(x + 36, y - 7, 0xf8e8c0); dot(x + 38, y - 8, 0xf8e8c0);
    fill(x + 41, y - 8, 4, 8, 0xd83a2a); fill(x + 41, y - 5, 4, 2, 0xf2c43a); fill(x + 41, y - 8, 4, 1, WHITE); fill(x + 41, y - 8, 1, 8, 0xe85a4a);
    fill(x + 43, y - 12, 1, 4, WHITE); dot(x + 43, y - 11, 0xd83a2a); dot(x + 44, y - 12, WHITE);
    const t = (now + x * 0.31) % 10;
    if (t < 2) { const s = t / 2; dot(x + 37 + Math.round(Math.sin(t * 4)), y - 10 - Math.floor(s * 3), WHITE, 0.6 * (1 - s)); }
  },
  /** A toy police helicopter: black and white on a little landing pad, its rotor turning slowly; every 20 s it lifts off a little, rotor whirring, its searchlight sweeping the desk. */
  gtaheli(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 34, y - 1, 11, 1, 0x4a4e58); dot(x + 39, y - 1, 0xf2c43a);
    const t = (now + x * 0.37) % 20, lift = t < 3 ? Math.round(Math.min(1, t, 3 - t) * 3) : 0, hy = y - 2 - lift;
    fill(x + 36, hy - 4, 6, 4, 0x1a1a20); fill(x + 36, hy - 2, 6, 1, WHITE); fill(x + 41, hy - 4, 2, 2, 0x8ac8e8); fill(x + 33, hy - 3, 3, 1, 0x1a1a20); dot(x + 33, hy - 4, 0x1a1a20);
    fill(x + 36, hy, 6, 1, 0x6a6e78);
    fill(x + 38, hy - 5, 1, 1, 0x4a4e58);
    const spin = Math.floor(now * (lift ? 16 : 3)) % 2;
    if (spin) fill(x + 33, hy - 6, 11, 1, 0x4a4e58, lift ? 0.6 : 1); else fill(x + 36, hy - 6, 5, 1, 0x4a4e58);
    dot(x + 37, hy - 4, Math.floor(now * 3) % 2 ? 0xff3a4a : 0x3a6aff);
    if (lift) { for (let k = 0; k < 3; k++) dot(x + 42 + k, hy + 1 + k, 0xfff8d0, 0.5); }
  },
  /** An exotic engram: a faceted gold gem floating over a little dark stand, turning slowly as its lit facet moves round, bobbing; it glows gold, brighter at night. */
  destinyengram(x, y) {
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 37, y - 2, 5, 2, 0x2a3a4a); fill(x + 37, y - 2, 5, 1, DST_GOLD);
    const bob = Math.round(Math.sin(now * 1.8 + x));
    engram(x + 39, y - 7 + bob, 9, DST_EXOTIC, now * 1.5);
    if (isNight) glowLate(() => glowAt(x + 39, y - 7 + bob, 6, 6, DST_EXOTIC, 0.22));
  },
  /** A pile of Glimmer: a heap of little glowing blue cubes, sparkling here and there; it glows faintly at night. */
  destinyglimmer(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    const cubes = [[35, -2], [38, -2], [41, -2], [36, -4], [39, -4], [42, -3], [37, -6], [40, -6], [39, -8]];
    cubes.forEach(([cx, cy], i) => { fill(x + cx, y + cy, 2, 2, 0x4ab0f0); dot(x + cx, y + cy, 0xa8e0ff); dot(x + cx + 1, y + cy + 1, 0x2a6ab8); });
    const k = Math.floor(now * 3 + x) % 12; if (k < cubes.length) dot(x + cubes[k][0], y + cubes[k][1], WHITE);
    if (isNight) glowLate(() => glowAt(x + 39, y - 4, 6, 4, 0x4ab0f0, 0.16));
  },
  /** A Gjallarhorn model: the gold rocket launcher with its wolf's-head muzzle on a little black stand; a gleam runs along the gold every 12 s. */
  destinygjallarhorn(x, y) {
    const gold = 0xe0b040, goldLight = 0xf8d870, goldDark = 0x9a7020;
    fill(x + 33, y, 12, 1, SHADOW);
    fill(x + 37, y - 3, 1, 3, 0x2a2a30); fill(x + 41, y - 3, 1, 3, 0x2a2a30); fill(x + 35, y - 1, 9, 1, 0x2a2a30);
    fill(x + 34, y - 7, 11, 4, gold); fill(x + 34, y - 7, 11, 1, goldLight); fill(x + 34, y - 4, 11, 1, goldDark);
    fill(x + 42, y - 9, 3, 7, gold); dot(x + 44, y - 9, goldDark); dot(x + 43, y - 8, 0x2a2a30); fill(x + 44, y - 4, 1, 2, goldDark);
    fill(x + 33, y - 6, 1, 2, goldDark); fill(x + 37, y - 9, 3, 2, 0x4a4a54); fill(x + 38, y - 3, 2, 2, goldDark);
    const g = (now + x * 0.21) % 12;
    if (g < 1) { const gx = x + 34 + Math.floor(g * 11); dot(gx, y - 6, WHITE, 0.9); }
  },
  /** A spare Ghost shell on a pedestal: angular white plates round a blue eye; every 14 s the plates spread apart and turn a notch, the eye blinking, and settle back. The eye glows at night. */
  destinyshell(x, y) {
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 37, y - 3, 5, 3, 0x6a6e78); fill(x + 37, y - 3, 5, 1, 0x9aa0aa); fill(x + 38, y - 4, 3, 1, 0x4a4e58);
    const t = (now + x * 0.29) % 14, open = t < 1.5 ? 1 : 0;
    ghostShell(x + 39, y - 8, open, t < 1.5 ? Math.floor(t * 4) : 0);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 8, 3, 3, DST_ARC, 0.25); dot(x + 39, y - 8, 0xd8f4ff); });
  },
  /** A toy Sparrow: a little white and blue hoverbike floating over a round base, bobbing gently, its thruster glowing blue and flaring every 10 s. */
  destinysparrow(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 2, 7, 2, 0x2a3a4a); fill(x + 36, y - 2, 7, 1, DST_TEAL, 0.6);
    const bob = Math.round(Math.sin(now * 2.2 + x)), by = y - 7 + bob, flare = (now + x * 0.27) % 10 < 1;
    fill(x + 35, by, 9, 2, DST_WHITE); fill(x + 37, by - 1, 5, 1, DST_WHITE); fill(x + 35, by + 2, 9, 1, 0x3a5a9a); fill(x + 43, by - 2, 1, 3, 0x6a6e78);
    dot(x + 40, by - 2, 0x2a2a30); dot(x + 38, by, 0x3a5a9a);
    fill(x + 33, by, 2, 2, flare ? 0xbfe8ff : DST_ARC); if (flare) dot(x + 32, by + 1, DST_ARC);
    glowAt(x + 34, by + 1, 3, 2, DST_ARC, flare ? 0.3 : 0.12);
    if (isNight) glowLate(() => glowAt(x + 34, by + 1, 4, 3, DST_ARC, 0.25));
  },
  /** A Hunter helmet on a stand: dark grey with a sweeping blue visor and a fin on top; a glint slides across the visor every 13 s. */
  destinyhelmet(x, y) {
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 38, y - 3, 3, 3, 0x2a2a30); fill(x + 36, y - 1, 7, 1, 0x2a2a30);
    fill(x + 36, y - 10, 7, 7, 0x4a4e58); fill(x + 37, y - 11, 5, 1, 0x4a4e58); fill(x + 36, y - 10, 1, 7, 0x6a6e78); fill(x + 42, y - 9, 1, 6, 0x2e3038);
    fill(x + 39, y - 12, 1, 2, 0x6a6e78);
    fill(x + 37, y - 8, 5, 2, 0x3a7ab8); fill(x + 37, y - 8, 5, 1, 0x6ab0e8);
    fill(x + 38, y - 5, 3, 1, 0x2e3038);
    const g = (now + x * 0.23) % 13;
    if (g < 0.8) dot(x + 37 + Math.floor(g * 6), y - 8, WHITE);
    if (isNight) glowLate(() => glowAt(x + 39, y - 7, 3, 2, 0x6ab0e8, 0.15));
  },
  /** A Mote of Light: a bright white-gold orb with a little ring turning round it, held up in a glass bell on a dark base; it pulses gently and glows at night. */
  destinymote(x, y) {
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 36, y - 2, 7, 2, 0x2a3a4a); fill(x + 36, y - 2, 7, 1, DST_GOLD);
    const pulse = (Math.sin(now * 2 + x) + 1) / 2, a = now * 2;
    disc(x + 39, y - 7, 1.8, 0xfff4d0); dot(x + 39, y - 7, WHITE);
    ellipseRing(x + 39, y - 7, 3, 1, DST_GOLD, 0.8); dot(x + 39 + Math.round(Math.cos(a) * 3), y - 7 + Math.round(Math.sin(a)), WHITE);
    glowAt(x + 39, y - 7, 4, 4, 0xfff0c0, 0.12 + 0.1 * pulse);
    fill(x + 36, y - 11, 7, 9, 0xdff0f6, 0.25); fill(x + 36, y - 11, 1, 9, 0xdff0f6, 0.55); fill(x + 42, y - 11, 1, 9, 0xdff0f6, 0.55); fill(x + 37, y - 12, 5, 1, 0xdff0f6, 0.6);
    dot(x + 37, y - 10, WHITE, 0.8);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 7, 5, 5, 0xfff0c0, 0.25); dot(x + 39, y - 7, WHITE); });
  },
  /** A Strange Coin: a gold coin stamped with an eye, stood on a little stand; every 15 s it spins round, edge-on and back, and catches the light. */
  destinycoin(x, y) {
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 37, y - 2, 5, 2, 0x2a2a30); fill(x + 37, y - 2, 5, 1, 0x4a4a54);
    const t = (now + x * 0.31) % 15, w = t < 1.6 ? [7, 5, 3, 1, 3, 5, 7, 7][Math.floor(t * 5)] || 7 : 7;
    const cx = x + 39, cy = y - 6;
    const half = Math.floor(w / 2);
    fill(cx - half, cy - 3, w, 7, 0xd8a838); fill(cx - half, cy - 3, w, 1, 0xf8d870); if (w > 2) { fill(cx - half + 1, cy - 4, w - 2, 1, 0xd8a838); fill(cx - half + 1, cy + 4, w - 2, 1, 0x9a7020); }
    if (w >= 5) { fill(cx - 1, cy, 3, 1, 0x6a4a1a); dot(cx, cy, 0x2a5a4a); dot(cx - 2, cy, 0x9a7020); dot(cx + 2, cy, 0x9a7020); }
    if (t < 1.6 && w === 1) dot(cx, cy - 4, WHITE);
  },
  /** A starfighter model on a stand: a white fuselage with red stripes and a canopy, its four wings in an X with cannons at their tips; every 20 s the wings open wider and the engines glow orange. */
  starwarsxwing(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 1, 7, 1, 0x4a4e58); fill(x + 39, y - 5, 1, 4, 0x8a909c);
    const t = (now + x * 0.27) % 20, open = t < 4 ? 1 : 0;
    // The wings and their cannons, behind the fuselage
    line(x + 35, y - 8, x + 37, y - 10 - open, 0xb8bcc4); line(x + 35, y - 6, x + 37, y - 4 + open, 0xb8bcc4);
    fill(x + 36, y - 10 - open, 6, 1, 0xd0d4dc); fill(x + 36, y - 4 + open, 6, 1, 0xa8acb4);
    dot(x + 38, y - 10 - open, 0xd8402a); dot(x + 38, y - 4 + open, 0xd8402a);
    // The fuselage, nose to the right
    fill(x + 34, y - 8, 9, 2, SW_WHITE); fill(x + 34, y - 8, 9, 1, WHITE); fill(x + 43, y - 7, 2, 1, 0xd0d4dc);
    fill(x + 37, y - 7, 3, 1, 0xd8402a); fill(x + 38, y - 9, 2, 1, 0x3a4a5a); dot(x + 38, y - 9, 0x8ab8d8);
    const glow = open ? 0xffa060 : 0xd86a3a;
    dot(x + 34, y - 9, glow); dot(x + 34, y - 6, glow);
    if (open) glowAt(x + 34, y - 7, 3, 3, 0xff8040, 0.2);
  },
  /** A dark lord's helmet on a pedestal: a black dome flaring at the foot, the triangle mask with its grille and dark lenses; every 15 s a red glint slides across the lenses. */
  starwarsvaderbust(x, y) {
    const black = 0x18181e, light = 0x34343e, dark = 0x0c0c10;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 2, 7, 2, 0x2a2a30); fill(x + 36, y - 2, 7, 1, 0x44444e);
    fill(x + 37, y - 12, 5, 1, black); fill(x + 36, y - 11, 7, 4, black); fill(x + 35, y - 7, 9, 3, black); fill(x + 35, y - 4, 2, 1, black); fill(x + 42, y - 4, 2, 1, black);
    fill(x + 37, y - 12, 2, 1, light); dot(x + 36, y - 11, light); dot(x + 35, y - 7, light); fill(x + 43, y - 7, 1, 4, dark);
    // The mask: lenses, the nose ridge and the grille
    fill(x + 37, y - 9, 2, 1, 0x3a2a30); fill(x + 40, y - 9, 2, 1, 0x3a2a30); dot(x + 39, y - 8, 0x5a5e68);
    fill(x + 38, y - 6, 3, 2, 0x4a4e58); dot(x + 38, y - 5, black); dot(x + 40, y - 5, black);
    dot(x + 37, y - 10, WHITE, 0.5);
    const g = (now + x * 0.31) % 15;
    if (g < 1) dot(x + 37 + Math.floor(g * 5), y - 9, 0xff4a3a);
  },
  /** A lightsaber on a stand: a silver hilt with a black grip and a red button standing in a cradle; every 18 s the green blade hisses up for four seconds, glowing, and drops back. */
  starwarssaber(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 2, 7, 2, 0x3a3d46); fill(x + 36, y - 2, 7, 1, 0x5a5e68); fill(x + 38, y - 3, 1, 1, 0x3a3d46); fill(x + 41, y - 3, 1, 1, 0x3a3d46);
    fill(x + 39, y - 7, 2, 5, 0xb8bcc4); dot(x + 39, y - 7, 0xd8dce4); fill(x + 39, y - 5, 2, 2, 0x26262c); dot(x + 40, y - 4, 0xd8402a);
    fill(x + 38, y - 8, 4, 1, 0xd0d4dc);
    const t = (now + x * 0.33) % 18;
    if (t < 4) {
      const len = Math.min(6, Math.floor(Math.min(t, 4 - t) * 30));
      if (len > 0) { swBlade(x + 39, y - 9, x + 39, y - 8 - len, 0x4aff6a); fill(x + 40, y - 8 - len, 1, len, 0x4aff6a, 0.6); }
    }
  },
  /** A holocron: a small glowing blue cube in a gold frame on a gold stand, turning a face then an edge to you, bobbing a pixel; its glow breathes, brighter at night. */
  starwarsholocron(x, y) {
    const gold = 0xc8a050, goldLight = 0xe8c878, blue = 0x5ab8ff;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 2, 5, 2, gold); fill(x + 37, y - 2, 5, 1, goldLight); fill(x + 38, y - 3, 3, 1, gold);
    const bob = Math.round(Math.sin(now * 0.8 + x) * 0.6), cy = y - 7 + bob, edge = Math.floor((now + x * 0.2) / 3) % 2;
    const a = 0.6 + 0.3 * Math.sin(now * 1.3 + x);
    if (edge) { fill(x + 38, cy - 2, 3, 5, blue); fill(x + 37, cy - 1, 5, 3, blue); dot(x + 39, cy - 3, gold); dot(x + 39, cy + 3, gold); dot(x + 36, cy, gold); dot(x + 42, cy, gold); dot(x + 39, cy, 0xd8f0ff); }
    else { fill(x + 37, cy - 2, 5, 5, blue); for (const [dx, dy] of [[0, -2], [4, -2], [0, 2], [4, 2]]) dot(x + 37 + dx, cy + dy, gold); fill(x + 38, cy - 1, 3, 3, 0x9ad8ff); dot(x + 39, cy, 0xe8f8ff); }
    glowAt(x + 39, cy, 5, 5, blue, 0.08 * a);
    if (isNight) glowLate(() => { glowAt(x + 39, cy, 6, 6, blue, 0.22 * a); dot(x + 39, cy, 0xe8f8ff); });
  },
  /** A freighter model on a stand: a round grey hull with panel lines, two mandibles at the front and the cockpit tube off to one side, a blue engine strip at the back that flares for a jump every 22 s. */
  starwarsfalcon(x, y) {
    const hull = 0xb8bcc4, light = 0xd8dce4, dark = 0x7a7e88;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 1, 7, 1, 0x4a4e58); fill(x + 39, y - 4, 1, 3, 0x8a909c);
    fill(x + 36, y - 9, 6, 1, light); fill(x + 35, y - 8, 8, 3, hull); fill(x + 36, y - 5, 6, 1, dark);
    fill(x + 43, y - 8, 2, 1, hull); fill(x + 43, y - 6, 2, 1, dark); fill(x + 42, y - 7, 1, 1, 0x5a5e68);
    fill(x + 41, y - 5, 3, 1, hull); dot(x + 44, y - 5, 0x6ab8d8);
    dot(x + 37, y - 7, dark); dot(x + 39, y - 6, dark); dot(x + 40, y - 8, dark); dot(x + 38, y - 10, light); dot(x + 37, y - 10, 0x8a909c);
    const t = (now + x * 0.29) % 22, jump = t < 1.6;
    fill(x + 34, y - 8, 1, 3, jump ? 0xe8f8ff : 0x6ad8ff);
    glowAt(x + 34, y - 7, jump ? 5 : 3, 3, 0x6ad8ff, jump ? 0.3 : 0.08);
    if (isNight) glowLate(() => glowAt(x + 34, y - 7, 3, 3, 0x6ad8ff, jump ? 0.3 : 0.14));
  },
  /** A trooper helmet left on the desk: a white dome with dark lenses, the grey frown of its grille and a dark stripe down the side; a glint crosses the dome every 20 s. */
  starwarstrooper(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 10, 6, 1, SW_WHITE); fill(x + 36, y - 9, 8, 5, SW_WHITE); fill(x + 37, y - 4, 6, 3, SW_WHITE); fill(x + 36, y - 2, 8, 2, 0xd0d4dc);
    fill(x + 36, y - 9, 1, 5, WHITE); fill(x + 43, y - 9, 1, 6, 0xb8bcc8);
    fill(x + 37, y - 7, 2, 2, 0x16161c); fill(x + 41, y - 7, 2, 2, 0x16161c); dot(x + 37, y - 5, 0x16161c); dot(x + 42, y - 5, 0x16161c);
    fill(x + 38, y - 3, 4, 1, 0x8a909c); dot(x + 39, y - 4, 0x5a5e68); dot(x + 40, y - 4, 0x5a5e68);
    fill(x + 36, y - 6, 1, 3, 0x4a5a7a);
    const g = (now + x * 0.23) % 20;
    if (g < 1) dot(x + 37 + Math.floor(g * 6), y - 9, WHITE); else dot(x + 38, y - 9, WHITE, 0.7);
  },
  /** A tall glass of blue milk on a coaster, a pale froth on top and a straw; every 9 s a bubble rises through it. */
  starwarsbluemilk(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 1, 7, 1, 0x8a5a32);
    fill(x + 38, y - 10, 4, 9, 0xdff0f6, 0.4); fill(x + 38, y - 10, 1, 9, 0xdff0f6, 0.7); fill(x + 41, y - 10, 1, 9, 0xdff0f6, 0.6);
    fill(x + 38, y - 8, 4, 7, 0x7ac8f0); fill(x + 38, y - 8, 1, 7, 0x9ad8f8); fill(x + 41, y - 8, 1, 7, 0x5aa8d8);
    fill(x + 38, y - 9, 4, 1, 0xd8f0ff);
    line(x + 40, y - 9, x + 42, y - 12, 0xf28fb0);
    const t = (now + x * 0.41) % 9;
    if (t < 2) dot(x + 39 + (Math.floor(t * 3) % 2), y - 2 - Math.floor(t * 3), 0xe8f8ff, 0.8);
    dot(x + 38, y - 7, WHITE, 0.6);
  },
  /** A gold lipstick, uncapped, its hot pink bullet out, the cap lying beside it; every 16 s the bullet twists up a pixel and back. */
  girly1lipstick(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    const up = (now + x * 0.31) % 16 < 2 ? 1 : 0;
    fill(x + 37, y - 6, 4, 6, G1_GOLD); fill(x + 37, y - 6, 1, 6, G1_GOLD_LIGHT); fill(x + 40, y - 6, 1, 6, G1_GOLD_DARK); fill(x + 37, y - 3, 4, 1, G1_GOLD_DARK);
    fill(x + 38, y - 9 - up, 2, 3 + up, G1_PINK); dot(x + 38, y - 10 - up, G1_PINK_LIGHT); dot(x + 39, y - 9 - up, G1_PINK_DARK); dot(x + 38, y - 8, G1_PINK_LIGHT);
    fill(x + 41, y - 2, 4, 2, G1_GOLD); fill(x + 41, y - 2, 4, 1, G1_GOLD_LIGHT); dot(x + 44, y - 1, G1_GOLD_DARK);
  },
  /** A perfume bottle: faceted pink glass with a gold collar and a tasselled bulb atomiser; every 14 s the bulb squeezes and a little pink mist puffs out. */
  girly1perfume(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 6, 6, 6, 0xf890c4, 0.75); fill(x + 37, y - 7, 4, 1, 0xf890c4, 0.75); fill(x + 36, y - 6, 1, 6, 0xffc0e0, 0.9); fill(x + 41, y - 6, 1, 6, G1_PINK, 0.8);
    fill(x + 37, y - 4, 4, 3, G1_PINK, 0.55); dot(x + 37, y - 5, WHITE, 0.8);
    fill(x + 38, y - 9, 2, 2, G1_GOLD); dot(x + 38, y - 9, G1_GOLD_LIGHT);
    const t = (now + x * 0.29) % 14, squeeze = t < 1.2;
    line(x + 39, y - 9, x + 42, y - 10, G1_GOLD_DARK); fill(x + 42, y - 11, squeeze ? 2 : 3, squeeze ? 2 : 3, G1_PINK_DARK); dot(x + 43, y - 8, G1_GOLD);
    if (t < 2.5) for (let k = 0; k < 3; k++) dot(x + 36 - Math.floor(t * 2) - k, y - 10 - (k % 2), 0xffb0d8, 0.6 - t * 0.2);
  },
  /** An open compact: a round gold case with pink powder in the lower half and a mirror in the lid; every 12 s light glints across the mirror. */
  girly1compact(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 3, 8, 3, G1_GOLD); fill(x + 37, y - 4, 6, 1, G1_GOLD); fill(x + 37, y - 3, 6, 2, 0xf4a0c0); dot(x + 38, y - 3, 0xfac0d8);
    fill(x + 36, y, 8, 1, G1_GOLD_DARK, 0.0);
    fill(x + 37, y - 11, 6, 7, G1_GOLD); fill(x + 36, y - 10, 8, 5, G1_GOLD); fill(x + 37, y - 10, 6, 5, 0xdce8ee); fill(x + 38, y - 11, 4, 1, 0xdce8ee);
    dot(x + 36, y - 10, G1_GOLD_LIGHT); g1Mark('smallheart', x + 39, y - 9, 0xf8b8d4, 0.5);
    const g = (now + x * 0.33) % 12;
    if (g < 1) line(x + 37 + Math.floor(g * 6), y - 6, x + 38 + Math.floor(g * 6), y - 10, WHITE, 0.9); else dot(x + 38, y - 10, WHITE, 0.7);
  },
  /** A pink princess phone: a curvy hot pink base with a white rotary dial, the handset in its cradle; every 30 s it rings for three seconds, the handset jiggling and little rings rising. */
  girly1phone(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 5, 8, 5, G1_PINK_LIGHT); fill(x + 37, y - 6, 6, 1, G1_PINK_LIGHT); fill(x + 36, y - 5, 1, 5, 0xf890c4); fill(x + 43, y - 5, 1, 5, G1_PINK); fill(x + 36, y - 1, 8, 1, G1_PINK_DARK);
    disc(x + 40, y - 3, 1.6, G1_WHITE); dot(x + 40, y - 3, G1_PINK);
    const t = (now + x * 0.23) % 30, ring = t < 3, j = ring ? Math.floor(now * 12) % 2 : 0;
    fill(x + 35, y - 8 - j, 10, 2, G1_PINK_LIGHT); fill(x + 35, y - 8 - j, 10, 1, 0xf890c4); fill(x + 35, y - 7 - j, 2, 2, G1_PINK); fill(x + 43, y - 7 - j, 2, 2, G1_PINK);
    if (ring) for (let k = 0; k < 2; k++) { const r = ((now * 2 + k * 0.5) % 1) * 3; dot(x + 34 - Math.round(r), y - 10 - Math.round(r), G1_PINK_DARK, 0.8); dot(x + 46 + Math.round(r) - 1, y - 10 - Math.round(r), G1_PINK_DARK, 0.8); }
  },
  /** Three nail polishes: pink, red and a gold glitter, square bottles with tall black caps; the glitter one twinkles every 9 s. */
  girly1polish(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    [[35, G1_PINK_LIGHT], [39, 0xd8202a], [42, G1_GOLD]].forEach(([dx, c], i) => {
      const h = i === 1 ? 1 : 0;
      fill(x + dx, y - 4 - h, 3, 4 + h, c, 0.95); dot(x + dx, y - 4 - h, WHITE, 0.6); fill(x + dx + 2, y - 4 - h, 1, 4 + h, shaded(c, 0.75));
      fill(x + dx + 1, y - 8 - h, 1, 4, 0x1a1a20); dot(x + dx + 1, y - 8 - h, 0x4a4a54);
    });
    const t = (now + x * 0.37) % 9;
    if (t < 1) g1Twinkle(x + 43, y - 3, t); else { dot(x + 43, y - 2, G1_GOLD_LIGHT); dot(x + 42, y - 3, WHITE, 0.6); }
  },
  /** A single pink stiletto standing on the desk: a pointed toe, a high arch and a gold spike heel; a sparkle catches the toe every 11 s. */
  girly1heel(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 5, 2, G1_PINK); fill(x + 35, y - 2, 5, 1, G1_PINK_LIGHT); dot(x + 34, y - 1, G1_PINK);
    line(x + 40, y - 2, x + 43, y - 6, G1_PINK); line(x + 40, y - 3, x + 43, y - 7, G1_PINK_LIGHT);
    fill(x + 43, y - 7, 2, 2, G1_PINK); fill(x + 41, y - 5, 2, 1, G1_PINK_DARK);
    fill(x + 44, y - 5, 1, 5, G1_GOLD); dot(x + 44, y - 1, G1_GOLD_DARK);
    const t = (now + x * 0.3) % 11;
    if (t < 1) g1Twinkle(x + 36, y - 3, t);
  },
  /** A tiara on a pink velvet cushion: a gold band rising to three peaks set with pink gems that twinkle one after another. */
  girly1tiara(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 3, 10, 3, G1_PINK_DARK); fill(x + 36, y - 4, 8, 1, G1_PINK); fill(x + 35, y - 3, 10, 1, G1_PINK); dot(x + 35, y - 1, 0x801050); dot(x + 44, y - 1, 0x801050);
    fill(x + 36, y - 5, 8, 1, G1_GOLD);
    for (const [dx, h] of [[37, 2], [40, 4], [43, 2]]) { fill(x + dx - 1, y - 5 - h, 1, h, G1_GOLD); dot(x + dx - (dx === 40 ? 1 : 0), y - 6 - h, G1_GOLD_LIGHT); }
    dot(x + 38, y - 7, G1_GOLD); dot(x + 41, y - 7, G1_GOLD);
    const k = Math.floor((now + x * 0.2) * 1.5) % 6;
    [[36, -8], [39, -10], [42, -8]].forEach(([dx, dy], i) => { dot(x + dx, y + dy, 0xff5ab8); if (k === i) g1Twinkle(x + dx, y + dy, (now * 1.5) % 1); });
  },
  /** A pink cupcake on a gold plate: a striped paper case, a swirl of pink frosting with sprinkles and a cherry on top; now and then a sprinkle glints. */
  girly1cupcake(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 1, 9, 1, G1_GOLD); fill(x + 35, y - 1, 9, 1, G1_GOLD_LIGHT, 0.5);
    fill(x + 37, y - 5, 5, 4, G1_WHITE); for (const dx of [37, 39, 41]) fill(x + dx, y - 5, 1, 4, G1_PINK_LIGHT);
    fill(x + 36, y - 7, 7, 2, 0xf8a8cc); fill(x + 37, y - 9, 5, 2, 0xf8a8cc); fill(x + 38, y - 10, 3, 1, 0xf8a8cc);
    fill(x + 36, y - 6, 7, 1, 0xe880b0); dot(x + 37, y - 9, 0xffd0e8); dot(x + 36, y - 7, 0xffd0e8);
    disc(x + 40, y - 11, 1.2, 0xd8202a); dot(x + 40, y - 12, 0xff8080);
    [[37, -7, 0x6ab8ff], [40, -8, G1_GOLD], [41, -6, 0x5ad88a], [39, -6, WHITE]].forEach(([dx, dy, c]) => dot(x + dx, y + dy, c));
    const t = (now + x * 0.27) % 8;
    if (t < 1) g1Twinkle(x + 40, y - 8, t, G1_GOLD_LIGHT);
  },
  /** Bubble tea: a clear cup of lilac milk tea with dark pearls at the bottom, a domed lid and a fat pink straw; every 10 s the pearls shuffle. */
  girly2boba(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 8, 6, 8, 0xdff0f6, 0.45); fill(x + 37, y - 7, 6, 7, 0xd8c0e8); fill(x + 37, y - 7, 1, 7, 0xe8d8f4); fill(x + 42, y - 7, 1, 7, 0xb8a0d0);
    const sh = Math.floor((now + x * 0.3) / 10) % 2;
    for (const [dx, dy] of [[1, -1], [3, -1], [2, -2], [4, -2], [1, -3]]) dot(x + 37 + dx + (sh && dy === -2 ? -1 : 0), y + dy, 0x3a2a2a);
    fill(x + 36, y - 9, 8, 1, 0xdff0f6, 0.8); fill(x + 37, y - 10, 6, 1, 0xdff0f6, 0.6);
    fill(x + 41, y - 13, 2, 5, G2_PINK_DARK); dot(x + 41, y - 13, G2_PINK);
    g2Kitty(x + 40, y - 4, 1, false, 0.0);
    dot(x + 38, y - 6, WHITE, 0.6);
  },
  /** A carton of strawberry milk: pink with a white top, a red strawberry and a little smiling face that blinks every 7 s, a straw poking out. */
  girly2milk(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 8, 6, 8, G2_PINK); fill(x + 37, y - 8, 1, 8, 0xfadce6); fill(x + 42, y - 8, 1, 8, G2_PINK_DARK);
    fill(x + 37, y - 10, 6, 2, WHITE); fill(x + 38, y - 11, 4, 1, WHITE); fill(x + 42, y - 10, 1, 2, 0xe0d8dc);
    fill(x + 38, y - 3, 3, 2, 0xe8304a); dot(x + 39, y - 4, 0x5ab86a); dot(x + 38, y - 3, 0xffd0d0);
    const blink = (now + x * 0.21) % 7 < 0.25;
    if (blink) { dot(x + 38, y - 6, 0x8a6a7a); dot(x + 41, y - 6, 0x8a6a7a); } else { dot(x + 38, y - 6, 0x1f1a1a); dot(x + 41, y - 6, 0x1f1a1a); }
    dot(x + 39, y - 5, 0x1f1a1a, 0.7); dot(x + 40, y - 5, 0x1f1a1a, 0.7);
    line(x + 41, y - 11, x + 43, y - 14, 0xbce4f8);
  },
  /** A little kitty plush sitting on the desk in blue overalls with her red bow; every 12 s she tilts her head. */
  girly2plush(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 4, 5, 4, 0x8ab4e8); fill(x + 37, y - 4, 1, 4, 0xa8c8f0); dot(x + 36, y - 3, 0xfffcfa); dot(x + 42, y - 3, 0xfffcfa); fill(x + 37, y - 1, 2, 1, 0xfffcfa); fill(x + 40, y - 1, 2, 1, 0xfffcfa);
    const tilt = (now + x * 0.33) % 12 < 1.5 ? 1 : 0;
    g2Kitty(x + 39 + tilt, y - 7, 1);
  },
  /** A pencil cup in pastel stripes holding pastel pencils and a star wand whose star twinkles every 9 s. */
  girly2pencils(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 6, 6, 6, G2_CREAM); for (const [dy, c] of [[-5, G2_PINK], [-3, G2_MINT], [-1, G2_LILAC]]) fill(x + 37, y + dy, 6, 1, c); fill(x + 42, y - 6, 1, 6, 0xe0d8d0, 0.6);
    [[38, 4, G2_PINK_DARK], [39, 6, G2_MINT_DARK], [41, 5, G2_LILAC_DARK]].forEach(([dx, h, c]) => { fill(x + dx, y - 6 - h, 1, h, c); dot(x + dx, y - 7 - h, 0xf0d8b8); dot(x + dx, y - 8 - h, 0x3a3a40); });
    line(x + 40, y - 6, x + 42, y - 11, 0xfffcfa);
    for (const [dx, dy] of [[0, 0], [-1, 0], [1, 0], [0, -1], [0, 1]]) dot(x + 42 + dx, y - 12 + dy, G2_NOSE);
    const t = (now + x * 0.29) % 9;
    if (t < 1) g1Twinkle(x + 42, y - 12, t);
  },
  /** A little tower of macarons on a cake stand: pink, mint, lilac and lemon, each with its cream filling. */
  girly2macarons(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 1, 8, 1, WHITE); fill(x + 39, y - 2, 2, 1, WHITE);
    [[0xf8b8c8, 35, -4], [G2_MINT, 40, -4], [G2_LILAC, 38, -7], [0xfff0a8, 38, -10]].forEach(([c, dx, dy]) => {
      fill(x + dx, y + dy, 4, 1, c); fill(x + dx, y + dy + 1, 4, 1, G2_CREAM); fill(x + dx, y + dy + 2, 4, 1, shaded(c, 0.9)); dot(x + dx, y + dy, shaded(c, 1.1));
    });

  },
  /** A mushroom lamp: a round pink cap with white spots on a cream stem; it glows warm, softly by day and brightly at night, breathing slowly. */
  girly2mushroom(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 38, y - 5, 4, 5, G2_CREAM); fill(x + 38, y - 5, 1, 5, WHITE); fill(x + 41, y - 5, 1, 5, 0xe0d4c0); fill(x + 37, y - 1, 6, 1, 0xe0d4c0);
    fill(x + 36, y - 8, 8, 3, 0xf490b0); fill(x + 37, y - 9, 6, 1, 0xf490b0); fill(x + 38, y - 10, 4, 1, 0xf490b0); fill(x + 36, y - 5, 8, 1, 0xd87090);
    dot(x + 38, y - 8, WHITE); dot(x + 41, y - 9, WHITE); dot(x + 42, y - 7, WHITE); dot(x + 37, y - 7, WHITE, 0.8);
    const a = 0.7 + 0.3 * Math.sin(now * 0.9 + x);
    glowAt(x + 40, y - 6, 6, 5, 0xffd0a0, 0.07 * a);
    if (isNight) glowLate(() => { glowAt(x + 40, y - 6, 8, 7, 0xffc8a0, 0.22 * a); fill(x + 38, y - 4, 4, 1, 0xfff0d8, 0.8); });
  },
  /** Lilac cat-ear headphones lying on the desk, the band arched over two round cups, little ears on top whose pink lights pulse every few seconds and glow at night. */
  girly2headphones(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    line(x + 36, y - 3, x + 37, y - 7, G2_LILAC); line(x + 37, y - 7, x + 42, y - 7, G2_LILAC); line(x + 42, y - 7, x + 43, y - 3, G2_LILAC); dot(x + 38, y - 7, 0xe0d4f6);
    const a = 0.4 + 0.6 * (Math.floor((now + x * 0.2) * 2) % 4 === 0 ? 1 : 0.3);
    fill(x + 37, y - 9, 2, 2, G2_LILAC); dot(x + 37, y - 10, G2_LILAC); dot(x + 38, y - 8, 0xff8ac0, a);
    fill(x + 41, y - 9, 2, 2, G2_LILAC); dot(x + 42, y - 10, G2_LILAC); dot(x + 41, y - 8, 0xff8ac0, a);
    fill(x + 35, y - 4, 3, 4, G2_LILAC_DARK); fill(x + 42, y - 4, 3, 4, G2_LILAC_DARK); dot(x + 35, y - 4, G2_LILAC); dot(x + 42, y - 4, G2_LILAC);
    dot(x + 36, y - 2, G2_PINK); dot(x + 43, y - 2, G2_PINK);
    if (isNight) glowLate(() => { glowAt(x + 40, y - 9, 5, 2, 0xff8ac0, 0.15 * a); });
  },
  /** A smiling rice ball: a white triangle with a nori band, rosy cheeks and a tiny smile, on a little leaf; it blinks every 6 s. */
  girly2onigiri(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 1, 10, 1, 0x6ac88a); dot(x + 44, y - 2, 0x6ac88a);
    for (let r = 0; r < 8; r++) fill(x + 40 - Math.floor(r / 2) - 1, y - 9 + r, 2 + Math.floor(r / 2) * 2, 1, 0xfffcfa);
    dot(x + 39, y - 9, 0xfffcfa); fill(x + 36, y - 2, 8, 1, 0xe8e4e8);
    fill(x + 38, y - 4, 4, 3, 0x1e3a2a); dot(x + 38, y - 4, 0x2e5a3e);
    const blink = (now + x * 0.25) % 6 < 0.2;
    dot(x + 38, y - 6, 0x1f1a1a, blink ? 0.3 : 1); dot(x + 41, y - 6, 0x1f1a1a, blink ? 0.3 : 1); dot(x + 39, y - 5, 0x8a4a4a); dot(x + 40, y - 5, 0x8a4a4a, 0.0);
    dot(x + 37, y - 5, G2_PINK_DARK, 0.8); dot(x + 42, y - 5, G2_PINK_DARK, 0.8);
  },
  /** Two stacks of cash, banded; every 12 s the top bill riffles up and flops back. */
  narcoscash(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    ncStack(x + 35, y - 4, 6, 4); ncStack(x + 39, y - 8, 6, 4); ncStack(x + 36, y - 8, 0, 0);
    const t = (now + x * 0.31) % 12;
    if (t < 1) { const up = [1, 2, 3, 2, 1][Math.floor(t * 5)]; fill(x + 40, y - 9 - up, 5, 1, NC_CASH_LIGHT); dot(x + 44, y - 9 - up, NC_CASH); }
  },
  /** A taped brick: a pale package wrapped in brown tape crossed over it with a little red star stamp; every 20 s a tiny white puff escapes from a corner. */
  narcosbrick(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 5, 10, 5, 0xe8e0d0); fill(x + 35, y - 5, 10, 1, 0xf8f4ec); fill(x + 44, y - 5, 1, 5, 0xc8c0b0); fill(x + 35, y - 1, 10, 1, 0xc8c0b0);
    fill(x + 35, y - 3, 10, 1, 0xa87a4a); fill(x + 39, y - 5, 2, 5, 0xa87a4a); dot(x + 39, y - 5, 0xc89a6a);
    dot(x + 37, y - 4, 0xd8202a); dot(x + 36, y - 4, 0xd8202a, 0.6); dot(x + 38, y - 4, 0xd8202a, 0.6);
    const t = (now + x * 0.23) % 20;
    if (t < 1.5) { dot(x + 45, y - 6 - Math.floor(t * 3), WHITE, 0.7 - t * 0.4); dot(x + 46, y - 7 - Math.floor(t * 3), WHITE, 0.5 - t * 0.3); }
  },
  /** A brick phone: a big grey handset standing on the desk with a black keypad, a green screen and a stubby antenna; every 25 s it rings, the screen flashing and a red light blinking on the antenna. */
  narcosphone(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 38, y - 10, 4, 10, 0x4a4a52); fill(x + 38, y - 10, 1, 10, 0x6a6a74); fill(x + 41, y - 10, 1, 10, 0x2a2a30);
    fill(x + 39, y - 13, 1, 3, 0x2a2a30);
    const t = (now + x * 0.27) % 25, ring = t < 3, flash = ring && Math.floor(now * 6) % 2 === 0;
    fill(x + 39, y - 8, 2, 2, flash ? 0xc8ff9a : 0x7aa86a);
    for (let r = 0; r < 3; r++) { dot(x + 39, y - 5 + r * 2 - 1, 0x1a1a20); dot(x + 40, y - 5 + r * 2 - 1, 0x1a1a20); }
    dot(x + 39, y - 13, ring && flash ? 0xff4a3a : 0x5a2a20);
    if (ring) { dot(x + 36, y - 9 - (Math.floor(now * 4) % 2), 0x3a3a40, 0.7); dot(x + 43, y - 9 - (Math.floor(now * 4 + 1) % 2), 0x3a3a40, 0.7); }
  },
  /** A brass balance scale: a post with a beam and two hanging pans; every 15 s the beam tips one way, then the other, and settles. */
  narcosscale(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    const brass = 0xc8a050, light = 0xe8c878, dark = 0x8a6a2a;
    fill(x + 37, y - 1, 6, 1, dark); fill(x + 39, y - 10, 2, 9, brass); fill(x + 39, y - 10, 1, 9, light);
    const t = (now + x * 0.29) % 15, tip = t < 2 ? [1, 0, -1, 0][Math.floor(t * 2)] : 0;
    line(x + 35, y - 10 + tip, x + 44, y - 10 - tip, brass); dot(x + 39, y - 11, light);
    for (const [px, d] of [[35, tip], [44, -tip]]) {
      line(x + px, y - 10 + d, x + px - 1, y - 6 + d, dark, 0.8); line(x + px, y - 10 + d, x + px + 1, y - 6 + d, dark, 0.8);
      fill(x + px - 2, y - 5 + d, 5, 1, brass); dot(x + px - 2, y - 5 + d, light);
    }
    dot(x + 35, y - 6 + tip, WHITE, 0.9); dot(x + 34, y - 6 + tip, WHITE, 0.7);
  },
  /** A fat cigar resting in a heavy glass ashtray, its tip glowing (brighter at night) and a thin curl of smoke rising and drifting. */
  narcoscigar(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, 0xc8e0e8, 0.6); fill(x + 35, y - 2, 9, 1, 0xe8f4f8, 0.7); fill(x + 37, y - 2, 5, 1, 0x6a6a6a, 0.6);
    line(x + 36, y - 4, x + 42, y - 3, 0x7a4a2a); line(x + 36, y - 5, x + 42, y - 4, 0x9a6a3a); dot(x + 40, y - 4, NC_GOLD); dot(x + 40, y - 5, 0xd8202a);
    const f = candlelight(x * 0.2); dot(x + 36, y - 4, 0xff7a3a, f); dot(x + 36, y - 5, 0x8a8a8a);
    for (let k = 0; k < 4; k++) { const ph = (now * 0.6 + k * 0.25) % 1; dot(x + 35 + Math.round(Math.sin(ph * 6 + k) * 1.5), y - 6 - Math.floor(ph * 7), 0xd8d8d8, 0.5 * (1 - ph)); }
    if (isNight) glowLate(() => { glowAt(x + 36, y - 4, 3, 2, 0xff7a3a, 0.2 * f); dot(x + 36, y - 4, 0xffb070); });
  },
  /** A bottle of dark rum with a gold label beside a tumbler of rum on ice; every 10 s the ice shifts with a glint. */
  narcosrum(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 8, 4, 8, 0x6a2a10); fill(x + 36, y - 11, 2, 3, 0x6a2a10); fill(x + 36, y - 12, 2, 1, 0x1a1a20); fill(x + 35, y - 8, 1, 8, 0x9a4a20);
    fill(x + 35, y - 6, 4, 3, NC_GOLD); fill(x + 36, y - 5, 2, 1, 0x3a2a1a);
    fill(x + 40, y - 5, 5, 5, 0xdff0f6, 0.4); fill(x + 40, y - 3, 5, 3, 0xc87a2a, 0.85); fill(x + 40, y - 5, 1, 5, 0xdff0f6, 0.7);
    const sh = Math.floor((now + x * 0.3) / 10) % 2;
    fill(x + 41 + sh, y - 4, 2, 2, 0xe8f8ff, 0.8); dot(x + 43 - sh, y - 3, 0xe8f8ff, 0.7);
    if ((now + x * 0.3) % 10 < 0.6) dot(x + 42, y - 5, WHITE);
  },
  /** A black pager lying on the desk with a green screen; every 18 s it buzzes, shaking side to side, its screen lit with a number. */
  narcospager(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    const t = (now + x * 0.25) % 18, buzz = t < 2, j = buzz ? (Math.floor(now * 14) % 2 ? 1 : -1) : 0;
    fill(x + 37 + j, y - 4, 7, 4, 0x1e1e24); fill(x + 37 + j, y - 4, 7, 1, 0x3a3a44); fill(x + 43 + j, y - 4, 1, 4, 0x101014);
    fill(x + 38 + j, y - 3, 4, 2, buzz ? 0xb8ff8a : 0x5a7a4a);
    if (buzz) { for (let k = 0; k < 3; k++) dot(x + 38 + j + k, y - 2, 0x1e3a1e); dot(x + 36, y - 3, 0x5a5a64, 0.6); dot(x + 45, y - 3, 0x5a5a64, 0.6); }
    dot(x + 43 + j, y - 5, 0x3a3a44);
    if (isNight && buzz) glowLate(() => glowAt(x + 40 + j, y - 2, 4, 2, 0xb8ff8a, 0.2));
  },
  /** A burlap sack of coffee rolled open at the top, full of dark beans, with a yellow, blue and red stripe printed on it; every 20 s a bean rolls out across the desk. */
  narcoscoffee(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 7, 8, 7, 0xc8a878); fill(x + 34, y - 6, 1, 6, 0xc8a878); fill(x + 35, y - 7, 1, 7, 0xd8bc8c); fill(x + 42, y - 7, 1, 7, 0xa08858);
    fill(x + 35, y - 8, 8, 1, 0xd8bc8c); fill(x + 36, y - 8, 6, 1, 0x4a2a1a); dot(x + 37, y - 9, 0x5a3420); dot(x + 40, y - 9, 0x4a2a1a); dot(x + 39, y - 9, 0x6a4028);
    fill(x + 36, y - 5, 6, 1, 0xf2c830); fill(x + 36, y - 4, 6, 1, 0x2a4a9a); fill(x + 36, y - 3, 6, 1, 0xc8202a);
    for (let k = 0; k < 6; k += 2) dot(x + 35 + k, y - 1, 0xa08858);
    const t = (now + x * 0.33) % 20;
    if (t < 3) dot(x + 43 + Math.min(2, Math.floor(t)), y - 1, 0x4a2a1a); else dot(x + 44, y - 1, 0x4a2a1a, 0.0);
  },
  /** A phone on a magnetic stand, its lock screen a bright wallpaper; every 15 s a white notification slides down over it. */
  apple1iphone(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 1, 5, 1, A1_SILVER_DARK); fill(x + 39, y - 3, 1, 2, A1_SILVER);
    const [c1, c2, c3] = A1_WALLS[Math.floor((now + x * 0.3) / 45) % 3];
    fill(x + 37, y - 12, 5, 9, 0x2c2c2e); fill(x + 38, y - 11, 3, 7, c1); fill(x + 38, y - 8, 3, 2, c2); fill(x + 38, y - 6, 3, 2, c3); dot(x + 39, y - 11, 0x1c1c1e);
    const t = (now + x * 0.29) % 15;
    if (t < 2.5) fill(x + 38, y - 11 + Math.min(1, Math.floor(t * 4)), 3, 1, WHITE, 0.9);
    if (isNight) glowLate(() => glowAt(x + 39, y - 7, 4, 5, c2, 0.12));
  },
  /** An earbuds case: a small white rounded case whose lid flips open every 14 s for three seconds, two white buds inside and a green light on its front. */
  apple1airpods(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    const t = (now + x * 0.27) % 14, open = t < 3;
    fill(x + 37, y - 5, 6, 5, WHITE); fill(x + 36, y - 4, 8, 3, WHITE); fill(x + 37, y - 1, 6, 1, 0xd8d8dc); fill(x + 43, y - 4, 1, 3, 0xd8d8dc);
    if (open) { fill(x + 37, y - 9, 6, 2, WHITE); fill(x + 36, y - 8, 1, 2, 0xe8e8ec); dot(x + 38, y - 6, 0xe8e8ec); dot(x + 41, y - 6, 0xe8e8ec); fill(x + 38, y - 6, 1, 1, WHITE); fill(x + 41, y - 6, 1, 1, WHITE); }
    else fill(x + 37, y - 6, 6, 1, 0xe8e8ec);
    dot(x + 40, y - 3, open ? 0x34c759 : 0xd8d8dc);
  },
  /** A watch on its charging puck, the face black with three activity rings that fill round now and then. */
  apple1watch(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, WHITE); fill(x + 35, y - 2, 9, 1, 0xf4f4f6); fill(x + 39, y - 1, 1, 1, 0xd8d8dc);
    fill(x + 37, y - 9, 6, 7, A1_SILVER); fill(x + 38, y - 8, 4, 5, 0x0c0c0e); fill(x + 43, y - 7, 1, 2, A1_SILVER_DARK);
    fill(x + 38, y - 11, 4, 2, 0xf07a6a); fill(x + 38, y - 2, 4, 1, 0xf07a6a, 0.0);
    const fillTo = Math.floor(((now + x * 0.2) % 10) / 10 * 4);
    [[0xff2d55, 0], [0x34c759, 1], [0x5ac8fa, 2]].forEach(([c, i]) => { for (let k = 0; k <= Math.min(fillTo, 3); k++) dot(x + 38 + k, y - 8 + i * 2, c); });
    if (isNight) glowLate(() => glowAt(x + 40, y - 6, 3, 3, 0x34c759, 0.1));
  },
  /** A small round mesh speaker in white, its top lit: every 18 s it listens, a swirl of colour turning on its top, glowing at night. */
  apple1homepod(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 7, 6, 7, 0xeeeef0); fill(x + 36, y - 6, 8, 5, 0xeeeef0); fill(x + 37, y - 8, 6, 1, 0xdcdce0);
    for (let r = 0; r < 5; r++) for (let c = 0; c < 4; c++) dot(x + 37 + c * 2 + (r % 2), y - 6 + r, 0xd0d0d4);
    fill(x + 36, y - 6, 1, 5, WHITE); fill(x + 43, y - 6, 1, 5, 0xc8c8cc);
    const t = (now + x * 0.23) % 18, listening = t < 3;
    if (listening) { const k = Math.floor(now * 6) % 3; fill(x + 38, y - 8, 4, 1, [0xff2d55, A1_BLUE, 0x34c759][k]); dot(x + 38 + k, y - 8, 0xaf52de); glowAt(x + 40, y - 8, 4, 2, A1_BLUE, 0.15); }
    else fill(x + 38, y - 8, 4, 1, 0xe4e4e8);
    if (isNight && listening) glowLate(() => glowAt(x + 40, y - 8, 5, 3, 0xaf52de, 0.2));
  },
  /** A white flat mouse with a silver base and, beside it, a little round tracker tag that sends out a ripple every 20 s. */
  apple1mouse(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 6, 2, WHITE); fill(x + 36, y - 3, 4, 1, WHITE); fill(x + 35, y - 1, 6, 1, A1_SILVER); dot(x + 37, y - 3, 0xf8f8fa);
    fill(x + 42, y - 2, 3, 2, WHITE); dot(x + 43, y - 3, WHITE); dot(x + 43, y - 1, A1_SILVER); dot(x + 43, y - 2, A1_SILVER_DARK);
    const t = (now + x * 0.31) % 20;
    if (t < 1.5) ring(x + 43, y - 2, 1 + t * 2, A1_BLUE, 0.6 * (1 - t / 1.5));
  },
  /** A tablet on a stand with a white pencil drawing on it: every 16 s a coloured line is sketched across its screen, then it clears. */
  apple1ipad(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 1, 6, 1, A1_SILVER_DARK);
    fill(x + 35, y - 11, 10, 9, 0x2c2c2e); fill(x + 36, y - 10, 8, 7, WHITE);
    const t = (now + x * 0.29) % 16, n = Math.min(8, Math.floor(t * 2));
    for (let k = 0; k < n; k++) dot(x + 36 + k, y - 6 - Math.round(Math.sin(k * 0.9) * 2), [0xff2d55, 0xff9500, A1_BLUE][Math.floor(k / 3)]);
    line(x + 36 + n, y - 7, x + 39 + n, y - 12, WHITE); dot(x + 36 + n, y - 7, 0x3a3a3c);
  },
  /** A headset on its stand: a curved dark glass front reflecting a sliver of colour, an aluminium frame and a grey knitted band; every 22 s a shimmer passes across the glass. */
  apple1vision(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 38, y - 5, 3, 5, A1_SILVER); fill(x + 37, y - 1, 5, 1, A1_SILVER_DARK);
    fill(x + 35, y - 10, 10, 5, A1_SILVER); fill(x + 36, y - 9, 8, 3, 0x1c1c22); fill(x + 36, y - 9, 8, 1, 0x3a3a48);
    fill(x + 35, y - 5, 10, 1, 0x8a8a90); fill(x + 34, y - 9, 1, 3, 0x8a8a90); fill(x + 45, y - 9, 1, 3, 0x8a8a90, 0.0);
    dot(x + 37, y - 8, 0x6a5ae8, 0.7); dot(x + 42, y - 8, 0x5ac8fa, 0.6);
    const t = (now + x * 0.21) % 22;
    if (t < 1) { const gx = x + 36 + Math.floor(t * 8); dot(gx, y - 9, WHITE); dot(gx - 1, y - 8, WHITE, 0.6); }
  },
  /** A shiny red apple with a bite out of it and a little leaf on its stalk, a highlight on its cheek; every 13 s it rocks on its base. */
  apple1apple(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    const rock = (now + x * 0.27) % 13 < 0.8 ? (Math.floor(now * 6) % 2 ? 1 : -1) : 0;
    fill(x + 37 + rock, y - 6, 6, 5, 0xd8202a); fill(x + 36 + rock, y - 5, 8, 3, 0xd8202a); fill(x + 38 + rock, y - 1, 4, 1, 0xa81820);
    fill(x + 37 + rock, y - 7, 2, 1, 0xd8202a); fill(x + 41 + rock, y - 7, 2, 1, 0xd8202a);
    fill(x + 43 + rock, y - 5, 1, 2, 0xf4e8c8); dot(x + 42 + rock, y - 4, 0xf4e8c8);
    dot(x + 37 + rock, y - 5, 0xff8a8a); dot(x + 38 + rock, y - 6, WHITE, 0.6);
    fill(x + 40 + rock, y - 9, 1, 2, 0x6a4a2a); fill(x + 41 + rock, y - 10, 2, 1, 0x4aa84a); dot(x + 41 + rock, y - 9, 0x3a8a3a);
  },
  /** A stack of three-and-a-half-inch floppies in blue, black and beige, a rainbow apple on the top label; every 14 s the top disk slides out a pixel and back. */
  apple2floppies(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    [[0x2a2a30, 0], [0x2a5ab8, 2], [A2_BEIGE, 4], [0x2a5ab8, 6]].forEach(([c, h]) => { fill(x + 36, y - 2 - h, 8, 2, c); fill(x + 36, y - 2 - h, 8, 1, shaded(c, 1.2)); fill(x + 40, y - 2 - h, 3, 1, 0xc8ccd4); });
    const out = (now + x * 0.29) % 14 < 1.5 ? 1 : 0;
    fill(x + 36 + out, y - 10, 8, 2, 0xe03a3e); fill(x + 36 + out, y - 10, 8, 1, 0xf06a6a); fill(x + 37 + out, y - 10, 3, 1, A2_PAPER); dot(x + 38 + out, y - 10, 0x61bb46);
  },
  /** A beige one-button mouse on its curly cord; every 8 s its single big button clicks down. */
  apple2mouse(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 38, y - 4, 6, 4, A2_BEIGE); fill(x + 38, y - 4, 6, 1, A2_BEIGE_LIGHT); fill(x + 43, y - 4, 1, 4, A2_BEIGE_DARK); fill(x + 38, y - 1, 6, 1, A2_BEIGE_DARK);
    const click = (now + x * 0.3) % 8 < 0.3;
    fill(x + 39, y - 4, 4, 1, click ? A2_BEIGE_DARK : 0xd8d0b8);
    for (let k = 0; k < 4; k++) dot(x + 37 - k, y - 3 + (k % 2), 0x8a8270);
  },
  /** A pocket notepad computer: a dark grey slab with a greenish screen, a stylus writing a line of scribble across it every 12 s. */
  apple2newton(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 10, 2, 0x2a2a30); fill(x + 36, y - 11, 8, 9, 0x3a3a40); fill(x + 36, y - 11, 8, 1, 0x5a5a64);
    fill(x + 37, y - 10, 6, 6, 0xb8c8a8); fill(x + 37, y - 4, 6, 1, 0x2a2a30);
    const t = (now + x * 0.27) % 12, n = Math.min(5, Math.floor(t * 2));
    for (let k = 0; k < n; k++) dot(x + 37 + k, y - 8 + (k % 2), 0x2a3a2a);
    line(x + 37 + n, y - 8, x + 40 + n, y - 13, 0x6a6a74);
  },
  /** A little translucent blue all-in-one, the rounded back of a teal-blue desk computer from the late 90s with a white base; its insides glow softly. */
  apple2imac(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 2, 6, 2, A2_PAPER); fill(x + 37, y - 2, 6, 1, WHITE);
    fill(x + 36, y - 10, 8, 8, 0x2ab8c8, 0.75); fill(x + 35, y - 9, 10, 6, 0x2ab8c8, 0.75); fill(x + 36, y - 10, 8, 1, 0x8ae8f0, 0.8);
    fill(x + 37, y - 8, 6, 4, 0x1a7a8a, 0.6); appleMark('small', x + 38, y - 9, WHITE, 0.0); fill(x + 38, y - 7, 3, 2, 0x5ad8e8, 0.6);
    dot(x + 36, y - 9, WHITE, 0.6);
    const a = 0.6 + 0.4 * Math.sin(now * 0.8 + x);
    if (isNight) glowLate(() => glowAt(x + 40, y - 6, 5, 4, 0x5ad8e8, 0.15 * a));
  },
  /** A white mug with a rainbow apple on it and coffee inside, a curl of steam rising. */
  apple2mug(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 37, y - 7, 6, 7, A2_PAPER); fill(x + 37, y - 7, 1, 7, WHITE); fill(x + 42, y - 7, 1, 7, 0xd8d8d0); fill(x + 43, y - 5, 2, 1, A2_PAPER); fill(x + 44, y - 5, 1, 3, A2_PAPER); dot(x + 43, y - 3, A2_PAPER);
    fill(x + 38, y - 7, 4, 1, 0x5a3a24);
    appleMark('small', x + 38, y - 6, (r) => A2_RAINBOW[r]);
    for (let k = 0; k < 3; k++) { const ph = (now * 0.5 + k * 0.33) % 1; dot(x + 39 + Math.round(Math.sin(ph * 6 + k)), y - 9 - Math.floor(ph * 5), WHITE, 0.5 * (1 - ph)); }
  },
  /** A white music player of the early 2000s with a grey click wheel and a small screen, its white earphones coiled beside it; the screen's play bar creeps along. */
  apple2ipod(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 10, 6, 10, 0xf4f4f6); fill(x + 36, y - 10, 1, 10, WHITE); fill(x + 41, y - 10, 1, 10, 0xd8d8dc);
    fill(x + 37, y - 9, 4, 3, 0xb8c8d8); fill(x + 37, y - 7, 1 + Math.floor((now * 0.5) % 4), 1, 0x2a5ab8);
    disc(x + 39, y - 3, 1.6, 0xc8c8cc); dot(x + 39, y - 3, WHITE);
    line(x + 42, y - 1, x + 44, y - 4, WHITE); dot(x + 44, y - 5, WHITE); dot(x + 43, y - 1, WHITE); dot(x + 45, y - 2, WHITE);
  },
  /** A stack of sticky notes in the six rainbow colours, fanned a little; every 11 s the top one lifts its corner in a breeze. */
  apple2stickies(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    A2_RAINBOW.forEach((c, i) => fill(x + 36 + (i % 2), y - 2 - i, 8, 1, c));
    fill(x + 36, y - 9, 8, 2, A2_RAINBOW[5]); fill(x + 37, y - 9, 5, 1, shaded(A2_RAINBOW[5], 1.2));
    const lift = (now + x * 0.33) % 11 < 1.2;
    if (lift) { fill(x + 42, y - 11, 2, 2, A2_RAINBOW[5]); dot(x + 43, y - 12, shaded(A2_RAINBOW[5], 1.2)); }
    fill(x + 37, y - 8, 4, 1, 0x1a4a6a, 0.6);
  },
  /** A little chrome toaster figurine with white wings, a slice of toast peeking out of it; every 9 s its wings give a few flaps. */
  apple2toaster(x, y) {
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 36, y - 6, 8, 6, 0xc8ccd4); fill(x + 36, y - 6, 8, 1, 0xe8ecf2); fill(x + 36, y - 6, 1, 6, 0xe0e4ea); fill(x + 43, y - 6, 1, 6, 0x8a8e98); fill(x + 36, y - 1, 8, 1, 0x8a8e98);
    fill(x + 38, y - 8, 3, 2, 0xd8a050); fill(x + 38, y - 8, 3, 1, 0xa86a2a); dot(x + 44, y - 4, 0x4a4a52);
    const flap = (now + x * 0.3) % 9 < 1.5 ? Math.floor(now * 6) % 2 : 0;
    if (flap) { fill(x + 33, y - 10, 3, 3, WHITE); fill(x + 44, y - 10, 2, 3, WHITE); } else { fill(x + 33, y - 6, 3, 2, WHITE); dot(x + 34, y - 7, WHITE); fill(x + 44, y - 7, 1, 2, WHITE, 0.0); }
  },
  /** A porg plush: a round little sea bird in brown with a white face and belly, big dark eyes and orange feet; every 14 s it flaps its stubby wings and squeaks, its beak opening. */
  starwarsporg(x, y) {
    const B = 0x9a6a44, BL = 0xb8865a, BD = 0x6a4a2e, W = 0xf4eee4, O = 0xe8782e;
    fill(x + 34, y, 11, 1, SHADOW);
    const t = (now + x * 0.29) % 14, flap = t < 1.4 ? Math.floor(now * 8) % 2 : -1;
    // The wings, flapping or folded
    if (flap === 1) { fill(x + 34, y - 8, 2, 2, B); fill(x + 44, y - 8, 2, 2, BD); }
    else if (flap === 0) { fill(x + 34, y - 6, 2, 2, B); fill(x + 44, y - 6, 2, 2, BD); }
    // The body, the white belly and face
    fill(x + 37, y - 10, 6, 9, B); fill(x + 36, y - 9, 8, 7, B); fill(x + 37, y - 10, 6, 1, BL); fill(x + 36, y - 9, 1, 7, BL); fill(x + 43, y - 9, 1, 7, BD);
    fill(x + 38, y - 5, 4, 4, W); fill(x + 37, y - 4, 6, 2, W); fill(x + 37, y - 9, 6, 3, W); dot(x + 37, y - 9, B); dot(x + 42, y - 9, B);
    // The big eyes, the beak, the feet
    fill(x + 37, y - 8, 2, 2, 0x1a1210); fill(x + 41, y - 8, 2, 2, 0x1a1210); dot(x + 37, y - 8, WHITE, 0.8); dot(x + 41, y - 8, WHITE, 0.8);
    dot(x + 39, y - 6, O); dot(x + 40, y - 6, O); if (flap >= 0) dot(x + 40, y - 5, 0x8a3a1a);
    dot(x + 38, y - 1, O); dot(x + 41, y - 1, O);
  },
  /** A braided challah, glossy and golden with sesame seeds, on a wooden board; now and then a wisp of warmth rises from it as if just out of the oven. */
  jewishchallah(x, y) {
    const crust = 0xc8843a, crustLight = 0xe8a858, crustDark = 0x8a5422, board = 0x8a5a32, boardLight = 0xa8703e, seed = 0xf6ecc8;
    fill(x + 33, y - 2, 13, 2, board); fill(x + 33, y - 2, 13, 1, boardLight); dot(x + 45, y - 1, 0x5a3b25);
    // The loaf: three braided humps
    fill(x + 34, y - 4, 11, 2, crust); fill(x + 35, y - 6, 9, 2, crust); fill(x + 37, y - 7, 5, 1, crust);
    for (const bx of [36, 39, 42]) { fill(x + bx, y - 6, 2, 1, crustLight); line(x + bx + 2, y - 6, x + bx + 1, y - 3, crustDark); }
    dot(x + 34, y - 4, crustLight); fill(x + 34, y - 3, 11, 1, crustDark, 0.6);
    for (const [sx, sy] of [[36, -5], [38, -7], [40, -5], [41, -6], [43, -4]]) dot(x + sx, y + sy, seed);
    // A wisp of warmth now and then
    const t = (now + x * 0.27) % 19;
    if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 39 + (r % 2), y - 9 - r, WHITE, 0.5 - r * 0.1); dot(x + 40 - (r % 2), y - 10 - r, WHITE, 0.35 - r * 0.07); }
  },
  /** A silver kiddush cup on its saucer, red wine in it and an engraved band round the bowl; a gleam slides down the bowl now and then. */
  jewishkiddush(x, y) {
    const silver = 0xc8ccd4, light = 0xeef0f4, dark = 0x8a8e98, wine = 0x7a1a30;
    // The saucer, the foot and the stem
    fill(x + 35, y - 1, 9, 1, silver); dot(x + 35, y - 1, light); dot(x + 43, y - 1, dark);
    fill(x + 38, y - 2, 3, 1, silver); fill(x + 39, y - 5, 1, 3, silver); dot(x + 39, y - 4, light);
    // The bowl, its band and the wine
    fill(x + 37, y - 9, 5, 4, silver); fill(x + 38, y - 5, 3, 1, silver); fill(x + 37, y - 9, 1, 4, light); fill(x + 41, y - 9, 1, 4, dark);
    fill(x + 37, y - 7, 5, 1, dark, 0.5);
    fill(x + 37, y - 10, 5, 1, light); fill(x + 38, y - 10, 3, 1, wine);
    const t = (now + x * 0.21) % 14;
    if (t < 1.2) dot(x + 38, y - 9 + Math.floor(t * 3), WHITE, 0.9);
  },
  /** A blue wooden dreidel with a gold letter-like mark on each face: every so often it spins, its faces flicking by, wobbles as it slows, and tips onto its side until the next spin. */
  jewishdreidel(x, y) {
    const wood = 0x3a6ab0, light = 0x6a9ae0, dark = 0x24467a, mark = 0xf2d35b;
    const marks = [[[0, 0], [1, 0], [1, 1]], [[0, 0], [0, 1], [1, 1]], [[0, 0], [1, 0], [0, 1]], [[1, 0], [1, 1], [0, 1]]];
    const t = (now + x * 0.33) % 17;
    if (t < 3.2) {
      // Spinning upright, then wobbling as it slows
      const k = Math.floor(now * (t < 2.4 ? 12 : 5)) % 4, w = t > 2.4 ? (Math.floor(now * 6) % 2 ? 1 : -1) : 0;
      fill(x + 37, y - 1, 5, 1, SHADOW);
      fill(x + 38 + w, y - 9, 1, 2, dark);
      fill(x + 36 + w, y - 7, 5, 4, wood); fill(x + 36 + w, y - 7, 1, 4, light); fill(x + 40 + w, y - 7, 1, 4, dark);
      fill(x + 37 + w, y - 3, 3, 1, wood); dot(x + 38 + w, y - 2, dark);
      for (const [mx, my] of marks[k]) dot(x + 37 + w + mx, y - 6 + my, mark);
      if (t < 2.4) { dot(x + 35, y - 5, light, 0.4); dot(x + 41, y - 6, light, 0.4); }
      return;
    }
    // Lying on its side
    fill(x + 36, y - 1, 7, 1, SHADOW);
    fill(x + 37, y - 4, 4, 3, wood); fill(x + 37, y - 4, 4, 1, light); fill(x + 37, y - 2, 4, 1, dark);
    fill(x + 35, y - 3, 2, 1, dark); dot(x + 41, y - 3, dark);
    for (const [mx, my] of marks[Math.floor((now + x * 0.33) / 17) % 4]) dot(x + 38 + mx, y - 4 + my, mark);
  },
  /** A blue-and-white ceramic bowl of pomegranates, one broken open to show its ruby seeds; a seed catches the light now and then. */
  jewishpomegranate(x, y) {
    const red = 0xc0283a, redLight = 0xe04a58, redDark = 0x801828, crown = 0x9a5a2a, seed = 0xe8405a, pith = 0xf2dcc0, blue = 0x2f5f9e, white = 0xf2f0ea;
    // Two whole fruits, each with its crown
    for (const [cx, cy] of [[37, -6], [41, -7]]) {
      disc(x + cx, y + cy, 2.6, red); dot(x + cx - 1, y + cy - 1, redLight); dot(x + cx + 1, y + cy + 1, redDark);
      dot(x + cx, y + cy - 3, crown); dot(x + cx - 1, y + cy - 3, crown, 0.7); dot(x + cx + 1, y + cy - 3, crown, 0.7);
    }
    // The open half in front, its seeds showing
    fill(x + 38, y - 5, 5, 2, pith); fill(x + 38, y - 5, 5, 1, red);
    for (const [sx, sy] of [[39, -4], [41, -4], [40, -5], [42, -5]]) dot(x + sx, y + sy, seed);
    const g = (now + x * 0.19) % 10;
    if (g < 0.8) dot(x + 40, y - 4, WHITE, 0.9);
    // The bowl
    fill(x + 34, y - 4, 11, 1, white); fill(x + 35, y - 3, 9, 2, white); fill(x + 36, y - 1, 7, 1, 0xd8d8de);
    fill(x + 35, y - 3, 9, 1, blue); for (const dx of [36, 39, 42]) dot(x + dx, y - 2, blue);
    dot(x + 34, y - 4, 0xffffff); dot(x + 44, y - 4, 0xc8c8d0);
  },
  /** Apple and honey for a sweet new year: a glass jar of golden honey with a wooden dipper beside a red apple; now and then a little bee comes visiting, circles the jar and goes. */
  jewishhoney(x, y) {
    const honey = 0xe8a830, honeyLight = 0xf8d070, honeyDark = 0xc08020, glass = 0xdff0f6, apple = 0xc8302a, appleLight = 0xe85a48, wood = 0xa8703e;
    // The jar of honey, its rim, and the dipper
    fill(x + 34, y - 7, 6, 7, honey); fill(x + 34, y - 7, 1, 7, honeyLight); fill(x + 39, y - 7, 1, 7, honeyDark);
    fill(x + 34, y - 8, 6, 1, glass, 0.7); dot(x + 35, y - 6, WHITE, 0.7);
    line(x + 38, y - 7, x + 40, y - 11, wood); fill(x + 40, y - 12, 2, 2, wood); dot(x + 40, y - 12, 0xc8904e);
    // The apple, its stalk and leaf
    disc(x + 43, y - 3, 2.2, apple); dot(x + 42, y - 4, appleLight); dot(x + 43, y - 6, 0x5a3b25); dot(x + 44, y - 6, 0x5a9a4a);
    fill(x + 34, y, 11, 1, SHADOW);
    // The bee
    const t = (now + x * 0.17) % 21;
    if (t < 4) {
      const a = t * 3, bx = x + 38 + Math.round(Math.cos(a) * 5), by = y - 10 + Math.round(Math.sin(a) * 2);
      dot(bx, by, 0xf2c030); dot(bx + 1, by, 0x2a2420); dot(bx, by - 1, WHITE, Math.floor(now * 12) % 2 ? 0.8 : 0.3);
    }
  },
  /** A little blue tin tzedakah box for charity, a slot in its lid and a painted white band with a heart; now and then a coin drops in with a glint. */
  jewishtzedakah(x, y) {
    const tin = 0x2f5f9e, light = 0x5a8ad0, dark = 0x1e3e6e, white = 0xf2f0ea, gold = 0xe8c060;
    fill(x + 36, y - 8, 7, 8, tin); fill(x + 36, y - 8, 1, 8, light); fill(x + 42, y - 8, 1, 8, dark); fill(x + 36, y - 9, 7, 1, light);
    fill(x + 38, y - 9, 3, 1, 0x14243e);
    fill(x + 36, y - 5, 7, 3, white); dot(x + 38, y - 4, 0xc0283a); dot(x + 40, y - 4, 0xc0283a); dot(x + 39, y - 3, 0xc0283a); dot(x + 39, y - 4, 0xc0283a);
    fill(x + 43, y - 1, 2, 1, SHADOW);
    const t = (now + x * 0.23) % 18;
    if (t < 1) { const fy = y - 14 + Math.floor(t * 5); dot(x + 39, fy, gold); dot(x + 39, fy - 1, 0xfff0a0, 0.6); }
    else if (t < 1.5) dot(x + 39, y - 10, WHITE, 0.8);
  },
  /** A silver spice tower for Havdalah: a pierced little turret on a stem, a pointed roof and a blue pennant that flutters now and then; a curl of fragrance rises from it now and then. */
  jewishspicebox(x, y) {
    const silver = 0xc8ccd4, light = 0xeef0f4, dark = 0x8a8e98, hole = 0x4a4e58, flag = 0x2f5f9e;
    // The foot, the stem and the pierced turret
    fill(x + 37, y - 1, 5, 1, silver); dot(x + 37, y - 1, light); fill(x + 38, y - 3, 3, 2, silver); dot(x + 40, y - 2, dark);
    fill(x + 37, y - 8, 5, 5, silver); fill(x + 37, y - 8, 1, 5, light); fill(x + 41, y - 8, 1, 5, dark);
    for (const [hx, hy] of [[38, -7], [40, -7], [39, -5], [38, -4], [40, -4]]) dot(x + hx, y + hy, hole);
    // The roof, the pole and its pennant
    fill(x + 37, y - 9, 5, 1, dark); fill(x + 38, y - 10, 3, 1, silver); dot(x + 39, y - 11, silver); dot(x + 39, y - 12, light);
    const flap = (now + x * 0.11) % 9 < 1.5 ? Math.floor(now * 6) % 2 : 0;
    dot(x + 40, y - 12, flag); dot(x + 41, y - 12 + flap, flag);
    fill(x + 42, y - 1, 2, 1, SHADOW);
    const s = (now + x * 0.19) % 16;
    if (s < 2.5) { const r = Math.floor(s * 2); dot(x + 42 + (r % 2), y - 6 - r, 0xc8a070, 0.6 - r * 0.1); }
  },
  /** Two sufganiyot on a little white plate: round golden doughnuts dusted with sugar, a dot of red jam on top of each; a grain of sugar sparkles now and then. */
  jewishsufganiyot(x, y) {
    const dough = 0xd89040, doughDark = 0xa86424, sugar = 0xfbf8f0, jam = 0xc0283a, plate = 0xf2f0ea;
    fill(x + 34, y - 2, 11, 1, plate); fill(x + 35, y - 1, 9, 1, 0xd8d8de);
    for (const [cx, cy] of [[37, -5], [42, -4]]) {
      fill(x + cx - 2, y + cy - 1, 5, 3, dough); fill(x + cx - 1, y + cy - 2, 3, 1, dough); fill(x + cx - 1, y + cy + 2, 3, 1, doughDark);
      fill(x + cx - 2, y + cy - 1, 5, 1, sugar, 0.8); dot(x + cx - 1, y + cy - 2, sugar); dot(x + cx + 1, y + cy - 2, sugar, 0.7);
      dot(x + cx, y + cy - 2, jam); dot(x + cx, y + cy - 1, shaded(jam, 0.8));
      fill(x + cx + 2, y + cy, 1, 2, doughDark);
    }
    const t = (now + x * 0.29) % 9;
    if (t < 0.7) dot(x + 36 + (Math.floor((now + x) / 9) % 3) * 3, y - 6, WHITE, 0.95);
  },
  /** Mint tea: a silver teapot with a curved spout and a small glass of amber tea with a sprig of mint on a brass tray; a curl of steam rises from the glass now and then. */
  muslimtea(x, y) {
    const brass = 0xc8a050, brassLight = 0xe8c878, silver = 0xc8ccd4, silverLight = 0xeef0f4, silverDark = 0x8a8e98, tea = 0xc87828, mint = 0x4a9a4a;
    fill(x + 33, y - 1, 13, 1, brass); fill(x + 34, y - 2, 11, 1, brassLight);
    // The teapot: body, lid and knob, spout, handle
    fill(x + 34, y - 7, 5, 5, silver); fill(x + 34, y - 7, 1, 5, silverLight); fill(x + 38, y - 7, 1, 5, silverDark);
    fill(x + 35, y - 8, 3, 1, silver); dot(x + 36, y - 9, silverLight); dot(x + 36, y - 10, silver);
    line(x + 39, y - 5, x + 41, y - 8, silver); dot(x + 33, y - 6, silverDark); dot(x + 33, y - 5, silverDark);
    // The glass of tea and its mint
    fill(x + 42, y - 5, 3, 3, tea, 0.85); fill(x + 42, y - 6, 3, 1, 0xdff0f6, 0.6); dot(x + 42, y - 5, WHITE, 0.5); dot(x + 43, y - 7, mint); dot(x + 44, y - 7, mint);
    const t = (now + x * 0.23) % 12;
    if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 43 + (r % 2), y - 8 - r, WHITE, 0.45 - r * 0.1); }
  },
  /** A small teal glazed bowl heaped with glossy dates, two loose beside it; one catches the light now and then. */
  muslimdates(x, y) {
    const date = 0x6a3418, dateLight = 0x9a5228, bowl = 0x2f8a8a, bowlLight = 0x5ab0aa, bowlDark = 0x1f5e5e;
    const heap = [[36, -6], [38, -7], [40, -6], [42, -6], [37, -5], [39, -5], [41, -5], [39, -8]];
    for (const [dx, dy] of heap) { fill(x + dx, y + dy, 2, 1, date); dot(x + dx, y + dy, dateLight); }
    fill(x + 35, y - 4, 10, 1, bowlLight); fill(x + 35, y - 3, 10, 2, bowl); fill(x + 36, y - 1, 8, 1, bowlDark);
    fill(x + 45, y - 1, 2, 1, date); dot(x + 45, y - 1, dateLight); fill(x + 33, y - 1, 2, 1, date);
    const g = (now + x * 0.31) % 11;
    if (g < 0.8) { const [dx, dy] = heap[Math.floor((now + x) / 11) % heap.length]; dot(x + dx, y + dy, WHITE, 0.8); }
  },
  /** A brass qibla compass in a round case, its lid standing open behind it; every so often the needle swings and settles again. */
  muslimqibla(x, y) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, face = 0xf4ecd8, cx = x + 39, cy = y - 4;
    // The open lid behind, the case and its face
    fill(cx - 3, y - 12, 7, 5, brass); fill(cx - 2, y - 11, 5, 3, brassDark); dot(cx - 3, y - 12, brassLight);
    disc(cx, cy, 3.6, brass); disc(cx, cy, 2.6, face); dot(cx - 2, cy - 2, brassLight);
    dot(cx, cy - 3, brassDark); dot(cx + 3, cy, brassDark, 0.6);
    // The needle: still pointing the same way, but swinging for a moment now and then
    const t = (now + x * 0.2) % 20, a = -0.6 + (t < 3 ? Math.sin(t * 6) * ((3 - t) / 3) * 1.2 : 0);
    line(cx, cy, cx + Math.round(Math.cos(a) * 2), cy + Math.round(Math.sin(a) * 2), 0xc0283a);
    dot(cx - Math.round(Math.cos(a) * 2), cy - Math.round(Math.sin(a) * 2), 0x3a3238);
    dot(cx, cy, brassDark);
    fill(cx - 3, y, 7, 1, SHADOW);
  },
  /** A string of amber prayer beads lying in a loose loop with a green tassel; now and then a bead brightens and the light moves on bead by bead, as if being counted. */
  muslimbeads(x, y) {
    const amber = 0xd8902a, amberLight = 0xf2c060, amberDark = 0x9a5a18, tassel = 0x2f6a4a;
    const t = (now + x * 0.3) % 14, at = t < 4 ? Math.floor(t * 4) % 16 : -1;
    for (let i = 0; i < 16; i++) {
      const a = (i / 16) * Math.PI * 2, bx = x + 39 + Math.round(Math.cos(a) * 5), by = y - 4 + Math.round(Math.sin(a) * 2.5);
      dot(bx, by, i === at ? 0xfff0b0 : i % 2 ? amber : amberDark);
      if (i === at) dot(bx, by - 1, amberLight, 0.5);
    }
    fill(x + 44, y - 3, 1, 1, amberLight); fill(x + 44, y - 2, 1, 2, tassel); dot(x + 43, y - 1, tassel); dot(x + 45, y - 1, tassel);
  },
  /** A brass dallah, the Arabic coffee pot, with its long beaked spout, pointed lid and handle, and a small cup beside it; a wisp of steam rises from the spout now and then. */
  muslimdallah(x, y) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, cup = 0xf2ead8;
    fill(x + 35, y - 1, 6, 1, brassDark); fill(x + 35, y - 5, 6, 4, brass); fill(x + 35, y - 5, 1, 4, brassLight); fill(x + 40, y - 5, 1, 4, brassDark);
    fill(x + 36, y - 7, 4, 2, brass); fill(x + 36, y - 8, 4, 1, brassLight); fill(x + 37, y - 10, 2, 2, brass); dot(x + 37, y - 11, brassLight);
    line(x + 41, y - 5, x + 43, y - 9, brass); dot(x + 44, y - 9, brass);
    fill(x + 33, y - 7, 1, 4, brassDark); dot(x + 34, y - 7, brassDark);
    fill(x + 42, y - 2, 3, 2, cup); dot(x + 42, y - 2, 0xd8b45a); dot(x + 44, y - 1, 0xc8c0b0);
    const t = (now + x * 0.17) % 13;
    if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 44 + (r % 2), y - 11 - r, WHITE, 0.45 - r * 0.1); }
  },
  /** A mosaic glass lamp: a round globe of little coloured glass pieces on a brass stand, lit from inside; it glows softly by day and shines through the night. */
  muslimlamp(x, y) {
    const brass = 0xc8a050, brassDark = 0x8a6a2a, cx = x + 39, cy = y - 8;
    const pieces = [0xe8503a, 0xf2a830, 0x4c8bf5, 0x4ab06a, 0xb05ad0, 0xf2d35b];
    fill(cx - 2, y - 1, 5, 1, brassDark); fill(cx, y - 4, 1, 3, brass); dot(cx - 1, y - 2, brass);
    disc(cx, cy, 3.2, 0x5a2a1a); dot(cx, cy - 4, brass); dot(cx, cy + 4, brass);
    const flick = 0.8 + 0.2 * candlelight(x * 0.1), glass = (a) => {
      for (let dy = -2; dy <= 2; dy++) for (let dx = -2; dx <= 2; dx++) {
        if (dx * dx + dy * dy > 6) continue;
        dot(cx + dx, cy + dy, pieces[(Math.abs(dx * 3 + dy * 5) + 7) % 6], a);
      }
      dot(cx, cy, 0xfff4c0, a);
    };
    glass(0.85 * flick);
    glowAt(cx, cy, 6, 6, 0xffb060, isNight ? 0.14 : 0.05);
    if (isNight) glowLate(() => { glowAt(cx, cy, 6, 6, 0xffb060, 0.16); glass(flick); });
  },
  /** A mabkhara incense burner: a tall square wooden stand inlaid with brass, a smouldering coal in its bowl, and a thin curl of fragrant smoke winding up and fading. */
  muslimincense(x, y) {
    const wood = 0x5a3424, woodLight = 0x7a4a34, woodDark = 0x3a2016, brass = 0xc8a050;
    fill(x + 36, y - 1, 7, 1, woodDark); fill(x + 37, y - 6, 5, 5, wood); fill(x + 37, y - 6, 1, 5, woodLight); fill(x + 41, y - 6, 1, 5, woodDark);
    dot(x + 39, y - 4, brass); dot(x + 38, y - 3, brass, 0.7); dot(x + 40, y - 3, brass, 0.7); fill(x + 36, y - 7, 7, 1, brass); fill(x + 37, y - 8, 5, 1, woodDark);
    dot(x + 39, y - 8, 0xe8502a, 0.7 + 0.3 * candlelight(x * 0.3));
    for (let k = 0; k < 4; k++) {
      const ph = (now * 0.5 + k / 4) % 1, sx = x + 39 + Math.round(Math.sin(ph * 6 + k) * 1.2), sy = y - 9 - Math.round(ph * 5);
      dot(sx, sy, 0xd8d4cc, 0.5 * (1 - ph));
    }
    if (isNight) glowLate(() => dot(x + 39, y - 8, 0xff8a50, 0.8));
  },
  /** A silver rosewater sprinkler with a long slender neck beside a single pink rose; now and then a fine mist sparkles from its tip. */
  muslimrosewater(x, y) {
    const silver = 0xc8ccd4, silverLight = 0xeef0f4, silverDark = 0x8a8e98, pink = 0xe86a90, pinkLight = 0xf8a8c0, pinkDark = 0xb04068, green = 0x4a7a3a;
    disc(x + 38, y - 3, 2.4, silver); dot(x + 37, y - 4, silverLight); dot(x + 40, y - 2, silverDark);
    fill(x + 37, y - 1, 3, 1, silverDark); fill(x + 38, y - 10, 1, 6, silver); dot(x + 38, y - 11, silverLight); dot(x + 37, y - 7, silverLight, 0.6);
    line(x + 43, y - 1, x + 43, y - 6, green); dot(x + 42, y - 3, green);
    fill(x + 42, y - 8, 3, 2, pink); dot(x + 43, y - 8, pinkDark); dot(x + 42, y - 9, pinkLight); dot(x + 44, y - 9, pink);
    const t = (now + x * 0.13) % 13;
    if (t < 1.2) for (const [dx, dy] of [[-1, -12], [1, -12], [0, -13], [2, -13]]) dot(x + 38 + dx, y + dy, WHITE, 0.7 - t * 0.4);
  },
  /** The Bible standing on a small wooden stand: black leather with a small gold cross and gold page edges, a red ribbon hanging below; a gleam crosses the cross now and then. */
  christianbible(x, y) {
    const wood = 0x8a5a32, woodDark = 0x5a3b25, cover = 0x2a2426, coverLight = 0x4a4044, gold = 0xd8b45a, red = 0xa8283a;
    fill(x + 35, y - 1, 10, 1, woodDark); fill(x + 36, y - 2, 8, 1, wood);
    fill(x + 35, y - 11, 8, 9, cover); fill(x + 35, y - 11, 8, 1, coverLight); fill(x + 35, y - 11, 1, 9, coverLight); fill(x + 43, y - 10, 1, 8, gold);
    fill(x + 39, y - 9, 1, 5, gold); fill(x + 38, y - 8, 3, 1, gold);
    dot(x + 41, y - 1, red); dot(x + 41, y - 2, red);
    const g = (now + x * 0.19) % 15;
    if (g < 1) dot(x + 39, y - 9 + Math.floor(g * 5), 0xfff0b0);
  },
  /** A little wooden nativity: a stable with a thatched roof and a star on top, two small robed figures kneeling by the manger, and the manger itself glowing softly in the straw (no faces shown); the star twinkles now and then and glows at night, brighter from December to Epiphany. */
  christiannativity(x, y) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, back = 0x3a2818, hay = 0xe8c860, cloth = 0xf6f2ea;
    const blue = 0x3a5aa0, brown = 0x7a5a3a, gold = 0xf2d35b, xmas = christmasTime();
    // The stable: its back, posts and roof
    fill(x + 34, y - 8, 11, 8, back); fill(x + 34, y - 8, 1, 8, woodDark); fill(x + 44, y - 8, 1, 8, woodDark);
    fill(x + 38, y - 11, 3, 1, woodLight); fill(x + 36, y - 10, 7, 1, wood); fill(x + 33, y - 9, 13, 1, woodLight);
    fill(x + 34, y - 1, 11, 1, hay, 0.6);
    // The manger, the straw and the glow within it
    const glow = xmas ? 0.3 : 0.16;
    glowAt(x + 39, y - 4, 4, 3, 0xfff0b0, glow);
    fill(x + 38, y - 3, 3, 2, wood); fill(x + 38, y - 4, 3, 1, hay); dot(x + 39, y - 4, cloth);
    // The two kneeling figures, robed, faces turned to the manger
    fill(x + 35, y - 4, 2, 4, blue); fill(x + 35, y - 6, 2, 2, blue); dot(x + 36, y - 3, shaded(blue, 1.2));
    fill(x + 42, y - 4, 2, 4, brown); fill(x + 42, y - 6, 2, 2, brown); dot(x + 42, y - 3, shaded(brown, 1.2)); line(x + 44, y - 7, x + 44, y - 1, woodLight);
    // The star
    const tw = (now + x * 0.2) % 6 < 0.6;
    dot(x + 39, y - 12, gold); if (tw) { dot(x + 38, y - 12, gold, 0.6); dot(x + 40, y - 12, gold, 0.6); }
    if (isNight) glowLate(() => { glowAt(x + 39, y - 12, 3, 3, 0xffe08a, xmas ? 0.3 : 0.18); dot(x + 39, y - 12, 0xfff8d0); glowAt(x + 39, y - 4, 3, 2, 0xfff0b0, glow * 0.8); });
  },
  /** An Easter lily in a terracotta pot: white trumpet flowers with yellow hearts on a green stem with long leaves; a flower nods now and then. */
  christianlily(x, y) {
    const pot = 0xc8643a, potLight = 0xd8845a, potDark = 0x9a4a2a, leaf = 0x3f7a3a, leafLight = 0x5a9a4a, white = 0xfbf8f0, shade = 0xe2dccc, yellow = 0xf2c030;
    fill(x + 36, y - 4, 7, 4, pot); fill(x + 35, y - 5, 9, 1, potLight); fill(x + 42, y - 4, 1, 4, potDark);
    line(x + 39, y - 5, x + 39, y - 10, leaf); line(x + 39, y - 7, x + 42, y - 10, leaf); line(x + 39, y - 6, x + 36, y - 9, leaf);
    dot(x + 37, y - 7, leafLight); dot(x + 41, y - 7, leafLight);
    const t = (now + x * 0.21) % 16, nod = t < 2 ? Math.floor(t * 3) % 2 : 0;
    for (const [fx, fy, n] of [[36, -11, 0], [42, -11, nod]]) {
      fill(x + fx - 1, y + fy + n, 3, 1, white); fill(x + fx, y + fy + 1 + n, 1, 2, shade); dot(x + fx, y + fy + n, yellow);
    }
    fill(x + 39, y - 12, 1, 2, shade); dot(x + 39, y - 12, white);
  },
  /** A thick cream pillar candle on a brass plate, a drip of wax down its side; its flame wavers and glows warm, through the night too. */
  christiancandle(x, y) {
    const wax = 0xf6efdc, waxShade = 0xe0d6c0, brass = 0xc8a050, brassLight = 0xe8c878;
    fill(x + 35, y - 1, 9, 1, brass); dot(x + 35, y - 1, brassLight);
    fill(x + 37, y - 9, 5, 8, wax); fill(x + 37, y - 9, 1, 8, 0xfffaf0); fill(x + 41, y - 9, 1, 8, waxShade); fill(x + 37, y - 9, 5, 1, waxShade);
    fill(x + 40, y - 8, 1, 3, 0xfffaf0);
    const f = candlelight(x * 0.1);
    dot(x + 39, y - 10, 0x3a3238); dot(x + 39, y - 11, 0xffd36b, f); dot(x + 39, y - 12, 0xff8a3d, 0.55 * f);
    glowAt(x + 39, y - 11, 6, 5, 0xffc060, 0.08 * f);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 10, 8, 6, 0xffc060, 0.2 * f); dot(x + 39, y - 11, 0xfff4c0); });
  },
  /** A rosary of wooden beads coiled in a small white dish, its strand and little silver cross hanging over the rim; the cross catches the light now and then. */
  christianrosary(x, y) {
    const bead = 0x8a5a32, beadLight = 0xb07a48, dish = 0xd8d4cc, dishLight = 0xf2f0ea, silver = 0xc8ccd4;
    fill(x + 34, y - 3, 9, 1, dishLight); fill(x + 34, y - 2, 9, 2, dish); fill(x + 35, y - 2, 7, 1, 0xe8e4dc);
    for (let i = 0; i < 12; i++) { const a = (i / 12) * Math.PI * 2; dot(x + 38 + Math.round(Math.cos(a) * 3), y - 4 + Math.round(Math.sin(a) * 1.4), i % 4 === 0 ? beadLight : bead); }
    dot(x + 42, y - 3, bead); dot(x + 43, y - 2, bead);
    fill(x + 44, y - 4, 1, 4, silver); fill(x + 43, y - 3, 3, 1, silver);
    const g = (now + x * 0.17) % 13;
    if (g < 0.8) dot(x + 44, y - 3, WHITE);
  },
  /** A little wicker basket of painted Easter eggs in pink, blue and yellow on green straw; now and then the middle egg wobbles. */
  christianeggs(x, y) {
    const wicker = 0xb08a50, wickerDark = 0x8a6a38, straw = 0x8ac060, eggs = [0xf2a0b8, 0x8ab8e8, 0xf2d35b];
    for (const [hx, hy] of [[35, -4], [36, -7], [37, -9], [39, -10], [41, -9], [42, -7], [43, -4]]) dot(x + hx, y + hy, wickerDark);
    const t = (now + x * 0.19) % 14, wob = t < 1 ? (Math.floor(now * 8) % 2 ? 1 : -1) : 0;
    [[36, -7], [39, -8], [42, -7]].forEach(([ex, ey], i) => {
      const c = eggs[i], w = i === 1 ? wob : 0;
      fill(x + ex + w, y + ey, 2, 3, c); dot(x + ex + w, y + ey, shaded(c, 1.15)); fill(x + ex + w, y + ey + 1, 2, 1, shaded(c, 0.8));
    });
    fill(x + 35, y - 5, 9, 1, straw);
    fill(x + 34, y - 4, 11, 4, wicker); fill(x + 34, y - 4, 11, 1, shaded(wicker, 1.15));
    for (let k = 0; k < 11; k += 2) { dot(x + 34 + k, y - 3, wickerDark); dot(x + 35 + k, y - 2, wickerDark); }
  },
  /** A many-pointed paper star lamp on a small wooden stand, lit warm from inside: it glows softly by day and shines through the night, brighter from December to Epiphany. */
  christianstar(x, y) {
    const paper = 0xfbf6e8, warm = 0xffe08a, stand = 0x5a3b25, cx = x + 39, cy = y - 8, xmas = christmasTime();
    fill(cx - 2, y - 1, 5, 1, stand); fill(cx, y - 4, 1, 3, stand);
    const tw = 0.85 + 0.15 * Math.sin(now * 2 + x);
    const star = (a) => {
      for (const [dx, dy] of [[0, -4], [0, 4], [-4, 0], [4, 0], [-3, -3], [3, -3], [-3, 3], [3, 3]]) line(cx, cy, cx + dx, cy + dy, paper, a);
      disc(cx, cy, 1.6, paper, a); dot(cx, cy, warm, a);
    };
    star(1);
    glowAt(cx, cy, 6, 6, warm, (xmas ? 0.12 : 0.06) * tw);
    if (isNight) glowLate(() => { glowAt(cx, cy, 7, 7, warm, (xmas ? 0.24 : 0.16) * tw); dot(cx, cy, 0xfff4c0); });
  },
  /** Three hot cross buns on a plate, glazed brown with a white cross of icing on each; a wisp of warmth rises now and then. */
  christianbuns(x, y) {
    const bun = 0xc8843a, bunLight = 0xe0a050, bunDark = 0x9a5a24, icing = 0xfbf6e8, plate = 0xf2f0ea;
    fill(x + 34, y - 2, 11, 1, plate); fill(x + 35, y - 1, 9, 1, 0xd8d8de);
    for (const [bx, by] of [[39, -7], [36, -5], [42, -5]]) {
      fill(x + bx - 2, y + by - 1, 5, 3, bun); fill(x + bx - 1, y + by - 2, 3, 1, bun); dot(x + bx - 1, y + by - 2, bunLight); fill(x + bx + 2, y + by, 1, 2, bunDark);
      fill(x + bx - 1, y + by - 1, 3, 1, icing); fill(x + bx, y + by - 2, 1, 3, icing);
    }
    const t = (now + x * 0.25) % 17;
    if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 39 + (r % 2), y - 10 - r, WHITE, 0.45 - r * 0.1); }
  },
  /** A bronze singing bowl on a small red cushion with its wooden striker beside it; every so often it is struck and soft rings of sound spread out from it and fade. */
  buddhistbowl(x, y) {
    const bronze = 0xb8863a, bronzeLight = 0xe0b060, bronzeDark = 0x7a5420, cushion = 0xa8282a;
    fill(x + 34, y - 2, 10, 2, cushion); fill(x + 34, y - 2, 10, 1, shaded(cushion, 1.2));
    fill(x + 35, y - 6, 8, 1, bronzeLight); fill(x + 35, y - 5, 8, 2, bronze); fill(x + 36, y - 3, 6, 1, bronzeDark);
    fill(x + 36, y - 6, 6, 1, 0x5a3a18); dot(x + 35, y - 5, bronzeLight);
    line(x + 43, y - 1, x + 45, y - 4, 0x8a5a32);
    const t = (now + x * 0.23) % 15;
    if (t < 3) for (let k = 0; k < 2; k++) { const r = (t - k * 0.6) * 3; if (r > 0 && r < 7) ellipseRing(x + 39, y - 6, 4 + r, 1 + r * 0.4, bronzeLight, 0.5 * (1 - r / 7)); }
  },
  /** A thin incense stick in a little lotus-shaped holder, its tip glowing and a slow thread of smoke winding up from it. */
  buddhistincense(x, y) {
    const holder = 0xf2a0b8, holderDark = 0xd8708e, stick = 0x8a4a2a;
    fill(x + 36, y - 2, 7, 2, holder); dot(x + 36, y - 2, 0xf8c8d4); dot(x + 42, y - 2, holderDark); dot(x + 37, y - 3, holder); dot(x + 41, y - 3, holder); dot(x + 39, y - 3, holderDark);
    line(x + 39, y - 3, x + 41, y - 10, stick);
    dot(x + 41, y - 10, 0xe8502a, 0.7 + 0.3 * candlelight(x * 0.2));
    for (let k = 0; k < 4; k++) {
      const ph = (now * 0.4 + k / 4) % 1, sx = x + 41 + Math.round(Math.sin(ph * 6 + k) * 1.3), sy = y - 11 - Math.round(ph * 4);
      dot(sx, sy, 0xd8d4cc, 0.5 * (1 - ph));
    }
    if (isNight) glowLate(() => dot(x + 41, y - 10, 0xff8a50, 0.8));
  },
  /** A little bonsai pine in a shallow blue-glazed pot: a twisting trunk and three flat pads of needles; now and then a needle-tuft drops and falls to the pot. */
  buddhistbonsai(x, y) {
    const pot = 0x2f5f9e, potLight = 0x5a8ad0, trunk = 0x6a4a30, pad = 0x3f7a3a, padLight = 0x5a9a4a;
    fill(x + 34, y - 3, 11, 3, pot); fill(x + 34, y - 3, 11, 1, potLight); dot(x + 35, y, 0x1e3e6e); dot(x + 43, y, 0x1e3e6e);
    fill(x + 35, y - 4, 9, 1, 0x6a5040);
    line(x + 38, y - 4, x + 37, y - 7, trunk); line(x + 37, y - 7, x + 40, y - 9, trunk); line(x + 39, y - 8, x + 42, y - 7, trunk);
    for (const [px, py, w] of [[34, -9, 5], [39, -11, 5], [41, -8, 4]]) { fill(x + px, y + py, w, 2, pad); fill(x + px, y + py, w, 1, padLight); }
    const t = (now + x * 0.17) % 20;
    if (t < 1.5) dot(x + 43, y - 6 + Math.floor(t * 2), pad);
  },
  /** A shallow celadon dish of water with a pink lotus flower open on it beside a round green pad; now and then a ripple spreads across the water. */
  buddhistlotus(x, y) {
    const dish = 0x7aa898, dishLight = 0x9ac8b8, water = 0x8ac8d8, pink = 0xf2a0b8, pinkLight = 0xf8c8d4, pinkDark = 0xd8708e, pad = 0x4a8a4a;
    fill(x + 34, y - 3, 11, 1, dishLight); fill(x + 35, y - 2, 9, 2, dish); fill(x + 35, y - 3, 9, 1, water);
    fill(x + 41, y - 4, 3, 1, pad); dot(x + 42, y - 5, pad);
    fill(x + 37, y - 5, 3, 2, pink); dot(x + 36, y - 6, pink); dot(x + 40, y - 6, pink); dot(x + 38, y - 7, pinkLight); dot(x + 38, y - 5, 0xf2d35b);
    dot(x + 37, y - 6, pinkDark); dot(x + 39, y - 6, pinkDark);
    const t = (now + x * 0.13) % 12;
    if (t < 1.5) { const r = Math.floor(t * 2); dot(x + 37 - r - 1, y - 3, WHITE, 0.5); dot(x + 40 + r, y - 3, WHITE, 0.5); }
  },
  /** A round brown clay teapot with a short spout and a small handleless cup of green tea beside it; a wisp of steam rises from the cup now and then. */
  buddhisttea(x, y) {
    const clay = 0x8a4a2a, clayLight = 0xa8603a, clayDark = 0x5a2e18, cup = 0xf2ead8, tea = 0x9ab850;
    fill(x + 34, y - 6, 6, 5, clay); fill(x + 35, y - 7, 4, 1, clay); fill(x + 35, y - 1, 4, 1, clayDark);
    fill(x + 34, y - 6, 1, 5, clayLight); fill(x + 39, y - 6, 1, 5, clayDark); dot(x + 36, y - 8, clayLight);
    dot(x + 40, y - 5, clay); dot(x + 41, y - 6, clay); dot(x + 33, y - 5, clayDark); dot(x + 33, y - 3, clayDark);
    fill(x + 42, y - 3, 3, 3, cup); fill(x + 42, y - 3, 3, 1, tea); dot(x + 44, y - 1, 0xd8d0c0);
    const t = (now + x * 0.23) % 12;
    if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 43 + (r % 2), y - 5 - r, WHITE, 0.45 - r * 0.1); }
  },
  /** A little wooden tray of pale sand raked into lines round two smooth stones, a tiny rake resting on its rim; every so often the rake draws a fresh line through the sand. */
  buddhistzentray(x, y) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, sand = 0xeee4cc, groove = 0xd4c8a8, stone = 0x6a6e72;
    fill(x + 33, y - 5, 13, 5, wood); fill(x + 33, y - 5, 13, 1, woodLight);
    fill(x + 34, y - 4, 11, 3, sand);
    for (let k = 0; k < 11; k += 2) dot(x + 34 + k, y - 3, groove);
    fill(x + 36, y - 4, 2, 2, stone); dot(x + 36, y - 4, 0x8a8e92); fill(x + 41, y - 3, 2, 1, stone);
    const t = (now + x * 0.27) % 22, at = t < 3 ? Math.floor((t / 3) * 10) : 10;
    line(x + 34 + at, y - 2, x + 36 + at, y - 7, 0x8a6a40); fill(x + 33 + at, y - 2, 3, 1, 0x6a4a28);
  },
  /** A brass butter lamp shaped like a goblet, its little flame wavering over the butter and glowing warm, through the night too. */
  buddhistbutterlamp(x, y) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a;
    fill(x + 36, y - 1, 7, 1, brassDark); fill(x + 38, y - 4, 3, 3, brass); dot(x + 38, y - 3, brassLight);
    fill(x + 36, y - 7, 7, 3, brass); fill(x + 36, y - 7, 1, 3, brassLight); fill(x + 42, y - 7, 1, 3, brassDark); fill(x + 37, y - 8, 5, 1, 0xf2e2a0);
    const f = candlelight(x * 0.13);
    dot(x + 39, y - 9, 0xffd36b, f); dot(x + 39, y - 10, 0xff8a3d, 0.55 * f);
    glowAt(x + 39, y - 9, 5, 4, 0xffc060, 0.08 * f);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 9, 7, 5, 0xffc060, 0.2 * f); dot(x + 39, y - 9, 0xfff4c0); });
  },
  /** Three smooth river stones balanced one on another, largest at the bottom; every so often the top one wobbles and settles. */
  buddhistcairn(x, y) {
    const stones = [[39, -2, 7, 3, 0x6a6e72], [39, -5, 5, 2, 0x7a7e82], [39, -8, 3, 2, 0x5a5e62]];
    const t = (now + x * 0.19) % 18, wob = t < 1.2 ? (Math.floor(now * 8) % 2 ? 1 : -1) : 0;
    fill(x + 35, y, 9, 1, SHADOW);
    stones.forEach(([cx, cy, w, h, c], i) => {
      const dx = i === 2 ? wob : 0, left = x + cx - (w >> 1) + dx;
      fill(left, y + cy - h + 1, w, h, c); fill(left + 1, y + cy - h, w - 2, 1, c);
      dot(left + 1, y + cy - h, shaded(c, 1.25)); dot(left + w - 1, y + cy, shaded(c, 0.8));
    });
  },
  /** The big red button: a domed red button on a yellow and black base under a clear hinged cover; every 25 s the cover flips up and the button glows, then it closes again. */
  volcanobutton(x, y) {
    const steel = 0x4a4e58, yellow = 0xf2c830, black = 0x1e1e22, red = 0xd8281e;
    fill(x + 34, y - 3, 11, 3, steel); fill(x + 34, y - 3, 11, 1, 0x6e727c);
    for (let k = 0; k < 11; k += 2) { dot(x + 34 + k, y - 1, yellow); dot(x + 35 + k, y - 1, black); }
    const t = (now + x * 0.29) % 25, armed = t < 4;
    fill(x + 37, y - 5, 5, 2, red); fill(x + 38, y - 6, 3, 1, red); dot(x + 38, y - 6, 0xf27a6a);
    if (armed) {
      line(x + 42, y - 4, x + 45, y - 10, 0xdff0f6, 0.6); line(x + 41, y - 4, x + 44, y - 10, 0xdff0f6, 0.4);
      glowAt(x + 39, y - 5, 4, 3, 0xff3020, 0.25);
    } else {
      fill(x + 36, y - 8, 7, 5, 0xdff0f6, 0.35); fill(x + 36, y - 8, 7, 1, 0xdff0f6, 0.7); dot(x + 37, y - 7, WHITE, 0.7);
    }
    if (isNight) glowLate(() => { if (armed) glowAt(x + 39, y - 5, 5, 4, 0xff3020, 0.3); dot(x + 39, y - 6, armed ? 0xff8070 : 0xa83020); });
  },
  /** A red hotline telephone with its handset on the cradle and a little white light; every 30 s it rings, the handset jumping on its rest and the light flashing. */
  volcanophone(x, y) {
    const red = 0xc8281e, redLight = 0xe85a4a, redDark = 0x8a1a14;
    const ring = (now + x * 0.23) % 30 < 2, j = ring ? Math.floor(now * 12) % 2 : 0;
    fill(x + 35, y - 4, 9, 4, red); fill(x + 36, y - 5, 7, 1, red); fill(x + 35, y - 4, 9, 1, redLight); fill(x + 35, y - 1, 9, 1, redDark);
    fill(x + 34, y - 7 - j, 11, 2, red); fill(x + 34, y - 7 - j, 11, 1, redLight); fill(x + 34, y - 6 - j, 2, 2, red); fill(x + 43, y - 6 - j, 2, 2, red);
    disc(x + 39, y - 2, 1, redDark);
    dot(x + 43, y - 3, ring && Math.floor(now * 4) % 2 ? 0xffffff : 0xd8d4cc);
    line(x + 34, y - 5, x + 33, y - 1, 0x2a2a30);
    if (isNight) glowLate(() => { if (ring) glowAt(x + 43, y - 3, 3, 3, 0xffffff, 0.3); });
  },
  /** A desk globe on a brass meridian, its land turning a step now and then, a red target sight blinking over one spot of it. */
  volcanoglobe(x, y) {
    const brass = 0xc8a050, brassDark = 0x8a6a2a, sea = 0x2f6aa8, land = 0x5a9a4a, cx = x + 39, cy = y - 7;
    fill(x + 36, y - 1, 7, 1, brassDark); fill(x + 39, y - 2, 1, 1, brass);
    disc(cx, cy, 4.2, sea); ring(cx, cy, 5, brass, 0.8);
    const turn = Math.floor((now + x * 0.3) / 3) % 6;
    for (const [lx, ly] of [[-2, -2], [-1, -2], [-2, -1], [1, 0], [2, 0], [1, 1], [-1, 2], [0, 2]]) {
      const sx = ((lx + turn + 6) % 7) - 3; if (Math.hypot(sx, ly) < 4) dot(cx + sx, cy + ly, land);
    }
    dot(cx - 2, cy - 3, WHITE, 0.5);
    if (Math.floor(now * 2 + x) % 3) { ring(cx + 1, cy, 1.8, 0xe8302a, 0.9); dot(cx + 1, cy, 0xe8302a); }
  },
  /** A seismograph: a drum of paper slowly turning under a fine pen that scratches a wavy line; every 40 s the volcano rumbles, the pen jumps into a big spike and the case shakes. */
  volcanoseismo(x, y) {
    const steel = 0x6e727c, steelDark = 0x4a4e58, paper = 0xf4eedc, ink = 0x2a2a30, red = 0xd8281e;
    const quake = (now + x * 0.31) % 40 < 2.5, sh = quake ? Math.floor(now * 14) % 2 : 0;
    fill(x + 33 + sh, y - 2, 12, 2, steelDark); fill(x + 34 + sh, y - 9, 10, 7, steel); fill(x + 34 + sh, y - 9, 10, 1, 0xa8adb6);
    fill(x + 35 + sh, y - 8, 8, 5, paper);
    for (let k = 0; k < 8; k++) {
      const ph = now * 3 - k, amp = quake && k < 4 ? 2 : 0.6;
      dot(x + 35 + sh + k, y - 6 + Math.round(Math.sin(ph * 2.3 + hash(Math.floor(ph), 3) % 3) * amp), ink);
    }
    line(x + 42 + sh, y - 6, x + 44 + sh, y - 11, steelDark); dot(x + 42 + sh, y - 6 + (quake ? (Math.floor(now * 10) % 3) - 1 : 0), red);
  },
  /** A cluster of black obsidian shards on a chunk of rock, glassy with a purple sheen; every 12 s a gleam runs up the tallest. */
  volcanoobsidian(x, y) {
    const obs = 0x1a1424, sheen = 0x5a4a7a, rock = 0x4a403c;
    fill(x + 34, y - 2, 11, 2, rock); fill(x + 34, y - 2, 11, 1, 0x6a5e58);
    for (const [sx, h, w] of [[36, 6, 2], [38, 10, 3], [41, 7, 2], [43, 4, 1]]) {
      fill(x + sx, y - 2 - h, w, h, obs); dot(x + sx, y - 2 - h, sheen); fill(x + sx, y - 1 - h, 1, Math.max(1, h - 3), sheen, 0.6);
    }
    const t = (now + x * 0.19) % 12;
    if (t < 1.2) dot(x + 39, y - 3 - Math.floor(t * 7), WHITE, 0.9);
  },
  /** A magma paperweight: a glass dome over a slow pool of glowing lava whose bright core swells and fades; it glows warm, through the night too. */
  volcanomagma(x, y) {
    const base = 0x2a2226, lava = 0xe8501e, hot = 0xffa040;
    fill(x + 35, y - 1, 9, 1, base);
    const pulse = (Math.sin(now * 1.2 + x * 0.2) + 1) / 2;
    const pool = () => { fill(x + 36, y - 4, 7, 3, lava); fill(x + 37, y - 5, 5, 1, lava); fill(x + 38, y - 4, 3, 1 + Math.round(pulse), hot); dot(x + 39, y - 4, 0xffe080, pulse); };
    pool();
    fill(x + 36, y - 7, 7, 6, 0xdff0f6, 0.3); fill(x + 37, y - 8, 5, 1, 0xdff0f6, 0.5); dot(x + 37, y - 7, WHITE, 0.8);
    glowAt(x + 39, y - 4, 5, 4, 0xff6020, 0.08 + pulse * 0.06);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 4, 6, 5, 0xff6020, 0.18 + pulse * 0.1); pool(); });
  },
  /** Secret plans: a blueprint unrolled on the desk under a paperweight of a bolt, its white lines drawing a rocket on blue, one roll still curled beside it; the red mark on it blinks. */
  volcanoplans(x, y) {
    const blue = 0x2a5aa8, blueLight = 0x4a7ac8, lineC = 0xdfe8f6;
    fill(x + 33, y - 3, 10, 3, blue); fill(x + 33, y - 3, 10, 1, blueLight);
    line(x + 35, y - 2, x + 40, y - 2, lineC); dot(x + 41, y - 2, lineC); dot(x + 36, y - 3, lineC, 0.7); dot(x + 39, y - 1, lineC, 0.7);
    if (Math.floor(now * 1.5 + x) % 3) dot(x + 38, y - 2, 0xe8302a);
    fill(x + 42, y - 4, 3, 4, blueLight); fill(x + 42, y - 4, 3, 1, 0x6a9ae8); dot(x + 43, y - 3, blue);
    fill(x + 36, y - 9, 7, 2, blue); fill(x + 36, y - 9, 7, 1, blueLight); dot(x + 42, y - 9, 0x1e3e7e); dot(x + 42, y - 8, 0x1e3e7e);
    fill(x + 34, y - 5, 3, 2, 0x6e727c); dot(x + 34, y - 5, 0xa8adb6);
  },
  /** A launch key in its slot on a little steel box with a green lamp; every 40 s the key turns and the lamp lights for a moment, then it turns back. */
  volcanokey(x, y) {
    const steel = 0x5a5e68, steelLight = 0x7e828c, brass = 0xe8c878, yellow = 0xf2c830, black = 0x1e1e22;
    fill(x + 34, y - 6, 10, 6, steel); fill(x + 34, y - 6, 10, 1, steelLight); fill(x + 43, y - 6, 1, 6, 0x3a3c44);
    for (let k = 0; k < 10; k += 2) { dot(x + 34 + k, y - 1, yellow); dot(x + 35 + k, y - 1, black); }
    disc(x + 38, y - 3, 1.6, 0x2a2c34);
    const turned = (now + x * 0.21) % 40 < 3;
    if (turned) { fill(x + 37, y - 3, 4, 1, brass); fill(x + 40, y - 4, 2, 3, brass); } else { fill(x + 38, y - 7, 1, 4, brass); fill(x + 37, y - 9, 3, 2, brass); }
    dot(x + 42, y - 4, turned ? 0x5aff6a : 0x1e5a2a);
    if (isNight && turned) glowLate(() => { glowAt(x + 42, y - 4, 3, 3, 0x5aff6a, 0.3); dot(x + 42, y - 4, 0xc0ffc8); });
  },
  /** Mise en place: a little steel tray of four ramekins, finely chopped shallot, chives, red chilli and flaky salt, with a tasting spoon laid beside them. */
  kitchenmise(x, y) {
    const steel = 0xc8ccd2;
    fill(x + 33, y - 1, 12, 1, 0x8a9098); fill(x + 33, y - 2, 12, 1, steel);
    [[34, 0xc8a0c0], [37, 0x4a9a3a], [40, 0xd8402a], [43, 0xf6f6f2]].forEach(([rx, c], k) => {
      fill(x + rx, y - 5, 3, 3, 0xf6f6f2); fill(x + rx, y - 5, 3, 1, c); dot(x + rx + (k % 2), y - 4, c, 0.8); dot(x + rx + 2, y - 3, 0xdcdcd8);
    });
    dot(x + 43, y - 5, 0xdcdcd8); line(x + 34, y - 7, x + 39, y - 7, 0xa8adb6); dot(x + 33, y - 7, 0xc8ccd2); dot(x + 33, y - 8, 0xc8ccd2);
  },
  /** A wooden chopping board with a neat pile of diced carrot and a chef's knife; every 8 s the knife rocks through a few quick chops. */
  kitchenboard(x, y) {
    const wood = 0xc8945a, woodDark = 0x8a5a32, carrot = 0xf08a2a, blade = 0xd8dce2;
    fill(x + 33, y - 3, 13, 3, wood); fill(x + 33, y - 3, 13, 1, 0xe0b078); fill(x + 33, y - 1, 13, 1, woodDark);
    for (const [cx, cy] of [[35, -4], [37, -4], [36, -5], [38, -4], [39, -5]]) dot(x + cx, y + cy, carrot);
    const t = (now + x * 0.27) % 8, chop = t < 1.6 ? Math.floor(t * 6) % 2 : 0;
    line(x + 40, y - 4 - chop, x + 44, y - 6 - chop, blade); fill(x + 41, y - 5 - chop, 2, 1, blade); dot(x + 40, y - 4 - chop, 0xeef2f6);
    line(x + 44, y - 6 - chop, x + 46, y - 7 - chop, 0x2a2220);
  },
  /** A small copper saucepan of brown sauce reducing with a wooden spoon in it; the sauce bubbles and steam curls up from it. */
  kitchensauce(x, y) {
    const copper = 0xc87a3a, copperLight = 0xe8a060, copperDark = 0x8a4a22, sauce = 0x6a2e1a;
    fill(x + 35, y - 6, 8, 6, copper); fill(x + 35, y - 6, 1, 6, copperLight); fill(x + 42, y - 6, 1, 6, copperDark); fill(x + 34, y - 6, 10, 1, copperLight);
    fill(x + 43, y - 5, 3, 1, 0x3a3a40); fill(x + 36, y - 5, 6, 1, sauce);
    if (Math.floor(now * 3 + x) % 4 === 0) dot(x + 37 + (Math.floor(now + x) % 4), y - 6, 0x8a4a2a);
    line(x + 38, y - 5, x + 36, y - 11, 0xc8a070);
    for (let k = 0; k < 3; k++) { const ph = (now * 0.5 + k / 3) % 1; dot(x + 39 + Math.round(Math.sin(ph * 6 + k)), y - 7 - Math.round(ph * 5), WHITE, 0.5 * (1 - ph)); }
  },
  /** The service bell on the pass, chrome on a black base; every 20 s it's struck for a pick-up, its button dipping and rings of sound spreading. */
  kitchenbell(x, y) {
    const chrome = 0xd8dce2, chromeLight = 0xf6f8fa, chromeDark = 0x8a9098;
    fill(x + 35, y - 2, 9, 2, 0x2a2a30); fill(x + 35, y - 2, 9, 1, 0x4a4a52);
    fill(x + 36, y - 5, 7, 3, chrome); fill(x + 37, y - 6, 5, 1, chrome); fill(x + 36, y - 5, 1, 3, chromeLight); fill(x + 42, y - 5, 1, 3, chromeDark);
    const t = (now + x * 0.31) % 20, hit = t < 0.3;
    fill(x + 39, y - 7 + (hit ? 1 : 0), 1, 1, chromeDark); dot(x + 39, y - 8 + (hit ? 1 : 0), 0x2a2a30);
    if (t < 1.8) { const r = 2 + t * 4; ellipseRing(x + 39, y - 5, r, r * 0.6, chromeLight, 0.6 * (1 - t / 1.8)); }
  },
  /** A plated dish waiting for the pass: a white plate with a seared fillet, a quenelle of purée, a dotted swoosh of sauce and a green herb; every 16 s a pair of tweezers comes down to set the herb just so. */
  kitchenplate(x, y) {
    fill(x + 33, y - 3, 13, 3, 0xf6f6f2); fill(x + 34, y - 4, 11, 1, 0xf6f6f2); fill(x + 33, y - 1, 13, 1, 0xdcdcd8);
    fill(x + 36, y - 4, 4, 2, 0xa8642a); fill(x + 36, y - 4, 4, 1, 0xc88040);
    fill(x + 41, y - 4, 2, 1, 0xf2dca0);
    for (const dx of [34, 35, 43, 44]) dot(x + dx, y - 2, 0x8a2a1e);
    dot(x + 38, y - 5, 0x4a9a3a);
    const t = (now + x * 0.37) % 16;
    if (t < 2) { const d = t < 1 ? Math.floor(t * 4) : 4 - Math.floor((t - 1) * 4); line(x + 39, y - 6 - (4 - d), x + 42, y - 11 - (4 - d), 0xa8adb6); }
  },
  /** A round white kitchen timer whose red dial creeps round; every 30 s it goes off, shaking on the bench with little ring marks. */
  kitchentimer(x, y) {
    const t = (now + x * 0.23) % 30, going = t < 1.6, sh = going ? (Math.floor(now * 14) % 2 ? 1 : -1) : 0, cx = x + 39 + sh, cy = y - 4;
    fill(x + 36, y - 1, 7, 1, SHADOW);
    disc(cx, cy, 3.4, 0xf6f6f2); if (going) ellipseRing(cx, cy, 5, 4, 0xd8402a, 0.5);
    dot(cx, cy - 4, 0xd8402a); fill(cx - 1, cy - 4, 3, 1, 0xd8402a);
    const a = (t / 30) * Math.PI * 2 - Math.PI / 2;
    line(cx, cy, cx + Math.round(Math.cos(a) * 2.4), cy + Math.round(Math.sin(a) * 2.4), 0xd8402a);
    dot(cx - 2, cy - 2, WHITE, 0.8); dot(cx + 2, cy + 2, 0xdcdcd8);
  },
  /** A cream-glazed salt pig, its open mouth full of flaky salt, beside a tall wooden pepper mill that grinds a pinch now and then. */
  kitchensalt(x, y) {
    const glaze = 0xeee4cc, glazeDark = 0xc8b898, wood = 0x6a4428;
    fill(x + 34, y - 6, 7, 6, glaze); fill(x + 35, y - 7, 5, 1, glaze); fill(x + 34, y - 6, 1, 6, 0xfaf4e4); fill(x + 40, y - 6, 1, 6, glazeDark);
    fill(x + 36, y - 5, 4, 2, 0x5a4a3a); dot(x + 37, y - 4, 0xf6f6f2); dot(x + 38, y - 5, 0xf6f6f2);
    fill(x + 42, y - 10, 3, 10, wood); fill(x + 42, y - 10, 1, 10, 0x8a5a32); fill(x + 42, y - 6, 3, 1, 0x4a2e18); dot(x + 43, y - 11, 0xa8adb6);
    const t = (now + x * 0.19) % 11;
    if (t < 1.2) for (let k = 0; k < 2; k++) dot(x + 43 - k, y + 1 - Math.floor((1.2 - t) * 3) + k, 0x2a2a2a, 0.7);
  },
  /** A round-bottomed copper bowl of cream with a balloon whisk in it beating to and fro, soft peaks forming. */
  kitchenwhisk(x, y) {
    const copper = 0xc87a3a, copperLight = 0xe8a060, copperDark = 0x8a4a22;
    fill(x + 34, y - 5, 11, 1, copperLight); fill(x + 34, y - 4, 11, 2, copper); fill(x + 35, y - 2, 9, 1, copper); fill(x + 36, y - 1, 7, 1, copperDark);
    fill(x + 35, y - 5, 9, 1, 0xfaf6ea); dot(x + 37, y - 6, 0xfaf6ea); dot(x + 41, y - 6, 0xfaf6ea);
    const w = Math.round(Math.sin(now * 6 + x) * 2);
    ellipseRing(x + 39 + w, y - 7, 2, 3, 0xd8dce2, 0.9); line(x + 39 + w, y - 10, x + 42 + w, y - 13, 0x8a9098);
  },
  /** A stethoscope coiled on the desk: black tubing in a loop, steel earpieces and a shiny chest piece that catches the light now and then. */
  hospitalstetho(x, y) {
    const tube = 0x2a2a30, steel = 0xc8ccd2;
    ellipseRing(x + 39, y - 3, 5, 2, tube); ellipseRing(x + 39, y - 3, 4, 1.4, tube, 0.6);
    line(x + 35, y - 4, x + 34, y - 8, steel); line(x + 37, y - 4, x + 38, y - 8, steel); dot(x + 34, y - 9, tube); dot(x + 38, y - 9, tube);
    disc(x + 44, y - 2, 1.6, steel); dot(x + 44, y - 3, WHITE, (now + x * 0.2) % 9 < 0.8 ? 1 : 0.5);
  },
  /** A weekly pill organiser of seven little lids in rainbow colours; now and then one lid pops open to show the pills inside, then shuts. */
  hospitalpills(x, y) {
    const cols = [0xd8402a, 0xe8782e, 0xf2d35b, 0x3ab86a, 0x4c8bf5, 0x8a6ad8, 0xf28fb0];
    fill(x + 33, y - 3, 13, 3, 0xf2f4f4); fill(x + 33, y - 1, 13, 1, 0xd8dee2);
    const open = Math.floor((now + x * 0.3) / 6) % 7, up = (now + x * 0.3) % 6 < 2;
    cols.forEach((c, k) => {
      const lx = x + 33 + k * 2 - (k > 5 ? 1 : 0);
      if (up && k === open) { fill(lx, y - 6, 2, 1, c); dot(lx, y - 3, WHITE); dot(lx + 1, y - 3, 0xf2d35b); }
      else fill(lx, y - 4, 2, 1, c);
    });
  },
  /** A digital thermometer in its little stand, its tip silver and a tiny screen whose reading blinks after a beep every 15 s. */
  hospitalthermo(x, y) {
    fill(x + 37, y - 2, 6, 2, 0x8a9098); fill(x + 37, y - 2, 6, 1, 0xa8adb6);
    fill(x + 39, y - 11, 3, 9, 0xf2f4f4); fill(x + 39, y - 11, 1, 9, WHITE); dot(x + 40, y - 12, 0xc8ccd2);
    fill(x + 39, y - 8, 3, 2, 0xb8d4c0); const beep = (now + x * 0.29) % 15 < 2 && Math.floor(now * 4) % 2;
    if (beep) { dot(x + 39, y - 8, 0x1e1e22); dot(x + 41, y - 7, 0x1e1e22); }
    dot(x + 40, y - 4, 0x4c8bf5);
  },
  /** The night shift's coffee: a tall paper cup with a cardboard sleeve and a lid, steam rising from its sip hole; a second empty cup lies crushed beside it. */
  hospitalcoffee(x, y) {
    fill(x + 36, y - 9, 5, 9, 0xf6f4ee); fill(x + 35, y - 10, 7, 1, 0xf6f4ee); fill(x + 35, y - 11, 7, 1, 0x2a2a30); fill(x + 36, y - 6, 5, 3, 0xa8784a); fill(x + 40, y - 9, 1, 9, 0xd8d6ce);
    fill(x + 42, y - 2, 3, 2, 0xe8e6de); dot(x + 43, y - 3, 0xd8d6ce); dot(x + 44, y - 2, 0xa8784a);
    for (let k = 0; k < 3; k++) { const ph = (now * 0.5 + k / 3) % 1; dot(x + 39 + Math.round(Math.sin(ph * 6 + k)), y - 12 - Math.round(ph * 5), WHITE, 0.5 * (1 - ph)); }
  },
  /** A plastic anatomical heart model on a little stand, red muscle with blue and red vessels at the top; every so often it gives two beats. */
  hospitalheart(x, y) {
    fill(x + 37, y - 1, 7, 1, 0x2a2a30); fill(x + 40, y - 3, 1, 2, 0x8a9098);
    const t = (now + x * 0.21) % 7, beat = t < 0.15 || (t > 0.3 && t < 0.45) ? 1 : 0;
    fill(x + 37 - beat, y - 8 - beat, 6 + beat * 2, 4 + beat, 0xc8302a); fill(x + 38, y - 4, 4, 1, 0xc8302a); dot(x + 40, y - 3, 0xa82a24);
    dot(x + 38, y - 8, 0xe85a4a); fill(x + 41, y - 10, 2, 2, 0x3a6ac8); fill(x + 38, y - 10, 2, 2, 0xd8402a); dot(x + 40, y - 11, 0xd8402a);
  },
  /** A black pager on the desk, its little green screen showing a line; every 25 s it buzzes, jittering on the desk, its light flashing. */
  hospitalpager(x, y) {
    const t = (now + x * 0.37) % 25, buzz = t < 2, j = buzz ? Math.floor(now * 14) % 2 : 0;
    fill(x + 36 + j, y - 4, 8, 4, 0x2a2a30); fill(x + 36 + j, y - 4, 8, 1, 0x4a4a52);
    fill(x + 37 + j, y - 3, 4, 2, 0x8ac89a); fill(x + 37 + j, y - 2, 3, 1, 0x2a4a3a, 0.7);
    dot(x + 42 + j, y - 3, buzz && Math.floor(now * 6) % 2 ? 0xff4030 : 0x5a1a14);
    if (buzz) { dot(x + 35, y - 3, 0x8a8a90, 0.6); dot(x + 45, y - 3, 0x8a8a90, 0.6); }
  },
  /** A roll of white crêpe bandage half unrolled, a box of plasters and a pair of blunt-nosed scissors. */
  hospitalbandage(x, y) {
    fill(x + 34, y - 5, 5, 5, 0xf6f4ee); fill(x + 34, y - 5, 5, 1, WHITE); ring(x + 36, y - 3, 1, 0xd8d6ce); fill(x + 39, y - 1, 6, 1, 0xf6f4ee);
    fill(x + 40, y - 6, 5, 4, 0xe8a070); fill(x + 40, y - 6, 5, 1, 0xf0b888); fill(x + 41, y - 5, 3, 1, WHITE);
    line(x + 37, y - 8, x + 42, y - 10, 0xc8ccd2); dot(x + 36, y - 8, 0x4c8bf5); dot(x + 36, y - 7, 0x4c8bf5);
  },
  /** A reflex hammer with its red rubber triangle head laid by a jar of wooden tongue depressors; every 18 s the hammer taps. */
  hospitalhammer(x, y) {
    fill(x + 35, y - 7, 4, 7, 0xdff0f6, 0.5); fill(x + 35, y - 7, 4, 1, 0xa8adb6);
    for (let k = 0; k < 3; k++) fill(x + 35 + k + (k > 1 ? 1 : 0), y - 10 + k, 1, 6, 0xe8d0a0);
    const t = (now + x * 0.23) % 18, tap = t < 0.8 && Math.floor(t * 6) % 2;
    line(x + 40, y - 1, x + 45, y - 3 - tap, 0xc8ccd2); fill(x + 43, y - 4 - tap, 3, 2, 0xd8402a); dot(x + 44, y - 5 - tap, 0xd8402a);
  },
  /** A grey felt fedora laid on the desk, its black band and a pinched crown; now and then it's tipped up, as if its owner were thinking. */
  noirfedora(x, y) {
    const felt = 0x6a6a70, light = 0x8a8a90, band = 0x1e1e22, t = (now + x * 0.19) % 20, tip = t < 1.5 ? 1 : 0;
    fill(x + 34, y - 2, 11, 2, felt); fill(x + 34, y - 2, 11, 1, light);
    fill(x + 36, y - 6 - tip, 7, 4, felt); fill(x + 36, y - 6 - tip, 7, 1, light); dot(x + 39, y - 6 - tip, 0x5a5a60);
    fill(x + 36, y - 3 - tip, 7, 1, band);
  },
  /** A brass magnifying glass laid over a fingerprint card; every 14 s it slides along to study the print, the whorls showing larger under the lens. */
  noirmagnifier(x, y) {
    fill(x + 33, y - 3, 11, 3, 0xf2ecd8); fill(x + 33, y - 3, 11, 1, 0xfaf6e8);
    for (const [px, py] of [[35, -2], [36, -2], [37, -1], [39, -2], [40, -2], [41, -1]]) dot(x + px, y + py, 0x3a3a40, 0.7);
    const t = (now + x * 0.23) % 14, sl = t < 3 ? Math.round(Math.sin((t / 3) * Math.PI) * 3) : 0, cx = x + 38 + sl, cy = y - 5;
    ring(cx, cy, 2.6, 0xc8a050); disc(cx, cy, 1.8, 0xdff0f6, 0.45); dot(cx - 1, cy - 1, WHITE, 0.8); dot(cx, cy, 0x3a3a40, 0.6);
    line(cx + 2, cy + 2, cx + 5, cy + 4, 0x4a2e1e);
  },
  /** A gold detective's shield in an open black leather wallet; a glint crosses the badge now and then. */
  noirbadge(x, y) {
    fill(x + 34, y - 6, 11, 6, 0x1e1e22); fill(x + 34, y - 6, 11, 1, 0x3a3a40); fill(x + 39, y - 6, 1, 6, 0x121214);
    fill(x + 41, y - 5, 3, 4, 0xd8b45a); dot(x + 42, y - 6, 0xd8b45a); dot(x + 42, y - 1, 0xd8b45a); dot(x + 42, y - 3, 0x8a6a2a);
    fill(x + 35, y - 5, 3, 3, 0xe8e4d8); dot(x + 36, y - 4, 0x8a8a90);
    if ((now + x * 0.31) % 10 < 0.6) dot(x + 41, y - 5, WHITE);
  },
  /** A thick white diner mug of black coffee, steaming, and a glazed donut with sprinkles on a napkin beside it. */
  noirdonut(x, y) {
    fill(x + 34, y - 7, 5, 7, 0xf2f0ea); fill(x + 34, y - 7, 5, 1, 0x2a1a12); fill(x + 38, y - 7, 1, 7, 0xd8d4cc); fill(x + 39, y - 5, 1, 3, 0xf2f0ea); dot(x + 40, y - 4, 0xf2f0ea);
    fill(x + 40, y - 1, 5, 1, 0xf6f6f2);
    fill(x + 40, y - 4, 5, 3, 0xd8a058); fill(x + 41, y - 4, 3, 1, 0xf28fb0); dot(x + 42, y - 3, 0x8a5a32);
    for (const [sx, c] of [[40, 0x4c8bf5], [43, 0xf2d35b], [44, 0x3ab86a]]) dot(x + sx, y - 4, c);
    for (let k = 0; k < 3; k++) { const ph = (now * 0.5 + k / 3) % 1; dot(x + 36 + Math.round(Math.sin(ph * 6 + k)), y - 8 - Math.round(ph * 4), WHITE, 0.5 * (1 - ph)); }
  },
  /** A manila case file, fat with papers and a photo clip, a red stamp block on its cover and a paper clip; every 18 s its cover lifts in a draught. */
  noircasefile(x, y) {
    fill(x + 33, y - 2, 12, 2, 0xd8b878); fill(x + 33, y - 2, 12, 1, 0xf2ecd8);
    const t = (now + x * 0.27) % 18, lift = t < 1.5 ? 1 : 0;
    fill(x + 33, y - 4 - lift, 12, 2, 0xe8c888); fill(x + 33, y - 4 - lift, 12, 1, 0xf2d898); fill(x + 35, y - 4 - lift, 4, 1, 0xd8302a, 0.8);
    fill(x + 42, y - 6 - lift, 1, 3, 0xa8adb6); fill(x + 40, y - 7 - lift, 3, 3, 0x6a6a70); dot(x + 41, y - 6 - lift, 0x9a9aa0);
  },
  /** A pair of steel handcuffs lying on the desk, two rings joined by a short chain, catching the light. */
  noircuffs(x, y) {
    ellipseRing(x + 37, y - 2, 2.4, 1.6, 0xa8adb6); ellipseRing(x + 43, y - 2, 2.4, 1.6, 0xa8adb6);
    line(x + 39, y - 3, x + 41, y - 3, 0x6e727c); dot(x + 40, y - 2, 0x6e727c);
    fill(x + 34, y - 2, 1, 1, 0x6e727c); fill(x + 45, y - 2, 1, 1, 0x6e727c);
    if ((now + x * 0.2) % 11 < 0.6) dot(x + 36, y - 3, WHITE);
  },
  /** A reel-to-reel tape recorder in a grey case, its two reels turning as it records a statement, a red light on and a needle twitching. */
  noirreel(x, y) {
    fill(x + 33, y - 7, 13, 7, 0x6a6a70); fill(x + 33, y - 7, 13, 1, 0x8a8a90); fill(x + 33, y - 1, 13, 1, 0x4a4a52);
    const a = now * 3;
    for (const cx of [x + 36, x + 43]) { disc(cx, y - 5, 2.2, 0x2a2a30); dot(cx + Math.round(Math.cos(a)), y - 5 + Math.round(Math.sin(a)), 0xa8adb6); }
    fill(x + 38, y - 3, 3, 1, 0x3a2a20); dot(x + 39, y - 2, Math.floor(now * 2) % 2 ? 0xff3020 : 0x8a1a14);
  },
  /** A police radio set with a coiled cord to its handset, a dial and a light that blinks with chatter every few seconds. */
  noirradio(x, y) {
    fill(x + 33, y - 6, 9, 6, 0x3a3a40); fill(x + 33, y - 6, 9, 1, 0x5a5a62); for (let k = 0; k < 3; k++) fill(x + 34, y - 4 + k, 4, 1, 0x1e1e22);
    disc(x + 40, y - 3, 1.2, 0xc8ccd2);
    const talk = (now + x * 0.3) % 6 < 1.4 && Math.floor(now * 7) % 2;
    dot(x + 40, y - 5, talk ? 0xffb040 : 0x5a3a14);
    fill(x + 43, y - 4, 2, 4, 0x2a2a30); fill(x + 43, y - 5, 2, 1, 0x4a4a52);
    for (let k = 0; k < 3; k++) dot(x + 42, y - 1 - k, 0x2a2a30);
  },
  /** A round-bottomed flask of bright green liquid on a ring stand over a little burner flame, bubbling hard, a curl of green vapour rising from its neck. */
  madlabflask(x, y) {
    const glass = 0xdff0f6, goo = 0x7ae04a;
    fill(x + 36, y - 1, 7, 1, 0x4a4e58); fill(x + 39, y - 3, 1, 2, 0x6e727c);
    dot(x + 39, y - 4, Math.floor(now * 8) % 2 ? 0x4a8af0 : 0x6aa8ff); dot(x + 39, y - 5, 0xffb040, 0.6);
    ring(x + 39, y - 7, 1.8, 0x6e727c, 0.6);
    disc(x + 39, y - 8, 2.6, goo); fill(x + 37, y - 10, 5, 1, goo, 0.6); fill(x + 38, y - 12, 3, 2, glass, 0.6); dot(x + 37, y - 9, WHITE, 0.7);
    for (let k = 0; k < 2; k++) dot(x + 38 + k * 2, y - 9 + (Math.floor(now * 6 + k) % 2), 0xc8ff9a);
    for (let k = 0; k < 3; k++) { const ph = (now * 0.6 + k / 3) % 1; dot(x + 39 + Math.round(Math.sin(ph * 6 + k)), y - 13 - Math.round(ph * 4), 0x9aff6a, 0.6 * (1 - ph)); }
    if (isNight) glowLate(() => glowAt(x + 39, y - 8, 4, 4, 0x7aff4a, 0.18));
  },
  /** A wooden rack of five test tubes in different colours; every 12 s one of them changes colour with a little puff, as if a reaction had just gone off. */
  madlabtubes(x, y) {
    fill(x + 33, y - 2, 12, 2, 0x8a5a32); fill(x + 33, y - 2, 12, 1, 0xa8703e); fill(x + 33, y - 6, 12, 1, 0x8a5a32);
    const cols = [0x7ae04a, 0xb88aff, 0xf2b840, 0x5ad8f0, 0xf28fb0], n = Math.floor((now + x * 0.3) / 12), t = (now + x * 0.3) % 12;
    for (let k = 0; k < 5; k++) {
      const c = cols[(k + n * (k === n % 5 ? 2 : 0)) % 5], tx = x + 34 + k * 2;
      fill(tx, y - 9, 1, 7, 0xdff0f6, 0.6); fill(tx, y - 5, 1, 3, c);
      if (t < 1 && k === n % 5) { dot(tx, y - 10, WHITE, 0.7); dot(tx + (Math.floor(t * 4) % 2 ? 1 : -1), y - 11, WHITE, 0.5); }
    }
  },
  /** A pair of brass goggles with round green-tinted lenses on a leather strap, lying on the desk; the lenses catch the light now and then. */
  madlabgoggles(x, y) {
    line(x + 33, y - 1, x + 45, y - 1, 0x5a3b25);
    for (const cx of [x + 37, x + 42]) { disc(cx, y - 3, 2.2, 0xc8a050); disc(cx, y - 3, 1.4, 0x4a8a6a); }
    fill(x + 39, y - 3, 1, 1, 0x8a6a2a);
    if ((now + x * 0.2) % 8 < 0.5) { dot(x + 36, y - 4, WHITE); dot(x + 41, y - 4, WHITE); }
  },
  /** A big old glass-jar battery with a zinc rod and two brass terminals on its lid; sparks jump between the terminals every few seconds. */
  madlabbattery(x, y) {
    fill(x + 35, y - 8, 9, 8, 0xdff0f6, 0.45); fill(x + 35, y - 5, 9, 5, 0x5a8a9a, 0.5); fill(x + 35, y - 8, 9, 1, 0x3a2a1e); fill(x + 35, y - 1, 9, 1, 0xa8adb6);
    fill(x + 38, y - 7, 3, 6, 0x8a8e98);
    fill(x + 36, y - 10, 2, 2, 0xc8a050); fill(x + 41, y - 10, 2, 2, 0xc8a050);
    if ((now + x * 0.29) % 4 < 0.3) { line(x + 37, y - 11, x + 39, y - 12, 0x8ae0ff); line(x + 39, y - 12, x + 42, y - 11, 0x8ae0ff); }
    dot(x + 36, y - 7, WHITE, 0.6);
  },
  /** A big filament bulb on a brass stand, its glowing coil flickering and now and then buzzing bright then nearly out; it glows warm, through the night too. */
  madlabbulb(x, y) {
    fill(x + 36, y - 1, 7, 1, 0x8a6a2a); fill(x + 38, y - 4, 3, 3, 0xc8a050);
    const f = (now + x * 0.3) % 9 < 0.5 ? (Math.floor(now * 12) % 2 ? 1.2 : 0.2) : candlelight(x * 0.2);
    disc(x + 39, y - 8, 2.6, 0xfff4d0, 0.25); ring(x + 39, y - 8, 2.8, 0xdff0f6, 0.7); dot(x + 37, y - 10, WHITE, 0.8);
    const coil = () => { for (let k = 0; k < 3; k++) dot(x + 38 + k, y - 8 + (k % 2), 0xffb040, Math.min(1, f)); };
    coil(); glowAt(x + 39, y - 8, 5, 4, 0xffc060, 0.08 * f);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 8, 6, 5, 0xffc060, 0.2 * f); coil(); });
  },
  /** A petri dish with a colony of something glowing growing in it, spreading out slowly and shrinking back, beside a glass dropper that lets a drop fall now and then. */
  madlabpetri(x, y) {
    fill(x + 34, y - 2, 9, 2, 0xdff0f6, 0.5); fill(x + 34, y - 3, 9, 1, 0xdff0f6, 0.7);
    const g = 1 + Math.round((Math.sin(now * 0.3 + x) + 1) * 1.5);
    fill(x + 38 - g, y - 2, g * 2 + 1, 1, 0x7ae04a); dot(x + 37, y - 3, 0xb88aff); dot(x + 40, y - 3, 0x7ae04a);
    line(x + 44, y - 4, x + 42, y - 11, 0xdff0f6); fill(x + 42, y - 12, 2, 2, 0xd8302a);
    const t = (now + x * 0.23) % 6; if (t < 0.8) dot(x + 41, y - 4 + Math.floor(t * 4), 0x7ae04a);
  },
  /** A mechanical hand of brass and steel on a little plinth, wired to a battery: every 10 s its fingers twitch, curl and open again. */
  madlabhand(x, y) {
    const brass = 0xc8a050, steel = 0xa8adb6;
    fill(x + 34, y - 2, 10, 2, 0x3a3a40); fill(x + 34, y - 2, 10, 1, 0x5a5a62);
    fill(x + 37, y - 6, 5, 4, brass); fill(x + 37, y - 6, 5, 1, 0xe8c878);
    const t = (now + x * 0.31) % 10, curl = t < 1.5 ? Math.floor(t * 4) % 2 : 0;
    for (let k = 0; k < 4; k++) fill(x + 37 + k + (k > 1 ? 0 : 0), y - 10 + curl * (k % 2 ? 2 : 1), 1, 4 - curl, steel);
    fill(x + 42, y - 7, 2, 1, steel);
    line(x + 44, y - 2, x + 46, y - 5, 0xb87333);
  },
  /** A big red horseshoe magnet with silver tips, a little pile of paper clips by it; every 14 s the clips leap up and cling to it, then drop. */
  madlabmagnet(x, y) {
    fill(x + 34, y - 9, 2, 8, 0xd8302a); fill(x + 40, y - 9, 2, 8, 0xd8302a); fill(x + 34, y - 10, 8, 2, 0xd8302a); fill(x + 35, y - 10, 6, 1, 0xe85a4a);
    fill(x + 34, y - 2, 2, 2, 0xc8ccd2); fill(x + 40, y - 2, 2, 2, 0xc8ccd2);
    const t = (now + x * 0.27) % 14, up = t < 2.5;
    for (const [cx, cy, ux, uy] of [[43, -1, 36, -1], [44, -2, 37, 0], [45, -1, 39, -1]]) dot(x + (up ? ux : cx), y + (up ? uy : cy), 0xa8adb6);
  },
  /** Three stacks of casino chips, red, blue and black with white edge spots, one leaning; every 15 s a chip is flipped off the top and dropped back. */
  casinochips(x, y) {
    [[35, 0xd8302a, 5], [39, 0x2a4ab8, 7], [43, 0x1e1e22, 4]].forEach(([cx, c, n]) => {
      for (let k = 0; k < n; k++) { fill(x + cx - 1, y - 1 - k, 3, 1, k % 2 ? shaded(c, 0.85) : c); dot(x + cx + (k % 2 ? -1 : 1), y - 1 - k, WHITE, 0.6); }
    });
    const t = (now + x * 0.23) % 15;
    if (t < 1.2) { const h = Math.round(Math.sin((t / 1.2) * Math.PI) * 4); fill(x + 38, y - 9 - h, 3, 1, 0x2a4ab8); dot(x + 39, y - 9 - h, WHITE, 0.7); }
  },
  /** A pair of red casino dice with white pips; every 12 s they're thrown, tumbling across the desk and settling on new faces. */
  casinodice(x, y) {
    const PIPS = [[[1, 1]], [[0, 0], [2, 2]], [[0, 0], [1, 1], [2, 2]], [[0, 0], [2, 0], [0, 2], [2, 2]], [[0, 0], [2, 0], [1, 1], [0, 2], [2, 2]], [[0, 0], [2, 0], [0, 1], [2, 1], [0, 2], [2, 2]]];
    const t = (now + x * 0.29) % 12, roll = t < 1.4, n = Math.floor((now + x * 0.29) / 12);
    [[35, 0], [41, 1]].forEach(([dx, k]) => {
      const face = roll ? Math.floor(now * 12 + k * 3) % 6 : hash(n, k) % 6, jump = roll ? Math.round(Math.abs(Math.sin(t * 6 + k)) * 3) : 0, slide = roll ? Math.round((1.4 - t) * 2) : 0;
      const left = x + dx - slide, top = y - 5 - jump;
      fill(left, top, 5, 5, 0xd8302a); fill(left, top, 5, 1, 0xe85a4a); fill(left + 4, top, 1, 5, 0xa81e18);
      for (const [px, py] of PIPS[face]) dot(left + 1 + px, top + 1 + py, WHITE);
    });
  },
  /** A deck of cards with a blue patterned back, the top few fanned out face up; every 16 s they're riffled together in a quick shuffle. */
  casinodeck(x, y) {
    fill(x + 34, y - 3, 6, 3, 0x2a4ab8); fill(x + 34, y - 3, 6, 1, 0x4a6ad8); for (let k = 0; k < 5; k += 2) dot(x + 35 + k, y - 2, 0xf6f6f2, 0.5);
    const t = (now + x * 0.27) % 16, sh = t < 1.5;
    for (let k = 0; k < 3; k++) {
      const cx = sh ? x + 38 + (Math.floor(now * 10 + k) % 3) : x + 39 + k * 2, cy = y - 7 + k;
      fill(cx, cy, 4, 5, 0xf6f6f2); fill(cx + 3, cy, 1, 5, 0xd8d8d4); dot(cx + 1, cy + 1, k === 1 ? 0x1e1e22 : 0xd8302a);
    }
  },
  /** A martini in a stemmed cone glass with a green olive on a pick; the olive bobs now and then. */
  casinomartini(x, y) {
    fill(x + 36, y - 1, 7, 1, 0xdff0f6, 0.7); fill(x + 39, y - 6, 1, 5, 0xdff0f6, 0.7);
    for (let r = 0; r < 4; r++) fill(x + 39 - (3 - r), y - 7 - r, (3 - r) * 2 + 1 + (r === 3 ? 0 : 0), 1, 0xdff0f6, 0.5);
    fill(x + 36, y - 10, 7, 1, 0xdff0f6, 0.8); fill(x + 37, y - 9, 5, 1, 0xf6f0c8, 0.6);
    const bob = (now + x * 0.2) % 8 < 0.6 ? 1 : 0;
    line(x + 38, y - 8 + bob, x + 42, y - 12 + bob, 0xd8302a); disc(x + 39, y - 9 + bob, 1, 0x6a9a3a); dot(x + 39, y - 9 + bob, 0xd8402a);
  },
  /** A little desk roulette wheel in polished wood with red and black pockets; every 18 s it spins, the white ball rattling round before it drops into a pocket. */
  casinoroulettemini(x, y) {
    const cx = x + 39, cy = y - 4;
    fill(cx - 6, cy + 2, 13, 2, 0x5a3b25); fill(cx - 6, cy + 2, 13, 1, 0x7a5232);
    const t = (now + x * 0.21) % 18, spin = t < 3 ? (3 - t) * 4 : 0, a0 = now * spin * 0.5 + x;
    for (let k = 0; k < 10; k++) { const a = a0 + (k / 10) * Math.PI * 2; dot(cx + Math.round(Math.cos(a) * 4), cy + Math.round(Math.sin(a) * 1.6), k === 0 ? 0x3ab86a : k % 2 ? 0xd8302a : 0x1e1e22); }
    ellipseRing(cx, cy, 5, 2, 0x8a5a32); dot(cx, cy, 0xd8b45a); dot(cx, cy - 1, 0xd8b45a);
    const ba = t < 3 ? -now * 9 : a0 + 0.3; dot(cx + Math.round(Math.cos(ba) * (t < 3 ? 5 : 4)), cy + Math.round(Math.sin(ba) * 2), WHITE);
  },
  /** A little desk slot machine in red and chrome with three reels; every 20 s its lever is pulled, the reels blur and stop one by one, and now and then they line up three sevens. */
  casinoslotmini(x, y) {
    fill(x + 34, y - 10, 9, 10, 0xc8302a); fill(x + 34, y - 10, 9, 1, 0xe85a4a); fill(x + 42, y - 10, 1, 10, 0x8a1e1a); fill(x + 34, y - 1, 9, 1, 0xa8adb6);
    fill(x + 35, y - 7, 7, 3, 0xf6f6f2);
    const t = (now + x * 0.31) % 20, n = Math.floor((now + x * 0.31) / 20), win = n % 4 === 0;
    for (let k = 0; k < 3; k++) {
      const spinning = t < 1 + k * 0.5, sym = spinning ? Math.floor(now * 14 + k) % 3 : win ? 0 : (hash(n, k) % 3);
      const c = [0xd8302a, 0xf2b840, 0x3ab86a][sym];
      if (spinning) fill(x + 35 + k * 2 + (k > 0 ? 0 : 0), y - 7, 1, 3, c, 0.6); else { dot(x + 35 + k * 2 + k % 1, y - 6, c); dot(x + 35 + k * 2, y - 7, c, sym === 0 ? 1 : 0.5); }
    }
    const pulled = t < 0.5;
    line(x + 44, y - 4, x + 44, pulled ? y - 4 : y - 10, 0xa8adb6); disc(x + 44, pulled ? y - 3 : y - 11, 1, 0xd8302a);
    if (win && t > 2 && t < 4) dot(x + 38, y - 11, 0xfff4c0);
  },
  /** A lucky golden horseshoe mounted points-up on a little black stand; a glint runs round it now and then. */
  casinohorseshoe(x, y) {
    fill(x + 36, y - 2, 7, 2, 0x1e1e22); fill(x + 36, y - 2, 7, 1, 0x3a3a40);
    const pts = [];
    for (let k = 0; k <= 8; k++) { const a = Math.PI * (k / 8); pts.push([x + 39 - Math.round(Math.cos(a) * 3), y - 5 + Math.round(Math.sin(a) * 3) - 4]); }
    for (const [px, py] of pts) { dot(px, py, 0xd8b45a); dot(px, py + 1, 0x9a7a30); }
    for (const [px, py] of [pts[1], pts[4], pts[7]]) dot(px, py, 0x6a5020);
    const t = (now + x * 0.2) % 9; if (t < 1) { const [gx, gy] = pts[Math.floor(t * 9) % 9]; dot(gx, gy, WHITE); }
  },
  /** A fat fold of green banknotes in a gold money clip, a couple of loose ones fanned beside it; now and then the top note lifts in a draught. */
  casinomoney(x, y) {
    const note = 0x8ab87a, noteDark = 0x5a8a4a;
    fill(x + 34, y - 2, 8, 2, noteDark); fill(x + 35, y - 3, 8, 1, note); fill(x + 41, y - 2, 4, 2, note); dot(x + 43, y - 1, noteDark);
    for (let k = 0; k < 3; k++) fill(x + 36, y - 6 + k, 7, 1, k % 2 ? noteDark : note);
    const lift = (now + x * 0.3) % 12 < 1 ? 1 : 0; fill(x + 36, y - 7 - lift, 7, 1, note); dot(x + 39, y - 7 - lift, 0xd8e8c8);
    fill(x + 38, y - 7, 2, 5, 0xd8b45a); dot(x + 38, y - 7, 0xf2d888);
  },
  /** A golden first-class ticket and a conductor's brass punch beside it; every 15 s the punch clicks and a new hole appears in the ticket. */
  orientticket(x, y) {
    fill(x + 33, y - 3, 8, 3, 0xf2d35b); fill(x + 33, y - 3, 8, 1, 0xf8e888); fill(x + 34, y - 2, 6, 1, 0xc8a030, 0.6);
    const n = Math.floor((now + x * 0.3) / 15) % 4; for (let k = 0; k < n; k++) dot(x + 39 - k * 2, y - 2, 0x8a6a2a);
    const t = (now + x * 0.3) % 15, click = t < 0.4 ? 1 : 0;
    fill(x + 42, y - 5 + click, 3, 2, 0xc8a050); line(x + 42, y - 3, x + 44, y - 1, 0xc8a050); line(x + 44, y - 4 + click, x + 45, y - 1, 0xc8a050);
  },
  /** A bone china teacup on its saucer, painted with a gold rim and a little blue flower, a silver spoon resting in it; steam curls up from the tea. */
  orientteacup(x, y) {
    fill(x + 34, y - 1, 10, 1, 0xf6f2e8); fill(x + 34, y - 1, 10, 1, 0xf6f2e8); dot(x + 34, y - 1, 0xd8b45a); dot(x + 43, y - 1, 0xd8b45a);
    fill(x + 36, y - 5, 6, 4, 0xf6f2e8); fill(x + 36, y - 5, 6, 1, 0xd8b45a); fill(x + 37, y - 4, 4, 1, 0xa86a3a); dot(x + 38, y - 3, 0x4a7ac8);
    dot(x + 42, y - 4, 0xf6f2e8); dot(x + 43, y - 3, 0xf6f2e8); line(x + 40, y - 4, x + 42, y - 7, 0xc8ccd2);
    for (let k = 0; k < 3; k++) { const ph = (now * 0.5 + k / 3) % 1; dot(x + 38 + Math.round(Math.sin(ph * 6 + k)), y - 6 - Math.round(ph * 4), WHITE, 0.45 * (1 - ph)); }
  },
  /** A gold pocket watch lying open on its chain, its white dial showing the real time and its second hand ticking round. */
  orientwatch(x, y) {
    const cx = x + 38, cy = y - 4;
    for (let k = 0; k < 5; k++) dot(x + 42 + k % 3, y - 1 - (k % 2), 0xd8b45a);
    disc(cx, cy, 3.4, 0xd8b45a); disc(cx, cy, 2.6, 0xf8f4ea); dot(cx, cy - 4, 0xd8b45a); fill(cx - 1, cy - 5, 3, 1, 0xe8c878);
    const d = new Date(), m = d.getMinutes() / 60 * Math.PI * 2 - Math.PI / 2, s = Math.floor(now) % 60 / 60 * Math.PI * 2 - Math.PI / 2;
    line(cx, cy, cx + Math.round(Math.cos(m) * 2), cy + Math.round(Math.sin(m) * 2), 0x1e1e22); dot(cx + Math.round(Math.cos(s) * 2), cy + Math.round(Math.sin(s) * 2), 0xc8302a);
  },
  /** A telegram in its buff envelope sealed with red wax, a strip of the message showing; now and then the flap lifts in the draught of the moving train. */
  orienttelegram(x, y) {
    fill(x + 34, y - 5, 10, 5, 0xe8d8b0); fill(x + 34, y - 5, 10, 1, 0xf2e8c8); fill(x + 34, y - 1, 10, 1, 0xc8b890);
    const lift = (now + x * 0.23) % 13 < 1 ? 1 : 0;
    line(x + 34, y - 5 - lift, x + 39, y - 2 - lift, 0xc8b890); line(x + 43, y - 5 - lift, x + 39, y - 2 - lift, 0xc8b890);
    disc(x + 39, y - 2 - lift, 1, 0xa81e18); dot(x + 39, y - 3 - lift, 0xd83a2a);
    fill(x + 35, y - 7, 7, 2, 0xf6f2e8); fill(x + 36, y - 7, 4, 1, 0x6a6a70, 0.7);
  },
  /** A guard's hand lamp of black tin with a brass handle, its lens turning between red and green every few seconds, a soft glow round it; it glows through the night. */
  orientlantern(x, y) {
    fill(x + 36, y - 9, 7, 9, 0x2a2a30); fill(x + 36, y - 9, 7, 1, 0x4a4a52); fill(x + 37, y - 11, 5, 2, 0x2a2a30); line(x + 37, y - 12, x + 41, y - 12, 0xc8a050);
    const green = Math.floor((now + x * 0.2) / 5) % 2, c = green ? 0x4aff6a : 0xff3a28;
    const lens = () => { disc(x + 39, y - 5, 2, c); dot(x + 38, y - 6, WHITE, 0.7); };
    lens(); glowAt(x + 39, y - 5, 4, 3, c, 0.12);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 5, 6, 4, c, 0.25); lens(); });
  },
  /** A little brass and black model steam locomotive on a length of track, its wheels' rods turning; every 18 s it lets a tiny puff of steam out of its chimney. */
  orientmodel(x, y) {
    fill(x + 33, y - 1, 13, 1, 0x5a4a3a); for (let k = 33; k < 46; k += 2) dot(x + k, y, 0x3a2a1e);
    fill(x + 35, y - 5, 7, 3, 0x2a2a30); fill(x + 35, y - 5, 7, 1, 0x4a4a52); fill(x + 41, y - 7, 3, 5, 0x1e2a4a); fill(x + 41, y - 7, 3, 1, 0xd8b45a);
    fill(x + 36, y - 7, 1, 2, 0x2a2a30); disc(x + 34, y - 4, 1, 0x2a2a30); dot(x + 34, y - 4, 0xd8b45a);
    for (const wx of [37, 40, 43]) dot(x + wx, y - 2, 0xc8302a);
    const a = now * 4; line(x + 37 + Math.round(Math.cos(a)), y - 2, x + 40 + Math.round(Math.cos(a)), y - 2, 0xc8ccd2);
    const t = (now + x * 0.27) % 18; if (t < 2) { const r = Math.floor(t * 2); fill(x + 35 + r, y - 9 - r, 2, 1, WHITE, 0.6 - r * 0.1); }
  },
  /** A worn leather passport lying open, its pages crowded with coloured stamps from the stations along the line; now and then a page turns. */
  orientpassport(x, y) {
    fill(x + 34, y - 3, 11, 3, 0x6a2e1e); fill(x + 35, y - 4, 4, 3, 0xf2ead8); fill(x + 40, y - 4, 4, 3, 0xf2ead8); fill(x + 39, y - 4, 1, 3, 0xc8b890);
    for (const [sx, sy, c] of [[35, -4, 0xc8302a], [37, -3, 0x2a5aa8], [41, -4, 0x2a8a4a], [42, -3, 0x8a4ab8]]) dot(x + sx, y + sy, c);
    const t = (now + x * 0.29) % 16; if (t < 0.8) { const k = Math.floor(t * 5); line(x + 39, y - 4, x + 39 + (k - 2), y - 6, 0xf2ead8); }
  },
  /** A folding camera with its bellows drawn out and a brass-rimmed lens; every 20 s the shutter clicks and the flash winks. */
  orientcamera(x, y) {
    fill(x + 34, y - 6, 5, 6, 0x2a2a30); fill(x + 34, y - 6, 5, 1, 0x4a4a52);
    for (let k = 0; k < 3; k++) fill(x + 39 + k, y - 5 + (k % 2 ? 0 : 0), 1, 4, k % 2 ? 0x3a3a40 : 0x1e1e22);
    disc(x + 43, y - 3, 1.8, 0xc8a050); dot(x + 43, y - 3, 0x1a2a3a); dot(x + 42, y - 4, WHITE, 0.6);
    const t = (now + x * 0.31) % 20; if (t < 0.25) { glowAt(x + 36, y - 8, 4, 3, WHITE, 0.5); dot(x + 36, y - 8, WHITE); } else fill(x + 35, y - 8, 3, 2, 0xa8adb6);
  },
  /** A skull with a fat candle on its crown, wax running down over its brow and dripping a new drop every few seconds; the flame flickers and the eye sockets glow faintly, through the night too. */
  halloweendxskull(x, y) {
    const bone = 0xe8dcc0, boneDark = 0xc8b898;
    fill(x + 36, y - 6, 7, 5, bone); fill(x + 37, y - 7, 5, 1, bone); fill(x + 37, y - 1, 5, 1, boneDark); fill(x + 42, y - 6, 1, 5, boneDark);
    const f = candlelight(x * 0.3);
    const sockets = (a) => { fill(x + 37, y - 5, 2, 2, 0x1e1418); fill(x + 40, y - 5, 2, 2, 0x1e1418); dot(x + 37, y - 4, 0xff8a3a, 0.5 * a); dot(x + 40, y - 4, 0xff8a3a, 0.5 * a); };
    sockets(f); dot(x + 39, y - 3, 0x1e1418); for (const k of [37, 39, 41]) dot(x + k, y - 1, 0x1e1418);
    fill(x + 38, y - 11, 3, 4, 0xf2ead8); dot(x + 37, y - 8, 0xf2ead8); dot(x + 41, y - 7, 0xf2ead8); dot(x + 37, y - 7, 0xf2ead8);
    const drip = (now + x * 0.2) % 5; if (drip < 1.5) dot(x + 37, y - 7 + Math.floor(drip * 2), 0xf2ead8);
    dot(x + 39, y - 12, 0xffd060, f); dot(x + 39, y - 13, 0xff8a3a, 0.5 * f);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 12, 5, 4, 0xffb050, 0.18 * f); dot(x + 39, y - 12, 0xfff0b0); sockets(f); });
  },
  /** A glossy caramel apple on a stick, rolled in chopped nuts, on a twist of paper; a drip of caramel slides down now and then. */
  halloweendxapple(x, y) {
    fill(x + 35, y - 1, 9, 1, 0xf2ead8);
    fill(x + 36, y - 6, 7, 5, 0xc87a2a); fill(x + 37, y - 7, 5, 1, 0xc87a2a); fill(x + 36, y - 6, 2, 3, 0xe8a050); dot(x + 37, y - 6, WHITE, 0.6);
    for (const k of [37, 39, 41]) dot(x + k, y - 2, 0xe8d0a0);
    fill(x + 39, y - 11, 1, 4, 0xd8b888);
    const d = (now + x * 0.23) % 9; if (d < 2) dot(x + 42, y - 4 + Math.floor(d), 0xa85a1a);
  },
  /** A bowl of wrapped sweets on the desk; every 20 s a green clawed hand shoots up out of it, grabs about and sinks back under the sweets. */
  halloweendxbowl(x, y) {
    fill(x + 34, y - 4, 11, 4, 0xe8782e); fill(x + 35, y - 1, 9, 1, 0xb8521e); fill(x + 34, y - 4, 11, 1, 0xf2a050);
    for (const [cx, c] of [[35, 0xd8302a], [37, 0x8a4ab8], [39, 0xf2d35b], [41, 0x3ab86a], [43, 0xf2f0ea]]) { fill(x + cx, y - 5, 2, 1, c); }
    const t = (now + x * 0.31) % 20;
    if (t < 2) {
      const up = Math.min(5, Math.floor(t * 8)) - (t > 1.4 ? Math.floor((t - 1.4) * 10) : 0), g = 0x7ab84a, gd = 0x4a7a3a;
      if (up > 0) { fill(x + 38, y - 5 - up, 3, up, g); fill(x + 38, y - 5 - up, 1, up, gd); const grab = Math.floor(now * 6) % 2; dot(x + 37 - grab, y - 6 - up, g); dot(x + 39, y - 7 - up, g); dot(x + 41 + grab, y - 6 - up, g); dot(x + 37 - grab, y - 7 - up, 0xe8dcc0); dot(x + 41 + grab, y - 7 - up, 0xe8dcc0); }
    }
  },
  /** A crystal ball on a clawed bronze stand, purple mist swirling inside it; every 15 s a face of light forms in the mist and fades. It glows, through the night too. */
  halloweendxcrystal(x, y) {
    fill(x + 36, y - 2, 7, 2, 0x8a6a2a); fill(x + 36, y - 2, 7, 1, 0xc8a050); dot(x + 35, y - 1, 0x8a6a2a); dot(x + 43, y - 1, 0x8a6a2a);
    const cx = x + 39, cy = y - 6;
    const ball = () => {
      disc(cx, cy, 3.6, 0x3a2a5a); disc(cx, cy, 3, 0x5a3a8a, 0.9);
      const a = now * 1.4; for (let k = 0; k < 3; k++) dot(cx + Math.round(Math.cos(a + k * 2) * 2), cy + Math.round(Math.sin(a + k * 2) * 1.5), 0xc8a8ff, 0.8);
      const t = (now + x * 0.2) % 15; if (t < 3) { const f = Math.min(1, t, 3 - t); dot(cx - 1, cy - 1, 0xe8f0ff, f); dot(cx + 1, cy - 1, 0xe8f0ff, f); dot(cx, cy + 1, 0xe8f0ff, f * 0.8); }
      dot(cx - 2, cy - 2, WHITE, 0.8);
    };
    ball();
    glowAt(cx, cy, 5, 5, 0xb88aff, 0.08);
    if (isNight) glowLate(() => { glowAt(cx, cy, 6, 6, 0xb88aff, 0.16); ball(); });
  },
  /** Three tarot cards laid out face down on a purple cloth; every 16 s they turn over one by one, a sun, a moon and a skull, then go back face down. */
  halloweendxtarot(x, y) {
    fill(x + 33, y - 2, 13, 2, 0x3a1e4a); fill(x + 33, y - 2, 13, 1, 0x5a3a6a);
    const t = (now + x * 0.27) % 16;
    for (let k = 0; k < 3; k++) {
      const cx = x + 34 + k * 4, cy = y - 7, up = t > 1 + k * 0.6 && t < 8;
      fill(cx, cy, 3, 5, up ? 0xf2ead8 : 0x2a1e4a); fill(cx, cy, 3, 1, up ? WHITE : 0x6a4a8a);
      if (!up) dot(cx + 1, cy + 2, 0xd8b45a);
      else if (k === 0) { dot(cx + 1, cy + 2, 0xf2b840); dot(cx, cy + 2, 0xf2d35b); dot(cx + 2, cy + 2, 0xf2d35b); }
      else if (k === 1) { dot(cx + 1, cy + 1, 0x8ab8e8); dot(cx + 1, cy + 3, 0x8ab8e8); dot(cx, cy + 2, 0x8ab8e8); }
      else { dot(cx + 1, cy + 2, 0x1e1a1a); dot(cx, cy + 2, 0x1e1a1a); dot(cx + 2, cy + 2, 0x1e1a1a); dot(cx + 1, cy + 3, 0x8a7a6a); }
    }
  },
  /** Three warty gourds, a green one, a white one and a striped orange one; now and then the orange one's carved face blinks. */
  halloweendxgourds(x, y) {
    fill(x + 33, y - 4, 4, 4, 0x5a8a3a); dot(x + 34, y - 5, 0x5a8a3a); for (const [wx, wy] of [[34, -3], [36, -2]]) dot(x + wx, y + wy, 0x7aaa4a); dot(x + 34, y - 6, 0x6a4a2a);
    fill(x + 37, y - 5, 4, 5, 0xeee8d8); fill(x + 38, y - 7, 2, 2, 0xeee8d8); dot(x + 39, y - 8, 0x6a4a2a); dot(x + 38, y - 3, 0xd8d0c0);
    fill(x + 41, y - 5, 5, 5, 0xe8782e); for (const k of [42, 44]) fill(x + k, y - 5, 1, 5, 0xb8521e, 0.6); dot(x + 43, y - 6, 0x4a7a3a);
    const blink = (now + x * 0.19) % 7 < 0.3;
    if (!blink) { dot(x + 42, y - 4, 0xffd060); dot(x + 44, y - 4, 0xffd060); } fill(x + 42, y - 2, 3, 1, 0xffd060);
    if (isNight) glowLate(() => { glowAt(x + 43, y - 3, 3, 3, 0xffb050, 0.16); if (!blink) { dot(x + 42, y - 4, 0xffe080); dot(x + 44, y - 4, 0xffe080); } });
  },
  /** A little black lacquer music box with its lid open, a tiny skeleton dancer turning on its spindle, jerking a bony arm up now and then. */
  halloweendxmusicbox(x, y) {
    fill(x + 34, y - 4, 11, 4, 0x1e1820); fill(x + 34, y - 4, 11, 1, 0x3a2e3a); fill(x + 34, y - 1, 11, 1, 0xc8a050);
    fill(x + 34, y - 11, 11, 1, 0x1e1820); line(x + 34, y - 11, x + 34, y - 4, 0x1e1820); fill(x + 35, y - 10, 9, 6, 0x6a2a4a, 0.5);
    const turn = Math.floor(now * 3) % 2, arm = (now + x * 0.2) % 4 < 0.5;
    fill(x + 39, y - 9, 1, 3, 0xf2eee0); dot(x + 39, y - 10, 0xf2eee0); dot(x + 38, y - 10, 0xf2eee0);
    dot(x + 38 + turn * 2, y - 8, 0xf2eee0); dot(x + 40 - turn * 2, arm ? y - 10 : y - 8, 0xf2eee0);
    dot(x + 38, y - 5, 0xf2eee0); dot(x + 40, y - 5, 0xf2eee0); fill(x + 39, y - 5, 1, 1, 0xc8a050);
    for (let k = 0; k < 2; k++) { const ph = (now * 0.5 + k / 2) % 1; dot(x + 44 + Math.round(ph * 2), y - 8 - Math.round(ph * 5), 0xd8b45a, 0.7 * (1 - ph)); }
  },
  /** A tall apothecary jar of greenish liquid with eyeballs floating in it, cork-stoppered; now and then the eyes all turn to look one way, then the other, and they glow faintly at night. */
  halloweendxjar(x, y) {
    fill(x + 36, y - 11, 7, 11, 0xa8d8a0, 0.45); fill(x + 36, y - 11, 1, 11, 0xd8f0d0, 0.6); fill(x + 36, y - 12, 7, 1, 0xc8a070); fill(x + 37, y - 13, 5, 1, 0xa8805a); fill(x + 36, y - 1, 7, 1, 0x6a8a6a);
    const look = Math.floor((now + x * 0.3) / 4) % 3 - 1;
    const eyes = () => [[38, -9], [40, -6], [38, -4], [41, -10]].forEach(([ex, ey]) => { fill(x + ex, y + ey, 2, 2, 0xf2f0ea); dot(x + ex + (look > 0 ? 1 : 0), y + ey + (look === 0 ? 1 : 0), look ? 0x3a7ac8 : 0x8a3a2a); });
    eyes(); dot(x + 37, y - 10, WHITE, 0.6);
    if (isNight) glowLate(() => glowAt(x + 39, y - 6, 4, 5, 0x9aff8a, 0.1));
  },
  /** A wide cup of latte on its saucer with a cat's face drawn in the foam, ears and whiskers, and a curl of steam. */
  catcafelatte(x, y) {
    fill(x + 34, y - 1, 11, 1, 0xf6f2ea); fill(x + 35, y - 5, 8, 4, 0xf6f2ea); fill(x + 43, y - 4, 2, 2, 0xf6f2ea); dot(x + 44, y - 3, 0xd8d4cc);
    fill(x + 36, y - 6, 6, 1, 0xc8945a); fill(x + 36, y - 5, 6, 1, 0xa87040);
    fill(x + 38, y - 6, 2, 1, 0xf2e8d8); dot(x + 37, y - 7, 0xf2e8d8); dot(x + 40, y - 7, 0xf2e8d8);
    for (let k = 0; k < 3; k++) { const ph = (now * 0.5 + k / 3) % 1; dot(x + 39 + Math.round(Math.sin(ph * 6 + k)), y - 8 - Math.round(ph * 4), WHITE, 0.45 * (1 - ph)); }
  },
  /** A little plate of iced cookies shaped like cat faces, a ginger one, a grey one and a calico one, sprinkles for whiskers; one goes missing every 18 s and comes back. */
  catcafecookie(x, y) {
    fill(x + 33, y - 1, 12, 1, 0xf6f2ea); fill(x + 34, y - 2, 10, 1, 0xf2ece0);
    const gone = Math.floor((now + x * 0.3) / 18) % 3, missing = (now + x * 0.3) % 18 < 6;
    [[35, 0xe8903a], [39, 0x8a96a8], [42, 0xf6f2ea]].forEach(([cx, c], k) => {
      if (missing && k === gone) { dot(x + cx + 1, y - 2, 0xd8b888); return; }
      fill(x + cx, y - 4, 3, 2, c); dot(x + cx, y - 5, c); dot(x + cx + 2, y - 5, c); dot(x + cx, y - 4, 0x2a2a30); dot(x + cx + 2, y - 4, 0x2a2a30);
      if (k === 2) dot(x + cx + 1, y - 4, 0xe8903a);
    });
  },
  /** A cardboard box on the desk, its flaps open, two ears showing over its edge; every 10 s a cat's head pops up to look about, then ducks back down. */
  catcafebox(x, y) {
    const card = 0xc8a070, cardDark = 0xa8804a, cardLight = 0xe0bc88;
    const kind = ['catcafeblue', 'catcafeginger', 'catcafebengal'][Math.floor(x / 60) % 3], c = CAFE_CATS[kind];
    const t = (now + x * 0.29) % 10, up = t < 3 ? (t < 0.4 ? Math.floor(t * 10) : t > 2.6 ? Math.floor((3 - t) * 10) : 4) : 0;
    fill(x + 37, y - 8 - up, 5, 3 + up, c.fur); dot(x + 37, y - 9 - up, c.fur); dot(x + 41, y - 9 - up, c.fur);
    if (up > 2) { dot(x + 38, y - 7 - up + 2, c.eye); dot(x + 40, y - 7 - up + 2, c.eye); dot(x + 39, y - 6 - up + 2, c.nose); }
    fill(x + 34, y - 7, 11, 7, card); fill(x + 34, y - 7, 11, 1, cardLight); fill(x + 44, y - 7, 1, 7, cardDark); fill(x + 38, y - 5, 3, 1, 0xd8c098);
    line(x + 34, y - 7, x + 32, y - 10, cardLight); line(x + 44, y - 7, x + 46, y - 10, cardDark);
  },
  /** A mug made like a cat: its ears on the rim, a sleepy face on the front, and its curled tail for the handle; steam rises from it. */
  catcafemug(x, y) {
    const c = 0xf6f2ea;
    fill(x + 36, y - 7, 6, 7, c); fill(x + 36, y - 7, 1, 7, WHITE); fill(x + 41, y - 7, 1, 7, 0xd8d4cc); dot(x + 36, y - 8, c); dot(x + 41, y - 8, c);
    dot(x + 37, y - 4, 0x2a2a30); dot(x + 40, y - 4, 0x2a2a30); dot(x + 38, y - 3, 0xf28fb0); dot(x + 39, y - 3, 0xf28fb0, 0.5);
    dot(x + 42, y - 5, c); dot(x + 43, y - 4, c); dot(x + 43, y - 3, c); dot(x + 42, y - 2, c);
    fill(x + 37, y - 7, 4, 1, 0x8a5a32);
    for (let k = 0; k < 2; k++) { const ph = (now * 0.5 + k / 2) % 1; dot(x + 38 + k * 2 + Math.round(Math.sin(ph * 6)), y - 9 - Math.round(ph * 4), WHITE, 0.45 * (1 - ph)); }
  },
  /** A glass jar of fish-shaped treats with a paw-print lid; every 18 s a cat's paw reaches up from the side of the desk to fish for one. */
  catcafetreats(x, y) {
    fill(x + 36, y - 9, 7, 9, 0xdff0f6, 0.45); fill(x + 36, y - 9, 1, 9, WHITE, 0.5); fill(x + 35, y - 10, 9, 2, 0xf2a0b8); dot(x + 39, y - 10, WHITE);
    for (const [fx, fy, c] of [[37, -2, 0xe8903a], [40, -3, 0xd8a050], [38, -5, 0xe8903a], [41, -6, 0xc87a3a], [37, -7, 0xd8a050]]) { fill(x + fx, y + fy, 2, 1, c); dot(x + fx + 2, y + fy, shaded(c, 0.8)); }
    const t = (now + x * 0.23) % 18;
    if (t < 2.5) { const c = CAFE_CATS.catcafetabby, up = Math.min(5, Math.floor(t * 5)) - (t > 2 ? 3 : 0); fill(x + 44, y - up, 2, up + 1, c.fur); dot(x + 44, y - up, 0xf6f2ea); dot(x + 43, y - up + (Math.floor(now * 8) % 2), 0xf6f2ea); }
  },
  /** A cat fast asleep curled up at the end of the desk, a different one at each desk, its tail twitching and a little z rising now and then. */
  catcafenapper(x, y) {
    const kinds = ['catcafeginger', 'catcafecalico', 'catcafeblue', 'catcafetabby', 'catcafepersian', 'catcafetortie', 'catcafemaine', 'catcafebengal'];
    const kind = kinds[Math.floor((x * 7 + y * 3) / 22) % kinds.length];
    fill(x + 34, y - 1, 11, 1, SHADOW);
    cafeCatAt(x + 40, y, kind, 'sleep', (x + y) % 2 === 0);
  },
  /** A toy mouse of grey felt with a long tail on a wire spring, wobbling; every 9 s a paw swats it from off the desk's edge and it springs back boinging. */
  catcafetoy(x, y) {
    fill(x + 37, y - 1, 5, 1, 0x8a5a32);
    const t = (now + x * 0.3) % 9, swat = t < 1.2, wob = swat ? Math.round(Math.sin(t * 20) * (1.2 - t) * 4) : Math.round(Math.sin(now * 3 + x) * 0.6);
    for (let k = 0; k < 5; k++) dot(x + 39 + Math.round(wob * k / 5), y - 2 - k, 0xa8adb6);
    const mx = x + 39 + wob, my = y - 8;
    fill(mx - 1, my, 4, 2, 0x8a8a90); dot(mx + 3, my + 1, 0xf2a0b0); dot(mx, my - 1, 0xf2a0b0); dot(mx + 2, my, PALETTE.eye); line(mx - 1, my + 1, mx - 4, my + 3, 0xf2a0b0);
    if (swat && t < 0.4) { const c = CAFE_CATS.catcafekitten; fill(x + 44, y - 7, 2, 2, c.fur); dot(x + 44, y - 6, c.light); }
  },
  /** A white teapot made like a sitting cat, a ginger patch on its head, its paw for the spout and its tail for the handle; steam puffs from its ears now and then. */
  catcafeteapot(x, y) {
    const c = 0xf6f2ea;
    fill(x + 36, y - 6, 7, 6, c); fill(x + 37, y - 7, 5, 1, c); fill(x + 36, y - 6, 1, 6, WHITE); fill(x + 42, y - 6, 1, 6, 0xd8d4cc);
    fill(x + 37, y - 10, 5, 3, c); dot(x + 37, y - 11, c); dot(x + 41, y - 11, c); fill(x + 38, y - 10, 2, 1, 0xe8903a);
    dot(x + 38, y - 9, 0x2a2a30); dot(x + 40, y - 9, 0x2a2a30); dot(x + 39, y - 8, 0xf28fb0);
    dot(x + 43, y - 4, c); dot(x + 44, y - 5, c); dot(x + 35, y - 4, c); dot(x + 34, y - 3, c); dot(x + 34, y - 5, c);
    const t = (now + x * 0.27) % 9; if (t < 2) for (const ex of [37, 41]) dot(x + ex, y - 12 - Math.floor(t * 2), WHITE, 0.5 - t * 0.2);
  },
  /** Three spray cans, red, cyan and yellow, with white caps; every so often one gives a short hiss and a puff of its colour drifts up and fades. */
  spiderversespray(x, y) {
    const cans = [[35, 0xd8202a, 7], [39, 0x30c8d8, 6], [43, 0xf2d35b, 7]];
    for (const [dx, c, h] of cans) {
      fill(x + dx, y - h, 3, h, c); fill(x + dx, y - h, 1, h, shaded(c, 1.2)); fill(x + dx + 2, y - h, 1, h, shaded(c, 0.8));
      fill(x + dx, y - h - 2, 3, 2, 0xf4f2ee); dot(x + dx + 1, y - h - 3, 0x8a8e96); fill(x + dx, y - 3, 3, 1, 0x1c1a22, 0.5);
    }
    fill(x + 34, y, 12, 1, SHADOW);
    const t = (now + x * 0.23) % 14;
    if (t < 1.6) { const k = Math.floor((now + x) / 14) % 3, [dx, c, h] = cans[k], r = Math.floor(t * 3); for (let j = 0; j <= r; j++) dot(x + dx + 1 + j, y - h - 4 - j, c, 0.6 - j * 0.12); }
  },
  /** Miles's big over-ear headphones in black and red resting on a little stand; when the music's on, small notes pop out now and then. */
  spiderversephones(x, y) {
    const black = 0x1c1a22, red = 0xd8202a;
    fill(x + 38, y - 1, 5, 1, 0x4a4e58); fill(x + 40, y - 9, 1, 8, 0x6e727c);
    ring(x + 40, y - 7, 4, black); fill(x + 36, y - 8, 1, 2, black);
    fill(x + 35, y - 7, 3, 5, black); fill(x + 43, y - 7, 3, 5, black); dot(x + 36, y - 5, red); dot(x + 44, y - 5, red);
    fill(x + 35, y - 7, 1, 5, 0x2e2a36); fill(x + 40, y - 11, 1, 1, red);
    const playing = frame.music?.playing === true;
    const t = (now + x * 0.19) % (playing ? 4 : 18);
    if (t < 1.5) { const r = Math.floor(t * 3); dot(x + 46, y - 8 - r, 0xf28fb0, 0.8 - r * 0.2); dot(x + 46, y - 9 - r, 0xf28fb0, 0.8 - r * 0.2); dot(x + 47, y - 10 - r, 0xf28fb0, 0.6 - r * 0.15); }
  },
  /** A web-shooter on its wristband: a steel cuff with a red cartridge and a little trigger pad; every so often it fires a web line across the desk that sticks in a white splat, then dissolves. */
  spiderverseshooter(x, y) {
    const steel = 0x6e727c, steelLight = 0xa8adb6, steelDark = 0x4a4e58, red = 0xd8202a;
    fill(x + 34, y - 4, 7, 4, steel); fill(x + 34, y - 4, 7, 1, steelLight); fill(x + 34, y - 1, 7, 1, steelDark);
    fill(x + 36, y - 6, 3, 2, red); dot(x + 36, y - 6, 0xf05050); dot(x + 41, y - 3, 0x2a2a30); fill(x + 33, y - 3, 1, 2, 0x1c1a22);
    fill(x + 34, y, 8, 1, SHADOW);
    const t = (now + x * 0.17) % 16;
    if (t < 2) {
      const reach = Math.min(5, Math.floor(t * 12));
      line(x + 41, y - 3, x + 41 + reach, y - 6, WHITE, 0.9);
      if (reach >= 5) { dot(x + 46, y - 6, WHITE); dot(x + 45, y - 7, WHITE, 0.7); dot(x + 47, y - 5, WHITE, 0.7); dot(x + 46, y - 8, WHITE, 0.5); }
    }
  },
  /** A stack of comics with bright spines, the top one showing a masked hero's face on its cover; now and then its cover flips open and shut. */
  spiderversecomics(x, y) {
    const spines = [0x2a4ab0, 0xf2d35b, 0xd8202a, 0x5ac8c8];
    spines.forEach((c, i) => { fill(x + 34 + (i % 2), y - 1 - i, 11, 1, c); dot(x + 34 + (i % 2), y - 1 - i, shaded(c, 1.2)); fill(x + 44 + (i % 2), y - 1 - i, 1, 1, 0xeae6dc); });
    const t = (now + x * 0.21) % 20, open = t < 1.2;
    fill(x + 34, y - 6, 11, 2, 0xf2d35b); fill(x + 38, y - 6, 3, 2, 0xd8202a); dot(x + 38, y - 6, WHITE); dot(x + 40, y - 6, WHITE);
    if (open) { fill(x + 34, y - 8, 1, 3, 0xeae6dc); fill(x + 33, y - 9, 1, 3, 0xf2d35b); }
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A red, white and black high-top sneaker with a swooping side stripe and a loose lace; now and then the lace flicks. */
  spiderversesneaker(x, y) {
    const red = 0xd8202a, white = 0xf4f2ee, black = 0x1c1a22;
    fill(x + 34, y - 2, 11, 2, white); fill(x + 34, y - 1, 11, 1, 0xd8d4cc);
    fill(x + 35, y - 7, 5, 5, red); fill(x + 39, y - 5, 5, 3, white); fill(x + 43, y - 4, 2, 2, red);
    fill(x + 35, y - 8, 4, 1, black); line(x + 36, y - 3, x + 43, y - 4, black); dot(x + 35, y - 7, shaded(red, 1.2));
    const t = (now + x * 0.29) % 12, flick = t < 0.8 ? Math.floor(t * 6) % 2 : 0;
    dot(x + 40, y - 6, white); dot(x + 41, y - 5 - flick, white); dot(x + 42, y - 4 - flick, white);
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A Spider Society wristwatch: a black band and a square face lit amber; every so often it projects a small holographic triangle of spider-marks above it that flickers and folds away. It glows at night. */
  spiderversewatch(x, y) {
    const band = 0x1c1a22, face = 0xff9a3a;
    fill(x + 34, y - 2, 11, 2, band); fill(x + 34, y - 2, 11, 1, 0x2e2a36);
    fill(x + 37, y - 5, 5, 4, 0x2e2a36); fill(x + 38, y - 4, 3, 2, face); dot(x + 39, y - 4, 0xffd090);
    fill(x + 34, y, 12, 1, SHADOW);
    const t = (now + x * 0.37) % 12;
    const holo = (a) => { for (let k = 0; k < 4; k++) fill(x + 39 - k, y - 7 - k, 1 + k * 2, 1, face, a * (0.7 - k * 0.1)); dot(x + 39, y - 9, 0xffe0b0, a); };
    if (t < 2.2 && Math.floor(now * 12) % 5) holo(1);
    glowAt(x + 39, y - 4, 4, 3, face, 0.06);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 4, 5, 3, face, 0.16); fill(x + 38, y - 4, 3, 2, face); if (t < 2.2) holo(0.8); });
  },
  /** A little model of the collider: a steel ring on a stand with a spark racing round it, now and then a flash of multiverse colours in its middle. It glows at night. */
  spiderversecollider(x, y) {
    const steel = 0x6e727c, steelLight = 0xa8adb6, steelDark = 0x4a4e58, cx = x + 39, cy = y - 7;
    fill(x + 35, y - 2, 9, 2, steelDark); fill(x + 35, y - 2, 9, 1, steel); fill(x + 39, y - 3, 1, 1, steel);
    ring(cx, cy, 4, steel); dot(cx - 3, cy - 3, steelLight);
    const k = Math.floor(now * 8 + x) % 12, a = (k / 12) * Math.PI * 2;
    dot(cx + Math.round(Math.cos(a) * 4), cy + Math.round(Math.sin(a) * 4), 0x30e0f0);
    const t = (now + x * 0.13) % 11;
    if (t < 0.8) { const c = [0xff3aa0, 0x30e0f0, 0xf2d35b][Math.floor(now * 8) % 3]; disc(cx, cy, 2, c, 0.85); }
    else dot(cx, cy, 0x6a3ab0, 0.7);
    fill(x + 34, y, 11, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(cx, cy, 6, 5, 0x60c0ff, 0.14); dot(cx + Math.round(Math.cos(a) * 4), cy + Math.round(Math.sin(a) * 4), 0xc0f8ff); });
  },
  /** Peter B.'s pizza: an open box with a pepperoni pizza, one slice gone and its crust left in the lid; a wisp of steam rises now and then. */
  spiderversepizza(x, y) {
    const card = 0xd8b884, cardDark = 0xb89464, crust = 0xd89040, cheese = 0xf2c84a, pep = 0xb83020;
    fill(x + 33, y - 9, 12, 5, card); fill(x + 33, y - 9, 12, 1, shaded(card, 1.1)); dot(x + 35, y - 7, 0xd8202a); dot(x + 36, y - 7, 0xd8202a);
    fill(x + 40, y - 7, 3, 1, crust);
    fill(x + 33, y - 4, 12, 4, cardDark); fill(x + 34, y - 4, 10, 3, crust); fill(x + 35, y - 4, 8, 2, cheese);
    fill(x + 39, y - 4, 4, 2, cardDark);
    for (const [px, py] of [[35, -4], [37, -3], [36, -2]]) dot(x + px, y + py, pep);
    fill(x + 33, y, 12, 1, SHADOW);
    const t = (now + x * 0.23) % 13;
    if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 37 + (r % 2), y - 6 - r, WHITE, 0.45 - r * 0.1); }
  },
  /** A steel fist-guard on a little stand with three claws; every 18 s or so they shoot out with a snikt, gleam, and slide back in. */
  xmenclaws(x, y) {
    const steel = 0xa8adb6, steelLight = 0xe0e4ea, guard = 0x6e727c;
    fill(x + 36, y - 2, 8, 2, 0x3a3a44); fill(x + 36, y - 2, 8, 1, 0x4a4a56);
    fill(x + 37, y - 6, 6, 4, guard); fill(x + 37, y - 6, 6, 1, 0x8a8e98); fill(x + 42, y - 6, 1, 4, 0x4a4e58);
    const t = (now + x * 0.21) % 18, out = t < 2.5 ? Math.min(5, Math.floor(t * 20)) : 1;
    for (const cx of [38, 40, 42]) { fill(x + cx, y - 6 - out, 1, out, steel); dot(x + cx, y - 6 - out, steelLight); }
    if (t > 0.3 && t < 0.6) dot(x + 40, y - 11, WHITE, 0.9);
    fill(x + 35, y, 10, 1, SHADOW);
  },
  /** A ruby-quartz visor resting on a little grey head form; every 14 s or so it fires a short red beam across the desk that fades. The lens glows red at night. */
  xmenvisor(x, y) {
    const grey = 0x8a8e98, red = 0xd8202a, ruby = 0xff3a3a;
    fill(x + 38, y - 2, 6, 2, 0x4a4e58); fill(x + 40, y - 4, 2, 2, grey);
    fill(x + 37, y - 10, 8, 6, grey); fill(x + 38, y - 11, 6, 1, grey); fill(x + 37, y - 10, 1, 6, 0xa8adb6);
    fill(x + 36, y - 8, 10, 2, 0x2a3a8a); fill(x + 37, y - 8, 8, 1, ruby); dot(x + 38, y - 8, 0xff9a9a);
    const t = (now + x * 0.17) % 14;
    if (t < 0.9) { const len = Math.floor(t * 12); line(x + 45, y - 8, x + 45 + Math.min(len, 6), y - 8, ruby, 0.85); glowAt(x + 46, y - 8, 4, 2, ruby, 0.2); }
    fill(x + 37, y, 8, 1, SHADOW);
    if (isNight) glowLate(() => { fill(x + 37, y - 8, 8, 1, ruby); glowAt(x + 41, y - 8, 5, 2, 0xff2020, 0.14); });
  },
  /** A fanned hand of playing cards, a red king in front; every 12 s one of them charges up in glowing magenta, crackles and goes quiet. It shines through the night when charged. */
  xmencards(x, y) {
    const card = 0xf4f2ee, edge = 0xc8c4bc;
    const cards = [[35, -6], [38, -7], [41, -6]];
    const t = (now + x * 0.27) % 12, hot = t < 2;
    cards.forEach(([cx, cy], i) => {
      fill(x + cx, y + cy, 4, 6, card); fill(x + cx + 3, y + cy, 1, 6, edge);
      dot(x + cx + 1, y + cy + 1, i === 1 ? 0xd8202a : 0x1c1a22); dot(x + cx + 2, y + cy + 3, i === 1 ? 0xd8202a : 0x1c1a22);
    });
    if (hot) {
      const f = Math.floor(now * 10) % 2;
      fill(x + 41, y - 6, 4, 6, 0xff3aa0, 0.55); dot(x + 46, y - 7 + f, 0xff8ad0); dot(x + 44, y - 8 - f, 0xff8ad0);
      glowAt(x + 43, y - 3, 5, 4, 0xff3aa0, 0.18);
    }
    fill(x + 34, y, 12, 1, SHADOW);
    if (isNight && hot) glowLate(() => { glowAt(x + 43, y - 3, 6, 5, 0xff3aa0, 0.25); fill(x + 41, y - 6, 4, 6, 0xff8ad0, 0.5); });
  },
  /** A chess game between two old friends: a small board seen at an angle with a white king and a black king facing off among a few pieces; every 25 s a piece slides a square. */
  xmenchess(x, y) {
    const light = 0xe8e2d4, dark = 0x5a3b25;
    fill(x + 33, y - 4, 13, 4, 0x3a2418); fill(x + 33, y - 4, 13, 1, 0x6a4a30);
    for (let r = 0; r < 3; r++) for (let c = 0; c < 6; c++) fill(x + 34 + c * 2, y - 3 + r, 2, 1, (r + c) % 2 ? dark : light);
    const t = (now + x * 0.33) % 25, slid = t < 1.5 ? 0 : 2;
    fill(x + 35, y - 7, 2, 3, WHITE); dot(x + 35, y - 8, WHITE); fill(x + 34, y - 4, 4, 1, 0xd8d4cc);
    fill(x + 43, y - 7, 2, 3, 0x2a2a30); dot(x + 44, y - 8, 0x2a2a30); fill(x + 42, y - 4, 4, 1, 0x1c1a22);
    dot(x + 38 + slid, y - 5, WHITE); dot(x + 38 + slid, y - 4, WHITE); dot(x + 40, y - 5, 0x2a2a30); dot(x + 40, y - 4, 0x2a2a30);
    fill(x + 33, y, 13, 1, SHADOW);
  },
  /** A little red helmet with purple cheek-guards on a stand, the master of magnetism's; now and then the paper clips beside it lift into the air, hang and drop. */
  xmenmagneto(x, y) {
    const red = 0xb8202a, redLight = 0xd84040, purple = 0x6a3a8a;
    fill(x + 36, y - 2, 7, 2, 0x4a4e58); fill(x + 39, y - 3, 1, 1, 0x4a4e58);
    fill(x + 36, y - 9, 7, 5, red); fill(x + 37, y - 10, 5, 1, red); fill(x + 36, y - 9, 1, 4, redLight); dot(x + 39, y - 11, red);
    fill(x + 36, y - 6, 1, 3, purple); fill(x + 42, y - 6, 1, 3, purple); fill(x + 38, y - 7, 3, 1, 0x1c1a22);
    const t = (now + x * 0.29) % 15, lift = t < 2.5 ? Math.min(3, Math.floor(t * 4)) - (t > 2 ? 3 : 0) : 0;
    for (const [cx, dl] of [[44, 0], [45, 1], [33, 1]]) { fill(x + cx, y - 1 - Math.max(0, lift - dl), 1, 1, 0xa8adb6); dot(x + cx, y - 2 - Math.max(0, lift - dl), 0xa8adb6, 0.6); }
    fill(x + 35, y, 9, 1, SHADOW);
  },
  /** A pair of steel dog tags on a bead chain lying in a loop; a glint runs over them now and then. */
  xmendogtags(x, y) {
    const steel = 0xa8adb6, steelDark = 0x6e727c;
    for (let k = 0; k < 10; k++) dot(x + 35 + k, y - 1 - Math.round(Math.sin((k / 9) * Math.PI) * 3), 0x8a8e98);
    fill(x + 38, y - 4, 4, 3, steel); fill(x + 38, y - 4, 4, 1, 0xd0d4dc); fill(x + 41, y - 4, 1, 3, steelDark);
    fill(x + 40, y - 3, 4, 3, steel); fill(x + 43, y - 3, 1, 3, steelDark); dot(x + 41, y - 2, steelDark); dot(x + 42, y - 2, steelDark);
    const t = (now + x * 0.19) % 9;
    if (t < 0.5) dot(x + 39 + Math.floor(t * 8), y - 4, WHITE, 0.9);
    fill(x + 35, y, 10, 1, SHADOW);
  },
  /** A storm in a jar: a tiny grey cloud in a glass jar raining onto its floor; every 10 s or so a little yellow bolt flashes down and lights the glass, through the night too. */
  xmenstorm(x, y) {
    fill(x + 36, y - 12, 8, 12, 0xdff0f6, 0.35); fill(x + 36, y - 12, 1, 12, 0xdff0f6, 0.6); fill(x + 43, y - 12, 1, 12, 0xdff0f6, 0.6);
    fill(x + 36, y - 13, 8, 1, 0x8a8e98); dot(x + 37, y - 10, WHITE, 0.7);
    fill(x + 37, y - 10, 6, 2, 0x8a8e98); fill(x + 38, y - 11, 3, 1, 0xa8adb6); fill(x + 37, y - 8, 6, 1, 0x6e727c);
    for (let k = 0; k < 3; k++) dot(x + 38 + k * 2, y - 6 + Math.floor(now * 4 + k) % 5, 0x6ab0e0, 0.7);
    fill(x + 36, y - 1, 8, 1, 0x6ab0e0, 0.5);
    const t = (now + x * 0.23) % 10, bolt = t < 0.4;
    if (bolt) { dot(x + 40, y - 7, 0xfff080); dot(x + 39, y - 6, 0xfff080); dot(x + 40, y - 5, 0xfff080); dot(x + 39, y - 4, 0xfff080); glowAt(x + 40, y - 6, 5, 5, 0xfff0a0, 0.25); }
    fill(x + 36, y, 8, 1, SHADOW);
    if (isNight && bolt) glowLate(() => glowAt(x + 40, y - 6, 6, 6, 0xfff0a0, 0.35));
  },
  /** Beast's flask: an Erlenmeyer flask of bright blue liquid bubbling on a little stand, bubbles rising and popping at the neck. It glows faintly at night. */
  xmenflask(x, y) {
    const blue = 0x3a9aff, blueLight = 0x8ad0ff;
    fill(x + 36, y - 1, 9, 1, 0x4a4e58); fill(x + 37, y - 2, 1, 1, 0x4a4e58); fill(x + 43, y - 2, 1, 1, 0x4a4e58);
    fill(x + 39, y - 12, 3, 4, 0xdff0f6, 0.5); fill(x + 38, y - 8, 5, 2, 0xdff0f6, 0.5); fill(x + 37, y - 6, 7, 4, 0xdff0f6, 0.5);
    fill(x + 38, y - 7, 5, 1, blue, 0.85); fill(x + 37, y - 6, 7, 4, blue, 0.85); dot(x + 37, y - 6, blueLight);
    fill(x + 39, y - 13, 3, 1, 0xc8c4bc);
    for (let k = 0; k < 3; k++) { const ph = (now * 0.8 + k / 3) % 1; dot(x + 39 + k, y - 4 - Math.round(ph * 8), blueLight, 0.8 * (1 - ph)); }
    fill(x + 36, y, 9, 1, SHADOW);
    if (isNight) glowLate(() => glowAt(x + 40, y - 5, 5, 4, 0x3a9aff, 0.14));
  },
  /** A scale model of a red car: low body, tall rear wing, front wing and fat black wheels, a yellow helmet in the cockpit; now and then a little puff comes from its exhaust, as if it revved. */
  f1car(x, y) {
    const red = 0xd81a1a, redDark = 0x9a1010;
    fill(x + 34, y - 3, 12, 2, red); fill(x + 37, y - 4, 5, 1, red); fill(x + 34, y - 1, 12, 1, redDark);
    fill(x + 33, y - 7, 2, 4, red); fill(x + 32, y - 7, 4, 1, 0x1c1a22); fill(x + 44, y - 2, 3, 1, 0x1c1a22);
    dot(x + 39, y - 5, 0xf2d020); dot(x + 40, y - 5, 0x2a2a30);
    disc(x + 36, y - 1, 1.6, 0x1c1c20); disc(x + 43, y - 1, 1.6, 0x1c1c20); dot(x + 36, y - 1, 0x6e727c); dot(x + 43, y - 1, 0x6e727c);
    fill(x + 33, y + 1, 13, 1, SHADOW);
    const t = (now + x * 0.19) % 15;
    if (t < 1.2) { const r = Math.floor(t * 3); dot(x + 32 - r, y - 3 - r, 0xd8d4cc, 0.6 - r * 0.15); }
  },
  /** A steering wheel lying flat-on: two grips, a little screen and buttons; now and then its row of shift lights climbs green, red, blue and resets. */
  f1wheel(x, y) {
    const body = 0x24262a, grip = 0x1c1a22;
    fill(x + 34, y - 8, 3, 7, grip); fill(x + 43, y - 8, 3, 7, grip);
    fill(x + 36, y - 7, 8, 6, body); fill(x + 36, y - 7, 8, 1, 0x3a3c42);
    fill(x + 38, y - 5, 4, 2, 0x2a6ab0); dot(x + 37, y - 3, 0xd81a1a); dot(x + 42, y - 3, 0xf2c41a); dot(x + 37, y - 5, 0x5ad86a); dot(x + 42, y - 5, 0xff8000);
    const t = (now + x * 0.23) % 7, n = t < 1.8 ? Math.floor(t * 4) : 0;
    for (let k = 0; k < 7; k++) dot(x + 37 + k, y - 9, k < n ? (k < 2 ? 0x5ad86a : k < 5 ? 0xff3a2a : 0x4a8aff) : 0x3a3c42);
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A soft tyre stood on its edge: black rubber with a red band and a silver rim; heat shimmers off it now and then. */
  f1tyre(x, y) {
    disc(x + 40, y - 6, 5.5, 0x1c1c20); ring(x + 40, y - 6, 4, 0xd81a1a); disc(x + 40, y - 6, 2.2, 0x8a8e98); dot(x + 40, y - 6, 0x2a2a30);
    dot(x + 37, y - 9, 0x3a3a40); fill(x + 36, y, 9, 1, SHADOW);
    const t = (now + x * 0.17) % 10;
    if (t < 1.5) for (let k = 0; k < 3; k++) dot(x + 38 + k * 2, y - 13 - (Math.floor(now * 6) + k) % 2, 0xf4f4f4, 0.3);
  },
  /** A podium champagne bottle in green and gold; every 16 s the cork pops and a spray of foam arcs out and rains down. */
  f1champagne(x, y) {
    const glass = 0x1a4a2a, gold = 0xd8b45a;
    fill(x + 38, y - 7, 4, 7, glass); fill(x + 38, y - 7, 1, 7, 0x2a6a3a); fill(x + 39, y - 10, 2, 3, glass); fill(x + 38, y - 5, 4, 2, gold); fill(x + 39, y - 11, 2, 1, gold);
    fill(x + 37, y, 6, 1, SHADOW);
    const t = (now + x * 0.29) % 16;
    if (t < 2) {
      const k = Math.floor(t * 8);
      for (let j = 0; j < Math.min(k, 8); j++) dot(x + 40 + j, y - 12 - Math.round(Math.sin((j / 8) * Math.PI) * 4), 0xf8f4e0, 0.85);
      if (k < 4) dot(x + 40, y - 13 - k, 0xc8a050);
    } else dot(x + 39, y - 12, 0xc8a050);
  },
  /** A team radio headset in papaya orange with a black boom mic, resting on the desk; now and then little radio waves pulse from the mic: box, box. */
  f1headset(x, y) {
    const band = 0xff8000, cup = 0x2a2a30;
    ring(x + 39, y - 5, 4.5, band); fill(x + 34, y - 5, 3, 5, cup); fill(x + 42, y - 5, 3, 5, cup); dot(x + 35, y - 3, band); dot(x + 43, y - 3, band);
    line(x + 36, y - 1, x + 32, y - 2, 0x1c1a22); dot(x + 32, y - 3, 0x1c1a22);
    fill(x + 34, y + 1, 12, 1, SHADOW);
    const t = (now + x * 0.21) % 11;
    if (t < 1.5) { const r = Math.floor(t * 3); ring(x + 32, y - 3, 1 + r, 0x5ad86a, 0.6 - r * 0.15); }
  },
  /** A red team cap lying on the desk with its peak turned out and a yellow shield on the front; a glint crosses the peak now and then. */
  f1cap(x, y) {
    const red = 0xd81a1a;
    fill(x + 36, y - 6, 7, 5, red); fill(x + 37, y - 7, 5, 1, red); fill(x + 36, y - 6, 1, 5, 0xe84040); fill(x + 42, y - 6, 1, 5, 0x9a1010);
    fill(x + 41, y - 2, 5, 2, 0xb81414); fill(x + 41, y - 2, 5, 1, 0xd83030); dot(x + 39, y - 8, red);
    fill(x + 38, y - 5, 2, 3, 0xf2c41a); dot(x + 38, y - 4, 0x1c1a22);
    const t = (now + x * 0.31) % 12;
    if (t < 0.6) dot(x + 42 + Math.floor(t * 6), y - 2, WHITE, 0.7);
    fill(x + 35, y, 11, 1, SHADOW);
  },
  /** A pit-stop wheel gun with its coiled air hose; every 12 s it whirs for a moment, a blur of spin at its socket and a spark. */
  f1wheelgun(x, y) {
    const body = 0x4a4e58, light = 0x6e727c;
    fill(x + 36, y - 7, 8, 4, body); fill(x + 36, y - 7, 8, 1, light); fill(x + 38, y - 3, 3, 3, 0x1c1a22); fill(x + 44, y - 6, 2, 2, 0x8a8e98);
    for (let k = 0; k < 3; k++) ring(x + 35, y - 2, 1.5 + k * 0.4, 0xf2c41a, 0.5);
    const t = (now + x * 0.27) % 12;
    if (t < 0.9) { const f = Math.floor(now * 16) % 2; dot(x + 47, y - 6 + f, WHITE, 0.8); dot(x + 47, y - 5 - f, 0xa8adb6, 0.6); if (t < 0.2) dot(x + 48, y - 4, 0xffd040); }
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A little chequered flag on a stick in a weighted stand; every so often it waves, its cloth rippling. */
  f1flag(x, y) {
    fill(x + 37, y - 2, 5, 2, 0x2a2a30); fill(x + 39, y - 12, 1, 10, 0x8a8e98);
    const t = (now + x * 0.25) % 10, wave = t < 2 ? Math.floor(t * 6) % 2 : 0;
    for (let r = 0; r < 3; r++) for (let c = 0; c < 3; c++) fill(x + 40 + c * 2, y - 12 + r * 2 + (c === 2 ? wave : 0), 2, 2, (r + c) % 2 ? 0x1c1a22 : 0xf4f4f4);
    fill(x + 36, y, 9, 1, SHADOW);
  },
  /** A pair of open-fingered fight gloves, red and blue, one lying on the other with white wrist straps. */
  ufcmitts(x, y) {
    fill(x + 34, y - 4, 6, 4, 0xd8202a); fill(x + 34, y - 4, 6, 1, 0xe84a4a); fill(x + 33, y - 3, 2, 2, 0xf4f4f4); dot(x + 40, y - 2, 0xd8202a);
    fill(x + 39, y - 6, 6, 4, 0x2a4ab0); fill(x + 39, y - 6, 6, 1, 0x4a6ad0); fill(x + 44, y - 5, 2, 2, 0xf4f4f4); dot(x + 38, y - 4, 0x2a4ab0);
    for (const k of [35, 37, 39]) dot(x + k, y - 1, 0xb81414);
    fill(x + 33, y, 13, 1, SHADOW);
  },
  /** A clear mouthguard in its open case, a little red one with a white rim; now and then the lid snaps shut and open again. */
  ufcmouthguard(x, y) {
    fill(x + 35, y - 3, 9, 3, 0x2a2a30); fill(x + 35, y - 3, 9, 1, 0x3a3a42);
    const t = (now + x * 0.21) % 13, shut = t < 0.8;
    if (shut) fill(x + 35, y - 5, 9, 2, 0x3a3a42);
    else { fill(x + 35, y - 8, 9, 2, 0x3a3a42); fill(x + 37, y - 4, 5, 1, 0xd8202a); dot(x + 36, y - 4, WHITE); dot(x + 42, y - 4, WHITE); }
    fill(x + 34, y, 11, 1, SHADOW);
  },
  /** Two rolled hand wraps, black and red, one half unrolled across the desk; every so often a loose end flutters. */
  ufcwraps(x, y) {
    fill(x + 34, y - 4, 4, 4, 0x1c1a22); ring(x + 36, y - 2, 1, 0x3a3a42); fill(x + 39, y - 5, 4, 5, 0xd8202a); dot(x + 40, y - 4, 0xb81414);
    const t = (now + x * 0.17) % 10, fl = t < 1 ? Math.floor(t * 6) % 2 : 0;
    fill(x + 43, y - 1, 3, 1, 0xd8202a); dot(x + 46, y - 1 - fl, 0xd8202a);
    fill(x + 33, y, 13, 1, SHADOW);
  },
  /** A squeeze bottle of water with a red cap and a long spout; now and then a drop beads at the tip and falls. */
  ufcbottle(x, y) {
    fill(x + 38, y - 9, 5, 9, 0xdff0f6, 0.55); fill(x + 38, y - 6, 5, 6, 0x6ab0e0, 0.6); fill(x + 38, y - 9, 1, 9, WHITE, 0.4);
    fill(x + 38, y - 11, 5, 2, 0xd8202a); line(x + 41, y - 12, x + 43, y - 14, 0xf4f4f4);
    const t = (now + x * 0.19) % 9;
    if (t < 1) dot(x + 43, y - 13 + Math.floor(t * 8), 0x6ab0e0, 0.8);
    fill(x + 37, y, 7, 1, SHADOW);
  },
  /** The cornerman's bucket: a red bucket with a sponge on its rim and a white towel over its side; a drip falls from the sponge now and then. */
  ufcbucket(x, y) {
    fill(x + 35, y - 8, 10, 8, 0xd8202a); fill(x + 36, y - 1, 8, 1, 0x9a1010); fill(x + 35, y - 8, 10, 1, 0xe84a4a); fill(x + 35, y - 8, 1, 7, 0xe84a4a);
    fill(x + 37, y - 10, 4, 2, 0xf2d35b); dot(x + 37, y - 10, 0xf8e890);
    fill(x + 43, y - 8, 3, 6, 0xf4f4f4); dot(x + 45, y - 2, 0xd8d4cc);
    const t = (now + x * 0.23) % 8;
    if (t < 0.8) dot(x + 39, y - 8 + Math.floor(t * 6), 0x6ab0e0, 0.8);
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A blue gel ice pack lying on a folded towel, frosty at its corners; now and then a little cold mist curls off it. */
  ufcicepack(x, y) {
    fill(x + 34, y - 2, 12, 2, 0xf4f4f4); fill(x + 34, y - 1, 12, 1, 0xd8d4cc);
    fill(x + 35, y - 5, 9, 3, 0x4a9ae8); fill(x + 35, y - 5, 9, 1, 0x8ac8f8); dot(x + 35, y - 5, WHITE); dot(x + 43, y - 3, WHITE, 0.7);
    const t = (now + x * 0.27) % 11;
    if (t < 2) { const r = Math.floor(t * 2); dot(x + 38 + r, y - 7 - r, 0xe0f0ff, 0.5 - r * 0.15); dot(x + 41 - r, y - 7 - r, 0xe0f0ff, 0.4 - r * 0.1); }
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A judge's scorecard on a clipboard: three rounds scored in pencil with a little red and blue column; the pencil taps it now and then. */
  ufcscorecard(x, y) {
    fill(x + 35, y - 10, 9, 10, 0x8a5a32); fill(x + 36, y - 9, 7, 8, 0xf4f2ee); fill(x + 38, y - 11, 3, 2, 0x8a8e98);
    fill(x + 37, y - 8, 1, 6, 0xd8202a, 0.6); fill(x + 41, y - 8, 1, 6, 0x2a4ab0, 0.6);
    for (let r = 0; r < 3; r++) { dot(x + 38, y - 7 + r * 2, 0x2a2a30); dot(x + 39, y - 7 + r * 2, 0x2a2a30); dot(x + 42, y - 7 + r * 2, 0x2a2a30); }
    const t = (now + x * 0.31) % 7, tap = t < 1 ? Math.floor(t * 6) % 2 : 0;
    line(x + 44, y - 2 - tap, x + 46, y - 7 - tap, 0xf2d35b); dot(x + 44, y - 2 - tap, 0x2a2a30);
    fill(x + 34, y, 11, 1, SHADOW);
  },
  /** The cutman's kit: a steel enswell, a tub of petroleum jelly with its lid off and a few cotton swabs in a cup; the steel glints now and then. */
  ufccutman(x, y) {
    fill(x + 34, y - 3, 5, 3, 0xa8adb6); fill(x + 34, y - 3, 5, 1, 0xd0d4dc); fill(x + 36, y - 6, 1, 3, 0x6e727c);
    fill(x + 39, y - 4, 4, 4, 0xf4f2ee); fill(x + 39, y - 4, 4, 1, 0xe8d8a8); dot(x + 40, y - 3, 0xe8d8a8);
    fill(x + 43, y - 4, 3, 4, 0x2a4ab0); for (const k of [43, 44, 45]) { dot(x + k, y - 6, 0xf4f4f4); dot(x + k, y - 5, 0xd8d4cc); }
    const t = (now + x * 0.17) % 9;
    if (t < 0.5) dot(x + 35 + Math.floor(t * 8), y - 3, WHITE);
    fill(x + 33, y, 13, 1, SHADOW);
  },
  /** An everything bagel on its paper, cut and spread thick with cream cheese, seeds speckled over the crust. */
  nyloftbagel(x, y) {
    fill(x + 34, y - 1, 12, 1, 0xf2ead8); fill(x + 34, y - 1, 12, 1, 0xe8dcc0);
    fill(x + 35, y - 5, 9, 4, 0xc88a4a); fill(x + 36, y - 6, 7, 1, 0xd89a5a); fill(x + 35, y - 3, 9, 1, 0xf6f2e8); dot(x + 39, y - 5, 0x8a5a2a);
    for (const [dx, dy] of [[36, -5], [38, -6], [41, -5], [42, -4], [37, -4]]) dot(x + dx, y + dy, (dx % 2) ? 0x2a2a30 : 0xf6f2e8);
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A deli coffee in a blue and white paper cup with a key-pattern band round it and a plastic lid; a wisp of steam rises now and then. */
  nyloftcoffee(x, y) {
    fill(x + 37, y - 8, 6, 8, 0xf4f2ee); fill(x + 37, y - 8, 1, 8, WHITE); fill(x + 42, y - 8, 1, 8, 0xd8d4cc);
    fill(x + 37, y - 6, 6, 3, 0x2a5ab0); for (let k = 0; k < 6; k += 2) dot(x + 37 + k, y - 5, 0xf4f2ee);
    fill(x + 36, y - 9, 8, 1, 0xe8e4dc); dot(x + 41, y - 10, 0xe8e4dc);
    fill(x + 36, y, 8, 1, SHADOW);
    const t = (now + x * 0.23) % 12;
    if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 40 + (r % 2), y - 11 - r, WHITE, 0.45 - r * 0.1); }
  },
  /** A big folded slice of cheese pizza on a white paper plate, a dusting of oregano; a string of cheese droops off it now and then. */
  nyloftslice(x, y) {
    fill(x + 33, y - 1, 13, 1, 0xf4f2ee);
    for (let k = 0; k < 5; k++) fill(x + 35 + k, y - 3 - k, 9 - k * 2 + k, 2, 0xf2c84a);
    fill(x + 34, y - 3, 10, 2, 0xf2c84a); fill(x + 43, y - 4, 2, 3, 0xd89040); dot(x + 37, y - 4, 0x6a8a3a); dot(x + 40, y - 6, 0x6a8a3a);
    const t = (now + x * 0.21) % 10;
    if (t < 1.4) fill(x + 35, y - 2, 1, 1 + Math.floor(t * 2), 0xf8e090);
    fill(x + 33, y, 13, 1, SHADOW);
  },
  /** A little yellow toy cab with a light on its roof and a black and white check down its side; now and then its roof light blinks. */
  nylofttaxi(x, y) {
    const yellow = 0xf2c41a;
    fill(x + 34, y - 4, 11, 3, yellow); fill(x + 36, y - 6, 6, 2, yellow); fill(x + 37, y - 6, 4, 1, 0x8ac0e0); fill(x + 34, y - 4, 11, 1, 0xf8d860);
    for (let k = 0; k < 10; k += 2) dot(x + 35 + k, y - 3, 0x1c1a22);
    const t = (now + x * 0.27) % 6;
    fill(x + 38, y - 7, 2, 1, t < 0.5 ? 0xfff4a0 : 0xd8c86a);
    disc(x + 36, y - 1, 1.3, 0x1c1c20); disc(x + 43, y - 1, 1.3, 0x1c1c20);
    fill(x + 34, y + 1, 11, 1, SHADOW);
  },
  /** A rosette succulent in a little grey concrete pot, its fleshy leaves pale green tipped with pink; a new leaf stirs now and then. */
  nyloftsucculent(x, y) {
    fill(x + 36, y - 4, 8, 4, 0x8a8e88); fill(x + 36, y - 4, 8, 1, 0xa8aca6); fill(x + 43, y - 4, 1, 4, 0x6a6e68);
    const t = (now + x * 0.17) % 14, s = t < 1 ? 1 : 0;
    for (const [dx, dy] of [[37, -6], [39, -7], [41, -6], [38, -5], [40, -5], [42, -5]]) { fill(x + dx, y + dy, 2, 2, 0x8ab89a); dot(x + dx, y + dy, 0xe8a0b0); }
    dot(x + 40, y - 8 - s, 0x9ac8aa);
    fill(x + 35, y, 10, 1, SHADOW);
  },
  /** A yellow transit card lying on the desk beside a few coins; a glint crosses the card now and then. */
  nyloftcard(x, y) {
    fill(x + 35, y - 2, 8, 2, 0xf2c41a); fill(x + 35, y - 2, 8, 1, 0xf8d860); fill(x + 36, y - 1, 6, 1, 0x1c1a22, 0.4);
    disc(x + 44, y - 1, 1, 0xa8adb6); dot(x + 44, y - 2, 0xc8a050);
    const t = (now + x * 0.13) % 8;
    if (t < 0.5) dot(x + 36 + Math.floor(t * 12), y - 2, WHITE);
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A soft pretzel from a street cart, glossy brown with flakes of salt, on a square of wax paper with a little pot of mustard. */
  nyloftpretzel(x, y) {
    fill(x + 34, y - 1, 11, 1, 0xe8dcc0);
    const brown = 0xa8603a;
    ring(x + 38, y - 5, 2.5, brown); ring(x + 41, y - 5, 2.5, brown); line(x + 36, y - 2, x + 43, y - 2, brown); dot(x + 39, y - 4, brown);
    for (const [dx, dy] of [[37, -7], [42, -6], [40, -3], [36, -5]]) dot(x + dx, y + dy, WHITE);
    fill(x + 44, y - 3, 2, 2, 0xf2c41a);
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** An old film camera in black and chrome with a leather strap curled beside it; now and then its flash pops. */
  nyloftcamera(x, y) {
    fill(x + 35, y - 6, 9, 6, 0x1c1a22); fill(x + 35, y - 6, 9, 2, 0xc8ccd4); fill(x + 35, y - 6, 1, 6, 0x3a3a42);
    disc(x + 39, y - 3, 2, 0x2a2a30); ring(x + 39, y - 3, 2, 0x8a8e98); dot(x + 38, y - 4, 0x8ab0e0); fill(x + 41, y - 7, 2, 1, 0x3a3a42);
    for (let k = 0; k < 4; k++) dot(x + 44 + (k % 2), y - 1 - k, 0x7a4a2a);
    const t = (now + x * 0.29) % 17;
    if (t < 0.25) { fill(x + 42, y - 6, 2, 1, WHITE); glowAt(x + 43, y - 6, 6, 4, 0xffffff, 0.4); }
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A golden croissant on a little white plate, its curled layers shining with butter; a few flakes drop off it now and then. */
  pariscafecroissant(x, y) {
    fill(x + 34, y - 1, 12, 1, WHITE); fill(x + 35, y, 10, 1, 0xd8d4cc);
    const gold = 0xd8963a, light = 0xf2b85a, dark = 0xa8642a;
    fill(x + 36, y - 4, 8, 3, gold); fill(x + 35, y - 3, 1, 2, gold); fill(x + 44, y - 3, 1, 2, gold); dot(x + 34, y - 2, dark); dot(x + 45, y - 2, dark);
    for (const k of [37, 40, 42]) fill(x + k, y - 4, 1, 3, dark); fill(x + 37, y - 5, 6, 1, light);
    const t = (now + x * 0.21) % 12;
    if (t < 1) dot(x + 38 + Math.floor(t * 4), y - 1, light);
  },
  /** An espresso in a thick white demitasse on its saucer with a sugar cube and a tiny spoon; a wisp of steam curls up now and then. */
  pariscafeespresso(x, y) {
    fill(x + 35, y - 1, 9, 1, WHITE); fill(x + 36, y, 7, 1, 0xd8d4cc);
    fill(x + 37, y - 5, 5, 4, WHITE); fill(x + 37, y - 5, 5, 1, 0x4a2a1a); dot(x + 42, y - 4, WHITE); dot(x + 43, y - 3, WHITE);
    fill(x + 44, y - 2, 2, 2, 0xf4f2ee); line(x + 35, y - 2, x + 34, y - 4, 0xa8adb6);
    const t = (now + x * 0.27) % 10;
    if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 39 + (r % 2), y - 7 - r, WHITE, 0.45 - r * 0.1); }
    fill(x + 34, y + 1, 11, 1, SHADOW);
  },
  /** A little tower of macarons in pastel pink, pistachio, lemon and lavender, each with its frilly foot and cream filling. */
  pariscafemacarons(x, y) {
    const cols = [0xf2a0b8, 0xb8d890, 0xf2e070, 0xc8a8e8];
    cols.forEach((c, i) => { const my = y - 2 - i * 3, mx = x + 37 + (i % 2); fill(mx, my, 6, 2, c); fill(mx, my + 1, 6, 1, WHITE, 0.7); dot(mx, my, shaded(c, 1.15)); dot(mx + 5, my + 1, shaded(c, 0.8)); });
    fill(x + 36, y, 8, 1, SHADOW);
  },
  /** A baguette poking out of a paper bag, crust golden with its slashes; the bag's top crinkles now and then. */
  pariscafebaguette(x, y) {
    fill(x + 37, y - 7, 6, 7, 0xe8dcc0); fill(x + 37, y - 7, 1, 7, 0xf4ecd8); fill(x + 42, y - 7, 1, 7, 0xc8b898);
    line(x + 39, y - 7, x + 44, y - 13, 0xd8963a); line(x + 40, y - 7, x + 45, y - 13, 0xc8862a); line(x + 39, y - 8, x + 44, y - 14, 0xe8a84a);
    dot(x + 41, y - 10, 0xf2c87a); dot(x + 43, y - 12, 0xf2c87a);
    const t = (now + x * 0.19) % 13;
    dot(x + 37 + (t < 0.8 ? 1 : 0), y - 8, 0xe8dcc0);
    fill(x + 36, y, 8, 1, SHADOW);
  },
  /** A glass of red wine on a slim stem, the light catching its bowl; now and then it's swirled and the wine climbs the glass. */
  pariscafewine(x, y) {
    fill(x + 37, y - 1, 5, 1, 0xdff0f6, 0.7); fill(x + 39, y - 5, 1, 4, 0xdff0f6, 0.7);
    fill(x + 37, y - 10, 5, 5, 0xdff0f6, 0.4); fill(x + 37, y - 10, 1, 5, 0xdff0f6, 0.7);
    const t = (now + x * 0.31) % 11, sw = t < 1.5 ? Math.floor(t * 6) % 2 : 0;
    fill(x + 37, y - 8 - sw, 5, 3 + sw, 0x7a1a2a, 0.9); dot(x + 38, y - 9, WHITE, 0.6);
    fill(x + 36, y, 7, 1, SHADOW);
  },
  /** A wooden board with a wedge of soft white-rinded cheese, a blue cheese and a few grapes, a little knife beside them. */
  pariscafecheese(x, y) {
    fill(x + 33, y - 2, 13, 2, 0xa8703e); fill(x + 33, y - 2, 13, 1, 0xc88a52);
    fill(x + 34, y - 5, 5, 3, 0xf6f0e0); fill(x + 34, y - 5, 5, 1, WHITE); fill(x + 35, y - 4, 3, 2, 0xf2e2a0);
    fill(x + 40, y - 4, 3, 2, 0xe8e4d4); dot(x + 41, y - 4, 0x5a7ab0); dot(x + 42, y - 3, 0x5a7ab0);
    for (const [dx, dy] of [[44, -3], [45, -4], [44, -5]]) dot(x + dx, y + dy, 0x6a3a6a);
    line(x + 34, y - 1, x + 39, y - 1, 0xa8adb6);
    fill(x + 33, y, 13, 1, SHADOW);
  },
  /** A single red rose in a slim glass bud vase; now and then a petal falls to the table. */
  pariscaferose(x, y) {
    fill(x + 38, y - 7, 3, 7, 0xdff0f6, 0.5); fill(x + 38, y - 7, 1, 7, 0xdff0f6, 0.75); fill(x + 38, y - 3, 3, 3, 0x9ac8d8, 0.5);
    line(x + 39, y - 7, x + 39, y - 11, 0x3a7a3a); dot(x + 40, y - 9, 0x4a8a4a);
    fill(x + 38, y - 13, 3, 2, 0xc8202a); dot(x + 39, y - 14, 0xa8101a); dot(x + 38, y - 13, 0xe84a4a);
    const t = (now + x * 0.17) % 30;
    if (t < 2) dot(x + 41 + Math.floor(t), y - 11 + Math.floor(t * 5), 0xc8202a); else if (t < 25) dot(x + 43, y - 1, 0xc8202a, 0.8);
    fill(x + 37, y, 5, 1, SHADOW);
  },
  /** The morning paper folded on the table beside a pair of round reading glasses; its corner lifts in a draught now and then. */
  pariscafepaper(x, y) {
    fill(x + 33, y - 3, 11, 3, 0xe8e4d8); fill(x + 33, y - 3, 11, 1, 0xf4f2ee);
    for (let k = 0; k < 4; k++) fill(x + 34 + k * 2, y - 2, 1, 1, 0x5a5a62); fill(x + 34, y - 1, 8, 1, 0xa8a8b0, 0.6);
    const t = (now + x * 0.29) % 15;
    if (t < 0.8) dot(x + 43, y - 4, 0xe8e4d8);
    ring(x + 41, y - 5, 1.2, 0x2a2a30); ring(x + 44, y - 5, 1.2, 0x2a2a30);
    fill(x + 33, y, 12, 1, SHADOW);
  },
  /** A big sugar skull decorated in icing: pink and turquoise flower eyes, a yellow flower on its brow, a stitched smile and a gold foil name band; a glint of sugar crystals catches the light now and then. */
  muertossugarskull(x, y) {
    fill(x + 35, y - 9, 9, 7, 0xf6f2ea); fill(x + 36, y - 10, 7, 1, 0xf6f2ea); fill(x + 36, y - 2, 7, 2, 0xf6f2ea); fill(x + 35, y - 9, 1, 7, WHITE);
    ring(x + 37, y - 6, 1.4, 0xe83a8a); dot(x + 37, y - 6, 0x1c1a22); ring(x + 42, y - 6, 1.4, 0x1a8a8a); dot(x + 42, y - 6, 0x1c1a22);
    dot(x + 39, y - 9, 0xf2c41a); dot(x + 40, y - 9, 0xf2c41a); dot(x + 39, y - 4, 0x8a3ab0);
    for (let k = 0; k < 5; k++) dot(x + 37 + k, y - 2, k % 2 ? 0x1c1a22 : 0xe83a8a);
    fill(x + 36, y - 10, 7, 1, 0xd8b45a);
    const t = (now + x * 0.19) % 8;
    if (t < 0.4) dot(x + 36 + Math.floor(t * 20), y - 8, WHITE);
    fill(x + 34, y, 11, 1, SHADOW);
  },
  /** A round loaf of pan de muerto dusted with sugar, its top crossed with dough bones and a little knob in the middle; a puff of sugar dust lifts off now and then. */
  muertospan(x, y) {
    fill(x + 35, y - 5, 10, 5, 0xd8963a); fill(x + 36, y - 6, 8, 1, 0xd8963a); fill(x + 35, y - 1, 10, 1, 0xa8642a);
    line(x + 36, y - 5, x + 44, y - 2, 0xb8742a); line(x + 44, y - 5, x + 36, y - 2, 0xb8742a); fill(x + 39, y - 8, 2, 2, 0xc8843a);
    for (const [dx, dy] of [[36, -5], [38, -6], [41, -6], [43, -4], [37, -3], [42, -2]]) dot(x + dx, y + dy, 0xf6f0e0);
    const t = (now + x * 0.23) % 12;
    if (t < 1) dot(x + 40, y - 9 - Math.floor(t * 3), WHITE, 0.5);
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A tall glass veladora candle painted with marigolds, its flame flickering above the glass and glowing warm, through the night too. */
  muertoscandle(x, y) {
    fill(x + 38, y - 9, 4, 9, 0xdff0f6, 0.55); fill(x + 38, y - 6, 4, 4, 0xe8902a, 0.85); dot(x + 39, y - 5, 0xf2c41a); fill(x + 38, y - 9, 1, 9, WHITE, 0.4);
    fill(x + 38, y - 8, 4, 1, 0xf6f2ea);
    const f = candlelight(x * 0.17);
    dot(x + 40, y - 10, 0xffd36b, f); dot(x + 40, y - 11, 0xff8a3d, 0.55 * f);
    glowAt(x + 40, y - 9, 5, 5, 0xffc060, 0.08 * f);
    fill(x + 37, y, 6, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x + 40, y - 9, 7, 7, 0xffb050, 0.22 * f); dot(x + 40, y - 10, 0xfff4c0); });
  },
  /** A clay pot of bright marigolds, cempasúchil, their ruffled orange heads above dark leaves; one nods now and then. */
  muertosmarigolds(x, y) {
    fill(x + 36, y - 4, 8, 4, 0xb8603a); fill(x + 35, y - 5, 10, 1, 0xc8703e); fill(x + 36, y - 1, 8, 1, 0x8a4a2a);
    for (const [dx, dy] of [[36, -7], [39, -8], [42, -7], [38, -6], [41, -6]]) { dot(x + dx, y + dy + 2, 0x3a7a3a); }
    const t = (now + x * 0.21) % 9, nod = t < 1 ? 1 : 0;
    for (const [dx, dy, k] of [[36, -9, 0], [39, -11, 1], [42, -9, 2]]) { const n = k === 1 ? nod : 0; fill(x + dx, y + dy + n, 3, 3, 0xe8902a); dot(x + dx + 1, y + dy + n, 0xf2b03a); dot(x + dx + 1, y + dy + 1 + n, 0xc86a1a); }
    fill(x + 35, y, 10, 1, SHADOW);
  },
  /** A little clay copal burner on three feet, a white curl of fragrant smoke winding up from the resin. */
  muertoscopal(x, y) {
    fill(x + 36, y - 5, 8, 3, 0x9a5a3a); fill(x + 37, y - 6, 6, 1, 0xb8703e); fill(x + 37, y - 2, 1, 2, 0x7a4a2a); fill(x + 42, y - 2, 1, 2, 0x7a4a2a); fill(x + 39, y - 2, 1, 2, 0x7a4a2a);
    dot(x + 39, y - 7, 0xe8502a, 0.7 + 0.3 * candlelight(x * 0.2)); dot(x + 40, y - 7, 0xf6f0e0);
    for (let k = 0; k < 5; k++) { const ph = (now * 0.35 + k / 5) % 1; dot(x + 40 + Math.round(Math.sin(ph * 7 + k) * 1.5), y - 8 - Math.round(ph * 6), 0xe8e4dc, 0.55 * (1 - ph)); }
    fill(x + 35, y, 10, 1, SHADOW);
  },
  /** A clay mug of hot chocolate with a carved wooden whisk standing in it, foam on top; now and then the whisk twirls and a curl of steam rises. */
  muertoschocolate(x, y) {
    fill(x + 36, y - 6, 7, 6, 0xb8603a); fill(x + 36, y - 6, 1, 6, 0xc8703e); fill(x + 43, y - 4, 1, 3, 0xb8603a); dot(x + 44, y - 4, 0xb8603a);
    fill(x + 37, y - 6, 5, 1, 0xd8b890); dot(x + 38, y - 4, 0xf6f2ea); dot(x + 40, y - 3, 0xe83a8a);
    const t = (now + x * 0.27) % 10, tw = t < 1.2 ? Math.floor(t * 8) % 2 : 0;
    fill(x + 39 + tw, y - 12, 1, 6, 0x8a5a32); fill(x + 38 + tw, y - 9, 3, 2, 0xa8703e);
    if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 37, y - 8 - r, WHITE, 0.4 - r * 0.1); }
    fill(x + 35, y, 9, 1, SHADOW);
  },
  /** A pair of painted maracas crossed on the desk, striped pink, yellow and green; every so often one gives a little shake. */
  muertosmaracas(x, y) {
    const t = (now + x * 0.33) % 11, sh = t < 0.8 ? Math.floor(now * 14) % 2 : 0;
    line(x + 35, y - 1, x + 39, y - 4, 0x8a5a32); disc(x + 41, y - 6 - sh, 2.5, 0xe83a8a); dot(x + 41, y - 7 - sh, 0xf2c41a); dot(x + 42, y - 5 - sh, 0x3a9a4a);
    line(x + 44, y - 1, x + 41, y - 3, 0x8a5a32); disc(x + 38, y - 4, 2.2, 0xf2c41a); dot(x + 38, y - 5, 0x1a8a8a); dot(x + 37, y - 3, 0xe83a8a);
    if (sh) { dot(x + 44, y - 8, WHITE, 0.6); dot(x + 45, y - 6, WHITE, 0.5); }
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A little carved alebrije: a fantastical winged cat-like creature painted in dots and stripes of every colour; now and then it seems to blink. */
  muertosfigurine(x, y) {
    fill(x + 36, y - 5, 7, 4, 0x1aa8a8); fill(x + 36, y - 1, 1, 1, 0xe83a8a); fill(x + 42, y - 1, 1, 1, 0xe83a8a); fill(x + 38, y - 1, 1, 1, 0xf2c41a); fill(x + 40, y - 1, 1, 1, 0xf2c41a);
    for (const [dx, dy, c] of [[37, -4, 0xf2c41a], [39, -5, 0xe83a8a], [41, -4, 0x8a3ab0], [38, -3, 0xe8902a]]) dot(x + dx, y + dy, c);
    fill(x + 42, y - 8, 3, 3, 0x8a3ab0); dot(x + 42, y - 9, 0x8a3ab0); dot(x + 44, y - 9, 0x8a3ab0);
    const blink = (now + x * 0.3) % 6 < 0.2;
    dot(x + 43, y - 7, blink ? 0x8a3ab0 : 0xf2c41a);
    fill(x + 37, y - 8, 3, 3, 0xe83a8a); dot(x + 36, y - 9, 0xf2c41a); dot(x + 35, y - 6, 0xe8902a);
    fill(x + 35, y, 10, 1, SHADOW);
  },
  /** A specimen jar with a glowing violet jellyfish drifting inside, its tentacles trailing; it pulses softly, through the night too. */
  submarinedeepjar(x, y) {
    fill(x + 37, y - 11, 7, 11, 0xdff0f6, 0.35); fill(x + 37, y - 12, 7, 1, 0x5a6670); fill(x + 37, y - 11, 1, 11, 0xdff0f6, 0.6); fill(x + 37, y - 4, 7, 4, 0x6ab0d0, 0.3);
    const b = Math.round(Math.sin(now * 1.2 + x) * 1.5);
    fill(x + 39, y - 9 + b, 3, 2, 0xc080ff, 0.9); dot(x + 39, y - 7 + b, 0xc080ff, 0.6); dot(x + 41, y - 6 + b, 0xc080ff, 0.6); dot(x + 40, y - 5 + b, 0xc080ff, 0.4);
    glowAt(x + 40, y - 7, 4, 4, 0xc080ff, 0.08);
    fill(x + 36, y, 9, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(x + 40, y - 8 + b, 4, 4, 0xc080ff, 0.25); fill(x + 39, y - 9 + b, 3, 2, 0xe0c0ff); });
  },
  /** A model of a little yellow ROV, the remote diving robot: a boxy frame with thrusters, a camera eye and two lights that blink on in turn. */
  submarinedeeprov(x, y) {
    fill(x + 35, y - 7, 10, 6, 0xf2c41a); fill(x + 35, y - 7, 10, 1, 0xf8d860); fill(x + 35, y - 2, 10, 1, 0xc89a10);
    fill(x + 36, y - 6, 8, 3, 0x2a2a30, 0.3); disc(x + 43, y - 4, 1.2, 0x1c1a22); dot(x + 43, y - 4, 0x4a8aff);
    fill(x + 34, y - 8, 2, 3, 0x5a6670); fill(x + 44, y - 8, 2, 3, 0x5a6670);
    const k = Math.floor(now * 1.5) % 2;
    dot(x + 45, y - 6, k ? 0xfff4c0 : 0x8a8a70); dot(x + 45, y - 3, k ? 0x8a8a70 : 0xfff4c0);
    fill(x + 36, y - 1, 2, 1, 0x2a2a30); fill(x + 42, y - 1, 2, 1, 0x2a2a30);
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A nautilus shell, cream striped with rust-brown flames, its spiral opening facing out; a glint runs round it now and then. */
  submarinedeepshell(x, y) {
    disc(x + 40, y - 5, 4.5, 0xf2e6d0); ring(x + 40, y - 5, 4.5, 0xd8c8a8);
    for (let k = 0; k < 4; k++) line(x + 37 + k * 2, y - 9, x + 36 + k * 2, y - 4, 0xb8603a, 0.8);
    disc(x + 42, y - 4, 2, 0xe8d8b8); disc(x + 42, y - 4, 1, 0x8a6a4a);
    const t = (now + x * 0.17) % 10;
    if (t < 0.6) { const a = t * 10; dot(x + 40 + Math.round(Math.cos(a) * 4), y - 5 + Math.round(Math.sin(a) * 4), WHITE); }
    fill(x + 35, y, 10, 1, SHADOW);
  },
  /** A tall orange steel thermos with a black cup-lid off beside it, steaming now and then. */
  submarinedeepthermos(x, y) {
    fill(x + 37, y - 11, 4, 11, 0xe8702a); fill(x + 37, y - 11, 1, 11, 0xf2904a); fill(x + 40, y - 11, 1, 11, 0xb8501a); fill(x + 37, y - 12, 4, 1, 0xa8b2ba);
    fill(x + 42, y - 3, 3, 3, 0x2a2a30); fill(x + 42, y - 3, 3, 1, 0x6a4a2a);
    const t = (now + x * 0.21) % 11;
    if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 43 + (r % 2), y - 5 - r, WHITE, 0.45 - r * 0.1); }
    fill(x + 36, y, 9, 1, SHADOW);
  },
  /** A branch of pink deep-sea coral in a shallow sample tray with a tag tied to it; a tiny white crab peeks out of it now and then. */
  submarinedeepcoral(x, y) {
    fill(x + 34, y - 2, 12, 2, 0xe8ecee); fill(x + 34, y - 2, 12, 1, WHITE);
    line(x + 39, y - 2, x + 39, y - 9, 0xf28fb0); line(x + 39, y - 6, x + 36, y - 9, 0xf28fb0); line(x + 39, y - 5, x + 42, y - 9, 0xf28fb0); line(x + 41, y - 7, x + 43, y - 7, 0xf28fb0);
    for (const [dx, dy] of [[36, -9], [39, -9], [42, -9], [43, -7]]) dot(x + dx, y + dy, 0xf8c0d0);
    fill(x + 44, y - 4, 2, 2, 0xf2e6c8); line(x + 42, y - 5, x + 44, y - 4, 0x8a8e98);
    const t = (now + x * 0.29) % 14;
    if (t < 1.5) { dot(x + 36, y - 3, WHITE); dot(x + 35, y - 4, WHITE); }
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A waterproof dive log, its orange cover open on a page of ruled entries and a sketched fish, a pencil tied to it on a cord; the pencil adds a line now and then. */
  submarinedeeplog(x, y) {
    fill(x + 34, y - 3, 12, 3, 0xe8702a); fill(x + 35, y - 4, 10, 3, 0xf4f6f8); fill(x + 40, y - 4, 1, 3, 0xd0d6da);
    for (let k = 0; k < 4; k++) dot(x + 36 + k, y - 3, 0x5a6670); fill(x + 42, y - 3, 2, 1, 0x2a90d0); dot(x + 44, y - 3, 0x2a90d0);
    const t = (now + x * 0.23) % 9, w = t < 1.5 ? Math.floor(t * 3) : 0;
    line(x + 37 + w, y - 2, x + 39 + w, y - 7, 0xf2c41a); dot(x + 37 + w, y - 2, 0x2a2a30);
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A rugged yellow handheld radio with a stubby antenna and a green status light; now and then it crackles with a burst of signal. */
  submarinedeepradio(x, y) {
    fill(x + 38, y - 9, 5, 9, 0xf2c41a); fill(x + 38, y - 9, 1, 9, 0xf8d860); fill(x + 42, y - 9, 1, 9, 0xc89a10);
    fill(x + 39, y - 8, 3, 3, 0x1c242e); fill(x + 40, y - 13, 1, 4, 0x2a2a30);
    for (let k = 0; k < 3; k++) fill(x + 39, y - 4 + k, 3, 1, 0x2a2a30, 0.5);
    const t = (now + x * 0.19) % 10, talk = t < 1.5;
    dot(x + 41, y - 10, talk ? 0xff3a2a : 0x5ad86a);
    if (talk) { const r = Math.floor(t * 4) % 3; ring(x + 40, y - 13, 1 + r, 0x5ad86a, 0.5 - r * 0.12); }
    fill(x + 37, y, 7, 1, SHADOW);
  },
  /** An open clam shell holding a single pearl that shines; now and then the shell closes and opens again. */
  submarinedeeppearl(x, y) {
    const t = (now + x * 0.27) % 16, shut = t < 1;
    fill(x + 35, y - 3, 10, 3, 0xd8c8b8); fill(x + 35, y - 3, 10, 1, 0xf0e4d8); for (let k = 0; k < 10; k += 2) dot(x + 35 + k, y - 1, 0xb8a898);
    if (shut) fill(x + 35, y - 5, 10, 2, 0xc8b8a8);
    else { fill(x + 36, y - 9, 8, 5, 0xc8b8a8); fill(x + 36, y - 9, 8, 1, 0xe0d4c8); for (let k = 0; k < 8; k += 2) dot(x + 36 + k, y - 7, 0xa89888); disc(x + 40, y - 3, 1.5, 0xf8f4f0); dot(x + 39, y - 4, WHITE); }
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A length of ice core in a clear tube on a little cradle, its layers banded white and pale blue with a dark line of old ash; a drip of meltwater falls from its end now and then. */
  arcticbasecore(x, y) {
    fill(x + 34, y - 2, 2, 2, 0x5a626a); fill(x + 43, y - 2, 2, 2, 0x5a626a);
    fill(x + 33, y - 6, 13, 4, 0xdff0f6, 0.4); for (let k = 0; k < 12; k += 2) fill(x + 34 + k, y - 5, 1, 2, k % 4 ? 0xe8f4fa : 0xb8d8ec); fill(x + 40, y - 5, 1, 2, 0x5a5a62);
    fill(x + 33, y - 6, 13, 1, WHITE, 0.6);
    const t = (now + x * 0.19) % 10;
    if (t < 1) dot(x + 46, y - 3 + Math.floor(t * 4), 0x9ac8e8);
    fill(x + 33, y, 13, 1, SHADOW);
  },
  /** A big enamel mug of cocoa piled with marshmallows; steam curls up from it now and then. */
  arcticbasecocoa(x, y) {
    fill(x + 37, y - 7, 7, 7, 0x2a5ab0); fill(x + 37, y - 7, 1, 7, 0x4a7ad0); dot(x + 38, y - 6, WHITE); dot(x + 41, y - 3, WHITE); fill(x + 44, y - 5, 1, 3, 0x2a5ab0); dot(x + 45, y - 4, 0x2a5ab0);
    fill(x + 37, y - 8, 7, 1, 0x6a3a22); fill(x + 38, y - 9, 2, 2, 0xf6f2ea); fill(x + 41, y - 9, 2, 1, 0xf8e8e0);
    const t = (now + x * 0.23) % 9;
    if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 40 + (r % 2), y - 11 - r, WHITE, 0.45 - r * 0.1); }
    fill(x + 36, y, 9, 1, SHADOW);
  },
  /** A pair of thick red mittens with white snowflake patterns, laid one over the other, still dusted with snow that melts away. */
  arcticbasemittens(x, y) {
    fill(x + 34, y - 4, 6, 4, 0xc8302a); fill(x + 34, y - 6, 3, 2, 0xc8302a); fill(x + 34, y - 1, 6, 1, 0xf2ece0); dot(x + 36, y - 3, WHITE);
    fill(x + 39, y - 5, 6, 4, 0xa82420); fill(x + 43, y - 7, 2, 2, 0xa82420); fill(x + 39, y - 2, 6, 1, 0xf2ece0); dot(x + 41, y - 4, WHITE);
    if ((now + x * 0.21) % 15 < 8) { dot(x + 35, y - 5, WHITE, 0.8); dot(x + 42, y - 6, WHITE, 0.8); }
    fill(x + 33, y, 13, 1, SHADOW);
  },
  /** A digital thermometer with a probe, its red readout showing the cold outside; the number flickers as it updates now and then. */
  arcticbasethermo(x, y) {
    fill(x + 37, y - 9, 6, 9, 0xe8ecee); fill(x + 37, y - 9, 1, 9, WHITE); fill(x + 42, y - 9, 1, 9, 0xb8c2ca);
    fill(x + 38, y - 8, 4, 3, 0x1c1a22); const t = (now + x * 0.27) % 7, flick = t < 0.3;
    if (!flick) { dot(x + 38, y - 7, 0xff3a2a); fill(x + 39, y - 8, 1, 3, 0xff3a2a); fill(x + 41, y - 8, 1, 3, 0xff3a2a); }
    dot(x + 39, y - 3, 0x2a5ab0); line(x + 43, y - 2, x + 46, y - 1, 0x8a929a);
    fill(x + 36, y, 8, 1, SHADOW);
  },
  /** An orange GPS beacon with a stubby antenna, its strobe flashing white every couple of seconds, bright through the night. */
  arcticbasebeacon(x, y) {
    fill(x + 37, y - 7, 6, 7, 0xe8702a); fill(x + 37, y - 7, 1, 7, 0xf2904a); fill(x + 42, y - 7, 1, 7, 0xb8501a); fill(x + 38, y - 4, 4, 2, 0x1c1a22);
    fill(x + 41, y - 11, 1, 4, 0x2a2a30);
    const flash = (now + x * 0.13) % 2.2 < 0.15;
    fill(x + 38, y - 9, 2, 2, flash ? WHITE : 0xc8ccd4);
    if (flash) glowAt(x + 39, y - 8, 5, 4, 0xffffff, 0.3);
    fill(x + 36, y, 8, 1, SHADOW);
    if (isNight && flash) glowLate(() => { glowAt(x + 39, y - 8, 6, 5, 0xffffff, 0.4); fill(x + 38, y - 9, 2, 2, WHITE); });
  },
  /** A pair of snow goggles with an orange mirrored lens and a broad strap, the sky gleaming across the lens now and then. */
  arcticbasegoggles(x, y) {
    line(x + 34, y - 2, x + 46, y - 2, 0x2a2a30); fill(x + 36, y - 6, 9, 4, 0x2a2a30);
    fill(x + 37, y - 5, 7, 2, 0xe8902a); fill(x + 37, y - 5, 7, 1, 0xf8c060); dot(x + 40, y - 3, 0x2a2a30);
    const t = (now + x * 0.29) % 9;
    if (t < 0.6) dot(x + 37 + Math.floor(t * 12), y - 5, WHITE);
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A snowflake under a brass loupe on a little black card: the six-armed crystal enlarged in the lens; it sparkles now and then before it melts and a new one is caught. */
  arcticbasecrystal(x, y) {
    fill(x + 34, y - 2, 8, 2, 0x1c1a22);
    ring(x + 40, y - 6, 4, 0xc8a050); fill(x + 43, y - 3, 3, 1, 0x8a5a32); disc(x + 40, y - 6, 3.4, 0x1a2a44, 0.6);
    const t = (now + x * 0.2) % 20, melt = t > 17;
    if (!melt) { for (let k = 0; k < 3; k++) { const a = (k / 3) * Math.PI; line(x + 40 - Math.round(Math.cos(a) * 2.5), y - 6 - Math.round(Math.sin(a) * 2.5), x + 40 + Math.round(Math.cos(a) * 2.5), y - 6 + Math.round(Math.sin(a) * 2.5), 0xd8f0ff); } }
    else dot(x + 40, y - 6, 0x9ac8e8);
    if (t < 0.6) dot(x + 41, y - 7, WHITE);
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A little camp stove with a blue flame under a dented tin pot of soup, steam rising from it; the flame flickers and glows at night. */
  arcticbasestove(x, y) {
    fill(x + 36, y - 3, 8, 3, 0x5a626a); fill(x + 36, y - 3, 8, 1, 0x8a929a);
    const f = candlelight(x * 0.11);
    dot(x + 39, y - 4, 0x4a8aff, f); dot(x + 40, y - 4, 0x8ab8ff, f); dot(x + 41, y - 4, 0x4a8aff, f);
    fill(x + 36, y - 9, 8, 5, 0xa8b2ba); fill(x + 36, y - 9, 1, 5, 0xd0d6da); fill(x + 36, y - 9, 8, 1, 0xc86a3a); line(x + 44, y - 8, x + 46, y - 9, 0x5a626a);
    for (let k = 0; k < 2; k++) { const ph = (now * 0.5 + k / 2) % 1; dot(x + 39 + k * 2, y - 10 - Math.round(ph * 4), WHITE, 0.45 * (1 - ph)); }
    fill(x + 35, y, 10, 1, SHADOW);
    if (isNight) glowLate(() => glowAt(x + 40, y - 4, 4, 2, 0x6aa0ff, 0.25));
  },
  /** Two sakura mochi, pink rice cakes each wrapped in a pickled cherry leaf, on a little square plate; a petal settles on them now and then. */
  sakuramochi(x, y) {
    fill(x + 34, y - 1, 12, 1, 0xf6f2ea); fill(x + 35, y, 10, 1, 0xd8d4cc);
    for (const mx of [35, 40]) { fill(mx + x, y - 5, 5, 4, 0xf6b8c8); fill(mx + x, y - 5, 5, 1, 0xfad0dc); fill(mx + x + 3, y - 4, 2, 3, 0x6a8a4a); dot(mx + x + 4, y - 5, 0x8aa86a); }
    if ((now + x * 0.2) % 14 < 5) dot(x + 37, y - 6, WHITE);
  },
  /** A skewer of three-colour dango, pink, white and green, on a little leaf-shaped dish; the skewer rolls a little now and then. */
  sakuradango(x, y) {
    fill(x + 34, y - 1, 12, 1, 0x8ab06a);
    const r = (now + x * 0.23) % 11 < 0.8 ? 1 : 0;
    line(x + 34 + r, y - 3, x + 46, y - 6, 0xd8b888);
    for (const [dx, c] of [[37, 0xf6b8c8], [40, 0xf6f2ea], [43, 0x9ac87a]]) { disc(x + dx + r, y - 4 - Math.floor((dx - 37) / 3), 1.6, c); dot(x + dx + r - 1, y - 5 - Math.floor((dx - 37) / 3), WHITE, 0.6); }
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A bowl of whisked matcha, bright green and frothy, with its bamboo whisk standing beside it; a curl of steam rises now and then. */
  sakuramatcha(x, y) {
    fill(x + 34, y - 5, 8, 5, 0x5a6a5a); fill(x + 35, y - 1, 6, 1, 0x3a4a3a); fill(x + 34, y - 5, 8, 1, 0x9ac85a); dot(x + 36, y - 5, 0xc8e88a); fill(x + 34, y - 4, 1, 3, 0x7a8a7a);
    fill(x + 43, y - 6, 3, 6, 0xd8c890); for (let k = 0; k < 3; k++) dot(x + 43 + k, y - 7, 0xc8b878); fill(x + 44, y - 9, 1, 3, 0xc8b878);
    const t = (now + x * 0.21) % 10;
    if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 38 + (r % 2), y - 7 - r, WHITE, 0.45 - r * 0.1); }
    fill(x + 33, y, 13, 1, SHADOW);
  },
  /** An open bento box: rice with a pickled plum, rolled omelette, a little octopus sausage and greens in its compartments, its lacquered lid propped beside it. */
  sakurabento(x, y) {
    fill(x + 33, y - 5, 10, 5, 0x2a2a30); fill(x + 33, y - 5, 10, 1, 0x4a4a52); fill(x + 38, y - 4, 1, 4, 0xd8302a);
    fill(x + 34, y - 4, 4, 3, WHITE); dot(x + 36, y - 3, 0xd8302a);
    fill(x + 39, y - 4, 3, 1, 0xf2d35b); dot(x + 39, y - 3, 0xe8502a); dot(x + 40, y - 3, 0xe8502a); dot(x + 41, y - 2, 0x5a9a4a); dot(x + 39, y - 2, 0x5a9a4a);
    fill(x + 44, y - 7, 2, 7, 0xd8302a); fill(x + 44, y - 7, 1, 7, 0xe84a4a);
    fill(x + 33, y, 13, 1, SHADOW);
  },
  /** A sprig of cherry blossom in a small white bud vase; now and then a petal falls onto the desk. */
  sakurasprig(x, y) {
    fill(x + 38, y - 5, 4, 5, 0xf6f2ea); fill(x + 38, y - 5, 1, 5, WHITE); fill(x + 39, y - 6, 2, 1, 0xf6f2ea);
    line(x + 40, y - 6, x + 38, y - 12, 0x5a3a2e); line(x + 40, y - 6, x + 43, y - 11, 0x5a3a2e);
    for (const [dx, dy] of [[37, -12], [39, -11], [42, -11], [43, -9], [38, -9]]) { fill(x + dx, y + dy, 2, 2, 0xf6c8d4); dot(x + dx, y + dy, WHITE); }
    const t = (now + x * 0.19) % 11;
    if (t < 1.8) dot(x + 44 + Math.round(Math.sin(t * 4)), y - 9 + Math.floor(t * 5), 0xf6c8d4);
    fill(x + 37, y, 6, 1, SHADOW);
  },
  /** A pale blue glass bottle of marble soda with its glass marble caught in the neck; bubbles rise through it now and then. */
  sakurasoda(x, y) {
    fill(x + 38, y - 7, 4, 7, 0x9ad0e8, 0.7); fill(x + 39, y - 10, 2, 3, 0x9ad0e8, 0.7); fill(x + 38, y - 7, 1, 7, WHITE, 0.4);
    disc(x + 40, y - 8, 1, 0xdff0f6); dot(x + 39, y - 9, WHITE); fill(x + 39, y - 11, 2, 1, 0x2a5ab0);
    const t = (now + x * 0.29) % 6;
    if (t < 1.5) for (let k = 0; k < 2; k++) dot(x + 39 + k, y - 2 - Math.floor(t * 3) - k, WHITE, 0.7);
    fill(x + 37, y, 6, 1, SHADOW);
  },
  /** Two triangular rice balls wrapped in strips of dark seaweed on a square of paper; a sesame seed or two catch the light. */
  sakuraonigiri(x, y) {
    fill(x + 34, y - 1, 12, 1, 0xf2ead8);
    for (const ox of [35, 40]) { for (let k = 0; k < 5; k++) fill(x + ox + Math.floor(k / 2), y - 2 - k, 5 - Math.floor(k / 2) * 2, 1, 0xf8f6f0); fill(x + ox, y - 3, 5, 2, 0x1e2a22); dot(x + ox + 2, y - 5, 0x2a2a30); }
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** An open sketchbook with a watercolour of a blossoming branch, a little round brush and a water tin beside it; now and then the brush adds a dab of pink. */
  sakurasketch(x, y) {
    fill(x + 33, y - 3, 11, 3, 0xf8f6f0); fill(x + 38, y - 3, 1, 3, 0xd8d4cc);
    line(x + 34, y - 1, x + 37, y - 3, 0x8a6a5a); dot(x + 35, y - 2, 0xf6b8c8); dot(x + 36, y - 3, 0xf6b8c8); fill(x + 40, y - 2, 2, 1, 0x9ac8e8, 0.6);
    fill(x + 44, y - 3, 2, 3, 0xa8b2ba); dot(x + 44, y - 3, 0x9ac8e8);
    const t = (now + x * 0.23) % 9, d = t < 1 ? 1 : 0;
    line(x + 41, y - 3 - d, x + 44, y - 7, 0x8a5a32); dot(x + 41, y - 3 - d, 0xf28fb0);
    fill(x + 33, y, 13, 1, SHADOW);
  },
  /** A wand on a little dark stand, its handle carved; every 15 s it gives off a shower of red and gold sparks, and at night its tip shines with a small white light. */
  potterwand(x, y) {
    const wood = 0x5a3a22, woodLight = 0x8a5a32, tipX = x + 44, tipY = y - 7;
    fill(x + 34, y - 2, 10, 2, 0x2a2420); fill(x + 34, y - 2, 10, 1, 0x4a3a30);
    line(x + 34, y - 3, x + 44, y - 7, wood); fill(x + 34, y - 4, 3, 2, woodLight); dot(x + 37, y - 4, wood); dot(x + 34, y - 4, 0xa8784a);
    const t = (now + x * 0.21) % 15;
    if (t < 1.6) for (let k = 0; k < 6; k++) {
      const a = -Math.PI / 2 + (k - 2.5) * 0.45, r = 1 + t * 4;
      dot(tipX + Math.round(Math.cos(a) * r), tipY + Math.round(Math.sin(a) * r), k % 2 ? 0xf2c830 : 0xe8402a, 1 - t / 1.6);
    }
    if (isNight) glowLate(() => { glowAt(tipX, tipY, 2, 2, 0xe8f4ff, 0.45); dot(tipX, tipY, WHITE); });
  },
  /** The Golden Snitch hovering over a little black stand, its silver wings a blur; every 10 s it darts about in a quick zigzag before settling back. */
  pottersnitch(x, y) {
    const gold = 0xe8b830, goldLight = 0xfff0a0, goldDark = 0xa87a18, wing = 0xe8e8f0;
    fill(x + 37, y - 2, 5, 2, 0x2a2a30); fill(x + 38, y - 3, 3, 1, 0x3a3a42);
    const t = (now + x * 0.17) % 10, dart = t < 1.4;
    const ox = dart ? Math.round(Math.sin(t * 11) * 3) : 0, oy = Math.round(Math.sin(now * 2 + x) * 0.6) + (dart ? -Math.round(Math.abs(Math.cos(t * 7)) * 3) : 0);
    const cx = x + 39 + ox, cy = y - 8 + oy, flap = Math.floor(now * 14) % 2;
    if (flap) { line(cx - 2, cy - 1, cx - 4, cy - 3, wing); line(cx + 2, cy - 1, cx + 4, cy - 3, wing); dot(cx - 3, cy - 3, wing, 0.6); dot(cx + 3, cy - 3, wing, 0.6); }
    else { fill(cx - 4, cy - 1, 2, 1, wing); fill(cx + 3, cy - 1, 2, 1, wing); }
    fill(cx - 1, cy - 1, 3, 3, gold); dot(cx - 1, cy - 1, goldLight); dot(cx + 1, cy + 1, goldDark); dot(cx, cy, goldLight, 0.6);
    if (isNight) glowLate(() => glowAt(cx, cy, 3, 3, 0xffe070, 0.15));
  },
  /** The old Sorting Hat, patched and frayed brown leather, slumped with its tip bent over; the folds of its brim make a face, and every 18 s the rip of its mouth opens and it talks for a moment. */
  pottersortinghat(x, y) {
    const hat = 0x6a4a2e, hatLight = 0x8a6440, hatDark = 0x4a321e, patch = 0x7a5a3a, mouth = 0x1e140c;
    fill(x + 33, y - 2, 13, 2, hat); fill(x + 33, y - 2, 13, 1, hatLight); fill(x + 33, y - 1, 13, 1, hatDark);
    for (const [dy, l, w] of [[3, 35, 9], [4, 35, 9], [5, 36, 7], [6, 36, 7], [7, 37, 5], [8, 37, 5], [9, 38, 4], [10, 39, 3]]) fill(x + l, y - dy, w, 1, hat);
    fill(x + 35, y - 7, 1, 4, hatLight); dot(x + 41, y - 11, hat); dot(x + 42, y - 11, hat); dot(x + 43, y - 10, hatDark);
    fill(x + 40, y - 9, 2, 2, patch); dot(x + 40, y - 9, hatDark, 0.6); dot(x + 37, y - 4, patch);
    const t = (now + x * 0.19) % 18, talk = t < 2.5 && Math.floor(now * 6) % 2;
    fill(x + 37, y - 7, 2, 1, hatDark); fill(x + 41, y - 7, 2, 1, hatDark);
    if (talk) { fill(x + 38, y - 5, 4, 2, mouth); dot(x + 38, y - 5, hatDark); } else fill(x + 38, y - 5, 4, 1, hatDark);
  },
  /** The Monster Book of Monsters: a shaggy green book with mean little eyes and teeth along its edge; every 12 s it gapes and snaps shut, shuffling sideways. */
  pottermonsterbook(x, y) {
    const fur = 0x4a6a2a, furLight = 0x6a8a3a, furDark = 0x2e4a1a, tooth = 0xf2ead8, eye = 0xe8c838;
    const t = (now + x * 0.23) % 12, open = t < 1.8 ? Math.floor(t * 5) % 2 : 0, sh = t < 1.8 ? Math.floor(t * 3) % 2 : 0;
    const bx = x + 34 + sh;
    fill(bx, y - 3, 11, 3, fur); fill(bx, y - 3, 11, 1, furLight); fill(bx, y - 1, 11, 1, furDark);
    if (open) {
      fill(bx, y - 8, 11, 3, fur); fill(bx, y - 8, 11, 1, furLight); fill(bx + 1, y - 5, 9, 2, 0x8a1e1e);
      for (let k = 0; k < 9; k += 2) { dot(bx + 1 + k, y - 5, tooth); dot(bx + 2 + k, y - 4, tooth); }
      dot(bx + 8, y - 7, eye); dot(bx + 6, y - 7, eye);
    } else {
      fill(bx, y - 5, 11, 2, fur); fill(bx, y - 5, 11, 1, furLight);
      for (let k = 1; k < 11; k += 2) dot(bx + k, y - 3, tooth);
      dot(bx + 8, y - 5, eye); dot(bx + 6, y - 5, eye);
    }
    for (const [dx, dy] of [[1, -4], [4, -4], [10, -3], [0, -2]]) dot(bx + dx, y + dy - (open && dy < -3 ? 4 : 0), furDark);
    fill(x + 34, y, 11, 1, SHADOW);
  },
  /** A Howler: a scarlet envelope with a wax seal propped on the desk; every 25 s it rises, flaps open into a shouting mouth and shakes, lines of noise bursting off it, then drops back down. */
  potterhowler(x, y) {
    const red = 0xc8281e, redLight = 0xe8483a, redDark = 0x8a1814, seal = 0x5a0a0a;
    const t = (now + x * 0.31) % 25;
    if (t < 3) {
      const sh = Math.floor(now * 12) % 2, up = 5;
      const cx = x + 39 + sh, cy = y - 6 - up;
      fill(cx - 4, cy - 2, 9, 6, red); fill(cx - 4, cy - 2, 9, 1, redLight); fill(cx - 4, cy + 3, 9, 1, redDark);
      fill(cx - 3, cy, 7, 2, 0x2a0a0a); for (const dx of [-3, -1, 1, 3]) dot(cx + dx, cy, WHITE, 0.8);
      dot(cx - 2, cy - 1, 0xffffff); dot(cx + 2, cy - 1, 0xffffff);
      for (const [dx, dy, ex, ey] of [[-6, -2, -8, -4], [6, -2, 8, -4], [-6, 2, -8, 3], [6, 2, 8, 3]]) line(cx + dx, cy + dy, cx + ex, cy + ey, red, 0.7);
      fill(x + 35, y, 9, 1, SHADOW);
      return;
    }
    fill(x + 34, y - 7, 10, 7, red); fill(x + 34, y - 7, 10, 1, redLight); fill(x + 43, y - 7, 1, 7, redDark);
    line(x + 34, y - 7, x + 39, y - 3, redDark); line(x + 43, y - 7, x + 39, y - 3, redDark);
    fill(x + 38, y - 4, 3, 2, seal); dot(x + 38, y - 4, 0x8a1a1a);
    fill(x + 33, y, 12, 1, SHADOW);
  },
  /** A Chocolate Frog's purple five-sided box trimmed in gold with its card beside it; every 14 s the chocolate frog hops out, two big leaps across the desk, and back in. */
  potterchocofrog(x, y) {
    const box = 0x5a2a7a, boxLight = 0x7a4a9a, gold = 0xd3a625, choc = 0x6a3a1e, chocLight = 0x8a5230;
    fill(x + 34, y - 6, 7, 6, box); fill(x + 35, y - 7, 5, 1, box); fill(x + 34, y - 6, 1, 6, boxLight); fill(x + 34, y - 6, 7, 1, gold); fill(x + 36, y - 4, 3, 2, gold, 0.7);
    fill(x + 42, y - 4, 3, 4, 0xeadcb4); dot(x + 43, y - 3, 0x8a6a4a); fill(x + 42, y - 1, 3, 1, gold);
    const t = (now + x * 0.17) % 14;
    if (t < 2.2) {
      const hop = t < 1.1 ? t / 1.1 : (t - 1.1) / 1.1, hx = x + 37 + Math.round((t < 1.1 ? hop : 1 - hop) * 8), hy = y - 2 - Math.round(Math.sin(hop * Math.PI) * 5);
      fill(hx, hy - 2, 3, 2, choc); dot(hx, hy - 2, chocLight); dot(hx + 2, hy - 3, choc); dot(hx - 1, hy - 1, choc); dot(hx + 3, hy - 1, choc);
    } else dot(x + 37, y - 8, choc);
  },
  /** A Time-Turner: a little gold hourglass inside two spinning rings, hung on a long gold chain over a small stand; every 20 s the rings whirl round and the sand glitters. */
  pottertimeturner(x, y) {
    const gold = 0xd3a625, goldLight = 0xf0cf5a, goldDark = 0x9a7418, sand = 0xf2d89a;
    fill(x + 36, y - 1, 7, 1, 0x3a2a20); fill(x + 39, y - 12, 1, 11, 0x5a4030); fill(x + 37, y - 12, 5, 1, 0x5a4030);
    dot(x + 40, y - 11, gold); dot(x + 41, y - 10, gold, 0.8);
    const cx = x + 41, cy = y - 6, t = (now + x * 0.29) % 20, spin = t < 2 ? Math.floor(t * 8) % 4 : 0;
    const rx = [3, 2, 1, 2][spin];
    ellipseRing(cx, cy, rx, 3, gold, 0.95); if (spin !== 2) ellipseRing(cx, cy, 3, [1, 2, 3, 2][spin], goldDark, 0.8);
    dot(cx, cy - 1, sand); dot(cx, cy + 1, sand); dot(cx, cy, goldLight); dot(cx - 1, cy - 2, gold); dot(cx + 1, cy - 2, gold); dot(cx - 1, cy + 2, gold); dot(cx + 1, cy + 2, gold);
    if (t < 2) dot(cx + (Math.floor(now * 9) % 3) - 1, cy + 1, WHITE, 0.9);
  },
  /** A folded copy of the Daily Prophet with its black masthead and a moving photo, a figure in it waving now and then; a pair of round wire glasses lies folded on top. */
  potterprophet(x, y) {
    const paper = 0xe8e2d0, paperDark = 0xc8c0a8, ink = 0x2a2a30, photo = 0x8a8478;
    fill(x + 33, y - 4, 13, 4, paper); fill(x + 33, y - 4, 13, 1, 0xf4f0e2); fill(x + 33, y - 1, 13, 1, paperDark);
    fill(x + 34, y - 7, 11, 3, paper); fill(x + 34, y - 7, 11, 1, ink); dot(x + 36, y - 7, paper); dot(x + 40, y - 7, paper);
    for (const lx of [34, 36, 38]) fill(x + lx, y - 3, 1, 2, ink, 0.4);
    fill(x + 40, y - 6, 4, 4, photo); fill(x + 40, y - 6, 4, 1, 0xa8a298);
    const wave = (now + x * 0.21) % 8 < 2 && Math.floor(now * 4) % 2;
    dot(x + 41, y - 5, 0xe8e2d0); fill(x + 41, y - 4, 1, 2, ink); dot(x + 42, y - 5 - (wave ? 1 : 0), 0xe8e2d0, 0.8);
    // The glasses
    ring(x + 35, y - 9, 1.2, 0x3a3238); ring(x + 39, y - 9, 1.2, 0x3a3238); dot(x + 37, y - 9, 0x3a3238); dot(x + 35, y - 10, WHITE, 0.5);
  },
  /** The One Ring lying on a little dark velvet cushion on its chain; it glints now and then, and every 20 s the fiery letters show round it and fade. */
  lotrring(x, y) {
    const gold = 0xe8b830, goldLight = 0xfff0a0, goldDark = 0xa87a18;
    fill(x + 35, y - 2, 9, 2, 0x3a1a2a); fill(x + 35, y - 2, 9, 1, 0x5a2a3a);
    line(x + 36, y - 3, x + 38, y - 4, 0xc8c8c8, 0.6); line(x + 42, y - 4, x + 44, y - 3, 0xc8c8c8, 0.6);
    ellipseRing(x + 40, y - 4, 2, 1.4, gold); dot(x + 38, y - 4, goldLight); dot(x + 42, y - 4, goldDark);
    if ((now + x * 0.3) % 5 < 0.3) dot(x + 39, y - 5, WHITE);
    const t = (now + x * 0.19) % 20;
    if (t < 2.5) {
      const a = Math.min(1, t * 1.5, 2.5 - t);
      ellipseRing(x + 40, y - 4, 4, 2.6, 0xff6a20, 0.8 * a);
      if (isNight) glowLate(() => glowAt(x + 40, y - 4, 5, 3, 0xff7a30, 0.25 * a));
    }
  },
  /** A hobbit's long-stemmed clay pipe on a stand beside a little pouch of Longbottom Leaf; the bowl glows and now and then a smoke ring floats up, widening as it rises. */
  lotrpipe(x, y) {
    const wood = 0x6a4228, woodLight = 0x8a5a32, pouch = 0x7a6a4a;
    fill(x + 34, y - 3, 4, 3, pouch); fill(x + 34, y - 3, 4, 1, shaded(pouch, 1.2)); dot(x + 35, y - 4, 0x5a4a32); dot(x + 36, y - 4, 0x8aa05a);
    line(x + 38, y - 2, x + 43, y - 3, woodLight); fill(x + 43, y - 6, 3, 4, wood); fill(x + 43, y - 6, 1, 4, woodLight);
    const f = candlelight(x * 0.2); dot(x + 44, y - 6, 0xe8602a, 0.6 + 0.4 * f);
    const t = (now + x * 0.23) % 9;
    if (t < 4) { const r = 1 + t * 0.6, ry = y - 9 - Math.round(t * 2.5); ellipseRing(x + 44, ry, r, r * 0.5, 0xe8e8e8, 0.6 * (1 - t / 4)); }
    if (isNight) glowLate(() => dot(x + 44, y - 6, 0xff7a3a, 0.8));
  },
  /** Elven waybread: two pale lembas cakes, one half unwrapped from its green mallorn leaf tied with a silver thread; a crumb or two now and then as it's nibbled. */
  lotrlembas(x, y) {
    const leaf = 0x6a9a4a, leafDark = 0x4a7a3a, bread = 0xf2e6c4, breadDark = 0xd8c89e;
    fill(x + 34, y - 3, 11, 3, leaf); fill(x + 34, y - 3, 11, 1, shaded(leaf, 1.2)); fill(x + 34, y - 1, 11, 1, leafDark);
    for (let k = 0; k < 11; k += 3) dot(x + 34 + k, y - 2, leafDark);
    fill(x + 36, y - 6, 6, 3, bread); fill(x + 36, y - 6, 6, 1, 0xfff6dc); fill(x + 41, y - 6, 1, 3, breadDark);
    line(x + 36, y - 6, x + 41, y - 4, breadDark, 0.6); dot(x + 34, y - 3, 0xd8dce8); dot(x + 44, y - 3, 0xd8dce8);
    const t = (now + x * 0.17) % 16;
    if (t < 1.5) dot(x + 43, y - 4 + Math.floor(t * 3), breadDark);
  },
  /** The Phial of Galadriel: a little crystal flask on a silver stand holding the light of Eärendil's star; it shines softly, brightens every 12 s, and at night lights the desk round it. */
  lotrphial(x, y) {
    const silver = 0xb8bcc4, silverDark = 0x8a8e96;
    fill(x + 36, y - 1, 7, 1, silverDark); fill(x + 38, y - 3, 3, 2, silver);
    fill(x + 38, y - 9, 3, 6, 0xdff0f6, 0.55); fill(x + 37, y - 7, 5, 3, 0xdff0f6, 0.55); fill(x + 39, y - 10, 1, 1, silver);
    const t = (now + x * 0.21) % 12, b = t < 2 ? 1 : 0.55;
    fill(x + 38, y - 7, 3, 3, 0xf0f8ff, b); dot(x + 39, y - 6, WHITE);
    glowAt(x + 39, y - 6, 5, 5, 0xd8ecff, 0.12 * b);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 6, 8, 7, 0xd0e8ff, 0.3 * b); dot(x + 39, y - 6, WHITE); dot(x + 38, y - 7, 0xe8f4ff); dot(x + 40, y - 5, 0xe8f4ff); });
  },
  /** The Red Book of Westmarch: a fat red leather volume with a gold clasp, open to a page of tiny writing and a little map; its pages riffle now and then. */
  lotrredbook(x, y) {
    const red = 0x9a2a22, redLight = 0xb8402e, redDark = 0x6a1a14, page = 0xf2ead8, ink = 0x5a4a3a;
    fill(x + 33, y - 2, 13, 2, red); fill(x + 33, y - 2, 13, 1, redLight); fill(x + 33, y - 1, 13, 1, redDark);
    fill(x + 34, y - 5, 5, 3, page); fill(x + 40, y - 5, 5, 3, page); fill(x + 39, y - 5, 1, 3, 0xc8c0a8);
    for (const ly of [y - 4, y - 3]) { fill(x + 35, ly, 3, 1, ink, 0.4); }
    dot(x + 42, y - 4, 0x7aa05a); dot(x + 43, y - 3, ink, 0.6); dot(x + 41, y - 3, 0x4a7ac8, 0.6);
    const t = (now + x * 0.29) % 14;
    if (t < 0.8) { const f = Math.floor(t * 5) % 2; fill(x + 39 - f * 2, y - 8, 2, 4, page); dot(x + 39 - f * 2, y - 8, 0xc8c0a8); }
    dot(x + 45, y - 2, 0xd8b45a);
  },
  /** A palantír, the seeing-stone: a dark glass orb on a black iron stand, mist turning slowly inside; every 25 s it flares and a fiery eye looks out of it for a moment. */
  lotrpalantir(x, y) {
    const iron = 0x2a2a30, ironLight = 0x4a4a52, orb = 0x1a1e2a;
    fill(x + 35, y - 2, 9, 2, iron); fill(x + 35, y - 2, 9, 1, ironLight); dot(x + 36, y - 3, iron); dot(x + 42, y - 3, iron);
    disc(x + 39, y - 7, 4, orb); dot(x + 37, y - 9, 0x6a7a9a, 0.8);
    const a = now * 0.8 + x;
    dot(x + 39 + Math.round(Math.cos(a) * 2), y - 7 + Math.round(Math.sin(a) * 2), 0x5a6a8a, 0.8); dot(x + 39 - Math.round(Math.cos(a) * 1.5), y - 7 - Math.round(Math.sin(a) * 1.5), 0x4a5a7a, 0.7);
    const t = (now + x * 0.31) % 25;
    if (t < 1.6) {
      fill(x + 37, y - 7, 5, 1, 0xff8a20); dot(x + 39, y - 8, 0xffc040); dot(x + 39, y - 6, 0xffc040); dot(x + 39, y - 7, 0x1a0808);
      glowAt(x + 39, y - 7, 5, 5, 0xff6a20, 0.2);
      if (isNight) glowLate(() => glowAt(x + 39, y - 7, 6, 5, 0xff6a20, 0.3));
    }
  },
  /** A little wicker basket heaped with brown field mushrooms from Farmer Maggot's, one rolled out on the desk; now and then one wobbles on top of the pile. */
  lotrmushrooms(x, y) {
    const wicker = 0xb08a4a, wickerDark = 0x8a6a34, cap = 0x8a6040, capLight = 0xa87a50, stem = 0xf2e6cc;
    fill(x + 34, y - 4, 9, 4, wicker); fill(x + 34, y - 4, 9, 1, shaded(wicker, 1.15)); for (let k = 0; k < 9; k += 2) dot(x + 34 + k, y - 2, wickerDark);
    line(x + 34, y - 4, x + 38, y - 9, wickerDark); line(x + 38, y - 9, x + 42, y - 4, wickerDark);
    const wob = (now + x * 0.17) % 11 < 0.8 ? (Math.floor(now * 8) % 2 ? 1 : -1) : 0;
    for (const [mx, my] of [[35, -6], [38, -7], [41, -6]]) { fill(x + mx + (my === -7 ? wob : 0), y + my, 3, 2, cap); dot(x + mx + (my === -7 ? wob : 0), y + my, capLight); }
    fill(x + 43, y - 3, 3, 2, cap); dot(x + 43, y - 3, capLight); dot(x + 44, y - 1, stem);
  },
  /** The Horn of Gondor: a great curved ox-horn bound in silver, on a small stand; every 30 s it sounds, faint rings of sound spreading from its mouth. */
  lotrhorn(x, y) {
    const horn = 0xe8dcc0, hornDark = 0xb8a888, silver = 0xb8bcc4;
    fill(x + 35, y - 1, 9, 1, 0x4a3020);
    line(x + 35, y - 3, x + 40, y - 5, horn); line(x + 35, y - 2, x + 41, y - 4, hornDark); line(x + 40, y - 5, x + 43, y - 8, horn); line(x + 41, y - 4, x + 44, y - 7, hornDark);
    fill(x + 43, y - 10, 3, 3, horn); dot(x + 45, y - 9, hornDark);
    for (const [bx, by] of [[37, -4], [41, -6]]) { dot(x + bx, y + by, silver); dot(x + bx, y + by + 1, silver); }
    dot(x + 34, y - 3, silver);
    const t = (now + x * 0.23) % 30;
    if (t < 2) for (let k = 0; k < 2; k++) { const r = (t - k * 0.5) * 4; if (r > 0 && r < 6) ring(x + 44, y - 9, r, 0xf2f0ea, 0.5 * (1 - r / 6)); }
  },
  /** Heisenberg's black porkpie hat on the desk beside a pair of dark glasses; now and then the glasses glint. */
  breakingbadhat(x, y) {
    const hat = 0x1e1e22, hatLight = 0x3a3a42, band = 0x2a2a30;
    fill(x + 33, y - 2, 11, 2, hat); fill(x + 33, y - 2, 11, 1, hatLight);
    fill(x + 35, y - 6, 7, 4, hat); fill(x + 35, y - 6, 7, 1, hatLight); fill(x + 36, y - 7, 5, 1, hat); fill(x + 35, y - 3, 7, 1, band);
    fill(x + 40, y - 1, 2, 1, 0x101014); fill(x + 43, y - 1, 2, 1, 0x101014); dot(x + 42, y - 1, 0x4a4a52);
    if ((now + x * 0.27) % 8 < 0.4) dot(x + 40, y - 1, WHITE, 0.9);
    fill(x + 33, y, 12, 1, SHADOW);
  },
  /** A steel tray of the famous sky-blue crystal, broken into shards; the shards glint in turn and the tray glows faintly blue at night. */
  breakingbadcrystal(x, y) {
    const steel = 0xa8adb6, steelDark = 0x6e727c, blue = 0x6ac8f0, blueLight = 0xc8f0ff, blueDark = 0x3a8ac8;
    fill(x + 33, y - 2, 12, 2, steel); fill(x + 33, y - 2, 12, 1, 0xd0d4da); fill(x + 33, y - 1, 12, 1, steelDark);
    const shards = [[34, -4, 2], [36, -5, 3], [39, -4, 2], [41, -5, 2], [43, -4, 1], [37, -3, 2], [40, -3, 3]];
    shards.forEach(([sx, sy, w], k) => { fill(x + sx, y + sy, w, 2, k % 2 ? blue : blueDark); dot(x + sx, y + sy, blueLight); });
    const g = Math.floor((now + x * 0.1) * 2) % 14;
    if (g < shards.length) dot(x + shards[g][0], y + shards[g][1], WHITE);
    if (isNight) glowLate(() => glowAt(x + 39, y - 4, 6, 3, 0x6ac8f0, 0.18));
  },
  /** An Erlenmeyer flask of blue liquid on a hot plate, bubbles rising through it and a wisp of vapour from its neck every so often. */
  breakingbadflask(x, y) {
    const plate = 0x3a3e46, glass = 0xdff0f6, liquid = 0x4aa8e0;
    fill(x + 35, y - 2, 9, 2, plate); fill(x + 35, y - 2, 9, 1, 0x5a5e66); dot(x + 42, y - 1, 0xe8402a);
    fill(x + 38, y - 11, 3, 3, glass, 0.5); fill(x + 37, y - 8, 5, 2, glass, 0.5); fill(x + 36, y - 6, 7, 4, glass, 0.5);
    fill(x + 36, y - 5, 7, 3, liquid, 0.9); fill(x + 37, y - 6, 5, 1, liquid, 0.8); dot(x + 36, y - 6, WHITE, 0.7);
    const b = Math.floor(now * 4 + x) % 4; dot(x + 38 + (b % 2) * 2, y - 3 - b, 0xc8f0ff, 0.9);
    if ((now + x * 0.2) % 7 < 2) { const v = Math.floor(now * 3) % 3; dot(x + 39 + (v % 2), y - 12 - v, WHITE, 0.5 - v * 0.12); }
    if (isNight) glowLate(() => dot(x + 42, y - 1, 0xff6a3a));
  },
  /** A tall stack of banded cash bundles, green with paper bands; every 20 s the top bundle's bills riffle as they're counted. */
  breakingbadmoney(x, y) {
    const bill = 0x8ab07a, billDark = 0x5a8050, billLight = 0xb8d0a8, band = 0xe8dcb4;
    for (let k = 0; k < 4; k++) { const by = y - 2 - k * 2, off = k % 2; fill(x + 34 + off, by, 9, 2, bill); fill(x + 34 + off, by, 9, 1, billLight); dot(x + 42 + off, by + 1, billDark); fill(x + 37 + off, by, 2, 2, band); }
    fill(x + 43, y - 2, 3, 2, bill); fill(x + 43, y - 2, 3, 1, billLight); dot(x + 44, y - 2, band);
    const t = (now + x * 0.19) % 20;
    if (t < 1.5) { const f = Math.floor(t * 8) % 3; fill(x + 35, y - 10 - f, 7, 1, billLight); dot(x + 35, y - 11 - f, bill); }
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A red-and-yellow Los Pollos Hermanos chicken bucket with drumsticks poking out of the top; steam curls up from it now and then. */
  breakingbadpollosbox(x, y) {
    const red = 0xc8281e, yellow = 0xf2c830, chicken = 0xc8803a, chickenLight = 0xe8a050;
    fill(x + 35, y - 7, 8, 7, red); fill(x + 36, y - 1, 6, 1, shaded(red, 0.8)); fill(x + 35, y - 5, 8, 2, yellow); dot(x + 38, y - 5, 0xf6f2ea); dot(x + 39, y - 5, 0xf6f2ea); fill(x + 35, y - 7, 1, 7, shaded(red, 1.2));
    for (const [cx, cy] of [[36, -9], [38, -10], [40, -9]]) { fill(x + cx, y + cy, 2, 2, chicken); dot(x + cx, y + cy, chickenLight); }
    dot(x + 42, y - 9, 0xf2ead8); dot(x + 42, y - 8, chicken);
    const t = (now + x * 0.23) % 10; if (t < 2) { const r = Math.floor(t * 2); dot(x + 38 + (r % 2), y - 12 - r, WHITE, 0.45 - r * 0.1); }
  },
  /** The pink teddy bear, a little singed, one glass eye missing, sitting slumped on the desk; it slowly tips over to one side every so often and is set upright again. */
  breakingbadbear(x, y) {
    const pink = 0xe890a8, pinkDark = 0xc06a84, singe = 0x6a4a4a, eye = 0x2a2a30;
    const tip = (now + x * 0.31) % 25 < 4 ? 1 : 0;
    if (tip) {
      fill(x + 34, y - 4, 8, 4, pink); fill(x + 41, y - 5, 4, 4, pink); dot(x + 42, y - 4, eye); dot(x + 44, y - 6, pinkDark); dot(x + 36, y - 3, singe); fill(x + 34, y, 11, 1, SHADOW);
      return;
    }
    fill(x + 37, y - 6, 6, 6, pink); fill(x + 36, y - 2, 2, 2, pink); fill(x + 42, y - 2, 2, 2, pink); fill(x + 38, y - 3, 4, 2, 0xf2b8c8);
    fill(x + 37, y - 11, 6, 5, pink); dot(x + 37, y - 12, pinkDark); dot(x + 42, y - 12, pinkDark); dot(x + 38, y - 9, eye); dot(x + 41, y - 9, singe, 0.6);
    fill(x + 39, y - 8, 2, 1, pinkDark); dot(x + 40, y - 5, singe); dot(x + 37, y - 4, singe, 0.7); dot(x + 42, y - 10, singe, 0.5);
    fill(x + 36, y, 9, 1, SHADOW);
  },
  /** A cheap flip-open burner phone on the desk; every 22 s it buzzes, shuffling sideways, its little screen lighting green. */
  breakingbadphone(x, y) {
    const body = 0x2a2a30, light = 0x4a4a52;
    const t = (now + x * 0.29) % 22, buzz = t < 2 ? Math.floor(now * 14) % 2 : 0;
    const px = x + 37 + buzz;
    fill(px, y - 2, 7, 2, body); fill(px, y - 2, 7, 1, light); for (let k = 1; k < 7; k += 2) dot(px + k, y - 1, 0x6a6a72);
    fill(px + 1, y - 7, 5, 5, body); fill(px + 2, y - 6, 3, 3, t < 2 ? 0x8af08a : 0x3a4a3a); dot(px + 1, y - 7, light);
    if (t < 2) { dot(px - 2, y - 4, WHITE, 0.5); dot(px + 8, y - 4, WHITE, 0.5); if (isNight) glowLate(() => glowAt(px + 3, y - 5, 3, 3, 0x8af08a, 0.25)); }
  },
  /** A little model of the battered old RV, white with a brown stripe and blue curtains; now and then its tiny vent puffs a wisp of smoke. */
  breakingbadrv(x, y) {
    const white = 0xf2ead8, cream = 0xd8ccb0, stripe = 0x8a5a32, window = 0x5a7a9a, tyre = 0x2a2a30;
    fill(x + 33, y - 7, 12, 5, white); fill(x + 33, y - 7, 12, 1, 0xfff8ec); fill(x + 42, y - 6, 3, 4, cream);
    fill(x + 33, y - 4, 12, 1, stripe); fill(x + 34, y - 6, 2, 1, window); fill(x + 37, y - 6, 3, 1, window); fill(x + 43, y - 6, 2, 1, window);
    fill(x + 33, y - 3, 12, 1, cream); dot(x + 35, y - 2, tyre); dot(x + 36, y - 2, tyre); dot(x + 42, y - 2, tyre); dot(x + 43, y - 2, tyre);
    fill(x + 36, y - 8, 2, 1, 0xa8adb6);
    const t = (now + x * 0.21) % 12; if (t < 3) { const r = Math.floor(t * 1.5); dot(x + 37 - r, y - 9 - r, 0xd8d8d8, 0.6 - r * 0.15); }
    fill(x + 33, y - 1, 12, 1, SHADOW);
  },
  /** A plate stacked with golden toaster waffles beside the yellow-and-red box they came from; now and then the top waffle vanishes and pops back. */
  strangerwaffles(x, y) {
    const waffle = 0xd89a3a, waffleLight = 0xf0b850, waffleDark = 0xa86a20, plate = 0xf2f2ee;
    fill(x + 33, y - 1, 9, 1, plate); fill(x + 32, y - 2, 11, 1, 0xd8d8d8);
    const gone = (now + x * 0.29) % 18 < 2 ? 1 : 0;
    for (let k = 0; k < 3 - gone; k++) { const wy = y - 3 - k * 2; fill(x + 34, wy, 7, 2, waffle); fill(x + 34, wy, 7, 1, waffleLight); dot(x + 36, wy + 1, waffleDark); dot(x + 38, wy + 1, waffleDark); }
    fill(x + 42, y - 9, 4, 9, 0xf2d040); fill(x + 42, y - 9, 4, 1, 0xfff070); fill(x + 42, y - 7, 4, 2, 0xd8302a); dot(x + 43, y - 4, waffle); dot(x + 44, y - 4, waffleLight);
  },
  /** A chunky black walkie-talkie with its antenna up; every 15 s its red light glows and little crackling lines of static burst off it. */
  strangerwalkie(x, y) {
    const body = 0x2a2a30, bodyLight = 0x4a4a52;
    fill(x + 37, y - 9, 6, 9, body); fill(x + 37, y - 9, 6, 1, bodyLight); fill(x + 37, y - 9, 1, 9, bodyLight);
    for (let k = 0; k < 3; k++) fill(x + 38, y - 6 + k * 2, 4, 1, 0x1a1a1e);
    fill(x + 41, y - 15, 1, 6, 0x6a6a72); dot(x + 41, y - 16, 0x8a8a92);
    const t = (now + x * 0.23) % 15, talk = t < 2.5;
    dot(x + 39, y - 8, talk ? 0xff3a2a : 0x5a1a1a);
    if (talk) { const f = Math.floor(now * 8) % 2; line(x + 44, y - 12 + f, x + 46, y - 13 + f, WHITE, 0.6); line(x + 44, y - 9 - f, x + 46, y - 8 - f, WHITE, 0.5); if (isNight) glowLate(() => dot(x + 39, y - 8, 0xff5a3a)); }
  },
  /** A spill of polyhedral dice, a big red twenty-sider among them; every 16 s the twenty-sider rolls, tumbling a little way across the desk. */
  strangerdice(x, y) {
    const red = 0xc8281e, redLight = 0xe8503a, W = 0xf6f2ea;
    fill(x + 34, y - 2, 2, 2, 0x3a8ac8); dot(x + 34, y - 2, 0x6ab0e8); fill(x + 43, y - 2, 2, 2, 0x3ac85a); dot(x + 43, y - 2, 0x7ae88a); fill(x + 37, y - 2, 2, 2, 0xe8c838);
    const t = (now + x * 0.21) % 16, roll = t < 1.2, dx = roll ? Math.round(t / 1.2 * 3) : 3, hop = roll ? Math.round(Math.abs(Math.sin(t * 8))) : 0;
    const cx = x + 37 + dx, cy = y - 5 - hop;
    fill(cx, cy, 4, 4, red); dot(cx + 1, cy - 1, red); dot(cx + 2, cy - 1, red); fill(cx, cy, 4, 1, redLight); dot(cx + 3, cy + 3, 0x8a1a14);
    if (!roll) { dot(cx + 1, cy + 1, W); dot(cx + 2, cy + 2, W); }
  },
  /** A brass pocket compass lying open; every 20 s its needle starts spinning wildly, pointing to the gate, then settles back to north. */
  strangercompass(x, y) {
    const brass = 0xc8a050, brassLight = 0xe8c878, face = 0xf2ead8;
    disc(x + 39, y - 4, 4, brass); disc(x + 39, y - 4, 3, face); dot(x + 36, y - 6, brassLight);
    fill(x + 38, y - 9, 3, 1, brass); dot(x + 39, y - 10, brass);
    const t = (now + x * 0.27) % 20, a = t < 3 ? now * 14 : -Math.PI / 2;
    dot(x + 39 + Math.round(Math.cos(a) * 2), y - 4 + Math.round(Math.sin(a) * 2), 0xc8281e); dot(x + 39 + Math.round(Math.cos(a) * 1), y - 4 + Math.round(Math.sin(a) * 1), 0xc8281e);
    dot(x + 39 - Math.round(Math.cos(a) * 2), y - 4 - Math.round(Math.sin(a) * 2), 0x3a3a42); dot(x + 39, y - 4, 0x2a2a30);
  },
  /** A little painted Demogorgon miniature on a round base, its tall grey body hunched; every 14 s its head splits open into red petals, then closes again. */
  strangerdemogorgon(x, y) {
    const skin = 0x8a7a72, skinDark = 0x5a4e4a, petal = 0xc8402a, inner = 0x6a1a1a;
    fill(x + 36, y - 1, 7, 1, 0x2a2a30); fill(x + 37, y - 2, 5, 1, 0x4a4a52);
    fill(x + 38, y - 8, 3, 6, skin); fill(x + 38, y - 8, 1, 6, 0xa89a90); dot(x + 37, y - 6, skin); dot(x + 41, y - 6, skin); dot(x + 36, y - 4, skinDark); dot(x + 42, y - 4, skinDark);
    dot(x + 38, y - 2, skinDark); dot(x + 40, y - 2, skinDark);
    const open = (now + x * 0.19) % 14 < 1.5;
    if (open) { dot(x + 37, y - 12, petal); dot(x + 41, y - 12, petal); dot(x + 36, y - 10, petal); dot(x + 42, y - 10, petal); dot(x + 39, y - 13, petal); fill(x + 38, y - 11, 3, 2, inner); dot(x + 39, y - 10, 0xf2ead8); }
    else { fill(x + 38, y - 11, 3, 3, skin); dot(x + 39, y - 11, skinDark); }
  },
  /** A blue cassette Walkman with orange foam headphones plugged in; every 12 s little music notes rise from the headphones, as a song plays on and on. */
  strangerwalkman(x, y) {
    const blue = 0x3a6ac8, blueLight = 0x5a8ae8, foam = 0xe8782e;
    fill(x + 34, y - 6, 7, 6, blue); fill(x + 34, y - 6, 7, 1, blueLight); fill(x + 35, y - 5, 5, 3, 0x2a2a30); dot(x + 36, y - 4, 0x8a8a92); dot(x + 38, y - 4, 0x8a8a92);
    fill(x + 34, y - 1, 7, 1, 0x2a4a9a);
    line(x + 41, y - 3, x + 43, y - 4, 0x2a2a30); ring(x + 43, y - 7, 2, 0x8a8a92); fill(x + 41, y - 6, 2, 2, foam); fill(x + 44, y - 6, 2, 2, foam);
    const t = (now + x * 0.17) % 12;
    if (t < 3) { const r = t / 3; dot(x + 44 + Math.round(Math.sin(r * 6) * 1), y - 9 - Math.round(r * 5), 0x2a2a30, 1 - r); dot(x + 45 + Math.round(Math.sin(r * 6) * 1), y - 10 - Math.round(r * 5), 0x2a2a30, 1 - r); }
  },
  /** A tall glass sundae from Scoops Ahoy: three scoops, a swirl of cream and a cherry, a striped spoon stuck in it; a drip runs down the glass now and then. */
  strangersundae(x, y) {
    const glass = 0xdff0f6, pink = 0xf2a0b8, choc = 0x7a4a2a, vanilla = 0xf6eedc, cream = 0xffffff;
    fill(x + 38, y - 1, 5, 1, glass, 0.7); fill(x + 40, y - 3, 1, 2, glass, 0.7); fill(x + 37, y - 7, 7, 4, glass, 0.5);
    fill(x + 37, y - 6, 2, 3, pink); fill(x + 39, y - 6, 3, 3, choc); fill(x + 42, y - 6, 2, 3, vanilla);
    fill(x + 38, y - 9, 5, 2, cream); dot(x + 40, y - 10, cream); dot(x + 40, y - 11, 0xd8302a);
    line(x + 42, y - 8, x + 44, y - 12, 0xd8302a); dot(x + 43, y - 10, WHITE);
    if ((now + x * 0.23) % 9 < 2) dot(x + 37, y - 3 + Math.floor((now * 2) % 2), pink);
  },
  /** A wooden baseball bat studded with nails, leaning on the desk's end; the nails catch the light now and then. */
  strangernailbat(x, y) {
    const wood = 0xb08050, woodDark = 0x8a5a32, nail = 0xb8bcc4;
    line(x + 35, y - 1, x + 44, y - 11, woodDark); line(x + 36, y - 1, x + 45, y - 11, wood); line(x + 37, y - 2, x + 45, y - 10, wood);
    fill(x + 34, y - 1, 3, 1, 0x3a2a20);
    const g = Math.floor((now + x * 0.1) * 1.5) % 9;
    [[41, -8], [43, -10], [44, -8], [42, -6], [45, -11], [40, -6]].forEach(([nx, ny], k) => dot(x + nx, y + ny, k === g ? WHITE : nail));
    fill(x + 34, y, 10, 1, SHADOW);
  },
  /** Three scaled dragon eggs on a bed of straw in a wooden bowl, one black-and-red, one cream-and-gold, one green-and-bronze; every 18 s one of them rocks and a crack of light shows on it. */
  goteggs(x, y) {
    const bowl = 0x6a4a2a, straw = 0xc8a858;
    fill(x + 33, y - 3, 13, 3, bowl); fill(x + 33, y - 3, 13, 1, 0x8a6440); for (let k = 0; k < 13; k += 2) dot(x + 33 + k, y - 4, straw);
    const t = (now + x * 0.27) % 18, who = Math.floor((now + x * 0.27) / 18) % 3, rock = t < 1.6;
    [[35, 0x2a2226, 0xa8281e], [39, 0xe8dcc0, 0xc8a050], [43, 0x3a6a4a, 0xa8784a]].forEach(([ex, c, s], k) => {
      const dx = rock && k === who ? (Math.floor(now * 8) % 2 ? 1 : -1) : 0;
      fill(x + ex - 1 + dx, y - 9, 3, 6, c); fill(x + ex - 2 + dx, y - 7, 5, 3, c); dot(x + ex - 1 + dx, y - 8, s); dot(x + ex + 1 + dx, y - 6, s); dot(x + ex + dx, y - 9, shaded(c, 1.3));
      if (rock && k === who) { dot(x + ex + dx, y - 7, 0xffd060); dot(x + ex + 1 + dx, y - 6, 0xffa030); if (isNight) glowLate(() => glowAt(x + ex, y - 7, 3, 3, 0xffa030, 0.3)); }
    });
  },
  /** A dented silver goblet of red wine beside a clay jug; every 15 s the goblet is lifted for a sip and set down again, a drop running down its side. */
  gotgoblet(x, y) {
    const silver = 0xb8bcc4, silverLight = 0xe0e4ea, silverDark = 0x7a7e86, wine = 0x7a1a2a, jug = 0x9a5a3a;
    fill(x + 34, y - 8, 5, 8, jug); fill(x + 35, y - 9, 3, 1, jug); fill(x + 34, y - 8, 1, 8, shaded(jug, 1.2)); dot(x + 33, y - 6, jug); dot(x + 33, y - 5, jug); fill(x + 34, y - 1, 5, 1, shaded(jug, 0.8));
    const t = (now + x * 0.21) % 15, up = t < 1.5 ? 2 : 0;
    const gx = x + 40, gy = y - up;
    fill(gx, gy - 8, 5, 4, silver); fill(gx, gy - 8, 5, 1, wine); fill(gx, gy - 8, 1, 4, silverLight); fill(gx + 4, gy - 8, 1, 4, silverDark);
    fill(gx + 2, gy - 4, 1, 3, silver); fill(gx + 1, gy - 1, 3, 1, silverDark);
    if (t > 1.5 && t < 4) dot(gx + 4, gy - 6 + Math.floor((t - 1.5) * 1.5), wine);
  },
  /** The catspaw dagger on a small stand: a slim blade of rippled grey Valyrian steel, a dragonbone hilt; the ripples shimmer along the blade now and then. */
  gotdagger(x, y) {
    const steel = 0x6a6e78, steelLight = 0xa8acb6, bone = 0xe8dcc0;
    fill(x + 34, y - 2, 11, 2, 0x2a2420); fill(x + 34, y - 2, 11, 1, 0x4a3a30);
    fill(x + 34, y - 5, 3, 2, bone); dot(x + 33, y - 4, 0xc8b890); fill(x + 37, y - 6, 1, 4, 0xc8a050);
    fill(x + 38, y - 5, 7, 2, steel); fill(x + 38, y - 5, 7, 1, steelLight); dot(x + 45, y - 4, steel);
    for (let k = 0; k < 7; k += 2) dot(x + 38 + k, y - 4, 0x4a4e58);
    const g = (now + x * 0.19) % 10; if (g < 1) dot(x + 38 + Math.floor(g * 7), y - 5, WHITE);
  },
  /** A little model of the Iron Throne on the desk, its jagged back of fused swords; every 20 s one of the blades catches the light. */
  gotthrone(x, y) {
    const iron = 0x5a5e66, ironLight = 0x8a8e96, ironDark = 0x3a3e46;
    fill(x + 35, y - 3, 9, 3, iron); fill(x + 35, y - 3, 9, 1, ironLight); fill(x + 37, y - 5, 5, 2, ironDark);
    for (const [bx, h] of [[35, 8], [36, 11], [37, 9], [38, 12], [39, 10], [40, 12], [41, 9], [42, 11], [43, 8]]) { fill(x + bx, y - 3 - h, 1, h - 2, iron); dot(x + bx, y - 3 - h, ironLight); }
    fill(x + 34, y - 6, 2, 1, ironLight); fill(x + 43, y - 6, 2, 1, ironLight);
    const t = (now + x * 0.23) % 20; if (t < 0.6) dot(x + 38, y - 14, WHITE);
  },
  /** The Hand of the King's gold badge, an open hand, lying on a folded red cloth; it glints now and then. */
  gotpin(x, y) {
    const red = 0x8a1e1e, gold = 0xe0b84a, goldDark = 0xa8842a, goldLight = 0xfff0a0;
    fill(x + 34, y - 2, 11, 2, red); fill(x + 34, y - 2, 11, 1, 0xa83030);
    fill(x + 37, y - 6, 5, 4, gold); for (let k = 0; k < 4; k++) fill(x + 37 + k + (k > 1 ? 0 : 0), y - 9, 1, 3, gold); dot(x + 42, y - 5, gold); dot(x + 43, y - 6, gold);
    fill(x + 37, y - 3, 5, 1, goldDark); dot(x + 37, y - 6, goldLight, (now + x * 0.2) % 6 < 0.4 ? 1 : 0.5);
  },
  /** A rolled message tied with string and sealed with black wax beside a black feather; every 22 s it unrolls a little, showing lines of writing, and rolls back. */
  gotscroll(x, y) {
    const parch = 0xeadcb4, parchDark = 0xc8b890, ink = 0x4a3a2a;
    const t = (now + x * 0.29) % 22, open = t < 3;
    if (open) { fill(x + 34, y - 4, 10, 4, parch); fill(x + 34, y - 4, 10, 1, 0xf6ecd0); for (const ly of [y - 3, y - 2]) fill(x + 35, ly, 7, 1, ink, 0.5); fill(x + 43, y - 5, 2, 5, parchDark); }
    else { fill(x + 35, y - 4, 9, 3, parch); fill(x + 35, y - 4, 9, 1, 0xf6ecd0); fill(x + 35, y - 2, 9, 1, parchDark); fill(x + 38, y - 4, 1, 3, 0x8a6a4a); dot(x + 39, y - 3, 0x1e1e22); dot(x + 40, y - 3, 0x1e1e22); }
    line(x + 33, y - 1, x + 37, y - 9, 0x1e1e26); line(x + 34, y - 1, x + 38, y - 8, 0x3a3a52);
  },
  /** A worn iron coin of the Faceless Men stamped with a face, standing on its edge in a little dish; every 18 s it spins, flashing, and falls flat, then stands again. */
  gotcoin(x, y) {
    const iron = 0x6a6e72, ironLight = 0x9a9ea2, ironDark = 0x4a4e52;
    fill(x + 35, y - 2, 9, 2, 0x3a3a42); fill(x + 35, y - 2, 9, 1, 0x5a5a62);
    const t = (now + x * 0.21) % 18;
    if (t < 1.5) { const w = [4, 2, 1, 2][Math.floor(t * 10) % 4]; fill(x + 39 - Math.floor(w / 2), y - 8, w, 5, iron); dot(x + 39, y - 7, ironLight); }
    else if (t < 3) { fill(x + 37, y - 3, 5, 1, iron); dot(x + 38, y - 3, ironLight); }
    else { disc(x + 39, y - 6, 2.5, iron); dot(x + 38, y - 7, ironDark); dot(x + 40, y - 7, ironDark); dot(x + 39, y - 5, ironDark); dot(x + 38, y - 8, ironLight); }
  },
  /** A few jagged shards of black dragonglass — obsidian — on a cloth, one bound as a dagger with leather; they glint a cold purple now and then. */
  gotdragonglass(x, y) {
    const glass = 0x1e1a24, sheen = 0x5a4a7a, cloth = 0x8a7a5a, leather = 0x6a4a2a;
    fill(x + 33, y - 2, 12, 2, cloth); fill(x + 33, y - 2, 12, 1, shaded(cloth, 1.2));
    for (const [sx, sy, w, h] of [[34, -5, 3, 3], [38, -4, 2, 2], [43, -5, 2, 3]]) { fill(x + sx, y + sy, w, h, glass); dot(x + sx, y + sy, sheen); }
    fill(x + 39, y - 9, 2, 5, glass); dot(x + 39, y - 10, glass); fill(x + 39, y - 5, 2, 2, leather); dot(x + 39, y - 9, sheen);
    if ((now + x * 0.23) % 7 < 0.5) dot(x + 40, y - 8, 0xc8a8f0);
  },
  /** A big pink-frosted donut with rainbow sprinkles on a little plate, a bite taken out of it; every 16 s another bite vanishes, then it's whole again. */
  simpsonsdonut(x, y) {
    const dough = 0xd89a5a, doughDark = 0xa86a3a, pink = 0xf28ab0, plate = 0xf2f2ee;
    fill(x + 33, y - 1, 12, 1, plate); fill(x + 34, y - 2, 10, 1, 0xdcdcd8);
    const bites = (now + x * 0.17) % 16 < 8 ? 1 : 2, w = 10 - bites;
    fill(x + 34, y - 5, w, 3, dough); fill(x + 35, y - 6, w - 2, 1, pink); fill(x + 34, y - 5, w, 1, pink); fill(x + 34, y - 3, w, 1, doughDark);
    dot(x + 34 + w, y - 3, dough); dot(x + 33 + w, y - 5, 0xf2ead8);
    fill(x + 38, y - 5, 2, 1, 0xc88a5a);
    for (const [dx, c] of [[35, 0x3a8ae8], [37, 0xf2e040], [41, 0x3ac85a], [36, 0xd8302a]]) if (dx < 34 + w) dot(x + dx, y - 5 - (dx % 2), c);
  },
  /** A red can of Duff beer with its white oval label; every 14 s the ring-pull pops and a fizz of bubbles spurts up. */
  simpsonsduffcan(x, y) {
    const red = 0xd8302a, redLight = 0xf05a4a, redDark = 0xa01e1a, silver = 0xb8bcc4;
    fill(x + 37, y - 9, 5, 9, red); fill(x + 37, y - 9, 1, 9, redLight); fill(x + 41, y - 9, 1, 9, redDark); fill(x + 37, y - 10, 5, 1, silver);
    fill(x + 38, y - 6, 3, 3, WHITE); dot(x + 39, y - 5, red);
    const t = (now + x * 0.21) % 14;
    if (t < 1.5) for (let k = 0; k < 3; k++) dot(x + 38 + k, y - 11 - Math.floor((t * 6 + k * 2) % 5), 0xf2f0d0, 0.8);
    fill(x + 36, y, 7, 1, SHADOW);
  },
  /** A glowing green rod of plutonium from the plant, lying on the desk where it shouldn't be; it pulses gently and glows green through the night. */
  simpsonsrod(x, y) {
    const green = 0x5ae84a, greenLight = 0xb0ff90, greenDark = 0x2a9a2a;
    const pulse = 0.7 + 0.3 * Math.sin(now * 2 + x);
    fill(x + 34, y - 3, 11, 3, green); fill(x + 34, y - 3, 11, 1, greenLight); fill(x + 34, y - 1, 11, 1, greenDark);
    fill(x + 33, y - 3, 1, 3, 0x8a8e96); fill(x + 45, y - 3, 1, 3, 0x8a8e96);
    glowAt(x + 39, y - 2, 7, 4, 0x7aff5a, 0.12 * pulse);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 2, 8, 5, 0x7aff5a, 0.3 * pulse); fill(x + 34, y - 3, 11, 1, greenLight); });
  },
  /** A tall Kwik-E-Mart Squishee cup of bright blue slush with a striped straw; every 12 s the slush level drops a little as it's slurped, and refills. */
  simpsonssquishee(x, y) {
    const cup = 0xf2f2ee, blue = 0x4ab0f0, red = 0xd8302a;
    fill(x + 37, y - 9, 6, 9, cup); fill(x + 37, y - 9, 1, 9, WHITE); fill(x + 42, y - 9, 1, 9, 0xd8d8d4);
    fill(x + 37, y - 6, 6, 2, red); fill(x + 37, y - 4, 6, 1, 0x3ac85a);
    const t = (now + x * 0.23) % 12, low = t < 3 ? Math.floor(t) : 0;
    fill(x + 37, y - 11 + low, 6, 2, blue); dot(x + 38, y - 11 + low, 0x9ad8ff);
    line(x + 41, y - 11, x + 43, y - 15, 0xf2f2ee); dot(x + 42, y - 13, red);
  },
  /** Lisa's gold saxophone propped on its stand; every 20 s a few jazzy notes float up from its bell. */
  simpsonssax(x, y) {
    const brass = 0xe8b830, brassLight = 0xfff070, brassDark = 0xa8841a;
    fill(x + 36, y - 1, 6, 1, 0x2a2a30); line(x + 37, y - 1, x + 38, y - 4, 0x2a2a30);
    fill(x + 38, y - 12, 2, 8, brass); fill(x + 38, y - 12, 1, 8, brassLight); dot(x + 37, y - 13, 0x2a2a30); dot(x + 38, y - 13, brassDark);
    fill(x + 39, y - 5, 3, 2, brass); fill(x + 41, y - 8, 3, 4, brass); fill(x + 42, y - 9, 2, 1, brassDark); dot(x + 41, y - 8, brassLight);
    for (const ky of [-10, -8, -6]) dot(x + 39, y + ky, brassDark);
    const t = (now + x * 0.19) % 20;
    if (t < 3) for (let k = 0; k < 2; k++) { const r = (t - k * 0.8) / 2; if (r > 0 && r < 1) { const nx = x + 43 + Math.round(r * 3), ny = y - 11 - Math.round(r * 6); dot(nx, ny, 0x2a2a30, 1 - r); dot(nx + 1, ny - 1, 0x2a2a30, 1 - r); dot(nx + 1, ny - 2, 0x2a2a30, 1 - r); } }
  },
  /** Bart's wooden slingshot lying on the desk with a little pile of pebbles; every 18 s its band twangs back and forth. */
  simpsonsslingshot(x, y) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, band = 0xd8302a;
    line(x + 37, y - 1, x + 39, y - 5, wood); line(x + 39, y - 5, x + 37, y - 9, woodLight); line(x + 39, y - 5, x + 42, y - 9, woodLight); dot(x + 39, y - 5, woodLight);
    const t = (now + x * 0.17) % 18, tw = t < 1 ? [2, -1, 1, 0][Math.floor(t * 8) % 4] : 0;
    line(x + 37, y - 9, x + 39 + tw, y - 8 + Math.abs(tw), band); line(x + 42, y - 9, x + 39 + tw, y - 8 + Math.abs(tw), band);
    for (const [px, py] of [[43, -1], [44, -1], [43, -2]]) dot(x + px, y + py, 0x8a8a8a);
  },
  /** A Homer bobblehead in his white shirt: round yellow head with two hairs and stubble; every 10 s his head bobs and he gives a cross little "D'oh" face. */
  simpsonsbobble(x, y) {
    const yellow = 0xf2d040, stubble = 0xc8a878, shirt = 0xf2f2ee;
    fill(x + 37, y - 1, 6, 1, 0x3a6ac8); fill(x + 38, y - 5, 4, 4, shirt); fill(x + 38, y - 5, 4, 1, 0xe0e0dc);
    const t = (now + x * 0.21) % 10, bob = t < 1.2 ? Math.round(Math.sin(t * 16)) : 0;
    const hx = x + 40 + bob;
    disc(hx, y - 9, 3.2, yellow); fill(hx - 2, y - 8, 5, 2, stubble);
    line(hx - 1, y - 13, hx - 1, y - 12, 0x2a2a30); line(hx + 1, y - 13, hx + 1, y - 12, 0x2a2a30);
    dot(hx - 1, y - 10, WHITE); dot(hx + 1, y - 10, WHITE); dot(hx - 1, y - 10, 0x2a2a30, 0.8);
    if (t < 1.2) fill(hx - 1, y - 7, 3, 1, 0x6a2a2a);
  },
  /** Maggie's red pacifier on the desk; every 8 s it sucks — squeak, squeak — the teat bobbing, little marks of sound beside it. */
  simpsonspacifier(x, y) {
    const red = 0xd8302a, redLight = 0xf05a4a, teat = 0xf2d8a0;
    const t = (now + x * 0.29) % 8, suck = t < 1.5 ? Math.floor(t * 6) % 2 : 0;
    ring(x + 39, y - 7, 2, red); fill(x + 37, y - 4, 5, 2, red); fill(x + 37, y - 4, 5, 1, redLight); fill(x + 38, y - 2 + suck, 3, 2 - suck, teat);
    if (t < 1.5) { dot(x + 43, y - 6 + suck, 0x2a2a30, 0.6); dot(x + 44, y - 7 + suck, 0x2a2a30, 0.6); }
  },
  /** A vodka martini in a cone glass with an olive on a pick, beside a silver shaker — shaken, not stirred: every 16 s the shaker rattles. */
  bondmartini(x, y) {
    const glass = 0xdff0f6, silver = 0xb8bcc4, silverLight = 0xe0e4ea;
    fill(x + 34, y - 1, 5, 1, glass, 0.8); fill(x + 36, y - 4, 1, 3, glass, 0.8);
    for (let r = 0; r < 4; r++) fill(x + 33 + r, y - 8 + r, 7 - r * 2, 1, r === 0 ? 0xc8e8f0 : glass, 0.7);
    line(x + 35, y - 9, x + 37, y - 6, 0x8a6a4a); dot(x + 36, y - 7, 0x6a8a2a);
    const sh = (now + x * 0.19) % 16 < 1.4 ? (Math.floor(now * 14) % 2 ? 1 : -1) : 0;
    fill(x + 41 + sh, y - 9, 4, 9, silver); fill(x + 41 + sh, y - 9, 1, 9, silverLight); fill(x + 42 + sh, y - 11, 2, 2, silver); dot(x + 42 + sh, y - 12, silverLight);
  },
  /** A Walther PPK pistol with its silencer laid beside it on a folded cloth; the steel catches the light now and then. */
  bondwalther(x, y) {
    const steel = 0x2a2a30, steelLight = 0x5a5a62, cloth = 0xe8e2d0;
    fill(x + 33, y - 2, 13, 2, cloth); fill(x + 33, y - 2, 13, 1, WHITE);
    fill(x + 35, y - 5, 8, 2, steel); fill(x + 35, y - 5, 8, 1, steelLight); fill(x + 35, y - 3, 3, 1, steel); dot(x + 38, y - 3, steelLight);
    fill(x + 40, y - 3, 5, 1, 0x3a3a42);
    if ((now + x * 0.23) % 8 < 0.4) dot(x + 41, y - 5, WHITE);
  },
  /** A steel diver's watch on a small cushion, its blue dial ticking; every 18 s a thin red laser beam shoots from its side and sweeps, then shuts off. */
  bondwatch(x, y) {
    const steel = 0xb8bcc4, dial = 0x1e3a6a, strap = 0x8a8e96;
    fill(x + 34, y - 2, 10, 2, 0x2a2a30); fill(x + 34, y - 2, 10, 1, 0x4a4a52);
    fill(x + 36, y - 4, 2, 2, strap); fill(x + 42, y - 4, 2, 2, strap);
    disc(x + 40, y - 6, 3, steel); disc(x + 40, y - 6, 2, dial);
    const s = (now % 60) / 60 * Math.PI * 2 - Math.PI / 2; dot(x + 40 + Math.round(Math.cos(s) * 2), y - 6 + Math.round(Math.sin(s) * 2), 0xf2d040); dot(x + 40, y - 6, WHITE);
    const t = (now + x * 0.29) % 18;
    if (t < 1.5) { const a = -0.4 + t * 0.4; line(x + 43, y - 6, x + 43 + Math.round(Math.cos(a) * 10), y - 6 - Math.round(Math.sin(a) * 6), 0xff2a2a); if (isNight) glowLate(() => dot(x + 43, y - 6, 0xff5a5a)); }
  },
  /** A hand of cards fanned face up beside towers of casino chips in red, black and gold; every 20 s the top chip flips over. */
  bondcards(x, y) {
    const W = 0xf6f4ee, red = 0xc8281e, black = 0x1e1e22, gold = 0xc8a050;
    for (let k = 0; k < 3; k++) { fill(x + 33 + k * 2, y - 6 + k, 4, 5, W); dot(x + 34 + k * 2, y - 5 + k, k === 1 ? black : red); fill(x + 33 + k * 2, y - 6 + k, 4, 1, 0xd8d4c8); }
    for (const [cx, n, c] of [[40, 4, red], [43, 3, black], [41, 2, gold]]) for (let k = 0; k < n; k++) { fill(x + cx, y - 1 - k, 3, 1, c); dot(x + cx + 1, y - 1 - k, W, 0.8); }
    if ((now + x * 0.17) % 20 < 0.6) { fill(x + 40, y - 6, 3, 1, red); dot(x + 41, y - 7, red); }
  },
  /** A gold-capped fountain pen lying on the desk; every 22 s someone clicks it — click, click, click — and a little red light at its end blinks, armed, then stops. */
  bondpen(x, y) {
    const body = 0x1e1e22, gold = 0xc8a050, goldLight = 0xe8c878;
    line(x + 34, y - 2, x + 44, y - 4, body); line(x + 34, y - 1, x + 44, y - 3, 0x3a3a42);
    fill(x + 42, y - 5, 3, 2, gold); dot(x + 42, y - 5, goldLight); dot(x + 34, y - 2, gold);
    const t = (now + x * 0.21) % 22;
    if (t < 3) { const on = Math.floor(t * 3) % 2; if (on) { dot(x + 45, y - 5, 0xff3a2a); if (isNight) glowLate(() => dot(x + 45, y - 5, 0xff5a3a)); } if (t < 1.5) dot(x + 46, y - 7 + Math.floor(t * 2), WHITE, 0.6); }
  },
  /** Q's black leather attaché case with brass catches; every 25 s it springs open, showing the gadget tray inside, and snaps shut. */
  bondbriefcase(x, y) {
    const leather = 0x1e1e24, leatherLight = 0x3a3a46, brass = 0xc8a050, felt = 0x8a1e2a;
    const t = (now + x * 0.27) % 25, open = t < 2.5;
    fill(x + 33, y - 6, 12, 6, leather); fill(x + 33, y - 6, 12, 1, leatherLight); fill(x + 37, y - 8, 4, 2, leather); fill(x + 38, y - 7, 2, 1, 0x0c0c10);
    dot(x + 35, y - 5, brass); dot(x + 42, y - 5, brass);
    if (open) { fill(x + 33, y - 12, 12, 6, leather); fill(x + 34, y - 11, 10, 4, felt); fill(x + 34, y - 6, 10, 1, felt); dot(x + 36, y - 6, 0xb8bcc4); fill(x + 39, y - 6, 3, 1, 0x3a3a42); dot(x + 43, y - 6, brass); }
    fill(x + 33, y, 12, 1, SHADOW);
  },
  /** A gleaming gold bullion bar stamped with a little seal; a glint runs along it now and then. */
  bondgoldbar(x, y) {
    const gold = 0xe8b830, goldLight = 0xfff0a0, goldDark = 0xa87a18;
    fill(x + 35, y - 3, 10, 3, gold); fill(x + 36, y - 5, 8, 2, goldLight); fill(x + 36, y - 5, 8, 1, 0xfff8d0); fill(x + 35, y - 1, 10, 1, goldDark); fill(x + 44, y - 3, 1, 3, goldDark);
    dot(x + 39, y - 4, goldDark); dot(x + 40, y - 4, goldDark);
    const g = (now + x * 0.19) % 7; if (g < 0.8) dot(x + 36 + Math.floor(g * 10), y - 5, WHITE);
    fill(x + 34, y, 12, 1, SHADOW);
  },
  /** A set of wind-up chattering teeth, but in steel, with a key in the side; every 14 s they chatter, clacking and shuffling across the desk. */
  bondteeth(x, y) {
    const gum = 0xc85a6a, steel = 0xc8ccd4, steelDark = 0x8a8e96;
    const t = (now + x * 0.23) % 14, chat = t < 2 ? Math.floor(now * 12) % 2 : 0, sh = t < 2 ? Math.floor(t * 2) : 0;
    const bx = x + 36 + sh;
    fill(bx, y - 2, 8, 2, gum); fill(bx, y - 3, 8, 1, steel); for (let k = 0; k < 8; k += 2) dot(bx + k, y - 3, steelDark);
    fill(bx, y - 6 - chat, 8, 2, gum); fill(bx, y - 4 - chat, 8, 1, steel); for (let k = 1; k < 8; k += 2) dot(bx + k, y - 4 - chat, steelDark);
    fill(bx + 8, y - 4, 2, 1, 0xc8a050); dot(bx + 9, y - 5, 0xc8a050);
  },
  /** A shallow dark bowl heaped with the spice melange, rust-orange powder that shimmers blue in places; now and then a few glittering motes rise from it, and it glows faintly at night. */
  dunespice(x, y) {
    const bowl = 0x3a2e24, spice = 0xd86a2a, spiceLight = 0xf09a4a, blue = 0x6ab0ff;
    fill(x + 34, y - 3, 11, 3, bowl); fill(x + 35, y, 9, 1, 0x2a2018); fill(x + 34, y - 3, 11, 1, 0x5a4636);
    fill(x + 35, y - 5, 9, 2, spice); fill(x + 36, y - 6, 7, 1, spice); fill(x + 37, y - 7, 5, 1, spiceLight); dot(x + 39, y - 8, spiceLight);
    const tw = Math.floor(now * 3 + x) % 5; dot(x + 36 + tw, y - 5, blue, 0.8);
    for (let k = 0; k < 3; k++) { const ph = (now * 0.4 + k / 3 + x * 0.01) % 1; dot(x + 37 + k * 2 + Math.round(Math.sin(ph * 6) * 1), y - 8 - Math.round(ph * 6), k % 2 ? blue : spiceLight, 0.7 * (1 - ph)); }
    if (isNight) glowLate(() => glowAt(x + 39, y - 6, 5, 3, 0xff8a40, 0.15));
  },
  /** A Fremen thumper planted in a little heap of sand: a stake with a spring-loaded hammer that pounds down every second or so, little rings of sand jumping round its foot. */
  dunethumper(x, y) {
    const steel = 0x6e727c, steelLight = 0xa8adb6, sand = 0xd8a868;
    fill(x + 34, y - 2, 11, 2, sand); fill(x + 35, y - 3, 9, 1, sand); fill(x + 34, y - 1, 11, 1, 0xc08a4e);
    fill(x + 39, y - 11, 2, 9, steel); fill(x + 39, y - 11, 1, 9, steelLight);
    const thump = Math.floor(now * 1.2 + x) % 2;
    fill(x + 38, y - 13 + thump * 3, 4, 2, 0x3a3a42); fill(x + 38, y - 13 + thump * 3, 4, 1, 0x5a5a62);
    if (thump) { dot(x + 36, y - 4, sand); dot(x + 43, y - 4, sand); dot(x + 35, y - 3, sand, 0.6); dot(x + 44, y - 3, sand, 0.6); }
  },
  /** The Reverend Mother's black box, a hole in its side, and the gom jabbar beside it: a fine poisoned needle on a little ring; the needle's tip glints now and then. */
  dunegombox(x, y) {
    const box = 0x1e1e24, boxLight = 0x3a3a46, needle = 0xc8ccd4;
    fill(x + 34, y - 7, 7, 7, box); fill(x + 34, y - 7, 7, 1, boxLight); fill(x + 34, y - 7, 1, 7, boxLight); fill(x + 41, y - 7, 1, 7, 0x0c0c10);
    fill(x + 35, y - 4, 3, 3, 0x050508);
    ring(x + 43, y - 2, 1, 0xc8a050); line(x + 44, y - 2, x + 45, y - 7, needle);
    if ((now + x * 0.27) % 7 < 0.4) dot(x + 45, y - 7, 0x9aff9a);
  },
  /** A Fremen literjon, a sealed water flask of stitched leather with a brass stopper, beside a small cup; a single bead of water forms at its stopper now and then — precious. */
  duneliterjon(x, y) {
    const leather = 0x8a5a32, leatherLight = 0xa8703e, leatherDark = 0x5a3a22, brass = 0xc8a050;
    fill(x + 35, y - 8, 6, 8, leather); fill(x + 36, y - 9, 4, 1, leather); fill(x + 35, y - 8, 1, 8, leatherLight); fill(x + 40, y - 8, 1, 8, leatherDark);
    for (let k = -7; k < 0; k += 2) dot(x + 38, y + k, leatherDark);
    fill(x + 37, y - 11, 2, 2, brass); dot(x + 37, y - 11, 0xe8c878);
    fill(x + 42, y - 3, 3, 3, 0x6a5a48); fill(x + 42, y - 3, 3, 1, 0x4a8ac8);
    const t = (now + x * 0.23) % 12; if (t < 2) dot(x + 39, y - 10 + Math.floor(t * 2), 0x8ac8ff);
  },
  /** A little date palm in a clay pot, its fronds fanning out with a cluster of orange dates; the fronds sway in a dry breeze now and then. */
  dunepalm(x, y) {
    const pot = 0xa86a3a, potDark = 0x7a4a24, trunk = 0x8a6a48, frond = 0x5a8a3a, frondLight = 0x7aaa4a;
    fill(x + 37, y - 4, 6, 4, pot); fill(x + 37, y - 4, 6, 1, 0xc8844a); fill(x + 38, y, 4, 1, potDark);
    fill(x + 39, y - 10, 2, 6, trunk); for (let k = -9; k < -4; k += 2) dot(x + 39, y + k, 0x6a4a30);
    const sw = (now + x * 0.21) % 9 < 1.5 ? Math.floor(now * 4) % 2 : 0;
    for (const [dx, dy] of [[-5, 2], [5, 2], [-4, -1], [4, -1], [-1, -3], [2, -3]]) line(x + 40, y - 11, x + 40 + dx + sw, y - 11 + dy, dx < 0 ? frond : frondLight);
    dot(x + 39, y - 10, 0xe8782e); dot(x + 41, y - 10, 0xe8782e); dot(x + 40, y - 9, 0xd8682a);
  },
  /** A Holtzman personal shield belt coiled on the desk, its emitter boxed in brass; every 14 s it switches on and a shimmering blue bubble flickers round it, then fades. */
  duneshield(x, y) {
    const belt = 0x3a3a42, brass = 0xc8a050;
    fill(x + 34, y - 2, 11, 2, belt); fill(x + 34, y - 2, 11, 1, 0x5a5a62); fill(x + 37, y - 4, 5, 3, brass); fill(x + 37, y - 4, 5, 1, 0xe8c878); dot(x + 39, y - 3, 0x6ab0ff);
    const t = (now + x * 0.29) % 14;
    if (t < 3) {
      const a = Math.min(1, t * 2, 3 - t) * (0.6 + 0.4 * (Math.floor(now * 12) % 2));
      ellipseRing(x + 39, y - 4, 7, 6, 0x8ac8ff, 0.6 * a); fill(x + 33, y - 9, 13, 9, 0x6ab0ff, 0.08 * a);
      if (isNight) glowLate(() => ellipseRing(x + 39, y - 4, 7, 6, 0xaad8ff, 0.5 * a));
    }
  },
  /** A tall hourglass of carved dark wood and glass, fine orange Arrakis sand trickling from top to bottom; every 30 s it's turned over. */
  dunesandtimer(x, y) {
    const wood = 0x3a2416, woodLight = 0x5a3a24, glass = 0xdff0f6, sand = 0xd8a060;
    const t = (now + x * 0.17) % 30, flip = t < 0.6, p = Math.min(1, t / 30);
    fill(x + 36, y - 1, 7, 1, wood); fill(x + 36, y - 13, 7, 1, wood); fill(x + 36, y - 13, 7, 1, woodLight);
    fill(x + 36, y - 12, 1, 11, wood); fill(x + 42, y - 12, 1, 11, wood);
    if (flip) { fill(x + 37, y - 12, 5, 11, glass, 0.4); return; }
    for (let r = 0; r < 5; r++) { fill(x + 37 + Math.floor(r / 2), y - 12 + r, 5 - Math.floor(r / 2) * 2, 1, glass, 0.45); fill(x + 37 + Math.floor(r / 2), y - 2 - r, 5 - Math.floor(r / 2) * 2, 1, glass, 0.45); }
    dot(x + 39, y - 7, glass, 0.6);
    const top = Math.round((1 - p) * 4), bottom = Math.round(p * 4);
    for (let r = 0; r < top; r++) fill(x + 38, y - 8 - r, 3, 1, sand);
    for (let r = 0; r < bottom; r++) fill(x + 38, y - 2 - r, 3, 1, sand);
    if (Math.floor(now * 6) % 2) dot(x + 39, y - 6, sand); else dot(x + 39, y - 5, sand);
  },
  /** A Fremen spice coffee service: a tall-spouted brass pot and two tiny cups on a round tray; steam curls from the spout now and then. */
  dunecoffee(x, y) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a;
    fill(x + 33, y - 1, 13, 1, brassDark); fill(x + 34, y - 2, 11, 1, brass);
    fill(x + 35, y - 8, 5, 6, brass); fill(x + 35, y - 8, 1, 6, brassLight); fill(x + 36, y - 10, 3, 2, brass); dot(x + 37, y - 11, brassLight);
    line(x + 40, y - 6, x + 42, y - 9, brass); line(x + 34, y - 7, x + 33, y - 4, brassDark);
    fill(x + 41, y - 4, 2, 2, 0xf2ead8); fill(x + 43, y - 4, 2, 2, 0xf2ead8); dot(x + 41, y - 4, 0x5a3a22); dot(x + 43, y - 4, 0x5a3a22);
    const t = (now + x * 0.23) % 9; if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 42 + (r % 2), y - 10 - r, WHITE, 0.45 - r * 0.1); }
  },
  /** The captain's straw hat with its red band, resting on the desk; now and then a gust lifts its brim a little and it settles again. */
  onepiecehat(x, y) {
    const straw = 0xf2d040, strawDark = 0xc8a830, band = 0xd8302a;
    const lift = (now + x * 0.23) % 12 < 0.8 ? Math.floor(now * 6) % 2 : 0;
    fill(x + 33, y - 2 - lift, 13, 2, straw); fill(x + 33, y - 1 - lift, 13, 1, strawDark);
    fill(x + 36, y - 6, 7, 4, straw); fill(x + 37, y - 7, 5, 1, straw); fill(x + 36, y - 3, 7, 1, band);
    for (let k = 0; k < 13; k += 3) dot(x + 33 + k, y - 2 - lift, strawDark);
    fill(x + 33, y, 13, 1, SHADOW);
  },
  /** A huge roasted haunch of meat on the bone, glistening, on a plate; every 14 s a big bite vanishes from it and it's whole again, steam rising. */
  onepiecemeat(x, y) {
    const meat = 0xa8502a, meatLight = 0xc86a3a, meatDark = 0x7a3418, bone = 0xf2ead8, plate = 0xf2f2ee;
    fill(x + 33, y - 1, 13, 1, plate); fill(x + 34, y - 2, 11, 1, 0xdcdcd8);
    const bite = (now + x * 0.19) % 14 < 5;
    fill(x + 35, y - 8, 7, 6, meat); fill(x + 36, y - 9, bite ? 3 : 5, 1, meat); fill(x + 35, y - 8, bite ? 4 : 7, 1, meatLight); fill(x + 35, y - 3, 7, 1, meatDark);
    if (bite) { fill(x + 39, y - 8, 3, 2, 0xe8a07a); dot(x + 40, y - 7, 0xf2c0a0); }
    dot(x + 37, y - 7, WHITE, 0.6);
    fill(x + 42, y - 6, 3, 2, bone); fill(x + 44, y - 7, 2, 1, bone); fill(x + 44, y - 4, 2, 1, bone);
    const t = (now + x * 0.21) % 8; if (t < 2.4) { const r = Math.floor(t * 1.5); dot(x + 37 + (r % 2), y - 11 - r, WHITE, 0.45 - r * 0.1); }
  },
  /** A Log Pose: a glass bubble on a leather wrist strap with a needle floating inside; the needle swings and points, then trembles now and then as it locks onto the next island. */
  onepiecelogpose(x, y) {
    const strap = 0x8a5a32, glass = 0xdff0f6;
    fill(x + 34, y - 2, 11, 2, strap); fill(x + 34, y - 2, 11, 1, 0xa8703e); dot(x + 36, y - 1, 0xc8a050);
    disc(x + 39, y - 6, 3.4, glass, 0.55); ring(x + 39, y - 6, 3.4, glass, 0.8); dot(x + 37, y - 8, WHITE, 0.8);
    const t = (now + x * 0.27) % 10, a = -Math.PI / 2 + (t < 1.5 ? Math.sin(t * 14) * 0.7 : 0.25);
    line(x + 39, y - 6, x + 39 + Math.round(Math.cos(a) * 2.5), y - 6 + Math.round(Math.sin(a) * 2.5), 0xd8302a); dot(x + 39, y - 6, 0x2a2a30);
  },
  /** A Devil Fruit: a round purple fruit covered in swirl patterns with a curly stem, on a small plate; it gives a strange little shimmer now and then. */
  onepiecedevilfruit(x, y) {
    const purple = 0x8a4ac8, purpleLight = 0xb07ae8, purpleDark = 0x5a2a8a, plate = 0xf2f2ee;
    fill(x + 34, y - 1, 11, 1, plate);
    disc(x + 39, y - 6, 4, purple); dot(x + 37, y - 8, purpleLight); dot(x + 38, y - 8, purpleLight);
    for (const [dx, dy] of [[-2, -1], [-1, -2], [0, -1], [1, 0], [2, -1], [1, 1], [-1, 1], [-2, 2], [2, 2]]) dot(x + 39 + dx, y - 6 + dy, purpleDark);
    line(x + 39, y - 10, x + 41, y - 12, 0x3a6a2a); dot(x + 42, y - 12, 0x3a6a2a); dot(x + 41, y - 11, 0x5a8a3a);
    if ((now + x * 0.21) % 9 < 0.8) { ring(x + 39, y - 6, 5, purpleLight, 0.5); if (isNight) glowLate(() => glowAt(x + 39, y - 6, 5, 5, 0xb07ae8, 0.2)); }
  },
  /** A little tangerine tree in a pot from the navigator's grove, a few bright tangerines among dark leaves; now and then one drops and rolls, then it's back on the branch. */
  onepiecetangerines(x, y) {
    const pot = 0xa86a3a, leaf = 0x3a7a3a, leafLight = 0x5a9a4a, orange = 0xf2902a;
    fill(x + 37, y - 4, 6, 4, pot); fill(x + 37, y - 4, 6, 1, 0xc8844a); fill(x + 39, y - 7, 2, 3, 0x6a4a30);
    disc(x + 40, y - 10, 4, leaf); dot(x + 38, y - 12, leafLight); dot(x + 42, y - 11, leafLight);
    const drop = (now + x * 0.23) % 15 < 2;
    for (const [ox, oy] of [[37, -10], [42, -9], [40, -12]]) dot(x + ox, y + oy, orange);
    if (drop) dot(x + 43 + Math.floor(((now + x * 0.23) % 15) * 1), y - 1, orange); else dot(x + 39, y - 8, orange);
  },
  /** The swordsman's sake: a big round earthenware jug with a red label and a rope round its neck, a shallow red cup beside it; the cup is filled again now and then. */
  onepiecesake(x, y) {
    const clay = 0xd8c8a8, clayDark = 0xa8987a, red = 0xc8281e;
    fill(x + 34, y - 9, 7, 9, clay); fill(x + 35, y - 10, 5, 1, clay); fill(x + 36, y - 12, 3, 2, clay); fill(x + 34, y - 9, 1, 9, 0xf0e4c8); fill(x + 40, y - 9, 1, 9, clayDark);
    fill(x + 35, y - 7, 5, 4, red); fill(x + 36, y - 6, 3, 2, WHITE, 0.8); fill(x + 36, y - 11, 3, 1, 0xa8885a);
    fill(x + 42, y - 2, 4, 2, red); fill(x + 42, y - 2, 4, 1, (now + x * 0.21) % 12 < 6 ? 0xf2ead8 : 0x8a1e1e);
  },
  /** A bottle of cola, the cyborg shipwright's fuel, beside a little star-shaped sunglasses; every 15 s the bottle fizzes up, bubbles rising. */
  onepiececola(x, y) {
    const glass = 0x3a2418, label = 0xd8302a, cap = 0xb8bcc4;
    fill(x + 36, y - 9, 4, 9, glass); fill(x + 37, y - 12, 2, 3, glass); fill(x + 37, y - 13, 2, 1, cap); fill(x + 36, y - 6, 4, 2, label); dot(x + 37, y - 5, WHITE); dot(x + 36, y - 9, 0x6a4a3a);
    fill(x + 41, y - 3, 2, 2, 0x3a8ae8); fill(x + 44, y - 3, 2, 2, 0x3a8ae8); dot(x + 43, y - 3, 0x2a2a30);
    const t = (now + x * 0.19) % 15; if (t < 1.5) for (let k = 0; k < 3; k++) dot(x + 37 + (k % 2), y - 14 - Math.floor((t * 6 + k * 2) % 5), 0xe8d8b8, 0.8);
  },
  /** A small block of a Poneglyph: a cube of deep red stone carved with rows of ancient glyphs; now and then the glyphs glow faintly. */
  onepieceponeglyph(x, y) {
    const stone = 0x8a2a2a, stoneLight = 0xa83a3a, stoneDark = 0x5a1a1a, glyph = 0x3a0e0e;
    fill(x + 35, y - 9, 9, 9, stone); fill(x + 35, y - 9, 9, 1, stoneLight); fill(x + 35, y - 9, 1, 9, stoneLight); fill(x + 43, y - 9, 1, 9, stoneDark);
    const glow = (now + x * 0.23) % 14 < 2;
    for (let r = 0; r < 3; r++) for (let k = 0; k < 3; k++) { const gx = x + 37 + k * 2, gy = y - 7 + r * 2; dot(gx, gy, glow ? 0xf2d080 : glyph); if (k % 2) dot(gx + 1, gy, glow ? 0xf2d080 : glyph, 0.6); }
    if (glow && isNight) glowLate(() => glowAt(x + 39, y - 5, 5, 5, 0xf2d080, 0.18));
    fill(x + 35, y, 9, 1, SHADOW);
  },
  /** A golden rune, a glowing gold sigil-fragment hovering just over the desk, turning slowly and shedding little golden motes; it glows warm at night. */
  eldenringrune(x, y) {
    const gold = 0xe8c050, goldLight = 0xfff0a0;
    const bob = Math.round(Math.sin(now * 1.5 + x) * 1), cx = x + 39, cy = y - 7 + bob, flip = Math.floor(now * 1.2 + x) % 4;
    const w = [3, 2, 1, 2][flip];
    fill(cx - Math.floor(w / 2), cy - 3, w, 6, gold); dot(cx, cy - 2, goldLight); dot(cx - 1, cy, gold); dot(cx + 1, cy, gold); dot(cx, cy + 3, gold);
    for (let k = 0; k < 2; k++) { const ph = (now * 0.6 + k / 2) % 1; dot(cx + (k ? 2 : -2), cy + 2 - Math.round(ph * 6), goldLight, 1 - ph); }
    glowAt(cx, cy, 4, 4, 0xf2d070, 0.12); fill(x + 36, y, 7, 1, SHADOW);
    if (isNight) glowLate(() => { glowAt(cx, cy, 5, 5, 0xf2d070, 0.3); dot(cx, cy - 2, WHITE); });
  },
  /** A Flask of Crimson Tears and a Flask of Cerulean Tears side by side in a little rack, red and blue liquid shimmering in their round glass; now and then one sloshes. They glow faintly at night. */
  eldenringflasks(x, y) {
    const glass = 0xdff0f6, red = 0xc8281e, blue = 0x2a6ae8, wood = 0x5a3a24;
    fill(x + 34, y - 2, 11, 2, wood); fill(x + 34, y - 2, 11, 1, 0x7a5232);
    for (const [fx, c] of [[37, red], [42, blue]]) {
      disc(fx + x, y - 6, 2.8, glass, 0.5); fill(x + fx - 1, y - 6, 3, 3, c); fill(x + fx - 2, y - 5, 5, 2, c); fill(x + fx - 1, y - 10, 2, 2, glass, 0.6); dot(x + fx, y - 11, 0x8a6a4a);
      const s = (now + fx + x * 0.2) % 8 < 0.6 ? 1 : 0; dot(x + fx - 1 + s, y - 6, shaded(c, 1.5)); dot(x + fx - 2, y - 7, WHITE, 0.7);
    }
    if (isNight) glowLate(() => { glowAt(x + 37, y - 5, 3, 3, 0xe83a3a, 0.2); glowAt(x + 42, y - 5, 3, 3, 0x4a8aff, 0.2); });
  },
  /** A tiny site of grace on the desk: a wisp of golden light rising from a pinch of earth, little sparkles twirling up round it; it brightens now and then, and glows through the night. */
  eldenringgrace(x, y) {
    const gold = 0xf2c850, goldLight = 0xfff0a0;
    fill(x + 36, y - 1, 7, 1, 0x5a4a32); fill(x + 37, y - 2, 5, 1, 0x6a5a3a);
    const b = (now + x * 0.2) % 8 < 1.5 ? 1 : 0.7;
    for (let k = 0; k < 5; k++) { const ph = (now * 0.8 + k / 5) % 1; dot(x + 39 + Math.round(Math.sin(ph * 8 + k) * 2), y - 3 - Math.round(ph * 9), k % 2 ? gold : goldLight, b * (1 - ph)); }
    fill(x + 39, y - 4, 1, 2, goldLight, b);
    glowAt(x + 39, y - 5, 4, 5, 0xf2c850, 0.1 * b);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 5, 5, 6, 0xf2c850, 0.3 * b); fill(x + 39, y - 4, 1, 2, WHITE); });
  },
  /** A Stonesword Key, a little stone sword-shaped key with a curled grip, lying on a square of grey stone carved with an imp's face; the imp's eyes glint now and then. */
  eldenringkey(x, y) {
    const stone = 0x8a8a84, stoneDark = 0x5a5a56, key = 0xb8b0a0, keyDark = 0x8a8478;
    fill(x + 34, y - 4, 7, 4, stone); fill(x + 34, y - 4, 7, 1, 0xa8a8a2); fill(x + 35, y - 3, 1, 1, stoneDark); fill(x + 38, y - 3, 1, 1, stoneDark); fill(x + 36, y - 2, 2, 1, stoneDark);
    if ((now + x * 0.21) % 10 < 0.4) { dot(x + 35, y - 3, 0xf2c850); dot(x + 38, y - 3, 0xf2c850); }
    line(x + 40, y - 1, x + 45, y - 6, key); line(x + 41, y - 1, x + 45, y - 5, keyDark); fill(x + 39, y - 2, 2, 2, keyDark); dot(x + 38, y - 1, key);
  },
  /** A small bronze spirit calling bell on a carved stand; every 20 s it rings, swinging, and a faint blue spirit shimmers up beside it and fades. */
  eldenringbell(x, y) {
    const bronze = 0xa8784a, bronzeLight = 0xc8945a, wood = 0x3a2a1c;
    fill(x + 35, y - 1, 7, 1, wood); fill(x + 35, y - 11, 1, 10, wood); fill(x + 35, y - 11, 6, 1, wood);
    const t = (now + x * 0.23) % 20, sw = t < 1.5 ? [0, 1, 0, -1][Math.floor(t * 8) % 4] : 0;
    dot(x + 40, y - 10, wood); fill(x + 38 + sw, y - 9, 4, 4, bronze); fill(x + 37 + sw, y - 6, 6, 1, bronzeLight); dot(x + 38 + sw, y - 9, bronzeLight); dot(x + 40 + sw, y - 5, 0x5a3a24);
    if (t < 3) { const a = Math.min(1, t, 3 - t) * 0.6; fill(x + 43, y - 9, 2, 6, 0x8ac8ff, a); dot(x + 43, y - 10, 0xc8e8ff, a); if (isNight) glowLate(() => glowAt(x + 44, y - 6, 3, 4, 0x8ac8ff, 0.3 * a)); }
  },
  /** A message scrawled in glowing orange on the desk, left by another traveller; every 12 s it flares brighter, as if someone appraised it. It glows at night. */
  eldenringsoapstone(x, y) {
    const orange = 0xe8782e, light = 0xffb060;
    const b = (now + x * 0.19) % 12 < 1.5 ? 1 : 0.55;
    for (const [mx, my, w] of [[34, -2, 5], [40, -2, 4], [35, -4, 3], [39, -4, 5]]) fill(x + mx, y + my, w, 1, orange, b);
    dot(x + 34, y - 4, light, b); dot(x + 44, y - 2, light, b);
    glowAt(x + 39, y - 3, 6, 2, 0xff9a40, 0.08 * b);
    if (isNight) glowLate(() => { glowAt(x + 39, y - 3, 7, 3, 0xff9a40, 0.25 * b); fill(x + 34, y - 2, 5, 1, light, 0.6 * b); });
  },
  /** A little warrior made of a clay jar, sturdy and cheerful, with a rim like a helmet and two dark eye-holes; every 15 s he rocks from side to side and gives a hearty little hop. */
  eldenringjar(x, y) {
    const clay = 0xb8784a, clayLight = 0xd89464, clayDark = 0x8a5434;
    const t = (now + x * 0.29) % 15, rock = t < 1.6 ? (Math.floor(now * 6) % 2 ? 1 : -1) : 0, hop = t > 1.2 && t < 1.6 ? 2 : 0;
    const jx = x + 36 + rock, jy = y - hop;
    fill(jx + 1, jy - 9, 7, 9, clay); fill(jx, jy - 7, 9, 5, clay); fill(jx + 2, jy - 10, 5, 1, clayDark); fill(jx + 1, jy - 11, 7, 1, clay);
    fill(jx + 1, jy - 9, 1, 8, clayLight); fill(jx + 7, jy - 8, 1, 7, clayDark); fill(jx + 2, jy - 1, 5, 1, clayDark);
    dot(jx + 3, jy - 7, 0x2a1a12); dot(jx + 5, jy - 7, 0x2a1a12);
    fill(x + 36, y, 9, 1, SHADOW);
  },
  /** A traveller's iron lantern on the desk, a warm flame inside its glass that flickers, lighting the desk round it at night. */
  eldenringlantern(x, y) {
    const iron = 0x2a2a30, ironLight = 0x4a4a52;
    fill(x + 37, y - 1, 6, 1, iron); fill(x + 37, y - 8, 6, 1, iron); fill(x + 37, y - 8, 1, 7, iron); fill(x + 42, y - 8, 1, 7, iron); fill(x + 38, y - 9, 4, 1, ironLight); ring(x + 40, y - 11, 1.2, iron);
    fill(x + 38, y - 7, 4, 6, 0xf2e2a0, 0.5);
    const f = candlelight(x * 0.17); dot(x + 40, y - 4, 0xffd36b, f); dot(x + 40, y - 5, 0xff8a3d, 0.6 * f);
    glowAt(x + 40, y - 4, 4, 4, 0xffc060, 0.08 * f);
    if (isNight) glowLate(() => { glowAt(x + 40, y - 4, 8, 6, 0xffc060, 0.25 * f); dot(x + 40, y - 4, 0xfff4c0); });
  },
  /** The Lumon egg: a smooth white ovoid on a little black disc, a faint seam down it. */
  sevegg(x, y) {
    const egg = 0xeeece4, eggLight = 0xffffff, eggShade = 0xcfccc0, base = 0x1c1e22;
    fill(x + 37, y - 1, 7, 1, base); fill(x + 38, y - 2, 5, 1, 0x2e3036); // the dark disc base
    fill(x + 39, y - 6, 3, 4, egg); fill(x + 38, y - 5, 5, 3, egg); fill(x + 39, y - 8, 3, 2, egg); // the smooth egg
    dot(x + 39, y - 7, eggLight); fill(x + 42, y - 6, 1, 3, eggShade); dot(x + 40, y - 5, eggShade, 0.6); // highlight, shade, seam
  },
  /** The Compliance Handbook: a slim teal hardback standing on end, gilt title lines on the spine. */
  sevhandbook(x, y) {
    const cover = 0x2f6456, coverLight = 0x3d7d6e, coverDark = 0x214f45, gold = 0xd6b25e, page = 0xeae6dc;
    fill(x + 38, y - 9, 5, 9, cover); fill(x + 38, y - 9, 1, 9, coverLight); fill(x + 42, y - 9, 1, 9, coverDark); fill(x + 43, y - 9, 1, 9, page); // the standing book
    fill(x + 39, y - 7, 3, 1, gold); fill(x + 39, y - 5, 2, 1, gold); dot(x + 38, y - 9, WHITE, 0.4); // gilt title lines
  },
  /** A woven bamboo finger trap, red and yellow weave, lying on the desk; a little MDR reward. */
  sevfingertrap(x, y) {
    const a = 0xd8402a, b = 0xe8c84a, straw = 0xcdb98a;
    fill(x + 36, y - 2, 9, 3, straw); fill(x + 36, y - 2, 9, 1, 0xe0d0a6); fill(x + 36, y, 9, 1, 0xa8956a); // the woven tube
    for (let i = 0; i < 9; i += 2) fill(x + 36 + i, y - 2, 1, 3, i % 4 ? a : b, 0.8); // its diagonal weave
    dot(x + 36, y - 1, 0x1a1a1a, 0.5); dot(x + 44, y - 1, 0x1a1a1a, 0.5); // the open ends
  },
  /** A waffle on a small plate with a dab of cream and a berry: the waffle party. */
  sevwaffle(x, y) {
    const plate = 0xe6e9e4, waffle = 0xd8a85a, waffleDark = 0xb5863f, cream = 0xf2ead6, berry = 0xd8402a;
    fill(x + 36, y - 1, 9, 1, 0xc4cac3); fill(x + 37, y - 2, 7, 1, plate); // the plate
    fill(x + 38, y - 5, 5, 3, waffle); fill(x + 38, y - 5, 5, 1, 0xe8bc70); // the waffle
    for (let gx = 0; gx < 2; gx++) dot(x + 39 + gx * 2, y - 4, waffleDark); // the grid
    dot(x + 40, y - 6, cream); dot(x + 41, y - 4, berry); // cream and a berry
  },
  /** A plain white break-room mug with a teal band, a little coffee in it. */
  sevcoffee(x, y) {
    const mug = 0xeef1ec, mugShade = 0xcfd4cd, teal = 0x2f6456, coffee = 0x3a2a1e;
    fill(x + 38, y - 6, 5, 6, mug); fill(x + 38, y - 6, 1, 6, WHITE, 0.5); fill(x + 42, y - 6, 1, 6, mugShade); // the mug
    fill(x + 38, y - 6, 5, 1, coffee); dot(x + 39, y - 6, 0x5a4330); // the coffee
    fill(x + 38, y - 4, 5, 1, teal); // the teal band
    fill(x + 43, y - 5, 1, 3, mugShade); dot(x + 44, y - 5, mug); dot(x + 44, y - 3, mug); // the handle
  },
  /** A wedge of honeydew melon, the Nurturable reward; pale green flesh and a dark rind. */
  sevmelon(x, y) {
    const rind = 0x3d7d6e, rindLight = 0x5a9a86, flesh = 0x9ed98a, fleshLight = 0xbfe8a8, seed = 0xe8e4c0;
    fill(x + 37, y, 8, 1, SHADOW); // under it
    fill(x + 37, y - 4, 8, 3, flesh); fill(x + 37, y - 4, 8, 1, fleshLight); // the melon flesh
    fill(x + 37, y - 1, 8, 1, rind); fill(x + 37, y - 2, 8, 1, rindLight); // the rind
    dot(x + 39, y - 3, seed); dot(x + 42, y - 3, seed); // a couple of seeds
  },
  /** A small grey desk fan in a round cage, its blades turning slowly. */
  sevfan(x, y) {
    const base = 0x9aa0a2, cage = 0xbfc4c5, cageDark = 0x6a6e70, blade = 0xdfe3e2;
    fill(x + 39, y - 1, 3, 1, cageDark); fill(x + 40, y - 4, 1, 3, base); // the stand
    disc(x + 41, y - 7, 4, cageDark); disc(x + 41, y - 7, 3.2, cage); // the round cage
    const a = now * 2; for (let k = 0; k < 3; k++) { const an = a + k * 2.1; line(x + 41, y - 7, x + 41 + Math.round(Math.cos(an) * 2.4), y - 7 + Math.round(Math.sin(an) * 2.4), blade); } // the blades
    dot(x + 41, y - 7, 0x5a5e60); ring(x + 41, y - 7, 3.4, cageDark, 0.6); // the hub and cage ring
  },
  /** A cream pen cup with a few Lumon-blue and teal pens poking out. */
  sevpencup(x, y) {
    const cup = 0xcdc6b0, cupLight = 0xe0dac6, cupDark = 0x9a9480, blue = 0x3a5b8c, teal = 0x2f6456, yellow = 0xe8c84a;
    fill(x + 38, y - 5, 6, 5, cup); fill(x + 38, y - 5, 1, 5, cupLight); fill(x + 43, y - 5, 1, 5, cupDark); // the cup
    fill(x + 39, y - 9, 1, 4, blue); fill(x + 40, y - 10, 1, 5, teal); fill(x + 42, y - 8, 1, 3, yellow); // pens
    dot(x + 40, y - 11, teal); dot(x + 39, y - 10, blue); // their tips
  },
  /** A desk bobblehead in a mustard shirt and glasses, its big head bobbing. */
  offbobblehead(x, y) {
    const base = 0x2a2a30, skin = 0xe3bb92, hair = 0x6a4a2a, shirt = 0xd0a83a, glasses = 0x3a3d42;
    const bob = Math.round(Math.sin(now * 3 + x) * 1);
    fill(x + 38, y - 2, 6, 2, base); fill(x + 38, y - 2, 6, 1, 0x4a4a50); // the base
    fill(x + 39, y - 5, 4, 3, shirt); fill(x + 39, y - 5, 4, 1, 0xe8c460); // the little body
    fill(x + 39, y - 10 + bob, 4, 5, skin); fill(x + 39, y - 11 + bob, 4, 1, hair); fill(x + 39, y - 7 + bob, 4, 1, glasses); // the bobbling head
    dot(x + 40, y - 8 + bob, PALETTE.eye); dot(x + 42, y - 8 + bob, PALETTE.eye);
  },
  /** A white "World's Best Boss" mug with printed words and a little coffee. */
  offbossmug(x, y) {
    const mug = 0xf2f0ea, mugShade = 0xd4d2c8, coffee = 0x3a2a1e, word = 0x2a2a30;
    fill(x + 38, y - 6, 5, 6, mug); fill(x + 38, y - 6, 1, 6, WHITE, 0.6); fill(x + 42, y - 6, 1, 6, mugShade); // the mug
    fill(x + 38, y - 6, 5, 1, coffee); fill(x + 39, y - 4, 3, 1, word); fill(x + 39, y - 3, 3, 1, word); // coffee and words
    fill(x + 43, y - 5, 1, 3, mugShade); dot(x + 44, y - 5, mug); dot(x + 44, y - 3, mug); // the handle
  },
  /** A stapler suspended in a block of orange Jell-O on a plate: a classic prank. */
  offjello(x, y) {
    const jello = 0xe88a2a, jelloLight = 0xf2a84a, stapler = 0x3a3d42, staplerLight = 0x5a5e64;
    fill(x + 36, y - 1, 9, 1, 0xc4cac3); // the plate
    fill(x + 37, y - 5, 7, 4, jello, 0.78); fill(x + 37, y - 5, 7, 1, jelloLight, 0.8); dot(x + 39, y - 4, WHITE, 0.4); // the wobbly jello
    fill(x + 39, y - 7, 4, 2, stapler); fill(x + 39, y - 7, 4, 1, staplerLight); dot(x + 39, y - 7, WHITE, 0.4); // the suspended stapler
  },
  /** A beige multi-line office phone with a keypad and a coiled cord. */
  offphone(x, y) {
    const base = 0xcac3ad, baseLight = 0xdcd6c2, baseDark = 0x9a9276, btn = 0x7a7e82, cord = 0x6a6e72;
    fill(x + 36, y - 3, 9, 3, base); fill(x + 36, y - 3, 9, 1, baseLight); fill(x + 36, y - 1, 9, 1, baseDark); // the body
    for (let r = 0; r < 2; r++) for (let c = 0; c < 3; c++) dot(x + 38 + c * 2, y - 2 + r, btn); // the keypad
    fill(x + 36, y - 5, 2, 2, base); fill(x + 43, y - 5, 2, 2, base); fill(x + 36, y - 5, 8, 1, baseLight); // the cradled handset
    line(x + 44, y - 4, x + 45, y - 1, cord); line(x + 45, y - 1, x + 44, y, cord); // the coiled cord
  },
  /** A small grey teapot with a lid knob and a curved spout. */
  offteapot(x, y) {
    const pot = 0x9aa0a4, potLight = 0xc0c4c6, potDark = 0x6a6e72, lid = 0x7a7e82;
    fill(x + 38, y - 5, 6, 5, pot); fill(x + 38, y - 5, 6, 1, potLight); fill(x + 38, y - 1, 6, 1, potDark); // the body
    fill(x + 39, y - 7, 3, 2, pot); dot(x + 40, y - 8, lid); // the lid and knob
    line(x + 44, y - 4, x + 46, y - 3, pot); dot(x + 46, y - 3, potLight); // the spout
    fill(x + 37, y - 4, 1, 3, pot); dot(x + 36, y - 3, pot); dot(x + 39, y - 4, WHITE, 0.5); // the handle and a glint
  },
  /** An open box of assorted donuts: pink, chocolate and glazed. */
  offdonuts(x, y) {
    const box = 0xd8b48a, boxLight = 0xe8c89a, boxDark = 0xa8865a, d1 = 0xe8a0c0, d2 = 0x7a4a2a, glaze = 0xf2d84a;
    fill(x + 36, y - 3, 9, 3, box); fill(x + 36, y - 3, 9, 1, boxLight); fill(x + 36, y - 1, 9, 1, boxDark); // the box
    disc(x + 38, y - 4, 1.6, d1); dot(x + 38, y - 4, 0xc87090); // a pink donut
    disc(x + 41, y - 4, 1.6, d2); dot(x + 41, y - 4, 0x5a3418); // a chocolate donut
    disc(x + 43, y - 4, 1.6, glaze); dot(x + 43, y - 4, 0xd8b830); // a glazed donut
  },
  /** A ream of Dunder Mifflin paper with a blue-striped label. */
  offpaperstack(x, y) {
    const paper = 0xf2efe6, paperShade = 0xd8d4c6, band = 0x2a5a8a, label = 0xeef0f4;
    fill(x + 37, y - 6, 8, 6, paper); fill(x + 37, y - 6, 8, 1, WHITE, 0.6); fill(x + 37, y - 1, 8, 1, paperShade); fill(x + 44, y - 6, 1, 6, paperShade); // the ream
    for (let i = 1; i < 6; i++) fill(x + 37, y - 6 + i, 8, 1, i % 2 ? paper : 0xeae6da); // sheet lines
    fill(x + 38, y - 4, 6, 2, label); fill(x + 39, y - 4, 4, 1, band); // the label
  },
  /** A big soft pretzel, salted, on the desk: Pretzel Day. */
  offpretzel(x, y) {
    const dough = 0xb57a34, doughLight = 0xd0954a, salt = 0xf2efe6;
    fill(x + 37, y - 1, 8, 1, SHADOW);
    ring(x + 39, y - 4, 2.4, dough); ring(x + 42, y - 4, 2.4, dough); // the two loops
    fill(x + 39, y - 3, 4, 2, dough); fill(x + 38, y - 2, 6, 1, dough); // the crossed middle
    dot(x + 38, y - 5, doughLight); dot(x + 42, y - 5, doughLight); // shine
    dot(x + 39, y - 4, salt); dot(x + 42, y - 3, salt); dot(x + 40, y - 2, salt); // salt grains
  },
  /** A cut-glass tumbler of amber whisky with a bright glint. */
  peakywhisky(x, y) {
    const glass = 0xdfe6e4, amber = 0xc87a2a, amberLight = 0xe89a4a;
    fill(x + 39, y - 5, 5, 5, glass, 0.45); fill(x + 39, y - 5, 1, 5, WHITE, 0.4); fill(x + 43, y - 5, 1, 5, 0xa8b0ae, 0.5); // the tumbler
    fill(x + 39, y - 2, 5, 2, amber, 0.85); fill(x + 39, y - 2, 5, 1, amberLight, 0.8); // the whisky
    fill(x + 39, y, 5, 1, 0xb0b6b4, 0.6); dot(x + 40, y - 4, WHITE, 0.6); // base and glint
  },
  /** A lit cigarette resting in a steel ashtray, a thin wisp of smoke rising. */
  peakyashtray(x, y) {
    const tray = 0x6a6e70, trayLight = 0x8a8e90, cig = 0xe8e4d8, ember = 0xe8782e, ash = 0x9a9488;
    fill(x + 37, y - 1, 8, 1, tray); fill(x + 37, y - 2, 8, 1, trayLight); fill(x + 38, y - 1, 6, 1, 0x4a4e50); // the ashtray
    fill(x + 39, y - 3, 6, 1, cig); fill(x + 44, y - 3, 1, 1, ember); dot(x + 40, y - 3, ash); // the cigarette
    const w = Math.floor(now * 2) % 3; dot(x + 45 - w, y - 5 - w, 0xbfc4c0, 0.4); dot(x + 45, y - 7, 0xbfc4c0, 0.25); // a smoke wisp
  },
  /** A stack of banknotes with a couple of gold coins on top. */
  peakycash(x, y) {
    const note = 0x6a8a5a, noteLight = 0x86a66e, noteDark = 0x4a6a3e, coin = 0xc8a04a, coinLight = 0xe8c878;
    fill(x + 37, y - 4, 8, 4, note); fill(x + 37, y - 4, 8, 1, noteLight); fill(x + 37, y - 1, 8, 1, noteDark); // the stack
    for (let i = 1; i < 4; i++) fill(x + 37, y - 4 + i, 8, 1, i % 2 ? note : 0x5a7a4e); // note edges
    disc(x + 38, y - 4, 1.2, coin); dot(x + 38, y - 5, coinLight); disc(x + 41, y - 4, 1.2, coin); dot(x + 41, y - 5, coinLight); // gold coins
  },
  /** An open red betting ledger with ruled entries down its pages. */
  peakyledger(x, y) {
    const cover = 0x6a2a2a, page = 0xe8dcc0, pageShade = 0xcbbf9e, ink = 0x3a2a1a;
    fill(x + 36, y - 3, 9, 3, cover); fill(x + 36, y - 3, 9, 1, 0x8a3a3a); // the red cover
    fill(x + 37, y - 4, 7, 2, page); fill(x + 37, y - 4, 3, 2, pageShade); fill(x + 40, y - 4, 1, 2, 0x8a7a5a); // the open pages
    fill(x + 38, y - 4, 2, 1, ink, 0.6); fill(x + 41, y - 4, 2, 1, ink, 0.6); fill(x + 38, y - 3, 2, 1, ink, 0.6); // ruled entries
  },
  /** A gold pocket watch lying open, its chain trailing off the desk. */
  peakywatch(x, y) {
    const gold = 0xc8a04a, goldLight = 0xe8c878, goldDark = 0x8a6a2a, face = 0xe8dcc0;
    disc(x + 41, y - 3, 3, goldDark); disc(x + 41, y - 3, 2.3, gold); disc(x + 41, y - 3, 1.6, face); // the case
    dot(x + 41, y - 3, goldDark); line(x + 41, y - 3, x + 42, y - 4, 0x3a2a1a); // a hand
    fill(x + 41, y - 6, 1, 1, goldLight); dot(x + 41, y - 7, gold); // the crown
    line(x + 40, y - 6, x + 37, y - 2, gold); dot(x + 37, y - 2, goldLight); // the chain
  },
  /** A pint of ale with a foam head and a bright glint. */
  peakypint(x, y) {
    const glass = 0xdfe6e4, ale = 0xc88a2a, aleLight = 0xe8aa4a, foam = 0xf2ecd8;
    fill(x + 39, y - 8, 5, 8, glass, 0.4); fill(x + 39, y - 8, 1, 8, WHITE, 0.4); fill(x + 43, y - 8, 1, 8, 0xa8b0ae, 0.5); // the glass
    fill(x + 39, y - 5, 5, 5, ale, 0.9); fill(x + 39, y - 5, 5, 1, aleLight, 0.85); // the ale
    fill(x + 39, y - 8, 5, 2, foam); fill(x + 39, y - 8, 5, 1, WHITE, 0.6); // the foam head
    fill(x + 44, y - 6, 1, 4, glass, 0.5); dot(x + 40, y - 3, WHITE, 0.5); // handle and glint
  },
  /** A table gramophone: a record on the platter and a flaring brass horn. */
  peakygramophone(x, y) {
    const box = 0x5a4030, boxLight = 0x7a5a40, brass = 0xc8a04a, brassLight = 0xe8c878, rec = 0x2a2018;
    fill(x + 37, y - 4, 8, 4, box); fill(x + 37, y - 4, 8, 1, boxLight); fill(x + 37, y - 1, 8, 1, 0x2e2016); // the box
    disc(x + 40, y - 5, 2, rec); dot(x + 40, y - 5, brass); // the record
    fill(x + 43, y - 7, 3, 2, brass); fill(x + 44, y - 9, 3, 2, brass); fill(x + 45, y - 11, 3, 3, brass); fill(x + 45, y - 11, 3, 1, brassLight); // the horn
    line(x + 42, y - 5, x + 44, y - 7, brass); // the tonearm
  },
  /** A folded newspaper with a masthead and columns of type. */
  peakynewspaper(x, y) {
    const paper = 0xd8d2c0, paperDark = 0xb8b2a0, ink = 0x3a3730;
    fill(x + 36, y - 3, 9, 3, paper); fill(x + 36, y - 3, 9, 1, 0xe4dece); fill(x + 36, y - 1, 9, 1, paperDark); // the folded paper
    fill(x + 37, y - 3, 7, 1, ink); // the masthead
    fill(x + 37, y - 2, 3, 1, ink, 0.6); fill(x + 41, y - 2, 3, 1, ink, 0.6); fill(x + 37, y - 1, 3, 1, ink, 0.5); // columns
    fill(x + 40, y - 2, 1, 2, paperDark); // the fold
  },
  /** An old fashioned on the rocks, an ice cube and an orange twist. */
  madmoldfashioned(x, y) {
    const glass = 0xdfe6e4, amber = 0xc87a2a, amberLight = 0xe89a4a, orange = 0xe8822a, ice = 0xeaf4f4;
    fill(x + 39, y - 5, 6, 5, glass, 0.45); fill(x + 39, y - 5, 1, 5, WHITE, 0.4); fill(x + 44, y - 5, 1, 5, 0xa8b0ae, 0.5); // the rocks glass
    fill(x + 39, y - 3, 6, 3, amber, 0.85); fill(x + 39, y - 3, 6, 1, amberLight, 0.8); // the whisky
    fill(x + 40, y - 4, 2, 2, ice, 0.7); dot(x + 42, y - 3, orange); dot(x + 40, y - 4, WHITE, 0.6); // ice and an orange twist
  },
  /** A typewriter with a sheet of paper in the platen and rows of round keys. */
  madmtypewriter(x, y) {
    const body = 0x3a3a40, bodyLight = 0x5a5a62, key = 0xcfc090, paper = 0xf2ecde, roller = 0x6a6e72;
    fill(x + 37, y - 4, 8, 4, body); fill(x + 37, y - 4, 8, 1, bodyLight); fill(x + 37, y - 1, 8, 1, 0x24242a); // the body
    fill(x + 38, y - 6, 6, 2, roller); fill(x + 39, y - 8, 4, 3, paper); fill(x + 39, y - 8, 4, 1, WHITE, 0.5); // platen and paper
    for (let r = 0; r < 2; r++) for (let c = 0; c < 4; c++) dot(x + 38 + c * 2, y - 3 + r, key); // keys
  },
  /** A rotary telephone with a dial and a cradled handset. */
  madmrotary(x, y) {
    const body = 0x2a2a30, bodyLight = 0x4a4a52, dial = 0xc0c4c8, cord = 0x3a3a40;
    fill(x + 38, y - 3, 7, 3, body); fill(x + 38, y - 3, 7, 1, bodyLight); fill(x + 38, y - 1, 7, 1, 0x1a1a20); // the base
    disc(x + 41, y - 2, 1.6, dial); dot(x + 41, y - 2, body); // the dial
    fill(x + 38, y - 5, 2, 2, body); fill(x + 43, y - 5, 2, 2, body); fill(x + 38, y - 5, 7, 1, bodyLight); // the handset
    line(x + 44, y - 3, x + 45, y, cord); line(x + 45, y, x + 44, y + 1, cord); // the coiled cord
  },
  /** A chrome ashtray with a lipstick-marked cigarette and a thin wisp of smoke. */
  madmashtray(x, y) {
    const tray = 0xc0c4c8, trayLight = 0xe4e8ea, cig = 0xf2ecde, ember = 0xe8782e;
    fill(x + 37, y - 1, 8, 1, tray); fill(x + 37, y - 2, 8, 1, trayLight); fill(x + 38, y - 1, 6, 1, 0x80868a); // the ashtray
    fill(x + 39, y - 3, 6, 1, cig); fill(x + 44, y - 3, 1, 1, ember); dot(x + 40, y - 3, 0xd84a2a); // the cigarette
    const w = Math.floor(now * 2) % 3; dot(x + 45 - w, y - 5 - w, 0xaab4c8, 0.35); // a thin wisp
  },
  /** A rolodex, its cards fanned up between two turn knobs. */
  madmrolodex(x, y) {
    const base = 0x3a3a40, baseLight = 0x5a5a62, card = 0xf2ecde, cardShade = 0xd4cdb8, knob = 0xc0c4c8;
    fill(x + 38, y - 2, 7, 2, base); fill(x + 38, y - 2, 7, 1, baseLight); // the base
    for (let i = 0; i < 5; i++) { const h = 3 + Math.round(Math.sin(i * 0.8) * 2); fill(x + 39 + i, y - 2 - h, 1, h, i % 2 ? card : cardShade); } // the fanned cards
    fill(x + 37, y - 4, 1, 2, knob); fill(x + 45, y - 4, 1, 2, knob); // the knobs
  },
  /** A chrome cigarette box open with cigarettes, and a table lighter with a tiny flame. */
  madmcigbox(x, y) {
    const box = 0xc0c4c8, boxLight = 0xe4e8ea, boxDark = 0x80868a, felt = 0x3a3a40, flame = 0xe8a030;
    fill(x + 37, y - 3, 6, 3, box); fill(x + 37, y - 3, 6, 1, boxLight); fill(x + 37, y - 1, 6, 1, boxDark); // the box
    fill(x + 38, y - 4, 4, 1, felt); for (let i = 0; i < 3; i++) dot(x + 38 + i, y - 4, 0xf2ecde); // cigarettes
    fill(x + 43, y - 3, 2, 3, box); fill(x + 43, y - 3, 1, 3, boxLight); dot(x + 44, y - 4, flame); // the lighter
  },
  /** A coffee cup and saucer with a gold rim band. */
  madmcoffee(x, y) {
    const cup = 0xf2ecde, cupShade = 0xd4cdb8, saucer = 0xe4dcc8, coffee = 0x3a2a1e, gold = 0xc8a04a;
    fill(x + 37, y, 8, 1, saucer); fill(x + 37, y - 1, 8, 1, 0xcfc8b4); // the saucer
    fill(x + 39, y - 5, 5, 4, cup); fill(x + 39, y - 5, 1, 4, WHITE, 0.5); fill(x + 43, y - 5, 1, 4, cupShade); // the cup
    fill(x + 39, y - 5, 5, 1, coffee); fill(x + 39, y - 2, 5, 1, gold); // coffee and the gold band
    fill(x + 44, y - 4, 1, 2, cupShade); dot(x + 45, y - 4, cup); // the handle
  },
  /** A slide projector with its carousel on top and a glowing lens. */
  madmprojector(x, y) {
    const body = 0xcfc090, bodyLight = 0xe4dcae, bodyDark = 0x9a8a5a, lens = 0x2a2a30, carousel = 0x3a3a40, glow = 0xfff0b0;
    fill(x + 37, y - 4, 7, 4, body); fill(x + 37, y - 4, 7, 1, bodyLight); fill(x + 37, y - 1, 7, 1, bodyDark); // the body
    disc(x + 40, y - 6, 2.4, carousel); ring(x + 40, y - 6, 2.6, bodyDark, 0.8); for (let k = 0; k < 6; k++) { const a = k / 6 * Math.PI * 2; dot(x + 40 + Math.round(Math.cos(a) * 2), y - 6 + Math.round(Math.sin(a) * 2), body); } // the carousel
    fill(x + 44, y - 3, 2, 2, lens); dot(x + 45, y - 2, glow); // the lens
    if (isNight) glowAt(x + 46, y - 2, 3, 2, glow, 0.14);
  },
  /** A round honeycomb dalgona candy with a shape pressed into it. */
  squiddalgona(x, y) {
    const candy = 0xd8a048, candyLight = 0xe8b860, candyDark = 0xb8832f;
    fill(x + 37, y - 1, 8, 1, SHADOW);
    disc(x + 41, y - 3, 3.4, candyDark); disc(x + 41, y - 3, 3, candy); dot(x + 39, y - 4, candyLight); // the candy
    ring(x + 41, y - 3, 1.5, candyDark); // a shape pressed in
  },
  /** A round tin holding a dalgona, a needle resting across it. */
  squidtin(x, y) {
    const tin = 0xc0c4c8, tinLight = 0xe4e8ea, tinDark = 0x80868a, candy = 0xd8a048, needle = 0x9a9488;
    fill(x + 37, y - 2, 8, 2, tin); fill(x + 37, y - 2, 8, 1, tinLight); fill(x + 37, y, 8, 1, tinDark); // the tin
    disc(x + 40, y - 3, 1.6, candy); // the candy
    line(x + 43, y - 5, x + 45, y - 2, needle); dot(x + 43, y - 5, tinLight); // a needle
  },
  /** Two folded ddakji paper tiles, one blue and one red. */
  squidddakji(x, y) {
    const blue = 0x3a6ac8, blueLight = 0x5a8ae8, red = 0xd83a3a, redLight = 0xe85a5a;
    fill(x + 37, y - 3, 5, 3, blue); fill(x + 37, y - 3, 5, 1, blueLight); fill(x + 38, y - 4, 3, 1, blue); // a blue tile
    fill(x + 41, y - 2, 5, 2, red); fill(x + 41, y - 2, 5, 1, redLight); dot(x + 39, y - 3, WHITE, 0.4); // a red tile
  },
  /** A little pile of glass marbles. */
  squidmarbles(x, y) {
    fill(x + 37, y, 8, 1, SHADOW);
    disc(x + 39, y - 2, 1.6, 0x4a8ad8); dot(x + 38, y - 3, 0xbfe0f4); // a blue marble
    disc(x + 42, y - 2, 1.6, 0x6abf6a); dot(x + 41, y - 3, 0xd0f0d0); // a green marble
    disc(x + 41, y - 4, 1.4, 0xe8b860); dot(x + 40, y - 5, 0xf8e0a0); // a cat's-eye on top
  },
  /** A green player badge with a printed number and a pin. */
  squidnumber(x, y) {
    const cloth = 0x1a9a7a, clothDark = 0x147a60, white = 0xf0eeea, pin = 0xc0c4c8;
    fill(x + 38, y - 5, 7, 5, cloth); fill(x + 38, y - 5, 7, 1, 0x2ab08a); fill(x + 38, y - 1, 7, 1, clothDark); // the patch
    fill(x + 39, y - 4, 5, 3, white); fill(x + 40, y - 4, 1, 3, 0x3a3a44); fill(x + 42, y - 4, 1, 3, 0x3a3a44); // a number
    dot(x + 41, y - 6, pin); // a pin
  },
  /** A bread roll and a bottle of milk: the dormitory ration. */
  squidbread(x, y) {
    const bread = 0xd8a860, breadDark = 0xb88840, bottle = 0xeaf4f4, milk = 0xf4f4ee, cap = 0xd83a3a;
    fill(x + 37, y - 2, 5, 2, bread); fill(x + 37, y - 3, 5, 1, 0xe8bc78); fill(x + 37, y, 5, 1, breadDark); // the roll
    fill(x + 43, y - 6, 3, 6, bottle, 0.7); fill(x + 43, y - 4, 3, 4, milk); fill(x + 43, y - 7, 2, 1, cap); dot(x + 43, y - 5, WHITE, 0.5); // the milk
  },
  /** A small glass jar of gold coins with a metal lid. */
  squidjar(x, y) {
    const glass = 0xbfe0e4, coin = 0xd6b25e, coinLight = 0xe8c878, lid = 0x9a9488;
    fill(x + 39, y - 6, 6, 6, glass, 0.5); fill(x + 39, y - 6, 1, 6, WHITE, 0.4); fill(x + 44, y - 6, 1, 6, 0x8ab0b4); // the jar
    for (let i = 0; i < 6; i++) dot(x + 40 + (i % 4), y - 2 - (i >> 2), (i % 2) ? coin : coinLight); // coins
    fill(x + 39, y - 7, 6, 1, lid); dot(x + 40, y - 7, 0xb8b4b0); // the lid
  },
  /** A black guard mask prop with its pink shape symbol. */
  squidmask(x, y) {
    const mask = 0x1a1a20, maskLight = 0x3a3a44, shape = 0xe84a7a;
    fill(x + 38, y - 6, 6, 6, mask); fill(x + 38, y - 6, 6, 1, maskLight); fill(x + 38, y - 1, 6, 1, 0x101018); // the mask
    ring(x + 41, y - 3, 1.8, shape); dot(x + 39, y - 5, maskLight); // its shape
  },
  /** A tall glass of milk. */
  boysmilk(x, y) {
    const glass = 0xeaf4f4, milk = 0xf6f4ee, milkShade = 0xe0ddd2;
    fill(x + 39, y - 7, 5, 7, glass, 0.4); fill(x + 39, y - 7, 1, 7, WHITE, 0.4); fill(x + 43, y - 7, 1, 7, 0xc0c8c6, 0.5); // the glass
    fill(x + 39, y - 5, 5, 5, milk); fill(x + 39, y - 5, 5, 1, WHITE, 0.6); fill(x + 39, y - 1, 5, 1, milkShade); // the milk
    dot(x + 40, y - 4, WHITE, 0.7);
  },
  /** A corporate coffee cup with a red sleeve and a star, lidded. */
  boyscoffee(x, y) {
    const cup = 0xf0f0ee, cupShade = 0xd4d4ce, red = 0xd21f2a, lid = 0x3a3a44;
    fill(x + 39, y - 6, 5, 6, cup); fill(x + 39, y - 6, 1, 6, WHITE, 0.6); fill(x + 43, y - 6, 1, 6, cupShade); // the cup
    fill(x + 39, y - 4, 5, 2, red); dot(x + 41, y - 3, WHITE); // a red sleeve with a star
    fill(x + 38, y - 7, 7, 1, lid); fill(x + 40, y - 8, 2, 1, lid); // the lid
  },
  /** A folded red superhero cape with gold trim. */
  boyscape(x, y) {
    const cape = 0xd21f2a, capeLight = 0xe8404a, capeDark = 0x9a1420, gold = 0xd6b25e;
    fill(x + 37, y - 4, 8, 4, cape); fill(x + 37, y - 4, 8, 1, capeLight); fill(x + 37, y - 1, 8, 1, capeDark); // the folded cape
    fill(x + 37, y - 4, 8, 1, gold); fill(x + 37, y - 2, 8, 1, gold, 0.6); dot(x + 39, y - 3, capeLight); // gold trim
  },
  /** A press microphone on a little stand with a red press flag. */
  boysmic(x, y) {
    const mic = 0x2a2a30, micLight = 0x4a4a52, grille = 0x9a9ea2, flag = 0xd21f2a;
    fill(x + 41, y - 2, 1, 2, 0x6a6e72); // the stand
    fill(x + 40, y - 7, 3, 5, mic); fill(x + 40, y - 7, 1, 5, micLight); disc(x + 41, y - 8, 1.6, grille); // the mic
    fill(x + 38, y - 6, 2, 3, flag); fill(x + 38, y - 6, 2, 1, 0xe8404a); dot(x + 38, y - 5, WHITE); // a press flag
  },
  /** A small gold hero statuette with arms outstretched. */
  boysstatuette(x, y) {
    const gold = 0xd6b25e, goldLight = 0xf0d884, goldDark = 0x9a7a2e, base = 0x2a2a30;
    fill(x + 39, y - 1, 5, 1, base); fill(x + 40, y - 2, 3, 1, 0x3a3a44); // the base
    fill(x + 40, y - 7, 3, 5, gold); fill(x + 40, y - 7, 1, 5, goldLight); fill(x + 42, y - 7, 1, 5, goldDark); // the figure
    dot(x + 41, y - 8, gold); fill(x + 39, y - 5, 1, 2, gold); fill(x + 43, y - 5, 1, 2, gold); dot(x + 40, y - 6, goldLight); // head and arms
  },
  /** A superhero comic book with a yellow title banner and a hero logo. */
  boyscomic(x, y) {
    const cover = 0x2a4a8a, coverLight = 0x3a6aba, red = 0xd21f2a, yellow = 0xf0d84a, white = 0xf0f0ee;
    fill(x + 37, y - 6, 8, 6, cover); fill(x + 37, y - 6, 8, 1, coverLight); fill(x + 44, y - 6, 1, 6, 0x1a2a4a); // the comic
    fill(x + 38, y - 5, 6, 1, yellow); // a title banner
    disc(x + 41, y - 3, 1.6, red); dot(x + 41, y - 3, white); dot(x + 38, y - 6, WHITE, 0.4); // a hero logo
  },
  /** A sleek smartphone showing a tiny selfie. */
  boysphone(x, y) {
    const body = 0x2a2a30, screen = 0x3a7adc, skin = 0xe3bb92, red = 0xd21f2a;
    fill(x + 40, y - 8, 4, 8, body); fill(x + 40, y - 8, 1, 8, 0x4a4a52); fill(x + 41, y - 7, 2, 6, screen); // the phone
    dot(x + 41, y - 6, skin); dot(x + 42, y - 6, skin); dot(x + 41, y - 4, red); dot(x + 42, y - 8, 0x1a1a20); // a selfie
  },
  /** A bold energy-drink can with a red and blue label. */
  boysenergy(x, y) {
    const can = 0x2a2a30, canLight = 0x4a4a52, red = 0xd21f2a, blue = 0x3a7adc, silver = 0xc0c4c8;
    fill(x + 40, y - 7, 4, 7, can); fill(x + 40, y - 7, 1, 7, canLight); fill(x + 43, y - 7, 1, 7, 0x1a1a20); // the can
    fill(x + 40, y - 5, 4, 2, red); fill(x + 40, y - 3, 4, 1, blue); // a label stripe
    fill(x + 40, y - 7, 4, 1, silver); dot(x + 41, y - 8, silver); dot(x + 40, y - 6, WHITE, 0.4); // the top
  },
  /** A Nuka-Cola bottle: amber glass, a white label and a star cap. */
  falloutnuka(x, y) {
    const glass = 0x7a4a2a, label = 0xf0f0ee, red = 0xd21f2a, cap = 0xc0c4c8;
    fill(x + 40, y - 8, 4, 8, glass, 0.85); fill(x + 40, y - 8, 1, 8, 0xa06a3a); fill(x + 43, y - 8, 1, 8, 0x4a2a16); // the bottle
    fill(x + 40, y - 5, 4, 3, label); fill(x + 40, y - 5, 4, 1, WHITE, 0.5); fill(x + 41, y - 4, 2, 1, red); // the label
    fill(x + 41, y - 9, 2, 1, cap); dot(x + 41, y - 10, red); dot(x + 40, y - 7, WHITE, 0.5); // the cap
  },
  /** A wrist Pip-Boy device with a glowing green screen and side dials. */
  falloutpipboy(x, y) {
    const casing = 0x5a5e54, caseLight = 0x7a7e74, screen = 0x0a2010, green = 0x3ae87a, dial = 0x8a8e84;
    fill(x + 38, y - 5, 7, 5, casing); fill(x + 38, y - 5, 7, 1, caseLight); fill(x + 38, y - 1, 7, 1, 0x3a3e34); // the body
    fill(x + 39, y - 4, 4, 3, screen); fill(x + 40, y - 3, 2, 1, green); fill(x + 40, y - 2, 3, 1, green, 0.7); // the screen
    dot(x + 44, y - 4, dial); dot(x + 44, y - 2, dial); // dials
    if (isNight) glowAt(x + 41, y - 3, 2, 2, green, 0.12);
  },
  /** A Vault-Tec bobblehead on a blue base, its big cartoon head bobbing. */
  falloutbobblehead(x, y) {
    const base = 0x2a4a7a, skin = 0xf0d0a8, hair = 0xd8c050, suit = 0x2e4e82, yellow = 0xf0c020;
    const bob = Math.round(Math.sin(now * 3 + x) * 1);
    fill(x + 38, y - 2, 6, 2, base); fill(x + 38, y - 2, 6, 1, 0x3a5a9a); // the base
    fill(x + 39, y - 5, 4, 3, suit); fill(x + 40, y - 5, 1, 3, yellow); // the little body
    fill(x + 39, y - 10 + bob, 4, 5, skin); fill(x + 39, y - 11 + bob, 4, 1, hair); // the bobbing head
    dot(x + 40, y - 8 + bob, PALETTE.eye); fill(x + 40, y - 7 + bob, 2, 1, 0xd89088); // a wink and grin
  },
  /** A stimpak: a red-tipped vial of red liquid with a needle. */
  falloutstimpak(x, y) {
    const body = 0xc0c4c8, bodyLight = 0xe4e8ea, red = 0xd21f2a, liquid = 0xd84a4a, needle = 0x8a8e94;
    fill(x + 38, y - 4, 6, 3, body); fill(x + 38, y - 4, 6, 1, bodyLight); fill(x + 39, y - 3, 3, 1, liquid); // the vial
    fill(x + 44, y - 4, 1, 3, red); line(x + 37, y - 3, x + 35, y - 1, needle); // an end and the needle
    fill(x + 39, y - 6, 2, 2, red); dot(x + 39, y - 4, WHITE, 0.5); // the plunger
  },
  /** A little pile of crimped bottle caps: wasteland currency. */
  falloutcaps(x, y) {
    fill(x + 37, y, 8, 1, SHADOW);
    const caps = [[39, -2, 0xc0392b], [42, -2, 0x9aa0a8], [40, -3, 0xd8b84a], [43, -3, 0x8a8e94], [41, -4, 0xc0392b]];
    for (const [cx, cy, c] of caps) { disc(x + cx, y + cy, 1.4, c); ring(x + cx, y + cy, 1.5, shaded(c, 0.8), 0.7); dot(x + cx - 1, y + cy - 1, WHITE, 0.4); }
  },
  /** A blue Vault-Tec lunchbox with a yellow band and a metal handle. */
  falloutlunchbox(x, y) {
    const box = 0x2a4a7a, boxLight = 0x3a6aba, metal = 0x8a8e94, yellow = 0xf0c020, red = 0xd21f2a;
    fill(x + 37, y - 5, 8, 5, box); fill(x + 37, y - 5, 8, 1, boxLight); fill(x + 37, y - 1, 8, 1, 0x1a2a4a); // the box
    fill(x + 37, y - 4, 8, 1, yellow); // a yellow band
    fill(x + 40, y - 7, 2, 2, metal); fill(x + 40, y - 7, 2, 1, 0xa8adb6); // the handle
    disc(x + 41, y - 3, 1.2, red); dot(x + 41, y - 3, WHITE); // a logo
  },
  /** A worn teddy bear, one eye missing, stitched seams and a patch. */
  falloutteddy(x, y) {
    const fur = 0x9a6a3a, furDark = 0x6a4a28, nose = 0x2a2018, patch = 0x7a5a3a;
    fill(x + 38, y - 5, 6, 5, fur); fill(x + 38, y - 5, 6, 1, 0xb08048); fill(x + 38, y - 1, 6, 1, furDark); // the body
    disc(x + 41, y - 7, 2, fur); dot(x + 39, y - 8, fur); dot(x + 43, y - 8, fur); // head and ears
    dot(x + 40, y - 7, PALETTE.eye); dot(x + 42, y - 7, nose); // an eye and nose
    fill(x + 39, y - 3, 1, 2, furDark); fill(x + 43, y - 3, 1, 2, furDark); dot(x + 40, y - 4, patch); // seams and a patch
  },
  /** A hanging IV bag of orange RadAway with a drip tube. */
  falloutradaway(x, y) {
    const bag = 0xe88a3a, bagLight = 0xf0a84a, fluid = 0xe87a2a, tube = 0xc0c4c8, hook = 0x8a8e94;
    fill(x + 39, y - 2, 1, 2, hook); dot(x + 39, y - 3, hook); // a hook
    fill(x + 38, y - 8, 5, 6, bag, 0.85); fill(x + 38, y - 8, 5, 1, bagLight); fill(x + 38, y - 5, 5, 3, fluid, 0.9); // the bag
    line(x + 40, y - 2, x + 41, y + 0, tube); dot(x + 39, y - 7, WHITE, 0.5); // the tube
  },
  /** The portal gun: a grey device with a green emitter lens. */
  rickportalgun(x, y) {
    const body = 0x9aa0a8, bodyLight = 0xc0c4c8, bodyDark = 0x6a6e74, green = 0x3ae84a, greenL = 0x7af06a, amber = 0xf0a020;
    fill(x + 37, y - 4, 7, 3, body); fill(x + 37, y - 4, 7, 1, bodyLight); fill(x + 37, y - 1, 7, 1, bodyDark); // the body
    fill(x + 43, y - 5, 2, 4, body); disc(x + 44, y - 4, 1.4, green); dot(x + 44, y - 4, greenL); // the emitter
    fill(x + 39, y - 6, 2, 2, amber); fill(x + 38, y - 1, 2, 2, bodyDark); // a chamber and grip
    if (isNight) glowAt(x + 44, y - 4, 2, 2, green, 0.12);
  },
  /** A round-bottom flask of bubbling green goo. */
  rickflask(x, y) {
    const glass = 0xcfe0e4, goo = 0x5adb4a, gooLight = 0x8af06a, cork = 0xc8a060;
    fill(x + 40, y - 3, 4, 3, glass, 0.5); fill(x + 41, y - 6, 2, 3, glass, 0.5); // the flask
    fill(x + 40, y - 2, 4, 2, goo, 0.85); fill(x + 40, y - 2, 4, 1, gooLight, 0.8); // the goo
    const b = Math.floor(now * 3) % 3; dot(x + 41 + (b % 2), y - 3 - b, gooLight, 0.8); // a rising bubble
    fill(x + 41, y - 7, 2, 1, cork); dot(x + 40, y - 3, WHITE, 0.4);
    if (isNight) glowAt(x + 42, y - 3, 3, 2, goo, 0.12);
  },
  /** A Plumbus: a lumpy fleshy household gizmo with dangly bits. */
  rickplumbus(x, y) {
    const flesh = 0xd87a6a, fleshLight = 0xe89a8a, fleshDark = 0xa85a4a, nub = 0xc86a5a;
    fill(x + 39, y - 5, 5, 4, flesh); fill(x + 39, y - 5, 5, 1, fleshLight); fill(x + 39, y - 2, 5, 1, fleshDark); // the body
    fill(x + 40, y - 7, 3, 2, flesh); // a top knob
    fill(x + 38, y - 3, 1, 3, nub); fill(x + 44, y - 4, 1, 2, nub); dot(x + 43, y - 1, nub); dot(x + 40, y - 5, fleshLight); // the dangly bits
  },
  /** A little butter robot holding a pat of butter, one sad eye. */
  rickbutterbot(x, y) {
    const metal = 0xb0b4b8, metalLight = 0xd8dce0, metalDark = 0x7a7e82, eye = 0x4a8ad8, butter = 0xf0d84a;
    fill(x + 39, y - 5, 5, 4, metal); fill(x + 39, y - 5, 5, 1, metalLight); fill(x + 39, y - 2, 5, 1, metalDark); // the body
    fill(x + 40, y - 7, 3, 2, metal); fill(x + 41, y - 7, 1, 1, eye); // the head and eye
    fill(x + 38, y - 4, 1, 2, metalDark); fill(x + 44, y - 4, 1, 2, metalDark); // arms
    fill(x + 40, y - 1, 3, 1, butter); dot(x + 41, y - 1, 0xf8e86a); // the butter
  },
  /** A red ray gun with a chrome muzzle glowing green. */
  rickraygun(x, y) {
    const body = 0xd84a2a, bodyLight = 0xe86a4a, bodyDark = 0xa83a1a, metal = 0x9aa0a8, green = 0x3ae84a;
    fill(x + 37, y - 4, 6, 3, body); fill(x + 37, y - 4, 6, 1, bodyLight); fill(x + 37, y - 1, 6, 1, bodyDark); // the body
    fill(x + 43, y - 4, 2, 2, metal); disc(x + 44, y - 3, 1.2, green); // the muzzle
    fill(x + 38, y - 1, 2, 3, body); fill(x + 40, y - 6, 2, 2, metal); dot(x + 38, y - 4, bodyLight); // grip and fin
  },
  /** A cluster of glowing green death-crystal shards. */
  rickcrystal(x, y) {
    const c = 0x3adb6a, cLight = 0x8af0a0, cDark = 0x1f9a4a;
    fill(x + 39, y, 5, 1, SHADOW);
    fill(x + 41, y - 7, 2, 7, c); fill(x + 40, y - 5, 1, 4, c); fill(x + 43, y - 4, 1, 3, c); // the shards
    fill(x + 41, y - 7, 1, 7, cLight); fill(x + 40, y - 5, 1, 1, cLight); // lit edges
    fill(x + 42, y - 6, 1, 5, cDark); dot(x + 41, y - 6, WHITE, 0.6);
    if (isNight) glowAt(x + 41, y - 4, 3, 4, c, 0.14);
  },
  /** A dented chrome hip flask with a little engraving. */
  rickhipflask(x, y) {
    const metal = 0xc0c4c8, metalLight = 0xe4e8ea, metalDark = 0x80868a, cap = 0x8a8e94;
    fill(x + 39, y - 6, 5, 6, metal); fill(x + 39, y - 6, 1, 6, metalLight); fill(x + 43, y - 6, 1, 6, metalDark); // the flask
    fill(x + 39, y - 6, 5, 1, metalLight); fill(x + 40, y - 7, 3, 1, cap); // the cap
    fill(x + 40, y - 4, 3, 2, metalDark, 0.4); dot(x + 40, y - 5, WHITE, 0.6); // an engraving and glint
  },
  /** A glowing alien mega seed with a faint green aura. */
  rickseed(x, y) {
    const seed = 0x8a6a3a, seedLight = 0xb08a4a, glow = 0x7af06a;
    fill(x + 39, y, 5, 1, SHADOW);
    disc(x + 41, y - 3, 2.4, seed); fill(x + 40, y - 4, 2, 1, seedLight); // the pod
    for (let k = 0; k < 4; k++) { const a = k / 4 * Math.PI * 2; dot(x + 41 + Math.round(Math.cos(a) * 2), y - 3 + Math.round(Math.sin(a) * 2), glow, 0.7); } // the aura
    dot(x + 40, y - 4, WHITE, 0.5);
    if (isNight) glowAt(x + 41, y - 3, 3, 3, glow, 0.1);
  },
  /** A sleek phone showing a rating. */
  mirrorphone(x, y) {
    const body = 0xeef0f2, bodyDark = 0xc8cdd4, screen = 0x1a3a5a, gold = 0xf0c84a, cyan = 0x5adce8;
    fill(x + 40, y - 8, 4, 8, body); fill(x + 40, y - 8, 1, 8, WHITE); fill(x + 43, y - 8, 1, 8, bodyDark); // the phone
    fill(x + 41, y - 7, 2, 6, screen); dot(x + 41, y - 6, gold); fill(x + 41, y - 4, 2, 1, cyan, 0.8); // a rating
    if (isNight) glowAt(x + 42, y - 5, 2, 2, cyan, 0.1);
  },
  /** The egg-shaped "cookie" consciousness device with a tiny status light. */
  mirrorcookie(x, y) {
    const shell = 0xf0f2f4, shellLight = 0xffffff, shellShade = 0xd8dce0, cyan = 0x5adce8, base = 0x1a1c22;
    fill(x + 38, y - 1, 7, 1, base); // a cradle
    fill(x + 40, y - 6, 4, 4, shell); fill(x + 39, y - 5, 6, 3, shell); fill(x + 40, y - 8, 4, 2, shell); // the smooth egg
    dot(x + 40, y - 7, shellLight); fill(x + 43, y - 6, 1, 3, shellShade); dot(x + 42, y - 5, cyan); // highlight and a light
    if (isNight) glowAt(x + 42, y - 5, 2, 1, cyan, 0.1);
  },
  /** An open lens case with two glowing eye-implant lenses. */
  mirrorlens(x, y) {
    const casing = 0xe0e4e8, caseLight = 0xf4f6f8, caseDark = 0xc8cdd4, lens = 0x8ad0e8, cyan = 0x5adce8;
    fill(x + 37, y - 3, 8, 3, casing); fill(x + 37, y - 3, 8, 1, caseLight); fill(x + 37, y - 1, 8, 1, caseDark); // the case
    disc(x + 39, y - 2, 1.4, lens, 0.7); ring(x + 39, y - 2, 1.5, cyan, 0.8); disc(x + 42, y - 2, 1.4, lens, 0.7); ring(x + 42, y - 2, 1.5, cyan, 0.8); // two lenses
    dot(x + 39, y - 3, WHITE, 0.5);
  },
  /** A tablet with a little UI on its screen. */
  mirrortablet(x, y) {
    const body = 0x2a2d35, screen = 0x1a3a5a, screenL = 0x4ab0f0, white = 0xeef0f2;
    fill(x + 37, y - 6, 8, 6, body); fill(x + 37, y - 6, 8, 1, 0x3a3d45); // the tablet
    fill(x + 38, y - 5, 6, 4, screen); for (let i = 0; i < 2; i++) fill(x + 39, y - 4 + i, 3 + i, 1, screenL, 0.7); fill(x + 42, y - 5, 2, 1, white); // UI
    dot(x + 44, y - 3, white, 0.5);
    if (isNight) glowAt(x + 41, y - 4, 3, 2, screenL, 0.1);
  },
  /** A smart-speaker orb with a glowing ring on top. */
  mirrorspeaker(x, y) {
    const mesh = 0x9aa0a8, meshLight = 0xc0c4c8, meshDark = 0x6a6e74, ringC = 0x5adce8;
    fill(x + 39, y, 5, 1, SHADOW);
    disc(x + 41, y - 3, 3, mesh); disc(x + 41, y - 3, 2.4, meshLight); fill(x + 39, y - 2, 5, 1, meshDark); // the orb
    for (let i = -2; i < 3; i++) dot(x + 41 + i, y - 4, meshDark, 0.5); // mesh
    ring(x + 41, y - 2, 1.4, ringC, 0.8); // a glowing ring
    if (isNight) glowAt(x + 41, y - 2, 3, 1, ringC, 0.1);
  },
  /** A VR headset with two glowing lenses and strap mounts. */
  mirrorvr(x, y) {
    const body = 0x2a2d35, bodyLight = 0x4a4d55, strap = 0x8a8e94, lens = 0x4ab0f0;
    fill(x + 38, y - 4, 7, 4, body); fill(x + 38, y - 4, 7, 1, bodyLight); fill(x + 38, y - 1, 7, 1, 0x1a1c22); // the goggles
    fill(x + 39, y - 3, 2, 2, lens); fill(x + 42, y - 3, 2, 2, lens); // the lenses
    fill(x + 37, y - 4, 1, 2, strap); fill(x + 45, y - 4, 1, 2, strap); // strap mounts
    if (isNight) glowAt(x + 41, y - 2, 3, 1, lens, 0.1);
  },
  /** A small translucent earpiece with a status light. */
  mirrorearpiece(x, y) {
    const body = 0xeef0f2, bodyDark = 0xc8cdd4, cyan = 0x5adce8;
    fill(x + 40, y - 4, 3, 4, body); fill(x + 40, y - 4, 1, 4, WHITE); fill(x + 42, y - 4, 1, 4, bodyDark); // the stem
    disc(x + 41, y - 5, 1.6, body); dot(x + 41, y - 5, cyan); dot(x + 40, y - 4, WHITE, 0.5); // the bud and light
  },
  /** A glossy white cube with an inner cyan glow node. */
  mirrorcube(x, y) {
    const face = 0xf0f2f4, faceLight = 0xffffff, faceDark = 0xd8dce0, cyan = 0x5adce8;
    fill(x + 39, y - 5, 5, 5, face); fill(x + 39, y - 5, 5, 1, faceLight); fill(x + 43, y - 5, 1, 5, faceDark); fill(x + 39, y - 1, 5, 1, faceDark); // the cube
    fill(x + 40, y - 4, 3, 3, faceLight, 0.5); dot(x + 41, y - 3, cyan); // a glow node
    if (isNight) glowAt(x + 41, y - 3, 2, 2, cyan, 0.1);
  },
  /** A wide-brimmed cowboy hat with a creased crown and a banded buckle. */
  westhat(x, y) {
    const felt = 0x6a4a2a, feltLight = 0x8a6a3a, feltDark = 0x4a3018, band = 0x3a2a1a;
    fill(x + 37, y - 1, 9, 1, felt); fill(x + 37, y - 2, 9, 1, feltLight); // the brim
    fill(x + 39, y - 5, 5, 3, felt); fill(x + 39, y - 5, 5, 1, feltLight); fill(x + 40, y - 6, 3, 1, felt); // the crown
    fill(x + 39, y - 3, 5, 1, band); dot(x + 41, y - 3, 0xc8a04a); fill(x + 41, y - 5, 1, 2, feltDark); // the band and crease
  },
  /** A revolver with a cylinder and a wooden grip. */
  westrevolver(x, y) {
    const metal = 0x8a8e94, metalLight = 0xc0c4c8, metalDark = 0x5a5e62, grip = 0x6a4a2a;
    fill(x + 37, y - 3, 8, 2, metal); fill(x + 37, y - 3, 8, 1, metalLight); fill(x + 37, y - 1, 8, 1, metalDark); // the frame
    disc(x + 41, y - 2, 1.4, metalDark); dot(x + 41, y - 2, metal); // the cylinder
    fill(x + 38, y - 1, 2, 3, grip); fill(x + 38, y - 1, 1, 3, 0x8a6a3a); // the grip
    fill(x + 43, y - 4, 1, 1, metal); dot(x + 37, y - 3, metalLight); // the hammer and a glint
  },
  /** A tumbler of amber whiskey with a bright glint. */
  westwhiskey(x, y) {
    const glass = 0xdfe6e4, amber = 0xc87a2a, amberLight = 0xe89a4a;
    fill(x + 39, y - 5, 5, 5, glass, 0.45); fill(x + 39, y - 5, 1, 5, WHITE, 0.4); fill(x + 43, y - 5, 1, 5, 0xa8b0ae, 0.5); // the tumbler
    fill(x + 39, y - 2, 5, 2, amber, 0.85); fill(x + 39, y - 2, 5, 1, amberLight, 0.8); dot(x + 40, y - 4, WHITE, 0.6); // the whiskey
  },
  /** The maze medallion: a gold coin etched with concentric maze rings. */
  westmazecoin(x, y) {
    const gold = 0xc8a04a, goldLight = 0xe8c878, goldDark = 0x8a6a2a, dark = 0x3a2a1a;
    fill(x + 39, y, 5, 1, SHADOW);
    disc(x + 41, y - 3, 3, goldDark); disc(x + 41, y - 3, 2.6, gold); // the coin
    for (let r = 2; r > 0; r -= 1) ring(x + 41, y - 3, r, goldDark, 0.8); dot(x + 41, y - 3, dark); dot(x + 40, y - 4, goldLight); // maze rings and a glint
  },
  /** A diagnostic tablet showing a host readout. */
  westtablet(x, y) {
    const body = 0xdfe0de, screen = 0x0e1a1e, cyan = 0x8ad0d8, white = 0xeaf2f0;
    fill(x + 37, y - 6, 8, 6, body); fill(x + 37, y - 6, 8, 1, 0xf0f0ee); // the tablet
    fill(x + 38, y - 5, 6, 4, screen); ring(x + 41, y - 3, 1.4, cyan, 0.8); fill(x + 39, y - 5, 1, 4, cyan, 0.6); // a readout
    dot(x + 44, y - 3, white, 0.5);
    if (isNight) glowAt(x + 41, y - 3, 3, 2, cyan, 0.1);
  },
  /** A host robotic hand laid out for repair, wrist wiring exposed. */
  westhand(x, y) {
    const skin = 0xc8b8a8, skinLight = 0xe0d0c0, skinDark = 0xa89888, metal = 0x8a8e94, wire = 0xd84a4a;
    fill(x + 37, y - 2, 6, 2, skin); fill(x + 37, y - 2, 6, 1, skinLight); fill(x + 37, y, 6, 1, skinDark); // the forearm
    for (let i = 0; i < 4; i++) fill(x + 42 + i % 2, y - 4 - i, 1, 2, skin); fill(x + 43, y - 2, 3, 1, skin); // fingers
    fill(x + 36, y - 2, 1, 2, metal); dot(x + 35, y - 2, wire); dot(x + 35, y - 1, 0x4a8ad8); // exposed wiring
  },
  /** A fanned poker hand of playing cards. */
  westcards(x, y) {
    const card = 0xf0ece0, cardShade = 0xd4d0c4, red = 0xd21f2a, black = 0x2a2a2e;
    for (let i = 0; i < 4; i++) { const cx = x + 38 + i, cy = y - 4 - (i % 2); fill(cx, cy, 4, 5, card); fill(cx, cy, 4, 1, WHITE); fill(cx + 3, cy, 1, 5, cardShade); dot(cx + 1, cy + 1, i % 2 ? red : black); } // the hand
    dot(x + 38, y - 4, WHITE, 0.4);
  },
  /** A boot spur with a little spinning rowel. */
  westspur(x, y) {
    const metal = 0x9aa0a8, metalLight = 0xc0c4c8, metalDark = 0x5a5e62, leather = 0x6a4a2a;
    fill(x + 38, y - 4, 4, 1, leather); fill(x + 38, y - 3, 1, 3, leather); // the heel band
    ring(x + 43, y - 2, 2, metal); for (let k = 0; k < 6; k++) { const a = k / 6 * Math.PI * 2; dot(x + 43 + Math.round(Math.cos(a) * 2.6), y - 2 + Math.round(Math.sin(a) * 2.6), metalLight); } // the rowel
    dot(x + 43, y - 2, metalDark); dot(x + 41, y - 3, metalLight);
  },
  /** A flux capacitor: a steel box with its three-tube Y lit flux-blue, flaring faster every few seconds; it glows at night. */
  bttffluxcap(x, y) {
    const steel = 0xb0b4bc, steelD = 0x7a7e86, flux = 0x6fd0ff, hot = 0xdff4ff;
    const cx = x + 40, t = (now + x * 0.2) % 6, fire = t < 1.2;              // a burst every ~6 s
    fill(cx - 4, y - 11, 9, 11, steel); fill(cx - 4, y - 11, 9, 1, 0xd4d8de); fill(cx + 4, y - 11, 1, 11, steelD); fill(cx - 4, y, 9, 1, SHADOW);
    fill(cx - 3, y - 10, 7, 9, 0x20242c);                                     // the dark inner face
    const lit = (bx, by) => { dot(bx, by, fire ? hot : flux); glowAt(bx, by, 2, 2, flux, fire ? 0.5 : 0.22); };
    for (let k = 0; k < 3; k++) lit(cx, y - 3 - k);                           // the stem
    for (let k = 1; k < 3; k++) { lit(cx - k, y - 6 - k); lit(cx + k, y - 6 - k); } // the two arms
    if (isNight) glowLate(() => glowAt(cx, y - 6, 6, 6, flux, fire ? 0.4 : 0.22));
  },
  /** Marty's pink hoverboard, bobbing as it hovers over a faint magenta cushion that glows at night. */
  bttfhoverboard(x, y) {
    const body = 0xe85a8a, bodyL = 0xf58ab0, rim = 0x3aa0c0, yellow = 0xf2d35b;
    const cx = x + 39, by = y - 5 + Math.round(Math.sin(now * 2 + x));        // gentle hover
    fill(cx - 5, by, 11, 3, body); fill(cx - 5, by, 11, 1, bodyL); fill(cx - 5, by + 2, 11, 1, 0xb84070);
    fill(cx - 5, by + 1, 2, 1, yellow); fill(cx + 4, by + 1, 2, 1, rim);      // the end plates
    dot(cx - 2, by, WHITE, 0.6);
    glowAt(cx, by + 3, 6, 2, 0xff70c0, isNight ? 0.3 : 0.12);                 // the hover cushion
    fill(cx - 1, y, 3, 1, SHADOW);
  },
  /** A Mr. Fusion home energy reactor: a round-topped steel pot with a hinged lid and a green power light that blinks. */
  bttfmrfusion(x, y) {
    const steel = 0xc0c4cc, steelD = 0x80848c, cap = 0x2a2e36, green = 0x7ad84a;
    const cx = x + 40;
    fill(cx - 4, y - 7, 9, 7, steel); fill(cx - 4, y - 7, 1, 7, 0xdfe3e9); fill(cx + 4, y - 7, 1, 7, steelD); fill(cx - 4, y, 9, 1, SHADOW);
    fill(cx - 3, y - 10, 7, 3, cap); fill(cx - 3, y - 10, 7, 1, 0x4a4e58); fill(cx - 1, y - 12, 3, 2, cap); // the lid and knob
    const blink = Math.floor(now * 2) % 2;
    dot(cx - 2, y - 4, green, blink ? 1 : 0.4); fill(cx + 1, y - 5, 2, 3, 0x3a3e46); // label and light
    if (isNight) glowLate(() => glowAt(cx - 2, y - 4, 2, 2, green, 0.3 * blink));
  },
  /** An 88 mph speedometer on a little stand: the needle climbs and pins at 88, where the dial flashes red. */
  bttfspeedo(x, y) {
    const case_ = 0x2a2e36, dial = 0xeee6d2, red = 0xd8402a, orange = 0xe8782e;
    const cx = x + 40, cy = y - 6;
    fill(cx - 2, y - 2, 5, 2, 0x4a4e58); fill(cx - 2, y, 5, 1, SHADOW);       // the stand
    disc(cx, cy, 5, case_); disc(cx, cy, 4, dial); ring(cx, cy, 4, 0x8a8276, 0.6);
    for (let k = 0; k < 5; k++) { const a = Math.PI + (k / 4) * Math.PI; dot(cx + Math.round(Math.cos(a) * 3), cy + Math.round(Math.sin(a) * 3), k === 4 ? red : 0x6a6256); }
    const t = (now + x * 0.3) % 5, at88 = t > 3.5, frac = at88 ? 1 : t / 3.5; // climbs to 88
    const a = Math.PI + frac * Math.PI;
    line(cx, cy, cx + Math.round(Math.cos(a) * 3), cy + Math.round(Math.sin(a) * 3), at88 ? red : orange);
    if (at88) glowAt(cx, cy, 4, 4, red, isNight ? 0.3 : 0.14);
  },
  /** A plutonium case: a dented metal briefcase marked with a radiation trefoil, a green light blinking on its latch. */
  bttfplutonium(x, y) {
    const metal = 0x9aa0a8, metalL = 0xc2c8d0, metalD = 0x686e76, yellow = 0xf2d35b, green = 0x7ad84a;
    const cx = x + 39;
    fill(cx - 5, y - 6, 12, 6, metal); fill(cx - 5, y - 6, 12, 1, metalL); fill(cx - 5, y - 1, 12, 1, metalD); fill(cx - 5, y, 12, 1, SHADOW);
    fill(cx - 1, y - 8, 4, 2, metalD); fill(cx, y - 9, 2, 1, metalD);          // the handle
    disc(cx + 1, y - 3, 2, yellow); dot(cx + 1, y - 3, 0x2a2a2a);              // the trefoil badge
    dot(cx - 4, y - 5, Math.floor(now * 2) % 2 ? green : 0x3a5a2a);            // the latch light
  },
  /** A sports almanac: a thick blue book in a dust jacket with gold page edges and a star on the cover. */
  bttfalmanac(x, y) {
    const cover = 0x3a6ac0, coverD = 0x27488a, pages = 0xe6dcc0, gold = 0xf2c04a;
    const cx = x + 39;
    fill(cx - 4, y - 9, 9, 9, cover); fill(cx - 4, y - 9, 1, 9, 0x5a8ae0); fill(cx + 4, y - 9, 1, 9, coverD); fill(cx - 4, y, 9, 1, SHADOW);
    fill(cx + 4, y - 8, 1, 7, pages);                                          // the page edges
    fill(cx - 2, y - 7, 5, 1, gold); fill(cx - 2, y - 5, 5, 1, 0xd8d0b8);      // the title band
    dot(cx, y - 3, gold);
  },
  /** A diner milkshake: a tall fluted glass of shake topped with whipped cream, a cherry and a striped straw. */
  bttfmilkshake(x, y) {
    const glass = 0xdff0f6, shake = 0xf2d8c8, cream = 0xfff4ea, cherry = 0xd8402a, straw = 0xd8402a;
    const cx = x + 40;
    fill(cx - 3, y - 8, 7, 8, shake); fill(cx - 3, y - 8, 7, 1, 0xf8e4d6); fill(cx + 3, y - 8, 1, 8, 0xd8b8a4);
    fill(cx - 3, y - 8, 1, 8, glass, 0.6); fill(cx - 3, y, 7, 1, SHADOW);
    fill(cx - 3, y - 11, 7, 3, cream); fill(cx - 2, y - 12, 5, 1, cream); dot(cx, y - 13, cherry); // cream and cherry
    fill(cx + 2, y - 13, 1, 5, straw); dot(cx + 2, y - 11, WHITE);             // the straw
    dot(cx - 1, y - 6, WHITE, 0.5);
  },
  /** A cassette Walkman with foam-padded headphones; its little reels turn while it plays. */
  bttfwalkman(x, y) {
    const body = 0xc9ccd2, bodyD = 0x898d95, win = 0x2a2e36, orange = 0xe8782e;
    const cx = x + 39;
    fill(cx - 4, y - 7, 10, 7, body); fill(cx - 4, y - 7, 10, 1, 0xe0e3e8); fill(cx - 4, y - 1, 10, 1, bodyD); fill(cx - 4, y, 10, 1, SHADOW);
    fill(cx - 3, y - 6, 8, 4, win);                                            // the cassette window
    const sp = Math.floor(now * 4) % 2;
    dot(cx - 1, y - 4, sp ? 0xd8d8d8 : 0x9a9a9a); dot(cx + 3, y - 4, sp ? 0x9a9a9a : 0xd8d8d8); // the reels
    fill(cx - 4, y - 2, 2, 1, orange); line(cx + 6, y - 9, cx + 6, y - 3, bodyD); fill(cx + 5, y - 10, 3, 1, win); // headphone band
  },
  /** A mosquito trapped in a golden amber bead on a little stand; it glows warm amber at night. */
  jpamber(x, y) {
    const amber = 0xe8a830, amberL = 0xf6c860, amberD = 0xb07818, stand = 0x3a2e1a, bug = 0x2a241a;
    const cx = x + 40;
    fill(cx - 1, y - 3, 3, 3, stand); fill(cx - 2, y, 5, 1, SHADOW);
    disc(cx, y - 7, 4, amber); disc(cx - 1, y - 8, 2, amberL); ring(cx, y - 7, 4, amberD, 0.5);
    dot(cx, y - 7, bug); dot(cx + 1, y - 6, bug); line(cx, y - 7, cx - 2, y - 9, bug);
    dot(cx - 2, y - 9, WHITE, 0.6);
    if (isNight) glowLate(() => glowAt(cx, y - 7, 5, 5, amber, 0.26));
  },
  /** A rack of three DNA vials glowing teal, the middle one bubbling; soft glow at night. */
  jpdnavial(x, y) {
    const rack = 0x4a4e56, glass = 0xdff0f6, dna = 0x3ad0c0, cap = 0xd83020;
    const cx = x + 39;
    fill(cx - 4, y - 1, 11, 2, rack); fill(cx - 4, y, 11, 1, SHADOW);
    for (let k = 0; k < 3; k++) { const vx = cx - 3 + k * 4; fill(vx, y - 8, 3, 7, glass, 0.5); fill(vx, y - 5, 3, 4, dna, 0.85); fill(vx, y - 9, 3, 1, cap);
      if (k === 1) { const b = Math.floor(now * 4) % 3; dot(vx + 1, y - 6 - b, 0x9ff0e4); } }
    if (isNight) glowLate(() => glowAt(cx, y - 5, 6, 5, dna, 0.2));
  },
  /** A toy tour jeep: a grey-and-red open-top 4x4 with a roof light bar. */
  jpjeep(x, y) {
    const body = 0xcfcabc, bodyD = 0x9a9488, red = 0xd83020, tyre = 0x1e2026, orange = 0xe8a830;
    const cx = x + 39;
    fill(cx - 5, y - 4, 12, 4, body); fill(cx - 5, y - 4, 12, 1, 0xe2ddce); fill(cx - 5, y - 1, 12, 1, bodyD);
    fill(cx - 5, y - 5, 2, 1, red); fill(cx - 2, y - 5, 2, 1, red); fill(cx + 1, y - 5, 2, 1, red);
    fill(cx - 3, y - 7, 8, 2, body); fill(cx - 2, y - 6, 6, 1, 0x2a3a4a);
    fill(cx - 2, y - 8, 5, 1, orange); dot(cx - 2, y - 8, 0xffe08a); dot(cx + 2, y - 8, 0xffe08a);
    disc(cx - 3, y, 2, tyre); disc(cx + 4, y, 2, tyre); dot(cx - 3, y, 0x4a4e56); dot(cx + 4, y, 0x4a4e56);
    fill(cx - 5, y + 1, 12, 1, SHADOW);
  },
  /** Night-vision goggles on a strap; the lenses glow green in the dark. */
  jpgoggles(x, y) {
    const body = 0x3a4030, bodyL = 0x5a6048, lens = 0x6ef06a, strap = 0x2a2e22;
    const cx = x + 40;
    fill(cx - 5, y - 5, 11, 4, body); fill(cx - 5, y - 5, 11, 1, bodyL); fill(cx - 5, y, 11, 1, SHADOW);
    fill(cx - 6, y - 4, 1, 3, strap); fill(cx + 6, y - 4, 1, 3, strap);
    fill(cx - 4, y - 7, 3, 3, body); fill(cx + 2, y - 7, 3, 3, body);
    const on = isNight || Math.floor(now * 2) % 4 !== 0;
    dot(cx - 3, y - 6, on ? lens : 0x3a5a3a); dot(cx + 3, y - 6, on ? lens : 0x3a5a3a);
    if (isNight) glowLate(() => { glowAt(cx - 3, y - 6, 2, 2, lens, 0.4); glowAt(cx + 3, y - 6, 2, 2, lens, 0.4); });
  },
  /** A dinosaur egg nested in straw, cracking open now and then to show a little claw. */
  jpraptoregg(x, y) {
    const egg = 0xe6dcc0, eggD = 0xc2b48c, straw = 0xc8a850, claw = 0x6a8a4a, crack = 0x3a3024;
    const cx = x + 40;
    for (let k = -4; k <= 4; k += 2) line(cx + k, y, cx + Math.round(k * 0.5), y - 2, straw);
    fill(cx - 3, y - 8, 7, 8, egg); fill(cx - 3, y - 8, 7, 1, 0xf2ead2); fill(cx + 3, y - 8, 1, 8, eggD);
    disc(cx, y - 8, 3, egg); fill(cx - 3, y, 7, 1, SHADOW);
    line(cx, y - 7, cx + 1, y - 4, crack); line(cx + 1, y - 4, cx - 1, y - 2, crack);
    const t = (now + x * 0.3) % 9; if (t < 2) { dot(cx, y - 4, claw); dot(cx + 1, y - 5, claw); dot(cx + 1, y - 3, PALETTE.eye); }
  },
  /** A cryo canister disguised as a shaving-cream can, its false top open and venting cold mist. */
  jpcanister(x, y) {
    const can = 0xc8ccd2, canD = 0x8a8e96, red = 0xd83020, cap = 0x2a2e36, mist = 0xcfe6ee;
    const cx = x + 40;
    fill(cx - 3, y - 9, 7, 9, can); fill(cx - 3, y - 9, 1, 9, 0xe0e3e9); fill(cx + 3, y - 9, 1, 9, canD); fill(cx - 3, y, 7, 1, SHADOW);
    fill(cx - 3, y - 5, 7, 2, red); fill(cx - 2, y - 11, 5, 2, cap);
    const t = (now + x * 0.2) % 6; if (t < 2) { fill(cx - 1, y - 13, 3, 2, mist, 0.6); dot(cx, y - 14, mist, 0.4); }
    dot(cx - 1, y - 7, WHITE, 0.5);
  },
  /** A burning red road flare in a little holder, its tip sparking and glowing. */
  jpflare(x, y) {
    const stick = 0x9a2a1a, holder = 0x3a3e46, hot = 0xffc060, red = 0xff5a3a;
    const cx = x + 40;
    fill(cx - 1, y - 3, 3, 3, holder); fill(cx - 2, y, 5, 1, SHADOW);
    fill(cx, y - 10, 2, 7, stick);
    const fl = 0.7 + 0.3 * Math.sin(now * 8 + x);
    fill(cx, y - 12, 2, 2, red, fl); dot(cx, y - 13, hot, fl); dot(cx + 1, y - 12, hot, fl);
    glowAt(cx + 1, y - 11, 4, 5, red, (isNight ? 0.3 : 0.14) * fl);
    if (isNight) glowLate(() => glowAt(cx + 1, y - 11, 6, 6, 0xff6a3a, 0.3 * fl));
  },
  /** A glass of water that ripples in time with distant, approaching footsteps. */
  jptremorcup(x, y) {
    const glass = 0xdff0f6, water = 0x6fb8d0, rim = 0xb8d8e2;
    const cx = x + 40;
    fill(cx - 3, y - 7, 7, 7, glass, 0.4); fill(cx - 3, y - 7, 1, 7, glass, 0.7); fill(cx + 3, y - 7, 1, 7, 0xb8cdd6, 0.6);
    fill(cx - 3, y - 4, 7, 4, water, 0.7); fill(cx - 3, y - 7, 7, 1, rim); fill(cx - 3, y, 7, 1, SHADOW);
    const t = now % 2.4; if (t < 0.5) { fill(cx - 3, y - 4, 7, 1, 0x9fdcf0); dot(cx - 1, y - 4, WHITE, 0.6); dot(cx + 2, y - 4, WHITE, 0.4); }
    dot(cx - 2, y - 6, WHITE, 0.5);
  },
  /** A gold Continental coin standing on a little stand, catching a glint; it gleams warm at night. */
  wickcoin(x, y) {
    const gold = 0xd6b25e, goldL = 0xf2d888, goldD = 0x9a7e3a, stand = 0x2a2a30;
    const cx = x + 40;
    fill(cx - 1, y - 3, 3, 3, stand); fill(cx - 2, y, 5, 1, SHADOW);
    disc(cx, y - 7, 4, gold); ring(cx, y - 7, 4, goldD, 0.6); disc(cx - 1, y - 8, 1.5, goldL);
    ring(cx, y - 7, 2.2, goldD, 0.5); dot(cx - 2, y - 9, WHITE, 0.7);
    if (isNight) glowLate(() => glowAt(cx, y - 7, 4, 4, gold, 0.2));
  },
  /** A blood-oath marker: a round gold-rimmed medallion with a red seal on a dark cushion. */
  wickmarker(x, y) {
    const gold = 0xd6b25e, goldL = 0xf2d888, red = 0x9a1a1a, cushion = 0x1a1a1e;
    const cx = x + 40;
    fill(cx - 4, y - 2, 9, 2, cushion); fill(cx - 4, y, 9, 1, SHADOW);
    disc(cx, y - 6, 4, gold); disc(cx, y - 6, 3, red); ring(cx, y - 6, 4, goldL, 0.6);
    dot(cx, y - 6, gold); dot(cx - 2, y - 8, WHITE, 0.6);
  },
  /** A cut-crystal tumbler of bourbon over a single ice cube, a decanter glint on the rim. */
  wickwhiskey(x, y) {
    const glass = 0xdff0f6, bourbon = 0xb0702a, ice = 0xcfe6ee;
    const cx = x + 40;
    fill(cx - 3, y - 6, 7, 6, glass, 0.45); fill(cx - 3, y - 6, 1, 6, glass, 0.7); fill(cx + 3, y - 6, 1, 6, 0xb8cdd6, 0.6);
    fill(cx - 3, y - 3, 7, 3, bourbon, 0.8); fill(cx - 2, y - 4, 2, 2, ice, 0.8); fill(cx - 3, y, 7, 1, SHADOW);
    dot(cx + 1, y - 5, WHITE, 0.6);
  },
  /** A black-and-gold hotel rotary phone; its dial-light blinks when a call comes in. */
  wickphone(x, y) {
    const body = 0x1a1a1e, bodyL = 0x3a3a42, gold = 0xd6b25e;
    const cx = x + 40;
    fill(cx - 4, y - 3, 10, 3, body); fill(cx - 4, y - 3, 10, 1, bodyL); fill(cx - 4, y, 10, 1, SHADOW);
    fill(cx - 5, y - 6, 3, 2, body); fill(cx + 4, y - 6, 3, 2, body); fill(cx - 5, y - 7, 12, 1, body); // the handset
    disc(cx + 1, y - 2, 2, 0x2a2a30); ring(cx + 1, y - 2, 2, gold, 0.6);        // the dial
    if (Math.floor(now * 2) % 6 === 0) dot(cx - 3, y - 5, 0xf2d888);            // ring light
  },
  /** A single deep-red rose in a slim gold vase, one petal catching the light. */
  wickrose(x, y) {
    const vase = 0xd6b25e, vaseD = 0x9a7e3a, stem = 0x3a6a3a, rose = 0x7a1a1a, roseL = 0xa82a2a;
    const cx = x + 40;
    fill(cx - 1, y - 5, 3, 5, vase); fill(cx - 1, y - 5, 1, 5, 0xf2d888); fill(cx + 1, y - 5, 1, 5, vaseD); fill(cx - 1, y, 3, 1, SHADOW);
    fill(cx, y - 9, 1, 4, stem); disc(cx, y - 11, 2, rose); dot(cx - 1, y - 12, roseL); dot(cx, y - 11, 0x5a1414);
    dot(cx - 1, y - 8, stem);
  },
  /** A dog's steel bowl with a gold-tag collar beside it -- a quiet tribute. */
  wickdogbowl(x, y) {
    const steel = 0xb4b8c0, steelD = 0x7e828a, food = 0x6a4a2a, gold = 0xd6b25e, collar = 0x3a2a1e;
    const cx = x + 39;
    fill(cx - 4, y - 2, 8, 2, steel); fill(cx - 4, y - 2, 8, 1, 0xd6dae0); fill(cx - 4, y, 8, 1, SHADOW);
    fill(cx - 3, y - 3, 6, 1, food);                                            // the kibble
    fill(cx + 4, y - 1, 4, 1, collar); dot(cx + 6, y - 1, gold);                // the collar + tag
  },
  /** A neat stack of gold Continental coins, the top one being set down and lifted now and then. */
  wickcoinstack(x, y) {
    const gold = 0xd6b25e, goldL = 0xf2d888, goldD = 0x9a7e3a;
    const cx = x + 40;
    fill(cx - 3, y, 7, 1, SHADOW);
    for (let k = 0; k < 5; k++) { fill(cx - 3, y - 1 - k * 2, 7, 2, gold); fill(cx - 3, y - 1 - k * 2, 7, 1, goldL); fill(cx - 3, y - k * 2, 7, 1, goldD, 0.6); }
    const t = (now + x * 0.3) % 6, lift = t < 1.5 ? 2 : 0;                       // one coin lifts
    fill(cx - 3, y - 11 - lift, 7, 2, gold); fill(cx - 3, y - 11 - lift, 7, 1, goldL);
    if (isNight) glowLate(() => glowAt(cx, y - 6, 5, 6, gold, 0.14));
  },
  /** A pair of gold cufflinks on a velvet tray, each with a tiny red stone. */
  wickcufflinks(x, y) {
    const tray = 0x3a1a1a, gold = 0xd6b25e, goldL = 0xf2d888, red = 0x9a1a1a;
    const cx = x + 40;
    fill(cx - 4, y - 2, 9, 2, tray); fill(cx - 4, y - 2, 9, 1, 0x5a2a2a); fill(cx - 4, y, 9, 1, SHADOW);
    for (const dx of [-2, 2]) { disc(cx + dx, y - 4, 1.6, gold); dot(cx + dx, y - 4, red); dot(cx + dx - 1, y - 5, goldL); }
  },
  /** A furled black umbrella with a tan crook handle, standing in a little brass ring. */
  kingsmanumbrella(x, y) {
    const black = 0x1a1a1e, blackL = 0x33333a, tan = 0xc8a878, brass = 0xc8a24a;
    const cx = x + 40;
    ring(cx, y - 1, 2, brass, 0.8); fill(cx - 2, y, 5, 1, SHADOW);
    fill(cx - 1, y - 11, 2, 10, black); fill(cx - 1, y - 11, 1, 10, blackL);    // the furled canopy
    fill(cx - 2, y - 11, 4, 1, black); dot(cx, y - 12, blackL);                 // the tip
    fill(cx - 3, y - 3, 3, 1, tan); fill(cx - 3, y - 4, 1, 2, tan);             // the crook handle
    dot(cx, y - 8, WHITE, 0.4);
  },
  /** Black-framed spy spectacles on a stand; a lens glints as the heads-up display sweeps across it. */
  kingsmanglasses(x, y) {
    const frame = 0x1a1a1e, glass = 0x8ac0e0, scan = 0xbfeaff, stand = 0x3a3a42;
    const cx = x + 40;
    fill(cx - 1, y - 2, 3, 2, stand); fill(cx - 2, y, 5, 1, SHADOW);
    fill(cx - 5, y - 6, 4, 3, frame); fill(cx + 1, y - 6, 4, 3, frame); fill(cx - 1, y - 5, 2, 1, frame); // the frames + bridge
    fill(cx - 4, y - 5, 2, 1, glass); fill(cx + 2, y - 5, 2, 1, glass);
    const sw = Math.floor(now * 3) % 4; dot(cx - 4 + (sw > 1 ? 1 : 0), y - 5, scan); dot(cx + 2 + (sw % 2), y - 5, scan);
    if (isNight) glowLate(() => { glowAt(cx - 3, y - 5, 2, 2, scan, 0.3); glowAt(cx + 3, y - 5, 2, 2, scan, 0.3); });
  },
  /** A single polished black oxford shoe with a mirror shine along the toe. */
  kingsmanoxford(x, y) {
    const leather = 0x1e1a18, leatherL = 0x3a3230, sole = 0x4a3a2a, shine = 0xcfd6dd;
    const cx = x + 38;
    fill(cx, y - 2, 11, 2, leather); fill(cx + 7, y - 4, 4, 2, leather); fill(cx + 7, y - 4, 4, 1, leatherL); // the toe box
    fill(cx, y - 3, 6, 1, leather); fill(cx + 1, y - 4, 4, 1, leatherL);        // the vamp
    fill(cx, y, 11, 1, sole); fill(cx, y, 11, 1, SHADOW);
    dot(cx + 9, y - 3, shine, 0.8); dot(cx + 3, y - 3, shine, 0.4);             // the shine
  },
  /** A heavy gold signet ring on a dark cushion, its crest catching the light. */
  kingsmansignet(x, y) {
    const gold = 0xc8a24a, goldL = 0xe8c878, goldD = 0x8a6a2a, cushion = 0x1e2a44;
    const cx = x + 40;
    fill(cx - 4, y - 2, 9, 2, cushion); fill(cx - 4, y, 9, 1, SHADOW);
    ring(cx, y - 4, 2.4, gold, 0.9); fill(cx - 2, y - 7, 4, 3, gold); fill(cx - 2, y - 7, 4, 1, goldL); // the band + face
    dot(cx, y - 6, goldD); dot(cx - 1, y - 7, WHITE, 0.7);
  },
  /** A slim lacquered gadget pen resting on a tray, a tiny light blinking at its tip. */
  kingsmanpen(x, y) {
    const body = 0x1e2a44, bodyL = 0x2e3e60, gold = 0xc8a24a, tip = 0x6ef06a;
    const cx = x + 38;
    fill(cx, y - 1, 12, 1, 0x3a3a42); fill(cx, y, 12, 1, SHADOW);
    fill(cx, y - 3, 11, 2, body); fill(cx, y - 3, 11, 1, bodyL); fill(cx + 9, y - 3, 2, 2, gold); // the barrel + cap
    dot(cx, y - 2, gold);                                                       // the nib
    if (Math.floor(now * 2) % 3 === 0) { dot(cx + 10, y - 3, tip); if (isNight) glowLate(() => glowAt(cx + 10, y - 3, 2, 2, tip, 0.3)); }
  },
  /** A brass trench lighter; flick it open and a little flame dances now and then. */
  kingsmanlighter(x, y) {
    const brass = 0xc8a24a, brassL = 0xe8c878, brassD = 0x8a6a2a, flame = 0xffd36b;
    const cx = x + 40;
    fill(cx - 2, y - 5, 5, 5, brass); fill(cx - 2, y - 5, 5, 1, brassL); fill(cx + 2, y - 5, 1, 5, brassD); fill(cx - 2, y, 5, 1, SHADOW);
    fill(cx - 2, y - 6, 5, 1, brassD);
    const t = (now + x * 0.3) % 5; if (t < 1.5) { const f = candlelight(x * 0.3); fill(cx, y - 8, 1, 2, flame, f); dot(cx, y - 9, 0xfff0c0, f); if (isNight) glowLate(() => glowAt(cx, y - 8, 3, 4, flame, 0.26 * f)); }
  },
  /** A cut-glass dram of whisky on a leather coaster, a warm amber glint. */
  kingsmanwhisky(x, y) {
    const glass = 0xdff0f6, whisky = 0xc88a2a, coaster = 0x4a2a1a;
    const cx = x + 40;
    fill(cx - 4, y - 1, 9, 1, coaster); fill(cx - 4, y, 9, 1, SHADOW);
    fill(cx - 2, y - 5, 6, 4, glass, 0.45); fill(cx - 2, y - 5, 1, 4, glass, 0.7); fill(cx + 3, y - 5, 1, 4, 0xb8cdd6, 0.6);
    fill(cx - 2, y - 3, 6, 2, whisky, 0.85); dot(cx + 2, y - 4, WHITE, 0.6);
  },
  /** A coiled tailor's tape measure with a stub of chalk beside it. */
  kingsmantape(x, y) {
    const tape = 0xf0d878, tapeD = 0xc8a848, mark = 0x8a6a2a, chalk = 0xe8e2d2;
    const cx = x + 39;
    ring(cx, y - 3, 3, tape, 0.95); ring(cx, y - 3, 3, tapeD, 0.4); disc(cx, y - 3, 1.5, 0x2a2a30);
    fill(cx + 2, y - 1, 4, 1, tape); for (let k = 0; k < 4; k++) dot(cx + 2 + k, y - 1, mark); // the hanging end with ticks
    fill(cx - 5, y - 2, 2, 2, chalk); dot(cx - 5, y - 2, WHITE, 0.6);           // the chalk
    fill(cx - 5, y, 11, 1, SHADOW);
  },
  /** A Wonka chocolate bar in a purple-and-gold wrapper, a corner of golden ticket peeking out. */
  wonkabar(x, y) {
    const wrap = 0x8a4ac0, wrapL = 0xaa74e0, gold = 0xe8c838, goldL = 0xf8e488, choc = 0x4a2814;
    const cx = x + 39;
    fill(cx - 5, y - 4, 12, 4, wrap); fill(cx - 5, y - 4, 12, 1, wrapL); fill(cx - 5, y, 12, 1, SHADOW);
    fill(cx - 4, y - 3, 10, 2, gold); fill(cx - 4, y - 3, 10, 1, goldL);        // the gold band
    fill(cx + 3, y - 6, 4, 3, goldL); fill(cx + 3, y - 6, 4, 1, 0xfff0a0); dot(cx + 5, y - 5, gold); // the peeking ticket
    fill(cx - 4, y - 2, 2, 1, choc);
  },
  /** A golden ticket standing upright, shimmering; it glows warmly at night. */
  wonkaticket(x, y) {
    const gold = 0xe8c838, goldL = 0xf8e488, goldD = 0xb08818, ink = 0x8a1a1a;
    const cx = x + 40;
    fill(cx - 5, y - 8, 11, 8, gold); fill(cx - 5, y - 8, 11, 1, goldL); fill(cx - 5, y - 1, 11, 1, goldD); fill(cx - 5, y, 11, 1, SHADOW);
    fill(cx - 4, y - 6, 9, 1, ink, 0.7); fill(cx - 4, y - 4, 7, 1, ink, 0.5); fill(cx - 4, y - 2, 5, 1, ink, 0.5);
    const sh = Math.floor(now * 3) % 11; dot(cx - 5 + sh, y - 8, WHITE, 0.8);   // the shimmer
    if (isNight) glowLate(() => glowAt(cx, y - 4, 7, 5, gold, 0.2));
  },
  /** An everlasting gobstopper on a stand, its colours slowly cycling; a soft glow at night. */
  wonkagobstopper(x, y) {
    const cols = [0xf06aa8, 0x5ad0a0, 0x8a4ac0, 0xe8c838, 0xd83050];
    const cx = x + 40, i = Math.floor(now * 0.8) % cols.length, c = cols[i], cL = shaded(c, 1.2);
    fill(cx - 1, y - 2, 3, 2, 0x3a3a42); fill(cx - 2, y, 5, 1, SHADOW);
    disc(cx, y - 6, 4, c); disc(cx - 1, y - 7, 1.6, cL); ring(cx, y - 6, 4, shaded(c, 0.8), 0.5);
    dot(cx - 2, y - 8, WHITE, 0.7);
    if (isNight) glowLate(() => glowAt(cx, y - 6, 5, 5, c, 0.2));
  },
  /** A giant swirl lollipop in a holder, its spiral turning slowly. */
  wonkalollipop(x, y) {
    const pink = 0xf06aa8, white = 0xfff2f6, stick = 0xeae0cc;
    const cx = x + 40, cy = y - 8;
    fill(cx, y - 5, 1, 5, stick); fill(cx - 2, y, 5, 1, SHADOW);
    disc(cx, cy, 4, white); const a = now * 1.2;
    for (let k = 0; k < 8; k++) { const t = a + k * 0.7; dot(cx + Math.round(Math.cos(t) * (k * 0.45)), cy + Math.round(Math.sin(t) * (k * 0.45)), pink); }
    ring(cx, cy, 4, 0xd04a88, 0.5); dot(cx - 2, cy - 2, WHITE, 0.7);
  },
  /** A glass of fizzy lifting drink, green and glowing, bubbles streaming up and off the top. */
  wonkafizzy(x, y) {
    const glass = 0xdff0f6, fizz = 0x5ad0a0, fizzL = 0x9ff0c8;
    const cx = x + 40;
    fill(cx - 3, y - 8, 7, 8, glass, 0.4); fill(cx - 3, y - 8, 1, 8, glass, 0.7); fill(cx + 3, y - 8, 1, 8, 0xb8cdd6, 0.6);
    fill(cx - 3, y - 6, 7, 6, fizz, 0.8); fill(cx - 3, y - 6, 7, 1, fizzL); fill(cx - 3, y, 7, 1, SHADOW);
    for (let k = 0; k < 4; k++) { const ph = (now * 1.5 + k / 4) % 1; dot(cx - 2 + (k * 2) % 6, y - 6 - Math.round(ph * 7), fizzL, 1 - ph); }
    if (isNight) glowLate(() => glowAt(cx, y - 4, 5, 6, fizz, 0.18));
  },
  /** A red-and-white candy cane hooked over a little stand. */
  wonkacane(x, y) {
    const red = 0xd83050, white = 0xfff2f6, stand = 0x4a2814;
    const cx = x + 40;
    fill(cx - 2, y - 1, 2, 1, stand); fill(cx - 2, y, 5, 1, SHADOW);
    for (let yy = 0; yy < 9; yy++) dot(cx, y - 1 - yy, (yy % 2) ? red : white);  // the shaft
    dot(cx - 1, y - 10, white); dot(cx - 2, y - 9, red); dot(cx - 2, y - 8, white); // the hook
  },
  /** Wonka's purple top hat on a stand, a gold band round the brim. */
  wonkahat(x, y) {
    const hat = 0x8a4ac0, hatL = 0xaa74e0, hatD = 0x5a2e84, gold = 0xe8c838;
    const cx = x + 40;
    fill(cx - 5, y - 2, 11, 2, hat); fill(cx - 5, y - 2, 11, 1, hatL); fill(cx - 5, y, 11, 1, SHADOW);  // the brim
    fill(cx - 3, y - 10, 7, 8, hat); fill(cx - 3, y - 10, 1, 8, hatL); fill(cx + 3, y - 10, 1, 8, hatD);
    fill(cx - 3, y - 4, 7, 1, gold); fill(cx - 3, y - 10, 7, 1, hatD);           // the band + top
    dot(cx - 2, y - 9, WHITE, 0.4);
  },
  /** A foil stick of three-course-dinner gum, the wrapper half peeled. */
  wonkagum(x, y) {
    const foil = 0xc0c8d0, foilL = 0xe4e8ee, gum = 0xc86a9a, label = 0xd83050;
    const cx = x + 40;
    fill(cx - 4, y - 3, 10, 3, foil); fill(cx - 4, y - 3, 10, 1, foilL); fill(cx - 4, y, 10, 1, SHADOW);
    fill(cx + 1, y - 3, 5, 3, gum); fill(cx + 1, y - 3, 5, 1, 0xe0a0c0);        // the exposed gum
    fill(cx - 4, y - 4, 5, 1, label);                                           // the label edge
    dot(cx + 5, y - 4, foilL);
  },
  /** A brass reception bell on a pink saucer; it gives a little shiver as if just rung. */
  wesbell(x, y) {
    const brass = 0xc8a24a, brassL = 0xe8c878, brassD = 0x8a6a2a, saucer = 0xe8a0a8;
    const cx = x + 40, ring = now % 7 < 0.3 ? 1 : 0;
    fill(cx - 4, y - 1, 9, 1, saucer); fill(cx - 4, y, 9, 1, SHADOW);
    fill(cx - 3 + ring, y - 5, 7, 4, brass); fill(cx - 3 + ring, y - 5, 7, 1, brassL); disc(cx + ring, y - 6, 2, brass);
    dot(cx + ring, y - 8, brassD); dot(cx - 1 + ring, y - 4, WHITE, 0.6);
  },
  /** A brass room key with a round tasselled fob, hanging from a little hook. */
  weskey(x, y) {
    const brass = 0xc8a24a, brassL = 0xe8c878, fob = 0x7a2a3a, tassel = 0xd8a848;
    const cx = x + 40;
    fill(cx - 1, y - 9, 1, 2, 0x8a8e96);                                        // the hook
    disc(cx, y - 6, 2.4, fob); dot(cx - 1, y - 7, 0x9a4a5a);                     // the fob
    fill(cx, y - 3, 1, 3, brass); fill(cx - 1, y, 2, 1, brass); fill(cx + 1, y - 1, 1, 1, brass); // the key
    for (let k = 0; k < 3; k++) dot(cx, y + k - 9 + 9, tassel);
    fill(cx - 2, y + 1, 5, 1, SHADOW);
  },
  /** A Mendl's-style pink pastry box tied with blue ribbon, a little courtesan cake on top. */
  wescake(x, y) {
    const box = 0xf0b8c4, boxL = 0xf8d0d8, ribbon = 0x9ac0d0, cream = 0xf2ead2, choc = 0x8a5038, pink = 0xe87898;
    const cx = x + 40;
    fill(cx - 5, y - 4, 11, 4, box); fill(cx - 5, y - 4, 11, 1, boxL); fill(cx - 5, y, 11, 1, SHADOW);
    fill(cx, y - 4, 1, 4, ribbon); fill(cx - 5, y - 3, 11, 1, ribbon, 0.7);     // the ribbon cross
    disc(cx - 1, y - 7, 2, cream); disc(cx - 1, y - 8, 1.4, choc); dot(cx - 1, y - 9, pink); // the little cake
  },
  /** A cream Bakelite valve radio with a mustard dial that glows warm when it's on. */
  wesradio(x, y) {
    const body = 0xe8dcc0, bodyL = 0xf2ead2, bodyD = 0xc0b088, dial = 0xd8a848, grille = 0x8a6a4a;
    const cx = x + 39;
    fill(cx - 5, y - 7, 12, 7, body); fill(cx - 5, y - 7, 12, 1, bodyL); fill(cx + 6, y - 7, 1, 7, bodyD); fill(cx - 5, y, 12, 1, SHADOW);
    for (let k = 0; k < 4; k++) fill(cx - 4, y - 6 + k * 1.5, 7, 1, grille, 0.6); // the speaker grille
    disc(cx + 4, y - 4, 2, dial); dot(cx + 4, y - 4, 0x8a6a2a);
    if (isNight) glowLate(() => glowAt(cx + 4, y - 4, 2, 2, 0xf2d06a, 0.26));
  },
  /** A black-and-brass candlestick telephone, the receiver hooked on its side. */
  wesphone(x, y) {
    const stem = 0x2a2a30, stemL = 0x44444c, brass = 0xc8a24a, base = 0x1a1a1e;
    const cx = x + 40;
    disc(cx, y - 1, 3, base); fill(cx - 3, y, 7, 1, SHADOW);                     // the base
    fill(cx - 1, y - 8, 2, 7, stem); fill(cx - 1, y - 8, 1, 7, stemL);           // the stem
    fill(cx - 2, y - 10, 5, 2, stem); fill(cx - 2, y - 10, 5, 1, brass);         // the mouthpiece cup
    fill(cx + 2, y - 7, 1, 4, stem); dot(cx + 3, y - 8, brass);                  // the receiver
  },
  /** A slim hardback novel in a dusty-rose dust jacket, a mustard bookmark ribbon trailing. */
  wesbook(x, y) {
    const cover = 0xd87a8a, coverL = 0xe89aa6, coverD = 0xa85060, pages = 0xf2ead2, mark = 0xd8a848;
    const cx = x + 40;
    fill(cx - 4, y - 7, 9, 7, cover); fill(cx - 4, y - 7, 1, 7, coverL); fill(cx + 4, y - 7, 1, 7, coverD); fill(cx - 4, y, 9, 1, SHADOW);
    fill(cx + 4, y - 6, 1, 6, pages);                                            // the page edge
    fill(cx - 2, y - 5, 5, 1, pages, 0.7); fill(cx - 2, y - 3, 3, 1, pages, 0.5); // the title
    fill(cx + 2, y - 7, 1, 3, mark);                                             // the bookmark
  },
  /** A pair of mustard field binoculars, lenses catching a glint. */
  wesbinoculars(x, y) {
    const body = 0xd8a848, bodyL = 0xe8c070, bodyD = 0x9a7428, lens = 0x9ac0d0;
    const cx = x + 40;
    fill(cx - 4, y - 5, 4, 5, body); fill(cx + 1, y - 5, 4, 5, body); fill(cx - 4, y - 5, 1, 5, bodyL); fill(cx + 4, y - 5, 1, 5, bodyD);
    fill(cx, y - 4, 1, 3, bodyD); fill(cx - 4, y, 9, 1, SHADOW);                  // the bridge
    dot(cx - 3, y - 4, lens); dot(cx + 2, y - 4, lens); dot(cx - 3, y - 4, WHITE, 0.5);
  },
  /** A tall faceted perfume bottle of pale rose scent with a tasselled atomiser bulb. */
  wesperfume(x, y) {
    const glass = 0xf0b8c4, glassL = 0xf8d8e0, cap = 0xc8a24a, bulb = 0x7a2a3a;
    const cx = x + 40;
    fill(cx - 2, y - 7, 5, 7, glass, 0.85); fill(cx - 2, y - 7, 1, 7, glassL); fill(cx - 2, y, 5, 1, SHADOW);
    fill(cx - 1, y - 9, 3, 2, cap); fill(cx - 1, y - 9, 3, 1, 0xe8c878);         // the cap
    disc(cx + 4, y - 7, 1.6, bulb); dot(cx + 3, y - 8, 0x9a4a5a);                // the atomiser bulb
    dot(cx - 1, y - 5, WHITE, 0.6);
  },
  /** A silver neuralyzer standing on end; every so often its tip flares a bright white flash. */
  mibneuralyzer(x, y) {
    const steel = 0xc0c6ce, steelL = 0xe4e8ee, steelD = 0x8a8e96, tip = 0xffffff;
    const cx = x + 40, t = (now + x * 0.3) % 8, flash = t < 0.4;
    fill(cx - 1, y - 2, 3, 2, 0x3a3e46); fill(cx - 2, y, 5, 1, SHADOW);
    fill(cx - 1, y - 10, 3, 8, steel); fill(cx - 1, y - 10, 1, 8, steelL); fill(cx + 1, y - 10, 1, 8, steelD);
    fill(cx - 1, y - 7, 3, 1, steelD); dot(cx, y - 5, flash ? tip : 0x6a7078);   // the dial
    if (flash) { disc(cx, y - 11, 2, tip); glowAt(cx, y - 11, 6, 6, tip, 0.6); }  // the flash
    else dot(cx, y - 11, 0x9aa0a8);
    if (isNight && flash) glowLate(() => glowAt(cx, y - 11, 8, 8, tip, 0.5));
  },
  /** A pair of black-framed Ray-Ban-style shades folded on the desk, a cool glint on each lens. */
  mibshades(x, y) {
    const frame = 0x15151a, frameL = 0x33333a, lens = 0x2a2e36, glint = 0x9ac0e0;
    const cx = x + 39;
    fill(cx - 5, y - 3, 5, 3, frame); fill(cx + 1, y - 3, 5, 3, frame); fill(cx - 1, y - 3, 2, 1, frame); // lenses + bridge
    fill(cx - 5, y - 3, 5, 1, frameL); fill(cx + 1, y - 3, 5, 1, frameL);
    fill(cx - 4, y - 2, 3, 1, lens); fill(cx + 2, y - 2, 3, 1, lens);
    dot(cx - 4, y - 3, glint); dot(cx + 2, y - 3, glint);
    fill(cx - 5, y, 11, 1, SHADOW);
  },
  /** A silver agency badge on a dark leather holder, a small galaxy emblem engraved on it. */
  mibbadge(x, y) {
    const leather = 0x1a1a1e, steel = 0xc0c6ce, steelL = 0xe4e8ee, galaxy = 0x3a3060;
    const cx = x + 40;
    fill(cx - 4, y - 6, 9, 6, leather); fill(cx - 4, y - 6, 9, 1, 0x2e2e34); fill(cx - 4, y, 9, 1, SHADOW);
    disc(cx, y - 3, 2.6, steel); ring(cx, y - 3, 2.6, steelL, 0.6); disc(cx, y - 3, 1.4, galaxy);
    dot(cx - 1, y - 4, steelL); dot(cx, y - 3, 0xbfd0ff);
  },
  /** A glass jar with a big brown alien bug clicking about inside; a green glow at night. */
  mibbugjar(x, y) {
    const glass = 0xdff0f6, lid = 0x8a8e96, bug = 0x5a3a24, bugL = 0x7a5236, alienG = 0x5ad86a;
    const cx = x + 40;
    fill(cx - 4, y - 9, 9, 9, glass, 0.4); fill(cx - 4, y - 9, 1, 9, glass, 0.7); fill(cx + 4, y - 9, 1, 9, 0xb8cdd6, 0.6);
    fill(cx - 4, y - 11, 9, 2, lid); fill(cx - 4, y - 11, 9, 1, 0xb4b8c0); fill(cx - 4, y, 9, 1, SHADOW);
    const sk = Math.floor(now * 4 + x) % 3 - 1; fill(cx - 2 + sk, y - 4, 4, 3, bug); fill(cx - 2 + sk, y - 4, 4, 1, bugL); // the bug
    dot(cx - 2 + sk, y - 5, bug); dot(cx + 2 + sk, y - 5, bug); dot(cx + sk, y - 4, alienG); // legs + eye
    if (isNight) glowLate(() => glowAt(cx, y - 5, 5, 5, alienG, 0.14));
  },
  /** A tiny galaxy held in a glass marble on a stand, slowly turning; it glows at night. */
  mibgalaxy(x, y) {
    const glass = 0xcfe0ea, core = 0x2a2060, star = 0xbfd0ff, arm = 0x6a4aa0;
    const cx = x + 40, cy = y - 6;
    fill(cx - 1, y - 2, 3, 2, 0x3a3e46); fill(cx - 2, y, 5, 1, SHADOW);
    disc(cx, cy, 4, core); ring(cx, cy, 4, glass, 0.5);
    const a = now * 0.8; for (let k = 0; k < 6; k++) { const t = a + k * 1.0, r = (k % 3) + 1; dot(cx + Math.round(Math.cos(t) * r), cy + Math.round(Math.sin(t) * r * 0.6), k % 2 ? star : arm); }
    dot(cx, cy, star); dot(cx - 2, cy - 2, WHITE, 0.6);
    if (isNight) glowLate(() => glowAt(cx, cy, 5, 4, arm, 0.2));
  },
  /** The worm guys' coffee: a tall mug with three little pink worms peeking over the rim and a wisp of steam. */
  mibcoffee(x, y) {
    const mug = 0xe8eaee, mugD = 0xb8bcc2, coffee = 0x4a3020, worm = 0xe89aa6, wormD = 0xc87888;
    const cx = x + 40;
    fill(cx - 3, y - 6, 7, 6, mug); fill(cx - 3, y - 6, 1, 6, 0xf4f6f8); fill(cx + 3, y - 6, 1, 6, mugD); fill(cx - 3, y, 7, 1, SHADOW);
    fill(cx + 4, y - 5, 2, 3, mug); fill(cx + 4, y - 4, 1, 1, 0x00000000);       // the handle
    fill(cx - 2, y - 7, 5, 1, coffee);
    for (let k = 0; k < 3; k++) { const wob = Math.floor(now * 3 + k) % 2; const wx = cx - 2 + k * 2; fill(wx, y - 9 - wob, 1, 2, worm); dot(wx, y - 9 - wob, wormD); } // the worms
    if (now % 3 < 1.5) dot(cx, y - 10, WHITE, 0.4);                              // steam
  },
  /** A potted alien plant: a bulb on a stalk that opens and closes, glowing faintly green. */
  mibalienplant(x, y) {
    const pot = 0x3a3e46, stalk = 0x3a7a4a, bulb = 0x5ad86a, bulbL = 0x9ff0a8, spot = 0xd83050;
    const cx = x + 40;
    fill(cx - 2, y - 3, 5, 3, pot); fill(cx - 2, y - 3, 5, 1, 0x4a4e56); fill(cx - 2, y, 5, 1, SHADOW);
    fill(cx, y - 8, 1, 5, stalk);
    const open = (Math.floor(now * 1.5) % 4 < 2) ? 1 : 0;
    disc(cx, y - 10 + open, 2 + open, bulb); dot(cx, y - 10 + open, open ? spot : bulbL); dot(cx - 1, y - 11, bulbL, 0.6);
    if (isNight) glowLate(() => glowAt(cx, y - 9, 4, 4, bulb, 0.18));
  },
  /** A little locket on a chain, open to show a whole galaxy inside -- the fate of the universe, in a snow-globe. */
  mibglobe(x, y) {
    const metal = 0xc8a24a, metalL = 0xe8c878, dome = 0xcfe0ea, core = 0x2a2060, star = 0xbfd0ff;
    const cx = x + 40;
    fill(cx - 1, y - 2, 3, 2, metal); fill(cx - 2, y, 5, 1, SHADOW);
    disc(cx, y - 6, 4, dome, 0.5); disc(cx, y - 6, 3, core); ring(cx, y - 6, 4, metalL, 0.7);
    for (let k = 0; k < 4; k++) { const a = now * 0.6 + k * 1.6; dot(cx + Math.round(Math.cos(a) * 2), y - 6 + Math.round(Math.sin(a) * 1.4), star); }
    dot(cx, y - 6, star); dot(cx - 2, y - 8, WHITE, 0.6);
    if (isNight) glowLate(() => glowAt(cx, y - 6, 4, 4, 0x6a4aa0, 0.16));
  },
  /** An identity disc on a stand, its cyan rim pulsing; it glows brighter at night. */
  tronidentity(x, y) {
    const dark = 0x14202c, cyan = 0x3ad0e8, cyanL = 0x9ff0ff;
    const cx = x + 40, cy = y - 7, pulse = 0.7 + 0.3 * Math.sin(now * 3 + x);
    fill(cx - 1, y - 3, 3, 3, 0x1a2a38); fill(cx - 2, y, 5, 1, SHADOW);
    disc(cx, cy, 4, dark); ring(cx, cy, 4, cyan, pulse); ring(cx, cy, 2, cyanL, pulse * 0.8); dot(cx, cy, cyanL);
    glowAt(cx, cy, 5, 5, cyan, (isNight ? 0.24 : 0.12) * pulse);
    if (isNight) glowLate(() => glowAt(cx, cy, 7, 7, cyan, 0.3 * pulse));
  },
  /** A model light-cycle, low and sleek, a cyan light ribbon trailing from its tail. */
  troncycle(x, y) {
    const body = 0x16222e, cyan = 0x3ad0e8, cyanL = 0x9ff0ff, wheel = 0x0e1620;
    const cx = x + 38;
    fill(cx, y - 4, 11, 3, body); fill(cx + 2, y - 6, 6, 2, body); fill(cx, y - 4, 11, 1, 0x24323e);
    fill(cx - 4, y - 2, 5, 1, cyan); dot(cx - 5, y - 2, cyanL);                  // the light trail
    fill(cx + 1, y - 5, 9, 1, cyan, 0.9);                                        // the body light line
    disc(cx + 1, y - 1, 1.6, wheel); disc(cx + 9, y - 1, 1.6, wheel); ring(cx + 1, y - 1, 1.6, cyan, 0.7); ring(cx + 9, y - 1, 1.6, cyan, 0.7);
    fill(cx, y, 11, 1, SHADOW);
    if (isNight) glowLate(() => glowAt(cx + 5, y - 3, 7, 3, cyan, 0.22));
  },
  /** A light-cycle baton upright in a holder, a cyan seam running up it that flickers. */
  tronbaton(x, y) {
    const dark = 0x1a2630, cyan = 0x3ad0e8, cyanL = 0x9ff0ff;
    const cx = x + 40, fl = 0.7 + 0.3 * Math.sin(now * 5 + x);
    fill(cx - 1, y - 2, 3, 2, 0x14202c); fill(cx - 2, y, 5, 1, SHADOW);
    fill(cx - 1, y - 11, 3, 9, dark); fill(cx, y - 11, 1, 9, cyan, fl); dot(cx, y - 11, cyanL);
    if (isNight) glowLate(() => glowAt(cx, y - 7, 3, 6, cyan, 0.2 * fl));
  },
  /** A tall glass of glowing energy drink, cyan and luminous, a bright meniscus at the top. */
  tronenergy(x, y) {
    const glass = 0xbfe4ea, energy = 0x3ad0e8, energyL = 0x9ff0ff;
    const cx = x + 40;
    fill(cx - 3, y - 9, 7, 9, glass, 0.35); fill(cx - 3, y - 9, 1, 9, glass, 0.6); fill(cx + 3, y - 9, 1, 9, 0x7aa0b0, 0.6);
    fill(cx - 3, y - 6, 7, 6, energy, 0.85); fill(cx - 3, y - 6, 7, 1, energyL); fill(cx - 3, y, 7, 1, SHADOW);
    dot(cx + 1, y - 8, WHITE, 0.6);
    if (isNight) glowLate(() => glowAt(cx, y - 4, 5, 6, energy, 0.22));
  },
  /** A black circuit chip with glowing cyan traces pulsing across it. */
  tronchip(x, y) {
    const chip = 0x10161e, pin = 0x6a6e76, trace = 0x3ad0e8;
    const cx = x + 39;
    fill(cx - 5, y - 4, 11, 4, chip); fill(cx - 5, y - 4, 11, 1, 0x20262e); fill(cx - 5, y, 11, 1, SHADOW);
    for (let k = -5; k < 6; k += 2) { dot(cx + k, y - 5, pin); dot(cx + k, y, pin); }
    const n = Math.floor(now * 3); for (let k = 0; k < 4; k++) { const on = (k + n) % 3 === 0; fill(cx - 4 + k * 3, y - 2, 2, 1, on ? trace : 0x1a4a58); }
    if (isNight) glowLate(() => glowAt(cx, y - 2, 6, 3, trace, 0.14));
  },
  /** A black grid helmet with a glowing cyan visor line, resting on the desk. */
  tronhelmet(x, y) {
    const shell = 0x14202c, shellL = 0x24323e, cyan = 0x3ad0e8, cyanL = 0x9ff0ff;
    const cx = x + 40;
    fill(cx - 4, y - 6, 9, 6, shell); fill(cx - 4, y - 6, 9, 1, shellL); fill(cx - 4, y - 1, 9, 1, 0x0a1016); fill(cx - 4, y, 9, 1, SHADOW);
    fill(cx - 3, y - 4, 7, 1, cyan); dot(cx - 3, y - 4, cyanL); dot(cx + 3, y - 4, cyanL); // the visor
    fill(cx - 4, y - 6, 1, 6, cyan, 0.6); fill(cx + 4, y - 6, 1, 6, cyan, 0.6);  // side seams
    if (isNight) glowLate(() => glowAt(cx, y - 4, 6, 3, cyan, 0.2));
  },
  /** A floating data cube hovering over a pad, slowly rotating its glowing cyan edges. */
  troncube(x, y) {
    const edge = 0x3ad0e8, edgeL = 0x9ff0ff, face = 0x163040, pad = 0x14202c;
    const cx = x + 40, hov = Math.round(Math.sin(now * 2 + x)), cy = y - 7 + hov;
    fill(cx - 3, y - 1, 7, 1, pad); fill(cx - 2, y, 5, 1, SHADOW);
    fill(cx - 2, cy - 2, 5, 5, face); fill(cx - 2, cy - 2, 5, 1, edge); fill(cx - 2, cy + 2, 5, 1, edge, 0.7); fill(cx - 2, cy - 2, 1, 5, edge); fill(cx + 2, cy - 2, 1, 5, edge, 0.7);
    dot(cx - 2, cy - 2, edgeL); dot(cx + 2, cy - 2, edgeL);
    glowAt(cx, cy, 4, 4, edge, isNight ? 0.2 : 0.1);
    if (isNight) glowLate(() => glowAt(cx, cy, 5, 5, edge, 0.26));
  },
  /** A brass arcade token standing on edge, an old quarter-slot nostalgia; a faint amber glint. */
  troncoin(x, y) {
    const brass = 0xc8a24a, brassL = 0xe8c878, brassD = 0x8a6a2a, cyan = 0x3ad0e8;
    const cx = x + 40;
    fill(cx - 1, y - 2, 3, 2, 0x14202c); fill(cx - 2, y, 5, 1, SHADOW);
    disc(cx, y - 6, 3.4, brass); ring(cx, y - 6, 3.4, brassD, 0.6); disc(cx - 1, y - 7, 1.2, brassL);
    dot(cx, y - 6, cyan); dot(cx - 2, y - 8, WHITE, 0.6);
  },
  /** The four-star Dragon Ball: a glowing orange orb with four red stars, on a little stand. */
  dbzdragonball(x, y) {
    const orb = 0xf2a83a, orbL = 0xffd080, orbD = 0xc87a18, star = 0xd83020;
    const cx = x + 40, cy = y - 6;
    fill(cx - 1, y - 2, 3, 2, 0x6a4a2a); fill(cx - 2, y, 5, 1, SHADOW);
    disc(cx, cy, 4, orb); disc(cx - 1, cy - 1, 2, orbL); ring(cx, cy, 4, orbD, 0.5);
    for (const [sx, sy] of [[-1, -1], [1, -1], [-1, 1], [1, 1]]) dot(cx + sx, cy + sy, star);
    dot(cx - 2, cy - 2, WHITE, 0.7);
    if (isNight) glowLate(() => glowAt(cx, cy, 5, 5, orb, 0.2));
  },
  /** A Capsule Corp capsule: a little chrome-and-blue pod; press it and it puffs a tiny cloud. */
  dbzcapsule(x, y) {
    const chrome = 0xcfd6dd, chromeD = 0x8a9098, blue = 0x3a6ac0, cap = 0xd83020, smoke = 0xdfe3ea;
    const cx = x + 40, t = (now + x * 0.3) % 6, puff = t < 0.8;
    fill(cx - 3, y - 3, 7, 3, chrome); fill(cx - 3, y - 3, 7, 1, 0xe4e8ee); fill(cx + 3, y - 3, 1, 3, chromeD); fill(cx - 3, y, 7, 1, SHADOW);
    fill(cx - 3, y - 2, 7, 1, blue); dot(cx, y - 4, cap);                        // the band + button
    if (puff) { fill(cx - 2, y - 6, 5, 2, smoke, 0.6); dot(cx, y - 8, smoke, 0.4); }
  },
  /** A little drawstring pouch of senzu beans, a few green beans spilling out. */
  dbzsenzu(x, y) {
    const pouch = 0xc8a060, pouchD = 0x9a7438, bean = 0x5ad04a, beanD = 0x3a9a2e, tie = 0x8a5a32;
    const cx = x + 40;
    fill(cx - 3, y - 5, 7, 5, pouch); fill(cx - 3, y - 5, 7, 1, 0xd8b478); fill(cx + 3, y - 5, 1, 5, pouchD); fill(cx - 3, y, 7, 1, SHADOW);
    fill(cx - 2, y - 6, 5, 1, tie); dot(cx, y - 7, tie);
    for (const [bx, by] of [[4, -3], [5, -2], [4, -1]]) { dot(cx + bx, y + by, bean); dot(cx + bx, y + by + 1, beanD); } // spilled beans
  },
  /** A green scouter eyepiece on a stand; its lens flickers as it reads a power level. */
  dbzscouter(x, y) {
    const body = 0x2a2e36, bodyL = 0x44484e, lens = 0x5ad04a, lensL = 0x9ff0a8;
    const cx = x + 40;
    fill(cx - 1, y - 2, 3, 2, 0x3a3e46); fill(cx - 2, y, 5, 1, SHADOW);
    fill(cx - 3, y - 6, 7, 2, body); fill(cx - 3, y - 6, 7, 1, bodyL);           // the ear piece
    fill(cx + 1, y - 5, 4, 3, lens, 0.7); fill(cx + 1, y - 5, 4, 1, lensL);       // the lens
    const fl = Math.floor(now * 4) % 3; dot(cx + 2 + fl, y - 4, lensL);
    if (isNight) glowLate(() => glowAt(cx + 3, y - 4, 3, 2, lens, 0.22));
  },
  /** A Dragon Radar: a round green handheld with concentric rings and an orange blip pinging. */
  dbzradar(x, y) {
    const body = 0xdfe3ea, bodyD = 0xa8acb2, screen = 0x143a24, rc = 0x3aa84a, blip = 0xf2a83a;
    const cx = x + 40;
    disc(cx, y - 5, 4, body); ring(cx, y - 5, 4, bodyD, 0.6); disc(cx, y - 5, 3, screen);
    ring(cx, y - 5, 2.4, rc, 0.6); ring(cx, y - 5, 1.2, rc, 0.5);
    const on = Math.floor(now * 2) % 2; if (on) { const a = now * 2; dot(cx + Math.round(Math.cos(a) * 2), y - 5 + Math.round(Math.sin(a) * 2), blip); }
    fill(cx - 2, y - 1, 5, 1, bodyD); fill(cx - 2, y, 5, 1, SHADOW);
    if (isNight) glowLate(() => glowAt(cx, y - 5, 4, 4, rc, 0.14));
  },
  /** A World Martial Arts Tournament trophy: a gold cup with winged handles on a dark base. */
  dbztrophy(x, y) {
    const gold = 0xe8c050, goldL = 0xf8e488, goldD = 0xa8842a, base = 0x3a2a1a;
    const cx = x + 40;
    fill(cx - 3, y - 2, 7, 2, base); fill(cx - 3, y, 7, 1, SHADOW);
    fill(cx - 1, y - 4, 3, 2, gold); fill(cx - 3, y - 9, 7, 5, gold); fill(cx - 3, y - 9, 7, 1, goldL); fill(cx + 3, y - 9, 1, 5, goldD);
    fill(cx - 5, y - 8, 2, 2, goldL); fill(cx + 4, y - 8, 2, 2, goldL);          // the winged handles
    dot(cx - 1, y - 7, WHITE, 0.6);
  },
  /** A big steaming bowl of noodles with chopsticks -- Goku's favourite; a wisp of steam rises. */
  dbznoodles(x, y) {
    const bowl = 0xd83020, bowlL = 0xf05a40, bowlD = 0xa01e12, noodle = 0xf2d888, chop = 0xc8a060;
    const cx = x + 40;
    fill(cx - 4, y - 4, 9, 4, bowl); fill(cx - 4, y - 4, 9, 1, bowlL); fill(cx - 4, y - 1, 9, 1, bowlD); fill(cx - 4, y, 9, 1, SHADOW);
    fill(cx - 3, y - 5, 7, 1, noodle); dot(cx - 1, y - 6, noodle); dot(cx + 1, y - 6, noodle);
    fill(cx + 2, y - 9, 1, 5, chop); fill(cx + 3, y - 9, 1, 5, chop);            // the chopsticks
    if (now % 3 < 1.5) dot(cx - 1, y - 8, WHITE, 0.4);
  },
  /** A neatly folded orange training gi with a blue belt and wristbands set on top. */
  dbzgifold(x, y) {
    const gi = 0xe87828, giL = 0xf89a48, giD = 0xc85a10, blue = 0x2a50c0;
    const cx = x + 39;
    fill(cx - 5, y - 4, 11, 4, gi); fill(cx - 5, y - 4, 11, 1, giL); fill(cx - 5, y - 1, 11, 1, giD); fill(cx - 5, y, 11, 1, SHADOW);
    fill(cx - 5, y - 2, 11, 1, blue);                                            // the belt
    fill(cx - 2, y - 7, 5, 3, gi); fill(cx - 2, y - 7, 5, 1, giL);               // the folded top
    dot(cx - 4, y - 6, blue); dot(cx + 4, y - 6, blue);                          // wristbands
  },
  /** A little crewmate figurine, its suit slowly cycling through the crew colours; a light visor glint. */
  amonguscrewmate(x, y) {
    const cols = [0xd83020, 0x3a6ad8, 0x4ad04a, 0xf2d84a, 0xf06ab0], c = cols[Math.floor(now * 0.6) % cols.length], cL = shaded(c, 1.2), cD = shaded(c, 0.78), visor = 0x9fd8e8;
    const cx = x + 40;
    fill(cx - 3, y - 9, 6, 9, c); fill(cx - 3, y - 9, 1, 9, cL); fill(cx + 2, y - 9, 1, 9, cD); fill(cx - 3, y - 9, 5, 1, cL); // the body
    fill(cx - 4, y - 7, 1, 5, cD);                                               // the backpack
    fill(cx, y - 7, 3, 2, visor); fill(cx, y - 7, 3, 1, 0xd8f0f8); dot(cx + 1, y - 7, WHITE, 0.7); // the visor
    fill(cx - 3, y - 1, 2, 1, cD); fill(cx, y - 1, 2, 1, cD); fill(cx - 3, y, 6, 1, SHADOW); // legs
  },
  /** The emergency meeting button under a glass dome; it flashes red when pressed. */
  amongusbutton(x, y) {
    const base = 0x6a7682, baseD = 0x44505a, dome = 0xbfe0ea, red = 0xd83020, redL = 0xf05a40;
    const cx = x + 40, t = (now + x * 0.3) % 7, press = t < 0.6;
    fill(cx - 4, y - 2, 9, 2, base); fill(cx - 4, y - 2, 9, 1, 0x8a96a2); fill(cx - 4, y, 9, 1, SHADOW);
    disc(cx, y - 5, 3, press ? redL : red); ring(cx, y - 5, 3, 0xa01e12, 0.6); dot(cx - 1, y - 6, press ? WHITE : redL);
    ring(cx, y - 5, 4, dome, 0.4);                                               // the glass dome
    if (press) glowAt(cx, y - 5, 4, 4, red, isNight ? 0.3 : 0.16);
  },
  /** A task console with a card slot; a swipe card slides through now and then, the light turning green. */
  amongustaskpad(x, y) {
    const body = 0x3a4450, bodyL = 0x56626e, slot = 0x14181e, card = 0xf2d84a, green = 0x4ad04a, red = 0xd83020;
    const cx = x + 39;
    fill(cx - 5, y - 6, 11, 6, body); fill(cx - 5, y - 6, 11, 1, bodyL); fill(cx - 5, y, 11, 1, SHADOW);
    fill(cx - 4, y - 5, 9, 1, slot);                                             // the card slot
    const t = (now + x * 0.3) % 5, sw = t < 1.2, px = cx - 5 + Math.round((t / 1.2) * 10);
    if (sw) fill(px, y - 6, 3, 2, card);                                         // the swiping card
    dot(cx + 4, y - 3, (sw || t < 2) ? green : red);
    if (isNight) glowLate(() => glowAt(cx + 4, y - 3, 2, 2, (sw || t < 2) ? green : red, 0.2));
  },
  /** A wiring task panel: four coloured nodes with little jumper wires arcing between them. */
  amonguswires(x, y) {
    const panel = 0x2a333c, panelL = 0x44505a, cols = [0xd83020, 0x3a6ad8, 0xf2d84a, 0x4ad04a];
    const cx = x + 39;
    fill(cx - 5, y - 7, 11, 7, panel); fill(cx - 5, y - 7, 11, 1, panelL); fill(cx - 5, y, 11, 1, SHADOW);
    for (let k = 0; k < 4; k++) { const ly = y - 6 + k * 1.5, ry = y - 6 + ((k + 1 + Math.floor(now)) % 4) * 1.5; dot(cx - 4, Math.round(ly), cols[k]); dot(cx + 4, Math.round(ry), cols[k]); line(cx - 4, Math.round(ly), cx + 4, Math.round(ry), cols[k], 0.7); }
  },
  /** A chunky fuel canister with a spout and a yellow hazard stripe. */
  amongusfuel(x, y) {
    const can = 0xd8a030, canL = 0xf0c050, canD = 0xa87818, spout = 0x6a7682, stripe = 0x2a2e36;
    const cx = x + 40;
    fill(cx - 3, y - 8, 7, 8, can); fill(cx - 3, y - 8, 1, 8, canL); fill(cx + 3, y - 8, 1, 8, canD); fill(cx - 3, y, 7, 1, SHADOW);
    fill(cx - 3, y - 5, 7, 1, stripe); fill(cx - 3, y - 3, 7, 1, stripe, 0.6);   // the hazard stripes
    fill(cx + 1, y - 10, 2, 2, spout); fill(cx - 2, y - 9, 4, 1, canD);          // the spout + cap
    dot(cx - 1, y - 7, WHITE, 0.5);
  },
  /** A spare crewmate helmet resting on the desk, its glass visor catching a cool reflection. */
  amongusvisor(x, y) {
    const shell = 0xdfe6ee, shellD = 0xa8b0ba, visor = 0x6ab0d0, visorL = 0xbfe4f0;
    const cx = x + 40;
    fill(cx - 4, y - 6, 9, 6, shell); fill(cx - 4, y - 6, 9, 1, 0xf0f4f8); fill(cx + 4, y - 6, 1, 6, shellD); fill(cx - 4, y, 9, 1, SHADOW);
    fill(cx - 2, y - 5, 6, 3, visor); fill(cx - 2, y - 5, 6, 1, visorL); dot(cx - 1, y - 5, WHITE, 0.8);
    if (isNight) glowLate(() => glowAt(cx + 1, y - 4, 4, 3, visor, 0.12));
  },
  /** A foil pouch of space snack with a little crewmate logo and a bendy straw. */
  amongussnack(x, y) {
    const foil = 0xc0c8d0, foilL = 0xe4e8ee, logo = 0xd83020, straw = 0x4ad04a;
    const cx = x + 40;
    fill(cx - 3, y - 7, 7, 7, foil); fill(cx - 3, y - 7, 1, 7, foilL); fill(cx - 3, y, 7, 1, SHADOW);
    fill(cx - 2, y - 5, 4, 3, logo); fill(cx - 1, y - 4, 2, 2, foilL);           // the little crewmate logo
    fill(cx + 3, y - 10, 1, 4, straw); dot(cx + 3, y - 10, straw); dot(cx + 4, y - 10, straw);
  },
  /** A mini medbay scan bed that sweeps a green diagnostic line up and down a little crewmate silhouette. */
  amongusscan(x, y) {
    const bed = 0x3a4450, bedL = 0x56626e, body = 0x24303a, green = 0x4ad04a;
    const cx = x + 39;
    fill(cx - 5, y - 2, 11, 2, bed); fill(cx - 5, y - 2, 11, 1, bedL); fill(cx - 5, y, 11, 1, SHADOW);
    fill(cx - 3, y - 6, 5, 4, body);                                             // the silhouette lying down
    const t = (now * 6) % 8, sy = y - 6 + Math.round(Math.abs(t - 4));
    fill(cx - 4, sy, 9, 1, green, 0.8); dot(cx - 4, sy, 0x9ff0a8);
    if (isNight) glowLate(() => glowAt(cx, sy, 5, 1, green, 0.2));
  },
  /** A steaming ramen bowl: a dark bowl with a red rim, pale broth, a slice of pink-and-white naruto and a green scallion fleck, chopsticks laid across; a wisp of steam curls up now and then. */
  tokyoramen(x, y) {
    const bowl = 0x2a2e3a;
    fill(x + 35, y - 3, 9, 3, bowl); fill(x + 36, y, 7, 1, 0x1a1e26); fill(x + 35, y - 3, 9, 1, 0x3e4450);
    fill(x + 36, y - 2, 7, 1, 0xe8e0cc); fill(x + 36, y - 2, 7, 1, 0xd83a2a, 0.0); fill(x + 35, y - 3, 9, 1, 0xd83a2a, 0.5);
    dot(x + 38, y - 2, 0xf28fb0); dot(x + 38, y - 2, 0xf6f6f2, 0.5); dot(x + 40, y - 2, 0x5aa84a); // naruto and scallion
    fill(x + 37, y - 4, 7, 1, 0x8a7a4a); dot(x + 44, y - 5, 0x8a7a4a); // chopsticks
    const st = (now + x * 0.2) % 4; if (st < 2.4) { const h = Math.floor(st * 3); dot(x + 39, y - 4 - h, WHITE, 0.3 - st * 0.1); dot(x + 40, y - 5 - h, WHITE, 0.25 - st * 0.08); }
  },
  /** A sake set: a white tokkuri flask and two little cups on a wooden tray, a faint blue brush-stroke on the flask; one cup gleams. */
  tokyosake(x, y) {
    fill(x + 35, y, 9, 1, 0x6f5236); fill(x + 35, y - 1, 9, 1, 0x8a6a44); // the tray
    fill(x + 36, y - 5, 3, 4, 0xeae6dc); fill(x + 36, y - 6, 3, 1, 0xeae6dc); dot(x + 37, y - 5, 0x3a5a9a); fill(x + 36, y - 5, 1, 4, WHITE, 0.4); // flask
    fill(x + 40, y - 2, 2, 2, 0xeae6dc); fill(x + 42, y - 2, 2, 2, 0xeae6dc); dot(x + 40, y - 2, WHITE, 0.5);
    if ((now + x * 0.3) % 7 < 0.4) dot(x + 42, y - 2, WHITE);
  },
  /** A maneki-neko: a white beckoning cat with a red collar and a gold coin, its left paw raised; the paw waves up and down slowly. */
  tokyomaneki(x, y) {
    const wave = Math.round(Math.sin(now * 2.2 + x * 0.2));
    fill(x + 37, y - 7, 6, 7, 0xf6f6f2); fill(x + 37, y, 6, 1, 0x1a1e26); fill(x + 37, y - 7, 6, 1, WHITE);
    fill(x + 37, y - 9, 2, 2, 0xf6f6f2); fill(x + 41, y - 9, 2, 2, 0xf6f6f2); // ears
    dot(x + 38, y - 6, PALETTE.eye); dot(x + 41, y - 6, PALETTE.eye); fill(x + 39, y - 5, 2, 1, 0xf28fb0);
    fill(x + 37, y - 3, 6, 1, 0xd83a2a); fill(x + 39, y - 2, 2, 1, 0xf2d35b); // collar and coin
    fill(x + 42, y - 6 - wave, 1, 2, 0xf6f6f2); dot(x + 42, y - 7 - wave, 0xf6f6f2); // the waving paw
  },
  /** A gachapon capsule machine in miniature: a clear dome of coloured capsules on a red base with a knob; every so often a capsule drops to the tray. */
  tokyogacha(x, y) {
    fill(x + 36, y - 3, 8, 3, 0xd83a2a); fill(x + 36, y - 3, 8, 1, 0xe86a5a); fill(x + 36, y, 8, 1, 0x8a1818); // base
    fill(x + 37, y - 9, 6, 6, 0xcfe6ee, 0.5); ring(x + 40, y - 6, 3.2, 0x9ab0ba, 0.7); // the dome
    const caps = [0xff4ab8, 0x3ae8ff, 0xf2d35b, 0x5aff8a];
    for (const [dx, dy, k] of [[38, -7, 0], [40, -8, 1], [42, -7, 2], [39, -5, 3], [41, -5, 0]]) dot(x + dx, y + dy, caps[k]);
    dot(x + 40, y - 2, 0x3a3a40); // the knob
    const t = (now + x * 0.3) % 9; if (t < 0.6) dot(x + 40, y - Math.round((1 - t / 0.6) * 2), caps[Math.floor(now) % 4]);
  },
  /** A bonsai: a gnarled trunk in a shallow blue dish with a cloud of fine green foliage clipped into tiers; a leaf or two drifts off now and then. */
  tokyobonsai(x, y) {
    fill(x + 36, y - 1, 8, 2, 0x3a5a7a); fill(x + 36, y - 1, 8, 1, 0x4a6e92); fill(x + 36, y + 1, 8, 1, 0x24384e); // dish
    fill(x + 39, y - 5, 1, 4, 0x6f5236); dot(x + 40, y - 4, 0x6f5236); fill(x + 41, y - 6, 1, 2, 0x6f5236); // trunk
    for (const [dx, dy, w] of [[36, -7, 5], [40, -9, 5], [38, -6, 4]]) { fill(x + dx, y + dy, w, 2, 0x3b7a46); fill(x + dx, y + dy, w, 1, 0x4f9a5a); }
    dot(x + 37, y - 8, 0x4f9a5a); dot(x + 44, y - 8, 0x4f9a5a);
    const t = (now + x * 0.4) % 11; if (t < 2) dot(x + 44, y - 6 + Math.floor(t * 2), 0x4f9a5a, 1 - t / 2);
  },
  /** A sushi plate: a black lacquer plate with two nigiri — a pink salmon and a pale one — a green dab of wasabi and a tuft of ginger; one piece gleams. */
  tokyosushi(x, y) {
    fill(x + 34, y - 1, 11, 2, 0x1a1a1e); fill(x + 34, y - 1, 11, 1, 0x3a3a40); // plate
    for (const [dx, c] of [[36, 0xf2967a], [40, 0xe8604a]]) { fill(x + dx, y - 3, 3, 2, 0xf4f0e4); fill(x + dx, y - 4, 3, 1, c); dot(x + dx, y - 4, shaded(c, 1.2)); fill(x + dx, y - 3, 1, 1, 0x3a3a40); } // rice + topping, nori band
    dot(x + 43, y - 2, 0x6aa84a); dot(x + 44, y - 3, 0xf2b8c0); // wasabi, ginger
    if ((now + x * 0.3) % 6 < 0.4) dot(x + 36, y - 4, WHITE);
  },
  /** A cast-iron teapot (tetsubin) with a bamboo handle and a little cup, sitting warm; a thread of steam rises from the spout now and then. */
  tokyoteapot(x, y) {
    const iron = 0x3a3e46;
    fill(x + 36, y - 4, 6, 4, iron); fill(x + 36, y, 6, 1, 0x1a1e24); fill(x + 36, y - 4, 6, 1, 0x55596a); // body
    fill(x + 35, y - 3, 1, 2, iron); dot(x + 35, y - 3, iron); // spout
    fill(x + 38, y - 6, 2, 1, 0x8a6a3a); dot(x + 38, y - 6, 0xa8844a); dot(x + 40, y - 6, 0xa8844a); // bamboo handle
    for (let k = 0; k < 3; k++) dot(x + 37 + k, y - 2, 0x55596a); // iron nubs
    fill(x + 43, y - 2, 2, 2, 0xeae6dc); dot(x + 43, y - 2, WHITE, 0.5); // the cup
    const st = (now + x * 0.25) % 5; if (st < 2) { const h = Math.floor(st * 2); dot(x + 35, y - 5 - h, WHITE, 0.3 - st * 0.1); }
  },
  /** A daruma doll: a round red papier-mache figure with a stern white face and one eye filled in; it rocks gently and now and then wobbles back upright. */
  tokyodaruma(x, y) {
    const rock = Math.round(Math.sin(now * 1.6 + x * 0.3));
    const bx = x + 38 + rock;
    fill(bx, y - 7, 6, 7, 0xd83a2a); fill(bx, y, 6, 1, 0x8a1818); fill(bx, y - 7, 6, 1, 0xe86a5a); // body
    fill(bx + 1, y - 5, 4, 3, 0xf4ecd8); // the face patch
    dot(bx + 1, y - 4, 0x1a1a1e); dot(bx + 4, y - 4, 0xf4ecd8); fill(bx + 2, y - 3, 2, 1, 0x3a2a1e); // one eye filled, one blank; brow/moustache
    fill(bx + 1, y - 6, 4, 1, 0xf2d35b, 0.7); // a gold brow band
  },
  /** A half-unrolled papyrus scroll with a row of tiny painted glyphs and a red title line; it rolls open a little further now and then. */
  egyptscroll(x, y) {
    const pap = 0xe2d2a4;
    const open = 3 + ((now + x * 0.3) % 10 < 2 ? 1 : 0);
    fill(x + 36, y - 2, 2, 3, 0xc9a86a); fill(x + 36 + 3 + open, y - 2, 2, 3, 0xc9a86a); // the two rolled ends
    fill(x + 38, y - 2, 1 + open, 3, pap); fill(x + 38, y - 2, 1 + open, 1, 0xf0e4c0);
    dot(x + 38, y - 1, 0xb84a32); for (let k = 0; k < open; k++) dot(x + 39 + k, y - 1, [0x241a12, 0x1f4e8a][k % 2]);
  },
  /** A canopic jar: a pale alabaster jar with a painted jackal-head stopper, blue and gold bands; a glyph column down the front. */
  egyptcanopic(x, y) {
    const jar = 0xe0d2a8, black = 0x241a12, gold = 0xd8b24e, blue = 0x3a6ab0;
    fill(x + 37, y - 5, 5, 5, jar); fill(x + 37, y, 5, 1, 0x1a1e24); fill(x + 37, y - 5, 5, 1, 0xf0e8d0); // body
    fill(x + 37, y - 8, 5, 3, black); fill(x + 37, y - 9, 2, 1, black); fill(x + 41, y - 9, 1, 1, black); // jackal head + ears
    dot(x + 38, y - 7, gold); dot(x + 40, y - 7, gold); fill(x + 41, y - 7, 1, 1, black); // eyes + snout
    fill(x + 37, y - 4, 5, 1, blue); fill(x + 39, y - 3, 1, 3, gold); // bands + glyph line
  },
  /** A painted queen's bust on a little stand: a tall blue-and-gold crown, a serene ochre face, a broad collar; one eye lined in black. */
  egyptbust(x, y) {
    const skin = 0xd79a64, gold = 0xd8b24e, blue = 0x1f4e8a, collar = 0x3ab8a0;
    fill(x + 38, y - 1, 4, 1, 0xc9a86a); fill(x + 38, y, 4, 1, 0x9a7a44); // the stand
    fill(x + 38, y - 5, 4, 4, skin); fill(x + 38, y - 5, 4, 1, 0xe8b074); // face
    fill(x + 37, y - 9, 6, 4, blue); for (let k = 0; k < 4; k++) fill(x + 37, y - 9 + k, 6, 1, k % 2 ? gold : blue); // tall crown
    dot(x + 39, y - 3, PALETTE.eye); fill(x + 38, y - 2, 4, 1, collar); // eye + collar
  },
  /** The scales of Ma'at: a small gold balance with two pans, a feather in one; the beam tips gently back and forth to level. */
  egyptscales(x, y) {
    const gold = 0xd8b24e, goldLight = 0xf2d888;
    const tip = Math.round(Math.sin(now * 1.4 + x * 0.2));
    fill(x + 39, y - 9, 1, 9, gold); fill(x + 38, y, 3, 1, gold); // stand
    fill(x + 35, y - 9 + tip, 9, 1, gold); dot(x + 35, y - 9 + tip, goldLight); // the beam
    fill(x + 34, y - 8 + tip, 3, 1, gold); fill(x + 42, y - 10 - tip, 3, 1, gold); // the two pans
    fill(x + 42, y - 12 - tip, 1, 2, 0xeae2cc); dot(x + 43, y - 12 - tip, 0xeae2cc); // the feather
  },
  /** A gold Bastet statuette: a sleek seated cat on a black base, pointed ears, a gold hoop earring and a scarab on its chest; it gleams. */
  egyptgoldcat(x, y) {
    const gold = 0xd8b24e, goldLight = 0xf2d888, goldDark = 0x9a7a2a, black = 0x241a12;
    fill(x + 38, y - 1, 4, 1, black); // base
    fill(x + 38, y - 8, 4, 7, gold); fill(x + 38, y - 8, 1, 7, goldLight); fill(x + 41, y - 8, 1, 7, goldDark); // seated body
    fill(x + 38, y - 10, 1, 2, gold); fill(x + 41, y - 10, 1, 2, gold); // ears
    dot(x + 39, y - 8, PALETTE.eye); dot(x + 42, y - 7, 0x1f4e8a); // eye + earring
    dot(x + 40, y - 5, 0x3ab8a0); // scarab
    if ((now + x * 0.3) % 6 < 0.4) dot(x + 39, y - 7, WHITE);
  },
  /** A scarab amulet on a little stand: a turquoise beetle with gold wing lines and outspread gold wings, glinting. */
  egyptamulet(x, y) {
    const turq = 0x3ab8a0, gold = 0xd8b24e, black = 0x241a12;
    fill(x + 38, y - 1, 4, 1, 0x9a7a44); // stand
    fill(x + 36, y - 4, 10, 2, gold); fill(x + 36, y - 4, 10, 1, 0xf2d888); // outspread wings
    fill(x + 39, y - 6, 3, 4, turq); fill(x + 39, y - 6, 3, 1, 0x5ad8c0); fill(x + 40, y - 5, 1, 3, black); // the beetle body + line
    dot(x + 40, y - 6, 0xeae2cc);
  },
  /** A bronze incense burner on three little legs with a perforated lid; a thread of pale smoke rises and curls away, glowing faintly. */
  egyptincense(x, y) {
    const bronze = 0x9a6a3a, bronzeLight = 0xc89a5a;
    fill(x + 37, y - 3, 6, 3, bronze); fill(x + 37, y - 3, 6, 1, bronzeLight); fill(x + 37, y, 1, 1, bronze); fill(x + 42, y, 1, 1, bronze); // bowl + legs
    fill(x + 38, y - 5, 4, 2, bronze); for (const dx of [38, 40, 41]) dot(x + dx, y - 5, 0x2a1a10); // lid holes
    const st = (now + x * 0.3) % 4; for (let k = 0; k < 3; k++) { const sy = y - 6 - k * 2 - Math.floor(st); dot(x + 40 + Math.round(Math.sin(st + k) * 1.5), sy, 0xb0a890, 0.35 - k * 0.08); }
    if ((now + x) % 6 < 0.5) dot(x + 40, y - 5, 0xe8782e);
  },
  /** A senet board: a little ebony-and-ivory game box with a grid of squares and a few playing pieces; a piece is nudged along now and then. */
  egyptsenet(x, y) {
    const wood = 0x5a3b25, ivory = 0xe8dcbe, gold = 0xd8b24e;
    fill(x + 34, y - 4, 11, 4, wood); fill(x + 34, y - 4, 11, 1, 0x7a5232); fill(x + 34, y, 11, 1, 0x3a2418); // box
    for (let r = 0; r < 3; r++) for (let c = 0; c < 5; c++) if ((r + c) % 2 === 0) fill(x + 35 + c * 2, y - 3 + r, 1, 1, ivory);
    fill(x + 34, y - 4, 11, 1, gold, 0.5);
    const move = (now + x * 0.3) % 7 < 2 ? 1 : 0; dot(x + 36 + move, y - 3, gold); dot(x + 42, y - 2, ivory); // pieces
  },
  /** A horn ale tankard banded with iron, frothing over; the foam settles and swells now and then. */
  vikingtankard(x, y) {
    const horn = 0xcfbf96, iron = 0x6a6e76, ale = 0xb8863a;
    fill(x + 37, y - 5, 5, 5, horn); fill(x + 37, y, 5, 1, 0x8a7a54); fill(x + 37, y - 5, 5, 1, iron); fill(x + 37, y - 2, 5, 1, iron); // body + bands
    fill(x + 42, y - 4, 1, 3, iron); // handle
    const froth = (now + x * 0.3) % 5 < 2 ? 1 : 0; fill(x + 37, y - 6 - froth, 5, 1, 0xeee6d2); dot(x + 38, y - 7 - froth, 0xeee6d2, 0.7);
  },
  /** A conical iron helm with a noseguard and riveted brow band on a small stand; a glint runs across it. */
  vikinghelm(x, y) {
    const iron = 0x8a8e96, ironLight = 0xb4b8c0, ironDark = 0x4a4e56;
    fill(x + 38, y - 1, 4, 1, 0x5a3b25); // stand
    fill(x + 38, y - 6, 4, 5, iron); fill(x + 39, y - 7, 2, 1, iron); fill(x + 38, y - 6, 1, 5, ironLight); fill(x + 41, y - 6, 1, 5, ironDark); // dome
    fill(x + 38, y - 3, 4, 1, ironDark); // brow band
    fill(x + 39, y - 3, 1, 3, iron); // noseguard
    dot(x + 38, y - 1, PALETTE.eye); dot(x + 41, y - 1, PALETTE.eye);
    if ((now + x * 0.3) % 6 < 0.4) dot(x + 39, y - 5, WHITE);
  },
  /** A bearded axe leaning on a small block: a dark iron head with a broad curved blade on a wrapped wooden haft; the edge gleams. */
  vikingaxe(x, y) {
    const iron = 0x8a8e96, ironDark = 0x4a4e56, haft = 0x6a4a2e, wrap = 0x3a2a1e;
    fill(x + 40, y - 9, 1, 9, haft); fill(x + 40, y - 9, 1, 9, haft); // haft
    for (const wy of [-6, -4, -2]) dot(x + 40, y + wy, wrap);
    fill(x + 36, y - 9, 4, 5, iron); fill(x + 35, y - 7, 1, 4, iron); fill(x + 36, y - 9, 4, 1, 0xb4b8c0); // the broad blade
    fill(x + 35, y - 6, 1, 3, 0xd8dce2); // bright edge
    fill(x + 39, y - 8, 2, 2, ironDark); // the eye/socket
    fill(x + 38, y, 5, 1, 0x5a3b25); // the block
  },
  /** A scatter of flat rune stones cast on a cloth; a couple are turned over to new marks now and then. */
  vikingrunecast(x, y) {
    const stone = 0x9a9a92, mark = 0xb83a2a, cloth = 0x4a2a2a;
    fill(x + 35, y - 1, 10, 2, cloth); fill(x + 35, y - 1, 10, 1, 0x5e3838); // the cloth
    const n = Math.floor((now + x * 0.3) / 4);
    for (const [dx, dy, k] of [[36, -3, 0], [39, -2, 1], [42, -3, 2], [38, -4, 3], [41, -5, 4]]) {
      fill(x + dx, y + dy, 2, 2, stone); dot(x + dx, y + dy, 0xb4b4ac);
      dot(x + dx, y + dy + (hash(n, k) % 2), mark);
    }
  },
  /** A round-bellied ceramic mead jug with a cork stopper and a painted band; a drip runs from the spout now and then. */
  vikingmead(x, y) {
    const jug = 0x7a5a3a, jugLight = 0x9a7a52, band = 0xd8a850;
    fill(x + 37, y - 6, 5, 6, jug); fill(x + 37, y, 5, 1, 0x4a3422); fill(x + 37, y - 6, 5, 1, jugLight); fill(x + 36, y - 5, 1, 3, jug); // body + spout
    fill(x + 37, y - 4, 5, 1, band); dot(x + 39, y - 3, 0xf0c070); // painted band
    fill(x + 38, y - 8, 3, 2, jug); dot(x + 39, y - 9, 0x6a4a2e); // neck + cork
    if ((now + x * 0.3) % 6 < 0.5) dot(x + 36, y - 3 + Math.floor(((now + x) % 1) * 2), band, 0.7);
  },
  /** A little carved model longship with a dragon prow, a striped sail and a row of tiny round shields; it rocks gently. */
  vikinglongship(x, y) {
    const wood = 0x6a4a2e, woodLight = 0x8a6440, sail = 0xcabf9a, red = 0xb83a2a;
    const rock = Math.round(Math.sin(now * 1.3 + x * 0.2));
    const by = y - 3 + rock;
    fill(x + 35, by, 10, 2, wood); fill(x + 34, by - 1, 2, 2, wood); dot(x + 33, by - 2, woodLight); fill(x + 44, by - 2, 1, 2, wood); // hull + prow + stern
    for (let k = 0; k < 4; k++) dot(x + 36 + k * 2, by, k % 2 ? red : 0x2a5a7a); // shields
    fill(x + 39, by - 6, 1, 4, wood); fill(x + 37, by - 6, 5, 3, sail); for (let k = 0; k < 3; k++) fill(x + 37, by - 6 + k, 5, 1, k % 2 ? red : sail); // mast + striped sail
  },
  /** A twisted silver torc neck-ring with beast-head terminals on a little stand; it gleams. */
  vikingtorc(x, y) {
    const silver = 0xc8ccd2, silverLight = 0xe8ecf0, silverDark = 0x8a8e96;
    fill(x + 39, y - 1, 2, 1, 0x5a3b25); // stand
    ring(x + 40, y - 5, 3.2, silver, 1); fill(x + 38, y - 3, 5, 2, 0x6a4a2e, 0); // open ring
    fill(x + 38, y - 3, 1, 2, silver); fill(x + 42, y - 3, 1, 2, silver); // the two terminals
    dot(x + 38, y - 3, PALETTE.eye); dot(x + 42, y - 3, PALETTE.eye);
    dot(x + 38, y - 7, silverLight); dot(x + 42, y - 6, silverDark);
    if ((now + x * 0.3) % 6 < 0.4) dot(x + 40, y - 8, WHITE);
  },
  /** A wooden feast platter: a roast leg, a round loaf, an apple and a wedge of cheese; steam lifts off the meat now and then. */
  vikingfeast(x, y) {
    const board = 0x7a5636, meat = 0x9a5a3a, loaf = 0xc89a5a, cheese = 0xe0c060, apple = 0xb83a2a;
    fill(x + 34, y - 1, 11, 2, board); fill(x + 34, y - 1, 11, 1, 0x9a7050); fill(x + 34, y + 1, 11, 1, 0x4a3422);
    fill(x + 35, y - 4, 4, 3, meat); fill(x + 35, y - 4, 4, 1, 0xb87a52); fill(x + 34, y - 3, 1, 2, 0xe8dcc0); // roast leg + bone
    disc(x + 41, y - 3, 2, loaf); dot(x + 41, y - 4, 0xd8b078); // loaf
    fill(x + 43, y - 3, 2, 2, cheese); dot(x + 40, y - 2, apple); // cheese + apple
    const st = (now + x * 0.3) % 5; if (st < 2) dot(x + 36, y - 5 - Math.floor(st * 2), WHITE, 0.3 - st * 0.1);
  },
  /** A crystal skull on a little stand: a clear quartz skull with faint facets and dark eye sockets; a glint crosses it and it catches a jade tint now and then. */
  aztecskull(x, y) {
    const crystal = 0xcfe6ee, crystalLight = 0xeaf6fa, dark = 0x3a4a52;
    fill(x + 38, y - 1, 4, 1, 0x55504a); // stand
    fill(x + 37, y - 6, 6, 5, crystal, 0.85); fill(x + 38, y - 7, 4, 1, crystal, 0.85); fill(x + 37, y - 6, 1, 5, crystalLight, 0.7);
    dot(x + 38, y - 5, dark); dot(x + 41, y - 5, dark); fill(x + 39, y - 2, 2, 1, dark); // sockets + jaw line
    const t = (now + x * 0.3) % 7; if (t < 0.5) dot(x + 38, y - 6, WHITE); else if (t < 1) dot(x + 40, y - 5, 0x3ab8a0, 0.5);
  },
  /** A small gold idol statuette on a base: a seated figure with a square feathered headdress and crossed arms, gleaming gold with red eyes. */
  aztecgoldidol(x, y) {
    const gold = 0xd8b24e, goldLight = 0xf2d888, goldDark = 0x9a7a2a, red = 0xb83a2a;
    fill(x + 38, y - 1, 4, 1, 0x3a2a1e); // base
    fill(x + 38, y - 5, 4, 4, gold); fill(x + 38, y - 5, 1, 4, goldLight); fill(x + 41, y - 5, 1, 4, goldDark); // body
    fill(x + 37, y - 8, 6, 3, gold); fill(x + 37, y - 8, 6, 1, goldLight); dot(x + 37, y - 9, gold); dot(x + 42, y - 9, gold); // headdress
    dot(x + 39, y - 6, red); dot(x + 41, y - 6, red); fill(x + 38, y - 3, 4, 1, goldDark); // eyes + arms
    if ((now + x * 0.3) % 6 < 0.4) dot(x + 38, y - 5, WHITE);
  },
  /** A ritual obsidian blade on a cloth: a leaf-shaped black glass knife with a bound handle, its edges catching a sharp white glint. */
  aztecobsidian(x, y) {
    const obs = 0x2a2a34, obsLight = 0x55586a, wrap = 0x8a5a3a, cloth = 0x3a4a2e;
    fill(x + 35, y - 1, 10, 2, cloth); fill(x + 35, y - 1, 10, 1, 0x4a5a3a); // cloth
    fill(x + 36, y - 3, 4, 2, obs); fill(x + 37, y - 4, 3, 1, obs); fill(x + 36, y - 2, 1, 2, obsLight); // the blade
    dot(x + 36, y - 3, 0xb8c4d0); // glint
    fill(x + 40, y - 3, 3, 2, wrap); for (const dx of [40, 42]) dot(x + dx, y - 3, 0x6a4a2e); // bound handle
  },
  /** A cup of frothy xocolatl: a painted clay cup of dark cacao drink with a reddish froth on top and a dusting of spice; a little steam rises. */
  azteccacao(x, y) {
    const cup = 0xb84a32, cupDark = 0x8a3020, cacao = 0x3a221a, froth = 0x7a4a32;
    fill(x + 37, y - 4, 5, 4, cup); fill(x + 37, y, 5, 1, cupDark); fill(x + 37, y - 4, 5, 1, 0xd8604a); // cup
    fill(x + 38, y - 5, 3, 1, cacao); fill(x + 37, y - 5, 5, 1, froth); dot(x + 39, y - 5, 0x9a5a3a); // cacao + froth
    fill(x + 37, y - 3, 1, 3, cupDark); // a painted stripe
    const st = (now + x * 0.3) % 5; if (st < 2) dot(x + 39, y - 6 - Math.floor(st * 2), WHITE, 0.3 - st * 0.1);
  },
  /** A round-bellied painted pot with bands of red and jade geometric patterns and two little handles; it sits heavy. */
  aztecpot(x, y) {
    const clay = 0xc07a4a, clayLight = 0xd89a68, clayDark = 0x8a5230, red = 0xb83a2a, jade = 0x3ab8a0;
    fill(x + 37, y - 6, 6, 6, clay); fill(x + 38, y - 7, 4, 1, clay); fill(x + 37, y, 6, 1, clayDark); fill(x + 37, y - 6, 1, 6, clayLight);
    fill(x + 36, y - 5, 1, 2, clayDark); fill(x + 43, y - 5, 1, 2, clayDark); // handles
    fill(x + 37, y - 5, 6, 1, red); for (let k = 0; k < 3; k++) dot(x + 38 + k * 2, y - 3, jade); fill(x + 37, y - 2, 6, 1, red, 0.7); // bands
  },
  /** A small stone calendar disc on a stand: a carved ring of day-glyphs around a central face, red and gold; it turns very slowly. */
  azteccalendar(x, y) {
    const stone = 0x8a7f6e, stoneDark = 0x55504a, red = 0xb83a2a, gold = 0xd8b24e;
    fill(x + 39, y - 1, 2, 1, stoneDark); // stand
    disc(x + 40, y - 5, 4, stone); ring(x + 40, y - 5, 4, stoneDark, 1); ring(x + 40, y - 5, 2.6, red, 0.8);
    const a = now * 0.3; for (let k = 0; k < 6; k++) { const ang = a + (k / 6) * Math.PI * 2; dot(x + 40 + Math.round(Math.cos(ang) * 3.4), y - 5 + Math.round(Math.sin(ang) * 3.4), k % 2 ? gold : stoneDark); }
    dot(x + 40, y - 5, gold);
  },
  /** A feathered headdress on a stand: a fan of long green and red quetzal feathers rising from a gold band; the feathers sway a touch. */
  aztecfeather(x, y) {
    const green = 0x3a9a5a, greenLight = 0x5ac07a, red = 0xb83a2a, gold = 0xd8b24e;
    fill(x + 38, y - 2, 4, 2, gold); fill(x + 38, y - 2, 4, 1, 0xf2d888); fill(x + 39, y, 2, 1, 0x8a6a2a); // the gold band + stand
    const sway = Math.round(Math.sin(now * 1.4 + x * 0.2));
    for (let k = 0; k < 5; k++) { const fx = x + 38 + k, h = 5 + (k === 2 ? 2 : (k === 1 || k === 3 ? 1 : 0)); const c = k % 2 ? red : green; fill(fx, y - 3 - h, 1, h, c); dot(fx + (k < 2 ? -sway : sway), y - 3 - h, k % 2 ? 0xd8604a : greenLight); }
  },
  /** A small offering of maize: two ears of corn in their husks and a little bowl of kernels on a woven mat. */
  aztecmaize(x, y) {
    const corn = 0xe0c04a, cornLight = 0xf2d870, husk = 0x9aae5a, mat = 0xc8a060, bowl = 0x8a5230;
    fill(x + 35, y, 10, 1, mat); fill(x + 35, y, 10, 1, 0xd8b070); // mat
    for (const [cx, lean] of [[36, 0], [38, 1]]) { fill(x + cx + lean, y - 6, 2, 6, corn); for (let k = 0; k < 3; k++) dot(x + cx + lean, y - 5 + k * 2, cornLight); fill(x + cx + lean - 1, y - 1, 3, 1, husk); } // ears
    fill(x + 41, y - 2, 4, 2, bowl); fill(x + 41, y - 2, 4, 1, corn); dot(x + 42, y - 2, cornLight); // bowl of kernels
  },
  /** A celestial globe: a dark blue sphere speckled with gold stars and faint constellation lines in a brass meridian ring on a little stand; it turns slowly. */
  obsglobe(x, y) {
    const sky = 0x1e2a4a, brass = 0xc8a050, brassLight = 0xe8c878;
    fill(x + 39, y - 1, 2, 1, 0x5a4a2a); fill(x + 38, y, 4, 1, brass); // stand
    disc(x + 40, y - 5, 4, sky); ring(x + 40, y - 5, 4.2, brass, 0.9);
    const r = Math.floor(now * 1.5) % 3; for (const [dx, dy] of [[-1, -2], [1, -1], [-2, 1], [2, 1], [0, 2]]) dot(x + 40 + dx + (r === 1 ? 1 : 0), y - 5 + dy, (dx + dy) % 2 ? 0xf2d888 : 0xf6f6f2);
    dot(x + 38, y - 6, brassLight);
  },
  /** A small brass desk telescope on a tripod, tilted up to the sky; its lens catches a glint. */
  obsdesktelescope(x, y) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a;
    for (const lx of [37, 40, 43]) line(x + 40, y - 3, x + lx, y, brassDark); // tripod legs
    line(x + 36, y - 2, x + 43, y - 9, brass); line(x + 36, y - 2, x + 43, y - 9, brass); fill(x + 42, y - 10, 3, 2, brass); fill(x + 42, y - 10, 1, 2, brassLight); // the tube
    fill(x + 35, y - 3, 2, 2, brass); dot(x + 44, y - 9, 0xfff0c0); // eyepiece + lens glint
  },
  /** A brass astrolabe lying flat with engraved rings and a star-pointer; a glint runs round its rim. */
  obsastrolabe(x, y) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, dark = 0x2a2416;
    disc(x + 40, y - 3, 4, dark); ring(x + 40, y - 3, 4, brass, 1); ring(x + 40, y - 3, 2.2, brassDark, 0.8);
    const a = now * 0.5; line(x + 40, y - 3, x + 40 + Math.round(Math.cos(a) * 3), y - 3 + Math.round(Math.sin(a) * 3), brassLight); dot(x + 40, y - 3, brassLight);
    for (let k = 0; k < 4; k++) { const ang = (k / 4) * Math.PI * 2; dot(x + 40 + Math.round(Math.cos(ang) * 3.6), y - 3 + Math.round(Math.sin(ang) * 3.6), brassLight); }
  },
  /** A dark pitted meteorite on a little display stand with a brass plaque; a metallic fleck glints now and then. */
  obsmeteorite(x, y) {
    const rock = 0x3a342e, rockLight = 0x55504a, rockDark = 0x201c18, brass = 0xc8a050;
    fill(x + 38, y - 1, 5, 1, 0x4a4238); fill(x + 39, y, 3, 1, 0x2a241e); fill(x + 38, y - 1, 5, 1, brass); // stand + plaque
    fill(x + 37, y - 5, 6, 4, rock); fill(x + 38, y - 6, 4, 1, rock); fill(x + 37, y - 5, 6, 1, rockLight);
    for (const [dx, dy] of [[38, -4], [41, -3], [39, -2]]) dot(x + dx, y + dy, rockDark); // pits
    if ((now + x * 0.3) % 5 < 0.3) dot(x + 40, y - 4, 0xcfd6ea);
  },
  /** A rolled star chart tied with a cord, half-open to show a scatter of stars and a faint arc; it unrolls a touch now and then. */
  obsstarchart(x, y) {
    const paper = 0x2e3a5e, paperEnd = 0x1e2742, cord = 0xc8a050;
    const open = 4 + ((now + x * 0.3) % 9 < 2 ? 1 : 0);
    fill(x + 36, y - 2, 2, 3, paperEnd); fill(x + 36 + 3 + open, y - 2, 2, 3, paperEnd);
    fill(x + 38, y - 2, 1 + open, 3, paper); fill(x + 38, y - 2, 1 + open, 1, 0x3e4a6e);
    for (let k = 0; k < open; k++) dot(x + 39 + k, y - 1, k % 2 ? 0xf6f6f2 : 0xf2d888, 0.9);
    dot(x + 37, y - 1, cord);
  },
  /** A model of Saturn on a rod: a pale gold banded planet tilted inside a flat ring; it turns slowly and catches the light. */
  obssaturn(x, y) {
    const planet = 0xe0c080, planetLight = 0xf0d8a0, band = 0xc8a060, ring = 0xe8dcb0;
    fill(x + 39, y, 2, 1, 0x3a3444); fill(x + 40, y - 3, 1, 3, 0x6a6474); // the rod
    ellipseRing(x + 40, y - 6, 5, 1.6, ring, 0.9); ellipseRing(x + 40, y - 6, 3.6, 1.1, band, 0.7);
    disc(x + 40, y - 6, 3, planet); fill(x + 38, y - 6, 5, 1, band); fill(x + 38, y - 7, 5, 1, planetLight, 0.7);
    dot(x + 39, y - 7, planetLight);
    if ((now + x * 0.3) % 6 < 0.4) dot(x + 38, y - 6, WHITE);
  },
  /** A brass pocket compass open on the desk, its blued needle swinging gently to settle on north. */
  obscompass(x, y) {
    const brass = 0xc8a050, brassLight = 0xe8c878, face = 0xe8e0cc, needle = 0xb83a2a;
    disc(x + 40, y - 3, 4, brass); disc(x + 40, y - 3, 3, face); ring(x + 40, y - 3, 3.2, 0x8a6a2a, 0.8);
    fill(x + 35, y - 4, 2, 2, brass); // the hinged lid tab
    const a = -Math.PI / 2 + Math.sin(now * 1.5 + x) * 0.3; const nx = Math.round(Math.cos(a) * 2.4), ny = Math.round(Math.sin(a) * 2.4);
    line(x + 40, y - 3, x + 40 + nx, y - 3 + ny, needle); line(x + 40, y - 3, x + 40 - nx, y - 3 - ny, 0x3a3a46); dot(x + 40, y - 3, brassLight);
  },
  /** An open observation journal with columns of tiny figures, a little sketch of the moon, and a pencil; the page catches the lamplight. */
  obsnotebook(x, y) {
    const paper = 0xe4dcc8, line = 0x9a94a0, ink = 0x2a3050, moon = 0xd8d8e0;
    fill(x + 35, y - 4, 10, 4, paper); fill(x + 35, y - 4, 10, 1, 0xf0e8d6); fill(x + 40, y - 4, 1, 4, 0xc8c0ac); // open book, spine
    for (let r = 0; r < 3; r++) { fill(x + 36, y - 3 + r, 3, 1, line, 0.5); dot(x + 36, y - 3 + r, ink); dot(x + 38, y - 3 + r, ink); } // figures on the left
    disc(x + 43, y - 2, 1.4, moon); dot(x + 42, y - 2, 0xb8b8c0); // moon sketch
    fill(x + 41, y - 5, 4, 1, 0xd8a850); dot(x + 45, y - 5, 0x3a3a40); // pencil
  },
  /** Half a coconut served as a drink with a striped straw, a cherry and a tiny paper umbrella; a sip bubbles up the straw now and then. */
  tikicoconut(x, y) {
    const shell = 0x6a4a2e, shellLight = 0x8a6440, milk = 0xeae2d0, straw = 0xe8404a;
    fill(x + 37, y - 4, 6, 4, shell); fill(x + 37, y - 4, 6, 1, shellLight); fill(x + 37, y, 6, 1, 0x3a2418);
    fill(x + 38, y - 5, 4, 1, milk); dot(x + 39, y - 5, 0xf4ecd8); // the milk surface
    for (let k = 0; k < 4; k++) fill(x + 41, y - 8 + k, 1, 1, k % 2 ? straw : 0xf0ece0); // striped straw
    dot(x + 38, y - 6, 0xe8404a); // cherry
    fill(x + 37, y - 9, 3, 1, 0xf2b040); fill(x + 38, y - 10, 1, 1, 0x6a4a2e); // umbrella
  },
  /** A whole pineapple hollowed into a cocktail with its leafy crown, a wedge, a straw and an umbrella; a glint on the glass of juice. */
  tikipineapple(x, y) {
    const skin = 0xe0a830, skinDark = 0xb8842a, leaf = 0x4f9a5a, juice = 0xf2c040;
    fill(x + 37, y - 6, 5, 6, skin); fill(x + 37, y - 6, 5, 1, 0xf0c050); fill(x + 37, y, 5, 1, 0x8a6420);
    for (let r = 0; r < 3; r++) for (let c = 0; c < 2; c++) dot(x + 38 + c * 2, y - 5 + r * 2, skinDark); // cross-hatch
    for (const [dx, dy] of [[-1, 0], [0, -1], [1, 0], [0, -2]]) fill(x + 39 + dx, y - 9 + dy, 1, 2, leaf); // crown
    fill(x + 41, y - 9, 1, 4, 0xf0ece0); dot(x + 42, y - 9, 0xe8404a); // straw + cherry
    dot(x + 38, y - 5, juice, 0.6);
  },
  /** A little ukulele propped up: a pear-shaped pale wood body with a round sound-hole, four strings and a short neck; a string shivers now and then. */
  tikiukulele(x, y) {
    const wood = 0xd8b06a, woodLight = 0xeaca88, woodDark = 0xa8803a, neck = 0x8a5a32;
    fill(x + 42, y - 10, 1, 7, neck); fill(x + 41, y - 11, 3, 2, neck); dot(x + 41, y - 11, 0xa06a3a); // neck + head
    disc(x + 38, y - 3, 3, wood); disc(x + 38, y - 3, 3, woodLight, 0); fill(x + 36, y - 5, 5, 1, woodLight); ring(x + 38, y - 3, 1.4, woodDark, 1); // body + hole
    const shiver = Math.floor(now * 6) % 2; for (let k = 0; k < 3; k++) line(x + 38, y - 3, x + 42, y - 9 + (k === 1 ? shiver : 0), 0xf0ece0, 0.7);
  },
  /** A big pink-mouthed conch shell on its side with a spiral of ridges and points; it catches the light. */
  tikishell(x, y) {
    const shell = 0xe8d0b0, shellLight = 0xf4e4ca, pink = 0xf0a8b0, ridge = 0xc8a888;
    fill(x + 36, y - 4, 7, 4, shell); fill(x + 36, y - 4, 7, 1, shellLight); fill(x + 42, y - 3, 2, 2, pink); // body + flared mouth
    for (const [dx, dy] of [[37, -5], [39, -6], [41, -5]]) { fill(x + dx, y + dy, 1, 2, shell); dot(x + dx, y + dy - 1, ridge); } // spire points
    for (let k = 0; k < 3; k++) fill(x + 37 + k * 2, y - 3, 1, 3, ridge, 0.6); // ridges
    if ((now + x * 0.3) % 6 < 0.4) dot(x + 38, y - 4, WHITE);
  },
  /** A flower lei looped on the desk: a ring of alternating pink and yellow blossoms with little green leaves; a petal lifts in a draught. */
  tikilei(x, y) {
    const pink = 0xf06a9a, yellow = 0xf2c040, leaf = 0x4f9a5a, white = 0xf2d8e0;
    for (let k = 0; k < 10; k++) { const a = (k / 10) * Math.PI * 2, bx = x + 40 + Math.round(Math.cos(a) * 4), by = y - 3 + Math.round(Math.sin(a) * 2.4); dot(bx, by, k % 3 === 2 ? leaf : (k % 2 ? pink : yellow)); if (k % 2) dot(bx, by, k === 5 && (now % 4 < 0.5) ? white : (k % 2 ? pink : yellow)); }
    dot(x + 40, y - 3, leaf);
  },
  /** A carved wooden bowl of tropical fruit: bananas, an orange, a lime and a wedge of watermelon piled up; a fly-free island snack. */
  tikibananas(x, y) {
    const bowl = 0x8a5a32, bowlLight = 0xa8723e, banana = 0xf2c040, orange = 0xe8822a, lime = 0x8aae3a, melon = 0xe8404a;
    fill(x + 35, y - 2, 10, 2, bowl); fill(x + 35, y - 2, 10, 1, bowlLight); fill(x + 35, y, 10, 1, 0x4a2e18); // bowl
    for (let k = 0; k < 4; k++) fill(x + 36 + k, y - 4 - (k === 1 || k === 2 ? 1 : 0), 1, 2, banana); dot(x + 36, y - 5, 0x5a4420); // banana bunch
    disc(x + 41, y - 3, 1.6, orange); dot(x + 41, y - 4, 0xf0a040); dot(x + 39, y - 3, lime); // orange + lime
    fill(x + 42, y - 5, 2, 2, melon); dot(x + 43, y - 5, 0x8aae3a); // melon wedge
  },
  /** A dark rum bottle with a hand-written island label and a cork, a little liquid sloshing; the glass catches a highlight. */
  tikibottle(x, y) {
    const glass = 0x3a5a3a, glassLight = 0x5a7a52, label = 0xe4dcc4, cork = 0x8a5a32, rum = 0x7a4a24;
    fill(x + 38, y - 8, 4, 8, glass); fill(x + 38, y - 8, 1, 8, glassLight); fill(x + 38, y, 4, 1, 0x1e2e1e); // body
    fill(x + 39, y - 11, 2, 3, glass); fill(x + 39, y - 12, 2, 1, cork); // neck + cork
    fill(x + 38, y - 5, 4, 3, label); fill(x + 38, y - 5, 4, 1, 0xf0e8d0); for (let r = 0; r < 2; r++) fill(x + 39, y - 4 + r, 2, 1, 0x8a7a5a, 0.6); // label
    fill(x + 38, y - 1, 4, 1, rum, 0.6); dot(x + 38, y - 7, WHITE, 0.5);
  },
  /** A big ceramic volcano-bowl cocktail: a brown tiki bowl with a crater in the middle where a little flame flickers over blue liquor, two long straws and a flower; it glows warm. */
  tikivolcano(x, y) {
    const bowl = 0x7a4a2a, bowlLight = 0x9a6440, blue = 0x3a8ad8, flame = 0xe8782e;
    fill(x + 35, y - 4, 10, 4, bowl); fill(x + 35, y - 4, 10, 1, bowlLight); fill(x + 35, y, 10, 1, 0x3a2414); // the bowl
    for (const [dx, c] of [[36, 0x6a3e22], [43, 0x6a3e22]]) fill(x + dx, y - 3, 1, 3, c); // carved tiki faces on the sides
    fill(x + 38, y - 5, 4, 1, blue); dot(x + 39, y - 5, 0x5aa8f0); // blue moat
    const f = candlelight(now + x), h = 2 + Math.round(f * 2); fill(x + 39, y - 5 - h, 2, h, flame); dot(x + 39, y - 6 - h, 0xffd36b, f); // the flame
    for (const sx of [36, 44]) fill(x + sx, y - 9, 1, 5, 0xf0ece0); // two straws
    dot(x + 37, y - 5, 0xf06a9a); // a flower
    if (isNight) glowLate(() => glowAt(x + 40, y - 6, 4, 3, 0xffa040, 0.2));
  },
  /** A tall milkshake in a fluted glass: pink shake with a swirl of cream, a cherry on top and a striped straw; the cream glistens. */
  dinermilkshake(x, y) {
    const glass = 0xe8f0f4, shake = 0xf2a8c0, cream = 0xf8f0f0, cherry = 0xe03a4a, straw = 0xe03a4a;
    fill(x + 38, y - 7, 4, 7, glass); fill(x + 38, y - 7, 1, 7, 0xf8fcff); fill(x + 38, y, 4, 1, 0x9ab0ba);
    fill(x + 39, y - 6, 3, 6, shake); fill(x + 39, y - 8, 3, 2, cream); dot(x + 40, y - 9, cream); dot(x + 41, y - 8, cream); // shake + swirl
    dot(x + 40, y - 10, cherry); for (let k = 0; k < 3; k++) dot(x + 42, y - 9 + k, k % 2 ? straw : 0xf0ece0); // cherry + straw
    dot(x + 39, y - 5, WHITE, 0.5);
  },
  /** A classic burger in a red plastic basket on checked paper: a sesame bun with lettuce and a patty peeking out, a toothpick frill on top. */
  dinerburger(x, y) {
    const basket = 0xe03a4a, paper = 0xf0ece0, bun = 0xe0a860, bunLight = 0xf0c078, patty = 0x7a4a2e, lettuce = 0x6aae4a;
    fill(x + 35, y - 1, 10, 2, basket); fill(x + 35, y - 1, 10, 1, 0xf05a68); fill(x + 36, y - 2, 8, 1, paper); // basket + paper
    fill(x + 37, y - 4, 6, 2, bun); fill(x + 37, y - 4, 6, 1, bunLight); for (const dx of [38, 40, 42]) dot(x + dx, y - 4, 0xf0d898); // top bun + seeds
    fill(x + 37, y - 2, 6, 1, lettuce); fill(x + 37, y - 3, 6, 1, patty); // lettuce + patty
    dot(x + 40, y - 6, 0x6a6a72); // a toothpick frill
  },
  /** A paper cone of golden fries with a few spilling out and a little paper cup of ketchup beside it. */
  dinerfries(x, y) {
    const cone = 0xf0ece0, fry = 0xf2c040, fryDark = 0xd8a030, ketchup = 0xe03a4a;
    fill(x + 36, y - 4, 5, 5, cone); fill(x + 36, y - 4, 5, 1, 0xf8f4ea); fill(x + 36, y, 5, 1, 0xc8c0ac); // cone
    for (let k = 0; k < 5; k++) { const fx = x + 37 + k; fill(fx, y - 7 + (k % 2), 1, 4, k % 2 ? fryDark : fry); } // fries sticking up
    dot(x + 35, y - 2, fry); // a spilled fry
    fill(x + 42, y - 2, 3, 2, cone); fill(x + 42, y - 2, 3, 1, ketchup); dot(x + 43, y - 2, 0xf05a68); // ketchup cup
  },
  /** A pair of squeezy ketchup and mustard bottles side by side, red and yellow with white caps, one tipped a touch. */
  dinercondiments(x, y) {
    const red = 0xe03a4a, yellow = 0xf2c040, cap = 0xf0ece0;
    const lean = Math.floor(now + x) % 7 < 1 ? 1 : 0;
    fill(x + 37, y - 6, 3, 6, red); fill(x + 37, y - 6, 1, 6, 0xf05a68); fill(x + 38, y - 8, 1, 2, red); fill(x + 38, y - 9, 1, 1, cap); // ketchup
    fill(x + 41 + lean, y - 6, 3, 6, yellow); fill(x + 41 + lean, y - 6, 1, 6, 0xf6d460); fill(x + 42 + lean, y - 8, 1, 2, yellow); fill(x + 42 + lean, y - 9, 1, 1, cap); // mustard
    dot(x + 37, y - 4, WHITE, 0.4);
  },
  /** A thick white diner coffee mug beside a glass pot of coffee on a warmer; a thread of steam rises from the mug. */
  dinercoffee(x, y) {
    const mug = 0xf0ece0, mugShade = 0xd0ccc0, coffee = 0x3a2416, pot = 0xcfe0e4, warmer = 0x8a8e96;
    fill(x + 37, y - 4, 4, 4, mug); fill(x + 37, y - 4, 4, 1, 0xf8f4ea); fill(x + 37, y, 4, 1, mugShade); fill(x + 38, y - 4, 2, 1, coffee); fill(x + 41, y - 3, 1, 2, mug); // mug + handle
    fill(x + 42, y, 3, 1, warmer); fill(x + 42, y - 5, 3, 5, pot, 0.6); fill(x + 42, y - 2, 3, 2, coffee); fill(x + 43, y - 6, 1, 1, 0x3a3a42); // pot on warmer
    const st = (now + x * 0.3) % 4; if (st < 2) dot(x + 39, y - 6 - Math.floor(st * 2), WHITE, 0.3 - st * 0.1);
  },
  /** A chrome tabletop jukebox selector: a little domed unit with a song-title card and page-flip tabs; a light blinks as a record is picked. */
  dinerselector(x, y) {
    const chrome = 0xc8ccd2, chromeLight = 0xe8ecf0, chromeDark = 0x8a8e96, card = 0xf0ece0, red = 0xe03a4a;
    fill(x + 37, y - 5, 7, 5, chrome); fill(x + 37, y - 5, 7, 1, chromeLight); fill(x + 37, y, 7, 1, chromeDark); // body
    fill(x + 38, y - 7, 5, 2, chrome); fill(x + 38, y - 7, 5, 1, chromeLight); // the dome top
    fill(x + 38, y - 4, 5, 3, card); for (let r = 0; r < 2; r++) fill(x + 39, y - 3 + r, 3, 1, 0x8a8e96, 0.7); // song card
    const on = (now + x * 0.3) % 2 < 1; dot(x + 43, y - 6, on ? red : 0x8a2028); // blinking light
    for (let k = 0; k < 3; k++) dot(x + 38 + k * 2, y - 1, chromeDark); // page tabs
  },
  /** A chrome napkin caddy flanked by salt and pepper shakers, a couple of napkins fanning from the top. */
  dinernapkins(x, y) {
    const chrome = 0xc8ccd2, chromeLight = 0xe8ecf0, chromeDark = 0x8a8e96, napkin = 0xf0ece0, red = 0xe03a4a;
    fill(x + 38, y - 4, 4, 4, chrome); fill(x + 38, y - 4, 4, 1, chromeLight); fill(x + 38, y, 4, 1, chromeDark); // caddy
    fill(x + 39, y - 6, 2, 2, napkin); dot(x + 38, y - 5, napkin); dot(x + 41, y - 5, napkin); // napkins
    fill(x + 35, y - 3, 2, 3, napkin); fill(x + 35, y - 4, 2, 1, chromeDark); dot(x + 35, y - 3, 0xc8ccd2); // salt
    fill(x + 43, y - 3, 2, 3, red); fill(x + 43, y - 4, 2, 1, chromeDark); // pepper
  },
  /** A pink-frosted donut with rainbow sprinkles on a little plate; the frosting catches the light. */
  dinerdonut(x, y) {
    const dough = 0xe0a860, frost = 0xf2a0c0, plate = 0xf0ece0, spr = [0xe03a4a, 0x4ab8a8, 0xf2c040, 0xf0ece0];
    fill(x + 35, y - 1, 10, 2, plate); fill(x + 35, y - 1, 10, 1, 0xf8f4ea); fill(x + 35, y + 1, 10, 1, 0xc8c0ac); // plate
    disc(x + 40, y - 4, 3.4, dough); disc(x + 40, y - 4, 3.4, frost, 0); fill(x + 37, y - 6, 7, 3, frost); fill(x + 37, y - 5, 7, 1, 0xf6b8d0); // donut + frosting
    disc(x + 40, y - 4, 1.2, plate); // the hole
    for (const [dx, dy, k] of [[38, -6, 0], [41, -5, 1], [39, -4, 2], [42, -6, 3], [40, -6, 0]]) dot(x + dx, y + dy, spr[k]); // sprinkles
  },
  /** A gavel resting on its round sound block; every so often it's rapped down twice with a little impact flash. */
  courtgavel(x, y) {
    const wood = 0x7a5232, woodLight = 0x9a6c42, woodDark = 0x4a3220;
    fill(x + 36, y - 1, 8, 2, wood); fill(x + 36, y - 1, 8, 1, woodLight); fill(x + 36, y + 1, 8, 1, woodDark); // the sound block
    const t = (now + x * 0.3) % 9, rap = t < 0.6 && (Math.floor(t * 10) % 2 === 0), yoff = rap ? 0 : -3;
    fill(x + 38, y - 5 + yoff, 4, 3, wood); fill(x + 38, y - 5 + yoff, 4, 1, woodLight); fill(x + 39, y - 2 + yoff, 1, 3, woodDark); // the gavel head + handle
    if (rap) { dot(x + 37, y - 2, WHITE, 0.7); dot(x + 43, y - 2, WHITE, 0.7); }
  },
  /** A small brass desk scales of justice on a marble base; the beam tips gently back and forth to level. */
  courtscalesmini(x, y) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, marble = 0xd8d0c0;
    const tip = Math.round(Math.sin(now * 1.3 + x * 0.2));
    fill(x + 38, y - 1, 5, 1, marble); fill(x + 40, y - 8, 1, 7, brass); dot(x + 40, y - 9, brassLight); // base + column
    fill(x + 36, y - 8 + tip, 9, 1, brass); // beam
    fill(x + 35, y - 6 + tip, 3, 1, brass); fill(x + 43, y - 10 - tip, 3, 1, brass); // the two pans
    dot(x + 36, y - 8 + tip, brassLight);
  },
  /** A short stack of thick law books with gold-banded spines and a bookmark ribbon hanging out. */
  courtlawbooks(x, y) {
    const cols = [0x7a2a2e, 0x2a5a3a, 0xa8824a], gold = 0xc8a050;
    for (let k = 0; k < 3; k++) { const by = y - 1 - k * 2, c = cols[k], w = 9 - k; fill(x + 36, by, w, 2, c); fill(x + 36, by, w, 1, shaded(c, 1.3)); fill(x + 36, by, w, 1, gold, 0.0); dot(x + 36, by, gold); dot(x + 36 + w - 1, by, gold); }
    fill(x + 36, y - 6, 1, 3, 0xc8a050); // gold band on top book
    fill(x + 42, y - 1, 1, 3, 0xd83040); // bookmark ribbon
  },
  /** A glass inkwell of dark ink with a brass lid and a white feather quill standing in it; the quill tip glints. */
  courtinkwell(x, y) {
    const glass = 0x2a3a4a, brass = 0xc8a050, ink = 0x1a1a2a, feather = 0xf0ece0;
    fill(x + 37, y - 4, 5, 4, glass); fill(x + 37, y - 4, 5, 1, 0x3e5262); fill(x + 37, y, 5, 1, 0x14202c); // the well
    fill(x + 38, y - 5, 3, 1, brass); fill(x + 38, y - 4, 3, 1, ink); // brass rim + ink
    line(x + 41, y - 3, x + 44, y - 11, feather); line(x + 41, y - 3, x + 43, y - 11, 0xd8d4c8); dot(x + 44, y - 11, feather); dot(x + 41, y - 3, 0x2a2a30); // quill
  },
  /** A polished brass desk nameplate on a little stand with an engraved line of lettering; a glint sweeps across it. */
  courtnameplate(x, y) {
    const brass = 0xc8a050, brassLight = 0xe8c878, brassDark = 0x8a6a2a, wood = 0x4a3220;
    fill(x + 35, y - 1, 10, 1, wood); // the wooden base
    fill(x + 35, y - 4, 10, 3, brass); fill(x + 35, y - 4, 10, 1, brassLight); fill(x + 35, y - 1, 10, 1, brassDark);
    for (let k = 0; k < 7; k++) fill(x + 37 + k, y - 3, 1, 1, brassDark, 0.8); // engraved lettering
    const t = (now + x * 0.3) % 6; if (t < 0.5) dot(x + 36 + Math.floor(t * 16), y - 3, WHITE, 0.8);
  },
  /** A cardboard coffee cup beside a manila case folder stuffed with papers and tabbed with a red sticker; steam lifts from the cup. */
  courtcoffee(x, y) {
    const cup = 0xe8e0d0, lid = 0xd0c8b4, folder = 0xd8a850, paper = 0xf0ece0, tab = 0xd83040;
    fill(x + 37, y - 5, 3, 5, cup); fill(x + 37, y - 5, 3, 1, lid); fill(x + 37, y - 6, 3, 1, lid); fill(x + 37, y, 3, 1, 0xc0b8a4); // cup
    fill(x + 41, y - 3, 4, 3, folder); fill(x + 41, y - 3, 4, 1, 0xe8bc70); fill(x + 41, y - 4, 3, 1, paper); fill(x + 44, y - 4, 1, 1, tab); // folder + papers + tab
    const st = (now + x * 0.3) % 4; if (st < 2) dot(x + 38, y - 7 - Math.floor(st * 2), WHITE, 0.3 - st * 0.1);
  },
  /** A bailiff's gold star badge on a little stand, points and a central shield picked out; a point glints. */
  courtbadge(x, y) {
    const gold = 0xc8a050, goldLight = 0xe8c878, goldDark = 0x8a6a2a, blue = 0x2a4a7a;
    fill(x + 39, y - 1, 2, 1, 0x4a3220); // stand
    const cx = x + 40, cy = y - 5;
    for (const [dx, dy] of [[0, -4], [0, 4], [-4, 0], [4, 0], [-3, -3], [3, 3], [3, -3], [-3, 3]]) { line(cx, cy, cx + dx, cy + dy, gold); dot(cx + dx, cy + dy, goldLight); }
    disc(cx, cy, 2.4, gold); disc(cx, cy, 1.4, blue); dot(cx, cy - 1, goldLight);
    if ((now + x * 0.3) % 5 < 0.4) dot(cx, cy - 3, WHITE);
    void goldDark;
  },
  /** A round rubber seal stamp with a turned wooden handle, pressed down onto a document with an embossed gold ring every so often. */
  courtstamp(x, y) {
    const handle = 0x6a4a2e, handleLight = 0x8a6440, metal = 0x8a8e96, paper = 0xf0ece0, gold = 0xc8a050;
    fill(x + 36, y - 2, 6, 2, paper); fill(x + 36, y - 2, 6, 1, 0xf8f4ea); // the document
    const t = (now + x * 0.3) % 7, press = t < 0.5, yoff = press ? 0 : -3;
    fill(x + 38, y - 5 + yoff, 3, 2, metal); fill(x + 39, y - 8 + yoff, 1, 3, handle); fill(x + 38, y - 9 + yoff, 3, 1, handle); fill(x + 38, y - 9 + yoff, 3, 1, handleLight); // stamp
    if (press) ring(x + 39, y - 2, 1.4, gold, 0.9); // the embossed seal
  },
  /** A kidney-shaped wooden palette loaded with blobs of bright paint and a brush resting across it; the colours glisten. */
  artpalettemini(x, y) {
    const wood = 0xc8a060, woodLight = 0xe0bc78, cols = [0xe8404a, 0x4a8ad8, 0xf2c040, 0x4ac86a, 0xff5ab0];
    fill(x + 35, y - 3, 9, 3, wood); fill(x + 35, y - 3, 9, 1, woodLight); fill(x + 35, y, 9, 1, 0x9a7840); dot(x + 42, y - 1, 0xd8d0c0); // palette + thumb hole
    for (let k = 0; k < 5; k++) dot(x + 36 + k, y - 2, cols[k]); // paint blobs
    line(x + 36, y - 4, x + 43, y - 1, 0x8a6a3a); dot(x + 36, y - 4, 0xe8404a); // a brush
  },
  /** A ceramic jar crammed with paintbrushes of different sizes, their bristles tipped in dried colour; one leans out a little. */
  artbrushes(x, y) {
    const jar = 0xcfe6ee, jarDark = 0x9ab0ba, handle = 0x8a6a3a;
    fill(x + 37, y - 4, 6, 4, jar, 0.7); fill(x + 37, y - 4, 6, 1, 0xe8f4f8); fill(x + 37, y, 6, 1, jarDark); ring(x + 40, y - 4, 3, jarDark, 0.5);
    const cols = [0xe8404a, 0x4a8ad8, 0xf2c040, 0x4ac86a, 0x8a5ad8];
    for (let k = 0; k < 5; k++) { const bx = x + 38 + k, lean = (k === 4) ? 1 : 0, top = y - 10 - (k % 2); fill(bx + lean, top, 1, (y - 4) - top, handle); dot(bx + lean, top, cols[k]); }
  },
  /** A couple of squeezed-out oil-paint tubes with the caps off and bright paint oozing from the ends, labels crinkled. */
  artpaints(x, y) {
    const tube = 0xd8d4cc, tubeDark = 0xaaa698;
    for (const [tx, c, lean] of [[36, 0xe8404a, 0], [40, 0x4a8ad8, 1]]) {
      fill(x + tx, y - 2 - lean, 5, 3, tube); fill(x + tx, y - 2 - lean, 5, 1, 0xe8e4dc); fill(x + tx, y + 1 - lean, 5, 1, tubeDark); // tube body
      fill(x + tx + 4, y - 2 - lean, 1, 3, tubeDark); // the crimped end
      fill(x + tx - 1, y - 2 - lean, 1, 2, 0xc8c4bc); dot(x + tx - 2, y - 2 - lean, c); dot(x + tx - 2, y - 1 - lean, c); // oozing paint from the nozzle
      fill(x + tx + 1, y - 1 - lean, 2, 1, c, 0.6); // label stripe
    }
  },
  /** A small canvas on a tabletop easel with a half-finished landscape sketched on it — a horizon, a sun and a few strokes; a brush rests on the tray. */
  artcanvasmini(x, y) {
    const wood = 0xc8a060, woodDark = 0x9a7840, canvas = 0xeae2cc, sky = 0xbfe0f0, land = 0x8aae5a, sun = 0xf2c040;
    fill(x + 38, y - 8, 7, 8, canvas); fill(x + 38, y - 8, 7, 1, 0xf0ece0); // the canvas
    fill(x + 38, y - 8, 7, 3, sky); fill(x + 38, y - 5, 7, 3, land); disc(x + 43, y - 7, 1.2, sun); line(x + 39, y - 5, x + 44, y - 4, 0x6a8e3a, 0.7); // the sketch
    fill(x + 40, y - 1, 3, 2, woodDark); fill(x + 38, y, 1, 1, wood); fill(x + 44, y, 1, 1, wood); // easel legs + tray
    line(x + 39, y - 1, x + 43, y - 1, 0x8a6a3a); dot(x + 39, y - 1, 0xe8404a); // a brush on the tray
  },
  /** A small unfinished clay bust on a turntable: a rough head-and-shoulders in grey-brown clay with a sculpting tool stuck in the base. */
  artclay(x, y) {
    const clay = 0x9a8878, clayLight = 0xb8a694, clayDark = 0x6a5e50, base = 0x4a3a2a, tool = 0x8a8e96;
    fill(x + 38, y - 1, 5, 1, base); fill(x + 39, y - 2, 3, 1, 0x5a4a38); // turntable base
    fill(x + 38, y - 6, 5, 4, clay); fill(x + 38, y - 6, 5, 1, clayLight); fill(x + 42, y - 6, 1, 4, clayDark); // shoulders
    fill(x + 39, y - 9, 3, 3, clay); fill(x + 39, y - 9, 1, 3, clayLight); dot(x + 40, y - 8, clayDark); // head
    fill(x + 43, y - 4, 1, 3, tool); dot(x + 43, y - 5, 0xc8ccd2); // sculpting tool
  },
  /** A tin cup bristling with pencils, charcoal sticks and a craft knife; one pencil is sharpened to a fine point. */
  artpencils(x, y) {
    const cup = 0x8a8e96, cupLight = 0xb4b8c0;
    fill(x + 37, y - 4, 6, 4, cup); fill(x + 37, y - 4, 6, 1, cupLight); fill(x + 37, y, 6, 1, 0x5a5e66);
    const items = [[38, 0xf2c040, -10], [39, 0x3a3a40, -11], [40, 0xe8822a, -9], [41, 0x4a8ad8, -11], [42, 0x9a8878, -10]];
    for (const [ix, c, top] of items) { fill(x + ix, y + top, 1, (y - 4) - (y + top), c); if (c === 0xf2c040) { dot(x + ix, y + top, 0xe8d0a0); dot(x + ix, y + top - 1, 0x3a3a30); } else dot(x + ix, y + top, c); }
  },
  /** A glass inkwell of black ink with a dip pen resting in it and a blotted sheet beside; the ink surface shines. */
  artink(x, y) {
    const glass = 0x2a3440, ink = 0x14141e, paper = 0xf0ece0, nib = 0xc8ccd2;
    fill(x + 37, y - 4, 5, 4, glass); fill(x + 37, y - 4, 5, 1, 0x3e4a58); fill(x + 37, y, 5, 1, 0x101620); fill(x + 38, y - 4, 3, 1, ink); // well + ink
    line(x + 40, y - 3, x + 44, y - 11, 0x6a4a2e); fill(x + 44, y - 12, 1, 2, nib); dot(x + 44, y - 10, ink); // dip pen
    fill(x + 34, y - 1, 3, 2, paper); dot(x + 35, y - 1, 0x4a4a52); // a blotted sheet
  },
  /** A jar of murky brush-washing water, the colour swirling greyish-blue, a brush standing in it; a swirl drifts round slowly. */
  artwater(x, y) {
    const jar = 0xcfe0e4, water = 0x5a6a7a, waterLight = 0x7a8a9a, handle = 0x8a6a3a;
    fill(x + 37, y - 5, 6, 5, jar, 0.5); fill(x + 37, y - 5, 6, 1, 0xe8f0f4); fill(x + 37, y, 6, 1, 0x9ab0ba);
    fill(x + 38, y - 3, 4, 3, water); const sw = Math.round(Math.sin(now * 0.8 + x) * 1); dot(x + 39 + sw, y - 2, waterLight); dot(x + 41 - sw, y - 3, 0x4a5a6a); // murky water + swirl
    fill(x + 41, y - 10, 1, 6, handle); dot(x + 41, y - 10, 0xe8404a); // a brush standing in it
  },
  /** A round fishbowl of blue water with a single orange goldfish swimming in slow circles, a pebble and a green sprig; the glass gleams. */
  aquariumbowl(x, y) {
    const glass = 0xbfe4f0, water = 0x2a6a9a, fish = 0xf0824a, weed = 0x3a9a5a;
    disc(x + 40, y - 4, 4.4, water); ring(x + 40, y - 4, 4.4, 0x9ab0ba, 0.7); fill(x + 38, y, 5, 1, 0x8a9aa2); // bowl
    const a = now * 1.5, fx = x + 40 + Math.round(Math.cos(a) * 2), fy = y - 4 + Math.round(Math.sin(a) * 2);
    fill(fx, fy, 2, 1, fish); dot(fx + (Math.cos(a) > 0 ? -1 : 2), fy, fish); // the fish (tail trails behind)
    dot(x + 39, y - 1, 0xc8b48a); fill(x + 42, y - 3, 1, 3, weed); // pebble + sprig
    dot(x + 38, y - 6, WHITE, 0.6);
  },
  /** A branch of bright coral on a little rock: pink and orange fingers reaching up with tiny pale tips; it sits colourful and still. */
  aquariumcoralmini(x, y) {
    const rock = 0x8a8278, pink = 0xf08a9a, orange = 0xf0824a, purple = 0x9a6ad8;
    fill(x + 37, y - 1, 7, 1, rock); fill(x + 38, y - 2, 5, 1, 0x9a9288); // the rock base
    for (const [bx, c, h] of [[38, pink, 5], [40, orange, 6], [42, purple, 4], [39, orange, 3], [43, pink, 4]]) { fill(x + bx, y - 2 - h, 1, h, c); dot(x + bx, y - 2 - h, 0xf4d0d8); dot(x + bx + (bx % 2 ? 1 : -1), y - 1 - h, c); }
  },
  /** A polished nautilus shell on a small stand, its spiral chambers banded in cream and russet; a glint runs along the curve. */
  aquariumnautilus(x, y) {
    const shell = 0xe8dcc0, band = 0xb86a44, shellLight = 0xf4ecd8;
    fill(x + 39, y - 1, 3, 1, 0x5a4a38); // stand
    disc(x + 40, y - 4, 4, shell); disc(x + 42, y - 3, 2, shellLight); // the spiral body + inner whorl
    for (const a of [0.4, 1.4, 2.4, 3.4, 4.4]) { const ex = x + 40 + Math.round(Math.cos(a) * 3.6), ey = y - 4 + Math.round(Math.sin(a) * 3.6); dot(ex, ey, band); }
    ring(x + 40, y - 4, 4, 0xc8a070, 0.6); dot(x + 38, y - 6, WHITE, 0.6);
  },
  /** A classic brass hard-hat diver figurine standing with its round helmet and porthole, a tiny bubble rising from the valve. */
  aquariumdiver(x, y) {
    const suit = 0xc0702a, suitLight = 0xd88a44, helm = 0xc8a050, glass = 0xbfe4f0, boots = 0x6a4a2e;
    fill(x + 38, y - 1, 4, 1, boots); fill(x + 38, y - 5, 4, 4, suit); fill(x + 38, y - 5, 1, 4, suitLight); // body
    fill(x + 38, y - 8, 4, 3, helm); fill(x + 39, y - 7, 2, 1, glass); fill(x + 38, y - 8, 4, 1, 0xe8c878); // helmet + porthole
    fill(x + 37, y - 4, 1, 2, suit); fill(x + 42, y - 4, 1, 2, suit); // arms
    const bub = (y - 8) - Math.floor(now * 3) % 4; dot(x + 42, bub, 0xbfe4f0, 0.8);
  },
  /** A small glass cup holding a sea anemone: a stubby base crowded with soft waving tentacles in pink and green that sway side to side. */
  aquariumanemone(x, y) {
    const cup = 0xbfe4f0, water = 0x2a6a9a, body = 0xf0a0b0, tent = 0xf06a9a, green = 0x6ac86a;
    fill(x + 37, y - 4, 6, 4, water, 0.6); fill(x + 37, y - 4, 6, 1, 0xd8f0f8); fill(x + 37, y, 6, 1, 0x8a9aa2); // cup
    fill(x + 39, y - 2, 2, 2, body); // the column
    const sway = Math.round(Math.sin(now * 1.6) * 1);
    for (let k = 0; k < 6; k++) { const tx = x + 38 + k + (k % 2 ? sway : -sway); dot(tx, y - 4, k % 2 ? tent : green); dot(tx, y - 5, k % 2 ? 0xf890b0 : green, 0.8); }
  },
  /** A clear jar of smooth frosted sea glass in soft blues, greens and white, catching the light; it glints now and then. */
  aquariumseaglass(x, y) {
    const jar = 0xd0e6ee, cols = [0x6ab0c8, 0x7ac8a0, 0xe8e4da, 0x9ab8d8, 0x8ad0b0];
    fill(x + 37, y - 5, 6, 5, jar, 0.5); fill(x + 37, y - 5, 6, 1, 0xe8f4f8); fill(x + 37, y, 6, 1, 0x9ab0ba); fill(x + 38, y - 6, 4, 1, 0xb0c8d0); // jar
    for (let k = 0; k < 7; k++) { const gx = x + 38 + (k % 4), gy = y - 1 - Math.floor(k / 4) * 2; dot(gx, gy, cols[k % 5]); }
    if ((now + x * 0.3) % 6 < 0.4) dot(x + 39, y - 4, WHITE);
  },
  /** A rough oyster shell propped open to show a single round pearl glowing softly on the pale nacre inside. */
  aquariumpearl(x, y) {
    const shell = 0x8a8278, nacre = 0xe0e4e8, pearl = 0xf4f0f4, shellDark = 0x5a564e;
    fill(x + 37, y - 2, 7, 2, shell); fill(x + 37, y - 2, 7, 1, 0x9a9288); // lower shell
    fill(x + 38, y - 4, 5, 2, shell); fill(x + 38, y - 5, 5, 1, shellDark); // upper shell propped open
    fill(x + 38, y - 2, 5, 1, nacre); // the nacre
    disc(x + 40, y - 2, 1.4, pearl); dot(x + 39, y - 3, WHITE); // the pearl
    if (isNight) glowLate(() => glowAt(x + 40, y - 2, 2, 2, 0xe8e0f0, 0.14));
  },
  /** A little yellow toy submarine with a round porthole, a red propeller and a periscope; the propeller spins and a bubble rises. */
  aquariumsub(x, y) {
    const body = 0xf2c040, bodyLight = 0xf6d870, bodyDark = 0xc89820, glass = 0xbfe4f0, prop = 0xe03a4a;
    fill(x + 36, y - 4, 8, 4, body); fill(x + 36, y - 4, 8, 1, bodyLight); fill(x + 36, y, 8, 1, bodyDark); fill(x + 35, y - 3, 1, 2, body); fill(x + 44, y - 3, 1, 2, body); // hull
    disc(x + 39, y - 2, 1.2, glass); ring(x + 39, y - 2, 1.4, bodyDark, 0.8); // porthole
    fill(x + 40, y - 6, 1, 2, bodyDark); fill(x + 40, y - 6, 2, 1, bodyDark); // periscope/tower
    const spin = Math.floor(now * 8) % 2; fill(x + 44, y - 3 - spin, 1, 1 + spin, prop); fill(x + 44, y - 2 + (1 - spin), 1, 1, prop); // spinning propeller
    const bub = (y - 7) - Math.floor(now * 3) % 4; dot(x + 41, bub, 0xbfe4f0, 0.7);
  },
  /** A red-and-white striped popcorn box heaped with golden kernels, a couple of them hopping over the rim. */
  circuspopcorn(x, y) {
    const red = 0xd8402a, cream = 0xf2e8d8, pop = 0xf2e0a8; const bx = x + 37;
    for (let k = 0; k < 6; k++) fill(bx + k, y - 5, 1, 5, k % 2 ? red : cream);
    fill(bx, y, 6, 1, 0xaa2a1a);
    for (const [dx, dy] of [[0, -6], [2, -7], [4, -6], [1, -5], [3, -5], [5, -5]]) { const hop = (Math.floor(now * 3) + dx) % 7 === 0 ? -1 : 0; dot(bx + dx, y + dy + hop, pop); dot(bx + dx, y + dy + hop - 1, 0xfff0c8); }
  },
  /** A black magician's top hat with a red band and a wand; every few seconds a little gold star puffs up out of the crown. */
  circustophat(x, y) {
    const hat = 0x1e1e26, hatL = 0x3a3a46, red = 0xd8402a, gold = 0xe8b84a; const bx = x + 37;
    fill(bx - 1, y - 1, 8, 1, hat); fill(bx - 1, y - 1, 8, 1, hatL); fill(bx + 1, y - 7, 4, 6, hat); fill(bx + 1, y - 7, 1, 6, hatL);
    fill(bx + 1, y - 3, 4, 1, red);
    fill(bx + 6, y - 6, 1, 6, 0x2a2a2e); dot(bx + 6, y - 6, 0xf2ead8);
    const t = (now + x * 0.2) % 6; if (t < 1.2) { const sy = y - 8 - Math.round(t * 4); dot(bx + 3, sy, gold); dot(bx + 2, sy, gold, 0.7); dot(bx + 4, sy, gold, 0.7); dot(bx + 3, sy - 1, 0xf2d888); }
  },
  /** A bunch of three bright balloons bobbing on their strings, tied to the desk's corner. */
  circusballoons(x, y) {
    const cols = [0xd8402a, 0xe8b84a, 0x4c8bf5]; const bx = x + 39, bob = Math.round(Math.sin(now * 1.2 + x) * 1);
    const spots = [[-2, -9], [2, -10], [0, -7]];
    for (const [dx, dy] of spots) line(bx + dx, y + dy + bob + 4, bx, y, 0xcfc4a8, 0.6);
    spots.forEach(([dx, dy], i) => { const c = cols[i]; fill(bx + dx - 1, y + dy + bob, 3, 4, c); dot(bx + dx - 1, y + dy + bob, shaded(c, 1.25)); dot(bx + dx, y + dy + bob + 4, c); });
  },
  /** A fat cloud of pink cotton candy on a paper cone, soft highlights round its edge. */
  circuscotton(x, y) {
    const cone = 0xe0d4b8, floss = 0xf2a8c8, flossL = 0xf8c8dc; const bx = x + 39;
    fill(bx, y - 3, 1, 3, cone); fill(bx - 1, y, 3, 1, cone);
    for (const [dx, dy, w, h] of [[-3, -8, 7, 4], [-2, -10, 5, 3], [-1, -6, 3, 2]]) fill(bx + dx, y + dy, w, h, floss);
    dot(bx - 3, y - 7, flossL); dot(bx + 3, y - 7, flossL); dot(bx, y - 10, flossL); dot(bx - 1, y - 9, WHITE, 0.4);
  },
  /** Three juggling clubs tumbling round in a steady cascade over the desk, cream handles and red ends. */
  circuspins(x, y) {
    const red = 0xd8402a, cream = 0xf2e8d8; const bx = x + 39, t = now * 2;
    for (let i = 0; i < 3; i++) { const a = t + i / 3 * Math.PI * 2; const cx = bx + Math.round(Math.cos(a) * 4), cy = y - 6 + Math.round(Math.sin(a) * 5); fill(cx, cy - 2, 1, 4, cream); fill(cx - 1, cy + 1, 3, 2, red); dot(cx, cy - 2, red); }
  },
  /** A little gold stand holding a fan of red-striped ADMIT-ONE tickets. */
  circusticket(x, y) {
    const gold = 0xe8b84a, red = 0xd8402a, cream = 0xf2e8d8; const bx = x + 36;
    fill(bx, y - 1, 9, 1, 0xb88a2a);
    for (let i = 0; i < 3; i++) { const tx = bx + i, ty = y - 2 - i * 2; fill(tx, ty - 3, 6, 4, cream); fill(tx, ty - 3, 6, 1, red); dot(tx + 2, ty - 1, red); dot(tx + 4, ty - 1, red); }
  },
  /** A blue jack-in-the-box with a gold crank; every few seconds the lid flips and a grinning clown springs up on its coil, then ducks back in. */
  circusjackbox(x, y) {
    const box = 0x4c8bf5, boxL = 0x6aa0f8, boxD = 0x2a5ab8, gold = 0xe8b84a; const bx = x + 37;
    fill(bx, y - 5, 7, 5, box); fill(bx, y - 5, 7, 1, boxL); fill(bx, y - 1, 7, 1, boxD); fill(bx, y - 5, 1, 5, boxL);
    dot(bx + 6, y - 3, gold);
    const t = (now + x * 0.3) % 6;
    if (t < 1.6) {
      const up = Math.round(Math.min(1, t / 0.5) * 6);
      for (let k = 0; k < up; k += 2) dot(bx + 3, y - 5 - k, 0xcfc4a8);
      fill(bx + 1, y - 7 - up, 5, 3, 0xf0c89a); fill(bx + 1, y - 8 - up, 5, 1, 0xd8402a); dot(bx + 2, y - 6 - up, PALETTE.eye); dot(bx + 4, y - 6 - up, PALETTE.eye); dot(bx + 3, y - 5 - up, 0xd8402a);
      fill(bx, y - 6, 7, 1, boxD);
    } else fill(bx, y - 6, 7, 1, boxL);
  },
  /** A glossy red candy apple on a stick, with a tiny green leaf and a bright glint. */
  circuscandy(x, y) {
    const red = 0xcc1f1f, redL = 0xf05a40, stick = 0xcfc4a8; const bx = x + 39;
    fill(bx, y - 5, 1, 5, stick);
    disc(bx, y - 7, 3, red); dot(bx - 1, y - 8, redL); dot(bx - 2, y - 7, WHITE, 0.6); dot(bx, y - 10, 0x5aa84a);
  },
  /** A library globe on a brass stand, its painted continents turning slowly. */
  museumglobe(x, y) {
    const brass = 0xc8a44e, sea = 0x3a6a8a, land = 0x6a8a4a, cx = x + 39;
    fill(cx - 2, y - 1, 5, 1, brass); fill(cx, y - 2, 1, 1, brass);
    disc(cx, y - 6, 4, sea);
    const sp = Math.floor(now * 1.5) % 7;
    for (const [a, r] of [[0, 1.5], [2.2, 1], [4.1, 2]]) { const ang = a + sp * 0.5; const gx = cx + Math.round(Math.cos(ang) * 2), gy = y - 6 + Math.round(Math.sin(ang) * 2); if ((gx - cx) ** 2 + (gy - (y - 6)) ** 2 <= 10) fill(gx - 1, gy, 2, 1, land); }
    ring(cx, y - 6, 4.3, brass, 0.7); dot(cx - 1, y - 8, WHITE, 0.5);
  },
  /** A small animal skull specimen on a wooden stand, empty eye sockets and a row of teeth. */
  museumskull(x, y) {
    const bone = 0xe8dcc0, boneD = 0xcfc0a0, stand = 0x3a2e1e; const bx = x + 38;
    fill(bx - 1, y - 1, 6, 1, stand);
    fill(bx, y - 6, 5, 5, bone); fill(bx, y - 6, 5, 1, 0xf2e8cc); fill(bx, y - 2, 5, 1, boneD);
    dot(bx + 1, y - 4, 0x2a2018); dot(bx + 3, y - 4, 0x2a2018);
    fill(bx + 1, y - 1, 3, 1, bone); dot(bx + 1, y - 1, boneD); dot(bx + 3, y - 1, boneD);
  },
  /** A brass microscope on the desk, its eyepiece catching the light. */
  museummicroscope(x, y) {
    const brass = 0xc8a44e, brassL = 0xe8c878, base = 0x3a2e1e; const bx = x + 38;
    fill(bx - 1, y - 1, 6, 1, base);
    fill(bx, y - 2, 4, 1, brass); fill(bx + 1, y - 9, 2, 7, brass); fill(bx + 1, y - 9, 1, 7, brassL);
    fill(bx, y - 11, 4, 2, brass); fill(bx + 3, y - 13, 1, 3, brass); dot(bx + 3, y - 13, brassL);
    fill(bx + 1, y - 4, 2, 1, 0x2a2620); dot(bx + 2, y - 13, WHITE, 0.5);
  },
  /** An ammonite coiled in a pale stone slab, its spiral chambers picked out in bone. */
  museumammonite(x, y) {
    const slab = 0x8a8274, slabD = 0x6a6256, bone = 0xcfc0a0, boneL = 0xe8dcc0; const bx = x + 39;
    fill(bx - 3, y - 6, 7, 6, slab); fill(bx - 3, y - 6, 7, 1, 0xa39a88); fill(bx - 3, y - 1, 7, 1, slabD);
    const cx = bx, cy = y - 3; for (let k = 0; k < 16; k++) { const a = k * 0.55, r = 0.4 + k * 0.17; dot(cx + Math.round(Math.cos(a) * r), cy + Math.round(Math.sin(a) * r), k % 2 ? bone : boneL); }
  },
  /** A specimen bell jar with a little curled creature floating inside, bobbing gently. */
  museumjar(x, y) {
    const glass = 0xbfe0e8, base = 0x3a2e1e, spec = 0xc88a6a; const bx = x + 38;
    fill(bx - 1, y - 1, 6, 1, base); fill(bx - 1, y - 2, 6, 1, 0x5a4a2e);
    fill(bx, y - 9, 4, 7, glass, 0.5); fill(bx, y - 9, 1, 7, glass, 0.7); fill(bx + 3, y - 9, 1, 7, glass, 0.3);
    fill(bx, y - 10, 4, 1, 0x8a8276); dot(bx + 1, y - 11, 0x8a8276);
    const bob = Math.round(Math.sin(now * 1.5 + x) * 1); fill(bx + 1, y - 6 + bob, 2, 3, spec); dot(bx + 1, y - 6 + bob, 0xe0a888); dot(bx, y - 8, WHITE, 0.6);
  },
  /** A magnifier on a brass arm over a pinned beetle; the beetle's legs twitch. */
  museummagnifier(x, y) {
    const brass = 0xc8a44e, glass = 0xbfe0e8, bug = 0x3a2e2a; const bx = x + 38;
    fill(bx, y - 1, 3, 2, bug); dot(bx + 1, y - 2, 0x5a4a3a);
    const tw = Math.floor(now * 4) % 2; dot(bx - 1, y - 1 + tw, bug); dot(bx + 3, y - 1 - tw, bug);
    fill(bx + 5, y - 8, 1, 8, brass); fill(bx + 1, y - 9, 5, 1, brass);
    ring(bx + 1, y - 6, 2.4, brass); disc(bx + 1, y - 6, 2, glass, 0.4); dot(bx, y - 7, WHITE, 0.6);
  },
  /** A dark pitted meteorite on a brass ring stand; a metallic glint travels across it. */
  museummeteorite(x, y) {
    const rock = 0x4a4038, rockL = 0x6a5e52, rockD = 0x2e2620, metal = 0xb8a890, stand = 0x8a6a3a; const bx = x + 38;
    fill(bx - 1, y - 1, 6, 1, stand); fill(bx, y - 2, 4, 1, stand);
    fill(bx, y - 6, 5, 4, rock); fill(bx, y - 6, 5, 1, rockL); fill(bx, y - 3, 5, 1, rockD); fill(bx + 4, y - 6, 1, 4, rockD);
    dot(bx + 1, y - 5, rockD); dot(bx + 3, y - 4, rockD);
    const g = Math.floor(now * 2) % 4; dot(bx + g, y - 5, metal); dot(bx + g, y - 5, WHITE, 0.5);
  },
  /** A trilobite fossil in a dark slab, its segmented body and two little eyes picked out in bone. */
  museumtrilo(x, y) {
    const slab = 0x4a4238, slabL = 0x6a6256, bone = 0xcfc0a0, boneD = 0x9a8e74; const bx = x + 37;
    fill(bx, y - 6, 8, 6, slab); fill(bx, y - 6, 8, 1, slabL); fill(bx, y - 1, 8, 1, 0x2e2822);
    fill(bx + 2, y - 5, 4, 1, boneD); for (let k = 0; k < 4; k++) fill(bx + 2, y - 4 + k, 4 - (k > 2 ? 1 : 0), 1, k % 2 ? bone : boneD);
    dot(bx + 2, y - 5, bone); dot(bx + 5, y - 5, bone);
  },
  /** A red present tied with a gold ribbon, its bow giving a little bounce now and then. */
  northpolegift(x, y) {
    const box = 0xc62828, boxD = 0x9a1e1e, ribbon = 0xe8c24a, ribbonL = 0xf2d878, bx = x + 37;
    fill(bx, y - 5, 7, 5, box); fill(bx, y - 5, 7, 1, 0xe34b3c); fill(bx, y - 1, 7, 1, boxD);
    fill(bx + 3, y - 5, 1, 5, ribbon); fill(bx, y - 3, 7, 1, ribbon);
    const b = Math.floor(now * 2) % 7 === 0 ? -1 : 0; fill(bx + 2, y - 7 + b, 1, 2, ribbon); fill(bx + 4, y - 7 + b, 1, 2, ribbon); dot(bx + 3, y - 6 + b, ribbonL);
  },
  /** A red mug of cocoa with two marshmallows and a thread of rising steam. */
  northpolecocoa(x, y) {
    const mug = 0xc62828, mugD = 0x9a1e1e, cocoa = 0x4a2a18, mallow = 0xf2f0e8, bx = x + 38;
    fill(bx, y - 5, 5, 5, mug); fill(bx, y - 5, 5, 1, cocoa); fill(bx, y - 5, 1, 5, 0xe34b3c); fill(bx + 4, y - 5, 1, 5, mugD); fill(bx + 5, y - 4, 1, 2, mugD);
    dot(bx + 1, y - 5, mallow); dot(bx + 3, y - 5, mallow);
    const t = Math.floor(now * 3) % 3; dot(bx + 1, y - 7 - t, mallow, 0.4); dot(bx + 3, y - 8 - t, mallow, 0.3);
  },
  /** A plate of chocolate-chip cookies and a glass of milk, left out for Santa. */
  northpolecookies(x, y) {
    const plate = 0xdfe8f0, cookie = 0xc89a5a, choc = 0x5a3420, milk = 0xf2f6fb, bx = x + 36;
    fill(bx, y - 1, 8, 1, plate); fill(bx + 1, y - 2, 6, 1, plate);
    disc(bx + 2, y - 3, 2, cookie); dot(bx + 1, y - 3, choc); dot(bx + 3, y - 2, choc);
    disc(bx + 5, y - 3, 1.5, cookie); dot(bx + 5, y - 3, choc);
    fill(bx + 7, y - 5, 2, 4, milk, 0.7); fill(bx + 7, y - 5, 2, 1, 0xfbfdff);
  },
  /** A snow globe with a little pine inside; give it a few seconds and the snow swirls up. It glows softly at night. */
  northpolesnowglobe(x, y) {
    const glass = 0xcfe8f0, base = 0x6a4428, baseL = 0x8a5a32, tree = 0x2e6b3e, snow = 0xf2f6fb, bx = x + 39;
    fill(bx - 3, y - 1, 7, 1, base); fill(bx - 3, y - 2, 7, 1, baseL);
    disc(bx, y - 6, 4, glass, 0.55); ring(bx, y - 6, 4, 0xaecad8, 0.6);
    fill(bx - 1, y - 6, 1, 3, tree); fill(bx - 2, y - 5, 3, 1, tree); fill(bx - 2, y - 7, 3, 1, tree);
    const sh = (now + x * 0.3) % 10 < 1.5; for (let k = 0; k < 5; k++) { const a = now * 3 + k; dot(bx + Math.round(Math.cos(a) * 2), y - 6 + Math.round(Math.sin(a) * 2) + (sh ? 0 : 1), snow, sh ? 0.9 : 0.5); }
    dot(bx - 1, y - 8, WHITE, 0.6);
    if (isNight) glowLate(() => glowAt(bx, y - 6, 4, 4, 0xbfe0f0, 0.14));
  },
  /** A toy nutcracker soldier in a red coat and a tall shako, rocking stiffly on his base now and then. */
  northpolesoldier(x, y) {
    const coat = 0xc62828, hat = 0x2a2a2e, gold = 0xe8c24a, skin = 0xf0c89a, white = 0xf2f6fb, bx = x + 38;
    const rock = Math.floor(now * 1.5) % 9 === 0 ? 1 : 0;
    fill(bx, y - 2, 4, 2, hat);
    fill(bx, y - 7, 4, 5, coat); fill(bx, y - 7, 4, 1, 0xe34b3c); fill(bx + 1, y - 5, 2, 1, gold);
    fill(bx, y - 9, 4, 2, skin); dot(bx + 1, y - 9, PALETTE.eye); dot(bx + 2, y - 9, PALETTE.eye); fill(bx, y - 8, 4, 1, white);
    fill(bx, y - 12, 4, 3, hat); fill(bx, y - 12, 4, 1, gold); dot(bx + 3 + rock, y - 6, coat);
  },
  /** A brass sleigh bell on a red strap; it swings and jingles every few seconds. */
  northpolebell(x, y) {
    const brass = 0xe8c24a, brassL = 0xf2d878, brassD = 0xb8902a, strap = 0xc62828, bx = x + 39;
    fill(bx - 2, y - 9, 5, 1, strap); fill(bx - 2, y - 9, 5, 1, 0xe34b3c);
    const sw = Math.round(Math.sin(now * 6) * ((now + x * 0.2) % 8 < 2 ? 1 : 0));
    disc(bx + sw, y - 5, 3, brass); dot(bx - 1 + sw, y - 6, brassL); fill(bx + sw, y - 4, 1, 2, brassD); dot(bx + sw, y - 2, brassD); dot(bx - 1 + sw, y - 8, brassL, 0.6);
  },
  /** A red-and-white striped candy cane leaning on the desk, a bright glint on its curve. */
  northpolecandycane(x, y) {
    const red = 0xd83030, white = 0xf2f6fb, bx = x + 39;
    for (let k = 0; k < 7; k++) fill(bx, y - 1 - k, 2, 1, k % 2 ? red : white);
    fill(bx, y - 9, 2, 1, red); fill(bx + 1, y - 10, 2, 1, white); fill(bx + 2, y - 9, 1, 2, red); dot(bx, y - 5, WHITE, 0.6);
  },
  /** A little wind-up toy train that chuffs along, wheels turning and a puff rising from its stack. */
  northpoletrain(x, y) {
    const body = 0x2e6b3e, bodyD = 0x1f4e2c, black = 0x2a2a2e, gold = 0xe8c24a, smoke = 0xdfe8f0, red = 0xd83030, bx = x + 35;
    fill(bx, y - 4, 9, 4, body); fill(bx, y - 4, 9, 1, 0x3f8a52); fill(bx, y - 1, 9, 1, bodyD);
    fill(bx + 6, y - 7, 3, 4, body); fill(bx + 6, y - 7, 3, 1, 0x3f8a52);
    fill(bx + 1, y - 6, 1, 3, black); dot(bx + 1, y - 7, gold);
    fill(bx, y, 9, 1, black); const wsp = Math.floor(now * 6) % 2; dot(bx + 1, y, wsp ? gold : red); dot(bx + 4, y, wsp ? red : gold); dot(bx + 7, y, wsp ? gold : red);
    const t = Math.floor(now * 3) % 3; dot(bx + 1, y - 8 - t, smoke, 0.7 - t * 0.2); dot(bx, y - 9 - t, smoke, 0.4);
  },
  /** A film clapperboard; its striped stick lifts and snaps shut every few seconds with a little flash. */
  soundstageclapper(x, y) {
    const black = 0x1a1a1e, white = 0xe8e4d8, body = 0x2a2a30, bx = x + 36;
    fill(bx, y - 4, 9, 4, body); fill(bx, y - 4, 9, 1, 0x3a3a40); fill(bx + 1, y - 2, 7, 1, 0x3a3a40); dot(bx + 2, y - 2, 0x5a5a60);
    const t = (now + x * 0.3) % 4, lift = t < 0.6 ? Math.round((0.6 - t) * 6) : 0;
    for (let k = 0; k < 9; k++) fill(bx + k, y - 5 - lift, 1, 1, (k + (lift ? 1 : 0)) % 2 ? black : white);
    if (t < 0.12) dot(bx + 4, y - 7, WHITE, 0.8);
  },
  /** A red director's megaphone with a silver rim; a little puff escapes the cone now and then. */
  soundstagemegaphone(x, y) {
    const red = 0xd83030, silver = 0xb8bcc4, dark = 0x2a2a2e, bx = x + 37;
    fill(bx, y - 4, 3, 4, red); fill(bx, y - 4, 1, 4, 0xf05a40);
    fill(bx + 3, y - 5, 4, 6, red); fill(bx + 6, y - 6, 2, 8, red); fill(bx + 7, y - 6, 1, 8, silver); fill(bx + 1, y - 5, 1, 2, dark);
    if (Math.floor(now * 3) % 6 === 0) { dot(bx + 9, y - 2, WHITE, 0.6); dot(bx + 10, y - 3, WHITE, 0.3); }
  },
  /** A little gold award statuette on a black base; a glint sweeps across it now and then. */
  soundstageaward(x, y) {
    const gold = 0xe8c24a, goldL = 0xf2d878, goldD = 0xb8902a, base = 0x1a1a1e, bx = x + 39;
    fill(bx - 1, y - 1, 5, 1, base); fill(bx - 1, y - 2, 5, 1, 0x2a2a2e);
    fill(bx, y - 4, 3, 2, gold); fill(bx, y - 8, 3, 4, gold); fill(bx, y - 8, 1, 4, goldL); fill(bx + 2, y - 8, 1, 4, goldD);
    fill(bx, y - 10, 3, 2, gold); dot(bx + 1, y - 10, goldL); fill(bx - 1, y - 7, 1, 2, gold); fill(bx + 3, y - 7, 1, 2, gold);
    if ((now + x * 0.2) % 5 < 0.4) dot(bx + 1, y - 9, WHITE, 0.9);
  },
  /** A bound shooting script with a red cover and brass brads; a page flips now and then. */
  soundstagescript(x, y) {
    const paper = 0xe8e4d8, paperD = 0xc8c2b2, cover = 0xd83030, brad = 0xc8a44e, bx = x + 37;
    fill(bx, y - 5, 8, 5, paper); fill(bx, y - 5, 8, 1, 0xfbf6ea); fill(bx, y - 1, 8, 1, paperD); fill(bx, y - 5, 2, 5, cover);
    dot(bx, y - 4, brad); dot(bx, y - 2, brad); fill(bx + 3, y - 4, 4, 1, 0x7a7a72); fill(bx + 3, y - 2, 3, 1, 0x7a7a72);
    if ((now + x * 0.3) % 5 < 0.4) { fill(bx + 5, y - 5, 3, 5, paperD); fill(bx + 5, y - 5, 1, 5, paper); }
  },
  /** A little fresnel spotlight on a stand with barn doors, casting a warm pool; it glows at night. */
  soundstagespotlight(x, y) {
    const body = 0x3a3e46, bodyL = 0x5a5e66, warm = 0xffcf6b, stand = 0x2a2a2e, bx = x + 38;
    fill(bx + 1, y - 1, 2, 1, stand); fill(bx + 1, y - 4, 1, 3, stand);
    fill(bx, y - 8, 5, 4, body); fill(bx, y - 8, 5, 1, bodyL); fill(bx, y - 4, 5, 1, 0x1a1a1e);
    fill(bx + 1, y - 7, 3, 2, warm); dot(bx + 2, y - 7, 0xffe9b0); fill(bx - 1, y - 8, 1, 3, body); fill(bx + 5, y - 8, 1, 3, body);
    for (let b = 0; b < 4; b++) dot(bx + 1 + (b % 3), y - 3 + b, warm, 0.25 - b * 0.04);
    if (isNight) glowLate(() => glowAt(bx + 2, y - 2, 4, 4, warm, 0.16));
  },
  /** A vintage movie camera with two round film magazines and a lens; its hand crank turns and a red tally glows. */
  soundstagecamera(x, y) {
    const body = 0x2a2a30, bodyL = 0x4a4a52, metal = 0x8a8e96, lens = 0x1a1a1e, red = 0xd83030, bx = x + 35;
    fill(bx, y - 6, 8, 6, body); fill(bx, y - 6, 8, 1, bodyL); fill(bx, y - 1, 8, 1, 0x18181c);
    disc(bx + 2, y - 8, 2, body); ring(bx + 2, y - 8, 2, metal); disc(bx + 6, y - 8, 2, body); ring(bx + 6, y - 8, 2, metal);
    fill(bx + 8, y - 5, 2, 3, lens); ring(bx + 9, y - 4, 1.4, metal);
    const a = now * 3; dot(bx - 1 + Math.round(Math.cos(a) * 1.5), y - 4 + Math.round(Math.sin(a) * 1.5), metal); dot(bx + 4, y - 4, red);
  },
  /** A stack of three silver film-reel cans with a little red label on top. */
  soundstagereelcan(x, y) {
    const can = 0x9a9ea6, canL = 0xc8ccd4, canD = 0x6a6e76, label = 0xe8e4d8, bx = x + 37;
    for (let i = 0; i < 3; i++) { const cy = y - 1 - i * 2; fill(bx, cy - 2, 8, 2, i % 2 ? can : canL); fill(bx, cy - 2, 8, 1, canL); fill(bx, cy, 8, 1, canD); }
    fill(bx + 1, y - 6, 4, 1, label); dot(bx + 2, y - 6, 0xd83030); dot(bx + 1, y - 4, WHITE, 0.5);
  },
  /** A paper to-go coffee cup with a sip lid and a kraft sleeve, a thread of steam rising. */
  soundstagecoffee(x, y) {
    const cup = 0xe8e4d8, cupD = 0xc8c2b2, lid = 0x3a3a40, sleeve = 0x8a5a32, bx = x + 38;
    fill(bx, y - 6, 4, 6, cup); fill(bx, y - 6, 1, 6, 0xfbf6ea); fill(bx + 3, y - 6, 1, 6, cupD);
    fill(bx, y - 7, 4, 1, lid); dot(bx + 1, y - 8, lid); fill(bx, y - 3, 4, 2, sleeve); fill(bx, y - 3, 4, 1, 0xa8703e);
    const t = Math.floor(now * 3) % 3; dot(bx + 1, y - 9 - t, cup, 0.4); dot(bx + 2, y - 10 - t, cup, 0.3);
  },
  /** A fat pillar candle with a steady flame; it glows warm at night. */
  cultcandle(x, y) {
    const wax = 0xe8e0cc, waxD = 0xcfc6ad, flame = 0xffd36b, flameC = 0xfff0b0, bx = x + 38;
    fill(bx, y - 6, 5, 6, wax); fill(bx, y - 6, 1, 6, 0xf2ead8); fill(bx + 4, y - 6, 1, 6, waxD); fill(bx, y - 6, 5, 1, waxD);
    const f = candlelight(x * 0.3); fill(bx + 2, y - 9, 1, 3, flame, f); dot(bx + 2, y - 10, flameC, f); dot(bx + 2, y - 7, flame);
    if (isNight) glowLate(() => glowAt(bx + 2, y - 8, 4, 5, flame, 0.16 * f));
  },
  /** An incense stick in a little holder, its ember glowing and a thread of smoke curling up. */
  cultincense(x, y) {
    const holder = 0xc08a5e, holderL = 0xd8a878, stick = 0x6a4a2a, ember = 0xf05a30, smoke = 0xcfc6ad, bx = x + 39;
    fill(bx - 2, y - 1, 6, 1, holder); fill(bx - 2, y - 1, 6, 1, holderL); fill(bx, y - 7, 1, 6, stick); dot(bx, y - 7, ember);
    const t = now * 1.5; for (let k = 0; k < 6; k++) dot(bx + Math.round(Math.sin(t + k * 0.6) * 1.5), y - 8 - k, smoke, 0.5 - k * 0.07);
  },
  /** A clay offering bowl heaped with fruit. */
  cultbowl(x, y) {
    const bowl = 0xc08a5e, bowlL = 0xd8a878, bowlD = 0x9a6a44, bx = x + 37;
    fill(bx, y - 2, 8, 2, bowl); fill(bx, y - 2, 8, 1, bowlL); fill(bx, y, 8, 1, bowlD);
    dot(bx + 1, y - 3, 0xd06a4a); dot(bx + 3, y - 4, 0xe0b24a); dot(bx + 5, y - 3, 0x8a9a5a); dot(bx + 2, y - 4, 0xd04a6a); dot(bx + 6, y - 4, 0xd06a4a); dot(bx + 4, y - 5, 0x8a9a5a);
  },
  /** A pale amethyst crystal cluster on a stone base; it pulses with a soft inner light, glowing at night. */
  cultcrystal(x, y) {
    const crys = 0xbfa8d8, crysL = 0xe0cff0, crysD = 0x8a6ab0, base = 0xa8926e, bx = x + 39;
    fill(bx - 2, y - 1, 5, 1, base);
    fill(bx, y - 6, 2, 5, crys); dot(bx, y - 6, crysL); fill(bx - 1, y - 4, 1, 3, crysD); fill(bx + 2, y - 4, 1, 3, crys); dot(bx + 2, y - 4, crysL); fill(bx - 2, y - 3, 1, 2, crys);
    const pulse = (Math.sin(now * 1.5) + 1) / 2; dot(bx, y - 5, WHITE, 0.4 + pulse * 0.4);
    if (isNight) glowLate(() => glowAt(bx, y - 4, 3, 3, crys, 0.1 + pulse * 0.1));
  },
  /** A brass ceremonial chalice holding a faintly shimmering violet draught. */
  cultchalice(x, y) {
    const brass = 0xc08a5e, brassL = 0xe0b24a, drink = 0x8a6ab0, drinkL = 0xbfa8d8, bx = x + 39;
    fill(bx - 1, y - 1, 4, 1, brass); fill(bx, y - 3, 2, 2, brass);
    fill(bx - 2, y - 7, 6, 4, brass); fill(bx - 2, y - 7, 6, 1, brassL); fill(bx - 1, y - 6, 4, 1, drink); dot(bx, y - 6, drinkL);
    if (Math.sin(now * 2) > 0.7) dot(bx + 1, y - 6, WHITE, 0.6); dot(bx - 2, y - 6, brassL);
  },
  /** A brass singing bowl on a red cushion; every ~10 s a ring of sound ripples out from its rim. */
  cultbell(x, y) {
    const bowl = 0xc8a44e, bowlL = 0xe0b24a, bowlD = 0x9a7a2a, cushion = 0x8a3a4a, bx = x + 37;
    fill(bx, y - 1, 8, 1, cushion); fill(bx, y - 1, 8, 1, 0xa84a5a);
    fill(bx + 1, y - 4, 6, 3, bowl); fill(bx + 1, y - 4, 6, 1, bowlL); fill(bx + 1, y - 2, 6, 1, bowlD);
    const t = (now + x * 0.2) % 10; if (t < 2) ring(bx + 4, y - 4, 4 + t * 3, bowlL, Math.max(0, 0.5 - t * 0.25));
    dot(bx + 2, y - 4, WHITE, 0.4);
  },
  /** A recruitment pamphlet with a big smiling sun-face; a page flutters now and then. */
  cultpamphlet(x, y) {
    const paper = 0xeceae2, paperD = 0xd0ccc0, gold = 0xe0b24a, ink = 0x6a5a4a, bx = x + 37;
    fill(bx, y - 6, 7, 6, paper); fill(bx, y - 6, 7, 1, 0xf6f2ea); fill(bx, y, 7, 1, paperD);
    disc(bx + 3, y - 4, 2, gold); dot(bx + 2, y - 4, ink); dot(bx + 4, y - 4, ink); dot(bx + 3, y - 3, ink); fill(bx + 1, y - 1, 5, 1, ink, 0.5);
    if ((now + x * 0.3) % 5 < 0.3) { fill(bx + 5, y - 6, 2, 6, paperD); fill(bx + 5, y - 6, 1, 6, paper); }
  },
  /** A single flower in a slim vase, turning slowly to face the light. */
  cultflower(x, y) {
    const vase = 0xdcd8cc, vaseD = 0xbfb6a4, stem = 0x6a7a4a, petal = 0xe0a890, center = 0xe0b24a, bx = x + 39;
    fill(bx, y - 3, 3, 3, vase); fill(bx, y - 3, 1, 3, 0xf0ece2); fill(bx + 2, y - 3, 1, 3, vaseD); fill(bx + 1, y - 7, 1, 4, stem);
    const sway = Math.round(Math.sin(now * 0.5) * 1);
    for (let a = 0; a < 5; a++) { const an = a / 5 * Math.PI * 2; dot(bx + 1 + sway + Math.round(Math.cos(an) * 1.5), y - 8 + Math.round(Math.sin(an) * 1.5), petal); }
    dot(bx + 1 + sway, y - 8, center);
  },
  /** A low-profile RGB mechanical keyboard, its keys lighting in a colour wave. */
  esportskeyboard(x, y) {
    const frame = 0x2a2a36, rgb = (k) => [0x2ad8e8, 0x8a3ae8, 0xe82ad8, 0xf03a8a, 0x9ae82a, 0x3a6aff][((k % 6) + 6) % 6], bx = x + 33, t = Math.floor(now * 3);
    fill(bx, y - 3, 12, 3, frame); fill(bx, y - 3, 12, 1, 0x3a3a46); fill(bx, y, 12, 1, 0x101016);
    for (let r = 0; r < 2; r++) for (let c = 0; c < 10; c++) dot(bx + 1 + c, y - 2 + r, rgb(t + c + r));
  },
  /** A gaming headset resting on a stand, its ear-cups glowing in shifting RGB. */
  esportsheadset(x, y) {
    const stand = 0x3a3a46, band = 0x2a2a36, cup = 0x14141c, rgb = (k) => [0x2ad8e8, 0x8a3ae8, 0xe82ad8, 0xf03a8a][((k % 4) + 4) % 4], bx = x + 38, t = Math.floor(now * 2);
    fill(bx + 1, y - 1, 2, 1, stand); fill(bx + 1, y - 7, 1, 6, stand);
    fill(bx - 1, y - 11, 6, 1, band); fill(bx - 1, y - 11, 1, 2, band); fill(bx + 4, y - 11, 1, 2, band);
    fill(bx - 2, y - 10, 2, 4, cup); fill(bx + 4, y - 10, 2, 4, cup); dot(bx - 1, y - 8, rgb(t)); dot(bx + 5, y - 8, rgb(t + 1)); fill(bx - 2, y - 6, 1, 2, 0x2a2a36);
  },
  /** A gaming mouse on a mat, its scroll and logo glowing. */
  esportsmouse(x, y) {
    const body = 0x1e1e28, bodyL = 0x2e2e3a, rgb = (k) => [0x2ad8e8, 0xe82ad8, 0x9ae82a][((k % 3) + 3) % 3], bx = x + 37, t = Math.floor(now * 2);
    fill(bx, y - 1, 9, 1, 0x14141c); fill(bx, y - 1, 9, 1, rgb(t), 0.3);
    fill(bx + 3, y - 5, 4, 4, body); fill(bx + 3, y - 5, 4, 1, bodyL); fill(bx + 3, y - 2, 4, 1, 0x14141c); fill(bx + 4, y - 5, 2, 1, 0x2a2a36);
    dot(bx + 4, y - 3, rgb(t)); dot(bx + 5, y - 3, rgb(t));
  },
  /** A tall energy-drink can with a lightning-bolt logo and a cold glint. */
  esportsenergy(x, y) {
    const can = 0x1e2a1e, canL = 0x2e3a2e, bolt = 0x9ae82a, boltL = 0xc8ff5a, silver = 0xb8bcc4, bx = x + 38;
    fill(bx, y - 7, 5, 7, can); fill(bx, y - 7, 1, 7, canL); fill(bx + 4, y - 7, 1, 7, 0x141c14); fill(bx, y - 7, 5, 1, silver); fill(bx, y, 5, 1, silver);
    fill(bx + 2, y - 6, 1, 2, bolt); fill(bx + 1, y - 4, 2, 1, bolt); fill(bx + 2, y - 3, 1, 2, boltL); dot(bx + 1, y - 6, WHITE, 0.5);
  },
  /** A game controller, its face buttons blinking and the sticks nudging. */
  esportscontroller(x, y) {
    const body = 0x1e1e28, bodyL = 0x2e2e3a, btnA = 0x9ae82a, btnB = 0xf03a8a, bx = x + 35;
    fill(bx + 1, y - 4, 7, 3, body); fill(bx + 1, y - 4, 7, 1, bodyL); fill(bx, y - 3, 2, 2, body); fill(bx + 8, y - 3, 2, 2, body);
    const nud = Math.round(Math.sin(now * 3)); dot(bx + 2, y - 2 + (nud > 0 ? 1 : 0), 0x3a3a46); dot(bx + 7, y - 2, 0x3a3a46);
    const bl = Math.floor(now * 3) % 2; dot(bx + 6, y - 3, bl ? btnA : 0x3a3a3a); dot(bx + 7, y - 3, bl ? 0x3a3a3a : btnB);
  },
  /** A collectible mascot figure on a base: a purple beast with cyan horns and glowing eyes. */
  esportsfigure(x, y) {
    const base = 0x1e1e28, body = 0x8a3ae8, bodyL = 0xb86af0, horn = 0x2ad8e8, eye = 0xf03a8a, bx = x + 38;
    fill(bx, y - 1, 5, 1, base); fill(bx, y - 2, 5, 1, 0x2e2e3a);
    fill(bx + 1, y - 6, 3, 4, body); fill(bx + 1, y - 6, 1, 4, bodyL); fill(bx + 1, y - 8, 3, 2, body);
    dot(bx + 1, y - 9, horn); dot(bx + 3, y - 9, horn); dot(bx + 1, y - 7, eye); dot(bx + 3, y - 7, eye);
    if (isNight) glowLate(() => glowAt(bx + 2, y - 7, 2, 2, eye, 0.12));
  },
  /** A golden championship trophy with two handles, a glint crossing the cup. */
  esportstrophy(x, y) {
    const gold = 0xe8c24a, goldL = 0xf2d878, goldD = 0xb8902a, base = 0x1e1e28, bx = x + 39;
    fill(bx - 1, y - 1, 5, 1, base); fill(bx - 1, y - 2, 5, 1, 0x2e2e3a); fill(bx, y - 4, 3, 2, gold);
    fill(bx - 1, y - 9, 5, 5, gold); fill(bx - 1, y - 9, 5, 1, goldL); fill(bx - 1, y - 5, 5, 1, goldD);
    fill(bx - 2, y - 8, 1, 3, gold); fill(bx + 4, y - 8, 1, 3, gold); dot(bx, y - 8, goldL);
    if (Math.floor(now * 2) % 5 === 0) dot(bx, y - 8, WHITE, 0.9);
  },
  /** A streaming condenser mic on a boom arm with a pop filter and a blinking live light. */
  esportsmic(x, y) {
    const arm = 0x2a2a36, mic = 0x3a3a46, micL = 0x5a5a66, grille = 0x1e1e28, live = 0xf03a8a, bx = x + 37;
    fill(bx, y - 1, 8, 1, 0x1e1e28); fill(bx + 1, y - 6, 1, 5, arm); fill(bx + 1, y - 6, 4, 1, arm);
    fill(bx + 4, y - 9, 3, 4, mic); fill(bx + 4, y - 9, 1, 4, micL); for (let k = 0; k < 3; k++) fill(bx + 4, y - 8 + k, 3, 1, k % 2 ? grille : mic);
    dot(bx + 7, y - 9, Math.floor(now * 1.5) % 2 ? live : 0x5a1a2a);
  },
  /** A knobbly wooden club leaning on the desk. */
  cavemanclub(x, y) {
    const wood = 0x6a4a2a, woodL = 0x8a6a42, woodD = 0x4a3418, bx = x + 39;
    for (let k = 0; k < 7; k++) { const w = 2 + (k > 4 ? k - 4 : 0); fill(bx - Math.floor((w - 2) / 2), y - 1 - k, w, 1, k > 4 ? wood : woodL); }
    dot(bx, y - 8, woodL); dot(bx + 1, y - 7, woodD); fill(bx, y - 1, 2, 1, woodD);
  },
  /** A big knobbed bone laid on the desk. */
  cavemanbone(x, y) {
    const bone = 0xe0d6b8, boneL = 0xf0e8cc, boneD = 0xc8bc98, bx = x + 35;
    fill(bx + 1, y - 3, 8, 2, bone); fill(bx + 1, y - 3, 8, 1, boneL); fill(bx + 1, y - 2, 8, 1, boneD);
    fill(bx, y - 4, 2, 1, bone); fill(bx, y - 2, 2, 1, bone); fill(bx + 8, y - 4, 2, 1, bone); fill(bx + 8, y - 2, 2, 1, bone); dot(bx, y - 4, boneL); dot(bx + 9, y - 2, boneD);
  },
  /** A speckled dino egg in a twig nest; it rocks and nearly cracks now and then. */
  cavemanegg(x, y) {
    const nest = 0x6a4a2a, nestD = 0x4a3418, egg = 0xcfe0c0, eggD = 0xa8c098, spot = 0x7a6a4a, bx = x + 39;
    fill(bx - 3, y - 1, 7, 1, nest); fill(bx - 3, y - 1, 7, 1, 0x8a6a42); dot(bx - 3, y - 1, nestD); dot(bx + 3, y - 1, nestD);
    const wob = Math.round(Math.sin(now * 2) * ((now + x * 0.3) % 8 < 2 ? 1 : 0));
    fill(bx - 1 + wob, y - 6, 3, 5, egg); fill(bx - 1 + wob, y - 6, 1, 5, 0xe0f0d0); fill(bx + 1 + wob, y - 6, 1, 5, eggD);
    dot(bx + wob, y - 4, spot); dot(bx + 1 + wob, y - 2, spot); dot(bx - 1 + wob, y - 3, spot);
  },
  /** Flint, striker and a tinder bundle; a shower of sparks flies now and then. */
  cavemanflint(x, y) {
    const flint = 0x5a5e66, flintL = 0x8a8e96, tinder = 0x8a6a42, spark = 0xffd36b, bx = x + 37;
    fill(bx, y - 1, 7, 1, tinder); fill(bx, y - 1, 7, 1, 0xa8784a);
    fill(bx + 1, y - 3, 3, 2, flint); fill(bx + 1, y - 3, 3, 1, flintL); fill(bx + 4, y - 3, 2, 1, 0x6a6e76);
    if (Math.floor(now * 4) % 7 === 0) { dot(bx + 4, y - 4, spark); dot(bx + 5, y - 5, spark, 0.6); dot(bx + 3, y - 4, spark, 0.5); }
  },
  /** A small sabre-tooth skull on a stand, its two long fangs showing. */
  cavemanskull(x, y) {
    const bone = 0xe0d6b8, boneD = 0xc8bc98, stand = 0x4a3418, bx = x + 38;
    fill(bx - 1, y - 1, 6, 1, stand); fill(bx, y - 6, 5, 5, bone); fill(bx, y - 6, 5, 1, 0xf0e8cc); fill(bx, y - 2, 5, 1, boneD);
    dot(bx + 1, y - 4, 0x2a2018); dot(bx + 3, y - 4, 0x2a2018); fill(bx + 1, y - 1, 1, 2, bone); fill(bx + 3, y - 1, 1, 2, bone);
  },
  /** A stone tablet carved with glyphs; a chip of stone flies as a new mark is cut. */
  cavemantablet(x, y) {
    const stone = 0x6e6056, stoneL = 0x8a7c6e, stoneD = 0x4a4038, glyph = 0xa0472a, bx = x + 37;
    fill(bx, y - 6, 8, 6, stone); fill(bx, y - 6, 8, 1, stoneL); fill(bx, y - 1, 8, 1, stoneD);
    for (let r = 0; r < 3; r++) for (let c = 0; c < 3; c++) if ((r + c) % 2 === 0) dot(bx + 1 + c * 2, y - 5 + r * 2, glyph);
    if ((now + x * 0.3) % 6 < 0.4) dot(bx + 6, y - 7, stoneL, 0.8);
  },
  /** A cluster of glowing blue cave crystals on a rock; they pulse and glow at night. */
  cavemancrystal(x, y) {
    const crys = 0x8ab8c8, crysL = 0xc8e8f0, crysD = 0x5a8a9a, rock = 0x4a4038, bx = x + 39;
    fill(bx - 2, y - 1, 5, 1, rock); fill(bx - 2, y - 1, 5, 1, 0x5e524a);
    fill(bx, y - 7, 2, 6, crys); dot(bx, y - 7, crysL); fill(bx - 1, y - 4, 1, 3, crysD); fill(bx + 2, y - 5, 1, 4, crys); dot(bx + 2, y - 5, crysL); fill(bx - 2, y - 3, 1, 2, crys);
    const pulse = (Math.sin(now * 1.5 + x) + 1) / 2; dot(bx, y - 6, WHITE, 0.3 + pulse * 0.5);
    if (isNight) glowLate(() => glowAt(bx, y - 4, 3, 4, crys, 0.12 + pulse * 0.12));
  },
  /** A charred roast on the bone, still steaming. */
  cavemanmeat(x, y) {
    const meat = 0x8a3a2a, meatL = 0xa84a38, meatD = 0x6a2818, bone = 0xe0d6b8, char = 0x3a1e12, bx = x + 36;
    fill(bx + 1, y - 5, 6, 4, meat); fill(bx + 1, y - 5, 6, 1, meatL); fill(bx + 1, y - 2, 6, 1, meatD);
    dot(bx + 2, y - 4, char); dot(bx + 5, y - 3, char); fill(bx + 6, y - 2, 3, 1, bone); dot(bx + 8, y - 2, 0xf0e8cc);
    const t = Math.floor(now * 3) % 3; dot(bx + 3, y - 7 - t, 0xcfc6ad, 0.4); dot(bx + 4, y - 8 - t, 0xcfc6ad, 0.3);
  },
  /** A brass three-branch candelabra, its flames flickering; glows at night. */
  lodgecandelabra(x, y) {
    const brass = 0xc8a44e, wax = 0xe8e0cc, flame = 0xffd36b, flameC = 0xfff0b0, bx = x + 39;
    fill(bx - 2, y - 1, 5, 1, brass); fill(bx, y - 4, 1, 3, brass); fill(bx - 3, y - 4, 7, 1, brass); fill(bx - 3, y - 4, 1, 2, brass); fill(bx + 3, y - 4, 1, 2, brass);
    for (const cx of [bx - 3, bx, bx + 3]) { fill(cx, y - 8, 1, 4, wax); const f = candlelight(cx * 0.3); fill(cx, y - 11, 1, 3, flame, f); dot(cx, y - 12, flameC, f); }
    if (isNight) glowLate(() => { for (const cx of [bx - 3, bx, bx + 3]) glowAt(cx, y - 10, 2, 4, flame, 0.12); });
  },
  /** A red wax seal pressed on a letter, a brass seal-stamp beside it. */
  lodgeseal(x, y) {
    const wax = 0x8a1e1e, waxL = 0xa82a2a, waxD = 0x6a1414, brass = 0xc8a44e, brassL = 0xe8c878, paper = 0xe0d2b0, bx = x + 37;
    fill(bx, y - 2, 6, 2, paper); fill(bx, y - 2, 6, 1, 0xeedcb0);
    disc(bx + 2, y - 2, 2, wax); ring(bx + 2, y - 2, 2, waxD); dot(bx + 2, y - 2, waxL); dot(bx + 1, y - 3, WHITE, 0.4);
    fill(bx + 6, y - 5, 2, 5, brass); fill(bx + 6, y - 5, 1, 5, brassL); fill(bx + 5, y - 1, 4, 1, brass);
  },
  /** A rolled oath scroll tied with a red ribbon; it unfurls a little to show its text now and then. */
  lodgescroll(x, y) {
    const parch = 0xe0d2b0, parchD = 0xc8b890, ribbon = 0x8a1e1e, ink = 0x3a2a1a, bx = x + 37;
    const un = (now + x * 0.3) % 7 < 2 ? 3 : 0;
    fill(bx, y - 4, 3, 4, parch); fill(bx, y - 4, 1, 4, 0xeedcb0);
    fill(bx + 8 - un, y - 4, 3, 4, parch); fill(bx + 8 - un, y - 4, 1, 4, parchD);
    fill(bx + 3, y - 3, Math.max(1, 5 - un), 2, parch); if (un) { fill(bx + 3, y - 3, 3, 1, ink, 0.5); fill(bx + 3, y - 2, 2, 1, ink, 0.5); }
    fill(bx + 5, y - 5, 1, 6, ribbon);
  },
  /** A tall brass ceremonial goblet with a dark draught, a glint on its rim. */
  lodgechalice(x, y) {
    const brass = 0xc8a44e, brassL = 0xe8c878, brassD = 0x8a6a2a, drink = 0x5a1e24, bx = x + 39;
    fill(bx - 1, y - 1, 4, 1, brass); fill(bx, y - 4, 2, 3, brass);
    fill(bx - 2, y - 9, 6, 5, brass); fill(bx - 2, y - 9, 6, 1, brassL); fill(bx - 1, y - 8, 4, 1, drink); fill(bx + 3, y - 9, 1, 5, brassD);
    dot(bx - 2, y - 8, brassL); if (Math.sin(now * 2) > 0.7) dot(bx + 1, y - 8, WHITE, 0.6);
  },
  /** A wooden gavel on a round sound-block; it taps down every few seconds with a little flash. */
  lodgegavel(x, y) {
    const wood = 0x5a3a24, woodL = 0x7a5436, woodD = 0x3a2418, brass = 0xc8a44e, bx = x + 37;
    fill(bx, y - 1, 8, 1, wood); fill(bx + 2, y - 2, 4, 1, woodL);
    const tap = (now + x * 0.3) % 4 < 0.3 ? 1 : 0;
    fill(bx + 1, y - 5 + tap, 6, 2, wood); fill(bx + 1, y - 5 + tap, 6, 1, woodL); fill(bx + 1, y - 3 + tap, 6, 1, woodD);
    fill(bx + 1, y - 5 + tap, 1, 2, brass); fill(bx + 6, y - 5 + tap, 1, 2, brass); fill(bx + 3, y - 4 + tap, 1, 4, wood);
    if (tap) { dot(bx, y - 1, WHITE, 0.5); dot(bx + 7, y - 1, WHITE, 0.5); }
  },
  /** An ornate brass key with a fancy bow; it catches the light now and then. */
  lodgekey(x, y) {
    const brass = 0xc8a44e, brassL = 0xe8c878, bx = x + 38;
    ring(bx, y - 7, 2, brass); dot(bx, y - 9, brassL);
    fill(bx, y - 5, 1, 5, brass); fill(bx, y - 5, 1, 5, brassL);
    fill(bx - 1, y - 1, 1, 1, brass); fill(bx + 1, y - 2, 1, 2, brass); fill(bx + 1, y, 1, 1, brass);
    if ((now + x * 0.2) % 5 < 0.4) dot(bx, y - 8, WHITE, 0.8);
  },
  /** An hourglass in a wood frame, the sand trickling from the top bulb to the bottom. */
  lodgehourglass(x, y) {
    const frame = 0x5a3a24, frameL = 0x7a5436, glass = 0xbfe0e8, sand = 0xd8b870, bx = x + 38;
    fill(bx, y - 9, 5, 1, frameL); fill(bx, y - 1, 5, 1, frame);
    for (let r = 0; r < 3; r++) fill(bx + r, y - 8 + r, Math.max(1, 5 - r * 2), 1, glass, 0.5);
    dot(bx + 2, y - 5, glass);
    for (let r = 0; r < 3; r++) fill(bx + 2 - r, y - 4 + r, 1 + r * 2, 1, glass, 0.5);
    const t = (now + x * 0.3) % 8, prog = Math.min(1, t / 7), topH = Math.round((1 - prog) * 2), botH = Math.round(prog * 2);
    for (let r = 0; r < topH; r++) fill(bx + 1 + r, y - 8 + r, Math.max(1, 3 - r * 2), 1, sand);
    for (let r = 0; r < botH; r++) fill(bx + 2 - (botH - 1 - r), y - 2 - r, 1 + (botH - 1 - r) * 2, 1, sand);
    if (t < 7) dot(bx + 2, y - 5, sand);
  },
  /** A pair of brass dividers open on parchment, their legs breathing slowly open and shut. */
  lodgecompass(x, y) {
    const brass = 0xc8a44e, brassL = 0xe8c878, paper = 0xe0d2b0, point = 0x6a6258, bx = x + 38;
    fill(bx - 3, y - 1, 8, 1, paper); fill(bx - 3, y - 1, 8, 1, 0xeedcb0);
    const spread = 1 + (Math.sin(now * 0.8) > 0 ? 1 : 0);
    fill(bx + 1, y - 7, 1, 1, brassL); line(bx + 1, y - 7, bx + 1 - spread, y - 2, brass); line(bx + 1, y - 7, bx + 1 + spread, y - 2, brass);
    dot(bx + 1 - spread, y - 1, point); dot(bx + 1 + spread, y - 1, point);
  },
  /** A couple of canned rations and a sealed MRE pouch stacked up. */
  prepperrations(x, y) {
    const can = 0xb4702a, canL = 0xd08a3a, canD = 0x8a5420, label = 0xe8e4d0, mre = 0x8a7a50, bx = x + 36;
    fill(bx, y - 3, 4, 3, can); fill(bx, y - 3, 4, 1, canL); fill(bx, y - 2, 4, 1, label); dot(bx + 1, y - 2, canD);
    fill(bx + 5, y - 3, 4, 3, can); fill(bx + 5, y - 3, 4, 1, canL); fill(bx + 5, y - 2, 4, 1, label);
    fill(bx + 1, y - 7, 6, 4, mre); fill(bx + 1, y - 7, 6, 1, 0xa89868); fill(bx + 2, y - 6, 4, 1, 0x5a4e34);
  },
  /** A gas mask on a little stand, round eye lenses and a filter canister. */
  preppergasmask(x, y) {
    const rubber = 0x3a3e38, rubberL = 0x4e5448, lens = 0x7a8e6a, lensL = 0xaecaa0, filter = 0x5a5e66, bx = x + 38;
    fill(bx, y - 1, 5, 1, 0x2a2e28);
    fill(bx, y - 7, 5, 6, rubber); fill(bx, y - 7, 5, 1, rubberL); fill(bx, y - 2, 5, 1, 0x24281f);
    dot(bx + 1, y - 5, lens); dot(bx + 3, y - 5, lens); dot(bx + 1, y - 5, lensL);
    fill(bx + 1, y - 2, 3, 1, filter); fill(bx + 5, y - 4, 2, 2, filter); dot(bx + 6, y - 4, 0x6e727c);
  },
  /** A handheld two-way radio with an antenna and a green signal light; it crackles with static now and then. */
  prepperradio(x, y) {
    const body = 0x3a4038, bodyL = 0x4e5448, screen = 0x0a1a0e, green = 0x3aa05a, antenna = 0x2a2e28, dial = 0xd8a82a, bx = x + 38;
    fill(bx, y - 7, 4, 7, body); fill(bx, y - 7, 1, 7, bodyL); fill(bx + 3, y - 7, 1, 7, 0x242a24);
    fill(bx + 1, y - 6, 2, 2, screen); dot(bx + 1, y - 6, green); dot(bx + 1, y - 3, dial); dot(bx + 2, y - 3, 0x2a2e28); fill(bx + 1, y - 10, 1, 3, antenna);
    if (Math.floor(now * 4) % 7 === 0) { dot(bx + 4, y - 8, green, 0.6); dot(bx - 1, y - 9, green, 0.4); }
  },
  /** A battery camping lantern glowing warm, with a rare low-battery flicker; glows at night. */
  prepperlantern(x, y) {
    const body = 0x3a4038, bodyL = 0x4e5448, glass = 0xfff0c0, glow = 0xffd36b, handle = 0x5a5e66, bx = x + 38;
    fill(bx + 1, y - 10, 3, 1, handle); fill(bx + 1, y - 10, 1, 2, handle); fill(bx + 3, y - 10, 1, 2, handle);
    fill(bx, y - 8, 5, 2, body); fill(bx, y - 2, 5, 2, body); fill(bx, y - 2, 5, 1, bodyL);
    const f = (Math.floor(now * 8) % 37 === 0) ? 0.4 : 1;
    fill(bx + 1, y - 6, 3, 4, glass, f); fill(bx + 1, y - 6, 1, 4, 0xfffbe0, f); dot(bx + 2, y - 4, glow, f);
    if (isNight) glowLate(() => glowAt(bx + 2, y - 4, 4, 4, glow, 0.16 * f));
  },
  /** A Geiger counter whose needle flicks and ticks as it reads. */
  preppergeiger(x, y) {
    const body = 0xd8a82a, bodyD = 0xa8821a, dial = 0xe8e4d0, needle = 0xc0392b, dark = 0x2a2620, bx = x + 37;
    fill(bx, y - 6, 8, 6, body); fill(bx, y - 6, 8, 1, 0xe8c04a); fill(bx, y - 1, 8, 1, bodyD);
    fill(bx + 1, y - 5, 6, 3, dial); fill(bx + 1, y - 5, 6, 1, 0xf2eee0);
    const na = -0.4 + (Math.floor(now * 6) % 5 === 0 ? 0.8 : 0); line(bx + 4, y - 2, bx + 4 + Math.round(Math.sin(na) * 2), y - 4, needle); dot(bx + 4, y - 2, dark);
    fill(bx + 2, y - 1, 4, 1, dark);
  },
  /** A white first-aid kit with a red cross and two latches. */
  prepperfirstaid(x, y) {
    const box = 0xe8e4d0, boxD = 0xc8c4b0, cross = 0xc0392b, latch = 0x8a8e96, bx = x + 37;
    fill(bx, y - 5, 8, 5, box); fill(bx, y - 5, 8, 1, 0xf2eee0); fill(bx, y - 1, 8, 1, boxD);
    fill(bx + 3, y - 4, 2, 3, cross); fill(bx + 2, y - 3, 4, 1, cross);
    fill(bx + 1, y - 3, 1, 2, latch); fill(bx + 6, y - 3, 1, 2, latch); fill(bx + 2, y - 6, 4, 1, boxD);
  },
  /** A military jerry can of fuel with an X-brace and a screw cap. */
  prepperjerrycan(x, y) {
    const can = 0x5a5a3a, canL = 0x6a6a46, canD = 0x3a3a24, cap = 0x2a2a1e, red = 0xc0392b, bx = x + 37;
    fill(bx, y - 8, 7, 8, can); fill(bx, y - 8, 7, 1, canL); fill(bx, y - 1, 7, 1, canD); fill(bx + 6, y - 8, 1, 8, canD);
    line(bx + 1, y - 7, bx + 5, y - 2, canD); line(bx + 5, y - 7, bx + 1, y - 2, canD);
    fill(bx + 1, y - 9, 2, 1, cap); dot(bx + 1, y - 10, cap); fill(bx + 3, y - 5, 2, 1, red);
  },
  /** An open multitool lying on the desk, its tools fanned out; a glint crosses the steel. */
  preppermultitool(x, y) {
    const steel = 0x8a8e96, steelL = 0xb8bcc4, steelD = 0x5a5e66, bx = x + 38;
    fill(bx, y - 1, 6, 1, 0x2a2e28); fill(bx + 1, y - 2, 4, 1, steel); fill(bx + 1, y - 2, 4, 1, steelL);
    line(bx + 2, y - 2, bx, y - 5, steel); line(bx + 3, y - 2, bx + 2, y - 6, steelL); line(bx + 4, y - 2, bx + 6, y - 5, steel); line(bx + 3, y - 2, bx + 5, y - 6, steelD);
    if ((now + x * 0.2) % 5 < 0.4) dot(bx + 3, y - 6, WHITE, 0.8);
  },
  /** Flag stand. A small striped flag with a blue star canton, furled on a gold pole in a little base. */
  paradiseflagstand(x, y) {
    const gold = 0xc8a050, goldLight = 0xeccf84, red = 0xc62828, white = 0xf2efe8, blue = 0x1f3a66;
    fill(x + 36, y, 7, 1, SHADOW);
    fill(x + 38, y - 1, 3, 1, gold); fill(x + 39, y - 11, 1, 10, gold); dot(x + 39, y - 12, goldLight);
    fill(x + 40, y - 11, 6, 5, white);
    for (const r of [0, 2, 4]) fill(x + 40, y - 11 + r, 6, 1, red);
    fill(x + 40, y - 11, 3, 3, blue); dot(x + 41, y - 10, white); dot(x + 42, y - 9, white);
  },
  /** Red secure phone. A red handset resting across a red cradle, a lit dial pad — the hotline. */
  paradisephone(x, y) {
    const red = 0xc62828, redLight = 0xe24a4a, redDark = 0x8a1c1c, dial = 0x1a1a1f;
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 36, y - 3, 8, 3, red); fill(x + 36, y - 3, 8, 1, redLight); fill(x + 36, y - 1, 8, 1, redDark);
    dot(x + 42, y - 2, dial); dot(x + 38, y - 2, 0xf2d35b);
    fill(x + 36, y - 6, 8, 2, red); fill(x + 36, y - 6, 8, 1, redLight);
    fill(x + 36, y - 7, 2, 2, redDark); fill(x + 42, y - 7, 2, 2, redDark);
  },
  /** Brass nameplate. An engraved brass plate on a little walnut stand. */
  paradisenameplate(x, y) {
    const wood = 0x5a3b25, gold = 0xc8a050, goldLight = 0xeccf84, goldDark = 0xa8863a;
    fill(x + 33, y, 12, 1, SHADOW);
    fill(x + 33, y - 2, 12, 2, wood); fill(x + 33, y - 2, 12, 1, 0x7a5232);
    fill(x + 34, y - 4, 10, 2, gold); fill(x + 34, y - 4, 10, 1, goldLight); fill(x + 34, y - 3, 10, 1, goldDark);
    for (let k = 0; k < 8; k++) dot(x + 35 + k, y - 3, 0x3a2e14, 0.7);
  },
  /** The big red button. A domed red button under a steel guard on a grey box; it twitches very rarely and glows at night. */
  paradisebigbutton(x, y) {
    const box = 0x3a3d47, boxLight = 0x5a5f6c, red = 0xd81a1a, redLight = 0xff5a4a, steel = 0x8a8f9c;
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 35, y - 4, 9, 4, box); fill(x + 35, y - 4, 9, 1, boxLight);
    fill(x + 36, y - 9, 1, 5, steel); fill(x + 43, y - 9, 1, 5, steel); fill(x + 36, y - 9, 8, 1, steel);
    const press = Math.sin(now * 0.7 + x) > 0.97 ? 1 : 0;
    disc(x + 39, y - 6 + press, 2.3, red); dot(x + 38, y - 7 + press, redLight);
    if (isNight) glowLate(() => glowAt(x + 39, y - 6, 3, 3, 0xff3a2a, 0.12));
  },
  /** Classified folders. A leaning stack of manila folders, a page peeking out and a red top-secret stripe. */
  paradisefolders(x, y) {
    const manila = 0xd8b86a, manilaLight = 0xe8ce8c, manilaDark = 0xa8863a, red = 0xc62828, paper = 0xeae6dc;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 10, 2, manila); fill(x + 35, y - 2, 10, 1, manilaLight);
    fill(x + 36, y - 4, 9, 2, manila); fill(x + 36, y - 4, 1, 2, manilaDark); fill(x + 37, y - 5, 6, 1, paper);
    fill(x + 34, y - 6, 10, 2, manila); fill(x + 34, y - 6, 10, 1, manilaLight);
    fill(x + 36, y - 6, 5, 1, red);
  },
  /** Desk intercom. A dark box with a speaker grille and line buttons, one of them blinking red. */
  paradiseintercom(x, y) {
    const case_ = 0x2a2d35, caseLight = 0x454a56, grille = 0x14161b, green = 0x5ae88a, red = 0xff3a2a;
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 35, y - 7, 9, 7, case_); fill(x + 35, y - 7, 9, 1, caseLight); fill(x + 35, y - 1, 9, 1, 0x14161b);
    for (let r = 0; r < 3; r++) fill(x + 36, y - 6 + r, 5, 1, grille);
    const on = Math.floor(now * 2) % 2;
    dot(x + 42, y - 6, on ? red : 0x5a1612); dot(x + 42, y - 4, green, 0.7); dot(x + 42, y - 2, 0x3a3d47);
  },
  /** Gold eagle. A little spread-winged eagle statuette in gold on a dark plinth. */
  paradiseeagle(x, y) {
    const gold = 0xc8a050, goldLight = 0xeccf84, goldDark = 0xa8863a, base = 0x3a2616;
    fill(x + 36, y, 7, 1, SHADOW);
    fill(x + 37, y - 2, 5, 2, base); fill(x + 37, y - 2, 5, 1, 0x5a3b25);
    fill(x + 39, y - 8, 2, 6, gold); dot(x + 39, y - 9, goldLight); dot(x + 41, y - 8, goldDark);
    fill(x + 36, y - 7, 3, 1, gold); fill(x + 37, y - 6, 2, 1, goldDark); fill(x + 41, y - 7, 3, 1, gold); fill(x + 41, y - 6, 2, 1, goldDark);
    dot(x + 38, y - 9, goldLight);
  },
  /** Banker's lamp. A brass desk lamp with a green glass shade, warm light pooling under it, lit at night. */
  paradiselamp(x, y) {
    const brass = 0xc8a050, brassLight = 0xeccf84, brassDark = 0xa8863a, glass = 0x2a7a46, glassLight = 0x4aa868;
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 38, y - 1, 4, 1, brassDark); fill(x + 39, y - 6, 2, 5, brass); dot(x + 39, y - 6, brassLight);
    fill(x + 35, y - 9, 10, 3, glass); fill(x + 35, y - 9, 10, 1, glassLight); fill(x + 35, y - 7, 10, 1, 0x1a4a2a);
    fill(x + 35, y - 6, 10, 1, brass);
    dot(x + 37, y - 8, WHITE, 0.4);
    glowAt(x + 40, y - 5, 5, 3, 0xffe08a, isNight ? 0.14 : 0.05);
    if (isNight) glowLate(() => glowAt(x + 40, y - 5, 6, 4, 0xffe08a, 0.16));
  },
  /** A tricorder. A handheld scanner with a blinking green sensor display and two amber lights. */
  startrektricorder(x, y) {
    const body = 0x3a4458, bodyLight = 0x5a6478, dark = 0x1a1f28, green = 0x6abf8a, amber = 0xd4a02a;
    fill(x + 36, y, 7, 1, SHADOW);
    fill(x + 37, y - 5, 5, 5, body); fill(x + 37, y - 5, 5, 1, bodyLight); fill(x + 37, y - 1, 5, 1, dark);
    fill(x + 38, y - 4, 3, 2, dark); dot(x + 39, y - 4, green, 0.6 + 0.4 * Math.sin(now * 4));
    dot(x + 38, y - 2, amber); dot(x + 40, y - 2, amber, 0.7);
  },
  /** A phaser. A type-2 phaser lying on the desk, grey with an orange emitter tip and a red setting light. */
  startrekphaser(x, y) {
    const body = 0x4a4e58, bodyLight = 0x6e727c, dark = 0x2a2d35, orange = 0xe08a3a;
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 36, y - 2, 7, 2, body); fill(x + 36, y - 2, 7, 1, bodyLight);
    fill(x + 42, y - 2, 2, 1, orange); dot(x + 43, y - 2, 0xffb060);
    fill(x + 37, y - 1, 2, 2, dark);
    dot(x + 39, y - 3, 0xd81a1a);
  },
  /** A PADD. A propped-up tablet with an amber LCARS rail and blue readout lines. */
  startrekpadd(x, y) {
    const frame = 0x2a2f3a, frameLight = 0x3a4458, amber = 0xd4a02a, blue = 0x4a8ec8, screen = 0x0c1430;
    fill(x + 37, y, 6, 1, SHADOW);
    fill(x + 38, y - 8, 5, 8, frame); fill(x + 38, y - 8, 5, 1, frameLight); fill(x + 38, y, 5, 1, 0x1a1f28);
    fill(x + 39, y - 7, 3, 6, screen);
    fill(x + 39, y - 7, 1, 6, amber, 0.8);
    for (let r = 0; r < 3; r++) fill(x + 40, y - 6 + r * 2, 2, 1, blue, 0.6);
  },
  /** Tea, Earl Grey. A cup of dark tea on a saucer, a wisp of steam rising. */
  startrekteacup(x, y) {
    const cup = 0xe8e4dc, cupDark = 0xc0bcb2, tea = 0x8a5a2a, saucer = 0xd8d4cc;
    fill(x + 36, y, 7, 1, SHADOW);
    fill(x + 37, y - 1, 6, 1, saucer);
    fill(x + 38, y - 5, 4, 4, cup); fill(x + 38, y - 5, 4, 1, 0xf4f0e8); fill(x + 38, y - 2, 4, 1, cupDark);
    fill(x + 38, y - 5, 4, 1, tea); dot(x + 39, y - 5, 0xa86a3a);
    fill(x + 42, y - 4, 1, 2, cupDark);
    const s = (now * 0.6) % 1; dot(x + 40, y - 7 - Math.round(s * 3), 0xffffff, 0.5 * (1 - s));
  },
  /** A starship model. A little saucer-and-nacelle ship on a stand, its warp nacelles glowing blue at night. */
  startrekmodel(x, y) {
    const hull = 0xc8ccd4, hullLight = 0xe8ecf4, hullDark = 0x8a8e96, nacelle = 0x4a8ec8, stand = 0x5a6478;
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 39, y - 3, 1, 3, stand); fill(x + 38, y - 1, 3, 1, stand);
    disc(x + 38, y - 6, 2.4, hull); dot(x + 37, y - 7, hullLight);
    fill(x + 39, y - 6, 4, 1, hull); fill(x + 42, y - 6, 1, 2, hullDark);
    fill(x + 41, y - 8, 3, 1, hullDark); dot(x + 44, y - 8, nacelle);
    fill(x + 41, y - 4, 3, 1, hullDark); dot(x + 44, y - 4, nacelle);
    if (isNight) glowLate(() => { dot(x + 44, y - 8, 0x8ac8f0); dot(x + 44, y - 4, 0x8ac8f0); });
  },
  /** A dilithium crystal. A faceted purple crystal in a holder, glowing, more at night. */
  startrekdilithium(x, y) {
    const holder = 0x3a4458, crystal = 0x9a6ad0, crystalLight = 0xc8a0f0, glow = 0xb080ff;
    fill(x + 37, y, 6, 1, SHADOW);
    fill(x + 38, y - 1, 5, 1, holder); fill(x + 39, y - 2, 3, 1, holder);
    fill(x + 39, y - 6, 3, 4, crystal); dot(x + 39, y - 6, crystalLight); dot(x + 41, y - 5, crystalLight);
    dot(x + 40, y - 7, crystal); dot(x + 40, y - 3, 0x6a3aa0);
    glowAt(x + 40, y - 4, 4, 4, glow, isNight ? 0.2 : 0.08);
    if (isNight) glowLate(() => glowAt(x + 40, y - 4, 5, 5, glow, 0.22));
  },
  /** A combadge. A gold arrowhead delta combadge on a little stand. */
  startrekcombadge(x, y) {
    const gold = 0xd4a82a, goldLight = 0xf2d87a, silver = 0xc8ccd4, stand = 0x3a4458;
    fill(x + 37, y, 6, 1, SHADOW);
    fill(x + 39, y - 2, 2, 2, stand);
    fill(x + 38, y - 5, 5, 1, silver);
    fill(x + 39, y - 6, 3, 1, gold); fill(x + 38, y - 4, 5, 1, gold); dot(x + 40, y - 7, goldLight);
    dot(x + 40, y - 5, goldLight);
  },
  /** A console gauge. A small readout box with a round dial and a needle swinging back and forth. */
  startrekgauge(x, y) {
    const case_ = 0x2a2f3a, caseLight = 0x3a4458, dial = 0x0c1430, green = 0x6abf8a, amber = 0xd4a02a, needle = 0xff5a4a;
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 36, y - 7, 8, 7, case_); fill(x + 36, y - 7, 8, 1, caseLight); fill(x + 36, y - 1, 8, 1, 0x1a1f28);
    disc(x + 40, y - 4, 2.6, dial); ring(x + 40, y - 4, 2.8, amber, 0.8);
    const a = -Math.PI * 0.75 + (Math.sin(now * 1.2 + x) + 1) / 2 * Math.PI * 1.5;
    line(x + 40, y - 4, x + 40 + Math.round(Math.cos(a) * 2), y - 4 + Math.round(Math.sin(a) * 2), needle);
    dot(x + 37, y - 6, green, 0.7); dot(x + 37, y - 4, amber, 0.6);
  },
  /** A whisky decanter. A squat crystal decanter of amber whisky with a stopper, a filled tumbler beside it. */
  gentlemendecanter(x, y) {
    const glass = 0xcfe0e8, glassDark = 0x9ab8c0, whisky = 0xc8822a, cork = 0x5a3b25;
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 36, y - 6, 5, 6, glass, 0.5); fill(x + 36, y - 6, 5, 1, glassDark); fill(x + 36, y - 3, 5, 3, whisky, 0.85);
    fill(x + 37, y - 8, 3, 2, glass, 0.6); dot(x + 38, y - 9, cork);
    dot(x + 37, y - 5, WHITE, 0.6);
    fill(x + 42, y - 3, 3, 3, glass, 0.5); fill(x + 42, y - 2, 3, 2, whisky, 0.8); dot(x + 42, y - 3, WHITE, 0.5);
  },
  /** A cigar box. An open cedar box with three banded cigars. */
  gentlemencigars(x, y) {
    const box = 0x5a3b25, boxLight = 0x7a5232, boxDark = 0x2e1a10, lid = 0x4a2e1a, cigar = 0x8a6038, cigarLight = 0xa8784a, band = 0xc8a050;
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 36, y - 3, 8, 3, box); fill(x + 36, y - 3, 8, 1, boxLight); fill(x + 36, y - 1, 8, 1, boxDark);
    fill(x + 36, y - 5, 8, 2, lid); fill(x + 36, y - 6, 8, 1, boxLight);
    for (let k = 0; k < 3; k++) { fill(x + 37 + k * 2, y - 3, 1, 2, cigar); dot(x + 37 + k * 2, y - 3, cigarLight); dot(x + 37 + k * 2, y - 2, band); }
  },
  /** A pipe and ashtray. A briar pipe resting in a dish, a faint wisp of smoke rising from the bowl. */
  gentlemenpipe(x, y) {
    const tray = 0x3a3d47, trayLight = 0x5a5f6c, wood = 0x5a3b25, woodLight = 0x7a5232, ash = 0xc8c4bc, ember = 0xe0702a;
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 36, y - 1, 8, 1, tray); fill(x + 36, y - 2, 8, 1, trayLight);
    fill(x + 37, y - 4, 2, 3, wood); dot(x + 37, y - 4, woodLight); dot(x + 38, y - 5, ember, 0.7);
    fill(x + 39, y - 2, 4, 1, wood); fill(x + 42, y - 3, 2, 1, 0x2a1d12);
    const smoke = (now * 0.5) % 1; dot(x + 38, y - 6 - Math.round(smoke * 3), ash, 0.4 * (1 - smoke));
  },
  /** A brick phone. A chunky 90s mobile with a stubby antenna, a little LCD and a keypad. */
  gentlemenbrick(x, y) {
    const body = 0x2a2d35, bodyLight = 0x454a56, dark = 0x14161b, screen = 0x6a8a4a, key = 0x5a5f6c, antenna = 0x3a3d47;
    fill(x + 37, y, 7, 1, SHADOW);
    fill(x + 38, y - 9, 5, 9, body); fill(x + 38, y - 9, 5, 1, bodyLight); fill(x + 38, y - 1, 5, 1, dark);
    fill(x + 41, y - 12, 1, 4, antenna); dot(x + 41, y - 12, bodyLight);
    fill(x + 39, y - 8, 3, 2, screen, 0.8);
    for (let r = 0; r < 3; r++) for (let c = 0; c < 3; c++) dot(x + 39 + c, y - 5 + r, key);
  },
  /** A fox mask. A small taxidermy fox face mounted on a little wooden plaque. */
  gentlemenfoxmask(x, y) {
    const wood = 0x5a3b25, fox = 0xc8622a, foxLight = 0xe0803a, white = 0xf0e8dc, nose = 0x2a1a12;
    fill(x + 36, y, 7, 1, SHADOW);
    fill(x + 37, y - 2, 5, 2, wood); fill(x + 37, y - 2, 5, 1, 0x7a5232);
    fill(x + 38, y - 6, 3, 4, fox); fill(x + 38, y - 6, 3, 1, foxLight);
    fill(x + 37, y - 7, 1, 2, fox); fill(x + 41, y - 7, 1, 2, fox);
    fill(x + 39, y - 3, 1, 1, white); dot(x + 39, y - 2, nose);
    dot(x + 38, y - 5, PALETTE.eye); dot(x + 40, y - 5, PALETTE.eye);
  },
  /** A leather ledger. A thick oxblood-leather accounts book with gold tooling on the spine. */
  gentlemenledger(x, y) {
    const leather = 0x5a2a2a, leatherLight = 0x7a3a3a, leatherDark = 0x3a1a1a, page = 0xe8e0cc, gold = 0xc8a050;
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 36, y - 5, 8, 5, leather); fill(x + 36, y - 5, 8, 1, leatherLight); fill(x + 36, y - 1, 8, 1, leatherDark);
    fill(x + 43, y - 5, 1, 5, page); fill(x + 36, y - 5, 1, 5, leatherDark);
    fill(x + 38, y - 4, 4, 1, gold, 0.8);
    dot(x + 37, y - 5, WHITE, 0.3);
  },
  /** A wax seal. A blob of red wax with an impressed crest and the brass seal stamp beside it. */
  gentlemenseal(x, y) {
    const brass = 0xc8a050, brassLight = 0xeccf84, brassDark = 0x8a6a2a, wood = 0x5a3b25, wax = 0x9a1e2a, waxLight = 0xc83a3a;
    fill(x + 35, y, 9, 1, SHADOW);
    disc(x + 38, y - 1, 2.4, wax); dot(x + 37, y - 2, waxLight); dot(x + 38, y - 1, brassDark, 0.5);
    fill(x + 42, y - 2, 2, 2, brass); fill(x + 42, y - 5, 2, 3, wood); dot(x + 42, y - 5, 0x7a5232); dot(x + 42, y - 2, brassLight);
  },
  /** A box of cartridges. An open green box of red shotgun shells with brass bases. */
  gentlemenshells(x, y) {
    const box = 0x3a5a3a, boxLight = 0x4a7a4a, boxDark = 0x2a3a2a, shell = 0xc83a2a, shellLight = 0xe05a4a, brass = 0xc8a050;
    fill(x + 35, y, 9, 1, SHADOW);
    fill(x + 36, y - 3, 8, 3, box); fill(x + 36, y - 3, 8, 1, boxLight); fill(x + 36, y - 1, 8, 1, boxDark);
    for (let k = 0; k < 4; k++) { fill(x + 37 + k * 2, y - 6, 1, 3, shell); dot(x + 37 + k * 2, y - 6, shellLight); dot(x + 37 + k * 2, y - 3, brass); }
  },
  /** A black Grok mug: a tall matte-black mug with a white slash and a pale rim; a curl of steam rises for a few seconds every 14 s. */
  grokmug(x, y) {
    const mug = 0x1a1b20, mugLight = 0x3a3d47, mugDark = 0x0e0f12;
    fill(x + 36, y - 7, 6, 7, mug); fill(x + 36, y - 7, 1, 7, mugLight); fill(x + 41, y - 7, 1, 7, mugDark);
    fill(x + 36, y - 7, 6, 1, 0x5a5e68); fill(x + 37, y - 7, 4, 1, 0x3a2a20); // the rim and the coffee
    fill(x + 42, y - 5, 2, 1, mug); fill(x + 43, y - 5, 1, 3, mug); fill(x + 42, y - 3, 2, 1, mug); // the handle
    for (let k = 0; k < 3; k++) dot(x + 38 + k, y - 2 - k, 0xf4f1ea, 0.9); // the slash, the mark cut down to the mug
    fill(x + 35, y, 9, 1, SHADOW);
    const t = (now + x * 0.41) % 14;
    if (t < 4) { const rise = Math.floor(t * 2); dot(x + 38 + (rise % 2), y - 9 - rise, WHITE, 0.55 - rise * 0.06); dot(x + 39 - (rise % 2), y - 11 - rise, WHITE, 0.35); }
  },
  /** A desk rocket: a white model rocket with a black band and fins on a little stand; every 35 s its engine puffs and it shudders on the stand. */
  grokrocket(x, y) {
    const body = 0xf4f1ea, bodyDark = 0xc8ccd4, band = 0x1a1b20, fin = 0x2a2c33;
    const t = (now + x * 0.23) % 35, shake = t < 2 ? (Math.floor(t * 12) % 2) : 0;
    fill(x + 36, y - 1, 7, 1, 0x3a3d47); fill(x + 39, y - 3, 1, 2, 0x6e727c); // the stand
    const rx = x + 38 + shake;
    fill(rx, y - 12, 3, 9, body); fill(rx + 2, y - 12, 1, 9, bodyDark); dot(rx + 1, y - 13, body); dot(rx + 1, y - 14, 0xa8adb6);
    fill(rx, y - 8, 3, 1, band); dot(rx + 1, y - 10, 0x8ab4ff); // its band and window
    fill(rx - 1, y - 5, 1, 3, fin); fill(rx + 3, y - 5, 1, 3, fin); fill(rx + 1, y - 3, 1, 1, fin);
    if (t < 2) { dot(rx + 1, y - 2, 0xffb648); dot(rx, y - 1, WHITE, 0.6); dot(rx + 2, y - 1, WHITE, 0.6); }
    fill(x + 35, y, 9, 1, SHADOW);
  },
  /** A glowing orb lamp: a frosted white sphere on a black puck, its light breathing slowly; it lights the desk at night. */
  grokorb(x, y) {
    const puck = 0x1a1b20, puckLight = 0x3a3d47, glass = 0xe8ecf4, glassShade = 0xc0c8d8;
    const breath = (Math.sin(now * 0.9 + x * 0.17) + 1) / 2;
    fill(x + 36, y - 2, 7, 2, puck); fill(x + 36, y - 2, 7, 1, puckLight);
    disc(x + 39, y - 6, 3.4, glassShade); disc(x + 39, y - 6, 2.6, glass); dot(x + 38, y - 8, WHITE);
    glowAt(x + 39, y - 6, 5, 4, 0xdfe8ff, 0.05 + breath * 0.06);
    if (isNight) glowLate(() => { disc(x + 39, y - 6, 2.6, 0xf4f8ff, 0.7 + breath * 0.3); glowAt(x + 39, y - 5, 9, 5, 0xdfe8ff, 0.12 + breath * 0.08); });
    fill(x + 35, y, 9, 1, SHADOW);
  },
  /** A folded towel: a neat stack of a bone-white towel with a thin blue stripe, always at hand; a corner of it lifts in a draught every 22 s. */
  groktowel(x, y) {
    const towel = 0xece8de, towelLight = 0xf8f6f0, towelDark = 0xc8c2b4, stripe = 0x8ab4ff;
    fill(x + 34, y - 4, 10, 4, towel); fill(x + 34, y - 4, 10, 1, towelLight); fill(x + 43, y - 4, 1, 4, towelDark);
    fill(x + 34, y - 2, 10, 1, towelDark, 0.6); fill(x + 34, y - 3, 10, 1, stripe, 0.7);
    fill(x + 35, y - 6, 8, 2, towel); fill(x + 35, y - 6, 8, 1, towelLight); fill(x + 35, y - 5, 8, 1, stripe, 0.7);
    if ((now + x * 0.31) % 22 < 1.5) dot(x + 43, y - 7, towelLight); else dot(x + 42, y - 6, towelDark);
    fill(x + 33, y, 12, 1, SHADOW);
  },
  /** A Grok bot plushie: a round black felt bot with a white visor and stubby arms, its antenna bobbing; its visor blinks every 9 s. */
  grokplush(x, y) {
    const felt = 0x2a2c33, feltLight = 0x3a3d47, feltDark = 0x1a1b20, visor = 0xf4f1ea;
    fill(x + 36, y - 7, 7, 7, felt); fill(x + 37, y - 8, 5, 1, felt); fill(x + 36, y - 7, 1, 6, feltLight); fill(x + 42, y - 7, 1, 7, feltDark);
    fill(x + 35, y - 4, 1, 2, felt); fill(x + 43, y - 4, 1, 2, felt); // its arms
    const blink = (now + x * 0.19) % 9 < 0.25;
    fill(x + 37, y - 6, 5, 2, blink ? feltDark : visor); if (!blink) { dot(x + 38, y - 5, 0x2a2c33); dot(x + 40, y - 5, 0x2a2c33); }
    const bob = Math.round(Math.sin(now * 2 + x)) > 0 ? 1 : 0;
    dot(x + 39, y - 9, feltLight); dot(x + 39 + bob, y - 10, 0x8ab4ff);
    fill(x + 35, y, 9, 1, SHADOW);
  },
  /** An in-tray of reports: a black wire tray with a stack of white pages; every 25 s a fresh page slides in on top. */
  grokbotqueue(x, y) {
    const wire = 0x3a3d47, wireDark = 0x1a1b20, page = 0xf4f1ea, pageEdge = 0xc8ccd4;
    fill(x + 34, y - 1, 11, 1, wireDark); fill(x + 34, y - 4, 1, 3, wire); fill(x + 44, y - 4, 1, 3, wire); fill(x + 34, y - 4, 11, 1, wire);
    for (let k = 0; k < 3; k++) { fill(x + 35, y - 2 - k, 9, 1, k % 2 ? pageEdge : page); }
    const t = (now + x * 0.29) % 25;
    if (t < 1.5) { const slide = Math.max(0, 6 - Math.floor(t * 6)); fill(x + 35 + slide, y - 5, 9 - Math.min(slide, 8), 1, page); }
    else { fill(x + 35, y - 5, 9, 1, page); fill(x + 36, y - 5, 4, 1, 0x8ab4ff, 0.6); }
    fill(x + 33, y, 12, 1, SHADOW);
  },
  /** A little beacon: a stub mast on a black base with an ion-blue light at its tip; every 6 s it sends out a ring, at night a bright one. */
  grokbotantenna(x, y) {
    const base = 0x1a1b20, baseLight = 0x3a3d47, mast = 0x8a8e98, ion = 0x8ab4ff;
    fill(x + 36, y - 2, 7, 2, base); fill(x + 36, y - 2, 7, 1, baseLight);
    fill(x + 39, y - 9, 1, 7, mast); fill(x + 38, y - 5, 3, 1, mast);
    const t = (now + x * 0.37) % 6;
    const tip = () => { dot(x + 39, y - 10, t < 0.3 ? WHITE : ion); if (t < 1.5) ellipseRing(x + 39, y - 10, 2 + t * 2, 1 + t, ion, 0.5 - t * 0.3); };
    tip();
    if (isNight) glowLate(tip);
    fill(x + 35, y, 9, 1, SHADOW);
  },
  /** A rock tumbler: a violet rubber barrel on its side on the rollers of a little green motor base, three stones it has polished beside it; every so often it runs for a few seconds, its ribs turning, the barrel rocking and its light on. */
  geodetumbler(x, y) {
    const base = 0x3e6a56, baseLight = 0x5e8a74, baseDark = 0x264232, drum = 0x3a2e48, drumLight = 0x5a4a6e, drumDark = 0x221a2e;
    const t = (now + x * 0.37) % 17, on = !isCalm() && t < 4, rock = on && Math.floor(now * 10) % 2 ? 1 : 0;
    fill(x + 33, y, 12, 1, SHADOW);
    // The motor base, its light, and the two rollers on it
    fill(x + 35, y - 3, 10, 3, base); fill(x + 35, y - 3, 10, 1, baseLight); fill(x + 44, y - 2, 1, 2, baseDark);
    dot(x + 36, y - 1, on ? 0x6aff8a : 0x2a4a3a);
    fill(x + 36, y - 4, 8, 1, 0x2a2a30); dot(x + 37, y - 4, 0x8a8a92); dot(x + 42, y - 4, 0x8a8a92);
    // The barrel on its side, its steel lid on the left; its ribs go round while it runs
    const top = y - 10 - rock;
    fill(x + 36, top, 8, 6, drum); fill(x + 36, top, 8, 1, drumLight); fill(x + 36, top + 5, 8, 1, drumDark); fill(x + 43, top + 1, 1, 4, drumDark);
    fill(x + 35, top + 1, 1, 4, 0x8a8e98); fill(x + 34, top + 2, 1, 2, 0x6e727c); dot(x + 35, top + 1, 0xc8ccd4);
    const turn = on ? Math.floor(now * 9) : 0;
    for (let k = 0; k < 2; k++) fill(x + 37, top + 1 + ((turn + k * 2) % 4), 6, 1, drumDark);
    dot(x + 38, top + 1, WHITE, 0.35);
    // The stones it has polished, on the desk beside it
    dot(x + 33, y - 1, 0xc84a8a); dot(x + 34, y - 1, 0x4ab0d0); dot(x + 33, y - 2, 0xe8b84a); dot(x + 33, y - 2, WHITE, 0.3);
  },
  /** A box of mineral specimens: a wooden box with its lid up and a label inside it, six little stones in its compartments (amethyst, citrine, malachite, rose quartz, azurite and pyrite); one of them glints now and then. */
  geodespecimens(x, y) {
    const wood = 0x8a5a32, woodLight = 0xa8703e, woodDark = 0x5a3b25, well = 0x3a2416;
    fill(x + 33, y, 13, 1, SHADOW);
    // The lid, up behind the box, its paper label
    fill(x + 34, y - 12, 11, 5, woodDark); fill(x + 34, y - 12, 11, 1, wood); fill(x + 35, y - 11, 9, 4, 0x6a4428);
    fill(x + 36, y - 10, 6, 2, PALETTE.paper); for (const dx of [37, 38, 40]) dot(x + dx, y - 9, 0x6a6a72);
    // The box, its compartments, a stone in each
    fill(x + 33, y - 7, 13, 7, wood); fill(x + 33, y - 7, 13, 1, woodLight); fill(x + 45, y - 6, 1, 6, woodDark); fill(x + 33, y - 1, 13, 1, woodDark);
    const stones = [[0x7a52b8, 0xc8a8f8], [0xe0a82a, 0xfae080], [0x1a8a58, 0x6ad49a], [0xd88aa4, 0xfad0dc], [0x2a5ad8, 0x7aa8ff], [0xa89040, 0xf0e090]];
    const spots = stones.map(([c, light], i) => {
      const sx = x + 34 + (i % 3) * 4, sy = y - 6 + Math.floor(i / 3) * 3;
      fill(sx, sy, 3, 2, well); fill(sx, sy + 1, 2, 1, c); dot(sx + 1, sy, c); dot(sx, sy + 1, light);
      return [sx + 1, sy];
    });
    const t = (now + x * 0.31) % 6.5;
    if (t < 0.5) { const [sx, sy] = spots[Math.floor((now + x * 0.31) / 6.5) % spots.length]; gdSparkle(sx, sy, t / 0.5); }
  },
  /** A miner's carbide lamp: a little brass canister with a polished reflector on its front, a small flame in the middle of it and a hook on top; the flame wavers, and by night it pools warm light on the desk. */
  geodelamp(x, y) {
    const brass = 0xc8a050, brassLight = 0xf0d080, brassDark = 0x7a5a24, f = candlelight(x * 0.11 + 3);
    fill(x + 34, y, 10, 1, SHADOW);
    // The canister, water over carbide, a band between and a foot
    fill(x + 38, y - 8, 5, 8, brass); fill(x + 38, y - 8, 1, 8, brassLight); fill(x + 42, y - 8, 1, 8, brassDark);
    fill(x + 38, y - 5, 5, 1, brassDark); fill(x + 37, y - 1, 7, 1, brassDark); fill(x + 39, y - 9, 3, 1, brassDark); dot(x + 40, y - 10, brass);
    // Its hook
    dot(x + 42, y - 10, 0x4a4e58); dot(x + 43, y - 11, 0x4a4e58); dot(x + 42, y - 12, 0x4a4e58);
    // The reflector on its front, and the flame in the middle of it
    fill(x + 36, y - 7, 2, 1, brass);
    disc(x + 35, y - 7, 2.7, 0xd8dce4); ring(x + 35, y - 7, 2.7, brassDark); dot(x + 34, y - 8, WHITE);
    dot(x + 35, y - 7, 0xfff4c0, f); dot(x + 35, y - 8, 0xffc050, 0.7 * f);
    glowAt(x + 35, y - 7, 4, 3, 0xffd080, (isNight ? 0.12 : 0.06) * f);
    if (isNight) glowLate(() => { glowAt(x + 35, y - 7, 6, 5, 0xffd080, 0.3 * f); glowAt(x + 37, y + 1, 10, 2, 0xffc860, 0.12 * f); dot(x + 35, y - 7, 0xfff4c0); });
  },
  /** A UV lamp over a rock: a dull grey stone under a little black lamp on a stand; every so often the lamp clicks on, its tube violet, and the stone lights up in bands of neon green, red and blue, then it goes dull again. By night that glow shows across the desk. */
  geodeuvlamp(x, y) {
    const t = (now + x * 0.41) % 19, on = !isCalm() && t < 5, k = on ? Math.min(1, t * 5, (5 - t) * 5) : 0;
    fill(x + 33, y, 12, 1, SHADOW);
    // The stand and the lamp's head; its tube is violet while it's on
    fill(x + 43, y - 11, 1, 11, 0x2a2a32); fill(x + 41, y - 1, 4, 1, 0x2a2a32); dot(x + 43, y - 11, 0x5a5a66); dot(x + 42, y - 1, on ? 0x6aff8a : 0x3a3a42);
    fill(x + 35, y - 12, 9, 2, 0x22222a); fill(x + 35, y - 12, 9, 1, 0x3a3a46);
    fill(x + 36, y - 10, 6, 1, on ? 0xd0a0ff : 0x4a3a6a);
    // The rock: dull grey, its minerals lit while the lamp's on
    fill(x + 34, y - 4, 7, 4, 0x8a8478); fill(x + 35, y - 5, 5, 1, 0x9a948a); fill(x + 34, y - 1, 7, 1, 0x6a645a); dot(x + 35, y - 4, 0xb0aa9e);
    dot(x + 37, y - 3, 0x9a9488); dot(x + 39, y - 2, 0x7a7468);
    const lit = [[1, -4, 0x4aff6a], [2, -4, 0x4aff6a], [3, -3, 0xff4a3a], [4, -3, 0xff4a3a], [5, -2, 0x4aff6a], [2, -2, 0x6a8aff], [5, -4, 0xff4a3a], [1, -1, 0x6a8aff], [3, -1, 0x4aff6a]];
    const shine = () => { for (const [dx, dy, c] of lit) dot(x + 34 + dx, y + dy, c, k); dot(x + 37, y - 10, WHITE, 0.6 * k); };
    if (on) { glowAt(x + 38, y - 5, 5, 4, 0x9a60ff, (isNight ? 0.16 : 0.1) * k); shine(); }
    if (on && isNight) glowLate(() => { glowAt(x + 38, y - 4, 8, 5, 0x9a60ff, 0.25 * k); shine(); });
  },
  /** A geode half on a little stand: grey rind, bands of agate and a hollow of violet crystal facing out; a point inside glints now and then, and by night the hollow glows. */
  geodehalf(x, y) {
    const cx = x + 39, cy = y - 7;
    fill(x + 34, y, 11, 1, SHADOW);
    fill(x + 35, y - 2, 9, 2, 0x5a3b25); fill(x + 35, y - 2, 9, 1, 0x8a5a32);
    disc(cx, cy, 5.2, 0x4e423c); disc(cx - 0.5, cy - 0.5, 4.6, 0x8a7e76);
    disc(cx, cy, 4.2, 0xe2dcea); disc(cx, cy, 3.6, 0x8a7aac); disc(cx, cy, 3.1, 0xc8bedc); disc(cx, cy, 2.6, 0x24142e);
    for (const [dx, dy, c] of [[-2, -1, 0x9a78d8], [-1, -2, 0xb090e8], [1, -2, 0x7a52b8], [2, -1, 0x9a78d8], [2, 1, 0x6a44a8], [1, 2, 0x7a52b8], [-1, 2, 0x9a78d8], [-2, 1, 0xb090e8], [0, -2, 0xd8c4f8]]) dot(cx + dx, cy + dy, c);
    dot(cx, cy, 0x5a3a96);
    const t = (now + x * 0.19) % 9;
    if (t < 0.5) gdSparkle(cx - 1, cy - 1, t / 0.5);
    if (isNight) glowLate(() => { glowAt(cx, cy, 4, 4, 0xa070f0, 0.25); dot(cx, cy - 2, 0xeee0ff); });
  },
  /** A gem gauge: a steel dial caliper lying on the desk, its jaws up round a cut sapphire on a scrap of red velvet; now and then the jaw slides open, the dial's needle swings, and it closes on the stone again. */
  geodecaliper(x, y) {
    const steel = 0x8a8e98, steelLight = 0xc8ccd4, steelDark = 0x4a4e58;
    const t = (now + x * 0.29) % 14, open = isCalm() ? 0 : t < 0.6 ? Math.round(t / 0.3) : t < 2.4 ? 2 : t < 3 ? Math.round((3 - t) / 0.3) : 0;
    fill(x + 33, y, 12, 1, SHADOW);
    // The velvet, and the stone on it
    fill(x + 34, y - 3, 7, 1, 0x7a1a30); fill(x + 34, y - 3, 7, 1, 0x9a2a40, 0.5);
    fill(x + 36, y - 6, 3, 3, 0x2a4ad8); dot(x + 36, y - 6, 0x8aa8ff); dot(x + 37, y - 6, 0x5a7af0); dot(x + 38, y - 4, 0x14287a); dot(x + 37, y - 5, 0x3a5ae8);
    // The beam along the desk, the fixed jaw on its left, the sliding jaw and its dial
    fill(x + 33, y - 2, 12, 1, steel); fill(x + 33, y - 1, 12, 1, steelDark);
    for (let k = 0; k < 5; k++) dot(x + 40 + k, y - 2, k % 2 ? steelLight : steelDark);
    fill(x + 34, y - 7, 2, 5, steel); dot(x + 34, y - 7, steelLight);
    const jx = x + 39 + open;
    fill(jx, y - 7, 2, 5, steel); dot(jx, y - 7, steelLight);
    disc(jx + 3, y - 5, 1.8, 0xf4f2ec); ring(jx + 3, y - 5, 2, steelDark);
    dot(jx + 3 + (open ? 1 : 0), y - 6 + (open ? 1 : 0), 0xd02030);
    if (t > 6 && t < 6.45) gdSparkle(x + 37, y - 6, (t - 6) / 0.45);
  },
  /** A prospector's pan: a shallow steel pan of wet gravel; now and then it's given a swirl, the gravel going round in it, and two gem chips, a red and a gold, come up out of it and glint. */
  geodepan(x, y) {
    const cx = x + 39, cy = y - 3, t = (now + x * 0.23) % 15, swirl = !isCalm() && t < 2.5, spin = swirl ? t * 4.5 : 0;
    fill(x + 33, y, 13, 1, SHADOW);
    // The pan, seen from above: its rim, its sloping side and the wet gravel in its middle
    const oval = (rx, ry, c, dy = 0) => { for (let r = -Math.floor(ry); r <= Math.floor(ry); r++) { const h = Math.round(rx * Math.sqrt(Math.max(0, 1 - (r / (ry + 0.5)) ** 2))); fill(cx - h, cy + dy + r, h * 2 + 1, 1, c); } };
    oval(6.5, 3, 0x6e727c); oval(5.5, 2.4, 0x4a4e58); oval(4, 1.6, 0x7a6a52); oval(2.6, 1, 0x5a7080, 0);
    fill(cx - 5, cy - 3, 11, 1, 0xa8adb6); dot(cx - 4, cy - 3, WHITE, 0.7); fill(cx - 5, cy + 3, 11, 1, 0x3a3e46);
    // The gravel, going round while it's swirled
    for (let k = 0; k < 8; k++) {
      const a = k * 0.8 + spin + (k % 3) * 0.4, rr = 0.45 + (k % 3) * 0.2;
      dot(cx + Math.round(Math.cos(a) * 4 * rr), cy + Math.round(Math.sin(a) * 1.6 * rr), k % 2 ? 0x5a4a38 : 0x9a8a6a);
    }
    // The gems, found when the swirl stops
    const g = swirl ? 0 : t < 3 ? (t - 2.5) / 0.5 : 1;
    dot(cx - 1, cy, 0xe02040, g); dot(cx + 2, cy + 1, 0xf8c840, g);
    if (t > 2.5 && t < 3) gdSparkle(cx - 1, cy, (t - 2.5) / 0.5);
    else if (t > 8 && t < 8.45) gdSparkle(cx + 2, cy + 1, (t - 8) / 0.45);
  },
  /** A rock hammer on a field notebook: a geologist's hammer with a blue grip, its head a square face and a pick, on a green notebook held shut by an elastic band; its steel glints now and then. */
  geodehammer(x, y) {
    const steel = 0x8a8e98, steelLight = 0xd0d4dc, steelDark = 0x4a4e58;
    fill(x + 33, y, 13, 1, SHADOW);
    // The notebook, its pages' edge and its elastic
    fill(x + 33, y - 4, 12, 4, 0x2e6a46); fill(x + 33, y - 4, 12, 1, 0x4a8a62); fill(x + 33, y - 1, 12, 1, PALETTE.paper); fill(x + 44, y - 3, 1, 3, 0x1e4a30);
    fill(x + 41, y - 4, 1, 4, 0x1a1a1e);
    // The hammer across it: its grip and shaft, then its head, the pick up and the square face down
    fill(x + 34, y - 6, 6, 2, 0x2a4a9a); fill(x + 34, y - 6, 6, 1, 0x4a6ac8); fill(x + 40, y - 6, 3, 2, steel); fill(x + 40, y - 6, 3, 1, steelLight);
    fill(x + 43, y - 9, 2, 7, steel); fill(x + 43, y - 9, 1, 7, steelLight); fill(x + 44, y - 4, 1, 2, steelDark);
    dot(x + 43, y - 10, steel); dot(x + 43, y - 11, steelDark); fill(x + 42, y - 3, 3, 1, steelDark);
    const t = (now + x * 0.17) % 11;
    if (t < 0.5) gdSparkle(x + 43, y - 8, t / 0.5);
  },
};
