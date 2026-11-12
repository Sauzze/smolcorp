package desktop

import (
	"path"
	"path/filepath"
	"runtime"
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
