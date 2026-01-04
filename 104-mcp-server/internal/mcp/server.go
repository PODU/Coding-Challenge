package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"sync"
)

const (
	protocolVersion = "2024-11-05"
	serverName      = "ccshared-mcp"
	serverVersion   = "1.0.0"
)

type state int

const (
	stateNew state = iota
	stateInitialised
)

// Server is an MCP server. Serve handles one message at a time, so replies
// are written in request order.
type Server struct {
	fetcher Fetcher
	tools   map[string]Tool

	mu    sync.Mutex
	state state

	logf func(format string, args ...any)
}

// NewServer returns a server that reads the README through f.
func NewServer(f Fetcher) *Server {
	s := &Server{
		fetcher: f,
		state:   stateNew,
		logf:    func(string, ...any) {},
	}
	s.tools = s.buildTools()
	return s
}

// SetLogger sets the log function. It must not write to stdout, which
// carries the protocol.
func (s *Server) SetLogger(logf func(format string, args ...any)) {
	if logf != nil {
		s.logf = logf
	}
}

// Serve answers newline-delimited JSON-RPC messages from in until EOF, which
// returns nil.
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	reader := bufio.NewReaderSize(in, 64*1024)
	writer := bufio.NewWriter(out)
	defer writer.Flush()

	for {
		line, readErr := reader.ReadBytes('\n')
		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			if resp := s.handleLine(trimmed); resp != nil {
				if err := writeResponse(writer, resp); err != nil {
					return err
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return readErr
		}
	}
}

// writeResponse writes resp as one line and flushes so the client isn't left
// waiting.
func writeResponse(w *bufio.Writer, resp *Response) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	return w.Flush()
}

// handleLine returns the reply for one message, or nil for notifications.
func (s *Server) handleLine(line []byte) *Response {
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		s.logf("parse error: %v", err)
		return errorResponse(json.RawMessage("null"), CodeParseError, "parse error: invalid JSON")
	}
	if req.JSONRPC != "2.0" {
		if req.IsNotification() {
			return nil
		}
		return errorResponse(req.ID, CodeInvalidRequest, `invalid request: "jsonrpc" must be "2.0"`)
	}
	return s.dispatch(&req)
}

func (s *Server) dispatch(req *Request) *Response {
	if req.IsNotification() {
		s.logf("notification: %s", req.Method)
		return nil
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "ping":
		return resultResponse(req.ID, struct{}{})
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "resources/list":
		return s.handleResourcesList(req)
	case "resources/read":
		return s.handleResourcesRead(req)
	default:
		return errorResponse(req.ID, CodeMethodNotFound, "method not found: "+req.Method)
	}
}

// handleInitialize advertises capabilities. A second initialize is rejected.
func (s *Server) handleInitialize(req *Request) *Response {
	s.mu.Lock()
	if s.state == stateInitialised {
		s.mu.Unlock()
		return errorResponse(req.ID, CodeInvalidRequest, "server already initialised")
	}
	s.state = stateInitialised
	s.mu.Unlock()

	result := map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]any{
			"tools":     map[string]any{},
			"resources": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    serverName,
			"version": serverVersion,
		},
	}
	return resultResponse(req.ID, result)
}

// requireInitialised returns an error response if initialize hasn't
// happened yet.
func (s *Server) requireInitialised(req *Request) *Response {
	s.mu.Lock()
	st := s.state
	s.mu.Unlock()
	if st != stateInitialised {
		return errorResponse(req.ID, CodeNotInitialised, "server not initialised: send 'initialize' first")
	}
	return nil
}
