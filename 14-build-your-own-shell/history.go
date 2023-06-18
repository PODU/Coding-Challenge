package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// History is the command history, kept in memory and in a file. A command
// that repeats the previous one is not recorded again.
type History struct {
	mu      sync.Mutex
	entries []string
	path    string
}

// historyPath returns ~/.ccsh_history, or a file in the current directory if
// there is no home directory.
func historyPath() string {
	home := homeDir()
	if home == "" {
		return ".ccsh_history"
	}
	return filepath.Join(home, ".ccsh_history")
}

// NewHistory returns a History stored at path. The file need not exist yet.
func NewHistory(path string) *History {
	return &History{path: path}
}

// Load reads the history file. A missing file is not an error.
func (h *History) Load() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	f, err := os.Open(h.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	h.entries = h.entries[:0]
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		h.entries = append(h.entries, line)
	}
	return scanner.Err()
}

// Add records cmd and appends it to the file. If the write fails, cmd is still
// kept in memory.
func (h *History) Add(cmd string) error {
	cmd = strings.TrimRight(cmd, "\r\n")
	if strings.TrimSpace(cmd) == "" {
		return nil
	}

	h.mu.Lock()
	if n := len(h.entries); n > 0 && h.entries[n-1] == cmd {
		h.mu.Unlock()
		return nil
	}
	h.entries = append(h.entries, cmd)
	path := h.path
	h.mu.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, cmd)
	return err
}

// Entries returns a copy of the in-memory history.
func (h *History) Entries() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.entries))
	copy(out, h.entries)
	return out
}

// Clear empties the history and deletes the file.
func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = nil
	_ = os.Remove(h.path)
}

// Save rewrites the history file from memory.
func (h *History) Save() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	f, err := os.OpenFile(h.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, e := range h.entries {
		if _, err := fmt.Fprintln(w, e); err != nil {
			return err
		}
	}
	return w.Flush()
}
