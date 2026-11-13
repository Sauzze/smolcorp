package desktop

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

// fencePaths are the path rules of the system whose tool calls a demo's fence reads. On the computer the
// fence runs on (goos "" or its own), they are path/filepath's and the disk's: links followed, short names
// made long. For another system (goos set, in a test) they are that system's rules alone, so a test on a Mac
// reads a Windows call as Windows would (C:\…, \ and / both separators, case aside), and the other way round.
// No disk is asked then: a hire's folder, which must be a checkout on the disk, is never held (fail-closed).
type fencePaths struct{ goos string }

// here says whether the rules are this computer's, with its disk.
func (p fencePaths) here() bool { return p.goos == "" || p.goos == runtime.GOOS }

func (p fencePaths) windows() bool {
	return p.goos == "windows" || p.goos == "" && runtime.GOOS == "windows"
}

func (p fencePaths) separator(c byte) bool { return c == '/' || c == '\\' && p.windows() }

// volume is a Windows path's drive (C:) or share (\\host\share, \\?\C:), "" for none and off Windows.
func (p fencePaths) volume(name string) string {
	switch {
	case !p.windows():
		return ""
	case p.here():
		return filepath.VolumeName(name)
	case len(name) >= 2 && name[1] == ':' && ('a' <= name[0]|0x20 && name[0]|0x20 <= 'z'):
		return name[:2]
	case len(name) >= 2 && p.separator(name[0]) && p.separator(name[1]):
		// \\host\share\…, \\?\C:\…: the two names after the \\.
		names := 0
		for i := 2; i < len(name); i++ {
			if p.separator(name[i]) {
				if names++; names == 2 {
					return name[:i]
				}
			}
		}
		return name
	}
	return ""
}

func (p fencePaths) isAbs(name string) bool {
	switch {
	case p.here():
		return filepath.IsAbs(name)
	case !p.windows():
		return strings.HasPrefix(name, "/")
	}
	volume := p.volume(name)
	if volume == "" {
		return false // \x is the top of the drive, not a whole path
	}
	return p.separator(volume[0]) || len(name) > len(volume) && p.separator(name[len(volume)])
}

