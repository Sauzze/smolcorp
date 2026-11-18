package sandbox

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"syscall"
)

// DeskWrites is whether the desk can write at place, a resolved path, or, when it is a folder, at a name in it: a deny
// of the folder alone, or a row that leaves a place below it the desk's (an allow there), says so. A desk's Policy
// denies only what it holds: an owner's folder no row holds (~/dotfiles) is one the desk writes. It is the "leads
// where?" test of the git rows' links (FXGITROWS2R8's deskWrites, the same test), used by the core for a link below a
// desk's place that leads to a repo (FXHIREPROJECTSR7).
func (p *Policy) DeskWrites(place string) (bool, error) {
	if v, _ := p.Decide(place, Write); v != Deny {
		return true, nil
	}
	info, err := os.Stat(filepath.FromSlash(place))
	switch {
	case errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR):
		return false, nil // not there: the desk can't make it
	case err != nil:
		return false, err
	case !info.IsDir():
		return false, nil
	}
	if v, _ := p.Decide(path.Join(place, "x"), Write); v != Deny {
		return true, nil
	}
	k := key(p.goos, place)
	return slices.ContainsFunc(p.rules, func(r Rule) bool {
		return r.Write == Allow && r.key != "" && r.key != k && under(r.key, k)
	}), nil
}
