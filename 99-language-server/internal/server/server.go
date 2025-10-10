// Package server implements the LSP session: lifecycle, document sync and the
// .greet language features.
package server

import (
	"encoding/json"
	"errors"
	"io"
	"log"

	"greetls/internal/document"
	"greetls/internal/jsonrpc"
)

type state int

const (
	stateCreated state = iota
	stateInitializing
	stateReady
	stateShuttingDown
)

// Server is one LSP session over a single stdio stream.
type Server struct {
	stream           *jsonrpc.Stream
	store            *document.Store
	state            state
	shutdownReceived bool
}

// New constructs a Server reading from r and writing to w.
func New(r io.Reader, w io.Writer) *Server {
	return &Server{
		stream: jsonrpc.NewStream(r, w),
		store:  document.NewStore(),
		state:  stateCreated,
	}
}

// Run handles messages until the input ends or an exit notification arrives,
// and returns the process exit code: 1 if exit came without a shutdown first,
// otherwise 0.
func (s *Server) Run() int {
	for {
		msg, err := s.stream.Read()
		if err != nil {
			if code, stop := s.handleReadError(err); stop {
				return code
			}
			continue
		}
		if done, code := s.dispatch(msg); done {
			return code
		}
	}
}

func (s *Server) handleReadError(err error) (code int, stop bool) {
	switch {
	case errors.Is(err, io.EOF):
		log.Println("stdin closed, ending read loop")
		return 0, true
	case errors.Is(err, io.ErrUnexpectedEOF):
		log.Println("unexpected EOF mid-message, ending read loop")
		return 0, true
	}

	var pe *jsonrpc.ParseError
	if errors.As(err, &pe) {
		if len(pe.ID) > 0 {
			s.replyError(pe.ID, jsonrpc.CodeParseError, "parse error: invalid JSON")
		} else {
			log.Printf("dropping unparseable message: %v", pe)
		}
		return 0, false
	}

	var fe *jsonrpc.FramingError
	if errors.As(err, &fe) {
		log.Printf("framing error (resyncing): %v", fe)
		return 0, false
	}

	log.Printf("fatal read error: %v", err)
	return 1, true
}

func (s *Server) dispatch(msg *jsonrpc.Message) (done bool, code int) {
	switch {
	case msg.IsRequest():
		s.handleRequest(msg)
		return false, 0
	case msg.IsNotification():
		return s.handleNotification(msg)
	default:
		// We never send requests, so a response has nothing to match up with.
		return false, 0
	}
}

func (s *Server) handleNotification(msg *jsonrpc.Message) (done bool, code int) {
	switch msg.Method {
	case "initialized":
		s.state = stateReady
	case "exit":
		if s.shutdownReceived {
			return true, 0
		}
		return true, 1
	case "textDocument/didOpen":
		s.didOpen(msg)
	case "textDocument/didChange":
		s.didChange(msg)
	case "textDocument/didClose":
		s.didClose(msg)
	case "textDocument/didSave":
		s.didSave(msg)
	}
	return false, 0
}

func (s *Server) reply(id json.RawMessage, result interface{}) {
	raw, err := json.Marshal(result)
	if err != nil {
		s.replyError(id, jsonrpc.CodeInternalError, "failed to marshal result")
		return
	}
	if err := s.stream.Write(&jsonrpc.Message{ID: id, Result: raw}); err != nil {
		log.Printf("write error: %v", err)
	}
}

func (s *Server) replyNull(id json.RawMessage) {
	if err := s.stream.Write(&jsonrpc.Message{ID: id, Result: json.RawMessage("null")}); err != nil {
		log.Printf("write error: %v", err)
	}
}

func (s *Server) replyError(id json.RawMessage, code int, message string) {
	if len(id) == 0 {
		return // notifications get no reply
	}
	if err := s.stream.Write(&jsonrpc.Message{ID: id, Error: &jsonrpc.Error{Code: code, Message: message}}); err != nil {
		log.Printf("write error: %v", err)
	}
}

// unmarshalParams treats missing or null params as empty.
func unmarshalParams(raw json.RawMessage, v interface{}) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, v)
}
