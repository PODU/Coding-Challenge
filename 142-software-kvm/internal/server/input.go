package server

import (
	"github.com/podu2997/cckvm/internal/layout"
	"github.com/podu2997/cckvm/internal/platform"
	"github.com/podu2997/cckvm/internal/protocol"
)

// onLocal handles one event from the primary screen's keyboard and mouse.
func (s *Server) onLocal(e platform.Event) {
	remote := s.active != s.primary
	switch e.Kind {
	case platform.MouseMotion:
		// Motion events can still be in flight just after Capture(true).
		if remote {
			return
		}
		s.primary.shape = s.cfg.Screen.Shape()
		s.x, s.y = e.X, e.Y
		if d, ok := s.atEdge(s.primary.shape, e.X, e.Y); ok {
			s.cross(d, e.X, e.Y)
		}
	case platform.MouseDelta:
		if remote {
			s.moveRemote(e.DX, e.DY)
		}
	case platform.MouseButton:
		if e.Button < 32 {
			if e.Down {
				s.buttons |= 1 << e.Button
			} else {
				s.buttons &^= 1 << e.Button
			}
		}
		if remote {
			if e.Down {
				s.sendTo(s.active, protocol.MouseDown{Button: e.Button})
			} else {
				s.sendTo(s.active, protocol.MouseUp{Button: e.Button})
			}
		}
	case platform.MouseWheel:
		if remote {
			s.sendTo(s.active, protocol.MouseWheel{DX: int16(e.DX), DY: int16(e.DY)})
		}
	case platform.Key:
		s.mask = e.Mask
		if !remote {
			return
		}
		switch {
		case e.Down && e.Repeat:
			s.sendTo(s.active, protocol.KeyRepeat{ID: e.KeyID, Mask: e.Mask, Count: 1, Button: e.KeyButton})
		case e.Down:
			s.sendTo(s.active, protocol.KeyDown{ID: e.KeyID, Mask: e.Mask, Button: e.KeyButton})
		default:
			s.sendTo(s.active, protocol.KeyUp{ID: e.KeyID, Mask: e.Mask, Button: e.KeyButton})
		}
	case platform.ClipboardChanged:
		s.localClipboardChanged(e.ClipboardID)
	case platform.ScreenSaver:
		s.broadcast(protocol.ScreenSaver{On: e.On}, s.primary.name)
		if e.On && remote {
			x, y := s.primary.shape.Center()
			s.switchTo(s.primary, x, y)
		}
	case platform.ShapeChanged:
		s.primary.shape = s.cfg.Screen.Shape()
	}
}

// atEdge reports which edge of r the local cursor is pressed against. The
// OS clamps the cursor, so touching the last pixel row or column counts.
func (s *Server) atEdge(r platform.Rect, x, y int) (layout.Direction, bool) {
	switch {
	case x <= r.X:
		return layout.Left, true
	case x >= r.X+r.W-1:
		return layout.Right, true
	case y <= r.Y:
		return layout.Up, true
	case y >= r.Y+r.H-1:
		return layout.Down, true
	}
	return 0, false
}

// moveRemote applies a captured mouse delta to the active remote screen.
// Moving past an edge with a connected neighbour switches screens;
// otherwise the cursor is clamped and the client told where it went.
func (s *Server) moveRemote(dx, dy int) {
	r := s.active.shape
	nx, ny := s.x+dx, s.y+dy
	var d layout.Direction
	crossed := true
	switch {
	case nx < r.X:
		d = layout.Left
	case nx >= r.X+r.W:
		d = layout.Right
	case ny < r.Y:
		d = layout.Up
	case ny >= r.Y+r.H:
		d = layout.Down
	default:
		crossed = false
	}
	nx, ny = r.Clamp(nx, ny)
	if crossed && s.cross(d, nx, ny) {
		return
	}
	if nx == s.x && ny == s.y {
		return
	}
	if s.cfg.Relative {
		s.sendTo(s.active, protocol.MouseRelMove{DX: int16(nx - s.x), DY: int16(ny - s.y)})
	} else {
		s.sendTo(s.active, protocol.MouseMove{X: int16(nx), Y: int16(ny)})
	}
	s.x, s.y = nx, ny
}

// cross moves the cursor, last seen at (x, y), through edge d of the active
// screen if a connected screen is linked there. It refuses while a mouse button is held
// (so drags stay on one screen) or while Scroll Lock locks the cursor.
func (s *Server) cross(d layout.Direction, x, y int) bool {
	if s.buttons != 0 || s.mask&protocol.ModScrollLock != 0 {
		return false
	}
	name, ok := s.cfg.Layout.Neighbor(s.active.name, d)
	if !ok {
		return false
	}
	dst := s.screens[name]
	if dst == nil {
		return false // linked but not connected
	}
	x, y = entryPoint(s.active.shape, dst.shape, d, x, y)
	s.log.Info("switching screens", "from", s.active.name, "to", dst.name, "edge", d.String(), "x", x, "y", y)
	s.switchTo(dst, x, y)
	return true
}

// entryPoint maps a cursor leaving src through edge d to the point where it
// appears on dst: at the same relative position along the edge, so screens
// of different sizes line up, and one pixel away from every edge of dst so
// it does not immediately trigger another switch.
func entryPoint(src, dst platform.Rect, d layout.Direction, x, y int) (int, int) {
	scale := func(v, from0, fromLen, to0, toLen int) int {
		if fromLen <= 1 {
			return to0 + toLen/2
		}
		return to0 + (v-from0)*(toLen-1)/(fromLen-1)
	}
	var nx, ny int
	switch d {
	case layout.Left:
		nx, ny = dst.X+dst.W-2, scale(y, src.Y, src.H, dst.Y, dst.H)
	case layout.Right:
		nx, ny = dst.X+1, scale(y, src.Y, src.H, dst.Y, dst.H)
	case layout.Up:
		nx, ny = scale(x, src.X, src.W, dst.X, dst.W), dst.Y+dst.H-2
	case layout.Down:
		nx, ny = scale(x, src.X, src.W, dst.X, dst.W), dst.Y+1
	}
	if dst.W > 2 && dst.H > 2 {
		dst = platform.Rect{X: dst.X + 1, Y: dst.Y + 1, W: dst.W - 2, H: dst.H - 2}
	}
	return dst.Clamp(nx, ny)
}

// switchTo makes dst the active screen with the cursor at (x, y).
func (s *Server) switchTo(dst *screen, x, y int) {
	src := s.active
	if src == dst {
		return
	}
	if src.proxy != nil {
		s.sendTo(src, protocol.Leave{})
	}
	s.active = dst
	s.x, s.y = x, y
	if dst == s.primary {
		s.cfg.Screen.Capture(false)
		s.cfg.Screen.MouseMove(x, y)
		return
	}
	if src == s.primary {
		s.cfg.Screen.Capture(true)
	}
	s.seq++
	s.sendTo(dst, protocol.Enter{X: int16(x), Y: int16(y), Seq: s.seq, Mask: s.mask})
	s.flushClipboards(dst)
}
