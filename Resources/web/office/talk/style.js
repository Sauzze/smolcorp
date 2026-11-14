/**
 * How the talk bubbles look (1.26, Talk Bubble): the open bubble over a head, the small question bubbles, the hover's
 * tag and the Needs you key, drawn as the office is: hard 2-pixel edges in the bubble's ink on its cream, corners
 * stepped by leaving the edge's corner pixels out, hard shadows, keys in capitals that press 2 pixels down. Reading
 * text stays in the system font at 13 px; the office's pixel font has only capitals and digits. Put on the page once,
 * by the bubble, as a <style> of its own. They sit below the deck's own panels (the ▤ slate, a peek, a confirm: z-index
 * 8 and up), so a panel on top is never covered by a bubble still clickable under it; and the open bubble sits above the
 * small question bubbles, which are drawn later (as other desks' questions come and go) and would lie over its keys. In
 * a short window the open bubble gives way in this order, so its keys and its input stay in view: the conversation
 * shrinks first (to 44 px), then the question (to two lines, scrolling), and only then the choices scroll inside
 * their box; the input stays at the foot. The Pixel Terminal is the ▤ slate's chalkboard in its wood frame, its
 * corners stepped, at the slate's height (z-index 8) over a dimmed room.
 *
 * Contrast (WCAG AA, checked by the node tests): reading text at least 4.5:1 on its own background, a focus ring at
 * least 3:1: orange on the dark parts, the bubble's ink on the cream ones (`--focus`), and over the room, where the
 * floor can be any colour, yellow round the key's own dark edge. The live region and the desk list are for a screen
 * reader: out of sight, the list shown in the room's corner while the keyboard is in it. With reduced motion nothing
 * grows, shrinks or slides; the terminal only fades in.
 */
