// Command goversion fails unless the go that runs it is go.mod's release line (internal/goline): the same major.minor
// as go.mod's go line, at or past its patch. scripts/build-app.sh runs it first, with the go it builds with, so a Mac
// with another go stops before anything is built:
//
//	go run ./scripts/goversion
//
// go run builds it with that very go, so runtime.Version is that go's. Run it from the repository's folder.
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/Sauzze/smolcorp/internal/goline"
)

func main() {
	goMod, err := os.ReadFile("go.mod")
	if err == nil {
		err = goline.Check(goMod, runtime.Version())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Stopped:", err)
		os.Exit(1)
	}
}
