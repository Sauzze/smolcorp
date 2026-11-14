package desktop

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// Take over in T3 Code and Hand back (t3takeover.go, t3internal.go): the gate, every refusal, the order each goes
// in, the two-writers check (anything still holding the session refuses Hand back), and T3 Code's internal calls
// against a fake server of its own; the fake t3 of t3desk_test.go signs in and out. No real T3 Code, t3, tmux or
// claude runs here: the hire's stop and resume are stand-ins (their tmux side is the fire's and the engine's).

const t3TakeOverSession = "256defd6-77e9-42af-8577-d7df83998fa7"

// t3FakeServer is T3 Code's server as far as the two calls: a ticket for the bearer the fake t3 issues, then its
// socket, which pings (a frame and a message) before it answers, the answer in two frames.
type t3FakeServer struct {
	srv    *httptest.Server
	mu     sync.Mutex
	calls  []string
	answer map[string]map[string]any
	// deny refuses every bearer.
	deny bool
}

func newT3FakeServer(t *testing.T, home string) *t3FakeServer {
	t.Helper()
	f := &t3FakeServer{answer: map[string]map[string]any{
		"agentSessions.import":          {"_tag": "Success", "value": map[string]any{"importedCount": 1, "skippedCount": 0}},
		"orchestration.forceStopThread": {"_tag": "Success", "value": map[string]any{"outcome": "released", "sharedWithThreadIds": []any{}}},
	}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/websocket-ticket", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		deny := f.deny
		f.mu.Unlock()
		if deny || r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer fake-tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"ticket":"tk-1","expiresAt":"2026-11-11T00:00:00.000Z"}`)
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("wsTicket") != "tk-1" || r.URL.Query().Get("orchestrationProtocol") != "2" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + t3CallGUID))
		fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n",
			base64.StdEncoding.EncodeToString(sum[:]))
		rw.Flush()
		// A client's frames are masked.
		peek, _ := rw.Reader.Peek(2)
		if len(peek) < 2 || peek[1]&0x80 == 0 {
			return
		}
		_, data, err := t3SocketRead(rw.Reader, 1<<20)
		if err != nil {
			return
		}
		var request map[string]any
		_ = json.Unmarshal(data, &request)
		method, _ := request["tag"].(string)
		payload, _ := json.Marshal(request["payload"])
		f.mu.Lock()
		f.calls = append(f.calls, method+" "+string(payload))
		exit := f.answer[method]
		f.mu.Unlock()
		_ = t3SocketWrite(conn, 9, []byte("hi"), false)
		if op, back, err := t3SocketRead(rw.Reader, 1<<20); err != nil || op != 10 || string(back) != "hi" {
			return
		}
		_ = t3SocketWrite(conn, 1, []byte(`{"_tag":"Ping"}`), false)
		if _, back, err := t3SocketRead(rw.Reader, 1<<20); err != nil || string(back) != `{"_tag":"Pong"}` {
			return
		}
		answer, _ := json.Marshal([]any{map[string]any{"_tag": "Exit", "requestId": "1", "exit": exit}})
		half := len(answer) / 2
		_, _ = conn.Write(append([]byte{0x01, byte(half)}, answer[:half]...))
		// A ping between the two fragments (RM5b S1): answered, and the first half kept.
		_ = t3SocketWrite(conn, 9, []byte("mid"), false)
		if op, back, err := t3SocketRead(rw.Reader, 1<<20); err != nil || op != 10 || string(back) != "mid" {
			return
		}
		_, _ = conn.Write(append([]byte{0x80, byte(len(answer) - half)}, answer[half:]...))
		_, _, _ = t3SocketRead(rw.Reader, 1<<20)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	f.listen(t, home, f.srv.URL)
	return f
}

// listen writes T3 Code's server-runtime.json in home's ~/.t3, its origin given.
func (f *t3FakeServer) listen(t *testing.T, home, origin string) {
	t.Helper()
	folder := filepath.Join(home, ".t3", "userdata")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	state, _ := json.Marshal(map[string]any{"version": 1, "pid": 1, "port": 1, "origin": origin, "startedAt": "2026-11-11T00:00:00.000Z"})
	if err := os.WriteFile(filepath.Join(folder, "server-runtime.json"), state, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *t3FakeServer) made() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// t3TakeOverRig is a hire Smolcorp runs in a tmux session of its own, in a real worktree of a real repo, with its
// session's transcript on the default account; and the ways, each a stand-in that writes down what it did.
type t3TakeOverRig struct {
	*t3Rig
	srv     *t3FakeServer
	hire    Hire
	session t3Session
	saved   Hire
	holders func() []int
	looks   bool
	stopped error
	resumed error
	// socket is the resumed tmux socket as tmux spells it, "" for none.
	socket   string
	stat     func(string) (os.FileInfo, error)
	now      time.Time
	pausedAt func()
}

func newT3TakeOverRig(t *testing.T, flags ...string) *t3TakeOverRig {
	t.Helper()
	r := newT3Rig(t)
	r.fake(flags...)
	repo := r.repo()
	wt := r.worktree()
	r.git(repo, "worktree", "add", "-q", "-b", "hire/ada-abcdef", wt)
	transcript := filepath.Join(r.home, ".claude", "projects", "-repo-hires-ada-abcdef", t3TakeOverSession+".jsonl")
	if err := os.MkdirAll(filepath.Dir(transcript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := Hire{ID: "abcdef123456", Name: "Ada", Role: "dev", Folder: wt, Worktree: wt, Repo: repo, Branch: "hire/ada-abcdef",
		TmuxSocket: filepath.Join(supportDir, hireSocketPrefix+"abcdef123456"), Model: "claude-haiku-5-5", State: "working"}
	k := &t3TakeOverRig{t3Rig: r, srv: newT3FakeServer(t, r.home), hire: h, session: t3Session{ID: t3TakeOverSession, Transcript: transcript},
		saved: h, holders: func() []int { return nil }, looks: true, stat: os.Stat, now: time.Unix(1_000_000, 0), pausedAt: func() {}}
	return k
}

func (k *t3TakeOverRig) ways() t3Handing {
	machine := k.machine()
	return t3Handing{Machine: machine,
		Call: func(program, method string, payload map[string]any) (map[string]any, error) {
			k.log = append(k.log, "call "+method)
			return t3Call(machine, program, method, payload)
		},
		Holding: func(folder, session string) ([]int, bool) {
			k.log = append(k.log, "look")
			if folder != k.hire.Worktree || session != t3TakeOverSession {
				k.t.Errorf("looked at %s %s", folder, session)
			}
			return k.holders(), k.looks
		},
		Name:  func(pid int) string { return "claude" },
		Stat:  func(path string) (os.FileInfo, error) { return k.stat(path) },
		Pause: func(time.Duration) { k.pausedAt() },
		Now:   func() time.Time { k.now = k.now.Add(time.Second); return k.now },
		Fence: func() (map[string]any, error) { return k.settings(), nil },
		Stop: func(h Hire, session string) error {
			k.log = append(k.log, "stop "+session)
			return k.stopped
		},
		Resume: func(h Hire, session string) (string, error) {
			k.log = append(k.log, "resume "+session)
			return k.socket, k.resumed
		},
		Save: func(id string, change func(*Hire)) error {
			if id != k.hire.ID {
				k.t.Errorf("saved %s", id)
			}
			change(&k.saved)
			k.log = append(k.log, "save")
			return nil
		},
	}
}

func (k *t3TakeOverRig) fence() string {
	return filepath.Join(k.hire.Worktree, ".claude", "settings.local.json")
}

// takenOver is the rig's hire as Take over leaves it: a T3 desk, its fence on its worktree.
func (k *t3TakeOverRig) takenOver() Hire {
	k.t.Helper()
	if !t3WriteFence(k.settings(), k.fence()) {
		k.t.Fatal("no fence")
	}
	h := k.hire
	h.Engine, h.T3Thread, h.T3Session, h.T3Transcript = t3Engine, t3ImportedThread+t3TakeOverSession, k.session.ID, k.session.Transcript
	k.saved = h
	return h
}

// MARK: The gate and the refusals

func TestT3TakeOverGateRefusesInOneSentenceEach(t *testing.T) {
	cases := []struct {
		on     bool
		status t3Status
		want   string
	}{
		{false, t3Status{t3Found, "v0.1.8"}, "T3 Code is off in Settings: turn it on to take a hire over in T3 Code."},
		{true, t3Status{t3None, ""}, "Install T3 Code and its t3 command to take a hire over in T3 Code (t3.codes)."},
		{true, t3Status{t3AppOnlyIs, ""}, "Taking a hire over in T3 Code needs its t3 command: in T3 Code, open Settings → About and install it."},
		{true, t3Status{t3Found, ""}, "Smolcorp couldn't read t3's version, so it won't take a hire over in T3 Code."},
		{true, t3Status{t3Found, "v0.1.9"}, "Take over uses T3 Code's internal calls, tried on t3 v0.1.8 only, and this is t3 v0.1.9, so Smolcorp won't use them."},
		{true, t3Status{t3CliOnly, "v0.1.7"}, "Take over uses T3 Code's internal calls, tried on t3 v0.1.8 only, and this is t3 v0.1.7, so Smolcorp won't use them."},
		{true, t3Status{t3Found, "v0.1.8"}, ""},
		{true, t3Status{t3CliOnly, "v0.1.8"}, ""},
	}
	for _, c := range cases {
		if got := t3TakeOverGate(c.on, c.status); got != c.want {
			t.Errorf("%v %+v: %q", c.on, c.status, got)
		}
		if c.want != "" && strings.Count(c.want, ". ") > 0 {
			t.Errorf("more than one sentence: %q", c.want)
		}
	}
}

func TestT3TakeOverRefusesEveryHireItCantTakeOver(t *testing.T) {
	home := "/home/ada"
	wt := "/home/ada/repo-hires/ada-abcdef"
	ok := Hire{ID: "abcdef123456", Name: "Ada", Role: "dev", Folder: wt, Worktree: wt, State: "working",
		TmuxSocket: "/support/" + hireSocketPrefix + "abcdef123456"}
	session := t3Session{ID: t3TakeOverSession, Transcript: filepath.Join(home, ".claude", "projects", "-x", t3TakeOverSession+".jsonl")}
	with := func(change func(*Hire)) Hire { h := ok; change(&h); return h }
	cases := []struct {
		h       Hire
		session t3Session
		test    bool
		want    string
	}{
		{ok, session, true, "A test copy doesn't take hires over in T3 Code: that's your real T3 Code."},
		{with(func(h *Hire) { h.State = "fired" }), session, false, "Ada is already gone."},
		{with(func(h *Hire) { h.Engine = t3Engine }), session, false, "Ada already runs in T3 Code."},
		{with(func(h *Hire) { h.T3Session = t3TakeOverSession }), session, false, "Ada is waiting to be handed back from T3 Code: hand it back first."},
		{with(func(h *Hire) { h.Role = gmRole }), session, false, "The General Manager isn't taken over in T3 Code."},
		// RMSY S1: it would come back on your subscription.
		{with(func(h *Hire) { h.Account = apiKeyAccountName }), session, false, "Ada works on the API-key account, and T3 Code's agent works on your Claude account, so it can't take this session over."},
		{with(func(h *Hire) { h.ViaLauncher, h.TmuxSocket = true, "" }), session, false, "Ada runs in a window of Smolcorp's launcher: Take over reaches only a hire in a tmux session of its own, for now."},
		{with(func(h *Hire) { h.TmuxSocket = "" }), session, false, "Only a hire Smolcorp runs in a tmux session of its own can be taken over in T3 Code, and Ada isn't one."},
		{with(func(h *Hire) { h.TmuxSocket = "/tmp/default" }), session, false, "Only a hire Smolcorp runs in a tmux session of its own can be taken over in T3 Code, and Ada isn't one."},
		{with(func(h *Hire) { h.Worktree = "" }), session, false, "Ada has no worktree of its own, and in T3 Code its fence stays on one, so it can't be taken over."},
		{with(func(h *Hire) { h.Folder = wt + "/web" }), session, false, "Ada works in a folder inside its worktree, and T3 Code takes over only a session started at a project's top."},
		{ok, t3Session{}, false, "Smolcorp hasn't heard from Ada's Claude Code yet, so it doesn't know which session to hand over."},
		{ok, t3Session{ID: "../../etc", Transcript: session.Transcript}, false, "Ada's session id isn't one T3 Code resumes, so it can't be taken over."},
		{ok, t3Session{ID: t3TakeOverSession, Transcript: "/home/ada/.claude-work/projects/-x/" + t3TakeOverSession + ".jsonl"}, false,
			"Ada works on another Claude account, and T3 Code's agent works on your default one, so it can't take this session over."},
		{ok, t3Session{ID: t3TakeOverSession}, false, "Ada works on another Claude account, and T3 Code's agent works on your default one, so it can't take this session over."},
		{ok, t3Session{ID: t3TakeOverSession, Transcript: filepath.Join(home, ".claude", "projects", "-x", "other.jsonl")}, false,
			"Ada works on another Claude account, and T3 Code's agent works on your default one, so it can't take this session over."},
		{ok, session, false, ""},
	}
	for i, c := range cases {
		if got := t3TakeOverRefusal(c.h, c.session, home, c.test); got != c.want {
			t.Errorf("%d: %q, want %q", i, got, c.want)
		}
	}
}

// MARK: Take over

func TestT3TakeOverStopsTheHireThenFencesImportsThatSessionAndMakesItAT3Desk(t *testing.T) {
	k := newT3TakeOverRig(t)
	thread, err := t3TakeOver(k.hire, k.session, k.ways())
	if err != nil || thread != t3ImportedThread+t3TakeOverSession {
		t.Fatalf("%q %v", thread, err)
	}
	// Looked first (it can see), stopped, on record, nothing holds it, then T3 Code; the import is the last word.
	want := []string{"look", "stop " + t3TakeOverSession, "save", "look", "save", "call agentSessions.import", "save"}
	if !slices.Equal(k.log, want) {
		t.Errorf("order:\n%q", k.log)
	}
	// t3 ran: project add, sign in, sign out, thread show; nothing else.
	firsts := k.firsts()
	if !reflect.DeepEqual(firsts, [][]string{{"project", "add"}, {"auth", "session"}, {"auth", "session"}, {"thread", "show"}}) {
		t.Errorf("t3 ran %q", firsts)
	}
	runs := k.runs()
	if !slices.Equal(runs[0], []string{"project", "add", k.hire.Worktree, "--title", "Ada · hire/ada-abcdef"}) ||
		!slices.Equal(runs[1], []string{"auth", "session", "issue", "--scope", "orchestration:operate", "--ttl", "2m", "--label", t3CallLabel, "--json"}) ||
		!slices.Equal(runs[2], []string{"auth", "session", "revoke", "s-1"}) ||
		!slices.Equal(runs[3], []string{"thread", "show", thread, "--json"}) {
		t.Errorf("t3 ran %q", runs)
	}
	// The fence was on disk, and kept out of git, when T3 Code heard of the worktree; and it's the hire's, byte for byte.
	notes, _ := os.ReadFile(k.notes)
	if strings.TrimSpace(string(notes)) != "worktree fence ignored" {
		t.Errorf("at project add: %q", notes)
	}
	if !t3FenceHolds(k.fence(), k.settings()) {
		t.Error("the fence isn't the hire's")
	}
	// THAT session: into the project just added, in its worktree.
	payload, _ := json.Marshal(map[string]any{"expectedWorkspaceRoot": k.hire.Worktree, "projectId": "p-1"})
	if calls := k.srv.made(); !slices.Equal(calls, []string{"agentSessions.import " + string(payload)}) {
		t.Errorf("T3 Code was asked %q", calls)
	}
	s := k.saved
	if s.Engine != t3Engine || s.T3Thread != thread || s.T3Session != t3TakeOverSession || s.T3Transcript != k.session.Transcript ||
		s.T3Link != "https://app.t3.codes/threads/env-1/mcp%3Aabc-1" || s.State != "idle" {
		t.Errorf("registry: %+v", s)
	}
	// A T3 desk now, with Hand back on its card.
	if offered := t3TakeOverJSON(s, true); offered["handBack"] != true || offered["tag"] != "EXPERIMENTAL" {
		t.Errorf("%v", offered)
	}
}

func TestT3TakeOverRefusesBeforeStoppingAnythingItCantCheck(t *testing.T) {
	k := newT3TakeOverRig(t)
	// It can't see which processes hold the session (the Go core on a Mac): nothing stopped, nothing run.
	k.looks = false
	_, err := t3TakeOver(k.hire, k.session, k.ways())
	if refused := refusalOf(t, err); refused.Status != http.StatusConflict || refused.Message != t3CantLook("Ada") {
		t.Errorf("%+v", refused)
	}
	if !slices.Equal(k.log, []string{"look"}) || len(k.runs()) != 0 || k.saved != k.hire {
		t.Errorf("%q %q %+v", k.log, k.runs(), k.saved)
	}
	// Its window isn't idle: the stop refuses, in its own words, and nothing else happens.
	k.looks, k.log = true, nil
	k.stopped = t3Refused(http.StatusConflict, t3WindowRefusal("Ada", PaneBusy, true))
	_, err = t3TakeOver(k.hire, k.session, k.ways())
	if refused := refusalOf(t, err); refused.Message != "Ada is working right now: take it over once it's done. Nothing was changed." {
		t.Errorf("%+v", refused)
	}
	if !slices.Equal(k.log, []string{"look", "stop " + t3TakeOverSession}) || len(k.runs()) != 0 || exists(k.fence()) {
		t.Errorf("%q %q", k.log, k.runs())
	}
	// The repo keeps its own settings.local.json in the worktree: never written over, nothing stopped.
	k.stopped, k.log = nil, nil
	if err := os.MkdirAll(filepath.Dir(k.fence()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(k.fence(), []byte(`{"mine":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = t3TakeOver(k.hire, k.session, k.ways())
	if refused := refusalOf(t, err); !strings.HasPrefix(refused.Message, "This repository keeps its own .claude/settings.local.json") || len(k.log) != 0 {
		t.Errorf("%+v %q", refused, k.log)
	}
	if data, _ := os.ReadFile(k.fence()); string(data) != `{"mine":true}` {
		t.Errorf("written over: %s", data)
	}
}

func TestT3TakeOverWithAProcessStillHoldingTheSessionHandsNothingToT3Code(t *testing.T) {
	k := newT3TakeOverRig(t)
	looked := 0
	// After the stop, something else (your own claude --resume, say) still holds the session.
	k.holders = func() []int {
		looked++
		if looked == 1 {
			return nil
		}
		return []int{4242}
	}
	_, err := t3TakeOver(k.hire, k.session, k.ways())
	refused := refusalOf(t, err)
	if refused.Status != http.StatusConflict || refused.Message != "Ada was stopped, but a process still holds its session or works in its worktree (claude, PID 4242), so it wasn't handed to T3 Code. Hand it back once that's closed." {
		t.Errorf("%+v", refused)
	}
	// T3 Code heard nothing; the hire is on record, so Hand back is offered; no fence was put on.
	if len(k.runs()) != 0 || len(k.srv.made()) != 0 || exists(k.fence()) {
		t.Errorf("%q %q", k.runs(), k.srv.made())
	}
	if k.saved.T3Session != t3TakeOverSession || k.saved.Engine != "" || t3TakeOverJSON(k.saved, true)["handBack"] != true {
		t.Errorf("%+v", k.saved)
	}
}

func TestT3TakeOverAnImportT3CodeRefusesIsHandedBackAtOnce(t *testing.T) {
	k := newT3TakeOverRig(t)
	k.srv.answer["agentSessions.import"] = map[string]any{"_tag": "Failure",
		"cause": map[string]any{"_tag": "Fail", "error": map[string]any{"_tag": "AgentSessionImportProjectChangedError", "projectId": "p-1"}}}
	_, err := t3TakeOver(k.hire, k.session, k.ways())
	refused := refusalOf(t, err)
	if refused.Message != "T3 Code didn't import Ada's session: T3 Code refused it (AgentSessionImportProjectChangedError). Nothing stayed in T3 Code, and Ada was resumed in its tmux session." {
		t.Errorf("%+v", refused)
	}
	// Handed back: T3 Code let go, its project removed, nothing holds it, the fence off, resumed.
	tail := k.log[slices.Index(k.log, "call agentSessions.import")+1:]
	if !slices.Equal(tail, []string{"look", "call orchestration.forceStopThread", "save", "look", "save", "resume " + t3TakeOverSession, "save"}) {
		t.Errorf("handed back: %q", tail)
	}
	if !slices.ContainsFunc(k.runs(), func(run []string) bool {
		return slices.Equal(run, []string{"project", "remove", k.hire.Worktree, "--force"})
	}) || exists(k.fence()) {
		t.Errorf("%q", k.runs())
	}
	if s := k.saved; s.Engine != "" || s.T3Session != "" || s.T3Thread != "" || s.State != "working" {
		t.Errorf("%+v", s)
	}
}

func TestT3TakeOverAProjectT3CodeWontAddIsNeverRemoved(t *testing.T) {
	// "already exists": someone else's project for that folder. Handed back without touching it.
	k := newT3TakeOverRig(t, "addfail")
	_, err := t3TakeOver(k.hire, k.session, k.ways())
	if refused := refusalOf(t, err); !strings.HasPrefix(refused.Message, "T3 Code didn't add Ada's worktree as its own project (Project already exists: ") ||
		!strings.HasSuffix(refused.Message, "Ada was resumed in its tmux session.") {
		t.Errorf("%+v", refused)
	}
	for _, run := range k.runs() {
		if slices.Equal(run[:2], []string{"project", "remove"}) {
			t.Errorf("removed a project it didn't add: %q", run)
		}
	}
	if slices.Contains(k.log, "call orchestration.forceStopThread") || exists(k.fence()) || k.saved.T3Session != "" {
		t.Errorf("%q %+v", k.log, k.saved)
	}
}

func TestT3TakeOverAProjectAddedSomewhereElseIsTakenBackAsSmolcorps(t *testing.T) {
	// T3 Code says it added a project, but not at the worktree: it's still the one Smolcorp asked for, so it's removed.
	k := newT3TakeOverRig(t, "elsewhere")
	_, err := t3TakeOver(k.hire, k.session, k.ways())
	if refused := refusalOf(t, err); !strings.HasSuffix(refused.Message, "Ada was resumed in its tmux session.") {
		t.Errorf("%+v", refused)
	}
	if !slices.ContainsFunc(k.runs(), func(run []string) bool {
		return slices.Equal(run, []string{"project", "remove", k.hire.Worktree, "--force"})
	}) || len(k.srv.made()) != 1 || !strings.HasPrefix(k.srv.made()[0], "orchestration.forceStopThread ") {
		t.Errorf("%q %q", k.runs(), k.srv.made())
	}
}

// MARK: Hand back

func TestT3HandBackLetsGoThenChecksThenResumesTheSameSession(t *testing.T) {
	k := newT3TakeOverRig(t)
	h := k.takenOver()
	note, err := t3HandBack(h, k.ways())
	if err != nil || note != "" {
		t.Fatalf("%q %v", note, err)
	}
	want := []string{"look", "call orchestration.forceStopThread", "save", "look", "save", "resume " + t3TakeOverSession, "save"}
	if !slices.Equal(k.log, want) {
		t.Errorf("order:\n%q", k.log)
	}
	payload, _ := json.Marshal(map[string]any{"threadId": t3ImportedThread + t3TakeOverSession})
	if calls := k.srv.made(); !slices.Equal(calls, []string{"orchestration.forceStopThread " + string(payload)}) {
		t.Errorf("%q", calls)
	}
	if !reflect.DeepEqual(k.firsts(), [][]string{{"auth", "session"}, {"auth", "session"}, {"project", "remove"}}) {
		t.Errorf("t3 ran %q", k.runs())
	}
	if exists(k.fence()) {
		t.Error("the fence stayed on the worktree")
	}
	if s := k.saved; s.Engine != "" || s.T3Thread != "" || s.T3Link != "" || s.T3Session != "" || s.T3Transcript != "" || s.State != "working" {
		t.Errorf("%+v", s)
	}
}

// Two writers: while anything still holds the session (T3 Code's claude that Force stop didn't end, your own
// claude --resume, a tail of its file), Hand back refuses, and never resumes.
func TestT3HandBackWithTheSessionStillHeldNeverResumes(t *testing.T) {
	k := newT3TakeOverRig(t)
	h := k.takenOver()
	k.holders = func() []int { return []int{777, 778} }
	_, err := t3HandBack(h, k.ways())
	refused := refusalOf(t, err)
	if refused.Status != http.StatusConflict || refused.Message != "A process still holds Ada's session or works in its worktree (claude, PID 777; claude, PID 778), so it wasn't resumed: two agents on one session lose turns silently. If that's yours, close it and hand back again." {
		t.Errorf("%+v", refused)
	}
	if slices.ContainsFunc(k.log, func(s string) bool { return strings.HasPrefix(s, "resume") }) || !exists(k.fence()) {
		t.Errorf("%q", k.log)
	}
	// Still on record, so Hand back is offered again; T3 Code's thread is gone (its project removed).
	if k.saved.T3Session != t3TakeOverSession || k.saved.T3Thread != "" || t3TakeOverJSON(k.saved, true)["handBack"] != true {
		t.Errorf("%+v", k.saved)
	}
	// Once it's let go, the same Hand back goes on from there, with nothing more of T3 Code's to stop.
	k.holders, k.log = func() []int { return nil }, nil
	if _, err := t3HandBack(k.saved, k.ways()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(k.log, []string{"look", "look", "save", "resume " + t3TakeOverSession, "save"}) {
		t.Errorf("%q", k.log)
	}
}

func TestT3HandBackRefusesWhatItCantBeSureOf(t *testing.T) {
	cases := []struct {
		name string
		set  func(k *t3TakeOverRig)
		want string
		ran  []string
	}{
		{"can't look", func(k *t3TakeOverRig) { k.looks = false }, t3CantLook("Ada"), []string{"look"}},
		{"only part stopped", func(k *t3TakeOverRig) {
			k.srv.answer["orchestration.forceStopThread"] = map[string]any{"_tag": "Success", "value": map[string]any{"outcome": "shared", "sharedWithThreadIds": []any{"t-2"}}}
		}, "T3 Code stopped only part of what holds Ada's session (shared), so it wasn't handed back.", []string{"look", "call orchestration.forceStopThread"}},
		{"project kept", func(k *t3TakeOverRig) { k.fake("removefail") },
			"T3 Code didn't remove Ada's project (Could not reach the T3 Code desktop app.), so it wasn't handed back: T3 Code could still open its session.", []string{"look", "call orchestration.forceStopThread"}},
		{"still being written", func(k *t3TakeOverRig) {
			k.pausedAt = func() {
				f, _ := os.OpenFile(k.session.Transcript, os.O_APPEND|os.O_WRONLY, 0)
				_, _ = f.WriteString("{}\n")
				f.Close()
			}
		}, "Ada's session is still being written, or its file is gone, so it wasn't resumed.", []string{"look", "call orchestration.forceStopThread", "save", "look"}},
		{"file gone", func(k *t3TakeOverRig) { _ = os.Remove(k.session.Transcript) },
			"Ada's session is still being written, or its file is gone, so it wasn't resumed.", []string{"look", "call orchestration.forceStopThread", "save", "look"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			k := newT3TakeOverRig(t)
			h := k.takenOver()
			c.set(k)
			_, err := t3HandBack(h, k.ways())
			if refused := refusalOf(t, err); refused.Message != c.want {
				t.Errorf("%q", refused.Message)
			}
			if !slices.Equal(k.log, c.ran) || !exists(k.fence()) {
				t.Errorf("%q", k.log)
			}
		})
	}
}

// RM5b S4: Hand back isn't stranded. T3 Code's server refusing (or its app closed), or its internal calls not to be
// used (T3 Code off, another t3), falls back to removing the project with --force, which ends T3 Code's agent for
// it, then the same hard check before any resume.
func TestT3HandBackFallsBackToRemovingTheProjectThenTheHardCheck(t *testing.T) {
	for _, c := range []struct {
		name string
		set  func(k *t3TakeOverRig, ways *t3Handing)
		log  []string
	}{
		{"its server refuses the force stop", func(k *t3TakeOverRig, _ *t3Handing) {
			k.srv.answer["orchestration.forceStopThread"] = map[string]any{"_tag": "Failure", "cause": map[string]any{"_tag": "Fail", "error": map[string]any{"_tag": "OrchestrationV2ForceStopThreadError"}}}
		}, []string{"look", "call orchestration.forceStopThread", "save", "look", "save", "resume " + t3TakeOverSession, "save"}},
		{"its app is closed", func(k *t3TakeOverRig, _ *t3Handing) {
			k.srv.listen(k.t, k.home, "http://127.0.0.1:1")
		}, []string{"look", "call orchestration.forceStopThread", "save", "look", "save", "resume " + t3TakeOverSession, "save"}},
		{"T3 Code off, or another t3", func(_ *t3TakeOverRig, ways *t3Handing) { ways.NoCalls = true },
			[]string{"look", "save", "look", "save", "resume " + t3TakeOverSession, "save"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := newT3TakeOverRig(t)
			h := k.takenOver()
			ways := k.ways()
			c.set(k, &ways)
			if _, err := t3HandBack(h, ways); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(k.log, c.log) {
				t.Errorf("order:\n%q", k.log)
			}
			if !slices.ContainsFunc(k.runs(), func(run []string) bool {
				return slices.Equal(run, []string{"project", "remove", h.Worktree, "--force"})
			}) {
				t.Errorf("the project wasn't removed: %q", k.runs())
			}
			if exists(k.fence()) || k.saved.T3Session != "" || k.saved.T3Thread != "" {
				t.Errorf("%+v", k.saved)
			}
		})
	}
	// The fallback is no way round the hard check: a process still holding the session is never resumed over.
	k := newT3TakeOverRig(t)
	h := k.takenOver()
	ways := k.ways()
	ways.NoCalls = true
	k.holders = func() []int { return []int{777} }
	_, err := t3HandBack(h, ways)
	if refusalOf(t, err).Status != http.StatusConflict || slices.ContainsFunc(k.log, func(s string) bool { return strings.HasPrefix(s, "resume") }) {
		t.Errorf("%v %q", err, k.log)
	}
	// Without t3 at all, nothing is changed, in Hand back's own words.
	k = newT3TakeOverRig(t)
	h = k.takenOver()
	_ = os.Remove(filepath.Join(k.home, ".local", "bin", "t3"))
	_, err = t3HandBack(h, k.ways())
	if refusalOf(t, err).Message != "Handing Ada back needs T3 Code's t3 command, to remove its project there, and it isn't there, so nothing was changed: in T3 Code, open Settings → About and install it, then hand back again." || len(k.log) != 0 {
		t.Errorf("%v %q", err, k.log)
	}
}

// RM5b S2: the registry tried again right after the stop; if it still can't be written, the advice is to fire it
// and hire again, nothing goes to T3 Code, and nothing is resumed: never a bare claude --resume, and never a start
// from a settings file (RT126SG1 B3).
func TestT3TakeOverWhoseRegistryCantBeWrittenAfterTheStopAdvisesAFireAndAHire(t *testing.T) {
	k := newT3TakeOverRig(t)
	ways := k.ways()
	save, failing := ways.Save, 2
	ways.Save = func(id string, change func(*Hire)) error {
		if failing > 0 {
			failing--
			return errors.New("disk full")
		}
		return save(id, change)
	}
	if _, err := t3TakeOver(k.hire, k.session, ways); err != nil {
		t.Fatalf("two failed tries, then saved: %v", err)
	}
	k = newT3TakeOverRig(t)
	ways = k.ways()
	tries := 0
	ways.Save = func(string, func(*Hire)) error { tries++; return errors.New("disk full") }
	_, err := t3TakeOver(k.hire, k.session, ways)
	want := "Ada was stopped, but the registry couldn't be written, so nothing went to T3 Code. Fire it and hire again."
	refused := refusalOf(t, err)
	if refused.Message != want || tries != t3SaveMost {
		t.Errorf("%d tries: %q", tries, refused.Message)
	}
	if strings.Contains(refused.Message, "--settings") || strings.Contains(refused.Message, "--resume") || strings.Contains(refused.Message, t3TakeOverSession) {
		t.Errorf("the advice starts a desk by hand: %q", refused.Message)
	}
	if len(k.runs()) != 0 || len(k.srv.made()) != 0 {
		t.Errorf("T3 Code heard of it: %q %q", k.runs(), k.srv.made())
	}
}

// RM5c B1: the thread is never only in memory. Its save failing once is tried again, and the last save holds it
// too, so Hand back removes T3 Code's project; failing every try, nothing more goes to T3 Code and the hire is
// handed back at once, its project removed.
func TestT3TakeOverNeverKeepsTheThreadOnlyInMemory(t *testing.T) {
	thread := t3ImportedThread + t3TakeOverSession
	removed := func(k *t3TakeOverRig) bool {
		return slices.ContainsFunc(k.runs(), func(run []string) bool {
			return slices.Equal(run, []string{"project", "remove", k.hire.Worktree, "--force"})
		})
	}
	// failing makes saves number from..to (counted from 1) fail.
	failing := func(k *t3TakeOverRig, ways t3Handing, from, to int) t3Handing {
		save, count := ways.Save, 0
		ways.Save = func(id string, change func(*Hire)) error {
			count++
			if count >= from && count <= to {
				return errors.New("disk full")
			}
			return save(id, change)
		}
		return ways
	}
	k := newT3TakeOverRig(t)
	if got, err := t3TakeOver(k.hire, k.session, failing(k, k.ways(), 2, 2)); err != nil || got != thread {
		t.Fatalf("%q %v", got, err)
	}
	if k.saved.Engine != t3Engine || k.saved.T3Thread != thread {
		t.Fatalf("the registry: %+v", k.saved)
	}
	if _, err := t3HandBack(k.saved, k.ways()); err != nil || !removed(k) {
		t.Errorf("Hand back: %v %q", err, k.runs())
	}

	k = newT3TakeOverRig(t)
	_, err := t3TakeOver(k.hire, k.session, failing(k, k.ways(), 2, 1+t3SaveMost))
	refused := refusalOf(t, err)
	if refused.Message != "The registry couldn't be written, so Ada wasn't handed over. Nothing stayed in T3 Code, and Ada was resumed in its tmux session." {
		t.Errorf("%q", refused.Message)
	}
	if !removed(k) || !slices.Contains(k.log, "resume "+t3TakeOverSession) {
		t.Errorf("not handed back: %q %q", k.runs(), k.log)
	}
	if slices.ContainsFunc(k.srv.made(), func(call string) bool { return strings.HasPrefix(call, "agentSessions.import") }) {
		t.Errorf("imported: %q", k.srv.made())
	}
	if k.saved.T3Session != "" || k.saved.T3Thread != "" || k.saved.Engine != "" {
		t.Errorf("the registry: %+v", k.saved)
	}
}

// RM5c N2: Hand back's last save is tried again; still unwritten, the hire runs in its tmux and Hand back says so.
func TestT3HandBackWhoseLastSaveFailsSaysSo(t *testing.T) {
	k := newT3TakeOverRig(t)
	h := k.takenOver()
	ways := k.ways()
	save, tries := ways.Save, 0
	ways.Save = func(id string, change func(*Hire)) error {
		probe := Hire{T3Session: "x"}
		if change(&probe); probe.T3Session == "" {
			tries++
			return errors.New("disk full")
		}
		return save(id, change)
	}
	note, err := t3HandBack(h, ways)
	if err != nil || note != t3ResumedUnsaved("Ada") || tries != t3SaveMost || !slices.Contains(k.log, "resume "+t3TakeOverSession) {
		t.Errorf("%q %v %d %q", note, err, tries, k.log)
	}
}

// RM5c N2 and N3: a hire whose Hand back resumed it but couldn't write its last save is handed back by the registry
// alone, as its own session runs at its own tmux socket: nothing looked at, stopped or resumed, t3 not run. With
// T3 Code off, t3 isn't asked its version. Without its session there, it's a Hand back as any other.
func TestT3HandBackOfAHireAlreadyResumedOnlyClearsTheRegistry(t *testing.T) {
	k := newT3TakeOverRig(t)
	keptMachine, keptStand := newT3Machine, t3HandingStand
	newT3Machine = k.machine
	t3HandingStand = func(*Deck, Hire) t3Handing { return k.ways() }
	t.Cleanup(func() { newT3Machine, t3HandingStand = keptMachine, keptStand })
	t3Kept.Lock()
	t3Kept.status, t3Kept.at = t3Status{}, time.Time{}
	t3Kept.Unlock()
	d := newTestDeck(t)
	resumed := k.hire
	resumed.T3Session, resumed.T3Transcript = k.session.ID, k.session.Transcript
	k.saved = resumed
	t3Registered(t, resumed)
	d.Ingest(map[string]any{"hook_event_name": "SessionStart", "session_id": t3TakeOverSession, "cwd": k.hire.Worktree,
		"transcript_path": k.session.Transcript}, "claude", "tmux:%1@"+k.hire.TmuxSocket, k.hire.Worktree, nowMillis())
	got := serve(t, d, t3PageCall("POST", "/setup/t3code/handback", `{"id":"`+k.hire.ID+`"}`))
	if got.Code != http.StatusOK || k.saved.T3Session != "" || k.saved.State != "working" || !slices.Equal(k.log, []string{"save"}) || len(k.runs()) != 0 {
		t.Errorf("%d %s %+v %q %q", got.Code, got.Body, k.saved, k.log, k.runs())
	}
	// The tmux that now drives its server is the one its row says, not the old stopped server's.
	if want := recordableTmux(tmuxPath()); k.saved.TmuxProgram != want {
		t.Errorf("its row's tmux: %q, want %q", k.saved.TmuxProgram, want)
	}
	// Taken over (in T3 Code, or with its thread on record), its session at its tmux is no proof: a Hand back as any other.
	for _, h := range []Hire{{Engine: t3Engine}, {T3Thread: t3ImportedThread + t3TakeOverSession}} {
		h.T3Session = t3TakeOverSession
		if t3AlreadyResumed(h, k.session) {
			t.Errorf("%+v", h)
		}
	}
	if t3AlreadyResumed(resumed, t3Session{}) || t3AlreadyResumed(resumed, t3Session{ID: "1" + t3TakeOverSession[1:]}) {
		t.Error("no session, or another, taken for its own")
	}
	// Its session gone from its tmux: the hard check and the resume, as any Hand back.
	d.Ingest(map[string]any{"hook_event_name": "SessionEnd", "session_id": t3TakeOverSession, "reason": "other"},
		"claude", "tmux:%1@"+k.hire.TmuxSocket, k.hire.Worktree, nowMillis())
	_ = updateRegistry(func(f *registryFile) { f.hires[0] = resumed })
	k.saved, k.log = resumed, nil
	if got := serve(t, d, t3PageCall("POST", "/setup/t3code/handback", `{"id":"`+k.hire.ID+`"}`)); got.Code != http.StatusOK ||
		!slices.Contains(k.log, "look") || !slices.Contains(k.log, "resume "+t3TakeOverSession) {
		t.Errorf("%d %s %q", got.Code, got.Body, k.log)
	}
	if slices.ContainsFunc(k.runs(), func(run []string) bool { return slices.Contains(run, "--version") }) {
		t.Errorf("t3 asked its version with T3 Code off: %q", k.runs())
	}
}

// RM5b S3: a hire taken over (in T3 Code, or stopped and waiting for Hand back) isn't fired from the office, Setup
// or the GM: hand it back first.
func TestT3AHireTakenOverIsHandedBackBeforeItsFired(t *testing.T) {
	waiting := Hire{ID: "abcdef123456", Name: "Ada", State: "working", T3Session: t3TakeOverSession}
	inT3 := waiting
	inT3.Engine = t3Engine
	for _, h := range []Hire{waiting, inT3} {
		want := "Ada was taken over in T3 Code: hand it back first, then fire it."
		if t3FireRefusal(h) != want || gmT3FireRefusal(h) != want {
			t.Errorf("%+v: %q %q", h, t3FireRefusal(h), gmT3FireRefusal(h))
		}
	}
	if t3FireRefusal(Hire{Name: "Bo"}) != "" || gmT3FireRefusal(Hire{Name: "Bo"}) != "" {
		t.Error("a plain hire refused")
	}
}

func TestT3HandBackThatCantResumeSaysSoAndOffersItAgain(t *testing.T) {
	k := newT3TakeOverRig(t)
	h := k.takenOver()
	k.resumed = errors.New("tmux didn't start it.")
	_, err := t3HandBack(h, k.ways())
	if refused := refusalOf(t, err); refused.Message != "Ada's session is free, but it couldn't be resumed: tmux didn't start it. Hand it back again to retry." {
		t.Errorf("%q", refused.Message)
	}
	if k.saved.Engine != "" || k.saved.T3Session != t3TakeOverSession || t3TakeOverJSON(k.saved, true)["handBack"] != true {
		t.Errorf("%+v", k.saved)
	}
}

// MARK: T3 Code's internal calls

func TestT3CallSignsInCallsAndSignsOut(t *testing.T) {
	r := newT3Rig(t)
	r.fake()
	srv := newT3FakeServer(t, r.home)
	value, err := t3Call(r.machine(), t3Program(r.machine()), "agentSessions.import", map[string]any{"projectId": "p-1"})
	if err != nil || value["importedCount"] != float64(1) {
		t.Fatalf("%v %v", value, err)
	}
	if !slices.Equal(srv.made(), []string{`agentSessions.import {"projectId":"p-1"}`}) {
		t.Errorf("%q", srv.made())
	}
	if !reflect.DeepEqual(r.firsts(), [][]string{{"auth", "session"}, {"auth", "session"}}) || r.runs()[1][2] != "revoke" {
		t.Errorf("%q", r.runs())
	}
}

func TestT3CallGoesOnlyToThisComputerAndNeverSaysTheToken(t *testing.T) {
	r := newT3Rig(t)
	r.fake()
	srv := newT3FakeServer(t, r.home)
	program := t3Program(r.machine())
	for _, origin := range []string{"http://192.168.1.5:3773", "https://127.0.0.1:3773", "http://example.com:80", "http://127.0.0.1", "http://u:p@127.0.0.1:3773", "not a url"} {
		srv.listen(t, r.home, origin)
		if _, err := t3Call(r.machine(), program, "agentSessions.import", nil); err == nil || err.Error() != "T3 Code's server isn't running on this computer" {
			t.Errorf("%s: %v", origin, err)
		}
	}
	if len(r.runs()) != 0 {
		t.Errorf("signed in for another computer: %q", r.runs())
	}
	srv.listen(t, r.home, srv.srv.URL)
	// A sign-in that fails says t3's error only, never what it printed (which could hold a token).
	r.fake("authfail")
	_, err := t3Call(r.machine(), program, "agentSessions.import", nil)
	if err == nil || err.Error() != "T3 Code didn't sign Smolcorp in: Could not open T3 Code's auth store." || strings.Contains(err.Error(), "tok") {
		t.Errorf("%v", err)
	}
	// A bearer the server refuses: said without it, and signed out all the same.
	r.fake()
	_ = os.Remove(r.calls)
	srv.mu.Lock()
	srv.deny = true
	srv.mu.Unlock()
	_, err = t3Call(r.machine(), program, "agentSessions.import", nil)
	if err == nil || err.Error() != "T3 Code's server didn't let Smolcorp in (Unauthorized)" {
		t.Errorf("%v", err)
	}
	if runs := r.runs(); len(runs) != 2 || runs[1][2] != "revoke" {
		t.Errorf("not signed out: %q", runs)
	}
}

// RM5b S1: a ping (and a pong) between two fragments of a message is answered where it comes, and the message
// comes back whole; without a ping answer, control frames come back on their own, as before.
func TestT3SocketKeepsAMessageAcrossAPingBetweenItsFragments(t *testing.T) {
	stream := func() *bufio.Reader {
		var b bytes.Buffer
		b.Write(append([]byte{0x01, 5}, "first"...))  // text, not final
		b.Write(append([]byte{0x89, 1}, 'p'))         // ping
		b.Write(append([]byte{0x8A, 1}, 'q'))         // pong
		b.Write(append([]byte{0x80, 6}, "second"...)) // continuation, final
		return bufio.NewReader(&b)
	}
	var pinged []string
	op, data, err := t3SocketReadAnswering(stream(), 1<<20, func(ping []byte) { pinged = append(pinged, string(ping)) })
	if err != nil || op != 1 || string(data) != "firstsecond" || !slices.Equal(pinged, []string{"p"}) {
		t.Errorf("%d %q %v, pinged %q", op, data, err, pinged)
	}
	if op, data, err := t3SocketRead(stream(), 1<<20); err != nil || op != 9 || string(data) != "p" {
		t.Errorf("unanswered: %d %q %v", op, data, err)
	}
	var closing bytes.Buffer
	closing.Write(append([]byte{0x01, 2}, "hi"...))
	closing.Write([]byte{0x88, 0})
	if op, _, err := t3SocketReadAnswering(bufio.NewReader(&closing), 1<<20, func([]byte) {}); err != nil || op != 8 {
		t.Errorf("a close between fragments: %d %v", op, err)
	}
}

func TestT3SocketFramesOfEverySize(t *testing.T) {
	for _, size := range []int{0, 125, 126, 65535, 65536} {
		server, client := net.Pipe()
		data := []byte(strings.Repeat("x", size))
		go func() { _ = t3SocketWrite(client, 1, data, true); client.Close() }()
		op, got, err := t3SocketRead(bufio.NewReader(server), 1<<20)
		if err != nil || op != 1 || string(got) != string(data) {
			t.Errorf("%d: %d %d %v", size, op, len(got), err)
		}
		server.Close()
	}
	// Past the limit, refused.
	server, client := net.Pipe()
	go func() { _ = t3SocketWrite(client, 1, make([]byte, 2048), false); client.Close() }()
	if _, _, err := t3SocketRead(bufio.NewReader(server), 1024); err == nil {
		t.Error("too long, read")
	}
	server.Close()
}

// MARK: Through the deck, the routes and the pages

func TestT3TakeOverAndHandBackThroughTheDeckAndItsRoutes(t *testing.T) {
	k := newT3TakeOverRig(t)
	keptMachine, keptStand := newT3Machine, t3HandingStand
	newT3Machine = k.machine
	t3HandingStand = func(*Deck, Hire) t3Handing { return k.ways() }
	t.Cleanup(func() { newT3Machine, t3HandingStand = keptMachine, keptStand })
	t3Kept.Lock()
	t3Kept.status, t3Kept.at = t3Status{}, time.Time{}
	t3Kept.Unlock()
	d := newTestDeck(t)
	d.mu.Lock()
	d.config.T3CodeEnabled = true
	d.mu.Unlock()
	t3Registered(t, k.hire)
	// Its Claude Code, as its hooks say, at a tab of its own tmux socket.
	d.Ingest(map[string]any{"hook_event_name": "SessionStart", "session_id": t3TakeOverSession, "cwd": k.hire.Worktree,
		"transcript_path": k.session.Transcript}, "claude", "tmux:%1@"+k.hire.TmuxSocket, k.hire.Worktree, nowMillis())
	if got := d.t3SessionOf(k.hire); got != k.session {
		t.Fatalf("%+v", got)
	}
	page := func(path, id string) (int, string) {
		got := serve(t, d, t3PageCall("POST", path, `{"id":"`+id+`","worktree":"/etc"}`))
		return got.Code, got.Body.String()
	}
	if status, body := page("/setup/t3code/takeover", "no-such-hire"); status != http.StatusNotFound || body != t3NotOnTeam {
		t.Errorf("%d %s", status, body)
	}
	if status, body := page("/setup/t3code/handback", k.hire.ID); status != http.StatusConflict || body != "Ada wasn't taken over from Smolcorp, so there's nothing to hand back." {
		t.Errorf("%d %s", status, body)
	}
	status, body := page("/setup/t3code/takeover", k.hire.ID)
	if status != http.StatusOK || !strings.Contains(body, `"thread":"import:claudeAgent:`+t3TakeOverSession+`"`) {
		t.Fatalf("%d %s", status, body)
	}
	// The stand-ins wrote k.saved; the registry is the deck's own, so it's written as they said.
	_ = updateRegistry(func(f *registryFile) { f.hires[0] = k.saved })
	// A T3 desk, with Hand back on its card and on its row; the team row says handBack.
	desk := t3DeskID(k.hire.ID)
	found := false
	for _, entry := range officeDesks(t, d) {
		if entry["id"] == desk {
			t3, _ := entry["t3"].(map[string]any)
			found = t3["handBack"] == true
		}
	}
	if !found {
		t.Errorf("no Hand back on its desk: %v", deskIDs(officeDesks(t, d)))
	}
	rows := t3Decorate(buildTeam(readRegistry(registryPath(), false).hires, nil, 0, func(string) string { return "" }),
		readRegistry(registryPath(), false).hires, d.t3)
	if rows[0]["handBack"] != true {
		t.Errorf("%v", rows[0])
	}
	// The desk's Hand back: the office's own window only, never a phone.
	phone := pair(t, d)
	if got := serve(t, d, call{method: "POST", target: "/act?action=handback&session=" + url.QueryEscape(desk) + "&t=" + phone, host: deckOnWiFi, from: aPhone}); got.Code != http.StatusForbidden {
		t.Errorf("from a phone: %d", got.Code)
	}
	got := serve(t, d, call{method: "POST", target: "/act?action=handback&session=" + url.QueryEscape(desk) + "&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
	if got.Code != http.StatusOK {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	if k.saved.Engine != "" || k.saved.T3Session != "" || !slices.Contains(k.log, "resume "+t3TakeOverSession) {
		t.Errorf("%+v %q", k.saved, k.log)
	}
	// RM5b S4: taken over again, then T3 Code turned off: Setup's Hand back still works, without T3 Code's
	// internal calls (its project removed with --force, then the hard check).
	k.saved = k.takenOver()
	_ = updateRegistry(func(f *registryFile) { f.hires[0] = k.saved })
	d.mu.Lock()
	d.config.T3CodeEnabled = false
	d.mu.Unlock()
	k.log = nil
	if status, body := page("/setup/t3code/handback", k.hire.ID); status != http.StatusOK {
		t.Fatalf("T3 Code off: %d %s", status, body)
	}
	if slices.Contains(k.log, "call orchestration.forceStopThread") || !slices.Contains(k.log, "resume "+t3TakeOverSession) || k.saved.T3Session != "" {
		t.Errorf("T3 Code off: %q %+v", k.log, k.saved)
	}
	if got := serve(t, d, call{method: "POST", target: "/act?action=takeover&session=nobody&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer}); got.Code != http.StatusNotFound {
		t.Errorf("no hire at that desk: %d", got.Code)
	}
}

func TestT3TakeOverRoutesAreThePagesAloneAndServedByThisAppOnly(t *testing.T) {
	for _, path := range []string{"/setup/t3code/takeover", "/setup/t3code/handback"} {
		if row := routeFor("POST", path); row == nil || !reflect.DeepEqual(row.callers, []string{"page"}) {
			t.Errorf("%s: %+v", path, row)
		}
		for _, who := range []caller{callerTool, callerGM, callerHire, callerNobody} {
			if routeAllows(who, "POST", path) {
				t.Errorf("%s: %s may", path, who.name())
			}
		}
		// Not the Swift engine's: in INVENTORY.md's Go-only table, the Mac's too since 1.25.0 (M7).
		inventory, _ := os.ReadFile(filepath.Join(contractFolder(), "INVENTORY.md"))
		if !regexp.MustCompile("(?m)^\\| POST \\| `" + regexp.QuoteMeta(path) + "` \\|.*\\| the Mac's too since 1.25.0").Match(inventory) {
			t.Errorf("%s isn't in INVENTORY.md's Go-only table", path)
		}
	}
	// The hire comes from the registry by its id, never a folder from the request.
	server, _ := os.ReadFile("server.go")
	at := strings.Index(string(server), `path == "/setup/t3code/takeover" || path == "/setup/t3code/handback"`)
	body := string(server)[at : at+500]
	if !strings.Contains(body, `id, _ := decodeJSON(r.Body)["id"].(string)`) || strings.Contains(body, "folder") || strings.Contains(body, `"worktree"`) {
		t.Errorf("%s", body)
	}
}

// t3 runs with these argument lists only, never through a shell; the calls go to T3 Code's server on this computer.
func TestT3TakeOverRunsWithItsPinnedArgumentListsOnly(t *testing.T) {
	if runs := t3RunsIn(t, "t3takeover.go", ".Run(program, "); !reflect.DeepEqual(runs, []string{
		`[]string{"project", "add", h.Worktree, "--title", title}`,
		`[]string{"project", "remove", h.Worktree, "--force"}`,
	}) {
		t.Errorf("%q", runs)
	}
	if runs := t3RunsIn(t, "t3internal.go", ".Run(program, "); !reflect.DeepEqual(runs, []string{
		`[]string{"auth", "session", "issue", "--scope", t3CallScope, "--ttl", "2m", "--label", t3CallLabel, "--json"}`,
		`[]string{"auth", "session", "revoke", id}`,
		`[]string{"auth", "session", "list", "--json"}`,
	}) {
		t.Errorf("%q", runs)
	}
	for _, file := range []string{"t3takeover.go", "t3internal.go", "t3takeover_tmux.go", "t3takeover_windows.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		code := regexp.MustCompile(`(?m)^\s*//.*$`).ReplaceAllString(string(data), "")
		for _, never := range []string{"--worktree", "os/exec", "exec.Command", "/bin/sh", "bash", `"-c"`, "send-keys", "settings.json", "secrets", "Printf", "log."} {
			if strings.Contains(code, never) {
				t.Errorf("%s: %s", file, never)
			}
		}
	}
}

// MARK: The pieces

func TestT3ResumeScriptGoesOnWithTheSessionOnItsOwnSettings(t *testing.T) {
	script, err := hireStart{Role: Role{ID: "dev", Name: "Dev", Agent: "claude", Model: "claude-haiku-5-5", Effort: "high"},
		Resume: t3TakeOverSession, DefaultAccount: true, Settings: "/s/settings.json", CLIDir: "/opt/smolcorp/bin", Ports: "4100-4103", SendKey: "k"}.startText()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "\nunset CLAUDE_CONFIG_DIR\n") ||
		!strings.HasSuffix(script, "exec 'claude' '--settings=/s/settings.json' '--model=claude-haiku-5-5' '--effort=high' '--resume' '"+t3TakeOverSession+"'\n") {
		t.Errorf("%s", script)
	}
}

func TestT3TakeOverWindowRefusalsSayNothingWasChanged(t *testing.T) {
	for _, c := range []struct {
		state    PaneState
		readable bool
		ok       bool
	}{{PaneIdle, true, true}, {PaneBusy, true, false}, {PaneDraft, true, false}, {PaneMenu, true, false}, {PaneIdle, false, false}, {PaneState(-1), true, false}} {
		why := t3WindowRefusal("Ada", c.state, c.readable)
		if (why == "") != c.ok || (!c.ok && !strings.Contains(why, "Nothing was changed") && !strings.Contains(why, "nothing was changed")) {
			t.Errorf("%v %v: %q", c.state, c.readable, why)
		}
	}
}

func TestT3NamesSessionAnywhereOnItsCommandLine(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
	}{
		{[]string{"claude", "--resume", t3TakeOverSession}, true},
		{[]string{"claude", "--resume=" + t3TakeOverSession}, true},
		{[]string{"tail", "-f", "/x/" + t3TakeOverSession + ".jsonl"}, true},
		{[]string{"claude", "--resume", "256defd6"}, false},
		{nil, false},
	} {
		if got := t3NamesSession(c.args, t3TakeOverSession); got != c.want {
			t.Errorf("%q: %v", c.args, got)
		}
	}
	if t3NamesSession([]string{"anything"}, "") {
		t.Error("an empty session names everything")
	}
}

func TestT3HoldingFindsAProcessNamingTheSession(t *testing.T) {
	if runtime.GOOS != "linux" {
		if found, looked := processesHolding(t.TempDir(), t3TakeOverSession); runtime.GOOS == "darwin" && (looked || found != nil) {
			t.Errorf("a Mac's Go core without the app can't look: %v %v", found, looked)
		}
		t.Skip("read from /proc on Linux; Windows's reader is Let go's (t3process_windows.go)")
	}
	holder := exec.Command("/bin/sh", "-c", "sleep 30; :", "sh", "--resume", t3TakeOverSession)
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Process.Kill(); _ = holder.Wait() })
	found, looked := processesHolding(t.TempDir(), t3TakeOverSession)
	if !looked || !slices.Contains(found, holder.Process.Pid) {
		t.Errorf("%v %v", found, looked)
	}
}

// MARK: TAKEOVER-2

// RM5c2 S1: a project T3 Code added but not as asked (or didn't answer about) has its thread on record before it's
// handed back, so when that Hand back stops at the project's removal, the next one still removes it.
func TestT3TakeOverAProjectAddedButNotAsAskedIsOnRecordBeforeItsHandedBack(t *testing.T) {
	removes := func(k *t3TakeOverRig) int {
		n := 0
		for _, run := range k.runs() {
			if slices.Equal(run, []string{"project", "remove", k.hire.Worktree, "--force"}) {
				n++
			}
		}
		return n
	}
	for _, how := range []string{"elsewhere", "no answer"} {
		k := newT3TakeOverRig(t, "elsewhere", "removefail")
		ways := k.ways()
		if how == "no answer" {
			run := ways.Machine.Run
			ways.Machine.Run = func(program string, args []string, within time.Duration) t3Ran {
				if slices.Equal(args[:2], []string{"project", "add"}) {
					return t3Ran{}
				}
				return run(program, args, within)
			}
		}
		_, err := t3TakeOver(k.hire, k.session, ways)
		if refused := refusalOf(t, err); !strings.Contains(refused.Message, "Then handing it back didn't finish either: T3 Code didn't remove Ada's project") {
			t.Fatalf("%s: %q", how, refused.Message)
		}
		if k.saved.T3Thread != t3ImportedThread+t3TakeOverSession || k.saved.T3Session != t3TakeOverSession {
			t.Fatalf("%s: only in memory: %+v", how, k.saved)
		}
		// T3 Code answers again: the next Hand back removes the project, then resumes.
		k.fake()
		if _, err := t3HandBack(k.saved, k.ways()); err != nil || removes(k) != 2 || k.saved.T3Thread != "" || k.saved.T3Session != "" {
			t.Errorf("%s: %v, %d removals, %+v", how, err, removes(k), k.saved)
		}
	}
}

// FXTO2 N1: when the thread can't be put on record either, the answer says the registry wasn't written, as the
// main path's does, and names the project to remove by hand.
func TestT3TakeOverSaysSoWhenTheRegistryCantHoldTheThreadOfAnUnansweredAdd(t *testing.T) {
	k := newT3TakeOverRig(t)
	ways := k.ways()
	run := ways.Machine.Run
	ways.Machine.Run = func(program string, args []string, within time.Duration) t3Ran {
		if slices.Equal(args[:2], []string{"project", "add"}) {
			return t3Ran{}
		}
		return run(program, args, within)
	}
	// The first save (the thread's turn to be marked taken over) holds; the later ones don't.
	saves := 0
	save := ways.Save
	ways.Save = func(id string, change func(*Hire)) error {
		if saves++; saves > 1 {
			return errors.New("disk full")
		}
		return save(id, change)
	}
	_, err := t3TakeOver(k.hire, k.session, ways)
	if refused := refusalOf(t, err); !strings.Contains(refused.Message, "The registry couldn't be written either: if T3 Code has a project for it, remove \"Ada · hire/ada-abcdef\" in T3 Code.") {
		t.Errorf("%q", refused.Message)
	}
}

// FXTO2 N2: a panic inside Take over leaves neither the desk nor the hire's slot held.
func TestT3TakeOverLetsGoOfItsDeskAndSlotWhenItPanics(t *testing.T) {
	k := newT3TakeOverRig(t)
	keptMachine, keptStand := newT3Machine, t3HandingStand
	newT3Machine = k.machine
	t3HandingStand = func(*Deck, Hire) t3Handing {
		ways := k.ways()
		ways.Stop = func(Hire, string) error { panic("boom") }
		return ways
	}
	t.Cleanup(func() { newT3Machine, t3HandingStand = keptMachine, keptStand })
	t3Kept.Lock()
	t3Kept.status, t3Kept.at = t3Status{}, time.Time{}
	t3Kept.Unlock()
	d := newTestDeck(t)
	d.mu.Lock()
	d.config.T3CodeEnabled = true
	d.mu.Unlock()
	t3Registered(t, k.hire)
	d.Ingest(map[string]any{"hook_event_name": "SessionStart", "session_id": t3TakeOverSession, "cwd": k.hire.Worktree,
		"transcript_path": k.session.Transcript}, "claude", "tmux:%1@"+k.hire.TmuxSocket, k.hire.Worktree, nowMillis())
	func() {
		defer func() {
			if recover() == nil {
				t.Error("no panic reached here")
			}
		}()
		d.takeOverIntoT3(k.hire.ID)
	}()
	firing.Lock()
	held := firing.takingOver[k.hire.ID]
	firing.Unlock()
	if held {
		t.Error("the slot is still held")
	}
	if !t3Desk.TryLock() {
		t.Fatal("the desk is still held")
	}
	t3Desk.Unlock()
}

// RM5c2 N1: Take over holds the hire from fires while it runs, and is refused while a fire holds it.
func TestT3TakeOverAndAFireOfOneHireNeverRunAtOnce(t *testing.T) {
	k := newT3TakeOverRig(t)
	keptMachine, keptStand := newT3Machine, t3HandingStand
	newT3Machine = k.machine
	held := false
	t3HandingStand = func(*Deck, Hire) t3Handing {
		ways := k.ways()
		stop := ways.Stop
		ways.Stop = func(h Hire, session string) error {
			firing.Lock()
			held = firing.takingOver[h.ID]
			firing.Unlock()
			return stop(h, session)
		}
		return ways
	}
	t.Cleanup(func() { newT3Machine, t3HandingStand = keptMachine, keptStand })
	t3Kept.Lock()
	t3Kept.status, t3Kept.at = t3Status{}, time.Time{}
	t3Kept.Unlock()
	d := newTestDeck(t)
	d.mu.Lock()
	d.config.T3CodeEnabled = true
	d.mu.Unlock()
	t3Registered(t, k.hire)
	d.Ingest(map[string]any{"hook_event_name": "SessionStart", "session_id": t3TakeOverSession, "cwd": k.hire.Worktree,
		"transcript_path": k.session.Transcript}, "claude", "tmux:%1@"+k.hire.TmuxSocket, k.hire.Worktree, nowMillis())
	takeOver := func() (int, string) {
		got := serve(t, d, t3PageCall("POST", "/setup/t3code/takeover", `{"id":"`+k.hire.ID+`"}`))
		return got.Code, got.Body.String()
	}
	firing.Lock()
	firing.ids[k.hire.ID] = true
	firing.Unlock()
	status, body := takeOver()
	firing.Lock()
	delete(firing.ids, k.hire.ID)
	firing.Unlock()
	if status != http.StatusConflict || body != "Ada is being fired right now, so it wasn't taken over." || len(k.log) != 0 {
		t.Errorf("while it's fired: %d %s %q", status, body, k.log)
	}
	if status, body := takeOver(); status != http.StatusOK || !held {
		t.Errorf("held %v: %d %s", held, status, body)
	}
	firing.Lock()
	defer firing.Unlock()
	if firing.takingOver[k.hire.ID] || firing.ids[k.hire.ID] {
		t.Error("still held once Take over was done")
	}
}

// FXM5c: the resumed hire's tmux socket, as tmux spells it, goes in Hand back's last save, which is tried again
// and said when it can't be written; a resume that gives none leaves the registry's.
func TestT3HandBackKeepsTheResumedSocketInItsCheckedSave(t *testing.T) {
	k := newT3TakeOverRig(t)
	h := k.takenOver()
	k.socket = "/private" + k.hire.TmuxSocket
	if _, err := t3HandBack(h, k.ways()); err != nil || k.saved.TmuxSocket != k.socket || k.saved.T3Session != "" {
		t.Errorf("%v %+v", err, k.saved)
	}
	k = newT3TakeOverRig(t)
	h = k.takenOver()
	if _, err := t3HandBack(h, k.ways()); err != nil || k.saved.TmuxSocket != k.hire.TmuxSocket {
		t.Errorf("%v %+v", err, k.saved)
	}
}

// RM5b N4, for RM5c2 N1/N2: Take over's and Hand back's saves change only an open row; one fired or failed meanwhile,
// or gone, is an error and stays as it is.
func TestT3SavesChangeOnlyAnOpenRow(t *testing.T) {
	isolate(t)
	open := Hire{ID: "abcdef123456", Name: "Ada", Folder: "/w", State: "working"}
	fired := Hire{ID: "fedcba654321", Name: "Bo", Folder: "/w", State: "fired"}
	t3Registered(t, open, fired)
	mark := func(r *Hire) { r.T3Session = t3TakeOverSession }
	if err := t3SaveOpen(open.ID, mark); err != nil {
		t.Error(err)
	}
	if t3SaveOpen(fired.ID, mark) == nil || t3SaveOpen("nobody", mark) == nil {
		t.Error("a row that isn't open was saved")
	}
	if h, _ := registryHire(open.ID); h.T3Session != t3TakeOverSession {
		t.Errorf("%+v", h)
	}
	if h, _ := registryHire(fired.ID); h.T3Session != "" {
		t.Errorf("%+v", h)
	}
}

// RMSY N1: a start script the core takes away at its start was never run. A hire still starting fails, as before;
// a hire handed back from T3 Code, whose resume's script it was, fails too, its notes kept after the reason. Any
// other hire's plain script leaves its row as it is, and so does a fired hire's resume script.
func TestT3AHandedBackHireWhoseResumeScriptWasTakenAwayFails(t *testing.T) {
	isolate(t)
	handedBack := Hire{ID: "abcdef123456", Name: "Ada", Folder: "/w", State: "working", Notes: "Owner's note."}
	starting := Hire{ID: "fedcba654321", Name: "Bo", Folder: "/w", State: "starting"}
	fired := Hire{ID: "aaaaaa111111", Name: "Cy", Folder: "/w", State: "fired"}
	working := Hire{ID: "bbbbbb222222", Name: "Di", Folder: "/w", State: "working"}
	t3Registered(t, handedBack, starting, fired, working)
	for id, name := range map[string]string{handedBack.ID: resumeScriptPrefix + "1.sh", starting.ID: "hire-1.sh", fired.ID: resumeScriptPrefix + "2.sh", working.ID: "hire-2.sh"} {
		folder := hireFolder(supportDir, id)
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, name), []byte("#!/bin/sh\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	removeLeftStartScripts()
	if h, _ := registryHire(handedBack.ID); h.State != "failed" || h.Notes != resumeScriptNotRun+"\nOwner's note." {
		t.Errorf("handed back: %+v", h)
	}
	if h, _ := registryHire(starting.ID); h.State != "failed" || h.Notes != startScriptNotRun {
		t.Errorf("starting: %+v", h)
	}
	for _, kept := range []Hire{fired, working} {
		if h, _ := registryHire(kept.ID); h.State != kept.State || h.Notes != "" {
			t.Errorf("%+v", h)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(supportDir, "hires", "*", "hire-*.sh")); len(left) != 0 {
		t.Errorf("%q", left)
	}
}

// RM5b N1: a sign-in whose answer is lost (t3 didn't end in time) or can't be read is signed out at once: every
// session T3 Code lists under Smolcorp's label, and no other.
func TestT3CallSignsOutASignInItCouldntRead(t *testing.T) {
	revoked := func(r *t3Rig) [][]string {
		var out [][]string
		for _, run := range r.runs() {
			if len(run) > 2 && run[2] == "revoke" {
				out = append(out, run)
			}
		}
		return out
	}
	r := newT3Rig(t)
	r.fake("authgarbled")
	newT3FakeServer(t, r.home)
	program := t3Program(r.machine())
	if _, err := t3Call(r.machine(), program, "agentSessions.import", nil); err == nil || err.Error() != "T3 Code didn't sign Smolcorp in the way Smolcorp knows" {
		t.Errorf("%v", err)
	}
	if got := revoked(r); !reflect.DeepEqual(got, [][]string{{"auth", "session", "revoke", "s-9"}}) {
		t.Errorf("unreadable: %q", r.runs())
	}
	r.fake()
	_ = os.Remove(r.calls)
	machine := r.machine()
	machine.Run = func(program string, args []string, within time.Duration) t3Ran {
		if slices.Contains(args, "issue") {
			return t3Ran{}
		}
		return runT3(program, args, within)
	}
	if _, err := t3Call(machine, program, "agentSessions.import", nil); err == nil || !strings.HasPrefix(err.Error(), "T3 Code didn't sign Smolcorp in") {
		t.Errorf("%v", err)
	}
	if got := revoked(r); !reflect.DeepEqual(got, [][]string{{"auth", "session", "revoke", "s-9"}}) {
		t.Errorf("no answer: %q", r.runs())
	}
	// A sign-in that ended with a failure may have issued all the same (the fake prints a session first): what's
	// listed as Smolcorp's is signed out, and nobody else's (FXTO2 N3).
	r.fake("authfail")
	_ = os.Remove(r.calls)
	_, _ = t3Call(r.machine(), program, "agentSessions.import", nil)
	if got := revoked(r); !reflect.DeepEqual(got, [][]string{{"auth", "session", "revoke", "s-9"}}) {
		t.Errorf("refused: %q", r.runs())
	}
}

// RM5b N3: a control frame that's fragmented or over 125 bytes is refused before it's read, answered or not.
func TestT3SocketRefusesAControlFrameThatIsntOne(t *testing.T) {
	for name, frame := range map[string][]byte{
		"fragmented ping": append([]byte{0x09, 1}, 'p'),
		"long ping":       append([]byte{0x89, 126, 0, 126}, bytes.Repeat([]byte("p"), 126)...),
		"long close":      append([]byte{0x88, 126, 0, 200}, bytes.Repeat([]byte("c"), 200)...),
	} {
		var b bytes.Buffer
		b.Write(append([]byte{0x01, 2}, "hi"...))
		b.Write(frame)
		b.Write(append([]byte{0x80, 2}, "!!"...))
		pinged := 0
		if _, _, err := t3SocketReadAnswering(bufio.NewReader(&b), 1<<20, func([]byte) { pinged++ }); err == nil || pinged != 0 {
			t.Errorf("%s: %v, pinged %d", name, err, pinged)
		}
		if _, _, err := t3SocketRead(bufio.NewReader(bytes.NewReader(frame)), 1<<20); err == nil {
			t.Errorf("%s, on its own: read", name)
		}
	}
	// 125 bytes is a ping still.
	ping := append([]byte{0x89, 125}, bytes.Repeat([]byte("p"), 125)...)
	if op, data, err := t3SocketRead(bufio.NewReader(bytes.NewReader(ping)), 1<<20); err != nil || op != 9 || len(data) != 125 {
		t.Errorf("%d %d %v", op, len(data), err)
	}
}
