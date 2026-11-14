package sandbox

import (
	"debug/elf"
	"debug/macho"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// Programs (SA-X2, SANDBOX-PLAN §A3.5.2, C3): the programs Smolcorp runs for itself, outside every desk, resolved once
// at the core's start, and what no desk may write for them: the Policy's InstallRoots. A desk that could write one would
// have Smolcorp run its code as the owner, outside the sandbox: a program replaced, a library or module it loads planted
// beside it, a link on the way to it pointed elsewhere, or a program of the same name put where a lookup takes it first.
//
// For each program, what is denied is:
//   - every link on the way to it, where it is met, the ones in the middle too (a middle hop put back would lead
//     elsewhere): Homebrew's bin/gh, the opt/<formula> a library is loaded through;
//   - its whole install root: the Homebrew keg or cask (Cellar/<formula>, Caskroom/<cask>), the outermost .app bundle
//     (Mac), the npm package (node_modules/<name> or node_modules/@scope/<name>), Claude Code's own folder
//     (~/.local/share/claude), and on a Mac the developer folder /usr/bin's tools hand on to (the .app it is in, for
//     Xcode), with xcrun's cache, which they read which program to hand on to from (RT126SAX2 B1). A program anywhere
//     else is denied by itself;
//   - what it loads, the same way, to the end: a Mach-O file's libraries (every load command, weak ones too; @rpath
//     through its LC_RPATHs), an ELF file's loader and libraries (through its RUNPATH), a script's interpreter, and the
//     files an npm or pnpm launcher script names beside itself ($basedir, %dp0%);
//   - every place a lookup could take instead: the same name in every search folder, before the one it was found in
//     and after it, so a desk can't put one there for the core's next start, or a lookup by name in another order
//     (findGH takes the login shell's PATH first), to take; an @rpath's or a RUNPATH's folders before the library's
//     own, and dyld's fallback for one that isn't anywhere. A place that doesn't exist is denied where a desk could
//     make it: in the home folder, as itself (the Policy's Hold makes the folders above it); elsewhere, the first
//     folder missing on the way, whose parent exists. A place is denied as spelled and where its links lead: the
//     kernel checks the place a link resolves to.
//
// Left out, as no desk writes them: a link at the top of the volume (/tmp, /var, /bin), the Mac's shared-cache
// libraries and every place its System Integrity Protection seals (/System, /usr but /usr/local, /bin, /sbin,
// /Library/Apple), and Linux's system libraries. No place denied is, or holds, the home folder, or is a
// top folder: a layout that would give one denies the program itself. Nothing is run: files are read, as a loader
// reads them. Anything that can't be read fails the whole: the desk doesn't start.

// ErrPrograms is a program, or something it loads, that can't be told whole: a link loop, too many files, a file that
// can't be read or parsed.
var ErrPrograms = errors.New("sandbox: a program Smolcorp runs can't be told whole")

// maxProgramFiles is how many files ResolvePrograms reads at most: Homebrew's node, with every library, is ~50.
const maxProgramFiles = 2000

// maxScript is how much of a script is read for its launcher's places: npm's and pnpm's are under 2 KiB.
const maxScript = 64 << 10

// ProgramName is a program Smolcorp runs for itself, by name.
type ProgramName struct {
	Name string
	// Systems are the systems it is run on; none is every one.
	Systems []string
	// Places are whole paths it is taken from besides the search: each is denied, there or not.
	Places []string
	// Why is who runs it.
	Why string
}

var (
	onMac   = []string{"darwin"}
	onLinux = []string{"linux"}
	// The usual places: findTmux's tmuxPlaces and the Swift app's Tool.find (macPlaces), findGH's ghPlaces (unixPlaces,
	// on every system but Windows). The Go finders look in the Mac's places on Linux too.
	macPlaces  = []string{"/opt/homebrew/bin", "/usr/local/bin", "/opt/local/bin", "/usr/bin"}
	unixPlaces = slices.Concat(macPlaces, []string{"/home/linuxbrew/.linuxbrew/bin", "/snap/bin"})
)

func in(folders []string, name string) []string {
	var out []string
	for _, folder := range folders {
		out = append(out, folder+"/"+name)
	}
	return out
}

var programNames = []ProgramName{
	{Name: "git", Why: "the core's git, on every repo (gitstate.go, hireplace.go)"},
	{Name: "gh", Systems: []string{"darwin", "linux"}, Places: in(unixPlaces, "gh"), Why: "the PR badge's gh pr view (ghpr.go)"},
	{Name: "gh", Systems: []string{"windows"}, Why: "the PR badge's gh pr view (ghpr.go)"},
	{Name: "tmux", Systems: onMac, Places: in(macPlaces, "tmux"), Why: "starts and drives hires (findTmux), and the Swift app's tabs (Tmux.find)"},
	{Name: "tmux", Systems: onLinux, Places: in(unixPlaces, "tmux"), Why: "starts and drives hires (findTmux)"},
	{Name: "pactl", Systems: onLinux, Why: "the volume, where there's no wpctl (music_linux.go)"},
	{Name: "wpctl", Systems: onLinux, Why: "the volume (music_linux.go)"},
	{Name: "xdg-open", Systems: onLinux, Why: "opens a page in the browser (network.go)"},
	{Name: "claude", Why: "Claude Code: the desks, the GM, ✨ Summary (deskstart.go, gmpreflight.go, summary_providers.go)"},
	{Name: "codex", Why: "Codex: desks and ✨ Summary (launcher.go, summary_providers.go)"},
	{Name: "gemini", Why: "Gemini: desks and ✨ Summary (launcher.go, summary_providers.go)"},
	{Name: "opencode", Why: "OpenCode: desks and ✨ Summary (opencode.go)"},
	{Name: "grok", Why: "Grok: ✨ Summary (summary_providers.go)"},
	{Name: "cursor-agent", Why: "Cursor's agent: ✨ Summary (summary_providers.go)"},
	{Name: "agent", Why: "Cursor's agent by its other name, asked its --version (summary_providers.go)"},
	{Name: "kitten", Systems: onMac, Places: []string{"/Applications/kitty.app/Contents/MacOS/kitten", "/opt/homebrew/bin/kitten", "/usr/local/bin/kitten"}, Why: "the Swift app's kitty tabs (TerminalDrivers.swift)"},
	{Name: "wezterm", Systems: onMac, Places: []string{"/Applications/WezTerm.app/Contents/MacOS/wezterm", "/opt/homebrew/bin/wezterm", "/usr/local/bin/wezterm"}, Why: "the Swift app's WezTerm tabs (TerminalDrivers.swift)"},
}

// ProgramNames are the programs Smolcorp runs for itself on goos, one per name.
func ProgramNames(goos string) []ProgramName {
	var out []ProgramName
	for _, n := range programNames {
		if len(n.Systems) == 0 || slices.Contains(n.Systems, goos) {
			out = append(out, n)
		}
	}
	return out
}

// Finding is what the core's start knows about where it finds programs. Every path is whole.
type Finding struct {
	// GOOS is the system; "" is this one. The files read are this computer's.
	GOOS string
	Home string
	// Search is the folders the core looks in for a program by name: its PATH, then its login shell's. Path takes the
	// first found in this order; the name is held in every one of them, so a caller's other order takes nothing a
	// desk put there. One that isn't whole is skipped, as a lookup skips it.
	Search []string
	// Executable is the core itself (os.Executable): Smolcorp's install is what it is in. "" leaves it out.
	Executable string
	// DeveloperDir is DEVELOPER_DIR in the core's environment, "" for the one xcode-select chose (a Mac).
	DeveloperDir string
	// UserTemp is the core's own temp folder on a Mac (confstr's _CS_DARWIN_USER_TEMP_DIR: the TMPDIR launchd gives
	// it, the app's NSTemporaryDirectory()), and XcrunDB is xcrun_db in the core's environment, "" when it isn't set.
	// /usr/bin's developer tools read the program they hand on to from xcrun's cache: XcrunDB when it is whole (one
	// that isn't is ignored), else xcrun_db in UserTemp; TMPDIR doesn't move it. Each is denied. When a program the
	// core runs is such a tool and neither is whole, ResolvePrograms fails.
	UserTemp, XcrunDB string
	// Also are programs the core runs that it found elsewhere, by name and path: each hire's recorded tmux.
	Also []Program
}

// Program is a program Smolcorp runs: Path is what the core runs, as found (links and all), "" when it wasn't found.
type Program struct {
	Name, Path string
}

// Programs are the programs Smolcorp runs, resolved once: Path never looks again.
type Programs struct {
	paths map[string]string
	roots []string
}

// Path is the program the core runs for name, "" for none: Smolcorp's own is "smolcorp".
func (ps *Programs) Path(name string) string { return ps.paths[name] }

// InstallRoots is every place no desk may write, for Places.InstallRoots: none inside another, in order.
func (ps *Programs) InstallRoots() []string { return slices.Clone(ps.roots) }

// ResolvePrograms resolves Smolcorp's install and every program of ProgramNames, then f.Also, once (see above). For a
// program found in more than one place (the bundled tmux, one of its Places, one on the search), Path is the first of
// those, in that order, and each is denied.
func ResolvePrograms(f Finding) (*Programs, error) {
	goos := f.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if !filepath.IsAbs(f.Home) || f.Executable != "" && !filepath.IsAbs(f.Executable) {
		return nil, ErrNotWhole
	}
	r := &resolver{goos: goos, home: filepath.Clean(f.Home), budget: maxProgramFiles, seen: map[string]bool{}}
	for _, folder := range f.Search {
		if filepath.IsAbs(folder) && !slices.Contains(r.search, filepath.Clean(folder)) {
			r.search = append(r.search, filepath.Clean(folder))
		}
	}
	if goos == "darwin" {
		r.devRoot = developerRoot(f.DeveloperDir)
		for _, db := range []string{f.XcrunDB, filepath.Join(f.UserTemp, "xcrun_db")} {
			if filepath.IsAbs(db) {
				r.xcrunDBs = append(r.xcrunDBs, filepath.Clean(db))
			}
		}
	}
	ps := &Programs{paths: map[string]string{}}
	var bundled []string
	if f.Executable != "" {
		if err := r.install(f.Executable); err != nil {
			return nil, fmt.Errorf("%w: Smolcorp's install: %w", ErrPrograms, err)
		}
		ps.paths["smolcorp"] = f.Executable
		if real, err := resolve(f.Executable); err == nil {
			bundled = append(bundled, filepath.Join(filepath.Dir(real), "tmux"))
		}
		if goos == "linux" {
			bundled = append(bundled, filepath.Join(r.home, ".local", "lib", "smolcorp", "tmux"))
		}
	}
	for _, n := range ProgramNames(goos) {
		var first []string
		if n.Name == "tmux" {
			first = bundled
		}
		path, err := r.named(n, first)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrPrograms, n.Name, err)
		}
		ps.paths[n.Name] = path
	}
	for _, also := range f.Also {
		if !filepath.IsAbs(also.Path) {
			return nil, ErrNotWhole
		}
		if err := r.hold(also.Path); err != nil {
			return nil, fmt.Errorf("%w: %s at %s: %w", ErrPrograms, also.Name, also.Path, err)
		}
	}
	ps.roots = outermost(goos, r.denied)
	return ps, nil
}

