package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const root = "../.."

// The office every app serves is exactly the build of its sources: a change
// made to office.js alone, or to a source without building, fails here.
func TestOfficeJSIsTheBuildOfItsSources(t *testing.T) {
	built, err := BuildFrom(filepath.Join(root, sources))
	if err != nil {
		t.Fatal(err)
	}
	office, err := os.ReadFile(filepath.Join(root, officeJS))
	if err != nil {
		t.Fatal(err)
	}
	if line := firstDifference(string(office), built); line > 0 {
		t.Errorf("%s isn't the build of %s/, from line %d: run go run ./scripts/buildoffice", officeJS, sources, line)
	}
	if err := run(root, true, false); err != nil {
		t.Errorf("-check: %v", err)
	}
}

// The Mac app's build and the Windows and Linux release check office.js
// against its sources first, and the Mac app carries no copy of Resources/web
// (the pages are inside its `smolcorp` command), so the sources stay out.
func TestTheBuildsCheckOfficeJS(t *testing.T) {
	for _, script := range []string{"scripts/build-app.sh", "scripts/release-desktop.sh"} {
		text, err := os.ReadFile(filepath.Join(root, script))
		if err != nil {
			t.Fatal(err)
		}
		check := strings.Index(string(text), "go run ./scripts/buildoffice -check\n")
		if check < 0 {
			t.Errorf("%s doesn't run go run ./scripts/buildoffice -check", script)
		}
		if script != "scripts/build-app.sh" {
			continue
		}
		if strings.Contains(string(text), "cp -R Resources/web") {
			t.Errorf("%s copies Resources/web into the app, which would carry office.js's sources", script)
		}
	}
}

// Splitting office.js gives the sources as they're committed, so a branch from
// before them, merged, is split back into the same files.
func TestSplittingOfficeJSGivesItsSources(t *testing.T) {
	office, err := os.ReadFile(filepath.Join(root, officeJS))
	if err != nil {
		t.Fatal(err)
	}
	main, files, err := Split(string(office))
	if err != nil {
		t.Fatal(err)
	}
	files[mainJS] = main
	for name, text := range files {
		committed, err := os.ReadFile(filepath.Join(root, sources, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if line := firstDifference(string(committed), text); line > 0 {
			t.Errorf("%s/%s differs from office.js split, from line %d", sources, name, line)
		}
	}
	// Every file in the folder is a source the build reads, or this folder's README.
	err = filepath.WalkDir(filepath.Join(root, sources), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name, _ := filepath.Rel(filepath.Join(root, sources), path)
		if _, known := files[filepath.ToSlash(name)]; !known && name != "README.md" && !strings.HasPrefix(entry.Name(), ".") {
			t.Errorf("%s/%s isn't a source buildoffice knows: list it in modules", sources, filepath.ToSlash(name))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A table is indented as its include line, but for its empty lines; a line of
// spaces keeps them.
func TestBuildIndentsEachTableAsItsIncludeLine(t *testing.T) {
	saved := modules
	defer func() { modules = saved }()
	modules = []module{{"data/a.js", "A"}, {"data/b.js", "B"}}
	tables := map[string]string{
		"data/a.js": "/** A. */\nconst A = {\n  one: 1,\n\n  \n};\n",
		"data/b.js": "const B = [];\n",
	}
	read := func(name string) (string, error) { return tables[name], nil }
	main := "(() => {\n  //#include data/a.js\n  function f() {\n    //#include data/b.js\n  }\n})();\n"
	want := "(() => {\n  /** A. */\n  const A = {\n    one: 1,\n\n    \n  };\n  function f() {\n    const B = [];\n  }\n})();\n"
	got, err := Build(main, read)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if back, files, err := Split(got); err != nil || back != main || files["data/a.js"] != tables["data/a.js"] || files["data/b.js"] != tables["data/b.js"] {
		t.Errorf("split again: %v\n%s\n%q", err, back, files)
	}

	for what, main := range map[string]string{
		"out of order": "  //#include data/b.js\n  //#include data/a.js\n",
		"missing":      "  //#include data/a.js\n",
		"twice":        "  //#include data/a.js\n  //#include data/a.js\n  //#include data/b.js\n",
		"unknown":      "  //#include data/a.js\n  //#include data/c.js\n",
	} {
		if _, err := Build(main, read); err == nil {
			t.Errorf("a table %s builds", what)
		}
	}
}

// -check fails when office.js was changed by itself, and says what to run.
func TestCheckFailsWhenOfficeJSWasChangedAlone(t *testing.T) {
	folder := t.TempDir()
	if err := os.MkdirAll(filepath.Join(folder, sources), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, m := range modules {
		path := filepath.Join(folder, sources, filepath.FromSlash(m.file))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("const "+m.name+" = {\n};\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var main strings.Builder
	main.WriteString("(() => {\n")
	for _, m := range modules {
		main.WriteString("  " + include + m.file + "\n")
	}
	main.WriteString("})();\n")
	if err := os.WriteFile(filepath.Join(folder, sources, mainJS), []byte(main.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(folder, false, false); err != nil {
		t.Fatal(err)
	}
	if err := run(folder, true, false); err != nil {
		t.Fatalf("-check right after a build: %v", err)
	}
	office := filepath.Join(folder, officeJS)
	built, _ := os.ReadFile(office)
	if err := os.WriteFile(office, append(built, "// changed here only\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(folder, true, false); err == nil || !strings.Contains(err.Error(), "go run ./scripts/buildoffice") {
		t.Errorf("-check after office.js changed alone: %v", err)
	}
	// Split back, that change goes into main.js, and the build has it.
	if err := run(folder, false, true); err != nil {
		t.Fatal(err)
	}
	if err := run(folder, true, false); err != nil {
		t.Errorf("-check after -split: %v", err)
	}
}
