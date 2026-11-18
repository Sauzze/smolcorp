package desktop

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Setup's Sandbox row (SA-X14, SANDBOX-PLAN §A5.2, §A5.3, AQ12, AQ17, AQ21), on Hires and roles, and its line in
// Start here. On a Mac: the switch (SA-X13's), each desk without its sandbox named with why and, where the button
// can, "Restart in the sandbox"; desks to restart so a change applies; Remote Login as the last desk start found it;
// and the opt-in "My local database asks for a password" (SA-M12's check). On Linux and Windows, plainly: no sandbox
// on this computer yet, and what Smolcorp does there instead. Smolcorp never stops or restarts a desk by itself.

// The row's refusals and words.
const (
	noSandboxToRestart  = "Desks on this computer don't start in a sandbox yet, so there's none to restart them in."
	sandboxOffToRestart = "The sandbox is off, so the desk would start without it again. Turn the sandbox on first."
	// gmSandboxOffLine ends the GM's introduction while the owner has the sandbox off (§A5.2, AQ1).
	gmSandboxOffLine = "Smolcorp's sandbox is off: the owner turned it off in Setup, so you and the desks started now " +
		"run without it, each tagged No sandbox. Only the owner can turn it back on."
	// noSandboxHere is a system's whose desks start in no sandbox (Linux and Windows in 1.26): what B still does there,
	// and nothing that says or hints that a sandbox holds them.
	noSandboxHere = "Desks here run as you, with no wall around them: a desk, or a program it starts, can read and " +
		"change what you can. What Smolcorp does here instead: the fence and the guard refuse the commands you've barred " +
		"and keep Claude Code out of your .env files and keys, and a desk can't loosen them while Smolcorp runs. Setup's " +
		"key stays in Smolcorp's memory, and the setup link works once. Each desk has keys of its own, and Smolcorp " +
		"keeps only their fingerprints. The General Manager's token is only in its own desk's environment, never in a " +
		"file. See Keys, records and the guard in Smolcorp's docs."
)

// restartRefusal is why desk h can't be restarted in its sandbox from Setup, or "" with the role and the account's
// config folder ("" for ~/.claude's) to start it with, from roles and the saved accounts. toApply: h runs in its
// sandbox, but started before a change it needs a restart for (restartToApply). Each refusal says what to do instead.
func restartRefusal(h Hire, roles []Role, saved []claudeProfile, sandboxOff, toApply bool) (Role, string, string) {
	name := calledHire(h)
	fireAndHire := " To have it in its sandbox, fire it and hire it again."
	refuse := func(why string) (Role, string, string) { return Role{}, "", why }
	switch {
	case !desksSandboxedHere:
		return refuse(noSandboxToRestart)
	case sandboxOff:
		return refuse(sandboxOffToRestart)
	case inSandbox(h) && !toApply:
		return refuse(name + " runs in its sandbox already.")
	case h.T3Session != "":
		return refuse("Hand " + name + " back from T3 Code: it comes back in its sandbox.")
	case h.Engine == t3Engine:
		return refuse(name + " runs in T3 Code, which has no sandbox." + fireAndHire)
	case h.Role == gmRole:
		return refuse("Your General Manager isn't restarted here, as its token would stop working. Fire it and hire it " +
			"again on Easy setup, and it starts in its sandbox.")
	case !onOwnTmux(h):
		return refuse(name + " wasn't started in a tmux of Smolcorp's own, so it can't be restarted here." + fireAndHire)
	case h.Account == apiKeyAccountName:
		return refuse(name + " works on the API key, which Smolcorp can't hand it again here." + fireAndHire)
	}
	role := Role{ID: h.Role, Name: h.Name, Agent: string(Claude)}
	if i := slices.IndexFunc(roles, func(r Role) bool { return r.ID == h.Role }); i >= 0 {
		role = roles[i]
	}
	if agentOrClaude(role.Agent) != Claude {
		return refuse(name + " isn't a Claude Code desk, and only those start in the sandbox." + fireAndHire)
	}
	role.Model = cmpOr(h.Model, role.Model)
	if h.Account == "" || h.Account == claudeDefaultName {
		return role, "", ""
	}
	account, ok := claudeAccountNamed(h.Account, claudeAccountList(saved))
	if !ok {
		return refuse(name + "'s Claude account, " + h.Account + ", isn't in Setup's list any more." + fireAndHire)
	}
	if account.Name == claudeDefaultName {
		return role, "", ""
	}
	return role, account.Dir, ""
}

