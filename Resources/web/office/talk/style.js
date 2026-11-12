/**
 * How the talk bubbles look (1.26, Talk Bubble): the open bubble over a head, the small question bubbles, the hover's
 * tag and the Needs you key, drawn as the office is: hard 2-pixel edges in the bubble's ink on its cream, corners
 * stepped by leaving the edge's corner pixels out, hard shadows, keys in capitals that press 2 pixels down. Reading
 * text stays in the system font at 13 px; the office's pixel font has only capitals and digits. Put on the page once,
 * by the bubble, as a <style> of its own.
 */
const TALK_STYLE = {
  ID: 'talk-bubble-style',

  CSS: `
.talk-bubble, .talk-small, .talk-tag, .talk-needs { --edge: #1b1e2a; --cream: #f4f1ea; --paper: #fffdf8; --screen: #171b26;
  --chalk: #e6e9df; --dim: #8d95a6; --orange: #ff8a1f; --yellow: #ffd34d; --slate: #3e495b;
  position: fixed; z-index: 40; box-sizing: border-box; color: var(--edge); font: 13px/1.4 -apple-system, system-ui, sans-serif; }
.talk-bubble *, .talk-small * { box-sizing: border-box; }
.talk-bubble[hidden], .talk-small[hidden], .talk-tag[hidden], .talk-needs[hidden], .talk-bubble [hidden] { display: none !important; }
.talk-edge { background: var(--cream);
  box-shadow: 0 -2px 0 var(--edge), 0 2px 0 var(--edge), -2px 0 0 var(--edge), 2px 0 0 var(--edge), 0 6px 0 rgba(12, 15, 22, .35); }
.talk-bubble { width: 320px; display: flex; flex-direction: column; }
.talk-bubble > * { flex: none; }
.talk-bubble:focus { outline: none; }
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
.talk-bubble > .talk-lines { flex: 0 1 auto; min-height: 44px; }
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
.talk-ask { display: grid; gap: 6px; padding: 8px 9px; background: #252c3d; color: var(--cream); box-shadow: inset 0 0 0 2px var(--orange); }
.talk-q { color: var(--yellow); font-weight: 600; white-space: pre-wrap; max-height: 7em; overflow: auto; }
.talk-keys { display: flex; flex-wrap: wrap; gap: 6px 8px; padding: 2px; }
.talk-hint { color: var(--dim); font-size: 12px; }
.talk-key { display: inline-flex; align-items: center; gap: 5px; min-height: 24px; min-width: 24px; margin: 2px; padding: 4px 7px; border: 0;
  border-radius: 0; background: var(--cream); color: var(--edge); cursor: pointer; text-align: left;
  font: 700 11px/1.15 ui-monospace, Menlo, monospace; text-transform: uppercase; letter-spacing: .03em;
  box-shadow: 0 -2px 0 var(--edge), 0 2px 0 var(--edge), -2px 0 0 var(--edge), 2px 0 0 var(--edge), inset 0 -2px 0 rgba(0, 0, 0, .18); }
.talk-key:active:not(:disabled) { transform: translateY(2px); box-shadow: 0 -2px 0 var(--edge), 0 2px 0 var(--edge), -2px 0 0 var(--edge), 2px 0 0 var(--edge); }
.talk-key:disabled { opacity: .45; cursor: default; }
.talk-key:focus-visible { outline: 2px solid var(--orange); outline-offset: 3px; }
.talk-key.go { background: var(--orange); }
.talk-key.dark { background: var(--slate); color: var(--cream); }
.talk-key i { font-style: normal; padding: 2px 4px; margin: -2px 0; background: var(--edge); color: var(--cream); }
.talk-input { display: flex; gap: 6px; align-items: center; padding: 7px 8px; }
.talk-input input { flex: 1; min-width: 0; height: 28px; padding: 3px 7px; border: 0; border-radius: 0; background: var(--paper); color: var(--edge);
  font: inherit; box-shadow: inset 0 0 0 2px var(--edge); }
.talk-input input:focus { outline: 2px solid var(--orange); outline-offset: 0; }
.talk-input input:disabled { background: #e3ded2; color: #6b7280; }
.talk-why, .talk-problem { margin: 0; padding: 7px 9px; color: #4b5263; }
.talk-problem { padding-top: 0; color: #a12d2d; }
.talk-tail { position: absolute; width: 14px; height: 9px; background: var(--edge);
  clip-path: polygon(0 0, 100% 0, 100% 22%, 78% 22%, 78% 56%, 57% 56%, 57% 100%, 43% 100%, 43% 56%, 22% 56%, 22% 22%, 0 22%); }
.talk-tail::after { content: ""; position: absolute; left: 2px; top: 0; width: 10px; height: 5px; background: var(--cream);
  clip-path: polygon(0 0, 100% 0, 100% 40%, 70% 40%, 70% 100%, 30% 100%, 30% 40%, 0 40%); }
.talk-small { display: flex; flex-direction: column; justify-content: center; gap: 2px; padding: 2px 5px; cursor: pointer; overflow: hidden;
  font-size: var(--talk-font, 11px); line-height: 1.2; }
.talk-small-q { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.talk-small .talk-keys { flex-wrap: nowrap; gap: 4px; padding: 1px 2px; }
.talk-small .talk-key { min-height: 0; padding: 1px 4px; margin: 1px; font-size: .85em; }
.talk-tag { max-width: 300px; padding: 3px 7px; font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; pointer-events: none; }
.talk-needs { z-index: 39; }
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
