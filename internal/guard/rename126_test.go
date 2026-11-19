package guard

import "testing"

// FXRENAME126A: Smolcorp's own name for the support folder (appdirs.NewName) is kept as the old one is, by its whole
// path: Application Support/Smolcorp, ~/.config/smolcorp, %APPDATA%\Smolcorp (…\AppData\Roaming\Smolcorp). A
// smolcorp folder anywhere else, the owner's own checkout among them, is any folder.
func TestTheSmolcorpSupportFolderIsKeptByItsWholePath(t *testing.T) {
	for _, dir := range []string{
		"/Users/ada/Library/Application Support/Smolcorp", "/Users/ada/Library/Application Support/Smolcorp/hires/h1",
		"/home/ada/.config/smolcorp", "/home/ada/.config/smolcorp/hires/h1", "/home/ada/.config/SmolCorp", "/home/ada/.config./smolcorp",
		`C:\Users\ada\AppData\Roaming\Smolcorp`, `C:\Users\ada\AppData\Roaming\Smolcorp\hires\h1`, `c:\users\ada\appdata\roaming\SMOLCORP.`,
		`%APPDATA%\Smolcorp`, `$env:APPDATA\Smolcorp`, `${env:APPDATA}\Smolcorp`,
	} {
		if !KeptCwd(dir) {
			t.Errorf("%s: not kept", dir)
		}
	}
	for _, dir := range []string{
		"/Users/ada/Projects/smolcorp", "/Users/ada/Projects/smolcorp/desktop", "/home/ada/Projects/smolcorp", "/home/ada/smolcorp",
		"/home/ada/.local/share/smolcorp", "/home/ada/src/smolcorp/.dev/support/hires/h1", `C:\Users\ada\Projects\smolcorp`,
		`C:\Users\ada\AppData\Local\Smolcorp`, "/home/ada/.config/smolcorp-notes",
	} {
		if KeptCwd(dir) {
			t.Errorf("%s: kept", dir)
		}
	}
}

func TestAMoveIntoTheSmolcorpSupportFolderIsRefused(t *testing.T) {
	refusedForBoth(t, []string{
		`cd ~/Library/Application\ Support/Smolcorp && echo '{}' > h*.json`, `cd "$HOME/Library/Application Support/Smolcorp"`,
		"cd ~/.config/smolcorp", "cd ~/.config/smolcorp && rm registry.json", "cd ~/.config && cd smolcorp", "pushd /home/ada/.config/smolcorp",
		"cd ~/.config/smolcorp/hires/h1 && rm ../../registry.json", `cd "$XDG_CONFIG_HOME/smolcorp"`,
		"git -C ~/.config/smolcorp rm -f registry.json", "env -C ~/.config/smolcorp rm registry.json",
		"cd ~/.c*/smol* && rm registry.json", "cd ~/.config/smol*", "cd ~/.conf?g/smolcorp", "cd ~/.config/[s]molcorp",
		`cd C:\Users\ada\AppData\Roaming\Smolcorp`, `cd C:\Users\ada\AppData\Roaming\Smol*`, `cd %APPDATA%\Smolcorp`,
		// A pattern that could fill in a kept file there, as before.
		"cat ~/.config/smolcorp/ph*s.json", "cat ~/.config/smolc*/hire-g*",
	})
}

func TestTheOwnersSmolcorpCheckoutStillWorks(t *testing.T) {
	runsForBoth(t, []string{
		"cd ~/Projects/smolcorp && git status", "cd /home/ada/smolcorp && make",
		"git -C ~/Projects/smolcorp status", "cd ~/src/smolcorp/desktop && go vet .", `cd C:\Users\ada\Projects\smolcorp`,
		"cd ~/Projects/smolcorp/internal/appdirs && go test .", "ls ~/.config/smolcorp/*.md", "cd ~/.local/share/smolcorp && ls",
		"cat ~/Projects/smolcorp/docs/*.md",
	})
}
