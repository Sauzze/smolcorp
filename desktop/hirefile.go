package desktop

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The core's reader of the files a desk can write (SA-X5, the sandbox plan's §A3.5.4 and N2). The core reads as the
// owner, and a desk can put a link, a hard link, a fifo or a huge file where the core will look: a STATUS.md linked to
// a key, a transcript_path naming a key (P4), a Claude Code session file naming another program's socket. So every
// such read goes through here:
//   - the file is named inside a place the core chose itself (a desk's folder, a Claude account's folder, the demo's
//     folder). The place is opened once; from there each folder on the way, and the file, is opened by its name, with
//     no link followed: a link anywhere below the place refuses the read, not only a link at the end;
//   - only a regular file with one name (st_nlink == 1: a hard link to a key made in a desk's folder is refused);
//   - opened in a way that never waits (a fifo can't hold the core), and judged as opened, never by a second look;
//   - readHireFile reads a whole file only up to a cap. openHireFile's callers read a tail or a chunk of their own
//     reach (a transcript can be long), and say so.
//
// What the place itself is stays the caller's: it is a path the core knows (a hire's recorded folder, an account
// folder of the owner's confirmed list), and links in it are followed, as the owner may have made them.

// errHireFile is a file the reader won't read: outside its place, behind a link, not a plain file of one name.
var errHireFile = errors.New("not a file the core reads from a desk")

// openHireFile opens path, which must lie below place, for reading, by the rules above. The caller closes it.
func openHireFile(place, path string) (*os.File, os.FileInfo, error) {
	if place == "" || !filepath.IsAbs(place) || !filepath.IsAbs(path) {
		return nil, nil, errHireFile
	}
	rel, err := filepath.Rel(place, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return nil, nil, errHireFile
	}
	names := strings.Split(rel, string(filepath.Separator))
	dir, err := openHireFolder(place, names[:len(names)-1])
	if err != nil {
		return nil, nil, err
	}
	defer dir.Close()
	name := names[len(names)-1]
	file, err := openWatched(dir, name) // O_NOFOLLOW and O_NONBLOCK; on Windows, a plain file looked at first
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err == nil && info.Mode().IsRegular() && fileLinks(file, info) == 1 {
		// The name still names the file opened (Windows opens after its look, so a link put in between is caught here).
		if at, err := dir.Lstat(name); err == nil && at.Mode().IsRegular() && os.SameFile(info, at) {
			return file, info, nil
		}
	}
	file.Close()
	return nil, nil, errHireFile
}

// openHireFolder opens the folder names, one name after another, below place: each a folder, never a link. The
// caller closes it.
func openHireFolder(place string, names []string) (*os.Root, error) {
	dir, err := os.OpenRoot(place)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		next, err := hireFolderStep(dir, name)
		dir.Close()
		if err != nil {
			return nil, err
		}
		dir = next
	}
	return dir, nil
}

// hireFolderStep opens the folder name inside dir: a folder there, never a link, and the one looked at.
func hireFolderStep(dir *os.Root, name string) (*os.Root, error) {
	if name == "" || name == "." || name == ".." {
		return nil, errHireFile
	}
	before, err := dir.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() { // a link is never a folder to Lstat
		return nil, errHireFile
	}
	next, err := dir.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	// A link swapped in after the look leads to another folder: what opened isn't what was looked at.
	if now, err := next.Stat("."); err != nil || !os.SameFile(before, now) {
		next.Close()
		return nil, errHireFile
	}
	return next, nil
}

// readHireFile is the whole of path, below place, by openHireFile's rules: refused past most bytes.
func readHireFile(place, path string, most int64) ([]byte, error) {
	file, info, err := openHireFile(place, path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if info.Size() > most {
		return nil, errHireFile
	}
	data, err := io.ReadAll(io.LimitReader(file, most+1))
	if err == nil && int64(len(data)) > most {
		return nil, errHireFile
	}
	return data, err
}

// transcriptPlace is the folder a transcript at path is read through: the Claude folder of
// <folder>/projects/<project>/<name>.jsonl, Claude Code's own layout, or the support folder for an ACP desk's,
// which the core writes itself (acptranscript.go); "" for any other path, which is never read. A subagent's
// transcript, below its session's, is read through its session's place.
func transcriptPlace(path string) string {
	if path == "" || !filepath.IsAbs(path) || filepath.Ext(path) != ".jsonl" {
		return ""
	}
	path = filepath.Clean(path)
	if filepath.Dir(path) == filepath.Join(supportDir, acpTranscriptFolder) {
		return supportDir
	}
	if projects := filepath.Dir(filepath.Dir(path)); filepath.Base(projects) == "projects" {
		return filepath.Dir(projects)
	}
	return ""
}

// openTranscript opens the transcript at path through its place (transcriptPlace).
func openTranscript(path string) (*os.File, os.FileInfo, error) {
	return openHireFile(transcriptPlace(path), path)
}

// transcriptInfo is the transcript at path as the reader opens it, never what a link there leads to (its size and
// time, for a window that asks again only when they change); false where the reader won't read it.
func transcriptInfo(path string) (os.FileInfo, bool) {
	file, info, err := openTranscript(path)
	if err != nil {
		return nil, false
	}
	file.Close()
	return info, true
}

// unconfirmedClaudeFolder is whether a transcript at path, which transcriptKnown didn't take, is in a Claude folder
// by its layout (<folder>/projects/<project>/<session>.jsonl) outside Smolcorp's own: what the owner could confirm in
// setup. Nothing is read or resolved.
func unconfirmedClaudeFolder(path string) bool {
	place := transcriptPlace(path)
	rel, err := filepath.Rel(supportDir, place)
	ours := err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
	return place != "" && !ours
}

// transcriptKnown is whether a transcript at path, as a hook event with desk's key names it ("" for the owner's
// sessions' key), lies in a place that desk's transcripts are kept (N2: by place):
//   - a desk with its own Claude folder (SA-X8, deskOwnClaude): that folder alone, never an account's or another
//     desk's;
//   - any other (the owner's sessions, a launcher's, a plain or a T3 desk): a Claude account's folder of the owner's,
//     ~/.claude or a confirmed account, and never a desk's Claude folder (isDeskClaudePlace).
//
// A path anywhere else (a key in the support folder, an ACP desk's transcript, another desk's place) is never taken.
// The folder is compared as written, and as the core's own folder resolves; the path itself is never resolved, as its
// links are a desk's. With the deck's lock held.
func (d *Deck) transcriptKnown(desk, path string) bool {
	place := transcriptPlace(path)
	if place == "" || place == supportDir {
		return false
	}
	if own, ok := deskOwnClaude(desk); ok {
		if place == own {
			return true
		}
		resolved, err := filepath.EvalSymlinks(own)
		return err == nil && resolved == place
	}
	if isDeskClaudePlace(place) {
		return false
	}
	return slices.ContainsFunc(claudeAccountList(d.config.ClaudeProfiles), func(p claudeProfile) bool {
		folder := filepath.Clean(p.Dir)
		resolved, err := filepath.EvalSymlinks(folder)
		if isDeskClaudePlace(folder) || err == nil && isDeskClaudePlace(resolved) {
			return false // an account named at a desk's folder is still that desk's
		}
		return err == nil && resolved == place || folder == place
	})
}
