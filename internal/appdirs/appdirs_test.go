package appdirs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNameOnEachSystem(t *testing.T) {
	for goos, want := range map[string]string{
		"darwin": "AgentDeck", "linux": "sauzzedeck-pro", "windows": "SauzzeDeck Pro", "freebsd": "sauzzedeck-pro",
	} {
		if got := Name(goos); got != want {
			t.Errorf("%s: got %q, want %q", goos, got, want)
		}
	}
}

func TestNewNameOnEachSystem(t *testing.T) {
	for goos, want := range map[string]string{
		"darwin": "Smolcorp", "linux": "smolcorp", "windows": "Smolcorp", "freebsd": "smolcorp",
	} {
		if got := NewName(goos); got != want {
			t.Errorf("%s: got %q, want %q", goos, got, want)
		}
	}
}

// The resolver on each system's layout: the Smolcorp folder only once it is a real folder holding registry.json,
// otherwise the old one, and it never makes either.
func TestResolveTakesTheSmolcorpFolderOnlyWithItsState(t *testing.T) {
	type layout struct {
		name  string
		make  func(t *testing.T, base, goos string) // what's in the config folder
		isNew bool
	}
	plain := func(dir, file string) func(t *testing.T, base, goos string) {
		return func(t *testing.T, base, goos string) {
			d := filepath.Join(base, dir)
			if dir == "" {
				d = filepath.Join(base, NewName(goos))
			}
			if err := os.MkdirAll(d, 0o700); err != nil {
				t.Fatal(err)
			}
			if file != "" {
				if err := os.WriteFile(filepath.Join(d, file), []byte("{}"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	layouts := []layout{
		{"nothing there", func(*testing.T, string, string) {}, false},
		{"only the old folder", func(t *testing.T, base, goos string) { plain(Name(goos), "registry.json")(t, base, goos) }, false},
		{"an empty Smolcorp folder", plain("", ""), false},
		// Linux: a developer's ~/.config/smolcorp/sign-identity is no state of the app's.
		{"a Smolcorp folder with only sign-identity", plain("", "sign-identity"), false},
		{"a Smolcorp folder with registry.json", plain("", "registry.json"), true},
		{"registry.json is a folder", func(t *testing.T, base, goos string) {
			if err := os.MkdirAll(filepath.Join(base, NewName(goos), "registry.json"), 0o700); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"registry.json is a link", func(t *testing.T, base, goos string) {
			plain(Name(goos), "registry.json")(t, base, goos)
			plain("", "")(t, base, goos)
			if err := os.Symlink(filepath.Join(base, Name(goos), "registry.json"), filepath.Join(base, NewName(goos), "registry.json")); err != nil {
				t.Skip(err) // Windows without the right to make links
			}
		}, false},
		{"the Smolcorp folder is a link to the old one", func(t *testing.T, base, goos string) {
			plain(Name(goos), "registry.json")(t, base, goos)
			if err := os.Symlink(filepath.Join(base, Name(goos)), filepath.Join(base, NewName(goos))); err != nil {
				t.Skip(err)
			}
		}, false},
		{"both, the Smolcorp one with registry.json", func(t *testing.T, base, goos string) {
			plain(Name(goos), "registry.json")(t, base, goos)
			plain("", "registry.json")(t, base, goos)
		}, true},
	}
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for _, l := range layouts {
			t.Run(goos+"/"+l.name, func(t *testing.T) {
				base := t.TempDir()
				l.make(t, base, goos)
				before := listing(t, base)
				want := filepath.Join(base, Name(goos))
				if l.isNew {
					want = filepath.Join(base, NewName(goos))
				}
				if got := ResolveIn(base, goos); got != want {
					t.Errorf("got %s, want %s", got, want)
				}
				if after := listing(t, base); after != before {
					t.Errorf("the resolver changed the config folder:\nbefore %s\nafter  %s", before, after)
				}
			})
		}
	}
}

// Resolve looks in the user's config folder: Application Support on a Mac, $XDG_CONFIG_HOME on Linux, %APPDATA% on
// Windows.
func TestResolveUsesTheConfigFolder(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	t.Setenv("APPDATA", base)
	t.Setenv("HOME", base)
	got, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if dir, _ := os.UserConfigDir(); got != ResolveIn(dir, runtime.GOOS) {
		t.Errorf("Resolve is %s, ResolveIn(%s) is %s", got, dir, ResolveIn(dir, runtime.GOOS))
	}
}

func listing(t *testing.T, base string) string {
	t.Helper()
	var out string
	err := filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(base, p)
		out += rel + " "
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
