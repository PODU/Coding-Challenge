// Package server serves public/ with live reload. Served HTML gets a small
// script that listens on an SSE stream; a poller rebuilds the site when
// content/ or themes/ change and tells the browsers to reload.
package server

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ccsg/internal/builder"
)

// liveReloadSnippet is injected before </body> of served HTML pages.
const liveReloadSnippet = `<script>
(function () {
  if (!window.EventSource) return;
  var es = new EventSource("/__livereload");
  es.onmessage = function (e) { if (e.data === "reload") location.reload(); };
})();
</script>`

// Server serves a built site with live reload.
type Server struct {
	root string

	mu      sync.Mutex
	clients map[chan string]struct{}

	// Log, if set, receives one line per rebuild. Defaults to os.Stdout.
	Log io.Writer
}

// New creates a Server rooted at a site directory (the one containing public/).
func New(root string) *Server {
	return &Server{
		root:    root,
		clients: make(map[chan string]struct{}),
		Log:     os.Stdout,
	}
}

// Handler returns the HTTP handler serving the site and the live-reload stream.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/__livereload", s.handleSSE)
	mux.HandleFunc("/", s.handleStatic)
	return mux
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if upath == "/" {
		upath = "/index.html"
	}
	publicDir := filepath.Join(s.root, "public")
	full := filepath.Join(publicDir, filepath.Clean("/"+upath))
	if full != publicDir && !strings.HasPrefix(full, publicDir+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info.IsDir() {
		full = filepath.Join(full, "index.html")
		if _, err := os.Stat(full); err != nil {
			http.NotFound(w, r)
			return
		}
	}

	if strings.EqualFold(filepath.Ext(full), ".html") {
		data, err := os.ReadFile(full)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, inject(string(data)))
		return
	}
	http.ServeFile(w, r, full)
}

// inject inserts the live-reload snippet before </body>, or appends it if no
// such tag exists.
func inject(html string) string {
	if idx := strings.LastIndex(html, "</body>"); idx >= 0 {
		return html[:idx] + liveReloadSnippet + "\n" + html[idx:]
	}
	return html + "\n" + liveReloadSnippet + "\n"
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan string, 4)
	s.addClient(ch)
	defer s.removeClient(ch)

	_, _ = io.WriteString(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) addClient(ch chan string) {
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
}

func (s *Server) removeClient(ch chan string) {
	s.mu.Lock()
	delete(s.clients, ch)
	s.mu.Unlock()
}

// Broadcast sends a message to every connected live-reload client without
// blocking on slow clients.
func (s *Server) Broadcast(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

// CheckOnce rebuilds and broadcasts a reload if content/ or themes/ changed
// since the last call. It returns true if a rebuild happened. sig is the caller-
// held signature from the previous call; the new signature is returned.
func (s *Server) CheckOnce(sig string) (string, bool, error) {
	cur := s.signature()
	if cur == sig {
		return cur, false, nil
	}
	if _, err := builder.Build(s.root); err != nil {
		return cur, false, err
	}
	s.Broadcast("reload")
	return cur, true, nil
}

// Signature returns a hash of the watched trees.
func (s *Server) Signature() string { return s.signature() }

// signature hashes the relative paths, sizes, and contents of every file under
// content/ and themes/, so any edit is detected regardless of mtime resolution.
func (s *Server) signature() string {
	h := fnv.New64a()
	for _, sub := range []string{"content", "themes"} {
		dir := filepath.Join(s.root, sub)
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(s.root, path)
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			_, _ = io.WriteString(h, filepath.ToSlash(rel))
			_, _ = io.WriteString(h, ":"+strconv.Itoa(len(data))+":")
			_, _ = h.Write(data)
			_, _ = io.WriteString(h, "\n")
			return nil
		})
	}
	return strconv.FormatUint(h.Sum64(), 16)
}

// Watch polls for changes every interval, rebuilding and reloading on change,
// until ctx is cancelled.
func (s *Server) Watch(ctx context.Context, interval time.Duration) {
	sig := s.signature()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			newSig, rebuilt, err := s.CheckOnce(sig)
			sig = newSig
			if err != nil {
				fmt.Fprintln(os.Stderr, "ccsg: rebuild failed:", err)
				continue
			}
			if rebuilt {
				fmt.Fprintln(s.Log, "rebuilt and reloaded")
			}
		}
	}
}

// ListenAndServe runs an initial build, then serves the site on the given port
// and watches for changes until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context, port int) error {
	if _, err := builder.Build(s.root); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	fmt.Fprintf(s.Log, "ccsg: serving http://localhost:%d (Ctrl-C to stop)\n", port)

	go s.Watch(ctx, 500*time.Millisecond)

	srv := &http.Server{Handler: s.Handler()}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
