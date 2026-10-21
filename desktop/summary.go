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
	// Each message's text, a tool's input and a tool's result, at most.
	summaryTextMost   = 1_500
	summaryToolMost   = 140
	summaryResultMost = 160
)

// transcriptExtract is the transcript's tail as short lines, oldest first,
// already redacted, and the language the user writes in.
type transcriptExtract struct {
	Text     string
	Language string
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
	// A line cut off where the reading starts is no use.
	if start > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	extract := extractTranscript(lines, summaryBudget)
	return extract, extract.Text != ""
}

// extractTranscript turns JSONL lines into the summary's excerpt, cut down to budget.
func extractTranscript(jsonl [][]byte, budget int) transcriptExtract {
	var items, asked []string
	lastAsk, lastAskLine := -1, ""
	for _, line := range jsonl {
		var entry map[string]any
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		kind, _ := entry["type"].(string)
		if (kind != "user" && kind != "assistant") || entry["isSidechain"] == true {
			continue
		}
		message, ok := entry["message"].(map[string]any)
		if !ok {
			continue
		}
		meta := entry["isMeta"] == true
		for _, r := range renderMessage(message, kind) {
			if r.asked {
				if meta {
					continue
				}
				asked = append(asked, r.text)
				line := "USER: " + clip(r.text, summaryTextMost)
				lastAsk, lastAskLine = len(items), line
				items = append(items, line)
			} else {
				items = append(items, r.text)
			}
		}
	}
	for i := range items {
		items[i] = redactText(items[i])
	}

	// From the end, as many lines as the budget takes.
	var kept []string
	used, firstKept := 0, len(items)
	for i := len(items) - 1; i >= 0; i-- {
		if used+len([]rune(items[i]))+1 > budget {
			break
		}
		kept = append([]string{items[i]}, kept...)
		used += len([]rune(items[i])) + 1
		firstKept = i
	}
	// What the user last asked, even from before the tail, so it's known what the agent is on.
	if lastAsk >= 0 && lastAsk < firstKept {
		kept = append([]string{redactText(clip(lastAskLine, 600)) + "\n…"}, kept...)
		for len(kept) > 1 && runeCount(kept) > budget {
			kept = append(kept[:1], kept[2:]...)
		}
	}
	recent := asked
	if len(recent) > 8 {
		recent = recent[len(recent)-8:]
	}
	return transcriptExtract{Text: strings.Join(kept, "\n"), Language: languageOf(recent)}
}

func runeCount(lines []string) int {
	n := 0
	for _, line := range lines {
		n += len([]rune(line)) + 1
	}
	return n
}

type rendered struct {
	text  string
	asked bool
}

func renderMessage(message map[string]any, kind string) []rendered {
	if text, ok := message["content"].(string); ok {
		return said(text, kind)
	}
	blocks, _ := message["content"].([]any)
	var out []rendered
	for _, b := range blocks {
		block, _ := b.(map[string]any)
		switch block["type"] {
		case "text":
			text, _ := block["text"].(string)
			out = append(out, said(text, kind)...)
		case "tool_use":
			out = append(out, rendered{text: "TOOL " + toolLine(block)})
		case "tool_result":
			first := firstLine(resultText(block["content"]))
			if first == "" {
				first = "(nothing)"
			}
			label := "RESULT: "
			if block["is_error"] == true {
				label = "ERROR: "
			}
			out = append(out, rendered{text: label + clip(first, summaryResultMost)})
		}
		// Thinking, images, and anything new: left out.
	}
	return out
}

func said(text, kind string) []rendered {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if kind != "user" {
		return []rendered{{text: "CLAUDE: " + clip(text, summaryTextMost)}}
	}
	// Claude Code's own notes in the user's turn: a command's output, a reminder, an interruption.
	if strings.HasPrefix(text, "<command-name>") {
		name := ""
		if end := strings.Index(text, "</command-name>"); end > len("<command-name>") {
			name = text[len("<command-name>"):end]
		}
		if name == "" {
			return nil
		}
		return []rendered{{text: "USER ran " + clip(name, 60)}}
	}
	if strings.HasPrefix(text, "<") || strings.HasPrefix(text, "Caveat:") {
		return nil
	}
	if strings.HasPrefix(text, "[Request interrupted") {
		return []rendered{{text: "(the user interrupted)"}}
	}
	return []rendered{{text: text, asked: true}}
}

// toolLine is a tool's call in a line: its name and the one part of its input
// that says what it did. Paths, commands and searches; never a file's contents.
func toolLine(block map[string]any) string {
	name, _ := block["name"].(string)
	if name == "" {
		name = "tool"
	}
	input, _ := block["input"].(map[string]any)
	for _, field := range []string{"file_path", "notebook_path", "path", "command", "pattern", "url", "query", "description", "subject", "skill"} {
		if value, ok := input[field].(string); ok && value != "" {
			return name + ": " + clip(firstLine(value), summaryToolMost)
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
	regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^/\s:@]+:[^/\s@]+@`),       // postgres://user:password@…
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
	if fr > en {
		return "French"
	}
	return "English"
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
		"From the transcript excerpt, write 3 to 5 short lines, plain text, no markdown, no bullets, no preamble:\n" +
		"1. what the user asked for;\n2. what the agent did or is doing now;\n" +
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
// summaryKeepFor so a second tap doesn't pay twice, and at most summaryMost
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
	text string
	at   time.Time
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
	s.mu.Lock()
	if hit, ok := s.kept[id]; ok && s.now().Sub(hit.at) < summaryKeepFor {
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

	result := s.request(path, project)
	s.mu.Lock()
	if result.err == nil {
		s.kept[id] = keptSummary{result.text, s.now()}
	}
	waiters := s.waiting[id]
	delete(s.waiting, id)
	s.mu.Unlock()
	for _, waiter := range waiters {
		waiter <- result
	}
	return result.text, result.err
}

func (s *summaries) request(path, project string) summaryResult {
	extract, ok := readTranscriptTail(path)
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
