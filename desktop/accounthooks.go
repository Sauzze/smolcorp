package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"
)

// The hooks in the other Claude accounts you confirmed (HOOKREFRESH1, SIERRA's ruling). refreshHooks brings
// ~/.claude's entries up to date at each start; an account in another folder (CLAUDE_CONFIG_DIR) may still hold the
// entries the Swift engine wrote, which run <support>/report.sh, a file a sandboxed hire can't read, so a hire on that
// account never reports. At each start, each confirmed account's settings.json gets this copy's `smolcorp hook` in
// place of every entry of Smolcorp's own there (isOurs: report.sh, or this program at another path).
//
// These files are yours, so the change is the least it can be: only the command's text of Smolcorp's own entries is
// replaced, byte for byte where it stands, and every other byte (your hooks, your keys, your spacing and order) stays
// as it was. Nothing is added: an account without Smolcorp's entries is left as it is, as is one whose entries name
// another office's port. Before the first change, a copy of the file as it was is kept beside it, named for the time;
// then the new file takes its place whole (a file beside it, renamed), so a crash leaves the old file or the new one.
// A file that can't be read or isn't valid JSON is never touched, and setup says so on that account's line.

// errAccountSettingsChanged: the file changed while it was being refreshed, so it's left to the next start.
var errAccountSettingsChanged = errors.New("the account's settings changed while Smolcorp read them")

// refreshAccountHooks refreshes the entries in every confirmed account's settings.json but ~/.claude's, which
// refreshHooks has. A test copy refreshes none: they're the real Smolcorp's.
func (d *Deck) refreshAccountHooks() {
	if leavesAgentsAlone() {
		return
	}
	d.mu.Lock()
	saved := slices.Clone(d.config.ClaudeProfiles)
	d.mu.Unlock()
	for _, p := range claudeAccountList(saved) {
		// Never a folder in Smolcorp's own, a desk's Claude folder say: its files are the core's, sealed or not.
		if p.Dir != claudeDefaultProfile().Dir && !isInside(supportDir, p.Dir) {
			_ = refreshAccountSettings(filepath.Join(p.Dir, "settings.json"), time.Now())
		}
	}
}

// isInside: path is folder or in it, by their names.
func isInside(folder, path string) bool {
	rel, err := filepath.Rel(folder, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// commandSpan is where a handler's command stands in a settings file: its JSON string from quote to quote, and its
// text.
type commandSpan struct {
	start, end int
	command    string
}

// handlerCommands is every handler's command in a Claude Code settings file, where it stands: the string at
// hooks → <event> → [i] → hooks → [j] → command. It reads the file token by token and changes nothing.
func handlerCommands(data []byte) ([]commandSpan, error) {
	type level struct {
		object, wantKey bool
		key             string
	}
	var stack []level
	var spans []commandSpan
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	for {
		before := int(decoder.InputOffset())
		token, err := decoder.Token()
		if err == io.EOF {
			return spans, nil
		}
		if err != nil {
			return nil, err
		}
		if delim, ok := token.(json.Delim); ok && (delim == '{' || delim == '[') {
			stack = append(stack, level{object: delim == '{', wantKey: delim == '{'})
			continue
		} else if ok {
			stack = stack[:len(stack)-1]
		} else if top := len(stack) - 1; top >= 0 && stack[top].wantKey {
			stack[top].key, _ = token.(string)
			stack[top].wantKey = false
			continue
		} else if command, ok := token.(string); ok && isHandlerCommand(len(stack), func(i int) (bool, string) { return stack[i].object, stack[i].key }) {
			// Between the key and its value lie only spaces and the colon: the first quote opens the string.
			start := before + bytes.IndexByte(data[before:], '"')
			spans = append(spans, commandSpan{start: start, end: int(decoder.InputOffset()), command: command})
		}
		// A value is done: the object it's in wants its next key.
		if top := len(stack) - 1; top >= 0 && stack[top].object {
			stack[top].wantKey = true
		}
	}
}

// isHandlerCommand: the levels open are root{hooks} → {event} → [ ] → {hooks} → [ ] → {command}.
func isHandlerCommand(depth int, at func(int) (bool, string)) bool {
	if depth != 6 {
		return false
	}
	for i, want := range []struct {
		object bool
		key    string
	}{{true, "hooks"}, {true, ""}, {false, ""}, {true, "hooks"}, {false, ""}, {true, "command"}} {
		object, key := at(i)
		if object != want.object || (want.key != "" && key != want.key) {
			return false
		}
	}
	return true
}

// accountSettingsText is the file's text, and an error when it can't be read or isn't one valid JSON object.
func accountSettingsText(file string) ([]byte, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, errUnreadable
	}
	var settings map[string]any
	if !json.Valid(data) || json.Unmarshal(data, &settings) != nil || settings == nil {
		return nil, errUnreadable
	}
	return data, nil
}

// refreshedAccountSettings is data with this copy's command in place of every entry of Smolcorp's own that differs,
// and whether anything changed. Left as it is when one of Smolcorp's entries names another office's port.
func refreshedAccountSettings(data []byte) ([]byte, bool, error) {
	spans, err := handlerCommands(data)
	if err != nil {
		return nil, false, errUnreadable
	}
	want := hookCommand(Claude)
	quoted, err := encodeSettingsValue(want)
	if err != nil {
		return nil, false, err
	}
	var out bytes.Buffer
	last, changed := 0, false
	for _, span := range spans {
		if !isOurs(map[string]any{"command": span.command}) || span.command == want {
			continue
		}
		if port := portInCommand(span.command); port != 0 && port != Port {
			return data, false, nil
		}
		out.Write(data[last:span.start])
		out.Write(quoted)
		last, changed = span.end, true
	}
	if !changed {
		return data, false, nil
	}
	out.Write(data[last:])
	if !sameButOurCommands(data, out.Bytes()) {
		return nil, false, fmt.Errorf("the refreshed settings differ in more than Smolcorp's entries")
	}
	return out.Bytes(), true, nil
}

// encodeSettingsValue is value in JSON as encodeSettings writes it: no HTML escapes, no newline after.
func encodeSettingsValue(value string) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}

