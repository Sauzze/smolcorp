package desktop

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The office page's source, as the app serves it and as the viewer carries it.
func officeSources(t *testing.T) (web, viewer []byte) {
	t.Helper()
	web, err := os.ReadFile("../Resources/web/office.js")
	if err != nil {
		t.Fatal(err)
	}
	viewer, err = os.ReadFile("../viewer/public/office.js")
	if err != nil {
		t.Fatal(err)
	}
	return web, viewer
}

func TestTheViewersOfficePageIsTheAppsOfficePage(t *testing.T) {
	web, viewer := officeSources(t)
	if !bytes.Equal(web, viewer) {
		t.Fatal("viewer/public/office.js differs from Resources/web/office.js: copy one over the other")
	}
}

func TestEscapeAndTheKeyBarsEscPutAwayTheDeskHoldMenu(t *testing.T) {
	web, _ := officeSources(t)
	src := string(web)

	// The Escape handler: after the picker and max-effort cases, it puts away the keys and chips a held desk put out.
	typed := src[strings.Index(src, "const typed = (e) =>"):]
	typed = typed[:strings.Index(typed, "win.document.addEventListener('visibilitychange'")]
	if !strings.Contains(typed, "e.key === 'Escape' && maxing") {
		t.Fatal("the Escape handler no longer answers max effort: update this test")
	}
	if !strings.Contains(typed, "keysFor = null") || !strings.Contains(typed, "held = null") {
		t.Errorf("Escape doesn't put away the held desk's keys and chips:\n%s", typed)
	}

	// The key bar's esc is only sent: the bar stays open, so a double Escape (as Claude Code and /clear use) works.
	press := src[strings.Index(src, "if (target.kind === 'key') {\n        const wanted"):]
	press = press[:strings.Index(press, "return on.key?.(target.action);")]
	if strings.Contains(press, "keysFor = null") || strings.Contains(press, "'esc'") {
		t.Errorf("the key bar's esc must be sent without putting the keys away:\n%s", press)
	}
}

func TestTheRoleChipsUnderAHeldDeskSlideWithTheMouseWheel(t *testing.T) {
	web, viewer := officeSources(t)
	src := string(web)

	// A wheel handler of its own, over the roles strip only, that moves the same offset a finger's drag does.
	at := strings.Index(src, "canvas.addEventListener('wheel', (e) => {\n      if (!rolesAt")
	if at < 0 {
		t.Fatal("no wheel handler for the role chips: a mouse can't reach the roles past the strip's end")
	}
	handler := src[at:]
	handler = handler[:strings.Index(handler, "}, { passive: false });")]
	for _, want := range []string{"box.left", "box.right", "e.preventDefault()", "e.deltaX", "e.deltaY", "roleOffset =", "rolesAt.most"} {
		if !strings.Contains(handler, want) {
			t.Errorf("the role chips' wheel handler lacks %q:\n%s", want, handler)
		}
	}
	if !bytes.Equal(web, viewer) {
		t.Fatal("the two office.js copies differ")
	}
}

func TestTheOfficeAndSetupShowWhyHiringIsOff(t *testing.T) {
	web, viewer := officeSources(t)
	for name, src := range map[string]string{"Resources/web/office.js": string(web), "viewer/public/office.js": string(viewer)} {
		if !strings.Contains(src, "on.hint?.(hold || (typeof hiring.reason === 'string' ? hiring.reason : ''))") {
			t.Errorf("%s: a free desk with hiring unavailable no longer shows hiring.reason as its hint", name)
		}
	}
	page, err := os.ReadFile("../Resources/web/setup.html")
	if err != nil {
		t.Fatal(err)
	}
	setup := string(page)
	if !strings.Contains(setup, "$('hire-setup').hidden = !h.canHire;") {
		t.Error("setup: the folder field must show whenever hiring could be offered (canHire), not only once hiring is on")
	}
	toggle := setup[strings.Index(setup, "$('hiring').onclick"):]
	toggle = toggle[:strings.Index(toggle, "\n")]
	if !strings.Contains(toggle, "showHiringProblem(") || !strings.Contains(setup, "$('hiring-problem').textContent = res.ok ? '' : await res.text()") {
		t.Error("setup: the hiring toggle must show the answer's text, under the step, when its request fails")
	}
}

