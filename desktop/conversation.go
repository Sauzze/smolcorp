package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ▤ Conversation, in the office's own window: an agent's conversation, read
// from the same Claude Code transcript ✨ Summary reads, as the Mac's
// Conversation.swift read it until M7: what the user wrote, what the agent
// answered, each tool it ran in a line, a result's first line, and the
// questions it asked with their answers.
//
// Read from the end, a page at a time, each message cut to
// conversationTextMost, and nothing of a tool's input or output beyond what
// Summary takes. Only the office's own window on this computer reads it (see
// the router), where nothing leaves the computer, so it's shown as it was
// written, as the Mac's window had it. A redacted reading for a phone isn't
// served: nothing here sends a conversation off this computer.

const (
	// Messages in a page, at most.
	conversationPageMost = 60
	// Each message's text, at most, before "… more".
	conversationTextMost = 2_000
	// How much of the file one page looks through, at most: tool results are
	// kept whole in a transcript, and can be large.
	conversationReach int64 = 4 * 1024 * 1024
	// A line longer than this is a tool's whole output: skipped, not read.
	conversationLineMost = 2 * 1024 * 1024
	// How much is read at a time, going back.
	conversationChunk int64 = 256 * 1024
)

// conversationItem is one thing said or done: user, peer, agent, tool,
// result, question, answer or note. Its id is where its line starts in the
// file and which of the line's items it is, so it stays put as the file grows.
type conversationItem struct {
	ID     string `json:"id"`
	Offset int64  `json:"offset"`
	N      int    `json:"n"`
	Kind   string `json:"kind"`
	Text   string `json:"text"`
	// Cut at conversationTextMost.
	More bool `json:"more"`
	// When it was written, in ms, if the transcript says.
	At *int64 `json:"at,omitempty"`
	// Who sent a peer's message, or the tool a line is about.
	From string `json:"from,omitempty"`
}

// conversationPage is a page, oldest first; where the next older page ends
// (null at the start of the file); and the file's size as it was read, to
// tell whether it has grown.
type conversationPage struct {
	Items  []conversationItem `json:"items"`
	Before *int64             `json:"before"`
	Size   int64              `json:"size"`
}

// openRegular opens a regular file only: a pipe, a socket or a device could block the read for ever, and
// with it everything that waits behind it. The file is checked before it is opened, and once more as opened.
func openRegular(path string) (*os.File, os.FileInfo, error) {
	if info, err := os.Stat(path); err != nil {
		return nil, nil, err
	} else if !info.Mode().IsRegular() {
		return nil, nil, errNotRegular
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, nil, errNotRegular
	}
	return file, info, nil
}

var errNotRegular = errors.New("not a regular file")

