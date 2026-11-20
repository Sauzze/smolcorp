package desktop

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The recap a fire writes, as the Mac's HireDesk.writeRecap: recaps/<hire>.md
// in the support folder, mode 0600, holding who, task, folder, branch,
// worktree, when, the cost the window showed and the last STATUS lines.

var costShown = regexp.MustCompile(`(?i)(?:total )?cost[: ]+(\$\d+(?:\.\d+)?)`)

// visibleCost is the last cost the window shows, such as "$1.23", or "".
func visibleCost(capture string) string {
	if all := costShown.FindAllStringSubmatch(capture, -1); len(all) > 0 {
		return all[len(all)-1][1]
	}
	return ""
}

// recapFile opens rel inside dir for the recap, or nil: only a plain file with
// one name, reached without leaving dir and through no link. A hire writes its
// own folder, so a link there (at any depth) could point anywhere the user can
// read, and a pipe would hold the fire for good: read through the core's
// reader (hirefile.go).
func recapFile(dir, rel string) *os.File {
	f, _, err := openHireFile(dir, filepath.Join(dir, rel))
	if err != nil {
		return nil
	}
	return f
}

// lastWords is the last count lines with words in them of the first of the
// files (named inside dir) that can be read (its last 4096 bytes), each cut to
// 200 characters.
func lastWords(count int, dir string, files ...string) []string {
	for _, name := range files {
		f := recapFile(dir, name)
		if f == nil {
			continue
		}
		if info, err := f.Stat(); err == nil && info.Size() > 4096 {
			f.Seek(info.Size()-4096, io.SeekStart)
		}
		data, _ := io.ReadAll(io.LimitReader(f, 4096))
		f.Close()
		var lines []string
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				lines = append(lines, firstRunes(line, 200))
			}
		}
		return lines[max(len(lines)-count, 0):]
	}
	return nil
}

// recapTask is what the hire was asked to do: the first line with words of
// the first BRIEF*.md in place that is a plain file, without its heading marks.
func recapTask(place string) string {
	names, _ := filepath.Glob(filepath.Join(place, "BRIEF*.md"))
	for _, name := range names {
		f := recapFile(place, filepath.Base(name))
		if f == nil {
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(f, 2048))
		f.Close()
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				return firstRunes(strings.Trim(line, "# "), 200)
			}
		}
	}
	return ""
}

// writeRecap writes the hire's recap and gives its path, "" if it couldn't be written.
func writeRecap(h Hire, capture string, worktreeRemoved bool, now time.Time) string {
	return writeRecapMoved(h, capture, worktreeRemoved, false, now)
}

// writeRecapMoved is writeRecap for an end that tried to take the hire's worktree away: moved says it was kept because
// it was moved or replaced since its hire (hireTreeMoved), so no git ran there, and its line says so and what to do
// (FXGMHIRENOGIT126R6).
func writeRecapMoved(h Hire, capture string, worktreeRemoved, moved bool, now time.Time) string {
	// Where it worked, read only while that still resolves to itself (the accessors, FXCOREWORKTREES126): a worktree or
	// folder moved or replaced since its hire isn't read, so nothing a link leads to (a GM's folder) is presented as the
	// hire's (RT126GMHIRENOGIT126R3's O3).
	place, _ := h.checkedFolder()
	if !h.Worktree.none() && !worktreeRemoved {
		place, _ = h.checkedTree()
	}
	// Its own folder first, where an engine hire's brief and status are with or without a worktree (hireBrief).
	own := place
	if plainHireID(h.ID) {
		own = hireFolder(supportDir, h.ID)
	}
	var status []string
	for _, where := range []string{own, place} {
		if where == "" {
			continue
		}
		if status = lastWords(5, where, "STATUS.md", filepath.Join(".team", "STATUS.md")); len(status) > 0 {
			break
		}
	}
	task := "not written down"
	for _, where := range []string{own, place} {
		if where == "" {
			continue
		}
		if t := recapTask(where); t != "" {
			task = t
			break
		}
	}
	who := h.Name
	if h.Role != "" {
		who += " (" + h.Role + ")"
	}
	if h.Nickname != "" {
		who += ", called " + h.Nickname
	}
	lines := []string{"# Recap: " + calledHire(h), "", "- Who: " + who, "- Task: " + task, "- Folder: " + h.Folder.storedOnly()}
	if h.Branch != "" {
		lines = append(lines, "- Branch: "+h.Branch)
	}
	if tree := h.Worktree.storedOnly(); tree != "" {
		line := fmt.Sprintf("- Worktree: %s (%s)", tree, map[bool]string{true: "removed", false: "kept"}[worktreeRemoved])
		if !worktreeRemoved && moved {
			// What works whatever is there now: git's own removal refuses a link, or empties a folder it leads to
			// (RT126GMHIRENOGIT126R3, N1); with nothing left at the path, prune drops the registration.
			line += ". It was moved or replaced, so Smolcorp left it alone. Don't run git in it. To remove it yourself, delete what's at " +
				tree + " (only the link, if it's a link; a folder, once you've copied out what you need, without git), then run `git worktree prune` in your own checkout."
		} else if !worktreeRemoved {
			// The owner's git in it could run a repo the hire made there (FXHIREPROJECTSR9, RT126FIXREVIEW-HIREPROJECTSR8 N1).
			branch := "<its branch>"
			if h.Branch != "" {
				branch = h.Branch
			}
			line += ". Don't run git in it: from your own checkout, `git log " + branch + "` shows its work and `git worktree remove --force " + tree + "` removes it, deleting what it left uncommitted: open or copy those files first, without git."
		}
		lines = append(lines, line)
	}
	if h.Started > 0 {
		lines = append(lines, "- Hired: "+time.UnixMilli(int64(h.Started)).UTC().Format(time.RFC3339))
	}
	cost := visibleCost(capture)
	if cost == "" {
		cost = "not shown"
	}
	lines = append(lines, "- Fired: "+now.UTC().Format(time.RFC3339), "- Cost: "+cost, "", "## Last status", "")
	if len(status) == 0 {
		lines = append(lines, "Nothing was written to STATUS.md.")
	}
	for _, s := range status {
		lines = append(lines, "- "+s)
	}
	name := h.ID
	if !plainHireID(name) {
		name = hireSlug(name)
	}
	dir := filepath.Join(supportDir, "recaps")
	if os.MkdirAll(dir, 0o700) != nil {
		return ""
	}
	file := filepath.Join(dir, name+".md")
	if writePrivate(file, []byte(strings.Join(lines, "\n")+"\n")) != nil {
		return ""
	}
	return file
}
