package desktop

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// A General Manager held to one project (FXX1c). The owner's hire of the GM may name a scope: the project
// it is hired into (`scope` in POST /setup/hire, for the gm role alone; the GM's token never hires the gm
// role). The app keeps it in the GM's registry row, so it outlasts a restart, and holds every request made
// with that GM's token to it, whichever way it came (`smolcorp gm`, `smolcorp mcp`, curl):
//   - a hire only into the project itself or the top of one of its own worktrees (the folder is the top of
//     its checkout, and the checkout's git folder is the project's), a folder that is there, in a terminal
//     of Smolcorp's, on the Claude account the GM was hired on (filled in when left out), never the API key;
//   - send, brief, fire, contacts, nickname, pin, unpin, watch and unwatch only for its own hires, folders
//     and watches; a pin only on the ground floor or a floor above where it has a session or a pin already;
//   - nothing else that changes the office (its switches, roles, layout, windows…); what it reads is as before.
// A demo's GM (its scope is a demo's project, <demo folder>/app) is held closer, whatever its fence reads (WIN125c):
//   - it hires only the roles the demo offers, as the demo registered them and its registry row keeps them;
//   - a worktree of the project it hires into, briefs or reaches has its top inside the demo folder, not only the
//     project's git folder: `cd elsewhere && git --git-dir=<project>/.git worktree add wt` makes one outside.
// Each refusal is one sentence. A GM with no scope, every other, is as it always was. `smolcorp demo`
// hires its GM so. Go only: the Mac's Swift engine has no scope (M7).

const gmScopeNeedsEngine = "A General Manager held to one project is hired only through Smolcorp's hire engine in tmux, which keeps its scope."

// checkedGMScope is setup's `scope` for hire h: "" when none is asked, else the folder h is hired into, or
// a status and why not.
func checkedGMScope(given any, h setupHire, home string) (string, int, string) {
	if given == nil {
		return "", 0, ""
	}
	text, isText := given.(string)
	switch {
	case !isText || strings.TrimSpace(text) == "":
		return "", http.StatusBadRequest, "The scope is the project's folder, as text."
	case h.Role != gmRole:
		return "", http.StatusBadRequest, "Only the General Manager is held to a scope."
	case h.T3:
		return "", http.StatusConflict, "A General Manager held to one project works in a terminal of Smolcorp's, not in T3 Code."
	}
	folder, status, why := checkedSetupFolder(text, home)
	if status != 0 {
		return "", status, why
	}
	if folder != h.Folder {
		return "", http.StatusBadRequest, "A General Manager's scope is the folder it's hired into."
	}
	return folder, 0, ""
}

// isDemoProject is whether folder has the shape of a demo's project: app in a folder smolcorp demo names so.
// Judged by its shape alone, so a GM held to one is held as a demo's even when its roles weren't kept.
func isDemoProject(folder string) bool {
	return folder != "" && filepath.Base(folder) == demoRepoName && demoFolderNamed(filepath.Base(filepath.Dir(folder)))
}

// demoScopeRoles is what a GM held to scope may hire when scope is a demo's project: the roles the demo was
// registered with, a comma between. "" for any other project; a status and why when the app wasn't told about
// that demo.
func demoScopeRoles(scope string) (string, int, string) {
	if !isDemoProject(scope) {
		return "", 0, ""
	}
	f, roles, why := registeredDemo(filepath.Dir(scope))
	if why != "" || f.repo != scope || len(roles) == 0 {
		return "", http.StatusConflict, "A General Manager held to a demo's project is hired from that demo's page: run smolcorp demo again."
	}
	return strings.Join(roles, ","), 0, ""
}

// gmScopeNow is the scope of the GM whose token works now, "" for none.
func gmScopeNow() string { return gmNow().Scope }

// gmNow is the GM whose token works now, a zero Hire for none: the open GM hired last, as each GM's hire
// makes a new token and takes the older one's place.
func gmNow() Hire { return gmOf(officeHires()) }

