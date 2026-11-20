//go:build unix

package tmuxfake

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
)

// The message types (tmux-protocol.h).
const (
	MsgVersion         uint32 = 12
	MsgIdentifyEnviron uint32 = 105
	MsgIdentifyDone    uint32 = 106
	MsgCommand         uint32 = 200
	MsgDetach          uint32 = 201
	MsgDetachKill      uint32 = 202
	MsgExit            uint32 = 203
	MsgExited          uint32 = 204
	MsgExiting         uint32 = 205
	MsgLock            uint32 = 206
	MsgReady           uint32 = 207
	MsgResize          uint32 = 208
	MsgShell           uint32 = 209
	MsgShutdown        uint32 = 210
	MsgOldStderr       uint32 = 211
	MsgSuspend         uint32 = 214
	MsgUnlock          uint32 = 215
	MsgWakeup          uint32 = 216
	MsgExec            uint32 = 217
	MsgFlags           uint32 = 218
	MsgReadOpen        uint32 = 300
	MsgRead            uint32 = 301
	MsgReadDone        uint32 = 302
	MsgWriteOpen       uint32 = 303
	MsgWrite           uint32 = 304
	MsgWriteReady      uint32 = 305
	MsgWriteClose      uint32 = 306
	MsgReadCancel      uint32 = 307
	MsgWriteDone       uint32 = 308
)

const (
	protocolVersion = 8
	headerSize      = 16
	maxMessage      = 16384
)

// Message is one message, either way: its type and what follows the header.
type Message struct {
	Type uint32
	Data []byte
}

func ints(values ...int32) []byte {
	out := make([]byte, 0, 4*len(values))
	for _, v := range values {
		out = binary.NativeEndian.AppendUint32(out, uint32(v))
	}
	return out
}

func text(s string) []byte { return append([]byte(s), 0) }

// Simple is a message with nothing after its header (MSG_READY, MSG_EXITED, MSG_SHUTDOWN, MSG_SUSPEND...).
func Simple(t uint32) Message { return Message{Type: t} }

// Exit ends a client with that status.
func Exit(status int32) Message { return Message{MsgExit, ints(status)} }

// ReadOpen asks the client to read: fd 0 for its stdin, -1 for the file at path.
func ReadOpen(stream, fd int32, path string) Message {
	data := ints(stream, fd)
	if path != "" {
		data = append(data, text(path)...)
	}
	return Message{MsgReadOpen, data}
}

// WriteOpen asks the client to write: fd 1 or 2 for its own, -1 for the file at path, opened with flags added.
func WriteOpen(stream, fd, flags int32, path string) Message {
	data := ints(stream, fd, flags)
	if path != "" {
		data = append(data, text(path)...)
	}
	return Message{MsgWriteOpen, data}
}

// Write gives the client data for a stream it opened.
func Write(stream int32, data string) Message {
	return Message{MsgWrite, append(ints(stream), data...)}
}

// WriteClose ends a stream the client opened.
func WriteClose(stream int32) Message { return Message{MsgWriteClose, ints(stream)} }

// Shell names a program for the client to run (a stock one-shot client runs it).
func Shell(program string) Message { return Message{MsgShell, text(program)} }

// Exec asks an attached client to run command in shell when it ends (detach-client -E).
func Exec(command, shell string) Message {
	return Message{MsgExec, append(text(command), text(shell)...)}
}

// Lock asks an attached client to run command (lock-client).
func Lock(command string) Message { return Message{MsgLock, text(command)} }

// Detach detaches an attached client; kill is MSG_DETACHKILL, which makes a stock client SIGHUP its parent.
func Detach(session string, kill bool) Message {
	if kill {
		return Message{MsgDetachKill, text(session)}
	}
	return Message{MsgDetach, text(session)}
}

// Flags replaces the client's flags (8 bytes).
func Flags(flags uint64) Message {
	return Message{MsgFlags, binary.NativeEndian.AppendUint64(nil, flags)}
}

// Step waits for the client to send a message of type After (0: don't wait), then sends Send.
type Step struct {
	After uint32
	Send  []Message
}

// Session is what one connection received from its client, in order.
type Session struct {
	Received []Message
}

// Has says whether the client sent a message of type t.
func (s Session) Has(t uint32) bool {
	return slices.ContainsFunc(s.Received, func(m Message) bool { return m.Type == t })
}

// Environ is the environment the client sent (MSG_IDENTIFY_ENVIRON), one "NAME=value" each.
func (s Session) Environ() []string {
	var out []string
	for _, m := range s.Received {
		if m.Type == MsgIdentifyEnviron {
			out = append(out, strings.TrimRight(string(m.Data), "\x00"))
		}
	}
	return out
}

