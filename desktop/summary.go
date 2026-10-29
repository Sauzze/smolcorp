package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/zalando/go-keyring"
)

// errNoSecretStore is a system with no secret store to keep a key in.
var errNoSecretStore = errors.New("no secret store")

// ✨ Summary: what an agent has been doing, in a few lines written by Claude
// Haiku from the end of its Claude Code transcript, for a phone that can't
// see the terminal. Off until it's turned on in setup, and made with your own
// Anthropic API key, billed to you, as the Mac app's Summary.swift does.
//
// Only the tail of the transcript is read, tool output cut down to a line,
// lines that look like secrets taken out, and at most summaryBudget characters
// sent, and only when a summary is asked for. The key is read as each call is
// made, held in memory only for it, and never written anywhere else, logged
// or answered.

const (
	// How many characters are sent at most, and how much of the file is read from its end.
	summaryBudget = 16_000
	summaryReach  = 1024 * 1024
	// How much further back the latest request is looked for, when it's not in summaryReach.
	summaryLookBack = 16 * 1024 * 1024
	// Each message's text, a tool's input and a tool's result, at most.
	summaryTextMost   = 1_500
	summaryToolMost   = 140
	summaryResultMost = 160
	// What comes just before the latest request, at most: enough to tell what a short answer answers.
	summaryLeadIn = 500
	// The latest request, when what follows it is too long to send whole.
	summaryLatestMost = 600
	// How what the user last asked is marked, so the summary is about it.
	summaryLatest = "LATEST REQUEST"
	// How a later message from another session is marked: it may give the agent its task now.
	summaryLatestPeer = "LATEST PEER MESSAGE"
)

// transcriptExtract is the transcript's tail as short lines, oldest first,
// already redacted, the language the user writes in, and which messages the
// latest request and any later peer message are (their ids in the
// transcript), to tell new ones from them.
type transcriptExtract struct {
	Text     string
	Language string
	Latest   string
}

// transcriptMessage is a message to the agent: what the user typed, or what
// another session sent it, From its name: a message, or a report it hands back.
type transcriptMessage struct {
	Kind           messageKind
	Text, ID, From string
}

type messageKind int

const (
	fromUser messageKind = iota
	fromPeer
	reportFromPeer
)

func (m transcriptMessage) label() string {
	switch m.Kind {
	case fromPeer:
		return "PEER " + m.From + ": "
	case reportFromPeer:
		return "REPORT from " + m.From + ": "
	}
	return "USER: "
}

// mark is how it's marked as the latest of its kind.
func (m transcriptMessage) mark() string {
	if m.Kind == fromUser {
		return summaryLatest + ": "
	}
	return summaryLatestPeer + " (from " + m.From + "): "
}

// line is its line: redacted before it's cut.
func (m transcriptMessage) line() string {
	return m.label() + clip(redactText(m.Text), summaryTextMost)
}

// latestMessage is a latest message and where it is in the lines, -1 when
// it's from before them.
type latestMessage struct {
	index   int
	message transcriptMessage
}

// readTranscriptTail is the tail of the transcript at path, or false if
// there's none to read.
func readTranscriptTail(path string) (transcriptExtract, bool) {
	if path == "" {
		return transcriptExtract{}, false
	}
	file, err := os.Open(path)
	if err != nil {
		return transcriptExtract{}, false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return transcriptExtract{}, false
	}
	start := int64(0)
	if info.Size() > summaryReach {
		start = info.Size() - summaryReach
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return transcriptExtract{}, false
	}
	data, err := io.ReadAll(io.LimitReader(file, summaryReach))
	if err != nil {
		return transcriptExtract{}, false
	}
	lines := bytes.Split(data, []byte("\n"))
	// A line cut off where the reading starts is no use here; it's the end of one looked at further back.
	var cut []byte
	if start > 0 && len(lines) > 0 {
		cut, lines = lines[0], lines[1:]
	}
	extract, asked := buildExtract(lines, summaryBudget, nil, nil)
	// Tool results are kept whole in a transcript, so the latest request is often further back than summaryReach.
	if !asked && start > 0 {
		if request, peer := lastMessagesBefore(file, start, cut); request != nil || peer != nil {
			extract, _ = buildExtract(lines, summaryBudget, request, peer)
		}
	}
	return extract, extract.Text != ""
}

