package desktop

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Who is waiting for you, and on what: every session asking, as the Mac's
// Approvals.swift lists it (wire format: Tests/fixtures/frontdesk/approvals.json).
// The list only shows. A prompt is answered by your tap alone, on a paired phone
// or the deck, with /act and /choose; nothing here, and no /setup/gm route,
// answers, accepts or dismisses one. Like the event log it is thin: the tool and
// a short label (for Bash the command's first word, for a file tool the file's
// name), never a whole command, a path or what a question says.

const approvalLabelLimit = 80

// approvalItem is one session asking, as the deck knows it.
type approvalItem struct {
	session, hire, name, nickname, agent string
	floor, desk                          *int
	kind, tool, input                    string
	since                                float64 // ms, 0 if unknown
	notified                             bool
}

// approvalLabel is the tool and what it works on in a few words: "Bash: git",
// "Edit: Deck.swift", or just the tool.
func approvalLabel(tool, input string) string {
	var object map[string]any
	_ = json.Unmarshal([]byte(input), &object)
	str := func(v any) string { s, _ := v.(string); return s }
	what := ""
	if tool == "Bash" {
		what = firstWordOf(str(object["command"]))
	} else {
		for _, field := range []string{"file_path", "notebook_path"} {
			if s := str(object[field]); s != "" {
				what = hidePhotos(filepath.Base(s))
				break
			}
		}
	}
	line := tool + ": " + what
	switch {
	case what == "":
		line = tool
	case tool == "":
		line = what
	}
	line = strings.Join(strings.FieldsFunc(line, func(r rune) bool { return r == '\n' || r == '\r' }), " ")
	if utf8.RuneCountInString(line) > approvalLabelLimit {
		return string([]rune(line)[:approvalLabelLimit-1]) + "…"
	}
	return line
}

// GET /approvals (phoneroutes.go): who waits for you, read only, to a paired phone over Tailscale, never over the
// Wi-Fi nor to the office's window. Answering is /act and /choose.
func init() {
	registerPhoneRoute(phoneRoute{method: http.MethodGet, path: "/approvals", access: accessPhoneSecure,
		refusal: says("The approvals show only over Tailscale's HTTPS address."),
		serve: func(d *Deck, w http.ResponseWriter, _ *request, _ string) {
			writeJSON(w, http.StatusOK, d.approvalsWithAsks(nowMillis()))
		}})
}

// approvalsAnswer is what GET /setup/gm/approvals answers, the longest waiting
// first and those whose time isn't known last. now is in milliseconds.
func approvalsAnswer(items []approvalItem, now float64) map[string]any {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if (a.since > 0) != (b.since > 0) {
			return a.since > 0
		}
		if a.since != b.since {
			return a.since < b.since
		}
		return a.session < b.session
	})
	rows := []map[string]any{}
	for _, it := range items {
		var since, waiting any
		if it.since > 0 {
			since, waiting = int64(it.since), int64(max(0, now-it.since)/1000)
		}
		rows = append(rows, map[string]any{
			"session": it.session, "hire": it.hire, "name": it.name, "nickname": it.nickname, "agent": it.agent,
			"floor": seatOrNull(it.floor), "desk": seatOrNull(it.desk), "kind": it.kind, "tool": it.tool,
			"label": approvalLabel(it.tool, it.input), "since": since, "waitingSeconds": waiting,
			// approvalItems never sets it: this app keeps no record of notes made for the paired phones.
			"notified": it.notified,
		})
	}
	return map[string]any{"at": int64(now), "approvals": rows}
}

// approvalItems is the sessions asking you now, with the hire the front desk
// knows each as and where it sits. It reads; it changes nothing.
func (d *Deck) approvalItems() []approvalItem {
	live := d.frontDeskLive()
	file := readRegistry(registryPath(), false)
	d.mu.Lock()
	asking := map[string]*Session{}
	for _, s := range d.sessions.All() {
		if s.State == Asking {
			asking[s.ID] = s
		}
	}
	type asked struct {
		tool, input string
		since       float64
	}
	facts := map[string]asked{}
	for id, s := range asking {
		tool := s.askedTool
		if tool == "" {
			tool = s.WaitingFor
		}
		since := s.askedSince
		if since <= 0 {
			since = s.LastEventAt
		}
		facts[id] = asked{tool, s.askedInput, since}
	}
	d.mu.Unlock()
	var items []approvalItem
	for _, l := range live {
		f, ok := facts[l.ID]
		if !ok {
			continue
		}
		it := approvalItem{session: l.ID, name: lastPart(l.Project), agent: l.Agent, floor: l.Floor, desk: l.Desk,
			kind: "permission", tool: f.tool, input: f.input, since: f.since}
		for _, h := range file.hires {
			if h.State != "fired" && h.State != "failed" && works(l, h) {
				it.hire, it.name, it.nickname = h.ID, h.Name, h.Nickname
				break
			}
		}
		if f.tool == "AskUserQuestion" {
			it.kind = "question"
		}
		items = append(items, it)
	}
	return items
}

// approvals answers GET /setup/gm/approvals. Any other method is refused: the
// route can't change a prompt.
func (rt *router) approvals(w http.ResponseWriter, r *request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeStatus(w)(http.StatusMethodNotAllowed, "The approvals only show. A prompt is answered by your tap alone.")
		return
	}
	writeJSON(w, http.StatusOK, rt.deck.approvalsWithAsks(float64(time.Now().UnixMilli())))
}

// approvalsWithAsks is approvalsAnswer and, beside the sessions asking, "asks": what a GM, an MCP key, a night job or
// a hire waits to be allowed, for the owner to answer (grants.go). The Mac's wire format has no such key, which its
// reader leaves alone.
func (d *Deck) approvalsWithAsks(now float64) map[string]any {
	answer := approvalsAnswer(d.approvalItems(), now)
	answer["asks"] = d.grants.asksAnswer(now)
	return answer
}