// oldGitPlants are the old git plants each desk's start found in its repo's shared .git (X12, findOldGitPlants), by
// desk id, with that .git: Setup's row shows them with a "Remove" button (§A5.3's NB2; RT126SAX14 B2). Git runs each
// at the owner's own commands, outside any sandbox, and a desk that ran without its sandbox could have left one.
// Smolcorp removes nothing by itself. Gone when the core is; each desk start looks again.
var oldGitPlants = struct {
	sync.Mutex
	of map[string]gitPlantsFound
}{of: map[string]gitPlantsFound{}}

type gitPlantsFound struct {
	gitDir string
	plants []oldGitPlant
}

func keepOldGitPlants(id, gitDir string, plants []oldGitPlant) {
	oldGitPlants.Lock()
	oldGitPlants.of[id] = gitPlantsFound{gitDir: gitDir, plants: plants}
	oldGitPlants.Unlock()
}

// listedGitPlants is every old git plant kept, once (desks of one repo find the same ones), in a steady order.
func listedGitPlants() []oldGitPlant {
	oldGitPlants.Lock()
	defer oldGitPlants.Unlock()
	all := []oldGitPlant{}
	for _, found := range oldGitPlants.of {
		for _, p := range found.plants {
			if !slices.ContainsFunc(all, func(q oldGitPlant) bool { return q.ID == p.ID }) {
				all = append(all, p)
			}
		}
	}
	slices.SortFunc(all, func(a, b oldGitPlant) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Key, b.Key), cmp.Compare(a.Value, b.Value), cmp.Compare(a.ID, b.ID))
	})
	return all
}

// removeGitPlant is the owner's "Remove" for listed plant id: removeOldGitPlant takes out only what was listed, and
// only while it is as it was. keep is their "Keep" instead, for one of git's substitutions: it's kept in each .git it
// was found in (keepGitSubst). Then each of those .git is looked through again, so the row shows what is there now,
// whether it was removed, kept or had changed.
func removeGitPlant(id string, keep bool) error {
	var err error
	if !keep {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		err = removeOldGitPlant(ctx, listedGitPlants(), id)
		cancel()
	}
	again := map[string][]string{} // a .git → the desks whose start found the plant in it
	oldGitPlants.Lock()
	for desk, found := range oldGitPlants.of {
		if slices.ContainsFunc(found.plants, func(p oldGitPlant) bool { return p.ID == id }) {
			again[found.gitDir] = append(again[found.gitDir], desk)
		}
	}
	oldGitPlants.Unlock()
	for gitDir, desks := range again {
		if keep {
			keepGitSubst(gitDir, id)
		}
		plants, _ := gitPlantsAt(gitDir)
		for _, desk := range desks {
			keepOldGitPlants(desk, gitDir, plants)
		}
	}
	return err
}

// gitPlantGone is a Remove for a plant no longer as it was listed: nothing was removed.
const gitPlantGone = "That isn't there as it was listed, so nothing was removed. The list now shows what's there."

// gitSubstNotKept is a Keep for a plant that isn't one of git's substitutions: those are only removed.
const gitSubstNotKept = "Only what changes what your git reads can be kept here. Remove this one, or look at it yourself."

// gitSubstFollowsNotKept is a Keep for a replace ref that follows another ref (gitSubstKeepable): what it substitutes
// can change at any time, so it's only removed.
const gitSubstFollowsNotKept = "A replace ref that follows another ref can't be kept: what your git reads through it can change at any time. Remove it, or look at it yourself."

// sandboxGitPlantRoute is POST /setup/sandbox/gitplant?id=[&keep=1], the page's alone: "Remove" for one old git
// plant, or "Keep" for one of git's substitutions (removeGitPlant), answered as GET /setup/sandbox. Its file is said
// to the page, never logged.
func (rt *router) sandboxGitPlantRoute(w http.ResponseWriter, r *request) {
	id, keep, listed := r.URL.Query().Get("id"), r.URL.Query().Get("keep") == "1", listedGitPlants()
	i := slices.IndexFunc(listed, func(p oldGitPlant) bool { return p.ID == id })
	if id == "" || i < 0 {
		writeStatus(w)(http.StatusNotFound, gitPlantGone)
		return
	}
	if keep && !listed[i].Held {
		writeStatus(w)(http.StatusConflict, gitSubstNotKept)
		return
	}
	if keep && !gitSubstKeepable(listed[i]) {
		writeStatus(w)(http.StatusConflict, gitSubstFollowsNotKept)
		return
	}
	file := listed[i].File
	switch err := removeGitPlant(id, keep); {
	case errors.Is(err, errGitPlantGone):
		writeStatus(w)(http.StatusConflict, gitPlantGone)
		return
	case err != nil:
		writeStatus(w)(http.StatusConflict, "Smolcorp couldn't remove it, so look at it yourself: "+file)
		return
	}
	writeJSON(w, http.StatusOK, rt.deck.sandboxState())
}