type resolver struct {
	goos, home, devRoot string
	search, xcrunDBs    []string
	denied              []string
	seen                map[string]bool
	budget              int
}

// named resolves the program n: first (the bundled tmux), its Places, then the search. Path is the first found.
func (r *resolver) named(n ProgramName, first []string) (string, error) {
	found := ""
	for _, place := range append(slices.Clone(first), n.Places...) {
		if found == "" && runnable(r.goos, place) {
			found = place
		}
		if err := r.hold(place); err != nil {
			return "", err
		}
	}
	hit, err := r.lookup(n.Name)
	if found == "" {
		found = hit
	}
	return found, err
}

// lookup is name as a lookup along the search finds it, "" for none: the name is held in every search folder, so
// what it finds is resolved, and so is anything a lookup in another order would take (RT126SAX2 N1).
func (r *resolver) lookup(name string) (string, error) {
	names := []string{name}
	if r.goos == "windows" {
		names = []string{name + ".com", name + ".exe", name + ".bat", name + ".cmd"}
	}
	found := ""
	for _, folder := range r.search {
		for _, file := range names {
			at := filepath.Join(folder, file)
			if found == "" && runnable(r.goos, at) {
				found = at
			}
			if err := r.hold(at); err != nil {
				return "", err
			}
		}
	}
	return found, nil
}

