package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// shortPathOf is the disk's 8.3 short name of a path that's there, or the path itself when the disk gives none.
func shortPathOf(t *testing.T, path string) string {
	t.Helper()
	long, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetShortPathName(long, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		t.Fatalf("GetShortPathName %s: %v", path, err)
	}
	return windows.UTF16ToString(buf[:n])
}

// FXDEMOWIN (CI run 44079051196): on this computer's disk, a short (8.3) name and its long one are the same place to
// the demo's fence, as the stand-in disk shows on any computer (TestTheDemosFenceReadsAShortNameAsItsLongOne). The
// CI runner's temp folder is C:\Users\RUNNER~1\…, which the hooks' shellPath writes.
func TestOnWindowsTheDemosFenceReadsShortNamesOnTheDisk(t *testing.T) {
	in := demoTestFolder(t)
	root := filepath.Join(in, "smolcorp-demo-long-name")
	repo := demoCheckout(t, root)
	if err := os.MkdirAll(filepath.Join(repo, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".claude", "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	shortRepo, shortClaude := shortPathOf(t, repo), shortPathOf(t, filepath.Join(repo, ".claude"))
	if !strings.Contains(shortRepo, "~") || !strings.Contains(filepath.Base(shortClaude), "~") {
		t.Skipf("this disk gives no 8.3 short names here (%s, %s)", shortRepo, shortClaude)
	}

	// The disk's long names, and both sides of the fence by them.
	if got, err := longPathHere(filepath.Join(shortRepo, "new", "file.txt")); err != nil || !strings.EqualFold(got, filepath.Join(repo, "new", "file.txt")) {
		t.Errorf("the long name of %s: %q %v, want %s", shortRepo, got, err, repo)
	}
	f, ok := demoFence{root: shortPathOf(t, root), roles: []string{"tester"}, home: homeDir}.withLongNames()
	if !ok || !strings.EqualFold(f.root, root) {
		t.Fatalf("the fence's folder %q (%v), want %s", f.root, ok, root)
	}

	call := func(name string, input map[string]any) demoCall {
		data, _ := json.Marshal(input)
		return demoCall{ToolName: name, ToolInput: data, Cwd: repo}
	}
	slashed := filepath.ToSlash(shortRepo)
	for _, c := range []demoCall{
		call("mcp__smolcorp__hire", map[string]any{"role": "tester", "folder": shortRepo}),
		bashCall("smolcorp gm hire --role tester --folder '"+slashed+"' --worktree --task 'fix it'", repo),
		call("Write", map[string]any{"file_path": filepath.Join(shortRepo, "notes.md")}),
	} {
		if why := f.check(c); why != "" {
			t.Errorf("refused %s %s: %s", c.ToolName, c.ToolInput, why)
		}
	}
	for _, c := range []demoCall{
		call("Write", map[string]any{"file_path": filepath.Join(shortClaude, "settings.json")}),
		call("Write", map[string]any{"file_path": filepath.Join(repo, filepath.Base(shortClaude), "settings.json")}),
		call("Edit", map[string]any{"file_path": filepath.Join(filepath.Base(shortClaude), "settings.json")}),
		// Outside quotes, the ~ of a short name is zsh's glob to the guard: written in quotes, or long.
		bashCall("smolcorp gm hire --role tester --folder "+slashed+" --worktree", repo),
	} {
		if why := f.check(c); !strings.HasPrefix(why, demoFenceRefusal) {
			t.Errorf("let through %s %s", c.ToolName, c.ToolInput)
		}
	}
}
