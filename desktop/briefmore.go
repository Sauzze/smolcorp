package desktop

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// A later part of a brief, for a hire that is already working, as the Mac's
// BriefMore: the text is added, dated, to the end of the hire's BRIEF file
// (never written over), and the hire is mailed a short note to read it. These
// are the file's side; POST /setup/gm/brief, which checks the request and
// sends the note through the mail gate, is the route's.

// briefMoreNote is the note the hire gets, with file as it should read it.
func briefMoreNote(file string) string { return "Your brief has a new part: read " + file }

// briefMorePart is what the part looks like in the file: a heading with the date, then the text.
func briefMorePart(text string, date time.Time) string {
	return "\n\n## New part, " + date.Format("2006-01-02 15:04") + "\n\n" + trimSwiftSpace(text) + "\n"
}

// briefFileOf is the brief file a hire reads: the path written down when it
// was hired, and only that. A hire from before that was kept is found only if
// it is managed, since its folder in Smolcorp's is its alone; any other is not
// guessed at, as a project folder can hold several hires' briefs and the
// owner's own. "" when there's none.
func briefFileOf(hire Hire, hires string) string {
	if hire.Brief != "" {
		if plainFile(hire.Brief) {
			return hire.Brief
		}
		return ""
	}
	if hire.Worktree == "" {
		return ""
	}
	folder := filepath.Join(hires, hire.ID)
	entries, _ := os.ReadDir(folder)
	var names []string
	for _, entry := range entries {
		if name := entry.Name(); strings.HasPrefix(name, "BRIEF") && strings.HasSuffix(name, ".md") {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	for _, name := range names {
		if path := filepath.Join(folder, name); plainFile(path) {
			return path
		}
	}
	return ""
}

// plainFile is whether path is a real file, not a link to somewhere else.
func plainFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

// openBriefFile opens a brief to add to it; tests put a file in the brief's place on the way.
var openBriefFile = os.OpenFile

// appendBriefPart adds the part to the end of file, as one write. The file is
// opened, never made, and a link put in its place since it was found is not
// followed: the open refuses one where it can, and on Windows, where it can't
// (noFollow is nothing there), the file opened must be the one checked.
func appendBriefPart(file, text string, now time.Time) error {
	checked, err := os.Lstat(file)
	if err != nil || !checked.Mode().IsRegular() {
		return os.ErrNotExist
	}
	out, err := openBriefFile(file, os.O_WRONLY|os.O_APPEND|noFollow, 0)
	if err != nil {
		return err
	}
	if opened, err := out.Stat(); err != nil || !os.SameFile(checked, opened) {
		out.Close()
		return os.ErrNotExist
	}
	_, err = out.WriteString(briefMorePart(text, now))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	return err
}
