package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
)

// The .claude.json editor (Phase 6 T602, with T705): pre-trust of a hire's
// worktree and the approval of an API-key hire's key, as the Mac app does them.
// Nothing here is wired in yet: the Linux hire engine calls preTrust after making
// the worktree and before the hire's window starts.

// claudeJSONRefusal: the file isn't the shape Claude Code writes, so nothing
// is changed.
var claudeJSONRefusal = errors.New("Smolcorp doesn't know how this account keeps its settings in .claude.json, so it changed nothing")

// claudeJSON is the account's .claude.json for the config folder ("" for
// ~/.claude's, which keeps it in the home folder).
func claudeJSON(configDir string) string {
	if configDir == "" {
		return filepath.Join(homeDir, ".claude.json")
	}
	return filepath.Join(configDir, ".claude.json")
}

// keyTail is the part of a key Claude Code records its answer under: the last
// 20 characters (runes, as the Mac counts them; it trims first). "" for no key.
func keyTail(key string) string {
	key = strings.TrimSpace(key)
	if runes := []rune(key); len(runes) > 20 {
		key = string(runes[len(runes)-20:])
	}
	return key
}

// keyApproval is what an API-key hire gives preTrust: a pointer to its key's
// tail, which is "" if the key is missing. A subscription hire gives nil.
func keyApproval(key string) *string {
	tail := keyTail(key)
	return &tail
}

// trusting is data, an account's .claude.json, with folder trusted: only
// projects[folder].hasTrustDialogAccepted set, by editing the text in that one
// place, so every other byte stays as Claude Code wrote it. A shape that isn't
// the one Claude Code writes is refused.
func trusting(folder string, data []byte) ([]byte, error) {
	text := string(data)
	root, ok := jsonValue(text, 0)
	if !ok || text[root.start] != '{' {
		return nil, claudeJSONRefusal
	}
	key := jsonQuoted(folder)
	var edited string
	if projects, found, ok := jsonMember(text, root, "projects"); !ok {
		return nil, claudeJSONRefusal
	} else if !found {
		edited = jsonInsert(text, root, `"projects": {`+key+`: {"hasTrustDialogAccepted": true}}`)
	} else if text[projects.start] != '{' {
		return nil, claudeJSONRefusal
	} else if project, found, ok := jsonMember(text, projects, folder); !ok {
		return nil, claudeJSONRefusal
	} else if !found {
		edited = jsonInsert(text, projects, key+`: {"hasTrustDialogAccepted": true}`)
	} else if text[project.start] != '{' {
		return nil, claudeJSONRefusal
	} else if trust, found, ok := jsonMember(text, project, "hasTrustDialogAccepted"); !ok {
		return nil, claudeJSONRefusal
	} else if found {
		edited = text[:trust.start] + "true" + text[trust.end:]
	} else {
		edited = jsonInsert(text, project, `"hasTrustDialogAccepted": true`)
	}
	return checkedEdit(data, edited, func(root map[string]any) {
		projects, _ := root["projects"].(map[string]any)
		if projects == nil {
			projects = map[string]any{}
			root["projects"] = projects
		}
		project, _ := projects[folder].(map[string]any)
		if project == nil {
			project = map[string]any{}
			projects[folder] = project
		}
		project["hasTrustDialogAccepted"] = true
	})
}

// approving is data with tail first among customApiKeyResponses.approved
// (Claude Code's "yes" to "use this API key?"), edited in that one place as
// trusting is. Already there, it is left as it is.
func approving(tail string, data []byte) ([]byte, error) {
	text := string(data)
	root, ok := jsonValue(text, 0)
	if !ok || text[root.start] != '{' || tail == "" {
		return nil, claudeJSONRefusal
	}
	quoted := jsonQuoted(tail)
	var edited string
	if responses, found, ok := jsonMember(text, root, "customApiKeyResponses"); !ok {
		return nil, claudeJSONRefusal
	} else if !found {
		edited = jsonInsert(text, root, `"customApiKeyResponses": {"approved": [`+quoted+`]}`)
	} else if text[responses.start] != '{' {
		return nil, claudeJSONRefusal
	} else if approved, found, ok := jsonMember(text, responses, "approved"); !ok {
		return nil, claudeJSONRefusal
	} else if !found {
		edited = jsonInsert(text, responses, `"approved": [`+quoted+`]`)
	} else if text[approved.start] != '[' {
		return nil, claudeJSONRefusal
	} else {
		var list []any
		if json.Unmarshal([]byte(text[approved.start:approved.end]), &list) != nil {
			return nil, claudeJSONRefusal
		}
		if slices.Contains(list, any(tail)) {
			return data, nil
		}
		edited = jsonInsert(text, approved, quoted)
	}
	return checkedEdit(data, edited, func(root map[string]any) {
		responses, _ := root["customApiKeyResponses"].(map[string]any)
		if responses == nil {
			responses = map[string]any{}
			root["customApiKeyResponses"] = responses
		}
		approved, _ := responses["approved"].([]any)
		responses["approved"] = append([]any{tail}, approved...)
	})
}

