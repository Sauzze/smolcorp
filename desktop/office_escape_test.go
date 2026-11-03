package desktop

import (
	"bytes"
	"os"
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
