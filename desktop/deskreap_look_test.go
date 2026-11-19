//go:build !windows

package desktop

import "testing"

// RT126FIXREVIEW-FIREREAPR4 O1: a look counts a process in a desk's sandbox only from one it found running whole, the
// same process, before the checks and after them (reapLookWith). macOS's steps are stood in, so each case is exact: a
// process being made at the first check (it answers denied to every deny, as one that's ended does), or another
// process, or one ending, by the end, is asked again, never counted in.

// lookScript stands in for a look's steps: each answers from its list in turn, and goes on giving its last answer once
// its list is used up.
type lookScript struct {
	running []lookRunning
	tokens  []bool
	ins     []int
	asked   int // how many times the sandbox was asked
}

// lookRunning is one answer of the running step: its state (1 whole, 2 zombie, 0 neither, -1 macOS won't say) and the
// start time it read.
type lookRunning struct {
	state int
	start uint64
}

func (s *lookScript) steps() reapSteps {
	running, tokens := 0, 0
	next := func(at *int, n int) int {
		i := min(*at, n-1)
		*at++
		return i
	}
	return reapSteps{
		running: func(int) (int, uint64) {
			r := s.running[next(&running, len(s.running))]
			return r.state, r.start
		},
		token: func(int) ([8]uint32, bool) {
			return [8]uint32{7: 1}, s.tokens[next(&tokens, len(s.tokens))] // the same program each time
		},
		in: func(*[8]uint32, int) int {
			in := s.ins[min(s.asked, len(s.ins)-1)]
			s.asked++
			return in
		},
	}
}

func TestALookCountsInOnlyAProcessItFoundRunningWholeBeforeAndAfter(t *testing.T) {
	never := make([]lookRunning, 20) // another start time at every read: never the same process twice
	for i := range never {
		never[i] = lookRunning{1, uint64(100 + i)}
	}
	for _, c := range []struct {
		name  string
		s     lookScript
		want  reapVerdict
		asked int
	}{
		// Asked again: the second look finds it running whole, the same process, and not in.
		{"being made at the first check", lookScript{running: []lookRunning{{0, 0}, {1, 5}}, tokens: []bool{true}, ins: []int{1, 0}}, reapNotIn, 2},
		{"being made at the first check, asked by its pid", lookScript{running: []lookRunning{{0, 0}, {1, 5}}, tokens: []bool{false}, ins: []int{1, 0}}, reapNotIn, 2},
		{"another process by the end", lookScript{running: []lookRunning{{1, 5}, {1, 6}}, tokens: []bool{true}, ins: []int{1, 0}}, reapNotIn, 2},
		{"another process by the end, asked by its pid", lookScript{running: []lookRunning{{1, 5}, {1, 6}}, tokens: []bool{false}, ins: []int{1, 0}}, reapNotIn, 2},
		// Asked again: by the second look it has ended, and its run with it.
		{"ending by the end", lookScript{running: []lookRunning{{1, 5}, {0, 0}}, tokens: []bool{true, true, false}, ins: []int{1, 0}}, reapGone, 2},
		{"a zombie by the end", lookScript{running: []lookRunning{{1, 5}, {2, 5}}, tokens: []bool{true}, ins: []int{1}}, reapZombie, 1},
		// It never settles: not answered after 8 tries, which fails closed.
		{"another process at every read", lookScript{running: never, tokens: []bool{true}, ins: []int{1}}, reapUnknown, 8},
		// The controls: the same process running whole throughout is in.
		{"running whole throughout", lookScript{running: []lookRunning{{1, 5}}, tokens: []bool{true}, ins: []int{1}}, reapIn, 1},
		{"running whole throughout, asked by its pid", lookScript{running: []lookRunning{{1, 5}}, tokens: []bool{false}, ins: []int{1}}, reapInUnreached, 1},
	} {
		run, verdict := reapLookWith(c.s.steps(), 4242)
		if verdict != c.want || c.s.asked != c.asked {
			t.Errorf("%s: the look said %v after asking %d times, want %v after %d", c.name, verdict, c.s.asked, c.want, c.asked)
		}
		if verdict == reapIn && run.token[7] != 1 || verdict == reapInUnreached && run.token != [8]uint32{} || run.pid != 4242 {
			t.Errorf("%s: the run %+v doesn't fit %v", c.name, run, verdict)
		}
	}
}