// rooted says whether a path that isn't whole starts at the top of a drive (\x, /x on Windows).
func (p fencePaths) rooted(name string) bool {
	return strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`)
}

// gitBash is a path as Git Bash (MSYS2), Claude Code's Bash on Windows, hands it to Windows: /c/…, /cygdrive/c/…
// and /proc/cygdrive/c/… are C:\… (RWIN5 B2). Only as written with /: Windows reads \c\… as a folder c at the
// top of the drive. Any other path is as it was, and off Windows every path is.
func (p fencePaths) gitBash(name string) string {
	if !p.windows() {
		return name
	}
	if drive := gitBashDrive.FindStringSubmatch(name); drive != nil {
		return strings.ToUpper(drive[1]) + `:\` + name[len(drive[0]):]
	}
	return name
}

var gitBashDrive = regexp.MustCompile(`^/(?i:proc/+)?(?i:cygdrive/+)?([A-Za-z])(?:/+|$)`)

// gitBashDevice says whether a path is one of Git Bash's devices (/dev/null, /dev/stdout, /dev/fd/2), written with
// / and staying in /dev once its .. are taken: no folder of the disk's. Off Windows, /dev is the disk's own.
func (p fencePaths) gitBashDevice(name string) bool {
	return p.windows() && strings.HasPrefix(name, "/dev/") && !strings.Contains(name, `\`) && strings.HasPrefix(path.Clean(name), "/dev/")
}

// win32 is a Windows path with each name as Win32 reads it: what follows a : taken off (an NTFS stream,
// registry.json::$DATA is registry.json) and the dots and blanks at its end (SauzzeDeck Pro. is SauzzeDeck Pro).
// The drive's own : stays, so C:x still drifts. A name of dots alone (., ..) is as it was. Off Windows, name. The
// fence reads a path into the app's support folder this way (supportRests); a write's path keeps its : and is
// refused (drifts).
func (p fencePaths) win32(name string) string {
	if !p.windows() {
		return name
	}
	volume := p.volume(name)
	var out strings.Builder
	out.WriteString(volume)
	for rest := name[len(volume):]; rest != ""; {
		end := strings.IndexAny(rest, `/\`)
		if end < 0 {
			end = len(rest)
		}
		part, _, _ := strings.Cut(rest[:end], ":")
		if trimmed := strings.TrimRight(part, ". "); trimmed != "" && strings.Trim(part, ".") != "" {
			part = trimmed
		}
		out.WriteString(part)
		if end < len(rest) {
			out.WriteByte(rest[end])
			end++
		}
		rest = rest[end:]
	}
	return out.String()
}

// drifts says whether a Windows path means a place the fence can't read from its letters: a drive with no
// folder after it (C:foo, C:..\x: from that drive's own current folder, not cwd), or a : after the drive
// (an NTFS stream, .git::$INDEX_ALLOCATION). Off Windows, a : is a letter.
func (p fencePaths) drifts(name string) bool {
	if !p.windows() {
		return false
	}
	volume := p.volume(name)
	return volume != "" && !p.isAbs(name) || strings.Contains(name[len(volume):], ":")
}

func (p fencePaths) clean(name string) string {
	switch {
	case p.here():
		return filepath.Clean(name)
	case !p.windows():
		return path.Clean(name)
	}
	volume := p.volume(name)
	rest := strings.ReplaceAll(name[len(volume):], `\`, "/")
	if rest == "" {
		rest = "."
	}
	return strings.ReplaceAll(volume, "/", `\`) + strings.ReplaceAll(path.Clean(rest), "/", `\`)
}

func (p fencePaths) join(base, name string) string {
	switch {
	case p.here():
		return filepath.Join(base, name)
	case !p.windows():
		return path.Join(base, name)
	}
	return p.clean(base + `\` + name)
}

// below is path's rest under root, parts split by /, "." for root itself; false when path isn't root or
// below it. Windows compares case aside, as its disk does.
func (p fencePaths) below(root, name string) (string, bool) {
	if p.here() {
		rest, err := filepath.Rel(root, name)
		if err != nil || rest == ".." || strings.HasPrefix(rest, ".."+string(filepath.Separator)) {
			return "", false
		}
		return filepath.ToSlash(rest), true
	}
	root, name = p.clean(root), p.clean(name)
	if !p.isAbs(root) || !p.isAbs(name) {
		return "", false
	}
	same := func(a, b string) bool { return a == b || p.windows() && strings.EqualFold(a, b) }
	if same(root, name) {
		return ".", true
	}
	top := root // with its separator at the end: C:\demo\, /demo/ (C:\ and / have it)
	if !p.separator(top[len(top)-1]) {
		top += map[bool]string{true: `\`, false: "/"}[p.windows()]
	}
	if len(name) <= len(top) || !same(name[:len(top)], top) {
		return "", false
	}
	return strings.ReplaceAll(name[len(top):], `\`, "/"), true
}

// caseAside says whether the system's disk reads names case aside: the Mac's (APFS and HFS+ as they come) and
// Windows'. Linux's doesn't.
func (p fencePaths) caseAside() bool {
	goos := p.goos
	if goos == "" {
		goos = runtime.GOOS
	}
	return goos == "darwin" || goos == "windows"
}

// parts are a path's names, its volume aside.
func (p fencePaths) parts(name string) []string {
	return strings.FieldsFunc(name[len(p.volume(name)):], func(r rune) bool { return r < 0x80 && p.separator(byte(r)) })
}

// belowAnyCase is below for the app's support folder, where a path the fence reads as outside it would reach
// the owner's secrets: fail-closed, a path the disk could read as root or below it counts as in it. Case aside
// where the disk reads names that way (caseAside: ~/library/application support/agentdeck is the folder on a
// Mac, RWIN4 B2), name by name, so a letter that changes length as it folds (K, the Kelvin sign) still matches.
// On this computer, the disk is asked too (belowOnDisk): it knows its own folding and its other names for a
// folder (the Mac's /System/Volumes/Data/Users is /Users), which no spelling rule does. The rest keeps name's case.
func (p fencePaths) belowAnyCase(root, name string) (string, bool) {
	if rest, in := p.below(root, name); in {
		return rest, true
	}
	if p.caseAside() {
		root, name = p.clean(root), p.clean(name)
		want, have := p.parts(root), p.parts(name)
		if p.isAbs(root) && p.isAbs(name) && strings.EqualFold(p.volume(root), p.volume(name)) && len(have) >= len(want) &&
			slices.EqualFunc(want, have[:len(want)], strings.EqualFold) {
			if len(have) == len(want) {
				return ".", true
			}
			return strings.Join(have[len(want):], "/"), true
		}
	}
	if p.here() {
		return belowOnDisk(root, name)
	}
	return "", false
}

// belowOnDisk is name's rest below root when name, or one of the folders it's in, is root on this computer's
// disk (os.SameFile: the same file whatever its spelling, links followed). The parts of name that aren't there
// yet are kept as written.
func belowOnDisk(root, name string) (string, bool) {
	want, err := os.Stat(root)
	if err != nil || !filepath.IsAbs(name) {
		return "", false
	}
	var rest []string
	for dir := filepath.Clean(name); ; dir = filepath.Dir(dir) {
		if info, err := os.Stat(dir); err == nil && os.SameFile(info, want) {
			if len(rest) == 0 {
				return ".", true
			}
			slices.Reverse(rest)
			return strings.Join(rest, "/"), true
		}
		if filepath.Dir(dir) == dir {
			return "", false
		}
		rest = append(rest, filepath.Base(dir))
	}
}

// resolveExisting and resolvePhysical are the package's, on this computer's disk; for another system, the
// path cleaned (no disk of that system's to ask).
func (p fencePaths) resolveExisting(name string) string {
	if p.here() {
		return resolveExisting(name)
	}
	return p.clean(name)
}

func (p fencePaths) resolvePhysical(name string) string {
	if p.here() {
		return resolvePhysical(name)
	}
	return p.clean(name)
}
