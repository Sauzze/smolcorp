package desktop

import (
	"fmt"
	"os"
)

// RunAttach is `smolcorp attach`: on Windows no desk runs in tmux, so there is nothing to attach to.
func RunAttach([]string) int {
	fmt.Fprintln(os.Stderr, "smolcorp attach: on Windows a hire runs in a window of its own, not in tmux.")
	return 1
}
