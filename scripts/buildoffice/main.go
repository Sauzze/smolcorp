// Command buildoffice builds Resources/web/office.js, the office every app
// serves, from its sources in Resources/web/office/: main.js, with a line
//
//	//#include data/outfit-parts.js
//
// where each of the big tables goes, and the tables themselves in data/. Each
// include line is replaced by its file, every line of it indented as the
// include line is, so the tables are written from the left edge and come out
// exactly where they were. The order is main.js's, and the list of tables is
// fixed below.
//
//	go run ./scripts/buildoffice          writes office.js from its sources
//	go run ./scripts/buildoffice -check   fails if office.js isn't exactly their build
//	go run ./scripts/buildoffice -split   the other way round: writes the sources from office.js
//
// office.js stays committed: the apps serve it, the viewer copies it, and the
// tests read it. A test fails when it isn't the build of its sources.
//
// -split is for a branch from before the sources, which still changed
// office.js itself: merged, its office.js is split back into the sources. Each
// table is found by its `const` line, with the doc comment right above it, down
// to the `};` that closes it at the same indent.
//
// It uses only the standard library. Run it from the repository's folder.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// A table moved out of office.js: its file in Resources/web/office/, and the
// name it's declared with.
type module struct {
	file, name string
}

// The tables, in the order they come in office.js.
var modules = []module{
	{"data/room-styles.js", "ROOM_STYLES"},
	{"data/paints.js", "PAINTS"},
	{"data/outfits.js", "OUTFITS"},
	{"data/wall-finishes.js", "WALL_FINISHES"},
	{"data/floor-finishes.js", "FLOOR_FINISHES"},
	{"data/wall-pieces.js", "WALL_PIECES"},
	{"data/monitors.js", "MONITORS"},
	{"data/workspace-parts.js", "WORKSPACE_PARTS"},
	{"data/desk-items.js", "DESK_ITEMS"},
	{"data/mic-heads.js", "MIC_HEADS"},
	{"data/button-styles.js", "BUTTON_STYLES"},
	{"data/outfit-parts.js", "OUTFIT_PARTS"},
}

const (
	officeJS = "Resources/web/office.js"
	sources  = "Resources/web/office"
	mainJS   = "main.js"
	include  = "//#include "
)

func main() {
	check := flag.Bool("check", false, "fail if office.js isn't exactly the build of its sources")
	split := flag.Bool("split", false, "write the sources from office.js")
	flag.Parse()
	if err := run(".", *check, *split); err != nil {
		fmt.Fprintln(os.Stderr, "buildoffice:", err)
		os.Exit(1)
	}
}