// checkedEdit is edited as bytes, once it's checked to be before with only
// what change does to before's decoded form (numbers as written).
func checkedEdit(before []byte, edited string, change func(root map[string]any)) ([]byte, error) {
	decode := func(data []byte) map[string]any {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		var root map[string]any
		if decoder.Decode(&root) != nil || decoder.More() {
			return nil
		}
		return root
	}
	old, now := decode(before), decode([]byte(edited))
	if old == nil || now == nil {
		return nil, claudeJSONRefusal
	}
	change(old)
	if !reflect.DeepEqual(old, now) {
		return nil, claudeJSONRefusal
	}
	return []byte(edited), nil
}

// claudeJSONLock keeps this process's preTrust calls on a file one at a time:
// each is a read, an edit and a write, and two at once would lose one's change.
var claudeJSONLock sync.Mutex

// preTrust trusts folder in file, and when tail isn't nil approves that key's
// tail too (an API-key hire; a nil tail is a subscription hire, and a tail of
// "" is a hire whose key is missing, which is refused with nothing changed, as
// the Mac refuses, since Claude Code would stop at "use this API key?"), written whole in one go (0600) and read back. Claude Code sessions
// rewrite this file all the time, so if one wrote over the change it is made
// once more from the file as it is then; no copy is ever put back over theirs.
// A symlink (a dotfiles manager's) is written through to its target. A missing
// file, or one not in the shape Claude Code writes, is refused. Nothing in the
// file is logged or said back.
func preTrust(folder, file string, tail *string) error {
	if tail != nil && *tail == "" {
		return errors.New("this hire has no API key, so Smolcorp changed nothing: Claude Code would stop at \"use this API key?\"")
	}
	claudeJSONLock.Lock()
	defer claudeJSONLock.Unlock()
	if resolved, err := filepath.EvalSymlinks(file); err == nil {
		file = resolved
	}
	before, err := os.ReadFile(file)
	if err != nil {
		return claudeJSONRefusal
	}
	for range 2 {
		written, err := trusting(folder, before)
		if err == nil && tail != nil {
			written, err = approving(*tail, written)
		}
		if err != nil {
			return err
		}
		if !bytes.Equal(written, before) {
			if err := writeClaudeJSON(file, written); err != nil {
				return fmt.Errorf("couldn't write %s", file)
			}
		} else if err := os.Chmod(file, 0o600); err != nil {
			return fmt.Errorf("couldn't write %s", file)
		}
		now, err := os.ReadFile(file)
		if err == nil && isTrusted(folder, now) && (tail == nil || isApproved(*tail, now)) {
			return nil
		}
		if err != nil {
			break
		}
		before = now
	}
	return fmt.Errorf("%s lost the change twice as it was written: another Claude Code session keeps rewriting it. Hire again in a moment", filepath.Base(file))
}

// writeClaudeJSON writes data to file through a temporary file beside it that
// only this call can have made (os.CreateTemp: new, exclusive, 0600, so a stale
// file or a planted symlink is never written through), renames it over file,
// and makes the finished file 0600.
func writeClaudeJSON(file string, data []byte) error {
	out, err := os.CreateTemp(filepath.Dir(file), filepath.Base(file)+".*.tmp")
	if err != nil {
		return err
	}
	temp := out.Name()
	_, err = out.Write(data)
	if closed := out.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = os.Rename(temp, file)
	}
	if err != nil {
		os.Remove(temp)
		return err
	}
	return os.Chmod(file, 0o600)
}

func isTrusted(folder string, data []byte) bool {
	var root struct {
		Projects map[string]struct {
			Trusted bool `json:"hasTrustDialogAccepted"`
		} `json:"projects"`
	}
	return json.Unmarshal(data, &root) == nil && root.Projects[folder].Trusted
}

