package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The hire's place (Phase 6 T602, part A), as the Mac's HireDesk.prepare: a git
// worktree and branch of its own, the worktree trusted in the hire's Claude
// account, and a range of ports. It gets the place ready, or refuses with
// nothing left behind. Nothing calls it yet: part B (tmux) and part C (wiring)
// follow. Nothing here types into a window or starts an agent.

const (
	firstHirePort, lastHirePort, portsPerHire = 4100, 4999, 4
	worktreeTimeout                           = 30 * time.Second
	gitTimeout                                = 10 * time.Second
)

var (
	errHireTrust = &hireRefusal{Status: 409, Message: "Trust this folder once in Claude Code, then hire again: Smolcorp doesn't know how this account keeps its trusted folders."}
	// A fresh account has no .claude.json until Claude Code has run on it once.
	// Smolcorp never makes one: it only edits what Claude Code wrote, so it refuses.
	errHireNoConfig = &hireRefusal{Status: 409, Message: "Start Claude Code once on this account, so it has a .claude.json, then hire again: Smolcorp only edits that file, it never creates it."}
	errHireNoKey    = &hireRefusal{Status: 409, Message: "Smolcorp couldn't read your Anthropic API key from the secret store. Save it again under Summaries, or hire on a Claude account."}
)

// gitRunner runs `git -C dir args`: its status and output (errors included),
// ok false if it couldn't be run or was stopped for taking too long.
type gitRunner func(dir string, args ...string) (status int, output string, ok bool)

func runGit(dir string, args ...string) (int, string, bool) {
	limit := gitTimeout
	if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
		limit = worktreeTimeout // a checkout of a big repo can take a while
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		if exit, isExit := err.(*exec.ExitError); isExit && ctx.Err() == nil {
			return exit.ExitCode(), strings.TrimSpace(string(out)), true
		}
		return 0, "", false
	}
	return 0, strings.TrimSpace(string(out)), true
}

// hireRequest is what the place is made for.
type hireRequest struct {
	HireID    string
	Name      string // its worktree and branch are named for it: the nickname, else the role
	Folder    string
	Claude    bool   // only Claude Code has trust, an account and an apiKeyHelper
	ConfigDir string // the account's config folder; "" is ~/.claude's
	APIKey    bool   // the API-key account was picked
	Worktree  bool   // a repo gets a worktree for it
}

// hirePlaceOptions are what prepareHire needs from outside, each with a test stand-in.
type hirePlaceOptions struct {
	Worktrees string   // the folder set for worktrees, "" for beside the repo
	Taken     []string // the port ranges of hires still working
	Git       gitRunner
	PortFree  func(int) bool
	Key       func() string // the API key, read from the secret store
}

// hirePlace is where the hire works.
type hirePlace struct {
	Folder   string // the folder the agent starts in
	Branch   string
	Worktree string
	Repo     string // the repo's top folder, when there is one
	Ports    string
	Note     string // something to tell whoever hired
}

const notARepo = "This folder isn't a git repository, so the hire works in the folder itself."