func run(root string, check, split bool) error {
	switch {
	case check && split:
		return errors.New("-check or -split, not both")
	case split:
		office, err := os.ReadFile(filepath.Join(root, officeJS))
		if err != nil {
			return err
		}
		main, files, err := Split(string(office))
		if err != nil {
			return err
		}
		files[mainJS] = main
		for name, text := range files {
			path := filepath.Join(root, sources, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
	built, err := BuildFrom(filepath.Join(root, sources))
	if err != nil {
		return err
	}
	path := filepath.Join(root, officeJS)
	if check {
		office, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if line := firstDifference(string(office), built); line > 0 {
			return fmt.Errorf("%s isn't the build of %s/ (from line %d): after changing a source, run go run ./scripts/buildoffice; "+
				"a change made in office.js itself goes in its source (on a branch from before the sources, go run ./scripts/buildoffice -split)",
				officeJS, sources, line)
		}
		return nil
	}
	return os.WriteFile(path, []byte(built), 0o644)
}

// BuildFrom builds office.js from the sources in folder.
func BuildFrom(folder string) (string, error) {
	main, err := os.ReadFile(filepath.Join(folder, mainJS))
	if err != nil {
		return "", err
	}
	return Build(string(main), func(name string) (string, error) {
		text, err := os.ReadFile(filepath.Join(folder, filepath.FromSlash(name)))
		return string(text), err
	})
}

// Build puts each table in main where its include line is, indented as that
// line is. Every table must be there, once, in the order of modules.
func Build(main string, read func(name string) (string, error)) (string, error) {
	var out strings.Builder
	next := 0
	for number, line := range strings.SplitAfter(main, "\n") {
		body := strings.TrimSuffix(line, "\n")
		indent := body[:len(body)-len(strings.TrimLeft(body, " "))]
		name, ok := strings.CutPrefix(body[len(indent):], include)
		if !ok {
			out.WriteString(line)
			continue
		}
		if next == len(modules) || name != modules[next].file {
			return "", fmt.Errorf("%s:%d: %s isn't the next table (they go in the order buildoffice lists them)", mainJS, number+1, name)
		}
		next++
		text, err := read(name)
		if err != nil {
			return "", err
		}
		if !strings.HasSuffix(text, "\n") || !strings.HasSuffix(line, "\n") {
			return "", fmt.Errorf("%s: a table and its include line each end with a line break", name)
		}
		for _, row := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
			if row != "" {
				out.WriteString(indent + row)
			}
			out.WriteString("\n")
		}
	}
	if next < len(modules) {
		return "", fmt.Errorf("%s has no include line for %s", mainJS, modules[next].file)
	}
	return out.String(), nil
}

// Split takes the tables out of office, each with the doc comment right above
// it, and leaves an include line where it was: main.js, and each table's
// file. Building them gives office back.
func Split(office string) (main string, files map[string]string, err error) {
	lines := strings.Split(office, "\n")
	files = map[string]string{}
	var out []string
	from := 0
	for _, m := range modules {
		at := -1
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimLeft(line, " "), "const "+m.name+" = ") {
				if at >= 0 {
					return "", nil, fmt.Errorf("%s is declared twice", m.name)
				}
				at = i
			}
		}
		if at < from {
			return "", nil, fmt.Errorf("%s isn't in office.js after the table before it", m.name)
		}
		indent := lines[at][:len(lines[at])-len(strings.TrimLeft(lines[at], " "))]
		end := at // a table on one line ends there
		for end < len(lines) && !strings.HasSuffix(lines[at], ";") && lines[end] != indent+"};" && lines[end] != indent+"];" {
			end++
		}
		if end == len(lines) {
			return "", nil, fmt.Errorf("%s has no %s}; to close it", m.name, indent)
		}
		// The doc comment right above: a /* … */ block (its last one only), or a run of // lines.
		start := at
		if above := at - 1; above >= from {
			switch line := lines[above]; {
			case strings.HasPrefix(line, indent+"//"):
				for start = above; start > from && strings.HasPrefix(lines[start-1], indent+"//"); start-- {
				}
			case strings.HasSuffix(line, "*/") && (strings.HasPrefix(line, indent+"/*") || strings.HasPrefix(line, indent+" *")):
				for start = above; start > from && !strings.HasPrefix(lines[start], indent+"/*"); start-- {
				}
				if !strings.HasPrefix(lines[start], indent+"/*") {
					start = at
				}
			}
		}
		var table strings.Builder
		for _, line := range lines[start : end+1] {
			if line != "" && !strings.HasPrefix(line, indent) {
				return "", nil, fmt.Errorf("%s: a line less indented than its const: %q", m.name, line)
			}
			table.WriteString(strings.TrimPrefix(line, indent) + "\n")
		}
		files[m.file] = table.String()
		out = append(out, lines[from:start]...)
		out = append(out, indent+include+m.file)
		from = end + 1
	}
	out = append(out, lines[from:]...)
	main = strings.Join(out, "\n")
	// Whatever office holds, its sources must build it again.
	built, err := Build(main, func(name string) (string, error) { return files[name], nil })
	if err != nil {
		return "", nil, err
	}
	if line := firstDifference(office, built); line > 0 {
		return "", nil, fmt.Errorf("the sources don't build office.js again (from line %d)", line)
	}
	return main, files, nil
}

// firstDifference is the first line where a and b differ, or 0 when they don't.
func firstDifference(a, b string) int {
	if a == b {
		return 0
	}
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := range al {
		if i >= len(bl) || al[i] != bl[i] {
			return i + 1
		}
	}
	return len(al) + 1
}
