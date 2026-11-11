package desktop

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	agentdeck "github.com/Sauzze/smolcorp"
)

// A hire's brief, as the Mac app makes it (Briefs.swift): the role's brief
// from the library that comes with the app (Resources/briefs), or a file
// picked in setup, filled in on hire and written as BRIEF.md where the hire
// reads it first. The text is the Mac's byte for byte for the same inputs:
// Tests/fixtures/briefs/briefs.json, which the Mac's tests write, holds both.

// The few rules a brief is written in: {{name}} for a value, {{#name}}…{{/name}}
// for text kept only when the value is set (not empty, false, no or 0) and
// {{^name}}…{{/name}} only when it isn't, <!-- … --> for notes to whoever edits
// it, and front matter between --- lines on top. A section tag alone on its
// line takes the line with it. The library's skeleton also has {{> slot}}, for
// each brief's own sections.
var (
	// Every value a hire fills, besides the front matter's own keys.
	briefNames = []string{"role", "date", "project_folder", "branch", "agent", "model", "effort", "job", "approval",
		"status_file", "brief_file", "report_file", "done_signal", "read_only"}

	briefComment       = regexp.MustCompile(`(?m)^[ \t]*<!--[\s\S]*?-->[ \t]*\n|<!--[\s\S]*?-->`)
	briefStandaloneTag = regexp.MustCompile(`(?m)^[ \t]*(\{\{[#^/][A-Za-z0-9_]+\}\})[ \t]*\n`)
	briefOpeningTag    = regexp.MustCompile(`^\{\{([#^])([A-Za-z0-9_]+)\}\}`)
	briefValue         = regexp.MustCompile(`\{\{([A-Za-z0-9_]+)\}\}`)
	briefSectionName   = regexp.MustCompile(`\{\{[#^/]([A-Za-z0-9_]+)\}\}`)
	briefSpaceBeforeNL = regexp.MustCompile(`[ \t]+\n`)
	briefBlankLines    = regexp.MustCompile(`\n{3,}`)
	briefFrontKey      = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	briefSlotLine      = regexp.MustCompile(`^<!-- slot: ([a-z_]+) -->$`)
	briefSlotWhole     = regexp.MustCompile(`^\{\{> ([a-z_]+)\}\}$`)
	briefSlotInline    = regexp.MustCompile(`\{\{> ([a-z_]+)\}\}`)
	briefReportName    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$`)
)

// The Mac's character classes (Foundation's CharacterSet), so text is trimmed and
// cleaned exactly as it is there.
func swiftWhitespace(r rune) bool { return r == '\t' || unicode.Is(unicode.Zs, r) }

func swiftWhitespaceOrNewline(r rune) bool {
	return swiftWhitespace(r) || r >= '\n' && r <= '\r' || r == 0x85 || r == 0x2028 || r == 0x2029
}

func swiftControl(r rune) bool { return unicode.In(r, unicode.Cc, unicode.Cf) }

func trimSwiftWhitespace(s string) string { return strings.TrimFunc(s, swiftWhitespace) }

func trimSwiftSpace(s string) string { return strings.TrimFunc(s, swiftWhitespaceOrNewline) }

// withoutControls is text without control characters, but those keep says.
func withoutControls(text string, keep string) string {
	return strings.Map(func(r rune) rune {
		if swiftControl(r) && !strings.ContainsRune(keep, r) {
			return -1
		}
		return r
	}, text)
}

// prefixRunes is at most n characters of text.
func prefixRunes(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	return string([]rune(text)[:n])
}

// briefProblem is why a picked file can't be a brief, in words setup shows as they are.
type briefProblem string

func (p briefProblem) Error() string { return string(p) }

const (
	briefEmpty    briefProblem = "The brief is empty."
	briefTooLong  briefProblem = "The brief is over 32 KB. Pick a shorter file."
	briefNotText  briefProblem = "That file isn't text. Pick a Markdown or text file."
	briefFileName briefProblem = "Pick a .md, .markdown or .txt file."
	briefSlots    briefProblem = "{{> …}} works only in the briefs that come with the app."
	briefSections briefProblem = "A {{#…}}, {{^…}} or {{/…}} in the brief has no partner."
)

func briefUnknown(names []string) briefProblem {
	wrap := func(list []string) string {
		out := make([]string, len(list))
		for i, name := range list {
			out[i] = "{{" + name + "}}"
		}
		return strings.Join(out, ", ")
	}
	known := slices.Sorted(slices.Values(briefNames))
	return briefProblem("The brief has placeholders Smolcorp can't fill: " + wrap(names) + ". It can fill " + wrap(known) + ".")
}

// briefFile is a picked file as kept: its text without a byte-order mark,
// with plain line ends and no control characters but tabs, and its name
// without a folder. Its placeholders and sections must be ones a hire can fill.
func briefFile(text, name string) (RoleBrief, error) {
	if strings.ContainsRune(text, 0) || strings.ContainsRune(text, utf8.RuneError) {
		return RoleBrief{}, briefNotText
	}
	text = strings.TrimPrefix(text, "\uFEFF")
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	text = withoutControls(text, "\n\t")
	if trimSwiftSpace(text) == "" {
		return RoleBrief{}, briefEmpty
	}
	if len(text) > roleBriefFileLimit {
		return RoleBrief{}, briefTooLong
	}
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' })
	base := ""
	if len(parts) > 0 {
		base = parts[len(parts)-1]
	}
	fileName := trimSwiftWhitespace(withoutControls(base, ""))
	count := utf8.RuneCountInString(fileName)
	if count < 1 || count > roleBriefFileNameLimit || !slices.Contains([]string{"md", "markdown", "txt"}, strings.ToLower(pathExtension(fileName))) {
		return RoleBrief{}, briefFileName
	}
	if err := checkBriefTemplate(text); err != nil {
		return RoleBrief{}, err
	}
	return RoleBrief{Kind: "file", Text: text, FileName: fileName}, nil
}

// pathExtension is a file name's extension as the Mac reads it: after the
// last dot, but none for a name that is only a dot and what follows.
func pathExtension(name string) string {
	i := strings.LastIndex(name, ".")
	if i <= 0 {
		return ""
	}
	return name[i+1:]
}

// briefBranch is the branch checked out where folder is, or "" outside git or
// with none checked out. Read from git's own files, without running git.
func briefBranch(folder string) string {
	dir := filepath.Clean(folder)
	for {
		dotGit := filepath.Join(dir, ".git")
		if info, err := os.Stat(dotGit); err == nil {
			gitDir := dotGit
			// A worktree's .git is a file naming its folder.
			if !info.IsDir() {
				data, err := os.ReadFile(dotGit)
				line := string(data)
				if err != nil || !strings.HasPrefix(line, "gitdir:") {
					return ""
				}
				path := trimSwiftSpace(strings.TrimPrefix(line, "gitdir:"))
				if filepath.IsAbs(path) {
					gitDir = path
				} else {
					gitDir = filepath.Join(dir, path)
				}
			}
			head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
			if err != nil {
				return ""
			}
			ref := trimSwiftSpace(string(head))
			name, ok := strings.CutPrefix(ref, "ref: refs/heads/")
			if !ok || name == "" || strings.ContainsFunc(name, swiftControl) {
				return ""
			}
			return name
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// briefFrontMatter is a brief's front matter (key: value lines, and its
// questions), the front matter as written with its --- lines, and the text after it.
type briefFrontMatter struct {
	values    map[string]string
	questions []string
	head      string
	body      string
}

func parseBriefFrontMatter(text string) briefFrontMatter {
	parsed := briefFrontMatter{values: map[string]string{}, body: text}
	if !strings.HasPrefix(text, "---\n") {
		return parsed
	}
	at := strings.Index(text[3:], "\n---\n")
	if at < 0 {
		return parsed
	}
	lower, upper := 3+at, 3+at+len("\n---\n")
	parsed.head, parsed.body = text[:upper], text[upper:]
	lines := ""
	if 4 < lower {
		lines = text[4:lower]
	}
	for _, line := range strings.Split(lines, "\n") {
		colon := strings.Index(line, ":")
		if line == "" || colon < 0 {
			continue
		}
		key, value := trimSwiftWhitespace(line[:colon]), trimSwiftWhitespace(line[colon+1:])
		if !briefFrontKey.MatchString(key) {
			continue
		}
		if key == "question" {
			if value != "" {
				parsed.questions = append(parsed.questions, value)
			}
		} else {
			parsed.values[key] = value
		}
	}
	return parsed
}

// briefIsSet is whether a value keeps a {{#…}} section: not empty, false, no or 0.
func briefIsSet(value string, ok bool) bool {
	value = trimSwiftSpace(value)
	if !ok || value == "" {
		return false
	}
	return !slices.Contains([]string{"false", "no", "0"}, strings.ToLower(value))
}

// replaceBrief is text with each match of pattern, left to right, replaced by
// what with gives for its groups.
func replaceBrief(pattern *regexp.Regexp, text string, with func(groups []string) string) string {
	var out strings.Builder
	last := 0
	for _, match := range pattern.FindAllStringSubmatchIndex(text, -1) {
		out.WriteString(text[last:match[0]])
		groups := make([]string, len(match)/2)
		for i := range groups {
			if match[2*i] >= 0 {
				groups[i] = text[match[2*i]:match[2*i+1]]
			}
		}
		out.WriteString(with(groups))
		last = match[1]
	}
	out.WriteString(text[last:])
	return out.String()
}

// replaceSections is one pass over text's innermost sections, as the Mac's
// pattern finds them, left to right: an opening tag, then text with no
// opening tag in it, then the first closing tag of the same name.
func replaceSections(text string, keep func(kind, name string) bool) (string, bool) {
	var out strings.Builder
	found := false
	last, at := 0, 0
	for at < len(text) {
		next := strings.Index(text[at:], "{{")
		if next < 0 {
			break
		}
		at += next
		tag := briefOpeningTag.FindStringSubmatch(text[at:])
		if tag == nil {
			at++
			continue
		}
		content := at + len(tag[0])
		closing := "{{/" + tag[2] + "}}"
		end := strings.Index(text[content:], closing)
		// Where the next opening tag starts: the section must close before it.
		stop := -1
		for _, opener := range []string{"{{#", "{{^"} {
			if i := strings.Index(text[content:], opener); i >= 0 && (stop < 0 || i < stop) {
				stop = i
			}
		}
		if end < 0 || stop >= 0 && stop < end {
			at++
			continue
		}
		out.WriteString(text[last:at])
		if keep(tag[1], tag[2]) {
			out.WriteString(text[content : content+end])
		}
		at = content + end + len(closing)
		last, found = at, true
	}
	out.WriteString(text[last:])
	return out.String(), found
}

// checkBriefTemplate is whether text (front matter and all) is a brief a hire
// can fill: only known values, sections that pair up, and no slots.
func checkBriefTemplate(text string) error {
	parsed := parseBriefFrontMatter(text)
	_, err := resolveBrief(parsed.body, briefKnown(parsed.values), map[string]string{})
	return err
}

// briefKnown is every name a brief can fill: the hire's values and the front matter's keys.
func briefKnown(front map[string]string) map[string]bool {
	known := map[string]bool{}
	for _, name := range briefNames {
		known[name] = true
	}
	for key := range front {
		known[key] = true
	}
	return known
}

// resolveBrief is body with its sections resolved and its values filled: each
// value as it is, never read again for tags of its own. Refused, saying why, if
// a tag would be left in it.
func resolveBrief(body string, known map[string]bool, values map[string]string) (string, error) {
	text := briefComment.ReplaceAllLiteralString(body, "")
	if strings.Contains(text, "{{>") {
		return "", briefSlots
	}
	unknown := map[string]bool{}
	for _, pattern := range []*regexp.Regexp{briefSectionName, briefValue} {
		for _, match := range pattern.FindAllStringSubmatch(text, -1) {
			if !known[match[1]] {
				unknown[match[1]] = true
			}
		}
	}
	if len(unknown) > 0 {
		names := make([]string, 0, len(unknown))
		for name := range unknown {
			names = append(names, name)
		}
		slices.Sort(names)
		return "", briefUnknown(names)
	}
	text = replaceBrief(briefStandaloneTag, text, func(groups []string) string { return groups[1] })
	// Innermost first, until none is left.
	for {
		var found bool
		text, found = replaceSections(text, func(kind, name string) bool {
			value, ok := values[name]
			return (kind == "#") == briefIsSet(value, ok)
		})
		if !found {
			break
		}
	}
	// Checked before the values go in, which may hold braces of their own.
	if strings.Contains(briefValue.ReplaceAllLiteralString(text, ""), "{{") {
		return "", briefSections
	}
	text = replaceBrief(briefValue, text, func(groups []string) string { return values[groups[1]] })
	text = briefSpaceBeforeNL.ReplaceAllLiteralString(text, "\n")
	text = briefBlankLines.ReplaceAllLiteralString(text, "\n\n")
	return trimSwiftSpace(text) + "\n", nil
}

// MARK: The library

// briefLibrary is the briefs that come with the app: _base.md, the skeleton
// they share, and each brief's own front matter and sections, poured into it.
type briefLibrary struct {
	files fs.FS
}

// bundledBriefs is the app's own briefs, built in.
func bundledBriefs() fs.FS {
	sub, err := fs.Sub(agentdeck.Files, "Resources/briefs")
	if err != nil {
		return fstestEmpty{}
	}
	return sub
}

// fstestEmpty is a folder with nothing in it.
type fstestEmpty struct{}

func (fstestEmpty) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }

// briefTemplatesDir is the briefs you can edit: the app's own, copied there on
// first use and put back by "Restore defaults". Inside the folder hires'
// briefs are kept in, in a folder of its own.
func briefTemplatesDir() string { return filepath.Join(briefsKeptDir(), "templates") }

// briefsKeptDir is where a hire's brief is written when its desk isn't pinned
// to its own project and it isn't managed.
func briefsKeptDir() string { return filepath.Join(supportDir, "briefs") }

// briefLibraryFiles are the files that make the library: the skeleton and each brief.
func briefLibraryFiles() []string {
	names := []string{"_base.md"}
	for _, id := range briefLibraryIDs {
		names = append(names, id+".md")
	}
	return names
}

// editableBriefs is the library as you can edit it, in folder: each file the
// app has that isn't there yet is copied in, one you've changed is never
// replaced. Without the app's own files, or a folder to write in, the app's
// are used.
func editableBriefs(source fs.FS, folder string) briefLibrary {
	copyBriefDefaults(source, folder, false)
	if _, err := os.Stat(filepath.Join(folder, "_base.md")); err == nil {
		return briefLibrary{files: os.DirFS(folder)}
	}
	return briefLibrary{files: source}
}

// restoreBriefDefaults is "Restore defaults": the app's own files over yours,
// the library's only. Anything else in the folder is left as it is.
func restoreBriefDefaults(source fs.FS, folder string) int {
	return copyBriefDefaults(source, folder, true)
}

func copyBriefDefaults(source fs.FS, folder string, replacing bool) int {
	_ = os.MkdirAll(folder, 0o700)
	copied := 0
	for _, name := range briefLibraryFiles() {
		data, err := fs.ReadFile(source, name)
		if err != nil {
			continue
		}
		to := filepath.Join(folder, name)
		if _, err := os.Lstat(to); !replacing && err == nil {
			continue
		}
		// Whole or not at all, and never through a link someone put there.
		_ = os.Remove(to)
		if writeBriefPrivately(to, data) == nil {
			copied++
		}
	}
	return copied
}

// writeBriefPrivately writes a file only you can read, whole: a temporary file
// beside it, renamed over it.
func writeBriefPrivately(path string, data []byte) error {
	temporary := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+"."+newID())
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

// briefEntry is one brief the library lists.
type briefEntry struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Summary   string   `json:"summary"`
	Questions []string `json:"questions"`
}

// entries are the briefs that are there, in briefLibraryIDs's order.
func (l briefLibrary) entries() []briefEntry {
	entries := []briefEntry{}
	for _, id := range briefLibraryIDs {
		text, ok := l.source(id)
		if !ok {
			continue
		}
		parsed := parseBriefFrontMatter(text)
		name, ok := parsed.values["name"]
		if !ok {
			name = id
		}
		questions := parsed.questions
		if questions == nil {
			questions = []string{}
		}
		entries = append(entries, briefEntry{ID: id, Name: name, Summary: parsed.values["summary"], Questions: questions})
	}
	return entries
}

func (l briefLibrary) read(name string) (string, bool) {
	if l.files == nil {
		return "", false
	}
	data, err := fs.ReadFile(l.files, name)
	if err != nil || !utf8.Valid(data) {
		return "", false
	}
	// A brief you edited and saved with Windows' line ends (CRLF) reads as one
	// saved with plain ones, or its front matter and slots would go unseen.
	return strings.ReplaceAll(string(data), "\r\n", "\n"), true
}

func (l briefLibrary) source(id string) (string, bool) {
	if !slices.Contains(briefLibraryIDs, id) {
		return "", false
	}
	return l.read(id + ".md")
}

// template is the whole brief, as a picked file would be: its front matter,
// then the skeleton with its sections in, placeholders left to fill. False for
// an id that isn't one of the library's.
func (l briefLibrary) template(id string) (string, bool) {
	text, ok := l.source(id)
	base, okBase := l.read("_base.md")
	if !ok || !okBase {
		return "", false
	}
	parsed := parseBriefFrontMatter(text)
	// Its sections, each from its <!-- slot: name --> line to the next.
	slots := map[string]string{}
	name, named := "", false
	var lines []string
	keep := func() {
		if named {
			slots[name] = trimSwiftSpace(strings.Join(lines, "\n"))
		}
	}
	for _, line := range strings.Split(parsed.body, "\n") {
		if found := briefSlotLine.FindStringSubmatch(line); found != nil {
			keep()
			name, named, lines = found[1], true, nil
		} else {
			lines = append(lines, line)
		}
	}
	keep()
	var poured []string
	for _, line := range strings.Split(base, "\n") {
		if found := briefSlotWhole.FindStringSubmatch(line); found != nil {
			if slot := slots[found[1]]; slot != "" {
				poured = append(poured, slot)
			}
			continue
		}
		// A slot inside a line.
		poured = append(poured, replaceBrief(briefSlotInline, line, func(groups []string) string { return slots[groups[1]] }))
	}
	return parsed.head + strings.Join(poured, "\n"), true
}

// MARK: Filling a brief in

// briefContext is what a hire's brief is filled in with, besides its role.
type briefContext struct {
	// Where the hire starts, and where its brief is written.
	folder string
	// Started in a desk's own project, so it asks you to confirm that one
	// rather than which project to work on.
	pinned bool
	// The answers to the role's questions, in their order; nil when the agent asks them itself, first.
	answers    []string
	date       time.Time
	briefFile  string
	statusFile string
	reportFile string
	// The task, written out by whoever hired (the GM): it is the job, in place of the role's questions.
	task string
}

const (
	briefAnswerLimit = 2000
	briefTaskLimit   = 8000
	briefApproval    = "The user hired you for this; don't wait for plan approval unless this brief says to ask."
	briefDoneSignal  = "JOB DONE"
)

// briefTask is a task as it goes into the brief: without control characters
// but line ends and tabs, trimmed, and not too long.
func briefTask(text string) string {
	return prefixRunes(trimSwiftSpace(withoutControls(text, "\n\t")), briefTaskLimit)
}

// briefAnswer is an answer as it goes into the brief: without control
// characters but line ends, trimmed, and each further line indented to stay in its item.
func briefAnswer(text string) string {
	clean := prefixRunes(trimSwiftSpace(withoutControls(text, "\n")), briefAnswerLimit)
	lines := strings.Split(clean, "\n")
	for i, line := range lines {
		lines[i] = trimSwiftWhitespace(line)
	}
	return strings.Join(lines, "\n  ")
}

// tildePath is path with ~ for your home folder, as the Mac shows paths.
func tildePath(path string) string {
	if homeDir == "" || homeDir == "/" {
		return path
	}
	if path == homeDir {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, homeDir+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rest
	}
	return path
}

// briefSource is where the role's brief comes from: the one it was kept with, or its default.
func (r Role) briefSource() RoleBrief {
	if r.Brief != nil {
		return *r.Brief
	}
	return *r.defaultBrief()
}

// briefMissing is a library brief this copy hasn't.
type briefMissing string

func (id briefMissing) Error() string {
	return "The brief " + string(id) + " isn't in this copy of Smolcorp."
}

// briefTemplate is the brief's text before it's filled in, or "" and false for
// a role started on its own instructions. An error if its library brief isn't there.
func (r Role) briefTemplate(library briefLibrary) (string, bool, error) {
	source := r.briefSource()
	switch source.Kind {
	case "file":
		return source.Text, true, nil
	case "library":
		template, ok := library.template(source.ID)
		if !ok {
			return "", false, briefMissing(source.ID)
		}
		return template, true, nil
	}
	return "", false, nil
}

// briefReport is the report file a brief names in its front matter, if any,
// as its base name: REVIEW for REVIEW.md.
func briefReport(template string) string {
	name := parseBriefFrontMatter(template).values["report"]
	if briefReportName.MatchString(name) {
		return name
	}
	return ""
}

// renderBrief is the brief filled in for a hire: its folder and branch, the
// answers to its questions (or the questions to ask first), how it runs, the
// pre-approval, and where it reports.
func (r Role) renderBrief(template string, context briefContext) (string, error) {
	parsed := parseBriefFrontMatter(template)
	agent := Agent(r.Agent)
	if !slices.Contains(agents, agent) {
		agent = Claude
	}
	values := map[string]string{}
	for key, value := range parsed.values {
		values[key] = value
	}
	values["role"] = r.Name
	values["date"] = context.date.Format("2006-01-02")
	values["project_folder"] = tildePath(context.folder)
	values["branch"] = briefBranch(context.folder)
	values["agent"] = agent.Name()
	values["model"] = r.Model
	values["effort"] = ""
	if agent.HasEffortDial() {
		values["effort"] = r.Effort
	}
	values["approval"] = briefApproval
	values["brief_file"] = tildePath(context.briefFile)
	values["status_file"] = tildePath(context.statusFile)
	values["report_file"] = ""
	if context.reportFile != "" {
		values["report_file"] = tildePath(context.reportFile)
	}
	done, ok := parsed.values["done_signal"]
	if !briefIsSet(done, ok) {
		values["done_signal"] = briefDoneSignal
	}
	if readOnly, ok := parsed.values["read_only"]; ok {
		values["read_only"] = readOnly
	} else {
		values["read_only"] = "false"
	}
	values["job"] = r.briefJob(context, tildePath(context.statusFile))
	return resolveBrief(parsed.body, briefKnown(parsed.values), values)
}

// briefJob is "The job": the role's questions with your answers, or, with none
// given, the questions the agent asks you first, as a hire always has.
func (r Role) briefJob(context briefContext, statusFile string) string {
	shown := tildePath(context.folder)
	if task := briefTask(context.task); task != "" {
		return "The task, from whoever hired you:\n\n" + task
	}
	if context.answers == nil {
		project := "Which project should you work on: a folder on this computer, or a GitHub repository?"
		if context.pinned {
			project = "You've started in " + shown + ", this desk's project. Is that the one to work on?"
		}
		asked := []string{"1. " + project}
		for i, question := range r.Questions {
			asked = append(asked, strconv.Itoa(i+2)+". "+question)
		}
		return "Before anything else, ask the user these questions in your first reply, all together, and wait for " +
			"their answers. Don't read code, run commands or start work until they've answered.\n\n" + strings.Join(asked, "\n") + "\n\n" +
			"Once they've answered, write each question with its answer at the top of `" + statusFile + "`, then start."
	}
	answered := []string{"- **Which project should you work on?** `" + shown + "`"}
	for i, question := range r.Questions {
		answer := ""
		if i < len(context.answers) {
			answer = briefAnswer(context.answers[i])
		}
		if answer == "" {
			answer = "*No answer: use your judgement, and say what you chose in `" + statusFile + "`.*"
		}
		answered = append(answered, "- **"+question+"** "+answer)
	}
	return "The user answered these questions as they hired you:\n\n" + strings.Join(answered, "\n")
}

// briefMessage is what the agent is first told when its brief is written: to
// read it, then to ask its questions first or, answered already, to start.
func (r Role) briefMessage(path string, asks bool) string {
	shown := "`" + tildePath(path) + "`"
	if asks {
		return "Read " + shown + " and follow it: it's your brief, as my " + r.Name + ". As it says, your first reply must only ask me its " +
			"questions, all together; wait for my answers before you read code or run commands."
	}
	return "Read " + shown + " and start: it's your brief, as my " + r.Name + ", with my answers to its questions."
}

// firstMessageAnswered is what the agent is first told when it was hired with
// your answers already, for a role on its own instructions: the folder it's
// in, the answers, then the role's instructions (Role.firstMessage(in:answers:)).
func (r Role) firstMessageAnswered(folder string, answers []string) string {
	var answered []string
	for i, question := range r.Questions {
		answer := ""
		if i < len(answers) {
			answer = briefAnswer(answers[i])
		}
		if answer == "" {
			answer = "(No answer: use your judgement.)"
		}
		answered = append(answered, "- "+question+" "+answer)
	}
	then := "Ask me what to do."
	if r.Instructions != "" {
		then = "As my " + r.Name + ": " + r.Instructions
	}
	text := "You've started in " + tildePath(folder) + ", the project to work on."
	if len(answered) > 0 {
		text += " I've answered your questions already:\n\n" + strings.Join(answered, "\n")
	}
	return text + "\n\n" + then
}

// MARK: Where a brief goes

// briefPlace is where a hire's brief goes. At a desk pinned to its own
// project, into that folder: BRIEF.md, with STATUS.md beside it (and its
// report, if it has one), or with any of them taken, BRIEF-<role>.md and the
// rest, then BRIEF-<role>-2.md and on. Anywhere else, so a shared folder such
// as ~/Projects stays clean, into Smolcorp's own briefs folder:
// <desk>-<role>-<yyyyMMdd-HHmm>.md, with <…>-STATUS.md beside it, then
// <…>-2.md and on. A managed hire's (one with a worktree of its own) never
// goes in its worktree, which is shared git state: BRIEF.md and STATUS.md go
// in <support>/hires/<id>/, a folder of its own, and the first message names
// where. A file that's there is never written over, nor named to the agent as its own.
type briefPlace struct {
	kind   string // "project", "kept" or "managed"
	folder string
	// The role, for a project's names; the stem, for those kept.
	role, stem string
}

// briefNamesOf are a brief's files: the brief, its status file and its report, if it has one.
type briefNamesOf struct {
	brief, status, report string
}

const briefMostTries = 99

// placeBrief is where a hire's brief goes: into managed for a managed hire,
// into folder at a desk pinned to it, otherwise into kept, named for the desk
// (as setup counts them, from 0, or nil), the role and the minute.
func placeBrief(role, folder string, pinned bool, desk *int, date time.Time, kept, managed string) briefPlace {
	if managed != "" {
		return briefPlace{kind: "managed", folder: managed}
	}
	if pinned {
		return briefPlace{kind: "project", folder: folder, role: role}
	}
	name := "desk"
	if desk != nil {
		name = strconv.Itoa(*desk)
	}
	return briefPlace{kind: "kept", folder: kept, stem: name + "-" + role + "-" + date.Format("20060102-1504")}
}

func (p briefPlace) names(report string, n int) briefNamesOf {
	var file func(base string) string
	var names briefNamesOf
	switch p.kind {
	case "project":
		tail := ""
		if n == 1 {
			tail = "-" + p.role
		} else if n > 1 {
			tail = fmt.Sprintf("-%s-%d", p.role, n)
		}
		file = func(base string) string { return filepath.Join(p.folder, base+tail+".md") }
		names = briefNamesOf{brief: file("BRIEF"), status: file("STATUS")}
	case "managed":
		tail := ""
		if n > 0 {
			tail = "-" + strconv.Itoa(n+1)
		}
		file = func(base string) string { return filepath.Join(p.folder, base+tail+".md") }
		names = briefNamesOf{brief: file("BRIEF"), status: file("STATUS")}
	default:
		name := p.stem
		if n > 0 {
			name += "-" + strconv.Itoa(n+1)
		}
		file = func(part string) string { return filepath.Join(p.folder, name+"-"+part+".md") }
		names = briefNamesOf{brief: filepath.Join(p.folder, name+".md"), status: file("STATUS")}
	}
	if report != "" {
		names.report = file(report)
	}
	return names
}

// briefNotWritten is a brief that couldn't be written where it goes.
type briefNotWritten string

func (folder briefNotWritten) Error() string {
	return "Couldn't write the brief into " + tildePath(string(folder)) + "."
}

// writeBrief writes the brief render makes for the names it's given, under
// the first names none of whose files is there. The brief is created only if
// it isn't there, so even a file that turns up meanwhile is left as it is.
// Smolcorp's own briefs folder is made if it isn't there, for you alone; a
// project's folder never is.
func writeBrief(place briefPlace, report string, render func(briefNamesOf) (string, error)) (briefNamesOf, error) {
	if place.kind != "project" {
		_ = os.MkdirAll(place.folder, 0o700)
	}
	there := func(path string) bool { _, err := os.Lstat(path); return err == nil }
	for n := 0; n <= briefMostTries; n++ {
		names := place.names(report, n)
		if there(names.brief) || there(names.status) || names.report != "" && there(names.report) {
			continue
		}
		text, err := render(names)
		if err != nil {
			return briefNamesOf{}, err
		}
		file, err := os.OpenFile(names.brief, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return briefNamesOf{}, briefNotWritten(place.folder)
		}
		_, err = file.WriteString(text)
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return briefNamesOf{}, briefNotWritten(place.folder)
		}
		return names, nil
	}
	return briefNamesOf{}, briefNotWritten(place.folder)
}

// MARK: Making one

// briefMade is a hire's brief: its files, its text, and what the agent is
// first told (to read the brief, and where it is).
type briefMade struct {
	names        briefNamesOf
	markdown     string
	firstMessage string
}

// briefRequest is what a hire's brief is made from.
type briefRequest struct {
	role    Role
	library briefLibrary
	folder  string
	pinned  bool
	desk    *int
	// nil when the agent asks its questions itself.
	answers []string
	task    string
	// The folder a managed hire's files go in (hireFolder), or "".
	managed string
	date    time.Time
	// Smolcorp's own briefs folder (briefsKeptDir).
	kept string
	// Off for a preview: nothing is written, and the names are those a first hire would get.
	write bool
}

// makeBrief is the one way a hire's brief is made and placed, for every way in
// (the office, the phone, setup and the GM): the role's brief, with the base
// rules and the task text, filled in, and, unless it is only previewed,
// written where placeBrief says. Nil for a role on its own instructions,
// which has no brief.
func makeBrief(r briefRequest) (*briefMade, error) {
	template, ok, err := r.role.briefTemplate(r.library)
	if err != nil || !ok {
		return nil, err
	}
	place := placeBrief(r.role.ID, r.folder, r.pinned, r.desk, r.date, r.kept, r.managed)
	report := briefReport(template)
	text := ""
	render := func(names briefNamesOf) (string, error) {
		var err error
		text, err = r.role.renderBrief(template, briefContext{folder: r.folder, pinned: r.pinned, answers: r.answers, date: r.date,
			briefFile: names.brief, statusFile: names.status, reportFile: names.report, task: r.task})
		return text, err
	}
	var names briefNamesOf
	if r.write {
		if names, err = writeBrief(place, report, render); err != nil {
			return nil, err
		}
	} else {
		names = place.names(report, 0)
		if _, err := render(names); err != nil {
			return nil, err
		}
	}
	asks := r.answers == nil && r.task == ""
	return &briefMade{names: names, markdown: text, firstMessage: r.role.briefMessage(names.brief, asks)}, nil
}
