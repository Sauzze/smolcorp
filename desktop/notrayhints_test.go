package desktop

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// SIERRA 1.24 F-2: with no tray the only way to a new setup link is to stop Smolcorp and start it again, because
// no command makes one and a second start only raises the office. Every hint says that, in one wording.
const wantNoTrayNewLink = "run smolcorp quit (or press Ctrl+C in its terminal), then start Smolcorp again"

func TestNoTrayNewLinkWordingIsOne(t *testing.T) {
	if noTrayNewLink != wantNoTrayNewLink {
		t.Fatalf("noTrayNewLink is %q, want %q", noTrayNewLink, wantNoTrayNewLink)
	}
	isolate(t)
	d := newTestDeck(t)
	var link bytes.Buffer
	printSetupLink(&link, d)
	if want := "For a new one, choose Open Setup… from the tray icon; with no tray, " + wantNoTrayNewLink + "."; !strings.Contains(link.String(), want) {
		t.Errorf("setup link text %q lacks %q", link.String(), want)
	}
	if want := "For a new setup link, " + wantNoTrayNewLink + "."; !strings.Contains(noTrayMessage, want) {
		t.Errorf("no-tray message %q lacks %q", noTrayMessage, want)
	}
	defer func(was string) { setupSystem = was }(setupSystem)
	setupSystem = "linux"
	var open bytes.Buffer
	setupClient{}.openSetup(&open)
	if want := "With no tray, use the link smolcorp prints as it starts; for a new one, " + wantNoTrayNewLink + "."; !strings.Contains(open.String(), want) {
		t.Errorf("setup command text %q lacks %q", open.String(), want)
	}
	if !strings.Contains(setupMenuWords(), "(for a new one, "+wantNoTrayNewLink+")") {
		t.Errorf("ready line words %q lack the no-tray way", setupMenuWords())
	}
	if want := "with no tray, for a new link " + wantNoTrayNewLink; !strings.Contains(setupAdvice("linux"), want) {
		t.Errorf("linux advice %q lacks %q", setupAdvice("linux"), want)
	}
}

// Windows always has a tray, so its words keep the tray; none of Linux's or Windows's text offers a command
// for a new setup link, or a bare second start.
func TestNoLinuxOrWindowsTextOffersACommandOrASecondStartForANewLink(t *testing.T) {
	isolate(t)
	d := newTestDeck(t)
	var link bytes.Buffer
	printSetupLink(&link, d)
	texts := map[string]string{"setup link": link.String(), "no-tray message": noTrayMessage}
	for _, goos := range []string{"linux", "windows"} {
		goos := goos
		func() {
			defer func(was string) { setupSystem = was }(setupSystem)
			setupSystem = goos
			var open bytes.Buffer
			setupClient{}.openSetup(&open)
			texts[goos+" openSetup"] = open.String()
			texts[goos+" menu words"] = setupMenuWords()
		}()
		texts[goos+" advice"] = setupAdvice(goos)
	}
	for name, text := range texts {
		if strings.Contains(text, "setup --open") || strings.Contains(text, "smolcorp-cli") {
			t.Errorf("%s offers a command: %q", name, text)
		}
		// "start Smolcorp again" is for the office in one line of the no-tray message; for a link it only follows a quit.
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, "To open the office") {
				continue
			}
			for _, again := range []string{"start Smolcorp again", "start it again"} {
				if i := strings.Index(line, again); i >= 0 && !strings.Contains(line[:i], "quit") && !strings.Contains(line[:i], "Ctrl+C") {
					t.Errorf("%s says %q for a new link without stopping first: %q", name, again, line)
				}
			}
		}
	}
	if strings.Contains(texts["windows openSetup"], "no tray") || strings.Contains(texts["windows advice"], "Ctrl+C") {
		t.Errorf("Windows always has a tray, but its text names none: %q %q", texts["windows openSetup"], texts["windows advice"])
	}
}

// The quit messages that say to use the tray also give the no-tray way, except Windows, which always has one.
func TestQuitMessagesGiveTheNoTrayWayOnLinux(t *testing.T) {
	src, err := os.ReadFile("quit.go")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(src), "quit it from its tray menu, or with no tray press Ctrl+C in its terminal"); n != 1 {
		t.Errorf("the reused-pid message gives the no-tray way %d times, want 1", n)
	}
	if n := strings.Count(string(src), "Quit it from its tray menu, or with no tray press Ctrl+C in its terminal."); n != 1 {
		t.Errorf("the still-running message gives the no-tray way %d times, want 1", n)
	}
	if !strings.Contains(string(src), `"On Windows, quit Smolcorp from its tray icon."`) {
		t.Error("Windows's message no longer says to use the tray")
	}
	install, err := os.ReadFile("../packaging/linux/install.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(install), "quit it from its tray menu (with no tray, press Ctrl+C in its terminal)") {
		t.Error("the Linux installer's message lacks the no-tray way")
	}
}

// The shared pages and the first-setup recipe, read from the source as the repo's other setup.html tests do.
func TestNoTrayHintsInThePagesAndTheRecipe(t *testing.T) {
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	pair, setup, first := read("../Resources/web/pair.html"), read("../Resources/web/setup.html"), read("../packaging/FIRST-SETUP.md")
	for name, text := range map[string]string{"pair.html": pair, "setup.html": setup} {
		if strings.Contains(text, "setup --open") {
			t.Errorf("%s still offers setup --open, which makes no link", name)
		}
	}
	for _, want := range []string{
		"click the Smolcorp icon in the menu bar or tray and choose <b>Pair a Phone…</b>. With no tray, open the setup link Smolcorp printed as it started " +
			"(for a new one, run <code>smolcorp quit</code> or press Ctrl+C in its terminal, then start Smolcorp again) and choose <b>Pair a phone</b>.",
	} {
		if !strings.Contains(pair, want) {
			t.Errorf("pair.html lacks %q", want)
		}
	}
	for _, want := range []string{
		// The Mac's text is unchanged; only the no-tray part is new.
		`"This page opens from Smolcorp's menu bar: choose Open Settings in Browser. "`,
		`Open Setup… in a browser). With no tray, use the link Smolcorp prints as it starts; "`,
		"for a new one, " + wantNoTrayNewLink + ". '",
		"with no tray, \"\n      + \"" + wantNoTrayNewLink + ".\")) return;",
		// The portable office's hint: the tray is one way, "with a tray"; the Mac's text is as it was.
		"'Off. Turn it on here, or, with a tray, with Going away: launch portable office in the tray.'",
		"'Off. Turn it on here, or with Going away: launch portable office in the menu bar.'",
		// Chosen by the Mac's own test, as the Mac's state names a launcher too since 1.25.0 (M7MENU).
		"detail = v.problem || (state && !onMac()\n",
	} {
		if !strings.Contains(setup, want) {
			t.Errorf("setup.html lacks %q", want)
		}
	}
	step := first[strings.Index(first, "**3. Open the setup page**"):strings.Index(first, "**4. Report**")]
	for _, want := range []string{"No command makes a setup link", "don't run\n`setup --open` for one", "`smolcorp quit`", "Ctrl+C in its terminal"} {
		if !strings.Contains(step, want) {
			t.Errorf("FIRST-SETUP.md step 3 lacks %q", want)
		}
	}
}
