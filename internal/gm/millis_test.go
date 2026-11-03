package gm

import (
	"encoding/json"
	"strings"
	"testing"
)

// The app once answered "at": 1793524230038.1179 and the CLI died (M4). Every
// time field reads a fractional number and rounds it down.
func TestFractionalMillisecondsAreRoundedDown(t *testing.T) {
	const frac = `1793524230038.1179`
	const whole = int64(1793524230038)
	cases := map[string]string{
		"Message":   `{"id":"m","at":` + frac + `}`,
		"Member":    `{"id":"h","lastEventAt":` + frac + `,"idleFor":12.7}`,
		"Event":     `{"id":3,"at":` + frac + `}`,
		"Approval":  `{"since":` + frac + `,"waitingSeconds":4.9}`,
		"TeamReply": `{"at":` + frac + `}`,
		"Approvals": `{"at":` + frac + `}`,
		"Watch":     `{"id":"w","createdAt":` + frac + `,"due":` + frac + `}`,
	}
	var m Message
	var mem Member
	var e Event
	var a Approval
	var tr TeamReply
	var ar ApprovalsReply
	var w Watch
	targets := map[string]any{"Message": &m, "Member": &mem, "Event": &e, "Approval": &a, "TeamReply": &tr, "Approvals": &ar, "Watch": &w}
	for name, body := range cases {
		if err := json.Unmarshal([]byte(body), targets[name]); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	got := []int64{int64(m.At), int64(*mem.LastEventAt), int64(e.At), int64(*a.Since), int64(tr.At), int64(ar.At), int64(w.CreatedAt), int64(w.Due)}
	for i, g := range got {
		if g != whole {
			t.Errorf("field %d = %d, want %d", i, g, whole)
		}
	}
	if *mem.IdleFor != 12 || *a.WaitingSeconds != 4 {
		t.Errorf("seconds = %d, %d", *mem.IdleFor, *a.WaitingSeconds)
	}
	if err := json.Unmarshal([]byte(`{"lastEventAt":null}`), &mem); err != nil || mem.LastEventAt != nil {
		t.Errorf("null: %v %v", err, mem.LastEventAt)
	}
	if err := json.Unmarshal([]byte(`{"at":"soon"}`), &m); err == nil {
		t.Error("a string was accepted as a time")
	}
}

// The outbox command (and so the MCP tool, which runs it) shows a message with a fractional time.
func TestOutboxReadsAFractionalTime(t *testing.T) {
	raw := `{"messages":[{"id":"gm-1","from":"gm","to":"abc","toName":"Rev","at":1793524230038.1179,"state":"queued","size":2,"attempts":0,"text":"hi"}]}`
	app := newFakeApp(t, serve([]byte(raw), 200))
	code, out, errOut := runGM(app.env(t, withToken()), "outbox")
	if code != 0 || !strings.Contains(out, "gm-1") {
		t.Fatalf("exit %d: %s%s", code, out, errOut)
	}
	_, out, _ = runGM(app.env(t, withToken()), "outbox", "--since", "1793524230038")
	if strings.Contains(out, "gm-1") {
		t.Errorf("a message at the same millisecond came back:\n%s", out)
	}
}
