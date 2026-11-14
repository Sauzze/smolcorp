# The office's sources

`Resources/web/office.js` is built from the files in this folder. It stays
committed, and it is still the one file every app uses: the Mac app and the
Windows and Linux app serve it, the viewer copies it at deploy, phones load it,
and many Swift and Go tests read it. Nothing here is served or built into an
app (`web.go` leaves this folder out, `scripts/build-app.sh` removes it from
the Mac app).

Change a file here, then build:

    go run ./scripts/buildoffice          # writes Resources/web/office.js
    go run ./scripts/buildoffice -check   # fails if office.js isn't exactly the build

Commit both the source and office.js. `go test ./scripts/buildoffice` (run in
CI) fails when office.js isn't exactly the build of this folder, and
`scripts/build-app.sh` and `scripts/release-desktop.sh` run `-check` before
they build. Don't change office.js by hand: the next build overwrites it.

## The build

No bundler and no dependencies: a short Go program, standard library only.
`main.js` is office.js with a line like

    //#include data/outfit-parts.js

where each table goes. The build puts each table there, every line of it
indented as the include line is (empty lines stay empty), so the tables are
written from the left edge here and come out exactly where they were. The
order is `main.js`'s, and must be the order of the list in
`scripts/buildoffice/main.go`, each table once.

Each table file is one `const`, with the doc comment right above it, and
parses on its own (`node --check`); it isn't a module of its own yet. It still
uses what's around its include line: `fill`, `dot`, `now`, `PALETTE` and the
rest of `drawOffice`'s closure for the ones inside it, and the IIFE's
top-level names for the first three.

## Module map (step 1, 1.25.0)

| File | Table | Size | What it is |
| --- | --- | --- | --- |
| `main.js` | everything else | 2.3 MB | the renderer, input, hiring, T3, choices and window code, as they were |
| `data/room-styles.js` | `ROOM_STYLES` | 66 KB | the office's styles: what each one furnishes |
| `data/paints.js` | `PAINTS` | 24 KB | the wall's paints and their colours |
| `data/outfits.js` | `OUTFITS` | 58 KB | every outfit's colours and style |
| `data/wall-finishes.js` | `WALL_FINISHES` | 83 KB | the walls' finishes besides paint |
| `data/floor-finishes.js` | `FLOOR_FINISHES` | 149 KB | the floors besides wood, tiles, concrete and carpet |
| `data/wall-pieces.js` | `WALL_PIECES` | 698 KB | what hangs on the back wall |
| `data/monitors.js` | `MONITORS` | 123 KB | the desks' monitors |
| `data/workspace-parts.js` | `WORKSPACE_PARTS` | 204 KB | the workspaces: desks and what's round them |
| `data/desk-items.js` | `DESK_ITEMS` | 639 KB | what sits at the end of a desk |
| `data/mic-heads.js` | `MIC_HEADS` | 30 KB | the tops of the mic's stand |
| `data/button-styles.js` | `BUTTON_STYLES` | 37 KB | the emergency buttons and their reactions |
| `data/outfit-parts.js` | `OUTFIT_PARTS` | 411 KB | the outfits' fronts, backs and hats |

The first three are at the top of the IIFE, the rest inside `drawOffice`.

## Code modules (1.26)

The first files here that are code, not data tables. Each is one `const` (an
object of functions), built in exactly as a table is, and parses on its own
(`node --check`). A feature that shows in the bubble or the Pixel Terminal
(a diff, a preview, a photo, the mic) goes in a `talk/<name>.js` of its own,
included after `talk/terminal.js`, and registers itself as its const is made
(`added: PIXEL_TERMINAL.addPanel(name, draw)` or `TALK_BUBBLE.addKey(name,
key)`): see the comment at the top of `talk/terminal.js`.

| File | Const | Where | What it is |
| --- | --- | --- | --- |
| `talk/style.js` | `TALK_STYLE` | first in the IIFE | the talk bubbles' CSS, put on the page once as a `<style>`: the pixel edges, keys, the open bubble, the small ones, the tag, Needs you and the Pixel Terminal |
| `talk/bubble.js` | `TALK_BUBBLE` | after `TALK_STYLE` | the talk bubble: the conversation's rows and page merge (shared with deck.html's ▤ slate), a prompt's choices and what each sends, the input's gate, and `create`, the live bubbles the room opens where the page gives `on.talk` its network, with their keyboard (Enter and Tab from the room, arrows, digits, Esc giving the focus back), the live region and the hidden desk list for a screen reader |
| `talk/terminal.js` | `PIXEL_TERMINAL` | after `TALK_BUBBLE` | the Pixel Terminal, Expand's full-size view of one desk: Back, Shrink, Next question, the whole conversation with Older messages (the page merge shared with the slate), the roomy input, the desk list and the office's picture; and `addPanel`, the seam other files add panels by |
| `talk/anchors.js` | `TALK_ANCHORS` | top of the IIFE, before `poseOf` | where each person's talk bubble is: its box and tail point in art and CSS pixels, the hover's timing, which small question bubbles fit, who has waited longest; plain functions of what they're given, so node tests run them alone |

## Adding a table

1. Move it out of `main.js` into `data/<name>.js`, from its doc comment to the
   `};` that closes it, without its indent, and put
   `//#include data/<name>.js` where it was, at its indent.
2. Add `{"data/<name>.js", "<NAME>"}` to `modules` in
   `scripts/buildoffice/main.go`, at its place in the order.
3. `go run ./scripts/buildoffice`, then `git diff Resources/web/office.js`: a
   move gives no diff at all.

`go test ./scripts/buildoffice` also checks that every file here is a source
the build knows (or this README).

## A branch from before this folder

A branch made before these sources changed office.js itself. Merged, its
office.js has the change and the sources don't, so the check fails. Split it
back:

    go run ./scripts/buildoffice -split

It finds each table in office.js by its `const` line (with the doc comment
right above it, down to its closing `};`), writes the tables and `main.js`,
and checks they build office.js again. A change to the code lands in
`main.js`, one to a table in that table's file. Once no such branch is left,
`-split` and its test can go.

## Private packs

The public build has none, and there is no build-time step that strips them:
`scripts/check-public.sh` checks every committed file, these sources included.
A private pack's outfits and pieces go in the same tables as everyone's, on
its private branch only; never commit one here on a public branch.

## Next steps (step 2 and later, a later release)

Step 1 moved the large data tables only, which nothing else in flight was
changing. Next, once the work changing them has landed:

- **More tables and art**: the pieces' own drawing functions between
  `DESK_ITEMS` and `drawPet` (about 700 KB, some 370 `draw…` functions), `drawPet`
  (325 KB of pet sprites), `SEASONAL_TOUCHES`, `WAVE_STYLES`, the outfits'
  costume helpers round `OUTFIT_PARTS`, the sounds, and the choices'
  tables (`ROOM_CHOICES`, `TEAM_CHOICES`, `WALL_CHOICES`).
- **The renderer in pieces**: `drawOffice` is still one function of about
  24,000 lines in `main.js`, and the tables reach into its closure. Splitting
  it means giving them what they draw with (`fill`, `dot`, `now` and the rest)
  as an argument, part by part, keeping office.js byte-identical where a part
  only moves and checking the office by eye where it doesn't.
- **Then input, hiring, choices and windows** as files of their own, each with
  its own `//#include`.
- Real modules (each file its own scope, imported by name) come last, once
  the closures are gone; office.js stays one file served as it is today.
