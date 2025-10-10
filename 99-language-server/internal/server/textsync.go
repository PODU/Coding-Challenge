package server

import (
	"log"

	"greetls/internal/analyzer"
	"greetls/internal/jsonrpc"
	"greetls/internal/lsp"
)

func (s *Server) didOpen(msg *jsonrpc.Message) {
	var p lsp.DidOpenTextDocumentParams
	if err := unmarshalParams(msg.Params, &p); err != nil {
		log.Printf("didOpen: bad params: %v", err)
		return
	}
	s.store.Open(p.TextDocument.URI, p.TextDocument.Text, p.TextDocument.Version)
	s.publishDiagnostics(p.TextDocument.URI, p.TextDocument.Version, p.TextDocument.Text)
}

func (s *Server) didChange(msg *jsonrpc.Message) {
	var p lsp.DidChangeTextDocumentParams
	if err := unmarshalParams(msg.Params, &p); err != nil {
		log.Printf("didChange: bad params: %v", err)
		return
	}
	// We advertise full sync, so the last change holds the whole text.
	text := ""
	if n := len(p.ContentChanges); n > 0 {
		text = p.ContentChanges[n-1].Text
	}
	s.store.Update(p.TextDocument.URI, text, p.TextDocument.Version)
	s.publishDiagnostics(p.TextDocument.URI, p.TextDocument.Version, text)
}

func (s *Server) didClose(msg *jsonrpc.Message) {
	var p lsp.DidCloseTextDocumentParams
	if err := unmarshalParams(msg.Params, &p); err != nil {
		log.Printf("didClose: bad params: %v", err)
		return
	}
	s.store.Close(p.TextDocument.URI)
	// Publishing for empty text clears the client's diagnostics.
	s.publishDiagnostics(p.TextDocument.URI, 0, "")
}

func (s *Server) didSave(msg *jsonrpc.Message) {
	var p lsp.DidSaveTextDocumentParams
	if err := unmarshalParams(msg.Params, &p); err != nil {
		log.Printf("didSave: bad params: %v", err)
		return
	}
	if doc, ok := s.store.Get(p.TextDocument.URI); ok {
		s.publishDiagnostics(doc.URI, doc.Version, doc.Text)
	}
}

func (s *Server) publishDiagnostics(uri string, version int, text string) {
	params := lsp.PublishDiagnosticsParams{
		URI:         uri,
		Version:     version,
		Diagnostics: analyzer.Diagnostics(text),
	}
	if err := s.stream.Write(jsonrpc.NewNotification("textDocument/publishDiagnostics", params)); err != nil {
		log.Printf("publishDiagnostics write error: %v", err)
	}
}

func (s *Server) handleHover(msg *jsonrpc.Message) {
	var p lsp.TextDocumentPositionParams
	if err := unmarshalParams(msg.Params, &p); err != nil {
		s.replyError(msg.ID, jsonrpc.CodeInvalidParams, "invalid hover params")
		return
	}
	doc, ok := s.store.Get(p.TextDocument.URI)
	if !ok {
		s.replyNull(msg.ID)
		return
	}
	h := analyzer.Hover(doc.Text, p.Position)
	if h == nil {
		s.replyNull(msg.ID)
		return
	}
	s.reply(msg.ID, h)
}

func (s *Server) handleCompletion(msg *jsonrpc.Message) {
	var p lsp.CompletionParams
	if err := unmarshalParams(msg.Params, &p); err != nil {
		s.replyError(msg.ID, jsonrpc.CodeInvalidParams, "invalid completion params")
		return
	}
	text := ""
	if doc, ok := s.store.Get(p.TextDocument.URI); ok {
		text = doc.Text
	}
	items := analyzer.Completion(text, p.Position)
	s.reply(msg.ID, lsp.CompletionList{IsIncomplete: false, Items: items})
}
