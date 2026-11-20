package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestALoginEntryUnderTheOldNameMovesToTheNewOne(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeFile(t, oldAutostartFile(), "[Desktop Entry]\nName=SauzzeDeck Pro\n")
	if !isOpenAtLogin() {
		t.Fatal("an entry under the old name doesn't count")
	}
	moveOpenAtLogin()
	if _, err := os.Stat(oldAutostartFile()); !os.IsNotExist(err) {
		t.Error("the old entry is still there, so the app would start twice")
	}
	entry, err := os.ReadFile(autostartFile())
	if err != nil || !strings.Contains(string(entry), "Name=Smolcorp\n") {
		t.Errorf("the new entry is %q, %v", entry, err)
	}
	setOpenAtLogin(false)
	if isOpenAtLogin() {
		t.Error("turning it off left an entry")
	}
}

// loginHome points HOME, XDG_CONFIG_HOME and XDG_DATA_HOME at temporary folders.
func loginHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	return home
}

func TestTheExecLineIsQuotedAsADesktopEntryAsksFor(t *testing.T) {
	for program, want := range map[string]string{
		`/opt/smol corp/smolcorp`: `"/opt/smol corp/smolcorp"`,
		`/opt/a"b/smolcorp`:       `"/opt/a\"b/smolcorp"`,
		`/opt/$HOME/smolcorp`:     `"/opt/\$HOME/smolcorp"`,
		"/opt/a`b/smolcorp":       "\"/opt/a\\`b/smolcorp\"",
		`/opt/a\b/smolcorp`:       `"/opt/a\\\\b/smolcorp"`,
		`/opt/100%/smolcorp`:      `"/opt/100%%/smolcorp"`,
	} {
		if got := execQuote(program); got != want {
			t.Errorf("execQuote(%q) = %s, want %s", program, got, want)
		}
	}
}

func TestTheLoginEntryUsesTheInstalledIconWhenThereIsOne(t *testing.T) {
	home := loginHome(t)
	setOpenAtLogin(true)
	entry, _ := os.ReadFile(autostartFile())
	if !strings.Contains(string(entry), "Icon=smolcorp\n") {
		t.Errorf("without an installed icon the entry is %q", entry)
	}
	icon := filepath.Join(home, ".local", "share", "smolcorp", "icon.png")
	writeFile(t, icon, "png")
	setOpenAtLogin(true)
	entry, _ = os.ReadFile(autostartFile())
	if !strings.Contains(string(entry), "Icon="+icon+"\n") {
		t.Errorf("with an installed icon the entry is %q", entry)
	}
}

func TestAStalePortableLoginEntryIsRewrittenToTheInstalledCopy(t *testing.T) {
	home := loginHome(t)
	installed := filepath.Join(home, ".local", "bin", "smolcorp")
	writeFile(t, installed, "x")
	writeFile(t, autostartFile(), "[Desktop Entry]\nType=Application\nName=Smolcorp\nExec=\"/tmp/portable/smolcorp\"\nIcon=smolcorp\n")
	moveOpenAtLogin()
	entry, _ := os.ReadFile(autostartFile())
	if !strings.Contains(string(entry), "Exec="+execQuote(installed)+"\n") || strings.Contains(string(entry), "portable") {
		t.Errorf("the entry is still %q", entry)
	}
}

func TestACorrectOrPortableOnlyLoginEntryIsLeftAlone(t *testing.T) {
	home := loginHome(t)
	portable := "[Desktop Entry]\nName=Smolcorp\nExec=\"/tmp/portable/smolcorp\"\nCustom=1\n"
	writeFile(t, autostartFile(), portable)
	moveOpenAtLogin() // nothing installed: nothing to move to
	if entry, _ := os.ReadFile(autostartFile()); string(entry) != portable {
		t.Errorf("an entry with no installed copy changed to %q", entry)
	}
	installed := filepath.Join(home, ".local", "bin", "smolcorp")
	writeFile(t, installed, "x")
	correct := "[Desktop Entry]\nName=Smolcorp\nExec=" + execQuote(installed) + "\nCustom=1\n"
	writeFile(t, autostartFile(), correct)
	moveOpenAtLogin()
	if entry, _ := os.ReadFile(autostartFile()); string(entry) != correct {
		t.Errorf("a correct entry changed to %q", entry)
	}
}

