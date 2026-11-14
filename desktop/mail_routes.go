package desktop

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

// The mail routes: POST /setup/gm/send and GET /setup/gm/outbox, ported from the
// Mac's MailRouter.swift (~161-311, ~508-528) and DeckRouter.swift (~1128-1137).
// They find who is asking and where the message goes, and hand the rest to the
// outbox (mail_outbox.go). Nothing is typed here: a new queued message to a hire
// stays queued until the delivery (T728) takes it. A message to a bot is dropped
// in the bot's mailbox folder at once (botmailbox.go).

// The refusals a hire and the GM or the owner get (MailRouter.hireRule, noKey, notTheGM).
const (
	mailHireRule = "A hire may write to the GM, and to another hire only when the GM or the owner has allowed that pair. " +
		"Ask the GM to allow it (smolcorp gm contacts)."
	mailMoreThanOne = "More than one hire or bot is called that. Use the id (bot:<id> for a bot)."
	mailBotPrefix   = "bot:"
)

// mailLimits reads the limits from a copy of the config taken under the Deck's lock,
// so the outbox never has to hold its own lock to read them.
func (d *Deck) mailLimits() mailLimits {
	d.mu.Lock()
	defer d.mu.Unlock()
	return mailLimitsOf(d.config)
}

// newDeckOutbox opens the Deck's one outbox, in the support folder.
func (d *Deck) newDeckOutbox() *mailOutbox {
	return newMailOutbox(filepath.Join(supportDir, "outbox.json"), d.mailLimits, nil)
}

// mailAsked is a name as asked: without the spaces around it (not line breaks), lowercase
// (the Mac's trimmingCharacters(in: .whitespaces)).
func mailAsked(name string) string {
	return strings.ToLower(strings.TrimFunc(name, func(r rune) bool { return r == '\t' || unicode.Is(unicode.Zs, r) }))
}

func mailHireName(h Hire) string {
	if h.Nickname == "" {
		return h.Name
	}
	return h.Nickname
}

// mailSenderOf is who a caller is to the outbox. Only the GM, the page and a hire reach the route.
func mailSenderOf(who caller) mailSender {
	switch who {
	case callerGM:
		return mailFromGM
	case callerHire:
		return mailFromHire
	}
	return mailFromOwner
}

// mailBody is what POST /setup/gm/send was sent. Anything that isn't {to, text} as strings
// leaves them blank (a 400); an id that is there and isn't a string can't be used (a 400).
func mailBody(raw []byte) (to, text, id string, idGiven bool) {
	var given map[string]json.RawMessage
	if json.Unmarshal(raw, &given) != nil {
		return
	}
	var t, x string
	if json.Unmarshal(given["to"], &t) == nil {
		to = t
	}
	if json.Unmarshal(given["text"], &x) == nil {
		text = x
	}
	if v, ok := given["id"]; ok {
		idGiven = true
		var s string
		if json.Unmarshal(v, &s) == nil {
			id = s
		}
	}
	return
}

// mailSend is POST /setup/gm/send (DeckRouter.swift ~1128-1131, MailRouter.send).
func (rt *router) mailSend(w http.ResponseWriter, r *request) {
	d := rt.deck
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeStatus(w)(http.StatusRequestEntityTooLarge, "That request is too big.")
		return
	}
	req := mailRequest{From: mailSenderOf(r.caller)}
	var signer *Hire
	if r.caller == callerHire {
		// A hire is whoever its send key was made for, found again here, never by anything it says.
		if h := readRegistry(registryPath(), false).hireForSendKey(r.Header.Get(hireKeyHeader)); h != nil {
			signer = h
			req.SignerID, req.SignerName = h.ID, mailHireName(*h)
		}
	}
	to, text, id, idGiven := mailBody(raw)
	req.Text, req.ID, req.IDGiven = text, id, idGiven
	if mailAsked(to) != "" {
		req.To = to
	}
	req.Resolve = func() (mailDest, *mailError) {
		if signer != nil {
			return d.mailTowardFrom(*signer, to)
		}
		return d.mailTarget(to)
	}
	added, refusal := d.mail.add(req)
	if refusal != nil {
		writeStatus(w)(refusal.Status, refusal.Message)
		return
	}
	d.owner.notOwners(req.Text) // mail is never the owner's words, even typed into the GM's terminal (ownerwords.go)
	if added.ToGM && !added.Duplicate && signer != nil {
		// Kept first, then told: the GM's inbox learns of it as it does of a bot's reply.
		d.events.append(mailInboxEntry(*signer, added.Message))
	}
	answer := added.Answer
	if added.Deliver && strings.HasPrefix(added.Message.To, mailBotPrefix) {
		// A bot has no window: the message becomes a file in its mailbox now, and the answer says how that went.
		d.mailDeliverBot(added.Message)
		if m, ok := d.mail.get(added.Message.ID); ok {
			answer = mailAnswer(m, false)
		}
	}
	writeJSON(w, http.StatusOK, answer)
}