// prepareHire gets the hire's place ready, or refuses with nothing left behind.
func prepareHire(r hireRequest, o hirePlaceOptions) (hirePlace, error) {
	git, free := o.Git, o.PortFree
	if git == nil {
		git = runGit
	}
	if free == nil {
		free = portIsFree
	}
	var repo string
	if status, out, ok := git(r.Folder, "rev-parse", "--show-toplevel"); ok && status == 0 && out != "" {
		repo = physical(out)
	}
	given := physical(r.Folder)
	configDir := r.ConfigDir
	file := claudeJSON(configDir)
	var tail *string
	// Before anything is made: the settings that would change who pays, and a config we can't edit.
	if r.Claude {
		if configDir == "" {
			configDir = filepath.Join(homeDir, ".claude")
		}
		if !r.APIKey {
			if helper := apiKeyHelper(r.Folder, repo, configDir); helper != "" {
				return hirePlace{}, &hireRefusal{Status: 409, Message: helper + " sets an apiKeyHelper, which would bill an API key instead of the Claude account. " +
					"Take it out, or hire on the API-key account."}
			}
		}
		resolved := file
		if link, err := filepath.EvalSymlinks(file); err == nil {
			resolved = link
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			return hirePlace{}, errHireNoConfig
		}
		if _, err := trusting("/", data); err != nil {
			return hirePlace{}, errHireTrust
		}
		// The API-key account: Claude Code asks "use this API key?" (default No) unless the key's tail is approved.
		if r.APIKey {
			var key string
			if o.Key != nil {
				key = o.Key()
			}
			if tail = keyApproval(key); *tail == "" {
				return hirePlace{}, errHireNoKey
			}
			if _, err := approving(*tail, data); err != nil {
				return hirePlace{}, errHireTrust
			}
		}
	}
	ports := allocatePorts(o.Taken, free)
	if ports == "" {
		return hirePlace{}, &hireRefusal{Status: 409, Message: "No free ports for a hire: fire someone, or free ports " +
			strconv.Itoa(firstHirePort) + " to " + strconv.Itoa(lastHirePort) + "."}
	}
	place := hirePlace{Folder: r.Folder, Ports: ports}
	if repo != "" && r.Worktree {
		// Both parts are slugs: the ID is never trusted to be a safe path part.
		name := hireSlug(r.Name) + "-" + firstRunes(hireSlug(r.HireID), 6)
		parent := hiresFolder(repo, o.Worktrees)
		path, branch := filepath.Join(parent, name), "hire/"+name
		_ = os.MkdirAll(parent, 0o755)
		status, out, ok := git(repo, "worktree", "add", "-b", branch, path)
		if !ok || status != 0 {
			// No answer: git was stopped for taking too long, and may have left a half-made worktree.
			if !ok {
				discardHalfMade(repo, parent, path, branch, git)
				out = "git didn't answer"
			}
			return hirePlace{}, &hireRefusal{Status: 409, Message: "Couldn't make the hire's worktree: " + firstRunes(out, 200)}
		}
		place.Repo, place.Branch, place.Worktree = repo, branch, physical(path)
		place.Folder = place.Worktree
		// By this system's separator: on Windows, given is C:\…\repo\sub.
		if rest, found := strings.CutPrefix(given, repo+string(filepath.Separator)); found {
			place.Folder = filepath.Join(place.Worktree, rest)
		}
	} else if repo == "" {
		place.Note = notARepo
	}
	if r.Claude {
		// With symlinks resolved: how Claude Code knows a folder.
		trusted := place.Folder
		if place.Worktree != "" {
			trusted = place.Worktree
		}
		if err := preTrust(physical(trusted), file, tail); err != nil {
			if place.Worktree != "" {
				removeWorkspace(place.Repo, place.Worktree, place.Branch, git)
			}
			if errors.Is(err, claudeJSONRefusal) {
				err = errHireTrust // as the up-front check, and the Mac
			} else if _, refused := err.(*hireRefusal); !refused {
				err = &hireRefusal{Status: 500, Message: err.Error()}
			}
			return hirePlace{}, err
		}
	}
	return place, nil
}

// hiresFolder is where the worktrees of a repo's hires are made: beside the
// repo, or under worktrees when that is set.
func hiresFolder(repo, worktrees string) string {
	if worktrees != "" {
		return filepath.Join(worktrees, filepath.Base(repo))
	}
	return filepath.Join(filepath.Dir(repo), filepath.Base(repo)+"-hires")
}

// discardHalfMade clears what a `worktree add` that was stopped may have left:
// its registration, its folder (new, named for this hire alone) and its branch.
// It never deletes a folder that isn't strictly inside parent.
func discardHalfMade(repo, parent, worktree, branch string, git gitRunner) {
	if !strictlyInside(parent, worktree) {
		return
	}
	git(repo, "worktree", "remove", "--force", worktree)
	os.RemoveAll(worktree)
	git(repo, "worktree", "prune")
	git(repo, "branch", "-D", branch)
}

