// Package session is the server's model: a Manager holds named sessions, a
// session holds windows, and a window holds panes, each of which is a shell on
// a PTY plus the screen it draws on. PTYs come from a PTYFactory so tests can
// supply mocks.
package session

import (
	"fmt"
	"sort"

	"cctmux/internal/layout"
	"cctmux/internal/pty"
	"cctmux/internal/screen"
)

type PTYFactory func(w, h int) (pty.PTY, error)

type Pane struct {
	ID     int
	PTY    pty.PTY
	Screen *screen.Screen

	// In copy mode the pane stops updating and ScrollOff is how many lines
	// it is scrolled back from the bottom.
	Copy      bool
	ScrollOff int
}

type Window struct {
	ID     int
	Name   string
	Layout *layout.Tree
	panes  map[int]*Pane
}

func (w *Window) ActivePane() *Pane { return w.panes[w.Layout.Active()] }

func (w *Window) Pane(id int) *Pane { return w.panes[id] }

// Panes returns the panes in layout order.
func (w *Window) Panes() []*Pane {
	ids := w.Layout.PaneIDs()
	out := make([]*Pane, 0, len(ids))
	for _, id := range ids {
		if p := w.panes[id]; p != nil {
			out = append(out, p)
		}
	}
	return out
}

type Session struct {
	Name     string
	Attached bool

	windows []*Window
	active  int
	nextWin int
	newPTY  PTYFactory
	areaW   int
	areaH   int // terminal height minus the status bar
}

func (s *Session) ActiveWindow() *Window { return s.windows[s.active] }

func (s *Session) ActiveIndex() int { return s.active }

func (s *Session) Windows() []*Window { return s.windows }

func (s *Session) WindowCount() int { return len(s.windows) }

func (s *Session) area() layout.Rect { return layout.Rect{X: 0, Y: 0, W: s.areaW, H: s.areaH} }

func (s *Session) newPane(id int, r layout.Rect) (*Pane, error) {
	w, h := max1(r.W), max1(r.H)
	p, err := s.newPTY(w, h)
	if err != nil {
		return nil, err
	}
	return &Pane{ID: id, PTY: p, Screen: screen.New(w, h)}, nil
}

func (s *Session) addWindow(name string) (*Window, error) {
	win := &Window{ID: s.nextWin, Name: name, Layout: layout.New(), panes: map[int]*Pane{}}
	s.nextWin++
	p, err := s.newPane(0, s.area())
	if err != nil {
		return nil, err
	}
	win.panes[0] = p
	s.windows = append(s.windows, win)
	return win, nil
}

// NewWindow adds a window and switches to it.
func (s *Session) NewWindow(name string) (*Window, error) {
	if name == "" {
		name = "bash"
	}
	w, err := s.addWindow(name)
	if err != nil {
		return nil, err
	}
	s.active = len(s.windows) - 1
	s.relayout()
	return w, nil
}

func (s *Session) SelectWindow(i int) bool {
	if i < 0 || i >= len(s.windows) {
		return false
	}
	s.active = i
	return true
}

func (s *Session) NextWindow() { s.active = (s.active + 1) % len(s.windows) }

func (s *Session) PrevWindow() { s.active = (s.active - 1 + len(s.windows)) % len(s.windows) }

// KillWindow closes window i. It returns false once no windows are left, at
// which point the caller should kill the session.
func (s *Session) KillWindow(i int) bool {
	if i < 0 || i >= len(s.windows) {
		return false
	}
	for _, p := range s.windows[i].panes {
		p.PTY.Close()
	}
	s.windows = append(s.windows[:i], s.windows[i+1:]...)
	if len(s.windows) == 0 {
		return false
	}
	if s.active >= len(s.windows) {
		s.active = len(s.windows) - 1
	}
	return true
}

// SplitActive splits the active pane and returns the new pane's id.
func (s *Session) SplitActive(o layout.Orientation) (int, error) {
	win := s.ActiveWindow()
	id := win.Layout.Split(o)
	if id < 0 {
		return -1, fmt.Errorf("cannot split")
	}
	// relayout gives it the right size.
	p, err := s.newPane(id, s.area())
	if err != nil {
		return -1, err
	}
	win.panes[id] = p
	s.relayout()
	return id, nil
}

// CloseActivePane closes the active pane, and its window if that was the last
// pane. It returns false when the session has no windows left.
func (s *Session) CloseActivePane() bool {
	win := s.ActiveWindow()
	id := win.Layout.Active()
	if p := win.panes[id]; p != nil {
		p.PTY.Close()
	}
	delete(win.panes, id)
	if !win.Layout.Close(id) {
		return s.KillWindow(s.active)
	}
	s.relayout()
	return true
}

func (s *Session) Navigate(d layout.Direction) {
	s.ActiveWindow().Layout.Navigate(s.area(), d)
}

func (s *Session) ResizePane(d layout.Direction, delta int) bool {
	if !s.ActiveWindow().Layout.Resize(s.area(), d, delta) {
		return false
	}
	s.relayout()
	return true
}

func (s *Session) ToggleZoom() {
	s.ActiveWindow().Layout.ToggleZoom()
	s.relayout()
}

func (s *Session) EnterCopyMode() {
	if p := s.ActiveWindow().ActivePane(); p != nil {
		p.Copy = true
		p.ScrollOff = 0
	}
}