// Read is what the client read for the server on stream (MSG_READ), put together.
func (s Session) Read(stream int32) string {
	var out []byte
	for _, m := range s.Received {
		if m.Type == MsgRead && len(m.Data) >= 4 && int32(binary.NativeEndian.Uint32(m.Data)) == stream {
			out = append(out, m.Data[4:]...)
		}
	}
	return string(out)
}

// Server is a fake tmux server on a unix socket. Every client that connects gets the same steps.
type Server struct {
	Path     string
	steps    []Step
	listener *net.UnixListener
	mu       sync.Mutex
	sessions []*Session
	wg       sync.WaitGroup
}

// Listen starts a fake server at path (which must not exist yet) answering each client with steps.
func Listen(path string, steps ...Step) (*Server, error) {
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	s := &Server{Path: path, steps: steps, listener: listener}
	s.wg.Add(1)
	go s.accept()
	return s, nil
}

func (s *Server) accept() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.AcceptUnix()
		if err != nil {
			return
		}
		session := &Session{}
		s.mu.Lock()
		s.sessions = append(s.sessions, session)
		s.mu.Unlock()
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer conn.Close()
			s.serve(conn, session)
		}()
	}
}

// serve plays the steps, then keeps reading (and recording) until the client hangs up.
func (s *Server) serve(conn *net.UnixConn, session *Session) {
	r := &reader{conn: conn}
	record := func() (Message, error) {
		m, err := r.next()
		if err == nil {
			s.mu.Lock()
			session.Received = append(session.Received, m)
			s.mu.Unlock()
		}
		return m, err
	}
	for _, step := range s.steps {
		for step.After != 0 {
			m, err := record()
			if err != nil {
				return
			}
			if m.Type == step.After {
				break
			}
		}
		for _, m := range step.Send {
			if _, err := conn.Write(encode(m)); err != nil {
				return
			}
		}
	}
	for {
		if _, err := record(); err != nil {
			return
		}
	}
}

// Sessions is a copy of what every client so far sent, one per connection.
func (s *Server) Sessions() []Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Session, len(s.sessions))
	for i, session := range s.sessions {
		out[i] = Session{Received: slices.Clone(session.Received)}
	}
	return out
}

// Close stops listening, waits up to two seconds for the open connections to end, and removes the socket.
func (s *Server) Close() {
	s.listener.Close()
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	os.Remove(s.Path)
}

func encode(m Message) []byte {
	out := make([]byte, 0, headerSize+len(m.Data))
	out = binary.NativeEndian.AppendUint32(out, m.Type)
	out = binary.NativeEndian.AppendUint32(out, uint32(headerSize+len(m.Data)))
	out = binary.NativeEndian.AppendUint32(out, protocolVersion)
	out = binary.NativeEndian.AppendUint32(out, uint32(os.Getpid()))
	return append(out, m.Data...)
}

// reader splits what a client sends into messages, and closes the file descriptors it passes (its stdin and
// stdout, with the identify messages) as they come.
type reader struct {
	conn *net.UnixConn
	buf  []byte
}

func (r *reader) next() (Message, error) {
	for {
		if len(r.buf) >= headerSize {
			// The length is the low 16 bits: tmux 3.8 marks a passed fd in its top bit, older ones in the next 16.
			size := int(binary.NativeEndian.Uint32(r.buf[4:8]) & 0xffff)
			if size < headerSize || size > maxMessage {
				return Message{}, errors.New("tmuxfake: a message of a size tmux never sends")
			}
			if len(r.buf) >= size {
				m := Message{Type: binary.NativeEndian.Uint32(r.buf[0:4]), Data: slices.Clone(r.buf[headerSize:size])}
				r.buf = r.buf[size:]
				return m, nil
			}
		}
		chunk, oob := make([]byte, maxMessage), make([]byte, syscall.CmsgSpace(4*16))
		n, oobn, _, _, err := r.conn.ReadMsgUnix(chunk, oob)
		closePassed(oob[:oobn])
		if n == 0 && err == nil {
			err = io.EOF
		}
		if err != nil {
			return Message{}, err
		}
		r.buf = append(r.buf, chunk[:n]...)
	}
}

func closePassed(oob []byte) {
	messages, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return
	}
	for _, m := range messages {
		fds, _ := syscall.ParseUnixRights(&m)
		for _, fd := range fds {
			syscall.Close(fd)
		}
	}
}
