package agentdeck

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every file of Resources/web is built in, but the sources office.js is built
// from: a new kind of file there needs its pattern in web.go.
func TestFilesHoldResourcesWebButOfficesSources(t *testing.T) {
	err := filepath.WalkDir("Resources/web", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name := filepath.ToSlash(path)
		_, missing := fs.Stat(Files, name)
		switch source := strings.HasPrefix(name, "Resources/web/office/"); {
		case source && missing == nil:
			t.Errorf("%s is built into the app: only office.js, its build, belongs there", name)
		case !source && missing != nil && !strings.HasPrefix(entry.Name(), "."):
			t.Errorf("%s isn't built into the app: add its pattern to web.go", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("Resources/web/office/main.js"); err != nil {
		t.Fatal("the office's sources weren't found:", err)
	}
	for _, name := range []string{"Resources/web/office.js", "Resources/web/setup.html", "Resources/web/sounds/keys.m4a", "Resources/opencode-plugin.js"} {
		if _, err := fs.Stat(Files, name); err != nil {
			t.Errorf("%s isn't built in: %v", name, err)
		}
	}
}
