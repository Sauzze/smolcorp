package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"unicode"
)

// Who may write to whom, and the keys hires sign with. Mirrors
// Sources/AgentDeck/FrontDesk.swift (the send keys and the contacts); the wire
// format of POST /setup/gm/contacts is Tests/fixtures/frontdesk/contacts.json.

const (
	// hireKeyHeader is the header a hire sends its key in (FrontDesk.sendKeyHeader).
	hireKeyHeader = "X-Smolcorp-Hire-Key"
	// gmTokenHeader is the header the GM sends its token in (GMIdentity.header).
	gmTokenHeader = "X-Smolcorp-GM-Token"
	gmRole        = "gm"
)

func isOpen(h Hire) bool { return h.State != "fired" && h.State != "failed" }

// hireForSendKey is the hire a send key was made for, or nil: a key nobody was
// given, or a hire that is fired or failed. Every hire is compared, so how long
// it takes doesn't say where a key was close (FrontDesk.hire(forSendKey:)).
func (f registryFile) hireForSendKey(key string) *Hire {
	if key == "" {
		return nil
	}
	given := hashKey(key) // the SHA-256 in hex that FrontDesk.hash makes
	var match *Hire
	for i := range f.hires {
		kept, ok := f.keys[f.hires[i].ID]
		if ok && constantTimeEqual(kept, given) && isOpen(f.hires[i]) {
			match = &f.hires[i]
		}
	}
	return match
}

// isHireKey says whether a header's value is an open hire's send key.
func isHireKey(path, key string) bool {
	if key == "" {
		return false
	}
	return readRegistry(path, false).hireForSendKey(key) != nil
}

// newSendKey makes the send key of the hire `hireID` (which must be in the
// registry already), keeps its hash and returns the key, for the hire engine
// to put in the start script. A GM-role hire gets none, "" with no error: it
// signs with its token, and with both the CLI would take the GM for a hire
// (Deck.swift sendKey(for:)). A hire asking again gets a new key; the old stops working.
func newSendKey(path, hireID string) (string, error) {
	frontDeskMu.Lock()
	defer frontDeskMu.Unlock()
	file := readRegistry(path, true)
	i := slices.IndexFunc(file.hires, func(h Hire) bool { return h.ID == hireID })
	if i < 0 {
		return "", errors.New("no such hire")
	}
	if file.hires[i].Role == gmRole {
		return "", nil
	}
	key := randomKey()
	if file.keys == nil {
		file.keys = map[string]string{}
	}
	file.keys[hireID] = hashKey(key)
	if err := file.save(path); err != nil {
		return "", errors.New("couldn't save the key")
	}
	return key, nil
}

// mayWrite is whether a hire may write to another: only an exact pair the GM or the page allowed.
func (f registryFile) mayWrite(from, to string) bool {
	return slices.Contains(f.contacts, Contact{From: from, To: to})
}

// findHire is the hire a name means: its id, else its nickname or name (open
// hires first, so a fired hire's name can be used again). Says why not when
// there is none or two (FrontDesk.find).
func findHire(name string, all []Hire) (Hire, *refusal) {
	asked := strings.ToLower(strings.TrimFunc(name, func(r rune) bool { return r == '\t' || unicode.Is(unicode.Zs, r) }))
	var found []Hire
	for _, h := range all {
		if h.ID == name {
			found = append(found, h)
		}
	}
	if len(found) == 0 {
		var named, open []Hire
		for _, h := range all {
			if strings.ToLower(h.Nickname) == asked || strings.ToLower(h.Name) == asked {
				named = append(named, h)
				if isOpen(h) {
					open = append(open, h)
				}
			}
		}
		found = named
		if len(open) > 0 {
			found = open
		}
	}
	if len(found) == 0 {
		return Hire{}, &refusal{http.StatusNotFound, "There's no hire called that. Look at the team for ids and names."}
	}
	if len(found) > 1 {
		return Hire{}, &refusal{http.StatusConflict, "More than one hire is called that. Use the id."}
	}
	return found[0], nil
}

