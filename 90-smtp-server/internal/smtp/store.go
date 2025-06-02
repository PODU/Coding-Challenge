package smtp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MessageStore saves accepted messages and returns an id for each.
type MessageStore interface {
	Save(env Envelope) (id string, err error)
}

// idCounter keeps ids unique when two saves land on the same nanosecond.
var idCounter uint64

func nextID(t time.Time) string {
	seq := atomic.AddUint64(&idCounter, 1)
	return fmt.Sprintf("%d-%06d", t.UnixNano(), seq)
}

// formatMessage prepends X-CC-* envelope headers and a blank line to the body.
func formatMessage(env Envelope) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "X-CC-From: <%s>\r\n", env.From)
	to := make([]string, len(env.To))
	for i, r := range env.To {
		to[i] = "<" + r + ">"
	}
	fmt.Fprintf(&b, "X-CC-To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "X-CC-Received: %s\r\n", env.Received.UTC().Format(time.RFC3339))
	b.WriteString("\r\n")
	b.Write(env.Data)
	return b.Bytes()
}

// MaildirStore writes each message to <dir>/<id>.eml.
type MaildirStore struct {
	dir string
	mu  sync.Mutex
}

// NewMaildirStore creates dir if needed.
func NewMaildirStore(dir string) (*MaildirStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create maildir: %w", err)
	}
	return &MaildirStore{dir: dir}, nil
}

// Save writes to a temp file, syncs it, and renames it into place so readers
// never see a partial message.
func (s *MaildirStore) Save(env Envelope) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := nextID(env.Received)
	final := filepath.Join(s.dir, id+".eml")
	tmp := final + ".tmp"

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(formatMessage(env)); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return id, nil
}

// MemoryStore keeps messages in memory. Tests use it.
type MemoryStore struct {
	mu       sync.Mutex
	Messages []Envelope
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

func (s *MemoryStore) Save(env Envelope) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := nextID(env.Received)
	s.Messages = append(s.Messages, env)
	return id, nil
}

func (s *MemoryStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Messages)
}

// All returns a copy of the stored envelopes.
func (s *MemoryStore) All() []Envelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Envelope, len(s.Messages))
	copy(out, s.Messages)
	return out
}
