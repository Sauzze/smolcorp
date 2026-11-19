//go:build !windows

package desktop

// sweepStartHolds is the projects folder a hire's start holds (desksandbox.go), for row A (sweep3p4_test.go).
func sweepStartHolds() string { return hireProjectsFolder() }
