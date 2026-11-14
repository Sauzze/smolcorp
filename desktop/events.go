package desktop

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// The office's record of what its sessions did, as the Mac app's EventLog: one
// JSON line per event in events.jsonl in the support folder, readable by you
// alone. The GM reads it through GET /setup/gm/events, and it outlasts
// restarts. Only the kinds below are kept, and of each only who, where, when and a
// thin detail: never a prompt, a reply, a tool's output or a command's
// arguments. Lines are numbered in the order they are added, across restarts.
// Past maxBytes the file becomes events.1.jsonl (the one before is dropped), so
// the newest are always kept.

const (
	eventDetailLimit = 200
	// How many of the latest lines are held in memory.
	eventsInMemory = 2000
	// The most one answer holds, the default, and the longest a request may wait.
	eventsLimitMost    = 1000
	eventsLimitDefault = 200
	eventsWaitMost     = 30 * time.Second
	eventsFileMost     = 10 << 20

	eventLimitDetail = "usage limit"

	// eventKindMail is a line in the GM's inbox: a hire wrote to the GM (the Mac's EventLog.Kind.mail).
	eventKindMail = "mail"
)

// eventEntry is one line. Its fields are in the order the Mac writes them
// (sorted by name).
type eventEntry struct {
	Agent   string `json:"agent"`
	At      int64  `json:"at"` // milliseconds since 1970
	Detail  string `json:"detail"`
	Folder  string `json:"folder"`
	ID      int64  `json:"id"`
	Kind    string `json:"kind"`
	Session string `json:"session"`
}

type eventLog struct {
	mu       sync.Mutex
	file     string
	earlier  string
	maxBytes int
	lastID   int64
	// The first line still kept, or the next one when there is none.
	oldestID  int64
	liveFirst int64
	liveBytes int
	recent    []eventEntry
	waiters   map[*chan struct{}]int64 // each waiting request, with the id it has seen
}

func newEventLog(folder string, maxBytes int) *eventLog {
	_ = os.MkdirAll(folder, 0o700)
	l := &eventLog{
		file: filepath.Join(folder, "events.jsonl"), earlier: filepath.Join(folder, "events.1.jsonl"),
		maxBytes: maxBytes, waiters: map[*chan struct{}]int64{},
	}
	before, live := readEventFile(l.earlier), readEventFile(l.file)
	kept := append(before, live...)
	if len(kept) > eventsInMemory {
		l.recent = append(l.recent, kept[len(kept)-eventsInMemory:]...)
	} else {
		l.recent = kept
	}
	if len(kept) > 0 {
		l.lastID, l.oldestID = kept[len(kept)-1].ID, kept[0].ID
	} else {
		l.oldestID = 1
	}
	l.liveFirst = l.lastID + 1
	if len(live) > 0 {
		l.liveFirst = live[0].ID
	}
	if info, err := os.Stat(l.file); err == nil {
		l.liveBytes = int(info.Size())
	}
	return l
}

// readEventFile reads the lines of file, skipping any that aren't one.
func readEventFile(file string) []eventEntry {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var found []eventEntry
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		var e eventEntry
		if json.Unmarshal(scanner.Bytes(), &e) == nil && e.ID > 0 {
			found = append(found, e)
		}
	}
	return found
}

// append adds e as the next line and wakes anyone waiting for it.
func (l *eventLog) append(e eventEntry) eventEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lastID++
	e.ID = l.lastID
	e.Detail = cutEventDetail(e.Detail)
	l.recent = append(l.recent, e)
	if len(l.recent) > eventsInMemory {
		l.recent = l.recent[len(l.recent)-eventsInMemory:]
	}
	line, err := json.Marshal(e)
	if err != nil {
		return e
	}
	line = append(line, '\n')
	l.liveBytes += len(line)
	rotate := l.liveBytes >= l.maxBytes
	if f, err := os.OpenFile(l.file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
		_, _ = f.Write(line)
		_ = f.Close()
	}
	if rotate {
		l.oldestID, l.liveFirst, l.liveBytes = l.liveFirst, e.ID+1, 0
		kept := l.recent[:0]
		for _, r := range l.recent {
			if r.ID >= l.oldestID {
				kept = append(kept, r)
			}
		}
		l.recent = kept
		_ = os.Remove(l.earlier)
		_ = os.Rename(l.file, l.earlier)
	}
	for wake, since := range l.waiters {
		if since < e.ID {
			delete(l.waiters, wake)
			close(*wake)
		}
	}
	return e
}

// entries are the lines after since that memory holds, at most limit of them;
// the since to ask with next; and the first line still kept. A since past the
// last line (the log was reset) answers with none and next = the last line.
// With l.mu held.
func (l *eventLog) entries(since int64, limit int) ([]eventEntry, int64, int64) {
	if since >= l.lastID {
		return []eventEntry{}, l.lastID, l.oldestID
	}
	found := []eventEntry{}
	for _, e := range l.recent {
		if e.ID > since && len(found) < max(1, limit) {
			found = append(found, e)
		}
	}
	next := max(since, 0)
	if len(found) > 0 {
		next = found[len(found)-1].ID
	}
	return found, next, l.oldestID
}

// snapshot is the lines held in memory, oldest first, for what follows the log.
func (l *eventLog) snapshot() []eventEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]eventEntry(nil), l.recent...)
}