// gmOf is gmNow among hires, a registry as read.
func gmOf(hires []Hire) Hire {
	var newest *Hire
	for i := range hires {
		if gmHired(hires[i]) && (newest == nil || hires[i].Started >= newest.Started) {
			newest = &hires[i]
		}
	}
	if newest == nil {
		return Hire{}
	}
	return *newest
}

// gmAccountWords is the Claude account a GM was hired on, as its refusal says it.
func gmAccountWords(account string) string {
	if account == "" || account == claudeDefaultName {
		return "your default Claude account, the one it was hired on"
	}
	return "the Claude account it was hired on, " + account
}

// gmScope is a scope as a request is held to it: the project's full path, as kept at the GM's hire
// (links already followed), and how git is asked.
type gmScope struct {
	root string
	git  gitRunner
	// A demo's GM: the demo folder, where every worktree it's held to must be, and the roles it may hire
	// (none when its row kept none). "" and nil for any other GM.
	demo  string
	roles []string
}

// gmScopeOf is the scope GM gm is held to: a demo's (isDemoProject) with its demo folder and roles.
func gmScopeOf(gm Hire) gmScope {
	s := gmScope{root: gm.Scope, git: scopeGit}
	if isDemoProject(gm.Scope) {
		s.demo = filepath.Dir(gm.Scope)
		if gm.ScopeRoles != "" {
			s.roles = strings.Split(gm.ScopeRoles, ",")
		}
	}
	return s
}

// holds is whether folder is the project itself or the top of one of its own worktrees, and is there; for a
// demo's GM, a worktree inside the demo folder.
func (s gmScope) holds(folder string) bool { _, ok := s.place(folder); return ok }

// place is folder, links followed, when it is held. Links are followed in folder, never in the project's
// path as kept: a project swapped for a link holds nothing.
func (s gmScope) place(folder string) (string, bool) {
	if folder == "" || !filepath.IsAbs(folder) {
		return "", false
	}
	real, err := filepath.EvalSymlinks(filepath.Clean(folder))
	if err != nil {
		return "", false
	}
	if info, err := os.Stat(real); err != nil || !info.IsDir() {
		return "", false
	}
	if real == s.root {
		return real, true
	}
	if _, err := os.Lstat(filepath.Join(real, ".git")); err != nil || s.demo != "" && !within(s.demo, real) {
		return "", false
	}
	status, out, ok := s.git(real, "rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir")
	lines := strings.Split(out, "\n")
	held := ok && status == 0 && len(lines) == 2 && physical(strings.TrimSpace(lines[0])) == real &&
		physical(strings.TrimSpace(lines[1])) == filepath.Join(s.root, ".git")
	return real, held && s.registered(real)
}

// registered is whether the project's git made a worktree at real, a folder with links followed: its registration
// in the project's .git names it (FXGMSCOPEPIN). A desk can't write those (the Policy's "a worktree's registration"
// rows), so a folder the GM made to pass as a worktree (its .git a file naming the project's) isn't held, and can't
// be swapped for a link once it has passed.
func (s gmScope) registered(real string) bool { _, ok := s.registration(real); return ok }

// registration is the name of the project's registration of a worktree at real. It is read as files, through a root
// that leads no link out of the project's .git: the owner's git has no `worktree list` (ownergit.go), and this needs
// no git at all (FXGMSCOPEPINR1). git keeps the worktree's place with links followed, so it is compared as written,
// never followed: a registered folder swapped for a link leads elsewhere, and isn't real.
func (s gmScope) registration(real string) (string, bool) {
	root, err := os.OpenRoot(filepath.Join(s.root, ".git"))
	if err != nil {
		return "", false
	}
	defer root.Close()
	folder, err := root.Open("worktrees")
	if err != nil {
		return "", false
	}
	entries, err := folder.ReadDir(-1)
	folder.Close()
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		at, ok := gitRegistrationFile(root, entry.Name(), "gitdir")
		if !ok {
			continue
		}
		if !filepath.IsAbs(at) { // worktree.useRelativePaths: from the registration's own folder
			at = filepath.Join(s.root, ".git", "worktrees", entry.Name(), at)
		}
		if at = filepath.Clean(at); filepath.Base(at) == ".git" && filepath.Dir(at) == real {
			return entry.Name(), true
		}
	}
	return "", false
}

