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
// one name, reached without leaving dir. A hire writes its own folder, so a
// link there (at any depth) could point anywhere the user can read, and a pipe
// would hold the fire for good: os.Root refuses a part that leaves dir,
// O_NONBLOCK keeps a pipe from blocking the open, O_NOFOLLOW refuses a link as
// the last part, and a hard link (more than one name) is refused as the
// watchers do.
func recapFile(dir, rel string) *os.File {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil
	}
	defer root.Close()
	f, err := openWatched(root, rel)
	if err != nil {
		return nil
	}
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() || fileLinks(f, info) > 1 {
		f.Close()
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
	place := h.Folder
	if h.Worktree != "" && !worktreeRemoved {
		place = h.Worktree
	}
	own := place
	if h.Worktree != "" {
		own = hireFolder(supportDir, h.ID)
	}
	var status []string
	for _, where := range []string{own, place} {
		if status = lastWords(5, where, "STATUS.md", filepath.Join(".team", "STATUS.md")); len(status) > 0 {
			break
		}
	}
	task := "not written down"
	for _, where := range []string{own, place} {
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
	lines := []string{"# Recap: " + calledHire(h), "", "- Who: " + who, "- Task: " + task, "- Folder: " + h.Folder}
	if h.Branch != "" {
		lines = append(lines, "- Branch: "+h.Branch)
	}
	if h.Worktree != "" {
		lines = append(lines, fmt.Sprintf("- Worktree: %s (%s)", h.Worktree, map[bool]string{true: "removed", false: "kept"}[worktreeRemoved]))
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