// runnable is whether a lookup takes the program at p: a regular file, its links followed, that can be run.
func runnable(goos, p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular() && (goos == "windows" || info.Mode().Perm()&0o111 != 0)
}

// hold denies p, whatever is there: a program (or anything) is resolved; a missing place is denied where a desk
// could make it, as spelled and where its links lead.
func (r *resolver) hold(p string) error {
	if _, err := os.Lstat(p); err == nil {
		return r.program(p, "")
	} else if !missing(err) {
		return err
	}
	at, err := resolve(p)
	if err != nil {
		return err
	}
	for _, place := range []string{p, at} {
		r.deny(r.appear(place))
	}
	return nil
}

// appear is where a missing p must be denied so a desk can't make it: p in the home folder, else the first folder
// missing on the way to it.
func (r *resolver) appear(p string) string {
	if k := key(r.goos, p); under(k, key(r.goos, r.home)) {
		return p
	}
	for at := p; ; {
		up := filepath.Dir(at)
		if up == at {
			return p
		}
		if _, err := os.Lstat(up); err == nil {
			return at
		}
		at = up
	}
}

// deny adds p, unless it is a top folder, holds the home folder or is sealed (see above).
func (r *resolver) deny(p string) {
	p = filepath.Clean(p)
	if k := key(r.goos, p); k == "" || topFolder(p) || under(key(r.goos, r.home), k) || sealed(r.goos, k) {
		return
	}
	r.denied = append(r.denied, p)
}

