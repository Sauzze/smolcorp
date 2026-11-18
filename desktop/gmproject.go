package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// POST /setup/gm/project, {name, clone?}: a new project folder in the GM's projects folder, made by the core as the
// owner (FXMAKEPROJECT). Since FXGMROOT the GM's sandbox reads its projects folder and writes nothing there, so it
// can't make a folder or clone a repo itself; it asks here, as it asks for a hire. Only that:
//   - the name is one plain folder name (gmProjectNameRefusal), made directly inside the projects folder, which is
//     held open from then on (projectsPlace): off Windows nothing is made there through its path again, and never
//     over anything at the name, a link included, which is never followed;
//   - with clone, an https address alone, cloned by `git clone` with the core's safe git (gmProjectClone) into a
//     staging folder of the core's own, in the support folder, where no desk can write (gmProjectStaging), then moved
//     into the projects folder in one step that fails on anything at the name (moveIn). No hooks, no templates, no
//     transport but https, no program from any config, and nothing git reads or runs is in a place a desk can write.
//     A clone that fails leaves nothing in the projects folder;
//   - the office's record says what was made (eventKindGMProject), which the GM reads in its inbox too.
// Nothing is ever renamed or removed here but a folder this request made, and nothing is touched outside the projects
// folder but the clone's own staging folder. The route table lets in the GM and the page (routes.go); an MCP key and a
// night job are refused at the gate (gates.go), a GM held to one project too (gmscope.go), and hiring off turns it off.
//
// A plain new folder is a project of its own: no `git init` is part of "new". An empty repo has no commit to make a
// hire's worktree from, while a plain folder is one the office's picker lists as it is; the GM's first hire there can
// start the repo.

const (
	// gmProjectPath is the route.
	gmProjectPath = "/setup/gm/project"
	// gmProjectNameMost is the longest a project's folder name may be, in characters (all of them plain ASCII).
	gmProjectNameMost = 64
	// gmProjectAddressMost is the longest clone address taken.
	gmProjectAddressMost = 2048
	// eventKindGMProject is the office's line about a project folder made here.
	eventKindGMProject = "gmproject"
	// gmProjectStagingName is the folder in the support folder where clones are made before they're moved in.
	gmProjectStagingName = "project-staging"
	// gmProjectRefusal is what an MCP key or a night job is told here.
	gmProjectRefusal = "Only the setup page or the GM itself can start a new project folder."
)

// gmProjectCloneMost is the longest a clone may take before it is stopped and its folder taken away.
var gmProjectCloneMost = 10 * time.Minute

// gmProjectNameRefusal is why name isn't a folder name the route makes, or "": plain letters, digits, "-", "_" and
// ".", no dot first or last, at most 64 characters, and not a name the office keeps for a project's hires.
func gmProjectNameRefusal(name string) string {
	switch {
	case name == "":
		return "Say the new project's folder name."
	case len(name) > gmProjectNameMost:
		return "A project's folder name is at most 64 characters."
	case strings.IndexFunc(name, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.')
	}) >= 0:
		return "A project's folder name is plain letters, digits, \"-\", \"_\" and \".\" only: no spaces, slashes or other signs."
	case strings.HasPrefix(name, ".") || strings.HasSuffix(name, "."):
		return "A project's folder name can't start or end with a dot."
	case strings.HasSuffix(strings.ToLower(name), "-hires"):
		// <project>-hires is where a project's hires get their worktrees (hiresFolder).
		return "A name ending in \"-hires\" is kept for a project's hires' worktrees: pick another."
	}
	return ""
}

// gmProjectAddressRefusal is why address isn't one the route clones, or "": https:// and a host, nothing else, with
// no name or password in it (it would be kept in the new repo's config) and no query or fragment.
func gmProjectAddressRefusal(address string) string {
	const only = "Smolcorp clones only an https:// address, such as https://github.com/owner/project.git."
	if len(address) > gmProjectAddressMost || strings.IndexFunc(address, func(r rune) bool { return r <= ' ' || r == 0x7f }) >= 0 {
		return only
	}
	u, err := url.Parse(address)
	switch {
	case err != nil || !strings.HasPrefix(strings.ToLower(address), "https://") || u.Scheme != "https" || u.Host == "" || u.Opaque != "":
		return only
	case u.User != nil:
		return "The address has a name or a password in it, which the new repo would keep: give it without them."
	case u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(address, "?#"):
		return only
	}
	return ""
}

