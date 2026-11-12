package gm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// `smolcorp mcp`: the gm commands as typed tools over MCP (JSON-RPC 2.0, one
// message per line on stdin/stdout). A tool runs the matching `gm` command
// with --json, so its request and its answer are the command's own. No tool
// answers a prompt, and there is none for rotate-token (the page only).

const mcpProtocol = "2024-11-05"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// mcpTool is one tool: its schema, and how its arguments become a gm command line.
type mcpTool struct {
	Name        string
	Description string
	Props       map[string]any // JSON-schema properties
	Required    []string
	Args        func(a map[string]any) ([]string, error)
}

func str(d string) map[string]any    { return map[string]any{"type": "string", "description": d} }
func num(d string) map[string]any    { return map[string]any{"type": "integer", "description": d} }
func flag01(d string) map[string]any { return map[string]any{"type": "boolean", "description": d} }

// argStr, argInt and argBool read a tool argument, "" / 0 / false when absent.
func argStr(a map[string]any, k string) (string, error) {
	v, ok := a[k]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", k)
	}
	return s, nil
}

func argInt(a map[string]any, k string) (int, bool, error) {
	v, ok := a[k]
	if !ok || v == nil {
		return 0, false, nil
	}
	f, ok := v.(float64)
	if !ok || f != float64(int(f)) {
		return 0, false, fmt.Errorf("%s must be a whole number", k)
	}
	return int(f), true, nil
}

func argBool(a map[string]any, k string) (bool, error) {
	v, ok := a[k]
	if !ok || v == nil {
		return false, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be true or false", k)
	}
	return b, nil
}

// build makes a command line from the command, its plain arguments and the
// flags named in order; a text flag is added when given, a switch when true.
func build(cmd string, plain []string, a map[string]any, text, ints, switches []string) ([]string, error) {
	out := append([]string{cmd}, plain...)
	for _, k := range text {
		s, err := argStr(a, k)
		if err != nil {
			return nil, err
		}
		if s != "" {
			out = append(out, "--"+k, s)
		}
	}
	for _, k := range ints {
		n, ok, err := argInt(a, k)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, "--"+k, fmt.Sprint(n))
		}
	}
	for _, k := range switches {
		b, err := argBool(a, k)
		if err != nil {
			return nil, err
		}
		if b {
			out = append(out, "--"+k)
		}
	}
	return out, nil
}

// need reads required text arguments in order.
func need(a map[string]any, keys ...string) ([]string, error) {
	var out []string
	for _, k := range keys {
		s, err := argStr(a, k)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("%s is required", k)
		}
		out = append(out, s)
	}
	return out, nil
}

func plainTool(name, desc string) mcpTool {
	return mcpTool{Name: name, Description: desc, Props: map[string]any{}, Args: func(map[string]any) ([]string, error) { return []string{name}, nil }}
}

