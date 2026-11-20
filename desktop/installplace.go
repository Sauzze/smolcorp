package desktop

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Putting Smolcorp's programs in its folder over the ones there, as an install
// or an update does on Windows (install_windows.go). It's here, with no build
// tag, so the tests run it everywhere.
//
// Windows won't replace a program while it runs, as Smolcorp itself does, or
// smolcorp-cli.exe in an agent's hook, but lets it be renamed. So each old
// program is renamed aside first, to <name>.old-<n>, and the new one moved in.
// Both or neither: each new file is written out whole, beside its place,
// before any old one moves, and if either can't be put in place every move is
// undone, so the folder holds the old pair as it was. The ones set aside go
// once nothing runs them: at once if they can, or at the next start
// (removeLeftovers).

// installedPrograms are the programs Windows' installed folder holds.
var installedPrograms = []string{"Smolcorp.exe", "smolcorp-cli.exe"}

// placeRename and placeRemove are the moves placing makes: variables, so the
// tests can hold a file in use, as Windows does.
var (
	placeRename = os.Rename
	placeRemove = os.Remove
)

// newProgram is a program to put in place: the new file, and where it goes.
type newProgram struct{ from, to string }

// placePrograms puts each new program at its place, or none of them. Its error
// is one plain sentence naming the program that couldn't be placed.
func placePrograms(programs []newProgram) error {
	staged := make([]string, len(programs))
	unstage := func() {
		for _, temp := range staged {
			if temp != "" {
				_ = placeRemove(temp)
			}
		}
	}
	for i, p := range programs {
		temp, err := stageCopy(p.from, p.to)
		if err != nil {
			unstage()
			return couldntPlace(p.to, err, nil)
		}
		staged[i] = temp
	}
	// moved are the steps taken so far, undone last first.
	type step struct {
		to, aside string // aside: where the old program went, "" for none
		in        bool   // the new one is at to
	}
	var moved []step
	undo := func() error {
		var failed error
		for i := len(moved) - 1; i >= 0; i-- {
			s := moved[i]
			var err error
			switch {
			case s.aside != "":
				// Over the new one, which nothing runs yet.
				err = placeRename(s.aside, s.to)
			case s.in:
				err = placeRemove(s.to)
			}
			if err != nil && failed == nil {
				failed = err
			}
		}
		return failed
	}
	for i, p := range programs {
		s := step{to: p.to}
		if _, err := os.Lstat(p.to); err == nil {
			aside, err := renameAside(p.to)
			if err != nil {
				unstage()
				return couldntPlace(p.to, err, undo())
			}
			s.aside = aside
		}
		moved = append(moved, s)
		if err := placeRename(staged[i], p.to); err != nil {
			unstage()
			return couldntPlace(p.to, err, undo())
		}
		staged[i] = ""
		moved[len(moved)-1].in = true
	}
	// The old ones nothing runs go now; the others at the next start.
	for _, s := range moved {
		if s.aside != "" {
			_ = placeRemove(s.aside)
		}
	}
	return nil
}

// stageCopy writes from's bytes to a new file beside to, and says its name:
// made by this call alone (os.CreateTemp's O_EXCL, under a random name), 0600
// from the start (Windows reads only its write bit, and runs it all the same),
// synced to the disk, and taken away on any failure. Renamed into place, it is
// never half a program.
func stageCopy(from, to string) (string, error) {
	source, err := os.Open(from)
	if err != nil {
		return "", err
	}
	defer source.Close()
	target, err := os.CreateTemp(filepath.Dir(to), "."+filepath.Base(to)+".new-*")
	if err != nil {
		return "", err
	}
	_, err = io.Copy(target, source)
	if err == nil {
		err = target.Sync()
	}
	if closed := target.Close(); err == nil {
		err = closed
	}
	if err != nil {
		_ = placeRemove(target.Name())
		return "", err
	}
	return target.Name(), nil
}

// renameAside moves the program at path to the first free <path>.old-<n>, and
// says where.
func renameAside(path string) (string, error) {
	for n := 1; n <= 99; n++ {
		aside := fmt.Sprintf("%s.old-%d", path, n)
		if _, err := os.Lstat(aside); err == nil {
			continue // one an earlier update set aside still runs
		}
		if err := placeRename(path, aside); err != nil {
			return "", err
		}
		return aside, nil
	}
	return "", errors.New("99 earlier copies are still set aside beside it")
}

// couldntPlace is the sentence for a program that couldn't be put at to, and
// whether the folder is as it was.
func couldntPlace(to string, err, undoFailed error) error {
	verb := "update"
	if _, missing := os.Lstat(to); missing != nil {
		verb = "install"
	}
	said := fmt.Sprintf("Smolcorp couldn't %s %s: %s.", verb, filepath.Base(to), plainReason(err))
	if undoFailed != nil {
		// Words that fit a first install too, where there were no old programs.
		return errors.New(said + " Undoing that failed too (" + plainReason(undoFailed) +
			"), so open the Smolcorp you downloaded again to finish.")
	}
	return errors.New(said + " Nothing was changed.")
}

// plainReason is what the system said, without the operation and paths Go puts
// before it: "access is denied", say.
func plainReason(err error) string {
	var link *os.LinkError
	var path *os.PathError
	switch {
	case errors.As(err, &link):
		err = link.Err
	case errors.As(err, &path):
		err = path.Err
	}
	text := strings.TrimSuffix(strings.TrimSpace(err.Error()), ".")
	if runes := []rune(text); len(runes) > 1 && unicode.IsUpper(runes[0]) && unicode.IsLower(runes[1]) {
		runes[0] = unicode.ToLower(runes[0])
		text = string(runes)
	}
	return text
}

// removeLeftovers takes away, quietly, the programs an update set aside in
// dir (<name>.old-<n>) and the new files one didn't finish (.<name>.new-*),
// for each of names: those still in use stay for the next start.
func removeLeftovers(dir string, names []string) {
	for _, name := range names {
		for _, pattern := range []string{name + ".old-*", "." + name + ".new-*"} {
			found, _ := filepath.Glob(filepath.Join(dir, pattern))
			for _, path := range found {
				_ = placeRemove(path)
			}
		}
	}
}

// startThenMakeWay puts the copy just installed in place of a Smolcorp that
// runs, as the auto-install does (installAndRelaunch). running finds the copy
// running now, if one listens, before anything starts: the installed copy
// listens too, once it holds the instance lock, and must never be the one
// asked. start then starts the installed copy, as one taking another's place,
// which waits for the lock; only once it has started is the copy found asked
// to make way. If it can't start, that copy isn't asked and runs on, and
// start's error says why. Here, with no build tag, so the tests run the order
// everywhere.
func startThenMakeWay(running func() (ask func() bool, done func()), start func() error) error {
	ask, done := running()
	defer done()
	if err := start(); err != nil {
		return err
	}
	ask()
	return nil
}
