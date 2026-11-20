package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The office's own hire into the folder for hires, on a Mac with the sandbox on (FXHIREPROJECTSR4, the owner's ruling
// of 18 Nov: a hire writes only its own project folder, one hire per repo). A desk in its sandbox never works in the
// whole projects folder, so the office asks which project first: /act's hire with no project answers with the projects
// to pick from, and nothing is hired; the hire sent again with the one picked goes into that project's own worktree, as
// any other hire. Cancel sends nothing. The sandbox off, and Windows and Linux, which have none, hire as before. A
// plain hire (on a Mac: a hire with no role, or a Claude Code role with the manager off; Codex, Gemini CLI, OpenCode
// and Grok are refused there) is asked too, where its launcher starts it in its sandbox (FXHIREPROJECTSR5, N1), and,
// since ruling (W) (FXHIREPROJECTSR7), works in a worktree of its own of the project picked, on its own branch, as the
// engine's hires do: no plain hire works in the owner's checkout.

// plainHiresSandboxed is whether a plain hire's launcher starts its agent in a sandbox on this system
// (launcherSandboxes: a Mac's), the owner's switch aside. A variable, for the tests: a deck whose hires' windows are
// stand-ins starts no launcher. Asked through plainHireSandboxed.
var plainHiresSandboxed = launcherSandboxes

// plainHireSandboxed is whether a plain hire's agent starts in a sandbox of its own (a pinned desk, another agent, no
// role, or a Claude role with the manager off): where this system's launcher makes one (plainHiresSandboxed), unless
// the owner turned the sandbox off in Setup (cfg.SandboxOff, AQ1), as the engine's desks go by it
// (engineOptions.sandboxing). One switch for every hire (FXSWITCHPLAIN, the owner's Q8), and the one question the
// office's plain hire asks for its pick, its place's check and its own worktree (hireWith; FXHIREPROJECTSR7, ruling
// (W)), and its launcher as it starts (launcherRun.sandboxed). With the switch off, a plain hire isn't asked which
// project and goes into the folder as before, as the engine's hire does (hiresSandboxed).
func plainHireSandboxed(cfg Config) bool { return plainHiresSandboxed && !cfg.SandboxOff }

// plainProjectName is whether name, a project picked in the folder for hires, can go on a plain hire's launcher's
// command line to be told to its agent (--project): a path within that folder, as hireProjectsIn lists them. A name
// that can't is only not told: the hire asks which project, as before. One that could break a line is never picked
// (hireProjectsIn, pickedProject), and is shown escaped if told (shownInWords).
func plainProjectName(name string) bool {
	return filepath.IsLocal(filepath.FromSlash(name))
}

// plainDeskIDShape is a plain hire's desk's id, as plainDeskIDNew makes it: the office's word to its launcher (--desk).
var plainDeskIDShape = regexp.MustCompile(`^plain-[0-9a-f]{12}$`)

// plainDeskID is whether id is a plain hire's desk's id the office made.
func plainDeskID(id string) bool { return plainDeskIDShape.MatchString(id) }

// plainDeskIDNew is a new plain hire's desk's id, plain-<12 hex>, as a launcher's own (plainHireTmuxID): the office
// makes it for a plain hire's worktree and branch (hire/<id>/…), and its launcher's sandbox is that desk's.
func plainDeskIDNew() string { return "plain-" + strings.ToLower(newID())[:12] }

// hirePick is what the office shows to pick a project: its question, the projects in the folder for hires by name,
// and what to do when there's none.
type hirePick struct {
	Title    string   `json:"title"`
	Folder   string   `json:"folder"`
	Projects []string `json:"projects"`
	Note     string   `json:"note"`
	None     string   `json:"none,omitempty"`
	Cancel   string   `json:"cancel"`
}

// hirePickTitle is the picker's question: the office's hire's first question, now asked by the office itself.
const hirePickTitle = "Which project should this hire work on?"

// pickFor is the picker for folder, the folder for hires, as /act's hire answers it (200, its JSON under "pick"), for
// a plain hire too: since ruling (W) it works in a worktree of its own and commits there, as any hire.
func pickFor(folder string) string {
	shown := tildePath(folder)
	pick := hirePick{Title: hirePickTitle, Folder: shown, Projects: hireProjectsIn(folder), Cancel: "Cancel",
		Note: "On this Mac, each hire works in one project of its own. These are the projects in " + shown + "."}
	if len(pick.Projects) == 0 {
		pick.Note = ""
		pick.None = "There's no project in your projects folder, " + shown + ", yet. Put one there (a folder of its own, or a git clone), then hire again."
	}
	data, _ := json.Marshal(map[string]hirePick{"pick": pick})
	return string(data)
}

