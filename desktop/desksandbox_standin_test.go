package desktop

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"slices"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// deskStandInVariable runs this test program as the three programs of a sandboxed desk's start on a Mac, end to end
// (desksandbox_darwin_test.go), and as a stand-in ACP agent (`acp-agent`, acpsandbox_darwin_test.go), told apart by
// their words: `smolcorp sandbox enter …`; the bundled tmux's start call
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
	case args[0] == "acp-agent":
		return acpStandInAgent(os.Stdin, os.Stdout)
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

// acpStandInAgent is a stand-in ACP agent on in and out: it answers initialize and session/new, and takes each prompt as
// a script, a JSON list of steps. Each step is a request it sends the client (its params' "terminalId" of "$last" is the
// last terminal the client made for it), or "self/read", a file it reads itself, where it runs. Once the steps are
// done it says their answers, as a JSON list, in one agent_message_chunk, then ends the turn.
func acpStandInAgent(in io.Reader, out io.Writer) int {
	lines := bufio.NewScanner(in)
	lines.Buffer(make([]byte, 0, 64<<10), 32<<20)
	send := func(m map[string]any) { b, _ := json.Marshal(m); out.Write(append(b, '\n')) }
	next := 100
	call := func(method string, params any) map[string]any {
		next++
		send(map[string]any{"jsonrpc": "2.0", "id": next, "method": method, "params": params})
		for lines.Scan() {
			var m map[string]any
			if json.Unmarshal(lines.Bytes(), &m) == nil && m["id"] == float64(next) && m["method"] == nil {
				return m
			}
		}
		return map[string]any{"error": "the client went away"}
	}
	for lines.Scan() {
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				Prompt []struct{ Text string } `json:"prompt"`
			} `json:"params"`
		}
		if json.Unmarshal(lines.Bytes(), &m) != nil || len(m.ID) == 0 {
			continue
		}
		reply := func(result any) { send(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": result}) }
		switch m.Method {
		case "initialize":
			reply(map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{}})
		case "session/new":
			reply(map[string]any{"sessionId": "s1"})
		case "session/prompt":
			var steps []struct {
				Method string         `json:"method"`
				Params map[string]any `json:"params"`
			}
			if len(m.Params.Prompt) > 0 {
				json.Unmarshal([]byte(m.Params.Prompt[0].Text), &steps)
			}
			var answers []map[string]any
			last := ""
			for _, step := range steps {
				if step.Method == "self/read" {
					data, err := os.ReadFile(step.Params["path"].(string))
					if err != nil {
						answers = append(answers, map[string]any{"error": err.Error()})
					} else {
						answers = append(answers, map[string]any{"result": string(data)})
					}
					continue
				}
				step.Params["sessionId"] = "s1"
				if step.Params["terminalId"] == "$last" {
					step.Params["terminalId"] = last
				}
				answer := call(step.Method, step.Params)
				if r, ok := answer["result"].(map[string]any); ok && r["terminalId"] != nil {
					last, _ = r["terminalId"].(string)
				}
				answers = append(answers, answer)
			}
			said, _ := json.Marshal(answers)
			send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": "s1",
				"update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": string(said)}}}})
			reply(map[string]any{"stopReason": "end_turn"})
		default:
			send(map[string]any{"jsonrpc": "2.0", "id": m.ID, "error": map[string]any{"code": -32601, "message": "method not found"}})
		}
	}
	return 0
}