// lastMessagesBefore is the last request in file before end, and the last
// peer message after it, looked for backwards a chunk at a time, at most
// summaryLookBack. cut is the end of the line end falls in.
func lastMessagesBefore(file *os.File, end int64, cut []byte) (request, peer *transcriptMessage) {
	stop := max(0, end-summaryLookBack)
	for end > stop {
		start := end - min(end-stop, summaryReach)
		chunk := make([]byte, end-start)
		if _, err := file.ReadAt(chunk, start); err != nil {
			break
		}
		lines := bytes.Split(append(chunk, cut...), []byte("\n"))
		cut = nil
		if start > 0 && len(lines) > 0 {
			cut, lines = lines[0], lines[1:]
		}
		for i := len(lines) - 1; i >= 0; i-- {
			var entry map[string]any
			if json.Unmarshal(lines[i], &entry) != nil {
				continue
			}
			message, ok := messageOf(entry)
			switch {
			case !ok:
			case message.Kind == fromUser:
				return &message, peer
			case message.Kind == fromPeer && peer == nil:
				peer = &message
			}
		}
		end = start
	}
	return nil, peer
}

// extractTranscript turns JSONL lines into the summary's excerpt, cut down to
// budget: from the latest request on, marked, with a short lead-in before it,
// and a later peer message marked too; or, if that's too long, those and the
// end of what came after them. earlier and earlierPeer are what's latest when
// it's before these lines.
func extractTranscript(jsonl [][]byte, budget int, earlier, earlierPeer *transcriptMessage) transcriptExtract {
	extract, _ := buildExtract(jsonl, budget, earlier, earlierPeer)
	return extract
}

// buildExtract is extractTranscript, and whether the user asked anything in jsonl.
func buildExtract(jsonl [][]byte, budget int, earlier, earlierPeer *transcriptMessage) (transcriptExtract, bool) {
	var items []string
	var messages []latestMessage
	for _, line := range jsonl {
		var entry map[string]any
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		if message, ok := messageOf(entry); ok {
			messages = append(messages, latestMessage{len(items), message})
			items = append(items, message.line())
			continue
		}
		kind, _ := entry["type"].(string)
		if (kind != "user" && kind != "assistant") || entry["isSidechain"] == true {
			continue
		}
		if message, ok := entry["message"].(map[string]any); ok {
			items = append(items, renderMessage(message, kind)...)
		}
	}
	for i := range items {
		items[i] = preparedLine(items[i])
	}

	var request, peer *latestMessage
	for i := len(messages) - 1; i >= 0 && request == nil; i-- {
		if messages[i].message.Kind == fromUser {
			request = &messages[i]
		}
	}
	if request == nil && earlier != nil {
		request = &latestMessage{-1, *earlier}
	}
	after := -1
	if request != nil {
		after = request.index
	}
	for i := len(messages) - 1; i >= 0 && peer == nil && messages[i].index > after; i-- {
		if messages[i].message.Kind == fromPeer {
			peer = &messages[i]
		}
	}
	if peer == nil && after < 0 && earlierPeer != nil {
		peer = &latestMessage{-1, *earlierPeer}
	}

	// The user's language, from their latest words if those lean either way: only what's sent of them.
	var words []string
	if earlier != nil {
		words = append(words, clip(redactText(earlier.Text), summaryTextMost))
	}
	for _, m := range messages {
		if m.message.Kind == fromUser {
			words = append(words, clip(redactText(m.message.Text), summaryTextMost))
		}
	}
	language := ""
	if len(words) > 0 {
		language = languageLean(words[len(words)-1:])
	}
	if language == "" {
		language = languageOf(words[max(0, len(words)-8):])
	}
	var ids []string
	var marks []latestMessage
	for _, m := range []*latestMessage{request, peer} {
		if m != nil {
			ids, marks = append(ids, m.message.ID), append(marks, *m)
		}
	}
	extract := transcriptExtract{Text: strings.Join(excerptLines(items, marks, budget), "\n"), Language: language, Latest: strings.Join(ids, " ")}
	return extract, request != nil && request.index >= 0
}