// readRegular is os.ReadFile for a regular file only.
func readRegular(path string) ([]byte, error) {
	file, _, err := openRegular(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

// conversationReads is where conversations are read: one at a time, as the
// Mac's own queue read them.
var conversationReads sync.Mutex

// readConversation is the page of the transcript at path that ends where
// before is (a line's start, from an earlier page), or at the end. False with
// no file.
func readConversation(path string, before *int64, most int) (conversationPage, bool) {
	if path == "" {
		return conversationPage{}, false
	}
	file, info, err := openRegular(path)
	if err != nil {
		return conversationPage{}, false
	}
	defer file.Close()
	size := info.Size()
	end := size
	if before != nil && *before < size {
		end = *before
	}
	stop := int64(0)
	if end > conversationReach {
		stop = end - conversationReach
	}

	// Back from end, a chunk at a time, whole lines only, newest first, till the page is full or the reach is spent.
	type line struct {
		offset int64
		entry  map[string]any
		count  int
	}
	var lines []line
	counted := 0
	position := end    // bytes from here to end have been looked at
	var pending []byte // the line position is in, as far as it's been read
	tooLong := false   // that line is longer than conversationLineMost: skipped
	earliest := end    // where the earliest line looked at starts
	take := func(data []byte, at int64) {
		earliest = at
		var entry map[string]any
		if len(data) == 0 || json.Unmarshal(data, &entry) != nil || entry == nil {
			return
		}
		count := len(conversationItems(entry, at, map[string]string{}))
		if count == 0 {
			return
		}
		lines = append(lines, line{at, entry, count})
		counted += count
	}
scan:
	for position > 0 {
		from := max(0, position-conversationChunk)
		data := make([]byte, position-from)
		if n, _ := file.ReadAt(data, from); n < len(data) {
			break
		}
		for {
			newline := bytes.LastIndexByte(data, '\n')
			if newline < 0 {
				break
			}
			start := from + int64(newline) + 1
			if length := len(data) - newline - 1 + len(pending); !tooLong && length <= conversationLineMost {
				take(append(append([]byte{}, data[newline+1:]...), pending...), start)
			} else {
				earliest = start
			}
			pending, tooLong = nil, false
			data = data[:newline]
			if counted >= most || start <= stop {
				break scan
			}
		}
		if !tooLong {
			pending = append(append([]byte{}, data...), pending...)
			if len(pending) > conversationLineMost {
				pending, tooLong = nil, true
			}
		}
		position = from
		if tooLong && position <= stop {
			// The reach is spent inside a line that is skipped anyway: the next page goes on from here.
			earliest = position
			break
		}
		if from == 0 {
			if !tooLong {
				take(pending, 0)
			} else {
				earliest = 0
			}
		}
	}

	// Oldest first, and no more than most: the oldest lines go first, but never the only one.
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	start := earliest
	for counted > most && len(lines) > 1 {
		counted -= lines[0].count
		lines = lines[1:]
		start = lines[0].offset
	}
	uses := map[string]string{} // a tool use's id, its tool: for a question's answer
	page := conversationPage{Items: []conversationItem{}, Size: size}
	for _, l := range lines {
		page.Items = append(page.Items, conversationItems(l.entry, l.offset, uses)...)
	}
	if start != 0 {
		page.Before = &start
	}
	return page, true
}

// conversationItems is one transcript line's items, oldest first, each cut.
func conversationItems(entry map[string]any, offset int64, uses map[string]string) []conversationItem {
	if entry["isSidechain"] == true {
		return nil
	}
	var at *int64
	if text, ok := entry["timestamp"].(string); ok {
		if when, err := time.Parse(time.RFC3339Nano, text); err == nil {
			ms := when.UnixMilli()
			at = &ms
		}
	}
	var items []conversationItem
	add := func(kind, text, from string) {
		if text = strings.TrimSpace(text); text != "" {
			items = append(items, conversationItemOf(kind, text, offset, len(items), at, from))
		}
	}

	// What the user wrote, or another session sent: Summary's own reading of it.
	if message, ok := messageOf(entry); ok {
		if message.Kind == fromUser {
			add("user", message.Text, "")
		} else {
			add("peer", message.Text, message.From)
		}
		return items
	}
	kind, _ := entry["type"].(string)
	message, ok := entry["message"].(map[string]any)
	if kind != "user" && kind != "assistant" || !ok {
		return nil
	}
	said := func(text string) {
		if kind == "assistant" {
			add("agent", text, "")
		} else if note, ok := conversationNote(text); ok {
			add("note", note, "")
		}
	}
	if text, ok := message["content"].(string); ok {
		said(text)
		return items
	}
	blocks, _ := message["content"].([]any)
	for _, b := range blocks {
		block, _ := b.(map[string]any)
		switch block["type"] {
		case "text":
			text, _ := block["text"].(string)
			said(text)
		case "tool_use":
			name, ok := block["name"].(string)
			if !ok {
				name = "tool"
			}
			if id, ok := block["id"].(string); ok {
				uses[id] = name
			}
			if name == "AskUserQuestion" {
				input, _ := block["input"].(map[string]any)
				if asked, ok := conversationQuestion(input); ok {
					add("question", asked, "")
					continue
				}
			}
			// Summary's line for it: the tool, and its path, command or search, never what it was given to write.
			add("tool", toolLine(block), name)
		case "tool_result":
			first := firstLine(resultText(block["content"]))
			id, _ := block["tool_use_id"].(string)
			tool := uses[id]
			if tool == "AskUserQuestion" {
				add("answer", conversationAnswer(first), "")
				continue
			}
			if first == "" {
				first = "(nothing)"
			}
			text := clip(first, summaryResultMost)
			if block["is_error"] == true {
				text = "Error: " + text
			}
			add("result", text, tool)
		}
		// Thinking, images, and anything new: left out.
	}
	return items
}

// conversationItemOf is an item, cut at conversationTextMost, with who it's from in a short line.
func conversationItemOf(kind, text string, offset int64, index int, at *int64, from string) conversationItem {
	// A photo sent to the desk (photo.go) is said as one, never by its path.
	text = hidePhotos(text)
	cut := utf8.RuneCountInString(text) > conversationTextMost
	if cut {
		text = string([]rune(text)[:conversationTextMost])
	}
	if from != "" {
		from = clip(firstLine(from), 60)
	}
	return conversationItem{ID: strconv.FormatInt(offset, 10) + "." + strconv.Itoa(index), Offset: offset, N: index, Kind: kind,
		Text: text, More: cut, At: at, From: from}
}

// conversationNote is what's left in the user's turn that isn't a message: a command run, or an interruption.
func conversationNote(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "<command-name>") {
		if close := strings.Index(text, "</command-name>"); close >= len("<command-name>") {
			if name := text[len("<command-name>"):close]; name != "" {
				return "ran " + clip(redactText(name), 60), true
			}
		}
		return "", false
	}
	if strings.HasPrefix(text, "[Request interrupted") {
		return "interrupted", true
	}
	return "", false
}

// conversationQuestion is AskUserQuestion's questions, each with its options' labels.
func conversationQuestion(input map[string]any) (string, bool) {
	asked, _ := input["questions"].([]any)
	var questions []string
	for _, q := range asked {
		question, _ := q.(map[string]any)
		text, _ := question["question"].(string)
		if text == "" {
			continue
		}
		options, _ := question["options"].([]any)
		var labels []string
		for _, o := range options {
			option, _ := o.(map[string]any)
			if label, ok := option["label"].(string); ok {
				labels = append(labels, strconv.Itoa(len(labels)+1)+". "+label)
			}
		}
		if len(labels) > 0 {
			text += "\n" + strings.Join(labels, "\n")
		}
		questions = append(questions, text)
	}
	return strings.Join(questions, "\n\n"), len(questions) > 0
}

// conversationAnswer is a question's answer, from its result's first line, without Claude Code's words round it.
func conversationAnswer(first string) string {
	text := first
	for _, prefix := range []string{"User has answered your questions:", "User has answered your question:"} {
		text = strings.TrimPrefix(text, prefix)
	}
	if cut := strings.Index(text, ". You can now continue"); cut >= 0 {
		text = text[:cut]
	}
	return clip(strings.Trim(text, " \t"), summaryTextMost)
}

// ▤ Conversation (phoneroutes.go), in the office's own window on this computer, never a phone: the session's
// conversation, a page at a time, as it was written.
func init() {
	registerPhoneRoute(phoneRoute{method: http.MethodGet, path: "/conversation", access: accessWindow,
		refusal: func(d *Deck) string { return d.shownOnlyInTheWindow("The conversation") },
		serve: func(d *Deck, w http.ResponseWriter, r *request, session string) {
			before, placed := conversationPlace(r.URL.Query(), "before")
			known, sized := conversationPlace(r.URL.Query(), "size")
			if !placed || !sized {
				writeStatus(w)(http.StatusBadRequest, "That isn't a place in the conversation.")
				return
			}
			writeAnswer(w)(d.conversation(session, before, known))
		}})
}

// conversationPlace reads a page's place from the query, ?before= or ?size=:
// none is nil, digits only else not ok.
func conversationPlace(query map[string][]string, name string) (*int64, bool) {
	values, given := query[name]
	if !given {
		return nil, true
	}
	text := ""
	if len(values) > 0 {
		text = values[0]
	}
	if text == "" || len(text) > 20 || strings.Trim(text, "0123456789") != "" {
		return nil, false
	}
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return nil, false
	}
	place := int64(min(value, math.MaxInt64))
	return &place, true
}

// conversation answers GET /conversation: a page of session id's transcript
// ending at before, or its end. With known, the size the window last saw of it,
// the end isn't read again while the file hasn't grown (204).
func (d *Deck) conversation(id string, before, known *int64) (int, any) {
	d.mu.Lock()
	session := d.sessions.Get(id)
	path := ""
	if session != nil {
		path = session.Transcript
	}
	d.mu.Unlock()
	if session == nil {
		return http.StatusNotFound, "That agent has gone."
	}
	if path == "" {
		return http.StatusNotFound, "No conversation for this agent."
	}
	conversationReads.Lock()
	defer conversationReads.Unlock()
	if info, err := os.Stat(path); err == nil && before == nil && known != nil && *known == info.Size() {
		return http.StatusNoContent, ""
	}
	page, ok := readConversation(path, before, conversationPageMost)
	if !ok {
		return http.StatusNotFound, "No conversation for this agent."
	}
	return http.StatusOK, page
}

// shownOnlyInTheWindow is why the conversation, or the info, is refused: they show in the office's own window alone.
func (d *Deck) shownOnlyInTheWindow(what string) string {
	return what + " shows only in the office's window on your " + d.machine() + "."
}
