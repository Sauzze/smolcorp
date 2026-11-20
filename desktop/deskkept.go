package desktop

import (
	"os"
	"slices"
	"strconv"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// deskKept is the core's look at the places no mount holds in a Linux desk's view (L1GUARD1FIRE), at a fire, a Take
// over or a Hand back; never the guard's, inside the view. sandbox.RestoreKept puts back only what the desk's guard
// stopped it for, still as it was then, what was there first set aside whole in <support>/hires/<id>/found/<time>/.
// Anything else changed is left as it is: your own edit, a change made again after the stop, your git's gc in the
// shared objects/info or in its branch folders, neither of which is ever put back. Those are only said. The copy is
// where the desk's last start left it (sandbox.KeptCopy): nothing when it keeps none, as only a Linux desk started in
// its view does.
//
// refused is the whole refusal, with the words, only when the places couldn't be checked against the copy or a place
// its guard stopped it for couldn't be put back (never a Repo or Branch place: those aren't put back). It fails
// closed: the caller goes no further. not is what then didn't happen ("fired").
func deskKept(h Hire, not string) (changes []sandbox.KeptChange, refused string) {
	held := sandbox.KeptCopy(supportDir, h.ID)
	if held == "" {
		return nil, ""
	}
	changes, err := sandbox.RestoreKept(held)
	if err != nil {
		// Removing held alone would leave a start's last copy beside it to be looked at next: that one too, when there.
		clear := strconv.Quote(held)
		if old := held + ".old"; held == sandbox.KeptFolder(supportDir, h.ID) {
			if _, err := os.Lstat(old); err == nil {
				clear += " and " + strconv.Quote(old)
			}
		}
		refused = joinNote(calledHire(h)+" wasn't "+not+": Smolcorp couldn't put back a place its desk was stopped for, "+
			"or couldn't check the places it may not change against its start's copy ("+err.Error()+"). Move that place "+
			"aside yourself and try again: what is there then is left as it is. Removing "+clear+
			" lets it go ahead with everything as it is now.", keptSaid(h, changes))
	}
	return changes, refused
}

// keptSaid is what a look found, as your card says it: each place, when it was seen and how, what was done (set aside
// and put back, or left as it is), its lines before and after, and where what was found is set aside. "" for nothing.
func keptSaid(h Hire, changes []sandbox.KeptChange) string {
	if len(changes) == 0 {
		return ""
	}
	return "Smolcorp checked what " + calledHire(h) + " may not change. " + sandbox.KeptWords(changes)
}

// keptBack is whether a look put ch back.
func keptBack(ch sandbox.KeptChange) bool { return ch.Back }

// keptOnce is what two looks of one fire found, each said once: what the first put back, then all the second found
// (a change left as it is is found by both).
func keptOnce(first, second []sandbox.KeptChange) []sandbox.KeptChange {
	return append(slices.DeleteFunc(slices.Clone(first), func(ch sandbox.KeptChange) bool { return !ch.Back }), second...)
}

// joinNote is two notes, the second on a line of its own, either one possibly "".
func joinNote(a, b string) string {
	if a == "" || b == "" {
		return a + b
	}
	return a + "\n" + b
}
