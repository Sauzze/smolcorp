package desktop

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

// 💬 Talk bubble, pick (1.26, P3): POST /talk/pick?session=&n=&since= answers the prompt a desk began asking at
// since with its choice n, as a tap on a labelled button in the office's own window. A choice is a key, never text:
// AskUserQuestion's options and a permission prompt's Yes are the choice's digit, which Claude Code's menus take and
// answer at once; a permission prompt's No is the agent's Reject, as ✕ presses it.
//
// The guards: the desk still asking that same prompt (else 409, choiceGone), looked at again on the key worker right
// before the key; its choices offered by label (pickable, else 409: the four keys answer it); n one of them (else
// 400); one pick per prompt (a second is 204 and presses nothing); a headless run, a hire in T3 Code and, on a test
// copy, any tab but its own hires' tmux panes refused as the office's keys are (PerformFrom).

const (
	// A permission prompt's choices on the card: the hooks don't say Claude Code's menu, so only these two (D4).
	talkPermissionYes = 1
	talkPermissionNo  = 2
	// Why a prompt whose choices aren't offered by label can't be picked: the four keys answer it.
	talkUseTheKeys = "Its choices can't be picked from here: answer it with the keys."
	talkNoChoice   = "Unknown choice"
)

// errTalkChoiceGone is a desk that stopped asking, or began asking something else, while its pick waited for the key
// worker: nothing is pressed.
var errTalkChoiceGone = errors.New(choiceGone)

// talkChoices is how many choices a pickable prompt has: its question's options, or Yes and No.
func talkChoices(asking *talkAsking) int {
	if asking.Kind == "question" {
		return len(asking.Questions[0].Options)
	}
	return talkPermissionNo
}

// talkStillAsks says whether session id still asks the prompt it began asking at since. Called without the lock.
func (d *Deck) talkStillAsks(id string, since int64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.sessions.Get(id)
	if s == nil {
		return false
	}
	asking := talkAskingOf(s)
	return asking != nil && asking.Since == since
}

// talkPick answers POST /talk/pick for session id.
func (d *Deck) talkPick(id string, query url.Values) (int, string) {
	n, err := strconv.Atoi(query.Get("n"))
	if err != nil {
		return http.StatusBadRequest, talkNoChoice
	}
	since, err := strconv.ParseInt(query.Get("since"), 10, 64)
	if err != nil || since <= 0 {
		return http.StatusBadRequest, "Unknown prompt"
	}
	d.mu.Lock()
	if hire, ok := d.t3HireAtDesk(id); ok {
		d.mu.Unlock()
		return http.StatusConflict, t3NoControls(calledHire(hire))
	}
	s := d.sessions.Get(id)
	if s == nil || s.Tab == "" {
		d.mu.Unlock()
		return http.StatusNotFound, "That agent has gone."
	}
	if s.Headless() {
		d.mu.Unlock()
		return http.StatusConflict, headlessNoControls
	}
	// A test copy does nothing in the owner's terminal tabs, only in its own hires' tmux panes (RWTC N1).
	if why := testCopyKeepsTab(s.Tab); why != "" {
		d.mu.Unlock()
		return http.StatusConflict, why
	}
	asking := talkAskingOf(s)
	if asking == nil || asking.Since != since {
		d.mu.Unlock()
		return http.StatusConflict, choiceGone
	}
	if !asking.Pickable {
		d.mu.Unlock()
		return http.StatusConflict, talkUseTheKeys
	}
	if n < 1 || n > talkChoices(asking) {
		d.mu.Unlock()
		return http.StatusBadRequest, talkNoChoice
	}
	tab, agent := s.Tab, s.Agent
	reject := asking.Kind == "permission" && n == talkPermissionNo
	key := agent.Reject()
	if !reject {
		var ok bool
		if key, ok = agent.Pick(n); !ok {
			d.mu.Unlock()
			return http.StatusConflict, talkUseTheKeys
		}
	}
	// One pick per prompt: a tap that lands twice, or the window's second look, presses nothing more.
	if !d.claimTalkPick(id, since) {
		d.mu.Unlock()
		return http.StatusNoContent, ""
	}
	d.mu.Unlock()

	// Asked something else, or nothing, while this waited for the worker: looked at again right before the key, as
	// the phone's words are, so a digit never lands in the agent's own box and an Escape never stops its turn.
	gone := false
	status, message := d.toAgent(tab, agent, func(t Tab) error {
		if !d.talkStillAsks(id, since) {
			gone = true
			return errTalkChoiceGone
		}
		return t.Press(key)
	})
	if gone || status >= http.StatusBadRequest {
		// A pick that didn't go doesn't count, so the next tap goes.
		d.mu.Lock()
		if d.talkPicks[id] == since {
			delete(d.talkPicks, id)
		}
		d.mu.Unlock()
		if gone {
			return http.StatusConflict, choiceGone
		}
		return status, message
	}
	// Claude Code ends the turn when its prompt is put away, and no hook says so: see StoppedAsking.
	if reject {
		d.mu.Lock()
		if d.sessions.StoppedAsking(id, nowMillis()) {
			d.changed()
		}
		d.mu.Unlock()
	}
	return status, message
}

// claimTalkPick takes the pick of the prompt session id began asking at since, if none was taken: false for a
// second. Prompts no longer asked are forgotten as it goes. With the lock held.
func (d *Deck) claimTalkPick(id string, since int64) bool {
	if d.talkPicks == nil {
		d.talkPicks = map[string]int64{}
	}
	for other, at := range d.talkPicks {
		if s := d.sessions.Get(other); s == nil || talkAskingOf(s) == nil || talkAskingOf(s).Since != at {
			delete(d.talkPicks, other)
		}
	}
	if d.talkPicks[id] == since {
		return false
	}
	d.talkPicks[id] = since
	return true
}