func isApproved(tail string, data []byte) bool {
	var root struct {
		Responses struct {
			Approved []any `json:"approved"`
		} `json:"customApiKeyResponses"`
	}
	return json.Unmarshal(data, &root) == nil && slices.Contains(root.Responses.Approved, any(tail))
}

// The text editing below finds a value's place in a JSON document without
// rewriting it, so an edit keeps every other byte. A span is [start, end).
type jsonSpan struct{ start, end int }

func jsonSkipSpace(text string, i int) int {
	for i < len(text) && strings.IndexByte(" \n\r\t", text[i]) >= 0 {
		i++
	}
	return i
}

// jsonStringEnd is the index after the string starting at i, or -1.
func jsonStringEnd(text string, i int) int {
	for i++; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return -1
}

// jsonValue is the value starting at i, after any space; false if it isn't whole.
func jsonValue(text string, i int) (jsonSpan, bool) {
	start := jsonSkipSpace(text, i)
	if start >= len(text) {
		return jsonSpan{}, false
	}
	switch text[start] {
	case '"':
		end := jsonStringEnd(text, start)
		return jsonSpan{start, end}, end > 0
	case '{', '[':
		closing := byte('}')
		if text[start] == '[' {
			closing = ']'
		}
		i := jsonSkipSpace(text, start+1)
		if i < len(text) && text[i] == closing {
			return jsonSpan{start, i + 1}, true
		}
		for i < len(text) {
			if closing == '}' {
				if text[i] != '"' {
					return jsonSpan{}, false
				}
				keyEnd := jsonStringEnd(text, i)
				if keyEnd < 0 {
					return jsonSpan{}, false
				}
				i = jsonSkipSpace(text, keyEnd)
				if i >= len(text) || text[i] != ':' {
					return jsonSpan{}, false
				}
				i++
			}
			member, ok := jsonValue(text, i)
			if !ok {
				return jsonSpan{}, false
			}
			i = jsonSkipSpace(text, member.end)
			if i >= len(text) {
				return jsonSpan{}, false
			}
			if text[i] == closing {
				return jsonSpan{start, i + 1}, true
			}
			if text[i] != ',' {
				return jsonSpan{}, false
			}
			i = jsonSkipSpace(text, i+1)
		}
		return jsonSpan{}, false
	}
	i = start
	for i < len(text) && strings.IndexByte(",}] \n\r\t", text[i]) < 0 {
		i++
	}
	return jsonSpan{start, i}, i > start
}

// jsonMember is the value of the member name directly in the object at object;
// found is false if it has none, and ok false if the object isn't well formed.
func jsonMember(text string, object jsonSpan, name string) (value jsonSpan, found, ok bool) {
	i := jsonSkipSpace(text, object.start+1)
	for i < object.end && text[i] == '"' {
		keyEnd := jsonStringEnd(text, i)
		if keyEnd < 0 {
			return jsonSpan{}, false, false
		}
		var key string
		json.Unmarshal([]byte(text[i:keyEnd]), &key)
		j := jsonSkipSpace(text, keyEnd)
		if j >= len(text) || text[j] != ':' {
			return jsonSpan{}, false, false
		}
		member, valid := jsonValue(text, j+1)
		if !valid {
			return jsonSpan{}, false, false
		}
		if key == name {
			return member, true, true
		}
		i = jsonSkipSpace(text, member.end)
		if i >= object.end || text[i] != ',' {
			return jsonSpan{}, false, true
		}
		i = jsonSkipSpace(text, i+1)
	}
	return jsonSpan{}, false, true
}

// jsonInsert is text with member ("key": value, or an array's element) first
// in the object or array at in, laid out as its own first member is.
func jsonInsert(text string, in jsonSpan, member string) string {
	afterBracket := in.start + 1
	first := jsonSkipSpace(text, afterBracket)
	space := text[afterBracket:first]
	if text[first] == '}' || text[first] == ']' {
		return text[:afterBracket] + member + text[afterBracket:]
	}
	return text[:afterBracket] + space + member + "," + space + text[first:]
}

// jsonQuoted is text as a JSON string, with its slashes and angle brackets as they are.
func jsonQuoted(text string) string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.Encode(text)
	return strings.TrimSuffix(out.String(), "\n")
}
