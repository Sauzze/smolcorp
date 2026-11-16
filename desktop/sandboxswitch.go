package desktop

import (
	"net/http"
	"runtime"
	"strings"
)

// The sandbox's switch, each desk's tag, and words sent from desk to desk (SA-X13, SANDBOX-PLAN §A5.2, §A5.3, AQ1,
// AQ10).
//
// Where desks start in their sandbox (deskSandboxed: a Mac), the engine starts each one there unless the owner
// turned the sandbox off: GET and POST /setup/sandbox, Setup's page alone (routes.go), never the GM, a tool, an MCP
// key, a night job or a hire. A desk's row says whether its last start was in its sandbox (Hire.Sandboxed). One
// started before this version, while the sandbox was off, taken over in T3 Code, or anywhere else isn't, and keeps
// that tag after the sandbox is on again, until it starts again. Smolcorp never stops or restarts a desk itself.
//
// Words go from one desk into another (mail, typed into its window or its T3 Code thread, and the note of a brief
// added to) only when both run in their sandbox, or neither does. A desk in its sandbox can't have its words run
// outside it (AQ10), and a desk without one can't reach into a sandboxed desk's input (§A5.3). The owner types and
// mails into any desk: Setup, the office's window and a paired phone aren't desks.

// inSandbox is whether desk h runs in its sandbox now: started in it, and not taken over in T3 Code since (C6).
func inSandbox(h Hire) bool { return h.Sandboxed && h.Engine != t3Engine && h.T3Session == "" }

// sandboxTag is what desk h is tagged with, in words for the owner: "" for a desk in its sandbox.
func sandboxTag(h Hire) string {
	switch {
	case !desksSandboxedHere:
		// First: on a system whose desks start in none, no desk's tag may hint that the others have one (RT126SAX14 B1).
		return noSandboxOnThisComputer(runtime.GOOS)
	case inSandbox(h):
		return ""
	case h.T3Session != "":
		return "No sandbox: taken over in T3 Code."
	case h.Engine == t3Engine:
		return "No sandbox: it runs in T3 Code."
	}
	return "No sandbox: it started without one, before this version or while the sandbox was off. Restart it with the sandbox on to put it in one."
}

// desksSandboxedHere is deskSandboxed, as the switch and the tags read it: a test plays a system without the
// sandbox with it.
var desksSandboxedHere = deskSandboxed

// noSandboxOnThisComputer is the tag of every desk on a system whose desks start in no sandbox: Linux until SA-L4,
// and Windows, which has none in 1.26 (the owner, 15 Nov). The same plain words on both (SA-X14, SA-W4's wording):
// nothing says or hints that a sandbox holds a desk there.
func noSandboxOnThisComputer(string) string { return "No sandbox on this computer yet." }

// deskEnd is one end of words sent from desk to desk: what it is called, and whether it runs in its sandbox.
// eitherSide is the GM's token while it may be held on both sides of the sandbox at once (gmEnd): words go
// neither way.
type deskEnd struct {
	name       string
	inSandbox  bool
	eitherSide bool
}

func hireEnd(h Hire) deskEnd { return deskEnd{name: calledHire(h), inSandbox: inSandbox(h)} }

// gmEnd is the GM as one end: on the side of the GM the office hired. While a GM started by hand is allowed
// (gmbyhand.go, the setting on under the Mac app), one GM token can be held by that GM, outside any sandbox, after a
// `smolcorp gm login`, or by the hired GM, and their requests can't be told apart: the hired GM's own requests mark
// the token "started by hand" too (noteGM). So with the setting on and the hired GM in its sandbox, the GM counts as
// on either side, and its words go into no desk, nor a desk's into it (AQ10 and §A5.3 both). With no hired GM in
// its sandbox, every holder is outside one. hires is the registry, read with d.mu let go; gmEnd takes it.
func (d *Deck) gmEnd(hires []Hire) deskEnd {
	d.mu.Lock()
	byHand := d.byHandAllowed()
	d.mu.Unlock()
	end := deskEnd{name: "the GM"}
	for _, h := range hires {
		if h.Role == gmRole && isOpen(h) {
			end.inSandbox = inSandbox(h)
			break
		}
	}
	if byHand && end.inSandbox {
		end.inSandbox, end.eitherSide = false, true
	}
	return end
}

// gmOnEitherSide is why the GM's words don't go, while its token may be held on both sides of the sandbox.
const gmOnEitherSide = " while Allow a General Manager started by hand is on: the GM's token may then be held outside the " +
	"sandbox as well as inside it. Only the owner can. With the setting off (Settings, Hires and roles), the GM in its sandbox " +
	"and the desks in theirs write to each other again."

