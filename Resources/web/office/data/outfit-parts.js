const OUTFIT_PARTS = {
  marvel1widow: {
    // Black Widow (brand nod: Marvel): a black catsuit with a silver zip and a belt with the red hourglass buckle; long red hair; her wrist bites spark blue every so often
    front(x, y, o, look) {
      fill(x, y, 1, 4, 0x8a8e98); dot(x, y, M1_SILVER);
      fill(x - 3, y + 4, 7, 1, 0x3a3c44); dot(x - 1, y + 4, M1_RED); dot(x + 1, y + 4, M1_RED); dot(x, y + 5, M1_RED); dot(x, y + 3, M1_RED);
      dot(x - 3, y + 1, 0x3a3c44); dot(x + 3, y + 1, 0x3a3c44);
      if (every(look, 19, 0.7, 4) !== null && Math.floor(now * 12) % 2) { dot(x - 4, y + 5, 0x7ad8ff); dot(x + 4, y + 5, 0x7ad8ff); dot(x - 5, y + 4, WHITE, 0.8); }
    },
    back(x, y, o) { fill(x - 3, y + 4, 7, 1, 0x3a3c44); fill(x - 2, y, 5, 3, 0xc8442a); fill(x - 1, y + 3, 3, 1, 0xa8342a); },
    head(x, top, look, away) {
      const red = 0xc8442a, redDark = 0x8e2a1c;
      fill(x - 3, top, 7, 2, red); dot(x - 2, top - 1, red); dot(x + 1, top - 1, red); fill(x - 3, top + 2, 1, 6, red); fill(x + 3, top + 2, 1, 6, redDark);
      dot(x - 4, top + 3, red); dot(x - 4, top + 6, redDark); dot(x + 4, top + 5, redDark);
      if (away) { fill(x - 2, top + 2, 5, 6, red); fill(x, top + 2, 1, 6, redDark, 0.5); }
    },
  },
  marvel1thor: {
    // Thor's armour (brand nod: Marvel): dark scale mail with four silver discs and a red cape at the shoulders; long blond hair, a short beard and a winged silver helmet; a spark runs over his shoulder now and then
    front(x, y, o, look) {
      for (const [dx, dy] of [[-2, 1], [1, 1], [-2, 3], [1, 3]]) { fill(x + dx, y + dy, 2, 2, M1_SILVER); dot(x + dx, y + dy, WHITE, 0.7); dot(x + dx + 1, y + dy + 1, M1_STEEL); }
      dot(x - 3, y, M1_RED); dot(x + 3, y, M1_RED); fill(x - 3, y + 1, 1, 5, M1_RED_DARK); fill(x + 3, y + 1, 1, 5, M1_RED_DARK);
      fill(x - 2, y + 5, 5, 1, 0x5a4a3a);
      if (every(look, 23, 0.8, 3) !== null && Math.floor(now * 10) % 2) { dot(x - 4, y - 1, 0xbfe8ff); dot(x - 5, y - 2, WHITE); }
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, M1_RED); fill(x - 3, y, 7, 1, M1_RED_LIGHT); fill(x - 1, y + 1, 1, 5, M1_RED_DARK, 0.7); fill(x + 2, y + 1, 1, 5, M1_RED_DARK, 0.7); },
    head(x, top, look, away) {
      const blond = 0xe8c060, blondDark = 0xb8903a;
      fill(x - 3, top, 7, 2, blond); fill(x - 3, top + 2, 1, 6, blond); fill(x + 3, top + 2, 1, 6, blondDark);
      fill(x - 2, top - 1, 5, 2, M1_SILVER); fill(x - 2, top - 1, 5, 1, WHITE, 0.6); dot(x - 3, top - 2, WHITE); dot(x - 4, top - 3, WHITE, 0.8); dot(x + 3, top - 2, WHITE); dot(x + 4, top - 3, WHITE, 0.8);
      if (away) { fill(x - 2, top + 2, 5, 5, blond); fill(x - 1, top + 2, 1, 5, blondDark, 0.5); return; }
      fill(x - 2, top + 6, 5, 1, blondDark); dot(x - 2, top + 5, blondDark); dot(x + 2, top + 5, blondDark);
    },
  },
  marvel1spidey: {
    // Spider-Man's suit (brand nod: Marvel): red with blue sides, dark web seams and a black spider on the chest; the full mask with big white eyes that narrow for a moment every so often
    front(x, y, o, look) {
      fill(x - 3, y + 2, 1, 4, M1_BLUE); fill(x + 3, y + 2, 1, 4, M1_BLUE);
      dot(x, y + 1, 0x1a1a1f); dot(x, y + 2, 0x1a1a1f); for (const [dx, dy] of [[-1, 0], [1, 0], [-1, 3], [1, 3]]) dot(x + dx, y + dy, 0x1a1a1f, 0.8);
      for (const [dx, dy] of [[-2, 1], [2, 1], [-2, 4], [2, 4], [0, 5]]) dot(x + dx, y + dy, M1_RED_DARK, 0.7);
    },
    back(x, y, o) {
      fill(x - 3, y + 2, 1, 4, M1_BLUE); fill(x + 3, y + 2, 1, 4, M1_BLUE);
      for (const [dx, dy] of [[0, 1], [0, 2], [0, 3], [-1, 1], [1, 1], [-2, 0], [2, 0], [-1, 3], [1, 3], [-2, 4], [2, 4]]) dot(x + dx, y + dy, M1_RED_DARK);
    },
    head(x, top, look, away) {
      if (away) { fill(x - 3, top, 7, 7, M1_RED); fill(x, top, 1, 7, M1_RED_DARK, 0.6); fill(x - 3, top + 3, 7, 1, M1_RED_DARK, 0.5); return; }
      m1Spidey(x - 3, top, 7);
      if (every(look, 11, 0.5, 5) !== null) { fill(x - 2, top + 2, 2, 1, M1_RED); fill(x + 1, top + 2, 2, 1, M1_RED); }
    },
  },
  marvel1cap: {
    // Captain America (brand nod: Marvel): blue with a white star on the chest and red and white stripes at the waist; a blue cowl with a white A and little white wings
    front(x, y, o, look) {
      for (const [dx, dy] of [[0, 0], [-1, 1], [0, 1], [1, 1], [-1, 2], [1, 2]]) dot(x + dx, y + dy, WHITE);
      fill(x - 3, y + 3, 7, 1, M1_RED); fill(x - 3, y + 4, 7, 1, M1_CREAM); fill(x - 3, y + 5, 7, 1, M1_RED);
      dot(x - 3, y, M1_BLUE_LIGHT);
    },
    back(x, y, o) { fill(x - 3, y + 3, 7, 1, M1_RED); fill(x - 3, y + 4, 7, 1, M1_CREAM); fill(x - 3, y + 5, 7, 1, M1_RED); dot(x, y + 1, WHITE, 0.7); },
    head(x, top, look, away) {
      if (away) { fill(x - 3, top, 7, 6, M1_BLUE); fill(x - 3, top, 7, 1, M1_BLUE_LIGHT); dot(x - 4, top + 2, WHITE); dot(x + 4, top + 2, WHITE); return; }
      fill(x - 3, top, 7, 3, M1_BLUE); fill(x - 3, top + 3, 1, 3, M1_BLUE); fill(x + 3, top + 3, 1, 3, M1_BLUE); fill(x - 2, top, 5, 1, M1_BLUE_LIGHT);
      dot(x, top + 1, WHITE); dot(x - 1, top + 2, WHITE); dot(x + 1, top + 2, WHITE);
      dot(x - 4, top + 2, WHITE); dot(x - 4, top + 1, WHITE, 0.7); dot(x + 4, top + 2, WHITE); dot(x + 4, top + 1, WHITE, 0.7);
    },
  },
  /* The styles', each in the agent's colour */
  astronaut: {
    front(x, y, o) {
      fill(x - 3, y, 7, 1, 0x9aa0a6); fill(x - 1, y + 2, 3, 2, 0x3a3f4a); dot(x - 1, y + 2, 0x6fd36f); dot(x + 1, y + 2, 0xd4140b);
      dot(x - 3, y + 1, o.accent); dot(x - 4, y + 2, o.accent); dot(x + 4, y + 2, o.accent);
    },
    back(x, y, o) { fill(x - 3, y, 7, 5, 0xc8cdd6); fill(x - 2, y + 1, 5, 3, 0xb0b5be); dot(x, y + 2, o.accent); },
    // The helmet's glass bubble
    head(x, top) { disc(x, top + 4, 5, 0xcfe3ef, 0.18); ring(x, top + 4, 5.2, 0xdcebf4, 0.85); dot(x - 3, top + 1, WHITE, 0.8); dot(x - 2, top, WHITE, 0.6); },
  },
  flightsuit: {
    front(x, y, o) { fill(x, y, 1, 5, 0xc8cdd6); fill(x - 2, y + 1, 2, 1, o.accent); dot(x + 2, y + 1, 0x3f6fc4); dot(x + 3, y + 1, 0xd4140b); fill(x - 3, y + 5, 7, 1, 0xa8582a); },
  },
  pirate: {
    front(x, y, o) { for (const r of [1, 3, 5]) fill(x - 3, y + r, 7, 1, o.accent); fill(x - 3, y, 2, 6, 0x1a1a1f); fill(x + 2, y, 2, 6, 0x1a1a1f); dot(x, y + 5, PALETTE.brass); },
    back(x, y, o) { for (const r of [1, 3, 5]) fill(x - 3, y + r, 7, 1, o.accent); fill(x - 3, y, 7, 4, 0x1a1a1f); },
    // A tricorn with a skull, and an eyepatch
    head(x, top, look, away) {
      fill(x - 4, top - 1, 9, 2, 0x1a1a1f); fill(x - 3, top - 2, 7, 1, 0x1a1a1f); dot(x - 4, top - 2, 0x1a1a1f); dot(x + 4, top - 2, 0x1a1a1f);
      fill(x - 4, top + 1, 9, 1, PALETTE.brass, 0.8);
      if (away) return;
      dot(x, top - 1, 0xeae6dc);
      dot(x - 2, top + 4, 0x1a1a1f); dot(x - 3, top + 3, 0x1a1a1f); dot(x - 1, top + 3, 0x1a1a1f); dot(x + 1, top + 2, 0x1a1a1f, 0.6);
    },
  },
  captain: {
    front(x, y) { fill(x - 1, y, 3, 2, 0xf4f1ea); for (const r of [2, 4]) { dot(x - 2, y + r, PALETTE.brass); dot(x + 2, y + r, PALETTE.brass); } },
    // A big hat with a white feather
    head(x, top) {
      fill(x - 5, top - 1, 11, 2, 0x1a1a1f); fill(x - 3, top - 3, 7, 2, 0x1a1a1f); fill(x - 5, top + 1, 11, 1, PALETTE.brass, 0.8);
      fill(x + 1, top - 5, 3, 1, WHITE); fill(x + 3, top - 4, 2, 1, WHITE); dot(x + 4, top - 3, WHITE);
    },
  },
  parka: {
    front(x, y, o) { for (const r of [2, 4]) fill(x - 3, y + r, 7, 1, shaded(o.shirt, 0.8)); fill(x, y, 1, 6, shaded(o.shirt, 0.7)); },
    back(x, y, o) { for (const r of [2, 4]) fill(x - 3, y + r, 7, 1, shaded(o.shirt, 0.8)); },
    // Its hood up, a fur trim round the face
    head(x, top, look, away, o) {
      fill(x - 4, top - 1, 9, 2, o.shirt); fill(x - 4, top + 1, 1, 6, o.shirt); fill(x + 4, top + 1, 1, 6, o.shirt);
      if (away) return fill(x - 3, top + 1, 7, 6, o.shirt);
      const fur = 0xeae6dc;
      fill(x - 3, top + 1, 7, 1, fur); fill(x - 3, top + 2, 1, 5, fur); fill(x + 3, top + 2, 1, 5, fur); dot(x - 2, top + 6, fur); dot(x + 2, top + 6, fur);
    },
  },
  explorer: {
    front(x, y) { fill(x - 3, y + 3, 7, 1, 0xdcebf4); fill(x, y, 1, 3, 0x9aa0a6); dot(x - 2, y + 1, 0x1a1a1f); },
    back(x, y) { fill(x - 3, y + 3, 7, 1, 0xdcebf4); },
    // A beanie in the agent's colour, snow goggles pushed up
    head(x, top, look, away, o) {
      fill(x - 3, top - 1, 7, 2, o.accent); dot(x, top - 2, WHITE);
      if (!away) { fill(x - 3, top + 1, 7, 1, 0x2a2d35); dot(x - 2, top + 1, 0x8fc6f0); dot(x + 2, top + 1, 0x8fc6f0); }
    },
  },
  robe: {
    front(x, y) { fill(x - 3, y, 7, 2, 0xeae6dc); dot(x - 2, y + 1, 0x1a1a1f); dot(x + 1, y, 0x1a1a1f); dot(x + 3, y + 1, 0x1a1a1f); fill(x, y + 2, 1, 4, PALETTE.brass); },
    back(x, y) { fill(x - 3, y, 7, 2, 0xeae6dc); dot(x - 1, y + 1, 0x1a1a1f); dot(x + 2, y, 0x1a1a1f); },
    head(x, top) { fill(x - 2, top - 1, 5, 1, PALETTE.brass); for (const dx of [-2, 0, 2]) dot(x + dx, top - 2, PALETTE.brass); dot(x, top - 1, 0xd4140b); },
  },
  knight: {
    front(x, y, o) {
      fill(x - 2, y, 5, 6, o.accent); fill(x, y + 1, 1, 4, WHITE); fill(x - 1, y + 2, 3, 1, WHITE);
      for (let r = 0; r < 6; r += 2) { dot(x - 3, y + r, 0xb8bec8); dot(x + 3, y + r, 0xb8bec8); }
    },
    back(x, y, o) { fill(x - 2, y, 5, 6, o.accent); },
  },
  wizard: {
    front(x, y, o) { for (const [dx, dy] of [[-2, 1], [2, 3], [-1, 5]]) dot(x + dx, y + dy, PALETTE.brass); fill(x - 3, y, 7, 1, shaded(o.shirt, 1.4)); },
    back(x, y) { for (const [dx, dy] of [[-1, 2], [2, 4]]) dot(x + dx, y + dy, PALETTE.brass); },
    // A tall pointed hat, a star on it
    head(x, top, look, away, o) {
      fill(x - 4, top, 9, 1, o.shirt);
      for (let r = 1; r <= 6; r++) { const half = Math.max(0, 3 - Math.floor(r / 2)); fill(x - half + (r > 4 ? 1 : 0), top - r, half * 2 + 1, 1, o.shirt); }
      dot(x + 2, top - 7, o.shirt); dot(x, top - 3, PALETTE.brass);
    },
  },
  gamer: {
    front(x, y, o) { dot(x - 3, y, o.accent); dot(x + 3, y, o.accent); dot(x - 3, y + 1, o.accent); dot(x + 3, y + 1, o.accent); fill(x, y + 2, 1, 3, WHITE); dot(x - 1, y + 2, WHITE); },
    back(x, y, o) { dot(x - 3, y, o.accent); dot(x + 3, y, o.accent); fill(x - 1, y + 1, 3, 1, WHITE); fill(x, y + 2, 1, 3, WHITE); },
    // A headset, its boom mic lit
    head(x, top, look, away, o) {
      fill(x - 3, top - 1, 7, 1, 0x3a3d47); fill(x - 4, top + 2, 1, 3, 0x3a3d47); fill(x + 4, top + 2, 1, 3, 0x3a3d47);
      dot(x - 4, top + 3, o.accent); dot(x + 4, top + 3, o.accent);
      if (!away) { dot(x - 3, top + 5, 0x3a3d47); dot(x - 2, top + 6, 0x3a3d47); dot(x - 1, top + 6, 0x6fd36f, 0.6 + 0.4 * Math.sin(now * 3)); }
    },
  },
  founder: {
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0xa8c8e8); dot(x - 1, y, 0xc8dcf0); dot(x + 1, y, 0xc8dcf0);
      dot(x - 1, y + 1, 0x1d1f25); dot(x - 1, y + 2, 0x1d1f25); fill(x - 1, y + 3, 2, 2, o.accent);
    },
    back(x, y) { fill(x - 3, y, 7, 1, 0xa8c8e8); },
  },
  gymkit: {
    front(x, y, o) { fill(x - 1, y, 3, 1, shaded(o.shirt, 0.7)); fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.8)); },
    // A sweatband, and wristbands
    head(x, top, look, away, o) { fill(x - 3, top + 2, 7, 1, WHITE); dot(x - 4, top + 11, o.accent); dot(x + 4, top + 11, o.accent); },
  },
  safari: {
    front(x, y) {
      fill(x - 3, y + 1, 2, 2, 0xa8905a); fill(x + 2, y + 1, 2, 2, 0xa8905a); dot(x, y + 1, 0xa8905a); dot(x, y + 3, 0xa8905a);
      fill(x - 3, y + 5, 7, 1, 0x5a3b25); dot(x, y + 5, PALETTE.brass);
    },
    // A pith helmet, its band in the agent's colour
    head(x, top, look, away, o) { fill(x - 3, top - 2, 7, 2, 0xe0d0a0); fill(x - 2, top - 3, 5, 1, 0xe0d0a0); fill(x - 4, top, 9, 1, 0xc8b888); fill(x - 3, top - 1, 7, 1, o.accent); },
  },
  flannel: {
    front(x, y, o) {
      for (let r = 0; r < 6; r++) for (let c = -3; c <= 3; c++) if (r % 3 === 1 || (c + 3) % 3 === 1) dot(x + c, y + r, shaded(o.shirt, 0.6));
      dot(x - 1, y, WHITE); dot(x + 1, y, WHITE);
    },
    back(x, y, o) { for (let r = 0; r < 6; r++) for (let c = -3; c <= 3; c++) if (r % 3 === 1 || (c + 3) % 3 === 1) dot(x + c, y + r, shaded(o.shirt, 0.6)); },
    head(x, top) { fill(x - 3, top - 1, 7, 2, 0x3f6a4a); fill(x - 3, top + 1, 7, 1, 0x2f5a3a); dot(x, top - 2, WHITE); },
  },

  /* The models' looks, shaded for the model's size */
  claude: {
    // A spark on the chest, ribbing at the hem
    front(x, y, o) {
      const c = 0xf4e8dc;
      dot(x, y + 1, c); dot(x, y + 3, c); fill(x - 1, y + 2, 3, 1, c);
      for (const [dx, dy] of [[-1, 1], [1, 1], [-1, 3], [1, 3]]) dot(x + dx, y + dy, c, 0.5);
      fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.85));
    },
  },
  gpt: {
    // A ring of a knot on a hoodie, its hood round the neck, green cuffs
    front(x, y, o) {
      dot(x - 2, y, shaded(o.shirt, 0.7)); dot(x + 2, y, shaded(o.shirt, 0.7));
      for (const [dx, dy] of [[0, 1], [-1, 2], [1, 2], [-1, 3], [1, 3], [0, 4]]) dot(x + dx, y + dy, WHITE);
      dot(x - 4, y + 4, 0x10a37f); dot(x + 4, y + 4, 0x10a37f);
    },
  },
  gemini: {
    // Blue to violet to pink, a four-point sparkle
    front(x, y, o) {
      [0x4c8bf5, 0x5a7cf0, 0x7a6ae8, 0x9a5ad8, 0xb85ac8, 0xd06ab8].forEach((c, r) => fill(x - 3, y + r, 7, 1, shaded(c, o.shade ?? 1)));
      fill(x, y + 1, 1, 3, WHITE); dot(x - 1, y + 2, WHITE); dot(x + 1, y + 2, WHITE);
    },
  },
  grok: {
    // A leather jacket over a white tee, and dark glasses
    front(x, y) { fill(x - 1, y, 3, 6, 0xf4f1ea); dot(x - 2, y, 0x3a3d47); dot(x + 2, y, 0x3a3d47); dot(x + 2, y + 3, 0xc8cdd6); },
    head(x, top, look, away) { if (!away) { fill(x - 3, top + 4, 7, 1, SHADES); dot(x - 2, top + 4, 0x3a3d47); } },
  },
  qwen: {
    // A mandarin collar, gold frog buttons
    front(x, y, o) { fill(x - 2, y, 5, 1, shaded(o.shirt, 1.25)); for (const r of [2, 4]) { fill(x - 1, y + r, 3, 1, PALETTE.brass); dot(x, y + r, 0xf2d38a); } },
  },
  llama: {
    // A woollen poncho's zigzag band, and its fringe
    front(x, y, o) {
      for (let c = -3; c <= 3; c++) { dot(x + c, y + 2 + ((c + 3) % 2), 0x3f6fc4); dot(x + c, y + 4 - ((c + 3) % 2), 0xd96a5a); }
      for (let c = -3; c <= 3; c += 2) dot(x + c, y + 5, shaded(o.shirt, 0.8));
    },
  },
  mistral: {
    // Stripes of orange, red and yellow, and a beret
    front(x, y) { fill(x - 3, y + 1, 7, 1, 0xf08a3a); fill(x - 3, y + 3, 7, 1, 0xe0503a); fill(x - 3, y + 5, 7, 1, 0xf2c200); },
    back(x, y) { fill(x - 3, y + 1, 7, 1, 0xf08a3a); fill(x - 3, y + 3, 7, 1, 0xe0503a); fill(x - 3, y + 5, 7, 1, 0xf2c200); },
    head(x, top) { fill(x - 3, top - 1, 7, 2, 0x1a1a1f); dot(x + 4, top, 0x1a1a1f); dot(x + 1, top - 2, 0x1a1a1f); },
  },
  deepseek: {
    // A turtleneck with a little whale, diving goggles pushed up
    front(x, y, o) { fill(x - 2, y, 5, 1, shaded(o.shirt, 1.25)); fill(x - 2, y + 3, 3, 1, 0x6fa8f0); fill(x - 1, y + 2, 2, 1, 0x6fa8f0); dot(x + 1, y + 2, 0x6fa8f0); dot(x + 2, y + 1, 0x6fa8f0); },
    head(x, top, look, away) { fill(x - 3, top + 2, 7, 1, 0x2a2d35); if (!away) { dot(x - 2, top + 2, 0x8fc6f0); dot(x + 2, top + 2, 0x8fc6f0); } },
  },
  kimi: {
    // A crescent moon and a star, white at the hem
    front(x, y) { for (const [dx, dy] of [[-1, 1], [-2, 2], [-2, 3], [-1, 4]]) dot(x + dx, y + dy, 0xf2d35b); dot(x + 1, y + 1, WHITE); fill(x - 3, y + 5, 7, 1, 0xd8d4c8); },
  },
  glm: {
    // A circuit trace that glows
    front(x, y) {
      const a = 0.6 + 0.4 * Math.sin(now * 2);
      for (const [dx, dy] of [[-2, 1], [-1, 1], [-1, 2], [-1, 3], [0, 3], [1, 3], [2, 3], [2, 4]]) dot(x + dx, y + dy, 0x4ad4e8, a);
      dot(x - 2, y + 1, WHITE, a); dot(x + 2, y + 4, WHITE, a);
    },
  },
  phi: {
    // A polo's collar, and a Greek phi
    front(x, y, o) {
      dot(x - 1, y, shaded(o.shirt, 1.3)); dot(x + 1, y, shaded(o.shirt, 1.3));
      fill(x, y + 1, 1, 4, WHITE); for (const [dx, dy] of [[-1, 2], [1, 2], [-1, 3], [1, 3]]) dot(x + dx, y + dy, WHITE);
    },
  },

  /* Halloween's */
  witch: {
    // A cloak over the robe, a brass clasp at the neck
    front(x, y, o) { fill(x - 3, y, 2, 6, shaded(o.shirt, 0.7)); fill(x + 2, y, 2, 6, shaded(o.shirt, 0.7)); dot(x, y, PALETTE.brass); },
    back(x, y, o) { fill(x - 3, y, 7, 1, shaded(o.shirt, 0.7)); },
    // A wide-brimmed pointed hat, its band in the agent's colour, its tip bent over
    head(x, top, look, away, o) {
      const hat = 0x2a2433;
      fill(x - 5, top, 11, 1, hat);
      fill(x - 3, top - 1, 7, 1, o.accent);
      if (!away) dot(x, top - 1, PALETTE.brass);
      for (const [dy, l, w] of [[2, -2, 5], [3, -2, 5], [4, -1, 3], [5, 0, 3], [6, 1, 2]]) fill(x + l, top - dy, w, 1, hat);
      dot(x + 3, top - 6, hat);
      dot(x + 4, top - 5, hat);
    },
  },
  skeleton: {
    // Bones on black, and a skull for a face
    front(x, y) {
      const bone = 0xeae6dc;
      fill(x, y, 1, 4, bone);
      for (const r of [1, 3]) { fill(x - 2, y + r, 2, 1, bone); fill(x + 1, y + r, 2, 1, bone); }
      dot(x - 2, y, bone); dot(x + 2, y, bone);
      fill(x - 2, y + 5, 5, 1, bone); dot(x, y + 5, 0x1d1d22);
    },
    back(x, y) { fill(x, y, 1, 6, 0xeae6dc); dot(x - 2, y + 1, 0xeae6dc); dot(x + 2, y + 1, 0xeae6dc); },
    head(x, top, look, away) {
      if (away) return;
      fill(x - 3, top + 2, 7, 5, 0xeae6dc);
      fill(x - 2, top + 3, 2, 2, 0x1a1a1f);
      fill(x + 1, top + 3, 2, 2, 0x1a1a1f);
      dot(x, top + 5, 0x6a6a70);
      fill(x - 2, top + 6, 5, 1, 0x1a1a1f);
      dot(x - 1, top + 6, 0xeae6dc);
      dot(x + 1, top + 6, 0xeae6dc);
    },
  },
  mummy: {
    // Wrapped from head to toe, a loose end hanging
    front(x, y, o) {
      for (let r = 0; r < 6; r++) for (let c = -3; c <= 3; c++) if ((c + r + 9) % 3 === 0) dot(x + c, y + r, 0xb8b098);
      dot(x + 4, y + 4, o.shirt);
      dot(x + 4, y + 5, 0xb8b098);
    },
    back(x, y) { for (let r = 0; r < 6; r++) for (let c = -3; c <= 3; c++) if ((c - r + 9) % 3 === 0) dot(x + c, y + r, 0xb8b098); },
    // Over the hair and the face, the eyes peeking out
    head(x, top, look, away, o) {
      const seam = 0xb8b098;
      fill(x - 2, top, 5, 1, o.shirt);
      fill(x - 3, top + 1, 7, 2, o.shirt);
      fill(x - 3, top + (away ? 3 : 5), 7, away ? 4 : 2, o.shirt);
      for (const [dx, dy] of [[-2, 1], [1, 2], [3, 1], [-1, 5], [2, 6], [-3, 6]]) dot(x + dx, top + dy, seam);
      if (!away) dot(x + 4, top + 6, o.shirt);
    },
  },
  refskeleton: {
    // A referee's stripes, a whistle on its cord, and the skeleton's skull
    front(x, y) {
      for (const dx of [-2, 0, 2]) fill(x + dx, y, 1, 6, 0x1d1d22);
      fill(x - 3, y, 7, 1, 0x1d1d22);
      dot(x - 1, y + 1, 0xc8cdd6); dot(x - 1, y + 2, 0xc8cdd6); fill(x - 1, y + 3, 2, 1, 0xc8cdd6);
    },
    back(x, y) { for (const dx of [-2, 0, 2]) fill(x + dx, y, 1, 6, 0x1d1d22); },
    head(x, top, look, away, o) { OUTFIT_PARTS.skeleton.head(x, top, look, away, o); },
  },
  mummyjersey: {
    // A team jersey in the agent's colour, its number 13, the wraps showing round it and coming loose
    front(x, y, o) {
      fill(x - 1, y, 3, 1, 0xe2dcc6);
      dot(x, y + 1, 0xe2dcc6);
      fill(x - 2, y + 2, 1, 3, WHITE);
      fill(x, y + 2, 2, 1, WHITE); dot(x + 1, y + 3, WHITE); fill(x, y + 4, 2, 1, WHITE);
      for (const [dx, dy] of [[-3, 1], [-3, 4], [3, 2], [3, 5], [-2, 5]]) dot(x + dx, y + dy, 0xe2dcc6);
      dot(x + 4, y + 4, 0xe2dcc6);
    },
    back(x, y) { fill(x - 1, y + 1, 3, 3, WHITE); dot(x, y + 2, 0xe2dcc6); dot(x - 3, y + 3, 0xe2dcc6); },
    head(x, top, look, away, o) { OUTFIT_PARTS.mummy.head(x, top, look, away, { ...o, shirt: 0xe2dcc6 }); },
  },
  santa: {
    // White fur down the front and round the hem, a black belt with a gold buckle
    front(x, y) {
      const fur = 0xf6f2ea;
      fill(x, y, 1, 6, fur); fill(x - 3, y + 5, 7, 1, fur);
      fill(x - 3, y + 3, 7, 1, 0x1a1a1f); fill(x - 1, y + 3, 3, 1, 0xe0b84a); dot(x, y + 3, 0x1a1a1f);
    },
    back(x, y) { fill(x - 3, y + 3, 7, 1, 0x1a1a1f); fill(x - 3, y + 5, 7, 1, 0xf6f2ea); },
    // A red hat, its tip flopped over to a white bobble, its brim white fur; and a white beard
    head(x, top, look, away) {
      const red = 0xc8302a, fur = 0xf6f2ea, shade = 0x9a2420;
      fill(x - 3, top - 1, 7, 1, red); fill(x - 2, top - 2, 5, 1, red); fill(x - 1, top - 3, 4, 1, red); fill(x + 1, top - 4, 3, 1, shade);
      dot(x + 3, top - 3, shade); fill(x + 4, top - 4, 2, 2, fur);
      fill(x - 4, top, 9, 2, fur);
      if (away) return;
      fill(x - 3, top + 5, 7, 2, fur); fill(x - 2, top + 7, 5, 1, fur); dot(x, top + 5, 0xe8d8c8);
    },
  },
  xmasjumper: {
    // A reindeer's face knitted on the front, antlers and all, its red nose lighting up now and then; a band of white knit at the hem
    front(x, y, o, look) {
      const face = 0x8a5a36, antler = 0xf0e0b8, knit = 0xf6f2ea;
      dot(x - 2, y, antler); dot(x + 2, y, antler); dot(x - 3, y, antler); dot(x + 3, y, antler);
      dot(x - 2, y + 1, antler); dot(x + 2, y + 1, antler);
      fill(x - 1, y + 1, 3, 3, face); dot(x - 1, y + 2, 0x1a1a1f); dot(x + 1, y + 2, 0x1a1a1f);
      const lit = (now + (look ? look.phase : 0) * 7) % 6 < 1;
      dot(x, y + 3, lit ? 0xff7a5a : 0xc8302a);
      for (let dx = -3; dx <= 3; dx++) dot(x + dx, y + 5, dx % 2 ? knit : 0xc8302a);
    },
    back(x, y) {
      for (let dx = -3; dx <= 3; dx++) { if (dx % 2) dot(x + dx, y + 1, 0xf6f2ea); dot(x + dx, y + 5, dx % 2 ? 0xf6f2ea : 0xc8302a); }
    },
  },
  beanie: {
    // A puffer's quilted rows and its zip, a berry scarf round the neck, its long end down the front, striped, fringed
    front(x, y, o) {
      const berry = 0xb8303a, cream = 0xf2ead8;
      for (const r of [2, 4]) fill(x - 3, y + r, 7, 1, shaded(o.shirt, 0.72));
      fill(x + 1, y + 1, 1, 5, lighter(o.shirt));
      fill(x - 3, y, 7, 1, berry); fill(x - 2, y + 1, 2, 4, berry); fill(x - 2, y + 3, 2, 1, cream);
      dot(x - 2, y + 5, cream); dot(x - 1, y + 5, cream, 0.7);
    },
    back(x, y, o) {
      for (const r of [2, 4]) fill(x - 3, y + r, 7, 1, shaded(o.shirt, 0.72));
      fill(x - 3, y, 7, 1, 0xb8303a); fill(x + 1, y + 1, 2, 3, 0xb8303a); fill(x + 1, y + 2, 2, 1, 0xf2ead8);
    },
    // A ribbed berry beanie, its brim turned up in cream, a big bobble; now and then they blow on their mittened hands
    head(x, top, look, away) {
      const berry = 0xb8303a, rib = 0x8a2430, cream = 0xf2ead8;
      fill(x - 3, top - 2, 7, 3, berry); fill(x - 2, top - 3, 5, 1, berry);
      for (const dx of [-2, 0, 2]) fill(x + dx, top - 2, 1, 3, rib);
      fill(x - 4, top + 1, 9, 1, cream);
      fill(x - 1, top - 5, 3, 2, cream); dot(x, top - 6, cream); dot(x - 1, top - 5, WHITE);
      if (away) return;
      const t = (now + look.phase * 19) % 21;
      if (t < 1.8) {
        fill(x - 3, top + 5, 3, 2, berry); fill(x + 1, top + 5, 3, 2, berry); dot(x - 3, top + 5, 0xe0606a); dot(x + 3, top + 5, 0xe0606a);
        for (let k = 0; k < 3; k++) {
          const rise = (t / 1.8 + k / 3) % 1;
          dot(x + 5 + Math.round(Math.sin(rise * 7 + k)), top + 4 - Math.round(rise * 6), WHITE, 0.75 * (1 - rise));
        }
      }
    },
  },
  cableknit: {
    // Two cables twisting down the front either side of a rib, a rolled collar, ribbing at the hem
    front(x, y, o) {
      const dark = shaded(o.shirt, 0.72), light = lighter(o.shirt);
      fill(x - 2, y, 5, 1, light);
      for (const cx of [-2, 2]) {
        for (let r = 1; r < 5; r++) {
          if (r % 2) dot(x + cx, y + r, dark);
          else { dot(x + cx - 1, y + r, dark); dot(x + cx + 1, y + r, dark); }
        }
      }
      fill(x, y + 1, 1, 4, light);
      fill(x - 3, y + 5, 7, 1, dark);
    },
    back(x, y, o) {
      const dark = shaded(o.shirt, 0.72);
      for (const cx of [-2, 2]) for (let r = 1; r < 5; r += 2) dot(x + cx, y + r, dark);
      fill(x - 3, y + 5, 7, 1, dark);
    },
    // Earmuffs: a dark band over the head, fluffy berry muffs over the ears
    head(x, top) {
      const band = 0x3a3d45, muff = 0xc8303a, fluff = 0xe8707a;
      fill(x - 3, top - 1, 7, 1, band); dot(x - 4, top, band); dot(x + 4, top, band); dot(x - 4, top + 1, band); dot(x + 4, top + 1, band);
      for (const mx of [x - 5, x + 4]) { fill(mx, top + 2, 2, 3, muff); dot(mx, top + 2, fluff); }
    },
  },
  werewolf: {
    // A flannel's checks, fur at its open collar and showing through its torn hem
    front(x, y, o) {
      const check = shaded(o.shirt, 0.65), fur = 0x6e5038;
      for (const dx of [-2, 1]) fill(x + dx, y, 1, 6, check);
      for (const dy of [2, 4]) fill(x - 3, y + dy, 7, 1, check);
      fill(x - 1, y, 3, 1, fur); dot(x, y + 1, fur); dot(x - 1, y + 1, 0x9a7a58);
      for (const [dx, dy] of [[-3, 5], [-1, 5], [2, 5], [3, 4]]) dot(x + dx, y + dy, fur);
    },
    back(x, y, o) {
      const check = shaded(o.shirt, 0.65);
      for (const dx of [-2, 1]) fill(x + dx, y, 1, 6, check);
      for (const dy of [2, 4]) fill(x - 3, y + dy, 7, 1, check);
      dot(x - 2, y + 5, 0x6e5038); dot(x + 1, y + 5, 0x6e5038);
    },
    // A furry head with pointed ears and tufted cheeks, a pale muzzle and yellow eyes; every so often it throws its head back and howls
    head(x, top, look, away) {
      const fur = 0x6e5038, light = 0x9a7a58, dark = 0x4a3424, ear = 0xc88a7a;
      fill(x - 2, top, 5, 1, fur); fill(x - 3, top + 1, 7, 6, fur);
      for (const side of [-1, 1]) {
        dot(x + side * 3, top - 2, fur); fill(side < 0 ? x - 3 : x + 2, top - 1, 2, 1, fur);
        if (!away) dot(x + side * 2, top - 1, ear);
        dot(x + side * 4, top + 4, fur); dot(x + side * 4, top + 5, dark);
      }
      if (away) { fill(x - 1, top + 2, 3, 3, dark); return; }
      const t = (now + look.phase * 23) % 26, howl = t < 1.8;
      if (howl) {
        // Head back, eyes shut, muzzle up, and the howl rising off it
        fill(x - 1, top + 3, 3, 2, light); dot(x, top + 3, 0x1a1a1f);
        dot(x - 2, top + 3, dark); dot(x + 2, top + 3, dark);
        dot(x, top + 5, 0x3a1a1a); dot(x, top + 6, light);
        for (let k = 0; k < 3; k++) {
          const rise = (t / 1.8 + k / 3) % 1;
          dot(x + 2 + k + Math.round(Math.sin(rise * 8)), top - 2 - Math.round(rise * 6), WHITE, 0.7 * (1 - rise));
        }
        return;
      }
      fill(x - 1, top + 4, 3, 3, light); dot(x, top + 4, 0x1a1a1f);
      const blinking = (now + look.phase * 5) % 4.5 < 0.15;
      if (!blinking) { dot(x - 2, top + 3, 0xf2d35b); dot(x + 2, top + 3, 0xf2d35b); }
      else { dot(x - 2, top + 3, dark); dot(x + 2, top + 3, dark); }
      dot(x - 1, top + 6, WHITE); dot(x + 1, top + 6, WHITE);
    },
  },
  tangsuit: {
    // A stand-up collar and the opening trimmed in gold, knot buttons across it, a gold hem
    front(x, y, o) {
      const gold = 0xe8b84a, seam = shaded(o.shirt, 0.62);
      fill(x - 2, y, 5, 1, gold);
      fill(x, y + 1, 1, 4, seam);
      for (const dy of [1, 3]) { fill(x - 1, y + dy, 3, 1, gold); dot(x, y + dy, 0xb8862e); }
      fill(x - 3, y + 5, 7, 1, shaded(gold, 0.85));
    },
    back(x, y) { fill(x - 2, y, 5, 1, 0xe8b84a); fill(x - 3, y + 5, 7, 1, 0xc89a3e); },
  },

  /* The playoffs' */
  playoffjersey: {
    // A white-trimmed neck and armholes, and 11 across the chest in white
    front(x, y) {
      fill(x - 1, y, 3, 1, WHITE);
      for (const side of [-3, 3]) dot(x + side, y, WHITE);
      for (const dx of [-2, 1]) GLYPHS[1].forEach((row, r) => [...row].forEach((on, c) => { if (on === '1') dot(x + dx - 1 + c, y + 1 + r, WHITE); }));
    },
    back(x, y) {
      for (const dx of [-2, 1]) GLYPHS[1].forEach((row, r) => [...row].forEach((on, c) => { if (on === '1') dot(x + dx - 1 + c, y + r, WHITE); }));
    },
    // Every so often, for a few seconds, a ball spun on a raised finger
    raise(look) { return (now + look.phase * 17) % 23 < 3.5; },
    hands(x, top, raised, look) {
      dot(x - 4, top + 11, WHITE);
      if (!raised) { dot(x + 4, top + 11, WHITE); return; }
      dot(x + 5, top + 5, WHITE);
      dot(x + 5, top + 1, look.skin);
      const by = top - 2, spin = Math.floor(now * 10) % 4;
      disc(x + 5, by, 2.4, 0xd9692a);
      dot(x + 4, by - 1, 0xf0a060);
      fill(x + 3 + spin, by - 2, 1, 5, 0x6a2e12, 0.8);
      fill(x + 3, by, 5, 1, 0x6a2e12, 0.6);
    },
  },
  superfan: {
    // A striped scarf round the neck, one end hanging down the front, fringed
    front(x, y) {
      for (let k = -3; k <= 3; k++) dot(x + k, y, Math.floor((k + 3) / 2) % 2 ? SIDE_ORANGE : SIDE_TEAL);
      for (let r = 1; r <= 4; r++) fill(x + 1, y + r, 2, 1, r % 2 ? SIDE_ORANGE : SIDE_TEAL);
      dot(x + 1, y + 5, 0xeae6dc); dot(x + 2, y + 5, 0xeae6dc);
    },
    back(x, y) {
      for (let k = -3; k <= 3; k++) dot(x + k, y, Math.floor((k + 3) / 2) % 2 ? SIDE_ORANGE : SIDE_TEAL);
      for (let r = 1; r <= 3; r++) dot(x - 2, y + r, r % 2 ? SIDE_TEAL : SIDE_ORANGE);
    },
    // A beanie striped in both sides' colours, its bobble on top; paint on the cheeks, a stripe of each
    head(x, top, look, away) {
      fill(x - 3, top - 1, 7, 2, SIDE_TEAL);
      fill(x - 3, top, 7, 1, SIDE_ORANGE);
      fill(x - 3, top + 1, 7, 1, SIDE_TEAL_DARK);
      fill(x - 1, top - 3, 3, 2, 0xeae6dc); dot(x, top - 3, WHITE);
      if (away) return;
      dot(x - 3, top + 5, SIDE_TEAL); dot(x - 2, top + 5, SIDE_ORANGE);
      dot(x + 2, top + 5, SIDE_TEAL); dot(x + 3, top + 5, SIDE_ORANGE);
    },
    // Every so often, for a moment, the foam finger raised high and shaken
    raise(look) { return (now + look.phase * 13) % 19 < 2.6; },
    hands(x, top, raised) {
      if (!raised) return;
      const shake = Math.floor(now * 5) % 2;
      DESK_ITEMS.foamfinger(x + 5 - 39 + shake, top + 3);
    },
  },

  /* The themes' */
  techwear: {
    // A dark jacket, high in the collar, its zip, seams and hem lit in the trim's colour
    front(x, y, o) {
      fill(x - 3, y, 7, 6, 0x23252c); fill(x - 2, y, 5, 1, 0x34374a);
      fill(x, y + 1, 1, 5, o.shirt);
      dot(x - 3, y + 2, o.shirt); dot(x + 3, y + 2, o.shirt);
      dot(x - 2, y + 3, 0x3a3d4a); dot(x + 2, y + 3, 0x3a3d4a);
      fill(x - 3, y + 5, 7, 1, o.shirt, 0.7);
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x23252c); fill(x - 3, y + 2, 7, 1, o.shirt); fill(x - 1, y + 4, 3, 1, o.shirt, 0.6); },
    // A visor across the eyes, lit along its edge; now and then a glint runs across it
    head(x, top, look, away, o) {
      if (away) { fill(x - 3, top + 3, 7, 1, 0x23252c); return; }
      fill(x - 4, top + 3, 9, 2, 0x1a1c26); fill(x - 3, top + 3, 7, 1, o.shirt, 0.6);
      const t = (now + look.phase * 17) % 9;
      if (t < 0.6) { const gx = x - 3 + Math.floor((t / 0.6) * 7); dot(gx, top + 4, WHITE, 0.95); dot(gx, top + 3, WHITE, 0.6); }
    },
  },
  submariner: {
    // A roll-neck jumper, ribbed, its neck turned down
    front(x, y, o) {
      fill(x - 2, y, 5, 2, shaded(o.shirt, 1.3)); fill(x - 2, y + 1, 5, 1, shaded(o.shirt, 1.1));
      for (const dx of [-2, 0, 2]) fill(x + dx, y + 2, 1, 3, shaded(o.shirt, 0.8));
      fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.75));
    },
    back(x, y, o) {
      fill(x - 2, y, 5, 1, shaded(o.shirt, 1.3));
      for (const dx of [-2, 0, 2]) fill(x + dx, y + 1, 1, 4, shaded(o.shirt, 0.8));
      fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.75));
    },
    // A knit cap, its cuff turned up, ribbed
    head(x, top, look, away, o) {
      const cap = shaded(o.shirt, 1.2), cuff = shaded(o.shirt, 0.95);
      fill(x - 2, top - 2, 5, 1, cap); fill(x - 3, top - 1, 7, 2, cap);
      fill(x - 4, top + 1, 9, 2, cuff);
      for (const dx of [-3, -1, 1, 3]) fill(x + dx, top + 1, 1, 2, shaded(o.shirt, 0.72));
    },
  },
  barista: {
    // An apron over the shirt, its straps up to the collar, a pocket and its ties
    front(x, y) {
      const apron = 0x8a5a3a;
      fill(x - 2, y + 1, 5, 5, apron); fill(x - 1, y, 3, 1, apron); dot(x - 2, y, shaded(apron, 0.75)); dot(x + 2, y, shaded(apron, 0.75));
      fill(x - 1, y + 3, 3, 1, shaded(apron, 0.75)); dot(x - 3, y + 2, apron); dot(x + 3, y + 2, apron);
    },
    back(x, y) { const apron = 0x8a5a3a; dot(x - 1, y, apron); dot(x + 1, y, apron); fill(x - 1, y + 2, 3, 1, apron); dot(x - 1, y + 3, apron); dot(x + 1, y + 3, apron); },
    // A slouchy mustard beanie, its cuff turned up
    head(x, top) {
      const knit = 0xd89a3a;
      fill(x - 3, top - 1, 7, 2, knit); dot(x - 2, top - 2, knit); fill(x - 1, top - 2, 3, 1, knit); dot(x + 2, top - 2, shaded(knit, 0.85));
      fill(x - 3, top + 1, 7, 1, shaded(knit, 0.8)); for (const dx of [-2, 0, 2]) dot(x + dx, top + 1, shaded(knit, 0.65));
    },
  },
  cowboy: {
    // A leather vest open over the shirt, a red bandana at the neck, a belt and its buckle
    front(x, y) {
      const vest = 0x6e4428, red = 0xc0392b;
      fill(x - 3, y, 2, 6, vest); fill(x + 2, y, 2, 6, vest); dot(x - 2, y + 2, PALETTE.brass);
      fill(x - 2, y, 5, 1, red); fill(x - 1, y + 1, 3, 1, red); dot(x, y + 2, shaded(red, 0.8));
      fill(x - 3, y + 5, 7, 1, 0x3a2616); dot(x, y + 5, PALETTE.brass);
    },
    back(x, y) { fill(x - 3, y, 7, 5, 0x6e4428); fill(x - 1, y, 3, 1, 0xc0392b); fill(x - 3, y + 5, 7, 1, 0x3a2616); },
    // A cowboy hat, brim curled up at its ends; now and then they tip it, lifting it off and setting it back
    head(x, top, look, away) {
      const hat = 0x9a6a3a, dark = 0x6e4428, band = 0x3a2616;
      const t = (now + look.phase * 19) % 19, tip = !away && t < 1.2 ? (t < 0.3 || t > 0.9 ? 1 : 2) : 0, lean = tip === 2 ? 1 : 0;
      const b = top + 1 - tip;
      fill(x - 5 + lean, b, 11, 1, hat); dot(x - 5 + lean, b - 1, hat); dot(x + 5 + lean, b - 1, hat);
      fill(x - 3 + lean, b - 3, 7, 3, hat); fill(x - 3 + lean, b - 1, 7, 1, band);
      dot(x + lean, b - 3, dark); dot(x - 3 + lean, b - 3, dark); dot(x + 3 + lean, b - 3, dark);
      fill(x - 5 + lean, b, 11, 1, WHITE, 0.08); fill(x - 4 + lean, b + 1, 9, 1, dark, away ? 0 : 0.35);
    },
  },
  kimono: {
    // A plain wrap jacket, its collar band crossed left over right on a pale under-layer, a dark sash tied at the waist
    front(x, y, o) {
      const band = shaded(o.shirt, 0.6), under = 0xeae2d0, sash = 0x2a2a30;
      fill(x - 1, y, 3, 1, under); dot(x, y + 1, under);
      dot(x - 2, y, band); dot(x - 1, y + 1, band); dot(x, y + 2, band);
      dot(x + 2, y, band); dot(x + 1, y + 1, band); dot(x + 1, y + 2, band); dot(x, y + 3, band); dot(x - 1, y + 3, band, 0.7);
      fill(x - 3, y + 4, 7, 1, sash); dot(x + 2, y + 4, shaded(o.shirt, 1.25)); dot(x + 2, y + 5, sash, 0.7);
      dot(x - 3, y + 2, shaded(o.shirt, 0.82)); dot(x + 3, y + 2, shaded(o.shirt, 0.82));
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, shaded(o.shirt, 0.6)); fill(x - 3, y + 4, 7, 1, 0x2a2a30); dot(x, y + 3, 0x2a2a30); },
  },

  /* New Year's Eve's */
  partyhat: {
    // A party shirt, confetti dots over it, its collar open
    front(x, y, o) {
      fill(x - 1, y, 3, 1, shaded(o.shirt, 1.25)); dot(x, y + 1, shaded(o.shirt, 0.8));
      for (const [dx, dy, c] of [[-2, 2, 0xf28fb0], [2, 1, 0x4c8bf5], [1, 3, 0xf4f1ea], [-1, 4, 0x4f9a5a], [2, 4, 0xe0508a], [-3, 3, 0xf4f1ea]]) {
        dot(x + dx, y + dy, c);
      }
    },
    back(x, y) { for (const [dx, dy, c] of [[-2, 1, 0xf28fb0], [1, 2, 0x4c8bf5], [-1, 4, 0xf4f1ea], [2, 4, 0x4f9a5a]]) dot(x + dx, y + dy, c); },
    // A cone hat, striped, a pompom on its tip, a little askew; now and then a party horn unrolls from their lips
    head(x, top, look, away) {
      const hat = 0xe0508a, stripe = 0xf2d35b;
      for (let r = 0; r < 8; r++) {
        const half = Math.round(3 - (r * 3) / 7), lean = r > 4 ? 1 : 0;
        fill(x - half + lean, top - r, half * 2 + 1, 1, r % 3 === 1 ? stripe : hat);
        if (half) dot(x + half + lean, top - r, shaded(r % 3 === 1 ? stripe : hat, 0.75));
      }
      fill(x, top - 9, 3, 2, WHITE); dot(x + 2, top - 9, 0xf2d35b); dot(x, top - 8, 0xe8e4da);
      if (away) return;
      const t = (now + look.phase * 11) % 12;
      if (t < 1.5) {
        const out = t < 0.3 ? t / 0.3 : t > 1.2 ? (1.5 - t) / 0.3 : 1, len = Math.max(1, Math.round(out * 6));
        for (let k = 0; k < len; k++) dot(x + 2 + k, top + 6, k % 2 ? 0xf2d35b : 0x4c8bf5);
        if (len >= 5) { dot(x + 2 + len, top + 5, 0x4c8bf5); dot(x + 2 + len, top + 6, 0x4c8bf5); dot(x + 2 + len, top + 7, 0x4c8bf5); }
      }
    },
  },
  sequin: {
    // A sequin jacket open over a white shirt, a black bow tie at the collar; now and then a glint runs over it
    front(x, y, o) {
      for (let dy = 0; dy < 6; dy++) for (const dx of [-3, -2, 2, 3]) if ((dx + dy) % 2 === 0) dot(x + dx, y + dy, lighter(o.shirt));
      fill(x - 1, y, 3, 6, 0xf4f1ea); dot(x - 1, y + 2, shaded(o.shirt, 0.7)); dot(x + 1, y + 2, shaded(o.shirt, 0.7));
      fill(x - 1, y + 3, 1, 3, shaded(o.shirt, 0.75)); fill(x + 1, y + 3, 1, 3, shaded(o.shirt, 0.75));
      dot(x - 1, y, 0x1a1a1f); dot(x + 1, y, 0x1a1a1f); dot(x, y, 0x34343c);
      const t = (now + x * 0.37 + y * 0.11) % 8;
      if (t < 0.5) {
        const gx = x - 3 + Math.floor((t / 0.5) * 7), gy = y + 2;
        if (gx < x - 1 || gx > x + 1) { dot(gx, gy, WHITE); dot(gx, gy - 1, WHITE, 0.6); dot(gx, gy + 1, WHITE, 0.6); }
      }
    },
    back(x, y, o) { for (let dy = 0; dy < 6; dy++) for (let dx = -3; dx <= 3; dx++) if ((dx + dy) % 2 === 0) dot(x + dx, y + dy, lighter(o.shirt), 0.7); },
  },

  /* The space pieces' */
  spacesuit: {
    // A white EVA suit: a boxy control unit on the chest, a mission patch each side, stripes in the agent's colour
    front(x, y, o) {
      fill(x - 3, y, 7, 1, o.accent); fill(x - 3, y + 4, 7, 1, o.accent);
      fill(x - 2, y + 1, 5, 3, 0xb8bec8); fill(x - 2, y + 1, 5, 1, 0xd8dce2); fill(x - 1, y + 2, 3, 1, 0x2a2e36);
      dot(x - 1, y + 2, 0x6fd36f); dot(x + 1, y + 2, 0xf2d35b); dot(x, y + 3, 0x8a919c);
      dot(x - 3, y + 2, 0x2a4a9a); dot(x - 3, y + 3, 0xf2d35b); dot(x + 3, y + 2, 0xd4703a); dot(x + 3, y + 3, 0x2a4a9a);
      fill(x - 3, y + 5, 7, 1, 0xa8aeb8);
    },
    // The life-support pack on the back
    back(x, y, o) {
      fill(x - 3, y, 7, 5, 0xe2e5ea); fill(x - 3, y, 7, 1, WHITE); fill(x - 2, y + 2, 5, 1, o.accent);
      dot(x - 2, y + 4, 0x8a919c); dot(x + 2, y + 4, 0x8a919c); fill(x - 3, y + 5, 7, 1, 0xa8aeb8);
    },
    // The hard helmet round the face, its gold visor raised over the top, a lamp at its side; now and then the visor glints
    head(x, top, look, away, o) {
      const shell = 0xf2f2ee, edge = 0xc8ccd2, gold = 0xd8a83a;
      fill(x - 3, top - 4, 7, 1, shell); fill(x - 4, top - 3, 9, 1, shell);
      fill(x - 5, top - 2, 11, 3, shell); fill(x - 6, top + 1, 2, 7, shell); fill(x + 5, top + 1, 2, 7, shell);
      fill(x - 6, top + 1, 1, 7, edge); fill(x + 6, top + 1, 1, 7, edge);
      fill(x - 4, top + 8, 9, 1, 0xa8aeb8); fill(x - 5, top + 7, 1, 2, edge); fill(x + 5, top + 7, 1, 2, edge);
      if (away) { fill(x - 4, top + 1, 9, 7, shell); fill(x - 4, top + 4, 9, 1, o.accent); return; }
      fill(x - 3, top - 3, 7, 2, gold); fill(x - 2, top - 3, 4, 1, 0xf2d36b); fill(x - 3, top - 1, 7, 1, 0xa87a2a);
      dot(x + 6, top + 2, 0xfff4c0); dot(x - 5, top + 4, o.accent);
      const t = (now + look.phase * 13) % 11;
      if (t < 0.6) { const gx = x - 3 + Math.floor((t / 0.6) * 7); dot(gx, top - 3, WHITE); dot(gx, top - 2, WHITE, 0.6); }
    },
  },

  /* The court's */
  jersey: {
    // A trimmed neck, piping down the sides and a 7 on the chest, in white
    front(x, y) {
      fill(x - 1, y, 3, 1, WHITE);
      GLYPHS[7].forEach((row, r) => [...row].forEach((on, c) => { if (on === '1') dot(x - 1 + c, y + 1 + r, WHITE); }));
      for (const side of [-3, 3]) { dot(x + side, y + 1, WHITE); dot(x + side, y + 3, WHITE); }
    },
    back(x, y) { GLYPHS[7].forEach((row, r) => [...row].forEach((on, c) => { if (on === '1') dot(x - 1 + c, y + r, WHITE); })); },
  },
  warmup: {
    // A zip up the middle, a stand-up collar, white stripes down the sides
    front(x, y, o) {
      fill(x - 2, y, 5, 1, shaded(o.shirt, 1.3));
      fill(x, y, 1, 6, 0xe8e4da);
      for (const side of [-3, 3]) fill(x + side, y + 1, 1, 5, WHITE);
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, shaded(o.shirt, 1.3)); for (const side of [-3, 3]) fill(x + side, y + 1, 1, 5, WHITE); },
  },
  referee: {
    // The stripes and the whistle, as the skeleton referee's
    front(x, y) { OUTFIT_PARTS.refskeleton.front(x, y); },
    back(x, y) { OUTFIT_PARTS.refskeleton.back(x, y); },
  },
  coachpolo: {
    // A polo's collar, and a whistle on a lanyard in the move's blue
    front(x, y, o) {
      dot(x - 2, y, shaded(o.shirt, 1.3)); dot(x + 2, y, shaded(o.shirt, 1.3)); fill(x - 1, y, 3, 1, shaded(o.shirt, 0.7));
      for (const [dx, dy] of [[-2, 1], [-1, 2], [1, 2], [2, 1]]) dot(x + dx, y + dy, 0x3558ce);
      fill(x, y + 3, 2, 1, 0xc8cdd6);
    },
  },
  /* Saint Valentine's */
  redsuit: {
    // A white collar and a pink tie between the red lapels, a rose in the left one
    front(x, y, o) {
      const lapel = shaded(o.shirt, 0.72);
      dot(x - 1, y, 0xf4f1ea); dot(x + 1, y, 0xf4f1ea);
      fill(x, y, 1, 4, 0xf28aa8); dot(x, y + 4, 0xd8607e);
      dot(x - 2, y, lapel); dot(x + 2, y, lapel); dot(x - 1, y + 1, lapel); dot(x + 1, y + 1, lapel);
      dot(x - 2, y + 1, 0xe8203a); dot(x - 3, y + 1, 0xf05a6a); dot(x - 2, y + 2, 0x3f8a3a);
      dot(x + 1, y + 4, lapel);
    },
    back(x, y, o) { fill(x, y + 3, 1, 3, shaded(o.shirt, 0.8)); },
  },
  heartsweater: {
    // A big heart knitted on its front, in red or, on a red sweater, pink; it beats now and then. Ribbing at the hem.
    front(x, y, o, look) {
      const r = (o.shirt >> 16) & 0xff, g = (o.shirt >> 8) & 0xff, colour = r > 150 && g < 120 ? 0xfbd4de : 0xd8344a;
      const beat = (now + (look ? look.phase * 13 : 0)) % 9 < 0.35;
      const rows = beat ? [[[-2, -1], [1, 2]], [[-3, 3]], [[-3, 3]], [[-2, 2]], [[-1, 1]], [[0, 0]]]
        : [[], [[-2, -1], [1, 2]], [[-2, 2]], [[-2, 2]], [[-1, 1]], [[0, 0]]];
      rows.forEach((runs, dy) => runs.forEach(([a, b]) => fill(x + a, y + dy, b - a + 1, 1, colour)));
      if (!beat) dot(x - 2, y + 1, WHITE, 0.4);
      fill(x - 3, y + 5, 2, 1, darker(o.shirt)); fill(x + 2, y + 5, 2, 1, darker(o.shirt));
    },
    back(x, y, o) { fill(x - 3, y + 5, 7, 1, darker(o.shirt)); },
  },
  blazer: {
    // A white shirt and a dark tie between the lapels, a mic clipped to one, a white pocket square
    front(x, y, o) {
      fill(x - 1, y, 3, 4, 0xf4f1ea);
      fill(x, y + 1, 1, 3, 0x1a1a1f);
      for (const dy of [0, 1]) { dot(x - 2, y + dy, shaded(o.shirt, 1.4)); dot(x + 2, y + dy, shaded(o.shirt, 1.4)); }
      dot(x + 2, y + 2, 0x1a1a1a);
      dot(x - 3, y + 2, WHITE);
    },
  },

  /*
   * The roles'. Each holds what it works with and, now and then, does
   * something with it: `every` is how far through it is, or null. Their
   * `arms` draw both arms and say so; floor-length things are put down
   * while sitting.
   */
  artdirector: {
    // A black turtleneck, its rolled collar, a red scarf round the neck, one end hanging down
    front(x, y, o) {
      fill(x - 2, y, 5, 1, lighter(o.shirt));
      fill(x - 3, y, 6, 1, 0xd4372c); dot(x + 3, y, 0xa32a24);
      fill(x - 2, y + 1, 2, 3, 0xd4372c); dot(x - 1, y + 1, 0xa32a24);
      dot(x - 2, y + 4, 0xa32a24); dot(x - 1, y + 4, 0xd4372c);
    },
    back(x, y) { fill(x - 3, y, 7, 1, 0xd4372c); fill(x + 1, y + 1, 2, 2, 0xa32a24); },
    // A beret, slouched to one side; framing, one eye shut
    head(x, top, look, away) {
      const B = 0x2a2c34;
      fill(x - 2, top - 2, 5, 1, B); fill(x - 3, top - 1, 8, 2, B); dot(x, top - 3, B);
      if (away) return;
      fill(x - 1, top - 2, 2, 1, 0x4a4d58);
      if (every(look, 15, 2.2, 13) !== null) { dot(x - 2, top + 4, look.skin); fill(x - 3, top + 4, 2, 1, PALETTE.eye, 0.6); }
    },
    // Paint on the sleeves; now and then a thumb held up at arm's length, to frame something
    arms(x, top, look, o, sleeve, pose) {
      sleeve(x - 4, top + 8, 1, 4); dot(x - 4, top + 12, look.skin);
      if (every(look, 15, 2.2, 13) !== null && pose.kind !== 'walk') {
        sleeve(x + 4, top + 8, 1, 1); sleeve(x + 5, top + 7, 1, 1); sleeve(x + 6, top + 6, 1, 1);
        fill(x + 7, top + 5, 1, 1, look.skin); fill(x + 7, top + 3, 1, 2, look.skin);
        dot(x + 5, top + 7, 0x4c8ef7);
      } else {
        sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin);
        dot(x + 4, top + 10, 0x4c8ef7);
      }
      dot(x - 4, top + 9, 0xffd23f); dot(x - 4, top + 11, 0xff8fb3);
      return true;
    },
  },
  lawyer: {
    // A pinstripe suit, a white shirt and a deep red tie
    front(x, y, o) {
      const stripe = shaded(o.shirt, 1.5);
      for (const dx of [-3, -1, 1, 3]) fill(x + dx, y, 1, 6, stripe);
      fill(x - 1, y, 3, 2, WHITE); dot(x - 2, y, shaded(o.shirt, 1.8)); dot(x + 2, y, shaded(o.shirt, 1.8));
      fill(x, y, 1, 4, 0x8a2433); dot(x, y + 4, 0x6a1a26);
    },
    back(x, y, o) { for (const dx of [-3, -1, 1, 3]) fill(x + dx, y, 1, 6, shaded(o.shirt, 1.5)); },
    // A briefcase at their side; now and then they straighten the tie
    arms(x, top, look, o, sleeve, pose) {
      sleeve(x - 4, top + 8, 1, 4); dot(x - 4, top + 12, look.skin);
      if (pose.kind !== 'sit') {
        fill(x - 6, top + 12, 2, 1, 0x3a2616);
        fill(x - 8, top + 13, 5, 3, 0x6e4a2e); fill(x - 8, top + 13, 5, 1, 0x8a6040); dot(x - 6, top + 14, PALETTE.brass);
      }
      const p = every(look, 12, 1.4, 11);
      if (p !== null) {
        // A hand at the knot, tugging it left and right
        sleeve(x + 4, top + 8, 1, 2); sleeve(x + 1, top + 8, 3, 1);
        fill(x - (Math.floor(p * 6) % 2), top + 7, 2, 1, look.skin);
      } else {
        sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin);
      }
      return true;
    },
  },
  therapist: {
    // A soft cardigan open over an oatmeal top, two buttons, ribbing at the hem
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0xe6dcc6); dot(x, y, shaded(0xe6dcc6, 0.85));
      dot(x - 2, y + 1, 0xc9a874); dot(x - 2, y + 3, 0xc9a874);
      fill(x - 3, y + 5, 2, 1, shaded(o.shirt, 0.82)); fill(x + 2, y + 5, 2, 1, shaded(o.shirt, 0.82));
    },
    back(x, y, o) { fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.82)); },
    // Round glasses
    head(x, top, look, away) {
      if (away) return;
      const F = 0x3a2a22;
      for (const ex of [x - 2, x + 2]) { dot(ex - 1, top + 4, F); dot(ex + 1, top + 4, F); dot(ex, top + 3, F); }
      dot(x, top + 4, F);
    },
    // Nodding, twice, as they jot it down
    nods(look) { const p = every(look, 11, 1.6, 17); return p !== null && p < 0.5 && Math.floor(p * 8) % 2 === 0 ? 1 : 0; },
    // A notepad held to the chest, a pen ready; now and then a note
    arms(x, top, look, o, sleeve, pose) {
      sleeve(x - 4, top + 8, 1, 3); dot(x - 4, top + 11, look.skin);
      fill(x - 3, top + 9, 3, 4, WHITE); fill(x - 3, top + 9, 3, 1, 0xd8a74a);
      const p = every(look, 11, 1.6, 17);
      for (const [k, dy] of [[0, 10], [1, 12]]) fill(x - 3, top + dy, p !== null && p > 0.35 + k * 0.3 ? 3 : 2, 1, k ? 0x9aa0a6 : 0x7d838f);
      if (p !== null && p > 0.3) {
        sleeve(x + 4, top + 8, 1, 2); sleeve(x + 1, top + 10, 3, 1);
        dot(x, top + 11 - (Math.floor(now * 8) % 2), look.skin); dot(x - 1, top + 10, 0x2a2c30);
      } else {
        sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin); dot(x + 5, top + 11, 0x2a2c30);
      }
      return true;
    },
  },
  plumber: {
    // Overalls over a red shirt: the straps, their brass buttons, a pocket on the bib
    front(x, y, o) {
      fill(x - 3, y, 7, 2, 0xc9443a);
      fill(x - 2, y, 1, 2, o.shirt); fill(x + 2, y, 1, 2, o.shirt);
      dot(x - 2, y + 2, 0xffd23f); dot(x + 2, y + 2, 0xffd23f);
      fill(x - 1, y + 3, 3, 2, shaded(o.shirt, 0.82));
    },
    back(x, y, o) { fill(x - 3, y, 7, 2, 0xc9443a); for (const [dx, dy] of [[-2, 0], [-1, 1], [1, 1], [2, 0]]) dot(x + dx, y + dy, o.shirt); },
    // A cap in the overalls' colour, its peak forward
    head(x, top, look, away, o) {
      const C = shaded(o.shirt, 0.9);
      fill(x - 3, top - 1, 7, 2, C); dot(x, top - 2, lighter(C));
      if (!away) fill(x - 4, top + 1, 9, 1, shaded(o.shirt, 0.65));
    },
    // A big wrench at their side, or on a shoulder sitting down; now and then a twirl
    arms(x, top, look, o, sleeve, pose) {
      const S = 0xb7bdc6, D = 0x7d838f;
      sleeve(x - 4, top + 8, 1, 4); dot(x - 4, top + 12, look.skin);
      const p = pose.kind === 'walk' ? null : every(look, 10, 1.2, 7);
      if (p !== null) {
        sleeve(x + 4, top + 8, 1, 2); const hx = x + 5, hy = top + 10;
        dot(hx, hy, look.skin);
        const a = p * Math.PI * 4 - Math.PI / 2, c = Math.cos(a), sn = Math.sin(a);
        for (let k = 1; k <= 4; k++) dot(hx + Math.round(c * k), hy + Math.round(sn * k), k === 4 ? D : S);
        dot(hx + Math.round(c * 5 - sn), hy + Math.round(sn * 5 + c), D); dot(hx + Math.round(c * 5 + sn), hy + Math.round(sn * 5 - c), D);
      } else if (pose.kind === 'sit') {
        sleeve(x + 4, top + 8, 1, 2); dot(x + 4, top + 10, look.skin);
        fill(x + 5, top + 6, 1, 5, S); fill(x + 4, top + 5, 3, 1, D); dot(x + 4, top + 4, D); dot(x + 6, top + 4, D);
      } else {
        sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin);
        fill(x + 5, top + 11, 1, 4, S); fill(x + 4, top + 15, 3, 1, D); dot(x + 4, top + 16, D); dot(x + 6, top + 16, D);
      }
      return true;
    },
  },
  gmexec: {
    // A three-piece suit: the waistcoat between the jacket's fronts, a white collar, the tie in the agent's colour, a watch chain
    front(x, y, o) {
      const vest = shaded(o.shirt, 1.55), tie = o.accent === o.shirt ? shaded(o.accent, 1.5) : o.accent;
      fill(x - 2, y + 1, 5, 5, vest);
      fill(x - 1, y, 3, 1, WHITE); dot(x - 2, y, shaded(o.shirt, 1.3)); dot(x + 2, y, shaded(o.shirt, 1.3));
      fill(x, y, 1, 5, tie); dot(x, y + 5, shaded(tie, 0.75));
      dot(x - 1, y + 2, darker(vest)); dot(x - 1, y + 4, darker(vest));
      dot(x - 2, y + 3, PALETTE.brass); dot(x - 1, y + 3, PALETTE.brass); dot(x - 2, y + 4, PALETTE.brass);
    },
    back(x, y, o) { fill(x, y + 3, 1, 3, darker(o.shirt)); },
    // Now and then the pocket watch, held up and opened
    arms(x, top, look, o, sleeve, pose) {
      const p = every(look, 13, 1.8, 19);
      if (p === null || pose.kind === 'walk') return false;
      sleeve(x - 4, top + 8, 1, 2); dot(x - 3, top + 10, look.skin);
      fill(x - 3, top + 8, 2, 2, PALETTE.brass); dot(x - 3, top + 8, WHITE); dot(x - 1, top + 10, PALETTE.brass);
      if (p > 0.2) fill(x - 4, top + 6, 2, 1, PALETTE.brass);
      sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin);
      return true;
    },
  },
  gmcoach: {
    // A blazer over a white polo, its collar up, a brass button
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0xeeeae2); dot(x - 1, y, WHITE); dot(x + 1, y, WHITE); dot(x, y + 1, 0xc8cdd6);
      dot(x - 2, y, shaded(o.shirt, 1.45)); dot(x + 2, y, shaded(o.shirt, 1.45)); dot(x - 2, y + 1, shaded(o.shirt, 1.45));
      dot(x - 2, y + 4, PALETTE.brass);
    },
    // A headset: its band, its ear cups, its mic to the mouth
    head(x, top, look, away) {
      const H = 0x2a2c30;
      fill(x - 3, top - 1, 7, 1, H); fill(x - 4, top, 1, 5, H); fill(x + 4, top, 1, 5, H);
      if (away) return;
      dot(x - 4, top + 5, H); fill(x - 3, top + 6, 2, 1, H); dot(x - 1, top + 6, 0x5a5f6c);
    },
    // A clipboard in hand; now and then a tap on it
    arms(x, top, look, o, sleeve, pose) {
      sleeve(x - 4, top + 8, 1, 3); dot(x - 4, top + 11, look.skin);
      fill(x - 8, top + 8, 4, 5, 0xb07a46); fill(x - 7, top + 9, 2, 3, WHITE); fill(x - 7, top + 8, 2, 1, 0x7d838f);
      dot(x - 7, top + 10, 0x7d838f);
      const p = every(look, 9, 1.4, 5);
      if (p !== null) {
        sleeve(x + 4, top + 8, 1, 3); sleeve(x - 2, top + 11, 6, 1);
        dot(x - 3, top + 11 - (Math.floor(p * 8) % 2), look.skin);
      } else {
        sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin);
      }
      return true;
    },
  },
  gmtycoon: {
    // A tailcoat over a white shirt front, a red bow tie and studs
    front(x, y, o) {
      fill(x - 1, y, 3, 5, WHITE);
      dot(x - 1, y, 0xc9443a); dot(x + 1, y, 0xc9443a); dot(x, y, 0xa32a24);
      dot(x, y + 2, 0xc8cdd6); dot(x, y + 4, 0xc8cdd6);
      dot(x - 2, y, shaded(o.shirt, 1.6)); dot(x + 2, y, shaded(o.shirt, 1.6));
    },
    back(x, y, o) { fill(x, y + 3, 1, 3, darker(o.shirt)); },
    // A tall top hat with a red band; now and then it's tipped
    head(x, top, look, away) {
      const H = 0x1d1f25, lift = every(look, 11, 1.2, 3) !== null && !away ? 2 : 0, tilt = lift ? 1 : 0;
      fill(x - 4 + tilt, top - lift, 9, 1, H);
      fill(x - 3 + tilt, top - 5 - lift, 7, 5, H);
      fill(x - 3 + tilt, top - 1 - lift, 7, 1, 0xa32a24);
      fill(x - 2 + tilt, top - 5 - lift, 1, 3, 0x3a3d47);
    },
    // A cane in one hand, the coat's tails behind; now and then the other hand tips the hat
    arms(x, top, look, o, sleeve, pose) {
      if (pose.kind !== 'sit') {
        fill(x - 3, top + 13, 1, 2, o.shirt); fill(x + 3, top + 13, 1, 2, o.shirt);
        fill(x - 5, top + 12, 1, 5, 0x3a2616); dot(x - 5, top + 11, PALETTE.brass);
      }
      sleeve(x - 4, top + 8, 1, 4); dot(x - 4, top + 12, look.skin);
      if (every(look, 11, 1.2, 3) !== null) {
        sleeve(x + 4, top + 5, 1, 3); sleeve(x + 5, top + 1, 1, 4); dot(x + 5, top, look.skin);
      } else {
        sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin);
      }
      return true;
    },
  },
  /*
   * The roles', again: a detective, a hard hat, a librarian, a janitor, a firefighter, a pilot, a postman, an
   * accountant, a marketer, a security guard and a chef. Each holds what it works with and, now and then, uses it.
   */
  detective: {
    // A belted trench coat over a white shirt: wide lapels, its buttons, the belt and its buckle
    front(x, y, o) {
      const L = shaded(o.shirt, 1.2), D = shaded(o.shirt, 0.7);
      fill(x - 1, y, 3, 1, WHITE); dot(x, y + 1, WHITE);
      dot(x - 2, y, L); dot(x + 2, y, L); dot(x - 1, y + 1, L); dot(x + 1, y + 1, L);
      dot(x - 2, y + 2, D); dot(x - 2, y + 3, D);
      fill(x - 3, y + 4, 7, 1, D); dot(x - 1, y + 4, PALETTE.brass);
    },
    back(x, y, o) { fill(x - 3, y, 7, 1, shaded(o.shirt, 1.2)); fill(x - 3, y + 4, 7, 1, shaded(o.shirt, 0.7)); },
    // A checked deerstalker, a peak in front and behind, its flaps tied up on top; now and then the glass to one eye
    head(x, top, look, away) {
      const C = 0x9a7650, K = 0x6a5038;
      fill(x - 2, top - 2, 5, 1, C); fill(x - 3, top - 1, 7, 2, C);
      for (const [dx, dy] of [[-1, -2], [1, -2], [-2, -1], [0, -1], [2, -1], [-3, 0], [-1, 0], [1, 0], [3, 0]]) dot(x + dx, top + dy, K);
      dot(x, top - 3, K); dot(x - 4, top + 1, C); dot(x + 4, top + 1, C);
      fill(x - 2, top + 1, 5, 1, K);
      if (away || every(look, 14, 2, 9) === null) return;
      // The magnifying glass held to the right eye, the eye big behind it
      const F = 0x3a2a22;
      fill(x + 1, top + 2, 3, 1, F); fill(x + 1, top + 6, 3, 1, F); fill(x, top + 3, 1, 3, F); fill(x + 4, top + 3, 1, 3, F);
      fill(x + 1, top + 3, 3, 3, GLASS, 0.55);
      fill(x + 1, top + 3, 2, 2, PALETTE.eye); dot(x + 1, top + 3, WHITE);
      dot(x + 4, top + 6, 0x6e4a2e); dot(x + 5, top + 7, 0x6e4a2e);
    },
    // The coat's skirt over the legs; the glass in hand at the side, raised now and then (see head)
    arms(x, top, look, o, sleeve, pose) {
      if (pose.kind !== 'sit') { fill(x - 3, top + 13, 7, 2, o.shirt); dot(x, top + 14, shaded(o.shirt, 0.7)); }
      sleeve(x - 4, top + 8, 1, 4); dot(x - 4, top + 12, look.skin);
      if (every(look, 14, 2, 9) !== null) {
        sleeve(x + 4, top + 8, 1, 3); dot(x + 5, top + 8, look.skin);
      } else {
        sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin);
        const F = 0x3a2a22;
        dot(x + 5, top + 12, 0x6e4a2e);
        fill(x + 5, top + 13, 3, 1, F); dot(x + 5, top + 14, F); dot(x + 6, top + 14, GLASS); dot(x + 7, top + 14, F); fill(x + 5, top + 15, 3, 1, F);
      }
      return true;
    },
  },
  hardhat: {
    // A hi-vis vest open over a blue work shirt, silver bands across it
    front(x, y) {
      const S = 0xe4e8ee;
      fill(x - 1, y, 3, 6, 0x5a7fb8); dot(x, y, 0x46679a);
      fill(x - 3, y + 1, 2, 1, S); fill(x + 2, y + 1, 2, 1, S); fill(x - 3, y + 5, 2, 1, S); fill(x + 2, y + 5, 2, 1, S);
    },
    back(x, y) { fill(x - 3, y + 1, 7, 1, 0xe4e8ee); fill(x - 3, y + 5, 7, 1, 0xe4e8ee); },
    // A yellow hard hat, its ridge and brim
    head(x, top) {
      fill(x - 2, top - 2, 5, 1, 0xffd23f); fill(x - 3, top - 1, 7, 2, 0xffd23f);
      fill(x, top - 2, 1, 2, 0xfff0a0); fill(x - 4, top + 1, 9, 1, 0xe8a33a);
    },
    // A tool belt with a pouch and a wrench; now and then a knock on the hat
    arms(x, top, look, o, sleeve, pose) {
      if (pose.kind !== 'sit') {
        fill(x - 3, top + 13, 7, 1, 0x8a5a2b); dot(x, top + 13, PALETTE.brass);
        fill(x - 3, top + 14, 2, 2, 0x6e4a2e); dot(x - 3, top + 14, 0x8a5a2b);
        dot(x + 3, top + 14, 0xb7bdc6); dot(x + 3, top + 15, 0xb7bdc6); dot(x + 2, top + 16, 0x7d838f); dot(x + 4, top + 16, 0x7d838f);
      }
      sleeve(x - 4, top + 8, 1, 4); dot(x - 4, top + 12, look.skin);
      const p = pose.kind === 'walk' ? null : every(look, 14, 1.4, 11);
      if (p !== null) {
        sleeve(x + 4, top + 7, 1, 1); sleeve(x + 5, top + 3, 1, 4); dot(x + 5, top + 1 + (Math.floor(p * 6) % 2), look.skin);
      } else {
        sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin);
      }
      return true;
    },
  },
  librarian: {
    // A cardigan open over a cream blouse, its round collar, buttons down one front
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0xf0e6d0); dot(x - 1, y, WHITE); dot(x + 1, y, WHITE);
      dot(x - 2, y + 1, darker(o.shirt)); dot(x - 2, y + 3, darker(o.shirt));
      fill(x - 3, y + 5, 2, 1, shaded(o.shirt, 0.82)); fill(x + 2, y + 5, 2, 1, shaded(o.shirt, 0.82));
    },
    back(x, y, o) { fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.82)); },
    // Half-moon glasses on a gold chain; now and then a finger to the lips: shh
    head(x, top, look, away) {
      if (away) return;
      const F = 0x3a2a22, G = 0xd8b45a;
      fill(x - 3, top + 5, 3, 1, F); fill(x + 1, top + 5, 3, 1, F); dot(x - 3, top + 4, F); dot(x + 3, top + 4, F);
      dot(x - 3, top + 6, G); dot(x + 3, top + 6, G);
      if (every(look, 13, 1.6, 7) !== null) fill(x, top + 5, 1, 2, shaded(look.skin, 0.68));
    },
    // A stack of books held to the chest, a quill in the top one
    arms(x, top, look, o, sleeve, pose) {
      sleeve(x - 4, top + 8, 1, 3);
      [[0xa32a24, 0], [0x2f5fb3, 1], [0x3a7a4a, 0]].forEach(([c, dx], i) => {
        fill(x - 4 + dx, top + 9 + i, 4, 1, c); dot(x - 1 + dx, top + 9 + i, 0xf0e6d0);
      });
      dot(x - 4, top + 12, look.skin);
      dot(x - 3, top + 8, WHITE); dot(x - 2, top + 7, WHITE);
      if (every(look, 13, 1.6, 7) !== null) {
        sleeve(x + 4, top + 8, 1, 1); sleeve(x + 1, top + 7, 3, 1); dot(x, top + 7, look.skin);
      } else {
        sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin);
      }
      return true;
    },
  },
  janitor: {
    // Grey coveralls: the collar, a zip down the front, a chest pocket with a pen
    front(x, y, o) {
      fill(x - 2, y, 5, 1, lighter(o.shirt)); dot(x, y, o.shirt);
      fill(x, y + 1, 1, 5, darker(o.shirt));
      fill(x - 3, y + 2, 2, 2, shaded(o.shirt, 0.85)); dot(x - 2, y + 1, 0x4c8ef7);
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, lighter(o.shirt)); },
    // A broom at their side (put down sitting); now and then a sweep, a puff of dust
    arms(x, top, look, o, sleeve, pose) {
      sleeve(x - 4, top + 8, 1, 4); dot(x - 4, top + 12, look.skin);
      if (pose.kind === 'sit') { sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin); return true; }
      sleeve(x + 4, top + 8, 1, 3); dot(x + 4, top + 11, look.skin);
      const p = pose.kind === 'walk' ? null : every(look, 9, 1.8, 5);
      const off = p === null ? 0 : Math.round(Math.sin(p * Math.PI * 4) * 2), bx = x + 5 + off;
      for (let k = 0; k < 9; k++) dot(x + 5 + Math.round((off * k) / 8), top + 6 + k, 0x8a5a2b);
      dot(bx, top + 14, 0xa32a24);
      fill(bx - 1, top + 15, 3, 1, 0xd8b45a); fill(bx - 2, top + 16, 5, 1, 0xb08a3a);
      if (p !== null && Math.floor(now * 6) % 2) dot(bx + (off >= 0 ? 3 : -3), top + 16, 0xc8c4ba, 0.7);
      return true;
    },
  },
  firefighter: {
    // A turnout coat: the raised collar, clasps down the front, reflective bands round the chest and the hem
    front(x, y, o) {
      const Y = 0xe6f05a;
      fill(x - 2, y, 5, 1, shaded(o.shirt, 1.15)); fill(x, y + 1, 1, 5, darker(o.shirt));
      fill(x - 3, y + 1, 7, 1, Y); fill(x - 3, y + 5, 7, 1, Y);
      dot(x - 1, y + 2, 0x2a2c30); dot(x - 1, y + 4, 0x2a2c30);
    },
    back(x, y) { fill(x - 3, y + 1, 7, 1, 0xe6f05a); fill(x - 3, y + 5, 7, 1, 0xe6f05a); },
    // A red helmet, a brass shield on its front, its long brim at the back
    head(x, top, look, away) {
      const R = 0xd4372c, D = 0xa32a24;
      fill(x - 2, top - 3, 5, 1, R); fill(x - 3, top - 2, 7, 3, R); fill(x, top - 3, 1, 3, D);
      if (away) { fill(x - 4, top + 1, 9, 2, D); return; }
      dot(x - 2, top - 2, 0xff7a6e);
      fill(x - 4, top + 1, 9, 1, D);
      fill(x - 1, top - 2, 3, 2, 0xf2c94a); dot(x, top - 1, D);
    },
    // Bands on the sleeves, an extinguisher at their side (put down sitting); now and then a puff from it
    arms(x, top, look, o, sleeve, pose) {
      for (const side of [-4, 4]) { sleeve(x + side, top + 8, 1, 4); dot(x + side, top + 12, look.skin); dot(x + side, top + 10, 0xe6f05a); }
      if (pose.kind === 'sit') return true;
      fill(x + 5, top + 12, 2, 4, 0xd4372c); dot(x + 5, top + 12, 0xff7a6e); fill(x + 5, top + 11, 2, 1, 0x2a2c30); dot(x + 7, top + 11, 0x2a2c30);
      const p = pose.kind === 'walk' ? null : every(look, 13, 1.2, 9);
      if (p !== null) {
        const r = Math.floor(p * 4);
        fill(x + 8, top + 10, 1 + r, 2, WHITE, 0.85 - p * 0.5); dot(x + 9 + r, top + 9, WHITE, 0.5); dot(x + 8 + r, top + 12, WHITE, 0.5);
      }
      return true;
    },
  },
  pilot: {
    // A pilot's jacket over a white shirt and a dark tie, gold wings on the chest, gold buttons
    front(x, y, o) {
      const G = 0xffd23f;
      fill(x - 1, y, 3, 2, WHITE); fill(x, y, 1, 4, 0x1d1f25);
      dot(x - 2, y, lighter(o.shirt)); dot(x + 2, y, lighter(o.shirt));
      fill(x - 3, y + 2, 3, 1, G); dot(x - 2, y + 2, 0xe8a33a); dot(x - 1, y + 4, G);
    },
    // A captain's cap: a white top, a dark band with a gold badge, a black peak
    head(x, top, look, away) {
      fill(x - 4, top - 2, 9, 1, 0xf4f1ea); fill(x - 3, top - 1, 7, 1, 0xf4f1ea); fill(x - 3, top, 7, 1, 0x1f2a4a);
      if (away) return;
      dot(x, top, 0xffd23f); dot(x, top - 1, 0xe8a33a);
      fill(x - 3, top + 1, 7, 1, 0x1a1a1f); dot(x - 2, top + 1, 0x3a3d47);
    },
    // Gold rings on the cuffs; now and then a salute
    arms(x, top, look, o, sleeve, pose) {
      const G = 0xffd23f;
      sleeve(x - 4, top + 8, 1, 4); dot(x - 4, top + 12, look.skin); dot(x - 4, top + 11, G);
      if (pose.kind !== 'walk' && every(look, 12, 1.4, 13) !== null) {
        sleeve(x + 4, top + 7, 1, 1); sleeve(x + 5, top + 3, 1, 4); dot(x + 5, top + 3, G); fill(x + 4, top + 2, 2, 1, look.skin);
      } else {
        sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin); dot(x + 4, top + 11, G);
      }
      return true;
    },
  },
  postman: {
    // A postal jacket: red piping at the collar and the hem, a satchel's strap across the chest
    front(x, y, o) {
      const R = 0xd4372c;
      fill(x - 1, y, 3, 1, 0xc8dcf0); dot(x - 2, y, R); dot(x + 2, y, R);
      fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.8)); dot(x - 3, y + 5, R); dot(x + 3, y + 5, R);
      for (let k = 0; k < 6; k++) dot(x + 3 - k, y + k, 0x8a5a2b);
    },
    back(x, y) { for (let k = 0; k < 6; k++) dot(x - 3 + k, y + k, 0x8a5a2b); },
    // A cap in the jacket's colour with a red band
    head(x, top, look, away, o) {
      const C = shaded(o.shirt, 0.9);
      fill(x - 3, top - 1, 7, 2, C); fill(x - 3, top, 7, 1, 0xd4372c); dot(x, top - 1, WHITE);
      if (!away) fill(x - 4, top + 1, 9, 1, 0x1a1a1f);
    },
    // The satchel at the hip, letters peeking out (put down sitting); now and then one held up
    arms(x, top, look, o, sleeve, pose) {
      if (pose.kind !== 'sit') {
        fill(x - 6, top + 10, 2, 1, WHITE); dot(x - 7, top + 10, 0xf0e6d0);
        fill(x - 8, top + 11, 4, 4, 0x8a5a2b); fill(x - 8, top + 11, 4, 1, 0x6e4a2e); dot(x - 6, top + 12, PALETTE.brass);
      }
      sleeve(x - 4, top + 8, 1, 4); dot(x - 4, top + 12, look.skin);
      const p = every(look, 11, 1.6, 7);
      if (p !== null) {
        const w = Math.floor(p * 6) % 2;
        sleeve(x + 4, top + 5, 1, 3); dot(x + 4, top + 4, look.skin);
        fill(x + 4 + w, top + 1, 4, 3, WHITE); dot(x + 5 + w, top + 2, 0xc4c8d0); dot(x + 6 + w, top + 2, 0xc4c8d0); dot(x + 7 + w, top + 1, 0xd4372c);
      } else {
        sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin);
      }
      return true;
    },
  },
  accountant: {
    // A shirt and a deep red tie
    front(x, y) {
      const B = 0x8a2433;
      dot(x - 1, y, WHITE); dot(x + 1, y, WHITE); fill(x, y, 1, 4, B); dot(x, y + 4, shaded(B, 0.75));
    },
    // A green eyeshade, its band round the head, its green shadow on the face
    head(x, top, look, away) {
      fill(x - 3, top + 1, 7, 1, 0x1f5a3a);
      if (away) return;
      fill(x - 4, top + 2, 9, 1, 0x3fbf6b, 0.9); fill(x - 3, top + 3, 7, 1, 0x3fbf6b, 0.22);
    },
    // Garters on the sleeves, a calculator in hand; now and then a sum tapped into it
    arms(x, top, look, o, sleeve, pose) {
      const G = 0x8a2433;
      sleeve(x - 4, top + 8, 1, 3); dot(x - 4, top + 9, G); dot(x - 4, top + 11, look.skin);
      fill(x - 3, top + 9, 3, 4, 0x3a3d47); fill(x - 3, top + 9, 3, 1, 0xa8d870);
      for (const [dx, dy] of [[-3, 11], [-1, 11], [-2, 12]]) dot(x + dx, top + dy, 0xc8cdd6);
      const p = every(look, 10, 1.6, 7);
      if (p !== null) {
        sleeve(x + 4, top + 8, 1, 2); sleeve(x + 1, top + 10, 3, 1); dot(x, top + 11 - (Math.floor(p * 10) % 2), look.skin);
        dot(x - 3 + (Math.floor(now * 8) % 3), top + 9, 0x2a4a1a);
      } else {
        sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin);
      }
      dot(x + 4, top + 9, G);
      return true;
    },
  },
  marketer: {
    // A sharp blazer over a black tee, a yellow pocket square
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0x1d1f25);
      dot(x - 2, y, lighter(o.shirt)); dot(x + 2, y, lighter(o.shirt)); dot(x - 2, y + 1, lighter(o.shirt));
      dot(x - 3, y + 2, 0xffd23f); dot(x - 2, y + 4, darker(o.shirt));
    },
    back(x, y, o) { fill(x, y + 3, 1, 3, darker(o.shirt)); },
    // Sunglasses pushed up on the head
    head(x, top, look, away) {
      fill(x - 3, top, 3, 1, SHADES); fill(x + 1, top, 3, 1, SHADES); dot(x, top, SHADES);
      if (!away) { dot(x - 2, top, 0x8a8f9c); dot(x + 2, top, 0x8a8f9c); }
    },
    // A megaphone in hand; now and then raised to the mouth and called through
    arms(x, top, look, o, sleeve, pose) {
      const W = 0xf4f1ea, R = 0xe0483e;
      sleeve(x - 4, top + 8, 1, 4); dot(x - 4, top + 12, look.skin);
      if (pose.kind !== 'walk' && every(look, 16, 2, 11) !== null) {
        sleeve(x + 4, top + 7, 1, 3);
        fill(x + 4, top + 5, 1, 2, W); fill(x + 5, top + 4, 1, 4, W); dot(x + 5, top + 7, 0xc4c8d0); fill(x + 6, top + 3, 1, 6, R);
        if (Math.floor(now * 5) % 2) { dot(x + 8, top + 3, W, 0.8); fill(x + 8, top + 5, 1, 2, W, 0.8); dot(x + 8, top + 8, W, 0.8); }
        else fill(x + 9, top + 4, 1, 4, W, 0.6);
      } else {
        sleeve(x + 4, top + 8, 1, 4); dot(x + 4, top + 12, look.skin);
        fill(x + 5, top + 11, 1, 2, W); fill(x + 6, top + 10, 1, 4, R);
      }
      return true;
    },
  },
  securityguard: {
    // A guard's shirt: silver-tipped epaulettes, a gold shield on the chest, a dark tie
    front(x, y, o) {
      const G = 0xffd23f, L = lighter(o.shirt);
      dot(x - 3, y, 0xc4c8d0); dot(x + 3, y, 0xc4c8d0); dot(x - 1, y, L); dot(x + 1, y, L);
      fill(x, y, 1, 4, 0x1a1a1f);
      fill(x - 3, y + 2, 2, 1, G); dot(x - 3, y + 3, 0xe8a33a); dot(x - 2, y + 3, G);
    },
    back(x, y) { dot(x - 3, y, 0xc4c8d0); dot(x + 3, y, 0xc4c8d0); },
    // A peaked cap in the uniform's colour, a silver badge, a black peak
    head(x, top, look, away, o) {
      const C = shaded(o.shirt, 0.9);
      fill(x - 4, top - 2, 9, 1, C); fill(x - 3, top - 1, 7, 2, C); fill(x - 3, top, 7, 1, 0x1a1a1f);
      if (away) return;
      dot(x, top - 1, 0xc4c8d0); fill(x - 3, top + 1, 7, 1, 0x1a1a1f); dot(x - 2, top + 1, 0x3a3d47);
    },
    // A duty belt, a torch in hand; now and then it's switched on, a soft beam ahead
    arms(x, top, look, o, sleeve, pose) {
      if (pose.kind !== 'sit') { fill(x - 3, top + 13, 7, 1, 0x1a1a1f); dot(x, top + 13, 0xc4c8d0); }
      for (const side of [-4, 4]) { sleeve(x + side, top + 8, 1, 4); dot(x + side, top + 12, look.skin); }
      fill(x + 5, top + 12, 2, 1, 0x3a3d47); dot(x + 7, top + 12, 0x7d838f);
      const p = every(look, 9, 2.6, 13);
      if (p !== null) {
        const on = p < 0.08 && Math.floor(now * 20) % 2 ? 0 : 1;
        dot(x + 7, top + 12, 0xfff3b0);
        for (let k = 1; k <= 6; k++) fill(x + 7 + k, top + 12 - Math.floor(k / 2), 1, 1 + 2 * Math.floor(k / 2), 0xfff3b0, 0.3 * on);
      }
      return true;
    },
  },
  chef: {
    // Chef's whites: double-breasted, a row of buttons, a red neckerchief
    front(x, y, o) {
      const D = shaded(o.shirt, 0.8);
      fill(x - 1, y, 3, 1, 0xe0483e); dot(x, y + 1, 0xe0483e); dot(x - 2, y, D); dot(x + 2, y, D);
      fill(x - 1, y + 2, 1, 4, D); dot(x - 2, y + 2, 0x9aa0a6); dot(x - 2, y + 4, 0x9aa0a6);
      dot(x - 3, y + 5, D); dot(x + 3, y + 5, D);
    },
    back(x, y, o) { fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.8)); },
    // A tall pleated toque
    head(x, top) {
      const W = 0xfbfaf6, S = 0xd8d4c8;
      fill(x - 2, top - 6, 5, 1, W); fill(x - 3, top - 5, 7, 4, W); fill(x - 4, top - 4, 9, 2, W);
      fill(x + 3, top - 4, 1, 2, S); dot(x + 4, top - 3, S); fill(x - 3, top - 2, 7, 1, S);
      fill(x - 3, top - 1, 7, 2, W); for (const dx of [-2, 0, 2]) dot(x + dx, top, S);
    },
    // Checked trousers; a mixing bowl in the crook of an arm and a whisk; now and then a whisk round the bowl
    arms(x, top, look, o, sleeve, pose) {
      const M = 0xc4c8d0, MD = 0x7d838f, B = 0xf2d35b;
      if (pose.kind === 'stand') for (const [dx, dy] of [[-2, 14], [-1, 15], [1, 15], [2, 14]]) dot(x + dx, top + dy, 0xc8cdd6);
      sleeve(x - 4, top + 8, 1, 3); dot(x - 4, top + 11, look.skin);
      fill(x - 3, top + 9, 2, 1, B); fill(x - 4, top + 10, 4, 1, M); fill(x - 3, top + 11, 2, 1, MD);
      const p = every(look, 9, 1.6, 5);
      if (p !== null) {
        const w = Math.floor(now * 10) % 2;
        sleeve(x + 4, top + 8, 1, 1); sleeve(x + 2, top + 9, 2, 1); dot(x + 1, top + 9, look.skin);
        dot(x, top + 8, MD); dot(x - 1 + w, top + 8, M); dot(x - 2 + w, top + 9, M);
        dot(w ? x - 4 : x - 1, top + 7 - w, B);
      } else {
        sleeve(x + 4, top + 8, 1, 3); dot(x + 4, top + 11, look.skin);
        fill(x + 5, top + 9, 1, 3, MD);
        for (const [dx, dy] of [[0, 5], [-1, 6], [1, 6], [-1, 7], [1, 7], [0, 8]]) dot(x + 5 + dx, top + dy, M);
      }
      return true;
    },
  },
  cultistrobe: {
    // A long plum robe with a rope belt and a green sigil on the chest that breathes; a deep hood with the face in shadow
    front(x, y, o) {
      const D = shaded(o.shirt, 0.7), L = shaded(o.shirt, 1.3);
      fill(x - 3, y, 1, 6, L); fill(x + 3, y, 1, 6, D); fill(x, y + 1, 1, 5, D);
      fill(x - 3, y + 4, 7, 1, 0xc8a050, 0.9);
      const a = 0.55 + 0.45 * Math.sin(now * 1.4 + x * 0.3);
      dot(x - 1, y + 1, 0x7fffc0, a); dot(x + 1, y + 1, 0x7fffc0, a); dot(x, y + 2, 0x7fffc0, a);
    },
    back(x, y, o) { fill(x - 3, y, 1, 6, shaded(o.shirt, 1.3)); fill(x + 3, y, 1, 6, shaded(o.shirt, 0.7)); fill(x, y, 1, 6, shaded(o.shirt, 0.8)); fill(x - 3, y + 4, 7, 1, 0xc8a050, 0.9); },
    head(x, top, look, away, o) {
      const H = o.shirt, L = shaded(H, 1.3), D = shaded(H, 0.65);
      fill(x - 3, top - 2, 7, 1, H); fill(x - 4, top - 1, 9, 2, H); fill(x - 4, top + 1, 2, 5, H); fill(x + 3, top + 1, 2, 5, H);
      dot(x - 3, top - 2, L); fill(x - 4, top - 1, 1, 3, L); fill(x + 4, top + 1, 1, 5, D);
      if (away) { fill(x - 2, top + 1, 5, 5, H); fill(x, top + 1, 1, 5, D); return; }
      fill(x - 2, top + 1, 5, 2, H); fill(x - 2, top + 3, 5, 3, 0x14101a, 0.85);
      const glint = (now + x * 0.37) % 12 < 2.5;
      dot(x - 1, top + 3, 0x9affd0, glint ? 1 : 0.5); dot(x + 1, top + 3, 0x9affd0, glint ? 1 : 0.5);
    },
  },
  investigator: {
    // A belted tan trench coat with turned-up lapels and a red scarf; a dark-banded fedora
    front(x, y, o) {
      const L = shaded(o.shirt, 1.2), D = shaded(o.shirt, 0.7);
      fill(x - 1, y, 3, 1, 0xa83a30); dot(x, y + 1, 0xa83a30);
      dot(x - 2, y, L); dot(x + 2, y, L); dot(x - 1, y + 1, L); dot(x + 1, y + 1, L);
      fill(x - 3, y + 2, 1, 3, D); fill(x, y + 2, 1, 2, D, 0.8); dot(x - 1, y + 3, PALETTE.brass);
      fill(x - 3, y + 4, 7, 1, D); dot(x, y + 4, PALETTE.brass);
    },
    back(x, y, o) { fill(x - 3, y, 7, 1, shaded(o.shirt, 1.2)); fill(x, y + 1, 1, 4, shaded(o.shirt, 0.75)); fill(x - 3, y + 4, 7, 1, shaded(o.shirt, 0.7)); },
    head(x, top, look, away) {
      const F = 0x5a5448, FL = 0x7a7468, band = 0x2a2420;
      fill(x - 2, top - 3, 5, 3, F); fill(x - 2, top - 3, 5, 1, FL); dot(x, top - 3, shaded(F, 0.75)); fill(x - 2, top - 1, 5, 1, band);
      fill(x - 5, top, 11, 1, F); fill(x - 5, top, 11, 1, FL, 0.0); dot(x - 5, top, FL); dot(x + 5, top, shaded(F, 0.7));
      if (!away) fill(x - 3, top + 1, 7, 1, shaded(F, 0.5), 0.5);
    },
  },
  deepone: {
    // Green scaly skin with a pale belly and a pattern of scales; a fish head: crest fin, big yellow eyes, wide mouth and gill slits
    front(x, y, o) {
      const D = shaded(o.shirt, 0.72), L = shaded(o.shirt, 1.25);
      fill(x - 1, y + 1, 3, 4, 0xc8dcc0); fill(x - 1, y + 1, 1, 4, 0xe0efd8);
      for (const [dx, dy] of [[-3, 0], [-2, 2], [-3, 3], [2, 1], [3, 3], [2, 5], [-2, 5]]) dot(x + dx, y + dy, D);
      for (const [dx, dy] of [[-3, 1], [3, 0], [3, 4], [-3, 5]]) dot(x + dx, y + dy, L);
      for (const dy of [2, 4]) dot(x, y + dy, 0xa0bca0);
    },
    back(x, y, o) {
      const D = shaded(o.shirt, 0.72);
      for (let k = 0; k < 6; k++) dot(x, y + k, k % 2 ? D : 0x3a7a8a);
      for (const [dx, dy] of [[-3, 1], [2, 2], [-2, 4], [3, 5]]) dot(x + dx, y + dy, D);
    },
    head(x, top, look, away, o) {
      const G = o.shirt, D = shaded(G, 0.7), L = shaded(G, 1.3), fin = 0x3a7a8a;
      fill(x - 3, top - 1, 7, 7, G); fill(x - 2, top - 2, 5, 1, G); fill(x - 3, top, 1, 6, L); fill(x + 3, top, 1, 6, D);
      for (let k = 0; k < 4; k++) dot(x - 1 + k % 3, top - 3 + (k === 1 ? 0 : 1), fin);
      fill(x - 5, top + 2, 2, 3, fin); fill(x + 4, top + 2, 2, 3, fin); dot(x - 5, top + 3, shaded(fin, 1.3)); dot(x + 5, top + 3, shaded(fin, 0.7));
      if (away) { for (const dx of [-1, 1]) dot(x + dx, top + 1, D); return; }
      const blink = (now + x * 0.41) % 7 < 0.2;
      for (const ex of [-2, 2]) { fill(x + ex - 1, top + 1, 2, 3, blink ? G : 0xe8d84a); if (!blink) { dot(x + ex - (ex < 0 ? 0 : 1), top + 2, 0x1f1a1a); dot(x + ex - 1, top + 1, 0xfff6a0); } }
      fill(x - 2, top + 5, 5, 1, 0x3a2a2a); dot(x - 2, top + 4, D); dot(x + 2, top + 4, D);
      dot(x - 3, top + 4, D); dot(x + 3, top + 4, D);
    },
  },
  professor: {
    // A tweed jacket flecked with lighter and darker threads, leather elbow patches, a bow tie; grey hair and round glasses that glint now and then
    front(x, y, o) {
      const L = shaded(o.shirt, 1.3), D = shaded(o.shirt, 0.65);
      fill(x - 1, y, 3, 2, 0xeae6dc); fill(x - 1, y, 3, 1, 0x7a1f2a); dot(x, y, 0x5a1018);
      for (const [dx, dy] of [[-3, 1], [-2, 3], [-3, 5], [2, 2], [3, 4], [2, 5], [-1, 4]]) dot(x + dx, y + dy, L);
      for (const [dx, dy] of [[-2, 2], [3, 1], [-3, 4], [2, 3], [1, 5]]) dot(x + dx, y + dy, D);
      fill(x - 3, y + 1, 1, 4, D, 0.0); dot(x - 1, y + 2, 0x4a3020); dot(x - 1, y + 4, 0x4a3020);
    },
    back(x, y, o) { const L = shaded(o.shirt, 1.3), D = shaded(o.shirt, 0.65); for (const [dx, dy] of [[-3, 0], [-1, 2], [2, 1], [0, 4], [3, 3], [-2, 5]]) dot(x + dx, y + dy, L); for (const [dx, dy] of [[-2, 1], [1, 0], [-3, 3], [2, 5], [0, 3]]) dot(x + dx, y + dy, D); },
    head(x, top, look, away) {
      const H = 0xcfcfd4, HD = 0x9a9aa4;
      fill(x - 3, top - 1, 7, 2, H); fill(x - 4, top, 1, 3, H); fill(x + 4, top, 1, 3, H); dot(x - 2, top - 2, H); dot(x + 1, top - 2, H); dot(x + 2, top - 2, HD);
      if (away) return;
      const G = 0x3a2a22, shine = (now + x * 0.29) % 15 < 0.6;
      for (const ex of [-2, 2]) { dot(x + ex - 1, top + 3, G); dot(x + ex + 1, top + 3, G); dot(x + ex, top + 2, G); dot(x + ex, top + 4, G); }
      dot(x, top + 3, G); if (shine) dot(x - 3, top + 2, WHITE);
      fill(x - 1, top + 5, 3, 1, H, 0.9);
    },
  },
  divingsuit: {
    // Heavy olive canvas with a riveted brass breastplate and an air hose at the hip; a round brass helmet with a glass porthole that glints, bubbles now and then
    front(x, y, o) {
      const B = 0xc8a050, BL = 0xe8c878, BD = 0x8a6a2a;
      fill(x - 2, y, 5, 4, B); fill(x - 2, y, 5, 1, BL); fill(x - 2, y + 3, 5, 1, BD); fill(x - 3, y + 1, 1, 2, B); fill(x + 3, y + 1, 1, 2, BD);
      dot(x - 1, y + 1, BD); dot(x + 1, y + 1, BD); dot(x - 1, y + 2, BL); dot(x + 1, y + 2, BL);
      fill(x - 3, y + 4, 7, 1, 0x3a2e1a); dot(x + 3, y + 5, BD); dot(x + 3, y + 4, B);
    },
    back(x, y, o) {
      const B = 0xc8a050, BD = 0x8a6a2a;
      fill(x - 2, y, 5, 4, B); fill(x - 2, y + 3, 5, 1, BD); dot(x, y + 1, BD); fill(x - 3, y + 4, 7, 1, 0x3a2e1a);
      fill(x + 1, y - 1, 1, 3, 0x2a2420); dot(x + 1, y + 2, B);
    },
    head(x, top, look, away) {
      const B = 0xc8a050, BL = 0xe8c878, BD = 0x8a6a2a;
      disc(x, top + 2.5, 4.6, BD); disc(x, top + 2.2, 4.1, B); fill(x - 3, top - 1, 3, 1, BL); dot(x - 4, top + 1, BL);
      fill(x - 3, top + 6, 7, 1, BD); dot(x, top - 3, BD);
      if (away) { fill(x - 1, top + 2, 3, 1, BD); return; }
      fill(x - 2, top + 1, 5, 4, 0x2a5a58); fill(x - 2, top + 1, 5, 1, BD); fill(x - 3, top + 2, 1, 2, BD); fill(x + 3, top + 2, 1, 2, BD);
      dot(x - 1, top + 2, 0x9affd0, 0.9); dot(x + 1, top + 3, 0x1a3a38);
      if ((now + x * 0.33) % 17 < 2) dot(x + 5, top - 1 - Math.floor(((now + x * 0.33) % 17) * 1.5), 0xcfffe8, 0.8);
    },
  },
  fleecevest: {
    // A navy fleece vest zipped over a pale blue shirt, its collar showing; a small mint coin badge on the chest
    front(x, y, o) {
      const L = shaded(o.shirt, 1.35), D = shaded(o.shirt, 0.7);
      fill(x - 1, y, 3, 1, 0xdce8f4); dot(x, y, 0xb8c8dc);
      fill(x, y + 1, 1, 5, D); dot(x, y + 1, 0xc4c9d0);
      dot(x - 3, y, L); dot(x + 3, y, L); dot(x - 2, y + 2, 0x5ae8a0);
      fill(x - 3, y + 5, 7, 1, D, 0.6);
    },
    back(x, y, o) { const L = shaded(o.shirt, 1.35); fill(x - 1, y, 3, 1, 0xdce8f4); dot(x - 3, y, L); dot(x + 3, y, L); fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.7), 0.6); },
  },
  bullparty: {
    // A shiny green party blazer with dark lapels over a black tee, a gold chain; sequins catch the light now and then; gold-rimmed green shades
    front(x, y, o) {
      const D = shaded(o.shirt, 0.6), L = shaded(o.shirt, 1.35);
      fill(x - 1, y, 3, 6, 0x1a1c20); dot(x - 2, y, D); dot(x + 2, y, D); dot(x - 2, y + 1, D); dot(x + 2, y + 1, D);
      dot(x - 1, y + 2, 0xf2c030); dot(x, y + 3, 0xf2c030); dot(x + 1, y + 2, 0xf2c030);
      dot(x - 3, y + 1, L);
      const glint = Math.floor(now * 2 + x) % 7;
      if (glint < 3) dot(x + (glint === 0 ? -3 : glint === 1 ? 3 : -2), y + 2 + glint, WHITE, 0.8);
    },
    back(x, y, o) { dot(x - 3, y + 1, shaded(o.shirt, 1.35)); fill(x - 1, y, 3, 1, shaded(o.shirt, 0.6)); if (Math.floor(now * 2 + x) % 7 === 0) dot(x + 2, y + 3, WHITE, 0.8); },
    head(x, top, look, away) {
      if (away) { fill(x - 3, top + 2, 7, 1, 0xd8b04a); return; }
      fill(x - 3, top + 2, 7, 1, 0xd8b04a); fill(x - 3, top + 3, 3, 1, 0x2ab868); fill(x + 1, top + 3, 3, 1, 0x2ab868); dot(x - 3, top + 3, 0x9affc0);
    },
  },
  blanketcape: {
    // A grey hoodie with its hood up, a red plaid blanket round the shoulders like a cape; a breath of cold steam now and then
    front(x, y, o) {
      const B = 0x8a2a2a, BL = 0xa83a36, line_ = 0x3a3a4a;
      fill(x - 3, y, 2, 6, B); fill(x + 2, y, 2, 6, B); dot(x - 3, y, BL); dot(x + 3, y, BL);
      dot(x - 3, y + 3, line_); dot(x + 3, y + 3, line_); fill(x - 1, y, 3, 1, shaded(o.shirt, 0.75));
      dot(x, y + 3, shaded(o.shirt, 0.7));
    },
    back(x, y, o) { const B = 0x8a2a2a; fill(x - 3, y, 7, 6, B); fill(x - 3, y, 7, 1, 0xa83a36); for (const k of [-2, 1]) fill(x + k, y, 1, 6, 0x3a3a4a, 0.55); fill(x - 3, y + 3, 7, 1, 0x3a3a4a, 0.5); },
    head(x, top, look, away, o) {
      const H = o.shirt, D = shaded(H, 0.7);
      fill(x - 4, top - 1, 9, 2, H); fill(x - 4, top + 1, 1, 6, H); fill(x + 4, top + 1, 1, 6, D); dot(x - 3, top - 1, shaded(H, 1.25));
      if (away) return fill(x - 3, top + 1, 7, 6, H);
      const t = (now + x * 0.37) % 11;
      if (t < 1.4) { dot(x + 5, top + 5 - Math.floor(t * 2), WHITE, 0.6 - t * 0.3); dot(x + 6, top + 4 - Math.floor(t * 2), WHITE, 0.4 - t * 0.2); }
    },
  },
  denhoodie: {
    // A black hoodie, hood down, a pair of big headphones round the neck and drawstrings that glow through the colours
    front(x, y, o) {
      const D = shaded(o.shirt, 0.7), cup = 0x3a3e48, glow = rainbow((now * 0.2 + x * 0.01) % 1);
      fill(x - 2, y, 5, 1, 0x2a2c34); dot(x - 3, y, cup); dot(x + 3, y, cup); dot(x - 3, y + 1, glow, 0.8); dot(x + 3, y + 1, glow, 0.8);
      fill(x - 1, y + 1, 1, 3, glow); fill(x + 1, y + 1, 1, 3, glow); dot(x - 1, y + 4, WHITE, 0.6); dot(x + 1, y + 4, WHITE, 0.6);
      fill(x - 2, y + 4, 5, 2, D, 0.7);
    },
    back(x, y, o) { fill(x - 2, y, 5, 2, shaded(o.shirt, 1.25)); fill(x - 3, y, 7, 1, 0x2a2c34); },
  },
  homejersey: {
    // The home shirt (brand nod: PSG): navy with a red vertical stripe edged in white, a white collar and a speck of a crest on the chest; a sleeve stripe catches the light now and then
    front(x, y, o) {
      fill(x - 1, y, 1, 6, WHITE); fill(x, y, 1, 6, PSG_RED); fill(x + 1, y, 1, 6, WHITE);
      fill(x - 1, y, 3, 1, WHITE); dot(x, y, 0xe8ecf2);
      dot(x + 2, y + 1, PSG_RED); dot(x - 3, y, PSG_NAVY_LIGHT); dot(x + 3, y + 5, PSG_NAVY_DARK);
      if ((now + x * 0.29) % 17 < 0.4) dot(x - 3, y + 2, WHITE, 0.9);
    },
    back(x, y, o) {
      fill(x - 1, y, 1, 6, WHITE); fill(x, y, 1, 6, PSG_RED); fill(x + 1, y, 1, 6, WHITE);
      fill(x - 1, y, 3, 1, 0xe8ecf2); dot(x - 3, y, PSG_NAVY_LIGHT);
    },
  },
  /** The Lakers home shirt: purple with a chest band in gold and a number block on the chest. */
  lakersjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.lakers, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.lakers, x, y); },
  },
  /** The Celtics home shirt: green with pinstripes in white and a number block on the chest. */
  celticsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.celtics, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.celtics, x, y); },
  },
  /** The Warriors home shirt: blue with trim down the sides in yellow and a number block on the chest. */
  warriorsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.warriors, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.warriors, x, y); },
  },
  /** The Knicks home shirt: blue with a chest band in orange and a number block on the chest. */
  knicksjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.knicks, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.knicks, x, y); },
  },
  /** The Bulls home shirt: red with a centre stripe in black and a number block on the chest. */
  bullsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.bulls, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.bulls, x, y); },
  },
  /** The Heat home shirt: red with trim down the sides in orange and a number block on the chest. */
  heatjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.heat, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.heat, x, y); },
  },
  /** The Spurs home shirt: black with pinstripes in silver and a number block on the chest. */
  spursjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.spurs, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.spurs, x, y); },
  },
  /** The Thunder home shirt: blue with a centre stripe in orange and a number block on the chest. */
  thunderjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.thunder, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.thunder, x, y); },
  },
  /** The Timberwolves home shirt: navy with a chest band in green and a number block on the chest. */
  wolvesjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.wolves, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.wolves, x, y); },
  },
  /** The Nuggets home shirt: navy with trim down the sides in gold and a number block on the chest. */
  nuggetsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.nuggets, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.nuggets, x, y); },
  },
  /** The Hawks home shirt: red with a centre stripe in volt and a number block on the chest. */
  hawksjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.hawks, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.hawks, x, y); },
  },
  /** The Nets home shirt: black with pinstripes in white and a number block on the chest. */
  netsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.nets, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.nets, x, y); },
  },
  /** The Hornets home shirt: teal with a chest band in purple and a number block on the chest. */
  hornetsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.hornets, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.hornets, x, y); },
  },
  /** The Cavaliers home shirt: wine with trim down the sides in gold and a number block on the chest. */
  cavsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.cavs, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.cavs, x, y); },
  },
  /** The Mavericks home shirt: blue with pinstripes in silver and a number block on the chest. */
  mavsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.mavs, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.mavs, x, y); },
  },
  /** The Pistons home shirt: blue with a chest band in red and a number block on the chest. */
  pistonsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.pistons, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.pistons, x, y); },
  },
  /** The Rockets home shirt: red with a centre stripe in silver and a number block on the chest. */
  rocketsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.rockets, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.rockets, x, y); },
  },
  /** The Pacers home shirt: navy with trim down the sides in gold and a number block on the chest. */
  pacersjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.pacers, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.pacers, x, y); },
  },
  /** The Clippers home shirt: red with pinstripes in blue and a number block on the chest. */
  clippersjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.clippers, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.clippers, x, y); },
  },
  /** The Grizzlies home shirt: navy with a chest band in blue and a number block on the chest. */
  grizzliesjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.grizzlies, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.grizzlies, x, y); },
  },
  /** The Bucks home shirt: green with a centre stripe in cream and a number block on the chest. */
  bucksjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.bucks, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.bucks, x, y); },
  },
  /** The Pelicans home shirt: navy with trim down the sides in red and a number block on the chest. */
  pelicansjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.pelicans, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.pelicans, x, y); },
  },
  /** The Magic home shirt: blue with a chest band in black and a number block on the chest. */
  magicjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.magic, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.magic, x, y); },
  },
  /** The 76ers home shirt: blue with pinstripes in red and a number block on the chest. */
  sixersjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.sixers, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.sixers, x, y); },
  },
  /** The Suns home shirt: purple with a centre stripe in orange and a number block on the chest. */
  sunsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.suns, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.suns, x, y); },
  },
  /** The Trail Blazers home shirt: red with trim down the sides in black and a number block on the chest. */
  blazersjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.blazers, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.blazers, x, y); },
  },
  /** The Kings home shirt: purple with a chest band in silver and a number block on the chest. */
  kingsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.kings, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.kings, x, y); },
  },
  /** The Raptors home shirt: red with pinstripes in black and a number block on the chest. */
  raptorsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.raptors, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.raptors, x, y); },
  },
  /** The Jazz home shirt: navy with a centre stripe in yellow and a number block on the chest. */
  jazzjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.jazz, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.jazz, x, y); },
  },
  /** The Wizards home shirt: navy with trim down the sides in red and a number block on the chest. */
  wizardsjersey: {
    front(x, y) { nbaJerseyFront(NBA_TEAMS.wizards, x, y); },
    back(x, y) { nbaJerseyBack(NBA_TEAMS.wizards, x, y); },
  },
  marvel2starlord: {
    // Star-Lord (brand nod: Marvel): an open red-brown leather jacket over a grey tee, orange headphones round the neck; now and then his
    // masked helmet flicks on for a moment, its red eyes glowing
    front(x, y, o, look) {
      fill(x - 1, y, 3, 6, 0x6a6e78); fill(x - 2, y, 1, 4, 0xa84a3a); fill(x + 2, y, 1, 4, 0x6a1a18);
      dot(x - 3, y + 2, 0xa84a3a); dot(x - 3, y + 4, 0x6a1a18); fill(x - 1, y, 3, 1, M2_ORANGE); dot(x - 2, y, 0x8a8e98); dot(x + 2, y, 0x8a8e98);
    },
    back(x, y, o) { fill(x - 3, y, 7, 1, 0xa84a3a); fill(x, y + 1, 1, 5, 0x6a1a18, 0.6); },
    head(x, top, look, away) {
      if (every(look, 17, 1.6, 6) !== null) {
        fill(x - 3, top, 7, 7, 0x3a3440); fill(x - 3, top, 7, 1, 0x5a5460); fill(x - 3, top + 6, 7, 1, 0x8a6a3a);
        if (!away) { fill(x - 3, top + 3, 3, 2, 0xe03030); fill(x + 1, top + 3, 3, 2, 0xe03030); if (isNight) glowLate(() => { fill(x - 3, top + 3, 3, 2, 0xff5050); fill(x + 1, top + 3, 3, 2, 0xff5050); }); }
        return;
      }
      dot(x - 4, top + 3, M2_ORANGE); dot(x + 4, top + 3, M2_ORANGE);
    },
  },
  marvel2strange: {
    // Doctor Strange (brand nod: Marvel): a deep blue tunic crossed by a gold-brown sash, the Eye of Agamotto at the throat and the red
    // Cloak of Levitation's high collar at the shoulders; dark hair grey at the temples and a goatee; the Eye glows green now and then
    front(x, y, o, look) {
      fill(x - 3, y, 1, 6, 0xb02a2a); fill(x + 3, y, 1, 6, 0x7a1a1a); dot(x - 3, y - 1, 0xd84a42); dot(x + 3, y - 1, 0xb02a2a);
      line(x - 2, y + 1, x + 2, y + 5, 0xa87a3a); fill(x - 2, y + 4, 5, 1, 0x8a5a2a);
      const glow = every(look, 21, 1.5, 7) !== null;
      dot(x, y, M2_GOLD); if (glow) { dot(x, y, 0x3ae87a); glowAt(x, y, 2, 2, 0x3ae87a, 0.2); }
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xb02a2a); fill(x - 3, y, 7, 1, 0xd84a42); fill(x - 1, y + 1, 1, 5, 0x7a1a1a, 0.6); fill(x + 2, y + 1, 1, 5, 0x7a1a1a, 0.6); },
    head(x, top, look, away) {
      const hair = 0x2a2220, grey = 0xc8c8cc;
      fill(x - 3, top, 7, 2, hair); fill(x - 2, top - 1, 4, 1, hair); dot(x - 3, top + 2, grey); dot(x + 3, top + 2, grey); dot(x - 3, top + 1, grey);
      dot(x - 4, top + 3, 0xb02a2a); dot(x + 4, top + 3, 0x7a1a1a); dot(x - 4, top + 4, 0xb02a2a); dot(x + 4, top + 4, 0x7a1a1a);
      if (away) { fill(x - 2, top + 2, 5, 3, hair); dot(x - 3, top + 3, grey); dot(x + 3, top + 3, grey); return; }
      fill(x - 1, top + 6, 3, 1, hair); dot(x - 2, top + 5, hair, 0.5); dot(x + 2, top + 5, hair, 0.5);
    },
  },
  marvel2panther: {
    // Black Panther (brand nod: Marvel): the black vibranium suit with its silver claw necklace and silver lines, the full mask with pointed
    // ears and white eye slits; now and then violet kinetic light runs down the suit's lines
    front(x, y, o, look) {
      for (let k = -3; k <= 3; k++) dot(x + k, y + (Math.abs(k) === 3 ? 0 : 1), M2_SILVER);
      dot(x, y + 2, M2_SILVER);
      const pulse = every(look, 19, 1.2, 5), c = pulse !== null ? M2_VIBE : 0x4a4a58;
      fill(x - 2, y + 2, 1, 4, c); fill(x + 2, y + 2, 1, 4, c); fill(x - 1, y + 4, 3, 1, c, 0.7);
      if (pulse !== null) glowAt(x, y + 3, 3, 3, M2_VIBE, 0.15);
    },
    back(x, y, o) { fill(x - 3, y, 7, 1, M2_SILVER, 0.7); fill(x, y + 1, 1, 5, 0x4a4a58); },
    head(x, top, look, away) {
      fill(x - 3, top, 7, 7, M2_PANTHER); fill(x - 3, top, 1, 7, 0x2a2a34); dot(x - 3, top - 1, M2_PANTHER); dot(x + 3, top - 1, M2_PANTHER);
      if (away) { fill(x, top, 1, 6, 0x4a4a58); return; }
      fill(x, top, 1, 3, 0x6a6a78); dot(x - 1, top + 2, 0x6a6a78); dot(x + 1, top + 2, 0x6a6a78);
      fill(x - 2, top + 4, 2, 1, WHITE); fill(x + 1, top + 4, 2, 1, WHITE);
      dot(x, top + 6, 0x6a6a78);
    },
  },
  marvel2loki: {
    // Loki (brand nod: Marvel): a green tunic under black leather with gold plates at the chest and a green cape behind; black hair to the
    // shoulders and the gold helmet with its long curving horns; the horns catch the light now and then
    front(x, y, o, look) {
      fill(x - 3, y, 2, 6, 0x1a1a22); fill(x + 2, y, 2, 6, 0x1a1a22); dot(x - 3, y, M2_LOKI); dot(x + 3, y, M2_LOKI);
      fill(x - 1, y + 1, 3, 1, M2_GOLD); fill(x - 1, y + 3, 3, 1, M2_GOLD); dot(x - 1, y + 1, M2_GOLD_LIGHT); dot(x - 2, y + 2, M2_GOLD_DARK); dot(x + 2, y + 2, M2_GOLD_DARK);
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, M2_LOKI); fill(x - 3, y, 7, 1, M2_LOKI_LIGHT); fill(x - 1, y + 1, 1, 5, M2_LOKI_DARK); fill(x + 2, y + 1, 1, 5, M2_LOKI_DARK); },
    head(x, top, look, away) {
      const hair = 0x14141a;
      fill(x - 3, top + 2, 1, 6, hair); fill(x + 3, top + 2, 1, 6, hair); dot(x - 4, top + 6, hair); dot(x + 4, top + 6, hair);
      if (away) fill(x - 2, top + 2, 5, 6, hair);
      fill(x - 3, top - 1, 7, 3, M2_GOLD); fill(x - 3, top - 1, 7, 1, M2_GOLD_LIGHT); fill(x - 2, top - 2, 5, 1, M2_GOLD);
      if (!away) { dot(x - 3, top + 2, M2_GOLD); dot(x + 3, top + 2, M2_GOLD_DARK); dot(x, top + 2, M2_GOLD); }
      const glint = every(look, 13, 0.5, 8) !== null;
      for (const side of [-1, 1]) for (const [dx, dy] of [[3, -2], [4, -2], [4, -3], [5, -3], [5, -4], [6, -5], [6, -6], [6, -7]]) dot(x + side * dx, top + dy, side < 0 && glint ? WHITE : side < 0 ? M2_GOLD_LIGHT : M2_GOLD_DARK);
    },
  },
  ironmanmk3: {
    // Iron Man, red and gold (brand nod: Marvel): the classic armour, gold plates at the waist and shoulders, the arc reactor glowing in
    // the chest; the helmet with its gold faceplate, which flips up now and then
    front(x, y, o, look) {
      dot(x - 3, y, IM_GOLD); dot(x + 3, y, IM_GOLD_DARK); fill(x - 3, y + 4, 7, 2, IM_GOLD); fill(x - 3, y + 4, 7, 1, IM_GOLD_LIGHT);
      disc(x, y + 2, 1, IM_ARC); dot(x, y + 2, IM_ARC_WHITE);
      if (isNight) glowLate(() => { dot(x, y + 2, IM_ARC_WHITE); glowAt(x, y + 2, 2, 2, IM_ARC, 0.25); });
    },
    back(x, y, o) { fill(x - 3, y + 4, 7, 2, IM_GOLD_DARK); dot(x - 2, y + 1, IM_RED_LIGHT); dot(x + 2, y + 1, IM_RED_LIGHT); },
    head(x, top, look, away) {
      if (away) { fill(x - 3, top, 7, 7, IM_RED); fill(x - 2, top, 5, 1, IM_RED_LIGHT); return; }
      if (every(look, 19, 2, 6) !== null) { fill(x - 3, top, 7, 2, IM_RED); fill(x - 3, top + 2, 1, 5, IM_RED); fill(x + 3, top + 2, 1, 5, IM_RED_DARK); fill(x - 2, top - 1, 5, 1, IM_GOLD); return; }
      imFace(x - 3, top);
    },
  },
  ironmanmk2: {
    // Iron Man, silver Mark II (brand nod: Marvel): bare brushed-steel armour with dark panel lines and rivets, the reactor glowing in the
    // chest, and the silver helmet with its slit eyes
    front(x, y, o, look) {
      fill(x - 3, y + 3, 7, 1, IM_STEEL_DARK); fill(x, y + 4, 1, 2, IM_STEEL_DARK); dot(x - 2, y + 5, WHITE, 0.6); dot(x + 2, y + 1, IM_STEEL_DARK);
      disc(x, y + 1, 1, IM_ARC); dot(x, y + 1, IM_ARC_WHITE);
      if (isNight) glowLate(() => { dot(x, y + 1, IM_ARC_WHITE); glowAt(x, y + 1, 2, 2, IM_ARC, 0.25); });
    },
    back(x, y, o) { fill(x - 3, y + 3, 7, 1, IM_STEEL_DARK); fill(x, y, 1, 6, IM_STEEL_DARK, 0.6); },
    head(x, top, look, away) {
      if (away) { fill(x - 3, top, 7, 7, IM_STEEL_LIGHT); fill(x, top, 1, 7, IM_STEEL, 0.7); return; }
      imFace(x - 3, top, [IM_STEEL_LIGHT, 0xc8ccd4]);
    },
  },
  ironmanwarmachine: {
    // War Machine (brand nod: Marvel): heavy gunmetal armour with black plates, the reactor in the chest, and the rotary cannon on his
    // right shoulder, which swings round now and then; a gunmetal helmet with white eyes
    front(x, y, o, look) {
      fill(x - 3, y, 2, 2, 0x2a2c34); fill(x + 2, y, 2, 2, 0x2a2c34); fill(x - 3, y + 4, 7, 2, 0x2a2c34); dot(x - 1, y + 4, 0x6e727c); dot(x + 1, y + 4, 0x6e727c);
      disc(x, y + 2, 1, IM_ARC); dot(x, y + 2, IM_ARC_WHITE);
      const swing = every(look, 15, 1.5, 4) !== null;
      fill(x + 3, y - 2, 2, 2, 0x2a2c34); fill(x + (swing ? 2 : 4), y - 3, swing ? 4 : 2, 1, 0x6e727c); dot(x + (swing ? 6 : 5), y - 3, 0x1a1a1f);
    },
    back(x, y, o) { fill(x - 3, y, 7, 2, 0x2a2c34); fill(x + 3, y - 2, 2, 2, 0x2a2c34); fill(x - 1, y + 2, 3, 3, 0x3a3e48); },
    head(x, top, look, away) {
      if (away) { fill(x - 3, top, 7, 7, 0x4a4e58); return; }
      imFace(x - 3, top, [0x4a4e58, 0x6e727c]);
    },
  },
  batcavebatsuit: {
    // Batman (brand nod: DC): the grey suit with the black bat across the chest and the yellow utility belt, the cape at his shoulders; the
    // black cowl with its pointed ears and white eye slits, his jaw showing below
    front(x, y, o, look) {
      dot(x - 3, y, BC_BLACK); dot(x + 3, y, BC_BLACK); fill(x - 3, y + 1, 1, 4, BC_BLACK, 0.6); fill(x + 3, y + 1, 1, 4, BC_BLACK, 0.6);
      for (const [dx, dy] of [[-2, 1], [-1, 1], [0, 1], [1, 1], [2, 1], [-1, 2], [0, 2], [1, 2], [0, 0], [-2, 0], [2, 0]]) dot(x + dx, y + dy, BC_BLACK);
      fill(x - 3, y + 5, 7, 1, BC_YELLOW); dot(x, y + 5, BC_YELLOW_DARK);
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, BC_BLACK); fill(x - 3, y, 7, 1, 0x2e3038); fill(x - 1, y + 2, 1, 4, 0x2e3038, 0.6); fill(x + 2, y + 2, 1, 4, 0x2e3038, 0.6); },
    head(x, top, look, away) {
      fill(x - 3, top, 7, 4, BC_BLACK); fill(x - 3, top + 4, 1, 2, BC_BLACK); fill(x + 3, top + 4, 1, 2, BC_BLACK);
      dot(x - 2, top - 1, BC_BLACK); dot(x - 2, top - 2, BC_BLACK); dot(x + 2, top - 1, BC_BLACK); dot(x + 2, top - 2, BC_BLACK);
      if (away) { fill(x - 3, top, 7, 7, BC_BLACK); return; }
      fill(x - 2, top + 3, 2, 1, WHITE); fill(x + 1, top + 3, 2, 1, WHITE); dot(x - 3, top + 1, 0x2e3038);
    },
  },
  batcaverobin: {
    // Robin (brand nod: DC): a red tunic with the yellow R badge, a yellow belt and the yellow cape at his shoulders, green sleeves; a black
    // domino mask round white eyes
    front(x, y, o, look) {
      dot(x - 3, y, BC_YELLOW); dot(x + 3, y, BC_YELLOW); disc(x - 1, y + 2, 1, BC_YELLOW); dot(x - 1, y + 2, BC_BLACK);
      fill(x - 3, y + 5, 7, 1, BC_YELLOW); dot(x, y + 5, BC_YELLOW_DARK);
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, BC_YELLOW); fill(x - 3, y, 7, 1, 0xf8e470); fill(x, y + 1, 1, 5, BC_YELLOW_DARK, 0.6); },
    head(x, top, look, away) {
      if (away) return;
      fill(x - 3, top + 3, 7, 2, BC_BLACK); dot(x - 2, top + 4, WHITE); dot(x + 2, top + 4, WHITE);
    },
  },
  batcavealfred: {
    // Alfred (brand nod: DC): a butler's black tailcoat over a crisp white shirt front and a black bow tie, a white cuff of a napkin over
    // his arm; a bald crown with a grey fringe and a neat grey moustache
    front(x, y, o, look) {
      fill(x - 1, y, 3, 6, 0xf2f2ee); fill(x - 1, y, 3, 1, BC_BLACK); dot(x, y + 1, BC_BLACK, 0.7); dot(x, y + 3, 0x9a9aa4); dot(x, y + 5, 0x9a9aa4);
      fill(x + 3, y + 3, 1, 3, 0xf2f2ee);
    },
    back(x, y, o) { fill(x - 1, y + 3, 1, 3, 0x0e0e12); fill(x + 1, y + 3, 1, 3, 0x0e0e12); },
    head(x, top, look, away) {
      const grey = 0xc8c8cc;
      fill(x - 2, top, 5, 2, look.skin); fill(x - 3, top + 1, 1, 3, grey); fill(x + 3, top + 1, 1, 3, grey);
      if (away) { fill(x - 2, top + 2, 5, 3, grey); return; }
      fill(x - 1, top + 5, 3, 1, grey); dot(x - 2, top + 5, grey, 0.6); dot(x + 2, top + 5, grey, 0.6);
    },
  },
  batcavecatwoman: {
    // Catwoman (brand nod: DC): a glossy black catsuit zipped to the throat, a whip coiled at the hip; a black cowl with pointed cat ears
    // and goggles, red lips; a glint runs over the suit now and then
    front(x, y, o, look) {
      fill(x, y, 1, 5, 0x6e727c); dot(x, y, 0xa8acb6);
      dot(x - 3, y + 4, 0x5a3a24); dot(x - 3, y + 5, 0x5a3a24); dot(x - 2, y + 5, 0x5a3a24);
      if (every(look, 13, 0.6, 3) !== null) { dot(x - 2, y + 1, WHITE, 0.8); dot(x + 2, y + 3, WHITE, 0.6); }
      else dot(x - 2, y + 1, 0x3a3a48);
    },
    back(x, y, o) { fill(x, y, 1, 6, 0x3a3a48); },
    head(x, top, look, away) {
      fill(x - 3, top, 7, 4, BC_BLACK); fill(x - 3, top + 4, 1, 3, BC_BLACK); fill(x + 3, top + 4, 1, 3, BC_BLACK);
      dot(x - 3, top - 1, BC_BLACK); dot(x - 3, top - 2, BC_BLACK); dot(x - 2, top - 1, BC_BLACK); dot(x + 3, top - 1, BC_BLACK); dot(x + 3, top - 2, BC_BLACK); dot(x + 2, top - 1, BC_BLACK);
      if (away) { fill(x - 3, top, 7, 7, BC_BLACK); return; }
      fill(x - 2, top + 3, 2, 1, 0x6a7a8a); fill(x + 1, top + 3, 2, 1, 0x6a7a8a); dot(x - 2, top + 3, 0xa8c0d0);
      fill(x - 1, top + 6, 3, 1, 0xc8283a);
    },
  },
  dcmixsuperman: {
    // Superman (brand nod: DC): the blue suit with the S shield on the chest, a red belt with a gold buckle and the red cape at his
    // shoulders; a black kiss curl on his forehead, and the cape stirs now and then as if in a breeze
    front(x, y, o, look) {
      dcS(x - 2, y, 1, true); fill(x - 3, y + 5, 7, 1, DC_RED); dot(x, y + 5, DC_YELLOW);
      dot(x - 3, y, DC_RED); dot(x + 3, y, DC_RED);
      if (every(look, 15, 1.2, 4) !== null) { dot(x - 4, y + 1, DC_RED); dot(x - 4, y + 3, DC_RED); }
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, DC_RED); fill(x - 3, y, 7, 1, DC_RED_LIGHT); fill(x, y + 1, 1, 5, DC_RED_DARK, 0.6); dcS(x - 2, y + 1, 0.9, true); },
    head(x, top, look, away) {
      fill(x - 3, top, 7, 2, 0x14141a); dot(x - 3, top + 2, 0x14141a); dot(x + 3, top + 2, 0x14141a);
      if (!away) { dot(x, top + 2, 0x14141a); dot(x - 1, top + 3, 0x14141a, 0.8); }
    },
  },
  dcmixwonder: {
    // Wonder Woman (brand nod: DC): a red bodice with the gold eagle across the chest, a gold belt over a blue skirt with white stars, silver
    // bracelets; long black hair and the gold tiara with its red star; the lasso at her hip glints now and then
    front(x, y, o, look) {
      fill(x - 3, y + 1, 7, 1, DC_GOLD); dot(x, y + 2, DC_GOLD); dot(x - 2, y, DC_GOLD); dot(x + 2, y, DC_GOLD);
      fill(x - 3, y + 4, 7, 1, DC_GOLD); fill(x - 3, y + 5, 7, 1, 0x1a3a8a); dot(x - 2, y + 5, WHITE); dot(x + 1, y + 5, WHITE);
      dot(x + 3, y + 3, every(look, 11, 0.6, 5) !== null ? 0xfff0a0 : DC_GOLD_DARK);
    },
    back(x, y, o) { fill(x - 3, y, 7, 4, 0x14141a); fill(x - 3, y + 4, 7, 1, DC_GOLD); fill(x - 3, y + 5, 7, 1, 0x1a3a8a); },
    head(x, top, look, away) {
      const hair = 0x14141a;
      fill(x - 3, top, 7, 2, hair); fill(x - 3, top + 2, 1, 6, hair); fill(x + 3, top + 2, 1, 6, hair); dot(x - 4, top + 5, hair); dot(x + 4, top + 6, hair);
      if (away) { fill(x - 2, top + 2, 5, 6, hair); return; }
      fill(x - 2, top + 1, 5, 1, DC_GOLD); dot(x, top, DC_GOLD); dot(x, top + 1, DC_RED);
    },
  },
  dcmixflash: {
    // The Flash (brand nod: DC): scarlet from head to toe, a white circle with a yellow bolt on the chest and a yellow lightning belt; the
    // cowl with its little lightning wings, and now and then sparks of the Speed Force crackle round him
    front(x, y, o, look) {
      disc(x, y + 2, 1.5, WHITE); dot(x, y + 1, DC_YELLOW); dot(x - 1, y + 2, DC_YELLOW); dot(x + 1, y + 2, DC_YELLOW); dot(x, y + 3, DC_YELLOW);
      fill(x - 3, y + 5, 7, 1, DC_YELLOW); dot(x - 1, y + 4, DC_YELLOW); dot(x + 2, y + 4, DC_YELLOW);
      if (every(look, 9, 0.5, 6) !== null) { dot(x - 4, y + 1, DC_YELLOW); dot(x + 4, y + 3, DC_YELLOW); dot(x - 5, y + 2, WHITE, 0.8); }
    },
    back(x, y, o) { fill(x - 3, y + 5, 7, 1, DC_YELLOW); dot(x - 1, y + 1, DC_RED_LIGHT); },
    head(x, top, look, away) {
      fill(x - 3, top, 7, 4, DC_RED); fill(x - 3, top + 4, 1, 2, DC_RED); fill(x + 3, top + 4, 1, 2, DC_RED); fill(x - 2, top, 5, 1, DC_RED_LIGHT);
      dot(x - 4, top + 2, DC_YELLOW); dot(x - 5, top + 1, DC_YELLOW); dot(x + 4, top + 2, DC_YELLOW); dot(x + 5, top + 1, DC_YELLOW);
      if (away) { fill(x - 3, top, 7, 7, DC_RED); return; }
      fill(x - 2, top + 3, 2, 1, WHITE); fill(x + 1, top + 3, 2, 1, WHITE);
    },
  },
  dcmixgreenlantern: {
    // Green Lantern (brand nod: DC): black and green with the white Lantern symbol on the chest, a green domino mask over glowing white
    // eyes, and the ring on his hand flaring emerald now and then
    front(x, y, o, look) {
      fill(x - 3, y + 3, 7, 3, 0x1a9a3a); fill(x - 3, y, 1, 3, 0x1a9a3a); fill(x + 3, y, 1, 3, 0x1a9a3a);
      ring(x, y + 1, 1.4, WHITE); fill(x - 1, y - 1, 3, 1, WHITE, 0.0);
      const flare = every(look, 12, 1, 7) !== null;
      dot(x + 4, y + 5, flare ? DC_GREEN_LIGHT : DC_GREEN);
      if (flare) glowAt(x + 4, y + 5, 3, 3, DC_GREEN, 0.3);
      if (isNight) glowLate(() => dot(x + 4, y + 5, DC_GREEN_LIGHT));
    },
    back(x, y, o) { fill(x - 3, y + 3, 7, 3, 0x1a9a3a); },
    head(x, top, look, away) {
      if (away) return;
      fill(x - 3, top + 3, 7, 2, 0x1a9a3a); fill(x - 2, top + 4, 1, 1, WHITE); fill(x + 2, top + 4, 1, 1, WHITE);
      if (isNight) glowLate(() => { dot(x - 2, top + 4, DC_GREEN_LIGHT); dot(x + 2, top + 4, DC_GREEN_LIGHT); });
    },
  },
  pokemontrainer: {
    // A Pokémon trainer's look: a blue jacket with white sleeves and collar left open over a black tee, and the red and
    // white cap with its green mark, brim forward; now and then the cap is tugged down a pixel, ready for a battle
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0x2a2a30); dot(x - 2, y, PKM_WHITE); dot(x + 2, y, PKM_WHITE); dot(x - 2, y + 1, PKM_WHITE); dot(x + 2, y + 1, PKM_WHITE);
      dot(x - 3, y + 1, shaded(o.shirt, 1.3)); fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.7));
      dot(x + 2, y + 3, 0xf8d030);
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, PKM_WHITE); dot(x - 3, y + 1, shaded(o.shirt, 1.3)); fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.7)); },
    head(x, top, look, away) {
      const tug = (now + x * 0.31) % 23 < 1 ? 1 : 0;
      fill(x - 3, top - 1 + tug, 7, 2, PKM_RED); fill(x - 3, top - 1 + tug, 7, 1, PKM_RED_LIGHT);
      if (away) { fill(x - 1, top + 1 + tug, 3, 1, PKM_RED_DARK); dot(x, top + 1 + tug, PKM_WHITE); return; }
      fill(x - 1, top - 1 + tug, 3, 2, PKM_WHITE); dot(x, top + tug, 0x3aa84a);
      fill(x - 4, top + 1 + tug, 9, 1, PKM_RED_DARK);
    },
  },
  pokemonrocket: {
    // Team Rocket's uniform: a white jacket over a black top with the big red R on the chest, a black belt; every 40 s a
    // star twinkles off into the sky over their head, blasting off again
    front(x, y, o) {
      fill(x - 2, y, 5, 1, 0x2a2a30); dot(x - 3, y + 1, WHITE);
      const R = 0xd8202a;
      fill(x - 1, y + 1, 2, 1, R); dot(x - 1, y + 2, R); dot(x + 1, y + 2, R); fill(x - 1, y + 3, 2, 1, R); dot(x - 1, y + 4, R); dot(x + 1, y + 4, R);
      fill(x - 3, y + 5, 7, 1, 0x2a2a30);
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, 0x2a2a30); fill(x - 3, y + 5, 7, 1, 0x2a2a30); },
    head(x, top, look, away) {
      const t = (now + x * 0.29) % 40;
      if (t < 1.2) { const k = Math.floor(t * 5); dot(x + 4 + k, top - 2 - k, WHITE, 1 - t / 1.2); if (k % 2) { dot(x + 3 + k, top - 2 - k, WHITE, 0.5); dot(x + 5 + k, top - 2 - k, WHITE, 0.5); } }
    },
  },
  pokemonnurse: {
    // Nurse Joy: a pink dress under a white apron, pink hair with its two big loops at the sides, and the white nurse's
    // cap with a red cross; now and then she gives a little nod
    front(x, y, o) {
      fill(x - 2, y + 1, 5, 5, PKM_WHITE); dot(x - 2, y, PKM_WHITE); dot(x + 2, y, PKM_WHITE); fill(x - 2, y + 5, 5, 1, PKM_GREY);
      dot(x, y, 0xffffff); dot(x - 3, y + 1, shaded(o.shirt, 1.15));
    },
    back(x, y, o) { fill(x - 3, y + 3, 7, 1, PKM_WHITE); dot(x - 1, y + 2, PKM_WHITE); dot(x + 1, y + 2, PKM_WHITE); dot(x - 2, y, PKM_WHITE); dot(x + 2, y, PKM_WHITE); },
    nods(look) { return (now + look.phase * 7) % 19 < 0.5 ? 1 : 0; },
    head(x, top, look, away) {
      const hair = 0xf27aa8, hairDark = 0xd05a88;
      fill(x - 2, top, 5, 1, hair); fill(x - 3, top + 1, 7, 1, hair); dot(x - 3, top + 2, hair); dot(x + 3, top + 2, hair);
      if (away) fill(x - 3, top + 2, 7, 5, hair);
      for (const s of [-1, 1]) {
        const lx = s < 0 ? x - 5 : x + 4;
        dot(lx, top + 3, hair); dot(lx + 1, top + 3, hair); dot(lx, top + 4, hairDark); dot(lx + (s < 0 ? 0 : 1), top + 5, hair); dot(lx + 1, top + 5, hair); dot(lx + (s < 0 ? 1 : 0), top + 4, hair, 0.4);
      }
      fill(x - 1, top - 3, 3, 1, WHITE); fill(x - 2, top - 2, 5, 2, PKM_WHITE); dot(x + 2, top - 1, PKM_GREY);
      if (!away) { dot(x, top - 3, 0xe8484a); fill(x - 1, top - 2, 3, 1, 0xe8484a); dot(x, top - 1, 0xe8484a); }
    },
  },
  pokemonoak: {
    // Professor Oak: a white lab coat open over a red shirt and its collar, grey swept-back hair; now and then he pats
    // the Pokédex in his breast pocket
    front(x, y, o) {
      fill(x - 1, y, 3, 5, 0xc83a3a); dot(x, y, 0x9a2a2a); dot(x - 1, y, 0xe05a5a);
      fill(x - 2, y + 1, 1, 5, 0xd8d6d0); fill(x + 2, y + 1, 1, 5, 0xd8d6d0);
      const pat = (now + x * 0.27) % 21 < 0.8;
      dot(x - 3, y + 2, pat ? 0xf0603a : PKM_RED); dot(x - 3, y + 1, 0xd8d6d0);
    },
    back(x, y, o) { fill(x, y + 2, 1, 4, 0xd8d6d0); fill(x - 3, y, 7, 1, shaded(o.shirt, 0.92)); },
    head(x, top, look, away) {
      const grey = 0xc8c8cc, greyDark = 0x9a9aa0;
      fill(x - 2, top - 1, 5, 1, grey); fill(x - 3, top, 7, 2, grey); dot(x - 3, top + 2, grey); dot(x + 3, top + 2, grey);
      dot(x - 2, top - 2, grey); dot(x + 1, top - 2, grey); fill(x - 3, top + 1, 7, 1, greyDark, 0.5);
      if (away) fill(x - 3, top + 2, 7, 4, grey);
    },
  },
  loljinx: {
    // Jinx: a black top with pink straps and a belt of bullets, long blue braids down past the shoulders and a
    // blue fringe; now and then a pink spark pops by her head
    front(x, y, o) {
      dot(x - 2, y, 0xf06aa8); dot(x + 2, y, 0xf06aa8); dot(x - 2, y + 1, 0xf06aa8); dot(x + 2, y + 1, 0xf06aa8);
      fill(x - 1, y + 2, 3, 1, 0x3a3a46); fill(x - 3, y + 5, 7, 1, 0x8a6a3a); for (let k = -2; k <= 2; k += 2) dot(x + k, y + 5, 0xd8b04a);
    },
    back(x, y, o) { fill(x - 3, y + 5, 7, 1, 0x8a6a3a); fill(x - 2, y, 1, 4, 0x3a8ae8); fill(x + 2, y, 1, 4, 0x3a8ae8); },
    head(x, top, look, away) {
      const blue = 0x3a8ae8, blueDark = 0x2a5ab8;
      fill(x - 2, top, 5, 1, blue); fill(x - 3, top + 1, 7, 1, blue); dot(x - 3, top + 2, blue); dot(x + 3, top + 2, blue);
      if (away) fill(x - 3, top + 2, 7, 5, blue);
      for (const bx of [x - 4, x + 4]) { fill(bx, top + 2, 1, 10, blue); for (let k = 3; k < 12; k += 2) dot(bx, top + k, blueDark); }
      if (!away) { dot(x - 1, top + 2, blue); dot(x + 1, top + 2, blue); }
      if ((now + x * 0.37) % 13 < 0.4) dot(x + 6, top + 1, 0xf06aa8);
    },
  },
  lolahri: {
    // Ahri: a white top with a red sash and red wrapped skirt, long black hair and two black fox ears tipped with pink
    // inside, which twitch now and then; from behind, the white tip of a tail
    front(x, y, o) {
      fill(x - 3, y, 2, 1, 0xc8364a); fill(x + 2, y, 2, 1, 0xc8364a); fill(x - 1, y, 3, 1, 0xf8f0f4);
      fill(x - 3, y + 4, 7, 1, 0xc8364a); dot(x, y + 4, LOL_GOLD);
    },
    back(x, y, o) { fill(x - 3, y + 4, 7, 1, 0xc8364a); fill(x + 3, y + 2, 2, 4, 0xf8f4f8); dot(x + 4, y + 2, 0xd8d0e0); },
    head(x, top, look, away) {
      const hair = 0x2a1e2a, inner = 0xf0a0b8;
      fill(x - 2, top, 5, 1, hair); fill(x - 3, top + 1, 7, 1, hair); fill(x - 3, top + 2, 1, 6, hair); fill(x + 3, top + 2, 1, 6, hair);
      if (away) fill(x - 3, top + 2, 7, 6, hair);
      const twitch = (now + x * 0.31) % 9 < 0.3 ? 1 : 0;
      fill(x - 3, top - 2 + twitch, 2, 2, hair); dot(x - 3, top - 3 + twitch, hair); fill(x + 2, top - 2, 2, 2, hair); dot(x + 3, top - 3, hair);
      if (!away) { dot(x - 2, top - 1 + twitch, inner); dot(x + 2, top - 1, inner); }
    },
  },
  lolgaren: {
    // Demacian armour: a steel breastplate under a deep blue tabard trimmed in gold, a gold winged emblem on the chest,
    // and the polished plate catching the light now and then
    front(x, y, o) {
      fill(x - 2, y, 5, 6, 0x2e3e6a); fill(x - 2, y, 1, 6, LOL_GOLD); fill(x + 2, y, 1, 6, LOL_GOLD);
      dot(x - 1, y + 2, LOL_GOLD_LIGHT); dot(x, y + 1, LOL_GOLD_LIGHT); dot(x + 1, y + 2, LOL_GOLD_LIGHT);
      dot(x - 3, y, 0xd8dce4); dot(x + 3, y, 0x6e727c);
      if ((now + x * 0.23) % 11 < 0.3) dot(x - 3, y + 1, WHITE);
    },
    back(x, y, o) { fill(x - 2, y, 5, 6, 0x2e3e6a); fill(x - 2, y, 5, 1, LOL_GOLD); dot(x - 3, y, 0xd8dce4); dot(x + 3, y, 0x6e727c); },
  },
  lollux: {
    // Lux: white and gold with a blue panel down the front, blonde hair under a gold and blue tiara; every 18 s a
    // little star of light sparkles by her hand
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0x4a7ad8); dot(x, y + 1, 0xbfe0ff); fill(x - 3, y, 7, 1, LOL_GOLD); dot(x - 3, y + 1, LOL_GOLD); dot(x + 3, y + 1, LOL_GOLD);
      fill(x - 3, y + 5, 7, 1, LOL_GOLD_DARK);
      const t = (now + x * 0.29) % 18;
      if (t < 1) { const k = Math.floor(t * 6) % 2; dot(x + 5, y + 3, WHITE); if (k) { dot(x + 4, y + 3, 0xfff4a0); dot(x + 6, y + 3, 0xfff4a0); dot(x + 5, y + 2, 0xfff4a0); dot(x + 5, y + 4, 0xfff4a0); } }
    },
    back(x, y, o) { fill(x - 3, y, 7, 1, LOL_GOLD); fill(x - 3, y + 5, 7, 1, LOL_GOLD_DARK); fill(x - 2, y, 5, 4, 0xf2d27a); },
    head(x, top, look, away) {
      const hair = 0xf2d27a, hairDark = 0xd0a84a;
      fill(x - 2, top, 5, 1, hair); fill(x - 3, top + 1, 7, 1, hair); fill(x - 3, top + 2, 1, 5, hair); fill(x + 3, top + 2, 1, 5, hair); dot(x - 3, top + 6, hairDark); dot(x + 3, top + 6, hairDark);
      if (away) { fill(x - 3, top + 2, 7, 6, hair); return; }
      fill(x - 2, top, 5, 1, LOL_GOLD); dot(x, top - 1, LOL_GOLD); dot(x, top, 0x4a7ad8);
    },
  },
  wowpaladin: {
    // An Alliance paladin: silver plate with gold pauldrons, a royal-blue tabard with a gold lion's head and gold trim;
    // every 30 s a ring of holy light sparkles round their head
    front(x, y, o) {
      fill(x - 2, y + 1, 5, 5, WOW_BLUE); fill(x - 2, y + 1, 1, 5, WOW_GOLD); fill(x + 2, y + 1, 1, 5, WOW_GOLD);
      dot(x, y + 2, WOW_GOLD); dot(x - 1, y + 3, WOW_GOLD_DARK); dot(x + 1, y + 3, WOW_GOLD_DARK);
      fill(x - 3, y, 2, 1, WOW_GOLD); fill(x + 2, y, 2, 1, WOW_GOLD); dot(x - 3, y, 0xfff0a0);
    },
    back(x, y, o) { fill(x - 3, y, 2, 1, WOW_GOLD); fill(x + 2, y, 2, 1, WOW_GOLD); fill(x - 2, y + 1, 5, 5, WOW_BLUE); },
    head(x, top, look, away) {
      const t = (now + x * 0.29) % 30;
      if (t < 1.5) for (let k = 0; k < 6; k++) { const a = k * Math.PI / 3 + t * 3; dot(x + Math.round(Math.cos(a) * 5), top + 3 + Math.round(Math.sin(a) * 2) - 2, 0xfff0a0, 1 - t / 1.5); }
    },
  },
  wowgrunt: {
    // A Horde grunt: brown leathers, a red sash across the chest, spiked iron pauldrons, a black topknot and two white
    // tusks jutting from the jaw
    front(x, y, o) {
      for (let k = 0; k < 5; k++) dot(x - 2 + k, y + 1 + k, WOW_RED); dot(x - 2, y + 1, 0xb83a2a);
      fill(x - 3, y, 2, 1, WOW_IRON); fill(x + 2, y, 2, 1, WOW_IRON); dot(x - 4, y - 1, 0xc8c8d0); dot(x + 4, y - 1, 0xc8c8d0);
      fill(x - 3, y + 5, 7, 1, 0x3a2a1a); dot(x, y + 5, 0x9a9aa0);
    },
    back(x, y, o) { fill(x - 3, y, 2, 1, WOW_IRON); fill(x + 2, y, 2, 1, WOW_IRON); dot(x - 4, y - 1, 0xc8c8d0); dot(x + 4, y - 1, 0xc8c8d0); for (let k = 0; k < 5; k++) dot(x + 2 - k, y + 1 + k, WOW_RED); },
    head(x, top, look, away) {
      fill(x - 1, top - 2, 3, 2, 0x1a1414); dot(x, top - 3, 0x1a1414);
      if (away) return;
      dot(x - 2, top + 5, 0xf4f0e0); dot(x + 2, top + 5, 0xf4f0e0); dot(x - 2, top + 6, 0xd8d4c4); dot(x + 2, top + 6, 0xd8d4c4);
    },
  },
  wowmage: {
    // A Kirin Tor mage: violet robes with gold trim down the front and a gold sash, a deep violet hood; every 20 s
    // three arcane motes circle them for a moment
    front(x, y, o) {
      fill(x, y, 1, 6, WOW_GOLD); dot(x - 1, y, WOW_GOLD); dot(x + 1, y, WOW_GOLD); fill(x - 3, y + 3, 7, 1, WOW_GOLD_DARK);
      dot(x - 3, y + 1, shaded(o.shirt, 1.3));
    },
    back(x, y, o) { fill(x - 3, y + 3, 7, 1, WOW_GOLD_DARK); fill(x - 2, y, 5, 2, shaded(o.shirt, 0.8)); },
    head(x, top, look, away, o) {
      const hood = shaded(o.shirt, 0.85);
      fill(x - 3, top - 1, 7, 2, hood); fill(x - 4, top + 1, 1, 6, hood); fill(x + 4, top + 1, 1, 6, hood); dot(x - 2, top - 2, hood); dot(x + 2, top - 2, hood); fill(x - 1, top - 2, 3, 1, hood);
      if (away) { fill(x - 3, top + 1, 7, 6, hood); return; }
      const t = (now + x * 0.33) % 20;
      if (t < 1.6) for (let k = 0; k < 3; k++) { const a = t * 4 + k * 2.1; dot(x + Math.round(Math.cos(a) * 6), top + 4 + Math.round(Math.sin(a) * 3), WOW_ARCANE); }
    },
  },
  wowelf: {
    // A night elf druid: green leathers with a leaf clasp, long ears swept up and out, teal hair and a circlet of
    // leaves; the ears twitch now and then
    front(x, y, o) {
      dot(x, y + 1, 0x7ac84a); dot(x - 1, y + 1, 0x5aa83a); fill(x - 3, y + 4, 7, 1, 0x5a4a2a); dot(x - 3, y, shaded(o.shirt, 1.25)); dot(x + 3, y, shaded(o.shirt, 1.25));
    },
    back(x, y, o) { fill(x - 3, y + 4, 7, 1, 0x5a4a2a); fill(x - 2, y, 5, 3, 0x3ac8b8); },
    head(x, top, look, away) {
      const hair = 0x3ac8b8, hairDark = 0x2a9a8a, twitch = (now + x * 0.37) % 10 < 0.3 ? 1 : 0;
      fill(x - 2, top, 5, 1, hair); fill(x - 3, top + 1, 7, 1, hair); fill(x - 3, top + 2, 1, 4, hair); fill(x + 3, top + 2, 1, 4, hair); dot(x - 3, top + 5, hairDark); dot(x + 3, top + 5, hairDark);
      if (away) fill(x - 3, top + 2, 7, 5, hair);
      for (const d of [-1, 1]) { fill(x + d * 4 - (d < 0 ? 1 : 0), top + 3, 2, 1, look.skin); dot(x + d * 6, top + 2 - twitch, look.skin); dot(x + d * 7, top + 1 - twitch, look.skin); }
      dot(x - 2, top, 0x5aa83a); dot(x, top - 1, 0x7ac84a); dot(x + 2, top, 0x5aa83a);
    },
  },
  gtatommy: {
    // A Vice City Hawaiian shirt: sky-blue, printed with pink and white hibiscus, worn open at the collar
    front(x, y, o) {
      for (const [dx, dy, c] of [[-2, 1, GTA_PINK], [1, 2, WHITE], [-1, 4, WHITE], [2, 4, GTA_PINK], [-3, 3, GTA_PINK], [3, 1, WHITE], [0, 5, GTA_PINK]]) dot(x + dx, y + dy, c);
      dot(x, y, shaded(o.shirt, 0.7)); dot(x - 1, y, WHITE); dot(x + 1, y, WHITE);
    },
    back(x, y, o) { for (const [dx, dy, c] of [[-2, 1, GTA_PINK], [1, 2, WHITE], [-1, 4, WHITE], [2, 4, GTA_PINK], [3, 0, WHITE]]) dot(x + dx, y + dy, c); },
  },
  gtacj: {
    // Grove Street: a white vest, a gold chain, and a green bandana tied round the head, its knot showing at the back
    front(x, y, o) {
      dot(x - 3, y, shaded(o.shirt, 0.85)); dot(x + 3, y, shaded(o.shirt, 0.85)); dot(x - 1, y, 0xd8b04a); dot(x, y + 1, 0xd8b04a); dot(x + 1, y, 0xd8b04a);
      fill(x - 3, y + 5, 7, 1, 0x2a5a8a);
    },
    back(x, y, o) { fill(x - 3, y + 5, 7, 1, 0x2a5a8a); },
    head(x, top, look, away) {
      const green = 0x2ab84a, greenDark = 0x1a8a34;
      fill(x - 3, top, 7, 2, green); fill(x - 3, top + 1, 7, 1, greenDark); dot(x - 1, top, WHITE, 0.6); dot(x + 2, top + 1, WHITE, 0.5);
      if (away) { fill(x - 1, top + 2, 3, 1, green); dot(x - 1, top + 3, greenDark); dot(x + 1, top + 3, greenDark); }
    },
  },
  gtacop: {
    // A police uniform: dark blue with a gold badge, a black duty belt and a radio on the shoulder, and a police cap
    // with a gold badge and black peak; now and then the radio's light blinks
    front(x, y, o) {
      dot(x - 2, y + 1, 0xd8b04a); dot(x + 2, y + 1, 0x3a4a6a); fill(x - 3, y + 5, 7, 1, 0x1a1a1a); dot(x, y + 5, GTA_CHROME);
      dot(x + 3, y, 0x1a1a1a); if ((now + x * 0.3) % 5 < 0.3) dot(x + 3, y, 0xff3a4a);
      dot(x, y, 0x4a5a7a); dot(x, y + 2, 0x4a5a7a); dot(x, y + 4, 0x4a5a7a);
    },
    back(x, y, o) { fill(x - 3, y + 5, 7, 1, 0x1a1a1a); dot(x + 3, y, 0x1a1a1a); },
    head(x, top, look, away) {
      fill(x - 3, top - 2, 7, 3, 0x1e2a4a); fill(x - 3, top - 2, 7, 1, 0x3a4a6a); fill(x - 3, top + 1, 7, 1, 0x1a1a1a);
      if (!away) { dot(x, top - 1, 0xd8b04a); fill(x - 4, top + 1, 9, 1, 0x1a1a1a); }
    },
  },
  gtaheist: {
    // A heist crew: all in black, a balaclava over the head with only the eyes showing, and a bulging money bag
    // slung over the shoulder with a dollar sign on it
    front(x, y, o) {
      line(x - 3, y, x + 3, y + 5, 0x6a5a3a); dot(x - 3, y + 1, 0x3a3a44);
    },
    back(x, y, o) { fill(x - 3, y, 6, 5, 0x8a7a5a); fill(x - 3, y, 6, 1, 0xa89a7a); dot(x, y + 2, 0x3a8a3a); dot(x - 1, y + 1, 0x3a8a3a); dot(x + 1, y + 3, 0x3a8a3a); },
    head(x, top, look, away) {
      const mask = 0x2a2a30;
      fill(x - 3, top, 7, 7, mask); fill(x - 2, top - 1, 5, 1, mask); dot(x, top - 2, mask);
      if (away) return;
      fill(x - 3, top + 3, 7, 3, look.skin); fill(x - 3, top + 3, 7, 1, mask); fill(x - 3, top + 5, 7, 1, mask);
      dot(x - 3, top + 4, mask); dot(x, top + 4, mask); dot(x + 3, top + 4, mask);
      dot(x - 2, top + 4, PALETTE.eye); dot(x + 2, top + 4, PALETTE.eye);
    },
  },
  destinyhunterwear: {
    // Hunter: dark plate under a blue cloak falling from the shoulders, a knife at the belt, and the hood up over a
    // helmet with a sweeping blue visor that glints now and then
    front(x, y, o) {
      fill(x - 3, y, 1, 6, 0x2a4a8a); fill(x + 3, y, 1, 6, 0x2a4a8a); dot(x - 3, y, 0x4a6ab8);
      fill(x - 2, y + 4, 5, 1, 0x2a2a30); dot(x + 1, y + 4, 0x9aa0aa); dot(x, y + 1, 0x5a6a7a);
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x2a4a8a); fill(x - 3, y, 7, 1, 0x4a6ab8); for (let k = -2; k <= 2; k += 2) dot(x + k, y + 5, 0x1a3060); },
    head(x, top, look, away) {
      const hood = 0x2a4a8a, hoodDark = 0x1a3060;
      fill(x - 3, top - 1, 7, 2, hood); fill(x - 4, top + 1, 1, 6, hood); fill(x + 4, top + 1, 1, 6, hoodDark); dot(x, top - 2, hood);
      if (away) { fill(x - 3, top + 1, 7, 6, hood); return; }
      fill(x - 3, top + 1, 7, 6, 0x4a4e58); fill(x - 3, top + 3, 7, 2, 0x3a7ab8); fill(x - 3, top + 3, 7, 1, 0x6ab0e8);
      if ((now + x * 0.29) % 9 < 0.4) dot(x - 2 + Math.floor(((now + x * 0.29) % 9) * 15), top + 3, WHITE);
    },
  },
  destinytitanwear: {
    // Titan: heavy grey plate with broad pauldrons, a red mark hanging from the belt, and a domed helmet with a slit of
    // orange visor
    front(x, y, o) {
      fill(x - 4, y, 2, 2, 0x6a6e78); fill(x + 3, y, 2, 2, 0x6a6e78); dot(x - 4, y, 0xb8bcc4); dot(x + 3, y, 0xb8bcc4);
      fill(x - 2, y + 1, 5, 2, shaded(o.shirt, 1.15)); fill(x - 3, y + 4, 7, 1, 0x3a3e46);
      fill(x - 1, y + 5, 3, 1, 0xa83a2a); dot(x, y + 5, DST_GOLD);
    },
    back(x, y, o) { fill(x - 4, y, 2, 2, 0x6a6e78); fill(x + 3, y, 2, 2, 0x6a6e78); fill(x - 3, y + 4, 7, 1, 0x3a3e46); },
    head(x, top, look, away) {
      const helm = 0x7a7e88;
      fill(x - 3, top - 1, 7, 8, helm); fill(x - 2, top - 2, 5, 1, helm); fill(x - 3, top - 1, 7, 1, 0xa8acb4); fill(x + 3, top, 1, 6, 0x5a5e66);
      if (away) return;
      fill(x - 2, top + 3, 5, 1, DST_SOLAR); dot(x - 2, top + 3, 0xffd090); fill(x - 1, top + 5, 3, 1, 0x5a5e66);
    },
  },
  destinywarlockwear: {
    // Warlock: a long ochre coat with gold trim in a V and a bond on the wrist; every 20 s a small flame of solar
    // Light flickers over their open hand
    front(x, y, o) {
      for (let k = 0; k < 4; k++) { dot(x - 3 + k, y + k, DST_GOLD); dot(x + 3 - k, y + k, DST_GOLD); }
      fill(x, y + 3, 1, 3, 0x4a3420); fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.75));
      const t = (now + x * 0.33) % 20;
      if (t < 1.5) { const f = Math.floor(now * 8) % 2; dot(x + 5, y + 3, DST_SOLAR); dot(x + 5, y + 2 - f, 0xffd060); }
    },
    back(x, y, o) { fill(x - 1, y, 3, 6, shaded(o.shirt, 0.85)); fill(x - 3, y, 7, 1, DST_GOLD); },
  },
  destinyexo: {
    // An Exo Hunter: a blue cloak and hood over dark plate, and under the hood a grey metal face with two glowing blue
    // eyes and a pair of horns swept back; the eyes flicker now and then
    front(x, y, o) {
      fill(x - 3, y, 1, 6, shaded(o.shirt, 1.2)); fill(x + 3, y, 1, 6, shaded(o.shirt, 0.8)); fill(x - 2, y, 5, 6, 0x3a3e46);
      fill(x - 2, y + 4, 5, 1, 0x2a2a30); dot(x - 1, y + 1, 0x6a6e78);
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, o.shirt); fill(x - 3, y, 7, 1, shaded(o.shirt, 1.25)); },
    head(x, top, look, away) {
      const metal = 0x8a8e98, metalDark = 0x5a5e66;
      fill(x - 3, top, 7, 7, metal); fill(x - 3, top, 7, 1, 0xb0b4bc); fill(x + 3, top + 1, 1, 6, metalDark);
      dot(x - 3, top - 1, metalDark); dot(x - 4, top - 2, metalDark); dot(x + 3, top - 1, metalDark); dot(x + 4, top - 2, metalDark);
      if (away) return;
      const flick = (now + x * 0.27) % 6 < 0.15;
      dot(x - 2, top + 3, flick ? 0x3a5a7a : 0x8ae8ff); dot(x + 2, top + 3, flick ? 0x3a5a7a : 0x8ae8ff);
      fill(x - 1, top + 5, 3, 1, metalDark); dot(x - 2, top + 5, metalDark); dot(x + 2, top + 5, metalDark);
      if (isNight) glowLate(() => { dot(x - 2, top + 3, 0xd8f8ff); dot(x + 2, top + 3, 0xd8f8ff); });
    },
  },
  starwarsjedi: {
    // A cream tunic with crossed folds under a brown cloak open at the front, a dark belt with a silver lightsaber hilt hanging from it
    front(x, y, o) {
      const cloak = 0x7a5838, cloakLight = 0x9a7450, fold = shaded(o.shirt, 0.82);
      fill(x - 3, y, 2, 6, cloak); fill(x - 3, y, 1, 6, cloakLight); fill(x + 2, y, 2, 6, cloak); fill(x + 3, y, 1, 6, shaded(cloak, 0.8));
      dot(x - 1, y, fold); dot(x, y + 1, fold); dot(x + 1, y, fold);
      fill(x - 1, y + 4, 3, 1, 0x3a2a1e); dot(x, y + 4, 0x8a6a48);
      dot(x + 1, y + 5, 0xb8bcc4); dot(x + 1, y + 4, 0xd0d4dc, 0.0);
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x7a5838); fill(x - 3, y, 1, 6, 0x9a7450); fill(x, y, 1, 6, 0x6a4a2e); fill(x - 3, y, 7, 1, 0x9a7450); },
  },
  starwarsvader: {
    // Black armour: a chest box whose little lights blink, a belt of silver boxes, a black cape; the helmet's dome and flared rim, a
    // triangle mask with dark lenses that catch a red glint now and then
    front(x, y, o) {
      fill(x - 3, y, 1, 6, 0x2a2a32); fill(x + 3, y, 1, 6, 0x2a2a32);
      fill(x - 1, y + 1, 3, 2, 0x4a4e58);
      const b = Math.floor(now * 2 + x) % 3;
      dot(x - 1, y + 1, 0xff4a3a, b === 0 ? 1 : 0.4); dot(x, y + 1, 0x5aff7a, b === 1 ? 1 : 0.4); dot(x + 1, y + 1, 0x6ab8ff, b === 2 ? 1 : 0.4);
      fill(x - 2, y + 4, 5, 1, 0x3a3a44); dot(x - 1, y + 4, 0xb8bcc4); dot(x + 1, y + 4, 0xb8bcc4);
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x101014); fill(x - 1, y + 1, 1, 5, 0x22222a); fill(x + 2, y + 2, 1, 4, 0x22222a); },
    head(x, top, look, away, o) {
      const H = 0x16161c, L = 0x3a3a44;
      fill(x - 3, top - 1, 7, 3, H); fill(x - 2, top - 2, 5, 1, H); fill(x - 4, top + 2, 9, 5, H); fill(x - 5, top + 5, 2, 2, H); fill(x + 4, top + 5, 2, 2, H);
      dot(x - 2, top - 2, L); dot(x - 3, top - 1, L); dot(x - 4, top + 2, L);
      if (away) { fill(x - 3, top + 2, 7, 4, H); dot(x, top, L); return; }
      fill(x - 2, top + 3, 2, 1, 0x3a2a30); fill(x + 1, top + 3, 2, 1, 0x3a2a30); dot(x, top + 4, 0x5a5e68);
      fill(x - 1, top + 5, 3, 2, 0x4a4e58); dot(x, top + 6, H);
      if ((now + x * 0.41) % 13 < 0.5) dot(x - 2, top + 3, 0xff4a3a);
    },
  },
  starwarsstormtrooper: {
    // White armour plates with black gaps at the waist and the chest, a white belt with black pouches; a white helmet with black
    // teardrop lenses, the grey frown of its grille and blue-grey side stripes
    front(x, y, o) {
      fill(x - 3, y + 3, 7, 1, 0x26262c); fill(x - 1, y, 3, 1, 0x26262c, 0.5); dot(x, y + 1, 0x8a909c);
      fill(x - 3, y, 1, 3, WHITE); fill(x + 3, y, 1, 6, 0xc4c8d0);
      fill(x - 3, y + 4, 7, 1, WHITE); dot(x - 2, y + 4, 0x26262c); dot(x + 2, y + 4, 0x26262c); dot(x, y + 5, 0x26262c, 0.6);
    },
    back(x, y, o) { fill(x - 3, y + 3, 7, 1, 0x26262c); fill(x - 2, y, 5, 3, WHITE); fill(x - 1, y + 1, 3, 1, 0xc4c8d0); fill(x - 3, y + 4, 7, 1, WHITE); },
    head(x, top, look, away, o) {
      const H = 0xf0f2f6, D = 0xc4c8d0, K = 0x16161c;
      fill(x - 3, top - 1, 7, 8, H); fill(x - 2, top - 2, 5, 1, H); fill(x - 4, top + 2, 1, 4, H); fill(x + 4, top + 2, 1, 4, D);
      dot(x - 2, top - 2, WHITE); fill(x + 3, top, 1, 6, D);
      if (away) { fill(x - 1, top, 3, 1, D); fill(x - 3, top + 6, 7, 1, D); return; }
      fill(x - 3, top + 3, 2, 1, K); fill(x + 2, top + 3, 2, 1, K); dot(x - 2, top + 4, K); dot(x + 2, top + 4, K);
      fill(x - 1, top + 6, 3, 1, 0x8a909c); dot(x, top + 5, 0x5a5e68);
      dot(x - 4, top + 3, 0x4a5a7a); dot(x + 4, top + 3, 0x4a5a7a);
    },
  },
  starwarsprincess: {
    // A white hooded gown to the floor with a silver belt; hair coiled in a bun over each ear
    front(x, y, o) {
      fill(x - 1, y, 3, 1, 0xdcd8d0); dot(x, y + 1, 0xdcd8d0);
      fill(x - 3, y + 3, 7, 1, 0xb8bcc4); dot(x - 2, y + 3, 0xe0e4ea); dot(x + 1, y + 3, 0xe0e4ea);
      fill(x + 3, y, 1, 6, 0xd8d4cc); fill(x - 3, y, 1, 6, WHITE);
    },
    back(x, y, o) { fill(x - 3, y, 7, 2, 0xe8e4dc); fill(x - 3, y + 3, 7, 1, 0xb8bcc4); fill(x, y + 4, 1, 2, 0xdcd8d0); },
    head(x, top, look, away) {
      const H = look.hair, D = shaded(look.hair, 0.75);
      fill(x - 3, top, 7, 2, H); fill(x - 6, top + 2, 3, 4, H); fill(x + 4, top + 2, 3, 4, H);
      dot(x - 5, top + 3, D); dot(x + 5, top + 3, D); dot(x - 6, top + 2, shaded(look.hair, 1.25)); dot(x + 4, top + 2, shaded(look.hair, 1.25));
      if (away) fill(x - 3, top + 2, 7, 3, H);
    },
  },
  girly1gingham: {
    // A pink gingham dress to the knee with a white collar and a white belt bow; a pink headband with a bow in the hair
    front(x, y, o) {
      const D = 0xe880b0;
      for (let r = 0; r < 6; r += 2) for (let c = -3; c <= 3; c += 2) dot(x + c, y + r + 1, D);
      fill(x - 2, y, 2, 1, WHITE); fill(x + 1, y, 2, 1, WHITE); dot(x, y + 1, WHITE, 0.0);
      fill(x - 3, y + 4, 7, 1, WHITE); dot(x - 1, y + 4, 0xf4dce6); dot(x + 1, y + 4, 0xf4dce6);
    },
    back(x, y, o) { for (let r = 0; r < 6; r += 2) for (let c = -3; c <= 3; c += 2) dot(x + c, y + r + 1, 0xe880b0); fill(x - 3, y + 4, 7, 1, WHITE); fill(x - 1, y + 4, 3, 2, WHITE); },
    head(x, top, look, away) {
      fill(x - 3, top + 1, 7, 1, G1_PINK); fill(x + 2, top - 1, 3, 2, G1_PINK); dot(x + 3, top, G1_PINK_LIGHT);
    },
  },
  girly1cowgirl: {
    // A white top under a hot pink fringed vest with gold studs; flared pink trousers; a pink cowboy hat with a gold band
    front(x, y, o) {
      fill(x - 3, y, 2, 5, G1_PINK); fill(x + 2, y, 2, 5, G1_PINK); fill(x - 3, y, 1, 5, G1_PINK_LIGHT);
      for (const dx of [-3, -2, 2, 3]) dot(x + dx, y + 5, G1_PINK_DARK);
      dot(x - 2, y + 2, G1_GOLD); dot(x + 2, y + 2, G1_GOLD); fill(x - 1, y + 4, 3, 1, G1_GOLD);
    },
    back(x, y, o) { fill(x - 3, y, 7, 5, G1_PINK); fill(x - 3, y, 7, 1, G1_PINK_LIGHT); for (let k = -3; k <= 3; k += 2) dot(x + k, y + 5, G1_PINK_DARK); },
    head(x, top, look, away) {
      fill(x - 3, top - 3, 7, 3, G1_PINK); dot(x, top - 3, G1_PINK_DARK); fill(x - 3, top - 3, 7, 1, G1_PINK_LIGHT);
      fill(x - 3, top - 1, 7, 1, G1_GOLD);
      fill(x - 6, top, 13, 1, G1_PINK); dot(x - 6, top - 1, G1_PINK); dot(x + 6, top - 1, G1_PINK); dot(x - 6, top, G1_PINK_LIGHT);
    },
  },
  girly1tracksuit: {
    // A hot pink velour tracksuit: a zip jacket with a gold zip and white stripes down the sides, matching trousers; big gold hoop earrings
    front(x, y, o) {
      fill(x, y, 1, 6, G1_GOLD); dot(x, y, G1_GOLD_LIGHT);
      fill(x - 3, y, 1, 6, WHITE); fill(x + 3, y, 1, 6, WHITE);
      fill(x - 1, y, 3, 1, G1_PINK_LIGHT); dot(x - 2, y + 2, G1_PINK_LIGHT); dot(x + 2, y + 3, G1_PINK_DARK);
    },
    back(x, y, o) { fill(x - 3, y, 1, 6, WHITE); fill(x + 3, y, 1, 6, WHITE); fill(x - 2, y, 5, 1, G1_PINK_LIGHT); dot(x, y + 2, G1_PINK_DARK); },
    head(x, top, look, away) {
      if (away) return;
      dot(x - 4, top + 5, G1_GOLD); dot(x - 4, top + 6, G1_GOLD_DARK); dot(x + 4, top + 5, G1_GOLD); dot(x + 4, top + 6, G1_GOLD_DARK);
    },
  },
  girly1gown: {
    // A pale pink ball gown to the floor with a sweetheart neckline and sparkles that twinkle over it; a gold tiara with a pink gem
    front(x, y, o) {
      fill(x - 2, y, 2, 1, 0xf0c8a8, 0.0);
      dot(x - 3, y, o.shirt); dot(x + 3, y, o.shirt); dot(x - 1, y, shaded(o.shirt, 0.85)); dot(x + 1, y, shaded(o.shirt, 0.85));
      fill(x - 3, y + 2, 7, 1, 0xe8a0c0);
      const k = Math.floor(now * 2 + x) % 4;
      [[-2, 4], [1, 3], [2, 5], [-1, 1]].forEach(([dx, dy], i) => dot(x + dx, y + dy, WHITE, i === k ? 1 : 0.45));
    },
    back(x, y, o) { fill(x - 3, y + 2, 7, 1, 0xe8a0c0); fill(x - 1, y + 2, 3, 2, 0xf4b0cc); dot(x - 2, y + 4, WHITE, 0.5); dot(x + 2, y + 5, WHITE, 0.5); },
    head(x, top, look, away) {
      fill(x - 2, top, 5, 1, G1_GOLD); dot(x - 2, top - 1, G1_GOLD); dot(x + 2, top - 1, G1_GOLD); dot(x, top - 2, G1_GOLD); dot(x, top - 1, 0xff5ab8);
    },
  },
  girly2kittyhood: {
    // A white hoodie with a pink kangaroo pocket and drawstrings; the hood up, with cat ears on top and a red bow on the left one
    front(x, y, o) {
      dot(x - 1, y, 0xe0d8dc); dot(x + 1, y, 0xe0d8dc); dot(x - 1, y + 1, G2_PINK_DARK); dot(x + 1, y + 1, G2_PINK_DARK);
      fill(x - 2, y + 3, 5, 2, G2_PINK); fill(x - 2, y + 3, 5, 1, 0xfadce6);
      fill(x - 3, y, 1, 6, WHITE); fill(x + 3, y, 1, 6, 0xe0d8dc);
    },
    back(x, y, o) { fill(x - 2, y, 5, 2, 0xf4eef0); fill(x + 3, y, 1, 6, 0xe0d8dc); },
    head(x, top, look, away) {
      const H = 0xfffcfa, D = 0xe0d8dc;
      fill(x - 4, top - 1, 9, 2, H); fill(x - 4, top + 1, 1, 6, H); fill(x + 4, top + 1, 1, 6, D);
      fill(x - 4, top - 3, 2, 2, H); fill(x + 3, top - 3, 2, 2, H); dot(x - 3, top - 2, G2_PINK_DARK); dot(x + 3, top - 2, G2_PINK_DARK);
      fill(x - 5, top - 4, 2, 2, G2_BOW); fill(x - 2, top - 4, 2, 2, G2_BOW); dot(x - 3, top - 3, 0xb81e34);
      if (away) fill(x - 3, top + 1, 7, 5, H);
    },
  },
  girly2bunnyhood: {
    // A pink dress to the knee with a white round collar; a pink bunny hood with long ears, white inside, a little flower on the right one
    front(x, y, o) {
      fill(x - 2, y, 2, 1, WHITE); fill(x + 1, y, 2, 1, WHITE); dot(x - 2, y + 1, WHITE); dot(x + 2, y + 1, WHITE);
      dot(x, y + 1, G2_BOW); fill(x - 3, y + 5, 7, 1, 0xe890b0);
    },
    back(x, y, o) { fill(x - 3, y, 7, 1, 0xf8b8cc); fill(x - 3, y + 5, 7, 1, 0xe890b0); },
    head(x, top, look, away) {
      const P = 0xf8b8cc, PD = 0xe890b0;
      fill(x - 4, top - 1, 9, 2, P); fill(x - 4, top + 1, 1, 6, P); fill(x + 4, top + 1, 1, 6, PD);
      fill(x - 3, top - 6, 2, 5, P); dot(x - 2, top - 5, WHITE); dot(x - 2, top - 4, WHITE);
      fill(x + 2, top - 6, 2, 5, P); dot(x + 2, top - 5, WHITE); dot(x + 2, top - 4, WHITE); dot(x + 4, top - 5, G2_NOSE); dot(x + 4, top - 6, WHITE);
      if (away) fill(x - 3, top + 1, 7, 5, P);
    },
  },
  girly2sailor: {
    // A cream sailor top with a lilac collar edged in white and a pink bow at the neck, a lilac pleated skirt; two star clips in the hair
    front(x, y, o) {
      fill(x - 3, y, 2, 2, G2_LILAC); fill(x + 2, y, 2, 2, G2_LILAC); dot(x - 1, y + 2, G2_LILAC); dot(x + 1, y + 2, G2_LILAC);
      fill(x - 1, y + 2, 3, 1, G2_PINK_DARK); dot(x, y + 3, G2_PINK_DARK);
      for (const dx of [-2, 0, 2]) dot(x + dx, y + 5, G2_LILAC_DARK, 0.6);
    },
    back(x, y, o) { fill(x - 3, y, 7, 3, G2_LILAC); fill(x - 3, y + 2, 7, 1, WHITE); },
    head(x, top, look, away) {
      for (const sx of [-3, 3]) { dot(x + sx, top + 1, G2_NOSE); dot(x + sx - 1, top + 1, G2_NOSE, 0.7); dot(x + sx + 1, top + 1, G2_NOSE, 0.7); dot(x + sx, top, G2_NOSE, 0.7); }
    },
  },
  girly2overalls: {
    // Lilac overalls over a mint tee, the bib with two white buttons and a strawberry patch; a strawberry clip in the hair
    front(x, y, o) {
      const L = 0xb8a0e0;
      fill(x - 2, y + 2, 5, 4, L); fill(x - 2, y, 1, 2, L); fill(x + 2, y, 1, 2, L);
      dot(x - 2, y + 2, WHITE); dot(x + 2, y + 2, WHITE);
      dot(x, y + 3, 0xe8304a); dot(x, y + 4, 0xd8202a); dot(x, y + 2, 0x5ab86a);
    },
    back(x, y, o) { const L = 0xb8a0e0; fill(x - 3, y + 3, 7, 3, L); line(x - 2, y, x + 1, y + 3, L); line(x + 2, y, x - 1, y + 3, L); },
    head(x, top, look, away) {
      dot(x + 3, top, 0xe8304a); dot(x + 4, top, 0xe8304a); dot(x + 3, top + 1, 0xd8202a); dot(x + 4, top - 1, 0x5ab86a);
    },
  },
  narcosboss: {
    // A pale blue polo with a collar and a two-button placket over jeans; big curly black hair and a thick moustache
    front(x, y, o) {
      fill(x - 2, y, 2, 1, 0xa8d0f0); fill(x + 1, y, 2, 1, 0xa8d0f0); fill(x, y, 1, 2, 0x6a98c0); dot(x, y + 1, WHITE); dot(x, y + 2, WHITE, 0.8);
      fill(x + 3, y, 1, 6, 0x6a98c0);
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, 0xa8d0f0); fill(x + 3, y, 1, 6, 0x6a98c0); },
    head(x, top, look, away) {
      const K = 0x1a1210;
      fill(x - 4, top - 1, 9, 3, K); fill(x - 5, top, 1, 5, K); fill(x + 5, top, 1, 5, K); for (const dx of [-3, -1, 1, 3]) dot(x + dx, top - 2, K); dot(x - 2, top - 1, 0x3a2a20); dot(x + 2, top, 0x3a2a20);
      if (away) { fill(x - 4, top + 2, 9, 3, K); return; }
      fill(x - 2, top + 6, 5, 1, K); dot(x - 2, top + 7, K, 0.6); dot(x + 2, top + 7, K, 0.6);
    },
  },
  narcosdea: {
    // A navy windbreaker with yellow letters on the chest and a zip, khaki trousers; a navy cap with a yellow badge
    front(x, y, o) {
      fill(x, y, 1, 6, 0x3a4a6a); fill(x - 3, y + 1, 2, 1, 0xf2c830); fill(x + 2, y + 1, 2, 1, 0xf2c830); dot(x - 2, y + 2, 0xf2c830, 0.6);
      fill(x - 3, y + 5, 7, 1, 0x141c34);
    },
    back(x, y, o) { fill(x - 3, y + 1, 7, 2, 0xf2c830); for (const dx of [-2, 0, 2]) dot(x + dx, y + 2, 0x1e2a4a); fill(x - 3, y + 5, 7, 1, 0x141c34); },
    head(x, top, look, away) {
      const C = 0x1e2a4a;
      fill(x - 3, top - 2, 7, 3, C); fill(x - 3, top - 2, 7, 1, 0x3a4a6a);
      if (!away) { fill(x - 1, top - 1, 3, 1, 0xf2c830); fill(x - 4, top + 1, 8, 1, C); fill(x - 5, top + 1, 2, 1, 0x141c34); }
      else fill(x - 3, top + 1, 7, 1, C);
    },
  },
  narcosmiami: {
    // A white jacket open over a pastel pink tee, its sleeves pushed up; dark wraparound sunglasses
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0xf4a0c0); fill(x - 1, y, 3, 1, 0xf8b8d0);
      fill(x - 2, y, 1, 4, 0xd8d6d0); fill(x + 2, y, 1, 4, 0xd8d6d0); dot(x - 3, y + 4, 0xd8d6d0); dot(x + 3, y + 4, 0xd8d6d0);
    },
    back(x, y, o) { fill(x, y + 1, 1, 5, 0xd8d6d0); fill(x - 3, y, 7, 1, 0xe8e6e0); },
    head(x, top, look, away) {
      if (away) { fill(x - 4, top + 4, 1, 1, 0x1a2a2a); fill(x + 4, top + 4, 1, 1, 0x1a2a2a); return; }
      fill(x - 3, top + 4, 7, 1, 0x1a2a2a); dot(x - 2, top + 4, 0x3a8a8a); dot(x + 2, top + 4, 0x3a8a8a);
    },
  },
  narcoshawaiian: {
    // A loud teal shirt printed with pink and yellow flowers, open at the neck over a gold chain; gold aviator sunglasses
    front(x, y, o) {
      fill(x - 1, y, 3, 2, 0xd8a878); dot(x, y + 2, 0xd8a878); dot(x - 1, y + 1, NC_GOLD); dot(x + 1, y + 1, NC_GOLD); dot(x, y + 2, NC_GOLD_LIGHT);
      for (const [dx, dy, c] of [[-3, 1, 0xf07aa0], [-2, 4, 0xf2d35b], [2, 3, 0xf07aa0], [3, 5, 0xf2d35b], [-3, 5, 0xf07aa0], [3, 1, 0xf2d35b], [0, 4, 0xf07aa0]]) dot(x + dx, y + dy, c);
      dot(x - 2, y, 0x5ad8c8); dot(x + 2, y, 0x5ad8c8);
    },
    back(x, y, o) { for (const [dx, dy, c] of [[-2, 1, 0xf07aa0], [1, 2, 0xf2d35b], [3, 4, 0xf07aa0], [-3, 4, 0xf2d35b], [0, 5, 0xf07aa0]]) dot(x + dx, y + dy, c); },
    head(x, top, look, away) {
      if (away) return;
      fill(x - 3, top + 3, 7, 1, NC_GOLD); fill(x - 3, top + 4, 3, 1, 0x3a3a2a); fill(x + 1, top + 4, 3, 1, 0x3a3a2a); dot(x - 2, top + 4, 0x6a5a3a);
    },
  },
  apple1turtleneck: {
    // A black turtleneck, its roll collar up to the chin, with blue jeans and grey trainers; round rimless glasses
    front(x, y, o) { fill(x - 2, y, 5, 1, 0x2c2c2e); dot(x - 3, y + 2, 0x2c2c2e, 0.6); fill(x + 3, y, 1, 6, 0x101012); },
    back(x, y, o) { fill(x - 2, y, 5, 1, 0x2c2c2e); fill(x + 3, y, 1, 6, 0x101012); },
    head(x, top, look, away) {
      if (away) return;
      for (const ex of [-2, 2]) { dot(x + ex - 1, top + 4, 0x8a8a90, 0.8); dot(x + ex + 1, top + 4, 0x8a8a90, 0.8); dot(x + ex, top + 3, 0x8a8a90, 0.8); dot(x + ex, top + 5, 0x8a8a90, 0.6); }
      dot(x, top + 4, 0x8a8a90, 0.8);
    },
  },
  apple1genius: {
    // A bright blue staff tee with a small white apple on the chest and a white lanyard
    front(x, y, o) {
      appleMark('small', x - 2, y + 1, WHITE, 0.0); dot(x - 2, y + 2, WHITE); dot(x - 1, y + 2, WHITE); dot(x - 2, y + 3, WHITE); dot(x - 1, y + 3, WHITE); dot(x - 1, y + 1, WHITE, 0.8);
      line(x - 1, y, x + 1, y + 4, WHITE); fill(x + 1, y + 4, 2, 2, WHITE); dot(x + 2, y + 5, A1_BLUE);
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, 0x1e6ac8); dot(x, y + 1, WHITE, 0.6); },
  },
  apple1headset: {
    // A light grey knit top over dark trousers; a headset over the eyes, its dark glass front catching colour, a grey band round the back
    front(x, y, o) { for (let r = 0; r < 6; r += 2) for (let c = -2; c <= 2; c += 2) dot(x + c, y + r, 0xb8b8bc); },
    back(x, y, o) { for (let r = 1; r < 6; r += 2) for (let c = -3; c <= 3; c += 2) dot(x + c, y + r, 0xb8b8bc); },
    head(x, top, look, away) {
      if (away) { fill(x - 3, top + 3, 7, 1, 0x6a6a70); return; }
      fill(x - 4, top + 3, 9, 3, A1_SILVER); fill(x - 3, top + 3, 7, 2, 0x1c1c22); dot(x - 2, top + 3, 0x6a5ae8, 0.8); dot(x + 2, top + 3, 0x5ac8fa, 0.7);
      if ((now + x * 0.3) % 9 < 0.5) dot(x, top + 3, WHITE);
    },
  },
  apple1hoodie: {
    // A space grey hoodie with a small white apple, drawstrings and a pocket, the hood down round the neck
    front(x, y, o) {
      fill(x - 3, y, 7, 1, 0x6a6a6e); dot(x - 1, y + 1, WHITE, 0.8); dot(x + 1, y + 1, WHITE, 0.8);
      dot(x + 2, y + 2, WHITE); dot(x + 2, y + 3, WHITE, 0.8);
      fill(x - 2, y + 4, 5, 2, 0x4a4a4e);
    },
    back(x, y, o) { fill(x - 2, y, 5, 2, 0x6a6a6e); fill(x - 1, y + 2, 3, 1, 0x6a6a6e); },
  },
  apple2bowtie: {
    // A double-breasted slate suit with wide lapels over a white shirt and a black bow tie
    front(x, y, o) {
      fill(x - 1, y, 3, 3, A2_PAPER); fill(x - 1, y, 3, 1, A2_INK); dot(x, y, 0x3a3a3a);
      dot(x - 2, y, shaded(o.shirt, 1.25)); dot(x + 2, y, shaded(o.shirt, 1.25)); dot(x - 2, y + 1, shaded(o.shirt, 1.25)); dot(x + 2, y + 1, shaded(o.shirt, 1.25));
      for (const [dx, dy] of [[-1, 4], [1, 4], [-1, 5], [1, 5]]) dot(x + dx, y + dy, 0x2a2e3a);
    },
    back(x, y, o) { fill(x, y + 1, 1, 5, shaded(o.shirt, 0.8)); },
  },
  apple2woz: {
    // A slate blue polo over grey trousers; a full dark beard and big square glasses
    front(x, y, o) { fill(x - 2, y, 2, 1, shaded(o.shirt, 1.2)); fill(x + 1, y, 2, 1, shaded(o.shirt, 1.2)); fill(x, y, 1, 2, shaded(o.shirt, 0.75)); dot(x, y + 1, WHITE); },
    back(x, y, o) { fill(x - 2, y, 5, 1, shaded(o.shirt, 1.2)); },
    head(x, top, look, away) {
      const H = 0x3a2a20;
      if (away) { fill(x - 3, top + 5, 1, 2, H); fill(x + 3, top + 5, 1, 2, H); return; }
      fill(x - 3, top + 4, 1, 3, H); fill(x + 3, top + 4, 1, 3, H); fill(x - 2, top + 6, 5, 1, H); dot(x - 1, top + 5, H); dot(x + 1, top + 5, H);
      for (const ex of [-2, 2]) { fill(x + ex - 1, top + 3, 3, 1, A2_INK); dot(x + ex - 1, top + 4, A2_INK); dot(x + ex + 1, top + 4, A2_INK); }
    },
  },
  apple2rainbowtee: {
    // A white tee with the rainbow apple on the chest, over jeans
    front(x, y, o) { appleMark('small', x - 2, y, (r) => A2_RAINBOW[r]); fill(x + 3, y, 1, 6, 0xd8d8d0); },
    back(x, y, o) { fill(x + 3, y, 1, 6, 0xd8d8d0); fill(x - 1, y, 3, 1, 0xe8e8e0); },
  },
  apple2runner: {
    // The runner's kit: a white tank top with a little drawing of a computer on it, red shorts over bare legs, white trainers; a sweatband
    front(x, y, o) {
      fill(x - 3, y, 1, 6, 0xe0b090, 0.0); fill(x - 1, y + 1, 3, 3, 0xc8c0a8); fill(x - 1, y + 1, 3, 2, A2_INK, 0.0); dot(x, y + 2, A2_INK); dot(x - 1, y + 1, A2_INK, 0.6); dot(x + 1, y + 1, A2_INK, 0.6);
      fill(x - 3, y, 1, 2, 0xe0b090); fill(x + 3, y, 1, 2, 0xe0b090);
    },
    back(x, y, o) { fill(x - 3, y, 1, 2, 0xe0b090); fill(x + 3, y, 1, 2, 0xe0b090); },
    head(x, top, look, away) { fill(x - 3, top + 1, 7, 1, 0xe03a3e); },
  },
  jewishkippah: {
    // A crisp white shirt open at the collar with a row of buttons and dark trousers; a navy suede kippah on the crown
    front(x, y) {
      dot(x - 1, y, 0xd8d8dc); dot(x + 1, y, 0xd8d8dc); dot(x, y + 1, 0xe2e2e6);
      for (const by of [2, 4]) dot(x, y + by, 0xb8bcc8);
      fill(x + 3, y, 1, 6, 0xdcdce2);
    },
    back(x, y) { fill(x - 3, y, 7, 1, 0xe8e8ec); fill(x, y + 1, 1, 5, 0xe2e2e6, 0.6); },
    head(x, top) {
      const k = 0x24306a;
      fill(x - 1, top - 1, 3, 1, k); fill(x - 2, top, 5, 1, k); dot(x - 1, top - 1, 0x3a4a8a); dot(x + 2, top, 0x8a90a8);
    },
  },
  jewishtichel: {
    // A cream blouse with a soft round collar over a long navy skirt; a teal tichel dotted with gold, wrapped over the
    // hair and down to the shoulders, knotted at the back with its ends falling behind
    front(x, y) { fill(x - 2, y, 5, 1, 0xf6efe0); dot(x, y + 1, 0xc8b8a0); dot(x, y + 3, 0xc8b8a0); fill(x - 3, y + 5, 7, 1, 0x34405e, 0.5); },
    back(x, y) { fill(x - 2, y, 5, 1, 0xf6efe0); fill(x, y, 2, 3, 0x2a7a7a); dot(x + 1, y + 3, 0x1a5a5a); },
    head(x, top, look, away) {
      const c = 0x2a7a7a, light = 0x4aa0a0, dark = 0x1a5a5a, spot = 0xf2d35b;
      fill(x - 3, top - 2, 7, 1, c); fill(x - 4, top - 1, 9, 3, c); fill(x - 4, top + 2, 2, 7, c); fill(x + 3, top + 2, 2, 7, c);
      fill(x - 3, top - 2, 3, 1, light); fill(x - 4, top - 1, 1, 3, light); fill(x + 4, top + 1, 1, 8, dark);
      for (const [dx, dy] of [[-2, -1], [1, -1], [3, 0], [-1, 1], [2, 1]]) dot(x + dx, top + dy, spot, 0.8);
      if (away) { fill(x - 2, top + 2, 5, 5, c); fill(x - 1, top + 1, 3, 2, dark); dot(x, top + 1, light); fill(x, top + 3, 1, 5, dark); }
    },
  },
  jewishknit: {
    // A blue knit sweater with a white collar and a white band with a blue zigzag; a crocheted white kippah with a blue ring
    front(x, y) {
      fill(x - 2, y, 5, 1, 0xf2f0ea); fill(x - 3, y + 2, 7, 1, 0xf2f0ea);
      for (let k = 0; k < 7; k += 2) dot(x - 3 + k, y + 2, 0x24467a);
      for (let k = 1; k < 7; k += 2) dot(x - 3 + k, y + 4, 0x5a8ad0, 0.6);
      fill(x - 3, y + 5, 7, 1, 0x2a5090);
    },
    back(x, y) { fill(x - 3, y + 2, 7, 1, 0xf2f0ea); fill(x - 3, y + 5, 7, 1, 0x2a5090); },
    head(x, top) {
      const w = 0xf2f0ea, b = 0x3a6ab0;
      fill(x - 1, top - 1, 3, 1, w); fill(x - 2, top, 5, 1, w); dot(x, top - 1, b); dot(x - 2, top, b); dot(x + 2, top, b);
    },
  },
  jewishdress: {
    // A long-sleeved navy midi dress with a small white polka dot, a white rounded collar and a thin belt; a fine gold chain
    front(x, y) {
      fill(x - 2, y, 2, 1, 0xf6f4ee); fill(x + 1, y, 2, 1, 0xf6f4ee); dot(x, y + 1, 0xe8c060);
      fill(x - 3, y + 3, 7, 1, 0x1a2648);
      for (const [dx, dy] of [[-2, 2], [2, 2], [0, 5], [-2, 5], [2, 5]]) dot(x + dx, y + dy, 0xf6f4ee, 0.8);
    },
    back(x, y) {
      fill(x - 3, y + 3, 7, 1, 0x1a2648);
      for (const [dx, dy] of [[-2, 1], [1, 2], [0, 5], [2, 4]]) dot(x + dx, y + dy, 0xf6f4ee, 0.8);
    },
  },
  muslimhijab: {
    // A cream top under an open grey-blue cardigan, dark trousers; a mauve hijab framing the face, wrapped under the
    // chin and laid over the shoulders
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0xf2e6d4); fill(x - 2, y, 1, 6, shaded(o.shirt, 0.8)); fill(x + 2, y, 1, 6, shaded(o.shirt, 0.8));
      dot(x - 2, y + 3, 0xd8d0c0); dot(x - 2, y + 5, 0xd8d0c0);
    },
    back(x, y, o) { fill(x, y + 2, 1, 4, shaded(o.shirt, 0.85)); },
    head(x, top, look, away) {
      const c = 0x8a5a7a, light = 0xa8789a, dark = 0x6a4060;
      if (away) { fill(x - 3, top - 2, 7, 1, c); fill(x - 4, top - 1, 9, 10, c); fill(x - 3, top - 2, 3, 1, light); fill(x + 4, top - 1, 1, 10, dark); fill(x, top + 3, 1, 6, dark, 0.5); return; }
      fill(x - 3, top - 2, 7, 1, c); fill(x - 4, top - 1, 9, 3, c); fill(x - 4, top + 2, 2, 5, c); fill(x + 3, top + 2, 2, 5, c);
      fill(x - 4, top + 7, 9, 2, c); fill(x - 2, top + 7, 5, 1, dark);
      fill(x - 3, top - 2, 3, 1, light); fill(x - 4, top - 1, 1, 8, light); fill(x + 4, top - 1, 1, 10, dark);
      fill(x - 2, top + 1, 5, 1, dark, 0.5);
    },
  },
  muslimabaya: {
    // A flowing plum-black abaya with gold embroidery at the neckline, down the front and at the cuffs, and a matching
    // hijab edged in a thin line of gold
    front(x, y) { fill(x - 1, y, 3, 1, 0xd8b45a); fill(x, y + 1, 1, 5, 0xd8b45a, 0.8); dot(x - 3, y + 5, 0xd8b45a); dot(x + 3, y + 5, 0xd8b45a); dot(x - 1, y + 2, 0xd8b45a, 0.5); dot(x + 1, y + 2, 0xd8b45a, 0.5); },
    back(x, y) { fill(x - 3, y + 5, 7, 1, 0xd8b45a, 0.5); },
    head(x, top, look, away) {
      const c = 0x2a2430, light = 0x4a4050, dark = 0x1a1620, gold = 0xd8b45a;
      if (away) { fill(x - 3, top - 2, 7, 1, c); fill(x - 4, top - 1, 9, 10, c); fill(x - 3, top - 2, 3, 1, light); fill(x + 4, top - 1, 1, 10, dark); return; }
      fill(x - 3, top - 2, 7, 1, c); fill(x - 4, top - 1, 9, 3, c); fill(x - 4, top + 2, 2, 5, c); fill(x + 3, top + 2, 2, 5, c);
      fill(x - 4, top + 7, 9, 2, c);
      fill(x - 3, top - 2, 3, 1, light); fill(x - 4, top - 1, 1, 8, light); fill(x + 4, top - 1, 1, 10, dark);
      fill(x - 2, top + 1, 5, 1, gold, 0.7); dot(x - 2, top + 7, gold, 0.6); dot(x + 2, top + 7, gold, 0.6);
    },
  },
  muslimthobe: {
    // A long white thobe with a band collar and a buttoned placket, sandals; a white crocheted kufi cap
    front(x, y) { fill(x - 1, y, 3, 1, 0xe0dcd2); fill(x, y + 1, 1, 3, 0xe0dcd2); dot(x, y + 1, 0xa8adb6); dot(x, y + 3, 0xa8adb6); dot(x - 2, y + 2, 0xe0dcd2); fill(x + 3, y, 1, 6, 0xdcd8ce); },
    back(x, y) { fill(x - 1, y, 3, 1, 0xe0dcd2); fill(x + 3, y, 1, 6, 0xdcd8ce); },
    head(x, top) {
      const w = 0xf6f4ee, knit = 0xd8d4c8;
      fill(x - 2, top - 2, 5, 1, w); fill(x - 3, top - 1, 7, 2, w);
      for (let k = 0; k < 7; k += 2) dot(x - 3 + k, top, knit); dot(x - 1, top - 1, knit); dot(x + 1, top - 1, knit);
    },
  },
  muslimkurta: {
    // A long cream kurta and matching trousers under a deep green waistcoat with small gold buttons
    front(x, y) {
      fill(x - 3, y, 2, 6, 0x2f5a4a); fill(x + 2, y, 2, 6, 0x2f5a4a); dot(x - 3, y, 0x4a7a66); dot(x - 2, y + 2, 0xd8b45a); dot(x - 2, y + 4, 0xd8b45a);
      fill(x, y, 1, 3, 0xd8cfb4); dot(x, y + 1, 0xa89a7a);
    },
    back(x, y) { fill(x - 3, y, 7, 6, 0x2f5a4a); fill(x, y, 1, 6, 0x24483a); },
  },
  christianpendant: {
    // An oatmeal cable-knit sweater and dark trousers, a small gold cross on a fine chain at the neck
    front(x, y) {
      fill(x - 2, y, 5, 1, 0xd8ccb4); fill(x - 2, y + 2, 1, 4, 0xd4c8b0); fill(x + 2, y + 2, 1, 4, 0xd4c8b0);
      dot(x - 1, y + 1, 0xd8b45a, 0.6); dot(x + 1, y + 1, 0xd8b45a, 0.6);
      dot(x, y + 2, 0xe8c060); fill(x - 1, y + 3, 3, 1, 0xe8c060); dot(x, y + 4, 0xe8c060);
    },
    back(x, y) { fill(x - 2, y, 5, 1, 0xd8ccb4); fill(x - 2, y + 2, 1, 4, 0xd4c8b0); fill(x + 2, y + 2, 1, 4, 0xd4c8b0); },
  },
  christianchoir: {
    // A long burgundy choir robe with a white collar and a gold stole hanging down the front
    front(x, y, o) {
      fill(x - 2, y, 5, 1, 0xf6f4ee); dot(x, y + 1, 0xf6f4ee);
      fill(x - 2, y + 1, 1, 5, 0xd8b45a); fill(x + 2, y + 1, 1, 5, 0xd8b45a); fill(x, y + 2, 1, 4, shaded(o.shirt, 0.8));
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, 0xf6f4ee); fill(x, y + 1, 1, 5, shaded(o.shirt, 0.8)); },
  },
  christiansuit: {
    // A navy Sunday suit over a white shirt and a burgundy tie, a small gold pin on the lapel
    front(x, y, o) {
      fill(x - 1, y, 3, 3, 0xf6f4ee); fill(x, y, 1, 4, 0x8a2430); dot(x, y + 4, 0x6a1a24);
      dot(x - 2, y, shaded(o.shirt, 1.3)); dot(x + 2, y, shaded(o.shirt, 1.3)); dot(x - 2, y + 1, 0xd8b45a);
      dot(x + 1, y + 4, shaded(o.shirt, 0.7));
    },
    back(x, y, o) { fill(x, y + 2, 1, 4, shaded(o.shirt, 0.8)); },
  },
  christianlace: {
    // A blue floral dress to below the knee and a white lace veil laid lightly over the hair
    front(x, y) {
      for (const [dx, dy, c] of [[-2, 1, 0xf2a0b8], [1, 2, 0xf6f4ee], [-1, 4, 0xf6f4ee], [2, 5, 0xf2a0b8], [0, 0, 0xf6f4ee]]) dot(x + dx, y + dy, c);
      fill(x - 3, y + 3, 7, 1, 0x4a6a90);
    },
    back(x, y) { for (const [dx, dy] of [[-2, 2], [1, 1], [2, 4], [-1, 5]]) dot(x + dx, y + dy, 0xf2a0b8); fill(x - 3, y + 3, 7, 1, 0x4a6a90); },
    head(x, top, look, away) {
      const c = 0xf6f4ee, lace = 0xd8d4cc;
      fill(x - 3, top - 2, 7, 1, c, 0.85); fill(x - 4, top - 1, 9, 3, c, 0.85);
      fill(x - 4, top + 2, 1, 6, c, 0.8); fill(x + 4, top + 2, 1, 6, c, 0.8);
      if (away) fill(x - 3, top + 2, 7, 6, c, 0.8);
      for (const [dx, dy] of [[-2, -1], [0, -1], [2, -1], [-3, 1], [3, 1], [-1, 0], [1, 0]]) dot(x + dx, top + dy, lace);
    },
  },
  buddhistrobe: {
    // A long saffron robe in the monastic manner, its cloth wrapped across the body from the left shoulder in deeper folds
    front(x, y, o) {
      const D = shaded(o.shirt, 0.78), L = shaded(o.shirt, 1.15);
      line(x - 3, y, x + 3, y + 5, D); line(x - 2, y, x + 3, y + 4, L); fill(x - 3, y, 1, 6, L); dot(x + 3, y, D); dot(x + 2, y + 1, D);
    },
    back(x, y, o) { line(x + 3, y, x - 3, y + 5, shaded(o.shirt, 0.78)); fill(x + 3, y, 1, 6, shaded(o.shirt, 0.85)); },
  },
  buddhistmaroonrobe: {
    // A long maroon robe in the Himalayan monastic manner over a saffron-gold vest showing at the neck
    front(x, y, o) {
      const D = shaded(o.shirt, 0.75);
      fill(x - 1, y, 3, 2, 0xe8a848); dot(x, y + 2, 0xe8a848); line(x - 3, y, x + 2, y + 5, D); fill(x + 3, y, 1, 6, D); dot(x - 2, y + 1, shaded(o.shirt, 1.2));
    },
    back(x, y, o) { fill(x - 1, y, 3, 1, 0xe8a848); line(x + 3, y, x - 2, y + 5, shaded(o.shirt, 0.75)); },
  },
  buddhistlinen: {
    // A soft grey linen wrap top crossing over and tied at the side, and loose trousers
    front(x, y, o) {
      const D = shaded(o.shirt, 0.82);
      line(x - 3, y, x + 2, y + 4, D); line(x + 3, y, x, y + 3, D, 0.6); dot(x + 3, y + 4, 0x8a8478); dot(x + 3, y + 5, 0x8a8478); fill(x - 3, y + 5, 7, 1, D, 0.6);
    },
    back(x, y, o) { fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.82), 0.6); },
  },
  buddhistmala: {
    // A sage cardigan open over a cream tee, a wooden mala of beads hanging at the neck with a small saffron tassel
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0xf2e6d4); fill(x - 2, y, 1, 6, shaded(o.shirt, 0.8)); fill(x + 2, y, 1, 6, shaded(o.shirt, 0.8));
      for (const [dx, dy] of [[-2, 0], [-1, 1], [-1, 2], [0, 3], [1, 2], [1, 1], [2, 0]]) dot(x + dx, y + dy, 0x7a4a2a);
      dot(x, y + 4, 0xe8902a); dot(x, y + 5, 0xe8902a);
    },
    back(x, y, o) { fill(x, y + 2, 1, 4, shaded(o.shirt, 0.85)); dot(x - 2, y, 0x7a4a2a); dot(x + 2, y, 0x7a4a2a); },
  },
  ironmantony2: {
    // Tony Stark in the workshop (brand nod: Marvel): a charcoal band tee, a little print below the arc reactor glowing through it, dark
    // jeans; his own face, hair and anchor goatee, whoever wears it
    look: TONY_LOOK,
    front(x, y, o) {
      fill(x - 1, y + 4, 3, 1, 0x8a8e98, 0.55); dot(x, y + 4, 0xc83a3a, 0.8);
      imTonyReactor(x, y + 2);
    },
    back(x, y, o) { fill(x - 2, y + 1, 5, 2, 0x8a8e98, 0.3); },
    head(x, top, look, away) { imTony(x, top, look, away, 'a'); },
  },
  ironmantony2b: {
    // Tony Stark in his shades (brand nod: Marvel): a heather-grey band tee over the arc reactor, the red-tinted glasses on, a glint
    // crossing them now and then; his own face, hair and anchor goatee
    look: TONY_LOOK,
    front(x, y, o) {
      for (const dx of [-2, 0, 2]) dot(x + dx, y + 4, 0x1a1a1f, 0.7);
      imTonyReactor(x, y + 2);
    },
    back(x, y, o) { fill(x - 2, y + 1, 5, 2, 0x1a1a1f, 0.35); },
    head(x, top, look, away) { imTony(x, top, look, away, 'b'); },
  },
  ironmantony2c: {
    // Tony Stark, the main one (brand nod: Marvel): a sharp charcoal suit open over a white shirt, the arc reactor glowing through it, a red
    // pocket square; groomed hair grey at the temples, the anchor goatee
    look: TONY_LOOK,
    front(x, y, o) {
      fill(x - 1, y, 3, 3, 0xeceae4); dot(x, y + 3, 0xeceae4); dot(x - 1, y, 0xffffff);
      dot(x - 2, y, shaded(o.shirt, 1.35)); dot(x + 2, y, shaded(o.shirt, 1.35)); dot(x - 2, y + 1, shaded(o.shirt, 1.2)); dot(x + 2, y + 1, shaded(o.shirt, 0.8));
      dot(x - 2, y + 2, 0xc83a3a); dot(x + 1, y + 4, 0x8a8e98, 0.6); dot(x + 1, y + 5, 0x8a8e98, 0.6);
      imTonyReactor(x, y + 2, isNight ? 0.7 : 0.3);
      if (isNight) glowLate(() => { glowAt(x, y + 2, 4, 4, IM_ARC, 0.18); dot(x - 1, y + 2, IM_ARC, 0.8); dot(x + 1, y + 2, IM_ARC, 0.8); dot(x, y + 1, IM_ARC, 0.8); dot(x, y + 3, IM_ARC, 0.8); });
    },
    back(x, y, o) { fill(x, y + 3, 1, 3, shaded(o.shirt, 0.8)); },
    head(x, top, look, away) { imTony(x, top, look, away, 'c'); },
  },
  ironmanmk1: imArmour({
    // Mark I (brand nod: Marvel): the scrap-iron suit Tony hammered together in the cave, bolted grey plates with wide riveted shoulders, a
    // bucket helmet with dark slots for eyes and the first reactor glowing dimly; now and then the flamethrower on its arm coughs fire
    helm: 0x7a7a74, plate: 0x8e8e86, body: 0x6a6a66, trim: 0x54544e, hands: 0x5a5a54, eyes: 'dark', reactor: 'dim', pads: true,
    chest(x, y) {
      fill(x - 3, y + 4, 7, 1, 0x3e3e3a); for (const [dx, dy] of [[-2, 2], [2, 2], [-2, 5], [2, 5], [0, 5]]) dot(x + dx, y + dy, 0x9e9e96);
    },
    crest(x, top) { for (const dy of [2, 4]) { dot(x - 3, top + dy, 0x9e9e96); dot(x + 3, top + dy, 0x8e8e86); } fill(x - 1, top + 5, 3, 1, 0x2a2a28); },
    rear(x, y) { fill(x - 2, y + 2, 5, 2, 0x4e4e4a); dot(x, y + 2, 0x9e9e96); },
    hands2(x, top, raised, look) {
      if (raised || every(look, 19, 1.4, 9) === null) return;
      const f = Math.floor(now * 10) % 2;
      dot(x + 5, top + 12, 0xffd060); dot(x + 6, top + 12 - f, 0xf8a030); dot(x + 7, top + 12, 0xe8502a, 0.8); dot(x + 6, top + 11 + f, 0xe8502a, 0.6);
      if (isNight) glowLate(() => glowAt(x + 6, top + 12, 3, 2, 0xf8a030, 0.25));
    },
  }),
  ironmanmk4: imArmour({
    // Mark IV (brand nod: Marvel): hot-rod red with gold arms, a gold band down the belly under the round reactor, gold faceplate
    helm: IM_RED, plate: IM_GOLD, body: IM_RED, trim: IM_GOLD, hands: IM_GOLD, reactor: 'round',
    chest(x, y) { fill(x - 1, y + 4, 3, 2, IM_GOLD); dot(x, y + 4, IM_GOLD_DARK); dot(x, y + 5, IM_GOLD_DARK); dot(x - 1, y + 4, IM_GOLD_LIGHT); dot(x - 3, y, IM_GOLD_LIGHT); dot(x + 3, y, IM_GOLD_DARK); },
  }),
  ironmanmk5: imArmour({
    // Mark V, the suitcase suit (brand nod: Marvel): red with brushed-silver arms, collar and belt instead of gold, a silver faceplate; the
    // case's handle still on its back
    helm: IM_RED, plate: 0xb8bcc6, body: IM_RED, trim: 0x9aa0aa, hands: 0x8a8e98, reactor: 'round',
    chest(x, y) { fill(x - 2, y, 5, 1, 0x9aa0aa); dot(x - 2, y, 0xc8ccd4); fill(x - 3, y + 5, 7, 1, 0x8a8e98); dot(x, y + 5, 0x5a5e68); },
    rear(x, y) { fill(x - 1, y, 3, 1, 0x5a5e68); dot(x - 1, y + 1, 0x5a5e68); dot(x + 1, y + 1, 0x5a5e68); },
  }),
  ironmanmk6: imArmour({
    // Mark VI (brand nod: Marvel): red and gold, gold shoulders and belt, and the new triangular reactor in its chest
    helm: IM_RED, plate: IM_GOLD, body: IM_RED, trim: IM_GOLD, hands: IM_GOLD, reactor: 'triangle',
    chest(x, y) { fill(x - 3, y, 2, 1, IM_GOLD_LIGHT); fill(x + 2, y, 2, 1, IM_GOLD); fill(x - 3, y + 5, 7, 1, IM_GOLD); dot(x, y + 5, IM_GOLD_DARK); dot(x - 1, y + 3, IM_RED_LIGHT); },
  }),
  ironmanmk7: imArmour({
    // Mark VII, the Avengers battle suit (brand nod: Marvel): bulkier red and gold, broad gold shoulder plates and pecs, gold arms, and
    // twin jets on its back
    helm: IM_RED, plate: IM_GOLD, body: IM_RED, trim: IM_GOLD, hands: IM_GOLD, reactor: 'round', pads: true,
    chest(x, y) { fill(x - 2, y + 2, 1, 2, IM_GOLD); fill(x + 2, y + 2, 1, 2, IM_GOLD_DARK); fill(x - 1, y + 5, 3, 1, IM_GOLD); },
    rear(x, y) { fill(x - 2, y + 2, 2, 3, IM_GOLD_DARK); fill(x + 1, y + 2, 2, 3, IM_GOLD_DARK); dot(x - 1, y + 5, IM_ARC); dot(x + 1, y + 5, IM_ARC); },
  }),
  ironmanmk15: imArmour({
    // Mark XV "Sneaky" (brand nod: Marvel): the stealth suit in matte slate and pale steel panels; now and then a shimmer of cloaking
    // slides down it
    helm: 0x666c7c, plate: 0x9aa4b4, body: 0x5a6070, trim: 0x8a94a4, hands: 0x8a94a4, reactor: 'round',
    chest(x, y, look) {
      fill(x - 2, y + 1, 1, 4, 0x8a94a4, 0.6); fill(x + 2, y + 1, 1, 4, 0x8a94a4, 0.6); fill(x - 1, y + 5, 3, 1, 0x3e434e);
      const c = every(look, 21, 1.6, 5); if (c !== null) fill(x - 3, y - 7 + Math.floor(c * 13), 7, 1, WHITE, 0.35);
    },
  }),
  ironmanmk16: imArmour({
    // Mark XVI "Nightclub" (brand nod: Marvel): the black stealth suit, matte black with graphite panels and thin silver edges, a
    // black faceplate and white eyes
    helm: 0x24242e, plate: 0x34343e, body: 0x1a1a22, trim: 0x3a3a46, hands: 0x34343e, reactor: 'round',
    chest(x, y) { dot(x - 3, y, 0x7a7a88); dot(x + 3, y, 0x5a5a68); fill(x - 2, y + 4, 5, 1, 0x34343e); dot(x - 2, y + 4, 0x7a7a88); fill(x - 1, y + 5, 3, 1, 0x2a2a34); },
    crest(x, top) { dot(x - 2, top, 0x7a7a88); },
  }),
  ironmanmk17: imArmour({
    // Mark XVII "Heartbreaker" (brand nod: Marvel): silver-white plates with red arms, boots and helmet round a big round unibeam in its
    // chest, which charges up bright now and then
    helm: IM_RED, plate: 0xd8dce4, body: 0xc0c4cc, trim: IM_RED, hands: 0xd8dce4, reactor: 'big', at: 2,
    chest(x, y) { dot(x - 3, y, IM_RED); dot(x + 3, y, IM_RED); fill(x - 2, y + 5, 5, 1, 0x8a8e98); },
    pulse(look) { const c = every(look, 23, 1.5, 4); return c === null ? 1 : 2.2; },
  }),
  ironmanmk33: imArmour({
    // Mark XXXIII "Silver Centurion" (brand nod: Marvel): polished chrome with crimson shoulder plates and a crimson V across the chest,
    // a silver faceplate on a chrome helmet
    helm: 0xc8ccd4, plate: 0xe4e8ee, body: 0xbcc0c8, trim: 0x9a1a2a, hands: 0x9a1a2a, reactor: 'round', at: 3, pads: true,
    chest(x, y) { for (const [dx, dy] of [[-2, 2], [-1, 3], [1, 3], [2, 2]]) dot(x + dx, y + dy, 0x9a1a2a); dot(x - 2, y + 4, WHITE, 0.6); fill(x - 1, y + 5, 3, 1, 0x8a8e98); },
    crest(x, top) { dot(x, top, 0x9a1a2a); dot(x, top + 1, 0x9a1a2a); },
  }),
  ironmanmk38: imArmour({
    // Mark XXXVIII "Igor" (brand nod: Marvel): the heavy lifter, a hulking gunmetal suit with huge shoulders, yellow warning marks and
    // great fists
    helm: 0x5a5e66, plate: 0x8a8e96, body: 0x4e525a, trim: 0x6a6e76, hands: 0x6a6e76, reactor: 'round', pads: true,
    chest(x, y) { dot(x - 2, y + 4, 0xe8c030); dot(x + 2, y + 4, 0xe8c030); fill(x - 1, y + 5, 3, 1, 0x2e3036); },
    crest(x, top) { dot(x - 3, top + 2, 0xe8c030); dot(x + 3, top + 2, 0xc8a020); },
    rear(x, y) { fill(x - 2, y + 1, 5, 3, 0x3a3e46); dot(x, y + 2, 0xe8c030); },
    hands2(x, top, raised) {
      fill(x - 5, top + 11, 2, 2, 0x6a6e76); dot(x - 5, top + 11, 0x8a8e96);
      if (!raised) { fill(x + 4, top + 11, 2, 2, 0x6a6e76); dot(x + 5, top + 12, 0x4a4e56); }
    },
  }),
  ironmanmk42: imArmour({
    // Mark XLII (brand nod: Marvel): the gold suit, mostly gold with red at the chest's sides, the helmet's crown and the boots
    helm: 0xc89a30, plate: 0xf0cc60, body: 0xd4a83a, trim: IM_RED, hands: IM_RED, reactor: 'round',
    chest(x, y) { fill(x - 3, y, 2, 3, IM_RED); fill(x + 2, y, 2, 3, IM_RED_DARK); dot(x - 3, y, IM_RED_LIGHT); fill(x - 1, y + 5, 3, 1, 0xa8802a); },
    crest(x, top) { fill(x - 1, top, 3, 2, IM_RED); dot(x - 1, top, IM_RED_LIGHT); },
  }),
  ironmanmk43: imArmour({
    // Mark XLIII (brand nod: Marvel): Age of Ultron's red and gold, a gold collar and a gold ladder of plates down the belly
    helm: IM_RED, plate: IM_GOLD, body: IM_RED, trim: IM_GOLD, hands: IM_GOLD, reactor: 'round',
    chest(x, y) { fill(x - 2, y, 5, 1, IM_GOLD); dot(x - 2, y, IM_GOLD_LIGHT); dot(x - 1, y + 4, IM_GOLD); dot(x + 1, y + 4, IM_GOLD); fill(x - 1, y + 5, 3, 1, IM_GOLD); dot(x, y + 4, IM_GOLD_DARK); },
  }),
  ironmanmk45: imArmour({
    // Mark XLV (brand nod: Marvel): sleeker, deep crimson with fine gold lines sweeping in from the shoulders to the reactor
    helm: 0x8c1018, plate: IM_GOLD_LIGHT, body: 0x8c1018, trim: IM_GOLD, hands: IM_GOLD, reactor: 'round',
    chest(x, y) { for (const [dx, dy] of [[-3, 0], [-2, 1], [3, 0], [2, 1], [-2, 4], [2, 4]]) dot(x + dx, y + dy, IM_GOLD); fill(x - 1, y + 5, 3, 1, IM_GOLD_DARK); },
    crest(x, top) { dot(x - 1, top + 1, IM_GOLD); dot(x + 1, top + 1, IM_GOLD); },
  }),
  ironmanmk46: imArmour({
    // Mark XLVI, Civil War (brand nod: Marvel): red over dark graphite, graphite arms, legs and side plates, a gold faceplate
    helm: IM_RED, plate: IM_GOLD, body: IM_RED, trim: 0x3a3e48, hands: 0x5a5e68, reactor: 'round',
    chest(x, y) { fill(x - 3, y + 2, 1, 4, 0x3a3e48); fill(x + 3, y + 2, 1, 4, 0x2a2c34); fill(x - 1, y + 4, 3, 2, 0x3a3e48); dot(x, y + 4, 0x6e727c); },
    crest(x, top) { dot(x - 3, top + 4, 0x3a3e48); dot(x + 3, top + 4, 0x2a2c34); },
  }),
  ironmanmk47: imArmour({
    // Mark XLVII, Homecoming (brand nod: Marvel): candy red with polished chrome instead of gold, a chrome faceplate and chrome abs
    helm: 0xc8141e, plate: 0xd0d6de, body: 0xc8141e, trim: 0xd0d6de, hands: 0xd0d6de, reactor: 'round',
    chest(x, y) { fill(x - 1, y + 4, 3, 2, 0xd0d6de); dot(x - 1, y + 4, WHITE); dot(x + 1, y + 5, 0x8a8e98); dot(x - 3, y, 0xe83a40); },
    crest(x, top) { dot(x - 1, top, WHITE, 0.6); },
  }),
  ironmanmk50: imArmour({
    // Mark L "Bleeding Edge" (brand nod: Marvel): Infinity War's nanotech suit, deep red with gold lines round a hexagonal housing; every
    // 20 s the helmet flows away into the collar to show Tony, and flows back
    helm: 0x9a0e1a, plate: IM_GOLD, body: 0x9a0e1a, trim: IM_GOLD, hands: IM_GOLD, reactor: 'hex',
    chest(x, y) { for (const [dx, dy] of [[-2, 0], [2, 0], [-2, 3], [2, 3], [-1, 4], [1, 4]]) dot(x + dx, y + dy, IM_GOLD); dot(x, y + 5, IM_GOLD_DARK); },
    helmet(x, top, look) {
      const r = every(look, 20, 3.5, 11);
      if (r === null) return;
      fill(x - 3, top + 2, 7, 5, TONY_SKIN); dot(x - 2, top + 4, PALETTE.eye); dot(x + 2, top + 4, PALETTE.eye);
      imTony(x, top, look, false, 'c');
      // The nanites: up over his face from the collar, and down again
      const cover = r < 0.15 ? 1 - r / 0.15 : r > 0.85 ? (r - 0.85) / 0.15 : 0, rows = Math.round(cover * 8);
      if (rows > 0) { fill(x - 3, top + 7 - rows, 7, rows, 0x9a0e1a); for (let k = 0; k < 7; k += 2) dot(x - 3 + k, top + 6 - rows, IM_GOLD, 0.8); }
      return 'off';
    },
  }),
  ironmanmk85: imArmour({
    // Mark LXXXV, Endgame (brand nod: Marvel): rounded red and bright gold, a gold yoke round the reactor; every 29 s he raises his right
    // hand and the six stones flash on its gauntlet: the snap
    helm: IM_RED, plate: IM_GOLD_LIGHT, body: IM_RED, trim: IM_GOLD, hands: IM_GOLD, reactor: 'round',
    chest(x, y) { fill(x - 2, y, 5, 1, IM_GOLD); fill(x - 2, y + 1, 1, 2, IM_GOLD); fill(x + 2, y + 1, 1, 2, IM_GOLD_DARK); fill(x - 1, y + 4, 3, 2, IM_GOLD); dot(x, y + 5, IM_GOLD_DARK); },
    raise(look) { return every(look, 29, 2.5, 13) !== null; },
    hands2(x, top, raised) {
      if (!raised) return;
      const stones = [0x8a3ae8, 0x3a6ae8, 0xe83a3a, 0xe8a03a, 0x3ae86a, 0xe8d83a];
      stones.forEach((c, k) => dot(x + 4 + (k % 3), top + 1 + Math.floor(k / 3) * (k % 2 ? 1 : 0) - (k > 2 ? 1 : 0), c));
      if (Math.floor(now * 6) % 2) dot(x + 5, top, WHITE);
      glowAt(x + 5, top + 1, 3, 3, 0xffe8a0, isNight ? 0.3 : 0.15);
    },
  }),
  ironmanpatriot: imArmour({
    // Iron Patriot (brand nod: Marvel): star-spangled armour, a blue chest with a white star above the reactor, red and white stripes at
    // the belly, red arms and legs, a silver faceplate
    helm: 0xb01c2a, plate: 0xd0d4dc, body: 0x1e3a8a, trim: 0xe8e8ec, hands: 0xd0d4dc, reactor: 'round', at: 3,
    chest(x, y) {
      dot(x, y, WHITE); fill(x - 1, y + 1, 3, 1, WHITE);
      fill(x - 3, y + 5, 7, 1, 0xb01c2a); for (const dx of [-2, 0, 2]) dot(x + dx, y + 5, 0xe8e8ec);
    },
    crest(x, top) { fill(x - 2, top, 5, 1, 0x2a4aa8); },
  }),
  ironmanironspider: imArmour({
    // Iron Spider (brand nod: Marvel): Peter's suit by Stark, red and dark blue with a gold spider on the chest and the big white eyes; every
    // 17 s four gold arms unfold from its back
    helm: 0xc0141c, plate: null, body: 0xc0141c, trim: 0xd8a838, hands: 0xc0141c,
    chest(x, y, look) {
      fill(x - 3, y + 2, 1, 4, 0x1a2a5a); fill(x + 3, y + 2, 1, 4, 0x14204a);
      fill(x, y + 1, 1, 2, 0xd8a838); for (const [dx, dy] of [[-1, 0], [1, 0], [-1, 3], [1, 3]]) dot(x + dx, y + dy, 0xd8a838, 0.75);
      if (every(look, 17, 4, 3) !== null) {
        const c = 0xd8a838;
        line(x - 2, y + 1, x - 6, y - 3, c); line(x + 2, y + 1, x + 6, y - 3, c); line(x - 3, y + 3, x - 7, y + 5, c); line(x + 3, y + 3, x + 7, y + 5, c);
      }
    },
    helmet(x, top) {
      fill(x - 2, top, 5, 1, 0xd83a40); fill(x - 3, top + 1, 7, 6, 0xc0141c); fill(x + 3, top + 1, 1, 6, 0x8a0e14);
      for (const [dx, dy] of [[0, 1], [0, 5], [-2, 6], [2, 6]]) dot(x + dx, top + dy, 0x7a0c12, 0.8);
      fill(x - 3, top + 2, 3, 3, 0x1a1a1f); fill(x + 1, top + 2, 3, 3, 0x1a1a1f);
      const eye = isNight ? 0xe8f4ff : WHITE;
      fill(x - 2, top + 3, 2, 1, eye); dot(x - 2, top + 2, eye); fill(x + 1, top + 3, 2, 1, eye); dot(x + 2, top + 2, eye);
      return 'off';
    },
  }),
  ironmanrescue: imArmour({
    // Rescue (brand nod: Marvel): Pepper's suit, royal blue with silver plates and lines, a silver-blue faceplate and a slim silver fin on
    // the helmet; blue glowing eyes
    helm: 0x2a3c8c, plate: 0x9aa4bc, body: 0x2a3c8c, trim: 0xb8c0d0, hands: 0xb8c0d0, reactor: 'round', eye: 0xcfe8ff,
    chest(x, y) { dot(x - 3, y, 0xb8c0d0); dot(x + 3, y, 0x8a94ac); dot(x - 2, y + 4, 0xb8c0d0); dot(x + 2, y + 4, 0xb8c0d0); fill(x - 1, y + 5, 3, 1, 0xb8c0d0); dot(x, y + 5, 0x6a7490); },
    crest(x, top) { fill(x, top - 1, 1, 3, 0xb8c0d0); dot(x, top - 1, WHITE, 0.7); },
  }),
  volcanojumpsuit: {
    // An orange henchman's jumpsuit zipped up the front, a black utility belt with a yellow buckle, and a grey peaked cap
    front(x, y, o) {
      fill(x, y, 1, 4, shaded(o.shirt, 0.75)); dot(x, y, 0xd8d8d8); fill(x - 3, y + 4, 7, 1, 0x2a2a30); dot(x, y + 4, 0xf2c830);
      dot(x - 2, y + 1, shaded(o.shirt, 1.2)); dot(x + 2, y + 1, shaded(o.shirt, 0.8)); dot(x + 2, y + 2, shaded(o.shirt, 0.8));
    },
    back(x, y) { fill(x - 3, y + 4, 7, 1, 0x2a2a30); },
    head(x, top) { fill(x - 3, top - 1, 7, 2, 0x4a4e58); fill(x - 3, top - 1, 7, 1, 0x6e727c); fill(x - 4, top + 1, 9, 1, 0x3a3c44); },
  },
  volcanovillain: {
    // A villain's grey suit with a high mandarin collar buttoned to the throat, a row of dark buttons and a red flower in the lapel
    front(x, y, o) {
      fill(x - 2, y, 5, 1, shaded(o.shirt, 1.25)); for (const dy of [1, 3, 5]) dot(x, y + dy, 0x2a2a30);
      dot(x - 2, y + 1, 0xd8281e); dot(x - 1, y + 1, 0xa81e18); fill(x + 2, y + 1, 1, 5, shaded(o.shirt, 0.8));
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, shaded(o.shirt, 1.25)); },
  },
  volcanoheatsuit: {
    // A silver heat suit of quilted aluminised cloth for the lava floor, its hood up with a gold visor that catches the light
    front(x, y, o) {
      for (const dy of [1, 3, 5]) fill(x - 3, y + dy, 7, 1, shaded(o.shirt, 0.85));
      dot(x - 2, y, WHITE, 0.6); fill(x - 1, y + 2, 3, 2, 0x8a8e98);
    },
    back(x, y, o) { for (const dy of [1, 3, 5]) fill(x - 3, y + dy, 7, 1, shaded(o.shirt, 0.85)); },
    head(x, top, look, away) {
      const silver = 0xc8ccd4, dark = 0x9a9ea8;
      fill(x - 4, top - 1, 9, 3, silver); fill(x - 3, top - 2, 7, 1, silver); fill(x - 4, top + 2, 1, 6, dark); fill(x + 4, top + 2, 1, 6, dark);
      if (away) { fill(x - 3, top + 2, 7, 6, silver); return; }
      fill(x - 3, top + 2, 7, 3, 0xd8a838); fill(x - 3, top + 2, 7, 1, 0xf2d06a); dot(x - 2, top + 3, WHITE, 0.7);
    },
  },
  volcanoguard: {
    // A lair guard's black uniform with red piping at the collar and shoulders, a red armband and a grey belt, and a black beret
    front(x, y) {
      dot(x - 1, y, 0xc8281e); dot(x + 1, y, 0xc8281e); dot(x - 3, y, 0xc8281e); dot(x + 3, y, 0xc8281e);
      fill(x - 3, y + 2, 1, 2, 0xc8281e); fill(x - 3, y + 4, 7, 1, 0x5a5e68); dot(x, y + 4, 0xa8adb6); dot(x, y + 2, 0x4a4a52);
    },
    back(x, y) { dot(x - 3, y, 0xc8281e); dot(x + 3, y, 0xc8281e); fill(x - 3, y + 4, 7, 1, 0x5a5e68); },
    head(x, top) { fill(x - 3, top - 1, 7, 2, 0x1e1e22); fill(x + 1, top - 2, 3, 1, 0x1e1e22); dot(x - 2, top, 0xc8281e); },
  },
  kitchenchef: {
    // The head chef: a white double-breasted jacket with two columns of cloth buttons, a thin black trim at the collar, and a
    // tall pleated toque
    front(x, y, o) {
      fill(x - 2, y, 5, 1, 0x2a2a2e, 0.7); for (const dy of [1, 3, 5]) { dot(x - 1, y + dy, 0xc8c8c4); dot(x + 2, y + dy, 0xc8c8c4); }
      line(x - 2, y + 1, x + 2, y + 5, shaded(o.shirt, 0.88)); dot(x + 3, y + 2, shaded(o.shirt, 0.85));
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, 0x2a2a2e, 0.5); fill(x, y + 1, 1, 5, shaded(o.shirt, 0.9)); },
    head(x, top) {
      fill(x - 3, top - 1, 7, 2, 0xf6f6f2); fill(x - 3, top, 7, 1, 0xd8d8d4);
      fill(x - 3, top - 7, 7, 6, 0xffffff); fill(x - 4, top - 7, 9, 2, 0xffffff); fill(x - 3, top - 8, 7, 1, 0xf6f6f2);
      for (const px of [-2, 0, 2]) fill(x + px, top - 6, 1, 5, 0xdcdcd8);
    },
  },
  kitchensous: {
    // The sous-chef: a white jacket with black piping down the front and on the collar, a folded side towel tucked at the hip,
    // and a black skull cap
    front(x, y) {
      fill(x - 2, y, 5, 1, 0x2a2a2e); line(x - 1, y + 1, x + 2, y + 5, 0x2a2a2e); for (const dy of [2, 4]) dot(x + 2, y + dy - 1, 0x2a2a2e);
      fill(x - 3, y + 4, 2, 2, 0xe8eef6); dot(x - 3, y + 5, 0x4c6aa8);
    },
    back(x, y) { fill(x - 2, y, 5, 1, 0x2a2a2e); fill(x + 2, y + 4, 2, 2, 0xe8eef6); },
    head(x, top) { fill(x - 3, top - 1, 7, 3, 0x1e1e22); fill(x - 3, top - 1, 7, 1, 0x3a3a40); },
  },
  kitchenpartie: {
    // A chef de partie on the line: a black chef's jacket with white cloth buttons, a long white bistro apron tied at the waist,
    // and a white bandana knotted round the head
    front(x, y, o) {
      for (const dy of [1, 3]) { dot(x - 1, y + dy, 0xe8e8e4); dot(x + 1, y + dy, 0xe8e8e4); }
      fill(x - 3, y + 4, 7, 2, 0xf6f6f2); fill(x - 3, y + 4, 7, 1, 0xd8d8d4); dot(x + 3, y + 4, 0xd8d8d4); dot(x - 2, y, shaded(o.shirt, 1.5));
    },
    back(x, y) { fill(x - 1, y + 4, 3, 1, 0xf6f6f2); dot(x, y + 5, 0xf6f6f2); },
    head(x, top) { fill(x - 3, top - 1, 7, 2, 0xf6f6f2); fill(x - 3, top, 7, 1, 0xd8d8d4); dot(x - 4, top + 1, 0xf6f6f2); dot(x - 5, top + 2, 0xf6f6f2); },
  },
  kitchencommis: {
    // The commis: a plain white jacket with a blue and white striped butcher's apron over it, and a white paper cap
    front(x, y) {
      fill(x - 2, y + 1, 5, 5, 0x4a6aa8); for (const dx of [-1, 1]) fill(x + dx, y + 1, 1, 5, 0xe8eef6); dot(x - 2, y, 0x4a6aa8); dot(x + 2, y, 0x4a6aa8);
    },
    back(x, y) { line(x - 2, y, x + 2, y + 4, 0x4a6aa8); line(x + 2, y, x - 2, y + 4, 0x4a6aa8); },
    head(x, top) { fill(x - 3, top - 2, 7, 3, 0xffffff); fill(x - 3, top, 7, 1, 0xdcdcd8); dot(x - 2, top - 3, 0xffffff); dot(x + 2, top - 3, 0xffffff); },
  },
  hospitalscrubs: {
    // Teal scrubs: a V-neck top with a breast pocket and a pen in it, and a teal surgical cap
    front(x, y, o) { const D = shaded(o.shirt, 0.75); dot(x - 1, y, D); dot(x, y + 1, D); dot(x + 1, y, D); fill(x + 1, y + 2, 2, 2, D, 0.6); dot(x + 2, y + 1, 0x2a2a30); },
    back() {},
    head(x, top) { fill(x - 3, top - 1, 7, 2, 0x3a9a9a); fill(x - 3, top - 1, 7, 1, 0x5ab8b8); },
  },
  hospitaldoctor: {
    // A doctor's white coat open over a pale blue shirt and a tie, a stethoscope round the neck and pens in the pocket
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0xb8d4f0); fill(x, y + 1, 1, 3, 0x2a4a8a); fill(x - 2, y, 1, 6, shaded(o.shirt, 0.85)); fill(x + 2, y, 1, 6, shaded(o.shirt, 0.85));
      dot(x - 2, y + 1, 0x2a2a30); dot(x - 2, y + 2, 0x2a2a30); dot(x + 2, y + 1, 0x2a2a30); dot(x + 1, y + 3, 0xa8adb6); dot(x + 3, y + 3, 0x4c8bf5);
    },
    back(x, y, o) { fill(x, y + 2, 1, 4, shaded(o.shirt, 0.88)); },
  },
  hospitalnurse: {
    // Navy nurse's scrubs with white piping at the collar and a little fob watch pinned on the chest
    front(x, y) { dot(x - 2, y, 0xf2f4f4); dot(x - 1, y + 1, 0xf2f4f4); dot(x + 1, y + 1, 0xf2f4f4); dot(x + 2, y, 0xf2f4f4); disc(x - 2, y + 3, 0.8, 0xc8ccd2); dot(x - 2, y + 2, 0xa8adb6); },
    back(x, y) { fill(x - 2, y, 5, 1, 0xf2f4f4, 0.6); },
  },
  hospitalsurgeon: {
    // A green surgical gown tied at the back, a blue mask over nose and mouth and a patterned surgical cap
    front(x, y, o) { fill(x - 3, y + 2, 7, 1, shaded(o.shirt, 0.85)); dot(x, y, shaded(o.shirt, 1.2)); },
    back(x, y) { fill(x, y, 1, 6, 0x4a7a64); dot(x - 1, y + 2, 0xf2f4f4); dot(x + 1, y + 2, 0xf2f4f4); },
    head(x, top, look, away) {
      fill(x - 3, top - 1, 7, 2, 0x4c8bf5); for (const dx of [-2, 0, 2]) dot(x + dx, top - 1, 0xf2d35b);
      if (!away) { fill(x - 3, top + 4, 7, 3, 0x8ac8e8); fill(x - 3, top + 4, 7, 1, 0xb8e0f4); }
    },
  },
  noirtrench: {
    // A belted beige trench coat with wide lapels and its collar turned up, and a grey felt fedora with a black band
    front(x, y, o) {
      const D = shaded(o.shirt, 0.78);
      line(x - 2, y, x, y + 3, D); line(x + 2, y, x, y + 3, D); dot(x, y + 1, 0xeeeae0); fill(x - 3, y + 4, 7, 1, D); dot(x + 1, y + 4, 0x8a6a3a);
      dot(x - 3, y, shaded(o.shirt, 1.15)); dot(x + 3, y, shaded(o.shirt, 1.15));
    },
    back(x, y, o) { fill(x - 3, y + 4, 7, 1, shaded(o.shirt, 0.78)); fill(x - 3, y, 7, 1, shaded(o.shirt, 1.1)); },
    head(x, top) { fill(x - 4, top + 1, 9, 1, 0x5a5a60); fill(x - 3, top - 2, 7, 3, 0x6a6a70); fill(x - 3, top, 7, 1, 0x1e1e22); fill(x - 1, top - 2, 3, 1, 0x5a5a60); },
  },
  noiruniform: {
    // A beat cop's navy uniform with brass buttons, a silver badge on the chest and a peaked cap with a badge of its own
    front(x, y) { for (const dy of [1, 3, 5]) dot(x, y + dy, 0xd8b45a); fill(x - 2, y + 1, 2, 2, 0xc8ccd2); dot(x - 2, y + 1, WHITE); fill(x - 3, y + 4, 7, 1, 0x121216); },
    back(x, y) { fill(x - 3, y + 4, 7, 1, 0x121216); },
    head(x, top) { fill(x - 3, top - 2, 7, 3, 0x1e2638); fill(x - 3, top - 2, 7, 1, 0x2e3a54); fill(x - 1, top + 1, 6, 1, 0x121216); dot(x, top - 1, 0xd8b45a); },
  },
  noirsuspenders: {
    // Shirtsleeves after midnight: a white shirt with the sleeves rolled, a loosened dark red tie and brown suspenders
    front(x, y) { fill(x - 2, y, 1, 6, 0x6a4428); fill(x + 2, y, 1, 6, 0x6a4428); fill(x, y + 1, 1, 3, 0x7a2424); dot(x, y, 0x5a1a1a); dot(x - 1, y, 0xd8d4c8); },
    back(x, y) { line(x - 2, y, x + 2, y + 5, 0x6a4428); line(x + 2, y, x - 2, y + 5, 0x6a4428); },
  },
  noirreporter: {
    // A crime reporter in a grey waistcoat over a pale shirt, a press card tucked in the band of a tweed newsboy cap
    front(x, y) { fill(x - 2, y + 1, 5, 5, 0x6a6a70); fill(x, y, 1, 6, 0xd8d4c8); for (const dy of [2, 4]) dot(x + 1, y + dy, 0x2a2a30); },
    back(x, y) { fill(x - 2, y + 1, 5, 5, 0x6a6a70); },
    head(x, top) { fill(x - 3, top - 1, 7, 2, 0x7a6a52); fill(x - 1, top + 1, 6, 1, 0x5a4a3a); dot(x - 2, top - 1, 0x9a8a6a); fill(x + 2, top - 3, 2, 2, 0xf2f0ea); },
  },
  madlabcoat: {
    // A long lab coat stained green and scorched at the cuffs, open over a purple waistcoat, and brass goggles pushed up on the forehead
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0x6a4a8a); dot(x, y + 2, 0xd8b45a); dot(x, y + 4, 0xd8b45a); fill(x - 2, y, 1, 6, shaded(o.shirt, 0.85)); fill(x + 2, y, 1, 6, shaded(o.shirt, 0.85));
      dot(x - 3, y + 3, 0x7ae04a, 0.8); dot(x + 3, y + 5, 0x7ae04a, 0.7); dot(x - 3, y + 5, 0x5a5048);
    },
    back(x, y, o) { fill(x, y + 2, 1, 4, shaded(o.shirt, 0.88)); dot(x + 2, y + 3, 0x7ae04a, 0.7); },
    head(x, top) { fill(x - 3, top + 1, 7, 1, 0x5a3b25); disc(x - 1, top + 1, 1, 0xc8a050); disc(x + 2, top + 1, 1, 0xc8a050); dot(x - 1, top + 1, 0x8ae0ff); dot(x + 2, top + 1, 0x8ae0ff); },
  },
  madlabapron: {
    // A long black rubber apron over a purple shirt, with elbow-length rubber gloves and a pair of dark welding goggles round the neck
    front(x, y) { fill(x - 2, y + 1, 5, 5, 0x1e1e22); fill(x - 2, y + 1, 5, 1, 0x3a3a40); dot(x - 2, y, 0x1e1e22); dot(x + 2, y, 0x1e1e22); fill(x - 1, y, 3, 1, 0x3a3a40); dot(x - 1, y, 0x5a8a8a); dot(x + 1, y, 0x5a8a8a); },
    back(x, y) { line(x - 2, y, x + 2, y + 4, 0x1e1e22); line(x + 2, y, x - 2, y + 4, 0x1e1e22); },
  },
  madlabassistant: {
    // A lab assistant's patched brown cloak with a rope belt, a hood down at the back and a lantern-yellow patch on the shoulder
    front(x, y, o) { fill(x - 3, y + 4, 7, 1, 0xc8a070); dot(x + 1, y + 5, 0xc8a070); fill(x + 1, y + 1, 2, 2, 0x8a6a3a); dot(x - 2, y, shaded(o.shirt, 0.75)); dot(x + 2, y, shaded(o.shirt, 0.75)); },
    back(x, y, o) { fill(x - 3, y, 7, 2, shaded(o.shirt, 0.8)); fill(x - 3, y + 4, 7, 1, 0xc8a070); },
  },
  madlabhazmat: {
    // A yellow hazmat suit, its zip and seams taped in black, the hood up with a wide clear visor
    front(x, y, o) { fill(x, y, 1, 6, 0x2a2a30); fill(x - 3, y + 3, 7, 1, shaded(o.shirt, 0.8)); dot(x - 2, y + 1, 0x2a2a30); dot(x + 2, y + 1, 0x2a2a30); },
    back(x, y, o) { fill(x - 3, y + 3, 7, 1, shaded(o.shirt, 0.8)); },
    head(x, top, look, away) {
      const suit = 0xf2c830, dark = 0xc8a020;
      fill(x - 4, top - 2, 9, 3, suit); fill(x - 4, top + 1, 1, 7, dark); fill(x + 4, top + 1, 1, 7, dark);
      if (away) { fill(x - 3, top + 1, 7, 7, suit); return; }
      fill(x - 3, top + 1, 7, 5, 0xdff0f6, 0.35); fill(x - 3, top + 1, 7, 1, 0x2a2a30); dot(x - 2, top + 2, WHITE, 0.8);
    },
  },
  casinodealer: {
    // A dealer's black waistcoat over a crisp white shirt, a red bow tie, and red sleeve garters
    front(x, y) { fill(x - 3, y + 1, 2, 5, 0x1e1e22); fill(x + 2, y + 1, 2, 5, 0x1e1e22); fill(x - 1, y, 3, 1, 0xd8302a); dot(x, y + 1, 0xd8302a); for (const dy of [3, 5]) dot(x - 2, y + dy, 0xd8b45a); },
    back(x, y) { fill(x - 3, y + 1, 7, 5, 0x1e1e22); },
  },
  casinotux: {
    // A black tuxedo with satin lapels over a pleated white shirt, a black bow tie and a red rose in the buttonhole
    front(x, y) { fill(x - 1, y, 3, 5, 0xf6f6f2); dot(x, y + 2, 0xd8d8d4); dot(x, y + 4, 0xd8d8d4); fill(x - 1, y, 3, 1, 0x121214); dot(x - 2, y + 1, 0x3a3a42); dot(x + 2, y + 1, 0x3a3a42); dot(x - 2, y + 2, 0xd8302a); },
    back() {},
  },
  casinohighroller: {
    // A high roller's white suit open over a black shirt, a fat gold chain, and gold-rimmed dark glasses
    front(x, y) { fill(x - 1, y, 3, 6, 0x1e1e22); for (const [dx, dy] of [[-1, 1], [0, 2], [1, 1]]) dot(x + dx, y + dy, 0xe8c060); dot(x, y + 3, 0xf2d35b); },
    back() {},
    head(x, top, look, away) { if (away) return; fill(x - 3, top + 3, 7, 1, 0xd8b45a); fill(x - 3, top + 3, 3, 2, 0x1e1e22); fill(x + 1, top + 3, 3, 2, 0x1e1e22); },
  },
  casinoshowman: {
    // A showman's gold sequinned jacket that sparkles as it catches the light, black lapels and a black bow tie
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0xf6f6f2); fill(x - 1, y, 3, 1, 0x121214); dot(x - 2, y + 1, 0x1e1e22); dot(x + 2, y + 1, 0x1e1e22);
      const t = Math.floor(now * 3 + x); for (let k = 0; k < 3; k++) dot(x + (k % 2 ? 3 : -3), y + 1 + ((t + k * 2) % 5), WHITE, 0.9);
    },
    back(x, y) { const t = Math.floor(now * 3 + x); for (let k = 0; k < 3; k++) dot(x - 2 + ((t + k) % 5), y + 1 + k * 2, WHITE, 0.8); },
  },
  orientconductor: {
    // A conductor's navy uniform with two rows of brass buttons and gold braid at the cuffs and collar, a ticket punch on a
    // chain, and a peaked cap with a gold band
    front(x, y) { for (const dy of [1, 3, 5]) { dot(x - 1, y + dy, 0xd8b45a); dot(x + 1, y + dy, 0xd8b45a); } fill(x - 2, y, 5, 1, 0xd8b45a, 0.8); dot(x + 3, y + 3, 0xa8adb6); },
    back(x, y) { fill(x - 2, y, 5, 1, 0xd8b45a, 0.6); },
    head(x, top) { fill(x - 3, top - 2, 7, 3, 0x1e2a4a); fill(x - 3, top, 7, 1, 0xd8b45a); fill(x - 1, top + 1, 6, 1, 0x121214); dot(x, top - 1, 0xf2d888); },
  },
  orientattendant: {
    // A sleeping-car attendant's chestnut jacket with a high collar and brass buttons, white gloves, and a round pillbox cap
    front(x, y, o) { fill(x - 2, y, 5, 1, shaded(o.shirt, 1.25)); for (const dy of [1, 2, 3, 4, 5]) dot(x + 1, y + dy, 0xd8b45a); line(x - 2, y + 1, x - 1, y + 5, shaded(o.shirt, 0.8)); },
    back(x, y, o) { fill(x - 2, y, 5, 1, shaded(o.shirt, 1.25)); },
    head(x, top) { fill(x - 2, top - 2, 5, 3, 0x6a3420); fill(x - 2, top - 2, 5, 1, 0x8a4a2e); fill(x - 2, top, 5, 1, 0xd8b45a); },
  },
  orienttraveller: {
    // A gentleman traveller's tweed three-piece suit, a watch chain across the waistcoat, a silk cravat, and a brown bowler hat
    front(x, y, o) {
      fill(x - 1, y, 3, 2, 0xd8d4c8); fill(x, y + 1, 1, 2, 0x6a2a4a); fill(x - 1, y + 2, 3, 4, shaded(o.shirt, 0.8)); line(x - 1, y + 4, x + 1, y + 3, 0xd8b45a);
      for (const dy of [1, 3]) dot(x - 3, y + dy, shaded(o.shirt, 1.2));
    },
    back() {},
    head(x, top) { fill(x - 3, top - 3, 7, 3, 0x5a3b25); fill(x - 2, top - 4, 5, 1, 0x5a3b25); fill(x - 4, top, 9, 1, 0x4a2e1e); fill(x - 3, top - 1, 7, 1, 0x2a1e14); dot(x - 2, top - 3, 0x7a5232); },
  },
  orientpearls: {
    // A drop-waisted teal dress with a beaded hem line, a long rope of pearls looped twice, and a sequinned headband with a feather
    front(x, y) {
      for (const [dx, dy] of [[-2, 0], [-1, 1], [0, 2], [1, 1], [2, 0], [-1, 3], [0, 4], [1, 3]]) dot(x + dx, y + dy, 0xf6f2e8);
      fill(x - 3, y + 4, 7, 1, 0xd8b45a, 0.7);
    },
    back(x, y) { fill(x - 3, y + 4, 7, 1, 0xd8b45a, 0.7); dot(x, y, 0xf6f2e8); },
    head(x, top) { fill(x - 3, top + 1, 7, 1, 0x1e1e22); dot(x - 2, top + 1, 0xd8b45a); dot(x + 2, top + 1, 0xd8b45a); fill(x + 3, top - 3, 1, 4, 0xf2ead8); dot(x + 4, top - 4, 0xf2ead8); },
  },
  halloweendxvampire: {
    // A vampire's black tailcoat over a blood-red waistcoat and a white jabot, a ruby pin at the throat, and a cape with a
    // tall stiff red-lined collar standing up behind the head
    front(x, y) { fill(x - 1, y, 3, 6, 0x8a1424); fill(x, y, 1, 3, 0xf6f2e8); dot(x - 1, y + 1, 0xf6f2e8); dot(x + 1, y + 1, 0xf6f2e8); dot(x, y + 3, 0xd8203a); dot(x - 3, y, 0xc8203a); dot(x + 3, y, 0xc8203a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x121014); fill(x - 3, y, 7, 1, 0x2a1a20); },
    head(x, top, look, away) { if (away) return; fill(x - 5, top + 3, 1, 5, 0xa81424); fill(x + 5, top + 3, 1, 5, 0xa81424); fill(x - 5, top + 2, 1, 1, 0x1e1418); fill(x + 5, top + 2, 1, 1, 0x1e1418); },
  },
  halloweendxplague: {
    // A plague doctor's long waxed coat buttoned to the neck with a leather belt and a sprig of herbs, a wide-brimmed black
    // hat and the pale long-beaked mask with round glass eyes
    front(x, y, o) { for (const dy of [1, 3, 5]) dot(x, y + dy, 0x8a6a3a); fill(x - 3, y + 4, 7, 1, 0x4a3420); dot(x - 2, y + 2, 0x6a9a4a); dot(x - 2, y + 1, 0x4a7a3a); dot(x - 3, y, shaded(o.shirt, 1.3)); },
    back(x, y) { fill(x - 3, y + 4, 7, 1, 0x4a3420); },
    head(x, top, look, away) {
      fill(x - 5, top, 11, 1, 0x1e1a1a); fill(x - 3, top - 3, 7, 3, 0x1e1a1a); fill(x - 3, top - 1, 7, 1, 0x3a2a20);
      if (away) return;
      fill(x - 3, top + 1, 7, 5, 0xe8dcc0); disc(x - 1, top + 3, 1, 0x3a4a5a); disc(x + 2, top + 3, 1, 0x3a4a5a); dot(x - 1, top + 2, WHITE, 0.7);
      fill(x + 3 * (look || 1), top + 4, 3, 2, 0xe8dcc0); dot(x + 5 * (look || 1), top + 5, 0xd8ccb0);
    },
  },
  halloweendxscarecrow: {
    // A scarecrow's patched orange flannel shirt with straw poking out at the collar and cuffs, a rope belt, and a battered
    // brown sack hat with a crow's feather in it
    front(x, y, o) {
      fill(x - 3, y + 1, 7, 1, shaded(o.shirt, 0.75)); fill(x - 3, y + 3, 7, 1, shaded(o.shirt, 0.75)); fill(x + 1, y + 2, 2, 2, 0x4a6a8a);
      for (const [dx, dy] of [[-2, 0], [0, -1], [2, 0], [-3, 5], [3, 5]]) dot(x + dx, y + dy, 0xe8c860); fill(x - 3, y + 5, 7, 1, 0xc8a070);
    },
    back(x, y, o) { fill(x - 3, y + 2, 7, 1, shaded(o.shirt, 0.75)); fill(x - 3, y + 5, 7, 1, 0xc8a070); },
    head(x, top) { fill(x - 5, top, 11, 1, 0x7a5a32); fill(x - 3, top - 3, 7, 3, 0x8a6a3a); fill(x - 2, top - 4, 4, 1, 0x8a6a3a); dot(x - 2, top - 2, 0x5a3b25); fill(x + 3, top - 5, 1, 3, 0x1e1e22); dot(x + 4, top - 6, 0x1e1e22); for (const dx of [-4, 4]) dot(x + dx, top + 1, 0xe8c860); },
  },
  halloweendxspider: {
    // A black lace gown patterned with a silver spider web across the front, a purple sash, and a jewelled spider brooch
    front(x, y) {
      for (const [dx, dy] of [[0, 1], [0, 3], [0, 5], [-2, 2], [-3, 4], [2, 2], [3, 4], [-1, 3], [1, 3], [-2, 5], [2, 5]]) dot(x + dx, y + dy, 0xa8adb8, 0.8);
      fill(x - 3, y + 4, 7, 1, 0x6a3a8a); fill(x - 1, y, 3, 2, 0x3a1e4a); dot(x, y, 0xb88aff); dot(x - 1, y + 1, 0x1e1e22); dot(x + 1, y + 1, 0x1e1e22);
    },
    back(x, y) { fill(x - 3, y + 4, 7, 1, 0x6a3a8a); for (const [dx, dy] of [[0, 1], [-2, 2], [2, 2], [0, 5]]) dot(x + dx, y + dy, 0xa8adb8, 0.7); },
  },
  catcafebarista: {
    // A café barista's white shirt under a dusky pink apron with a white paw print on its bib and a pen in its pocket
    front(x, y) { fill(x - 2, y + 1, 5, 5, 0xd88a9a); dot(x - 2, y, 0xd88a9a); dot(x + 2, y, 0xd88a9a); dot(x, y + 3, WHITE); for (const [dx, dy] of [[-1, 2], [1, 2], [0, 2]]) dot(x + dx, y + dy, WHITE, 0.8); dot(x + 2, y + 2, 0x2a2a30); },
    back(x, y) { line(x - 2, y, x + 2, y + 4, 0xd88a9a); line(x + 2, y, x - 2, y + 4, 0xd88a9a); },
  },
  catcafeears: {
    // A pink hoodie with a kangaroo pocket and the hood up, two little cat ears on top of it
    front(x, y, o) { fill(x - 2, y + 3, 5, 2, shaded(o.shirt, 0.85)); dot(x - 1, y, 0xf6f2ea); dot(x + 1, y, 0xf6f2ea); },
    back(x, y, o) { fill(x - 3, y, 7, 2, shaded(o.shirt, 0.9)); },
    head(x, top) { const c = 0xf2a0b8; fill(x - 4, top - 1, 9, 3, c); fill(x - 4, top + 2, 1, 5, c); fill(x + 4, top + 2, 1, 5, c); dot(x - 3, top - 2, c); dot(x - 2, top - 3, c); dot(x + 3, top - 2, c); dot(x + 2, top - 3, c); dot(x - 2, top - 2, 0xf8d0dc); dot(x + 2, top - 2, 0xf8d0dc); },
  },
  catcafejumper: {
    // A teal knitted jumper with a big cream cat face on the front, its ears, eyes and whiskers knitted in
    front(x, y) { fill(x - 2, y + 2, 5, 3, 0xf2e8d8); dot(x - 2, y + 1, 0xf2e8d8); dot(x + 2, y + 1, 0xf2e8d8); dot(x - 1, y + 3, 0x2a2a30); dot(x + 1, y + 3, 0x2a2a30); dot(x, y + 4, 0xf28fb0); dot(x - 3, y + 4, 0xf2e8d8); dot(x + 3, y + 4, 0xf2e8d8); },
    back(x, y, o) { for (let k = 0; k < 6; k += 2) fill(x - 3, y + k, 7, 1, shaded(o.shirt, 0.88)); },
  },
  catcafeoveralls: {
    // Blue denim dungarees over a yellow tee, a fish-shaped patch on the bib pocket and brass buckles
    front(x, y) { fill(x - 2, y + 2, 5, 4, 0x4a6aa8); fill(x - 2, y, 1, 2, 0x4a6aa8); fill(x + 2, y, 1, 2, 0x4a6aa8); dot(x - 2, y + 2, 0xd8b45a); dot(x + 2, y + 2, 0xd8b45a); fill(x - 1, y + 3, 2, 1, 0xf28fb0); dot(x + 1, y + 3, 0xf28fb0); dot(x - 1, y + 3, 0x2a2a30); },
    back(x, y) { line(x - 2, y, x + 2, y + 2, 0x4a6aa8); line(x + 2, y, x - 2, y + 2, 0x4a6aa8); fill(x - 2, y + 3, 5, 3, 0x4a6aa8); },
  },
  spiderversemiles: {
    // Miles's suit: black, with a red spider across the chest and fine red webbing over the shoulders
    front(x, y, o) {
      const red = 0xd8202a;
      fill(x, y + 1, 1, 4, red); dot(x - 1, y + 2, red); dot(x + 1, y + 2, red); dot(x - 2, y + 1, red); dot(x + 2, y + 1, red); dot(x - 1, y + 4, red); dot(x + 1, y + 4, red);
      dot(x - 3, y, red, 0.6); dot(x + 3, y, red, 0.6); dot(x - 3, y + 3, red, 0.4); dot(x + 3, y + 3, red, 0.4);
    },
    back(x, y, o) { fill(x - 2, y + 1, 5, 1, 0xd8202a, 0.6); fill(x, y, 1, 6, 0xd8202a, 0.5); },
  },
  spiderversegwensuit: {
    // Gwen's suit: white with a pink-lined hood at the neck, teal lines down the sides and a small black spider on the chest
    front(x, y, o) {
      fill(x - 2, y, 5, 1, 0xf28fb0); dot(x - 3, y, 0xe06a98); dot(x + 3, y, 0xe06a98);
      fill(x - 3, y + 1, 1, 5, 0x5ac8c8); fill(x + 3, y + 1, 1, 5, 0x5ac8c8);
      fill(x, y + 2, 1, 2, 0x1c1a22); dot(x - 1, y + 2, 0x1c1a22); dot(x + 1, y + 2, 0x1c1a22); dot(x - 1, y + 4, 0x1c1a22); dot(x + 1, y + 4, 0x1c1a22);
    },
    back(x, y, o) { fill(x - 2, y, 5, 2, 0xf4f2ee); fill(x - 1, y, 3, 1, 0xf28fb0); fill(x - 3, y + 1, 1, 5, 0x5ac8c8); fill(x + 3, y + 1, 1, 5, 0x5ac8c8); },
  },
  spiderversepeterb: {
    // Peter B.'s worn red suit with blue sides and a black spider, a bit of a belly, and his old brown bathrobe hanging open over it
    front(x, y, o) {
      const robe = 0x8a5a3a;
      fill(x - 3, y, 1, 6, robe); fill(x + 3, y, 1, 6, robe); dot(x - 2, y, robe); dot(x + 2, y, robe);
      fill(x, y + 1, 1, 3, 0x1c1a22); dot(x - 1, y + 2, 0x1c1a22); dot(x + 1, y + 2, 0x1c1a22);
      fill(x - 1, y + 4, 3, 2, shaded(o.shirt, 1.12)); dot(x - 2, y + 5, 0x2a4ab0); dot(x + 2, y + 5, 0x2a4ab0);
      dot(x - 3, y + 4, 0x6a4028); dot(x + 3, y + 4, 0x6a4028);
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x8a5a3a); fill(x - 3, y + 3, 7, 1, 0x6a4028); },
  },
  spiderversepunk: {
    // Spider-Punk's sleeveless denim vest: spikes along the shoulders, safety pins, a red patch with a white cross and a
    // little guitar pin
    front(x, y, o) {
      const steel = 0xc8ccd4;
      dot(x - 3, y - 1, steel); dot(x - 2, y - 1, steel); dot(x + 2, y - 1, steel); dot(x + 3, y - 1, steel);
      fill(x - 1, y, 3, 6, 0x1c1a22); dot(x, y + 1, 0xd8202a); dot(x, y + 3, 0xd8202a);
      fill(x - 3, y + 2, 2, 2, 0xd8202a); dot(x - 3, y + 2, WHITE); dot(x - 2, y + 3, WHITE);
      dot(x + 2, y + 1, steel); dot(x + 3, y + 2, steel); dot(x + 2, y + 4, 0xf2d35b); dot(x + 3, y + 5, 0xf2d35b);
    },
    back(x, y, o) { for (const dx of [-3, -1, 1, 3]) dot(x + dx, y - 1, 0xc8ccd4); fill(x - 2, y + 2, 5, 2, 0xd8202a); fill(x - 1, y + 2, 3, 1, WHITE); },
  },
  xmenwolverine: {
    // Wolverine's yellow suit with blue shoulders sweeping down the sides, black stripes and a red belt with an X buckle
    front(x, y, o) {
      const blue = 0x2a4ab0;
      fill(x - 3, y, 2, 2, blue); fill(x + 2, y, 2, 2, blue); dot(x - 3, y + 2, blue); dot(x + 3, y + 2, blue);
      dot(x - 3, y + 3, 0x1c1a22); dot(x + 3, y + 3, 0x1c1a22); dot(x - 2, y + 4, 0x1c1a22); dot(x + 2, y + 4, 0x1c1a22);
      fill(x - 3, y + 5, 7, 1, 0xd8202a); dot(x, y + 5, 0xf2c41a);
    },
    back(x, y, o) { fill(x - 3, y, 7, 2, 0x2a4ab0); fill(x - 3, y + 5, 7, 1, 0xd8202a); },
  },
  xmencyclops: {
    // Cyclops's blue suit with yellow straps crossing on the chest and a yellow belt
    front(x, y, o) {
      const gold = 0xf2c41a;
      line(x - 3, y, x + 2, y + 4, gold); line(x + 3, y, x - 2, y + 4, gold); fill(x - 3, y + 5, 7, 1, gold); dot(x, y + 5, 0xd8202a);
    },
    back(x, y, o) { line(x - 3, y, x + 3, y + 4, 0xf2c41a); line(x + 3, y, x - 3, y + 4, 0xf2c41a); fill(x - 3, y + 5, 7, 1, 0xf2c41a); },
  },
  xmengambit: {
    // Gambit's long brown coat hanging open over magenta chest armour, the coat's collar turned up
    front(x, y, o) {
      fill(x - 1, y, 3, 5, 0xc83a8a); fill(x - 1, y, 3, 1, 0xe85aa8); dot(x, y + 2, 0x8a2a5a); fill(x - 1, y + 5, 3, 1, 0x2a2a3a);
      dot(x - 3, y - 1, shaded(o.shirt, 0.8)); dot(x + 3, y - 1, shaded(o.shirt, 0.8)); fill(x - 2, y, 1, 6, shaded(o.shirt, 0.8)); fill(x + 2, y, 1, 6, shaded(o.shirt, 0.8));
    },
    back(x, y, o) { fill(x - 3, y - 1, 7, 1, shaded(o.shirt, 0.8)); fill(x, y + 2, 1, 4, shaded(o.shirt, 0.85)); },
  },
  xmenstormsuit: {
    // Storm's black suit with a silver-white cape falling from the shoulders and a gold clasp at the throat
    front(x, y, o) {
      fill(x - 3, y, 1, 6, 0xe8e8f0); fill(x + 3, y, 1, 6, 0xe8e8f0); dot(x - 2, y, 0xe8e8f0); dot(x + 2, y, 0xe8e8f0);
      dot(x, y, 0xf2c41a); dot(x - 1, y, 0xd8b45a); dot(x + 1, y, 0xd8b45a); line(x - 1, y + 2, x + 1, y + 4, 0xf2c41a, 0.6);
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xe8e8f0); fill(x - 3, y + 5, 7, 1, 0xc8c8d4); },
  },
  f1racesuit: {
    // A red race suit with a black collar, white sponsor patches on the chest and a little yellow shield badge
    front(x, y, o) {
      fill(x - 2, y, 5, 1, 0x1c1a22); fill(x - 3, y + 2, 2, 1, WHITE); fill(x + 2, y + 2, 2, 1, WHITE); dot(x - 2, y + 4, 0xf2c41a); dot(x - 2, y + 3, 0xf2c41a);
      fill(x + 1, y + 4, 3, 1, WHITE, 0.8); fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.8));
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, 0x1c1a22); fill(x - 2, y + 2, 5, 2, WHITE); },
  },
  f1silversuit: {
    // A silver race suit with teal stripes down the sides and a black collar and belt
    front(x, y, o) {
      fill(x - 2, y, 5, 1, 0x1c1a22); fill(x - 3, y + 1, 1, 5, 0x00a19c); fill(x + 3, y + 1, 1, 5, 0x00a19c); dot(x, y + 2, 0x2a2a30); dot(x - 1, y + 3, 0x2a2a30); dot(x + 1, y + 3, 0x2a2a30);
      fill(x - 2, y + 5, 5, 1, 0x2a2a30);
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, 0x1c1a22); fill(x - 3, y + 1, 1, 5, 0x00a19c); fill(x + 3, y + 1, 1, 5, 0x00a19c); },
  },
  f1pitcrew: {
    // Papaya pit crew overalls with black shoulder panels and a reflective silver band across the chest
    front(x, y, o) {
      fill(x - 3, y, 2, 2, 0x2a2a30); fill(x + 2, y, 2, 2, 0x2a2a30); fill(x - 3, y + 3, 7, 1, 0xc8ccd4); dot(x - 1, y + 3, WHITE); dot(x, y + 1, 0x2a2a30);
    },
    back(x, y, o) { fill(x - 3, y, 7, 2, 0x2a2a30); fill(x - 3, y + 3, 7, 1, 0xc8ccd4); },
  },
  f1teampolo: {
    // A navy team polo with a red collar and placket, a yellow sun badge and a red stripe across one shoulder
    front(x, y, o) {
      fill(x - 2, y, 5, 1, 0xd8202a); fill(x, y + 1, 1, 2, 0xd8202a); dot(x + 2, y + 2, 0xf2c41a); dot(x + 2, y + 3, 0xd8202a); line(x - 3, y + 1, x - 2, y + 2, 0xd8202a);
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, 0xd8202a); fill(x - 1, y + 2, 3, 1, 0xf2c41a); },
  },
  ufcrashguard: {
    // A black rashguard with red panels down the sides and a red stripe across the chest
    front(x, y, o) { fill(x - 3, y + 1, 1, 5, 0xd8202a); fill(x + 3, y + 1, 1, 5, 0xd8202a); fill(x - 2, y + 2, 5, 1, 0xd8202a); dot(x, y + 4, 0x3a3a42); },
    back(x, y, o) { fill(x - 3, y + 1, 1, 5, 0xd8202a); fill(x + 3, y + 1, 1, 5, 0xd8202a); fill(x - 2, y + 2, 5, 1, 0xd8202a); },
  },
  ufcreferee: {
    // The referee's blue shirt with a black collar, a badge on the chest and black gloves at the cuffs
    front(x, y, o) { fill(x - 2, y, 5, 1, 0x1c1a22); fill(x, y + 1, 1, 5, shaded(o.shirt, 0.8)); fill(x + 2, y + 2, 1, 1, 0xd8b45a); dot(x - 3, y + 5, 0x1c1a22); dot(x + 3, y + 5, 0x1c1a22); },
    back(x, y, o) { fill(x - 2, y, 5, 1, 0x1c1a22); fill(x - 2, y + 2, 5, 2, 0x1c1a22, 0.5); },
  },
  ufccornerhoodie: {
    // The corner team's black hoodie, its hood down, a red stripe down each arm and a kangaroo pocket
    front(x, y, o) { fill(x - 2, y, 5, 1, 0x3a3a42); fill(x - 3, y, 1, 6, 0xd8202a); fill(x + 3, y, 1, 6, 0xd8202a); fill(x - 1, y + 4, 3, 2, 0x34343c); dot(x - 1, y + 1, 0xd8d4cc); dot(x + 1, y + 1, 0xd8d4cc); },
    back(x, y, o) { fill(x - 2, y - 1, 5, 2, 0x34343c); fill(x - 3, y, 1, 6, 0xd8202a); fill(x + 3, y, 1, 6, 0xd8202a); },
  },
  ufcwalkout: {
    // A gold satin walkout robe with black lapels and a black belt, its sheen catching the light now and then
    front(x, y, o) {
      line(x - 2, y, x, y + 3, 0x1c1a22); line(x + 2, y, x, y + 3, 0x1c1a22); fill(x - 3, y + 4, 7, 1, 0x1c1a22); dot(x + 2, y + 5, 0x1c1a22);
      if ((now + x * 0.3) % 6 < 0.5) dot(x - 2, y + 2, WHITE, 0.7);
    },
    back(x, y, o) { fill(x - 3, y + 4, 7, 1, 0x1c1a22); fill(x - 2, y + 1, 5, 2, 0x1c1a22, 0.4); },
  },
  nyloftpeacoat: {
    // A navy wool peacoat, double-breasted with two rows of dark buttons and a wide collar over a grey scarf
    front(x, y, o) {
      fill(x - 2, y, 5, 1, 0x8a8e98); dot(x - 3, y, shaded(o.shirt, 1.25)); dot(x + 3, y, shaded(o.shirt, 1.25));
      for (const dy of [2, 4]) { dot(x - 1, y + dy, 0x101420); dot(x + 1, y + dy, 0x101420); }
      line(x - 2, y + 1, x, y + 3, shaded(o.shirt, 0.75));
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, 0x8a8e98); fill(x, y + 3, 1, 3, shaded(o.shirt, 0.8)); },
  },
  nyloftflannel: {
    // A red and black check flannel shirt, sleeves rolled, open over a white tee
    front(x, y, o) {
      fill(x - 3, y + 1, 7, 1, 0x1c1a22, 0.6); fill(x - 3, y + 4, 7, 1, 0x1c1a22, 0.6); fill(x - 2, y, 1, 6, 0x1c1a22, 0.5); fill(x + 2, y, 1, 6, 0x1c1a22, 0.5);
      fill(x - 1, y, 3, 6, 0xf4f2ee); dot(x, y + 2, 0xe8e4dc);
    },
    back(x, y, o) { fill(x - 3, y + 1, 7, 1, 0x1c1a22, 0.6); fill(x - 3, y + 4, 7, 1, 0x1c1a22, 0.6); fill(x - 2, y, 1, 6, 0x1c1a22, 0.5); fill(x + 2, y, 1, 6, 0x1c1a22, 0.5); },
  },
  nyloftturtleneck: {
    // A slim black turtleneck with its rolled collar, and a thin gold chain with a small pendant
    front(x, y, o) { fill(x - 2, y - 1, 5, 2, 0x2e2a36); dot(x - 1, y + 1, 0xd8b45a); dot(x, y + 2, 0xd8b45a); dot(x + 1, y + 1, 0xd8b45a); },
    back(x, y, o) { fill(x - 2, y - 1, 5, 2, 0x2e2a36); },
  },
  nyloftleather: {
    // A black leather jacket with a diagonal silver zip and wide lapels open over a grey tee
    front(x, y, o) {
      fill(x - 1, y, 3, 3, 0x8a8e98); line(x - 2, y, x + 1, y + 5, 0xc8ccd4); dot(x - 3, y, 0x3a3a42); dot(x + 3, y, 0x3a3a42); dot(x + 2, y + 1, 0x3a3a42);
      fill(x - 3, y + 5, 7, 1, 0x3a3a42); dot(x + 3, y + 3, 0xc8ccd4);
    },
    back(x, y, o) { fill(x - 3, y + 5, 7, 1, 0x3a3a42); fill(x - 3, y, 7, 1, 0x3a3a42); },
  },
  pariscafemariniere: {
    // A navy and white striped top with a little red neckerchief knotted at the throat
    front(x, y, o) {
      for (const dy of [1, 3, 5]) fill(x - 3, y + dy, 7, 1, 0x2a3a6a);
      fill(x - 1, y, 3, 1, 0xd8302a); dot(x, y + 1, 0xd8302a); dot(x + 1, y + 2, 0xb81a1a);
    },
    back(x, y, o) { for (const dy of [1, 3, 5]) fill(x - 3, y + dy, 7, 1, 0x2a3a6a); fill(x - 1, y, 3, 1, 0xd8302a); },
  },
  pariscafegarcon: {
    // The waiter's white shirt and black waistcoat with a little bow tie, and the long white apron tied at the waist
    front(x, y, o) {
      fill(x - 3, y, 2, 4, 0x1c1a22); fill(x + 2, y, 2, 4, 0x1c1a22); dot(x - 1, y, 0x1c1a22); dot(x + 1, y, 0x1c1a22); dot(x, y, 0x2a2a30);
      fill(x - 3, y + 4, 7, 2, WHITE); fill(x - 3, y + 4, 7, 1, 0xd8d4cc);
    },
    back(x, y, o) { fill(x - 3, y, 7, 4, 0x1c1a22); fill(x - 1, y + 4, 3, 1, WHITE); },
  },
  pariscafetrench: {
    // A belted beige trench coat with wide lapels, a double row of buttons and the belt knotted at the side
    front(x, y, o) {
      const D = shaded(o.shirt, 0.78);
      line(x - 2, y, x, y + 2, D); line(x + 2, y, x, y + 2, D); dot(x - 1, y + 3, 0x5a4a32); dot(x + 1, y + 3, 0x5a4a32);
      fill(x - 3, y + 4, 7, 1, D); dot(x + 3, y + 5, D);
    },
    back(x, y, o) { fill(x - 3, y + 4, 7, 1, shaded(o.shirt, 0.78)); fill(x - 3, y, 7, 2, shaded(o.shirt, 0.9)); },
  },
  pariscafepearls: {
    // A little black dress with a boat neck and a string of pearls, a glint running along them now and then
    front(x, y, o) {
      for (const [dx, dy] of [[-2, 0], [-1, 1], [0, 1], [1, 1], [2, 0]]) dot(x + dx, y + dy, 0xf4f0e6);
      if ((now + x * 0.3) % 7 < 0.4) dot(x, y + 1, WHITE);
      fill(x - 3, y + 3, 7, 1, shaded(o.shirt, 1.3), 0.5);
    },
    back(x, y, o) { dot(x - 2, y, 0xf4f0e6); dot(x + 2, y, 0xf4f0e6); dot(x, y, 0xf4f0e6); },
  },
  muertoscatrinadress: {
    // A Catrina's long plum gown with a black lace yoke, a ruffle at the hem and a big orange marigold pinned at the chest
    front(x, y, o) {
      for (let k = -3; k <= 3; k += 2) dot(x + k, y, 0x1c1a22); fill(x - 2, y + 1, 5, 1, 0x1c1a22, 0.6);
      fill(x + 1, y + 2, 2, 2, 0xe8902a); dot(x + 1, y + 2, 0xf2b03a); dot(x + 2, y + 4, 0x3a7a3a);
      for (let k = -3; k <= 3; k += 2) dot(x + k, y + 5, shaded(o.shirt, 1.3));
    },
    back(x, y, o) { fill(x - 2, y, 5, 2, 0x1c1a22, 0.7); for (let k = -3; k <= 3; k += 2) dot(x + k, y + 5, shaded(o.shirt, 1.3)); },
  },
  muertoscharro: {
    // A short black charro jacket trimmed with rows of silver buttons and embroidery, a red bow tie at the neck
    front(x, y, o) {
      fill(x - 1, y, 3, 1, 0xd8202a); dot(x, y + 1, 0xf4f2ee); dot(x, y + 2, 0xf4f2ee);
      for (let k = 0; k < 4; k++) { dot(x - 2, y + 1 + k, 0xc8ccd4); dot(x + 2, y + 1 + k, 0xc8ccd4); }
      dot(x - 3, y + 2, 0xc8a050); dot(x + 3, y + 2, 0xc8a050); dot(x - 3, y + 4, 0xc8a050); dot(x + 3, y + 4, 0xc8a050);
    },
    back(x, y, o) { for (let k = -2; k <= 2; k += 2) dot(x + k, y + 2, 0xc8ccd4); line(x - 2, y + 4, x + 2, y + 4, 0xc8a050); },
  },
  muertosblouse: {
    // A white blouse embroidered with bright flowers round its square neckline and down its front
    front(x, y, o) {
      fill(x - 2, y, 5, 1, 0xe83a8a); dot(x - 3, y, 0xf2c41a); dot(x + 3, y, 0x1a8a8a);
      for (const [dx, dy, c] of [[-2, 2, 0xe8902a], [2, 2, 0xe83a8a], [0, 3, 0x8a3ab0], [-1, 4, 0x3a9a4a], [1, 4, 0x3a9a4a]]) dot(x + dx, y + dy, c);
    },
    back(x, y, o) { fill(x - 2, y, 5, 1, 0xe83a8a); dot(x, y + 2, 0xe8902a); },
  },
  muertosbones: {
    // A black top printed with a white skeleton's ribs and spine, a little pink heart drawn where the real one would be
    front(x, y, o) {
      fill(x, y, 1, 6, 0xe8e4dc); for (const dy of [1, 2, 3]) { fill(x - 2, y + dy, 2, 1, 0xe8e4dc); fill(x + 1, y + dy, 2, 1, 0xe8e4dc); }
      dot(x - 2, y + 5, 0xe8e4dc); dot(x + 2, y + 5, 0xe8e4dc); dot(x + 2, y + 2, 0xe83a8a);
    },
    back(x, y, o) { fill(x, y, 1, 6, 0xe8e4dc); fill(x - 2, y + 4, 5, 1, 0xe8e4dc); },
  },
  submarinedeepcoverall: {
    // Orange crew coveralls with a zip down the front, a reflective silver band across the chest and a little patch with a wave on it
    front(x, y, o) { fill(x, y, 1, 6, 0xa84a1a); fill(x - 3, y + 2, 7, 1, 0xc8ccd4); fill(x - 3, y + 1, 2, 1, 0x2a5ab0); dot(x - 2, y + 1, WHITE); },
    back(x, y, o) { fill(x - 3, y + 2, 7, 1, 0xc8ccd4); },
  },
  submarinedeepwetsuit: {
    // A black wetsuit with teal panels down the sides and across the shoulders, its zip cord trailing down the back
    front(x, y, o) { fill(x - 3, y, 7, 1, 0x1aa8a8); fill(x - 3, y + 1, 1, 5, 0x1aa8a8); fill(x + 3, y + 1, 1, 5, 0x1aa8a8); },
    back(x, y, o) { fill(x - 3, y, 7, 1, 0x1aa8a8); fill(x, y, 1, 6, 0x5a6670); dot(x + 1, y + 5, 0xf2c41a); },
  },
  submarinedeeplabcoat: {
    // A white lab coat over a navy shirt, a pen in the pocket and a clip-on ID badge
    front(x, y, o) { fill(x - 1, y, 3, 6, 0x2a3a5a); line(x - 2, y, x - 1, y + 2, 0xd8dce0); line(x + 2, y, x + 1, y + 2, 0xd8dce0); fill(x - 3, y + 3, 2, 1, 0xd8dce0); dot(x - 3, y + 2, 0x2a5ab0); fill(x + 2, y + 3, 2, 2, 0xe8702a); dot(x + 2, y + 3, WHITE); },
    back(x, y, o) { fill(x, y + 3, 1, 3, 0xd8dce0); },
  },
  submarinedeepsweater: {
    // A navy ribbed captain's sweater with suede patches on the shoulders and a white collar showing at the neck
    front(x, y, o) { fill(x - 1, y, 3, 1, WHITE); fill(x - 3, y, 2, 2, 0x5a6a7a); fill(x + 2, y, 2, 2, 0x5a6a7a); for (let k = -2; k <= 2; k += 2) fill(x + k, y + 3, 1, 3, shaded(o.shirt, 0.8)); },
    back(x, y, o) { fill(x - 3, y, 2, 2, 0x5a6a7a); fill(x + 2, y, 2, 2, 0x5a6a7a); },
  },
  arcticbaseparka: {
    // A red expedition parka with a thick fur-trimmed hood ruff round the neck, a big zip and a reflective strip
    front(x, y, o) { fill(x - 3, y - 1, 7, 2, 0xd8ccb8); dot(x - 3, y - 1, 0xb8a890); dot(x + 3, y - 1, 0xb8a890); fill(x, y + 1, 1, 5, 0x2a2a30); fill(x - 3, y + 4, 7, 1, 0xc8ccd4); },
    back(x, y, o) { fill(x - 3, y - 1, 7, 2, 0xd8ccb8); fill(x - 3, y + 4, 7, 1, 0xc8ccd4); },
  },
  arcticbasefleece: {
    // A blue fleece jacket with a high zip collar, grey shoulder panels and a little chest pocket
    front(x, y, o) { fill(x - 1, y, 3, 2, shaded(o.shirt, 0.8)); fill(x, y + 2, 1, 4, 0xc8ccd4); fill(x - 3, y, 2, 2, 0x6a727a); fill(x + 2, y, 2, 2, 0x6a727a); dot(x + 2, y + 3, 0xc8ccd4); },
    back(x, y, o) { fill(x - 3, y, 7, 2, 0x6a727a); },
  },
  arcticbasecoldsuit: {
    // An orange cold-weather work suit with black knee-high bib straps, a reflective chest band and a radio clipped at the shoulder
    front(x, y, o) { fill(x - 2, y, 1, 6, 0x1c1a22); fill(x + 2, y, 1, 6, 0x1c1a22); fill(x - 3, y + 3, 7, 1, 0xc8ccd4); fill(x + 2, y - 1, 2, 2, 0xf2c41a); },
    back(x, y, o) { line(x - 2, y, x + 2, y + 3, 0x1c1a22); line(x + 2, y, x - 2, y + 3, 0x1c1a22); fill(x - 3, y + 4, 7, 1, 0xc8ccd4); },
  },
  arcticbaseknit: {
    // A cream Nordic jumper with a round yoke patterned in navy and red stars
    front(x, y, o) { fill(x - 3, y, 7, 2, 0x2a3a5a); for (const dx of [-2, 0, 2]) { dot(x + dx, y, 0xf2ece0); dot(x + dx, y + 1, 0xc8302a); } for (let k = -3; k <= 3; k += 2) dot(x + k, y + 2, 0x2a3a5a); },
    back(x, y, o) { fill(x - 3, y, 7, 2, 0x2a3a5a); for (const dx of [-2, 0, 2]) dot(x + dx, y + 1, 0xc8302a); },
  },
  sakurayukata: {
    // A pale pink cotton yukata printed with white blossoms, crossed left over right, and a deep red obi sash at the waist
    front(x, y, o) {
      line(x - 2, y, x + 1, y + 3, shaded(o.shirt, 0.8)); line(x + 2, y, x, y + 2, shaded(o.shirt, 0.8));
      fill(x - 3, y + 4, 7, 2, 0xb82a3a); fill(x - 3, y + 4, 7, 1, 0xd84a5a); dot(x + 2, y + 5, 0xf2c41a);
      dot(x - 2, y + 2, WHITE); dot(x + 3, y + 1, WHITE);
    },
    back(x, y, o) { fill(x - 3, y + 4, 7, 2, 0xb82a3a); fill(x - 1, y + 3, 3, 1, 0xb82a3a); dot(x - 2, y + 1, WHITE); },
  },
  sakuracardigan: {
    // A mint pastel cardigan buttoned over a pink blouse with a little round collar
    front(x, y, o) { fill(x - 1, y, 3, 6, 0xf6c8d4); fill(x - 1, y, 3, 1, WHITE); for (const dy of [2, 4]) dot(x - 2, y + dy, WHITE); },
    back(x, y, o) { fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.85)); },
  },
  sakurasailor: {
    // A white top with a broad navy sailor collar edged with a white stripe and a pink neckerchief tied at the front
    front(x, y, o) { fill(x - 3, y, 2, 2, 0x2a3a6a); fill(x + 2, y, 2, 2, 0x2a3a6a); dot(x - 3, y + 1, WHITE); dot(x + 3, y + 1, WHITE); fill(x - 1, y + 1, 3, 1, 0xf28fb0); dot(x, y + 2, 0xf28fb0); dot(x, y + 3, 0xd86a8a); },
    back(x, y, o) { fill(x - 3, y, 7, 3, 0x2a3a6a); fill(x - 3, y + 2, 7, 1, WHITE); },
  },
  sakuraflorist: {
    // A florist's sage-green canvas apron over a white shirt, a pair of pruning shears and a pink blossom tucked in its pocket
    front(x, y, o) { fill(x - 2, y + 1, 5, 5, 0x7a9a7a); dot(x - 2, y, 0x7a9a7a); dot(x + 2, y, 0x7a9a7a); fill(x - 1, y + 3, 3, 2, 0x6a8a6a); dot(x + 1, y + 2, 0xf6b8c8); dot(x - 1, y + 2, 0xa8adb6); },
    back(x, y, o) { line(x - 2, y, x + 2, y + 4, 0x7a9a7a); line(x + 2, y, x - 2, y + 4, 0x7a9a7a); },
  },
  pottergryffindor: {
    // A black school robe open over a white shirt, a grey jumper and a scarlet-and-gold striped tie, the house crest on the
    // chest; round wire glasses, a lightning-bolt scar and untidy black hair
    front(x, y) {
      fill(x - 1, y, 3, 6, 0x6a6a72); fill(x - 1, y, 3, 1, 0xf2f0ea);
      fill(x, y + 1, 1, 4, 0x8a1e1e); dot(x, y + 2, 0xd3a625); dot(x, y + 4, 0xd3a625);
      dot(x - 3, y + 2, 0x8a1e1e); dot(x - 3, y + 3, 0xd3a625);
    },
    back(x, y) { fill(x - 3, y, 7, 1, 0x2a2a30); fill(x, y + 1, 1, 5, 0x141418); },
    head(x, top, look, away) {
      const hair = 0x1a1612;
      fill(x - 3, top - 2, 7, 2, hair); dot(x - 4, top - 1, hair); dot(x + 4, top - 1, hair); dot(x - 2, top - 3, hair); dot(x + 1, top - 3, hair); dot(x + 3, top - 2, hair);
      if (away) { fill(x - 3, top, 7, 2, hair); return; }
      const G = 0x2a2a30;
      for (const ex of [-2, 2]) { dot(x + ex - 1, top + 3, G); dot(x + ex + 1, top + 3, G); dot(x + ex, top + 2, G); dot(x + ex, top + 4, G); }
      dot(x, top + 3, G);
      dot(x + 1, top, 0xb84a3a); dot(x + 2, top + 1, 0xb84a3a);
    },
  },
  potterslytherin: {
    // A black school robe with green linings over a white shirt and a green-and-silver striped tie, a silver serpent pin;
    // slicked-back platinum hair
    front(x, y) {
      fill(x - 1, y, 3, 6, 0xf2f0ea); fill(x, y + 1, 1, 5, 0x1a5a32); dot(x, y + 2, 0xc8ccd4); dot(x, y + 4, 0xc8ccd4);
      fill(x - 3, y, 1, 6, 0x1a5a32); fill(x + 3, y, 1, 6, 0x1a5a32); dot(x - 2, y + 2, 0xc8ccd4);
    },
    back(x, y) { fill(x - 3, y, 7, 1, 0x1a5a32); fill(x, y + 1, 1, 5, 0x141418); },
    head(x, top, look, away) {
      const hair = 0xf0e6c0, hairDark = 0xd0c498;
      fill(x - 3, top - 2, 7, 2, hair); fill(x - 4, top - 1, 1, 3, hair); fill(x + 4, top - 1, 1, 3, hair); fill(x - 2, top - 2, 5, 1, 0xfff8dc);
      if (away) { fill(x - 3, top, 7, 3, hair); fill(x - 2, top + 1, 5, 1, hairDark); return; }
      dot(x + 3, top, hairDark);
    },
  },
  potterquidditch: {
    // Scarlet Quidditch robes with gold trim and a gold number on the back, brown leather arm guards; a pair of flying goggles
    // pushed up on the head
    front(x, y, o) {
      fill(x - 3, y, 7, 1, 0xd3a625); fill(x - 1, y + 1, 3, 1, shaded(o.shirt, 0.8)); dot(x, y + 2, shaded(o.shirt, 0.8));
      fill(x - 3, y + 4, 1, 2, 0x6a4a2e); fill(x + 3, y + 4, 1, 2, 0x6a4a2e); fill(x - 2, y + 5, 5, 1, 0xd3a625, 0.8);
    },
    back(x, y) { fill(x - 3, y, 7, 1, 0xd3a625); fill(x - 1, y + 1, 1, 4, 0xd3a625); fill(x + 1, y + 1, 1, 4, 0xd3a625); dot(x, y + 1, 0xd3a625); dot(x, y + 4, 0xd3a625); },
    head(x, top, look, away) {
      fill(x - 4, top, 9, 1, 0x6a4a2e);
      if (!away) { fill(x - 3, top - 1, 3, 2, 0x6a4a2e); fill(x + 1, top - 1, 3, 2, 0x6a4a2e); dot(x - 2, top - 1, 0xc8e0f0); dot(x + 2, top - 1, 0xc8e0f0); }
    },
  },
  potterheadmaster: {
    // Long midnight-purple robes sewn with little silver stars and moons that twinkle now and then; half-moon spectacles,
    // a long silver beard down the chest and a tall pointed hat
    front(x, y, o) {
      fill(x - 1, y, 3, 5, 0xe8e8ec); fill(x, y + 5, 1, 1, 0xe8e8ec); dot(x - 1, y + 4, 0xc8c8d0);
      const tw = Math.floor((now + x * 0.3) % 6) === 0;
      for (const [dx, dy] of [[-3, 1], [3, 3], [-2, 5], [2, 0]]) dot(x + dx, y + dy, 0xd8dce8, tw ? 1 : 0.6);
      dot(x + 3, y + 5, 0xf2d35b, 0.8);
    },
    back(x, y) { for (const [dx, dy] of [[-2, 1], [2, 2], [0, 4], [-3, 5], [3, 0]]) dot(x + dx, y + dy, 0xd8dce8, 0.7); },
    head(x, top, look, away, o) {
      const hat = o.shirt, star = 0xd8dce8, beard = 0xe8e8ec;
      fill(x - 5, top, 11, 1, hat); fill(x - 3, top - 1, 7, 1, hat);
      for (const [dy, l, w] of [[2, -3, 6], [3, -2, 5], [4, -2, 4], [5, -1, 3], [6, -1, 2], [7, 0, 2], [8, 1, 1]]) fill(x + l, top - dy, w, 1, hat);
      dot(x - 1, top - 3, star); dot(x + 1, top - 5, star, 0.8); dot(x - 5, top, shaded(hat, 1.3));
      fill(x - 4, top + 1, 1, 6, beard); fill(x + 4, top + 1, 1, 6, beard);
      if (away) { fill(x - 3, top + 1, 7, 6, beard, 0.9); return; }
      fill(x - 2, top + 5, 5, 3, beard); dot(x - 3, top + 6, beard); dot(x + 3, top + 6, beard);
      for (const ex of [-2, 2]) { fill(x + ex - 1, top + 3, 3, 1, 0xc8a050); }
    },
  },
  lotrgandalf: {
    // A long grey travelling robe with a grey cloak and a belt with a small pouch; a long grey beard and a tall, wide-brimmed
    // pointed hat with its tip bent
    front(x, y, o) {
      fill(x - 1, y, 3, 5, 0xc8c8c8); dot(x, y + 5, 0xc8c8c8); dot(x - 1, y + 4, 0xa8a8a8);
      fill(x - 3, y, 1, 6, shaded(o.shirt, 0.75)); fill(x + 3, y, 1, 6, shaded(o.shirt, 0.75)); fill(x - 3, y + 4, 7, 1, 0x5a4a3a); dot(x + 2, y + 5, 0x6a5a40);
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, shaded(o.shirt, 0.8)); fill(x, y, 1, 6, shaded(o.shirt, 0.7)); },
    head(x, top, look, away, o) {
      const hat = 0x6a6a70, beard = 0xc8c8c8;
      fill(x - 6, top, 13, 1, hat); fill(x - 3, top - 1, 7, 1, hat);
      for (const [dy, l, w] of [[2, -3, 6], [3, -2, 5], [4, -2, 4], [5, -1, 3], [6, 0, 2], [7, 1, 2], [8, 2, 2]]) fill(x + l, top - dy, w, 1, hat);
      dot(x + 4, top - 8, hat); dot(x - 6, top, shaded(hat, 1.3));
      fill(x - 4, top + 1, 1, 6, beard); fill(x + 4, top + 1, 1, 6, beard);
      if (away) { fill(x - 3, top + 1, 7, 6, beard, 0.9); return; }
      fill(x - 2, top + 5, 5, 3, beard); dot(x - 3, top + 5, beard); dot(x + 3, top + 5, beard); dot(x, top + 8, beard);
      fill(x - 2, top + 2, 2, 1, 0xa8a8a8); fill(x + 1, top + 2, 2, 1, 0xa8a8a8);
    },
  },
  lotrhobbit: {
    // A cream shirt, a mustard waistcoat with brass buttons and braces, a green cloak with a leaf-shaped brooch at the
    // throat; curly brown hair and bare feet
    front(x, y) {
      fill(x - 3, y, 2, 5, 0xc8902a); fill(x + 2, y, 2, 5, 0xc8902a); dot(x - 2, y + 1, 0xd8b45a); dot(x - 2, y + 3, 0xd8b45a);
      fill(x - 3, y, 7, 1, 0x3a6a3a); dot(x, y, 0x8aba6a); dot(x, y + 1, 0x6a9a4a);
    },
    back(x, y) { fill(x - 3, y, 7, 6, 0x3a6a3a); fill(x - 3, y, 1, 6, 0x4a7a4a); },
    head(x, top, look, away) {
      const hair = 0x6a4228, curl = 0x8a5a32;
      fill(x - 3, top - 2, 7, 2, hair); fill(x - 4, top - 1, 1, 4, hair); fill(x + 4, top - 1, 1, 4, hair);
      for (const [dx, dy] of [[-3, -2], [-1, -3], [1, -2], [3, -3], [-4, 1], [4, 1]]) dot(x + dx, top + dy, curl);
      if (away) fill(x - 3, top, 7, 3, hair);
    },
  },
  lotrelf: {
    // A grey-green elven tunic with a silver leaf clasp and a quiver strap across the chest; long straight pale-blond hair
    // falling to the shoulders, a braid at each side, and pointed ears
    front(x, y, o) {
      line(x - 3, y, x + 3, y + 5, 0x6a4a2a); dot(x - 2, y, 0xd8dce8); dot(x - 2, y + 1, 0xb8c0c8);
      fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.75));
    },
    back(x, y) { line(x + 3, y, x - 3, y + 5, 0x6a4a2a); fill(x + 1, y - 2, 2, 5, 0x7a5232); dot(x + 1, y - 3, 0xe8e8e8); dot(x + 2, y - 3, 0xe8e8e8); },
    head(x, top, look, away) {
      const hair = 0xf0e0a8, hairDark = 0xd0bc80, skin = 0xf0d4b4;
      fill(x - 3, top - 2, 7, 2, hair); fill(x - 4, top - 1, 1, 9, hair); fill(x + 4, top - 1, 1, 9, hair);
      dot(x - 4, top + 8, hairDark); dot(x + 4, top + 8, hairDark);
      if (away) { fill(x - 3, top, 7, 8, hair); fill(x, top, 1, 7, hairDark); return; }
      dot(x - 5, top + 2, skin); dot(x + 5, top + 2, skin); dot(x - 5, top + 1, skin);
    },
  },
  lotrranger: {
    // A dark weathered ranger's coat over a leather jerkin, a sword belt across it and the silver star of the Dúnedain at the
    // throat; long dark hair and a short beard
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0x5a4028); fill(x - 1, y, 1, 6, 0x6a4a30); dot(x, y, 0xd8dce8);
      line(x - 3, y + 1, x + 3, y + 5, 0x2a2018); dot(x + 3, y + 5, 0xb8bcc4);
      fill(x - 3, y, 1, 6, shaded(o.shirt, 1.2));
    },
    back(x, y, o) { fill(x - 3, y, 7, 6, shaded(o.shirt, 0.9)); line(x + 3, y + 1, x - 3, y + 5, 0x2a2018); },
    head(x, top, look, away) {
      const hair = 0x2a2018;
      fill(x - 3, top - 2, 7, 2, hair); fill(x - 4, top - 1, 1, 8, hair); fill(x + 4, top - 1, 1, 8, hair);
      if (away) { fill(x - 3, top, 7, 7, hair); return; }
      fill(x - 2, top + 5, 5, 2, hair, 0.75); dot(x - 3, top + 4, hair);
    },
  },
  breakingbadhazmat: {
    // A baggy yellow hazmat suit with a zip down the front and black gloves; a black respirator mask with two pink filters
    // over the face and the suit's hood up
    front(x, y, o) { fill(x, y, 1, 6, shaded(o.shirt, 0.75)); dot(x, y, 0xa8adb6); fill(x - 3, y + 5, 1, 1, 0x2a2a30); fill(x + 3, y + 5, 1, 1, 0x2a2a30); dot(x - 2, y + 2, shaded(o.shirt, 1.15)); },
    back(x, y, o) { fill(x - 3, y, 7, 1, shaded(o.shirt, 0.85)); fill(x, y + 1, 1, 5, shaded(o.shirt, 0.85)); },
    head(x, top, look, away, o) {
      const hood = o.shirt;
      fill(x - 4, top - 2, 9, 3, hood); fill(x - 4, top + 1, 2, 7, hood); fill(x + 3, top + 1, 2, 7, hood); fill(x - 3, top - 3, 7, 1, hood);
      if (away) { fill(x - 2, top + 1, 5, 7, hood); return; }
      fill(x - 2, top + 4, 5, 3, 0x2a2a30); dot(x - 3, top + 5, 0xe890a8); dot(x + 3, top + 5, 0xe890a8); dot(x, top + 6, 0x4a4a52);
    },
  },
  breakingbadheisenberg: {
    // A black zip jacket over a pale green shirt; the black porkpie hat, rimless dark glasses and a goatee
    front(x, y) { fill(x - 1, y, 3, 6, 0xa8c0a0); fill(x, y, 1, 6, 0x8aa080); dot(x - 2, y, 0x4a4a52); dot(x + 2, y, 0x4a4a52); },
    back() {},
    head(x, top, look, away) {
      const hat = 0x1e1e22;
      fill(x - 5, top, 11, 1, hat); fill(x - 3, top - 3, 7, 3, hat); fill(x - 3, top - 1, 7, 1, 0x3a3a42);
      if (away) return;
      fill(x - 3, top + 3, 3, 1, 0x101014); fill(x + 1, top + 3, 3, 1, 0x101014); dot(x, top + 3, 0x4a4a52);
      fill(x - 1, top + 6, 3, 2, 0x5a4a3a); dot(x, top + 8, 0x5a4a3a);
    },
  },
  breakingbadjesse: {
    // A baggy mustard-yellow hoodie with a big front pocket and drawstrings, sagging jeans; a dark grey beanie pulled low
    front(x, y, o) { dot(x - 1, y, 0xf2f0ea); dot(x + 1, y, 0xf2f0ea); dot(x - 1, y + 1, 0xf2f0ea); dot(x + 1, y + 1, 0xf2f0ea); fill(x - 2, y + 3, 5, 3, shaded(o.shirt, 0.82)); fill(x - 2, y + 3, 5, 1, shaded(o.shirt, 0.7)); },
    back(x, y, o) { fill(x - 2, y, 5, 2, shaded(o.shirt, 0.82)); },
    head(x, top, look, away) {
      const b = 0x4a4a52;
      fill(x - 4, top - 2, 9, 4, b); fill(x - 3, top - 3, 7, 1, b); for (let k = -3; k < 5; k += 2) dot(x + k, top + 1, 0x3a3a42);
      if (away) fill(x - 3, top + 2, 7, 2, b);
    },
  },
  breakingbadsaul: {
    // A loud lilac suit over a canary-yellow shirt and a gaudy patterned tie; a combed-over brown hairdo
    front(x, y, o) {
      fill(x - 1, y, 3, 4, 0xf2d040); fill(x, y + 1, 1, 4, 0xc8281e); dot(x, y + 2, 0x4a8ad8); dot(x, y + 4, 0x4a8ad8);
      dot(x - 2, y, shaded(o.shirt, 1.25)); dot(x + 2, y, shaded(o.shirt, 1.25)); dot(x - 2, y + 3, 0xf6f2ea);
    },
    back(x, y, o) { fill(x, y + 2, 1, 4, shaded(o.shirt, 0.8)); },
    head(x, top, look, away) {
      const hair = 0x6a4228;
      fill(x - 3, top - 1, 7, 2, hair); fill(x - 3, top - 2, 6, 1, hair); dot(x + 3, top - 2, shaded(hair, 1.3)); dot(x - 4, top, hair); dot(x + 4, top, hair);
      if (away) fill(x - 3, top + 1, 7, 2, hair);
    },
  },
  strangereleven: {
    // A pink dress with a white Peter Pan collar under a navy jacket; close-cropped dark hair, and now and then a single drop
    // of a nosebleed
    front(x, y) { fill(x - 1, y, 3, 1, 0xf6f2ea); fill(x - 3, y, 2, 6, 0x2a3a6a); fill(x + 2, y, 2, 6, 0x2a3a6a); dot(x - 2, y, 0x3a4a7a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x2a3a6a); },
    head(x, top, look, away) {
      const hair = 0x3a2a20;
      fill(x - 3, top - 1, 7, 2, hair); dot(x - 4, top, hair); dot(x + 4, top, hair);
      if (away) { fill(x - 3, top + 1, 7, 2, hair); return; }
      if ((now + x * 0.3) % 16 < 2) dot(x - 1, top + 5, 0xc8181e);
    },
  },
  strangerhellfiretee: {
    // A black Hellfire Club raglan tee with red sleeves and the red horned skull on the chest, a denim vest over it; long
    // wild dark curly hair
    front(x, y) {
      fill(x - 3, y, 1, 3, 0xc8281e); fill(x + 3, y, 1, 3, 0xc8281e);
      fill(x - 1, y + 1, 3, 3, 0xc8281e); dot(x - 2, y, 0xc8281e); dot(x + 2, y, 0xc8281e); dot(x - 1, y + 2, 0x1e1e22); dot(x + 1, y + 2, 0x1e1e22);
      fill(x - 3, y + 3, 1, 3, 0x4a6a9a); fill(x + 3, y + 3, 1, 3, 0x4a6a9a);
    },
    back(x, y) { fill(x - 3, y, 7, 6, 0x4a6a9a); fill(x - 1, y + 2, 3, 2, 0xc8281e); },
    head(x, top, look, away) {
      const hair = 0x2a1e18, curl = 0x4a3428;
      fill(x - 4, top - 2, 9, 3, hair); fill(x - 5, top, 2, 8, hair); fill(x + 4, top, 2, 8, hair);
      for (const [dx, dy] of [[-4, -2], [-1, -3], [2, -3], [4, -1], [-5, 4], [5, 5], [-5, 7], [5, 2]]) dot(x + dx, top + dy, curl);
      if (away) fill(x - 3, top, 7, 7, hair);
    },
  },
  strangerscoopsuniform: {
    // The Scoops Ahoy sailor uniform: a white top with a navy sailor collar, red-and-white striped neckerchief and red trim; a
    // little white sailor cap
    front(x, y) {
      fill(x - 3, y, 7, 2, 0x1e2a5a); fill(x - 1, y, 3, 1, 0xf6f2ea); fill(x - 1, y + 2, 3, 1, 0xd8302a); dot(x, y + 3, 0xd8302a); dot(x, y + 4, 0xf6f2ea, 0.5);
      fill(x - 3, y + 5, 7, 1, 0xd8302a, 0.7);
    },
    back(x, y) { fill(x - 3, y, 7, 3, 0x1e2a5a); fill(x - 3, y + 2, 7, 1, 0xf6f2ea); },
    head(x, top, look, away) {
      const cap = 0xf6f2ea;
      fill(x - 3, top - 2, 7, 2, cap); fill(x - 4, top - 1, 9, 1, cap); fill(x - 3, top - 2, 7, 1, 0x1e2a5a); if (!away) fill(x - 2, top - 1, 5, 1, 0xd8d8d8);
    },
  },
  strangerhopper: {
    // The Hawkins police chief's tan uniform shirt with a brown badge patch on the shoulder, a gold star badge and a dark tie;
    // a broad-brimmed tan campaign hat
    front(x, y, o) { fill(x, y, 1, 5, 0x3a2a20); dot(x - 2, y + 1, 0xe8c838); dot(x - 3, y + 2, 0xc8a050); fill(x + 2, y + 1, 2, 2, shaded(o.shirt, 0.8)); dot(x + 3, y, 0x6a4a2a); },
    back(x, y) { dot(x - 3, y, 0x6a4a2a); dot(x + 3, y, 0x6a4a2a); },
    head(x, top, look, away) {
      const hat = 0xa8905a, band = 0x5a4a32;
      fill(x - 6, top, 13, 1, hat); fill(x - 3, top - 3, 7, 3, hat); fill(x - 3, top - 1, 7, 1, band); dot(x, top - 3, shaded(hat, 0.8)); dot(x - 6, top, shaded(hat, 1.2));
      if (!away) fill(x - 2, top + 6, 5, 2, 0x5a4030, 0.7);
    },
  },
  gotnightswatch: {
    // The Night's Watch blacks: a black leather jerkin and a heavy black cloak with a thick fur collar over the shoulders;
    // dark curly hair
    front(x, y) { fill(x - 3, y, 7, 2, 0x3a3430); dot(x - 2, y, 0x5a524a); dot(x + 2, y, 0x5a524a); fill(x, y + 2, 1, 4, 0x2e2e36); dot(x, y + 3, 0x6a6e72); },
    back(x, y) { fill(x - 3, y, 7, 2, 0x3a3430); fill(x - 3, y + 2, 7, 4, 0x141418); },
    head(x, top, look, away) {
      const hair = 0x1e1612;
      fill(x - 3, top - 2, 7, 2, hair); fill(x - 4, top - 1, 1, 6, hair); fill(x + 4, top - 1, 1, 6, hair); dot(x - 2, top - 3, hair); dot(x + 2, top - 3, hair);
      if (away) { fill(x - 3, top, 7, 6, hair); return; }
      fill(x - 2, top + 6, 5, 1, hair, 0.6);
    },
  },
  gotlannisterred: {
    // A crimson doublet with gold lion buttons and a gold-trimmed collar, a gold pauldron on one shoulder; golden hair
    front(x, y, o) { fill(x - 1, y, 3, 1, 0xe0b84a); dot(x, y + 2, 0xe0b84a); dot(x, y + 4, 0xe0b84a); fill(x + 2, y, 2, 2, 0xe0b84a); dot(x + 3, y, 0xfff0a0); fill(x - 3, y + 5, 7, 1, shaded(o.shirt, 0.75)); },
    back(x, y) { fill(x - 3, y, 2, 2, 0xe0b84a); fill(x - 3, y, 7, 1, 0xe0b84a, 0.6); },
    head(x, top, look, away) {
      const hair = 0xe8c860;
      fill(x - 3, top - 2, 7, 2, hair); fill(x - 4, top - 1, 1, 5, hair); fill(x + 4, top - 1, 1, 5, hair); fill(x - 2, top - 2, 3, 1, 0xfff0a0);
      if (away) fill(x - 3, top, 7, 5, hair);
    },
  },
  gotdaenerys: {
    // A pale blue-white gown with a silver three-headed dragon clasp at the shoulder and a chain across the chest; long
    // silver-white hair in braids
    front(x, y) { line(x - 3, y, x + 3, y + 2, 0x8a8e96); dot(x - 3, y, 0xc8ccd4); dot(x - 2, y + 1, 0xc8ccd4); fill(x - 2, y + 3, 5, 3, 0xc8d8e8, 0.7); },
    back(x, y) { fill(x - 1, y, 3, 6, 0xf2f0ea); dot(x, y + 1, 0xd8d4cc); dot(x, y + 3, 0xd8d4cc); },
    head(x, top, look, away) {
      const hair = 0xf2f0ea, braid = 0xd8d4cc;
      fill(x - 3, top - 2, 7, 2, hair); fill(x - 4, top - 1, 1, 9, hair); fill(x + 4, top - 1, 1, 9, hair);
      for (let k = 0; k < 9; k += 2) { dot(x - 4, top + k, braid); dot(x + 4, top + k, braid); }
      fill(x - 3, top - 1, 7, 1, braid);
      if (away) { fill(x - 3, top, 7, 8, hair); fill(x, top, 1, 8, braid); }
    },
  },
  gotwhitewalker: {
    // Frost-rimed armour of pale blue-grey plates; icy pale skin, a crown of small horns and eyes that burn bright blue,
    // glowing through the night
    front(x, y, o) {
      const L = shaded(o.shirt, 1.25), D = shaded(o.shirt, 0.75);
      for (let k = 0; k < 6; k += 2) { fill(x - 3, y + k, 7, 1, L); fill(x - 3, y + k + 1, 7, 1, D); }
      dot(x - 2, y + 1, 0xe8f4ff); dot(x + 2, y + 3, 0xe8f4ff);
    },
    back(x, y, o) { for (let k = 0; k < 6; k += 2) fill(x - 3, y + k, 7, 1, shaded(o.shirt, 1.2)); },
    head(x, top, look, away) {
      const skin = 0xc8d8e4, horn = 0x8a9aa8;
      fill(x - 3, top - 1, 7, 8, skin); for (const dx of [-3, -1, 1, 3]) dot(x + dx, top - 2, horn); dot(x - 2, top - 3, horn); dot(x + 2, top - 3, horn); dot(x, top - 3, horn);
      if (away) return;
      dot(x - 2, top + 3, 0x4ac8ff); dot(x + 2, top + 3, 0x4ac8ff); fill(x - 1, top + 5, 3, 1, 0x8a9aa8);
      if (isNight) glowLate(() => { dot(x - 2, top + 3, 0x8ae8ff); dot(x + 2, top + 3, 0x8ae8ff); glowAt(x, top + 3, 4, 2, 0x4ac8ff, 0.2); });
    },
  },
  simpsonshomer: {
    // A short-sleeved white shirt with a collar and blue trousers; a bald head with two hairs on top and one over each ear, and
    // a five-o'clock shadow of stubble
    front(x, y) { dot(x - 1, y, 0xd8d8d4); dot(x + 1, y, 0xd8d8d4); dot(x, y + 2, 0xc8c8c4); dot(x, y + 4, 0xc8c8c4); },
    back() {},
    head(x, top, look, away) {
      const hair = 0x2a2a30;
      line(x - 1, top - 3, x - 1, top - 1, hair); line(x + 1, top - 3, x + 1, top - 1, hair); dot(x - 4, top + 2, hair); dot(x + 4, top + 2, hair);
      if (!away) fill(x - 2, top + 5, 5, 2, 0x8a7868, 0.55);
    },
  },
  simpsonsmarge: {
    // A strapless green dress with a necklace of red beads; a towering blue beehive of hair
    front(x, y) { for (const dx of [-2, -1, 0, 1, 2]) dot(x + dx, y + (Math.abs(dx) === 2 ? 0 : 1), 0xd8302a); },
    back(x, y) { dot(x - 2, y, 0xd8302a); dot(x + 2, y, 0xd8302a); },
    head(x, top, look, away) {
      const blue = 0x3a6ad8, light = 0x5a8af0, dark = 0x2a4aa8;
      fill(x - 3, top - 2, 7, 3, blue); fill(x - 4, top - 1, 1, 5, blue); fill(x + 4, top - 1, 1, 5, blue);
      fill(x - 3, top - 14, 7, 12, blue); fill(x - 2, top - 15, 5, 1, blue); fill(x - 3, top - 14, 1, 12, light); fill(x + 3, top - 14, 1, 12, dark);
      for (let k = 0; k < 12; k += 3) dot(x - 1 + (k % 2), top - 13 + k, dark);
      if (away) fill(x - 3, top, 7, 4, blue);
    },
  },
  simpsonsbart: {
    // A plain orange T-shirt and blue shorts; a spiky crown of hair in a zigzag all round the top of the head
    front() {},
    back() {},
    head(x, top, look, away) {
      const hair = 0xf2d040, dark = 0xc8a830;
      fill(x - 3, top - 1, 7, 2, hair);
      for (let k = -3; k <= 3; k += 2) { dot(x + k, top - 2, hair); dot(x + k, top - 3, dark); }
      if (away) fill(x - 3, top + 1, 7, 2, hair, 0.6);
    },
  },
  simpsonslisa: {
    // A red dress with a zigzag hem and a string of white pearls; a starburst of spiky points of hair
    front(x, y) { for (const dx of [-2, -1, 0, 1, 2]) dot(x + dx, y + (Math.abs(dx) === 2 ? 0 : 1), WHITE); for (let k = -3; k <= 3; k += 2) dot(x + k, y + 5, shaded(0xd8302a, 0.75)); },
    back(x, y) { for (let k = -3; k <= 3; k += 2) dot(x + k, y + 5, shaded(0xd8302a, 0.75)); },
    head(x, top, look, away) {
      const hair = 0xf2d040, dark = 0xc8a830;
      fill(x - 3, top - 1, 7, 2, hair);
      for (const [dx, dy] of [[-4, -2], [-2, -3], [0, -4], [2, -3], [4, -2], [-5, 0], [5, 0]]) { dot(x + dx, top + dy, hair); dot(x + dx, top + dy - 1, dark); }
      if (away) fill(x - 3, top + 1, 7, 2, hair, 0.6);
    },
  },
  bondtux: {
    // A black dinner suit with satin lapels over a white dress shirt and a black bow tie, a white pocket square; short dark
    // hair swept back
    front(x, y) {
      fill(x - 1, y, 3, 6, 0xf6f4ee); fill(x - 1, y, 3, 1, 0x0c0c10); dot(x, y, 0x2a2a30);
      dot(x - 2, y + 1, 0x3a3a46); dot(x + 2, y + 1, 0x3a3a46); dot(x, y + 3, 0xd8d4cc); dot(x, y + 5, 0xd8d4cc); dot(x - 3, y + 2, 0xf6f4ee);
    },
    back() {},
    head(x, top, look, away) { const hair = 0x2a2018; fill(x - 3, top - 2, 7, 2, hair); dot(x - 4, top, hair); dot(x + 4, top, hair); dot(x - 2, top - 2, 0x4a3a28); if (away) fill(x - 3, top, 7, 2, hair); },
  },
  bondwhitejacket: {
    // An ivory dinner jacket with black trousers, a black bow tie and a red carnation in the buttonhole
    front(x, y, o) {
      fill(x - 1, y, 3, 6, 0xf6f6f2); fill(x - 1, y, 3, 1, 0x0c0c10); dot(x, y, 0x2a2a30);
      dot(x - 2, y + 1, shaded(o.shirt, 0.85)); dot(x + 2, y + 1, shaded(o.shirt, 0.85)); dot(x - 2, y + 2, 0xc8281e); dot(x - 3, y + 2, 0x3a7a3a);
    },
    back(x, y, o) { fill(x, y + 2, 1, 4, shaded(o.shirt, 0.88)); },
  },
  bondmoneypenny: {
    // A smart navy skirt suit over a cream silk blouse with a pearl necklace; dark hair pinned up in a neat chignon
    front(x, y) { fill(x - 1, y, 3, 4, 0xf2e8d0); for (const dx of [-1, 0, 1]) dot(x + dx, y + (dx === 0 ? 1 : 0), WHITE); dot(x + 1, y + 4, 0x2a3450); },
    back() {},
    head(x, top, look, away) {
      const hair = 0x3a2418;
      fill(x - 3, top - 2, 7, 2, hair); fill(x - 4, top - 1, 1, 4, hair); fill(x + 4, top - 1, 1, 4, hair); disc(x, top - 3, 1.6, hair);
      if (away) { fill(x - 3, top, 7, 4, hair); disc(x, top + 2, 1.6, shaded(hair, 0.8)); }
    },
  },
  bondoddjob: {
    // A black chauffeur's tunic buttoned high with a white collar and a black sash; a black bowler hat with a steel brim,
    // which every 30 s spins off like a discus and comes back
    front(x, y) { fill(x - 1, y, 3, 1, 0xf2f2ee); for (let k = 1; k < 6; k += 2) dot(x + 1, y + k, 0x4a4a52); fill(x - 3, y + 4, 7, 1, 0x2a2a30); },
    back() {},
    head(x, top, look, away) {
      const hat = 0x16161c, brim = 0xa8adb6, t = (now + x * 0.3) % 30;
      if (t < 1.5) { const r = Math.sin(t / 1.5 * Math.PI) * 10; fill(x - 3 + Math.round(r), top - 3, 7, 1, brim); fill(x - 1 + Math.round(r), top - 4, 3, 1, hat); return; }
      fill(x - 4, top, 9, 1, brim); fill(x - 3, top - 3, 7, 3, hat); fill(x - 2, top - 4, 5, 1, hat); dot(x - 2, top - 3, 0x3a3a46);
    },
  },
  dunestillsuit: {
    // A dark grey-brown Fremen stillsuit, ribbed with catchpockets and tubes across the chest; a tube runs up to a nose plug,
    // and a desert hood frames the face
    front(x, y, o) {
      const D = shaded(o.shirt, 0.75), L = shaded(o.shirt, 1.25);
      for (let k = 0; k < 6; k += 2) fill(x - 3, y + k, 7, 1, D); line(x - 2, y, x + 2, y + 3, L); dot(x + 2, y + 4, 0x8a8a82); dot(x - 2, y + 5, 0x8a8a82);
    },
    back(x, y, o) { for (let k = 0; k < 6; k += 2) fill(x - 3, y + k, 7, 1, shaded(o.shirt, 0.75)); fill(x - 1, y + 1, 3, 3, shaded(o.shirt, 0.65)); },
    head(x, top, look, away, o) {
      const hood = 0x8a6a48;
      fill(x - 4, top - 2, 9, 3, hood); fill(x - 4, top + 1, 1, 7, hood); fill(x + 4, top + 1, 1, 7, hood); fill(x - 3, top - 3, 7, 1, hood);
      if (away) { fill(x - 3, top + 1, 7, 6, hood); return; }
      dot(x, top + 4, 0x3a3a34); line(x + 1, top + 5, x + 3, top + 8, 0x3a3a34);
    },
  },
  dunefremen: {
    // Sand-coloured desert robes wrapped and belted with a rust sash, a crysknife at the hip; a loose hood and the eyes blue
    // within blue from the spice, which seem to glow at night
    front(x, y, o) { line(x - 3, y, x + 2, y + 5, shaded(o.shirt, 0.78)); fill(x - 3, y + 3, 7, 1, 0x8a3a24); dot(x + 2, y + 4, 0xf2ece0); dot(x + 2, y + 5, 0xf2ece0); },
    back(x, y, o) { fill(x - 3, y + 3, 7, 1, 0x8a3a24); fill(x, y, 1, 6, shaded(o.shirt, 0.82)); },
    head(x, top, look, away, o) {
      const hood = shaded(o.shirt, 0.9);
      fill(x - 4, top - 2, 9, 3, hood); fill(x - 4, top + 1, 2, 6, hood); fill(x + 3, top + 1, 2, 6, hood);
      if (away) { fill(x - 2, top + 1, 5, 6, hood); return; }
      fill(x - 2, top + 3, 2, 1, 0x2a6ae8); fill(x + 1, top + 3, 2, 1, 0x2a6ae8); dot(x - 2, top + 3, 0x6ab0ff); dot(x + 2, top + 3, 0x6ab0ff);
      if (isNight) glowLate(() => { dot(x - 2, top + 3, 0x8ac8ff); dot(x + 2, top + 3, 0x8ac8ff); });
    },
  },
  dunebenegesserit: {
    // The black robes of a Bene Gesserit sister, high-collared with a fine silver trim, and a black veil drawn over the head
    // and lower face, only the eyes showing
    front(x, y) { fill(x - 1, y, 3, 1, 0x8a8e96); fill(x, y + 1, 1, 5, 0x8a8e96, 0.6); },
    back() {},
    head(x, top, look, away) {
      const veil = 0x16161c;
      fill(x - 4, top - 2, 9, 3, veil); fill(x - 4, top + 1, 2, 8, veil); fill(x + 3, top + 1, 2, 8, veil); fill(x - 3, top - 3, 7, 1, veil);
      if (away) { fill(x - 2, top + 1, 5, 8, veil); return; }
      fill(x - 2, top + 4, 5, 4, veil); fill(x - 2, top + 1, 5, 1, veil, 0.8);
    },
  },
  duneharkonnenblack: {
    // Harkonnen black: a tight black uniform with a glossy high collar and a pale blue griffin badge; a hairless chalk-white
    // head and no eyebrows
    front(x, y) { fill(x - 2, y, 5, 1, 0x2e2e36); dot(x - 2, y + 2, 0x6a8ac8); dot(x - 1, y + 2, 0x9ab8e8); fill(x, y + 1, 1, 5, 0x24242a); },
    back(x, y) { fill(x - 2, y, 5, 1, 0x2e2e36); },
    head(x, top, look, away) {
      const skin = 0xf2f0f0;
      fill(x - 3, top - 2, 7, 3, skin); fill(x - 4, top - 1, 1, 5, skin); fill(x + 4, top - 1, 1, 5, skin); dot(x - 2, top - 2, WHITE);
      if (away) fill(x - 3, top + 1, 7, 3, skin);
    },
  },
  onepieceluffy: {
    // An open red vest with yellow buttons over a bare chest with an X-shaped scar, blue shorts and sandals; the straw hat with
    // its red band, shaggy black hair and a little scar under the left eye
    front(x, y) { fill(x - 1, y, 3, 6, 0xf0c8a0); dot(x - 2, y + 2, 0xf2d040); dot(x - 2, y + 4, 0xf2d040); line(x - 1, y + 1, x + 1, y + 3, 0xc88a6a); line(x + 1, y + 1, x - 1, y + 3, 0xc88a6a); },
    back() {},
    head(x, top, look, away) {
      const straw = 0xf2d040, strawDark = 0xc8a830, hair = 0x1e1612;
      fill(x - 4, top - 1, 1, 4, hair); fill(x + 4, top - 1, 1, 4, hair);
      fill(x - 6, top, 13, 1, straw); fill(x - 3, top - 3, 7, 3, straw); fill(x - 3, top - 1, 7, 1, 0xd8302a); dot(x - 6, top, strawDark); dot(x + 6, top, strawDark); dot(x - 1, top - 3, strawDark);
      if (away) return;
      dot(x - 2, top + 4, 0xb85a3a); dot(x - 3, top + 4, 0xb85a3a, 0.6);
    },
  },
  onepiecezoro: {
    // A white open-necked shirt with a green haramaki sash round the waist and three swords at the hip; short moss-green hair
    // and three little gold earrings
    front(x, y) { dot(x, y, 0xe8c8a0); fill(x - 3, y + 3, 7, 2, 0x3a8a3a); fill(x - 3, y + 3, 7, 1, 0x5aaa5a); line(x + 3, y + 3, x + 4, y + 6, 0x2a2a30); line(x + 2, y + 4, x + 4, y + 7, 0xf2f2ee); },
    back(x, y) { fill(x - 3, y + 3, 7, 2, 0x3a8a3a); },
    head(x, top, look, away) {
      const hair = 0x5aa84a, dark = 0x3a7a3a;
      fill(x - 3, top - 2, 7, 2, hair); dot(x - 4, top - 1, hair); dot(x + 4, top - 1, hair); dot(x - 1, top - 2, dark); dot(x + 2, top - 2, dark);
      if (away) { fill(x - 3, top, 7, 2, hair); return; }
      for (const ey of [2, 3, 4]) dot(x - 4, top + ey, 0xf2d040);
    },
  },
  onepiecenami: {
    // A blue-and-white striped top and white trousers, a tattoo of a tangerine and pinwheel on the shoulder, the Log Pose on the
    // wrist; long bright orange hair
    front(x, y) { for (let k = 1; k < 6; k += 2) fill(x - 3, y + k, 7, 1, 0xf2f2ee); dot(x + 3, y, 0xf2902a); dot(x + 3, y + 1, 0x3a7a3a); },
    back(x, y) { for (let k = 1; k < 6; k += 2) fill(x - 3, y + k, 7, 1, 0xf2f2ee); fill(x - 2, y, 5, 4, 0xf2802a); },
    head(x, top, look, away) {
      const hair = 0xf2802a, light = 0xf8a050;
      fill(x - 3, top - 2, 7, 2, hair); fill(x - 4, top - 1, 1, 9, hair); fill(x + 4, top - 1, 1, 9, hair); fill(x - 2, top - 2, 3, 1, light);
      if (away) fill(x - 3, top, 7, 8, hair);
    },
  },
  onepiecesanji: {
    // A sharp black suit over a blue shirt and a dark tie; blond hair swept down over one eye, and the curly eyebrow showing
    // under the other; a wisp of smoke rises now and then
    front(x, y) { fill(x - 1, y, 3, 4, 0x6a8ac8); fill(x, y, 1, 5, 0x2a2a30); dot(x - 2, y, 0x3a3a46); dot(x + 2, y, 0x3a3a46); },
    back() {},
    head(x, top, look, away) {
      const hair = 0xf2d870, dark = 0xd0b450;
      fill(x - 3, top - 2, 7, 2, hair); fill(x - 4, top - 1, 1, 6, hair); fill(x + 4, top - 1, 1, 4, hair);
      if (away) { fill(x - 3, top, 7, 4, hair); return; }
      fill(x - 3, top, 3, 4, hair); dot(x - 2, top + 3, dark);
      dot(x + 1, top + 2, dark); dot(x + 2, top + 1, dark); dot(x + 3, top + 2, dark);
      if ((now + x * 0.27) % 10 < 2) { const r = Math.floor((now * 2) % 3); dot(x + 4 + (r % 2), top + 4 - r * 2, 0xd8d8d8, 0.6 - r * 0.15); }
    },
  },
  eldenringtarnished: {
    // Battered steel plate armour over a faded blue surcoat, a tattered cloak at the back; a closed steel helm with a narrow
    // eye slit
    front(x, y, o) { fill(x - 1, y, 3, 6, 0x3a4a6a); fill(x - 1, y + 4, 3, 1, 0x8a6a3a); dot(x - 3, y, shaded(o.shirt, 1.3)); dot(x + 3, y, shaded(o.shirt, 1.3)); dot(x - 2, y + 2, shaded(o.shirt, 1.2)); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x5a4a3a); dot(x - 3, y + 5, 0x3a2a1e); dot(x + 1, y + 5, 0x3a2a1e); },
    head(x, top, look, away) {
      const steel = 0x8a8e96, steelLight = 0xb8bcc4, steelDark = 0x5a5e66;
      fill(x - 4, top - 2, 9, 10, steel); fill(x - 3, top - 3, 7, 1, steel); fill(x - 4, top - 2, 1, 10, steelLight); fill(x + 4, top - 2, 1, 10, steelDark);
      if (away) { fill(x - 1, top - 3, 3, 1, steelLight); return; }
      fill(x - 3, top + 3, 7, 1, 0x1a1a1e); fill(x, top + 4, 1, 3, steelDark);
    },
  },
  eldenringmaiden: {
    // A long deep violet travelling cloak with a gold clasp, its hood up and drawn low, a glimpse of a pale face and a calm
    // golden eye
    front(x, y, o) { fill(x - 3, y, 1, 6, shaded(o.shirt, 1.2)); fill(x + 3, y, 1, 6, shaded(o.shirt, 0.75)); dot(x, y, 0xd8b45a); fill(x - 1, y + 1, 3, 5, shaded(o.shirt, 0.85)); },
    back(x, y, o) { fill(x, y, 1, 6, shaded(o.shirt, 0.8)); },
    head(x, top, look, away, o) {
      const hood = o.shirt;
      fill(x - 4, top - 2, 9, 4, hood); fill(x - 4, top + 2, 2, 6, hood); fill(x + 3, top + 2, 2, 6, hood); fill(x - 3, top - 3, 7, 1, hood);
      if (away) { fill(x - 2, top + 2, 5, 6, hood); return; }
      dot(x + 1, top + 4, 0xf2c850);
    },
  },
  eldenringvalkyrie: {
    // Gilded armour over a red skirt, a long red cape; flowing scarlet hair and a golden winged helm
    front(x, y, o) { fill(x - 2, y, 5, 3, shaded(o.shirt, 1.2)); fill(x - 2, y, 5, 1, 0xf2d888); fill(x - 3, y + 4, 7, 2, 0x8a1e1e); dot(x, y + 2, 0x8a6a2a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xa81e1e); fill(x - 3, y, 1, 6, 0xc8302a); },
    head(x, top, look, away) {
      const hair = 0xc8281e, gold = 0xd8b45a, goldLight = 0xf2d888;
      fill(x - 4, top - 1, 1, 10, hair); fill(x + 4, top - 1, 1, 10, hair); dot(x - 5, top + 6, hair); dot(x + 5, top + 6, hair);
      fill(x - 3, top - 2, 7, 4, gold); fill(x - 3, top - 2, 7, 1, goldLight); line(x - 4, top - 1, x - 6, top - 4, goldLight); line(x + 4, top - 1, x + 6, top - 4, goldLight); dot(x - 5, top - 2, gold); dot(x + 5, top - 2, gold);
      if (away) { fill(x - 3, top + 2, 7, 7, hair); return; }
      fill(x - 3, top + 1, 7, 1, 0x8a6a2a);
    },
  },
  eldenringwitch: {
    // A white fur mantle over a dark blue gown; a tall wide-brimmed blue-black witch's hat with a point, and long hair the
    // colour of embers
    front(x, y) { fill(x - 3, y, 7, 2, 0xf2f2ee); fill(x - 3, y, 7, 1, WHITE); fill(x - 2, y + 2, 5, 4, 0x2a2a4a); dot(x, y + 2, 0x8ab8f0); },
    back(x, y) { fill(x - 3, y, 7, 3, 0xf2f2ee); },
    head(x, top, look, away) {
      const hat = 0x2a2a3a, hair = 0xc8402a;
      fill(x - 4, top, 1, 6, hair); fill(x + 4, top, 1, 6, hair);
      fill(x - 7, top, 15, 1, hat); fill(x - 3, top - 4, 7, 4, hat); fill(x - 2, top - 6, 5, 2, hat); fill(x - 1, top - 8, 3, 2, hat); dot(x, top - 9, hat);
      dot(x - 7, top, 0x4a4a5a); dot(x - 3, top - 4, 0x4a4a5a);
      if (away) fill(x - 3, top + 1, 7, 5, hair);
    },
  },
  sevmdr: {
    // A blue MDR dress shirt with a white collar and a dark narrow tie
    front(x, y) { fill(x - 2, y, 5, 2, 0xf2f2ee); fill(x - 1, y, 1, 6, 0x27406a); fill(x + 1, y, 1, 6, 0x27406a); fill(x, y, 1, 6, 0x1c2d4a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x34527e); fill(x - 3, y, 7, 1, 0x4a6ea0); },
    head(x, top, look, away) { const hair = 0x3a2e22; fill(x - 3, top - 2, 7, 3, hair); fill(x - 3, top - 2, 4, 1, shaded(hair, 1.2)); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  sevmanager: {
    // A sharp purple blazer with a gold tie and a white pocket square
    front(x, y) { fill(x - 3, y, 7, 6, 0x5a2e6a); fill(x - 1, y, 3, 1, 0xf2f2ee); fill(x, y + 1, 1, 5, 0xe8c84a); dot(x - 2, y + 1, 0xe8c84a); fill(x - 3, y, 1, 6, 0x6a3a7a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x4a2458); fill(x - 3, y, 7, 1, 0x6a3a7a); },
    head(x, top, look, away) { const hair = 0x1a1410; fill(x - 3, top - 1, 7, 2, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  sevwellness: {
    // A soft sage wellness cardigan over a pale top
    front(x, y) { fill(x - 3, y, 7, 6, 0x6a8a7a); fill(x - 1, y, 3, 6, 0xdfe6df); fill(x - 3, y, 1, 6, 0x86a699); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x6a8a7a); fill(x - 3, y, 7, 1, 0x86a699); },
  },
  sevsecurity: {
    // A black security suit over a white shirt, an earpiece coil at the collar
    front(x, y) { fill(x - 3, y, 7, 6, 0x24262c); fill(x, y, 1, 6, 0xeef1ec); fill(x - 3, y, 1, 6, 0x34363c); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x1e2024); },
    head(x, top, look, away) { const hair = 0x2a2420; fill(x - 3, top - 1, 7, 2, hair); dot(x + 4, top + 2, 0xbfc7c0); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  offdwight: {
    // A mustard short-sleeve shirt with a brown tie
    front(x, y) { fill(x - 3, y, 7, 6, 0xc8a038); fill(x - 1, y, 1, 2, 0xf2f0ea); fill(x + 1, y, 1, 2, 0xf2f0ea); fill(x, y, 1, 6, 0x5a3a18); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xb8902e); fill(x - 3, y, 7, 1, 0xd8b048); },
    head(x, top, look, away) { const hair = 0x4a3420; fill(x - 3, top - 1, 7, 2, hair); if (!away) { fill(x - 2, top + 1, 1, 1, 0x3a3d42); fill(x + 1, top + 1, 1, 1, 0x3a3d42); } if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  offjim: {
    // A grey suit jacket over a light-blue shirt
    front(x, y) { fill(x - 3, y, 7, 6, 0x8a98a8); fill(x - 1, y, 3, 6, 0xbcd0e0); fill(x, y + 1, 1, 5, 0x4a6a8a); fill(x - 3, y, 1, 6, 0x9aa8b8); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x7a8898); fill(x - 3, y, 7, 1, 0x9aa8b8); },
    head(x, top, look, away) { const hair = 0x3a2a1a; fill(x - 3, top - 2, 7, 3, hair); fill(x - 3, top - 2, 3, 1, shaded(hair, 1.2)); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  offmichael: {
    // A dark suit with a white shirt and a bold red tie
    front(x, y) { fill(x - 3, y, 7, 6, 0x2a3340); fill(x - 1, y, 3, 1, 0xf2f0ea); fill(x, y, 1, 6, 0xc83a3a); fill(x - 3, y, 1, 6, 0x3a4350); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x222a36); fill(x - 3, y, 7, 1, 0x3a4350); },
    head(x, top, look, away) { const hair = 0x2a211a; fill(x - 3, top - 1, 7, 2, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  offpam: {
    // A soft rose cardigan over a pale top
    front(x, y) { fill(x - 3, y, 7, 6, 0xc8a8b8); fill(x - 1, y, 3, 6, 0xeee4ea); fill(x - 3, y, 1, 6, 0xd8bccb); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xc8a8b8); fill(x - 3, y, 7, 1, 0xd8bccb); },
    head(x, top, look, away) { const hair = 0x8a6a42; fill(x - 4, top - 1, 9, 3, hair); fill(x - 4, top + 2, 1, 3, hair); fill(x + 4, top + 2, 1, 3, hair); if (away) fill(x - 4, top + 1, 9, 5, hair); },
  },
  peakytommy: {
    // A dark three-piece suit, a waistcoat with a watch chain, and a flat cap
    front(x, y) { fill(x - 3, y, 7, 6, 0x2a3034); fill(x - 1, y, 3, 6, 0x3a4248); fill(x, y, 1, 4, 0x8a2a2a); dot(x - 1, y + 2, 0xc8a04a); dot(x + 1, y + 3, 0xc8a04a); fill(x - 3, y, 1, 6, 0x3a4248); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x242a2e); fill(x - 3, y, 7, 1, 0x3a4248); },
    head(x, top, look, away) { const cap = 0x3a4044, capL = 0x4e565a; fill(x - 4, top - 1, 9, 2, cap); fill(x - 4, top - 1, 9, 1, capL); fill(x - 5, top, 3, 1, cap); if (away) fill(x - 3, top + 1, 7, 5, 0x2a2420); },
  },
  peakyarthur: {
    // A worn grey-brown suit over a collarless shirt, a flat cap pulled low
    front(x, y) { fill(x - 3, y, 7, 6, 0x3a3632); fill(x - 1, y, 3, 6, 0x4a4440); fill(x, y, 1, 3, 0xcbbfa6); fill(x - 3, y, 1, 6, 0x4a4440); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x322e2a); fill(x - 3, y, 7, 1, 0x4a4440); },
    head(x, top, look, away) { const cap = 0x4a443a, capL = 0x5e564a; fill(x - 4, top - 1, 9, 2, cap); fill(x - 4, top - 1, 9, 1, capL); fill(x - 5, top, 3, 1, cap); if (!away) fill(x - 2, top + 3, 5, 1, 0x7a6a5a, 0.5); if (away) fill(x - 3, top + 1, 7, 5, 0x3a2e22); },
  },
  peakypolly: {
    // An elegant dark 1920s dress with a beaded necklace
    front(x, y) { fill(x - 3, y, 7, 6, 0x2a2030); fill(x - 3, y, 7, 1, 0x3a2e42); fill(x - 1, y + 1, 3, 1, 0xc8a04a); dot(x, y + 2, 0xe8c878); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x241a2a); fill(x - 3, y, 7, 1, 0x3a2e42); },
    head(x, top, look, away) { const hair = 0x2a2018; fill(x - 4, top - 1, 9, 3, hair); fill(x - 4, top + 2, 1, 2, hair); fill(x + 4, top + 2, 1, 2, hair); if (away) fill(x - 4, top + 1, 9, 5, hair); },
  },
  peakybarman: {
    // A cream collarless shirt under a brown waistcoat, with sleeve garters
    front(x, y) { fill(x - 3, y, 7, 6, 0xd8cbb0); fill(x - 2, y, 1, 6, 0x6a4a2a); fill(x + 2, y, 1, 6, 0x6a4a2a); fill(x, y, 1, 6, 0x5a3a1a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xc8bba0); fill(x - 2, y + 2, 1, 1, 0x8a2a2a); fill(x + 2, y + 2, 1, 1, 0x8a2a2a); },
    head(x, top, look, away) { const hair = 0x3a2a1a; fill(x - 3, top - 1, 7, 2, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  madmdon: {
    // A sharp grey suit, a white shirt and a skinny navy tie
    front(x, y) { fill(x - 3, y, 7, 6, 0x6a6e78); fill(x - 1, y, 3, 6, 0xf2ecde); fill(x, y, 1, 6, 0x24344f); fill(x - 3, y, 1, 6, 0x7a7e88); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x5e626c); fill(x - 3, y, 7, 1, 0x7a7e88); },
    head(x, top, look, away) { const hair = 0x241c14; fill(x - 3, top - 2, 7, 3, hair); fill(x - 3, top - 2, 4, 1, shaded(hair, 1.25)); fill(x + 3, top - 1, 1, 2, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  madmroger: {
    // A pinstripe grey suit with an orange tie and a pocket square
    front(x, y) { fill(x - 3, y, 7, 6, 0x5a5e6a); fill(x - 3, y, 1, 6, 0x6a6e78); fill(x - 1, y, 1, 6, 0x6e727c); fill(x + 1, y, 1, 6, 0x6e727c); fill(x, y, 1, 5, 0xc87a2a); dot(x - 2, y + 1, 0xd84a2a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x4e525c); fill(x - 3, y, 7, 1, 0x6a6e78); },
    head(x, top, look, away) { const hair = 0xcfcfc8; fill(x - 3, top - 1, 7, 2, hair); fill(x - 3, top - 1, 3, 1, WHITE); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  madmjoan: {
    // A striking red dress with a pale collar, a brooch and pearls
    front(x, y) { fill(x - 3, y, 7, 6, 0x9a2a2a); fill(x - 3, y, 7, 1, 0xb83a3a); fill(x - 1, y, 3, 1, 0xf2ecde); dot(x, y + 1, 0xe8c878); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x8a2424); fill(x - 3, y, 7, 1, 0xb83a3a); },
    head(x, top, look, away) { const hair = 0x8a3a1a; fill(x - 4, top - 2, 9, 3, hair); fill(x - 3, top - 3, 5, 1, 0xa04e24); if (away) fill(x - 4, top + 1, 9, 5, hair); },
  },
  madmpeggy: {
    // A modest olive work dress with a pale collar
    front(x, y) { fill(x - 3, y, 7, 6, 0x7a8a4a); fill(x - 1, y, 3, 1, 0xeee4ca); fill(x - 3, y, 1, 6, 0x8a9a5a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x6e7e42); fill(x - 3, y, 7, 1, 0x8a9a5a); },
    head(x, top, look, away) { const hair = 0x5a3a1a; fill(x - 3, top - 1, 7, 2, hair); fill(x - 3, top + 1, 1, 1, hair); fill(x + 3, top + 1, 1, 1, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  squidplayer: {
    // A teal numbered tracksuit with a front zip and a white number
    front(x, y) { fill(x - 3, y, 7, 6, 0x1a9a7a); fill(x - 3, y, 7, 1, 0x2ab08a); fill(x, y, 1, 6, 0x147a60); fill(x - 2, y + 1, 2, 2, 0xf0eeea); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x178a6e); fill(x - 3, y, 7, 1, 0x2ab08a); fill(x - 2, y + 1, 4, 3, 0xf0eeea); },
    head(x, top, look, away) { const hair = 0x1a1410; fill(x - 3, top - 1, 7, 2, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  squidguard: {
    // A pink jumpsuit with a black belt and a black mask bearing a pink shape
    front(x, y) { fill(x - 3, y, 7, 6, 0xe84a7a); fill(x - 3, y, 7, 1, 0xf06a94); fill(x - 3, y + 4, 7, 1, 0x1a1a20); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xd8407a); fill(x - 3, y + 4, 7, 1, 0x1a1a20); },
    head(x, top, look, away) { const mask = 0x1a1a20; fill(x - 3, top - 1, 7, 7, mask); fill(x - 3, top - 1, 7, 1, 0x3a3a44); if (!away) ring(x, top + 2, 1.6, 0xe84a7a); },
  },
  squidfrontman: {
    // A long black cloak and a glossy black mask
    front(x, y) { fill(x - 3, y, 7, 6, 0x1a1a20); fill(x - 3, y, 7, 1, 0x2a2a34); fill(x, y, 1, 6, 0x0a0a0e); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x141418); fill(x - 3, y, 7, 1, 0x2a2a34); },
    head(x, top, look, away) { const mask = 0x14141a; fill(x - 3, top - 1, 7, 7, mask); fill(x - 3, top - 1, 4, 1, 0x3a3a44); if (!away) dot(x, top + 2, 0x4a4a54); },
  },
  squidvip: {
    // A gaudy gold VIP jacket over a dark shirt
    front(x, y) { fill(x - 3, y, 7, 6, 0xd6b25e); fill(x - 3, y, 7, 1, 0xe8c878); fill(x - 1, y, 3, 6, 0x2a2a30); fill(x, y, 1, 6, 0x8a2a2a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xc8a24e); fill(x - 3, y, 7, 1, 0xe8c878); },
    head(x, top, look, away) { const hair = 0x2a2420; fill(x - 3, top - 1, 7, 2, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  boyshomelander: {
    // A blue hero suit with a white collar and a red cape behind
    front(x, y) { fill(x - 3, y, 7, 6, 0x2a4a8a); fill(x - 3, y, 7, 1, 0x3a5a9a); fill(x - 1, y, 3, 1, 0xf0f0ee); dot(x, y + 2, 0xf0f0ee); },
    back(x, y) { fill(x - 4, y, 9, 6, 0xd21f2a); fill(x - 4, y, 9, 1, 0xe8404a); fill(x - 4, y + 5, 9, 1, 0x9a1420); },
    head(x, top, look, away) { const hair = 0xd8b85a; fill(x - 3, top - 1, 7, 2, hair); fill(x - 3, top - 1, 3, 1, 0xe8cc70); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  boysstarlight: {
    // A white hero suit with gold trim
    front(x, y) { fill(x - 3, y, 7, 6, 0xf0e8d0); fill(x - 3, y, 7, 1, WHITE); fill(x, y, 1, 6, 0xd6b25e); fill(x - 3, y + 4, 7, 1, 0xd6b25e); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xe4dcc4); fill(x - 3, y, 7, 1, WHITE); },
    head(x, top, look, away) { const hair = 0xe8cc70; fill(x - 4, top - 1, 9, 3, hair); fill(x - 4, top + 2, 1, 2, hair); fill(x + 4, top + 2, 1, 2, hair); if (away) fill(x - 4, top + 1, 9, 5, hair); },
  },
  boysbutcher: {
    // A long black trench coat over a grey tee
    front(x, y) { fill(x - 3, y, 7, 6, 0x1a1a1e); fill(x - 1, y, 3, 6, 0x5a5e62); fill(x - 3, y, 1, 6, 0x2a2a2e); fill(x + 3, y, 1, 6, 0x2a2a2e); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x141418); fill(x - 3, y, 7, 1, 0x2a2a2e); },
    head(x, top, look, away) { const hair = 0x1a1410; fill(x - 3, top - 1, 7, 2, hair); if (!away) fill(x - 2, top + 3, 5, 1, 0x4a3a2a, 0.5); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  boysexec: {
    // A dark corporate suit with a bold red tie
    front(x, y) { fill(x - 3, y, 7, 6, 0x2a2a34); fill(x - 1, y, 3, 1, 0xf0f0ee); fill(x, y, 1, 6, 0xd21f2a); fill(x - 3, y, 1, 6, 0x3a3a44); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x222230); fill(x - 3, y, 7, 1, 0x3a3a44); },
    head(x, top, look, away) { const hair = 0x2a241a; fill(x - 3, top - 1, 7, 2, hair); fill(x - 3, top - 1, 3, 1, shaded(hair, 1.3)); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  falloutvaultsuit: {
    // A blue Vault-Tec jumpsuit with a yellow collar and a number
    front(x, y) { fill(x - 3, y, 7, 6, 0x2e4e82); fill(x - 3, y, 7, 1, 0xf0c020); fill(x, y + 1, 1, 5, 0x1e3a66); fill(x - 1, y + 2, 3, 2, 0xf0c020); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x284470); fill(x - 3, y, 7, 1, 0xf0c020); fill(x - 2, y + 2, 4, 2, 0xe8e2d0); },
    head(x, top, look, away) { const hair = 0x3a2a1a; fill(x - 3, top - 1, 7, 2, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  falloutranger: {
    // A dusty wasteland duster with a round-lensed gas mask
    front(x, y) { fill(x - 3, y, 7, 6, 0x4a443a); fill(x - 1, y, 3, 6, 0x3a352c); fill(x - 3, y, 1, 6, 0x5a544a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x423c32); fill(x - 3, y, 7, 1, 0x5a544a); },
    head(x, top, look, away) { const mask = 0x3a352c, lens = 0x8a9e4a; fill(x - 3, top - 1, 7, 7, mask); fill(x - 3, top - 1, 7, 1, 0x4a443a); if (!away) { dot(x - 1, top + 2, lens); dot(x + 1, top + 2, lens); } },
  },
  falloutraider: {
    // Scrappy patched raider armour with a mismatched metal plate and a mohawk
    front(x, y) { fill(x - 3, y, 7, 6, 0x6a4a2a); fill(x - 1, y, 3, 4, 0x8a8e94); fill(x - 3, y + 3, 7, 1, 0x4a3420); dot(x + 2, y + 1, 0x9aa0a8); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x5a3e24); fill(x - 1, y, 3, 3, 0x7a7e84); },
    head(x, top, look, away) { const hair = 0x8a2a2a, skin = 0xcaa682; fill(x - 1, top - 2, 3, 3, hair); if (!away) fill(x - 3, top + 1, 7, 1, skin, 0.3); if (away) fill(x - 3, top + 1, 7, 5, 0x3a2a1a); },
  },
  falloutscientist: {
    // A Vault-Tec lab coat over a blue jumpsuit
    front(x, y) { fill(x - 3, y, 7, 6, 0xe4e4de); fill(x - 1, y, 3, 6, 0x2e4e82); fill(x - 3, y, 1, 6, 0xf0f0ea); fill(x + 2, y + 1, 1, 1, 0xf0c020); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xd8d8d2); fill(x - 3, y, 7, 1, 0xf0f0ea); },
    head(x, top, look, away) { const hair = 0x9a9488; fill(x - 3, top - 1, 7, 2, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  rickcoat: {
    // A white lab coat over a teal shirt, with wild light-blue hair
    front(x, y) { fill(x - 3, y, 7, 6, 0xd8e0e4); fill(x - 1, y, 3, 6, 0x3a8a8a); fill(x - 3, y, 1, 6, 0xeef2f4); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xccd4d8); fill(x - 3, y, 7, 1, 0xeef2f4); },
    head(x, top, look, away) { const hair = 0x9ad8e8; fill(x - 4, top - 3, 9, 3, hair); fill(x - 5, top - 2, 1, 2, hair); fill(x + 4, top - 2, 1, 2, hair); fill(x - 2, top - 4, 2, 1, hair); fill(x + 1, top - 4, 2, 1, hair); if (away) fill(x - 4, top, 9, 5, hair); },
  },
  rickmorty: {
    // A yellow tee and blue jeans with a brown bowl cut
    front(x, y) { fill(x - 3, y, 7, 6, 0xf0d84a); fill(x - 3, y, 7, 1, 0xf8e46a); fill(x - 3, y + 4, 7, 2, 0x3a6ac8); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xe0c83e); fill(x - 3, y + 4, 7, 2, 0x3a6ac8); },
    head(x, top, look, away) { const hair = 0x8a5a2a; fill(x - 4, top - 2, 9, 3, hair); fill(x - 4, top + 1, 1, 1, hair); fill(x + 4, top + 1, 1, 1, hair); if (away) fill(x - 4, top + 1, 9, 5, hair); },
  },
  ricksummer: {
    // A teal top with long reddish hair
    front(x, y) { fill(x - 3, y, 7, 6, 0x4aa0a0); fill(x - 3, y, 7, 1, 0x5ab0b0); fill(x - 1, y, 3, 1, 0xe8c8d0); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x429090); fill(x - 3, y, 7, 1, 0x5ab0b0); },
    head(x, top, look, away) { const hair = 0xc87a3a; fill(x - 4, top - 2, 9, 3, hair); fill(x - 4, top + 1, 1, 4, hair); fill(x + 4, top + 1, 1, 4, hair); if (away) fill(x - 4, top + 1, 9, 6, hair); },
  },
  rickbirdperson: {
    // A feathered brown figure with a crested head
    front(x, y) { fill(x - 3, y, 7, 6, 0x6a5a3a); fill(x - 3, y, 7, 1, 0x7a6a4a); fill(x - 1, y + 1, 3, 4, 0x8a7a5a); for (let i = 0; i < 3; i++) dot(x - 2 + i * 2, y + 3, 0x4a3a2a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x5a4a30); fill(x - 3, y, 7, 1, 0x7a6a4a); },
    head(x, top, look, away) { const feather = 0x5a4a30, light = 0x8a7a5a; fill(x - 3, top - 1, 7, 3, feather); fill(x - 1, top - 3, 3, 2, light); fill(x, top - 4, 1, 2, light); if (!away) fill(x + 3, top + 1, 2, 1, 0xf0a020); if (away) fill(x - 3, top + 1, 7, 4, feather); },
  },
  mirrorsuit: {
    // A sleek minimalist grey suit with a thin cyan collar line
    front(x, y) { fill(x - 3, y, 7, 6, 0x5a5e66); fill(x - 1, y, 3, 6, 0xeef0f2); fill(x, y, 1, 2, 0x4ab0f0); fill(x - 3, y, 1, 6, 0x6a6e76); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x4e525a); fill(x - 3, y, 7, 1, 0x6a6e76); },
    head(x, top, look, away) { const hair = 0x2a241a; fill(x - 3, top - 1, 7, 2, hair); fill(x - 3, top - 1, 3, 1, shaded(hair, 1.3)); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  mirrorjumpsuit: {
    // A grey exercise jumpsuit with a printed number
    front(x, y) { fill(x - 3, y, 7, 6, 0x8a8e94); fill(x - 3, y, 7, 1, 0x9aa0a8); fill(x - 1, y + 2, 3, 2, 0xc8cdd4); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x7e828a); fill(x - 3, y, 7, 1, 0x9aa0a8); },
    head(x, top, look, away) { const hair = 0x3a2a1a; fill(x - 3, top - 1, 7, 2, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  mirrortech: {
    // A dark tech hoodie with a drawstring
    front(x, y) { fill(x - 3, y, 7, 6, 0x2a3d5a); fill(x - 3, y, 7, 1, 0x3a4d6a); fill(x - 1, y, 1, 4, 0xc8cdd4); fill(x + 1, y, 1, 4, 0xc8cdd4); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x243450); fill(x - 3, y - 1, 7, 2, 0x3a4d6a); },
    head(x, top, look, away) { const hair = 0x1a1410; fill(x - 3, top - 1, 7, 2, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  mirrorgown: {
    // A glossy minimalist white gown
    front(x, y) { fill(x - 3, y, 7, 6, 0xeef0f2); fill(x - 3, y, 7, 1, WHITE); fill(x - 1, y + 2, 3, 4, 0xd8dce0); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xe4e8ec); fill(x - 3, y, 7, 1, WHITE); },
    head(x, top, look, away) { const hair = 0x4a3a2a; fill(x - 4, top - 1, 9, 3, hair); fill(x - 4, top + 2, 1, 3, hair); fill(x + 4, top + 2, 1, 3, hair); if (away) fill(x - 4, top + 1, 9, 5, hair); },
  },
  westcowboy: {
    // A chambray shirt under a leather vest, with a cowboy hat
    front(x, y) { fill(x - 3, y, 7, 6, 0x8a6a3a); fill(x - 1, y, 3, 6, 0xaab4c0); fill(x - 3, y, 1, 6, 0x9a7a4a); dot(x, y + 2, 0xc8a04a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x7a5a2e); fill(x - 3, y, 7, 1, 0x9a7a4a); },
    head(x, top, look, away) { const felt = 0x5a3a1e, feltL = 0x7a5230; fill(x - 5, top, 11, 1, felt); fill(x - 3, top - 2, 7, 2, felt); fill(x - 3, top - 2, 7, 1, feltL); if (away) fill(x - 3, top + 1, 7, 5, 0x3a2a1a); },
  },
  westhost: {
    // A plain grey diagnostic bodysuit
    front(x, y) { fill(x - 3, y, 7, 6, 0xc8b8a8); fill(x - 3, y, 7, 1, 0xe0d0c0); fill(x, y + 1, 1, 4, 0xa89888); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xbcaa9a); fill(x - 3, y, 7, 1, 0xe0d0c0); },
    head(x, top, look, away) { const hair = 0x5a3a2a; fill(x - 3, top - 1, 7, 2, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  westtech: {
    // A dark QA lab jumpsuit with a lanyard badge
    front(x, y) { fill(x - 3, y, 7, 6, 0x2a2d32); fill(x - 3, y, 7, 1, 0x3a3d42); fill(x, y, 1, 4, 0x8ad0d8); fill(x - 1, y + 3, 2, 1, 0xf0f0ee); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x24262a); fill(x - 3, y, 7, 1, 0x3a3d42); },
    head(x, top, look, away) { const hair = 0x2a241a; fill(x - 3, top - 1, 7, 2, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  westgunslinger: {
    // All in black: a black duster and a black hat
    front(x, y) { fill(x - 3, y, 7, 6, 0x1a1a1e); fill(x - 1, y, 3, 6, 0x2a2a2e); fill(x - 3, y, 1, 6, 0x2a2a2e); dot(x, y + 2, 0x8a8e94); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x141418); fill(x - 3, y, 7, 1, 0x2a2a2e); },
    head(x, top, look, away) { const felt = 0x14141a, feltL = 0x2a2a30; fill(x - 5, top, 11, 1, felt); fill(x - 3, top - 2, 7, 2, felt); fill(x - 3, top - 2, 7, 1, feltL); if (away) fill(x - 3, top + 1, 7, 5, 0x1a1a1e); },
  },
  bttfmarty: {
    // A puffy orange-red life-preserver vest over a pale plaid shirt
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xd8e0ea); fill(x - 3, y, 2, 6, 0xd8402a); fill(x + 2, y, 2, 6, 0xd8402a); fill(x - 1, y + 1, 3, 1, 0x3a5a8a); dot(x, y + 3, 0x3a5a8a); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xd8402a); fill(x - 3, y, 7, 1, 0xf26a4a); },
  },
  bttfdoc: {
    // A white lab coat with wild white hair sticking out
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xeef2ee); fill(x - 3, y, 1, 6, WHITE); fill(x + 3, y, 1, 6, 0xcacec8); fill(x, y + 1, 1, 5, 0xbcc0ba); dot(x - 1, y + 2, 0x3aa0a0); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xeef2ee); fill(x + 3, y, 1, 6, 0xcacec8); },
    head(x, top, look, away) { const hair = 0xeef2ee; for (const dx of [-4, -3, 3, 4]) fill(x + dx, top - 2, 1, 4, hair); fill(x - 4, top - 2, 9, 1, hair); dot(x - 5, top - 1, hair); dot(x + 5, top - 1, hair); if (away) fill(x - 3, top + 1, 7, 5, 0xd8d8d2); },
  },
  bttffuture: {
    // A self-adjusting silver jacket with a red zip and glowing blue cuffs
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xb0b8c4); fill(x - 3, y, 7, 1, 0xd6dce4); fill(x, y, 1, 6, 0xd8402a); dot(x - 3, y + 4, 0x6fd0ff); dot(x + 3, y + 4, 0x6fd0ff); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xb0b8c4); fill(x - 3, y, 7, 1, 0xd6dce4); },
  },
  bttfsodajerk: {
    // A white soda-jerk shirt with a red bow tie and a folded paper hat
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xeef2ee); fill(x - 3, y, 1, 6, WHITE); fill(x - 1, y, 3, 1, 0xd8402a); dot(x, y + 1, 0xd8402a); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xeef2ee); },
    head(x, top, look, away) { fill(x - 3, top - 1, 7, 2, 0xf6f2ea); fill(x - 3, top - 1, 7, 1, WHITE); dot(x, top - 2, 0xf6f2ea); if (away) fill(x - 3, top + 1, 7, 5, 0x3a2a1a); },
  },
  jpranger: {
    // A khaki warden shirt with a brimmed bush hat
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xbcae7e); fill(x - 3, y, 7, 1, 0xcebf8e); fill(x - 1, y, 1, 6, 0x9a8c5e); dot(x + 1, y + 1, 0x5a7a3a); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xbcae7e); fill(x - 3, y, 7, 1, 0xcebf8e); },
    head(x, top, look, away) { const hat = 0x7a5a2a; fill(x - 5, top - 1, 11, 1, hat); fill(x - 3, top - 3, 7, 2, hat); fill(x - 3, top - 3, 7, 1, 0x9a7038); if (away) fill(x - 3, top + 1, 7, 5, 0x3a2a1a); },
  },
  jpscientist: {
    // A white lab coat over a shirt, with an amber tie
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xeef2ee); fill(x - 3, y, 1, 6, WHITE); fill(x + 3, y, 1, 6, 0xcacec8); fill(x, y, 1, 4, 0xe8a830); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xeef2ee); fill(x + 3, y, 1, 6, 0xcacec8); },
  },
  jpvisitor: {
    // A visitor's light shirt with a park lanyard, under a red ball cap
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xd8d4c8); fill(x - 3, y, 7, 1, 0xe6e2d6); fill(x, y, 1, 5, 0xd83020); dot(x, y + 2, 0xe8c838); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xd8d4c8); },
    head(x, top, look, away) { fill(x - 3, top - 1, 7, 1, 0xd83020); fill(x + 2, top - 1, 3, 1, 0xd83020); if (away) fill(x - 3, top + 1, 7, 5, 0x3a2a1a); },
  },
  jpnedry: {
    // A loud, rumpled shirt and round glasses
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xd86030); fill(x - 3, y, 7, 1, 0xe88050); dot(x - 1, y + 2, 0xe8c838); dot(x + 2, y + 1, 0x3ad0c0); dot(x, y + 4, 0x3ad0c0); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xd86030); },
    head(x, top, look, away) { if (!away) { dot(x - 2, top + 2, 0x2a2e36); dot(x + 2, top + 2, 0x2a2e36); dot(x, top + 2, 0x2a2e36); } fill(x - 3, top - 1, 7, 1, 0x3a2a1a); if (away) fill(x - 3, top + 1, 7, 5, 0x3a2a1a); },
  },
  wicksuit: {
    // A sharp black suit over a black shirt, buttoned to the collar
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x15151a); fill(x - 3, y, 1, 6, 0x2e2e36); fill(x, y, 1, 5, 0x05050a); fill(x - 1, y, 3, 1, 0x05050a); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x15151a); fill(x - 3, y, 7, 1, 0x2e2e36); },
  },
  wickconciergeoutfit: {
    // A midnight-blue tailcoat with gold trim and buttons
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x1e2440); fill(x - 3, y, 1, 6, 0x2e3a60); fill(x, y, 1, 6, 0xf0ead6); fill(x - 3, y, 7, 1, 0xd6b25e); dot(x - 1, y + 2, 0xd6b25e); dot(x - 1, y + 4, 0xd6b25e); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x1e2440); fill(x - 3, y, 7, 1, 0xd6b25e); },
  },
  wicksommelier: {
    // A tailor's dark waistcoat over a white shirt, a measuring tape round the neck
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xf0ece2); fill(x - 2, y, 5, 6, 0x2a2a30); fill(x, y, 1, 6, 0xf0ece2); fill(x - 3, y, 1, 1, 0x9a1a1a); fill(x + 3, y, 1, 1, 0x9a1a1a); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x2a2a30); },
  },
  wickadjudicator: {
    // Severe high-table black buttoned high, with gold pins at the collar
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x101016); fill(x - 3, y, 1, 6, 0x24242c); fill(x - 1, y, 3, 2, 0x05050a); dot(x - 2, y, 0xd6b25e); dot(x + 2, y, 0xd6b25e); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x101016); },
    head(x, top, look, away) { if (away) fill(x - 3, top + 1, 7, 5, 0x2a2a2e); },
  },
  kingsmangalahad: {
    // A navy double-breasted suit with a tie and a white pocket square
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x1e2a44); fill(x - 3, y, 1, 6, 0x2e3e60); fill(x, y, 1, 5, 0xeef2ee); dot(x, y + 1, 0x8a1a1a); dot(x, y + 3, 0x8a1a1a); dot(x + 2, y + 1, 0xeef2ee); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x1e2a44); fill(x - 3, y, 7, 1, 0x2e3e60); },
  },
  kingsmanmerlin: {
    // A dark green roll-neck jumper under a tweed waistcoat
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x2a4a34); fill(x - 3, y, 7, 1, 0x3a5e44); fill(x - 2, y + 1, 1, 5, 0x6a6050); fill(x + 2, y + 1, 1, 5, 0x6a6050); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x2a4a34); },
    head(x, top, look, away) { if (!away) { fill(x - 2, top + 1, 5, 1, 0x1a1a1e); dot(x - 2, top + 2, 0x8ac0e0); dot(x + 2, top + 2, 0x8ac0e0); } fill(x - 3, top - 1, 7, 1, 0x6a5a4a); if (away) fill(x - 3, top + 1, 7, 5, 0x6a5a4a); },
  },
  kingsmantailor: {
    // Shirtsleeves with a dark apron and a tape measure round the neck
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xeae4d6); fill(x - 2, y + 1, 5, 5, 0x3a3a40); fill(x - 3, y, 1, 1, 0xc8a24a); fill(x + 3, y, 1, 1, 0xc8a24a); dot(x - 1, y + 3, 0xc8b878); dot(x + 1, y + 4, 0xc8b878); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x3a3a40); },
  },
  kingsmanagent: {
    // A grey morning suit with a burgundy tie
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x4a4e58); fill(x - 3, y, 1, 6, 0x5e626c); fill(x, y, 1, 5, 0xeef2ee); fill(x, y + 1, 1, 3, 0x6a2020); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x4a4e58); fill(x - 3, y, 7, 1, 0x5e626c); },
  },
  wonkawonka: {
    // A plum tailcoat over a gold waistcoat, with a top hat
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x8a4ac0); fill(x - 3, y, 1, 6, 0xaa74e0); fill(x - 1, y, 3, 6, 0xe8c838); fill(x, y + 1, 1, 4, 0x8a1a1a); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x8a4ac0); fill(x - 3, y + 4, 7, 2, 0x5a2e84); },
    head(x, top, look, away) { const hat = 0x8a4ac0; fill(x - 4, top - 1, 9, 1, hat); fill(x - 3, top - 4, 7, 3, hat); fill(x - 3, top - 2, 7, 1, 0xe8c838); if (away) fill(x - 3, top + 1, 7, 5, 0x6a5038); },
  },
  wonkaoompa: {
    // An Oompa-Loompa's orange skin and white dungarees with a brown belt
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xf0ece0); fill(x - 2, y, 1, 6, 0x8a5a2a); fill(x + 1, y, 1, 6, 0x8a5a2a); fill(x - 3, y + 3, 7, 1, 0x6a3a1c); dot(x - 2, y + 1, 0xe8c838); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xf0ece0); fill(x - 3, y + 3, 7, 1, 0x6a3a1c); },
    head(x, top, look, away) { fill(x - 3, top, 7, 6, 0xe8782e); if (!away) { dot(x - 1, top + 2, PALETTE.eye); dot(x + 1, top + 2, PALETTE.eye); } fill(x - 3, top - 2, 7, 2, 0x3aa84a); dot(x, top - 3, 0x3aa84a); },
  },
  wonkacharlie: {
    // A ticket winner's simple tan coat with a golden ticket in the pocket
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x9a7a4a); fill(x - 3, y, 7, 1, 0xb89a68); fill(x, y, 1, 5, 0x7a5a2e); fill(x + 1, y + 2, 2, 2, 0xe8c838); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x9a7a4a); },
  },
  wonkaguide: {
    // A candy-striped pink-and-white guide's blazer with a mint bow tie
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xf6c0d8); for (let k = -3; k < 4; k += 2) fill(x + k, y, 1, 6, 0xf06aa8); fill(x, y, 1, 1, 0x5ad0a0); dot(x, y + 1, 0x5ad0a0); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xf6c0d8); for (let k = -3; k < 4; k += 2) fill(x + k, y, 1, 6, 0xf06aa8); },
  },
  wesgustave: {
    // A plum concierge tailcoat with gold buttons and a bow tie
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x6a2a5a); fill(x - 3, y, 1, 6, 0x8a4a7a); fill(x, y, 1, 6, 0xf2ead2); dot(x, y, 0xd8a848); dot(x - 1, y + 2, 0xc8a24a); dot(x + 1, y + 2, 0xc8a24a); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x6a2a5a); fill(x - 3, y + 4, 7, 2, 0x4a1a3e); },
  },
  weszero: {
    // A lobby boy's plum tunic with a pillbox cap
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x7a2a3a); fill(x - 3, y, 1, 6, 0x9a4a5a); fill(x, y, 1, 6, 0xd8a848); dot(x - 1, y + 2, 0xc8a24a); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x7a2a3a); },
    head(x, top, look, away) { fill(x - 2, top - 2, 5, 2, 0x7a2a3a); fill(x - 2, top - 2, 5, 1, 0x9a4a5a); dot(x, top - 3, 0xd8a848); if (away) fill(x - 3, top + 1, 7, 5, 0x2a1a14); },
  },
  wesagatha: {
    // A baker's dusty-rose dress with a white apron
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xd87a8a); fill(x - 3, y, 7, 1, 0xe89aa6); fill(x - 1, y, 3, 6, 0xf2ead2); dot(x, y + 2, 0xd8a848); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xd87a8a); },
  },
  wesguest: {
    // A mustard travelling coat with a powder-blue scarf
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xd8a848); fill(x - 3, y, 1, 6, 0xe8c070); fill(x, y, 1, 5, 0x9a7428); fill(x - 2, y, 5, 1, 0x9ac0d0); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xd8a848); fill(x - 3, y, 7, 1, 0xe8c070); },
  },
  mibsuit: {
    // A black suit over a white shirt and a thin black tie, with shades
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x15151a); fill(x - 3, y, 1, 6, 0x2e2e36); fill(x, y, 1, 6, 0xe8eaee); fill(x, y + 1, 1, 4, 0x05050a); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x15151a); fill(x - 3, y, 7, 1, 0x2e2e36); },
    head(x, top, look, away) { if (!away) { fill(x - 2, top + 1, 5, 1, 0x15151a); dot(x - 2, top + 1, 0x2a2e36); dot(x + 2, top + 1, 0x2a2e36); } },
  },
  mibzed: {
    // A senior agent's charcoal suit, white shirt, silver tie; grey hair
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x24242c); fill(x - 3, y, 1, 6, 0x3a3a44); fill(x, y, 1, 6, 0xe8eaee); fill(x, y + 1, 1, 4, 0xb4b8c0); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x24242c); },
    head(x, top, look, away) { fill(x - 3, top - 1, 7, 1, 0xc4c8ce); if (away) fill(x - 3, top + 1, 7, 5, 0xc4c8ce); },
  },
  mibalien: {
    // A trench coat hiding an alien -- a green hand at the cuff gives it away
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x6a5a3a); fill(x - 3, y, 1, 6, 0x8a7a52); fill(x, y, 1, 6, 0x5a4a2e); dot(x + 3, y + 4, 0x5ad86a); dot(x - 3, y + 4, 0x5ad86a); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x6a5a3a); },
    head(x, top, look, away) { if (!away) { dot(x - 1, top + 1, 0x5ad86a); dot(x + 1, top + 1, 0x5ad86a); } fill(x - 3, top - 1, 7, 1, 0x4a3a2a); if (away) fill(x - 3, top + 1, 7, 5, 0x6a9a4a); },
  },
  mibworm: {
    // A pink worm-guy in a tiny shirt and tie, grinning
    front(x, y, o) { fill(x - 2, y, 5, 6, 0xe89aa6); fill(x - 2, y, 1, 6, 0xf4b4c0); fill(x, y, 1, 5, 0xe8eaee); fill(x, y + 1, 1, 3, 0x2a2a30); },
    back(x, y, o) { fill(x - 2, y, 5, 6, 0xe89aa6); },
    head(x, top, look, away) { fill(x - 2, top, 5, 6, 0xe89aa6); if (!away) { dot(x - 1, top + 2, PALETTE.eye); dot(x + 1, top + 2, PALETTE.eye); fill(x - 1, top + 4, 3, 1, 0xc87888); } },
  },
  tronblue: {
    // A black program suit lined with glowing cyan circuitry
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x12202c); fill(x, y, 1, 6, 0x3ad0e8); fill(x - 3, y, 1, 6, 0x3ad0e8, 0.7); fill(x + 3, y, 1, 6, 0x3ad0e8, 0.7); dot(x, y + 2, 0x9ff0ff); dot(x - 2, y + 4, 0x3ad0e8); dot(x + 2, y + 4, 0x3ad0e8); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x12202c); disc(x, y + 2, 2, 0x3ad0e8, 0.8); dot(x, y + 2, 0x9ff0ff); },
  },
  tronorange: {
    // A black suit lined with glowing orange circuitry -- a hostile program
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x1a1410); fill(x, y, 1, 6, 0xe8782e); fill(x - 3, y, 1, 6, 0xe8782e, 0.7); fill(x + 3, y, 1, 6, 0xe8782e, 0.7); dot(x, y + 2, 0xf8a860); dot(x - 2, y + 4, 0xe8782e); dot(x + 2, y + 4, 0xe8782e); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x1a1410); disc(x, y + 2, 2, 0xe8782e, 0.8); dot(x, y + 2, 0xf8a860); },
  },
  tronflynn: {
    // The Creator's long pale-grey robe with a faint cyan seam
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xd0d4da); fill(x - 3, y, 1, 6, 0xe4e8ee); fill(x, y, 1, 6, 0xb0b4ba); dot(x, y + 1, 0x3ad0e8); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xd0d4da); },
    head(x, top, look, away) { fill(x - 3, top - 1, 7, 1, 0xd8dce2); if (away) fill(x - 3, top + 1, 7, 5, 0xd8dce2); },
  },
  tronsiren: {
    // A sleek white grid bodysuit with bright white-cyan seams
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x1a2630); fill(x, y, 1, 6, 0xe8f8ff); fill(x - 2, y, 1, 6, 0x9ff0ff); fill(x + 2, y, 1, 6, 0x9ff0ff); dot(x, y + 3, 0x3ad0e8); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x1a2630); fill(x, y, 1, 6, 0x9ff0ff); },
  },
  dbzgoku: {
    // An orange gi over a blue undershirt, with a blue sash
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xe87828); fill(x - 3, y, 1, 6, 0xf89a48); fill(x - 1, y, 3, 4, 0x2a50c0); fill(x - 3, y + 4, 7, 1, 0x2a50c0); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xe87828); fill(x - 3, y + 4, 7, 1, 0x2a50c0); dot(x, y + 1, 0xf2c84a); },
    head(x, top, look, away) { const hair = 0x1a1a1e; fill(x - 4, top - 3, 9, 2, hair); fill(x - 3, top - 5, 2, 2, hair); fill(x, top - 5, 2, 3, hair); fill(x + 2, top - 5, 2, 2, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  dbzvegeta: {
    // White Saiyan battle armour with blue underlayer and gold shoulder guards
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x2a50c0); fill(x - 2, y, 5, 5, 0xe8eef2); fill(x - 2, y, 5, 1, 0xf6f8fa); fill(x - 4, y, 2, 2, 0xe8c050); fill(x + 2, y, 2, 2, 0xe8c050); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x2a50c0); fill(x - 2, y, 5, 5, 0xe8eef2); },
    head(x, top, look, away) { const hair = 0x1a1a22; fill(x - 3, top - 4, 7, 3, hair); fill(x - 1, top - 6, 3, 3, hair); if (away) fill(x - 3, top + 1, 7, 5, hair); },
  },
  dbzpiccolo: {
    // A purple Namekian gi with a white cape and a red sash
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x7a4a9a); fill(x - 3, y, 1, 6, 0x9a6aba); fill(x - 3, y + 4, 7, 1, 0xd83020); dot(x, y + 1, 0xf2ead2); },
    back(x, y, o) { fill(x - 4, y, 9, 6, 0xf2ead2); fill(x - 4, y, 9, 1, WHITE); },
    head(x, top, look, away) { fill(x - 3, top - 2, 7, 2, 0x5aa85a); fill(x - 3, top - 2, 7, 1, 0x7ac87a); dot(x - 2, top - 3, 0x3a7a3a); dot(x + 2, top - 3, 0x3a7a3a); if (away) fill(x - 3, top + 1, 7, 5, 0x5aa85a); },
  },
  dbzstudent: {
    // A plain white training gi with a black belt
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xf0ece0); fill(x - 3, y, 1, 6, 0xf8f6ee); fill(x, y, 1, 4, 0xd8d2c4); fill(x - 3, y + 4, 7, 1, 0x1a1a1e); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xf0ece0); fill(x - 3, y + 4, 7, 1, 0x1a1a1e); },
  },
  amongusred: {
    // A red crewmate spacesuit with a glass visor and a backpack
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xd83020); fill(x - 3, y, 1, 6, 0xf05a40); fill(x + 2, y, 1, 6, 0xa01e12); fill(x, y + 1, 3, 2, 0x9fd8e8); dot(x + 1, y + 1, WHITE, 0.6); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xd83020); fill(x - 4, y + 1, 1, 4, 0xa01e12); },
  },
  amonguscyan: {
    // A cyan crewmate spacesuit with a glass visor
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x44d0e0); fill(x - 3, y, 1, 6, 0x8ae8f0); fill(x + 2, y, 1, 6, 0x2a9aa8); fill(x, y + 1, 3, 2, 0xcff4fa); dot(x + 1, y + 1, WHITE, 0.6); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x44d0e0); fill(x - 4, y + 1, 1, 4, 0x2a9aa8); },
  },
  amongusyellow: {
    // A yellow crewmate spacesuit with a glass visor
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xf2d84a); fill(x - 3, y, 1, 6, 0xfcec84); fill(x + 2, y, 1, 6, 0xc0a828); fill(x, y + 1, 3, 2, 0x9fd8e8); dot(x + 1, y + 1, WHITE, 0.6); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0xf2d84a); fill(x - 4, y + 1, 1, 4, 0xc0a828); },
  },
  amongusimposter: {
    // A dark crewmate suit with a faintly red visor -- a little bit sus
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x2a2e36); fill(x - 3, y, 1, 6, 0x44484e); fill(x + 2, y, 1, 6, 0x14161a); fill(x, y + 1, 3, 2, 0x8a2a2a); dot(x + 1, y + 1, 0xf05a40, 0.7); },
    back(x, y, o) { fill(x - 3, y, 7, 6, 0x2a2e36); fill(x - 4, y + 1, 1, 4, 0x14161a); },
  },
  tokyohappi: {
    // An indigo happi coat with white collar bands down the front, a white hem, and a knotted sash in the agent's colour
    front(x, y, o) { fill(x - 3, y, 1, 6, 0xeae6dc); fill(x + 3, y, 1, 6, 0xeae6dc); fill(x - 3, y + 5, 7, 1, 0xeae6dc); fill(x - 3, y + 3, 7, 1, o.accent); dot(x, y + 3, shaded(o.accent, 1.3)); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x2a3e6a); disc(x, y + 3, 2.2, 0xeae6dc); ring(x, y + 3, 2.4, 0x1c2c4e, 0.8); },
  },
  tokyokimono: {
    // A wine kimono crossed left-over-right with a pale collar and a wide obi sash in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 1, 0xe8dcc8); dot(x - 2, y + 1, 0xe8dcc8); dot(x + 1, y + 2, 0xe8dcc8); fill(x - 3, y + 3, 7, 2, o.accent); fill(x - 3, y + 3, 7, 1, shaded(o.accent, 1.2)); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x7a2a4a); fill(x - 3, y + 3, 7, 2, 0x3a1424); },
  },
  tokyosalaryman: {
    // A charcoal suit: dark lapels over a white shirt with a thin tie in the agent's colour
    front(x, y, o) { fill(x - 3, y, 2, 6, 0x242a34); fill(x + 2, y, 2, 6, 0x242a34); fill(x, y, 1, 6, 0xf0f0ea); fill(x, y + 1, 1, 4, o.accent); dot(x, y, 0xf0f0ea); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x2e3440); fill(x - 3, y, 7, 1, 0x3e4450); },
  },
  tokyostreet: {
    // A bright bomber jacket open over a white tee, a ribbed collar and cuffs, and headphones round the neck
    front(x, y, o) { fill(x - 1, y, 3, 6, 0xf0f0ea); fill(x - 3, y, 7, 1, shaded(0xff4ab8, 0.85)); fill(x - 3, y + 5, 7, 1, shaded(0xff4ab8, 0.85)); dot(x - 3, y + 1, o.accent); dot(x + 3, y + 1, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xff4ab8); fill(x - 3, y, 7, 1, 0x3ae8ff); },
  },
  egyptking: {
    // A bare chest over a white kilt with a broad gold-and-blue wesekh collar and a gold belt in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 2, 0xd8b24e); fill(x - 3, y + 1, 7, 1, 0x1f4e8a); fill(x - 3, y + 3, 7, 3, 0xeae2cc); fill(x - 3, y + 3, 7, 1, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xd79a64); fill(x - 3, y, 7, 2, 0xd8b24e); },
  },
  egyptpriest: {
    // A white linen robe with a leopard skin slung over one shoulder, its spots dark on gold
    front(x, y) { fill(x - 3, y, 7, 6, 0xeae2cc); fill(x - 3, y, 3, 6, 0xcaa23e); for (const [dx, dy] of [[-2, 1], [-1, 3], [-2, 4], [-1, 2]]) dot(x + dx, y + dy, 0x241a12); dot(x - 2, y, 0x3a2418); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xeae2cc); fill(x + 1, y, 3, 6, 0xcaa23e); for (const dy of [1, 3, 4]) dot(x + 2, y + dy, 0x241a12); },
  },
  egyptqueen: {
    // A pleated white sheath dress with gold straps and a turquoise-and-gold broad collar in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xeae2cc); for (let k = x - 2; k < x + 4; k += 2) fill(k, y, 1, 6, 0xd8d0b8); fill(x - 3, y, 7, 2, 0x3ab8a0); fill(x - 3, y, 7, 1, o.accent); dot(x - 1, y, 0xd8b24e); dot(x + 1, y, 0xd8b24e); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xeae2cc); fill(x - 3, y, 7, 1, 0xd8b24e); },
  },
  egyptworker: {
    // A plain short linen kilt and a bare chest with a simple cord in the agent's colour
    front(x, y, o) { fill(x - 3, y + 3, 7, 3, 0xcfc2a0); fill(x - 3, y + 3, 7, 1, 0xe8dcbe); fill(x - 3, y, 7, 3, 0xd79a64); dot(x, y + 2, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xcfc2a0); fill(x - 3, y, 7, 3, 0xd79a64); },
  },
  vikingwarrior: {
    // A chain-mail shirt over a tunic with a fur-trimmed shoulder mantle and leather straps in the agent's colour
    front(x, y, o) { for (let gy = 0; gy < 6; gy++) for (let gx = -3; gx < 4; gx++) if ((gx + gy) % 2 === 0) dot(x + gx, y + gy, 0xb4b8c0); fill(x - 3, y, 7, 1, 0x7a6a54); fill(x - 1, y + 1, 1, 5, o.accent); dot(x + 1, y + 2, 0x4a4e56); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x8a8e96); fill(x - 3, y, 7, 2, 0x7a6a54); },
  },
  vikingmaiden: {
    // A red wool dress with an apron-dress over it held by two oval brooches and a string of amber beads
    front(x, y, o) { fill(x - 2, y, 4, 6, 0x9a4a3a); dot(x - 2, y, 0xc8a050); dot(x + 1, y, 0xc8a050); for (const dy of [1, 2]) dot(x, y + dy, o.accent); fill(x - 1, y + 2, 3, 1, 0xd8a850); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x7a2a3a); fill(x - 2, y, 5, 6, 0x9a4a3a); },
  },
  vikingjarl: {
    // A rich tunic under a heavy fur cloak over the shoulders, a big silver cloak-pin in the agent's colour
    front(x, y, o) { fill(x - 3, y, 2, 6, 0x6a5440); fill(x + 2, y, 2, 6, 0x6a5440); fill(x - 3, y, 2, 2, 0x8a7258); fill(x + 2, y, 2, 2, 0x8a7258); dot(x - 1, y + 1, 0xc8ccd2); dot(x - 1, y + 1, o.accent); fill(x - 1, y + 3, 3, 1, 0xc8a050); },
    back(x, y) { fill(x - 4, y, 9, 6, 0x6a5440); fill(x - 4, y, 9, 2, 0x8a7258); },
  },
  vikingseer: {
    // A deep blue-grey hooded cloak over a robe, hung with little amulets and a bone pendant in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x3a4a5a); fill(x - 3, y, 7, 1, 0x4e5e6e); fill(x - 1, y, 3, 2, 0x2a3444); dot(x, y + 3, 0xd8ccae); dot(x - 2, y + 2, o.accent); dot(x + 2, y + 2, 0xc8a050); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x3a4a5a); fill(x - 2, y - 1, 5, 2, 0x2a3444); },
  },
  aztecjaguarw: {
    // A spotted jaguar-pelt tunic with dark rosettes and a bone-and-jade necklace in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xc89a4a); for (const [dx, dy] of [[-2, 1], [0, 2], [2, 1], [-1, 4], [2, 4]]) dot(x + dx, y + dy, 0x3a2a18); fill(x - 2, y, 5, 1, 0x3ab8a0); dot(x, y, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xc89a4a); for (const [dx, dy] of [[-2, 1], [1, 2], [2, 4], [-1, 3]]) dot(x + dx, y + dy, 0x3a2a18); },
  },
  azteceaglew: {
    // A white eagle-feather costume with a gold collar and down-feathers over the chest, a beak-like clasp in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xe8dcc0); for (let gx = -3; gx < 4; gx += 2) { dot(x + gx, y + 2, 0xd0c4a4); dot(x + gx, y + 4, 0xd0c4a4); } fill(x - 3, y, 7, 1, 0xd8b24e); dot(x, y + 1, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xe8dcc0); for (let gx = -3; gx < 4; gx += 2) dot(x + gx, y + 3, 0xd0c4a4); },
  },
  aztecpriest: {
    // A dark robe with ash-grey markings and a necklace, a bold red-and-jade band in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x2a2a30); fill(x - 3, y, 7, 1, 0x3e3e46); fill(x - 3, y + 2, 7, 1, 0xb83a2a); dot(x - 1, y + 3, 0x3ab8a0); dot(x + 1, y + 3, o.accent); dot(x, y + 4, 0xd8b24e); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x2a2a30); fill(x - 3, y + 2, 7, 1, 0xb83a2a); },
  },
  aztecnoble: {
    // A rich green feather cloak over the shoulders with a gold pectoral and a turquoise clasp in the agent's colour
    front(x, y, o) { fill(x - 3, y, 2, 6, 0x3a9a5a); fill(x + 2, y, 2, 6, 0x3a9a5a); fill(x - 3, y, 2, 1, 0x5ac07a); fill(x + 2, y, 2, 1, 0x5ac07a); fill(x - 1, y, 3, 2, 0xd8b24e); dot(x, y + 1, o.accent); dot(x, y + 3, 0x3ab8a0); },
    back(x, y) { fill(x - 4, y, 9, 6, 0x3a9a5a); fill(x - 4, y, 9, 1, 0x5ac07a); },
  },
  obsastronomer: {
    // A white lab coat open over a dark jumper, a lanyard and a pocket pen in the agent's colour
    front(x, y, o) { fill(x - 3, y, 2, 6, 0xeef0f4); fill(x + 2, y, 2, 6, 0xeef0f4); fill(x - 1, y, 3, 6, 0x2a3444); fill(x, y, 1, 3, o.accent); dot(x + 1, y + 1, 0xd8b24e); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xeef0f4); fill(x - 3, y, 7, 1, 0xd8dce2); },
  },
  obsprofessor: {
    // A tweed jacket with elbow patches over a waistcoat, a little bow tie in the agent's colour
    front(x, y, o) { fill(x - 3, y, 2, 6, 0x6a5a3a); fill(x + 2, y, 2, 6, 0x6a5a3a); fill(x - 1, y, 3, 6, 0x9a8a5a); dot(x, y, o.accent); dot(x - 1, y, o.accent); dot(x - 3, y + 3, 0x4a3e26); dot(x + 3, y + 3, 0x4a3e26); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x7a6a4a); dot(x - 3, y + 3, 0x4a3e26); dot(x + 3, y + 3, 0x4a3e26); },
  },
  obsastronaut: {
    // A white EVA suit with a soft helmet ring, chest control panel and life-support hoses in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xeef0f4); fill(x - 3, y, 7, 1, 0xd0d4da); fill(x - 2, y + 2, 3, 2, 0x2a3040); dot(x - 2, y + 2, o.accent); dot(x, y + 3, 0x3ae87a); fill(x + 2, y + 1, 1, 4, 0xc8ccd2); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xeef0f4); fill(x - 2, y + 1, 5, 4, 0xd0d4da); },
    head(x, top, look, away) { if (away) return; ring(x, top + 2, 4, 0xc8ccd2, 0.8); },
  },
  obsstargazer: {
    // A chunky blue knit jumper under a long scarf wound round the neck in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x3a5a7a); for (let gy = y + 1; gy < y + 6; gy += 2) fill(x - 3, gy, 7, 1, 0x2e4a66, 0.6); fill(x - 3, y, 7, 1, o.accent); fill(x - 2, y + 1, 1, 5, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x3a5a7a); fill(x - 3, y, 7, 1, 0x4a6a8a); },
  },
  tikibartender: {
    // A red aloha shirt covered in little pale palm/flower prints, open at the collar, a flower tucked in the pocket in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xe8404a); for (const [dx, dy] of [[-2, 1], [1, 2], [-1, 4], [2, 3], [0, 1]]) dot(x + dx, y + dy, 0xf0d0a0); fill(x - 1, y, 3, 1, 0xf0ece0); dot(x + 2, y + 1, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xe8404a); for (const [dx, dy] of [[-2, 2], [1, 1], [0, 4], [2, 3]]) dot(x + dx, y + dy, 0xf0d0a0); },
  },
  tikisurfer: {
    // A bare tanned chest with a lei of leaves and a wetsuit top knotted at the waist in the agent's colour
    front(x, y, o) { fill(x - 3, y + 3, 7, 3, 0x1a6a78); fill(x - 3, y + 3, 7, 1, 0x2a9aa8); fill(x - 3, y, 7, 3, 0xcf9a66); for (let gx = -2; gx < 3; gx += 2) dot(x + gx, y, 0x4f9a5a); dot(x, y + 1, o.accent); },
    back(x, y) { fill(x - 3, y + 3, 7, 3, 0x1a6a78); fill(x - 3, y, 7, 3, 0xcf9a66); },
  },
  tikiluau: {
    // A grass skirt over the shoulders worn as a cape with a thick flower lei round the neck in the agent's colour
    front(x, y, o) { fill(x - 3, y + 2, 7, 4, 0x8aae4a); for (let gx = -3; gx < 4; gx++) dot(x + gx, y + 5, 0x6a8e36); fill(x - 3, y, 7, 2, 0xf06a9a); dot(x - 2, y, 0xf2c040); dot(x + 1, y, o.accent); dot(x + 2, y + 1, 0xf2c040); },
    back(x, y) { fill(x - 3, y + 2, 7, 4, 0x8aae4a); fill(x - 3, y, 7, 1, 0xf06a9a); },
  },
  tikitourist: {
    // A loud yellow floral shirt with a camera on a strap across the chest in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xf2c040); for (const [dx, dy] of [[-2, 1], [1, 3], [-1, 4], [2, 1]]) dot(x + dx, y + dy, 0xe8604a); line(x - 3, y, x + 3, y + 3, 0x3a2a1e); fill(x + 1, y + 3, 2, 2, 0x2a2a30); dot(x + 1, y + 3, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xf2c040); for (const [dx, dy] of [[-1, 2], [2, 4], [-2, 3]]) dot(x + dx, y + dy, 0xe8604a); },
  },
  dinerwaitress: {
    // A teal waitress dress under a white half-apron with a pencil tucked at the waist and a little name pin in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x4ab8a8); fill(x - 2, y + 3, 5, 3, 0xf0ece0); fill(x - 3, y, 7, 1, 0x6ad8c8); dot(x + 1, y + 1, o.accent); dot(x + 2, y + 4, 0xf2c040); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x4ab8a8); fill(x - 2, y + 3, 5, 3, 0xf0ece0); },
  },
  dinercook: {
    // A white cook's tee under a greasy apron, a folded paper hat and a towel over one shoulder in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xf0ece0); fill(x - 2, y + 1, 5, 5, 0xe0dccc); fill(x - 2, y + 2, 5, 1, 0xd8a0a0, 0.5); dot(x - 3, y, o.accent); fill(x + 2, y, 2, 3, 0xf0ece0); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xf0ece0); },
    head(x, top, look, away) { if (away) return; fill(x - 2, top - 1, 5, 2, 0xf0ece0); fill(x - 2, top - 1, 5, 1, 0xf8f4ea); },
  },
  dinergreaser: {
    // A black leather jacket with a popped collar over a white tee, a slick comb in the pocket in the agent's colour
    front(x, y, o) { fill(x - 3, y, 2, 6, 0x2a2a30); fill(x + 2, y, 2, 6, 0x2a2a30); fill(x - 1, y, 3, 6, 0xf0ece0); fill(x - 3, y, 2, 1, 0x3e3e46); fill(x + 2, y, 2, 1, 0x3e3e46); dot(x - 2, y + 1, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x2a2a30); fill(x - 3, y, 7, 1, 0x3e3e46); dot(x, y + 2, 0x8a8e96); },
  },
  dinerpoodle: {
    // A white blouse over a pink full skirt with a little poodle appliqué and a neck scarf in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 3, 0xf0ece0); fill(x - 3, y + 3, 7, 3, 0xf06a9a); fill(x - 3, y + 3, 7, 1, 0xf890b0); dot(x, y, o.accent); dot(x - 1, y + 4, 0xf0ece0); dot(x, y + 4, 0xf0ece0); },
    back(x, y) { fill(x - 3, y, 7, 3, 0xf0ece0); fill(x - 3, y + 3, 7, 3, 0xf06a9a); },
  },
  courtjudge: {
    // A flowing black judicial robe with a pleated white jabot (collar band) at the throat and a slim accent trim in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x1a1a22); fill(x - 3, y, 7, 1, 0x2e2e38); fill(x - 1, y, 3, 3, 0xf0ece0); dot(x, y + 1, 0xd8d4c8); fill(x, y + 3, 1, 3, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x1a1a22); fill(x - 3, y, 7, 1, 0x2e2e38); },
  },
  courtlawyer: {
    // A sharp navy suit with lapels over a white shirt and a tie in the agent's colour
    front(x, y, o) { fill(x - 3, y, 2, 6, 0x242a38); fill(x + 2, y, 2, 6, 0x242a38); fill(x - 1, y, 3, 6, 0xf0ece0); fill(x, y + 1, 1, 4, o.accent); dot(x, y, 0xf0ece0); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x2a3040); fill(x - 3, y, 7, 1, 0x3a4050); },
  },
  courtbailiff: {
    // A tan bailiff's uniform shirt with a shoulder patch, a dark tie and a gold star pinned on in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x6a7258); fill(x - 3, y, 7, 1, 0x7e8668); fill(x, y + 1, 1, 4, 0x2a2a30); dot(x - 2, y + 1, 0xc8a050); dot(x + 2, y + 1, o.accent); fill(x - 3, y + 1, 1, 2, 0x5a624a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x6a7258); fill(x - 3, y, 7, 1, 0x7e8668); },
  },
  courtclerk: {
    // A plum blazer over a cream blouse with a lanyard and reading glasses tucked at the collar in the agent's colour
    front(x, y, o) { fill(x - 3, y, 2, 6, 0x5a3a5a); fill(x + 2, y, 2, 6, 0x5a3a5a); fill(x - 1, y, 3, 6, 0xeee6d8); dot(x - 1, y, o.accent); fill(x, y + 2, 1, 3, 0x8a8e96); dot(x + 1, y + 1, 0x6a6a72); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x5a3a5a); fill(x - 3, y, 7, 1, 0x6e4a6e); },
  },
  artsmock: {
    // A loose cream painter's smock spattered all over with little dabs of bright paint, a brush tucked at the chest in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xe8e4da); fill(x - 3, y, 7, 1, 0xf0ece0); for (const [dx, dy, c] of [[-2, 1, 0xe8404a], [1, 2, 0x4a8ad8], [-1, 4, 0xf2c040], [2, 4, 0x4ac86a], [0, 3, 0xff5ab0]]) dot(x + dx, y + dy, c); fill(x + 2, y, 1, 3, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xe8e4da); for (const [dx, dy, c] of [[-2, 2, 0xe8404a], [1, 3, 0x4ac86a], [2, 1, 0x4a8ad8]]) dot(x + dx, y + dy, c); },
  },
  artberet: {
    // A striped Breton shirt under a slim scarf, worn with a beret (on the head); a splash of the agent's colour at the collar
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xf0ece0); for (let gy = y + 1; gy < y + 6; gy += 2) fill(x - 3, gy, 7, 1, 0x2a3a5a); fill(x - 1, y, 3, 1, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xf0ece0); for (let gy = y + 1; gy < y + 6; gy += 2) fill(x - 3, gy, 7, 1, 0x2a3a5a); },
    head(x, top, look, away) { if (away) return; fill(x - 3, top, 7, 1, 0x1a1a22); fill(x - 2, top - 1, 5, 1, 0x1a1a22); dot(x + 2, top - 1, 0x2a2a30); },
  },
  artsculptor: {
    // A heavy canvas sculptor's apron dusted with grey clay over a dark tee, a wooden tool in the chest pocket in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x2a2a30); fill(x - 2, y + 1, 5, 5, 0x9a7a5a); fill(x - 2, y + 1, 5, 1, 0xb8946a); for (const [dx, dy] of [[-1, 2], [1, 4], [0, 3]]) dot(x + dx, y + dy, 0xcac0b0); dot(x + 2, y + 1, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x2a2a30); fill(x - 3, y, 7, 1, 0x9a7a5a); },
  },
  artstudent: {
    // A casual denim shirt over a graphic tee with a canvas tote strap across the chest and a paint smudge in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x5a7a9a); fill(x - 1, y, 3, 6, 0xd8d0c0); dot(x, y + 2, 0xe8404a); line(x - 3, y, x + 3, y + 3, 0xa88a5a); dot(x + 1, y + 3, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x5a7a9a); fill(x - 3, y, 7, 1, 0x6a8aaa); },
  },
  aquariumwetsuit: {
    // A black neoprene wetsuit with a bright accent panel down one side and a zip, the panel in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x24303c); fill(x - 3, y, 7, 1, 0x34404c); fill(x - 3, y, 1, 6, o.accent); fill(x, y, 1, 6, 0x4a5058); dot(x, y + 1, 0x8a8e96); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x24303c); fill(x + 3, y, 1, 6, 0x3a4a5a); },
  },
  aquariumkeeper: {
    // A teal aquarist polo tucked into chest-high waders with a shoulder strap and a whistle in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 3, 0x2a8a9a); fill(x - 3, y + 3, 7, 3, 0x3a4a3a); fill(x - 3, y, 7, 1, 0x3aa8b8); fill(x - 1, y + 2, 1, 2, 0x2a3a2a); dot(x + 1, y, 0xf2c040); dot(x, y + 1, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 3, 0x2a8a9a); fill(x - 3, y + 3, 7, 3, 0x3a4a3a); },
  },
  aquariumbiologist: {
    // A white lab coat open over a navy tee with a clipboard tucked under one arm and a pen in the agent's colour
    front(x, y, o) { fill(x - 3, y, 2, 6, 0xeef0f4); fill(x + 2, y, 2, 6, 0xeef0f4); fill(x - 1, y, 3, 6, 0x24344a); fill(x + 1, y + 2, 2, 3, 0xd8d0b8); dot(x + 1, y + 2, 0x8a7a5a); dot(x - 1, y + 1, o.accent); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xeef0f4); fill(x - 3, y, 7, 1, 0xd8dce2); },
  },
  aquariumscuba: {
    // A dark wetsuit under a buoyancy vest with regulator hoses and an air tank on the back in the agent's colour
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x24303c); fill(x - 2, y + 1, 5, 4, 0x3a4a5a); fill(x - 2, y + 1, 1, 4, o.accent); fill(x + 1, y, 1, 4, 0x8a8e96); dot(x - 1, y + 1, 0xf2c040); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x24303c); fill(x - 1, y - 1, 3, 6, 0xf0ece0); fill(x - 1, y - 1, 3, 1, 0xe03a4a); fill(x - 1, y + 4, 3, 1, 0x8a8e96); },
  },
  circusringmaster: {
    // A red tailcoat with gold frogging over a white shirt front; a shining top hat; black tails behind
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xcc1f1f); fill(x - 1, y, 3, 6, 0xf2e8d8); dot(x, y + 1, 0xe8b84a); dot(x, y + 3, 0xe8b84a); fill(x - 3, y, 1, 6, 0xf05a40); fill(x + 3, y, 1, 6, 0xaa1818); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x8a1414); fill(x - 3, y + 4, 2, 3, 0x6a1010); fill(x + 2, y + 4, 2, 3, 0x6a1010); },
    head(x, top, look, away) { const hat = 0x1e1e26, hatL = 0x3a3a46; fill(x - 4, top - 1, 9, 1, hat); fill(x - 3, top - 6, 7, 5, hat); fill(x - 3, top - 6, 1, 5, hatL); fill(x - 3, top - 2, 7, 1, 0xcc1f1f); },
  },
  circusclown: {
    // A baggy polka-dot clown suit in clashing colours with a frilled collar, a shock of red hair and a red nose
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x4c8bf5); fill(x - 3, y, 7, 1, 0xf2e8d8); dot(x - 1, y + 2, 0xf2d35b); dot(x + 2, y + 3, 0xd8402a); dot(x, y + 4, 0xf2d35b); fill(x - 3, y, 1, 6, 0x6aa0f8); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x3a6ad0); },
    head(x, top, look, away) { const hair = 0xf05a40; fill(x - 5, top, 1, 3, hair); fill(x + 4, top, 1, 3, hair); fill(x - 5, top - 1, 2, 1, hair); fill(x + 3, top - 1, 2, 1, hair); if (!away) dot(x, top + 3, 0xd8402a); },
  },
  circusacrobat: {
    // A sleek sequined leotard with a gold sash, built for the trapeze
    front(x, y, o) { fill(x - 3, y, 7, 6, shaded(o.shirt, 1)); fill(x - 3, y + 2, 7, 1, 0xe8b84a); for (const dx of [-2, 0, 2]) dot(x + dx, y + (dx ? 4 : 1), WHITE, 0.6); fill(x - 3, y, 1, 6, shaded(o.shirt, 1.2)); },
    back(x, y, o) { fill(x - 3, y, 7, 6, shaded(o.shirt, 0.9)); },
  },
  circusstrongman: {
    // A classic red singlet over a barrel chest and bare arms, a studded belt, a handlebar moustache
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xd8402a); fill(x - 2, y, 1, 6, 0xf0c89a); fill(x + 1, y, 1, 6, 0xf0c89a); fill(x - 3, y + 4, 7, 1, 0x5a3b25); dot(x, y + 4, 0xe8b84a); fill(x - 3, y, 1, 6, 0xf05a40); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xaa2a1a); fill(x - 3, y + 4, 7, 1, 0x5a3b25); },
    head(x, top, look, away) { const hair = 0x2a2018; fill(x - 2, top - 1, 5, 1, hair); if (!away) { fill(x - 3, top + 3, 2, 1, hair); fill(x + 2, top + 3, 2, 1, hair); } },
  },
  museumcurator: {
    // A tweed jacket with a knitted tie and elbow patches, and round spectacles
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x6a5a3e); fill(x - 1, y, 3, 6, 0xe8e2d4); dot(x, y, 0x6a2a2e); dot(x, y + 1, 0x6a2a2e); dot(x - 3, y + 3, 0x4a3e2a); dot(x + 3, y + 3, 0x4a3e2a); fill(x - 3, y, 1, 6, 0x7a6a4a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x5a4a30); },
    head(x, top, look, away) { if (!away) { dot(x - 2, top + 2, 0x2a2620); dot(x + 1, top + 2, 0x2a2620); dot(x - 1, top + 2, 0x2a2620); } },
  },
  museumpaleo: {
    // A khaki field shirt with buttoned pockets and a utility strap, under a wide safari hat
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xb0a06a); fill(x - 3, y, 7, 1, 0xc8b884); fill(x - 2, y + 2, 2, 2, 0x8a7a4a); fill(x + 1, y + 2, 2, 2, 0x8a7a4a); fill(x - 3, y, 1, 6, 0x8a2a1e); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x9a8a5a); fill(x - 3, y + 1, 7, 1, 0x6a5a3a); },
    head(x, top, look, away) { const hat = 0x8a7a4a, hatL = 0xa89a6a; fill(x - 5, top - 1, 11, 1, hat); fill(x - 3, top - 4, 7, 3, hat); fill(x - 3, top - 4, 7, 1, hatL); },
  },
  museumdocent: {
    // A smart green blazer over a white shirt, a lanyard and a little gold name badge
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x2e4a3e); fill(x - 1, y, 3, 6, 0xe8e2d4); fill(x, y, 1, 4, 0xc8a44e); dot(x + 2, y + 1, 0xc8a44e); fill(x - 3, y, 1, 6, 0x3e5a4e); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x244038); },
  },
  museumprofessor: {
    // A white lab coat over a dark waistcoat and a burgundy bowtie, a pen in the pocket, wild white hair
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xe8e4da); fill(x - 1, y, 3, 6, 0x3a3a42); dot(x, y + 1, 0x6a2a2e); dot(x - 1, y + 1, 0x6a2a2e); dot(x + 1, y + 1, 0x6a2a2e); fill(x + 2, y + 2, 1, 2, 0xc8a44e); fill(x - 3, y, 1, 6, 0xf2eee6); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xdcd8ce); },
    head(x, top, look, away) { const hair = 0xe8e2d4; fill(x - 4, top, 1, 3, hair); fill(x + 3, top, 1, 3, hair); fill(x - 4, top - 1, 2, 1, hair); fill(x + 2, top - 1, 2, 1, hair); },
  },
  northpolesanta: {
    // A red fur-trimmed coat with a black belt and gold buckle; a white beard-front; a red hat with white trim
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xc62828); fill(x - 3, y, 7, 1, 0xf2f6fb); fill(x - 3, y + 3, 7, 1, 0x2a2a2e); dot(x, y + 3, 0xe8c24a); fill(x - 1, y, 3, 3, 0xf2f6fb); fill(x - 3, y, 1, 6, 0xe34b3c); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x9a1e1e); fill(x - 3, y, 7, 1, 0xf2f6fb); },
    head(x, top, look, away) { const red = 0xc62828, white = 0xf2f6fb; fill(x - 3, top - 1, 7, 1, white); fill(x - 3, top - 4, 6, 3, red); fill(x + 2, top - 6, 2, 3, red); dot(x + 3, top - 7, white); if (!away) fill(x - 2, top + 3, 5, 1, white); },
  },
  northpoleelf: {
    // A green tunic with a belt and gold buttons; a long pointy hat with a bell and pointed ears
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x2e6b3e); fill(x - 3, y + 3, 7, 1, 0x6a4428); dot(x, y + 3, 0xe8c24a); fill(x - 3, y, 7, 1, 0x3f8a52); for (let i = -2; i <= 2; i += 2) dot(x + i, y + 1, 0xe8c24a); fill(x - 3, y, 1, 6, 0x3f8a52); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x1f4e2c); },
    head(x, top, look, away) { const green = 0x2e6b3e; fill(x - 2, top - 2, 5, 2, green); fill(x, top - 4, 3, 2, green); fill(x + 1, top - 6, 2, 2, green); dot(x + 2, top - 7, 0xe8c24a); if (!away) { dot(x - 4, top + 2, 0xf0c89a); dot(x + 3, top + 2, 0xf0c89a); } },
  },
  northpolemrsclaus: {
    // A red dress under a white apron; grey hair in a bun and round spectacles
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xc62828); fill(x - 2, y + 1, 5, 5, 0xf2f6fb); fill(x - 2, y + 1, 5, 1, 0xdfe8f0); fill(x - 3, y, 1, 6, 0xe34b3c); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x9a1e1e); },
    head(x, top, look, away) { const hair = 0xcfd4da; fill(x - 3, top - 1, 7, 1, hair); fill(x - 2, top - 3, 5, 2, hair); fill(x - 1, top - 4, 3, 1, hair); if (!away) { dot(x - 1, top + 2, 0x2a2620); dot(x + 1, top + 2, 0x2a2620); } },
  },
  northpolesweater: {
    // A festive knit sweater: deep green with red fair-isle bands and a white-and-gold snowflake
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x1f4e2c); fill(x - 3, y, 7, 1, 0xd83030); fill(x - 3, y + 5, 7, 1, 0xd83030); dot(x, y + 2, 0xf2f6fb); dot(x - 2, y + 3, 0xf2f6fb); dot(x + 2, y + 3, 0xf2f6fb); dot(x, y + 4, 0xf2f6fb); dot(x - 1, y + 3, 0xe8c24a); dot(x + 1, y + 3, 0xe8c24a); fill(x - 3, y, 1, 6, 0x2e6b3e); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x1f4e2c); fill(x - 3, y, 7, 1, 0xd83030); fill(x - 3, y + 5, 7, 1, 0xd83030); },
  },
  soundstagedirector: {
    // A dark jacket with a red scarf thrown over, and a flat beret
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x2a2a30); fill(x - 1, y, 3, 2, 0xd83030); fill(x - 1, y + 2, 3, 4, 0x3a3a42); dot(x - 2, y + 2, 0xd83030); fill(x - 3, y, 1, 6, 0x3a3a42); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x242428); fill(x - 1, y, 3, 1, 0xd83030); },
    head(x, top, look, away) { const beret = 0x2a2a30; fill(x - 3, top - 2, 7, 2, beret); fill(x - 3, top - 2, 7, 1, 0x44444c); dot(x + 2, top - 3, beret); },
  },
  soundstagestarlet: {
    // A glamorous gown with a white fur stole and gold jewels; styled hair and dark sunglasses
    front(x, y, o) { fill(x - 3, y, 7, 6, shaded(o.shirt, 1)); fill(x - 3, y, 7, 2, 0xe8e4d8); dot(x - 2, y + 3, 0xe8c24a); dot(x + 1, y + 4, 0xe8c24a); fill(x - 3, y, 1, 6, shaded(o.shirt, 1.2)); },
    back(x, y, o) { fill(x - 3, y, 7, 6, shaded(o.shirt, 0.9)); fill(x - 3, y, 7, 2, 0xe8e4d8); },
    head(x, top, look, away) { if (!away) { fill(x - 2, top + 2, 5, 1, 0x1a1a1e); dot(x - 2, top + 2, 0x3a3a42); dot(x + 2, top + 2, 0x3a3a42); } },
  },
  soundstagecrew: {
    // An olive work shirt with a tool belt and a radio clipped on, a beanie and a headset
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x2e3a2e); fill(x - 3, y + 3, 7, 1, 0x1a1a1e); dot(x - 2, y + 3, 0x8a8e96); dot(x + 2, y + 2, 0x2a2a2e); fill(x - 3, y, 1, 6, 0x3e4a3e); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x263226); },
    head(x, top, look, away) { const beanie = 0x3a3a42; fill(x - 3, top - 2, 7, 2, beanie); if (!away) { fill(x + 3, top + 1, 1, 2, 0x2a2a2e); dot(x + 3, top, 0x2a2a2e); } },
  },
  soundstagetux: {
    // A black tuxedo with satin lapels, a white shirt front and a black bowtie; slicked hair
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x1c1c20); fill(x - 1, y, 3, 6, 0xf2f0ea); dot(x, y, 0x1a1a1e); fill(x - 1, y + 1, 3, 1, 0x1a1a1e); fill(x - 3, y, 2, 6, 0x2a2a30); fill(x + 2, y, 2, 6, 0x2a2a30); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x18181c); },
    head(x, top, look, away) { const hair = 0x2a2420; fill(x - 2, top - 1, 5, 1, hair); fill(x - 2, top - 1, 1, 2, hair); fill(x + 2, top - 1, 1, 2, hair); },
  },
  cultrobe: {
    // A plain white acolyte's robe with a rope belt and a little gold sun pin; a soft hood frames the face
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xeceae2); fill(x - 3, y + 3, 7, 1, 0xc8bfa8); dot(x, y + 1, 0xe0b24a); fill(x - 3, y, 1, 6, 0xf6f2ea); fill(x + 3, y, 1, 6, 0xdcd8cc); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xdcd8cc); },
    head(x, top, look, away) { const hood = 0xeceae2; fill(x - 4, top - 2, 9, 2, hood); fill(x - 4, top - 2, 9, 1, 0xf6f2ea); fill(x - 4, top, 1, 4, hood); fill(x + 4, top, 1, 4, hood); if (away) fill(x - 3, top + 1, 7, 4, hood); },
  },
  cultleader: {
    // A gilded white robe with a radiant gold collar and trim, and a thin gold circlet
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xf2ead8); fill(x - 3, y, 7, 1, 0xe0b24a); dot(x, y + 1, 0xe0b24a); dot(x - 1, y + 2, 0xe0b24a); dot(x + 1, y + 2, 0xe0b24a); fill(x - 3, y, 1, 6, 0xf6f2ea); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xe8dcc0); fill(x - 3, y, 7, 1, 0xe0b24a); },
    head(x, top, look, away) { const gold = 0xe0b24a; fill(x - 3, top - 2, 7, 1, gold); dot(x - 2, top - 3, gold); dot(x, top - 3, gold); dot(x + 2, top - 3, gold); },
  },
  cultelder: {
    // A weathered sage-green hooded robe with a wooden bead necklace; the face sits in shadow
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x8a8a6a); fill(x - 3, y + 2, 7, 1, 0x6a5a3a); dot(x - 1, y + 2, 0x4a3a2a); dot(x + 1, y + 2, 0x4a3a2a); fill(x - 3, y, 1, 6, 0x9a9a7a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x6a6a52); },
    head(x, top, look, away) { const hood = 0x6a6a52; fill(x - 4, top - 2, 9, 3, hood); fill(x - 4, top - 2, 9, 1, 0x7a7a60); fill(x - 3, top, 7, 1, 0x3a3a2e); if (away) fill(x - 3, top + 1, 7, 4, hood); else fill(x - 2, top + 1, 5, 1, 0x3a3a2e, 0.4); },
  },
  cultinitiate: {
    // A plain undyed beige shift with a paper sun badge; no hood yet
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xc8bfa8); fill(x - 3, y + 3, 7, 1, 0xa89878); dot(x + 1, y + 1, 0xe0b24a); fill(x - 3, y, 1, 6, 0xd8cfb8); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xb0a890); },
  },
  esportsjersey: {
    // A black team jersey with cyan and magenta trim, a player number and a sponsor flash
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x14141c); fill(x - 3, y, 7, 1, 0x2ad8e8); fill(x - 3, y + 5, 7, 1, 0xe82ad8); dot(x, y + 2, 0xf0f4ff); dot(x - 1, y + 3, 0x9ae82a); fill(x - 3, y, 1, 6, 0x2a2a36); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x14141c); fill(x - 3, y, 7, 1, 0x2ad8e8); dot(x, y + 2, 0xf0f4ff); dot(x, y + 3, 0xf0f4ff); },
  },
  esportshoodie: {
    // A dark gaming hoodie with neon drawstrings and a logo; hood bunched at the back
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x2a2a36); fill(x - 1, y, 1, 4, 0x2ad8e8); fill(x + 1, y, 1, 4, 0xe82ad8); dot(x, y + 2, 0x9ae82a); fill(x - 3, y, 1, 6, 0x3a3a46); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x24242e); fill(x - 3, y, 7, 2, 0x2a2a36); },
    head(x, top, look, away) { if (away) fill(x - 3, top + 1, 7, 2, 0x2a2a36); },
  },
  esportscaster: {
    // A sharp dark suit with a white shirt and a shoutcaster's headset mic with a live light
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x14141c); fill(x - 1, y, 3, 6, 0xf0f4ff); dot(x, y + 1, 0x2ad8e8); fill(x - 3, y, 2, 6, 0x1e1e28); fill(x + 2, y, 2, 6, 0x1e1e28); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x101016); },
    head(x, top, look, away) { if (!away) { fill(x + 3, top + 1, 1, 3, 0x2a2a36); dot(x + 3, top + 3, 0xf03a8a); } },
  },
  esportspro: {
    // A purple pro jersey with a compression arm-sleeve, under a team cap
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x8a3ae8); fill(x - 3, y, 7, 1, 0xb86af0); fill(x - 3, y + 5, 7, 1, 0x2ad8e8); dot(x, y + 2, 0xf0f4ff); fill(x + 3, y, 1, 6, 0x6a2ab8); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x6a2ab8); fill(x - 3, y, 7, 1, 0xb86af0); dot(x, y + 2, 0xf0f4ff); },
    head(x, top, look, away) { const cap = 0x14141c; fill(x - 3, top - 2, 7, 2, cap); fill(x - 3, top - 2, 7, 1, 0x2a2a36); if (!away) fill(x + 3, top, 2, 1, cap); dot(x, top - 2, 0x2ad8e8); },
  },
  cavemanfur: {
    // A rough one-shoulder fur tunic with a jagged hem; a bone stuck in messy hair
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x7a5436); fill(x - 3, y, 7, 1, 0x9a7452); fill(x + 1, y, 1, 6, 0xe8c0a0); for (const dx of [-2, 0, 2]) dot(x + dx, y + 4, 0x5a3b25); fill(x - 3, y, 1, 6, 0x8a6442); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x5a4028); fill(x - 3, y, 7, 1, 0x7a5436); },
    head(x, top, look, away) { if (!away) dot(x + 2, top - 1, 0xe0d6b8); fill(x - 3, top - 1, 7, 1, 0x3a2818); },
  },
  cavemanhunter: {
    // A hide vest over a bare chest, a bone necklace and a quiver strap
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xb89a6a); fill(x - 1, y, 3, 6, 0x6a4a2a); dot(x - 1, y + 1, 0xe0d6b8); dot(x, y + 1, 0xe0d6b8); dot(x + 1, y + 1, 0xe0d6b8); fill(x + 3, y, 1, 6, 0x8a2a1e); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x9a7a4a); fill(x + 1, y, 1, 6, 0x8a2a1e); },
  },
  cavemanshaman: {
    // A dark hide robe daubed with ochre marks and a bone pendant, under an antler headdress
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x5a4028); fill(x - 3, y + 2, 7, 1, 0xa0472a); dot(x - 1, y + 3, 0xc89a3e); dot(x + 1, y + 4, 0xc89a3e); dot(x, y + 1, 0xe0d6b8); fill(x - 3, y, 1, 6, 0x6a4a30); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x4a3420); },
    head(x, top, look, away) { const antler = 0xd8cbb0; fill(x - 4, top - 1, 9, 1, 0x3a2818); dot(x - 4, top - 2, antler); dot(x - 5, top - 3, antler); dot(x + 4, top - 2, antler); dot(x + 5, top - 3, antler); dot(x, top - 2, 0xe0d6b8); },
  },
  cavemanchief: {
    // A big fur cloak over the shoulders, a tusk necklace and a crown of tusks
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x8a6a42); fill(x - 4, y, 2, 4, 0x9a7a52); fill(x + 3, y, 2, 4, 0x9a7a52); fill(x - 1, y, 3, 6, 0x6a4a2a); fill(x - 1, y + 1, 3, 1, 0xe0d6b8); fill(x - 3, y, 1, 6, 0xa88a5a); },
    back(x, y) { fill(x - 4, y, 9, 6, 0x7a5436); fill(x - 4, y, 9, 1, 0x9a7452); },
    head(x, top, look, away) { const tusk = 0xe0d6b8; fill(x - 4, top - 1, 9, 1, 0x6a4a2a); fill(x - 4, top - 3, 1, 2, tusk); fill(x + 4, top - 3, 1, 2, tusk); dot(x - 4, top - 4, tusk); dot(x + 4, top - 4, tusk); dot(x, top - 2, 0xc89a3e); },
  },
  lodgerobe: {
    // A dark hooded ceremonial robe with a gold cord and clasp; the deep hood shadows the face
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x3a1418); fill(x - 3, y + 3, 7, 1, 0xc8a44e); dot(x, y + 1, 0xc8a44e); fill(x - 3, y, 1, 6, 0x4a1a20); fill(x + 3, y, 1, 6, 0x2a1014); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x2a1014); },
    head(x, top, look, away) { const hood = 0x3a1418; fill(x - 4, top - 2, 9, 3, hood); fill(x - 4, top - 2, 9, 1, 0x4a1a20); fill(x - 4, top, 1, 4, hood); fill(x + 4, top, 1, 4, hood); fill(x - 2, top + 1, 5, 1, 0x1a0e0c, 0.5); if (away) fill(x - 3, top + 1, 7, 4, hood); },
  },
  lodgemaster: {
    // Black regalia over a white shirt, a gold chain collar of office and a ceremonial sash
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x1a1612); fill(x - 1, y, 3, 6, 0xe0d2b0); fill(x - 2, y, 1, 1, 0xc8a44e); fill(x + 1, y, 1, 1, 0xc8a44e); dot(x, y + 1, 0xc8a44e); fill(x - 2, y + 1, 1, 4, 0x5a1e24); fill(x - 3, y, 1, 6, 0x2a2620); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x141210); },
    head(x, top, look, away) { fill(x - 3, top - 2, 7, 1, 0xc8a44e); dot(x, top - 3, 0xe8c878); },
  },
  lodgeapron: {
    // A dark suit with a white ceremonial apron bearing a small emblem
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x1a1612); fill(x - 2, y + 2, 5, 4, 0xe8e4d8); fill(x - 2, y + 2, 5, 1, 0xf2eee6); dot(x, y + 4, 0xc8a44e); fill(x - 1, y, 3, 2, 0xe0d2b0); fill(x - 3, y, 1, 6, 0x2a2620); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x141210); },
  },
  lodgeguard: {
    // The tyler: a dark hooded cloak, a staff held at the side
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x2a1812); fill(x - 4, y, 1, 6, 0x3a241a); fill(x + 3, y, 1, 6, 0x1a1210); dot(x, y + 1, 0xc8a44e); fill(x + 4, y - 2, 1, 8, 0x5a3a24); dot(x + 4, y - 3, 0xc8a44e); },
    back(x, y) { fill(x - 4, y, 9, 6, 0x24160e); },
    head(x, top, look, away) { const hood = 0x2a1812; fill(x - 4, top - 2, 9, 3, hood); fill(x - 4, top, 1, 4, hood); fill(x + 4, top, 1, 4, hood); fill(x - 2, top + 1, 5, 1, 0x14100c, 0.5); if (away) fill(x - 3, top + 1, 7, 4, hood); },
  },
  prepperhazmat: {
    // A full yellow hazmat suit with a sealed seam and a hooded visor
    front(x, y, o) { fill(x - 3, y, 7, 6, 0xd8c84a); fill(x - 3, y, 7, 1, 0xe8dc6a); fill(x - 3, y + 3, 7, 1, 0xa8982a); dot(x, y + 2, 0x3a3a3a); fill(x - 3, y, 1, 6, 0xe8dc6a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0xc8b83a); fill(x - 3, y, 7, 1, 0xd8c84a); },
    head(x, top, look, away) { const suit = 0xd8c84a; fill(x - 4, top - 2, 9, 5, suit); fill(x - 4, top - 2, 9, 1, 0xe8dc6a); if (!away) { fill(x - 2, top, 5, 2, 0x3a5a4a); fill(x - 2, top, 5, 1, 0x5a8a6a); } else fill(x - 3, top, 7, 2, 0xc8b83a); },
  },
  prepperfatigues: {
    // Olive fatigues under a tactical vest with pouches, and a helmet
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x5a5a3a); dot(x - 2, y + 1, 0x454528); dot(x + 1, y + 2, 0x454528); dot(x, y + 1, 0x6a6a46); fill(x - 3, y + 2, 7, 2, 0x3a3a24); fill(x - 2, y + 2, 2, 2, 0x2a2a1a); fill(x + 1, y + 2, 2, 2, 0x2a2a1a); fill(x - 3, y, 1, 6, 0x6a6a46); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x4a4a2e); dot(x - 1, y + 2, 0x3a3a24); dot(x + 1, y + 3, 0x3a3a24); },
    head(x, top, look, away) { const helm = 0x454528; fill(x - 4, top - 2, 9, 3, helm); fill(x - 4, top - 2, 9, 1, 0x5a5a3a); dot(x + 2, top - 2, 0x3a3a24); },
  },
  prepperbugout: {
    // A field jacket with pack-straps and a bandana at the neck, a big bug-out pack on the back and a cap
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x6a5e40); fill(x - 3, y + 2, 1, 4, 0x4a4228); fill(x + 3, y + 2, 1, 4, 0x4a4228); fill(x - 1, y + 3, 3, 2, 0xa84a3a); dot(x, y + 1, 0x8a7a50); fill(x - 3, y, 1, 6, 0x7a6e50); },
    back(x, y) { fill(x - 4, y - 1, 9, 7, 0x4a4228); fill(x - 4, y - 1, 9, 1, 0x5a5230); dot(x - 2, y + 1, 0x8a7a50); dot(x + 1, y + 2, 0x8a7a50); fill(x - 2, y, 1, 4, 0xa84a3a); },
    head(x, top, look, away) { const cap = 0x4a5a3a; fill(x - 3, top - 2, 7, 2, cap); fill(x - 3, top - 2, 7, 1, 0x5a6a4a); if (!away) fill(x + 3, top, 2, 1, cap); },
  },
  prepperranger: {
    // A plaid flannel under a utility vest with a respirator at the neck, and a boonie hat
    front(x, y, o) { fill(x - 3, y, 7, 6, 0x6a4a3a); for (let i = -2; i <= 2; i += 2) { dot(x + i, y + 1, 0x8a5a44); dot(x + i, y + 3, 0x4a3226); } fill(x - 2, y + 2, 5, 2, 0x4a5a3a); fill(x - 2, y + 2, 5, 1, 0x5a6a4a); dot(x, y, 0x3a3e38); fill(x - 3, y, 1, 6, 0x7a5a4a); },
    back(x, y) { fill(x - 3, y, 7, 6, 0x5a4234); fill(x - 2, y + 2, 5, 2, 0x4a5a3a); },
    head(x, top, look, away) { const hat = 0x4a5a3a; fill(x - 5, top, 11, 1, hat); fill(x - 3, top - 3, 7, 3, hat); fill(x - 3, top - 3, 7, 1, 0x5a6a4a); },
  },
  paradisepresident: {
    // Navy suit over a white shirt, a red tie, and a tiny gold flag pin on the lapel
    front(x, y, o) {
      dot(x - 1, y, 0xf2efe8); dot(x + 1, y, 0xf2efe8);
      fill(x, y, 1, 4, o.tie); dot(x, y + 4, 0x8a1c1c);
      dot(x - 2, y + 1, 0x1a2236); dot(x + 2, y + 1, 0x2e3c5e);
      dot(x - 2, y + 2, 0xc8a050);
    },
    back(x, y, o) { fill(x - 1, y, 1, 5, 0x2e3c5e); },
  },
  paradisesecret: {
    // Black suit and dark tie, a pale coiled earpiece wire down one side; dark sunglasses on the face
    front(x, y, o) {
      dot(x - 1, y, 0x3a3d47); dot(x + 1, y, 0x3a3d47);
      fill(x, y, 1, 4, o.tie);
      dot(x + 4, y - 1, 0xd8d4cc); dot(x + 4, y + 1, 0xc8c4bc); dot(x + 4, y + 3, 0xd8d4cc, 0.8);
    },
    back(x, y, o) { fill(x - 1, y, 1, 5, 0x3a3d47); dot(x + 4, y + 1, 0xd8d4cc); },
    head(x, head, look, away, o) {
      if (away) return;
      fill(x - 3, head + 4, 7, 1, 0x14161b);
      dot(x - 3, head + 3, 0x3a3d47); dot(x + 3, head + 3, 0x3a3d47);
      dot(x + 4, head + 4, 0xd8d4cc);
    },
  },
  paradiseaide: {
    // Shirtsleeves and a loosened tie, a dark lanyard to a little blue ID badge
    front(x, y, o) {
      dot(x - 1, y, 0xffffff); dot(x + 1, y, 0xffffff);
      fill(x, y, 1, 3, o.tie);
      dot(x - 2, y + 1, 0x2a2d35); dot(x - 1, y + 2, 0x2a2d35); dot(x, y + 3, 0x2a2d35);
      fill(x - 1, y + 3, 2, 2, 0xf2efe8); dot(x - 1, y + 3, 0x4c8bf5);
    },
    back(x, y, o) { fill(x - 1, y, 1, 4, 0x9ab8d8); },
  },
  paradisehazmat: {
    // A sealed suit with a front zip and a little green chest dosimeter; a hooded respirator with a clear visor
    front(x, y, o) {
      fill(x, y, 1, 5, 0xc8bd7a);
      fill(x - 2, y + 2, 2, 2, 0x3a3d47); dot(x - 2, y + 2, 0x6fd36f);
    },
    back(x, y, o) { fill(x, y, 1, 5, 0xc8bd7a); fill(x - 3, y, 7, 1, 0xd8ce8a); },
    head(x, head, look, away, o) {
      const suit = 0xe6dc9a, suitDark = 0xc8bd7a, glass = 0x9fd4c8;
      fill(x - 4, head - 1, 9, 2, suit); fill(x - 4, head + 1, 1, 6, suit); fill(x + 4, head + 1, 1, 6, suit);
      if (away) return fill(x - 3, head + 1, 7, 6, suit);
      fill(x - 3, head + 1, 7, 4, glass, 0.6); fill(x - 3, head + 1, 7, 1, 0xd8f0e8);
      fill(x - 2, head + 5, 5, 2, suitDark); dot(x - 1, head + 6, 0x4a4e58); dot(x + 1, head + 6, 0x4a4e58);
    },
  },
  startrekgold: {
    // Command gold: a black shoulder yoke and a gold combadge delta on the chest
    front(x, y, o) { fill(x - 3, y, 7, 1, 0x14161b); dot(x - 3, y + 1, 0x14161b); dot(x + 3, y + 1, 0x14161b); dot(x - 1, y + 1, 0xe8c84a); dot(x - 1, y + 2, 0xf2d87a); },
    back(x, y, o) { fill(x - 3, y, 7, 1, 0x14161b); },
  },
  startrekblue: {
    // Science blue: the same black yoke and gold combadge
    front(x, y, o) { fill(x - 3, y, 7, 1, 0x14161b); dot(x - 3, y + 1, 0x14161b); dot(x + 3, y + 1, 0x14161b); dot(x - 1, y + 1, 0xe8c84a); dot(x - 1, y + 2, 0xf2d87a); },
    back(x, y, o) { fill(x - 3, y, 7, 1, 0x14161b); },
  },
  startrekred: {
    // Operations red: the same black yoke and gold combadge
    front(x, y, o) { fill(x - 3, y, 7, 1, 0x14161b); dot(x - 3, y + 1, 0x14161b); dot(x + 3, y + 1, 0x14161b); dot(x - 1, y + 1, 0xe8c84a); dot(x - 1, y + 2, 0xf2d87a); },
    back(x, y, o) { fill(x - 3, y, 7, 1, 0x14161b); },
  },
  startrekcaptain: {
    // The captain: command gold with a gold collar rim and four rank pips at the collar
    front(x, y, o) { fill(x - 3, y, 7, 1, 0x14161b); fill(x - 2, y, 5, 1, 0xd4a82a); dot(x - 3, y + 1, 0x14161b); dot(x + 3, y + 1, 0x14161b); dot(x - 1, y + 1, 0xe8c84a); dot(x - 1, y + 2, 0xf2d87a); },
    back(x, y, o) { fill(x - 3, y, 7, 1, 0x14161b); fill(x - 2, y, 5, 1, 0xd4a82a); },
  },
  gentlementweedsuit: {
    // A tweed three-piece: a waistcoat with gold buttons over a white shirt, a dark-red tie, flecks of tweed
    front(x, y, o) {
      dot(x - 1, y, 0xeae6dc); dot(x + 1, y, 0xeae6dc); fill(x, y, 1, 4, o.tie);
      fill(x - 2, y + 1, 1, 5, 0x6a5a38); fill(x + 2, y + 1, 1, 5, 0x6a5a38); dot(x, y + 3, 0xc8a050); dot(x, y + 5, 0xc8a050);
      dot(x - 2, y + 3, 0x5a4a28); dot(x + 2, y + 4, 0x5a4a28);
    },
    back(x, y, o) { fill(x - 1, y, 1, 5, 0x6a5a38); },
  },
  gentlemengamekeeper: {
    // A waxed olive jacket with a belt line, and a flat cap
    front(x, y, o) {
      fill(x, y, 1, 5, 0x3a4a2e);
      dot(x - 2, y + 1, 0x3a4a2e); dot(x + 2, y + 1, 0x3a4a2e);
      fill(x - 3, y + 4, 7, 1, 0x5a4a32);
      dot(x - 2, y + 3, 0x2a3a1e);
    },
    back(x, y, o) { fill(x - 3, y + 4, 7, 1, 0x5a4a32); },
    head(x, top, look, away, o) {
      const cap = 0x4a5a3a, capDark = 0x3a4a2e;
      fill(x - 4, top - 1, 9, 2, cap); fill(x - 5, top, 3, 1, cap);
      fill(x - 3, top - 3, 7, 3, cap); fill(x - 3, top - 3, 7, 1, 0x5a6a4a); dot(x - 5, top, capDark);
      if (away) fill(x - 3, top + 1, 7, 5, 0x3a2616);
    },
  },
  gentlemengangster: {
    // A sharp dark suit, white shirt, red tie and a gold lapel pin; slicked-back dark hair
    front(x, y, o) {
      dot(x - 1, y, 0xf2efe8); dot(x + 1, y, 0xf2efe8); fill(x, y, 1, 4, o.tie); dot(x, y + 4, 0x6a0e0e);
      dot(x - 2, y + 1, 0x1f2127); dot(x + 2, y + 1, 0x3a3d47);
      dot(x + 2, y + 2, 0xc8a050);
    },
    back(x, y, o) { fill(x - 1, y, 1, 5, 0x3a3d47); },
    head(x, top, look, away, o) {
      const hair = 0x1a1612;
      fill(x - 3, top, 7, 2, hair); fill(x - 3, top, 1, 4, hair); fill(x + 3, top, 1, 4, hair);
      if (away) fill(x - 3, top + 1, 7, 5, hair);
    },
  },
  gentlementracksuit: {
    // A navy tracksuit with white piping and a gold chain; a cap worn backwards
    front(x, y, o) {
      fill(x, y, 1, 6, 0x1a3a6a);
      dot(x - 3, y + 1, 0xf0f0f0); dot(x + 3, y + 1, 0xf0f0f0);
      fill(x - 3, y + 2, 1, 4, 0xf0f0f0, 0.7); fill(x + 3, y + 2, 1, 4, 0xf0f0f0, 0.7);
      dot(x, y + 2, 0xc8a050); dot(x, y + 3, 0xc8a050);
    },
    back(x, y, o) { fill(x - 3, y + 2, 1, 4, 0xf0f0f0, 0.7); fill(x + 3, y + 2, 1, 4, 0xf0f0f0, 0.7); },
    head(x, top, look, away, o) {
      const cap = 0x1a3a6a;
      fill(x - 3, top - 1, 7, 2, cap); fill(x + 2, top - 1, 4, 1, cap);
      dot(x + 4, top, 0xf0f0f0);
      if (away) { fill(x - 3, top + 1, 7, 5, 0x2a2016); fill(x - 4, top - 1, 9, 2, cap); }
    },
  },
  // The Geode's gem outfits, each drawn from its gem's row (see gdGemOutfit)
  ...Object.fromEntries(Object.keys(GEODE_GEMS).map((gem) => [`geode${gem}`, gdGemOutfit(GEODE_GEMS[gem])])),
  castleking: {
    // The king: a gold crown of points over red velvet, set with jewels; long white hair and a great white beard down his chest;
    // a red robe to the floor with a white ermine collar and trim, a gold sceptre in hand. Now and then he raises the sceptre
    // high, by royal decree, and its jewel flashes.
    look: costumeLook(0xf0c8a4, 0xf2f0ea),
    front(x, y) {
      const ermine = 0xf8f6f0, tail = 0x1e1e22;
      // The ermine collar over the shoulders and down the front, its black tails; the robe lit on the left
      fill(x - 3, y, 7, 1, ermine); fill(x - 1, y + 1, 3, 5, ermine);
      for (const [dx, dy] of [[1, 0], [0, 2], [0, 4]]) dot(x + dx, y + dy, tail);
      fill(x - 3, y + 1, 1, 5, 0xc83a3a); fill(x + 3, y + 1, 1, 5, 0x801c1c);
    },
    back(x, y) { fill(x - 3, y, 7, 1, 0xf8f6f0); dot(x - 1, y, 0x1e1e22); dot(x + 2, y, 0x1e1e22); },
    head(x, top, look, away) {
      const G = CASTLE_GOLD, GL = CASTLE_GOLD_LIGHT, GD = CASTLE_GOLD_DARK, red = 0xb02030, W = look.hair, WD = 0xc8c6be;
      // The crown: a jewelled band, red velvet between its points, a gold ball on top
      fill(x - 3, top, 7, 1, G); dot(x - 3, top, GL); dot(x + 3, top, GD); dot(x - 2, top, red); dot(x, top, 0x3a6ad8); dot(x + 2, top, red);
      fill(x - 2, top - 1, 5, 1, red); for (const dx of [-3, -1, 1, 3]) dot(x + dx, top - 1, G);
      dot(x - 3, top - 2, GL); dot(x, top - 2, G); dot(x + 3, top - 2, GD); dot(x, top - 3, GL);
      // Long white hair at the sides
      fill(x - 3, top + 1, 1, 4, W); fill(x + 3, top + 1, 1, 4, WD); fill(x - 4, top + 2, 1, 3, W);
      if (away) { fill(x - 3, top + 1, 7, 6, W); return; }
      // The great beard, round the mouth and down his chest, a rosy nose over it
      fill(x - 3, top + 5, 7, 2, W); fill(x - 2, top + 7, 5, 2, W); fill(x - 1, top + 9, 3, 1, W); dot(x, top + 10, WD);
      dot(x - 1, top + 5, WD); dot(x + 1, top + 5, WD); dot(x + 2, top + 7, WD); dot(x + 1, top + 9, WD);
      dot(x, top + 5, 0xe8a090);
    },
    // The sceptre in his right hand; now and then raised high, its jewel flashing
    arms(x, top, look, o, sleeve, pose) {
      const p = pose.kind === 'walk' ? null : every(look, 18, 2.2, 3);
      const sceptre = (sx, sy) => {
        fill(sx, sy + 1, 1, 6, CASTLE_GOLD); dot(sx, sy + 1, CASTLE_GOLD_LIGHT); dot(sx, sy, 0xc8283a);
        dot(sx - 1, sy + 1, CASTLE_GOLD_DARK); dot(sx + 1, sy + 1, CASTLE_GOLD_DARK);
      };
      costumeArm(x, top, -4, look, sleeve);
      if (p !== null) {
        // Held a column out from the crown, as the office raises an arm
        sleeve(x + 4, top + 7, 1, 2); sleeve(x + 5, top + 3, 1, 4); dot(x + 5, top + 2, look.skin);
        sceptre(x + 5, top - 5); costumeTwinkle(x + 5, top - 5, 1);
      }
      else { costumeArm(x, top, 4, look, sleeve); sceptre(x + 5, top + 6); }
      return true;
    },
  },
  castlequeen: {
    // The queen: a slim gold circlet of three points, long auburn hair in two braids over her shoulders; a purple gown to the
    // floor with a square neckline, a string of pearls, gold trim and a gold girdle. Now and then she opens a fan and fans herself.
    look: costumeLook(0xf6d6c0, 0x9a3a1a),
    front(x, y, o, look) {
      // The square neckline and the pearls, the gold-trimmed bodice, the girdle hanging down the front; lit on the left
      fill(x - 1, y, 3, 1, look.skin); for (const dx of [-1, 0, 1]) dot(x + dx, y + 1, 0xf4f0f8); dot(x, y + 1, 0xd8d0e8);
      fill(x - 2, y, 1, 4, CASTLE_GOLD); fill(x + 2, y, 1, 4, CASTLE_GOLD_DARK);
      fill(x - 2, y + 4, 5, 1, CASTLE_GOLD); dot(x, y + 5, CASTLE_GOLD_DARK);
      fill(x - 3, y, 1, 6, 0x7a44aa); fill(x + 3, y, 1, 6, 0x3c1a5a);
    },
    back(x, y, o, look) { fill(x - 2, y, 1, 4, look.hair); fill(x + 2, y, 1, 4, look.hair); },
    head(x, top, look, away) {
      const H = look.hair, HL = 0xc85a2a, HD = 0x6a2410;
      // The circlet: a gold band, three slim points, a green jewel in the middle
      fill(x - 3, top, 7, 1, CASTLE_GOLD); dot(x - 3, top, CASTLE_GOLD_LIGHT); dot(x, top, 0x3a9a6a);
      for (const dx of [-2, 0, 2]) dot(x + dx, top - 1, CASTLE_GOLD); dot(x, top - 2, CASTLE_GOLD_LIGHT);
      // The braids, over her shoulders and down her front, their gold ties
      for (const side of [-3, 3]) {
        fill(x + side, top + 1, 1, 9, H);
        for (let r = 2; r < 10; r += 2) dot(x + side, top + r, side < 0 ? HL : HD);
        dot(x + side, top + 10, CASTLE_GOLD);
      }
      if (away) { fill(x - 2, top + 1, 5, 6, H); return; }
      dot(x, top + 6, 0xd84a5a);
      // The fan, open by her face and fanning (see arms)
      if (look.castleFan) {
        const f = Math.floor(now * 8) % 2;
        fill(x + 4, top + 4 + f, 3, 1, 0xd8b8f0); fill(x + 4, top + 5 + f, 2, 1, 0xb890e0); dot(x + 4, top + 6 + f, CASTLE_GOLD);
        dot(x + 6, top + 3 + f, 0xd8b8f0); dot(x + 5, top + 3 + f, 0xe8d0f8);
      }
    },
    // A folded fan in her hand; now and then raised to her face and opened (drawn with the face). The skirt lit on the left
    // and shaded on the right, as the bodice is, standing still
    arms(x, top, look, o, sleeve, pose) {
      if (costumeLift(pose) && pose.kind !== 'walk') { fill(x - 3, top + 13, 1, 3, 0x7a44aa); fill(x + 3, top + 13, 1, 3, 0x3c1a5a); }
      look.castleFan = pose.kind !== 'walk' && every(look, 14, 2.4, 9) !== null;
      costumeArm(x, top, -4, look, sleeve);
      if (look.castleFan) { sleeve(x + 4, top + 7, 1, 3); dot(x + 4, top + 7, look.skin); }
      else { costumeArm(x, top, 4, look, sleeve); fill(x + 5, top + 10, 1, 3, 0xb890e0); dot(x + 5, top + 10, CASTLE_GOLD); }
      return true;
    },
  },
  castlejester: {
    // The jester: a three-pointed cap of yellow and red with a gold bell on each point, a zigzag collar; a tunic and hose in
    // halves of yellow and red, curled shoes; a fool's stick with a little head on it. Now and then he juggles three balls in
    // arcs high over his cap, the bells glinting.
    look: costumeLook(0xf2c8a0, 0x8a5a2a),
    front(x, y) {
      const R = 0xd83a3a, Y = 0xf0c030;
      // The halves, yellow and red; the zigzag collar; a bell at the hem
      fill(x - 3, y, 3, 6, Y); fill(x, y, 4, 6, R);
      for (const dx of [-3, -1, 1, 3]) dot(x + dx, y, dx < 0 ? R : Y);
      for (const dx of [-2, 0, 2]) dot(x + dx, y + 1, dx < 0 ? R : Y);
      dot(x - 2, y + 5, CASTLE_GOLD); dot(x + 1, y + 5, CASTLE_GOLD);
    },
    back(x, y) { fill(x - 3, y, 3, 6, 0xd83a3a); fill(x, y, 4, 6, 0xf0c030); },
    head(x, top, look, away) {
      const R = 0xd83a3a, RD = 0x9a2222, Y = 0xf0c030, YD = 0xb88a1a;
      const bell = (bx, by) => { dot(bx, by, CASTLE_GOLD_LIGHT); if (look.castleJuggle && !away) costumeTwinkle(bx, by, 0.5); };
      // The cap: a band round the brow, three floppy points (left, up and right), a bell on each
      fill(x - 3, top - 1, 3, 2, Y); fill(x, top - 1, 4, 2, R); fill(x - 3, top + 1, 7, 1, YD);
      fill(x - 5, top, 2, 1, Y); dot(x - 6, top + 1, Y); bell(x - 6, top + 2);
      fill(x - 1, top - 3, 2, 2, R); dot(x, top - 4, R); dot(x + 1, top - 5, RD); bell(x + 2, top - 5);
      fill(x + 4, top, 2, 1, R); dot(x + 6, top + 1, RD); bell(x + 6, top + 2);
      if (away) return;
      // A wide toothy grin, its corners a row clear of the eyes
      fill(x - 2, top + 6, 5, 1, 0x8a3a3a); fill(x - 1, top + 6, 3, 1, 0xf6efe2);
      // The three balls, each in an arc from hand to hand high over the cap (see arms)
      if (look.castleJuggle) {
        [0x3a6ad8, 0x3aa84a, 0xe8602a].forEach((colour, i) => {
          const u = (now * 0.9 + i / 3) % 1, k = (u % 0.5) * 2, bx = u < 0.5 ? x - 5 + Math.round(k * 10) : x + 5 - Math.round(k * 10);
          dot(bx, top + 4 - Math.round(Math.sin(Math.PI * k) * 12), colour);
        });
      }
    },
    // Hose in halves and curled shoes; the fool's stick; now and then both hands up, juggling (the balls are drawn with the head)
    arms(x, top, look, o, sleeve, pose) {
      const lift = costumeLift(pose), R = 0xd83a3a;
      if (lift) {
        fill(x, top + 13, 4, 1, R); fill(x + 1, top + 14 - lift[1], 2, 2, R);
        dot(x - 3, top + 16 - lift[0], 0x3a2a6a); dot(x + 3, top + 16 - lift[1], 0x3a2a6a);
      }
      look.castleJuggle = pose.kind !== 'walk' && every(look, 15, 3, 6) !== null;
      if (look.castleJuggle) {
        // The hands out a column clear of his face
        const toss = Math.floor(now * 5) % 2;
        sleeve(x - 4, top + 6, 1, 3); sleeve(x + 4, top + 6, 1, 3); dot(x - 5, top + 5 - toss, look.skin); dot(x + 5, top + 4 + toss, look.skin);
      } else {
        // The fool's stick, its little capped head a row below his grin
        costumeArm(x, top, 4, look, sleeve); costumeArm(x, top, -4, look, sleeve);
        fill(x - 5, top + 9, 1, 4, 0x8a5a2a); dot(x - 5, top + 8, look.skin); dot(x - 6, top + 7, R); dot(x - 4, top + 7, 0xf0c030);
      }
      return true;
    },
  },
  castleherald: {
    // The herald: a flat velvet cap with a long white plume; a tabard quartered in blue and gold over white sleeves; a long
    // brass trumpet hung with a little blue banner. Now and then he raises it and blows a fanfare, notes flying from its bell.
    look: costumeLook(0xe8c0a0, 0x5a3a20),
    front(x, y) {
      const B = 0x2a5aa8, G = 0xe8b830;
      // The tabard in quarters, its edges lit on the left
      fill(x - 3, y, 3, 3, B); fill(x, y, 4, 3, G); fill(x - 3, y + 3, 3, 3, G); fill(x, y + 3, 4, 3, B);
      dot(x - 3, y, 0x4a7ac8); dot(x + 3, y + 3, 0x1a3a7a);
    },
    back(x, y) { fill(x - 3, y, 3, 3, 0xe8b830); fill(x, y, 4, 3, 0x2a5aa8); fill(x - 3, y + 3, 3, 3, 0x2a5aa8); fill(x, y + 3, 4, 3, 0xe8b830); },
    head(x, top, look, away) {
      const cap = 0x1e2a5a, capLight = 0x34428a;
      // The cap, flat and soft, tipped to one side; the plume curling up and back from it
      fill(x - 3, top - 1, 7, 2, cap); fill(x - 4, top, 2, 1, cap); fill(x - 3, top - 1, 3, 1, capLight);
      for (const [dx, dy, c] of [[3, -2, 0xf6f4ee], [4, -3, 0xf6f4ee], [5, -4, 0xe8e6e0], [6, -4, 0xd8d6d0], [4, -2, 0xd8d6d0]]) dot(x + dx, top + dy, c);
      if (away) return;
      // Blowing the fanfare: the trumpet at his lips, a row below his eye, its banner, a cheek puffed out, the notes flying from
      // its bell (see arms)
      const p = look.castleFanfare;
      if (p !== null && p !== undefined) {
        fill(x + 1, top + 6, 7, 1, CASTLE_GOLD); dot(x + 1, top + 6, CASTLE_GOLD_DARK); fill(x + 8, top + 5, 1, 3, CASTLE_GOLD_LIGHT);
        dot(x + 9, top + 4, CASTLE_GOLD); dot(x + 9, top + 8, CASTLE_GOLD);
        fill(x + 4, top + 7, 3, 2, 0x2a5aa8); dot(x + 5, top + 8, CASTLE_GOLD); dot(x + 4, top + 9, 0x2a5aa8);
        dot(x - 4, top + 5, look.skin); dot(x - 4, top + 6, shaded(look.skin, 0.85));
        const k = (p * 3) % 1, nx = x + 10 + Math.round(k * 3), ny = top + 3 - Math.round(k * 4);
        dot(nx, ny, 0xf6f4ee, 1 - k); dot(nx, ny - 1, 0xf6f4ee, 1 - k); dot(nx + 1, ny - 1, 0xf6f4ee, 1 - k);
      }
    },
    // The trumpet held upright at his side, its banner hanging; now and then raised to his lips (drawn with the face)
    arms(x, top, look, o, sleeve, pose) {
      look.castleFanfare = pose.kind === 'walk' ? null : every(look, 16, 2.4, 11);
      costumeArm(x, top, -4, look, sleeve);
      if (look.castleFanfare !== null) { sleeve(x + 4, top + 7, 1, 2); dot(x + 4, top + 7, look.skin); dot(x + 3, top + 7, look.skin); }
      else {
        costumeArm(x, top, 4, look, sleeve);
        fill(x + 5, top + 2, 1, 10, CASTLE_GOLD); fill(x + 4, top + 1, 3, 1, CASTLE_GOLD_DARK); dot(x + 5, top + 1, CASTLE_GOLD_LIGHT);
        fill(x + 6, top + 4, 2, 2, 0x2a5aa8); dot(x + 6, top + 5, CASTLE_GOLD);
      }
      return true;
    },
  },
  castleknight: {
    // The tournament knight: shining plate under a surcoat in halves of red and white with a belt; a helmet with its visor up
    // and a great spray of red and white plumes, a red shield with a white band, and a tall lance with its pennon. Now and then
    // he dips the lance forward in salute.
    look: costumeLook(0xe8c09c, 0x6a4a2a),
    front(x, y) {
      const R = 0xc8302a, W = 0xf2f0ea;
      // The surcoat in halves over the plate, the belt and its buckle; the plate at the sides, a rivet or two
      fill(x - 2, y, 2, 6, R); fill(x, y, 3, 6, W); fill(x - 2, y + 4, 5, 1, 0x6a4424); dot(x, y + 4, CASTLE_GOLD);
      dot(x - 3, y + 1, 0xe8ecf2); dot(x + 3, y + 1, 0x7a808a); dot(x - 3, y + 3, 0xe8ecf2); dot(x + 3, y + 3, 0x7a808a);
    },
    back(x, y) { fill(x - 2, y, 2, 6, 0xf2f0ea); fill(x, y, 3, 6, 0xc8302a); },
    head(x, top, look, away) {
      const S = CASTLE_STEEL, SL = 0xe8ecf2, SD = 0x6e747e;
      // The helmet: a round steel bowl, its visor raised over the brow, cheek plates down the sides
      fill(x - 3, top - 1, 7, 3, S); fill(x - 2, top - 2, 5, 1, S); dot(x - 2, top - 2, SL); fill(x - 3, top - 1, 1, 2, SL); fill(x + 3, top - 1, 1, 3, SD);
      fill(x - 3, top + 1, 7, 1, SD); fill(x - 3, top + 2, 1, 4, S); fill(x + 3, top + 2, 1, 4, SD);
      // The plumes: a great spray from its crown, leaning back, a red one rising between two white
      const R = 0xc8302a, RD = 0xa01c1c, W = 0xf2f0ea, WD = 0xc8c6c0;
      fill(x + 1, top - 7, 2, 1, R); fill(x, top - 6, 3, 1, R); fill(x, top - 5, 2, 1, R); dot(x + 1, top - 4, R); dot(x + 1, top - 3, RD);
      dot(x + 3, top - 6, W); fill(x + 2, top - 5, 2, 1, W); dot(x + 2, top - 4, WD);
      dot(x - 1, top - 5, W); fill(x - 1, top - 4, 2, 1, W); dot(x, top - 3, WD);
      if (away) fill(x - 3, top + 2, 7, 4, S);
    },
    // Greaves; the shield on his left arm; the lance upright in his right hand, now and then dipped forward in salute
    arms(x, top, look, o, sleeve, pose) {
      const lift = costumeLift(pose), wood = 0x8a5a32, steel = CASTLE_STEEL;
      if (lift) { fill(x - 2, top + 14 - lift[0], 2, 2, 0xa8aeb8); fill(x + 1, top + 14 - lift[1], 2, 2, 0x8a909a); }
      fill(x - 6, top + 8, 3, 4, 0xc8302a); dot(x - 5, top + 12, 0xc8302a); for (let i = 0; i < 3; i++) dot(x - 6 + i, top + 9 + i, 0xf2f0ea);
      dot(x - 6, top + 8, 0xe85a4a);
      costumeArm(x, top, 4, look, sleeve, 0x8a909a);
      if (pose.kind !== 'walk' && every(look, 17, 2.2, 7) !== null) {
        line(x + 5, top + 12, x + 12, top - 2, wood); dot(x + 12, top - 3, steel); dot(x + 13, top - 4, steel);
        dot(x + 11, top - 1, 0xc8302a); dot(x + 12, top, 0xf2f0ea);
      } else {
        fill(x + 5, top - 4, 1, 17, wood); fill(x + 5, top - 6, 1, 2, steel); dot(x + 5, top - 7, 0xe8ecf2);
        fill(x + 6, top - 4, 2, 1, 0xc8302a); fill(x + 6, top - 3, 2, 1, 0xf2f0ea); dot(x + 8, top - 4, 0xc8302a);
        fill(x + 4, top + 9, 3, 1, steel);
      }
      return true;
    },
  },
  castlepage: {
    // The page: a blond bowl cut with straight bangs; a short green tunic with puffed shoulders and a belt, yellow hose; a red
    // velvet cushion carried in both hands with a little gold crown on it. Now and then he bows, and the crown wobbles.
    look: costumeLook(0xf6d2b0, 0xe0b040),
    front(x, y) {
      // Puffed shoulders, the belt, the tunic lit on the left
      dot(x - 3, y, 0x5aaa5a); dot(x + 3, y, 0x2a6a32); fill(x - 3, y + 4, 7, 1, 0x6a4424); dot(x, y + 4, CASTLE_GOLD);
      fill(x - 3, y + 1, 1, 3, 0x4a9a52); fill(x + 3, y + 1, 1, 3, 0x2a6a32);
    },
    back(x, y) { fill(x - 3, y + 4, 7, 1, 0x6a4424); },
    head(x, top, look, away) {
      const H = look.hair, HL = 0xf8d870, HD = 0xb08a2a;
      // The bowl cut: round on top, straight bangs over the brow, straight at the sides down to the jaw
      fill(x - 2, top - 1, 5, 1, H); fill(x - 3, top, 7, 3, H); fill(x - 3, top + 3, 1, 2, H); fill(x + 3, top + 3, 1, 2, HD);
      fill(x - 2, top - 1, 2, 1, HL); dot(x - 3, top, HL); fill(x - 2, top + 2, 5, 1, HD);
      if (away) { fill(x - 3, top + 3, 7, 2, H); return; }
      // The crown on the cushion: three points over its band, a row of tunic clear of his chin; it wobbles as he bows (see arms)
      const wob = look.castleBow ? (Math.floor(now * 8) % 2 ? 1 : -1) : 0, cx = x + wob, cy = top + 9;
      fill(cx - 2, cy, 5, 1, CASTLE_GOLD); dot(cx - 2, cy, CASTLE_GOLD_LIGHT); dot(cx + 2, cy, CASTLE_GOLD_DARK);
      dot(cx - 2, cy - 1, CASTLE_GOLD_LIGHT); dot(cx, cy - 1, CASTLE_GOLD); dot(cx + 2, cy - 1, CASTLE_GOLD);
    },
    // The cushion held out in both hands, the tunic's hem; now and then a bow (see nods)
    arms(x, top, look, o, sleeve, pose) {
      if (costumeLift(pose)) fill(x - 3, top + 13, 7, 1, 0x2a6a32);
      look.castleBow = pose.kind !== 'walk' && every(look, 15, 1.6, 8) !== null;
      const cy = top + 10 + (look.castleBow ? 1 : 0);
      sleeve(x - 4, top + 8, 1, 3); sleeve(x + 4, top + 8, 1, 3);
      fill(x - 4, cy, 9, 2, 0xb02030); fill(x - 4, cy, 9, 1, 0xd03a4a); dot(x - 5, cy + 1, CASTLE_GOLD); dot(x + 5, cy + 1, CASTLE_GOLD);
      dot(x - 4, cy + 2, look.skin); dot(x + 4, cy + 2, look.skin);
      return true;
    },
    nods(look) { return look.castleBow ? 1 : 0; },
  },
  castlecook: {
    // The royal cook: a puffed white linen cap over red hair, rosy cheeks; a brown dress to the floor under a big white apron,
    // sleeves rolled up; a great golden pie held out on a platter. Now and then a blackbird pops out of it and sings.
    look: costumeLook(0xf0c0a0, 0xb04a24),
    front(x, y, o, look) {
      // The dress's neckline, the apron's bib below it, shaded on the right
      dot(x, y, look.skin); fill(x - 2, y + 1, 5, 5, 0xf6f4ee); fill(x + 2, y + 1, 1, 5, 0xd8d6d0);
    },
    back(x, y) { fill(x - 2, y + 2, 5, 1, 0xf6f4ee); },
    head(x, top, look, away) {
      const L = 0xf6f4ee, LD = 0xd8d6d0;
      // The cap: puffed white linen gathered round the head into a frill, red hair showing at the sides
      fill(x - 3, top - 2, 7, 1, L); fill(x - 4, top - 1, 9, 2, L); dot(x - 3, top - 2, WHITE);
      for (let dx = -4; dx <= 4; dx++) dot(x + dx, top + 1, dx % 2 ? L : LD);
      fill(x - 3, top + 2, 1, 2, look.hair); fill(x + 3, top + 2, 1, 2, look.hair);
      if (away) { fill(x - 3, top + 2, 7, 3, look.hair); return; }
      dot(x - 2, top + 5, 0xf09080, 0.6); dot(x + 2, top + 5, 0xf09080, 0.6);
      // The blackbird: it pops out of the pie through its crust, flutters up past her face and sits on her cap to sing (see arms)
      const p = look.castleBird;
      if (p !== null && p !== undefined) {
        const B = 0x1a1a22, beak = 0xe89a2a, flap = Math.floor(now * 10) % 2;
        const bird = (bx, by, flying) => { fill(bx, by, 2, 2, B); dot(bx + 2, by, beak); dot(bx + 1, by, WHITE, 0.6); dot(bx - 1, by + (flying ? flap : 1), 0x2a2a34); };
        fill(x, top + 8, 3, 1, 0x5a3418);
        if (p < 0.12) { fill(x + 1, top + 8, 2, 1, B); dot(x + 3, top + 8, beak); }
        else if (p < 0.2) bird(x + 5, top + 3, true);
        else if (p < 0.26) bird(x + 3, top - 3, true);
        else {
          bird(x + 1, top - 4, false);
          const k = ((p - 0.26) * 3) % 1;
          dot(x + 4, top - 5 - Math.round(k * 3), 0xf6f4ee, 1 - k); dot(x + 5, top - 6 - Math.round(k * 3), 0xf6f4ee, 1 - k);
        }
      }
    },
    // The pie held out on its platter in both hands
    arms(x, top, look, o, sleeve, pose) {
      look.castleBird = pose.kind === 'walk' ? null : every(look, 16, 2.8, 4);
      sleeve(x - 4, top + 8, 1, 2); sleeve(x + 4, top + 8, 1, 2); dot(x - 4, top + 10, look.skin); dot(x + 4, top + 10, look.skin);
      fill(x - 5, top + 11, 11, 1, 0x9a9ea8); dot(x - 5, top + 11, 0xc8ccd4);
      fill(x - 3, top + 9, 7, 2, 0xd8962a); fill(x - 2, top + 8, 5, 1, 0xe8a840);
      for (const dx of [-2, 0, 2]) dot(x + dx, top + 9, 0xa86a1a); dot(x - 1, top + 8, 0xf8c868);
      return true;
    },
  },
  castlefalconer: {
    // The falconer: a brown hunting hat with a long striped feather, dark hair; a green hunting tunic with a leather baldric and a
    // pouch, tall boots; a hooded falcon on her raised leather glove. Now and then it spreads its wings and flaps.
    look: costumeLook(0xe8bc98, 0x2a1a12),
    front(x, y) {
      // The baldric across, its buckle, the belt and its pouch; the tunic lit on the left
      for (let r = 0; r < 5; r++) dot(x - 2 + r, y + r, 0x6a4424); dot(x, y + 2, CASTLE_GOLD);
      fill(x - 3, y + 4, 7, 1, 0x5a3a1e); fill(x + 1, y + 5, 2, 1, 0x8a5a32);
      fill(x - 3, y, 1, 4, 0x5a9a5a); fill(x + 3, y, 1, 4, 0x2a5a2e);
    },
    back(x, y) { for (let r = 0; r < 5; r++) dot(x + 2 - r, y + r, 0x6a4424); },
    head(x, top, look, away) {
      const hat = 0x6a4424, hatLight = 0x8a6034, hatDark = 0x4a2e16;
      // The hat: a soft crown and a wide brim, the feather sweeping up and back
      fill(x - 2, top - 2, 5, 2, hat); fill(x - 2, top - 2, 2, 1, hatLight); fill(x - 4, top, 9, 1, hat); dot(x - 4, top, hatLight); dot(x + 4, top, hatDark);
      for (const [dx, dy, c] of [[3, -2, 0xe8dcc0], [4, -3, 0x8a6a3a], [5, -4, 0xe8dcc0], [6, -5, 0x8a6a3a], [7, -6, 0xe8dcc0]]) dot(x + dx, top + dy, c);
      // Dark hair at the sides, down to just above her eyes
      fill(x - 3, top + 1, 1, 3, look.hair); fill(x + 3, top + 1, 1, 3, look.hair);
      if (away) { fill(x - 3, top + 1, 7, 6, look.hair); return; }
      // The falcon on her glove, a column clear of her face, in profile looking out: the leather hood, the pale hooked beak,
      // the pale breast in front and the slate back, its feet on the glove (its tail hangs below it, see arms). Now and then
      // it flaps: its wings up in a V over its back, then spread low in front, clear of her
      const B = 0x5a6474, BD = 0x3a4250;
      fill(x - 7, top + 1, 2, 2, 0x5e3018); dot(x - 7, top + 1, 0x925228); dot(x - 8, top + 2, 0xb8c0cc);
      fill(x - 7, top + 3, 3, 3, B); fill(x - 7, top + 3, 1, 3, 0xeee6d6); fill(x - 5, top + 3, 1, 3, BD);
      fill(x - 6, top + 6, 2, 1, 0xe8c040);
      if (look.castleFlap) {
        if (Math.floor(now * 9) % 2) { dot(x - 6, top, B); dot(x - 7, top, BD); dot(x - 7, top - 1, B); dot(x - 8, top - 2, BD); fill(x - 5, top + 1, 1, 2, BD); }
        else { fill(x - 9, top + 3, 2, 1, B); fill(x - 10, top + 4, 2, 1, BD); dot(x - 8, top + 4, BD); }
      }
    },
    // The arm raised with its thick glove under the falcon's feet, the falcon's tail hanging below it (the falcon is drawn
    // with the face); the other at her side; tall boots
    arms(x, top, look, o, sleeve, pose) {
      const lift = costumeLift(pose);
      if (lift) { fill(x - 2, top + 14 - lift[0], 2, 2, 0x3a2a1a); fill(x + 1, top + 14 - lift[1], 2, 2, 0x3a2a1a); }
      look.castleFlap = pose.kind !== 'walk' && every(look, 14, 1.8, 2) !== null;
      costumeArm(x, top, 4, look, sleeve);
      sleeve(x - 4, top + 7, 1, 3); fill(x - 6, top + 7, 2, 2, 0x8a5a32); dot(x - 6, top + 7, 0xa8743e);
      fill(x - 6, top + 9, 1, 2, 0x3a4250);
      return true;
    },
  },
  castlelady: {
    // The lady-in-waiting: a tall cone hennin in blue with a gold band and a white veil floating from its tip, a high plucked
    // brow; a blue gown to the floor with a gold band under the bust and long hanging sleeves. Now and then she curtsies, her
    // skirt spreading.
    look: costumeLook(0xf6dcc8, 0xc89a5a),
    front(x, y, o, look) {
      // The neckline, the gold band high under the bust, the gown lit on the left
      fill(x - 1, y, 3, 1, look.skin); dot(x, y + 1, 0xf6f4ee); fill(x - 3, y + 2, 7, 1, CASTLE_GOLD); dot(x + 3, y + 2, CASTLE_GOLD_DARK);
      fill(x - 3, y, 1, 2, 0x5a9ad8); fill(x - 3, y + 3, 1, 3, 0x5a9ad8); fill(x + 3, y + 3, 1, 3, 0x1e4a7a);
    },
    head(x, top, look, away) {
      const C = 0x2a5aa8, CL = 0x4a7ac8, veil = 0xf6f8ff;
      // The high plucked brow: no hair showing at the front
      if (!away) fill(x - 2, top + 1, 5, 1, look.skin);
      // The hennin: a tall cone leaning back, lit on the left and shaded down its straight right edge, a gold band at its foot;
      // the veil from its tip floating down behind
      fill(x - 3, top, 7, 1, CASTLE_GOLD); dot(x - 3, top, CASTLE_GOLD_LIGHT);
      [[-2, 5], [-2, 5], [-1, 4], [-1, 4], [0, 3], [0, 3], [1, 2], [1, 2], [2, 1]].forEach(([dx, w], i) => {
        fill(x + dx, top - 1 - i, w, 1, C); dot(x + dx, top - 1 - i, CL); if (w > 1) dot(x + dx + w - 1, top - 1 - i, 0x1e4a7a);
      });
      const sway = Math.round(Math.sin(now * 1.3 + look.phase * 6));
      for (let i = 0; i < 12; i++) {
        const vx = x + 3 + Math.floor(i / 4) + (i > 6 ? sway : 0), vy = top - 9 + i;
        dot(vx, vy, veil, 0.8); if (i > 4) dot(vx + 1, vy, veil, 0.45);
      }
      if (away) { fill(x - 3, top + 1, 7, 5, look.hair); return; }
      dot(x, top + 6, 0xd86a7a);
    },
    // Long hanging sleeves; now and then a curtsy (see nods), the skirt spreading
    arms(x, top, look, o, sleeve, pose) {
      look.castleCurtsy = pose.kind !== 'walk' && every(look, 16, 1.8, 12) !== null;
      if (costumeLift(pose) && look.castleCurtsy) { fill(x - 4, top + 14, 9, 2, 0x2a5aa8); dot(x - 4, top + 14, 0x5a9ad8); }
      for (const side of [-1, 1]) {
        sleeve(x + side * 4, top + 8, 1, 4); dot(x + side * 4, top + 12, look.skin);
        fill(x + side * 5, top + 10, 1, 4, 0x4a7ac8, 0.9); dot(x + side * 5, top + 14, 0x2a5aa8);
      }
      return true;
    },
    nods(look) { return look.castleCurtsy ? 1 : 0; },
  },
  olympuszeus: {
    // Zeus: white curls and a great white beard; a white himation round him and over one shoulder, the chest bare on the other
    // side under a gold border, a gold cord at the waist, gold sandals; a golden thunderbolt in his hand. Now and then he raises
    // it high and it crackles with lightning; by night it lights him up. His whites are shaded storm-grey on the right, his arms
    // bare and tanned, so he holds his own against pale stone and marble.
    look: costumeLook(0xe8c0a0, 0xf2f0ea),
    front(x, y, o, look) {
      // The bare chest at his left shoulder above the gold border, the cloth's folds, the cord; lit on the left, storm-grey in
      // the shade on the right
      dot(x + 2, y, look.skin); dot(x + 3, y, look.skin); dot(x + 3, y + 1, shaded(look.skin, 0.9));
      dot(x + 1, y, 0xe8b830); dot(x + 2, y + 1, 0xe8b830); dot(x + 3, y + 2, 0xa07818);
      dot(x - 1, y + 3, 0xd8d4c8); fill(x + 3, y + 3, 1, 3, 0x9ca0b0); fill(x - 3, y + 4, 7, 1, 0xc8a050);
      fill(x - 3, y, 1, 4, 0xffffff);
    },
    back(x, y) { fill(x + 3, y, 1, 6, 0x9ca0b0); fill(x - 3, y + 4, 7, 1, 0xc8a050); },
    head(x, top, look, away) {
      const W = look.hair, WL = 0xffffff, WD = 0xc8c6be, WS = 0x767c90;
      // The curls, full round the head and down to the jaw, their right edge in storm-grey shade
      fill(x - 3, top - 1, 7, 1, W); fill(x - 4, top, 9, 2, W); fill(x - 4, top + 2, 1, 3, W); fill(x - 3, top + 2, 1, 2, W); fill(x + 3, top + 2, 1, 2, WD);
      for (const [dx, dy] of [[-2, -1], [1, -1], [-3, 0], [0, 0], [3, 1], [-1, 1]]) dot(x + dx, top + dy, WL);
      for (const [dx, dy] of [[2, 0], [-4, 1]]) dot(x + dx, top + dy, WD);
      fill(x + 4, top, 1, 5, WS); dot(x + 3, top - 1, WS);
      if (away) { fill(x - 3, top + 2, 7, 5, W); fill(x + 3, top + 2, 1, 3, WD); fill(x + 3, top + 5, 1, 2, WS); return; }
      // Stern brows and the great beard, curling down onto his chest, shaded down its right edge; light next to the eyes
      dot(x - 2, top + 3, WD); dot(x + 2, top + 3, WD);
      fill(x - 3, top + 5, 7, 2, W); fill(x - 2, top + 7, 5, 2, W); fill(x - 1, top + 9, 3, 1, W);
      for (const [dx, dy] of [[-1, 5], [1, 5], [-2, 7], [0, 9]]) dot(x + dx, top + dy, WD);
      for (const [dx, dy] of [[3, 5], [3, 6], [2, 7], [2, 8], [1, 9]]) dot(x + dx, top + dy, WS);
    },
    // The thunderbolt in his hand, a gold zigzag; now and then raised high over his head, crackling, lighting him by night: his
    // arm up and out past his curls, as the office raises an arm, the bolt's tip in his fist
    arms(x, top, look, o, sleeve, pose) {
      const p = pose.kind === 'walk' ? null : every(look, 15, 2.2, 7), lift = costumeLift(pose);
      // The himation's shade on down to his feet, standing
      if (lift) { dot(x + 3, top + 13, 0x9ca0b0); if (pose.kind !== 'walk') fill(x + 3, top + 14, 1, 2, 0x9ca0b0); }
      // `w` wide from `bx`, `by` down: a stroke down to the left, a jog back across, a stroke down to the left again; pale at
      // the top, gold across, amber at the tip
      const bolt = (bx, by, w) => {
        for (let i = 0; i < w; i++) {
          dot(bx + w - 1 - i, by + i, 0xfff0a0); dot(bx + i, by + w, 0xffd84a); dot(bx + w - 1 - i, by + w + 1 + i, 0xe8a828);
        }
      };
      costumeArm(x, top, -4, look, sleeve);
      if (p !== null) {
        sleeve(x + 4, top + 7, 1, 2); sleeve(x + 5, top + 3, 1, 4); dot(x + 5, top + 2, look.skin);
        bolt(x + 5, top - 7, 4);
        const flick = Math.floor(now * 12) % 2;
        for (const [dx, dy] of flick ? [[5, -6], [-1, -5], [4, -1]] : [[1, -8], [5, -3], [-2, -4]]) dot(x + 5 + dx, top + dy, 0xfff6c0, 0.9);
        if (isNight) glowLate(() => { glowAt(x + 7, top - 3, 8, 8, 0xfff0a0, 0.25); bolt(x + 5, top - 7, 4); }); else glowAt(x + 7, top - 3, 6, 6, 0xfff0a0, 0.12);
      } else {
        costumeArm(x, top, 4, look, sleeve);
        bolt(x + 5, top + 6, 3);
      }
      return true;
    },
  },
  olympushera: {
    // Hera: a tall gold crown, dark hair waved at the sides, gold earrings; a white gown under a peacock-teal mantle with a gold
    // brooch at each shoulder, a peacock feather in her hand. Now and then a peacock's tail fans out behind her, its eyes
    // shimmering.
    look: costumeLook(0xf0d0b8, 0x2e1e14),
    front(x, y) {
      // The white gown down the middle, the mantle either side, its brooches; lit on the left
      fill(x - 1, y, 3, 6, 0xf6f4ee); fill(x + 1, y, 1, 6, 0xdcdad2); dot(x - 2, y, 0xe8b830); dot(x + 2, y, 0xa07818);
      fill(x - 3, y, 1, 6, 0x4aaab4); fill(x + 3, y, 1, 6, 0x1a5a64);
    },
    head(x, top, look, away) {
      const G = 0xe8b830, GL = 0xffe08a, GD = 0xa07818, H = look.hair;
      // Dark hair waved at the sides
      fill(x - 3, top + 1, 1, 4, H); fill(x + 3, top + 1, 1, 4, H); dot(x - 4, top + 2, H); dot(x + 4, top + 2, H);
      // The crown: a tall gold cylinder rising to three points, a lighter band round it, a teal jewel
      fill(x - 2, top - 3, 5, 3, G); fill(x - 2, top - 3, 1, 3, GL); fill(x + 2, top - 3, 1, 3, GD); fill(x - 2, top - 2, 5, 1, GL); dot(x, top - 2, 0x2a8a96);
      dot(x - 2, top - 4, GL); dot(x, top - 4, GL); dot(x + 2, top - 4, G);
      if (away) { fill(x - 2, top + 2, 5, 4, H); fill(x - 1, top + 6, 3, 1, H); return; }
      dot(x - 3, top + 5, G); dot(x + 3, top + 5, G); dot(x, top + 6, 0xc84a5a);
    },
    // The feather in her hand; now and then the peacock's tail fanning out behind her head and shoulders (drawn here, before her
    // head, so she stays in front of it)
    arms(x, top, look, o, sleeve, pose) {
      const p = pose.kind === 'walk' ? null : every(look, 17, 2.6, 5);
      if (p !== null) {
        const r = Math.round(4 + Math.min(1, p * 4) * 3), shimmer = Math.floor(now * 6) % 2;
        for (let i = 0; i <= 8; i++) {
          const a = Math.PI + (i / 8) * Math.PI, fx = x + Math.round(Math.cos(a) * r), fy = top + 4 + Math.round(Math.sin(a) * r);
          line(x, top + 4, fx, fy, 0x3a8a4a, 0.9);
          dot(fx, fy, (i + shimmer) % 2 ? 0x2a6ad8 : 0x2ab0a8); if (r > 5) dot(fx, fy + (fy < top + 4 ? 1 : 0), 0xe8b830, 0.8);
        }
      }
      costumeArm(x, top, -4, look, sleeve); costumeArm(x, top, 4, look, sleeve);
      fill(x + 5, top + 6, 1, 7, 0x3a8a4a); dot(x + 5, top + 5, 0x2a6ad8); dot(x + 5, top + 4, 0x2ab0a8); dot(x + 6, top + 5, 0xe8b830);
      return true;
    },
  },
  olympusposeidon: {
    // Poseidon: wild sea-green hair and a sea-green beard under a gold crown of points; a teal mantle patterned like fish scales
    // over a white tunic; a gold trident. Now and then a wave rises round his feet and spray flies off the trident.
    look: costumeLook(0xd8b090, 0x2a8a7a),
    front(x, y) {
      // The white tunic at the neck and its gold clasp, the scales on the mantle; lit on the left
      fill(x - 1, y, 3, 1, 0xf2f0e8); dot(x - 2, y, 0xe8b830);
      for (const [dx, dy] of [[-2, 2], [0, 2], [2, 2], [-1, 3], [1, 3], [-2, 4], [0, 4], [2, 4], [-1, 5], [1, 5]]) dot(x + dx, y + dy, 0x1a6a6a);
      fill(x - 3, y, 1, 6, 0x4ab4a4); fill(x + 3, y, 1, 6, 0x10484a);
    },
    head(x, top, look, away) {
      const H = look.hair, HL = 0x4ab4a4, HD = 0x1a5a50, G = 0xe8b830;
      // Wild sea-green hair, out past the ears, and the gold crown of points on it
      fill(x - 4, top, 9, 2, H); fill(x - 4, top + 2, 1, 3, H); fill(x + 4, top + 2, 1, 3, HD); fill(x - 3, top + 2, 1, 2, H); fill(x + 3, top + 2, 1, 2, HD);
      for (const [dx, dy] of [[-4, 0], [-1, 1], [2, 0], [4, 1]]) dot(x + dx, top + dy, HL);
      fill(x - 3, top - 1, 7, 1, G); for (const dx of [-3, -1, 1, 3]) dot(x + dx, top - 2, G); dot(x - 1, top - 2, 0xffe08a);
      if (away) { fill(x - 3, top + 2, 7, 5, H); return; }
      // The sea-green beard, curling down onto his chest
      fill(x - 3, top + 5, 7, 2, H); fill(x - 2, top + 7, 5, 2, H); dot(x, top + 9, H);
      for (const [dx, dy] of [[-2, 5], [1, 6], [-1, 7], [2, 7], [0, 8]]) dot(x + dx, top + dy, HL);
      dot(x - 1, top + 5, HD); dot(x + 1, top + 5, HD);
    },
    // The trident upright in his right hand; now and then a wave round his feet and spray off the trident's tip
    arms(x, top, look, o, sleeve, pose) {
      const p = pose.kind === 'walk' ? null : every(look, 16, 2.4, 9), lift = costumeLift(pose), G = 0xe8b830;
      costumeArm(x, top, -4, look, sleeve); costumeArm(x, top, 4, look, sleeve);
      // The trident taller than him, its head clear of his crown: a crossbar and three prongs with a gap between each
      fill(x + 5, top - 3, 1, 19, G); fill(x + 3, top - 4, 5, 1, G); dot(x + 7, top - 4, 0xa07818);
      for (const dx of [3, 7]) fill(x + dx, top - 6, 1, 2, G); fill(x + 5, top - 7, 1, 3, G);
      for (const dx of [3, 5, 7]) dot(x + dx, top - (dx === 5 ? 7 : 6), 0xffe08a);
      if (p !== null) {
        const k = Math.floor(now * 6) % 3, rise = k === 1 ? 1 : 0;
        if (lift) { fill(x - 5, top + 15 - rise, 11, 2, 0x4ab0d8, 0.85); for (const dx of [-5, -2, 1, 4]) dot(x + dx + (k % 2), top + 14 - rise, 0xf2f8ff); }
        for (const [dx, dy] of [[-2, -2], [2, -3], [0, -4], [1, -1]]) dot(x + 5 + dx, top - 7 + dy - k, 0xbfe8ff, 0.85);
      }
      return true;
    },
  },
  olympusathena: {
    // Athena: a bronze helmet pushed back on her head, its face-guard tipped up and a tall blue crest along it, long brown hair
    // under it; a golden scaled aegis over the shoulders of a blue gown; a spear, and a little owl on her shoulder. Now and then
    // the owl hops up flapping, turns its head and blinks.
    look: costumeLook(0xf0d4bc, 0x4a2a1a),
    front(x, y) {
      // The aegis: gold scales over the shoulders and chest, its fringe; the blue gown under it with a gold girdle; lit on the left
      for (let r = 0; r < 2; r++) for (let dx = -3; dx <= 3; dx++) dot(x + dx, y + r, (dx + r) % 2 ? 0xe8b830 : 0xa07818);
      for (const dx of [-2, 0, 2]) dot(x + dx, y + 2, 0xa07818);
      fill(x - 3, y + 2, 1, 4, 0x5a7ac8); fill(x + 3, y + 2, 1, 4, 0x1e3a7a); fill(x - 3, y + 4, 7, 1, 0xe8b830);
    },
    head(x, top, look, away) {
      const B = 0xc8903a, BL = 0xe8b860, BD = 0x7a5420;
      // Long brown hair under the helmet, down the sides
      fill(x - 3, top + 1, 1, 6, look.hair); fill(x + 3, top + 1, 1, 6, look.hair);
      // The helmet pushed back: a bronze bowl over the brow, the tipped-up face-guard with its eye holes (from the front only),
      // the crest along the top
      fill(x - 3, top - 2, 7, 3, B); fill(x - 3, top - 2, 1, 3, BL); fill(x + 3, top - 2, 1, 3, BD);
      fill(x - 1, top - 6, 2, 4, 0x2a5aa8); dot(x - 1, top - 6, 0x6a9ae8); dot(x + 1, top - 5, 0x2a5aa8); dot(x + 1, top - 4, 0x1a3a7a); dot(x - 2, top - 4, 0x6a9ae8);
      if (away) { fill(x - 3, top + 1, 7, 6, look.hair); return; }
      dot(x - 2, top, 0x3a2410); dot(x + 2, top, 0x3a2410);
      dot(x, top + 6, 0xc86a6a);
      // The owl on her shoulder: grey-brown, lit on the left, ear tufts, a pale face and breast, two big yellow eyes; now and
      // then it hops up off her shoulder flapping its wings, turns its head and blinks (see arms)
      const turn = look.costumeOwl, hop = turn !== null && turn !== undefined && turn < 0.5;
      const ox = x - 5, oy = top + 3 - (hop ? 1 : 0), owl = 0xb09c80, owlLight = 0xd0c0a4, tuft = 0x5e4e3e, face = 0xf4ecdc, eye = 0xffd23a;
      fill(ox - 1, oy, 3, 4, owl); fill(ox - 1, oy + 2, 1, 2, owlLight); dot(ox - 1, oy, tuft); dot(ox + 1, oy, tuft);
      if (hop) { const wy = oy + 1 + (Math.floor(now * 8) % 2); dot(ox - 2, wy, owlLight); dot(ox + 2, wy, owl); }
      if (turn !== null && turn !== undefined && turn < 0.6) { dot(ox - 1, oy + 1, eye); dot(ox, oy + 1, face); }
      else if (turn !== null && turn !== undefined && turn < 0.75) { dot(ox - 1, oy + 1, tuft); dot(ox, oy + 1, face); dot(ox + 1, oy + 1, tuft); }
      else { dot(ox - 1, oy + 1, eye); dot(ox, oy + 1, face); dot(ox + 1, oy + 1, eye); }
      dot(ox, oy + 2, 0xe8a030); dot(ox, oy + 3, face); dot(ox - 1, oy + 4, 0xe8a030); dot(ox + 1, oy + 4, 0xe8a030);
    },
    // The spear in her right hand; the owl's moment (drawn with the head)
    arms(x, top, look, o, sleeve, pose) {
      look.costumeOwl = pose.kind === 'walk' ? null : every(look, 13, 2, 4);
      costumeArm(x, top, -4, look, sleeve); costumeArm(x, top, 4, look, sleeve);
      fill(x + 5, top - 2, 1, 16, 0x8a5a32); fill(x + 5, top - 4, 1, 2, 0xc8903a); dot(x + 5, top - 5, 0xe8b860);
      return true;
    },
  },
  olympusapollo: {
    // Apollo: short golden curls; a white chiton to the knee with a gold belt and a gold border at the hem, a red cloak pinned at
    // one shoulder, gold sandals; a golden lyre. Now and then he strums it and notes float up.
    look: costumeLook(0xf0c8a0, 0xf0c040),
    front(x, y) {
      // The red cloak at his shoulders and its gold pin, the gold belt; the chiton lit on the left
      dot(x - 3, y, 0xc8302a); dot(x + 3, y, 0x8a1c1c); fill(x + 3, y + 1, 1, 3, 0x8a1c1c); dot(x + 2, y, 0xe8b830);
      fill(x - 3, y + 3, 7, 1, 0xe8b830); fill(x - 3, y + 1, 1, 2, 0xffffff);
    },
    head(x, top, look, away) {
      const H = look.hair, HL = 0xffe080, HD = 0xb88a1a;
      // Short golden curls all over, lit on top
      fill(x - 3, top - 1, 7, 1, H); fill(x - 4, top, 9, 2, H); dot(x - 4, top + 2, H); dot(x + 4, top + 2, HD); dot(x - 3, top + 2, H); dot(x + 3, top + 2, HD);
      for (const [dx, dy] of [[-2, -1], [1, -1], [-3, 0], [0, 0], [3, 1], [-1, 1]]) dot(x + dx, top + dy, HL);
      for (const [dx, dy] of [[2, 0], [-4, 1], [4, 1]]) dot(x + dx, top + dy, HD);
      if (away) { fill(x - 3, top + 2, 7, 3, H); return; }
      // Strumming: amber-gold notes floating up off the lyre (see arms), to read on pale stone and dark wood alike
      const p = look.costumeLyre;
      if (p !== null && p !== undefined) for (let i = 0; i < 2; i++) {
        const k = (p * 2 + i * 0.5) % 1, nx = x + 7 + i * 2, ny = top + 4 - Math.round(k * 8);
        dot(nx, ny, 0xd8921c, 1 - k); dot(nx, ny - 1, 0xd8921c, 1 - k); dot(nx + 1, ny - 1, 0xd8921c, 1 - k);
      }
    },
    // The lyre held at his side, now and then raised and strummed; sitting at a desk, held up at his shoulder, clear of the
    // desk; the chiton's gold hem when the legs show
    arms(x, top, look, o, sleeve, pose) {
      const lift = costumeLift(pose), G = 0xe8b830, GD = 0xa07818;
      if (lift) { fill(x - 3, top + 13, 7, 1, 0xf6f2e8); for (const dx of [-3, -1, 1, 3]) dot(x + dx, top + 13, G); }
      const p = pose.kind === 'walk' ? null : every(look, 14, 2.6, 6);
      look.costumeLyre = p;
      // The lyre: two arms curving up out of a round sound box and out at their tips, a crossbar under the tips, two strings
      const lyre = (lx, ly) => {
        dot(lx - 1, ly - 1, G); dot(lx + 4, ly - 1, GD); fill(lx, ly, 1, 4, G); fill(lx + 3, ly, 1, 4, GD);
        fill(lx, ly + 1, 4, 1, G); fill(lx + 1, ly + 4, 2, 1, GD); dot(lx + 1, ly + 4, G);
        fill(lx + 1, ly + 2, 1, 2, 0xf8f6f0, 0.85); fill(lx + 2, ly + 2, 1, 2, 0xf8f6f0, 0.55);
      };
      costumeArm(x, top, -4, look, sleeve);
      if (p !== null) { sleeve(x + 4, top + 7, 1, 3); lyre(x + 5, top + 5); dot(x + 6 + (Math.floor(now * 8) % 2), top + 7, look.skin); }
      else if (!lift) { lyre(x + 4, top + 5); dot(x + 4, top + 9, look.skin); }
      else { costumeArm(x, top, 4, look, sleeve); lyre(x + 5, top + 9); }
      return true;
    },
  },
  olympusartemis: {
    // Artemis: auburn hair tied up high, a silver crescent moon on her brow; a short pale chiton belted at the waist, hunting
    // sandals, a quiver of arrows over her shoulder; a silver bow. Now and then she draws the bow, an arrow on it, and looses it;
    // by night her crescent shines.
    look: costumeLook(0xf2d0b4, 0x9a3a1a),
    front(x, y) {
      // The quiver's strap over one shoulder, the belt and its silver clasp; the chiton lit on the left
      dot(x + 2, y, 0x6a4424); dot(x + 1, y + 1, 0x6a4424); dot(x, y + 2, 0x6a4424); fill(x - 3, y + 3, 7, 1, 0x6a4424); dot(x, y + 3, 0xc8ccd4);
      fill(x - 3, y, 1, 3, 0xffffff);
    },
    back(x, y) { fill(x + 1, y - 1, 2, 5, 0x8a5a32); for (const dx of [1, 2]) dot(x + dx, y - 2, 0xf2f0ea); },
    head(x, top, look, away) {
      const H = look.hair, HL = 0xc85a2a, HD = 0x6a2410, silver = 0xe8ecf4;
      // The hair tied up high in a knot on a short tail
      fill(x - 1, top - 3, 3, 2, H); dot(x - 1, top - 3, HL); dot(x + 1, top - 2, HD); dot(x, top - 1, H);
      fill(x - 3, top + 1, 1, 3, H); fill(x + 3, top + 1, 1, 3, H);
      if (away) { fill(x - 3, top + 2, 7, 4, H); return; }
      // The arrows' fletching peeking over her shoulder (from behind, the quiver shows its own)
      dot(x + 4, top + 6, 0xf2f0ea); dot(x + 5, top + 6, 0xc83a3a); dot(x + 4, top + 7, 0x8a5a32);
      // The silver crescent on her brow, shining through the night: a soft light on the brow alone
      const moon = () => { dot(x - 1, top + 1, silver); dot(x + 1, top + 1, silver); dot(x, top + 2, silver); };
      if (isNight) glowLate(() => { glowAt(x, top + 1, 2, 1, 0xe0e8ff, 0.3); moon(); }); else moon();
    },
    // The bow in her left hand, its silver a step darker than pale stone; now and then drawn, an arrow on the string, then
    // loosed with a streak
    arms(x, top, look, o, sleeve, pose) {
      const p = pose.kind === 'walk' ? null : every(look, 15, 2.2, 8), S = 0xc0c6d2, SD = 0x767a86;
      costumeArm(x, top, 4, look, sleeve);
      if (p === null) {
        // At rest, upright at her side: the limbs bowed out from tip to tip, the string straight between them
        costumeArm(x, top, -4, look, sleeve);
        dot(x - 5, top + 6, SD); dot(x - 6, top + 7, S); fill(x - 7, top + 8, 1, 5, S); dot(x - 6, top + 13, S); dot(x - 5, top + 14, SD);
        fill(x - 5, top + 7, 1, 7, 0xf2f2f2, 0.6);
        return true;
      }
      // Drawn: the bow out at arm's length, the string back to her chest; or loosed, a streak where the arrow went
      sleeve(x - 4, top + 8, 1, 1); sleeve(x - 6, top + 8, 2, 1); dot(x - 7, top + 8, look.skin);
      fill(x - 8, top + 4, 1, 9, S); dot(x - 7, top + 3, SD); dot(x - 7, top + 13, SD);
      if (p < 0.7) {
        line(x - 7, top + 4, x - 2, top + 8, 0xf2f2f2, 0.7); line(x - 7, top + 12, x - 2, top + 8, 0xf2f2f2, 0.7);
        fill(x - 10, top + 8, 8, 1, 0x8a5a32); dot(x - 11, top + 8, 0xc8ccd4); dot(x - 2, top + 7, 0xc83a3a);
      } else {
        line(x - 8, top + 4, x - 8, top + 12, 0xf2f2f2, 0.7);
        const k = (p - 0.7) / 0.3; for (let i = 0; i < 3; i++) dot(x - 11 - Math.round(k * 6) - i * 2, top + 8, 0xe8ecf4, 0.8 - i * 0.25);
      }
      return true;
    },
  },
  olympushermes: {
    // Hermes: a round brimmed traveller's hat with a little white wing either side, brown curls; a short white tunic with a red
    // cloak at the shoulders and a satchel of letters at the hip; winged sandals; the caduceus. Now and then a letter flies out
    // of the satchel and zooms off, delivered.
    look: costumeLook(0xe8bc94, 0x5a3418),
    front(x, y) {
      // The red cloak round his shoulders and its clasp, the satchel's strap across; lit on the left
      dot(x - 3, y, 0xc8302a); dot(x - 2, y, 0xc8302a); dot(x + 2, y, 0x9a2020); dot(x + 3, y, 0x9a2020); dot(x, y, 0xe8b830);
      dot(x - 1, y + 1, 0x6a4424); dot(x, y + 2, 0x6a4424); dot(x + 1, y + 3, 0x6a4424);
      fill(x - 3, y + 1, 1, 3, 0xffffff);
    },
    head(x, top, look, away) {
      const hat = 0xb08a5a, wing = 0xf6f6f2, wingDark = 0xc8ccd4, flap = look.costumeLetter !== null && look.costumeLetter !== undefined && Math.floor(now * 10) % 2;
      // The hat: a low round crown and a wide brim, a little white wing out to each side, beating while a letter goes
      fill(x - 2, top - 2, 5, 2, hat); fill(x - 2, top - 2, 2, 1, 0xd0aa78); fill(x - 4, top, 9, 1, hat); dot(x - 4, top, 0xd0aa78); dot(x + 4, top, 0x7a5a34);
      const up = flap ? 1 : 0;
      dot(x - 5, top - 1 - up, wing); dot(x - 6, top - 2 - up, wing); dot(x - 5, top - 2 - up, wingDark);
      dot(x + 5, top - 1 - up, wing); dot(x + 6, top - 2 - up, wingDark); dot(x + 5, top - 2 - up, wing);
      fill(x - 3, top + 1, 1, 2, look.hair); fill(x + 3, top + 1, 1, 2, look.hair);
      if (away) fill(x - 3, top + 1, 7, 4, look.hair);
    },
    // The caduceus in his left hand, the satchel at his right hip, the sandals' wings; now and then a letter zooms off
    arms(x, top, look, o, sleeve, pose) {
      const lift = costumeLift(pose), G = 0xe8b830;
      // The sandals' little wings, out from each heel, clear of the tunic
      if (lift) for (const [side, l] of [[-1, lift[0]], [1, lift[1]]]) { dot(x + side * 3, top + 16 - l, 0xd8dce4); dot(x + side * 4, top + 15 - l, 0xf6f6f2); }
      const p = pose.kind === 'walk' ? null : every(look, 14, 1.6, 3);
      look.costumeLetter = p;
      costumeArm(x, top, -4, look, sleeve); costumeArm(x, top, 4, look, sleeve);
      // The caduceus: a gold staff, two green snakes winding up it, little wings at its top
      fill(x - 5, top + 3, 1, 11, G); dot(x - 6, top + 3, 0xf6f6f2); dot(x - 4, top + 3, 0xf6f6f2); dot(x - 5, top + 2, 0xffe08a);
      for (let i = 0; i < 4; i++) dot(x - 6 + (i % 2) * 2, top + 5 + i * 2, 0x3a9a4a);
      // The satchel, a letter in it, or the letter flying off
      fill(x + 2, top + 11, 3, 2, 0x8a5a32); dot(x + 2, top + 11, 0xa8743e);
      if (p === null) dot(x + 3, top + 10, 0xf6f4ee);
      else {
        const k = Math.min(1, p * 1.2), lx = x + 4 + Math.round(k * 12), ly = top + 9 - Math.round(k * 5);
        fill(lx, ly, 3, 2, 0xf6f4ee); dot(lx + 1, ly, 0xc8302a);
        for (let i = 1; i <= 3; i++) dot(lx - i * 2, ly + 1, 0xd8dce4, 0.8 - i * 0.2);
      }
      return true;
    },
  },
  olympusares: {
    // Ares: a bronze helmet closing over his face but for the T of its opening, a tall red crest arching over it; a bronze cuirass
    // shaped to the chest, a red cloak, a short red skirt, bronze greaves; a round bronze shield and a spear. Now and then he
    // strikes the shield with a war cry, and it rings.
    look: costumeLook(0xd8a888, 0x2a1a12),
    front(x, y) {
      const BL = 0xe8b860, BD = 0x7a5420;
      // The cuirass lit on the left, the chest and belly modelled in it; the red cloak at the shoulders; the skirt's strips
      fill(x - 2, y, 1, 4, BL); dot(x - 1, y + 1, BD); dot(x + 1, y + 1, BD); dot(x, y + 2, BD); dot(x - 1, y + 3, BD); dot(x + 1, y + 3, BD); fill(x + 2, y, 1, 4, BD);
      dot(x - 3, y, 0xc8302a); dot(x + 3, y, 0x8a1c1c); fill(x - 3, y + 4, 7, 2, 0xb02a2a); for (const dx of [-2, 0, 2]) dot(x + dx, y + 5, 0x7a1414);
    },
    back(x, y) { fill(x - 3, y, 7, 6, 0xb02a2a); fill(x + 3, y, 1, 6, 0x7a1414); },
    head(x, top, look, away) {
      const B = 0xc8903a, BL = 0xe8b860, BD = 0x7a5420, crest = 0xc8302a;
      // The helmet: a bronze bowl closing over the face, the T of its opening showing his eyes and mouth
      fill(x - 3, top - 1, 7, 8, B); fill(x - 3, top - 1, 1, 8, BL); fill(x + 3, top - 1, 1, 8, BD); fill(x - 2, top - 2, 5, 1, B);
      if (!away) {
        fill(x - 2, top + 4, 5, 1, look.skin); fill(x, top + 5, 1, 2, look.skin); dot(x - 2, top + 4, PALETTE.eye); dot(x + 2, top + 4, PALETTE.eye); dot(x, top + 3, BD);
        if (look.costumeCry) dot(x, top + 6, 0x3a1010);
      }
      // The crest: tall red horsehair arching over the top from front to back
      fill(x - 1, top - 6, 3, 4, crest); fill(x - 2, top - 5, 1, 3, crest); fill(x + 2, top - 5, 1, 3, 0x8a1c1c); dot(x - 1, top - 6, 0xe85a4a); dot(x, top - 7, crest);
    },
    // Greaves on the shins; the round shield on his left arm, ringing when struck; the spear in his right hand, raised to strike
    arms(x, top, look, o, sleeve, pose) {
      const lift = costumeLift(pose), B = 0xc8903a, BL = 0xe8b860, BD = 0x7a5420;
      if (lift) { fill(x - 2, top + 14 - lift[0], 2, 2, B); fill(x + 1, top + 14 - lift[1], 2, 2, BD); }
      const p = pose.kind === 'walk' ? null : every(look, 15, 1.8, 2), hit = p !== null && Math.floor(p * 6) % 2 === 0;
      look.costumeCry = p !== null;
      costumeArm(x, top, -4, look, sleeve);
      const sx = x - 6 + (hit ? 1 : 0);
      disc(sx, top + 10, 3, B); disc(sx - 1, top + 9, 1.5, BL); ring(sx, top + 10, 3.2, BD, 0.9); dot(sx, top + 10, BD);
      if (hit) costumeTwinkle(sx - 2, top + 8, 0.9);
      costumeArm(x, top, 4, look, sleeve);
      const up = p !== null ? 3 : 0;
      fill(x + 5, top - 3 - up, 1, 16, 0x8a5a32); fill(x + 5, top - 5 - up, 1, 2, 0xc8ccd4); dot(x + 5, top - 6 - up, 0xe8ecf2);
      return true;
    },
  },
  olympusaphrodite: {
    // Aphrodite: long golden waves down to her waist with a pink rose in them, red lips; a rose-pink chiton with a gold girdle and a
    // pearly scallop-shell clasp, a golden apple in her hand. Now and then little hearts float up from her.
    look: costumeLook(0xf6dcc8, 0xf2c860),
    front(x, y) {
      // The shell clasp at her shoulder, the gold girdle under the bust, the chiton's folds; lit on the left
      dot(x + 2, y, 0xf6eee6); dot(x + 3, y, 0xe8d8cc); dot(x + 2, y + 1, 0xe8d8cc);
      fill(x - 2, y + 2, 5, 1, 0xe8b830); dot(x, y + 3, 0xe8b830);
      for (const dx of [-1, 1]) fill(x + dx, y + 3, 1, 3, 0xe07888);
      fill(x - 3, y, 1, 6, 0xf8b8c4);
    },
    back(x, y, o, look) { fill(x - 3, y, 7, 5, look.hair); for (const dx of [-2, 0, 2]) dot(x + dx, y + 2, 0xc8a040); },
    head(x, top, look, away) {
      const H = look.hair, HL = 0xfff0a8, HD = 0xc8a040;
      // The long golden waves: over the head and down both sides past the shoulders, a rose tucked in them
      fill(x - 3, top - 1, 7, 1, H); fill(x - 4, top, 9, 2, H);
      for (const side of [-1, 1]) { fill(x + side * 4, top + 2, 1, 10, H); fill(x + side * 3, top + 2, 1, 4, H); }
      for (const [dx, dy] of [[-2, -1], [1, 0], [-4, 4], [-4, 7], [-4, 10], [4, 3], [4, 6], [4, 9]]) dot(x + dx, top + dy, dx < 0 ? HL : HD);
      dot(x - 3, top, 0xe8405a); dot(x - 4, top, 0xc82a4a); dot(x - 3, top + 1, 0x4a8a3a);
      if (away) { fill(x - 3, top + 2, 7, 5, H); return; }
      fill(x - 1, top + 6, 3, 1, 0xd8283c); dot(x - 2, top + 5, 0xf0a0a8, 0.5); dot(x + 2, top + 5, 0xf0a0a8, 0.5);
      // The hearts floating up from her (see arms)
      const p = look.costumeHearts;
      if (p !== null && p !== undefined) for (let i = 0; i < 3; i++) {
        const k = (p * 1.5 + i / 3) % 1, hx = x - 3 + i * 3 + Math.round(Math.sin(k * 6 + i)), hy = top - 1 - Math.round(k * 8), a = 1 - k;
        dot(hx - 1, hy - 1, 0xf0507a, a); dot(hx + 1, hy - 1, 0xf0507a, a); fill(hx - 1, hy, 3, 1, 0xf0507a, a); dot(hx, hy + 1, 0xf0507a, a);
      }
    },
    // The golden apple in her right hand; the hearts' moment (drawn with the head)
    arms(x, top, look, o, sleeve, pose) {
      look.costumeHearts = pose.kind === 'walk' ? null : every(look, 13, 2.4, 6);
      costumeArm(x, top, -4, look, sleeve); costumeArm(x, top, 4, look, sleeve);
      // The apple held out past her hair, so it doesn't melt into it: lit on the top left, a leaf at its stalk
      fill(x + 5, top + 11, 2, 2, 0xe8b830); dot(x + 5, top + 11, 0xffe08a); dot(x + 6, top + 12, 0xa07818); dot(x + 6, top + 10, 0x4a8a3a);
      return true;
    },
  },
  zoolion: {
    // The lion onesie: a golden suit with a cream belly, its hood ringed by a big shaggy mane falling over the shoulders, two little
    // round ears on top. Now and then it roars: the mane bristles out and the mouth opens wide.
    look: zooTucked,
    front(x, y) {
      // The cream belly, the zip, the suit lit on the left
      fill(x - 1, y + 2, 3, 4, 0xf2e2b8); dot(x, y + 2, 0xb88a2a);
      fill(x - 3, y + 2, 1, 4, 0xf0c060); fill(x + 3, y + 2, 1, 4, 0xa87820);
    },
    back(x, y) { fill(x - 3, y, 7, 2, 0xb8642a); dot(x + 1, y + 5, 0xb88a2a); },
    head(x, top, look, away) {
      const M = 0xb8642a, ML = 0xd8843a, MD = 0x8a4418, F = 0xe0aa48, FD = 0xb88a2a, roar = look.costumeRoar ? 1 : 0;
      // The mane: a shaggy ring round the hood and under the chin, out over the shoulders; it bristles out further as it roars
      const L = x - 5 - roar, R = x + 5 + roar, T = top - 3 - roar, B = top + 8 + roar;
      fill(L, T + 1, x - 3 - L, B - T - 1, M); fill(x + 4, T + 1, R - x - 3, B - T - 1, M);
      fill(x - 3, T, 7, top - 1 - T, M); fill(x - 3, top + 7, 7, B - top - 6, M);
      for (let yy = T + 1; yy < B; yy += 2) { dot(L - 1, yy, M); dot(R + 1, yy + 1, MD); }
      for (let xx = x - 3; xx <= x + 3; xx += 2) dot(xx, T - 1, ML);
      fill(L, T + 1, 1, B - T - 1, ML); fill(R, T + 1, 1, B - T - 1, MD);
      // The hood's golden fur inside it, round the face; two little round ears on top of the mane
      fill(x - 3, top - 1, 7, 3, F); fill(x - 3, top + 2, 1, 5, F); fill(x + 3, top + 2, 1, 5, FD); dot(x - 3, top - 1, 0xf0c060);
      fill(x - 4, T - 2, 2, 2, F); fill(x + 3, T - 2, 2, 2, FD); dot(x - 3, T - 1, 0x7a3a1a); dot(x + 3, T - 1, 0x7a3a1a);
      if (away) { fill(x - 3, top + 2, 7, 5, M); return; }
      // Roaring: the mouth wide open
      if (roar) { fill(x - 1, top + 6, 3, 1, 0x5a1a1a); dot(x, top + 5, 0x5a1a1a); }
    },
    // Paws for hands; the roar's moment (drawn with the head)
    arms(x, top, look, o, sleeve, pose) {
      look.costumeRoar = pose.kind !== 'walk' && every(look, 14, 1.4, 5) !== null;
      costumeArm(x, top, -4, look, sleeve, 0xb88a2a); costumeArm(x, top, 4, look, sleeve, 0xb88a2a);
      return true;
    },
  },
  zooelephant: {
    // The elephant onesie: a grey suit, its hood with two great round ears pink inside and a trunk hanging down between the eyes,
    // a little white tusk either side of it. Now and then the ears flap and the trunk swings up with a toot.
    look: zooTucked,
    front(x, y) {
      // The paler belly, the suit lit on the left
      fill(x - 1, y + 1, 3, 4, 0xb4b8c0); fill(x - 3, y, 1, 6, 0xb4b8c0); fill(x + 3, y, 1, 6, 0x6e727c);
    },
    back(x, y) { dot(x, y + 5, 0x6e727c); dot(x, y + 6, 0x4a4e58); },
    head(x, top, look, away) {
      const G = 0x9a9ea8, GL = 0xb8bcc6, GD = 0x6e727c, pink = 0xe8a8b4, toot = look.costumeToot;
      costumeHood(x, top, G, GL, GD, away);
      // The ears: great round flaps either side, pink inside, flapping as it toots
      const flap = toot !== null && toot !== undefined && Math.floor(now * 6) % 2, ew = flap ? 2 : 3;
      fill(x - 4 - ew, top - 1, ew, 7, GL); dot(x - 4 - ew, top - 1, G);
      fill(x + 5, top - 1, ew, 7, GD); dot(x + 4 + ew, top - 1, G);
      // From behind, the backs of the ears: grey, the pink only inside
      if (away) return;
      fill(x - 3 - ew, top, ew - 1, 5, pink); fill(x + 5, top, ew - 1, 5, pink);
      // The trunk down between the eyes, or swung up with a toot: over the brow in its darker grey, so it shows on the hood, and
      // out past it, its tip turned up; the toot puffs off the tip. The tusks.
      if (toot !== null && toot !== undefined) {
        dot(x, top + 2, G); dot(x, top + 1, GD); dot(x + 1, top, GD); dot(x + 2, top - 1, GD); dot(x + 3, top - 2, GD);
        dot(x + 4, top - 3, G); dot(x + 4, top - 4, GL);
        const k = (toot * 3) % 1, rise = Math.round(k * 2);
        dot(x + 5 + rise, top - 4 - rise, WHITE, 0.8 * (1 - k)); dot(x + 6 + rise, top - 3 - rise, WHITE, 0.6 * (1 - k));
      } else { fill(x, top + 2, 1, 4, G); dot(x, top + 6, GD); dot(x + 1, top + 6, GD); }
      dot(x - 1, top + 5, 0xf6f4ee); dot(x + 1, top + 5, 0xf6f4ee);
    },
    // Grey paws; the toot's moment (drawn with the head)
    arms(x, top, look, o, sleeve, pose) {
      look.costumeToot = pose.kind === 'walk' ? null : every(look, 14, 1.8, 4);
      costumeArm(x, top, -4, look, sleeve, 0x6e727c); costumeArm(x, top, 4, look, sleeve, 0x6e727c);
      return true;
    },
  },
  zoogiraffe: {
    // The giraffe onesie: a yellow suit with brown patches, its hood rising tall with patches of its own, two horns tipped in brown
    // and little ears out to the sides. Now and then the ears flick and a long dark tongue curls out.
    look: zooTucked,
    front(x, y) {
      // The patches, the suit lit on the left
      for (const [dx, dy] of [[-2, 0], [1, 1], [-1, 3], [2, 3], [-3, 4], [0, 5]]) fill(x + dx, y + dy, 2, 1, 0xa86a2a);
      fill(x - 3, y, 1, 4, 0xf8d878); fill(x + 3, y, 1, 3, 0xc8962a);
    },
    back(x, y) { for (const [dx, dy] of [[-2, 1], [1, 2], [-1, 4], [2, 5]]) fill(x + dx, y + dy, 2, 1, 0xa86a2a); },
    head(x, top, look, away) {
      const Y = 0xf0c050, YL = 0xf8d878, YD = 0xc8962a, spot = 0xa86a2a, flick = look.costumeFlick ? 1 : 0;
      costumeHood(x, top, Y, YL, YD, away);
      // Rising tall over the head, patched
      fill(x - 2, top - 4, 5, 2, Y); dot(x - 2, top - 4, YL); dot(x + 2, top - 3, YD);
      for (const [dx, dy] of [[-1, -3], [2, -1], [-3, 0], [1, 1], [-4, 3], [3, 4]]) dot(x + dx, top + dy, spot);
      // The horns, tipped in brown; the ears out to the sides, flicking now and then
      fill(x - 2, top - 6, 1, 2, YD); dot(x - 2, top - 7, 0x5a3418); fill(x + 2, top - 6, 1, 2, YD); dot(x + 2, top - 7, 0x5a3418);
      dot(x - 5, top - 2 - flick, Y); dot(x - 6, top - 2 - flick, YL); dot(x + 5, top - 2 - flick, Y); dot(x + 6, top - 2 - flick, YD);
      if (!away && flick) { dot(x, top + 6, 0x4a2a6a); dot(x + 1, top + 7, 0x4a2a6a); dot(x + 2, top + 7, 0x6a4a8a); }
    },
    // Hooves for hands; the flick's moment (drawn with the head)
    arms(x, top, look, o, sleeve, pose) {
      look.costumeFlick = pose.kind !== 'walk' && every(look, 12, 1.4, 7) !== null;
      costumeArm(x, top, -4, look, sleeve, 0x5a3418); costumeArm(x, top, 4, look, sleeve, 0x5a3418);
      return true;
    },
  },
  zoozebra: {
    // The zebra onesie: a white suit striped in black, its hood striped too, with a black-and-white mane standing up over the top
    // and pointed ears tipped black. Now and then the ears flick and the mane bristles.
    look: zooTucked,
    front(x, y) {
      // The stripes, curving round the body
      for (const r of [0, 2, 4]) { fill(x - 3, y + r, 2, 1, 0x1a1a1e); fill(x + 2, y + r + 1, 2, 1, 0x1a1a1e); }
      dot(x - 1, y + 1, 0x1a1a1e); dot(x + 1, y + 3, 0x1a1a1e); dot(x - 1, y + 5, 0x1a1a1e);
    },
    back(x, y) {
      // Its stripes curving round the back as on the front, not in straight bands like a referee's shirt
      for (const r of [0, 2, 4]) { fill(x + 2, y + r, 2, 1, 0x1a1a1e); fill(x - 3, y + r + 1, 2, 1, 0x1a1a1e); }
      dot(x + 1, y + 1, 0x1a1a1e); dot(x - 1, y + 3, 0x1a1a1e); dot(x + 1, y + 5, 0x1a1a1e);
    },
    head(x, top, look, away) {
      const W = 0xf4f4f0, WD = 0xc8c8c4, K = 0x1a1a1e, up = look.costumeFlick ? 1 : 0;
      costumeHood(x, top, W, 0xffffff, WD, away);
      // The hood's stripes, down either side
      for (const [dx, dy] of [[-4, 0], [-4, 2], [-4, 4], [-3, -1], [4, 1], [4, 3], [4, 5], [3, -1], [-2, -2], [2, -2]]) dot(x + dx, top + dy, K);
      // The mane standing up over the top, striped, bristling now and then; the pointed ears tipped black
      for (let i = 0; i < 4; i++) dot(x, top - 3 - i - up, i % 2 ? W : K);
      dot(x - 1, top - 3 - up, K); dot(x + 1, top - 4 - up, K);
      dot(x - 3, top - 3, W); dot(x - 3, top - 4 + up, K); dot(x + 3, top - 3, WD); dot(x + 3, top - 4 + up, K);
      if (!away) dot(x, top + 6, 0x8a8a90);
    },
    // Black hooves for hands; the flick's moment (drawn with the head)
    arms(x, top, look, o, sleeve, pose) {
      look.costumeFlick = pose.kind !== 'walk' && every(look, 12, 1.4, 2) !== null;
      costumeArm(x, top, -4, look, sleeve, 0x1a1a1e); costumeArm(x, top, 4, look, sleeve, 0x1a1a1e);
      return true;
    },
  },
  zoopenguin: {
    // The penguin onesie: black with a big white belly and orange feet; its hood black with a white face round the opening and an
    // orange beak over the brow; flippers for arms. Now and then it flaps them.
    look: zooTucked,
    front(x, y) {
      // The white belly, the black sides
      fill(x - 2, y, 5, 6, 0xf6f6f2); dot(x - 2, y, 0x1e1e26); dot(x + 2, y, 0x1e1e26); dot(x + 2, y + 1, 0xdcdcd8); fill(x + 2, y + 2, 1, 4, 0xdcdcd8);
      fill(x - 3, y, 1, 6, 0x3a3a44);
    },
    head(x, top, look, away) {
      const K = 0x1e1e26;
      costumeHood(x, top, K, 0x3a3a44, 0x0e0e12, away);
      if (away) return;
      // The white face round the opening, the orange beak over the brow
      fill(x - 3, top + 2, 1, 4, 0xf6f6f2); fill(x + 3, top + 2, 1, 4, 0xdcdcd8);
      fill(x - 1, top + 1, 3, 1, 0xf08a20); dot(x, top + 2, 0xd8701a);
    },
    // Flippers for arms, no hands; flapping now and then
    arms(x, top, look, o, sleeve, pose) {
      const flap = pose.kind !== 'walk' && every(look, 12, 1.6, 3) !== null && Math.floor(now * 8) % 2;
      for (const side of [-1, 1]) {
        if (flap) { fill(x + side * 4, top + 8, 1, 2, 0x1e1e26); fill(x + side * 5, top + 7, 1, 3, 0x2a2a34); }
        else fill(x + side * 4, top + 8, 1, 5, side < 0 ? 0x2a2a34 : 0x14141a);
      }
      return true;
    },
  },
  zoopanda: {
    // The panda onesie: white with black arms and legs and a black band over the shoulders; its hood white with round black ears
    // and a panda's face on it, black patches round its eyes and a black nose. Now and then it munches a stalk of bamboo.
    look: zooTucked,
    front(x, y) {
      // The black band over the shoulders, the white belly lit on the left
      fill(x - 3, y, 7, 2, 0x1e1e22); fill(x - 3, y + 2, 1, 4, 0xffffff); fill(x + 3, y + 2, 1, 4, 0xd8d8d4);
    },
    back(x, y) { fill(x - 3, y, 7, 2, 0x1e1e22); dot(x, y + 5, 0xf6f6f2); },
    head(x, top, look, away) {
      const W = 0xf6f6f2, K = 0x1e1e22;
      costumeHood(x, top, W, 0xffffff, 0xd0d0cc, away);
      // The round black ears on top
      fill(x - 4, top - 4, 2, 2, K); fill(x + 3, top - 4, 2, 2, K); dot(x - 4, top - 4, 0x3a3a40);
      if (away) return;
      // The panda's face on the hood over the brow: black patches round its eyes, their glints, a black nose
      fill(x - 3, top - 1, 2, 2, K); fill(x + 2, top - 1, 2, 2, K); dot(x - 2, top - 1, WHITE); dot(x + 2, top - 1, WHITE); dot(x, top + 1, K);
      // Munching bamboo (see arms)
      if (look.costumeMunch) {
        const chew = Math.floor(now * 6) % 2;
        fill(x + 1, top + 4 + chew, 1, 5, 0x5aa83a); dot(x + 1, top + 3 + chew, 0x8ad86a); dot(x + 2, top + 5 + chew, 0x3a8a2a); dot(x + 1, top + 6 + chew, 0x2e7a24);
      }
    },
    // Black paws; now and then one comes up with a stalk of bamboo (the stalk is drawn with the head)
    arms(x, top, look, o, sleeve, pose) {
      look.costumeMunch = pose.kind !== 'walk' && every(look, 15, 2.6, 8) !== null;
      costumeArm(x, top, -4, look, sleeve, 0x1e1e22);
      if (look.costumeMunch) { sleeve(x + 4, top + 7, 1, 3); dot(x + 3, top + 7, 0x1e1e22); dot(x + 2, top + 8, 0x1e1e22); }
      else costumeArm(x, top, 4, look, sleeve, 0x1e1e22);
      return true;
    },
  },
  zoomonkey: {
    // The monkey onesie: brown with a tan belly, its hood with big round ears out at the sides, tan inside, and a tan face round the
    // opening; a banana in hand. Now and then it peels the banana and takes a bite.
    look: zooTucked,
    front(x, y) {
      // The tan belly, the suit lit on the left
      fill(x - 1, y + 1, 3, 5, 0xd8b088); fill(x - 3, y, 1, 6, 0x9a6a42); fill(x + 3, y, 1, 6, 0x5a3420);
    },
    // Its tail curled up its back, in the darker brown so it shows on the suit
    back(x, y) { for (const [dx, dy] of [[2, 5], [3, 4], [3, 3], [2, 2]]) dot(x + dx, y + dy, 0x5a3420); },
    head(x, top, look, away) {
      const B = 0x7a4a2a, BL = 0x9a6a42, BD = 0x5a3420, T = 0xd8b088;
      costumeHood(x, top, B, BL, BD, away);
      // Big round ears out at the sides, tan inside
      fill(x - 7, top + 1, 3, 3, B); fill(x - 6, top + 2, 2, 1, T); dot(x - 7, top + 1, BL);
      fill(x + 5, top + 1, 3, 3, BD); fill(x + 5, top + 2, 2, 1, T);
      if (away) return;
      // The tan face round the opening
      fill(x - 2, top + 1, 5, 1, T); dot(x - 3, top + 2, T); dot(x + 3, top + 2, T); dot(x - 3, top + 6, T); dot(x + 3, top + 6, T);
      // The banana, up at the mouth, peeled and bitten (see arms)
      const p = look.costumeBanana;
      if (p !== null && p !== undefined) {
        fill(x + 1, top + 5, 1, 4, 0xf2d84a); dot(x + 2, top + 8, 0xc8a82a);
        if (p > 0.3) { dot(x, top + 5, 0xf2d84a); dot(x + 2, top + 5, 0xe8c83a); dot(x, top + 6, 0xe8c83a); }
        if (p > 0.6) dot(x + 1, top + 4, 0xfaf2d0);
      }
    },
    // Brown paws, a banana in one; now and then the banana goes up to its mouth (drawn with the head)
    arms(x, top, look, o, sleeve, pose) {
      const p = pose.kind === 'walk' ? null : every(look, 14, 2.4, 3);
      look.costumeBanana = p;
      costumeArm(x, top, -4, look, sleeve, 0x5a3420);
      if (p !== null) { sleeve(x + 4, top + 7, 1, 3); dot(x + 3, top + 8, 0x5a3420); }
      else { costumeArm(x, top, 4, look, sleeve, 0x5a3420); dot(x + 5, top + 11, 0xf2d84a); dot(x + 5, top + 12, 0xf2d84a); dot(x + 6, top + 10, 0xe8c83a); }
      return true;
    },
  },
  zooflamingo: {
    // The flamingo onesie: pink all over, darker at the wing edges; its hood with a long neck curving up out of it and the
    // flamingo's head on top, its bent beak black at the tip. Now and then the neck bends down to preen.
    look: zooTucked,
    front(x, y) {
      // Feathered rows, the wings' darker edges, lit on the left
      for (const dx of [-2, 0, 2]) dot(x + dx, y + 2, 0xe0708e); for (const dx of [-1, 1]) dot(x + dx, y + 4, 0xe0708e);
      fill(x - 3, y, 1, 6, 0xf8b0c4); fill(x + 3, y, 1, 6, 0xc85a7a);
    },
    head(x, top, look, away) {
      const P = 0xf28aa8, PL = 0xf8b0c4, PD = 0xc85a7a, dip = look.costumePreen ? 1 : 0;
      costumeHood(x, top, P, PL, PD, away);
      // The neck rising out of the hood in a curve, the head on top with its eye and bent beak, all of it within the outfit
      // picker's picture; bent down to preen now and then
      const neck = dip ? [[0, -3], [1, -4], [2, -4], [3, -3], [4, -2]] : [[0, -3], [1, -4], [1, -5]];
      for (const [dx, dy] of neck) dot(x + dx, top + dy, P);
      const [hx, hy] = dip ? [x + 5, top - 2] : [x - 1, top - 7];
      fill(hx - 1, hy, 3, 2, P); dot(hx - 1, hy, PL); dot(hx + (dip ? 0 : -1), hy, 0x1a1a1a);
      if (dip) { dot(hx + 2, hy + 1, 0xf6f2ea); dot(hx + 2, hy + 2, 0xf6f2ea); dot(hx + 1, hy + 3, 0x1a1a1a); }
      else { dot(hx - 2, hy + 1, 0xf6f2ea); dot(hx - 2, hy + 2, 0xf6f2ea); dot(hx - 1, hy + 3, 0x1a1a1a); }
    },
    // Pink wings for arms, their edges darker; the preen's moment (drawn with the head)
    arms(x, top, look, o, sleeve, pose) {
      look.costumePreen = pose.kind !== 'walk' && every(look, 13, 2, 9) !== null;
      costumeArm(x, top, -4, look, sleeve, 0xc85a7a); costumeArm(x, top, 4, look, sleeve, 0xc85a7a);
      dot(x - 4, top + 11, 0xc85a7a); dot(x + 4, top + 11, 0xc85a7a);
      return true;
    },
  },
  zoocroc: {
    // The crocodile onesie: green with a pale yellow belly in bands; its hood is the crocodile's open jaws, the face looking out
    // between rows of white teeth, the bulging yellow eyes on top, a ridged tail behind. Now and then the jaws snap a little.
    look: zooTucked,
    front(x, y) {
      // The banded belly, the suit lit on the left
      fill(x - 2, y + 1, 5, 5, 0xe8e0a0); for (const r of [2, 4]) fill(x - 2, y + r, 5, 1, 0xc8c080);
      fill(x - 3, y, 1, 6, 0x6aaa4a); fill(x + 3, y, 1, 6, 0x2e5a22);
    },
    back(x, y) { for (const dx of [-2, 0, 2]) dot(x + dx, y, 0x2e5a22); for (const r of [2, 4]) dot(x, y + r, 0x2e5a22); },
    head(x, top, look, away) {
      const G = 0x4a8a3a, GD = 0x2e5a22, tooth = 0xf6f6f0, snap = look.costumeSnap ? 1 : 0;
      costumeHood(x, top, G, 0x6aaa4a, GD, away);
      // The bulging yellow eyes on top, the bumps along the head
      fill(x - 3, top - 3, 2, 1, G); fill(x + 2, top - 3, 2, 1, G); dot(x - 3, top - 3, 0xf0d840); dot(x + 3, top - 3, 0xf0d840);
      dot(x - 2, top - 3, 0x1a1a1a); dot(x + 2, top - 3, 0x1a1a1a); for (const dx of [-1, 1]) dot(x + dx, top - 2, GD);
      if (away) return;
      // The teeth: a row along the upper jaw over the brow, another along the lower jaw at the chin; snapping a little now and then
      for (const dx of [-2, 0, 2]) dot(x + dx, top + 2 + snap, tooth);
      fill(x - 3, top + 7, 7, 1, G); for (const dx of [-1, 1]) dot(x + dx, top + 6, tooth);
    },
    // Green claws; the snap's moment (drawn with the head)
    arms(x, top, look, o, sleeve, pose) {
      look.costumeSnap = pose.kind !== 'walk' && every(look, 11, 1.2, 6) !== null && Math.floor(now * 5) % 2 === 0;
      costumeArm(x, top, -4, look, sleeve, 0x2e5a22); costumeArm(x, top, 4, look, sleeve, 0x2e5a22);
      return true;
    },
  },
  /*
   * The ant colony (an original team, no brand): nine ants of one nest, each in an ant onesie over the wearer's own face, with
   * the work that ant does. Their hood, antennae, compound eyes and mandibles, and their second pair of legs, are drawn by
   * costumeAntHead and costumeAntLegs, just after this table.
   */
  antcolonyqueen: {
    // The queen: a deep red-brown ant hood with a little gold crown between its antennae; a long gown banded like a queen ant's
    // great abdomen, a gold sash across it. Now and then she gives a royal wave, and the crown glints.
    front(x, y) {
      // The narrow waist and the banded abdomen of the gown below it, lit on the left; the gold sash from her shoulder down
      fill(x - 3, y + 3, 7, 1, 0x3a1008); fill(x - 3, y + 4, 7, 1, 0x8a3420); fill(x - 3, y + 5, 7, 1, 0x6a2414);
      fill(x - 3, y, 1, 3, 0xa04a30); dot(x - 2, y + 4, 0xb8604a);
      for (let r = 1; r < 4; r++) fill(x - 4 + r, y + r, 2, 1, 0xe8b830);
      dot(x - 3, y + 1, 0xffe08a); dot(x, y + 3, 0xb88a20);
    },
    back(x, y) { fill(x - 3, y + 3, 7, 1, 0x3a1008); fill(x - 3, y + 5, 7, 1, 0x6a2414); },
    head(x, top, look, away) {
      costumeAntHead(x, top, look, 0x7a2a1a, 0xa04a30, 0x4a1408, away);
      // The little gold crown between the antennae, a ruby in it
      fill(x - 1, top - 3, 3, 1, 0xe8b830); dot(x - 1, top - 4, 0xffe08a); dot(x + 1, top - 4, 0xe8b830); dot(x, top - 3, 0xc8283a);
      if (look.costumeWave) costumeTwinkle(x + 1, top - 4, 0.8);
    },
    // Now and then a royal wave: the arm raised out past the hood's edge, the hand waggling by her cheek; the second pair of legs
    arms(x, top, look, o, sleeve, pose) {
      look.costumeWave = pose.kind !== 'walk' && every(look, 14, 2.2, 5) !== null;
      costumeArm(x, top, -4, look, sleeve, 0x4a1408);
      if (look.costumeWave) { sleeve(x + 4, top + 7, 1, 2); sleeve(x + 5, top + 4, 1, 3); dot(x + 5 + Math.floor(now * 4) % 2, top + 3, 0x4a1408); }
      else costumeArm(x, top, 4, look, sleeve, 0x4a1408);
      costumeAntLegs(x, top, pose, 0x4a1408);
      return true;
    },
  },
  antcolonyworker: {
    // The worker: a black ant hood; a black suit pinched in at the waist; a big sugar cube carried in both arms. Now and then it
    // hoists the cube high over its head, proud of its strength, a bead of sweat on its brow.
    front(x, y) { fill(x - 3, y + 3, 7, 1, 0x0a0a0e); dot(x - 2, y + 4, 0x4a4a58); fill(x - 3, y, 1, 3, 0x3a3a46); },
    back(x, y) { fill(x - 3, y + 3, 7, 1, 0x0a0a0e); },
    head(x, top, look, away) {
      costumeAntHead(x, top, look, 0x1e1e26, 0x3a3a46, 0x0e0e12, away);
      if (!away && look.costumeHoist) dot(x - 2, top + 3, 0x8ac8f0, 0.85);   // a bead of sweat
    },
    // The cube in both arms, or held high over the head, between the antennae; the second pair of legs
    arms(x, top, look, o, sleeve, pose) {
      const p = pose.kind === 'walk' ? null : every(look, 15, 2.4, 3), hand = 0x0e0e12;
      look.costumeHoist = p !== null && p > 0.15;
      // A sugar cube: its lit top face, its front, its shaded side, a grain or two
      const cube = (cx, cy) => {
        fill(cx, cy, 5, 4, 0xeeeeea); fill(cx, cy, 5, 1, WHITE); fill(cx + 4, cy + 1, 1, 3, 0xc4c8d2);
        dot(cx + 1, cy + 2, 0xd4d4ce); dot(cx + 3, cy + 3, 0xd4d4ce);
      };
      if (look.costumeHoist) {
        // Both arms up round the hood, elbows out, the hands under the cube's corners
        for (const side of [-1, 1]) {
          for (const [dx, dy] of [[4, 7], [4, 8], [5, 4], [5, 5], [5, 6], [6, 0], [6, 1], [6, 2], [6, 3], [5, -2], [5, -1]]) {
            sleeve(x + side * dx, top + dy, 1, 1);
          }
          dot(x + side * 4, top - 3, hand); dot(x + side * 3, top - 4, hand);
        }
        cube(x - 2, top - 7);
        costumeTwinkle(x - 1, top - 6, 0.7);
      } else {
        sleeve(x - 4, top + 8, 1, 2); sleeve(x + 4, top + 8, 1, 2);
        cube(x - 2, top + 8);
        dot(x - 3, top + 10, hand); dot(x + 3, top + 10, hand);
      }
      costumeAntLegs(x, top, pose, hand);
      return true;
    },
  },
  antcolonysoldier: {
    // The soldier: a dark red ant hood with great curved amber mandibles round the chin and a stern brow; a dark red suit with a
    // heavy armoured thorax. Now and then it snaps the mandibles shut and open, on guard.
    front(x, y) { fill(x - 3, y + 3, 7, 1, 0x2a0808); fill(x - 3, y, 7, 1, 0x6a1810); fill(x - 3, y + 1, 1, 2, 0xa83a2a); dot(x - 2, y + 4, 0xc04a3a); },
    back(x, y) { fill(x - 3, y + 3, 7, 1, 0x2a0808); },
    head(x, top, look, away) {
      costumeAntHead(x, top, look, 0x8a2018, 0xb83a2a, 0x3a0806, away, { big: true, shut: !!look.costumeSnap });
      if (!away) { dot(x - 2, top + 3, 0x5a1410); dot(x + 2, top + 3, 0x5a1410); }   // a stern brow
    },
    // Fists at its sides; now and then the snap (drawn with the head); the second pair of legs
    arms(x, top, look, o, sleeve, pose) {
      look.costumeSnap = pose.kind !== 'walk' && every(look, 11, 1.2, 4) !== null && Math.floor(now * 5) % 2 === 0;
      costumeArm(x, top, -4, look, sleeve, 0x3a0806); costumeArm(x, top, 4, look, sleeve, 0x3a0806);
      costumeAntLegs(x, top, pose, 0x3a0806);
      return true;
    },
  },
  antcolonyleafcutter: {
    // The leafcutter: a rust-red ant hood; a rust suit; a big green leaf held up over its head like a parasol on its stalk, a
    // tiny ant riding on it. Now and then the leaf sways in the breeze and the rider waves.
    front(x, y) { fill(x - 3, y + 3, 7, 1, 0x4a1e0a); fill(x - 3, y, 1, 3, 0xc06a3a); dot(x - 2, y + 4, 0xc06a3a); },
    back(x, y) { fill(x - 3, y + 3, 7, 1, 0x4a1e0a); },
    head(x, top, look, away) {
      // The leaf held up over the head, tip up to the left, lit above its midrib, the rider on its edge: from the front it's
      // behind the head and its antennae, its stalk down to the hand (see arms); from behind, over the hood
      const sway = look.costumeSway ? (Math.floor(now * 3) % 2 ? 1 : -1) : 0, lx = x - 5 + sway, ly = top - 6;
      const leaf = () => {
        costumeAntSprite(['LLLL........', 'LmmGLLL.....', '.GGmmmGLL...', '..dGGGmmGL..', '....ddGGmmL.', '.......ddGms'], lx, ly,
          { L: 0x6ab84a, G: 0x4a9a3a, m: 0x9ad870, d: 0x2e6a24, s: 0x5a8a3a });
        dot(lx + 5, ly, 0x5a2410); dot(lx + 6, ly, 0x5a2410); if (look.costumeSway) dot(lx + 6 + (Math.floor(now * 4) % 2), ly - 1, 0x5a2410);
      };
      if (away) { costumeAntHead(x, top, look, 0xa04a20, 0xc06a3a, 0x5a2410, true); leaf(); return; }
      leaf(); fill(x + 6, top, 1, 3, 0x5a8a3a);
      costumeAntHead(x, top, look, 0xa04a20, 0xc06a3a, 0x5a2410, false);
    },
    // The arm up beside the hood holding the leaf's stalk; the other at its side; the second pair of legs
    arms(x, top, look, o, sleeve, pose) {
      look.costumeSway = pose.kind !== 'walk' && every(look, 13, 2.4, 9) !== null;
      costumeArm(x, top, -4, look, sleeve, 0x5a2410);
      sleeve(x + 4, top + 7, 1, 2); sleeve(x + 5, top + 4, 1, 3); dot(x + 6, top + 3, 0x5a2410);
      costumeAntLegs(x, top, pose, 0x5a2410);
      return true;
    },
  },
  antcolonynurse: {
    // The nurse: a ginger ant hood; a ginger suit, a lavender sling from her shoulder, a pale larva cradled in it. Now and then
    // the larva wriggles and she rocks it.
    front(x, y) {
      // The top of the sling's strap, over her shoulder (the rest with the pouch, see arms); the waist; lit on the left
      dot(x + 3, y, 0xb89ae0); dot(x + 2, y, 0x9a7ac8);
      fill(x - 3, y + 4, 7, 1, 0x6a3a10); fill(x - 3, y, 1, 3, 0xe09a4a);
    },
    back(x, y) { for (let r = 0; r < 4; r++) dot(x - 3 + r, y + r, 0xb89ae0); },
    head(x, top, look, away) { costumeAntHead(x, top, look, 0xc8742a, 0xe09a4a, 0x6a3a10, away, { twitch: look.costumeRock ? Math.floor(now * 4) % 2 : 0 }); },
    // The larva in the sling, her arms round it, wriggling and rocked now and then; the second pair of legs
    arms(x, top, look, o, sleeve, pose) {
      const p = pose.kind === 'walk' ? null : every(look, 12, 2.4, 7);
      look.costumeRock = p !== null;
      const rock = p !== null ? (Math.floor(now * 3) % 2 ? 1 : 0) : 0, wr = p !== null ? Math.floor(now * 8) % 2 : 0, lx = x - 2 + rock;
      sleeve(x - 4, top + 8, 1, 3); sleeve(x + 4, top + 8, 1, 3);
      // The larva: a pale grub, plump and segmented, its little amber head at one end, its tail lifting as it wriggles
      fill(lx, top + 8, 4, 2, 0xf4ecd8); dot(lx, top + 8, 0xfffaf0); dot(lx + 1, top + 9, 0xd8ccb0); dot(lx + 3, top + 9, 0xd8ccb0);
      dot(lx + 3, top + 8, 0xe8b07a); dot(lx - 1, top + 9 - wr, 0xf4ecd8);
      // The sling's pouch under it, round at the bottom, the strap up to her shoulder; her hands at its corners
      fill(x - 2, top + 10, 5, 1, 0xb89ae0); fill(x - 1, top + 11, 3, 1, 0x9a7ac8); dot(x - 2, top + 10, 0xd0b8f0);
      fill(x + 3, top + 8, 1, 2, 0xb89ae0);
      dot(x - 3, top + 10, 0x6a3a10); dot(x + 3, top + 10, 0x6a3a10);
      costumeAntLegs(x, top, pose, 0x6a3a10);
      return true;
    },
  },
  antcolonydrone: {
    // The drone: a slim black-brown ant hood with extra-big compound eyes; two pairs of clear wings out from its shoulders, veined,
    // a dark spot on each forewing. Now and then the wings buzz.
    front(x, y) { fill(x - 3, y + 3, 7, 1, 0x0e0a08); fill(x - 3, y, 1, 3, 0x4a3e36); },
    back(x, y) { for (const dx of [-3, 2]) { fill(x + dx, y, 2, 6, 0xdceaf4, 0.5); dot(x + dx, y + 2, 0x8a9aa8, 0.6); } },
    head(x, top, look, away) { costumeAntHead(x, top, look, 0x2a2420, 0x4a3e36, 0x0e0c0a, away, { eyes: 3, twitch: look.costumeBuzz ? Math.floor(now * 6) % 2 : 0 }); },
    // The wings, out either side, buzzing now and then (a blur between two beats); the second pair of legs
    arms(x, top, look, o, sleeve, pose) {
      look.costumeBuzz = pose.kind !== 'walk' && every(look, 12, 1.8, 11) !== null;
      const beat = look.costumeBuzz ? Math.floor(now * 14) % 2 : 0, a = look.costumeBuzz ? 0.7 : 1;
      const wing = { w: [0xdceaf4, 0.55 * a], e: [0xeef6fc, 0.75 * a], v: [0x8a9aa8, 0.8 * a], s: [0x3a3440, 0.85 * a], g: [WHITE, 0.85 * a] };
      for (const side of [-1, 1]) {
        // The forewing up and out from the shoulder, its dark spot near the tip; the hind wing smaller, below
        costumeAntSprite(['...ve', '..vsw', '.vwgw', 'vwwwe', 'vwwe.', '.we..'], x + side * 5, top + 1 - beat, wing, 1, side < 0);
        costumeAntSprite(['vwwe', '.wwe', '..e.'], x + side * 5, top + 7 + beat, wing, 1, side < 0);
      }
      costumeArm(x, top, -4, look, sleeve, 0x0e0c0a); costumeArm(x, top, 4, look, sleeve, 0x0e0c0a);
      costumeAntLegs(x, top, pose, 0x0e0c0a);
      return true;
    },
  },
  antcolonyarchitect: {
    // The architect: a brown ant hood, round glasses, a pencil tucked behind one antenna; a brown suit with a pen in its pocket;
    // a rolled blueprint. Now and then it unrolls the blueprint, the colony's tunnels drawn on it, and studies it.
    front(x, y) { fill(x - 3, y + 3, 7, 1, 0x2a1a0c); dot(x + 2, y + 1, 0x3a6ad8); dot(x + 2, y + 2, 0x3a6ad8); fill(x - 3, y, 1, 3, 0x8a6a42); },
    back(x, y) { fill(x - 3, y + 3, 7, 1, 0x2a1a0c); },
    head(x, top, look, away) {
      costumeAntHead(x, top, look, 0x6a4a2a, 0x8a6a42, 0x3a2614, away);
      if (away) return;
      // The round glasses: a pale rim round each eye, lit at the top, the eye clear inside it; the pencil behind the antenna
      for (const ex of [x - 2, x + 2]) {
        dot(ex, top + 3, 0xe4f0f6); dot(ex - 1, top + 4, 0xd0e2ec); dot(ex + 1, top + 4, 0xd0e2ec); dot(ex, top + 5, 0xb8ccd8);
      }
      dot(x + 4, top - 3, 0xf0c838); dot(x + 5, top - 4, 0xf0c838); dot(x + 6, top - 5, 0xe88a9a);
    },
    // The blueprint rolled in its hand, or unrolled in both and studied; the second pair of legs
    arms(x, top, look, o, sleeve, pose) {
      const p = pose.kind === 'walk' ? null : every(look, 15, 2.8, 2);
      if (p !== null) {
        sleeve(x - 4, top + 8, 1, 2); sleeve(x + 4, top + 8, 1, 2);
        fill(x - 4, top + 8, 9, 4, 0x2a5aa8); fill(x - 4, top + 8, 9, 1, 0x3a6ac0);
        for (const [dx, dy] of [[-3, 9], [-2, 9], [-1, 9], [-1, 10], [0, 10], [1, 10], [2, 9], [3, 9], [1, 11], [-3, 10]]) dot(x + dx, top + dy, 0xe8f0ff, 0.9);
        dot(x - 5, top + 8, 0x3a2614); dot(x + 5, top + 8, 0x3a2614);
      } else {
        costumeArm(x, top, -4, look, sleeve, 0x3a2614); costumeArm(x, top, 4, look, sleeve, 0x3a2614);
        fill(x + 5, top + 9, 1, 4, 0x2a5aa8); dot(x + 5, top + 9, 0xe8f0ff); dot(x + 5, top + 12, 0x1a3a7a);
      }
      costumeAntLegs(x, top, pose, 0x3a2614);
      return true;
    },
  },
  antcolonyhoneypot: {
    // The honeypot: an amber ant hood; a suit swelling below the chest into a great round golden belly of stored honey, glossy
    // and clear. Now and then the honey sloshes and a drop glints; by night the belly glows softly.
    front(x, y) { fill(x - 3, y, 1, 2, 0xf0bc5a); },
    back(x, y) { disc(x, y + 4, 3.5, 0xe0a030); },
    head(x, top, look, away) { costumeAntHead(x, top, look, 0xd8a040, 0xf0bc5a, 0x8a5a14, away); },
    // The honey belly, out past the arms, its highlight sloshing now and then, glowing by night; the second pair of legs
    arms(x, top, look, o, sleeve, pose) {
      const p = pose.kind === 'walk' ? null : every(look, 12, 1.8, 6), slosh = p !== null ? Math.round(Math.sin(p * Math.PI * 4) * 1) : 0;
      costumeArm(x, top, -5, look, sleeve, 0x8a5a14); costumeArm(x, top, 5, look, sleeve, 0x8a5a14);
      fill(x - 3, top + 9, 7, 1, 0x6a4010);
      disc(x, top + 13, 4.4, 0xe8a830); disc(x, top + 13, 3.4, 0xf2c050, 0.8);
      dot(x - 2 + slosh, top + 11, WHITE, 0.85); dot(x - 1 + slosh, top + 11, WHITE, 0.5); dot(x + 2, top + 15, 0xc88a20);
      if (p !== null) costumeTwinkle(x + 2 + slosh, top + 12, 0.8);
      if (isNight) glowLate(() => glowAt(x, top + 13, 6, 5, 0xffc860, 0.18));
      costumeAntLegs(x, top, pose, 0x8a5a14);
      return true;
    },
  },
  antcolonyfarmer: {
    // The aphid farmer: a black-brown ant hood; a dark suit under a brown leather apron; a plump green aphid perched on its shoulder,
    // a little milk pail in hand. Now and then the aphid hops and gives a golden drop of honeydew.
    front(x, y) { fill(x - 2, y + 1, 5, 5, 0x8a5a32); fill(x - 2, y + 1, 5, 1, 0xa8743e); dot(x - 2, y, 0x6a4424); dot(x + 2, y, 0x6a4424); fill(x - 3, y, 1, 3, 0x4a3e36); },
    back(x, y) { fill(x - 3, y + 2, 7, 1, 0x6a4424); },
    head(x, top, look, away) {
      costumeAntHead(x, top, look, 0x2e241e, 0x4a3e36, 0x0e0c0a, away);
      if (away) return;
      // The aphid on its shoulder: a plump green body, a dark eye, little legs; hopping and giving a drop now and then (see arms)
      const hop = look.costumeHop ? 1 : 0, ax = x + 5, ay = top + 5 - hop;
      fill(ax - 1, ay, 3, 2, 0x6ac04a); dot(ax - 1, ay, 0x8ad86a); dot(ax + 1, ay, 0x1a1a1a); dot(ax - 1, ay + 2, 0x3a7a2a); dot(ax + 1, ay + 2, 0x3a7a2a);
      if (hop) { dot(ax - 2, ay + 3, 0xf0c040); costumeTwinkle(ax - 2, ay + 3, 0.6); }
    },
    // The pail in one hand; the hop's moment (drawn with the head); the second pair of legs
    arms(x, top, look, o, sleeve, pose) {
      look.costumeHop = pose.kind !== 'walk' && every(look, 13, 1.6, 4) !== null && Math.floor(now * 4) % 2 === 0;
      costumeArm(x, top, -4, look, sleeve, 0x0e0c0a); costumeArm(x, top, 4, look, sleeve, 0x0e0c0a);
      fill(x - 6, top + 11, 2, 2, 0xa8adb6); dot(x - 6, top + 11, 0xd8dce4); dot(x - 5, top + 10, 0x6e727c);
      costumeAntLegs(x, top, pose, 0x0e0c0a);
      return true;
    },
  },
};
