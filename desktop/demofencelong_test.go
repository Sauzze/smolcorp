package desktop

import (
	"encoding/json"
	"errors"
	"runtime"
	"strings"
	"testing"
)

// shortNameGuardRefusal is the demo fence's refusal, on Windows, of a Bash line that names the demo's folder by its
// short name, SMOLCO~1 (FXWINCITESTS1, FXCITRIAGE2's W3). The guard reads each part shaped like a short name as each
// name it keeps that the part may stand for (GUARDPS1, FXGUARDPS1R1 B4): SMOLCO~1 may be SMOLCORP_GM_TOKEN, which is
// always refused, before the fence reads the line's paths. It's fail-closed and costs the demo nothing: the GM is told
// its folder's long name (demoShellFolder). Only a Windows guard reads short names.
const shortNameGuardRefusal = demoFenceRefusal + "Refused by Smolcorp's guard (guard: SMOLCORP_GM_TOKEN)"

// guardReadsAsItsToken says this computer's guard reads a part of a Bash line as SMOLCORP_GM_TOKEN's short name: on
// Windows, a part that starts SMOLCO~ (shortNameGuardRefusal).
func guardReadsAsItsToken(line string) bool {
	return runtime.GOOS == "windows" && strings.Contains(strings.ToUpper(line), "SMOLCO~")
}

