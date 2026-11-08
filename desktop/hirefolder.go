package desktop

// The folder new hires work in, in one move (Start here's first step): a
// folder to suggest when none is set, the checks a folder must pass, each
// problem in a plain sentence with its fix, and keeping one. As the Mac
// app's HireFolder, with the same JSON.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// likelyProjectFolders are where projects usually are already, in order: the
// first one there that passes the checks is suggested. Only these, never a search.
var likelyProjectFolders = []string{"Projects", "projects", "Developer", "Code", "code", "src", "dev", "repos", "source/repos"}

// fallbackHireFolder is what's suggested when none of those is there: made
// only once it's accepted.
const fallbackHireFolder = "Smolcorp"

// What the page offers for a problem: make the folder, or choose another.
const (
	hireFolderCreate = "create"
	hireFolderChoose = "choose"
)

// hireFolderCheck is a folder as checked: its full path, whether it's there,
// and what's wrong with it, if anything, with its fix.
type hireFolderCheck struct {
	folder  string
	exists  bool
	problem string
	fix     string
}

// hireFolderUse is how a folder is checked: as one being chosen; as the one
// already set, only for what hiring needs (there, a folder, writable), so a
// folder kept before these checks stays as it was; or as one being kept.
// Windows judges whether a folder is writable only then, by really making a
// file there and taking it away (see hirefolder_windows.go).
type hireFolderUse int

const (
	choosingHireFolder hireFolderUse = iota
	savedHireFolder
	keepingHireFolder
)

// systemFolders are where hires may not work, by a folder's first part from
// the disk's top, wherever the folder isn't inside the home folder: as the
// Mac's HireFolder has them.
var systemFolders = []string{"applications", "bin", "boot", "cores", "dev", "etc", "lib", "lib32", "lib64", "libx32",
	"library", "private", "proc", "run", "sbin", "snap", "sys", "system", "tmp", "usr", "var",
	"windows", "program files", "program files (x86)", "programdata"}

// diskFolders are where other disks show up: one of these, or a disk right
// inside it (/mnt/data), is a whole disk.
var diskFolders = []string{"volumes", "mnt", "media"}

// hireFolderExample is the folder named in every problem, as this system writes paths.
var hireFolderExample = "such as ~" + string(filepath.Separator) + "Projects"