// gmProjectsFolderFor is the folder the open GM was hired into, resolved, when it works over a projects folder (a GM
// with a workspace, FXGMROOT: hired into a folder that isn't a repo checkout or worktree), or why there is none.
func gmProjectsFolderFor(gm Hire) (string, string) {
	if gm.ID == "" {
		return "", "There's no General Manager now, so there's no projects folder to start a project in."
	}
	if gm.Repo != "" || gm.Worktree != "" || gm.Folder == "" {
		return "", "The General Manager works in a repository, not over a projects folder, so it can't start a project here."
	}
	resolved, err := filepath.EvalSymlinks(gm.Folder)
	if err != nil {
		return "", "The General Manager's projects folder, " + gm.Folder + ", isn't there."
	}
	if info, err := os.Stat(resolved); err != nil || !info.IsDir() {
		return "", "The General Manager's projects folder, " + gm.Folder + ", isn't a folder."
	}
	return resolved, ""
}

// gmProject answers POST /setup/gm/project.
func (d *Deck) gmProject(w http.ResponseWriter, r *request) {
	reply := writeStatus(w)
	var body map[string]any
	if json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&body) != nil || body == nil {
		reply(http.StatusBadRequest, "Send {\"name\": the new folder's name}, and \"clone\": an https address if it's a clone.")
		return
	}
	for key := range body {
		if key != "name" && key != "clone" {
			reply(http.StatusBadRequest, "A new project takes a name and, for a clone, an address: nothing else.")
			return
		}
	}
	name, isText := body["name"].(string)
	if !isText {
		reply(http.StatusBadRequest, "Say the new project's folder name, as text.")
		return
	}
	if why := gmProjectNameRefusal(name); why != "" {
		reply(http.StatusBadRequest, why)
		return
	}
	address, isText := body["clone"].(string)
	if body["clone"] != nil && !isText {
		reply(http.StatusBadRequest, "The address to clone is text.")
		return
	}
	if body["clone"] != nil {
		if why := gmProjectAddressRefusal(address); why != "" {
			reply(http.StatusBadRequest, why)
			return
		}
	}
	d.mu.Lock()
	hiring := d.config.HiringEnabled
	d.mu.Unlock()
	if !hiring {
		reply(http.StatusForbidden, hiringOff)
		return
	}
	projects, why := gmProjectsFolderFor(gmNow())
	if why != "" {
		reply(http.StatusConflict, why)
		return
	}
	folder := filepath.Join(projects, name)
	place, err := openProjectsPlace(projects)
	if err != nil {
		log.Printf("gm project: %v", err)
		reply(http.StatusConflict, "The General Manager's projects folder, "+projects+", changed while Smolcorp was opening it: try again.")
		return
	}
	defer place.close()
	already := "There's already something called " + name + " in " + projects + ": pick another name."
	// Before a clone's wait, so it isn't for nothing; the make or the move below checks again, in the same step.
	if err := place.lacks(name); err != nil {
		if errors.Is(err, fs.ErrExist) {
			reply(http.StatusConflict, already)
			return
		}
		log.Printf("gm project: %v", err)
		reply(http.StatusInternalServerError, "Smolcorp couldn't look in "+projects+".")
		return
	}
	var made error
	if address == "" {
		made = place.makeFolder(name)
	} else {
		staging, err := gmProjectStaging()
		if err != nil {
			log.Printf("gm project: %v", err)
			reply(http.StatusInternalServerError, "Smolcorp couldn't make a place for the clone in its own folder.")
			return
		}
		defer os.RemoveAll(staging) // the core's own folder: what's left of a clone that failed or wasn't moved in
		clone := filepath.Join(staging, name)
		ctx, cancel := context.WithTimeout(context.Background(), gmProjectCloneMost)
		err = gmProjectClone(ctx, clone, address)
		cancel()
		if err != nil {
			log.Printf("gm project: clone of %s: %v", address, err)
			reply(http.StatusBadGateway, "Git couldn't clone "+address+", so Smolcorp made nothing in "+projects+". "+
				"Check the address, and that it needs no sign-in.")
			return
		}
		made = place.moveIn(clone, name)
		if errors.Is(made, fs.ErrExist) {
			already = "Something called " + name + " appeared in " + projects + " while Git was cloning, so Smolcorp " +
				"left it as it is and kept nothing of the clone: pick another name."
		}
	}
	switch {
	case errors.Is(made, fs.ErrExist):
		reply(http.StatusConflict, already)
		return
	case made != nil && movedAcrossDisks(made):
		reply(http.StatusConflict, "Smolcorp clones in its own folder first, then moves the clone in, and "+projects+
			" is on another disk, so it can't: ask the owner to clone it there.")
		return
	case made != nil:
		log.Printf("gm project: %v", made)
		reply(http.StatusInternalServerError, "Smolcorp couldn't make the folder "+name+" in "+projects+".")
		return
	}
	who := "The General Manager"
	if r.caller == callerPage {
		who = "Setup"
	}
	line := who + " made the project folder " + name
	if address != "" {
		line += ", a clone of " + address
	}
	d.events.append(eventEntry{At: time.Now().UnixMilli(), Kind: eventKindGMProject, Folder: folder, Detail: cutEventDetail(line + ".")})
	answer := map[string]any{"ok": true, "folder": folder}
	if address != "" {
		answer["cloned"] = address
	}
	writeJSON(w, http.StatusOK, answer)
}