// sealed is whether the place with key k is one a Mac's System Integrity Protection keeps from the owner too.
func sealed(goos, k string) bool {
	return goos == "darwin" && !under(k, "/usr/local") &&
		slices.ContainsFunc([]string{"/system", "/usr", "/bin", "/sbin", "/library/apple"}, func(s string) bool { return under(k, s) })
}

// topFolder is whether p is the top of its volume or a folder right below it.
func topFolder(p string) bool {
	top := filepath.VolumeName(p) + string(filepath.Separator)
	return p == top || filepath.Dir(p) == top
}

// program denies what the program (or library, or folder) at p needs (see above). exe is the folder of the program
// that loads it, for a Mach-O library's @executable_path; "" for a program run by itself.
func (r *resolver) program(p, exe string) error {
	hops, real, err := linkHops(p)
	if err != nil {
		return err
	}
	for _, hop := range hops {
		r.deny(hop)
	}
	// A library is walked once for each program that loads it: its @executable_path is that program's (N5).
	info, err := os.Stat(real)
	switch seen := real + "\x00" + exe; {
	case missing(err):
		return r.hold(real)
	case err != nil:
		return err
	case r.seen[seen]:
		return nil
	default:
		r.seen[seen] = true
	}
	if r.budget--; r.budget < 0 {
		return errors.New("too many files")
	}
	if info.IsDir() {
		r.deny(real)
		return nil
	}
	if r.goos == "darwin" && filepath.Dir(real) == "/usr/bin" && r.devRoot != "" {
		// /usr/bin's developer tools hand on to the same name in the developer folder (xcrun): Xcode's is in its app.
		dev := r.devRoot
		if strings.HasSuffix(strings.ToLower(dev), ".app") {
			dev = filepath.Join(dev, "Contents", "Developer")
		}
		if tool := filepath.Join(dev, "usr", "bin", filepath.Base(real)); runnable(r.goos, tool) {
			if err := r.program(tool, ""); err != nil {
				return err
			}
		}
	}
	r.deny(r.rootOf(real))
	if exe == "" {
		exe = filepath.Dir(real)
	}
	return r.loads(real, exe)
}

// rootOf is the install root of the file at real (whole, resolved): see above.
func (r *resolver) rootOf(real string) string {
	parts := strings.Split(filepath.ToSlash(real), "/")
	join := func(n int) string { return filepath.FromSlash(strings.Join(parts[:n], "/")) }
	root := ""
	for i := 1; root == "" && i < len(parts)-1; i++ {
		if r.goos == "darwin" && strings.HasSuffix(strings.ToLower(parts[i]), ".app") {
			root = join(i + 1)
		}
	}
	for i := 1; root == "" && i+2 < len(parts); i++ {
		switch parts[i] {
		case "Cellar", "Caskroom":
			root = join(i + 2)
		case "node_modules":
			if strings.HasPrefix(parts[i+1], "@") && i+3 < len(parts) {
				root = join(i + 3)
			} else {
				root = join(i + 2)
			}
		}
	}
	for _, folder := range []string{r.devRoot, filepath.Join(r.home, ".local", "share", "claude")} {
		if root == "" && folder != "" && key(r.goos, real) != key(r.goos, folder) && under(key(r.goos, real), key(r.goos, folder)) {
			root = folder
		}
	}
	if root == "" || topFolder(root) || under(key(r.goos, r.home), key(r.goos, root)) {
		return real
	}
	return root
}