// styleDressOf gave a project whose hash remainder came out negative list[negative], undefined: nothing of the
// style on. It counts back from the end of the list now, and a name whose remainder was not negative keeps its item.
// The function runs in node on a tiny stand-in for its surroundings; without node the test is skipped.
func TestAStyleDressesEveryNameEvenWhenItsRemainderIsNegative(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: styleDressOf can't be run")
	}
	web, _ := officeSources(t)
	src := string(web)
	at := strings.Index(src, "function styleDressOf(seed) {")
	if at < 0 {
		t.Fatal("styleDressOf is gone: update this test")
	}
	fn := src[at:]
	fn = fn[:strings.Index(fn, "\n  }\n")+4]
	script := `
const decor = { preset: 'p' };
const STYLE_DRESS = { p: ['a', 'b', 'c', 'd', 'e', 'f', 'g'] };
` + fn + `
let negative = 0, other = 0;
for (let n = 0; n < 2000; n++) {
  const seed = 'project-' + n;
  let h = 0;
  for (const c of seed) h = Math.imul(h ^ c.charCodeAt(0), 0x5bd1e995) >>> 0;
  const r = (h ^ (h >>> 13)) % 7; // what the old code indexed with
  const got = styleDressOf(seed);
  if (got === undefined || got === null) throw new Error(seed + ' is not dressed');
  if (r < 0) { negative++; if (got !== STYLE_DRESS.p[r + 7]) throw new Error(seed + ' not counted back from the end'); }
  else { other++; if (got !== STYLE_DRESS.p[r]) throw new Error(seed + ' changed suit'); }
}
if (!negative || !other) throw new Error('the names covered one sign only');
`
	dir := t.TempDir()
	file := filepath.Join(dir, "dress.js")
	if err := os.WriteFile(file, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, file).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}

// A style that dresses each person differently puts someone named after one of its own outfits in that one, whatever the
// case of the name: a Grok bot named RUBY, in the Geode (`geode`), wears `geoderuby`. Anyone else is dressed by the hash,
// as before, and a style whose outfits aren't named after it, as the Hall of Armor's `ironman…` aren't, dresses nobody by
// name. Runs styleDressOf in node, as the test above does; without node it's skipped.
func TestSomeoneNamedAfterOneOfAStylesOutfitsWearsIt(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: styleDressOf can't be run")
	}
	web, _ := officeSources(t)
	src := string(web)
	at := strings.Index(src, "function styleDressOf(seed) {")
	if at < 0 {
		t.Fatal("styleDressOf is gone: update this test")
	}
	fn := src[at:]
	fn = fn[:strings.Index(fn, "\n  }\n")+4]
	script := `
let decor = { preset: 'geode' };
const STYLE_DRESS = { geode: ['geoderuby', 'geodeopal', 'geodemoonstone', 'geodejade'], ironmanhall: ['ironmanmk1', 'ironmanmk2', 'ironmanrescue'] };
` + fn + `
const hashed = (seed, list) => {
  let h = 0;
  for (const c of seed) h = Math.imul(h ^ c.charCodeAt(0), 0x5bd1e995) >>> 0;
  const i = (h ^ (h >>> 13)) % list.length;
  return list[i < 0 ? i + list.length : i];
};
for (const [seed, want] of [['RUBY', 'geoderuby'], ['Opal', 'geodeopal'], ['moonstone', 'geodemoonstone'], ['JADE', 'geodejade']]) {
  if (styleDressOf(seed) !== want) throw new Error(seed + ' wears ' + styleDressOf(seed) + ', not ' + want);
}
for (const seed of ['api-server', 'ONYX', 'RUBY 2', 'ruby-bot', 'geoderuby']) {
  if (styleDressOf(seed) !== hashed(seed, STYLE_DRESS.geode)) throw new Error(seed + ' is no longer dressed by the hash');
}
decor = { preset: 'ironmanhall' };
for (const seed of ['mk1', 'ironmanmk1', 'rescue', 'MK2']) {
  if (styleDressOf(seed) !== hashed(seed, STYLE_DRESS.ironmanhall)) throw new Error(seed + ' is dressed by name in a style whose outfits are not named after it');
}
`
	dir := t.TempDir()
	file := filepath.Join(dir, "namesake.js")
	if err := os.WriteFile(file, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, file).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
