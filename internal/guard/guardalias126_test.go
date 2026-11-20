package guard

import (
	"strings"
	"testing"
)

// FXGUARDALIAS126: gh's own aliases of a refused command (each command's --help, under ALIASES) are refused with it:
// pr, issue, repo and gist create are also new, codespace is also cs, and extension is also extensions and ext. Each
// row runs for a hire and the GM, with the gate groups on, as a Mac's or Linux's line and as Windows reads one. And gh
// auth git-credential, which prints gh's token as gh auth token does, is refused with it.

// aliasSpellings are line, as written and in the readings the guard already handles: its case changed, through a
// wrapper, with -R before the subcommand, and deeper in a line.
func aliasSpellings(line string) []string {
	words := strings.SplitN(line, " ", 3)
	withRepo := words[0] + " " + words[1] + " -R ada/app " + strings.Join(words[2:], " ")
	return []string{
		line, strings.ToUpper(line), "GH " + strings.TrimPrefix(line, "gh "),
		"npx " + line, "env " + line, "sudo " + line, "command " + line, "/opt/homebrew/bin/" + line, withRepo,
		"sh -c '" + line + "'", `bash -c "sh -c '` + line + `'"`, "true && " + line, "echo $(" + line + ")", "(cd x && " + line + ")",
	}
}

func TestGhsOwnAliasesOfARefusedCommandAreRefused(t *testing.T) {
	aliases := []string{
		"gh pr new --fill", "gh issue new -t x", "gh repo new ada/app --public", "gh gist new x.txt",
		"gh cs create -R ada/app", "gh cs delete --codespace x", "gh cs ssh --codespace x", "gh codespace create -R ada/app",
		"gh ext install ada/gh-x", "gh extensions install ada/gh-x", "gh extension install ada/gh-x",
	}
	// gh auth git-credential is git's credential helper: asked with get, it prints the token, as gh auth token does.
	tokenPrinters := []string{"gh auth git-credential get", "gh auth token -h github.com"}
	// The spellings refused before stay refused.
	before := []string{"gh pr create --fill", "gh issue create -t x", "gh repo create ada/app", "gh gist create x.txt", "gh pr merge 1", "gh repo delete ada/app --yes"}
	readings(t, func(t *testing.T) {
		var refused []string
		for _, line := range append(append(aliases, before...), tokenPrinters...) {
			refused = append(refused, aliasSpellings(line)...)
		}
		runsAndRefused(t, []string{
			// The same words where gh reads no refused command: reads, and new as a value.
			"gh pr list --label new", "gh pr view 12", "gh pr checkout new", "gh issue ls --search new", "gh repo clone ada/new",
			"gh repo view ada/new", "gh gist view new", "gh cs list", "gh cs ls", "gh codespace view -c x", "gh ext list",
			"gh ext ls", "gh extensions upgrade --all", "gh release view new", "echo gh pr new", "git log --grep 'pr new'",
			"gh auth status", "gh auth switch -u ada", "git log --grep git-credential",
		}, refused)
		for who, groups := range both {
			for _, line := range append(aliases, tokenPrinters...) {
				if r := Check(line, WithGates(groups)); r == nil || r.Group != "publish" {
					t.Errorf("%s: %q: %v", who, line, r)
				}
			}
		}
	})
}