// wait answers with the lines after since as soon as there are any, or with none
// after timeout (at most eventsWaitMost) or when ctx ends.
func (l *eventLog) wait(ctx context.Context, since int64, limit int, timeout time.Duration) []eventEntry {
	l.mu.Lock()
	if since > l.lastID {
		l.mu.Unlock()
		return []eventEntry{}
	}
	// Older than memory holds: from the files.
	if since < l.lastID && len(l.recent) > 0 && since < l.recent[0].ID-1 {
		l.mu.Unlock()
		found := []eventEntry{}
		for _, e := range append(readEventFile(l.earlier), readEventFile(l.file)...) {
			if e.ID > since && len(found) < max(1, limit) {
				found = append(found, e)
			}
		}
		return found
	}
	now, _, _ := l.entries(since, limit)
	if len(now) > 0 || timeout <= 0 {
		l.mu.Unlock()
		return now
	}
	wake := make(chan struct{})
	l.waiters[&wake] = since
	l.mu.Unlock()
	timer := time.NewTimer(min(timeout, eventsWaitMost))
	defer timer.Stop()
	select {
	case <-wake:
	case <-timer.C:
	case <-ctx.Done():
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.waiters, &wake)
	found, _, _ := l.entries(since, limit)
	return found
}

// answer is the body of GET /setup/gm/events.
func (l *eventLog) answer(found []eventEntry, since int64) map[string]any {
	l.mu.Lock()
	defer l.mu.Unlock()
	next := min(since, l.lastID)
	if len(found) > 0 {
		next = found[len(found)-1].ID
	}
	return map[string]any{"events": found, "next": next, "oldest": l.oldestID}
}

// eventQuestion is what GET /setup/gm/events was asked, each kept to what is allowed.
func eventQuestion(query url.Values) (since int64, wait time.Duration, limit int) {
	since, _ = strconv.ParseInt(query.Get("since"), 10, 64)
	since = max(0, since)
	seconds, err := strconv.ParseFloat(query.Get("wait"), 64)
	if err != nil || math.IsNaN(seconds) {
		seconds = 0
	}
	wait = time.Duration(min(eventsWaitMost.Seconds(), max(0, seconds)) * float64(time.Second))
	limit = eventsLimitDefault
	if n, err := strconv.Atoi(query.Get("limit")); err == nil {
		limit = min(eventsLimitMost, max(1, n))
	}
	return
}

// hookEventKinds are the hook events that are kept, and what each is logged as.
var hookEventKinds = map[string]string{
	"SessionStart": "sessionStart", "SessionEnd": "sessionStop", "UserPromptSubmit": "prompt",
	"PermissionRequest": "permissionAsked", "PermissionDenied": "permissionDenied",
	"Notification": "notification", "Stop": "idle",
}

// eventFor is the line for a hook event of a session at folder, run by agent, or
// false when it is not one of the kinds kept.
func eventFor(event map[string]any, agent, folder string, at float64) (eventEntry, bool) {
	name, _ := event["hook_event_name"].(string)
	kind, ok := hookEventKinds[name]
	if !ok {
		return eventEntry{}, false
	}
	session, _ := event["session_id"].(string)
	return eventEntry{Agent: agent, At: int64(math.Round(at)), Detail: hidePhotos(eventDetail(event, kind)), Folder: folder,
		Kind: kind, Session: session}, true
}

// eventDetail is what is worth knowing about event beyond its kind: for a
// permission the tool, and for Bash only the command's first word, for a file
// tool the file; for a notification its message only in Claude Code's own
// wording; why a session started or ended.
func eventDetail(event map[string]any, kind string) string {
	str := func(v any) string { s, _ := v.(string); return s }
	switch kind {
	case "permissionAsked", "permissionDenied":
		tool := str(event["tool_name"])
		input, _ := event["tool_input"].(map[string]any)
		what := ""
		if tool == "Bash" {
			what = firstWordOf(str(input["command"]))
		} else {
			for _, field := range []string{"file_path", "notebook_path"} {
				if s := str(input[field]); s != "" {
					what = s
					break
				}
			}
		}
		switch {
		case what == "":
			return cutEventDetail(tool)
		case tool == "":
			return cutEventDetail(what)
		}
		return cutEventDetail(tool + ": " + what)
	case "notification":
		message := str(event["message"])
		if eventLimitWording.MatchString(message) {
			return eventLimitDetail
		}
		if message == "Claude is waiting for your input" || eventOwnPermission.MatchString(message) {
			return cutEventDetail(message)
		}
		return "notification"
	case "sessionStart":
		return cutEventDetail(str(event["source"]))
	case "sessionStop":
		return cutEventDetail(str(event["reason"]))
	}
	return ""
}

var (
	eventOwnPermission = regexp.MustCompile(`^Claude needs your permission to use [A-Za-z0-9_.:\-]+$`)
	eventLimitWording  = regexp.MustCompile(`(?i)(usage|rate|session|weekly|5-hour|daily) limit|hit your limit|limit reached`)
)

// firstWordOf is the program a shell command runs: its first word, past any
// NAME=value set before it (those can hold a key).
func firstWordOf(command string) string {
	for _, word := range strings.Fields(command) {
		if !strings.Contains(word, "=") {
			return word
		}
	}
	return ""
}

// cutEventDetail puts text on one line, at most eventDetailLimit characters.
func cutEventDetail(text string) string {
	line := strings.Join(strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' || r == ' ' || r == ' ' }), " ")
	if utf8.RuneCountInString(line) > eventDetailLimit {
		return string([]rune(line)[:eventDetailLimit-1]) + "…"
	}
	return line
}
