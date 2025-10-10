package server

import (
	"greetls/internal/jsonrpc"
	"greetls/internal/lsp"
)

func (s *Server) handleRequest(msg *jsonrpc.Message) {
	method := msg.Method

	if s.state == stateCreated && method != "initialize" {
		s.replyError(msg.ID, jsonrpc.CodeServerNotInitialized, "server not initialized")
		return
	}
	if s.state == stateShuttingDown && method != "shutdown" {
		s.replyError(msg.ID, jsonrpc.CodeInvalidRequest, "server is shutting down")
		return
	}

	switch method {
	case "initialize":
		s.handleInitialize(msg)
	case "shutdown":
		s.handleShutdown(msg)
	case "textDocument/hover":
		s.handleHover(msg)
	case "textDocument/completion":
		s.handleCompletion(msg)
	default:
		s.replyError(msg.ID, jsonrpc.CodeMethodNotFound, "method not found: "+method)
	}
}

func (s *Server) handleInitialize(msg *jsonrpc.Message) {
	s.state = stateInitializing
	result := lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync:   lsp.SyncFull,
			HoverProvider:      true,
			CompletionProvider: &lsp.CompletionOptions{TriggerCharacters: []string{".", "@"}},
		},
		ServerInfo: &lsp.ServerInfo{Name: "greetls", Version: "0.1.0"},
	}
	s.reply(msg.ID, result)
}

func (s *Server) handleShutdown(msg *jsonrpc.Message) {
	s.shutdownReceived = true
	s.state = stateShuttingDown
	s.replyNull(msg.ID)
}