func (s *Session) ExitCopyMode() {
	if p := s.ActiveWindow().ActivePane(); p != nil {
		p.Copy = false
		p.ScrollOff = 0
	}
}

// Scroll moves back (positive delta) or forward through the active pane's
// scrollback while in copy mode, and returns the new offset.
func (s *Session) Scroll(delta int) int {
	p := s.ActiveWindow().ActivePane()
	if p == nil || !p.Copy {
		return 0
	}
	p.ScrollOff += delta
	if p.ScrollOff < 0 {
		p.ScrollOff = 0
	}
	if lim := p.Screen.ScrollbackLen(); p.ScrollOff > lim {
		p.ScrollOff = lim
	}
	return p.ScrollOff
}

func (s *Session) RenameWindow(name string) {
	if name != "" {
		s.ActiveWindow().Name = name
	}
}

// Rename only changes the name; use Manager.Rename to keep the index right.
func (s *Session) Rename(name string) { s.Name = name }

// Resize lays every window out again for a terminal of termW x termH, which
// includes the status bar.
func (s *Session) Resize(termW, termH int) {
	s.areaW = max1(termW)
	s.areaH = max1(termH - 1)
	s.relayout()
}

// relayout resizes every pane's screen and PTY to fit the current layout.
func (s *Session) relayout() {
	for _, w := range s.windows {
		for id, r := range w.Layout.Geometry(s.area()) {
			if p := w.panes[id]; p != nil {
				cw, ch := max1(r.W), max1(r.H)
				p.Screen.Resize(cw, ch)
				p.PTY.Resize(cw, ch)
			}
		}
	}
}

type Manager struct {
	sessions map[string]*Session
	order    []string
	newPTY   PTYFactory
	termW    int
	termH    int
}

// NewManager returns an empty Manager for a client terminal of termW x termH.
func NewManager(f PTYFactory, termW, termH int) *Manager {
	return &Manager{
		sessions: map[string]*Session{},
		newPTY:   f,
		termW:    max1(termW),
		termH:    max1(termH),
	}
}

// Create starts a session with one window. An empty name gets a number.
func (m *Manager) Create(name string) (*Session, error) {
	if name == "" {
		name = fmt.Sprintf("%d", len(m.order))
	}
	if _, ok := m.sessions[name]; ok {
		return nil, fmt.Errorf("duplicate session: %s", name)
	}
	s := &Session{
		Name:   name,
		newPTY: m.newPTY,
		areaW:  m.termW,
		areaH:  max1(m.termH - 1),
	}
	if _, err := s.addWindow("bash"); err != nil {
		return nil, err
	}
	s.active = 0
	m.sessions[name] = s
	m.order = append(m.order, name)
	return s, nil
}

func (m *Manager) Get(name string) *Session { return m.sessions[name] }

// Kill closes all of a session's shells and forgets it. It returns false if
// there is no such session.
func (m *Manager) Kill(name string) bool {
	s, ok := m.sessions[name]
	if !ok {
		return false
	}
	for _, w := range s.windows {
		for _, p := range w.panes {
			p.PTY.Close()
		}
	}
	delete(m.sessions, name)
	for i, n := range m.order {
		if n == name {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	return true
}

// Rename renames a session, failing if it doesn't exist or the new name is
// taken.
func (m *Manager) Rename(oldName, newName string) bool {
	s, ok := m.sessions[oldName]
	if !ok || newName == "" {
		return false
	}
	if _, clash := m.sessions[newName]; clash && newName != oldName {
		return false
	}
	delete(m.sessions, oldName)
	s.Rename(newName)
	m.sessions[newName] = s
	for i, n := range m.order {
		if n == oldName {
			m.order[i] = newName
			break
		}
	}
	return true
}

// Names returns session names in creation order.
func (m *Manager) Names() []string {
	out := make([]string, len(m.order))
	copy(out, m.order)
	return out
}

func (m *Manager) Count() int { return len(m.sessions) }

func (m *Manager) TermSize() (int, int) { return m.termW, m.termH }

// SetTermSize records the client's new terminal size and resizes every session.
func (m *Manager) SetTermSize(w, h int) {
	m.termW, m.termH = max1(w), max1(h)
	for _, s := range m.sessions {
		s.Resize(m.termW, m.termH)
	}
}

// Empty reports whether all sessions are gone, meaning the server can exit.
func (m *Manager) Empty() bool { return len(m.sessions) == 0 }

// Info is one line of list-sessions.
type Info struct {
	Name     string
	Windows  int
	Attached bool
}

// List describes the sessions in creation order.
func (m *Manager) List() []Info {
	out := make([]Info, 0, len(m.order))
	for _, n := range m.order {
		s := m.sessions[n]
		out = append(out, Info{Name: s.Name, Windows: len(s.windows), Attached: s.Attached})
	}
	return out
}

// FormatList formats the sessions like tmux's list-sessions, sorted by name.
func (m *Manager) FormatList() string {
	infos := m.List()
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	out := ""
	for _, in := range infos {
		att := ""
		if in.Attached {
			att = " (attached)"
		}
		win := "window"
		if in.Windows != 1 {
			win = "windows"
		}
		out += fmt.Sprintf("%s: %d %s%s\n", in.Name, in.Windows, win, att)
	}
	return out
}

func max1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}