// setContacts is POST /setup/gm/contacts, with {from, to, oneWay?, remove?}:
// from and to are hires' ids, nicknames or names. By default two hires may
// write to each other; with oneWay, only from to to. remove takes the pair
// away (with oneWay, only that direction). Answers the pairs as they are now
// (FrontDesk.setContacts).
func setContacts(path string, body []byte) ([]Contact, *refusal) {
	badShape := &refusal{http.StatusBadRequest, "Send {from, to, oneWay?, remove?}: two hires, by id or name."}
	var given map[string]json.RawMessage
	if json.Unmarshal(body, &given) != nil {
		return nil, badShape
	}
	var from, to string
	if json.Unmarshal(given["from"], &from) != nil || json.Unmarshal(given["to"], &to) != nil || isNull(given["from"]) || isNull(given["to"]) {
		return nil, badShape
	}
	var flags [2]bool
	for i, name := range []string{"oneWay", "remove"} {
		if v, ok := given[name]; ok && (isNull(v) || json.Unmarshal(v, &flags[i]) != nil) {
			return nil, badShape
		}
	}
	oneWay, removing := flags[0], flags[1]
	frontDeskMu.Lock()
	defer frontDeskMu.Unlock()
	file := readRegistry(path, true)
	first, refused := findHire(from, file.hires)
	if refused != nil {
		return nil, refused
	}
	second, refused := findHire(to, file.hires)
	if refused != nil {
		return nil, refused
	}
	if first.ID == second.ID {
		return nil, &refusal{http.StatusBadRequest, "That is one hire. Name two."}
	}
	if first.Role == gmRole || second.Role == gmRole {
		return nil, &refusal{http.StatusBadRequest, "The GM isn't one of these pairs: a hire may always write to the GM, and the GM to any hire."}
	}
	if !removing {
		for _, h := range []Hire{first, second} {
			if !isOpen(h) {
				return nil, &refusal{http.StatusConflict, h.Name + " is already gone."}
			}
		}
	}
	pairs := []Contact{{first.ID, second.ID}}
	if !oneWay {
		pairs = append(pairs, Contact{second.ID, first.ID})
	}
	if removing {
		file.contacts = slices.DeleteFunc(file.contacts, func(c Contact) bool { return slices.Contains(pairs, c) })
	} else {
		for _, pair := range pairs {
			if !slices.Contains(file.contacts, pair) {
				file.contacts = append(file.contacts, pair)
			}
		}
	}
	if file.save(path) != nil {
		return nil, &refusal{http.StatusInternalServerError, "Couldn't save the change."}
	}
	return file.contactsJSON(), nil
}

// The GM's token is kept in memory only, on the Deck: no file, no secret store.
// The hire engine makes one when it hires or replaces the GM (newGMToken), and
// takes it away when the GM goes (clearGMToken). Until then nobody is the GM.

// newGMToken makes a fresh token and makes it the only valid one.
func (d *Deck) newGMToken() string {
	token := randomKey()
	d.mu.Lock()
	d.gmToken = token
	d.mu.Unlock()
	return token
}

// clearGMToken stops the GM's token working; nobody is the GM after it.
func (d *Deck) clearGMToken() {
	d.mu.Lock()
	d.gmToken = ""
	d.mu.Unlock()
}

// clearGMTokenIf stops token working if it is still the GM's: a GM that
// failed or was fired takes its own token away, never a newer GM's.
func (d *Deck) clearGMTokenIf(token string) {
	d.mu.Lock()
	if token != "" && constantTimeEqual(d.gmToken, token) {
		d.gmToken = ""
	}
	d.mu.Unlock()
}

// isGMToken says whether a header's value is the GM's token. It never makes
// one: with no token yet, nobody is the GM. The caller holds d.mu.
func (d *Deck) isGMToken(candidate string) bool {
	return candidate != "" && d.gmToken != "" && constantTimeEqual(candidate, d.gmToken)
}

// The two texts the GM's token gets when it asks to hire the General Manager
// (BriefRoutes.secondGMRefusal and onlyOwnerHiresGMRefusal).
const (
	secondGMRefusal         = "This office already has a General Manager. The owner can replace it from Setup."
	onlyOwnerHiresGMRefusal = "Only the owner hires the General Manager, from Setup."
)

// gmIsHired is whether a GM is in the registry and not fired or failed
// (BriefRoutes.gmIsHired).
func (f registryFile) gmIsHired() bool {
	return slices.ContainsFunc(f.hires, func(h Hire) bool { return h.Role == gmRole && isOpen(h) })
}

// gmHireRefusal is what the GM's token is told when it asks to hire the gm
// role: one GM per office, and only the setup page hires or replaces it, whether
// or not a GM is there. Any other caller or role is not refused here
// (BriefRoutes.gmHireRefusal). The hire engine calls it too, before it hires
// or replaces a General Manager for a request that came in over HTTP.
func gmHireRefusal(role string, who caller, gmHired bool) (status int, text string, refused bool) {
	if who != callerGM || role != gmRole {
		return 0, "", false
	}
	if gmHired {
		return http.StatusConflict, secondGMRefusal, true
	}
	return http.StatusConflict, onlyOwnerHiresGMRefusal, true
}
