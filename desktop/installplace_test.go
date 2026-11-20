package desktop

import (
	"encoding/binary"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

// FXWININSTALL: Windows as these rows need it, on any computer. A program in use can be renamed, as a running
// Smolcorp.exe or the smolcorp-cli.exe an agent's hook runs, but not replaced or removed; a file held without leave to
// rename it, as a scanner may hold one, can't be renamed either.
type windowsFiles struct {
	inUse, held map[string]bool
}

var errSharing = errors.New("The process cannot access the file because it is being used by another process.")

func withWindowsFiles(t *testing.T) *windowsFiles {
	t.Helper()
	w := &windowsFiles{inUse: map[string]bool{}, held: map[string]bool{}}
	savedRename, savedRemove := placeRename, placeRemove
	t.Cleanup(func() { placeRename, placeRemove = savedRename, savedRemove })
	placeRename = func(from, to string) error {
		if w.held[from] || w.held[to] || w.inUse[to] {
			return &os.LinkError{Op: "rename", Old: from, New: to, Err: errSharing}
		}
		if err := os.Rename(from, to); err != nil {
			return err
		}
		if w.inUse[from] {
			delete(w.inUse, from)
			w.inUse[to] = true // the running program goes with its file
		}
		return nil
	}
	placeRemove = func(path string) error {
		if w.held[path] || w.inUse[path] {
			return &os.PathError{Op: "remove", Path: path, Err: errSharing}
		}
		return os.Remove(path)
	}
	return w
}

// aPair is an installed folder with the old pair in it, and a download with the new one.
func aPair(t *testing.T) (dir string, programs []newProgram) {
	t.Helper()
	dir, download := t.TempDir(), t.TempDir()
	for _, name := range installedPrograms {
		writeFile(t, filepath.Join(dir, name), "old "+name)
		writeFile(t, filepath.Join(download, name), "new "+name)
		programs = append(programs, newProgram{from: filepath.Join(download, name), to: filepath.Join(dir, name)})
	}
	return dir, programs
}

// folderHolds is the folder's files, each with what it holds.
func folderHolds(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	held := map[string]string{}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		held[entry.Name()] = string(data)
	}
	return held
}

// A tester's 1.26 report: an update while Smolcorp and its hooks run renames both old programs aside, puts the new
// pair in, and leaves the old ones, still running, for the next start.
func TestAnUpdateRenamesAProgramInUseAsideAndMovesTheNewOneIn(t *testing.T) {
	w := withWindowsFiles(t)
	dir, programs := aPair(t)
	for _, p := range programs {
		w.inUse[p.to] = true
	}
	if err := placePrograms(programs); err != nil {
		t.Fatalf("an update over the running pair: %v", err)
	}
	want := map[string]string{
		"Smolcorp.exe": "new Smolcorp.exe", "smolcorp-cli.exe": "new smolcorp-cli.exe",
		"Smolcorp.exe.old-1": "old Smolcorp.exe", "smolcorp-cli.exe.old-1": "old smolcorp-cli.exe",
	}
	if got := folderHolds(t, dir); !maps.Equal(got, want) {
		t.Errorf("the folder after the update: %v, want %v", got, want)
	}
	// A second update, the first one's set-aside pair still running: each goes to the next free name.
	for _, p := range programs {
		w.inUse[p.to] = true
		writeFile(t, p.from, "newer "+filepath.Base(p.to))
	}
	if err := placePrograms(programs); err != nil {
		t.Fatalf("a second update: %v", err)
	}
	if got := folderHolds(t, dir); got["smolcorp-cli.exe"] != "newer smolcorp-cli.exe" || got["smolcorp-cli.exe.old-2"] != "new smolcorp-cli.exe" ||
		got["smolcorp-cli.exe.old-1"] != "old smolcorp-cli.exe" {
		t.Errorf("the folder after a second update: %v", got)
	}
	// None in use: the old ones go at once, and nothing else is left.
	dir, programs = aPair(t)
	if err := placePrograms(programs); err != nil {
		t.Fatal(err)
	}
	if got := folderHolds(t, dir); !maps.Equal(got, map[string]string{"Smolcorp.exe": "new Smolcorp.exe", "smolcorp-cli.exe": "new smolcorp-cli.exe"}) {
		t.Errorf("the folder after an update with nothing running: %v", got)
	}
}