// staleEntry runs moveOpenAtLogin on an entry with an installed copy present
// and returns what the entry was, and what it is after.
func staleEntry(t *testing.T, entry string) (installed, before, after string) {
	t.Helper()
	home := loginHome(t)
	installed = filepath.Join(home, ".local", "bin", "smolcorp")
	writeFile(t, installed, "x")
	before = strings.ReplaceAll(entry, "INSTALLED", installed)
	writeFile(t, autostartFile(), before)
	moveOpenAtLogin()
	got, _ := os.ReadFile(autostartFile())
	return installed, before, string(got)
}

func TestADisabledStaleLoginEntryIsNotSwitchedBackOn(t *testing.T) {
	for _, off := range []string{"X-GNOME-Autostart-enabled=false", "Hidden=true"} {
		_, before, after := staleEntry(t, "[Desktop Entry]\nName=Smolcorp\nExec=\"/tmp/portable/smolcorp\"\n"+off+"\n")
		if after != before {
			t.Errorf("with %s the entry changed from %q to %q", off, before, after)
		}
	}
}

func TestAStaleLoginEntryKeepsEveryOtherLine(t *testing.T) {
	installed, _, after := staleEntry(t, "[Desktop Entry]\n# mine\nName=Smolcorp\nComment=mine\nExec=\"/tmp/portable/smolcorp\"\nX-GNOME-Autostart-Delay=30\nIcon=custom\nX-GNOME-Autostart-enabled=true\n")
	want := "[Desktop Entry]\n# mine\nName=Smolcorp\nComment=mine\nExec=" + execQuote(installed) + "\nX-GNOME-Autostart-Delay=30\nIcon=custom\nX-GNOME-Autostart-enabled=true\n"
	if after != want {
		t.Errorf("the entry is %q, want %q", after, want)
	}
}

func TestAHandEditedLoginEntryIsLeftAlone(t *testing.T) {
	for _, exec := range []string{
		`"/usr/local/bin/my-wrapper" "/tmp/portable/smolcorp"`,
		`"/tmp/portable/smolcorp" --minimized`,
		`/home/me/start-smolcorp.sh`,
		`env FOO=1 /tmp/portable/smolcorp`,
	} {
		_, before, after := staleEntry(t, "[Desktop Entry]\nName=Smolcorp\nExec="+exec+"\n")
		if after != before {
			t.Errorf("the hand-edited Exec %s was changed to %q", exec, after)
		}
	}
}

func TestAnOldQuotedLoginEntryOfTheInstalledCopyHasItsQuotingFixed(t *testing.T) {
	home := filepath.Join(t.TempDir(), "a$b")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	installed := filepath.Join(home, ".local", "bin", "smolcorp")
	writeFile(t, installed, "x")
	// Go's %q left the $ bare, which a desktop would read as the start of a variable.
	writeFile(t, autostartFile(), "[Desktop Entry]\nExec=\""+installed+"\"\nHidden=false\n")
	moveOpenAtLogin()
	want := "[Desktop Entry]\nExec=" + execQuote(installed) + "\nHidden=false\n"
	if entry, _ := os.ReadFile(autostartFile()); string(entry) != want {
		t.Errorf("the entry is %q, want %q", entry, want)
	}
}

// RWTC N3: a test copy never writes the owner's autostart entry, at start or when asked.
func TestATestCopyLeavesTheLoginEntryAlone(t *testing.T) {
	loginHome(t)
	writeFile(t, oldAutostartFile(), "[Desktop Entry]\nName=SauzzeDeck Pro\n")
	onPort(t, 7799)
	moveOpenAtLogin()
	setOpenAtLogin(true)
	if _, err := os.Stat(oldAutostartFile()); err != nil {
		t.Error("the old entry was moved")
	}
	if _, err := os.Stat(autostartFile()); !os.IsNotExist(err) {
		t.Error("an entry was written for the test copy")
	}
	setOpenAtLogin(false)
	if _, err := os.Stat(oldAutostartFile()); err != nil {
		t.Error("the owner's entry was removed")
	}
}