// commitOf is the commit the project's worktree at real (a folder with links followed, held) is on, by its
// registration's HEAD, as the project's git names it: what a hire's worktree from it starts from, made from the
// project itself (FXGMSCOPEPINR1). The project is its own HEAD's. "" when it can't be told.
func (s gmScope) commitOf(real string) string {
	head := "HEAD"
	if real != s.root {
		name, ok := s.registration(real)
		if !ok {
			return ""
		}
		root, err := os.OpenRoot(filepath.Join(s.root, ".git"))
		if err != nil {
			return ""
		}
		value, ok := gitRegistrationFile(root, name, "HEAD")
		root.Close()
		ref, isRef := strings.CutPrefix(value, "ref: ")
		switch {
		case !ok:
			return ""
		case isRef && gitBranchFile(ref):
			head = ref
		case !isRef && gitObjectID(value):
			head = value
		default:
			return ""
		}
	}
	status, out, ok := s.git(s.root, "rev-parse", "--verify", "-q", head+"^{commit}")
	if out = strings.TrimSpace(out); !ok || status != 0 || !gitObjectID(out) {
		return ""
	}
	return out
}

// gitRegistrationFile is a small file of the worktree registration name, under root (a repo's common .git), trimmed.
func gitRegistrationFile(root *os.Root, name, file string) (string, bool) {
	f, err := root.Open(path.Join("worktrees", name, file))
	if err != nil {
		return "", false
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	data, err := io.ReadAll(io.LimitReader(f, 4096))
	value := strings.TrimSpace(string(data))
	return value, err == nil && value != "" && !strings.ContainsAny(value, "\n\x00")
}

// gitObjectID is whether id is a whole object name as git writes one: 40 (sha1) or 64 (sha256) lower-case hex digits.
func gitObjectID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	return strings.Trim(id, "0123456789abcdef") == ""
}

// hiresOnly is the scope's refusal of a hire's folder, as its sentence goes on after "This General Manager".
func (s gmScope) hiresOnly() string {
	if s.demo != "" {
		return "hires only into " + s.root + " or the top of one of its worktrees in " + s.demo + ", a folder that is there."
	}
	return "hires only into " + s.root + " or the top of one of its worktrees, a folder that is there."
}

// watchesOnly is a held GM's refusal of a watch: at the gate, and at the route for a file it resolved itself.
func (s gmScope) watchesOnly() string {
	return "watches only its own hires and the files in " + s.root + "."
}

// namesOnly is a held GM's refusal of a nickname: at the gate, and at the route for the folder as checked.
func (s gmScope) namesOnly() string {
	return "names only who works in " + s.root + " or its worktrees."
}

// pinned is whether a desk's pin, as config.json keeps it, is held.
func (s gmScope) pinned(pin string) bool {
	folder, _ := existingFolder(pin)
	return s.holds(folder)
}

// hasFloor is whether a floor above the ground floor (1 to 5) is one the scope has: one where a session of
// the project or its worktrees sits now, or where one of its folders is pinned to a desk.
func (s gmScope) hasFloor(d *Deck, floor int) bool {
	// What's on each floor is read with the lock held; whether it's the project's, which runs git, is not.
	type sitter struct {
		floor   int
		project string
	}
	var sitters []sitter
	d.mu.Lock()
	for _, pin := range d.config.FloorPins {
		sitters = append(sitters, sitter{pin.Floor, pin.Folder})
	}
	_, overflow := d.layout()
	seated, _ := d.seatedBuilding(overflow)
	for at, people := range seated {
		for _, one := range people {
			sitters = append(sitters, sitter{at, one.person.Project})
		}
	}
	d.mu.Unlock()
	for _, one := range sitters {
		if one.floor == floor && s.holds(one.project) {
			return true
		}
	}
	return false
}

