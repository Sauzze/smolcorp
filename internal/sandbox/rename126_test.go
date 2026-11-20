package sandbox

import (
	"slices"
	"strings"
	"testing"
)

// FXRENAME126A: Smolcorp's own name for the support folder (appdirs.NewName: Application Support/Smolcorp,
// ~/.config/smolcorp, %APPDATA%\Smolcorp) is kept as the old one is, on every system and whichever of the two the core
// uses: a desk reads, writes and connects to nothing there, with no exception, there or not. A smolcorp folder anywhere
// else (the owner's own checkout, ~/Projects/smolcorp) is as any other folder.
func TestASupportFolderByItsOtherNameIsDeniedWhole(t *testing.T) {
	const (
		macNew = home + "/Library/Application Support/Smolcorp"
		linNew = lHome + "/.config/smolcorp"
		winOld = `C:\Users\Ada\AppData\Roaming\SauzzeDeck Pro`
		winNew = `C:\Users\Ada\AppData\Roaming\Smolcorp`
	)
	testCopy := hirePlaces(copyDir)
	testCopy.OtherSupports = []string{realSupport}
	linuxCopy := linuxPlaces(lCopy)
	linuxCopy.OtherSupports = []string{lSupport}
	windows := func(support string) Places {
		return Places{GOOS: "windows", Subject: Hire, Home: `C:\Users\Ada`, Support: support, OwnID: "h1", Root: `C:\Users\Ada\Projects\app-hires\h1`}
	}
	for _, c := range []struct {
		name string
		pl   Places
		kept string // the folder by the other name
		open string // a smolcorp folder elsewhere
		like string // a folder beside it with another name, which it is read like
	}{
		{"a Mac hire, its core at AgentDeck", hirePlaces(realSupport), macNew, home + "/Projects/smolcorp", home + "/Projects/smolcorx"},
		{"a Mac hire, its core at Smolcorp", hirePlaces(macNew), realSupport, home + "/Projects/smolcorp", home + "/Projects/smolcorx"},
		{"a Mac test copy's hire", testCopy, macNew, home + "/Projects/smolcorp", home + "/Projects/smolcorx"},
		{"a Linux hire, its core at sauzzedeck-pro", linuxPlaces(lSupport), linNew, lHome + "/Projects/smolcorp", lHome + "/Projects/smolcorx"},
		{"a Linux hire, its core at smolcorp", linuxPlaces(linNew), lSupport, lHome + "/Projects/smolcorp", lHome + "/Projects/smolcorx"},
		{"a Linux test copy's hire", linuxCopy, linNew, lHome + "/.local/share/smolcorp", lHome + "/.local/share/smolcorx"},
		{"a Windows hire, its core at SauzzeDeck Pro", windows(winOld), winNew, `C:\Users\Ada\Projects\smolcorp`, `C:\Users\Ada\Projects\smolcorx`},
		{"a Windows hire, its core at Smolcorp", windows(winNew), winOld, `C:\Users\Ada\Projects\smolcorp`, `C:\Users\Ada\Projects\smolcorx`},
	} {
		p := mustNew(t, c.pl)
		sep := "/"
		if c.pl.GOOS == "windows" {
			sep = `\`
		}
		for _, at := range []string{"", "registry.json", "phones.json", "hook-key", "sign-identity", "hire-guard.json", "nightshift.json",
			"hires" + sep + "h1" + sep + "STATUS.md", "hires" + sep + "h1" + sep + "BRIEF.md", "a-file-added-later"} {
			at := c.kept + sep + at
			if at[len(at)-1:] == sep {
				at = c.kept
			}
			for _, op := range []Op{Read, Write, Connect} {
				if v, r := p.Decide(at, op); v != Deny || r.Row != rowSupportOtherName {
					t.Errorf("%s: %v %s: %v by %q", c.name, op, at, v, r.Row)
				}
			}
		}
		for _, op := range []Op{Read, Write} {
			got, by := p.Decide(c.open+sep+"README.md", op)
			want, _ := p.Decide(c.like+sep+"README.md", op)
			if got != want || by.Row == rowSupportOtherName {
				t.Errorf("%s: %v %s: %v by %q, not %v as any other folder", c.name, op, c.open, got, by.Row, want)
			}
		}
	}
}

// The other name is the old one's sibling only for a support folder by one of appdirs' names: a test copy's own
// folder, or one with any other name, has none, and one already named is not named twice.
func TestOnlyASupportFolderByItsNamesHasASibling(t *testing.T) {
	for _, c := range []struct {
		goos     string
		supports []string
		want     []string
	}{
		{"darwin", []string{realSupport}, []string{home + "/Library/Application Support/Smolcorp"}},
		{"darwin", []string{home + "/Library/Application Support/agentdeck"}, []string{home + "/Library/Application Support/Smolcorp"}},
		{"darwin", []string{copyDir}, nil},
		{"darwin", []string{copyDir, realSupport}, []string{home + "/Library/Application Support/Smolcorp"}},
		{"darwin", []string{realSupport, home + "/Library/Application Support/Smolcorp"}, nil},
		{"linux", []string{lSupport}, []string{lHome + "/.config/smolcorp"}},
		{"linux", []string{lHome + "/.config/SAUZZEDECK-PRO"}, nil}, // another folder on Linux
		{"windows", []string{`C:\Users\Ada\AppData\Roaming\SauzzeDeck Pro`}, []string{"C:/Users/Ada/AppData/Roaming/Smolcorp"}},
		{"windows", []string{`C:\Users\Ada\AppData\Roaming\smolcorp`}, []string{"C:/Users/Ada/AppData/Roaming/SauzzeDeck Pro"}},
	} {
		if got := supportSiblings(c.goos, c.supports); !slices.Equal(got, c.want) {
			t.Errorf("%s %q: %q, want %q", c.goos, c.supports, got, c.want)
		}
	}
}

// On Linux the view puts an empty tmpfs over the folder by its other name when it's there, with nothing of it brought
// back, whichever name the core uses; when it isn't there, nothing makes it (no hold, nothing the view makes), and
// nothing of it is in the view. Neither name is ever held, on a Mac either.
func TestTheLinuxViewCoversTheSupportFolderByItsOtherName(t *testing.T) {
	const linNew = lHome + "/.config/smolcorp"
	for _, c := range []struct {
		name          string
		support, kept string
	}{
		{"its core at sauzzedeck-pro", lSupport, linNew},
		{"its core at smolcorp", linNew, lSupport},
	} {
		for _, there := range []bool{true, false} {
			f := linuxComputer()
			if there {
				f.add(linNew+"/registry.json", linNew+"/phones.json", linNew+"/sign-identity", linNew+"/hires/h1/STATUS.md")
			} else if c.support == linNew {
				continue // its core's own folder is there
			}
			if c.support == linNew {
				// The old folder is there already: linuxComputer has it.
				f.add(linNew + "/hires/h1/BRIEF.md")
			}
			p, args := viewOf(t, setup{"linux hire, " + c.name, linuxPlaces(c.support)}, f)
			text := strings.Join(args, " ")
			if there || c.kept == lSupport {
				if !strings.Contains(text, "--tmpfs "+c.kept+" ") {
					t.Errorf("%s (there: %v): no tmpfs over %s:\n%s", c.name, there, c.kept, argvText(args))
				}
			}
			for i, a := range args {
				if strings.HasPrefix(a, c.kept+"/") && i > 0 && args[i-1] != "--tmpfs" {
					t.Errorf("%s (there: %v): %s is in the view: %s %s", c.name, there, a, args[i-1], a)
				}
			}
			if _, made := f[linNew]; !there && made {
				t.Errorf("%s: the start made %s", c.name, linNew)
			}
			for _, held := range p.Hold() {
				if held == c.kept || strings.HasPrefix(held, c.kept+"/") {
					t.Errorf("%s: %s is held", c.name, held)
				}
			}
		}
	}
	macNew := home + "/Library/Application Support/Smolcorp"
	for support, other := range map[string]string{realSupport: macNew, macNew: realSupport} {
		for _, held := range mustNew(t, hirePlaces(support)).Hold() {
			if held == other || strings.HasPrefix(held, other+"/") {
				t.Errorf("a Mac hire with its core at %s: %s is held", support, held)
			}
		}
	}
}