// FXDEMOWIN: on Windows, a short (8.3) name and its long one are the same place to the demo's fence, both sides
// made long before they're compared, and a short name never dodges a refusal: one the disk can't tell is refused.
// Read by Windows' rules on any computer, with a stand-in disk for the long names (fencePaths.long).
func TestTheDemosFenceReadsAShortNameAsItsLongOne(t *testing.T) {
	const home = `C:\Users\Ada Lovelace`
	// The stand-in disk's short names, each under the long name of the folder it's in (as GetLongPathName walks a
	// path), and one it can't answer for (LOCKED~1, a folder it may not list).
	pairs := [][2]string{
		{`C:\Users\ADALOV~1`, home},
		{home + `\SMOLCO~1`, home + `\smolcorp-demo`},
		{home + `\smolcorp-demo\app\CLAUDE~1`, home + `\smolcorp-demo\app\.claude`},
		{home + `\smolcorp-demo\app\GIT~1`, home + `\smolcorp-demo\app\.git`},
		{home + `\AppData\Roaming\SMOLCO~1`, home + `\AppData\Roaming\Smolcorp`},
	}
	long := func(name string) (string, error) {
		name = strings.ReplaceAll(name, "/", `\`)
		for _, pair := range pairs {
			short := pair[0]
			if len(name) >= len(short) && strings.EqualFold(name[:len(short)], short) && (len(name) == len(short) || name[len(short)] == '\\') {
				name = pair[1] + name[len(short):]
			}
		}
		if strings.Contains(strings.ToUpper(name), `\LOCKED~1`) {
			return "", errors.New("access is denied")
		}
		return name, nil
	}
	tool := func(name, key, value string) demoCall {
		input, _ := json.Marshal(map[string]string{key: value})
		return demoCall{ToolName: name, ToolInput: input}
	}
	write := func(path string) demoCall { return tool("Write", "file_path", path) }
	read := func(path string) demoCall { return tool("Read", "file_path", path) }
	bash := func(command string) demoCall { return bashCall(command, "") }

	// Both sides: the fence started with its folders by their short names reads them long.
	short := demoFence{root: `C:\Users\ADALOV~1\SMOLCO~1`, support: `C:\Users\ADALOV~1\AppData\Roaming\SMOLCO~1`,
		roles: []string{"tester"}, home: home, goos: "windows", long: long}
	f, ok := short.withLongNames()
	if !ok || f.root != home+`\smolcorp-demo` || f.support != home+`\AppData\Roaming\Smolcorp` {
		t.Fatalf("the fence's folders by their long names: %q %q %v", f.root, f.support, ok)
	}
	if _, ok := (demoFence{root: home + `\LOCKED~1\smolcorp-demo`, goos: "windows", long: long}).withLongNames(); ok {
		t.Error("a demo folder whose long name can't be told was taken")
	}
	cwd := home + `\smolcorp-demo\app`
	allowed := []demoCall{
		write(`C:\Users\ADALOV~1\smolcorp-demo\app\bill.py`),
		write(`C:/Users/ADALOV~1/SMOLCO~1/app/notes.md`),
		write(`C:\Users\Ada Lovelace\SMOLCO~1\app\docs\x.md`),
		read(`C:\Users\ADALOV~1\SMOLCO~1\app\bill.py`),
		bash("cat 'C:/Users/ADALOV~1/smolcorp-demo/app/notes.md'"),
	}
	// SMOLCO~1 in a Bash line: the guard's on Windows (FXWINCITESTS1, FXCITRIAGE2's W3), the fence's elsewhere.
	const byShortNameLine = "cat 'C:/Users/ADALOV~1/SMOLCO~1/app/notes.md'"
	byShortName := bash(byShortNameLine)
	if byShortName.Cwd = cwd; guardReadsAsItsToken(byShortNameLine) {
		if why := f.check(byShortName); !strings.HasPrefix(why, shortNameGuardRefusal) {
			t.Errorf("on Windows, %s: %q, want the guard's refusal", byShortNameLine, why)
		}
	} else {
		allowed = append(allowed, byShortName)
	}
	for _, c := range allowed {
		if c.Cwd = cwd; f.check(c) != "" {
			t.Errorf("refused %s %s: %s", c.ToolName, c.ToolInput, f.check(c))
		}
	}
	for _, c := range []demoCall{
		// The fence's own folders, by their short names.
		write(`C:\Users\ADALOV~1\SMOLCO~1\app\CLAUDE~1\settings.json`),
		write(home + `\smolcorp-demo\app\GIT~1\hooks\pre-push`),
		write(`CLAUDE~1\settings.json`),
		// The app's own folder, by a short name.
		read(home + `\AppData\Roaming\SMOLCO~1\registry.json`),
		read(`C:\Users\ADALOV~1\AppData\Roaming\SMOLCO~1\hires\h1\BRIEF.md`),
		bash("cat 'C:/Users/ADALOV~1/AppData/Roaming/SMOLCO~1/registry.json'"),
		tool("Glob", "pattern", `C:\Users\ADALOV~1\AppData\Roaming\SMOLCO~1\*.json`),
		// A short name the disk can't tell: refused, not read as written (it'd be inside the demo, or beside the
		// app's folder).
		write(home + `\smolcorp-demo\app\LOCKED~1\x.txt`),
		read(home + `\AppData\LOCKED~1\Smolcorp\registry.json`),
		// Outside the demo, by short names.
		write(`C:\Users\ADALOV~1\elsewhere.txt`),
	} {
		if c.Cwd = cwd; !strings.HasPrefix(f.check(c), demoFenceRefusal) {
			t.Errorf("let through %s %s", c.ToolName, c.ToolInput)
		}
	}

	// Off Windows a ~ and a digit is a name like any other.
	mac := demoFence{root: "/Users/ada/smolcorp-demo", roles: []string{"tester"}, home: "/Users/ada", goos: "darwin"}
	if c := write("/Users/ada/smolcorp-demo/app/NOTES~1.MD"); mac.check(c) != "" {
		t.Errorf("on a Mac, refused a name with ~1: %s", mac.check(c))
	}

	// With no disk of Windows' to ask and no stand-in, a short name's shape is never read as written. (On Windows,
	// the fence asks its own disk: TestOnWindowsTheDemosFenceReadsShortNamesOnTheDisk.)
	bare := demoFence{root: home + `\smolcorp-demo`, support: home + `\AppData\Roaming\Smolcorp`, roles: []string{"tester"}, home: home, goos: "windows"}
	if runtime.GOOS == "windows" {
		return
	}
	for _, c := range []demoCall{
		write(home + `\smolcorp-demo\app\CLAUDE~1\settings.json`),
		read(home + `\AppData\Roaming\SMOLCO~1\registry.json`),
		write(home + `\smolcorp-demo\app\NOTES~1.MD`),
	} {
		if c.Cwd = cwd; !strings.HasPrefix(bare.check(c), demoFenceRefusal) {
			t.Errorf("with no disk, let through %s %s", c.ToolName, c.ToolInput)
		}
	}
	if c := write(home + `\smolcorp-demo\app\notes.md`); bare.check(c) != "" {
		t.Errorf("with no disk, refused a long name: %s", bare.check(c))
	}
}