func mcpTools() []mcpTool {
	return []mcpTool{
		plainTool("team", "Who is on the team and what each is doing (gm team). The JSON counts desks from 0; the office, and pin and unpin, from 1."),
		{Name: "inbox", Description: "The events after `since`, waiting up to `wait` seconds (0-30) for one (gm inbox).",
			Props: map[string]any{"since": num("only events after this id"), "wait": num("seconds to wait, 0 to 30")},
			Args: func(a map[string]any) ([]string, error) {
				return build("inbox", nil, a, nil, []string{"since", "wait"}, nil)
			}},
		plainTool("version", "The app, its API and its features (gm version)."),
		{Name: "hire", Description: "Hire an employee (gm hire).",
			Props: map[string]any{"role": str("the role"), "folder": str("the folder"), "account": str("the Claude account"), "model": str("the model"),
				"nickname": str("a nickname"), "floor": num("the floor, 0 to 5"), "worktree": flag01("a worktree of its own"),
				"api-key": flag01("on the API-key account"), "task": str("what to do first")},
			Required: []string{"role"},
			Args: func(a map[string]any) ([]string, error) {
				if _, err := need(a, "role"); err != nil {
					return nil, err
				}
				return build("hire", nil, a, []string{"role", "folder", "account", "model", "nickname", "task"}, []string{"floor"}, []string{"worktree", "api-key"})
			}},
		{Name: "brief", Description: "Send a brief to a hire already working (gm brief).",
			Props:    map[string]any{"hire": str("an id, a name or a nickname"), "text": str("the brief")},
			Required: []string{"hire", "text"},
			Args: func(a map[string]any) ([]string, error) {
				p, err := need(a, "hire", "text")
				if err != nil {
					return nil, err
				}
				return []string{"brief", p[0], "--text", p[1]}, nil
			}},
		{Name: "fire", Description: "Fire an employee (gm fire).",
			Props:    map[string]any{"hire": str("an id, a name or a nickname"), "force": flag01("even with unsaved changes"), "remove-worktree": flag01("delete its worktree")},
			Required: []string{"hire"},
			Args: func(a map[string]any) ([]string, error) {
				p, err := need(a, "hire")
				if err != nil {
					return nil, err
				}
				return build("fire", p, a, nil, nil, []string{"force", "remove-worktree"})
			}},
		{Name: "send", Description: "Send a message to a hire. The same `id` twice is delivered once (gm send).",
			Props:    map[string]any{"to": str("who to send to"), "text": str("the message"), "id": str("a message id, for safe repeats")},
			Required: []string{"to", "text"},
			Args: func(a map[string]any) ([]string, error) {
				p, err := need(a, "to", "text")
				if err != nil {
					return nil, err
				}
				return build("send", p[:1], a, []string{"text", "id"}, nil, nil)
			}},
		{Name: "outbox", Description: "The messages sent, oldest first (gm outbox).",
			Props: map[string]any{"since": num("a time in milliseconds since 1970")},
			Args:  func(a map[string]any) ([]string, error) { return build("outbox", nil, a, nil, []string{"since"}, nil) }},
		{Name: "watch", Description: "Ask the app to watch something; firings come to inbox (gm watch).",
			Props: map[string]any{"kind": str("file, event, idle, exit or time"), "target": str("what to watch"), "pattern": str("what to look for"),
				"minutes": num("minutes"), "repeat": flag01("fire again and again")},
			Required: []string{"kind", "target"},
			Args: func(a map[string]any) ([]string, error) {
				p, err := need(a, "kind", "target")
				if err != nil {
					return nil, err
				}
				return build("watch", p, a, []string{"pattern"}, []string{"minutes"}, []string{"repeat"})
			}},
		{Name: "unwatch", Description: "Take a watch away (gm unwatch).",
			Props: map[string]any{"id": str("the watch's id")}, Required: []string{"id"},
			Args: func(a map[string]any) ([]string, error) {
				p, err := need(a, "id")
				if err != nil {
					return nil, err
				}
				return []string{"unwatch", p[0]}, nil
			}},
		{Name: "watches", Description: "The watches (gm watches).",
			Props: map[string]any{"all": flag01("include the app's own")},
			Args:  func(a map[string]any) ([]string, error) { return build("watches", nil, a, nil, nil, []string{"all"}) }},
		{Name: "contacts", Description: "Which hires may write to each other; with `action`, allow or remove a pair (gm contacts).",
			Props: map[string]any{"action": map[string]any{"type": "string", "enum": []string{"allow", "remove"}, "description": "leave out to list"},
				"a": str("the first hire"), "b": str("the second hire"), "one-way": flag01("only from a to b")},
			Args: func(a map[string]any) ([]string, error) {
				act, err := argStr(a, "action")
				if err != nil {
					return nil, err
				}
				if act == "" {
					return []string{"contacts"}, nil
				}
				if act != "allow" && act != "remove" {
					return nil, fmt.Errorf("action is allow or remove")
				}
				p, err := need(a, "a", "b")
				if err != nil {
					return nil, err
				}
				return build("contacts", append([]string{act}, p...), a, nil, nil, []string{"one-way"})
			}},
		{Name: "pin", Description: "Pin a project to a desk (gm pin). Without `desk`, the desk its session sits at now; without `folder`, the server's own folder.",
			Props: map[string]any{"desk": num("the desk, counted from 1 as the office shows"), "folder": str("the project's folder")},
			Args: func(a map[string]any) ([]string, error) {
				return build("pin", nil, a, []string{"folder"}, []string{"desk"}, nil)
			}},
		{Name: "unpin", Description: "Take what is pinned to a desk away (gm unpin).",
			Props: map[string]any{"desk": num("the desk, counted from 1 as the office shows")}, Required: []string{"desk"},
			Args: func(a map[string]any) ([]string, error) {
				if _, ok, err := argInt(a, "desk"); err != nil || !ok {
					if err == nil {
						err = fmt.Errorf("desk is required")
					}
					return nil, err
				}
				return build("unpin", nil, a, nil, []string{"desk"}, nil)
			}},
		{Name: "nickname", Description: "Name whoever works in a folder (gm nickname). An empty `name` takes the nickname away; 16 characters at most.",
			Props:    map[string]any{"name": str("the nickname, or empty for none"), "folder": str("the project's folder; the server's own if left out")},
			Required: []string{"name"},
			Args: func(a map[string]any) ([]string, error) {
				name, err := argStr(a, "name")
				if err != nil {
					return nil, err
				}
				if v, ok := a["name"]; !ok || v == nil {
					return nil, fmt.Errorf("name is required (\"\" for none)")
				}
				return build("nickname", []string{name}, a, []string{"folder"}, nil, nil)
			}},
		plainTool("approvals", "The waits for the owner. Read-only: nothing here answers a prompt (gm approvals)."),
		plainTool("accounts", "The Claude accounts the app knows (gm accounts). Not served on Linux and Windows yet."),
	}
}