// Setup's Install half-failed: Smolcorp.exe in, smolcorp-cli.exe not. Now the first is put back, the folder holds the
// old pair untouched, and one plain sentence says what failed.
func TestWhenTheSecondProgramCantBePlacedTheFirstIsPutBack(t *testing.T) {
	for _, c := range []struct {
		name  string
		block func(w *windowsFiles, cli newProgram)
	}{
		{"the old one can't be set aside", func(w *windowsFiles, cli newProgram) { w.held[cli.to] = true }},
		{"the new one can't be moved in", func(_ *windowsFiles, cli newProgram) {
			rename := placeRename
			placeRename = func(from, to string) error {
				if to == cli.to && strings.Contains(filepath.Base(from), ".new-") {
					return &os.LinkError{Op: "rename", Old: from, New: to, Err: errSharing}
				}
				return rename(from, to)
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := withWindowsFiles(t)
			dir, programs := aPair(t)
			before := folderHolds(t, dir)
			c.block(w, programs[1])
			err := placePrograms(programs)
			const said = "Smolcorp couldn't update smolcorp-cli.exe: the process cannot access the file because it is " +
				"being used by another process. Nothing was changed."
			if err == nil || err.Error() != said {
				t.Errorf("said %v, want %q", err, said)
			}
			if got := folderHolds(t, dir); !maps.Equal(got, before) {
				t.Errorf("the folder after: %v, want it as it was: %v", got, before)
			}
		})
	}
	// Putting the old one back refused too: it says so, and sends the owner to the download to finish, not "Nothing was
	// changed".
	w := withWindowsFiles(t)
	dir, programs := aPair(t)
	w.held[programs[1].to] = true
	rename := placeRename
	placeRename = func(from, to string) error {
		if from == programs[0].to+".old-1" {
			return &os.LinkError{Op: "rename", Old: from, New: to, Err: errors.New("Access is denied.")}
		}
		return rename(from, to)
	}
	const undoFailed = "Smolcorp couldn't update smolcorp-cli.exe: the process cannot access the file because it is being " +
		"used by another process. Undoing that failed too (access is denied), so open the Smolcorp you downloaded " +
		"again to finish."
	if err := placePrograms(programs); err == nil || err.Error() != undoFailed {
		t.Errorf("the undo refused: said %v, want %q", err, undoFailed)
	}
	want := map[string]string{"Smolcorp.exe": "new Smolcorp.exe", "Smolcorp.exe.old-1": "old Smolcorp.exe", "smolcorp-cli.exe": "old smolcorp-cli.exe"}
	if got := folderHolds(t, dir); !maps.Equal(got, want) {
		t.Errorf("the folder after the undo refused: %v, want %v, as the sentence says", got, want)
	}

	// Nothing there before: it says install.
	w = withWindowsFiles(t)
	dir, programs = aPair(t)
	for _, p := range programs {
		if err := os.Remove(p.to); err != nil {
			t.Fatal(err)
		}
	}
	w.held[programs[1].to] = true
	if err := placePrograms(programs); err == nil || !strings.HasPrefix(err.Error(), "Smolcorp couldn't install smolcorp-cli.exe: ") {
		t.Errorf("a first install: %v", err)
	}
	if got := folderHolds(t, dir); len(got) != 0 {
		t.Errorf("a first install that failed left %v", got)
	}
}

// The next start takes away, quietly, what an update set aside and a new file one didn't finish: not one still in use,
// and nothing else.
func TestTheNextStartTakesAwayWhatAnUpdateSetAside(t *testing.T) {
	w := withWindowsFiles(t)
	dir := t.TempDir()
	for _, name := range []string{"Smolcorp.exe", "smolcorp-cli.exe", "Smolcorp.exe.old-1", "smolcorp-cli.exe.old-1",
		"smolcorp-cli.exe.old-2", ".Smolcorp.exe.new-123", "notes.old-1", "other.exe.old-1"} {
		writeFile(t, filepath.Join(dir, name), name)
	}
	w.inUse[filepath.Join(dir, "smolcorp-cli.exe.old-2")] = true // a hook still runs it
	removeLeftovers(dir, installedPrograms)
	var left []string
	for name := range folderHolds(t, dir) {
		left = append(left, name)
	}
	slices.Sort(left)
	if want := []string{"Smolcorp.exe", "notes.old-1", "other.exe.old-1", "smolcorp-cli.exe", "smolcorp-cli.exe.old-2"}; !slices.Equal(left, want) {
		t.Errorf("left %v, want %v", left, want)
	}
	// It's RunApp's, once this copy holds the instance lock: the one before has let go of its program by then.
	app, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	run := runAppBody(t, app)
	if lock, removed := strings.Index(run, "acquire()"), strings.Index(run, "removeOldPrograms()"); lock < 0 || removed < lock {
		t.Error("RunApp doesn't take away what an update set aside, after the instance lock")
	}
}

// The hand-over, as the auto-install does it (the Windows row TestOnWindowsAnUpdateWhileSmolcorpAndItsHooksRunHandsOver
// runs the ask and the lock): installed, it finds the copy running before anything starts, starts the installed copy as
// one taking its place, which waits for the instance lock, and only then asks the copy found to make way. If the
// installed one can't start, the running one isn't asked, and runs on.
func TestTheAutoInstallAsksTheRunningCopyToMakeWay(t *testing.T) {
	var did []string
	running := func() (func() bool, func()) {
		did = append(did, "find the running copy")
		return func() bool { did = append(did, "ask it to make way"); return true }, func() { did = append(did, "let go of it") }
	}
	start := func(fails error) func() error {
		return func() error { did = append(did, "start the installed copy"); return fails }
	}
	want := []string{"find the running copy", "start the installed copy", "ask it to make way", "let go of it"}
	if err := startThenMakeWay(running, start(nil)); err != nil || !slices.Equal(did, want) {
		t.Errorf("the installed copy started: %v, did %q, want %q", err, did, want)
	}
	did = nil
	refused := errors.New("access is denied")
	want = []string{"find the running copy", "start the installed copy", "let go of it"}
	if err := startThenMakeWay(running, start(refused)); err != refused || !slices.Equal(did, want) {
		t.Errorf("the installed copy couldn't start: %v, did %q, want its error and %q: the running one never asked", err, did, want)
	}

	// The auto-install hands over through startThenMakeWay alone (install_windows.go, read here on any computer).
	source, err := os.ReadFile("install_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	body := text[strings.Index(text, "func installAndRelaunch() bool {"):]
	body = body[:strings.Index(body, "\n}\n")]
	hands := strings.Index(body, "startThenMakeWay(runningToMakeWay, func() error {\n\t\treturn startProgram(installed, ")
	if hands < 0 || !strings.Contains(body[hands:], `restartedVariable+"=1")`+"\n\t})") ||
		strings.Count(body, "runningToMakeWay") != 1 || strings.Count(body, "startProgram(") != 1 {
		t.Errorf("installAndRelaunch doesn't start the installed copy in the running one's place through startThenMakeWay, then ask it to make way:\n%s", body)
	}
	if strings.Count(body, "notifyUser(") != 2 {
		t.Error("the auto-install doesn't say what failed, installing or starting: Smolcorp.exe has no console for its error")
	}
	// If the installed copy couldn't start, the auto-install says so and this copy stays: true only once it started.
	const startFailed = "\n\t})\n\tif err != nil {\n"
	if at := strings.Index(body, startFailed); hands < 0 || at < hands ||
		!strings.HasSuffix(body[at:], "\t\tnotifyUser(said)\n\t\treturn false\n\t}\n\treturn true") ||
		strings.Count(body[at:], "return true") != 1 {
		t.Errorf("installAndRelaunch doesn't stay, and say why, when the installed copy couldn't start:\n%s", body)
	}
}

// Round 2, item 2: a hook names smolcorp-cli.exe by its long path when no shell needs it quoted; else by a short form
// read back to be that very file; never by an alias an update moved to another file.
func TestAWindowsHookNamesTheConsoleProgramItself(t *testing.T) {
	const usual = `C:\Users\ada\AppData\Local\Smolcorp\smolcorp-cli.exe`
	const usualShort = `C:\Users\ada\AppData\Local\Smolcorp\SMOLCO~1.EXE`
	const spaced = `C:\Users\Ada Lovelace\AppData\Local\Smolcorp\smolcorp-cli.exe`
	const spacedShort = `C:\Users\ADALOV~1\AppData\Local\Smolcorp\SMOLCO~1.EXE`
	// The disk: the short name it gives a path, and the long name it reads a short one back as. They disagree when
	// the alias was handed to another file in between, as an update's renames do.
	disk := func(short, long map[string]string) (func(string) (string, error), func(string) (string, error)) {
		shortOf := func(path string) (string, error) {
			if s, ok := short[path]; ok {
				return s, nil
			}
			return "", errors.New("no short name")
		}
		longOf := func(path string) (string, error) {
			if l, ok := long[strings.ReplaceAll(path, "/", `\`)]; ok {
				return l, nil
			}
			return path, nil
		}
		return shortOf, longOf
	}
	quoted := `"C:/Users/Ada Lovelace/AppData/Local/Smolcorp/smolcorp-cli.exe"`
	for _, c := range []struct {
		name, path  string
		short, long map[string]string
		want        string
	}{
		// The short name reads back as this very file, so only the usual case's own branch keeps the long path
		// (RT126FIXREVIEW-WININSTALLR2 O1: with no read-back, a short name was never in the running).
		{"the usual folder", usual, map[string]string{usual: usualShort}, map[string]string{usualShort: usual},
			"C:/Users/ada/AppData/Local/Smolcorp/smolcorp-cli.exe"},
		{"a space in the user's name", spaced, map[string]string{spaced: spacedShort}, map[string]string{spacedShort: spaced},
			"C:/Users/ADALOV~1/AppData/Local/Smolcorp/SMOLCO~1.EXE"},
		{"the alias now Smolcorp.exe's", spaced, map[string]string{spaced: spacedShort},
			map[string]string{spacedShort: `C:\Users\Ada Lovelace\AppData\Local\Smolcorp\Smolcorp.exe`}, quoted},
		{"the alias now a program set aside's", spaced, map[string]string{spaced: spacedShort},
			map[string]string{spacedShort: spaced + ".old-1"}, quoted},
		{"short names off", spaced, nil, nil, quoted},
	} {
		shortOf, longOf := disk(c.short, c.long)
		if got := windowsHookPath(c.path, shortOf, longOf); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
	// The hook as written is the deck's, so an update replaces it rather than adding another.
	if !isOurs(map[string]any{"command": "C:/Users/ada/AppData/Local/Smolcorp/smolcorp-cli.exe hook --port 7710"}) {
		t.Error("a hook naming the long path isn't read as the deck's")
	}
}

// aShortcut is a .lnk as Windows writes one for a file on a local disk: the header, an item list, then the LinkInfo
// with the target's path, in Unicode too where unicode is set.
func aShortcut(target string, unicode bool) []byte {
	const header = 0x4C
	link := make([]byte, header)
	binary.LittleEndian.PutUint32(link, header)
	binary.LittleEndian.PutUint32(link[20:], 0x1|0x2|0x80) // an item list, a LinkInfo, Unicode strings
	link = append(link, 4, 0, 0xAA, 0xBB, 0, 0)            // a 4-byte item list, read past
	headerSize := 0x1C
	if unicode {
		headerSize = 0x24
	}
	info := make([]byte, headerSize)
	volume := []byte{0x10, 0, 0, 0, 3, 0, 0, 0, 0, 0, 0, 0, 0x10, 0, 0, 0}
	basePath := len(info) + len(volume)
	info = append(info, volume...)
	info = append(append(info, []byte(target)...), 0)
	suffix := len(info)
	info = append(info, 0)
	var unicodeBase, unicodeSuffix int
	if unicode {
		unicodeBase = len(info)
		for _, unit := range utf16.Encode([]rune(target)) {
			info = binary.LittleEndian.AppendUint16(info, unit)
		}
		info = append(info, 0, 0)
		unicodeSuffix = len(info)
		info = append(info, 0, 0)
	}
	for at, value := range []int{len(info), headerSize, 0x1, headerSize, basePath, 0, suffix} {
		binary.LittleEndian.PutUint32(info[at*4:], uint32(value))
	}
	if unicode {
		binary.LittleEndian.PutUint32(info[28:], uint32(unicodeBase))
		binary.LittleEndian.PutUint32(info[32:], uint32(unicodeSuffix))
	}
	return append(link, info...)
}

// Round 2, item 1: Smolcorp's own shortcut in the Startup folder is found by its target, never by its name; another
// app's, or one that isn't a shortcut, stays. So does one to smolcorp-cli.exe (smolcorp serve at sign-in, say), or to a
// short name that may be either program: it's the app's shortcut alone that opens Smolcorp a second time.
func TestOnlySmolcorpsOwnStartupShortcutIsFound(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Smolcorp.lnk", aShortcut(`C:\Users\ada\Downloads\Smolcorp-Windows-x86_64\Smolcorp.exe`, true))
	write("old build.LNK", aShortcut(`C:\Users\ada\AppData\Local\Smolcorp\Smolcorp.exe`, false))
	write("before the new name.lnk", aShortcut(`C:\Users\ada\AppData\Local\SauzzeDeck\SauzzeDeck.exe`, true))
	write("Smolcorp server.lnk", aShortcut(`C:\Users\ada\AppData\Local\Smolcorp\smolcorp-cli.exe`, true))
	write("old server.lnk", aShortcut(`C:\Users\ada\AppData\Local\SauzzeDeck\sauzzedeck-cli.exe`, true))
	write("short.lnk", aShortcut(`C:\Users\ADALOV~1\AppData\Local\Smolcorp\SMOLCO~1.EXE`, false))
	write("Smolcorp notes.lnk", aShortcut(`C:\Windows\System32\notepad.exe`, true))
	write("Other.lnk", aShortcut(`C:\Program Files\Other\other.exe`, true))
	write("smolcorp folder.lnk", aShortcut(`C:\Users\ada\smolcorp`, true))
	write("broken.lnk", []byte("not a shortcut"))
	write("desktop.ini", []byte("[.ShellClassInfo]"))
	var found []string
	for _, path := range smolcorpShortcuts(dir) {
		found = append(found, filepath.Base(path))
	}
	slices.Sort(found)
	if want := []string{"Smolcorp.lnk", "before the new name.lnk", "old build.LNK"}; !slices.Equal(found, want) {
		t.Errorf("found %v, want %v", found, want)
	}
	if target, err := shortcutTarget(aShortcut(`C:\Users\Hélène\Smolcorp\Smolcorp.exe`, true)); err != nil || target != `C:\Users\Hélène\Smolcorp\Smolcorp.exe` {
		t.Errorf("a target read in Unicode: %q %v", target, err)
	}
}

// The firewall approves once: the script keeps one Tailscale rule, for the program it's given, after taking away every
// rule for any smolcorp*.exe, the unzip folder's among them; and a copy that installs itself does so before it listens,
// so Windows never asks for the unzip folder's program.
func TestTheFirewallKeepsOneRuleForTheInstalledProgram(t *testing.T) {
	script, err := os.ReadFile("firewall.ps1")
	if err != nil {
		t.Fatal(err)
	}
	text := string(script)
	removes := strings.Index(text, "Remove-NetFirewallRule")
	adds := regexp.MustCompile(`New-NetFirewallRule[^\n]*\n[^\n]*`).FindAllStringIndex(text, -1)
	if !strings.Contains(text, `Where-Object { $_.Program -like '*smolcorp*.exe' }`) || removes < 0 || len(adds) == 0 || removes > adds[0][0] {
		t.Error("the script doesn't take away every rule for a smolcorp*.exe before it adds its own")
	}
	lan := strings.Index(text, "if ($Mode -eq 'lan')")
	outside := 0
	for _, add := range adds {
		if !strings.Contains(text[add[0]:add[1]], "-Program $Program ") {
			t.Errorf("a rule for another program: %s", text[add[0]:add[1]])
		}
		if add[0] < lan {
			outside++
		}
	}
	if outside != 1 || lan < 0 {
		t.Errorf("%d rules added for Tailscale alone, want 1", outside)
	}
	app, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	run := runAppBody(t, app)
	if installs, serves := strings.Index(run, "installAndRelaunch()"), strings.Index(run, "Serve(deck)"); installs < 0 || serves < installs {
		t.Error("RunApp listens before it installs itself: Windows would ask for the unzip folder's program")
	}
	// A rule Windows' own box made for the unzip folder's program is said, and Set the rules takes it away.
	stale := firewallRule{DisplayName: "smolcorp.exe", Program: downloadsCopy, Profile: "Private", RemoteAddress: "Any",
		Protocol: "TCP", Action: "Allow", Enabled: true}
	ours := firewallRule{DisplayName: firewallTailscaleRule, Program: installed, Profile: "Any", RemoteAddress: "100.64.0.0/10",
		Protocol: "TCP", Action: "Allow", Enabled: true}
	if c := firewallVerdict([]firewallRule{ours, stale}, installed, listenTailscale); c.Fix != fixFirewall || !strings.Contains(c.Detail, downloadsCopy) {
		t.Errorf("with the unzip folder's rule beside the installed one: %+v", c)
	}
}

// runAppBody is RunApp's code, as app.go has it.
func runAppBody(t *testing.T, app []byte) string {
	t.Helper()
	text := string(app)
	start := strings.Index(text, "func RunApp() int {")
	if start < 0 {
		t.Fatal("no RunApp in app.go")
	}
	end := strings.Index(text[start:], "\n}\n")
	if end < 0 {
		t.Fatal("RunApp doesn't end")
	}
	return text[start : start+end]
}