// excerptLines is items cut down to budget around marks, what's latest, marked.
func excerptLines(items []string, marks []latestMessage, budget int) []string {
	if len(marks) == 0 {
		return tailLines(items, budget)
	}
	items = append([]string(nil), items...)
	for _, m := range marks {
		if m.index >= 0 {
			items[m.index] = markedLine(items[m.index], m.message)
		}
	}
	// All of it from the first marked line on, if that's among items and fits, with a lead-in.
	if first := marks[0].index; first >= 0 {
		if used := runeCount(items[first:]); used <= budget {
			return append(leadLines(items[:first], min(summaryLeadIn, budget-used)), items[first:]...)
		}
	}
	// Or each marked line first, cut short, then the end that fits: a later
	// peer's message stays where it is when the end from it fits.
	var head []string
	from := 0
	most := max(2, min(summaryLatestMost, budget/len(marks)-3)) // each with its "…", within budget
	for _, m := range marks {
		if m.index >= 0 && m.message.Kind == fromPeer && runeCount(items[m.index:]) <= budget-runeCount(head) {
			break
		}
		line := markedLine(preparedLine(m.message.line()), m.message)
		if m.index >= 0 {
			line, from = items[m.index], m.index+1
		}
		head = append(head, clip(line, most), "…")
	}
	return append(head, tailLines(items[from:], budget-runeCount(head))...)
}

// preparedLine is item redacted, and with no line of its own starting as a
// mark does: only the latest messages' may.
func preparedLine(item string) string {
	return strings.ReplaceAll(redactText(item), "\nLATEST ", "\n LATEST ")
}

// markedLine is a latest message's line, item, with its mark for its label.
func markedLine(item string, message transcriptMessage) string {
	return message.mark() + strings.TrimPrefix(item, message.label())
}

// tailLines is as many of items, from the end, as budget takes.
func tailLines(items []string, budget int) []string {
	var kept []string
	used := 0
	for i := len(items) - 1; i >= 0; i-- {
		if used+len([]rune(items[i]))+1 > budget {
			break
		}
		kept = append([]string{items[i]}, kept...)
		used += len([]rune(items[i])) + 1
	}
	return kept
}

// leadLines is the end of what came before the latest request, in at most
// most characters, "…" first if there was more: whole lines, or the end of
// the last one alone, where a question is asked.
func leadLines(before []string, most int) []string {
	most -= 2 // for "…"
	var kept []string
	used := 0
	for i := len(before) - 1; i >= 0; i-- {
		if n := len([]rune(before[i])); used+n+1 <= most {
			kept = append([]string{before[i]}, kept...)
			used += n + 1
			continue
		}
		// The line is redacted already; again after the cut, as a line cut mid-word can start to look like a secret.
		if len(kept) == 0 && most > 40 {
			kept = []string{redactText(clipStart(before[i], most-1))}
		}
		break
	}
	if len(kept) < len(before) && most > 0 {
		kept = append([]string{"…"}, kept...)
	}
	return kept
}

func runeCount(lines []string) int {
	n := 0
	for _, line := range lines {
		n += len([]rune(line)) + 1
	}
	return n
}

// messageOf is the entry as a message to the agent, if it is one: what the
// user typed, or what another session sent it. One sent while the agent
// works is an attachment, queued into its turn.
func messageOf(entry map[string]any) (transcriptMessage, bool) {
	// A compaction's summary of the conversation so far is in the user's turn, but no one asked it.
	if entry["isSidechain"] == true || entry["isCompactSummary"] == true || entry["isVisibleInTranscriptOnly"] == true {
		return transcriptMessage{}, false
	}
	var content any
	var origin map[string]any
	var meta bool
	switch entry["type"] {
	case "user":
		message, ok := entry["message"].(map[string]any)
		if !ok {
			return transcriptMessage{}, false
		}
		content, meta = message["content"], entry["isMeta"] == true
		origin, _ = entry["origin"].(map[string]any)
	case "attachment":
		queued, _ := entry["attachment"].(map[string]any)
		if queued["type"] != "queued_command" || queued["commandMode"] != "prompt" {
			return transcriptMessage{}, false
		}
		content, meta = queued["prompt"], queued["isMeta"] == true
		origin, _ = queued["origin"].(map[string]any)
	default:
		return transcriptMessage{}, false
	}
	text := strings.TrimSpace(wordsOf(content))
	id := entryID(entry, text)
	switch kind, _ := origin["kind"].(string); kind {
	case "human":
		return transcriptMessage{Kind: fromUser, Text: text, ID: id}, text != "" && !isNote(text)
	case "peer":
		// A report handed back is what its body says around it.
		report := origin["handback"] == true
		said := text
		if body, ok := origin["body"].(string); ok {
			said = body
		}
		if body, ok := origin["handbackReportText"].(string); ok && report {
			said = body
		}
		said = strings.TrimSpace(said)
		// Its name is redacted too: it's shown as given.
		name, _ := origin["name"].(string)
		if name = clip(redactText(firstLine(name)), 60); name == "" {
			name = "another session"
		}
		kind := fromPeer
		if report {
			kind = reportFromPeer
		}
		return transcriptMessage{Kind: kind, Text: said, ID: id, From: name}, said != ""
	case "":
		// Where Claude Code didn't write who a message came from: the user's words, but not its own notes.
		ok := entry["type"] == "user" && !meta && text != "" && !strings.HasPrefix(text, "<") &&
			!strings.HasPrefix(text, "Caveat:") && !isNote(text)
		return transcriptMessage{Kind: fromUser, Text: text, ID: id}, ok
	default: // a task's notification, and anything new
		return transcriptMessage{}, false
	}
}

