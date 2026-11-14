package desktop

import (
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// One gate service (1.26's F-S, piece FS1): who may do what is decided here, by decide, for the GM, an outside
// app's MCP key, a night shift job and a hire, with the gates that already hold them, unchanged:
//   - a route: the route table (routeAllows), the manager's switch (managerRefuses), then the scope of a GM held
//     to one project (gmScopeRefusal);
//   - a shell line: the hire guard, with the hire's rules or the GM's (guard.Profiled, as `smolcorp guard` picks
//     them), refusing every line while the guard file is broken;
//   - a hire the GM route makes: fenced Claude roles only (fenceRefusal).
// The owner's own (the page, a tool with the setup key) passes what the route table lets it; their terminal has
// no guard. An MCP key and a night job are held as the GM is: its routes, its scope, its guard profile, its fence.
// A gate that needs the owner says askOwner; decide then keeps the ask in the grant record (grants.go, FS2), where
// only the owner's tap grants it, and a later identical action by the same actor finds its grant, once. The guard's
// merge and spend groups (FS3, internal/guard/merge.go) hold every actor but the owner's own: a merge onto a base
// branch is refused, and a line only the spend group refuses asks the owner.

// actor is who asks: "gm", "hire" or "hire:<id>", "mcp:<key>", "night:<job>", or the owner's own, "page" and
// "tool". Anyone else is "nobody".
type actor string

const (
	actorGM     actor = "gm"
	actorHire   actor = "hire"
	actorPage   actor = "page"
	actorTool   actor = "tool"
	actorNobody actor = "nobody"
)

// mcpActor is an outside app holding the MCP key named key; nightActor is the night shift's job id.
func mcpActor(key string) actor   { return actor("mcp:" + key) }
func nightActor(job string) actor { return actor("night:" + job) }

// hireActor is the hire with that id: its asks and grants are its own, never another hire's (grants.go).
func hireActor(id string) actor { return actor("hire:" + id) }

// actorOf is a setup caller as an actor (caller.name spells it as the route table does).
func actorOf(who caller) actor { return actor(who.name()) }

// kind is what the actor is, without its key, job or id: an MCP key, a night job or a hire with an empty name is
// nobody.
func (a actor) kind() string {
	if kind, name, named := strings.Cut(string(a), ":"); named {
		if (kind == "mcp" || kind == "night" || kind == "hire") && name != "" {
			return kind
		}
		return string(actorNobody)
	}
	switch a {
	case actorGM, actorHire, actorPage, actorTool:
		return string(a)
	}
	return string(actorNobody)
}

// heldAsGM is whether the actor has the GM's powers and fences: the GM, an MCP key, a night job.
func (a actor) heldAsGM() bool {
	kind := a.kind()
	return kind == "gm" || kind == "mcp" || kind == "night"
}

// routeCaller is the route table's caller the actor is checked as. An MCP key and a night job have the GM's
// column: the GM's powers, never the page's (X1 and N2 may give them a column of their own).
func (a actor) routeCaller() caller {
	switch a.kind() {
	case "page":
		return callerPage
	case "tool":
		return callerTool
	case "hire":
		return callerHire
	case "gm", "mcp", "night":
		return callerGM
	}
	return callerNobody
}

// action is what an actor asks to do: exactly one of a route, a shell line, or a fenced hire.
type action struct {
	route *routeAsk
	shell *shellAsk
	hire  *fenceAsk
}

// routeAsk is a request to a setup route; body is read only when a GM's scope must check it.
type routeAsk struct {
	method, path string
	query        url.Values
	body         io.Reader
}

// shellAsk is a command line, checked with the guard file at rules ("" for the guard's own groups, as Load
// reads it), run in the folder cwd (an absolute path; "" when it isn't known, and the merge group then refuses
// what it can't place).
type shellAsk struct {
	line, rules, cwd string
}

// fenceAsk is a hire the GM route makes: fenceRefusal's words.
type fenceAsk struct {
	program, pinned        string
	tmux, launcher, macApp bool
}

// verdictKind is what decide says: go on, no, or ask the owner first.
type verdictKind int

const (
	verdictAllow verdictKind = iota
	verdictRefuse
	verdictAskOwner
)

// verdict is decide's answer: allow, refuse(why), or askOwner(what). status is the HTTP status a route or a
// hire answers a refusal with (0 for a shell line). body, for a route allowed, is the request's body as it was
// checked, which the route is given in place of what was sent; nil to leave it.
type verdict struct {
	kind   verdictKind
	status int
	why    string
	what   string
	amount string // an amount the owner is asked to allow, as it reads, if any
	body   any
}

func allow() verdict { return verdict{kind: verdictAllow} }

func refuse(status int, why string) verdict {
	return verdict{kind: verdictRefuse, status: status, why: why}
}

// askOwner is a verdict that waits for the owner's tap. Until the owner grants it, it is answered as a no (silence
// is a no): askOwnerAnswer says what waits. Deck.decide keeps the ask (grants.go).
func askOwner(what string) verdict { return askOwnerFor(what, "") }

// askOwnerFor is askOwner for something with an amount: "12 dollars", as the owner is to read it.
func askOwnerFor(what, amount string) verdict {
	why := askOwnerAnswer + what
	if amount != "" {
		why += " (" + amount + ")"
	}
	return verdict{kind: verdictAskOwner, status: http.StatusForbidden, what: what, amount: amount, why: why}
}

const askOwnerAnswer = "This waits for the owner's OK: "

// nightRefusal is what an MCP key or a night job is told at the night shift's routes.
const nightRefusal = "Only the setup page or the GM itself can see or propose night jobs."

func (v verdict) allowed() bool { return v.kind == verdictAllow }

// shellGates are more checks of a shell line, after the guard's, for an actor the guard holds. None: FS3's merge
// and spend groups are the guard's own (decideShell asks the owner for spend).
var shellGates []func(a actor, line string) verdict

// spendAsk is what the owner is asked to allow for a line the spend group refuses.
const spendAsk = "run a command that spends money: "

// decide is whether a may do what, as each gate it wraps decides it today. A route's decision takes the deck's
// lock and reads the registry: call it with the lock let go. A line or a hire needs neither. A gate that says
// askOwner is settled against the grant record: the owner's grant for this actor and this ask lets it through
// once; else the ask waits in the approvals list and the answer is a no. The record has a lock of its own, never
// the deck's, so a hire's gate may be asked with that lock held.
func (d *Deck) decide(a actor, what action) verdict {
	v := d.decideNow(a, what)
	if v.kind == verdictAskOwner {
		return d.grants.settle(a, v)
	}
	return v
}

func (d *Deck) decideNow(a actor, what action) verdict {
	switch {
	case what.route != nil:
		return d.decideRoute(a, *what.route)
	case what.shell != nil:
		return decideShell(a, *what.shell)
	case what.hire != nil:
		return decideHire(a, *what.hire)
	}
	return refuse(http.StatusForbidden, "Smolcorp couldn't tell what was asked.")
}

// decideRoute is server.go's gate for setup's routes, in its order: the table (setup's own page, /setup, is
// the page's and isn't in it), the manager's switch, then the GM's scope.
func (d *Deck) decideRoute(a actor, r routeAsk) verdict {
	if r.path != "/setup" && !routeAllows(a.routeCaller(), r.method, r.path) {
		return refuse(http.StatusForbidden, routeRefusal(r.method, r.path))
	}
	// The night shift's jobs are the owner's, and proposed by the GM itself (answer 9): an MCP key or a night job,
	// held as the GM elsewhere, can't list, propose or change one.
	if isNightRoute(r.path) && (a.kind() == "mcp" || a.kind() == "night") {
		return refuse(http.StatusForbidden, nightRefusal)
	}
	d.mu.Lock()
	managerOn := d.config.ManagerEnabled
	d.mu.Unlock()
	if managerRefuses(r.method, r.path, managerOn) {
		return refuse(http.StatusConflict, managerOffAnswer)
	}
	if !a.heldAsGM() {
		return allow()
	}
	gm := gmNow()
	if !gmScopeHolds(gm, r.method) {
		return allow()
	}
	var raw []byte
	if r.body != nil {
		raw, _ = io.ReadAll(io.LimitReader(r.body, gmScopeBody))
	}
	status, why, body := d.gmScopeRefusal(gm, r.method, r.path, r.query, raw)
	if status != 0 {
		return refuse(status, why)
	}
	v := allow()
	v.body = body
	return v
}

// decideShell is the hire guard's answer to a line, with the profile the actor is held to: the GM's for the GM,
// an MCP key and a night job, a hire's for a hire. The owner's own terminal has no guard.
func decideShell(a actor, s shellAsk) verdict {
	profile := guard.ProfileHire
	switch {
	case a.heldAsGM():
		profile = guard.ProfileGM
	case a.kind() == "hire":
	case a.kind() == "page" || a.kind() == "tool":
		return allow()
	default:
		return refuse(0, "Refused by Smolcorp's hire guard: it couldn't tell who asked.")
	}
	groups, problem := guard.Profiled(s.rules, profile)
	if problem != "" {
		return refuse(0, guard.Refused(s.line, groups, problem))
	}
	groups = guard.At(groups, guard.PlaceOf(s.cwd))
	if r := guard.Check(s.line, groups); r != nil {
		if r.Group != guard.SpendGroup {
			return refuse(0, r.Error())
		}
		// Spend's real gate is the owner's OK (Q10), when nothing else in the line is refused: a grant is for that
		// whole line, so a push beside the spend is refused as it is.
		others := slices.DeleteFunc(slices.Clone(groups), func(g guard.Group) bool { return g.Name == guard.SpendGroup })
		if other := guard.Check(s.line, others); other != nil {
			return refuse(0, other.Error())
		}
		return askOwner(spendAsk + s.line)
	}
	for _, gate := range shellGates {
		if v := gate(a, s.line); !v.allowed() {
			return v
		}
	}
	return allow()
}

// decideHire is the fence on a hire the GM route makes: whoever asks it, a fenced Claude role only.
func decideHire(a actor, h fenceAsk) verdict {
	if a.kind() == "nobody" {
		return refuse(http.StatusForbidden, "Smolcorp couldn't tell who asked for this hire.")
	}
	if why := fenceRefusal(h.program, h.pinned, h.tmux, h.launcher, h.macApp); why != "" {
		return refuse(http.StatusConflict, why)
	}
	return allow()
}
