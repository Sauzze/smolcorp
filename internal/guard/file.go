package guard

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// File is hire-guard.json, in the app's support folder: the groups every
// hire is refused. Since 1.26 the app holds its own copy and writes the file
// from it (Settings); a desk's guard gets that copy in its command line, and
// the file, read at each command, only adds to it (Held). An edit by hand
// then only adds, and the next Settings save drops it.
type File struct {
	Groups []Group `json:"groups"`
}

// Load is the groups in the file at path, or, if it isn't there or isn't
// a guard file this can read, the default ones and what's wrong with it. A
// broken file falls back to the default, never to nothing (the hook goes
// further and refuses everything while the file is broken). It reads the
// file with ReadFile, so a link, a pipe, a device, a hard link or a file over
// 64 KiB is a broken file, at once.
func Load(path string) (groups []Group, problem string) {
	if path == "" {
		return DefaultGroups(), ""
	}
	data, err := readGuardFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultGroups(), ""
		}
		return DefaultGroups(), "can't read it: " + err.Error()
	}
	groups, problem = Parse(data)
	if problem != "" {
		return DefaultGroups(), problem
	}
	return groups, ""
}

// Parse reads a guard file's text, or says why it can't: it must be JSON
// (and nothing after it) with at least one group, each with a name, an
// "on" (a group left without one would be silently off) and only rules with
// words in.
func Parse(data []byte) ([]Group, string) {
	var f struct {
		Groups []struct {
			Name     string `json:"name"`
			On       *bool  `json:"on"`
			Commands []Rule `json:"commands"`
		} `json:"groups"`
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, "it isn't a guard file: " + err.Error()
	}
	if err := dec.Decode(&json.RawMessage{}); err != io.EOF {
		return nil, "there is text after the guard file's JSON"
	}
	if len(f.Groups) == 0 {
		return nil, `it has no "groups"`
	}
	var groups []Group
	seen := map[string]bool{}
	for i, g := range f.Groups {
		if strings.TrimSpace(g.Name) == "" {
			return nil, fmt.Sprintf("group %d has no name", i+1)
		}
		if seen[g.Name] {
			return nil, fmt.Sprintf("there are two groups called %q", g.Name)
		}
		seen[g.Name] = true
		if g.On == nil {
			return nil, fmt.Sprintf(`group %q has no "on" (true or false)`, g.Name)
		}
		for _, r := range g.Commands {
			if len(strings.Fields(string(r))) == 0 {
				return nil, fmt.Sprintf("group %q has an empty command", g.Name)
			}
		}
		groups = append(groups, Group{Name: g.Name, On: *g.On, Commands: g.Commands})
	}
	return groups, ""
}

// DefaultFile is the default groups as a file's text, for the owner to edit.
func DefaultFile() []byte {
	data, _ := json.MarshalIndent(File{Groups: DefaultGroups()}, "", "  ")
	return append(data, '\n')
}
