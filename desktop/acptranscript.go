package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// An ACP desk's transcript (ACP1): what was asked and what the agent said and did, written as Claude Code writes its
// own, a line of JSON per message, so the conversation (conversation.go) and Summary read it as they read Claude
// Code's. The agent writes nothing itself: the desk writes every line, from the prompts it sent and the updates it
// was given.
//
// Each desk's is a file of its own in the support folder's acpTranscriptFolder, named by the core (never by the
// agent's session id), private to the owner (0700 folder, 0600 file). It holds what was said, not keys: in the
// support folder's inventory it is "not secret", and it grants nothing.

// acpTranscriptFolder is the support folder's folder of ACP transcripts.
const acpTranscriptFolder = "acp-transcripts"

const (
	// A message of the agent's, at most, before the rest is left out: chunks keep coming until the turn ends.
	acpMessageMost = 256 * 1024
	// A tool's result, and its input as JSON, at most: the conversation shows only the first line of a result.
	acpToolTextMost = 64 * 1024
)

var errACPTranscriptFolder = errors.New("the ACP transcripts folder isn't a folder of its own")

// acpTranscript is one desk's transcript, written by its desk's worker alone. A nil one writes nothing.
type acpTranscript struct {
	path    string
	file    *os.File
	session string
	// The agent's message so far: its chunks, written as one message when something else happens.
	message strings.Builder
	cut     bool
	broken  error // a write failed: nothing more is written
}

// newACPTranscript makes a new, empty transcript for session in support's folder of them.
func newACPTranscript(support, session string) (*acpTranscript, error) {
	folder := filepath.Join(support, acpTranscriptFolder)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return nil, err
	}
	// The folder itself, never a link to somewhere else.
	if info, err := os.Lstat(folder); err != nil || !info.IsDir() {
		return nil, errACPTranscriptFolder
	}
	if err := os.Chmod(folder, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(folder, randomHex(16)+".jsonl")
	// O_EXCL: a new file, never one (or a link) already there.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, err
	}
	return &acpTranscript{path: path, file: file, session: session}, nil
}

// user is a prompt sent, as the owner's own words.
func (t *acpTranscript) user(text string, at time.Time) {
	t.flush(at)
	t.write(map[string]any{"type": "user", "origin": map[string]any{"kind": "human"},
		"message": map[string]any{"role": "user", "content": text}}, at)
}

// said is a chunk of the agent's message.
func (t *acpTranscript) said(chunk string) {
	if t == nil || t.cut {
		return
	}
	if room := acpMessageMost - t.message.Len(); len(chunk) > room {
		chunk, t.cut = clipBytes(chunk, room), true
	}
	t.message.WriteString(chunk)
}

// flush writes the agent's message so far, if there is one.
func (t *acpTranscript) flush(at time.Time) {
	if t == nil {
		return
	}
	text := t.message.String()
	if t.cut {
		text += " …"
	}
	t.message.Reset()
	t.cut = false
	if strings.TrimSpace(text) == "" {
		return
	}
	t.write(map[string]any{"type": "assistant",
		"message": map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": text}}}}, at)
}

// toolUse is a tool call starting: the tool, by Claude Code's name for it, and what it was given, as an object.
func (t *acpTranscript) toolUse(id, name string, input json.RawMessage, at time.Time) {
	t.flush(at)
	var given any = map[string]any{}
	if len(input) <= acpToolTextMost && len(input) > 0 && input[0] == '{' {
		given = input
	}
	t.write(map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant",
		"content": []any{map[string]any{"type": "tool_use", "id": id, "name": name, "input": given}}}}, at)
}

// toolResult is a tool call's end: its text, and whether it failed.
func (t *acpTranscript) toolResult(id, text string, failed bool, at time.Time) {
	t.flush(at)
	if len(text) > acpToolTextMost {
		text = clipBytes(text, acpToolTextMost) + " …"
	}
	t.write(map[string]any{"type": "user", "message": map[string]any{"role": "user",
		"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "content": text, "is_error": failed}}}}, at)
}

// close writes what's left of the agent's message and closes the file.
func (t *acpTranscript) close(at time.Time) {
	if t == nil {
		return
	}
	t.flush(at)
	t.file.Close()
}

// write adds one line: the entry, with the session, the time and an id of its own.
func (t *acpTranscript) write(entry map[string]any, at time.Time) {
	if t == nil || t.broken != nil {
		return
	}
	entry["sessionId"], entry["timestamp"], entry["uuid"] = t.session, at.UTC().Format(time.RFC3339Nano), randomHex(16)
	line, err := json.Marshal(entry)
	if err == nil {
		// One write per line, in append mode: a reader never sees half of one followed by another.
		_, err = t.file.Write(append(line, '\n'))
	}
	t.broken = err
}

// clipBytes is s cut to at most n bytes, never inside a character.
func clipBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