// install denies Smolcorp's own install: on a Mac the app it runs from (rootOf); on Linux the program and the folder
// its tmux comes in (~/.local/lib/smolcorp); on Windows its own folder.
func (r *resolver) install(exe string) error {
	if err := r.program(exe, ""); err != nil {
		return err
	}
	switch r.goos {
	case "linux":
		return r.hold(filepath.Join(r.home, ".local", "lib", "smolcorp"))
	case "windows":
		if real, err := resolve(exe); err == nil && strings.Contains(strings.ToLower(filepath.Base(filepath.Dir(real))), "smolcorp") {
			r.deny(filepath.Dir(real))
		}
	}
	return nil
}

// loads denies what the file at real loads: a Mach-O or ELF file's libraries, a script's interpreter and launcher
// places. Any other file loads nothing.
func (r *resolver) loads(real, exe string) error {
	f, err := os.Open(real)
	if err != nil {
		return err
	}
	defer f.Close()
	var head [8]byte
	n, _ := io.ReadFull(f, head[:])
	be, le := binary.BigEndian.Uint32(head[:4]), binary.LittleEndian.Uint32(head[:4])
	switch {
	case n >= 4 && be == macho.MagicFat && (n < 8 || binary.BigEndian.Uint32(head[4:]) > 30):
		return nil // 0xcafebabe with no likely count of architectures after it: a Java class, as file(1) tells them (N5)
	case n >= 4 && (be == macho.Magic32 || be == macho.Magic64 || le == macho.Magic32 || le == macho.Magic64 || be == macho.MagicFat || be == 0xcafebabf):
		return r.machoLoads(real, exe)
	case n >= 4 && string(head[:4]) == elf.ELFMAG:
		return r.elfLoads(real)
	case n >= 2 && string(head[:2]) == "#!", r.goos == "windows" && slices.Contains([]string{".cmd", ".bat", ".ps1"}, strings.ToLower(filepath.Ext(real))):
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		text, err := io.ReadAll(io.LimitReader(f, maxScript))
		if err != nil {
			return err
		}
		return r.script(real, string(text))
	}
	return nil
}

// Mach-O load commands that name a library, and LC_RPATH: each is a command, its size and the offset of its name.
const (
	lcLoadDylib     = 0xc
	lcLazyLoadDylib = 0x20
	lcLoadWeakDylib = 0x80000018
	lcReexport      = 0x8000001f
	lcLoadUpward    = 0x80000023
	lcRpath         = 0x8000001c
)

// xcselect is the library /usr/bin's developer tools hand on through: xcode-select's, which runs xcrun.
const xcselect = "/usr/lib/libxcselect.dylib"

func (r *resolver) machoLoads(real, exe string) error {
	var files []*macho.File
	if fat, err := macho.OpenFat(real); err == nil {
		defer fat.Close()
		for _, arch := range fat.Arches {
			files = append(files, arch.File)
		}
	} else if !errors.Is(err, macho.ErrNotFat) {
		return err
	} else if f, err := macho.Open(real); err == nil {
		defer f.Close()
		files = append(files, f)
	} else {
		return err
	}
	loader := filepath.Dir(real)
	for _, f := range files {
		var libs, rpaths []string
		for _, load := range f.Loads {
			raw := load.Raw()
			if len(raw) < 12 || f.ByteOrder.Uint32(raw[8:12]) >= uint32(len(raw)) {
				continue
			}
			name, _, _ := strings.Cut(string(raw[f.ByteOrder.Uint32(raw[8:12]):]), "\x00") // up to its NUL
			switch f.ByteOrder.Uint32(raw) {
			case lcLoadDylib, lcLazyLoadDylib, lcLoadWeakDylib, lcReexport, lcLoadUpward:
				libs = append(libs, name)
			case lcRpath:
				rpaths = append(rpaths, name)
			}
		}
		if r.goos == "darwin" && slices.Contains(libs, xcselect) {
			if err := r.xcrunCache(); err != nil {
				return err
			}
		}
		for _, lib := range libs {
			var candidates []string
			if rest, ok := strings.CutPrefix(lib, "@rpath/"); ok {
				for _, rpath := range rpaths {
					if at := atPath(rpath, loader, exe); at != "" {
						candidates = append(candidates, filepath.Join(at, rest))
					}
				}
			} else if at := atPath(lib, loader, exe); at != "" {
				candidates = []string{at}
			}
			// Where none is, dyld looks for an older program's library by its name in /usr/local/lib (man dyld:
			// DYLD_FALLBACK_LIBRARY_PATH; /usr/lib after it is sealed).
			candidates = append(candidates, filepath.Join("/usr/local/lib", path.Base(lib)))
			if err := r.first(candidates, exe); err != nil {
				return err
			}
		}
	}
	return nil
}