// mailTowardFrom is where a message from signer may go: the GM, or a hire if the pair is allowed
// (MailRouter.toward, ~244-256). An unknown name and a name not allowed get the same refusal,
// so a hire can't use this to find out who is on the team. A hire never writes to a bot.
func (d *Deck) mailTowardFrom(signer Hire, name string) (mailDest, *mailError) {
	refusal := &mailError{http.StatusForbidden, mailHireRule}
	if asked := mailAsked(name); asked == "gm" || asked == "the gm" {
		return mailDest{GM: true}, nil
	}
	file := readRegistry(registryPath(), false)
	found, why := findHire(name, file.hires)
	if why != nil {
		return mailDest{}, refusal
	}
	if found.Role == gmRole {
		return mailDest{GM: true}, nil
	}
	if found.ID == signer.ID || !isOpen(found) || !file.mayWrite(signer.ID, found.ID) {
		return mailDest{}, refusal
	}
	return mailDest{ID: found.ID, Name: mailHireName(found)}, nil
}

// mailTarget is where a message from the GM or the owner goes: a hire by id, nickname or name,
// or a bot by id, name or bot:<id> (MailRouter.resolve, ~284-310).
func (d *Deck) mailTarget(name string) (mailDest, *mailError) {
	asked := mailAsked(name)
	if asked == "gm" || asked == "the gm" {
		return mailDest{}, &mailError{http.StatusForbidden, "Mail is never put into the GM's own session. The GM reads its mail with smolcorp gm inbox."}
	}
	prefixed := strings.HasPrefix(asked, mailBotPrefix)
	bare := asked
	if prefixed {
		bare = strings.TrimPrefix(asked, mailBotPrefix)
	}
	d.mu.Lock()
	bots := append([]CloudBot(nil), d.cloud.bots...)
	d.mu.Unlock()
	var named []CloudBot
	for _, b := range bots {
		if b.ID == bare || (!prefixed && strings.ToLower(b.Name) == asked) {
			named = append(named, b)
		}
	}
	found, why := findHire(name, readRegistry(registryPath(), false).hires)
	if why != nil {
		if why.status == http.StatusNotFound {
			switch len(named) {
			case 1:
				return d.mailBotDest(named[0])
			case 0:
				return mailDest{}, &mailError{http.StatusNotFound, "There's no hire or bot called that. Look at the team for ids and names."}
			}
			return mailDest{}, &mailError{http.StatusConflict, mailMoreThanOne}
		}
		return mailDest{}, &mailError{why.status, why.message}
	}
	if len(named) > 0 {
		return mailDest{}, &mailError{http.StatusConflict, mailMoreThanOne}
	}
	if found.Role == gmRole {
		return mailDest{}, &mailError{http.StatusForbidden, "Mail is never put into the GM's own session. The GM reads its mail with smolcorp gm inbox."}
	}
	if !isOpen(found) {
		return mailDest{}, &mailError{http.StatusConflict, found.Name + " is already gone."}
	}
	return mailDest{ID: found.ID, Name: mailHireName(found)}, nil
}

// mailBotDest is a bot as a target: it needs a mailbox folder that is not barred. One that is set but
// not usable is told now, not left to fail; one that is only not synced yet is not (MailRouter.send, ~211-214).
func (d *Deck) mailBotDest(b CloudBot) (mailDest, *mailError) {
	if b.Mailbox == "" {
		return mailDest{}, &mailError{http.StatusConflict, b.Name + " has no mailbox folder. The owner sets one on the setup page."}
	}
	if problem := botMailboxRefusal(b.Mailbox, d.mailboxBarred()); problem != nil && problem.Never {
		return mailDest{}, &mailError{http.StatusConflict, problem.Why}
	}
	return mailDest{ID: mailBotPrefix + b.ID, Name: b.Name}, nil
}

// mailOutboxList is GET /setup/gm/outbox?limit= (DeckRouter.swift ~1136). The texts go to the GM and the
// page only; a tool is not given them. A limit that isn't a whole number is none.
func (rt *router) mailOutboxList(w http.ResponseWriter, r *request) {
	var asked *int
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
		asked = &n
	}
	writeJSON(w, http.StatusOK, rt.deck.mail.list(asked, r.caller != callerTool))
}

// mailInboxEntry is the GM's inbox line for a hire's message to the GM: who wrote it (its id and
// name, never its key) and what (MailRouter.inboxEntry, ~264-270). It is the same line the Mac
// makes, so the text is in it: the inbox is how the GM reads the message.
func mailInboxEntry(h Hire, m mailMessage) eventEntry {
	clean := func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.Is(unicode.Cc, r) {
				return ' '
			}
			return r
		}, s)
	}
	name := m.FromName
	if name == "" {
		name = mailHireName(h)
	}
	return eventEntry{At: int64(m.At), Folder: h.Folder, Kind: eventKindMail,
		Detail: clean(name) + " (hire " + h.ID + ") wrote to you: " + clean(m.Text)}
}