// sameButOurCommands checks the edit as a whole, read back: before and after are the same settings, but for the
// command of Smolcorp's own entries.
func sameButOurCommands(before, after []byte) bool {
	read := func(data []byte) any {
		var settings any
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if decoder.Decode(&settings) != nil {
			return nil
		}
		top, _ := settings.(map[string]any)
		hooks, _ := top["hooks"].(map[string]any)
		for _, groups := range hooks {
			list, _ := groups.([]any)
			for _, group := range list {
				g, _ := group.(map[string]any)
				handlers, _ := g["hooks"].([]any)
				for _, handler := range handlers {
					if h, ok := handler.(map[string]any); ok && isOurs(h) {
						h["command"] = "smolcorp's"
					}
				}
			}
		}
		return settings
	}
	old, refreshed := read(before), read(after)
	return old != nil && reflect.DeepEqual(old, refreshed)
}

// refreshAccountSettings refreshes Smolcorp's entries in file, an account's settings.json: nothing when it isn't
// there or holds nothing to change, and errUnreadable, the file untouched, when it can't be read or isn't valid
// JSON. A link is followed, so the file it leads to is the one that changes, as writeSettings does.
func refreshAccountSettings(file string, now time.Time) error {
	if _, err := os.Lstat(file); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	real, err := filepath.EvalSymlinks(file)
	if err != nil {
		return errUnreadable
	}
	info, err := os.Stat(real)
	if err != nil || !info.Mode().IsRegular() {
		return errUnreadable
	}
	data, err := accountSettingsText(real)
	if err != nil {
		return err
	}
	refreshed, changed, err := refreshedAccountSettings(data)
	if err != nil || !changed {
		return err
	}
	if err := keepTimedBackup(real, data, info.Mode().Perm(), now); err != nil {
		return err
	}
	return replaceIfUnchanged(real, data, refreshed, info.Mode().Perm())
}

// keepTimedBackup writes data beside file, under a name for the time (settings.json.smolcorp-20261115-071500.bak),
// whole, and never over a file already there.
func keepTimedBackup(file string, data []byte, mode fs.FileMode, now time.Time) error {
	base := file + ".smolcorp-" + now.Format("20060102-150405")
	for i := 1; i <= 100; i++ {
		name := base + ".bak"
		if i > 1 {
			name = fmt.Sprintf("%s-%d.bak", base, i)
		}
		if _, err := os.Lstat(name); errors.Is(err, fs.ErrNotExist) {
			return keepFirst(name, data, mode)
		}
	}
	return fmt.Errorf("no free name for a backup of %s", file)
}

// replaceIfUnchanged puts refreshed in file's place whole, through a file beside it renamed over it, if file still
// holds was: one you changed meanwhile is left as you made it.
func replaceIfUnchanged(file string, was, refreshed []byte, mode fs.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(file), filepath.Base(file)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name()) // gone once renamed; left behind only by a failure
	if _, err := temp.Write(refreshed); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temp.Name(), mode); err != nil {
		return err
	}
	if now, err := os.ReadFile(file); err != nil || !bytes.Equal(now, was) {
		return errAccountSettingsChanged
	}
	return os.Rename(temp.Name(), file)
}

// accountSettingsProblem is what setup says on an account's line when its settings.json is there but can't be read
// or isn't valid JSON, which Smolcorp leaves untouched; "" otherwise.
func accountSettingsProblem(dir string) string {
	file := filepath.Join(dir, "settings.json")
	if _, err := os.Lstat(file); errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if _, err := accountSettingsText(file); err != nil {
		return "Its settings.json can't be read or isn't valid JSON, so Smolcorp leaves it as it is. Fix it, and Smolcorp's hooks in it are brought up to date at the next start."
	}
	return ""
}
