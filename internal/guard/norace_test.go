//go:build !race

package guard

// raceSlowdown is 1 without the race detector: see race_test.go.
const raceSlowdown = 1
