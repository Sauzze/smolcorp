package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// FXMEMKEEPR1: the pages' side of the owner's memory routes, run in node on the pages' own functions with a small
// stand-in for the DOM. Without node the rows are skipped.

// memoryPagePart is the part of the page file from from up to and including to, for a row that runs it in node.
func memoryPagePart(t *testing.T, file, from, to string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "Resources", "web", file))
	if err != nil {
		t.Fatal(err)
	}
	page := string(data)
	start := strings.Index(page, from)
	end := strings.Index(page[max(start, 0):], to)
	if start < 0 || end < 0 {
		t.Fatalf("%s has no %q", file, from)
	}
	return page[start : start+end+len(to)]
}

// memoryNode runs script in node and gives what it printed, trimmed.
func memoryNode(t *testing.T, script string) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: the page's own functions can't be run")
	}
	out, err := exec.Command(node, "-e", script).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// memoryPageDOM stands in for the DOM Setup's memory cards are built with: elements that keep their children, and
// what each shows as text.
const memoryPageDOM = `
class El {
  constructor(tag) { this.tag = tag; this.children = []; this.dataset = {}; this.attrs = {}; this.textContent = ''; this.className = ''; }
  append(...c) { this.children.push(...c); }
  replaceWith(x) { replaced = x; }
  focus() {}
  setAttribute(k, v) { this.attrs[k] = v; }
}
let replaced = null;
const document = { createElement: (tag) => new El(tag) };
const texts = (e) => typeof e === 'string' ? [e] : [e.textContent, ...e.children.flatMap(texts)].filter(Boolean);
const buttons = (e) => typeof e === 'string' ? [] : [...(e.tag === 'button' ? [e.textContent] : []), ...e.children.flatMap(buttons)];
const ROTATE_WAIT = 0;
`

// memoryCardScript is Setup's own card and edit form, from MEMORY_TYPES to the end of memoryEdit.
func memoryCardScript(t *testing.T) string {
	return memoryPageDOM +
		memoryPagePart(t, "setup.html", "  const MEMORY_TYPES = {", "\n") +
		memoryPagePart(t, "setup.html", "  /** Where a note's words came from", "\n    inputs.name.focus();\n  }\n")
}

// O5: in a browser's Setup on a Mac (state.windowOnly), a note's card has no Keep, Edit, Drop or Forget, only the
// page's own note; in Smolcorp's own window, and off a Mac, the buttons are there.
func TestSetupsMemoryCardsHaveNoButtonsInABrowserOnAMac(t *testing.T) {
	const windowOnly = "Change this in Smolcorp's own Settings window, not in a browser."
	script := memoryCardScript(t) + `
let state = {};
const waiting = { id: 'a1', name: 'Docs folder', type: 'reference', description: 'Where the docs are', body: 'In docs.', from: 'seen', said: 'ls docs', shown: 'h' };
const kept = { id: 'k1', name: 'Kept fact', type: 'project', description: 'A fact', body: 'A body.', from: 'owner' };
const out = {};
out.window = [buttons(memoryCard(waiting, true)), buttons(memoryCard(kept, false))];
state = { windowOnly: ` + "`" + windowOnly + "`" + ` };
const looking = memoryCard(waiting, true);
out.browser = [buttons(looking), buttons(memoryCard(kept, false)), texts(looking).includes(state.windowOnly), texts(memoryCard(kept, false)).includes(state.windowOnly)];
console.log(JSON.stringify(out));`
	want := `{"window":[["Keep","Edit","Drop"],["Forget"]],"browser":[[],[],true,true]}`
	if got := memoryNode(t, script); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// O2: the edit form shows why a note is held, as its card does, so the owner edits with the reason in sight.
func TestSetupsEditFormShowsWhyANoteIsHeld(t *testing.T) {
	script := memoryCardScript(t) + `
let state = {};
const held = 'Look at this one closely: the quote leaves out a "not".';
const n = { id: 'a1', name: 'Tests run with pnpm', type: 'feedback', description: 'Which tool', rule: 'Run with pnpm', why: 'Said so', how: 'Always', from: 'terminal', said: 'do not use npm', shown: 'h', held };
const card = memoryCard(n, true);
memoryEdit(card, n);
const form = replaced;
const plain = { ...n, held: undefined };
memoryEdit(memoryCard(plain, true), plain);
console.log(JSON.stringify([form.tag, texts(form).includes(held), buttons(form), texts(replaced).includes(held)]));`
	want := `["form",true,["Keep my version","Cancel"],false]`
	if got := memoryNode(t, script); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// O4: once the computer refuses the office's badge, nothing asks again: not the timer, not the window coming back in
// sight. A badge that couldn't reach the computer still asks again.
func TestTheBadgeAsksNoMoreOnceRefused(t *testing.T) {
	script := `
let preview = false, asked = 0, answers = [null, { ok: false }];
const document = { hidden: false };
const badge = { hidden: false, textContent: '' };
const $ = () => badge, clearInterval = () => {};
const get = async () => { asked++; return answers.shift() || { ok: true, json: async () => ({ waiting: 3 }) }; };
` + memoryPagePart(t, "deck.html", "  const MEMORY_EVERY = 30_000;", "\n  }\n") + `
(async () => {
  await checkMemory(); // out of reach: asks again later
  await checkMemory(); // refused
  await checkMemory(); // the timer, had it fired once more
  await checkMemory(); // the window back in sight
  console.log(JSON.stringify([asked, badge.hidden]));
})();`
	if got := memoryNode(t, script); got != `[2,true]` {
		t.Errorf("got %s, want [2,true]: asked twice, the badge hidden", got)
	}
}

// FXMEMKEEPR2 O3: a waiting note that Smolcorp's checks refuse has no Keep, which the computer would refuse, only Edit
// and Drop, as its card says; its edit form says why it's refused. A note they don't refuse keeps its Keep, and its
// form says nothing of a refusal.
func TestSetupsRefusedNoteOffersNoKeepAndItsEditFormSaysWhy(t *testing.T) {
	script := memoryCardScript(t) + `
let state = {};
const refused = 'the “Note” field has an e-mail address: memory never keeps personal data';
const n = { id: 'a1', name: 'Docs folder', type: 'reference', description: 'Where the docs are', body: 'Write to ada@example.com.', from: 'seen', said: 'ls docs', shown: 'h', refused };
const plain = { ...n, refused: undefined };
const card = memoryCard(n, true);
const out = [buttons(card), buttons(memoryCard(plain, true))];
memoryEdit(card, n);
out.push(texts(replaced).includes("Smolcorp's checks refuse this note as it is (" + refused + ")."), buttons(replaced));
memoryEdit(memoryCard(plain, true), plain);
out.push(texts(replaced).some((text) => text.includes('refuse')));
console.log(JSON.stringify(out));`
	want := `[["Edit","Drop"],["Keep","Edit","Drop"],true,["Keep my version","Cancel"],false]`
	if got := memoryNode(t, script); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}