// acrossSandbox is why words from one desk can't go into another, or "": both ends in their sandbox, or neither.
func acrossSandbox(from, to deskEnd) string {
	switch {
	case from.eitherSide:
		return "The GM can't type or send mail into " + to.name + gmOnEitherSide
	case to.eitherSide:
		return "A desk can't type or send mail into the GM" + gmOnEitherSide
	case from.inSandbox == to.inSandbox:
		return ""
	}
	name := to.name
	if rest, ok := strings.CutPrefix(name, "the "); ok {
		name = "The " + rest
	}
	if from.inSandbox {
		return name + " runs without the sandbox, so a desk in its sandbox can't type or send mail into it. Only the owner can."
	}
	return name + " runs in its sandbox, and a desk without one can't type or send mail into it. Only the owner can."
}

// mailSandboxRefusal is a send's refusal by the sandbox (AQ10), or nil: from the GM (who) or a hire (signer) to dest,
// as POST /setup/gm/send and the GM's brief resolve it. The owner's mail, and mail for a bot, which is a file in its
// mailbox and no desk, are never refused here.
func (d *Deck) mailSandboxRefusal(who caller, signer *Hire, dest mailDest) *mailError {
	if who != callerGM && signer == nil || strings.HasPrefix(dest.ID, mailBotPrefix) {
		return nil
	}
	hires := readRegistry(registryPath(), false).hires
	var from deskEnd
	if signer != nil {
		from = hireEnd(*signer)
	} else {
		from = d.gmEnd(hires)
	}
	to := deskEnd{name: dest.Name}
	if dest.GM {
		to = d.gmEnd(hires)
	} else if h, ok := hireIn(hires, dest.ID); ok {
		to = hireEnd(h)
	}
	if why := acrossSandbox(from, to); why != "" {
		return &mailError{http.StatusForbidden, why}
	}
	return nil
}

// mailDeliveryRefusal is why queued message m isn't typed into desk h now, or "": its ends compared again at each
// try, as either may have started again, in its sandbox or out of it, while the message waited. hires is the
// registry the pump read; a sender no longer in it counts as outside a sandbox.
func (d *Deck) mailDeliveryRefusal(m mailMessage, h Hire, hires []Hire) string {
	var from deskEnd
	switch m.From {
	case mailFromGM:
		from = d.gmEnd(hires)
	case mailFromHire:
		from = deskEnd{name: m.FromName}
		if sender, ok := hireIn(hires, m.FromID); ok {
			from = hireEnd(sender)
		}
	default:
		return ""
	}
	return acrossSandbox(from, hireEnd(h))
}

func hireIn(hires []Hire, id string) (Hire, bool) {
	for _, h := range hires {
		if h.ID == id {
			return h, true
		}
	}
	return Hire{}, false
}

// sandboxRoute answers GET and POST /setup/sandbox, the page's alone (AQ1): whether desks start in their sandbox
// (on: never on a system that starts none there), whether this system starts any there (here), whether a GM was
// started by hand (outside any sandbox), and each desk still open with its tag. POST ?on=0 turns the sandbox off
// for the desks started from then on, and ?on=1 on again; desks already running keep theirs, as they are. On a
// system with no sandbox there is nothing to switch: 409.
func (rt *router) sandboxRoute(w http.ResponseWriter, r *request) {
	d := rt.deck
	if r.Method == http.MethodPost {
		on := r.URL.Query().Get("on")
		if on != "0" && on != "1" {
			writeStatus(w)(http.StatusBadRequest, "Say on=0 or on=1")
			return
		}
		if !desksSandboxedHere {
			writeStatus(w)(http.StatusConflict, noSandboxToSwitch)
			return
		}
		d.mu.Lock()
		// A sealed setting (records_heldconfig.go): while the records' key can't be read, a change that wouldn't last.
		if refused := d.config.heldRefusal(func(c *Config) { c.SandboxOff = on == "0" }); refused != "" {
			d.mu.Unlock()
			writeStatus(w)(http.StatusConflict, refused)
			return
		}
		d.config.SandboxOff = on == "0"
		d.config.save()
		d.changed()
		d.mu.Unlock()
	}
	writeJSON(w, http.StatusOK, d.sandboxState())
}

// noSandboxToSwitch is POST /setup/sandbox's answer on a system whose desks start in no sandbox.
const noSandboxToSwitch = "Desks on this computer don't start in a sandbox, so there's no sandbox to turn on or off."

// sandboxState is GET /setup/sandbox's answer, with Setup's row's own (sandboxRow, SA-X14).
func (d *Deck) sandboxState() map[string]any {
	desks := []map[string]any{}
	hires := readRegistry(registryPath(), false).hires
	for _, h := range hires {
		if isOpen(h) {
			desks = append(desks, map[string]any{"id": h.ID, "name": calledHire(h), "sandboxed": inSandbox(h), "tag": sandboxTag(h)})
		}
	}
	d.mu.Lock()
	state := map[string]any{"on": desksSandboxedHere && !d.config.SandboxOff, "here": desksSandboxedHere, "gmByHand": d.startedByHand(), "desks": desks}
	d.mu.Unlock()
	d.sandboxRow(state, hires)
	return state
}