// xcrunCache denies xcrun's cache, where /usr/bin's developer tools read which program to hand on to (RT126SAX2 B1):
// each place the core's xcrun may keep it (Finding.UserTemp), as spelled, every link on the way and where they lead.
// It is never read. A desk that can't write it still runs those tools: xcrun then asks xcodebuild each time.
func (r *resolver) xcrunCache() error {
	if len(r.xcrunDBs) == 0 {
		return errors.New("xcrun's cache: no whole user temp folder")
	}
	for _, db := range r.xcrunDBs {
		hops, real, err := linkHops(db)
		if err != nil {
			return err
		}
		for _, place := range append(hops, db, real) {
			r.deny(r.appear(place))
		}
	}
	return nil
}

// atPath is a Mach-O library's place with @loader_path and @executable_path put in, "" when it isn't whole.
func atPath(name, loader, exe string) string {
	if rest, ok := strings.CutPrefix(name, "@loader_path"); ok {
		name = loader + rest
	} else if rest, ok := strings.CutPrefix(name, "@executable_path"); ok {
		name = exe + rest
	}
	if !filepath.IsAbs(name) {
		return ""
	}
	return filepath.Clean(name)
}

// systemLibrary is whether a library at p is the system's own: the Mac's shared cache, Linux's library folders.
func systemLibrary(goos, p string) bool {
	prefixes := []string{"/usr/lib/", "/System/"}
	if goos == "linux" {
		prefixes = []string{"/lib/", "/lib32/", "/lib64/", "/usr/lib/", "/usr/lib32/", "/usr/lib64/"}
	}
	return slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(p, prefix) })
}

// first is a loader's search through candidates: each one before the first that exists is held, and that one is
// resolved; a system library stops it (the system's own is taken).
func (r *resolver) first(candidates []string, exe string) error {
	for _, at := range candidates {
		if systemLibrary(r.goos, at) {
			return nil
		}
		if _, err := os.Stat(at); err == nil {
			return r.program(at, exe)
		}
		if err := r.hold(at); err != nil {
			return err
		}
	}
	return nil
}

func (r *resolver) elfLoads(real string) error {
	f, err := elf.Open(real)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, prog := range f.Progs {
		if prog.Type != elf.PT_INTERP {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(prog.Open(), 4096))
		if err != nil {
			return err
		}
		if interp := strings.TrimRight(string(data), "\x00"); filepath.IsAbs(interp) && !systemLibrary(r.goos, interp) {
			if err := r.hold(interp); err != nil {
				return err
			}
		}
	}
	needed, err := f.DynString(elf.DT_NEEDED)
	if err != nil {
		return err
	}
	runpath, err := f.DynString(elf.DT_RUNPATH)
	if err == nil && len(runpath) == 0 {
		runpath, err = f.DynString(elf.DT_RPATH)
	}
	if err != nil {
		return err
	}
	var folders []string
	origin := filepath.Dir(real)
	for _, list := range runpath {
		for _, folder := range strings.Split(list, ":") {
			folder = strings.NewReplacer("${ORIGIN}", origin, "$ORIGIN", origin).Replace(folder)
			if filepath.IsAbs(folder) {
				folders = append(folders, filepath.Clean(folder))
			}
		}
	}
	for _, lib := range needed {
		var candidates []string
		if filepath.IsAbs(lib) {
			candidates = []string{lib}
		} else if !strings.Contains(lib, "/") {
			for _, folder := range folders {
				candidates = append(candidates, filepath.Join(folder, lib))
			}
		}
		if err := r.first(candidates, ""); err != nil {
			return err
		}
	}
	return nil
}

