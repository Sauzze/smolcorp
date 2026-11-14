package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// 💬 Talk bubble, read side (1.26, P1): what the office's own window shows in a desk's bubble. GET /talk is one
// desk's card, GET /talk/asking is every waiting desk's question for the small bubbles. Both are the window's
// alone, on this computer (the router): the card holds what the agent last said and what it asks.
//
// A question is read from what a prompt is about (Session.askedInput, kept in memory, never saved) and cut. A
// permission prompt gives its label only, never the tool's input. Nothing here types, presses or answers.

const (
	// The card's cuts, in characters.
	talkQuestionMost    = 300
	talkLabelMost       = 80
	talkDescriptionMost = 200
	talkLastMost        = 200
	// How many questions, and options in each, a card lists. A menu takes one digit per choice, so a question with
	// more than talkOptionsMost options isn't offered by label.
	talkQuestionsMost = 8
	talkOptionsMost   = 9
	// How many items of the transcript's end are looked through for the agent's last words.
	talkLastLook = 12
	// How many transcripts' last lines are kept: past it, the cache starts over.
	talkLastKept = 200
)

// talkOption is a choice of a question: n is the digit that picks it.
type talkOption struct {
	N           int    `json:"n"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// talkQuestion is one question AskUserQuestion asks, with its options.
type talkQuestion struct {
	Question    string       `json:"question"`
	Header      string       `json:"header"`
	MultiSelect bool         `json:"multiSelect"`
	Options     []talkOption `json:"options"`
}

// talkAsking is what a desk waiting on you is about. Since is when it began asking, in ms: a pick says which
// prompt it means by it. Kind is "question" (AskUserQuestion), "permission" or "unknown" (asking with nothing said:
// a notification, or another agent). Pickable is whether its choices can be offered by label: one question that
// isn't multi-select, or a Bash, Write or Edit permission prompt (Yes and No: n 1 and 2), of an agent whose menus take a choice's
// number (POST /talk/pick), in a tab whose screen a pick can read to see the menu is still there (talkShowsScreen);
// otherwise the window shows the four keys.
type talkAsking struct {
	Since     int64          `json:"since"`
	Kind      string         `json:"kind"`
	Tool      string         `json:"tool"`
	Label     string         `json:"label"`
	Questions []talkQuestion `json:"questions"`
	Pickable  bool           `json:"pickable"`
	// A Bash prompt's command, its first line, or a Write or Edit prompt's file path: looked for in the menu on the
	// screen before a pick's key, never on the card.
	command, file string
}

// talkCard is GET /talk: one desk. Asking is null unless the desk is asking; Last is what the agent last said
// (or the last question it asked), cut; Size is the transcript's size, so the window reads /conversation only
// when it has grown.
type talkCard struct {
	State  SessionState `json:"state"`
	Asking *talkAsking  `json:"asking"`
	Last   string       `json:"last"`
	Size   int64        `json:"size"`
	// Whether the window can type into the desk now (talksay.go), so the bubble greys its input with the reason.
	Typing deskTyping `json:"typing"`
	// Whether the window can send it a picture now (photo.go): its typing, and an agent that takes pictures.
	Photo deskPhoto `json:"photo"`
}

// talkAskingOf is the session's prompt, or nil when it isn't asking. With the deck's lock held.
func talkAskingOf(s *Session) *talkAsking {
	if s.State != Asking {
		return nil
	}
	tool := s.askedTool
	if tool == "" {
		tool = s.WaitingFor
	}
	since := s.askedSince
	if since <= 0 {
		since = s.LastEventAt
	}
	asking := &talkAsking{Since: int64(since), Kind: "unknown", Tool: tool, Label: approvalLabel(tool, s.askedInput),
		Questions: []talkQuestion{}}
	switch {
	case tool == "AskUserQuestion":
		asking.Kind = "question"
		var whole bool
		asking.Questions, whole = talkQuestionsOf(s.askedInput)
		// One question, with its options all listed, that takes one answer.
		asking.Pickable = whole && len(asking.Questions) == 1 && !asking.Questions[0].MultiSelect && len(asking.Questions[0].Options) > 0
	case tool != "":
		// Yes or No: the hooks don't say Claude Code's menu, so nothing more is offered by label. Leaving plan
		// mode's choice 1 also changes the mode (auto-accept edits), more than a Yes says: the keys answer it.
		asking.Kind, asking.Pickable = "permission", tool != "ExitPlanMode"
		if tool == "Bash" {
			var input struct {
				Command string `json:"command"`
			}
			_ = json.Unmarshal([]byte(s.askedInput), &input)
			asking.command, _, _ = strings.Cut(input.Command, "\n")
		}
		if tool == "Write" || tool == "Edit" {
			var input struct {
				FilePath string `json:"file_path"`
			}
			_ = json.Unmarshal([]byte(s.askedInput), &input)
			asking.file = input.FilePath
		}
		// Only a menu a pick can tell from another's (talkMenuOn): a Bash command's, a Write or Edit file's. Any other
		// tool's Yes would only ever be refused, so its card offers the four keys up front (FXP3c).
		if asking.command == "" && asking.file == "" {
			asking.Pickable = false
		}
	}
	// Only where a choice can be pressed (Agent.Pick): another agent's menu is answered with the four keys. And
	// only where the menu can be seen still there right before the key: elsewhere the four keys answer it.
	if _, picks := s.Agent.Pick(1); !picks || !talkShowsScreen(s.Tab) {
		asking.Pickable = false
	}
	return asking
}

// talkQuestionsOf is AskUserQuestion's questions from its input as JSON, cut, as conversationQuestion reads them,
// and whether they are all there: none left out for the card's limits.
func talkQuestionsOf(input string) ([]talkQuestion, bool) {
	var parsed struct {
		Questions []struct {
			Question    string `json:"question"`
			Header      string `json:"header"`
			MultiSelect bool   `json:"multiSelect"`
			Options     []struct {
				Label       string `json:"label"`
				Description string `json:"description"`
			} `json:"options"`
		} `json:"questions"`
	}
	questions := []talkQuestion{}
	// An input that isn't what AskUserQuestion sends gives none: the window then shows the four keys.
	if json.Unmarshal([]byte(input), &parsed) != nil {
		return questions, false
	}
	whole := true
	for _, q := range parsed.Questions {
		if q.Question == "" {
			continue
		}
		if len(questions) >= talkQuestionsMost {
			whole = false
			continue
		}
		card := talkQuestion{Question: clip(q.Question, talkQuestionMost), Header: clip(q.Header, talkLabelMost),
			MultiSelect: q.MultiSelect, Options: []talkOption{}}
		for _, o := range q.Options {
			if o.Label == "" {
				continue
			}
			if len(card.Options) >= talkOptionsMost {
				whole = false
				continue
			}
			card.Options = append(card.Options, talkOption{N: len(card.Options) + 1, Label: clip(o.Label, talkLabelMost),
				Description: clip(o.Description, talkDescriptionMost)})
		}
		questions = append(questions, card)
	}
	return questions, whole
}

// talkLast is what a transcript's last words were as of the file's size and time.
type talkLast struct {
	size int64
	at   time.Time
	text string
}

// talkLasts remembers each transcript's last words, so a hover costs a stat and the file is read again only when
// it has changed. Kept in memory, per path, a bounded number.
var talkLasts = struct {
	sync.Mutex
	byPath map[string]talkLast
}{byPath: map[string]talkLast{}}

// talkLastOf is the transcript's size, and what the agent last said in it (its last reply or question, cut), or
// "" with no file, or none in its last items.
func talkLastOf(path string) (int64, string) {
	if path == "" {
		return 0, ""
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return 0, ""
	}
	talkLasts.Lock()
	known, found := talkLasts.byPath[path]
	talkLasts.Unlock()
	if found && known.size == info.Size() && known.at.Equal(info.ModTime()) {
		return known.size, known.text
	}
	conversationReads.Lock()
	page, ok := readConversation(path, nil, talkLastLook)
	conversationReads.Unlock()
	text := ""
	if ok {
		for i := len(page.Items) - 1; i >= 0; i-- {
			if kind := page.Items[i].Kind; kind == "agent" || kind == "question" {
				text = clip(page.Items[i].Text, talkLastMost)
				break
			}
		}
	}
	// The size the page was read at, which may be more than the stat saw: the window asks again for any more.
	size := info.Size()
	if ok {
		size = page.Size
	}
	talkLasts.Lock()
	if len(talkLasts.byPath) >= talkLastKept {
		talkLasts.byPath = map[string]talkLast{}
	}
	talkLasts.byPath[path] = talkLast{size, info.ModTime(), text}
	talkLasts.Unlock()
	return size, text
}

// The talk bubble's read routes, in the office's own window on this computer, never a phone (phoneroutes.go): one
// desk's card, and every waiting desk's question. Neither changes anything.
func init() {
	registerPhoneRoute(phoneRoute{method: http.MethodGet, path: "/talk", access: accessWindow, refusal: talkRefusal,
		serve: func(d *Deck, w http.ResponseWriter, _ *request, session string) {
			writeAnswer(w)(d.talk(session))
		}})
	registerPhoneRoute(phoneRoute{method: http.MethodGet, path: "/talk/asking", access: accessWindow, refusal: talkRefusal,
		serve: func(d *Deck, w http.ResponseWriter, _ *request, _ string) {
			writeJSON(w, http.StatusOK, d.talkAskingAll())
		}})
}

// talkRefusal is what a phone asking for the talk bubble is told.
func talkRefusal(d *Deck) string { return d.shownOnlyInTheWindow("The talk bubble") }

// talk answers GET /talk: the card of session id, 404 when it has gone. A desk with no transcript still has one:
// the bubble opens, and says there is no conversation.
func (d *Deck) talk(id string) (int, any) {
	d.mu.Lock()
	s := d.sessions.Get(id)
	var card talkCard
	path := ""
	var agent Agent
	if s != nil {
		card = talkCard{State: s.State, Asking: talkAskingOf(s)}
		path, agent = s.Transcript, s.Agent
	}
	d.mu.Unlock()
	if s == nil {
		return http.StatusNotFound, "That agent has gone."
	}
	typing, found := d.windowTyping(id)
	if !found {
		return http.StatusNotFound, "That agent has gone."
	}
	card.Typing = typing
	card.Photo = photoTaking(typing, agent)
	card.Size, card.Last = talkLastOf(path)
	return http.StatusOK, card
}

// talkAskingRow is one waiting desk, for the small bubbles.
type talkAskingRow struct {
	Session string `json:"session"`
	talkAsking
}

// talkAskingAll answers GET /talk/asking: every desk waiting on you, the longest waiting first (those whose time
// isn't known last, as the approvals list has them).
func (d *Deck) talkAskingAll() map[string]any {
	d.mu.Lock()
	rows := []talkAskingRow{}
	for _, s := range d.sessions.All() {
		if asking := talkAskingOf(s); asking != nil {
			rows = append(rows, talkAskingRow{s.ID, *asking})
		}
	}
	d.mu.Unlock()
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if (a.Since > 0) != (b.Since > 0) {
			return a.Since > 0
		}
		if a.Since != b.Since {
			return a.Since < b.Since
		}
		return a.Session < b.Session
	})
	return map[string]any{"asking": rows}
}
