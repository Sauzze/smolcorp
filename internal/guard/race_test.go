//go:build race

package guard

// raceSlowdown is how many times longer a time limit on heavy work is under the race detector,
// which makes such work up to 20 times slower (Go's documentation says 2 to 20; the checks of
// the huge lines here are 6 to 9 times slower on a Mac). Stretched so, a limit still catches a
// guard gone quadratic, which takes minutes.
const raceSlowdown = 20
