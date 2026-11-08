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

// checkHireFolder checks given as the folder for new hires. ~ is home.
func checkHireFolder(given, home string) hireFolderCheck {
	const example = "such as ~/Projects"
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
	if filepath.Dir(folder) == folder {
		return problem("That's your computer's whole disk. Choose a folder inside your home folder, "+example+".", hireFolderChoose)
	}
	rel, err := filepath.Rel(base, folder)
	if err == nil && rel == "." {
		return problem("That's your whole home folder. Choose a folder inside it, "+example+", so hires keep to their projects.", hireFolderChoose)
	}
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return problem("Choose a folder inside your home folder, "+example+".", hireFolderChoose)
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if parts[0] == "Library" || parts[0] == "AppData" || slices.ContainsFunc(parts, func(p string) bool {
		return strings.HasPrefix(p, ".") || strings.HasSuffix(p, ".app")
	}) {
		return problem("That folder holds your computer's or your apps' own files. Choose a project folder, "+example+".", hireFolderChoose)
	}
	shown := "~" + string(filepath.Separator) + rel
	info, err := os.Stat(folder)
	if err != nil {
		return problem("There's no folder at "+shown+" yet.", hireFolderCreate)
	}
	check.exists = true
	if !info.IsDir() {
		return problem(shown+" is a file, not a folder.", hireFolderChoose)
	}
	if !canWriteIn(folder) {
		return problem("Smolcorp can't write in "+shown+". Choose a folder of your own, "+example+".", hireFolderChoose)
	}
	return check
}

// hireFolderSuggestion is the folder to suggest when none is set: the first
// likely one that's there and passes the checks, else ~/Smolcorp.
func hireFolderSuggestion(home string) string {
	for _, name := range likelyProjectFolders {
		if found := checkHireFolder(filepath.Join(home, filepath.FromSlash(name)), home); found.exists && found.problem == "" {
			return found.folder
		}
	}
	return checkHireFolder(filepath.Join(home, fallbackHireFolder), home).folder
}

// hireFolderState is GET /setup/hire-folder: the folder set (saved), or,
// with none, the one suggested; or the one asked about (?check=, as you
// type). folder is what's set or asked, suggested the suggestion when
// neither; exists, problem and fix are about whichever is shown.
func hireFolderState(saved string, asked *string, home string) map[string]any {
	if asked != nil {
		return hireFolderJSON(checkHireFolder(*asked, home), "", home)
	}
	if saved == "" {
		suggested := hireFolderSuggestion(home)
		shown := checkHireFolder(suggested, home)
		shown.folder = ""
		// Not there yet is no problem for the suggestion: it's made once accepted.
		if shown.fix == hireFolderCreate {
			shown.problem, shown.fix = "", ""
		}
		return hireFolderJSON(shown, suggested, home)
	}
	shown := checkHireFolder(saved, home)
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
	found := checkHireFolder(given, home)
	if create && found.fix == hireFolderCreate {
		if err := os.MkdirAll(found.folder, 0o755); err != nil {
			rel, _ := filepath.Rel(realPath(home), found.folder)
			found.problem = "Couldn't make ~" + string(filepath.Separator) + rel + ". Choose another folder, such as ~/Projects."
			found.fix = hireFolderChoose
		} else {
			found = checkHireFolder(found.folder, home)
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