// gmProjectStaging makes a fresh, empty folder for one clone in <support>/project-staging. Only the core writes
// there: every desk's profile refuses the support folder whole on a Mac, so while git runs no desk can plant a hook or
// a config in the clone, nor move it or put a link in its way (FXMAKEPROJECTR1 B2). It's on the projects folder's
// disk when the support folder is, which moveIn needs.
func gmProjectStaging() (string, error) {
	root := filepath.Join(supportDir, gmProjectStagingName)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	return os.MkdirTemp(root, "clone-")
}

// gmProjectStagingSkipped is whether folder is a support folder's project-staging folder, which the link check
// passes over: a clone there can be a big tree, and it's there only until it's moved in. No desk can write or link in
// it, and a second name there of a file the check looks at is still counted as a name it didn't meet (X12LINK1).
func gmProjectStagingSkipped(supports []string, folder string) bool {
	return slices.ContainsFunc(supports, func(s string) bool {
		return s != "" && filepath.Clean(folder) == filepath.Join(s, gmProjectStagingName)
	})
}

// gmProjectClone clones address into folder, a name not there yet in a staging folder (gmProjectStaging), with the
// core's safe git: the read's environment (gitReadEnv: no config of the owner's or the system's, nothing to ask, no
// program from outside) with https the one transport allowed, and no templates; every program a config key can name
// is blanked as for a read (gitReadArgs), the hooks folder among them (the null device), and core.askPass too. So
// nothing git reads or runs is in a place a desk can write (FXMAKEPROJECTR1 B1): no hooks folder, no config file but
// the new repo's own (in the staging folder), and its temporary files in the staging folder too (TMPDIR).
// Git's own messages go nowhere: the answer says only that it failed.
func gmProjectClone(ctx context.Context, folder, address string) error {
	env := slices.DeleteFunc(gitReadEnv(os.Environ(), nil), func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		name = strings.ToUpper(name)
		return name == "GIT_ALLOW_PROTOCOL" || name == "GIT_TEMPLATE_DIR" || name == "TMPDIR"
	})
	env = append(env, "GIT_ALLOW_PROTOCOL=https", "TMPDIR="+filepath.Dir(folder))
	args := append(slices.Clone(gitReadArgs), "-c", "core.askPass=", "-c", "protocol.https.allow=always",
		"clone", "--quiet", "--template=", "--", address, folder)
	_, status, err := execGitIn(ctx, env, nil, args...)
	if err == nil && status != 0 {
		err = errors.New("git clone ended with status " + strconv.Itoa(status))
	}
	return err
}