// isNote is a command run or an interruption, in the user's turn: shown as
// what it is, not as a request.
func isNote(text string) bool {
	return strings.HasPrefix(text, "<command-name>") || strings.HasPrefix(text, "[Request interrupted")
}

// wordsOf is what a message says in words: its text, or its text blocks together.
func wordsOf(content any) string {
	if text, ok := content.(string); ok {
		return text
	}
	return resultText(content)
}

// entryID is which message a transcript entry is: its uuid, or failing that
// its time, or failing that what it says.
func entryID(entry map[string]any, text string) string {
	for _, field := range []string{"uuid", "timestamp"} {
		if id, _ := entry[field].(string); id != "" {
			return id
		}
	}
	return text
}

// renderMessage is a message's lines, each redacted before it's cut.
func renderMessage(message map[string]any, kind string) []string {
	if text, ok := message["content"].(string); ok {
		return said(text, kind)
	}
	blocks, _ := message["content"].([]any)
	var out []string
	for _, b := range blocks {
		block, _ := b.(map[string]any)
		switch block["type"] {
		case "text":
			text, _ := block["text"].(string)
			out = append(out, said(text, kind)...)
		case "tool_use":
			out = append(out, "TOOL "+toolLine(block))
		case "tool_result":
			first := redactText(firstLine(resultText(block["content"])))
			if first == "" {
				first = "(nothing)"
			}
			label := "RESULT: "
			if block["is_error"] == true {
				label = "ERROR: "
			}
			out = append(out, label+clip(first, summaryResultMost))
		}
		// Thinking, images, and anything new: left out.
	}
	return out
}

func said(text, kind string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if kind != "user" {
		return []string{"CLAUDE: " + clip(redactText(text), summaryTextMost)}
	}
	// What was asked is a message; what's left in the user's turn is Claude Code's own: a command, an interruption.
	if strings.HasPrefix(text, "<command-name>") {
		name := ""
		if end := strings.Index(text, "</command-name>"); end > len("<command-name>") {
			name = text[len("<command-name>"):end]
		}
		if name == "" {
			return nil
		}
		return []string{"USER ran " + clip(redactText(name), 60)}
	}
	if strings.HasPrefix(text, "[Request interrupted") {
		return []string{"(the user interrupted)"}
	}
	return nil
}

// toolLine is a tool's call in a line: its name and the one part of its input
// that says what it did. From its input, only paths, commands and searches,
// never text it was given to write. What it gave back goes as its first line
// (see above): for a file it read, that file's first line.
func toolLine(block map[string]any) string {
	name, _ := block["name"].(string)
	if name == "" {
		name = "tool"
	}
	input, _ := block["input"].(map[string]any)
	for _, field := range []string{"file_path", "notebook_path", "path", "command", "pattern", "url", "query", "description", "subject", "skill"} {
		if value, ok := input[field].(string); ok && value != "" {
			return name + ": " + clip(redactText(firstLine(value)), summaryToolMost)
		}
	}
	return name
}

