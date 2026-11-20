package desktop

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// licensesName is the file of open-source notices a copy of Smolcorp ships: the licences of what is built into
// it (tmux, with libevent, ncurses, utf8proc and jemalloc), written by scripts/build-tmux.sh from the sources it
// built, so they always match the version that ships.
const licensesName = "THIRD-PARTY-NOTICES.txt"

// licensesLimit is the most of that file the core reads. The real one is a few hundred kilobytes.
const licensesLimit = 4 << 20

// licensesPaths are where the notices are, by what this program is run from: Smolcorp.app's
// Resources/licenses beside its bin folder (the Mac app), or beside the program (a Linux or Windows download).
// The route never takes a path from a request: it serves one of these, as a file, and nothing else.
func licensesPaths() []string {
	program, err := executable()
	if err != nil {
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(program); err == nil {
		program = resolved
	}
	folder := filepath.Dir(program)
	return []string{
		filepath.Join(filepath.Dir(folder), "licenses", licensesName),
		filepath.Join(folder, licensesName),
	}
}

// serveLicenses answers GET /licenses: the notices as plain text, for anyone who can reach the core, as they
// hold nothing of yours. A copy built from the source folder has none, and says so.
func serveLicenses(w http.ResponseWriter) {
	for _, path := range licensesPaths() {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(file, licensesLimit))
		_ = file.Close()
		if err != nil || len(data) == 0 {
			continue
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		_, _ = w.Write(data)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = io.WriteString(w, "The open-source notices aren't in this copy of Smolcorp.\n")
}
