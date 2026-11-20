// Package browserguard holds no code of the app's: its test reads every Go file of the module, tests and the code
// beside them, and fails if any runs an installed browser, looks for one, or reads a variable that names one
// (FXCHROMEPOPUP, FXCHROMEPOPUPR1). It is a package of its own so that the browser names it looks for aren't read as
// a browser test's own by the guards of the desktop's and the sandbox's packages
// (desktop/headlesschrome_guard_test.go, internal/sandbox/escape3_darwin_test.go).
package browserguard