// covers is whether path is inside the project or one of its worktrees: the nearest folder holding it is held.
func (s gmScope) covers(path string) bool {
	if path == "" || !filepath.IsAbs(path) {
		return false
	}
	for dir := realPath(path); ; dir = filepath.Dir(dir) {
		if s.holds(dir) {
			return true
		}
		if filepath.Dir(dir) == dir {
			return false
		}
	}
}

// hasHire is whether h is one of the GM's own: a hire, not the GM, working in the project or a worktree of it.
func (s gmScope) hasHire(h Hire) bool {
	return h.Role != gmRole && s.holds(h.Folder.storedOnly()) && (h.Worktree.none() || s.holds(h.Worktree.storedOnly())) // compared only
}

// named is the hire a name stands for, as the routes find it (by id, nickname or name, or the name `gm team` shows:
// shown), and whether it's one of the GM's.
func (s gmScope) named(name string, shown map[string]string) (Hire, bool) {
	h, refused := findHire(name, readRegistry(registryPath(), false).hires, shown)
	return h, refused == nil && s.hasHire(h)
}

// gmScopeHolds is whether a request to method is held to gm's scope: a GM with one, and anything but a read.
func gmScopeHolds(gm Hire, method string) bool { return gm.Scope != "" && method != http.MethodGet }

// gmScopeBody is the most of a request's body gmScopeRefusal reads.
const gmScopeBody = 1 << 20

