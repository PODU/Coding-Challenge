// Package document keeps the text of open documents, keyed by URI.
package document

import "sync"

// Document is a snapshot of one open text document.
type Document struct {
	URI     string
	Text    string
	Version int
}

// Store is safe for concurrent use.
type Store struct {
	mu   sync.RWMutex
	docs map[string]Document
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{docs: make(map[string]Document)}
}

// Open stores a newly opened document, replacing any previous copy.
func (s *Store) Open(uri, text string, version int) Document {
	return s.put(uri, text, version)
}

// Update replaces a document's text and version, creating it if the URI is
// unknown.
func (s *Store) Update(uri, text string, version int) Document {
	return s.put(uri, text, version)
}

func (s *Store) put(uri, text string, version int) Document {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := Document{URI: uri, Text: text, Version: version}
	s.docs[uri] = d
	return d
}

// Get returns a snapshot of the document and whether it exists.
func (s *Store) Get(uri string) (Document, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.docs[uri]
	return d, ok
}

// Close removes a document from the store.
func (s *Store) Close(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, uri)
}

// Len returns the number of open documents.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.docs)
}
