package desktop

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// The mail pump: every few seconds it tries the queued messages that are due, one after another on one
// goroutine (MailRouter.pump ~402-468; the timer is MailDelivery.startMail, MailDelivery.swift ~193).
// Like the Mac's, it is not tied to the manager switch: mail already queued is still delivered.
// A message for a bot becomes a file in its mailbox (mailDeliverBot); one for a hire is typed into
// that hire's own tmux window, never the GM's; one for a hire that is gone fails. Each round also
// looks in the bots' mailboxes for replies. Nothing here answers a question: see mailType.

// mailPumpEvery is how often the pump runs (MailDelivery.pumpEvery).
const mailPumpEvery = 5 * time.Second

// mailPump is the pump with what it needs given, so tests give stand-ins.
type mailPump struct {
	outbox *mailOutbox
	// hires is who is in the registry now.
	hires func() []Hire
	// bot delivers a message for a bot (Deck.mailDeliverBot).
	bot func(mailMessage)
	// typeIn types a hire's message into its window: commit is called just before the first thing that types.
	typeIn func(h Hire, text string, commit func() bool) mailResult
	// look looks in the bots' mailboxes for replies, at the given time.
	look func(time.Time)
	now  func() time.Time
}

// round is one pass of the pump (MailRouter.pump): each due queued message in outbox order, once.
func (p *mailPump) round() {
	open := p.hires()
	for _, m := range p.outbox.due() {
		switch {
		case strings.HasPrefix(m.To, mailBotPrefix):
			p.bot(m)
		default:
			var found *Hire
			for i := range open {
				if open[i].ID == m.To && isOpen(open[i]) {
					found = &open[i]
					break
				}
			}
			if found == nil {
				// A try still out is left to finish (due leaves such a message out already).
				p.outbox.fail(m.ID, m.ToName+" is gone.")
				continue
			}
			p.deliverToHire(m, *found)
		}
	}
	p.look(p.now())
}

// deliverToHire is one try at one message to a hire; the try is counted by commit, just before typing.
func (p *mailPump) deliverToHire(m mailMessage, h Hire) {
	attempt := p.outbox.start(m.ID)
	if attempt == nil {
		return
	}
	var result mailResult
	defer func() {
		// A panic in a stand-in or tmux's output must not leave the message in flight for ever.
		if r := recover(); r != nil {
			result = mailResult{Outcome: mailTrouble, Note: fmt.Sprint("Delivery stopped: ", r)}
		}
		attempt.Finish(result)
	}()
	if h.Role == gmRole {
		// Never typed into; mailType says so too, this keeps the window out of reach altogether.
		_, why := mailGate(mailTarget{GM: true}, "", false)
		result = mailResult{Outcome: mailCannot, Note: why}
		return
	}
	result = p.typeIn(h, attempt.Text, attempt.Commit)
}

// startMailPump runs the pump every mailPumpEvery until stop is called. stop waits for a round that is
// under way, and may be called more than once.
func (d *Deck) startMailPump() (stop func()) {
	p := &mailPump{
		outbox: d.mail,
		hires:  func() []Hire { return readRegistry(registryPath(), false).hires },
		bot:    d.mailDeliverBot,
		typeIn: d.mailTypeIntoHire,
		look:   d.botMailboxLook,
		now:    time.Now,
	}
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	var ended sync.WaitGroup
	ended.Add(1)
	go func() {
		defer ended.Done()
		ticker := time.NewTicker(mailPumpEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			p.round()
		}
	}()
	return func() {
		once.Do(cancel)
		ended.Wait()
	}
}

// mailDeckAsking says whether the app knows, from its hooks, that a session working where h does is at a
// question, read just then (MailDelivery.hireIsAsking). It is also how T730's built-in watches see a question.
func (d *Deck) mailDeckAsking(h Hire) bool {
	type place struct{ folder, project string }
	var asking []place
	d.mu.Lock()
	for _, s := range d.sessions.All() {
		if s.State == Asking {
			asking = append(asking, place{s.Folder, s.Project})
		}
	}
	d.mu.Unlock()
	// Compared outside the lock: it resolves links on disk.
	for _, a := range asking {
		if works(liveSession{Folder: a.folder, Project: a.project}, h) {
			return true
		}
	}
	return false
}
