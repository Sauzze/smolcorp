package desktop

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"slices"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// deskStandInVariable runs this test program as the three programs of a sandboxed desk's start on a Mac, end to end
// (desksandbox_darwin_test.go), told apart by their words: `smolcorp sandbox enter …`; the bundled tmux's start call
// (`-f /dev/null -S <socket> start-server ; …`), which starts a server and says its pane as tmux's -P -F does, the
// server's pid standing in for the pane's; and that server (`serve <socket>`), which listens there and holds every
// connection open, as tmux does.
const deskStandInVariable = "SMOLCORP_DESK_STANDIN"

func deskStandIn(args []string) int {
	switch {
	case args[0] == "sandbox":
		return sandbox.Run(args[1:], os.Stdout, os.Stderr)
	case args[0] == "serve" && len(args) == 2:
		ln, err := net.Listen("unix", args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("listening")
		var held []net.Conn
		for {
			conn, err := ln.Accept()
			if err != nil {
				return 1
			}
			held = append(held, conn)
		}
	case args[0] == "-f":
		i := slices.Index(args, "-S")
		if i < 0 || i+1 >= len(args) {
			return 2
		}
		self, err := os.Executable()
		if err != nil {
			return 1
		}
		server := disclaim.Cmd(exec.Command(self, "serve", args[i+1]))
		out, err := server.StdoutPipe()
		if err != nil || server.Start() != nil {
			return 1
		}
		if line, _ := bufio.NewReader(out).ReadString('\n'); line != "listening\n" {
			return 1
		}
		fmt.Printf("%%0 %d %s\n", server.Process.Pid, args[i+1])
		return 0
	}
	return 2
}