// hireProjectsIn are the projects in folder, the folder for hires, by name, in order (FXHIREPROJECTSR5, B1, option
// (a)): each repo in it (a folder with a .git), and in a folder of it that isn't a repo, each repo in that, as its own
// entry by its path there (clients/acme); a folder that isn't a repo and holds none, at any depth, is a project of
// its own (notes). A folder that holds repos isn't listed itself, nor is one gitBelow can't look all through (more
// than gitBelowMost folders, or one it can't read): a hire there would be refused (gitPlaceWords). Never a link,
// wherever it leads (O3): a hire's place is a folder in the projects folder. Never one whose name, or the name of the
// folder it's in there, holds a character that could break a line of what the hire is told (breaksWords: a line end,
// an escape), so no such line reaches a hire's first message or brief (FXPICKEDPROJECTR1, B1).
func hireProjectsIn(folder string) []string {
	projects := []string{}
	for _, name := range projectFolders(folder) {
		if strings.ContainsFunc(name, breaksWords) {
			continue
		}
		at := filepath.Join(folder, name)
		if isRepoFolder(at) {
			projects = append(projects, name)
			continue
		}
		var repos []string
		for _, inner := range projectFolders(at) {
			if isRepoFolder(filepath.Join(at, inner)) && !strings.ContainsFunc(inner, breaksWords) {
				repos = append(repos, name+"/"+inner)
			}
		}
		if len(repos) > 0 {
			projects = append(projects, repos...)
		} else if found, err := gitBelow(at, true); err == nil && found.repo == "" && found.bare == "" {
			projects = append(projects, name)
		}
	}
	return projects
}

// projectFolders are the folders in folder that could be projects, by name, in order: each folder (never a link),
// but a hidden one (a name starting with a dot: .smolcorp, .team) and the folder Smolcorp keeps a repo's hires'
// worktrees in beside it (<repo>-hires, hiresFolder).
func projectFolders(folder string) []string {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return slices.DeleteFunc(slices.Clone(names), func(name string) bool {
		repo, isHires := strings.CutSuffix(name, "-hires")
		return isHires && slices.Contains(names, repo)
	})
}

// isRepoFolder is whether folder is a repo's checkout or worktree: it has a .git of its own.
func isRepoFolder(folder string) bool {
	_, err := os.Lstat(filepath.Join(folder, ".git"))
	return err == nil
}

// pickedProject is the project picked in folder, the folder for hires, as a path, or why it can't be hired into: one
// of the projects hireProjectsIn lists there now, by its name there alone. A name that could break a line is refused
// in words of its own, which don't repeat it (FXPICKEDPROJECTR1, B1).
func pickedProject(folder, name string) (string, string) {
	if strings.ContainsFunc(name, breaksWords) {
		return "", "That project's name has a line break or another control character in it, so a hire can't be told it. " +
			"Rename its folder in your projects folder, " + tildePath(folder) + ", then hire again."
	}
	if !slices.Contains(hireProjectsIn(folder), name) {
		return "", "There's no project called " + name + " in your projects folder, " + tildePath(folder) + ", now. Hire again to pick another."
	}
	return filepath.Join(folder, filepath.FromSlash(name)), ""
}

// projectFirst is, for the office's hire into the folder for hires, folder, the project picked (project, "" for none
// yet): the folder to hire into, or the answer to give instead: the picker, or why the pick can't be hired into. Its
// callers say when it's asked: a role's Claude hire through the engine, the GM aside (its workspace is its place), on
// a Mac with the sandbox on (hireWith); a plain hire whose launcher starts it in its sandbox (FXHIREPROJECTSR5, N1).
// With the lock held, and d.hiring set: the lock is let go while the projects are looked through.
func (d *Deck) projectFirst(folder, project string) (string, int, string) {
	d.mu.Unlock()
	defer d.mu.Lock()
	if project == "" {
		return "", http.StatusOK, pickFor(folder)
	}
	picked, why := pickedProject(folder, project)
	if why != "" {
		return "", http.StatusConflict, why
	}
	return picked, 0, ""
}