// gmScopeRefusal is why a request made with the GM's token (raw, its body) goes beyond gm's scope: a status
// and one sentence, or 0 to go on. It decides only; body is what was checked, which the route is given in
// place of what was sent (gates.go writes it back), nil when nothing was checked. A pin's folder is set in query
// as checked, which the route is given too.
func (d *Deck) gmScopeRefusal(gm Hire, method, path string, query url.Values, raw []byte) (status int, why string, body any) {
	root := gm.Scope
	if !gmScopeHolds(gm, method) {
		return 0, "", nil
	}
	s := gmScopeOf(gm)
	where := " in " + root
	asked := map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&asked)
	if asked == nil {
		// A literal null decodes to a nil map: read as an empty request, never a panic.
		asked = map[string]any{}
	}
	body = asked
	refuse := func(text string) (int, string, any) {
		return http.StatusForbidden, "This General Manager " + text, body
	}
	text := func(key string) string { v, _ := asked[key].(string); return v }
	// A name given as key, if it is text and not blank, as the routes trim a name (mailAsked). Only such a name is looked
	// up and written back as the hire it found: a name missing, null, not text or blank is never filled in with a hire
	// nobody named, so the route gives its own answer to it (RT126POLISHBR1 B1).
	givenName := func(key string) (string, bool) {
		v, isText := asked[key].(string)
		return v, isText && mailAsked(v) != ""
	}
	// The names `gm team` shows, as the routes find a hire by them too (Deck.shownNames): worked out once, if asked.
	var names map[string]string
	shown := func() map[string]string {
		if names == nil {
			names = d.shownNames(readRegistry(registryPath(), false).hires)
		}
		return names
	}
	post := method == http.MethodPost
	switch {
	case post && path == "/setup/gm/hire":
		if v := asked["apiKey"]; v != nil && v != false {
			return refuse("never hires on the API key.")
		}
		// Its hires work on the account it was hired on (the one the demo's card showed): named or left out,
		// never another. Left out, the app fills it in, so a GM on another account than the default (a test
		// copy's, which never hires on the default one) hires on its own.
		own := gm.Account
		if own == claudeDefaultName {
			own = ""
		}
		named, isName := asked["account"].(string)
		named = strings.Trim(named, " \t")
		if v := asked["account"]; v != nil && !isName || named != "" && named != own && !(named == claudeDefaultName && own == "") {
			return refuse("hires only on " + gmAccountWords(gm.Account) + ".")
		}
		// The default one is named too, so the route never asks which (with more than one saved, it would).
		if own != "" {
			asked["account"] = own
		} else {
			asked["account"] = claudeDefaultName
		}
		// A demo's GM hires the demo's roles alone, each on Claude Code, so each reads the demo's fence too.
		if role, _ := asked["role"].(string); s.demo != "" && !slices.Contains(s.roles, role) {
			if len(s.roles) == 0 {
				return refuse("hires no one: the demo's roles weren't kept at its hire, so hire it again from the demo's page.")
			}
			return refuse("hires only the demo's roles, " + strings.Join(s.roles, ", ") + ".")
		}
		if t3, known := checkedHireEngine(asked["engine"]); t3 || !known {
			return refuse("hires only in a terminal of Smolcorp's.")
		}
		folder, why := gmHireFolder(asked["folder"])
		real, held := s.place(folder)
		if why != "" || !held {
			return refuse(s.hiresOnly())
		}
		// The route hires into the folder as checked, links followed, never the words it was sent; it resolves it
		// again, and the hire's place once more, and each must still be this folder, held (gates.go hands on s).
		asked["folder"] = real
	case post && path == "/setup/gm/send":
		to, given := givenName("to")
		if !given {
			break // the route answers its own 400 (Send {to, text, id}), as mailSend leaves a blank name out
		}
		if a := mailAsked(to); a == "gm" || a == "the gm" {
			break // the route says mail never goes into the GM's own session
		}
		// Found as the route finds it (mailTarget): the office's names only when no bot has the name.
		bots := d.botsNamed(to)
		var names map[string]string
		if len(bots) == 0 {
			names = shown()
		}
		h, ours := s.named(to, names)
		if !ours {
			return refuse("writes only to its own hires" + where + ".")
		}
		if len(bots) > 0 {
			// The route's own answer, as it gave it before: never a bot, never the hire in its place.
			return http.StatusConflict, mailMoreThanOne, body
		}
		// The route is given the hire as checked, by its id, never the name sent: the office's names can change in
		// between, and a held GM changes them (RT126POLISHB N1), as the folder routes are given the folder as checked.
		asked["to"] = h.ID
	case post && (path == "/setup/gm/brief" || path == "/setup/gm/fire"):
		if h, found := hireByID(text("id")); !found || !s.hasHire(h) {
			return refuse("briefs and fires only its own hires" + where + ".")
		}
	case post && path == "/setup/gm/contacts":
		fromName, fromGiven := givenName("from")
		toName, toGiven := givenName("to")
		_, fromText := asked["from"].(string)
		_, toText := asked["to"].(string)
		if !fromText || !toText {
			break // missing, null or not text: the route answers its own 400 (setContacts) before it finds anyone
		}
		if !fromGiven || !toGiven {
			// Blank: the route would look that up as a name (findHire), a hire nobody named. Its own 400 instead.
			return http.StatusBadRequest, contactsShape, body
		}
		from, fromOurs := s.named(fromName, shown())
		to, toOurs := s.named(toName, shown())
		if !fromOurs || !toOurs {
			return refuse("pairs only its own hires" + where + ".")
		}
		// The route pairs the hires as checked, by their ids, never the names sent (as at send).
		asked["from"], asked["to"] = from.ID, to.ID
	case post && path == "/setup/gm/nickname":
		var given struct {
			Folder *string `json:"folder"`
			Name   *string `json:"name"`
		}
		_ = json.Unmarshal(raw, &given)
		if given.Folder == nil {
			return refuse(s.namesOnly())
		}
		real, held := s.place(strings.TrimSpace(*given.Folder))
		if !held {
			return refuse(s.namesOnly())
		}
		// The route names whoever works in the folder as checked, links followed, never the words it was sent.
		given.Folder = &real
		body = given
	case post && (path == "/setup/pin" || path == "/setup/unpin"):
		// The desk on whichever floor it's named on, as the route reads it (pinDesk).
		floor, index, ok := pinDesk(query)
		d.mu.Lock()
		var pinned *string
		if ok {
			pinned = d.config.pinAt(floor, index)
		}
		d.mu.Unlock()
		folder, _ := existingFolder(query.Get("folder"))
		real, held := s.place(folder)
		if pinned != nil && !s.pinned(*pinned) || path == "/setup/pin" && !held {
			return refuse("pins only" + where + " and its worktrees, never over another desk's pin.")
		}
		// A pin upstairs moves the folder's sessions to that floor: only to a floor the GM already has.
		if ok && floor > 0 && path == "/setup/pin" && !s.hasFloor(d, floor) {
			return refuse("pins only on the ground floor or a floor where its own sessions sit or it has a pin" + where + ".")
		}
		// The route pins the folder as checked, links followed, never the words it was sent (gates.go writes the
		// query back).
		if path == "/setup/pin" {
			query.Set("folder", real)
		}
	case post && path == "/setup/gm/watch":
		// Read as the route reads it (watchers.add): the hire by id, nickname or name; a file by its full path,
		// or a path inside a hire's folder.
		line := func(key string) string { return oneLineLimit(text(key), watchTextMost) }
		// The route is given each hire as checked, by its id, never the name sent (as at send).
		hires := readRegistry(registryPath(), false).hires
		ours := func(key string) (Hire, bool) {
			h, refused := findWatchHire(line(key), hires, shown())
			if refused != nil || !s.hasHire(h) {
				return h, false
			}
			asked[key] = h.ID
			return h, true
		}
		ok := true
		switch strings.ToLower(line("kind")) {
		case "idle", "exit":
			_, ok = ours("target")
		case "file":
			file := expandTilde(line("target"), homeDir)
			if !filepath.IsAbs(file) && line("hire") != "" {
				var h Hire
				h, ok = ours("hire")
				file = filepath.Join(h.storedPlace(), file) // checked by the scope below and by the watch's own route
			}
			// The route resolves the path again, and checks what it found against the scope too (watchers.add): a
			// link on the way can be swapped in between (RT126POLISHBR2 N1).
			ok = ok && s.covers(file)
		}
		if !ok {
			return refuse(s.watchesOnly())
		}
	case (post || method == http.MethodDelete) && path == "/setup/gm/unwatch":
		id := text("id")
		if query.Has("id") {
			id = query.Get("id")
		}
		if w, found := d.watchers.get(id); found && w.By != "gm" {
			return refuse("takes away only its own watches.")
		}
	case post && path == nightProposePath:
		// Read as the route reads it (decodedNight: strictly, each key spelled as nightAsk's), and the route is given
		// what was read, so "Kind" or a second "KIND" can't make a hire of what was checked as a task (RT126N1 B2).
		// A night job's hire only into the project or the top of one of its worktrees, as the folder checked; a task
		// for the GM is the GM's own, held to its scope when it runs. Anything but a task for the GM is held to the
		// folder, so a kind the route would refuse anyway gets no further here.
		var ask nightAsk
		if !decodedNight(bytes.NewReader(raw), &ask) {
			return http.StatusBadRequest, "That isn't a night job's proposal: only its words, each key spelled once and as the page spells it.", nil
		}
		body = ask
		if ask.Kind != nightGM {
			real, held := s.place(ask.Folder)
			if !held {
				return refuse("proposes night jobs only in " + root + " or the top of one of its worktrees, a folder that is there.")
			}
			ask.Folder = real
			body = ask
		}
	case post && path == "/setup/preview":
		// Named as a hire, one of its own: never a desk by its session id, which could be anyone's.
		h, refused := d.hireNamed(text("hire"))
		if text("session") != "" || refused != nil || !s.hasHire(h) {
			return refuse("sets the preview only of its own hires" + where + ", named as \"hire\".")
		}
		// The route is given the hire as checked, by its id, never the name sent (as at send): hireNamed reads the
		// office's names too, which a held GM changes (RT126POLISHBR1 N1). A blank name was refused just above.
		asked["hire"] = h.ID
	case post && path == gmProjectPath:
		return refuse("works only" + where + ": it doesn't start new project folders.")
	case post && (path == "/setup/gm/tile" || path == "/setup/gm/screen"):
		return refuse("works only" + where + ": it doesn't move the office's windows.")
	default:
		return refuse("works only" + where + ": it can't change the office's settings.")
	}
	return 0, "", body
}
