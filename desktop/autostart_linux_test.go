package desktop

import (
	"os"
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
