// Package acp is a client for the Agent Client Protocol (ACP): JSON-RPC 2.0,
// one message per line, over an agent's stdin and stdout. Smolcorp is the
// client; the agent is a child process it starts with a plain argument list
// (never through a shell).
//
// The client asks, it never decides: every permission request, file read or
// write and terminal request the agent sends goes up to the caller's Handler.
// Nothing is approved on its own; without a handler for it, the agent is told
// no.
package acp

import (
	"encoding/json"
	"fmt"
)

// ProtocolVersion is the one ACP version this client speaks. An agent that
// answers initialize with another one is refused (VersionError).
const ProtocolVersion = 1

// JSON-RPC error codes the client sends and recognises.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// RPCError is a JSON-RPC error, from the agent or sent back to it. A Handler
// returns one to answer the agent with its own code and message.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return fmt.Sprintf("acp: %s (%d)", e.Message, e.Code) }

// VersionError: the agent speaks another ACP version than ProtocolVersion.
type VersionError struct{ Agent int }

func (e *VersionError) Error() string {
	return fmt.Sprintf("acp: the agent speaks ACP version %d, Smolcorp speaks %d", e.Agent, ProtocolVersion)
}

// ---- initialize

type Implementation struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

type FileSystemCapability struct {
	ReadTextFile  bool `json:"readTextFile"`
	WriteTextFile bool `json:"writeTextFile"`
}

type ClientCapabilities struct {
	FS       FileSystemCapability `json:"fs"`
	Terminal bool                 `json:"terminal"`
}

type initializeRequest struct {
	ProtocolVersion    int                `json:"protocolVersion"`
	ClientCapabilities ClientCapabilities `json:"clientCapabilities"`
	ClientInfo         *Implementation    `json:"clientInfo,omitempty"`
}

type PromptCapabilities struct {
	Image           bool `json:"image"`
	Audio           bool `json:"audio"`
	EmbeddedContext bool `json:"embeddedContext"`
}

type AgentCapabilities struct {
	LoadSession        bool               `json:"loadSession"`
	PromptCapabilities PromptCapabilities `json:"promptCapabilities"`
}

type AuthMethod struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// InitializeResponse is the agent's answer to initialize. Unknown fields are
// ignored, as the protocol is young.
type InitializeResponse struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities AgentCapabilities `json:"agentCapabilities"`
	AuthMethods       []AuthMethod      `json:"authMethods,omitempty"`
	AgentInfo         *Implementation   `json:"agentInfo,omitempty"`
}

// ---- sessions

// MCPServer is passed through to the agent as given (A1 sends none by default).
type MCPServer = json.RawMessage

type newSessionRequest struct {
	Cwd        string      `json:"cwd"`
	MCPServers []MCPServer `json:"mcpServers"`
}

type loadSessionRequest struct {
	SessionID  string      `json:"sessionId"`
	Cwd        string      `json:"cwd"`
	MCPServers []MCPServer `json:"mcpServers"`
}

type NewSessionResponse struct {
	SessionID string `json:"sessionId"`
}

// ContentBlock is one piece of a prompt or of an update: text, or an image,
// audio or resource the agent said it takes.
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
	URI      string `json:"uri,omitempty"`
}

// Text is a text content block.
func Text(s string) ContentBlock { return ContentBlock{Type: "text", Text: s} }

type promptRequest struct {
	SessionID string         `json:"sessionId"`
	Prompt    []ContentBlock `json:"prompt"`
}

// StopReason says why a turn ended: end_turn, max_tokens,
// max_turn_requests, refusal or cancelled.
type StopReason string

const StopCancelled StopReason = "cancelled"

type PromptResponse struct {
	StopReason StopReason `json:"stopReason"`
}

type cancelNotification struct {
	SessionID string `json:"sessionId"`
}

// SessionNotification is one session/update: a message chunk, a thought, a
// tool call or its progress, a plan, a mode change. The common fields are
// read out; Raw keeps the whole update for the kinds a caller wants more of.
type SessionNotification struct {
	SessionID string        `json:"sessionId"`
	Update    SessionUpdate `json:"update"`
}

type SessionUpdate struct {
	Kind       string          `json:"sessionUpdate"` // agent_message_chunk, tool_call, tool_call_update, …
	Content    *ContentBlock   `json:"-"`             // the chunk, for the *_chunk kinds
	ToolCallID string          `json:"toolCallId,omitempty"`
	Title      string          `json:"title,omitempty"`
	ToolKind   string          `json:"kind,omitempty"`
	Status     string          `json:"status,omitempty"`
	Raw        json.RawMessage `json:"-"`
}

func (u *SessionUpdate) UnmarshalJSON(b []byte) error {
	type plain SessionUpdate
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*u = SessionUpdate(p)
	u.Raw = append(json.RawMessage(nil), b...)
	// A chunk's content is one block; a tool call's is a list: read only the first.
	var c struct {
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(b, &c) == nil && len(c.Content) > 0 && c.Content[0] == '{' {
		var cb ContentBlock
		if json.Unmarshal(c.Content, &cb) == nil {
			u.Content = &cb
		}
	}
	return nil
}

// ---- permission (agent → client)

type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // allow_once, allow_always, reject_once, reject_always
}

type ToolCall struct {
	ToolCallID string          `json:"toolCallId"`
	Title      string          `json:"title,omitempty"`
	Kind       string          `json:"kind,omitempty"`
	Status     string          `json:"status,omitempty"`
	RawInput   json.RawMessage `json:"rawInput,omitempty"`
}

type PermissionRequest struct {
	SessionID string             `json:"sessionId"`
	ToolCall  ToolCall           `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
}

// PermissionOutcome is the caller's answer. An empty OptionID means
// "cancelled" (no option picked); an OptionID that isn't one of the
// request's options is sent as cancelled too, never as a pick.
type PermissionOutcome struct{ OptionID string }

type outcomeWire struct {
	Outcome  string `json:"outcome"` // selected or cancelled
	OptionID string `json:"optionId,omitempty"`
}

type permissionResponse struct {
	Outcome outcomeWire `json:"outcome"`
}

// ---- files (agent → client)

type ReadTextFileRequest struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
	Line      *int   `json:"line,omitempty"`
	Limit     *int   `json:"limit,omitempty"`
}

type ReadTextFileResponse struct {
	Content string `json:"content"`
}

type WriteTextFileRequest struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
	Content   string `json:"content"`
}

// ---- terminals (agent → client)

type EnvVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CreateTerminalRequest struct {
	SessionID       string        `json:"sessionId"`
	Command         string        `json:"command"`
	Args            []string      `json:"args,omitempty"`
	Env             []EnvVariable `json:"env,omitempty"`
	Cwd             string        `json:"cwd,omitempty"`
	OutputByteLimit *int64        `json:"outputByteLimit,omitempty"`
}

type CreateTerminalResponse struct {
	TerminalID string `json:"terminalId"`
}

// TerminalRequest names a terminal: terminal/output, wait_for_exit, kill, release.
type TerminalRequest struct {
	SessionID  string `json:"sessionId"`
	TerminalID string `json:"terminalId"`
}

type TerminalExitStatus struct {
	ExitCode *int    `json:"exitCode,omitempty"`
	Signal   *string `json:"signal,omitempty"`
}

type TerminalOutputResponse struct {
	Output     string              `json:"output"`
	Truncated  bool                `json:"truncated"`
	ExitStatus *TerminalExitStatus `json:"exitStatus,omitempty"`
}
