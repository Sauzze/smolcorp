package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// testSettings is a stand-in desk settings file named path, never written: for a plan, which only names it.
func testSettings(path string) sealedSettings {
	return sealedSettings{Path: path, Sum: strings.Repeat("a", len(guard.SettingsSumBlank))}
}

// writtenTestSettings writes text, which holds guard.SettingsSumBlank, as a sealed settings file in a temp
// folder, as writeHireSettings does.
func writtenTestSettings(t *testing.T, text string) sealedSettings {
	t.Helper()
	sum, ok := guard.SettingsSum([]byte(text))
	if !ok {
		t.Fatal("the stand-in settings hold no blank for their seal")
	}
	path := filepath.Join(t.TempDir(), "hires", "x", hireSettingsFile)
	if err := writeSettingsFile(path, []byte(strings.ReplaceAll(text, guard.SettingsSumBlank, sum))); err != nil {
		t.Fatal(err)
	}
	return sealedSettings{Path: path, Sum: sum}
}

// fileSettings are the settings in the file a start names (--settings=<file>), "" if it names none or can't be read.
func fileSettings(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}