const TALK_STYLE = {
  ID: 'talk-bubble-style',

  CSS: `
.talk-desks, .talk-bubble, .talk-small, .talk-tag, .talk-needs { --edge: #1b1e2a; --cream: #f4f1ea; --paper: #fffdf8; --screen: #171b26;
  --chalk: #e6e9df; --dim: #8d95a6; --orange: #ff8a1f; --yellow: #ffd34d; --slate: #3e495b; --focus: var(--orange);
  position: fixed; z-index: 7; box-sizing: border-box; color: var(--edge); font: 13px/1.4 -apple-system, system-ui, sans-serif; }
.talk-bubble *, .talk-small * { box-sizing: border-box; }
.talk-bubble[hidden], .talk-small[hidden], .talk-tag[hidden], .talk-needs[hidden], .talk-desks[hidden], .talk-bubble [hidden] { display: none !important; }
.talk-edge { background: var(--cream);
  box-shadow: 0 -2px 0 var(--edge), 0 2px 0 var(--edge), -2px 0 0 var(--edge), 2px 0 0 var(--edge), 0 6px 0 rgba(12, 15, 22, .35); }
.talk-bubble { width: 320px; display: flex; flex-direction: column; }
.talk-bubble > * { flex: none; }
.talk-bubble:focus { outline: none; }
.talk-bubble:focus-visible { outline: 2px solid var(--yellow); outline-offset: 2px; }
.talk-bubble.grow { animation: talk-grow 120ms steps(3, end); }
.talk-bubble.shrink { animation: talk-grow 120ms steps(3, end) reverse; }
@keyframes talk-grow { from { transform: scale(.4); } to { transform: scale(1); } }
@media (prefers-reduced-motion: reduce) { .talk-bubble.grow, .talk-bubble.shrink { animation: none; } }
.talk-head { display: flex; align-items: center; gap: 6px; padding: 5px 6px 5px 8px; background: var(--edge); color: var(--cream);
  font: 700 11px/1 ui-monospace, Menlo, monospace; text-transform: uppercase; letter-spacing: .04em; white-space: nowrap; }
.talk-name { overflow: hidden; text-overflow: ellipsis; min-width: 0; }
.talk-state { font-weight: 400; color: var(--dim); flex: 1; overflow: hidden; text-overflow: ellipsis; }
.talk-dot { flex: none; width: 8px; height: 8px; background: var(--dim); }
.talk-dot.thinking { background: #3b82f6; } .talk-dot.asking { background: var(--orange); }
.talk-dot.unread { background: #22c55e; } .talk-dot.error { background: #ef4444; }
.talk-bubble > .talk-lines { flex: 0 100 auto; min-height: 44px; }
.talk-bubble > .talk-ask { flex: 0 1 auto; min-height: 0; overflow-y: auto; }
.talk-lines { display: flex; flex-direction: column; gap: 6px; max-height: 190px; overflow-y: auto; padding: 8px 9px;
  background: var(--screen); color: var(--chalk); overflow-wrap: anywhere; }
.talk-lines:empty { display: none; }
.talk-lines .board-row { flex: none; white-space: pre-wrap; max-height: 5.6em; overflow: hidden; }
.talk-lines .board-row.user { align-self: flex-end; max-width: 88%; padding: 3px 7px; background: var(--paper); color: var(--edge); }
.talk-lines .board-row.tools, .talk-lines .board-row.result, .talk-lines .board-row.tool { color: var(--dim);
  font: 12px/1.35 ui-monospace, Menlo, monospace; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.talk-lines .board-row.tools::before { content: "> "; }
.talk-lines .from { display: block; font: 700 10px/1.2 ui-monospace, Menlo, monospace; text-transform: uppercase; color: var(--orange); }
.talk-lines .more { color: var(--dim); }
.talk-lines .talk-none { color: var(--dim); font-style: italic; }
.talk-ask { display: flex; flex-direction: column; gap: 6px; padding: 8px 9px; background: #252c3d; color: var(--cream); box-shadow: inset 0 0 0 2px var(--orange); }
.talk-ask > * { flex: none; }
.talk-ask > .talk-q { flex: 0 1 auto; min-height: 2.8em; }
.talk-q { color: var(--yellow); font-weight: 600; white-space: pre-wrap; max-height: 7em; overflow: auto; }
.talk-keys { display: flex; flex-wrap: wrap; gap: 6px 8px; padding: 2px; }
.talk-hint { color: var(--dim); font-size: 12px; }
.talk-key { display: inline-flex; align-items: center; gap: 5px; min-height: 24px; min-width: 24px; margin: 2px; padding: 4px 7px; border: 0;
  border-radius: 0; background: var(--cream); color: var(--edge); cursor: pointer; text-align: left;
  font: 700 11px/1.15 ui-monospace, Menlo, monospace; text-transform: uppercase; letter-spacing: .03em;
  box-shadow: 0 -2px 0 var(--edge), 0 2px 0 var(--edge), -2px 0 0 var(--edge), 2px 0 0 var(--edge), inset 0 -2px 0 rgba(0, 0, 0, .18); }
.talk-key:active:not(:disabled) { transform: translateY(2px); box-shadow: 0 -2px 0 var(--edge), 0 2px 0 var(--edge), -2px 0 0 var(--edge), 2px 0 0 var(--edge); }
.talk-key:disabled { opacity: .45; cursor: default; }
.talk-key:focus-visible, .talk-small-q:focus-visible { outline: 2px solid var(--focus); outline-offset: 3px; }
.talk-input, .talk-small, .pt-menu, .talk-desks { --focus: var(--edge); }
.talk-needs:focus-visible { outline-color: var(--yellow); outline-offset: 2px; }
canvas[data-talk-room]:focus { outline: none; }
canvas[data-talk-room]:focus-visible { outline: 2px solid #ffd34d; outline-offset: -2px; }
.talk-key.go { background: var(--orange); }
.talk-key.dark { background: var(--slate); color: var(--cream); }
.talk-key i { font-style: normal; padding: 2px 4px; margin: -2px 0; background: var(--edge); color: var(--cream); }
.talk-input { display: flex; gap: 6px; align-items: center; padding: 7px 8px; }
.talk-input input { flex: 1; min-width: 0; height: 28px; padding: 3px 7px; border: 0; border-radius: 0; background: var(--paper); color: var(--edge);
  font: inherit; box-shadow: inset 0 0 0 2px var(--edge); }
.talk-input input:focus { outline: 3px solid var(--focus); outline-offset: 1px; }
.talk-input input[readonly], .pt-input textarea[readonly] { background: #e3ded2; color: #4b5263; -webkit-text-fill-color: #4b5263; opacity: 1; }
.talk-input input::placeholder, .pt-input textarea::placeholder { color: #5b6170; -webkit-text-fill-color: #5b6170; opacity: 1; }
.talk-why, .talk-problem { margin: 0; padding: 7px 9px; color: #4b5263; }
.talk-problem { padding-top: 0; color: #a12d2d; }
.talk-tail { position: absolute; width: 14px; height: 9px; background: var(--edge);
  clip-path: polygon(0 0, 100% 0, 100% 22%, 78% 22%, 78% 56%, 57% 56%, 57% 100%, 43% 100%, 43% 56%, 22% 56%, 22% 22%, 0 22%); }
.talk-tail::after { content: ""; position: absolute; left: 2px; top: 0; width: 10px; height: 5px; background: var(--cream);
  clip-path: polygon(0 0, 100% 0, 100% 40%, 70% 40%, 70% 100%, 30% 100%, 30% 40%, 0 40%); }
.talk-small { display: flex; flex-direction: column; justify-content: center; gap: 2px; padding: 2px 5px; cursor: pointer; overflow: hidden;
  font-size: var(--talk-font, 11px); line-height: 1.2; }
.talk-small-q { display: block; width: 100%; margin: 0; padding: 0; border: 0; background: none; color: inherit; font: inherit; font-weight: 600;
  text-align: left; cursor: pointer; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.talk-small .talk-keys { flex-wrap: nowrap; gap: 4px; padding: 1px 2px; }
.talk-small .talk-key { min-height: 0; padding: 1px 4px; margin: 1px; font-size: .85em; }
.talk-small .talk-small-q:focus-visible { outline-offset: 0; }
.talk-tag { max-width: 300px; padding: 3px 7px; font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; pointer-events: none; }
.talk-needs, .talk-small { z-index: 6; }
.talk-head .talk-added { display: contents; }
.talk-live, .talk-desks:not(:focus-within) { position: fixed; left: 0; top: 0; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%);
  white-space: nowrap; }
.talk-desks:focus-within { position: fixed; left: 8px; top: 8px; z-index: 7; display: flex; flex-direction: column; gap: 4px; padding: 8px;
  max-width: min(320px, calc(100vw - 16px)); max-height: calc(100vh - 16px); overflow-y: auto; }
.talk-desk.talk-key { display: block; text-transform: none; letter-spacing: 0; font: 600 12px/1.3 -apple-system, system-ui, sans-serif;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
@media (prefers-reduced-motion: reduce) {
  .talk-key:active:not(:disabled) { transform: none; }
}
.pt, .pt-dim { --edge: #1b1e2a; --cream: #f4f1ea; --paper: #fffdf8; --chalk: #eef0ea; --board: #1f2b26; --wood: #6b4a2b;
  --dim: #9fb0a6; --orange: #ff8a1f; --yellow: #f2d16b; --slate: #3e495b; --focus: var(--orange); position: fixed; z-index: 8; box-sizing: border-box; }
.pt *, .pt *::before { box-sizing: border-box; }
.pt[hidden], .pt-dim[hidden], .pt [hidden] { display: none !important; }
.pt-dim { inset: 0; background: rgba(10, 12, 18, .6); }
.pt { inset: 16px; display: flex; flex-direction: column; overflow: hidden; background: var(--board); color: var(--chalk);
  font: 13px/1.4 -apple-system, system-ui, sans-serif; -webkit-user-select: text; user-select: text;
  clip-path: polygon(0 6px, 2px 6px, 2px 2px, 6px 2px, 6px 0, calc(100% - 6px) 0, calc(100% - 6px) 2px, calc(100% - 2px) 2px,
    calc(100% - 2px) 6px, 100% 6px, 100% calc(100% - 6px), calc(100% - 2px) calc(100% - 6px), calc(100% - 2px) calc(100% - 2px),
    calc(100% - 6px) calc(100% - 2px), calc(100% - 6px) 100%, 6px 100%, 6px calc(100% - 2px), 2px calc(100% - 2px), 2px calc(100% - 6px), 0 calc(100% - 6px));
  box-shadow: inset 0 0 0 4px var(--wood), inset 0 0 0 6px var(--edge); padding: 6px; }
.pt:focus { outline: none; }
.pt.enter, .pt-dim.enter { animation: pt-in 200ms steps(4, end); }
@keyframes pt-in { from { opacity: 0; } to { opacity: 1; } }
.pt-head { position: relative; display: flex; flex-wrap: wrap; align-items: center; gap: 6px; padding: 6px 8px; background: var(--edge);
  color: var(--cream); font: 700 11px/1 ui-monospace, Menlo, monospace; text-transform: uppercase; letter-spacing: .04em; white-space: nowrap; }
.pt-head .talk-name { color: var(--chalk); margin-left: 6px; }
.pt-head .talk-state { color: var(--dim); }
.pt-menu { position: absolute; right: 8px; top: calc(100% + 4px); z-index: 1; display: flex; flex-direction: column; padding: 6px; min-width: 180px; }
.pt-cols { flex: 1; min-height: 0; display: flex; }
.pt-main { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.pt-main > * { flex: none; }
.pt-body { flex: 1 1 auto; min-height: 60px; overflow-y: auto; padding: 6px 12px 12px; display: flex; flex-direction: column; gap: 7px; overflow-wrap: anywhere; }
.pt-body > * { flex: none; }
.pt-body .board-top { align-self: center; color: var(--dim); font-size: 12px; padding: 4px 0; }
.pt-body .board-row { white-space: pre-wrap; }
.pt-body .board-row.user, .pt-body .board-row.answer { align-self: flex-end; max-width: 85%; padding: 5px 9px; background: var(--cream); color: var(--edge);
  box-shadow: 0 -2px 0 var(--edge), 0 2px 0 var(--edge), -2px 0 0 var(--edge), 2px 0 0 var(--edge); }
.pt-body .board-row.answer { background: var(--yellow); }
.pt-body .board-row.question { color: var(--yellow); }
.pt-body .board-row.peer { color: #9cc7f0; }
.pt-body .board-row .from { display: block; font: 700 10px/1.2 ui-monospace, Menlo, monospace; text-transform: uppercase; opacity: .85; }
.pt-body .board-row.tool, .pt-body .board-row.result { font: 11px/1.35 ui-monospace, Menlo, monospace; color: var(--dim);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.pt-body .board-row.result { padding-left: 1.6em; opacity: .9; }
.pt-body .board-row .more { opacity: .55; }
.pt .talk-ask { max-height: 40%; overflow-y: auto; }
.pt-main > .pt-panels { flex: 0 1 auto; min-height: 0; max-height: 40%; overflow-y: auto; }
.pt-panels:empty, .pt-panel:empty { display: none; }
.pt-panel { padding: 8px 12px; border-top: 2px solid var(--edge); }
.pt-input { display: flex; gap: 8px; align-items: flex-end; padding: 8px; background: var(--edge); }
.pt-input textarea { flex: 1; min-width: 0; min-height: 64px; max-height: 40vh; resize: vertical; padding: 6px 8px; border: 0; border-radius: 0;
  background: var(--paper); color: var(--edge); font: inherit; box-shadow: inset 0 0 0 2px var(--edge); }
.pt-input textarea:focus { outline: 2px solid var(--orange); outline-offset: 0; }
.pt .talk-why { color: var(--dim); }
.pt .talk-problem { padding-top: 7px; color: #f2a19b; }
.pt-side { flex: none; width: 204px; display: flex; flex-direction: column; gap: 6px; padding: 8px; border-left: 2px solid var(--edge); overflow-y: auto; }
.pt-side > * { flex: none; }
.pt-office.talk-key { padding: 2px; display: block; }
.pt .pt-picture { display: block; position: static; width: 184px; height: auto; max-width: 100%; image-rendering: pixelated; }
.pt-list-head { font: 700 11px/1 ui-monospace, Menlo, monospace; text-transform: uppercase; letter-spacing: .06em; color: var(--dim); padding: 4px 2px 0; }
.pt-list { display: flex; flex-direction: column; gap: 4px; }
.pt-desk.talk-key { display: flex; gap: 6px; text-transform: none; letter-spacing: 0; font: 600 12px/1.2 -apple-system, system-ui, sans-serif; }
.pt-desk.asking { background: var(--yellow); }
.pt-desk-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pt-desk-state { color: #4b5263; font-weight: 400; }
.pt-none { color: var(--dim); font-style: italic; }
`,

  /** The bubbles' style, put on `doc` once. */
  inject(doc) {
    if (doc.getElementById(TALK_STYLE.ID)) return;
    const style = doc.createElement('style');
    style.id = TALK_STYLE.ID;
    style.textContent = TALK_STYLE.CSS;
    doc.head.append(style);
  },
};