func resultText(content any) string {
	if text, ok := content.(string); ok {
		return text
	}
	blocks, _ := content.([]any)
	var parts []string
	for _, b := range blocks {
		if block, ok := b.(map[string]any); ok && block["type"] == "text" {
			if text, ok := block["text"].(string); ok {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func firstLine(text string) string {
	for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' }) {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

func clip(text string, most int) string {
	runes := []rune(text)
	if len(runes) <= most {
		return text
	}
	return string(runes[:most-1]) + "…"
}

// clipStart is text's end, in at most most characters, keeping its label.
func clipStart(text string, most int) string {
	runes := []rune(text)
	if len(runes) <= most {
		return text
	}
	label := ""
	for _, l := range []string{"USER: ", "CLAUDE: ", "RESULT: ", "ERROR: "} {
		if strings.HasPrefix(text, l) {
			label = l
			break
		}
	}
	if colon := strings.Index(text, ": "); label == "" && (strings.HasPrefix(text, "PEER ") || strings.HasPrefix(text, "REPORT from ")) && colon > 0 {
		label = text[:colon+2]
	}
	keep := max(0, most-len([]rune(label))-1)
	return label + "…" + string(runes[len(runes)-keep:])
}

// Lines that look like they hold a secret, taken out before anything is sent.
var secretLine = []*regexp.Regexp{
	regexp.MustCompile(`(^|[^A-Za-z0-9_-])sk-[A-Za-z0-9]`), // Anthropic, OpenAI and others' keys
	regexp.MustCompile(`(^|[^A-Za-z0-9_-])re_[A-Za-z0-9]`), // Resend
	regexp.MustCompile(`(?i)key['"]?\s*[=:]`),              // API_KEY=…, "apiKey": …
	regexp.MustCompile(`(?i)passw(or)?d`),
	regexp.MustCompile(`(?i)token`),
	regexp.MustCompile(`(?i)secret`),
	regexp.MustCompile(`(?i)bearer\s`),
	regexp.MustCompile(`(^|[^A-Za-z0-9])(ghp|gho|github_pat|xox[abpr]|AKIA)[A-Za-z0-9_-]`),
	regexp.MustCompile(`(^|[^A-Za-z0-9_])(sk|rk|pk)_(live|test)_[A-Za-z0-9]`), // Stripe
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.`),           // a JWT
	regexp.MustCompile(`[A-Za-z0-9+.-]://[^/\s:@]*:[^/\s@]+@`),                // postgres://user:password@… or redis://:password@…, found from its "://", as in Summary.swift
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY`),
}

var (
	pemEnd = regexp.MustCompile(`-----END [A-Z ]*PRIVATE KEY`)
	// Runs of the characters an AWS secret key is made of; hasAWSSecret looks at each whole one.
	keyChars = regexp.MustCompile(`[A-Za-z0-9/+=]+`)
)

// hasAWSSecret is whether line holds a bare AWS secret key: 40 characters with
// capitals, small letters and digits, which a git commit's 40 lowercase hex never has.
func hasAWSSecret(line string) bool {
	for _, token := range keyChars.FindAllString(line, -1) {
		if len(token) == 40 && strings.ContainsAny(token, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") &&
			strings.ContainsAny(token, "abcdefghijklmnopqrstuvwxyz") && strings.ContainsAny(token, "0123456789") {
			return true
		}
	}
	return false
}

const redactedPlaceholder = "[redacted]"

func isSecretLine(line string) bool {
	for _, pattern := range secretLine {
		if pattern.MatchString(line) {
			return true
		}
	}
	return hasAWSSecret(line)
}

// redactText is text with each line that looks like a secret replaced, keeping
// its label, and a private key's every line, from its BEGIN through its END.
func redactText(text string) string {
	lines := strings.Split(text, "\n")
	inKey := false
	for i, line := range lines {
		if inKey {
			inKey = !pemEnd.MatchString(line)
			lines[i] = redactedPlaceholder
			continue
		}
		if !isSecretLine(line) {
			continue
		}
		if strings.Contains(line, "-----BEGIN") && strings.Contains(line, "PRIVATE KEY") && !pemEnd.MatchString(line) {
			inKey = true
		}
		lines[i] = redactedPlaceholder
		for _, label := range []string{"USER: ", "CLAUDE: ", "RESULT: ", "ERROR: "} {
			if strings.HasPrefix(line, label) {
				lines[i] = label + redactedPlaceholder
			}
		}
		// "TOOL Bash: export X=…" keeps "TOOL Bash:", so the summary still knows a command ran.
		if strings.HasPrefix(line, "TOOL ") {
			if colon := strings.Index(line, ":"); colon >= 0 {
				lines[i] = line[:colon+1] + " " + redactedPlaceholder
			}
		}
	}
	return strings.Join(lines, "\n")
}

var (
	frenchWords  = wordSet("le la les un une des est et je tu il on nous vous pas que qui pour dans sur avec ça c'est mais ou du au aux ce cette fais peux merci oui non bien faut veux moi mon ma mes ton ta tes comme aussi")
	englishWords = wordSet("the a an is and i you it we not that who for in on with this but or of to do can please thanks yes no my your what should make")
	accented     = regexp.MustCompile(`[éèêàùçôî]`)
)

func wordSet(words string) map[string]bool {
	set := map[string]bool{}
	for _, word := range strings.Fields(words) {
		set[word] = true
	}
	return set
}

// languageOf is French or English, from the words the user wrote.
func languageOf(texts []string) string {
	if language := languageLean(texts); language != "" {
		return language
	}
	return "English"
}

// languageLean is French or English, or "" if texts lean neither way.
func languageLean(texts []string) string {
	fr, en := 0, 0
	for _, text := range texts {
		for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && r != '\'' }) {
			if frenchWords[word] {
				fr++
			}
			if englishWords[word] {
				en++
			}
		}
		if accented.MatchString(text) {
			fr++
		}
	}
	switch {
	case fr > en:
		return "French"
	case en > fr:
		return "English"
	}
	return ""
}

// MARK: Asking Claude

// SummaryError is why a summary couldn't be had, in the phone's words: fixed
// text, never a reply's body, the request or the key.
type SummaryError struct {
	Message string
	Status  int
}

func (e *SummaryError) Error() string { return e.Message }

var (
	errSummaryOff       = &SummaryError{"Summaries are off: turn them on in setup, on your computer.", 403}
	errSummaryNoKey     = &SummaryError{"No Anthropic API key: add yours in setup, on your computer.", 403}
	errSummaryNoSession = &SummaryError{"That agent has gone.", 404}
	errSummaryNoHistory = &SummaryError{"No history for this agent.", 404}
	errSummaryTooMany   = &SummaryError{"Too many summaries just now: try again in a few minutes.", 429}
	errSummaryTimeout   = &SummaryError{"Anthropic didn't answer in time. Try again.", 504}
	errSummaryOffline   = &SummaryError{"Couldn't reach Anthropic from your computer.", 502}
	errSummaryBadKey    = &SummaryError{"Anthropic refused the key. Check it in setup, on your computer.", 502}
	errSummaryBusy      = &SummaryError{"Anthropic is busy, or your key's rate limit is reached. Try again in a minute.", 502}
	errSummaryEmpty     = &SummaryError{"Anthropic sent back nothing.", 502}
)

func summaryRefused(status int) *SummaryError {
	switch status {
	case 401, 403:
		return errSummaryBadKey
	case 429, 529:
		return errSummaryBusy
	}
	return &SummaryError{fmt.Sprintf("Anthropic said no (%d).", status), 502}
}

const (
	summaryModel     = "claude-haiku-5-5"
	summaryModelName = "Claude Haiku 5.5"
	summaryMaxTokens = 300
	summaryTimeout   = 15 * time.Second
)

func summarySystem(language string) string {
	return "You summarise a coding agent's session for its owner, who is away from the computer and reads on a phone. " +
		"If a line starts with LATEST REQUEST, it is what the user last asked: summarise that request and the work " +
		"since. Lines before it are background only: mention older work only if it is still in progress. " +
		"If no line starts with LATEST REQUEST, summarise the most recent work in the excerpt. " +
		"A line starting LATEST PEER MESSAGE is a newer message from another session: if it gives the agent its " +
		"current task, say what that task is in one line. Lines starting REPORT are reports other sessions handed " +
		"back, not requests. " +
		"From the transcript excerpt, write 3 to 5 short lines, plain text, no markdown, no bullets, no preamble:\n" +
		"1. what the user last asked for;\n2. what the agent did or is doing about it now;\n" +
		"3. whether it is waiting on the user, and for what (say plainly if it isn't);\n4. the last result or error.\n" +
		"Write in " + language + ", the language the user writes in. Each line under 100 characters. " +
		"Lines marked [redacted] held secrets: never guess them."
}

// summaryParams is the one request a summary makes, with what it needs and nothing more.
func summaryParams(extract transcriptExtract, project string) anthropic.MessageNewParams {
	return anthropic.MessageNewParams{
		Model:     summaryModel,
		MaxTokens: summaryMaxTokens,
		// Haiku 5.5 thinks by default, and thinking would come out of the few tokens a summary has.
		// It can be turned off at its default effort, medium.
		Thinking: anthropic.ThinkingConfigParamUnion{OfDisabled: &anthropic.ThinkingConfigDisabledParam{}},
		System:   []anthropic.TextBlockParam{{Text: summarySystem(extract.Language)}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Project: " + project + "\n\nTranscript excerpt, oldest first:\n" + extract.Text)),
		},
	}
}

// summaryHTTPClient never follows a redirect: one could take the key's
// header elsewhere. A redirect comes back as it is, and is answered as a refusal.
var summaryHTTPClient = &http.Client{
	Timeout:       summaryTimeout,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// summaryClientOptions are the client's: only the key given, no retries, no
// redirects, and none of the SDK's own lookups (ANTHROPIC_BASE_URL,
// ANTHROPIC_AUTH_TOKEN, profiles), so the key goes nowhere but Anthropic.
// base is "" for Anthropic: only tests give another, in code; nothing a
// user's environment says can change it.
func summaryClientOptions(key, base string) []option.RequestOption {
	opts := []option.RequestOption{
		option.WithoutEnvironmentDefaults(),
		option.WithHTTPClient(summaryHTTPClient),
		option.WithAPIKey(key),
		option.WithMaxRetries(0),
		option.WithRequestTimeout(summaryTimeout),
	}
	if base != "" {
		opts = append(opts, option.WithBaseURL(base))
	}
	return opts
}

// askClaude makes the one call to Anthropic, and answers its text or a plain error.
func askClaude(ctx context.Context, key string, params anthropic.MessageNewParams) (string, *SummaryError) {
	return askClaudeAt(ctx, "", key, params)
}

// askClaudeAt is askClaude, at base: "" for Anthropic, a stand-in in tests.
func askClaudeAt(ctx context.Context, base, key string, params anthropic.MessageNewParams) (string, *SummaryError) {
	client := anthropic.NewClient(summaryClientOptions(key, base)...)
	message, err := client.Messages.New(ctx, params)
	if err != nil {
		var apiErr *anthropic.Error
		switch {
		case errors.As(err, &apiErr):
			return "", summaryRefused(apiErr.StatusCode)
		case errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Client.Timeout"):
			return "", errSummaryTimeout
		default:
			return "", errSummaryOffline
		}
	}
	var parts []string
	for _, block := range message.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			parts = append(parts, text.Text)
		}
	}
	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if text == "" {
		return "", errSummaryEmpty
	}
	return text, nil
}

// MARK: The key

// The key Summary uses: one pasted in setup, kept in this computer's own
// secret store (Windows Credential Manager, or the Secret Service on Linux),
// or else ANTHROPIC_API_KEY from the app's environment. Never a file of its
// own, a project's .env or any other file: with no secret store, only the
// environment's key works.
const (
	summaryKeyService = "smolcorp-summary"
	summaryKeyAccount = "anthropic-api-key"
	summaryKeyEnv     = "ANTHROPIC_API_KEY"
)

// secretStore keeps the pasted key: the system's keyring, or a stand-in in tests.
type secretStore struct {
	get    func() (string, error)
	set    func(string) error
	delete func() error
}

// isAnthropicKey is a key as Anthropic makes them: sk-ant-…, one word, nothing else.
func isAnthropicKey(text string) bool {
	if !strings.HasPrefix(text, "sk-ant-") || len(text) < 20 || len(text) > 512 {
		return false
	}
	for _, r := range text {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// keyHint is "sk-ant-…ab12": enough to tell which key, nothing to use.
func keyHint(key string) string { return "sk-ant-…" + key[len(key)-4:] }

// summaryKeys finds the key and remembers what setup may show of it.
type summaryKeys struct {
	mu     sync.Mutex
	store  secretStore
	getenv func(string) string
	// known: the pasted key's hint once looked at ("" for none), so setup,
	// asking every second or so, doesn't go to the keyring each time.
	known     bool
	hint      string
	storeDown bool
}

func newSummaryKeys(store secretStore, getenv func(string) string) *summaryKeys {
	return &summaryKeys{store: store, getenv: getenv}
}

// Read is the key a summary is made with, read as it's made: the pasted one first.
func (k *summaryKeys) Read() string {
	if key, err := k.store.get(); err == nil && isAnthropicKey(key) {
		return key
	}
	if key := strings.TrimSpace(k.getenv(summaryKeyEnv)); isAnthropicKey(key) {
		return key
	}
	return ""
}

// Shown is what setup may show: where the key comes from ("store", the
// system's secret store, or "environment") and its hint, never the key.
func (k *summaryKeys) Shown() (source, hint string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if !k.known {
		key, err := k.store.get()
		k.storeDown = err != nil && !errors.Is(err, keyring.ErrNotFound)
		k.hint = ""
		if err == nil && isAnthropicKey(key) {
			k.hint = keyHint(key)
		}
		k.known = true
	}
	if k.hint != "" {
		return "store", k.hint
	}
	if key := strings.TrimSpace(k.getenv(summaryKeyEnv)); isAnthropicKey(key) {
		return "environment", keyHint(key)
	}
	return "", ""
}

// Present is whether there's a key, from either place.
func (k *summaryKeys) Present() bool {
	source, _ := k.Shown()
	return source != ""
}

// NoStore is whether this computer has no secret store to keep a pasted key in.
func (k *summaryKeys) NoStore() bool {
	k.Shown()
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.storeDown
}

// Save keeps a pasted key, replacing any before it.
func (k *summaryKeys) Save(text string) *SummaryError {
	key := strings.TrimSpace(text)
	if !isAnthropicKey(key) {
		return &SummaryError{"That isn't an Anthropic API key. It starts with sk-ant-.", 400}
	}
	if err := k.store.set(key); err != nil {
		return &SummaryError{"There's no secret store on this computer to keep it in. Set ANTHROPIC_API_KEY for Smolcorp instead.", 400}
	}
	k.mu.Lock()
	k.known, k.hint, k.storeDown = true, keyHint(key), false
	k.mu.Unlock()
	return nil
}

func (k *summaryKeys) Remove() {
	_ = k.store.delete()
	k.mu.Lock()
	k.known, k.hint = true, ""
	k.mu.Unlock()
}

// MARK: Summaries

// summaries are by session: one call at a time for each, kept for
// summaryKeepFor so a second tap doesn't pay twice unless the user has asked
// something since, and at most summaryMost
// calls in summaryWindow for the whole computer, so no phone can run up your
// bill. Never retried: a tap is one call at most.
type summaries struct {
	mu      sync.Mutex
	kept    map[string]keptSummary
	waiting map[string][]chan summaryResult
	calls   []time.Time
	key     func() string
	ask     func(ctx context.Context, key string, params anthropic.MessageNewParams) (string, *SummaryError)
	now     func() time.Time
}

const (
	summaryKeepFor = 30 * time.Second
	summaryMost    = 20
	summaryWindow  = 10 * time.Minute
)

type keptSummary struct {
	text   string
	latest string // the latest request it covered
	at     time.Time
}

type summaryResult struct {
	text string
	err  *SummaryError
}

func newSummaries(key func() string) *summaries {
	return &summaries{kept: map[string]keptSummary{}, waiting: map[string][]chan summaryResult{}, key: key, ask: askClaude, now: time.Now}
}

// Summarize answers the summary of session id from its transcript at path.
func (s *summaries) Summarize(id, path, project string) (string, *SummaryError) {
	// Read first: a summary kept is served only while the user's latest request is still the one it covered.
	extract, ok := readTranscriptTail(path)
	s.mu.Lock()
	if hit, found := s.kept[id]; found && ok && hit.latest == extract.Latest && s.now().Sub(hit.at) < summaryKeepFor {
		s.mu.Unlock()
		return hit.text, nil
	}
	done := make(chan summaryResult, 1)
	if waiters, ok := s.waiting[id]; ok {
		s.waiting[id] = append(waiters, done)
		s.mu.Unlock()
		result := <-done
		return result.text, result.err
	}
	recent := s.calls[:0]
	for _, at := range s.calls {
		if s.now().Sub(at) < summaryWindow {
			recent = append(recent, at)
		}
	}
	s.calls = recent
	if len(s.calls) >= summaryMost {
		s.mu.Unlock()
		return "", errSummaryTooMany
	}
	s.calls = append(s.calls, s.now())
	s.waiting[id] = []chan summaryResult{done}
	s.mu.Unlock()

	result := s.request(extract, ok, project)
	s.mu.Lock()
	if result.err == nil {
		s.kept[id] = keptSummary{result.text, extract.Latest, s.now()}
	}
	waiters := s.waiting[id]
	delete(s.waiting, id)
	s.mu.Unlock()
	for _, waiter := range waiters {
		waiter <- result
	}
	return result.text, result.err
}

func (s *summaries) request(extract transcriptExtract, ok bool, project string) summaryResult {
	if !ok {
		return summaryResult{err: errSummaryNoHistory}
	}
	key := s.key()
	if key == "" {
		return summaryResult{err: errSummaryNoKey}
	}
	ctx, cancel := context.WithTimeout(context.Background(), summaryTimeout)
	defer cancel()
	text, err := s.ask(ctx, key, summaryParams(extract, project))
	return summaryResult{text, err}
}

// Forget drops every summary kept, as the setting goes off or the key changes.
func (s *summaries) Forget() {
	s.mu.Lock()
	s.kept = map[string]keptSummary{}
	s.mu.Unlock()
}
