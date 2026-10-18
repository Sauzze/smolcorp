/*
 * The Agent Office: the deck's sessions as people in a little pixel-art
 * office, for the phone's Office view and the /office page.
 *
 * Each key is a desk, in the same order, with a person at it who acts out
 * what their session is doing. Everything is drawn in code on a 320 × 180
 * grid of art pixels, or 224 × 372 on a screen held upright, scaled by a
 * whole number of the screen's pixels so every edge stays sharp.
 *
 *   AgentOffice.create(canvas, on) → { show(snapshot), close(), select(id), setVisible(shown), setFramed(framed), moved() }
 *
 * `show` takes what the deck streams; `close` empties the office when the
 * stream drops, and whoever is there when it's back is already at work.
 * `select` marks the session the deck's controls act on. `setFramed` puts
 * the room behind a window with rounded corners, as the phone shows it
 * beside its controls. `moved` says the canvas is now in another window,
 * such as one kept on top of the others, or back on its page: the room
 * goes on there, sized for that window.
 *
 * The room can be tapped, and says what was tapped through `on`, for the
 * page to do what its keys and buttons would: `on.select(id)` for a desk or
 * its person, `on.max()` for the nuclear button, `on.mic('down' | 'up')` for
 * the microphone, held, `on.style(id)` for one of the office's styles, from
 * its key in the corner, `on.music(action)` for a piece that plays music, to
 * play or pause the computer's music, 'playpause', from a tap, or from the
 * control a finger held on it opens, 'previous', 'playpause' or 'next',
 * `on.key(action)` for one of the picked agent's keys along the foot of the
 * room, out from a finger held on the agent till a tap elsewhere, 'esc', 'send',
 * 'clear', 'up', 'down' or 'button:<id>' for one of the layout's own
 * buttons (/clear and a button marked `confirm` on a second tap),
 * `on.hire(desk, role)` for an empty desk held and hired into: by the id
 * of one of the snapshot's `hiring.roles`, along the foot of the room, or
 * null for Anyone, or for its Hire button where there are no roles,
 * `on.scroll(lines | 'bottom')` for two fingers moving up and down the room,
 * or tapped together, and `on.hint(text)` for a word on the status line.
 *
 * Its sound, once set up and tapped, is in the style of an old console. A
 * piece of the room asks for one of its sounds by name with
 * `office.cue(name, x)`; see SOUNDS.
 */
(() => {
  'use strict';

  /* ------------------------------------------------------------ the floor */

  // The room is 16:9, like a screen. Desks fill the left in three rows of
  // three, one for each key in reading order; a corridor runs down from the
  // door between them and the lounge on the right.
  const WIDTH = 320, HEIGHT = 180;
  const WALL_BOTTOM = 40;   // where the back wall meets the floor
  const LOUNGE_LEFT = 224;  // the lounge's carpet starts here
  const CORRIDOR_X = 214;
  const DOOR = { x: CORRIDOR_X, y: 37 }; // just inside the doorway
  const DESK_WIDTH = 44;
  const DESK_COLUMNS = [22, 88, 154];
  const DESK_ROWS = [58, 102, 146];
  /** The top-left corner of each desk's surface, as the room has them unless arranged otherwise… */
  const STANDARD_DESKS = DESK_ROWS.flatMap((y) => DESK_COLUMNS.map((x) => ({ x, y })));
  /** …and where they are now: see `desksOf`. */
  let DESKS = STANDARD_DESKS;

  /** Where the person at a desk sits, feet hidden behind it. */
  const seat = (desk) => ({ x: DESKS[desk].x + 28, y: DESKS[desk].y + 4 });

  /** Where idle people spend their time in the lounge, whatever its corner holds, and the part of the lounge each is by. */
  const LOUNGE_SPOTS = [
    { x: 243, y: 50, pose: 'away', part: 'wall' },  // the coffee machine
    { x: 274, y: 47, pose: 'away', part: 'wall' },  // the window
    { x: 297, y: 55, pose: 'stand', part: 'wall' }, // the water cooler
    { x: 258, y: 116, pose: 'sit', part: 'couch' },  // the couch
    { x: 276, y: 116, pose: 'sit', part: 'couch' },
    { x: 294, y: 116, pose: 'sit', part: 'couch' },
    { x: 238, y: 158, pose: 'stand', part: 'pong' }, // ping-pong
    { x: 298, y: 158, pose: 'stand', part: 'pong' },
  ];
  const PING_PONG = [6, 7];

  // The nuclear button stands at the foot of the corridor, clear of every
  // route. Its presser stands to its left and reaches across onto the dome;
  // anyone else waits on the other side.
  const BUTTON = { x: CORRIDOR_X, y: 161 };
  const PRESSER = { x: CORRIDOR_X - 9, y: 163 };
  const NEXT_IN_LINE = { x: CORRIDOR_X + 12, y: 163 };

  /** The microphone on its stand, in the lounge's bottom-right corner: where its base meets the floor. */
  const MIC = { x: 311, y: 176 };
  /**
   * What the mic can be, each on the same stand: how tall its pole is, what
   * a finger finds (`tap`, from where its base meets the floor), where its
   * sound waves come from (`mouth`), and what it sounds like going on.
   */
  const MIC_KINDS = {
    mic: { pole: 17, tap: { left: -5, top: -25, right: 6, bottom: 1 }, mouth: { dx: 0, dy: -21 }, sound: 'mic-on' },
    megaphone: { pole: 17, tap: { left: -9, top: -25, right: 6, bottom: 1 }, mouth: { dx: -8, dy: -20 }, sound: 'megaphone' },
    studio: { pole: 16, tap: { left: -5, top: -29, right: 6, bottom: 1 }, mouth: { dx: 0, dy: -23 }, sound: 'mic-on' },
    podcast: { pole: 16, tap: { left: -15, top: -29, right: 6, bottom: 1 }, mouth: { dx: -12, dy: -24 }, sound: 'mic-on' },
    karaoke: { pole: 15, tap: { left: -5, top: -28, right: 6, bottom: 1 }, mouth: { dx: 0, dy: -24 }, sound: 'mic-on' },
    headset: { pole: 15, tap: { left: -8, top: -25, right: 6, bottom: 1 }, mouth: { dx: -6, dy: -17 }, sound: 'mic-on' },
    walkie: { pole: 15, tap: { left: -4, top: -31, right: 6, bottom: 1 }, mouth: { dx: 0, dy: -22 }, sound: 'mic-on' },
    tincan: { pole: 15, tap: { left: -5, top: -23, right: 9, bottom: 1 }, mouth: { dx: -3, dy: -19 }, sound: 'mic-on' },
    radio: { pole: 15, tap: { left: -6, top: -28, right: 6, bottom: 1 }, mouth: { dx: 0, dy: -22 }, sound: 'mic-on' },
  };
  /** The plant, moved to the lounge's bottom-left to make room for it: its leaves' left edge. */
  const PLANT_X = 226;

  /**
   * The room's two shapes. Wide, as above, for a screen on its side. Tall,
   * for one held upright: the same back wall, door and desks, with the
   * lounge below them, behind a wall of its own that the corridor runs
   * through. Each part of the lounge is drawn as in the wide room, moved by
   * the tall room's offset for that part, and so is every place round it.
   */
  const WIDE = { width: WIDTH, height: HEIGHT, offsets: {} };
  const TALL = {
    width: 224, height: 372,
    loungeWall: 216, // where the lounge's wall meets the floor: the back wall's height below the desks
    offsets: {
      wall: { x: -214, y: 176 }, // the counter, the window and the cooler, along the lounge's wall
      corner: { x: -120, y: 170 },
      couch: { x: -236, y: 140 }, // with its rug and coffee table
      pong: { x: -150, y: 170 },
      plant: { x: -222, y: 192 },
      button: { x: -10, y: 129 }, // in the lounge, below the doorway at the foot of the corridor
      mic: { x: -97, y: 192 },
    },
  };
  let shape = WIDE;

  /**
   * `point`, as it is in the wide room, where `part` of the lounge puts it in
   * the room's shape now, and as the room is arranged (see ARRANGEABLE):
   * moved with its piece, and spread out from the piece's anchor as far as
   * it's grown. A point of a hidden piece says so, with `off`. `arranged`
   * false leaves the arrangement out, for what stays put on a wall.
   */
  function placed(part, point, arranged = true) {
    // A piece added to the lounge comes in where the corner's piece stands.
    const base = isAdded(part) ? 'corner' : part;
    const { x, y } = shape.offsets[base] || { x: 0, y: 0 };
    const piece = arranged ? pieceOf(part) : STANDARD_PIECE;
    const anchor = PIECE_ANCHORS[base] || { x: 0, y: 0 };
    const grow = (value, from) => from + (value - from) * piece.size;
    const moved = { ...point, x: grow(point.x, anchor.x) + x + piece.x, y: grow(point.y, anchor.y) + y + piece.y };
    if (point.depth !== undefined) moved.depth = grow(point.depth, anchor.y) + y + piece.y;
    if (piece.off) moved.off = true;
    return moved;
  }

  /**
   * The room arranged your way, from the setup page's editor: what can be
   * moved, each by its name in the layout's `room.plan`, with what it's
   * called there, and whether it can be made bigger or hidden. Desks only
   * move: each is a key, and the people at them don't grow. The red button
   * and the mic are controls, so they stay. Each shape of the room has an
   * arrangement of its own, `wide` and `tall`. And pieces for the lounge
   * can be added, any of those its corner can hold, as many as ADDED_MOST,
   * the same one more than once: `added.wide` and `added.tall`, each
   * `{ kind, x, y, size }` from where the corner's piece stands. In the
   * room, the nth is the part `added:n`.
   */
  const ARRANGEABLE = {
    ...Object.fromEntries(STANDARD_DESKS.map((_, i) => [`desk${i + 1}`, { name: `Desk ${i + 1}`, grows: false, hides: true }])),
    wall: { name: 'Coffee counter and water cooler', grows: true, hides: true },
    corner: { name: "The lounge's piece", grows: true, hides: true },
    couch: { name: 'The couch, its rug and table', grows: true, hides: true },
    pong: { name: 'The ping-pong table', grows: true, hides: true },
    plant: { name: 'The plant', grows: true, hides: true },
    button: { name: 'The red button', grows: true, hides: false },
    mic: { name: 'The mic', grows: true, hides: false },
  };
  const ADDED_MOST = 12;
  /** The most pieces the back wall holds, placed where you like along it. */
  const HUNG_MOST = 12;
  const isHung = (part) => part.startsWith('hung:');
  /**
   * The lounge's pieces that play music. With the jukebox on in setup, a tap
   * on any of them plays or pauses the computer's music, and they show
   * whether it's playing, where the computer can tell.
   */
  const MUSIC_KINDS = ['jukebox', 'bubbler', 'neonbox', 'wallbox', 'djtable', 'turntable', 'boombox', 'hifi', 'synth'];
  const isAdded = (part) => part.startsWith('added:');
  /** How big a piece can be: as it comes, half as big again, or twice. */
  const PIECE_SIZES = [1, 1.5, 2];
  /** How far a piece can be moved from where it stands, either way, in art pixels. */
  const PIECE_REACH = 400;
  const STANDARD_PIECE = Object.freeze({
    x: 0, y: 0, size: 1, off: false, workspace: 'auto', monitor: 'auto', finish: 'auto', chair: 'auto', thing: 'auto',
  });
  /**
   * Where each part of the lounge stands on the floor, in the wide room: it
   * grows from there. And the part of the room each takes up, for the
   * editor to pick it by and outline it.
   */
  const PIECE_ANCHORS = {
    wall: { x: 276, y: 50 }, corner: { x: 278, y: 100 }, couch: { x: 276, y: 144 }, pong: { x: 268, y: 166 },
    plant: { x: 231, y: 168 }, button: { x: 214, y: 177 }, mic: { x: 311, y: 176 },
  };
  const PIECE_BOXES = {
    wall: { x: 230, y: 19, w: 83, h: 30 }, couch: { x: 244, y: 102, w: 64, h: 42 }, pong: { x: 246, y: 146, w: 46, h: 16 },
    plant: { x: 226, y: 154, w: 11, h: 23 }, button: { x: 206, y: 159, w: 17, h: 19 }, mic: { x: 301, y: 150, w: 14, h: 28 },
  };
  /** What each corner piece takes up, as drawn, with whatever moves: smoke, a swinging bag. */
  const CORNER_BOXES = {
    arcade: { x: 264, y: 60, w: 16, h: 27 }, vending: { x: 262, y: 58, w: 20, h: 29 }, foosball: { x: 255, y: 75, w: 34, h: 20 },
    fishtank: { x: 257, y: 60, w: 31, h: 27 }, books: { x: 265, y: 57, w: 43, h: 41 }, dragon: { x: 244, y: 68, w: 59, h: 29 },
    pinball: { x: 261, y: 51, w: 27, h: 40 }, jukebox: { x: 263, y: 50, w: 23, h: 39 }, pool: { x: 250, y: 72, w: 52, h: 21 },
    piano: { x: 261, y: 56, w: 31, h: 36 }, drums: { x: 250, y: 68, w: 54, h: 26 }, telescope: { x: 266, y: 54, w: 36, h: 39 },
    jellyfish: { x: 260, y: 51, w: 31, h: 41 }, bonsai: { x: 260, y: 60, w: 34, h: 32 }, jungle: { x: 243, y: 56, w: 64, h: 38 },
    coffee: { x: 244, y: 54, w: 58, h: 40 }, servers: { x: 258, y: 50, w: 35, h: 40 }, printer: { x: 257, y: 56, w: 42, h: 36 },
    boxing: { x: 268, y: 53, w: 41, h: 41 }, treadmill: { x: 260, y: 63, w: 32, h: 31 }, hottub: { x: 253, y: 63, w: 48, h: 34 },
    ballpit: { x: 253, y: 75, w: 48, h: 22 }, campfire: { x: 248, y: 63, w: 57, h: 33 },
    bubbler: { x: 261, y: 45, w: 27, h: 44 }, neonbox: { x: 263, y: 46, w: 23, h: 43 }, wallbox: { x: 256, y: 61, w: 41, h: 33 },
    djtable: { x: 244, y: 55, w: 64, h: 43 }, turntable: { x: 259, y: 54, w: 45, h: 39 }, boombox: { x: 257, y: 55, w: 36, h: 38 },
    hifi: { x: 250, y: 50, w: 52, h: 43 }, synth: { x: 257, y: 49, w: 40, h: 44 },
  };
  /** What `part` takes up, in the wide room as it comes: the corner's by what's in it, null with nothing there. */
  const pieceRect = (part) => (part === 'corner' ? CORNER_BOXES[decor.corner] || null
    : isAdded(part) ? CORNER_BOXES[addedOf(decor)[Number(part.slice(6))]?.kind] || null : PIECE_BOXES[part]);

  /** A room's arrangement, as given: anything unknown left out, positions within reach, sizes as offered. */
  function planOf(given) {
    const raw = given && typeof given === 'object' && !Array.isArray(given) ? given : {};
    const at = (n) => (Number.isFinite(n) ? Math.max(-PIECE_REACH, Math.min(PIECE_REACH, Math.round(n))) : 0);
    const shaped = (pieces) => {
      const arranged = {};
      if (!pieces || typeof pieces !== 'object' || Array.isArray(pieces)) return arranged;
      for (const [part, piece] of Object.entries(pieces)) {
        if (!Object.hasOwn(ARRANGEABLE, part) || !piece || typeof piece !== 'object') continue;
        const { grows, hides } = ARRANGEABLE[part];
        arranged[part] = {
          x: at(piece.x), y: at(piece.y),
          size: grows && PIECE_SIZES.includes(piece.size) ? piece.size : 1,
          off: hides && piece.off === true,
          // A desk's own workspace, monitor, finish, chair and what's on it, whoever sits at it, unless their project has its own
          ...Object.fromEntries(DESK_OWN.map((own) => [own,
            part.startsWith('desk') && DESK_CHOICES[own]().includes(piece[own]) && piece[own] !== 'auto' ? piece[own] : 'auto'])),
        };
      }
      return arranged;
    };
    const added = (list) => (Array.isArray(list) ? list : [])
      .filter((piece) => piece && typeof piece === 'object' && Object.hasOwn(CORNER_BOXES, piece.kind))
      .slice(0, ADDED_MOST)
      .map((piece) => ({ kind: piece.kind, x: at(piece.x), y: at(piece.y), size: PIECE_SIZES.includes(piece.size) ? piece.size : 1 }));
    const more = raw.added && typeof raw.added === 'object' ? raw.added : {};
    // The back wall's pieces, each round its middle: a list for the wall as you hung it, or null for its five places
    const hung = (list) => (Array.isArray(list) ? list
      .filter((piece) => piece && typeof piece === 'object' && WALL_CHOICES.includes(piece.kind) && piece.kind !== 'bare')
      .slice(0, HUNG_MOST)
      .map((piece) => ({ kind: piece.kind, x: Number.isFinite(piece.x) ? Math.max(0, Math.min(WIDTH, Math.round(piece.x))) : WALL_SLOTS[2] }))
      : null);
    const walls = raw.hung && typeof raw.hung === 'object' ? raw.hung : {};
    return {
      wide: shaped(raw.wide), tall: shaped(raw.tall), added: { wide: added(more.wide), tall: added(more.tall) },
      hung: { wide: hung(walls.wide), tall: hung(walls.tall) },
    };
  }

  /** What a desk can have of its own, each but Auto: read when asked, the lists being further on. */
  const DESK_CHOICES = {
    workspace: () => ROOM_CHOICES.workspace, monitor: () => ROOM_CHOICES.monitor, finish: () => ROOM_CHOICES.desk,
    chair: () => ROOM_CHOICES.chair, thing: () => TEAM_CHOICES.desk,
  };

  /**
   * What desk `desk` has of `own` (one of DESK_OWN): its own, as arranged,
   * or its style's mix, or 'auto' for the room's. A desk looks the same
   * held upright as on its side, wherever it stands: what it has in the
   * room's other shape counts too.
   */
  function deskOwn(desk, own, room = decor) {
    if (desk === null || desk === undefined) return 'auto';
    const other = room && room.plan && room.plan[shape === TALL ? 'wide' : 'tall'];
    const mine = [pieceOf(`desk${desk + 1}`, room)[own], other && other[`desk${desk + 1}`] && other[`desk${desk + 1}`][own]]
      .find((given) => given && given !== 'auto');
    if (mine) return mine;
    const style = ROOM_STYLES[room.preset];
    const mix = style && (own === 'thing' ? style.things : STYLE_MIXES[room.preset] && STYLE_MIXES[room.preset][own]);
    return mix ? mix[desk % mix.length] : 'auto';
  }
  /** A desk's monitor, desk finish and chair: its own, its style's, or the room's. */
  const monitorAt = (desk) => (deskOwn(desk, 'monitor') !== 'auto' ? deskOwn(desk, 'monitor') : decor.monitor);
  const finishAt = (desk) => DESK_FINISHES[deskOwn(desk, 'finish') !== 'auto' ? deskOwn(desk, 'finish') : decor.desk];
  const chairAt = (desk) => CHAIRS[deskOwn(desk, 'chair') !== 'auto' ? deskOwn(desk, 'chair') : decor.chair];

  /**
   * A room in a style written out as your own: each part the style
   * furnishes as it furnishes it, and its mix at each desk with nothing of
   * its own, so that changing one piece keeps the rest. For the setup page.
   */
  function furnishingOf(given) {
    const room = structuredClone(given && typeof given === 'object' && !Array.isArray(given) ? given : {});
    const style = ROOM_STYLES[room.preset];
    if (!style) return room;
    for (const part of STYLED) {
      room[part] = Object.hasOwn(style.room, part) ? structuredClone(style.room[part]) : part === 'wall' ? [...STANDARD_WALL] : ROOM_CHOICES[part][0];
    }
    const plan = room.plan && typeof room.plan === 'object' && !Array.isArray(room.plan) ? room.plan : {};
    const object = (value) => (value && typeof value === 'object' && !Array.isArray(value) ? value : {});
    const shapes = { wide: object(plan.wide), tall: object(plan.tall) };
    STANDARD_DESKS.forEach((_, desk) => {
      const name = `desk${desk + 1}`, pieces = { wide: object(shapes.wide[name]), tall: object(shapes.tall[name]) };
      for (const own of ['workspace', 'monitor', 'thing']) {
        const mix = own === 'thing' ? style.things : STYLE_MIXES[room.preset] && STYLE_MIXES[room.preset][own];
        // What the desk has of its own in either shape it keeps, as deskOwn has it: the mix only goes where it has nothing.
        const given = (piece) => piece[own] !== 'auto' && DESK_CHOICES[own]().includes(piece[own]);
        if (!mix || given(pieces.wide) || given(pieces.tall)) continue;
        pieces.wide[own] = pieces.tall[own] = mix[desk % mix.length];
      }
      for (const key of ['wide', 'tall']) if (Object.keys(pieces[key]).length) shapes[key][name] = pieces[key];
    });
    // The back wall as hung, in each shape, as the style shows it: its pieces in your places, in turn
    const wall = room.wall;
    const hung = object(plan.hung);
    const walls = Object.fromEntries(['wide', 'tall'].map((side) => [side, Array.isArray(hung[side])
      ? hung[side].map((piece, i) => ({ ...piece, kind: wall[i % wall.length] })) : hung[side]]));
    room.plan = { ...plan, ...shapes, ...(plan.hung ? { hung: { ...hung, ...walls } } : {}) };
    room.preset = 'yours';
    return room;
  }

  /** How `part` is arranged in the room's shape now. */
  function pieceOf(part, room = decor) {
    if (isAdded(part)) {
      const piece = addedOf(room)[Number(part.slice(6))];
      return piece ? { x: piece.x, y: piece.y, size: piece.size, off: false } : { ...STANDARD_PIECE, off: true };
    }
    const pieces = room && room.plan && room.plan[shape === TALL ? 'tall' : 'wide'];
    return (pieces && pieces[part]) || STANDARD_PIECE;
  }

  /** Every piece in the lounge's corner and added to it, by its part and what it is, those put away left out. */
  const loungePieces = (room = decor) => [
    { part: 'corner', kind: room.corner }, ...addedOf(room).map((piece, i) => ({ part: `added:${i}`, kind: piece.kind })),
  ].filter(({ part, kind }) => kind !== 'none' && !pieceOf(part, room).off);

  /** The pieces added to the lounge in the room's shape now. */
  const addedOf = (room) => (room && room.plan && room.plan.added && room.plan.added[shape === TALL ? 'tall' : 'wide']) || [];

  /** Where each desk is, as the room's shape now and its arrangement put them, and whether it's been put away. */
  const desksOf = (room) => STANDARD_DESKS.map((desk, i) => {
    const piece = pieceOf(`desk${i + 1}`, room);
    return { x: desk.x + piece.x, y: desk.y + piece.y, off: piece.off };
  });

  /**
   * What hangs on the back wall in the room's shape now, each round its
   * middle `x`: as you hung it, or in the wall's five places as set up. In a
   * style, the style's pieces go where yours are, in turn.
   */
  function hungOf(room = decor) {
    const placed = room && room.plan && room.plan.hung && room.plan.hung[shape === TALL ? 'tall' : 'wide'];
    if (!Array.isArray(placed)) return room.wall.map((kind, i) => ({ kind, x: WALL_SLOTS[i] }));
    return ROOM_STYLES[room.preset] ? placed.map((piece, i) => ({ ...piece, kind: room.wall[i % room.wall.length] })) : placed;
  }

  /** Held upright, whether `point` is in the lounge, past its wall. */
  const inLounge = (point) => shape.loungeWall !== undefined && point.y > shape.loungeWall;

  /**
   * How the office is furnished, as set up on the setup page: the layout's
   * `room`. Each part's choices, the first being what an office has when the
   * layout says nothing. The apps' Layout.swift and layout.go keep the same.
   */
  /**
   * Styles for the whole office, picked from the phone or the setup page:
   * each furnishes it from the standard. Your own furnishing stays under a
   * style, as Yours, and what a style leaves alone stays yours too. A
   * style's `things` go on its desks, one to each, unless a project has
   * its own.
   */
  const ROOM_STYLES = {
    classic: { name: 'Classic', room: {} },
    campsite: { name: 'Campsite', room: { workspace: 'tent', floor: 'forest', paint: 'logs', lounge: 'teal', corner: 'campfire', pet: 'fox',
      chair: 'green', monitor: 'laptop', wall: ['cabin', 'trail', 'cabin', 'paddles', 'cabin'] }, things: ['lantern', 'enamel', 'pinecone'] },
    royal: { name: 'Royal court', room: { workspace: 'throne', floor: 'marble', paint: 'damask', lounge: 'plum', corner: 'piano', desk: 'walnut',
      chair: 'red', pet: 'corgi', monitor: 'gilt', wall: ['stained', 'tapestry', 'portrait', 'shield', 'stained'] },
      things: ['candelabra', 'quill', 'goblet'] },
    space: { name: 'Space station', room: { workspace: 'cockpit', floor: 'plating', paint: 'hull', lounge: 'navy', corner: 'telescope', pet: 'robodog',
      desk: 'black', chair: 'grey', staff: 'none', monitor: 'holo', wall: ['viewport', 'panel', 'whiteboard', 'pipes', 'porthole'] },
      things: ['helmet', 'rocket', 'terrarium'] },
    pirates: { name: 'Pirate cove', room: { workspace: 'ship', floor: 'deck', paint: 'timber', lounge: 'sand', corner: 'jukebox', desk: 'walnut',
      pet: 'parrot', monitor: 'brass', wall: ['seaPorthole', 'map', 'seaPorthole', 'cutlasses', 'flag'] }, things: ['compass', 'rum', 'skull'] },
    arctic: { name: 'Arctic base', room: { workspace: 'igloo', floor: 'snow', paint: 'ice', lounge: 'teal', corner: 'hottub', desk: 'white',
      chair: 'grey', pet: 'penguin', monitor: 'ice', wall: ['frost', 'skis', 'frost', 'weather', 'frost'] },
      things: ['snowglobe', 'thermos', 'penguin'] },
    lair: { name: "Dragon's lair", room: { workspace: 'greenhouse', floor: 'flagstone', paint: 'stone', lounge: 'plum', corner: 'dragon',
      pet: 'owl', chair: 'green', monitor: 'crystal', wall: ['torch', 'arch', 'torch', 'arch', 'torch'] },
      things: ['potions', 'egg', 'spellbook'] },
    gamers: { name: 'Gamer den', room: { workspace: 'gaming', floor: 'arcade', paint: 'foam', lounge: 'navy', corner: 'pinball', desk: 'black', pet: 'hamster',
      chair: 'black', monitor: 'ultrawide', wall: ['neon', 'tv', 'hex', 'tv', 'neon'] }, things: ['controller', 'energy', 'headset'] },
    garage: { name: 'Startup garage', room: { workspace: 'box', floor: 'epoxy', paint: 'brick', lounge: 'sand', corner: 'servers',
      chair: 'grey', staff: 'many', monitor: 'dual', wall: ['whiteboard', 'roller', 'pegboard', 'whiteboard', 'bike'] },
      things: ['circuit', 'pizza', 'coffeepot'] },
    gym: { name: 'Fitness floor', room: { workspace: 'standing', floor: 'rubber', paint: 'block', lounge: 'teal', corner: 'boxing',
      mic: 'megaphone', desk: 'white', monitor: 'tablet', wall: ['mirror', 'whiteboard', 'timer', 'climbing', 'mirror'] },
      things: ['dumbbell', 'shaker', 'kettlebell'] },
    jungle: { name: 'Jungle', room: { workspace: 'beanbag', floor: 'grass', paint: 'bamboo', lounge: 'teal', corner: 'jungle', pet: 'monkey',
      monitor: 'bamboo', wall: ['vines', 'blind', 'tiki', 'blind', 'vines'] }, things: ['coconut', 'pineapple', 'tikimug'] },
  };
  /** What a style furnishes. The sky, the seasons, the sound and your team stay as you have them. */
  const STYLED = ['mic', 'wall', 'floor', 'paint', 'lounge', 'loungePaint', 'corner', 'pet', 'desk', 'workspace', 'monitor', 'chair', 'staff'];
  /**
   * How a style mixes its desks, desk by desk round each list, wherever a
   * desk has nothing of its own: its own workspace among others, and its
   * monitors. Its `things` go on them the same way.
   */
  const STYLE_MIXES = {
    campsite: { workspace: ['tent', 'log', 'tent'] },
    royal: { workspace: ['throne', 'gilded', 'throne'] },
    space: { workspace: ['cockpit', 'console', 'hover'], monitor: ['holo', 'holo', 'portrait'] },
    pirates: { workspace: ['ship', 'chest', 'ship'] },
    arctic: { workspace: ['igloo', 'crates', 'igloo'] },
    lair: { workspace: ['greenhouse', 'altar', 'greenhouse'] },
    gamers: { workspace: ['gaming', 'glass', 'gaming'], monitor: ['ultrawide', 'dual', 'ultrawide'] },
    garage: { workspace: ['box', 'bench', 'box'], monitor: ['crt', 'dual', 'crt'] },
    gym: { workspace: ['standing', 'rack', 'standing'] },
    jungle: { workspace: ['beanbag', 'bamboo', 'beanbag'] },
  };
  /** What a desk can have of its own in the room's arrangement, besides where it is. */
  const DESK_OWN = ['workspace', 'monitor', 'finish', 'chair', 'thing'];

  const ROOM_CHOICES = {
    mic: ['mic', 'megaphone', 'studio', 'podcast', 'karaoke', 'headset', 'walkie', 'tincan', 'radio'],
    floor: ['wood', 'tiles', 'concrete', 'carpet', 'plating', 'deck', 'marble', 'flagstone', 'snow', 'grass', 'forest', 'rubber',
      'epoxy', 'arcade'],
    paint: ['slate', 'sage', 'clay', 'cream', 'hull', 'timber', 'damask', 'stone', 'ice', 'logs', 'brick', 'bamboo', 'block', 'foam'],
    // The lounge's floor: a carpet, or any of the office's floors; and its wall, as the office's unless it has its own
    lounge: ['navy', 'teal', 'plum', 'sand', 'wood', 'tiles', 'concrete', 'carpet', 'plating', 'deck', 'marble', 'flagstone', 'snow', 'grass',
      'forest', 'rubber', 'epoxy', 'arcade'],
    loungePaint: ['auto', 'slate', 'sage', 'clay', 'cream', 'hull', 'timber', 'damask', 'stone', 'ice', 'logs', 'brick', 'bamboo', 'block',
      'foam'],
    corner: ['arcade', 'vending', 'foosball', 'fishtank', 'books', 'dragon', 'pinball', 'jukebox', 'pool', 'piano', 'drums',
      'telescope', 'jellyfish', 'bonsai', 'jungle', 'coffee', 'servers', 'printer', 'boxing', 'treadmill', 'hottub', 'ballpit',
      'campfire', 'bubbler', 'neonbox', 'wallbox', 'djtable', 'turntable', 'boombox', 'hifi', 'synth', 'none'],
    sky: ['live', 'day'],
    pet: ['none', 'cat', 'dog', 'lizard', 'monkey', 'llama', 'penguin', 'parrot', 'owl', 'capybara', 'redpanda', 'corgi', 'hamster',
      'robodog', 'bat', 'snail', 'fox', 'axolotl'],
    season: ['calendar', 'off'],
    sound: ['off', 'quiet', 'normal'],
    desk: ['wood', 'walnut', 'white', 'black'],
    workspace: ['desk', 'standing', 'gaming', 'tent', 'box', 'beanbag', 'igloo', 'van', 'cockpit', 'ship', 'throne', 'greenhouse',
      'console', 'hover', 'chest', 'gilded', 'crates', 'altar', 'log', 'bench', 'bamboo', 'rack', 'drafting', 'glass'],
    monitor: ['standard', 'crt', 'holo', 'brass', 'crystal', 'gilt', 'ice', 'ultrawide', 'dual', 'laptop', 'bamboo', 'tablet', 'portrait'],
    chair: ['teal', 'black', 'red', 'grey', 'green'],
    staff: ['some', 'none', 'many'],
    preset: ['yours', ...Object.keys(ROOM_STYLES)],
  };
  /**
   * The desks' finish when no effort paints them, from the edge catching the
   * light to the drawer; the chairs' colour, its back and its top; and how
   * many of the office's own staff hang about in the lounge with no sessions
   * running, one fewer for each that is.
   */
  const DESK_FINISHES = {
    wood: { edge: 0xdcb78c, top: 0xc49a6c, front: 0x8c6744, lip: 0x76573a, drawer: 0x9b7550 },
    walnut: { edge: 0xa87a56, top: 0x7e5536, front: 0x5a3b25, lip: 0x4a301e, drawer: 0x6a4630 },
    white: { edge: 0xfbfaf6, top: 0xe9e6de, front: 0xc9c4b8, lip: 0xb3ad9f, drawer: 0xd8d3c8 },
    black: { edge: 0x5a5f6c, top: 0x3c404a, front: 0x272a31, lip: 0x1f2228, drawer: 0x30333b },
  };
  const CHAIRS = {
    teal: [0x2f5560, 0x3f7080], black: [0x23252c, 0x353842], red: [0x7a2e2e, 0x9a3c3a], grey: [0x5a5f6c, 0x737987],
    green: [0x2f5a3a, 0x3f7a4c],
  };
  const STAFF_COUNTS = { none: 0, some: 4, many: 6 };
  /**
   * What each desk can be, besides a desk: where its screen is, from the
   * desk's corner (the top-left of a monitor 13 × 9, whose back shows the
   * turn's time); whether its person stands at it; whether it has the
   * office's chair, or what's carried over their head instead when they're
   * moved to another desk; where what's on the desk goes, moved from
   * where a desk has it; and how far above the desk's corner it reaches, so
   * a finger finds it there. Each is drawn in drawOffice's WORKSPACE_PARTS.
   */
  const WORKSPACES = {
    desk: { screen: { dx: 6, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -14 },
    standing: { screen: { dx: 6, dy: -12 }, stands: true, chair: false, carry: [0x5a5f6c, 0x737987], things: { dx: 0, dy: 0 }, top: -14 },
    gaming: { screen: { dx: 6, dy: -10 }, stands: false, chair: false, carry: [0x1d1f25, 0xd4140b], things: { dx: 0, dy: 0 }, top: -15 },
    tent: { screen: { dx: 22, dy: -3 }, laptop: true, stands: false, chair: false, carry: [0x6f8a4a, 0x8aa65e], things: null, top: -24 },
    box: { screen: { dx: 6, dy: -9 }, laptop: true, stands: false, chair: false, carry: [0xa07a4a, 0xc9a06a], things: { dx: 0, dy: 1 }, top: -14 },
    beanbag: { screen: { dx: 22, dy: -4 }, laptop: true, stands: false, chair: false, carry: [0xc0584a, 0xd96a5a], things: { dx: 4, dy: 12 }, top: -14 },
    igloo: { screen: { dx: 6, dy: -10 }, stands: false, chair: false, carry: [0xc4d2de, 0xeef4f8], things: { dx: 0, dy: 0 }, top: -22 },
    van: { screen: { dx: 11, dy: -10 }, stands: false, chair: true, things: { dx: -1, dy: 0 }, top: -21 },
    cockpit: { screen: { dx: 6, dy: -10 }, stands: false, chair: false, carry: [0x3a3f4a, 0x8a93a3], things: { dx: 0, dy: 0 }, top: -24 },
    ship: { screen: { dx: 6, dy: -10 }, stands: false, chair: false, carry: [0x7a5232, 0x3a3a40], things: null, top: -24 },
    throne: { screen: { dx: 6, dy: -10 }, stands: false, chair: false, carry: [0xd6b25e, 0x9a2a3a], things: { dx: 0, dy: 0 }, top: -23 },
    greenhouse: { screen: { dx: 4, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -23 },
    // Desks of other kinds, each with the office's chair and a monitor where a desk has it
    console: { screen: { dx: 6, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -14 },
    hover: { screen: { dx: 6, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -14 },
    chest: { screen: { dx: 6, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -14 },
    gilded: { screen: { dx: 6, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -14 },
    crates: { screen: { dx: 6, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -14 },
    altar: { screen: { dx: 6, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -14 },
    log: { screen: { dx: 6, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -14 },
    bench: { screen: { dx: 6, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -14 },
    bamboo: { screen: { dx: 6, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -14 },
    rack: { screen: { dx: 6, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -14 },
    drafting: { screen: { dx: 6, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -14 },
    glass: { screen: { dx: 6, dy: -10 }, stands: false, chair: true, things: { dx: 0, dy: 0 }, top: -14 },
  };
  /** The colours of what can sit on a desk besides a plant, papers and a lamp. */
  const DESK_THINGS = { cactus: 0x5a9a4a, duck: 0xf2cf3a, beak: 0xe8823a, sky: 0x8fc0e8, coffee: 0x5a3a24 };
  /** What each of the back wall's five places can hold, and what it holds unless set up otherwise. */
  const WALL_CHOICES = ['window', 'whiteboard', 'pinboard', 'poster', 'shelf', 'plant', 'bare',
    'viewport', 'porthole', 'panel', 'pipes', 'seaPorthole', 'map', 'flag', 'cutlasses', 'tapestry', 'portrait', 'stained',
    'shield', 'frost', 'skis', 'weather', 'torch', 'arch', 'neon', 'tv', 'hex', 'pegboard', 'roller',
    'bike', 'mirror', 'timer', 'climbing', 'vines', 'tiki', 'blind', 'cabin', 'trail', 'paddles'];
  const STANDARD_WALL = ['window', 'whiteboard', 'window', 'pinboard', 'window'];
  /** The middle of each of the back wall's places, left to right. */
  const WALL_SLOTS = [44, 77, 110, 143, 176];
  /**
   * How wide each piece on the back wall is drawn, in art pixels, sills,
   * shadows and whatever moves on it included, as far each way from its
   * middle as its wider side: the editor keeps them apart by it. Unlisted, a window's width.
   */
  const WALL_WIDTHS = {
    window: 36, whiteboard: 24, pinboard: 20, poster: 16, shelf: 26, plant: 16, viewport: 36, porthole: 22, panel: 26,
    pipes: 40, seaPorthole: 22, map: 28, flag: 24, cutlasses: 22, tapestry: 26, portrait: 20, stained: 26, shield: 24,
    frost: 36, skis: 20, weather: 22, torch: 20, arch: 20, neon: 14, tv: 32, hex: 26, pegboard: 26, roller: 34,
    bike: 26, mirror: 24, timer: 28, climbing: 26, vines: 32, tiki: 12, blind: 36, cabin: 34, trail: 26, paddles: 22,
  };
  const wallWidthOf = (kind) => WALL_WIDTHS[kind] || 32;
  /** Where along the back wall a piece can hang, edge to edge, in each shape: clear of the doorway. */
  const WALL_RANGE = { wide: [0, 194], tall: [0, 194] };

  /** A room as the layout gives it: anything unknown or left out is as standard. */
  /**
   * The dress code: an outfit and colour for each size of model, and for
   * each agent while its model isn't known, and your own rules for telling
   * a model's size from its name. Outfits and colours are the team's
   * choices. The apps' Layout.swift and layout.go keep the same.
   */
  const DRESS_SIZES = ['small', 'medium', 'large', 'top'];
  const DRESS_AGENTS = ['claude', 'codex', 'gemini', 'opencode', 'grok'];
  /** The most rules a dress code keeps, and the most characters a rule's words can be. */
  const DRESS_RULE_LIMIT = 20;
  const DRESS_WORDS_LIMIT = 32;
  const AUTO_ATTIRE = { outfit: 'auto', colour: 'auto' };

  /** How many pets the office can have at once. */
  const PETS_MOST = 3;

  /** A dress code as given: anything unknown is Auto, and a rule that can't be read is left out. */
  function dressOf(given) {
    const raw = given && typeof given === 'object' && !Array.isArray(given) ? given : {};
    // Read only when there's something to read: this runs as the office loads, before the team's choices exist.
    const attire = (a) => (a && typeof a === 'object' ? {
      outfit: TEAM_CHOICES.outfit.includes(a.outfit) ? a.outfit : 'auto',
      colour: TEAM_CHOICES.colour.includes(a.colour) ? a.colour : 'auto',
    } : { ...AUTO_ATTIRE });
    const each = (keys, from) => Object.fromEntries(keys.map((key) =>
      [key, attire(from && typeof from === 'object' && Object.hasOwn(from, key) ? from[key] : null)]));
    const rules = (Array.isArray(raw.rules) ? raw.rules : [])
      .map((rule) => ({
        contains: rule && typeof rule.contains === 'string'
          ? [...rule.contains.replace(/[\x00-\x1f\x7f]/g, '').trim().toLowerCase()].slice(0, DRESS_WORDS_LIMIT).join('') : '',
        size: rule && rule.size,
      }))
      .filter((rule) => rule.contains && DRESS_SIZES.includes(rule.size))
      .slice(0, DRESS_RULE_LIMIT);
    return { sizes: each(DRESS_SIZES, raw.sizes), agents: each(DRESS_AGENTS, raw.agents), rules };
  }

  function roomOf(given) {
    const raw = given && typeof given === 'object' ? given : {};
    const room = {};
    for (const [part, choices] of Object.entries(ROOM_CHOICES)) room[part] = choices.includes(raw[part]) ? raw[part] : choices[0];
    room.wall = STANDARD_WALL.map((standard, i) => (Array.isArray(raw.wall) && WALL_CHOICES.includes(raw.wall[i]) ? raw.wall[i] : standard));
    room.team = teamOf(raw.team);
    room.dress = dressOf(raw.dress);
    room.plan = planOf(raw.plan);
    // In a style, the style furnishes it, from the standard.
    const style = ROOM_STYLES[room.preset];
    if (style) {
      for (const part of STYLED) {
        const standard = part === 'wall' ? [...STANDARD_WALL] : ROOM_CHOICES[part][0];
        room[part] = Object.hasOwn(style.room, part) ? structuredClone(style.room[part]) : standard;
      }
    }
    // The pets: up to PETS_MOST of them, each once, given as a list, or one by its id as before. In a style, the ones you've
    // picked take the place of its own; with none picked, it has its own.
    const picked = (Array.isArray(raw.pet) ? raw.pet : [raw.pet]).filter((kind, i, all) => kind !== 'none' && ROOM_CHOICES.pet.includes(kind) && all.indexOf(kind) === i).slice(0, PETS_MOST);
    room.pets = style && !picked.length ? (room.pet === 'none' ? [] : [room.pet]) : picked;
    room.pet = room.pets[0] || 'none';
    return room;
  }

  /**
   * Whether someone at a spot of this pose has their back to the room:
   * standing 'away', walking on the spot on a treadmill, or perched on a
   * seat that faces away, as at a piano. 'sit' and 'stand' face the room.
   */
  const facesAway = (pose) => pose === 'away' || pose === 'walk' || pose === 'perch';

  /** Where idle people go round the lounge's corner piece: to play, to buy a snack, to watch the fish. */
  const CORNER_SPOTS = {
    arcade: [{ x: 272, y: 93, pose: 'away' }],
    vending: [{ x: 272, y: 93, pose: 'away' }],
    foosball: [{ x: 272, y: 76, pose: 'stand' }, { x: 272, y: 97, pose: 'away' }],
    fishtank: [{ x: 272, y: 93, pose: 'away' }],
    books: [{ x: 274, y: 94, pose: 'sit' }],
    dragon: [{ x: 306, y: 96, pose: 'stand' }],
    pinball: [{ x: 274, y: 95, pose: 'away' }],
    jukebox: [{ x: 274, y: 94, pose: 'away' }],
    pool: [{ x: 245, y: 86, pose: 'stand' }, { x: 306, y: 86, pose: 'stand' }],
    piano: [{ x: 276, y: 90, pose: 'perch' }],
    drums: [{ x: 276, y: 84, pose: 'sit' }],
    telescope: [{ x: 270, y: 94, pose: 'away' }],
    jellyfish: [{ x: 275, y: 95, pose: 'away' }],
    bonsai: [{ x: 258, y: 94, pose: 'away' }],
    jungle: [],
    coffee: [{ x: 274, y: 77, pose: 'stand' }, { x: 262, y: 98, pose: 'away' }],
    servers: [{ x: 268, y: 95, pose: 'away' }],
    printer: [{ x: 272, y: 95, pose: 'away' }],
    boxing: [{ x: 266, y: 90, pose: 'stand' }],
    treadmill: [{ x: 274, y: 89, pose: 'walk' }],
    hottub: [{ x: 266, y: 88, pose: 'sit' }, { x: 287, y: 88, pose: 'sit' }],
    ballpit: [{ x: 277, y: 88, pose: 'sit' }],
    campfire: [{ x: 255, y: 92, pose: 'sit' }, { x: 297, y: 92, pose: 'sit' }],
    bubbler: [{ x: 274, y: 95, pose: 'away' }],
    neonbox: [{ x: 274, y: 94, pose: 'away' }],
    wallbox: [{ x: 266, y: 85, pose: 'sit' }, { x: 286, y: 85, pose: 'sit' }],
    djtable: [{ x: 276, y: 82, pose: 'stand' }],
    turntable: [{ x: 276, y: 95, pose: 'away' }],
    boombox: [{ x: 296, y: 93, pose: 'stand' }],
    hifi: [{ x: 276, y: 96, pose: 'away' }],
    synth: [{ x: 277, y: 93, pose: 'away' }],
    none: [],
  };

  /** The first whiteboard on the back wall, if there is one: where a to-do list gets written up. */
  const boardOf = (room = decor) => hungOf(room).find((piece) => piece.kind === 'whiteboard') || null;
  /** In front of it, where whoever writes the list stands. */
  function boardSpotsOf(room) {
    const board = boardOf(room);
    return board ? [{ x: board.x - 5, y: 44 }, { x: board.x + 6, y: 44 }] : [];
  }

  // The office as it's furnished now. There is one to a page, so the
  // drawing and the people's comings and goings read it from here.
  let decor = roomOf(null);
  /** The lounge's spots, and those round its corner piece, where the room's shape puts them. */
  const spotsOf = (room) => [
    ...LOUNGE_SPOTS.map((spot) => placed(spot.part, spot)),
    // Round each piece in the lounge, each spot knowing which piece it's at, and the first of each its first
    ...[{ part: 'corner', kind: room.corner }, ...addedOf(room).map((piece, i) => ({ part: `added:${i}`, kind: piece.kind }))]
      .flatMap(({ part, kind }) => CORNER_SPOTS[kind].map((spot, i) => ({ ...placed(part, spot), kind, first: i === 0 }))),
  ];
  /** Where the spots round the lounge's `part` start, among the room's spots. */
  function spotsFrom(part, room = decor) {
    let at = LOUNGE_SPOTS.length + CORNER_SPOTS[room.corner].length;
    if (part === 'corner') return LOUNGE_SPOTS.length;
    const added = addedOf(room);
    for (let i = 0; i < Number(part.slice(6)); i++) at += CORNER_SPOTS[added[i].kind].length;
    return at;
  }
  let spots = spotsOf(decor);
  let boardSpots = boardSpotsOf(decor);
  /**
   * What a session's error, by its kind, has its agent do in the office:
   * 'auth' locked out at the door, 'overloaded' watching a storm, 'billing'
   * out of order at the desk. A rate limit waits under the clock, and any
   * other error is head in hands at the desk.
   */
  const FAILURE_KINDS = {
    authentication_failed: 'auth', oauth_org_not_allowed: 'auth', account_on_hold: 'auth', cloud_credential_error: 'auth',
    overloaded: 'overloaded', billing_error: 'billing',
  };
  const failureOf = (session) => (session && session.state === 'error' && FAILURE_KINDS[session.errorKind]) || null;
  /** Which of two ways a person takes a failure: knocking or patting their pockets, at a window or under a cloud, a sign or a closed one. */
  const takeOf = (person) => (person.look.phase < 0.5 ? 0 : 1);
  /** In front of the door, where someone locked out knocks or looks for their key. */
  const LOCKED_SPOTS = [{ x: 214, y: 46 }, { x: 205, y: 48 }, { x: 223, y: 48 }];
  /** The back wall's pieces you can see a storm through, and their glass, from the middle of their place. */
  const STORM_PANES = {
    window: [-16, 6, 32, 20], viewport: [-16, 6, 32, 20], frost: [-16, 6, 32, 20], cabin: [-14, 6, 28, 20], blind: [-16, 6, 32, 20],
  };
  /** In front of each window on the back wall, where someone watches a storm: which of the wall's pieces it is, too. */
  const stormSpotsOf = (room) => hungOf(room).flatMap((piece, i) => (STORM_PANES[piece.kind] ? [{ x: piece.x, y: 44, hung: i }] : []));
  let stormSpots = stormSpotsOf(decor);
  /** After this long at a failure, in seconds, a sigh now and then. */
  const SIGH_AFTER = 90;

  /** Under the clock by the door, where a rate-limited agent waits. */
  const CLOCK_SPOTS = [{ x: 202, y: 47 }, { x: 193, y: 47 }, { x: 184, y: 47 }];
  /**
   * Where the agents of desks put away wait, having nowhere to sit: in a
   * line along the back wall, left of the clock by the door, facing the
   * room, the first nearest the door.
   */
  const CROWD_GAP = 14;
  const CROWD_SPOTS = Array.from({ length: STANDARD_DESKS.length }, (_, i) => ({ x: 176 - i * CROWD_GAP, y: 46 }));
  /**
   * Where each put-away desk's agent stands, as the last frame drew them,
   * given as the corner a desk there would have: what its chips, its
   * button, its name and the marks of the picked desk go round, in place
   * of the desk, and `feet`, where they stand. See `anchorOf`.
   */
  const crowdAnchors = new Map();
  /** The corner a desk's chips, button, name and marks go round: the desk's own, or for one put away, where its agent stands. */
  const anchorOf = (desk) => (DESKS[desk] && DESKS[desk].off && crowdAnchors.get(desk)) || DESKS[desk];
  /** Where a desk's subagents stand round it: two on each side. */
  const INTERN_SLOTS = [{ dx: -6, dy: 4 }, { dx: DESK_WIDTH + 5, dy: 4 }, { dx: -6, dy: 15 }, { dx: DESK_WIDTH + 5, dy: 15 }];
  /** The shredder beside a desk, on the floor to its right, and where its feeder stands. */
  const SHREDDER = { dx: DESK_WIDTH + 2 };
  const SHREDDING = { dx: DESK_WIDTH + 13, dy: 15 };

  // Where a person is, or is heading: 'desk:n', 'spot:n', 'door', 'button',
  // 'queue' (the spot beside it), 'board:n', 'clock:n', 'shred:desk', or for
  // a subagent, 'near:desk:slot'.
  const deskPlace = (desk) => `desk:${desk}`;
  const spotPlace = (spot) => `spot:${spot}`;
  const numberAfter = (prefix, place) => (place.startsWith(prefix) ? Number(place.split(':')[1]) : null);
  const deskOf = (place) => numberAfter('desk:', place);
  const spotOf = (place) => numberAfter('spot:', place);

  function pointOf(place) {
    const [kind, a, b] = place.split(':');
    const desk = DESKS[Number(a)];
    switch (kind) {
      case 'desk': return seat(Number(a));
      case 'spot': return spots[Number(a)];
      case 'board': return boardSpots[Number(a)];
      case 'clock': return CLOCK_SPOTS[Number(a)];
      case 'locked': return LOCKED_SPOTS[Number(a)];
      case 'storm': return stormSpots[Number(a)];
      case 'shred': return { x: desk.x + SHREDDING.dx, y: desk.y + SHREDDING.dy };
      case 'near': {
        const at = anchorOf(Number(a));
        return { x: at.x + INTERN_SLOTS[Number(b)].dx, y: at.y + INTERN_SLOTS[Number(b)].dy };
      }
      case 'crowd': return CROWD_SPOTS[Number(a)];
      default: return { door: DOOR, button: placed('button', PRESSER), queue: placed('button', NEXT_IN_LINE) }[place];
    }
  }

  /*
   * Walking: the floor as a grid of cells, each free or taken by something
   * standing on it (a desk, the couch, the lounge's piece…, as they're
   * arranged), with the walls round it and, held upright, the lounge's own
   * wall but for its doorway. A* finds the shortest way between free cells,
   * then the way is pulled straight wherever the floor between two of its
   * points is free, so people walk in long straight lines round what's in
   * their way. A place taken by furniture itself, such as a seat at a desk
   * or in the hot tub, is reached from the free cell nearest to it.
   */

  /** How big a cell of the floor is, in art pixels. */
  const CELL = 2;
  /** What each part of the lounge stands on, in the wide room, as it comes: placed as it's arranged. */
  const FOOTPRINTS = {
    wall: [{ x: 230, y: 41, w: 28, h: 9 }, { x: 302, y: 42, w: 11, h: 7 }], // the counter, the cooler
    couch: [{ x: 246, y: 104, w: 62, h: 14 }, { x: 256, y: 130, w: 40, h: 8 }], // the couch, its table
    pong: [{ x: 246, y: 150, w: 46, h: 10 }],
    plant: [{ x: 226, y: 160, w: 12, h: 8 }],
    button: [{ x: 207, y: 168, w: 15, h: 9 }],
    mic: [{ x: 308, y: 170, w: 6, h: 6 }],
  };
  /** The doorway onto the corridor, at the top of the floor, kept free of the wall: and held upright, the lounge's. */
  const DOORWAY = { left: 206, right: 222 };

  let floor = null; // the grid, for the room as it is now: built when it's first walked, and again once the room changes

  /** The room's floor has changed: its shape, its furniture or where it stands. */
  function floorChanged() {
    floor = null;
  }

  function floorNow() {
    if (floor) return floor;
    const columns = Math.ceil(shape.width / CELL), rows = Math.ceil(shape.height / CELL);
    const taken = new Uint8Array(columns * rows);
    const take = (left, top, right, bottom) => {
      for (let row = Math.max(0, Math.floor(top / CELL)); row <= Math.min(rows - 1, Math.floor(bottom / CELL)); row++) {
        for (let column = Math.max(0, Math.floor(left / CELL)); column <= Math.min(columns - 1, Math.floor(right / CELL)); column++) {
          taken[row * columns + column] = 1;
        }
      }
    };
    // The back wall, but for the doorway onto the corridor
    take(0, 0, DOORWAY.left - 1, WALL_BOTTOM - 1);
    take(DOORWAY.right + 1, 0, shape.width, WALL_BOTTOM - 1);
    take(DOORWAY.left, 0, DOORWAY.right, DOOR.y - 3);
    // Held upright, the lounge's wall, but for its doorway
    if (shape.loungeWall !== undefined) {
      take(0, shape.loungeWall - WALL_BOTTOM, DOORWAY.left - 1, shape.loungeWall - 1);
      take(DOORWAY.right + 1, shape.loungeWall - WALL_BOTTOM, shape.width, shape.loungeWall - 1);
    }
    for (const { x, y, off } of DESKS) if (!off) take(x - 1, y - 1, x + DESK_WIDTH + 1, y + 17);
    // Each piece in the lounge stands on the bottom of what it takes up: what's above is the piece itself, or its smoke.
    const standing = ({ part, kind }) => {
      const box = CORNER_BOXES[kind], h = Math.max(6, Math.round(box.h * 0.4));
      return [part, [{ ...box, y: box.y + box.h - h, h }]];
    };
    for (const [part, rects] of [...Object.entries(FOOTPRINTS), ...loungePieces().map(standing)]) {
      if (pieceOf(part).off) continue;
      for (const { x, y, w, h } of rects) {
        const a = placed(part, { x, y }), b = placed(part, { x: x + w, y: y + h });
        take(a.x, a.y, b.x, b.y);
      }
    }
    // How far each free cell is from anything taken, in cells, up to CLEARANCE_MOST: routes keep off the edges
    const clearance = new Uint8Array(columns * rows);
    const queue = [];
    for (let cell = 0; cell < taken.length; cell++) if (taken[cell]) queue.push(cell); else clearance[cell] = CLEARANCE_MOST;
    for (let i = 0; i < queue.length; i++) {
      const at = queue[i], column = at % columns, row = Math.floor(at / columns), d = taken[at] ? 0 : clearance[at];
      if (d + 1 >= CLEARANCE_MOST) continue;
      for (let dy = -1; dy <= 1; dy++) {
        for (let dx = -1; dx <= 1; dx++) {
          const c = column + dx, r = row + dy;
          if ((!dx && !dy) || c < 0 || r < 0 || c >= columns || r >= rows) continue;
          const next = r * columns + c;
          if (taken[next] || clearance[next] <= d + 1) continue;
          clearance[next] = d + 1;
          queue.push(next);
        }
      }
    }
    floor = { columns, rows, taken, clearance };
    return floor;
  }
  /** How far a route keeps from a point it's asked to avoid, in art pixels. */
  const AVOID_REACH = 6;
  /** How far, in cells, routes care to keep from an edge, and what it costs to go nearer, by how near: next to it, one off, two off. */
  const CLEARANCE_MOST = 4;
  const EDGE_COST = [0, 3, 0.9, 0.25];

  /** Whether nothing stands on the floor at `point`. */
  function standsFree(point) {
    const { columns, rows, taken } = floorNow();
    const column = Math.floor(point.x / CELL), row = Math.floor(point.y / CELL);
    return column >= 0 && row >= 0 && column < columns && row < rows && !taken[row * columns + column];
  }

  /** The way from where someone stands to a place, round what's in the way. */
  function route(from, place) {
    const target = pointOf(place);
    const path = walkway(from, target);
    return path.filter((p, i) => i > 0 ? p.x !== path[i - 1].x || p.y !== path[i - 1].y : p.x !== from.x || p.y !== from.y);
  }

  /**
   * The points to walk through from `from` to `to`, `to` last; with
   * `strict`, null when there's no way through. `avoid` are points to keep
   * `AVOID_REACH` from as well, such as a pet sitting where it's going.
   */
  function walkway(from, to, strict = false, avoid = []) {
    const floorHere = floorNow(), { columns, rows } = floorHere;
    let { taken, clearance } = floorHere;
    if (avoid.length) {
      taken = taken.slice();
      clearance = clearance.slice();
      const reach = Math.ceil(AVOID_REACH / CELL);
      // Never near where the way starts or ends: those stay open, so it can always set off and arrive
      const open = (c, r) => [from, to].some((p) => Math.hypot(c * CELL + CELL / 2 - p.x, r * CELL + CELL / 2 - p.y) < AVOID_REACH);
      for (const p of avoid) {
        const pc = Math.floor(p.x / CELL), pr = Math.floor(p.y / CELL);
        for (let r = pr - reach; r <= pr + reach; r++) for (let c = pc - reach; c <= pc + reach; c++) {
          if (c < 0 || r < 0 || c >= columns || r >= rows || (c - pc) ** 2 + (r - pr) ** 2 > reach * reach || open(c, r)) continue;
          const cell = r * columns + c;
          taken[cell] = 1;
          clearance[cell] = 0;
        }
      }
    }
    const cellOf = (p) => {
      const column = Math.max(0, Math.min(columns - 1, Math.floor(p.x / CELL)));
      const row = Math.max(0, Math.min(rows - 1, Math.floor(p.y / CELL)));
      return row * columns + column;
    };
    const centre = (cell) => ({ x: (cell % columns) * CELL + CELL / 2, y: Math.floor(cell / columns) * CELL + CELL / 2 });
    /** The free cell nearest to `cell`, itself if it's free. */
    const nearestFree = (cell) => {
      if (!taken[cell]) return cell;
      const seen = new Uint8Array(columns * rows);
      const queue = [cell];
      seen[cell] = 1;
      for (let i = 0; i < queue.length; i++) {
        const at = queue[i], column = at % columns, row = Math.floor(at / columns);
        for (const [dx, dy] of [[0, -1], [0, 1], [-1, 0], [1, 0]]) {
          const c = column + dx, r = row + dy;
          if (c < 0 || r < 0 || c >= columns || r >= rows) continue;
          const next = r * columns + c;
          if (seen[next]) continue;
          if (!taken[next]) return next;
          seen[next] = 1;
          queue.push(next);
        }
      }
      return cell;
    };
    const start = nearestFree(cellOf(from)), goal = nearestFree(cellOf(to));
    const cells = search(start, goal, columns, rows, taken, clearance);
    if (!cells) return strict ? null : [{ x: to.x, y: to.y }]; // walled in: straight there, as before there were walls
    // Pulled straight: from each point, on to the farthest one in plain sight, as far off the edges as the two ends are
    const room = (p) => clearance[cellOf(p)];
    const clear = (pa, pb) => {
      const need = Math.min(2, room(pa), room(pb));
      // Every half pixel, so a line that only grazes a corner of something still counts
      const steps = Math.ceil(Math.hypot(pb.x - pa.x, pb.y - pa.y) / 0.5);
      for (let k = 1; k < steps; k++) {
        const cell = cellOf({ x: pa.x + ((pb.x - pa.x) * k) / steps, y: pa.y + ((pb.y - pa.y) * k) / steps });
        if (taken[cell] || clearance[cell] < need) return false;
      }
      return true;
    };
    const points = [];
    // From where they stand, or out from behind the furniture first
    let here = start === cellOf(from) ? { x: from.x, y: from.y } : centre(start);
    if (start !== cellOf(from)) points.push(here);
    let at = 0;
    while (at < cells.length - 1) {
      // As far along the way as can be seen from here, looking a little further each time
      let next = at + 1;
      while (next + 1 < cells.length && clear(here, centre(cells[next + 1]))) next++;
      here = centre(cells[next]);
      points.push(here);
      at = next;
    }
    // The corners rounded, where there's room: each cut a quarter of the way along both its sides
    for (let pass = 0; pass < 2; pass++) {
      for (let i = points.length - 2; i >= (start === cellOf(from) ? 0 : 1); i--) {
        const before = i === 0 ? { x: from.x, y: from.y } : points[i - 1], corner = points[i], after = points[i + 1];
        const a = { x: corner.x + (before.x - corner.x) * 0.25, y: corner.y + (before.y - corner.y) * 0.25 };
        const b = { x: corner.x + (after.x - corner.x) * 0.25, y: corner.y + (after.y - corner.y) * 0.25 };
        if (Math.hypot(a.x - b.x, a.y - b.y) > 1.5 && clear(before, a) && clear(a, b) && clear(b, after)) points.splice(i, 1, a, b);
      }
    }
    points.push({ x: to.x, y: to.y });
    return points;
  }

  /**
   * One step along `path` for a walker at `position`, `stride` far: what's
   * left over at a turn carries on round it, so the pace holds. Returns
   * where it got to and the way it was heading, and shortens `path`.
   */
  function stepAlong(position, path, stride) {
    let { x, y } = position, heading = { x: 0, y: 0 };
    while (path.length && stride > 0) {
      const to = path[0], dx = to.x - x, dy = to.y - y, distance = Math.hypot(dx, dy);
      if (distance > stride) {
        x += (dx / distance) * stride;
        y += (dy / distance) * stride;
        heading = { x: dx, y: dy };
        stride = 0;
      } else {
        x = to.x;
        y = to.y;
        if (distance > 0.01) heading = { x: dx, y: dy };
        stride -= distance;
        path.shift();
      }
    }
    return { position: { x, y }, heading };
  }
  /** A flier's way: a gentle arc up and over from `from` to `to`, as points. */
  function flightOf(from, to) {
    const d = Math.hypot(to.x - from.x, to.y - from.y), lift = Math.min(28, d * 0.35);
    const control = { x: (from.x + to.x) / 2, y: Math.min(from.y, to.y) - lift };
    const points = [];
    const count = Math.max(2, Math.ceil(d / 8));
    for (let k = 1; k <= count; k++) {
      const s = k / count;
      points.push({ x: (1 - s) ** 2 * from.x + 2 * (1 - s) * s * control.x + s * s * to.x, y: Math.max(1, (1 - s) ** 2 * from.y + 2 * (1 - s) * s * control.y + s * s * to.y) });
    }
    return points;
  }

  /** A*, from cell to cell, straight and diagonally but never round a corner: the cells on the way, or null with none. */
  function search(start, goal, columns, rows, taken, clearance = null) {
    if (start === goal) return [start];
    const cost = new Float32Array(columns * rows).fill(Infinity);
    const from = new Int32Array(columns * rows).fill(-1);
    const done = new Uint8Array(columns * rows);
    const goalColumn = goal % columns, goalRow = Math.floor(goal / columns);
    const guess = (cell) => {
      const dx = Math.abs((cell % columns) - goalColumn), dy = Math.abs(Math.floor(cell / columns) - goalRow);
      // A hair over the distance, so of ways equally short the one heading for the goal is tried first
      return (Math.max(dx, dy) + (Math.SQRT2 - 1) * Math.min(dx, dy)) * 1.001;
    };
    // A binary heap of [estimate, cell]
    const heap = [];
    const push = (item) => {
      heap.push(item);
      for (let i = heap.length - 1; i > 0;) {
        const parent = (i - 1) >> 1;
        if (heap[parent][0] <= heap[i][0]) break;
        [heap[parent], heap[i]] = [heap[i], heap[parent]];
        i = parent;
      }
    };
    const pop = () => {
      const top = heap[0], last = heap.pop();
      if (heap.length) {
        heap[0] = last;
        for (let i = 0; ;) {
          const l = i * 2 + 1, r = l + 1;
          let least = i;
          if (l < heap.length && heap[l][0] < heap[least][0]) least = l;
          if (r < heap.length && heap[r][0] < heap[least][0]) least = r;
          if (least === i) break;
          [heap[least], heap[i]] = [heap[i], heap[least]];
          i = least;
        }
      }
      return top;
    };
    cost[start] = 0;
    push([guess(start), start]);
    while (heap.length) {
      const [, cell] = pop();
      if (done[cell]) continue;
      if (cell === goal) {
        const cells = [];
        for (let at = goal; at !== -1; at = from[at]) cells.push(at);
        return cells.reverse();
      }
      done[cell] = 1;
      const column = cell % columns, row = Math.floor(cell / columns);
      for (let dy = -1; dy <= 1; dy++) {
        for (let dx = -1; dx <= 1; dx++) {
          if (!dx && !dy) continue;
          const c = column + dx, r = row + dy;
          if (c < 0 || r < 0 || c >= columns || r >= rows) continue;
          const next = r * columns + c;
          if (taken[next] || done[next]) continue;
          if (dx && dy && (taken[row * columns + c] || taken[r * columns + column])) continue; // no cutting corners
          const through = cost[cell] + (dx && dy ? Math.SQRT2 : 1) + (clearance ? EDGE_COST[clearance[next]] || 0 : 0);
          if (through >= cost[next]) continue;
          cost[next] = through;
          from[next] = cell;
          push([through + guess(next), next]);
        }
      }
    }
    return null;
  }

  /* ---------------------------------------------------------- the palette */

  const PALETTE = {
    wall: 0x2f3446, wallTop: 0x252a3a, loungeWall: 0x353447, wainscot: 0x272b3b, rail: 0x3d4359,
    baseboard: 0x1b1e2a, frame: 0x1b1e2a, sill: 0x8a90a6,
    wood: 0x6f5440, plankSeam: 0x624a38, carpet: 0x3f4a5c, carpetEdge: 0x353e4e, rug: 0x3f6b6b, rugEdge: 0x4f8080,
    deskEdge: 0xdcb78c, deskTop: 0xc49a6c, deskFront: 0x8c6744, handle: 0x3a2a1c,
    monitor: 0x262a35, monitorPanel: 0x2f3442, monitorStand: 0x3d4250, monitorOn: 0x4a5266,
    keyboard: 0x4a5060, keys: 0x5f6678,
    paper: 0xeae6dc, mug: 0xf1ede4, leaf: 0x4f9a5a, leafDark: 0x3b7a46, pot: 0xa0603e,
    door: 0x6b4e37, doorPanel: 0x5c422e, doorway: 0x0d0f14, brass: 0xd6b25e, mat: 0x7a3b3b,
    whiteboard: 0xe9edf2, cork: 0xb08a5a,
    counterTop: 0xc9c3b8, counter: 0x6b4e37, machine: 0x2a2a2e, water: 0x8fc6f0, cooler: 0xe2e2e2,
    couch: 0x7a4a5a, couchLight: 0x8c5868, couchDark: 0x633a49, table: 0x2f7a5b, tableLine: 0xe6efe9, tableLeg: 0x23272f,
    pants: 0x2e3550, shoes: 0x1a1a1f, eye: 0x1f1a1a, bubble: 0xf4f1ea, bubbleEdge: 0x1b1e2a, smoke: 0x9a9aa0, night: 0x0a1030,
    hazard: 0xf2c200, dome: 0xd4140b, domeLight: 0xff7a6b, domeDark: 0x7d0602, alarm: 0xff2a1a, alarmOff: 0x5a1a1a,
    shockwave: 0xffb04a,
    terminal: 0x3ddc84, sea: 0x3f7fc0, sweat: 0x8fc6f0, marker: 0x3d4359, shredder: 0x4a4e5a, shredderTop: 0x5f6678,
  };
  const BLACK = 0x000000, WHITE = 0xffffff;
  const MEGAPHONE = 0xe8e4da;

  /** The wall's paint: the wall, its top edge, the panelling below the rail, the rail, and the lounge's own wall. */
  const PAINTS = {
    slate: { wall: 0x2f3446, top: 0x252a3a, wainscot: 0x272b3b, rail: 0x3d4359, lounge: 0x353447 },
    sage: { wall: 0x34463d, top: 0x2a3931, wainscot: 0x2b3a32, rail: 0x485c50, lounge: 0x3b4a43 },
    clay: { wall: 0x4b3631, top: 0x3e2c28, wainscot: 0x3c2b27, rail: 0x60463f, lounge: 0x51393b },
    cream: { wall: 0xb5ad98, top: 0xa39b86, wainscot: 0x8f8773, rail: 0xcac2ad, lounge: 0xc0b6a0 },
    // A wall with a finish of its own, drawn in drawOffice's WALL_FINISHES: its colours, for above the room and the setup page
    hull: { wall: 0x3a404c, top: 0x2a2f39, wainscot: 0x2a2f39, rail: 0x6a7280, lounge: 0x3a404c },
    timber: { wall: 0x5a3b25, top: 0x3a2616, wainscot: 0x3a2616, rail: 0xc9a06a, lounge: 0x5a3b25 },
    damask: { wall: 0x6a1e2a, top: 0x4e1620, wainscot: 0x4a2618, rail: 0xd6b25e, lounge: 0x6a1e2a },
    stone: { wall: 0x4d5056, top: 0x26282c, wainscot: 0x34363b, rail: 0x6a6d73, lounge: 0x4d5056 },
    ice: { wall: 0xb4d0e2, top: 0x86aac0, wainscot: 0xe8f0f6, rail: 0xdcebf4, lounge: 0xb4d0e2 },
    logs: { wall: 0x7a5232, top: 0x2a1d16, wainscot: 0x5a3b25, rail: 0x9a6a42, lounge: 0x7a5232 },
    brick: { wall: 0x8a3a2a, top: 0x3a2e2a, wainscot: 0x4a5060, rail: 0x5a6070, lounge: 0x8a3a2a },
    bamboo: { wall: 0xa8a050, top: 0x2a2a18, wainscot: 0x8a8440, rail: 0xb8b05a, lounge: 0xa8a050 },
    block: { wall: 0x9aa0a8, top: 0x6a7078, wainscot: 0x2a2c30, rail: 0x3a3d42, lounge: 0x9aa0a8 },
    foam: { wall: 0x16171c, top: 0x0e0f13, wainscot: 0x16171c, rail: 0x24262e, lounge: 0x16171c },
  };
  /** The lounge's carpet, and its edge along the office's floor. */
  const LOUNGES = {
    navy: [0x3f4a5c, 0x353e4e], teal: [0x2f5656, 0x284a4a], plum: [0x513d50, 0x453444], sand: [0x7b6e58, 0x6a5f4c],
  };
  /** The deck's text colour, for a theme that leaves it out: it marks the picked desk. */
  const INK = 0xececec;
  /** The edge of the deck's keys, for a theme that leaves it out: it rims the window onto the room. */
  const EDGE = 0x2c2c2c;
  const SHADOW = [BLACK, 0.25];

  /** A desk's paint for each effort level, from white to the ☢ dome's red. */
  const EFFORT_PAINT = { low: 0xf2efe9, medium: 0xf3cdb8, high: 0xee9f82, xhigh: 0xe0603f, max: 0xd4140b };
  /** Each agent wears its own colour. */
  const SHIRTS = { claude: 0xd97757, codex: 0x10a37f, gemini: 0x4c8bf5, opencode: 0x9aa0a6, grok: 0xe8e8e8 };
  /**
   * Each outfit, and what goes with it: jeans and trainers with a T-shirt, a
   * dark suit and an orange tie, denim overalls, a tracksuit's bottoms to
   * match its top, a white shirt's sleeves under a waistcoat.
   */
  const DENIM = 0x46679a, TRAINERS = 0xe9e6df;
  const OUTFITS = {
    tee: { style: 'tee', pants: DENIM, shoes: TRAINERS },
    hoodie: { style: 'hoodie' },
    collar: { style: 'collar' },
    sweater: { style: 'sweater' },
    jacket: { style: 'jacket' },
    suit: { style: 'suit', shirt: 0x2c2f3a, pants: 0x23252d, tie: 0xd97757 },
    labcoat: { style: 'labcoat', shirt: 0xeeeae2 },
    polo: { style: 'polo' },
    cardigan: { style: 'cardigan' },
    hawaiian: { style: 'hawaiian', shoes: 0x8a6a4a },
    tracksuit: { style: 'tracksuit', matching: true, shoes: TRAINERS },
    overalls: { style: 'overalls', pants: DENIM },
    waistcoat: { style: 'waistcoat', sleeves: 0xeeeae2 },
    // The styles' clothes: `tint` is the shirt as a shade of the agent's colour, `robe` reaches the floor,
    // `sleeves` and `legs` can be bare, the 'skin'. Drawn in drawOffice's OUTFIT_PARTS.
    astronaut: { style: 'astronaut', shirt: 0xe8e8ec, pants: 0xd8d8de, shoes: 0x6a7080, sleeves: 0xffffff },
    flightsuit: { style: 'flightsuit', shirt: 0xe0703a, pants: 0xc0602e, shoes: 0x3a3d47 },
    pirate: { style: 'pirate', shirt: 0xf4f1ea, pants: 0x3a2616, shoes: 0x1a1a1f, sleeves: 0xffffff },
    captain: { style: 'captain', tint: 0.6, robe: true, shoes: 0x1a1a1f },
    parka: { style: 'parka', tint: 1, pants: 0x3a3f4a, shoes: 0x5a3b25 },
    explorer: { style: 'explorer', shirt: 0xe0703a, pants: 0x3a3f4a, shoes: 0x2a2c30 },
    robe: { style: 'robe', tint: 0.7, robe: true, shoes: 0x2a1d16 },
    knight: { style: 'knight', shirt: 0x9aa0a6, pants: 0x8a8f9c, shoes: 0x5a5f6c, sleeves: 0xb0b5bc },
    wizard: { style: 'wizard', tint: 0.45, robe: true, shoes: 0x2a1d16 },
    gamer: { style: 'gamer', shirt: 0x1d1f25, pants: 0x2a2d35, shoes: TRAINERS },
    founder: { style: 'founder', shirt: 0x6a7080, pants: 0x3a4a6a, shoes: TRAINERS, sleeves: 0xd8f0ff },
    gymkit: { style: 'gymkit', tint: 1, pants: 0x2a2c30, shoes: TRAINERS, sleeves: 'skin', legs: 'skin' },
    safari: { style: 'safari', shirt: 0xc9b07a, pants: 0x8a7a5a, shoes: 0x5a3b25 },
    flannel: { style: 'flannel', tint: 1, pants: DENIM, shoes: 0x5a3b25 },
    // The hire's: a blue suit, its trousers too, a paper collar and an orange tie
    hire: { style: 'hire', shirt: 0x3b82f6, pants: 0x3b82f6, shoes: 0x1e2433, tie: 0xff8a1f },
    // The models' looks, one for each family
    claude: { style: 'claude', shirt: 0xd97757 },
    gpt: { style: 'gpt', shirt: 0x2a2d35 },
    gemini: { style: 'gemini', shirt: 0x6a7ae8 },
    grok: { style: 'grok', shirt: 0x1a1a1f },
    qwen: { style: 'qwen', shirt: 0x6a4ac8 },
    llama: { style: 'llama', shirt: 0xe8dcc0 },
    mistral: { style: 'mistral', shirt: 0xf4f1ea, sleeves: 0xffffff },
    deepseek: { style: 'deepseek', shirt: 0x1f3a7a },
    kimi: { style: 'kimi', shirt: 0x23252c },
    glm: { style: 'glm', shirt: 0x2f4a9a },
    phi: { style: 'phi', shirt: 0x2f9a92 },
  };
  /**
   * What an agent left on Auto wears for the workspace they're at, in their
   * agent's colour, whatever the room: a desk of its own kind gives its
   * clothes, and any other the agent's own look. The office's staff, who
   * have no desk, wear the second of what most desks give, where there is
   * one: launch suits among astronauts, captains among pirates.
   */
  const WORKSPACE_DRESS = {
    cockpit: 'astronaut', console: 'astronaut', hover: 'astronaut', ship: 'pirate', chest: 'pirate', throne: 'robe', gilded: 'robe',
    igloo: 'parka', crates: 'parka', greenhouse: 'wizard', altar: 'wizard', tent: 'flannel', log: 'flannel', van: 'flannel',
    box: 'founder', bench: 'founder', bamboo: 'safari', rack: 'gymkit', gaming: 'gamer',
  };
  const STAFF_DRESS = { astronaut: 'flightsuit', pirate: 'captain', parka: 'explorer', robe: 'knight' };
  /**
   * Each family of models by the words in its models' names, as their own
   * words or with a version after them (`qwen3`, `llama4`), and the family
   * of an agent that says nothing of its model.
   */
  const FAMILY_WORDS = {
    claude: ['claude', 'opus', 'sonnet', 'haiku', 'fable'], gpt: ['gpt', 'codex', 'o1', 'o3', 'o4'], gemini: ['gemini'], grok: ['grok'],
    qwen: ['qwen'], llama: ['llama'], mistral: ['mistral', 'codestral', 'devstral', 'magistral'], deepseek: ['deepseek'],
    kimi: ['kimi', 'moonshot'], glm: ['glm', 'zai'], phi: ['phi'],
  };
  const AGENT_FAMILIES = { claude: 'claude', codex: 'gpt', gemini: 'gemini', grok: 'grok' };
  /** A model's look a shade lighter for a small one, darker for a big one. */
  const FAMILY_SHADES = { small: 1.15, medium: 1, large: 0.85, top: 0.72 };

  /** The family of the model an agent runs, by its name, or by the agent's when it gives none. */
  function familyOf(model, agent) {
    if (!model) return AGENT_FAMILIES[agent] ?? null;
    const words = model.toLowerCase().split(/[^a-z0-9]+/);
    const says = (name) => words.some((word) => word === name || (word.startsWith(name) && /^[0-9]/.test(word.slice(name.length))));
    return Object.keys(FAMILY_WORDS).find((family) => FAMILY_WORDS[family].some(says)) ?? null;
  }

  /** An outfit as worn, its shirt a shade of `accent` for clothes that take the wearer's colour. */
  function dressFor(style, accent) {
    const outfit = { style: 'plain', shirt: accent, pants: PALETTE.pants, shoes: PALETTE.shoes, accent, ...(style ? OUTFITS[style] : {}) };
    if (outfit.tint !== undefined) outfit.shirt = shaded(accent, outfit.tint);
    if (outfit.robe) outfit.pants = outfit.shirt;
    return outfit;
  }

  /**
   * The bigger the model the smarter, for a model of no family the office
   * knows: small models in a T-shirt and jeans, medium in a collared shirt,
   * large in a jacket, the top ones in a suit and tie.
   */
  const SIZE_OUTFITS = { small: 'tee', medium: 'collar', large: 'jacket', top: 'suit' };
  /**
   * Words model names use for their size, as their own words: so `gemini`
   * isn't `mini`. A model with none of them is medium.
   */
  const SIZE_WORDS = {
    top: ['fable', 'ultra', 'heavy', 'max'],
    small: ['haiku', 'mini', 'nano', 'lite', 'fast', 'small'],
    large: ['opus', 'pro', 'large'],
    medium: ['sonnet', 'flash', 'medium'],
  };

  /** How big a model is, by its name: your own rules first, then the words in it. */
  function sizeOf(model) {
    const name = model.toLowerCase();
    const rule = decor.dress.rules.find((r) => name.includes(r.contains));
    if (rule) return rule.size;
    if (/deep-?think/.test(name)) return 'top';
    const words = name.split(/[^a-z0-9]+/);
    return ['top', 'small', 'large', 'medium'].find((size) => SIZE_WORDS[size].some((word) => words.includes(word))) || 'medium';
  }

  /**
   * What an agent running `model` wears: its project's own outfit on the team
   * card first, then the dress code for the model's size, or for the agent
   * while its model isn't known. Left on Auto, at a `workspace` of its own
   * kind they wear its clothes in their agent's colour (see
   * WORKSPACE_DRESS), and otherwise their model's family's look, a shade
   * lighter or darker for its size, or for a model of no family the office
   * knows, clothes for its size in their agent's colour.
   */
  function outfitOf(agent, model, member, workspace = null) {
    const own = SHIRTS[agent] ?? SHIRTS.claude;
    const size = model ? sizeOf(model) : null;
    const code = (size ? decor.dress.sizes[size] : decor.dress.agents[agent]) || AUTO_ATTIRE;
    const colour = member && member.colour !== 'auto' ? member.colour : code.colour !== 'auto' ? code.colour : null;
    const accent = colour ? SHIRT_COLOURS[colour] : own;
    const chosen = member && member.outfit !== 'auto' ? member.outfit : code.outfit !== 'auto' ? code.outfit : null;
    const styled = chosen ? null : WORKSPACE_DRESS[workspace] ?? null;
    const family = chosen || styled ? null : familyOf(model, agent);
    const style = chosen || styled || family || (size ? SIZE_OUTFITS[size] : null);
    const outfit = dressFor(style, accent);
    if (family) {
      outfit.shade = FAMILY_SHADES[size || 'medium'];
      outfit.shirt = shaded(colour ? accent : outfit.shirt, outfit.shade);
    } else if (colour && outfit.tint === undefined) {
      outfit.shirt = accent;
    }
    // A suit's tie is in its agent's colour.
    if (style === 'suit' && agent && agent !== 'claude') outfit.tie = own;
    if (outfit.matching) outfit.pants = darker(outfit.shirt);
    return outfit;
  }

  /** What one of the office's staff wears: plain clothes, or `dress`, what most desks give (see WORKSPACE_DRESS), in their colour. */
  function staffOutfitOf(look, dress) {
    const shirt = STAFF_SHIRTS[Math.floor(look.phase * STAFF_SHIRTS.length) % STAFF_SHIRTS.length];
    return dress ? dressFor(STAFF_DRESS[dress] ?? dress, shirt) : { style: 'plain', shirt, pants: PALETTE.pants, shoes: PALETTE.shoes };
  }
  /** Staff wear plain colours, none of them an agent's. */
  const STAFF_SHIRTS = [0x8a8f9c, 0x3e4a66, 0xd8d4c8, 0xb08a3a, 0x6b7f5a];
  /** The deck's own colours for each state, for a theme that leaves one out. */
  const STANDARD_STATES = { idle: '#f2f2f2', thinking: '#3b82f6', asking: '#ff8a1f', unread: '#22c55e', error: '#ef4444' };

  /** A colour for the canvas: `hex`, darkened by `shade`, at `alpha`. */
  function css(hex, alpha = 1, shade = 1) {
    const r = Math.round(((hex >> 16) & 0xff) * shade);
    const g = Math.round(((hex >> 8) & 0xff) * shade);
    const b = Math.round((hex & 0xff) * shade);
    return alpha >= 1 ? `rgb(${r},${g},${b})` : `rgba(${r},${g},${b},${Math.max(0, alpha)})`;
  }

  /** A colour at six tenths of its brightness, for the side of something lit from above. */
  function darker(hex) {
    const part = (shift) => Math.round(((hex >> shift) & 0xff) * 0.6) << shift;
    return part(16) | part(8) | part(0);
  }

  /** A colour a third of the way to white, for a knitted band or a highlight. */
  function lighter(hex) {
    const part = (shift) => Math.round((((hex >> shift) & 0xff) * 2 + 255) / 3) << shift;
    return part(16) | part(8) | part(0);
  }

  /** A colour at `shade` of its brightness, as a number: below one darker, above it lighter, as far as white. */
  function shaded(hex, shade) {
    const part = (shift) => Math.min(255, Math.round(((hex >> shift) & 0xff) * shade)) << shift;
    return part(16) | part(8) | part(0);
  }

  /** A bright colour round the colour wheel, `turn` of the way from red, for lights that cycle. */
  function rainbow(turn) {
    const part = (n) => {
      const k = (n + turn * 12) % 12;
      return Math.round(255 * (0.5 - 0.5 * Math.max(-1, Math.min(k - 3, 9 - k, 1))));
    };
    return (part(0) << 16) | (part(8) << 8) | part(4);
  }

  /** What people wear besides their clothes: dark lenses, headphones, a bow, glasses, a lab coat's shirt and pen. */
  const SHADES = 0x15161a, HEADPHONES = 0x6a7080, BOW = 0xe0507a, GLASS = 0xcfe3ef, LAB_SHIRT = 0x8fb3d9, LAB_PEN = 0x3f6fc4;
  /** A Hawaiian shirt's yellow flowers, and the overalls' buttons. */
  const HIBISCUS = 0xf2d35b;

  /** A colour as the deck's theme writes it, `#rrggbb`, as a number. */
  function fromCss(text) {
    return /^#[0-9a-f]{6}$/i.test(text || '') ? parseInt(text.slice(1), 16) : null;
  }

  const SKINS = [0xf6d3b3, 0xe8b48a, 0xc98e5f, 0x9b6a43, 0x6e4a2e];
  const HAIRS = [0x2a1d16, 0x4a3022, 0x8a5a2b, 0xd9b26a, 0x1a1a1a, 0x9a9aa0, 0xa8432a];
  const HAIR_STYLES = ['short', 'long', 'spiky', 'bun'];

  /**
   * Your team: each project's agent as you'd like them, by the project's
   * name. Anything left on 'auto' is as the project's name picks it, and
   * clothes on 'auto' are the model's. The apps' Layout.swift and layout.go
   * keep the same choices.
   */
  const TEAM_CHOICES = {
    skin: ['auto', 'porcelain', 'fair', 'light', 'tan', 'olive', 'brown', 'deep', 'ebony', 'cream'],
    hair: ['auto', 'black', 'espresso', 'brown', 'chestnut', 'auburn', 'ginger', 'blond', 'platinum', 'grey', 'white',
      'pink', 'blue', 'green', 'purple', 'orange'],
    hairstyle: ['auto', 'short', 'long', 'spiky', 'bun', 'curly', 'ponytail', 'afro', 'mohawk', 'buzz', 'bald'],
    extra: ['none', 'glasses', 'sunglasses', 'headphones', 'cap', 'beanie', 'beard', 'moustache', 'bow'],
    outfit: ['auto', 'tee', 'hoodie', 'collar', 'sweater', 'jacket', 'suit', 'labcoat', 'polo', 'cardigan', 'hawaiian', 'tracksuit',
      'overalls', 'waistcoat', 'astronaut', 'flightsuit', 'pirate', 'captain', 'parka', 'explorer', 'robe', 'knight', 'wizard', 'gamer',
      'founder', 'gymkit', 'safari', 'flannel', 'hire', 'claude', 'gpt', 'gemini', 'grok', 'qwen', 'llama', 'mistral', 'deepseek', 'kimi', 'glm',
      'phi'],
    colour: ['auto', 'orange', 'red', 'pink', 'purple', 'blue', 'teal', 'green', 'yellow', 'grey', 'black', 'white'],
    desk: ['auto', 'plant', 'papers', 'lamp', 'cactus', 'duck', 'trophy', 'photo', 'mug',
      'helmet', 'rocket', 'terrarium', 'quill', 'candelabra', 'goblet', 'rum', 'compass', 'skull', 'snowglobe', 'thermos', 'penguin',
      'potions', 'egg', 'spellbook', 'controller', 'energy', 'headset', 'circuit', 'pizza', 'coffeepot', 'dumbbell', 'shaker',
      'kettlebell', 'coconut', 'pineapple', 'tikimug', 'enamel', 'lantern', 'pinecone', 'none'],
    workspace: ['auto', ...ROOM_CHOICES.workspace],
  };
  /** The most a nickname can be, in characters, and the most projects a team remembers. */
  const NICKNAME_LIMIT = 16;
  const TEAM_LIMIT = 50;
  const SKIN_TONES = {
    porcelain: 0xfbe3cf, fair: 0xf6d3b3, light: 0xe8b48a, tan: 0xd59b6a, olive: 0xb8875a, brown: 0x9b6a43, deep: 0x6e4a2e,
    ebony: 0x4a3020,
    // Paper, as a drawn character's face is
    cream: 0xf4f1ea,
  };
  const HAIR_COLOURS = {
    black: 0x1a1a1a, espresso: 0x2a1d16, brown: 0x4a3022, chestnut: 0x8a5a2b, auburn: 0xa8432a, ginger: 0xd4722f,
    blond: 0xd9b26a, platinum: 0xece0b8, grey: 0x9a9aa0, white: 0xe8e8e8, pink: 0xe878a8, blue: 0x4a7fd8,
    green: 0x4fae6a, purple: 0x8a5ac8, orange: 0xff8a1f,
  };
  const SHIRT_COLOURS = {
    orange: 0xd97757, red: 0xc9443a, pink: 0xe07aa0, purple: 0x7d5ab8, blue: 0x3f6fc4, teal: 0x2f9a92, green: 0x4f9a5a,
    yellow: 0xe8c04a, grey: 0x8a8f9c, black: 0x2c2f3a, white: 0xeeeae2,
  };

  /**
   * A team as given, with anything unknown left out or as 'auto'. It has no
   * prototype, so a project called `constructor` or `__proto__` is only a name.
   */
  function teamOf(given) {
    const team = Object.create(null);
    if (!given || typeof given !== 'object' || Array.isArray(given)) return team;
    // The first projects by name, as many as a team remembers, as the apps keep them.
    const projects = Object.entries(given).filter(([project]) => project).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
    for (const [project, raw] of projects.slice(0, TEAM_LIMIT)) {
      const member = {};
      const parts = raw && typeof raw === 'object' ? raw : {};
      for (const [part, choices] of Object.entries(TEAM_CHOICES)) member[part] = choices.includes(parts[part]) ? parts[part] : choices[0];
      // On one line, as the apps keep it.
      const name = typeof parts.name === 'string' ? parts.name.replace(/[\u0000-\u001f\u007f]/g, '').trim() : '';
      member.name = [...name].slice(0, NICKNAME_LIMIT).join('');
      team[project] = member;
    }
    return team;
  }

  /**
   * How a person looks, the same every time for the same project: the
   * project's name, hashed with FNV-1a, picks the skin, hair and hairstyle.
   * `phase` spreads out blinks and floating things, so no two people move
   * in step.
   */
  function lookOf(seed) {
    let hash = 0xcbf29ce484222325n;
    for (const byte of new TextEncoder().encode(seed)) hash = ((hash ^ BigInt(byte)) * 0x100000001b3n) & 0xffffffffffffffffn;
    return {
      skin: SKINS[Number(hash % 5n)],
      hair: HAIRS[Number((hash / 5n) % 7n)],
      hairStyle: HAIR_STYLES[Number((hash / 35n) % 4n)],
      phase: Number((hash / 140n) % 1000n) / 1000,
    };
  }

  /** A session's place on your team, if you've made one for its project. */
  const memberOf = (session) => (session && Object.hasOwn(decor.team, session.name) && decor.team[session.name]) || null;

  /**
   * What an agent works at: its own, from your team; or else what the desk
   * it's at was given when the room was arranged; or else the office's. An
   * empty desk is its own, or the office's.
   */
  function workspaceOf(session, desk = null) {
    const member = memberOf(session);
    if (member && member.workspace !== 'auto') return member.workspace;
    const own = deskOwn(desk, 'workspace');
    return own !== 'auto' ? own : decor.workspace;
  }

  /** How a person looks: as their project's name picks it, with your choices for them over the top. */
  function lookFor(person) {
    const member = person.isStaff ? null : memberOf(person.session);
    if (!member) return { ...person.look, extra: 'none' };
    return {
      ...person.look,
      skin: SKIN_TONES[member.skin] ?? person.look.skin,
      hair: HAIR_COLOURS[member.hair] ?? person.look.hair,
      hairStyle: member.hairstyle === 'auto' ? person.look.hairStyle : member.hairstyle,
      extra: member.extra,
    };
  }

  const random = (low, high) => low + Math.random() * (high - low);
  /**
   * Floors as drawn, a canvas each, by the floor, where it is and how big:
   * so a floor is drawn square by square once, not every frame. A few are
   * kept for each document the office is drawn in.
   */
  const floorsDrawn = new WeakMap();
  const FLOORS_KEPT = 6;
  /** A number from 0 to 2³² − 1 for a pair of whole numbers, the same every time: to vary a pattern from tile to tile. */
  const hash = (x, y) => {
    let h = (Math.imul(x | 0, 374761393) + Math.imul(y | 0, 668265263)) | 0;
    h = Math.imul(h ^ (h >>> 13), 1274126177);
    return (h ^ (h >>> 16)) >>> 0;
  };
  /** The first multiple of `step`, offset by `off`, at or left of `left`: so a pattern lines up wherever it's cut. */
  const from = (left, step, off = 0) => left - ((((left - off) % step) + step) % step);
  const pick = (list) => list[Math.floor(Math.random() * list.length)];

  /* ----------------------------------------------------------- the people */

  /**
   * What a tool has its agent do: 'read' (leafing through papers), 'shell'
   * (a terminal on screen), 'web' (a globe on the desk), 'todo' (the
   * whiteboard) or 'type'. Each agent names its tools its own way.
   */
  function toolKind(tool) {
    const name = (tool || '').toLowerCase();
    if (!name) return null;
    if (['read', 'notebookread', 'read_file', 'read_many_files', 'grep', 'glob', 'search_file_content'].includes(name)) return 'read';
    if (['bash', 'shell', 'run_shell_command'].includes(name)) return 'shell';
    if (['webfetch', 'web_fetch', 'websearch', 'web_search', 'google_web_search'].includes(name)) return 'web';
    if (name === 'todowrite') return 'todo';
    return 'type';
  }

  /** One of an agent's subagents: a smaller person who comes to help at their desk. */
  class Intern {
    constructor(subagent, session) {
      this.id = subagent.id;
      this.agentId = session.id;
      this.agent = session.agent; // they wear their agent's colour
      this.model = subagent.model || ''; // or dress for their own model, once it's known
      this.look = lookOf(`${subagent.type} ${subagent.id}`);
      this.position = { ...DOOR };
      this.path = [];
      this.place = 'door';
      this.slot = null;       // where round the desk they stand; none while it's crowded
      this.isLeaving = false; // their subagent has stopped
    }

    get isWalking() { return this.path.length > 0; }
  }

  /** One session, as a person in the office, or one of the office's own staff. */
  class Person {
    constructor(session, desk) {
      this.session = session;
      this.desk = desk; // staff have none
      this.look = lookOf(session.name);
      this.place = desk === null ? 'door' : deskPlace(desk);
      this.position = desk === null ? { ...DOOR } : seat(desk);
      this.path = [];          // the points still to walk through
      this.isLeaving = false;  // their session has ended: they're on their way out
      // The effort their desk is painted for. It catches up with the
      // session's when they press the nuclear button.
      this.deskEffort = session.effort;
      // When it last went up, for the coffee that comes with it to drop in
      this.effortRoseAt = null;
      // The last ☢ Max press they've answered, so only a new one sends them.
      this.answeredPress = session.maxPressedAt;
      this.isGoingNuclear = false;
      // Their session has shown max since the press. Until it does, the
      // press is still on its way to the agent, and an old effort means
      // nothing.
      this.sawMax = false;
      this.pressedAt = null;
      this.isFacingAway = false;
      this.nextMove = Infinity; // when an idle person next decides where to go
      this.arrivedAt = 0;
      this.toolSince = 0; // when their session's current tool started
      this.askingSince = null;  // when they started waiting on you, as far as the office has seen
      this.celebratedAt = null; // when they finished a turn of an hour or more
      this.isNew = false;       // just hired, on their way in to sit down for the first time
      this.settledAt = null;    // when they first sat down, hired
    }

    get isWalking() { return this.path.length > 0; }
    get isStaff() { return this.desk === null; }

    /** Settled at their own desk, where its keyboard and mug are theirs. */
    get isAtDesk() {
      return this.desk !== null && !this.isWalking && !this.isLeaving && this.place === deskPlace(this.desk);
    }
  }

  /**
   * The office's pet, as set up: it wanders between places of its own, in
   * straight lines, since it can climb and jump where people can't.
   */
  class Pet {
    constructor(kind) {
      this.kind = kind;
      this.position = { x: DOOR.x, y: DOOR.y + 6 }; // in through the door
      this.target = null;      // where it's going or staying: { x, y, depth, pose, follow, grab }
      this.isWalking = false;
      this.facing = -1;        // right, or left
      this.nextMove = 0;
      this.arrivedAt = 0;
      this.carrying = false;   // a cup the monkey took from the counter
      this.path = [];          // the points still to go through, round what's in its way
      this.rerouted = 0;       // when its way after someone it follows was last worked out
      this.followed = { x: 0, y: 0 }; // where it was making for, behind them, then
      this.waited = 0;         // how long it's waited for someone in its way
      this.isIn = false;       // in the office yet: pets come in through the door one after another
      this.leg = 0;            // how far it set off to go, for the penguin to slide a long way
      this.zoom = null;        // the corgi's laps of the rug: { at, c }
      this.trail = [];         // the snail's, fading: { x, y, at }
    }
  }

  /** Each pet's pace, in art pixels a second, and how long it stays put, in seconds. */
  const PETS = {
    cat: { speed: 20, stay: [8, 20] },
    dog: { speed: 30, stay: [5, 14] },
    lizard: { speed: 6, stay: [15, 40] },
    monkey: { speed: 38, stay: [5, 12] },
    llama: { speed: 14, stay: [10, 24] },
    penguin: { speed: 16, stay: [6, 16] },
    parrot: { speed: 46, stay: [6, 16], flies: true },
    owl: { speed: 34, stay: [20, 50], flies: true },
    capybara: { speed: 10, stay: [20, 45] },
    redpanda: { speed: 22, stay: [8, 18] },
    corgi: { speed: 32, stay: [5, 14] },
    hamster: { speed: 26, stay: [4, 10] },
    robodog: { speed: 24, stay: [8, 18] },
    bat: { speed: 40, stay: [25, 60], flies: true },
    snail: { speed: 2.5, stay: [20, 40] },
    fox: { speed: 28, stay: [10, 24] },
    axolotl: { speed: 0, stay: [999, 999] },
  };
  /** The corgi's laps of the rug, once an agent has finished, in seconds; the snail's trail, how long it shines. */
  const ZOOMIES = 3.2;
  const TRAIL_FOR = 24;
  /**
   * How far, in art pixels, a pet settles from where people come and go or
   * press things: the door, the red button and its queue, the mic, the
   * corridor down from the door; and from anyone standing still, so they
   * don't end up on top of each other.
   */
  const PET_CLEAR = { door: 14, button: 14, mic: 12, corridor: 6, person: 8, pet: 10 };
  /** On foot, a pet waits for anyone this far ahead in its way, up to this long, in seconds, then goes round them. */
  const GIVE_WAY = 9;
  const GIVE_WAY_FOR = 0.8;
  /** The least gap a pet leaves between itself and another, in art pixels. */
  const PET_GAP = 7;
  /** A settled pet turns to watch anyone walking by this close. */
  const PET_WATCHES = 40;
  /** The lizard freezes while anyone is this close, in art pixels. */
  const LIZARD_SHY = 14;
  /** The dog follows someone walking for this long at most, in seconds. */
  const FOLLOW_FOR = 8;

  /**
   * The people in the office and where they're going, moved on a little each
   * frame.
   *
   * The deck says what each session is doing; everything else is the
   * office's own: anyone with work to do goes to their desk, and anyone idle
   * wanders between their desk and the lounge. A press of ☢ Max sends its
   * agent running to the nuclear button. Staff fill the lounge while the
   * desks are quiet, and make way for the agents.
   */
  class Office {
    constructor() {
      this.people = [];
      this.seats = [];
      this.colours = {};
      this.nuclearQueue = []; // everyone going nuclear, in the order they set off; the first presses
      this.detonation = null; // the last press of the button: { desk, at }
      // The people in the first snapshot after connecting are already at
      // work; later ones walk in.
      this.isOpen = false;
      this.lastStep = null;
      this.nextStaffChange = 0;
      this.staffHired = 0; // numbers each new member of staff, for a face of their own
      this.interns = [];
      this.pets = [];
      // When each session started waiting on you, by its id. It outlasts
      // close(), so a room started over after being out of sight still
      // knows who has been kept waiting.
      this.waiting = new Map();
      this.cues = []; // sounds asked for by name since the last frame: see SOUNDS
    }

    /** Asks for one of the office's sounds by name, from `x` across the room, or the middle. */
    cue(name, x = null) {
      this.cues.push({ name, x });
    }

    /**
     * Furnishes the office as the layout says. Anyone at a spot by a corner
     * piece that has gone, or at a whiteboard that has moved, goes elsewhere:
     * an agent to their desk, staff to another spot, or home. Rearranged, the
     * room is laid out again, `now`.
     */
    furnish(given, now) {
      const next = roomOf(given);
      if (JSON.stringify(next) === JSON.stringify(decor)) return;
      const cornerGone = next.corner !== decor.corner;
      // Pieces added or taken out of the lounge move the spots round them: anyone there goes elsewhere.
      const kinds = (room) => JSON.stringify(['wide', 'tall'].map((side) => room.plan.added[side].map((piece) => piece.kind)));
      const piecesChanged = kinds(next) !== kinds(decor);
      // The pets as listed now: those staying stay where they are, a new one comes in at the door, a while after the last
      if (JSON.stringify(next.pets) !== JSON.stringify(decor.pets)) {
        this.pets = next.pets.map((kind, i) => this.pets.find((p) => p.kind === kind) || Object.assign(new Pet(kind), { nextMove: now + i * 2 }));
      }
      const boardGone = JSON.stringify(boardOf(next)) !== JSON.stringify(boardOf(decor));
      const rearranged = JSON.stringify(next.plan) !== JSON.stringify(decor.plan);
      decor = next;
      spots = spotsOf(decor);
      boardSpots = boardSpotsOf(decor);
      stormSpots = stormSpotsOf(decor);
      // Moved furniture takes whoever is at it, or on their way, along with it.
      if (rearranged) this.relayout(now);
      for (const person of this.people) {
        if (person.isLeaving) continue;
        const spot = spotOf(person.place);
        const stranded = ((cornerGone || piecesChanged) && spot !== null && spot >= LOUNGE_SPOTS.length)
          || (boardGone && person.place.startsWith('board:')) || (spot !== null && spots[spot].off)
          || (person.place.startsWith('storm:') && !stormSpots[numberAfter('storm:', person.place)])
          // A desk put away sends its agent to the line by the wall, and one brought back has them sit at it again.
          || (!person.isStaff && person.place.startsWith('desk:') && DESKS[person.desk] && DESKS[person.desk].off)
          || (!person.isStaff && person.place.startsWith('crowd:') && DESKS[person.desk] && !DESKS[person.desk].off);
        if (!stranded) continue;
        if (!person.isStaff) {
          this.send(person, this.home(person));
          continue;
        }
        const free = this.freeSpots(person);
        if (free.length) {
          this.send(person, spotPlace(pick(free)));
        } else {
          person.isLeaving = true;
          this.send(person, 'door');
        }
      }
    }

    /**
     * The room takes another shape. The desks stay where they are; anyone
     * on the way somewhere is there at once, the lounge's places having
     * moved, and the pet starts again from the door.
     */
    reshape(next, now) {
      shape = next;
      spots = spotsOf(decor);
      // Each shape has its back wall hung its own way
      boardSpots = boardSpotsOf(decor);
      stormSpots = stormSpotsOf(decor);
      this.relayout(now);
      this.pets = this.pets.map((pet, i) => Object.assign(new Pet(pet.kind), { nextMove: now + i * 2 }));
    }

    /**
     * The desks and the lounge's places have moved, with the room's shape or
     * its arrangement: anyone on the way somewhere is there at once, and
     * everyone at their place as it is now.
     */
    relayout(now) {
      DESKS = desksOf(decor);
      floorChanged();
      // The pet finds somewhere else at once, its places having moved, or gone
      for (const pet of this.pets) if (pet.kind !== 'axolotl') Object.assign(pet, { path: [], isWalking: false, nextMove: now, zoom: null });
      for (const walker of [...this.people, ...this.interns]) {
        const wasWalking = walker.isWalking;
        walker.path = [];
        // At a whiteboard or a window taken down: an agent to where they belong, staff to a spot free, or home
        if (!pointOf(walker.place)) {
          const free = walker.isStaff && this.freeSpots(walker);
          walker.place = !walker.isStaff ? this.home(walker) : free.length ? spotPlace(pick(free)) : 'door';
          if (walker.place === 'door') walker.isLeaving = true;
        }
        // On their way, they go on from where they are, round the room as it is now; settled, they're at their place as it is now
        if (wasWalking) Object.assign(walker, { path: route(walker.position, walker.place), refuge: null, rounded: null });
        else walker.position = { ...pointOf(walker.place) };
        const spot = spotOf(walker.place);
        if (spot !== null && !wasWalking) walker.isFacingAway = facesAway(spots[spot].pose);
      }
    }

    apply(seats, colours, now) {
      this.seats = seats.slice(0, DESKS.length);
      this.colours = colours || {};

      const current = new Map();
      this.seats.forEach((s, desk) => { if (s.session) current.set(s.session.id, { desk, session: s.session }); });
      for (const id of this.waiting.keys()) if (current.get(id)?.session.state !== 'asking') this.waiting.delete(id);
      for (const [id, { session }] of current) if (session.state === 'asking' && !this.waiting.has(id)) this.waiting.set(id, now);
      // Someone on their way out isn't there any more: a session back on a
      // key in the meantime gets a person of its own.
      const known = new Set(this.people.filter((p) => !p.isLeaving).map((p) => p.session.id));
      const newcomers = new Map([...current].filter(([id]) => !known.has(id)));

      for (const person of this.people) {
        if (person.isLeaving || person.isStaff) continue;
        const placement = current.get(person.session.id);
        const successor = placement ? null : [...newcomers].find(([, n]) => n.desk === person.desk);
        if (placement) {
          this.update(person, placement, now);
        } else if (successor) {
          // A new session at the same desk is the same tab after /clear: the
          // same person, carrying on.
          newcomers.delete(successor[0]);
          person.answeredPress = successor[1].session.maxPressedAt;
          this.update(person, successor[1], now);
        } else {
          this.standDown(person);
          person.isLeaving = true;
          this.send(person, 'door');
        }
      }

      for (const placement of [...newcomers.values()].sort((a, b) => a.desk - b.desk)) {
        const person = new Person(placement.session, placement.desk);
        person.askingSince = this.waiting.get(placement.session.id) ?? null;
        if (this.isOpen) {
          person.position = { ...DOOR };
          person.isNew = true;
          this.send(person, this.home(person));
        } else {
          // Already in: at their desk, or in the line by the wall
          person.place = this.home(person);
          person.position = { ...pointOf(person.place) };
          person.arrivedAt = now;
          person.nextMove = now + random(2, 12);
        }
        this.people.push(person);
      }
      if (!this.isOpen) {
        // Staff already in the lounge, like the agents at their desks.
        for (let i = 0; i < this.staffWanted; i++) {
          const free = this.freeSpots(null);
          if (!free.length) break;
          const spot = pick(free);
          const member = this.hireStaff();
          member.place = spotPlace(spot);
          member.position = { x: spots[spot].x, y: spots[spot].y };
          member.isFacingAway = facesAway(spots[spot].pose);
          member.arrivedAt = now - random(0, 20);
          member.nextMove = now + random(5, 25);
        }
        this.nextStaffChange = now + random(...STAFF_INTERVAL);
      }
      this.hireInterns();
      this.isOpen = true;
    }

    /**
     * Each running subagent gets an intern: new ones walk in through the
     * door (or are there already, in the first snapshot), and those whose
     * subagent has stopped walk out.
     */
    hireInterns() {
      const wanted = new Map();
      for (const s of this.seats) {
        if (s.session) for (const subagent of s.session.subagents || []) wanted.set(subagent.id, { subagent, session: s.session });
      }
      for (const intern of this.interns) {
        const still = wanted.get(intern.id);
        // A subagent's model is known once it has replied.
        if (still && still.subagent.model) intern.model = still.subagent.model;
        if (intern.isLeaving || still) continue;
        intern.isLeaving = true;
        // One that never got in just goes.
        if (intern.slot === null) intern.path = [];
        else intern.path = route(intern.position, 'door');
        intern.slot = null;
        intern.place = 'door';
      }
      const known = new Set(this.interns.map((i) => i.id));
      for (const [id, { subagent, session }] of wanted) {
        if (known.has(id)) continue;
        const intern = new Intern(subagent, session);
        intern.arrives = this.isOpen;
        this.interns.push(intern);
      }
    }

    /** The desk an agent's session is at, or null once it's gone. */
    deskOfSession(id) {
      const desk = this.seats.findIndex((s) => s.session && s.session.id === id);
      return desk < 0 ? null : desk;
    }

    /** Interns take free places round their agent's desk, at most four to a desk; the rest wait their turn. */
    placeInterns(now) {
      for (const intern of this.interns) {
        if (intern.isLeaving) continue;
        const desk = this.deskOfSession(intern.agentId);
        if (intern.slot !== null && intern.desk === desk) continue;
        intern.slot = null;
        const taken = new Set(this.interns.filter((i) => i !== intern && !i.isLeaving && i.desk === desk && i.slot !== null).map((i) => i.slot));
        const slot = INTERN_SLOTS.findIndex((_, i) => !taken.has(i));
        if (desk === null || slot < 0) continue;
        intern.desk = desk;
        intern.slot = slot;
        intern.place = `near:${desk}:${slot}`;
        if (intern.arrives) {
          intern.path = route(intern.position, intern.place);
        } else {
          intern.position = pointOf(intern.place);
          intern.path = [];
        }
        intern.arrives = true;
      }
      for (const intern of this.interns) this.walk(intern, this.elapsed, now);
      this.interns = this.interns.filter((i) => !(i.isLeaving && !i.isWalking));
    }

    /** Interns waiting for room at a desk, beyond the four round it. */
    waitingAt(desk) {
      return this.interns.filter((i) => !i.isLeaving && i.slot === null && this.deskOfSession(i.agentId) === desk).length;
    }

    /** The deck has gone: everyone leaves at once. */
    close() {
      this.people = [];
      this.seats = [];
      this.nuclearQueue = [];
      this.detonation = null;
      this.isOpen = false;
      this.nextStaffChange = 0;
      this.interns = [];
    }

    /** The effort a desk is painted for, if its session has one. */
    deskEffort(desk) {
      const person = this.people.find((p) => p.desk === desk && !p.isLeaving);
      return person && person.deskEffort ? person.deskEffort : null;
    }

    /** When the effort at a desk last went up, if it has. */
    effortRoseAt(desk) {
      const person = this.people.find((p) => p.desk === desk && !p.isLeaving);
      return person ? person.effortRoseAt : null;
    }

    step(now) {
      this.elapsed = Math.min(now - (this.lastStep ?? now), 0.1);
      this.lastStep = now;
      this.changeStaff(now);
      for (const person of this.people) {
        this.decide(person, now);
        this.walk(person, this.elapsed, now);
      }
      this.press(now);
      this.people = this.people.filter((p) => !(p.isLeaving && !p.isWalking));
      this.placeInterns(now);
      for (const pet of this.pets) this.movePet(pet, this.elapsed, now);
    }

    /** The agent who has been working longest, at their desk, if anyone is. */
    hardestWorker() {
      const working = this.people.filter((p) => !p.isStaff && p.isAtDesk && p.session.state === 'thinking' && p.session.turnStartedAt);
      return working.sort((a, b) => a.session.turnStartedAt - b.session.turnStartedAt)[0] || null;
    }

    /** Where the pet could go next: each kind has its own haunts. */
    petPlaces(pet) {
      const places = [];
      const worker = this.hardestWorker();
      const desk = worker && DESKS[worker.desk];
      const taken = (spot) => this.people.some((p) => p.place === spotPlace(spot) && !p.isLeaving);
      /** On top of the corner piece, where there's a top to sit on, drawn after it. */
      const tops = {
        arcade: { x: 271, y: 60 }, vending: { x: 271, y: 58 }, fishtank: { x: 272, y: 60 }, books: { x: 297, y: 57 },
        pinball: { x: 274, y: 51, depth: 90.5 }, jukebox: { x: 274, y: 56, depth: 88.5 }, piano: { x: 268, y: 58 },
        jellyfish: { x: 276, y: 51, depth: 91.5 }, servers: { x: 283, y: 50, depth: 89.5 }, printer: { x: 270, y: 56, depth: 91.5 },
        coffee: { x: 289, y: 63, depth: 93.5 }, bubbler: { x: 274, y: 53, depth: 89 }, neonbox: { x: 274, y: 55, depth: 88.5 },
        hifi: { x: 257, y: 59, depth: 92.5 },
      };
      /** Beside the corner piece, where the cat likes to be: watching the fish, warm by the fire, on the dragon's gold. */
      const watches = {
        fishtank: { x: 262, y: 95, pose: 'sit' }, jellyfish: { x: 262, y: 95, pose: 'sit' },
        campfire: { x: 276, y: 99, pose: 'sleep' }, dragon: { x: 284, y: 97, pose: 'sleep' },
      };
      switch (pet.kind) {
        case 'cat':
          [3, 4, 5].forEach((spot) => { if (!taken(spot)) places.push(placed('couch', { x: LOUNGE_SPOTS[spot].x, y: 113, pose: 'sleep' })); });
          for (const p of this.people) {
            if (p.isStaff || !p.isAtDesk) continue;
            // Where what's on the desk is, or on a desk with nowhere for it, its end.
            const { dx, dy } = WORKSPACES[workspaceOf(p.session, p.desk)].things || { dx: 0, dy: 0 };
            const { x, y } = DESKS[p.desk];
            places.push({ x: x + 36 + dx, y: y + 2 + dy, depth: y + 5.6 + dy, pose: 'sit' });
          }
          if (desk) places.push({ x: desk.x - 5, y: desk.y + 15, pose: 'sit' });
          for (const { part, kind } of loungePieces()) if (watches[kind]) places.push(placed(part, watches[kind]));
          places.push(placed('wall', { x: 252, y: 33, depth: 47, pose: 'sit' }));
          break;
        case 'dog': {
          places.push(placed('couch', { x: 276, y: 138, pose: 'lie' }), { x: CORRIDOR_X, y: 72, pose: 'sit' }, placed('corner', { x: 290, y: 62, pose: 'sit' }));
          if (desk) places.push({ x: desk.x + DESK_WIDTH + 6, y: desk.y + 15, pose: 'lie' });
          const someone = this.people.find((p) => p.isWalking && !p.isLeaving && !this.pets.some((o) => o !== pet && o.target && o.target.follow === p));
          if (someone) places.push({ follow: someone, pose: 'stand' });
          break;
        }
        case 'lizard':
          hungOf().forEach((piece) => { if (piece.kind === 'window') places.push({ x: piece.x + 19, y: 22, depth: 41.5, pose: 'still' }); });
          places.push(placed('wall', { x: 296, y: 24, depth: 41.5, pose: 'still' }), placed('corner', { x: 276, y: 58, pose: 'still' }));
          if (desk) {
            const { screen } = WORKSPACES[workspaceOf(worker.session, worker.desk)];
            places.push({ x: desk.x + screen.dx + 6, y: desk.y + screen.dy, depth: desk.y + 5.6, pose: 'still' });
          }
          break;
        case 'monkey':
          places.push(placed('wall', { x: 308, y: 18, depth: 50.5, pose: 'sit' }), { x: CORRIDOR_X, y: 13, depth: 41.5, pose: 'sit' },
            placed('couch', { x: 270, y: 102, depth: 110.5, pose: 'sit' }), placed('wall', { x: 247, y: 33, depth: 47, pose: 'sit', grab: true }));
          for (const { part, kind } of loungePieces()) if (tops[kind]) places.push(placed(part, { depth: 86.5, ...tops[kind], pose: 'sit' }));
          break;
      }
      const seatOf = (person) => seat(person.desk);
      // Someone walking about that no other pet is following already
      const walker = this.people.find((p) => p.isWalking && !p.isLeaving && !this.pets.some((o) => o !== pet && o.target && o.target.follow === p));
      switch (pet.kind) {
        case 'llama':
          // Chewing by the plant, by the cooler or a desk; lying down on the rug
          places.push(placed('plant', { x: 243, y: 170, pose: 'chew' }), placed('wall', { x: 290, y: 57, pose: 'chew' }),
            placed('couch', { x: 290, y: 141, pose: 'lie' }), { x: CORRIDOR_X, y: 92, pose: 'stand' });
          if (desk) places.push({ x: desk.x + DESK_WIDTH + 8, y: desk.y + 16, pose: 'chew' });
          break;
        case 'penguin':
          places.push(placed('wall', { x: 296, y: 57, pose: 'stand' }), placed('wall', { x: 270, y: 54, pose: 'stand' }),
            placed('couch', { x: 262, y: 141, pose: 'stand' }), { x: CORRIDOR_X, y: 120, pose: 'stand' });
          if (desk) places.push({ x: desk.x - 5, y: desk.y + 16, pose: 'stand' });
          break;
        case 'parrot':
          // On the busiest agent's shoulder, on a monitor, on the windows' tops, on the couch's back
          if (worker) places.push({ x: seatOf(worker).x + 4, y: desk.y - 3, depth: desk.y + 5.8, pose: 'perch' },
            { x: desk.x + 12, y: desk.y - 10, depth: desk.y + 5.7, pose: 'perch' });
          hungOf().forEach((piece) => { if (STORM_PANES[piece.kind]) places.push({ x: piece.x, y: 5, depth: 41.5, pose: 'perch' }); });
          places.push(placed('couch', { x: 300, y: 103, depth: 110.5, pose: 'perch' }));
          for (const { part, kind } of loungePieces()) if (tops[kind]) places.push(placed(part, { depth: 86.5, ...tops[kind], pose: 'perch' }));
          break;
        case 'owl':
          hungOf().forEach((piece) => places.push({ x: piece.x, y: 5, depth: 41.5, pose: 'perch' }));
          places.push(placed('wall', { x: 308, y: 18, depth: 50.5, pose: 'perch' }));
          for (const { part, kind } of loungePieces()) if (tops[kind]) places.push(placed(part, { depth: 86.5, ...tops[kind], pose: 'perch' }));
          break;
        case 'capybara':
          places.push(placed('couch', { x: 270, y: 140, pose: 'sit' }), placed('wall', { x: 286, y: 58, pose: 'sit' }),
            placed('plant', { x: 245, y: 171, pose: 'sit' }));
          for (const { part, kind } of loungePieces()) if (kind === 'hottub') places.push(placed(part, { x: 270, y: 82, depth: 88.5, pose: 'soak' }));
          break;
        case 'redpanda':
          places.push(placed('couch', { x: 286, y: 104, depth: 110.5, pose: 'sit' }), placed('plant', { x: 231, y: 157, depth: 178, pose: 'sit' }),
            { x: CORRIDOR_X, y: 80, pose: 'sit' });
          for (const { part, kind } of loungePieces()) if (tops[kind]) places.push(placed(part, { depth: 86.5, ...tops[kind], pose: 'sit' }));
          if (desk) places.push({ x: desk.x - 5, y: desk.y + 16, pose: 'sit' });
          break;
        case 'corgi':
          places.push(placed('couch', { x: 276, y: 140, pose: 'lie' }), { x: CORRIDOR_X, y: 72, pose: 'sit' }, placed('wall', { x: 284, y: 58, pose: 'sit' }));
          if (desk) places.push({ x: desk.x + DESK_WIDTH + 6, y: desk.y + 15, pose: 'lie' });
          if (walker) places.push({ follow: walker, pose: 'stand' });
          break;
        case 'hamster':
          // Rolling about the floor: between the desks, down the corridor, round the lounge
          DESKS.forEach((d) => { if (!d.off) places.push({ x: d.x + DESK_WIDTH + 11, y: d.y + 18, pose: 'sit' }); });
          places.push({ x: CORRIDOR_X, y: 110, pose: 'sit' }, placed('couch', { x: 250, y: 141, pose: 'sit' }), placed('pong', { x: 268, y: 168, pose: 'sit' }));
          break;
        case 'robodog':
          places.push(placed('wall', { x: 234, y: 57, pose: 'charge' }), { x: CORRIDOR_X, y: 72, pose: 'sit' }, placed('couch', { x: 296, y: 141, pose: 'sit' }));
          if (desk) places.push({ x: desk.x + DESK_WIDTH + 6, y: desk.y + 15, pose: 'sit' });
          if (walker) places.push({ follow: walker, pose: 'stand' });
          break;
        case 'bat':
          // Hanging from the ceiling, over the desks and in the lounge
          for (const x of [44, 110, 176]) places.push({ x, y: 1, depth: 41.5, pose: 'hang' });
          places.push(placed('wall', { x: 262, y: 1, depth: 41.5, pose: 'hang' }), placed('wall', { x: 300, y: 1, depth: 41.5, pose: 'hang' }));
          break;
        case 'snail':
          hungOf().forEach((piece) => { if (STORM_PANES[piece.kind]) places.push({ x: piece.x + 6, y: 22, depth: 41.5, pose: 'still' }); });
          places.push(placed('plant', { x: 236, y: 172, pose: 'still' }), placed('wall', { x: 300, y: 58, pose: 'still' }));
          break;
        case 'fox':
          places.push(placed('couch', { x: 300, y: 141, pose: 'curl' }), { x: CORRIDOR_X, y: 100, pose: 'sit' });
          for (const { part, kind } of loungePieces()) if (watches[kind]) places.push(placed(part, { ...watches[kind], pose: 'curl' }));
          if (desk) places.push({ x: desk.x - 6, y: desk.y + 17, pose: 'curl' });
          break;
      }
      // Nowhere on a piece that's been put away, nor on the floor where something stands or someone needs to get by.
      return places.filter((place) => !place.off && this.petCanSettle(place, pet));
    }

    /**
     * Whether the pet can settle at `place`: up on something (it has a
     * depth) wherever that is now; on the floor only where it's free, out of
     * the way of the door, the button, the mic and the corridor, and not on
     * top of anyone.
     */
    petCanSettle(place, pet) {
      if (place.follow) return true;
      // Not where another pet is, or is going
      const others = this.pets.filter((other) => other !== pet && other.isIn);
      if (others.some((other) => [other.position, other.target].some((p) => p && !p.follow && Math.hypot(p.x - place.x, p.y - place.y) < PET_CLEAR.pet))) return false;
      if (place.depth !== undefined) return true;
      if (place.x < 2 || place.x > shape.width - 2 || place.y > shape.height - 2 || !standsFree(place)) return false;
      const near = (point, reach) => point && Math.hypot(point.x - place.x, point.y - place.y) < reach;
      if (near(DOOR, PET_CLEAR.door) || near(placed('button', PRESSER), PET_CLEAR.button) || near(placed('button', NEXT_IN_LINE), PET_CLEAR.button)
        || near(placed('mic', MIC), PET_CLEAR.mic)) return false;
      if (Math.abs(place.x - CORRIDOR_X) < PET_CLEAR.corridor || this.inSomeonesWay(place, PET_CLEAR.person)) return false;
      return !this.people.some((p) => !p.isWalking && !p.isLeaving && near(p.position, PET_CLEAR.person));
    }

    /** Whether anyone walking is about to pass within `reach` of `point`, on the next few legs of their way. */
    inSomeonesWay(point, reach) {
      const near = (a, b) => {
        const dx = b.x - a.x, dy = b.y - a.y, len = dx * dx + dy * dy;
        const s = len ? Math.max(0, Math.min(1, ((point.x - a.x) * dx + (point.y - a.y) * dy) / len)) : 0;
        return Math.hypot(a.x + dx * s - point.x, a.y + dy * s - point.y) < reach;
      };
      return this.people.some((p) => {
        if (!p.isWalking) return false;
        let from = p.position;
        for (const to of p.path.slice(0, 4)) {
          if (near(from, to)) return true;
          from = to;
        }
        return false;
      });
    }

    /**
     * The way to `place`: flying, straight there; on foot, round what's in
     * its way as people go, to the floor below where it's going, then a
     * hop or a climb up onto it.
     */
    petRoute(pet, place) {
      if (PETS[pet.kind].flies) return flightOf(pet.position, place);
      const up = place.depth !== undefined && !standsFree(place);
      const ground = up ? { x: place.x, y: Math.min(shape.height - 2, place.depth + 3) } : place;
      const settled = this.pets.filter((o) => o !== pet && o.isIn && !o.isWalking && !(o.target && o.target.depth !== undefined)).map((o) => o.position);
      // Up on something, it gets down first, straight down to the floor in front of it
      const down = !standsFree(pet.position) && pet.target && pet.target.depth !== undefined
        ? { x: pet.position.x, y: Math.min(shape.height - 2, pet.target.depth + 3) } : null;
      const way = down ? [down, ...walkway(down, ground, false, settled)] : walkway(pet.position, ground, false, settled);
      if (up) way.push({ x: place.x, y: place.y });
      return way;
    }

    movePet(pet, elapsed, now) {
      const { speed, stay } = PETS[pet.kind];
      // The axolotl swims in its tank on the counter, and never leaves it.
      if (pet.kind === 'axolotl') {
        pet.position = placed('wall', { x: 238, y: 33 });
        pet.target = { ...pet.position, depth: 47, pose: 'swim' };
        pet.isWalking = false;
        pet.isIn = true;
        return;
      }
      // The corgi does laps of the rug when an agent finishes: round the coffee table, fast, and back where it was
      if (pet.kind === 'corgi' && this.finishedAt !== undefined && now - this.finishedAt < 0.2 && !pet.zoom) {
        const lap = [{ x: 248, y: 142 }, { x: 304, y: 142 }, { x: 304, y: 124 }, { x: 248, y: 124 }, { x: 248, y: 142 }].map((p) => placed('couch', p));
        if (lap.every((p) => standsFree(p))) {
          let from = pet.position;
          const path = [];
          for (const p of [...lap, ...lap]) { path.push(...walkway(from, p)); from = p; }
          pet.zoom = { at: now };
          pet.target = { x: from.x, y: from.y, pose: 'stand' };
          pet.path = path;
          pet.isWalking = true;
        }
      }
      if (pet.zoom && !pet.path.length) pet.zoom = null;
      // The snail leaves a trail, shining for a while
      if (pet.kind === 'snail') {
        const last = pet.trail[pet.trail.length - 1];
        if (pet.isWalking && (!last || Math.hypot(last.x - pet.position.x, last.y - pet.position.y) >= 1)) pet.trail.push({ x: Math.round(pet.position.x), y: Math.round(pet.position.y), at: now });
        pet.trail = pet.trail.filter((p) => now - p.at < TRAIL_FOR);
      }
      const follow = pet.target && pet.target.follow;
      if (follow && (follow.isLeaving || !follow.isWalking || now - pet.arrivedAt > FOLLOW_FOR)) {
        pet.target = null;
        pet.isWalking = false;
        pet.nextMove = now;
      }
      // Settled too near another pet on the floor, the one later in the list moves on
      if (!pet.isWalking && pet.isIn && now < pet.nextMove && !PETS[pet.kind].flies && !(pet.target && pet.target.depth !== undefined)) {
        const mine = this.pets.indexOf(pet);
        if (this.pets.some((o, i) => i < mine && o.isIn && !PETS[o.kind].flies && Math.hypot(o.position.x - pet.position.x, o.position.y - pet.position.y) < PET_GAP)) pet.nextMove = now;
      }
      // Settled on the floor where someone has come to stand, or is about to walk: it moves on, out of their way
      if (!pet.isWalking && pet.isIn && pet.target && pet.target.depth === undefined && now < pet.nextMove && !PETS[pet.kind].flies
        && (this.people.some((p) => !p.isWalking && !p.isLeaving && Math.hypot(p.position.x - pet.position.x, p.position.y - pet.position.y) < PET_CLEAR.person - 2)
          || this.inSomeonesWay(pet.position, 5))) {
        pet.nextMove = now;
      }
      // Not in yet: it comes in when its turn comes, and the door is clear of the other pets
      if (!pet.isIn) {
        if (now < pet.nextMove || this.pets.some((o) => o !== pet && o.isIn && Math.hypot(o.position.x - DOOR.x, o.position.y - DOOR.y - 6) < PET_CLEAR.pet)) return;
        pet.isIn = true;
        pet.position = { x: DOOR.x, y: DOOR.y + 6 };
      }
      if (!pet.isWalking && now >= pet.nextMove) {
        let others = this.petPlaces(pet).filter((p) => !pet.target || p.x !== pet.target.x || p.y !== pet.target.y);
        // Nowhere of its own to go, and in the way where it is: the nearest free spot out of everyone's way
        if (!others.length && !pet.target?.depth && !this.petCanSettle({ x: pet.position.x, y: pet.position.y }, pet)) {
          for (let r = 10; r <= 50 && !others.length; r += 8) {
            for (let k = 0; k < 12; k++) {
              const spot = { x: pet.position.x + Math.cos((k / 12) * Math.PI * 2) * r, y: pet.position.y + Math.sin((k / 12) * Math.PI * 2) * r * 0.7, pose: 'sit' };
              if (this.petCanSettle(spot, pet)) others.push(spot);
            }
          }
        }
        if (others.length) {
          // The monkey leaves the counter with a cup, and keeps it until it moves on again.
          pet.carrying = !!(pet.target && pet.target.grab);
          if (pet.carrying) pet.tookCupAt = now;
          pet.target = pick(others);
          if (pet.target.follow) pet.followAt = now;
          pet.isWalking = true;
          pet.arrivedAt = now;
          pet.path = pet.target.follow ? [] : this.petRoute(pet, pet.target);
          pet.rerouted = 0;
          pet.waited = 0;
          pet.leg = pet.path.reduce((sum, p, i) => sum + Math.hypot(p.x - (i ? pet.path[i - 1] : pet.position).x, p.y - (i ? pet.path[i - 1] : pet.position).y), 0);
        } else {
          pet.nextMove = now + random(...stay);
        }
      }
      if (!pet.isWalking) {
        // Settled, it turns to watch whoever walks by: in, out, fired or hired; or another pet; the cat takes no notice of dogs
        const ignores = pet.kind === 'cat' ? ['dog', 'corgi', 'robodog'] : [];
        const by = [...this.people, ...this.pets.filter((o) => o !== pet && !ignores.includes(o.kind))]
          .find((p) => p.isWalking && Math.hypot(p.position.x - pet.position.x, p.position.y - pet.position.y) < PET_WATCHES);
        if (by && Math.abs(by.position.x - pet.position.x) > 3) pet.facing = by.position.x > pet.position.x ? 1 : -1;
        return;
      }
      // Up a wall, the lizard won't move while anyone is near; on the floor it gets out of the way like any pet.
      if (pet.kind === 'lizard' && !standsFree(pet.position) && this.people.some((p) => Math.hypot(p.position.x - pet.position.x, p.position.y - pet.position.y) < LIZARD_SHY)) {
        if (!pet.frozen) pet.frozeAt = now;
        pet.frozen = true;
        return;
      }
      pet.frozen = false;
      const following = pet.target.follow;
      const behind = following && { x: following.position.x - 7 * (following.position.x >= pet.position.x ? 1 : -1), y: following.position.y + 1 };
      if (following && (!pet.path.length || Math.hypot(behind.x - pet.followed.x, behind.y - pet.followed.y) > 6) && now - pet.rerouted > 0.4) {
        // After someone, a little behind them, its way worked out again once they've gone on a bit
        pet.path = PETS[pet.kind].flies ? flightOf(pet.position, behind) : walkway(pet.position, behind);
        if (!PETS[pet.kind].flies && !standsFree(behind)) pet.path.pop(); // as near as it can get
        pet.followed = behind;
        pet.rerouted = now;
      }
      const flies = PETS[pet.kind].flies;
      // On foot, it gives way: anyone just ahead, it waits for, a moment, then goes round them
      if (!flies && pet.path.length) {
        const ahead = pet.path[0], dx = ahead.x - pet.position.x, dy = ahead.y - pet.position.y, d = Math.hypot(dx, dy) || 1;
        // People, and other pets: one settled, or ahead of it in the list, so two never wait for each other
        const mine = this.pets.indexOf(pet);
        const others = [...this.people, ...this.pets.filter((o, i) => o !== pet && o.isIn && !PETS[o.kind].flies && o.kind !== 'axolotl' && (!o.isWalking || o.waited > 0 || i < mine))];
        const inWay = others.find((p) => {
          const px = p.position.x - pet.position.x, py = p.position.y - pet.position.y, along = (px * dx + py * dy) / d;
          return p !== following && along > 0 && along < GIVE_WAY && Math.abs(px * dy - py * dx) / d < 5;
        });
        // Nor a step that would bring it closer than PET_GAP to a pet it gives way to
        const step = Math.min(d, speed * elapsed), next = { x: pet.position.x + (dx / d) * step, y: pet.position.y + (dy / d) * step };
        const pets = this.pets.filter((o) => o !== pet && o.isIn && !PETS[o.kind].flies && o.kind !== 'axolotl');
        // A pet on the move gets a little more room: both take their step in the same moment
        const tooClose = pets.find((o) => Math.hypot(o.position.x - next.x, o.position.y - next.y) < PET_GAP + (o.isWalking ? 3 : 0)
          && Math.hypot(o.position.x - next.x, o.position.y - next.y) < Math.hypot(o.position.x - pet.position.x, o.position.y - pet.position.y));
        const blocker = inWay || tooClose;
        // Someone walking straight at it: it steps aside at once, the side that's free, then goes on
        const coming = blocker && blocker.isWalking && blocker.path && blocker.path[0]
          && Math.hypot(blocker.path[0].x - pet.position.x, blocker.path[0].y - pet.position.y) < Math.hypot(blocker.position.x - pet.position.x, blocker.position.y - pet.position.y);
        if (coming && !pet.steppedAside) {
          const goal = pet.target.follow ? behind : (pet.path[pet.path.length - 1] || pet.target);
          for (const side of [1, -1]) {
            const aside = { x: pet.position.x - (dy / d) * 8 * side, y: pet.position.y + (dx / d) * 8 * side };
            if (!standsFree(aside)) continue;
            pet.path = [...walkway(pet.position, aside), ...walkway(aside, goal, false, [blocker.position])];
            pet.steppedAside = true;
            break;
          }
        }
        if (!blocker) pet.steppedAside = false;
        if (blocker && !coming) {
          pet.waited += elapsed;
          if (pet.waited < GIVE_WAY_FOR) return;
          // Round them, the way worked out again keeping clear of them, then on
          const goal = pet.target.follow ? behind : (pet.path[pet.path.length - 1] || pet.target);
          const round = walkway(pet.position, goal, true, [blocker.position]);
          if (round) pet.path = round;
          pet.waited = 0;
        } else {
          pet.waited = 0;
        }
      }
      const pace = speed * (pet.zoom ? 2.2 : 1) * elapsed;
      // Too close to another pet already: first a step straight away from it, if there's floor there
      if (!flies) {
        const crowding = this.pets.find((o) => o !== pet && o.isIn && !PETS[o.kind].flies && o.kind !== 'axolotl'
          && Math.hypot(o.position.x - pet.position.x, o.position.y - pet.position.y) < PET_GAP);
        if (crowding) {
          const ax = pet.position.x - crowding.position.x, ay = pet.position.y - crowding.position.y, ad = Math.hypot(ax, ay) || 1;
          const away = { x: pet.position.x + (ax / ad) * pace, y: pet.position.y + (ay / ad) * pace };
          if (standsFree(away)) {
            pet.position = away;
            if (Math.abs(ax) > 0.01) pet.facing = ax > 0 ? 1 : -1;
            return;
          }
        }
      }
      const { position, heading } = stepAlong(pet.position, pet.path, pace);
      pet.position = position;
      // It faces the way it goes, but not for every little step up or down
      if (Math.abs(heading.x) > Math.abs(heading.y) * 0.35 && Math.abs(heading.x) > 0.01) pet.facing = heading.x > 0 ? 1 : -1;
      if (pet.path.length) return;
      if (following) return; // keeps up until the person stops
      pet.position = { x: pet.target.x, y: pet.target.y };
      pet.isWalking = false;
      pet.arrivedAt = now;
      pet.nextMove = now + random(...stay);
    }

    /**
     * The first in line, once at the button: hand up, then down on the dome,
     * then a moment to watch before going back to their desk.
     */
    press(now) {
      const presser = this.nuclearQueue[0];
      if (!presser || presser.place !== 'button' || presser.isWalking) return;
      const held = now - presser.arrivedAt;
      if (presser.pressedAt === null && held >= WIND_UP) {
        presser.pressedAt = now;
        presser.deskEffort = presser.session.effort;
        this.detonation = { desk: presser.desk, at: now };
      } else if (held >= WIND_UP + AFTERGLOW) {
        this.standDown(presser);
        this.send(presser, this.home(presser));
      }
    }

    /* Staff */

    get staffWanted() {
      return Math.max(0, STAFF_COUNTS[decor.staff] - this.people.filter((p) => !p.isStaff && !p.isLeaving).length);
    }

    hireStaff() {
      this.staffHired += 1;
      const name = `staff ${this.staffHired}`;
      const member = new Person({ id: name, name, state: 'idle', effort: '', agent: 'claude' }, null);
      this.people.push(member);
      return member;
    }

    /** One arrives or one leaves, then a pause before the next. */
    changeStaff(now) {
      if (!this.isOpen || now < this.nextStaffChange) return;
      const present = this.people.filter((p) => p.isStaff && !p.isLeaving);
      const free = this.freeSpots(null);
      if (present.length < this.staffWanted && free.length) {
        this.send(this.hireStaff(), spotPlace(pick(free)));
      } else if (present.length > this.staffWanted) {
        const leaver = present.find((p) => !p.isWalking) || present[0];
        leaver.isLeaving = true;
        this.send(leaver, 'door');
      } else {
        return;
      }
      this.nextStaffChange = now + random(...STAFF_INTERVAL);
    }

    /** Lounge spots nobody else has taken. Agents don't count staff, who make way for them. */
    freeSpots(person) {
      const agent = person ? !person.isStaff : false;
      return spots.map((_, spot) => spot).filter((spot) => !spots[spot].off && !this.people.some((other) =>
        other !== person && !other.isLeaving && other.place === spotPlace(spot) && !(agent && other.isStaff)));
    }

    standDown(person) {
      person.isGoingNuclear = false;
      person.pressedAt = null;
      person.deskEffort = person.session.effort;
      this.nuclearQueue = this.nuclearQueue.filter((p) => p !== person);
    }

    update(person, placement, now) {
      const before = person.session;
      const wasIdle = before.state === 'idle';
      const session = placement.session;
      if (session.tool !== before.tool) person.toolSince = now;
      person.session = session;

      person.askingSince = this.waiting.get(session.id) ?? null;
      // A turn of an hour or more, finished: a little party.
      const finished = before.state === 'thinking' && (session.state === 'unread' || session.state === 'idle');
      if (finished && before.turnStartedAt && Date.now() - before.turnStartedAt >= LONG_HAUL * 1000) person.celebratedAt = now;
      if (finished) this.finishedAt = now;

      // Only a press of ☢ Max sends them to the button; the dial reaching max
      // just repaints the desk. Turned back down before they get there, they
      // go back to their desk.
      const pressed = session.maxPressedAt;
      if (pressed && pressed !== person.answeredPress && !person.isGoingNuclear) {
        person.isGoingNuclear = true;
        person.sawMax = false;
        this.nuclearQueue.push(person);
      }
      if (person.isGoingNuclear && session.effort === 'max') person.sawMax = true;
      if (person.isGoingNuclear && person.sawMax && session.effort !== 'max' && person.pressedAt === null) {
        this.standDown(person);
        this.send(person, this.home(person));
      }
      person.answeredPress = pressed;
      if (!person.isGoingNuclear) {
        if (EFFORTS.indexOf(session.effort) > EFFORTS.indexOf(person.deskEffort)) person.effortRoseAt = now;
        person.deskEffort = session.effort;
      }

      if (person.desk !== placement.desk) {
        person.desk = placement.desk;
        if (deskOf(person.place) !== null || person.place.startsWith('crowd:')) this.send(person, this.home(person));
      }
      // Done and seen: a moment to stretch before getting up.
      if (session.state === 'idle' && !wasIdle && !person.isWalking) person.nextMove = now + random(3, 6);
    }

    decide(person, now) {
      if (person.isLeaving) return;
      if (person.isStaff) {
        // Staff only ever move between lounge spots.
        if (person.isWalking || now < person.nextMove) return;
        const free = this.freeSpots(person);
        if (free.length) this.send(person, spotPlace(pick(free)));
        else person.nextMove = now + random(5, 10);
        return;
      }
      const desk = this.home(person);
      if (person.isGoingNuclear) {
        const place = this.nuclearQueue[0] === person ? 'button' : 'queue';
        if (person.place !== place) this.send(person, place);
        return;
      }
      const duty = this.dutyOf(person, now);
      if (duty) {
        if (person.place !== duty) this.send(person, duty);
        return;
      }
      // Nothing to do any more, away from the desk on a job: back to it first.
      if (/^(board|clock|shred|locked|storm):/.test(person.place) && person.place !== desk) {
        this.send(person, desk);
        return;
      }
      if (person.isWalking || now < person.nextMove) return;

      const free = this.freeSpots(person);
      const backToDesk = person.place !== desk && (!free.length || Math.random() < 0.35);
      if (backToDesk) {
        this.send(person, desk);
      } else if (free.length) {
        const spot = pick(free);
        this.send(person, spotPlace(spot));
        // Whoever of the staff is there finds somewhere else to be.
        for (const member of this.people) if (member.isStaff && member.place === spotPlace(spot)) member.nextMove = now;
      } else {
        person.nextMove = now + random(8, 20);
      }
    }

    /**
     * Where a session's work takes its agent, or null when it has none and
     * they're free to wander: the shredder while it compacts, the clock while
     * it waits out a rate limit, the whiteboard for a to-do list that takes a
     * while, and otherwise their desk.
     */
    dutyOf(person, now) {
      const s = person.session, desk = this.home(person);
      // Compacting, at the shredder by their desk: in the line, there's none.
      if (s.compacting && !DESKS[person.desk].off) return `shred:${person.desk}`;
      if (s.state === 'error' && s.errorKind === 'rate_limit') return this.wallSpot(person, 'clock', CLOCK_SPOTS.length) || desk;
      const failure = failureOf(s);
      if (failure === 'auth') return this.wallSpot(person, 'locked', LOCKED_SPOTS.length) || desk;
      if (failure === 'overloaded' && takeOf(person) === 0) return this.wallSpot(person, 'storm', stormSpots.length) || desk;
      if (s.state === 'thinking' && toolKind(s.tool) === 'todo' && now - person.toolSince >= LONG_TOOL) {
        return this.wallSpot(person, 'board', boardSpots.length) || desk;
      }
      return s.state === 'idle' ? null : desk;
    }

    /** One of the spots along the back wall nobody else has taken. */
    wallSpot(person, kind, count) {
      if (person.place.startsWith(`${kind}:`)) return person.place;
      for (let i = 0; i < count; i++) {
        const place = `${kind}:${i}`;
        if (!this.people.some((p) => p !== person && !p.isLeaving && p.place === place)) return place;
      }
      return null;
    }

    /** Where an agent belongs: at their desk, or in the line along the wall while it's put away. */
    home(person) {
      if (!DESKS[person.desk] || !DESKS[person.desk].off) return deskPlace(person.desk);
      return this.wallSpot(person, 'crowd', CROWD_SPOTS.length) || 'crowd:0';
    }

    send(person, place) {
      person.place = place;
      person.refuge = null;
      person.rounded = null;
      person.path = route(person.position, place);
      person.nextMove = Infinity;
    }

    /**
     * Who goes first when two meet: anyone hurrying to the button, then
     * anyone leaving, then whoever came in first. The other gives way.
     */
    goesFirst(a, b) {
      const rank = (p) => (p.isGoingNuclear ? 0 : p.isLeaving ? 1 : 2);
      if (rank(a) !== rank(b)) return rank(a) < rank(b);
      return this.people.indexOf(a) < this.people.indexOf(b);
    }

    /** Where `person` will be `seconds` from now, going on along their way at their pace. */
    ahead(person, seconds) {
      if (!person.isWalking) return { ...person.position };
      let left = WALKING_SPEED * (person.isGoingNuclear ? HURRY : 1) * seconds, { x, y } = person.position;
      for (const to of person.path) {
        const d = Math.hypot(to.x - x, to.y - y);
        if (d >= left) return { x: x + ((to.x - x) / d) * left, y: y + ((to.y - y) / d) * left };
        left -= d;
        ({ x, y } = to);
      }
      return { x, y };
    }

    /** Whether `a` and `b`, going on as they are, come within PERSON_GAP of each other in the next PERSON_LOOKS steps. */
    meeting(a, b) {
      for (let k = 1; k <= PERSON_LOOKS; k++) {
        const pa = this.ahead(a, k * PERSON_STEP), pb = this.ahead(b, k * PERSON_STEP);
        if (Math.hypot(pa.x - pb.x, pa.y - pb.y) < PERSON_GAP) return true;
      }
      return false;
    }

    /** Whether `walker`'s way, the next stretch of it, passes within `reach` of `point`. */
    passes(walker, point, reach) {
      for (let k = 0; k <= PERSON_LOOKS * 2; k++) {
        const p = this.ahead(walker, k * PERSON_STEP);
        if (Math.hypot(p.x - point.x, p.y - point.y) < reach) return true;
      }
      return false;
    }

    /**
     * Before a step: whether to wait, or a way round someone. Coming in,
     * one at a time through the door. Then, looking a second ahead: anyone
     * who goes first (see `goesFirst`) and would come too close, they wait
     * for; and if that one's way goes right over where they are, as in the
     * strip behind the back row of desks, they step out of it to the
     * nearest free spot, wait there till they've gone by, and carry on.
     * Going first, they go round anyone who's stopped or standing in their
     * way. Near where they're going they can come close.
     */
    giveWay(person, elapsed) {
      const near = (a, b, reach) => Math.hypot(a.x - b.x, a.y - b.y) < reach;
      const up = (p) => p !== person && !p.entering && !(p.isAtDesk && !p.isWalking);
      if (person.entering === undefined) person.entering = near(person.position, DOOR, 0.5) && !person.isLeaving;
      if (person.entering) {
        if (this.people.some((p) => up(p) && near(p.position, DOOR, PERSON_GAP + 2))) return true;
        person.entering = false;
      }
      // Stepped out of someone's way: waiting there till they've gone by, then on; they've gone, then on at once
      if (person.refuge && (!this.people.includes(person.refuge.for) || !person.refuge.for.isWalking)) {
        person.path = walkway(person.position, person.refuge.goal);
        person.refuge = null;
      }
      const refuge = person.refuge;
      const end = refuge ? refuge.goal : person.path[person.path.length - 1];
      if (refuge && near(person.position, refuge.at, 1)) {
        // Someone else going first coming right through here: on to another spot
        const coming = this.people.find((p) => up(p) && p.isWalking && p !== refuge.for && this.goesFirst(p, person) && this.passes(p, person.position, PERSON_GAP));
        const spot = coming && this.clearOf(person, coming);
        if (spot) {
          person.refuge = { ...refuge, at: spot };
          person.path = [...walkway(person.position, spot), ...walkway(spot, refuge.goal)];
          return false;
        }
        const by = refuge.for;
        if (coming || (this.people.includes(by) && by.isWalking && (near(by.position, person.position, PERSON_GAP + 4) || this.passes(by, person.position, PERSON_GAP + 1)))) return true;
        person.refuge = null;
        person.path = walkway(person.position, refuge.goal);
        return false;
      }
      if (!refuge && near(person.position, end, PERSON_ROOM)) return false;
      // A pet sitting, or waiting, on the floor in the way: round it, again whenever it's moved
      for (const pet of this.pets) {
        if (!pet.isIn || PETS[pet.kind].flies || (pet.isWalking && !(pet.waited > 0)) || (pet.target && pet.target.depth !== undefined && !standsFree(pet.position))) continue;
        if (!this.passes(person, pet.position, PERSON_GAP - 1)) continue;
        if (this.roundedAlready(person, pet)) continue;
        const round = walkway(person.position, end, true, [pet.position]);
        if (round) person.path = round;
      }
      for (const other of this.people) {
        if (!up(other)) continue;
        if (other.isWalking && this.goesFirst(other, person)) {
          if (!this.meeting(person, other)) continue;
          // Their way goes right over us: out of it, to the nearest spot clear of it, unless already on the way to one
          if (!refuge && this.passes(other, person.position, PERSON_GAP + 1)) {
            const spot = this.clearOf(person, other);
            if (spot) {
              person.refuge = { at: spot, for: other, goal: end };
              person.path = [...walkway(person.position, spot), ...walkway(spot, end)];
              return false;
            }
          }
          return true;
        }
        // Going first: round anyone stopped or standing in our way, again whenever they've moved
        if ((!other.isWalking || other.refuge) && this.passes(person, other.position, PERSON_GAP) && !this.roundedAlready(person, other)) {
          const round = walkway(person.position, end, true, [other.position]);
          if (round) person.path = round;
        }
      }
      return false;
    }

    /**
     * Whether `person` has already looked for a way round `obstacle` where it
     * is now, and if not, notes that they have. Kept for each obstacle, so two
     * in the way don't take turns being forgotten and have the whole way
     * worked out again every frame; and kept when no way round was found, so
     * that isn't tried again until the obstacle moves.
     */
    roundedAlready(person, obstacle) {
      person.rounded ??= new Map();
      const at = person.rounded.get(obstacle);
      if (at && near(at, obstacle.position, 3)) return true;
      person.rounded.set(obstacle, { ...obstacle.position });
      return false;
    }

    /** The nearest free spot to `person`, a little way off, clear of the way `by` is going and of everyone else. */
    clearOf(person, by) {
      let best = null;
      for (let r = 6; r <= 22 && !best; r += 4) {
        for (let k = 0; k < 16; k++) {
          const a = (k / 16) * Math.PI * 2, spot = { x: person.position.x + Math.cos(a) * r, y: person.position.y + Math.sin(a) * r * 0.8 };
          if (spot.x < 2 || spot.y < 2 || spot.x > shape.width - 2 || spot.y > shape.height - 2 || !standsFree(spot)) continue;
          const { columns, clearance } = floorNow();
          if (clearance[Math.floor(spot.y / CELL) * columns + Math.floor(spot.x / CELL)] < 2) continue;
          // Clear of where anyone's walking, not just them, and of anyone standing
          if (this.people.some((p) => p !== person && p.isWalking && this.passes(p, spot, PERSON_GAP + 2))) continue;
          if (this.people.some((p) => p !== person && p !== by && Math.hypot(p.position.x - spot.x, p.position.y - spot.y) < PERSON_GAP + 1)) continue;
          if (!best || Math.hypot(spot.x - person.position.x, spot.y - person.position.y) < Math.hypot(best.x - person.position.x, best.y - person.position.y)) best = spot;
        }
      }
      return best;
    }

    walk(person, elapsed, now) {
      if (!person.path.length) return;
      if (this.giveWay(person, elapsed)) return;
      const stride = WALKING_SPEED * (person.isGoingNuclear ? HURRY : 1) * elapsed;
      const { position, heading } = stepAlong(person.position, person.path, stride);
      person.position = position;
      // Back to the room going up the room, and only then: a little sideways doesn't turn them round
      if (heading.y < 0 && -heading.y > Math.abs(heading.x) * 1.2) person.isFacingAway = true;
      else if (heading.y > 0 || Math.abs(heading.x) > -heading.y * 1.2) person.isFacingAway = false;
      if (person.path.length) return;
      person.arrivedAt = now;
      // Hired, they've come in and sat down: a role's hire shows what they're there for.
      if (person.isNew && person.place === this.home(person)) {
        person.isNew = false;
        person.settledAt = now;
      }
      const spot = spotOf(person.place);
      if (spot !== null) {
        person.isFacingAway = facesAway(spots[spot].pose);
        person.nextMove = now + random(12, 30);
      } else {
        // At the whiteboard, at a window, or knocking at the door, their back is to the room.
        person.isFacingAway = /^(board|storm):/.test(person.place) || (person.place.startsWith('locked:') && takeOf(person) === 0);
        person.nextMove = now + random(8, 20);
      }
    }
  }

  /** People: the gap kept from anyone up and about, how far ahead they look, and how near where they're going they stop minding. */
  const PERSON_GAP = 6;
  const PERSON_ROOM = 8;
  /** How far ahead people look for each other: this many steps of this many seconds. */
  const PERSON_LOOKS = 8;
  const PERSON_STEP = 0.1;
  /** Art pixels a second: crossing the room takes about ten seconds. */
  const WALKING_SPEED = 28;
  /** Going nuclear is worth hurrying for. */
  const HURRY = 1.8;
  /** How long the presser holds their hand up before bringing it down. */
  const WIND_UP = 0.45;
  /** How long they stay to watch, after pressing, before heading back. */
  const AFTERGLOW = 1.6;
  /** Staff arrive and leave one at a time, this many seconds apart. */
  const STAFF_INTERVAL = [3, 6];
  /** Only a tool that's been running this long gets its agent up from the desk, so nobody jitters back and forth. */
  const LONG_TOOL = 1.5;

  /* ------------------------------------------------------------ drawing */

  const SHAKE_LENGTH = 0.6;
  const FLASH_LENGTH = 0.6;
  const SHOCKWAVE_LENGTH = 1;
  /** How long the warning light turns after a press. */
  const ALARM_LENGTH = 4;
  /** The desk labels' text, in art pixels… */
  const LABEL_SIZE = 3.4;
  /** …and never smaller than this on screen, in CSS pixels, so a phone can read it. */
  const LABEL_SMALLEST = 8;
  /**
   * How long a turn has run, short enough for its monitor: 9:41 for the
   * first ten minutes, then 12m, then 2h.
   */
  function turnLength(seconds) {
    const minutes = Math.floor(seconds / 60);
    if (minutes < 10) return `${minutes}:${String(seconds % 60).padStart(2, '0')}`;
    if (minutes < 60) return `${minutes}m`;
    return `${Math.min(Math.floor(minutes / 60), 99)}h`;
  }

  /** Waiting on you longer than this, in seconds, an agent gets agitated, and more so the longer it goes on… */
  const AGITATED_AFTER = 300;
  /** …reaching its worst this much later. */
  const AGITATION_RISE = 600;
  /** A turn this long, in seconds, ends in confetti… */
  const LONG_HAUL = 3600;
  /** …for this long. */
  const PARTY_LENGTH = 3;

  /** How agitated someone waiting on you is, from 0 to 1, or 0 for anyone else. */
  function agitationOf(person, now) {
    if (person.askingSince === null || person.isLeaving) return 0;
    const waited = now - person.askingSince - AGITATED_AFTER;
    return waited < 0 ? 0 : Math.min(1, 0.3 + (0.7 * waited) / AGITATION_RISE);
  }

  /** The season of each month, January first, as the windows show it. */
  const SEASONS = ['winter', 'winter', 'spring', 'spring', 'spring', 'summer', 'summer', 'summer', 'autumn', 'autumn', 'autumn', 'winter'];
  /** In summer a bird crosses each window this often, in seconds, taking this long. */
  const BIRD_EVERY = 23;
  const BIRD_CROSSING = 4;

  /** After this long, a turn's timer shows its agent sweating, in seconds. */
  const LONG_TURN = 600;
  /** Digits and a few marks, three art pixels wide and five tall, for timers and counts. */
  const GLYPHS = {
    0: ['111', '101', '101', '101', '111'], 1: ['010', '110', '010', '010', '111'], 2: ['111', '001', '111', '100', '111'],
    3: ['111', '001', '111', '001', '111'], 4: ['101', '101', '111', '001', '001'], 5: ['111', '100', '111', '001', '111'],
    6: ['111', '100', '111', '101', '111'], 7: ['111', '001', '001', '001', '001'], 8: ['111', '101', '111', '101', '111'],
    9: ['111', '101', '111', '001', '111'], ':': ['0', '1', '0', '1', '0'], '+': ['000', '010', '111', '010', '000'],
    '-': ['000', '000', '111', '000', '000'],
    m: ['000', '000', '111', '111', '101'], h: ['100', '100', '111', '101', '101'],
    // The letters the office's buttons spell
    D: ['110', '101', '101', '101', '110'], E: ['111', '100', '110', '100', '111'], F: ['111', '100', '110', '100', '100'],
    G: ['011', '100', '101', '101', '011'],
    H: ['101', '101', '111', '101', '101'], I: ['111', '010', '010', '010', '111'], K: ['101', '101', '110', '101', '101'],
    O: ['010', '101', '101', '101', '010'], P: ['110', '101', '110', '100', '100'], R: ['110', '101', '110', '101', '101'],
    U: ['101', '101', '101', '101', '111'], Y: ['101', '101', '010', '010', '010'], "'": ['1', '1', '0', '0', '0'],
    A: ['010', '101', '111', '101', '101'], C: ['011', '100', '100', '100', '011'], L: ['100', '100', '100', '100', '111'],
    N: ['101', '111', '111', '101', '101'], S: ['011', '100', '010', '001', '110'], T: ['111', '010', '010', '010', '010'],
    W: ['101', '101', '111', '111', '101'],
    ' ': ['00', '00', '00', '00', '00'],
  };
  /** How wide `text` is in the pixel font. */
  const textWidth = (text) => [...text].reduce((w, c) => w + GLYPHS[c][0].length + 1, -1);
  /** How long everyone looks up when the mic goes on, as if the boss were shouting. */
  const LOOK_UP_LENGTH = 1;
  /** The listener's nod, once the mic is let go. */
  const NOD_LENGTH = 0.6;
  /** A wobble, for the button or the mic tapped with no desk picked. */
  const WOBBLE_LENGTH = 0.4;
  /** Sound waves leave the mic this often while it's held, and spread this far. */
  const WAVE_EVERY = 0.5;
  const WAVE_REACH = 110;

  /**
   * Where the deck's own controls are out of sight (full screen, /office, the
   * pop-out window), the picked agent waiting on you gets keys in place of
   * its bubble: ▲ ▼ through the options, ✓ to accept, ✕ to reject. Each key,
   * in art pixels…
   */
  const CHOICE_KEY = { w: 16, h: 14 };
  /** …the gap between keys, and the wider one between the arrows and the answers… */
  const CHOICE_GAP = 3;
  const CHOICE_SPLIT = 6;
  /** …and from the panel's edge to the keys. */
  const CHOICE_INSET = 3;
  /** A tapped key stays down at least this long, so even a quick tap shows. */
  const CHOICE_PRESS_LENGTH = 0.15;
  /** Each key's mark, a row of art pixels at a time. */
  const CHOICE_MARKS = {
    up: ['0001000', '0011100', '0111110', '1111111'],
    down: ['1111111', '0111110', '0011100', '0001000'],
    accept: ['0000011', '0000110', '1101100', '0111000', '0010000'],
    reject: ['11011', '01110', '00100', '01110', '11011'],
  };

  /**
   * Where the dial is out of sight, a finger held on a Claude Code desk puts
   * paint chips along its front, one for each effort from white for low to
   * red for max, the colours the desk is painted in: a tap on one sets that
   * effort, as the dial does, and puts them away. Each chip, in art pixels,
   * and the gap between them.
   */
  const CHIP = { w: 11, h: 10 };
  /** They come out for a finger held this long on the desk or its person, in seconds: a tap only picks it. */
  const HOLD_FOR_CHIPS = 0.45;
  /**
   * Held, then moved this far, in CSS pixels, the finger picks the agent up
   * instead: carried to another desk and let go there, the two desks swap.
   */
  const CARRY_AFTER = 10;
  const CHIP_GAP = 1;
  const EFFORTS = ['low', 'medium', 'high', 'xhigh', 'max'];

  /** The chips on the desk `desk`: across its front, a little wider than it, kept inside the room. */
  function effortChips(desk) {
    const { x, y } = anchorOf(desk);
    const w = EFFORTS.length * CHIP.w + (EFFORTS.length - 1) * CHIP_GAP;
    const left = Math.max(1, Math.min(shape.width - 1 - w, Math.round(x + DESK_WIDTH / 2 - w / 2)));
    return { left, top: y + 5, w, h: CHIP.h, chips: EFFORTS.map((level, i) => ({ level, x: left + i * (CHIP.w + CHIP_GAP), y: y + 5 })) };
  }

  /**
   * With hiring and firing on, a desk held down also offers a pixel button:
   * YOU'RE FIRED above its agent's chips, or HIRE on an empty desk. In art
   * pixels, kept inside the room.
   */
  function deskButton(desk, kind) {
    const label = kind === 'fire' ? "YOU'RE FIRED" : '+ HIRE';
    const w = textWidth(label) + 8, h = 11;
    const { x, y } = anchorOf(desk);
    const left = Math.max(1, Math.min(shape.width - 1 - w, Math.round(x + DESK_WIDTH / 2 - w / 2)));
    return { kind, label, left, top: kind === 'fire' ? y - 8 : y + 4, w, h };
  }

  /** The question before anyone is fired: a panel in the middle of the room, with its two keys. */
  const FIRE_DIALOG = { w: 132, h: 52 };
  const DIALOG_KEY = { w: 34, h: 13 };
  function fireDialog() {
    const { w, h } = FIRE_DIALOG;
    const left = Math.round(shape.width / 2 - w / 2), top = Math.round(shape.height / 2 - h / 2);
    const y = top + h - DIALOG_KEY.h - 6;
    return {
      left, top, w, h,
      keys: [
        { action: 'keep', label: 'KEEP', x: left + 22, y, ...DIALOG_KEY },
        { action: 'fired', label: 'FIRE', x: left + w - 22 - DIALOG_KEY.w, y, ...DIALOG_KEY },
      ],
    };
  }

  /**
   * Where the deck's controls are out of sight, a pixel key in the room's
   * top-left corner offers the office's styles: a panel over the room with a
   * picture of each, Yours first. In art pixels: the key, from the corner of
   * what's seen of the room (further in where the window's corners are
   * rounded), its mark (a paint roller), each picture, and the gaps round
   * them.
   */
  const STYLE_KEY = { w: 15, h: 13, inset: 3, rounded: 6 };
  /** How long a tapped music piece shows ▶ or ❚❚ over it, in seconds. */
  const MUSIC_TAP_LENGTH = 0.9;
  /**
   * A music piece's control, opened by a finger held on the piece as long as
   * a desk's chips take: its keys, what each asks of the computer's music
   * through `on.music`, and how long it stays open unused, in seconds.
   */
  const MUSIC_KEY = { w: 11, h: 9, gap: 2 };
  /**
   * The picked agent's keys, in a row under its desk: their size in art
   * pixels, their labels' size, how long a key that wants a second tap
   * stays armed for it, in seconds, and what's always there, first.
   */
  const KEY_ROW = { h: 14, gap: 2, inset: 3, text: 7, icon: 13, pad: 4 };
  const KEY_ARMED_LENGTH = 2.5;
  /** Two fingers scroll a line for this many CSS pixels they move; down and up again within this many seconds, still, they're a tap. */
  const SCROLL_STEP = 4;
  const TWO_FINGER_TAP = 0.35;
  /**
   * What a touch sends only once the finger lifts, still, with no second
   * finger come down: the keys, choices, chips and buttons that type into an
   * agent, or fire one. A first finger landing on them for a two-finger
   * scroll sends nothing.
   */
  const SENT_ON_LIFT = ['key', 'choice', 'chip', 'button', 'fired'];
  /** How long the question before firing ignores taps, in seconds: a double tap that opened it doesn't answer it too. */
  const FIRE_QUESTION_SETTLES = 0.5;
  /** Return again this soon, by accept or send, is one tap landing twice, in seconds. */
  const RETURN_REPEAT = 0.6;
  const ROW_KEYS = [
    { action: 'esc', label: 'esc' }, { action: 'send', icon: 'send' }, { action: 'clear', label: '/clear', confirm: true },
    { action: 'up', icon: 'up' }, { action: 'down', icon: 'down' },
  ];
  /** The layout's own buttons that are already the office's, by their ids, and the names of the keys a button can press. */
  const BUILT_IN_BUTTONS = ['accept', 'reject', 'mic', 'up', 'down', 'esc', 'send', 'clear', 'max', 'efforts'];
  const KEY_NAMES = { enter: '⏎', escape: 'esc', tab: '⇥', 'shift-tab': '⇤', up: '↑', down: '↓', 'ctrl-c': '^C' };
  /** ⏎, in pixels. */
  const SEND_MARK = ['0000001', '0000001', '0100001', '1111111', '0100000'];
  /** The keys for the picked agent: the office's own, then your buttons from the layout, each by its label, its text or the key it presses. */
  function rowKeysOf(layout) {
    const yours = (layout && Array.isArray(layout.rows) ? layout.rows : []).flatMap((row) => (row && Array.isArray(row.buttons) ? row.buttons : []))
      .filter((b) => b && typeof b.id === 'string' && !BUILT_IN_BUTTONS.includes(b.id))
      .map((b) => ({ action: `button:${b.id}`, label: String(b.label || b.text || KEY_NAMES[b.key] || b.key || b.id).slice(0, 16), confirm: b.confirm === true }));
    return [...ROW_KEYS, ...yours];
  }
  /**
   * The badges a role can give its agents, in the order setup offers them:
   * the five roles' own props first, then eight for roles of your own, worn
   * on a lanyard. Each has its colour, for its name on a desk, and its icon,
   * 16 by 16, for the role's chip and its card in setup: a letter a pixel in
   * BADGE_INKS' colours, '.' for none. A lanyard's card has a picture three
   * by three, `pin`, the same way.
   */
  const BADGE_INKS = {
    k: 0x1f2129, w: 0xf4f1ea, g: 0xc4c8d0, G: 0x7d838f, r: 0xe0483e, R: 0xa32a24, y: 0xffd23f, Y: 0xe8a33a,
    b: 0x4c8ef7, B: 0x2f5fb3, c: 0xa8d8f0, n: 0xb07a46, N: 0x6e4a2e, p: 0xff8fb3, o: 0xff8a1f, e: 0x3fbf6b, t: 0x4fb8b0, T: 0x2e7d78,
    s: 0xe0ac7e, m: 0xa0603a,
  };
  const BADGES = {
    glasses: {
      colour: 0xff7a6e,
      icon: [
        '................',
        '................',
        '.kkkkk....kkkkk.',
        'kcccwck..kcccwck',
        'kccwcckkkkccwcck',
        'kcwccck..kcwccck',
        '.kkkkk....kkkkk.',
        '................',
        '.............kk.',
        '...........kkrRk',
        '.........kkrrrRk',
        '.......kkrrrRkk.',
        '.....kkwwwRkk...',
        '....kGwwkkk.....',
        '...kGGkk........',
        '...kk...........',
      ],
    },
    clipboard: {
      colour: 0x7ed49a,
      icon: [
        '......kkkk......',
        '..kkkkgGGgkkkk..',
        '.knnnkGggGknnnk.',
        '.knnkkkkkkkknnk.',
        '.knkwwwwwwwwknk.',
        '.knkwGGGGGwwknk.',
        '.knkwwwwwwwwknk.',
        '.knkweGGGGwwknk.',
        '.knkeewwwwwwknk.',
        '.knkwGGGGGGwknk.',
        '.knkwwwwwwwwknk.',
        '.knkwGGGGwwwknk.',
        '.knkwwwwwwwwknk.',
        '.knkkkkkkkkkknk.',
        '.kNnnnnnnnnnnNk.',
        '..kkkkkkkkkkkk..',
      ],
    },
    magnifier: {
      colour: 0x8fd3ff,
      icon: [
        '....kkkkk.......',
        '..kkgggggkk.....',
        '.kgkcccccGgk....',
        '.kgcwwcccccGk...',
        'kgcwwcccccccGk..',
        'kgcwccccccccGk..',
        'kgcccccccccccGk.',
        'kgcccccccccccGk.',
        'kgccccccccccGk..',
        '.kGcccccccccGk..',
        '.kGGcccccccGGk..',
        '..kkGGGGGGGkNNk.',
        '....kkkkkkkNNNk.',
        '...........kNNNk',
        '............kNNk',
        '.............kk.',
      ],
    },
    quill: {
      colour: 0x8fb3ff,
      icon: [
        '.............kk.',
        '............kwwk',
        '...........kwgwk',
        '..........kwgwk.',
        '.........kwgwk..',
        '..kkkkkkkwgwkk..',
        '.kBbbbbkwgwkwk..',
        '.kBbbbkwgwkwwk..',
        '.kBbbbkggkbwwk..',
        '.kBbbkGkkbbwwk..',
        '.kBbkGkbbbbwwk..',
        '.kBbkkbbbbbwwk..',
        '.kBbbbbbbbbwwk..',
        '.kBbbbbbbbbwwk..',
        '.kBBBBBBBBBkkk..',
        '..kkkkkkkkkk....',
      ],
    },
    wrench: {
      colour: 0xc6ccd6,
      icon: [
        '..........kkk...',
        '.........kgggk..',
        '........kggGGk.k',
        '........kgGk.kgk',
        '........kgGkkggk',
        '.......kggGGggGk',
        '......kggGGGGGk.',
        '.....kggGGkkkk..',
        '....kggGGk......',
        '...kggGGk.......',
        '..kggGGk........',
        '.kggGGk.........',
        'kggGGk..........',
        'kgGGk...........',
        'kGGk............',
        '.kk.............',
      ],
    },
    star: {
      colour: 0xffd23f,
      pin: ['.y.', 'yyy', 'y.y'],
      icon: [
        '.......kk.......',
        '......kyyk......',
        '......kyyk......',
        '.....kywyyk.....',
        'kkkkkkywyyykkkkk',
        'kyyyyywyyyyyyyyk',
        '.kyyyyyyyyyyyyk.',
        '..kyyyyyyyyyYk..',
        '...kyyyyyyyYk...',
        '...kyyyyyyyYk...',
        '..kyyyyyyyyyYk..',
        '..kyyyyYYyyyYk..',
        '.kyyyyYkkYyyyYk.',
        '.kyyYYk..kYYyYk.',
        'kyYYkk....kkYYYk',
        'kkkk........kkkk',
      ],
    },
    bulb: {
      colour: 0xfff08a,
      pin: ['yyy', 'yyy', '.g.'],
      icon: [
        '.....kkkkkk.....',
        '...kkyyyyyykk...',
        '..kyywwyyyyyyk..',
        '.kyywwyyyyyyyYk.',
        '.kywwyyyyyyyyYk.',
        '.kywyyyyyyyyyYk.',
        '.kyyyyyyyyyyyYk.',
        '..kyyyyyyyyyYk..',
        '...kyyyyyyyYk...',
        '....kyyYYyYk....',
        '....kkkkkkkk....',
        '....kggggGGk....',
        '....kGGGGGGk....',
        '....kggggGGk....',
        '.....kGGGGk.....',
        '......kkkk......',
      ],
    },
    headset: {
      colour: 0x6fd6cc,
      pin: ['GGG', 'G.G', 't.t'],
      icon: [
        '.....kkkkkk.....',
        '...kkGGGGGGkk...',
        '..kGGkkkkkkGGk..',
        '.kGkk......kkGk.',
        '.kGk........kGk.',
        'kGk..........kGk',
        'kkkk........kkkk',
        'kttTk......kTttk',
        'kttTk......kTttk',
        'kttTk......kTttk',
        'kttTk......kTttk',
        'kkkkk......kkkkk',
        '.kGk.........kk.',
        '..kGk.......kk..',
        '...kGGGkkkkkrrk.',
        '....kkkk...krrk.',
      ],
    },
    flag: {
      colour: 0xff6b5e,
      pin: ['Nrr', 'Nrr', 'N..'],
      icon: [
        '.kk.............',
        'kNNk............',
        'kNNkkkkk...kkk..',
        'kNNkrrrrkkkrrrk.',
        'kNNkrwrrrrrrrrk.',
        'kNNkrwrrrrrrrrk.',
        'kNNkrrrrrrrrrRk.',
        'kNNkrrrrrrrrRRk.',
        'kNNkrrrrrrrRRk..',
        'kNNkRRkkkkRRk...',
        'kNNkkk....kk....',
        'kNNk............',
        'kNNk............',
        'kNNk............',
        'kNNk............',
        '.kk.............',
      ],
    },
    shield: {
      colour: 0x7aa8ff,
      pin: ['bwb', 'bbb', '.b.'],
      icon: [
        '.kkkkkkkkkkkkkk.',
        'kbbbbbbwwbbbbbBk',
        'kbwbbbbwwbbbbbBk',
        'kbwbbbbwwbbbbbBk',
        'kbbbbbbwwbbbbbBk',
        'kwwwwwwwwwwwwwwk',
        'kwwwwwwwwwwwwwwk',
        'kbbbbbbwwbbbbbBk',
        'kbbbbbbwwbbbbbBk',
        '.kbbbbbwwbbbbBk.',
        '.kbbbbbwwbbbbBk.',
        '..kbbbbwwbbbBk..',
        '...kbbbwwbbBk...',
        '....kbbwwbBk....',
        '.....kkBBkk.....',
        '.......kk.......',
      ],
    },
    rocket: {
      colour: 0xff9a4a,
      pin: ['.r.', 'www', 'r.r'],
      icon: [
        '.......kk.......',
        '......krrk......',
        '.....krrrRk.....',
        '.....kkkkkk.....',
        '.....kwwwgk.....',
        '....kwwkkwgk....',
        '....kwkcbkgk....',
        '....kwkbBkgk....',
        '....kwwkkwgk....',
        '...kkwwwwwgkk...',
        '..krkwwwwwgkrk..',
        '.krrkwwwwwgkrRk.',
        '.krkkkkkkkkkkRk.',
        '.kk..kyoyok..kk.',
        '......kyok......',
        '.......kk.......',
      ],
    },
    coffee: {
      colour: 0xe0b07a,
      pin: ['NNN', 'wwg', 'ww.'],
      icon: [
        '......g....g....',
        '.....g....g.....',
        '.....g....g.....',
        '......g....g....',
        '................',
        '.kkkkkkkkkkk....',
        'kwNNNNNNNNNwk...',
        'kwwwwwwwwwwgkkk.',
        'kwwwwwwwwwwgkGgk',
        'kbbbbbbbbbbBk.gk',
        'kBBBBBBBBBBBk.gk',
        'kwwwwwwwwwwgkGgk',
        'kwwwwwwwwwwgkkk.',
        '.kwwwwwwwwgk....',
        '..kggggggGk.....',
        '...kkkkkkk......',
      ],
    },
    heart: {
      colour: 0xff8fb3,
      pin: ['p.r', 'rrr', '.r.'],
      icon: [
        '................',
        '..kkkk....kkkk..',
        '.kpppRk..kRrrrk.',
        'krwwrrRkkRrrrrRk',
        'krwrrrrrrrrrrrRk',
        'krrrrrrrrrrrrrRk',
        'krrrrrrrrrrrrrRk',
        '.krrrrrrrrrrrRk.',
        '..krrrrrrrrrRk..',
        '...krrrrrrrRk...',
        '....krrrrrRk....',
        '.....krrrRk.....',
        '......krRk......',
        '.......kk.......',
        '................',
        '................',
      ],
    },
  };
  const BADGE_IDS = Object.keys(BADGES);
  /** The hire's own, for Anyone: a head and shoulders in its blue suit and orange tie. */
  const ANYONE_ICON = [
    '................',
    '.....kkkkkk.....',
    '....kNNNNNNk....',
    '...kNNNNNNNNk...',
    '...kNssssssNk...',
    '...kssssssssk...',
    '...kskssssksk...',
    '...kssssssssk...',
    '....kssmmssk....',
    '.....kssssk.....',
    '...kkkwwwwkkk...',
    '..kbbbwoowbbbk..',
    '.kbbbbwoowbbbbk.',
    '.kbbbbbobbbbbbk.',
    '.kbbbbbobbbbbbk.',
    '.kkkkkkkkkkkkkk.',
  ];
  /**
   * The roles a held empty desk offers, along the foot of the room where the
   * keys go: each chip's height, the gaps, the strip's inset, and the space
   * either side of a name.
   */
  const ROLE_ROW = { h: 20, gap: 2, inset: 3, pad: 4 };
  /** A badge's icon, or Anyone's, on a canvas of its own, `scale` screen pixels to each of its pixels. */
  function paintIcon(rows, scale) {
    const canvas = document.createElement('canvas');
    canvas.width = canvas.height = 16 * scale;
    const ctx = canvas.getContext('2d');
    rows.forEach((row, r) => [...row].forEach((ch, c) => {
      if (ch === '.') return;
      ctx.fillStyle = css(BADGE_INKS[ch]);
      ctx.fillRect(c * scale, r * scale, scale, scale);
    }));
    return canvas;
  }
  const iconsPainted = new Map();
  /** A role's icon, painted once: its badge's, or the hire's for Anyone; none for a badge the office doesn't know. */
  function roleIcon(role) {
    const rows = role.id === null ? ANYONE_ICON : role.badge && Object.hasOwn(BADGES, role.badge) ? BADGES[role.badge].icon : null;
    if (!rows) return null;
    const key = role.id === null ? '' : role.badge;
    if (!iconsPainted.has(key)) iconsPainted.set(key, paintIcon(rows, 1));
    return iconsPainted.get(key);
  }

  /** How long someone just hired takes to get to what they're there for, once sat down, in seconds. */
  const HIRE_MOMENT = 2.5;
  /** A session's role's badge, if it has one the office knows. */
  const badgeOf = (session) => (session && session.role && Object.hasOwn(BADGES, session.role.badge) ? session.role.badge : null);

  const MUSIC_ACTIONS = ['previous', 'playpause', 'next'];
  const MUSIC_PANEL_LENGTH = 6;
  /** Play, pause, previous and next, in pixels. */
  const MUSIC_ICONS = {
    play: ['100', '110', '111', '110', '100'],
    pause: ['101', '101', '101', '101', '101'],
    previous: ['1..1..1', '1.11.11', '1111111', '1.11.11', '1..1..1'],
    next: ['1..1..1', '11.11.1', '1111111', '11.11.1', '1..1..1'],
  };
  const STYLE_MARK = ['1111110', '1111110', '0000010', '0011110', '0010000', '0010000', '0010000'];
  const STYLE_THUMB = { w: 62, h: 34 };
  const STYLE_GAP = 4;
  const STYLE_LABEL = 8;
  const STYLE_INSET = 6;
  const STYLE_TITLE = 14;
  const STYLE_IDS = ['yours', ...Object.keys(ROOM_STYLES)];
  const styleName = (id) => (id === 'yours' ? 'Yours' : ROOM_STYLES[id].name);

  /** The styles' panel, in the middle of the room: four pictures to a row, or three held upright. */
  function styleMenu() {
    const columns = shape === TALL ? 3 : 4;
    const rows = Math.ceil(STYLE_IDS.length / columns);
    const w = STYLE_INSET * 2 + columns * STYLE_THUMB.w + (columns - 1) * STYLE_GAP;
    const h = STYLE_TITLE + STYLE_INSET + rows * (STYLE_THUMB.h + STYLE_LABEL) + (rows - 1) * STYLE_GAP;
    const left = Math.round(shape.width / 2 - w / 2), top = Math.round(shape.height / 2 - h / 2);
    return {
      left, top, w, h,
      tiles: STYLE_IDS.map((id, i) => ({
        id,
        x: left + STYLE_INSET + (i % columns) * (STYLE_THUMB.w + STYLE_GAP),
        y: top + STYLE_TITLE + Math.floor(i / columns) * (STYLE_THUMB.h + STYLE_LABEL + STYLE_GAP),
        w: STYLE_THUMB.w, h: STYLE_THUMB.h + STYLE_LABEL,
      })),
    };
  }

  /** The choice keys for the agent at `desk`, or in the line for it: a panel just above its head, pointing down at it, kept inside the room. */
  function choicePanel(desk) {
    const w = CHOICE_INSET * 2 + CHOICE_KEY.w * 4 + CHOICE_GAP * 2 + CHOICE_SPLIT;
    const h = CHOICE_INSET * 2 + CHOICE_KEY.h;
    const line = DESKS[desk].off && crowdAnchors.get(desk);
    const tip = line ? { x: line.feet.x - 1, y: line.feet.y - 21 } : { x: seat(desk).x - 1, y: DESKS[desk].y - 15 };
    const left = Math.max(1, Math.min(shape.width - 1 - w, tip.x - Math.floor(w / 2)));
    const top = tip.y - 1 - h;
    let x = left + CHOICE_INSET;
    const keys = ['up', 'down', 'accept', 'reject'].map((action, i) => {
      const key = { action, x, y: top + CHOICE_INSET };
      x += CHOICE_KEY.w + (i === 1 ? CHOICE_SPLIT : CHOICE_GAP);
      return key;
    });
    return { left, top, w, h, tip, keys };
  }

  /** What a person is doing with their body, at `now`. */
  function poseOf(person, now) {
    if (person.isWalking) return { kind: 'walk', step: Math.floor(now * 6) % 2 };
    if (person.isGoingNuclear && person.place === 'button') return { kind: 'press', raised: person.pressedAt === null };
    if (person.place.startsWith('board:')) return { kind: 'write', stroke: Math.floor(now * 3) % 2 };
    if (person.place.startsWith('clock:')) return { kind: 'tap', beat: Math.floor(now * 4) % 2 };
    // Locked out, knocking or patting their pockets; or watching a storm. After a while, a sigh now and then.
    const sigh = now - person.arrivedAt > SIGH_AFTER && (now + person.look.phase * 8) % 8 > 7 ? 1 : 0;
    if (person.place.startsWith('crowd:')) {
      if (person.session.state !== 'asking') return { kind: 'laptop', beat: Math.floor(now * 8 + person.look.phase * 8) % 2 };
      const agitation = agitationOf(person, now);
      return { kind: 'wave', frame: Math.floor(now * (4 + 8 * agitation)) % 2, hop: agitation ? Math.floor(now * (3 + 5 * agitation)) % 2 : 0 };
    }
    if (person.place.startsWith('locked:')) return { kind: takeOf(person) === 0 ? 'knock' : 'pockets', beat: Math.floor(now * 5) % 4 < 2 ? 0 : 1, sigh };
    if (person.place.startsWith('storm:')) return { kind: 'stand', sigh };
    if (person.place.startsWith('shred:')) return { kind: 'shred' };
    if (deskOf(person.place) !== null) {
      if (person.celebratedAt !== null && now - person.celebratedAt < PARTY_LENGTH) return { kind: 'cheer', frame: Math.floor(now * 6) % 2 };
      if (person.session.state === 'asking') {
        // Kept waiting, they wave harder and start to hop.
        const agitation = agitationOf(person, now);
        return { kind: 'wave', frame: Math.floor(now * (4 + 8 * agitation)) % 2, hop: agitation ? Math.floor(now * (3 + 5 * agitation)) % 2 : 0 };
      }
      // Out of order, they hold up an empty wallet, or put their head in their hands.
      if (person.session.state === 'error') return failureOf(person.session) === 'billing' && takeOf(person) === 0 ? { kind: 'wallet' } : { kind: 'slump' };
      return { kind: WORKSPACES[workspaceOf(person.session, deskOf(person.place))].stands ? 'stand' : 'sit' };
    }
    const spot = spotOf(person.place);
    if (spot !== null) {
      const { pose } = spots[spot];
      if (pose === 'walk') return { kind: 'walk', step: Math.floor(now * 6) % 2 };
      return { kind: pose === 'sit' || pose === 'perch' ? 'sit' : 'stand' };
    }
    return { kind: 'stand' };
  }

  /** The top row of a person's hair. Sitting lowers it; standing at the desk to wave raises them back up. */
  function topOf(person, pose) {
    const feet = Math.round(person.position.y);
    if (pose.kind === 'sit' || pose.kind === 'wallet') return feet - 15;
    if (pose.kind === 'slump') return feet - 14;
    return feet - 17 - (pose.hop || 0) + (pose.sigh || 0);
  }

  /**
   * Where the room goes on a canvas `width` × `height`, at least `margin`
   * from its edges: a whole number of the screen's pixels to each art pixel
   * keeps every edge sharp, and what's left over is a border round it.
   */
  function fitRoom(width, height, margin = 0) {
    const fit = Math.min((width - margin * 2) / shape.width, (height - margin * 2) / shape.height);
    const scale = fit >= 1 ? Math.floor(fit) : fit;
    return { scale, ox: Math.round((width - shape.width * scale) / 2), oy: Math.round((height - shape.height * scale) / 2) };
  }

  /**
   * Beside the phone's controls, the room is seen through a window: its
   * corners rounded like the deck's keys, a shadow inside its edge, and the
   * edge itself. In CSS pixels.
   */
  const WINDOW_CORNER = 16;
  const WINDOW_SHADOW = 10;
  /** The shadow at the window's edge, fading to nothing WINDOW_SHADOW in. */
  const WINDOW_SHADOW_DEPTH = 0.32;

  /**
   * Floating, the room's window fills its space but for a margin of the
   * deck's background, with a soft shadow under it, and the office goes on
   * past the room to the window's edges. The margin is wider in the pop-out
   * window, which has its own edge round it, than on a whole screen, where
   * every pixel counts, and there is none as the Mac's screen saver: the
   * office goes from edge to edge, with square corners. In CSS pixels.
   */
  const FLOAT_MARGINS = { window: 14, screen: 6, edge: 0 };
  /**
   * How far the office goes on past the room, at most: half the room's
   * width or height each way. Beyond that, as on a phone held upright, it
   * would be bare wall and floor.
   */
  const FLOAT_REACH = 1.5;
  const FLOAT_SHADOW = 16;
  const FLOAT_DROP = 3;
  /**
   * While an agent waits on you, the floating window's edge glows in the
   * waiting colour, rising and falling once in this many seconds: the
   * pop-out is mostly seen out of the corner of an eye.
   */
  const GLOW_PERIOD = 1.6;
  const GLOW_REACH = 14;

  /**
   * Draws one frame of the office: the room, then everything on the floor
   * from back to front, then what floats above it: the screens' light, the
   * speech bubbles and the name on each desk.
   */
  function drawOffice(ctx, office, frame) {
    const { now, hour, minute, month, width, height, ratio, selected, mic, framed, floating } = frame;
    const inset = floating ? FLOAT_MARGINS[floating] * ratio : 0;
    const room = fitRoom(width, height, inset);
    // The scale and the origin change for a while as a piece grown bigger is drawn: see `moved`.
    let { scale, ox, oy } = room;

    // The room shakes for a moment when the button goes down.
    const blast = office.detonation;
    const sinceBlast = blast ? now - blast.at : Infinity;
    if (sinceBlast >= 0 && sinceBlast < SHAKE_LENGTH) {
      const amplitude = (1 - sinceBlast / SHAKE_LENGTH) * 1.5;
      ox += Math.round(Math.sin(sinceBlast * 90) * amplitude) * scale;
      oy += Math.round(Math.cos(sinceBlast * 71) * amplitude) * scale;
    }

    /** Fills art pixels: whole squares of the room. `color` is a hex number or [hex, alpha]. */
    function fill(x, y, w, h, color, alpha = 1) {
      const [hex, a] = Array.isArray(color) ? color : [color, alpha];
      if (a <= 0) return;
      ctx.fillStyle = css(hex, a);
      // Edges on whole screen pixels, so squares side by side meet, even half as big again
      const left = Math.round(ox + Math.round(x) * scale), top = Math.round(oy + Math.round(y) * scale);
      ctx.fillRect(left, top, Math.round(ox + (Math.round(x) + Math.round(w)) * scale) - left,
        Math.round(oy + (Math.round(y) + Math.round(h)) * scale) - top);
    }
    const dot = (x, y, color, alpha) => fill(x, y, 1, 1, color, alpha);
    /** Where an art pixel lands on the canvas, for what's drawn at the screen's own resolution. */
    const screen = (x, y) => ({ x: ox + x * scale, y: oy + y * scale });

    const pixelWidth = textWidth;

    /** A few digits in art pixels, on a dark backing unless what's under them is dark already. */
    function pixelText(text, x, y, color, backing = true) {
      if (backing) fill(x - 1, y - 1, pixelWidth(text) + 2, 7, PALETTE.frame, 0.75);
      let at = x;
      for (const c of text) {
        GLYPHS[c].forEach((row, r) => [...row].forEach((on, column) => { if (on === '1') dot(at + column, y + r, color); }));
        at += GLYPHS[c][0].length + 1;
      }
    }

    /** A round thing of radius `r`, filled, row by row. */
    function disc(cx, cy, r, color, alpha = 1) {
      for (let row = -Math.floor(r); row <= Math.floor(r); row++) {
        const half = Math.floor(Math.sqrt(r * r - row * row));
        fill(cx - half, cy + row, half * 2 + 1, 1, color, alpha);
      }
    }
    /** The rim of a round thing of radius `r`, a dot at a time. */
    function ring(cx, cy, r, color, alpha = 1) {
      const steps = Math.ceil(r * 8);
      for (let k = 0; k < steps; k++) {
        const a = (k / steps) * Math.PI * 2;
        dot(cx + Math.round(Math.cos(a) * r), cy + Math.round(Math.sin(a) * r), color, alpha);
      }
    }
    /** A line of dots from one point to another. */
    function line(x0, y0, x1, y1, color, alpha = 1) {
      const steps = Math.max(Math.abs(x1 - x0), Math.abs(y1 - y0)) || 1;
      for (let k = 0; k <= steps; k++) dot(Math.round(x0 + ((x1 - x0) * k) / steps), Math.round(y0 + ((y1 - y0) * k) / steps), color, alpha);
    }
    /*
     * What the calendar brings to a room furnished for it, as the seasons
     * come to the windows: each touch goes with something in the room, in
     * its months. Drawn on the wall, on the floor, in the air over
     * everything, or glowing over the night; or over a piece of the back
     * wall (`over`), or in its place (`instead`).
     */
    const has = (piece) => hungOf().some((p) => p.kind === piece);
    const hung = (piece) => hungOf().flatMap((p) => (p.kind === piece ? [p.x] : []));
    /** The office's floor, to the lounge. */
    const officeFloor = () => ({
      left: seen.left, right: shape === WIDE ? LOUNGE_LEFT : seen.right, top: WALL_BOTTOM, bottom: shape === WIDE ? seen.bottom : shape.loungeWall,
    });
    const LEAVES = [0xd9893a, 0xb8562a, 0xe0b040, 0xc0402a];
    /** Snow falling through the room, or through `area`. */
    const snowfall = (count, area = seen) => {
      const w = area.right - area.left, h = area.bottom - area.top;
      for (let k = 0; k < count; k++) {
        const x = area.left + (((k * 37 + Math.round(Math.sin(now + k) * 2)) % w) + w) % w;
        dot(x, Math.round(area.top + ((now * 6 + k * 13) % h)), WHITE, 0.85);
      }
    };
    /** Snow on the desks, either side of the monitor, `depth` deep. */
    const snowOnDesks = (depth) => {
      for (const { x, y, off } of DESKS) {
        if (off) continue;
        fill(x + 1, y - depth, 5, depth, 0xf2f6fa);
        fill(x + 34, y - depth, 9, depth, 0xf2f6fa);
      }
    };
    /** Icicles from `y` all along the wall, `grow` long and a little more, dripping as they melt. */
    const icicles = (y, grow, step, dripping) => {
      for (let x = from(seen.left, step, 3); x < seen.right; x += step) {
        const length = grow + (((x * 7) % 3) + 3) % 3;
        fill(x, y, 1, length, 0x7aa8c8);
        dot(x, y, WHITE);
        dot(x, y + length - 1, 0x5a8ab0);
        if (dripping) {
          const fall = (now * 0.8 + x * 0.13) % 1;
          dot(x, y + length + Math.round(fall * 8), 0x8fc6f0, 1 - fall);
        }
      }
    };
    /** A garland swung along the wall at `y`, `drop` deep, in loops `span` wide. */
    const garland = (y, drop, span, paint) => {
      for (let x = seen.left; x < seen.right; x++) paint(x, y + Math.round(Math.abs(Math.sin((x * Math.PI) / span)) * drop));
    };

    const SEASONAL_TOUCHES = [
      // Arctic: snow building up from December to February; icicles growing, dripping in March
      { months: [11, 0, 1], when: () => decor.floor === 'snow' || decor.paint === 'ice',
        floor() {
          const deep = { 11: 1, 0: 2, 1: 4 }[month];
          for (let x = from(seen.left, 2); x < seen.right; x += 2) {
            const h = Math.max(0, deep + Math.round(Math.sin(x * 0.3) * 0.8 + Math.sin(x * 0.11 + 1)));
            if (!h) continue;
            fill(x, WALL_BOTTOM, 2, h, 0xf2f6fa);
            fill(x, WALL_BOTTOM + h - 1, 2, 1, 0xd2e0ea);
          }
        },
        air() { snowOnDesks({ 11: 1, 0: 2, 1: 3 }[month]); snowfall(24); } },
      { months: [0, 1, 2], when: () => decor.paint === 'ice', wall() { icicles(31, { 0: 1, 1: 3, 2: 3 }[month], 6, month === 2); } },
      // Campsite: leaves piling up through the autumn; snow in winter; fireflies on summer nights
      { months: [8, 9, 10], when: () => decor.floor === 'forest' || decor.paint === 'logs',
        floor() {
          const area = officeFloor(), w = area.right - area.left, pile = { 8: 60, 9: 140, 10: 260 }[month];
          for (let k = 0; k < pile; k++) {
            const h = hash(k, 7);
            fill(area.left + (h % w), area.top + 1 + ((h >>> 8) % (k % 3 ? 4 : area.bottom - area.top - 2)), 2, 1, LEAVES[(h >>> 4) % 4]);
          }
        },
        air() {
          for (let k = 0; k < 16; k++) {
            const fall = (now * 0.15 + k / 16) % 1, x = seen.left + ((k * 29) % seen.width) + Math.round(Math.sin(fall * 9 + k) * 4);
            fill(x, Math.round(seen.top + fall * seen.height), 2, 1, LEAVES[k % 4]);
          }
        } },
      { months: [11, 0, 1], when: () => decor.floor === 'forest' || decor.paint === 'logs',
        wall() { for (const x of [...hung('cabin'), ...hung('window')]) fill(x - 16, 27, 32, 2, 0xf2f6fa); },
        floor() { fill(seen.left, WALL_BOTTOM, seen.width, 1, 0xf2f6fa); },
        air() { snowOnDesks(2); snowfall(20); } },
      { months: [5, 6, 7], when: () => decor.floor === 'forest',
        glow() {
          if (!isNight) return;
          for (let k = 0; k < 14; k++) {
            const x = Math.round(seen.left + seen.width / 2 + Math.sin(now * 0.4 + k * 1.7) * seen.width * 0.42);
            const y = Math.round(WALL_BOTTOM + (seen.bottom - WALL_BOTTOM) / 2 + Math.cos(now * 0.3 + k * 2.3) * (seen.bottom - WALL_BOTTOM) * 0.4);
            const a = Math.max(0, Math.sin(now * 2 + k * 1.3));
            if (a < 0.1) continue;
            dot(x, y, 0xe8f870, a);
            fill(x - 1, y - 1, 3, 3, 0xe8f870, 0.15 * a);
          }
        } },
      // Space station: a tree floating about in December; the Earth's seasons in a porthole all year
      { months: [11], when: () => decor.paint === 'hull' || decor.floor === 'plating',
        air() {
          const tx = 110 + Math.round(Math.sin(now * 0.3) * 60), ty = 64 + Math.round(Math.cos(now * 0.4) * 8), lean = Math.round(Math.sin(now * 0.5));
          for (let r = 0; r < 7; r++) {
            const half = Math.floor(r / 2) + 1;
            fill(tx - half + (r < 3 ? lean : 0), ty + r, half * 2 + 1, 1, 0x2f7a4a);
          }
          fill(tx, ty + 7, 1, 2, 0x6e4a2e);
          dot(tx + lean, ty - 1, 0xf2d35b);
          for (const [dx, dy, c] of [[-1, 2, 0xd4140b], [1, 4, 0x4c8bf5], [-2, 5, 0xf2d35b], [2, 6, 0xd4140b]]) dot(tx + dx, ty + dy, c);
          for (let k = 0; k < 3; k++) {
            const bx = 70 + k * 40 + Math.round(Math.sin(now * 0.6 + k) * 6), by = 56 + Math.round(Math.cos(now * 0.5 + k * 2) * 6);
            fill(bx, by, 2, 2, [0xd4140b, 0xf2d35b, 0x4c8bf5][k]);
            dot(bx, by, WHITE, 0.6);
          }
        } },
      { months: [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11], piece: 'porthole',
        over(x) {
          const px = x + 1, py = 18, season = SEASONS[month];
          disc(px, py, 4.5, 0x3f7fc0);
          const land = { winter: 0x4f9a5a, spring: 0x5fba5a, summer: 0xb8b040, autumn: 0xd9893a }[season];
          for (const [dx, dy] of [[-2, -1], [-1, -1], [-1, 0], [0, 0], [1, 1], [2, 1], [2, 2], [-3, 2], [0, 3]]) dot(px + dx, py + dy, land);
          if (season === 'winter') {
            fill(px - 2, py - 4, 5, 1, WHITE);
            fill(px - 3, py - 3, 7, 1, WHITE);
            fill(px - 4, py - 2, 3, 1, WHITE, 0.8);
          }
          fill(px - 2 + Math.round(Math.sin(now * 0.7) * 2), py + 1, 3, 1, WHITE, 0.6);
        } },
      // Pirate cove: an autumn storm at sea; ice on the rigging in winter
      { months: [9, 10], piece: 'seaPorthole',
        over(x) {
          const cx = x, cy = 16;
          for (let row = -7; row <= 7; row++) {
            const half = Math.floor(Math.sqrt(49 - row * row));
            fill(cx - half, cy + row, half * 2 + 1, 1, row <= 0 ? 0x3a4252 : 0x1f3a52);
          }
          for (let row = 1; row <= 6; row++) {
            const half = Math.floor(Math.sqrt(49 - row * row));
            for (let k = -half; k <= half; k++) if ((((k + row * 2 + Math.floor(now * 6)) % 4) + 4) % 4 === 0) dot(cx + k, cy + row - (k % 2 ? 1 : 0), WHITE, 0.7);
          }
          for (let k = 0; k < 10; k++) {
            const rx = cx - 6 + ((k * 5 + Math.floor(now * 25)) % 13), ry = cy - 6 + ((k * 3 + Math.floor(now * 30)) % 12);
            if (Math.hypot(rx - cx, ry - cy) < 7) dot(rx, ry, 0xa8c0d8, 0.8);
          }
        } },
      { months: [9, 10], when: () => has('seaPorthole'), air() { if ((now % 6) < 0.12) fill(seen.left, seen.top, seen.width, seen.height, WHITE, 0.1); } },
      { months: [11, 0, 1], when: () => decor.paint === 'timber',
        wall() { icicles(32, 1, 5, false); },
        floor() {
          const area = officeFloor(), w = area.right - area.left, h = area.bottom - area.top;
          for (let k = 0; k < 120; k++) { const r = hash(k, 3); dot(area.left + (r % w), area.top + 1 + ((r >>> 8) % (h - 1)), 0xf2f6fa, 0.7); }
        } },
      // Royal court: holly on the rail through Christmas; garlands of flowers in spring
      { months: [11, 0], when: () => decor.paint === 'damask',
        wall() {
          garland(31, 2, 20, (x, y) => {
            dot(x, y, x % 3 ? 0x2f6a3a : 0x3f8a4a);
            if (x % 4 === 0) dot(x, y - 1, 0x2f6a3a);
            if (((x % 10) + 10) % 10 === 5) dot(x, y + 1, 0xd4140b);
          });
          for (let x = from(seen.left, 40); x < seen.right; x += 40) fill(x, 30, 1, 2, 0xd4140b);
        } },
      { months: [3, 4], when: () => decor.paint === 'damask',
        wall() {
          garland(3, 3, 23, (x, y) => {
            dot(x, y, 0x3f8a4a);
            if (((x % 5) + 5) % 5 === 2) dot(x, y + 1, [0xf6b8cc, WHITE, 0xf2d35b][((x % 3) + 3) % 3]);
          });
        },
        air() {
          for (let k = 0; k < 10; k++) {
            const fall = (now * 0.12 + k / 10) % 1;
            dot(seen.left + ((k * 41) % seen.width) + Math.round(Math.sin(fall * 8 + k) * 3), Math.round(seen.top + 6 + fall * seen.height), 0xf6b8cc, 1 - fall * 0.5);
          }
        } },
      // Dragon's lair: cobwebs and jack-o'-lanterns in October; mushrooms in the stones in spring
      { months: [9], when: () => decor.paint === 'stone' || decor.floor === 'flagstone',
        wall() {
          for (const [cx, dir] of [[seen.left, 1], [seen.right - 1, -1]]) {
            for (let r = 3; r <= 12; r += 3) for (let a = 0; a <= 8; a++) {
              const angle = (a / 8) * (Math.PI / 2);
              dot(cx + dir * Math.round(Math.cos(angle) * r), Math.round(Math.sin(angle) * r), WHITE, 0.4);
            }
            for (const angle of [0.2, 0.8, 1.3]) line(cx, 0, cx + dir * Math.round(Math.cos(angle) * 13), Math.round(Math.sin(angle) * 13), WHITE, 0.35);
          }
          const sx = seen.left + 9, sy = 14 + Math.round(Math.sin(now * 1.2) * 3);
          line(sx, 0, sx, sy, WHITE, 0.4);
          fill(sx - 1, sy, 3, 2, 0x1a1a1f);
          dot(sx - 2, sy + 1, 0x1a1a1f);
          dot(sx + 2, sy + 1, 0x1a1a1f);
        },
        floor() {
          for (const [px, py] of [[DESKS[6].x - 10, DESKS[6].y + 16], [DESKS[2].x + DESK_WIDTH + 4, DESKS[2].y + 16]]) {
            const flicker = 0.7 + 0.3 * Math.sin(now * 9 + px);
            fill(px - 1, py - 6, 9, 9, 0xff8a3d, 0.1 * flicker);
            fill(px, py - 4, 7, 5, 0xe0702a);
            fill(px + 1, py - 5, 5, 1, 0xe0702a);
            dot(px + 3, py - 6, 0x3f8a4a);
            fill(px + 3, py - 4, 1, 5, 0xc05a20);
            dot(px + 1, py - 3, 0xf2d35b, flicker);
            dot(px + 5, py - 3, 0xf2d35b, flicker);
            fill(px + 2, py - 1, 3, 1, 0xf2d35b, flicker);
          }
        } },
      { months: [3, 4, 5], when: () => decor.floor === 'flagstone',
        wall() { for (let x = from(seen.left, 7); x < seen.right; x += 7) fill(x + (((x % 3) + 3) % 3), 37, 3, 1, 0x3b5a3a); },
        floor() {
          const area = officeFloor(), w = area.right - area.left, h = area.bottom - area.top;
          for (let k = 0; k < 24; k++) {
            const r = hash(k, 11), x = area.left + 2 + (r % (w - 4)), y = area.top + 6 + ((r >>> 8) % (h - 8)), glow = k % 3 === 0;
            fill(x, y - 1, 1, 2, 0xeae6dc);
            fill(x - 1, y - 2, 3, 1, glow ? 0x4ad4e8 : 0xc9443a);
            if (glow) fill(x - 2, y - 4, 5, 4, 0x4ad4e8, 0.1 + 0.05 * Math.sin(now * 2 + k));
            else dot(x, y - 2, WHITE);
            dot(x + 1, y, 0x3b5a3a);
            dot(x - 2, y, 0x4f7a3a);
          }
        } },
      // Gamer den: Christmas lights in December, on the strip and the TV; a neon pumpkin in October
      { months: [11], when: () => decor.paint === 'foam',
        wall() {
          for (let x = seen.left; x < seen.right; x++) {
            const c = (Math.floor(x / 4) + Math.floor(now * 4)) % 2 ? 0xd4140b : 0x2fa04a;
            dot(x, 2, c);
            dot(x, 3, c, 0.25);
          }
        } },
      { months: [11], piece: 'tv',
        over(x) {
          const l = x - 13, t = 8;
          fill(l, t, 26, 15, 0x0b1022);
          for (let r = 0; r < 9; r++) { const half = Math.floor(r / 2) + 1; fill(x - half, t + 3 + r, half * 2 + 1, 1, 0x2fa04a); }
          dot(x, t + 2, 0xf2d35b, 0.6 + 0.4 * Math.sin(now * 4));
          fill(x, t + 12, 1, 2, 0x6e4a2e);
          for (const [dx, dy, c] of [[-1, 5, 0xd4140b], [2, 7, 0x4c8bf5], [-3, 9, 0xf2d35b], [3, 10, 0xd4140b]]) dot(x + dx, t + dy, c);
          for (let k = 0; k < 5; k++) dot(l + 2 + k * 5, t + Math.floor((now * 4 + k * 3) % 14), WHITE, 0.7);
        } },
      { months: [9], piece: 'neon',
        instead(x) {
          const tube = (px, py, c) => { fill(px - 1, py - 1, 3, 3, c, 0.1); dot(px, py, lighter(c)); };
          ['..XXXXX..', '.X.....X.', 'X.X...X.X', 'X.......X', 'X..XXX..X', '.X.....X.', '..XXXXX..']
            .forEach((row, r) => [...row].forEach((c, k) => { if (c === 'X') tube(x - 4 + k, 10 + r, 0xff8a3d); }));
          tube(x, 9, 0x6fd36f);
          tube(x + 1, 8, 0x6fd36f);
          const bx = x + 8 + Math.round(Math.sin(now * 2) * 2), wing = Math.floor(now * 6) % 2;
          for (const [dx, dy] of [[0, 0], [-1, wing - 1], [1, wing - 1]]) tube(bx + dx, 8 + dy, 0xc05aff);
        } },
      // Startup garage: snow on the drive in winter, a heater on; leaves blowing in in autumn
      { months: [11, 0, 1], piece: 'roller',
        over(x) {
          const l = x - 15, w = 30;
          fill(l, 31, w, 7, 0xf2f6fa);
          fill(l, 27, w, 1, 0xf2f6fa);
          fill(x + 3, 29, 10, 1, 0xf2f6fa);
          fill(x + 5, 27, 6, 1, 0xf2f6fa);
          snowfall(6, { left: l, right: l + w, top: 19, bottom: 38 });
        } },
      { months: [11, 0, 1], when: () => has('roller') || decor.paint === 'brick',
        floor() {
          const hx = DESKS[0].x + DESK_WIDTH + 4, hy = DESKS[0].y + 16, glow = 0.7 + 0.3 * Math.sin(now * 3);
          fill(hx - 3, hy - 9, 12, 10, 0xff8a3d, 0.08 * glow);
          fill(hx, hy - 6, 6, 6, 0x5a5f6c);
          fill(hx + 1, hy - 5, 4, 4, 0x2a2a2e);
          for (const r of [1, 3]) fill(hx + 1, hy - 5 + r, 4, 1, 0xff6a2a, glow);
          fill(hx, hy, 6, 1, SHADOW);
        } },
      { months: [9, 10], when: () => has('roller'),
        floor() {
          for (const x of hung('roller')) for (let k = 0; k < 14; k++) { const r = hash(k, 5); fill(x - 14 + (r % 30), WALL_BOTTOM + ((r >>> 6) % 3), 2, 1, LEAVES[(r >>> 3) % 4]); }
        },
        air() {
          const area = officeFloor();
          for (const x of hung('roller')) for (let k = 0; k < 6; k++) {
            const go = (now * 0.25 + k / 6) % 1, lx = x + Math.round(go * 110);
            if (lx < area.right) fill(lx, WALL_BOTTOM + 2 + Math.round(Math.abs(Math.sin(go * 12 + k)) * 6) + k * 4, 2, 1, LEAVES[k % 4], 1 - go * 0.6);
          }
        } },
      // Fitness floor: a New Year banner in January; a fan in summer
      { months: [0], when: () => decor.paint === 'block',
        wall() {
          for (let x = from(seen.left, 6); x < seen.right; x += 6) {
            const c = [0xd4140b, 0xf2c200, 0x3f6fc4][((x / 6) % 3 + 3) % 3];
            fill(x, 2, 5, 1, c);
            fill(x + 1, 3, 3, 1, c);
            dot(x + 2, 4, c);
          }
          const bx = WALL_SLOTS[2] - 19;
          fill(bx, 1, 38, 8, 0xd4140b);
          fill(bx + 1, 2, 36, 6, 0xb8100a);
          pixelText('NEW YEAR', bx + 3, 2, WHITE, false);
        } },
      { months: [6, 7], when: () => decor.paint === 'block' || decor.floor === 'rubber',
        floor() {
          const fx = DESKS[1].x - 11, fy = DESKS[4].y;
          fill(fx - 3, fy + 16, 7, 1, 0x3a3d47);
          fill(fx, fy + 6, 1, 10, 0x5a5f6c);
          ring(fx, fy, 5, 0x9aa0a6);
          for (let k = 0; k < 3; k++) {
            const a = now * 12 + (k * Math.PI * 2) / 3;
            for (let r = 1; r <= 4; r++) dot(fx + Math.round(Math.cos(a) * r), fy + Math.round(Math.sin(a) * r), 0xc8cdd6);
          }
          dot(fx, fy, 0x3a3d47);
          for (let k = 0; k < 3; k++) for (let s = 1; s <= 6; s++) dot(fx + 5 + s, fy - 2 + k * 2 + Math.round(Math.sin(now * 14 + s + k) * 0.8), [0xd4140b, 0xf2c200, 0x3f6fc4][k]);
        } },
      // Jungle: the rainy season in spring; butterflies in summer
      { months: [3, 4], piece: 'blind',
        over(x) { for (let k = 0; k < 16; k++) dot(x - 14 + ((k * 5 + Math.floor(now * 20)) % 30), 6 + ((k * 7 + Math.floor(now * 40)) % 20), 0xa8c0d8, 0.7); } },
      { months: [3, 4], piece: 'vines',
        over(x) { for (const dx of [-13, -8, -3, 2, 7, 12]) { const fall = (now * 0.9 + dx * 0.17 + 5) % 1; dot(x + dx, 18 + Math.round(fall * 20), 0x8fc6f0, 1 - fall); } } },
      { months: [3, 4], when: () => decor.paint === 'bamboo' || has('vines') || has('blind'),
        floor() {
          const area = officeFloor();
          for (const [fx, fy] of [[0.18, 0.5], [0.55, 0.85], [0.8, 0.3]]) {
            const px = Math.round(area.left + (area.right - area.left) * fx), py = Math.round(area.top + (area.bottom - area.top) * fy);
            fill(px - 4, py, 9, 2, 0x5a8ab0, 0.6);
            fill(px - 3, py - 1, 7, 1, 0x5a8ab0, 0.4);
            const ripple = (now * 1.5 + px) % 1;
            fill(px - Math.round(ripple * 4), py, Math.round(ripple * 8) + 1, 1, 0xa8c8e0, 0.6 * (1 - ripple));
          }
        } },
      { months: [6, 7], when: () => decor.floor === 'grass',
        air() {
          for (let k = 0; k < 8; k++) {
            const x = Math.round(seen.left + seen.width / 2 + Math.sin(now * 0.5 + k * 1.9) * seen.width * 0.42);
            const y = Math.round(WALL_BOTTOM + (seen.bottom - WALL_BOTTOM) * 0.4 + Math.sin(now * 0.7 + k * 2.7) * (seen.bottom - WALL_BOTTOM) * 0.3);
            const open = Math.floor(now * 8 + k) % 2, c = [0xf2d35b, 0x4c8bf5, 0xe0507a, 0xff8a3d][k % 4];
            dot(x, y, 0x1a1a1f);
            dot(x - 1, y - 1, c);
            dot(x + 1, y - 1, c);
            if (open) { dot(x - 1, y, c); dot(x + 1, y, c); }
          }
        } },
    ];

    /** The touches the calendar brings to the room as it's furnished, for `layer`: 'wall', 'floor', 'air' or 'glow'. */
    function drawSeasonal(layer) {
      if (decor.season !== 'calendar') return;
      for (const touch of SEASONAL_TOUCHES) if (touch[layer] && touch.months.includes(month) && (!touch.when || touch.when())) touch[layer]();
    }

    /** Draws only inside `rects`, each [x, y, w, h], leaving the rest as it was. */
    function within(rects, draw) {
      ctx.save();
      ctx.beginPath();
      for (const [x, y, w, h] of rects) ctx.rect(ox + x * scale, oy + y * scale, w * scale, h * scale);
      ctx.clip();
      try {
        draw();
      } finally {
        ctx.restore();
      }
    }

    // The sky keeps the computer's time, or it's always the middle of the day.
    const daylight = decor.sky === 'day' ? 12 : hour;
    const isNight = daylight >= 20 || daylight < 5;
    const alarm = sinceBlast >= 0 && sinceBlast < ALARM_LENGTH ? sinceBlast : null;
    const colour = (state) => fromCss(office.colours[state]) ?? fromCss(STANDARD_STATES[state]);
    const ink = fromCss(office.colours.ink) ?? INK;
    const sessionAt = (desk) => (desk < office.seats.length ? office.seats[desk].session : null);
    /** What most desks give their agents to wear, for the office's staff: null where most are desks of no kind of their own. */
    const crewDress = (() => {
      const counts = new Map();
      DESKS.forEach((d, desk) => {
        if (d.off) return;
        const dress = WORKSPACE_DRESS[workspaceOf(sessionAt(desk), desk)] ?? null;
        counts.set(dress, (counts.get(dress) || 0) + 1);
      });
      return [...counts].reduce((most, entry) => (entry[1] > most[1] ? entry : most), [null, 0])[0];
    })();
    /** How far above its corner a desk's workspace reaches, in art pixels: a negative number. */
    const reachOf = (desk) => WORKSPACES[workspaceOf(sessionAt(desk), desk)].top;
    // Someone being carried is drawn where the finger has them, so nothing of them stays at their desk.
    const carried = frame.carrying && frame.carrying.person;
    /** Who is at a desk now, if anyone. */
    const occupantOf = (desk) => office.people.find((p) => p.desk === desk && p.isAtDesk && p !== carried) || null;
    const selectedDesk = office.seats.findIndex((s) => s.session && s.session.id === selected);
    /** How far a wobbling thing is pushed sideways, in art pixels. */
    const wobble = (what) => (frame.wobble && frame.wobble.what === what && now - frame.wobble.at < WOBBLE_LENGTH
      ? Math.round(Math.sin((now - frame.wobble.at) * 50)) : 0);
    const lookingUp = mic.heldAt !== null && now - mic.heldAt < LOOK_UP_LENGTH;

    ctx.clearRect(0, 0, width, height);

    // Through a window, the room stays inside it, even while it shakes.
    let glass = framed && { x: room.ox, y: room.oy, w: shape.width * scale, h: shape.height * scale, r: WINDOW_CORNER * ratio };
    if (floating) {
      // Edge to edge, bare wall and floor beat bands of nothing.
      const edge = floating === 'edge', reach = edge ? Infinity : FLOAT_REACH;
      const w = Math.min(width - inset * 2, shape.width * scale * reach);
      const h = Math.min(height - inset * 2, shape.height * scale * reach);
      glass = { x: Math.round((width - w) / 2), y: Math.round((height - h) / 2), w, h, r: edge ? 0 : WINDOW_CORNER * ratio };
    }
    // What can be seen of the office, in art pixels: the room, and floating,
    // as much of the wall and floor round it as fills the window.
    const seen = floating
      ? { left: Math.floor((glass.x - ox) / scale), top: Math.floor((glass.y - oy) / scale),
          right: Math.ceil((glass.x + glass.w - ox) / scale), bottom: Math.ceil((glass.y + glass.h - oy) / scale) }
      : { left: 0, top: 0, right: shape.width, bottom: shape.height };
    seen.width = seen.right - seen.left;
    seen.height = seen.bottom - seen.top;
    // Floating, the window glows while anyone waits on you.
    const calling = floating && office.seats.some((s) => s.session && s.session.state === 'asking');
    const glow = calling ? 0.5 - 0.5 * Math.cos((now / GLOW_PERIOD) * 2 * Math.PI) : 0;
    if (floating) drawFloat(glass);
    if (glass) {
      ctx.save();
      roundedRect(ctx, glass.x, glass.y, glass.w, glass.h, glass.r);
      ctx.clip();
    }

    /* The room */

    function sky() {
      const hour = daylight;
      if (hour >= 5 && hour < 7) return [0x4a5a9a, 0xe8b08a];
      if (hour >= 7 && hour < 17) return [0x7fb8e8, 0xa9d3f2];
      if (hour >= 17 && hour < 20) return [0x5b5aa0, 0xf0a070];
      return [0x0c1230, 0x1a2448];
    }

    function drawWindow(x, y, w, h, moon) {
      const [top, bottom] = sky();
      fill(x - 1, y - 1, w + 2, h + 2, PALETTE.frame);
      fill(x, y, w, h / 2, top);
      fill(x, y + h / 2, w, h / 2, bottom);
      if (isNight) {
        for (const [sx, sy] of [[3, 2], [11, 6], [20, 3], [26, 8], [7, 12], [17, 14]]) if (sx < w) dot(x + sx, y + sy, WHITE, 0.75);
        if (moon) fill(x + w - 9, y + 3, 3, 3, 0xf1ead2);
      } else if (daylight >= 7 && daylight < 17) {
        const drift = (x * 7) % (w - 12);
        fill(x + 3 + drift, y + 4, 7, 2, WHITE, 0.85);
        fill(x + 5 + drift, y + 3, 3, 1, WHITE, 0.85);
      }
      if (decor.season === 'calendar') drawSeason(x, y, w, h);
      fill(x + w / 2, y, 1, h, PALETTE.frame);
      fill(x, y + h / 2, w, 1, PALETTE.frame);
      fill(x - 2, y + h + 1, w + 4, 1, decor.season === 'calendar' && SEASONS[month] === 'winter' ? 0xf2f4f8 : PALETTE.sill);
    }

    /** A pane of sky, as the windows have it but without their frame: stars at night, a cloud by day. */
    function skyPane(x, y, w, h) {
      const [top, bottom] = sky();
      fill(x, y, w, Math.ceil(h / 2), top);
      fill(x, y + Math.ceil(h / 2), w, Math.floor(h / 2), bottom);
      if (isNight) {
        for (const [sx, sy] of [[3, 2], [11, 6], [20, 3], [26, 8], [7, 12], [17, 14]]) if (sx < w && sy < h) dot(x + sx, y + sy, WHITE, 0.75);
      } else if (daylight >= 7 && daylight < 17 && w > 14) {
        const drift = (x * 7) % (w - 12);
        fill(x + 3 + drift, y + 4, 7, 2, WHITE, 0.85);
        fill(x + 5 + drift, y + 3, 3, 1, WHITE, 0.85);
      }
    }

    /** What the season puts in a window: snow, falling leaves or blossom, or a bird crossing now and then in summer. */
    function drawSeason(x, y, w, h) {
      const season = SEASONS[month];
      if (season === 'summer') {
        const lap = ((now + x * 0.37) % BIRD_EVERY) / BIRD_CROSSING;
        const bx = x - 2 + Math.round(lap * (w + 4)), by = y + 4 + Math.round(Math.sin(lap * 6));
        if (lap < 1 && bx > x && bx < x + w - 3) for (const [dx, dy] of [[0, 0], [1, 1], [2, 0]]) dot(bx + dx, by + dy, PALETTE.frame);
        return;
      }
      const falling = { winter: [WHITE], autumn: [0xd9893a, 0xb8562a, 0xe0b040], spring: [0xf6b8cc, WHITE] }[season];
      const speed = season === 'winter' ? 5 : 8, sway = season === 'winter' ? 1 : 2;
      for (let k = 0; k < Math.round(w / 5); k++) {
        const px = x + ((k * 11 + x * 3) % w) + Math.round(Math.sin(now * 1.5 + k) * sway);
        const py = y - 1 + ((now * speed + k * 7.3 + x) % (h + 2));
        if (px >= x && px < x + w && py >= y && py < y + h) dot(px, py, falling[k % falling.length]);
      }
    }

    /** The door stands open while someone passes through it. */
    function drawDoor() {
      // Someone locked out in front of it doesn't open it.
      const lockedOut = (p) => !p.isWalking && !p.isLeaving && p.place.startsWith('locked:');
      const open = office.people.some((p) => !lockedOut(p) && Math.hypot(p.position.x - DOOR.x, p.position.y - DOOR.y) < 14);
      fill(205, 13, 18, 27, PALETTE.frame);
      if (open) {
        fill(207, 15, 14, 25, PALETTE.doorway);
        fill(207, 15, 3, 25, PALETTE.door);
      } else {
        fill(207, 15, 14, 25, PALETTE.door);
        fill(209, 17, 10, 9, PALETTE.doorPanel);
        fill(209, 28, 10, 10, PALETTE.doorPanel);
        fill(218, 27, 1, 2, PALETTE.brass);
      }
      // Locked out: the keypad beside it red, a padlock on it for whoever looks for their key, and knocks
      const locked = office.people.filter(lockedOut);
      if (!locked.length) return;
      fill(224, 22, 4, 6, PALETTE.frame);
      fill(225, 23, 2, 2, 0x3a3d47);
      dot(225, 26, PALETTE.alarm, 0.6 + 0.4 * Math.sin(now * 4));
      if (!open && locked.some((p) => takeOf(p) === 1)) {
        ring(218, 25, 1.5, 0x9aa0a6);
        fill(216, 26, 5, 4, PALETTE.brass);
        fill(216, 26, 5, 1, 0xf2d38a);
        dot(218, 28, PALETTE.frame);
      }
      if (!open && locked.some((p) => takeOf(p) === 0) && Math.floor(now * 5) % 4 === 1) {
        for (const [kx, ky] of [[219, 30], [220, 32], [219, 34]]) dot(kx, ky, WHITE, 0.8);
      }
    }

    /** A clock above the door, keeping the computer's time. */
    function drawClock(x, y) {
      for (const [radius, color] of [[5.5, PALETTE.frame], [4.5, PALETTE.bubble]]) {
        for (let row = -Math.floor(radius); row <= Math.floor(radius); row++) {
          const half = Math.floor(Math.sqrt(radius * radius - row * row));
          fill(x - half, y + row, half * 2 + 1, 1, color);
        }
      }
      const hand = (turns, length, color) => {
        for (let step = 1; step <= length; step++) {
          const angle = turns * 2 * Math.PI;
          dot(x + Math.round(Math.sin(angle) * step), y - Math.round(Math.cos(angle) * step), color);
        }
      };
      hand(minute / 60, 3, PALETTE.eye);
      hand(((hour % 12) + minute / 60) / 12, 2, 0xd96a5a);
      dot(x, y, PALETTE.eye);
    }

    /**
     * Draws `part` of the lounge where the room's shape puts it: just as in
     * the wide room, moved.
     */
    /**
     * Draws `part` of the lounge where the room's shape puts it, and as it's
     * arranged: moved, and bigger round its anchor, every art pixel of it so.
     * `arranged` false leaves the arrangement out.
     */
    function moved(part, draw, arranged = true) {
      const anchor = PIECE_ANCHORS[part] || { x: 0, y: 0 };
      const at = placed(part, anchor, arranged);
      const size = arranged ? pieceOf(part).size : 1;
      const kept = { ox, oy, scale };
      ox += (at.x - anchor.x * size) * scale;
      oy += (at.y - anchor.y * size) * scale;
      scale *= size;
      try {
        draw();
      } finally {
        ({ ox, oy, scale } = kept);
      }
    }

    /**
     * The walls' finishes besides paint, each drawn from `L` to `R` as the
     * back wall is, from its top down to the baseboard: its top edge, a
     * rail at 30 and the baseboard at 38, so what hangs on it sits as it
     * does on paint.
     */
    const WALL_FINISHES = {
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
    };

    /** What moves on a floor, over it as it's kept: where the snow glints, each `h` setting when, from `L`, `T` to `R`, `B`. */
    const FLOOR_GLINTS = {
      snow(L, T, R, B) {
        const glints = [];
        for (let y = T, r = 0; y < B; y += 6, r++) for (let x = from(L, 14, (r % 2) * 7); x < R; x += 14) {
          const h = hash(x, r), gx = x + ((h >>> 9) % 11), gy = y + ((h >>> 4) % 5);
          if ((h >>> 8) % 4 === 1 && gx >= L && gx < R && gy < B) glints.push({ x: gx, y: gy, h });
        }
        return glints;
      },
    };

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
    };

    /** The lounge's wall: its own, or the office's. */
    const loungePaint = () => (decor.loungePaint === 'auto' ? decor.paint : decor.loungePaint);

    /**
     * A wall from `l` to `r`, from `top` down to the floor, in `kind`: a
     * finish of its own, the same all along and its colour above the room,
     * or paint, in its lounge's shade for a `lounge`.
     */
    function paintWall(kind, l, r, top, lounge = false) {
      const paint = PAINTS[kind], finish = WALL_FINISHES[kind];
      if (finish) {
        if (top < 0) fill(l, top, r - l, -top, paint.wall);
        return finish(l, r);
      }
      fill(l, top, r - l, WALL_BOTTOM - top, lounge ? paint.lounge : paint.wall);
      fill(l, top, r - l, 2, paint.top);
      fill(l, 30, r - l, 8, paint.wainscot);
      fill(l, 30, r - l, 1, paint.rail);
      fill(l, 38, r - l, 2, PALETTE.baseboard);
    }

    function drawWall() {
      const { left, top, right, width: across } = seen;
      const paint = PAINTS[decor.paint];
      // Held upright, the back wall is the office's alone: the lounge has its own, below.
      const wide = shape === WIDE;
      if (wide) {
        paintWall(decor.paint, left, LOUNGE_LEFT, top);
        paintWall(loungePaint(), LOUNGE_LEFT, right, top, true);
      } else {
        paintWall(decor.paint, left, right, top);
      }
      if (wide) fill(LOUNGE_LEFT, top, 1, WALL_BOTTOM - top, paint.top);

      hungOf().forEach((piece, i) => drawWallPiece(piece.kind, piece.x, i));
      // String lights along the top of the wall through December
      if (decor.season === 'calendar' && month === 11) {
        fill(left, 3, across, 1, PALETTE.frame);
        const bulbs = [0xff5a4a, 0xf2d35b, 0x6fd36f, 0x4c8bf5];
        for (let bx = left - (left % 6) + 3, i = 0; bx < right; bx += 6, i++) {
          const lit = (Math.floor(now * 1.5) + i) % 4 !== 0;
          fill(bx, 4, 2, 2, bulbs[((i % 4) + 4) % 4], lit ? 1 : 0.35);
        }
      }
      if (wide) drawWindow(260, 6, 30, 20, true);

      drawDoor();
      drawClock(CORRIDOR_X, 6);

      // The warning light beside the door, lit while the alarm sounds
      fill(195, 27, 7, 2, PALETTE.frame);
      if (alarm === null) fill(196, 23, 5, 4, PALETTE.alarmOff);
      else fill(196, 23, 5, 4, PALETTE.alarm, 0.75 + 0.25 * Math.sin(alarm * 20));
      fill(197, 23, 2, 1, WHITE, alarm === null ? 0.15 : 0.6);
    }

    /** A shelf of books, `w` wide and six high, from `l`, `y`: `row` varies them from shelf to shelf. */
    function drawBooks(l, y, w, row) {
      const colours = [0xd96a5a, 0x4c8bf5, 0xf2d35b, 0x4f9a5a, 0xeae6dc, 0x9a6ad0, 0xe08a3a];
      fill(l, y, w, 6, PALETTE.doorway);
      for (let b = 0, bx = l + 1; bx < l + w - 2; b++) {
        const width = Math.min(2 + ((b + row) % 2), l + w - 1 - bx), height = 4 + (((b * 3 + row) % 3) % 2);
        fill(bx, y + 6 - height, width, height, colours[(b * 2 + row * 3) % colours.length]);
        bx += width + (b % 4 === 3 ? 2 : 0);
      }
      fill(l, y + 6, w, 1, PALETTE.deskEdge);
    }

    /** One of the back wall's places, as set up, around its middle `x`. */
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
      /** Crossed paddles. Two canoe paddles, their blades painted. */
      paddles(x) {
        for (const [dir, tipColour] of [[1, 0xc9443a], [-1, 0x3f6fc4]]) {
          line(x - dir * 9, 6, x + dir * 6, 22, 0xc49a6c);
          fill(x - dir * 9 - 1, 5, 3, 1, 0x8a6a42);
          const bx = x + dir * 7, by = 23;
          fill(bx - 2, by - 2, 5, 6, 0xd8b080); fill(bx - 1, by + 4, 3, 1, 0xd8b080); fill(bx - 2, by + 2, 5, 2, tipColour); dot(bx, by + 4, tipColour);
        }
      },
    };

    /**
     * One of the back wall's places, as set up, around its middle `x`: what
     * the season makes of it, and a storm through it while someone watches.
     */
    function drawWallPiece(piece, x, hung = null) {
      const touches = decor.season === 'calendar' ? SEASONAL_TOUCHES.filter((touch) => touch.piece === piece && touch.months.includes(month)) : [];
      const instead = touches.find((touch) => touch.instead);
      if (instead) instead.instead(x);
      else drawWallPieceItself(piece, x);
      for (const touch of touches) if (touch.over) touch.over(x);
      const pane = STORM_PANES[piece];
      const watched = hung !== null && pane && office.people.some((p) => !p.isWalking && p.place.startsWith('storm:')
        && stormSpots[numberAfter('storm:', p.place)] && stormSpots[numberAfter('storm:', p.place)].hung === hung);
      if (watched) drawStorm(x + pane[0], pane[1], pane[2], pane[3]);
    }

    /** A storm through a pane: dark clouds, rain, and lightning now and then. */
    function drawStorm(l, t, w, h) {
      const flash = (now % 5) < 0.12 || ((now % 5) > 0.25 && (now % 5) < 0.32);
      within([[l, t, w, h]], () => {
        fill(l, t, w, h / 2, 0x3a4252);
        fill(l, t + h / 2, w, h / 2, 0x4a5262);
        for (const [cx, cy, cw] of [[2, 1, 12], [16, 2, 14], [8, 4, 10], [22, 5, 9]]) {
          fill(l + cx, t + cy, cw, 2, 0x2a303c);
          fill(l + cx + 2, t + cy - 1, cw - 4, 1, 0x2a303c);
        }
        for (let k = 0; k < 14; k++) {
          const rx = l + ((k * 7 + Math.floor(now * 30) + k * k) % w), ry = t + ((k * 5 + Math.floor(now * 40)) % h);
          dot(rx, ry, 0xa8c0d8, 0.8);
          dot(rx + 1, ry + 1, 0xa8c0d8, 0.5);
        }
        if (flash) {
          fill(l, t, w, h, WHITE, 0.55);
          for (const [dx, dy] of [[20, 1], [19, 2], [19, 3], [18, 4], [19, 5], [18, 6], [17, 7], [18, 8]]) dot(l + dx, t + dy, 0xfff6c0);
        }
      });
      fill(l + w / 2, t, 1, h, PALETTE.frame);
      fill(l, t + h / 2, w, 1, PALETTE.frame);
    }

    function drawWallPieceItself(piece, x) {
      if (Object.hasOwn(WALL_PIECES, piece)) return WALL_PIECES[piece](x);
      switch (piece) {
        case 'window':
          return drawWindow(x - 16, 6, 32, 20, false);
        case 'whiteboard': {
          const l = x - 12;
          fill(l, 9, 24, 16, 0xb8bfcc);
          fill(l + 1, 10, 22, 14, PALETTE.whiteboard);
          fill(l + 3, 12, 10, 1, 0x4c8bf5);
          fill(l + 3, 15, 14, 1, 0xd96a5a);
          fill(l + 3, 18, 8, 1, 0x4f9a5a);
          fill(l + 15, 18, 5, 1, 0x4c8bf5);
          // A to-do list, written up line by line while someone stands at the first whiteboard
          if (boardOf() && x === boardOf().x) {
            for (const person of office.people) {
              const spot = numberAfter('board:', person.place);
              if (spot === null || person.isWalking) continue;
              const length = 1 + (Math.floor((now - person.arrivedAt) * 3) % 9);
              fill(spot === 0 ? l + 2 : l + 13, 21, length, 1, PALETTE.marker);
            }
          }
          return fill(l, 25, 24, 1, PALETTE.frame);
        }
        case 'pinboard': {
          const l = x - 10;
          fill(l, 10, 20, 16, PALETTE.doorPanel);
          fill(l + 1, 11, 18, 14, PALETTE.cork);
          fill(l + 3, 13, 4, 4, 0xf2d35b);
          fill(l + 9, 14, 5, 4, 0xf28fb0);
          fill(l + 4, 19, 5, 3, 0x8fc6f0);
          return fill(l + 12, 20, 4, 3, PALETTE.paper);
        }
        case 'poster': {
          // Mountains at sunset, framed
          const l = x - 8;
          fill(l, 7, 16, 20, PALETTE.frame);
          fill(l + 1, 8, 14, 9, 0xf0a070);
          fill(l + 1, 17, 14, 9, 0x5b5aa0);
          fill(l + 9, 10, 3, 3, 0xffe08a);
          for (let k = 0; k < 6; k++) fill(l + 2 + k, 20 - k, 1, 6 + k, 0x3e7a6a);
          for (let k = 0; k < 6; k++) fill(l + 8 + k, 15 + k, 1, 11 - k, 0x356b5c);
          fill(l + 6, 15, 2, 1, WHITE, 0.8);
          return fill(l + 1, 24, 14, 2, 0x2f5a4c);
        }
        case 'shelf': {
          // Books on three shelves, down to the rail
          const l = x - 13;
          fill(l, 7, 26, 23, PALETTE.counter);
          [8, 15, 22].forEach((shelf, row) => drawBooks(l + 1, shelf, 24, row));
          return;
        }
        case 'plant': {
          // A planter on a bracket, trailing leaves
          fill(x - 6, 20, 12, 1, PALETTE.frame);
          fill(x - 5, 15, 10, 5, PALETTE.pot);
          fill(x - 5, 15, 10, 1, 0x8a4f32);
          fill(x - 7, 11, 5, 4, PALETTE.leaf);
          fill(x - 2, 9, 5, 6, PALETTE.leafDark);
          fill(x + 3, 11, 5, 4, PALETTE.leaf);
          fill(x - 7, 15, 2, 9, PALETTE.leaf);
          fill(x + 6, 15, 2, 7, PALETTE.leafDark);
          return fill(x + 2, 20, 2, 5, PALETTE.leaf);
        }
        default:
          return; // bare
      }
    }

    /**
     * A floor of `kind` from `L`, `T` to `R`, `B`, lined up from the room's
     * corner: the office's, or one in the lounge. Drawn once and kept, at
     * whole screen pixels to an art pixel, then copied each frame, with
     * whatever moves on it over the top.
     */
    function paintFloor(kind, L, T, R, B) {
      if (R <= L || B <= T) return;
      if (!Number.isInteger(scale) || !Number.isInteger(ox) || !Number.isInteger(oy)) {
        drawFloorOf(kind, L, T, R, B);
        if (FLOOR_GLINTS[kind]) drawGlints(FLOOR_GLINTS[kind](L, T, R, B));
        return;
      }
      const doc = ctx.canvas.ownerDocument || document;
      if (!floorsDrawn.has(doc)) floorsDrawn.set(doc, new Map());
      const kept = floorsDrawn.get(doc), key = `${kind} ${L} ${T} ${R} ${B} ${scale}`;
      let floor = kept.get(key);
      if (floor) {
        kept.delete(key); // the most recently drawn last, so the oldest goes first
      } else {
        const canvas = doc.createElement('canvas');
        canvas.width = (R - L) * scale;
        canvas.height = (B - T) * scale;
        const main = { ctx, ox, oy };
        ctx = canvas.getContext('2d');
        ox = -L * scale;
        oy = -T * scale;
        try {
          drawFloorOf(kind, L, T, R, B);
        } finally {
          ({ ctx, ox, oy } = main);
        }
        floor = { canvas, glints: FLOOR_GLINTS[kind] ? FLOOR_GLINTS[kind](L, T, R, B) : null };
        if (kept.size >= FLOORS_KEPT) kept.delete(kept.keys().next().value);
      }
      kept.set(key, floor);
      ctx.drawImage(floor.canvas, ox + L * scale, oy + T * scale);
      if (floor.glints) drawGlints(floor.glints);
    }

    /** Glints on a floor, each twinkling in its own time. */
    function drawGlints(glints) {
      for (const { x, y, h } of glints) dot(x, y, WHITE, 0.4 + 0.6 * Math.max(0, Math.sin(now * 2 + h)));
    }

    /** Draws a floor of `kind` from `L`, `T` to `R`, `B`, square by square. */
    function drawFloorOf(kind, L, T, R, B) {
      const depth = B - T, across = R - L;
      /** Squares of `size` from the room's corner outwards, cut at `R`: `paint(x, y, column, row, width)` for each. */
      const squares = (size, paint) => {
        for (let y = T, r = 0; y < B; y += size, r++) {
          for (let x = Math.floor(L / size) * size; x < R; x += size) paint(x, y, Math.floor(x / size), r, Math.min(size, R - x));
        }
      };
      if (kind === 'tiles') {
        fill(L, T, across, depth, 0x777a81);
        squares(9, (x, y, c, r, w) => fill(x, y, w - 1, 8, (c + r) % 2 === 0 ? 0x9c9fa6 : 0x8c8f96));
      } else if (kind === 'concrete') {
        fill(L, T, across, depth, 0x73767b);
        squares(36, (x, y, c, r, w) => {
          fill(x, y, w, 1, 0x67696e);
          fill(x, y, 1, 36, 0x67696e);
          for (let k = 0; k < 7; k++) dot(x + ((c * 13 + r * 7 + k * 11) % 33) + 1, y + ((c * 5 + r * 17 + k * 9) % 33) + 2, 0x80838a);
        });
      } else if (FLOOR_FINISHES[kind]) {
        FLOOR_FINISHES[kind](L, T, R, B);
      } else if (kind === 'carpet') {
        fill(L, T, across, depth, 0x4b5065);
        squares(12, (x, y, c, r, w) => {
          if ((c + r) % 2 === 0) fill(x, y, w, 12, 0x474c60);
          dot(x + ((c * 5 + r * 3) % 10) + 1, y + ((c * 7 + r) % 10) + 1, 0x565b71);
        });
      } else {
        fill(L, T, across, depth, PALETTE.wood);
        let row = 0;
        for (let y = T + 4; y < B + 4; y += 5, row++) {
          fill(L, y, across, 1, PALETTE.plankSeam);
          // The planks' ends, staggered from row to row, from the room's left edge outwards
          let x = (row % 3) * 11;
          while (x > L) x -= 33;
          for (; x < R; x += 33) fill(x, y - 4, 1, 4, PALETTE.plankSeam);
        }
      }
    }

    function drawFloor() {
      const { left, right, bottom } = seen;
      // The office's floor, to the lounge: on its right, or held upright, behind its wall.
      const wide = shape === WIDE;
      const officeRight = wide ? LOUNGE_LEFT : right, officeBottom = wide ? bottom : shape.loungeWall;
      const top = WALL_BOTTOM, depth = officeBottom - top;
      paintFloor(decor.floor, left, top, officeRight, officeBottom);
      // The lounge's: a carpet, or a floor of its own, kept to the lounge
      const carpet = LOUNGES[decor.lounge];
      const lounge = wide ? { x: LOUNGE_LEFT, y: top } : { x: left, y: officeBottom };
      if (carpet) fill(lounge.x, lounge.y, right - lounge.x, bottom - lounge.y, carpet[0]);
      else within([[lounge.x, lounge.y, right - lounge.x, bottom - lounge.y]], () => paintFloor(decor.lounge, lounge.x, lounge.y, right, bottom));
      if (wide) fill(LOUNGE_LEFT, top, 1, depth, carpet ? carpet[1] : SHADOW);
      if (!pieceOf('couch').off) {
        moved('couch', () => {
          fill(244, 124, 64, 20, PALETTE.rugEdge);
          fill(246, 126, 60, 16, PALETTE.rug);
        });
      }
      fill(207, 40, 14, 2, PALETTE.mat);
      fill(left, top, right - left, 1, SHADOW);
    }

    /**
     * Held upright, the lounge's own wall, as high as the back wall, with
     * its window, and a doorway under the back door through which the
     * corridor goes on into the lounge. It hides the office's floor behind it.
     */
    function drawLoungeWall() {
      const { left, right } = seen;
      const paint = PAINTS[loungePaint()], finish = WALL_FINISHES[loungePaint()];
      const top = shape.loungeWall - WALL_BOTTOM;
      const face = (l, r) => {
        fill(l, top, r - l, WALL_BOTTOM, paint.lounge);
        fill(l, top + 30, r - l, 8, paint.wainscot);
        fill(l, top + 30, r - l, 1, paint.rail);
        fill(l, top + 38, r - l, 2, PALETTE.baseboard);
      };
      if (finish) {
        // Its finish, as the back wall has it, either side of the doorway and above it
        const kept = oy;
        oy += top * scale;
        try {
          within([[left, 0, 207 - left, WALL_BOTTOM], [221, 0, right - 221, WALL_BOTTOM], [207, 0, 14, 13]], () => finish(left, right));
        } finally {
          oy = kept;
        }
      } else {
        // Either side of the doorway, and above it
        face(left, 207);
        face(221, right);
        fill(207, top, 14, 13, paint.lounge);
        fill(left, top, right - left, 2, paint.top);
      }
      // The doorway's frame, open onto the corridor
      fill(205, top + 13, 18, 2, PALETTE.frame);
      fill(205, top + 15, 2, 25, PALETTE.frame);
      fill(221, top + 15, 2, 25, PALETTE.frame);
      moved('wall', () => {
        drawWindow(260, 6, 30, 20, true);
        drawWallPiece('pinboard', 364); // between the cooler and the doorway
      }, false); // on the wall, wherever the counter goes
    }

    /* Furniture */

    /* Desks, and what else an agent can work at */

    /**
     * The office's monitors besides the standard one, from behind, each at
     * the top-left `x`, `y` of where a standard one goes, and where its
     * screen is, if it isn't there: for the screen's light.
     */
    const MONITORS = {
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
    };

    /** The monitor at the desk being drawn: its own, its style's, or the room's. */
    let monitorNow = decor.monitor;

    /** A monitor from behind, its top-left at `x`, `y`: its screen faces whoever sits at it. The desk's own kind, if it has one. */
    function drawMonitor(x, y) {
      if (MONITORS[monitorNow]) return MONITORS[monitorNow].draw(x, y);
      fill(x, y, 13, 9, PALETTE.monitor);
      fill(x + 1, y + 1, 11, 7, PALETTE.monitorPanel);
      fill(x + 5, y + 9, 3, 2, PALETTE.monitorStand);
    }

    /** A laptop from behind, the top-left of its lid at `x`, `y`, its base seen edge on below it. */
    function drawLaptop(x, y) {
      fill(x, y, 13, 8, PALETTE.monitor);
      fill(x + 1, y + 1, 11, 6, PALETTE.monitorPanel);
      fill(x - 1, y + 8, 15, 1, PALETTE.keys);
    }

    /** A keyboard on the desk in front of whoever sits at `s`, `top` its row. */
    function drawKeyboard(s, top) {
      fill(s.x - 5, top, 10, 2, PALETTE.keyboard);
      fill(s.x - 4, top, 8, 1, PALETTE.keys);
    }

    /** The standard desk's top and front, in its finish or its effort's paint. */
    function drawDeskBody({ x, y, s, tint, finish }) {
      fill(x, y, DESK_WIDTH, 1, tint(finish.edge, 1));
      fill(x, y + 1, DESK_WIDTH, 4, tint(finish.top, 0.9));
      fill(x + 9, y + 1, 7, 1, PALETTE.monitorStand);
      fill(x, y + 5, DESK_WIDTH, 11, tint(finish.front, 0.72));
      fill(x, y + 5, DESK_WIDTH, 1, tint(finish.lip, 0.62));
      fill(x + 30, y + 8, 10, 6, tint(finish.drawer, 0.82));
      fill(x + 34, y + 10, 2, 1, PALETTE.handle);
      fill(x + 1, y + 16, DESK_WIDTH - 2, 1, SHADOW);
      drawKeyboard(s, y + 2);
    }

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
            const outfit = outfitOf(occupant.session.agent, occupant.session.model, memberOf(occupant.session), 'tent');
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
            const outfit = outfitOf(occupant.session.agent, occupant.session.model, memberOf(occupant.session), 'beanbag');
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
    };

    /** The back of a desk's workspace, behind whoever is at it. */
    function drawWorkspaceBack(desk) {
      const kind = workspaceOf(sessionAt(desk), desk);
      if (WORKSPACE_PARTS[kind].back) WORKSPACE_PARTS[kind].back(workspaceAt(desk));
    }

    /**
     * How a desk's workspace is drawn: where it is, its colours (painted for
     * its session's effort, from white for low to red for max, plain when
     * there's none, flashing as the button goes down) and who is at it.
     */
    function workspaceAt(desk) {
      const { x, y } = DESKS[desk];
      // The desk keeps its own finish whatever the effort, which shows in mugs of coffee on it; it flashes as the button goes down.
      const paint = blast && blast.desk === desk && sinceBlast >= 0 && sinceBlast < 1.2 && Math.floor(sinceBlast * 6) % 2 === 0 ? WHITE : null;
      return {
        x, y, s: seat(desk),
        tint: (plain, shade) => (paint === null ? plain : shaded(paint, shade)),
        finish: finishAt(desk),
        occupant: occupantOf(desk),
      };
    }

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
    };

    /** Something on a desk whose corner is at `x`, `y`: a plant, papers, a lamp, or what you've put on it. */
    function drawDeskThing(thing, x, y) {
      if (Object.hasOwn(DESK_ITEMS, thing)) return DESK_ITEMS[thing](x, y);
      if (thing === 'plant') {
        fill(x + 38, y - 2, 3, 3, PALETTE.pot);
        fill(x + 37, y - 5, 5, 3, PALETTE.leaf);
        dot(x + 39, y - 6, PALETTE.leafDark);
      } else if (thing === 'papers') {
        fill(x + 35, y + 1, 6, 3, PALETTE.paper);
        fill(x + 36, y + 2, 4, 1, PALETTE.counterTop);
      } else if (thing === 'lamp') {
        fill(x + 39, y - 6, 1, 6, PALETTE.frame);
        fill(x + 36, y - 7, 5, 2, PALETTE.brass);
        fill(x + 37, y, 5, 1, PALETTE.frame);
      } else if (thing === 'cactus') {
        fill(x + 38, y - 2, 3, 3, PALETTE.pot);
        fill(x + 39, y - 7, 1, 5, DESK_THINGS.cactus);
        dot(x + 38, y - 5, DESK_THINGS.cactus);
        dot(x + 40, y - 4, DESK_THINGS.cactus);
      } else if (thing === 'duck') {
        fill(x + 36, y - 2, 4, 2, DESK_THINGS.duck);
        fill(x + 38, y - 4, 2, 2, DESK_THINGS.duck);
        dot(x + 40, y - 3, DESK_THINGS.beak);
      } else if (thing === 'trophy') {
        fill(x + 36, y - 6, 5, 3, PALETTE.brass);
        dot(x + 38, y - 3, PALETTE.brass);
        fill(x + 37, y - 2, 3, 2, darker(PALETTE.brass));
      } else if (thing === 'photo') {
        fill(x + 36, y - 6, 5, 6, PALETTE.frame);
        fill(x + 37, y - 5, 3, 3, DESK_THINGS.sky);
        fill(x + 37, y - 3, 3, 1, PALETTE.leaf);
      } else if (thing === 'mug') {
        fill(x + 37, y - 3, 3, 3, PALETTE.mug);
        dot(x + 40, y - 2, PALETTE.mug);
        fill(x + 37, y - 3, 3, 1, DESK_THINGS.coffee);
      }
    }

    function drawDesk(desk) {
      const { x, y } = DESKS[desk], s = seat(desk);
      const kind = workspaceOf(sessionAt(desk), desk), workspace = WORKSPACES[kind];
      monitorNow = monitorAt(desk);
      WORKSPACE_PARTS[kind].front(workspaceAt(desk));

      // Something different on each desk: what you've put on it, for its project or for the desk, its style's, or the office's own
      const member = memberOf(sessionAt(desk)), own = deskOwn(desk, 'thing');
      const thing = member && member.desk !== 'auto' ? member.desk : own !== 'auto' ? own : ['plant', 'papers', 'lamp'][desk % 3];
      if (workspace.things) drawDeskThing(thing, x + workspace.things.dx, y + workspace.things.dy);

      const occupant = occupantOf(desk);
      drawEffort(desk, { x, y, kind, occupant });
      if (!occupant) return;
      const failure = failureOf(occupant.session);
      if (failure) drawFailureAtDesk(failure, takeOf(occupant), { x, y, s, workspace, occupant });
      const skin = lookFor(occupant).skin;
      const tool = occupant.session.state === 'thinking' ? toolKind(occupant.session.tool) : null;
      if (tool === 'web') {
        // A little globe at the end of the desk, turning, moved as what's on the desk is
        const turn = Math.floor(now * 4) % 5;
        const { dx, dy } = workspace.things || { dx: 0, dy: 0 }, gx = x + dx, gy = y + dy;
        fill(gx + 2, gy - 5, 3, 1, PALETTE.sea);
        fill(gx + 1, gy - 4, 5, 3, PALETTE.sea);
        fill(gx + 2, gy - 1, 3, 1, PALETTE.sea);
        fill(gx + 1 + turn, gy - 4, 1, 2, PALETTE.leaf);
        dot(gx + 1 + ((turn + 2) % 5), gy - 2, PALETTE.leaf);
        dot(gx + 3, gy, PALETTE.frame);
      }
      if (tool === 'read') {
        // Leafing through papers, a page lifting now and then
        fill(s.x - 4, y + 1, 8, 2, PALETTE.paper);
        if (Math.floor(now * 2 + occupant.look.phase * 2) % 2 === 0) fill(s.x - 3, y, 6, 1, PALETTE.paper);
        dot(s.x - 5, y + 2, skin);
        dot(s.x + 4, y + 2, skin);
      } else if (occupant.session.state === 'thinking') {
        // Typing: one hand, then the other.
        const beat = Math.floor(now * 8 + occupant.look.phase * 8) % 2;
        dot(s.x - 3, y + 1 + beat, skin);
        dot(s.x + 2, y + 2 - beat, skin);
      } else if (occupant.session.state === 'unread') {
        // A coffee, now it's done.
        fill(s.x + 7, y - 1, 3, 3, PALETTE.mug);
        dot(s.x + 10, y, PALETTE.mug);
        const rise = (now * 0.6 + occupant.look.phase) % 1;
        dot(s.x + 8, y - 2 - Math.round(rise * 4), WHITE, 0.6 * (1 - rise));
      } else {
        dot(s.x - 3, y + 2, skin);
        dot(s.x + 2, y + 2, skin);
      }
    }

    /**
     * What a failure looks like at the desk: a storm cloud of their own over
     * the head of whoever has no window to watch, or out of order, a sign
     * taped across the desk or a closed one hung from the screen.
     */
    function drawFailureAtDesk(failure, take, { x, y, s, workspace, occupant }) {
      if (failure === 'overloaded') {
        const cx = s.x + Math.round(Math.sin(now * 0.8)), cy = topOf(occupant, poseOf(occupant, now)) - 9;
        fill(cx - 5, cy, 11, 3, 0x5a6070);
        fill(cx - 3, cy - 2, 7, 2, 0x5a6070);
        fill(cx - 4, cy + 2, 9, 1, 0x4a5060);
        fill(cx - 2, cy - 2, 3, 1, 0x7a8090);
        for (let k = 0; k < 5; k++) dot(cx - 4 + k * 2, Math.round(cy + 4 + ((now * 18 + k * 3.7) % 7)), 0x8fc6f0, 0.8);
        if ((now % 4) < 0.1) for (const [dx, dy] of [[0, 3], [-1, 4], [0, 5]]) dot(cx + dx, cy + dy, 0xfff6c0);
      } else if (failure === 'billing' && take === 0) {
        fill(x + 8, y + 3, 28, 13, PALETTE.frame, 0.25);
        fill(x + 7, y + 2, 28, 13, 0xf4f1ea);
        fill(x + 7, y + 2, 28, 1, PALETTE.dome);
        fill(x + 7, y + 14, 28, 1, PALETTE.dome);
        pixelText('OUT OF', x + 10, y + 4, PALETTE.dome, false);
        pixelText('ORDER', x + 11, y + 9, PALETTE.dome, false);
        for (const [dx, dy] of [[0, 0], [26, 0], [0, 12], [26, 12]]) fill(x + 7 + dx, y + 2 + dy, 2, 1, 0xe8e0c0, 0.9);
      } else if (failure === 'billing') {
        // Hung on a string from the screen, swinging a little
        const sx = x + workspace.screen.dx, sy = y + workspace.screen.dy, swing = Math.round(Math.sin(now * 1.6) * 1.2);
        line(sx + 3, sy, sx + 2 + swing, sy + 4, 0xc8c4ba);
        line(sx + 10, sy, sx + 11 + swing, sy + 4, 0xc8c4ba);
        const l = sx - 5 + swing;
        fill(l, sy + 4, 25, 8, PALETTE.dome);
        fill(l + 1, sy + 5, 23, 6, 0xf4f1ea);
        pixelText('CLOSED', l + 1, sy + 5, PALETTE.dome, false);
      }
    }

    /**
     * Where the cups go at a workspace with no top: a row on the ground in
     * front of it, inside what it stands on, so it's in no one's way; on a
     * ship, on the deck behind the rail, a cup between each of its posts.
     * Each its first cup's top-left and its pot's, from the desk's corner,
     * and whether they stand on the ground, with a shadow under them.
     */
    const CUPS_AT = {
      tent: { cups: { dx: 12, dy: 9 }, pot: { dx: 29, dy: 7 }, ground: true },
      beanbag: { cups: { dx: 12, dy: 12 }, pot: { dx: 29, dy: 10 }, ground: true },
      ship: { cups: { dx: 2, dy: 2 }, pot: { dx: 18, dy: 0 }, ground: false },
    };
    /** On a desk: along its front, under the monitor; the pot in the gap before the keyboard. */
    const CUPS_ON_DESK = { cups: { dx: 1, dy: 2 }, pot: { dx: 18, dy: 0 }, ground: false };
    const EFFORT_MUGS = { low: 1, medium: 2, high: 3, xhigh: 4 };
    /** A mug dropping in, in seconds and art pixels: how long it falls, from how high, and its bounce. */
    const MUG_DROP = { fall: 0.22, height: 14, bounce: 0.18 };

    /** A cup drunk, 3 wide and 3 tall, the coffee left in it on top, its handle on the right: its top-left at `x`, `y`. */
    function drawCup(x, y) {
      fill(x, y, 3, 1, DESK_THINGS.coffee);
      fill(x, y + 1, 3, 2, PALETTE.mug);
      dot(x + 3, y + 1, shaded(PALETTE.mug, 0.72));
      dot(x, y + 2, shaded(PALETTE.mug, 0.86));
    }
    /** A little moka pot, 5 wide and 5 tall, its top-left at `x`, `y`: its knob, its lid, its spout and handle, its waist and its base. */
    function drawLittlePot(x, y) {
      dot(x + 2, y, 0x2a2a2e);
      fill(x + 1, y + 1, 3, 1, 0xc8cdd6);
      dot(x, y + 2, 0x9aa0a6);
      fill(x + 1, y + 2, 3, 1, 0xb8bdc6);
      dot(x + 4, y + 2, 0x2a2a2e);
      fill(x + 1, y + 3, 3, 1, 0x8a8f99);
      fill(x + 1, y + 4, 3, 1, 0xa8adb6);
      dot(x + 1, y + 2, WHITE, 0.6);
    }
    /** Steam rising, `count` puffs, from `x`, `y`. */
    function drawSteam(x, y, count, reach) {
      for (let k = 0; k < count; k++) {
        const rise = (now * 0.7 + k / count) % 1;
        fill(x + Math.round(Math.sin(rise * 6 + k) * 2), y - Math.round(rise * reach), 2, 1, WHITE, 0.6 * (1 - rise));
      }
    }

    /**
     * How hard an agent is working, in coffee: cups already drunk in a row,
     * one for low effort up to four for extra high, along the front of the
     * desk under the monitor and clear of the keyboard and what's on it, or
     * at a workspace with no top, in front of it (see CUPS_AT); and for the
     * most, a little moka pot at the end of the row, steaming, and steam off
     * the agent. The floor to a desk's sides is the interns'.
     */
    function drawEffort(desk, { x, y, kind, occupant }) {
      const effort = office.deskEffort(desk);
      if (!effort) return;
      const at = CUPS_AT[kind] || CUPS_ON_DESK;
      // The effort just gone up, the newest of the coffee drops in from above and bounces
      const rose = office.effortRoseAt(desk), since = rose === null ? Infinity : now - rose;
      const drop = since < MUG_DROP.fall ? -Math.round(MUG_DROP.height * (1 - (since / MUG_DROP.fall) ** 2))
        : since < MUG_DROP.fall + MUG_DROP.bounce ? -Math.round(2 * Math.sin((Math.PI * (since - MUG_DROP.fall)) / MUG_DROP.bounce)) : 0;
      // The row: four cups at most, the newest dropping in, and the pot at its end
      const cups = effort === 'max' ? 4 : EFFORT_MUGS[effort] ?? 1;
      const cx = x + at.cups.dx, cy = y + at.cups.dy, px = x + at.pot.dx, py = y + at.pot.dy;
      if (at.ground) fill(cx - 1, cy + 3, cups * 4 + 1 + (effort === 'max' ? px - cx - cups * 4 + 5 : 0), 1, SHADOW);
      for (let i = 0; i < cups; i++) drawCup(cx + i * 4, cy + (effort !== 'max' && i === cups - 1 ? drop : 0));
      if (effort !== 'max') return;
      drawLittlePot(px, py + drop);
      drawSteam(px - 1, py + 1 + drop, 3, 6);
      if (occupant) drawSteam(occupant.position.x - 1, topOf(occupant, poseOf(occupant, now)) - 1, 4, 9);
    }

    /** The office's chair, at a workspace that has one. */
    function drawChair(desk) {
      if (!WORKSPACES[workspaceOf(sessionAt(desk), desk)].chair) return;
      const s = seat(desk), y = DESKS[desk].y;
      // A rounded back, lower than the monitor beside it
      const [chair, chairTop] = chairAt(desk);
      fill(s.x - 3, y - 7, 7, 1, chairTop);
      fill(s.x - 4, y - 6, 9, 6, chair);
      fill(s.x - 4, y - 6, 1, 6, chairTop);
    }

    function drawCounter() {
      fill(230, 33, 26, 2, PALETTE.counterTop);
      fill(230, 35, 26, 12, PALETTE.counter);
      fill(232, 37, 10, 8, PALETTE.doorPanel);
      fill(244, 37, 10, 8, PALETTE.doorPanel);
      // The coffee machine, a cup under it, and a stack of cups
      fill(235, 22, 9, 11, PALETTE.machine);
      fill(236, 23, 7, 3, 0x3a3a40);
      dot(242, 24, 0x6fd36f);
      dot(239, 27, 0x3a3a40);
      fill(238, 30, 3, 3, PALETTE.mug);
      fill(248, 29, 3, 4, PALETTE.mug);
      const rise = (now * 0.5) % 1;
      dot(239 + Math.round(Math.sin(rise * 6) * 0.8), 29 - Math.round(rise * 6), WHITE, 0.5 * (1 - rise));
    }

    function drawCooler() {
      fill(306, 19, 4, 1, 0x6fa6d0);
      fill(305, 20, 6, 8, PALETTE.water);
      fill(303, 28, 10, 20, PALETTE.cooler);
      fill(304, 31, 2, 2, 0xd04a4a);
      fill(310, 31, 2, 2, 0x4a7ad0);
      fill(303, 48, 10, 1, SHADOW);
    }

    function drawCouchBack() {
      fill(250, 102, 52, 3, PALETTE.couchLight);
      fill(250, 105, 52, 7, PALETTE.couch);
      fill(267, 105, 1, 7, PALETTE.couchDark);
      fill(285, 105, 1, 7, PALETTE.couchDark);
      fill(250, 112, 52, 2, PALETTE.couchLight);
    }

    /** The seat's front and the arms, which hide the legs of whoever sits. */
    function drawCouchFront() {
      fill(250, 114, 52, 6, PALETTE.couchDark);
      fill(246, 104, 5, 16, PALETTE.couch);
      fill(301, 104, 5, 16, PALETTE.couch);
      fill(246, 104, 5, 1, PALETTE.couchLight);
      fill(301, 104, 5, 1, PALETTE.couchLight);
      fill(247, 120, 58, 1, SHADOW);
    }

    function drawCoffeeTable() {
      fill(264, 128, 24, 4, PALETTE.deskTop);
      fill(264, 132, 24, 2, PALETTE.deskFront);
      fill(265, 134, 1, 2, PALETTE.handle);
      fill(286, 134, 1, 2, PALETTE.handle);
      fill(270, 127, 2, 2, PALETTE.mug);
      fill(278, 129, 5, 2, PALETTE.paper);
    }

    /* The lounge's corner piece, as set up, and whoever is at it */

    // Where the spots round the piece being drawn start, among the room's spots: set as each is drawn.
    let cornerBase = LOUNGE_SPOTS.length;
    /** Someone at the nth spot round the piece being drawn. */
    const atCorner = (i) => office.people.some((p) => p.place === spotPlace(cornerBase + i) && !p.isWalking && !p.isLeaving);

    /** An arcade cabinet: invaders march across its screen, and a ship fires back while someone plays. */
    function drawArcade() {
      const l = 265, playing = atCorner(0);
      fill(l - 1, 86, 16, 1, SHADOW);
      fill(l, 60, 14, 26, 0x2b2350);
      fill(l, 60, 1, 26, 0x8a5cf0);
      fill(l + 13, 60, 1, 26, 0x8a5cf0);
      fill(l + 1, 61, 12, 4, 0x6a3fb5);
      fill(l + 3, 62, 8, 2, 0xffd36b, 0.6 + 0.4 * Math.sin(now * 4));
      fill(l + 2, 66, 10, 9, PALETTE.doorway);
      fill(l + 3, 67, 8, 7, playing ? 0x14204a : 0x0b1022);
      const march = Math.floor(now * 2) % 4;
      for (let i = 0; i < 3; i++) dot(l + 4 + i * 2 + (march < 2 ? 0 : 1), 68 + (march % 2), 0x6fd36f);
      if (playing) {
        const ship = l + 4 + Math.round((Math.sin(now * 2.5) + 1) * 2.5);
        dot(ship, 73, WHITE);
        const shot = (now * 2.5) % 1;
        dot(ship, 72 - Math.round(shot * 4), 0xffd36b, 1 - shot * 0.5);
      }
      fill(l + 1, 75, 12, 3, 0x3d3470);
      fill(l + 4, 74, 1, 2, PALETTE.frame);
      dot(l + 4, 73, 0xd4140b);
      dot(l + 8, 76, 0xf2d35b);
      dot(l + 10, 76, 0x4c8bf5);
      fill(l + 5, 80, 4, 2, PALETTE.frame);
      dot(l + 6, 80, 0xd96a5a);
    }

    /** A vending machine, lit up: a snack drops now and then for whoever is at it. */
    function drawVending() {
      const l = 263;
      fill(l - 1, 86, 20, 1, SHADOW);
      fill(l, 58, 18, 28, 0xb8322a);
      fill(l, 58, 18, 2, 0xd9493f);
      fill(l + 2, 61, 11, 17, 0x9fd3e8);
      const snacks = [0xf2d35b, 0xd96a5a, 0x4f9a5a, 0x4c8bf5, 0xe08a3a, 0x9a6ad0];
      for (let r = 0; r < 4; r++) {
        for (let c = 0; c < 3; c++) fill(l + 3 + c * 4, 62 + r * 4, 2, 2, snacks[(r * 4 + c * 5) % snacks.length]);
        fill(l + 2, 64 + r * 4, 11, 1, 0x7fb3c8);
      }
      fill(l + 2, 61, 2, 17, WHITE, 0.2);
      fill(l + 14, 61, 3, 9, PALETTE.machine);
      for (let r = 0; r < 3; r++) dot(l + 15, 62 + r * 2, r === 0 ? 0x6fd36f : PALETTE.keys);
      fill(l + 14, 72, 3, 2, PALETTE.frame);
      fill(l + 2, 80, 11, 4, PALETTE.frame);
      const fall = (now % 3) / 1.2;
      if (atCorner(0) && fall < 1) fill(l + 7, 62 + Math.round(fall * 18), 2, 2, 0xf2d35b);
    }

    /** A foosball table: its men twitch while someone's at it, and a match starts when both sides are taken. */
    function drawFoosball() {
      const busy = atCorner(0) || atCorner(1), match = atCorner(0) && atCorner(1);
      fill(255, 90, 34, 1, SHADOW);
      fill(257, 90, 2, 5, PALETTE.tableLeg);
      fill(285, 90, 2, 5, PALETTE.tableLeg);
      fill(256, 78, 32, 12, PALETTE.counter);
      fill(258, 80, 28, 8, 0x2f7a4a);
      fill(271, 80, 1, 8, WHITE, 0.5);
      fill(257, 82, 1, 4, PALETTE.frame);
      fill(286, 82, 1, 4, PALETTE.frame);
      [261, 265, 269, 274, 278, 282].forEach((x, i) => {
        fill(x, 76, 1, 16, 0xc9c3b8);
        fill(x, 75, 1, 2, PALETTE.frame);
        fill(x, 91, 1, 2, PALETTE.frame);
        const shift = busy ? Math.round(Math.sin(now * 5 + i * 1.7)) : 0;
        for (const y of [81, 85]) fill(x, y + shift, 1, 2, i % 2 ? 0x4c8bf5 : 0xd96a5a);
      });
      if (!match) return;
      const lap = (now * 0.8) % 2, across = lap < 1 ? lap : 2 - lap;
      dot(260 + Math.round(across * 24), 83 + Math.round(Math.sin(now * 3)), WHITE);
    }

    /** A fish tank on a cabinet: fish swim to and fro, and bubbles rise. */
    function drawFishTank() {
      fill(257, 86, 31, 1, SHADOW);
      fill(258, 76, 29, 10, PALETTE.counter);
      fill(260, 78, 11, 6, PALETTE.doorPanel);
      fill(274, 78, 11, 6, PALETTE.doorPanel);
      fill(258, 60, 29, 1, PALETTE.monitorOn);
      fill(258, 61, 29, 15, PALETTE.frame);
      fill(259, 62, 27, 13, 0x2f7fb0);
      fill(259, 62, 27, 2, 0x6fb6e0);
      fill(259, 72, 27, 3, 0xd9c38a);
      fill(262, 66, 1, 6, PALETTE.leaf);
      fill(263, 68, 1, 4, PALETTE.leafDark);
      fill(282, 67, 1, 5, PALETTE.leaf);
      for (const [speed, phase, y, colour] of [[0.13, 0, 66, 0xff8a3d], [0.09, 0.6, 69, 0xf2d35b], [0.17, 1.3, 64, 0xf28fb0]]) {
        const swim = (now * speed + phase) % 2, across = swim < 1 ? swim : 2 - swim, right = swim < 1;
        const x = 261 + Math.round(across * 20);
        fill(x, y, 3, 2, colour);
        dot(right ? x - 1 : x + 3, y, colour);
        dot(right ? x - 1 : x + 3, y + 1, colour);
        dot(right ? x + 2 : x, y, PALETTE.eye);
      }
      const rise = (now * 0.6) % 1;
      dot(284, 71 - Math.round(rise * 8), WHITE, 0.7 * (1 - rise));
      dot(283, 71 - Math.round(((rise + 0.5) % 1) * 8), WHITE, 0.5);
    }

    /** A bookcase, and a beanbag to read in, drawn round whoever sits in it. */
    function drawBookcase() {
      fill(287, 86, 21, 1, SHADOW);
      fill(288, 57, 19, 29, PALETTE.counter);
      [59, 66, 73, 80].forEach((y, row) => drawBooks(289, y, 17, row));
    }

    function drawBeanbagBack() {
      fill(266, 87, 16, 6, 0xc0584a);
      fill(268, 86, 12, 1, 0xd96a5a);
    }

    function drawBeanbagFront() {
      fill(265, 92, 18, 4, 0xd96a5a);
      fill(266, 96, 16, 1, 0xa84a3e);
      fill(265, 97, 18, 1, SHADOW);
    }

    /**
     * A dragon asleep on its hoard: its breath lifts its back, smoke curls
     * from its nose, and the gold glints. Someone come to look wakes one eye.
     */
    function drawDragon() {
      const scale = 0x3f8a5a, dark = 0x2c6a44, belly = 0xe8c87a, gold = 0xf2c94c, deep = 0xc9952a;
      const watched = atCorner(0), breath = Math.sin(now * 1.3) > 0.2 ? 1 : 0;
      // The hoard, a mound of coins
      fill(253, 96, 50, 1, SHADOW);
      for (let row = 84; row <= 95; row++) {
        const half = Math.round(24 * Math.sqrt((row - 83) / 12));
        fill(278 - half, row, half * 2, 1, gold);
        for (let k = (row % 3) * 2; k < half * 2; k += 5) dot(278 - half + k, row, deep);
      }
      fill(256, 93, 5, 3, PALETTE.brass);
      dot(258, 92, 0xd4140b);
      fill(296, 91, 4, 2, 0x8fc0e8);
      // The tail, round the front of the hoard to its tip
      for (const [tx, ty] of [[292, 86], [295, 88], [297, 90], [298, 92], [296, 93], [293, 94]]) fill(tx, ty, 3, 2, scale);
      fill(290, 94, 3, 2, dark);
      dot(289, 93, dark);
      // The body, curled on top, rising as it breathes, and the wings folded on it
      fill(267, 77 - breath, 26, 10 + breath, scale);
      fill(265, 79 - breath, 2, 7 + breath, scale);
      fill(293, 79 - breath, 2, 7, scale);
      fill(268, 84, 22, 3, belly);
      for (let k = 0; k < 5; k++) dot(270 + k * 4, 85, deep);
      fill(271, 72 - breath, 14, 5, dark);
      fill(273, 70 - breath, 8, 2, dark);
      fill(276, 69 - breath, 3, 1, dark);
      for (let k = 0; k < 6; k++) dot(268 + k * 4, 76 - breath, 0xd96a5a);
      // The neck down to the head, resting on the gold, nose to the left
      fill(259, 80, 9, 5, scale);
      fill(250, 81, 11, 6, scale);
      fill(248, 83, 3, 4, scale);
      fill(250, 86, 11, 1, belly);
      dot(249, 83, PALETTE.frame);
      fill(256, 78, 1, 3, belly);
      fill(259, 77, 1, 3, belly);
      if (watched) {
        fill(253, 82, 3, 2, 0xf2d35b);
        dot(254, 82, PALETTE.frame);
        dot(254, 83, PALETTE.frame);
      } else {
        fill(253, 83, 3, 1, dark);
      }
      // Smoke from its nose, puff after puff, quicker when it's awake
      const puffs = watched ? 3 : 2;
      for (let k = 0; k < puffs; k++) {
        const rise = (now * (watched ? 0.5 : 0.3) + k / puffs) % 1;
        const size = 1 + Math.round(rise * 2);
        fill(247 - Math.round(rise * 5), 82 - Math.round(rise * 14), size, size, PALETTE.smoke, 0.7 * (1 - rise));
      }
      // Coins catching the light, one after another
      for (let k = 0; k < 4; k++) {
        const glint = (now * 0.7 + k * 0.27) % 1;
        if (glint < 0.12) dot([262, 284, 272, 291][k], [90, 88, 93, 92][k], WHITE);
      }
    }

    /** A pinball table, its playfield lit: the flippers flick and the ball flies while someone plays. */
    function drawPinball() {
      const playing = atCorner(0), cabinet = 0x2b2350, trim = 0x8a5cf0;
      fill(261, 90, 27, 1, SHADOW);
      fill(263, 82, 2, 8, PALETTE.machine);
      fill(284, 82, 2, 8, PALETTE.machine);
      // The backbox, its lights chasing round the score
      fill(262, 51, 25, 13, cabinet);
      fill(262, 51, 25, 1, trim);
      fill(264, 53, 21, 7, PALETTE.doorway);
      for (let k = 0; k < 10; k++) {
        const lit = (Math.floor(now * 6) + k) % 4 === 0;
        dot(264 + k * 2, 61, lit ? 0xffd36b : 0x4a3a80);
      }
      for (let k = 0; k < 5; k++) fill(266 + k * 4, 55, 3, 3, playing ? 0xff8a3d : 0x6a3f2a);
      fill(273, 64, 3, 2, PALETTE.machine);
      // The playfield, seen from the end it's played at
      fill(261, 66, 27, 17, cabinet);
      fill(263, 67, 23, 14, 0x1d3a7a);
      fill(261, 66, 1, 17, trim);
      fill(287, 66, 1, 17, trim);
      const bumpers = [[267, 69, 0xd96a5a], [276, 68, 0xf2d35b], [281, 72, 0x6fd36f]];
      bumpers.forEach(([bx, by, colour], i) => {
        const hit = playing && (Math.floor(now * 3) + i) % 3 === 0;
        fill(bx, by, 3, 3, hit ? WHITE : colour);
      });
      fill(264, 75, 3, 1, 0x8fc0e8);
      fill(282, 75, 3, 1, 0x8fc0e8);
      const flick = playing && Math.floor(now * 2.5) % 2 === 0 ? 1 : 0;
      fill(268, 79 - flick, 4, 1, WHITE);
      fill(277, 79 - flick, 4, 1, WHITE);
      if (playing) {
        const bx = 274 + Math.round(Math.sin(now * 2.3) * 9), by = 73 + Math.round(Math.cos(now * 3.1) * 5);
        dot(bx, by, 0xe0e0e8);
      } else {
        dot(286, 79, 0xe0e0e8);
      }
      fill(261, 82, 27, 2, cabinet);
      dot(268, 82, 0xd4140b);
      dot(281, 82, 0xd4140b);
    }

    /** A jukebox, its arch of light running round the colours: while someone picks a record, it spins and notes rise. */
    function drawJukebox() {
      // Playing: the computer's music, with the jukebox on, or someone at it
      const playing = frame.music?.playing === true || atCorner(0), wood = 0x7a3b2a;
      fill(263, 88, 23, 1, SHADOW);
      fill(266, 56, 17, 1, wood);
      fill(264, 57, 21, 2, wood);
      fill(263, 59, 23, 29, wood);
      // The arch of light, its colours moving round
      for (let k = 0; k < 30; k++) {
        const [ax, ay] = k < 10 ? [264, 86 - k * 2.6] : k < 20 ? [266 + (k - 10) * 1.7, 59 - Math.round(Math.sin(((k - 10) / 9) * Math.PI) * 2)] : [284, 60 + (k - 20) * 2.6];
        fill(Math.round(ax), Math.round(ay), 1, 2, rainbow((k / 30 + now * 0.3) % 1));
      }
      // The records behind glass, one turning
      fill(267, 62, 15, 9, PALETTE.doorway);
      fill(267, 62, 15, 1, WHITE, 0.25);
      const spin = playing ? Math.floor(now * 8) % 2 : 0;
      fill(271, 65, 7, 4, 0x1a1a1f);
      dot(274, 66, 0xd96a5a);
      dot(272 + spin * 4, 65 + spin * 3, 0x5a5a66);
      // The buttons, the grille and the coin slot
      for (let k = 0; k < 6; k++) dot(267 + k * 3, 73, playing && k === 2 ? 0x6fd36f : PALETTE.paper);
      for (let row = 76; row < 84; row += 2) fill(267, row, 15, 1, 0x4a2a1e);
      fill(266, 85, 17, 3, 0x5a2a1e);
      if (!playing) return;
      for (let k = 0; k < 3; k++) {
        const rise = (now * 0.45 + k / 3) % 1;
        const nx = 268 + k * 6 + Math.round(Math.sin(rise * 6 + k) * 2), ny = 57 - Math.round(rise * 5);
        const colour = [0xf2d35b, 0x8fc0e8, 0xf28fb0][k];
        fill(nx, ny, 2, 1, colour, 1 - rise);
        fill(nx + 1, ny - 3, 1, 3, colour, 1 - rise);
      }
    }

    /** A pool table: while someone's at it, the cue goes in, the white runs up the table and the rack breaks. */
    function drawPool() {
      const rail = 0x6b3a22, felt = 0x2f7a4a;
      const left = atCorner(0), right = atCorner(1);
      fill(250, 92, 52, 1, SHADOW);
      for (const lx of [252, 298]) fill(lx, 86, 3, 6, rail);
      fill(250, 72, 52, 14, rail);
      fill(253, 74, 46, 10, felt);
      fill(253, 74, 46, 1, shaded(felt, 0.7));
      fill(250, 86, 52, 2, shaded(rail, 0.7));
      for (const [px, py] of [[252, 73], [275, 73], [298, 73], [252, 84], [275, 84], [298, 84]]) fill(px, py, 2, 1, PALETTE.frame);
      // The balls: racked at one end, the white at the other
      const shot = left || right ? (now % 4) / 4 : 0;
      const toward = right && !left ? -1 : 1;
      const rack = [[285, 79], [287, 78], [287, 80], [289, 77], [289, 79], [289, 81]];
      const colours = [0xf2d35b, 0xd4140b, 0x4c8bf5, 0x1a1a1f, 0x9a6ad0, 0xe08a3a];
      const spread = shot > 0.5 ? Math.min(1, (shot - 0.5) * 3) : 0;
      rack.forEach(([bx, by], i) => {
        const sx = toward > 0 ? bx : 545 - bx;
        dot(sx + Math.round(spread * toward * ((i % 3) + 1)), by + Math.round(spread * (by - 79) * 1.5), colours[i]);
      });
      const run = shot < 0.25 ? 0 : Math.min(1, (shot - 0.25) * 4);
      const white = toward > 0 ? 262 + Math.round(run * 20) : 290 - Math.round(run * 20);
      dot(white, 79, WHITE);
      if (!left && !right) return;
      // The cue, drawn back, then in
      const back = shot < 0.2 ? Math.round((shot / 0.2) * 4) : shot < 0.25 ? 0 : 2;
      for (let k = 0; k < 14; k++) {
        const cx = toward > 0 ? white - 2 - back - k : white + 2 + back + k;
        dot(cx, 79 + Math.round(k / 5), k < 2 ? 0xeae6dc : 0xc49a6c);
      }
    }

    /** An upright piano and its bench: whoever plays has their back to the room, keys going down and notes rising. */
    function drawPiano() {
      const playing = atCorner(0), body = 0x1d1f25, shine = 0x3a3d47;
      fill(261, 82, 31, 1, SHADOW);
      fill(262, 58, 29, 2, shine);
      fill(262, 60, 29, 22, body);
      fill(263, 60, 1, 22, shine);
      // The music on its stand, and the candles
      fill(270, 62, 13, 6, PALETTE.paper);
      for (const row of [63, 65]) fill(271, row, 11, 1, 0x8a8f9c);
      fill(265, 57, 1, 3, 0xf2efe9);
      fill(287, 57, 1, 3, 0xf2efe9);
      dot(265, 56, 0xffd36b, 0.6 + 0.4 * Math.sin(now * 9));
      dot(287, 56, 0xffd36b, 0.6 + 0.4 * Math.sin(now * 7 + 1));
      // The keys, some pressed while it's played
      fill(263, 71, 27, 3, 0xf4f1ea);
      for (let k = 0; k < 9; k++) dot(264 + k * 3, 71, PALETTE.frame);
      if (playing) {
        for (let k = 0; k < 3; k++) {
          const key = (Math.floor(now * 5) * (k + 2) + k * 7) % 26;
          dot(264 + key, 73, 0xb8b3a8);
        }
      }
      fill(263, 74, 27, 1, shine);
      fill(265, 79, 3, 2, PALETTE.brass);
      fill(285, 79, 3, 2, PALETTE.brass);
      if (!playing) return;
      for (let k = 0; k < 3; k++) {
        const rise = (now * 0.5 + k / 3) % 1;
        const nx = 266 + k * 9 + Math.round(Math.sin(rise * 5 + k) * 2), ny = 57 - Math.round(rise * 5);
        fill(nx, ny, 2, 1, PALETTE.paper, 1 - rise);
        fill(nx + 1, ny - 3, 1, 3, PALETTE.paper, 1 - rise);
      }
    }

    /** The piano's bench, in front of whoever sits at it. */
    function drawPianoBench() {
      fill(268, 86, 17, 2, 0x3a3d47);
      fill(268, 88, 17, 1, 0x1d1f25);
      fill(269, 89, 2, 2, 0x1d1f25);
      fill(282, 89, 2, 2, 0x1d1f25);
      fill(267, 91, 19, 1, SHADOW);
    }

    /** A drum kit, the drummer behind it: while they play, the sticks come down and the cymbals shake. */
    function drawDrums() {
      const playing = atCorner(0), shell = 0x2f5aa8, chrome = 0xc9c3b8, brass = 0xe0b84a;
      const beat = playing ? Math.floor(now * 6) % 2 : 0;
      fill(250, 93, 54, 1, SHADOW);
      // Cymbals on their stands, shaking when hit
      for (const [cx, top, hit] of [[256, 72, beat], [298, 68, 1 - beat]]) {
        fill(cx, top + 1, 1, 93 - top - 1, chrome);
        fill(cx - 2, 92, 5, 1, chrome);
        const tilt = playing && hit ? 1 : 0;
        fill(cx - 4, top - tilt, 4, 1, brass);
        fill(cx + 1, top + tilt, 4, 1, brass);
        dot(cx, top, brass);
      }
      // The toms over the bass drum, the floor tom and the snare
      fill(260, 82, 9, 3, chrome);
      fill(260, 85, 9, 2, shell);
      fill(291, 81, 9, 3, chrome);
      fill(291, 84, 9, 7, shell);
      fill(292, 91, 1, 2, chrome);
      fill(298, 91, 1, 2, chrome);
      fill(266, 76, 8, 2, chrome);
      fill(266, 78, 8, 3, shell);
      fill(279, 76, 8, 2, chrome);
      fill(279, 78, 8, 3, shell);
      // The bass drum, its front head marked, thumping
      const thump = playing && beat ? 1 : 0;
      fill(268, 81 + thump, 17, 12 - thump, shell);
      fill(270, 82 + thump, 13, 10 - thump, 0xf2efe9);
      fill(274, 85 + thump, 5, 4, 0xd96a5a);
      fill(268, 81 + thump, 17, 1, chrome);
      if (!playing) return;
      // The sticks, coming down in turn
      const lift = [beat ? 3 : 0, beat ? 0 : 3];
      [[270, 268], [282, 286]].forEach(([hand, tip], i) => {
        for (let k = 0; k < 5; k++) dot(hand + Math.round(((tip - hand) * k) / 4), 81 - lift[i] - k + (k > 2 ? 1 : 0), 0xe9d3a0);
      });
    }

    /** A brass telescope on its tripod, aimed at the sky: it drifts while no one looks, and stars twinkle where it points. */
    function drawTelescope() {
      const looking = atCorner(0), tube = 0xe8e4da, leg = 0x6b4e37;
      fill(266, 92, 24, 1, SHADOW);
      for (const [fx, fy] of [[267, 91], [287, 91], [279, 88]]) {
        for (let k = 0; k <= 16; k++) dot(Math.round(278 + ((fx - 278) * k) / 16), Math.round(74 + ((fy - 74) * k) / 16), leg);
      }
      fill(276, 72, 5, 3, PALETTE.brass);
      // The tube, from the eyepiece up to its lens, swinging a little while no one's at it
      const drift = looking ? 0 : Math.round(Math.sin(now * 0.4) * 2);
      const from = { x: 271, y: 79 }, to = { x: 292, y: 60 + drift };
      for (let k = 0; k <= 20; k++) {
        const tx = Math.round(from.x + ((to.x - from.x) * k) / 20), ty = Math.round(from.y + ((to.y - from.y) * k) / 20);
        const band = k === 7 || k === 15;
        fill(tx - 1, ty - 1, 3, 3, band ? PALETTE.brass : tube);
        dot(tx + 1, ty + 1, 0xb8b3a8);
      }
      fill(to.x - 1, to.y - 2, 4, 4, PALETTE.brass);
      fill(to.x, to.y - 1, 2, 2, 0x1d3a7a);
      fill(from.x - 2, from.y, 3, 2, PALETTE.frame);
      // A star chart on a stand beside it
      fill(296, 80, 1, 12, leg);
      fill(292, 78, 10, 7, 0x1d3a7a);
      for (const [sx, sy] of [[294, 80], [297, 79], [299, 82], [295, 83]]) dot(sx, sy, PALETTE.paper);
      // Stars, out where it points
      for (let k = 0; k < 3; k++) {
        const t = (now * 0.6 + k * 0.33) % 1;
        if (t < 0.4) dot(to.x + 4 + k * 3, to.y - 5 - k * 2, 0xfff0b0, 1 - t / 0.4);
      }
    }

    /** A tall tank of jellyfish, glowing as they pulse up and drift down, bubbles rising past them. */
    function drawJellyfish() {
      const metal = 0x5a5f6c;
      fill(260, 91, 31, 1, SHADOW);
      fill(261, 84, 29, 7, PALETTE.counter);
      fill(263, 86, 25, 4, PALETTE.doorPanel);
      fill(262, 51, 27, 2, metal);
      fill(262, 82, 27, 2, metal);
      // The water, darker at the bottom, and the glass's shine
      for (let row = 53; row < 82; row++) fill(262, row, 27, 1, row < 64 ? 0x1a4a7a : row < 74 ? 0x153d68 : 0x0e2a4a);
      fill(263, 54, 1, 26, WHITE, 0.2);
      fill(286, 54, 1, 26, WHITE, 0.1);
      for (const [speed, phase, jx, colour] of [[0.5, 0, 268, 0xf28fb0], [0.4, 2.1, 280, 0xb88af0], [0.6, 4.2, 274, 0x8fe0f0]]) {
        const bob = Math.sin(now * speed + phase), pulse = Math.floor(now * 2 + phase) % 2;
        const y = 66 + Math.round(bob * 8), x = jx + Math.round(Math.sin(now * 0.3 + phase) * 2);
        fill(x - 3, y - 3, 7, 7, colour, 0.15);
        fill(x - 1, y - 1, 4 - pulse, 1, colour);
        fill(x - 2 + pulse, y, 6 - pulse * 2, 2, colour);
        for (let k = 0; k < 3; k++) {
          const sway = Math.round(Math.sin(now * 3 + phase + k) * 0.6);
          fill(x - 1 + k * 2 + sway, y + 2, 1, 3 + (k % 2), colour, 0.7);
        }
      }
      for (let k = 0; k < 2; k++) {
        const rise = (now * 0.4 + k / 2) % 1;
        dot(284 - k * 17, 80 - Math.round(rise * 26), WHITE, 0.6 * (1 - rise));
      }
    }

    /** A bonsai on a low stand, a stone lantern beside it: a leaf falls now and then, and a trimmer's snips glint. */
    function drawBonsai() {
      const wood = 0x5a3b25;
      fill(260, 91, 34, 1, SHADOW);
      fill(261, 80, 32, 2, 0x8c6744);
      fill(261, 82, 32, 2, wood);
      fill(263, 84, 2, 7, wood);
      fill(289, 84, 2, 7, wood);
      // The pot, the trunk twisting up, and the pads of leaves
      fill(267, 76, 18, 4, 0x4a6a8a);
      fill(266, 76, 20, 1, 0x5a7a9a);
      fill(268, 75, 16, 1, 0x3b5a3a);
      for (const [tx, ty] of [[276, 74], [276, 72], [275, 70], [274, 68], [275, 66], [277, 64]]) fill(tx, ty, 2, 2, 0x6e4a2e);
      fill(270, 69, 5, 1, 0x6e4a2e);
      fill(278, 67, 5, 1, 0x6e4a2e);
      for (const [px, py, w] of [[264, 66, 10], [279, 64, 10], [271, 60, 11]]) {
        fill(px + 1, py, w - 2, 1, PALETTE.leaf);
        fill(px, py + 1, w, 3, PALETTE.leaf);
        fill(px + 1, py + 3, w - 2, 1, PALETTE.leafDark);
        dot(px + 2, py + 1, 0x6fba6a);
      }
      // The stone lantern
      fill(288, 74, 3, 6, 0x8a8f9c);
      fill(286, 73, 7, 1, 0x9aa0a6);
      dot(289, 76, 0xffd36b, 0.8);
      // A leaf falling to the stand
      const fall = (now * 0.25) % 1;
      if (fall < 0.6) dot(283 + Math.round(Math.sin(fall * 12)), 68 + Math.round((fall / 0.6) * 11), PALETTE.leaf);
      if (atCorner(0) && Math.floor(now * 2) % 3 === 0) dot(266, 65, WHITE);
    }

    /** A jungle of plants: a palm, a monstera and a fern, their leaves stirring, and a parrot that bobs and flaps. */
    function drawJungle() {
      const leaf = PALETTE.leaf, dark = PALETTE.leafDark, light = 0x6fba6a, deep = 0x2f6a3a;
      fill(246, 93, 60, 1, SHADOW);
      const sway = (phase) => Math.round(Math.sin(now * 0.9 + phase) * 0.8);
      // The monstera, to the left, its broad leaves split
      fill(250, 86, 10, 7, PALETTE.pot);
      fill(249, 86, 12, 1, 0x8a4f32);
      for (const [lx, ly, phase] of [[244, 72, 0.5], [252, 64, 1.5], [257, 75, 2.5], [246, 80, 3.5]]) {
        const s = sway(phase);
        [3, 6, 8, 8, 7, 4].forEach((w, r) => fill(lx + s + Math.floor((8 - w) / 2), ly + r, w, 1, r < 3 ? deep : 0x285a32));
        for (let r = 1; r < 5; r++) dot(lx + s + 1 + r, ly + r, 0x3f8a4a);
        dot(lx + s + 1, ly + 2, PALETTE.wood);
        dot(lx + s + 6, ly + 3, PALETTE.wood);
        // Its stem, down to the pot
        for (let k = 0; k <= 8; k++) {
          const sy = ly + 6 + Math.round(((86 - ly - 6) * k) / 8);
          if (sy < 86) dot(Math.round(lx + 4 + s + ((255 - lx - 4 - s) * k) / 8), sy, deep);
        }
      }
      // The palm, in the middle, its fronds arching over the rest
      fill(272, 86, 9, 7, PALETTE.pot);
      fill(271, 86, 11, 1, 0x8a4f32);
      for (let k = 0; k < 8; k++) fill(275 + (k > 4 ? 1 : 0), 62 + k * 3, 3, 3, k % 2 ? 0x8a6a42 : 0x6e5233);
      for (const [dir, length, lift, phase] of [[-1, 14, 3, 0], [1, 14, 3, 1], [-1, 9, 0, 2], [1, 9, 0, 3], [-1, 6, 5, 4], [1, 6, 5, 5]]) {
        for (let k = 1; k <= length; k++) {
          const fx = 277 + dir * k, fy = 61 - lift + Math.round((k * k) / (length + 2)) + (k > length / 2 ? sway(phase) : 0);
          dot(fx, fy, leaf);
          dot(fx, fy + 1, k % 2 ? dark : leaf);
          if (k > 2 && k % 2 === 0) dot(fx, fy + 2, dark);
        }
      }
      fill(275, 59, 5, 3, leaf);
      dot(277, 58, light);
      // The fern, to the right, its fronds arching out of the pot
      fill(292, 87, 9, 6, PALETTE.pot);
      fill(291, 87, 11, 1, 0x8a4f32);
      for (let f = 0; f < 7; f++) {
        const angle = Math.PI * (1.12 + f * 0.13), s = sway(f);
        for (let k = 1; k <= 9; k++) {
          const fx = 296 + Math.round(Math.cos(angle) * k * 1.1) + (k > 5 ? s : 0);
          const fy = 86 + Math.round(Math.sin(angle) * k) + Math.round((k * k) / 22);
          dot(fx, fy, k % 3 ? leaf : light);
          if (k > 1 && k < 8) dot(fx + (k % 2 ? 1 : -1), fy + 1, dark);
        }
      }
      // The parrot on the palm, bobbing, and now and then flapping
      const px = 284, py = 63, bob = Math.floor(now * 2) % 2, flap = (now % 5) < 0.6 ? Math.floor(now * 10) % 2 : -1;
      fill(px, py - 4 + bob, 3, 3, 0xd4140b);
      dot(px + 3, py - 3 + bob, 0xf2d35b);
      dot(px + 1, py - 3 + bob, WHITE);
      fill(px, py - 1, 3, 4, 0xd4140b);
      fill(px - 1, py + 3, 2, 3, 0x4c8bf5);
      if (flap >= 0) fill(px + 3, py - 2 + flap * 2, 3, 2, 0x4c8bf5);
      else fill(px - 1, py, 2, 3, 0x4c8bf5);
      dot(px + 1, py + 3, 0xf2d35b);
    }

    /** A coffee bar, a chalkboard in front: steam from the machine, and a cup filling while the barista's in. */
    function drawCoffeeBar() {
      const wood = 0x5a3b25;
      fill(252, 93, 50, 1, SHADOW);
      // The espresso machine, chrome, on the bar
      fill(282, 63, 14, 10, 0xc9c3b8);
      fill(282, 63, 14, 1, WHITE);
      fill(284, 65, 10, 3, PALETTE.machine);
      dot(286, 66, 0x6fd36f);
      dot(291, 66, 0xd96a5a);
      fill(286, 69, 2, 2, PALETTE.machine);
      fill(291, 69, 2, 2, PALETTE.machine);
      const brewing = atCorner(0), fillUp = brewing ? (now * 0.25) % 1 : 0;
      fill(286, 72, 3, 1, PALETTE.mug);
      if (fillUp > 0) fill(286, 72, 3, 1, 0x5a3a24);
      for (let k = 0; k < 2; k++) {
        const rise = (now * 0.5 + k / 2) % 1;
        dot(292 + Math.round(Math.sin(rise * 6 + k)), 61 - Math.round(rise * 8), WHITE, 0.6 * (1 - rise));
      }
      // Cups stacked, and the bar itself
      for (let k = 0; k < 3; k++) fill(256, 70 - k * 2, 4, 2, k % 2 ? 0xe9e6df : PALETTE.mug);
      fill(262, 71, 3, 2, 0x8a5a2b);
      fill(254, 73, 46, 3, 0x8c6744);
      fill(254, 73, 46, 1, 0xa87a56);
      fill(254, 76, 46, 16, wood);
      for (let k = 0; k < 5; k++) fill(257 + k * 9, 78, 1, 13, shaded(wood, 0.8));
      fill(254, 76, 46, 1, shaded(wood, 0.7));
      // The chalkboard, standing on the floor
      fill(244, 82, 9, 10, PALETTE.counter);
      fill(245, 83, 7, 7, 0x2a2e2a);
      for (const [cx, cy, w] of [[246, 84, 5], [246, 86, 4], [246, 88, 5]]) fill(cx, cy, w, 1, PALETTE.paper, 0.75);
      fill(244, 92, 9, 1, SHADOW);
    }

    /** Two racks of servers, their lights blinking, a cable bundle over them, a red light now and then. */
    function drawServers() {
      fill(258, 89, 35, 1, SHADOW);
      const lights = [0x6fd36f, 0x6fd36f, 0xf2d35b, 0x4c8bf5];
      for (const [rx, seed] of [[259, 0], [276, 7]]) {
        fill(rx, 52, 16, 37, 0x1d1f25);
        fill(rx, 52, 16, 1, 0x3a3f4a);
        fill(rx, 52, 1, 37, 0x3a3f4a);
        fill(rx + 15, 52, 1, 37, 0x3a3f4a);
        for (let u = 0; u < 11; u++) {
          const uy = 54 + u * 3;
          fill(rx + 2, uy, 12, 2, 0x2a2e36);
          fill(rx + 9, uy, 4, 1, 0x14161b);
          for (let k = 0; k < 3; k++) {
            const on = (Math.floor(now * (3 + ((u + k + seed) % 4))) + u * 3 + k + seed) % 3 !== 0;
            dot(rx + 3 + k * 2, uy, lights[(u + k + seed) % lights.length], on ? 1 : 0.2);
          }
        }
      }
      if (Math.floor(now * 1.2) % 7 === 0) dot(288, 63, PALETTE.alarm);
      // A tray of cables along the tops, dropping into each rack
      fill(259, 50, 33, 2, 0x5a5f6c);
      [0x4c8bf5, 0xf2d35b, 0xd96a5a].forEach((colour, k) => {
        fill(261 + k * 2, 50, 1, 3, colour);
        fill(278 + k * 2, 50, 1, 3, colour);
        dot(266 + k * 6, 50, colour);
      });
    }

    /** A 3D printer on a table, its head running back and forth as the print grows layer by layer, its spool turning. */
    function drawPrinter() {
      const frame = 0x3a3f4a, rail = 0x8a93a3, filament = 0x4c8bf5;
      fill(257, 91, 42, 1, SHADOW);
      fill(258, 78, 40, 2, 0x8c6744);
      fill(258, 80, 40, 2, PALETTE.counter);
      fill(260, 82, 2, 9, PALETTE.counter);
      fill(294, 82, 2, 9, PALETTE.counter);
      // The frame and its bed
      fill(265, 56, 2, 22, frame);
      fill(286, 56, 2, 22, frame);
      fill(265, 56, 23, 2, frame);
      fill(267, 75, 19, 2, 0x2a2e36);
      // The print, growing, then taken off for the next
      const progress = (now / 30) % 1, layers = Math.min(12, Math.floor(progress * 14));
      for (let k = 0; k < layers; k++) {
        const w = k < 4 ? 9 : k < 8 ? 7 : 5;
        fill(276 - Math.floor(w / 2), 74 - k, w, 1, k % 2 ? filament : shaded(filament, 0.85));
      }
      // The gantry, at the layer it's on, and the head running along it
      const gy = 72 - layers;
      fill(267, gy, 19, 1, rail);
      const run = layers < 12 ? Math.round(Math.sin(now * 4) * 4) : 0;
      fill(274 + run, gy - 2, 5, 3, PALETTE.machine);
      dot(276 + run, gy + 1, layers < 12 ? 0xff8a3d : PALETTE.machine);
      // The spool, turning while it prints, and its little screen
      fill(290, 62, 7, 7, filament);
      fill(292, 64, 3, 3, PALETTE.paper);
      const spoke = layers < 12 ? Math.floor(now * 3) % 2 : 0;
      dot(293 + (spoke ? 1 : -1), 65, 0x9aa0a6);
      for (let k = 0; k < 6; k++) dot(289 - Math.round(k * 0.5), 66 + k, filament);
      fill(268, 79, 6, 1, PALETTE.frame);
      fill(268, 79, Math.round(progress * 6), 1, 0x6fd36f);
    }

    /** A heavy bag on its stand: it swings when it's hit, a star where the punch lands. */
    function drawBoxing() {
      const post = 0x3a3f4a, bag = 0xb8322a;
      fill(268, 93, 36, 1, SHADOW);
      fill(296, 53, 3, 39, post);
      fill(278, 53, 21, 2, post);
      fill(293, 91, 9, 2, post);
      // The bag, hung from its chain, swinging out from the top
      const hitting = atCorner(0), swing = hitting ? Math.sin(now * 5) : Math.sin(now * 0.8) * 0.15;
      fill(281, 55, 1, 5, 0x9aa0a6);
      for (let row = 0; row < 22; row++) {
        const sx = Math.round(swing * row * 0.15);
        const w = row === 0 || row === 21 ? 7 : 9;
        fill(282 - Math.ceil(w / 2) + sx, 60 + row, w, 1, row === 4 || row === 16 ? 0x2a2e36 : bag);
        if (row > 0 && row < 21) dot(282 - 4 + sx, 60 + row, lighter(bag));
      }
      if (hitting && swing > 0.85) {
        const sx = Math.round(swing * 10 * 0.15);
        for (const [dx, dy] of [[0, 0], [-1, 0], [1, 0], [0, -1], [0, 1]]) dot(276 + sx + dx, 70 + dy, 0xffd36b);
      }
      // A pair of dumbbells, and a towel over the stand
      fill(301, 90, 7, 1, 0x5a5f6c);
      fill(300, 89, 2, 3, PALETTE.machine);
      fill(307, 89, 2, 3, PALETTE.machine);
      fill(299, 56, 3, 7, 0xe9e6df);
    }

    /** The treadmill's console and handrails, beyond whoever walks on it. */
    function drawTreadmillConsole() {
      const grey = 0x5a5f6c;
      fill(264, 68, 2, 14, grey);
      fill(286, 68, 2, 14, grey);
      fill(262, 63, 28, 7, 0x2a2e36);
      fill(262, 63, 28, 1, grey);
      fill(265, 65, 10, 3, 0x1a3a2a);
      const running = atCorner(0);
      for (let k = 0; k < 4; k++) {
        const level = running ? 1 + ((Math.floor(now * 2) + k * 2) % 3) : 1;
        fill(266 + k * 2, 68 - level, 1, level, 0x6fd36f);
      }
      dot(279, 65, running ? 0x6fd36f : 0x5a5f6c);
      dot(282, 65, 0xd96a5a);
      dot(285, 65, 0x4c8bf5);
    }

    /** The treadmill's belt, running under whoever walks on it. */
    function drawTreadmillBelt() {
      fill(260, 93, 32, 1, SHADOW);
      fill(261, 79, 30, 4, 0x3a3f4a);
      fill(263, 83, 26, 9, 0x23252c);
      fill(261, 83, 2, 9, 0x8a93a3);
      fill(289, 83, 2, 9, 0x8a93a3);
      fill(261, 92, 30, 1, 0x5a5f6c);
      const shift = atCorner(0) ? Math.floor(now * 8) % 3 : 0;
      for (let row = 83 + shift; row < 92; row += 3) fill(263, row, 26, 1, 0x34373f);
    }

    /** A hot tub's back rim, behind whoever is in it. */
    function drawHotTubBack() {
      fill(254, 76, 46, 2, 0xa87a56);
      fill(255, 78, 44, 3, 0x7a5232);
      for (let k = 0; k < 8; k++) fill(257 + k * 6, 78, 1, 3, 0x5a3b25);
    }

    /** The hot tub's water, up to the shoulders of whoever sits in it, bubbling and steaming, a duck afloat, and the tub's front. */
    function drawHotTubFront() {
      fill(253, 96, 48, 1, SHADOW);
      fill(255, 81, 44, 6, 0x3fa0c0);
      fill(255, 81, 44, 1, 0x8fd6ea);
      for (let k = 0; k < 6; k++) {
        const t = (now * 0.9 + k * 0.37) % 1;
        dot(258 + ((k * 13) % 40), 82 + (k % 3), WHITE, t < 0.5 ? 0.8 : 0.2);
      }
      const bob = Math.round(Math.sin(now * 2));
      fill(292, 80 + bob, 4, 2, DESK_THINGS.duck);
      fill(294, 78 + bob, 2, 2, DESK_THINGS.duck);
      dot(296, 79 + bob, DESK_THINGS.beak);
      fill(253, 86, 48, 2, 0xa87a56);
      fill(254, 88, 46, 8, 0x7a5232);
      for (let k = 0; k < 8; k++) fill(256 + k * 6, 88, 1, 8, 0x5a3b25);
      fill(254, 90, 46, 1, 0x3a3a40);
      for (let k = 0; k < 3; k++) {
        const rise = (now * 0.35 + k / 3) % 1;
        fill(262 + k * 14 + Math.round(Math.sin(rise * 5 + k) * 2), 78 - Math.round(rise * 18), 2 + Math.round(rise * 2), 1, WHITE, 0.35 * (1 - rise));
      }
    }

    /** A ball pit's back wall, padded. */
    function drawBallPitBack() {
      fill(254, 75, 46, 2, 0x5a8ad0);
      fill(254, 77, 46, 7, 0x3f6aa8);
      for (let k = 0; k < 5; k++) fill(256 + k * 9, 77, 1, 7, 0x2f5a8a);
    }

    /** The balls, up to the shoulders of whoever's in, one bouncing up now and then, and the pit's front wall. */
    function drawBallPitFront() {
      const colours = [0xd4140b, 0xf2d35b, 0x4c8bf5, 0x6fd36f, 0xf28fb0, 0xe08a3a];
      fill(253, 96, 48, 1, SHADOW);
      for (let row = 0; row < 4; row++) {
        for (let k = 0; k < 22; k++) fill(255 + k * 2 + (row % 2), 81 + row * 2, 2, 2, colours[(k * 7 + row * 5) % colours.length]);
      }
      const hop = (now * 0.8) % 1;
      if (hop < 0.5) {
        const h = Math.round(Math.sin(hop * 2 * Math.PI) * 6);
        fill(266 + Math.floor(now * 0.8) % 4 * 7, 80 - h, 2, 2, colours[Math.floor(now * 0.8) % colours.length]);
      }
      fill(253, 88, 48, 2, 0xd96a5a);
      fill(254, 90, 46, 6, 0xb8322a);
      for (let k = 0; k < 5; k++) fill(256 + k * 9, 90, 1, 6, 0x8a2a24);
    }

    /** A campfire in a ring of stones, flames flickering, sparks and smoke rising, a glow on the floor. */
    function drawCampfire() {
      const cx = 276;
      fill(cx - 16, 88, 33, 8, 0xff8a3d, 0.08);
      fill(cx - 10, 86, 21, 10, 0xff8a3d, 0.08);
      for (const [sx, sy] of [[266, 91], [270, 93], [276, 94], [282, 93], [286, 91], [269, 89], [283, 89]]) {
        fill(sx - 1, sy, 3, 2, 0x8a8f9c);
        dot(sx - 1, sy, 0xa8adb6);
      }
      fill(269, 90, 14, 2, 0x5a3b25);
      fill(271, 89, 10, 1, 0x6e4a2e);
      // The flames, each column its own height, red to yellow
      for (let k = 0; k < 9; k++) {
        const h = 4 + Math.round(4 * Math.abs(Math.sin(now * (5 + k) + k * 1.7))) - Math.abs(k - 4);
        if (h <= 0) continue;
        const fx = cx - 4 + k;
        fill(fx, 89 - h, 1, h, 0xd4140b);
        if (h > 2) fill(fx, 89 - h + 1, 1, h - 1, 0xff8a3d);
        if (h > 4) fill(fx, 89 - h + 3, 1, h - 3, 0xf2d35b);
      }
      for (let k = 0; k < 3; k++) {
        const rise = (now * 0.8 + k / 3) % 1;
        dot(cx - 2 + k * 2 + Math.round(Math.sin(rise * 7 + k) * 2), 82 - Math.round(rise * 14), 0xffd36b, 1 - rise);
      }
      for (let k = 0; k < 2; k++) {
        const rise = (now * 0.25 + k / 2) % 1;
        fill(cx - 1 + Math.round(Math.sin(rise * 4 + k) * 3), 76 - Math.round(rise * 20), 2 + Math.round(rise * 2), 2, PALETTE.smoke, 0.35 * (1 - rise));
      }
      // Marshmallows toasting on sticks, held out by whoever sits by it
      [[0, 259, 269], [1, 293, 283]].forEach(([i, hand, tip]) => {
        if (!atCorner(i)) return;
        for (let k = 0; k <= 8; k++) dot(Math.round(hand + ((tip - hand) * k) / 8), 85 - Math.round(k * 0.4), 0x8a6a42);
        const toast = (now * 0.1 + i * 0.5) % 1;
        fill(tip + (i ? -1 : 0), 81, 2, 2, toast < 0.5 ? 0xf4f1ea : 0xd9b26a);
      });
    }

    /** The logs either side of the fire, in front of whoever sits on them. */
    function drawCampfireLogs() {
      for (const lx of [248, 290]) {
        fill(lx, 90, 15, 4, 0x6e4a2e);
        fill(lx, 90, 15, 1, 0x8a6a42);
        fill(lx, 90, 2, 4, 0xc9a06a);
        dot(lx + 1, 91, 0x8a6a42);
        fill(lx, 94, 15, 1, SHADOW);
      }
    }

    /*
     * The pieces that play music: each comes alive while the computer's
     * music plays, with the control on, or while someone is at it.
     */

    /** Notes rising from `x`, `y`, as the jukebox has them, `spread` apart. */
    function notes(x, y, spread = 6) {
      for (let k = 0; k < 3; k++) {
        const rise = (now * 0.45 + k / 3) % 1;
        const nx = x + k * spread + Math.round(Math.sin(rise * 6 + k) * 2), ny = y - Math.round(rise * 5);
        const colour = [0xf2d35b, 0x8fc0e8, 0xf28fb0][k];
        fill(nx, ny, 2, 1, colour, 1 - rise);
        fill(nx + 1, ny - 3, 1, 3, colour, 1 - rise);
      }
    }
    /** On the beat, twice a second or so: for what pumps or pulses. */
    const onBeat = () => Math.floor(now * 2.2) % 2 === 0;
    /** Records' sleeves, seen edge on in a crate or a cabinet. */
    const RECORDS = [0xd96a5a, 0x4c8bf5, 0xf2d35b, 0x4f9a5a, 0xeae6dc, 0x9a6ad0, 0xe08a3a];

    /** Bubbler jukebox. A chrome arch round a glowing tube, bubbles rising in the side tubes. Playing, the tube runs round the colours and the bubbles rise faster. */
    function drawBubbler() {
      // Playing: the computer's music, with the control on, or someone at it
      const playing = frame.music?.playing === true || atCorner(0);
      const wood = 0x6b3a22, chrome = 0xc8cdd6, cx = 274;
      const tube = playing ? lighter(rainbow((now * 0.12) % 1)) : 0xb0703a;
      fill(261, 88, 27, 1, SHADOW);
      for (let row = 54; row <= 66; row++) {
        const d = 66 - row, half = (r) => Math.round(Math.sqrt(Math.max(0, r * r - d * d)));
        fill(cx - half(12.5), row, half(12.5) * 2 + 1, 1, chrome);
        if (row >= 56) fill(cx - half(10.5), row, half(10.5) * 2 + 1, 1, tube);
        if (row >= 58) fill(cx - half(8), row, half(8) * 2 + 1, 1, 0x2a1a2a);
      }
      fill(cx - 4, 60, 1, 4, WHITE, 0.3); dot(cx - 3, 59, WHITE, 0.3);
      // the pilasters, a bubble tube in each
      for (const px of [262, 282]) {
        fill(px, 66, 5, 20, wood); fill(px, 66, 1, 20, 0x8a4a2e);
        fill(px + 1, 67, 3, 18, playing ? 0xe0507a : 0x8a3a52);
        for (let k = 0; k < 4; k++) {
          const by = playing ? 84 - ((now * 7 + k * 4.6 + px) % 17) : 70 + k * 4;
          dot(px + 1 + (k % 2) * 2, Math.round(by), WHITE, playing ? 0.85 : 0.4);
        }
      }
      fill(267, 66, 15, 20, wood);
      // the record turning behind glass, the buttons, the grille
      fill(268, 67, 13, 6, PALETTE.doorway); fill(268, 67, 13, 1, WHITE, 0.2);
      fill(271, 69, 7, 3, 0x1a1a1f); dot(274, 70, 0xd96a5a);
      if (playing) dot(271 + (Math.floor(now * 8) % 2) * 6, 70, 0x5a5a66);
      fill(268, 74, 13, 2, chrome); for (let k = 0; k < 6; k++) dot(269 + k * 2, 74, playing && k === 3 ? 0x6fd36f : 0x8a8f9c);
      fill(268, 77, 13, 8, playing ? 0xf0a040 : 0x8a5a2a);
      for (let row = 77; row < 85; row += 2) fill(268, row, 13, 1, chrome);
      fill(262, 85, 25, 1, chrome); fill(262, 86, 25, 2, 0x4a2618);
      if (playing) notes(266, 52, 7);
    }

    /** Neon jukebox. A black cabinet outlined in neon, an equaliser in its window. Playing, the neon pulses on the beat and the bars dance. */
    function drawNeonJukebox() {
      // Playing: the computer's music, with the control on, or someone at it
      const playing = frame.music?.playing === true || atCorner(0);
      const body = 0x14161b, pink = 0xff5aa8, cyan = 0x4ad4e8;
      const pulse = playing ? (onBeat() ? 1 : 0.65) : 0.3;
      fill(263, 88, 23, 1, SHADOW);
      if (playing) fill(261, 52, 27, 37, pink, 0.05 * pulse);
      fill(267, 56, 15, 1, body); fill(265, 57, 19, 31, body);
      const tube = (x, y, c) => { dot(x, y, playing ? lighter(c) : shaded(c, 0.55), playing ? 1 : 0.8); if (playing) fill(x - 1, y - 1, 3, 3, c, 0.07 * pulse); };
      for (let x = 267; x <= 281; x++) tube(x, 57, pink);
      for (let y = 58; y <= 72; y++) { tube(265, y, pink); tube(283, y, pink); }
      fill(268, 60, 13, 10, PALETTE.doorway);
      for (let k = 0; k < 6; k++) {
        const h = playing ? 1 + Math.round(3.5 * (1 + Math.sin(now * (4 + k * 1.3) + k))) : 1;
        for (let r = 0; r < h; r++) dot(269 + k * 2, 68 - r, r > 4 ? pink : r > 2 ? 0xf2d35b : cyan);
      }
      for (let x = 267; x <= 281; x++) { tube(x, 74, cyan); tube(x, 86, cyan); }
      for (let y = 75; y <= 85; y++) { tube(267, y, cyan); tube(281, y, cyan); }
      for (const sx of [271, 278]) {
        disc(sx, 80, 2.5, 0x3a3d47); disc(sx, 80, playing && onBeat() ? 1.5 : 1, 0x1d1f25); dot(sx, 80, 0x6a7080);
      }
      if (playing) notes(268, 53);
    }

    /** Diner booth. A red booth, a wall-box jukebox on the table between two milkshakes. Playing, its title cards flip and its lights chase. */
    function drawBoothBench() {
      const red = 0xb8322a;
      fill(256, 66, 41, 16, red); fill(256, 66, 41, 2, 0xd94a3f);
      for (let x = 260; x < 296; x += 5) fill(x, 68, 1, 13, 0x9a2a2a);
      for (const sx of [256, 293]) { fill(sx, 80, 4, 12, 0x7a1e1e); fill(sx, 80, 4, 1, red); }
    }

    /** Diner booth. A red booth, a wall-box jukebox on the table between two milkshakes. Playing, its title cards flip and its lights chase. */
    function drawBoothTable() {
      // Playing: the computer's music, with the control on, or someone at it
      const playing = frame.music?.playing === true || atCorner(0);
      const chrome = 0xc8cdd6;
      fill(258, 93, 37, 1, SHADOW);
      fill(260, 81, 33, 1, chrome); fill(260, 82, 33, 3, 0xe8e4da); fill(260, 85, 33, 1, 0x9aa0a6);
      for (const [dx, dy] of [[4, 0], [12, 1], [20, 0], [28, 1]]) dot(260 + dx, 82 + dy, 0xc8c4ba);
      fill(275, 86, 3, 6, chrome); fill(272, 92, 9, 1, 0x9aa0a6);
      // the wall-box: a domed chrome case, title cards behind glass, lights along its top
      fill(271, 74, 10, 8, chrome); fill(272, 73, 8, 1, chrome); fill(271, 74, 10, 1, 0xe0e4ea);
      fill(272, 75, 8, 4, 0xf4f1ea);
      const flip = playing ? Math.floor(now * 1.5) % 3 : 0;
      for (let r = 0; r < 4; r++) fill(273, 75 + r, 6, 1, [0xd96a5a, 0x4c8bf5, 0xf4f1ea, 0xd96a5a][(r + flip) % 4], r % 2 ? 0.5 : 0.9);
      for (let k = 0; k < 4; k++) dot(272 + k * 2, 73, playing && (Math.floor(now * 4) + k) % 4 === 0 ? 0xf2d35b : 0x9aa0a6);
      fill(272, 80, 8, 1, 0x5a5f6c); dot(279, 79, 0xd4140b);
      // two milkshakes
      for (const mx of [262, 289]) { fill(mx, 77, 3, 4, 0xf6b8cc); fill(mx, 77, 3, 1, WHITE); fill(mx + 2, 74, 1, 3, 0xd4140b); dot(mx + 1, 76, 0xd4140b); }
      if (playing) notes(271, 68, 4);
    }

    /** DJ booth. Two decks and a mixer, a light strip along the front. Playing, the platters spin, the faders ride, the meters jump and the floor glows. */
    function drawDjBooth() {
      // Playing: the computer's music, with the control on, or someone at it
      const playing = frame.music?.playing === true || atCorner(0);
      fill(251, 92, 50, 1, SHADOW);
      if (playing) fill(244, 90, 64, 8, rainbow((now * 0.15) % 1), 0.08);
      fill(252, 77, 48, 1, 0x5a5f6c); fill(252, 78, 48, 3, 0x3a3d47);
      for (const [dx, phase] of [[254, 0], [285, 1.7]]) {
        fill(dx, 77, 13, 4, 0x2a2c30); fill(dx + 3, 77, 6, 1, 0x14161b); fill(dx + 2, 78, 8, 2, 0x14161b);
        const a = playing ? now * 6 + phase : phase;
        dot(dx + 6 + Math.round(Math.cos(a) * 3), 78 + Math.round(Math.sin(a) * 0.9), 0xd96a5a);
        dot(dx + 11, 77, 0x9aa0a6); dot(dx + 10, 78, 0x9aa0a6); dot(dx + 9, 79, 0x9aa0a6);
      }
      fill(270, 77, 13, 4, 0x1d1f25);
      for (let k = 0; k < 3; k++) { const pos = playing ? Math.round(1 + Math.sin(now * (1.3 + k) + k)) : 1; fill(271 + k * 3, 77, 1, 3, 0x0e0f12); dot(271 + k * 3, 77 + pos, 0xc8cdd6); }
      for (const mx of [280, 281]) { const h = playing ? 1 + Math.round(1.5 * (1 + Math.sin(now * 9 + mx))) : 0; for (let r = 0; r < h; r++) dot(mx, 80 - r, r > 2 ? 0xd4140b : r > 1 ? 0xf2d35b : 0x6fd36f); }
      fill(252, 81, 48, 11, 0x1d1f25); fill(252, 81, 48, 1, 0x2a2c30);
      for (let x = 253; x < 299; x++) dot(x, 85, playing ? rainbow(((x - 253) / 46 + now * 0.5) % 1) : 0x3a3d47);
      fill(255, 88, 42, 1, 0x2a2c30);
      if (playing) notes(266, 62, 8);
    }

    /** Record player. A turntable on a cabinet of records, its lid up, a crate of records beside it. Playing, the arm is down, the record turns and notes rise. */
    function drawRecordPlayer() {
      // Playing: the computer's music, with the control on, or someone at it
      const playing = frame.music?.playing === true || atCorner(0);
      fill(259, 91, 32, 1, SHADOW);
      fill(263, 63, 20, 10, 0xcfe3ef, 0.12); fill(263, 63, 20, 1, 0xcfe3ef, 0.5); fill(263, 63, 1, 10, 0xcfe3ef, 0.35); fill(282, 63, 1, 10, 0xcfe3ef, 0.35);
      fill(260, 77, 30, 1, 0xa87a56); fill(260, 78, 30, 12, 0x8c6744);
      fill(262, 80, 12, 9, 0x3a2616); for (let k = 0; k < 11; k++) fill(263 + k, 81 + (k % 3 === 0 ? 1 : 0), 1, 8 - (k % 3 === 0 ? 1 : 0), RECORDS[k % RECORDS.length]);
      fill(276, 80, 12, 9, 0x3a2616); fill(277, 83, 10, 4, 0x2a2a2e); dot(279, 84, playing ? 0xf0b040 : 0x5a4a2a); dot(284, 84, 0x9aa0a6);
      fill(261, 90, 2, 1, 0x5a3b25); fill(287, 90, 2, 1, 0x5a3b25);
      fill(263, 73, 20, 4, 0xc8cdd6); fill(263, 73, 20, 1, 0xe0e4ea);
      fill(266, 71, 10, 1, 0x14161b); fill(265, 72, 12, 2, 0x14161b); dot(271, 72, 0xd96a5a);
      if (playing) dot(266 + (Math.floor(now * 10) % 10), 72, 0x5a5a66);
      dot(280, 72, 0x9aa0a6);
      if (playing) line(280, 72, 276, 71, 0x9aa0a6); else fill(280, 73, 1, 3, 0x9aa0a6);
      if (playing) notes(266, 61);
    }

    /** Record player. A turntable on a cabinet of records, its lid up, a crate of records beside it. Playing, the arm is down, the record turns and notes rise. */
    function drawRecordCrate() {
      fill(291, 92, 13, 1, SHADOW); fill(292, 84, 11, 8, 0xa07a4a); fill(292, 87, 11, 1, 0x7a5a36); fill(292, 84, 11, 1, 0xb8905a);
      for (let k = 0; k < 5; k++) fill(293 + k * 2, 81 + (k % 2), 2, 3 - (k % 2), RECORDS[(k + 2) % RECORDS.length]);
    }

    /** Boombox. A boombox on a milk crate. Playing, the cones pump, the tape turns and sound rings out. */
    function drawBoombox() {
      // Playing: the computer's music, with the control on, or someone at it
      const playing = frame.music?.playing === true || atCorner(0);
      fill(264, 92, 21, 1, SHADOW);
      fill(265, 80, 19, 12, 0x3f6fc4); fill(265, 80, 19, 1, 0x5a8ad0);
      for (let y = 83; y < 91; y += 3) for (let x = 267; x < 283; x += 3) fill(x, y, 2, 2, 0x2a4a8a);
      fill(266, 66, 17, 1, 0x5a5f6c); fill(266, 67, 1, 2, 0x5a5f6c); fill(282, 67, 1, 2, 0x5a5f6c);
      line(284, 68, 290, 58, 0x9aa0a6); dot(290, 57, 0xc8cdd6);
      fill(262, 69, 25, 11, 0xc8cdd6); fill(262, 69, 25, 1, 0xe0e4ea); fill(262, 79, 25, 1, 0x8a8f9c);
      for (const cx of [267, 282]) {
        disc(cx, 75, 4, 0x3a3d47); disc(cx, 75, 3, 0x1d1f25); disc(cx, 75, playing && onBeat() ? 2.2 : 1.4, 0x4a4e56); dot(cx, 75, 0x8a8f9c);
      }
      fill(271, 72, 8, 5, 0x2a2a2e); fill(272, 73, 6, 2, 0x5a5f6c);
      const turn = playing ? Math.floor(now * 6) % 2 : 0;
      for (const rx of [273, 276]) { dot(rx + turn, 73, 0xeae6dc); dot(rx + 1 - turn, 74, 0xeae6dc); }
      for (let k = 0; k < 4; k++) dot(271 + k * 2, 70, k === 1 && playing ? 0xd4140b : 0x5a5f6c);
      if (playing) {
        for (let k = 0; k < 2; k++) {
          const r = 5 + ((now * 6 + k * 3) % 6), fade = 0.45 * (1 - (r - 5) / 6);
          for (const [cx, dir] of [[267, -1], [282, 1]]) for (let a = -0.6; a <= 0.6; a += 0.2) dot(cx + dir * Math.round(Math.cos(a) * r), 75 + Math.round(Math.sin(a) * r), WHITE, fade);
        }
        notes(268, 62);
      }
    }

    /** Hi-fi stack. A rack of separates between two tall speakers. Playing, the VU needles swing, the reels turn and the woofers pump. */
    function drawHifi() {
      // Playing: the computer's music, with the control on, or someone at it
      const playing = frame.music?.playing === true || atCorner(0);
      fill(250, 92, 52, 1, SHADOW);
      const pump = playing && onBeat();
      for (const sx of [252, 290]) {
        fill(sx, 60, 10, 32, 0x5a3b25); fill(sx, 60, 10, 1, 0x7a5232); fill(sx + 9, 61, 1, 31, 0x3a2616);
        disc(sx + 5, 65, 1.5, 0x2a2a2e); dot(sx + 5, 65, 0x8a8f9c);
        disc(sx + 5, 76, 4, 0x2a2a2e); disc(sx + 5, 76, pump ? 2.6 : 1.8, 0x3a3d47); dot(sx + 5, 76, 0x8a8f9c);
        fill(sx + 3, 86, 4, 2, 0x1d1f25);
      }
      fill(266, 66, 20, 26, 0x2a2a2e);
      fill(267, 63, 18, 3, 0x5a3b25); fill(269, 62, 12, 1, 0x14161b); dot(274, 62, 0xd96a5a);
      fill(267, 67, 18, 6, 0xc8cdd6); fill(267, 67, 18, 1, 0xe0e4ea);
      for (const [vx, ph] of [[268, 0], [276, 1.3]]) {
        fill(vx, 68, 6, 3, playing ? 0xf0b040 : 0x9a7a3a);
        const a = playing ? -0.5 + 0.5 * Math.abs(Math.sin(now * 5 + ph)) : -0.9;
        dot(vx + 3 + Math.round(Math.sin(a) * 2), 70 - Math.round(Math.cos(a) * 2), 0x1d1f25); dot(vx + 3, 70, 0x1d1f25);
      }
      fill(283, 69, 1, 2, 0x5a5f6c);
      fill(267, 74, 18, 6, 0x3a3d47); fill(269, 75, 14, 3, 0x1d1f25);
      const turn = playing ? Math.floor(now * 5) % 2 : 0;
      for (const rx of [271, 279]) { fill(rx, 75, 3, 3, 0x5a5f6c); dot(rx + 1, 76, 0x1d1f25); dot(rx + turn * 2, 75 + turn * 2, 0xeae6dc); }
      fill(267, 81, 18, 4, 0x2a2c30); fill(268, 82, 16, 1, 0x4ad4e8, playing ? 0.7 : 0.3); fill(274, 81, 1, 3, 0xd4140b);
      for (let k = 0; k < 14; k++) fill(268 + k, 87 + (k % 4 === 0 ? 1 : 0), 1, 4 - (k % 4 === 0 ? 1 : 0), RECORDS[k % RECORDS.length]);
      if (playing) { fill(266, 66, 20, 8, 0xf0b040, 0.06); notes(266, 57, 8); }
    }

    /** Synth rig. A keyboard on an X-stand under a modular case, patch cables hanging. Playing, keys light, LEDs blink and the cables sway. */
    function drawSynth() {
      // Playing: the computer's music, with the control on, or someone at it
      const playing = frame.music?.playing === true || atCorner(0);
      fill(257, 92, 40, 1, SHADOW);
      for (const lx of [262, 282]) { line(lx, 81, lx + 10, 91, 0x3a3d47); line(lx + 10, 81, lx, 91, 0x3a3d47); }
      fill(264, 74, 1, 2, 0x5a5f6c); fill(290, 74, 1, 2, 0x5a5f6c);
      fill(262, 60, 30, 14, 0x5a3b25); fill(264, 61, 26, 12, 0x2a2c30);
      for (let k = 1; k < 5; k++) fill(264 + k * 6, 61, 1, 12, 0x1d1f25);
      for (let k = 0; k < 10; k++) dot(266 + (k % 5) * 6, 63 + Math.floor(k / 5) * 3, 0x9aa0a6);
      const leds = [0xd4140b, 0x6fd36f, 0xf2d35b, 0x4ad4e8];
      for (let k = 0; k < 5; k++) dot(268 + k * 6, 69, leds[k % 4], playing ? ((Math.floor(now * (3 + k)) % 2) ? 1 : 0.25) : 0.25);
      for (const [x0, y0, x1, y1, c, ph] of [[266, 66, 278, 64, 0xd96a5a, 0], [272, 70, 286, 66, 0xf2d35b, 1], [280, 63, 288, 70, 0x4c8bf5, 2]]) {
        const sag = 4 + (playing ? Math.sin(now * 2 + ph) * 0.8 : 0);
        for (let t = 0; t <= 1; t += 0.08) dot(Math.round(x0 + (x1 - x0) * t), Math.round(y0 + (y1 - y0) * t + Math.sin(t * Math.PI) * sag), c);
      }
      fill(258, 76, 38, 5, 0x2a2d35); fill(258, 76, 38, 1, 0x3a3d47);
      for (let k = 0; k < 6; k++) dot(260 + k * 2, 77, 0x9aa0a6);
      fill(285, 77, 8, 2, 0x1a3a2a); fill(285, 77, playing ? 1 + (Math.floor(now * 4) % 7) : 2, 1, 0x6fd36f);
      fill(260, 79, 34, 2, 0xf4f1ea);
      for (let k = 0; k < 34; k++) if ([1, 2, 4, 5, 6].includes(k % 7)) dot(260 + k, 79, 0x1d1f25);
      if (playing) for (let k = 0; k < 3; k++) dot(260 + ((Math.floor(now * 6) + k * 11) % 34), 80, 0xc05aff);
      if (playing) notes(266, 56, 8);
    }

    /* The pet */

    /**
     * The pet, a few pixels high, facing the way it's going: walking, or
     * settled the way its place has it (sitting, lying, asleep, clinging).
     */
    function drawPet(pet) {
      const x = Math.round(pet.position.x), y = Math.round(pet.position.y);
      // Drawn facing right; facing left mirrors it about x.
      const p = (dx, dy, w, h, colour, alpha) => fill(pet.facing > 0 ? x + dx : x - dx - w + 1, y + dy, w, h, colour, alpha);
      const step = pet.isWalking ? Math.floor(now * 8) % 2 : 0;
      const pose = pet.isWalking ? 'walk' : pet.target ? pet.target.pose : 'sit';
      const flying = PETS[pet.kind].flies && pet.isWalking;
      const grounded = (!pet.target || !pet.target.depth || pet.isWalking) && !flying && !['axolotl', 'hamster'].includes(pet.kind);
      if (grounded) p(-4, 0, 9, 1, SHADOW);
      switch (pet.kind) {
        case 'cat': {
          const fur = 0xb8b3a8, dark = 0x77726b;
          if (pose === 'sleep') {
            p(-3, -3, 7, 3, fur);
            p(-3, -3, 7, 1, dark);
            p(3, -4, 2, 1, fur);
            const rise = (now * 0.4) % 1;
            fill(x + 3 + Math.round(rise * 2), y - 6 - Math.round(rise * 5), 2, 1, WHITE, 1 - rise);
            break;
          }
          if (pose === 'walk') {
            p(-3, -4, 6, 3, fur);
            p(2, -6, 3, 3, fur);
            p(2, -7, 1, 1, fur);
            p(4, -7, 1, 1, fur);
            p(-4, -7, 1, 3, dark);
            p(-3, -1, 1, 1, dark);
            p(1 + step, -1, 1, 1, dark);
            p(4, -5, 1, 1, PALETTE.eye);
            break;
          }
          p(-2, -5, 4, 5, fur);
          p(-1, -8, 4, 3, fur);
          p(-1, -9, 1, 1, fur);
          p(2, -9, 1, 1, fur);
          p(2, -7, 1, 1, PALETTE.eye);
          p(-4, -1, 3, 1, dark);
          p(-4, -2 - (Math.floor(now * 1.2) % 2), 1, 1, dark);
          break;
        }
        case 'dog': {
          const fur = 0x9a6a3a, dark = 0x6e4a26;
          const wag = Math.floor(now * 6) % 2;
          if (pose === 'lie') {
            p(-4, -3, 9, 3, fur);
            p(4, -5, 3, 3, fur);
            p(4, -5, 1, 3, dark);
            p(7, -4, 1, 1, PALETTE.eye);
            p(-5, -2 - wag, 1, 1, dark);
            break;
          }
          p(-4, -6, 8, 3, fur);
          p(3, -9, 4, 4, fur);
          p(3, -9, 1, 3, dark);
          p(7, -7, 1, 1, PALETTE.eye);
          p(5, -8, 1, 1, PALETTE.eye);
          p(-5, -8 - wag, 1, 2, dark);
          if (pose === 'sit') {
            p(-4, -3, 3, 3, fur);
            p(2, -3, 1, 3, dark);
          } else {
            p(-3, -3, 1, 3 - step, dark);
            p(-1, -3, 1, 2 + step, dark);
            p(1, -3, 1, 3 - step, dark);
            p(3, -3, 1, 2 + step, dark);
          }
          break;
        }
        case 'lizard': {
          const skin = 0x5aa84a, dark = 0x3d7a33;
          p(-3, -2, 6, 2, skin);
          p(3, -2, 2, 2, skin);
          p(4, -2, 1, 1, PALETTE.eye);
          p(-6, -1, 3, 1, dark);
          p(-2, 0 - step, 1, 1, dark);
          p(2, -1 + step, 1, 1, dark);
          p(-2, -3 + step, 1, 1, dark);
          p(2, -3, 1, 1, dark);
          break;
        }
        case 'monkey': {
          const fur = 0x7a5232, face = 0xd9b48a;
          p(-2, -6, 4, 5, fur);
          p(-2, -9, 4, 3, fur);
          p(-1, -8, 3, 2, face);
          p(-3, -8, 1, 1, fur);
          p(2, -8, 1, 1, fur);
          p(1, -8, 1, 1, PALETTE.eye);
          p(-3, -2, 1, 1, fur);
          p(-4, -4, 1, 3, fur);
          p(-4, -5, 2, 1, fur);
          if (pose === 'walk') {
            p(-2, -1, 1, 1 + step, fur);
            p(1, -1, 1, 2 - step, fur);
          } else {
            p(-2, -1, 4, 1, fur);
          }
          if (pet.carrying) p(2, -5, 2, 2, PALETTE.mug);
          break;
        }
        case 'llama': {
          const wool = 0xeee3cf, shade = 0xcfc2a8, hoof = 0x6b5a48, face = 0xe2d4b8;
          // Spitting after an agent who's been fired, on their way out
          const fired = office.people.find((q) => q.isFired && q.isLeaving);
          const spits = fired && Math.hypot(fired.position.x - x, fired.position.y - y) < 120;
          if (spits) pet.facing = fired.position.x > x ? 1 : -1;
          const chew = pose === 'chew' || spits ? Math.floor(now * 2.5) % 2 : 0;
          if (pose === 'lie') {
            // Kushed: legs folded under, neck up
            p(-4, -4, 9, 4, wool);
            p(-4, -1, 9, 1, shade);
            p(-5, -4, 1, 2, wool);
          } else {
            p(-4, -7, 9, 4, wool);
            p(-4, -4, 9, 1, shade);
            p(-5, -7, 1, 2, wool);
            // Legs, stepping as it walks
            for (const [lx, k] of [[-3, 0], [-1, 1], [2, 0], [4, 1]]) p(lx, -3, 1, 3 - (step && k ? 1 : 0), shade);
            for (const lx of [-3, -1, 2, 4]) p(lx, -1, 1, 1, hoof);
          }
          const up = pose === 'lie' ? 3 : 0;
          // A blanket on its back, in stripes
          p(-2, -8 + up, 4, 1, 0xd4140b);
          p(-2, -7 + up, 4, 1, 0x2f9a92);
          // The long neck and the head, its ears up, chewing
          p(3, -12 + up, 2, 5, wool);
          p(3, -14 + up, 3, 3, wool);
          p(5, -13 + up, 2, 2 - chew, face);
          p(3, -16 + up, 1, 2, shade);
          p(5, -16 + up, 1, 2, shade);
          p(5, -14 + up, 1, 1, PALETTE.eye);
          if (spits) {
            const mouth = { x: x + pet.facing * 6, y: y - 12 + up }, to = { x: fired.position.x, y: fired.position.y - 10 };
            for (const lag of [0, 0.12]) {
              const f = (now * 1.1 + lag) % 1;
              dot(Math.round(mouth.x + (to.x - mouth.x) * f), Math.round(mouth.y + (to.y - mouth.y) * f - Math.sin(f * Math.PI) * 10), 0xdfe8ea, 1 - f * 0.4);
            }
          }
          break;
        }
        case 'penguin': {
          const black = 0x1e2433, white = 0xf4f1ea, beak = 0xff8a1f;
          // A long way, on its belly
          if (pose === 'walk' && pet.leg > 50) {
            p(-4, -3, 8, 2, black);
            p(-3, -1, 7, 1, white);
            p(4, -3, 1, 1, beak);
            p(2, -3, 1, 1, PALETTE.eye);
            p(-6, -2, 2, 1, beak);
            break;
          }
          const sway = pose === 'walk' ? (step ? 1 : -1) : 0;
          p(-2 + sway, -8, 5, 8, black);
          p(0 + sway, -6, 2, 5, white);
          p(3 + sway, -7, 1, 1, beak);
          p(1 + sway, -7, 1, 1, Math.floor(now * 0.8) % 7 === 0 ? black : white);
          p(1 + sway, -7, 1, 1, PALETTE.eye);
          p(-3 + sway, -6, 1, 3, black);
          p(-1, 0, 2, 1, beak);
          p(2, 0, 2, 1, beak);
          break;
        }
        case 'parrot': {
          const red = 0xd4140b, blue = 0x3f6fc4, yellow = 0xf2d35b;
          if (flying) {
            const flap = Math.floor(now * 10) % 2;
            p(-2, -5, 5, 2, red);
            p(3, -6, 2, 2, red);
            p(5, -6, 1, 1, yellow);
            p(4, -6, 1, 1, PALETTE.eye);
            p(-1, flap ? -8 : -3, 3, 3, blue);
            p(0, flap ? -9 : -2, 1, 1, yellow);
            p(-5, -5, 3, 1, blue);
            break;
          }
          p(-1, -6, 3, 4, red);
          p(0, -8, 3, 2, red);
          p(3, -8, 1, 2, yellow);
          p(2, -7, 1, 1, PALETTE.eye);
          p(-1, -5, 2, 3, blue);
          p(-1, -3, 1, 1, yellow);
          p(-2, -2, 1, 3, blue);
          p(0, -2, 1, 1, 0x3a3a40);
          p(1, -2, 1, 1, 0x3a3a40);
          break;
        }
        case 'owl': {
          const feather = 0x8a6a4a, disc = 0xe8dcc4, yellow = 0xf2d35b;
          if (flying) {
            const flap = Math.floor(now * 7) % 2;
            p(-2, -6, 5, 3, feather);
            p(-6, flap ? -8 : -4, 4, 2, feather);
            p(3, flap ? -8 : -4, 4, 2, feather);
            p(-1, -6, 3, 1, disc);
            break;
          }
          // Asleep by day; at night wide awake, its head turning
          const turn = isNight ? [0, 0, 1, 0, 0, -1][Math.floor(now * 0.8) % 6] : 0;
          fill(x - 3, y - 8, 7, 8, feather);
          fill(x - 3, y - 9, 1, 1, feather);
          fill(x + 3, y - 9, 1, 1, feather);
          fill(x - 2 + turn, y - 7, 5, 3, disc);
          if (isNight) {
            fill(x - 2 + turn, y - 7, 2, 2, yellow);
            fill(x + 1 + turn, y - 7, 2, 2, yellow);
            dot(x - 1 + turn, y - 6, PALETTE.eye);
            dot(x + 2 + turn, y - 6, PALETTE.eye);
          } else {
            fill(x - 2, y - 6, 2, 1, PALETTE.eye);
            fill(x + 1, y - 6, 2, 1, PALETTE.eye);
            const rise = (now * 0.35) % 1;
            fill(x + 4 + Math.round(rise * 2), y - 10 - Math.round(rise * 5), 2, 1, WHITE, 1 - rise);
          }
          dot(x + turn, y - 5, 0xd9a03a);
          fill(x - 1, y - 3, 3, 2, shaded(feather, 1.2));
          fill(x - 1, y, 1, 1, 0xd9a03a);
          fill(x + 1, y, 1, 1, 0xd9a03a);
          break;
        }
        case 'capybara': {
          const fur = 0x9a6a3a, dark = 0x7a5230;
          if (pose === 'soak') {
            // In the hot tub to the chin, the orange still on its head
            p(-2, -4, 6, 3, fur);
            p(3, -3, 1, 1, 0x4a3020);
            p(1, -4, 1, 1, PALETTE.eye);
            p(0, -6, 2, 2, 0xff8a1f);
            p(1, -7, 1, 1, PALETTE.leaf);
            break;
          }
          p(-4, -5, 9, 4, fur);
          p(-4, -5, 9, 1, shaded(fur, 1.1));
          p(3, -6, 4, 4, fur);
          p(6, -4, 1, 1, 0x4a3020);
          p(4, -5, 1, 1, PALETTE.eye);
          p(3, -7, 1, 1, dark);
          if (pose === 'walk') {
            for (const [lx, k] of [[-3, 0], [-1, 1], [2, 0], [4, 1]]) p(lx, -1, 1, 1 - (step && k ? 1 : 0), dark);
          } else {
            p(-3, -1, 8, 1, dark);
          }
          // A little orange on its head, and its leaf
          p(4, -9, 2, 2, 0xff8a1f);
          p(5, -10, 1, 1, PALETTE.leaf);
          break;
        }
        case 'redpanda': {
          const rust = 0xc0582a, dark = 0x3a2616, light = 0xe08a5a;
          // The red button gone off: up on its hind legs, arms in the air
          const startled = office.detonation && now - office.detonation.at < 2.5;
          if (startled) {
            p(-1, -9, 3, 8, rust);
            p(-1, -4, 3, 3, dark);
            p(-2, -12, 1, 3, dark);
            p(2, -12, 1, 3, dark);
            p(-1, -12, 3, 3, rust);
            p(-1, -11, 3, 1, WHITE);
            p(0, -11, 1, 1, PALETTE.eye);
            p(-3, -3, 2, 2, light);
            break;
          }
          p(-3, -5, 6, 3, rust);
          p(-3, -2, 6, 1, dark);
          p(2, -7, 4, 4, rust);
          p(4, -5, 2, 1, WHITE);
          p(3, -6, 1, 1, PALETTE.eye);
          p(2, -8, 1, 1, WHITE);
          p(5, -8, 1, 1, WHITE);
          // The ringed tail
          for (let k = 0; k < 4; k++) p(-7 + k, -5 + (pose === 'walk' ? 0 : 1), 1, 2, k % 2 ? light : rust);
          if (pose === 'walk') {
            p(-2, -1, 1, 1 + step, dark);
            p(1, -1, 1, 2 - step, dark);
          } else {
            p(-2, -1, 4, 1, dark);
          }
          break;
        }
        case 'corgi': {
          const fur = 0xd9893a, white = 0xf4f1ea;
          if (pose === 'lie') {
            p(-4, -3, 8, 3, fur);
            p(-4, -1, 8, 1, white);
            p(3, -5, 3, 3, fur);
            p(3, -7, 1, 2, fur);
            p(5, -7, 1, 2, fur);
            p(5, -4, 1, 1, white);
            p(4, -5, 1, 1, PALETTE.eye);
            break;
          }
          const zoom = pet.zoom ? 1 : 0;
          p(-4, -5 - zoom * (Math.floor(now * 12) % 2), 8, 3, fur);
          p(-4, -3, 8, 1, white);
          p(-5, -5, 1, 2, white);
          p(3, -8, 4, 4, fur);
          p(3, -10, 1, 2, fur);
          p(5, -10, 1, 2, fur);
          p(6, -6, 1, 1, white);
          p(5, -7, 1, 1, PALETTE.eye);
          if (pose === 'walk') {
            p(-3, -2, 1, 2 - step, white);
            p(2, -2, 1, 1 + step, white);
          } else {
            p(-3, -2, 1, 2, white);
            p(2, -2, 1, 2, white);
          }
          break;
        }
        case 'hamster': {
          // In its ball, rolling: the hamster running inside it
          const cx = x, cy = y - 4;
          for (let k = 0; k < 16; k++) {
            const a = (k / 16) * Math.PI * 2;
            dot(Math.round(cx + Math.cos(a) * 4), Math.round(cy + Math.sin(a) * 4), 0xcfe3ef, 0.75);
          }
          fill(cx - 3, cy - 3, 7, 7, 0xcfe3ef, 0.18);
          const roll = (pet.position.x * 0.5) % (Math.PI * 2);
          dot(Math.round(cx + Math.cos(roll) * 4), Math.round(cy + Math.sin(roll) * 4), WHITE);
          const run = pose === 'walk' ? step : 0;
          p(-1, -4, 3, 2, 0xd9a066);
          p(-1, -3, 3, 1, WHITE);
          p(1, -5, 1, 1, 0xd9a066);
          p(1, -4, 1, 1, PALETTE.eye);
          p(-1 + run, -2, 1, 1, 0xd96a8a);
          fill(cx - 3, y, 7, 1, SHADOW);
          break;
        }
        case 'robodog': {
          const metal = 0x9aa0a6, dark = 0x5a5f6c, glow = 0x4ad4e8;
          if (pose === 'charge') {
            // On its pad, charging, the bolt pulsing
            const pulse = 0.5 + 0.5 * Math.sin(now * 3);
            fill(x - 6, y - 1, 13, 2, 0x2a2d35);
            fill(x - 5, y - 1, 11, 1, glow, 0.4 + 0.4 * pulse);
            p(-4, -4, 8, 3, metal);
            p(3, -6, 4, 3, metal);
            p(5, -5, 2, 1, glow);
            fill(x - 1, y - 10, 2, 1, 0xf2d35b, pulse);
            fill(x, y - 9, 2, 1, 0xf2d35b, pulse);
            break;
          }
          p(-4, -6, 8, 3, metal);
          p(-4, -6, 8, 1, shaded(metal, 1.15));
          p(3, -9, 4, 4, metal);
          p(5, -8, 2, 1, glow);
          p(4, -11, 1, 2, dark);
          p(4, -12, 1, 1, Math.floor(now * 2) % 2 ? 0xd4140b : 0x3ddc84);
          p(-5, -7, 1, 1, dark);
          for (const [lx, k] of [[-3, 0], [-1, 1], [1, 0], [3, 1]]) p(lx, -3, 1, 3 - (pose === 'walk' && step === k ? 1 : 0), dark);
          break;
        }
        case 'bat': {
          const fur = 0x3a2e4a, wing = 0x2a2236;
          if (pose === 'hang' && !pet.isWalking) {
            // Upside down from the ceiling, wrapped in its wings
            fill(x, y, 1, 1, wing);
            fill(x - 1, y + 1, 3, 5, wing);
            fill(x, y + 2, 1, 3, fur);
            dot(x - 1, y + 6, fur);
            dot(x + 1, y + 6, fur);
            dot(x, y + 5, 0xd4140b);
            break;
          }
          const flap = Math.floor(now * 12) % 2;
          p(-1, -5, 3, 3, fur);
          p(-5, flap ? -7 : -4, 4, 2, wing);
          p(2, flap ? -7 : -4, 4, 2, wing);
          p(0, -6, 1, 1, fur);
          p(2, -6, 1, 1, fur);
          break;
        }
        case 'snail': {
          for (const tr of pet.trail) dot(tr.x, tr.y, 0xdfe8ea, 0.5 * (1 - (now - tr.at) / TRAIL_FOR));
          const body = 0xc8c4a0, shell = 0xb8763a;
          const stretch = pose === 'walk' ? Math.floor(now * 1.5) % 2 : 0;
          p(-3, -1, 7 + stretch, 1, body);
          p(3 + stretch, -2, 2, 1, body);
          p(4 + stretch, -4, 1, 2, body);
          dot(pet.facing > 0 ? x + 4 + stretch : x - 4 - stretch, y - 5, PALETTE.eye);
          p(-2, -5, 4, 4, shell);
          p(-1, -6, 2, 1, shell);
          p(-1, -4, 2, 2, 0xe0a060);
          p(0, -4, 1, 1, shell);
          break;
        }
        case 'fox': {
          const fur = 0xe0703a, white = 0xf4f1ea, dark = 0x3a2616;
          if (pose === 'curl') {
            // Curled up, its tail round it, asleep
            p(-4, -3, 8, 3, fur);
            p(-4, -3, 8, 1, shaded(fur, 1.1));
            p(2, -4, 3, 2, fur);
            p(3, -3, 1, 1, PALETTE.eye);
            p(-5, -2, 2, 2, fur);
            p(4, -1, 2, 1, white);
            const rise = (now * 0.4) % 1;
            fill(x + 3 + Math.round(rise * 2), y - 6 - Math.round(rise * 5), 2, 1, WHITE, 1 - rise);
            break;
          }
          // A pounce, once it gets where it's going
          const to = pet.target && !pet.target.follow ? Math.hypot(pet.target.x - pet.position.x, pet.target.y - pet.position.y) : Infinity;
          const hop = pose === 'walk' && to < 8 ? Math.round(Math.sin((1 - to / 8) * Math.PI) * 5) : 0;
          const lift = (dx, dy, w, h, c) => p(dx, dy - hop, w, h, c);
          lift(-3, -5, 6, 3, fur);
          lift(-2, -3, 4, 1, white);
          lift(2, -8, 4, 4, fur);
          lift(6, -6, 1, 1, dark);
          lift(4, -5, 2, 1, white);
          lift(4, -7, 1, 1, PALETTE.eye);
          lift(2, -9, 1, 1, dark);
          lift(4, -9, 1, 1, dark);
          lift(-7, -6, 4, 2, fur);
          lift(-8, -6, 1, 2, white);
          if (pose === 'walk') {
            lift(-2, -2, 1, 2 - step, dark);
            lift(1, -2, 1, 1 + step, dark);
          } else {
            lift(-2, -2, 1, 2, dark);
            lift(1, -2, 1, 2, dark);
          }
          break;
        }
        case 'axolotl': {
          // In a tank on the counter, swimming from end to end
          const water = 0x7ab8d8, pink = 0xf2a0b8, gill = 0xd96a8a;
          fill(x - 7, y - 8, 14, 8, PALETTE.frame);
          fill(x - 6, y - 7, 12, 7, water, 0.55);
          fill(x - 6, y - 7, 12, 1, WHITE, 0.3);
          fill(x - 6, y - 1, 12, 1, 0xd8c8a0);
          const swim = Math.sin(now * 0.6), ax = Math.round(x + swim * 3), dir = Math.cos(now * 0.6) > 0 ? 1 : -1;
          const q = (dx, dy, w, h, c) => fill(dir > 0 ? ax + dx : ax - dx - w + 1, y + dy, w, h, c);
          q(-2, -4, 4, 2, pink);
          q(2, -5, 2, 2, pink);
          q(3, -6, 1, 1, gill);
          q(3, -3, 1, 1, gill);
          q(-4, -4, 2, 1, pink);
          q(3, -5, 1, 1, PALETTE.eye);
          if (Math.floor(now * 0.7) % 5 === 0) dot(ax + dir * 4, y - 7 - (Math.floor(now * 4) % 3), WHITE);
          break;
        }
      }
    }

    /** What the lounge's corner piece is made of, each part at the depth it stands at. */
    const CORNER_PARTS = {
      arcade: [[86, drawArcade]], vending: [[86, drawVending]], foosball: [[89, drawFoosball]], fishtank: [[86, drawFishTank]],
      books: [[86, drawBookcase], [90, drawBeanbagBack], [94.5, drawBeanbagFront]],
      dragon: [[96, drawDragon]], pinball: [[90, drawPinball]], jukebox: [[88, drawJukebox]], pool: [[92, drawPool]],
      piano: [[82, drawPiano], [91, drawPianoBench]], drums: [[93, drawDrums]], telescope: [[92, drawTelescope]],
      jellyfish: [[91, drawJellyfish]], bonsai: [[91, drawBonsai]], jungle: [[93, drawJungle]], coffee: [[93, drawCoffeeBar]],
      servers: [[89, drawServers]], printer: [[91, drawPrinter]], boxing: [[93, drawBoxing]],
      treadmill: [[80, drawTreadmillConsole], [88, drawTreadmillBelt]], hottub: [[80, drawHotTubBack], [89, drawHotTubFront]],
      ballpit: [[80, drawBallPitBack], [89, drawBallPitFront]], campfire: [[92.5, drawCampfire], [93, drawCampfireLogs]],
      bubbler: [[88.5, drawBubbler]], neonbox: [[88, drawNeonJukebox]], wallbox: [[78, drawBoothBench], [88, drawBoothTable]],
      djtable: [[90, drawDjBooth]], turntable: [[90, drawRecordPlayer], [92, drawRecordCrate]], boombox: [[92, drawBoombox]],
      hifi: [[92, drawHifi]], synth: [[88, drawSynth]],
      none: [],
    };

    /** A rally starts whenever both ends of the table are taken. */
    function drawPingPong() {
      fill(246, 148, 46, 9, PALETTE.table);
      fill(246, 148, 46, 1, PALETTE.tableLine);
      fill(246, 156, 46, 1, PALETTE.tableLine);
      fill(246, 148, 1, 9, PALETTE.tableLine);
      fill(291, 148, 1, 9, PALETTE.tableLine);
      fill(246, 152, 46, 1, PALETTE.tableLine, 0.6);
      fill(268, 146, 2, 11, PALETTE.frame);
      fill(246, 157, 46, 1, PALETTE.tableLeg);
      fill(248, 157, 2, 5, PALETTE.tableLeg);
      fill(288, 157, 2, 5, PALETTE.tableLeg);

      const players = PING_PONG.filter((spot) =>
        office.people.some((p) => p.place === spotPlace(spot) && !p.isWalking && !p.isLeaving));
      if (players.length !== 2) return;
      const lap = (now * 0.9) % 2;
      const across = lap < 1 ? lap : 2 - lap;
      dot(249 + Math.round(across * 40), 151 - Math.round(Math.abs(Math.sin(across * Math.PI * 2)) * 4), WHITE);
    }

    function drawPlant() {
      const x = PLANT_X;
      fill(x, 158, 4, 6, PALETTE.leaf);
      fill(x + 4, 154, 4, 10, PALETTE.leafDark);
      fill(x + 7, 159, 4, 6, PALETTE.leaf);
      fill(x + 2, 162, 7, 6, PALETTE.leaf);
      fill(x + 2, 168, 7, 8, PALETTE.pot);
      fill(x + 1, 168, 9, 1, 0x8a4f32);
      fill(x + 2, 176, 7, 1, SHADOW);
    }

    /**
     * The office's microphone, on a stand: the deck's 🎙. Its on-air light
     * glows while it's held; with no voice input in the picked session, it's
     * dimmed.
     */
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
    };

    function drawMic() {
      const x = MIC.x + wobble('mic'), y = MIC.y, { pole } = MIC_KINDS[decor.mic];
      const a = mic.usable ? 1 : 0.4;
      fill(x - 4, y, 9, 1, SHADOW);
      fill(x - 3, y - 1, 7, 1, PALETTE.machine, a);
      fill(x - 1, y - 2, 3, 1, PALETTE.machine, a);
      fill(x, y - pole, 1, pole - 2, PALETTE.monitorStand, a);
      MIC_HEADS[decor.mic](x, y, a, mic.held);
      // The on-air light, on the stand's clip
      fill(x - 1, y - 15, 3, 2, PALETTE.frame, a);
      if (mic.held) {
        fill(x - 2, y - 16, 5, 3, PALETTE.alarm, 0.3);
        fill(x - 1, y - 17, 3, 5, PALETTE.alarm, 0.3);
        fill(x, y - 15, 1, 1, PALETTE.alarm);
      } else {
        fill(x, y - 15, 1, 1, PALETTE.alarmOff, a);
      }
    }

    /**
     * The nuclear button: a red dome on a post in hazard stripes, like the
     * ☢ Max button on the deck. The dome sinks while it's pressed.
     */
    function drawButton() {
      const x = BUTTON.x + wobble('button'), y = BUTTON.y;
      const pressedAt = office.nuclearQueue[0] ? office.nuclearQueue[0].pressedAt : null;
      const sink = frame.buttonDown || (pressedAt !== null && now - pressedAt < 0.5) ? 1 : 0;
      for (let row = 0; row < 9; row++) {
        for (let column = 0; column < 11; column++) {
          dot(x - 5 + column, y + 6 + row, Math.floor((column + row) / 2) % 2 === 0 ? PALETTE.hazard : PALETTE.machine);
        }
      }
      fill(x - 7, y + 15, 15, 1, PALETTE.frame);
      fill(x - 7, y + 16, 15, 1, SHADOW);
      fill(x - 6, y + 4, 13, 2, PALETTE.machine);
      fill(x - 2, y + sink, 5, 1, PALETTE.domeLight);
      fill(x - 3, y + 1 + sink, 7, 3 - sink, PALETTE.dome);
      dot(x - 2, y + 1 + sink, PALETTE.domeLight);
      if (sink === 0) fill(x - 3, y + 3, 7, 1, PALETTE.domeDark);
    }

    /* People */

    /** The one the mic talks to: while it's held, and for the nod once it's let go. */
    const isListener = (person) => !person.isStaff && person.session.id === mic.listener;

    /**
     * A person is 17 art pixels tall, standing: two rows of hair, five of
     * face, six of shirt, a belt, two of legs and a row of shoes.
     */
    function drawPerson(person) {
      const pose = poseOf(person, now);
      const x = Math.round(person.position.x);
      // The listener nods, twice, once the mic is let go.
      const since = mic.releasedAt === null ? Infinity : now - mic.releasedAt;
      const nod = isListener(person) && since < NOD_LENGTH && Math.floor((since / NOD_LENGTH) * 4) % 2 === 0 ? 1 : 0;
      const top = topOf(person, pose) + nod;
      const look = lookFor(person), away = person.isFacingAway;
      const outfit = person.isStaff ? staffOutfitOf(look, crewDress)
        : outfitOf(person.session.agent, person.session.model, memberOf(person.session), workspaceOf(person.session, person.desk));
      const shirtColour = outfit.shirt, parts = OUTFIT_PARTS[outfit.style];
      // Sleeves are the shirt's, or a white shirt's under a waistcoat, or bare arms.
      const sleeve = (px, py, w, h) => {
        ctx.fillStyle = outfit.sleeves === 'skin' ? css(look.skin) : css(outfit.sleeves ?? shirtColour, 1, 0.78);
        ctx.fillRect(ox + px * scale, oy + py * scale, w * scale, h * scale);
      };

      // Legs, for anyone not sitting down: bare below shorts, or a robe's to the floor
      if (pose.kind !== 'sit' && pose.kind !== 'slump' && pose.kind !== 'wallet') {
        const lift = pose.kind === 'walk' ? (pose.step === 0 ? [1, 0] : [0, 1]) : pose.kind === 'tap' ? [0, pose.beat] : [0, 0];
        fill(x - 3, top + 17, 7, 1, SHADOW);
        fill(x - 3, top + 13, 7, 1, outfit.pants);
        fill(x - 2, top + 14 - lift[0], 2, 2, outfit.pants);
        fill(x + 1, top + 14 - lift[1], 2, 2, outfit.pants);
        if (outfit.legs === 'skin') {
          fill(x - 2, top + 15 - lift[0], 2, 1, look.skin);
          fill(x + 1, top + 15 - lift[1], 2, 1, look.skin);
        }
        if (outfit.robe && pose.kind !== 'walk') fill(x - 3, top + 14, 7, 2, outfit.pants);
        fill(x - 2, top + 16 - lift[0], 2, 1, outfit.shoes);
        fill(x + 1, top + 16 - lift[1], 2, 1, outfit.shoes);
      }

      // Shirt and arms
      fill(x - 3, top + 7, 7, 6, shirtColour);
      if (!away) drawOutfitFront(x, top + 7, 3, 6, outfit, look);
      else if (parts && parts.back) parts.back(x, top + 7, outfit, look);
      if (pose.kind === 'knock') {
        // Back to us, knocking at the door
        sleeve(x - 4, top + 8, 1, 4);
        dot(x - 4, top + 12, look.skin);
        sleeve(x + 4, top + 4 + pose.beat, 1, 4);
        dot(x + 4, top + 3 + pose.beat, look.skin);
      } else if (pose.kind === 'pockets') {
        // Patting their pockets for a key that isn't there
        const pat = Math.floor(now * 3 + look.phase * 3) % 2;
        sleeve(x - 4, top + 8, 1, 4);
        sleeve(x + 4, top + 8, 1, 4);
        dot(x - 3 - pat, top + 12 + pat, look.skin);
        dot(x + 3 + pat, top + 13 - pat, look.skin);
      } else if (pose.kind === 'wallet') {
        // Holding up an empty wallet, a moth fluttering out of it
        sleeve(x - 4, top + 8, 1, 4);
        dot(x - 4, top + 12, look.skin);
        sleeve(x + 4, top + 6, 1, 4);
        dot(x + 4, top + 5, look.skin);
        fill(x + 3, top + 2, 4, 3, 0x6e4a2e);
        fill(x + 4, top + 3, 2, 1, 0x3a2616);
        const flutter = (now * 0.5 + look.phase) % 1, wing = Math.floor(now * 10) % 2;
        if (flutter < 0.8) {
          const mx = x + 5 + Math.round(Math.sin(flutter * 12) * 2), my = top - Math.round(flutter * 10);
          dot(mx, my, 0xc8c4ba);
          dot(mx - 1, my - wing, 0xe8e4da);
          dot(mx + 1, my - wing, 0xe8e4da);
        }
      } else if (pose.kind === 'wave') {
        sleeve(x - 4, top + 8, 1, 4);
        dot(x - 4, top + 12, look.skin);
        sleeve(x + 4, top + 4, 1, 4);
        dot(x + 4 + pose.frame, top + 3, look.skin);
      } else if (pose.kind === 'cheer') {
        // Both arms up, punching the air
        sleeve(x - 4, top + 4 + pose.frame, 1, 4);
        dot(x - 4, top + 3 + pose.frame, look.skin);
        sleeve(x + 4, top + 5 - pose.frame, 1, 4);
        dot(x + 4, top + 4 - pose.frame, look.skin);
      } else if (pose.kind === 'laptop') {
        // Standing with a laptop held open, its lid to us, its screen's light round its edge in their state's colour
        const state = person.session.state, ly = top + 8, beat = state === 'thinking' ? pose.beat : 0;
        sleeve(x - 4, top + 8, 1, 3);
        sleeve(x + 4, top + 8, 1, 3);
        fill(x - 3, ly - 1, 7, 1, colour(state), state === 'idle' ? 0.4 : 0.9);
        fill(x - 3, ly, 7, 4, PALETTE.monitor);
        fill(x - 2, ly + 1, 5, 2, PALETTE.monitorPanel);
        fill(x - 4, ly + 4, 9, 1, PALETTE.keys);
        // Typing while they think
        dot(x - 4, ly + 3 + beat, look.skin);
        dot(x + 4, ly + 4 - beat, look.skin);
      } else if (pose.kind === 'slump') {
        sleeve(x - 4, top + 6, 1, 3);
        sleeve(x + 4, top + 6, 1, 3);
      } else if (pose.kind === 'write') {
        // Back to us, writing on the whiteboard
        sleeve(x - 4, top + 8, 1, 4);
        dot(x - 4, top + 12, look.skin);
        sleeve(x + 4, top + 3, 1, 5);
        dot(x + 4 + pose.stroke, top + 2, look.skin);
      } else if (pose.kind === 'shred') {
        // Feeding a sheet into the shredder on their left
        const feed = (now * 1.2 + look.phase) % 1;
        sleeve(x - 4, top + 8, 1, 2);
        sleeve(x - 5, top + 9, 1, 1);
        dot(x - 6, top + 10, look.skin);
        fill(x - 9, top + 8 + Math.round(feed * 2), 3, 2, PALETTE.paper);
        sleeve(x + 4, top + 8, 1, 4);
        dot(x + 4, top + 12, look.skin);
      } else if (pose.kind === 'press') {
        // Hand high, then down on the dome
        sleeve(x - 4, top + 8, 1, 4);
        dot(x - 4, top + 12, look.skin);
        if (pose.raised) {
          sleeve(x + 4, top + 5, 1, 3);
          sleeve(x + 5, top + 3, 1, 2);
          fill(x + 5, top + 1, 1, 2, look.skin);
        } else {
          sleeve(x + 4, top + 8, 1, 2);
          sleeve(x + 5, top + 10, 1, 2);
          sleeve(x + 6, top + 12, 1, 1);
          fill(x + 7, top + 13, 1, 2, look.skin);
        }
      } else {
        sleeve(x - 4, top + 8, 1, 4);
        sleeve(x + 4, top + 8, 1, 4);
        dot(x - 4, top + 12, look.skin);
        dot(x + 4, top + 12, look.skin);
      }

      // Fired, they carry their things out in a box.
      if (person.isFired && person.isLeaving) {
        fill(x - 4, top + 8, 9, 5, 0xc49a6c);
        fill(x - 4, top + 8, 9, 1, 0xa07a50);
        fill(x, top + 8, 1, 5, 0xe0c890);
        dot(x - 2, top + 7, PALETTE.leaf);
        dot(x + 2, top + 7, PALETTE.mug);
      }

      // With music playing, a head at a desk or in the lounge bobs on the beat, each a little off from the next.
      const beat = frame.music?.playing === true && !person.isWalking && (pose.kind === 'sit' || pose.kind === 'stand')
        && (person.isAtDesk || spotOf(person.place) !== null);
      const bob = beat ? Math.floor(now * 4 + look.phase * 4) % 2 : 0;
      const head = top + bob;

      // Head: a face, or the back of it, and the hair. A shaved head shows
      // skin where the hair would be, a buzz cut a thin layer of it.
      const shaved = look.hairStyle === 'bald' || look.hairStyle === 'mohawk';
      const cropped = shaved || look.hairStyle === 'buzz';
      fill(x - 3, head + 2, 7, 5, away && !shaved ? look.hair : look.skin);
      if (away) fill(x - 2, head + 6, 5, 1, look.skin);
      if (cropped) {
        fill(x - 2, head + 1, 5, 1, shaved ? look.skin : look.hair);
      } else {
        fill(x - 2, head, 5, 1, look.hair);
        fill(x - 3, head + 1, 7, 1, look.hair);
        dot(x - 3, head + 2, look.hair);
        dot(x + 3, head + 2, look.hair);
      }
      if (look.hairStyle === 'long') {
        fill(x - 3, head + 2, 1, 7, look.hair);
        fill(x + 3, head + 2, 1, 7, look.hair);
      } else if (look.hairStyle === 'spiky') {
        for (const dx of [-2, 0, 2]) dot(x + dx, head - 1, look.hair);
      } else if (look.hairStyle === 'bun') {
        fill(x - 1, head - 2, 3, 2, look.hair);
      } else if (look.hairStyle === 'curly') {
        for (const dx of [-3, -1, 1, 3]) dot(x + dx, head - 1, look.hair);
        dot(x - 4, head + 2, look.hair);
        dot(x + 4, head + 2, look.hair);
      } else if (look.hairStyle === 'ponytail') {
        fill(x + 4, head + 1, 1, 5, look.hair);
      } else if (look.hairStyle === 'afro') {
        fill(x - 4, head - 2, 9, 3, look.hair);
        fill(x - 4, head + 1, 1, 4, look.hair);
        fill(x + 4, head + 1, 1, 4, look.hair);
      } else if (look.hairStyle === 'mohawk') {
        fill(x, head - 2, 1, 3, look.hair);
      }
      // A hat that comes with the outfit takes the place of one of your own.
      if (!(parts && parts.head && ['cap', 'beanie', 'headphones', 'bow'].includes(look.extra))) drawHeadwear(x, head, look.extra, outfit.shirt, away);

      if (away) {
        if (parts && parts.head) parts.head(x, head, look, true, outfit);
        drawRole(person, x, top, head, pose, true, look);
        return;
      }
      drawFace(x, head, look);
      const blinking = (now + look.phase * 5) % 4.5 < 0.15;
      // While the mic is held its listener looks toward it, and when it goes
      // on, everyone looks up for a moment.
      if (isListener(person) && mic.held && pose.kind !== 'slump') {
        dot(x - 1, head + 4, PALETTE.eye);
        dot(x + 3, head + 4, PALETTE.eye);
      } else if ((lookingUp || pose.kind === 'tap') && pose.kind !== 'slump') {
        dot(x - 2, head + 3, PALETTE.eye);
        dot(x + 2, head + 3, PALETTE.eye);
      } else if (pose.kind === 'slump') {
        // Eyes shut, head in hands
        dot(x - 2, head + 4, PALETTE.eye, 0.45);
        dot(x + 2, head + 4, PALETTE.eye, 0.45);
        fill(x - 4, head + 3, 1, 3, look.skin);
        fill(x + 4, head + 3, 1, 3, look.skin);
      } else if (pose.kind === 'wave' || pose.kind === 'press') {
        dot(x - 2, head + 4, PALETTE.eye);
        dot(x + 2, head + 4, PALETTE.eye);
        dot(x, head + 6, PALETTE.eye);
      } else if (!blinking) {
        dot(x - 2, head + 4, PALETTE.eye);
        dot(x + 2, head + 4, PALETTE.eye);
      }
      // Sunglasses stay on, whatever the eyes behind them do.
      if (look.extra === 'sunglasses') fill(x - 3, head + 4, 7, 1, SHADES);
      // What the outfit puts on the head and face, over the eyes: a helmet, a hat, an eyepatch, dark glasses
      if (parts && parts.head) parts.head(x, head, look, false, outfit);
      drawRole(person, x, top, head, pose, false, look);
    }

    /**
     * A role's badge, worn over whatever the dress code puts them in: the
     * Reviewer's red glasses and a pen in the pocket, the Tester's clipboard,
     * the Bug hunter's magnifying glass at one eye, the Docs writer's quill
     * behind an ear and notebook, the Refactorer's wrench on a shoulder, or
     * your own role's on a card on a lanyard. What's held is put down while the
     * hands are busy. Just hired, they come in with it as their own: the
     * Reviewer polishing the glasses, the Tester's clipboard under an arm,
     * the Bug hunter searching the floor; sat down, they get to it a moment.
     */
    function drawRole(person, x, top, head, pose, away, look) {
      const badge = person.isStaff ? null : badgeOf(person.session);
      if (!badge) return;
      const I = BADGE_INKS, free = ['walk', 'stand', 'sit', 'tap'].includes(pose.kind);
      const arriving = person.isNew && person.isWalking;
      const since = person.settledAt === null ? Infinity : now - person.settledAt;
      const moment = !arriving && since < HIRE_MOMENT && free ? since / HIRE_MOMENT : null;
      const tick = Math.floor(now * 6) % 2;
      const twinkle = (tx, ty, strength = 1) => {
        dot(tx, ty, WHITE, strength);
        if (tick) for (const [dx, dy] of [[-1, 0], [1, 0], [0, -1], [0, 1]]) dot(tx + dx, ty + dy, WHITE, 0.5 * strength);
      };
      if (badge === 'glasses') {
        const frames = I.R, worn = !arriving && !(moment !== null && moment < 0.4);
        if (!away && worn) {
          fill(x - 3, head + 3, 3, 1, frames);
          fill(x + 1, head + 3, 3, 1, frames);
          for (const dx of [-3, -1, 0, 1, 3]) dot(x + dx, head + 4, frames);
          fill(x - 3, head + 5, 3, 1, frames);
          fill(x + 1, head + 5, 3, 1, frames);
          if (moment !== null && moment < 0.7) twinkle(x + 3, head + 3, 1 - moment);
        } else if (!away && free) {
          // Off, held up and polished on the shirt
          const rub = tick;
          for (const dx of [0, 2]) dot(x + dx, top + 9, frames);
          dot(x + 1, top + 9, GLASS);
          dot(x + 3, top + 9, GLASS);
          fill(x + 1 + rub, top + 10, 2, 1, look.skin);
        }
        if (!away) {
          dot(x + 2, top + 7, WHITE);
          dot(x + 2, top + 8, I.r);
        }
      } else if (badge === 'clipboard') {
        if (away || !free) return;
        if (arriving) {
          // Under an arm
          fill(x - 6, top + 8, 2, 5, I.n);
          fill(x - 6, top + 9, 1, 3, I.w);
          return;
        }
        // Held up in front, high enough to show over a desk
        fill(x - 3, top + 7, 4, 5, I.n);
        fill(x - 2, top + 6, 2, 1, I.G);
        fill(x - 2, top + 8, 2, 3, I.w);
        dot(x - 2, top + 9, I.G);
        dot(x - 4, top + 9, look.skin);
        // Sat down, they tick something off.
        if (moment !== null) {
          if (moment > 0.35) dot(x - 1, top + 8, I.e);
          if (moment > 0.5) dot(x - 2, top + 9, I.e);
          if (moment < 0.6) dot(x + 1, top + 8 + tick, look.skin);
        }
      } else if (badge === 'magnifier') {
        if (away || !free) return;
        if (arriving) {
          // Searching the floor ahead of them, side to side
          const sweep = Math.round(Math.sin(now * 5));
          const lx = x + 5 + sweep, ly = top + 13;
          fill(lx - 1, ly - 1, 3, 1, I.g);
          dot(lx - 1, ly, I.g);
          dot(lx, ly, GLASS);
          dot(lx + 1, ly, I.g);
          fill(lx - 1, ly + 1, 3, 1, I.g);
          dot(x + 4, top + 11, I.N);
          dot(x + 4, top + 12, look.skin);
          return;
        }
        fill(x + 1, head + 3, 3, 1, I.g);
        dot(x + 1, head + 4, I.g);
        dot(x + 3, head + 4, I.g);
        fill(x + 1, head + 5, 3, 1, I.g);
        dot(x + 4, head + 6, I.N);
        dot(x + 4, head + 7, I.N);
        dot(x + 4, head + 8, look.skin);
        // Sat down, they find a bug on the desk, and it runs for it.
        if (moment !== null) {
          const run = Math.floor(moment * 10);
          dot(x - 2 - run, top + 12, PALETTE.eye);
          if (tick) dot(x - 1 - run, top + 12, PALETTE.eye, 0.5);
        }
      } else if (badge === 'quill') {
        // Behind an ear
        dot(x + 4, head + 3, I.w);
        dot(x + 4, head + 2, I.g);
        dot(x + 5, head + 1, I.w);
        dot(x + 5, head, I.w);
        dot(x + 6, head - 1, I.g);
        if (away || !free) return;
        if (arriving) {
          fill(x - 6, top + 8, 2, 5, I.b);
          fill(x - 6, top + 9, 1, 3, I.w);
          return;
        }
        if (moment !== null) {
          // Open, and written in
          fill(x - 3, top + 9, 6, 3, I.w);
          fill(x - 3, top + 12, 6, 1, I.B);
          dot(x, top + 9, I.g);
          const line = Math.floor(moment * 8);
          for (let i = 0; i < Math.min(line, 4); i++) dot(x - 2 + (i % 2) + (i > 1 ? 3 : 0), top + 10 + (i % 2), I.G);
          dot(x + 1 + tick, top + 8, look.skin);
          return;
        }
        fill(x - 3, top + 8, 3, 4, I.b);
        fill(x - 3, top + 8, 1, 4, I.B);
        fill(x - 1, top + 9, 1, 3, I.w);
      } else if (badge === 'wrench') {
        if (!free) return;
        if (arriving || (moment !== null && moment < 0.6)) {
          // Swinging it in a hand, or giving it a twirl
          const up = arriving ? tick : Math.floor(now * 10) % 2;
          dot(x + 4, top + 12, look.skin);
          if (up) {
            fill(x + 5, top + 9, 1, 3, I.g);
            dot(x + 4, top + 8, I.g);
            dot(x + 6, top + 8, I.g);
          } else {
            fill(x + 5, top + 12, 3, 1, I.g);
            dot(x + 8, top + 11, I.g);
            dot(x + 8, top + 13, I.g);
          }
          return;
        }
        // Over a shoulder, its open end up
        dot(x + 4, top + 8, look.skin);
        dot(x + 4, top + 7, I.G);
        dot(x + 5, top + 6, I.g);
        dot(x + 5, top + 5, I.g);
        fill(x + 4, top + 4, 3, 1, I.g);
        dot(x + 4, top + 3, I.g);
        dot(x + 6, top + 3, I.g);
        if (moment !== null) twinkle(x + 5, top + 1, 1 - moment);
      } else {
        if (away) return;
        // On a white card on a lanyard, high enough to show over a desk
        const { pin } = BADGES[badge];
        dot(x - 2, top + 7, I.k);
        dot(x + 2, top + 7, I.k);
        fill(x - 2, top + 8, 5, 4, I.w);
        pin.forEach((row, r) => [...row].forEach((ch, c) => { if (ch !== '.') dot(x - 1 + c, top + 8 + r, I[ch]); }));
        if (moment !== null) twinkle(x + 3, top + 7, 1 - moment);
      }
    }

    /** What's worn on the head, seen from in front or behind: headphones, a cap, a beanie or a bow. */
    function drawHeadwear(x, top, extra, colour, away) {
      if (extra === 'headphones') {
        fill(x - 3, top - 1, 7, 1, HEADPHONES);
        fill(x - 4, top, 1, 2, HEADPHONES);
        fill(x + 4, top, 1, 2, HEADPHONES);
        fill(x - 4, top + 2, 1, 3, HEADPHONES);
        fill(x + 4, top + 2, 1, 3, HEADPHONES);
      } else if (extra === 'cap') {
        fill(x - 3, top - 1, 7, 2, colour);
        if (!away) fill(x - 4, top + 1, 9, 1, darker(colour));
      } else if (extra === 'beanie') {
        fill(x - 3, top - 1, 7, 2, colour);
        fill(x - 3, top + 1, 7, 1, darker(colour));
        dot(x, top - 2, PALETTE.paper);
      } else if (extra === 'bow') {
        dot(x + 1, top - 1, BOW);
        dot(x + 3, top - 1, BOW);
        dot(x + 2, top - 1, darker(BOW));
      }
    }

    /** What's on the face, under the eyes drawn over it: glasses, a beard or a moustache. */
    function drawFace(x, top, look) {
      if (look.extra === 'glasses') {
        fill(x - 3, top + 3, 3, 1, PALETTE.eye);
        fill(x + 1, top + 3, 3, 1, PALETTE.eye);
        for (const dx of [-3, -1, 1, 3]) dot(x + dx, top + 4, GLASS);
      } else if (look.extra === 'beard') {
        fill(x - 3, top + 4, 1, 3, look.hair);
        fill(x + 3, top + 4, 1, 3, look.hair);
        fill(x - 2, top + 6, 5, 1, look.hair);
      } else if (look.extra === 'moustache') {
        fill(x - 1, top + 5, 3, 1, look.hair);
      }
    }

    /* Above the floor */

    /**
     * A white flash, the room going red, a shockwave across the floor, and
     * the warning light turning until the alarm stops.
     */
    function drawDetonation() {
      if (alarm === null) return;
      const since = alarm;
      const fade = Math.min(1, (ALARM_LENGTH - since) / 0.5);
      const all = (color, alpha) => fill(seen.left, seen.top, seen.width, seen.height, color, alpha);
      if (since < 0.08) all(WHITE, 0.5);
      if (since < FLASH_LENGTH) all(PALETTE.alarm, 0.35 * (1 - since / FLASH_LENGTH));
      all(PALETTE.alarm, 0.07 * (0.5 + 0.5 * Math.sin(since * 12)) * fade);

      // Two rings, flattened onto the floor
      const button = placed('button', BUTTON);
      for (const delay of [0, 0.15]) {
        const age = (since - delay) / SHOCKWAVE_LENGTH;
        if (age < 0 || age >= 1) continue;
        const radius = 3 + age * 150;
        const count = Math.floor(radius * 4);
        for (let k = 0; k < count; k++) {
          const angle = (k / count) * 2 * Math.PI;
          dot(Math.round(button.x + Math.cos(angle) * radius), Math.round(button.y + Math.sin(angle) * radius * 0.55),
            PALETTE.shockwave, 1 - age);
        }
      }

      // The warning light's beam, swinging from side to side
      const side = Math.floor(since * 6) % 2 === 0 ? -1 : 1;
      for (const [reach, spread] of [[10, 2], [20, 4], [32, 6]]) {
        fill(side < 0 ? 196 - reach : 201, 25 - spread, reach, spread * 2, PALETTE.alarm, 0.12 * fade);
      }
    }

    /** Each screen lights its edges in its session's colour, the same as its key on the deck. */
    function drawScreenLight(desk) {
      const session = sessionAt(desk);
      if (!session || DESKS[desk].off) return;
      // The screen's top-left, wherever the desk has it, and its size: a laptop's, or the office's monitor's, or two of them
      const workspace = WORKSPACES[workspaceOf(session, desk)];
      const at = { x: DESKS[desk].x + workspace.screen.dx, y: DESKS[desk].y + workspace.screen.dy };
      const monitor = !workspace.laptop && MONITORS[monitorAt(desk)];
      const screens = [].concat(monitor && monitor.screen ? monitor.screen(at.x, at.y) : { ...at, w: 13, h: 9 });
      const { x, y } = screens[0];
      if (failureOf(session) === 'billing') return; // out of order, the screen is dark
      const blinkedOff = session.state === 'asking' && Math.floor(now * 2) % 2 === 1;
      // A command running turns the screen terminal-green.
      const shell = session.state === 'thinking' && toolKind(session.tool) === 'shell';
      const light = shell ? PALETTE.terminal
        : session.state === 'idle' || blinkedOff ? PALETTE.monitorOn : colour(session.state);

      for (const { x: left, y: top, w, h, round } of screens) {
        if (round) {
          // A ring round a round one
          const cx = left + (w - 1) / 2, cy = top + (h - 1) / 2;
          if (session.state !== 'idle') disc(cx, cy, w / 2 + 2, light, 0.12);
          ring(cx, cy, w / 2 + 0.5, light);
          continue;
        }
        if (session.state !== 'idle') fill(left - 3, top - 3, w + 6, h + 5, light, 0.12);
        fill(left - 1, top - 1, w + 2, 1, light);
        fill(left - 1, top, 1, h, light);
        fill(left + w, top, 1, h, light);
      }

      if (shell) {
        // Lines of output, scrolling up
        for (let k = 0; k < 4; k++) {
          const rise = (now * 1.4 + k / 4) % 1;
          fill(x + 1 + (k % 2) * 2, y - 2 - Math.round(rise * 9), 5 + (k % 3) * 2, 1, light, 1 - rise);
        }
      } else if (session.state === 'thinking') {
        for (let k = 0; k < 3; k++) {
          const rise = (now * 0.8 + k / 3 + desk * 0.17) % 1;
          dot(x + 2 + k * 4, y - 2 - Math.round(rise * 9), light, 1 - rise);
        }
      } else if (session.state === 'error') {
        for (let k = 0; k < 3; k++) {
          const rise = (now * 0.5 + k / 3) % 1;
          const size = rise > 0.5 ? 3 : 2;
          const drift = Math.round(Math.sin(rise * 6 + k) * 1.5);
          fill(x + 3 + k * 2 + drift, y - 3 - Math.round(rise * 12), size, size, PALETTE.smoke, 0.8 * (1 - rise));
        }
      }
    }

    function drawBubble(person) {
      // Going nuclear says enough on its own, and the choice keys stand in
      // for the bubble of the agent they answer.
      if (person.isLeaving || person.isGoingNuclear) return;
      if (frame.choices && !person.isStaff && person.session.id === selected) return;
      const state = person.session.state;
      const pose = poseOf(person, now);
      const top = topOf(person, pose);
      let x = Math.round(person.position.x);

      if (mic.held && isListener(person)) {
        // Listening: sound lines coming in from the right.
        const bottom = top - 3;
        fill(x - 4, bottom - 7, 9, 9, PALETTE.bubbleEdge);
        fill(x - 5, bottom - 6, 11, 7, PALETTE.bubbleEdge);
        fill(x - 4, bottom - 6, 9, 7, PALETTE.bubble);
        dot(x - 1, bottom + 1, PALETTE.bubble);
        dot(x - 1, bottom + 2, PALETTE.bubbleEdge);
        const lit = Math.floor(now * 4) % 3;
        for (let k = 0; k <= lit; k++) {
          const lx = x - 2 + k * 2;
          fill(lx, bottom - 4 - k, 1, 3 + k * 2, PALETTE.bubbleEdge);
        }
        return;
      }

      if (state === 'idle') {
        // A nap, after a while on the couch or the beanbag, not at the drums or in the hot tub
        const spot = spotOf(person.place);
        if (person.isWalking || spot === null || spots[spot].pose !== 'sit' || now - person.arrivedAt <= 20) return;
        if (spots[spot].kind && spots[spot].kind !== 'books') return;
        const rise = (now * 0.35 + person.look.phase) % 1;
        const zx = x + 3 + Math.round(rise * 3), zy = top - 3 - Math.round(rise * 8);
        fill(zx, zy, 3, 1, WHITE, 1 - rise);
        dot(zx + 1, zy + 1, WHITE, 1 - rise);
        fill(zx, zy + 2, 3, 1, WHITE, 1 - rise);
        return;
      }

      // Watching a storm or out of order, the scene says it: no bubble over it.
      if (['overloaded', 'billing'].includes(failureOf(person.session))) return;

      // Kept waiting, the bubble shakes and turns red, and steam rises.
      const agitation = state === 'asking' ? agitationOf(person, now) : 0;
      const bob = state === 'asking' && Math.floor(now * (3 + 6 * agitation)) % 2 === 0 ? -1 : 0;
      const bottom = top - 3 + bob;
      if (agitation) {
        for (const side of [-1, 1]) {
          const rise = (now * 1.4 + (side > 0 ? 0.5 : 0)) % 1;
          fill(x + side * 5, top + 2 - Math.round(rise * 6), 2, 1, PALETTE.smoke, agitation * (1 - rise));
        }
        x += Math.floor(now * 14) % 2 ? 1 : -1;
      }
      fill(x - 4, bottom - 7, 9, 9, PALETTE.bubbleEdge);
      fill(x - 5, bottom - 6, 11, 7, PALETTE.bubbleEdge);
      fill(x - 4, bottom - 6, 9, 7, PALETTE.bubble);
      dot(x - 1, bottom + 1, PALETTE.bubble);
      dot(x - 1, bottom + 2, PALETTE.bubbleEdge);

      // A 5 × 5 mark in the middle, in the state's colour
      const marks = {
        asking: [[0, 0], [0, 1], [0, 2], [0, 4]],
        unread: [[-2, 2], [-1, 3], [0, 2], [1, 1], [2, 0]],
        error: [[-2, 0], [2, 0], [-1, 1], [1, 1], [0, 2], [-1, 3], [1, 3], [-2, 4], [2, 4]],
      };
      const cells = state === 'thinking'
        ? Array.from({ length: Math.floor(now * 3 + person.look.phase * 3) % 4 }, (_, i) => [-2 + i * 2, 2])
        : marks[state] || [];
      if (failureOf(person.session) === 'auth') {
        // A key, and a red mark: knocking, an exclamation; patting their pockets, a cross
        const kx = x - 3, ky = bottom - 4;
        fill(kx, ky + 1, 2, 2, PALETTE.brass);
        dot(kx, ky, PALETTE.brass);
        fill(kx + 2, ky + 1, 3, 1, PALETTE.brass);
        dot(kx + 3, ky + 2, PALETTE.brass);
        if (takeOf(person) === 0) for (const dy of [-1, 0, 2]) dot(x + 3, ky + dy, PALETTE.alarm);
        else for (let k = 0; k < 4; k++) { dot(x - 2 + k, bottom - 6 + k, PALETTE.alarm); dot(x + 1 - k, bottom - 6 + k, PALETTE.alarm); }
        return;
      }
      for (const [column, row] of cells) dot(x + column, bottom - 5 + row, agitation ? PALETTE.alarm : colour(state));
    }

    /** Confetti over whoever just finished a turn of an hour or more, falling and fading. */
    function drawConfetti(person) {
      const since = person.celebratedAt === null ? Infinity : now - person.celebratedAt;
      if (since >= PARTY_LENGTH || person.isLeaving) return;
      const x = person.position.x, top = topOf(person, poseOf(person, now));
      const colours = [0xf2d35b, 0xd96a5a, 0x4c8bf5, 0x4f9a5a, 0xf28fb0, 0x86d6bc];
      for (let i = 0; i < 28; i++) {
        // The same scatter every time, from the person's own look
        const r = (k) => ((Math.sin((i + 1) * 12.9898 + k * 78.233 + person.look.phase * 43.1) * 43758.5453) % 1 + 1) % 1;
        const vx = (r(1) - 0.5) * 50, vy = -35 - r(2) * 25, t = since + r(3) * 0.15;
        const px = x + vx * t, py = top - 2 + vy * t + 45 * t * t;
        const flutter = Math.floor((now * 8 + i) % 2);
        fill(px, py, 1 + flutter, 2 - flutter, colours[i % colours.length], Math.min(1, (PARTY_LENGTH - since) / 0.8));
      }
    }

    /**
     * The project's name under each desk, with its state's colour beside it,
     * and under it, what the agent is doing, tinted with that colour. A key
     * pinned to a project with nothing running keeps its name, dimmed.
     *
     * Where the text is bigger than the art, as on a phone, only the picked
     * desk says what its agent is doing: every desk's would cover the row
     * below.
     */
    function drawLabel(desk) {
      if (desk >= office.seats.length) return;
      const s = office.seats[desk];
      // A nickname you've given their agent, or their project's name; a role's agent, its role first.
      const member = memberOf(s.session);
      const project = s.session ? (member && member.name) || s.session.name : s.pin;
      if (!project) return;
      const role = s.session && s.session.role && s.session.role.name ? String(s.session.role.name).slice(0, 16) : null;
      const name = role ? `${role} · ${project}` : project;

      const roomy = LABEL_SIZE * scale >= LABEL_SMALLEST * ratio;
      const size = roomy ? LABEL_SIZE * scale : LABEL_SMALLEST * ratio;
      const font = (weight, px) => `${weight} ${px}px ui-rounded, "SF Pro Rounded", -apple-system, system-ui, sans-serif`;
      ctx.font = font(600, size);
      ctx.textBaseline = 'middle';
      ctx.textAlign = 'left';
      const dotSize = s.session ? size * 0.55 : 0;
      const gap = s.session ? size * 0.4 : 0;
      const padding = size * 0.5;
      // Cut to the space between desks, so labels never run into each other: in the line, to the space
      // between its people, but for the picked one's, drawn over theirs.
      const inLine = DESKS[desk].off && crowdAnchors.has(desk);
      if (DESKS[desk].off && !inLine) return;
      const between = inLine ? (desk === selectedDesk ? DESK_WIDTH : CROWD_GAP - 1) : DESK_COLUMNS[1] - DESK_COLUMNS[0] - 2;
      const room = between * scale - dotSize - gap - padding * 2;
      const shown = fitText(ctx, name, room);
      const nameWidth = ctx.measureText(shown).width + dotSize + gap + padding * 2;

      const activity = s.session && s.session.activity && ((roomy && !inLine) || desk === selectedDesk) ? s.session.activity : null;
      const small = size * 0.82;
      let doing = '', doingWidth = 0;
      if (activity) {
        ctx.font = font(500, small);
        doing = fitText(ctx, activity, DESK_WIDTH * scale - padding * 2);
        doingWidth = ctx.measureText(doing).width + padding * 2;
      }

      const feet = inLine && crowdAnchors.get(desk).feet;
      const center = inLine ? screen(feet.x, feet.y + 4) : screen(DESKS[desk].x + DESK_WIDTH / 2, DESKS[desk].y + 20.5);
      const h = size * 1.25, extra = activity ? small * 1.4 : 0;
      const w = Math.max(nameWidth, doingWidth);
      const left = center.x - w / 2;
      ctx.fillStyle = 'rgba(0,0,0,0.45)';
      roundedRect(ctx, left, center.y - h / 2, w, h + extra, h / 2);
      ctx.fill();
      const nameLeft = center.x - nameWidth / 2;
      if (s.session) {
        ctx.fillStyle = css(colour(s.session.state));
        ctx.beginPath();
        ctx.arc(nameLeft + padding + dotSize / 2, center.y, dotSize / 2, 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.font = font(600, size);
      ctx.fillStyle = `rgba(255,255,255,${s.session ? 0.92 : 0.45})`;
      const nameAt = nameLeft + padding + dotSize + gap;
      // The role in its badge's colour, where it's whole
      const badge = badgeOf(s.session);
      if (role && badge && shown.startsWith(`${role} ·`)) {
        const rest = shown.slice(role.length);
        ctx.fillText(rest, nameAt + ctx.measureText(role).width, center.y);
        ctx.fillStyle = css(BADGES[badge].colour);
        ctx.fillText(role, nameAt, center.y);
      } else {
        ctx.fillText(shown, nameAt, center.y);
      }
      if (activity) {
        ctx.font = font(500, small);
        ctx.fillStyle = css(colour(s.session.state));
        ctx.fillText(doing, center.x - doingWidth / 2 + padding, center.y + h / 2 + extra / 2 - small * 0.2);
      }
    }

    /** The shredder a compacting session's agent feeds, beside its desk. */
    function drawShredder(desk) {
      const { x, y } = DESKS[desk], sx = x + SHREDDER.dx;
      // The cutting head, its slot on top
      fill(sx, y + 8, 6, 1, PALETTE.shredderTop);
      fill(sx + 1, y + 8, 4, 1, PALETTE.frame);
      fill(sx, y + 9, 6, 2, PALETTE.shredder);
      // The bin under it, filling with strips, one falling
      fill(sx, y + 11, 6, 5, PALETTE.sill);
      for (const [dx, height] of [[1, 2], [2, 3], [4, 2]]) fill(sx + dx, y + 16 - height, 1, height, PALETTE.paper);
      dot(sx + 3, y + 11 + (Math.floor(now * 6) % 3), PALETTE.paper);
      fill(sx, y + 16, 6, 1, SHADOW);
    }

    /**
     * The fronts, backs and hats of the styles' outfits and the models'
     * looks, each over a shirt 7 wide and 6 tall from `x`, `y`, in `o`, the
     * outfit, its `accent` the agent's colour, its `shade` the model's size.
     */
    const OUTFIT_PARTS = {
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
    };

    /**
     * The front of an outfit, over a shirt `half` either side of `x` and
     * `rows` tall from `y`: a T-shirt's stripe, a shirt's collar and buttons,
     * the shirt under a jacket, or a suit's tie.
     */
    function drawOutfitFront(x, y, half, rows, outfit, look) {
      const white = PALETTE.paper, parts = OUTFIT_PARTS[outfit.style];
      if (parts) {
        // An intern's is too small for the whole of it: a touch of its colour.
        if (half < 3) return dot(x, y + 1, outfit.accent !== outfit.shirt && outfit.accent ? outfit.accent : lighter(outfit.shirt));
        if (parts.front) parts.front(x, y, outfit, look);
        return;
      }
      if (outfit.style === 'tee') {
        fill(x - half, y + 2, half * 2 + 1, 1, white);
      } else if (outfit.style === 'hoodie') {
        // The hood round the neck, its strings, and the pocket
        dot(x - 2, y, darker(outfit.shirt));
        dot(x + 2, y, darker(outfit.shirt));
        dot(x - 1, y + 2, white);
        dot(x + 1, y + 2, white);
        fill(x - half + 1, y + rows - 2, half * 2 - 1, 1, darker(outfit.shirt));
      } else if (outfit.style === 'sweater') {
        // A knitted band across the chest, and the ribbing at the hem
        fill(x - half, y + 2, half * 2 + 1, 1, lighter(outfit.shirt));
        fill(x - half, y + rows - 1, half * 2 + 1, 1, darker(outfit.shirt));
      } else if (outfit.style === 'labcoat') {
        // Open over a blue shirt, a pen in the pocket
        fill(x, y, 1, rows - 1, LAB_SHIRT);
        dot(x - 2, y + 2, LAB_PEN);
      } else if (outfit.style === 'collar') {
        dot(x - 2, y, white);
        dot(x + 2, y, white);
        for (let row = 2; row < rows; row += 2) dot(x, y + row, darker(outfit.shirt));
      } else if (outfit.style === 'jacket') {
        fill(x, y, 1, rows - 2, white);
      } else if (outfit.style === 'suit') {
        dot(x - 1, y, white);
        dot(x + 1, y, white);
        fill(x, y, 1, rows - 2, outfit.tie);
      } else if (outfit.style === 'hire') {
        // The collar's points either side of the tie, the tie down to the last row but one
        fill(x - 2, y, 2, 1, white);
        fill(x + 1, y, 2, 1, white);
        dot(x - 1, y + 1, white);
        dot(x + 1, y + 1, white);
        fill(x, y, 1, rows - 1, outfit.tie);
      } else if (outfit.style === 'polo') {
        // A soft collar, and a short placket with a button
        dot(x - 1, y, lighter(outfit.shirt));
        dot(x + 1, y, lighter(outfit.shirt));
        dot(x, y + 1, darker(outfit.shirt));
        dot(x, y + 2, white);
      } else if (outfit.style === 'cardigan') {
        // Open over a white shirt, buttoned down one side
        fill(x, y, 1, rows - 1, white);
        for (let row = 2; row < rows; row += 2) dot(x - 1, y + row, darker(outfit.shirt));
      } else if (outfit.style === 'hawaiian') {
        // An open collar, and flowers
        dot(x - 1, y, white);
        dot(x + 1, y, white);
        for (const [dx, dy, flower] of [[-half, 1, white], [half - 1, 2, HIBISCUS], [1 - half, 3, HIBISCUS], [half, 4, white]]) {
          if (dy < rows) dot(x + dx, y + dy, flower);
        }
      } else if (outfit.style === 'tracksuit') {
        // A zip down the middle, and a stripe on each shoulder
        fill(x, y, 1, rows - 1, lighter(outfit.shirt));
        dot(x - half, y, white);
        dot(x + half, y, white);
      } else if (outfit.style === 'overalls') {
        // The bib over the shirt, its straps and their buttons
        fill(x - half + 1, y + 2, half * 2 - 1, rows - 2, DENIM);
        for (const side of [-1, 1]) {
          fill(x + side * (half - 1), y, 1, 2, DENIM);
          dot(x + side * (half - 1), y + 2, HIBISCUS);
        }
      } else if (outfit.style === 'waistcoat') {
        // The white shirt showing at the neck, and buttons down the front
        fill(x - 1, y, 3, 1, outfit.sleeves);
        dot(x, y + 1, outfit.sleeves);
        for (let row = 2; row < rows; row += 2) dot(x, y + row, darker(outfit.shirt));
      }
    }

    /**
     * A subagent, as an intern: 13 art pixels tall, dressed as their agent's
     * agents are or for their own model, with a badge.
     */
    function drawIntern(intern) {
      const x = Math.round(intern.position.x), feet = Math.round(intern.position.y), top = feet - 13;
      // Dressed for their agent's desk
      const desk = office.deskOfSession(intern.agentId);
      const { look } = intern, outfit = outfitOf(intern.agent, intern.model, null, desk === null ? null : workspaceOf(sessionAt(desk), desk));
      const shirt = outfit.shirt;
      const step = intern.isWalking ? Math.floor(now * 7) % 2 : null;
      const lift = step === null ? [0, 0] : step === 0 ? [1, 0] : [0, 1];
      fill(x - 2, feet, 5, 1, SHADOW);
      fill(x - 2, top + 9, 5, 1, outfit.pants);
      fill(x - 1, top + 10 - lift[0], 1, 2, outfit.pants);
      fill(x + 1, top + 10 - lift[1], 1, 2, outfit.pants);
      dot(x - 1, top + 12 - lift[0], outfit.shoes);
      dot(x + 1, top + 12 - lift[1], outfit.shoes);
      fill(x - 2, top + 5, 5, 4, shirt);
      drawOutfitFront(x, top + 5, 2, 4, outfit);
      // The badge goes beside buttons, a jacket's opening or a tie.
      if (outfit.style === 'plain' || outfit.style === 'tee') dot(x, top + 6, PALETTE.bubble);
      else dot(x + 1, top + 7, PALETTE.bubble);
      ctx.fillStyle = css(outfit.sleeves ?? shirt, 1, 0.78);
      for (const side of [-3, 3]) ctx.fillRect(ox + (x + side) * scale, oy + (top + 6) * scale, scale, 2 * scale);
      dot(x - 3, top + 8, look.skin);
      dot(x + 3, top + 8, look.skin);
      fill(x - 2, top + 1, 5, 4, look.skin);
      fill(x - 1, top, 3, 1, look.hair);
      fill(x - 2, top + 1, 5, 1, look.hair);
      if ((now + look.phase * 5) % 4 >= 0.15) {
        dot(x - 1, top + 3, PALETTE.eye);
        dot(x + 1, top + 3, PALETTE.eye);
      }
    }

    /**
     * How long the turn has run, on the back of the monitor while the agent
     * works, and sweat once it's been a long one. Beside the desk, how many
     * interns are waiting for room round it.
     */
    function drawTimer(desk) {
      if (DESKS[desk].off) return;
      const { x, y } = DESKS[desk];
      const waiting = office.waitingAt(desk);
      if (waiting) pixelText(`+${waiting}`, x - 9, y - 16, PALETTE.bubble);

      const session = sessionAt(desk);
      if (!session || !session.turnStartedAt || (session.state !== 'thinking' && session.state !== 'asking')) return;
      const seconds = Math.max(0, Math.floor((Date.now() - session.turnStartedAt) / 1000));
      const text = turnLength(seconds);
      // The screen is eleven pixels across, inside its edge: the colon goes
      // without the gaps round it, so m:ss fits inside it.
      const gap = (c, next) => (c === ':' || next === ':' ? 0 : 1);
      const chars = [...text];
      const width = chars.reduce((w, c, i) => w + GLYPHS[c][0].length + (i < chars.length - 1 ? gap(c, chars[i + 1]) : 0), 0);
      const { screen } = WORKSPACES[workspaceOf(session, desk)];
      let at = x + screen.dx + 1 + Math.floor((11 - width) / 2);
      chars.forEach((c, i) => {
        GLYPHS[c].forEach((row, r) => [...row].forEach((on, column) => { if (on === '1') dot(at + column, y + screen.dy + 2 + r, PALETTE.bubble); }));
        at += GLYPHS[c][0].length + gap(c, chars[i + 1]);
      });

      const occupant = occupantOf(desk);
      if (seconds < LONG_TURN || !occupant) return;
      const s = seat(desk), top = topOf(occupant, poseOf(occupant, now));
      const run = (now * 1.5 + occupant.look.phase) % 1;
      dot(s.x + 4, top + 2 + Math.round(run * 4), PALETTE.sweat, 1 - run * 0.6);
    }

    /**
     * The picked desk, as the deck marks its key: a pool of light on the
     * floor under it, and corners in the ink colour round it. Put away, the
     * pool is round its agent in the line.
     */
    function drawSpotlight() {
      if (selectedDesk < 0) return;
      const { x, y } = DESKS[selectedDesk], line = DESKS[selectedDesk].off && crowdAnchors.get(selectedDesk);
      if (DESKS[selectedDesk].off && !line) return;
      const left = line ? line.feet.x - 7 : x - 4, right = line ? line.feet.x + 8 : x + DESK_WIDTH + 4;
      const top = line ? line.feet.y - 19 : y + reachOf(selectedDesk), bottom = line ? line.feet.y + 3 : y + 20;
      for (const [grow, alpha] of [[0, 0.1], [3, 0.06]]) {
        fill(left - grow, top + grow / 2, right - left + grow * 2, bottom - top - grow, ink, alpha);
      }
    }

    function drawSelectionCorners() {
      if (selectedDesk < 0) return;
      const { x, y } = DESKS[selectedDesk], line = DESKS[selectedDesk].off && crowdAnchors.get(selectedDesk);
      if (DESKS[selectedDesk].off && !line) return;
      // In the line, round them, from their hair to their feet
      const left = line ? line.feet.x - 6 : x - 3, right = line ? line.feet.x + 6 : x + DESK_WIDTH + 2;
      const top = line ? line.feet.y - 18 : y + reachOf(selectedDesk), bottom = line ? line.feet.y + 2 : y + 18;
      for (const [cx, cy, dx, dy] of [[left, top, 1, 1], [right, top, -1, 1], [left, bottom, 1, -1], [right, bottom, -1, -1]]) {
        fill(dx > 0 ? cx : cx - 3, cy, 4, 1, ink);
        fill(cx, dy > 0 ? cy : cy - 3, 1, 4, ink);
      }
    }

    /** A desk marked as the picked desk is, blinking: where a carried agent would land, or where a hire would sit. */
    function drawBlinkingCorners(desk) {
      const d = DESKS[desk];
      const left = d.x - 3, right = d.x + DESK_WIDTH + 2, top = d.y + reachOf(desk), bottom = d.y + 18;
      const blink = 0.55 + 0.45 * Math.sin(now * 8);
      for (const [cx, cy, dx, dy] of [[left, top, 1, 1], [right, top, -1, 1], [left, bottom, 1, -1], [right, bottom, -1, -1]]) {
        fill(dx > 0 ? cx : cx - 3, cy, 4, 1, ink, blink);
        fill(cx, dy > 0 ? cy : cy - 3, 1, 4, ink, blink);
      }
    }

    /**
     * An agent carried to another desk: lifted to just above the finger,
     * legs dangling, their chair held over their head, and the desk they'd
     * land at marked as the picked desk is.
     */
    function drawCarried({ person, x, y, over, from }) {
      if (over !== null && over !== from) drawBlinkingCorners(over);
      if (!person) return;
      // Feet a little above the finger, so the finger doesn't hide them
      const lifted = Object.create(person, {
        position: { value: { x: Math.round(x), y: Math.round(y) - 6 } },
        path: { value: [null] }, // walking in the air
        place: { value: 'carried' },
      });
      drawPerson(lifted);
      const top = topOf(lifted, poseOf(lifted, now));
      // Their chair, or what they sit on at a workspace with none
      const workspace = WORKSPACES[workspaceOf(person.session, from)];
      const [chair, chairTop] = workspace.carry || chairAt(from);
      fill(lifted.position.x - 4, top - 5, 9, 2, chairTop);
      fill(lifted.position.x - 3, top - 3, 1, 2, chair);
      fill(lifted.position.x + 3, top - 3, 1, 2, chair);
    }

    /**
     * Sound waves from the mic while it's held, spreading out into the room
     * from its corner; or, with someone picked to listen, a beam of them
     * aimed at their head, carried as far as they are, wherever they are.
     */
    function drawMicWaves() {
      if (!mic.held) return;
      const heldFor = now - mic.heldAt;
      const at = placed('mic', MIC);
      const { dx, dy } = MIC_KINDS[decor.mic].mouth, mouth = { x: at.x + dx, y: at.y + dy };
      // As far as the listener's head, the waves being flatter than they're wide
      const listener = office.people.find(isListener);
      const head = listener && { x: listener.position.x, y: topOf(listener, poseOf(listener, now)) + 3 };
      const reach = head ? Math.max(WAVE_REACH, Math.hypot(head.x - mouth.x, (head.y - mouth.y) / 0.8)) : WAVE_REACH;
      const toward = head ? Math.atan2((head.y - mouth.y) / 0.8, head.x - mouth.x) : 0;
      for (let k = 0; k < 3; k++) {
        const age = (heldFor - k * WAVE_EVERY) % (WAVE_EVERY * 3) / (WAVE_EVERY * 3);
        if (heldFor < k * WAVE_EVERY) continue;
        const radius = 6 + age * reach;
        // A quarter turn, out into the room from the corner, or aimed at the listener
        const [from, turn] = head ? [toward - 0.4, 0.8] : [Math.PI, Math.PI / 2];
        const count = Math.floor(radius * turn);
        // Strong most of the way, fading as it gets there
        const alpha = 0.6 * Math.min(1, (1 - age) * 3);
        for (let i = 0; i <= count; i++) {
          const angle = from + (i / count) * turn;
          dot(Math.round(mouth.x + Math.cos(angle) * radius), Math.round(mouth.y + Math.sin(angle) * radius * 0.8), PALETTE.alarm, alpha);
        }
      }
    }

    /**
     * The choice keys over the picked agent: dark keys on a speech bubble's
     * paper, each sinking a pixel while it's pressed.
     */
    function drawChoices() {
      const { left, top, w, h, tip, keys } = frame.choices;
      fill(left + 1, top, w - 2, h, PALETTE.bubbleEdge);
      fill(left, top + 1, w, h - 2, PALETTE.bubbleEdge);
      fill(left + 1, top + 1, w - 2, h - 2, PALETTE.bubble);
      // The tail, down to the agent's head
      fill(tip.x - 1, top + h - 1, 3, 1, PALETTE.bubble);
      dot(tip.x - 1, top + h, PALETTE.bubbleEdge);
      dot(tip.x, top + h, PALETTE.bubble);
      dot(tip.x + 1, top + h, PALETTE.bubbleEdge);
      dot(tip.x, tip.y, PALETTE.bubbleEdge);

      const down = frame.choiceDown;
      const marks = { up: PALETTE.bubble, down: PALETTE.bubble, accept: colour('unread'), reject: colour('error') };
      for (const { action, x, y } of keys) {
        const { w: kw, h: kh } = CHOICE_KEY;
        const sunk = down && down.action === action && (down.held || now - down.at < CHOICE_PRESS_LENGTH) ? 1 : 0;
        fill(x + 1, y, kw - 2, kh, PALETTE.frame);
        fill(x, y + 1, kw, kh - 2, PALETTE.frame);
        fill(x + 1, y + 1, kw - 2, kh - 2, PALETTE.wainscot);
        fill(x + 1, y + 1 + sunk, kw - 2, kh - 4, PALETTE.rail);
        fill(x + 2, y + 1 + sunk, kw - 4, 1, PALETTE.monitorOn);
        const rows = CHOICE_MARKS[action];
        const mx = x + Math.floor((kw - rows[0].length) / 2), my = y + 1 + sunk + Math.floor((kh - 4 - rows.length) / 2);
        rows.forEach((row, r) => [...row].forEach((on, column) => { if (on === '1') dot(mx + column, my + r, marks[action]); }));
      }
    }

    /**
     * The paint chips on the picked desk: the effort it's set to stands
     * proud in a white outline, a tapped chip sinks, and max carries the
     * red button's trefoil.
     */
    function drawChips() {
      const { left, top, w, h, chips } = frame.chips;
      const session = sessionAt(selectedDesk);
      fill(left - 1, top - 1, w + 2, h + 3, PALETTE.frame);
      fill(left - 1, top + h + 2, w + 2, 1, SHADOW);
      const down = frame.chipDown;
      for (const { level, x, y } of chips) {
        const paint = EFFORT_PAINT[level];
        const set = session && session.effort === level;
        const sunk = down && down.level === level && (down.held || now - down.at < CHOICE_PRESS_LENGTH) ? 1 : 0;
        // A chip is a face over a darker edge: the set one stands a pixel
        // proud, and a tapped one sinks flat.
        const face = y + sunk - (set && !sunk ? 1 : 0);
        fill(x, face + h - 1, CHIP.w, y - face + 1, darker(paint));
        fill(x, face, CHIP.w, h - 1, paint);
        fill(x, face, CHIP.w, 1, WHITE, 0.35);
        if (set) {
          fill(x, face, CHIP.w, 1, WHITE);
          fill(x, face + h - 2, CHIP.w, 1, WHITE);
          fill(x, face, 1, h - 1, WHITE);
          fill(x + CHIP.w - 1, face, 1, h - 1, WHITE);
        }
        if (level === 'max') {
          // The trefoil, in three dots round a fourth
          const cx = x + 5, cy = face + 4;
          for (const [dx, dy] of [[0, -2], [-2, 1], [2, 1], [0, 0]]) dot(cx + dx, cy + dy, PALETTE.domeDark);
        }
      }
    }

    /** A pixel key: a face over a darker edge, with its label in the pixel font. */
    function drawPixelKey(x, y, w, h, face, label) {
      fill(x + 1, y, w - 2, h, PALETTE.frame);
      fill(x, y + 1, w, h - 2, PALETTE.frame);
      fill(x + 1, y + 1, w - 2, h - 2, darker(face));
      fill(x + 1, y + 1, w - 2, h - 4, face);
      fill(x + 2, y + 1, w - 4, 1, WHITE, 0.3);
      pixelText(label, x + Math.round((w - textWidth(label)) / 2), y + Math.floor((h - 4 - 5) / 2) + 1, WHITE, false);
    }

    /** YOU'RE FIRED in red, or HIRE in green, on a held desk. */
    function drawDeskButton({ kind, label, left, top, w, h }) {
      drawPixelKey(left, top, w, h, kind === 'fire' ? PALETTE.dome : 0x2f8f5b, label);
    }

    /** One of MUSIC_ICONS, its top-left at `x`, `y`. */
    function drawMusicIcon(name, x, y, colour) {
      MUSIC_ICONS[name].forEach((row, r) => [...row].forEach((on, column) => { if (on === '1') dot(x + column, y + r, colour); }));
    }

    /** What a tap on the music does, centred on `cx`: ❚❚ while it plays, ▶ while it doesn't, both when the computer can't tell. */
    function drawPlayPause(cx, y, playing, play, pause = play) {
      if (playing === null) {
        drawMusicIcon('play', cx - 3, y, play);
        drawMusicIcon('pause', cx + 1, y, pause);
      } else {
        drawMusicIcon(playing ? 'pause' : 'play', cx - 1, y, playing ? pause : play);
      }
    }

    /**
     * A music piece just tapped: a dark key over it with what the tap did,
     * green ▶ when nothing was playing, amber ❚❚ when something was, both
     * when the computer couldn't tell, going down as it's pressed.
     */
    function drawMusicTap({ box, was, since }) {
      const w = was === null ? 15 : 11, h = 11, sink = since < CHOICE_PRESS_LENGTH ? 1 : 0;
      const x = Math.round((box.left + box.right) / 2 - w / 2), y = Math.round(box.top) - h - 3 + sink;
      fill(x + 1, y + h - sink, w - 2, 1 + sink, PALETTE.frame);
      fill(x + 1, y, w - 2, h, PALETTE.frame);
      fill(x, y + 1, w, h - 2, PALETTE.frame);
      fill(x + 1, y + 1, w - 2, h - 3, 0x3a3d47);
      fill(x + 2, y + 1, w - 4, 1, WHITE, 0.25);
      fill(x + 1, y + h - 2, w - 2, 1, 0x2a2d35);
      drawPlayPause(x + Math.floor(w / 2), y + 3, was, PALETTE.terminal, PALETTE.hazard);
    }

    /**
     * A music piece's control: previous, play or pause, and next, dark keys
     * on a speech bubble's paper over the piece, each going down as it's
     * pressed and lit while it is.
     */
    function drawMusicPanel({ left, top, w, h, tip, keys, playing, down }) {
      fill(left + 1, top, w - 2, h, PALETTE.bubbleEdge);
      fill(left, top + 1, w, h - 2, PALETTE.bubbleEdge);
      fill(left + 1, top + 1, w - 2, h - 2, PALETTE.bubble);
      fill(tip.x - 1, top + h - 1, 3, 1, PALETTE.bubble);
      dot(tip.x - 1, top + h, PALETTE.bubbleEdge);
      dot(tip.x, top + h, PALETTE.bubble);
      dot(tip.x + 1, top + h, PALETTE.bubbleEdge);
      dot(tip.x, top + h + 1, PALETTE.bubbleEdge);
      for (const { action, x, y } of keys) {
        const sunk = down && down.action === action && (down.held || now - down.at < CHOICE_PRESS_LENGTH) ? 1 : 0, ky = y + sunk;
        fill(x + 1, ky, MUSIC_KEY.w - 2, MUSIC_KEY.h - sunk, PALETTE.frame);
        fill(x, ky + 1, MUSIC_KEY.w, MUSIC_KEY.h - 2 - sunk, PALETTE.frame);
        fill(x + 1, ky + 1, MUSIC_KEY.w - 2, MUSIC_KEY.h - 2 - sunk, sunk ? 0x2a2d35 : 0x3a3d47);
        if (!sunk) fill(x + 1, ky + MUSIC_KEY.h - 2, MUSIC_KEY.w - 2, 1, 0x2a2d35);
        fill(x + 2, ky + 1, MUSIC_KEY.w - 4, 1, WHITE, sunk ? 0.1 : 0.25);
        const ink = sunk ? PALETTE.terminal : PALETTE.bubble;
        if (action === 'playpause') drawPlayPause(x + 5, ky + 2, playing, ink);
        else drawMusicIcon(action, x + 2, ky + 2, ink);
      }
    }

    /**
     * The picked agent's keys, wrapped on a strip of a bubble's paper along
     * the foot of what can be seen, the same whoever is picked: the office's
     * own first, ⏎ and the arrows in pixels, the rest labelled in the name
     * labels' lettering. Where the foot is the room's own, it stops short of
     * the red button and the mic, so they can still be pressed. A key that
     * wants a second tap shows SURE? in red while it waits for it. Where each
     * key lands is kept on `row.at`, and the strip on `row.box`, for taps.
     */
    function drawKeyRow(row) {
      const { keys, armed, down } = row;
      const font = `600 ${Math.max(KEY_ROW.text * scale, 7 * ratio)}px ui-rounded, "SF Pro Rounded", -apple-system, system-ui, sans-serif`;
      ctx.font = font;
      const isArmed = (key) => armed && armed.action === key.action && now - armed.at < KEY_ARMED_LENGTH;
      const labelOf = (key) => (isArmed(key) ? 'sure?' : key.label);
      const widthOf = (key) => (key.icon && !isArmed(key) ? KEY_ROW.icon : Math.ceil(ctx.measureText(labelOf(key)).width / scale) + KEY_ROW.pad * 2);
      const widths = keys.map(widthOf);
      /** The keys wrapped onto rows `across` wide. */
      const wrap = (across) => {
        const rows = [[]];
        let used = 0;
        keys.forEach((key, i) => {
          if (used && used + KEY_ROW.gap + widths[i] > across - KEY_ROW.inset * 2) { rows.push([]); used = 0; }
          rows[rows.length - 1].push({ key, w: widths[i] });
          used += (used ? KEY_ROW.gap : 0) + widths[i];
        });
        return rows;
      };
      const heightOf = (rows) => rows.length * (KEY_ROW.h + KEY_ROW.gap) - KEY_ROW.gap + KEY_ROW.inset * 2;
      let span = { left: seen.left + 2, right: seen.right - 2 };
      let rows = wrap(span.right - span.left);
      for (let pass = 0; pass < 2; pass++) {
        span = footSpan(heightOf(rows), span);
        rows = wrap(span.right - span.left);
      }
      const { left, right } = span, w = right - left, h = heightOf(rows), top = Math.round(seen.bottom - 2 - h);
      drawFootStrip(left, top, w, h);
      row.box = { left, top, right: left + w, bottom: top + h };
      row.at = [];
      rows.forEach((r, i) => {
        let x = left + KEY_ROW.inset;
        const y = top + KEY_ROW.inset + i * (KEY_ROW.h + KEY_ROW.gap);
        for (const { key, w: kw } of r) {
          const sunk = down && down.action === key.action && (down.held || now - down.at < CHOICE_PRESS_LENGTH) ? 1 : 0, ky = y + sunk;
          const ready = isArmed(key);
          fill(x + 1, ky, kw - 2, KEY_ROW.h - sunk, PALETTE.frame);
          fill(x, ky + 1, kw, KEY_ROW.h - 2 - sunk, PALETTE.frame);
          fill(x + 1, ky + 1, kw - 2, KEY_ROW.h - 2 - sunk, ready ? darker(PALETTE.dome) : sunk ? 0x2a2d35 : 0x3a3d47);
          if (!sunk) fill(x + 1, ky + KEY_ROW.h - 2, kw - 2, 1, ready ? darker(darker(PALETTE.dome)) : 0x2a2d35);
          fill(x + 2, ky + 1, kw - 4, 1, WHITE, sunk ? 0.1 : 0.25);
          const ink = sunk ? PALETTE.terminal : PALETTE.bubble;
          if (key.icon && !ready) {
            const mark = key.icon === 'send' ? SEND_MARK : CHOICE_MARKS[key.icon];
            const mx = x + Math.floor((kw - mark[0].length) / 2), my = ky + Math.floor((KEY_ROW.h - 2 - mark.length) / 2) + 1;
            mark.forEach((bits, r2) => [...bits].forEach((bit, c) => { if (bit === '1') dot(mx + c, my + r2, ink); }));
          } else {
            const at = screen(x + kw / 2, ky + (KEY_ROW.h - 2) / 2 + 0.5);
            ctx.font = font;
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillStyle = css(ready ? WHITE : ink);
            ctx.fillText(labelOf(key), at.x, at.y);
          }
          row.at.push({ action: key.action, left: x, top: y, right: x + kw, bottom: y + KEY_ROW.h });
          x += kw + KEY_ROW.gap;
        }
      });
    }

    /**
     * How far across the foot of what can be seen a strip `h` tall can go,
     * within `span`: where the foot is the room's own, it stops short of the
     * red button and the mic, so they can still be pressed.
     */
    function footSpan(h, span) {
      let { left, right } = span;
      const top = seen.bottom - 2 - h;
      const { tap } = MIC_KINDS[decor.mic];
      const controls = [['button', BUTTON.x - 7, BUTTON.y - 1, BUTTON.x + 8, BUTTON.y + 16], ['mic', MIC.x + tap.left, MIC.y + tap.top, MIC.x + tap.right, MIC.y + tap.bottom]]
        .map(([part, l, tp, r, b]) => ({ a: placed(part, { x: l, y: tp }), b: placed(part, { x: r, y: b }) }));
      for (const { a, b } of controls) {
        if (b.y < top || b.x < left || a.x > right) continue;
        if ((a.x + b.x) / 2 > (left + right) / 2) right = Math.min(right, Math.floor(a.x) - 3);
        else left = Math.max(left, Math.ceil(b.x) + 3);
      }
      return { left, right };
    }

    /** A strip of a bubble's paper, for keys along the foot of the room. */
    function drawFootStrip(left, top, w, h) {
      fill(left + 1, top, w - 2, h, PALETTE.bubbleEdge);
      fill(left, top + 1, w, h - 2, PALETTE.bubbleEdge);
      fill(left + 1, top + 1, w - 2, h - 2, PALETTE.bubble);
    }

    /** A dark key, going down as it's pressed, as the picked agent's are. */
    function drawDarkKey(x, y, w, h, sunk) {
      const ky = y + sunk;
      fill(x + 1, ky, w - 2, h - sunk, PALETTE.frame);
      fill(x, ky + 1, w, h - 2 - sunk, PALETTE.frame);
      fill(x + 1, ky + 1, w - 2, h - 2 - sunk, sunk ? 0x2a2d35 : 0x3a3d47);
      if (!sunk) fill(x + 1, ky + h - 2, w - 2, 1, 0x2a2d35);
      fill(x + 2, ky + 1, w - 4, 1, WHITE, sunk ? 0.1 : 0.25);
    }

    /**
     * The roles an empty desk held down can hire into, Anyone first, along
     * the foot of what can be seen where the picked agent's keys go: each a
     * dark key with its badge's icon and its name, in one row that slides
     * along under a finger, `row.offset` art pixels, a fade at either end
     * with more past it. The desk held blinks as the picked desk's corners.
     * Where each lands is kept on `row.at`, the strip on `row.box` and how
     * far it can slide on `row.most`, for taps.
     */
    function drawRoleRow(row) {
      const { roles, down } = row;
      if (DESKS[row.desk]) drawBlinkingCorners(row.desk);
      const font = `600 ${Math.max(KEY_ROW.text * scale, 7 * ratio)}px ui-rounded, "SF Pro Rounded", -apple-system, system-ui, sans-serif`;
      ctx.font = font;
      const names = roles.map((role) => String(role.name).slice(0, 16));
      const textOf = names.map((name) => Math.ceil(ctx.measureText(name).width / scale));
      const icons = roles.map(roleIcon);
      const widths = roles.map((_, i) => (icons[i] ? 2 + 16 + 3 : ROLE_ROW.pad) + textOf[i] + ROLE_ROW.pad);
      const ch = ROLE_ROW.h, h = ch + ROLE_ROW.inset * 2;
      const { left, right } = footSpan(h, { left: seen.left + 2, right: seen.right - 2 });
      const w = right - left, top = Math.round(seen.bottom - 2 - h);
      drawFootStrip(left, top, w, h);
      row.box = { left, top, right: left + w, bottom: top + h };
      row.at = [];
      // The row goes no further than its last chip, and what's past the strip's ends is cut off.
      const length = widths.reduce((a, b) => a + b + ROLE_ROW.gap, -ROLE_ROW.gap);
      const most = Math.max(0, length - (w - ROLE_ROW.inset * 2));
      const offset = Math.round(Math.max(0, Math.min(most, row.offset || 0)));
      row.most = most;
      ctx.save();
      const a = screen(left + 1, top), b = screen(right - 1, top + h);
      ctx.beginPath();
      ctx.rect(a.x, a.y, b.x - a.x, b.y - a.y);
      ctx.clip();
      let x = left + ROLE_ROW.inset - offset;
      const y = top + ROLE_ROW.inset;
      roles.forEach((role, i) => {
        const kw = widths[i];
        const sunk = down && down.id === role.id && (down.held || now - down.at < CHOICE_PRESS_LENGTH) ? 1 : 0, ky = y + sunk;
        drawDarkKey(x, y, kw, ch, sunk);
        const p = screen(x + 2, ky + 1);
        if (icons[i]) ctx.drawImage(icons[i], p.x, p.y, 16 * scale, 16 * scale);
        const at = screen(x + (icons[i] ? 2 + 16 + 3 : ROLE_ROW.pad), ky + (ch - 2) / 2 + 0.5);
        ctx.font = font;
        ctx.textAlign = 'left';
        ctx.textBaseline = 'middle';
        ctx.fillStyle = css(sunk ? PALETTE.terminal : PALETTE.bubble);
        ctx.fillText(names[i], at.x, at.y);
        if (x + kw > left && x < right) row.at.push({ id: role.id, left: Math.max(x, left), top: y, right: Math.min(x + kw, right), bottom: y + ch });
        x += kw + ROLE_ROW.gap;
      });
      ctx.restore();
      // A fade at either end that has more past it
      for (let i = 0; i < 6; i++) {
        if (offset > 0) fill(left + 1 + i, top + 1, 1, h - 2, PALETTE.bubble, 0.85 - i * 0.14);
        if (offset < most) fill(right - 2 - i, top + 1, 1, h - 2, PALETTE.bubble, 0.85 - i * 0.14);
      }
    }

    /** Arranging the room: a dotted line round each piece that can be picked, and a solid one round the one picked. */
    function drawArrangement({ pieces, picked, blocked, handle }) {
      for (const { part, box } of pieces) {
        const chosen = part === picked, cut = blocked.includes(part);
        // Picked, in the deck's blue; cut off from the door, in its red
        const edge = cut ? colour('error') : chosen ? colour('thinking') : WHITE, alpha = chosen || cut ? 1 : 0.4, step = chosen ? 1 : 2;
        const l = Math.round(box.left), t = Math.round(box.top), r = Math.round(box.right), b = Math.round(box.bottom);
        for (let x = l; x <= r; x += step) {
          dot(x, t, edge, alpha);
          dot(x, b, edge, alpha);
        }
        for (let y = t; y <= b; y += step) {
          dot(l, y, edge, alpha);
          dot(r, y, edge, alpha);
        }
      }
      if (handle) {
        const x = Math.round(handle.right) - 1, y = Math.round(handle.bottom) - 1;
        fill(x - 1, y - 1, 5, 5, BLACK, 0.6);
        fill(x, y, 3, 3, colour('thinking'));
      }
    }

    /** The styles' key in the top-left corner, held down while its panel is up. Where it lands is kept on `key.at`, for taps. */
    function drawStyleKey(key) {
      const { open } = key;
      const inset = floating && floating !== 'edge' ? STYLE_KEY.rounded : STYLE_KEY.inset;
      const { w, h } = STYLE_KEY, x = seen.left + inset, y = seen.top + inset;
      key.at = { x, y, w, h };
      drawPixelKey(x, y + (open ? 1 : 0), w, h - (open ? 1 : 0), open ? darker(PALETTE.rail) : PALETTE.rail, '');
      const mx = x + Math.floor((w - STYLE_MARK[0].length) / 2), my = y + Math.floor((h - 2 - STYLE_MARK.length) / 2) + (open ? 1 : 0);
      STYLE_MARK.forEach((row, r) => [...row].forEach((on, c) => { if (on === '1') dot(mx + c, my + r, WHITE); }));
    }

    /** The styles over a dimmed room: a picture of each, the one in use outlined. */
    function drawStyleMenu({ left, top, w, h, tiles, current, pictures }) {
      fill(seen.left, seen.top, seen.width, seen.height, BLACK, 0.45);
      fill(left + 1, top, w - 2, h, PALETTE.bubbleEdge);
      fill(left, top + 1, w, h - 2, PALETTE.bubbleEdge);
      fill(left + 1, top + 1, w - 2, h - 2, PALETTE.bubble);
      const size = Math.max(LABEL_SIZE * scale, LABEL_SMALLEST * ratio);
      const font = (weight, px) => `${weight} ${px}px ui-rounded, "SF Pro Rounded", -apple-system, system-ui, sans-serif`;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillStyle = css(PALETTE.bubbleEdge);
      ctx.font = font(700, size * 1.15);
      ctx.fillText('Office style', screen(left + w / 2, 0).x, screen(0, top + STYLE_TITLE / 2 + 1).y);
      ctx.font = font(600, size * 0.9);
      for (const tile of tiles) {
        const on = tile.id === current;
        if (on) fill(tile.x - 2, tile.y - 2, tile.w + 4, STYLE_THUMB.h + 4, colour('thinking'));
        fill(tile.x - 1, tile.y - 1, tile.w + 2, STYLE_THUMB.h + 2, PALETTE.bubbleEdge);
        const picture = pictures.get(tile.id);
        if (picture) {
          const at = screen(tile.x, tile.y);
          ctx.imageSmoothingEnabled = false;
          ctx.drawImage(picture, at.x, at.y, STYLE_THUMB.w * scale, STYLE_THUMB.h * scale);
        }
        ctx.fillStyle = css(PALETTE.bubbleEdge, on ? 1 : 0.75);
        ctx.fillText(fitText(ctx, styleName(tile.id), (tile.w + STYLE_GAP) * scale),
          screen(tile.x + tile.w / 2, 0).x, screen(0, tile.y + STYLE_THUMB.h + STYLE_LABEL / 2 + 1).y);
      }
    }

    /** The question before firing, over a dimmed room. */
    function drawFireDialog({ left, top, w, h, keys, name }) {
      fill(seen.left, seen.top, seen.width, seen.height, BLACK, 0.45);
      fill(left + 1, top, w - 2, h, PALETTE.bubbleEdge);
      fill(left, top + 1, w, h - 2, PALETTE.bubbleEdge);
      fill(left + 1, top + 1, w - 2, h - 2, PALETTE.bubble);
      const size = Math.max(LABEL_SIZE * scale, LABEL_SMALLEST * ratio);
      const font = (weight, px) => `${weight} ${px}px ui-rounded, "SF Pro Rounded", -apple-system, system-ui, sans-serif`;
      const middle = screen(left + w / 2, 0).x;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillStyle = css(PALETTE.bubbleEdge);
      ctx.font = font(700, size * 1.25);
      ctx.fillText(fitText(ctx, `Fire ${name}?`, (w - 12) * scale), middle, screen(0, top + 11).y);
      ctx.font = font(500, size);
      ctx.fillStyle = css(PALETTE.bubbleEdge, 0.7);
      ctx.fillText(fitText(ctx, 'Its agent stops, and its tab closes.', (w - 12) * scale), middle, screen(0, top + 21).y);
      for (const key of keys) drawPixelKey(key.x, key.y, key.w, key.h, key.action === 'fired' ? PALETTE.dome : PALETTE.rail, key.label);
    }

    /**
     * The window onto the room: rings of shadow from its edge inwards, each
     * fainter than the last, then the edge in the keys' colour.
     */
    function drawWindowFrame({ x, y, w, h, r }) {
      ctx.save();
      ctx.lineWidth = ratio;
      for (let ring = 0; ring < WINDOW_SHADOW; ring++) {
        const inset = (ring + 0.5) * ratio;
        ctx.strokeStyle = css(BLACK, WINDOW_SHADOW_DEPTH * (1 - ring / WINDOW_SHADOW) ** 2);
        roundedRect(ctx, x + inset, y + inset, w - inset * 2, h - inset * 2, Math.max(0, r - inset));
        ctx.stroke();
      }
      ctx.strokeStyle = css(fromCss(office.colours.edge) ?? EDGE);
      roundedRect(ctx, x + ratio / 2, y + ratio / 2, w - ratio, h - ratio, r - ratio / 2);
      ctx.stroke();
      if (calling) {
        ctx.strokeStyle = css(colour('asking'), 0.35 + 0.65 * glow);
        ctx.lineWidth = 1.5 * ratio;
        roundedRect(ctx, x + ratio * 0.75, y + ratio * 0.75, w - ratio * 1.5, h - ratio * 1.5, r - ratio * 0.75);
        ctx.stroke();
      }
      ctx.restore();
    }

    /**
     * Under the floating window, before the room: a soft shadow, and while
     * anyone waits on you, the glow round it.
     */
    function drawFloat({ x, y, w, h, r }) {
      ctx.save();
      roundedRect(ctx, x, y, w, h, r);
      ctx.fillStyle = css(BLACK);
      ctx.shadowColor = css(BLACK, 0.55);
      ctx.shadowBlur = FLOAT_SHADOW * ratio;
      ctx.shadowOffsetY = FLOAT_DROP * ratio;
      ctx.fill();
      if (calling) {
        ctx.shadowColor = css(colour('asking'), 0.25 + 0.6 * glow);
        ctx.shadowBlur = GLOW_REACH * ratio;
        ctx.shadowOffsetY = 0;
        ctx.fill();
      }
      ctx.restore();
    }

    /* The frame, back to front */

    drawWall();
    drawSeasonal('wall');
    drawFloor();
    drawSeasonal('floor');
    drawSpotlight();

    // What stands on the floor, drawn in order of where it touches the
    // floor, so that nearer things cover farther ones.
    /** A part of the lounge, at the depth it stands at in the wide room, where the room's shape puts it. */
    const lounge = (part, depth, draw) => [placed(part, { x: 0, y: depth }).y, () => { if (!pieceOf(part).off) moved(part, draw); }];
    const items = [
      lounge('wall', 46.5, drawCounter), lounge('wall', 50, drawCooler), lounge('couch', 110, drawCouchBack),
      lounge('couch', 117, drawCouchFront), lounge('couch', 134, drawCoffeeTable), lounge('pong', 157, drawPingPong),
      lounge('plant', 176, drawPlant), lounge('button', 176.5, drawButton), lounge('mic', 177, drawMic),
      // The corner's piece and each added one, each told where the spots round it start
      ...loungePieces().flatMap(({ part, kind }) => CORNER_PARTS[kind].map(([depth, draw]) => lounge(part, depth, () => {
        cornerBase = spotsFrom(part);
        draw();
      }))),
    ];
    if (shape.loungeWall !== undefined) items.push([shape.loungeWall, drawLoungeWall]);
    for (const pet of office.pets) if (pet.isIn) items.push([!pet.isWalking && pet.target && pet.target.depth ? pet.target.depth : pet.position.y, () => drawPet(pet)]);
    DESKS.forEach((d, desk) => {
      if (d.off) return;
      items.push([d.y + 5, () => drawDesk(desk)]);
      items.push([d.y + 3.5, () => drawChair(desk)]); // just behind whoever sits in it
      items.push([d.y + 3, () => drawWorkspaceBack(desk)]);
    });
    // Someone being carried is drawn on top, where the finger has them, not at their desk. Someone at
    // the clock or the whiteboard is in front of the first row's workspaces, the tallest of which reach
    // the wall: they're there to be seen.
    const atWall = (person) => !person.isWalking && /^(clock|board|crowd):/.test(person.place);
    for (const person of office.people) {
      if (!person.isStaff && DESKS[person.desk] && DESKS[person.desk].off) {
        const { x, y } = person.position;
        crowdAnchors.set(person.desk, { x: Math.round(x) - DESK_WIDTH / 2, y: Math.round(y) - 3, feet: { x: Math.round(x), y: Math.round(y) } });
      }
      if (person !== carried && !person.entering) items.push([atWall(person) ? DESKS[0].y + 5.5 : person.position.y, () => drawPerson(person)]);
    }
    office.seats.forEach((s, desk) => { if (s.session && s.session.compacting && !DESKS[desk].off) items.push([DESKS[desk].y + 16, () => drawShredder(desk)]); });
    for (const intern of office.interns) {
      // One still waiting for room hasn't come in yet.
      if (intern.slot !== null || intern.isLeaving) items.push([intern.position.y, () => drawIntern(intern)]);
    }
    items.sort((a, b) => a[0] - b[0]).forEach(([, draw]) => draw());

    drawSeasonal('air');
    if (isNight) fill(seen.left, seen.top, seen.width, seen.height, PALETTE.night, 0.3);
    drawSeasonal('glow');
    drawSelectionCorners();
    drawDetonation();
    drawMicWaves();
    // The screens' light stays off whoever stands in the line in front of them
    const inLine = office.people.filter((p) => !p.isWalking && p.place.startsWith('crowd:'));
    if (inLine.length) {
      ctx.save();
      ctx.beginPath();
      ctx.rect(0, 0, width, height);
      for (const p of inLine) {
        const feet = { x: Math.round(p.position.x), y: Math.round(p.position.y) }, top = topOf(p, poseOf(p, now));
        ctx.rect(ox + (feet.x - 5) * scale, oy + top * scale, 11 * scale, (feet.y + 1 - top) * scale);
      }
      ctx.clip('evenodd');
    }
    DESKS.forEach((_, desk) => drawScreenLight(desk));
    if (inLine.length) ctx.restore();
    DESKS.forEach((_, desk) => drawTimer(desk));
    if (frame.carrying) drawCarried(frame.carrying);
    office.people.forEach((person) => { if (!person.entering) drawBubble(person); });
    office.people.forEach(drawConfetti);
    // The picked one's last, over those beside it in the line
    DESKS.forEach((_, desk) => { if (desk !== selectedDesk) drawLabel(desk); });
    if (selectedDesk >= 0) drawLabel(selectedDesk);
    if (frame.chips) drawChips();
    if (frame.deskButton) drawDeskButton(frame.deskButton);
    if (frame.choices) drawChoices();
    if (frame.arranging) drawArrangement(frame.arranging);
    if (frame.musicTap) drawMusicTap(frame.musicTap);
    if (frame.musicPanel) drawMusicPanel(frame.musicPanel);
    if (frame.keyRow) drawKeyRow(frame.keyRow);
    if (frame.roleRow) drawRoleRow(frame.roleRow);
    if (frame.styleKey) drawStyleKey(frame.styleKey);
    if (frame.firing) drawFireDialog(frame.firing);
    if (frame.styles) drawStyleMenu(frame.styles);

    if (glass) {
      ctx.restore();
      drawWindowFrame(glass);
    }
  }

  /** `text`, cut with an ellipsis to fit `room` pixels in the context's font. */
  function fitText(ctx, text, room) {
    if (ctx.measureText(text).width <= room) return text;
    let chars = [...text];
    while (chars.length > 1 && ctx.measureText(`${chars.join('')}…`).width > room) chars = chars.slice(0, -1);
    return `${chars.join('').trimEnd()}…`;
  }

  function roundedRect(ctx, x, y, w, h, radius) {
    // Square corners, as the screen saver has, can bring an inner ring's
    // radius below nothing, which the canvas refuses outright.
    const r = Math.max(0, radius);
    ctx.beginPath();
    ctx.moveTo(x + r, y);
    ctx.arcTo(x + w, y, x + w, y + h, r);
    ctx.arcTo(x + w, y + h, x, y + h, r);
    ctx.arcTo(x, y + h, x, y, r);
    ctx.arcTo(x, y, x + w, y, r);
    ctx.closePath();
  }

  /* -------------------------------------------------------------- sound */

  /** How loud the office is, as set up: its master volume. */
  const SOUND_LEVELS = { off: 0, quiet: 0.4, normal: 1 };
  /**
   * The loudest the room gets: the reminder's level, at 0 dB. Everything
   * else is set below it, in dB, so the one sound that asks for you stands
   * out and the rest stays in the background.
   */
  const REFERENCE = 0.15;
  const dB = (level) => REFERENCE * 10 ** (level / 20);
  /** A console's frame: envelopes and pitch sweeps step at this rate, which is most of what makes them sound 8-bit. */
  const FRAME = 1 / 60;

  /** A note by name, natural notes only, C4 being middle C. Every tune keeps to C major pentatonic, so nothing that overlaps can clash. */
  const NOTE = { C: 0, D: 2, E: 4, F: 5, G: 7, A: 9, B: 11 };
  const hz = (name) => 440 * 2 ** ((NOTE[name[0]] + 12 * (Number(name.slice(1)) + 1) - 69) / 12);

  /** Where across the room a sound comes from, as a pan: kept towards the middle, so headphones don't feel lopsided. */
  const panAt = (x) => Math.max(-1, Math.min(1, (x / WIDTH) * 2 - 1)) * 0.6;

  /**
   * The channels of an old console: pulse waves of three widths, a triangle
   * in 16 steps, as the NES plays it, and noise from a shift register, kept
   * for what should crunch. Each is made once for a context.
   */
  const waveCache = new WeakMap();
  function chipWaves(audio) {
    if (waveCache.has(audio)) return waveCache.get(audio);
    const HARMONICS = 40;
    // A wave held at each level of `steps` for an equal share of its cycle, as its harmonics.
    const stepped = (steps) => {
      const real = new Float32Array(HARMONICS + 1), imag = new Float32Array(HARMONICS + 1);
      for (let n = 1; n <= HARMONICS; n++) {
        steps.forEach((level, k) => {
          const a = (2 * Math.PI * n * k) / steps.length, b = (2 * Math.PI * n * (k + 1)) / steps.length;
          real[n] += (level * (Math.sin(b) - Math.sin(a))) / (Math.PI * n);
          imag[n] += (level * (Math.cos(a) - Math.cos(b))) / (Math.PI * n);
        });
      }
      return audio.createPeriodicWave(real, imag);
    };
    const pulse = (eighths) => stepped(Array.from({ length: 8 }, (_, k) => (k < eighths ? 1 : -1)));
    const triangle = stepped(Array.from({ length: 32 }, (_, k) => (k < 16 ? k : 31 - k) / 7.5 - 1));
    // Each step of the register is held for two samples; a sound pitches it with its playback rate.
    const noise = (length) => {
      const buffer = audio.createBuffer(1, length, audio.sampleRate);
      const data = buffer.getChannelData(0);
      for (let i = 0, register = 1; i < length; i += 2) {
        const bit = (register ^ (register >> 1)) & 1;
        register = (register >> 1) | (bit << 14);
        data.fill(register & 1 ? -1 : 1, i, i + 2);
      }
      return buffer;
    };
    const waves = {
      pulse12: pulse(1), pulse25: pulse(2), pulse50: pulse(4), triangle,
      hiss: noise(audio.sampleRate),
    };
    waveCache.set(audio, waves);
    return waves;
  }

  /** Plays chip notes into one output: everything the office and the deck say is made of these two. */
  class Chip {
    constructor(audio, out) {
      this.audio = audio;
      this.out = out;
      this.waves = chipWaves(audio);
    }

    /** Into the output, panned from -1, the left, to 1, the right. */
    route(node, pan) {
      if (!pan || !this.audio.createStereoPanner) return node.connect(this.out);
      const panner = this.audio.createStereoPanner();
      panner.pan.value = pan;
      node.connect(panner).connect(this.out);
    }

    /**
     * The volume stepping down a frame at a time, in 15 steps, from `level`
     * to nothing over `length`; the higher the `curve`, the quicker it drops.
     * Each step eases in over a few milliseconds, which keeps it from clicking
     * or buzzing.
     */
    envelope(at, length, level, curve) {
      const gain = this.audio.createGain();
      gain.gain.value = 0;
      gain.gain.setValueAtTime(0, at);
      const frames = Math.max(1, Math.round(length / FRAME));
      for (let k = 0; k < frames; k++) {
        gain.gain.setTargetAtTime((level * Math.ceil(15 * (1 - k / frames) ** curve)) / 15, at + k * FRAME, k ? 0.005 : 0.002);
      }
      gain.gain.setTargetAtTime(0, at + length, 0.004);
      return gain;
    }

    /** A pitch stepped a frame at a time from `from` to `to`, with a wobble of `vibrato`, a share of the pitch. */
    sweep(param, at, length, from, to, vibrato) {
      const frames = Math.max(1, Math.round(length / FRAME));
      for (let k = 0; k <= frames; k++) {
        const wobble = vibrato ? 1 + vibrato * Math.sin(k * FRAME * 2 * Math.PI * 6) : 1;
        param.setValueAtTime(from * (to / from) ** (k / frames) * wobble, at + k * FRAME);
      }
    }

    /** A note on a pulse or the triangle. */
    note(at, { wave = 'pulse25', pitch, to = pitch, length, level, pan = 0, curve = 1.5, vibrato = 0 }) {
      const osc = this.audio.createOscillator();
      osc.setPeriodicWave(this.waves[wave]);
      this.sweep(osc.frequency, at, length, pitch, to, vibrato);
      this.route(osc.connect(this.envelope(at, length, level, curve)), pan);
      osc.start(at);
      osc.stop(at + length + 0.05);
    }

    /** A burst of noise, its `rate` a playback rate from 1, the brightest, down. */
    noise(at, { rate = 1, to = rate, length, level, pan = 0, curve = 1.5, lowpass = 0 }) {
      const source = this.audio.createBufferSource();
      source.buffer = this.waves.hiss;
      source.loop = true;
      this.sweep(source.playbackRate, at, length, rate, to, 0);
      let node = source;
      if (lowpass) {
        const filter = this.audio.createBiquadFilter();
        filter.type = 'lowpass';
        filter.frequency.value = lowpass;
        node = node.connect(filter);
      }
      this.route(node.connect(this.envelope(at, length, level, curve)), pan);
      source.start(at, Math.random() * source.buffer.duration * 0.9);
      source.stop(at + length + 0.05);
    }
  }

  /** The deck's one reminder: two bright notes and their echo, the only sound that asks for you. */
  function remindSound(chip, at, pan = 0) {
    [['A5', 0], ['E6', 0.14]].forEach(([name, after]) => {
      for (const [late, level] of [[0, 0], [0.13, -12]]) {
        chip.note(at + after + late, { wave: 'pulse25', pitch: hz(name), length: 0.42, level: dB(level), pan, curve: 1.2 });
        chip.note(at + after + late, { wave: 'triangle', pitch: hz(name) / 2, length: 0.3, level: dB(level - 9), pan });
      }
    });
  }

  /**
   * Every sound the office knows, by name, with how soon it may play again,
   * in seconds, and how loud it is, in dB below the reminder. Anything in the
   * room can ask for one with `office.cue(name, x)`; a name with nothing
   * here yet is let go quietly.
   */
  const SOUNDS = {
    // A long turn done: a fanfare on a pulse over a triangle bass, -10.
    fanfare: {
      gap: 2,
      play(chip, at, pan) {
        const lead = [['E5', 0.09], ['G5', 0.09], ['C6', 0.09], ['E6', 0.09], ['D6', 0.12], ['E6', 0.12], ['G6', 0.5]];
        let when = at;
        for (const [name, length] of lead) {
          chip.note(when, { wave: 'pulse25', pitch: hz(name), length: length + 0.04, level: dB(-10), pan, curve: 0.6, vibrato: length > 0.3 ? 0.006 : 0 });
          when += length;
        }
        [['C3', 0.36], ['G3', 0.24], ['C4', 0.6]].reduce((t, [name, length]) => {
          chip.note(t, { wave: 'triangle', pitch: hz(name), length, level: dB(-12), pan, curve: 0.5 });
          return t + length;
        }, at);
      },
    },
    // The red button: a chip explosion, then a two-tone siren for as long as the warning light turns, -8.
    blast: {
      gap: 1,
      play(chip, at, pan) {
        chip.noise(at, { rate: 0.5, to: 0.02, length: 1.1, level: dB(-8), pan, curve: 1.2 });
        chip.note(at, { wave: 'triangle', pitch: hz('G2'), to: hz('C1'), length: 0.5, level: dB(-10), pan });
        for (let k = 0; k < 10; k++) {
          chip.note(at + 0.5 + k * 0.34, { wave: 'pulse25', pitch: hz(k % 2 ? 'D5' : 'G5'), length: 0.32, level: dB(-18 - k * 1.4), pan: 0, curve: 0.4 });
        }
      },
    },
    // Someone new at a desk: three notes up, -10.
    hire: {
      gap: 3,
      play(chip, at, pan) {
        ['C5', 'E5', 'G5', 'C6'].forEach((name, k) => {
          chip.note(at + k * 0.07, { wave: 'pulse25', pitch: hz(name), length: k === 3 ? 0.3 : 0.08, level: dB(-10), pan });
        });
      },
    },
    // Someone fired, picking up their box: a sad trombone, falling, -10.
    fire: {
      gap: 3,
      play(chip, at, pan) {
        [['G4', 0.16], ['E4', 0.16], ['D4', 0.16], ['C4', 0.5]].reduce((t, [name, length], k) => {
          const pitch = hz(name);
          chip.note(t, { wave: 'pulse50', pitch, to: k === 3 ? pitch * 0.94 : pitch * 0.98, length, level: dB(-12), pan, curve: 0.7, vibrato: k === 3 ? 0.012 : 0 });
          return t + length + 0.02;
        }, at);
      },
    },
    // The door, as someone comes or goes: the latch's click, then ka-chunk, -14.
    door: {
      gap: 1.2,
      play(chip, at, pan) {
        chip.note(at, { wave: 'pulse12', pitch: hz('C6'), length: 0.015, level: dB(-26), pan, curve: 3 });
        chip.note(at + 0.01, { wave: 'triangle', pitch: hz('G3'), to: hz('D3'), length: 0.08, level: dB(-14), pan });
        chip.note(at + 0.14, { wave: 'triangle', pitch: hz('D3'), to: hz('G2'), length: 0.1, level: dB(-15), pan });
      },
    },
    // The ball off a paddle, -18, and off the table between, -26.
    paddle: {
      play(chip, at, pan) {
        chip.note(at, { wave: 'pulse12', pitch: pan < 0 ? hz('D6') : hz('C6'), to: hz('G5'), length: 0.05, level: dB(-18), pan, curve: 2 });
        chip.noise(at, { rate: 0.7, length: 0.02, level: dB(-24), pan, curve: 2 });
      },
    },
    table: {
      play(chip, at, pan) {
        chip.note(at, { wave: 'pulse12', pitch: hz('G6'), to: hz('E6'), length: 0.03, level: dB(-26), pan, curve: 2 });
      },
    },
    // The coffee machine: bubbles rising, then a drip, -18.
    coffee: {
      gap: 4,
      play(chip, at, pan) {
        for (let k = 0; k < 6; k++) {
          const pitch = hz(pick(['C4', 'D4', 'E4', 'G4']));
          chip.note(at + 0.25 + k * random(0.1, 0.15), { wave: 'triangle', pitch, to: pitch * 1.9, length: 0.06, level: dB(-18), pan });
        }
        chip.note(at + 1.2, { wave: 'pulse12', pitch: hz('G6'), to: hz('C6'), length: 0.05, level: dB(-22), pan, curve: 2 });
      },
    },
    // An agent getting to work: a second of a mechanical keyboard, each key
    // a switch's click and the key bottoming out, the space bar lower, -18.
    typing: {
      gap: 1.5,
      play(chip, at, pan) {
        const count = Math.floor(random(13, 17));
        for (let k = 0, when = at; k < count; k++) {
          const space = k === count - 1 || (k > 2 && Math.random() < 0.12);
          const thock = space ? hz('E3') : hz('A3') * random(0.94, 1.12);
          chip.note(when, { wave: 'pulse12', pitch: random(2200, 2900), length: 0.01, level: dB(-26), pan, curve: 3 });
          chip.note(when + 0.004, { wave: 'triangle', pitch: thock * 2, to: thock, length: space ? 0.05 : 0.03, level: dB(space ? -18 : -20), pan, curve: 2 });
          when += random(0.055, 0.12) * (Math.random() < 0.12 ? 2.2 : 1); // now and then, a pause for thought
        }
      },
    },
    // Picking an agent up off their desk, and putting them down at another, -16.
    'pick-up': {
      play(chip, at, pan) {
        chip.note(at, { wave: 'pulse25', pitch: hz('G4'), to: hz('G5'), length: 0.08, level: dB(-16), pan });
      },
    },
    'put-down': {
      play(chip, at, pan) {
        chip.note(at, { wave: 'triangle', pitch: hz('E4'), to: hz('E3'), length: 0.1, level: dB(-14), pan });
      },
    },
    // The mic switched on and off: a blip up and a blip down, -18.
    'mic-on': {
      play(chip, at, pan) {
        chip.note(at, { wave: 'pulse12', pitch: hz('C6'), length: 0.03, level: dB(-18), pan });
        chip.note(at + 0.04, { wave: 'pulse12', pitch: hz('G6'), length: 0.04, level: dB(-18), pan });
      },
    },
    'mic-off': {
      play(chip, at, pan) {
        chip.note(at, { wave: 'pulse12', pitch: hz('G6'), length: 0.03, level: dB(-20), pan });
        chip.note(at + 0.04, { wave: 'pulse12', pitch: hz('C6'), length: 0.04, level: dB(-20), pan });
      },
    },
    // The megaphone's on: its blips, and a little squeal of feedback, -24.
    megaphone: {
      play(chip, at, pan) {
        SOUNDS['mic-on'].play(chip, at, pan);
        chip.note(at + 0.08, { wave: 'pulse50', pitch: hz('E7'), to: hz('G7'), length: 0.16, level: dB(-28), pan, curve: 0.8, vibrato: 0.02 });
      },
    },
    // The pets, now and then as they settle somewhere, -18; the lizard barely, -26.
    cat: {
      gap: 40,
      play(chip, at, pan) {
        chip.note(at, { wave: 'pulse12', pitch: hz('G5'), to: hz('C6'), length: 0.1, level: dB(-18), pan, curve: 0.3 });
        chip.note(at + 0.1, { wave: 'pulse12', pitch: hz('C6'), to: hz('F5'), length: 0.2, level: dB(-18), pan, vibrato: 0.01 });
      },
    },
    dog: {
      gap: 40,
      play(chip, at, pan) {
        for (const after of [0, 0.15]) chip.note(at + after, { wave: 'pulse25', pitch: hz('A5'), to: hz('E5'), length: 0.07, level: dB(-18), pan });
      },
    },
    lizard: {
      gap: 60,
      play(chip, at, pan) {
        for (const after of [0, 0.06]) chip.note(at + after, { wave: 'pulse12', pitch: hz('C7'), length: 0.015, level: dB(-26), pan, curve: 3 });
      },
    },
    monkey: {
      gap: 40,
      play(chip, at, pan) {
        [['E4', 'A4'], ['E4', 'A4'], ['G4', 'D5']].forEach(([from, to], k) => {
          chip.note(at + k * 0.11, { wave: 'triangle', pitch: hz(from), to: hz(to), length: 0.09, level: dB(-18), pan });
        });
      },
    },
    // The first pets' habits: the monkey off with a cup, the dog after someone, the cat purring asleep, the lizard frozen on a wall
    monkeyCup: {
      gap: 20,
      play(chip, at, pan) {
        chip.note(at, { wave: 'pulse12', pitch: hz('E7'), length: 0.03, level: dB(-22), pan, curve: 2 });
        chip.note(at + 0.05, { wave: 'pulse12', pitch: hz('B6'), length: 0.04, level: dB(-24), pan, curve: 2 });
        for (const [after, from, to] of [[0.16, 'E4', 'A4'], [0.26, 'G4', 'D5']]) chip.note(at + after, { wave: 'triangle', pitch: hz(from), to: hz(to), length: 0.08, level: dB(-20), pan });
      },
    },
    dogFollow: {
      gap: 30,
      play(chip, at, pan) {
        for (const after of [0, 0.13]) chip.note(at + after, { wave: 'pulse25', pitch: hz('F5'), to: hz('C5'), length: 0.06, level: dB(-20), pan });
      },
    },
    catPurr: {
      gap: 60,
      play(chip, at, pan) {
        for (const after of [0, 0.42]) chip.note(at + after, { wave: 'triangle', pitch: hz('C3'), to: hz('D3'), length: 0.36, level: dB(-26), pan, curve: 0.6, vibrato: 0.04 });
      },
    },
    lizardFreeze: {
      gap: 60,
      play(chip, at, pan) {
        chip.noise(at, { rate: 1, length: 0.015, level: dB(-30), pan, curve: 3 });
        chip.noise(at + 0.05, { rate: 1, length: 0.012, level: dB(-32), pan, curve: 3 });
      },
    },
    // The new pets, as they settle, -20 to -34; and their habits, as they happen, the llama's spit the boldest, -16.
    llama: {
      gap: 50,
      play(chip, at, pan) {
        // A slow chew: low, soft, twice
        for (const after of [0, 0.32]) chip.note(at + after, { wave: 'triangle', pitch: hz('G3'), to: hz('E3'), length: 0.2, level: dB(-22), pan, curve: 0.8 });
      },
    },
    llamaSpit: {
      gap: 8,
      play(chip, at, pan) {
        // Ptoo: a puff, a little whistle down as it flies, and the splat
        chip.noise(at, { rate: 1, to: 0.6, length: 0.05, level: dB(-16), pan, curve: 2 });
        chip.note(at + 0.04, { wave: 'pulse12', pitch: hz('C6'), to: hz('E5'), length: 0.22, level: dB(-20), pan, curve: 0.8 });
        chip.noise(at + 0.3, { rate: 0.5, to: 0.3, length: 0.09, level: dB(-18), pan, lowpass: 1800 });
      },
    },
    penguin: {
      gap: 45,
      play(chip, at, pan) {
        for (const after of [0, 0.16]) chip.note(at + after, { wave: 'pulse25', pitch: hz('E5'), to: hz('C5'), length: 0.1, level: dB(-20), pan, curve: 0.6 });
      },
    },
    penguinSlide: {
      gap: 20,
      play(chip, at, pan) {
        chip.noise(at, { rate: 0.5, to: 0.25, length: 0.7, level: dB(-26), pan, curve: 0.6, lowpass: 2600 });
      },
    },
    parrot: {
      gap: 40,
      play(chip, at, pan) {
        chip.note(at, { wave: 'pulse12', pitch: hz('A6'), to: hz('E6'), length: 0.07, level: dB(-20), pan, curve: 0.5 });
        chip.note(at + 0.09, { wave: 'pulse12', pitch: hz('C7'), to: hz('G6'), length: 0.11, level: dB(-20), pan, curve: 0.5 });
      },
    },
    owl: {
      gap: 60,
      play(chip, at, pan) {
        // Hoo, hoooo: only at night
        chip.note(at, { wave: 'triangle', pitch: hz('G4'), to: hz('F4'), length: 0.22, level: dB(-22), pan, curve: 0.7 });
        chip.note(at + 0.34, { wave: 'triangle', pitch: hz('G4'), to: hz('E4'), length: 0.5, level: dB(-22), pan, curve: 0.5, vibrato: 0.012 });
      },
    },
    capybara: {
      gap: 60,
      play(chip, at, pan) {
        for (const after of [0, 0.08, 0.16]) chip.note(at + after, { wave: 'triangle', pitch: hz('E5'), length: 0.05, level: dB(-24), pan, curve: 1.5 });
      },
    },
    redpanda: {
      gap: 45,
      play(chip, at, pan) {
        for (const after of [0, 0.1]) chip.note(at + after, { wave: 'pulse12', pitch: hz('B5'), to: hz('D6'), length: 0.05, level: dB(-22), pan });
      },
    },
    redpandaStartle: {
      gap: 6,
      play(chip, at, pan) {
        chip.note(at, { wave: 'pulse25', pitch: hz('C5'), to: hz('C6'), length: 0.16, level: dB(-18), pan, curve: 0.4 });
      },
    },
    corgi: {
      gap: 40,
      play(chip, at, pan) {
        chip.note(at, { wave: 'pulse25', pitch: hz('C6'), to: hz('A5'), length: 0.06, level: dB(-20), pan });
      },
    },
    corgiLap: {
      gap: 6,
      play(chip, at, pan) {
        for (const after of [0, 0.14, 0.28]) chip.note(at + after, { wave: 'pulse25', pitch: hz('B5'), to: hz('F5'), length: 0.07, level: dB(-18), pan });
      },
    },
    hamster: {
      gap: 40,
      play(chip, at, pan) {
        // A squeak, not a whistle: a fuller pulse, a little lower
        chip.note(at, { wave: 'pulse25', pitch: hz('C7'), to: hz('D7'), length: 0.035, level: dB(-27), pan, curve: 2 });
      },
    },
    hamsterRoll: {
      gap: 25,
      play(chip, at, pan) {
        chip.noise(at, { rate: 0.28, to: 0.22, length: 0.8, level: dB(-28), pan, curve: 0.5, lowpass: 900 });
      },
    },
    robodog: {
      gap: 40,
      play(chip, at, pan) {
        [['C6', 0], ['E6', 0.07], ['G6', 0.14]].forEach(([name, after]) => chip.note(at + after, { wave: 'pulse50', pitch: hz(name), length: 0.045, level: dB(-22), pan, curve: 1 }));
      },
    },
    robodogCharge: {
      gap: 40,
      play(chip, at, pan) {
        chip.note(at, { wave: 'triangle', pitch: hz('C4'), to: hz('C5'), length: 0.6, level: dB(-26), pan, curve: 0.3 });
      },
    },
    bat: {
      gap: 50,
      play(chip, at, pan) {
        for (const after of [0, 0.05, 0.1]) chip.note(at + after, { wave: 'pulse25', pitch: hz('B6'), length: 0.02, level: dB(-30), pan, curve: 2 });
      },
    },
    snail: {
      gap: 90,
      play(chip, at, pan) {
        chip.noise(at, { rate: 0.2, length: 0.5, level: dB(-34), pan, curve: 0.5, lowpass: 600 });
      },
    },
    fox: {
      gap: 45,
      play(chip, at, pan) {
        for (const after of [0, 0.12]) chip.note(at + after, { wave: 'pulse25', pitch: hz('E6'), to: hz('B5'), length: 0.08, level: dB(-20), pan, curve: 0.6 });
      },
    },
    foxPounce: {
      gap: 20,
      play(chip, at, pan) {
        chip.noise(at, { rate: 0.4, to: 0.25, length: 0.07, level: dB(-22), pan, lowpass: 1400 });
        chip.note(at, { wave: 'triangle', pitch: hz('E4'), to: hz('C4'), length: 0.08, level: dB(-22), pan });
      },
    },
    axolotl: {
      gap: 30,
      play(chip, at, pan) {
        // Bubbles, rising
        [['C6', 0], ['E6', 0.09], ['A6', 0.2]].forEach(([name, after]) => chip.note(at + after, { wave: 'triangle', pitch: hz(name), to: hz(name) * 1.2, length: 0.03, level: dB(-28), pan, curve: 1.2 }));
      },
    },
    // The seasons in the windows, now and then: the wind whistling in winter, -30, a bird in spring and summer, -26.
    wind: {
      play(chip, at) {
        chip.note(at, { wave: 'triangle', pitch: hz('D5'), to: hz('A5'), length: 1.2, level: dB(-30), curve: 0.3, vibrato: 0.01 });
        chip.note(at + 1.2, { wave: 'triangle', pitch: hz('A5'), to: hz('E5'), length: 1.4, level: dB(-30), curve: 1, vibrato: 0.01 });
      },
    },
    bird: {
      play(chip, at) {
        const pan = random(-0.4, 0.4);
        for (let k = 0; k < (Math.random() < 0.5 ? 2 : 3); k++) {
          chip.note(at + k * 0.12, { wave: 'pulse12', pitch: hz('E7'), to: hz('G7'), length: 0.05, level: dB(-26), pan });
        }
      },
    },
    // The corner piece, while someone's at it.
    arcade: {
      play(chip, at, pan) {
        // A bit of a game, -24: a run of notes, and sometimes a coin's two notes up.
        const scale = ['C6', 'D6', 'E6', 'G6', 'A6'];
        let when = at;
        for (let k = 0; k < Math.floor(random(3, 6)); k++, when += 0.07) chip.note(when, { wave: 'pulse12', pitch: hz(pick(scale)), length: 0.05, level: dB(-24), pan });
        if (Math.random() < 0.3) {
          chip.note(when + 0.1, { wave: 'pulse25', pitch: hz('B5'), length: 0.06, level: dB(-24), pan });
          chip.note(when + 0.16, { wave: 'pulse25', pitch: hz('E6'), length: 0.25, level: dB(-24), pan });
        }
      },
    },
    foosball: {
      play(chip, at, pan) {
        // The ball struck and hitting the side, -20.
        for (const after of Math.random() < 0.5 ? [0] : [0, random(0.2, 0.4)]) {
          chip.note(at + after, { wave: 'pulse12', pitch: hz('E6'), length: 0.012, level: dB(-24), pan, curve: 3 });
          chip.note(at + after, { wave: 'triangle', pitch: hz('A4'), to: hz('E4'), length: 0.05, level: dB(-20), pan });
        }
      },
    },
    fishtank: {
      play(chip, at, pan) {
        // Bubbles, -26.
        for (let k = 0; k < 3; k++) chip.note(at + k * random(0.08, 0.16), { wave: 'triangle', pitch: hz('G4'), to: hz('D5') * random(1, 1.4), length: 0.04, level: dB(-26), pan });
      },
    },
    // The newer corner pieces, as quiet as the rest of the room's goings-on.
    pinball: {
      play(chip, at, pan) {
        // Flippers, -24, and the ball off a bumper or two, -26.
        chip.note(at, { wave: 'triangle', pitch: hz('E4'), to: hz('C4'), length: 0.04, level: dB(-24), pan, curve: 3 });
        let when = at + random(0.15, 0.3);
        for (let k = 0; k < Math.floor(random(1, 4)); k++, when += random(0.1, 0.2)) {
          chip.note(when, { wave: 'pulse25', pitch: hz(pick(['G5', 'A5', 'C6'])), to: hz('C7'), length: 0.06, level: dB(-26), pan });
        }
      },
    },
    jukebox: {
      play(chip, at, pan) {
        // A phrase of a song, -26, a bass under it.
        const tune = pick([['C5', 'E5', 'G5', 'E5'], ['A4', 'C5', 'D5', 'E5'], ['G5', 'E5', 'D5', 'C5']]);
        tune.forEach((name, k) => {
          chip.note(at + k * 0.22, { wave: 'pulse25', pitch: hz(name), length: 0.2, level: dB(-26), pan, curve: 0.8 });
          if (k % 2 === 0) chip.note(at + k * 0.22, { wave: 'triangle', pitch: hz(name) / 4, length: 0.4, level: dB(-28), pan });
        });
      },
    },
    pool: {
      play(chip, at, pan) {
        // The cue, then a ball off another, -22.
        chip.note(at, { wave: 'triangle', pitch: hz('C4'), to: hz('G3'), length: 0.04, level: dB(-24), pan, curve: 3 });
        const click = at + random(0.25, 0.45);
        chip.note(click, { wave: 'pulse12', pitch: hz('E7'), length: 0.015, level: dB(-22), pan, curve: 3 });
        if (Math.random() < 0.5) chip.note(click + random(0.3, 0.5), { wave: 'triangle', pitch: hz('A3'), to: hz('E3'), length: 0.08, level: dB(-26), pan });
      },
    },
    piano: {
      play(chip, at, pan) {
        // A few notes up and back, -24.
        const run = pick([['C5', 'E5', 'G5', 'C6', 'G5'], ['A4', 'C5', 'E5', 'A5'], ['D5', 'G5', 'A5', 'D6', 'A5']]);
        run.forEach((name, k) => chip.note(at + k * 0.16, { wave: 'triangle', pitch: hz(name), length: 0.5, level: dB(-24), pan, curve: 1.2 }));
      },
    },
    drums: {
      play(chip, at, pan) {
        // A bar on the kit: kick, hat, snare, hat, -24.
        for (let k = 0; k < 4; k++) {
          const beat = at + k * 0.2;
          if (k % 2 === 0) chip.note(beat, { wave: 'triangle', pitch: hz('C4'), to: hz('C2'), length: 0.1, level: dB(-22), pan, curve: 2 });
          else chip.note(beat, { wave: 'pulse50', pitch: hz('E5'), to: hz('C5'), length: 0.06, level: dB(-28), pan, curve: 3 });
          chip.note(beat + 0.1, { wave: 'pulse12', pitch: hz('G7'), length: 0.012, level: dB(-30), pan, curve: 3 });
        }
      },
    },
    dragon: {
      gap: 20,
      play(chip, at, pan) {
        // A snore from deep in its chest, -26.
        chip.note(at, { wave: 'triangle', pitch: hz('C2'), to: hz('E2'), length: 1.1, level: dB(-26), pan, curve: 0.5, vibrato: 0.04 });
        chip.note(at + 1.3, { wave: 'triangle', pitch: hz('E2'), to: hz('A1'), length: 0.9, level: dB(-28), pan, curve: 0.6, vibrato: 0.05 });
      },
    },
    twinkle: {
      play(chip, at, pan) {
        // A star found, -28.
        for (let k = 0; k < 3; k++) chip.note(at + k * 0.09, { wave: 'pulse12', pitch: hz(pick(['E7', 'G7', 'A7'])), length: 0.08, level: dB(-28), pan });
      },
    },
    hum: {
      play(chip, at, pan) {
        // Machines at work: a few blinks, -28.
        for (let k = 0; k < 3; k++) chip.note(at + k * random(0.12, 0.25), { wave: 'pulse12', pitch: hz(pick(['C7', 'G6'])), length: 0.03, level: dB(-28), pan });
      },
    },
    printer: {
      play(chip, at, pan) {
        // The head stepping to and fro, -28.
        for (let k = 0; k < 6; k++) {
          chip.note(at + k * 0.1, { wave: 'pulse50', pitch: hz(pick(['C4', 'D4', 'E4'])), length: 0.08, level: dB(-30), pan, curve: 0.5 });
        }
      },
    },
    boxing: {
      play(chip, at, pan) {
        // The bag hit, once or twice, -22.
        for (const after of Math.random() < 0.5 ? [0] : [0, 0.22]) {
          chip.note(at + after, { wave: 'triangle', pitch: hz('A3'), to: hz('C3'), length: 0.06, level: dB(-22), pan, curve: 2.5 });
        }
      },
    },
    treadmill: {
      play(chip, at, pan) {
        // Steps on the belt, -28.
        for (let k = 0; k < 6; k++) chip.note(at + k * 0.18, { wave: 'triangle', pitch: hz(k % 2 ? 'E3' : 'D3'), length: 0.03, level: dB(-28), pan, curve: 3 });
      },
    },
    splash: {
      play(chip, at, pan) {
        // Bubbles in warm water, lower than the fish tank's, -26.
        for (let k = 0; k < 4; k++) chip.note(at + k * random(0.08, 0.18), { wave: 'triangle', pitch: hz('C4'), to: hz('G4') * random(1, 1.3), length: 0.05, level: dB(-26), pan });
      },
    },
    rustle: {
      play(chip, at, pan) {
        // Balls rolling over each other, -28.
        for (let k = 0; k < 5; k++) chip.note(at + k * random(0.04, 0.09), { wave: 'pulse12', pitch: hz(pick(['G5', 'A5', 'C6', 'D6'])), length: 0.02, level: dB(-28), pan, curve: 3 });
      },
    },
    crackle: {
      play(chip, at, pan) {
        // The fire popping, now and then, -28.
        for (let k = 0; k < Math.floor(random(2, 5)); k++) {
          chip.note(at + random(0, 0.6), { wave: 'pulse12', pitch: hz(pick(['C6', 'E6', 'A6'])), length: 0.01, level: dB(-28), pan, curve: 3 });
        }
      },
    },
    vending: {
      gap: 4,
      play(chip, at, pan) {
        // A button, the machine working, a can dropping, -18.
        chip.note(at + 0.2, { wave: 'pulse12', pitch: hz('A6'), length: 0.04, level: dB(-22), pan });
        chip.note(at + 0.27, { wave: 'pulse12', pitch: hz('A6'), length: 0.04, level: dB(-22), pan });
        chip.note(at + 0.4, { wave: 'pulse50', pitch: hz('C3'), length: 0.5, level: dB(-28), pan, curve: 0.3, vibrato: 0.03 });
        chip.note(at + 0.95, { wave: 'triangle', pitch: hz('G3'), to: hz('C3'), length: 0.12, level: dB(-18), pan });
      },
    },
    remind: { play: remindSound },
  };

  /** Sounds that are only the room getting on with its day: no more than one of them in this many seconds. */
  const BACKGROUND_GAP = 4;
  /** A rally's hits are heard for its first few: after that, the players carry on in silence. */
  const RALLY_HEARD = 12;
  /**
   * The sound each corner piece makes while someone's at it, if any: its
   * own, or one it shares. The coffee bar has the coffee machine's.
   */
  const CORNER_SOUNDS = {
    arcade: 'arcade', foosball: 'foosball', fishtank: 'fishtank', jellyfish: 'fishtank', dragon: 'dragon', pinball: 'pinball',
    jukebox: 'jukebox', pool: 'pool', piano: 'piano', drums: 'drums', telescope: 'twinkle', jungle: 'bird', coffee: 'coffee',
    servers: 'hum', printer: 'printer', boxing: 'boxing', treadmill: 'treadmill', hottub: 'splash', ballpit: 'rustle', campfire: 'crackle',
    bubbler: 'jukebox', neonbox: 'jukebox', wallbox: 'jukebox', djtable: 'jukebox', turntable: 'jukebox', boombox: 'jukebox',
    hifi: 'jukebox', synth: 'piano',
  };
  /** How often the weather in the windows, or the corner piece being played, makes itself heard, in seconds. */
  const WEATHER_EVERY = [50, 120];
  const CORNER_EVERY = [6, 14];
  /** The share of a pet's settlings that it makes a sound for. */
  const PET_CHATTY = 0.35;

  /**
   * The office's sound, in the style of an old console: the door, the coffee
   * machine, the ping-pong ball, an agent getting to work on their keyboard,
   * the pets and the seasons, a fanfare for a long turn done, and the red
   * button's blast and siren, with silence in between. All of it is made on
   * the spot, with nothing to download. Browsers only let a page play sound once
   * it's been tapped, so it wakes with the first tap, and it sleeps while the
   * office is out of sight.
   */
  class Ambience {
    constructor() {
      this.audio = null;      // the sound's own context, once a tap allows it
      this.chip = null;       // what it plays its sounds with
      this.master = null;
      this.since = null;      // when it started following the room, on the room's clock
      this.next = new Map();  // when each sound may next play, on the sound's clock
      this.quietUntil = 0;    // when the next of the room's background sounds may play
      this.nextWeather = null;
      this.nextCorner = null;
      this.rally = { players: null, bounce: null, table: null, hits: 0 };
      this.blast = null;      // the red button's last press, heard
      this.petSettled = new WeakMap(); // what's been heard of each pet: { settled, set, zoom, spat, first }
      this.mic = null;        // when the mic was last held and let go, heard
      this.carried = null;    // where the agent being carried is, as it's heard
      this.heard = new WeakMap(); // what's been played for each person: { arrived, celebrated, fired, out, working }
    }

    /**
     * From a tap: starts the sound, or carries it on. Asleep, or cut off by
     * a call on iOS ('interrupted'), it resumes.
     */
    wake(win, level) {
      if (!level) return;
      if (this.audio) {
        if (this.audio.state !== 'running' && this.audio.state !== 'closed') this.audio.resume().catch(() => {});
        return;
      }
      const Context = win.AudioContext || win.webkitAudioContext;
      if (!Context) return;
      // On iOS, mixed in with whatever else is playing rather than stopping it, and quiet on the silent switch.
      try { if (win.navigator.audioSession) win.navigator.audioSession.type = 'ambient'; } catch {}
      const audio = new Context();
      this.audio = audio;
      // Everything goes through a gentle low-pass, which takes the edge off
      // the pulses, then the volume, then a limiter that only a pile-up reaches.
      const soft = audio.createBiquadFilter();
      soft.type = 'lowpass';
      soft.frequency.value = 7000;
      soft.Q.value = 0;
      this.master = audio.createGain();
      this.master.gain.value = level;
      const limiter = audio.createDynamicsCompressor();
      limiter.threshold.value = -6;
      limiter.knee.value = 3;
      limiter.ratio.value = 12;
      limiter.attack.value = 0.003;
      limiter.release.value = 0.25;
      soft.connect(this.master).connect(limiter).connect(audio.destination);
      this.chip = new Chip(audio, soft);
    }

    sleep() {
      if (this.audio && this.audio.state === 'running') this.audio.suspend().catch(() => {});
    }

    /**
     * A sound by name, from `x` across the room, or the middle, `after` so
     * many seconds. It doesn't play when the office is quiet, when there's no
     * such sound, or when it played too recently. Says whether it played.
     */
    play(name, x = null, after = 0) {
      const sound = SOUNDS[name];
      if (!sound || !this.chip || this.audio.state !== 'running') return false;
      const t = this.audio.currentTime;
      if ((this.next.get(name) || 0) > t) return false;
      this.next.set(name, t + (sound.gap || 0));
      sound.play(this.chip, t + 0.02 + after, x === null ? 0 : panAt(x));
      return true;
    }

    /** One of the room's background sounds, if none has played just before it. */
    murmur(name, x) {
      if (this.audio.currentTime < this.quietUntil || !this.play(name, x)) return false;
      this.quietUntil = this.audio.currentTime + BACKGROUND_GAP;
      return true;
    }

    /**
     * Once a frame: the volume as set up, and a sound for whatever has just
     * happened in the room, or been cued by it. `held` is what a finger is
     * doing on it: the mic it holds, and the agent it carries.
     */
    follow(office, now, level, held = {}) {
      const cues = office.cues.splice(0);
      const audio = this.audio;
      if (!audio || audio.state !== 'running') return;
      // Turned off, it sleeps rather than play on at nothing; turned back on, a tap wakes it.
      if (!level) return this.sleep();
      this.master.gain.setTargetAtTime(level, audio.currentTime, 0.2);
      const t = audio.currentTime;
      // Only what happens from now on is heard: everyone already there when it starts is taken as read.
      const settling = this.since === null || now - this.since < 1;
      if (this.since === null) this.since = now;
      for (const cue of cues) this.play(cue.name, cue.x);

      // The ping-pong ball, off each paddle and the table between, for the start of each rally
      const players = PING_PONG.map((spot) => office.people.find((p) => p.place === spotPlace(spot) && !p.isWalking && !p.isLeaving));
      const rally = this.rally;
      if (!players.every(Boolean)) Object.assign(rally, { players: null, bounce: null, table: null, hits: 0 });
      else {
        if (!rally.players || rally.players.some((p, k) => p !== players[k])) Object.assign(rally, { players, hits: 0 });
        // As the room draws it: at the left paddle on each even count, the right on each odd, the table halfway.
        const bounce = Math.floor(now * 0.9), table = Math.floor(now * 1.8);
        if (rally.hits < RALLY_HEARD && rally.bounce !== null) {
          if (bounce !== rally.bounce) {
            this.play('paddle', players[bounce % 2].position.x);
            rally.hits++;
          } else if (table !== rally.table && table % 2) this.play('table', (players[0].position.x + players[1].position.x) / 2);
        }
        Object.assign(rally, { bounce, table });
      }

      // The red button going down; a red panda, if there is one, startled by it
      const blast = office.detonation && office.detonation.at;
      if (blast && blast !== this.blast && !settling) {
        this.play('blast', pointOf('button').x);
        const panda = office.pets.find((p) => p.isIn && p.kind === 'redpanda');
        if (panda) this.play('redpandaStartle', panda.position.x, 0.35);
      }
      this.blast = blast || this.blast;

      for (const person of office.people) {
        const x = person.position.x;
        let heard = this.heard.get(person);
        if (!heard) {
          heard = { arrived: person.arrivedAt, celebrated: person.celebratedAt, fired: !!person.isFired, out: false, working: person.isAtDesk && person.session.state === 'thinking' };
          this.heard.set(person, heard);
          // Someone new, coming in at the door: an agent hired, or staff
          const atDoor = Math.hypot(x - DOOR.x, person.position.y - DOOR.y) < 14;
          if (!settling && person.isWalking && atDoor) {
            this.play('door', DOOR.x);
            if (!person.isStaff) this.play('hire', x);
          }
          continue;
        }
        // Out through the door
        if (person.isLeaving && !heard.out && Math.hypot(x - DOOR.x, person.position.y - DOOR.y) < 10) {
          heard.out = true;
          this.play('door', DOOR.x);
        }
        // Getting to work, once a turn: a turn waiting on you isn't over yet
        const state = person.session.state;
        if (state === 'idle' || state === 'unread') heard.working = false;
        else if (state === 'thinking' && person.isAtDesk && !heard.working) {
          heard.working = true;
          this.play('typing', x);
        }
        if (person.isFired && !heard.fired) {
          heard.fired = true;
          this.play('fire', x);
        }
        // Arrived somewhere: the coffee machine, or the vending machine
        if (!person.isWalking && heard.arrived !== person.arrivedAt) {
          heard.arrived = person.arrivedAt;
          const spot = spotOf(person.place);
          if (spot === 0) this.play('coffee', x);
          if (spot !== null && spots[spot].kind === 'vending' && spots[spot].first) this.play('vending', x);
        }
        // A long turn finished
        if (person.celebratedAt !== null && heard.celebrated !== person.celebratedAt) {
          heard.celebrated = person.celebratedAt;
          this.play('fanfare', x);
        }
      }

      // The corner piece, now and then, while someone's at it
      const cornering = office.people.find((p) => !p.isWalking && !p.isLeaving && spotOf(p.place) >= LOUNGE_SPOTS.length);
      // The jukebox keeps quiet while the computer's own music plays.
      const cornerKind = cornering && spots[spotOf(cornering.place)].kind;
      const cornerSound = cornering && !(held.music && cornerKind === 'jukebox') && CORNER_SOUNDS[cornerKind];
      if (!cornering || !cornerSound) this.nextCorner = null;
      else if (this.nextCorner === null) this.nextCorner = t + random(1, 4);
      else if (t >= this.nextCorner) this.nextCorner = t + (this.murmur(cornerSound, cornering.position.x) ? random(...CORNER_EVERY) : 1);

      // Each pet: its habits as they happen; now and then as it settles somewhere new, or sets off; the owl only at night
      const hour = new Date().getHours(), night = decor.sky !== 'day' && (hour >= 20 || hour < 5);
      const fired = office.people.find((p) => p.isFired && p.isLeaving);
      for (const pet of office.pets) {
        if (!pet.isIn) continue;
        const x = pet.position.x;
        let heard = this.petSettled.get(pet);
        if (!heard) this.petSettled.set(pet, heard = { settled: pet.arrivedAt, set: pet.isWalking ? pet.arrivedAt : null, zoom: pet.zoom && pet.zoom.at, spat: fired || null, first: true });
        if (pet.kind === 'llama' && fired && heard.spat !== fired && Math.hypot(fired.position.x - x, fired.position.y - pet.position.y) < 120) this.play('llamaSpit', x, 0.2);
        heard.spat = fired || heard.spat;
        if (pet.kind === 'corgi' && pet.zoom && heard.zoom !== pet.zoom.at) this.play('corgiLap', x);
        heard.zoom = pet.zoom && pet.zoom.at;
        // The first pets' habits, as they happen
        if (pet.tookCupAt !== undefined && heard.cup !== pet.tookCupAt && heard.cup !== undefined) this.play('monkeyCup', x);
        heard.cup = pet.tookCupAt ?? null;
        if (pet.kind === 'dog' && pet.followAt !== undefined && heard.follow !== pet.followAt && heard.follow !== undefined) this.murmur('dogFollow', x);
        heard.follow = pet.followAt ?? null;
        if (pet.frozeAt !== undefined && heard.froze !== pet.frozeAt && heard.froze !== undefined) this.murmur('lizardFreeze', x);
        heard.froze = pet.frozeAt ?? null;
        if (pet.kind === 'axolotl' && Math.random() < 1 / (60 * 45)) this.murmur('axolotl', x);
        if (pet.isWalking) {
          // Setting off: the penguin on its belly, the hamster rolling
          if (heard.set !== pet.arrivedAt) {
            if (heard.set !== null && Math.random() < PET_CHATTY) {
              if (pet.kind === 'penguin' && pet.leg > 50) this.murmur('penguinSlide', x);
              else if (pet.kind === 'hamster') this.murmur('hamsterRoll', x);
            }
            heard.set = pet.arrivedAt;
          }
          continue;
        }
        if (pet.arrivedAt === heard.settled) continue;
        heard.settled = pet.arrivedAt;
        if (heard.first) { heard.first = false; continue; }
        // The fox lands its pounce every time; the rest now and then
        if (pet.kind === 'fox' && pet.target && pet.target.pose !== 'curl') { this.play('foxPounce', x); continue; }
        if (Math.random() >= PET_CHATTY) continue;
        const pose = pet.target && pet.target.pose;
        const name = pet.kind === 'robodog' && pose === 'charge' ? 'robodogCharge' : pet.kind === 'cat' && pose === 'sleep' ? 'catPurr' : pet.kind;
        if (pet.kind === 'owl' && !night) continue;
        this.murmur(name, x);
      }

      // The season in the windows, once in a while; autumn's leaves fall without a sound
      const weather = decor.season === 'calendar' ? { winter: 'wind', spring: 'bird', summer: 'bird' }[SEASONS[new Date().getMonth()]] : null;
      if (!weather) this.nextWeather = null;
      else if (this.nextWeather === null) this.nextWeather = t + random(20, 60);
      else if (t >= this.nextWeather) this.nextWeather = t + (this.murmur(weather) ? random(...WEATHER_EVERY) : 2);

      // The mic switched on and off, and an agent picked up and put down
      // A quick tap can hold and let go of the mic between two frames: each is told by its time.
      const { heldAt = null, releasedAt = null } = held.mic || {};
      if (this.mic && heldAt !== this.mic.heldAt && heldAt !== null) this.play(MIC_KINDS[decor.mic].sound);
      if (this.mic && releasedAt !== this.mic.releasedAt && releasedAt !== null) this.play('mic-off', null, 0.12);
      this.mic = { heldAt, releasedAt };
      if (held.carrying && this.carried === null) this.play('pick-up', held.carrying.x);
      if (!held.carrying && this.carried !== null) this.play('put-down', this.carried);
      this.carried = held.carrying ? held.carrying.x : null;
    }
  }

  /* --------------------------------------------------------- on the page */

  /** At most 30 frames a second: the art moves in whole pixels, and more would only spend battery. */
  const FRAME_TIME = 1000 / 30;
  /**
   * Out of sight this long, the room starts over from the latest snapshot
   * when it's seen again, everyone in place: nobody replays a press of ☢ Max
   * or a walk out that happened while nobody was looking.
   */
  const STALE_AFTER = 2000;
  /**
   * How many of the latest snapshots the computer's clock is judged by. Each
   * is a little late, so the smallest gap between the clocks is the truest:
   * a snapshot held up on its way can only make it seem bigger.
   */
  const CLOCK_SAMPLES = 5;
  /** The numbers of keys a layout can fix; anything else is Auto, a key for each agent. */
  const FIXED_KEYS = [3, 6, 9];

  /**
   * The deck's snapshot, as the office reads it: a seat per key, its session
   * if it has one. Turns are timed on this device's clock, `ahead` of the
   * computer's by however much, so a computer set wrong doesn't make every
   * turn look an hour old.
   */
  function seatsOf(snapshot, ahead) {
    return (snapshot.keys || []).map((key) => ({
      pin: key.pin || null,
      session: key.session && {
        ...key.session,
        // Each project always gets the same face, whatever folder its agent is in.
        name: key.session.project || key.session.name || '',
        effort: key.session.effort || '',
        maxPressedAt: key.session.maxPressedAt ?? null,
        turnStartedAt: key.session.turnStartedAt ? key.session.turnStartedAt + ahead : null,
      },
    }));
  }

  /** Nothing to tap is smaller than this, in CSS pixels, so a finger finds it. */
  const SMALLEST_TARGET = 44;

  function create(canvas, on = {}) {
    const ctx = canvas.getContext('2d');
    const office = new Office();
    const clock = () => performance.now() / 1000;
    let visible = true;
    let framed = false;  // seen through a window, beside the phone's controls
    let floating = null;  // floating in a 'window' of its own or a whole 'screen', from 'edge' to edge, or not
    let frame = null;
    let last = 0;
    let selected = null; // the session the deck's controls act on
    let latest = null;   // the deck's last snapshot
    let ahead = 0;       // how far this device's clock is ahead of the computer's, in ms
    let gaps = [];       // how far ahead it seemed as each of the latest snapshots came, in ms
    // The mic: whether a finger holds it, when it last went on and off, and
    // who it talks to.
    const mic = { held: false, heldAt: null, releasedAt: null, listener: null, usable: true };
    let wobbling = null;     // the button or the mic, tapped with no desk picked: { what, at }
    let buttonDown = false;  // a finger on the nuclear button
    let choiceDown = null;   // the last choice key tapped: { action, at, held }
    let chipDown = null;     // the last paint chip tapped: { level, at, held }
    // A desk held down: { kind: 'session', id } for its agent, or { kind: 'empty', desk } for a hire
    let held = null;
    let firing = null;       // an agent about to be fired, while the question is up: { id, name, at }
    let carrying = null;     // an agent carried to another desk: { person, from, x, y, over, desks }
    let styling = false;     // the styles' panel is up
    let music = null;        // what the computer says of its music, with the jukebox on: { playing: true | false | null }
    let musicTap = null;     // a music piece just tapped: { part, at, was }, for a moment's ▶ or ❚❚ over it
    let musicPanel = null;   // a music piece's control, held open: { part, at }, `at` when it was last used
    let musicDown = null;    // the music control's key last pressed: { action, at, held }
    let keysFor = null;      // the agent whose keys are out, by its session's id: from a finger held on them till a tap elsewhere
    let keyArmed = null;     // a key that wants a second tap, tapped once: { action, at }
    let keyDown = null;      // the key last pressed: { action, at, held }
    const returnAt = new Map(); // when accept or send last went to each session, so a tap landing twice sends Return once
    let keysAt = null;       // where the keys were last drawn, in art pixels: { keys, box }
    let roleDown = null;     // the role last tapped, on a held empty desk: { id, at, held }
    let roleOffset = 0;      // how far the roles are slid along, in art pixels
    let rolesAt = null;      // where the roles were last drawn, in art pixels: { roles, box, most }
    // Two fingers on the room scroll the picked agent's Terminal: each finger by its id, where it is, in CSS pixels
    const touches = new Map();
    let scrolling = null;    // { y, at, moved, carry }, while two fingers are down
    // Arranging the room, on the setup page: the piece picked, and the one being dragged.
    let arranging = null;    // { picked, drag: { part, x, y, from } }
    let styleKeyAt = null;   // where the styles' key was last drawn, in art pixels
    const stylePictures = new Map(); // a picture of each style, by its id, drawn as the panel opens
    let hiring = { fire: false, hire: false }; // what the computer lets a held desk offer
    const ambience = new Ambience();
    const loudness = () => SOUND_LEVELS[decor.sound];
    let finger = null;       // the one finger on the room: { id }
    // The window the canvas is in, its page's or one it has moved to: the
    // room draws with that window's frames and pixels, and pauses only when
    // it's hidden. The page can be out of sight while that window is on top.
    let host = null;         // { window, resizing, seen }

    // The canvas holds the screen's own pixels, so the art can be scaled by
    // a whole number of them.
    function resize() {
      const ratio = host.window.devicePixelRatio || 1;
      const rect = canvas.getBoundingClientRect();
      canvas.width = Math.max(1, Math.round(rect.width * ratio));
      canvas.height = Math.max(1, Math.round(rect.height * ratio));
      ctx.imageSmoothingEnabled = false;
    }

    /** Follows the window the canvas is in now: its size, and whether it can be seen. */
    function bind() {
      const win = canvas.ownerDocument.defaultView || window;
      if (host && host.window === win) return;
      unbind();
      const resizing = new win.ResizeObserver(resize);
      resizing.observe(canvas);
      const seen = () => {
        if (win.document.hidden && finger) lift();
        if (win.document.hidden) ambience.sleep();
        else if (visible) ambience.wake(win, loudness()); // seen again: once tapped, it may carry on
        run();
      };
      // Any tap on the page, not only on the room, may start the sound.
      const tapped = () => { if (visible) ambience.wake(win, loudness()); };
      win.document.addEventListener('visibilitychange', seen);
      win.document.addEventListener('pointerdown', tapped, { capture: true });
      host = { window: win, resizing, seen, tapped };
    }

    function unbind() {
      if (!host) return;
      if (frame !== null) host.window.cancelAnimationFrame(frame);
      frame = null;
      host.resizing.disconnect();
      host.window.document.removeEventListener('visibilitychange', host.seen);
      host.window.document.removeEventListener('pointerdown', host.tapped, { capture: true });
      host = null;
    }

    function selectedSession() {
      const seat = office.seats.find((s) => s.session && s.session.id === selected);
      return seat ? seat.session : null;
    }

    /** The paint chips, once held out on the picked desk of an agent with an effort setting, while the dial is out of sight. */
    function chips() {
      if (framed || !held || held.kind !== 'session' || held.id !== selected) return null;
      const desk = office.seats.findIndex((s) => s.session && s.session.id === selected);
      return desk >= 0 && office.seats[desk].session.dial ? effortChips(desk) : null;
    }

    /**
     * Whether a hire on `desk` would sit at it: a pinned desk with nothing
     * running, whose project the new agent starts in, or the one desk an
     * agent hired into no project lands on. That's the first free key's;
     * with none free, Auto adds a key after the last, while there's room
     * for one, and a fixed number of keys has nowhere to put them.
     */
    function hiresInto(desk) {
      if (framed || !hiring.hire) return false;
      const seat = office.seats[desk];
      if (seat && seat.pin) return !seat.session;
      const free = office.seats.findIndex((s) => !s.pin && !s.session);
      if (free >= 0) return desk === free;
      const fixed = FIXED_KEYS.includes(latest && latest.layout && latest.layout.keys);
      return !fixed && desk === office.seats.length && desk < DESKS.length;
    }

    /** The roles the computer can hire into, as you've set them up: none, and a hire is anyone. */
    const roles = () => (Array.isArray(hiring.roles) ? hiring.roles.filter((r) => r && typeof r.id === 'string' && r.name) : []);

    /** The roles a held empty desk can hire into, along the foot of the room, Anyone first, if there are any to choose from. */
    function roleRow() {
      if (framed || !held || held.kind !== 'empty' || !hiresInto(held.desk) || !roles().length || arranging || styling || firing || carrying) return null;
      return { desk: held.desk, roles: [{ id: null, name: 'Anyone' }, ...roles()], down: roleDown, offset: roleOffset };
    }

    /** The button on a held desk, if the computer allows it: fire its agent, or hire one into it, where there are no roles to choose from. */
    function heldButton() {
      if (framed || !held) return null;
      if (held.kind === 'empty') return hiresInto(held.desk) && !roles().length ? deskButton(held.desk, 'hire') : null;
      if (held.id !== selected || !hiring.fire) return null;
      const desk = office.seats.findIndex((s) => s.session && s.session.id === held.id);
      return desk >= 0 ? deskButton(desk, 'fire') : null;
    }

    /** The styles' key, where the deck's own controls are out of sight and the page can change the style. */
    function styleKey() {
      if (framed || !on.style || !latest || arranging) return null;
      return { open: styling };
    }

    /** A picture of each style, in the room as you furnished it, for the panel: drawn as it opens. */
    function drawStylePictures() {
      const room = latest && latest.layout && latest.layout.room;
      for (const id of STYLE_IDS) {
        const picture = stylePictures.get(id) || canvas.ownerDocument.createElement('canvas');
        picture.width = STYLE_THUMB.w;
        picture.height = STYLE_THUMB.h;
        preview(picture, { preset: id }, { room, now: 3.3 });
        stylePictures.set(id, picture);
      }
    }

    /** The choice keys, while the picked agent waits on you and the deck's own controls are out of sight. */
    /** The music control, over its piece and kept in the room: its keys, and its tip pointing at the piece. */
    function musicControl() {
      if (!musicPanel || !music || !on.music || !loungePieces().some(({ part }) => part === musicPanel.part)) return null;
      const box = pieceBox(musicPanel.part);
      const w = 2 + MUSIC_KEY.w * 3 + MUSIC_KEY.gap * 2 + 2, h = MUSIC_KEY.h + 4;
      const tip = { x: Math.round((box.left + box.right) / 2), y: Math.round(box.top) - 2 };
      const left = Math.max(1, Math.min(shape.width - w - 1, tip.x - Math.floor(w / 2))), top = Math.max(1, tip.y - h - 2);
      tip.x = Math.max(left + 2, Math.min(left + w - 3, tip.x));
      return { left, top, w, h, tip, keys: MUSIC_ACTIONS.map((action, i) => ({ action, x: left + 2 + i * (MUSIC_KEY.w + MUSIC_KEY.gap), y: top + 2 })) };
    }

    /** The picked agent's keys, along the foot of the room, if a finger held on them put them out and nothing else is up in their place. */
    function keyRow() {
      if (framed || !keysFor || keysFor !== selected || !on.key || arranging || styling || firing || carrying || !selectedSession()) return null;
      return { keys: rowKeysOf(latest && latest.layout), armed: keyArmed, down: keyDown };
    }

    function choices() {
      if (framed) return null;
      const desk = office.seats.findIndex((s) => s.session && s.session.id === selected);
      return desk >= 0 && office.seats[desk].session.state === 'asking' ? choicePanel(desk) : null;
    }

    function tick() {
      frame = null;
      if (!visible || host.window.document.hidden) return;
      frame = host.window.requestAnimationFrame(tick);
      // The page's clock, not the frame's: each window counts its frames'
      // time from when it opened, and the room can move between them.
      const time = performance.now();
      if (time - last < FRAME_TIME - 1) return;
      if (last && time - last > STALE_AFTER && latest) {
        office.close();
        office.apply(seatsOf(latest, ahead), latest.theme, time / 1000);
      }
      last = time;
      const now = time / 1000, date = new Date();
      // Dimmed only for a picked session with no voice input: with none
      // picked, a tap wobbles it and asks for a desk.
      const session = selectedSession();
      mic.usable = !session || !!session.mic;
      // A finger held on a desk, or its person, puts out its chips and
      // button; on a desk a hire would sit at, the button to hire.
      // Held on a music piece, its control opens; left unused a while, it goes away again.
      if (finger && !finger.held && finger.target.kind === 'music' && now - finger.at >= HOLD_FOR_CHIPS) {
        finger.held = true;
        musicPanel = { part: finger.target.part, at: now };
      }
      if (musicPanel && now - musicPanel.at > MUSIC_PANEL_LENGTH && !(musicDown && musicDown.held)) musicPanel = null;
      const holding = finger && !finger.held && ['session', 'pinned', 'empty'].includes(finger.target.kind);
      if (holding && now - finger.at >= HOLD_FOR_CHIPS) {
        finger.held = true;
        const { kind, id, desk } = finger.target;
        held = kind === 'session' ? { kind, id } : hiresInto(desk) ? { kind: 'empty', desk } : null;
        roleOffset = 0;
        // An agent held, at a desk or in the line, puts out their keys too, whether or not they have chips.
        if (kind === 'session') keysFor = id;
        if (chips() || heldButton() || keyRow() || roleRow()) on.held?.();
      }
      // The room takes its canvas's shape: tall on a screen held upright, where it comes out bigger.
      const fitting = canvas.height > canvas.width ? TALL : WIDE;
      if (fitting !== shape) office.reshape(fitting, now);
      office.step(now);
      ambience.follow(office, now, loudness(), { mic, carrying, music: music && music.playing === true });
      const key = styleKey();
      if (!key) styling = false;
      const keys = keyRow(), hires = roleRow();
      drawOffice(ctx, office, {
        now, hour: date.getHours(), minute: date.getMinutes(), month: date.getMonth(),
        width: canvas.width, height: canvas.height, ratio: host.window.devicePixelRatio || 1,
        selected, mic, wobble: wobbling, buttonDown, framed, floating, choices: choices(), choiceDown, chips: chips(), chipDown,
        deskButton: heldButton(), firing: firing && { ...fireDialog(), name: firing.name }, carrying,
        styleKey: key, styles: styling && key && { ...styleMenu(), current: decor.preset, pictures: stylePictures },
        arranging: arranging && {
          pieces: arrangeable(), picked: arranging.picked, blocked: arranging.blocked || [],
          // The picked piece's handle, if it can grow
          handle: arranging.picked && !isHung(arranging.picked) && (isAdded(arranging.picked) || ARRANGEABLE[arranging.picked]?.grows)
            ? pieceBox(arranging.picked) : null,
        },
        music,
        // Gone with its piece, if the room changed under it, rather than failing every frame until it would have
        musicTap: musicTap && now - musicTap.at < MUSIC_TAP_LENGTH && pieceBox(musicTap.part)
          && { box: pieceBox(musicTap.part), was: musicTap.was, since: now - musicTap.at },
        musicPanel: musicControl() && { ...musicControl(), playing: music.playing, down: musicDown },
        keyRow: keys,
        roleRow: hires,
      });
      styleKeyAt = key && key.at;
      keysAt = keys && keys.at ? { keys: keys.at, box: keys.box } : null;
      rolesAt = hires && hires.at ? { roles: hires.at, box: hires.box, most: hires.most } : null;
    }

    function run() {
      if (frame === null && visible && !host.window.document.hidden) frame = host.window.requestAnimationFrame(tick);
    }

    /**
     * What's under a point on the page, if it can be tapped. Each target
     * reaches at least SMALLEST_TARGET round its middle; of those in reach,
     * the nearest wins, and on a tie choice keys come before people, people
     * before things, and things before desks. The choice keys' panel covers
     * what's under it: a tap on it goes to a key or nowhere.
     */
    /** A point on the page, in the room's art pixels, with the scale the room is drawn at. */
    function roomAt(clientX, clientY) {
      const rect = canvas.getBoundingClientRect();
      const ratio = host.window.devicePixelRatio || 1;
      const { scale, ox, oy } = fitRoom(canvas.width, canvas.height, floating ? FLOAT_MARGINS[floating] * ratio : 0);
      return {
        x: ((clientX - rect.left) * (canvas.width / rect.width) - ox) / scale,
        y: ((clientY - rect.top) * (canvas.height / rect.height) - oy) / scale,
        scale, ratio,
      };
    }

    /**
     * Where a key's desk is, to a finger: its workspace, as high as that
     * reaches, down to the desk's front; and its label below. A tall one
     * reaches up into the label of the desk behind it, where it wins.
     */
    function deskBoxes(desk) {
      const { x, y } = DESKS[desk], seat = office.seats[desk] || {};
      const left = x - 2, right = x + DESK_WIDTH + 2;
      return [
        { left, right, top: y + WORKSPACES[workspaceOf(seat.session, desk)].top, bottom: y + 18 },
        { left, right, top: y + 18, bottom: y + 27 },
      ];
    }

    /** The key's desk under a point of the room, if there is one: where a carried agent would land. */
    function deskAt(x, y) {
      const within = (box) => x >= box.left && x <= box.right && y >= box.top && y <= box.bottom;
      const boxes = DESKS.slice(0, office.seats.length).map((d, desk) => (d.off ? null : deskBoxes(desk)));
      let desk = boxes.findIndex((box) => box && within(box[0]));
      if (desk < 0) desk = boxes.findIndex((box) => box && within(box[1]));
      return desk >= 0 ? desk : null;
    }

    function targetAt(clientX, clientY) {
      const { x, y, scale, ratio } = roomAt(clientX, clientY);
      const reach = (SMALLEST_TARGET * ratio) / scale;
      const now = clock();
      let best = null;
      const consider = (target, box, rank) => {
        const grow = (low, high) => {
          const extra = Math.max(0, reach - (high - low)) / 2;
          return [low - extra, high + extra];
        };
        const [left, right] = grow(box.left, box.right), [top, bottom] = grow(box.top, box.bottom);
        if (x < left || x > right || y < top || y > bottom) return;
        const distance = Math.hypot(Math.max(box.left - x, 0, x - box.right), Math.max(box.top - y, 0, y - box.bottom));
        if (!best || distance < best.distance || (distance === best.distance && rank < best.rank)) best = { target, distance, rank };
      };

      // While the styles are up, they're all there is; a tap anywhere else puts them away.
      if (styling && styleKey()) {
        for (const tile of styleMenu().tiles) {
          consider({ kind: 'style', id: tile.id }, { left: tile.x, right: tile.x + tile.w, top: tile.y, bottom: tile.y + tile.h }, -1);
        }
        return best ? best.target : { kind: 'unstyle' };
      }
      // While the question is up, its keys are all there is; a tap anywhere else keeps them on.
      if (firing) {
        if (clock() - firing.at < FIRE_QUESTION_SETTLES) return null;
        for (const key of fireDialog().keys) consider({ kind: key.action }, { left: key.x, right: key.x + key.w, top: key.y, bottom: key.y + key.h }, -1);
        return best ? best.target : { kind: 'keep' };
      }

      for (const person of office.people) {
        if (person.isStaff || person.isLeaving || person.entering) continue;
        const px = Math.round(person.position.x), top = topOf(person, poseOf(person, now));
        // The sprite, its arms, and the bubble over its head
        consider({ kind: 'session', id: person.session.id },
          { left: px - 5, right: px + 8, top: top - 11, bottom: Math.round(person.position.y) + 1 }, 0);
      }
      // A music piece, with the jukebox on: play or pause
      if (music && on.music) {
        for (const { part, kind } of loungePieces()) if (MUSIC_KINDS.includes(kind)) consider({ kind: 'music', part }, pieceBox(part), 1);
      }
      if (styleKey() && styleKeyAt) {
        const { x: kx, y: ky, w: kw, h: kh } = styleKeyAt;
        consider({ kind: 'styles' }, { left: kx, right: kx + kw, top: ky, bottom: ky + kh }, -1);
      }
      // Each where it's arranged, as big as it's been made
      const box = (part, left, top, right, bottom) => {
        const a = placed(part, { x: left, y: top }), b = placed(part, { x: right, y: bottom });
        return { left: a.x, top: a.y, right: b.x, bottom: b.y };
      };
      consider({ kind: 'button' }, box('button', BUTTON.x - 7, BUTTON.y - 1, BUTTON.x + 8, BUTTON.y + 16), 1);
      const { tap } = MIC_KINDS[decor.mic];
      consider({ kind: 'mic' }, box('mic', MIC.x + tap.left, MIC.y + tap.top, MIC.x + tap.right, MIC.y + tap.bottom), 1);
      DESKS.forEach((d, desk) => {
        // A desk put away isn't there to tap: its agent, in the line, is.
        if (d.off) return;
        const seat = office.seats[desk] || {};
        // An empty desk only matters where a hire would sit.
        const target = seat.session ? { kind: 'session', id: seat.session.id }
          : seat.pin ? { kind: 'pinned', name: seat.pin, desk }
          : hiresInto(desk) ? { kind: 'empty', desk } : null;
        // The desk, its chair and monitor, and below them its label
        if (!target) return;
        const [workspace, label] = deskBoxes(desk);
        consider(target, workspace, 2);
        consider(target, label, 3);
      });
      const button = heldButton();
      if (button) {
        consider({ kind: button.kind }, { left: button.left, right: button.left + button.w, top: button.top, bottom: button.top + button.h }, -1);
        const onButton = x >= button.left - 1 && x <= button.left + button.w + 1 && y >= button.top - 1 && y <= button.top + button.h + 1;
        if (onButton) return best && best.target.kind === button.kind ? best.target : null;
      }
      const strip = chips();
      if (strip) {
        for (const chip of strip.chips) {
          consider({ kind: 'chip', level: chip.level }, { left: chip.x, right: chip.x + CHIP.w, top: chip.y, bottom: chip.y + CHIP.h }, -1);
        }
        // A tap on the strip goes to a chip or nowhere, never to what's under it.
        const onStrip = x >= strip.left - 1 && x <= strip.left + strip.w + 1 && y >= strip.top - 1 && y <= strip.top + strip.h + 2;
        if (onStrip) return best && best.target.kind === 'chip' ? best.target : null;
      }
      // The picked agent's keys cover what's under them: a tap on their strip goes to a key or nowhere.
      if (keysAt && keyRow()) {
        for (const k of keysAt.keys) consider({ kind: 'key', action: k.action }, k, -1);
        const { box } = keysAt;
        if (x >= box.left && x <= box.right && y >= box.top && y <= box.bottom) return best && best.target.kind === 'key' ? best.target : null;
      }
      // So do the roles a held empty desk offers: a finger on their strip is on a role, or slides them along.
      if (rolesAt && roleRow()) {
        for (const r of rolesAt.roles) consider({ kind: 'role', id: r.id, chip: true }, r, -1);
        const { box } = rolesAt;
        if (x >= box.left && x <= box.right && y >= box.top && y <= box.bottom) return best && best.target.kind === 'role' ? best.target : { kind: 'role', chip: false };
      }
      // The music control covers what's under it: a tap on it goes to a key or nowhere.
      const control = musicControl();
      if (control) {
        for (const key of control.keys) {
          consider({ kind: 'musicKey', action: key.action }, { left: key.x, right: key.x + MUSIC_KEY.w, top: key.y, bottom: key.y + MUSIC_KEY.h }, -1);
        }
        const onControl = x >= control.left && x <= control.left + control.w && y >= control.top && y <= control.tip.y;
        if (onControl) return best && best.target.kind === 'musicKey' ? best.target : null;
      }
      const panel = choices();
      if (panel) {
        for (const key of panel.keys) {
          consider({ kind: 'choice', action: key.action },
            { left: key.x, right: key.x + CHOICE_KEY.w, top: key.y, bottom: key.y + CHOICE_KEY.h }, -1);
        }
        const onPanel = x >= panel.left && x <= panel.left + panel.w && y >= panel.top && y <= panel.tip.y;
        if (onPanel) return best && best.target.kind === 'choice' ? best.target : null;
      }
      return best && best.target;
    }

    /** Everything a key or button does on the deck, done by tapping the room. */
    function press(target) {
      // Anything tapped but the keys out and the chips puts the keys away; a finger held on an agent puts theirs out (see `tick`).
      if (target.kind !== 'key' && target.kind !== 'chip') keysFor = null;
      if (target.kind !== 'key') keyArmed = null;
      if (target.kind === 'key') {
        const wanted = rowKeysOf(latest && latest.layout).find((k) => k.action === target.action);
        // A key that wants to be sure acts on its second tap, while it says SURE?
        const arming = wanted && wanted.confirm && !(keyArmed && keyArmed.action === target.action && clock() - keyArmed.at < KEY_ARMED_LENGTH);
        if (!arming && isRepeatedReturn(target.action)) return; // ignored, so it isn't pressed in either
        keyDown = { action: target.action, at: clock(), held: true };
        if (arming) {
          keyArmed = { action: target.action, at: clock() };
          return on.held?.();
        }
        keyArmed = null;
        return on.key?.(target.action);
      }
      // A role hires once the finger lifts, if it hasn't slid them along instead (see `lift`).
      if (target.kind === 'role') {
        roleDown = target.chip ? { id: target.id, at: clock(), held: true } : null;
        return;
      }
      // Anything else tapped puts the held desk's chips and button away.
      const holding = held;
      if (target.kind !== 'chip') held = null;
      // Anything but its keys puts the music control away.
      if (target.kind !== 'musicKey') musicPanel = null;
      if (target.kind === 'keep') {
        firing = null;
        return;
      }
      if (target.kind === 'styles') {
        styling = !styling;
        if (styling) drawStylePictures();
        return;
      }
      // A music piece plays or pauses once the finger lifts, if it isn't held to open the control.
      if (target.kind === 'music') return;
      if (target.kind === 'musicKey') {
        musicDown = { action: target.action, at: clock(), held: true };
        if (musicPanel) musicPanel.at = clock();
        return on.music?.(target.action);
      }
      if (target.kind === 'unstyle') {
        styling = false;
        return;
      }
      if (target.kind === 'style') {
        styling = false;
        return on.style?.(target.id);
      }
      // They carry their things out in a box only once the computer says
      // they're fired; if it couldn't, the deck says why.
      if (target.kind === 'fired') {
        if (!firing) return; // they left while the finger was down
        const { id } = firing;
        firing = null;
        return Promise.resolve(on.fire?.(id)).then((fired) => {
          const person = office.people.find((p) => !p.isStaff && p.session.id === id);
          if (fired && person) person.isFired = true;
        });
      }
      if (target.kind === 'fire') {
        const seat = office.seats.find((s) => s.session && s.session.id === holding.id);
        // Asked about by the name on their desk: a nickname, or their project's.
        if (seat) firing = { id: holding.id, name: memberOf(seat.session)?.name || seat.session.name, at: clock() };
        return on.held?.();
      }
      if (target.kind === 'hire') return on.hire?.(holding.desk, null);
      // An empty or pinned desk says something on a tap, once the finger lifts: holding it is something else.
      if (target.kind === 'empty' || target.kind === 'pinned') return;
      if (target.kind === 'choice') {
        if (isRepeatedReturn(target.action)) return;
        choiceDown = { action: target.action, at: clock(), held: true };
        return on.choose?.(target.action);
      }
      // Max from a chip is the red button's: its agent runs to slam it.
      if (target.kind === 'chip') {
        chipDown = { level: target.level, at: clock(), held: true };
        return target.level === 'max' ? on.max?.() : on.effort?.(target.level);
      }
      if (target.kind === 'session') return on.select?.(target.id);
      const session = selectedSession();
      if (!session) {
        wobbling = { what: target.kind, at: clock() };
        return on.hint?.('Pick a desk first');
      }
      // As the deck's ☢ Max and 🎙: only for an agent with an effort setting,
      // or with voice input.
      if (target.kind === 'button' && session.dial) {
        buttonDown = true;
        on.max?.();
      } else if (target.kind === 'mic' && session.mic) {
        Object.assign(mic, { held: true, heldAt: clock(), releasedAt: null, listener: session.id });
        on.mic?.('down');
      }
    }

    /**
     * Accept or send to the picked session again a moment after the last: one
     * tap landing twice. A Yes to one agent then another both go, and up, down
     * and the rest repeat as tapped.
     */
    function isRepeatedReturn(action) {
      if (action !== 'accept' && action !== 'send') return false;
      const now = clock();
      for (const [id, at] of returnAt) if (now - at >= RETURN_REPEAT) returnAt.delete(id);
      if (returnAt.has(selected)) return true;
      returnAt.set(selected, now);
      return false;
    }

    /**
     * The finger lifts, or is taken away: the mic must always hear about it,
     * or it keeps listening. Lifted over another desk, a carried agent swaps
     * desks with it; taken away, they go back.
     */
    function lift(dropped = false) {
      // A touch waiting to send goes now if it lifted where it landed; taken away, or joined by a second finger, it sends nothing.
      if (finger && finger.waiting) {
        finger.waiting = false;
        if (dropped && !finger.moved) press(finger.target);
      }
      if (carrying) {
        const { from, over } = carrying;
        carrying = null;
        finger = null;
        if (dropped && over !== null && over !== from) on.swap?.(from, over);
        return;
      }
      // A quick tap on a music piece plays or pauses the computer's music; held, it opened the control instead.
      if (finger && finger.target.kind === 'music' && !finger.held && dropped) {
        musicTap = { part: finger.target.part, at: clock(), was: music ? music.playing : null };
        on.music?.('playpause');
      }
      if (musicDown) musicDown.held = false;
      if (keyDown) keyDown.held = false;
      // A chip, once chosen, goes away with the rest.
      if (finger && finger.target.kind === 'chip') held = null;
      // A role tapped, not slid along, hires into the held desk, and the roles go away.
      if (finger && finger.target.kind === 'role') {
        if (roleDown) roleDown.held = false;
        const { chip, id } = finger.target;
        if (dropped && chip && !finger.slid && held && held.kind === 'empty') {
          const { desk } = held;
          held = null;
          on.hire?.(desk, id);
        }
      }
      if (finger && !finger.held) {
        const { kind, name, desk } = finger.target;
        const hold = hiresInto(desk) ? 'Hold the desk to hire someone' : '';
        if (kind === 'pinned') on.hint?.(`Nothing running in ${name}${hold ? `. ${hold}` : ''}`);
        else if (kind === 'empty') on.hint?.(hold);
      }
      finger = null;
      buttonDown = false;
      if (choiceDown) choiceDown.held = false;
      if (chipDown) chipDown.held = false;
      if (!mic.held) return;
      mic.held = false;
      mic.releasedAt = clock();
      on.mic?.('up');
    }

    /* Arranging the room */

    const shapeName = () => (shape === TALL ? 'tall' : 'wide');
    /** How far down the back wall a piece hanging on it reaches, for picking it: from near the ceiling to above the floor. */
    const HUNG_SPAN = { top: 3, bottom: WALL_BOTTOM - 10 };
    /** How close to the picked piece's corner a finger counts as on its handle, in CSS pixels, and how big it's drawn, in art pixels. */
    const HANDLE_REACH = 14;
    const HANDLE_SIZE = 3;

    /** The part of the room `part` takes up now, in art pixels: a desk with its workspace, or a piece as grown and moved. */
    function pieceBox(part) {
      if (part.startsWith('desk')) {
        const desk = Number(part.slice(4)) - 1, { x, y } = DESKS[desk];
        const seat = office.seats[desk] || {};
        return { left: x - 2, top: y + WORKSPACES[workspaceOf(seat.session, desk)].top, right: x + DESK_WIDTH + 2, bottom: y + 20 };
      }
      if (isHung(part)) {
        const piece = hungOf()[Number(part.slice(5))];
        if (!piece) return null; // taken off the wall meanwhile
        const half = wallWidthOf(piece.kind) / 2;
        return { left: piece.x - half, top: HUNG_SPAN.top, right: piece.x + half, bottom: HUNG_SPAN.bottom };
      }
      const rect = pieceRect(part);
      if (!rect) return null; // taken out of the room meanwhile
      const { x, y, w, h } = rect;
      const a = placed(part, { x, y }), b = placed(part, { x: x + w, y: y + h });
      return { left: a.x, top: a.y, right: b.x, bottom: b.y };
    }

    /** What can be arranged and is in the room, the smallest first, so a piece on a bigger one is picked first. */
    function arrangeable() {
      const added = addedOf(decor).map((_, i) => `added:${i}`);
      const hung = hungOf().map((_, i) => `hung:${i}`).filter((part) => hungOf()[Number(part.slice(5))].kind !== 'bare');
      return [...Object.keys(ARRANGEABLE), ...added].filter((part) => !pieceOf(part).off && (part.startsWith('desk') || pieceRect(part)))
        .concat(hung)
        .map((part) => ({ part, box: pieceBox(part) }))
        .sort((a, b) => (a.box.right - a.box.left) * (a.box.bottom - a.box.top) - (b.box.right - b.box.left) * (b.box.bottom - b.box.top));
    }

    /** How far behind a desk its chair can be got to, in art pixels: the floor there has to be free. */
    const CHAIR_ROOM = 8;
    /**
     * The desks, and the red button, that nobody can walk to from the door,
     * as the room is arranged now: a desk with something right behind it,
     * where its chair is, or with no way there.
     */
    function blocked() {
      const behind = (desk) => {
        for (let dy = 2; dy <= CHAIR_ROOM; dy += CELL) {
          const at = { x: seat(desk).x, y: DESKS[desk].y - dy };
          if (standsFree(at)) return at;
        }
        return null;
      };
      const parts = DESKS.map((desk, i) => (desk.off ? null : { part: `desk${i + 1}`, at: behind(i) })).filter(Boolean);
      const presser = placed('button', PRESSER);
      parts.push({ part: 'button', at: standsFree(presser) ? presser : null });
      return parts.filter(({ at }) => !at || !walkway(DOOR, at, true)).map(({ part }) => part);
    }

    /** The room arranged as `plan` from now on: here at once, and then on the page, which keeps it in the layout. */
    function settle(plan) {
      decor = { ...decor, plan: planOf(plan) };
      spots = spotsOf(decor);
      // The whiteboard and the windows go where they're hung now, and so does whoever's at them.
      boardSpots = boardSpotsOf(decor);
      stormSpots = stormSpotsOf(decor);
      office.relayout(clock());
      floorChanged();
      if (arranging) arranging.blocked = blocked();
      on.arranged?.(decor.plan);
    }

    /** The back wall as you hung it in the room's shape now, written out from its five places the first time it's changed. */
    function hungList(plan) {
      const side = shapeName();
      if (!Array.isArray(plan.hung[side])) plan.hung[side] = hungOf({ ...decor, preset: 'yours' }).filter((piece) => piece.kind !== 'bare');
      // In a style, the wall as it's seen, the style's pieces in your places: what's changed is what you see
      else if (ROOM_STYLES[decor.preset]) plan.hung[side] = hungOf().map((piece) => ({ ...piece }));
      return plan.hung[side];
    }

    /** Where along the wall a piece of `kind` can hang at `x`, nearest it, clear of `others`: or null where there's no room. */
    function hangAt(kind, x, others) {
      const [from, to] = WALL_RANGE[shapeName()], half = wallWidthOf(kind) / 2;
      const fits = (at) => at - half >= from && at + half <= to
        && others.every((other) => Math.abs(other.x - at) >= half + wallWidthOf(other.kind) / 2);
      for (let d = 0; d <= to - from; d++) {
        for (const at of [Math.round(x) - d, Math.round(x) + d]) if (fits(at)) return at;
      }
      return null;
    }

    /**
     * `part` arranged anew in the room's shape now, kept inside the room:
     * moved, grown, put away, back, or, for a piece added or hung, taken out.
     */
    function rearrange(part, change) {
      const plan = structuredClone(decor.plan);
      const side = shapeName();
      if (isHung(part)) {
        // Found by what it is and where, as the wall's five places, written out, leave out the bare ones
        const was = hungOf()[Number(part.slice(5))], list = hungList(plan);
        const i = list.findIndex((piece) => piece.kind === was.kind && piece.x === was.x);
        if (i < 0) return settle(plan);
        if (change.remove || change.off) list.splice(i, 1);
        else if (change.x !== undefined) {
          const at = hangAt(list[i].kind, change.x, list.filter((_, j) => j !== i));
          if (at !== null) list[i] = { ...list[i], x: at };
        }
        // What was picked or carried is called by its place in the list now
        if (arranging && !(change.remove || change.off)) {
          if (arranging.picked === part) arranging.picked = `hung:${i}`;
          if (arranging.drag && arranging.drag.part === part) arranging.drag.part = `hung:${i}`;
        }
        settle(plan);
        if (arranging && arranging.picked !== part && !(change.remove || change.off)) on.picked?.(arranging.picked);
        return;
      }
      let piece;
      if (isAdded(part)) {
        const i = Number(part.slice(6));
        if (change.remove) plan.added[side].splice(i, 1);
        else piece = plan.added[side][i] = { ...plan.added[side][i], ...change };
      } else {
        piece = plan[side][part] = { ...(plan[side][part] || STANDARD_PIECE), ...change };
      }
      decor = { ...decor, plan };
      // A desk is measured where it's going, not where it was
      if (part.startsWith('desk')) DESKS = desksOf(decor);
      if (piece && !piece.off) {
        // Kept inside the room, however big it's grown
        const box = pieceBox(part);
        piece.x += Math.max(0, -box.left) - Math.max(0, box.right - shape.width);
        piece.y += Math.max(0, -box.top) - Math.max(0, box.bottom - shape.height);
      }
      settle(plan);
    }

    /** `part` moved so the middle of what it takes up is at `point`, in art pixels. */
    function centreOn(part, point) {
      const box = pieceBox(part), piece = pieceOf(part);
      rearrange(part, { x: piece.x + Math.round(point.x - (box.left + box.right) / 2), y: piece.y + Math.round(point.y - (box.top + box.bottom) / 2) });
    }

    /**
     * A piece added to the lounge: where nothing else is, as near as can be
     * to where the corner's piece stands, or there if the lounge is full.
     */
    function addPiece(kind) {
      const side = shapeName();
      if (addedOf(decor).length >= ADDED_MOST) return null;
      const part = `added:${addedOf(decor).length}`;
      const plan = structuredClone(decor.plan);
      plan.added[side].push({ kind, x: 0, y: 0, size: 1 });
      decor = { ...decor, plan };
      const others = arrangeable().filter((other) => other.part !== part).map(({ box }) => box);
      const overlaps = (a, b) => a.left < b.right && b.left < a.right && a.top < b.bottom && b.top < a.bottom;
      const tries = [];
      for (let dy = -160; dy <= 160; dy += 4) for (let dx = -200; dx <= 120; dx += 4) tries.push({ dx, dy });
      tries.sort((a, b) => Math.hypot(a.dx, a.dy) - Math.hypot(b.dx, b.dy));
      const piece = plan.added[side][plan.added[side].length - 1];
      for (const { dx, dy } of tries) {
        Object.assign(piece, { x: dx, y: dy });
        const box = pieceBox(part);
        const inside = box.left >= 0 && box.top >= WALL_BOTTOM && box.right <= shape.width && box.bottom <= shape.height;
        if (inside && !others.some((other) => overlaps(box, other))) break;
        Object.assign(piece, { x: 0, y: 0 });
      }
      rearrange(part, {});
      return part;
    }

    /**
     * Something from the editor's drawer, into the room at `point` (art
     * pixels), or where it fits best without one: `{ part }` for a desk or a
     * piece put away, `{ kind }` for a piece added to the lounge, or
     * `{ wall }` for one to hang on the back wall. Its part, or null where
     * there's no room for it.
     */
    function bring(what, point) {
      if (what.wall) {
        if (!WALL_CHOICES.includes(what.wall) || what.wall === 'bare') return null;
        const plan = structuredClone(decor.plan), list = hungList(plan);
        if (list.length >= HUNG_MOST) return null;
        const [from, to] = WALL_RANGE[shapeName()];
        const at = hangAt(what.wall, point ? point.x : (from + to) / 2, list);
        if (at === null) return null;
        list.push({ kind: what.wall, x: at });
        settle(plan);
        return `hung:${hungOf().length - 1}`;
      }
      let part = what.part;
      if (what.kind) part = Object.hasOwn(CORNER_BOXES, what.kind) ? addPiece(what.kind) : null;
      else if (!Object.hasOwn(ARRANGEABLE, part) || !ARRANGEABLE[part].hides) return null;
      else rearrange(part, { off: false });
      if (part && point) centreOn(part, point);
      return part;
    }

    /** The room, in the shape shown or in `sides`, with nothing in it but its floor, walls and door, the mic and the red button. */
    function empty(sides = [shapeName()]) {
      const plan = structuredClone(decor.plan);
      for (const side of sides) {
        for (const [part, kind] of Object.entries(ARRANGEABLE)) if (kind.hides) plan[side][part] = { ...(plan[side][part] || STANDARD_PIECE), off: true };
        plan.added[side] = [];
        plan.hung[side] = [];
      }
      settle(plan);
    }

    /** The shape shown arranged as the other one is: what's in it, where, and how big, kept inside this one. */
    function copyFrom(other) {
      const plan = structuredClone(decor.plan), side = shapeName();
      if (other === side) return;
      plan[side] = structuredClone(plan[other]);
      plan.added[side] = structuredClone(plan.added[other]);
      plan.hung[side] = plan.hung[other] ? plan.hung[other].map((piece) => ({ ...piece })) : null;
      settle(plan);
      // Each piece back inside this shape's walls
      for (const { part } of arrangeable()) if (!isHung(part)) rearrange(part, {});
    }

    /** Whether `point` (client pixels) is on the picked piece's handle, for growing it. */
    function onHandle(clientX, clientY) {
      const part = arranging && arranging.picked;
      if (!part || isHung(part) || !(isAdded(part) || (ARRANGEABLE[part] && ARRANGEABLE[part].grows))) return false;
      const { x, y, scale, ratio } = roomAt(clientX, clientY), box = pieceBox(part);
      const reach = Math.max(HANDLE_SIZE, (HANDLE_REACH * ratio) / scale);
      return Math.abs(x - box.right) <= reach && Math.abs(y - box.bottom) <= reach;
    }

    canvas.addEventListener('pointerdown', (e) => {
      if (arranging) {
        const { x, y } = roomAt(e.clientX, e.clientY);
        // The picked piece's handle grows it
        if (onHandle(e.clientX, e.clientY)) {
          const part = arranging.picked, box = pieceBox(part);
          canvas.setPointerCapture(e.pointerId);
          arranging.drag = { id: e.pointerId, part, grow: { left: box.left, top: box.top, w: box.right - box.left, h: box.bottom - box.top, size: pieceOf(part).size } };
          return;
        }
        const hit = arrangeable().find(({ box }) => x >= box.left && x <= box.right && y >= box.top && y <= box.bottom);
        arranging.picked = hit ? hit.part : null;
        on.picked?.(arranging.picked);
        if (!hit) return;
        canvas.setPointerCapture(e.pointerId);
        const from = isHung(hit.part) ? { x: hungOf()[Number(hit.part.slice(5))].x, y: 0 } : { ...pieceOf(hit.part) };
        arranging.drag = { id: e.pointerId, part: hit.part, x, y, from };
        return;
      }
      if (e.pointerType === 'touch') touches.set(e.pointerId, e.clientY);
      // A second finger down: the two scroll the picked agent's Terminal, and what the first was doing stops.
      if (touches.size === 2 && !scrolling) {
        if (finger) lift();
        canvas.setPointerCapture(e.pointerId);
        scrolling = { y: middleOf(touches), at: clock(), moved: false, carry: 0 };
        return;
      }
      if (scrolling) return;
      if (finger) return; // one finger at a time
      const target = targetAt(e.clientX, e.clientY);
      // A tap on the floor, with nothing there, puts away whatever was held out.
      if (!target) {
        held = null;
        musicPanel = null;
        keysFor = null;
        keyArmed = null;
        return;
      }
      canvas.setPointerCapture(e.pointerId);
      finger = { id: e.pointerId, target, at: clock(), x: e.clientX, y: e.clientY };
      if (e.pointerType === 'touch' && SENT_ON_LIFT.includes(target.kind)) {
        finger.waiting = true;
        return;
      }
      press(target);
    });
    for (const type of ['pointerup', 'pointercancel', 'lostpointercapture']) {
      canvas.addEventListener(type, (e) => {
        if (!arranging || !arranging.drag || e.pointerId !== arranging.drag.id) return;
        const { part, grow, from, carried } = arranging.drag;
        arranging.drag = null;
        // Let go off the room, as onto the editor's drawer, or on a place the page takes pieces away at, such as its trash:
        // back where it was carried from, and the page says what becomes of it
        const rect = canvas.getBoundingClientRect();
        const outside = e.clientX < rect.left || e.clientX > rect.right || e.clientY < rect.top || e.clientY > rect.bottom
          || !!on.takesAway?.(e.clientX, e.clientY);
        if (type === 'pointerup' && !grow && outside) {
          rearrange(arranging.picked || part, isHung(part) ? { x: from.x } : { x: from.x, y: from.y });
          on.droppedOut?.(arranging.picked || part, e.clientX, e.clientY);
        }
        if (carried) on.dragging?.(null);
      });
    }
    for (const type of ['pointerup', 'pointercancel', 'lostpointercapture']) {
      canvas.addEventListener(type, (e) => {
        if (finger && e.pointerId === finger.id) lift(type === 'pointerup');
        if (!touches.delete(e.pointerId) || !scrolling || touches.size) return;
        // Both up: a quick tap of two fingers, still, goes to the bottom
        if (type === 'pointerup' && !scrolling.moved && clock() - scrolling.at < TWO_FINGER_TAP) on.scroll?.('bottom');
        scrolling = null;
      });
    }
    /** Where two fingers are, between them, in CSS pixels down the page. */
    const middleOf = (fingers) => [...fingers.values()].reduce((sum, y) => sum + y, 0) / fingers.size;
    canvas.addEventListener('pointermove', (e) => {
      if (arranging) {
        const { x, y } = roomAt(e.clientX, e.clientY);
        const drag = arranging.drag;
        if (drag && e.pointerId === drag.id && drag.grow) {
          // Grown by how far the handle is from the piece's top-left corner, to the nearest size it can be
          const { left, top, w, h, size } = drag.grow;
          const wanted = size * Math.max((x - left) / w, (y - top) / h);
          const nearest = PIECE_SIZES.reduce((best, s) => (Math.abs(s - wanted) < Math.abs(best - wanted) ? s : best));
          if (nearest !== pieceOf(drag.part).size) rearrange(drag.part, { size: nearest });
        } else if (drag && e.pointerId === drag.id) {
          // Carried, not just picked: the page can show where pieces are taken away
          if (!drag.carried && Math.hypot(x - drag.x, y - drag.y) >= 1) {
            drag.carried = true;
            on.dragging?.(drag.part);
          }
          const moved = { x: drag.from.x + Math.round(x - drag.x), y: drag.from.y + Math.round(y - drag.y) };
          if (isHung(drag.part)) {
            if (moved.x !== hungOf()[Number(drag.part.slice(5))].x) rearrange(drag.part, { x: moved.x });
          } else {
            const now = pieceOf(drag.part);
            if (moved.x !== now.x || moved.y !== now.y) rearrange(drag.part, moved);
          }
        } else if (e.pointerType === 'mouse') {
          canvas.style.cursor = onHandle(e.clientX, e.clientY) ? 'nwse-resize'
            : arrangeable().some(({ box }) => x >= box.left && x <= box.right && y >= box.top && y <= box.bottom) ? 'grab' : '';
        }
        return;
      }
      if (scrolling && touches.has(e.pointerId)) {
        touches.set(e.pointerId, e.clientY);
        if (touches.size < 2) return;
        // Fingers going up scroll down, a line for each so far they go, as they go
        const y = middleOf(touches);
        scrolling.carry += scrolling.y - y;
        scrolling.y = y;
        if (Math.abs(scrolling.carry) >= SCROLL_STEP) scrolling.moved = true;
        const lines = Math.trunc(scrolling.carry / SCROLL_STEP);
        if (lines) {
          scrolling.carry -= lines * SCROLL_STEP;
          on.scroll?.(lines);
        }
        return;
      }
      if (finger && e.pointerId === finger.id && finger.waiting) {
        if (Math.hypot(e.clientX - finger.x, e.clientY - finger.y) >= CARRY_AFTER) finger.moved = true;
        return;
      }
      if (finger && e.pointerId === finger.id && finger.target.kind === 'role') return slide(e);
      if (finger && e.pointerId === finger.id) return carry(e);
      // A mouse shows what can be clicked.
      if (e.pointerType === 'mouse' && !finger) canvas.style.cursor = targetAt(e.clientX, e.clientY) ? 'pointer' : '';
    });

    /** A finger on the roles, moved along them further than a tap would, slides them with it. */
    function slide(e) {
      if (!finger.slid) {
        if (Math.abs(e.clientX - finger.x) < CARRY_AFTER) return;
        finger.slid = true;
        finger.from = roleOffset;
        roleDown = null;
      }
      const a = roomAt(finger.x, finger.y), b = roomAt(e.clientX, e.clientY);
      roleOffset = Math.max(0, Math.min(rolesAt ? rolesAt.most : 0, finger.from + a.x - b.x));
    }

    /** A finger held on an agent's desk, then moved, picks the agent up and carries them. */
    /** Who is at each desk, and which are pinned: the desks a swap is counted by, by number. */
    const desksNow = () => office.seats.map((s) => `${s.session ? s.session.id : ''} ${s.pin || ''}`).join('\n');

    function carry(e) {
      const { x, y } = roomAt(e.clientX, e.clientY);
      if (!carrying) {
        if (!finger.held || finger.target.kind !== 'session') return;
        if (Math.hypot(e.clientX - finger.x, e.clientY - finger.y) < CARRY_AFTER) return;
        const from = office.seats.findIndex((s) => s.session && s.session.id === finger.target.id);
        if (from < 0) return;
        held = null;
        const person = office.people.find((p) => !p.isStaff && !p.isLeaving && p.session.id === finger.target.id) || null;
        carrying = { person, from, x, y, over: from, desks: desksNow() };
        on.held?.();
      }
      const over = deskAt(x, y);
      if (over !== null && over !== carrying.over && over !== carrying.from) on.held?.(); // a tick for each desk passed over
      Object.assign(carrying, { x, y, over });
    }
    bind();
    resize();
    run();

    return {
      show(snapshot) {
        latest = snapshot;
        if (snapshot.now) gaps = [...gaps.slice(1 - CLOCK_SAMPLES), Date.now() - snapshot.now];
        ahead = gaps.length ? Math.min(...gaps) : 0;
        hiring = snapshot.hiring || { fire: false, hire: false };
        music = snapshot.music || null;
        office.furnish(snapshot.layout && snapshot.layout.room, clock());
        office.apply(seatsOf(snapshot, ahead), snapshot.theme, clock());
        // The question and what's held out go with their agent; a hire, once its desk is taken.
        const there = (id) => office.deskOfSession(id) !== null;
        if (firing && (!hiring.fire || !there(firing.id))) firing = null;
        if (held && (held.kind === 'session' ? !there(held.id) : !hiresInto(held.desk))) held = null;
        // A swap is sent as two desks' numbers. With someone gone or come, or a desk pinned, meanwhile, they'd
        // be other desks, and the wrong two would swap and stay so. The carry is let go, and the finger, still
        // down, picks the agent up again as it moves, counted from the desks as they are now.
        if (carrying && carrying.desks !== desksNow()) carrying = null;
      },
      close() {
        latest = null;
        gaps = [];
        office.close();
      },
      select(id) {
        // Another agent picked: what was held out for the last one goes away.
        if (held && held.kind === 'session' && held.id !== id) held = null;
        selected = id;
      },
      setVisible(shown) {
        visible = shown;
        if (shown) {
          resize();
          run();
          ambience.wake(host.window, loudness());
        } else {
          if (finger) lift();
          ambience.sleep();
        }
      },
      setFramed(shown) { framed = shown; },
      /** Floats the room's window in a 'window' of its own or a whole 'screen', fills it from 'edge' to edge, or stops, with null. */
      setFloating(how) { floating = how; },
      /**
       * On the setup page, the room is for arranging: a tap picks a piece,
       * `on.picked(part)`, and dragging moves it, `on.arranged(plan)` with
       * the room's whole arrangement as it goes; the picked piece's handle
       * grows it, and a piece let go off the room, or where
       * `on.takesAway(clientX, clientY)` says pieces are taken away, says so
       * with `on.droppedOut(part, clientX, clientY)`. `on.dragging(part)`
       * says a piece is being carried, and `on.dragging(null)` that it's down. `arrange(part, change)`
       * changes a piece, such as its size, or with `remove` takes an added
       * or hung one out; `add(kind)` adds one; `pick(part)` picks one.
       */
      setArranging(shown) {
        arranging = shown ? { picked: null, drag: null, blocked: blocked() } : null;
        if (shown) styling = false;
      },
      pick(part) { if (arranging) arranging.picked = part; },
      arrange(part, change) { if (arranging) rearrange(part, change); },
      /** Adds a piece to the lounge, any the corner can hold, and picks it: its part, or null with the lounge full. */
      add(kind) {
        if (!arranging || !Object.hasOwn(CORNER_BOXES, kind)) return null;
        const part = addPiece(kind);
        if (part) arranging.picked = part;
        return part;
      },
      /**
       * Brings something from the editor's drawer into the room, at the
       * client point given if it's over the room, or where it fits: `{ part }`
       * for a desk or a piece put away, `{ kind }` for a lounge piece, or
       * `{ wall }` for one to hang. Picks it: its part, or null with no room.
       */
      bring(what, clientX, clientY) {
        if (!arranging) return null;
        let point = null;
        if (clientX !== undefined) {
          const { x, y } = roomAt(clientX, clientY);
          if (x >= 0 && y >= 0 && x <= shape.width && y <= shape.height) point = { x, y };
        }
        const part = bring(what, point);
        if (part) arranging.picked = part;
        return part;
      },
      /** Whether a client point is over the room itself, for something carried from the drawer. */
      over(clientX, clientY) {
        const { x, y } = roomAt(clientX, clientY);
        return x >= 0 && y >= 0 && x <= shape.width && y <= shape.height;
      },
      /** Empties the room in the shape shown, or on its side and upright with `both`, but for its floor, walls and door, the mic and the red button. */
      empty(both = false) { if (arranging) { arranging.picked = null; empty(both ? ['wide', 'tall'] : undefined); } },
      /** Arranges the shape shown as the other one, 'wide' or 'tall', is arranged. */
      copyFrom(side) { if (arranging) { arranging.picked = null; copyFrom(side); } },
      /** The desks, and the red button, nobody can walk to from the door. */
      blocked() { return arranging ? [...(arranging.blocked || [])] : []; },
      /** What hangs on the back wall in the shape shown, as `hung:i` names it. */
      hung() { return hungOf().map((piece) => ({ ...piece })); },
      /**
       * The canvas is in another window now, or back on its page. Moving it
       * takes the finger off without a word to the canvas, so the mic is let
       * go here, however the move came about.
       */
      moved() {
        if (finger) lift();
        bind();
        resize();
        run();
      },
    };
  }

  /** What a preview shows of the room, in the wide room's art pixels: round the corner piece, or round the first desk. */
  const PREVIEW_CROPS = {
    corner: { x: 238, y: 49, w: 80, h: 51 },
    workspace: { x: STANDARD_DESKS[0].x - 6, y: STANDARD_DESKS[0].y - 27, w: DESK_WIDTH + 14, h: 45 },
    preset: { x: STANDARD_DESKS[0].x - 10, y: STANDARD_DESKS[0].y - 24, w: STYLE_THUMB.w, h: STYLE_THUMB.h },
    // A piece on the back wall, hung in its middle place, on a wall with nothing else on it
    wall: { x: WALL_SLOTS[2] - 29, y: 0, w: 58, h: 45 },
    // A part of the lounge, on its own: the counter and cooler, the couch, the table or the plant
    counter: { x: 230, y: 2, w: 83, h: 65 },
    couch: { x: 244, y: 98, w: 64, h: 50 },
    pong: { x: 240, y: 132, w: 58, h: 45 },
    plant: { x: 202, y: 135, w: 58, h: 45 },
  };
  /** The plant, in its picture, moved clear of the red button beside it. */
  const PREVIEW_PLANT = { x: 30, y: 0 };
  let previewRoom = null; // the canvas previews draw the whole room on
  /** Someone at a workspace in its preview, at ease, so no bubble hides it. */
  const PREVIEW_AGENT = {
    id: 'preview', name: 'preview', state: 'idle', effort: '', agent: 'claude', model: '', tool: 'Edit', subagents: [],
    maxPressedAt: null, turnStartedAt: null, dial: false, mic: false,
  };

  /**
   * One of the room's choices on its own, for the setup page to show it by:
   * `what` is { corner } with someone at each of its places, { workspace }
   * with an agent at it, { preset }, a style, round the first desk, { wall },
   * a piece for the back wall, on it alone, or { part }, a piece of the
   * room the editor puts away: 'desk', an empty one, or a part of the
   * lounge, the rest of the lounge put away round it. It's
   * drawn by the room's own code in
   * the layout's `room` (its floor, paint and finishes), `now` seconds in,
   * and cut to the piece, as big as fits `canvas` in whole pixels. `tall`
   * shows it where the room held upright has it.
   */
  function preview(canvas, what, { room = null, now = 0, tall = false } = {}) {
    const kept = { decor, spots, boardSpots, stormSpots, shape, DESKS, floor };
    try {
      shape = tall ? TALL : WIDE;
      // Shown in the room as you furnished it, unless it's a style being shown.
      // As it comes, not as it's arranged: a picture of a piece, not of where it is.
      const { wall: hang, part, ...choice } = what;
      const lounge = part && part !== 'desk' ? part : null;
      // A part of the lounge alone: the rest of it put away
      const alone = lounge && Object.fromEntries(['wall', 'corner', 'couch', 'pong', 'plant']
        .map((other) => [other, other === lounge ? { ...(lounge === 'plant' ? PREVIEW_PLANT : {}) } : { off: true }]));
      decor = { ...roomOf({ ...room, preset: 'yours', ...choice, team: null, plan: alone ? { wide: alone, tall: alone } : null }),
        staff: 'none', sky: 'day', pet: 'none' };
      if (hang) decor.wall = ['bare', 'bare', hang, 'bare', 'bare'];
      // On the wall, nothing in front of it
      DESKS = hang ? STANDARD_DESKS.map((desk) => ({ ...desk, off: true })) : STANDARD_DESKS;
      floor = null;
      spots = spotsOf(decor);
      boardSpots = boardSpotsOf(decor);
      stormSpots = stormSpotsOf(decor);
      const office = new Office();
      office.apply(what.corner || hang || part ? [] : [{ pin: null, session: PREVIEW_AGENT }], {}, now);
      if (what.corner) CORNER_SPOTS[decor.corner].forEach((_, i) => {
        const spot = LOUNGE_SPOTS.length + i, member = office.hireStaff();
        member.place = spotPlace(spot);
        member.position = { x: spots[spot].x, y: spots[spot].y };
        member.isFacingAway = facesAway(spots[spot].pose);
      });
      const crop = { ...PREVIEW_CROPS[what.corner ? 'corner' : what.preset ? 'preset' : hang ? 'wall'
        : lounge === 'wall' ? 'counter' : lounge || 'workspace'] };
      if (what.corner) Object.assign(crop, placed('corner', crop));
      if (lounge) Object.assign(crop, placed(lounge, crop));
      const scale = Math.max(1, Math.floor(Math.min(canvas.width / crop.w, canvas.height / crop.h)));
      // One canvas for the whole room, kept from one preview to the next
      if (!previewRoom || previewRoom.ownerDocument !== canvas.ownerDocument) previewRoom = canvas.ownerDocument.createElement('canvas');
      const whole = previewRoom;
      whole.width = shape.width * scale; // clears it, too
      whole.height = shape.height * scale;
      drawOffice(whole.getContext('2d'), office, {
        now, hour: 12, minute: 0, month: 5, width: whole.width, height: whole.height, ratio: 1, selected: null,
        mic: { held: false, heldAt: null, releasedAt: null, listener: null, usable: true }, framed: false, floating: null,
      });
      const ctx = canvas.getContext('2d');
      ctx.imageSmoothingEnabled = false;
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      const w = crop.w * scale, h = crop.h * scale;
      ctx.drawImage(whole, crop.x * scale, crop.y * scale, w, h,
        Math.floor((canvas.width - w) / 2), Math.floor((canvas.height - h) / 2), w, h);
    } finally {
      ({ decor, spots, boardSpots, stormSpots, shape, DESKS, floor } = kept);
    }
  }

  /** The pop-out window's size, in CSS pixels: two to each of the room's, and the margin round it. */
  const popOutSize = { width: WIDTH * 2 + FLOAT_MARGINS.window * 2, height: HEIGHT * 2 + FLOAT_MARGINS.window * 2 };

  /** The deck's reminder, on its own context: it plays whatever the office's volume is. */
  function remind(audio) {
    const soft = audio.createBiquadFilter();
    soft.type = 'lowpass';
    soft.frequency.value = 7000;
    soft.Q.value = 0;
    soft.connect(audio.destination);
    remindSound(new Chip(audio, soft), audio.currentTime + 0.02);
  }

  // `preview` draws a piece for the setup page, and `arrangeable` and `sizes` are what its editor can arrange;
  // `furnishingOf` writes a room in a style out as your own, for the setup page to change one piece of it;
  // `remind` is the deck's reminder; `Ambience` and `sounds` let each of the office's sounds be tried on its own.
  window.AgentOffice = {
    create, popOutSize, preview, arrangeable: ARRANGEABLE, sizes: PIECE_SIZES, remind, Ambience, sounds: Object.keys(SOUNDS), furnishingOf,
    // What the back wall can hold, each piece's width, and where along the wall they can go, for the editor's drawer
    wallPieces: Object.fromEntries(WALL_CHOICES.filter((kind) => kind !== 'bare').map((kind) => [kind, { w: wallWidthOf(kind) }])),
    wallRange: WALL_RANGE, hungMost: HUNG_MOST, addedMost: ADDED_MOST,
    // The badges a role can wear, in setup's order, and each one's icon on a canvas of its own for a role's card
    badges: BADGE_IDS, badgeIcon: (id, scale = 1) => (Object.hasOwn(BADGES, id) ? paintIcon(BADGES[id].icon, Math.max(1, Math.round(scale))) : null),
  };
})();
