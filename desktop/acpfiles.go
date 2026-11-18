//go:build !windows

package desktop

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sauzze/smolcorp/internal/acp"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// An ACP desk's fs/* (SA-M11, SANDBOX-PLAN §A3.4's ACP row and §A3.5.4): its agent asks the core to read and write
// files, and the core does it as the owner, outside the desk's sandbox. So each one is first put to the desk's Policy,
// the rows its sandbox is made from, and then done by the core's reader rules (hirefile.go), never following a link:
//   - the path is resolved as the kernel would (the desk's own links included; for a write, its folder), and the Policy
//     is asked about the resolved path: the one the desk's sandbox would judge. Only an Allow goes on;
//   - the file is then reached from "/" one folder at a time with no link followed (openHireFolder): a link put in after
//     the resolve refuses it, so what is read or written is what the Policy was asked about;
//   - a read is readHireFile's: a regular file of one name, up to acpFileMost;
//   - a write goes to a new file beside the path, then is renamed over its name (the new name is put to the Policy
//     too): a second name at the path (a hard link) is replaced, never written through, so a desk's hard link to a
//     file its Policy denies can't be used to write that file. A link, a folder or anything else that isn't a regular
//     file at the path is refused.
//
// The agent is told a refusal in one sentence naming its path; the Policy row is only logged. Whether the session is
// the desk's, and the guard, come first, in the desk that answers the agent (ACP4b).

// acpFileMost is the most of a file an agent is given, in bytes: well under the client's 16 MiB message cap.
const acpFileMost = 8 << 20

// acpFiles answers one ACP desk's fs/* against its sandbox.
type acpFiles struct{ desk *deskSandbox }

var _ acp.FileHandler = acpFiles{}

// acpRefused is the agent's answer when it may not do what it asked.
func acpRefused(what string) *acp.RPCError {
	return &acp.RPCError{Code: acp.CodeInternalError, Message: "Smolcorp's sandbox doesn't let this desk " + what}
}

// acpResolved is path, whole, resolved as the kernel resolves it: all of it (it must be there), or, with folderOnly,
// its folder (for a file that may not be there yet), its own name kept. false for a path that isn't whole or can't be.
func acpResolved(path string, folderOnly bool) (string, bool) {
	if !filepath.IsAbs(path) {
		return "", false
	}
	path = filepath.Clean(path)
	if !folderOnly {
		resolved, err := filepath.EvalSymlinks(path)
		return resolved, err == nil
	}
	folder, err := filepath.EvalSymlinks(filepath.Dir(path))
	name := filepath.Base(path)
	if err != nil || path == "/" {
		return "", false
	}
	return filepath.Join(folder, name), true
}

// allows is whether the desk's Policy allows op on path, a resolved path. A refusal is logged with its row.
func (f acpFiles) allows(path string, op sandbox.Op) bool {
	verdict, rule := f.desk.policy.Decide(path, op)
	if verdict != sandbox.Allow {
		log.Printf("sandbox: ACP desk %s: %s %s refused by %q", f.desk.at.ID, op, path, rule.Row)
	}
	return verdict == sandbox.Allow
}

// ReadTextFile is fs/read_text_file: the file at req.Path, by the rules above, from its line req.Line (counted from 1)
// for req.Limit lines when they're given.
func (f acpFiles) ReadTextFile(_ context.Context, req acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	path, ok := acpResolved(req.Path, false)
	if !ok || !f.allows(path, sandbox.Read) {
		return acp.ReadTextFileResponse{}, acpRefused("read " + req.Path)
	}
	data, err := readHireFile("/", path, acpFileMost)
	if err != nil {
		log.Printf("sandbox: ACP desk %s: reading %s: %v", f.desk.at.ID, path, err)
		return acp.ReadTextFileResponse{}, acpRefused("read " + req.Path)
	}
	return acp.ReadTextFileResponse{Content: acpLines(string(data), req.Line, req.Limit)}, nil
}

// acpLines is limit lines of text from line (counted from 1); all of it when neither is given.
func acpLines(text string, line, limit *int) string {
	lines := strings.SplitAfter(text, "\n")
	from, to := 0, len(lines)
	if line != nil && *line > 1 {
		from = min(*line-1, to)
	}
	if limit != nil && *limit >= 0 {
		to = min(from+*limit, to)
	}
	return strings.Join(lines[from:to], "")
}

// WriteTextFile is fs/write_text_file: req.Content becomes the file at req.Path, by the rules above. The folder must
// be there.
func (f acpFiles) WriteTextFile(_ context.Context, req acp.WriteTextFileRequest) error {
	path, ok := acpResolved(req.Path, true)
	temp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".smolcorp-"+randomHex(8))
	if !ok || !f.allows(path, sandbox.Write) || !f.allows(temp, sandbox.Write) {
		return acpRefused("write " + req.Path)
	}
	if err := writeHireFile(path, temp, []byte(req.Content)); err != nil {
		log.Printf("sandbox: ACP desk %s: writing %s: %v", f.desk.at.ID, path, err)
		return acpRefused("write " + req.Path)
	}
	return nil
}

// writeHireFile makes data the file at path, a resolved path, through a new file at temp in the same folder, renamed
// over path's name; the folder is reached from "/" with no link followed. A file already at path keeps its
// permissions; anything there that isn't a regular file refuses the write.
func writeHireFile(path, temp string, data []byte) error {
	folder := strings.TrimPrefix(filepath.Dir(path), string(filepath.Separator))
	var names []string
	if folder != "" {
		names = strings.Split(folder, string(filepath.Separator))
	}
	dir, err := openHireFolder(string(filepath.Separator), names)
	if err != nil {
		return err
	}
	defer dir.Close()
	name, tempName := filepath.Base(path), filepath.Base(temp)
	perm := fs.FileMode(0o644)
	if info, err := dir.Lstat(name); err == nil {
		if !info.Mode().IsRegular() {
			return errHireFile
		}
		perm = info.Mode().Perm()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	file, err := dir.OpenFile(tempName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = dir.Rename(tempName, name)
	}
	if err != nil {
		dir.Remove(tempName)
	}
	return err
}