// remoteLoginSeen is whether Remote Login answered at the last desk start's check (SA-M12), and whether any start has
// checked it since Smolcorp started.
func remoteLoginSeen() (on, known bool) {
	checked.Lock()
	defer checked.Unlock()
	return checked.remoteLogin, !checked.remoteLoginAt.IsZero()
}

// deskCheckedOf is what desk id's start found (deskStartChecks): nothing for a desk whose start checked nothing.
func deskCheckedOf(id string) sandboxChecked {
	checked.Lock()
	defer checked.Unlock()
	return checked.desks[id]
}

// toApply is restartToApply's tags, once a desk's start was checked: with none, nothing is dialed. dial false
// dials nothing at all (restartToApplyKnown): Start here's, which isn't the page's alone.
func toApply(optIn, dial bool) map[string]string {
	checked.Lock()
	none := len(checked.desks) == 0
	checked.Unlock()
	switch {
	case none || !desksSandboxedHere:
		return map[string]string{}
	case !dial:
		return restartToApplyKnown(optIn)
	}
	return restartToApply(optIn)
}

// sandboxRow is what GET /setup/sandbox says besides SA-X13's switch and tags: for each desk, whether "Restart in
// the sandbox" is offered (restart) or why not (restartWhy), and a desk's "restart to apply" (apply); how many desks
// run without the sandbox, a GM started by hand among them (§A5.3's count); Remote Login as last found ("on", "off",
// or "" before any desk start checked it); and AQ21's opt-in.
func (d *Deck) sandboxRow(state map[string]any, hires []Hire) {
	d.mu.Lock()
	off, optIn, byHand := d.config.SandboxOff, d.config.LocalDatabaseAsksPassword, d.startedByHand()
	roles, saved := slices.Clone(d.config.Roles), slices.Clone(d.config.ClaudeProfiles)
	d.mu.Unlock()
	apply := toApply(optIn, true)
	desks, _ := state["desks"].([]map[string]any)
	unsandboxed := 0
	if byHand && desksSandboxedHere {
		unsandboxed++
	}
	for _, desk := range desks {
		i := slices.IndexFunc(hires, func(h Hire) bool { return h.ID == desk["id"] })
		if i < 0 {
			continue
		}
		h := hires[i]
		words := apply[h.ID]
		if !inSandbox(h) && desksSandboxedHere {
			unsandboxed++
		} else if words == "" {
			continue
		}
		if words != "" && inSandbox(h) {
			desk["apply"] = words
		}
		if _, _, why := restartRefusal(h, roles, saved, off, words != ""); why == "" {
			desk["restart"] = true
		} else {
			desk["restartWhy"] = why
		}
	}
	login := ""
	if on, known := remoteLoginSeen(); known && desksSandboxedHere {
		login = map[bool]string{true: "on", false: "off"}[on]
	}
	state["unsandboxed"], state["remoteLogin"], state["databaseOptIn"] = unsandboxed, login, optIn
	plants := []gitPlantRow{}
	for _, p := range listedGitPlants() {
		plants = append(plants, gitPlantRow{p, p.Why == whyHook || p.Why == whyRemote || p.Key != "" || p.Held && p.Command != "", gitSubstKeepable(p)})
	}
	state["gitPlants"] = plants
}

// gitPlantRow is an old git plant as the row shows it: Removable when its "Remove" can take it out (a hook, a remote
// file, a setting, one of git's substitutions with a command), not a folder Smolcorp couldn't read or look through,
// which the owner looks at. Keepable when it's one of git's substitutions the owner can keep (gitSubstKeepable).
type gitPlantRow struct {
	oldGitPlant
	Removable bool `json:"removable"`
	Keepable  bool `json:"keepable,omitempty"`
}

