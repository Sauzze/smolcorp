/** What's on top of the mic's stand, at `x`, `y` where its base meets the floor, `a` its opacity: each of MIC_KINDS. */
const MIC_HEADS = {
  /** The head: a grille in a rounded case. */
  mic(x, y, a) {
    fill(x - 1, y - 24, 3, 1, PALETTE.sill, a);
    fill(x - 2, y - 23, 5, 5, PALETTE.sill, a);
    for (const row of [-22, -20]) fill(x - 1, y + row, 3, 1, PALETTE.monitor, a);
    fill(x - 1, y - 18, 3, 1, PALETTE.monitorStand, a);
  },
  /** A megaphone on the stand, its horn to the room, a red rim on its mouth. */
  megaphone(x, y, a) {
    fill(x - 1, y - 22, 4, 5, MEGAPHONE, a);
    fill(x + 2, y - 22, 1, 5, PALETTE.dome, a);
    fill(x, y - 18, 2, 2, PALETTE.machine, a);
    for (let k = 1; k <= 6; k++) {
      const half = 2 + Math.floor(k / 2);
      fill(x - 1 - k, y - 20 - half, 1, half * 2 + 1, k === 6 ? PALETTE.dome : MEGAPHONE, a);
    }
    fill(x - 6, y - 23, 4, 1, WHITE, 0.5 * a);
  },
  /** Ribbon mic. A chrome ribbon mic held in a yoke, a gold band round it: a 1950s studio. */
  studio(x, y, a, held) {
    fill(x - 3, y - 19, 7, 1, 0x9aa0a6, a); fill(x - 3, y - 24, 1, 5, 0x9aa0a6, a); fill(x + 3, y - 24, 1, 5, 0x9aa0a6, a);
    fill(x - 1, y - 28, 3, 1, 0xc8cdd6, a); fill(x - 2, y - 27, 5, 8, 0xc8cdd6, a); fill(x - 1, y - 19, 3, 1, 0xc8cdd6, a);
    for (let r = -26; r <= -20; r += 2) fill(x - 1, y + r, 3, 1, 0x5a5f6c, a);
    fill(x - 2, y - 23, 5, 1, PALETTE.brass, a); dot(x - 2, y - 26, WHITE, 0.5 * a);
  },
  /** Podcast mic. A broadcast mic hung in a shock mount on a boom arm, a pop filter in front of it. */
  podcast(x, y, a, held) {
    line(x, y - 16, x - 4, y - 20, 0x3a3d47, a); line(x - 4, y - 20, x - 6, y - 22, 0x3a3d47, a);
    dot(x - 2, y - 19, 0x9aa0a6, a); dot(x - 4, y - 21, 0x9aa0a6, a);
    fill(x - 9, y - 27, 5, 1, 0x5a5f6c, a); fill(x - 9, y - 20, 5, 1, 0x5a5f6c, a);
    fill(x - 8, y - 28, 3, 8, 0x1d1f25, a); fill(x - 8, y - 28, 3, 3, 0x3a3d47, a); dot(x - 7, y - 27, 0x6a7080, a);
    line(x - 9, y - 21, x - 11, y - 21, 0x3a3d47, a);
    disc(x - 12, y - 24, 2.5, 0x1d1f25, 0.35 * a); ring(x - 12, y - 24, 2.5, 0x2a2a2e, a);
  },
  /** Karaoke mic. Gold, with a silver ball. Held, lights run round it in every colour. */
  karaoke(x, y, a, held) {
    fill(x - 1, y - 16, 3, 1, PALETTE.machine, a);
    fill(x, y - 21, 2, 6, PALETTE.brass, a); fill(x, y - 21, 1, 6, 0xf2d38a, a);
    disc(x, y - 23, 2.2, 0xc8cdd6, a); for (const [dx, dy] of [[-1, -23], [1, -24], [0, -22], [1, -22]]) dot(x + dx, y + dy, 0x8a8f9c, a);
    fill(x, y - 19, 2, 1, held ? rainbow((now * 0.8) % 1) : 0xd4140b, a);
    if (held) for (let k = 0; k < 6; k++) { const ang = now * 3 + (k * Math.PI) / 3; dot(x + Math.round(Math.cos(ang) * 4), y - 23 + Math.round(Math.sin(ang) * 4), rainbow(k / 6)); }
    else if ((now % 3) < 0.25) dot(x - 1, y - 25, WHITE);
  },
  /** Headset on a stand. A gaming headset in grey, a light rim on its band, a boom mic, its cup's light green, brighter while it's held. */
  headset(x, y, a, held) {
    disc(x, y - 20, 2.5, 0x6a7080, a);
    fill(x - 2, y - 24, 5, 1, 0x9aa0a6, a); dot(x - 3, y - 23, 0x737987, a); dot(x + 3, y - 23, 0x737987, a);
    fill(x - 4, y - 22, 2, 4, 0x5a5f6c, a); fill(x + 3, y - 22, 2, 4, 0x5a5f6c, a);
    fill(x - 4, y - 22, 1, 4, 0x8a93a3, a); fill(x + 3, y - 22, 1, 4, 0x8a93a3, a);
    line(x - 4, y - 18, x - 6, y - 16, 0x8a93a3, a); fill(x - 7, y - 17, 2, 2, 0x3a3d47, a);
    dot(x + 4, y - 21, 0x6fd36f, held ? a : 0.45 * a);
  },
  /** Walkie-talkie. Standing in its charger, its light slowly blinking green; red while it's held. */
  walkie(x, y, a, held) {
    fill(x - 2, y - 17, 6, 2, 0x3a3d47, a);
    fill(x - 1, y - 25, 4, 8, 0x2a2c30, a); fill(x - 1, y - 25, 4, 1, 0x3a3d47, a);
    fill(x, y - 23, 2, 1, 0x5a5f6c, a); fill(x, y - 21, 2, 1, 0x5a5f6c, a);
    fill(x + 2, y - 29, 1, 4, 0x1d1f25, a); dot(x + 2, y - 30, 0x3a3d47, a); dot(x - 1, y - 26, 0x5a5f6c, a);
    fill(x - 2, y - 22, 1, 2, 0xe8c04a, a);
    dot(x, y - 25, held ? 0xd4140b : 0x6fd36f, held ? 1 : (Math.floor(now * 0.8) % 2 ? 1 : 0.35));
  },
  /** Tin-can phone. A can on the stand, its string running off out of the room. */
  tincan(x, y, a, held) {
    const sway = Math.sin(now * 1.5) * 0.6;
    for (let k = 0; k <= 8; k++) dot(x + 3 + Math.round(k * 0.75), y - 19 + Math.round(k * 0.6 + Math.sin((k / 8) * Math.PI) * (1.5 + sway)), 0xeae6dc, 0.8 * a);
    fill(x - 2, y - 21, 5, 5, 0xa8adb6, a); fill(x - 2, y - 21, 5, 1, 0xc8cdd6, a); fill(x - 2, y - 17, 5, 1, 0x8a8f9c, a);
    fill(x - 2, y - 20, 5, 2, 0xc9443a, a); dot(x, y - 20, WHITE, a);
    fill(x - 3, y - 21, 1, 5, 0x5a5f6c, a);
  },
  /** Radio ring mic. A 1930s broadcast mic, hung on springs in a chrome ring over its station plate. */
  radio(x, y, a, held) {
    fill(x - 3, y - 17, 7, 1, PALETTE.brass, a);
    ring(x, y - 22, 4.5, 0xc8cdd6, a);
    for (const [dx, dy] of [[-2, -25], [-3, -26], [2, -25], [3, -26], [-2, -19], [-3, -18], [2, -19], [3, -18]]) dot(x + dx, y + dy, 0x9aa0a6, a);
    fill(x - 1, y - 24, 3, 4, 0x2a2c30, a); fill(x - 1, y - 24, 3, 1, 0x5a5f6c, a); dot(x, y - 22, 0x5a5f6c, a);
  },
  /** Reporter's mic. A handheld mic in the stand's clip, a square flag on it, white with the move's blue down one side. */
  reporter(x, y, a) {
    fill(x, y - 21, 1, 6, 0x2a2a2e, a);
    fill(x - 2, y - 25, 5, 4, 0xf5f5f5, a); fill(x - 2, y - 25, 1, 4, 0x3558ce, a); fill(x + 2, y - 25, 1, 4, 0xd8d8d8, a);
    disc(x, y - 27, 2, 0x3a3d47, a); dot(x - 1, y - 28, 0x6a7080, a);
  },
  /** Claude spark mic. A terracotta Claude spark round a cream grille on a charcoal collar; it glows softly at night, and beats a step longer in its rays while someone speaks. */
  clawdspark(x, y, a, held) {
    const spark = 0xd97757, light = 0xf0a080, dark = 0xb35a3c, heart = 0xfbe3c8, coal = 0x3a3631, cy = y - 24;
    const grow = held ? Math.floor(now * 4) % 2 : (now + x * 0.1) % 26 < 1.2 ? 1 : 0;
    // The collar the spark sits in
    fill(x - 2, y - 18, 5, 2, coal, a); fill(x - 2, y - 18, 5, 1, 0x57514a, a); fill(x, y - 20, 1, 2, coal, a);
    const head = () => {
      claudeSpark(x, cy, 5, spark, a, grow); disc(x, cy, 2.4, spark, a);
      line(x + 1, cy + 1, x + 3 + grow, cy + 3 + grow, dark, a); line(x - 1, cy - 1, x - 3, cy - 3, light, a);
      disc(x, cy, 1.5, heart, a); dot(x, cy, WHITE, 0.8 * a);
      if (held) dot(x, cy, 0xffe9b8, a);
    };
    if (isNight) glowLate(() => { glowAt(x, cy, 9, 8, 0xffa060, (held ? 0.34 : 0.2) * a); head(); });
    else head();
  },
  /** 1940s ribbon mic. A big chrome ribbon mic with a slatted grille, slung in a yoke with a brass screw on each side. */
  ribbon(x, y, a) {
    const chrome = 0xc8cdd6, light = 0xeef1f6, dark = 0x8a8f9c, slot = 0x3a3d47;
    fill(x - 5, y - 18, 11, 1, dark, a); fill(x - 5, y - 28, 1, 10, dark, a); fill(x + 5, y - 28, 1, 10, dark, a);
    fill(x - 3, y - 31, 7, 1, chrome, a); fill(x - 4, y - 30, 9, 11, chrome, a); fill(x - 3, y - 19, 7, 1, dark, a);
    fill(x - 4, y - 30, 1, 11, light, a); fill(x - 3, y - 31, 4, 1, light, a); fill(x + 4, y - 29, 1, 10, dark, a);
    for (const c of [-2, 0, 2]) fill(x + c, y - 29, 1, 9, slot, a);
    fill(x - 4, y - 25, 9, 1, PALETTE.brass, a); dot(x - 5, y - 25, 0xe8c878, a); dot(x + 5, y - 25, 0xe8c878, a);
    dot(x - 3, y - 29, WHITE, 0.7 * a);
  },
  /** Rock-and-roll mic. A chrome 1950s mic, a wide oval cage of horizontal bars with a bright rim, on a short neck; a glint crosses it now and then. */
  elvis(x, y, a) {
    const chrome = 0xd4d9e0, light = 0xf6f8fb, dark = 0x7e8492, bar = 0x4a4e58;
    const rows = [[2, 5], [1, 7], [0, 9], [0, 9], [0, 9], [0, 9], [0, 9], [1, 7], [2, 5]];
    rows.forEach(([off, w], i) => fill(x - 4 + off, y - 31 + i, w, 1, i % 2 ? bar : chrome, a));
    fill(x - 3, y - 31, 7, 1, light, a); fill(x - 3, y - 23, 7, 1, dark, a);
    fill(x - 4, y - 29, 1, 5, light, a); fill(x + 4, y - 29, 1, 5, dark, a);
    fill(x, y - 31, 1, 9, chrome, a); fill(x, y - 31, 1, 1, WHITE, a);
    fill(x - 1, y - 22, 3, 3, chrome, a); fill(x - 1, y - 22, 1, 3, light, a); fill(x + 1, y - 20, 1, 1, dark, a);
    fill(x - 2, y - 19, 5, 2, dark, a);
    const g = (now + x * 0.2) % 14; if (g < 0.6) dot(x - 3 + Math.floor(g * 10), y - 28 + Math.floor(g * 5), WHITE, a);
  },
  /** Carbon desk mic. A 1920s black bakelite mic: a round body with a brass ring and a woven grille, tilted in a C of a cradle. */
  carbon(x, y, a) {
    const black = 0x26282e, bake = 0x3e414a, brass = 0xc8a050, brassLight = 0xe8c878;
    fill(x - 5, y - 17, 11, 1, brass, a); fill(x - 6, y - 25, 1, 8, brass, a); fill(x + 6, y - 25, 1, 8, brass, a);
    disc(x, y - 24, 5, black, a); ring(x, y - 24, 5, bake, a); ring(x, y - 24, 4.2, brass, 0.9 * a);
    for (let r = -3; r <= 3; r++) for (let c = -3; c <= 3; c++) if ((r + c + 6) % 2 === 0 && r * r + c * c < 11) dot(x + c, y - 24 + r, 0x70747e, a);
    dot(x - 3, y - 27, brassLight, a); dot(x - 3, y - 28, WHITE, 0.5 * a);
    dot(x - 6, y - 28, brassLight, a); dot(x + 6, y - 28, brassLight, a);
  },
  /** Condenser mic. A champagne large-diaphragm capsule in a shock-mount of red elastics, a round pop filter hung on a gooseneck in front. */
  condenser(x, y, a, held) {
    const frame = 0x2a2c34, body = 0xd8c9a0, bodyLight = 0xf0e4c0, bodyDark = 0xa89a70, mesh = 0x6a644c;
    fill(x - 4, y - 17, 9, 1, frame, a); fill(x - 4, y - 30, 1, 13, frame, a); fill(x + 4, y - 30, 1, 13, frame, a); fill(x - 4, y - 31, 9, 1, frame, a);
    for (const yy of [-29, -25, -21]) { dot(x - 3, y + yy, 0xd8402a, a); dot(x + 3, y + yy, 0xd8402a, a); }
    fill(x - 2, y - 29, 5, 11, body, a); fill(x - 2, y - 29, 1, 11, bodyLight, a); fill(x + 2, y - 28, 1, 10, bodyDark, a);
    for (let r = 0; r < 4; r++) for (let c = 0; c < 3; c++) if ((r + c) % 2 === 0) dot(x - 1 + c, y - 28 + r * 2, mesh, a);
    fill(x - 2, y - 20, 5, 1, mesh, a);
    // The pop filter on its gooseneck, in front and to the left
    line(x - 3, y - 17, x - 6, y - 19, 0x5a5f6c, a);
    disc(x - 7, y - 24, 3.2, 0x1d1f25, 0.3 * a); ring(x - 7, y - 24, 3.2, 0x3a3d47, a); dot(x - 8, y - 26, WHITE, 0.4 * a);
    if (held) dot(x + 3, y - 31, 0xd4140b, a);
  },
  /** Banana phone. A banana for a handset, curved on the stand, its two dark holes for ear and mouth; now and then a brown tip nods. */
  banana(x, y, a) {
    const yellow = 0xf2d35b, light = 0xfff0a0, dark = 0xc9a42a, brown = 0x6a4a2a;
    const arc = [[-1, -31], [-3, -29], [-4, -27], [-4, -25], [-4, -23], [-3, -21], [-2, -19]];
    for (const [dx, dy] of arc) { fill(x + dx, y + dy, 5, 2, yellow, a); fill(x + dx, y + dy, 1, 2, light, a); fill(x + dx + 4, y + dy, 1, 2, dark, a); }
    const nod = (now + x * 0.2) % 18 < 0.5 ? 1 : 0;
    fill(x - 1, y - 33 + nod, 2, 2, brown, a); dot(x - 2, y - 18, brown, a); dot(x - 1, y - 18, brown, a);
    dot(x - 2, y - 29, 0x3a2a1a, a); dot(x - 2, y - 23, 0x3a2a1a, a); dot(x + 1, y - 26, dark, a); dot(x, y - 22, dark, a);
  },
  /** Hairbrush mic. A pink hairbrush, bristles to the room in a grid of white tips, its handle down the stand; a bristle sparkle twinkles now and then. */
  hairbrush(x, y, a) {
    const pad = 0xf28fb0, padLight = 0xffb8d0, padDark = 0xc8607f, tip = 0xfff4f8;
    const rows = [[1, 5], [0, 7], [-1, 9], [-1, 9], [-1, 9], [-1, 9], [0, 7], [1, 5]];
    rows.forEach(([off, w], i) => fill(x - 3 + off, y - 31 + i, w, 1, pad, a));
    fill(x - 3, y - 29, 1, 4, padLight, a); fill(x + 4, y - 28, 1, 4, padDark, a);
    for (let r = 0; r < 4; r++) for (let c = 0; c < 4; c++) dot(x - 2 + c * 2 - (r % 2), y - 30 + r * 2, tip, a);
    fill(x - 1, y - 23, 3, 6, padDark, a); fill(x - 1, y - 23, 1, 6, pad, a);
    const g = (now + x * 0.3) % 15; if (g < 0.5) dot(x + 3, y - 32, WHITE, a);
  },
  /** Rubber-duck mic. A yellow rubber duck bobbing on the stand with an orange beak and a tiny mic capsule at it; the beak opens while it's held. */
  duck(x, y, a, held) {
    const yellow = 0xf8d34a, light = 0xfff2a0, dark = 0xd8a82a, orange = 0xf08a2a, eye = 0x2a2420;
    const bob = held ? 0 : Math.floor((now * 0.9 + x * 0.1) % 4) === 0 ? 1 : 0;
    fill(x - 4, y - 21 + bob, 9, 4, yellow, a); fill(x - 5, y - 20 + bob, 1, 2, yellow, a); fill(x - 3, y - 17 + bob, 6, 1, dark, a);
    fill(x - 5, y - 21 + bob, 3, 1, light, a); dot(x + 3, y - 20 + bob, dark, a);
    fill(x - 1, y - 27 + bob, 5, 6, yellow, a); fill(x - 1, y - 27 + bob, 1, 6, light, a); fill(x - 1, y - 28 + bob, 4, 1, yellow, a);
    dot(x + 1, y - 25 + bob, eye, a);
    fill(x + 4, y - 24 + bob, 3, held ? 1 : 2, orange, a); if (held) fill(x + 4, y - 22 + bob, 3, 1, orange, a);
    disc(x + 9, y - 23 + bob, 1.5, 0x3a3d47, a); dot(x + 8, y - 24 + bob, 0x8a93a3, a); line(x + 7, y - 22 + bob, x + 8, y - 22 + bob, 0x3a3d47, a);
  },
  /** Ice-cream mic. A waffle cone upside-down on the stand's clip with a pink and a mint scoop and a cherry; every so often a drip runs down the cone. */
  icecone(x, y, a) {
    const waffle = 0xd8a05a, waffleDark = 0xa8742e, pink = 0xf4a0c0, pinkLight = 0xffc8dc, mint = 0x9fe0c0, red = 0xd8402a;
    for (let i = 0; i < 6; i++) fill(x - 3 + Math.floor(i / 2), y - 22 + i, 7 - Math.floor(i / 2) * 2, 1, waffle, a);
    for (const [dx, dy] of [[-1, -20], [1, -20], [0, -18], [0, -21]]) dot(x + dx, y + dy, waffleDark, a);
    fill(x - 4, y - 23, 9, 2, mint, a); disc(x, y - 26, 4.2, pink, a);
    fill(x - 3, y - 29, 2, 1, pinkLight, a); dot(x - 3, y - 27, pinkLight, a);
    fill(x - 3, y - 24, 7, 1, pink, a);
    disc(x, y - 31, 1.4, red, a); dot(x - 1, y - 32, WHITE, 0.7 * a); dot(x + 1, y - 33, 0x4f9a5a, a);
    const d = (now + x * 0.37) % 24; if (d < 5) dot(x + 4, y - 22 + Math.floor(d * 1.2), pink, a);
  },
  /** Cat-paw mic. A grey tabby paw held up on the stand, pink toe beans on its pad; the toes curl for a moment every so often and the claws show. */
  catpaw(x, y, a) {
    const fur = 0xa7adb8, furLight = 0xc8ced8, furDark = 0x7a808c, bean = 0xf0a0b0, beanDark = 0xd8809a;
    const flex = (now + x * 0.17) % 16 < 1.4;
    fill(x - 2, y - 19, 5, 3, furDark, a); fill(x - 2, y - 19, 1, 3, fur, a);
    fill(x - 5, y - 27, 11, 8, fur, a); fill(x - 4, y - 28, 9, 1, fur, a); fill(x - 4, y - 19, 9, 1, furDark, a);
    fill(x - 5, y - 27, 1, 8, furLight, a); fill(x + 5, y - 26, 1, 7, furDark, a);
    // The four toes, with their beans; the seams between them
    for (const dx of [-5, -2, 1, 4]) { fill(x + dx, y - 29, 2, 2, fur, a); dot(x + dx, y - 29, furLight, a); fill(x + dx, y - 26, 2, 2, bean, a); }
    for (const dx of [-3, 0, 3]) fill(x + dx, y - 28, 1, 3, furDark, a);
    // The big pad
    fill(x - 2, y - 23, 5, 3, bean, a); fill(x - 3, y - 22, 7, 1, bean, a); fill(x - 1, y - 20, 3, 1, beanDark, a); dot(x + 1, y - 22, beanDark, a);
    if (flex) for (const dx of [-5, -2, 1, 4]) dot(x + dx, y - 31, WHITE, 0.9 * a), dot(x + dx + 1, y - 31, WHITE, 0.9 * a);
  },
  /** Tentacle mic. A sea-green tentacle swaying up from the stand, suckers down its near side, its tip curled round a dark mic ball that glows faintly at night. */
  tentacle(x, y, a) {
    const green = 0x5aa07a, light = 0x8fd0aa, dark = 0x2e5e48, sucker = 0xd8f0d0;
    const sway = Math.round(Math.sin(now * 0.9 + x * 0.2) * 1.2);
    const path = [[0, -17], [0, -19], [-1, -21], [-2, -23], [-2, -25], [-1, -27], [1 + sway, -28], [3 + sway, -27], [4 + sway, -25]];
    path.forEach(([dx, dy], i) => {
      const w = i < 3 ? 3 : 2, ox = i < 3 ? -1 : 0;
      fill(x + dx + ox, y + dy, w, 2, green, a); fill(x + dx + ox, y + dy, 1, 2, light, a);
      if (i > 0 && i % 2 === 1) dot(x + dx + ox + w, y + dy, sucker, a);
    });
    fill(x - 1, y - 18, 1, 1, dark, a);
    const ball = () => { disc(x + 4 + sway, y - 24, 2.4, 0x2a3a34, a); dot(x + 3 + sway, y - 25, 0x7fffc0, 0.7 * a); dot(x + 4 + sway, y - 24, 0x4a6a5c, a); };
    ball();
    if (isNight) glowLate(() => { glowAt(x + 4 + sway, y - 24, 5, 5, 0x7fffc0, 0.12 * a); dot(x + 3 + sway, y - 25, 0xd8fff0, 0.8 * a); });
  },
  /** Little Cthulhu mic. A green idol head on the stand, bat-wing nubs, a beard of four tentacles; its yellow eyes glow at night, brighter and wider while it's held. */
  cthulhu(x, y, a, held) {
    const green = 0x3f8f98, light = 0x7fd0d4, dark = 0x1e4a56, eye = 0xf2e05a;
    fill(x - 4, y - 31, 9, 8, green, a); fill(x - 3, y - 32, 7, 1, green, a); fill(x - 4, y - 31, 1, 8, light, a); fill(x + 4, y - 30, 1, 7, dark, a);
    fill(x - 6, y - 29, 2, 3, dark, a); fill(x + 5, y - 29, 2, 3, dark, a);
    // The tentacle beard, one curling each way now and then
    const curl = (now + x * 0.2) % 11 < 0.6 ? 1 : 0;
    for (const [dx, len] of [[-3, 4], [-1, 5], [1, 5], [3, 4]]) fill(x + dx, y - 23, 1, len, green, a);
    dot(x - 3 + (curl ? -1 : 0), y - 19, dark, a); dot(x + 3 + (curl ? 1 : 0), y - 19, dark, a);
    for (const dx of [-1, 1]) dot(x + dx, y - 22, dark, a);
    fill(x - 4, y - 28, 9, 1, dark, a);
    const eyes = () => { fill(x - 3, y - 27, 2, held ? 2 : 1, eye, a); fill(x + 2, y - 27, 2, held ? 2 : 1, eye, a); };
    eyes();
    if (isNight) glowLate(() => { glowAt(x, y - 26, 6, 4, 0xe8e050, (held ? 0.25 : 0.12) * a); eyes(); });
  },
  /** Coin mic. A gold coin with a ring of mesh in its middle, on edge on the stand; every 20 s it spins for a few seconds. */
  coinmic(x, y, a) {
    const gold = 0xf2c94c, light = 0xfff0a0, dark = 0xb88a22, mesh = 0x8a6a22;
    const t = (now + x * 0.3) % 20, w = t < 3 ? [5, 3, 1, 3][Math.floor(t * 4) % 4] : 5;
    const r = w;
    fill(x - 1, y - 18, 3, 2, dark, a);
    for (let row = 0; row < 11; row++) {
      const dy = row - 5, half = Math.round(Math.sqrt(Math.max(0, 30 - dy * dy)) * (r / 5)); // 5.5 wide at the middle
      if (half > 0) fill(x - half, y - 30 + row, half * 2, 1, row < 2 || gold, a);
    }
    if (w === 5) {
      ring(x, y - 25, 5.2, dark, a);
      for (const [dx, dy] of [[-1, -1], [1, -1], [0, 0], [-1, 1], [1, 1], [0, -2], [0, 2]]) dot(x + dx, y - 25 + dy, mesh, a);
      dot(x - 3, y - 28, light, a); dot(x - 4, y - 26, light, a);
    } else fill(x, y - 29, 1, 9, dark, a);
    if (isNight) glowLate(() => { glowAt(x, y - 25, 6, 6, 0xffd860, 0.1 * a); });
  },
  /** Bull-horn mic. A little bull's head on the stand: brown, cream horns curving up, a gold ring in its pink muzzle that carries the mic's grille; it snorts a puff when held. */
  bullhorn(x, y, a, held) {
    const brown = 0x7a4a2e, light = 0x9a6440, dark = 0x4e2e1c, horn = 0xf0e6c8, hornDark = 0xc8b890, pink = 0xe8a0a0, gold = 0xe8c060;
    fill(x - 3, y - 28, 7, 7, brown, a); fill(x - 3, y - 28, 1, 7, light, a); fill(x + 3, y - 27, 1, 6, dark, a);
    fill(x - 2, y - 24, 5, 4, pink, a); fill(x - 2, y - 20, 5, 1, 0xc88080, a);
    dot(x - 1, y - 23, dark, a); dot(x + 1, y - 23, dark, a);
    dot(x - 2, y - 26, 0x1f1a1a, a); dot(x + 2, y - 26, 0x1f1a1a, a);
    // The horns: out and up
    for (const s of [-1, 1]) { fill(x + s * 4 - (s < 0 ? 1 : 0), y - 29, 2, 1, horn, a); fill(x + s * 5 - (s < 0 ? 1 : 0), y - 30, 2, 2, horn, a); fill(x + s * 5 - (s < 0 ? 1 : 0), y - 32, 1, 2, horn, a); dot(x + s * 6 - (s < 0 ? 1 : 0), y - 30, hornDark, a); }
    ring(x, y - 21.5, 1.6, gold, a);
    // The ears twitch
    const f = (now + x * 0.3) % 9 < 0.4 ? 1 : 0;
    fill(x - 5, y - 27 - f, 2, 1, dark, a); fill(x + 4, y - 27, 2, 1, dark, a);
    if (held && Math.floor(now * 3) % 3) { dot(x - 3, y - 20, WHITE, 0.6 * a); dot(x + 3, y - 19, WHITE, 0.5 * a); }
  },
  /** Navy-and-red mic. A navy ball-grille mic with a red band and a white pin-stripe, a club scarf round its neck whose ends flutter a step now and then. */
  psgmic(x, y, a) {
    const navy = PSG_NAVY, navyLight = PSG_NAVY_LIGHT, navyDark = PSG_NAVY_DARK, red = PSG_RED;
    disc(x, y - 26, 4.2, navy, a); dot(x - 2, y - 28, navyLight, a); dot(x - 1, y - 29, navyLight, a);
    for (const dy of [-28, -26, -24]) fill(x - 3, y + dy, 7, 1, navyDark, 0.6 * a);
    fill(x - 4, y - 25, 9, 1, red, a); fill(x - 4, y - 24, 9, 1, WHITE, 0.8 * a);
    fill(x - 2, y - 21, 5, 4, navy, a); fill(x - 2, y - 21, 1, 4, navyLight, a);
    fill(x - 2, y - 20, 5, 1, red, a); dot(x, y - 20, WHITE, a);
    // The scarf: stripes, its ends flutter
    const f = (now + x * 0.3) % 10 < 1 ? 1 : 0;
    fill(x - 3, y - 22, 7, 2, red, a); fill(x - 3, y - 21, 7, 1, WHITE, a);
    fill(x + 2 + f, y - 21, 2, 5, navy, a); fill(x + 2 + f, y - 19, 2, 1, red, a); fill(x + 2 + f, y - 17, 2, 1, WHITE, a);
  },
  /** Basketball mic. An orange basketball with black seams set in a red hoop with a white net; the ball rolls its seams while it's held, and bounces a pixel now and then. */
  hoopmic(x, y, a, held) {
    const orange = 0xe8782e, light = 0xf6a060, seam = 0x2a1a12;
    const bounce = !held && (now + x * 0.25) % 13 < 0.35 ? 2 : 0;
    const spin = held ? Math.floor(now * 4) % 4 - 2 : 0;
    const cy = y - 27 - bounce;
    disc(x, cy, 4.6, orange, a); dot(x - 2, cy - 2, light, a); dot(x - 1, cy - 3, light, a);
    fill(x + spin, cy - 4, 1, 9, seam, a); fill(x - 4, cy, 9, 1, seam, a);
    fill(x - 3, cy - 2, 1, 1, seam, a); dot(x - 3, cy + 2, seam, a); dot(x + 3, cy - 2, seam, a); dot(x + 3, cy + 2, seam, a);
    // The hoop and net
    fill(x - 5, y - 22, 11, 1, 0xd8402a, a);
    for (let i = 0; i < 4; i++) { const w = 9 - i * 2; fill(x - Math.floor(w / 2), y - 21 + i, w, 1, i % 2 ? 0xdcdcdc : WHITE, 0.55 * a); }
    for (const dx of [-3, 0, 3]) dot(x + dx, y - 20, 0xdcdcdc, a);
  },
  /** Hologram mic. A small emitter plate throwing up a cyan hologram of a mic that flickers and sweeps a scan line up it; it glows through the night. */
  holomic(x, y, a, held) {
    const plate = 0x3a3d47, cyan = 0x6ef0ff;
    fill(x - 3, y - 18, 7, 2, plate, a); fill(x - 3, y - 18, 7, 1, 0x6a7080, a); fill(x - 1, y - 17, 3, 1, cyan, a);
    const draw = () => {
      const flick = (Math.floor(now * 6 + x) % 17 === 0) ? 0.5 : 1, al = a * flick;
      for (let k = 0; k < 8; k++) dot(x - 3 + Math.floor(k / 2), y - 19 - k, cyan, 0.25 * al), dot(x + 3 - Math.floor(k / 2), y - 19 - k, cyan, 0.25 * al);
      fill(x - 2, y - 30, 5, 8, cyan, 0.45 * al); ring(x, y - 26, 2.6, cyan, 0.9 * al);
      for (const yy of [-28, -26, -24]) fill(x - 1, y + yy, 3, 1, cyan, 0.9 * al);
      fill(x, y - 22, 1, 4, cyan, 0.7 * al);
      const sweep = Math.floor((now * (held ? 14 : 6) + x) % 12);
      fill(x - 4, y - 31 + sweep, 9, 1, 0xd8fcff, 0.7 * al);
    };
    if (isNight) glowLate(() => { glowAt(x, y - 25, 7, 8, cyan, (held ? 0.22 : 0.14) * a); draw(); });
    else draw();
  },
  /** Retro-future ring mic. A chrome atomic-age ring on three spokes round a glowing teal core and a small orange sphere; the core pulses while it's held. */
  ringmic(x, y, a, held) {
    const chrome = 0xc8cdd6, light = 0xf0f4fa, dark = 0x7e8492, teal = 0x5af0d0, orange = 0xf08a2a;
    const cy = y - 25, p = held ? 0.5 + 0.5 * Math.sin(now * 6) : 0.4 + 0.15 * Math.sin(now * 1.5);
    ring(x, cy, 5.5, chrome, a); ring(x, cy, 4.6, dark, 0.8 * a);
    dot(x - 4, cy - 3, light, a); dot(x - 3, cy - 4, light, a);
    for (const [dx, dy] of [[0, -5], [-4, 3], [4, 3]]) { dot(x + dx, cy + dy, WHITE, a); }
    const core = () => { ring(x, cy, 3.2, teal, (0.4 + p * 0.5) * a); disc(x, cy, 1.5, orange, a); dot(x - 1, cy - 1, 0xffd8a0, a); };
    core();
    fill(x - 1, y - 19, 3, 2, dark, a);
    if (isNight) glowLate(() => { glowAt(x, cy, 7, 7, teal, (0.1 + p * 0.1) * a); core(); });
  },
  /** Space comm. A white-and-orange space handset with a stubby antenna, a round blue speaker and a screen of green bars that dance while it's held; the antenna tip blinks red. */
  spacecomm(x, y, a, held) {
    const white = 0xe8eaf0, shade = 0xaab0bc, orange = 0xe8782e, dark = 0x2a2c30;
    fill(x - 3, y - 17, 7, 1, shade, a);
    fill(x - 3, y - 29, 7, 12, white, a); fill(x - 3, y - 29, 1, 12, 0xffffff, a); fill(x + 3, y - 28, 1, 11, shade, a);
    fill(x - 3, y - 20, 7, 2, orange, a);
    fill(x - 2, y - 27, 5, 4, dark, a);
    const bars = [0, 1, 2].map((i) => held ? 1 + (Math.floor(now * 7 + i * 3) % 3) : 1);
    bars.forEach((h, i) => fill(x - 1 + i * 1.5 | 0, y - 24 - h + 1, 1, h, 0x6fe08a, a));
    dot(x, y - 18, 0x4c8bf5, a); dot(x - 1, y - 18, 0x4c8bf5, a); dot(x + 1, y - 18, 0x4c8bf5, a);
    fill(x + 2, y - 33, 1, 4, dark, a); dot(x + 2, y - 34, (now + x * 0.1) % 2 < 1 ? 0xd4140b : 0x6a2a2a, a);
    dot(x - 3, y - 26, 0xf2d35b, a);
  },
  /** Disco-ball mic. A mirror ball on the stand, its tiles shifting a step as it turns, a few sparkles twinkling round it; the sparkles shine through the night. */
  discomic(x, y, a, held) {
    const cy = y - 25, turn = Math.floor(now * (held ? 4 : 1.5)) % 2;
    disc(x, cy, 5, 0x9aa0ac, a);
    for (let r = -4; r <= 4; r++) for (let c = -4; c <= 4; c++) if (r * r + c * c <= 21 && (r + c + turn) % 2 === 0) dot(x + c, cy + r, (r + c) % 3 === 0 ? 0xf4f8ff : 0xd0d6e0, a);
    dot(x - 3, cy - 3, WHITE, a); fill(x, y - 31, 1, 2, 0x5a5f6c, a);
    const sp = Math.floor(now * 3 + x) % 6, spots = [[-7, -4], [6, -6], [-6, 3], [7, 2], [0, -9], [5, 5]];
    const twinkle = () => { for (let k = 0; k < 2; k++) { const [dx, dy] = spots[(sp + k * 3) % 6]; dot(x + dx, cy + dy, k ? 0xffd0f0 : 0xd0f4ff, a); dot(x + dx, cy + dy - 1, WHITE, 0.5 * a); dot(x + dx, cy + dy + 1, WHITE, 0.5 * a); } };
    if (isNight) glowLate(() => { glowAt(x, cy, 7, 7, 0xd8e8ff, 0.1 * a); twinkle(); });
    else if (held) twinkle();
  },
  /** Cactus mic. A small green cactus in a terracotta pot, spines in pale dots and a pink flower on top that opens while it's held. */
  cactusmic(x, y, a, held) {
    const green = 0x4f9a5a, light = 0x7ac084, dark = 0x3b7a46, pot = 0xa0603e, potLight = 0xc07a52, pink = 0xf28fb0;
    fill(x - 3, y - 21, 7, 5, pot, a); fill(x - 4, y - 22, 9, 2, potLight, a); fill(x + 3, y - 21, 1, 5, 0x7a4630, a);
    fill(x - 2, y - 30, 5, 9, green, a); fill(x - 2, y - 30, 1, 9, light, a); fill(x + 2, y - 29, 1, 8, dark, a); fill(x - 1, y - 31, 3, 1, green, a);
    fill(x - 5, y - 27, 3, 2, green, a); fill(x - 5, y - 29, 2, 3, green, a); fill(x - 5, y - 29, 1, 3, light, a);
    fill(x + 3, y - 25, 3, 2, green, a); fill(x + 4, y - 28, 2, 4, green, a); fill(x + 5, y - 28, 1, 4, dark, a);
    for (const [dx, dy] of [[0, -28], [1, -25], [-1, -23], [-4, -28], [5, -26]]) dot(x + dx, y + dy, 0xeef4d8, 0.9 * a);
    if (held) { dot(x - 1, y - 33, pink, a); dot(x + 1, y - 33, pink, a); dot(x, y - 34, pink, a); dot(x, y - 32, pink, a); dot(x, y - 33, 0xf2d35b, a); }
    else { fill(x - 1, y - 33, 3, 1, pink, a); dot(x, y - 34, pink, a); }
  },
  /** Lightbulb mic. A glass bulb with a zigzag filament on a screw base; the filament is dim until someone speaks, and then it lights, shining through the night. */
  bulbmic(x, y, a, held) {
    const glass = 0xdff0f6, base = 0x9aa0a6, baseDark = 0x6a7080;
    disc(x, y - 27, 4.6, glass, 0.45 * a); ring(x, y - 27, 4.6, glass, 0.75 * a);
    fill(x - 2, y - 23, 5, 2, glass, 0.45 * a);
    fill(x - 2, y - 21, 5, 1, base, a); fill(x - 2, y - 20, 5, 1, baseDark, a); fill(x - 2, y - 19, 5, 1, base, a); fill(x - 1, y - 18, 3, 1, 0x3a3d47, a);
    dot(x - 3, y - 29, WHITE, 0.8 * a);
    const lit = held, col = lit ? 0xffe08a : 0xb8a070, al = lit ? 1 : 0.7;
    const fil = () => { fill(x - 1, y - 24, 1, 3, 0x6a7080, a); fill(x + 1, y - 24, 1, 3, 0x6a7080, a);
      for (const [dx, dy] of [[-1, -27], [0, -28], [1, -27], [2, -28], [-2, -28]]) dot(x + dx, y + dy, col, al * a); dot(x - 1, y - 25, col, al * a); dot(x + 1, y - 25, col, al * a); };
    fil();
    if (lit) { glowAt(x, y - 27, 7, 7, 0xffe08a, 0.16 * a); if (isNight) glowLate(() => { glowAt(x, y - 27, 9, 9, 0xffe08a, 0.28 * a); fil(); }); }
    else if (isNight) glowLate(() => { glowAt(x, y - 27, 4, 4, 0xffc060, 0.08 * a); dot(x, y - 28, 0xffc060, 0.7 * a); });
  },
  /** Pumpkin mic. A small jack-o'-lantern with a green stem; its candle flickers in the carved eyes and grin, which shine through the night, and brighten while it's held. */
  pumpkinmic(x, y, a, held) {
    const orange = 0xe8782e, light = 0xf6a050, dark = 0xb24e1c, stem = 0x4f6a2a;
    fill(x - 5, y - 26, 11, 7, orange, a); fill(x - 4, y - 27, 9, 1, orange, a); fill(x - 4, y - 19, 9, 1, dark, a);
    fill(x - 2, y - 27, 1, 8, dark, a); fill(x + 2, y - 27, 1, 8, dark, a);
    fill(x - 5, y - 25, 1, 5, light, a); fill(x + 5, y - 24, 1, 5, dark, a);
    fill(x - 1, y - 30, 2, 3, stem, a); dot(x + 1, y - 31, stem, a); dot(x + 2, y - 30, 0x6a8a3a, a);
    const f = candlelight(x * 0.1) * (held ? 1.15 : 1), face = 0xffd060;
    const carve = () => {
      const c = f > 0.85 ? face : 0xe8a030;
      fill(x - 4, y - 25, 2, 2, c, a); fill(x + 3, y - 25, 2, 2, c, a); dot(x - 3, y - 26, c, a); dot(x + 4, y - 26, c, a);
      fill(x - 3, y - 22, 7, 1, c, a); dot(x - 2, y - 21, c, a); dot(x + 2, y - 21, c, a); dot(x, y - 23, c, a);
    };
    carve();
    if (isNight) glowLate(() => { glowAt(x, y - 23, 8, 6, 0xffa030, 0.16 * f * a); carve(); });
  },
  /** Rocket mic. A tiny red-and-white rocket on the stand with a round blue window; a flame flickers under it while it's held, and a spark of exhaust drifts off it now and then. */
  rocketmic(x, y, a, held) {
    const white = 0xe8eaf0, shade = 0xaab0bc, red = 0xd8402a, blue = 0x4c8bf5;
    fill(x - 2, y - 28, 5, 9, white, a); fill(x - 1, y - 30, 3, 2, white, a); fill(x, y - 32, 1, 2, red, a); fill(x - 1, y - 31, 3, 1, red, a); fill(x - 2, y - 29, 5, 1, red, a);
    fill(x - 2, y - 28, 1, 9, 0xffffff, a); fill(x + 2, y - 27, 1, 8, shade, a);
    disc(x, y - 24, 1.6, blue, a); dot(x - 1, y - 25, 0xa8d0ff, a);
    fill(x - 4, y - 22, 2, 4, red, a); fill(x + 3, y - 22, 2, 4, red, a); dot(x - 4, y - 23, red, a); dot(x + 4, y - 23, red, a);
    fill(x - 1, y - 19, 3, 1, shade, a);
    const flame = () => { const k = Math.floor(now * 12) % 3; fill(x - 1, y - 18, 3, 1 + k, 0xf2c040, a); fill(x, y - 18, 1, 2 + k, 0xff7a30, a); };
    if (held) { flame(); if (isNight) glowLate(() => { glowAt(x, y - 17, 5, 4, 0xffa040, 0.2 * a); flame(); }); }
    else if ((now + x * 0.2) % 17 < 0.7) dot(x, y - 18, 0xc8c8c8, 0.6 * a);
  },
  // mic-art: more heads above
};