type mcpServer struct {
	env   Env
	tools []mcpTool
}

// RunMCP is `smolcorp mcp`; it serves until stdin ends and returns the exit code.
func RunMCP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "smolcorp mcp: takes no argument; it speaks MCP on standard input and output.")
		return exitUsage
	}
	if err := serveMCP(osEnv(), stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "smolcorp mcp: %v\n", err)
		return exitApp
	}
	return exitOK
}

// AppVersion is the app's own, set by main from the one `smolcorp version` prints.
var AppVersion = "dev"

func serveMCP(env Env, in io.Reader, out io.Writer) error {
	s := &mcpServer{env: env, tools: mcpTools()}
	r := bufio.NewReaderSize(in, 1<<20)
	for {
		line, err := r.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			if resp := s.handle(bytes.TrimSpace(line)); resp != nil {
				resp = append(resp, '\n')
				if _, werr := out.Write(resp); werr != nil {
					return werr
				}
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func rpcResult(id json.RawMessage, result any) []byte {
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	return b
}

func rpcError(id json.RawMessage, code int, msg string) []byte {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": msg}})
	return b
}

// handle answers one message; a notification (no id) gets no answer.
func (s *mcpServer) handle(line []byte) []byte {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		return rpcError(nil, -32700, "parse error")
	}
	if len(req.ID) == 0 {
		return nil
	}
	switch req.Method {
	case "initialize":
		return rpcResult(req.ID, map[string]any{
			"protocolVersion": mcpProtocol,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "smolcorp", "version": AppVersion},
		})
	case "ping":
		return rpcResult(req.ID, map[string]any{})
	case "tools/list":
		list := []map[string]any{}
		for _, t := range s.tools {
			schema := map[string]any{"type": "object", "properties": t.Props}
			if len(t.Required) > 0 {
				schema["required"] = t.Required
			}
			list = append(list, map[string]any{"name": t.Name, "description": t.Description, "inputSchema": schema})
		}
		return rpcResult(req.ID, map[string]any{"tools": list})
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return rpcError(req.ID, -32602, "bad params")
		}
		for _, t := range s.tools {
			if t.Name == p.Name {
				return rpcResult(req.ID, s.call(t, p.Arguments))
			}
		}
		return rpcError(req.ID, -32602, fmt.Sprintf("unknown tool %q", p.Name))
	}
	return rpcError(req.ID, -32601, "method not found")
}

func toolText(text string, isErr bool) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isErr}
}

// call runs the tool's gm command with --json; its answer is the tool's text.
func (s *mcpServer) call(t mcpTool, a map[string]any) map[string]any {
	if a == nil {
		a = map[string]any{}
	}
	argv, err := t.Args(a)
	if err != nil {
		return toolText(t.Name+": "+err.Error(), true)
	}
	argv = append(argv, "--json")
	var out, errOut bytes.Buffer
	code := run(argv, s.env, &out, &errOut)
	if code != exitOK {
		msg := strings.TrimSpace(errOut.String())
		// A usage error repeats the whole usage text; the first line says it.
		if i := strings.Index(msg, "\n\n"); i > 0 {
			msg = msg[:i]
		}
		return toolText(fmt.Sprintf("%s (exit %d)", msg, code), true)
	}
	return toolText(strings.TrimSpace(out.String()), false)
}