// checkHireFolder checks given as the folder for new hires, as use says. ~ is home.
func checkHireFolder(given, home string, use hireFolderUse) hireFolderCheck {
	example := hireFolderExample
	trimmed := strings.TrimSpace(given)
	if trimmed == "" {
		return hireFolderCheck{problem: "Type the folder's full path, " + example + ".", fix: hireFolderChoose}
	}
	expanded := trimmed
	if trimmed == "~" || strings.HasPrefix(trimmed, "~/") || strings.HasPrefix(trimmed, `~\`) {
		expanded = filepath.Join(home, trimmed[1:])
	}
	if !filepath.IsAbs(expanded) {
		return hireFolderCheck{problem: "Give the folder's full path, " + example + ".", fix: hireFolderChoose}
	}
	folder, base := realPath(expanded), realPath(home)
	check := hireFolderCheck{folder: folder}
	problem := func(text, fix string) hireFolderCheck {
		check.problem, check.fix = text, fix
		return check
	}
	if use != savedHireFolder {
		if refusal := hireFolderRefusal(folder, base); refusal != "" {
			return problem(refusal, hireFolderChoose)
		}
	}
	shown := shownHireFolder(folder, base)
	info, err := os.Stat(folder)
	if err != nil {
		return problem("There's no folder at "+shown+" yet.", hireFolderCreate)
	}
	check.exists = true
	if !info.IsDir() {
		return problem(shown+" is a file, not a folder.", hireFolderChoose)
	}
	if !canWriteIn(folder, use == keepingHireFolder) {
		return problem("Smolcorp can't write in "+shown+". Choose a folder of your own, "+example+".", hireFolderChoose)
	}
	return check
}

// hireFolderRefusal says why folder (worked out by realPath) can't be a new
// hires folder, or "" if it can. Names are compared whatever their letter
// case, as a Mac's or Windows' disk does.
func hireFolderRefusal(folder, home string) string {
	example := hireFolderExample
	rel, err := filepath.Rel(home, folder)
	if err == nil && rel == "." {
		return "That's your whole home folder. Choose a folder inside it, " + example + ", so hires keep to their projects."
	}
	inside := err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
	from := rel
	if !inside {
		from = strings.TrimPrefix(folder, filepath.VolumeName(folder))
	}
	var parts []string
	for _, part := range strings.Split(from, string(filepath.Separator)) {
		if part != "" {
			parts = append(parts, strings.ToLower(part))
		}
	}
	first := ""
	if len(parts) > 0 {
		first = parts[0]
	}
	if !inside {
		deepest := 2
		if first == "media" {
			deepest = 3 // /media/<user>/<disk>
		}
		if filepath.Dir(folder) == folder || len(parts) == 0 || slices.Contains(diskFolders, first) && len(parts) <= deepest {
			return "That's a whole disk. Choose a project folder, " + example + "."
		}
	}
	system := slices.Contains(systemFolders, first)
	if inside {
		system = first == "library" || first == "appdata"
	}
	if system || slices.ContainsFunc(parts, func(p string) bool { return strings.HasSuffix(p, ".app") }) {
		return "That folder holds your computer's or your apps' own files. Choose a project folder, " + example + "."
	}
	return hiddenHireFolderRefusal(parts, example)
}

// hiddenHireFolderRefusal says why a path with a hidden (dot) folder in it
// can't be used, saying so: the folder itself, or one it's inside. "" if
// there's none. As the Mac's HireFolder.hiddenRefusal.
func hiddenHireFolderRefusal(parts []string, example string) string {
	at := slices.IndexFunc(parts, func(p string) bool { return strings.HasPrefix(p, ".") })
	switch {
	case at < 0:
		return ""
	case at == len(parts)-1:
		return "A hidden folder (one whose name starts with '.') can't be used. Choose a project folder, " + example + "."
	}
	return "A folder inside a hidden folder (one whose name starts with '.') can't be used: choose one outside it."
}

// shownHireFolder is folder with ~ for home, as the page shows it.
func shownHireFolder(folder, home string) string {
	rel, err := filepath.Rel(home, folder)
	switch {
	case err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel):
		return folder
	case rel == ".":
		return "~"
	}
	return "~" + string(filepath.Separator) + rel
}

// hireFolderSuggestion is the folder to suggest when none is set: the first
// likely one that's there and passes the checks, else ~/Smolcorp.
func hireFolderSuggestion(home string) string {
	for _, name := range likelyProjectFolders {
		if found := checkHireFolder(filepath.Join(home, filepath.FromSlash(name)), home, choosingHireFolder); found.exists && found.problem == "" {
			return found.folder
		}
	}
	return checkHireFolder(filepath.Join(home, fallbackHireFolder), home, choosingHireFolder).folder
}

// hireFolderState is GET /setup/hire-folder: the folder set (saved), or,
// with none, the one suggested; or the one asked about (?check=, as you
// type). folder is what's set or asked, suggested the suggestion when
// neither; exists, problem and fix are about whichever is shown.
func hireFolderState(saved string, asked *string, home string) map[string]any {
	if asked != nil {
		return hireFolderJSON(checkHireFolder(*asked, home, choosingHireFolder), "", home)
	}
	if saved == "" {
		suggested := hireFolderSuggestion(home)
		shown := checkHireFolder(suggested, home, choosingHireFolder)
		shown.folder = ""
		// Not there yet is no problem for the suggestion: it's made once accepted.
		if shown.fix == hireFolderCreate {
			shown.problem, shown.fix = "", ""
		}
		return hireFolderJSON(shown, suggested, home)
	}
	shown := checkHireFolder(saved, home, savedHireFolder)
	shown.folder = saved
	return hireFolderJSON(shown, "", home)
}

// hireFolderJSON is the route's answer, the same on a Mac.
func hireFolderJSON(check hireFolderCheck, suggested, home string) map[string]any {
	return map[string]any{"folder": check.folder, "suggested": suggested, "exists": check.exists, "problem": check.problem,
		"fix": check.fix, "home": realPath(home)}
}

// keepHireFolder is POST /setup/hire-folder?folder=…[&create=1]: the folder to
// keep, checked, made first when it isn't there yet and create says so; or why it can't be.
func keepHireFolder(given string, create bool, home string) (string, *hireFolderCheck) {
	found := checkHireFolder(given, home, keepingHireFolder)
	if create && found.fix == hireFolderCreate {
		if err := os.MkdirAll(found.folder, 0o755); err != nil {
			found.problem = "Couldn't make " + shownHireFolder(found.folder, realPath(home)) + ". Choose another folder, " + hireFolderExample + "."
			found.fix = hireFolderChoose
		} else {
			found = checkHireFolder(found.folder, home, keepingHireFolder)
		}
	}
	if found.problem != "" {
		return "", &found
	}
	return found.folder, nil
}

// realPath is path cleaned, with the symlinks of its part that's there
// worked out, as the Mac's check does.
func realPath(path string) string {
	path = filepath.Clean(path)
	rest := ""
	for at := path; ; {
		if resolved, err := filepath.EvalSymlinks(at); err == nil {
			return filepath.Join(resolved, rest)
		}
		up := filepath.Dir(at)
		if up == at {
			return path
		}
		rest = filepath.Join(filepath.Base(at), rest)
		at = up
	}
}