// sandboxDatabaseRoute is POST /setup/sandbox/database?on=0|1, the page's alone: AQ21's opt-in (setDatabaseOptIn),
// answered as GET /setup/sandbox. Its refusal names the place and the login that got in: said to the page, never
// logged.
func (rt *router) sandboxDatabaseRoute(w http.ResponseWriter, r *request) {
	if !desksSandboxedHere {
		writeStatus(w)(http.StatusConflict, noSandboxToSwitch)
		return
	}
	if status, why := rt.deck.setDatabaseOptIn(r.URL.Query().Get("on")); status != http.StatusNoContent {
		writeStatus(w)(status, why)
		return
	}
	writeJSON(w, http.StatusOK, rt.deck.sandboxState())
}

// sandboxRestartRoute is POST /setup/sandbox/restart?id=, the page's alone: "Restart in the sandbox" for one desk
// (restartInSandbox). Setup's "Restart all" asks for each desk in turn.
func (rt *router) sandboxRestartRoute(w http.ResponseWriter, r *request) {
	status, answer := rt.deck.restartInSandbox(r.URL.Query().Get("id"))
	if words, ok := answer.(string); ok {
		writeStatus(w)(status, words)
		return
	}
	writeJSON(w, status, answer)
}

// sandboxCheck is Start here's Sandbox line: on a Mac, whether desks start in their sandbox and how many run without
// it (the count only: the names are on Setup's row, the page's alone, while the self-test is a tool's too); on Linux
// and Windows, that there's none, and what Smolcorp does instead.
//
// At the Linux chain's merge (L1APPARMOR1's own `sandbox` line, RT126SAX14): one line per system. On Linux until
// SA-L4 starts desks there, these words, but L1's fix status and its one command whenever its bwrap check fails
// (its fail-closed gate stays); L1's "Hires start in their own sandbox" only once L4 does.
func (d *Deck) sandboxCheck(hires []Hire) check {
	if !desksSandboxedHere {
		return check{ID: "sandbox", Status: checkInfo, Title: "No sandbox on this computer yet", Detail: noSandboxHere}
	}
	d.mu.Lock()
	off, optIn, byHand := d.config.SandboxOff, d.config.LocalDatabaseAsksPassword, d.startedByHand()
	d.mu.Unlock()
	c := check{ID: "sandbox", Status: checkTodo, Title: "Desks run in their sandbox"}
	unsandboxed, apply := 0, len(toApply(optIn, false))
	if byHand {
		unsandboxed++
	}
	for _, h := range hires {
		if isOpen(h) && !inSandbox(h) {
			unsandboxed++
		}
	}
	desks := func(n int) string {
		if n == 1 {
			return "1 desk"
		}
		return strconv.Itoa(n) + " desks"
	}
	switch {
	case off:
		c.Detail = "Off: the desks you start now run without their sandbox, and can read Smolcorp's keys. Turn it on in " +
			"Hires and roles, under Sandbox."
	case unsandboxed > 0:
		c.Detail = "The sandbox doesn't protect Smolcorp's keys while " + desks(unsandboxed) + " run without it. See " +
			"which in Hires and roles, under Sandbox, and restart them there, or let them finish."
	case apply > 0:
		c.Detail = desks(apply) + " started before a change on this Mac, and keep what their sandbox allowed then. " +
			"Restart them in Hires and roles, under Sandbox."
	default:
		c.Status = checkOK
		c.Detail = "Each desk starts in a sandbox of its own: it can't read Smolcorp's keys, or change another desk's work " +
			"or your own checkout."
	}
	// The old git plants, by count too (RT126SAX14 B2): they run outside any sandbox, at your own git commands. Git's
	// substitutions are said apart, never named either: a desk waits for them.
	listed := listedGitPlants()
	held := len(slices.DeleteFunc(slices.Clone(listed), func(p oldGitPlant) bool { return !p.Held }))
	var said []string
	if n := len(listed) - held; n > 0 {
		said = append(said, "A desk's start found "+map[bool]string{true: "1 hook or setting", false: strconv.Itoa(n) + " hooks or settings"}[n == 1]+
			" in a repository's .git that run a program when you use git, outside any sandbox. See them in Hires and "+
			"roles, under Sandbox, and remove what you didn't put there.")
	}
	if held > 0 {
		said = append(said, "A desk didn't start, as a repository's .git holds something that changes what your git reads. "+
			"See it in Hires and roles, under Sandbox, and remove it, or keep it if it's yours.")
	}
	if words := strings.Join(said, " "); words != "" {
		if c.Status == checkOK {
			c.Detail = words
		} else {
			c.Detail += " " + words
		}
		c.Status = checkTodo
	}
	return c
}