// script denies a script's interpreter (one `env` finds is looked up on the search), and each place its launcher text
// names beside it: npm's and pnpm's `"$basedir/…"`, Windows' `"%dp0%\…"`. A bare name beside it that isn't there
// (`$basedir/node`) is the one the launcher then runs by name: it is looked up too.
func (r *resolver) script(real, text string) error {
	if line, ok := strings.CutPrefix(text, "#!"); ok {
		line, _, _ = strings.Cut(line, "\n")
		words := strings.Fields(line)
		if len(words) > 0 && filepath.IsAbs(words[0]) {
			if err := r.hold(words[0]); err != nil {
				return err
			}
			if filepath.Base(words[0]) == "env" {
				if name := envProgram(words[1:]); name != "" {
					if _, err := r.lookup(name); err != nil {
						return err
					}
				}
			}
		}
	}
	for _, marker := range []string{"$basedir/", "${basedir}/", `%dp0%\`, `%~dp0\`, `$basedir\`} {
		for rest := text; ; {
			i := strings.Index(rest, marker)
			if i < 0 {
				break
			}
			rest = rest[i+len(marker):]
			end := strings.IndexAny(rest, "\"' \t\r\n;:)`")
			if end < 0 {
				end = len(rest)
			}
			name := strings.ReplaceAll(rest[:end], `\`, "/")
			if name == "" {
				continue
			}
			at := filepath.Join(filepath.Dir(real), filepath.FromSlash(name))
			if _, err := os.Lstat(at); missing(err) && !strings.Contains(name, "/") {
				if _, err := r.lookup(strings.TrimSuffix(name, filepath.Ext(name))); err != nil {
					return err
				}
			}
			if err := r.hold(at); err != nil {
				return err
			}
		}
	}
	return nil
}

// envProgram is the program `env` runs from its words: the first that isn't an option or a NAME=value.
func envProgram(words []string) string {
	for i := 0; i < len(words); i++ {
		switch w := words[i]; {
		case w == "-u" || w == "-P" || w == "-C":
			i++
		case strings.HasPrefix(w, "-") || strings.Contains(w, "="):
		default:
			return w
		}
	}
	return ""
}

// developerRoot is a Mac's developer folder, as DEVELOPER_DIR or xcode-select names it (its link, read: nothing
// runs), else the command line tools': the .app it is in, for Xcode.
func developerRoot(given string) string {
	dir := given
	if !filepath.IsAbs(dir) {
		if to, err := os.Readlink("/var/db/xcode_select_link"); err == nil && filepath.IsAbs(to) {
			dir = to
		} else {
			dir = "/Library/Developer/CommandLineTools"
		}
	}
	if real, err := resolve(filepath.Clean(dir)); err == nil {
		dir = real
	}
	if i := strings.Index(strings.ToLower(dir), ".app/"); i >= 0 {
		return dir[:i+len(".app")]
	}
	return dir
}

// linkHops is where p ends, as the system walks it (as resolve does), and every link met on the way, each at the
// place it was met.
func linkHops(p string) (hops []string, at string, err error) {
	budget := 255
	var walk func(p string) (string, error)
	walk = func(p string) (string, error) {
		vol := filepath.VolumeName(p)
		at := vol + string(filepath.Separator)
		parts := strings.FieldsFunc(p[len(vol):], func(r rune) bool { return r < 0x80 && os.IsPathSeparator(uint8(r)) })
		for i, part := range parts {
			switch part {
			case ".":
				continue
			case "..":
				at = filepath.Dir(at)
				continue
			}
			next := filepath.Join(at, part)
			info, err := os.Lstat(next)
			if missing(err) {
				return filepath.Join(append([]string{next}, parts[i+1:]...)...), nil
			} else if err != nil {
				return "", err
			}
			if !linkish(info.Mode()) {
				at = next
				continue
			}
			if budget--; budget < 0 {
				return "", errLinkLoop
			}
			hops = append(hops, next)
			target, err := os.Readlink(next)
			if err != nil {
				return "", err
			}
			if !filepath.IsAbs(target) {
				target = at + string(filepath.Separator) + target
			}
			if at, err = walk(target); err != nil {
				return "", err
			}
		}
		return at, nil
	}
	if !filepath.IsAbs(p) {
		return nil, "", ErrNotWhole
	}
	at, err = walk(p)
	return hops, at, err
}

// outermost is places with none inside another and no two the same, in order.
func outermost(goos string, places []string) []string {
	sorted := slices.Clone(places)
	slices.SortFunc(sorted, func(a, b string) int { return len(key(goos, a)) - len(key(goos, b)) })
	var out []string
	for _, p := range sorted {
		if !slices.ContainsFunc(out, func(o string) bool { return under(key(goos, p), key(goos, o)) }) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}
