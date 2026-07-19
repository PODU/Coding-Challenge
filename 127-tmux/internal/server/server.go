//go:build linux

// Package server is the cctmux daemon. It owns the sessions and their shells,
// accepts clients on a Unix socket, and sends each attached client a fresh
// frame whenever something changes. It exits when the last session ends.
package server

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"cctmux/internal/command"
	"cctmux/internal/input"
	"cctmux/internal/protocol"
	"cctmux/internal/pty"
	"cctmux/internal/render"
	"cctmux/internal/session"
)

type Server struct {
	sockPath string
	shell    string
	prefix   byte

	ln  net.Listener
	mgr *session.Manager

	mu      sync.Mutex
	readers map[*session.Pane]bool // panes that already have a readPane goroutine
	redraw  chan struct{}          // buffered (1), so redraw requests coalesce
	done    chan struct{}
}

// Run serves on sockPath until the last session ends. termW x termH is the
// terminal size assumed until a client reports its own.
func Run(sockPath, shell string, prefix byte, termW, termH int) error {
	if _, err := os.Stat(sockPath); err == nil {
		// Stale socket from a previous run; a live server would already hold it.
		os.Remove(sockPath)
	}
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", sockPath, err)
	}
	s := &Server{
		sockPath: sockPath,
		shell:    shell,
		prefix:   prefix,
		ln:       ln,
		mgr:      session.NewManager(factory(shell), termW, termH),
		readers:  map[*session.Pane]bool{},
		redraw:   make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
	defer os.Remove(sockPath)

	go s.acceptLoop()
	<-s.done
	return nil
}

func factory(shell string) session.PTYFactory {
	return func(w, h int) (pty.PTY, error) { return pty.OpenShell(shell, w, h) }
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

// signalRedraw asks for a repaint without blocking.
func (s *Server) signalRedraw() {
	select {
	case s.redraw <- struct{}{}:
	default:
	}
}

func (s *Server) shutdown() {
	s.ln.Close()
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

// handle serves one connection. The first message says what the client wants;
// for new or attach the connection then stays open.
func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	msg, err := protocol.ReadMessage(conn)
	if err != nil {
		return
	}
	switch msg.Type {
	case protocol.ListSessions:
		protocol.WriteMessage(conn, protocol.SessionList, []byte(s.mgr.FormatList()))
	case protocol.KillSession:
		if s.mgr.Kill(msg.Str()) {
			protocol.WriteMessage(conn, protocol.OK, nil)
		} else {
			protocol.WriteMessage(conn, protocol.Error, []byte("no such session: "+msg.Str()))
		}
		s.maybeShutdown()
	case protocol.NewSession:
		sess, err := s.mgr.Create(msg.Str())
		if err != nil {
			protocol.WriteMessage(conn, protocol.Error, []byte(err.Error()))
			return
		}
		s.attach(conn, sess)
	case protocol.AttachSession:
		sess := s.mgr.Get(msg.Str())
		if sess == nil {
			protocol.WriteMessage(conn, protocol.Error, []byte("no such session: "+msg.Str()))
			return
		}
		s.attach(conn, sess)
	default:
		protocol.WriteMessage(conn, protocol.Error, []byte("unexpected message"))
	}
}

func (s *Server) attach(conn net.Conn, sess *session.Session) {
	sess.Attached = true
	defer func() { sess.Attached = false }()

	disp := input.New(sess, s.prefix)
	s.ensureReaders(sess)

	// The command prompt opened with prefix+:.
	var prompt struct {
		active bool
		buf    []byte
	}

	stop := make(chan struct{})
	var once sync.Once
	stopRender := func() { once.Do(func() { close(stop) }) }
	defer stopRender()

	// Repaint on request, and every second to keep the clock current.
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			s.paint(conn, sess, prompt.active, prompt.buf)
			select {
			case <-stop:
				return
			case <-s.redraw:
			case <-tick.C:
			}
		}
	}()

	for {
		msg, err := protocol.ReadMessage(conn)
		if err != nil {
			return // client disconnected without detaching; session persists
		}
		switch msg.Type {
		case protocol.Detach:
			return
		case protocol.Resize:
			if w, h, err := protocol.DecodeResize(msg.Payload); err == nil {
				s.mgr.SetTermSize(w, h)
				s.signalRedraw()
			}
		case protocol.Command:
			s.runCommand(sess, msg.Str())
			s.signalRedraw()
		case protocol.Input:
			for _, b := range msg.Payload {
				if prompt.active {
					if done, line := editPrompt(&prompt.buf, b); done {
						prompt.active = false
						if line != "" {
							s.runCommand(sess, line)
						}
						prompt.buf = nil
					}
					s.signalRedraw()
					continue
				}
				res := disp.Dispatch(b)
				if len(res.Forward) > 0 {
					if p := sess.ActiveWindow().ActivePane(); p != nil {
						p.PTY.Write(res.Forward)
					}
				}
				if res.Prompt {
					prompt.active = true
					prompt.buf = nil
				}
				if res.Detach {
					return
				}
				s.ensureReaders(sess) // new panes may have appeared
				s.signalRedraw()
			}
		}
	}
}

func (s *Server) runCommand(sess *session.Session, line string) {
	res, err := command.Exec(s.mgr, sess, line)
	if err != nil {
		return
	}
	if res.KillSession != "" {
		s.mgr.Kill(res.KillSession)
		s.maybeShutdown()
	}
	s.ensureReaders(sess)
}

// paint sends the client a full frame. While the prompt is open it replaces
// the status bar.
func (s *Server) paint(conn net.Conn, sess *session.Session, promptActive bool, promptBuf []byte) {
	w, h := s.mgr.TermSize()
	grid := render.Render(sess, w, h, w, time.Now().Format("15:04:05"))
	if promptActive {
		line := []rune(":" + string(promptBuf))
		for x := 0; x < w; x++ {
			r := ' '
			if x < len(line) {
				r = line[x]
			}
			grid.Set(x, h-1, r, "7")
		}
	}
	protocol.WriteMessage(conn, protocol.Output, render.Emit(grid))
}

// ensureReaders starts readPane for any pane that doesn't have one yet.
func (s *Server) ensureReaders(sess *session.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range sess.Windows() {
		for _, p := range w.Panes() {
			if s.readers[p] {
				continue
			}
			s.readers[p] = true
			go s.readPane(p)
		}
	}
}

// readPane copies a pane's shell output onto its screen. Output that arrives
// in copy mode is dropped.
func (s *Server) readPane(p *session.Pane) {
	buf := make([]byte, 4096)
	for {
		n, err := p.PTY.Read(buf)
		if n > 0 && !p.Copy {
			p.Screen.Write(buf[:n])
			s.signalRedraw()
		}
		if err != nil {
			s.mu.Lock()
			delete(s.readers, p)
			s.mu.Unlock()
			return
		}
	}
}

func (s *Server) maybeShutdown() {
	if s.mgr.Empty() {
		s.shutdown()
	}
}

// editPrompt applies one key to the prompt line. On Enter it returns true and
// the line; on Escape it returns true and "".
func editPrompt(buf *[]byte, b byte) (bool, string) {
	switch b {
	case '\r', '\n':
		return true, string(*buf)
	case 0x1b:
		return true, ""
	case 0x7f, '\b':
		if len(*buf) > 0 {
			*buf = (*buf)[:len(*buf)-1]
		}
	default:
		if b >= 0x20 {
			*buf = append(*buf, b)
		}
	}
	return false, ""
}