// strictlyInside is whether path, resolved and without "..", is below parent
// and is not parent itself.
func strictlyInside(parent, path string) bool {
	base := filepath.Base(path)
	if base == "." || base == ".." || base == string(filepath.Separator) || strings.Contains(path, "..") {
		return false
	}
	resolved := filepath.Join(physical(filepath.Dir(path)), base)
	if info, err := os.Lstat(resolved); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	rel, err := filepath.Rel(physical(parent), resolved)
	return err == nil && rel != "." && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// removeWorkspace removes a worktree made for a hire, and its branch, when it
// is clean: nothing the hire did is lost. False when it was left, as it holds
// changes (or git couldn't say).
func removeWorkspace(repo, worktree, branch string, git gitRunner) bool {
	if repo == "" || worktree == "" {
		return false
	}
	if status, out, ok := git(worktree, "status", "--porcelain"); !ok || status != 0 || out != "" {
		return false
	}
	if status, _, ok := git(repo, "worktree", "remove", worktree); !ok || status != 0 {
		return false
	}
	git(repo, "branch", "-d", branch)
	return true
}

// allocatePorts is the first range of portsPerHire ports that no taken range
// touches and that are all free, written "4100-4103"; "" for none.
func allocatePorts(taken []string, free func(int) bool) string {
	type span struct{ from, to int }
	var used []span
	for _, text := range taken {
		from, to, found := strings.Cut(text, "-")
		a, errA := strconv.Atoi(from)
		b, errB := strconv.Atoi(to)
		if found && errA == nil && errB == nil && a <= b && b >= firstHirePort && a <= lastHirePort {
			used = append(used, span{max(a, firstHirePort), min(b, lastHirePort)})
		}
	}
next:
	for start := firstHirePort; start+portsPerHire-1 <= lastHirePort; {
		end := start + portsPerHire - 1
		for _, u := range used {
			if u.from <= end && start <= u.to {
				start = u.to + 1
				continue next
			}
		}
		for port := start; port <= end; port++ {
			if !free(port) {
				start = port + 1
				continue next
			}
		}
		return strconv.Itoa(start) + "-" + strconv.Itoa(end)
	}
	return ""
}

// portIsFree is whether nothing is listening on port on this computer.
func portIsFree(port int) bool {
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// hireSlug is lower case letters, digits and dashes: a folder or branch name's
// part. "hire" for nothing usable.
func hireSlug(text string) string {
	var out strings.Builder
	for _, c := range strings.ToLower(text) {
		if c < 128 && (c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			out.WriteRune(c)
		} else {
			out.WriteByte('-')
		}
	}
	joined := strings.Join(strings.FieldsFunc(out.String(), func(r rune) bool { return r == '-' }), "-")
	if joined == "" {
		return "hire"
	}
	return firstRunes(joined, 30)
}

func firstRunes(text string, n int) string {
	if runes := []rune(text); len(runes) > n {
		return string(runes[:n])
	}
	return text
}

// apiKeyHelper is the first settings file among those of places (folders with a
// .claude/ inside, or an account's own folder) that sets an apiKeyHelper, which
// would bill a key rather than the account; "" if none.
func apiKeyHelper(places ...string) string {
	for _, place := range places {
		if place == "" {
			continue
		}
		for _, name := range []string{".claude/settings.json", ".claude/settings.local.json", "settings.json", "settings.local.json"} {
			file := filepath.Join(place, name)
			data, err := os.ReadFile(file)
			var root map[string]json.RawMessage
			if err == nil && json.Unmarshal(data, &root) == nil {
				if _, has := root["apiKeyHelper"]; has {
					return file
				}
			}
		}
	}
	return ""
}
